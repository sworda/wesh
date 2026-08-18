package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// selfSignedCert 在测试内生成 ecdsa.P256 自签 localhost 证书（stdlib 模式，
// ~40 行）——禁止落盘 *.pem fixture、禁止依赖 GODEBUG（Pitfall 9：GODEBUG
// 进程启动期读取，t.Setenv 无效）。
func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// TestTLSVersionAndCipherFloor 锁定 D-07 自动化断言矩阵（SEC-05）：
// 客户端 MaxVersion 1.1 握手必败、1.2 必成、零值客户端默认协商出 1.3、
// CBC-only 客户端必败。握手失败断言 = tls.Dial 返回错误，不区分客户端本地
// 拒绝还是服务端拒绝（Pitfall 9 纪律）。全部客户端 InsecureSkipVerify:
// true——测试内连接自签证书的正当用途（断言对象是版本/cipher 而非证书链）。
func TestTLSVersionAndCipherFloor(t *testing.T) {
	cfg := TLSConfig()
	cfg.Certificates = []tls.Certificate{selfSignedCert(t)}

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts.TLS = cfg
	// 拒绝用例必然产生服务端 handshake error 日志，静默掉保持测试输出干净。
	ts.Config.ErrorLog = log.New(io.Discard, "", 0)
	ts.StartTLS()
	defer ts.Close()
	addr := ts.Listener.Addr().String()

	tests := []struct {
		name      string
		clientCfg *tls.Config
		wantErr   bool
		wantVer   uint16 // 0 = 不断言版本
	}{
		{name: "tls11-rejected", clientCfg: &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11}, wantErr: true},
		{name: "tls12-ok", clientCfg: &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}, wantVer: tls.VersionTLS12},
		{name: "tls13-default", clientCfg: &tls.Config{}, wantVer: tls.VersionTLS13},
		{name: "cbc-only-rejected", clientCfg: &tls.Config{MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := tt.clientCfg.Clone()
			cc.InsecureSkipVerify = true // 自签证书，测试内正当用途
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", addr, cc)
			if tt.wantErr {
				if err == nil {
					_ = conn.Close()
					t.Fatalf("握手成功，want 失败（版本/cipher 下限被突破）")
				}
				return
			}
			if err != nil {
				t.Fatalf("握手失败 err = %v, want 成功", err)
			}
			defer conn.Close()
			if tt.wantVer != 0 {
				if v := conn.ConnectionState().Version; v != tt.wantVer {
					t.Errorf("协商版本 = %#04x, want %#04x", v, tt.wantVer)
				}
			}
		})
	}
}

// TestSecurityHeaders 双分支断言（D-06/Pitfall 7）：tlsOn=false 恒在六项精确
// 值且无 Strict-Transport-Security（明文发 HSTS 与反代策略打架）；tlsOn=true
// 追加 HSTS 精确值。CSP 逐字符精确断言——安全契约，防手滑漂移。
func TestSecurityHeaders(t *testing.T) {
	const csp = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	const hsts = "max-age=63072000; includeSubDomains"
	want := map[string]string{
		"Content-Security-Policy":      csp,
		"X-Frame-Options":              "DENY",
		"X-Content-Type-Options":       "nosniff",
		"Referrer-Policy":              "strict-origin-when-cross-origin",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	t.Run("明文分支无 HSTS", func(t *testing.T) {
		rec := httptest.NewRecorder()
		securityHeaders(next, false).ServeHTTP(rec, httptest.NewRequest("GET", "http://wesh.local/", nil))
		h := rec.Header()
		for k, v := range want {
			if got := h.Get(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}
		if _, ok := h["Strict-Transport-Security"]; ok {
			t.Errorf("明文分支出现 Strict-Transport-Security——HSTS 仅 TLS 分支（Pitfall 7）")
		}
	})

	t.Run("TLS 分支追加 HSTS", func(t *testing.T) {
		rec := httptest.NewRecorder()
		securityHeaders(next, true).ServeHTTP(rec, httptest.NewRequest("GET", "http://wesh.local/", nil))
		h := rec.Header()
		for k, v := range want {
			if got := h.Get(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}
		if got := h.Get("Strict-Transport-Security"); got != hsts {
			t.Errorf("Strict-Transport-Security = %q, want %q", got, hsts)
		}
	})
}
