package server_test

// emptyexit_test.go 锁定 SESS-01/02 服务端半侧行为（06-02）：注册表空触发断开退出
// 六路径——立即退出（grace=0）/宽限取消/宽限到期/kick 第二移除路径触发/exiting 门
// 抑制终结广播误触发/宽限计时器晚于 lifecycle 到期不补发（review #5 吸收）。helper
// 复用 e2e_test.go/exit_test.go/slowclient_test.go/limits_test.go 同包零改动
// （06-PATTERNS exact）。
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
func TestExitWhenEmptyImmediate(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	c.Close(websocket.StatusNormalClosure, "")

	// SIGHUP 致死 → ExitCode()=-1 → exitf(-1)（恰好一次：termOnce 收口 + 200ms
	// 静默锁定无第二次）。
	waitExit(t, exitCh, -1)
	assertNoExit(t, exitCh)
}

// TestExitWhenEmptyGraceCancel（SESS-02 宽限取消，D-14）：grace=2s → 唯一客户端
// 断开启动宽限计时 → 300ms 后再 attach 成功（registerLocked 取消点 Stop+置 nil，
// 恰好一次）→ echo 验证会话存活 → 越过旧计时器到期点 exitCh 静默（取消实证）→
// 再次断开重新计时 → 到期收码。
//
// plan 字面「1.5s 时点静默」早于旧 timer 到期点 2s，不构成取消证据（算术松散）——
// 按 parenthetical 语义要求（「旧 timer 若未取消此时已到期」）延长静默窗越过到期点
// +500ms 余量（deviation 登记）。
func TestExitWhenEmptyGraceCancel(t *testing.T) {
	const grace = 2 * time.Second
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: grace,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c1, _ := dialHello(t, ctx, wsURL, 80, 24)
	closedAt := time.Now()
	c1.Close(websocket.StatusNormalClosure, "") // 注册表空迁移 → 宽限计时启动

	// 宽限内 300ms 再 attach：registerLocked 成功 → 计时取消（恰好一次，置 nil
	// 防重复 Stop 与重复事件）。
	time.Sleep(300 * time.Millisecond)
	c2, _ := dialHello(t, ctx, wsURL, 80, 24)

	// echo 验证会话存活（宽限取消后会话继续——D-14「计时内任一端 attach 成功则
	// 退出取消、会话继续」）。
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
	//（grace+2s 余量内收码；accept-255 断言常量 -1）。
	c2.Close(websocket.StatusNormalClosure, "")
	select {
	case code := <-exitCh:
		if code != -1 {
			t.Fatalf("exit code = %d, want -1（accept-255 门裁决断言常量）", code)
		}
	case <-time.After(grace + 2*time.Second):
		t.Fatal("exitf not called within grace+2s after second detach — re-armed timer did not fire")
	}
}

