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
	"net"
	"net/http"
	"regexp"
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
