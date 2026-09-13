package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
)

// 会话 cookie 认证（2026-09-13，herdr-web 登录摩擦修复）。
//
// 背景：整站 Basic 认证依赖浏览器 HTTP auth 缓存的持久性，iOS Safari / 隐身 /
// ITP / 清缓存等场景下缓存丢失，fetch 401 不弹原生框 → 只能刷新重登；且
// 无凭据探测此前会计入 per-IP 节流（已在 auth.go 探测豁免修复）。本文件以
// 表单登录 + HttpOnly Cookie 会话取代浏览器 Basic 门，并把会话持久化到磁盘，
// 使 wesh 崩溃重启（systemd Restart=on-failure）后 cookie 仍有效——用户无感。
//
// 与 ticket 的关系：ticket 机制零改动（60s、单次、内存态）——cookie 只替代
// /api/attach 的 HTTP 层凭据，WS 仍由一次性 ticket 引导（纵深不变）。
//
// 多实例：cookie 作用域按 host 不含端口，tui/pane/ro 多端口形态共享同一
// session 文件即可「登录一次全端口通用」；内存 miss 且文件 mtime 变化时 reload
// 重判，写侧读-合并-写 tmp+rename 原子替换（并发写最坏丢一次会话，可接受）。

// defaultSessionTTL 会话默认存活期（30 天）。滑动续期：剩余 <50% 时刷新 exp。
// iOS ITP 的 7 天 cap 只约束 JS document.cookie，服务端 Set-Cookie 的 HttpOnly
// cookie 不受限；个人运维工具「登录一次管一个月」是目标体验。
const defaultSessionTTL = 720 * time.Hour

// sessionCookieName 会话 cookie 名（host-only，无 Domain）。
const sessionCookieName = "wesh_session"

// sessionEntry 单条会话登记项（unix 秒，JSON 持久化形态）。
type sessionEntry struct {
	Iat int64 `json:"iat"`
	Exp int64 `json:"exp"`
}

// sessionFile 是磁盘持久化文件形态（v=1）。cred 为凭据摘要——改密码后摘要变化，
// 整表作废（旧 cookie 全吊销），这是「改密码即全会话失效」的自然兑现。
type sessionFile struct {
	V        int                     `json:"v"`
	Cred     string                  `json:"cred"`
	Sessions map[string]sessionEntry `json:"sessions"`
}

// sessionStore 内存会话表 + 可选磁盘持久化。不变量：
//   - token 为 crypto/rand 16B → base64url 22 字符（与 ticketStore.issue 同形态，
//     128bit 空间使在线枚举无意义）；
//   - 过期按不存在处理，签发/校验顺手清扫（无常驻 janitor goroutine）；
//   - filePath 为空 = 纯内存模式（零持久化，行为与无文件形态一致）；
//   - 续期落盘节流（>1h 才写），签发/吊销即时落盘。
type sessionStore struct {
	mu          sync.Mutex
	m           map[string]sessionEntry
	ttl         time.Duration
	filePath    string
	cred        string
	modTime     time.Time // 已加载文件的 mtime（reload-on-miss 判据之一）
	size        int64     // 已加载文件大小（mtime 粒度粗时兜底判据）
	lastPersist time.Time // 上次落盘时刻（续期写节流）
}

