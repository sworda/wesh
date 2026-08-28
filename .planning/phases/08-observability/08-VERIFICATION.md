---
phase: 08-observability
verified: 2026-08-28T06:27:12Z
status: human_needed
score: 24/24 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 24/24
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "A1 真实 Prometheus scrape 兼容性：按 README「运维（Phase 8）→ 指标」scrape_configs 配方（basic_auth 与 wesh 凭据同组）配置真实 Prometheus，等两个 scrape 周期后查 Status → Targets 与 Graph"
    expected: "target UP；wesh_clients_connected / wesh_pty_output_bytes_total / wesh_build_info{version} 等 17 条 series 全部可查询；无 parse error。负面对照（可选）：填错 basic_auth → target down + wesh 日志出 throttled 事件"
    why_human: "无真实 Prometheus 实例（08-RESEARCH Environment Availability 登记）；自动化等价面已绿——exposition 规范条款逐字断言（TestMetricsExposition）+ phase08.mjs S2/S3 真实二进制全链断言 + 本验证者独立冒烟（17 series 逐名枚举/CT 逐字/末行 0x0a）"
  - test: "A2 journald 实机 ingest 与 jq 检索：systemd 部署 wesh，制造一次认证失败与一次 attach/detach，执行 journalctl -u wesh -o cat | jq -c 'select(.event==\"auth_failed\")' 与 'select(.client_id==N)'"
    expected: "auth_failed 事件行可检索（无 user/username 键）；同一 client_id 的 attach 与 detach 各一条（reason=normal）；journald 不截断不转义 JSON 行"
    why_human: "依赖 systemd/journald 实机环境；自动化等价面已绿——phase08.mjs S5 + 本验证者真实二进制冒烟（stderr JSON 行逐字观测，auth_failed 全文零用户名串）"
  - test: "A3 draining 窗口编排观测率：systemd 部署下 systemctl restart wesh，同时 0.2s 周期轮询 curl /healthz，观测重启窗口内状态码序列"
    expected: "窗口内可观测 503（status=draining）出现在 200 与连接拒绝（000）之间；默认 stop-signal HUP 无 timeout 时窗口亚秒级属预期（探活周期匹配问题，非缺陷）；需确定性摘流则配 --stop-timeout 拉宽"
    why_human: "依赖真实编排/init 系统的重启时序；自动化等价面已绿——phase08.mjs S4（trap+3s 窗口确定性观测 503）+ TestHealthzDraining + 本验证者独立冒烟（trap HUP + --stop-timeout 3s，SIGTERM 后 0.6s 实测 503 draining 恰四键，进程 255 退出）"
---

# Phase 8: 可观测性 Verification Report

**Phase Goal:** ttyd 缺失的可运维性补齐——健康检查（OPS-06 /healthz）、指标（OPS-07 /metrics）、JSON 结构化审计日志（OPS-08）
**Verified:** 2026-08-28T06:27:12Z
**Status:** human_needed
**Re-verification:** 二次独立验证。目录内存在前件（2026-08-28T04:48:33Z，human_needed 24/24，无 gaps 区）——按 Step 0 规则无 gaps 即 INITIAL MODE，本验证**未采信前件任何结论**，全部检查（fingerprint grep / 代码通读 / 真实二进制冒烟 / 命名测试与全量套件 -race / UAT 复跑）由本验证者独立重执行。

## Goal Achievement

### Observable Truths

