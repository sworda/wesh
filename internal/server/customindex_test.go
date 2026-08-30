package server_test

// customindex_test.go —— 09-04 OPS-03 自定义首页伺服行为锁（D-05/D-06/D-07，
// 09-UI-SPEC §Custom Index Contract 逐字边界）：
//   - 双通道 byte-identity：/（空路径回落）、/index.html、/s/{token}/（有效
//     token）三路径返回同一启动读入字节（D-06 全通道统一——装饰层在 sharePage
//     委托上游，sharetoken.go 零改动的行为证明）；无效 token 无认证模式经
//     root 链给同一自定义页（现状语义保持——不改写落 FileServerFS 404 不复活）；
//   - 相对资源 404（契约语义：wesh 不伺服自定义页引用的资源——index.html
//     之外路径照旧走 FileServerFS）；
//   - gzip/Vary/Content-Type 三态：Accept-Encoding 显式含 gzip →
//     Content-Encoding: gzip + 解压后 byte-identity（§4 预压定稿）；identity
//     显式编码 → 明文字节；两态 Vary: Accept-Encoding 恒在；Content-Type
//     text/html; charset=utf-8；
//   - 安全头同源：自定义页响应恒带既有 securityHeaders 全套且与内建页响应
//     逐头同值（§5——最外层装配不区分内建/自定义页）；
//   - 0 字节 200 空 body（D-07 拒绝列表不含空文件——伺服空白页是用户明示的
//     整页替换语义）；
//   - BasePath /wesh 组合：{bp}/ 给自定义字节（§6 mux 前缀内自然成立）；
//   - 认证面不变（T-09-04d）：/api/attach 无认证 404 探测信号照旧 + WS 握手
//     Hello→Welcome 照旧（--index 不改变认证/门禁攻击面）；
//   - nil 零值兜底：CustomIndex 未配置 → GET / 200 内建页不含探针串（与
//     现状逐字节一致——TestBasePathEmptyUnchanged 同族零漂移纪律）。
//
// 红线（sharetoken_test 同款纪律延伸）：share token 与探针串只作断言材料，
// 永不进 t.Log/错误信息——失败消息只含状态码/布尔/长度/头值形状；经 label
// 标识请求（不打印含 token 的 URL）。
//
// Go transport gzip 语义适配：显式设置 Accept-Encoding 的请求不经 transport
// 自动 gzip 协商与透传解压——明文/gzip 两伺服态以 identity/gzip 两显式编码
// 直证（裸 http.Get 会被 transport 自动加 Accept-Encoding: gzip 且透明解压，
// 明文态结构性不可观测）。

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// customIndexPage 为自定义首页探针 HTML（唯一探针串——byte-identity 断言材料，
// body 长度/含探针布尔之外的形态永不入测试输出）。
const customIndexPage = "<!doctype html><html><body>CUSTOM-INDEX-PROBE-7f4a</body></html>"

// fetchPage GET 指定 URL（acceptEnc 非空时显式设 Accept-Encoding）并读尽 body，
// 返回状态码/响应头/body。label 供失败消息标识请求（不含 share token——红线）。
func fetchPage(t *testing.T, url, acceptEnc, label string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request (%s): %v", label, err)
	}
	if acceptEnc != "" {
		req.Header.Set("Accept-Encoding", acceptEnc)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET (%s): %v", label, err)
	}
	defer resp.Body.Close()
	b, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		t.Fatalf("GET (%s) read body: %v", label, rerr)
	}
	return resp.StatusCode, resp.Header, b
}

// assertCustomPage 断言 200 + 明文 body 与自定义页逐字节一致 + text/html
// Content-Type + Vary: Accept-Encoding 恒在（identity 编码消 transport 自动
// gzip 透传——三断言共用出口）。
func assertCustomPage(t *testing.T, label string, code int, h http.Header, body []byte) {
	t.Helper()
	if code != http.StatusOK {
		t.Fatalf("%s: status = %d, want 200（自定义页给页通道）", label, code)
	}
	if string(body) != customIndexPage {
		t.Errorf("%s: body = %d bytes (probe present: %v), want 自定义页字节 byte-identity", label, len(body), bytes.Contains(body, []byte("CUSTOM-INDEX-PROBE-7f4a")))
	}
	if ct := h.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("%s: Content-Type = %q, want %q", label, ct, "text/html; charset=utf-8")
	}
	if v := h.Get("Vary"); v != "Accept-Encoding" {
		t.Errorf("%s: Vary = %q, want %q（恒发——中间缓存键完整性）", label, v, "Accept-Encoding")
	}
}