// credentialDigest 由预哈希凭据派生一个稳定的「凭据版本」摘要（十六进制）。
// 输入是 Credential 的 userHash/passHash（本身即 SHA-256 定长摘要），故无需
// 明文密码即可判定「凭据是否变化」——main 在凭据归一化后调用一次并透传。
// 摘要不是 secret（不可逆于明文，仅作版本标记），不参与认证判定。
func credentialDigest(creds []Credential) string {
	h := sha256.New()
	for _, c := range creds {
		h.Write(c.userHash[:])
		h.Write(c.passHash[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// newSessionStore 构造会话表；ttl<=0 兜底 defaultSessionTTL；filePath 非空时
// 加载既有会话（文件不存在/损坏/凭据不符均按空表处理，只记警告不阻断启动）。
func newSessionStore(ttl time.Duration, filePath, cred string) *sessionStore {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	ss := &sessionStore{m: make(map[string]sessionEntry), ttl: ttl, filePath: filePath, cred: cred}
	if filePath != "" {
		ss.loadLocked(false)
	}
	return ss
}

// loadLocked 读取磁盘会话文件。replace=true（初次构造）时整表替换；false
// （reload-on-miss）时合并——磁盘有而内存无的条目并入，冲突以内存为准。
// 文件缺失/损坏/版本或凭据不符 → 空表（仅警告，不阻断）。
func (ss *sessionStore) loadLocked(replace bool) {
	data, err := os.ReadFile(ss.filePath)
	if err != nil {
		return // 不存在/不可读：空表起步（非错误路径）
	}
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil || sf.V != 1 || sf.Cred != ss.cred {
		// 损坏或凭据已轮换：旧会话不得复活（改密码 → 全会话吊销）。
		if !replace {
			return // reload 路径不因坏文件清空在内存态
		}
		slog.Warn("wesh: session file unusable; starting with empty sessions", "path", ss.filePath)
		return
	}
	now := time.Now()
	for tok, e := range sf.Sessions {
		if now.UnixMilli() >= e.Exp {
			continue // 过期不加载
		}
		if _, exists := ss.m[tok]; exists && !replace {
			continue // 冲突以内存为准（本进程可能刚续期）
		}
		ss.m[tok] = e
	}
	if info, err := os.Stat(ss.filePath); err == nil {
		ss.modTime = info.ModTime()
		ss.size = info.Size()
	}
}

// reloadIfStaleLocked 在内存 miss 时按 mtime/size 变化 reload（多实例共享文件：
// 另一进程签发的会话对本进程可见）。size 兜底 mtime 粒度粗的文件系统；文件未变
// 则空转。
func (ss *sessionStore) reloadIfStaleLocked() {
	if ss.filePath == "" {
		return
	}
	info, err := os.Stat(ss.filePath)
	if err != nil {
		return
	}
	if !info.ModTime().After(ss.modTime) && info.Size() == ss.size {
		return
	}
	ss.loadLocked(false)
}

// issue 签发会话 token（crypto/rand 16B → base64url 22），登记并即时落盘。
func (ss *sessionStore) issue(now time.Time) string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand 失败即进程级问题，沿用 ticketStore 惯例
	tok := base64.RawURLEncoding.EncodeToString(b[:])
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.reloadIfStaleLocked()
	ss.pruneLocked(now)
	ss.m[tok] = sessionEntry{Iat: now.Unix(), Exp: now.Add(ss.ttl).UnixMilli()}
	ss.persistLocked(now, true)
	return tok
}

// validate 校验 token；命中且剩余 <ttl/2 时滑动续期（renewed=true，调用方重发
// Set-Cookie）。内存 miss 时按文件 mtime 变化 reload 重判（多实例共享）。
func (ss *sessionStore) validate(token string, now time.Time) (ok, renewed bool) {
	if token == "" {
		return false, false
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	e, found := ss.m[token]
	if !found {
		ss.reloadIfStaleLocked()
		e, found = ss.m[token]
	}
	if !found {
		return false, false
	}
	if now.UnixMilli() >= e.Exp {
		delete(ss.m, token)
		return false, false
	}
	if time.UnixMilli(e.Exp).Sub(now) < ss.ttl/2 {
		e.Exp = now.Add(ss.ttl).UnixMilli()
		ss.m[token] = e
		renewed = true
		ss.persistLocked(now, false) // 续期落盘节流（>1h）
	}
	return true, renewed
}

// revoke 吊销单个会话（logout）并即时落盘。
func (ss *sessionStore) revoke(token string, now time.Time) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.m, token)
	ss.persistLocked(now, true)
}

// pruneLocked 清扫过期条目（签发时顺手调用，防 map 无界增长）。
func (ss *sessionStore) pruneLocked(now time.Time) {
	for tok, e := range ss.m {
		if now.UnixMilli() >= e.Exp {
			delete(ss.m, tok)
		}
	}
}

// persistLocked 读-合并-写 tmp+rename。force=false 时节流（距上次 >1h 才写，
// 避免滑动续期把磁盘写放大）。合并语义：磁盘上其他进程签发的会话不丢，本进程
// 内存条目为准；写入前清扫过期项。I/O 失败只警告（会话仍在内存有效）。
func (ss *sessionStore) persistLocked(now time.Time, force bool) {
	if ss.filePath == "" {
		return
	}
	if !force && now.Sub(ss.lastPersist) < time.Hour {
		return
	}
	merged := make(map[string]sessionEntry, len(ss.m))
	if data, err := os.ReadFile(ss.filePath); err == nil {
		var sf sessionFile
		if json.Unmarshal(data, &sf) == nil && sf.V == 1 && sf.Cred == ss.cred {
			for tok, e := range sf.Sessions {
				if now.UnixMilli() < e.Exp {
					merged[tok] = e
				}
			}
		}
	}
	for tok, e := range ss.m {
		if now.UnixMilli() < e.Exp {
			merged[tok] = e
		}
	}
	sf := sessionFile{V: 1, Cred: ss.cred, Sessions: merged}
	data, err := json.Marshal(sf)
	if err != nil {
		slog.Warn("wesh: session file marshal failed", "err", err)
		return
	}
	tmp := ss.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		slog.Warn("wesh: session file write failed", "path", tmp, "err", err)
		return
	}
	if err := os.Rename(tmp, ss.filePath); err != nil {
		slog.Warn("wesh: session file rename failed", "path", ss.filePath, "err", err)
		_ = os.Remove(tmp)
		return
	}
	ss.lastPersist = now
	if info, err := os.Stat(ss.filePath); err == nil {
		ss.modTime = info.ModTime()
		ss.size = info.Size()
	}
}

// sessionCookie 构造会话 cookie。Path=/（多端口共享）、HttpOnly、SameSite=Lax
//（允许书签/二维码顶层导航携带；跨站 POST 不带 cookie 天然防 CSRF）、TLS 时
// Secure（明文 loopback 调试不设，免 Safari 拒存）。maxAge>0 = 持久 cookie
//（秒）；maxAge<0 = 立即删除（登出）；maxAge==0 = 不下发 Max-Age 属性。
func sessionCookie(token string, tlsOn bool, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   tlsOn,
		MaxAge:   maxAge,
	}
}

