---
phase: 09-release-polish
plan: "03"
subsystem: ui
tags: [aria, role-alert, websocket-close-code, 1001-shutdown, jsdom, blackhole-fixture, esbuild, dist-fingerprint, d18-ship-clearance]

requires:
  - phase: 09-release-polish
    provides: "09-01 phase 骨架与发布序（wave 2 依赖锚）；07-05 D-23 1001 优雅下线面板与 CORE-05 重连状态机（本 plan 修订面）；05-08 文案常量化单写口纪律（第三次沿用）"
provides:
  - "D-18 三项 UI WARNING 清零（07-UI-REVIEW / 07 deferred-items.md 三行路由终点）：① 1001 面板 hint 条件句式（C-10 定稿逐字）② #status role=\"alert\" 面板族 AT 播报 ③ pre-onopen 1001 按码分派先于 !opened 截流"
  - "showShutdown 单写口 helper（1001 面板 title/body/hint 三件套唯一调用形态）+ HINT_SHUTDOWN 常量（C-10 条件句式——systemd 自重启/非自重启两部署形态下皆为真）"
  - "phase06-dom.mjs D12/D13 新场景与黑洞 TCP 夹具（accept-never-upgrade：onopen 结构性永不触发，pre-onopen 窗口无限驻留零竞态）——jsdom 行为锁 D11a/D12a/D13a"
  - "dist 真实产物重建：C-10 字面与 role=alert 入产物、旧 hint 字面恰 1 处（case 1000 保留）——v1.0.0 发布物无已知 UI WARNING 挂账"
affects: [09-release-polish（09-09 README 发布面描述）, ship]

actuals:
  tokens: 19200
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "黑洞 TCP 伺服器夹具（createServer accept 后永不应答 + SpyWebSocket 构造器 URL 端口改写）：pre-onopen 驻留形态的确定性构造——onopen 结构性永不触发使 opened 恒 false 且无原生 close/error 事件竞争断言面"
    - "产物指纹断言以 grep -o 计出现次数：esbuild 单行 bundle 下 grep -c 计行数失真（恒 1）；esbuild 不去重字符串字面量（实证：旧 hint 改前产物 2 处 → 改后 1 处）"
    - "UAT 对 dist 的 TDD RED 演证形态：先更新断言对旧产物跑出 FAIL（三断言恰证 bug 形态）再重建转 GREEN——jsdom-对-dist 类改动的可归因回归证据"

key-files:
  created: []
  modified:
    - web/src/main.ts
    - web/index.html
    - web/uat/phase06-dom.mjs
    - web/dist/index.html

key-decisions:
  - "D-18 三项 as-locked 落地：HINT_SHUTDOWN C-10 条件句式常量 + showShutdown 单写口（pre-onopen 分派与稳态 case 1001 唯一调用形态）；case 1001 在分派序修订后实际不可达——按 plan 字面保留为防回归纵深（单写口使两处零文案漂移，验收 grep showShutdown ≥3）"
  - "D13 夹具取黑洞 TCP 伺服器而非 plan 括注建议的 hold/不发 Hello：fetch hold 期间 WS 未构造（无 socket 可驱动）；『不发 Hello』不保 opened=false（opened 在 WS onopen 即置位、先于 Hello 发送）——两建议形态结构性不可达，黑洞形态 accept-never-upgrade 使 onopen 永不触发，确定性零竞态（Rule 3 deviation 登记）"
  - "TDD RED 先证：更新后的 phase06-dom 对未重建旧 dist 跑出 D11a/D12a/D13a 三 FAIL（D13a title=Unable to connect 正是 WARNING#3 bug 形态）→ 重建 dist 后 40/40 全绿——按 09-02 先例 task 级 tdd 在 plan 既定 feat/test 提交结构内兑现"

patterns-established:
  - "pre-onopen 窗口驻留夹具形态（黑洞 TCP + URL 端口改写经 loadTerminal opts 注入）——未来任何『握手未完成收 X』类前端分派断言的复用模板"

requirements-completed: [OPS-10]

