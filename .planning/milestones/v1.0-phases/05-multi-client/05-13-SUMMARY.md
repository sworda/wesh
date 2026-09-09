---
phase: 05-multi-client
plan: 13
subsystem: server
tags: [resize-arbitration, credit-gate, backpressure, websocket, white-box-test, gap-closure]

requires:
  - phase: 05-multi-client
    provides: 05-10 G-05-1 会话尺寸下发三通道（WelcomeFrame 4 参签名）、05-04 arbiter/参与集、05-02 信用门/afterDrain、05-REVIEW WR-01/WR-02 逐字补丁
provides:
  - pushSessionDimsLocked 推送循环内踢出后的 arbiter.last 复检（stale 扇出中止——留存端终值恒 == 最新仲裁尺寸）
  - afterDrain 恢复开门时补发当前会话尺寸 Welcome（creditBlocked 期丢弃的尺寸推送帧收敛）
  - TestPushSessionDimsKickRecalc / TestAfterDrainResendsDims 两个白盒回归锁
affects: [05-multi-client 复验（gaps_found → gaps_closed 判据）, Phase 8 OPS-07 观测性]

actuals:
  tokens: 4810
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "推送循环内嵌套重算复检模式：跨层嵌套状态推进后，外层捕获形参以单一事实源（arbiter.last）复检中止 stale 扇出"
    - "信用门恢复补发模式：blocked 期非暂存类帧的收敛出口 = afterDrain 清位后/Broadcast 前按当前状态补发（outbox FIFO + hubMu 全程持有保证有序）"
    - "map 遍历随机序 × 循环内踢出交织的白盒复现：真实 conn 夹具（httptest+Dial）+ 至多 32 轮迭代命中危险序，不静默 skip"

key-files:
  created: []
  modified:
    - internal/server/resize.go
    - internal/server/clients.go
    - internal/server/resize_test.go
    - internal/server/clients_test.go

key-decisions:
  - "[Phase 05-13]: WR-01 修复取复检中止形态（05-REVIEW 逐字补丁）——嵌套推送已把 T2 送达全部留存端，stale 外层扇出直接 return；踢出不改仲裁或走信用路径时 last==target，外层零代价正确继续"
  - "[Phase 05-13]: WR-02 修复取 option (a)（用户 2026-08-22 裁决）——afterDrain 清位后、Broadcast 前补发当前 sessionDimsLocked() 的 Welcome，prefs 按 c.mode 选档（D-13 双档在补发通道不漂移）；补发有序性归因 = afterDrain 全程持有 hubMu + outbox FIFO（非门仍闭合）"
  - "[Phase 05-13]: 「触发帧不丢」承诺收窄为首帧暂存语义 + afterDrain 补发收敛——幂等置位守卫下已 blocked 的后续帧不覆写暂存（防二次暂存覆写首帧），尺寸推送类帧收敛出口 = afterDrain 补发，resize.go/clients.go 两函数注释互相指路"

patterns-established:
  - "嵌套重算复检：跨层嵌套调用可能推进共享状态后，外层以权威字段复检决定继续/中止"
  - "恢复点补发：信用/阻塞恢复点按当前状态补发阻塞期丢失的非暂存类控制帧"

requirements-completed: [MULTI-04, RES-04]

coverage:
  - id: D1
    description: "WR-01：pushSessionDimsLocked 推送循环内踢出后复检 arbiter.last != target 中止 stale 扇出——留存端最近一帧 Welcome 尺寸恒 == 最新仲裁尺寸"
    requirement: MULTI-04
    verification:
      - kind: unit
        ref: "internal/server/resize_test.go#TestPushSessionDimsKickRecalc（-race，B-first 危险序 32 轮内命中）"
        status: pass
      - kind: e2e
        ref: "node web/uat/phase05.mjs ./wesh（28/28 + 1 skip）+ phase05-dims.mjs（3/3）"
        status: pass
    human_judgment: false
  - id: D2
    description: "WR-02：afterDrain 恢复开门补发当前会话尺寸 Welcome（option (a)）——creditBlocked 期丢弃的尺寸推送帧收敛，选档双档不漂移"
    requirement: RES-04
    verification:
      - kind: unit
        ref: "internal/server/clients_test.go#TestAfterDrainResendsDims（-race，两子测：守卫不覆写暂存 + 补发收敛帧序/尺寸/选档）"
        status: pass
      - kind: e2e
        ref: "time go test ./... -race -count=1（5/5 包绿）+ 三层 UAT 28+1skip/19/3 零回归"
        status: pass
    human_judgment: false

