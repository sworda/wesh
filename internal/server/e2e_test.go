package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// TestEchoPTY（VALIDATION 1-01-06）：`wesh --writable -- /bin/cat` 下 WS 客户端发
// INPUT 帧应收同字节 OUTPUT 帧；随后断开客户端，断言 exitf 以退出码 0 被调用（D-11 触发面）。
// 02-02 起建连经 dialHello 过 wesh.v1 握手（writable 装配保持 echo 语义）。
func TestEchoPTY(t *testing.T) {
	sess, err := pty.Start([]string{"/bin/cat"})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh := make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, server.Options{Writable: true})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()
	go http.Serve(ln, srv.Handler())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, "ws://"+ln.Addr().String()+"/ws", 80, 24)

	payload := []byte("hello wesh")
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}

	// PTY 回显可能分块，累积至收齐
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}

	// 客户端断开 → D-11：SIGHUP 子进程进程组 + exitf(0)
	c.Close(websocket.StatusNormalClosure, "")
	select {
	case code := <-exitCh:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("exitf not called within 5s after client disconnect")
	}
}

// TestDrainBeforeAttach（D-12 行为证明）：无 WS 客户端 attach 时，输出超 64KiB PTY
// 内核缓冲的命令必须照常退出——若 ReadLoop 未自 New 启动 drain，子进程写满内核缓冲后
// 阻塞、永不退出，本测试超时即暴露接线缺失。
func TestDrainBeforeAttach(t *testing.T) {
	sess, err := pty.Start([]string{"seq", "1", "200000"}) // 约 1.3MB 输出
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh := make(chan int, 1)
	server.New(sess, func(code int) { exitCh <- code }, server.Options{}) // 不连接任何 WS 客户端（零值 Options 仅需编译适配 New 签名）

	select {
	case code := <-exitCh:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not exit within 5s — D-12 drain not wired")
	}
}

// ====== plan 01-03 增量：生命周期 e2e 四测（D-09/D-10/D-11 + 关闭码纪律）======

// startTestServerWith 复用 plan 01-01 的构造模式：sess + New(sess, exitf 捕获桩, opts)
// + 127.0.0.1:0 监听，返回 exitf 捕获通道与 /ws URL。统一收口各生命周期/握手测试的装配。
func startTestServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string) {
	t.Helper()
	sess, err := pty.Start(argv)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh = make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, opts)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go http.Serve(ln, srv.Handler())
	return exitCh, "ws://" + ln.Addr().String() + "/ws"
}

// startTestServer 兼容包装：Writable:true 装配，保持既有五个 Dial 测试的 echo 语义。
func startTestServer(t *testing.T, argv []string) (exitCh chan int, wsURL string) {
	t.Helper()
	return startTestServerWith(t, argv, server.Options{Writable: true})
}

// dialHello 统一收口握手（RESEARCH §Wave 0）：以 wesh.v1 子协议 Dial → 发 Hello 首帧
// → 读首帧断言为 Welcome 且 JSON 解码取 mode → 返回 (conn, mode)。
// cols/rows 参数化是签名硬要求——02-03 TestReadOnlyAllowsResize 以 (111, 44) 复用
// 同一签名验证 Hello 携尺寸生效，禁止硬编码 80x24；既有测试调用处统一传 (80, 24)。
func dialHello(t *testing.T, ctx context.Context, wsURL string, cols, rows int) (*websocket.Conn, string) {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	payload, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: cols, Rows: rows})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, payload...)); err != nil {
		t.Fatalf("write Hello: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read Welcome: %v", err)
	}
	if len(data) == 0 || data[0] != proto.Welcome {
		t.Fatalf("first frame = %v, want Welcome ('W')", data)
	}
	var wp proto.WelcomePayload
	if err := json.Unmarshal(data[1:], &wp); err != nil {
		t.Fatalf("decode Welcome: %v", err)
	}
	return c, wp.Mode
}

// waitExit 断言 exitf 在 5s 内以 want 码被调用（两条终结路径的收口断言点）。
func waitExit(t *testing.T, exitCh chan int, want int) {
	t.Helper()
	select {
	case code := <-exitCh:
		if code != want {
			t.Fatalf("exit code = %d, want %d", code, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("exitf not called within 5s (want code %d)", want)
	}
}

// ====== plan 03-03 增量：认证集成 helper（Phase 3 集成组复用，同包 server_test）======

// dialHelloTicket 是 dialHello 的 ticket 变体：逐字复制其形态，HelloPayload 加
// Ticket 字段（03-03 起认证模式建连形态）。dialHello 本体不动（既有调用零漂移）。
func dialHelloTicket(t *testing.T, ctx context.Context, wsURL string, ticket string, cols, rows int) (*websocket.Conn, string) {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	payload, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: cols, Rows: rows, Ticket: ticket})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, payload...)); err != nil {
		t.Fatalf("write Hello: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read Welcome: %v", err)
	}
	if len(data) == 0 || data[0] != proto.Welcome {
		t.Fatalf("first frame = %v, want Welcome ('W')", data)
	}
	var wp proto.WelcomePayload
	if err := json.Unmarshal(data[1:], &wp); err != nil {
		t.Fatalf("decode Welcome: %v", err)
	}
	return c, wp.Mode
}

