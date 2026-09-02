---
phase: 04-frontend
plan: 02
subsystem: ui
tags: [xterm, unicode11, web-links, osc8, title-sync, sanitize, pnpm, supply-chain]

# Dependency graph
requires:
  - phase: 01-pty
    provides: Terminal 构造选项表（fontFamily 栈同源）、webgl/fit addon 加载段、main.ts 常驻接线形态
  - phase: 03-auth
    provides: WELCOME ro 分支现状（本 plan 改造点）、dist 产物入库纪律（03-05）
provides:
  - unicode11 加载-激活硬顺序接线（FE-02 宽度测量经 activeVersion='11' 生效）
  - web-links 默认通道 + OSC8 linkHandler 双通道链接化（FE-04）+ xterm-hover tooltip 展示真实 URL
  - sanitizeTitle 纯函数（C0/DEL/C1 剥离 + 128 code point 截断 + 'wesh' 回退）与 node --test 回归锁
  - setTitle() 单一写口 + isRO/remoteTitle 模块级状态（CORE-03；[ro] 前缀恒最前，兑现 P2 D-14 挂账）
  - web/pnpm-workspace.yaml overrides 钉 js-base64 3.9.2（pnpm 11 settings 新家）
affects: [04-03 粘贴门（isRO 复用）, 04-05 osc52 门/prefs 消费（isRO/ClipboardAddon 推迟导入）, 04-06 UAT 渲染层清单]

# Actuals (#2632) — chars/4 over realized diff，与 plan estimate（30000, confidence: low）配对校准
# 全量 diff 221802 中约 218K 为 dist/index.html 重建产物（487KB 单文件 minify 行）与 pnpm-lock.yaml；
# 源码级 diff（排除 dist/lockfile）为 13277 chars ≈ 3319 tokens
actuals:
  tokens: 221802
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added:
    - "@xterm/addon-unicode11@0.9.0"
    - "@xterm/addon-web-links@0.12.0"
    - "@xterm/addon-clipboard@0.2.0（本 plan 仅入包，条件加载在 04-05）"
    - "js-base64 override 钉 3.9.2（addon-clipboard 传递依赖，避 1 天新包 3.9.3）"
  patterns:
    - "unicode11 加载-激活硬顺序：loadAddon 紧随 activeVersion='11'（仅加载不激活等于没装）"
    - "链接双通道统一 tooltip：web-links hover/leave 与 OSC8 linkHandler.hover/leave 共用 showLinkTooltip/hideLinkTooltip"
    - "标题单一写口：onTitleChange → remoteTitle=sanitizeTitle(t) → setTitle() 拼 (isRO?'[ro] ':'')+remoteTitle"
    - "纯函数抽 lib/ + node --test 直跑 .ts（Node 24 type stripping 零新依赖，tsconfig exclude 测试文件）"
    - "pnpm 11 settings 落 pnpm-workspace.yaml（package.json pnpm 字段不再被读取）"

key-files:
  created:
    - web/src/lib/title.ts
    - web/src/lib/title.test.ts
    - web/pnpm-workspace.yaml
  modified:
    - web/package.json
    - web/pnpm-lock.yaml
    - web/src/main.ts
    - web/index.html
    - web/tsconfig.json
    - web/dist/index.html

key-decisions:
  - "js-base64 override 落 web/pnpm-workspace.yaml 而非 plan 字面的 package.json pnpm 字段——pnpm 11.21.0（CI 同钉）安装时明示 WARN 不再读该字段，overrides 新家即 pnpm-workspace.yaml；钉 3.9.2 意图逐字保持"
  - "ClipboardAddon 本 plan 只入包不导入——noUnusedLocals 下未用导入报错，条件加载随 04-05 随用随加"

patterns-established:
  - "瞬态元素 z-index 序注释：resize 浮层 900（04-03）< link tooltip 901 < 状态面板 1000"
  - "安全相关纯函数一律抽 web/src/lib/ 并配 node --test 回归锁（sanitize 不可旁路的测试证据）"

