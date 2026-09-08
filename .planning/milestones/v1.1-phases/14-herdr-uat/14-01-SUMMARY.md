---
phase: 14-herdr-uat
plan: 01
subsystem: testing
tags: [go-test, dual-mode, harness, per-client, race-detector, t-run]

requires:
  - phase: 13-resource-defense
    provides: per-client 机制全收口（teardown stop-signal 序列 / pcSupervisor last-reaped-code / 13-05 session_active 恒 true 裁决 / StopTimeout KILL 兜底）
  - phase: 12-per-client
    provides: dwell 10s 看门狗语义与 SlowDwell 测试覆写通道 / perclient_test.go 装配族（startPerClientServer/WithSpawn）
  - phase: 11-per-client
    provides: perclient_test.go 36 测与泵源夹具（readPump/drainQuiet/accumFramesUntil）
provides:
  - newTestServer 小族四形态装配点（harness_test.go：普通/tracked/handle/sess）——后续 5 个改造 plan 的统一 mode 参数化入口
  - startPerClientServerTrackedWithSpawn 姊妹变体（per-client 族 handler 追踪补齐——stderr 捕获类测双跑的结构前提）
  - 五文件双模式分叉表改造（exit/stopseq/health/slowclient/limits，13 测 36 个 mode= 子测试）
  - D-01 执行机械实证：装配点 → t.Run 双跑 → 断言分叉表 → -race 双模式绿全链经 tracer 薄片验证
affects: [14-02, 14-03, 14-04, 14-05, 14-06, 14-08, 14-12]

actuals:
  tokens: 24461   # 97844 chars / 4 over 651ae98..HEAD (internal/server)
  tasks: 2
  commits: 2

tech-stack:
  added: []   # 零新依赖（T-14-SC 红线）
  patterns:
    - "newTestServer(t, mode, argv, mutate) 小族四形态——两族母本直传零改写，defaultPCSpawnFn 镜像生产闭包"
    - "断言分叉表（mode → expected 显式表）+ shared 列期望值逐字未动（D-02 零回归证据本体）"
    - "t.Run(\"mode=shared\"/\"mode=per-client\") 双跑——CI 结构零改动单 step 天然双模式覆盖"

key-files:
  created:
    - internal/server/harness_test.go
  modified:
    - internal/server/perclient_test.go
    - internal/server/exit_test.go
    - internal/server/stopseq_test.go
    - internal/server/health_test.go
    - internal/server/slowclient_test.go
    - internal/server/limits_test.go

key-decisions:
  - "TestExitFrameSignal 单客户端形态下广播/私有化不可观测——wire 面同断言双跑，他端零感知分叉由 TestExitFrameBroadcast per-client 列承载；exitf 面分叉（shared waitExit(-1) / per-client assertNoExit 静默）"
  - "TestHealthzDraining 退出码分叉 shared=-1 / per-client=0（零会话零收割 last-reaped-code 缺省经 pcSupervisor terminate(0)）——两列同构走 newHandleTestServer，waitHandlers 弃置（wg 惰性）"
  - "TestSlowConsumerKick per-client 列 SlowDwell=500ms 短值覆写（默认 10s 使 1013 落在断言窗外；dwell 语义本体由 perclient_test.go Phase 12 三测承载）"
  - "TestGlobalCredit 按蓝本 shared-only 行走 D-03：per-client 列断言停读端满箱期间对照端输出持续到达（信用门误装配可证伪）"
  - "TestPreHelloReadLimit 装配经 newTestServer 后 shared 列 Writable true（原零值 false）——pre-Hello 路径首读即 1009 切断，该旗标结构性不可观测（注释锚定）"

patterns-established:
  - "Pattern 1: 小族装配点四形态（newTestServer/newTrackedTestServer/newHandleTestServer/newSessTestServer）——未知 mode 一律 t.Fatalf fail-fast"
  - "Pattern 2: 断言分叉表参数化（want := shared 值; if per-client { want = 分叉值 }——PITFALLS :264 认可形态，shared 值字面在场）"
  - "Pattern 3: per-client 列 kick 观测通道 = /healthz clients 计数轮询（TestPerClientDwellKick 先例——首次 Read 必须推迟到踢出可观测之后）"

requirements-completed: [PC-13]

