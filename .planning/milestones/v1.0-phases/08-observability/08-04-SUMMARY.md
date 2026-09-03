---
phase: 08-observability
plan: 04
subsystem: infra
tags: [metrics, prometheus-exposition, ops-endpoint, observability, go-stdlib, atomic-counters]

requires:
  - phase: 08-observability
    plan: 03
    provides: /healthz 注册接线点（Handler() 认证两分支之外唯一注册区 + 405 成对先例）+ sessionAlive/draining atomic.Bool（wesh_session_active 数据源）+ startTestServerWith/httpBaseOf/getStatus 测试三件套
provides:
  - GET /metrics 手写 Prometheus text 0.0.4 exposition（D-01）：Content-Type 逐字 text/plain; version=0.0.4; charset=utf-8、UTF-8 \n 行尾、末行恒带换行、每 series HELP/TYPE/样本三行单组——stdlib 零依赖，go.mod/go.sum 逐字节不动
  - 17 series 契约序全量（D-02..D-06）：连接三件套（connected gauge / total counter / kicked counter）+ session_active gauge + outbox max/sum 聚合 gauge + 字节三件套（pty_output 源单计 / ws_sent fan-out ×N / ws_recv 双站点）+ auth 两计数器 + input 两计数器 + credit gate transitions + goroutines/mem runtime + build_info{version} gauge=1
  - label 红线（D-02/D-06，T-08-04a）：全 series 零身份 label；build_info 仅 version 单 label 且值过 escLabel 单侧定义转义（反斜杠先行三字符，顺序敏感）
  - 认证闸跟随（D-08）：凭据模式 basicAuth 包装（401 与 / 同链同文、429 同 store），--no-auth 直通；根路径固定不受 bp 影响（D-09）；POST 405+Allow:GET fallback 不包认证
  - version 经 Options.Version 单一通道（main var version 原样透传，零值兜底 "dev"；ldflags 注入属 Phase 9）
  - basicAuth 新签名五参形态（末参 mc *metricsCounters，三调用点同改传 &s.mc）；registry.clientsTotal int64（hubMu 保护，registerLocked 唯一加点）
affects: [08-05 UAT/README 收口（phase08.mjs metrics 场景的 Go 侧行为锁即本 plan；Prometheus basic_auth 配方与 Pitfall 6 明示义务的数据源）, Phase 9 发布构建 ldflags 注入的 plumbing 通道]

actuals:
  tokens: 13295
  tasks: 2
  commits: 5

tech-stack:
  added: []
  patterns:
    - "手写 text 0.0.4 exposition 三件套：snapshotMetrics（hubMu 一趟内逐 outbox.mu 读深度，R-07 afterDrain 先例逐字形态——atomic 计数器锁内 Load 与 plain int 并存）+ writeGauge/writeCounter/writeBuildInfo 三行组 writer + escLabel 单侧定义转义"
    - "热路径计数器 atomic 选型：metricsCounters 五枚 atomic.Int64（递增点在 onChunk/writer/读循环/认证中间件，hubMu 外或持锁内均有）——与 registry.kicks/clientsTotal 的 hubMu plain int 成场景化选型（纯计数无状态关联 → atomic；计数与状态变更同锁原子 → plain）"
    - "运维端点注册三区制：凭据分支 basicAuth 包装（D-08 跟随）/ 无认证分支直通 / 两分支之外 path-only 405 fallback 不包认证（POST 两模式同文）——/healthz 免认证例外与 /metrics 认证跟随在同区并存"

key-files:
  created:
    - internal/server/metrics.go
    - internal/server/metrics_test.go
  modified:
    - internal/server/server.go
    - internal/server/clients.go
    - internal/server/auth.go
    - cmd/wesh/main.go

key-decisions:
  - "metricsCounters 全五字段 + Server.mc + 快照计数器读取在 Task 1 一次落地（plan 字面『Task 1 两 auth 字段 / Task 2 扩五字段+Server.mc』的任务边界调整）——避免 tracer 提交携带五个硬编码 0 占位 series（tracer 纪律：production-quality，绝不 throwaway）；最终态与 plan must_haves 逐字一致"
  - "TestMetricsExposition 取黑盒形态（真实实例 HTTP GET）而非 plan 字面的白盒直调 metricsHandler——metrics_test.go 与全部既有测试同为 package server_test 外部包，未导出 handler 不可直调；黑盒经真实注册路径额外锁定接线（08-03 TestHealthz 同形态先例）"
  - "ws_sent ≥ 2×pty_output 放大比断言的可判定性论证：/bin/cat 零 pre-attach 输出 + 两端在册后才驱 INPUT ⇒ 全部 chunk 恒扇出至两端；ws_sent 计帧含类型字节与 Welcome 帧使严格大于成立"
  - "auth 计数器精确值锁（==1/==2）强于 plan 的 ≥1——实例私有 + 请求序列确定（http.Client 无重试面），WS 侧站点经既有 dialHelloTicketWantAuthFailed 复用驱动"

