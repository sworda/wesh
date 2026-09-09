---
phase: 08-observability
plan: 01
subsystem: infra
tags: [slog, json-logging, structured-logging, go-stdlib, observability, uat-migration]

requires:
  - phase: 07-deployment
    provides: sanitizeRemoteUser/proxy 提取层（remote_user 来源纪律）、captureStderr + startTrackedServerWith 同步纪律（05-01 落地、07 沿用）
provides:
  - internal/server/log.go：stderrW 动态 writer + eventLog 包级单例（slog JSONHandler）+ emitEvent 底层出口（08-02 扩展字段事件挂点）+ logEvent（签名不变，18 调用点零改动）
  - internal/server/log_test.go：parseEvents/countByEvent 断言 helper + TestLogEventJSON 端到端
  - D-18 schema 落地：msg 恒 "event" + event 独立字段 + time/level slog 默认键，jq/Loki 直打字段索引解锁
  - 全量事件断言面 JSON 化（5 Go 测试文件 + 3 UAT 脚本），凭据红线负断言子串形态逐字保留
affects: [08-02 审计事件目录, 08-03 healthz, 08-04 metrics, 08-05 UAT/README 收口]

actuals:
  tokens: 9862
  tasks: 3
  commits: 4

tech-stack:
  added: [log/slog（stdlib，go.mod 零变更）]
  patterns:
    - "动态 stderr writer（stderrW）：每次 Write 调用时读 os.Stderr 变量——captureStderr 置换语义的唯一保真形态（slog handler 构造时捕获 writer 的对抗解）"
    - "parseEvents 按行 JSON 解析断言：滤非 '{' 起始行（混合流成员不 FAIL）+ 单行非法 JSON 即 FAIL；Go/JS 双侧同构，禁止子串/正则断言 JSON 行"
    - "JSON 数字断言 float64 纪律（Pitfall 4）；行尾锚定在 event 名精确相等语义下天然消解"

key-files:
  created:
    - internal/server/log.go
    - internal/server/log_test.go
  modified:
    - internal/server/server.go
    - internal/server/limits_test.go
    - internal/server/emptyexit_test.go
    - internal/server/auth_e2e_test.go
    - internal/server/proxy_e2e_test.go
    - internal/server/multi_test.go
    - web/uat/phase05.mjs
    - web/uat/phase07.mjs
    - web/uat/phase07-b2.mjs

key-decisions:
  - "emitEvent(attrs ...slog.Attr) 设为底层出口：msg=\"event\" 单写口，08-02 扩展字段事件（attach/detach/session_*）的挂点即本函数——LogAttrs + 类型化 attr 防 !BADKEY"
  - "包级 eventLog 不调 slog.SetDefault（RESEARCH 倾向采纳）：不污染全局默认 logger，测试隔离性更好；D-15 恒 JSON 恒 INFO 无配置面，main.go 零改动"
  - "实证纠偏登记：slog JSONHandler 的 time 键 = RFC3339Nano + 进程本地时区（GOROOT json_handler.go:93-99 appendJSONTime），非 plan/RESEARCH 所述「RFC3339 毫秒 + UTC」（appendRFC3339Millis 仅 TextHandler 分支）——08-02+ 引用以本实证为准"

patterns-established:
  - "动态 writer 保捕获语义：stdlib handler 构造时捕获 io.Writer 的通用对抗形态——writer 实现内调用时解析全局变量"
  - "事件断言迁移机械变换：子串/行尾锚定 → parseEvents 字段断言（event 名精确相等 + code float64 + 键缺席断言），红线负断言（全文不含敏感串）与 JSON 化正交逐字保留"

requirements-completed: [OPS-08]

