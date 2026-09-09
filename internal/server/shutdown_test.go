package server_test

// shutdown_test.go 锁定 07-05（D-23，P6 deferred 兑现）1001 优雅下线：
// Server.Shutdown() 向全部已注册客户端发 1001 Going Away（close reason
// server_shutting_down，无 EXIT 帧前置——进程未退出，终结语义由关闭码承载）
// → 子进程进程组收 stop-signal 序列（默认 HUP，可配 + stop-timeout 补 KILL，
// 复用 07-04 落地的 stopSignal/stopTimeout 字段——Options 单一通道）
// → 子进程死亡经既有 lifecycle 收口 exitf 恰好一次（Shutdown 是触发源不是
// exitf 分支，P1 硬约束零新 exit 分支）。
// helper 复用 e2e_test.go/stopseq_test.go/perclient_test.go 同包零改动
// （dialHello/waitExit/assertNoExit/waitMarker/killServer/readSessionPid/
// waitPgroupESRCH）。
//
// 14-03 双模式归一（D-01/D-02，蓝本 PITFALLS :387 行「shutdown_test = 双模式
// 断言分叉：per-client=N 组信号 + 有界 join（P10）」）：装配统一换
// newHandleTestServer 小族（srv 直调 Shutdown 语义两列同构；waitHandlers 忽略
// ——本组不断言 stderr 事件行的列同步边不需要，wg 惰性无害，harness_test.go
// 注释）；原本地 helper startShutdownServerWith 删除（:107/:137 为其最后调用
// 点——14-01 已迁出 health_test.go:251，删除后全仓零引用，D-01 散点消除）。
// 断言分叉表（两列断言逐字搬入）：
//
//	shared 列（v1.0 逐字）= 1001 广播 + 单进程组信号序列：
//	  TestShutdown1001 单客户端 sleep 100 / TestShutdownStopTimeout trap ""
//	  TERM + 300ms KILL 补发。
//	per-client 列 = 全部存活进程组各执行一遍 stop-signal 序列 + 有界 join +
//	  session_end 事件数==N（Phase 13 既有断言归位）：TestShutdown1001 ←
//	  TestPerClientShutdownTwoGroups（形态一：在线双端，N=2）、
//	  TestShutdownStopTimeout ← TestPerClientShutdownJoinBounded（形态三：
//	  KILL 兜底收割的有界 join 时长双锚）。
//
// per-client-only 保持不动（无 shared 对照面，登记 14-03-SUMMARY）：
// TestPerClientShutdownResidualGroup（形态二：断开未收割残留——pcSessions
// 独立于 registry 存在的 Pitfall 6 告警面）、TestPerClientShutdownDeadlineExits
//（形态四：join 到期无条件退出 D-state 代理）。
//
// 夹具纪律：TERM 忽略形态复用 07-04 stopseq 夹具（trap 安装与关停信号竞态经
// 落盘标记文件同步——dialHello 完成不等价 trap 已就位）；TestShutdown1001 的
// 默认 HUP 形态无需标记（HUP 默认动作即死亡，无安装窗口）。客户端 Read 永不带
// deadline ctx（Pitfall 2 回归锁）。

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// readCloseAsync 起客户端读循环 goroutine 读至首个错误（途中数据帧全部跳过）
// 并经缓冲 channel 上报——Shutdown 的 Close 握手需要客户端在读来应答关闭帧
// （库 close 回显走读路径：无在读 Read 则服务端 Close 等满内建 5s 上界，
// close.go:87-89；真实浏览器协议栈透明回显无此窗口）。调用方在 Shutdown 之前
// 启动本循环（plan behavior「客户端读循环收到 CloseError」字面形态），
// c.Read 不可并发的纪律不受影响（dialHello 已返回，本 goroutine 是唯一读者）。
func readCloseAsync(ctx context.Context, c *websocket.Conn) <-chan error {
	ch := make(chan error, 1)
	go func() {
		for {
			_, _, err := c.Read(ctx)
			if err != nil {
				ch <- err
				return
			}
		}
	}()
	return ch
}

