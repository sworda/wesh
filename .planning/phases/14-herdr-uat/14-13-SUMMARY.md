---
phase: 14-herdr-uat
plan: 13
subsystem: docs
tags: [mermaid, documentation, validation, jsdom, gap-closure]

# Dependency graph
requires:
  - phase: 14-herdr-uat (14-11)
    provides: PC-12 文档三件套之 ARCHITECTURE.md 双模式架构节（含两个 mermaid 拓扑图——本 plan 修复其组件图词法错误）
provides:
  - docs/ARCHITECTURE.md 组件图渲染修复（4 subgraph 合法 id + 引号标题 + 12 处带标签边引号化）——G-14-34 闭合，UAT test 34 转 pass（34/34）
  - scripts/check-mermaid.mjs mermaid 词法校验载具（jsdom + mermaid.parse 11.17.2，负对照自证）——堵 14-11 结构化校验不查词法的漏检面，后续文档 mermaid 改动的常驻复验通道
affects: [后续 docs/ mermaid 改动（check-mermaid.mjs 复验义务）, Phase 14 VERIFICATION（test 34 终局对账）]

# Actuals — pairs with plan estimate (15000 tokens, low confidence)
actuals:
  tokens: 13400   # 代码面 diff 45596 chars + 本轮 .planning 收口文件 ~8k chars，/4
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: [mermaid ^11.17.2（scripts/ dev 校验面）, jsdom ^30.0.1（scripts/ dev 校验面）]
  patterns: [负对照自证（修复前 exit 1 → 修复后 exit 0 红绿链，14-08 D3 先例延续）, jsdom 注入链动态 import mermaid（Linux 禁浏览器约束下渲染合法性的 Node 端等价物）]

key-files:
  created: [scripts/check-mermaid.mjs, scripts/package.json, scripts/pnpm-lock.yaml]
  modified: [docs/ARCHITECTURE.md]

key-decisions:
  - "修复方案逐字采用 UAT missing 钦定 + /tmp/mermaid-check 草案 v2 预验证形态（VERBATIM-MATCH）——subgraph id 是 mermaid 内部标识、渲染显示文本来自引号标题串，语义零漂移；与第二块 L98 规范风格统一"
  - "词法校验载具落 scripts/ 局部安装（pnpm + lockfile 冻结）不进 CI/生产——14-12 收口闸已定不重开验证面，载具为手动可重跑工具"

patterns-established:
  - "负对照先例第三例（14-08 D3 → 14-13 Task 1）：校验工具落地必须先证判别力（对坏输入真实报红）再承载修复红绿链"
  - "mermaid 图改动复验通道：node scripts/check-mermaid.mjs（默认扫 docs/*.md 全部块，任一 FAIL exit 1）"

requirements-completed: [PC-12]  # PC-12 已于 14-12 勾选收口；本 plan 为其文档渲染面的 gap closure（G-14-34），不重复勾选

coverage:
  - id: D1
    description: "mermaid 词法校验载具 scripts/check-mermaid.mjs（jsdom 注入链 + 逐块 mermaid.parse，含负对照自证）"
    requirement: PC-12
    verification:
      - kind: other
        ref: "node scripts/check-mermaid.mjs（Task 1 负对照 NEGATIVE-CONTROL-OK：修复前块 1 FAIL exit 1 + TESTING.md 单独 exit 0；修复后全 3 块 PASS exit 0）"
        status: pass
    human_judgment: false
  - id: D2
    description: "docs/ARCHITECTURE.md 组件图渲染修复（G-14-34：subgraph 合法 id + 边标签引号化，GitHub/渲染器正常渲染）"
    requirement: PC-12
    verification:
      - kind: other
        ref: "node scripts/check-mermaid.mjs → ARCHITECTURE 块 1/块 2 + TESTING 块全 PASS exit 0（flowchart-v2，FIXED-GREEN；diff 零回归门：删/增各恰 16 行、hunk 域 L11-56）"
        status: pass
      - kind: manual_procedural
        ref: "Task 3 human-verify checkpoint：GitHub/渲染器目检 approved（2026-09-08 用户确认）"
        status: pass
    human_judgment: true
    rationale: "mermaid 渲染为渲染器观感面，Linux 开发机禁浏览器（CODEBUDDY.md 双机拓扑硬约束）无自动化等价物；Node 端 mermaid.parse 为词法层等价证，最终观感需人工目检——Task 3 blocking 门用户已 approved"

