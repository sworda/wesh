package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// ====== plan 03-03 增量：认证集成测试组（SEC-01..SEC-04 行为锁）======
//
// 黑盒 package server_test：复用 e2e_test.go 的 startTestServerWith/dialHello/
// waitExit 与 03-03 新增 dialHelloTicket/attachURL/postAttach helper（白盒
// auth_test.go/throttle_test.go/origin_test.go 保持 03-01/03-02 纯单元测零改动）。
// 超时护栏一律 10s ctx；每场景独立 startTestServerWith 实例（单会话模型约束：
// 每服务器实例仅一次握手机会，见 plan objective）。
//
// 测试与需求映射：
//   - TestAttachFlow（SEC-01/SEC-02，D-02/D-10/D-11）：tracer 主链路
//     Basic → ticket → Hello 核销 → Welcome（401 同文 / 200+no-store / mode=rw）
//   - TestTicketInvalid（SEC-02，D-10）：非法 ticket 统一 Error{auth_failed}+1008
//   - TestNoAuthMode（D-02）：无认证模式 /api/attach 404 探测 + 无 ticket 直连零漂移
//   - TestAttachEndpoint（SEC-02）：端点守卫链 405+Allow / 413 / 403 不回显 /
//     200 三头 + ticket 形态 / nosniff 横向证据
//   - TestTicketExpiry（SEC-02，D-10）：TTL 过期与非法同口径 auth_failed
//   - TestLogRedaction（SEC-01 红线）：运行时捕获断言四类禁出串 + 正向对照
//   - TestThrottleHTTP（SEC-03，D-08/D-09）：429+Retry-After / 成功清零 / 级数重启
//   - TestThrottleHelloSharedCounter（D-08）：HTTP 失败共享计数器闸住 WS 核销
//   - TestOriginEndpoints（SEC-04，D-12/D-13）：双端点 Origin 白名单执行

// dialHelloTicketWantAuthFailed 负例 helper：Hello 携 ticket 拨号，断言首帧收
// Error{code:auth_failed, message 非空}（正常客户端可见错误发 Error 帧，D-06），
// 续读 CloseError Code==1008 且 Reason==auth_failed（code 与 close reason 同名
// 机器串，D-07/D-10）。过期/非法/重放/节流中 wire 形态不可区分（D-10 无 oracle）。
func dialHelloTicketWantAuthFailed(t *testing.T, ctx context.Context, wsURL, ticket string) {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()
	hello, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24, Ticket: ticket})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, hello...)); err != nil {
		t.Fatalf("write Hello: %v", err)
	}
	// 首读：'E' Error 帧（与 version_mismatch 同形态，PATTERNS 关闭码+reason 双断言模式）。
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read Error frame: %v", err)
	}
	if len(data) == 0 || data[0] != proto.Error {
		t.Fatalf("first frame = %v, want Error ('E')", data)
	}
	var ep proto.ErrorPayload
	if err := json.Unmarshal(data[1:], &ep); err != nil {
		t.Fatalf("decode Error frame: %v", err)
	}
	if ep.Code != proto.ErrAuthFailed {
		t.Fatalf("error code = %q, want %q（D-10 统一口径）", ep.Code, proto.ErrAuthFailed)
	}
	if ep.Message == "" {
		t.Fatal("error message empty — D-07 requires human-readable message for client display")
	}
	// 继续读至 1008 关闭，reason 与 Error code 同名机器串。
	var ce websocket.CloseError
	for {
		if _, _, rerr := c.Read(ctx); rerr != nil {
			if !errors.As(rerr, &ce) {
				t.Fatalf("read terminated without CloseError: %v", rerr)
			}
			break
		}
	}
	if ce.Code != websocket.StatusPolicyViolation {
		t.Fatalf("close code = %d, want %d (1008)", ce.Code, websocket.StatusPolicyViolation)
	}
	if ce.Reason != proto.ErrAuthFailed {
		t.Fatalf("close reason = %q, want %q（与 Error code 同名机器串，D-07）", ce.Reason, proto.ErrAuthFailed)
	}
}

