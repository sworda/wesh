package server

// export_test.go —— 测试期专属出口（仅 -test 编译，零生产 API 面）。

import (
	"time"

	"github.com/sworda/wesh/internal/pty"
)

// LockStderr 供 server_test 包的 captureStderr 在置换/恢复 os.Stderr 两个
// 写点持写锁：与 stderrW.Write 的 RLock 配对，把「置换写」与「在途事件读」
// 串行化（08-01 门禁修正——跨测试遗留 handler 的事件写出曾与下一测试的
// 置换构成 data race）。返回解锁函数；调用方必须在置换语句后立即解锁，
// 禁止整个捕获期持锁（事件写出会全部阻塞，waitHandlers 死锁）。
func LockStderr() func() {
	stderrMu.Lock()
	return stderrMu.Unlock
}

// ShrinkOutboxForTest 把 attachSeq 对应客户端的 outbox 容量改写为 newCap
// （08-review WR-01 回归夹具，emptyexit_test.go TestExitWhenEmptyPromoteKickOnce）：
// newCap 小于升格 Welcome 帧长时 promoteNextLocked 的 trySend 结构性必败
// （bytes≥0 ⇒ bytes+len(frame) > cap 恒成立）——「递补者 stalled 到 outbox 连升格
// 通知都写不进」的确定性等价注入。不用真实字节填充的理由：writer 的 drain 是
// 整批 swap 语义，填充与 drain 竞态下「填满」状态会在填充返回后、promote 前被
// 一次 drain 清空（实测）；改写 cap 无此窗口，且与 TCP 吸收带/平台缓冲无关。
//
// 故障注入语义仅服务测试：生产路径 outbox cap 自 newOutbox 后不变（New 装配期
// 固化）；afterDrain 的「cap ≥ 64KiB」数学保证注释针对生产配置，本注入刻意
// 制造 promote 失败边角。调用方不得持 hubMu；返回 false = 该 seq 不在注册表。
func (s *Server) ShrinkOutboxForTest(seq int64, newCap int) bool {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	for c := range s.registry.set {
		if c.attachSeq != seq {
			continue
		}
		c.outbox.mu.Lock()
		c.outbox.cap = newCap
		c.outbox.mu.Unlock()
		return true
	}
	return false
}

// 11-03 观测出口（D-02/D-03 容量闸测试的数据源）：返回 per-client 会话活性
// 注册表 pcSessions 的当前登记数（hubMu 内 len 读）。per-client 注册表活性 =
// 未收割会话数——含断开待收割 linger 形态（断开 SIGHUP 已发但 trap 免疫的
// 会话登记至 Wait 收割完成，容量再闸测试的确定性注入载体）。shared 模式
// pcSessions 为 nil——len(nil)==0 自然成立。故障注入语义仅服务测试：生产
// 路径 pcSessions 仅经 upgradePerClient 登记与 teardown 慢半段移除。调用方
// 不得持 hubMu（上方 ShrinkOutboxForTest 注释形态同款纪律）。
func (s *Server) PCSessionsLenForTest() int {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	return len(s.pcSessions)
}

// 12-03 观测出口（PC-10/PC-11 停读/续读/dwell 测试的数据源）：返回
// registry.gateTransitions 当前值（hubMu 内读）。递增点 = shared 信用门
// 置位/清位（kickOrCreditLocked/afterDrain）与 per-client 停读/续读
// （perclient.go ReadLoop 输出闭包，D-05 mode-agnostic 聚合）——per-client
// 单模式实例下只有后两者可达，差值断言即闭包两递增点的直接观测。故障注入
// 语义仅服务测试：生产路径本计数只经 metrics.go snapshotMetrics 锁内快照
// 读取（wesh_credit_gate_transitions_total 既有 series）。调用方不得持
// hubMu（上方 ShrinkOutboxForTest 注释形态同款纪律）。
func (s *Server) GateTransitionsForTest() int {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	return s.registry.gateTransitions
}

