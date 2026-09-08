package server_test

// emptyexit_test.go 锁定 SESS-01/02 服务端半侧行为（06-02）：注册表空触发断开退出
// 六路径——立即退出（grace=0）/宽限取消/宽限到期/kick 第二移除路径触发/exiting 门
// 抑制终结广播误触发/宽限计时器晚于 lifecycle 到期不补发（review #5 吸收）。helper
// 复用 e2e_test.go/exit_test.go/slowclient_test.go/limits_test.go 同包零改动
// （06-PATTERNS exact）。
//
// 14-03 双模式归一（D-01/D-02，蓝本 PITFALLS :386 行「emptyexit_test =
// 双模式断言分叉：per-client 退出路径=空迁移 terminate（P1）」）：装配统一换
// newTestServer 小族（harness_test.go）t.Run 双跑；Phase 13 混入的两 per-client 测
// （TestEmptyExitPerClientChildFirst/ClientFirst）断言逐字归位进
// LifecycleGate/Immediate 的 per-client 列（证据链搬运非删除）。分叉表：
//
//	时序            shared --once（v1.0 逐字）              per-client（第二终结源列）
//	子进程先死      lifecycle Wait→exitf(子码)              watcher 收割子码 →
//	                （TestExitWhenEmptyLifecycleGate）        EXIT+1000 → detach 致空 →
//	                                                         pcExitReq → supervisor →
//	                                                         exitf(子码)
//	客户端先断      detach→HUP→子死(-1)→exitf(-1) → 255     detach→pcExitReq+HUP→
//	                （TestExitWhenEmptyImmediate）            watcher 收割(-1)→
//	                                                         exitf(-1) → 255
//	宽限到期/取消   timer 武装/取消/复查四守卫两模式共用     同左（13-03 取消点与门闩
//	                （clients.go maybeExitWhenEmptyLocked）    清零落地）；到期触发事件
//	                                                         恰 1 条 + pcExitReq
//
// OQ1 门裁决（06-02 Task 1，2026-08-23 用户裁决 accept-255）：断开退出收口路径
// exitf 以子进程原码收口——SIGHUP 致死 ExitCode()=-1（GOROOT exec_posix.go:155-157
// 语义，与 D-09「信号死亡 exit_code=-1」同源），故本文件断言常量 = -1；os.Exit(-1)
// 被 Unix 截断为进程退出状态 255 只在真实二进制出现，由 06-06 phase06.mjs S3/S4/S5
// 进程级断言、06-07 README 明示文案承接（门裁决值三处下游消费点单点落地）。
//
// 客户端 Read 永不带 deadline ctx（Pitfall 2 回归锁）——静默窗口一律 select +
// time.After 竞速形态。

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// TestExitWhenEmptyImmediate（SESS-01/02 立即形态，D-14 grace=0 合法显式值）：
// 唯一客户端断开 → 注册表非空→空迁移 → 迁移点直接 SIGHUP 进程组（无计时器）→
// 既有 lifecycle 单一路径收口——exitf 捕获桩 5s 内收 -1 恰好一次（accept-255
// 断言常量，见文件头）。
//
// 14-03 双模式归一：per-client 列 = 13-03 TestEmptyExitPerClientClientFirst
// 断言逐字归位——同触发条件（唯一客户端断开致空迁移），第二终结源路径：detach
// teardown 快半段 SIGHUP 其会话 + maybeExitWhenEmptyLocked per-client 分支置位
// pcExitReq（不发信号）→ watcher 收割 -1（last-reaped-code）→ pcSupervisor
// terminate——exitf(-1) 同码（进程级 255 对齐）+ 触发端正面证据 exit_when_empty
// 事件恰 1 条（remote/code 1000 同 shared schema——mode 分歧只在动作面不在
// 事件面）。同步边：事件 emit 先于 pcExitReq 置位/Broadcast → supervisor →
// exitf，waitExit 收码即事件已落流（hubMu 链），stderr 断言无独立同步需求。
func TestExitWhenEmptyImmediate(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = 0
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24)

			// 断言分叉表（D-02）：
			//	shared（v1.0 逐字）= SIGHUP 致死 → ExitCode()=-1 → exitf(-1) 恰好
			//	  一次（termOnce 收口 + 200ms 静默锁定无第二次）。
			//	per-client（Phase 13 断言归位）= 同触发条件，终结目标为该客户端
			//	  进程组 + 退出状态 255 对齐（Go 桩常量 -1）+ exit_when_empty 事件
			//	  恰 1 条。
			if mode == server.SessionModePerClient {
				restore := captureStderr(t)
				defer restore() // 失败路径兜底恢复 os.Stderr（幂等，limits_test.go 先例）

				if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
					t.Fatalf("close: %v", err)
				}

				// SIGHUP 致死 → ExitCode()=-1 → watcher 收割 → exitf(-1) 恰好一次
				//（accept-255 断言常量，见文件头 OQ1 门注释）。
				waitExit(t, exitCh, -1)
				assertNoExit(t, exitCh)

				out := restore()
				evs := parseEvents(t, out)
				if n := countByEvent(evs, "exit_when_empty"); n != 1 {
					t.Fatalf("exit_when_empty count = %d, want 1（per-client 立即形态触发事件；out=%q）", n, out)
				}
				return
			}

			// shared 列（v1.0 期望值逐字）。
			c.Close(websocket.StatusNormalClosure, "")
			waitExit(t, exitCh, -1)
			assertNoExit(t, exitCh)
		})
	}
}

