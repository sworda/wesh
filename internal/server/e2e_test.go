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
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// TestEchoPTY（VALIDATION 1-01-06）：`wesh --writable -- /bin/cat` 下 WS 客户端发
// INPUT 帧应收同字节 OUTPUT 帧；随后断开客户端，断言不触发 exitf 且服务端存活
// 可再 attach echo（多客户端推论：P1 D-11 单次语义终结，断开 = 注册表移除）。
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

	// 客户端断开 → 多客户端推论：注册表移除，不触发 exitf（200ms 静默反证）。
	c.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)

	// 服务端存活断言：同实例再 attach 成功且 echo 一致（断开即移除的行为化证明）。
	c2, _ := dialHello(t, ctx, "ws://"+ln.Addr().String()+"/ws", 80, 24)
	payload2 := []byte("still alive after detach")
	if err := c2.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload2...)); err != nil {
		t.Fatalf("write INPUT after re-attach: %v", err)
	}
	got2 := make([]byte, 0, len(payload2))
	for len(got2) < len(payload2) {
		_, data, err := c2.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT after re-attach: %v (got %q so far)", err, got2)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got2 = append(got2, data[1:]...)
	}
	if string(got2) != string(payload2) {
		t.Fatalf("echo payload after re-attach = %q, want %q", got2, payload2)
	}
	c2.Close(websocket.StatusNormalClosure, "")
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

// startTrackedServerWith 是 startTestServerWith 的 handler 追踪变体：返回的
// waitHandlers 阻塞至全部在途 HTTP handler（含 /ws 的 Attach goroutine）返回。
// stderr 捕获类测试在 restore() 前调用——logEvent 读 os.Stderr 必然先于其 handler
// 返回（goroutine 程序序），wg.Wait 与 handler 完成建立 WaitGroup happens-before
// 边，restore() 写 os.Stderr 由此同步（多客户端推论后 exitf 不再随客户端断开
// 触发，waitExit 提供的 channel 同步边消亡，本变体是 race detector 认可的替代
// 同步形态）。
func startTrackedServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, waitHandlers func()) {
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
	var wg sync.WaitGroup
	h := srv.Handler()
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()
		h.ServeHTTP(w, r)
	}))
	return exitCh, "ws://" + ln.Addr().String() + "/ws", wg.Wait
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

// assertNoExit 断言 exitf 在 200ms 内未被调用（多客户端必然推论：任何客户端断开
// 不再触发终结路径——P1 D-11 单次语义终结；exitf 唯一触发源是子进程退出，D-10）。
// 200ms 窗口同时是服务端读循环完成 detach/stderr 事件写出的充分余量。
func assertNoExit(t *testing.T, exitCh chan int) {
	t.Helper()
	select {
	case code := <-exitCh:
		t.Fatalf("exitf called with code %d — client disconnect must not terminate server (multi-client corollary)", code)
	case <-time.After(200 * time.Millisecond):
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

// dialHelloPayload 是 dialHello 的载荷变体（P4 D-13）：握手形态相同，但返回完整
// Welcome 载荷 map——供 prefs 键级断言（dialHello 只解码 mode 不够）。dialHello
// 本体不动（既有调用零漂移）。
func dialHelloPayload(t *testing.T, ctx context.Context, wsURL string, cols, rows int) (*websocket.Conn, map[string]any) {
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
	var wm map[string]any
	if err := json.Unmarshal(data[1:], &wm); err != nil {
		t.Fatalf("decode Welcome: %v", err)
	}
	return c, wm
}

// attachURL 把 startTestServerWith 返回的 /ws URL 映射为 /api/attach 的 http URL
// （ws:// → http://  scheme 替换 + /ws → /api/attach 路径替换）。
func attachURL(wsURL string) string {
	return strings.TrimSuffix(strings.Replace(wsURL, "ws://", "http://", 1), "/ws") + "/api/attach"
}

// postAttach 发 POST /api/attach 并返回响应（调用方负责读取并 Close body）。
// user/pass 均为空串时跳过 SetBasicAuth（无凭据负例开关）；headers 逐对注入
// （Origin 等场景头）。请求体恒空（D-11），大 body 负例由调用方自建请求。
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
	marker, markerArg := "", ""
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			marker = os.Args[i+1]
			if i+2 < len(os.Args) {
				markerArg = os.Args[i+2]
			}
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
	case "wesh-helper-winch":
		// D-11 SIGWINCH 送达证据（TestSigwinchOnAttach 的演员）：同步纪律——先从
		// stdin 读一字节（测试经 INPUT 帧发送，规范模式行缓冲须携 '\n'）再装
		// SIGWINCH 处理器并报 READY：attach 完成时服务端的显式 SIGWINCH 若先于
		// 处理器安装会被默认忽略（丢失即测试红），READY 回读确认后再由第二
		// 客户端 attach 触发第二次信号，消除安装竞态。收到信号落盘标记文件
		//（markerArg 为路径）——落盘标记是信号送达证据的既定纪律：stdout 标记在
		// WS 断开后不可观测（Phase 01-03 决策）。
		if markerArg == "" {
			os.Exit(3)
		}
		b := make([]byte, 1)
		for {
			if _, err := os.Stdin.Read(b); err != nil {
				os.Exit(3)
			}
			if b[0] == '\n' {
				break
			}
		}
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGWINCH)
		fmt.Println("READY")
		<-sigCh
		if err := os.WriteFile(markerArg, []byte("GOT_WINCH\n"), 0o644); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
}