duration: 22min
completed: 2026-08-22
status: complete
---

# Phase 05 Plan 13: WR-01/WR-02 尺寸推送与信用门交界面两缺陷闭合 Summary

**pushSessionDimsLocked 嵌套重算复检中止 stale 扇出（WR-01）+ afterDrain 开门补发当前会话尺寸 Welcome（WR-02 option (a)），两个白盒交织回归锁，全量 -race 5/5 包与三层 UAT 零回归**

## Performance

- **Duration:** 22 min
- **Started:** 2026-08-22T10:49:05Z
- **Completed:** 2026-08-22T11:10:46Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- **WR-01 闭合（Blocker）**：pushSessionDimsLocked 循环内 `kickOrCreditLocked` 返回后复检 `if s.arbiter.last != target { return }`（05-REVIEW 逐字补丁）——推送循环内踢出经 clients.go:479-480 removeMember→嵌套 recalcNow 把仲裁推进到 T2 并把 W(T2) 送达全部留存端的交织下，外层 stale T1 扇出直接中止，任何留存端持有的最近一帧 Welcome 尺寸恒 == 最新仲裁尺寸；resize.go 安全性注释改写为覆盖真实可达的 removeMember 路径（原 promoteNextLocked 路径实际不可达，压缩为一个从句），「触发帧不丢」承诺收窄为首帧暂存 + afterDrain 补发收敛
- **WR-02 闭合（option (a)，用户裁决）**：afterDrain 在 `c.creditBlocked = false` 之后、`hubCond.Broadcast()` 之前补发一帧当前 `sessionDimsLocked()` 的 Welcome（prefs 按 c.mode 选档，D-13 双档不漂移）——creditBlocked 期间被 `if !c.creditBlocked` 守卫静默丢弃的尺寸推送帧在恢复时收敛，该端 sessionDims 不再过期至下次尺寸事件；补发有序性归因落注释（afterDrain 全程持有 hubMu + outbox FIFO，按 plan-check 修订后的措辞）
- **两个白盒回归锁**：TestPushSessionDimsKickRecalc（真实 conn 夹具 + creackpty 活 master，至多 32 轮迭代命中 B-first 危险序，普适不变量末帧 == arbiter.last，修复前必败的测试牙齿）；TestAfterDrainResendsDims（守卫不覆写暂存语义锁 + 补发收敛锁：帧序/当前尺寸/rw-ro 选档区分度断言）
- **零协议面变化**：proto.go 未动、WelcomeFrame 4 参签名不变、无新帧类型、无新导出符号、前端（web/）零改动；全量 `go test ./... -race` 5/5 包绿、`go vet` 干净、phase05 三层 UAT（28/28+1skip、19/19、3/3）与前序 UAT（phase02 12/12、phase03 18/18、phase04 10/10、phase04-t1-width PASS）全绿

## Task Commits

Each task was committed atomically:

1. **Task 1: WR-01 — pushSessionDimsLocked 复检中止 stale 扇出 + 注释改写 + TestPushSessionDimsKickRecalc** - `74d1bff` (fix)
2. **Task 2: WR-02 — afterDrain 开门补发当前会话尺寸 Welcome（option (a)）+ TestAfterDrainResendsDims** - `4936f1c` (fix)

**Plan metadata:** 见末尾 final commit（docs: complete 05-13 plan）

