package server_test

// exit_test.go 锁定 SESS-03 端到端行为（06-01 tracer，06-VALIDATION 06-01-01）：
// 子进程退出后全部在线客户端先收类型化 EXIT 帧（'X'，{"exit_code":N,"message":M}）
// 再收 1000 正常关闭（D-08/D-09/D-10）——帧序（EXIT 必先于 1000 到达）、双端
// 帧体逐字节一致（rw+ro 混合全员同帧，终结无权限语义）、退出码传递、信号死亡
// exit_code=-1 + 大写信号名文案四行为。helper 复用 e2e_test.go 零改动
//（startTestServerWith/dialHello/waitExit 同包先例，06-PATTERNS exact）。
//
// 14-01 双模式分叉表改造（D-01/D-02，PITFALLS :385 行「exit_test（EXIT 广播）
// = 双模式断言分叉」）：装配统一换 newTestServer 小族（harness_test.go），
// t.Run("mode=shared"/"mode=per-client") 双跑；断言分叉点显式成表——shared 列
// 期望值与改造前逐字一致（v1.0 零回归证据本体，D-02 红线），per-client 列 =
// EXIT 私有化 + 他端零感知（PC-04 语义）。per-client 列夹具复用 perclient_test.go
// 既有泵源件（readPump/drainQuiet/accumFramesUntil——客户端 Read 永不带
// per-read deadline，静默窗一律 select + time.After 竞速形态，perclient_test.go:3-14
// 夹具纪律红线）。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
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
//
// 14-01 双模式分叉表：shared 列 = 上述 v1.0 广播语义逐字；per-client 列 =
// EXIT 私有化（PC-04）——A（属主）收私有 EXIT{42} + 1000，B（自有会话）在
// 静默窗口内零帧到达且连接保持，窗后 echo 存活标记（服务端续跑 + B 会话无
// 扰动双重实证）；exitf 静默（per-client 第二终结源仅经 exit-when-empty/
// Shutdown 触发——PC-02/PC-03 语义，B 仍在线时无路径）。
func TestExitFrameBroadcast(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"bash", "--norc", "--noprofile"}, nil)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cA, _ := dialHello(t, ctx, wsURL, 80, 24)
			cB, _ := dialHello(t, ctx, wsURL, 100, 40)

			// 触发子进程退出：A（owner rw）发 exit 42——PTY 规范模式 ICRNL 把 \r 转 \n
			//（真实终端按键同形）。
			if err := cA.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("exit 42\r")...)); err != nil {
				t.Fatalf("write INPUT on A: %v", err)
			}

			// 断言分叉表（D-02，mode → expected 显式表——PITFALLS :264 形态）：
			//	broadcast（shared，v1.0 逐字）= true：EXIT 帧广播全员——rw+ro 混合
			//	  双端末帧逐字节一致 + 双 1000 + exitf(42)。
			//	broadcast（per-client，PC-04）= false：EXIT 私有化——属主端收私有
			//	  EXIT{42} + 1000，他端零帧零错误零扰动。
			broadcast := mode == server.SessionModeShared
			if !broadcast {
				// per-client 列：B 端读取泵先行（readPump 泵源——静默窗观测通道）。
				resCh := make(chan frameRes, 8)
				quit := make(chan struct{})
				defer close(quit)
				go readPump(ctx, cB, resCh, quit)
				drainQuiet(t, resCh, 300*time.Millisecond) // B 自身会话启动提示符收干（假阳性前置排除）

				framesA, codeA := readExitClose(t, ctx, cA)
				if codeA != websocket.StatusNormalClosure {
					t.Fatalf("A close code = %d, want %d (1000)", codeA, websocket.StatusNormalClosure)
				}
				if len(framesA) == 0 {
					t.Fatalf("collected frames A=%d, want >=1（属主私有 EXIT 帧缺失，PC-04）", len(framesA))
				}
				epA := decodeExitFrame(t, framesA[len(framesA)-1])
				if epA.ExitCode != 42 {
					t.Fatalf("EXIT exit_code = %d, want 42", epA.ExitCode)
				}
				if epA.Message != "The process exited with code 42." {
					t.Fatalf("EXIT message = %q, want %q（UI-SPEC 文案表逐字）", epA.Message, "The process exited with code 42.")
				}

				// B 端 1.5s 静默窗：任何帧/错误到达即 FAIL（他端零感知强形态——A 的
				// 终结余波连 OUTPUT 级都不得泄漏到 B；select + time.After 竞速形态，
				// 客户端 Read 永不带 per-read deadline）。
				select {
				case r := <-resCh:
					t.Fatalf("B 端在 A 终结后静默窗内收到帧/错误——EXIT 串台或余波扰动（PC-04）: data=%q err=%v", r.data, r.err)
				case <-time.After(1500 * time.Millisecond):
				}

				// 窗后 B echo 唯一标记回读（服务端续跑 + B 会话无扰动）。
				if err := cB.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("echo BEXITOK_7q2\r")...)); err != nil {
					t.Fatalf("write INPUT on B: %v", err)
				}
				// 行首锚定容忍两形态：(?:\$ )? PS1 交错（perclient_test.go
				// TestPerClientExitPrivate42 CI run 33843785651 同源竞态先例）；
				// (?:\x1b\[\?2004l\r)? bash 5.2 bracketed-paste 关闭序列+CR——本测
				// argv 是 bash（先例均 sh 无 readline 序列），CI ubuntu bash 5.2
				// readline 接受行后输出 \x1b[?2004l\r，结果行前缀为 \r 非 \n，
				// (?m)^ 只认 \n 会漏配（实证：run 34198964828 累积串
				// "echo BEXITOK_7q2\r\n\x1b[?2004l\rBEXITOK_7q2\r\n..."超时；
				// 本地 bash 4.4 无此序列故全绿——环境差异）。回显行带 "echo "
				// 前缀仍不误命中，锚定强度不变。
				accumFramesUntil(t, resCh, regexp.MustCompile(`(?m)^(?:\x1b\[\?2004l\r)?(?:\$ )?BEXITOK_7q2\r?$`))

				// exitf 静默：B 仍在线，per-client 无第二终结源路径（PC-02/03）。
				assertNoExit(t, exitCh)
				_ = cB.Close(websocket.StatusNormalClosure, "") // 已收齐标记——幂等收口
				return
			}

			// shared 列（v1.0 期望值逐字搬入）：两端各自读至 CloseError（并行
			// Close 下 B 帧在管道缓冲，顺序读取两端无竞态）。
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
			// 尾部余波 W 帧剥离：A 的 Close(1000) 触发其 reader detach →
			// removeMember(A) → 嵌套 recalcNow（resize.go:186-197 注释自证）向留存
			// 端 B 补发尺寸推送 W——与 lifecycle goroutine(B) 的 Close(1000) 竞态
			//（EXIT 直写绕过 outbox（server.go:1593 写序论证），W 经 outbox writer
			// drain，两条并发写路径谁先上 wire 不定）。B 末帧可为 X 或余波 W（darwin
			// CI run 34201354818 实证 [X, W(100x40), 1000] 形态——A 移除后单成员
			// last-wins 仲裁 100x40）。剥离后比较：「EXIT 必先于 1000」（readExitClose
			// 结构性质——全部数据帧先于关闭帧到达）与「ro/rw 全员同帧」两断言本质
			// 零损失，仅免疫关停余波噪声。
			trimTrailingWelcome := func(frames [][]byte) []byte {
				for len(frames) > 0 && len(frames[len(frames)-1]) > 0 && frames[len(frames)-1][0] == proto.Welcome {
					frames = frames[:len(frames)-1]
				}
				if len(frames) == 0 {
					return nil
				}
				return frames[len(frames)-1]
			}
			lastA, lastB := trimTrailingWelcome(framesA), trimTrailingWelcome(framesB)
			if lastA == nil || lastB == nil {
				t.Fatalf("A/B 帧序剥余波后为空（EXIT 帧缺失）：A=%d B=%d", len(framesA), len(framesB))
			}
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
		})
	}
}