# Metrics
duration: 35min（执行器纯时长，不含 Task 3 人工门等待间隔）
completed: 2026-09-08
status: complete
---

# Phase 14 Plan 13: G-14-34 mermaid 组件图词法修复 Summary

**ARCHITECTURE.md 组件图 16 行词法修复（subgraph 合法 id + 引号标题 + 12 处边标签引号化含 L41 `{token}` DIAMOND_START 主修复点）红转绿，词法校验载具 check-mermaid.mjs 常驻固化，用户渲染目检 approved——UAT gap G-14-34 / test 34 闭合（34/34）**

## Performance

- **Duration:** 35min 执行器纯时长（Task 1+2 于 11:00-11:17 +0800 完成后 blocking 人工门等待；本轮 continuation 收口 ~5min）——人工门等待间隔不计
- **Started:** 2026-09-08T03:00Z 前后（前次 executor 运行）
- **Completed:** 2026-09-08T03:35Z（continuation 收口）
- **Tasks:** 3/3（Task 1 auto / Task 2 auto / Task 3 human-verify approved）
- **Files modified:** 5（scripts/check-mermaid.mjs + scripts/package.json + scripts/pnpm-lock.yaml + docs/ARCHITECTURE.md + .planning/phases/14-herdr-uat/14-UAT.md）

## Accomplishments