coverage:
  - id: D1
    description: "logEvent 迁移 slog JSONHandler：单行 JSON 六键（time/level/msg/event/remote/code）、动态 writer 保 captureStderr 语义、remote_user 空串/缺省不出键"
    requirement: OPS-08
    verification:
      - kind: unit
        ref: "internal/server/log_test.go#TestLogEventJSON"
        status: pass
      - kind: integration
        ref: "真实二进制冒烟：错凭据 POST → stderr JSON 行经 jq -c 'select(.event==\"auth_failed\")' 可检索"
        status: pass
    human_judgment: false
  - id: D2
    description: "5 个 Go 测试文件事件断言 JSON 字段化（limits/emptyexit/auth_e2e/proxy_e2e/multi），凭据/ticket/token 红线负断言逐字保留"
    requirement: OPS-08
    verification:
      - kind: unit
        ref: "go test ./internal/server/ -count=1 全绿 + 迁移面 -race 专项绿"
        status: pass
    human_judgment: false
  - id: D3
    description: "phase05 S6/phase07 S4/phase07-b2 B2b UAT 断言迁移 JSON 行解析 + 全量 UAT 回归（phase02-07 七脚本）"
    requirement: OPS-08
    verification:
      - kind: e2e
        ref: "node web/uat/{phase05,phase07,phase07-b2,phase02,phase03,phase04,phase06}.mjs 全退出 0（28/28、34/34、4/4、12/12、18/18、10/10、23/23）"
        status: pass
      - kind: integration
        ref: "go test -race -count=1 ./... 五包全绿"
        status: pass
    human_judgment: false

duration: 48min
completed: 2026-08-27
status: complete
---

# Phase 8 Plan 01: slog JSON 日志基座原子迁移 Summary

**logEvent 唯一出口原子迁移 slog JSONHandler（stdlib 零新依赖）：动态 stderr writer 保 captureStderr 语义，D-18 schema（msg="event" + event 字段）落地，5 Go 测试文件 + 3 UAT 脚本断言面 JSON 化，凭据红线零削弱，全量 -race 与七 UAT 脚本回归绿**

## Performance

- **Duration:** 48 min
- **Started:** 2026-08-27T15:29:23Z
- **Completed:** 2026-08-27T16:17:09Z
- **Tasks:** 3/3
- **Files modified:** 11（2 新建 + 9 修改，与 plan files_modified 清单逐一对应）

## Accomplishments

- **D-13 原子迁移落地**：logEvent 从 server.go 的 fmt.Fprintf 文本行迁入新 log.go，内部换 slog.NewJSONHandler（18 个调用点零改动、无双轨窗口）；输出单行 JSON 六键——真实二进制实证 `jq -c 'select(.event=="auth_failed")'` 直打检索成立
- **动态 writer 保住测试捕获语义**：stderrW 每次 Write 调用时读 os.Stderr 变量（RESEARCH Pitfall 1 防线）——全部 captureStderr 断言测试在 slog 形态下零失明继续绿；JSONHandler 内建 mutex 使并发 emit 行级原子（比旧 fmt.Fprintf 更强）
- **断言面机械迁移完成**：parseEvents/countByEvent（Go）与内联 parseEvents（JS，三脚本各自自含）成为唯一消费形态；行尾锚定断言在 event 名精确相等语义下消解；b64 凭据/明文口令/ticket/authorization/roTok/rwTok 红线负断言逐字保留子串形态并全绿
- **零新 CLI flag、零新外部依赖**：go.mod 不动（D-15/D-01 哲学）；启动行/分享链接行/警告行人读文本逐字节不变（D-14/D-16——全部 UAT 启动行解析消费者零适配通过）

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: TestLogEventJSON 失败测试先行（tdd）** - `e05791c` (test)
2. **Task 1 GREEN (tracer): log.go slog 基座 + server.go 旧实现删除** - `9a3f166` (feat)
3. **Task 2: 5 个 Go 测试文件事件断言迁移** - `0e4496b` (test)
4. **Task 3: phase05/07/07-b2 UAT 断言迁移 + 全量回归** - `e6e79fa` (test)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP）

