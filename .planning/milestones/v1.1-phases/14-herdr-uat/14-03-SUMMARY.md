---
phase: 14-herdr-uat
plan: 03
subsystem: testing
tags: [go-test, dual-mode, emptyexit, shutdown, metrics, session-active, bounded-join, second-termination-source, race-detector, t-run]

requires:
  - phase: 14-herdr-uat
    plan: 01
    provides: newTestServer 小族四形态装配点（harness_test.go）+ startPerClientServerTrackedWithSpawn 姊妹变体
  - phase: 14-herdr-uat
    plan: 02
    provides: e2e/multi 双模式分叉表先例形态（D-03 四段式 / 同断言双跑判定注释 / 泵源静默窗）
  - phase: 13-resource-defense
    provides: per-client 终结与观测机制全收口（pcSupervisor/pcExitReq/Shutdown N 组信号+有界 join/session_active 模式分支/21 series 扩展）
provides:
  - emptyexit_test.go 双模式归一（7 测试体）：--once/--exit-when-empty 断言分叉表——第二终结源列显式成表（Phase 13 混入两测断言逐字归位）
  - shutdown_test.go 双模式归一（4 测试体）：Shutdown N 组信号 + 有界 join 分叉表（TwoGroups/JoinBounded 断言归位）+ startShutdownServerWith 散点删除（全仓代码引用零命中）
  - metrics_test.go 双模式归一（5 测试体）：session_active 语义 + series 镜像 + HELP 文案三分叉面成表（TestMetricsPerClient 吸收归位）
  - 三文件 16 处两装配族调用点收编 newTestServer 小族单一装配（D-01 混合形态消除）
affects: [14-04, 14-05, 14-06, 14-12]

actuals:
  tokens: 32094   # 128376 chars / 4 over 25cbab2..HEAD（三测试文件，估 70000 的 46%——14-02 同款模式化重排低于 planner 低置信预估）
  tasks: 3
  commits: 3

tech-stack:
  added: []   # 零新依赖（T-14-SC 红线，go.mod/go.sum/ci.yml 零 diff）
  patterns:
    - "断言归位（assertion relocation）：被吸收测试的断言逐字搬入 mode= 列而非重写——TestEmptyExitPerClient*→Immediate/LifecycleGate、TwoGroups/JoinBounded→Shutdown1001/StopTimeout、TestMetricsPerClient 两子测→Exposition 两列"
    - "同断言双跑判定注释（GraceCancel/GraceExpire/KickTrigger/Auth/BuildInfo）：可观测期望两列同值时共体执行 + 机制分叉注释（14-02 Pattern 4 延伸）"
    - "分叉值 want 变量 + 双原文案 if/else（TimerAfterLifecycle 触发计数/Exposition session_active）：期望值字面两列均在场"
    - "迟触发污染防线：per-client 长宽限计时器悬挂场景尾部 srv.Shutdown() 收口（exiting 位使回调静默——per-client 无 shared lifecycle-exiting 免疫结构）"

key-files:
  created: []
  modified:
    - internal/server/emptyexit_test.go
    - internal/server/shutdown_test.go
    - internal/server/metrics_test.go