// awaitCloseError 收 readCloseAsync 的上报并断言为 CloseError（5s 护栏——
// 无关闭到达即结构性失败）。
func awaitCloseError(t *testing.T, errCh <-chan error) websocket.CloseError {
	t.Helper()
	var ce websocket.CloseError
	select {
	case err := <-errCh:
		if !errors.As(err, &ce) {
			t.Fatalf("read terminated without CloseError: %v", err)
		}
		return ce
	case <-time.After(5 * time.Second):
		t.Fatal("client did not observe close within 5s")
		return ce // 不可达（t.Fatal 不返回），编译形态
	}
}

// TestShutdown1001（07-05，D-23 主干）：Writable:true 起 `sh -c 'sleep 100'`
// （无输出干扰帧断言），dialHello 一客户端 → 调 s.Shutdown() → 客户端读循环
// 收到 CloseError 且 code == 1001（websocket.StatusGoingAway）、reason 含
// server_shutting_down → 子进程经 stop-signal 序列（默认 HUP 零值兜底）终结
// → lifecycle 收口 exitf(-1) 恰好一次（默认 HUP 信号死亡 = -1 桩码，P6 OQ1
// accept-255 同源；assertNoExit 200ms 静默锁定无第二次）。
//
// 14-03 双模式归一：per-client 列 = 13-04 TestPerClientShutdownTwoGroups 断言
// 逐字归位（形态一：在线双端）——N=2 组分叉本体：两客户端 attach（各自独立
// sh）→ Shutdown → 两端各收 1001 + server_shutting_down → 两 pgid 各 ESRCH
// （stop-signal 序列经 detach 链/快照链送达，teardownOnce 幂等承接双触发）→
// exitf(-1) 恰好一次（默认 HUP 信号死亡，last-reaped-code 规则——Shutdown
// 不调 exitf，drained 形态）→ session_end 事件数 == 2（join 界限内 watcher
// 收割链正常 emit——PC-09「不丢 session_end」）。同步边：waitExit 收码 ⟹
// terminate 已触发 ⟹ pcSupervisor 谓词 len==0 已满足 ⟹ 全部 emit 已落流
// （emit→close(waitDone)→delete 程序序链）。shared 列保持单组语义原断言
// （plan action 3：多客户端 N 组形态由 per-client 列承载）。
func TestShutdown1001(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			if mode == server.SessionModePerClient {
				restore := captureStderr(t)
				defer restore()
				// 夹具分叉：交互 sh（readSessionPid 需 echo $$ 回读 pid——pid 锚点
				// 供 ESRCH 断言；shared 列 sleep 100 无回显面故用直跑形态）。
				exitCh, wsURL, _, srv := newHandleTestServer(t, mode, []string{"sh"}, nil)

				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cA, _ := dialHello(t, ctx, wsURL, 80, 24)
				cB, _ := dialHello(t, ctx, wsURL, 90, 30)
				pidA := readSessionPid(t, ctx, cA, "PCSH1A")
				pidB := readSessionPid(t, ctx, cB, "PCSH1B")

				errChA := readCloseAsync(ctx, cA) // 客户端读循环先行——Close 握手应答面
				errChB := readCloseAsync(ctx, cB)
				srv.Shutdown()

				for i, errCh := range []<-chan error{errChA, errChB} {
					ce := awaitCloseError(t, errCh)
					if ce.Code != websocket.StatusGoingAway {
						t.Fatalf("client[%d] close code = %d, want %d (1001)", i, ce.Code, websocket.StatusGoingAway)
					}
					if !strings.Contains(ce.Reason, "server_shutting_down") {
						t.Fatalf("client[%d] close reason = %q, want containing %q", i, ce.Reason, "server_shutting_down")
					}
				}

				// 两会话 HUP 信号死亡 → watcher 收割 -1 → pcSupervisor
				// terminate(last-reaped-code)——Shutdown 不调 exitf（drained 形态）。
				waitExit(t, exitCh, -1)
				assertNoExit(t, exitCh)
				waitPgroupESRCH(t, pidA, 5*time.Second)
				waitPgroupESRCH(t, pidB, 5*time.Second)

				out := restore()
				evs := parseEvents(t, out)
				ends := eventsNamed(evs, "session_end")
				if len(ends) != 2 {
					t.Fatalf("session_end count = %d, want 2（join 界限内收割链零丢失）: %q", len(ends), out)
				}
				for _, e := range ends {
					if e["exit_code"] != float64(-1) {
						t.Fatalf("session_end exit_code = %v, want -1（默认 HUP 信号死亡）: %v", e["exit_code"], e)
					}
					if e["signal"] != "SIGHUP" {
						t.Fatalf("session_end signal = %v, want SIGHUP（信号死亡归因键）: %v", e["signal"], e)
					}
				}
				return
			}

			// shared 列（v1.0 期望值逐字）：单客户端单进程组形态。
			exitCh, wsURL, _, srv := newHandleTestServer(t, mode, []string{"sh", "-c", "sleep 100"}, nil)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24)

			errCh := readCloseAsync(ctx, c) // 客户端读循环先行——Close 握手应答面（见函数注释）
			srv.Shutdown()

			ce := awaitCloseError(t, errCh)
			if ce.Code != websocket.StatusGoingAway {
				t.Fatalf("close code = %d, want %d (1001 Going Away)", ce.Code, websocket.StatusGoingAway)
			}
			if !strings.Contains(ce.Reason, "server_shutting_down") {
				t.Fatalf("close reason = %q, want containing %q（D-23 机器串）", ce.Reason, "server_shutting_down")
			}

			// 子进程终结经既有 lifecycle 收口（Shutdown 不调 exitf——触发源非分支）。
			waitExit(t, exitCh, -1)
			assertNoExit(t, exitCh)
		})
	}
}

