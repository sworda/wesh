package server_test

// health_test.go —— 08-03 OPS-06 /healthz 探活端点行为锁（D-07 免认证窄例外 /
// D-09 根路径固定 / D-10 200+状态 JSON 四字段 / D-11 关停 503 draining）：
//   - ok 四字段：无认证实例 GET /healthz → 200 application/json，status=="ok"、
//     clients（dialHello 前 0 → 后 1）、max_clients==32（默认）、session_active==true；
//   - 免认证窄例外：凭据实例无 Authorization 头 GET /healthz → 200 同形态，
//     对照 GET / 同请求仍 401（D-07 整站 Basic 闸唯一例外，例外不蔓延）；
//   - bp 固定：bp=/wesh 两模式 GET /healthz → 200；GET /wesh/healthz → 无认证
//     404 / 凭据 401（D-09 根路径固定，拒绝双挂）；
//   - 405：POST /healthz → 405 + Allow: GET（方法模式 + path-only fallback 成对
//     注册——内建 405 会被 "/" 子树吞掉，sharetoken.go 先例）；
//   - session_active 翻转：exitf 桩实例子进程退出后 GET /healthz → 200 且
//     session_active==false（sessionAlive 与 session_end 同区段置位）。
//
// 红线（D-10/T-08-03a）：body 键集白名单断言（DisallowUnknownFields）——
// 恒为 status/clients/max_clients/session_active 四字段粗粒度容量面，无版本号、
// 无客户端身份、无内部错误细节（version 只在需认证的 /metrics build_info）。
// http 客户端用 net/http 直发（wsURL 推导 http base：ws://→http:// 换 scheme
// 去 /ws 尾，startBasePathServer 同款推导）。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/server"
)

// healthzBody 为 /healthz 响应 JSON 的四字段粗粒度容量面（D-10 逐字键名）。
type healthzBody struct {
	Status        string `json:"status"`
	Clients       int64  `json:"clients"`
	MaxClients    int    `json:"max_clients"`
	SessionActive bool   `json:"session_active"`
}

// httpBaseOf 从 wsURL 推导 http base（ws://→http:// 换 scheme 去 /ws 尾）。
func httpBaseOf(wsURL string) string {
	return "http://" + strings.TrimSuffix(strings.TrimPrefix(wsURL, "ws://"), "/ws")
}

// getHealthz GET 指定 URL，断言 200/503 两态之一 + Content-Type application/json，
// 严格解码 body 返回（键集白名单锁——DisallowUnknownFields，多一键即 FAIL，
// T-08-03a prohibition 行为锁形态）。仅用于健康端点本身；404/401 对照用裸 GET。
func getHealthz(t *testing.T, url string) (int, healthzBody) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		_, _ = io.Copy(io.Discard, resp.Body)
		t.Fatalf("GET %s status = %d, want 200 或 503（/healthz 两态）", url, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		_, _ = io.Copy(io.Discard, resp.Body)
		t.Fatalf("GET %s Content-Type = %q, want application/json", url, ct)
	}
	var hb healthzBody
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&hb); err != nil {
		t.Fatalf("GET %s body 严格解码失败（键集白名单违反？）: %v", url, err)
	}
	return resp.StatusCode, hb
}

