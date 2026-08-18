---
phase: 04-frontend
plan: 04
subsystem: testing
tags: [uat, websocket, protocol, prefs, osc52, e2e, node, zero-dependency]

# Dependency graph
requires:
  - phase: 03-auth
    provides: phase03.mjs 零依赖 UAT 骨架（startWesh/spawnExpectExit/dialHello/check/出口码）与单次语义纪律、detail 红线先例
  - phase: 04-frontend
    provides: 04-01 落地的 --client-option/--osc52 两 flag、Welcome prefs 通道（omitempty）、白名单校验与启动拒绝文案（本 plan 全部断言对象）
provides:
  - web/uat/phase04.mjs 十场景协议断言脚本（S1-S6 六正 + E1-E4 四负，零依赖 Node >= 22）
  - FE-07 wire 可断言面全锁：omitempty 缺席回归/逐键注入/theme 对象/osc52 下发/组合/last-wins/四类启动拒绝
  - wire 兼容第三方证据：phase02.mjs 11/11 + phase03.mjs 18/18 零修改全绿
affects: [04-05, 04-06, phase-5-multi-client]

# Actuals (#2632) — chars/4 over realized diff，与 plan estimate（20000, confidence: low）配对校准
actuals:
  tokens: 3083   # web/uat/phase04.mjs 12330 chars / 4（新文件 264 行）
  tasks: 2
  commits: 1     # Task 2 为纯验证任务（脚本一次全绿零文件变更），无任务提交

# Tech tracking
tech-stack:
  added: []  # 零新依赖（node:child_process 内建模块 + Node 原生 WebSocket/fetch）
  patterns:
    - "prefs 形状断言形态：check detail 只打键形状/布尔/状态码，值内容永不进测试输出（phase03.mjs 红线向 prefs/theme 用户配置延伸）"
    - "启动拒绝 spawn-exit 矩阵复用：parse 期 fail-fast 场景走 spawnExpectExit 3s 自退形态（E1-E4 单函数分组，phase03 场景 6 先例）"

key-files:
  created:
    - web/uat/phase04.mjs
  modified: []

key-decisions:
  - "S 场景 spawn 命令尾补 '-- bash --norc --noprofile'——plan 字面 startWesh([]) 为简写（无命令 wesh exit 2 无法到 Welcome），照 phase03.mjs 全场景既定形态补全；断言面零影响"
  - "ws.close() 后 await waitClose(ws, 3000) 落定关闭握手再 SIGKILL——plan 骨架件 waitClose 的实际挂点，零断言纯收口"

patterns-established:
  - "UAT 新 phase 脚本起步式：逐字复用前一 phase 骨架件 + 单次语义独立 spawn + detail 红线注释头——phase04.mjs 与 phase03.mjs 同构"
  - "last-wins/组合等聚合语义经独立实例 WS 握手断言（不经单测之外的第二条通道）"

requirements-completed: [FE-07]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "phase04.mjs 十场景全绿：Welcome prefs 六正形状（omitempty 缺席回归/两键注入/theme 对象/osc52 下发/组合/last-wins）+ client-option 四负启动拒绝（白名单外 key/非法 JSON/osc52 key/缺 '='）"
    requirement: FE-07
    verification:
      - kind: e2e
        ref: "node web/uat/phase04.mjs /tmp/wesh-uat/wesh（10/10 PASS，exit 0）"
        status: pass
    human_judgment: false
  - id: D2
    description: "wire 兼容回归：Welcome 加可选 prefs 键后旧 UAT 脚本零修改全绿（旧前端零漂移的第三方证据）"
    requirement: FE-07
    verification:
      - kind: e2e
        ref: "node web/uat/phase02.mjs /tmp/wesh-uat/wesh（11/11 PASS，exit 0）"
        status: pass
      - kind: e2e
        ref: "node web/uat/phase03.mjs /tmp/wesh-uat/wesh（18/18 PASS，exit 0）"
        status: pass
    human_judgment: false

# Metrics
duration: 6min
completed: 2026-08-18
status: complete
---

# Phase 4 Plan 04: FE-07 协议层自动化 UAT Summary