key-decisions:
  - "TestShutdown1001 per-client 列=TwoGroups 归位（N=2 组）而非新写单客户端镜像列——plan behavior「per-client 列=全部存活进程组各一遍 stop-signal + session_end==N（Phase 13 既有断言归位）」+ action 3「shared 列对应测保持单组语义原断言」双条明示；夹具按模式分叉（交互 sh 供 readSessionPid 回显 vs sleep 100 直跑）"
  - "TestShutdownStopTimeout per-client 列=JoinBounded 归位（唯一同需求映射：stop-timeout KILL 兜底）；ResidualGroup（形态二）/DeadlineExits（形态四）判 per-client-only 保持原样——pcSessions 残留/D-state 收口无 shared 对照概念面（truth 5 裁决：默认不动）"
  - "TestExitWhenEmptyPromoteKickOnce per-client 列 D-03 四段式（14-02 owner 四测先例）：毒化 outbox（cap=1）在场静默窗零帧零错误（promote 误装配即翻车）+ ShrinkOutboxForTest 反向改写恢复容量后 echo（INPUT 生效）；尾部 Shutdown 收口防 1min 计时器 +1min 迟触发事件污染后继捕获窗"
  - "TestMetricsExposition 吸收 TestMetricsPerClient 两子测（shared_zeros→shared 列 attach 后采样——HELP 静态两态同值；per_client_branch→per-client 列）；统护 ctx 15s（收敛轮询 10s 护栏所需，shared 列 10→15s 非断言面放宽）"
  - "TestMetricsValues 放大比分叉：shared fan-out ×2 / per-client 1:1（R-08 分工表——每会话输出只发属主，sent ≥ 1×ptyOut 结构性下界）；夹具分叉（shared 单端 INPUT 扇出 / per-client 双端各发各收）"
  - "TestMetricsSnapshotRace per-client 列放宽 spawn 节流桶 100/100/100/100（churn 格 load_test.go 先例值）：默认 per-IP burst 4 下 2s 窗几乎全 1008 拒绝，注册表/pcSessions 搅动面消失；~100-200 spawns/2s 窗负载注记 CI 观察"
  - "series 镜像口径以 metrics.go 现状为准（plan action 2 明示）：两模式同持 21 series（shared 四 spawn 计数器恒 0 不摘——13-05 落地），plan 文本「17 shared / 21 per-client」与现状不符，偏差登记（WINDOWS #42）"

patterns-established:
  - "Pattern 6: 断言归位纪律（14-03 核心）——被吸收测试函数退役但断言逐字存活于 mode= 列，git 字面清单自审核对 DIMINISHED 项逐一归类（合并去重/参数化渲染/夹具去重三类白名单）"
  - "Pattern 7: per-client 悬挂计时器收口（PromoteKickOnce 尾部 Shutdown）——长 grace 计时器在 per-client 无 exiting 免疫面，测试收尾必须显式置 exiting 或消费事件，否则 +grace 迟触发 logEvent 落后继测试捕获窗"

requirements-completed: []   # PC-13 共享 ID 门：herdr E2E 证据归 14-09 pw 层承载（14-08 先例延续），勾选留 phase 末收口

coverage:
  - id: D1
    description: "emptyexit_test.go 九测双模式归一：第二终结源列显式成表（per-client 空迁移 terminate 路径 P1）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/emptyexit_test.go#TestExitWhenEmptyImmediate|GraceCancel|GraceExpire|KickTrigger|LifecycleGate|TimerAfterLifecycle|PromoteKickOnce/mode=shared+per-client（14 子测试）"
        status: pass
    human_judgment: false
  - id: D2
    description: "shutdown_test.go 六测双模式归一：N 组信号 + 有界 join 列 + startShutdownServerWith 散点删除"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/shutdown_test.go#TestShutdown1001|TestShutdownStopTimeout/mode=shared+per-client（session_end==2/时长双锚在 per-client 列）+ ResidualGroup/DeadlineExits 保持单模式原样"
        status: pass
    human_judgment: false
  - id: D3
    description: "metrics_test.go 六测双模式归一：session_active 语义 + series 镜像 + HELP 文案三分叉面"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/metrics_test.go#TestMetricsExposition|Auth|BuildInfo|Values|SnapshotRace/mode=shared+per-client（30 子测试）"
        status: pass
    human_judgment: false
  - id: D4
    description: "双证据链零削弱（D-02 红线）：shared 列与 Phase 13 per-client 断言两列字面均逐字保持"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "git 字面清单自审（三文件 before/after 全量核对）：emptyexit 6 处计数减少均两同构断言体合一、shutdown 仅被删 helper 自有字面消失、metrics 4 处为吸收归并夹具去重 + 放大比消息参数化（shared 渲染逐字不变）；全部断言期望值字面 1:1 存活"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-06
status: complete
---

# Phase 14 Plan 03: emptyexit/shutdown/metrics 三文件双模式归一 Summary

**mode-mapped 终结/观测批归一落地——21 测收编为 16 测试体（7+4+5）经 newTestServer 小族双 t.Run：第二终结源列（exitf(42)/(-1) 与触发事件 0/1 真值相反对照）、N 组信号 + 有界 join 列（session_end==N/时长双锚）、session_active 语义 + series 镜像 + HELP 三分叉面显式成表；三文件 16 处两装配族调用点归一，startShutdownServerWith 散点删除；48 个 mode= 子测试（本批）+ 52 含 stopseq 既有 -race 逐名绿，shared 列与 Phase 13 per-client 断言两列字面逐字保持**

