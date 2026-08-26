package server_test

// basepath_test.go —— 07-01 OPS-02 base-path 反代子路径挂载（D-13/D-14）：
// Options.BasePath="/wesh" 实例的 HTTP/WS 全链断言——mux 前缀装配（StripPrefix
// 仅包静态伺服链）+ 307 尾斜杠规范化（go1.22+ mux matchOrRedirect 免费机制，
// GOROOT server.go:2687,2721-2745）+ bp 外路由 404 + 405 Allow + share 页与
// ticket 签发交叉 + WS 升级握手；BasePath 零值实例锁 / 与 /ws 两路由存活
//（未配置时注册形态与现状逐字节一致）。
//
// 红线（sharetoken_test.go 同款纪律）：token 值只存局部变量作断言材料，永不进
// t.Log/错误信息——断言输出只含状态码/布尔/形状；307 Location 断言用占位路径段
//（"abc" 非真实 token）规避 Location 头携真实 token 入测试输出（D-03 暴露面
// 纪律延伸到测试输出面）。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// startBasePathServer 装配 BasePath="/wesh" 的测试实例（e2e_test.go
// startTestServerWith 既有 helper 复用，无认证 + --writable 形态——bp 装配语义
// 与认证模式正交，凭据矩阵由 sharetoken_test 既有覆盖不重复），返回 http base
// URL 与 bp 前缀 ws URL。
func startBasePathServer(t *testing.T, opts server.Options) (httpBase, bpWSURL string) {
	t.Helper()
	opts.BasePath = "/wesh"
	_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, opts)
	host := strings.TrimSuffix(strings.TrimPrefix(wsURL, "ws://"), "/ws")
	return "http://" + host, "ws://" + host + "/wesh/ws"
}

// readBodyStr 读尽并关闭响应体，返回串（调用方不再 Close）。
func readBodyStr(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// TestBasePathRoutes 锁定 bp 形态全部 HTTP 行为（must_have truths 前两行）：
// 裸 /wesh → 307 且 Location 为 /wesh/（RawQuery 保留）；/wesh/ → 200 终端页；
// bp 外 / 与 /api/attach → 404；非 POST 打 /wesh/api/attach → 405 + Allow: POST；
// share 交叉：/wesh/s/{token}/ 携有效 token 给页、裸 /wesh/s/{token} 307 补斜杠、
// POST /wesh/api/attach 携有效 token 签发 ticket（无 token 维持 404 探测信号）。
func TestBasePathRoutes(t *testing.T) {
	roTok := server.GenerateShareToken()
	rwTok := server.GenerateShareToken()
	httpBase, _ := startBasePathServer(t, server.Options{
		Writable:     true,
		ShareTokenRO: roTok,
		ShareTokenRW: rwTok,
	})
	// 307 断言不跟随重定向（sharetoken_test.go 同款 client 形态）。
	noFollow := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(url string) *http.Response {
		t.Helper()
		resp, err := noFollow.Get(url)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		return resp
	}

	// 裸 /wesh → 307 补斜杠（matchOrRedirect 内建行为——注册 /wesh/ 子树即免费
	// 获得，D-14 尾斜杠规范化零自写代码）；Location 为 /wesh/。
	resp := get(httpBase + "/wesh")
	readBodyStr(t, resp)
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("GET /wesh status = %d, want %d (307 补斜杠)", resp.StatusCode, http.StatusTemporaryRedirect)
	}
	if loc := resp.Header.Get("Location"); loc != "/wesh/" {
		t.Errorf("GET /wesh Location = %q, want %q", loc, "/wesh/")
	}
	// RawQuery 保留（GOROOT matchOrRedirect 组 URL 时透传 RawQuery）。
	resp = get(httpBase + "/wesh?x=1")
	readBodyStr(t, resp)
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("GET /wesh?x=1 status = %d, want %d (307)", resp.StatusCode, http.StatusTemporaryRedirect)
	}
	if loc := resp.Header.Get("Location"); loc != "/wesh/?x=1" {
		t.Errorf("GET /wesh?x=1 Location = %q, want %q（RawQuery 保留）", loc, "/wesh/?x=1")
	}

	// GET /wesh/ → 200 终端 HTML（StripPrefix 剥离前缀后 embed 页送达）。
	resp = get(httpBase + "/wesh/")
	body := readBodyStr(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /wesh/ status = %d, want %d (终端页)", resp.StatusCode, http.StatusOK)
	}
	if len(body) == 0 {
		t.Error("GET /wesh/ body 为空——embed 页未送达")
	}

	// bp 外路由 404（无 "/" 注册，mux 直接 404）——/ 与 /api/attach 双探测点。
	for _, path := range []string{"/", "/api/attach"} {
		resp = get(httpBase + path)
		readBodyStr(t, resp)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d（bp 外路由不可达）", path, resp.StatusCode, http.StatusNotFound)
		}
	}

	// 非 POST 打 /wesh/api/attach → 405 + Allow: POST（path-only fallback 显式
	// 注册——方法模式内建 405 会被 /wesh/ 子树吞掉，P3 /api/attach 同款纪律）。
	resp = get(httpBase + "/wesh/api/attach")
	readBodyStr(t, resp)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /wesh/api/attach status = %d, want %d (405)", resp.StatusCode, http.StatusMethodNotAllowed)
	}
	if allow := resp.Header.Get("Allow"); allow != http.MethodPost {
		t.Errorf("GET /wesh/api/attach Allow = %q, want %q", allow, http.MethodPost)
	}

	// share 交叉①：有效 token GET /wesh/s/{token}/ → 200 给页（ro/rw 各一）。
	for name, tok := range map[string]string{"ro": roTok, "rw": rwTok} {
		resp = get(httpBase + "/wesh/s/" + tok + "/")
		body = readBodyStr(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("有效 %s token GET bp 形态页 status = %d, want 200", name, resp.StatusCode)
		}
		if len(body) == 0 {
			t.Errorf("有效 %s token GET bp 形态页 body 为空——embed 页未送达", name)
		}
	}

	// share 交叉②：裸 /wesh/s/{token}（占位段 "abc" 非真实 token，输出面无
	// 红线）→ 307 补斜杠（通配语义三坑注释同机制，Location 携路径段属 D-03
	// 已接受暴露面）。
	resp = get(httpBase + "/wesh/s/abc")
	readBodyStr(t, resp)
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("GET /wesh/s/abc status = %d, want %d (307 补斜杠)", resp.StatusCode, http.StatusTemporaryRedirect)
	}
	if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "/wesh/s/abc/") {
		t.Errorf("GET /wesh/s/abc Location = %q, want 以 /wesh/s/abc/ 结尾（补斜杠）", loc)
	}

	// share 交叉③：POST /wesh/api/attach 携有效 token → 200 签发 ticket
	//（无认证模式 OQ1 正交语义在 bp 形态保持）；无 token body → 404 探测
	// 信号不变（前端 bp 页跳过 fetch 直连 WS 的判定面）。
	issueBody, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: roTok})
	if err != nil {
		t.Fatalf("marshal token body: %v", err)
	}
	resp, err = http.Post(httpBase+"/wesh/api/attach", "application/json", strings.NewReader(string(issueBody)))
	if err != nil {
		t.Fatalf("POST /wesh/api/attach: %v", err)
	}
	body = readBodyStr(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /wesh/api/attach 携 token status = %d, want %d (200 签发)", resp.StatusCode, http.StatusOK)
	}
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal([]byte(body), &issued); err != nil {
		t.Fatalf("decode ticket response: %v", err)
	}
	if issued.Ticket == "" {
		t.Fatal("ticket empty in 200 response")
	}
	resp, err = http.Post(httpBase+"/wesh/api/attach", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /wesh/api/attach 无 body: %v", err)
	}
	readBodyStr(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("POST /wesh/api/attach 无 token status = %d, want %d（探测信号不变）", resp.StatusCode, http.StatusNotFound)
	}
}