// TestShutdownStopTimeout（07-05，D-23 × D-22 序列）：Options{StopSignal:
// SIGTERM, StopTimeout: 300ms} + 子进程 trap 忽略 TERM（`trap "" TERM` +
// while 循环显式恒活夹具——07-04 stopseq 夹具形态复用，trap 安装经落盘标记
// 同步）→ Shutdown 广播 1001 后 TERM 被忽略、stop-timeout 到期补发 SIGKILL
// → 信号死亡 exitf(-1) 恰好一次。KILL 补发的结构证据：trap 忽略 TERM 的
// 循环夹具只有 KILL 能致死——无补发则进程必然存活到 waitExit 5s 护栏翻车。
//
// 14-03 双模式归一：per-client 列 = 13-04 TestPerClientShutdownJoinBounded
// 断言逐字归位（形态三：有界 join——T-13-13 前半）——stop-timeout KILL 兜底
// 的 per-client 形态：trap 免疫子进程在线 + StopTimeout=500ms → Shutdown →
// join 等待 KILL 兜底到期收割（500ms 前免疫子进程无死路）→ join 在界限内
// 完成。时长双锚：下界 250ms 证明等待真实发生；上界 = stopTimeout + 余量 +
// 护栏证明有界——无界 join 形态（等全部 Wait 返回）在此翻车（免疫子进程可
// 长期存活）。
func TestShutdownStopTimeout(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			if mode == server.SessionModePerClient {
				restore := captureStderr(t)
				defer restore()
				exitCh, wsURL, _, srv := newHandleTestServer(t, mode, trapHupLoopArgv("PCSH3J"), func(o *server.Options) {
					o.StopTimeout = 500 * time.Millisecond
				})

				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				c, _ := dialHello(t, ctx, wsURL, 80, 24)
				pid := readSessionPid(t, ctx, c, "PCSH3J")
				defer func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }()
				errCh := readCloseAsync(ctx, c)

				start := time.Now()
				srv.Shutdown()
				dur := time.Since(start)

				if ce := awaitCloseError(t, errCh); ce.Code != websocket.StatusGoingAway {
					t.Fatalf("close code = %d, want %d (1001)", ce.Code, websocket.StatusGoingAway)
				}
				if dur < 250*time.Millisecond {
					t.Fatalf("Shutdown duration = %v, want ≥ 250ms（join 应等待 KILL 兜底收割）", dur)
				}
				if upper := 500*time.Millisecond + 2*time.Second + 2*time.Second; dur >= upper {
					t.Fatalf("Shutdown duration = %v, want < %v（有界 join 上界）", dur, upper)
				}

				waitExit(t, exitCh, -1) // KILL 兜底收割 → -1
				assertNoExit(t, exitCh)
				waitPgroupESRCH(t, pid, 5*time.Second)

				out := restore()
				evs := parseEvents(t, out)
				ends := eventsNamed(evs, "session_end")
				if len(ends) != 1 {
					t.Fatalf("session_end count = %d, want 1: %q", len(ends), out)
				}
				if ends[0]["signal"] != "SIGKILL" {
					t.Fatalf("session_end signal = %v, want SIGKILL（KILL 兜底归因）: %v", ends[0]["signal"], ends[0])
				}
				return
			}

			// shared 列（v1.0 期望值逐字）。
			marker := filepath.Join(t.TempDir(), "trap-armed")
			exitCh, wsURL, _, srv := newHandleTestServer(t, mode, []string{"sh", "-c", fmt.Sprintf(`trap "" TERM; touch %s; while :; do sleep 10; done`, marker)}, func(o *server.Options) {
				o.StopSignal = syscall.SIGTERM
				o.StopTimeout = 300 * time.Millisecond
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24)
			waitMarker(t, marker) // trap 已安装——Shutdown 的 TERM 不再竞态（07-04 夹具纪律）

			errCh := readCloseAsync(ctx, c) // 客户端读循环先行（同上）
			srv.Shutdown()

			// 关停广播与信号配置无关——客户端同样收 1001 + server_shutting_down。
			ce := awaitCloseError(t, errCh)
			if ce.Code != websocket.StatusGoingAway {
				t.Fatalf("close code = %d, want %d (1001 Going Away)", ce.Code, websocket.StatusGoingAway)
			}
			if !strings.Contains(ce.Reason, "server_shutting_down") {
				t.Fatalf("close reason = %q, want containing %q", ce.Reason, "server_shutting_down")
			}

			// TERM 被 trap 忽略 → stop-timeout(300ms) 到期 Shutdown 内补发 SIGKILL
			//（ESRCH 幂等纪律）→ exitf(-1) 恰好一次。
			waitExit(t, exitCh, -1)
			assertNoExit(t, exitCh)
		})
	}
}

