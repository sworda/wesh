package server_test

// shutdown_test.go 锁定 07-05（D-23，P6 deferred 兑现）1001 优雅下线：
// Server.Shutdown() 向全部已注册客户端发 1001 Going Away（close reason
// server_shutting_down，无 EXIT 帧前置——进程未退出，终结语义由关闭码承载）
// → 子进程进程组收 stop-signal 序列（默认 HUP，可配 + stop-timeout 补 KILL，
// 复用 07-04 落地的 stopSignal/stopTimeout 字段——Options 单一通道）
// → 子进程死亡经既有 lifecycle 收口 exitf 恰好一次（Shutdown 是触发源不是
// exitf 分支，P1 硬约束零新 exit 分支）。
// helper 复用 e2e_test.go/stopseq_test.go 同包零改动（dialHello/waitExit/
// assertNoExit/waitMarker/killServer）。
//
// 装配形态登记：plan 字面「startTrackedServerWith 起实例」按意图修正为同形态
// 本地变体 startShutdownServerWith——Shutdown 的直接调用面需要 *server.Server
// 句柄，而 startTrackedServerWith 只返回 (exitCh, wsURL, waitHandlers) 三元组；
// 本 helper 逐字复制其装配序列（pty.Start → New → Listen → Cleanup → Serve）
// 并额外返回 srv 句柄（本组不断言 stderr 事件行，waitHandlers 同步边不需要）。
//
// 夹具纪律：TERM 忽略形态复用 07-04 stopseq 夹具（trap 安装与关停信号竞态经
// 落盘标记文件同步——dialHello 完成不等价 trap 已就位）；TestShutdown1001 的
// 默认 HUP 形态无需标记（HUP 默认动作即死亡，无安装窗口）。客户端 Read 永不带
// deadline ctx（Pitfall 2 回归锁）。

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// startShutdownServerWith 是 startTrackedServerWith（e2e_test.go）的同形态
// 装配变体：逐字复制其装配序列并额外返回 *server.Server 句柄（Shutdown 直接
// 调用面——startTrackedServerWith 不暴露 srv，见文件头登记）。
func startShutdownServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, srv *server.Server) {
	t.Helper()
	sess, err := pty.Start(argv, pty.StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh = make(chan int, 1)
	srv = server.New(sess, func(code int) { exitCh <- code }, opts)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { killServer(ln, sess) })
	go http.Serve(ln, srv.Handler())
	return exitCh, "ws://" + ln.Addr().String() + "/ws", srv
}

// readCloseAsync 起客户端读循环 goroutine 读至首个错误（途中数据帧全部跳过）
// 并经缓冲 channel 上报——Shutdown 的 Close 握手需要客户端在读来应答关闭帧
// （库 close 回显走读路径：无在读 Read 则服务端 Close 等满内建 5s 上界，
// close.go:87-89；真实浏览器协议栈透明回显无此窗口）。调用方在 Shutdown 之前
// 启动本循环（plan behavior「客户端读循环收到 CloseError」字面形态），
// c.Read 不可并发的纪律不受影响（dialHello 已返回，本 goroutine 是唯一读者）。
func readCloseAsync(ctx context.Context, c *websocket.Conn) <-chan error {
	ch := make(chan error, 1)
	go func() {
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				ch <- err
				return
			}
		}
	}()
	return ch
}

// awaitCloseError 收 readCloseAsync 的上报并断言为 CloseError（5s 护栏——
// 无关闭到达即结构性失败）。
func awaitCloseError(t *testing.T, errCh <-chan error) websocket.CloseError {
	t.Helper()
	var ce websocket.CloseError
	select {
	case err := <-errCh:
		if !errors.As(err, &ce) {
			t.Fatalf("read terminated without CloseError: %v", err)
		}
		return ce
	case <-time.After(5 * time.Second):
		t.Fatal("client did not observe close within 5s")
		return ce // 不可达（t.Fatal 不返回），编译形态
	}
}

// TestShutdown1001（07-05，D-23 主干）：Writable:true 起 `sh -c 'sleep 100'`
// （无输出干扰帧断言），dialHello 一客户端 → 调 s.Shutdown() → 客户端读循环
// 收到 CloseError 且 code == 1001（websocket.StatusGoingAway）、reason 含
// server_shutting_down → 子进程经 stop-signal 序列（默认 HUP 零值兜底）终结
// → lifecycle 收口 exitf(-1) 恰好一次（默认 HUP 信号死亡 = -1 桩码，P6 OQ1
// accept-255 同源；assertNoExit 200ms 静默锁定无第二次）。
func TestShutdown1001(t *testing.T) {
	exitCh, wsURL, srv := startShutdownServerWith(t, []string{"sh", "-c", "sleep 100"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	errCh := readCloseAsync(ctx, c) // 客户端读循环先行——Close 握手应答面（见函数注释）
	srv.Shutdown()

	ce := awaitCloseError(t, errCh)
	if ce.Code != websocket.StatusGoingAway {
		t.Fatalf("close code = %d, want %d (1001 Going Away)", ce.Code, websocket.StatusGoingAway)
	}
	if !strings.Contains(ce.Reason, "server_shutting_down") {
		t.Fatalf("close reason = %q, want containing %q（D-23 机器串）", ce.Reason, "server_shutting_down")
	}

	// 子进程终结经既有 lifecycle 收口（Shutdown 不调 exitf——触发源非分支）。
	waitExit(t, exitCh, -1)
	assertNoExit(t, exitCh)
}

// TestShutdownStopTimeout（07-05，D-23 × D-22 序列）：Options{StopSignal:
// SIGTERM, StopTimeout: 300ms} + 子进程 trap 忽略 TERM（`trap "" TERM` +
// while 循环显式恒活夹具——07-04 stopseq 夹具形态复用，trap 安装经落盘标记
// 同步）→ Shutdown 广播 1001 后 TERM 被忽略、stop-timeout 到期补发 SIGKILL
// → 信号死亡 exitf(-1) 恰好一次。KILL 补发的结构证据：trap 忽略 TERM 的
// 循环夹具只有 KILL 能致死——无补发则进程必然存活到 waitExit 5s 护栏翻车。
func TestShutdownStopTimeout(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "trap-armed")
	exitCh, wsURL, srv := startShutdownServerWith(t, []string{"sh", "-c", fmt.Sprintf(`trap "" TERM; touch %s; while :; do sleep 10; done`, marker)}, server.Options{
		Writable:    true,
		StopSignal:  syscall.SIGTERM,
		StopTimeout: 300 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	waitMarker(t, marker) // trap 已安装——Shutdown 的 TERM 不再竞态（07-04 夹具纪律）

	errCh := readCloseAsync(ctx, c) // 客户端读循环先行（同上）
	srv.Shutdown()

	// 关停广播与信号配置无关——客户端同样收 1001 + server_shutting_down。
	ce := awaitCloseError(t, errCh)
	if ce.Code != websocket.StatusGoingAway {
		t.Fatalf("close code = %d, want %d (1001 Going Away)", ce.Code, websocket.StatusGoingAway)
	}
	if !strings.Contains(ce.Reason, "server_shutting_down") {
		t.Fatalf("close reason = %q, want containing %q", ce.Reason, "server_shutting_down")
	}

	// TERM 被 trap 忽略 → stop-timeout(300ms) 到期 Shutdown 内补发 SIGKILL
	//（ESRCH 幂等纪律）→ exitf(-1) 恰好一次。
	waitExit(t, exitCh, -1)
	assertNoExit(t, exitCh)
}