ROADMAP 三条 Success Criteria 为合同主线（SC1/SC2/SC3），下挂合并后的 plan must_have 真理（已去重）。每条证据均为本验证者当次独立产出。

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | /healthz 返回服务健康状态，可用于反代/编排探活 | ✓ VERIFIED | 独立冒烟 + TestHealthz 族 -race + UAT S1/S4（分解如下） |
| 1 | GET /healthz 免认证 200 + 四字段 JSON（status/clients/max_clients/session_active）；凭据模式无 Authorization 头同 200，对照 GET / 仍 401（D-07 例外不蔓延） | ✓ VERIFIED | 独立冒烟实测：no-auth 200 body 逐字 `{"status":"ok","clients":0,"max_clients":32,"session_active":true}`（CT application/json）；凭据模式（--credential alice:secret123）无头 200、GET / 401；TestHealthz 五子测 -race PASS；phase08.mjs S1 复跑 PASS |
| 2 | 优雅关停中 /healthz 503 + status="draining"（D-11，Shutdown 入口置位唯一） | ✓ VERIFIED | 独立冒烟实测：trap 忽略 HUP + --stop-timeout 3s 实例 SIGTERM 后 0.6s curl 得 503 `{"status":"draining","clients":0,"max_clients":32,"session_active":true}`（恰四键，进程 255 退出）；server.go:1391 `draining.Store(true)` 唯一置位点（grep==1）；TestHealthzDraining -race PASS；UAT S4a/S4b PASS |
| 3 | /healthz 根路径固定（D-09）：bp=/wesh 下 /healthz 200、/wesh/healthz 404/401，拒绝双挂 | ✓ VERIFIED | 独立冒烟实测：bp=/wesh 实例根 /healthz 200、/wesh/healthz 404、/wesh/metrics 404；server.go:538-539 注册不带 bp 前缀；UAT S1c PASS |
| 4 | POST /healthz 405 + Allow: GET（方法模式 + path-only fallback 成对） | ✓ VERIFIED | 独立冒烟实测：POST → `HTTP/1.1 405 Method Not Allowed` + `Allow: GET`（响应头 dump 观测，securityHeaders 自动继承同帧可见）；UAT S1d PASS |
| 5 | session_active 数据源 = sessionAlive atomic.Bool（New 置 true，lifecycle 置 false） | ✓ VERIFIED | server.go:412 Store(true)（New 尾部 goroutine 启动前）/ server.go:1291 Store(false)（session_end 同区段）；health.go:44 读取端；TestHealthz 翻车子测 -race PASS |
| SC2 | /metrics 暴露连接数、会话数、收发字节数、每客户端 outbox 深度与踢出计数 | ✓ VERIFIED | 独立冒烟 + TestMetrics 族 -race + UAT S2/S3（分解如下） |
| 6 | 手写 Prometheus text 0.0.4 exposition（D-01）：Content-Type 逐字、HELP/TYPE/样本三行组、末行换行；stdlib 零依赖 | ✓ VERIFIED | 独立冒烟实测 CT 逐字 `text/plain; version=0.0.4; charset=utf-8`、末字节 0x0a（xxd 实测）；metrics.go:152-166 三行组 writer；go.mod/go.sum 零 prometheus/zap/logrus 依赖（grep==0），go.mod 最近变更为 07-06（本 phase 未触碰）；TestMetricsExposition -race PASS |
| 7 | 17 series 全量（连接三件套/session_active/outbox max·sum/字节三/auth 两/input 两/gate/goroutines/mem/build_info） | ✓ VERIFIED | 独立冒烟逐名枚举 17 条 `# HELP` 与 plan 契约清单逐字一致（wesh_clients_connected…wesh_build_info）；metrics.go:122-145 逐 series 接线真实数据源；UAT S3a PASS |
| 8 | 零身份 label（D-02/D-06）；build_info 仅 version 单 label 且 escLabel 转义（反斜杠先行） | ✓ VERIFIED | 独立冒烟 exposition 全文无任何 identity label（唯一 label = build_info version）；metrics.go:172-177 escLabel 单侧定义（反斜杠先行三字符）；TestBuildInfo 三子测 -race PASS |
| 9 | /metrics 认证闸跟随（D-08）+ 根路径固定（D-09）+ POST 405 | ✓ VERIFIED | 独立冒烟实测：凭据模式无/错凭据 401（同窗口内后续请求 429——throttle 文档化行为，RESEARCH Pitfall 6 形态，与 UAT S5a 一致）/ 过窗后正确凭据 200；bp 下 /metrics 200 根固定；UAT S2a/S2b/S2c/S3c PASS |
| 10 | 数值正确性：connected/total、ws_sent ≥ pty_output 放大比、kicked 计数、auth 两计数器汇聚；快照锁序 hubMu > outbox.mu -race 干净 | ✓ VERIFIED | TestMetricsValues 三子测（bytes_and_clients/kick_counter/auth_counters）+ TestMetricsSnapshotRace 本验证者 -race 复跑 PASS（9.1s 命名集）；metrics.go:87-110 单趟快照锁序与注释一致；UAT S3a（connected==2/total==2/sent≥pty）PASS |
| 11 | version 经 Options.Version 单一通道（main var version 直传，零值兜底 "dev"） | ✓ VERIFIED | 独立冒烟实测 `wesh_build_info{version="dev"} 1`；cmd/wesh/main.go `Version: version` ×1（grep==1）；server.go:486/527 注册两分支；UAT S3b PASS |
| SC3 | 日志为 JSON 结构化输出（slog），认证失败/连接建立断开/会话生命周期可检索；无凭据；用户可控字段剥离控制字符 | ✓ VERIFIED | 独立冒烟 + log/events 测试族 -race + UAT S5/S6（分解如下） |
| 12 | 运行期事件全部经 slog JSONHandler 输出单行 JSON：msg 恒 "event"、event 独立字段、time/level slog 默认键、恒 INFO（D-13/D-15/D-18） | ✓ VERIFIED | 独立冒烟 stderr 行逐字观测：`{"time":"2026-08-28T14:21:16.489039393+08:00","level":"INFO","msg":"event","event":"session_start","pid":1678036}`——RFC3339Nano+本地时区（WR-01 修正后形态与 log.go:11-14 注释一致）；log.go:58 JSONHandler + log.go:65-67 emitEvent；TestLogEventJSON -race PASS |
| 13 | logEvent 签名不变单出口迁移（D-13 原子切换，零双轨）：server.go 旧 Fprintf 实现清零 | ✓ VERIFIED | `grep -c Fprintf server.go`==0、`func logEvent` 仅 log.go:93 一处、非测试调用点 15 处（server.go 10 + auth.go 1 + clients.go 4）全走 slog |
| 14 | captureStderr 测试通道不失明（stderrW 动态 writer 调用时解析 os.Stderr） | ✓ VERIFIED | log.go:48-54 stderrW.Write 每次读 os.Stderr 变量 + stderrMu 竞态防护（c4a8eed 门禁面）；TestLogEventJSON（经 captureStderr 捕获断言）-race PASS——若 writer 静态捕获则该测试结构性失明 |
| 15 | 启动行/分享链接行（含 token）与 wesh: warning: 警告行保持人读文本（D-14/D-16） | ✓ VERIFIED | 独立冒烟实测 stdout `listening on http://127.0.0.1:PORT` + `share read-only: http://...s/<token>/` 纯文本；全部 UAT 脚本启动行解析消费者零适配通过（phase05/07/08 复跑均绿） |
| 16 | 事件目录全量（D-17）：auth_failed/throttled + attach/detach + session_start/session_end/shutdown + exit_when_empty 族 | ✓ VERIFIED | 独立冒烟观测 session_start/auth_failed/throttled/shutdown/session_end 真实 JSON 行（v2.err 全文）；server.go:409(session_start)/1394(shutdown)、clients.go:527/736(emitDetachLocked 两调用点)、auth.go:125-130(throttled 携 retry_after)；events_test.go 六测试 -race 全 PASS |
| 17 | attach/detach 携 client_id 关联（D-20）；detach 单事件 reason 四值（D-21），kick/pong_timeout 独立行零残留 | ✓ VERIFIED | TestAttachDetachEvents + TestDetachReason 四子测 -race PASS；UAT S5d（attach=1 detach=1 client_id 相等 reason=normal）；phase05.mjs S6 终态复跑 28/28（detach reason=kick code=1013）；phase05-dom.mjs D5 迁移修复在场（L417-419 parseEvents 字段断言）复跑 19/19 |
| 18 | detach reason 跨 goroutine 传递 -race 干净（pinger 取 hubMu 写 pongTimedOut、detach 同锁读） | ✓ VERIFIED | TestDetachReason/pong_timeout 子测在本验证者 -race 复跑下 PASS；clients.go:122 字段 + server.go pinger 置位 + clients.go:731 区同锁读 |
| 19 | session_end 字段 = exit_code + signal（仅信号死亡出键）+ duration_seconds（D-22）；exitSignalNum 单侧定义 | ✓ VERIFIED | 独立冒烟观测 `session_end{exit_code:-1,duration_seconds:4.53…,signal:"SIGHUP"}`（kill 实例真实产出）；TestSessionEnd 两子测（exit 42 / SIGHUP）-race PASS；UAT S5e（exit_code==42、dur>0、无 signal 键、进程 42 退出）PASS；server.go `func exitSignalNum` 单一定义 |
| 20 | throttled 携 retry_after（与 Retry-After 头同值，D-23）；auth_failed 不含用户名（SEC-01 红线） | ✓ VERIFIED | 独立冒烟实测 throttled 行 `"retry_after":1`、auth_failed 行逐字无 user/username 键且全文 grep `nosuchuser7f3a`==0；auth.go:125 retry_after attr；TestThrottledRetryAfter + TestAuthFailedNoUsername -race PASS；UAT S5b/S5c PASS |
| 21 | 用户可控字段剥离控制字符（D-19）：remote（XFF 链首）与 remote_user 过 sanitizeRemoteUser，双闸并存 | ✓ VERIFIED | proxy.go:126 `sanitizeRemoteUser(p.clientIP(r))` ×1 + IN-02 分叉边界注释（L119）；TestRemoteSanitize -race PASS；UAT S6a/S6b/S6c PASS（NEL 线形探针 remote_user=alice、remote 逐码点无 C0/C1/DEL、对照组无键且非空集防假绿） |
| 22 | 断言面迁移完成：5 Go 测试文件 + UAT 脚本 JSON 字段断言（parseEvents），凭据红线负断言子串形态逐字保留 | ✓ VERIFIED | log_test.go parseEvents/countByEvent 在场；limits/emptyexit/auth_e2e/proxy_e2e/multi 五文件迁移（全量套件 -race 五包绿 59s 本验证者复跑）；`b64Wrong`×3（≥2）、`roTok`×5（≥2）红线负断言逐字保留；`reason=` 在 *_test.go 仅存于 events_test.go 新 helper 注释/Fatalf 格式串（非旧文本行断言形态） |
| 23 | 零新 CLI flag、零新外部依赖（D-15/D-01 哲学） | ✓ VERIFIED | main.go flag 区无 --log-format/--log-level（仅 log.go 注释提及 D-15 决策本身）；go.mod/go.sum 本 phase 零提交（最近变更 07-06 TOML）；prometheus/zap/logrus grep==0 |
| 24 | 全量回归绿：go test -race 五包 + 既有 UAT 脚本 | ✓ VERIFIED | 本验证者独立复跑：`go test -race -count=1 ./...` 五包全 ok（server 58.5s）；`go vet ./...` 净；GOROOT gofmt -l internal/ cmd/ 输出为空；phase08.mjs 21/21、phase05.mjs 28/28、phase07.mjs 34/34、phase05-dom.mjs 19/19 复跑全绿 |

