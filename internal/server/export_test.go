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
