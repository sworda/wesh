package server_test

// perclient_test.go —— 11-01 per-client 生命周期主干测试（PC-02/PC-03/PC-04，
// D-05 独立文件纪律：per-client-only 新文件，本阶段不碰任何既有测试文件
// 装配点；Pitfall 11 红线——shared 零回归证据不动一行，既有测试期望值
// 逐字未动，禁止断言放宽成「两模式都接受」形态）。
//
// helper 复用 e2e_test.go/exit_test.go/events_test.go 同包（server_test）
// 既有形态零改动（dialHello/dialHelloPayload/captureStderr/parseEvents/
// eventsNamed）；新增件 = startPerClientServer harness（spawned 追踪收口
// 夹具）与 readOutputUntil（OUTPUT 帧累积断言）。
//
// 夹具纪律红线（stopseq_test.go:30-31 逐字）：客户端 Read 永不带 per-read
// deadline——静默窗口一律 select + time.After 竞速形态，统护 ctx 只做护栏。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// startPerClientServerWithSpawn 是 per-client 模式测试装配 harness 的通用形态
// （e2e_test.go startTestServerWith 的 per-client 变体，D-05：不碰既有 harness
// 装配点；11-03 扩展——spawnFn 可注入：失败桩/竞态 barrier 注入均经此参数，
// 生产闭包的测试镜像）。装配：New(nil, exitf 捕获桩, Options{SessionMode:
// per-client, SpawnFunc: spawnFn 追踪包装, Writable: true} + mutate 覆写) +
// 127.0.0.1:0 监听。返回 srv（测试期观测口出口消费面）与 spawnedSessions
// 访问器（mu 保护拷贝返回——每次成功 spawn 的 *pty.Session 追踪切片；spawnFn
// 失败的注入不追踪，无资源可收口）。
//
// 收口夹具（killServer e2e_test.go:123-127 的 per-client N 会话形态，实证
// 注释逐字重现）：「泄漏的子进程在测试返回后继续输出，服务端 ReadLoop 的
// onChunk make+copy 后丢弃是 CPU 密集操作，在 CPU 受限 CI 上抢占后续测试
// 调度（ubuntu-latest 实测级联减速）」——per-client 下 sess 不经 New 持握，
// harness 经 SpawnFunc 包装追踪全部已 spawn 会话，Cleanup 逐一 Kill+Close
// （Kill 对已收割进程收 os.ErrProcessDone 静默忽略；Close 幂等——teardown
// 慢半段已关则 no-op，pty/io.go:65-76）。
func startPerClientServerWithSpawn(t *testing.T, spawnFn func(cols, rows int) (*pty.Session, error), mutate func(*server.Options)) (exitCh chan int, wsURL string, srv *server.Server, spawnedSessions func() []*pty.Session) {
	t.Helper()
	var mu sync.Mutex
	var spawned []*pty.Session
	exitCh = make(chan int, 1)
	opts := server.Options{
		SessionMode: server.SessionModePerClient,
		SpawnFunc: func(cols, rows int) (*pty.Session, error) {
			sess, err := spawnFn(cols, rows)
			if err != nil {
				return nil, err
			}
			mu.Lock()
			spawned = append(spawned, sess)
			mu.Unlock()
			return sess, nil
		},
		Writable: true,
	}
	if mutate != nil {
		mutate(&opts)
	}
	srv = server.New(nil, func(code int) { exitCh <- code }, opts)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, sess := range spawned {
			if sess.Cmd != nil && sess.Cmd.Process != nil {
				_ = sess.Cmd.Process.Kill()
			}
			_ = sess.Close()
		}
	})
	go http.Serve(ln, srv.Handler())
	spawnedSessions = func() []*pty.Session {
		mu.Lock()
		defer mu.Unlock()
		return append([]*pty.Session(nil), spawned...)
	}
	return exitCh, "ws://" + ln.Addr().String() + "/ws", srv, spawnedSessions
}

