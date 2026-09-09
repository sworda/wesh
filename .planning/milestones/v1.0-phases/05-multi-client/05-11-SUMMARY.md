---
phase: 05-multi-client
plan: 11
subsystem: ui
tags: [websocket, xterm, resize, viewport-constraint, multi-client, typescript]

# Dependency graph
requires:
  - phase: 05-multi-client
    provides: 05-10 Welcome 三通道恒携会话 cols/rows（attach/升格/运行期推送同形）+ 05-08 前端升格纠正链 + 05-04 resize 仲裁
provides:
  - refit() 统一 resize 入口：上报尺寸 = fit.proposeDimensions() 窗口物理尺寸（恒报全值驱动仲裁）；渲染尺寸 = 逐轴 min(fit, sessionDims)
  - WELCOME 分支会话尺寸消费：成对校验 [1,1000] 正整数 → sessionDims 赋值 → mode 分支 → 统一 refit（ro attach 即约束渲染，升格自然解除）
  - sendResize lastReported 等值去重（ro 期 isRO 门拦截不记账，升格后首次 refit 必真实上报）
  - ro 一次性 console 提示 roNotified 门闩（运行期尺寸推送下每连接恰一次）
affects: [05-12 行为级端到端断言（D6/S10）, phase-06 重连（connect 重置块四状态清零已就位）]

# Actuals (#2632) — chars/4 over the realized diff（estimate 同尺）
# 口径注：仅计 web/src/main.ts 源 diff（17169 chars /4 ≈ 4300）；web/dist/index.html 为生成产物
# 单行整体重写（~1MB 字符 churn），计入会把校准信号变成 esbuild 输出噪声测量，故排除。
actuals:
  tokens: 4300
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "上报/渲染双概念拆分：resize 链路唯一入口 refit()——上报恒窗口物理尺寸（仲裁输入），渲染逐轴 min(fit, 会话尺寸)（视口约束）；禁止 CSS 约束容器形态（proposeDimensions 被污染则两概念无法拆分）"
    - "xterm 6 下去 onResize 化：term.resize 唯一调用方收编进 refit，上报/浮层职责内聚，无旁路触发面"
    - "接线前 export 防 noUnusedLocals（queryKeys 先例第二次沿用）：模块级门闩先声明后接线时 export 过渡，接线完成去 export"

key-files:
  created: []
  modified:
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "[Phase 05-11]: 上报/渲染双概念拆分——refit() 唯一入口收编窗口监听/onopen/升格/prefs 四调用点；上报恒 fit 物理尺寸驱动 PTY 仲裁（升格纠正链不断裂），渲染 term.resize 逐轴 min(fit, sessionDims)；不采用 CSS 约束容器（proposeDimensions 会被污染）"
  - "[Phase 05-11]: term.onResize 订阅拆除 + sendResize lastReported 去重——ro 期 isRO 门拦截不记账使升格后首次 refit 必真实上报（05-08 纠正链保持）；onopen Hello 发出后同步 lastReported=Hello 载荷尺寸（防握手 Welcome 后冗余等值 RESIZE，线序零漂移）"
  - "[Phase 05-11]: ro 一次性 console 提示改 roNotified 门闩承载（运行期尺寸推送打破「ro Welcome 每 attach 仅一次」天然无重复不变量的等价物），文案逐字不动（D1b 三要素断言锁定；min 逐轴下小窗口轴仍裁剪，文案语义依然为真）"

patterns-established:
  - "Welcome 幂等矩阵（注释登记）：尺寸推送重放同值 → term.resize 变化守卫 + sendResize 去重 + overlay fitChanged 门三重拦截零副作用；prefs 重放 queryKeys 跳过 + osc52Loaded 门闩；welcomeDone/beforeunload DOM 同 listener 去重"

