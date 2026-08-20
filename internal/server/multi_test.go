package server_test

// multi_test.go 锁定 MULTI-01 主干与多客户端生命周期推论（VALIDATION
// 05-W0-01/05-01-01）：fan-out 两端逐字节一致 / 断开不退出且再 attach 成功 /
// 子进程退出广播 1000 + exitf。helper 复用 e2e_test.go 零改动（PATTERNS exact
// 先例：startTestServerWith/dialHello/waitExit/assertNoExit 同包直接复用）。

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
)

// TestMultiClientFanout（VALIDATION 05-01-01，MULTI-01 主干断言）：双客户端 attach
// 同一服务端各自收到 Welcome，且实时收到同一 OUTPUT 字节流——两端累积 payload
// 逐字节一致（hub 每 chunk 组一次共享只读帧的行为证据，P5-1）。
// 异尺寸参数化（80x24 与 132x43——dialHello 签名参数化的既定用法，e2e_test.go
// 注释：禁止硬编码 80x24）。
func TestMultiClientFanout(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
	cB, modeB := dialHello(t, ctx, wsURL, 132, 43)
	// 两端各自 Welcome mode 一致（无认证 --writable 均 rw）。
	if modeA != proto.ModeRW || modeB != proto.ModeRW {
		t.Fatalf("welcome modes = %q / %q, want both %q", modeA, modeB, proto.ModeRW)
	}

	// 子进程输出驱动：A 发 INPUT，行规程回显经 hub 扇出到 A/B。
	payload := []byte("fanout-payload")
	if err := cA.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT on A: %v", err)
	}
	// 两端各自累积收齐同一 payload（TestEchoPTY 累积模式双份，e2e_test.go 既定形态；
	// 载荷无空白字符故直接比较——strings.Fields 切分免疫 ONLCR 纪律本例无需动用）。
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
	gotA := accum(cA)
	gotB := accum(cB)
	// 两端 payload 逐字节一致断言（string 比较即逐字节）。
	if string(gotA) != string(gotB) {
		t.Fatalf("fan-out payload mismatch: A=%q B=%q", gotA, gotB)
	}
	if string(gotA) != string(payload) {
		t.Fatalf("fan-out payload = %q, want %q", gotA, payload)
	}

	cA.Close(websocket.StatusNormalClosure, "")
	cB.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestDetach（VALIDATION 05-W0-01，多客户端生命周期推论）：任一客户端断开不再触发
