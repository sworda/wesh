package server

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// defaultTicketTTL 一次性 ticket 默认存活期 60s（ROADMAP 锁定）；
// 测试经 newTicketStore 参数覆写提速（HelloTimeout 先例）。
const defaultTicketTTL = 60 * time.Second

// ticketEntry 单条 ticket 登记项。
type ticketEntry struct {
	// proto.ModeRO/ModeRW，签发时绑定（05-06 注释兑现——两签发通道，结构零改动）：
	// Basic 通道 = 全局 --writable 派生模式（D-11，server.go attachHandler）；
	// 分享 token 通道 = token 绑定 mode（D-01，server.go shareAttach）。
	mode string
	exp  time.Time // 过期时刻；过期按不存在处理（D-10 同口径）
}

// ticketStore 一次性 ticket 表（SEC-02）：签发 = crypto/rand 随机 + 登记，
// 核销 = 原子查删。不变量：
//   - 核销即删（单次使用）：查即删使重放/过期/非法同归 false——
//     D-10 统一口径，不给攻击者区分 oracle；
//   - 签发顺手机会性清扫过期项：map 不随历史签发单调增长（Pitfall 4），
//     无常驻 janitor goroutine（零新 exitf 分支纪律）；
//   - mu+map 最小形态，零值可用字段经 newTicketStore 兜底默认常量。
type ticketStore struct {
	mu  sync.Mutex
	m   map[string]ticketEntry // key = base64url(16B random)
	ttl time.Duration
}

// newTicketStore 构造 ticket 表；ttl <= 0 时兜底 defaultTicketTTL（零值可用纪律，
// Options 零值兜底先例）。
func newTicketStore(ttl time.Duration) *ticketStore {
	if ttl <= 0 {
		ttl = defaultTicketTTL
	}
	return &ticketStore{m: make(map[string]ticketEntry), ttl: ttl}
}

// issue 签发绑定 mode 的一次性 ticket：crypto/rand 16 字节 → base64.RawURLEncoding
// 22 字符。ticket 与静态凭据是独立 secret——crypto/rand 直接生成，不从凭据派生
// （PITFALLS C6 锁定项：可预测 ticket = 认证绕过；128bit 空间使在线枚举无意义）。
func (ts *ticketStore) issue(mode string, now time.Time) string {
	var b [16]byte
	_, _ = rand.Read(b[:])                          // crypto/rand 失败即进程级问题，沿用 Go 惯例可读性处理
	t := base64.RawURLEncoding.EncodeToString(b[:]) // 16B → 22 字符
	ts.mu.Lock()
	defer ts.mu.Unlock()
	// 惰性清理：签发顺手机会性清扫过期项（Pitfall 4 无界增长防线，无常驻 goroutine）。
	for k, e := range ts.m {
		if now.After(e.exp) {
			delete(ts.m, k)
		}
	}
	ts.m[t] = ticketEntry{mode: mode, exp: now.Add(ts.ttl)}
	return t
}

// redeem 原子查删：查即删保证单次使用；过期按不存在处理——重放/过期/非法
// 同归 false（D-10 统一口径，无 oracle）。
func (ts *ticketStore) redeem(t string, now time.Time) (mode string, ok bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	e, ok := ts.m[t]
	if !ok {
		return "", false
	}
	delete(ts.m, t) // 无论成败先删——单次使用语义
	if now.After(e.exp) {
		return "", false
	}
	return e.mode, true
}
