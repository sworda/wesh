---
phase: 03-auth
plan: 05
subsystem: auth
tags: [typescript, websocket, basic-auth, one-time-ticket, xterm, vite-singlefile]

# Dependency graph
requires:
  - phase: 03-auth (plan 03-01)
    provides: proto.go HelloPayload.Ticket 可选字段 + ErrAuthFailed="auth_failed" 契约
  - phase: 03-auth (plan 03-03)
    provides: POST /api/attach ticket 签发端点（200 ticket / 404 无认证探测 / 401·429 分派）
provides:
  - web/src/main.ts connect() 认证感知连接流程（fetch ticket → WS → Hello{ticket} → auth_failed 静默重试一次）
  - ws/wss scheme 按 location.protocol 分支（https 页面下 wss，03-04 TLS 伺服浏览器可用）
  - 重建入库的 web/dist/index.html（内嵌新连接流程，裸 clone 构建即认证感知前端）
affects: [03-06 human-check 汇总（浏览器弹窗凭据缓存/fetch 携带/过期重试人工确认）, Phase 5 多客户端（retriedAuth/连接状态机沿用）]

# Actuals (#2632) — same estimateTokens scale (chars/4 over the realized diff)
actuals:
  tokens: 26474
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []（零新增依赖——fetch/WebSocket 均浏览器内建，T-03-SC accept）
  patterns:
    - "认证感知 connect() 函数包裹连接建立：fetch 按 200/404/429/401·其余/catch 五路分派，per-connection 状态每次尝试开头重置"
    - "auth_failed 单次门闩重试（retriedAuth 置位后 void connect()，重试再失败落既有 1008 分支展示）"
    - "连接句柄模块级 WebSocket | null + null 闸（常驻接线 onData/sendResize 在 fetch 窗口期静默吞输入）"

key-files:
  created: []
  modified:
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "ws 声明为模块级 WebSocket | null（非 plan 字面 let ws: WebSocket）+ onData/sendResize null 闸——消除 fetch 异步窗口期用户敲击 ws.readyState 的 TypeError 回归（Rule 1）"
  - "dist .gz 不入库按 .gitignore 既定策略执行（plan 字面与仓库策略冲突，仓库优先；README:40 陈旧声明登记 deferred-items.md）"
  - "401 与其余非 ok 响应同口径通用认证失败面板（无 oracle 纪律延伸到前端文案）；面板引导重新加载页面触发浏览器原生 Basic 弹窗（Pitfall 6），零新 UI（D-02）"

patterns-established:
  - "fetch 401 不弹原生框 → 面板引导 reload 重新导航触发弹窗，而非自建登录表单"
  - "connect() 内 const sock = ws 闭包确定句柄写法（TS 严格模式对模块级可空 let 不做闭包收窄）"
  - "帧常量注释区与 proto.go 双向指路：Hello 载荷 ticket 可选字段 + auth_failed Error code（D-16 手工对齐纪律）"

requirements-completed: [SEC-02]

coverage:
  - id: D1
    description: "connect() 认证感知连接流程：fetch POST /api/attach 五路分派（200 取 ticket / 404 无认证直连 / 429 稍候面板 / 401·其余认证失败面板 / catch unreachable 面板），Hello 携带 ticket（undefined 自动省略），auth_failed 静默重试一次"
    requirement: SEC-02
    verification:
      - kind: other
        ref: "pnpm -C web exec tsc --noEmit && pnpm -C web build（TS 严格 + vite 构建退出 0）"
        status: pass
      - kind: other
        ref: "grep 验收：main.ts 含 connect()/retriedAuth/auth_failed//api/attach/wss 分支；prohibition grep 无 localStorage/URL/console 泄漏 ticket"
        status: pass
    human_judgment: true
    rationale: "浏览器行为半侧（原生 Basic 弹窗凭据缓存随同源 fetch 自动携带、ticket 60s 过期静默重试、401 reload 重新弹窗）需真实浏览器人工确认——plan <verification> 已明确汇总至 03-06 human-check 清单执行"
  - id: D2
    description: "dist 产物重建入库：index.html 含 /api/attach 与 wss:// 检索串（新流程入包证据），go build ./... embed 链编译绿"
    verification:
      - kind: other
        ref: "pnpm -C web build && test -f web/dist/index.html.gz && grep -q '/api/attach' web/dist/index.html && grep -q 'wss://' web/dist/index.html && go build ./..."
        status: pass
      - kind: other
        ref: "go test ./... 全量（proto/pty/server/cmd/web 五包 PASS，跨 phase 回归闸）"
        status: pass
    human_judgment: false

