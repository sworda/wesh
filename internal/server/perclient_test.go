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
	"runtime"
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

// TestPerClientWelcomeSession（D-08，12-01）：per-client 模式 Welcome.session
// e2e 断言——harness 装配（SessionMode: per-client）下 dialHelloPayload 回读
// Welcome JSON：session=="per-client"（服务端组帧第 5 实参 s.sessionMode 的
// 端到端证据），且 mode/cols/rows 既有键同帧共存不挤压（additive 纪律——
// 加键不改既有键形态）。shared 模式 Welcome.session=="shared" 的协议层对照
// 由 12-04 phase12.mjs S1 承担（本文件保 per-client 单一归属，D-11）。
func TestPerClientWelcomeSession(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, wm := dialHelloPayload(t, ctx, wsURL, 80, 24)
	defer c.Close(websocket.StatusNormalClosure, "")
	if wm["session"] != server.SessionModePerClient {
		t.Fatalf("Welcome session = %v, want %q（D-08 模式位——per-client 装配组帧）", wm["session"], server.SessionModePerClient)
	}
	if wm["mode"] != "rw" {
		t.Fatalf("Welcome mode = %v, want rw（Writable:true 装配；additive 键不挤压既有键）", wm["mode"])
	}
	if wm["cols"] != float64(80) || wm["rows"] != float64(24) {
		t.Fatalf("Welcome cols/rows = %v/%v, want 80/24（Hello 钳制尺寸回显；additive 键不挤压既有键）", wm["cols"], wm["rows"])
	}
}

// ====== 11-01 Task 2 增量：装配契约与输入链测试（三测名仅出现于 func 声明行——
// 验收 grep 行计数闸；doc 注释不引测试名字面量）======