patterns-established:
  - "exposition 结构锁三件套：metricsSeries17 契约清单（命名/类型/序一次锁死）+ assertExpositionShape（51 行恰好 + 三行组序 + TYPE 逐字）+ metricSample（name+空格前缀精确行匹配解析整数值）"
  - "计数器数值黑盒锁形态：stall 夹具驱踢出（slowclient_test.go 复用）+ 爬梯 pacing 驱 429（auth_e2e 镜像）+ dialHelloTicketWantAuthFailed 驱 WS 侧站点——认证计数器两站点汇聚精确计数"

requirements-completed: [OPS-07]

coverage:
  - id: D1
    description: "/metrics exposition 全形态（D-01/D-03/D-05）：Content-Type 逐字、17 series 契约序三行组、末行换行、基线值（connected 0→1 / session_active==1 / goroutines>0 / mem_alloc>0 / build_info dev）；认证两态 + bp 固定 + 405（D-08/D-09）；escLabel 表驱动转义锁（T-08-04d）"
    requirement: OPS-07
    verification:
      - kind: unit
        ref: "internal/server/metrics_test.go#TestMetricsExposition + #TestMetricsAuth（四子测）+ #TestBuildInfo（三子测，-race 全绿）"
        status: pass
      - kind: integration
        ref: "真实二进制冒烟：curl /metrics 首行 `# HELP wesh_clients_connected`、17 series、末行 0x0a；凭据模式 401/429/200 两态 + POST 405 Allow:GET + auth 两计数器真实递增"
        status: pass
    human_judgment: false
  - id: D2
    description: "计数器数值正确性（D-04/D-05/D-06）：双客户端回显驱动 connected/total==2 + pty_output>0/ws_recv>0/ws_sent ≥ 2×pty_output 放大比；stall 夹具驱踢出 kicked==1/connected==1；401→429→WS 非法 ticket 序列 auth_failed==2/auth_throttled==1 两站点汇聚；T-08-04a exposition 全文无身份串反断言"
    requirement: OPS-07
    verification:
      - kind: unit
        ref: "internal/server/metrics_test.go#TestMetricsValues（三子测 -race 全绿）"
        status: pass
    human_judgment: false
  - id: D3
    description: "快照锁序 -race 压力（T-08-04e）：并发 attach/close × 连续采集 2s 数据竞争检测 + 终态 GET 限时可达死锁探测；全仓 -race 绿 + 既有 UAT 八脚本 132 断言零回归（basicAuth 签名扩展间接受影响面）"
    requirement: OPS-07
    verification:
      - kind: unit
        ref: "internal/server/metrics_test.go#TestMetricsSnapshotRace（-race 绿）；go test -race -count=1 ./... 五包绿（server 58.3s）"
        status: pass
      - kind: e2e
        ref: "node web/uat/{phase02,phase03,phase04,phase05,phase06,phase07,phase07-b2,phase07-b3}.mjs 全退出 0（12/12、18/18、10/10、28/28、23/23、34/34、4/4、3/3）"
        status: pass
    human_judgment: false

duration: 48min
completed: 2026-08-28
status: complete
---

# Phase 8 Plan 04: /metrics 监控端点 Summary

**OPS-07 全行为落地：metrics.go 新文件装手写 Prometheus text 0.0.4 exposition 三件套（snapshotMetrics 锁序 R-07 单趟快照 + 三行组 writer + escLabel 单侧定义转义），17 series 契约序覆盖连接三件套/会话存活/字节双指标/outbox 聚合/踢出/认证/runtime/build_info；Handler() 三区注册（凭据 basicAuth 跟随 / 无认证直通 / 405 fallback 不包认证）+ Options.Version 单一通道 plumbing；basicAuth 五参扩展三调用点同改；全仓 -race 绿 + 八 UAT 脚本 132 断言零回归 + go.mod 零漂移**

## Performance

