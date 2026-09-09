---
phase: 04-frontend
plan: 05
subsystem: ui
tags: [prefs, query-override, theme, osc52, clipboard, whitelist, xterm]

# Dependency graph
requires:
  - phase: 04-frontend (plan 01)
    provides: Welcome prefs 通道（WelcomePayload.Prefs omitempty + ValidClientOptionKey 恰 10 键白名单 + --client-option/--osc52 flag）
  - phase: 04-frontend (plan 03)
    provides: 三开关量埋点（welcomeDone/resizeOverlayOn/confirmBeforeUnloadOn）与 clipboardOK 存在性门控、beforeunload 条件注册挂点
provides:
  - web/src/lib/prefs.ts 纯函数（XTERM_PREF_KEYS/BEHAVIOR_PREF_KEYS 白名单 + parseQueryPrefs/splitPrefs/mergeTheme）与 node --test 8 用例回归锁
  - defaultTheme tango 调色板常量（构造初值/query 特判/WELCOME 合并三处同源单一事实源）
  - query 通道：构造前解析 + queryKeys 优先级跳过机制（URL query > --client-option > 内置默认，D-16）
  - WELCOME prefs 应用段（xterm 键运行时应用 + theme 合并 + fit.fit() 重算 + behavior 开关 + OSC52 write-only 条件加载）
  - dist 产物重建入库（含 prefs 接线检索串）
affects: [04-06 UAT 渲染层人工确认清单（prefs 视觉效果/query 覆盖/theme 合并/OSC52 写入）]

# Actuals (#2632) — chars/4 over realized diff，与 plan estimate（30000, confidence: low）配对校准
# 全量 diff 965033 chars 中约 947K 为 dist/index.html 单文件产物行级 churn（minify 行重排）；
# 源码级 diff（web/src/）为 17596 chars ≈ 4399 tokens
actuals:
  tokens: 241258
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []  # 零新依赖（addon-clipboard 已于 04-02 装包，本 plan 随用随加 import；T-04-SC accept 既定）
  patterns:
    - "prefs 纯函数抽取至 lib/prefs.ts（零 DOM 依赖）——node --test 直跑 .ts 回归锁（RESEARCH §A3 形态同 title.test.ts）"
    - "调色板单一事实源：defaultTheme as const 模块级常量，构造初值/query theme 特判/WELCOME theme 合并三处同源"
    - "优先级经 queryKeys 跳过实现：query 成功键记集合，WELCOME prefs 逐项跳过——URL query > --client-option > 内置默认"
    - "OSC52 write-only provider：readText 恒 resolve ''（非 reject——核心异步 OSC 链 rethrow 成 unhandled rejection），clipboardOK 双门条件加载"

key-files:
  created:
    - web/src/lib/prefs.ts
    - web/src/lib/prefs.test.ts
  modified:
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "queryKeys 以 export 标记——Task 1 声明时无消费点（Task 2 WELCOME 分支才引用），noUnusedLocals 下裸 const 使 tsc 必红"
  - "query xterm 键 spread 经 as Partial<ITerminalOptions> 收窄——Record<string, unknown> 直接展开会使构造字面量属性类型变 unknown，tsc 必红；白名单已保证键合法性"
  - "OSC52 provider 对象以 IClipboardProvider 注解对齐 d.ts 签名——_sel 参数类型经上下文推断即 ClipboardSelectionType，避开 const enum 在 isolatedModules 下的导入复杂性（plan 尾注『以 d.ts 为准』落地形态）"

patterns-established:
  - "query 与 WELCOME prefs 两通道 theme 合并同源：均经 mergeTheme/{...defaultTheme, ...} 形态，部分 theme 未指定键保留 tango 调色板（RESEARCH §Pitfall 3 修正覆盖两通道）"
  - "behavior 键双通道同一校验：typeof v === 'boolean' 才写开关量（服务端对值只验 JSON 不验类型，前端防御性应用），非布尔 console.warn 忽略"