// 输入链全链实证（PC-02/PC-03）：rw 客户端 INPUT `echo ECHOMARK_9z2\r` →
// 输出流含 ECHOMARK_9z2 结果行（cl.inQ → 每会话 inputWriter → master 全链）。
// 正则行首锚定只命中结果行——命令回显行以 "echo " 起首不命中（phase06.mjs
// readPid「正则只命中结果行」纪律的 Go 同构）。(?:\$ )? 容忍 shell 启动
// 提示符与结果行同行的交错形态：CI 慢 runner 上 shell 冷启动慢，PS1 打印
// 晚于 tty 回显（echo ...\r\n 之后、结果行之前），实证 = CI run 33843785651
// ubuntu leg 累积串 "echo X\r\n$ X\r\n$ "——回显行 "$ echo X" 行首为
// "$ echo" 仍不命中，断言强度不变（marker 逐字 + 结果行锚定保留）。
func TestPerClientInputEcho(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c.Close(websocket.StatusNormalClosure, "")

	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo ECHOMARK_9z2\r")...)); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}
	// 命中即通过；未命中则统护 ctx 到期，readOutputUntil 内 Fatal。(?:\$ )?
	// 容忍 PS1 交错形态（慢 runner 上 shell 启动提示符晚于回显——同函数头注释）。
	readOutputUntil(t, ctx, c, regexp.MustCompile(`(?m)^(?:\$ )?ECHOMARK_9z2\r?$`))
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
		readOutputUntil(t, ctx, c, regexp.MustCompile(`(?m)^(?:\$ )?`+marker+`\r?$`))
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

// ====== 11-04 增量：per-client 生命周期行为锁定（PC-03/PC-04 的 Go 侧证据。
// 测名仅出现于 func 声明行——验收 grep 行计数闸，doc 注释不引测名字面量，
// 同 11-01 Task 2 / 11-03 纪律）。新增 helper：frameRes/readPump（静默窗与
// 竞态注入的并发读取源）、drainQuiet（严格静默窗的前置收干）、accumFramesUntil
//（泵源形态 readOutputUntil 变体）、readSessionPid（echo $$ 回读 pid——
// setsid pgid==pid 进程组锚点）、waitPgroupESRCH（进程组消失含收割完成的
// 轮询）。======

// frameRes 是 readPump 的单次 Read 结果（data 为帧体含类型字节；err 含
// CloseError/net.ErrClosed——泵在 err 送达后停止）。
type frameRes struct {
	data []byte
	err  error
}

// readPump 把 c 的 Read 结果持续泵入 out（静默窗收干与竞态注入的并发读取
// 源——客户端 Read 永不带 per-read deadline，夹具纪律红线；统护 ctx 到期/
// 对端关闭/quit 关闭均使泵收口）。out 须带缓冲：消费者 Fatal 后泵不因推送
// 阻塞泄漏。
func readPump(ctx context.Context, c *websocket.Conn, out chan<- frameRes, quit <-chan struct{}) {
	for {
		_, data, err := c.Read(ctx)
		select {
		case out <- frameRes{data, err}:
		case <-quit:
			return
		}
		if err != nil {
			return
		}
	}
}

// drainQuiet 消费泵源直至「至少一帧到达后 quiet 时长内零帧到达」——严格静默窗
// 断言的前置收干：交互 sh 的启动/命令后提示符是本端自身会话的正常输出，不
// 收干会被静默窗误判为他端余波（假阳性）。首帧前的等待不设上限（交互 sh 的
// 启动提示符必然到达——「提示符尚未到达就静默」的假空窗结构性排除；统护 ctx
// 经泵源 Read 护栏，ctx 到期出错送达即 Fatal）。read error 即 Fatal（本组测试
// 场景连接应存活）。
func drainQuiet(t *testing.T, resCh <-chan frameRes, quiet time.Duration) {
	t.Helper()
	seen := false
	for {
		var quietC <-chan time.Time
		if seen {
			quietC = time.After(quiet)
		}
		select {
		case r := <-resCh:
			if r.err != nil {
				t.Fatalf("收干期间 read error: %v（连接应存活）", r.err)
			}
			seen = true
		case <-quietC:
			return
		}
	}
}

// accumFramesUntil 从泵源累积 OUTPUT 帧载荷（剥类型字节）直至 re 命中并返回
// 命中串——readOutputUntil 的泵源变体（该端读取已被 readPump 独占时的回读
// 断言通道；统护 ctx 经泵源 Read 护栏）。
func accumFramesUntil(t *testing.T, resCh <-chan frameRes, re *regexp.Regexp) []byte {
	t.Helper()
	var acc []byte
	for {
		if m := re.Find(acc); m != nil {
			return m
		}
		r := <-resCh
		if r.err != nil {
			t.Fatalf("泵源 read: %v（已累积 %q，等待 %s）", r.err, acc, re)
		}
		if len(r.data) == 0 || r.data[0] != proto.Output {
			continue // 静默跳过非 OUTPUT 帧（Welcome 已由 dialHello 消费）
		}
		acc = append(acc, r.data[1:]...)
	}
}

// readSessionPid 发 `echo <tag>=$$\r` 并回读解析 pid——setsid pgid==pid 不变量
// （pty spawn 既有）下即该会话的进程组锚点。正则只命中结果行：命令回显含 $$
// 字面（无数字）不命中（phase06.mjs readPid 纪律同构；11-03 容量闸测的
// PCAPID 内联形态抽取为 helper）。
func readSessionPid(t *testing.T, ctx context.Context, c *websocket.Conn, tag string) int {
	t.Helper()
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo "+tag+"=$$\r")...)); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}
	m := readOutputUntil(t, ctx, c, regexp.MustCompile(tag+`=(\d+)`))
	pid, err := strconv.Atoi(string(m[len(tag)+1:]))
	if err != nil {
		t.Fatalf("parse %s pid from %q: %v", tag, m, err)
	}
	return pid
}

// waitPgroupESRCH 轮询（25ms 步进，guard 总护栏）至 kill(-pid, 0) 返回
// ESRCH——setsid pgid==pid 不变量下进程组消失的强证据：组成员僵尸在未收割前
// 对信号 0 仍视为存在，故 ESRCH ⊇ 死亡且收割完成（无僵尸）。**严禁**把
// 「kill 0 无错」当死亡证据——那是存活探针（无错 = 进程组存在）。
//
// macOS XNU 退出过渡态补注（G-11-2，实证 = CI run 33832096581）：进程组成员
// 处于退出过渡态（P_LIST_EXITED 标记/proc_exit 进行中）时 kill(-pgid, 0)
// 对该瞬态可返回 EPERM 而非 0——TestPerClientTeardownRaceOnce 的竞态注入
// 设计（首探针 µs 级紧贴 closeCh 确认）是唯一命中者，同断言的
// DisconnectSIGHUP/StopTimeoutKillFallback 无该设计故未命中，排除环境级
// 拦截的对照证据。POSIX 语义 EPERM = 目标存在但权限拒绝 ≠ 消失，与探针
// 无错同属「进程组仍存在」形态。「ESRCH ⊇ 死亡且收割完成」论证的 macOS
// 组信号语义差异：Linux 僵尸成员对信号 0 视为存在的论断在 macOS 退出
// 过渡态被 EPERM 瞬态遮蔽——形态不同、结论同向，ESRCH 仍为进程组消失含
// 收割完成的强证据。故 EPERM 落入护栏轮询：护栏内 ESRCH 收敛即 PASS，
// guard 到期未 ESRCH 仍 Fatal（僵尸残留/卡死进程组检测保留）；其余非
// ESRCH 非 EPERM 错误维持立即 Fatal（环境异常检测面零弱化）。
func waitPgroupESRCH(t *testing.T, pid int, guard time.Duration) {
	t.Helper()
	if err := waitPgroupESRCHWithProbe(pid, guard, syscall.Kill); err != nil {
		t.Fatal(err)
	}
}

// waitPgroupESRCHWithProbe 是 waitPgroupESRCH 的探针参数化核心（error 返回
// 形态，不接 testing.T——Fatal 路径可在无 Goexit 技巧下以返回值断言）。
// 探针注入设计理由：EPERM 分支在 Linux 同 uid 测试进程下结构性不可达，
// 真实 EPERM 夹具（异 uid 子进程）会破坏服务端 SIGHUP teardown 语义（非
// root 无法向异 uid 进程组发信号），macOS 瞬态本身非确定（唯一命中者正是
// 竞态注入测试）——四语义案例的唯一确定性锁定通道 = 脚本化探针注入，由
// TestWaitPgroupESRCHProbeSemantics 直调本核心断言返回值，同时反向锁定
// 护栏保留与他错立即 Fatal 两半边不被顺手弱化。探针调用点恒
// probe(-pid, 0)：负 pgid 进程组语义 + 信号 0 探测语义，与包装层直传的
// syscall.Kill 实参形态逐语义等价（linux/darwin 双平台同型）。
func waitPgroupESRCHWithProbe(pid int, guard time.Duration, probe func(int, syscall.Signal) error) error {
	deadline := time.Now().Add(guard)
	for {
		err := probe(-pid, 0)
		if err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return nil // 护栏内收敛即 PASS（既有语义）。
			}
			if !errors.Is(err, syscall.EPERM) {
				return fmt.Errorf("kill(-%d, 0) = %v, want ESRCH（进程组消失含收割完成）", pid, err)
			}
			// EPERM：目标存在但权限拒绝 ≠ 消失——与「无错」同归「进程组
			// 仍存在」形态，落入 deadline 检查续轮询（语义论证见头注释）。
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("pgid(%d) %v 护栏内进程组仍存在（kill 0 无错 = 存活探针；EPERM = 目标存在但权限拒绝，同为存活形态——两者皆非死亡证据）", pid, guard)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// scriptedProbe 返回消费预置 error 返回值序列的探针闭包（耗尽后重复末值，
// 支撑恒 EPERM 案例），并在每次调用内断言入参形态：首参 = 负 pid（-pid
// 进程组语义）、次参 = 信号 0（探测语义）——锚定核心内探针调用恒
// probe(-pid, 0)。探针在测试 goroutine 内被同步调用，t.Fatalf 形态安全。
func scriptedProbe(t *testing.T, seq ...error) func(int, syscall.Signal) error {
	t.Helper()
	calls := 0
	return func(pid int, sig syscall.Signal) error {
		if pid >= 0 {
			t.Fatalf("probe 首参 = %d, want 负 pid（-pid 进程组语义）", pid)
		}
		if sig != 0 {
			t.Fatalf("probe 次参 = %d, want 信号 0（探测语义）", sig)
		}
		i := calls
		if i >= len(seq) {
			i = len(seq) - 1
		}
		calls++
		return seq[i]
	}
}

// TestWaitPgroupESRCHProbeSemantics 锁定 waitPgroupESRCHWithProbe 的四语义
// 案例（G-11-2 修复的行为锁）：瞬态容忍收敛（EPERM, EPERM, ESRCH → nil，
// CI run 33832096581 FAIL 形态的消除证明）/ 持续到期翻车（恒 EPERM +
// 150ms 护栏 → 非 nil 且含「护栏」，僵尸残留检测保留，容忍非无限化）/
// 他错立即失败（EINVAL → 非 nil 且含 %v 渲染的 invalid argument，错误串
// 无字面 EINVAL——文案 %v 逐字不变纪律，不为 errors.Is 断言改 %w）/
// 无错存活（nil, nil, ESRCH → nil，既有语义保留）。EPERM 分支在 Linux
// 同 uid 下结构性不可达、macOS 为非确定瞬态——本单测是四语义案例的唯一
// 确定性锁定通道（探针参数化理由详见 waitPgroupESRCHWithProbe 注释）。
// guard 取值：150ms 案例测试时长 ~150ms；2s 案例收敛于 2-3 次轮询约
// 50ms。
func TestWaitPgroupESRCHProbeSemantics(t *testing.T) {
	t.Run("eperm-transient-converges", func(t *testing.T) {
		err := waitPgroupESRCHWithProbe(4242, 2*time.Second, scriptedProbe(t, syscall.EPERM, syscall.EPERM, syscall.ESRCH))
		if err != nil {
			t.Fatalf("EPERM 瞬态容忍 + 护栏内 ESRCH 收敛应返回 nil: %v", err)
		}
	})
	t.Run("eperm-persistent-guard-expiry", func(t *testing.T) {
		err := waitPgroupESRCHWithProbe(4242, 150*time.Millisecond, scriptedProbe(t, syscall.EPERM))
		if err == nil {
			t.Fatal("恒 EPERM 到护栏到期应返回错误——僵尸残留检测保留（容忍非无限化）")
		}
		if !strings.Contains(err.Error(), "护栏") {
			t.Fatalf("护栏到期错误文案应含「护栏」: %v", err)
		}
		if !strings.Contains(err.Error(), "EPERM") {
			t.Fatalf("护栏到期错误文案应含「EPERM」（存活探针两形态明示）: %v", err)
		}
	})
	t.Run("other-error-immediate-fatal", func(t *testing.T) {
		err := waitPgroupESRCHWithProbe(4242, 2*time.Second, scriptedProbe(t, syscall.EINVAL))
		if err == nil {
			t.Fatal("非 ESRCH 非 EPERM 错误应立即返回——环境异常检测面零弱化")
		}
		if !strings.Contains(err.Error(), "invalid argument") {
			t.Fatalf("立即失败错误文案应含 syscall.EINVAL 经 %%v 渲染的 invalid argument: %v", err)
		}
	})
	t.Run("alive-then-esrch", func(t *testing.T) {
		err := waitPgroupESRCHWithProbe(4242, 2*time.Second, scriptedProbe(t, nil, nil, syscall.ESRCH))
		if err != nil {
			t.Fatalf("无错存活后 ESRCH 收敛应返回 nil: %v", err)
		}
	})
}

// EXIT 私有化强形态一（PC-04，11-04 Task 1；Security Mistakes 表「EXIT 广播
// 习惯禁带过来」行的 Go 侧锁，T-11-04a 信息泄露面）：A/B 两端 attach 同一
// per-client 实例 → A 发 exit 42 自杀 → A 端 readExitClose 收至 CloseError：
// 末帧 EXIT exit_code==42 + close 1000（exit_test.go 三件套同包复用）；B 端
// 在 A 终结后 1.5s 静默窗（select + time.After 竞速）内任何 Read 到达即
// FAIL——含 OUTPUT/EXIT/任何类型字节（「他端连 A 的终结余波都不可见」强
// 形态）；窗后 B echo 唯一标记回读（服务端续跑 + B 会话无扰动双重实证）。
//
// 静默窗前置（防假阳性）：B 端先经 drainQuiet 收干——交互 sh 的启动提示符是
// B 自身会话的正常输出，不收干会被窗口误判为 A 的余波。
func TestPerClientExitPrivate42(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	cB, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer cA.Close(websocket.StatusNormalClosure, "") // 已收 CloseError——幂等无害
	defer cB.Close(websocket.StatusNormalClosure, "")

	resCh := make(chan frameRes, 8)
	quit := make(chan struct{})
	defer close(quit)
	go readPump(ctx, cB, resCh, quit)

	drainQuiet(t, resCh, 300*time.Millisecond) // B 端存量输出（启动提示符）收干

	// A 发 exit 42（PTY 规范模式 ICRNL 把 \r 转 \n——真实终端按键同形，
	// exit_test.go 先例）。
	if err := cA.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("exit 42\r")...)); err != nil {
		t.Fatalf("write INPUT on A: %v", err)
	}
	framesA, codeA := readExitClose(t, ctx, cA)
	if codeA != websocket.StatusNormalClosure {
		t.Fatalf("A close code = %d, want %d (1000)", codeA, websocket.StatusNormalClosure)
	}
	if len(framesA) == 0 {
		t.Fatal("A 端无帧——EXIT 帧缺失")
	}
	epA := decodeExitFrame(t, framesA[len(framesA)-1])
	if epA.ExitCode != 42 {
		t.Fatalf("A 端末帧 EXIT exit_code = %d, want 42", epA.ExitCode)
	}

	// B 端 1.5s 静默窗：任何帧/错误到达即 FAIL（他端零扰动强形态——A 的终结
	// 余波连 OUTPUT 级都不得泄漏到 B）。
	select {
	case r := <-resCh:
		t.Fatalf("B 端在 A 终结后静默窗内收到帧/错误——EXIT 串台或余波扰动（T-11-04a）: data=%q err=%v", r.data, r.err)
	case <-time.After(1500 * time.Millisecond):
	}

	// 窗后 B echo 唯一标记回读（服务端续跑 + B 会话无扰动）。
	if err := cB.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo BSURVIVE_4k8\r")...)); err != nil {
		t.Fatalf("write INPUT on B: %v", err)
	}
	// (?:\$ )? 同 TestPerClientInputEcho：容忍 PS1 交错形态（CI run
	// 33843785651 同源竞态——B 端窗后 echo 的结果行可能粘启动提示符）。
	accumFramesUntil(t, resCh, regexp.MustCompile(`(?m)^(?:\$ )?BSURVIVE_4k8\r?$`))
}

