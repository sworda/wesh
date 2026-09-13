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
//     session_active==false（sessionAlive 与 session_end 同区段置位）；
//   - draining：Shutdown() 后 GET /healthz → 503 status=="draining" 四键在场
//     （D-11——1001 广播开始前即翻转，关停全程探活器不再导新流）。
//
// 14-01 双模式分叉表改造（D-01/D-02，PITFALLS :393 行「health = 双模式断言
// 分叉：session_active 语义」）：装配统一换 newTestServer/newHandleTestServer
// 小族 t.Run 双跑。断言分叉点 = session_active 语义（13-05 D-06 裁决口径）：
// shared = 生命周期跟随（sessionAlive 位）；per-client = 恒 true（会话服务
// 恒可用——含零会话形态，health.go:44-46；生命周期跟随实现若被误装配到此
// 面，session_active_flip 的 per-client 列翻车，可证伪）。其余四子测（免认证
// 例外 / bp 固定 / 405 / ok 四字段值）两模式同断言。
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
// 严格解码 body 返回。键集白名单锁（T-08-03a prohibition 行为锁完整形态）：
// 四键恰好——多一键（版本/身份/错误细节混入）或少一键（字段丢失）皆 FAIL，
// 200 与 503 两态同锁（draining body 四键仍在场，D-11）。仅用于健康端点本身；
// 404/401 对照用裸 GET。
func getHealthz(t *testing.T, url string) (int, healthzBody) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET %s read body: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET %s status = %d, want 200 或 503（/healthz 两态）", url, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET %s Content-Type = %q, want application/json", url, ct)
	}
	var keys map[string]any
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatalf("GET %s body 非合法 JSON: %q: %v", url, b, err)
	}
	for _, k := range []string{"status", "clients", "max_clients", "session_active"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("GET %s body 缺键 %q（四键白名单）: %s", url, k, b)
		}
		delete(keys, k)
	}
	if len(keys) != 0 {
		t.Fatalf("GET %s body 多键 %v（白名单外——版本/身份/错误细节禁止混入）: %s", url, keys, b)
	}
	var hb healthzBody
	if err := json.Unmarshal(b, &hb); err != nil {
		t.Fatalf("GET %s body 解码失败: %q: %v", url, b, err)
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
// 14-01 双模式双跑——session_active 语义分叉表见各子测（shared = 生命周期跟随
// / per-client = 13-05 恒 true），其余断言两模式同值。
func TestHealthz(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			// ok 四字段：无认证实例，dialHello 前后 clients 0→1；status ok、
			// max_clients 默认 32。断言分叉表（D-02）：session_active 两列同为
			// true——语义分叉：shared = 单会话存活（sessionAlive 位）；per-client
			// = 13-05 恒 true 裁决（attach 前零会话形态即为该裁决的零会话证面）。
			t.Run("ok_fields", func(t *testing.T) {
				_, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, nil)
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
				_, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.Credentials = []server.Credential{cred}
				})
				base := httpBaseOf(wsURL)

				code, hb := getHealthz(t, base+"/healthz")
				if code != http.StatusOK {
					t.Fatalf("凭据实例无头 GET /healthz status = %d, want %d（D-07 免认证窄例外）", code, http.StatusOK)
				}
				assertHealthz(t, hb, "ok", 0, 32, true)

				// 2026-09-13 表单登录改造：GET / 公开（SPA shell 承载登录视图），
				// 认证闸收窄到 /api/attach、/ws 与 /metrics——对照点随之由 401
				// 变为 200（免认证例外仍在 /healthz 本身，语义不受影响）。
				if got := getStatus(t, base+"/"); got != http.StatusOK {
					t.Errorf("对照 GET / status = %d, want %d（GET / 公开承载登录视图，2026-09-13）", got, http.StatusOK)
				}
			})

			// bp 固定：bp=/wesh 实例下 /healthz 仍 200、/wesh/healthz 不可达（404）
			// ——D-09 根路径固定，拒绝双挂（探活路径可写死进 k8s probe 配置）。
			// 2026-09-13：GET / 公开后 bp 子树不再经 basicAuth 拦截，故凭据实例的
			// /wesh/healthz 也从 401 变为 404（embed FS 无此路径），与无认证形态一致。
			t.Run("basepath_pinned", func(t *testing.T) {
				// 无认证 bp 实例：/healthz 200；/wesh/healthz 404（embed FS 无此路径）。
				_, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.BasePath = "/wesh"
				})
				base := httpBaseOf(wsURL)
				if code, hb := getHealthz(t, base+"/healthz"); code != http.StatusOK {
					t.Errorf("bp 实例 GET /healthz status = %d, want %d（D-09 根路径固定）", code, http.StatusOK)
				} else {
					assertHealthz(t, hb, "ok", 0, 32, true)
				}
				if got := getStatus(t, base+"/wesh/healthz"); got != http.StatusNotFound {
					t.Errorf("无认证 bp 实例 GET /wesh/healthz status = %d, want %d（拒绝双挂）", got, http.StatusNotFound)
				}

				// 凭据 bp 实例：/healthz 无头 200；/wesh/healthz 无头 404（2026-09-13
				// GET / 公开后 bp 子树不再有认证闸拦截，落 embed FS 404）。
				cred, err := server.ParseCredential("hz-bp:hz-pass")
				if err != nil {
					t.Fatalf("ParseCredential: %v", err)
				}
				_, wsURLCred := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.BasePath = "/wesh"
					o.Credentials = []server.Credential{cred}
				})
				baseCred := httpBaseOf(wsURLCred)
				if code, _ := getHealthz(t, baseCred+"/healthz"); code != http.StatusOK {
					t.Errorf("凭据 bp 实例无头 GET /healthz status = %d, want %d", code, http.StatusOK)
				}
				if got := getStatus(t, baseCred+"/wesh/healthz"); got != http.StatusNotFound {
					t.Errorf("凭据 bp 实例无头 GET /wesh/healthz status = %d, want %d（bp 子树无认证闸，embed 404）", got, http.StatusNotFound)
				}
			})

			// 405：POST /healthz → 405 + Allow: GET（path-only fallback 显式注册——
			// 否则 POST 落进 "/" 子树静态伺服，探活器配置错误被静默掩盖，RESEARCH
			// Pitfall 7）。
			t.Run("method_405", func(t *testing.T) {
				_, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, nil)
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
			// 200。断言分叉表（D-02，13-05 D-06 裁决口径）：
			//	shared（v1.0 逐字）= false——lifecycle 的 sessionAlive 置 false
			//	  先于 terminate→exitf（程序序），waitExit 收码即同步边；无客户端
			//	  形态（启动期 spawn 即退 42）。
			//	per-client = true 恒——会话服务恒可用语义（子会话死亡不翻转探活
			//	  面；attach 期 spawn 使零客户端形态无会话可死，本列必须 attach
			//	  构造会话死亡形态）；exitf 静默分叉（无第二终结源，服务端续跑）。
			t.Run("session_active_flip", func(t *testing.T) {
				exitCh, wsURL := newTestServer(t, mode, []string{"sh", "-c", "exit 42"}, nil)
				base := httpBaseOf(wsURL)

				if mode == server.SessionModePerClient {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					c, _ := dialHello(t, ctx, wsURL, 80, 24) // attach 期 spawn sh -c "exit 42"
					frames, code := readExitClose(t, ctx, c)
					if code != websocket.StatusNormalClosure {
						t.Fatalf("per-client 属主 close code = %d, want %d (1000)", code, websocket.StatusNormalClosure)
					}
					if len(frames) == 0 {
						t.Fatal("per-client 属主无帧——私有 EXIT 帧缺失")
					}

					// 注册表收敛轮询（detach 移除点与 CloseError 读到点存在微竞态，
					// 2s 护栏——Phase 9 轮询纪律）。
					deadline := time.Now().Add(2 * time.Second)
					var hb healthzBody
					var gotCode int
					for {
						gotCode, hb = getHealthz(t, base+"/healthz")
						if hb.Clients == 0 || time.Now().After(deadline) {
							break
						}
						time.Sleep(10 * time.Millisecond)
					}
					if gotCode != http.StatusOK {
						t.Fatalf("子会话退出后 GET /healthz status = %d, want %d", gotCode, http.StatusOK)
					}
					assertHealthz(t, hb, "ok", 0, 32, true)
					// exitf 静默：per-client 无第二终结源（exit-when-empty 未开），
					// 会话死亡仅私有化送达属主——服务端续跑（PC-02/03 语义面）。
					assertNoExit(t, exitCh)
					return
				}

				waitExit(t, exitCh, 42)

				code, hb := getHealthz(t, base+"/healthz")
				if code != http.StatusOK {
					t.Fatalf("子进程退出后 GET /healthz status = %d, want %d（进程存活即探活通过）", code, http.StatusOK)
				}
				assertHealthz(t, hb, "ok", 0, 32, false)
			})
		})
	}
}

