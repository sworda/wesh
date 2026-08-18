package server

import "net/http"

// securityHeaders 统一安全头中间件（D-06，OWASP HTTP Headers Cheat Sheet 基线
// 按 RESEARCH Pattern 6 定稿）。本仓首个 func(http.Handler) http.Handler
// 中间件先例，保持最小形态，不引入框架。
//
// 恒在六项：
//   - Content-Security-Policy：script/style 'unsafe-inline' 是单文件全内联
//     现实的已裁决接受项（D-06）；connect-src 'self' 在 CSP3 覆盖同源
//     ws/wss（前端 WS 地址由 location.host 构造恒同源）——ASSUMED 项：
//     老 Safari（<15.x）曾有不认 'self' 含 wss 的 bug，若 UAT 命中回退
//     `connect-src 'self' ws: wss:`；
//   - X-Frame-Options: DENY（frame-ancestors 'none' 的老浏览器兼容，双发）；
//   - X-Content-Type-Options: nosniff；
//   - Referrer-Policy: strict-origin-when-cross-origin（OWASP 基线原值）；
//   - Cross-Origin-Opener-Policy: same-origin；
//   - Cross-Origin-Resource-Policy: same-origin（本站资源全部同源内联，
//     零功能代价）。
//
// tlsOn 分支（Pitfall 7）：Strict-Transport-Security 仅 TLS 时发送——规范要求
// 仅 TLS，明文分支发送会与反代终止场景的反代策略打架。max-age=63072000
// （两年，OWASP 基线，Open Question 3 裁决落地）；去 preload——自托管工具
// 不进浏览器预载表，误配代价大。
//
// Go 默认不发 Server/X-Powered-By 头，无需移除动作；废弃头
// （X-XSS-Protection/Expect-CT/Public-Key-Pins）不发送（OWASP 业界共识，
// 以 CSP 替代）。
func securityHeaders(next http.Handler, tlsOn bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if tlsOn {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
