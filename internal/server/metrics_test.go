package server_test

// metrics_test.go —— 08-04 OPS-07 /metrics 端点行为锁（D-01 手写 Prometheus
// text 0.0.4 exposition / D-02·D-06 零身份 label 红线 / D-04 收发字节双指标 /
// D-05 会话口径+连接三件套 / D-08 认证闸跟随 / D-09 根路径固定）：
//   - TestMetricsExposition：Content-Type 逐字、HELP/TYPE/样本三行组序、末行
//     \n 收尾、17 series 在场与契约序、基线值（connected 0→1 / session_active==1 /
//     goroutines>0 / mem_alloc>0 / build_info{version="dev"} 1）；
//   - TestMetricsAuth：凭据两态（无/错凭据 401 与 / 同链同文 → 正确凭据 200
//     同形态）、无认证直通、bp=/wesh 下 /metrics 200 与 /wesh/metrics 精确码、
//     POST 405+Allow:GET（两模式同文——fallback 不包认证）；
//   - TestBuildInfo：默认 dev 兜底 + 自定义 version + escLabel 转义表驱动
//    （引号/反斜杠/换行——反斜杠先行顺序敏感，T-08-04d exposition 注入防线）
//     经 exposition build_info 行逐字断言。
//   - TestMetricsValues / TestMetricsSnapshotRace：Task 2 数值正确性与 -race
//     快照竞态压力。
//
// http 客户端用 net/http 直发（health_test.go httpBaseOf/getStatus 同款形态）。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// metricsSeries17 为 17 条 series 的契约清单（must_haves truths 序逐字——
// series 命名/类型/组织序是采集契约，D-01 costly 级暴露面一次性锁死）。
var metricsSeries17 = []struct{ name, typ string }{
	{"wesh_clients_connected", "gauge"},
	{"wesh_clients_total", "counter"},
	{"wesh_clients_kicked_total", "counter"},
	{"wesh_session_active", "gauge"},
	{"wesh_outbox_depth_bytes_max", "gauge"},
	{"wesh_outbox_depth_bytes_sum", "gauge"},
	{"wesh_pty_output_bytes_total", "counter"},
	{"wesh_ws_sent_bytes_total", "counter"},
	{"wesh_ws_recv_bytes_total", "counter"},
	{"wesh_auth_failed_total", "counter"},
	{"wesh_auth_throttled_total", "counter"},
	{"wesh_input_rate_dropped_total", "counter"},
	{"wesh_input_queue_dropped_total", "counter"},
	{"wesh_credit_gate_transitions_total", "counter"},
	{"wesh_goroutines", "gauge"},
	{"wesh_mem_alloc_bytes", "gauge"},
	{"wesh_build_info", "gauge"},
}

// getMetrics GET 指定 URL，断言 200 + Content-Type 逐字
// text/plain; version=0.0.4; charset=utf-8（D-01 规范条款逐字），读尽 body 返回。
func getMetrics(t *testing.T, url string) string {
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", url, resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("GET %s Content-Type = %q, want 逐字 %q（D-01）", url, ct, "text/plain; version=0.0.4; charset=utf-8")
	}
	return string(b)
}

// reqMetrics 发 GET（可携 Basic 凭据），返回状态码/Allow 头/body——认证两态、
// 同文对照与 405 断言的裸通道（getMetrics 的 200 专用断言不适用负场景）。
func reqMetrics(t *testing.T, url, user, pass string) (status int, allow, body string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET %s read body: %v", url, err)
	}
	return resp.StatusCode, resp.Header.Get("Allow"), string(b)
}