// TestExitFrameSignal（SESS-03 信号死亡形态，D-09 + RESEARCH Pitfall 3）：
// argv `sh -c 'sleep 1; kill -HUP $$'`——sleep 保 attach 窗口，kill -HUP $$
// 自发信号死亡 → 客户端收 EXIT{exit_code:-1} 且 message 含大写 "SIGHUP"
// （显式大写名映射断言——裸用 Signal.String() 会产出小写描述词 "hangup"）
// → 1000；exitf 捕获桩收到 -1（ExitError ExitCode 语义；os.Exit 截断 255
// 只在真实二进制出现，UAT 层 06-06 断言）。
//
// 14-01 双模式分叉表：wire 面（EXIT{-1, SIGHUP} + 1000）两模式同值——单客户端
// 形态下广播/私有化不可观测，他端零感知分叉由 TestExitFrameBroadcast 的
// per-client 列承载；exitf 面分叉——shared = lifecycle 收口 -1（原断言逐字），
// per-client = 静默（无第二终结源，服务端续跑；信号死亡私有 EXIT 本体与
// perclient_test.go TestPerClientExitSignalMinus1 一致，本测双跑锁文案与
// 退出码语义两模式同值）。
func TestExitFrameSignal(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"sh", "-c", "sleep 1; kill -HUP $$"}, nil)

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

			// 断言分叉表（D-02）：exitf 收口面。shared = -1（ExitError.ExitCode()
			// 信号死亡语义同源传递，原断言逐字）；per-client = 静默（第二终结源
			// 仅经 exit-when-empty/Shutdown——PC-02/03）。
			if mode == server.SessionModePerClient {
				assertNoExit(t, exitCh)
				return
			}
			waitExit(t, exitCh, -1)
		})
	}
}