- **Duration:** 48 min
- **Started:** 2026-08-28T02:03:47Z
- **Completed:** 2026-08-28T02:51:41Z
- **Tasks:** 2/2
- **Files modified:** 6（2 新建 + 4 修改，与 plan files_modified 清单逐一对应）

## Accomplishments

- **D-01 手写 exposition 端到端**：metricsHandler 快照 → Content-Type 逐字 → builder 逐 series 三行组 ×17 → 末行恒 \n；真实二进制冒烟首行 `# HELP wesh_clients_connected`、17 series 名契约序、尾字节 0x0a——任意 text 0.0.4 解析器可消费
- **17 series 全量数据源接线（D-02..D-06）**：连接三件套（connected=registry.n / total=新 registry.clientsTotal 只增不减 / kicked=registry.kicks）+ session_active=sessionAlive（08-03 字段复用）+ outbox max/sum 聚合（hubMu 一趟内逐 outbox.mu）+ 字节三件套（onChunk 入口 PTY 源单计 / writer 成功 Write 后 fan-out ×N / Attach Hello 首读+稳态循环两站点上行）+ auth 两计数器（basicAuth 401/429 站点 + WS Hello 核销失败站点三处汇聚，无 IP label）+ input 两预埋计数器 + gateTransitions + goroutines/mem runtime + build_info{version}
- **D-08/D-09 认证与挂载**：凭据模式无/错凭据 401 与 GET / 逐字节同文（爬梯 pacing 锁定）、正确凭据 200 同形态；bp=/wesh 下 /metrics 200 而 /wesh/metrics 无认证 404/凭据 401；POST 405+Allow:GET 两模式同文且 fallback 不包认证（凭据实例无凭据 POST 仍 405 的行为锁）
- **T-08-04d exposition 注入防线**：escLabel 三字符转义反斜杠先行（顺序敏感判别行锁定——真换行输入得两字符 \n 而非反斜杠翻倍产物），经 Options→s.version→escLabel→exposition 全链逐字行断言
- **数值正确性行为锁**：ws_sent ≥ 2×pty_output 放大比可观测（ROADMAP SC2 准则）；踢出后 kicked==1/connected==1；auth_failed HTTP+WS 两站点汇聚 ==2、auth_throttled==1（429 短路不追加）
- **T-08-04e 快照竞态**：并发 attach/close ×4 × 连续采集 2s -race 干净 + 终态 GET 限时可达（ABBA 死锁探测）

## Task Commits

Each task was committed atomically（TDD RED→GREEN 每任务两提交）:

1. **Task 1 RED: /metrics 失败测试先行（exposition/认证闸/build_info）** - `99acc3b` (test)
2. **Task 1 GREEN (tracer): metrics.go exposition 三件套 + 三区注册 + Version plumbing + clientsTotal** - `e2939b5` (feat)
3. **Task 2 RED: 计数器数值与快照竞态失败测试先行** - `06d5650` (test)
4. **Task 2 GREEN: 字节/auth 计数器挂点 + basicAuth 五参扩展** - `557104d` (feat)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP/REQUIREMENTS）

_Tracer feedback gate（autonomous，08-01/08-02/08-03 同款）：Task 1 提交后 verify 端到端重跑通过（TestMetrics*+TestHealthz -race PASS / go vet OK / go.mod 零漂移），方进入 Task 2 扩展面。_

## TDD Gate Compliance

两任务 tdd="true" 均按 RED→GREEN 两提交序列执行：
- Task 1 RED（99acc3b）为**编译失败形态**——TestBuildInfo 引用尚未存在的 `Options.Version` 字段，包构建失败即预期失败态（Go TDD 对新 API 面测试的标准 RED 形态）；GREEN（e2939b5）落地后全绿
- Task 2 RED（06d5650）为**运行失败形态**——bytes_and_clients/auth_counters 两子测失败（五计数器递增点未挂接）；kick_counter 先行绿（kicks 系 Phase 5 预埋挂点、clientsTotal 系 Task 1 已接线，属既有行为回归而非新兑现）；TestMetricsSnapshotRace 为回归锁（与递增挂点同批提交，-race 受力面在 GREEN 后真实存在）
- REFACTOR 门：无需要（GREEN 后代码即终态，gofmt 净）

## Files Created/Modified