coverage:
  - id: D1
    description: "R1（D-18①）1001 关停面板 hint 条件化：HINT_SHUTDOWN 常量（C-10 逐字 'If wesh is not restarted for you, start it again from your shell, then'）+ showShutdown 单写口 helper——条件句式通吃 systemd Restart= 自重启与非自重启两部署形态；case 1000（Session ended）旧 hint 逐字不动"
    requirement: OPS-10
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs#D11a/D11c（稳态 1001 与重连上下文 1001 面板 hint 含 C-10 前缀逐字，40/40 全绿 2026-08-30）"
        status: pass
      - kind: other
        ref: "pnpm -C web exec tsc --noEmit 退出 0；grep 机械断言（C-10 字面 main.ts 恰 1、HINT_SHUTDOWN 恰 2、旧 hint 恰 1）"
        status: pass
    human_judgment: false
  - id: D2
    description: "R2（D-18②）#status 容器 role=\"alert\" 单属性：面板族（C-1..C-10/Reconnecting）整体获 assertive 播报（隐含 aria-live=assertive + aria-atomic=true）；零视觉影响零 CSS 改动零新顶层元素"
    requirement: OPS-10
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs#D12a（jsdom 读取 #status getAttribute('role')==='alert'，PASS 2026-08-30）"
        status: pass
    human_judgment: false
  - id: D3
    description: "R3（D-18③）pre-onopen 1001 按码分派：onclose 在 !opened 截流之前先分派 ev.code===1001 → 落与稳态完全同一 showShutdown 单写口 C-10 专版（不再误述为 C-4 'Unable to connect / refusing new connections'）；不触发重连（1001 不在 CORE-05 触发集，07-05 D11b 回归锁零漂移）"
    requirement: OPS-10
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs#D13a/D13b（黑洞夹具 opened=false 收 1001 → title='Server shutting down' 非 'Unable to connect' + 2.5s 守候窗零新连接；RED 证据：重建前同场景 FAIL title=\\\"Unable to connect\\\"）"
        status: pass
    human_judgment: false
  - id: D4
    description: "dist 真实产物重建（P1 D-18 构建链 pnpm -C web build 先于 go 侧消费）：产物含 C-10 新文案串与 role=alert，源码/产物断言面守恒（esbuild 重命名标识符——断言以字符串字面量与 HTML 属性为指纹）；前端零新运行时依赖（pnpm-lock.yaml 逐字节不动）"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "time pnpm -C web build 成功；grep -oF 指纹：C-10 前缀 dist ≥1、role=alert ==1、旧 hint 恰 1 处（case 1000 保留）；git diff --exit-code web/pnpm-lock.yaml"
        status: pass
      - kind: automated_ui
        ref: "回归三脚本：phase06-dom 40/40 + phase04-dom 37/37 + phase05-dom 19/19 全绿退出 0（assertOutputClean 自净通过）"
        status: pass
    human_judgment: false
  - id: D5
    description: "role=alert 真实屏幕阅读器播报与节流行为（含 Reconnecting 1Hz 倒计时 assertive 重读边界）——按 must_haves backstop 行以 skipped+reason 记录（D12b），风险接受登记于 09-UI-SPEC §D-18 ② 已知边界（v2 候选：真实 AT 噪音反馈再评估 Reconnecting live region 拆分）"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "web/uat/phase06-dom.mjs#D12b（skip+reason 在场：平台原生豁免面——真实 AT 栈按 CODEBUDDY.md 分层测试策略豁免，jsdom 仅断言 role 属性；skip 记录即 must_haves 定义的完成形态）"
        status: pass
    human_judgment: true
    rationale: "真实屏幕阅读器播报/节流行为属平台原生行为显式豁免面（CODEBUDDY.md 分层测试策略第 5 条：不列为阻塞项，skipped+reason 记录并风险接受）；headless Linux 侧无真实 AT 栈可驱动。verify 时请人工确认该风险接受仍然成立（D-18② 裁决单属性最小修复 + UI-SPEC 已知边界登记）。"

duration: 13min
completed: 2026-08-30
status: complete
---

# Phase 9 Plan 3: D-18 Ship-Clearance（三项 UI WARNING 清零）Summary

**1001 关停面板 C-10 条件句式 hint + #status role="alert" AT 播报 + pre-onopen 1001 按码分派——07-UI-REVIEW 三项 WARNING 全结，jsdom 行为锁（D11a 更新 + D12/D13 新场景）与 dist 产物重建全绿，v1.0.0 发布物无已知 UI WARNING 挂账**