requirements-completed: [FE-06, FE-07]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "query 通道：构造前 parseQueryPrefs（白名单 10 键逐键 JSON.parse，osc52 结构性排除）+ queryKeys 模块级集合 + query xterm 键作构造初值 + query theme 经 mergeTheme 特判合并（非对象中和默认 + warn）+ query behavior 键 boolean 校验 startup 写开关量"
    requirement: FE-07
    verification:
      - kind: unit
        ref: "web/src/lib/prefs.test.ts#8 用例（合法键解析/非法 JSON invalid/未知键忽略/osc52 排除/theme 对象解析/splitPrefs 分派/mergeTheme 合并与非对象 null）"
        status: pass
      - kind: other
        ref: "pnpm -C web exec tsc --noEmit（退出 0）+ grep mergeTheme(defaultTheme web/src/main.ts 命中"
        status: pass
    human_judgment: false
  - id: D2
    description: "WELCOME prefs 应用段：xterm 键（queryKeys 跳过 + theme {...defaultTheme, ...} 合并 + 非对象 warn）→ fit.fit() 重算 → behavior 键（boolean 校验写开关量，不写 term.options）→ osc52===true && clipboardOK 时 ClipboardAddon write-only 加载（readText 恒 resolve ''）；应用位置在 welcomeDone/beforeunload 注册点之前"
    requirement: FE-07
    verification:
      - kind: e2e
        ref: "node web/uat/phase04.mjs /tmp/wesh-uat/wesh（十场景 10/10：Welcome prefs 形状/注入/theme 对象/osc52 下发/last-wins/启动拒绝矩阵）"
        status: pass
      - kind: other
        ref: "tsc --noEmit（退出 0）+ grep defaultTheme/ClipboardAddon/Promise.resolve('') 命中 + head -15 main.ts 含 prefs（帧常量注释块同步）"
        status: pass
    human_judgment: false
  - id: D3
    description: "FE-06 开关接线：resizeOverlay/confirmBeforeUnload 经 query/prefs 两通道 boolean 校验后可关（04-03 埋点开关量纯翻转，条件注册代码零改动）"
    requirement: FE-06
    verification:
      - kind: other
        ref: "tsc --noEmit（退出 0）+ WELCOME 应用段顺序人工核查（xterm→fit→behavior→osc52→welcomeDone 注册点）+ behavior 键零 term.options 写入路径核查"
        status: pass
    human_judgment: false
  - id: D4
    description: "dist 产物重建入库：全量构建 496.01 kB / gzip 132.56 kB，含 osc52/resizeOverlay/confirmBeforeUnload 检索串，产物时间戳新于 main.ts，.gz 不入库"
    verification:
      - kind: other
        ref: "time pnpm -C web build（退出 0）+ grep 三检索串命中 + go build -o /tmp/wesh-uat/wesh ./cmd/wesh（退出 0）"
        status: pass
    human_judgment: false
  - id: D5
    description: "浏览器渲染层确认：prefs 下发视觉效果/query 覆盖/theme 合并后色相/OSC52 写入系统剪贴板"
    requirement: FE-07
    verification: []
    human_judgment: true
    rationale: "渲染层视觉效果与真实剪贴板权限模型不可自动化（plan verify human-check 既定）；按 plan 约定并入 04-06 的 04-UAT.md 清单统一人工确认"

# Metrics
duration: 14min
completed: 2026-08-18
status: complete
---

# Phase 4 Plan 05: FE-07 前端 prefs 消费 + FE-06 开关接线 Summary

**FE-07 前端半侧闭环：lib/prefs.ts 白名单纯函数（node --test 8 用例锁定，osc52 结构性排除）→ query 构造前解析与 queryKeys 优先级 → WELCOME prefs 逐字顺序应用（theme 合并保留 tango 调色板 + fit 重算）→ FE-06 两开关双通道接线 → OSC52 write-only 条件加载，dist 重建入库且 UAT 十场景全绿**

## Performance

- **Duration:** 14 min
- **Started:** 2026-08-18T16:59:14Z
- **Completed:** 2026-08-18T17:13:29Z
- **Tasks:** 3
- **Files modified:** 4（2 新建 + 2 修改）