// TestHealthzDraining（08-03，D-11）：srv.Shutdown() 调用前 GET /healthz →
// 200 status=="ok"；调用后 → 503 status=="draining" 且四键仍在场（键集白名单
// 同锁，值不锚定——子进程死亡时点与请求时序竞争，session_active 两值皆合法）。
// 置位点 = Shutdown 入口（与 s.exiting=true 同源）——1001 广播开始前即翻转，
// 关停全程探活器不再导新流（waitExit 后复访仍 503 锁定全程语义）。
//
// 14-01 双模式分叉表改造：:251 原 startShutdownServerWith 本地 srv 变体收编至
// newHandleTestServer（srv 直调 Shutdown 语义两列同构；waitHandlers 忽略——
// wg 惰性无害，harness_test.go 注释）。断言分叉表（D-02）：
//
//	退出码——shared = -1（默认 HUP 信号死亡，lifecycle 收口，原断言逐字）；
//	  per-client = 0（零会话零收割形态：Shutdown exiting 位唤醒 pcSupervisor，
//	  last-reaped-code 缺省 0 → terminate(0)）。
//	末次 session_active——shared = false（子进程死亡翻转）；per-client = true
//	  恒（13-05 D-06 裁决，探活面不随关停/会话死亡翻转）。
func TestHealthzDraining(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			// srv 直调面收编（原 startShutdownServerWith 装配序列 = 本形态 wg
			// 包裹版）；waitHandlers 弃置——本组不断言 stderr 事件行，同步边
			// 不需要。
			exitCh, wsURL, _, srv := newHandleTestServer(t, mode, []string{"sh", "-c", "sleep 100"}, nil)
			base := httpBaseOf(wsURL)

			// Shutdown 前：200 ok 四字段（per-client 零会话形态 session_active
			// 恒 true——13-05 裁决的零会话证面）。
			code, hb := getHealthz(t, base+"/healthz")
			if code != http.StatusOK {
				t.Fatalf("Shutdown 前 GET /healthz status = %d, want %d", code, http.StatusOK)
			}
			assertHealthz(t, hb, "ok", 0, 32, true)

			// Shutdown 后：503 draining（draining 置位 = Shutdown 入口，先于 1001 广播
			// 与 stop-signal 序列——Shutdown 返回即确定性可见）。
			srv.Shutdown()
			code, hb = getHealthz(t, base+"/healthz")
			if code != http.StatusServiceUnavailable {
				t.Fatalf("Shutdown 后 GET /healthz status = %d, want %d (503 draining)", code, http.StatusServiceUnavailable)
			}
			if hb.Status != "draining" {
				t.Errorf("Shutdown 后 status = %q, want %q", hb.Status, "draining")
			}

			// 断言分叉表（D-02）：子进程终结收口码。shared = 默认 HUP 信号死亡
			// -1（原断言逐字）；per-client = 零会话零收割 last-reaped-code 缺省 0
			//（pcSupervisor 经 Shutdown 的 exiting 位唤醒，13-03）。
			wantExit := -1
			if mode == server.SessionModePerClient {
				wantExit = 0
			}
			waitExit(t, exitCh, wantExit)
			assertNoExit(t, exitCh)
			code, hb = getHealthz(t, base+"/healthz")
			if code != http.StatusServiceUnavailable || hb.Status != "draining" {
				t.Errorf("子进程死亡后 GET /healthz = %d/%q, want 503/draining（draining 不翻回）", code, hb.Status)
			}
			// 末次 session_active 分叉：shared = false（生命周期跟随翻转）；
			// per-client = true 恒（13-05 裁决）。
			wantActive := false
			if mode == server.SessionModePerClient {
				wantActive = true
			}
			assertHealthz(t, hb, "draining", 0, 32, wantActive)
		})
	}
}
