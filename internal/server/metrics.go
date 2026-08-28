package server

// metrics.go —— 08-04 OPS-07 /metrics 监控端点（D-01 手写 Prometheus text
// 0.0.4 exposition / D-02 聚合 gauge / D-03 基础 runtime / D-04 收发字节双指标 /
// D-05 会话口径+连接三件套 / D-06 认证计数器+build_info / D-08 认证闸跟随 /
// D-09 根路径固定）。
//
// D-01：手写 exposition，stdlib 零依赖（不引 prometheus/client_golang——指标集
// 小且全 gauge/counter，与 slog stdlib、stdlib mux 的 STACK.md 哲学一致）。
// 规范条款（CITED 官方 exposition_formats.md）：Content-Type 逐字
// text/plain; version=0.0.4; charset=utf-8；UTF-8 + \n 行尾；末行恒带换行；
// 每 series 按 HELP/TYPE/样本三行单组，TYPE 必须在首个样本之前；label 值转义
// \→\\、"→\"、\n→\\n（仅 build_info 的 version label 一个消费点）。
//
// label 红线（D-02/D-06，T-08-04a——日志红线 SEC-01 在 metrics 面的镜像）：
// 全部 series 零身份 label（remote/remote_user/client_id/ticket 永不进
// label——隐私 + 基数纪律）；outbox 深度为 max/sum 聚合 gauge（max 即慢客户端
// 检测的运维信号）；build_info 仅 version 单 label 且值过 escLabel 单侧定义
// 转义。per-IP/每连接明细查日志事件，metrics 只看总量与聚合。
//
// D-08/D-09：/metrics 跟随认证闸（凭据模式过 basicAuth——Prometheus
// scrape_config 原生 basic_auth 可采集；--no-auth 模式直通），根路径固定不带
// bp 前缀（采集器直连后端端口，路径恒定可写死进静态配置；拒绝双挂）；POST 405
// + Allow: GET 的 path-only fallback 不包认证（注册形态见 server.go Handler）。
//
// 采集兼容性说明（flagged_assumptions 登记）：无真实 Prometheus 实例验证——
// exposition 合法性以规范条款逐字断言代替（TestMetricsExposition 三行组序/
// 末行换行/Content-Type），真实 scrape 随 08-05 人工清单复核。

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
)

// metricsCounters 为 /metrics 的热路径计数器集（08-04 OPS-07，挂点注释见各
// 递增点）：atomic.Int64 承载——递增点在读循环/写端/认证中间件热路径
// （hubMu 外或持锁内均有），读取端（snapshotMetrics）亦需免锁形态，故全组
// atomic（inputDrops/droppedInputs 先例形态；与 registry.kicks 的 hubMu 内
// plain int 成场景化选型——那边计数与状态变更同锁原子是记账不变量，这边
// 是纯计数无状态关联）。
//
//   - authFailed/authThrottled（D-06）：basicAuth 401/429 站点与 WS Hello
//     ticket 核销失败站点递增（无 IP label——per-IP 明细查日志事件）；
//   - ptyOutputBytes/wsSentBytes/wsRecvBytes（D-04 双指标分开）：
//     pty_output = onChunk 入口 PTY 源单计；ws_sent = writer 成功 Write 后
//     （fan-out ×N 真实带宽）；ws_recv = Attach 读循环 Hello 首读 + 稳态循环
//     两站点（忠实「WS 网络流量」字面，RESEARCH A4）。ws_sent ÷ pty_output
//     即吞吐放大比。
type metricsCounters struct {
	authFailed     atomic.Int64
	authThrottled  atomic.Int64
	ptyOutputBytes atomic.Int64
	wsSentBytes    atomic.Int64
	wsRecvBytes    atomic.Int64
}

// metricsSnap 为一次采集的 registry 状态快照（字段全部在 snapshotMetrics 的
// 单趟 hubMu 持有内填齐——hubMu > outbox.mu 锁序 R-07，afterDrain 先例形态）。
type metricsSnap struct {
	clientsConnected int64 // registry.n（锁内读 atomic Load——双通道读均可，锁内 plain 化）
	clientsTotal     int64 // registry.clientsTotal（hubMu plain int，R-07 刻意选型）
	kicks            int64 // registry.kicks（同上）
	gateTransitions  int64 // registry.gateTransitions（同上）
	outboxMax        int   // 逐客户端 outbox.bytes 聚合 max（D-02 慢客户端信号）
	outboxSum        int   // 聚合 sum
	// 五枚 atomic 计数器的锁内 Load——hubMu 外读本合法（atomic 天性），纳入
	// 快照仅为一站式取数（与快照内 plain int 读取并存，场景化选型注释）。
	authFailed     int64
	authThrottled  int64
	ptyOutputBytes int64
	wsSentBytes    int64
	wsRecvBytes    int64
}