## Files Created/Modified

- `internal/server/resize.go` — pushSessionDimsLocked 增 arbiter.last 复检；文档注释安全性段落改写（removeMember→嵌套 recalcNow 路径四要素论证 + 触发帧不丢承诺收窄）
- `internal/server/clients.go` — afterDrain 补发当前会话尺寸 Welcome（语句序：重投 creditPending → 清位 → gateTransitions++ → 补发 → Broadcast）；afterDrain/kickOrCreditLocked 文档注释同步（WR-02 语义 + 首帧暂存边界，两函数互指）
- `internal/server/resize_test.go` — 新增 TestPushSessionDimsKickRecalc（package server 白盒，httptest+websocket.Dial 真实 conn 夹具，creackpty.Open 活 master 吸收 TIOCSWINSZ，至多 32 轮迭代命中 B-first 交织）
- `internal/server/clients_test.go` — 新增 TestAfterDrainResendsDims（两子测：blocked 期后续推送帧不覆写暂存的守卫语义锁；afterDrain 补发收敛锁——表驱动 rw/ro 两行断言帧序/尺寸/prefs 选档）

## Decisions Made

- WR-01 修复严格按 05-REVIEW 逐字补丁形态（复检挂点唯一 = kickOrCreditLocked 返回后，信用路径 last 不变复检自然通过零代价）；不引入有序注册表结构——map 遍历随机性是测试覆盖交织的来源而非缺陷
- WR-02 修复严格按用户裁决的 option (a) 逐字补丁（`_ =` 形态保持——重投后 cur < cap/2 的数学保证下 ~100B Welcome 入队必成，失败分支属配置错误不兜底）；option (b) 注释收窄兜底方案作废未混用
- 测试夹具真实 conn（httptest+Dial）：kickSlowConsumerLocked 的异步 goroutine 调 `c.conn.Close(1013, ...)`，nil conn 会 panic 炸掉整个测试进程——plan 明示的红线，照此装配

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 测试夹具 registerLocked 调用点接收者修正**
- **Found during:** Task 1（TestPushSessionDimsKickRecalc 首次编译）
- **Issue:** plan 夹具描述的 `registerLocked(A)`/`registerLocked(B)` 为散文简写；`registerLocked` 实际定义在 `*registry` 上（clients.go:264），直写 `s.registerLocked(...)` 编译失败（`*Server` 无此方法）
- **Fix:** 改为 `s.registry.registerLocked(a/b)`（机械解析，与 TestClientCountInvariant 白盒先例同形态）
- **Files modified:** internal/server/resize_test.go（Task 2 的 clients_test.go 同样按此形态装配，一并适用）
- **Verification:** 编译通过，两测试 -race 全绿
- **Committed in:** 74d1bff / 4936f1c（随任务提交）

---

**Total deviations:** 1 auto-fixed（Rule 3 blocking，测试夹具机械修正）
**Impact on plan:** 纯散文简写→真实 API 形态的机械调和，零行为/零契约影响，无 scope creep。

## Issues Encountered

- Edit 工具对 clients.go afterDrain 注释区首次替换未命中（`// （本函数）` 行 `//` 后有一个空格，与凭记忆构造的 old_string 差一个字符）——按字节级核对实际文件内容后拆分小编辑完成，无语义影响。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 05-VERIFICATION.md gaps 块 missing 两条（WR-01/WR-02）全部落地，复验可由 gaps_found 转 gaps_closed：05-10 truth #5「任意时刻任意在线客户端持有的最近一帧 Welcome 尺寸 == 服务端会话尺寸」在全部已核读交织下成立
- 范围红线保持：IN-01..IN-04 打磨项未混入（REVIEW 挂账不动）；proto.go/前端/UAT 脚本零改动
- 无新增阻塞项；outbox 容量/水位/strikes 参数标定仍为 Phase 9 既定挂账（与本 plan 无关）

## Self-Check: PASSED