- `internal/server/metrics.go`（新建，177 行）— D-01/D-02/D-03/D-04/D-05/D-06/D-08/D-09 注释头 + label 红线登记；metricsCounters 五枚 atomic.Int64（递增点场景化选型注释）；metricsSnap + snapshotMetrics（hubMu 一趟内逐 outbox.mu，T-08-04e/Pitfall 3 防线注释）；metricsHandler 17 series 契约序输出；writeGauge/writeCounter/writeBuildInfo/escLabel 单侧定义
- `internal/server/metrics_test.go`（新建，607 行）— 五测试：TestMetricsExposition / TestMetricsAuth（四子测）/ TestBuildInfo（三子测 escLabel 表驱动）/ TestMetricsValues（三子测数值锁）/ TestMetricsSnapshotRace（-race 压力）；metricsSeries17 契约清单 + assertExpositionShape/metricSample/getMetrics/reqMetrics/readUntilMarker 断言件；夹具全复用（startTestServerWith/dialHello/httpBaseOf/getStatus/assertKicked1013/dialHelloTicketWantAuthFailed）零新装配
- `internal/server/server.go` — Options.Version 生产直传字段（注释分档先例）；Server 加 version/mc 字段；New 零值兜底 "dev" + 装配直传；Handler() 凭据分支 `"GET /metrics"` basicAuth 包装 + 无认证分支直通 + 两分支之外 path-only 405 fallback；wsRecvBytes.Add 两站点（Hello 首读 + 稳态循环）；Hello 核销失败站点 authFailed.Add；basicAuth 三调用点传 &s.mc
- `internal/server/clients.go` — registry.clientsTotal int64（hubMu 保护注释登记）+ registerLocked 唯一加点（对称记账）；onChunk 入口 ptyOutputBytes.Add；writer 成功 Write 后 wsSentBytes.Add
- `internal/server/auth.go` — basicAuth 签名加末参 mc *metricsCounters（401/429 两站点递增，计数与事件同址纪律注释；文档头补 08-04 段）
- `cmd/wesh/main.go` — server.New Options 字面量尾部追加 Version: version（var version 单一通道）

## Decisions Made

- **metricsCounters 全五字段 + Server.mc + 快照计数器读取在 Task 1 一次落地**（详见 Deviations #1）
- **TestMetricsExposition 取黑盒形态**（详见 Deviations #2）
- **ws_sent ≥ 2×pty_output 放大比断言可判定性**：/bin/cat 零 pre-attach 输出 + 两端在册后才驱 INPUT ⇒ 全部 chunk 恒扇出至两端，逐 chunk 2·len+2 > 2·len 严格成立 + 两帧 Welcome 余量——放大比断言确定性成立零 flaky 面
- **auth 计数器精确值锁（==1/==2）**：强于 plan behavior 的 ≥1——实例私有 + 序列确定（http.Client 无重试、429 短路不 recordFail 不延长窗口由 auth.go 既有语义保证），精确等值是更强行为锁

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] metricsCounters 全五字段与 Server.mc 在 Task 1 落地（plan 字面：Task 1 仅两 auth 字段、Task 2 扩五字段 + Server.mc）**
- **Found during:** Task 1（GREEN 实现设计——plan action 逐字追踪时发现的结构性矛盾）
- **Issue:** plan Task 1 字面（metricsCounters 仅 authFailed/authThrottled、Server.mc 待 Task 2 落地）下，metricsHandler 在 Task 1 无法读取五枚计数器中的任何一枚，17 series 验收（"全在场"）只能以五个硬编码 0 占位——tracer 纪律（production-quality，绝不 throwaway）与 Known Stubs 红线双重违反
- **Fix:** Task 1 一次落地全五字段 + Server.mc + snapshotMetrics 五计数器 Load（值为真实计数器的真实零值，非占位字面量）；Task 2 只做递增挂点与数值测试（与 plan「Task 2 只做字节/auth 计数器与数值测试」的意图逐字一致）；最终代码态与 plan must_haves/artifacts 逐字一致
- **Files modified:** internal/server/metrics.go, internal/server/server.go（相对 plan 字面仅任务边界移动，终态零差异）
- **Verification:** Task 1/Task 2 全部验收 grep 通过（含 Task 2 的 server.go `&s.mc`==3、auth.go `mc \*metricsCounters`==1）；两任务 verify 命令全绿
- **Committed in:** e2939b5（Task 1 GREEN）

