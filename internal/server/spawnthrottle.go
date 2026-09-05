package server

// spawnthrottle.go —— 13-02 spawn 双令牌桶（PC-08 churn 防线本体，D-03/D-04/
// D-05；研究 Pattern 1 + Don't Hand-Roll 表）：
//
//   - 全局桶（8/s burst 16）防断网惊群：N 浏览器同时自动重连 = N 个
//     fork+exec，全局速率上限使惊群瞬时放行 ≤ burst、稳态压回 8 spawn/s
//     （T-13-05）；前端 30s 封顶退避既有（06-03）且节流拒绝码 1011 不在
//     shouldReconnect 触发集（D-04）——拒绝不触发重连放大；
//   - per-IP 桶（1/s burst 4）防单点 churn：合法票据 connect→spawn→
//     disconnect 死循环被压回 1 spawn/s 稳态，fork 预算防线（PITFALLS P4
//     fork bomb，T-13-04）。
//
// D-03 内部常量纪律（12-03 defaultSlowDwell 先例同构）：四值零 CLI flag/
// TOML 键——零调优需求，公开契约面不膨胀；Options 四字段仅测试覆写通道
//（OutboxBytes/SlowDwell 注释分档同档）。取值依据（PITFALLS P4 推荐量级）：
// 个人工具单/少用户场景，8/s 全局 + 1/s per-IP 使正常使用（人手速 attach、
// 偶发重连）永不触桶，churn/惊群形态全部拦截。
//
// 键来源纪律（D-05）：调用方传 clientIP 产物（server.go Attach 既有提取点
// ——trust 开启时 XFF 链首换键），本件不碰 *http.Request（throttleStore 同
// 纪律——键提取在 proxy.clientIP 单点，再写一份即键分叉：同 IP 两配额）。
//
// 内存界（throttle.go throttleStore 先例平移）：mu + map + 惰性过期，无常驻
// janitor goroutine（零新 exitf 分支纪律）；条目 ≈56B × 4096 IP ≈ 230KB
// 可接受。桶计数为 rate.Limiter 库级既定实现（perclient.go 每客户端输入
// 限速同件在用），判定输出为布尔无舍入歧义。

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultSpawnGlobalRate 全局桶速率（spawn/s）——断网惊群上限。
	defaultSpawnGlobalRate = 8
	// defaultSpawnGlobalBurst 全局桶突发容量。
	defaultSpawnGlobalBurst = 16
	// defaultSpawnPerIPRate per-IP 桶速率（spawn/s/IP）——单点 churn 上限。
	defaultSpawnPerIPRate = 1
	// defaultSpawnPerIPBurst per-IP 桶突发容量。
	defaultSpawnPerIPBurst = 4
)

// spawnPerIPEntry 为单 IP 的令牌桶条目（throttleEntry 同位形态）。
type spawnPerIPEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time // 惰性过期依据（>15min 未活动重置——throttle.go 先例）
}

// spawnThrottleStore 为 spawn 双令牌桶存储：全局单例桶 + per-IP map 桶。
// 不变量：
//   - 取桶与判定在 allow 内持 mu 完成单锁最小临界区（纯内存无阻塞——
//     Anti-Pattern 1 的「闸内判定」半面；spawnFunc 调用点保持 hubMu 之外）；
//   - lastSeen 超 15min 的条目在 allow 内惰性重置（map 上界纪律，
//     throttle.go recordFail 先例；15min > per-IP 恢复窗一个数量级以上，
//     不误重置活跃桶）——重置即满额新桶（rate.NewLimiter 出生满 burst）；
//   - 判序 per-IP 先、全局后（短路语义）：单 IP churn 的过量尝试在 per-IP
//     桶即拒（AllowN 失败不消耗令牌），不耗全局预算——反代后合法多用户的
//     共享全局配额不被单一 churning IP 占干（D-05 多用户不误伤的判序面）；
//     反向代价仅「全局耗尽（惊群）时 per-IP 令牌被拒绝尝试消耗」——≤
//     burst/IP 且只影响请求方自身速率面，可接受。
type spawnThrottleStore struct {
	mu         sync.Mutex
	global     *rate.Limiter
	perIP      map[string]spawnPerIPEntry
	perIPRate  int
	perIPBurst int
}

// newSpawnThrottleStore 构造双桶；四参数零值逐一兜底 default 常量
//（newThrottleStore base/cap 零值兜底先例——测试经参数覆写提速；New 侧
// Options 兜底后的本兜底为直构形态的防御面）。
func newSpawnThrottleStore(globalRate, globalBurst, perIPRate, perIPBurst int) *spawnThrottleStore {
	if globalRate <= 0 {
		globalRate = defaultSpawnGlobalRate
	}
	if globalBurst <= 0 {
		globalBurst = defaultSpawnGlobalBurst
	}
	if perIPRate <= 0 {
		perIPRate = defaultSpawnPerIPRate
	}
	if perIPBurst <= 0 {
		perIPBurst = defaultSpawnPerIPBurst
	}
	return &spawnThrottleStore{
		global:     rate.NewLimiter(rate.Limit(globalRate), globalBurst),
		perIP:      make(map[string]spawnPerIPEntry),
		perIPRate:  perIPRate,
		perIPBurst: perIPBurst,
	}
}

// allow 判定一次 spawn 预算：per-IP 桶与全局桶串联，任一 AllowN 失败即
// false。now 参数化——测试时间注入面（throttleStore recordFail(now) 先例；
// 双桶均走 AllowN(now, 1) 注入同一时刻，消费语义 = 通过即各消耗一令牌）。
func (st *spawnThrottleStore) allow(ip string, now time.Time) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	e, ok := st.perIP[ip]
	if !ok || now.Sub(e.lastSeen) > 15*time.Minute {
		// 新键或惰性过期重置（throttle.go recordFail 先例——map 上界纪律）。
		e = spawnPerIPEntry{limiter: rate.NewLimiter(rate.Limit(st.perIPRate), st.perIPBurst)}
	}
	e.lastSeen = now
	st.perIP[ip] = e
	if !e.limiter.AllowN(now, 1) {
		return false
	}
	return st.global.AllowN(now, 1)
}
