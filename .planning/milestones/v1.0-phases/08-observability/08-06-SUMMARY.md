---
phase: 08-observability
plan: 06
subsystem: docs/testing
tags: [journald, systemd, jq, slog, uat, gap-closure]
gap_ids: [G-08-2]

# Dependency graph
requires:
  - phase: 08-observability (08-01/08-02)
    provides: slog JSON 事件流（D-18 msg=event schema / D-20 client_id / D-21 detach reason / D-23 认证字段边界）
  - phase: 08-observability (08-05)
    provides: phase08.mjs UAT 骨架件（startWesh/waitEvent/dialHello/check/assertOutputClean）
provides:
  - G-08-2 闭合：README 两则 journald jq 示例在 systemd 默认配置（StandardOutput=journal）下复制粘贴即可运行
  - web/uat/phase08-journal.mjs 合流模拟回归夹具（负对照自证 + 全流纯度 + 两则示例管道端到端）
  - README:336 D-13 迁移前文本事件行示例 JSON 化（08-VERIFICATION Anti-Patterns 收口）
affects: [09, verify-work, uat-resume]

# Actuals (#2632) — chars/4 over the realized diff，与 plan estimate 同尺度
actuals:
  tokens: 5538
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []  # 零新增依赖（Node stdlib + 系统既有 jq/grep，T-08-06-SC accept 成立）
  patterns:
    - "journal 合流模拟夹具：spawn stdio 分离捕获 → 进程 'close' 收口 → stdout+stderr 按 journal 时序拼接经 stdin 喂入 /bin/sh -c 管道断言（journalctl 段 stdin 等价替换）"
    - "README 示例 ↔ 测试脚本防漂移条款：逐字一致范围 = 管道形态 + grep 防护段 + 引号形态；夹具确定性 select 字面量显式豁免（脚本 ==1 vs README ==7 不得互改）"

key-files:
  created:
    - web/uat/phase08-journal.mjs
  modified:
    - README.md

key-decisions:
  - "G-08-2 修复取 README 侧（plan 方向 3）：示例统一 grep '^{' 预滤 + 合流机理说明；wesh 源码与启动输出零改动（D-14/D-15/D-16 锁定决策的推论）"
  - "进程收口取 child 'close' 事件形态（plan『waitExit 等价形态』字面）——'exit' 先于 stdio 流 flush 的竞态由 'close' 结构性消除，合流流构造前捕获全文落定"

patterns-established:
  - "合流流纯度机械证明形态：grep '^\\{' 预滤后 jq . 退出 0 且 stderr 空 ≡ 每行皆合法 JSON；'{' 起始行数 ≥4 防 vacuous 通过"
  - "负对照自证纪律：夹具必须先复现原缺陷形态（无防护管道必 parse error）再证修复面，防空转通过"

requirements-completed: [OPS-08]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "README 结构化日志节两则 journald 示例补 grep '^\\{' 防护段 + 引言句改写为合流机理说明（G-08-2 修复面）；README:336 文本事件行 JSON 化"
    requirement: OPS-08
    verification:
      - kind: other
        ref: "grep 断言组：防护段 ≥2 / 无未防护 journalctl 行 / ^wesh: 文本事件行清零 / select(.client_id==7) 字面量保持"
        status: pass
      - kind: e2e
        ref: "node web/uat/phase08-journal.mjs（J2 全流纯度 / J3 示例一 / J4 示例二——README 逐字管道在合流流上端到端）"
        status: pass
    human_judgment: false
  - id: D2
    description: "web/uat/phase08-journal.mjs 合流模拟回归夹具（负对照不空转 / 全流纯度 / 两则示例 / assertOutputClean 红线自证）"
    requirement: OPS-08
    verification:
      - kind: e2e
        ref: "node web/uat/phase08-journal.mjs — 6/6 全绿（J0-J4 + SEC）"
        status: pass
      - kind: integration
        ref: "node web/uat/phase08.mjs — 21/21 回归零破坏；go test -race -count=1 ./... 五包全绿"
        status: pass
    human_judgment: false
  - id: D3
    description: "真实 journald 复测：sg systemd-journal 代跑 README 两则新示例（jq 零 parse error、检出既存 auth_failed 与 attach/detach 对）"
    requirement: OPS-08
    verification: []
    human_judgment: true
    rationale: "依赖真实 systemd/journald 环境的检索面复核；plan verification 第 4 条明示为 UAT resume 项、非本 plan 阻塞（A2 blocked 点已随用户加组解除，事件在 journal 可回溯，无需重制）"

# Metrics
duration: 9min
completed: 2026-08-28
status: complete
---

# Phase 8 Plan 06: G-08-2 闭合（README journald 示例防护 + 合流回归夹具）Summary

**README 两则 journald jq 示例补 `grep '^\{'` 预滤 + 合流机理说明使 systemd 默认配置下复制粘贴即可运行，新增 phase08-journal.mjs 合流模拟回归夹具（负对照自证不空转）6/6 全绿，wesh 源码零改动**

## Performance

- **Duration:** 9 min
- **Started:** 2026-08-28T11:27:49Z
- **Completed:** 2026-08-28T11:36:45Z
- **Tasks:** 2
- **Files modified:** 2（1 修改 + 1 新建）

## Accomplishments