## Performance

- **Duration:** 25 min（23:02-23:27 +0800）
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- **emptyexit_test.go 九测归一（Task 1，7 测试体）**：:42/:69/:134/:180/:211 换 newTestServer、:250 换 newTrackedTestServer（stderr 同步边保留）、:322 换 newHandleTestServer；Phase 13 混入两测断言逐字归位（ClientFirst→Immediate per-client 列含 exit_when_empty 恰 1 条正面证据；ChildFirst→LifecycleGate 共体——wire/exitf 面两列同值，last-reaped-code 机制注释）；TimerAfterLifecycle 断言分叉表（shared=exitf(42)+触发事件零次原断言逐字 / per-client=exitf(-1)+触发事件恰 1 次——时序翻转的正面对照列）；PromoteKickOnce per-client 列 D-03 未装配四段式（B 恒 rw + healthz/pcSessions 账目收敛 + 毒化 outbox 静默窗零帧零错误 + 恢复容量 echo + 尾部 Shutdown 收口防迟触发污染）；KickTrigger per-client 列 SlowDwell=500ms（14-01 勘误通道）；GraceExpire 双模式覆盖兑现 Looks-Done :342 行
- **shutdown_test.go 六测归一（Task 2，4 测试体）**：:107/:137 收编 newHandleTestServer（waitHandlers 忽略——wg 惰性）；TestShutdown1001 per-client 列=TwoGroups 断言归位（双 pgid ESRCH + session_end==2 + exit_code -1/signal SIGHUP 逐字）；TestShutdownStopTimeout per-client 列=JoinBounded 断言归位（时长双锚 [250ms, 2.5s) + session_end SIGKILL 归因）；startShutdownServerWith 删除——全仓代码引用零命中（events_test.go:105/health_test.go:307 为既有注释级谱系提及）；ResidualGroup/DeadlineExits 保持 per-client-only 原样
- **metrics_test.go 六测归一（Task 3，5 测试体）**：15 处 startTestServerWith + 1 处 startPerClientServerWithSpawn 全收编；TestMetricsPerClient 吸收归位（shared_zeros→shared 列、per_client_branch→per-client 列——session_active 2→1 收敛与 spawn_total 恒 2 判别面逐字）；session_active 语义分叉（shared 探活 1 / per-client 零会话 0）；series 镜像双模式同持 21 series；HELP 文案两列逐字；Values 放大比分叉（×2 / 1:1）+ kick 子测 SlowDwell+healthz 同步边；SnapshotRace per-client 节流放宽（churn 格先例值）；pty 导入随吸收移除
- **验证矩阵**：本批 48 个 mode= 子测试（emptyexit 14 + shutdown 4 + metrics 30）-race 逐名绿；含 stopseq 既有合计 52；包 -race 全量三轮绿（141.4s/141.5s/146.8s）+ 全仓 5 包绿（148.0s）；go vet 与 GOROOT gofmt（go1.26.3）零输出；go.mod/go.sum/ci.yml 零 diff（T-14-SC）
- **CI 时长增量登记**：包 -race 全量从 14-02 基线 ~132-133s 增至 ~141-147s（+9-14s；三文件双跑 + SnapshotRace per-client spawn 搅动为主贡献）——14-12 收口闸知悉 CI 时长预算（蓝本估算 3-4 min/leg，当前累计仍在界内）

## Task Commits

Each task was committed atomically:

1. **Task 1: emptyexit_test.go 九测双模式归一（D-01/D-02，第二终结源列）** - `a5ef367` (test)
2. **Task 2: shutdown_test.go 六测双模式归一（D-01/D-02，N 组信号+有界 join 列）** - `62d406f` (test)
3. **Task 3: metrics_test.go 六测双模式归一（D-01/D-02，session_active 语义+series 镜像列）** - `f4d8fad` (test)

**Plan metadata:** （见下方 docs 收口提交）

## Files Created/Modified