// TestSecondClientAttach（MULTI-01 主干；原 TestSecondClient409 的多客户端化替换）：
// 第二个 WS 客户端 attach 同一服务端成功（409 单客户端原子门已随注册表拆除），
// 两端各自实时收到同一 OUTPUT 字节流且累积 payload 逐字节一致。
// 第二 Dial 同样携带 wesh.v1 子协议（守卫链 ① 形态不变）。
// 第三人 503 断言不在此做——max-clients 闸 05-07 才落地（该 plan 的 TestMaxClients503 覆盖）。
func TestSecondClientAttach(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c1, _ := dialHello(t, ctx, wsURL, 80, 24)
	c2, _ := dialHello(t, ctx, wsURL, 80, 24) // 双 attach 成功（原 409 语义终结）

	// c1 发 INPUT，行规程回显经 hub 组一次共享帧扇出 → c1/c2 两端各自累积收齐。
	payload := []byte("dual attach echo")
	if err := c1.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT on c1: %v", err)
	}
	accum := func(c *websocket.Conn) []byte {
		t.Helper()
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
		return got
	}
	got1 := accum(c1)
	got2 := accum(c2)
	if string(got1) != string(payload) || string(got2) != string(payload) {
		t.Fatalf("fan-out payload = %q / %q, want both %q", got1, got2, payload)
	}
	if string(got1) != string(got2) {
		t.Fatalf("fan-out streams differ (byte-identical violated): %q vs %q", got1, got2)
	}

	// 清理：双端关闭不触发 exitf（多客户端推论，静默反证）。
	c1.Close(websocket.StatusNormalClosure, "")
	c2.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestExitCodePropagation（D-10 唯一终结路径）：子进程以 42 退出时，lifecycle
// 广播 1000 正常关闭帧到全部已 attach 客户端（本例单客户端），随后 exitf 被以
// 42 调用（退出码传递语义不变，T-03-03 缓解固化）。
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
	// 1000 广播关闭帧先于 exitf（lifecycle：先并行关闭全部客户端再 terminate）——
	// 此后 exitf 必以 42 收口
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

	// 服务端 reader 随关闭握手终结 → 多客户端推论：detach 不触发 exitf（静默反证）
	// + 会话存活断言（同实例再 attach 成功并 echo 一致，同 TestEchoPTY 新形态）。
	assertNoExit(t, exitCh)
	c2, _ := dialHello(t, ctx, wsURL, 80, 24)
	payload := []byte("alive after 1002")
	if err := c2.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT after re-attach: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c2.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT after re-attach: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload after re-attach = %q, want %q", got, payload)
	}
	c2.Close(websocket.StatusNormalClosure, "")
}

// ====== plan 02-02 增量：握手 tracer 端到端 ======

