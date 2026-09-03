---
phase: 05-multi-client
plan: 10
subsystem: api
tags: [websocket, protocol, resize-arbitration, multi-client, go]

# Dependency graph
requires:
  - phase: 05-multi-client
    provides: 05-04 resize 仲裁器（arbiter/recalcNow/participates 矩阵）+ 05-03 递补升格 Welcome 推送先例 + 05-08 前端升格纠正链
provides:
  - Welcome 帧 JSON 载荷恒携会话 cols/rows（attach 升档 / 递补升格 / 运行期推送三通道同形）
  - pty.SpawnCols/SpawnRows 导出常量（spawn 尺寸单一事实源）
  - sessionDimsLocked() 会话尺寸取值（last 零值回落 spawn 尺寸，与 PTY 实际尺寸同源）
  - pushSessionDimsLocked() 运行期尺寸下发（recalcNow 的 last 变化分支唯一挂点）
affects: [05-11 前端视口约束（消费 Welcome cols/rows）, 05-12, phase-06 协议文档]

# Actuals (#2632)
actuals:
  tokens: 9768
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "运行期尺寸下发单挂点：recalcNow 的 last 变化分支 = 唯一检测点即唯一推送点（attach/detach/kick/升格/防抖五调用点全覆盖，目标不变零推送）"
    - "会话尺寸同源论证：arbiter.last（参与期）= PTY 实际尺寸；last 零值 = spawn 尺寸回落（pty.SpawnCols/SpawnRows 单一事实源）"

key-files:
  created: []
  modified:
    - internal/proto/proto.go
    - internal/proto/proto_test.go
    - internal/pty/spawn.go
    - internal/server/resize.go
    - internal/server/server.go
    - internal/server/clients.go
    - internal/server/clients_test.go
    - internal/server/multi_test.go
    - internal/server/resize_arb_test.go
    - internal/server/resize_test.go

key-decisions:
  - "[Phase 05-10]: G-05-1 方向 A 落地——Welcome 三通道（attach/升格/运行期推送）恒携会话 cols/rows，恒序列化无 omitempty（「缺席 = 旧服务端」识别契约，P2 D-02 加键兼容增量零新类型字节）"
  - "[Phase 05-10]: attach 升档序列重排——addMember/recalcNow 前移至 Welcome 组帧之前（Welcome 恒携 attach 完成后生效的会话尺寸）；Welcome 恒首帧与 hubMu > sess.fdMu 锁序两不变量保持，推送循环不触达未登记的 attach 者自身（零重复帧）"
  - "[Phase 05-10]: 升格 Welcome 携 cand.dims 而非重算 last——owner 模式升格后参与集恒为 {cand} 单员，两值恒等；保持 trySend 在前、失败踢出重扫的既有形态（避免参与集回滚复杂度）"
  - "[Phase 05-10]: 运行期推送按各端当前生效 mode 组帧 + prefs 双档选档（D-13 纪律在推送通道不漂移：ro 端推送永不含 osc52）；trySend 失败走 kickOrCreditLocked（R-08 分工复用）"

patterns-established:
  - "尺寸推送 range 内踢出安全性：map delete in range 为 Go spec 安全 + 每次踢出永久移除一端保证嵌套 recalcNow/推送有界终止（≤ max-clients）"

