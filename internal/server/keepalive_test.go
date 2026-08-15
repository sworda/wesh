package server_test

import (
	"context"
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
	waitExit(t, exitCh, 0) // D-11：客户端断开收口
}