// TestExitWhenEmptyGraceCancel（SESS-02 宽限取消，D-14）：grace=2s → 唯一客户端
// 断开启动宽限计时 → 300ms 后再 attach 成功（registerLocked 取消点 Stop+置 nil，
// 恰好一次）→ echo 验证会话存活 → 越过旧计时器到期点 exitCh 静默（取消实证）→
// 再次断开重新计时 → 到期收码。
//
// plan 字面「1.5s 时点静默」早于旧 timer 到期点 2s，不构成取消证据（算术松散）——
// 按 parenthetical 语义要求（「旧 timer 若未取消此时已到期」）延长静默窗越过到期点
// +500ms 余量（deviation 登记）。
//
// 14-03 双模式双跑（同断言双跑判定，14-02 Pattern 4）：四守卫与计时器机械两模式
// 共用（clients.go maybeExitWhenEmptyLocked 同代码），可观测期望两列同值——
// 取消点 per-client 由 13-03 落地（upgradePerClient registerLocked 后补宽限取消
// 点 + 空纪元门闩清零，perclient.go）；分叉仅在机制面：shared 再 attach 接回同一
// 存活会话，per-client 再 attach 出生全新会话（echo 语义两列同验——后者证新会话
// 存活与服务可用）；到期收码 shared = SIGHUP→lifecycle，per-client = 计时器→
// pcExitReq→supervisor（watcher 收割 -1，last-reaped-code）。
func TestExitWhenEmptyGraceCancel(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			const grace = 2 * time.Second
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = grace
			})

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			c1, _ := dialHello(t, ctx, wsURL, 80, 24)
			closedAt := time.Now()
			c1.Close(websocket.StatusNormalClosure, "") // 注册表空迁移 → 宽限计时启动

			// 宽限内 300ms 再 attach：registerLocked 成功 → 计时取消（恰好一次，置 nil
			// 防重复 Stop 与重复事件）。per-client 列：attach 期 spawn 全新会话（13-03
			// 取消点同步清零空纪元门闩）。
			time.Sleep(300 * time.Millisecond)
			c2, _ := dialHello(t, ctx, wsURL, 80, 24)

			// echo 验证会话存活（宽限取消后会话继续——D-14「计时内任一端 attach 成功则
			// 退出取消、会话继续」；per-client 列验证新出生会话的 echo 语义同链）。
			payload := []byte("grace cancel echo")
			if err := c2.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
				t.Fatalf("write INPUT after re-attach: %v", err)
			}
			got := make([]byte, 0, len(payload))
			for len(got) < len(payload) {
				_, data, err := c2.Read(ctx)
				if err != nil {
					t.Fatalf("read OUTPUT after re-attach: %v (got %q so far)", err, got)
				}
				if len(data) == 0 || data[0] != proto.Output {
					t.Fatalf("unexpected frame: %v", data)
				}
				got = append(got, data[1:]...)
			}
			if string(got) != string(payload) {
				t.Fatalf("echo payload = %q, want %q", got, payload)
			}

			// 越过旧计时器到期点（closedAt+grace）+500ms 余量，exitCh 静默 = 取消实证
			//（旧 timer 若未取消，此时已到期发 SIGHUP 致死收 -1）。
			if remain := time.Until(closedAt.Add(grace + 500*time.Millisecond)); remain > 0 {
				select {
				case code := <-exitCh:
					t.Fatalf("exitf called with code %d past old timer expiry — grace timer not canceled by re-attach", code)
				case <-time.After(remain):
				}
			}

			// 再次断开 → 注册表再次空迁移 → 重新计时 → 到期 SIGHUP → lifecycle 收口
			//（grace+2s 余量内收码；accept-255 断言常量 -1；per-client 列 = 计时器
			// 到期 pcExitReq → supervisor 收 -1，Looks-Done 清单宽限到期行双模式覆盖）。
			c2.Close(websocket.StatusNormalClosure, "")
			select {
			case code := <-exitCh:
				if code != -1 {
					t.Fatalf("exit code = %d, want -1（accept-255 门裁决断言常量）", code)
				}
			case <-time.After(grace + 2*time.Second):
				t.Fatal("exitf not called within grace+2s after second detach — re-armed timer did not fire")
			}
		})
	}
}