// TestAttachFlow（03-03 tracer）：认证主链路端到端——401 无凭据 → sleep 过窗 →
// 401 错凭据（与无凭据同文，无枚举 oracle）→ sleep 过窗 → 200 正确凭据取 ticket
//（Cache-Control: no-store）→ Hello 核销 → Welcome{mode:"rw"}（D-11 ticket 绑定
// = 全局 writable）→ 正常关闭 → waitExit(0)。单一 happy path 证明
// Basic → ticket → Hello → Welcome 全链路可达（ROADMAP 准则 1 行为落地）。
//
// ThrottleBase 必须注入 ms 级覆写：本编排含 ≥2 次连续失败请求，生产默认 base=1s
// 下 #1 的 401 会使该 IP 进入 1s 窗口，后续请求确定性 429 而非预期断言形态
//（镜像 TestThrottleHTTP 的 pacing 模式）。
func TestAttachFlow(t *testing.T) {
	cred, err := server.ParseCredential("uat-alice:correct-horse")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:     true,
		Credentials:  []server.Credential{cred},
		ThrottleBase: 50 * time.Millisecond,
	})
	url := attachURL(wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// #1 无凭据 → 401 + WWW-Authenticate challenge（fails=1，notBefore=+50ms）。
	resp := postAttach(t, url, "", "", nil)
	bodyNoCred, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read 401 body: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-credential status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	if wa := resp.Header.Get("WWW-Authenticate"); wa != `Basic realm="wesh", charset="UTF-8"` {
		t.Fatalf("WWW-Authenticate = %q, want %q (RFC 7617)", wa, `Basic realm="wesh", charset="UTF-8"`)
	}

	time.Sleep(100 * time.Millisecond) // 过窗（fail#1 窗口 = 1×base = 50ms）

	// #2 错凭据 → 401 且与无凭据 body 逐字节相等（同文无 oracle，OWASP 纪律；
	// fails=2，notBefore=+100ms=2×base）。
	resp = postAttach(t, url, "uat-alice", "wrong-horse", nil)
	bodyWrongCred, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read 401 body: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-credential status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	if string(bodyNoCred) != string(bodyWrongCred) {
		t.Fatalf("401 bodies differ — 枚举 oracle（无/错凭据必须完全同文）:\nno-cred:    %q\nwrong-cred: %q", bodyNoCred, bodyWrongCred)
	}

	time.Sleep(150 * time.Millisecond) // 过窗（fail#2 窗口 = 2×base = 100ms）

	// #3 正确凭据 → 200 + Cache-Control: no-store + ticket 非空（recordSuccess
	// 清零计数器，后续 Hello 核销无节流残留）。
	resp = postAttach(t, url, "uat-alice", "correct-horse", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("correct-credential status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		resp.Body.Close()
		t.Fatalf("Cache-Control = %q, want no-store（ticket 不可落缓存）", cc)
	}
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
		resp.Body.Close()
		t.Fatalf("decode ticket response: %v", err)
	}
	resp.Body.Close()
	if issued.Ticket == "" {
		t.Fatal("ticket empty in 200 response")
	}

	// Hello 携 ticket 核销 → Welcome mode=="rw"（D-11：ticket 绑定 = 全局 writable）。
	c, mode := dialHelloTicket(t, ctx, wsURL, issued.Ticket, 80, 24)
	if mode != proto.ModeRW {
		t.Fatalf("welcome mode = %q, want %q（ticket 绑定 = writable 装配）", mode, proto.ModeRW)
	}

	// 正常关闭 → D-11 收口。
	c.Close(websocket.StatusNormalClosure, "")
	waitExit(t, exitCh, 0)
}