// startPerClientServer 是 startPerClientServerWithSpawn 的薄包装（11-01 五测
// 调用点签名保持，断言零改动）：默认 spawnFn = pty.StartWithSize 直通闭包捕获
// argv——即 cmd/wesh/main.go run() 生产闭包的镜像形态（Options.SpawnFunc
// 注释描述的消费形态）。
func startPerClientServer(t *testing.T, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string) {
	t.Helper()
	exitCh, wsURL, _, _ = startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(argv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, mutate)
	return exitCh, wsURL
}

// readOutputUntil 累积 OUTPUT 帧（剥类型字节）直至 re 命中并返回命中串。
// 统护 ctx 由调用方供给（护栏）；本函数自身不带 per-read deadline（夹具
// 纪律红线，见文件头）。
func readOutputUntil(t *testing.T, ctx context.Context, c *websocket.Conn, re *regexp.Regexp) []byte {
	t.Helper()
	var acc []byte
	for {
		if m := re.Find(acc); m != nil {
			return m
		}
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT: %v（已累积 %q，等待 %s）", err, acc, re)
		}
		if len(data) == 0 || data[0] != proto.Output {
			continue // Welcome 已由 dialHello 消费；静默跳过非 OUTPUT 帧
		}
		acc = append(acc, data[1:]...)
	}
}

// handshakeCollectUntilClose 执行裸握手（websocket.Dial + proto 格式 Hello 帧）
// 并收集服务端帧至 CloseError——升档拒绝路径（spawn 失败 / 容量再闸）的断言
// 通道：Welcome 永不到达的场景 dialHello 不适用（readExitClose exit_test.go
// 形态同构：读至 CloseError 收集帧序；统护 ctx 由调用方供给，本函数不带
// per-read deadline——夹具纪律红线）。
func handshakeCollectUntilClose(t *testing.T, ctx context.Context, wsURL string) (frames [][]byte, code websocket.StatusCode) {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	payload, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, payload...)); err != nil {
		t.Fatalf("write Hello: %v", err)
	}
	for {
		_, data, rerr := c.Read(ctx)
		if rerr != nil {
			var ce websocket.CloseError
			if !errors.As(rerr, &ce) {
				t.Fatalf("read terminated without CloseError: %v（已收集 %d 帧）", rerr, len(frames))
			}
			return frames, ce.Code
		}
		frames = append(frames, data)
	}
}

// decodeSingleErrorFrame 断言帧集恰一帧且为 Error 帧并解码载荷（wire 拒绝面
// 逐字断言的共用前半——D-07：code 机器串 + message 英文人话由前端直显）。
func decodeSingleErrorFrame(t *testing.T, frames [][]byte) proto.ErrorPayload {
	t.Helper()
	if len(frames) != 1 {
		t.Fatalf("帧数 = %d, want 恰 1（Error 帧）: %q", len(frames), frames)
	}
	f := frames[0]
	if len(f) == 0 || f[0] != proto.Error {
		t.Fatalf("帧类型 = %v, want Error ('E')", f)
	}
	var ep proto.ErrorPayload
	if err := json.Unmarshal(f[1:], &ep); err != nil {
		t.Fatalf("decode Error payload: %v", err)
	}
	return ep
}

