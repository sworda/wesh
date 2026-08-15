package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// ====== plan 02-03 增量：SEC-08 守卫链与握手违规测试组 ======
//
// 测试装配统一经 e2e_test.go 的 startTestServerWith/startTestServer/dialHello/waitExit
// 收口（同包直接复用）；超时护栏一律 10s ctx。

// TestHalfOpenPerIP429（SEC-08/D-04）：per-IP 半开连接上限闸——MaxHalfOpenPerIP=1 注入时，
// c1 半开（Accept 后不发 Hello）占住唯一名额，c2 在 Accept 前收到 HTTP 429；
// c1 随后补发 Hello 正常完成握手（429 不误伤在先连接——acquire/release 恰好一次的间接证明，
// 若 c1 的半开名额泄漏或 409 拒绝残留计数，后续握手必被 429 误杀）。
func TestHalfOpenPerIP429(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, MaxHalfOpenPerIP: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// c1：Dial（带 wesh.v1）成功但不发 Hello——占住唯一半开名额。
	// Dial 返回即 101 已收，服务端 handler 顺序执行 acquire→Accept，无时序竞态。
	c1, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("c1 dial: %v", err)
	}

	// c2：同 IP 第二条半开连接 → Accept 前 HTTP 429（零 WS 资源分配）。
	_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err == nil {
		t.Fatal("c2 dial unexpectedly succeeded — per-IP half-open gate missing (D-04)")
	}
	if resp == nil {
		t.Fatalf("c2 dial failed without HTTP response: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("c2 dial status = %d, want %d (429)", resp.StatusCode, http.StatusTooManyRequests)
	}

	// c1 补发 Hello → 收 Welcome：429 未误伤在先连接（名额在 Hello 完成时才释放，
	// 但 acquire 已在手——release 恰好一次，不双重释放导致后续误判）。
	hello, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c1.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, hello...)); err != nil {
		t.Fatalf("c1 write Hello: %v", err)
	}
	_, data, err := c1.Read(ctx)
	if err != nil {
		t.Fatalf("c1 read Welcome: %v", err)
	}
	if len(data) == 0 || data[0] != proto.Welcome {
		t.Fatalf("c1 first frame = %v, want Welcome ('W')", data)
	}

	// INPUT echo 一句确认全链路（writable 装配）。
	payload := []byte("half-open survivor")
	if err := c1.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("c1 write INPUT: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c1.Read(ctx)
		if err != nil {
			t.Fatalf("c1 read OUTPUT: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}

	// 清理：正常关闭 → D-11 收口。
	c1.Close(websocket.StatusNormalClosure, "")
	waitExit(t, exitCh, 0)
}