// TestTicketInvalid（SEC-02/D-10）：Hello 携从未签发的 ticket（22 字符合法形态
// 但表内无键）→ 首帧 Error{auth_failed} → 1008 且 close reason 同名机器串。
// wire 级非法 ticket 与重放（已删除键）走同一代码路径——重放拒绝由此路径 +
// 03-01 ticketStore 单元测重放 false + 03-06 UAT 端到端三层组合证明
//（单会话模型约束下 wire 级重放不可构造，plan objective 裁决）。
func TestTicketInvalid(t *testing.T) {
	cred, err := server.ParseCredential("uat-bob:hunter2-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:     true,
		Credentials:  []server.Credential{cred},
		ThrottleBase: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// 单次失败无 pacing 需求（每测试独立实例）。
	dialHelloTicketWantAuthFailed(t, ctx, wsURL, "AAAAAAAAAAAAAAAAAAAAAA")

	// 服务端关 conn 后落入读循环，下一拍 reader 终结 → D-11 收口。
	waitExit(t, exitCh, 0)
}

// TestNoAuthMode（D-02）：无凭据装配 → /api/attach 返回 404（前端探测信号：
// 跳过 fetch 直连 WS）；Hello 无 ticket 字段直接收 Welcome（核销分支整体跳过，
// 既有行为零漂移——全部既有无凭据测试零改动保持绿色的同链路反证）。
func TestNoAuthMode(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{})

	// /api/attach 显式注册 404（无认证模式探测信号）。
	resp := postAttach(t, attachURL(wsURL), "", "", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("no-auth /api/attach status = %d, want %d (404)", resp.StatusCode, http.StatusNotFound)
	}

	// Hello 无 ticket 直连 → Welcome（无认证模式 throttle 为 nil，无 pacing 需求）。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 80, 24)
	if mode != proto.ModeRO {
		t.Fatalf("welcome mode = %q, want %q（零值 Options = 默认只读）", mode, proto.ModeRO)
	}

	// 正常关闭 → D-11 收口。
	c.Close(websocket.StatusNormalClosure, "")
	waitExit(t, exitCh, 0)
}

// ====== Task 2：端点守卫链 / ticket 过期 / 日志红线 ======

// TestAttachEndpoint（SEC-02 端点守卫链）：/api/attach 的 HTTP 层守卫形态——
// a) GET → 405 + Allow: POST（ServeMux 方法模式白拿，处理器未达）；
// b) POST 超 1KiB body → 413（MaxBytesReader 纯防御上限，D-11 请求体为空语义）；
// c) POST 邪恶 Origin → 403 且正文不回显 Origin 值（无反射面）；
// d) POST 正确凭据（无 Origin——D-13 非浏览器放行）→ 200 + Content-Type:
// application/json + Cache-Control: no-store + 22 字符 ticket；
// e) 200 响应同时携带 X-Content-Type-Options: nosniff（securityHeaders 最外层
// 装配的横向证据）。
// 本测试只验 HTTP 层：全程不 dial WS（无终结路径触发，exitf 不断言）；b)/d)
// 均认证成功（recordSuccess），c) 在 basicAuth 之前 403——无失败计数编排，
// 生产默认节流参数下无 pacing 需求。
func TestAttachEndpoint(t *testing.T) {
	cred, err := server.ParseCredential("ep-carol:endpoint-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:    true,
		Credentials: []server.Credential{cred},
	})
	url := attachURL(wsURL)

	// a) GET → 405 + Allow 头含 POST（方法模式在 mux 层拒绝，不进任何中间件）。
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d (405)", resp.StatusCode, http.StatusMethodNotAllowed)
	}
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, http.MethodPost) {
		t.Fatalf("Allow = %q, want 含 POST（ServeMux 方法模式）", allow)
	}

	// b) POST 2KiB body（正确凭据）→ 413（1KiB MaxBytesReader 上限）。
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(make([]byte, 2048)))
	if err != nil {
		t.Fatalf("new POST request: %v", err)
	}
	req.SetBasicAuth("ep-carol", "endpoint-pass")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST 2KiB body: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body status = %d, want %d (413)", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}

	// c) POST 邪恶 Origin（正确凭据）→ 403（Origin 闸在 basicAuth 之前）且
	// 正文不回显 Origin 值。
	resp = postAttach(t, url, "ep-carol", "endpoint-pass", map[string]string{"Origin": "https://evil.example"})
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read 403 body: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("evil-origin status = %d, want %d (403)", resp.StatusCode, http.StatusForbidden)
	}
	if strings.Contains(string(body), "evil.example") {
		t.Fatalf("403 body echoes Origin value（反射面）: %q", body)
	}

	// d)+e) POST 正确凭据（无 Origin——D-13 非浏览器放行）→ 200 三头 + ticket 形态。
	resp = postAttach(t, url, "ep-carol", "endpoint-pass", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("correct-credential status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		resp.Body.Close()
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		resp.Body.Close()
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	if nosniff := resp.Header.Get("X-Content-Type-Options"); nosniff != "nosniff" {
		resp.Body.Close()
		t.Fatalf("X-Content-Type-Options = %q, want nosniff（securityHeaders 最外层装配证据）", nosniff)
	}
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
		resp.Body.Close()
		t.Fatalf("decode ticket response: %v", err)
	}
	resp.Body.Close()
	if len(issued.Ticket) != 22 {
		t.Fatalf("ticket length = %d, want 22（base64.RawURLEncoding(16B)）", len(issued.Ticket))
	}
	// 收口：d) 的 ticket 不使用——本测试只验 HTTP 层，不 dial 即无终结路径触发，直接 return。
}