// TestExitWhenEmptyGraceExpire（SESS-02 宽限到期，D-14）：grace=400ms → 唯一客户端
// 断开启动宽限计时 → 不再 attach → 到期前 100ms 时点 exitCh 静默（不过早退出）→
// 到期 SIGHUP → 3s 内收 -1 恰好一次（AfterFunc 单次触发 + termOnce 双保险）。
//
// 14-03 双模式双跑（Looks-Done 清单 :342 行「宽限到期形态在 per-client 真的退出
// （无子等陷阱 P1）；验证 grace>0 + 无重连 → 进程退 255」的双模式覆盖）：per-client
// 列同触发同期望——detach teardown 快半段即刻 SIGHUP 其会话（watcher 收割 -1，
// pcSessions 归零），服务端仍等宽限计时到期（pcExitReq 仅在到期回调置位）→
// pcSupervisor terminate(-1)——「注册表已空且无子进程可等」形态下第二终结源真实
// 生效（不会永不退出，P1 窗口期闭合的到期侧证明）。
func TestExitWhenEmptyGraceExpire(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = 400 * time.Millisecond
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24)
			c.Close(websocket.StatusNormalClosure, "")

			// 到期前 100ms 时点静默（grace=400ms 远未到期——不过早退出；per-client
			// 列 pcExitReq 未置位、exiting 未置位，supervisor 谓词不成立同理静默）。
			select {
			case code := <-exitCh:
				t.Fatalf("exitf called with code %d at 100ms, before grace expiry (400ms) — premature exit", code)
			case <-time.After(100 * time.Millisecond):
			}

			// 到期 SIGHUP → lifecycle 收口：3s 内收 -1（accept-255 断言常量）恰好一次
			//（per-client 列 = 到期回调 pcExitReq → supervisor 收 last-reaped -1）。
			select {
			case code := <-exitCh:
				if code != -1 {
					t.Fatalf("exit code = %d, want -1（accept-255 门裁决断言常量）", code)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("exitf not called within 3s after grace expiry")
			}
			assertNoExit(t, exitCh)
		})
	}
}