## Accomplishments
- query 通道打通：构造前 `parseQueryPrefs(location.search)`——白名单 10 键逐键 JSON.parse，成功键记 `queryKeys` 并作构造初值；非法 JSON/白名单外键静默忽略 + console.warn（D-16）；osc52 结构性不在白名单（D-12 专项测试锁定）
- theme 常量化与两通道同源合并：`defaultTheme` tango 调色板 as const 单一事实源；query 构造段经 `mergeTheme` 特判、WELCOME 分支经 `{...defaultTheme, ...}` 展开——部分 theme 未指定键均保留调色板（RESEARCH §Pitfall 3 修正定稿两通道覆盖），非对象 theme 值 query 通道中和为默认、WELCOME 通道 warn 忽略
- WELCOME prefs 应用段按 UI-SPEC §Prefs Contract 逐字顺序：xterm 键（queryKeys 跳过，D-16 优先级落地）→ `fit.fit()` 重算（D-13/§Pitfall 7）→ behavior 键 boolean 校验写开关量（禁写 term.options）→ osc52 条件加载，全部位于 welcomeDone/beforeunload 注册点之前（04-03 条件注册零改动）
- OSC52 write-only provider：`readText` 恒 `Promise.resolve('')`（RESEARCH §Pitfall 4 定稿——reject 在核心异步 OSC 链 rethrow 成 unhandled rejection），`writeText` 委托 navigator.clipboard；`prefs.osc52 === true && clipboardOK` 双门（D-12 服务端专有开启 + OSC52 惰性）
- dist 重建入库（496.01 kB / gzip 132.56 kB，三检索串命中）；`phase04.mjs` 十场景重跑 10/10 全绿（FE-07 通道端到端回归）

## Task Commits

Each task was committed atomically:

1. **Task 1: lib/prefs.ts 纯函数 + node --test + query 构造前解析与 theme 常量化** - `8bd9546` (feat)
2. **Task 2: WELCOME prefs 应用 + theme 合并 + behavior 开关 + OSC52 write-only 加载** - `cc053d2` (feat)
3. **Task 3: dist 重建与产物提交 + 端到端冒烟** - `ec002ea` (chore)

**Plan metadata:** 见尾部 docs 提交

## Files Created/Modified
- `web/src/lib/prefs.ts` - XTERM_PREF_KEYS/BEHAVIOR_PREF_KEYS 白名单常量（与 Go 侧 ValidClientOptionKey 语义同源注释）+ parseQueryPrefs/splitPrefs/mergeTheme 纯函数
- `web/src/lib/prefs.test.ts` - node --test 8 用例（osc52 排除专项、splitPrefs 分派、mergeTheme 部分合并保留 base 与非对象 null）
- `web/src/main.ts` - defaultTheme 常量、export queryKeys、query 解析与 theme 构造特判、query behavior startup 应用、WELCOME prefs 应用段、ClipboardAddon 条件加载、帧常量注释块 prefs 说明同步
- `web/dist/index.html` - 重建产物入库（.gz 按 .gitignore 既定策略不入库）