// TestBasePathWS 锁定 bp 形态 WS 升级全链（must_have truth 第一行后半）：
// 拨 /wesh/ws 完成 wesh.v1 子协议握手 + Hello → 收到 Welcome（dialHello 同款路径）。
func TestBasePathWS(t *testing.T) {
	_, bpWSURL := startBasePathServer(t, server.Options{
		Writable:     true,
		ShareTokenRO: server.GenerateShareToken(),
		ShareTokenRW: server.GenerateShareToken(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, bpWSURL, 80, 24)
	if mode != proto.ModeRW {
		t.Fatalf("welcome mode = %q, want %q (writable=true)", mode, proto.ModeRW)
	}
	c.Close(websocket.StatusNormalClosure, "")
}

// TestBasePathEmptyUnchanged 锁定 BasePath 零值兜底（must_have truth 第三行）：
// Options.BasePath="" 时 Handler() 注册形态与现状一致——/ 给页 200、/ws 升级
// 握手存活（无认证模式 404 探测语义由 sharetoken_test 既有断言覆盖，不重复；
// /s/{token}/ 与 /api/attach 全行为矩阵同由既有套件锁定）。
func TestBasePathEmptyUnchanged(t *testing.T) {
	_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
	httpBase := "http://" + strings.TrimSuffix(strings.TrimPrefix(wsURL, "ws://"), "/ws")
	resp, err := http.Get(httpBase + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body := readBodyStr(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d（零值 BasePath 根挂载不变）", resp.StatusCode, http.StatusOK)
	}
	if len(body) == 0 {
		t.Error("GET / body 为空——embed 页未送达")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 80, 24)
	if mode != proto.ModeRW {
		t.Fatalf("welcome mode = %q, want %q (writable=true)", mode, proto.ModeRW)
	}
	c.Close(websocket.StatusNormalClosure, "")
}