// snapshotMetrics 采集 registry 单趟快照。锁序 R-07 唯一合法形态（08-RESEARCH
// Pattern 3 逐字）：hubMu 一趟持锁内读 n/clientsTotal/kicks/gateTransitions +
// 遍历 registry.set 逐 outbox.mu 读 bytes 聚合 max/sum——绝不反序同持（反序
// 与 onChunk→trySend 构 ABBA 死锁，采集一发即全站 fan-out 冻结，T-08-04e；
// RESEARCH Pitfall 3）。kicks/gateTransitions/clientsTotal 保持 hubMu 内
// plain int 承载，绝不为读取改 atomic（为读改写的反向耦合破坏 R-07 选型）。
//
// 采集是低频路径（Prometheus 默认 15s 间隔），临界区 O(客户端数≤32) 纯读
// 微秒级——onChunk 每 chunk 持同锁做 fan-out，采集不构成可观测竞争。
func (s *Server) snapshotMetrics() metricsSnap {
	var sn metricsSnap
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	sn.clientsConnected = s.registry.n.Load()
	sn.clientsTotal = s.registry.clientsTotal
	sn.kicks = int64(s.registry.kicks)
	sn.gateTransitions = int64(s.registry.gateTransitions)
	for c := range s.registry.set {
		c.outbox.mu.Lock()
		d := c.outbox.bytes
		c.outbox.mu.Unlock()
		sn.outboxSum += d
		if d > sn.outboxMax {
			sn.outboxMax = d
		}
	}
	sn.authFailed = s.mc.authFailed.Load()
	sn.authThrottled = s.mc.authThrottled.Load()
	sn.ptyOutputBytes = s.mc.ptyOutputBytes.Load()
	sn.wsSentBytes = s.mc.wsSentBytes.Load()
	sn.wsRecvBytes = s.mc.wsRecvBytes.Load()
	return sn
}

// metricsHandler 为 GET /metrics 的处理函数：快照 → Content-Type → builder
// 逐 series 输出 17 条（契约清单与序见 metricsSeries17 测试侧镜像）→ 末行
// 恒 \n（builder 每行 \n 收尾——规范硬性要求）。runtime gauge 直采
// （NumGoroutine/ReadMemStats，D-03）；session_active 读 sessionAlive
// （08-03 字段，hubMu 外 atomic 读）；input 两计数器读既有 atomic 预埋挂点
// （server.go inputDrops / clients.go inputQ.droppedInputs，review #10）。
func (s *Server) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	snap := s.snapshotMetrics()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	var b strings.Builder
	writeGauge(&b, "wesh_clients_connected", "Currently attached WebSocket clients.", snap.clientsConnected)
	writeCounter(&b, "wesh_clients_total", "Total attached clients since process start.", snap.clientsTotal)
	writeCounter(&b, "wesh_clients_kicked_total", "Clients kicked with 1013 (slow consumer).", snap.kicks)
	// D-05：共享进程模型下会话数恒 1 退化——session_active gauge 落探活语义。
	var sessionActive int64
	if s.sessionAlive.Load() {
		sessionActive = 1
	}
	writeGauge(&b, "wesh_session_active", "Whether the PTY session is alive (1) or exited (0).", sessionActive)
	writeGauge(&b, "wesh_outbox_depth_bytes_max", "Maximum per-client outbox depth in bytes (aggregate over clients).", int64(snap.outboxMax))
	writeGauge(&b, "wesh_outbox_depth_bytes_sum", "Sum of per-client outbox depths in bytes (aggregate over clients).", int64(snap.outboxSum))
	writeCounter(&b, "wesh_pty_output_bytes_total", "Bytes read from the PTY master (fan-out source, counted once).", snap.ptyOutputBytes)
	writeCounter(&b, "wesh_ws_sent_bytes_total", "Bytes written to WebSocket clients (fan-out xN real bandwidth).", snap.wsSentBytes)
	writeCounter(&b, "wesh_ws_recv_bytes_total", "Bytes received from WebSocket clients (Hello first read and steady-state loop).", snap.wsRecvBytes)
	writeCounter(&b, "wesh_auth_failed_total", "Authentication failures (HTTP 401 and WS Hello ticket rejection).", snap.authFailed)
	writeCounter(&b, "wesh_auth_throttled_total", "Requests rejected by the throttle gate (HTTP 429).", snap.authThrottled)
	writeCounter(&b, "wesh_input_rate_dropped_total", "INPUT frames dropped by the per-client rate limiter.", s.inputDrops.Load())
	writeCounter(&b, "wesh_input_queue_dropped_total", "INPUT payloads dropped by the bounded session input queue.", s.inputQ.droppedInputs.Load())
	writeCounter(&b, "wesh_credit_gate_transitions_total", "Global credit gate open/close transitions.", snap.gateTransitions)
	writeGauge(&b, "wesh_goroutines", "Current goroutine count (leak observability).", int64(runtime.NumGoroutine()))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	writeGauge(&b, "wesh_mem_alloc_bytes", "Heap bytes allocated and in use (runtime.MemStats.Alloc).", int64(m.Alloc)) // Alloc==HeapAlloc，GOROOT mstats.go:58-61
	writeBuildInfo(&b, "wesh_build_info", "wesh build metadata.", s.version)
	fmt.Fprint(w, b.String())
}

// writeGauge/writeCounter 按规范写一 series 的三行组（HELP/TYPE/样本——
// 「All lines for a given metric must be provided as one single group, with the
// optional HELP and TYPE lines first」）；HELP 行每 metric 恒一条。
func writeGauge(b *strings.Builder, name, help string, v int64) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, v)
}

func writeCounter(b *strings.Builder, name, help string, v int64) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, v)
}

// writeBuildInfo 写 build_info 伪指标（gauge=1，version 单 label——官方
// naming.md「foobar_build_info」惯例）；label 值过 escLabel（T-08-04d：
// version 经 ldflags 注入（Phase 9 既定），理论可控，转义是 exposition 注入
// 的纵深防线——含引号/换行的注入值可伪造 series 行）。
func writeBuildInfo(b *strings.Builder, name, help, version string) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s gauge\n%s{version=\"%s\"} 1\n", name, help, name, name, escLabel(version))
}

// escLabel 转义 exposition label 值（规范逐字：\→\\、"→\"、\n→\\n）。
// 反斜杠先行，顺序敏感——先转义换行会把换行转义产物（两字符 \n）里的
// 反斜杠再翻倍（TestBuildInfo 表驱动判别行锁定）。单侧定义：build_info 的
// version label 是唯一消费点（Don't Hand-Roll——禁止逐站点 ad-hoc 转义）。
func escLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
