package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// ====== plan 02-03 增量：SEC-08 守卫链与握手违规测试组 ======
//
// 测试装配统一经 e2e_test.go 的 startTestServerWith/startTestServer/dialHello/waitExit
// 收口（同包直接复用）；超时护栏一律 10s ctx。

// TestHalfOpenPerIP429（SEC-08/D-04）：per-IP 半开连接上限闸——MaxHalfOpenPerIP=1 注入时，
// c1 半开（Accept 后不发 Hello）占住唯一名额，c2 在 Accept 前收到 HTTP 429；
// c1 随后补发 Hello 正常完成握手（429 不误伤在先连接——acquire/release 恰好一次的间接证明，
// 若 c1 的半开名额泄漏或 409 拒绝残留计数，后续握手必被 429 误杀）。
func TestHalfOpenPerIP429(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, MaxHalfOpenPerIP: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// c1：Dial（带 wesh.v1）成功但不发 Hello——占住唯一半开名额。
	// Dial 返回即 101 已收，服务端 handler 顺序执行 acquire→Accept，无时序竞态。
	c1, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("c1 dial: %v", err)
	}

	// c2：同 IP 第二条半开连接 → Accept 前 HTTP 429（零 WS 资源分配）。
	_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err == nil {
		t.Fatal("c2 dial unexpectedly succeeded — per-IP half-open gate missing (D-04)")
	}
	if resp == nil {
		t.Fatalf("c2 dial failed without HTTP response: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("c2 dial status = %d, want %d (429)", resp.StatusCode, http.StatusTooManyRequests)
	}

	// c1 补发 Hello → 收 Welcome：429 未误伤在先连接（名额在 Hello 完成时才释放，
	// 但 acquire 已在手——release 恰好一次，不双重释放导致后续误判）。
	hello, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c1.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, hello...)); err != nil {
		t.Fatalf("c1 write Hello: %v", err)
	}
	_, data, err := c1.Read(ctx)
	if err != nil {
		t.Fatalf("c1 read Welcome: %v", err)
	}
	if len(data) == 0 || data[0] != proto.Welcome {
		t.Fatalf("c1 first frame = %v, want Welcome ('W')", data)
	}

	// INPUT echo 一句确认全链路（writable 装配）。
	payload := []byte("half-open survivor")
	if err := c1.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("c1 write INPUT: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c1.Read(ctx)
		if err != nil {
			t.Fatalf("c1 read OUTPUT: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}

	// 清理：正常关闭 → 多客户端推论：detach 不触发 exitf（静默反证）。
	c1.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestSubprotocolRequired（SEC-08/D-03）：子协议双闸之 HTTP 预检——无子协议与错子协议
// 的握手在 Accept 前收到 HTTP 400（零 WS 资源分配，扫描器/旧客户端最早被拦）；
// 多值头含 wesh.v1 正常放行（Pitfall 5 回归：token 拆分语义，整头匹配会误拒）。
func TestSubprotocolRequired(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// (a)(b) 负例共用断言：Dial 失败且带 HTTP 响应，状态码为 want。
	dialWantStatus := func(opts *websocket.DialOptions, want int, what string) {
		t.Helper()
		_, resp, err := websocket.Dial(ctx, wsURL, opts)
		if err == nil {
			t.Fatalf("%s dial unexpectedly succeeded — subprotocol gate missing (D-03)", what)
		}
		if resp == nil {
			t.Fatalf("%s dial failed without HTTP response: %v", what, err)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s dial status = %d, want %d", what, resp.StatusCode, want)
		}
	}

	// (a) 不带 Subprotocols → 400
	dialWantStatus(nil, http.StatusBadRequest, "no-subprotocol")
	// (b) 错子协议 wesh.v2 → 400
	dialWantStatus(&websocket.DialOptions{Subprotocols: []string{"wesh.v2"}}, http.StatusBadRequest, "wrong-subprotocol")

	// (c) 多值头 "other, wesh.v1" → token 命中放行，协商结果为 wesh.v1。
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{"other", proto.Subprotocol}})
	if err != nil {
		t.Fatalf("multi-value subprotocol dial: %v", err)
	}
	if sp := c.Subprotocol(); sp != proto.Subprotocol {
		t.Fatalf("negotiated subprotocol = %q, want %q", sp, proto.Subprotocol)
	}

	// 收口：(a)(b) 未建连不触发终结；(c) 直接 Close——服务端预认证首读随关闭帧
	// 终结，多客户端推论下不触发 exitf（半开名额经 defer release 兜底释放）。
	c.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestHelloTimeout（SEC-08/D-04）：5s 未认证超时的测试注入形态（HelloTimeout=200ms）——
// Dial 后静默不发 Hello 的连接收到 1008 且 close reason 为 hello_timeout 机器串（D-07）。
func TestHelloTimeout(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, HelloTimeout: 200 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// 静默等待 hello_timeout。容忍窗 2s（注入 200ms 的 10 倍余量）：超时未收到关闭
	// 即测试失败——rctx 到期库内 AfterFunc 杀客户端连接，Read 返回非 CloseError。
	rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rcancel()
	var ce websocket.CloseError
	for {
		if _, _, rerr := c.Read(rctx); rerr != nil {
			if !errors.As(rerr, &ce) {
				t.Fatalf("read terminated without CloseError within 2s window: %v", rerr)
			}
			break
		}
	}
	if ce.Code != websocket.StatusPolicyViolation {
		t.Fatalf("close code = %d, want %d (1008)", ce.Code, websocket.StatusPolicyViolation)
	}
	if ce.Reason != "hello_timeout" {
		t.Fatalf("close reason = %q, want %q", ce.Reason, "hello_timeout")
	}

	// 服务端 reader 随 hello_timeout 关闭终结 → 多客户端推论：不触发 exitf。
	assertNoExit(t, exitCh)
}

// TestPrematureFrame（D-04/D-06）：抢跑帧——Hello 前直接发 INPUT 帧，服务端 1002 直关；
// 攻击面零反馈：全程不得收到任何数据帧（第一个读事件即关闭），尤其无 'E' Error 帧。
func TestPrematureFrame(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// 不发 Hello，直接发 INPUT（抢跑）。
	if err := c.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x'}); err != nil {
		t.Fatalf("write premature INPUT: %v", err)
	}

	gotData := false
	var ce websocket.CloseError
	for {
		_, data, rerr := c.Read(ctx)
		if rerr != nil {
			if !errors.As(rerr, &ce) {
				t.Fatalf("read terminated without CloseError: %v", rerr)
			}
			break
		}
		gotData = true
		if len(data) > 0 && data[0] == proto.Error {
			t.Fatal("received Error frame on attack path — D-06 zero-feedback violated")
		}
	}
	if gotData {
		t.Fatal("received data frame before close — attack path must close with zero feedback (D-06)")
	}
	if ce.Code != websocket.StatusProtocolError {
		t.Fatalf("close code = %d, want %d (1002)", ce.Code, websocket.StatusProtocolError)
	}

	// 服务端关 conn 后落入读循环，下一拍 reader 终结 → 多客户端推论：不触发 exitf。
	assertNoExit(t, exitCh)
}

// TestVersionMismatch（D-06/D-07）：Hello.version 不符的正常客户端路径——先收 'E'
// Error 帧（code=version_mismatch 且 message 非空）再收 1008，Error code 与 close
// reason 两处机器串同名（抓包/devtools 可辨）。
func TestVersionMismatch(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	hello, err := json.Marshal(proto.HelloPayload{Version: "wesh.v9", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, hello...)); err != nil {
		t.Fatalf("write Hello: %v", err)
	}

	// 首读：'E' Error 帧。
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
	if ep.Code != proto.ErrVersionMismatch {
		t.Fatalf("error code = %q, want %q", ep.Code, proto.ErrVersionMismatch)
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
	if ce.Reason != proto.ErrVersionMismatch {
		t.Fatalf("close reason = %q, want %q (same machine string as error code, D-07)", ce.Reason, proto.ErrVersionMismatch)
	}

	// 服务端关 conn 后落入读循环，下一拍 reader 终结 → 多客户端推论：不触发 exitf。
	assertNoExit(t, exitCh)
}

// TestReadOnlyDropsInput（CORE-04/D-13，Pitfall 7 服务端边界回归）：ro 模式下裸 WS
// 客户端发 INPUT 不进 PTY（/bin/cat 无启动 banner，200ms 静默窗口内无任何读事件即
// 服务端丢弃证明）且连接存活（随后 RESIZE 写通、正常关闭握手完成）。
//
// 窗口实现硬约束：客户端 Read 禁止携带 deadline ctx（Pitfall 2 回归锁——deadline
// 到期库内 AfterFunc 直接 c.close() 关整条连接且无关闭帧，conn.go:188-199，随后的
// RESIZE 存活断言写已关闭连接必败）；窗口以 goroutine 跑 c.Read(context.Background())
// 结果入缓冲 channel、select 对 time.After(200ms) 竞速实现。
func TestReadOnlyDropsInput(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: false})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 80, 24)
	if mode != proto.ModeRO {
		t.Fatalf("welcome mode = %q, want %q (writable=false)", mode, proto.ModeRO)
	}
	defer c.CloseNow() // 兜底：失败路径下解开挂起的读 goroutine（幂等，成功路径无害）

	// ro 服务端边界：INPUT 应被静默丢弃（/bin/cat 无 banner，窗口内静默即证明）。
	if err := c.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x'}); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}
	type readResult struct {
		data []byte
		err  error
	}
	readCh := make(chan readResult, 1) // 带缓冲：读 goroutine 永不阻塞于发送，无泄漏
	go func() {
		_, data, err := c.Read(context.Background())
		readCh <- readResult{data, err}
	}()
	select {
	case r := <-readCh:
		t.Fatalf("silence window violated: read returned data=%q err=%v — INPUT not dropped or connection killed", r.data, r.err)
	case <-time.After(200 * time.Millisecond):
		// 静默证明成立：窗口内无任何读事件（读到数据=INPUT 未被丢弃；读到错误=连接被误关）
	}

	// 连接存活断言：ro 下 RESIZE 被静默忽略（05-04 D-09 第二闸，修订 P2 D-13
	// 放行语义）——忽略 ≠ 关连接，写路径不报错。
	resize, err := json.Marshal(struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	}{Cols: 90, Rows: 30})
	if err != nil {
		t.Fatalf("marshal RESIZE: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Resize}, resize...)); err != nil {
		t.Fatalf("write RESIZE: %v", err)
	}

	// 正常关闭握手：写关闭帧成功即连接存活证据。服务端库收关闭帧自动回显
	//（read.go:358-361），读 goroutine 以 CloseError 1000 收场。
	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close handshake: %v", err)
	}
	select {
	case r := <-readCh:
		var ce websocket.CloseError
		if !errors.As(r.err, &ce) || ce.Code != websocket.StatusNormalClosure {
			t.Fatalf("read goroutine ended with data=%q err=%v, want CloseError 1000", r.data, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read goroutine did not terminate after close handshake")
	}
	// 多客户端推论：客户端断开不触发 exitf（静默反证）。
	assertNoExit(t, exitCh)
}

// TestReadOnlyAllowsResize（CORE-04/D-13，05-04 起经 D-09 修订）：ro 下 Hello 携
// (111,44) 生效（80x24 首帧窗口消除——纯 ro 会话全部 ro 端 Hello 首尺寸参与仲裁，
// D-09 矩阵第三行，否则会话冻结 80x24）；运行期 RESIZE(120,50) 自 05-04 起被服务端
// 直接忽略（D-09 第二闸——P2 D-13『ro 放行 RESIZE』为单客户端语境，多客户端下 ro
// 尺寸不参与仲裁），第二个 stty 保持 "44 111" 不跟随。
//
// argv 为测试编排夹具（非产品 spawn 路径）：前导 sleep 0.5 是硬要求——sh 随 spawn
// 立即执行，attach 前 PTY 输出被 onChunk drain 丢弃（server.go 现状），无前导休眠则
// stty#1 输出丢失、stty#2 后 sh 退出以 1000 关闭，第二个尺寸断言永不可达；0.5s 窗口
// 保证 dialHello 完成握手后两次 stty 才执行（本地回环握手毫秒级，竞态余量充足）。
func TestReadOnlyAllowsResize(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "sleep 0.5; stty size; sleep 1; stty size"}
	exitCh, wsURL := startTestServerWith(t, argv, server.Options{Writable: false})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 111, 44)
	if mode != proto.ModeRO {
		t.Fatalf("welcome mode = %q, want %q (writable=false)", mode, proto.ModeRO)
	}

	// readSize 累积 OUTPUT 帧直至负载按 strings.Fields 切分出至少 2 个字段，
	// 返回 (rows, cols)。PTY 输出可能分块到达；Fields 切分免疫 ONLCR 注入的 \r
	//（Phase 1 决策沿用）。两次 stty 间隔 1s，输出不会合并。
	readSize := func() (string, string) {
		t.Helper()
		buf := make([]byte, 0, 64)
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("read stty output: %v (got %q so far)", err, buf)
			}
			if len(data) == 0 || data[0] != proto.Output {
				t.Fatalf("unexpected frame: %v", data)
			}
			buf = append(buf, data[1:]...)
			if fields := strings.Fields(string(buf)); len(fields) >= 2 {
				return fields[0], fields[1]
			}
		}
	}

	// 首个 stty：Hello 携 (cols=111, rows=44) 生效。
	rows, cols := readSize()
	if rows != "44" || cols != "111" {
		t.Fatalf("first stty size = %q %q, want \"44 111\" (Hello-carried size)", rows, cols)
	}

	// ro 下发 RESIZE {cols:120, rows:50} → 05-04 起 D-09 第二闸忽略（修订 P2 D-13
	// 放行语义：多客户端下 ro 尺寸不参与仲裁；纯 ro 会话运行期缩放不上报，运行期
	// 窗口裁剪行为推论见 RESEARCH Pattern 4/A3，README 明示归 05-09）。
	resize, err := json.Marshal(struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	}{Cols: 120, Rows: 50})
	if err != nil {
		t.Fatalf("marshal RESIZE: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Resize}, resize...)); err != nil {
		t.Fatalf("write RESIZE: %v", err)
	}

	// 第二个 stty：尺寸不跟随（D-09 第二闸忽略证据——RESIZE 已送达服务端，
	// 若被放行 stty 必读出 "50 120"）。
	rows, cols = readSize()
	if rows != "44" || cols != "111" {
		t.Fatalf("second stty size = %q %q, want \"44 111\" (ro RESIZE ignored since 05-04, D-09 second gate)", rows, cols)
	}

	// sh 随夹具结束退出 → D-10：lifecycle 发 1000 正常关闭帧（客户端读取循环
	// 使库自动回显关闭帧，服务端 Close 握手不等超时），exitf 以 0 收口。
	var ce websocket.CloseError
	for {
		if _, _, rerr := c.Read(ctx); rerr != nil {
			if !errors.As(rerr, &ce) {
				t.Fatalf("read terminated without CloseError: %v", rerr)
			}
			break
		}
	}
	if ce.Code != websocket.StatusNormalClosure {
		t.Fatalf("close code = %d, want %d (1000)", ce.Code, websocket.StatusNormalClosure)
	}
	waitExit(t, exitCh, 0)
}