_Tracer feedback gate（autonomous）：Task 1 提交后 verify 端到端重跑通过（TestLogEventJSON + vet + build），方进入 Task 2/3 扩展面。_

## Files Created/Modified

- `internal/server/log.go`（新建）— stderrW 动态 writer + eventLog 包级单例 + emitEvent 底层出口 + logEvent（签名不变迁入）；注释头登记 D-13/D-15/D-18 与迁移来源，SEC-01 红线注释逐字随迁
- `internal/server/log_test.go`（新建）— parseEvents/countByEvent helper + TestLogEventJSON 端到端（六键断言 + remote_user 缺席语义）
- `internal/server/server.go` — 旧 logEvent 文本实现与注释块整体删除（注释随迁 log.go 并去「过渡形态」表述）；os import 清理（fmt 仍服务 exitMessage 保留）；logIfMessageTooBig 原位不动
- `internal/server/limits_test.go` — TestOversize1009：countByEvent==1 + code float64 比 1009 + remote 前缀
- `internal/server/emptyexit_test.go` — 触发行零次/启动行存在 → countByEvent 精确计数（行尾锚定消解）；strings import 清理
- `internal/server/auth_e2e_test.go` — 正向对照升级 countByEvent≥1；四红线负断言逐字保留
- `internal/server/proxy_e2e_test.go` — TestRemoteUserLogging（auth_failed==3 + alice/carol + 携键总数==2）与双通道 503/稳态 1009 断言迁移；token/ticket 红线逐字保留
- `internal/server/multi_test.go` — TestMaxClients503 存在性断言（event==max_clients 且 code==503 float64）；strings import 清理
- `web/uat/phase05.mjs` — 内联 parseEvents；S6a 改 event==slow_consumer 且 code==1013 字段断言
- `web/uat/phase07.mjs` — 内联 parseEvents；S4b/S4c/S4d 字段断言化（XFF 链首精确相等/NEL 剥离/对照组无键）
- `web/uat/phase07-b2.mjs` — 内联 parseEvents；B2b 迁移（首值 alice + bob 零泄漏字段形态）；B2a/B2c/B2d 零改动幸存

## Decisions Made

- **emitEvent 底层出口形态**（plan action ① 授权）：`emitEvent(attrs ...slog.Attr)` 单写口承担 msg="event"（D-18），本 plan 仅 logEvent 消费，08-02 扩展字段事件直接挂本函数——避免 08-02 再造第二出口
- **不调 slog.SetDefault**（RESEARCH Pattern 1 倾向采纳）：server 包私有 eventLog 更内聚，main.go 零改动
- **plan verify 正则按意图修正**（详见 Deviations #1）
- **S4d 对照组防假绿加强**（详见 Deviations #2）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] plan Task 2 verify 正则不匹配任何实际测试名**
- **Found during:** Task 2（verify 准备）
- **Issue:** plan 字面 `-run 'TestOversize1009|TestExitEmpty|TestAuth|TestProxy|TestMaxClients|TestClientCount'` 中 `TestExitEmpty` 不匹配实际测试名（emptyexit_test.go 为 `TestExitWhenEmpty*` 族）——按字面执行则迁移面的 emptyexit/auth_e2e（TestLogRedaction）/proxy_e2e（TestRemoteUserLogging）等核心迁移测试根本不被 -race 运行，verify 落空
- **Fix:** 按意图修正为显式枚举迁移面全量测试：`TestOversize1009|TestExitWhenEmpty|TestLogRedaction|TestRemoteUserLogging|TestXFFThrottleKey|TestAuthHeaderNoAuthBypass|TestProxyClientIP|TestMaxClients503|TestClientCountInvariant|TestLogEventJSON`，-race 全绿（13.5s）
- **Files modified:** 无（verify 命令层面修正）
- **Verification:** 修正后 regex 命中全部迁移测试并 PASS
- **Committed in:** 0e4496b（commit message 已登记）