// healthzClients 返回 GET /healthz 的 clients 字段值（registry.n 计数源的
// 外部可观测面——health.go clients 字段）。scheme/路径替换形态同
// attachURL（e2e_test.go）。
func healthzClients(t *testing.T, wsURL string) int64 {
	t.Helper()
	base := strings.TrimSuffix(strings.Replace(wsURL, "ws://", "http://", 1), "/ws")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(base + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Clients int64 `json:"clients"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /healthz body: %v", err)
	}
	return body.Clients
}

// TestPerClientTwoClientsTwoPids（PC-02 主干，11-01 冒烟一）：两客户端 attach
// （cols/rows 各异）→ 各自 INPUT `echo WESHPID=$$\r` → 各自回读
// WESHPID=(\d+)——两 pid 不等 = 各自独立 PTY 子进程（协议层强证据；
// phase06.mjs S6 readPid「正则只命中结果行」纪律同构：命令回显含 $$ 无数字
// 不命中，正则只命中结果行）；A 端发唯一标记串，B 端 2s 静默窗内零该标记
// = 各端输出即自身进程输出，互不串台。
func TestPerClientTwoClientsTwoPids(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 90, 30)
	cB, _ := dialHello(t, ctx, wsURL, 111, 44)
	defer cA.Close(websocket.StatusNormalClosure, "")
	defer cB.Close(websocket.StatusNormalClosure, "")

	pidRe := regexp.MustCompile(`WESHPID=(\d+)`)
	readPid := func(c *websocket.Conn) []byte {
		t.Helper()
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo WESHPID=$$\r")...)); err != nil {
			t.Fatalf("write INPUT: %v", err)
		}
		return readOutputUntil(t, ctx, c, pidRe)
	}
	pidA, pidB := readPid(cA), readPid(cB)
	if bytes.Equal(pidA, pidB) {
		t.Fatalf("两端回读 %q 相同——per-client 各自独立进程不成立（PC-02）", pidA)
	}

	// A 端发唯一标记串，B 端 2s 静默窗（select + time.After 竞速形态）零命中
	// ——B 自身会话输出（提示符等）允许，唯独不得含 A 的标记（串台证据）。
	marker := []byte("AMARK_7x9q_z")
	if err := cA.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo "+string(marker)+"\r")...)); err != nil {
		t.Fatalf("write INPUT on A: %v", err)
	}
	type readRes struct {
		data []byte
		err  error
	}
	resCh := make(chan readRes, 1)
	quit := make(chan struct{})
	defer close(quit)
	go func() {
		for {
			_, data, err := cB.Read(ctx)
			select {
			case resCh <- readRes{data, err}:
			case <-quit:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	silent := time.After(2 * time.Second)
	for {
		select {
		case r := <-resCh:
			if r.err != nil {
				t.Fatalf("B 端静默窗内 read error: %v", r.err)
			}
			if bytes.Contains(r.data, marker) {
				t.Fatalf("B 端收到 A 端唯一标记 %q——per-client 输出串台（PC-02 违反）: %q", marker, r.data)
			}
		case <-silent:
			return
		}
	}
}

// TestPerClientWelcomeDims（PC-03 冒烟二，ROADMAP SC1 后半）：dialHelloPayload
// cols=111/rows=44 → 首帧类型字节 proto.Welcome（dialHelloPayload 内断言）
// 且 JSON 载荷 cols==111/rows==44/mode=="rw"——Hello 钳制尺寸经
// StartWithSize 直通出生即正确尺寸（无 80x24 中间态闪烁）；Welcome
// cols/rows 回显本端 Hello 钳制尺寸（不经 sessionDimsLocked——per-client
// 无仲裁，G-05-1 契约退化为恒等式）。
func TestPerClientWelcomeDims(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, wm := dialHelloPayload(t, ctx, wsURL, 111, 44)
	defer c.Close(websocket.StatusNormalClosure, "")
	if wm["mode"] != "rw" {
		t.Fatalf("Welcome mode = %v, want rw（Writable:true 装配）", wm["mode"])
	}
	if wm["cols"] != float64(111) || wm["rows"] != float64(44) {
		t.Fatalf("Welcome cols/rows = %v/%v, want 111/44（Hello 钳制尺寸回显，无 80x24 中间态）", wm["cols"], wm["rows"])
	}
}

// ====== 11-01 Task 2 增量：装配契约与输入链测试（三测名仅出现于 func 声明行——
// 验收 grep 行计数闸；doc 注释不引测试名字面量）======

// 输入链全链实证（PC-02/PC-03）：rw 客户端 INPUT `echo ECHOMARK_9z2\r` →
// 输出流含 ECHOMARK_9z2 结果行（cl.inQ → 每会话 inputWriter → master 全链）。
// 正则行首锚定只命中结果行——命令回显行以 "echo " 起首不命中（phase06.mjs
// readPid「正则只命中结果行」纪律的 Go 同构）。
func TestPerClientInputEcho(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c.Close(websocket.StatusNormalClosure, "")

	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo ECHOMARK_9z2\r")...)); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}
	// 命中即通过；未命中则统护 ctx 到期，readOutputUntil 内 Fatal。
	readOutputUntil(t, ctx, c, regexp.MustCompile(`(?m)^ECHOMARK_9z2\r?$`))
}

// sess×mode 装配契约锁定（11-01 planner 裁定登记：契约承载于 New 入口程序
// 错误 panic 而非 ValidateOptions 签名扩展——D-05「options_test.go 一字节
// 不动」与 10-review WR-02 前移纪律调和，PATTERNS §8E 的 options_test 加行
// 建议被本裁定取代）。表驱动两形态：每子测 defer recover 捕获，断言 panic 值
// 含锁定文案子串；两形态均断言 exitf 桩未被调用（装配失败零终结事件——两
// panic 均在 goroutine 启动前触发，零残留；零值 &pty.Session{} 仅作非 nil
// 哨兵，永不 Start）。
func TestNewModeSessContract(t *testing.T) {
	tests := []struct {
		name      string
		sess      *pty.Session
		opts      server.Options
		wantPanic string
	}{
		{"shared requires non-nil sess", nil, server.Options{}, "requires non-nil sess"},
		{"per-client requires nil sess", &pty.Session{}, server.Options{
			SessionMode: server.SessionModePerClient,
			SpawnFunc:   func(cols, rows int) (*pty.Session, error) { return nil, nil },
		}, "requires nil sess"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitfCalled := false
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("New 未 panic，want 含 %q 的程序错误", tt.wantPanic)
				}
				if !strings.Contains(fmt.Sprint(r), tt.wantPanic) {
					t.Fatalf("panic = %v, want 含 %q", r, tt.wantPanic)
				}
				if exitfCalled {
					t.Fatal("装配失败路径 exitf 被调用——零终结事件纪律违反")
				}
			}()
			server.New(tt.sess, func(code int) { exitfCalled = true }, tt.opts)
		})
	}
}

// D-04 窗口期空白锁：captureStderr 窗口内构造 per-client server → 事件流零
// session_start 行；随后 GET /healthz → 200 且 session_active==false
// （11→13 已知中间态显式锁——Phase 13 语义裁决 OQ①② 落地时本断言按裁决
// 翻转）。同步纪律（events_test.go 文件头）：New 内 emit 先于本函数返回
// （程序序），restore() 前的 happens-before 边由 harness 构造序列天然成立
// （本窗口期无 goroutine emit 面——零事件正是断言对象）。
func TestPerClientNoSessionStartEvent(t *testing.T) {
	restore := captureStderr(t)
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)
	out := restore()
	if starts := eventsNamed(parseEvents(t, out), "session_start"); len(starts) != 0 {
		t.Fatalf("per-client New emit 了 %d 条 session_start——D-04 窗口期空白语义违反: %q", len(starts), out)
	}

	// wsURL → http URL（attachURL e2e_test.go 形态同款 scheme/路径替换）。
	base := strings.TrimSuffix(strings.Replace(wsURL, "ws://", "http://", 1), "/ws")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(base + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		SessionActive bool `json:"session_active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /healthz body: %v", err)
	}
	if body.SessionActive {
		t.Fatal("/healthz session_active = true, want false（D-04 窗口期：sessionAlive 不置位；Phase 13 OQ①② 裁决落地时本断言随之翻转）")
	}
}