// assertExpositionShape 锁 exposition 结构形态：末行 \n 收尾（规范硬性要求）、
// 恰 17×3 行、每 series 按 # HELP/# TYPE/样本三行组序且 TYPE 行逐字
// （「All lines for a given metric must be provided as one single group, with the
// optional HELP and TYPE lines first」规范条款的分组形态锁）。
func assertExpositionShape(t *testing.T, body string) {
	t.Helper()
	if !strings.HasSuffix(body, "\n") {
		t.Fatalf("exposition 末行未以 \\n 收尾（规范硬性要求）: 尾部 %q", body[len(body)-8:])
	}
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) != 3*len(metricsSeries17) {
		t.Fatalf("exposition 行数 = %d, want %d（17 series × HELP/TYPE/样本三行组）", len(lines), 3*len(metricsSeries17))
	}
	for i, sr := range metricsSeries17 {
		help, typ, sample := lines[3*i], lines[3*i+1], lines[3*i+2]
		if !strings.HasPrefix(help, "# HELP "+sr.name+" ") {
			t.Errorf("series #%d HELP 行形态错: %q（want 前缀 %q）", i, help, "# HELP "+sr.name+" ")
		}
		if want := "# TYPE " + sr.name + " " + sr.typ; typ != want {
			t.Errorf("series #%d TYPE 行 = %q, want 逐字 %q", i, typ, want)
		}
		if !strings.HasPrefix(sample, sr.name+" ") && !strings.HasPrefix(sample, sr.name+"{") {
			t.Errorf("series #%d 样本行形态错: %q（want 以 series 名起行）", i, sample)
		}
	}
}

// metricSample 在 exposition body 中精确查找 series 样本行并解析整数值
// （行形态 = name + 空格 + 整数值；name+空格前缀保证精确名匹配不撞前缀族。
// label 形态行（build_info）不适用本 helper——逐字断言代替）。
func metricSample(t *testing.T, body, name string) (int64, bool) {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, name+" ") {
			v, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, name)), 10, 64)
			if err != nil {
				t.Fatalf("series %q 样本值非整数: %q: %v", name, line, err)
			}
			return v, true
		}
	}
	return 0, false
}

// TestMetricsExposition（08-04 OPS-07，D-01/D-03/D-05）：无认证实例 exposition
// 全形态 + 基线值——attach 前 connected==0、attach 后 1；session_active==1；
// goroutines/mem_alloc>0；build_info{version="dev"} 1（Options.Version 零值兜底）。
func TestMetricsExposition(t *testing.T) {
	_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
	base := httpBaseOf(wsURL)

	body := getMetrics(t, base+"/metrics")
	assertExpositionShape(t, body)
	if v, ok := metricSample(t, body, "wesh_clients_connected"); !ok || v != 0 {
		t.Fatalf("wesh_clients_connected = %d (present=%v), want 0（attach 前）", v, ok)
	}
	if v, _ := metricSample(t, body, "wesh_session_active"); v != 1 {
		t.Errorf("wesh_session_active = %d, want 1（会话存活）", v)
	}
	if v, _ := metricSample(t, body, "wesh_goroutines"); v <= 0 {
		t.Errorf("wesh_goroutines = %d, want >0（D-03 runtime gauge）", v)
	}
	if v, _ := metricSample(t, body, "wesh_mem_alloc_bytes"); v <= 0 {
		t.Errorf("wesh_mem_alloc_bytes = %d, want >0（D-03 runtime gauge）", v)
	}
	if !strings.Contains(body, "wesh_build_info{version=\"dev\"} 1\n") {
		t.Errorf("build_info 行缺席或形态错——want 逐字 %q 行", `wesh_build_info{version="dev"} 1`)
	}

	// attach 后 connected 翻 1（registry.n 数据源端到端）。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "") }()

	body = getMetrics(t, base+"/metrics")
	assertExpositionShape(t, body)
	if v, _ := metricSample(t, body, "wesh_clients_connected"); v != 1 {
		t.Errorf("attach 后 wesh_clients_connected = %d, want 1", v)
	}
}

