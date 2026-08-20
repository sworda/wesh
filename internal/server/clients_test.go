package server

import "testing"

// clients_test.go —— TestClientCountInvariant（VALIDATION 05-01-08，RES-03 /
// T-05-04b 计数器泄漏防线，review #7）：registry 内嵌计数器 n 与成员集 set 的
// 对称不变量白盒锁定。同包白盒（tickets_test.go 先例——内部类型不导出，
// 直构造 registry 零值可用，无需 Server/pty 装配）。
//
// 不变量论证的结构半侧（clients.go registry.n 字段注释逐字登记）：运行期全部
// 移除路径——reader-error detach / kickOrCreditLocked 1013 踢出 / pinger
// CloseNow 后 reader 错误 detach——均收口于 removeLocked 单点，加减排它性对称，
// 漂移结构上不可能；lifecycle 广播后进程即退出（计数无后续读者）、panic 按 Go
// 语义进程崩溃（无恢复面）——两路径无漂移窗口。本测试锁定机械半侧：交错序列
// 逐步断言 + 幂等防御。

// TestClientCountInvariant：N 次 registerLocked 与交错 removeLocked 序列，每步
// 断言 r.n.Load() == int64(len(r.set))（逐步不变量——计数与成员集恒同步的机械
// 证据）；非成员 removeLocked 幂等 no-op 且计数不变（Pitfall 4 防线：减计数不得
// 越过实际移除——重复移除/幽灵移除若穿透减计数，运行时间增长计数静默低于实际
// 占用，③位 503 闸形同虚设：全员被拒而注册表实际为空）。
func TestClientCountInvariant(t *testing.T) {
	var r registry // 零值可用（registerLocked 惰性建 map）
	step := 0
	assertInvariant := func() {
		t.Helper()
		step++
		if got, want := r.n.Load(), int64(len(r.set)); got != want {
			t.Fatalf("step %d: n = %d, len(set) = %d——计数对称不变量破坏（review #7）", step, got, want)
		}
	}
	assertInvariant() // 零值起点：0 == 0

	// N 次注册，逐步断言。
	const n = 16
	clients := make([]*client, n)
	for i := range clients {
		clients[i] = &client{}
		r.registerLocked(clients[i])
		assertInvariant()
	}
	// 交错移除（奇数下标），逐步断言。
	for i := 1; i < n; i += 2 {
		if !r.removeLocked(clients[i]) {
			t.Fatalf("removeLocked(成员 #%d) = false, want true", i)
		}
		assertInvariant()
	}
	// 幂等防御：重复移除（刚移除的奇数成员）与幽灵移除（从未注册者）均为
	// no-op 且计数不变。
	if r.removeLocked(clients[1]) {
		t.Fatal("重复移除返回 true——幂等不变量破坏（Pitfall 4：减计数越过实际移除）")
	}
	assertInvariant()
	if r.removeLocked(&client{}) {
		t.Fatal("幽灵移除（从未注册）返回 true——幂等不变量破坏")
	}
	assertInvariant()
	// 再注册补充（先前移除的奇数位重新入册）+ 全量移除至空，逐步断言。
	for i := 1; i < n; i += 2 {
		r.registerLocked(clients[i])
		assertInvariant()
	}
	for i := 0; i < n; i++ {
		if !r.removeLocked(clients[i]) {
			t.Fatalf("removeLocked(成员 #%d) = false, want true", i)
		}
		assertInvariant()
	}
	if r.n.Load() != 0 {
		t.Fatalf("全量移除后 n = %d, want 0（计数泄漏，T-05-04b）", r.n.Load())
	}
}