// ====== 11-03 增量：spawn 失败清理清单与容量闸三形态（三测名仅出现于 func
// 声明行——验收 grep 行计数闸，doc 注释不引测名字面量，同 11-01 Task 2 纪律）======

// spawn 失败 Pitfall 5 清理清单逐条锁（PC-02/ROADMAP SC2，D-04，11-03）：
// SpawnFunc 注入 atomic 失败开关——A attach（开关关）echo 正常 → 开关闭合 →
// B 裸握手收恰一帧 Error{server_error, message 逐字} + close 1011；事件流含
// spawn_failed 恰一条（code==1011、四段 schema、零敏感值——不含注入错误文本
// 与 argv 路径，T-11-03b）；无 B 的 attach 事件（失败点在注册之前——零注册
// 零登记零残留）；/healthz clients==1（B 未进注册表）；A echo 照常（他端零
// 感知）。半开名额 release 恰好一次由 server.go Attach close(helloDone)+
// release() 共用行结构性保证（失败路径在升档分岔内 return，defer 兜底幂等）。
//
// 事件断言同步边（events_test.go 文件头 wire 观测纪律）：logEvent 在 Error
// 直写与 Close 之间程序序执行（同一 goroutine）——观测到 CloseError 即 emit
// 已完成；A 的 attach emit 先于五 goroutine 启动（Welcome 上 wire 之前程序
// 序），dialHello 收 Welcome 即同步。
func TestPerClientSpawnFailure(t *testing.T) {
	argv := []string{"/bin/sh"}
	var failSpawn atomic.Bool
	restore := captureStderr(t)
	_, wsURL, _, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		if failSpawn.Load() {
			// 注入错误文本只进 stub 返回值——wire 面与事件面断言其零出现
			//（定值常量文案纪律，T-11-03b）。
			return nil, errors.New("injected spawn failure for test")
		}
		return pty.StartWithSize(argv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	echoMarker := func(c *websocket.Conn, marker string) {
		t.Helper()
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo "+marker+"\r")...)); err != nil {
			t.Fatalf("write INPUT: %v", err)
		}
		readOutputUntil(t, ctx, c, regexp.MustCompile(`(?m)^`+marker+`\r?$`))
	}
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer cA.Close(websocket.StatusNormalClosure, "")
	echoMarker(cA, "SPAWNOK_3k7") // A attach 成功（开关关），echo 链正常

	failSpawn.Store(true) // 开关闭合——此后 spawn 必败
	frames, code := handshakeCollectUntilClose(t, ctx, wsURL)
	if code != websocket.StatusInternalError {
		t.Fatalf("B close code = %d, want %d (1011)", code, websocket.StatusInternalError)
	}
	ep := decodeSingleErrorFrame(t, frames)
	if ep.Code != proto.ErrServerError {
		t.Fatalf("B Error code = %q, want %q", ep.Code, proto.ErrServerError)
	}
	if ep.Message != "failed to start process" {
		t.Fatalf("B Error message = %q, want %q 逐字（定值常量，零底层错误细节）", ep.Message, "failed to start process")
	}

	out := restore()
	evs := parseEvents(t, out)
	fails := eventsNamed(evs, "spawn_failed")
	if len(fails) != 1 {
		t.Fatalf("spawn_failed event count = %d, want exactly 1: %q", len(fails), out)
	}
	if fails[0]["code"] != float64(websocket.StatusInternalError) {
		t.Fatalf("spawn_failed code = %v, want float64(1011)", fails[0]["code"])
	}
	if remote, _ := fails[0]["remote"].(string); !strings.HasPrefix(remote, "127.0.0.1:") {
		t.Fatalf("spawn_failed remote = %q, want 127.0.0.1: 前缀（四段 schema）", remote)
	}
	// 零敏感值红线（D-04/Pitfall 5）：审计面不含注入错误文本与 argv 路径。
	if strings.Contains(out, "injected") {
		t.Fatalf("stderr 含注入错误文本（敏感值泄露，T-11-03b）: %q", out)
	}
	if strings.Contains(out, argv[0]) {
		t.Fatalf("stderr 含 argv 路径 %q（敏感值泄露，T-11-03b）: %q", argv[0], out)
	}
	// 无 B 的 attach 事件（零注册零登记零残留——失败点在注册之前）。
	if atts := eventsNamed(evs, "attach"); len(atts) != 1 {
		t.Fatalf("attach event count = %d, want exactly 1（仅 A；B 零注册）: %q", len(atts), out)
	}

	// B 未进注册表：/healthz clients==1（registry.n 外部可观测面）。
	if n := healthzClients(t, wsURL); n != 1 {
		t.Fatalf("/healthz clients = %d, want 1（B spawn 失败零注册）", n)
	}
	// 他端零感知：A echo 照常。
	echoMarker(cA, "STILLECHO_8w2")
}