// TestMetricsAuth（08-04 OPS-07，D-08/D-09）：/metrics 认证闸两态 + bp 根路径
// 固定 + 405 成对。爬梯 sleep 驱动 pacing（ThrottleBase 50ms 覆写，auth_e2e
// 镜像形态——连续失败请求的退避窗口逐次过窗）。
func TestMetricsAuth(t *testing.T) {
	// 凭据两态（D-08）：无/错凭据 401（与 / 同链同文——同一 basicAuth 链）→
	// 正确凭据 200 同形态。
	t.Run("credential_gate", func(t *testing.T) {
		cred, err := server.ParseCredential("mx-op:mx-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:     true,
			Credentials:  []server.Credential{cred},
			ThrottleBase: 50 * time.Millisecond,
		})
		base := httpBaseOf(wsURL)

		// #0 GET / 无凭据 → 401 基线 body（同链同文对照面；fails=1，窗口 +50ms）。
		st, _, bodyRoot := reqMetrics(t, base+"/", "", "")
		if st != http.StatusUnauthorized {
			t.Fatalf("#0 GET / status = %d, want %d（整站 Basic 闸基线）", st, http.StatusUnauthorized)
		}
		time.Sleep(100 * time.Millisecond) // 过窗（fail#1 窗口 = 1×base = 50ms）

		// #1 /metrics 无凭据 → 401 同文（fails=2，窗口 +100ms）。
		st, _, bodyNo := reqMetrics(t, base+"/metrics", "", "")
		if st != http.StatusUnauthorized {
			t.Fatalf("#1 无凭据 GET /metrics status = %d, want %d（D-08 认证闸跟随）", st, http.StatusUnauthorized)
		}
		if bodyNo != bodyRoot {
			t.Fatalf("#1 无凭据 401 body 与 GET / 不同文——/metrics 必须与整站同链同文:\nmetrics: %q\nroot:    %q", bodyNo, bodyRoot)
		}
		time.Sleep(150 * time.Millisecond) // 过窗（fail#2 窗口 = 2×base = 100ms）

		// #2 /metrics 错凭据 → 401 同文（fails=3，窗口 +200ms；无枚举 oracle）。
		st, _, bodyWrong := reqMetrics(t, base+"/metrics", "mx-op", "wrong-pass")
		if st != http.StatusUnauthorized {
			t.Fatalf("#2 错凭据 GET /metrics status = %d, want %d", st, http.StatusUnauthorized)
		}
		if bodyWrong != bodyRoot {
			t.Fatalf("#2 错凭据 401 body 与无凭据/根路径不同文（枚举 oracle 面）:\nwrong: %q\nroot:  %q", bodyWrong, bodyRoot)
		}
		time.Sleep(250 * time.Millisecond) // 过窗（fail#3 窗口 = 3×base = 150ms→200ms 余量）

		// #3 正确凭据 → 200 同形态（recordSuccess 清零；Prometheus basic_auth 可采集面）。
		st, _, body := reqMetrics(t, base+"/metrics", "mx-op", "mx-pass")
		if st != http.StatusOK {
			t.Fatalf("#3 正确凭据 GET /metrics status = %d, want %d", st, http.StatusOK)
		}
		assertExpositionShape(t, body)
	})

	// 无认证直通（D-08 后半句：--no-auth 模式自然免）。
	t.Run("noauth_passthrough", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
		assertExpositionShape(t, getMetrics(t, httpBaseOf(wsURL)+"/metrics"))
	})

	// bp 固定（D-09）：bp=/wesh 实例下 /metrics 仍 200、/wesh/metrics 不可达
	// （无认证 404 / 凭据 401）——根路径固定，拒绝双挂（采集路径可写死进
	// Prometheus 静态配置）。
	t.Run("basepath_pinned", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, BasePath: "/wesh"})
		base := httpBaseOf(wsURL)
		assertExpositionShape(t, getMetrics(t, base+"/metrics"))
		if st := getStatus(t, base+"/wesh/metrics"); st != http.StatusNotFound {
			t.Errorf("无认证 bp 实例 GET /wesh/metrics status = %d, want %d（拒绝双挂）", st, http.StatusNotFound)
		}

		cred, err := server.ParseCredential("mx-bp:mx-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		_, wsURLCred := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			BasePath:    "/wesh",
			Credentials: []server.Credential{cred},
		})
		baseCred := httpBaseOf(wsURLCred)
		st, _, _ := reqMetrics(t, baseCred+"/metrics", "mx-bp", "mx-pass")
		if st != http.StatusOK {
			t.Errorf("凭据 bp 实例正确凭据 GET /metrics status = %d, want %d（D-09 根路径固定）", st, http.StatusOK)
		}
		if st := getStatus(t, baseCred+"/wesh/metrics"); st != http.StatusUnauthorized {
			t.Errorf("凭据 bp 实例无头 GET /wesh/metrics status = %d, want %d（bp 子树经 Basic 闸）", st, http.StatusUnauthorized)
		}
	})

	// 405：POST /metrics → 405 + Allow: GET，两模式同文（path-only fallback 显式
	// 注册——否则 POST 落进 "/" 子树静态伺服，RESEARCH Pitfall 7）；凭据模式
	// 无凭据 POST 同样 405——fallback 不包认证（与 /api/attach 405 先例同形态）。
	t.Run("method_405", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
		base := httpBaseOf(wsURL)
		resp, err := http.Post(base+"/metrics", "application/octet-stream", nil)
		if err != nil {
			t.Fatalf("POST /metrics: %v", err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("无认证 POST /metrics status = %d, want %d (405)", resp.StatusCode, http.StatusMethodNotAllowed)
		}
		if allow := resp.Header.Get("Allow"); allow != http.MethodGet {
			t.Errorf("无认证 POST /metrics Allow = %q, want %q", allow, http.MethodGet)
		}

		cred, err := server.ParseCredential("mx-405:mx-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		_, wsURLCred := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:    true,
			Credentials: []server.Credential{cred},
		})
		resp2, err := http.Post(httpBaseOf(wsURLCred)+"/metrics", "application/octet-stream", nil)
		if err != nil {
			t.Fatalf("凭据实例 POST /metrics: %v", err)
		}
		defer resp2.Body.Close()
		_, _ = io.Copy(io.Discard, resp2.Body)
		if resp2.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("凭据实例无凭据 POST /metrics status = %d, want %d（405 fallback 不包认证——先于 Basic 闸命中）", resp2.StatusCode, http.StatusMethodNotAllowed)
		}
		if allow := resp2.Header.Get("Allow"); allow != http.MethodGet {
			t.Errorf("凭据实例 POST /metrics Allow = %q, want %q", allow, http.MethodGet)
		}
	})
}