// EXIT 私有化强形态二（PC-04 信号死亡 -1 语义，11-04 Task 1）：A 发
// kill -HUP $$（sh 自杀）→ A 端末帧 EXIT exit_code==-1 + close 1000，且
// message 含大写信号名 SIGHUP。-1 语义经 watcher 内联退出码提取
// （exec.ExitError.ExitCode() 对信号死亡返回 -1——shared lifecycle
// :1418-1423 同形）与 exitMessage/exitSignalNum 复用面送达
// （emptyexit_test.go 文件头 accept-255 断言常量同源；exit_test.go 信号
// 形态测的大写信号名断言同形——同为 HUP 自杀夹具）。
//
// 信号选型勘误（11-04 执行期实测修正）：plan 文本写 kill -TERM——但**交互式
// shell 无 trap 时忽略 SIGTERM**（bash 手册 Signals 节/dash 同语义），TERM
// 自杀不致死（实测 10s 统护到期，提示符照常）；HUP 对交互 shell 致死
// （exit_test.go TestExitFrameSignal 的既有信号夹具同款），语义断言面不变。
func TestPerClientExitSignalMinus1(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer cA.Close(websocket.StatusNormalClosure, "") // 已收 CloseError——幂等无害

	if err := cA.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("kill -HUP $$\r")...)); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}
	frames, code := readExitClose(t, ctx, cA)
	if code != websocket.StatusNormalClosure {
		t.Fatalf("close code = %d, want %d (1000)", code, websocket.StatusNormalClosure)
	}
	if len(frames) == 0 {
		t.Fatal("无帧——EXIT 帧缺失")
	}
	ep := decodeExitFrame(t, frames[len(frames)-1])
	if ep.ExitCode != -1 {
		t.Fatalf("EXIT exit_code = %d, want -1（信号死亡不得粉饰为正常退出码）", ep.ExitCode)
	}
	if !strings.Contains(ep.Message, "SIGHUP") {
		t.Fatalf("EXIT message = %q, want 含大写信号名 SIGHUP（exitSignalNum/signalName 复用面）", ep.Message)
	}
}

// 断开 SIGHUP 无宽限无僵尸（PC-03 行为锁，ROADMAP SC3，11-04 Task 1）：A
// attach 回读 pid（readSessionPid——setsid pgid==pid 进程组锚点）→
// Close(1000) → 2s 护栏轮询 kill(-pid, 0) 至 ESRCH（进程组消失含收割完成——
// 无僵尸；反特性 A2「无宽限」ttyd parity——宽限窗内进程占资源且收割竞态面
// 增大，断线不死需求由子进程侧 herdr/tmux 承接）→ pcSessions 收敛 0 →
// /healthz clients==0。
func TestPerClientDisconnectSIGHUP(t *testing.T) {
	_, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize([]string{"sh"}, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	pid := readSessionPid(t, ctx, c, "PCDPID")
	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close: %v", err)
	}

	waitPgroupESRCH(t, pid, 2*time.Second)

	// 注册表收敛（teardown 慢半段移除点，2s 护栏）。
	deadline := time.Now().Add(2 * time.Second)
	for srv.PCSessionsLenForTest() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("pcSessions 2s 内未收敛到 0（当前 %d）——断开收割/移除未落定", srv.PCSessionsLenForTest())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// ESRCH ⊇ detach 已执行（SIGHUP 在 detach 的 teardown 触发内发出）——
	// registry 清空先于进程死亡，此处单点断言即充分。
	if n := healthzClients(t, wsURL); n != 0 {
		t.Fatalf("/healthz clients = %d, want 0（断开即移除）", n)
	}
}

// 断线重连 = 全新进程（PC-03 语义面，ttyd parity，11-04 Task 1）：A attach
// 回读 pid1 → Close → pid1 ESRCH 轮询（复用断开 SIGHUP 形态）→ 同 wsURL
// A2 attach 回读 pid2 → pid2 != pid1。D-06 S7 语义锚定：服务端语义本阶段
// 成立；前端 terminal.reset() 归 Phase 12 不在此断言。
func TestPerClientReconnectNewPid(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c1, _ := dialHello(t, ctx, wsURL, 80, 24)
	pid1 := readSessionPid(t, ctx, c1, "PCRPID")
	if err := c1.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	waitPgroupESRCH(t, pid1, 2*time.Second)

	c2, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c2.Close(websocket.StatusNormalClosure, "")
	pid2 := readSessionPid(t, ctx, c2, "PCRPID")
	if pid2 == pid1 {
		t.Fatalf("重连回读 pid == 首次 %d——重连须为全新进程（ttyd parity）", pid1)
	}
}

// ====== 11-04 Task 2 增量：D-01 KILL 兜底时序双断言 + teardown 恰好一次竞态
// 注入（Pitfall 8/3 的 Phase 11 侧锁；测名纪律同上——仅现于 func 声明行）======