# Metrics
duration: 14min
completed: 2026-08-17
status: complete
---

# Phase 03 Plan 05: 前端连接流程认证化改造 Summary

**connect() 函数包裹 WS 直连：同源 fetch POST /api/attach 取一次性 ticket 入 Hello，auth_failed 单次门闩静默重试，ws/wss 按页面协议分支，dist 产物重建入库**

## Performance

- **Duration:** 14 min
- **Started:** 2026-08-17T09:27:50Z
- **Completed:** 2026-08-17T09:41:53Z
- **Tasks:** 2
- **Files modified:** 2（web/src/main.ts、web/dist/index.html）

## Accomplishments

- D-02 前端半侧落地：零新 UI 组件——先导航 GET / 的浏览器原生 Basic 弹窗是唯一凭据入口，connect() 内同源 fetch 自动携带缓存凭据取 ticket；401/429/catch 各落复用现状三态面板的人话文案（401 引导 reload 重新触发原生弹窗，Pitfall 6；429 提示稍候重试）
- D-10 前端半侧落地：onclose 见 `lastError?.code === 'auth_failed'` 且 retriedAuth 未置位 → 置位 + 清 lastError + `void connect()` 重取 ticket 重试一次；重试再失败落既有 1008 分支展示，非无限循环（T-03-25 缓解）
- 无认证模式零摩擦：404 探测 → ticket=undefined 直连，Hello JSON 自动省略 ticket 键（03-01 omitempty 契约兼容）；wss scheme 分支使 03-04 TLS 伺服可被 https 页面真实连接
- dist 产物与新流程同步入库：index.html 含 /api/attach 与 wss:// 检索串；go test ./... 五包全绿（跨 phase 回归零破坏）

## Task Commits

Each task was committed atomically:

1. **Task 1: main.ts——connect() 认证感知连接流程 + auth_failed 重试一次** - `dc1f177` (feat)
2. **Task 2: dist 重建与产物提交** - `8eb2fd0` (chore)

**Plan metadata:** 见最终 docs 提交（随 STATE/ROADMAP 更新同 commit）

## Files Created/Modified

- `web/src/main.ts` — connect() 认证感知连接流程：fetch 五路分派取 ticket → new WebSocket（scheme 按 location.protocol）→ Hello{version,cols,rows,ticket?} → auth_failed 守卫单次重试；ws 提升模块级可空句柄；onmessage/onerror/onclose 既有逻辑逐字移入；帧常量注释区与 proto.go 对齐（D-16）
- `web/dist/index.html` — pnpm -C web build 全链（tsc && vite build && gzip -k9）重建产物，内嵌新连接流程

## Decisions Made

- **ws 可空声明 + null 闸**（Rule 1 自动修复，详见 Deviations #1）
- **.gz 不入库按仓库 .gitignore 既定策略**（plan 字面冲突，详见 Deviations #2）
- **401 与其余非 ok 同口径**：通用认证失败面板不细分状态（无 oracle 纪律延伸到前端文案，T-03-26）；catch 面板逐字复用既有 unreachable 文案（UI-SPEC 逐字文案纪律）
- **const sock = ws 闭包句柄**：TS 严格模式对模块级可空 let 不做闭包收窄，handler 体内引用本地确定句柄；onData/sendResize 常驻接线读模块级当前连接（重试后自动指向新连接）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ws 声明为 `WebSocket | null` + onData/sendResize null 闸**
- **Found during:** Task 1（main.ts connect() 改造）
- **Issue:** plan 字面 `let ws: WebSocket`（connect() 内赋值）存在运行时回归窗口——`void connect()` 启动后 fetch 异步等待期间 ws 值为 undefined，终端已打开可聚焦，用户敲击触发 onData 读 `ws.readyState` 抛 TypeError（旧代码 ws 顶层同步创建零窗口）；sendResize 同理（虽有 helloSent 门在前，防御顺序仍应补齐）
- **Fix:** 声明改 `let ws: WebSocket | null = null`；onData 加 `ws !== null &&` 前置条件、sendResize 加 `ws === null ||` 前置条件（既有 readyState/helloSent 门原样保留，输入静默吞掉符合 UI-SPEC 面板期丢输入纪律）；connect() 内 `const sock = ws` 供 handler 闭包引用（TS 严格模式闭包不收窄模块级可空 let）
- **Files modified:** web/src/main.ts
- **Verification:** tsc --noEmit 退出 0（strict 全绿）；null 闸语义与「面板显示期间输入静默丢弃」现状一致
- **Committed in:** dc1f177（Task 1 commit 一部分）