## Performance

- **Duration:** 13 min
- **Started:** 2026-08-30T03:58:55Z
- **Completed:** 2026-08-30T04:12:07Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- **R1（WARNING#1）**：`case 1001` hint 由旧文案（自重启形态下为无效指引）改为 C-10 定稿条件句式 `HINT_SHUTDOWN`——一条文案在 systemd Restart= 自重启与非自重启两部署形态下皆为真（静态替换非运行时分支）；title/body 逐字不变（07-UI-REVIEW 判定 house-legal）
- **R2（WARNING#2）**：`web/index.html` `#status` 容器加 `role="alert"` 单属性——面板族整体获得 assertive 播报，零视觉影响零 CSS 改动
- **R3（WARNING#3）**：onclose 分派序修订——`!opened` 截流之前先按码分派 `ev.code === 1001`，pre-onopen 优雅关停落 C-10 专版（不再误述为「refusing new connections」）；重连上下文 1001 仍先经 stopReconnect 清循环（D11c 零漂移）
- **单写口纪律**：`showShutdown` helper 承载 1001 面板三件套唯一调用形态（pre-onopen 分派 + 稳态 case 1001 两处同源，05-08 常量化纪律第三次沿用）；`case 1001` 按计划字面保留为分派序防回归纵深
- **TDD RED→GREEN**：更新后断言对未重建旧 dist 先跑出三 FAIL（D11a 旧 hint / D12a role=null / D13a title="Unable to connect"——恰为三项 WARNING 的 bug 形态）→ `time pnpm -C web build` 重建后 40/40 全绿
- **回归**：phase06-dom 40/40（含 D12/D13 新场景、D12b/D9 两 skip 均平台豁免）+ phase04-dom 37/37 + phase05-dom 19/19；assertOutputClean 运行时自净通过；pnpm-lock.yaml 逐字节不动（零新依赖）

## Task Commits

Each task was committed atomically:

1. **Task 1: 源码修订——main.ts HINT_SHUTDOWN + showShutdown 单写口 + onclose 1001 优先分派（R1/R3）+ index.html role="alert"（R2）** - `43ea7e0` (feat)
2. **Task 2: jsdom 断言扩展 + dist 重建——phase06-dom.mjs D11a 更新 + D12/D13 新场景 + 前端 UAT 回归 + 产物指纹断言** - `6564963` (test)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP/WINDOWS）

## Files Created/Modified

- `web/src/main.ts` — HINT_SHUTDOWN 常量（常量族区 HINT_RESTART 旁）+ showShutdown 单写口 helper（showStatus 定义后）+ onclose 分派序修订（1001 先于 !opened 截流）+ case 1001 函数体改经 helper
- `web/index.html` — `#status` 容器加 `role="alert"` 单属性 + R2 注释登记（aria-live 语义与已知边界指针）
- `web/uat/phase06-dom.mjs` — 头注释覆盖清单补 D12/D13；D11a hint 断言改 C-10 前缀逐字；D12 场景（role 属性断言 + D12b 真实 AT 豁免 skip）；D13 场景（黑洞 TCP 夹具 pre-onopen 1001 分派 + 守候窗零新连接）；loadTerminal 增 blackholePort 注入（SpyWebSocket 构造器 URL 端口改写）
- `web/dist/index.html` — 真实产物重建（tsc && vite build && gzip -k9）：含 C-10 字面与 role="alert"

## Decisions Made