// D-01 KILL 兜底（PC-03，Pitfall 8「HUP 免疫泄漏」的 Phase 11 侧 Go 证据；
// stopseq_test.go:97-134「stop-timeout 前静默 / 到期后护栏内收码」时序双断言
// 形态的 per-client 同构——断言对象从 exitf(-1) 换为 pgid ESRCH + pcSessions
// 收敛）：Options.StopTimeout=1s + argv 为 trap "" HUP 免疫死循环（启动即印
// pid——trap 安装先于 echo，回读 pid 即 trap 已就位的同步点，11-03 容量闸测
// 同款免落盘纪律：客户端仍在线时 stdout 可观测）→ A attach 回读 pid →
// Close(1000) → 断开后 300ms 时点进程组仍存活（HUP 免疫实证——stop-timeout
// 前静默窗；循环夹具无自然死亡路径，此时点消失即 trap 未生效或 KILL 提前
// 补发的序列错误）→ 1s 到期 AfterFunc 补 SIGKILL（经 reaped 闸复检）→ 5s
// 护栏内 ESRCH（SIGKILL 补发 + Wait 收割 + pcSessions 移除全链——D-01 固定
// 序列逐段锚定：SIGHUP 被 trap 免疫 → 1s AfterFunc 补 KILL → Drain(200ms)
// → Close(master) → watcher Wait 返回 → 注册表单点移除）→ pcSessions ==0。
//
// 尾部清场纪律：defer SIGKILL 幂等清场先行注册（已 ESRCH 则静默）——护栏外
// 仍存活即 FAIL，FAIL 路径同样清场，防断言调整留下泄漏窗口（CI 级联减速
// 夹具纪律，killServer 注释先例）；harness Cleanup 的 spawned Kill+Close 对
// 已收割会话幂等无害。
func TestPerClientStopTimeoutKillFallback(t *testing.T) {
	argv := []string{"sh", "-c", "trap '' HUP; echo PCKPID=$$; while true; do sleep 1; done"}
	_, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(argv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) { o.StopTimeout = time.Second })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	pid := readSessionPid(t, ctx, c, "PCKPID")
	defer func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }() // 幂等清场（见 doc 注释）
	closedAt := time.Now()
	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close: %v", err)
	}

	// stop-timeout(1s) 前 300ms 时点静默——HUP 被 trap 忽略，无自然死亡路径
	//（stopseq_test.go:122-128 时点断言形态镜像；syscall.Kill 探测为直接调用，
	// 无 ctx 面，夹具纪律红线不触及）。
	time.Sleep(300 * time.Millisecond)
	if err := syscall.Kill(-pid, 0); err != nil {
		t.Fatalf("断开后 300ms 时点 pgid(%d) 已消失（%v）——trap 免疫应使进程组存活至 stop-timeout 到期（Pitfall 8 夹具未就位）", pid, err)
	}
	t.Logf("断开后 300ms 时点 pgid(%d) 存活——HUP 免疫实证（stop-timeout 前静默窗）", pid)

	// 1s 到期 AfterFunc 补 SIGKILL → 5s 护栏内 ESRCH（KILL 补发的结构证据——
	// 无补发则 trap 免疫进程必然存活到护栏翻车）。
	waitPgroupESRCH(t, pid, 5*time.Second)
	t.Logf("断开至 pgid ESRCH 历时 %v（stop-timeout=1s 到期后 KILL 兜底收割）", time.Since(closedAt))

	// 注册表收敛（teardown 慢半段移除点，2s 护栏）。
	deadline := time.Now().Add(2 * time.Second)
	for srv.PCSessionsLenForTest() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("pcSessions 2s 内未收敛到 0（当前 %d）——KILL 兜底收割/移除未落定", srv.PCSessionsLenForTest())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// teardown 恰好一次竞态注入（PC-03，Pitfall 3「断开↔子死双路收口竞态」的
// Phase 11 侧锁）：单实例 10 轮——每轮新客户端 dialHello（argv sh）→ 回读
// pid → 同端接连发 exit 0 与 conn Close(1000)（两触发源并发：子死 watcher
// 路径 vs 断开 detach 路径）→ 读连接至终结 → 终态四件套断言：pcSessions
// 收敛 0 / /healthz clients==0 / 回读 pid 的 pgid ESRCH / exitf 桩全程零
// 调用。
//
// 竞态固有二形态（≠断言放宽——Pitfall 11 红线保护对象是 shared 零回归期望
// 值，本测锁定的是 per-client 竞态不变量，注释明示区别）：两触发源的胜者
// 不定——形态①子死先收口：EXIT 帧可到达后 1000；形态②断开先落地：服务端
// 库在 detach 前已自动回显关闭并 full-close 服务端 conn，watcher 的 EXIT
// 直写在 +200ms Drain 后必然失败静默（S1 直写纪律：Write 失败不补救直接
// Close），帧集零 EXIT。两形态同为正确终结；锁定对象 = quiescent 终态 +
// exitf 零调用 + 恰好一次，非时序胜者身份。
//
// 「读连接至 CloseError」的并发泵实现注解：客户端主动 Close 完成后 Read 恒
// 返 net.ErrClosed（库 prepareRead 先查 c.closed），故关闭码证据由两通道
// 承载——泵观测到对端关闭帧 = CloseError{1000} 直接断言；Close 返回 nil =
// 库 closeHandshake 内 CloseStatus==code 校验已通过（对端即 1000）；Close
// 返 net.ErrClosed = 连接已被对端关闭帧收口（泵侧必然已观测 CloseError）。
// EXIT 帧至多一个（恰好一次的 wire 面观察）；到达即 exit_code==0——EXIT 上
// wire 的前提 = watcher 在客户端关闭帧被服务端处理前完成终结写（conn 仍
// 开），该前提蕴含子死路径先收口（断开先收口则 conn 已随库自动回显关闭，
// 见上），子死先收口蕴含死因 = exit 0（reaped 栅栏内 SIGHUP 不发）。
//
// 恰好一次的结构性锁：teardownOnce 双触发单执行（Pitfall 3「两路径都只触发、
// 执行序列只有一个」）；waitDone/teardownDone 双 close 若失守即 panic（重复
// close channel 使测试二进制整体崩坏必现形）——零 panic 由结构承载不另断言。
// exitf 零调用 = per-client 会话终结绝不触达 exitf（D-10/D-13 硬约束）——
// 本断言同时是 11-01 已知中间态①②（--once/--exit-when-empty per-client
// 永不退出，第二终结源归 Phase 13 pcSupervisor）的反向锁：Phase 13 落地时
// 本断言按裁决翻转（11-04 flagged_assumptions 锚定）。
func TestPerClientTeardownRaceOnce(t *testing.T) {
	// StopTimeout=1s 覆写（2026-09-04 post-merge 调查实证，Rule 3 偏差登记）：
	// plan 原文未覆写（stopTimeout=0 纯 HUP 路径），但本机 bash 4.4 交互模式在
	//「提示符 pselect 稳定态 + 竞态输入行恰好待读」窗口内 readline 信号中止会
	// 丢弃输入行并错过 termsig 退出——SIGHUP 被壳侧无声吸收（kill(-pgid) 成功
	// 发出、disposition=caught、SigBlk/ShdPnd 全零、进程存活的实证链见
	// 11-04-SUMMARY 附录）。纯 HUP 即杀由 TestPerClientDisconnectSIGHUP（无并发
	// 输入竞争，quiescent 提示态）确定性锁定；本测的 pgid ESRCH 终态改经 D-01
	// KILL 兜底生产路径达成——HUP 被吸收场景的 1s 补发正是该序列的真实消费点，
	// 恰好一次/quiescent/exitf 零调用三不变量锁定面逐字不动。1s 到期 + ESRCH
	// < 2s 护栏的时序容差论证同 TestPerClientStopTimeoutKillFallback。
	exitCh, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize([]string{"sh"}, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) { o.StopTimeout = time.Second })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for i := 0; i < 10; i++ {
		c, _ := dialHello(t, ctx, wsURL, 80, 24)
		pid := readSessionPid(t, ctx, c, "PCRIPID")

		resCh := make(chan frameRes, 16)
		quit := make(chan struct{})
		go readPump(ctx, c, resCh, quit)

		// 两触发源并发接连发出（执行序不定是竞态固有性）：子死（exit 0）与
		// 断开（Close(1000)）。
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("exit 0\r")...)); err != nil {
			t.Fatalf("第 %d 轮 write INPUT: %v", i, err)
		}
		closeCh := make(chan error, 1)
		go func() { closeCh <- c.Close(websocket.StatusNormalClosure, "") }()

		// 读至终结（两形态均合法——见 doc 注释「竞态固有二形态」）。
		var frames [][]byte
		closeVerified1000 := false
		for {
			r := <-resCh
			if r.err == nil {
				frames = append(frames, r.data)
				continue
			}
			var ce websocket.CloseError
			switch {
			case errors.As(r.err, &ce):
				if ce.Code != websocket.StatusNormalClosure {
					t.Fatalf("第 %d 轮对端关闭码 = %d, want %d (1000)", i, ce.Code, websocket.StatusNormalClosure)
				}
				closeVerified1000 = true
			case errors.Is(r.err, net.ErrClosed):
				// Close 握手侧已观测关闭完成——1000 证据由 closeCh 通道承载。
			default:
				t.Fatalf("第 %d 轮 read error: %v（非 CloseError/net.ErrClosed 终结形态）", i, r.err)
			}
			break
		}
		close(quit)
		switch err := <-closeCh; {
		case err == nil:
			// closeHandshake 成功返回 = 库内 CloseStatus==code 校验已过（对端 1000）。
			closeVerified1000 = true
		case errors.Is(err, net.ErrClosed):
			// 幂等形态：连接已被对端关闭帧收口（泵侧已观测 CloseError{1000}）。
		default:
			t.Fatalf("第 %d 轮 Close = %v, want nil 或 net.ErrClosed 幂等形态", i, err)
		}
		if !closeVerified1000 {
			t.Fatalf("第 %d 轮两通道均未取得 1000 关闭证据（泵与 Close 握手双通道校验）", i)
		}

		// wire 面恰好一次 + EXIT 上 wire 蕴含 exit 0 先收口（论证见 doc 注释）。
		exitFrames := 0
		for _, f := range frames {
			if len(f) > 0 && f[0] == proto.Exit {
				exitFrames++
				if ep := decodeExitFrame(t, f); ep.ExitCode != 0 {
					t.Fatalf("第 %d 轮 EXIT exit_code = %d, want 0（EXIT 上 wire 蕴含 exit 0 先收口）", i, ep.ExitCode)
				}
			}
		}
		if exitFrames > 1 {
			t.Fatalf("第 %d 轮收到 %d 个 EXIT 帧——恰好一次违反（Pitfall 3）", i, exitFrames)
		}

		// 终态四件套（quiescent 收敛，护栏纪律）。
		waitPgroupESRCH(t, pid, 2*time.Second)
		deadline := time.Now().Add(2 * time.Second)
		for srv.PCSessionsLenForTest() != 0 {
			if time.Now().After(deadline) {
				t.Fatalf("第 %d 轮 pcSessions 2s 内未收敛到 0（当前 %d）", i, srv.PCSessionsLenForTest())
			}
			time.Sleep(10 * time.Millisecond)
		}
		// /healthz 外部视图同样轮询收敛（CI flake 收口：registry.n 外部计数
		// 递减与 pcSessions 内部视图归零存在异步窗口，macos 慢机一次性断言
		// 翻车——PC-10 dwell 到期观测同款护栏形态，非断言弱化：2s 护栏内
		// 必达仍是硬约束）。
		deadline = time.Now().Add(2 * time.Second)
		for {
			if n := healthzClients(t, wsURL); n == 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("第 %d 轮 /healthz clients 2s 内未收敛到 0（当前 %d）", i, healthzClients(t, wsURL))
			}
			time.Sleep(10 * time.Millisecond)
		}
		// 轮间 100ms 收敛等待（真实等待 + 护栏上限纪律，phase06.mjs 时序容差
		// 论证先例的 Go 同构）——并给任何失守的第二终结触发留出暴露窗口，
		// 使 exitf 零调用断言覆盖 quiescent 后 100ms。
		time.Sleep(100 * time.Millisecond)
		if n := len(exitCh); n != 0 {
			t.Fatalf("第 %d 轮 exitf 被调用（exitCh len=%d）——per-client 会话终结绝不触达 exitf（D-10/D-13 硬约束）", i, n)
		}
	}
}

