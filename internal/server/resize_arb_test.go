package server_test

// resize_arb_test.go 锁定 MULTI-04 resize 仲裁的黑盒集成行为（VALIDATION
// 05-01-04）：all 模式异尺寸双端 min-rect（Getsize 实证）+ 2→1 恢复
// last-wins + 防抖合并（窗口内未应用、窗口后应用为最后上报值）+ owner 模式
// 参与集分层（旁观者尺寸不影响 PTY、递补后新 owner 尺寸接管）+ ro RESIZE
// 忽略闸（D-09 第二闸）。断言通道 = creack/pty Getsize 读回 PTY 真实尺寸
//（RESEARCH 核实 winsize.go:18）。arbitrate 纯函数表测在同目录 resize_test.go
//（package server 白盒——内部类型不导出；Go 单文件单 package 约束使两测试
// 分文件落位，SUMMARY 登记 plan 字面偏差）。
//
// 参数序陷阱双保险（review MEDIUM 处置）：creack/pty Getsize 返回 (rows, cols)，
// 与 sess.Resize(cols, rows) 入参序相反（io_test.go:24-25 注释纪律的仲裁断言
// 映射）——本文件尺寸读回一律经 ptySize helper 归一为 (cols, rows)，断言处
// 切勿按 Getsize 原生 (rows, cols) 返回序误读。

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	creackpty "github.com/creack/pty"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// startResizeServer 装配形态照抄 e2e_test.go startTestServerWith（pty.Start +
// server.New + net.Listen + http.Serve），返回值加挂 sess 供 creack/pty Getsize
// 读回仲裁结果。不改 startTestServerWith 签名（其全部调用点在 e2e_test.go 与
// 本 phase 新增测试文件，零波及——checker W3 方案 b 爆炸半径裁断）。
func startResizeServer(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, sess *pty.Session) {
	t.Helper()
	sess, err := pty.Start(argv)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh = make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, opts)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go http.Serve(ln, srv.Handler())
	return exitCh, "ws://" + ln.Addr().String() + "/ws", sess
}

// ptySize 读回当前 PTY 尺寸并归一为 (cols, rows)。注意：creack/pty Getsize
// 返回 (rows, cols)——与 sess.Resize(cols, rows) 入参序相反（io_test.go:24-25
// 参数序纪律的仲裁断言映射），此处统一换算，调用方按 (cols, rows) 断言。
func ptySize(t *testing.T, sess *pty.Session) (cols, rows int) {
	t.Helper()
	r, c, err := creackpty.Getsize(sess.Master)
	if err != nil {
		t.Fatalf("pty.Getsize: %v", err)
	}
	return c, r // (rows, cols) 原生返回序 → (cols, rows) 归一
}