coverage:
  - id: D1
    description: "newTestServer 小族四形态装配点（harness_test.go：普通/tracked/handle/sess，两族母本直传零改写 + defaultPCSpawnFn 生产闭包镜像）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "command: go test -race -count=1 -v -run 'TestExitFrame' ./internal/server/ → TestExitFrameBroadcast/mode=shared+per-client PASS"
        status: pass
      - kind: unit
        ref: "command: go vet ./... && $(go env GOROOT)/bin/gofmt -l internal/server/ → 零输出"
        status: pass
    human_judgment: false
  - id: D2
    description: "startPerClientServerTrackedWithSpawn 姊妹变体（per-client 族 handler 追踪补齐，既有 36 测零改动）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/limits_test.go#TestOversize1009/mode=per-client（tracked 形态运行期首证：stderr 同步边 waitHandlers 两列同构）"
        status: pass
    human_judgment: false
  - id: D3
    description: "exit_test EXIT 广播两测双模式分叉表（shared 全员广播 v1.0 逐字 / per-client 属主私有化 + 他端零感知 PC-04）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/exit_test.go#TestExitFrameBroadcast/mode=shared, TestExitFrameBroadcast/mode=per-client, TestExitFrameSignal/mode=shared, TestExitFrameSignal/mode=per-client"
        status: pass
    human_judgment: false
  - id: D4
    description: "stopseq 两测双跑（per-client 列 = 信号序列发到该会话自身进程组，exitf 收口链分叉 lifecycle 直收 vs pcSupervisor last-reaped-code）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/stopseq_test.go#TestExitWhenEmptyStopSignalTERM/mode=shared+per-client, TestExitWhenEmptyStopTimeoutKills/mode=shared+per-client"
        status: pass
    human_judgment: false
  - id: D5
    description: "health 双模式（TestHealthz 五子测 + TestHealthzDraining :251 收编 newHandleTestServer；session_active 语义分叉 shared 生命周期跟随 / per-client 13-05 恒 true）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/health_test.go#TestHealthz/mode=shared+per-client（含五内层子测）, TestHealthzDraining/mode=shared+per-client"
        status: pass
    human_judgment: false
  - id: D6
    description: "slowclient 双模式（TestSlowConsumerKick 满即踢分叉 R-08 1:1 退化 + TestGlobalCredit D-03 显式断言信用门未装配）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/slowclient_test.go#TestSlowConsumerKick/mode=shared+per-client, TestGlobalCredit/mode=shared（两子测）+mode=per-client（D-03 列）"
        status: pass
    human_judgment: false
  - id: D7
    description: "limits 四测双跑（TestOversize1009 经 newTrackedTestServer；洪水类同断言双跑）+ TestReadLimitBoundary shared 单跑 D-02 偏差登记"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/limits_test.go#TestOversize1009/mode=shared+per-client, TestFragmentedFlood1009/mode=*, TestEmptyFragmentFloodResilience/mode=*, TestPreHelloReadLimit/mode=*, TestReadLimitBoundary（单跑）"
        status: pass
    human_judgment: false

duration: 43min
completed: 2026-09-06
status: complete
---

# Phase 14 Plan 01: 双模式验证矩阵 tracer（newTestServer 小族 + 五文件 mode-mapped 批）Summary

**newTestServer 四形态小族装配点收编 shared/per-client 两装配族 + per-client tracked 姊妹变体，五文件 13 测 36 个 mode= 子测试 -race 双模式逐名绿，shared 列期望值逐字未动（D-02 零回归证据）**

## Performance