// ====== 12-02 增量：RESIZE 直通 / 尺寸隔离 / ro INPUT 门控 / 限速保留断言组
// （PC-05/PC-07 服务端收口，D-11 单一家纪律——五测全部落本文件；winsize 观测面
// 复用同包 resize_arb_test.go 的 ptySize/pollSize（creack/pty Getsize 即
// TIOCGWINSZ 直读，io.go Resize 合法观测依据）；RESIZE 帧发送复用同包 sendResize）======

// TestPerClientResizePassthroughRW（PC-05 rw case，12-02）：rw 端 dialHello(111x44)
// → spawnedSessions[0] PTY 出生即 111x44（Hello 钳制尺寸直通，PC-03 既有语义）；
// RESIZE 120x50 → 50ms 防抖窗后 PTY == 120x50（轮询非固定 sleep——Phase 9 flake
// 教训；直通路径 = server.go RESIZE case cl.pc != nil 分支 → 每会话 debouncer →
// sess.Resize 仅 fdMu）。DecodeResize 钳制 [1,1000] 在解码层既有（D-16），本测
// 覆盖正常值域直通；越界钳制为 proto 层既有锁定（proto_test），不重复断言。
func TestPerClientResizePassthroughRW(t *testing.T) {
	_, wsURL, _, spawnedSessions := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize([]string{"sh"}, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 111, 44)
	defer c.Close(websocket.StatusNormalClosure, "")
	if mode != proto.ModeRW {
		t.Fatalf("Welcome mode = %q, want %q（Writable:true 默认装配）", mode, proto.ModeRW)
	}
	sess := spawnedSessions()[0]
	pollSize(t, sess, 111, 44) // Hello 钳制尺寸出生即正确（基线）

	sendResize(t, ctx, c, 120, 50)
	pollSize(t, sess, 120, 50) // 直通 + 50ms 防抖后应用（轮询至上限 5s）
}

// TestPerClientResizePassthroughRO（PC-05 ro case，D-06）：mutate 覆写
// Writable:false（Options 既有字段，零新装配面）→ ro 端照常独立 spawn（PC-07
// 前半），其 RESIZE 133x55 直通自己 PTY——D-09 第二闸（ro 丢弃）在 per-client
// 分支不生效的直行为证据；shared 半侧 ro 丢弃闸由既有 TestResizeArbitration
// 「owner模式参与集与ro忽略闸」子测锁定（零改动全绿即保持）。
func TestPerClientResizePassthroughRO(t *testing.T) {
	_, wsURL, _, spawnedSessions := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize([]string{"sh"}, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) { o.Writable = false })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 111, 44)
	defer c.Close(websocket.StatusNormalClosure, "")
	if mode != proto.ModeRO {
		t.Fatalf("Welcome mode = %q, want %q（mutate 覆写 Writable:false）", mode, proto.ModeRO)
	}
	sess := spawnedSessions()[0]
	pollSize(t, sess, 111, 44) // ro 客户端照常 spawn，Hello 尺寸出生（基线）

	sendResize(t, ctx, c, 133, 55)
	pollSize(t, sess, 133, 55) // D-06：ro RESIZE 直通自己 PTY
}

// TestPerClientResizeIsolation（PC-05 成功准则 1，12-02）：双端 attach（Hello
// 尺寸各异）→ A RESIZE 110x50 → A 的 PTY 变 110x50、B 的 PTY 恒为自己的 Hello
// 值 120x40（各归各会话，无仲裁无串扰）；且 attach Welcome 之后双端零 'W' 帧
// （静默窗断言——per-client 运行期线上无 Welcome 约束帧，resize 仲裁/fan-out
// 不装配，arbiter 零值天然 no-op）。300ms 负向余量 > 50ms 防抖窗（resize_arb_test
// 「owner模式参与集与ro忽略闸」既有形态）：若 B 被误串扰（仲裁残留路径），防抖
// 到期早已应用。
func TestPerClientResizeIsolation(t *testing.T) {
	_, wsURL, _, spawnedSessions := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize([]string{"sh"}, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 100, 30)
	defer cA.Close(websocket.StatusNormalClosure, "")
	cB, _ := dialHello(t, ctx, wsURL, 120, 40)
	defer cB.Close(websocket.StatusNormalClosure, "")

	sess := spawnedSessions()
	if len(sess) != 2 {
		t.Fatalf("spawned sessions = %d, want 2（双端 attach 各自独立 spawn）", len(sess))
	}
	pollSize(t, sess[0], 100, 30) // A 各自 Hello 尺寸
	pollSize(t, sess[1], 120, 40) // B 各自 Hello 尺寸

	// 双端读泵（夹具纪律：无 per-read deadline，goroutine + 缓冲 channel +
	// select time.After 竞速）——静默窗 'W' 帧收集通道；泵启动前 Welcome 均已
	// 被 dialHello 消费，泵内任何 Welcome 帧即运行期约束帧违规。
	resChA := make(chan frameRes, 16)
	resChB := make(chan frameRes, 16)
	quit := make(chan struct{})
	defer close(quit)
	go readPump(ctx, cA, resChA, quit)
	go readPump(ctx, cB, resChB, quit)

	// A resize → A winsize 变（轮询），B winsize 恒为 Hello 值（负向断言）。
	sendResize(t, ctx, cA, 110, 50)
	pollSize(t, sess[0], 110, 50)
	time.Sleep(300 * time.Millisecond) // > 50ms 防抖窗余量——B 若被误串扰早已应用
	if cols, rows := ptySize(t, sess[1]); cols != 120 || rows != 40 {
		t.Fatalf("B PTY size = %dx%d, want 120x40——per-client 双端 resize 互不影响（PC-05）", cols, rows)
	}

	// 静默窗（select + time.After 竞速）：泵启动以来双端累积帧零 'W'——
	// 窗口内其余帧（sh 提示符/回显等自身会话输出）允许，唯独不得有 Welcome。
	silent := time.After(500 * time.Millisecond)
	welcomes := 0
	for {
		select {
		case r := <-resChA:
			if r.err != nil {
				t.Fatalf("A 静默窗 read error: %v", r.err)
			}
			if len(r.data) > 0 && r.data[0] == proto.Welcome {
				welcomes++
			}
		case r := <-resChB:
			if r.err != nil {
				t.Fatalf("B 静默窗 read error: %v", r.err)
			}
			if len(r.data) > 0 && r.data[0] == proto.Welcome {
				welcomes++
			}
		case <-silent:
			if welcomes != 0 {
				t.Fatalf("双端静默窗内收到 %d 帧 'W'——per-client 运行期零 Welcome 约束帧（PC-05）", welcomes)
			}
			return
		}
	}
}