requirements-completed: [MULTI-01, MULTI-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Welcome 帧恒携会话 cols/rows（恒序列化契约 + dims round-trip + 缺席键识别旧服务端）"
    requirement: MULTI-01
    verification:
      - kind: unit
        ref: "internal/proto/proto_test.go#TestWelcomeFrameErrorFrame（dims round-trip + dims keys always present 子测）"
        status: pass
    human_judgment: false
  - id: D2
    description: "attach Welcome 携 attach 完成后生效的会话尺寸（owner 单员 last-wins / ro 旁观者携会话尺寸而非自身窗口 / all 模式 min-rect 重算）+ 升格 Welcome 携新 owner 尺寸 cand.dims"
    requirement: MULTI-01
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestWelcomeSessionDims（owner模式attach与升格携会话尺寸 / all模式attach携min-rect重算尺寸）"
        status: pass
    human_judgment: false
  - id: D3
    description: "owner 窗口 resize 经 50ms 防抖重算后全部在线客户端（含 ro 旁观者与上报者自身）收到携新会话尺寸的 'W' 推送 + PTY 跟随"
    requirement: MULTI-04
    verification:
      - kind: integration
        ref: "internal/server/resize_arb_test.go#TestResizeArbitration/运行期尺寸变化推送"
        status: pass
    human_judgment: false
  - id: D4
    description: "sessionDimsLocked 会话尺寸取值（spawn 回落 / 参与期恒等）"
    requirement: MULTI-04
    verification:
      - kind: unit
        ref: "internal/server/resize_test.go#TestSessionDimsLocked"
        status: pass
    human_judgment: false
  - id: D5
    description: "既有行为零回归：全量 go test -race（S1b 逐字节一致广播路径不动 / ro 端 RESIZE 双闸不动 / 升格 PTY 跟随不动 / TestAllPolicy 2→1 推送适配断言）"
    requirement: MULTI-04
    verification:
      - kind: integration
        ref: "go test ./... -race -count=1（全包 ok）"
        status: pass
    human_judgment: false

# Metrics
duration: 28min
completed: 2026-08-22
status: complete
---

# Phase 05 Plan 10: G-05-1 会话尺寸下发（服务端半侧）Summary

**Welcome 帧三通道（attach 升档 / 递补升格 / 运行期 recalcNow 推送）恒携会话 cols/rows——任意时刻任意在线客户端持有的最近一帧 Welcome 尺寸 == 服务端会话尺寸（恒 = PTY 实际尺寸），为 05-11 前端视口约束提供同源数据源。**

## Performance

- **Duration:** 28 min
- **Started:** 2026-08-22T03:11:50Z
- **Completed:** 2026-08-22T03:39:39Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments

- WelcomePayload 增 Cols/Rows 恒在键（刻意无 omitempty：「缺席 = 旧服务端」双向兼容识别契约），WelcomeFrame 签名携尺寸，三处组帧点（attach / promoteNextLocked / 运行期推送）原子适配——Go 包级编译原子单位内零漂移窗口
- attach 升档序列重排：addMember/recalcNow 前移至 Welcome 组帧之前，消除 Welcome 携带 pre-attach 过时尺寸的时序缺陷；Welcome 恒首帧（入队仍先于 registerLocked）与 hubMu > sess.fdMu 锁序两不变量注释级论证保持
- 运行期尺寸下发唯一挂点闭合：pushSessionDimsLocked 挂进 recalcNow 的 last 变化分支——attach/detach/kick/升格/防抖五调用点全覆盖，目标尺寸不变零推送（无放大无循环）；ro 旁观者与上报者自身均在协议层可观测收到新会话尺寸（100/30 断言锁定）
- pty.SpawnCols/SpawnRows 导出常量消除 spawn 尺寸魔法数双写；sessionDimsLocked 的「last 零值 = spawn 80x24 回落」与 PTY 实际尺寸同源论证注释落地

## Task Commits

Each task was committed atomically:

1. **Task 1: Welcome 契约增量（proto cols/rows + pty spawn 常量 + sessionDimsLocked）与三处组帧点原子适配** - `75e4def` (feat)
2. **Task 2: recalcNow 推送挂点（运行期尺寸下发）+ Go 行为测试组（attach/升格/推送三通道断言）** - `9cc76f4` (feat)

**Plan metadata:** 见最终 docs 提交（docs(05-10): complete ... plan）

## Files Created/Modified

- `internal/proto/proto.go` - WelcomePayload 增 Cols/Rows 恒在键 + WelcomeFrame 签名携尺寸 + 'W' 常量注释补尺寸键语义（G-05-1 根因与方向 A 裁决登记）
- `internal/proto/proto_test.go` - 三调用点适配 + dims round-trip 断言 + 「prefs nil 时 cols/rows 键恒在」map 解码子测
- `internal/pty/spawn.go` - SpawnCols=80/SpawnRows=24 导出常量，StartWithSize 引用（单一事实源）
- `internal/server/resize.go` - sessionDimsLocked()（spawn 回落/恒等）+ pushSessionDimsLocked() 挂进 recalcNow last 变化分支
- `internal/server/server.go` - attach 升档序列重排（addMember/recalcNow → sessionDimsLocked → Welcome 组帧 → registerLocked）+ 两段时序注释更新
- `internal/server/clients.go` - promoteNextLocked 升格 Welcome 携 cand.dims + 单员恒等论证注释
- `internal/server/clients_test.go` - mergeBatch 形状测试两调用点补尺寸实参
- `internal/server/multi_test.go` - TestWelcomeSessionDims 新测试（owner attach/升格/all min-rect 三通道）+ TestAllPolicy 2→1 推送适配
- `internal/server/resize_arb_test.go` - TestResizeArbitration/运行期尺寸变化推送 子测（双端收推送 + PTY 跟随）
- `internal/server/resize_test.go` - TestSessionDimsLocked 白盒单测

## Decisions Made

- **G-05-1 方向 A 落地形态**：Welcome 三通道同形携会话 cols/rows；恒序列化无 omitempty——新前端靠「缺席 = 旧服务端」识别遗留形态，旧前端忽略未知键，双向兼容零协议破坏（P2 D-02 加键纪律，零新类型字节）
- **attach 升档重排的不变量保持**：Welcome 入队先于 registerLocked（恒首帧）；recalcNow 推送循环遍历注册表不触达未登记的 attach 者自身（其会话尺寸由 Welcome 承载，零重复帧）；hubMu > sess.fdMu 锁序不变
- **升格 Welcome 携 cand.dims**：owner 模式升格后参与集恒为 {cand} 单员，arbitrate 单员 = cand.dims = 升格后 recalcNow 的 last，两值恒等——不为重算尺寸前移 addMember/recalcNow（trySend 失败路径免参与集回滚）；旁观期缩窗瞬态偏差由 05-08 前端升格 refit→RESIZE 纠正链收口
- **推送通道 D-13 双档不漂移**：每客户端按当前生效 mode 组帧（ro 推送永不含 osc52）；trySend 失败走 kickOrCreditLocked（连 ~100B 推送都容不下 = 事实 stalled，R-08 分工复用）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestAllPolicy 适配 G-05-1 运行期推送（planner 回归自检遗漏面）**
- **Found during:** Task 2（推送挂点实现后的既有测试回归面排查）
- **Issue:** plan step 5 回归自检覆盖 S1 owner 模式形态（last 不变零推送）与 TestResizeArbitration（推送落未读连接），但遗漏 TestAllPolicy：all 模式 A(80x24)/B(132,43)，A detach → 参与集 2→1 收缩 → recalcNow target {132,43} ≠ last {80,24} → 推送 Welcome 落在 B 的后续读流上 → accumPayload 对非 OUTPUT 帧 fatal（确定性失败：detach 本地 µs 级 vs echo 经 inputQ+cat 往返 ms 级，推送恒先到）
- **Fix:** 测试改为显式消费并断言该推送帧——mode 恒 rw（all 模式无升格 = mode 不翻转的行为证据，原测试「无升格帧」意图的 G-05-1 映射）且 cols/rows == 132/43（2→1 last-wins 新会话尺寸），随后 INPUT echo 断言形态不变
- **Files modified:** internal/server/multi_test.go
- **Verification:** TestAllPolicy -race 通过；全量 -race 零回归
- **Committed in:** `9cc76f4`（Task 2 commit）

**2. [Rule 1 - Bug] plan 字面 all 模式子测尺寸算术自相矛盾，按明示意图修正**
- **Found during:** Task 2（TestWelcomeSessionDims all 模式子测首跑失败：B welcome dims = 60x20, want 60x24）
- **Issue:** plan 字面「B(60,20) 使 min(132,60)xmin(43,24)=60/24 ≠ B 自身 60/20」算术矛盾——B(60,20) 时 min(43,20)=20，min-rect = {60,20} 恰等于 B 自身尺寸，无区分度；期望值 60/24 对应 B.rows=24 的 min 计算，与其声称的 B(60,20) 亦矛盾
- **Fix:** 按 plan 明示意图（「rows 维产生区分度断言」——会话尺寸 ≠ B 自身尺寸的 rows 维差异）取 B(60,50)：min(132,60)×min(43,50) = {60,43} ≠ B 自身 {60,50}，rows 维 43≠50 区分度成立；测试注释内登记修正理由
- **Files modified:** internal/server/multi_test.go
- **Verification:** TestWelcomeSessionDims/all模式attach携min-rect重算尺寸 通过
- **Committed in:** `9cc76f4`（Task 2 commit）

---

**Total deviations:** 2 auto-fixed (2 Rule 1 - Bug)
**Impact on plan:** 两处均为 plan 字面与自身意图/既有行为的机械矛盾修正，契约面（Welcome 恒携 cols/rows、推送单挂点、五调用点覆盖）与全部 must_have truths 逐字落地，无范围蔓延。

## Issues Encountered

- 编辑 resize_arb_test.go 时首次 Edit 误吞前一子测收尾段（old_string 边界取值过宽）——立即核读文件发现并恢复「owner模式参与集与ro忽略闸」子测的 A detach/递补/pollSize 收尾，go vet + 全量测试确认结构复原。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **05-11（前端视口约束）数据源就绪**：Welcome 三通道恒携会话 cols/rows——前端 main.ts 消费 wm.cols/wm.rows 维护 sessionDims，同 cols 渲染同字节流 = 异尺寸双端逐屏严格一致；旧服务端缺席键 → 不约束渲染的双向兼容分支已按契约预留
- **05-12 及后续**：推送挂点单点化（recalcNow 内）使任何未来参与集/策略变化自动继承尺寸下发
- 无阻塞项；威胁模型三条目（T-05G-01/02/03）形态与 plan 登记一致，无新增安全面（零新依赖，供应链闸不触发）

---
*Phase: 05-multi-client*
*Completed: 2026-08-22*

## Self-Check: PASSED

- 全部 10 个修改文件与 05-10-SUMMARY.md 落盘确认（FOUND）
- Task 提交 75e4def / 9cc76f4 均在 git log 确认（FOUND）
- 验证证据：go build/vet 干净；go test ./... -race -count=1 全包 ok（38.2s）