**Score:** 24/24 truths verified（0 present-but-behavior-unverified——全部行为依赖真理均有行为测试或真实二进制冒烟证据，且全部由本验证者当次独立执行）

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/server/log.go` | stderrW 动态 writer + eventLog 单例 + emitEvent + logEvent（slog 实现） | ✓ VERIFIED | 103 行实质实现（通读）；`slog.NewJSONHandler`×1、os.Stderr 动态解析 + stderrMu、`func logEvent`×1；WR-01 修正后注释（RFC3339Nano+本地时区）在场 |
| `internal/server/log_test.go` | parseEvents/countByEvent + TestLogEventJSON | ✓ VERIFIED | 120 行；两符号在场；TestLogEventJSON -race PASS |
| `internal/server/events_test.go` | attach/detach/session/auth 六测试 | ✓ VERIFIED | 644 行；六测试函数在场，本验证者 -race 复跑全 PASS |
| `internal/server/health.go` | healthzHandler（draining 分支 + 状态 JSON） | ✓ VERIFIED | 54 行实质实现（通读）；json.Marshal 四字段、draining 分支、零 basicAuth 引用、`, _ = w.Write` 显式丢弃 |
| `internal/server/health_test.go` | TestHealthz 五子测 + TestHealthzDraining | ✓ VERIFIED | 281 行；两函数在场；-race PASS |
| `internal/server/metrics.go` | metricsHandler + snapshotMetrics + writer 三件套 + escLabel | ✓ VERIFIED | 177 行实质实现（通读）；17 series 契约序、锁序注释与实现一致；IN-01 修正（metrics.go:146 `_, _ = fmt.Fprint`）在场 |
| `internal/server/metrics_test.go` | 五测试（Exposition/Auth/BuildInfo/Values/SnapshotRace） | ✓ VERIFIED | 607 行；五函数在场；-race PASS |
| `internal/server/server.go` | 注册/字段/打点接线 | ✓ VERIFIED | `"GET /healthz"`×1 + fallback×1（538-539）、`"GET /metrics"`×2（486 凭据 basicAuth 包装 + 527 无认证直通）+ fallback×1（548）、sessionAlive×4、draining.Store(true)×1（1391）、`&s.mc`×3、session_start emit×1（409）、shutdown emit×1（1394） |
| `internal/server/clients.go` | pongTimedOut/emitDetachLocked/字节挂点 | ✓ VERIFIED | emitDetachLocked 两调用点（527 kick/736 detach）+ 定义（762）；ptyOutputBytes/wsSentBytes 挂点（writer 成功 Write 后） |
| `internal/server/auth.go` | basicAuth 五参 + retry_after + 计数器站点 | ✓ VERIFIED | `mc *metricsCounters`×1、retry_after attr（125）、emitEvent throttled（130） |
| `internal/server/proxy.go` | remote() trust 分支 sanitize 推广 | ✓ VERIFIED | `sanitizeRemoteUser(p.clientIP(r))`×1（L126）+ IN-02 分叉边界注释（L119） |
| `cmd/wesh/main.go` | Options.Version 透传 | ✓ VERIFIED | `Version: version`×1 |
| `web/uat/phase08.mjs` | 六场景 UAT + parseEvents + PHASE08_ONLY | ✓ VERIFIED | 619 行；复跑 21/21 PASS（含 SEC 输出自净） |
| `web/uat/phase05-dom.mjs` | D5 踢出检测 detach reason=kick 迁移（08-05 回归捕获修复） | ✓ VERIFIED | L417-419 parseEvents 字段断言在场；复跑 19/19 PASS |
| `README.md` | 「## 运维（Phase 8）」节三小节 | ✓ VERIFIED | 节标题×1、三小节（378/392/433）、`唯一例外`×2、`basic_auth`×2、`select(.event`×1、IN-03 修复（448-449 auth_failed/throttled 行标 remote_user 可选键） |
| `08-UAT.md` | 三人工复核项 | ✓ VERIFIED | A1/A2/A3 在场（canonical Tests 区 pending×3，机器可解析） |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| logEvent 15 调用点 | eventLog → stderrW → os.Stderr | 调用时解析 os.Stderr（动态 writer） | ✓ WIRED | TestLogEventJSON 经 captureStderr 捕获断言 -race PASS——捕获语义的行为证据 |
| pinger（pong_timeout 置位） | detach() reason 读取 | hubMu 同步边（cl.pongTimedOut） | ✓ WIRED | TestDetachReason/pong_timeout -race PASS |
| lifecycle sess.Wait / Shutdown 入口 | detach reason=shutdown 判定 | s.exiting + closeBroadcastCode（1000/1001 同源） | ✓ WIRED | TestDetachReason/shutdown 两形态 -race PASS |
| exitMessage 信号提取 | session_end signal 字段 | exitSignalNum 单侧定义共用 | ✓ WIRED | TestExitSignalNum + TestSessionEnd -race PASS；冒烟 SIGHUP 实例 signal 键实测 |
| SIGTERM → Shutdown() | healthzHandler draining 分支 | Shutdown 首行 draining.Store(true) | ✓ WIRED | 独立冒烟 SIGTERM→503 实测；TestHealthzDraining -race PASS |
| lifecycle sess.Wait 返回 | session_active 字段 | sessionAlive.Store(false)（session_end 同区段） | ✓ WIRED | server.go:1291；TestHealthz 翻车子测 -race PASS |
| onChunk/writer/Attach 读循环 | 字节三 counter | atomic.Int64 热路径递增 | ✓ WIRED | TestMetricsValues 放大比断言 -race PASS；UAT S3a sent≥pty |
| basicAuth 401/429 + Hello 核销失败 | auth_failed/auth_throttled counter | 三调用点同传 &s.mc | ✓ WIRED | server.go:486 注册点实测 401/429/200 序列；TestMetricsValues/auth_counters 精确值锁 -race PASS |
| main.go var version | wesh_build_info{version} | Options.Version → s.version → escLabel | ✓ WIRED | 冒烟实测 build_info{version="dev"} 1 |
| phase08.mjs S4 SIGTERM | /healthz 503 draining | trap 忽略 HUP + --stop-timeout 3s 拉宽窗口 | ✓ WIRED | UAT S4a/S4b 复跑 PASS；独立冒烟同法实测 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| healthzHandler | clients | registry.n.Load() | 是（冒烟 clients:0 实测；dialHello 前后 0→1 行为锁在 TestHealthz） | ✓ FLOWING |
| healthzHandler | max_clients | s.maxClients（New 装配固化） | 是（默认 32 冒烟实测） | ✓ FLOWING |
| healthzHandler | session_active | sessionAlive atomic（lifecycle 翻转） | 是（翻车子测 + 冒烟 true 实测） | ✓ FLOWING |
| healthzHandler | status | draining atomic（Shutdown 置位） | 是（SIGTERM 实测 503 draining） | ✓ FLOWING |
| metricsHandler | 17 series 全部 | snapshotMetrics（registry 锁内快照 + atomic 计数器 + runtime 直采） | 是（数值测试 + 冒烟 17 series 与 build_info 实测） | ✓ FLOWING |
| JSON 事件流 | remote/code/reason/client_id/exit_code/pid/retry_after 等 | 真实 emit 点（Attach/detach/lifecycle/Shutdown/basicAuth/pinger/clients） | 是（独立冒烟真实事件行观测：pid 1678036、exit_code -1、retry_after 1 等真实值） | ✓ FLOWING |

无 STATIC/DISCONNECTED/HOLLOW_PROP 项。

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| /healthz 200 四字段 + CT + 405 + Allow | 真实二进制 --port 0 + curl | body 逐字四键 `{"status":"ok","clients":0,"max_clients":32,"session_active":true}`；POST 405 + Allow: GET | ✓ PASS |
| 凭据模式 /healthz 免认证 + 例外不蔓延 | curl（无头 /healthz、GET /） | 200 / 401 | ✓ PASS |
| /metrics 17 series 逐名 + CT + 末行换行 + build_info | curl + grep/xxd | 17 HELP 行与契约清单逐字一致；CT 逐字；末字节 0x0a；version="dev" | ✓ PASS |
| /metrics 认证闸两态 + bp 固定 | curl（无凭据/错凭据/正确凭据；bp 实例） | 401（同窗 429 文档化节流）/ 200；bp 下 /wesh/metrics 404 | ✓ PASS |
| SIGTERM → 503 draining + 进程退出 | trap HUP + --stop-timeout 3s + kill -TERM + curl | 0.6s 后 503 `status:"draining"` 恰四键；255 退出 | ✓ PASS |
| JSON 事件 schema（msg/event/level/time） | stderr 行观测 | `{"time":…+08:00,"level":"INFO","msg":"event","event":"session_start","pid":1678036}` | ✓ PASS |
| auth_failed 无用户名 + throttled 携 retry_after | 错凭据 curl + grep | 事件行零 `nosuchuser7f3a` 串；throttled 行 `"retry_after":1` | ✓ PASS |
| 启动行/分享链接行人读文本保持 | stdout 观测 | `listening on …` + `share read-only: …/s/<token>/` 纯文本 | ✓ PASS |
| 16 个 phase-08 命名测试 -race | `go test -race -run 'TestLogEventJSON\|…\|TestMetricsSnapshotRace'` | 全 PASS（9.1s） | ✓ PASS |
| 全量套件 -race | `go test -race -count=1 ./...` | 五包全 ok（server 58.5s） | ✓ PASS |
| phase08.mjs 六场景 | `node web/uat/phase08.mjs` | 21/21 PASS（含 SEC 自净） | ✓ PASS |
| 回归三脚本 | phase05 / phase07 / phase05-dom | 28/28（1 skipped 豁免）/ 34/34（1 skipped 豁免）/ 19/19 | ✓ PASS |

### Probe Execution

SKIPPED——本 phase 无 `scripts/*/tests/probe-*.sh` 形态探针（项目无 scripts/ 目录；验证面为 Go 测试 + web/uat/*.mjs 协议脚本，已在 Behavioral Spot-Checks 全部独立执行）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| OPS-06 | 08-03, 08-05 | /healthz 健康检查端点 | ✓ SATISFIED | health.go + TestHealthz 族 + 独立冒烟（200 四字段/405/draining 503/bp 固定/免认证例外）+ UAT S1/S4；REQUIREMENTS.md L67 已勾选、Traceability L152 Complete |
| OPS-07 | 08-04, 08-05 | /metrics 监控端点（连接数、会话数、收发字节数） | ✓ SATISFIED | metrics.go 17 series + TestMetrics 族 + 独立冒烟（17 逐名/CT/认证两态/build_info）+ UAT S2/S3；REQUIREMENTS.md L68 已勾选、Traceability L153 Complete |
| OPS-08 | 08-01, 08-02, 08-05 | 结构化日志（JSON），含审计事件（认证失败、连接建立/断开、会话生命周期） | ✓ SATISFIED | log.go + 事件目录 + events_test 族 + 独立冒烟（五类事件行 + 无用户名 + retry_after）+ UAT S5/S6；REQUIREMENTS.md L69 已勾选、Traceability L154 Complete |

**Orphaned requirements:** 无——REQUIREMENTS.md Traceability 表 Phase 8 行恰为 OPS-06/07/08 三条，全部被 plan `requirements:` 字段认领（08-01: OPS-08 / 08-02: OPS-08 / 08-03: OPS-06 / 08-04: OPS-07 / 08-05: OPS-06+07+08）。

### Prohibitions 复核

| 来源 | Statement | Tier | Status | Evidence |
|------|-----------|------|--------|----------|
| 08-01 P1 | 凭据/ticket/token/Authorization 任何形态不进 JSON 事件流 | test | ✓ VERIFIED | TestAuthFailedNoUsername -race PASS + 独立冒烟 auth_failed 行零用户名串 + UAT S5b + assertOutputClean 自净 |
| 08-01 P2 | 启动行/分享链接行 token 不迁入 JSON 结构化字段 | judgment | ✓ VERIFIED（直接观测） | 独立冒烟实测：share 链接行（含 token）为 stdout 纯文本；全部 JSON 事件行零 token 字段——判断级条目获直接证据支撑 |
| 08-01 P3 | 运行期事件无双轨输出 | test | ✓ VERIFIED | server.go Fprintf==0、logEvent 唯一出口（log.go:93）、冒烟 stderr 全 JSON 行无文本事件行 |
| 08-02 P1 | auth_failed 不含用户名 | test | ✓ VERIFIED | logEvent 四参签名结构性无用户名通道 + TestAuthFailedNoUsername -race PASS + 冒烟负断言 |
| 08-03 P1 | /healthz body 不含版本/身份/错误细节 | test | ✓ VERIFIED | 键集白名单测试（恰四键，200/503 两态）+ 冒烟 200/503 两态 body 逐字观测 |
| 08-04 P1 | metrics label 零身份（remote/remote_user/client_id/ticket） | test | ✓ VERIFIED | 冒烟 exposition 全文零身份 label（唯一 label=build_info version 经 escLabel）+ TestMetricsValues 反断言 |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| README.md | 336 | 陈旧示例：`wesh: close remote=… reason=exit_when_empty remote_user=alice` 为 D-13 迁移前的文本事件行格式，与迁移后实际 JSON 输出（README:438 自身示例即正确形态）矛盾 | ⚠️ Warning | 文档一致性缺口（Phase 7 节未被 08-01 迁移同步更新）；不影响行为面与任何 must-have 真理（README 真理字面仅覆盖「运维（Phase 8）」节，该节内容经核验全部正确）；前件验证与 code review 均未捕获——建议以文档修复跟进（将示例改为 JSON 事件行形态），不阻塞本 phase 状态判定 |
| 其余新/改文件 | - | TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER/占位词 | 无 | log.go/health.go/metrics.go/三个新测试文件/phase08.mjs grep 全空；全部字段真实接线 |

### Human Verification Required

以下 3 项来自 08-UAT.md（canonical Tests 区 pending×3），均为运维栈实机复核——自动化等价面已全绿，属项目 CODEBUDDY.md 测试策略第 5 层「平台原生行为显式豁免」范畴，不阻塞但需人工勾选闭环：

### 1. A1 真实 Prometheus scrape 兼容性

**Test:** 按 README「运维（Phase 8）→ 指标」scrape_configs 配方（basic_auth 与 wesh 凭据同组）配置真实 Prometheus，等两个 scrape 周期后查 Status → Targets 与 Graph
**Expected:** target UP；17 条 series 全部可查询可见；无 parse error
**Why human:** 无真实 Prometheus 实例；自动化等价面已绿（exposition 规范逐字断言 + phase08.mjs S2/S3 + 本验证者独立冒烟 17 series 逐名枚举）

### 2. A2 journald 实机 ingest 与 jq 检索

**Test:** systemd 部署下制造认证失败与 attach/detach，执行 `journalctl -u wesh -o cat | jq -c 'select(.event=="auth_failed")'` 与 `select(.client_id==N)'`
**Expected:** auth_failed 可检索（无 user/username 键）；同 client_id 的 attach/detach 各一条；journald 不截断不转义 JSON 行
**Why human:** 依赖 systemd/journald 实机环境；自动化等价面已绿（phase08.mjs S5 + 本验证者冒烟 stderr JSON 行逐字观测）

### 3. A3 draining 窗口编排观测率

**Test:** systemd 部署下 `systemctl restart wesh`，同时 0.2s 周期轮询 `curl /healthz`，观测重启窗口状态码序列
**Expected:** 窗口内可观测 503 draining（200 → 503 → 000）；默认配置窗口亚秒级属预期（探活周期匹配问题）；需确定性摘流则配 --stop-timeout
**Why human:** 依赖真实编排/init 系统重启时序；自动化等价面已绿（phase08.mjs S4 + TestHealthzDraining + 本验证者 SIGTERM 实测 503）

### Gaps Summary

无差距。24/24 must-have 真理全部经本验证者独立产出的代码证据 + 行为证据（命名测试 -race 复跑 + 真实二进制冒烟 + UAT 复跑）验证；OPS-06/07/08 三需求全部达成；CONTEXT D-01..D-23 决策抽查全部兑现；REVIEW 四项发现（WR-01/IN-01/IN-02/IN-03）在 HEAD 提交 0ae4c0f 的修复经本验证逐条代码级复核（log.go:11-14 RFC3339Nano 注释、metrics.go:146 显式丢弃、proxy.go:119 分叉边界注释、README:448-449 remote_user 可选键）。新增一项前件未捕获的 ⚠️ Warning（README:336 陈旧文本格式示例），已列入 Anti-Patterns 表建议文档跟进，不构成 must-have 失败。状态为 human_needed 仅因 08-UAT.md 三条运维栈实机复核项待人工执行——其自动化等价面已全部验证通过。

---

_Verified: 2026-08-28T06:27:12Z_
_Verifier: Claude (gsd-verifier)_
