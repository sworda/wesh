package server

import (
	"testing"
	"time"
)

// TestThrottleStore 锁定 per-IP 指数退避计数器语义（SEC-03，D-08/D-09 标定）：
// 级数 1/2/4/8/16/30/30（位移即 ×2 幂，cap 封顶）/ 窗口内命中不延长 notBefore /
// 成功清零（D-08）/ lastSeen 超 15min 惰性重置（Pitfall 4 内存界纪律）。
// 全部经 now 手工注入推进，零真实 sleep；同包白盒（同 tickets_test.go 纪律）。
func TestThrottleStore(t *testing.T) {
	base := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	t.Run("未知 IP allow true", func(t *testing.T) {
		ts := newThrottleStore(time.Second, 30*time.Second)
		if !ts.allow("203.0.113.1", base) {
			t.Errorf("未知 IP allow = false, want true（无条目即放行）")
		}
	})

	t.Run("首次失败起级且窗口内命中不延长", func(t *testing.T) {
		ts := newThrottleStore(time.Second, 30*time.Second)
		now := base
		ip := "203.0.113.7"
		ts.recordFail(ip, now)
		e := ts.m[ip]
		if e.fails != 1 {
			t.Errorf("fails = %d, want 1", e.fails)
		}
		if !e.notBefore.Equal(now.Add(time.Second)) {
			t.Errorf("notBefore = %v, want %v（第 1 次失败恰好 now+base）", e.notBefore, now.Add(time.Second))
		}
		if ts.allow(ip, now.Add(500*time.Millisecond)) {
			t.Errorf("窗口内 allow = true, want false")
		}
		// 窗口内重复 allow 只读：notBefore 与 fails 均不变（恢复期可预期）。
		ts.allow(ip, now.Add(600*time.Millisecond))
		ts.allow(ip, now.Add(700*time.Millisecond))
		if got := ts.m[ip]; !got.notBefore.Equal(e.notBefore) || got.fails != 1 {
			t.Errorf("窗口内 allow 后条目 = %+v, want notBefore=%v fails=1（不延长窗口）", got, e.notBefore)
		}
		if !ts.allow(ip, now.Add(time.Second)) {
			t.Errorf("窗口到期 allow = false, want true（now 不早于 notBefore）")
		}
	})

	t.Run("指数退避级数与 cap 封顶", func(t *testing.T) {
		ts := newThrottleStore(time.Second, 30*time.Second)
		now := base
		ip := "203.0.113.9"
		// D-09 标定：base<<min(fails-1,5)，超 cap 截断 → 1/2/4/8/16/30/30。
		want := []time.Duration{1, 2, 4, 8, 16, 30, 30}
		for i, sec := range want {
			ts.recordFail(ip, now)
			e := ts.m[ip]
			if e.fails != i+1 {
				t.Fatalf("第 %d 次失败 fails = %d, want %d", i+1, e.fails, i+1)
			}
			if wantNB := now.Add(sec * time.Second); !e.notBefore.Equal(wantNB) {
				t.Errorf("第 %d 次失败 notBefore = now+%v, want now+%vs", i+1, e.notBefore.Sub(now), sec)
			}
		}
	})

	t.Run("成功清零后从 base 重新起级", func(t *testing.T) {
		ts := newThrottleStore(time.Second, 30*time.Second)
		now := base
		ip := "203.0.113.11"
		ts.recordFail(ip, now)
		ts.recordFail(ip, now)
		ts.recordSuccess(ip)
		if _, ok := ts.m[ip]; ok {
			t.Fatalf("recordSuccess 后条目仍在 map——D-08 成功清零要求 delete")
		}
		if !ts.allow(ip, now) {
			t.Errorf("清零后 allow = false, want true")
		}
		ts.recordFail(ip, now)
		if e := ts.m[ip]; e.fails != 1 || !e.notBefore.Equal(now.Add(time.Second)) {
			t.Errorf("清零后再失败条目 = %+v, want fails=1 notBefore=now+base（从 base 重启）", e)
		}
	})

	t.Run("lastSeen 超 15min 惰性重置", func(t *testing.T) {
		ts := newThrottleStore(time.Second, 30*time.Second)
		now := base
		ip := "203.0.113.13"
		ts.recordFail(ip, now)
		ts.recordFail(ip, now)
		ts.recordFail(ip, now) // fails=3
		now2 := now.Add(16 * time.Minute)
		ts.recordFail(ip, now2) // 惰性过期：视为新 IP 从 base 起级
		e := ts.m[ip]
		if e.fails != 1 {
			t.Errorf("16min 后再失败 fails = %d, want 1（惰性重置）", e.fails)
		}
		if !e.notBefore.Equal(now2.Add(time.Second)) {
			t.Errorf("重置后 notBefore = now2+%v, want now2+base", e.notBefore.Sub(now2))
		}
		if !e.lastSeen.Equal(now2) {
			t.Errorf("lastSeen = %v, want %v（随 recordFail 刷新）", e.lastSeen, now2)
		}
	})

	t.Run("零值 base/cap 兜底默认常量", func(t *testing.T) {
		ts := newThrottleStore(0, 0)
		if ts.base != defaultThrottleBase || ts.cap != defaultThrottleCap {
			t.Errorf("newThrottleStore(0,0) = base %v cap %v, want %v/%v",
				ts.base, ts.cap, defaultThrottleBase, defaultThrottleCap)
		}
		if defaultThrottleBase != time.Second || defaultThrottleCap != 30*time.Second {
			t.Errorf("默认常量 = %v/%v, want 1s/30s（D-09 标定：OWASP 1s 翻倍锚点）",
				defaultThrottleBase, defaultThrottleCap)
		}
	})

	t.Run("爆破 100 次累计等待至少 47min（ROADMAP 准则 2 锚点）", func(t *testing.T) {
		ts := newThrottleStore(0, 0) // 生产默认 1s/30s
		now := base
		var total time.Duration
		for i := 0; i < 100; i++ {
			ts.recordFail("198.51.100.23", now)
			total += ts.m["198.51.100.23"].notBefore.Sub(now)
		}
		// 级数和 = 1+2+4+8+16 + 95×30s = 2881s ≈ 48min；稳态限速 60s/30s = 2 次/分钟/IP。
		if total < 47*time.Minute {
			t.Errorf("100 次爆破累计等待 = %v, want ≥47min（可观测退避锚点）", total)
		}
	})

	// 03-03 追加：retryAfter 只读访问器（429 Retry-After 头数据源）。
	t.Run("retryAfter 剩余等待只读不延长窗口", func(t *testing.T) {
		ts := newThrottleStore(time.Second, 30*time.Second)
		now := base
		ip := "203.0.113.17"
		// 未知 IP：(0, false)。
		if wait, ok := ts.retryAfter(ip, now); ok || wait != 0 {
			t.Errorf("未知 IP retryAfter = (%v, %v), want (0, false)", wait, ok)
		}
		// fail#1 后窗口内：返回剩余等待（base - 已逝）。
		ts.recordFail(ip, now)
		if wait, ok := ts.retryAfter(ip, now.Add(250*time.Millisecond)); !ok || wait != 750*time.Millisecond {
			t.Errorf("窗口内 retryAfter = (%v, %v), want (750ms, true)", wait, ok)
		}
		// 窗口内重复调用后 notBefore 不变（只读纪律，与 allow 同款不延长不变量）。
		nb := ts.m[ip].notBefore
		ts.retryAfter(ip, now.Add(300*time.Millisecond))
		ts.retryAfter(ip, now.Add(400*time.Millisecond))
		if got := ts.m[ip].notBefore; !got.Equal(nb) {
			t.Errorf("retryAfter 重复调用后 notBefore = %v, want %v（只读不延长）", got, nb)
		}
		// 窗口已过（now 不早于 notBefore）：(0, false)。
		if wait, ok := ts.retryAfter(ip, now.Add(time.Second)); ok || wait != 0 {
			t.Errorf("窗口过 retryAfter = (%v, %v), want (0, false)", wait, ok)
		}
	})
}