- `internal/server/emptyexit_test.go` - 9 测→7 测试体：两 per-client 测断言归位 + TimerAfterLifecycle 真值相反分叉表 + PromoteKickOnce D-03 四段式 + SlowDwell 覆写
- `internal/server/shutdown_test.go` - 6 测→4 测试体：TwoGroups/JoinBounded 归位 + startShutdownServerWith 删除 + ResidualGroup/DeadlineExits 保持原样
- `internal/server/metrics_test.go` - 6 测→5 测试体：TestMetricsPerClient 吸收 + 三分叉面成表 + 放大比/kick/节流三处夹具分叉

## per-client-only 保持不动清单（truth 5 登记）

| 测试 | 归属判定 | 理由 |
|------|---------|------|
| TestPerClientShutdownResidualGroup（形态二） | per-client-only 保持原样 | 断开未收割残留 = pcSessions 独立于 registry 的概念面（Pitfall 6），shared 无对应语义 |
| TestPerClientShutdownDeadlineExits（形态四） | per-client-only 保持原样 | join 到期无条件退出 = per-client 有界 join 机械的 D-state 代理，shared 无 join |
| emptyexit 两混入测 | 有 shared 对照面 → 归位吸收（非保持） | ClientFirst↔Immediate、ChildFirst↔LifecycleGate 两时序对（研究 §4.3 表） |
| metrics TestMetricsPerClient | 有 shared 对照面 → 归位吸收（非保持） | 同名 series 双模式镜像（蓝本 :393 health/metrics 行） |

## Decisions Made

（见 frontmatter key-decisions——7 条全量）

## Deviations from Plan

### 口径偏差（plan 文本与实测现状，D-02 登记）

**1. [口径] plan behavior「series 镜像 17 shared / 21 per-client」与 metrics.go 现状不符**
- **Found during:** Task 3
- **Issue:** 13-05 落地形态为双模式同持 21 series（shared 四 spawn 计数器恒 0 不摘——credit_gate 恒 0 先例），「17 shared」计数与代码现状矛盾
- **Resolution:** 按 plan action 2 明示「series 清单断言以 metrics.go 现状为准核定」执行——assertExpositionShape 双模式同断言 21 series；登记 WINDOWS #42 供 14-12 diff 白名单审查知悉

**2. [口径] startShutdownServerWith「全仓零引用（grep 零命中）」的可达形态**
- **Found during:** Task 2
- **Issue:** events_test.go:105 / health_test.go:307 为既有注释级谱系提及（14-01 遗留，非本 plan 辖区），纯文本 grep 非零
- **Resolution:** 代码引用零命中（helper 已删、调用点已收编）；本文件头 2 处登记性注释提及保留为 D-01 散点消除的文档证据

### Auto-fixed Issues

**3. [Rule 1 - 文案参数化] TimerAfterLifecycle 触发计数 / Exposition session_active 两处分叉值消息**
- **Found during:** Task 1/3
- **Issue:** 两列真值相反（0/1）的计数断言需共体执行，原 shared 消息文案与 per-client 语义不可共用
- **Fix:** 分叉值经 want 变量 + 消息按模式 if/else 分叉——两列原消息文案均在场逐字（TimerAfterLifecycle shared「timer callback missing exiting recheck」保留；Exposition shared「（会话存活）」保留）
- **Files:** internal/server/emptyexit_test.go、internal/server/metrics_test.go
- **Committed in:** a5ef367、f4d8fad

**4. [Rule 1 - 参数化渲染] TestMetricsValues 放大比消息参数化**
- **Found during:** Task 3
- **Issue:** shared「want >= 2×...（fan-out ×2 放大比）」与 per-client 1× 下界需共体
- **Fix:** amp/ampDesc 参数化——shared 列渲染输出与原文案逐字一致（amp=2/desc 同串）；期望值下界 2 为 shared 列不变量
- **Files:** internal/server/metrics_test.go
- **Committed in:** f4d8fad

**5. [Rule 3 - 迟触发污染防线] PromoteKickOnce per-client 列尾部 Shutdown 收口**
- **Found during:** Task 1（设计期推演）
- **Issue:** grace=1min 悬挂计时器在 per-client 无 shared 的 lifecycle-exiting 免疫结构——测试结束后 +1min 回调触发 exit_when_empty 事件落后继测试捕获窗（计数断言 flake 面）
- **Fix:** 尾部 cB.CloseNow() + srv.Shutdown()（exiting 位使回调复查静默）+ waitExit(-1) 消费收口；grace 值 shared 列 1min 逐字保持
- **Files:** internal/server/emptyexit_test.go
- **Committed in:** a5ef367