- **词法校验载具固化（Task 1，`68ea29c`）**：scripts/check-mermaid.mjs（69 行）收编 /tmp/mermaid-check 预验证形态——jsdom 构造 DOM 注入 globalThis.window/document 后 dynamic import('mermaid')（顺序不可颠倒），逐块提取 ```mermaid 围栏调用 mermaid.parse，块级 PASS/FAIL 判定 + 退出码门（任一 FAIL exit 1）。负对照自证 NEGATIVE-CONTROL-OK：对修复前文档报 ARCHITECTURE 块 1 FAIL（Lexical error / DIAMOND_START 类）+ exit 1，TESTING.md 单独跑 exit 0——判别力先于修复落地（14-08 D3 先例延续），root_cause 指出的 14-11 结构化校验漏检面（只查结构完整性不查词法）自此有常驻复验通道。依赖 mermaid ^11.17.2 + jsdom ^30.0.1 钉版 + pnpm-lock.yaml 冻结，仅 scripts/ 局部安装不进 CI/生产。
- **组件图修复红转绿（Task 2，`17962d5`）**：docs/ARCHITECTURE.md 组件图（第一块 L11-56）16 行改动——4 处 subgraph 改合法 id + 引号标题（CMDWESH["cmd/wesh（CLI 装配层）"]/SRV/PTYLAYER/WEBPKG，id 为 mermaid 内部标识、显示文本来自引号标题，语义零漂移）+ 12 处带标签边统一 `-->|"..."|` 引号形态（L41 `GET / · /s/{token}/` 的 `{` DIAMOND_START 误解析为主修复点，其余 11 处风格统一 + 特殊字符免疫），与第二块 L98 双模式拓扑图规范风格统一。修复草案与 /tmp/mermaid-check/fix-check.mjs 草案 v2 逐字比对 VERBATIM-MATCH。修复后 check-mermaid.mjs 3 块全 PASS（两块 flowchart-v2 + TESTING 1 块）exit 0——FIXED-GREEN。
- **渲染目检人工门 approved（Task 3）**：blocking human-verify checkpoint，用户在 GitHub/渲染器目检确认：4 个 subgraph 框成形（含全角括号标题）、边箭头完整、`GET / · /s/{token}/` 边标签完整显示、无 mermaid 语法错误占位；第二块双模式拓扑图渲染如常（零回归）。UAT test 34 由 issue 转 pass（34/34 全过），**gap G-14-34 闭合**。
- **continuation 终局绿确认**：收口前重跑 `node scripts/check-mermaid.mjs` → docs/ 全部 mermaid 块 3 块 PASS exit 0（ARCHITECTURE×2 flowchart-v2 + TESTING×1；CONFIGURATION/DEPLOYMENT/DEVELOPMENT/GETTING-STARTED 无块跳过）。

## Task Commits

Each task was committed atomically:

1. **Task 1: 固化 mermaid 词法校验载具（含负对照自证）** - `68ea29c` (feat)
2. **Task 2: 按预验证草案 v2 修复组件图（红→绿）** - `17962d5` (fix)
3. **Task 3: 渲染目检人工门** - 无代码提交（blocking checkpoint，用户 approved；结果记录于 14-UAT.md + 本 SUMMARY）

**Plan metadata:** 见本轮 docs(14-13)/docs(phase-14) 收口提交

## Verification Evidence Chain

1. **词法合法性（自动化）**：`node scripts/check-mermaid.mjs` → docs/ 3 块全 PASS（ARCHITECTURE 块 1/块 2 diagramType=flowchart-v2 + TESTING 块 1），exit 0（修复后两轮验证：Task 2 FIXED-GREEN + continuation 收口前复跑）。
2. **判别力（负对照）**：修复前同脚本对组件图块 1 报 FAIL + exit 1（Lexical error：三处 subgraph id 含 `/` + L41 `{` DIAMOND_START），TESTING.md 单独跑 exit 0——非恒绿实现（Task 1 NEGATIVE-CONTROL-OK）。
3. **改动纯净性（diff 零回归门）**：`git diff -U0 docs/ARCHITECTURE.md` 删/增行各恰 16（4 subgraph + 12 带标签边），全部 hunk 旧/新侧行域落在 L11-56 组件图块内（增删两侧对称断言）——节点声明行/5 条无标签边/第二块双模式拓扑图/围栏外正文零字节改动；ci.yml / web/ / internal/ 零 diff。
4. **渲染目检（人工门）**：Task 3 checkpoint approved——GitHub/渲染器中组件图 subgraph 成形、边完整、标签正常显示、无语法错误占位（G-14-34 truth 达成）。

## Files Created/Modified

- `scripts/check-mermaid.mjs` - mermaid 词法校验载具：jsdom 注入链 + 逐块 mermaid.parse + 块级 PASS/FAIL + exit-code 门（默认扫 docs/*.md）
- `scripts/package.json` - 载具依赖声明（private + type: module，mermaid ^11.17.2 + jsdom ^30.0.1）
- `scripts/pnpm-lock.yaml` - 依赖冻结（1182 行，dev 校验工具面）
- `docs/ARCHITECTURE.md` - 组件图（第一块）16 行词法修复；第二块与围栏外正文零改动
- `.planning/phases/14-herdr-uat/14-UAT.md` - test 34 转 pass（34/34）+ G-14-34 转 resolved（resolved_by 14-13-PLAN.md）

## Decisions Made

- 修复方案逐字采用 UAT missing 钦定 + 草案 v2 预验证形态（VERBATIM-MATCH），subgraph id 是 mermaid 内部标识、渲染显示文本来自引号标题串——语义零漂移且与第二块规范风格统一（plan 既定，无执行期新决策）
- 载具落 scripts/ 局部安装不进 CI（14-12 收口闸既定不重开验证面）；pnpm + lockfile 冻结（CODEBUDDY.md 工具偏好）

## Deviations from Plan

None - plan executed exactly as written.（Task 3 为 plan 内设计的 blocking 人工门，用户 approved 后由 continuation agent 收口——非偏差）

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 14 全部 13/13 plan 完成（12 executed + 14-13 gap closure）；UAT 34/34 全过、gap 清零——phase 终局对账（VERIFICATION）由 orchestrator 于本 SUMMARY 返回后处理
- 后续 docs/ mermaid 改动的复验义务通道：`node scripts/check-mermaid.mjs`（任一块 FAIL exit 1，负对照已自证判别力）

## Known Stubs

None——零 stub/占位/TODO 遗留。

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-08*