// TestExitWhenEmptyGraceExpire（SESS-02 宽限到期，D-14）：grace=400ms → 唯一客户端
// 断开启动宽限计时 → 不再 attach → 到期前 100ms 时点 exitCh 静默（不过早退出）→
// 到期 SIGHUP → 3s 内收 -1 恰好一次（AfterFunc 单次触发 + termOnce 双保险）。
func TestExitWhenEmptyGraceExpire(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 400 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	c.Close(websocket.StatusNormalClosure, "")

	// 到期前 100ms 时点静默（grace=400ms 远未到期——不过早退出）。
	select {
	case code := <-exitCh:
		t.Fatalf("exitf called with code %d at 100ms, before grace expiry (400ms) — premature exit", code)
	case <-time.After(100 * time.Millisecond):
	}

	// 到期 SIGHUP → lifecycle 收口：3s 内收 -1（accept-255 断言常量）恰好一次。
	select {
	case code := <-exitCh:
		if code != -1 {
			t.Fatalf("exit code = %d, want -1（accept-255 门裁决断言常量）", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("exitf not called within 3s after grace expiry")
	}
	assertNoExit(t, exitCh)
}

// TestExitWhenEmptyKickTrigger（SESS-01/02 第二移除路径）：stall 客户端被 1013
// slow_consumer 踢出（kickSlowConsumerLocked——removeLocked 第二调用点）→ 注册表
// 空迁移 → grace=0 立即 SIGHUP → 5s 内收 -1。stall 夹具形态照抄
// slowclient_test.go（OutboxBytes 小值覆写 + seq 洪水）；Writable:false 使唯一
// 客户端为 ro——ro 满即踢永不持信用（R-08 分工表；rw 独端会落信用门持块而非
// 被踢，踢出断言结构性不成立）。
//
// 断言序（05-07 登记的 stall 夹具戒律——踢出触发前绝不 Read）：先等 exitf(-1)——
// 本夹具下无人读取时令 seq 死亡的唯一路径就是 outbox 写满 → 1013 踢出 → 注册表
// 空迁移 → SIGHUP（38.9MB 洪水 ≫ 不读取客户端 ~4-10MiB 吸收量，子进程结构性
// 不可能自然跑完），exitf(-1) 到达即踢出与空触发的结构性证据；随后再读连接取证
// 1013 关闭帧。反序（先 assertKicked1013）会让 readUntilError 立刻开始排空管道，
// 踢出永不成立（实测竞态：~50% 跑到子进程自然退出收 1000）。
func TestExitWhenEmptyKickTrigger(t *testing.T) {
	// seq 1 5000000 ≈ 38.9MB 洪水（slowclient_test 先例量级：> 单连接最坏吸收
	// ~10MiB + 64KiB outbox——stall 必然传导到 outbox 写满）。
	exitCh, wsURL := startTestServerWith(t, []string{"seq", "1", "5000000"}, server.Options{
		Writable:           false,
		OutboxBytes:        64 * 1024,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 0,
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
	// SIGHUP；accept-255 断言常量 -1）恰好一次。
	waitExit(t, exitCh, -1)
	assertNoExit(t, exitCh)
	// 取证踢出关闭帧：1013 slow_consumer 逐字（踢出事件的客户端可见形态）。
	assertKicked1013(t, stall, 15*time.Second, "stall client")
	stall.CloseNow()
}

// TestExitWhenEmptyLifecycleGate（planner 推导不变量，D-13 防线）：ExitWhenEmpty
// grace=0 + argv `sh -c 'sleep 1; exit 42'` → 客户端在线等子进程自然退出 → 收
// EXIT{exit_code:42} + 1000（06-01 形态）→ exitf(42) 恰好一次——广播 Close 引发的
// detach 致空被 exiting 门抑制（无门则广播期 detach 再生 SIGHUP/误导性
// exit_when_empty 事件；自然退出 exit 42 路径的 SIGHUP 翻码竞态防线）。
func TestExitWhenEmptyLifecycleGate(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"sh", "-c", "sleep 1; exit 42"}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 0,
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
		t.Fatalf("EXIT exit_code = %d, want 42（exiting 门抑制空触发——被 SIGHUP 翻码则为 -1）", ep.ExitCode)
	}

	// exitf 以子进程原码 42 恰好一次收口（SIGHUP 未染指本路径的语义证据）。
	waitExit(t, exitCh, 42)
	assertNoExit(t, exitCh)
}

// TestExitWhenEmptyTimerAfterLifecycle（review #5 吸收——宽限计时器在 lifecycle
// 启动后到期不得再触发）：grace=2s → 客户端 close 启动宽限计时（到期 ≈close+2s）→
// 子进程 ~1s 自然退出先于计时器到期 → lifecycle 置 exiting + exitf(42) → 收 42
// 恰好一次 → 越过计时器到期点（close+3s 护栏）无第二次收码，且 stderr 无
// exit_when_empty 触发事件（事件名精确相等区分——exit_when_empty_wait 启动
// 事件允许存在；JSON 字段语义下两事件名独立，无前缀歧义）。
// 回归形态 = 计时器回调缺 exiting 复查时向已终结会话补发 SIGHUP 并打误导性触发日志
// （review『confusing log』关切的可执行闭合）。
func TestExitWhenEmptyTimerAfterLifecycle(t *testing.T) {
	restore := captureStderr(t)
	defer restore() // 失败路径兜底恢复 os.Stderr（幂等，limits_test.go 先例）

	exitCh, wsURL, waitHandlers := startTrackedServerWith(t, []string{"sh", "-c", "sleep 1; exit 42"}, server.Options{
		Writable:           true,
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	closedAt := time.Now()
	c.Close(websocket.StatusNormalClosure, "") // 注册表空迁移 → 宽限计时启动（到期 ≈closedAt+2s）

	// 子进程 ~1s 自然退出（先于计时器到期）→ lifecycle 快照前置 exiting →
	// exitf(42) 恰好一次。
	waitExit(t, exitCh, 42)

	// 越过计时器到期点（closedAt+3s 护栏）：无第二次收码（回调 exiting 复查守卫；
	// 回归形态下回调到期补发 SIGHUP——exitf 仍被 termOnce 拦住，此断言由下方
	// stderr 触发行零次承接）。
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
	// return（exiting 复查），不触 os.Stderr。
	waitHandlers()
	out := restore()
	evs := parseEvents(t, out)
	if n := countByEvent(evs, "exit_when_empty"); n != 0 {
		t.Fatalf("stderr contains exit_when_empty trigger event after lifecycle (count=%d, out=%q) — timer callback missing exiting recheck", n, out)
	}
	// 启动事件允许存在且必须存在（事件名精确相等的正面证据——wait 事件在
	// 证明计时器真实武装过，触发事件零次才有意义）。
	if n := countByEvent(evs, "exit_when_empty_wait"); n != 1 {
		t.Fatalf("stderr missing exit_when_empty_wait start event (count=%d, out=%q) — grace timer did not start on detach", n, out)
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
func TestExitWhenEmptyPromoteKickOnce(t *testing.T) {
	restore := captureStderr(t)
	defer restore() // 失败路径兜底恢复 os.Stderr（幂等，limits_test.go 先例）

	exitCh, wsURL, waitHandlers, srv := startTrackedServerHandle(t, []string{"/bin/cat"}, server.Options{
		Writable:           true, // owner 默认策略（WritePolicy 零值兜底 owner，New 装配）
		ExitWhenEmpty:      true,
		ExitWhenEmptyGrace: time.Minute,
	})
	_ = exitCh // 本测试不断言子进程退出（cleanup killServer 收口）

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// A：首个 rw attach → 立 owner（D-06）；B：后续 rw attach → D-07 降级 ro 进
	// 递补队列（rwEligible=true——promoteNextLocked 的唯一候选）。attachSeq 由
	// registerLocked 从 1 起顺序分配——本实例先 A 后 B，B 恒为 2。
	cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
	if modeA != proto.ModeRW {
		t.Fatalf("A welcome mode = %q, want %q（首个 rw attach 立 owner）", modeA, proto.ModeRW)
	}
	cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
	if modeB != proto.ModeRO {
		t.Fatalf("B welcome mode = %q, want %q（D-07 降级进递补队列）", modeB, proto.ModeRO)
	}
	defer cB.CloseNow()

	// 注入「B stalled 到 outbox 连升格 Welcome 都写不进」：cap 改写为 1（任何帧
	// trySend 结构性必败）。B 保持正常 Read——cap 注入后读不读对 promote 的
	// trySend 失败无影响，且使 kick 的关闭握手即时完成（waitHandlers 不等写超时），
	// 顺带取 1013 客户端侧证据。
	if !srv.ShrinkOutboxForTest(2, 1) {
		t.Fatal("ShrinkOutboxForTest: B（attachSeq=2）不在注册表——夹具前提不成立")
	}

	// owner A 硬断 → detach(A) → promote 命中 B → 升格 Welcome trySend 必败 →
	// 同义踢出 B → 注册表恰空 → kick 内首次触发 + 外层 detach 二次到达。
	cA.CloseNow()

	// B 端客户端侧证据：1013 slow_consumer 逐字（promote 同义踢出复用 R-10 命名族
	// 关闭帧）。outbox 空 + TCP 空 → 关闭帧必达（CloseError 形态，无 EOF 变体面）。
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
}