**6. [Rule 1 - 转录空格] shutdown/metrics 重写后 gofmt 命中 4 处 doc 注释 CJK 标点接续行**
- **Found during:** Task 2/3
- **Issue:** 整文件重写转录丢失原文件 doc 注释 `// （` 前空格 3 处 + TestMetricsValues 注释列表前缺空行 1 处（GOROOT gofmt go1.26.3 收口闸工具命中）
- **Fix:** gofmt -w 归一（10-05 既定工具，13-08 段①同款处置）
- **Files:** internal/server/shutdown_test.go、internal/server/metrics_test.go
- **Committed in:** 62d406f、f4d8fad

### 归并去重登记（字面清单自审白名单）

**7. [D-02 登记] 字面计数减少三类白名单（零期望值删除）**
- emptyexit 6 处（"/bin/cat" 5→4、"sleep 1; exit 42" 3→2、"sh"/"-c" 3→2、"no frames collected" 2→1、"close code...1000" 2→1）：两同构断言体合一（LifecycleGate/ChildFirst、Immediate/ClientFirst 共体）——同值断言双模式共体执行，覆盖不减
- shutdown 8 处（"ws://"、"/ws"、"tcp"、"pty.Start: %v"、"net.Listen: %v"、"net/http"、"net"、"127.0.0.1:0"）：被删 startShutdownServerWith 自有字面（plan action 2 既定）
- metrics 5 处（"/metrics" 24→23、"/bin/cat" 15→13：吸收归并夹具去重；放大比消息参数化；"shared_zeros"/"per_client_branch"：被吸收子测名；pty 导入移除）：断言期望值全量 1:1 存活

---

**Total deviations:** 2 口径登记（plan 文本 vs 现状）+ 4 auto-fixed（Rule 1×3/Rule 3×1）+ 1 归并去重白名单登记
**Impact on plan:** 零范围蔓延——所有处置均在 plan 分叉表语义框架内（metrics.go 现状核定为 plan action 2 明示路径；消息参数化不触期望值；迟触发防线为测试卫生）；口径偏差已登记 WINDOWS #42 供 14-12 收口闸核对。

## Issues Encountered

None——SnapshotRace per-client 列 spawn 负载（~100-200 spawns/2s 窗，节流放宽后）本机两轮稳定，CI 首跑观察项已在 key-decisions 注记。

## User Setup Required

None - no external service configuration required.

## Known Stubs

None——全部断言为可执行真断言，零 t.Skip、零 TODO/FIXME、零放宽形态（P11 和稀泥禁令逐分叉点核验：TimerAfterLifecycle 触发计数 0/1 两列真值相反均显式断言、放大比 2×/1× 两列下界各自锁死、session_active 0/1/2/收敛全形态在表）。

## Threat Flags

无新增安全面（零生产代码变更；三测试文件改造不触 trust boundary——T-14-05 经字面清单自审闭环、T-14-06 经 per-client-only 清单闭环、T-14-SC 经依赖面零 diff 闭环）。

## Next Phase Readiness

- 三文件归一批收口——D-01 混合形态（两装配族并存）在 emptyexit/shutdown/metrics 消除；后续 14-04（resize_arb，newSessTestServer 首消费）、14-05（handshake/keepalive/auth_e2e mode-agnostic 批）、14-06（proxy/basepath/customindex/events/log）可复用本批形态（断言归位纪律 / 同断言双跑判定 / D-03 四段式 / want 变量分叉）
- 14-12 收口闸知悉：CI 时长累计 ~141-147s（包内）、WINDOWS #42（series 口径）、startShutdownServerWith 注释级谱系提及、promote 毒化 outbox 反向改写（ShrinkOutboxForTest 恢复容量）四处审查锚点

## Self-Check: PASSED

- 三测试文件 + SUMMARY 均在场；三任务提交（a5ef367/62d406f/f4d8fad）均可在 git log 检出；SUMMARY frontmatter `status: complete` 在场。
