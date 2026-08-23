package server_test

// exit_test.go 锁定 SESS-03 端到端行为（06-01 tracer，06-VALIDATION 06-01-01）：
// 子进程退出后全部在线客户端先收类型化 EXIT 帧（'X'，{"exit_code":N,"message":M}）
// 再收 1000 正常关闭（D-08/D-09/D-10）——帧序（EXIT 必先于 1000 到达）、双端
// 帧体逐字节一致（rw+ro 混合全员同帧，终结无权限语义）、退出码传递、信号死亡
// exit_code=-1 + 大写信号名文案四行为。helper 复用 e2e_test.go 零改动
//（startTestServerWith/dialHello/waitExit 同包先例，06-PATTERNS exact）。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// readExitClose 从 conn 读至 CloseError，途中数据帧全部收集（『读到 CloseError
// 为止途中帧丢弃』形态的收集变体——multi_test.go TestExitBroadcast 先例），返回
// 帧序列与关闭码。客户端 Read 带调用方的 10s 统护 ctx（Pitfall 2 纪律的 e2e
// 既有形态——永不带 per-read deadline）。
func readExitClose(t *testing.T, ctx context.Context, c *websocket.Conn) (frames [][]byte, code websocket.StatusCode) {
	t.Helper()
	var ce websocket.CloseError
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			if !errors.As(err, &ce) {
				t.Fatalf("read terminated without CloseError: %v (collected %d frames)", err, len(frames))
			}
			return frames, ce.Code
		}
		frames = append(frames, data)
	}
}

// decodeExitFrame 断言帧为 EXIT 帧（'X' 类型字节）并解码 JSON 载荷。
func decodeExitFrame(t *testing.T, frame []byte) proto.ExitPayload {
	t.Helper()
	if len(frame) == 0 {
		t.Fatalf("empty frame, want EXIT 'X'(%#x)", proto.Exit)
	}
	if frame[0] != proto.Exit {
		t.Fatalf("frame[0] = %#x, want EXIT 'X'(%#x)", frame[0], proto.Exit)
	}
	var ep proto.ExitPayload
	if err := json.Unmarshal(frame[1:], &ep); err != nil {
		t.Fatalf("EXIT payload unmarshal: %v", err)
	}
	return ep
}

// TestExitFrameBroadcast（SESS-03 主干，D-10 广播序列）：Writable:true 起
// `bash --norc --noprofile`（无 rc 启动输出干扰帧序断言），dialHello 双客户端
// （80x24 owner rw / 100x40 D-07 降级 ro——rw+ro 混合在线）→ A 写 INPUT
// "exit 42\r" → 两端各自读至 CloseError：最后到达的数据帧为 EXIT 帧（帧序
// 断言——EXIT 必先于 onclose/1000）且两端 EXIT 帧体逐字节一致（exit_code==42、
// message 逐字等），CloseError.Code==1000；waitExit(42)（退出码传递语义不变）。
func TestExitFrameBroadcast(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"bash", "--norc", "--noprofile"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	cB, _ := dialHello(t, ctx, wsURL, 100, 40)

	// 触发子进程退出：A（owner rw）发 exit 42——PTY 规范模式 ICRNL 把 \r 转 \n
	//（真实终端按键同形）。
	if err := cA.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("exit 42\r")...)); err != nil {
		t.Fatalf("write INPUT on A: %v", err)
	}

	// 两端各自读至 CloseError（并行 Close 下 B 帧在管道缓冲，顺序读取两端无竞态）。
	framesA, codeA := readExitClose(t, ctx, cA)
	framesB, codeB := readExitClose(t, ctx, cB)
	if codeA != websocket.StatusNormalClosure || codeB != websocket.StatusNormalClosure {
		t.Fatalf("close codes = %d / %d, want both %d (1000)", codeA, codeB, websocket.StatusNormalClosure)
	}

	// 帧序断言：最后到达的数据帧为 EXIT 帧——Read 仅在关闭帧到达后返回错误，
	// 末帧数据帧先于 1000 上线即「EXIT 必先于 onclose 到达」的客户端侧证据。
	if len(framesA) == 0 || len(framesB) == 0 {
		t.Fatalf("collected frames A=%d B=%d, want >=1（EXIT 帧缺失）", len(framesA), len(framesB))
	}
	lastA, lastB := framesA[len(framesA)-1], framesB[len(framesB)-1]
	epA := decodeExitFrame(t, lastA)
	// 双端 EXIT 帧体逐字节一致（rw+ro 全员同帧——终结无权限语义，无分档）。
	if !bytes.Equal(lastA, lastB) {
		t.Fatalf("EXIT frames differ: A=%q B=%q（ro/rw 全员同帧违反）", lastA, lastB)
	}
	if epA.ExitCode != 42 {
		t.Fatalf("EXIT exit_code = %d, want 42", epA.ExitCode)
	}
	if epA.Message != "The process exited with code 42." {
		t.Fatalf("EXIT message = %q, want %q（UI-SPEC 文案表逐字）", epA.Message, "The process exited with code 42.")
	}

	// 广播后 exitf 以子进程退出码收口（D-10 退出码传递语义不变）。
	waitExit(t, exitCh, 42)
}

// TestExitFrameSignal（SESS-03 信号死亡形态，D-09 + RESEARCH Pitfall 3）：
// argv `sh -c 'sleep 1; kill -HUP $$'`——sleep 保 attach 窗口，kill -HUP $$
// 自发信号死亡 → 客户端收 EXIT{exit_code:-1} 且 message 含大写 "SIGHUP"
// （显式大写名映射断言——裸用 Signal.String() 会产出小写描述词 "hangup"）
// → 1000；exitf 捕获桩收到 -1（ExitError ExitCode 语义；os.Exit 截断 255
// 只在真实二进制出现，UAT 层 06-06 断言）。
func TestExitFrameSignal(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"sh", "-c", "sleep 1; kill -HUP $$"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	frames, code := readExitClose(t, ctx, c)
	if code != websocket.StatusNormalClosure {
		t.Fatalf("close code = %d, want %d (1000)", code, websocket.StatusNormalClosure)
	}
	if len(frames) == 0 {
		t.Fatal("no frames collected——EXIT 帧缺失")
	}
	ep := decodeExitFrame(t, frames[len(frames)-1])
	if ep.ExitCode != -1 {
		t.Fatalf("EXIT exit_code = %d, want -1（信号死亡不得粉饰为正常退出码）", ep.ExitCode)
	}
	if !strings.Contains(ep.Message, "SIGHUP") {
		t.Fatalf("EXIT message = %q, want 含大写信号名 SIGHUP（Pitfall 3 显式映射）", ep.Message)
	}

	// exitf 捕获桩收到 -1（ExitError.ExitCode() 信号死亡语义同源传递）。
	waitExit(t, exitCh, -1)
}