// D-02 pre-spawn 容量再闸（确定性注入经 linger 形态，11-03）：MaxClients=1 +
// argv = trap "" HUP 免疫死循环（启动即印 pid——trap 安装先于 echo，PCAPID
// 回读即 trap 已就位的同步点；stopseq_test.go 夹具纪律同构但免落盘标记——
// 客户端仍在线时 stdout 可观测，断开后不可观测的约束不触及本时序）——
// A attach 得 Welcome 并回读 PCAPID → A 关闭（detach → teardown 发 SIGHUP
// 被 trap 免疫 → 会话待收割滞留 pcSessions，registry 清空）→ 轮询至「注册表
// ==1 且 /healthz clients==0」（linger 形态就位——③位 503 闸对 B 放行的
// 前提，registry.n==0）→ B 裸握手过 ③位后命中 D-02 闸：恰一帧 Error 容量
// 文案逐字 + close 1011 + max_clients 事件恰一条；注册表计数不变（闸在
// spawn 前——B 零 spawn）。尾部断言 A 的 pgid 仍存活（trap 免疫实证——
// setsid 不变量 pgid==pid，kill 信号 0 探活无错）并显式 SIGKILL 收口滞留
// 进程组（防 CI 级联减速的夹具纪律，harness Cleanup Kill+Close 幂等兜底；
// KILL 后 watcher 收割走通既有 sessionWatcher/teardown 序列，非本测断言面）。
func TestPerClientCapacityGate(t *testing.T) {
	argv := []string{"sh", "-c", "trap '' HUP; echo PCAPID=$$; while true; do sleep 1; done"}
	restore := captureStderr(t)
	_, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(argv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) { o.MaxClients = 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	m := readOutputUntil(t, ctx, cA, regexp.MustCompile(`PCAPID=(\d+)`))
	pidA, err := strconv.Atoi(string(m[len("PCAPID="):]))
	if err != nil {
		t.Fatalf("parse PCAPID from %q: %v", m, err)
	}
	if err := cA.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("A close: %v", err)
	}

	// linger 形态就位轮询（5s 护栏）：注册表滞留 1（HUP 免疫——收割阻塞）且
	// registry 清空（detach removeLocked 已完成）。
	deadline := time.Now().Add(5 * time.Second)
	for {
		if srv.PCSessionsLenForTest() == 1 && healthzClients(t, wsURL) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("linger 形态 5s 内未就位：注册表 = %d, /healthz clients = %d（want 1/0）",
				srv.PCSessionsLenForTest(), healthzClients(t, wsURL))
		}
		time.Sleep(10 * time.Millisecond)
	}

	frames, code := handshakeCollectUntilClose(t, ctx, wsURL)
	if code != websocket.StatusInternalError {
		t.Fatalf("B close code = %d, want %d (1011)", code, websocket.StatusInternalError)
	}
	ep := decodeSingleErrorFrame(t, frames)
	if ep.Code != proto.ErrServerError {
		t.Fatalf("B Error code = %q, want %q", ep.Code, proto.ErrServerError)
	}
	if ep.Message != "server is at capacity" {
		t.Fatalf("B Error message = %q, want %q 逐字（D-02 容量文案）", ep.Message, "server is at capacity")
	}
	// 闸在 spawn 前——B 零 spawn，注册表计数不变。
	if n := srv.PCSessionsLenForTest(); n != 1 {
		t.Fatalf("容量闸后注册表 = %d, want 1（pre-spawn 闸零 spawn 零登记）", n)
	}

	// 事件断言同步边：logEvent 在 Error 直写与 Close 之间程序序——观测到
	// CloseError 即 emit 已完成（wire 观测纪律，events_test.go 文件头）。
	out := restore()
	if n := len(eventsNamed(parseEvents(t, out), "max_clients")); n != 1 {
		t.Fatalf("max_clients event count = %d, want exactly 1: %q", n, out)
	}

	// trap 免疫实证 + 显式收口（夹具纪律，见 doc 注释）。
	if err := syscall.Kill(-pidA, 0); err != nil {
		t.Fatalf("A 的 pgid(%d) 探活失败 %v——trap 免疫不成立（断开 SIGHUP 后应 linger）", pidA, err)
	}
	_ = syscall.Kill(-pidA, syscall.SIGKILL)
}