// getStatus 裸 GET 断言状态码（404/401 对照面），读尽并关闭 body。
func getStatus(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// assertHealthz 断言四字段逐值相等（D-10 语义锁）。
func assertHealthz(t *testing.T, hb healthzBody, status string, clients int64, maxClients int, sessionActive bool) {
	t.Helper()
	if hb.Status != status {
		t.Errorf("status = %q, want %q", hb.Status, status)
	}
	if hb.Clients != clients {
		t.Errorf("clients = %d, want %d", hb.Clients, clients)
	}
	if hb.MaxClients != maxClients {
		t.Errorf("max_clients = %d, want %d", hb.MaxClients, maxClients)
	}
	if hb.SessionActive != sessionActive {
		t.Errorf("session_active = %v, want %v", hb.SessionActive, sessionActive)
	}
}

// TestHealthz（08-03 OPS-06 主干，D-07/D-09/D-10）：五子测锁定 /healthz 全行为。
func TestHealthz(t *testing.T) {
	// ok 四字段：无认证实例，dialHello 前后 clients 0→1；status ok、max_clients
	// 默认 32、session_active true（会话存活）。
	t.Run("ok_fields", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
		base := httpBaseOf(wsURL)

		code, hb := getHealthz(t, base+"/healthz")
		if code != http.StatusOK {
			t.Fatalf("GET /healthz status = %d, want %d", code, http.StatusOK)
		}
		assertHealthz(t, hb, "ok", 0, 32, true)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c, _ := dialHello(t, ctx, wsURL, 80, 24)
		defer func() { _ = c.Close(websocket.StatusNormalClosure, "") }()

		code, hb = getHealthz(t, base+"/healthz")
		if code != http.StatusOK {
			t.Fatalf("GET /healthz（attach 后）status = %d, want %d", code, http.StatusOK)
		}
		assertHealthz(t, hb, "ok", 1, 32, true)
	})

	// 免认证窄例外：凭据实例无 Authorization 头 GET /healthz → 200 同形态；
	// 对照 GET / 同请求仍 401（D-07——例外不蔓延的行为锁）。
	t.Run("unauthenticated_exception", func(t *testing.T) {
		cred, err := server.ParseCredential("hz-op:hz-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			Credentials: []server.Credential{cred},
		})
		base := httpBaseOf(wsURL)

		code, hb := getHealthz(t, base+"/healthz")
		if code != http.StatusOK {
			t.Fatalf("凭据实例无头 GET /healthz status = %d, want %d（D-07 免认证窄例外）", code, http.StatusOK)
		}
		assertHealthz(t, hb, "ok", 0, 32, true)

		if got := getStatus(t, base+"/"); got != http.StatusUnauthorized {
			t.Errorf("对照 GET / status = %d, want %d（整站 Basic 闸不受例外影响）", got, http.StatusUnauthorized)
		}
	})

	// bp 固定：bp=/wesh 实例下 /healthz 仍 200、/wesh/healthz 不可达（无认证 404 /
	// 凭据 401）——D-09 根路径固定，拒绝双挂（探活路径可写死进 k8s probe 配置）。
	t.Run("basepath_pinned", func(t *testing.T) {
		// 无认证 bp 实例：/healthz 200；/wesh/healthz 404（embed FS 无此路径）。
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, BasePath: "/wesh"})
		base := httpBaseOf(wsURL)
		if code, hb := getHealthz(t, base+"/healthz"); code != http.StatusOK {
			t.Errorf("bp 实例 GET /healthz status = %d, want %d（D-09 根路径固定）", code, http.StatusOK)
		} else {
			assertHealthz(t, hb, "ok", 0, 32, true)
		}
		if got := getStatus(t, base+"/wesh/healthz"); got != http.StatusNotFound {
			t.Errorf("无认证 bp 实例 GET /wesh/healthz status = %d, want %d（拒绝双挂）", got, http.StatusNotFound)
		}

		// 凭据 bp 实例：/healthz 无头 200；/wesh/healthz 无头 401（bp 子树经
		// basicAuth 闸——探活例外不蔓延到 bp 挂载面）。
		cred, err := server.ParseCredential("hz-bp:hz-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		_, wsURLCred := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			BasePath:    "/wesh",
			Credentials: []server.Credential{cred},
		})
		baseCred := httpBaseOf(wsURLCred)
		if code, _ := getHealthz(t, baseCred+"/healthz"); code != http.StatusOK {
			t.Errorf("凭据 bp 实例无头 GET /healthz status = %d, want %d", code, http.StatusOK)
		}
		if got := getStatus(t, baseCred+"/wesh/healthz"); got != http.StatusUnauthorized {
			t.Errorf("凭据 bp 实例无头 GET /wesh/healthz status = %d, want %d（例外不蔓延）", got, http.StatusUnauthorized)
		}
	})

	// 405：POST /healthz → 405 + Allow: GET（path-only fallback 显式注册——
	// 否则 POST 落进 "/" 子树静态伺服，探活器配置错误被静默掩盖，RESEARCH Pitfall 7）。
	t.Run("method_405", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
		base := httpBaseOf(wsURL)

		resp, err := http.Post(base+"/healthz", "application/octet-stream", nil)
		if err != nil {
			t.Fatalf("POST /healthz: %v", err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST /healthz status = %d, want %d (405)", resp.StatusCode, http.StatusMethodNotAllowed)
		}
		if allow := resp.Header.Get("Allow"); allow != http.MethodGet {
			t.Errorf("POST /healthz Allow = %q, want %q", allow, http.MethodGet)
		}
	})

	// session_active 翻转：exitf 桩实例子进程退出（exit 42）后 GET /healthz →
	// 200 且 session_active==false——lifecycle 的 sessionAlive 置 false 先于
	// terminate→exitf（程序序），waitExit 收码即同步边。
	t.Run("session_active_flip", func(t *testing.T) {
		exitCh, wsURL := startTestServerWith(t, []string{"sh", "-c", "exit 42"}, server.Options{Writable: true})
		base := httpBaseOf(wsURL)
		waitExit(t, exitCh, 42)

		code, hb := getHealthz(t, base+"/healthz")
		if code != http.StatusOK {
			t.Fatalf("子进程退出后 GET /healthz status = %d, want %d（进程存活即探活通过）", code, http.StatusOK)
		}
		assertHealthz(t, hb, "ok", 0, 32, false)
	})
}
