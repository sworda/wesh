package server

// export_test.go —— 测试期专属出口（仅 -test 编译，零生产 API 面）。

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
//（08-review WR-01 回归夹具，emptyexit_test.go TestExitWhenEmptyPromoteKickOnce）：
// newCap 小于升格 Welcome 帧长时 promoteNextLocked 的 trySend 结构性必败
//（bytes≥0 ⇒ bytes+len(frame) > cap 恒成立）——「递补者 stalled 到 outbox 连升格
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
