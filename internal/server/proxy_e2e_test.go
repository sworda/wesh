package server_test

// proxy_e2e_test.go 锁定 07-03 反代信任特性的黑盒全链行为（SEC-07，D-15..D-20）：
// TestRemoteUserLogging——attach 链路 logEvent 行携 sanitize 后 remote_user，
// 不携头行与现状逐字节一致（不出 remote_user 键）；TestXFFThrottleKey——trust
// 开启时 throttle per-IP 键换 XFF 链首（不同 XFF 独立计数），未配置时 XFF 完全
// 忽略（回退 TCP 对端共享计数，D-20 单一信任闸）；TestAuthHeaderNoAuthBypass——
// 伪造头不绕过 Basic 认证（D-17 正交性回归锁）。
//
// 与白盒 proxy_test.go 分文件（05-04 resize 两测试分文件先例）：Go 单文件单
// package 约束——本组断言依赖 server_test 包的 startTrackedServerWith/
// captureStderr（05-01 登记的 -race 同步纪律：restore() 与 logEvent 读
// os.Stderr 的同步边只能由 handler 追踪 WaitGroup 建立）。

import (
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
// logEvent 行携 remote_user=<sanitize 后头值>；头缺席时日志行与现状逐字节一致
// （不出 remote_user 键）。触发事件取 WS attach 携无效 ticket 的 auth_failed
// 行——attach 链路唯一的确定性即时事件（凭据模式）。C1 控制字符头值（U+0085
// NEL——Go http 客户端合法可发、服务端按 D-19 剥离）断言 sanitize 在提取点
// 生效；C0/DEL 在 Go http 客户端侧即被拒（httpguts.ValidHeaderFieldValue），
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
	if n := strings.Count(out, "reason=auth_failed"); n != 3 {
		t.Fatalf("auth_failed rows = %d, want 3: %q", n, out)
	}
	if !strings.Contains(out, " remote_user=alice\n") {
		t.Errorf("stderr missing ` remote_user=alice` row: %q", out)
	}
	if !strings.Contains(out, " remote_user=carol\n") {
		t.Errorf("stderr missing sanitized ` remote_user=carol` row（C1 剥离证据）: %q", out)
	}
	if n := strings.Count(out, "remote_user="); n != 2 {
		t.Errorf("remote_user= occurrences = %d, want 2（不携头行与现状逐字节一致不出键）: %q", n, out)
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