## Decisions Made
- **queryKeys 以 export 标记**（Task 1）：plan 字面 `const queryKeys = query.keys;` 在 Task 1 无消费点（Task 2 WELCOME 分支才引用），`noUnusedLocals` 下裸 const 使 Task 1 的 tsc 验证必红——export 为最小调和，Task 2 直接引用语义不变。
- **query xterm spread 经 `as Partial<ITerminalOptions>` 收窄**（Task 1）：`Record<string, unknown>` 直接展开进构造字面量会使各属性类型被 unknown 覆盖，tsc 必红；白名单已保证键合法性，一次收窄 cast 与 Task 2 WELCOME 分支的 cast 注释口径一致。
- **OSC52 provider 以 IClipboardProvider 注解对齐 d.ts**（Task 2）：plan 尾注要求 `_sel` 参数类型"以 d.ts 为准"——对象字面量经 `IClipboardProvider` 注解后 `_sel` 由上下文推断为 `ClipboardSelectionType`，避开 const enum 在 isolatedModules 下的导入复杂性；plan verify 检索串 `Promise.resolve('')` 单行形态保持。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] queryKeys 改 export 防 noUnusedLocals 误报**
- **Found during:** Task 1（tsc 验证前类型推导预检）
- **Issue:** plan 字面 `const queryKeys = query.keys;`——Task 1 范围内 queryKeys 无消费点（WELCOME 分支引用属 Task 2），tsconfig `noUnusedLocals: true` 下 `pnpm -C web exec tsc --noEmit` 必报 TS6133，Task 1 verify 不可达
- **Fix:** 声明为 `export const queryKeys = query.keys;` 并注释说明（exported 变量不受 noUnusedLocals 约束；入口模块 export 对 vite 构建无害）
- **Files modified:** web/src/main.ts
- **Verification:** tsc --noEmit 退出 0；Task 2 WELCOME 分支正常引用
- **Committed in:** 8bd9546（Task 1 提交）

**2. [Rule 3 - Blocking] query xterm 键 spread 加 Partial<ITerminalOptions> 收窄 cast**
- **Found during:** Task 1（构造选项接线）
- **Issue:** plan 字面「构造选项字面量尾部展开 `...queryParts.xterm`」——`splitPrefs` 返回 `Record<string, unknown>`，unknown 值 spread 使构造字面量属性类型被 unknown 覆盖，不可赋给 ITerminalOptions，tsc 必红
- **Fix:** `...queryParts.xterm as Partial<ITerminalOptions>`（白名单已保证键合法性，注释引 lib 层与 Go 侧同源）；随之引入 `type ITerminalOptions` 内联 type import（verbatimModuleSyntax 纪律）
- **Files modified:** web/src/main.ts
- **Verification:** tsc --noEmit 退出 0；node --test 8/8 绿
- **Committed in:** 8bd9546（Task 1 提交）

---

**Total deviations:** 2 auto-fixed (2 blocking——均为 plan 字面形态与 tsc 严格配置的调和，零语义偏移)
**Impact on plan:** 两修正均为达成 plan 自身验证命令的必要最小改动；D-12/D-13/D-16/D-19 语义逐字保持。

## Issues Encountered
None — 三任务均一轮通过，无调试往返；node --test 8/8、tsc、构建、UAT 10/10 首次执行即全绿。

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- FE-07 全链闭合：CLI → Welcome prefs → 前端按优先级应用（query > --client-option > 内置默认）+ fit 重算；FE-06 开关双通道可关；D-12 OSC52 服务端专有开启 + write-only 闭合
- 浏览器渲染层人工确认项（prefs 视觉效果/query 覆盖/theme 合并色相/OSC52 写入）按 plan 约定汇入 04-06 的 04-UAT.md 清单（coverage D5）
- 全量 `go test -race ./...` 按 phase 约定在 04-06 收口统一执行
- 无阻塞项

## Self-Check: PASSED

- 文件存在性：web/src/lib/prefs.ts、web/src/lib/prefs.test.ts、web/src/main.ts、web/dist/index.html、本 SUMMARY 全部 FOUND
- 提交存在性：`8bd9546`（Task 1 feat）、`cc053d2`（Task 2 feat）、`ec002ea`（Task 3 chore）均在 git log
- Plan 级 verify 复跑全绿：`node --test web/src/lib/prefs.test.ts` 退出 0（8/8）；`pnpm -C web exec tsc --noEmit` 退出 0；dist 含 osc52/resizeOverlay/confirmBeforeUnload 检索串；产物时间戳（01:11:51）新于 main.ts（01:09:50）；`node web/uat/phase04.mjs /tmp/wesh-uat/wesh` 十场景 10/10
- 三次任务提交零意外删除（post-commit deletion check 均空）；无新未跟踪文件（函数/变量名 minify 擦除为构建常态，检索串按 plan 定义的字符串字面量口径验证）

---
*Phase: 04-frontend*
*Completed: 2026-08-18*
