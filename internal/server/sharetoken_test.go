package server

// sharetoken_test.go —— TestShareToken（VALIDATION 05-02-01 Go 侧，MULTI-05）：
// lookup 矩阵（同包白盒）+ /s/ 门禁（handler 集成）+ /api/attach token 分支
// 三面锁定。同包白盒（tickets_test.go 先例——内部类型不导出）；handler 集成须
// 完整 New 装配（shares 接线在 New 内），harness 为本文件局部最小复刻
// （startShareServer/dialTicketMode——e2e_test.go startTestServerWith/
// dialHelloTicket 的同包映射，Go 单文件单 package 约束下跨包不可复用，
// 05-04 两测试分文件先例的同族推论）。
//
// 红线（D-03/SEC-01 延伸，phase04.mjs 同款纪律的 Go 映射）：token 值只存局部
// 变量作断言材料，永不进 t.Log/错误信息——断言输出只含状态码/布尔/形状。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
)

// startShareServer 是本文件的最小装配 harness（startTestServerWith 的同包映射）：
// pty.Start + New + 127.0.0.1:0 监听，返回 http base URL。/bin/cat 测试期间不
// 退出，exitf 不断言（生命周期断言归 e2e/multi 套件），noop 桩即可。
func startShareServer(t *testing.T, opts Options) string {
	t.Helper()
	// 零值等价形态（07-04 选项化适配：Uid/Gid -1 = 不降权，Dir/Term 空 = 现状）。
	sess, err := pty.Start([]string{"/bin/cat"}, pty.StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	srv := New(sess, func(int) {}, opts)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	// 本包（server 内部包）无法复用 server_test 包的 killServer——内联同款清理
	//（子进程 Kill 触发 lifecycle 风收；sess.Close 幂等兜底；泄漏危害见
	// e2e_test.go killServer 注释）。
	t.Cleanup(func() {
		ln.Close()
		if sess.Cmd != nil && sess.Cmd.Process != nil {
			_ = sess.Cmd.Process.Kill()
		}
		_ = sess.Close()
	})
	go http.Serve(ln, srv.Handler())
	return "http://" + ln.Addr().String()
}

// dialTicketMode 是 dialHelloTicket 的同包最小映射：携 ticket 完成 Hello 握手，
// 返回 Welcome mode（连接随即正常关闭——断开只走 detach，多客户端推论）。
func dialTicketMode(t *testing.T, ctx context.Context, base, ticket string) string {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/ws"
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	payload, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24, Ticket: ticket})
	if err != nil {
		t.Fatalf("marshal Hello: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, payload...)); err != nil {
		t.Fatalf("write Hello: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read Welcome: %v", err)
	}
	if len(data) == 0 || data[0] != proto.Welcome {
		t.Fatalf("first frame = %v, want Welcome ('W')", data)
	}
	var wp proto.WelcomePayload
	if err := json.Unmarshal(data[1:], &wp); err != nil {
		t.Fatalf("decode Welcome: %v", err)
	}
	return wp.Mode
}

// postAttachBody 发 POST /api/attach（body 原样给定，nil = 空体）。
func postAttachBody(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/attach: %v", err)
	}
	return resp
}

// readBody 读尽并关闭响应体，返回字节串（调用方不再 Close）。
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

// issueViaToken 经分享 token 通道取一次性 ticket：POST body 携 token → 断言
// 200 + Cache-Control: no-store + ticket 非空，返回 ticket（断言材料，不打印）。
func issueViaToken(t *testing.T, url string, token string) string {
	t.Helper()
	body, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: token})
	if err != nil {
		t.Fatalf("marshal token body: %v", err)
	}
	resp := postAttachBody(t, url, body)
	if resp.StatusCode != http.StatusOK {
		readBody(t, resp)
		t.Fatalf("token 通道签发 status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		readBody(t, resp)
		t.Fatalf("Cache-Control = %q, want no-store（ticket 不可落缓存）", cc)
	}
	var issued struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(readBody(t, resp), &issued); err != nil {
		t.Fatalf("decode ticket response: %v", err)
	}
	if issued.Ticket == "" {
		t.Fatal("ticket empty in 200 response")
	}
	return issued.Ticket
}

func TestShareToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// lookup 矩阵（白盒）：ro/rw 各自命中返回绑定 mode（D-01）；错 token/空串/
	// 22 字符同形异值同归 ("", false) 无 oracle；生成形态 22 字符
	// base64.RawURLEncoding（16B，tickets.go:45-49 形态）；nil/空串 = 通道关闭。
	t.Run("lookup 矩阵与生成形态", func(t *testing.T) {
		roTok := GenerateShareToken()
		rwTok := GenerateShareToken()
		for _, tok := range []string{roTok, rwTok} {
			if len(tok) != 22 {
				t.Fatalf("len(GenerateShareToken) = %d, want 22（16B → base64.RawURLEncoding）", len(tok))
			}
			raw, err := base64.RawURLEncoding.DecodeString(tok)
			if err != nil || len(raw) != 16 {
				t.Fatalf("生成形态非 base64url(16B)：err=%v rawlen=%d", err, len(raw))
			}
		}
		if roTok == rwTok {
			t.Fatal("ro/rw 两 token 相同——随机源失效（C6）")
		}
		st := newShareTokens(roTok, rwTok)
		if st == nil {
			t.Fatal("newShareTokens(两非空) = nil，want 非 nil")
		}
		if mode, ok := st.lookup(roTok); !ok || mode != proto.ModeRO {
			t.Errorf("lookup(ro) = (%q, %v), want (%q, true)", mode, ok, proto.ModeRO)
		}
		if mode, ok := st.lookup(rwTok); !ok || mode != proto.ModeRW {
			t.Errorf("lookup(rw) = (%q, %v), want (%q, true)", mode, ok, proto.ModeRW)
		}
		// 22 字符同形异值（首字符翻转）+ 空串 + 畸形串 + 超长串同归 ("", false)。
		wrong := "A" + roTok[1:]
		if wrong == roTok {
			wrong = "B" + roTok[1:]
		}
		for i, bad := range []string{"", "not-a-token", wrong, roTok + "x"} {
			if mode, ok := st.lookup(bad); ok || mode != "" {
				t.Errorf("lookup(非法形态 #%d) = (%q, %v), want (\"\", false)（无 oracle）", i, mode, ok)
			}
		}
		// 通道关闭语义：任一空串构造 → nil；nil receiver 恒 miss。
		if newShareTokens("", rwTok) != nil || newShareTokens(roTok, "") != nil {
			t.Error("任一空串 = 通道关闭（nil）语义破坏")
		}
		var nilSt *shareTokens
		if _, ok := nilSt.lookup(roTok); ok {
			t.Error("nil store lookup 命中——通道关闭语义破坏")
		}
	})

	// /s/ 门禁（handler 集成）：凭据模式——有效 ro/rw token GET 200 且无 Basic
	// challenge；错 token → 401 challenge（委托 / 链）；POST → 405 + Allow: GET；
	// 无尾斜杠 → 307 补斜杠（GOROOT 内建行为）。无认证模式——错 token 同样 200
	// 给页（全站本无门，D-01 字面落地）。
	t.Run("/s/ 门禁", func(t *testing.T) {
		cred, err := ParseCredential("share-op:test-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		roTok := GenerateShareToken()
		rwTok := GenerateShareToken()
		base := startShareServer(t, Options{
			Writable:     true,
			Credentials:  []Credential{cred},
			ThrottleBase: 50 * time.Millisecond,
			ShareTokenRO: roTok,
			ShareTokenRW: rwTok,
		})

		// 有效 token → 200 且无 challenge（断言不出现认证挑战头，不打印 token）。
		for name, suffix := range map[string]string{"ro": "/s/" + roTok + "/", "rw": "/s/" + rwTok + "/"} {
			resp, err := http.Get(base + suffix)
			if err != nil {
				t.Fatalf("GET 有效 token 页: %v", err)
			}
			body := readBody(t, resp)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("有效 %s token GET status = %d, want 200", name, resp.StatusCode)
			}
			if wa := resp.Header.Get("WWW-Authenticate"); wa != "" {
				t.Errorf("有效 %s token GET 出现 WWW-Authenticate——token 门禁未生效（委托错进了 basicAuth）", name)
			}
			if len(body) == 0 {
				t.Errorf("有效 %s token GET body 为空——embed 页未送达", name)
			}
		}

		// 错 token → 委托 / 链：凭据模式 401 challenge（D-08 recordFail 经此路径
		// 自动计入，fail#1；本子测后续不再触 basicAuth，免 pacing）。
		resp, err := http.Get(base + "/s/" + roTok + "x/") // 23 字符同形异值
		if err != nil {
			t.Fatalf("GET 错 token 页: %v", err)
		}
		readBody(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("错 token GET status = %d, want %d (401 challenge)", resp.StatusCode, http.StatusUnauthorized)
		}
		if wa := resp.Header.Get("WWW-Authenticate"); wa != `Basic realm="wesh", charset="UTF-8"` {
			t.Errorf("错 token GET WWW-Authenticate = %q, want RFC 7617 challenge", wa)
		}

		// POST /s/abc/ → 405 + Allow: GET（path-only fallback；mux 层不触
		// basicAuth）。占位路径段非真实 token，不涉红线。
		resp = postAttachBody(t, base+"/s/abc/", nil)
		readBody(t, resp)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("POST /s/abc/ status = %d, want %d (405)", resp.StatusCode, http.StatusMethodNotAllowed)
		}
		if allow := resp.Header.Get("Allow"); allow != http.MethodGet {
			t.Errorf("POST /s/abc/ Allow = %q, want %q", allow, http.MethodGet)
		}

		// GET /s/abc（无尾斜杠）→ 307 补斜杠（GOROOT matchOrRedirect 内建行为——
		// go1.22+ 新 mux 恒用 307 保方法，GOROOT server.go:2687；RESEARCH『301』
		// 系笔误，GET 下两码语义等价；Location 头携路径段属 D-03 已接受暴露面）。
		noFollow := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err = noFollow.Get(base + "/s/abc")
		if err != nil {
			t.Fatalf("GET 无尾斜杠: %v", err)
		}
		readBody(t, resp)
		if resp.StatusCode != http.StatusTemporaryRedirect {
			t.Fatalf("GET /s/abc status = %d, want %d (307 补斜杠)", resp.StatusCode, http.StatusTemporaryRedirect)
		}
		if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "/s/abc/") {
			t.Errorf("GET /s/abc Location = %q, want 以 /s/abc/ 结尾（补斜杠）", loc)
		}

		// 无认证模式（OQ1 正交）：/s/ 路由同样注册；错 token → 200 给页（全站
		// 本无门——无/错 token 时矩阵不变的字面落地），有效 token → 200 给页。
		baseNoAuth := startShareServer(t, Options{
			Writable:     true,
			ShareTokenRO: roTok,
			ShareTokenRW: rwTok,
		})
		for name, suffix := range map[string]string{"有效": "/s/" + roTok + "/", "错": "/s/" + roTok + "x/"} {
			resp, err := http.Get(baseNoAuth + suffix)
			if err != nil {
				t.Fatalf("无认证 GET %stoken 页: %v", name, err)
			}
			body := readBody(t, resp)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("无认证 %s token GET status = %d, want 200（给页无门）", name, resp.StatusCode)
			}
			if len(body) == 0 {
				t.Errorf("无认证 %s token GET body 为空——embed 页未送达", name)
			}
		}
	})

	// /api/attach token 分支：ro/rw token → ticket → Hello 核销得绑定 mode
	// （全链）；错 token 与无 token 的 401 逐字节一致（无 oracle）；无认证模式
	// 携 token 非 404 出 ticket 且 mode 绑定兑现（OQ1），无 body 维持 404
	// （前端探测信号不变），错 token → 401（G-05-7，C-3 承接）。
	t.Run("/api/attach token 分支", func(t *testing.T) {
		cred, err := ParseCredential("share-op:test-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		roTok := GenerateShareToken()
		rwTok := GenerateShareToken()
		attachPath := "/api/attach"

		// —— 凭据模式（--writable，owner 默认策略）——
		base := startShareServer(t, Options{
			Writable:     true,
			Credentials:  []Credential{cred},
			ThrottleBase: 50 * time.Millisecond,
			ShareTokenRO: roTok,
			ShareTokenRW: rwTok,
		})
		url := base + attachPath

		// 成功流先行（throttle 干净）：ro token → ticket → Hello → mode ro。
		if mode := dialTicketMode(t, ctx, base, issueViaToken(t, url, roTok)); mode != proto.ModeRO {
			t.Errorf("ro token 全链 Welcome mode = %q, want %q（D-01 mode 绑定）", mode, proto.ModeRO)
		}
		// rw token → ticket → Hello → mode rw。
		if mode := dialTicketMode(t, ctx, base, issueViaToken(t, url, rwTok)); mode != proto.ModeRW {
			t.Errorf("rw token 全链 Welcome mode = %q, want %q（D-01 mode 绑定）", mode, proto.ModeRW)
		}

		// 失败流殿后（pacing 镜像 TestAttachFlow：fail#1 窗口 = 1×base = 50ms，
		// sleep 100ms 过窗）——错 token body → 401（fail#1）。
		badBody, err := json.Marshal(struct {
			Token string `json:"token"`
		}{Token: roTok + "x"}) // 同形异值，断言材料不打印
		if err != nil {
			t.Fatalf("marshal wrong-token body: %v", err)
		}
		respBad := postAttachBody(t, url, badBody)
		bodyBad := readBody(t, respBad)
		waBad := respBad.Header.Get("WWW-Authenticate")
		if respBad.StatusCode != http.StatusUnauthorized {
			t.Fatalf("错 token attach status = %d, want %d（委托 Basic 链）", respBad.StatusCode, http.StatusUnauthorized)
		}
		time.Sleep(100 * time.Millisecond) // 过窗（fail#1 = 50ms）
		// 无 token body → 401（fail#2）——与错 token 响应逐字节一致（无 oracle，
		// T-05-05 缓解断言面）。
		respNone := postAttachBody(t, url, nil)
		bodyNone := readBody(t, respNone)
		if respNone.StatusCode != http.StatusUnauthorized {
			t.Fatalf("无 token attach status = %d, want %d", respNone.StatusCode, http.StatusUnauthorized)
		}
		if !bytes.Equal(bodyBad, bodyNone) || waBad != respNone.Header.Get("WWW-Authenticate") {
			t.Error("错 token 与无 token 的 401 响应不同文——枚举 oracle（T-05-05）")
		}

		// —— 无认证模式（--writable；OQ1 正交：ro/rw mode 绑定在无密码演示场景
		// 兑现；throttle 面不存在，免 pacing）——
		baseNoAuth := startShareServer(t, Options{
			Writable:     true,
			ShareTokenRO: roTok,
			ShareTokenRW: rwTok,
		})
		urlNoAuth := baseNoAuth + attachPath

		// body 携 ro token → 非 404 出 ticket → Hello 核销 mode ro。
		if mode := dialTicketMode(t, ctx, baseNoAuth, issueViaToken(t, urlNoAuth, roTok)); mode != proto.ModeRO {
			t.Errorf("无认证 ro token 全链 Welcome mode = %q, want %q（OQ1 mode 绑定）", mode, proto.ModeRO)
		}
		// body 携 rw token → mode rw。
		if mode := dialTicketMode(t, ctx, baseNoAuth, issueViaToken(t, urlNoAuth, rwTok)); mode != proto.ModeRW {
			t.Errorf("无认证 rw token 全链 Welcome mode = %q, want %q（OQ1 mode 绑定）", mode, proto.ModeRW)
		}
		// 无 body → 404（前端探测信号不变）。
		resp := postAttachBody(t, urlNoAuth, nil)
		readBody(t, resp)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("无认证无 body attach status = %d, want %d（探测信号不变）", resp.StatusCode, http.StatusNotFound)
		}
		// 错 token → 401（G-05-7，用户 2026-08-22 裁决：不弹登录框的通道
		// 必须给 Invalid share link 面板——前端「携 token 401 → C-3」承接）。
		resp = postAttachBody(t, urlNoAuth, badBody)
		readBody(t, resp)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("无认证错 token attach status = %d, want %d（G-05-7 C-3 承接）", resp.StatusCode, http.StatusUnauthorized)
		}
		if resp.Header.Get("WWW-Authenticate") != "" {
			t.Error("无认证错 token 401 不应携带 WWW-Authenticate 挑战头（无凭据可弹）")
		}
	})
}