// TestExitWhenEmptyKickTrigger（SESS-01/02 第二移除路径）：stall 客户端被 1013
// slow_consumer 踢出（kickSlowConsumerLocked——removeLocked 第二调用点）→ 注册表
// 空迁移 → grace=0 立即 SIGHUP → 5s 内收 -1。stall 夹具形态照抄
// slowclient_test.go（OutboxBytes 小值覆写 + seq 洪水）；Writable:false 使唯一
// 客户端为 ro——shared 下 ro 满即踢永不持信用（R-08 分工表；rw 独端会落信用门
// 持块而非被踢，踢出断言结构性不成立）。
//
// 断言序（05-07 登记的 stall 夹具戒律——踢出触发前绝不 Read）：先等 exitf(-1)——
// 本夹具下无人读取时令 seq 死亡的唯一路径就是 outbox 写满 → 1013 踢出 → 注册表
// 空迁移 → SIGHUP（38.9MB 洪水 ≫ 不读取客户端 ~4-10MiB 吸收量，子进程结构性
// 不可能自然跑完），exitf(-1) 到达即踢出与空触发的结构性证据；随后再读连接取证
// 1013 关闭帧。反序（先 assertKicked1013）会让 readUntilError 立刻开始排空管道，
// 踢出永不成立（实测竞态：~50% 跑到子进程自然退出收 1000）。
//
// 14-03 双模式双跑：per-client 列同触发同期望——无信用门 1:1 退化（R-08：满箱
// 停读后 dwell 到期踢，14-01 TestSlowConsumerKick per-client 列同款 SlowDwell=
// 500ms 短值覆写使踢出落在 waitExit 断言窗内，默认 10s 恒在窗外）→ 注册表空迁移
// → pcExitReq → teardown HUP → watcher 收割 -1 → supervisor exitf(-1)；踢出后
// 1013 取证同链。dwell 语义本体由 perclient_test.go Phase 12 三测单一承载（本列
// 只锁「满即踢 + 空迁移第二终结源」分叉点，T-14-04 防双写漂移）。
func TestExitWhenEmptyKickTrigger(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			// seq 1 5000000 ≈ 38.9MB 洪水（slowclient_test 先例量级：> 单连接最坏吸收
			// ~10MiB + 64KiB outbox——stall 必然传导到 outbox 写满）。
			exitCh, wsURL := newTestServer(t, mode, []string{"seq", "1", "5000000"}, func(o *server.Options) {
				o.Writable = false
				o.OutboxBytes = 64 * 1024
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = 0
				if mode == server.SessionModePerClient {
					o.SlowDwell = 500 * time.Millisecond // dwell 短值覆写（14-01 同款）
				}
			})

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			stall, _ := dialHello(t, ctx, wsURL, 80, 24)
			// 放宽读上限：writer 合并段使积压后的单条 WS 消息超 Go 客户端库默认 32KiB
			// 读上限会被 1009 误关（slowclient_test 实测先例）；stall = dialHello 成功后
			// 不再 Read，直到 exitf 证据到达后再取证踢出。
			stall.SetReadLimit(4 * 1024 * 1024)

			// exitf(-1) = kick 移除路径空触发 SIGHUP 的结构性证据（无人读取时子进程
			// 不可能跑完 38.9MB 洪水——唯一致死路径即 outbox 满 → 踢出 → 注册表空 →
			// SIGHUP；accept-255 断言常量 -1）恰好一次。per-client 列：唯一致死路径
			// 同构（outbox 满 → dwell(500ms) 踢出 → 空迁移 pcExitReq → teardown HUP）。
			waitExit(t, exitCh, -1)
			assertNoExit(t, exitCh)
			// 取证踢出关闭帧：1013 slow_consumer 逐字（踢出事件的客户端可见形态）。
			assertKicked1013(t, stall, 15*time.Second, "stall client")
			stall.CloseNow()
		})
	}
}