// TestPerClientROInputDropped（PC-07 INPUT 门控，12-02）：ro 端（Writable:false
// 实例）发 INPUT `echo SHOULD_NOT_ECHO\r` → 静默窗内其 OUTPUT 零 MARKER 命中
// ——INPUT 在服务端 mode 闸被丢弃，对自身进程同样无效（ro 独占进程不构成放开
// 输入面的理由，T-05-08 越权面保持）；rw 对照（Writable:true 实例）echo 正常
// 回显——同一断言通道证明丢弃是 mode 闸语义而非链路故障。
func TestPerClientROInputDropped(t *testing.T) {
	// 半场一：ro 实例（mutate 覆写 Writable:false）——INPUT 丢弃证据。
	{
		_, wsURL := startPerClientServer(t, []string{"sh"}, func(o *server.Options) { o.Writable = false })
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c, mode := dialHello(t, ctx, wsURL, 80, 24)
		defer c.Close(websocket.StatusNormalClosure, "")
		if mode != proto.ModeRO {
			t.Fatalf("ro 实例 Welcome mode = %q, want %q", mode, proto.ModeRO)
		}
		marker := []byte("SHOULD_NOT_ECHO")
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo SHOULD_NOT_ECHO\r")...)); err != nil {
			t.Fatalf("ro write INPUT: %v", err)
		}
		resCh := make(chan frameRes, 16)
		quit := make(chan struct{})
		defer close(quit)
		go readPump(ctx, c, resCh, quit)
		var acc []byte
		silent := time.After(2 * time.Second)
	roSilent:
		for {
			select {
			case r := <-resCh:
				if r.err != nil {
					t.Fatalf("ro 静默窗 read error: %v", r.err)
				}
				if len(r.data) > 0 && r.data[0] == proto.Output {
					acc = append(acc, r.data[1:]...)
					if bytes.Contains(acc, marker) {
						t.Fatalf("ro INPUT 到达自身进程——echo 结果出现在 OUTPUT（PC-07 丢弃闸失效）")
					}
				}
			case <-silent:
				if bytes.Contains(acc, marker) {
					t.Fatalf("ro 静默窗后 OUTPUT 含 %q（PC-07 丢弃闸失效）", marker)
				}
				// 12-03 Rule 1 修复：原 return 使半场二（rw 对照）不可达——
				// go vet unreachable 暴露的 12-02 真缺陷（rw 半场从未执行，
				// 「丢弃是 mode 闸语义而非链路故障」对照证据缺位）。labeled
				// break 退出静默窗循环，断言语义逐字不动。
				break roSilent
			}
		}
	}
	// 半场二：rw 对照（默认 Writable:true）——同通道 echo 正常（丢弃是 mode 闸
	// 语义而非链路故障的对照证据）。(?:\$ )? 容忍 PS1 交错形态（11-07 CI flake
	// 修正先例）。
	{
		_, wsURL := startPerClientServer(t, []string{"sh"}, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c, mode := dialHello(t, ctx, wsURL, 80, 24)
		defer c.Close(websocket.StatusNormalClosure, "")
		if mode != proto.ModeRW {
			t.Fatalf("rw 对照 Welcome mode = %q, want %q", mode, proto.ModeRW)
		}
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo ECHOK_PC07\r")...)); err != nil {
			t.Fatalf("rw write INPUT: %v", err)
		}
		readOutputUntil(t, ctx, c, regexp.MustCompile(`(?m)^(?:\$ )?ECHOK_PC07\r?$`))
	}
}