// TestTicketExpiry（SEC-02 TTL/D-10）：TicketTTL=100ms 注入 → 取 ticket 后
// sleep 200ms 过期 → Hello 携该 ticket 与非法 ticket 同口径
// Error{auth_failed} + 1008 + reason 同名（过期不可区分，无 oracle）。
// 真实 sleep 仅此一处——TTL 相对语义由 now 注入无法跨进程模拟；200ms 对 10s
// 护栏余量充足。
func TestTicketExpiry(t *testing.T) {
	cred, err := server.ParseCredential("exp-dave:expiry-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:    true,
		Credentials: []server.Credential{cred},
		TicketTTL:   100 * time.Millisecond,
	})

	resp := postAttach(t, attachURL(wsURL), "exp-dave", "expiry-pass", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("issue ticket status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
		resp.Body.Close()
		t.Fatalf("decode ticket response: %v", err)
	}
	resp.Body.Close()
	if issued.Ticket == "" {
		t.Fatal("ticket empty in 200 response")
	}

	time.Sleep(200 * time.Millisecond) // 等 TTL 过期（100ms 的 2 倍余量）

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dialHelloTicketWantAuthFailed(t, ctx, wsURL, issued.Ticket)

	// 服务端关 conn 后落入读循环，下一拍 reader 终结 → D-11 收口。
	waitExit(t, exitCh, 0)
}