requirements-completed: [MULTI-01, MULTI-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "refit() 统一入口与双概念拆分（四 fit 调用点收编 / onResize 订阅移除无残余 / sendResize 去重在既有四门之后 / onopen lastReported 同步）"
    requirement: MULTI-04
    verification:
      - kind: other
        ref: "cd web && time pnpm build（tsc 严格模式含 noUnusedLocals + vite + gzip 链干净通过，2.3s）"
        status: pass
    human_judgment: false
  - id: D2
    description: "WELCOME 分支尺寸键成对校验 [1,1000] → sessionDims 赋值 → mode 分支 → 统一 refit 顺序锁定；旧服务端（无尺寸键）sessionDims 恒 null 渲染=fit 零漂移"
    requirement: MULTI-01
    verification:
      - kind: e2e
        ref: "node web/uat/phase05-dom.mjs 16/16（jsdom 加载真实 dist 连真实 spawn 实例，Welcome 恒携 05-10 cols/rows 下行链路全程经过新代码路径）"
        status: pass
    human_judgment: false
  - id: D3
    description: "ro console 一次性提示门闩（三要素 /input is not sent/、/clipped/、/reattach/ 逐字锁定且每连接恰一次）"
    requirement: MULTI-01
    verification:
      - kind: e2e
        ref: "web/uat/phase05-dom.mjs#D1b（infos=1 条）+ #D2a（升格前旁观 infos=1 不随 Welcome 重放增长）"
        status: pass
    human_judgment: false
  - id: D4
    description: "既有行为零回归：ro 端窗口 resize 零 RESIZE 上行（D-09 前端闸在 refit 链路下保持）/ 递补升格 INPUT 恢复 / 503/错链/1013 专版"
    requirement: MULTI-04
    verification:
      - kind: e2e
        ref: "web/uat/phase05-dom.mjs#D1d（上行帧=[]）/ #D2c（INPUT 已上行）/ D3a-D5c 全过"
        status: pass
    human_judgment: false
  - id: D5
    description: "dist 重建产物（embed 伺服链）：index.html 时间戳本次构建 + Math.min 结构指纹 ≥1"
    requirement: MULTI-04
    verification:
      - kind: other
        ref: "ls -l dist/index.html（Aug 22 12:04 本次产物）+ grep -c 'Math\\.min' dist/index.html = 10"
        status: pass
    human_judgment: false
  - id: D6
    description: "约束渲染行为面：宽端 ro attach 后 xterm 按会话矩形渲染留白、同 cols 双端逐屏一致、升格后 min(fit,session) 解除约束回窗口渲染"
    requirement: MULTI-01
    verification: []
    human_judgment: true
    rationale: "行为级端到端断言由 05-12 D6/S10 承载（05-11 plan 既定分工：本 plan 自证到构建 + 回归层）；@xterm/headless 约束渲染 rows/cols 断言与协议面 S10 在 05-12 落地后本条可改判"

# Metrics
duration: 22min
completed: 2026-08-22
status: complete
---

# Phase 05 Plan 11: G-05-1 会话尺寸视口约束（前端半侧）Summary

**resize 链路 refit() 统一入口落地「上报=窗口 fit 物理尺寸 / 渲染=逐轴 min(fit, 会话尺寸)」双概念拆分——宽端 ro 旁观者约束到会话矩形渲染留白（readline 相对寻址流异尺寸双端逐屏一致的修复落点），rw 上行恒驱动 PTY 仲裁且升格后尺寸接管纠正链一次往返收口，旧服务端（Welcome 无尺寸键）行为与改造前逐字节一致。**

## Performance

- **Duration:** 22 min
- **Started:** 2026-08-22T03:47:51Z
- **Completed:** 2026-08-22T04:10:11Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- refit() 统一入口收编窗口监听/onopen/升格分支/prefs 段四个 fit.fit() 调用点：渲染 term.resize(逐轴 min(fit, sessionDims))（变化才调，推送重放零抖动）与上报 sendResize(fit 全值) 拆分——「CSS 约束容器再 fit」形态被明确否决（proposeDimensions 会返回被约束尺寸，上报/渲染两概念无法拆分，升格纠正链必断裂）
- sendResize 等值去重（lastReported）落在既有四道门之后：ro 期被 isRO 门拦截不记账 → 升格后首次 refit 必真实上行当前窗口尺寸（05-08 尺寸接管纠正链保持）；onopen Hello 发出后同步 lastReported=Hello 载荷尺寸，握手 Welcome 到达后的 refit 不再重发等值冗余 RESIZE（线序零漂移）
- term.onResize 订阅整段拆除（xterm 6 下 term.resize 唯一调用方为 refit，无旁路触发面）；overlay D-17 语义钉死为窗口物理尺寸——fitChanged 门使尺寸推送重放/约束变化不闪浮层，ro 端窗口 resize 浮层同显（R4 保持）
- WELCOME 分支尺寸键成对校验（任一键出现即新形态，两键均须 [1,1000] 正整数，非法 console.warn 保持旧值——D-16/T-05G-04）；统一 refit 落于 mode 分支后/prefs 段前，顺序硬约束 sessionDims 先赋值 → refit 最后，升格时 min(fit, session) 自然解除约束回窗口渲染
- ro console 一次性提示包 roNotified 门闩（运行期尺寸推送打破「每 attach 仅一次」不变量的等价物），文案逐字不动；connect() 重置块补四状态清零（IN-01 延伸）；Welcome 幂等矩阵注释登记
- dist 重建并提交（embed 伺服链）；补验证据：重打 wesh 二进制后 phase05-dom.mjs **16/16 通过**（D1b 门闩恰一次 / D1d ro 窗口 resize 零上行 / D2c 升格 INPUT 恢复——新代码路径全链行为回归）

## Task Commits

Each task was committed atomically:

1. **Task 1: resize 链路重构——refit() 统一入口（上报=fit / 渲染=min 逐轴拆分）+ sendResize 去重 + overlay 语义钉死** - `ced81ed` (feat)
2. **Task 2: WELCOME 分支尺寸应用与升格交互 + ro 提示门闩 + dist 重建** - `31d8a68` (feat)

**Plan metadata:** 见最终 docs 提交（docs(05-11): complete ... plan）

## Files Created/Modified

- `web/src/main.ts` - 四模块级状态（sessionDims/lastReported/prevFit/roNotified）+ refit() 统一入口 + sendResize 去重 + onResize 订阅拆除 + WELCOME 尺寸键校验/赋值/统一 refit + ro 提示门闩 + connect() 重置块四状态清零 + 帧常量注释与幂等矩阵登记（+116/-35 净额）
- `web/dist/index.html` - 重建产物（tsc 严格 + vite + gzip -k9 既定链；Math.min 指纹 ×10；.gz 按 .gitignore 既定不入库）

## Decisions Made

- **双概念拆分的形态裁决**：渲染约束用 term.resize(ITerminal 稳定 API) + fit.proposeDimensions() 取窗口物理尺寸，刻意不采用「CSS 约束容器再 fit」——后者使 proposeDimensions 返回被约束尺寸，rw 端无法上报窗口物理尺寸驱动仲裁（G-05-1 设计约束 3）；#terminal 零 CSS/元素新增（留白 = body 背景纯布局，D-07）
- **onopen lastReported 同步**：Hello 载荷即首次尺寸上报，发送后同批置位 lastReported——消除握手 Welcome 后 refit 把等值尺寸当「变化」重发一帧冗余 RESIZE 的线序漂移（改造前握手后无此帧，纪律保持）
- **ro 提示门闩等价物**：运行期尺寸推送使 ro Welcome 每连接可多次到达，原「每 attach 仅一次天然无重复」不变量失效，一次性语义改由 roNotified 门闩承载；文案逐字不动（D1b 断言锁定，min 逐轴下「小窗口轴仍裁剪」语义依然为真）
- **roNotified 接线过渡形态**：Task 1 先声明未接线时以 export 防 noUnusedLocals 误报（queryKeys 04-05 先例第二次沿用），Task 2 接线完成即去 export——每个提交点 tsc 严格模式独立干净

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] roNotified Task 1 声明即触发 noUnusedLocals，按 queryKeys 先例 export 过渡**
- **Found during:** Task 1（四状态声明后构建前走查）
- **Issue:** plan Task 1 item 1 要求四状态同批声明且标注 roNotified「Task 2 接线」，同时 Task 1 done 要求 tsc 严格模式干净通过——tsconfig noUnusedLocals 下未接线的模块级 let 必报 TS6133，两要求机械冲突
- **Fix:** Task 1 以 `export let roNotified` 过渡（04-05 queryKeys「export 防 noUnusedLocals 在接线前误报」既定先例第二次沿用），Task 2 接线完成即去 export——两提交点各自 tsc 干净
- **Files modified:** web/src/main.ts
- **Verification:** Task 1/Task 2 各自 `pnpm build`（tsc+vite）干净通过
- **Committed in:** `ced81ed`（Task 1 commit），去 export 随 `31d8a68`（Task 2 commit）