// TestPerClientInputRateLimitKept（PC-07 限速保留，12-02）：rw 端超速率 flood
// INPUT（50×(3KiB+'\n') ≈ 153KiB 瞬时，≫ 64KiB burst + 32KiB/s 补充——远超
// 每客户端限速，超限帧按 R-02 drop 语义丢弃）→ 连接不被踢不关闭（RES-02：
// 超限唯一动作 = 丢弃 + inputDrops 计数，不断开），后续正常输入照常回显
//（洪水后令牌恢复，drop 语义不损伤连接）。洪水后 250ms 回充等待（令牌桶
// 数学确定性：250ms × 32KiB/s = 8KiB 令牌 ≫ 探针帧 14B——非时序断言，是
// 回充护栏）；探针命令前的 `\r` 先收口 canonical 行缓冲中的残留 'x'。探针
// 为轮询重发形态（CI flake 收口）：统护 ctx 下并发读累积（readUntilError
// 形态变体——夹具纪律红线 Read 无 per-read deadline；websocket Read 随
// ctx 取消杀连接，故「短 deadline 试读后重发」不可行，改主循环只 Write
// 探针 + 检查累积），每 300ms 重发收口 + 探针，重发幂等（'\r' 于空行缓冲
// 仅产出空命令回显，探针串重复出现不影响 Find 首中），重发窗 12s（ctx
// 15s 留 3s 洪水写入与收尾余量）。
//
// 洪水分片带 '\n' 的执行期实证演化（CI flake 收口，与 12-03 滴漏形态登记
// 同纪律）：首版 10×15KiB 无换行在 darwin 上 3 次失败——macos BSD pty 输
// 入队列对无换行洪水 flow-control 堵死（master write 阻塞等 bash 消费，
// bash 等 '\r' 交付，'\r' 滞留于堵死字节之后——三方死锁，重发探针亦滞留）；
// Linux n_tty 对超长行溢出字符静默丢弃故从未暴露。分片带 '\n' 使 canonical
// 行缓冲完整交付，bash 持续消费，输入侧结构性不堵死——测试判别面（drop
// 计数 / 不断开 / 探针回显）零改动。
func TestPerClientInputRateLimitKept(t *testing.T) {
	_, wsURL := startPerClientServer(t, []string{"sh"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 80, 24)
	defer c.Close(websocket.StatusNormalClosure, "")
	if mode != proto.ModeRW {
		t.Fatalf("Welcome mode = %q, want %q", mode, proto.ModeRW)
	}

	// 超速率洪水：50×(3KiB 'x' + '\n')（帧总 ≈153KiB，均 < 16KiB
	// ReadLimitPostAuth 单帧硬顶）。分片带换行让 canonical 行缓冲（4096）
	// 完整交付、bash 持续消费输入（CI flake 收口：无换行洪水在 darwin BSD
	// pty 上触发输入队列 flow-control 堵死——收口 '\r' 与探针滞留于堵死
	// 洪水之后永不交付，重发探针同样滞留；Linux n_tty 对溢出字符静默丢弃
	// 故从未暴露）。判别面不变：总字节 ≫ 64KiB burst，超限帧按 R-02 drop
	// 语义丢弃（不断开），通过帧进 inQ（≈64KiB < 256KiB 容量）由
	// inputWriter 写 master；bash 执行 'x' 命令的 not found 噪声是 tty
	// 语义（无换行形态「超长行丢弃」的分片化等价），与本测无关。
	flood := append(bytes.Repeat([]byte{'x'}, 3*1024), '\n')
	for i := 0; i < 50; i++ {
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, flood...)); err != nil {
			t.Fatalf("write flood INPUT #%d: %v", i, err)
		}
	}
	// 并发读累积（统护 ctx，夹具纪律红线）；readErr 供主循环即时报告连接被
	// 服务端终结（若发生即 drop 语义 RES-02 违反的直接证据，不等 12s 护栏）。
	var (
		accMu   sync.Mutex
		acc     []byte
		readErr = make(chan error, 1)
	)
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				readErr <- err
				return
			}
			if len(data) == 0 || data[0] != proto.Output {
				continue // Welcome 已由 dialHello 消费；静默跳过非 OUTPUT 帧
			}
			accMu.Lock()
			acc = append(acc, data[1:]...)
			accMu.Unlock()
		}
	}()
	probeRe := regexp.MustCompile(`(?m)^(?:\$ )?RATEOK_PC07\r?$`)
	time.Sleep(250 * time.Millisecond)
	probeDeadline := time.Now().Add(12 * time.Second)
	for {
		accMu.Lock()
		hit := probeRe.Find(acc) != nil
		tail := acc
		if len(tail) > 200 {
			tail = tail[len(tail)-200:]
		}
		accMu.Unlock()
		if hit {
			break
		}
		select {
		case err := <-readErr:
			t.Fatalf("read OUTPUT: %v（连接被服务端终结——drop 语义 RES-02 违反；已累积尾部 %q）", err, tail)
		default:
		}
		if time.Now().After(probeDeadline) {
			t.Fatalf("探针回显在 12s 重发窗内未出现（已累积尾部 %q）", tail)
		}
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, '\r')); err != nil {
			t.Fatalf("write flush newline: %v", err)
		}
		if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo RATEOK_PC07\r")...)); err != nil {
			t.Fatalf("write probe INPUT: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// ====== 12-03 增量：per-client 背压语义断言组（PC-10/PC-11 服务端收口，D-11
// 单一家纪律——四测全部落本文件；stall 夹具与 seqFlood/assertKicked1013/
// readUntilError 同包 slowclient_test.go 直接复用；停读/续读观测经 12-03 新增
// GateTransitionsForTest 出口——per-client 单模式实例下该计数只有 ReadLoop
// 输出闭包两个递增点（shared 信用门路径结构性不装配），差值即闭包行为直接
// 观测。测名仅出现于 func 声明行——验收 grep 行计数闸，doc 注释不引测名
// 字面量，同 11-01/11-03/12-02 纪律）======

// waitGateTransitions 轮询 GateTransitionsForTest 直至相对起点递增 ≥ want
// （间隔 15ms、guard 总护栏——轮询替代固定 sleep，STATE.md Blockers Phase 9
// flake 教训既定规避形态；hubMu 内读纪律由 ForTest 出口承载）。返回观测值。
func waitGateTransitions(t *testing.T, srv *server.Server, from, want int, guard time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(guard)
	for {
		cur := srv.GateTransitionsForTest()
		if cur-from >= want {
			return cur
		}
		if time.Now().After(deadline) {
			t.Fatalf("gateTransitions %v 内未达 +%d（当前 %d，起点 %d）——停读/续读递增点未发生", guard, want, cur, from)
		}
		time.Sleep(15 * time.Millisecond)
	}
}

// assertSeqContinuity 断言 seqFlood 洪水的完整字节流连续性：fields 严格 +1
// 无缺口（首值恒 1——stall 客户端自洪水首字节起接收，无中段接合切面；任何
// 断裂 = 停读/续读转换丢段 = PC-11 违反），末位 floodLast（darwin 容忍 ≥95%：
// watcher EXIT 直写绕过 outbox 与残余批的 close 竞态——TestGlobalCredit
// darwin 实测同源形态；连续性断言才是字节精确的核心证据）。
func assertSeqContinuity(t *testing.T, acc []byte, floodLast int) {
	t.Helper()
	fields := strings.Fields(string(acc))
	if len(fields) == 0 {
		t.Fatal("未收到任何 OUTPUT 载荷——洪水为空")
	}
	prev := 0
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("field %d = %q 不是 seq 整数（字节流损坏）: %v", i, f, err)
		}
		if n != prev+1 {
			t.Fatalf("seq 在 field %d 断裂: %d（want %d）——停读/续读转换丢段（PC-11 违反）", i, n, prev+1)
		}
		prev = n
	}
	if prev != floodLast {
		if runtime.GOOS == "darwin" {
			if prev < floodLast*95/100 {
				t.Fatalf("seq 末位 = %d, want ≥ %d（95%% of %d，darwin outbox-close 竞态容忍）", prev, floodLast*95/100, floodLast)
			}
			t.Logf("darwin 容忍：seq 末位 = %d（< %d，outbox-close 竞态；连续性断言为核心证据）", prev, floodLast)
		} else {
			t.Fatalf("seq 末位 = %d, want %d（洪水未收齐即终结）", prev, floodLast)
		}
	}
}

// PC-11 停读续读不丢数据（12-03，D-01）：seqFlood 洪水 + stall 夹具（dialHello
// 后不 Read——TCP 吸收带 ~10MiB 填满 → writer 阻塞 → outbox 涨满）→ ReadLoop
// 闭包阻塞持帧停读（gateTransitions +1 轮询观测——停读点递增；闭包停摆 →
// PTY 内核缓冲积压 → 子进程写阻塞，ttyd pty_pause 等效）→ 静默窗
// （clamp(2×停读确认时长, 1s, 5s)，≪ dwell）内连接不被踢 → 恢复读取 → 续读
// （gateTransitions 再 +1——drain 恢复信号唤醒持帧闭包重试同一帧成功）→ 全量
// 收齐至子进程退出广播 1000 → seq 字节连续性断言（停读期输出零丢失）。
//
// SlowDwell 覆写 30s（D-03/D-12 测试覆写通道）：默认 10s 下慢 CI 的洪水填充
// 时长会侵蚀「静默窗 ≪ dwell」约束——30s 使该约束在任何环境稳健成立；dwell
// 到期踢出语义由短值覆写的踢出测承担。静默窗「不被踢」的观测通道：窗内不读
// （读即破坏 stall），证据推迟到恢复阶段——窗内被踢则恢复读取必以 1013 终结
// 而非 1000 退出广播。
func TestPerClientStallBlocksAndResumes(t *testing.T) {
	floodArgv, floodLast := seqFlood()
	_, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(floodArgv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) {
		o.OutboxBytes = 64 * 1024
		o.SlowDwell = 30 * time.Second
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c.CloseNow()
	// writer 同类型连续段合并使积压后的单条 WS 消息可达 outbox cap（64KiB）
	//——超过 Go 客户端库默认 32KiB 读上限触发 1009（TestSlowConsumerKick
	// 实测先例；生产前端为浏览器 WebSocket 无此上限）。
	c.SetReadLimit(4 * 1024 * 1024)

	// 停读观测（+1 轮询；护栏 15s = 洪水填满 TCP 吸收带 ~10MiB 的慢环境上界，
	// TestSlowConsumerKick 12MiB/15s 先例）。
	t0 := time.Now()
	base := srv.GateTransitionsForTest()
	waitGateTransitions(t, srv, base, 1, 15*time.Second)
	confirmDur := time.Since(t0)

	// 静默窗：clamp(2×停读确认时长, 1s, 5s)——下限保「持续停读」判别力，
	// 上限防慢环境（确认窗本身已长）侵蚀 ≪ dwell(30s) 约束。
	silent := 2 * confirmDur
	if silent < time.Second {
		silent = time.Second
	}
	if silent > 5*time.Second {
		silent = 5 * time.Second
	}
	time.Sleep(silent)

	// 恢复读取：readUntilError 全速收干至终结；同时观测续读点递增（+1）。
	res := readUntilError(c)
	waitGateTransitions(t, srv, base, 2, 15*time.Second)

	select {
	case r := <-res:
		var ce websocket.CloseError
		if !errors.As(r.err, &ce) || ce.Code != websocket.StatusNormalClosure {
			t.Fatalf("恢复读取后终结 = %v, want CloseError 1000（子进程退出广播；静默窗内被踢则此处为 1013——PC-11 违反）", r.err)
		}
		assertSeqContinuity(t, r.acc, floodLast)
	case <-time.After(45 * time.Second):
		t.Fatal("恢复读取后 45s 内未收齐洪水——续读后流未恢复（持帧阻塞未被唤醒，死锁）")
	}
}

// PC-10 持续过载 1013（12-03，D-02/D-03）：SlowDwell=500ms 覆写（o.StopTimeout
// 覆写先例形态）+ stall 恒不读 → outbox 写满停读（+1 轮询观测——dwell 自
// 停读起点武装）→ dwell 到期回调 → kickSlowConsumerLocked 既有序列 →
// /healthz clients 归零（只读 HTTP 观测通道不打扰 WS stall 面，轮询替代固定
// sleep）→ wire 面证据：assertKicked1013 两合法终结形态逐字复用
// （CloseError 1013 slow_consumer 机器串逐字 / CI 慢速 frame-cut EOF + acc
// 阈值）。
func TestPerClientDwellKick(t *testing.T) {
	floodArgv, _ := seqFlood()
	_, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(floodArgv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) {
		o.OutboxBytes = 64 * 1024
		o.SlowDwell = 500 * time.Millisecond
	})

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c.CloseNow()
	c.SetReadLimit(4 * 1024 * 1024) // writer 合并消息可达 outbox cap——慢客户端测试既定放宽

	// 停读就位（+1）——此后 dwell 500ms 从停读起点武装。
	base := srv.GateTransitionsForTest()
	waitGateTransitions(t, srv, base, 1, 15*time.Second)

	// dwell 到期观测：kick → removeLocked → /healthz clients 归零。护栏 5s
	//（= dwell 500ms + 调度余量；到期未被踢即护栏翻车——「护栏内到达」断言，
	// 非精确时点断言）。
	deadline := time.Now().Add(5 * time.Second)
	for {
		if n := healthzClients(t, wsURL); n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("dwell(500ms) 到期后 5s 内客户端未被移除——持续过载 1013 踢出未发生（PC-10 违反）")
		}
		time.Sleep(15 * time.Millisecond)
	}

	// wire 面证据：两合法终结形态逐字复用（slowclient_test.go）。
	assertKicked1013(t, c, 15*time.Second, "per-client stall client")
}