// ===== 13-04（PC-09）per-client Shutdown per-client-only 测试组（14-03 归一
// 判定：无 shared 对照面，保持单模式原样——登记 14-03-SUMMARY）=====
//
// 锁定 13-04 落地的 Shutdown per-client 分支（server.go stop-signal 段）：
// pcSessions 快照逐组 stop-signal（含断开未收割残留者——Pitfall 6）+ 有界
// join（上界 = stopTimeout + shutdownJoinMargin，PITFALLS P10）+ Broadcast
// 补行唤醒 pcSupervisor（13-03 消费端）；join 到期未清零经 terminate/
// termOnce 无条件收口（T-13-13）。本组两形态（残留/到期无条件退出）在 shared
// 无对应概念面（pcSessions 独立于 registry 存在/join 到期收口均为 per-client
// 特有机械）——N 组信号与有界 join 的双模式分叉面已由 TestShutdown1001/
// TestShutdownStopTimeout per-client 列承载（TwoGroups/JoinBounded 归位）。
//
// 装配复用（plan 文本「startShutdownServerWith 参数化」的落点）：per-client
// 变体夹具即 perclient_test.go startPerClientServerWithSpawn（11-03 已参数化
// spawnFn+mutate，同包 server_test 零新装配代码）——New(nil, exitf 捕获桩,
// Options{SessionMode: per-client, SpawnFunc: 追踪包装, Writable: true} +
// mutate 覆写），返回 srv 句柄供 Shutdown 直调；spawned 追踪收口夹具对已
// 收割会话幂等（Kill→ErrProcessDone 静默 + Close 幂等）。
//
// 夹具纪律（stopseq_test.go:11-17 逐字沿用）：trap 安装与关停信号存在竞态
// ——免疫夹具 argv 均 `trap '' HUP; echo TAG=$$; while :; do sleep 10; done`
// 形态（trap 安装先于 echo，客户端在线期回读 pid 即 trap 已就位的同步点，
// 11-03 免落盘纪律）；SIG_IGN 跨 exec 持久使整组免疫 HUP（真实死亡路径唯一
// = SIGKILL 兜底）。时长断言一律下界/上界双锚，禁精确时点断言（Pitfall 7）。