requirements-completed: [CORE-03, FE-02, FE-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "三 addon 精确钉版入包 + js-base64 override 钉 3.9.2（供应链完整，T-04-SC 缓解）"
    verification:
      - kind: other
        ref: "grep js-base64 web/pnpm-lock.yaml → 3.9.2（override 生效）；package.json 三包精确版无 ^ 前缀"
        status: pass
    human_judgment: false
  - id: D2
    description: "sanitizeTitle 契约：C0/DEL/C1 剥离 + 恰 128 code point 截断 + emoji surrogate pair 不拆 + 全控制/空串回退 'wesh'"
    requirement: CORE-03
    verification:
      - kind: unit
        ref: "web/src/lib/title.test.ts#node --test（7 用例全 PASS）"
        status: pass
    human_judgment: false
  - id: D3
    description: "dist 产物重建入库：含 xterm-hover/activeVersion 检索串（新接线入包证据），go build ./... embed 链编译通过，.gz 不入库"
    verification:
      - kind: other
        ref: "grep xterm-hover/activeVersion web/dist/index.html + go build ./...（退出 0）"
        status: pass
    human_judgment: false
  - id: D4
    description: "浏览器渲染面行为：CJK/emoji Unicode 11 宽度、中文/日文 IME 组合输入不丢字、链接 hover 真实 URL 与单击无 confirm 框、OSC 0/2 标题同步与 [ro] 前缀保持"
    requirement: FE-02
    verification: []
    human_judgment: true
    rationale: "真实 IME 栈与渲染层交互不可自动化（UI-SPEC 🧪 backstop 既定）；按 plan 约定并入 04-06 的 04-UAT.md 清单统一人工确认"

# Metrics
duration: 16min
completed: 2026-08-18
status: complete
---

# Phase 4 Plan 02: unicode11/web-links 双通道 + 标题单一写口 Summary

**xterm 三 addon 钉版入包（unicode11 0.9.0/web-links 0.12.0/clipboard 0.2.0 + js-base64 override 3.9.2），unicode11 加载-激活硬顺序落地 FE-02 宽度测量，文本 URL 与 OSC8 双通道链接化统一 hover 真实 URL tooltip，标题同步收敛为 sanitizeTitle→setTitle 单一写口且 [ro] 前缀恒最前**

## Performance

- **Duration:** 16 min
- **Started:** 2026-08-18T15:53:38Z
- **Completed:** 2026-08-18T16:10:20Z
- **Tasks:** 3
- **Files modified:** 9（含 3 新建）

## Accomplishments
- FE-02：`term.loadAddon(new Unicode11Addon())` 紧随 `term.unicode.activeVersion = '11'`（硬顺序注释引 RESEARCH §Pitfall 2）；IME 组合输入走 xterm 内建 composition 既定路线，输入链路零改动
- FE-04：web-links 保持库默认正则/handler（0.12.0 产物实为仅 http(s)，D-05/D-07）；OSC8 显式 `term.options.linkHandler`（activate 置 opener=null 等价 rel=noopener，不设则回退 confirm() 原生框）；`allowNonHttpProtocols` 默认 false 结构性忽略 javascript:/file:
- FE-04 tooltip：双通道统一 xterm-hover div（term.element 内，核心 hover 路径对该 class 提前 return 防抖动），完整 URL 原文无前缀不截断（max-width 480px + break-all），+8px 偏移视口边缘翻转，z-index 901
- CORE-03：`sanitizeTitle`（C0/DEL/C1 剥离 + Array.from 128 code point 截断 + 'wesh' 回退）经 node --test 7 用例锁定；`setTitle()` 单一写口 + `isRO`/`remoteTitle` 模块级化；WELCOME ro 分支旧一次性前缀拼接删除，auth_failed 重试零双前缀（兑现 P2 D-14 挂账）
- 供应链：三包精确钉版无 ^ 前缀；js-base64 经 pnpm-workspace.yaml overrides 钉 3.9.2 避开发布仅 1 天的 3.9.3（RESEARCH §Package Legitimacy Audit [SUS too-new] 规避）
- dist 重建入库（487.56 kB / gzip 129.42 kB），产物含 xterm-hover 与 activeVersion 检索串；`go build ./...` 绿

## Task Commits

Each task was committed atomically:

1. **Task 1: 依赖钉版安装 + addon 加载段 + OSC8 linkHandler + link tooltip** - `a319698` (feat)
2. **Task 2: 标题同步单一写口——lib/title.ts 纯函数 + node --test + setTitle/isRO 接线** - `e0f46be` (feat)
3. **Task 3: dist 重建与产物提交** - `5209b2c` (chore)

**Plan metadata:** 见尾部 docs 提交

## Files Created/Modified
- `web/package.json` - 三 addon 精确钉版入 dependencies（0.9.0/0.12.0/0.2.0）
- `web/pnpm-workspace.yaml` - 新建：overrides 钉 js-base64 3.9.2（pnpm 11 settings 新家）
- `web/pnpm-lock.yaml` - lockfile 更新（js-base64 解析 3.9.2）
- `web/src/main.ts` - 双 addon 导入与加载段、linkHandler 三键、showLinkTooltip/hideLinkTooltip、isRO/remoteTitle 模块级、setTitle()、onTitleChange 接线、WELCOME ro 分支改造
- `web/index.html` - .xterm-hover 样式规则（z-index 901/max-width 480px/break-all/pointer-events none）
- `web/src/lib/title.ts` - 新建：sanitizeTitle 纯函数
- `web/src/lib/title.test.ts` - 新建：node --test 7 用例回归锁
- `web/tsconfig.json` - exclude src/**/*.test.ts
- `web/dist/index.html` - 重建产物入库

## Decisions Made
- **js-base64 override 落 pnpm-workspace.yaml**（详见 Deviations 1）——pnpm 11 不再读 package.json pnpm 字段，plan 字面机制在钉版工具链下不生效；迁移到官方指定新家后 lockfile 解析 3.9.2，规避意图逐字保持。
- **ClipboardAddon 只入包不导入**：noUnusedLocals 下未用导入直接报错，条件加载属 04-05 职责（plan 既定），本 plan 仅完成供应链钉版。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] pnpm.overrides 机制迁移：package.json pnpm 字段 → pnpm-workspace.yaml**
- **Found during:** Task 1（依赖安装后 lockfile 校验）
- **Issue:** plan 指定在 `web/package.json` 加 `"pnpm": { "overrides": { "js-base64": "3.9.2" } }`；但环境钉版 pnpm 11.21.0（CI 同钉，01-04 决策）安装时明示 WARN：`The "pnpm" field in package.json is no longer read by pnpm`，overrides 被忽略，lockfile 仍解析 js-base64 3.9.3（发布仅 1 天的 [SUS too-new] 包）——Task 1 验收「lockfile 解析 js-base64 为 3.9.2」必红。
- **Fix:** 删除 package.json 死配置，新建 `web/pnpm-workspace.yaml` 写 `overrides: { js-base64: 3.9.2 }`（pnpm 11 settings 官方新家，非 workspace 项目同样适用）；重装后 lockfile 三处解析均为 3.9.2。
- **Files modified:** web/package.json, web/pnpm-workspace.yaml（新建）, web/pnpm-lock.yaml
- **Verification:** `grep js-base64 web/pnpm-lock.yaml` 全为 3.9.2；install 无 WARN；tsc --noEmit 绿
- **Committed in:** a319698（Task 1 提交）

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** 唯一偏差为工具链版本导致的机制迁移，js-base64 钉 3.9.2 的安全意图逐字保持；无范围蔓延。

## Issues Encountered
None — 三任务均一次通过，无调试往返。

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- 04-03（选中复制/粘贴门）与 04-05（osc52 门/prefs 消费）可直接复用模块级 `isRO`；ClipboardAddon 已在包内随时可条件加载
- 渲染层人工确认项（CJK/IME、链接 hover/单击、标题同步与 [ro] 前缀）已按 plan 约定汇入 04-06 04-UAT.md 清单（coverage D4）
- 无阻塞项

## Self-Check: PASSED

- 文件存在性：web/src/lib/title.ts、web/src/lib/title.test.ts、web/pnpm-workspace.yaml、本 SUMMARY 全部 FOUND
- 提交存在性：`a319698`（Task 1 feat）、`e0f46be`（Task 2 feat）、`5209b2c`（Task 3 chore）均在 git log
- Plan 级 verify 复跑全绿：node --test 7/7 PASS、tsc --noEmit 退出 0、dist 含 xterm-hover/activeVersion、go build ./... 退出 0
- 无新未跟踪文件（.gz 按既定 .gitignore 策略不入库）；三次任务提交零意外删除

---
*Phase: 04-frontend*
*Completed: 2026-08-18*