// TestBuildInfo（08-04 OPS-07，D-06 + T-08-04d）：wesh_build_info{version} 单
// label gauge=1——默认 "dev" 兜底（Options.Version 零值）；自定义 version 直传；
// escLabel 三字符转义表驱动（反斜杠先行顺序敏感——version 由发布构建 ldflags
// 注入（Phase 9 既定），理论可控，escLabel 是 exposition 注入的纵深防线）。
func TestBuildInfo(t *testing.T) {
	// 默认兜底：Options 未携 Version → dev（测试直构造形态即生产 dev 构建形态）。
	t.Run("default_dev", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
		body := getMetrics(t, httpBaseOf(wsURL)+"/metrics")
		if !strings.Contains(body, "wesh_build_info{version=\"dev\"} 1\n") {
			t.Fatalf("默认 build_info 行缺席或形态错——want 逐字 %q 行", `wesh_build_info{version="dev"} 1`)
		}
	})

	// 自定义 version 直传（Options.Version 单一通道）。
	t.Run("custom", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, Version: "1.2.3-rc"})
		body := getMetrics(t, httpBaseOf(wsURL)+"/metrics")
		if !strings.Contains(body, "wesh_build_info{version=\"1.2.3-rc\"} 1\n") {
			t.Fatalf("自定义 build_info 行缺席或形态错——want version=\"1.2.3-rc\"")
		}
	})

	// escLabel 表驱动（规范逐字：\→\\、"→\"、\n→\\n，反斜杠先行）——经真实
	// 实例 exposition 行逐字断言（Options → s.version → escLabel → 文本全链）。
	t.Run("escape", func(t *testing.T) {
		rows := []struct{ in, want string }{
			{`v"q`, `v\"q`},             // 引号 → \"
			{`a\b`, `a\\b`},             // 反斜杠 → 翻倍
			{"a\nb", `a\nb`},            // 真换行 → 两字符 \n（顺序敏感判别行：反斜杠先行则换行转义产物不再被翻倍）
			{"x\"\\\n", "x\\\"\\\\\\n"}, // 复合：引号+反斜杠+真换行
		}
		for _, row := range rows {
			_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, Version: row.in})
			body := getMetrics(t, httpBaseOf(wsURL)+"/metrics")
			wantLine := "wesh_build_info{version=\"" + row.want + "\"} 1"
			if !strings.Contains(body, wantLine+"\n") {
				t.Errorf("version=%q：build_info 行 = want 逐字 %q，exposition 中缺席", row.in, wantLine)
			}
		}
	})
}