- **G-08-2 闭合（major gap）**：README「结构化日志」节引言句改写为合流机理说明（systemd 默认 `StandardOutput=journal` 把 stdout 启动横幅与 stderr JSON 事件合流，jq 遇非 JSON 行中止管道），两则示例统一插入 `| grep '^\{'` 预滤段——与自动化测试 parseEvents 滤行约定同款；select 示例数字 `==7` 保持原样（示例 N 豁免）
- **合流模拟回归夹具**：`web/uat/phase08-journal.mjs`（342 行零依赖）spawn 真实二进制分离捕获 stdout/stderr，按 journal 时序拼接后经 stdin 喂入 `/bin/sh -c` 管道断言——①负对照证明夹具确复现原缺陷形态（无防护管道 exit=4 且 stderr 含 parse error）②全流纯度（防护后 jq . 零 parse error，`{` 起始行恰 4 条防 vacuous）③示例一 auth_failed 恰 1 行零 user/username 键（D-23）④示例二 attach+detach 恰 2 行 client_id 均 1、reason=normal（D-20/D-21）；grep+jq 段与 README 新示例逐字一致
- **顺手收口**：README:336 D-13 迁移前文本事件行示例（`wesh: close remote=…`）整块替换为 JSON detach 事件行——字段序与 emitDetachLocked（clients.go:762）同形（time/level/msg → event→remote→client_id→code→reason→末位 remote_user），围栏改 json（08-VERIFICATION Anti-Patterns 登记项）
- **回归零破坏**：phase08.mjs 21/21 全绿；`go test -race -count=1 ./...` 五包全绿；两次提交 diff 边界 = README.md + 新脚本，wesh 源码与启动输出零改动（D-14/D-15/D-16 保持）

## Task Commits

Each task was committed atomically:

1. **Task 1: README 结构化日志节——journald 示例补 grep 防护 + 合流机理说明** - `f19811b` (docs)
2. **Task 2: 合流模拟回归脚本 web/uat/phase08-journal.mjs（负对照 + 两则示例管道端到端）** - `9ebbbe4` (test)

## Files Created/Modified

- `README.md` - 结构化日志节引言句改写 + 两则 journald 示例插入 grep 防护段（行 459-466 区域）；反代身份透传节文本事件行示例 JSON 化（行 336 区域）；全文件其余零改动（5+/5-）
- `web/uat/phase08-journal.mjs` - 新建：G-08-2 合流模拟回归夹具，骨架件逐字复用 phase08.mjs（startWesh/waitEvent/dialHello/check/assertOutputClean），jq 缺失 skipped 豁免兜底

## Decisions Made

- **进程收口取 child 'close' 事件**（plan「waitExit 等价形态」字面内的执行选型）：合流流构造前必须保证 stdout/stderr 捕获全文落定，`'exit'` 事件先于 stdio 流 flush 的竞态由 `'close'`（进程退出且流全闭）结构性消除，恒带 5s 超时护栏（waitExit 同款纪律）
- **J2 防 vacuous 断言取 `{` 起始行计数 ≥4**（JS 侧镜像 grep 语义，与 jq 输出格式无关）：夹具事件 session_start/auth_failed/attach/detach 恰 4 条，纯度断言（jq . 退出 0 + stderr 空）在零行入选时平凡为真，计数下界是「夹具自证不空转」must_have 的机械兑现
- 其余零新增决策——plan 逐字执行（修复方向、豁免范围、禁止项均已在 plan 内锁定）

## Deviations from Plan

None - plan executed exactly as written.

（Task 2 的 waitProcClose 与 J2 行数下界均在 plan 明示的裁量字面内——「waitExit 等价形态」「夹具自证不空转」——不构成偏差。）

## Issues Encountered

None。首跑全绿：Task 1 grep 断言组一次通过；Task 2 脚本首跑 6/6（含负对照 exit=4 parse error 实测——jq 1.6 行为与 UAT A2 实机记录一致）；两项回归（phase08.mjs 21/21、go test -race 五包）零破坏。

## Known Stubs

None——README 示例与脚本全部真实接线（脚本经真实二进制端到端驱动，无 mock/占位）；jq 缺失 skipped 兜底为 plan 明示的实机工具豁免形态（与浏览器层 UAT skipped 纪律一致），非 stub。

## User Setup Required

None - no external service configuration required.

（UAT resume 提示：真实 journald 复测项见 08-UAT.md A2——用户已加 systemd-journal 组，sg 代跑 README 两则新示例即可，既存事件在 journal 可回溯无需重制；coverage D3 已登记 human_judgment。）

## Next Phase Readiness

- Phase 08 全部 6 plan 执行完毕（08-01..08-06），G-08-2 闭合待 /gsd-verify-work 对账 resolved（frontmatter `gap_ids: [G-08-2]`）
- 阻塞/挂账零新增；08-UAT A2 的真实 journal 复测为唯一残余人工项（非阻塞）
- wesh 源码零改动 ⇒ Phase 9（自定义首页/单二进制发布/参数标定）无本 plan 引入的前置清理

---

*Phase: 08-observability*
*Completed: 2026-08-28*

## Self-Check: PASSED

- 文件存在：README.md / web/uat/phase08-journal.mjs / 08-06-SUMMARY.md 全部 FOUND
- 提交存在：f19811b（Task 1）/ 9ebbbe4（Task 2）git log 全部 FOUND
- 关键内容：README 两则示例防护段逐字在场（示例二 `==7` 字面量保持）
- 验证证据：`node web/uat/phase08-journal.mjs` 6/6、`node web/uat/phase08.mjs` 21/21、`go test -race -count=1 ./...` 五包全绿