// attachURL 把 startTestServerWith 返回的 /ws URL 映射为 /api/attach 的 http URL
//（ws:// → http://  scheme 替换 + /ws → /api/attach 路径替换）。
func attachURL(wsURL string) string {
	return strings.TrimSuffix(strings.Replace(wsURL, "ws://", "http://", 1), "/ws") + "/api/attach"
}

// postAttach 发 POST /api/attach 并返回响应（调用方负责读取并 Close body）。
// user/pass 均为空串时跳过 SetBasicAuth（无凭据负例开关）；headers 逐对注入
//（Origin 等场景头）。请求体恒空（D-11），大 body 负例由调用方自建请求。
func postAttach(t *testing.T, url, user, pass string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("new POST request: %v", err)
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// helperArgv 构造 TestHelperProcess 子进程 argv。
// 守卫走 argv 标记而非 env 变量（钉死）：spawn 路径 cmd.Env = whitelistEnv() 为替换式
// 注入，自定义守卫变量（如 GO_WANT_HELPER_PROCESS）不在 SEC-06 白名单内会被剥离——
// helper 收不到 env 守卫将直接 return、以退出码 0 空过（≠42 必红）；argv 则经 spawn
// 原样透传（D-02）必然到达。
func helperArgv(t *testing.T, marker string, extra ...string) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return append([]string{exe, "-test.run=TestHelperProcess", "--", marker}, extra...)
}

// TestHelperProcess 是 spawn 出去的"子进程演员"（Go 惯例 TestHelperProcess 模式，
// argv 守卫变体见 helperArgv 注释）。正常 `go test` 运行时 os.Args 无 wesh-helper-
// 标记，直接 return（空过恒绿）；被 spawn 时按 `--` 后标记分派到对应分支。
func TestHelperProcess(t *testing.T) {
	marker := ""
	var args []string
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			marker, args = os.Args[i+1], os.Args[i+2:]
			break
		}
	}
	if !strings.HasPrefix(marker, "wesh-helper-") {
		return
	}
	switch marker {
	case "wesh-helper-exit42":
		// 读到首个 '\n' 再以 42 自杀：保证 D-10 的 1000 关闭帧发出时仍有已 attach
		// 的接收方，消除"子进程抢在 Dial 完成前退出"的竞态。PTY 规范模式行缓冲，
		// 客户端须发 '\n' 才送达。
		b := make([]byte, 1)
		for {
			if _, err := os.Stdin.Read(b); err != nil {
				os.Exit(3) // 哨兵：输入路径异常，≠42 使测试红
			}
			if b[0] == '\n' {
				break
			}
		}
		os.Exit(42)
	case "wesh-helper-sighup":
		// 安装 SIGHUP 处理器后先报 READY（经 PTY → WS 通知测试端可断开），
		// 消除"信号先于处理器就位"的竞态。
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGHUP)
		fmt.Println("READY")
		select {
		case <-sigCh:
			// 标记落盘而非 stdout：WS 断开后 server 侧 onChunk 丢弃输出（conn 已清），
			// stdout 标记对测试不可观测；落盘文件是跨平台可断言的送达证据。
			if len(args) > 0 {
				_ = os.WriteFile(args[0], []byte("GOT_SIGHUP\n"), 0o600)
			}
			os.Exit(0)
		case <-time.After(15 * time.Second):
			os.Exit(4) // 哨兵：未收到 SIGHUP，D-11 信号路径失效
		}
	}
}