// TestExitWhenEmptyLifecycleGate（planner 推导不变量，D-13 防线）：ExitWhenEmpty
// grace=0 + argv `sh -c 'sleep 1; exit 42'` → 客户端在线等子进程自然退出 → 收
// EXIT{exit_code:42} + 1000（06-01 形态）→ exitf(42) 恰好一次——广播 Close 引发的
// detach 致空被 exiting 门抑制（无门则广播期 detach 再生 SIGHUP/误导性
// exit_when_empty 事件；自然退出 exit 42 路径的 SIGHUP 翻码竞态防线）。
//
// 14-03 双模式归一：per-client 列 = 13-03 TestEmptyExitPerClientChildFirst 断言
// 逐字归位——两时序之「子先死」列（研究 ARCHITECTURE §4.3 逐位对齐）：watcher
// 收割 42 → EXIT{42}+1000 私有化直写 → 客户端终结触发 reader detach 致空 →
// pcExitReq → supervisor 以 last-reaped-code 42 收口——「exitf(42) 而非 -1」即
// SIGHUP 未染指本路径的语义证据（wire 面与 exitf 面两列同值，机制分叉仅在
// lifecycle Wait vs watcher 收割链；EXIT 私有化 vs 广播的多客户端面归
// exit_test.go TestExitFrameBroadcast per-client 列单一承载，T-14-04）。
func TestExitWhenEmptyLifecycleGate(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"sh", "-c", "sleep 1; exit 42"}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = 0
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24) // sleep 1 保 attach 窗口

			frames, code := readExitClose(t, ctx, c)
			if code != websocket.StatusNormalClosure {
				t.Fatalf("close code = %d, want %d (1000)", code, websocket.StatusNormalClosure)
			}
			if len(frames) == 0 {
				t.Fatal("no frames collected——EXIT 帧缺失")
			}
			ep := decodeExitFrame(t, frames[len(frames)-1])
			if ep.ExitCode != 42 {
				if mode == server.SessionModeShared {
					t.Fatalf("EXIT exit_code = %d, want 42（exiting 门抑制空触发——被 SIGHUP 翻码则为 -1）", ep.ExitCode)
				}
				t.Fatalf("EXIT exit_code = %d, want 42（子进程退出码透传）", ep.ExitCode)
			}

			// exitf 以子进程原码 42 恰好一次收口（SIGHUP 未染指本路径的语义证据；
			// per-client 列 = supervisor 以 watcher 收割码 42 收口，last-reaped-code
			// 规则与 shared lifecycle 同码）。
			waitExit(t, exitCh, 42)
			assertNoExit(t, exitCh)
		})
	}
}

// TestExitWhenEmptyTimerAfterLifecycle（review #5 吸收——宽限计时器在 lifecycle
// 启动后到期不得再触发）：grace=2s → 客户端 close 启动宽限计时（到期 ≈close+2s）→
// 子进程 ~1s 自然退出先于计时器到期 → lifecycle 置 exiting + exitf(42) → 收 42
// 恰好一次 → 越过计时器到期点（close+3s 护栏）无第二次收码，且 stderr 无
// exit_when_empty 触发事件（事件名精确相等区分——exit_when_empty_wait 启动
// 事件允许存在；JSON 字段语义下两事件名独立，无前缀歧义）。
// 回归形态 = 计时器回调缺 exiting 复查时向已终结会话补发 SIGHUP 并打误导性触发日志
// （review『confusing log』关切的可执行闭合）。
//
// 14-03 双模式归一（:250 收编 newTrackedTestServer——stderr 同步边保留）：断言
// 分叉表（D-02，同一场景两列真值相反的正面对照）：
//
//	shared（v1.0 逐字）= 子进程 ~1s 自然退出先于计时器（断开不杀 shared 会话）
//	  → lifecycle 置 exiting → exitf(42)；计时器 2s 到期复查 exiting 静默——
//	  触发事件零次（回归面本体）。
//	per-client = 时序翻转：detach teardown 快半段即刻 SIGHUP 子进程（客户端先断
//	  形态）→ watcher 收割 -1、pcSessions 归零等待；计时器 2s 到期不受 exiting
//	  抑制（无 lifecycle 在场）→ 触发事件恰 1 次 + pcExitReq → supervisor
//	  exitf(-1)（last-reaped-code）——计时器真实生效的正面对照列。
func TestExitWhenEmptyTimerAfterLifecycle(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			restore := captureStderr(t)
			defer restore() // 失败路径兜底恢复 os.Stderr（幂等，limits_test.go 先例）

			exitCh, wsURL, waitHandlers := newTrackedTestServer(t, mode, []string{"sh", "-c", "sleep 1; exit 42"}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = 2 * time.Second
			})

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			c, _ := dialHello(t, ctx, wsURL, 80, 24)
			closedAt := time.Now()
			c.Close(websocket.StatusNormalClosure, "") // 注册表空迁移 → 宽限计时启动（到期 ≈closedAt+2s）

			// 子进程终结收口（分叉表）：shared = ~1s 自然退出 → lifecycle 快照前置
			// exiting → exitf(42) 恰好一次；per-client = teardown HUP 即刻致死 →
			// watcher 收割 -1 等待，计时器 2s 到期 pcExitReq → exitf(-1)。
			wantExit, wantTrigger := 42, 0
			if mode == server.SessionModePerClient {
				wantExit, wantTrigger = -1, 1
			}
			waitExit(t, exitCh, wantExit)

			// 越过计时器到期点（closedAt+3s 护栏）：无第二次收码（回调 exiting 复查守卫
			//（shared）/termOnce 收口（两列）；回归形态下回调到期补发 SIGHUP——exitf
			// 仍被 termOnce 拦住，此断言由下方 stderr 触发行计数承接）。
			if remain := time.Until(closedAt.Add(3 * time.Second)); remain > 0 {
				select {
				case code := <-exitCh:
					t.Fatalf("exitf called second time with code %d after lifecycle — timer fired past lifecycle", code)
				case <-time.After(remain):
				}
			}

			// 同步边（05-01 决策先例）：/ws handler 随 detach 返回——其内 logEvent
			//（exit_when_empty_wait 启动行）先于 handler 返回，WaitGroup happens-before
			// 使 restore() 写 os.Stderr 与该读同步；计时器回调的正确形态在 logEvent 之前
			// return（exiting 复查，shared 列），不触 os.Stderr。
			waitHandlers()
			out := restore()
			evs := parseEvents(t, out)
			if n := countByEvent(evs, "exit_when_empty"); n != wantTrigger {
				if mode == server.SessionModeShared {
					t.Fatalf("stderr contains exit_when_empty trigger event after lifecycle (count=%d, out=%q) — timer callback missing exiting recheck", n, out)
				}
				t.Fatalf("exit_when_empty count = %d, want 1（per-client 宽限到期第二终结源触发；out=%q）", n, out)
			}
			// 启动事件允许存在且必须存在（事件名精确相等的正面证据——wait 事件在
			// 证明计时器真实武装过，触发事件零次（shared）/恰一次（per-client）才
			// 有意义）。
			if n := countByEvent(evs, "exit_when_empty_wait"); n != 1 {
				t.Fatalf("stderr missing exit_when_empty_wait start event (count=%d, out=%q) — grace timer did not start on detach", n, out)
			}
		})
	}
}