**2. [Rule 2 - Missing Critical] phase07.mjs S4d 对照组 noUserKey 防空捕获假绿**
- **Found during:** Task 3（S4d 迁移）
- **Issue:** 旧断言 `!includes('remote_user=')` 是全文负断言，迁移为「无一事件携 remote_user 键」字段形态后，若 stderr 未捕获到任何事件（空捕获），`every()` 对空集恒真——断言退化为空洞真（旧形态下由 loopbackRemote 的存在性旁证兜住，新形态需显式护栏）
- **Fix:** `noUserKey = evsC.length > 0 && evsC.every((m) => !('remote_user' in m))`——事件集非空前置
- **Files modified:** web/uat/phase07.mjs
- **Verification:** phase07.mjs 34/34 PASS（S4d 真实事件流下成立）
- **Committed in:** e6e79fa

### 文档纠偏（非代码偏差）

**3. [实证纠偏] slog JSONHandler time 键 = RFC3339Nano + 本地时区，非 plan 所述「RFC3339 毫秒」/「UTC」**
- **Found during:** Task 3（真实二进制 jq 冒烟）
- **Issue:** plan behavior 与 flagged_assumptions 称 time 为「RFC3339 毫秒」且「时区 UTC（stdlib 固定）」；实测输出 `2026-08-28T00:13:10.51779565+08:00`。GOROOT 实证：JSONHandler 走 `appendJSONTime`（json_handler.go:93-99，`t.AppendFormat(RFC3339Nano)`，保留记录时区）；`appendRFC3339Millis`（handler.go:622，RESEARCH 引用处）仅服务 TextHandler 分支——RESEARCH 核实了错误的分支
- **影响评估：** 零验收/真理影响——must_have 只锁「time/level 为 slog 默认键」（键名默认，成立）；TestLogEventJSON 只断言 time 键存在非空；jq/Loki 检索不受精度/时区影响。D-15 实质（恒 JSON 恒 INFO 零配置面）不变
- **处置：** 代码保持 stdlib 默认（ReplaceAttr 强制 UTC/毫秒会引入配置化雏形，违背 D-15 本意）；登记本纠偏供 08-02+ 与 README 运维节（08-05）引用时以实证为准

---

**Total deviations:** 2 auto-fixed（1 blocking verify 修正 + 1 missing critical 防假绿）+ 1 文档纠偏
**Impact on plan:** 全部为保证验证有效性与断言强度所必需；零 scope creep；行为面与 plan 锁定语义逐字一致

## Issues Encountered

- **log_test.go CJK 注释 gofmt 空格规则**（`//（` → `// （`）——本任务新建文件，即时修正后干净；multi_test.go/slowclient_test.go 的 GOROOT gofmt 漂移经 `git show HEAD` 复验为 HEAD 预存（07-deployment/deferred-items.md 07-01 条目已登记同一族），按 SCOPE BOUNDARY 纪律不随本 plan 修复

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **08-02（审计事件目录）挂点就绪**：emitEvent 底层出口就位（扩展字段事件唯一挂点）；logEvent 签名不变故 18 调用点现状即为 JSON 形态；phase05.mjs S6 的二次迁移（slow_consumer → detach reason=kick）已在本 plan 注释中预告
- **实证基线供引用**：time 键 RFC3339Nano+本地时区（纠偏 #3）；JSONHandler 行级原子（并发 emit 无交错）；C1 穿透语义不变（D-19 sanitize 推广仍属 08-02 必需——本 plan 未削弱既有 remote_user sanitize，proxy.go 零改动）
- **无阻塞项**

## Self-Check: PASSED

- 文件：internal/server/log.go ✓、internal/server/log_test.go ✓、08-01-SUMMARY.md ✓
- 提交：e05791c ✓、9a3f166 ✓、0e4496b ✓、e6e79fa ✓
- 关键指纹：log.go `slog.NewJSONHandler` ×1 ✓、log_test.go `func parseEvents` ×1 ✓