// TestHelloWelcome（02-02 tracer）：Hello→Welcome 握手端到端——Welcome mode 与服务端
// writable 配置一致（D-14）；rw 半侧补 INPUT echo 全链路断言（沿用 TestEchoPTY 累积模式）。
// 两个半侧各自独立装配（ro/rw 由 New 装配期固化），均以客户端正常关闭 + exitCh 静默
// 反证 + 断开即移除（再 attach 成功）收口（多客户端推论形态）。
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
	assertNoExit(t, exitCh) // 多客户端推论：客户端断开不触发 exitf
	// 断开即移除的行为化：再 attach 成功且 mode 判定不变
	c2, mode2 := dialHello(t, ctx, wsURL, 80, 24)
	if mode2 != proto.ModeRO {
		t.Fatalf("re-attach welcome mode = %q, want %q", mode2, proto.ModeRO)
	}
	c2.Close(websocket.StatusNormalClosure, "")

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
	assertNoExit(t, exitChRW)
	// 断开即移除的行为化：再 attach 成功且 mode 判定不变
	cRW2, modeRW2 := dialHello(t, ctx, wsURLRW, 80, 24)
	if modeRW2 != proto.ModeRW {
		t.Fatalf("re-attach welcome mode = %q, want %q", modeRW2, proto.ModeRW)
	}
	cRW2.Close(websocket.StatusNormalClosure, "")
}

// ====== plan 04-01 增量：Welcome prefs 端到端 ======

// TestWelcomePrefs（P4 D-13 端到端两半侧）：Options.ClientPrefsRO/ClientPrefsRW
// 注入时握手收到的 Welcome JSON 含 prefs 键且逐键值相等；未注入（零值默认装配）
// 时无 prefs 键——omitempty 缺席回归，旧前端零漂移（P2 D-02 加字段纪律）。
// 两半侧各自独立装配，均以客户端正常关闭 + exitCh 静默反证 + 再 attach 成功收口
//（多客户端推论形态）。
// 05-03 适配：ClientPrefs 单字段分裂为 ro/rw 双档（D-13）——本测试锁定的语义是
//「注入 blob → Welcome 逐键透传」而非双档分化，故两档注同一 blob（单客户端 rw
// 半侧实际选 rw 档）；ro/rw 选档与 osc52 强制缺席行为由 TestOwnerPolicy 专测。
func TestWelcomePrefs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 注入半侧：ClientPrefsRO/RW 非空 → Welcome 携 prefs 键，逐键值相等
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:      true,
		ClientPrefsRO: json.RawMessage(`{"fontSize":18,"osc52":true}`),
		ClientPrefsRW: json.RawMessage(`{"fontSize":18,"osc52":true}`),
	})
	c, wm := dialHelloPayload(t, ctx, wsURL, 80, 24)
	if wm["mode"] != proto.ModeRW {
		t.Errorf("welcome mode = %v, want %q (writable=true)", wm["mode"], proto.ModeRW)
	}
	prefs, ok := wm["prefs"].(map[string]any)
	if !ok {
		t.Fatalf("Welcome prefs = %v, want JSON object (ClientPrefsRO/RW injected)", wm["prefs"])
	}
	if got := prefs["fontSize"]; got != float64(18) {
		t.Errorf("prefs.fontSize = %v, want 18", got)
	}
	if got := prefs["osc52"]; got != true {
		t.Errorf("prefs.osc52 = %v, want true", got)
	}
	if len(prefs) != 2 {
		t.Errorf("prefs keys = %v, want exactly {fontSize, osc52}", prefs)
	}
	c.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
	// 断开即移除的行为化：再 attach 成功且 prefs 逐键值一致（Welcome 经 outbox
	// 首条入队路径对再 attach 同样成立）
	c2, wm2 := dialHelloPayload(t, ctx, wsURL, 80, 24)
	prefs2, ok := wm2["prefs"].(map[string]any)
	if !ok || prefs2["fontSize"] != float64(18) || prefs2["osc52"] != true {
		t.Errorf("re-attach Welcome prefs = %v, want {fontSize:18, osc52:true}", wm2["prefs"])
	}
	c2.Close(websocket.StatusNormalClosure, "")

	// 未注入半侧：ClientPrefsRO/RW 零值 → Welcome JSON 无 "prefs" 键（omitempty 缺席）
	exitChNil, wsURLNil := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
	cNil, wmNil := dialHelloPayload(t, ctx, wsURLNil, 80, 24)
	if _, present := wmNil["prefs"]; present {
		t.Errorf("Welcome JSON = %v, must not contain %q key (omitempty, zero ClientPrefsRO/RW)", wmNil, "prefs")
	}
	cNil.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitChNil)
	// 断开即移除的行为化：再 attach 成功且仍无 prefs 键
	cNil2, wmNil2 := dialHelloPayload(t, ctx, wsURLNil, 80, 24)
	if _, present := wmNil2["prefs"]; present {
		t.Errorf("re-attach Welcome JSON = %v, must not contain %q key", wmNil2, "prefs")
	}
	cNil2.Close(websocket.StatusNormalClosure, "")
}