// 13-02 观测出口（PC-08 spawn 节流 tracer e2e 的计数器递增观测）：返回
// ptySpawnThrottled 计数器当前值（atomic Load——mc 注释的锁外读合法形态）。
// 本阶段 /metrics 尚无对应 series（wesh_pty_spawn_throttled_total 输出归
// 13-05 接线），tracer e2e 经本出口观测递增；GateTransitionsForTest 观测
// 出口先例同形态。故障注入语义仅服务测试。
func (s *Server) PTYSpawnThrottledForTest() int64 {
	return s.mc.ptySpawnThrottled.Load()
}

// 13-02 直调薄桥（Task 2 惰性过期单元级断言的数据源）：spawnThrottleStore
// 与 allow 为包内私有，server_test 经本包装类型以 now 注入直调（时间注入面 =
// throttleStore recordFail(now) 先例）；ForTest 四件套纪律——仅 -test 编译，
// 零生产 API 面，包装直通构造/调用不改变被测行为。
type SpawnThrottleProbeForTest struct {
	st *spawnThrottleStore
}

// NewSpawnThrottleProbeForTest 构造直调探针（newSpawnThrottleStore 四参数
// 直通——零值兜底语义同生产构造）。
func NewSpawnThrottleProbeForTest(globalRate, globalBurst, perIPRate, perIPBurst int) *SpawnThrottleProbeForTest {
	return &SpawnThrottleProbeForTest{st: newSpawnThrottleStore(globalRate, globalBurst, perIPRate, perIPBurst)}
}

// Allow 时间注入直调（store.allow 同签名同语义）。
func (p *SpawnThrottleProbeForTest) Allow(ip string, now time.Time) bool {
	return p.st.allow(ip, now)
}

// TeardownReapedFenceForTest（13-03 WR-02 白盒构造出口，PC-09/T-13-10）：
// 构造「waitDone 已预关闭 + reaped 未置位」的 pcSession 并持 hubMu 直调
// teardownPCLocked，返回 teardownDone 通道供调用方断言收口完成。该状态在
// 正常时序下不可观测——sessionWatcher 序列中 close(waitDone) 与 hubMu 内
// 置 reaped 相邻建立（Wait-return→hubMu-acquire 微窗口 = Phase 11 REVIEW
// WR-02 登记的 kill-after-reap 理论面），本出口将其确定性注入：被测行为
// = WR-02 栅栏跳过信号面后收口链仍完整（Drain/Close/waitDone/delete/
// teardownDone）。行为面不可区分「未发信号」半边（kill 已收割 pgid 收
// ESRCH 静默）由 perclient_test.go TestPerClientReapedFence 的源码 region
// 断言承载。ForTest 四件套纪律：仅 -test 编译零生产 API 面；调用方不得
// 持 hubMu（上方 ShrinkOutboxForTest 同款纪律）；传入真实 *pty.Session
// （慢半段 Drain/Close 对真实 master fd 收口，测试侧为本会话唯一 Wait
// 调用方——无 watcher 无僵尸残留）。
func (s *Server) TeardownReapedFenceForTest(pcSess *pty.Session) <-chan struct{} {
	pc := &pcSession{
		sess:         pcSess,
		inQ:          newInputQ(defaultInputQueueBytes),
		inputDone:    make(chan struct{}),
		waitDone:     make(chan struct{}),
		teardownDone: make(chan struct{}),
		startedAt:    time.Now(),
		resizeDeb:    newDebouncer(time.Hour, func() {}),
	}
	close(pc.waitDone) // 预关闭：Wait 已返回形态（内核态 reap 完成点）
	// reaped 零值 false——「置位在途」微窗口注入（构造即栅栏触发面）。
	s.hubMu.Lock()
	s.teardownPCLocked(pc)
	s.hubMu.Unlock()
	return pc.teardownDone
}
