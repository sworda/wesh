package server_test

// stopseq_test.go 锁定 07-04（OPS-04，D-22）exit-when-empty 收口路径的可配
// stop-signal 序列：--stop-signal TERM 时收口向子进程进程组发 SIGTERM 而非
// SIGHUP（trap 捕获以特异退出码作证）；--stop-timeout > 0 时 stop-signal 被
// 忽略后 AfterFunc 异步补发 SIGKILL（trap "" 忽略形态 + 时序双断言）。
// helper 复用 e2e_test.go 同包零改动（startTestServerWith/dialHello/waitExit/
// assertNoExit）；默认 HUP 零漂移由 emptyexit_test.go 既有六测试（零值 Options
// 经 New 兜底 SIGHUP）承接。
//
// 客户端 Read 永不带 deadline ctx（Pitfall 2 回归锁）——静默窗口一律 select +
// time.After 竞速形态。

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/server"
)

// TestExitWhenEmptyStopSignalTERM（07-04，OPS-04，D-22）：Options.StopSignal=SIGTERM
// 时，注册表空触发收口向子进程进程组发 SIGTERM——argv
// `sh -c 'trap "exit 43" TERM; sleep 100'`：TERM 命中 sh 的 trap 以特异退出码
// 43 退出（同组的 sleep 被同一 TERM 默认动作杀死后 sh 执行 trap）→ exitf(43)
// 为 TERM 送达的结构证据（若误发 SIGHUP——旧行为——sh 无 HUP trap 直接信号
// 死亡，exitf 收 -1）。
func TestExitWhenEmptyStopSignalTERM(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"sh", "-c", `trap "exit 43" TERM; sleep 100`}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 0,
		StopSignal:         syscall.SIGTERM,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	c.Close(websocket.StatusNormalClosure, "")

	// TERM 送达 → trap exit 43 → exitf(43) 恰好一次（200ms 静默锁定无第二次）。
	waitExit(t, exitCh, 43)
	assertNoExit(t, exitCh)
}

// TestExitWhenEmptyStopTimeoutKills（07-04，OPS-04，D-22 + RESEARCH Pitfall 8）：
// Options.StopSignal=SIGTERM + StopTimeout=400ms 时，子进程 trap 忽略 TERM
//（`trap "" TERM`）→ stop-timeout 到期 AfterFunc 异步补发 SIGKILL → 信号死亡
// → exitf(-1)（accept-255 同常量）。时序双断言：stop-timeout 前 exitf 静默
//（TERM 被忽略无自然死亡路径——sleep 100 自然存活 100s）；其后 5s 内收 -1
//（KILL 补发的结构证据——无补发则进程必然存活到测试护栏翻车）。
func TestExitWhenEmptyStopTimeoutKills(t *testing.T) {
	const stopTimeout = 400 * time.Millisecond
	exitCh, wsURL := startTestServerWith(t, []string{"sh", "-c", `trap "" TERM; sleep 100`}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 0,
		StopSignal:         syscall.SIGTERM,
		StopTimeout:        stopTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
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