**Phase 4 协议层 UAT 十场景一次全绿：Welcome prefs 六正形状断言（omitempty 缺席回归/逐键注入/theme 对象/osc52 下发/组合/last-wins）+ client-option 四负启动拒绝矩阵，phase02/phase03 旧脚本零修改回归全绿构成 wire 兼容第三方证据**

## Performance

- **Duration:** 6 min
- **Started:** 2026-08-18T16:46:51Z
- **Completed:** 2026-08-18T16:52:49Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- `web/uat/phase04.mjs`（264 行，零依赖）十场景全绿：S1-S6 覆盖 Welcome prefs 全部 wire 可断言面，E1-E4 覆盖 --client-option 启动校验拒绝面（含 D-12 osc52 安全不对称）
- detail 红线延伸落地：全部 10 个 check 的 detail 只含键形状/布尔/退出码/错误类别布尔，prefs/theme 值内容零进输出（T-04-11 缓解锁定）
- wire 兼容证据：同一二进制上 phase02.mjs 11/11、phase03.mjs 18/18 零修改全绿——Welcome 加可选键对旧断言零漂移

## Task Commits

Each task was committed atomically:

1. **Task 1: web/uat/phase04.mjs——六正四负场景脚本** - `f0e8438` (test)
2. **Task 2: 二进制构建接入与十场景全绿** - 无提交（纯验证任务：脚本按 Task 1 落盘形态一次全绿，零文件变更；测试二进制落 /tmp/wesh-uat/wesh 临时路径，dist 重建字节一致无漂移）

**Plan metadata:** 见尾部 docs 提交

## Files Created/Modified
- `web/uat/phase04.mjs` - Phase 4 协议层 UAT：S1-S6 Welcome prefs 形状断言 + E1-E4 启动拒绝矩阵（逐字复用 phase03.mjs 骨架件，无新导出符号）

## Decisions Made
- **S 场景命令尾补全**：plan 字面 `startWesh([])`/`startWesh(['--client-option',...])` 为简写——无命令时 wesh parse 期 exit 2（missing command）永远到不了 Welcome；照 phase03.mjs 全场景既定形态补 `'--', 'bash', '--norc', '--noprofile'`（`--norc --noprofile` 隔离用户 rc 文件）。E 场景参数与 plan 逐字一致。
- **waitClose 挂点**：plan 骨架件清单含 waitClose——挂于每个 S 场景 `ws.close()` 之后（`await waitClose(ws, 3000)`），让关闭握手落定再由 finally SIGKILL 收口；零断言，不产生新验收面。

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None — Task 1 脚本一次写成，Task 2 构建（pnpm 1.7s + go build 0.6s）与三脚本运行全部首跑通过，无调试往返、无修复重跑。

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- 04-05（前端 prefs 消费/URL query 覆盖）与 04-06（渲染层人工 UAT 清单）的协议面基线就绪：Welcome prefs 形状、osc52 下发、启动拒绝文案均有自动化回归锁
- FE-07 由 04-01/04-04/04-05/04-06 四个 plan 共同声明——REQUIREMENTS.md 的 Complete 标记受 shared-ID gate 约束，待 04-05/04-06 SUMMARY 齐备后经 ready-ids 解除
- 无阻塞项

## Self-Check: PASSED

- 文件存在性：`web/uat/phase04.mjs`（264 行）落盘确认
- 提交存在性：`f0e8438`（Task 1 test）在 git log
- 验收重跑：`node web/uat/phase04.mjs /tmp/wesh-uat/wesh` 10/10 PASS exit 0；`phase02.mjs` 11/11、`phase03.mjs` 18/18 回归 exit 0（输出全文见执行记录）
- 红线静态审计：check 调用无值内容字面量（grep 复核 `#000000`/`18`/`22` 等仅出现于断言表达式与 spawn 参数，不进 detail/name 打印串）；仅 `node:child_process` 单 import
- 构建产物时间戳核实：/tmp/wesh-uat/wesh 2026-08-18T16:50:29Z（当次构建）；tracked 工作树构建后零漂移（dist/index.html 字节一致，.gz 按既定 .gitignore 策略不入库）

---
*Phase: 04-frontend*
*Completed: 2026-08-18*