**2. [Rule 3 - Blocking] TestMetricsExposition 取黑盒形态（plan 字面：白盒构造最小 Server 直调 metricsHandler 经 httptest.ResponseRecorder）**
- **Found during:** Task 1（RED 测试设计）
- **Issue:** metrics_test.go 与全部既有测试文件同为 `package server_test` 外部包（plan 单文件五测试的归属约束），未导出方法 `metricsHandler` 与未导出字段在外部包不可达——plan 字面的白盒直调形态结构性不可达；另起内部包文件则违背 plan「metrics_test.go provides 五测试」的工件面
- **Fix:** 黑盒经真实实例 HTTP GET 断言（08-03 TestHealthz 同形态先例——startTestServerWith + httpBaseOf + getMetrics）——经真实 mux 注册路径，接线错误（如漏注册/注册错分支）同样被捕获，断言强度不降反升；17 series 契约序/三行组/末行换行/基线值全项保留
- **Files modified:** internal/server/metrics_test.go
- **Verification:** TestMetricsExposition -race PASS；assertExpositionShape 在四个场景实例（无认证/凭据 200 态/bp/无认证直通）复用全过
- **Committed in:** 99acc3b（RED）/ e2939b5（GREEN）

---

**Total deviations:** 2 auto-fixed（任务边界移动消除占位 + 测试形态不可达修正——终态与 plan 逐字一致）
**Impact on plan:** 全部锁定语义与 plan must_haves 逐字一致；无 scope creep；无 Rule 4 架构变更、无认证门、无包安装（go.mod/go.sum 逐字节不动由验收命令锁定）

## Issues Encountered

- **一次全量 -race 回归的孤立 flake**：Task 2 GREEN 后首轮 `go test -race -count=1 ./...` 报 FAIL（尾部事件流为 exit_when_empty/session_end 家族，失败测试名未捕获即被后续运行覆盖；该轮耗时 72.4s 显著慢于常态 ~55s，CPU 竞争特征）。随后同代码 5 连绿（立即重跑 + 三连循环 + 时敏家族 -count=2 定向复跑），无 DATA RACE 输出。归因：本 plan 改动为热路径纯 atomic 递增（ns 级无语义变化）+ 新增测试与既有测试零共享状态；flake 落在既有时敏测试面，按 SCOPE BOUNDARY 纪律登记不改动——若后续复现需按 07-deployment/deferred-items.md 路由登记
- **冒烟观察（非问题）**：凭据模式错凭据紧跟无凭据 401 即收 429（默认 ThrottleBase 1s 窗口内）——D-08「跟随认证闸」含节流语义的真实二进制实证（RESEARCH Pitfall 6 形态），README 配方明示义务随 08-05

## Authentication Gates

None——纯本地执行，无认证门。

## Known Stubs

无——17 series 全部真实数据源接线（无占位值：五计数器读真实 atomic 零值、快照读真实 registry 状态、runtime 直采）；无 TODO/FIXME/空数据流。

## Next Phase Readiness

- **08-05 phase08.mjs metrics 场景的 Go 侧行为锁全部就位**：认证两态/bp 固定/405/数值断言可直接镜像本 plan 测试的驱动序列（爬梯 pacing、stall 夹具）；真实 Prometheus 实例验证按 flagged_assumptions 登记随 08-05 人工清单复核（exposition 规范逐字断言已代替）
- **Phase 9 ldflags 注入 plumbing 就绪**：`var version` → Options.Version → s.version → escLabel → build_info 链路全程贯通，发布构建只需 `-X main.version=...` 一个注入点
- **无阻塞项**

## Self-Check: PASSED

- 文件：metrics.go ✓（新建）、metrics_test.go ✓（新建）、server.go/clients.go/auth.go/main.go ✓（修改）、08-04-SUMMARY.md ✓
- 提交：99acc3b ✓、e2939b5 ✓、06d5650 ✓、557104d ✓
- 关键指纹：metrics.go `func (s \*Server) snapshotMetrics` ×1 ✓、server.go `"GET /metrics"` ×2 ✓ / `&s.mc` ×3 ✓ / `wsRecvBytes.Add` ×2 ✓、main.go `Version: version` ×1 ✓、clients.go `clientsTotal` ×4（≥2）✓ / `ptyOutputBytes.Add|wsSentBytes.Add` ×2 ✓、auth.go `mc \*metricsCounters` ×1 ✓
- 验收命令全过：Task 1 verify（-race 专项 + 全包 + vet + go.mod 零漂移）✓；Task 2 verify（TestMetrics -race + 全仓 -race 五包绿 58.3s + vet ./...）✓；真实二进制冒烟（17 series/首行/尾换行/凭据两态/405/计数器递增）✓；八 UAT 脚本 132 断言退出 0 ✓
