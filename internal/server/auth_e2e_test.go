package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
