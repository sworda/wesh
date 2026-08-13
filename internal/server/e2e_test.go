package server_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// TestEchoPTY（VALIDATION 1-01-06）：`wesh -- /bin/cat` 下 WS 客户端发 INPUT 帧
// 应收同字节 OUTPUT 帧；随后断开客户端，断言 exitf 以退出码 0 被调用（D-11 触发面）。
func TestEchoPTY(t *testing.T) {
	sess, err := pty.Start([]string{"/bin/cat"})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh := make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()
	go http.Serve(ln, srv.Handler())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws://"+ln.Addr().String()+"/ws", nil)
	if err != nil {
		t.Fatalf("websocket.Dial: %v", err)
	}

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
	server.New(sess, func(code int) { exitCh <- code }) // 不连接任何 WS 客户端

	select {
	case code := <-exitCh:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child did not exit within 5s — D-12 drain not wired")
	}
}
