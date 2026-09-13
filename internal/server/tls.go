package server

import (
	"crypto/tls"
	"net/http"
)

// TLSConfig 返回声明式 TLS 下限配置（D-06，SEC-05），供 03-04 main.go 的
// run() TLS 分岔装配（hs.TLSConfig = server.TLSConfig() 后 ServeTLS）。
//
// GOROOT go1.26.3 一手核实事实（RESEARCH Pattern 5）：
//   - 默认 cipher 列表（cipher_suites.go:283-312）含 4 个 ECDHE CBC-SHA1
//     套件（defaults.go:69-77 的 DeleteFunc 不删它们）——不设显式清单
//     testssl.sh 必报 "Obsoleted CBC ciphers" 弱项（Pitfall 2），故此处
//     显式六件 AEAD（ECDSA/RSA × AES-128-GCM/AES-256-GCM/ChaCha20-Poly1305）；
//   - CipherSuites 仅约束 TLS 1.0–1.2；TLS 1.3 的三套件全是 AEAD 且不可配
//     ——配置 1.3 cipher 即死代码，禁止尝试；
//   - MinVersion 1.2；MaxVersion 不设（默认 1.3，D-06 默认协商 1.3）；
//   - PreferServerCipherSuites 已废弃（Go 1.14+ 恒按服务端偏好），不使用。
//
// 附带收益：Go 1.26 默认曲线含 X25519MLKEM768 混合后量子密钥交换，零配置获得。
//
// 2026-09-13（herdr-web 登录摩擦排查）：NextProtos 显式钉死 http/1.1——Go 标准库
// 的 HTTP/2 不支持 WebSocket over HTTP/2（RFC 8441 未被 net/http h2 实现），
// ALPN 协商 h2 会使 iOS Safari 的 wss:// 建连不稳（实测 ALPN=h2 时握手失败）。
// 注意：仅设 NextProtos 不够——TLSNextProto==nil 时 Go 会自动装配 h2 并把 "h2"
// **追加**进 NextProtos（GOROOT h2_bundle.go:4277-4281，go1.26.3 一手核实）。
// 结构性闸是 DisableHTTP2（非 nil 空 TLSNextProto map），二者配对使用。
func TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
	}
}

// DisableHTTP2 关闭 net/http 的 HTTP/2 自动装配（2026-09-13）。
// Go 在 ServeTLS 时若 TLSNextProto==nil 会自动注入 h2，且会把 "h2" 追加进
// TLSConfig.NextProtos——只设 NextProtos 挡不住（见 TLSConfig 注释）。
// 置为非 nil 空 map 即让 shouldConfigureHTTP2ForServe 判定不自动装配
//（GOROOT server.go:3031 注释：TLSNextProto 非 nil 时不自动启用 HTTP/2）。
func DisableHTTP2(hs *http.Server) {
	hs.TLSNextProto = map[string]func(*http.Server, *tls.Conn, http.Handler){}
}