// TestCustomIndex（09-04 主干，D-05/D-06/D-07 行为锁）：八子测覆盖 §Custom
// Index Contract 全行为面（头注释逐条）。
func TestCustomIndex(t *testing.T) {
	// D-06 双通道 + 显式路径：/、/index.html、/s/{有效 token}/ 三路径同一
	// 字节；无效 token 无认证模式经 root 链（wh 装饰态）给同一字节——sharePage
	// 委托上游装饰的行为证明。
	t.Run("dual channel byte identity", func(t *testing.T) {
		roTok := server.GenerateShareToken()
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:     true,
			CustomIndex:  []byte(customIndexPage),
			ShareTokenRO: roTok,
			ShareTokenRW: server.GenerateShareToken(),
		})
		base := httpBaseOf(wsURL)
		for _, tc := range []struct{ label, path string }{
			{"root empty-path fallback", "/"},
			{"explicit index.html path", "/index.html"},
			{"share token channel (valid)", "/s/" + roTok + "/"},
			{"invalid token via no-auth root chain", "/s/not-a-real-token-zz/"},
		} {
			code, h, body := fetchPage(t, base+tc.path, "identity", tc.label)
			assertCustomPage(t, tc.label, code, h, body)
		}
	})

	// 相对资源 404：自定义页引用的相对路径资源不经 --index 暴露——index.html
	// 之外一切路径照旧 FileServerFS（dist 内嵌 FS 无自定义资源 → 404，T-09-04e）。
	t.Run("relative assets 404", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{CustomIndex: []byte(customIndexPage)})
		base := httpBaseOf(wsURL)
		for _, path := range []string{"/style.css", "/assets/app.js", "/sub/page.html"} {
			code, _, _ := fetchPage(t, base+path, "identity", "asset "+path)
			if code != http.StatusNotFound {
				t.Errorf("GET %s = %d, want 404（其余路径照旧 FileServerFS——契约语义）", path, code)
			}
		}
	})

	// gzip 预压双态（§4 定稿采纳预压）：gzip 显式编码 → Content-Encoding: gzip
	// + 解压后 byte-identity；identity 显式编码 → 明文字节零 Content-Encoding；
	// 两态 Vary: Accept-Encoding 恒在。
	t.Run("gzip precompressed and plain states", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{CustomIndex: []byte(customIndexPage)})
		base := httpBaseOf(wsURL)

		code, h, body := fetchPage(t, base+"/", "gzip", "gzip request")
		if code != http.StatusOK {
			t.Fatalf("gzip request: status = %d, want 200", code)
		}
		if ce := h.Get("Content-Encoding"); ce != "gzip" {
			t.Fatalf("gzip request: Content-Encoding = %q, want gzip", ce)
		}
		if v := h.Get("Vary"); v != "Accept-Encoding" {
			t.Errorf("gzip request: Vary = %q, want Accept-Encoding", v)
		}
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("gzip request: body 非合法 gzip 流: %v", err)
		}
		plain, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("gzip request: gunzip: %v", err)
		}
		if string(plain) != customIndexPage {
			t.Errorf("gzip request: gunzip body = %d bytes (probe present: %v), want 自定义页字节", len(plain), bytes.Contains(plain, []byte("CUSTOM-INDEX-PROBE-7f4a")))
		}
		if ct := h.Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("gzip request: Content-Type = %q, want text/html; charset=utf-8", ct)
		}

		code2, h2, body2 := fetchPage(t, base+"/", "identity", "plain request")
		assertCustomPage(t, "plain request", code2, h2, body2)
		if ce := h2.Get("Content-Encoding"); ce != "" {
			t.Errorf("plain request: Content-Encoding = %q, want 空（明文伺服）", ce)
		}
	})

	// 安全头同源（§5）：自定义页响应与内建页响应 securityHeaders 逐头同值
	//（非空断言 + 同值断言——现状同源，CSP 不区分内建/自定义页）。
	t.Run("security headers same as built-in page", func(t *testing.T) {
		_, wsURLC := startTestServerWith(t, []string{"/bin/cat"}, server.Options{CustomIndex: []byte(customIndexPage)})
		_, wsURLB := startTestServerWith(t, []string{"/bin/cat"}, server.Options{})
		_, hC, _ := fetchPage(t, httpBaseOf(wsURLC)+"/", "identity", "custom page headers")
		_, hB, _ := fetchPage(t, httpBaseOf(wsURLB)+"/", "identity", "built-in page headers")
		for _, name := range []string{
			"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options",
			"Referrer-Policy", "Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy",
		} {
			vc, vb := hC.Get(name), hB.Get(name)
			if vc == "" {
				t.Errorf("自定义页响应缺安全头 %q（securityHeaders 恒在）", name)
				continue
			}
			if vc != vb {
				t.Errorf("安全头 %q: 自定义页与内建页不同值（现状同源破坏）", name)
			}
		}
	})

	// 0 字节 CustomIndex（非 nil 空切片）→ 200 空 body + text/html Content-Type
	//（D-07 拒绝列表不含空文件）。
	t.Run("empty page 200 empty body", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{CustomIndex: []byte{}})
		code, h, body := fetchPage(t, httpBaseOf(wsURL)+"/", "identity", "empty page")
		if code != http.StatusOK {
			t.Fatalf("empty page: status = %d, want 200（0 字节合法）", code)
		}
		if len(body) != 0 {
			t.Errorf("empty page: body = %d bytes, want 0", len(body))
		}
		if ct := h.Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("empty page: Content-Type = %q, want text/html; charset=utf-8", ct)
		}
	})

	// BasePath /wesh 组合（§6）：{bp}/ 给自定义字节（StripPrefix 剥前缀后落
	// 装饰层——mux 前缀内自然成立）；bp 外 / 照旧 404（07-01 既有语义不变）。
	t.Run("base-path combo serves custom page", func(t *testing.T) {
		httpBase, _ := startBasePathServer(t, server.Options{Writable: true, CustomIndex: []byte(customIndexPage)})
		code, h, body := fetchPage(t, httpBase+"/wesh/", "identity", "bp root")
		assertCustomPage(t, "bp root", code, h, body)
		codeOut, _, _ := fetchPage(t, httpBase+"/", "identity", "outside bp")
		if codeOut != http.StatusNotFound {
			t.Errorf("outside bp: GET / = %d, want 404（bp 装配现状语义不变）", codeOut)
		}
	})

	// 认证面不变（T-09-04d）：/api/attach 无认证 404 探测信号照旧（前端直连
	// WS 既有形态）+ WS 握手 Hello→Welcome 照旧（--index 不改变认证/门禁面）。
	t.Run("auth surface unchanged", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			CustomIndex: []byte(customIndexPage),
		})
		resp := postAttach(t, attachURL(wsURL), "", "", nil)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("POST /api/attach (no-auth, no token) = %d, want 404（探测信号照旧）", resp.StatusCode)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c, mode := dialHello(t, ctx, wsURL, 80, 24)
		defer c.Close(websocket.StatusNormalClosure, "")
		if mode != proto.ModeRW {
			t.Errorf("Welcome mode = %q, want %q（WS 握手照旧，writable=true）", mode, proto.ModeRW)
		}
	})

	// nil 零值兜底：CustomIndex 未配置 → GET / 200 内建页不含探针串（与现状
	// 逐字节一致；装饰层只在非 nil 时包裹——零值兜底纪律）。
	t.Run("nil fallback serves built-in page", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
		code, h, body := fetchPage(t, httpBaseOf(wsURL)+"/", "identity", "nil fallback")
		if code != http.StatusOK {
			t.Fatalf("nil fallback: status = %d, want 200（内建页现状）", code)
		}
		if bytes.Contains(body, []byte("CUSTOM-INDEX-PROBE-7f4a")) {
			t.Error("内建页 body 含自定义页探针串——nil 零值兜底破坏")
		}
		if ct := h.Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("nil fallback: Content-Type = %q, want text/html; charset=utf-8", ct)
		}
	})
}