// trapHupLoopArgv 构造 trap "" HUP 免疫恒活夹具 argv（11-04
// TestPerClientStopTimeoutKillFallback / 13-03 signal_sigkill 同款）。
func trapHupLoopArgv(tag string) []string {
	return []string{"sh", "-c", "trap '' HUP; echo " + tag + "=$$; while :; do sleep 10; done"}
}

// TestPerClientShutdownResidualGroup（形态二：断开未收割残留——Pitfall 6
// 告警面，「只有客户端在线形态」即不及格）：trap "" HUP 免疫子进程 +
// StopTimeout=2s（长值防 KILL 兜底在 Shutdown 快照前抢跑——短值下会话先死
// 先删，pcSessions 空、快照无残留可覆盖）→ attach 回读 pid → 客户端断开
// （detach 的 teardown HUP 被免疫，会话残留 pcSessions）→ Shutdown → 快照链
// 覆盖该残留组 → join 等待 KILL 兜底收割 → exitf(-1) + ESRCH + session_end
// 恰 1（signal=SIGKILL 归因）。时长双锚：下界 1s 证明 join 真实等待收割
// （11-01 空 if 体旧形态 Shutdown ~0ms 返回即翻车）；上界 = stopTimeout +
// 余量 + 护栏。
func TestPerClientShutdownResidualGroup(t *testing.T) {
	restore := captureStderr(t)
	defer restore()
	exitCh, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int, _ string) (*pty.Session, error) {
		return pty.StartWithSize(trapHupLoopArgv("PCSH2R"), pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, func(o *server.Options) { o.StopTimeout = 2 * time.Second })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	pid := readSessionPid(t, ctx, c, "PCSH2R")
	defer func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }() // 幂等清场（FAIL 路径同清场，CI 级联减速夹具纪律）
	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close: %v", err)
	}

	// detach 完成同步边：/healthz clients==0（removeLocked 与
	// teardownPCLocked 同一 hubMu 临界区——观测到 0 即 HUP 已发 + KILL 兜底
	// 已武装，时长锚点自此确定）。
	detachDeadline := time.Now().Add(2 * time.Second)
	for healthzClients(t, wsURL) != 0 {
		if time.Now().After(detachDeadline) {
			t.Fatal("detach 2s 内未完成——夹具未就位")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 残留在册实证（Pitfall 6 形态本体）：免疫子进程无死路，会话必留
	// pcSessions——快照源的覆盖对象。
	if n := srv.PCSessionsLenForTest(); n != 1 {
		t.Fatalf("pcSessions = %d, want 1（断开未收割残留会话）", n)
	}

	start := time.Now()
	srv.Shutdown()
	dur := time.Since(start)
	if dur < time.Second {
		t.Fatalf("Shutdown duration = %v, want ≥ 1s（join 应等待 KILL 兜底收割残留组——空 if 体旧形态 ~0ms 返回）", dur)
	}
	if upper := 2*time.Second + 2*time.Second + 2*time.Second; dur >= upper {
		t.Fatalf("Shutdown duration = %v, want < %v（有界 join 上界 stopTimeout+余量+护栏）", dur, upper)
	}

	waitExit(t, exitCh, -1) // SIGKILL 死亡 → last-reaped-code -1
	assertNoExit(t, exitCh)
	waitPgroupESRCH(t, pid, 5*time.Second)

	out := restore()
	evs := parseEvents(t, out)
	ends := eventsNamed(evs, "session_end")
	if len(ends) != 1 {
		t.Fatalf("session_end count = %d, want 1: %q", len(ends), out)
	}
	if ends[0]["signal"] != "SIGKILL" {
		t.Fatalf("session_end signal = %v, want SIGKILL（KILL 兜底归因）: %v", ends[0]["signal"], ends[0])
	}
	if ends[0]["exit_code"] != float64(-1) {
		t.Fatalf("session_end exit_code = %v, want -1", ends[0]["exit_code"])
	}
}

// TestPerClientShutdownDeadlineExits（形态四：join 到期无条件退出——T-13-13
// 后半「有界 join 后无条件经 termOnce 退出」的 D-state 代理形态）：trap ""
// HUP 免疫子进程 + StopTimeout=0（零值 = 不补 KILL，合法现状语义——「不可
// 被本服务端信号配置杀死」的 D-state 代理：HUP 被免疫且无补杀通道，会话
// 永不收割）→ Shutdown → join 到期（上界 = 0 + shutdownJoinMargin 2s）后
// 无条件继续并经 terminate/termOnce 收口：exitf(0)（无会话收割 →
// last-reaped-code 缺省 0——pcSupervisor 出循环同规则）。时长双锚证明等待
// 确被 join 占据且被 deadline 截断（无界 join / 无到期收口两形态均翻车：
// 前者 Shutdown 永不返回，后者 waitExit 5s 护栏超时）。
//
// 收口同步边（13-03 TestAuthFailedNoUsername 修法同款）：断言后体内 Kill
// 免疫子进程 → 排空收割链（PCSessionsLenForTest()==0——session_end emit →
// close(waitDone) → delete 全落本捕获窗）后才 restore，迟写事件不污染后继
// 测试的捕获窗。
func TestPerClientShutdownDeadlineExits(t *testing.T) {
	restore := captureStderr(t)
	defer restore()
	exitCh, wsURL, srv, _ := startPerClientServerWithSpawn(t, func(cols, rows int, _ string) (*pty.Session, error) {
		return pty.StartWithSize(trapHupLoopArgv("PCSH4D"), pty.StartOptions{Uid: -1, Gid: -1}, cols, rows)
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	pid := readSessionPid(t, ctx, c, "PCSH4D")
	defer func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }()
	errCh := readCloseAsync(ctx, c)

	start := time.Now()
	srv.Shutdown()
	dur := time.Since(start)

	if ce := awaitCloseError(t, errCh); ce.Code != websocket.StatusGoingAway {
		t.Fatalf("close code = %d, want %d (1001)", ce.Code, websocket.StatusGoingAway)
	}
	// join 上界 = stopTimeout(0) + shutdownJoinMargin(2s)：下界 1.5s 证明等待
	// 占据近全程；上界 4s（上界 + 2s 护栏）证明 deadline 截断。
	if dur < 1500*time.Millisecond {
		t.Fatalf("Shutdown duration = %v, want ≥ 1.5s（join 应等待至 deadline）", dur)
	}
	if dur >= 4*time.Second {
		t.Fatalf("Shutdown duration = %v, want < 4s（deadline 截断有界 join）", dur)
	}
	// 到期无条件退出：免疫 + 无 KILL 通道 → 无会话收割 → last-reaped-code
	// 缺省 0；exitf 恰好一次（termOnce——pcSupervisor 后续唤醒重估时 no-op）。
	waitExit(t, exitCh, 0)
	assertNoExit(t, exitCh)
	// 免疫子进程仍存活（退出不拖死实证的另一面：残余非本信号配置可杀，
	// stopTimeout=0 知情风险——操作者/清场处置，Pitfall 8 语义）。
	if err := syscall.Kill(-pid, 0); err != nil {
		t.Fatalf("免疫子进程不应被关停序列杀死（kill 0 探针 = %v）", err)
	}

	// 收口同步边：体内 Kill → 收割链排空后才 restore（见函数头注释）。
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		t.Fatalf("kill residual: %v", err)
	}
	drainDeadline := time.Now().Add(5 * time.Second)
	for srv.PCSessionsLenForTest() != 0 {
		if time.Now().After(drainDeadline) {
			t.Fatal("收割链 5s 内未排空（体内 Kill 后）")
		}
		time.Sleep(10 * time.Millisecond)
	}

	out := restore()
	evs := parseEvents(t, out)
	ends := eventsNamed(evs, "session_end")
	if len(ends) != 1 {
		t.Fatalf("session_end count = %d, want 1（体内 Kill 的迟收割 emit 落本捕获窗）: %q", len(ends), out)
	}
	if ends[0]["signal"] != "SIGKILL" {
		t.Fatalf("session_end signal = %v, want SIGKILL: %v", ends[0]["signal"], ends[0])
	}
}