---

**Total deviations:** 1 auto-fixed (1 Rule 3 - Blocking)
**Impact on plan:** 纯 plan 内部机械矛盾的既定先例解法，零范围蔓延；全部 must_have truths 逐字落地（五条款：约束渲染 / 上报恒 fit / 升格解除 / 幂等矩阵 / 旧服务端零漂移）

## Issues Encountered

- plan Task 1 item 1 括注「connect() 重置块同批重置」与 Task 2 step 5「重置块补四状态」为同一终态的重复指派——按 Task 2 step 5 显式指令落点（Task 2 提交），Task 1 中间态四状态无行为面（sessionDims 恒 null 未赋值、roNotified 未接线），语义安全
- plan Task 1 done 注「本 wave 服务端尚未下发尺寸键」为并行 wave 假设；实际顺序执行 05-10 已先在 main 落地（75e4def/9cc76f4），dist 重建即消费真实 cols/rows——行为级 D6 断言仍按 plan 既定分工留 05-12，本 plan 以 phase05-dom.mjs 16/16 作回归层证据（超出 plan verify 字面但不越分工）

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **05-12（行为级端到端断言）消费面就绪**：前端约束渲染/升格解除/幂等全链已落地并随 dist 入库，D6（@xterm/headless 约束 rows/cols 断言 + 升格解除）与 S10（协议面）可直接对真实产物断言；SUMMARY coverage D6 待 05-12 证据回填改判
- **Phase 6 重连前提再夯实**：connect() per-connection 重置块已含 resize 四状态清零（IN-01 登记延伸）
- 无阻塞项；威胁模型 T-05G-04/05/06/SC 四条目形态与 plan 登记一致（成对校验+区间闸 / 三重幂等拦截反馈环结构性不存在 / 纯布局整数不进日志 / 零新依赖），无新增安全面

---
*Phase: 05-multi-client*
*Completed: 2026-08-22*

## Self-Check: PASSED

- 全部 2 个修改文件落盘确认：web/src/main.ts（四状态 L238-247 / refit L278 / WELCOME 校验段 L502-511 / 统一 refit L539-548 核读）、web/dist/index.html（Aug 22 12:04 本次构建产物，Math.min ×10）
- Task 提交 ced81ed / 31d8a68 均在 git log 确认（FOUND）
- 验证证据：Task 1/Task 2 `time pnpm -C web build` 各自干净（tsc 严格 + vite + gzip）；phase05-dom.mjs 16/16（重打二进制嵌本次 dist）；两提交 post-commit deletion check 均为零删除