// D-03 注册点复检回收（barrier 竞态注入，11-03）：MaxClients=1 + SpawnFunc
// 进入即阻塞于共享 barrier（原子计数器 ==2 后测试放行）——两客户端并发裸握手
// （各自 Dial + Hello，两端同过 pre-spawn 闸后同卡 barrier = Pitfall 4 超编
// 窗口的确定性打开）；放行后两端分别 spawn 成功，先注册者得 Welcome，后注册者
// 复检超编被 reapOrphanSession 回收：收恰一帧 Error 容量文案 + close 1011。
//
// 断言锁定「恰一胜一负 + 注册表终态 ==1 + 败者被收割」不变量——谁胜谁负不
// 确定是竞态固有性（两结果同为正确形态，此非断言放宽，红线纪律注释锚定）。
// 败者收割实证：wire 面无法映射「连接 ↔ 会话」身份（两容量拒绝形态有意不可
// 区分），以 spawnedSessions() 两会话 pgid「恰一个轮询至 ESRCH（2s 护栏——
// 回收含 Wait 收割、无僵尸）+ 另一个（胜者）仍存活」承载该语义。
func TestPerClientCapacityRecheckRace(t *testing.T) {
	var entered atomic.Int64
	barrier := make(chan struct{})
	var barrierOnce sync.Once
	defer barrierOnce.Do(func() { close(barrier) }) // 失败路径兜底放行（防 spawnFn 挂死泄漏）
	_, wsURL, srv, spawnedSessions := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		entered.Add(1)
		<-barrier // 双端进入后测试放行——两并发升档同过 pre-spawn 闸
		return pty.StartWithSize([]string{"sh"}, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) { o.MaxClients = 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 单端裸握手结果收集（goroutine 内不调 t.Fatal——结果回主 goroutine 断言，
	// testing 纪律：Fatalf 只能由测试 goroutine 调用）。胜者收 Welcome 即返回
	//（连接保持，供收口）；败者收 Error 后读至 CloseError。
	type outcome struct {
		welcome bool
		errCode string
		errMsg  string
		closeCd websocket.StatusCode
		conn    *websocket.Conn
		err     error
	}
	handshake := func() outcome {
		var o outcome
		c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
		if err != nil {
			o.err = err
			return o
		}
		payload, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24})
		if err != nil {
			o.err = err
			return o
		}
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, payload...)); err != nil {
			o.err = err
			return o
		}
		for {
			_, data, rerr := c.Read(ctx)
			if rerr != nil {
				var ce websocket.CloseError
				if errors.As(rerr, &ce) {
					o.closeCd = ce.Code
				} else {
					o.err = rerr
				}
				return o
			}
			if len(data) == 0 {
				continue
			}
			switch data[0] {
			case proto.Welcome:
				o.welcome, o.conn = true, c
				return o
			case proto.Error:
				var ep proto.ErrorPayload
				if err := json.Unmarshal(data[1:], &ep); err != nil {
					o.err = err
					return o
				}
				o.errCode, o.errMsg = ep.Code, ep.Message
			}
		}
	}
	resCh := make(chan outcome, 2)
	go func() { resCh <- handshake() }()
	go func() { resCh <- handshake() }()

	// 双端进入 SpawnFunc（计数器 ==2）后放行——竞态窗口确定性打开（5s 护栏）。
	barrierDeadline := time.Now().Add(5 * time.Second)
	for entered.Load() < 2 {
		if time.Now().After(barrierDeadline) {
			t.Fatalf("5s 内仅 %d 端进入 SpawnFunc——并发升档未成立", entered.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}
	barrierOnce.Do(func() { close(barrier) })

	o1, o2 := <-resCh, <-resCh
	if o1.err != nil || o2.err != nil {
		t.Fatalf("握手错误: %v / %v", o1.err, o2.err)
	}
	// 恰一胜一负（身份不定——竞态固有）：胜者 Welcome 且连接保持；败者恰一帧
	// Error 容量文案 + close 1011。
	var win, lose outcome
	switch {
	case o1.welcome && !o2.welcome:
		win, lose = o1, o2
	case o2.welcome && !o1.welcome:
		win, lose = o2, o1
	default:
		t.Fatalf("两端 welcome = %v/%v——恰一胜一负不成立（%+v / %+v）", o1.welcome, o2.welcome, o1, o2)
	}
	defer win.conn.Close(websocket.StatusNormalClosure, "")
	if lose.closeCd != websocket.StatusInternalError {
		t.Fatalf("败者 close code = %d, want %d (1011)", lose.closeCd, websocket.StatusInternalError)
	}
	if lose.errCode != proto.ErrServerError || lose.errMsg != "server is at capacity" {
		t.Fatalf("败者 Error = {%q, %q}, want {%q, %q} 逐字（复检回收与 pre-spawn 闸同 wire 形态，D-02 wire 聚合）",
			lose.errCode, lose.errMsg, proto.ErrServerError, "server is at capacity")
	}

	// 注册表终态 ==1（「并发子进程数 ≤ maxClients」硬不变量的实测落点，5s 护栏）。
	finalDeadline := time.Now().Add(5 * time.Second)
	for srv.PCSessionsLenForTest() != 1 {
		if time.Now().After(finalDeadline) {
			t.Fatalf("注册表终态 = %d, want 1", srv.PCSessionsLenForTest())
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 败者被收割：两会话 pgid 恰一个至 ESRCH（回收含 Wait——无僵尸），胜者存活。
	spawns := spawnedSessions()
	if len(spawns) != 2 {
		t.Fatalf("spawned 会话数 = %d, want 2（两端均 spawn 成功）", len(spawns))
	}
	pid0, pid1 := spawns[0].Cmd.Process.Pid, spawns[1].Cmd.Process.Pid
	alive := func(pid int) bool { return syscall.Kill(-pid, 0) == nil }
	reapDeadline := time.Now().Add(2 * time.Second)
	for {
		a0, a1 := alive(pid0), alive(pid1)
		if a0 != a1 {
			break // 恰一死一活——败者收割、胜者存活
		}
		if !a0 {
			t.Fatalf("两会话 pgid(%d/%d) 均 ESRCH——胜者会话不应被收割", pid0, pid1)
		}
		if time.Now().After(reapDeadline) {
			t.Fatalf("败者 pgid 2s 内未至 ESRCH（%d/%d 均存活）——回收未含 Wait 收割", pid0, pid1)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