**2. [Plan 字面与仓库既定策略冲突] .gz 产物不入库，仅提交 index.html**
- **Found during:** Task 2（dist 产物提交）
- **Issue:** plan action 指示「git add 两个产物文件随本 plan 提交」并引 README 构建节为据；但 `.gitignore` 自建仓 commit c055b41 起含 `web/dist/*.gz`，.gz 从未被跟踪（HEAD 中不存在），`git add` 被忽略规则拦截；README:40「及其 .gz」声明为陈旧笔误（与同 commit 的 .gitignore 矛盾）。且 gzip 头嵌原始文件 mtime——每次构建 .gz 字节必然漂移，强制入库将使每次前端构建都脏库（这正是忽略规则的存在理由）；embed.go:4 注释亦确认设计意图为 index.html 入库、.gz 缺失时明文伺服优雅降级
- **Fix:** 按仓库既定策略仅提交 `web/dist/index.html`；.gz 已在磁盘同步重建（本地构建的二进制仍走预压缩旁路）；README 陈旧声明登记 `.planning/phases/03-auth/deferred-items.md`（范围外不修）
- **Files modified:** 无（策略执行，无代码变更）；web/dist/index.html 正常入库
- **Verification:** `git ls-files web/dist/` 确认跟踪集不变；plan 核心目标「裸 clone 构建嵌的是认证感知前端」由 index.html 单独完整达成（go build ./... 绿 + 检索串证据）
- **Committed in:** 8eb2fd0（Task 2 commit，message 内注明策略依据）

---

**Total deviations:** 2 auto-fixed（1 Rule 1 bug 修复，1 plan-vs-repo 策略冲突按仓库执行）
**Impact on plan:** 两处均为正确性/仓库卫生必需，无范围蔓延；plan 全部 must_haves truths 与验收标准达成（.gz 入库一条按仓库更高优先级策略调和，plan 意图——产物与源码同步可裸建——完整成立）

## Issues Encountered

- pnpm 构建一次通过，无依赖/工具链问题；Go 全量测试（含 03-01..03-04 认证套件）在 main.ts 改造后全绿，确认前端协议常量改动与 proto.go 对齐无漂移

## Known Stubs

None——无占位/空数据/TODO 残留；三处新面板文案均为完整人话逐字稿，ticket 全链路（fetch → 闭包 → Hello 载荷）真实接线

## Threat Flags

None——新网络面（fetch /api/attach、wss 分支、auth_failed 重试门闩）均已在 plan <threat_model> 登记（T-03-24/25/26），无新增未建模暴露面

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- SEC-02 前后端闭环成型：浏览器 →（Basic 弹窗）→ /api/attach →（ticket）→ Hello 核销 →（auth_failed）→ 静默重试一次；03-06 可汇总执行 human-check 清单（弹窗凭据缓存/fetch 携带/过期重试/TLS 实测）
- 浏览器人工确认项（A2 假设：HTTP auth 缓存条目随同源 fetch 自动附带）留待 03-06 UAT——若该假设不成立，fallback 需重回本文件 fetch credentials 处理（Pitfall 6 已标注验证点）

## Self-Check: PASSED

- FOUND: web/src/main.ts（connect()/retriedAuth/auth_failed/wss 分支在位）
- FOUND: web/dist/index.html（含 /api/attach 与 wss:// 检索串，时间戳新于 main.ts）
- FOUND: commit dc1f177（feat(03-05) main.ts 改造）
- FOUND: commit 8eb2fd0（chore(03-05) dist 重建）
- 验证链全绿：tsc --noEmit ✓ / pnpm -C web build ✓ / go build ./... ✓ / go test ./... 五包 PASS ✓

---
*Phase: 03-auth*
*Completed: 2026-08-17*
