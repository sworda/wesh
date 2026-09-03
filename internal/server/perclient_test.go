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
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// startPerClientServer 是 per-client 模式的测试装配 harness（e2e_test.go
// startTestServerWith 的 per-client 变体，D-05：不碰既有 harness 装配点）：
// New(nil, exitf 捕获桩, Options{SessionMode: per-client, SpawnFunc: 闭包,
// Writable: true} + mutate 覆写) + 127.0.0.1:0 监听。SpawnFunc 闭包捕获 argv
// 直通 pty.StartWithSize——即 cmd/wesh/main.go run() 生产闭包的镜像形态
// （Options.SpawnFunc 注释描述的消费形态）。
//
// 收口夹具（killServer e2e_test.go:123-127 的 per-client N 会话形态，实证
// 注释逐字重现）：「泄漏的子进程在测试返回后继续输出，服务端 ReadLoop 的
// onChunk make+copy 后丢弃是 CPU 密集操作，在 CPU 受限 CI 上抢占后续测试
// 调度（ubuntu-latest 实测级联减速）」——per-client 下 sess 不经 New 持握，
// harness 经 SpawnFunc 包装追踪全部已 spawn 会话，Cleanup 逐一 Kill+Close
// （Kill 对已收割进程收 os.ErrProcessDone 静默忽略；Close 幂等——teardown
// 慢半段已关则 no-op，pty/io.go:65-76）。
func startPerClientServer(t *testing.T, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string) {
	t.Helper()
	var mu sync.Mutex
	var spawned []*pty.Session
	exitCh = make(chan int, 1)
	opts := server.Options{
		SessionMode: server.SessionModePerClient,
		SpawnFunc: func(cols, rows int) (*pty.Session, error) {
			sess, err := pty.StartWithSize(argv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
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
	srv := server.New(nil, func(code int) { exitCh <- code }, opts)

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
	return exitCh, "ws://" + ln.Addr().String() + "/ws"
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