// TestSecondClient409（D-09）：第二个 WS 连接 attach 请求在 Accept 之前收到 HTTP 409；
// 已 attach 的第一条连接不受影响（Phase 1 单客户端临时语义，T-03-01 缓解固化）。
// 02-02 起第一连接经 dialHello 完成握手；第二次 Dial 同样携带 wesh.v1 子协议——
// 否则会被子协议预检以 400 拦截而非到达 409（负例语义漂移防空断言）。
func TestSecondClient409(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c1, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 第二次握手必须在 Accept 之前被拒（HTTP 409，不消耗 PTY/WS 资源）
	_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err == nil {
		t.Fatal("second dial unexpectedly succeeded — D-09 violated")
	}
	if resp == nil {
		t.Fatalf("second dial failed without HTTP response: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second dial status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	// 第一连接仍然存活可 echo（409 不干扰已 attach 会话）
	payload := []byte("still alive")
	if err := c1.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT on first conn: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c1.Read(ctx)
		if err != nil {
			t.Fatalf("first conn broken after 409: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}

	// 清理：关闭第一连接触发 D-11 → SIGHUP cat + exitf(0)
	c1.Close(websocket.StatusNormalClosure, "")
	waitExit(t, exitCh, 0)
}

// TestExitCodePropagation（D-10）：子进程以 42 退出时，已 attach 的 WS 客户端先收到
// 1000 正常关闭帧，随后 exitf 被以 42 调用（退出码传递，T-03-03 缓解固化）。
func TestExitCodePropagation(t *testing.T) {
	exitCh, wsURL := startTestServer(t, helperArgv(t, "wesh-helper-exit42"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	// 触发 helper 自杀：helper 读到 '\n' 后 os.Exit(42)
	if err := c.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}

	// 读到关闭帧为止（途中 PTY 回显等 OUTPUT 帧丢弃）
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
	// 1000 关闭帧先于 exitf（lifecycle：先关客户端再 terminate）——此后 exitf 必以 42 收口
	waitExit(t, exitCh, 42)
}

// TestUnknownFrame1002（D-16 关闭码纪律）：未知类型帧（'9'）导致 WS 以 1002 关闭；
// 全程不出现 1006（1006 永不写入，RFC6455 §7.4 / PITFALLS C9，T-03-02 缓解固化）。
func TestUnknownFrame1002(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	if err := c.Write(ctx, websocket.MessageBinary, []byte{'9', 'x'}); err != nil {
		t.Fatalf("write unknown frame: %v", err)
	}

	var ce websocket.CloseError
	for {
		if _, _, rerr := c.Read(ctx); rerr != nil {
			if !errors.As(rerr, &ce) {
				t.Fatalf("read terminated without CloseError: %v", rerr)
			}
			break
		}
	}
	if ce.Code == websocket.StatusAbnormalClosure {
		t.Fatal("close code 1006 observed — must never be written (RFC6455 §7.4, PITFALLS C9)")
	}
	if ce.Code != websocket.StatusProtocolError {
		t.Fatalf("close code = %d, want %d (1002)", ce.Code, websocket.StatusProtocolError)
	}

	// 服务端 reader 随关闭握手终结 → D-11 路径 SIGHUP cat + exitf(0)
	waitExit(t, exitCh, 0)
}

// TestClientDisconnectSIGHUP（D-11）：WS 客户端断开后子进程进程组收到 SIGHUP
// 且 exitf 被以 0 调用（T-03-03 缓解固化）。helper 收 SIGHUP 后落盘标记作送达证据。
func TestClientDisconnectSIGHUP(t *testing.T) {
	markerFile := filepath.Join(t.TempDir(), "sighup-marker")
	exitCh, wsURL := startTestServer(t, helperArgv(t, "wesh-helper-sighup", markerFile))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	// 等 helper 就绪（SIGHUP 处理器已安装）——READY 经 PTY → OUTPUT 帧到达
	ready := make([]byte, 0, 64)
	for !strings.Contains(string(ready), "READY") {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read before READY: %v (got %q so far)", err, ready)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		ready = append(ready, data[1:]...)
	}

	// 主动断开（CloseNow 不发关闭帧，覆盖网络断开路径）→ D-11：SIGHUP 进程组 + exitf(0)
	c.CloseNow()
	waitExit(t, exitCh, 0)

	// SIGHUP 送达证据：helper 收信号后落盘标记（exitf 先于 helper 信号处理，须轮询）
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, rerr := os.ReadFile(markerFile); rerr == nil && strings.Contains(string(b), "GOT_SIGHUP") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SIGHUP marker not observed within 5s — D-11 signal path broken")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ====== plan 02-02 增量：握手 tracer 端到端 ======

// TestHelloWelcome（02-02 tracer）：Hello→Welcome 握手端到端——Welcome mode 与服务端
// writable 配置一致（D-14）；rw 半侧补 INPUT echo 全链路断言（沿用 TestEchoPTY 累积模式）。
// 两个半侧各自独立装配（ro/rw 由 New 装配期固化），均以客户端正常关闭 + waitExit(0) 收口。
func TestHelloWelcome(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ro 半侧：默认只读（D-14 Welcome{mode:"ro"}）
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: false})
	c, mode := dialHello(t, ctx, wsURL, 80, 24)
	if mode != proto.ModeRO {
		t.Fatalf("welcome mode = %q, want %q (writable=false)", mode, proto.ModeRO)
	}
	c.Close(websocket.StatusNormalClosure, "")
	waitExit(t, exitCh, 0) // D-11：客户端断开收口

	// rw 半侧：--writable 等价装配（D-15）→ Welcome{mode:"rw"} + INPUT 正常 echo
	exitChRW, wsURLRW := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
	cRW, modeRW := dialHello(t, ctx, wsURLRW, 80, 24)
	if modeRW != proto.ModeRW {
		t.Fatalf("welcome mode = %q, want %q (writable=true)", modeRW, proto.ModeRW)
	}
	payload := []byte("hello wesh")
	if err := cRW.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := cRW.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}
	cRW.Close(websocket.StatusNormalClosure, "")
	waitExit(t, exitChRW, 0)
}
