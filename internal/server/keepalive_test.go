package server_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// keepalive_test.go 锁定 CORE-06 保活三态（D-16）：默认工作（长空闲存活）/
// pong 超时服务端主动断开 / PingInterval=0 禁用。全部经 Options 注入短时长
// （Planner 注意 8：Options 注入形态，沿用 exitf 注入先例），超时护栏 10s。
//
// 两条库纪律贯穿三测（conn.go:218-220 / read.go:317-323 源码核实）：
//   - 库只在读路径回 pong——"客户端停止 Read"即"不回 pong"的注射形态；
//   - 客户端 Read 永不携带 deadline ctx（Pitfall 2 回归锁）——静默窗口用 sleep/竞速，
//     不用 ctx 到期（到期 = 库内 AfterFunc 整连接关闭，测试会自证其果）。

// TestPingKeepalive（CORE-06）：PingInterval=100ms 注入下，握手完成后空闲 350ms
// （>3 个 ping 间隔）连接保持存活，随后 INPUT echo 全链路断言功能完好。
// 空闲存活的反面是 Pitfall 2/3（读 deadline 误杀、ping 塞进读循环饿死），本测试即其回归。
func TestPingKeepalive(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, PingInterval: 100 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 空闲 350ms（>3 个 ping 间隔）。此间客户端不 Read 故不回 pong，但 PongTimeout
	// 取默认 10s 远大于窗口——若保活误装配（读路径带 deadline / ping 无并发 reader），
	// 连接在此窗口即已被误杀。
	time.Sleep(350 * time.Millisecond)

	// 存活证据：INPUT echo 全链路。客户端恢复 Read 后滞留的 ping 经库 handleControl
	// 补回 pong（read.go:317-323），服务端在途 Ping 正常返回，连接未被保活路径触碰。
	payload := []byte("keepalive")
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT after idle: %v — 空闲连接被误杀", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT after idle: %v (got %q so far) — 空闲连接被误杀", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}

	c.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh) // 多客户端推论：客户端断开不触发 exitf
}

// TestPongTimeout（CORE-06/D-16）：PingInterval=100ms + PongTimeout=300ms 注入下，
// 客户端握手后停止一切 Read——库只在读路径回 pong（read.go:317-323），不 Read 即
// 不应答——服务端 pinger 在 interval+timeout 内必然 pong 超时：stderr 单行事件 +
// CloseNow 主动断开，不泄漏半死连接；reader 终结走 detach 收口（多客户端推论：
// 不触发 exitf）。
func TestPongTimeout(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, PingInterval: 100 * time.Millisecond, PongTimeout: 300 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 停止 Read 600ms（> interval 100ms + timeout 300ms）：服务端首个 ping 等不到
	// pong，pongTimeout 到期 → CloseNow。此窗内客户端任何 Read 都会回 pong 破坏
	// 负例语义，故只能 sleep 等待。
	time.Sleep(600 * time.Millisecond)

	// 下一次 Read 必然返回错误：CloseNow 无关闭帧，客户端见本地合成 1006/异常——
	// 断言 err 非 nil 即可，不断言码。2s 护栏 ctx 防实现回归时测试挂死：若错误是
	// 护栏到期（DeadlineExceeded，read.go:255 直接返回 ctx.Err()）说明服务端未在
	// interval+timeout 内断开，按失败处理而非"断言通过"。
	rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, _, rerr := c.Read(rctx)
	rcancel()
	if rerr == nil {
		t.Fatal("read after pong timeout unexpectedly succeeded — server kept dead connection")
	}
	if errors.Is(rerr, context.DeadlineExceeded) {
		t.Fatalf("read hit 2s guard — server did not CloseNow within interval+timeout: %v", rerr)
	}

	// 服务端 reader 随 CloseNow 终结走 detach 收口（多客户端推论：不触发 exitf，
	// 服务端存活——子进程继续运行，唯一终结路径是子进程退出 D-10）。
	assertNoExit(t, exitCh)
}

// TestPingDisabled（CORE-06/D-16）：PingInterval=0 不启动 ticker——客户端不回 pong
// 也永不因保活断开（用户显式选择）。强断言形态：不回 pong 仍存活反证未发任何 ping。
func TestPingDisabled(t *testing.T) {
	// PongTimeout=100ms 是陷阱参数：若 0 被误作"取默认值"而在窗口内发出 ping，
	// 100ms pong 超时必触发断开，后续 echo 必然失败。
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, PingInterval: 0, PongTimeout: 100 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 客户端停止 Read 500ms（不回 pong）——若保活被误启用，连接已被 CloseNow。
	time.Sleep(500 * time.Millisecond)

	// 存活证据反证无 ping：INPUT echo 全链路成功即连接未被保活路径触碰。
	payload := []byte("ping disabled")
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT with ping disabled: %v — 0 禁用语义被破坏", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT with ping disabled: %v (got %q so far) — 0 禁用语义被破坏", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}

	c.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh) // 多客户端推论：客户端断开不触发 exitf
}
