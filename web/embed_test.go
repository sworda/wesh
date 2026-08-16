package web

import "testing"

// TestAcceptsGzip：Accept-Encoding 按 token 解析——裸 gzip 与 q>0 接受；
// "gzip;q=0"（显式拒绝）、"x-gzip"（前缀误匹配）、畸形 q 值一律拒绝。
func TestAcceptsGzip(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"GZIP", true}, // codings 大小写不敏感
		{"gzip, br", true},
		{"br, gzip", true},
		{"gzip;q=1.0", true},
		{"gzip; q=0.5", true},
		{"gzip;q=0", false},   // 显式拒绝 gzip
		{"gzip;q=0.0", false}, // q=0 等价拒绝
		{"x-gzip", false},     // 子串误匹配
		{"br", false},
		{"", false},
		{"gzip;q=abc", false}, // 畸形 q 保守拒绝
	}
	for _, c := range cases {
		if got := acceptsGzip(c.header); got != c.want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", c.header, got, c.want)
		}
	}
}