// readUntilMarker 读 OUTPUT 帧直至载荷含 marker（/bin/cat 回显证据——双端
// 各自读到才证明 fan-out 两路写出完成：ws_sent 计数在 writer 成功 Write 后
// 递增，客户端可读严格后于计数，断言时序由此确定化）。
func readUntilMarker(t *testing.T, c *websocket.Conn, ctx context.Context, marker string) {
	t.Helper()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read until %q: %v", marker, err)
		}
		if len(data) > 0 && data[0] == proto.Output && strings.Contains(string(data[1:]), marker) {
			return
		}
	}
}

// TestMetricsValues（08-04 OPS-07，D-02/D-04/D-05/D-06）：计数器数值正确性
// 黑盒锁——连接三件套/字节三件套/踢出/认证两计数器，与 T-08-04a 零身份 label
// 红线反断言（exposition 全文无 remote 值串）。
func TestMetricsValues(t *testing.T) {
	// 字节三件套 + 连接双口径（D-04/D-05）：两客户端 attach 后 /bin/cat 行回显
	// 驱动——connected==2、clients_total==2；pty_output>0、ws_recv>0；
	// ws_sent ≥ 2×pty_output（fan-out ×N 语义：两端窗口期下行总量不少于
	// 2×源——两 series 相除即吞吐放大比，ROADMAP SC2 成功准则）。
	t.Run("bytes_and_clients", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
		base := httpBaseOf(wsURL)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c1, _ := dialHello(t, ctx, wsURL, 80, 24)
		defer c1.CloseNow()
		c2, _ := dialHello(t, ctx, wsURL, 80, 24)
		defer c2.CloseNow()

		// 两端均在册后驱 INPUT（/bin/cat canonical 模式行回显）——全部 onChunk
		// chunk 必然扇出至两端（pre-attach 输出在 cat 下为零，无单边窗口）。
		frame := append([]byte{proto.Input}, "metrics-echo-line\n"...)
		if err := c1.Write(ctx, websocket.MessageBinary, frame); err != nil {
			t.Fatalf("write INPUT: %v", err)
		}
		readUntilMarker(t, c1, ctx, "metrics-echo-line")
		readUntilMarker(t, c2, ctx, "metrics-echo-line")

		body := getMetrics(t, base+"/metrics")
		if v, _ := metricSample(t, body, "wesh_clients_connected"); v != 2 {
			t.Errorf("wesh_clients_connected = %d, want 2", v)
		}
		if v, _ := metricSample(t, body, "wesh_clients_total"); v != 2 {
			t.Errorf("wesh_clients_total = %d, want 2（累计 counter 只增不减）", v)
		}
		ptyOut, _ := metricSample(t, body, "wesh_pty_output_bytes_total")
		if ptyOut <= 0 {
			t.Errorf("wesh_pty_output_bytes_total = %d, want >0（onChunk 入口 PTY 源单计）", ptyOut)
		}
		if v, _ := metricSample(t, body, "wesh_ws_recv_bytes_total"); v <= 0 {
			t.Errorf("wesh_ws_recv_bytes_total = %d, want >0（Hello 首读 + 稳态循环两站点）", v)
		}
		sent, _ := metricSample(t, body, "wesh_ws_sent_bytes_total")
		if sent < 2*ptyOut {
			t.Errorf("wesh_ws_sent_bytes_total = %d, want >= 2×wesh_pty_output_bytes_total(%d)（fan-out ×2 放大比）", sent, ptyOut)
		}
		// T-08-04a 红线反断言：exposition 全文无 remote 值串（身份永不进
		// label——per-IP 明细查日志事件，metrics 只看总量与聚合）。
		if strings.Contains(body, "127.0.0.1") {
			t.Errorf("exposition 混入客户端身份串 127.0.0.1（T-08-04a 零身份 label 红线）")
		}
	})

	// 踢出计数（D-05 三件套之三）：复用 slowclient_test.go stall 夹具驱一次
	// 1013 踢出——kicked==1 且 connected==1（stall 端移除、正常端在册）。
	// 夹具纪律与洪水量级论证见 slowclient_test.go 文件头（38.9MB > 单连接
	// 最坏吸收 ~10MiB + 64KiB outbox）。
	t.Run("kick_counter", func(t *testing.T) {
		exitCh, wsURL := startTestServerWith(t, []string{"seq", "1", "5000000"}, server.Options{
			Writable:    true,
			WritePolicy: "all",
			OutboxBytes: 64 * 1024,
		})
		_ = exitCh // 本测试不断言子进程退出（同 TestSlowConsumerKick）
		base := httpBaseOf(wsURL)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		stall, _ := dialHello(t, ctx, wsURL, 80, 24)
		normal, _ := dialHello(t, ctx, wsURL, 80, 24)
		stall.SetReadLimit(4 * 1024 * 1024)
		normal.SetReadLimit(4 * 1024 * 1024)

		var normalBytes atomic.Int64
		go func() {
			for {
				_, data, err := normal.Read(context.Background())
				if err != nil {
					return
				}
				if len(data) > 0 && data[0] == proto.Output {
					normalBytes.Add(int64(len(data) - 1))
				}
			}
		}()

		// 等正常端累积超 12MiB——此时 stall 端管道+outbox 必然已满、踢出已触发。
		deadline := time.Now().Add(15 * time.Second)
		for normalBytes.Load() < 12*1024*1024 {
			if time.Now().After(deadline) {
				t.Fatalf("normal client received %d bytes in 15s, want >= 12MiB (flood not flowing)", normalBytes.Load())
			}
			time.Sleep(50 * time.Millisecond)
		}
		assertKicked1013(t, stall, 10*time.Second, "stall client")

		// 注册表移除在踢出判定时刻同步完成（先于客户端观测关闭帧）——此刻
		// 采集必然见 kicked==1 / connected==1 终态。
		body := getMetrics(t, base+"/metrics")
		if v, _ := metricSample(t, body, "wesh_clients_kicked_total"); v != 1 {
			t.Errorf("wesh_clients_kicked_total = %d, want 1（1013 踢出计数）", v)
		}
		if v, _ := metricSample(t, body, "wesh_clients_connected"); v != 1 {
			t.Errorf("踢出后 wesh_clients_connected = %d, want 1（stall 端已移除，正常端在册）", v)
		}
		stall.CloseNow()
		normal.CloseNow()
	})

	// 认证两计数器（D-06）：错凭据 401 → auth_failed+1；爬梯窗口内再错 → 429
	// → auth_throttled+1（爬梯 pacing 形态镜像 auth_e2e）；WS 侧 Hello 非法
	// ticket 核销失败 → auth_failed 再 +1（两站点同计数器汇聚——HTTP 401 与
	// WS 1008 同语义同 series，无 IP label）。
	t.Run("auth_counters", func(t *testing.T) {
		cred, err := server.ParseCredential("mx-cnt:mx-pass")
		if err != nil {
			t.Fatalf("ParseCredential: %v", err)
		}
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:     true,
			Credentials:  []server.Credential{cred},
			ThrottleBase: 50 * time.Millisecond,
		})
		base := httpBaseOf(wsURL)

		// #1 错凭据 → 401（auth_failed+1；fails=1，窗口 +50ms）。
		st, _, _ := reqMetrics(t, base+"/metrics", "mx-cnt", "wrong-pass")
		if st != http.StatusUnauthorized {
			t.Fatalf("#1 错凭据 GET /metrics status = %d, want %d", st, http.StatusUnauthorized)
		}
		// #2 窗口内立即错凭据 → 429（auth_throttled+1；429 短路不 recordFail
		// 不延长窗口——窗口内第 1 次请求即命中节流）。
		st, _, _ = reqMetrics(t, base+"/metrics", "mx-cnt", "wrong-pass")
		if st != http.StatusTooManyRequests {
			t.Fatalf("#2 窗口内错凭据 GET /metrics status = %d, want %d (429)", st, http.StatusTooManyRequests)
		}
		time.Sleep(100 * time.Millisecond) // 过窗（fail#1 窗口 = 1×base = 50ms）

		// #3 正确凭据 → 200：HTTP 侧两计数器各 ==1（本实例私有，序列确定）。
		st, _, body := reqMetrics(t, base+"/metrics", "mx-cnt", "mx-pass")
		if st != http.StatusOK {
			t.Fatalf("#3 正确凭据 GET /metrics status = %d, want %d", st, http.StatusOK)
		}
		if v, _ := metricSample(t, body, "wesh_auth_failed_total"); v != 1 {
			t.Fatalf("wesh_auth_failed_total = %d, want 1（HTTP 401 站点）", v)
		}
		if v, _ := metricSample(t, body, "wesh_auth_throttled_total"); v != 1 {
			t.Fatalf("wesh_auth_throttled_total = %d, want 1（HTTP 429 站点）", v)
		}

		// WS 侧站点：Hello 携非法 ticket → 核销失败 auth_failed+1（server.go
		// Attach 事件点同址递增；wire 形态由 dialHelloTicketWantAuthFailed
		// 锁定：Error{auth_failed} + 1008）。fails=2，窗口 +100ms。
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		dialHelloTicketWantAuthFailed(t, ctx, wsURL, "AAAAAAAAAAAAAAAAAAAAAA")
		time.Sleep(150 * time.Millisecond) // 过窗（fail#2 窗口 = 2×base = 100ms）

		st, _, body = reqMetrics(t, base+"/metrics", "mx-cnt", "mx-pass")
		if st != http.StatusOK {
			t.Fatalf("#4 正确凭据 GET /metrics status = %d, want %d", st, http.StatusOK)
		}
		if v, _ := metricSample(t, body, "wesh_auth_failed_total"); v != 2 {
			t.Errorf("wesh_auth_failed_total = %d, want 2（HTTP 401 + WS Hello 核销失败两站点汇聚）", v)
		}
		if v, _ := metricSample(t, body, "wesh_auth_throttled_total"); v != 1 {
			t.Errorf("wesh_auth_throttled_total = %d, want 1（429 短路不追加）", v)
		}
	})
}

