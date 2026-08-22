package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
)

// Credential 是一组启动时预哈希的凭据（SHA-256 摘要对，32B 定长）。
//
// 不变量：字段不导出，只能经 ParseCredential 构造——保证比较路径上的操作数
// 永远是预哈希摘要而非明文。SHA-256 等长化消除长度侧信道：
// subtle.ConstantTimeCompare 官方明示"If the lengths of x and y do not match
// it returns 0 immediately"，直接比较明文会泄露凭据长度（Pitfall 1）。
//
// 导出原因：03-04 main.go 经 Options.Credentials 传入（跨包消费）。
type Credential struct {
	userHash [sha256.Size]byte
	passHash [sha256.Size]byte
}

// ParseCredential 解析 "user:pass" 形态凭据：strings.Cut 切首个 ':'
// （密码可含 ':'；user 不可含——RFC 7617 user-id 约束）。无冒号或空 user
// 报错；空 pass 合法（"user:" → passHash 为空串摘要，文档化决策不额外禁止）。
// 供 03-04 --credential flag 的 fs.Func 回调在 parse 期校验（导出）。
func ParseCredential(s string) (Credential, error) {
	u, p, ok := strings.Cut(s, ":")
	if !ok || u == "" {
		return Credential{}, fmt.Errorf("credential must be user:pass")
	}
	return Credential{
		userHash: sha256.Sum256([]byte(u)),
		passHash: sha256.Sum256([]byte(p)),
	}, nil
}

// matchCredential 逐组轮询、位或累积——不短路不 break，循环恒跑满全部组：
// 耗时与组数恒定正交，无"第几组匹配"的组序号时序泄露（RESEARCH Pattern 2）。
// ConstantTimeCompare 的操作数均为 sha256.Sum256 的 32B 定长摘要，禁止传入
// 原始字符串（Pitfall 1 长度泄露）。
//
// planner erratum 修正：RESEARCH Pattern 2 行 288-297 定稿代码为
// `matched &= ...` 且 matched 初值 0——`0 & x` 恒 0，结果永为 false（正确
// 凭据永远拒绝）。本实现为修正形态 `matched |= user比较 & pass比较`，保持
// "耗时与组数正交"设计意图；TestCredentialMatch 多组各自命中锁死该回归。
//
// 无凭据模式（creds 空）调用方不进此函数；防御性语义下空列表亦返回 false。
func matchCredential(creds []Credential, user, pass string) bool {
	uh := sha256.Sum256([]byte(user))
	ph := sha256.Sum256([]byte(pass))
	matched := 0
	for _, c := range creds {
		matched |= subtle.ConstantTimeCompare(uh[:], c.userHash[:]) &
			subtle.ConstantTimeCompare(ph[:], c.passHash[:])
	}
	return matched == 1
}

// authRequiredBody 是全部 401 响应的统一 body（2026-08-22 用户裁决：错 token
// 触发登录弹窗时必须有提示告知 token 失效、需登录访问）。浏览器原生 Basic 弹窗
// 本身不可定制文案（Chrome 连 realm 都不展示），唯一可达通道是 401 body——
// 用户取消/失败后浏览器渲染该纯文本。全部挑战点位同一串：无/错凭据、错 token
// 委托链完全同文，无枚举 oracle（OWASP 纪律延伸到文案层）。
const authRequiredBody = `authentication required

If you opened a share link and were prompted to log in, the link is invalid or has expired (share links are regenerated each time wesh restarts). Enter the operator credentials to continue, or ask the operator for a new link.`

// basicAuth 整站 Basic 认证中间件（D-02：/ 与 /api/attach 挂载；/ws 不挂——
// ticket 即其认证）。守卫顺序（敏感，与 /api/attach 守卫链口径一致）：
//
//	① 429 节流闸：retryAfter 命中 → 429 + Retry-After（ceil(剩余等待) 秒，
//	  窗口内恒 ≥1）+ 通用文案；retryAfter 只读不延长窗口（429 短路不
//	  recordFail——节流命中不再追加惩罚）；
//	② Basic 解析与常数时间比较：r.BasicAuth()（stdlib 解析器，禁止手拆
//	  Authorization base64——RESEARCH Don't Hand-Roll 表）+ matchCredential；
//	  无/错凭据完全同文 401（WWW-Authenticate: Basic realm="wesh",
//	  charset="UTF-8"，RFC 7617 + 通用 body——无枚举 oracle，OWASP 纪律）+
//	  recordFail（D-08 统一计数器：与 Hello ticket 核销失败同一 per-IP store）；
//	③ 认证成功 → recordSuccess 清零（D-08）→ next。
//
// 红线（SEC-01）：凭据/Authorization 头任何形态（含 base64）永不入日志参数——
// logEvent 三要素只有 remote/code/reason；HTTP 层事件 code 复用 HTTP 状态码值
// （websocket.StatusCode 底层 int，三要素结构不变，PATTERNS Shared Patterns 裁决）。
// Hello 侧节流闸仍用 allow（无 Retry-After 需求），两处闸共享同一 store（D-08）。
// 401/403/429 body 恒为通用文案——不回显用户名、Origin 值或任何请求细节。
func basicAuth(next http.Handler, creds []Credential, th *throttleStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if wait, throttled := th.retryAfter(ip, time.Now()); throttled {
			// ceil 秒整数向上取整；窗口内 wait>0 故商恒 ≥1。
			retry := int64((wait + time.Second - 1) / time.Second)
			w.Header().Set("Retry-After", strconv.FormatInt(retry, 10))
			logEvent(r.RemoteAddr, websocket.StatusCode(http.StatusTooManyRequests), "throttled")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || !matchCredential(creds, u, p) {
			th.recordFail(ip, time.Now())
			w.Header().Set("WWW-Authenticate", `Basic realm="wesh", charset="UTF-8"`) // RFC 7617
			logEvent(r.RemoteAddr, websocket.StatusCode(http.StatusUnauthorized), proto.ErrAuthFailed)
			http.Error(w, authRequiredBody, http.StatusUnauthorized)
			return
		}
		th.recordSuccess(ip) // D-08：认证成功清零
		next.ServeHTTP(w, r)
	})
}
