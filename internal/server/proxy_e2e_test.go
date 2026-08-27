package server_test

// proxy_e2e_test.go 锁定 07-03 反代信任特性的黑盒全链行为（SEC-07，D-15..D-20）：
// TestRemoteUserLogging——attach 链路 logEvent 事件携 sanitize 后 remote_user
// 字段，不携头事件不出 remote_user 键（与迁移前文本行语义一致）；TestXFFThrottleKey——trust
// 开启时 throttle per-IP 键换 XFF 链首（不同 XFF 独立计数），未配置时 XFF 完全
// 忽略（回退 TCP 对端共享计数，D-20 单一信任闸）；TestAuthHeaderNoAuthBypass——
// 伪造头不绕过 Basic 认证（D-17 正交性回归锁）。
//
// 与白盒 proxy_test.go 分文件（05-04 resize 两测试分文件先例）：Go 单文件单
// package 约束——本组断言依赖 server_test 包的 startTrackedServerWith/
// captureStderr（05-01 登记的 -race 同步纪律：restore() 与 logEvent 读
// os.Stderr 的同步边只能由 handler 追踪 WaitGroup 建立）。

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// TestRemoteUserLogging（D-15/D-19）：Options.AuthHeader 配置后，attach 链路的
// logEvent 事件携 remote_user 键（值为 sanitize 后头值）；头缺席时事件不出
// remote_user 键（与迁移前文本行语义一致）。触发事件取 WS attach 携无效 ticket
// 的 auth_failed——attach 链路唯一的确定性即时事件（凭据模式）。C1 控制字符头值
// （U+0085 NEL——Go http 客户端合法可发、服务端按 D-19 剥离）断言 sanitize 在
// 提取点生效；C0/DEL 在 Go http 客户端侧即被拒（httpguts.ValidHeaderFieldValue），
// 剥离覆盖由白盒 TestSanitizeRemoteUser 表驱动锁定。
func TestRemoteUserLogging(t *testing.T) {
	restore := captureStderr(t)
	defer restore() // 失败路径兜底恢复 os.Stderr（幂等）

	cred, err := server.ParseCredential("ru-op:ru-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	_, wsURL, waitHandlers := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{
		Credentials: []server.Credential{cred},
		AuthHeader:  "X-Remote-User",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// dialBadTicket 携无效 ticket 完成 WS 握手 → 服务端 auth_failed（1008）——
	// attach 链路 logEvent 的确定性触发形态。
	dialBadTicket := func(headers http.Header) {
		t.Helper()
		c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			Subprotocols: []string{proto.Subprotocol},
			HTTPHeader:   headers,
		})
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		payload, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24, Ticket: "invalid-ticket"})
		if err != nil {
			t.Fatalf("marshal Hello: %v", err)
		}
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, payload...)); err != nil {
			t.Fatalf("write Hello: %v", err)
		}
		ce := readCloseErr(t, c, ctx)
		if ce.Code != websocket.StatusPolicyViolation {
			t.Fatalf("close code = %d, want %d (1008 auth_failed)", ce.Code, websocket.StatusPolicyViolation)
		}
	}

	dialBadTicket(http.Header{"X-Remote-User": []string{"alice"}})
	dialBadTicket(http.Header{"X-Remote-User": []string{"c\u0085arol"}}) // C1 NEL 剥离 → carol
	dialBadTicket(nil)                                                   // 不携头：不出 remote_user 键

	// 同步边：等全部 Attach handler 返回——logEvent 在 handler 内先于返回执行，
	// WaitGroup happens-before 使 restore() 的 os.Stderr 写与该读同步（05-01 纪律）。
	waitHandlers()
	out := restore()
	evs := parseEvents(t, out)
	if n := countByEvent(evs, proto.ErrAuthFailed); n != 3 {
		t.Fatalf("auth_failed events = %d, want 3: %q", n, out)
	}
	withUser := 0
	var hasAlice, hasCarol bool
	for _, m := range evs {
		u, ok := m["remote_user"]
		if !ok {
			continue
		}
		withUser++
		if u == "alice" {
			hasAlice = true
		}
		if u == "carol" {
			hasCarol = true
		}
	}
	if !hasAlice {
		t.Errorf("stderr missing remote_user=alice event: %q", out)
	}
	if !hasCarol {
		t.Errorf("stderr missing sanitized remote_user=carol event（C1 剥离证据）: %q", out)
	}
	if withUser != 2 {
		t.Errorf("携 remote_user 键的事件数 = %d, want 2（不携头事件不出键语义保持）: %q", withUser, out)
	}
}