// exitf——exitCh 200ms 静默 + 其他客户端继续 echo 正常 + 断开者立即重新 attach
// 成功（注册表移除断言的行为化；P1 D-11 单次语义终结，服务端生命周期只随子进程）。
func TestDetach(t *testing.T) {
	exitCh, wsURL := startTestServer(t, []string{"/bin/cat"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	cB, _ := dialHello(t, ctx, wsURL, 80, 24)

	// A 断开（CloseNow 不发关闭帧，覆盖网络断开路径）→ exitCh 200ms 静默反证。
	cA.CloseNow()
	assertNoExit(t, exitCh)

	// B 继续 echo 正常（断开者对在线客户端零影响）。
	payload := []byte("b-survives-detach")
	if err := cB.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT on B: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := cB.Read(ctx)
		if err != nil {
			t.Fatalf("B read OUTPUT after A detach: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("B echo payload = %q, want %q", got, payload)
	}

	// A 立即重新 attach 成功（注册表移除断言的行为化）。
	cA2, modeA2 := dialHello(t, ctx, wsURL, 80, 24)
	if modeA2 != proto.ModeRW {
		t.Fatalf("re-attach welcome mode = %q, want %q", modeA2, proto.ModeRW)
	}
	cB.Close(websocket.StatusNormalClosure, "")
	cA2.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestExitBroadcast（VALIDATION 05-W0-01，D-10 唯一终结路径的多客户端形态）：
// 子进程退出是唯一终结路径——lifecycle Wait → Drain → 并行广播 1000 关闭全部
// 客户端（双端均收 CloseError 1000）→ exitf 以子进程退出码收口（退出码传递
// 语义不变）。
func TestExitBroadcast(t *testing.T) {
	// sh 读一行后以 3 退出：保证广播时两客户端均已 attach（消除子进程抢在 Dial
	// 完成前退出的竞态——wesh-helper-exit42 同款编排，此处无需专用 helper）。
	exitCh, wsURL := startTestServer(t, []string{"/bin/sh", "-c", "read x; exit 3"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	cB, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 触发子进程退出：A 发一行（INPUT 经 master 送达 sh stdin，规范模式行缓冲）。
	if err := cA.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
		t.Fatalf("write INPUT on A: %v", err)
	}

	// A/B 两端均收 1000 关闭（读到 CloseError 为止，途中回显等 OUTPUT 帧丢弃——
	// TestExitCodePropagation 既定读取循环形态双份）。
	for i, c := range []*websocket.Conn{cA, cB} {
		var ce websocket.CloseError
		for {
			if _, _, rerr := c.Read(ctx); rerr != nil {
				if !errors.As(rerr, &ce) {
					t.Fatalf("client %d read terminated without CloseError: %v", i, rerr)
				}
				break
			}
		}
		if ce.Code != websocket.StatusNormalClosure {
			t.Fatalf("client %d close code = %d, want %d (1000)", i, ce.Code, websocket.StatusNormalClosure)
		}
	}
	// 广播后 exitf 以子进程退出码收口（D-10 退出码传递语义不变）。
	waitExit(t, exitCh, 3)
}

// TestSigwinchOnAttach（D-11 送达证据）：新客户端 attach 完成时服务端向 PTY 前台
// 进程组显式发一次 SIGWINCH——helper 收到信号后落盘标记文件（GOT_WINCH）。
// 两端同尺寸 80x24：排除内核 TIOCSWINSZ 异尺寸发信号的干扰（P5-3 本机实证 Linux
// 同尺寸不发信号）——标记出现即显式 SignalForegroundGroup 送达的证据，而非
// resize 副作用。同步纪律：helper 先从 stdin 读一字节再装处理器报 READY，c1 发
// INPUT 驱动并回读 READY 确认处理器就位，c2 attach 触发第二次信号——消除
//「attach 信号先于处理器安装被默认忽略」的竞态。
// 本测试在 CI macos runner 同样执行（.github/workflows/ci.yml 双平台矩阵既定）——
// darwin 同尺寸行为假设 A1 的验证通道（review MEDIUM 项处置：以 CI 双平台运行
// 实证替代本机平台断言；即便 darwin 同尺寸发信号，显式 SIGWINCH 也只是冗余无害）。
func TestSigwinchOnAttach(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "got_winch")
	_, wsURL := startTestServer(t, helperArgv(t, "wesh-helper-winch", marker))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c1, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c1.CloseNow()
	// 驱动 helper 装处理器：INPUT 携 '\n'（PTY 规范模式行缓冲）；回读 READY 确认
	// 就位（Pitfall 2：Read 永不带 deadline ctx——goroutine + select time.After）。
	if err := c1.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
		t.Fatalf("write INPUT to arm helper: %v", err)
	}
	ready := make(chan struct{}, 1)
	go func() {
		var acc []byte
		for {
			_, data, err := c1.Read(context.Background())
			if err != nil {
				return
			}
			if len(data) > 0 && data[0] == proto.Output {
				acc = append(acc, data[1:]...)
				if bytes.Contains(acc, []byte("READY")) {
					ready <- struct{}{}
					return
				}
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("helper READY not observed within 5s — winch helper not armed")
	}

	// 处理器就位后 attach c2（同尺寸 80x24）：其 attach 完成的显式 SIGWINCH 必然
	// 送达前台进程组 → helper 落盘标记。轮询 5s 断言标记文件出现。
	c2, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c2.CloseNow()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return // GOT_WINCH 落盘——D-11 送达证据齐全
		}
		if time.Now().After(deadline) {
			t.Fatal("GOT_WINCH marker not created within 5s of second attach — SignalForegroundGroup not delivered")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