// PC-11 慢但在前进永不踢（12-03，D-02 判据核心）：SlowDwell=3s 覆写 + 事件
// 驱动 duty-cycle——每轮「停读形成（gateTransitions +1，dwell 武装）→ 刻意
// 停读 0.4×dwell → 全速读取至续读（再 +1，dwell 重置）」，循环 3 轮：每轮
// 停读间隔 ~0.42×dwell < dwell（安全），累计停读 ~1.26×dwell > dwell（若
// dwell 不随续读重置——跨停读累计形态——第 3 轮停读中段即被 1013 踢出，
// 测试判别力内建）；3 轮后全速收干 → CloseError 1000 + seq 连续无缺口 +
// gateTransitions ≥ +6（3 对停读/续读递增点）。
//
// dwell 基数 3s 的两轮 CI flake 收口（1s 两版停读比例 0.5/0.4 均在 ubuntu
// runner 误踢，kick 恒于 attach+1.37s）：续读链路「客户端读取→多 MB send
// queue 排空→writer 解阻塞→drain→notFull→续读」的绝对耗时由内核 send
// queue 容量与 runner 读取速率决定（2 vCPU + race 检测器下 >0.5s 实证），
// 停读比例缩放不改变绝对预算——唯一稳健收口是放大 dwell 基数（3s 使续读
// 窗口 ~1.7s ≈ 3.4×实证瓶颈），时长代价 ~+5s/CI 可接受，判别力结构不变。
//
// 滴漏形态的执行期实证演化（Rule 3 调试结论，与 plan 文本「每 ~dwell/3 读
// 一小批」的差异登记）：配额泵滴漏（每 tick 读 128KiB）在本机实证下永不触
// 发服务端续读——PTY 行规程使 seq 逐行产出 ~50-500B 微帧，内核 send queue
// 自适应至 ~3-4MiB，writer 阻塞写在多 MB 管线之后，亚秒级配额滴漏的窗口
// 信用不足以完成「管线排空→writer 解阻塞→drain→notFull→续读」链路（停读
// 态按 D-02 定义持续，dwell 正常触发踢出——机制行为正确，测试形态错误）。
// 事件驱动 duty-cycle 以停读/续读事件本身为节拍：停读间隔由测试刻意构造
// （固定 0.4×dwell），续读由全速读取触发（全速续读的可行性由停读续读主测
// 锁定）——参数机器无关，判别力内建。
func TestPerClientDwellNoKickWhileProgressing(t *testing.T) {
	floodArgv, floodLast := seqFlood()
	dwell := 3 * time.Second
	_, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(floodArgv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) {
		o.OutboxBytes = 64 * 1024
		o.SlowDwell = dwell
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c.CloseNow()
	c.SetReadLimit(4 * 1024 * 1024)

	var acc []byte
	base := srv.GateTransitionsForTest()
	// 三轮 duty-cycle：停读（不读，等 +1）→ 刻意停读 0.5×dwell（dwell 计时
	// 中——本段是判别力的载体）→ 全速读到续读（+1）。
	for cycle := 0; cycle < 3; cycle++ {
		stalled := waitGateTransitions(t, srv, base, 2*cycle+1, 15*time.Second)
		time.Sleep(dwell * 2 / 5) // 停读持续 0.4×dwell——累计跨轮 > dwell，单轮 < dwell
		for srv.GateTransitionsForTest() < stalled+1 {
			_, data, err := c.Read(ctx)
			if err != nil {
				var ce websocket.CloseError
				if errors.As(err, &ce) && ce.Code == websocket.StatusTryAgainLater {
					t.Fatalf("第 %d 轮被 1013 踢出——慢但在前进的客户端不得被 dwell 踢（D-02/PC-11 违反）", cycle)
				}
				t.Fatalf("第 %d 轮续读段 read: %v", cycle, err)
			}
			if len(data) > 0 && data[0] == proto.Output {
				acc = append(acc, data[1:]...)
			}
		}
	}
	// 三对停读/续读递增点齐备（+6；此后全速收干段的瞬态二次停读可能追加
	// 成对增量——下界断言）。
	if diff := srv.GateTransitionsForTest() - base; diff < 6 {
		t.Fatalf("duty-cycle gateTransitions 差值 = %d, want ≥ 6（3 对停读/续读递增点未齐备）", diff)
	}
	// 全速收干至终结：零 1013、退出广播 1000、seq 连续（慢消费全程零丢数据）。
	res := readUntilError(c)
	select {
	case r := <-res:
		acc = append(acc, r.acc...)
		var ce websocket.CloseError
		if !errors.As(r.err, &ce) || ce.Code != websocket.StatusNormalClosure {
			t.Fatalf("duty-cycle 后终结 = %v, want CloseError 1000（慢但在前进全程未被踢——D-02 违反）", r.err)
		}
		assertSeqContinuity(t, acc, floodLast)
	case <-time.After(45 * time.Second):
		t.Fatal("duty-cycle 后 45s 内未收齐洪水——全速收干阶段流未恢复")
	}
}

// D-05 观测计数收口（12-03）：起止快照 GateTransitionsForTest 差值断言——
// 停读+续读一轮 = +2。per-client 单模式实例下 gateTransitions 只有 ReadLoop
// 输出闭包两个递增点（shared 信用门路径结构性不装配）：+1 于「客户端恒不读」
// 窗口观测到（停读点唯一可能——续读需 notFull 需 drain 需读取），读取启动后
// 再 +1（续读点），配对成立即两递增点经差值锁定。差值下界 ≥2 而非精确 ==2：
// 恢复读取后的瞬态二次停读（慢 CI 下 TCP 写瞬断 + outbox 64KiB 亚毫秒回填）
// 会追加成对增量——精确计数断言有 flake 面（STATE.md Blockers Phase 9 教训
// 同源，「禁精确时点断言」纪律的计数面同构）。
func TestPerClientStallGateTransitions(t *testing.T) {
	floodArgv, _ := seqFlood()
	_, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(floodArgv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) {
		o.OutboxBytes = 64 * 1024
		o.SlowDwell = 30 * time.Second
	})

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c.CloseNow()
	c.SetReadLimit(4 * 1024 * 1024)

	// 起点快照 → 停读（+1，客户端恒不读窗口）→ 恢复读取 → 续读（再 +1）→
	// 终点快照差值 ≥2。
	start := srv.GateTransitionsForTest()
	waitGateTransitions(t, srv, start, 1, 15*time.Second)
	readUntilError(c) // 恢复读取（计数断言达成即收口，无需收齐洪水——harness Cleanup 收尾）
	end := waitGateTransitions(t, srv, start, 2, 15*time.Second)
	if diff := end - start; diff < 2 {
		t.Fatalf("gateTransitions 起止差值 = %d, want ≥ 2（停读+续读两递增点，D-05）", diff)
	}
	t.Logf("gateTransitions 起止差值 = %d（≥2 达成；瞬态二次停读可能追加成对增量）", end-start)
}