// TestMetricsSnapshotRace（08-04 OPS-07，T-08-04e）：-race 快照竞态压力——
// goroutine 组并发 attach/close 注册表搅动 + 主循环连续 GET /metrics 采集，
// 与计数器 atomic 递增（onChunk/writer/读循环热路径）并发交错。锁序纪律：
// snapshotMetrics 单快照 hubMu > outbox.mu（R-07，afterDrain 先例形态），
// 反序同持即 ABBA 死锁（采集一发全站 fan-out 冻结）；数据竞争/死锁即红。
// 死锁探测：压力窗口后终态 GET 必须限时可达（hubMu 楔死则超时 FAIL）。
func TestMetricsSnapshotRace(t *testing.T) {
	_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})
	base := httpBaseOf(wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
				if err != nil {
					return // ctx 到期——压力窗口正常结束
				}
				payload, err := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24})
				if err != nil {
					c.CloseNow()
					return
				}
				if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, payload...)); err != nil {
					c.CloseNow()
					return
				}
				if _, _, err := c.Read(ctx); err != nil { // Welcome
					c.CloseNow()
					return
				}
				c.CloseNow()
			}
		}()
	}
	// 主循环连续采集（与注册表搅动/计数器递增并发交错——-race 受力面）。
	client := &http.Client{Timeout: 5 * time.Second}
	for ctx.Err() == nil {
		resp, err := client.Get(base + "/metrics")
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	wg.Wait()

	// 终态可达性断言：ABBA 死锁下 hubMu 永久楔死，本请求超时即 FAIL。
	resp, err := client.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("压力窗口后 GET /metrics 不可达（死锁嫌疑）: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("压力窗口后 GET /metrics status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