- **case 1001 保留为不可达纵深**：分派序修订后 `if (ev.code === 1001) return` 先行截流使 switch 的 case 1001 实际不可达——按 plan 字面保留（验收 grep showShutdown ≥3 的机械要求），注释如实登记「防回归纵深 + 单写口零漂移」；删除该 case 反而违反计划字面与验收
- **D13 夹具形态（Rule 3 deviation，登记 WINDOWS.md）**：plan 括注建议「hold 或不发 Hello 使 opened 保持 false」两形态结构性不可达（见 Deviations）——黑洞 TCP 伺服器（accept-never-upgrade）+ SpyWebSocket URL 端口改写使 onopen 永不触发，确定性构造 pre-onopen 驻留；fetch /api/attach 仍走真实 wesh 实例 404 探测直连链路
- **产物指纹以 grep -o 计出现次数**：esbuild 产物为单行 bundle，`grep -c` 计行数恒 1 失真；`grep -oF ... | wc -l` 计出现次数（esbuild 不去重字符串字面量——实证旧 hint 改前产物 2 处、改后恰 1 处）
- **deferred-items.md 不改状态**（08-05 gofmt 清零同款先例）：07-deployment/deferred-items.md 三行 UI WARNING 的闭合记录以本 SUMMARY + STATE.md blocker 行闭环为准，登记文件保持历史原貌

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] D13 夹具形态——plan 括注建议形态结构性不可达，取黑洞 TCP 夹具**
- **Found during:** Task 2（D13 场景实现）
- **Issue:** plan 对 D13 的夹具建议「hold 或不发 Hello 使 opened 保持 false」两形态均不可达——(a) fetch hold 期间 WS 未构造（sockets 数组空，无 socket 可 synthClose）；(b) 「不发 Hello」不保 opened=false（`opened` 在 WS onopen 即置位、先于 Hello 发送——main.ts:819-831）
- **Fix:** 黑洞 TCP 伺服器夹具：`createServer(() => {})` 监听随机 loopback 端口（接受连接永不完成 WS 升级）+ `loadTerminal` 新增 `opts.blackholePort` 使 SpyWebSocket 构造器改写 URL 端口——onopen 结构性永不触发（opened 恒 false 且无原生 close/error 事件竞争断言面）；fetch 不受影响仍走真实实例
- **Files modified:** web/uat/phase06-dom.mjs
- **Verification:** RED 运行中 D13a 以 title="Unable to connect" FAIL（旧产物 pre-onopen 1001 误述——夹具正确驱动 opened=false 路径）；重建后 D13a PASS title="Server shutting down"，D13b 守候窗零新连接
- **Committed in:** 6564963（Task 2 提交）

---

**Total deviations:** 1 auto-fixed（1 blocking——夹具形态替换，断言面与 plan 行为逐字一致）
**Impact on plan:** 夹具实现细节偏离，行为断言（title/hint/负断言/守候窗）与 plan 完全一致。无范围蔓延。

## TDD Gate Compliance

Plan type=execute（非 tdd）——plan 级 RED/GREEN 门序列不适用（09-02 先例）。两 task 均标 tdd="true"，以 plan 既定提交结构（Task 1 feat / Task 2 test）+ RED 演证兑现：Task 2 先更新断言对未重建旧 dist 运行，D11a/D12a/D13a 三 FAIL（三断言分别对应三项 WARNING 的 bug 形态——D13a title="Unable to connect" 即 WARNING#3 原文复现）→ dist 重建后全绿。RED 证据为运行日志（非独立提交——plan 字面指定单 test 提交含 dist 重建）。

## Issues Encountered

- `grep -c` 在 esbuild 单行 bundle 上计行数恒 1——产物「旧 hint 恰 1 处」断言改用 `grep -oF | wc -l` 计出现次数（改前 2 / 改后 1，兼证实 esbuild 不去重字符串字面量）
- 无其他问题：tsc 一次通过、三 UAT 脚本一次全绿、lockfile 零漂移

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 07-UI-REVIEW 三项 WARNING 全结——D-18 ship 清零兑现，v1.0.0 发布物无已知 UI WARNING 挂账
- 前端面板契约现状：C-1..C-10 + Reconnecting 全族带 role="alert" AT 播报语义；1001 面板三件套单写口锁定
- 09-09（README/发布面文档）可引用本 plan 结论：1001 面板文案已条件句式化（两部署形态皆真）
- 已知边界（非阻塞）：真实 AT 播报/节流行为属平台原生豁免面（D12b skip 登记，09-UI-SPEC §D-18 ② v2 候选路由）

## Self-Check: PASSED

- 09-03-SUMMARY.md 在场 ✓
- 任务提交 43ea7e0 / 6564963 均在 git log ✓
- 产物四文件在场（main.ts / index.html / phase06-dom.mjs / dist/index.html）✓
- 指纹复核：main.ts HINT_SHUTDOWN==2；index.html 与 dist/index.html role="alert" 各==1 ✓

---
*Phase: 09-release-polish*
*Completed: 2026-08-30*