// TestExitWhenEmptyPromoteKickOnce（08-review WR-01 回归锁定）：「递补升格踢出致空」
// 边角路径下 exit-when-empty 事件恰好一次纪律——owner A detach → promoteNextLocked
// 命中 rwEligible 但 outbox 预填至满（连升格 Welcome 都放不下 = 事实上 stalled）的
// 唯一递补者 B → 同义踢出（05-03 该踢出重扫分支正是为此边角所建）→ 注册表恰空 →
// kick 内 maybeExitWhenEmptyLocked 首次触发；外层 detach 的 maybeExitWhenEmptyLocked
// 在注册表仍空下二次到达——exitEmptySignaled 空纪元门闩使 exit_when_empty_wait
// 全程恰 1 条（修复前恰 2 条、remote 分属 B/A，计时器亦被 Stop 后重复武装）。
//
// 夹具论证（WR-01 触发条件叠加的确定性构造）：
//   - /bin/cat 无 INPUT 即静默——onChunk 不触发，B 不会被 onChunk 路径先踢
//     （ro 满即踢的唯一触发点是 trySend 失败的 chunk 到达，kickOrCreditLocked）；
//   - Options.PingInterval 零值 → pinger 禁用（New 对 PingInterval 无兜底直传 0，
//     pinger interval<=0 直接返回）——保活路径全程不介入本场景；
//   - B 的「outbox 满到连升格 Welcome 都写不进」由白盒出口 ShrinkOutboxForTest
//     注入（cap 改写为 1——trySend 对任何帧结构性必败）；不用真实字节填充：
//     writer 的 drain 是整批 swap 语义，填充与 drain 竞态下填满状态会在 promote
//     前被 drain 清空（实测），cap 改写无此窗口且与 TCP 吸收带/平台缓冲无关；
//   - grace=1min 计时器在测试窗口内不触发；cleanup killServer 后 lifecycle 置
//     exiting，到期回调复查静默返回——零跨测试 stderr 污染。
//
// 同步边：kick 的异步 Close 与正常读取的 B 完成关闭握手后即时收口，B 的
// handler 随连接终结返回——waitHandlers 返回即全部事件已落 stderr（logEvent
// 在 A handler 的 detach 内先于其返回，WaitGroup happens-before 先例同
// TestExitWhenEmptyTimerAfterLifecycle）。
//
// 14-03 双模式归一（:322 收编 newHandleTestServer——waitHandlers+srv 全保留）：
// per-client 列 = D-03 显式断言未装配（14-02 owner 四测四段式先例）——promote/
// 升格/同义踢出机械为 shared-only 装配（蓝本 :391 owner 递补行），A 断开后 B
// 零扰动：①B 恒 rw（无 D-07 降级——各端自有会话）；②注册表/会话账目收敛
// （clients==1 + pcSessions==1——A 会话收割完毕、B 在册未被踢）；③毒化 outbox
// （cap=1）在场下静默窗零帧零错误——promote 误装配即升格 Welcome trySend 必败
// 踢出 B（可证伪）；④INPUT 生效（恢复容量后 echo——自有会话无扰动）。尾部
// B 关闭 + Shutdown 收口：exiting 位使 1min 宽限计时器迟触发静默（per-client
// 无 shared 的 lifecycle-exiting 免疫结构，不收口即 +1min 迟触发 exit_when_empty
// 事件污染后继测试捕获窗）；exitf 经 supervisor last-reaped-code 收 -1。
func TestExitWhenEmptyPromoteKickOnce(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			restore := captureStderr(t)
			defer restore() // 失败路径兜底恢复 os.Stderr（幂等，limits_test.go 先例）

			// Writable 基线 true 由小族统一装配（owner 默认策略——WritePolicy
			// 零值兜底 owner，New 装配；per-client 无 write-policy 仲裁面）。
			exitCh, wsURL, waitHandlers, srv := newHandleTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.ExitWhenEmpty = true
				o.ExitWhenEmptyGrace = time.Minute
			})

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			// A：首个 rw attach → 立 owner（D-06）；B：后续 rw attach → D-07 降级 ro 进
			// 递补队列（rwEligible=true——promoteNextLocked 的唯一候选）。attachSeq 由
			// registerLocked 从 1 起顺序分配——本实例先 A 后 B，B 恒为 2。per-client
			// 列：B 恒 rw（自有会话，D-03 分叉点①）。
			cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
			if modeA != proto.ModeRW {
				t.Fatalf("A welcome mode = %q, want %q（首个 rw attach 立 owner）", modeA, proto.ModeRW)
			}
			cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
			if mode == server.SessionModePerClient {
				if modeB != proto.ModeRW {
					t.Fatalf("B welcome mode = %q, want %q（per-client 无 owner 降级——各端自有会话恒 rw，D-03）", modeB, proto.ModeRW)
				}
			} else if modeB != proto.ModeRO {
				t.Fatalf("B welcome mode = %q, want %q（D-07 降级进递补队列）", modeB, proto.ModeRO)
			}
			defer cB.CloseNow()

			// 注入「B stalled 到 outbox 连升格 Welcome 都写不进」：cap 改写为 1（任何帧
			// trySend 结构性必败）。B 保持正常 Read——cap 注入后读不读对 promote 的
			// trySend 失败无影响，且使 kick 的关闭握手即时完成（waitHandlers 不等写超时），
			// 顺带取 1013 客户端侧证据。per-client 列同一注入在场（毒化面——静默窗
			// 断言③的判别力载体）。
			if !srv.ShrinkOutboxForTest(2, 1) {
				t.Fatal("ShrinkOutboxForTest: B（attachSeq=2）不在注册表——夹具前提不成立")
			}

			// owner A 硬断 → detach(A) → promote 命中 B → 升格 Welcome trySend 必败 →
			// 同义踢出 B → 注册表恰空 → kick 内首次触发 + 外层 detach 二次到达。
			// per-client 列：A 断开仅终结其自有会话——注册表非空（B 在册），无 promote
			// 无踢出无空迁移。
			cA.CloseNow()

			if mode == server.SessionModePerClient {
				// ② 注册表/会话账目收敛：clients==1（B 在册未被踢）+ pcSessions==1
				//（A 会话 teardown HUP → 收割 → 删除完毕，B 会话存活）。
				convergeDeadline := time.Now().Add(2 * time.Second)
				for healthzClients(t, wsURL) != 1 || srv.PCSessionsLenForTest() != 1 {
					if time.Now().After(convergeDeadline) {
						t.Fatalf("A 断开后账目未收敛（clients=%d, pcSessions=%d）——want 1/1（B 未被踢、A 会话已收割）",
							healthzClients(t, wsURL), srv.PCSessionsLenForTest())
					}
					time.Sleep(10 * time.Millisecond)
				}

				// ③ 静默窗（1.5s，readPump 泵源——客户端 Read 永不带 per-read deadline）：
				// 毒化 outbox 在场下零帧零错误——promote 升格被误装配即 trySend 必败
				// 同义踢出 B（错误送达即翻车，D-03 红线可证伪面）。
				resCh := make(chan frameRes, 8)
				quit := make(chan struct{})
				defer close(quit)
				go readPump(ctx, cB, resCh, quit)
				window := time.After(1500 * time.Millisecond)
			noKick:
				for {
					select {
					case r := <-resCh:
						if r.err != nil {
							t.Fatalf("B 静默窗内 read error: %v（未被踢——连接应存活）", r.err)
						}
						if len(r.data) > 0 {
							t.Fatalf("B 静默窗内收到帧 %q——promote 升格/同义踢出被误装配进 per-client（D-03 红线）", r.data)
						}
					case <-window:
						break noKick
					}
				}

				// ④ INPUT 生效（自有会话无扰动）：恢复 outbox 容量（默认 512KiB，
				// clients.go defaultOutboxBytes——同一白盒出口反向改写）后 echo 回读。
				if !srv.ShrinkOutboxForTest(2, 512*1024) {
					t.Fatal("ShrinkOutboxForTest: B（attachSeq=2）容量恢复失败")
				}
				if err := cB.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, []byte("pc-no-promote-echo")...)); err != nil {
					t.Fatalf("write INPUT on B: %v", err)
				}
				accumFramesUntil(t, resCh, regexp.MustCompile("pc-no-promote-echo"))

				// 尾部收口（见函数头 14-03 注释）：B 关闭 + Shutdown——exiting 位静默
				// 1min 计时器；exitf 经 supervisor last-reaped-code（B 会话 teardown
				// HUP 信号死亡）收 -1。
				cB.CloseNow()
				srv.Shutdown()
				waitExit(t, exitCh, -1)
				assertNoExit(t, exitCh)
				return
			}

			// shared 列（v1.0 期望值逐字）：B 端客户端侧证据——1013 slow_consumer
			// 逐字（promote 同义踢出复用 R-10 命名族关闭帧）。outbox 空 + TCP 空 →
			// 关闭帧必达（CloseError 形态，无 EOF 变体面）。
			assertKicked1013(t, cB, 10*time.Second, "B (promotion target)")

			// 等 A/B handler 全返回——事件落 stderr 先于 handler 返回，restore 读与事件
			// 写由 WaitGroup 同步（startTrackedServerWith 同步边先例）。
			waitHandlers()
			out := restore()
			evs := parseEvents(t, out)

			// 主断言：空纪元事件恰好一次（修复前 2 条：kick 内 B 一条 + 外层 A 一条）。
			if n := countByEvent(evs, "exit_when_empty_wait"); n != 1 {
				t.Fatalf("exit_when_empty_wait count = %d, want 1（空纪元恰好一次纪律失守；out=%q）", n, out)
			}
			// grace>0 形态无立即事件（计时器 1min 未到期）。
			if n := countByEvent(evs, "exit_when_empty"); n != 0 {
				t.Fatalf("exit_when_empty count = %d, want 0（grace 计时器未到期；out=%q）", n, out)
			}
			// 正面证据：B 经 promote 同义踢出分支收口（detach reason=kick 恰 1 条）——
			// 场景真实走过「升格失败 → kick → 致空」路径，排除「B 先被其他路径移除、
			// detach(A) 直接致空」的假绿形态。
			kickN := 0
			for _, m := range evs {
				if m["event"] == "detach" && m["reason"] == "kick" {
					kickN++
				}
			}
			if kickN != 1 {
				t.Fatalf("detach reason=kick count = %d, want 1（promote 同义踢出分支证据；out=%q）", kickN, out)
			}
		})
	}
}
