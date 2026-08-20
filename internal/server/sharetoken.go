package server

// sharetoken.go —— MULTI-05 分享链接（05-06，D-01..D-04 确认门 as-locked 通过）：
// 分享 token = 独立第三认证通道（D-01）：启动时生成 ro/rw 两个 128bit 随机 token，
// 持有效 token 可 GET 页面 + POST /api/attach 换一次性 ticket（绕过 Basic）；
// 无/错 token 时 P3 Basic 矩阵不变；与凭据共存（operator 走凭据、旁观者走链接）、
// 与认证模式正交（OQ1 用户 2026-08-19 裁决：无认证模式同样生成并打印链接）。
// token 可复用至进程重启（D-02）——「一次性」落在 SEC-02 ticket 上；ro/rw token
// 每轮启动重新随机生成，重启即废全部旧链接，吊销语义 = 重启。
//
// 红线（D-03，SEC-01 延伸）：token 任何形态（明文/base64/哈希前缀）永不作
// logEvent 参数、永不入 stderr 事件流/错误响应体/metrics——唯一合法输出面是
// 启动打印两行（MULTI-05 授权的产品行为）与 URL 路径本身（/s/{token}/ 路径段
// 会进反代访问日志，D-03 已裁决接受的取舍，README 脱敏建议由 05-09 落地）。

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/sworda/wesh/internal/proto"
)

// shareTokens 分享 token store（R-04：仅两条目、生命周期=进程——不用 map 无
// janitor，map/TTL 清理全是过度设计）。两明文启动生成即预哈希（SHA-256 等长化，
// SEC-01 长度侧信道纪律，matchCredential 同款），server 只存哈希——明文由 main
// 持有供启动打印。
type shareTokens struct {
	ro, rw [sha256.Size]byte
}

// newShareTokens 以 ro/rw 两明文构造 store；任一空串 = 通道关闭返回 nil
// （本 phase main 恒生成——nil 分支是防御性兜底，非功能路径）。
func newShareTokens(ro, rw string) *shareTokens {
	if ro == "" || rw == "" {
		return nil
	}
	return &shareTokens{ro: sha256.Sum256([]byte(ro)), rw: sha256.Sum256([]byte(rw))}
}

// GenerateShareToken 生成一个 128bit 随机分享 token（tickets.go:45-49 生成形态
// 逐字复用：crypto/rand 16B → base64.RawURLEncoding 22 字符）。token 与静态凭据
// 是独立 secret——crypto/rand 直接生成，不从凭据派生（PITFALLS C6 锁定项：
// 可预测 token = 认证绕过；128bit 空间使在线枚举无意义，2^128）。
// 导出原因：cmd/wesh main.go 启动生成点（跨包消费，ParseCredential 先例）。
func GenerateShareToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])                            // crypto/rand 失败即进程级问题，沿用 Go 惯例可读性处理
	return base64.RawURLEncoding.EncodeToString(b[:]) // 16B → 22 字符
}

// lookup 校验 token 并返回绑定 mode（D-01：ro token → ModeRO、rw token → ModeRW）。
// matchCredential 同款形态（auth.go:56-65 修正形态同源）：SHA-256 等长化消除长度
// 侧信道（Pitfall 1——ConstantTimeCompare 对不等长输入立即返回 0），两组比较位或
// 累积不短路——耗时与命中哪个 token 正交（ro/rw 是公开语义，无需组序号防泄露，
// RESEARCH Code Examples 注）。nil receiver（通道关闭）恒 miss。
// 红线（D-03）：token 值永不入 logEvent 参数——本函数与全部调用方均不打印。
func (st *shareTokens) lookup(token string) (string, bool) {
	if st == nil {
		return "", false
	}
	h := sha256.Sum256([]byte(token))
	roHit := subtle.ConstantTimeCompare(h[:], st.ro[:])
	rwHit := subtle.ConstantTimeCompare(h[:], st.rw[:])
	if roHit|rwHit == 0 {
		return "", false
	}
	if rwHit == 1 {
		return proto.ModeRW, true
	}
	return proto.ModeRO, true
}

// sharePage 是 GET /s/{token}/ 的页面门禁 handler（D-01/D-03，R-05 零新响应形态）：
// 有效 token → r.URL.Path 改写为 "/" 后委托既有 embed handler（gzip 旁路与 Vary
// 头语义保留；dist 单文件全内联无相对资源问题——RESEARCH Pattern 6 本 session
// 核实）；无效/缺席 token → 原样委托 / 处理链 root（凭据模式 basicAuth → 401
// challenge 且 recordFail 自动计入 D-08 统一 per-IP 计数器，R-03 零新代码；
// 无认证模式直接给页——全站本无门，D-01「无/错 token 时 P3 Basic 矩阵不变」的
// 字面落地）。不加 Cache-Control 新头。
func (s *Server) sharePage(root http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.shares.lookup(r.PathValue("token")); ok { // GOROOT request.go:1469
			r.URL.Path = "/"
		}
		root.ServeHTTP(w, r)
	}
}

// registerShareRoutes 装配分享链接两条路由（凭据与无认证模式均注册——OQ1 token
// 通道与认证模式正交；root 为 / 已注册的处理链，sharePage 委托目标）：
//   - GET /s/{token}/ 页面门禁（r.PathValue("token") 取值）；
//   - path-only 405 fallback（Allow: GET）——方法模式内建 405 仅在没有任何其它
//     模式匹配时触发，会被 "/" 子树吞掉（P3 /api/attach 同款纪律，GOROOT
//     server.go:2699-2710 n==nil 分支），故显式注册同文 fallback 补齐守卫链。
//
// GOROOT 1.22+ 通配语义三坑登记（RESEARCH Pattern 6，go1.26.3 源码核实）：
//  1. 尾斜杠 = 匿名多段通配——/s/abc/任意/深度 也命中本模式（token 段取值不受
//     影响，PathValue 恒取首段 abc）；
//  2. /s/abc（无尾斜杠）由 mux 内建 301 补斜杠——token 出现在 Location 头属
//     D-03 已接受的暴露面；
//  3. 单段通配天然限长（22 字符 base64url），路径解析零自写代码
//     （Don't Hand-Roll 表：正则/手拆路径禁止）。
func (s *Server) registerShareRoutes(mux *http.ServeMux, root http.Handler) {
	mux.Handle("GET /s/{token}/", s.sharePage(root))
	mux.HandleFunc("/s/{token}/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})
}
