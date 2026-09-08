package server_test

// stopseq_test.go 锁定 07-04（OPS-04，D-22）exit-when-empty 收口路径的可配
// stop-signal 序列：--stop-signal TERM 时收口向子进程进程组发 SIGTERM 而非
// SIGHUP（trap 捕获以特异退出码作证）；--stop-timeout > 0 时 stop-signal 被
// 忽略后 AfterFunc 异步补发 SIGKILL（trap "" 忽略形态 + 时序双断言）。
// helper 复用 e2e_test.go 同包零改动（startTestServerWith/dialHello/waitExit/
// assertNoExit）；默认 HUP 零漂移由 emptyexit_test.go 既有六测试（零值 Options
// 经 New 兜底 SIGHUP）承接。
//
// 14-01 双模式分叉表改造（D-01/D-02，PITFALLS :387 行「stopseq = 双模式断言
// 分叉：per-client=N 组信号」）：装配统一换 newTestServer 小族（harness_test.go）
// t.Run 双跑。per-client 列语义（本列 N=1 形态）：断开/终结形态下信号序列发到
// 该会话自身进程组——detach 的 teardownPCLocked 对 pc.sess 发 s.stopSignal
//（perclient.go:609，--stop-signal 可配通道自然继承），每个客户端各一遍；
// exitf 收口链分叉——shared = lifecycle 直收，per-client = 收割码经
// pcSupervisor last-reaped-code 规则 terminate（两列同值不同链路）。与
// perclient_test.go 既有断开 SIGHUP 测（TestPerClientDisconnectSIGHUP）的边界：
// 本列聚焦「序列」语义（stop-signal 可配 + 组语义），不断言 teardown 竞态面。
//
// 夹具纪律（2026-08-26 两轮实证——初版探针误判后经真实二进制冒烟修正）：
//   - trap 安装与 detach 信号之间存在竞态——子进程经 setsid+exec 后安装 trap
//     需要非零时间，dialHello 完成不等价 trap 已就位（组合运行负载下实测命中：
//     TERM 先于 trap 到达，sh 按默认动作死亡收 -1）。两测试均以落盘标记文件
//     同步「trap 已安装」（Phase 01-03 决策先例：stdout 标记在 WS 断开后被
//     onChunk 丢弃不可观测，落盘标记是信号类断言的既定形态）。本竞态是
//     KILL 测初版失败的唯一根因。per-client 下 argv 于 attach 期 spawn，
//     waitMarker 同步点同构成立（spawn 先于 dialHello 返回的 Welcome）。
//   - `trap "" TERM` 的恒活机理是 SIG_IGN 跨 exec 持久（POSIX：被忽略的信号
//     掩码在 fork/exec 后保持）——sh 忽略 TERM 时 fork+exec 的 sleep 同样继承
//     SIG_IGN，整组免疫 TERM（真实二进制冒烟实证：stop-timeout=1s 时 wesh 在
//     close+1002ms 经 KILL 退出 255，而非 TERM 后自然退出）。trap "exit 43"
//     形态相反：捕获型 disposition 在 exec 时复位默认，TERM 命中 sh 的 trap
//     退出 43——两形态互补锁定送达与忽略两语义。TERM 测等待形态用
//     `sleep 100 & wait`（bash 手册 wait+trapped 信号立即中断保证；前台 sleep
//     形态在 macOS bash 3.2 下 trap 延后至命令完成导致超时，2026-08-29 CI
//     run 33151570736 裁决，机理详见测试注释）。
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
// `sh -c 'trap "exit 43" TERM; touch M; sleep 100 & wait'`（探针实证形态）：
// TERM 命中 sh 的 trap 以特异退出码 43 退出 → exitf(43) 为 TERM 送达的结构证据
// （若误发 SIGHUP——旧行为——sh 无 HUP trap 直接信号死亡，exitf 收 -1）。
// 等待形态选 `sleep 100 & wait` 而非前台 `sleep 100`（2026-08-29 macOS CI 裁决）：
// bash 手册 Jobs/Signals 节明文——wait builtin 等待期间收到 trapped 信号，
// wait 立即返回并在其后执行 trap；而 bash 等待**前台命令**时 trap 延后至命令
// 完成（dash 则立即中断等待执行 trap）——前台形态在 Linux dash 恒绿，但 macOS
// /bin/sh 是 bash 3.2 posix mode，trap 延后语义使 TERM 测试稳定超时（CI run
// 33151570736：trap 已装、sh 5s 不死、cleanup KILL 收尸）。wait 形态不依赖
// 「sleep 被同组 TERM 杀死」的间接链条（bash 3.2 与 dash 通用的 portable
// 语义），TERM 送达 sh 的证据反而更直接；trap exit 后后台 sleep 成孤儿由
// killServer 进程组 KILL 收尸。
func TestExitWhenEmptyStopSignalTERM(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "trap-armed")
			exitCh, wsURL := newTestServer(t, mode, []string{"sh", "-c", fmt.Sprintf(`trap "exit 43" TERM; touch %s; sleep 100 & wait`, marker)}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = 0
				o.StopSignal = syscall.SIGTERM
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24)
			waitMarker(t, marker) // trap 已安装——detach 信号不再竞态
			c.Close(websocket.StatusNormalClosure, "")

			// 断言分叉表（D-02，mode → expected 显式表）：TERM 送达 → trap exit 43
			// → exitf(43) 恰好一次（200ms 静默锁定无第二次）——两列同值，链路分叉：
			//	shared = 收口路径 SignalGroup(TERM) → lifecycle 直收 43（v1.0 逐字）；
			//	per-client = detach teardown 对该会话自身进程组发可配 stop-signal
			//	  （perclient.go:609）→ trap exit 43 → 收割 43 经 pcSupervisor
			//	  last-reaped-code 规则 terminate(43)——若误发默认 HUP（旧行为），
			//	  sh 无 HUP trap 信号死亡收 -1，本断言翻车（可证伪）。
			waitExit(t, exitCh, 43)
			assertNoExit(t, exitCh)
		})
	}
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
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "trap-armed")
			exitCh, wsURL := newTestServer(t, mode, []string{"sh", "-c", fmt.Sprintf(`trap "" TERM; touch %s; while :; do sleep 10; done`, marker)}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = 0
				o.StopSignal = syscall.SIGTERM
				o.StopTimeout = stopTimeout
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24)
			waitMarker(t, marker) // trap 已安装——detach 信号不再竞态
			c.Close(websocket.StatusNormalClosure, "")

			// stop-timeout 前 300ms 时点静默——TERM 被 trap 忽略，不存在自然死亡路径
			//（此时点收码即序列错误：TERM 未走忽略语义或 KILL 提前补发）。两列同构：
			// per-client 下 supervisor 终结条件 (pcExitReq && len(pcSessions)==0) 的
			// 后半在 KILL 收割前结构性不满足（会话未死未移除），静默同成立。
			select {
			case code := <-exitCh:
				t.Fatalf("exitf(%d) 在 stop-timeout(%v) 前到达——TERM 应被 trap 忽略，无致死路径", code, stopTimeout)
			case <-time.After(300 * time.Millisecond):
			}

			// 400ms 到期 AfterFunc 补 SIGKILL（不占 hubMu、ESRCH 幂等）→ exitf(-1)
			// 恰好一次。断言分叉表（D-02）：shared = KILL → 信号死亡 lifecycle 直收
			// -1（v1.0 逐字）；per-client = teardown 武装的 AfterFunc KILL（复检
			// !reaped + waitDone 栅栏后发送）→ 收割 -1 → pcSupervisor
			// last-reaped-code terminate(-1)。
			waitExit(t, exitCh, -1)
			assertNoExit(t, exitCh)
		})
	}
}
