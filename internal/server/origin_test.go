package server

import (
	"net/http/httptest"
	"testing"
)

// TestNormalizeOrigin 锁定 --origin 值规范化语义（SEC-04/D-12）：小写 host +
// 剥离默认端口（http:80/https:443——浏览器 Origin 序列化省略默认端口，
// RFC 6454，Pitfall 3 白名单永不命中防线）；拒绝 path/query/fragment/
// userinfo/glob 字符（path.Match 退化为精确比较）/非 http(s) scheme/空 host。
func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "https 默认端口剥离+大写 host 小写化", in: "https://Wesh.Example.com:443", want: "https://wesh.example.com"},
		{name: "http 默认端口剥离", in: "http://example.com:80", want: "http://example.com"},
		{name: "非默认端口保留", in: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "裸斜杠 path 放行", in: "https://example.com/", want: "https://example.com"},
		{name: "无 scheme 拒绝", in: "example.com", wantErr: true},
		{name: "非 http(s) scheme 拒绝", in: "ftp://example.com", wantErr: true},
		{name: "空 host 拒绝", in: "https://", wantErr: true},
		{name: "含路径拒绝", in: "https://example.com/path", wantErr: true},
		{name: "含 query 拒绝", in: "https://example.com?q=1", wantErr: true},
		{name: "含 fragment 拒绝", in: "https://example.com#frag", wantErr: true},
		{name: "含 userinfo 拒绝", in: "https://user@example.com", wantErr: true},
		{name: "含 glob 星号拒绝", in: "https://*.example.com", wantErr: true},
		{name: "含 glob 方括号拒绝", in: "https://exa[mp]le.com", wantErr: true},
		// WR-02 裁决钉死：合法 IPv6 字面量 Origin 因 [ 命中 glob 拒绝——显式
		// 不支持白名单配置（同源 IPv6 访问走 originAllowed ②，不受影响）。
		{name: "IPv6 字面量 Origin 拒绝（WR-02 裁决）", in: "https://[::1]:8443", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeOrigin(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeOrigin(%q) = %q 未报错，want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeOrigin(%q) err = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeOrigin(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestOriginAllowed 锁定四段语义（D-13，与 coder/websocket accept.go:228-264
// 逐项对齐——RESEARCH Pattern 4 一手核实）：空 Origin 放行（非浏览器客户端
// 零摩擦）→ 同源（EqualFold Host）放行 → 规范化集合精确查找 → 否则拒绝。
// Origin: null 拒绝专断言（沙箱 iframe 的 null 是 CSWSH 常见载体）。
func TestOriginAllowed(t *testing.T) {
	allowed := map[string]struct{}{
		"https://wesh.example.com": {},
	}

	tests := []struct {
		name    string
		origin  string // 空串 = 不设 Origin 头
		allowed map[string]struct{}
		want    bool
	}{
		{name: "无 Origin 放行", origin: "", allowed: allowed, want: true},
		{name: "同源放行（大小写不敏感）", origin: "http://WESH.LOCAL:7681", allowed: allowed, want: true},
		{name: "白名单集合内放行", origin: "https://wesh.example.com", allowed: allowed, want: true},
		{name: "Origin 带默认端口规范化命中", origin: "https://wesh.example.com:443", allowed: allowed, want: true},
		{name: "跨源不在集合拒绝", origin: "https://evil.example.com", allowed: allowed, want: false},
		{name: "null Origin 拒绝", origin: "null", allowed: allowed, want: false},
		{name: "畸形 Origin 拒绝", origin: "http://[::1", allowed: allowed, want: false},
		{name: "空集合同源仍放行", origin: "http://wesh.local:7681", allowed: map[string]struct{}{}, want: true},
		{name: "空集合跨源拒绝", origin: "https://wesh.example.com", allowed: map[string]struct{}{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://wesh.local:7681/ws", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if got := originAllowed(r, tt.allowed); got != tt.want {
				t.Errorf("originAllowed(Origin=%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}