// pollSize 轮询断言 PTY 尺寸在 5s 内变为 (wantCols, wantRows)（100ms 步进——
// e2e 先例形态；attach/detach 即时重算与防抖到期重算均为异步通道，直接断言
// 有竞态）。
func pollSize(t *testing.T, sess *pty.Session, wantCols, wantRows int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		cols, rows := ptySize(t, sess)
		if cols == wantCols && rows == wantRows {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("PTY size = %dx%d, want %dx%d within 5s", cols, rows, wantCols, wantRows)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// sendResize 发送 RESIZE 帧（{"cols":C,"rows":R}——proto.DecodeResize 钳制
// [1,1000] 在上游生效，仲裁器输入已钳语义）。
func sendResize(t *testing.T, ctx context.Context, c *websocket.Conn, cols, rows int) {
	t.Helper()
	payload, err := json.Marshal(struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	}{Cols: cols, Rows: rows})
	if err != nil {
		t.Fatalf("marshal RESIZE: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Resize}, payload...)); err != nil {
		t.Fatalf("write RESIZE: %v", err)
	}
}

// TestResizeArbitration（VALIDATION 05-01-04，MULTI-04 行为锁定）：异尺寸多
// 客户端共存的渲染正确性——min-rect / 2→1 last-wins / 防抖合并 / D-09 参与集
// 分层与 ro 忽略双闸，全部经 Getsize 读回 PTY 真实尺寸实证。
func TestResizeArbitration(t *testing.T) {
	// all 模式参与集：全部生效 rw 端（D-09 矩阵第二行）。A(132x43)/B(80x24)
	// 双 rw attach → PTY = min(132,80)×min(43,24) = 80x24（任何参与端窗口
	// ≥ PTY 尺寸，min-rect 不变量）；B 断开 → 2→1 恢复 last-wins（剩余者
	// A 尺寸 132x43）。
	t.Run("all模式min-rect与2to1恢复", func(t *testing.T) {
		exitCh, wsURL, sess := startResizeServer(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			WritePolicy: server.WritePolicyAll,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cA, modeA := dialHello(t, ctx, wsURL, 132, 43)
		if modeA != proto.ModeRW {
			t.Fatalf("A welcome mode = %q, want %q", modeA, proto.ModeRW)
		}
		pollSize(t, sess, 132, 43) // attach 即时重算：单成员 last-wins

		cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
		if modeB != proto.ModeRW {
			t.Fatalf("B welcome mode = %q, want %q", modeB, proto.ModeRW)
		}
		// 双成员 → min-rect：min(132,80)×min(43,24) = 80x24（attach 即时重算不防抖）。
		pollSize(t, sess, 80, 24)

		// B 断开 → detach 即时重算 → 2→1 恢复 last-wins（剩余者 A 尺寸）。
		cB.CloseNow()
		pollSize(t, sess, 132, 43)

		cA.Close(websocket.StatusNormalClosure, "")
		assertNoExit(t, exitCh)
	})

	// 防抖合并（PITFALLS Pitfall 10 SIGWINCH 风暴防线）：ResizeDebounce 覆写
	// 200ms，owner 单成员（last-wins）在 100ms 内连发 3 次异尺寸 RESIZE →
	// 窗口内（+100ms 时点）Getsize 未变（单 time.Timer reset 合并，未逐次
	// 应用）→ 轮询至 2s Getsize == 最后上报值（合并为一次应用）。
	t.Run("防抖合并", func(t *testing.T) {
		exitCh, wsURL, sess := startResizeServer(t, []string{"/bin/cat"}, server.Options{
			Writable:       true,
			ResizeDebounce: 200 * time.Millisecond,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
		if modeA != proto.ModeRW {
			t.Fatalf("A welcome mode = %q, want %q（owner 模式默认，单成员 last-wins）", modeA, proto.ModeRW)
		}
		pollSize(t, sess, 80, 24) // Hello 首尺寸生效基线

		// 100ms 内连发 3 次异尺寸 RESIZE（loopback 毫秒级送达）。
		sendResize(t, ctx, cA, 100, 40)
		sendResize(t, ctx, cA, 90, 30)
		sendResize(t, ctx, cA, 120, 50)

		// +100ms 时点（< 200ms 防抖窗）：Getsize 未变——窗口内未应用（合并中）。
		time.Sleep(100 * time.Millisecond)
		if cols, rows := ptySize(t, sess); cols != 80 || rows != 24 {
			t.Fatalf("防抖窗口内 PTY size = %dx%d, want 80x24（单 timer reset 合并，窗口内未应用）", cols, rows)
		}
		// 窗口后：合并为一次应用，应用值 = 最后上报 120x50（轮询 2s 上限）。
		deadline := time.Now().Add(2 * time.Second)
		for {
			cols, rows := ptySize(t, sess)
			if cols == 120 && rows == 50 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("防抖窗口后 PTY size = %dx%d, want 120x50（合并为一次应用，最后上报值）", cols, rows)
			}
			time.Sleep(50 * time.Millisecond)
		}

		cA.Close(websocket.StatusNormalClosure, "")
		assertNoExit(t, exitCh)
	})

	// owner 模式参与集分层 + ro 忽略闸（D-09 矩阵第一/四行）：A(owner 132x43)/
	// B(降级 ro 80x24) → Getsize 跟随 A（132x43 而非 min——ro 旁观者永不参与、
	// 不影响可写端 PTY 尺寸）；B 发 RESIZE → Getsize 不变（D-09 第二闸：服务端
	// 直接忽略）；A 断开 → B 递补升格 → Getsize 切到 B 的 Hello 登记尺寸 80x24
	//（递补后新 owner 尺寸接管）。
	t.Run("owner模式参与集与ro忽略闸", func(t *testing.T) {
		exitCh, wsURL, sess := startResizeServer(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			WritePolicy: server.WritePolicyOwner,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cA, modeA := dialHello(t, ctx, wsURL, 132, 43)
		if modeA != proto.ModeRW {
			t.Fatalf("A welcome mode = %q, want %q（首个 rw attach 立 owner）", modeA, proto.ModeRW)
		}
		pollSize(t, sess, 132, 43) // owner 单成员 last-wins

		cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
		if modeB != proto.ModeRO {
			t.Fatalf("B welcome mode = %q, want %q（D-07 降级 ro 旁观者）", modeB, proto.ModeRO)
		}
		// B 为 ro 旁观者 → 不参与仲裁：PTY 保持 132x43 而非 min(80,24)。B 的
		// Welcome 已读回（attach 已落定）——若 B 误参与，attach 即时重算早已应用
		// min-rect；300ms 余量（> 50ms 防抖窗）后断言不变。
		time.Sleep(300 * time.Millisecond)
		if cols, rows := ptySize(t, sess); cols != 132 || rows != 43 {
			t.Fatalf("ro 旁观者 attach 后 PTY size = %dx%d, want 132x43（D-09：旁观者尺寸永不影响可写端）", cols, rows)
		}

		// D-09 第二闸：B（降级 ro）发 RESIZE → 服务端直接忽略，Getsize 不变。
		sendResize(t, ctx, cB, 100, 30)
		time.Sleep(300 * time.Millisecond) // > 50ms 防抖窗——若误受理早已应用
		if cols, rows := ptySize(t, sess); cols != 132 || rows != 43 {
			t.Fatalf("ro 端 RESIZE 后 PTY size = %dx%d, want 132x43（D-09 第二闸：服务端直接忽略）", cols, rows)
		}

		// A 断开 → detach → FIFO 递补升格 B → 参与集切换（sizes 只留新 owner）
		// → Getsize 切到 B 的 Hello 登记尺寸 80x24（递补后新 owner 尺寸接管）。
		//（G-05-1 起升格 Welcome 与重算推送帧均落 cB 未被读流，Getsize 断言面不受影响。）
		cA.CloseNow()
		pollSize(t, sess, 80, 24)

		cB.Close(websocket.StatusNormalClosure, "")
		assertNoExit(t, exitCh)
	})

	// 运行期尺寸变化推送（G-05-1 运行期尺寸下发通道，05-10）：owner 模式
	// A(owner rw 80x24)/B(降级 ro 60x20)；A 窗口 resize（RESIZE 上报）经 50ms
	// 防抖重算后，recalcNow 的 last 变化分支向全部在线客户端推送携新会话尺寸的
	// Welcome——两端（含 ro 旁观者与上报者自身）各在 5s 窗口收到第二帧 Welcome
	// cols/rows==100/30；B 的推送帧 mode==ro（按各端当前生效 mode 组帧，D-13
	// 双档纪律在推送通道不漂移）；pollSize 附带锁定 PTY 跟随（既有断言形态）。
	//
	// 回归自检（写进注释不断言）：S1 形态（owner 模式 A rw + B ro 降级，B 不参与）
	// attach 无 resize——A attach 的 recalcNow 推送落在空注册表（升档重排后
	// attach 者尚未 registerLocked），B attach 不参与不重算，last 不变零推送，
	// accumPayload 的「窗口内无第二帧 Welcome」断言（multi_test.go）不受影响；
	// TestResizeArbitration 既有三子测的 dialHello 尺寸组合（132x43/80x24 等）
	// 触发的推送帧均落在未被读流的连接上，Getsize 断言面不受影响。
	// 禁止负向静默窗口断言（「resize 前 B 无第二帧 Welcome」类——防抖/调度时序
	// 引入 flaky 面，省）。
	t.Run("运行期尺寸变化推送", func(t *testing.T) {
		exitCh, wsURL, sess := startResizeServer(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			WritePolicy: server.WritePolicyOwner,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
		if modeA != proto.ModeRW {
			t.Fatalf("A welcome mode = %q, want %q（首个 rw attach 立 owner）", modeA, proto.ModeRW)
		}
		pollSize(t, sess, 80, 24) // owner 单成员 last-wins 基线

		cB, modeB := dialHello(t, ctx, wsURL, 60, 20)
		if modeB != proto.ModeRO {
			t.Fatalf("B welcome mode = %q, want %q（D-07 降级 ro 旁观者）", modeB, proto.ModeRO)
		}

		// 双端 attach Welcome 均被 dialHello 消费；武装第二帧 Welcome 读取
		//（Pitfall 2 竞速形态——客户端 Read 永不带 deadline ctx）。
		chA := readUntilWelcome(cA)
		chB := readUntilWelcome(cB)

		// A（owner）上报新窗口尺寸 → 50ms 防抖 → recalcNow → last 变化 → 双端推送。
		sendResize(t, ctx, cA, 100, 30)
		// B（ro 旁观者）：推送按各端当前 mode 组帧——mode 恒 ro，cols/rows 为新
		// 会话尺寸 100x30（旁观者约束渲染的数据通道，G-05-1 服务端半侧闭合）。
		select {
		case wm := <-chB:
			if wm == nil {
				t.Fatal("B read terminated before dims-push Welcome（ro 旁观者运行期推送丢失）")
			}
			if wm["mode"] != proto.ModeRO {
				t.Fatalf("B dims-push Welcome mode = %v, want %q（推送按各端当前生效 mode 组帧）", wm["mode"], proto.ModeRO)
			}
			if wm["cols"] != float64(100) || wm["rows"] != float64(30) {
				t.Fatalf("B dims-push Welcome dims = %vx%v, want 100x30", wm["cols"], wm["rows"])
			}
		case <-time.After(5 * time.Second):
			t.Fatal("B 未在 5s 内收到 dims-push Welcome——ro 旁观者运行期尺寸下发失败（G-05-1）")
		}
		// A（上报者自身）：同收 100x30（上报者的会话尺寸确认回执——前端无需
		// 特判自身上报路径）。
		select {
		case wm := <-chA:
			if wm == nil {
				t.Fatal("A read terminated before dims-push Welcome（上报者自身推送丢失）")
			}
			if wm["mode"] != proto.ModeRW {
				t.Fatalf("A dims-push Welcome mode = %v, want %q", wm["mode"], proto.ModeRW)
			}
			if wm["cols"] != float64(100) || wm["rows"] != float64(30) {
				t.Fatalf("A dims-push Welcome dims = %vx%v, want 100x30", wm["cols"], wm["rows"])
			}
		case <-time.After(5 * time.Second):
			t.Fatal("A 未在 5s 内收到 dims-push Welcome——上报者自身运行期尺寸下发失败（G-05-1）")
		}
		// PTY 跟随断言（既有形态）：防抖重算后 PTY 实际尺寸 = 推送的会话尺寸。
		pollSize(t, sess, 100, 30)

		cA.Close(websocket.StatusNormalClosure, "")
		cB.Close(websocket.StatusNormalClosure, "")
		assertNoExit(t, exitCh)
	})
}