// TestXFFThrottleKey（D-20）：trust 开启（--auth-header 给定）时 throttle
// per-IP 计数键换 XFF 链首——同一 XFF 连续失败触发 429 而另一 XFF 不受影响
// （各自独立计数）；未配置时 XFF 完全忽略——直连客户端自设 XFF 无任何效果，
// 两不同 XFF 共享 TCP 对端回退键计数（D-20 零双轨）。ThrottleBase 30s 使退避
// 窗口在测试期间恒不到期（窗口语义与断言解耦）。
func TestXFFThrottleKey(t *testing.T) {
	cred, err := server.ParseCredential("xff-op:xff-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	newServer := func(authHeader string) string {
		_, wsURL, _ := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{
			Credentials:  []server.Credential{cred},
			ThrottleBase: 30 * time.Second,
			AuthHeader:   authHeader,
		})
		return attachURL(wsURL)
	}
	postBadCreds := func(url, xff string) int {
		t.Helper()
		headers := map[string]string{}
		if xff != "" {
			headers["X-Forwarded-For"] = xff
		}
		resp := postAttach(t, url, "xff-op", "wrong-pass", headers)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	t.Run("trust on: XFF 首 IP 独立计数", func(t *testing.T) {
		url := newServer("X-Remote-User")
		if code := postBadCreds(url, "203.0.113.7"); code != http.StatusUnauthorized {
			t.Fatalf("fail#1 status = %d, want %d (401)", code, http.StatusUnauthorized)
		}
		if code := postBadCreds(url, "203.0.113.7"); code != http.StatusTooManyRequests {
			t.Fatalf("same XFF second status = %d, want %d (429——throttle 键已换 XFF 首 IP)", code, http.StatusTooManyRequests)
		}
		if code := postBadCreds(url, "198.51.100.9"); code != http.StatusUnauthorized {
			t.Fatalf("other XFF status = %d, want %d (401——另一 XFF 独立计数不受影响)", code, http.StatusUnauthorized)
		}
	})

	t.Run("trust off: XFF 完全忽略共享回退键", func(t *testing.T) {
		url := newServer("")
		if code := postBadCreds(url, "203.0.113.7"); code != http.StatusUnauthorized {
			t.Fatalf("fail#1 status = %d, want %d (401)", code, http.StatusUnauthorized)
		}
		if code := postBadCreds(url, "198.51.100.9"); code != http.StatusTooManyRequests {
			t.Fatalf("different XFF status = %d, want %d (429——XFF 忽略，键回退 TCP 对端共享计数)", code, http.StatusTooManyRequests)
		}
	})
}

// TestAuthHeaderNoAuthBypass（D-17 正交性回归锁）：携伪造 X-Remote-User: root
// 头、无凭据请求 /api/attach → 401 照旧（Basic 挑战形态不变）——auth-header
// 值只做记录不做任何认证决定，伪造头不能绕过任何认证检查（T-07-03a/Pitfall 7
// 「头存在跳过 Basic」反模式防线）。
func TestAuthHeaderNoAuthBypass(t *testing.T) {
	cred, err := server.ParseCredential("bypass-op:bypass-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	_, wsURL, _ := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{
		Credentials: []server.Credential{cred},
		AuthHeader:  "X-Remote-User",
	})
	resp := postAttach(t, attachURL(wsURL), "", "", map[string]string{"X-Remote-User": "root"})
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("forged header without creds status = %d, want %d (401——伪造头不得绕过 Basic)", resp.StatusCode, http.StatusUnauthorized)
	}
	if wa := resp.Header.Get("WWW-Authenticate"); wa != `Basic realm="wesh", charset="UTF-8"` {
		t.Errorf("WWW-Authenticate = %q, want RFC 7617 challenge（挑战形态不因伪造头改变）", wa)
	}
}

