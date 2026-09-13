package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/sworda/wesh/internal/server"
)

// postJSON 发一个 JSON POST（登录/登出端点用），返回响应（调用方负责 Close body）。
func postJSON(t *testing.T, url string, body any, cookie *http.Cookie) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// postAttachWithCookie 发 POST /api/attach 仅携 cookie（无 Basic），返回状态码。
func postAttachWithCookie(t *testing.T, url string, cookie *http.Cookie) int {
	t.Helper()
	resp := postJSON(t, url, nil, cookie)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// TestSessionLoginFlow（2026-09-13 表单登录 + cookie 会话，模式无关双跑）：
// 错误口令 401 → 正确口令 200 + HttpOnly/Lax cookie → 携 cookie 的 /api/attach
// 200（无需 Basic）→ 无凭据 attach 401（探测）→ 登出清 cookie → 旧 cookie 失效。
// 同时验证会话持久化：另一实例共享同一 SessionFile 时同一 cookie 仍有效
//（wesh 崩溃/重启用户无感的机制证据）。
func TestSessionLoginFlow(t *testing.T) {
	cred, err := server.ParseCredential("login-erin:login-pass")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			sessFile := filepath.Join(t.TempDir(), "sessions.json")
			_, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.Credentials = []server.Credential{cred}
				o.ThrottleBase = 50 * time.Millisecond
				o.SessionTTL = time.Hour
				o.SessionFile = sessFile
			})
			base := httpBaseOf(wsURL)
			attach := attachURL(wsURL)

			// ① 错误口令 → 401（计数走节流；等待过窗避免污染后续正确口令）。
			resp := postJSON(t, base+"/api/login", map[string]string{"user": "login-erin", "pass": "wrong"}, nil)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("错误口令 status = %d, want 401", resp.StatusCode)
			}
			time.Sleep(150 * time.Millisecond)

			// ② 正确口令 → 200 + 会话 cookie（HttpOnly + SameSite=Lax）。
			resp = postJSON(t, base+"/api/login", map[string]string{"user": "login-erin", "pass": "login-pass"}, nil)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("正确口令 status = %d, want 200", resp.StatusCode)
			}
			var cookie *http.Cookie
			for _, c := range resp.Cookies() {
				if c.Name == "wesh_session" {
					cookie = c
				}
			}
			if cookie == nil || cookie.Value == "" {
				t.Fatal("登录成功但未下发 wesh_session cookie")
			}
			if !cookie.HttpOnly {
				t.Error("会话 cookie 缺 HttpOnly")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
			}

			// ③ 携 cookie 的 /api/attach → 200（无需 Basic 凭据）。
			if code := postAttachWithCookie(t, attach, cookie); code != http.StatusOK {
				t.Fatalf("携 cookie attach status = %d, want 200", code)
			}
			// ④ 无 cookie 无凭据 → 401（探测，不挑战）。
			if code := postAttachWithCookie(t, attach, nil); code != http.StatusUnauthorized {
				t.Fatalf("无凭据 attach status = %d, want 401", code)
			}

			// ⑤ 持久化：另一实例（模拟 wesh 重启）共享 SessionFile → 同一 cookie 有效。
			_, wsURL2 := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.Credentials = []server.Credential{cred}
				o.SessionTTL = time.Hour
				o.SessionFile = sessFile
			})
			if code := postAttachWithCookie(t, attachURL(wsURL2), cookie); code != http.StatusOK {
				t.Fatalf("重启后携 cookie attach status = %d, want 200（会话未持久化）", code)
			}

			// ⑥ 登出 → 200；旧 cookie 立即失效。
			resp = postJSON(t, base+"/api/logout", nil, cookie)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("logout status = %d, want 200", resp.StatusCode)
			}
			if code := postAttachWithCookie(t, attach, cookie); code != http.StatusUnauthorized {
				t.Fatalf("登出后旧 cookie attach status = %d, want 401", code)
			}
		})
	}
}
