package server

import (
	"sync"
	"time"
)

// D-09 研究标定（RESEARCH Pattern 3 参数表）：OWASP Authentication Cheat Sheet
// 原文 1s 起翻倍锚点；cap 30s 使爆破 100 次累计等待 ≥47min（ROADMAP 准则 2
// 「可观测退避」验收锚点），稳态限速 2 次/分钟/IP。
const (
	defaultThrottleBase = 1 * time.Second
	defaultThrottleCap  = 30 * time.Second
)

// throttleEntry per-IP 失败计数与退避窗口。
type throttleEntry struct {
	fails     int
	notBefore time.Time // 该时刻前一律拒绝
	lastSeen  time.Time // 惰性过期依据（>15min 未活动重置）
}

// throttleStore per-IP 指数退避节流计数器（SEC-03；D-08 两处失败——/api/attach
// 凭据失败与 Hello ticket 核销失败——统一计入本计数器）。不变量：
//   - 认证成功 recordSuccess delete 清零（D-08），下次失败从 base 重新起级；
//   - 窗口内 allow 命中只读不改条目——不延长 notBefore（恢复期可预期，
//     稳态限速已达，不给攻击者窗口操控面）；
//   - lastSeen 超 15min 的条目在 recordFail 内惰性重置（Pitfall 4 内存界纪律，
//     halfOpenCounter 同款；15min > 30s cap 一个数量级以上，不误清活跃退避）；
//   - mu+map 最小形态，无常驻 janitor goroutine（零新 exitf 分支纪律）。
//
// 不做临时锁定（lockout）状态机：个人工具单/少用户场景，封顶退避已达稳态
// 2 次/分钟/IP 限速，锁定反而增加「锁定期 vs 正常期」可区分 oracle（RESEARCH
// Pattern 3 裁决）。内存界：条目约 56B，4096 恶意 IP ≈ 230KB 可接受；惰性过期
// 已回收非活跃条目，不设硬上限驱逐（随机驱逐复杂度换不到安全收益）。
type throttleStore struct {
	mu   sync.Mutex
	m    map[string]throttleEntry
	base time.Duration // 默认 1s；测试经 newThrottleStore 参数覆写提速（HelloTimeout 先例）
	cap  time.Duration // 默认 30s
}

// newThrottleStore 构造节流计数器；base/cap 零值分别兜底 defaultThrottleBase/
// defaultThrottleCap（零值可用纪律，Options 零值兜底先例）。
func newThrottleStore(base, cap time.Duration) *throttleStore {
	if base <= 0 {
		base = defaultThrottleBase
	}
	if cap <= 0 {
		cap = defaultThrottleCap
	}
	return &throttleStore{m: make(map[string]throttleEntry), base: base, cap: cap}
}

// allow 返回 false 表示该 IP 处于退避窗口（调用方以 429/auth_failed 同口径拒绝）。
// 只读不改条目——窗口内命中不延长 notBefore。
func (t *throttleStore) allow(ip string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.m[ip]
	if !ok {
		return true
	}
	return !now.Before(e.notBefore)
}

// recordFail 记一次失败并后移退避窗口：d = base << min(fails-1, 5)（位移即 ×2 幂，
// 级数 1/2/4/8/16/32s），超 cap 截断（1s/30s 默认下实际级数 1/2/4/8/16/30/30…）。
// lastSeen 超 15min 的条目先重置为零值——惰性过期，map 内存界防线。
func (t *throttleStore) recordFail(ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.m[ip]
	if now.Sub(e.lastSeen) > 15*time.Minute {
		e = throttleEntry{} // 惰性过期重置（map 上界纪律）
	}
	e.fails++
	d := t.base << min(e.fails-1, 5)
	if d > t.cap {
		d = t.cap
	}
	e.notBefore = now.Add(d)
	e.lastSeen = now
	t.m[ip] = e
}

// recordSuccess 认证成功清零（D-08）：delete 条目，下次失败从 base 重新起级。
func (t *throttleStore) recordSuccess(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, ip)
}