- **Duration:** 43 min
- **Started:** 2026-09-06T11:52:50Z
- **Completed:** 2026-09-06T12:35:53Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- **harness_test.go 小族四形态落地（D-01 执行机械）**：newTestServer 二值普通形 / newTrackedTestServer 三值形（waitHandlers 同步边）/ newHandleTestServer 四值形（+srv）/ newSessTestServer 三值形（sessions 访问器，14-04 首消费方）——两分支直传两族母本零改写（shared 启动期 spawn vs per-client attach 期 spawn 的 Cleanup 纪律双侧保留），defaultPCSpawnFn 逐字镜像 perclient_test.go:116-120 生产闭包形态；未知 mode 一律 t.Fatalf fail-fast
- **per-client 族 handler 追踪补齐**：startPerClientServerTrackedWithSpawn 姊妹变体（装配序列与母本逐字同构，唯一差异 = http.Serve handler 经 sync.WaitGroup 包裹镜像 startTrackedServerWith :186-193 并返回 wg.Wait）；既有函数与 36 测零改动（git diff 零删除行验证）
- **五文件双模式分叉表改造**：exit（EXIT 广播/私有化 PC-04 + 静默窗零帧 + echo 存活标记）、stopseq（信号序列发到该会话自身进程组 + exitf 收口链分叉）、health（session_active 恒 true 13-05 裁决 + :251 收编 newHandleTestServer + 退出码 -1/0 分叉）、slowclient（满即踢 R-08 1:1 退化 + TestGlobalCredit D-03 信用门未装配可证伪防线）、limits（TestOversize1009 经 newTrackedTestServer 双跑 = tracked 形态运行期首证 + TestReadLimitBoundary 单跑 D-02 偏差登记）
- **CI 时长影响登记（plan action 5 预告项）**：本批双跑使 internal/server 包 -race 全量从 ~98.5s（改造前实测）增至 ~104-106s（改造后四轮实测 103.9/104.2/104.4/105.9s）——增量 ~6-8s（洪水类测双跑为主贡献源）；后续 14-02..14-06 批量推开将持续累积，14-12 收口闸应知悉 CI 时长预算（蓝本估算 3-4 min/leg）
- **验证矩阵**：36 个 mode= 子测试逐名 PASS（4+4+14+6+8 = exit 4 / stopseq 4 / health 14 / slowclient 6 / limits 8）；全仓 5 包 -race 零回归；go vet 与 GOROOT gofmt（go1.26.3）零输出；ci.yml 零 diff

## Task Commits

Each task was committed atomically:

1. **Task 1: 【phase tracer】newTestServer 小族装配点 + per-client tracked 姊妹变体 + exit_test 双模式分叉表改造** - `f7207ab` (test)
2. **Task 2: 小型 mode-mapped 批改造——stopseq/health/slowclient/limits** - `f3521f1` (test)

**Plan metadata:** （本提交见下方 docs 收口提交）

## Files Created/Modified

- `internal/server/harness_test.go` - （新）newTestServer 小族四形态装配点 + defaultPCSpawnFn——D-01 执行机械，后续 5 个改造 plan 的统一入口
- `internal/server/perclient_test.go` - startPerClientServerTrackedWithSpawn 姊妹变体新增（per-client 族 handler 追踪补齐）
- `internal/server/exit_test.go` - 两测双跑 + EXIT 广播/私有化断言分叉表（PC-04）
- `internal/server/stopseq_test.go` - 两测双跑 + stop-signal 序列 N 组分叉表
- `internal/server/health_test.go` - TestHealthz 五子测双跑 + TestHealthzDraining 收编 newHandleTestServer + session_active 语义分叉表
- `internal/server/slowclient_test.go` - TestSlowConsumerKick 双跑（dwell 短值覆写）+ TestGlobalCredit D-03 列
- `internal/server/limits_test.go` - 四测双跑（TestOversize1009 经 newTrackedTestServer）+ TestReadLimitBoundary D-02 偏差登记注释

## Decisions Made