// TestShareChannelRemoteUser（D-15 双通道同口径 + D-03 红线延伸自净，07-03
// Task 2）：issueTicketJSON 是两签发通道（Basic 链 attachHandler / token 分支
// shareAttach）唯一共享签发点——其 max_clients 503 事件在两通道均携 remote_user
// （Task 1 单点改造的自然覆盖，本测试双通道断言锁定）；token 渠道进入的 WS
// attach 经同一 Attach 入口提取点（remote/remoteUser 与 Basic 渠道同码同源，
// server.go Attach 注释登记），以其稳态 message_too_big 事件断言该渠道客户端
// 的 remote_user 已随 client 装配生效。收尾断言 share token 与一次性 ticket
// 值不出现在 stderr 任何事件行（D-03 红线随新字段延伸的运行时自净断言，
// T-07-03c——remote_user 提取源在结构上不可能是 token/ticket）。
func TestShareChannelRemoteUser(t *testing.T) {
	restore := captureStderr(t)
	defer restore() // 失败路径兜底恢复 os.Stderr（幂等）

	cred, err := server.ParseCredential("share-op:share-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	roTok := server.GenerateShareToken()
	rwTok := server.GenerateShareToken()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// —— ① 双通道 503：MaxClients=1 满员后，Basic 链与 token 分支各探测一次 ——
	_, wsURL, waitHandlers := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{
		Credentials:  []server.Credential{cred},
		AuthHeader:   "X-Remote-User",
		ShareTokenRO: roTok,
		ShareTokenRW: rwTok,
		MaxClients:   1,
	})
	// 占位客户端（Basic 渠道正式 attach）占满容量。
	resp := postAttach(t, attachURL(wsURL), "share-op", "share-pass", nil)
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
		resp.Body.Close()
		t.Fatalf("decode placeholder ticket: %v", err)
	}
	resp.Body.Close()
	if issued.Ticket == "" {
		t.Fatal("placeholder ticket empty")
	}
	placeholder, _ := dialHelloTicket(t, ctx, wsURL, issued.Ticket, 80, 24)

	// Basic 通道 503：有效凭据 + 满员 → 503，事件 remote_user 为 bob。
	resp = postAttach(t, attachURL(wsURL), "share-op", "share-pass", map[string]string{"X-Remote-User": "bob"})
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Basic 通道满员 status = %d, want %d (503)", resp.StatusCode, http.StatusServiceUnavailable)
	}

	// token 通道 503：body 携有效 ro token + 满员 → 503，事件 remote_user 为 carol
	//（shareAttach → issueTicketJSON 同一签发点，token 分支绕过 Basic 是 D-01
	// 既定 capability 语义）。
	tokenBody, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: roTok})
	if err != nil {
		t.Fatalf("marshal token body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, attachURL(wsURL), bytes.NewReader(tokenBody))
	if err != nil {
		t.Fatalf("new token POST: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Remote-User", "carol")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("token POST: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("token 通道满员 status = %d, want %d (503)", resp.StatusCode, http.StatusServiceUnavailable)
	}

	// 收口占位客户端使其 Attach handler 返回——waitHandlers 同步边的前置
	//（WS attach 是长生命周期 handler，不关闭则 wg.Wait 永不返回）。
	_ = placeholder.Close(websocket.StatusNormalClosure, "")
	waitHandlers()

	// —— ② token 渠道 WS attach 提取点（独立实例，默认容量）——
	// token 渠道签发的 ticket + WS 升级携 X-Remote-User: dave → 该客户端的
	// remoteUser 来自与 Basic 渠道同一 Attach 入口提取行。
	_, wsURL2, waitHandlers2 := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{
		Credentials:  []server.Credential{cred},
		AuthHeader:   "X-Remote-User",
		ShareTokenRO: roTok,
		ShareTokenRW: rwTok,
	})
	req2, err := http.NewRequest(http.MethodPost, attachURL(wsURL2), bytes.NewReader(tokenBody))
	if err != nil {
		t.Fatalf("new token POST #2: %v", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("token POST #2: %v", err)
	}
	var issued2 struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issued2); err != nil {
		resp.Body.Close()
		t.Fatalf("decode token-channel ticket: %v", err)
	}
	resp.Body.Close()
	if issued2.Ticket == "" {
		t.Fatal("token-channel ticket empty")
	}
	c2, _, err := websocket.Dial(ctx, wsURL2, &websocket.DialOptions{
		Subprotocols: []string{proto.Subprotocol},
		HTTPHeader:   http.Header{"X-Remote-User": []string{"dave"}},
	})
	if err != nil {
		t.Fatalf("dial token channel: %v", err)
	}
	hello, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24, Ticket: issued2.Ticket})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c2.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, hello...)); err != nil {
		t.Fatalf("write Hello: %v", err)
	}
	// Welcome 首帧 = attach 完成证据（token 渠道 ro mode）。
	_, data, err := c2.Read(ctx)
	if err != nil {
		t.Fatalf("read Welcome: %v", err)
	}
	if len(data) == 0 || data[0] != proto.Welcome {
		t.Fatalf("first frame = %v, want Welcome ('W')", data)
	}
	// 稳态事件触发：超限消息 → 库自动 1009 + message_too_big 事件 remote_user 为 dave。
	if err := c2.Write(ctx, websocket.MessageBinary, make([]byte, proto.ReadLimitPostAuth+1)); err != nil {
		t.Fatalf("write oversize: %v", err)
	}
	ce := readCloseErr(t, c2, ctx)
	if ce.Code != websocket.StatusMessageTooBig {
		t.Fatalf("close code = %d, want %d (1009)", ce.Code, websocket.StatusMessageTooBig)
	}
	waitHandlers2()

	out := restore()
	evs := parseEvents(t, out)
	// 双通道 503 同口径：两条 max_clients 事件各携对应通道的 remote_user。
	if n := countByEvent(evs, "max_clients"); n != 2 {
		t.Fatalf("max_clients events = %d, want 2（Basic/token 双通道）: %q", n, out)
	}
	var bobOK, carolOK, daveOK bool
	for _, m := range evs {
		switch m["event"] {
		case "max_clients":
			if m["remote_user"] == "bob" {
				bobOK = true
			}
			if m["remote_user"] == "carol" {
				carolOK = true
			}
		case "message_too_big":
			if m["remote_user"] == "dave" {
				daveOK = true
			}
		}
	}
	if !bobOK {
		t.Errorf("Basic 通道 503 事件缺 remote_user=bob: %q", out)
	}
	if !carolOK {
		t.Errorf("token 通道 503 事件缺 remote_user=carol: %q", out)
	}
	// token 渠道 WS attach 提取点：稳态事件携 dave。
	if n := countByEvent(evs, "message_too_big"); n != 1 {
		t.Fatalf("message_too_big events = %d, want 1: %q", n, out)
	}
	if !daveOK {
		t.Errorf("token 渠道 WS attach 稳态事件缺 remote_user=dave: %q", out)
	}
	// D-03 红线延伸运行时自净断言（T-07-03c）：share token 与一次性 ticket
	// 值不出现在 stderr 任何事件行——remote_user/remote 任何字段在结构上
	// 不可能携带 token（提取源只能是配置头名对应的 HTTP 头）。
	if strings.Contains(out, roTok) || strings.Contains(out, rwTok) {
		t.Errorf("stderr leaks share token（D-03 红线）: %q", out)
	}
	if strings.Contains(out, issued.Ticket) || strings.Contains(out, issued2.Ticket) {
		t.Errorf("stderr leaks ticket（D-03 红线）: %q", out)
	}
}