// TestLogRedaction（SEC-01 红线，RESEARCH Pattern 8 定稿）：os.Pipe 捕获 stderr
// 跑完整失败轮——(0) 正确凭据取一张 ticket 作对照样本（200，recordSuccess 清零
// 计数器，不影响后续编排）；(a) 错凭据 → 401（fails=1，notBefore=+1s 生产默认
// base）；(b) 立即再错凭据 → 429（窗口内第 1 次请求即命中节流——429 起于 fail 后
// 窗口内请求，无需 sleep 确定性成立；429 短路不 recordFail 不延长窗口）；
// (c) Hello 携非法 ticket → auth_failed（窗口内节流命中与非法 ticket 同口径，
// D-10——事件行照出）。
// 恢复 stderr 后断言四类禁出串：base64(错凭据)、明文错密码、已签发 ticket 值、
// "authorization"（大小写不敏感）；正向对照：auth_failed/throttled 事件行确实
// 在捕获中（捕获有效性证明，防空捕获假绿）。
// 捕获复用 limits_test.go 的 captureStderr（os.Pipe 置换，进程全局不并行纪律）。
func TestLogRedaction(t *testing.T) {
	cred, err := server.ParseCredential("redact-user:redact-pass-9f2c")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:    true,
		Credentials: []server.Credential{cred},
	})
	url := attachURL(wsURL)

	restore := captureStderr(t)
	defer func() { _ = restore() }() // Fatal 路径兜底恢复（幂等，正常路径二次调用空过）

	// (0) 对照样本 ticket（凭据值本身不得出现在任何日志）。
	resp := postAttach(t, url, "redact-user", "redact-pass-9f2c", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("control ticket status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
		resp.Body.Close()
		t.Fatalf("decode control ticket: %v", err)
	}
	resp.Body.Close()
	ticket := issued.Ticket
	if ticket == "" {
		t.Fatal("control ticket empty")
	}
	// (a) 错凭据 → 401（fails=1，notBefore=+1s）。
	resp = postAttach(t, url, "redact-user", "redact-WRONG-9f2c", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-credential status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	// (b) 立即再错凭据 → 429（fail 后窗口内第 1 次请求即命中）。
	resp = postAttach(t, url, "redact-user", "redact-WRONG-9f2c", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("in-window status = %d, want %d (429)", resp.StatusCode, http.StatusTooManyRequests)
	}
	// (c) Hello 携非法 ticket → auth_failed（窗口内节流命中同口径，D-10）。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	dialHelloTicketWantAuthFailed(t, ctx, wsURL, "AAAAAAAAAAAAAAAAAAAAAA")
	cancel()

	out := restore()

	// 四类禁出串（SEC-01 红线；authorization 大小写不敏感）。
	b64Wrong := base64.StdEncoding.EncodeToString([]byte("redact-user:redact-WRONG-9f2c"))
	if strings.Contains(out, b64Wrong) {
		t.Errorf("stderr contains base64(credential) %q — 日志红线（ttyd server.c:142 反例）:\n%s", b64Wrong, out)
	}
	if strings.Contains(out, "redact-WRONG-9f2c") {
		t.Errorf("stderr contains plaintext password — 日志红线:\n%s", out)
	}
	if strings.Contains(out, ticket) {
		t.Errorf("stderr contains issued ticket value — 日志红线:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "authorization") {
		t.Errorf("stderr contains \"authorization\"（大小写不敏感）— 日志红线:\n%s", out)
	}
	// 正向对照：事件行确实被捕获（防空捕获假绿）。
	if !strings.Contains(out, proto.ErrAuthFailed) {
		t.Errorf("stderr missing auth_failed event line — 捕获失效或事件缺失:\n%s", out)
	}
	if !strings.Contains(out, "throttled") {
		t.Errorf("stderr missing throttled event line — 捕获失效或事件缺失:\n%s", out)
	}

	// (c) 后服务端关 conn 落入读循环，reader 终结 → D-11 收口。
	waitExit(t, exitCh, 0)
}

// ====== Task 3：429 闸 / D-08 共享计数器 / 双端点 Origin ======

// TestThrottleHTTP（SEC-03，D-08/D-09）：HTTP 层指数退避的完整生命周期——
// ThrottleBase=200ms 注入（节流语义锚点 03-01：fail#1 后 notBefore=+1×base，
// 窗口内第 2 次请求即 429——「×2→401 后 #3→429」编排必红，严禁按旧序列写）：
// #1 错凭据 → 401（fails=1，notBefore=+200ms）→ sleep 250ms 过窗 →
// #2 错凭据 → 401（fails=2，notBefore=+400ms）→ #3 立即正确凭据 → 429 +
// Retry-After ≥1（正确凭据也 429 证明节流闸在 Basic 之前；429 短路不
// recordFail、notBefore 不延长）→ sleep 450ms 过 400ms 窗 →
// 正确凭据 → 200（recordSuccess 清零生效）→ 紧接错凭据 → 401（级数从 base
// 重启，fail#1 仍可请求）→ sleep 250ms 过窗 → 末次正确凭据 → 200 收口
//（避开 +200ms 窗口防 429；清零防状态污染——每测试独立实例）。
// 本测试只验 HTTP 层：不 dial WS，exitf 不断言。
func TestThrottleHTTP(t *testing.T) {
	cred, err := server.ParseCredential("th-erin:throttle-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:     true,
		Credentials:  []server.Credential{cred},
		ThrottleBase: 200 * time.Millisecond,
	})
	url := attachURL(wsURL)
	post := func(user, pass string) *http.Response {
		t.Helper()
		resp := postAttach(t, url, user, pass, nil)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp
	}

	// #1 错凭据 → 401（fails=1，notBefore=+200ms）。
	if resp := post("th-erin", "wrong-1"); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("#1 status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	time.Sleep(250 * time.Millisecond) // 过 200ms 窗口
	// #2 错凭据 → 401（fails=2，notBefore=+400ms）。
	if resp := post("th-erin", "wrong-2"); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("#2 status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	// #3 立即正确凭据 → 429 + Retry-After ≥1（节流闸在 Basic 之前；短路不
	// recordFail、notBefore 不延长——窗口内第 1 次请求即 429）。
	resp := post("th-erin", "throttle-pass")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("#3 status = %d, want %d (429)", resp.StatusCode, http.StatusTooManyRequests)
	}
	ra, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || ra < 1 {
		t.Fatalf("Retry-After = %q, want ≥1 整数（ceil(剩余等待) 秒）", resp.Header.Get("Retry-After"))
	}
	time.Sleep(450 * time.Millisecond) // 过 400ms 窗口
	// 正确凭据 → 200（窗口过 + recordSuccess 清零）。
	if resp := post("th-erin", "throttle-pass"); resp.StatusCode != http.StatusOK {
		t.Fatalf("post-window status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
	// 紧接错凭据 → 401（清零后级数从 base 重启：fail#1 仍可请求，非 429）。
	if resp := post("th-erin", "wrong-3"); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-reset status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	time.Sleep(250 * time.Millisecond) // 过 fail#1 的 +200ms 窗口
	// 末次正确凭据 → 200 收口。
	if resp := post("th-erin", "throttle-pass"); resp.StatusCode != http.StatusOK {
		t.Fatalf("final status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
}

// TestThrottleHelloSharedCounter（D-08 统一计数器行为级反证）：HTTP 侧凭据失败
// 把 WS 侧 Hello 核销闸住——先取合法 ticket，再编排 #1 错凭据 401（fails=1，
// notBefore=+200ms）→ #2 立即错凭据 429（窗口内命中，不 recordFail 不延长
// 窗口——429 起于 fail 后的第 1 次窗口内请求）→ 立即 Hello 携合法 ticket →
// Error{auth_failed}+1008。
// 反证逻辑：无共享计数器时令牌闸不拦 WS 侧，合法 ticket 必核销成功收
// Welcome——被拒即 HTTP 失败与 Hello 核销计入同一 per-IP store 的直接证据。
// 走查点：节流命中短路在 redeem 之前，ticket 未核销（wire 上按 D-10 不可区分）。
func TestThrottleHelloSharedCounter(t *testing.T) {
	cred, err := server.ParseCredential("sc-frank:shared-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:     true,
		Credentials:  []server.Credential{cred},
		ThrottleBase: 200 * time.Millisecond,
	})
	url := attachURL(wsURL)

	// 正确凭据取合法 ticket（recordSuccess，计数器干净）。
	resp := postAttach(t, url, "sc-frank", "shared-pass", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("issue ticket status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
		resp.Body.Close()
		t.Fatalf("decode ticket response: %v", err)
	}
	resp.Body.Close()
	if issued.Ticket == "" {
		t.Fatal("ticket empty in 200 response")
	}

	// #1 错凭据 → 401（fails=1，notBefore=+200ms）。
	resp = postAttach(t, url, "sc-frank", "wrong-pass", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("#1 status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	// #2 立即错凭据 → 429（窗口内命中，不 recordFail 不延长窗口）。
	resp = postAttach(t, url, "sc-frank", "wrong-pass", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("#2 status = %d, want %d (429)", resp.StatusCode, http.StatusTooManyRequests)
	}

	// 立即 Hello 携合法 ticket → auth_failed + 1008（共享计数器闸住 WS 核销）。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dialHelloTicketWantAuthFailed(t, ctx, wsURL, issued.Ticket)

	// 服务端关 conn 后落入读循环，下一拍 reader 终结 → D-11 收口。
	waitExit(t, exitCh, 0)
}

// TestOriginEndpoints（SEC-04，D-12/D-13 双端点执行）：--origin 等效装配
//（Origins: ["https://portal.example"] 已规范化形态 + ThrottleBase:50ms——
// /api/attach 侧 g)→h) 为连续失败编排需 pacing）。全部场景均为 HTTP 层拒绝
//（400/403/401，零 attach）——无终结路径触发，WS 与 HTTP 场景共用一个实例，
// exitf 不断言（注释钉死，区别于单会话约束需独立实例的成功握手场景）。
//
// /ws 侧（⓪ 守卫 + 库 OriginPatterns 二次校验）：
// a) 无 Origin + 无子协议 → 400（过 ⓪ 到达 ①——无 Origin 放行证明，优先 400
// 路径：不与 ticket 核销/节流交互）；
// b) 邪恶源 https://evil.example + wesh.v1 子协议 → 403（⓪ 拒绝）；
// c) Origin: null + 子协议 → 403（沙箱 iframe 载体，规范化失败按拒绝）；
// d) Origin = 同源值（http://<r.Host>）→ 400（过 ⓪ 到达 ①，非 403 即放行证明）；
// e) 白名单 https://portal.example → 400（过 ⓪ 到达 ①，D-12 白名单放行）。
//
// /api/attach 侧（originMiddleware 在 basicAuth 之前）：
// f) 邪恶源 POST 错凭据 → 403（Origin 闸先拒，不 recordFail——g) 仍是 fail#1）；
// g) 无 Origin POST 错凭据 → 401（过 Origin 闸达 Basic；fails=1，notBefore=+50ms）；
// sleep 100ms 过窗；
// h) 白名单源 POST 错凭据 → 401（f)→g) 之间无需 sleep——403 不计数）。
func TestOriginEndpoints(t *testing.T) {
	cred, err := server.ParseCredential("or-grace:origin-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:     true,
		Credentials:  []server.Credential{cred},
		Origins:      []string{"https://portal.example"},
		ThrottleBase: 50 * time.Millisecond,
	})
	url := attachURL(wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// dialWantStatus 负例三段式（handshake_test.go 同款）：dial 失败 + resp 非 nil
	// + 状态码相等。
	dialWantStatus := func(origin string, subprotocols []string, want int, what string) {
		t.Helper()
		opts := &websocket.DialOptions{Subprotocols: subprotocols}
		if origin != "" {
			opts.HTTPHeader = http.Header{"Origin": []string{origin}}
		}
		_, resp, err := websocket.Dial(ctx, wsURL, opts)
		if err == nil {
			t.Fatalf("%s dial unexpectedly succeeded — Origin gate missing (SEC-04/D-13)", what)
		}
		if resp == nil {
			t.Fatalf("%s dial failed without HTTP response: %v", what, err)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s dial status = %d, want %d", what, resp.StatusCode, want)
		}
	}

	// /ws 侧五场景。
	dialWantStatus("", nil, http.StatusBadRequest, "no-origin")                                       // a) 过 ⓪ 达 ①
	dialWantStatus("https://evil.example", []string{proto.Subprotocol}, http.StatusForbidden, "evil") // b) ⓪ 拒绝
	dialWantStatus("null", []string{proto.Subprotocol}, http.StatusForbidden, "null-origin")          // c) null 拒绝
	sameOrigin := "http://" + strings.TrimSuffix(strings.TrimPrefix(wsURL, "ws://"), "/ws")
	dialWantStatus(sameOrigin, nil, http.StatusBadRequest, "same-origin")                    // d) 同源放行（非 403）
	dialWantStatus("https://portal.example", nil, http.StatusBadRequest, "whitelist-origin") // e) 白名单放行（非 403）

	// /api/attach 侧三场景。
	resp := postAttach(t, url, "or-grace", "wrong-pass", map[string]string{"Origin": "https://evil.example"})
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read 403 body: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("attach evil-origin status = %d, want %d (403)", resp.StatusCode, http.StatusForbidden)
	}
	if strings.Contains(string(body), "evil.example") {
		t.Fatalf("403 body echoes Origin value（反射面）: %q", body)
	}
	resp = postAttach(t, url, "or-grace", "wrong-pass", nil) // g) 无 Origin → 过 Origin 闸达 Basic（fail#1）
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("attach no-origin status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
	time.Sleep(100 * time.Millisecond) // 过窗（fail#1 窗口 = 50ms）
	resp = postAttach(t, url, "or-grace", "wrong-pass", map[string]string{"Origin": "https://portal.example"}) // h) 白名单源
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("attach whitelist-origin status = %d, want %d (401)", resp.StatusCode, http.StatusUnauthorized)
	}
}
