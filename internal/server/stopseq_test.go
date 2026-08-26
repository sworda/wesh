package server_test

// stopseq_test.go 锁定 07-04（OPS-04，D-22）exit-when-empty 收口路径的可配
// stop-signal 序列：--stop-signal TERM 时收口向子进程进程组发 SIGTERM 而非
// SIGHUP（trap 捕获以特异退出码作证）；--stop-timeout > 0 时 stop-signal 被
// 忽略后 AfterFunc 异步补发 SIGKILL（trap "" 忽略形态 + 时序双断言）。
// helper 复用 e2e_test.go 同包零改动（startTestServerWith/dialHello/waitExit/
// assertNoExit）；默认 HUP 零漂移由 emptyexit_test.go 既有六测试（零值 Options
// 经 New 兜底 SIGHUP）承接。
//
// 夹具纪律（2026-08-26 两轮实证——初版探针误判后经真实二进制冒烟修正）：
//   - trap 安装与 detach 信号之间存在竞态——子进程经 setsid+exec 后安装 trap
//     需要非零时间，dialHello 完成不等价 trap 已就位（组合运行负载下实测命中：
//     TERM 先于 trap 到达，sh 按默认动作死亡收 -1）。两测试均以落盘标记文件
//     同步「trap 已安装」（Phase 01-03 决策先例：stdout 标记在 WS 断开后被
//     onChunk 丢弃不可观测，落盘标记是信号类断言的既定形态）。本竞态是
//     KILL 测初版失败的唯一根因。
//   - `trap "" TERM` 的恒活机理是 SIG_IGN 跨 exec 持久（POSIX：被忽略的信号
//     掩码在 fork/exec 后保持）——sh 忽略 TERM 时 fork+exec 的 sleep 同样继承
//     SIG_IGN，整组免疫 TERM（真实二进制冒烟实证：stop-timeout=1s 时 wesh 在
//     close+1002ms 经 KILL 退出 255，而非 TERM 后自然退出）。trap "exit 43"
//     形态相反：捕获型 disposition 在 exec 时复位默认，sleep 被 TERM 杀死、
//     sh 执行 trap 退出 43——两形态互补锁定送达与忽略两语义。
//   - KILL 测取 `while :; do sleep 10; done` 循环形态：不依赖 SIG_IGN 继承
//     机理的显式恒活（即使未来 shell 行为差异也更易诊断）。
//
// 客户端 Read 永不带 deadline ctx（Pitfall 2 回归锁）——静默窗口一律 select +
// time.After 竞速形态。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/server"
)

// waitMarker 轮询等待子进程落盘标记出现（5s 护栏）——trap 安装完成的同步点
// （落盘标记先例，见文件头夹具纪律）。
func waitMarker(t *testing.T, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("trap 安装标记 %s 5s 内未出现——子进程夹具未就位", marker)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestExitWhenEmptyStopSignalTERM（07-04，OPS-04，D-22）：Options.StopSignal=SIGTERM
// 时，注册表空触发收口向子进程进程组发 SIGTERM——argv
// `sh -c 'trap "exit 43" TERM; touch M; sleep 100'`（探针实证形态）：TERM 命中
// sh 的 trap 以特异退出码 43 退出（同组的 sleep 被同一 TERM 默认动作杀死后 sh
// 执行 trap）→ exitf(43) 为 TERM 送达的结构证据（若误发 SIGHUP——旧行为——sh
// 无 HUP trap 直接信号死亡，exitf 收 -1）。
func TestExitWhenEmptyStopSignalTERM(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "trap-armed")
	exitCh, wsURL := startTestServerWith(t, []string{"sh", "-c", fmt.Sprintf(`trap "exit 43" TERM; touch %s; sleep 100`, marker)}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 0,
		StopSignal:         syscall.SIGTERM,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	waitMarker(t, marker) // trap 已安装——detach 信号不再竞态
	c.Close(websocket.StatusNormalClosure, "")

	// TERM 送达 → trap exit 43 → exitf(43) 恰好一次（200ms 静默锁定无第二次）。
	waitExit(t, exitCh, 43)
	assertNoExit(t, exitCh)
}

// TestExitWhenEmptyStopTimeoutKills（07-04，OPS-04，D-22 + RESEARCH Pitfall 8）：
// Options.StopSignal=SIGTERM + StopTimeout=400ms 时，子进程 trap 忽略 TERM
// （`trap "" TERM` + while 循环显式恒活夹具——SIG_IGN 跨 exec 持久机理与
// 夹具选型见文件头夹具纪律）→ stop-timeout 到期 AfterFunc 异步补发 SIGKILL →
// 信号死亡 → exitf(-1)（accept-255 同常量）。
// 时序双断言：stop-timeout 前 exitf 静默（TERM 被忽略无自然死亡路径——循环
// 夹具只有 KILL 能致死）；其后 5s 内收 -1（KILL 补发的结构证据——无补发则
// 进程必然存活到测试护栏翻车）。
func TestExitWhenEmptyStopTimeoutKills(t *testing.T) {
	const stopTimeout = 400 * time.Millisecond
	marker := filepath.Join(t.TempDir(), "trap-armed")
	exitCh, wsURL := startTestServerWith(t, []string{"sh", "-c", fmt.Sprintf(`trap "" TERM; touch %s; while :; do sleep 10; done`, marker)}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 0,
		StopSignal:         syscall.SIGTERM,
		StopTimeout:        stopTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	waitMarker(t, marker) // trap 已安装——detach 信号不再竞态
	c.Close(websocket.StatusNormalClosure, "")

	// stop-timeout 前 300ms 时点静默——TERM 被 trap 忽略，不存在自然死亡路径
	//（此时点收码即序列错误：TERM 未走忽略语义或 KILL 提前补发）。
	select {
	case code := <-exitCh:
		t.Fatalf("exitf(%d) 在 stop-timeout(%v) 前到达——TERM 应被 trap 忽略，无致死路径", code, stopTimeout)
	case <-time.After(300 * time.Millisecond):
	}

	// 400ms 到期 AfterFunc 补 SIGKILL（不占 hubMu、ESRCH 幂等）→ exitf(-1)
	// 恰好一次。
	waitExit(t, exitCh, -1)
	assertNoExit(t, exitCh)
}