- **TestExitFrameSignal 单客户端形态的不可观测面处理**：wire 面（EXIT{-1, SIGHUP} + 1000）两模式同值双跑，广播/私有化分叉不可观测（单客户端无他端）——他端零感知由 TestExitFrameBroadcast per-client 列承载；exitf 面分叉（shared waitExit(-1) lifecycle 直收 / per-client assertNoExit 静默——无第二终结源）
- **TestHealthzDraining 退出码分叉实证**：per-client 零会话零 attach 形态下 Shutdown → exiting 位唤醒 pcSupervisor → last-reaped-code 缺省 0 → terminate(0)——分叉表 shared=-1 / per-client=0；末次 session_active 分叉 shared=false / per-client=true 恒（13-05 裁决）
- **TestSlowConsumerKick per-client 列 SlowDwell=500ms 覆写**：默认 10s 使 1013 落在 assertKicked1013 10s 断言窗外；dwell 语义本体（停读续读/前进不踢/门转换）由 perclient_test.go Phase 12 三测承载，本列只锁「无信用门 + 满即踢」分叉点（plan 蓝本 R-08 行口径）
- **TestGlobalCredit D-03 列形态**：一端停读期间对照端输出持续到达（600ms 总窗口净增长）——比「双端停读」形态更贴合 plan 文本「对照端 PTY 输出持续到达」，且证伪面完整（全局信用门误装配 → 停读端满箱关门全局读路径 → 对照端停滞翻车）
- **TestPreHelloReadLimit 装配注记**：经 newTestServer 后 shared 列 Writable true（原零值 false）——pre-Hello 路径首读即 1009 切断，Writable（Welcome 模式字段 + INPUT 门控）结构性不可观测；测试体注释锚定该论证

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestSlowConsumerKick per-client 列首次 Read 过早续读重置 dwell**
- **Found during:** Task 2（全量 -race 回归第一轮）
- **Issue:** per-client 列沿用了 shared 列的同步形态（正常端 12MiB 等待后即读 stall 端）——但 per-client 下两客户端各持独立洪水，正常端字节进度与 stall 端停读/dwell 计时结构性解耦；全量 -race 负载下 stall 端 dwell（500ms）尚未到期时 assertKicked1013 的首次 Read 消化管道积存 → outbox 排空 → dwell 续读重置 → 会话以 1000 正常收尾（实测命中：`stall client close code = 1000, want 1013`）
- **Fix:** kick 观测换 /healthz clients 计数轮询（kick → removeLocked → 2→1；TestPerClientDwellKick 既定观测通道——只读 HTTP 不打扰 WS stall 面，轮询替代固定 sleep），首次 Read 推迟到踢出已可观测之后
- **Files modified:** internal/server/slowclient_test.go
- **Verification:** 修复后 TestSlowConsumerKick -race -count=5 五连绿 + 全量 -race 连续两轮绿（103.9s/104.9s）+ 全仓 5 包绿；修复前失败形态（1000 收尾）结构性不可达（踢出前无任何读方，TCP 吸收上限使 outbox 必然写满 → dwell 必然到期）
- **Committed in:** f3521f1（Task 2 提交内）

---

**Total deviations:** 1 auto-fixed（1 bug）
**Impact on plan:** 修复为 per-client 列同步边的必要正确性补齐（shared 列同步形态在独立洪水拓扑下失效），无范围蔓延；修复形态复用既有先例通道（TestPerClientDwellKick 的 /healthz 轮询）。

## Issues Encountered

None——plan 预告的洪水类测双跑 CI 时长增量已实测登记（~6-8s/包，见 Accomplishments）。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 小族四形态经 tracer 薄片端到端实证（tracked 形态获 TestOversize1009 运行期首证；handle 形态获 TestHealthzDraining 运行期首证；sess 形态编译期在场，首个消费方 = 14-04 resize_arb 四子测试）——后续 14-02（e2e/multi/emptyexit）、14-03（shutdown/metrics 归一）、14-05（handshake/keepalive/auth_e2e mode-agnostic 批）、14-06（proxy/basepath/customindex/events/log）可直接复用
- tracer 反馈闸结论：`<verify>` 端到端复跑通过（-race 双模式逐名绿 + 全量回归 + vet/gofmt 零输出 + diff 白名单自审）——plan frontmatter autonomous: true + 全自动 verify 通过，按 Pattern A 直接推进扩展任务（Task 2）
- PC-13 需求勾选按 shared-ID 门阻断（10 个兄弟 plan 声明中，herdr E2E 证据未齐）——归 phase 末收口 plan
- TestReadLimitBoundary D-02 偏差已登记（蓝本 limits 行的例外），14-12 收口闸 diff 白名单应包含本测单跑注记

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-06*

## Self-Check: PASSED

- 7 个 key-files 全部在盘（harness_test.go 新建 + 6 改造文件）
- 2 个任务提交在库（f7207ab / f3521f1）
- frontmatter `status: complete` 在场
- ci.yml 零 diff（D-01 结构零改动验证）
- 36 个 mode= 子测试 -race 逐名 PASS；全仓 5 包 -race 零回归；go vet + GOROOT gofmt 零输出