// loginRequest 登录表单提交体（前端自绘表单 JSON）。
type loginRequest struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

// loginHandler 处理 POST /api/login（表单登录，2026-09-13）。守卫顺序与全站
// 一致：① 429 优先闸（与 auth.go 同 store 同语义，TestThrottleHTTP 不受影响）；
// ② 凭据校验——仅错误口令 recordFail（正确口令即使窗口内也按 429 拒绝，与
// 全站口径一致：窗口只由真实错误口令形成）；③ 成功 recordSuccess + 签发会话
// cookie（TTL 秒）+ 200。响应体固定 schema，无凭据回显（SEC-01）。
// 红线：user/pass 明文永不入日志；错误响应与 authRequiredBody 同文（无 oracle）。
func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	ip := s.proxy.clientIP(r)
	now := time.Now()
	if wait, throttled := s.throttle.retryAfter(ip, now); throttled {
		retry := int64((wait + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.FormatInt(retry, 10))
		s.mc.authThrottled.Add(1)
		emitEvent(
			slog.String("event", "throttled"),
			slog.String("remote", s.proxy.remote(r)),
			slog.Int("code", http.StatusTooManyRequests),
			slog.Int64("retry_after", retry),
		)
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 畸形 body：通用 401（不回显、不计失败——非凭据尝试，探测豁免同口径）。
		http.Error(w, authRequiredBody, http.StatusUnauthorized)
		return
	}
	if !matchCredential(s.credentials, req.User, req.Pass) {
		s.throttle.recordFail(ip, now)
		s.mc.authFailed.Add(1)
		logEvent(s.proxy.remote(r), websocket.StatusCode(http.StatusUnauthorized), proto.ErrAuthFailed, s.proxy.remoteUser(r))
		http.Error(w, authRequiredBody, http.StatusUnauthorized)
		return
	}
	s.throttle.recordSuccess(ip)
	tok := s.sessions.issue(now)
	http.SetCookie(w, sessionCookie(tok, s.tlsOn, int(s.sessionTTL.Seconds())))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// logoutHandler 处理 POST /api/logout：吊销 cookie 会话并清 cookie（Max-Age<0）。
// 无 cookie 也返回 200（幂等，不泄露会话是否存在）。
func (s *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	if c, err := r.Cookie(sessionCookieName); err == nil {
		s.sessions.revoke(c.Value, now)
	}
	http.SetCookie(w, sessionCookie("", s.tlsOn, -1))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// sessionAuth 是 /api/attach 的 HTTP 层认证中间件（2026-09-13，取代整站
// basicAuth 挂在 attach 链上）。三态：
//  1. 会话 cookie 有效 → 放行（滑动续期时重发 Set-Cookie）；
//  2. 无 cookie 但显式携带 Authorization → 委托 basicAuth 原链（curl/脚本
//     兼容 + 保留 RFC 7617 挑战头与错误凭据计数语义，TestThrottleHTTP 不动）；
//  3. 两者皆无 → 探测：401 通用文案，**不带 WWW-Authenticate**（避免浏览器
//     原生弹窗）、**不计节流**（探测永远无法通过认证，计数只会污染 per-IP 桶）。
//
// 无认证模式（len(credentials)==0）不装配本中间件（Handler 分支保证）。
func (s *Server) sessionAuth(next http.Handler) http.Handler {
	basic := basicAuth(next, s.credentials, s.throttle, s.proxy, &s.mc)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			if ok, renewed := s.sessions.validate(c.Value, time.Now()); ok {
				if renewed {
					http.SetCookie(w, sessionCookie(c.Value, s.tlsOn, int(s.sessionTTL.Seconds())))
				}
				next.ServeHTTP(w, r)
				return
			}
		}
		if _, _, ok := r.BasicAuth(); ok {
			basic.ServeHTTP(w, r) // 显式凭据：委托原链（含 429 闸/挑战头/计数）
			return
		}
		// 无 cookie 无凭据：探测——静默 401（不计数不打日志，SPA 据此显示登录视图）。
		http.Error(w, authRequiredBody, http.StatusUnauthorized)
	})
}
