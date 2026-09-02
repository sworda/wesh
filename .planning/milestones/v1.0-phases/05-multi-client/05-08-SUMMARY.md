---
phase: 05-multi-client
plan: 08
subsystem: ui
tags: [typescript, xterm, websocket, share-link, promotion, osc52, copywriting, multi-client, vite, singlefile]

# Dependency graph
requires:
  - phase: 05-multi-client plan 02/03/04/06/07
    provides: 1013 慢消费者踢出服务端侧（05-02）；升格 Welcome 推送与 prefs 双档（05-03）；resize 仲裁与 ro RESIZE 服务端忽略第二闸（05-04）；/s/{token}/ 页面门禁与 /api/attach token 分支（05-06）；/api/attach 503 早闸与 WS ③位 503（05-07）
provides:
  - /s/{token}/ 进入流程：^/s/([^/]+)/$ 提取 shareToken → POST /api/attach body JSON{token}（无 token 空 body 现状不变）；token 红线注释（闭包+body only，禁 URL 重写 API 剥离）
  - fetch 响应分派矩阵七行（UI-SPEC 逐字）：ok/404 探测/401 携 token→C-3 Invalid share link/401 未携→P3 不变/429 不变/503→C-2 Server is full/throw→C-4
  - onclose 1013→C-1 Disconnected 专版（只认 code 不渲染 reason，零自动重连）；C-4 三处同源 UNREACHABLE_BODY（R1）；default→C-5（R2）；C-6 共用 HINT_RESTART（R3）；Session ended (1000) 不变
  - sendResize `if (isRO) return` 门（D-09 第一闸）；WELCOME 升格 rw 分支五步（isRO/disableStdin 翻转→setTitle 去前缀→fit.fit() 尺寸上报→prefs 幂等重放→beforeunload 幂等重注册）
  - osc52Loaded 模块级一次性门闩（D-13 前端落点）；ro 分支一次性 console.info 反馈（review #4 三要素）；dist 重建入库（mtime 本次构建时刻）
affects: [05-09 README/UAT（phase05.mjs 协议层验证：ro/rw 链接全链、满员 503、1013 关闭码断言锚点全部就绪）, Phase 6 CORE-05（重连能力边界——本 plan 1013 零自动重连为 D-10 既定形态）]

# Actuals (#2632)
actuals:
  tokens: 4448   # main.ts realized diff 17792 chars / 4；dist 为生成产物不计入本口径（minified 单行整体替换会淹没信号，estimate 55000 同为源码工作量口径）
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "文案同源常量化：UNREACHABLE_BODY（C-4 三处：fetch catch/onerror/onclose !opened）与 HINT_RESTART（C-6：1008/1009/1011）单写口定义——UI-SPEC『三处同源』的源码形态，同时满足验收 grep ==1 机械约束"
    - "验收 grep ==0 红线断言的注释纪律：被禁串（旧文案旧句、被禁 API 名）不得以引述形式进源码注释——机械 grep 对注释同样计数，指代式表述替代逐字引用"
    - "minified 产物行为断言形态：标识符断言不可达（esbuild 重命名全部模块级变量，仅属性名幸存）——以结构指纹（osc52===!0&&X&&!X&&(X=!0）锁定逻辑本体，字符类通配适配任意压缩名"
    - "升格分支幂等设计：握手 Welcome 与升格 Welcome 同分支无害（isRO 重复赋 false/fit 重复触发/prefs 重放/beforeunload 同参去重均幂等），零状态机分支"

key-files:
  created: []
  modified:
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "[Phase 05-08]: C-4/C-6 文案常量化（UNREACHABLE_BODY/HINT_RESTART 单写口）——验收 grep ==1 约束与 UI-SPEC『三处同源』的机械调和；旧句引用不得进源码注释（验收 grep ==0 红线断言是源码级机械检查，注释提及旧句字面同样计数）"
  - "[Phase 05-08]: dist 产物 osc52Loaded 验收断言以结构指纹替代标识符 grep——esbuild 压缩重命名全部模块级标识符（helloSent/isRO/showStatus 均不入产物，仅属性名 disableStdin/osc52 幸存），grep 'osc52Loaded' 恒 0；指纹 osc52===!0&&X&&!X&&(X=!0 锁定门闩逻辑本体，比裸标识符更强证据"

patterns-established:
  - "fetch 分派矩阵落码形态：401 按 shareToken !== undefined 二分（携 token→C-3 专版/未携→P3 既有）——前端自知本次请求内容分派文案，无 oracle 纪律不约束前端侧"
  - "升格五步的前后段分工：1-3 步（翻转/标题/fit）显式落 rw 分支内，4-5 步（prefs/beforeunload）由既有流程照常执行承载 + 幂等注释登记——不重排既有代码顺序"

requirements-completed: [MULTI-02, MULTI-03, MULTI-04, MULTI-05]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "/s/{token}/ 进入流程与 fetch 响应分派矩阵（MULTI-05 前端半侧）：shareMatch/shareToken 提取、body JSON{token}、ok/404/401×2/429/503/throw 七行分派、C-3 Invalid share link 与 C-2 Server is full 专版"
    requirement: MULTI-05
    verification:
      - kind: other
        ref: "pnpm -C web exec tsc --noEmit（exit 0）+ grep 断言（JSON.stringify({ token ==1、history.replaceState ==0、Invalid share link|Server is full|Disconnected ==3）"
        status: pass
    human_judgment: false
  - id: D2
    description: "onclose 文案契约（MULTI-03 前端半侧 + R1/R2/R3 清扫）：1013→C-1 Disconnected 专版（只认 code 不渲染 reason、零自动重连）、C-4 三处同源改写、default→C-5、1008/1009/1011→C-6 共用提示行、Session ended 不变"
    requirement: MULTI-03
    verification:
      - kind: other
        ref: "grep 断言（another client is already attached ==0、To reattach from the latest output, ==1、If the problem persists… ==1）+ node --test src/lib/*.test.ts 16/16"
        status: pass
    human_judgment: false
  - id: D3
    description: "ro 不发 RESIZE 第一闸与升格 rw 分支五步（MULTI-02/MULTI-04 前端半侧）：sendResize isRO 门；Welcome rw → disableStdin 翻转 + setTitle 去 [ro] 前缀 + fit.fit() 尺寸上报 + prefs/beforeunload 幂等承载"
    requirement: MULTI-02
    verification:
      - kind: other
        ref: "grep 断言（if (isRO) return ==1、disableStdin = false ==1）+ tsc exit 0 + node --test 16/16"
        status: pass
    human_judgment: false
  - id: D4
    description: "OSC52 一次性门闩与 ro console 反馈（D-13 前端落点 + review #4）：osc52Loaded 模块级 flag 防二次注册；ro 分支 console.info 三要素一次性反馈；dist 重建入库且 mtime 为本次构建时刻"
    requirement: MULTI-02
    verification:
      - kind: other
        ref: "time pnpm -C web build（exit 0，2.5s）+ ls mtime 验证（2026-08-21 07:24:41 +0800）+ dist 检索串（Invalid share link/Server is full/Disconnected/read-only mode 各 ==1）+ 门闩结构指纹 ==1"
        status: pass
    human_judgment: false

# Metrics
duration: 22min
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 08: 前端多客户端改造（/s/ 进入流程 + 分派矩阵 + 三专版 + 升格分支）Summary

**/s/{token}/ 分享链接进入流程与七行 fetch 响应分派矩阵落地（token 红线：闭包+body only、replaceState 零调用），1013/503/无效链接三条专版与单客户端语义文案全清扫（C-1..C-6 逐字），sendResize isRO 第一闸 + WELCOME 升格 rw 五步 + osc52Loaded 门闩 + ro 一次性 console 反馈，dist 重建入库时间戳验证通过，tsc/build/既有测试全绿。**

## Performance

- **Duration:** 22min
- **Started:** 2026-08-20T23:08:40Z
- **Completed:** 2026-08-20T23:31:00Z
- **Tasks:** 2（Task 1 进入流程+文案 + Task 2 升格+门闩+dist）
- **Files modified:** 2（web/src/main.ts、web/dist/index.html）

## Accomplishments

- **分享链接进入流程**（MULTI-05 前端半侧）：`location.pathname` 匹配 `^/s/([^/]+)/$` 提取 shareToken → `fetch('/api/attach', {method:'POST', headers, body: JSON.stringify({token: shareToken})})`；无 token 保持空 body 现状；token 红线注释落码（只存 connect() 闭包变量与 POST body，禁 console/localStorage/sessionStorage/日志，禁 URL 重写 API 剥离——D-03 书签契约 + D-10 刷新入口）；前端不解析不分支 token 种类（Welcome.mode 唯一来源）
- **响应分派矩阵七行**（UI-SPEC §Share Link Entry Contract 逐字）：ok→ticket 继续；404→无认证探测直连（仅未携 token 可达）；401 携 token→C-3 Invalid share link 专版（前端自知携 token，无 oracle 不约束前端文案）；401 未携→P3 Authentication failed 不变；429→P3 Too many attempts 不变；503→C-2 Server is full 专版（OQ2 早闸）；fetch throw→C-4 改写后文案
- **onclose 三专版与文案清扫**：1013→C-1（Disconnected + "could not keep up with the session output" + "To reattach from the latest output,"——只认 ev.code 不渲染 reason 防伪造钓鱼，零自动重连 D-10）；C-4 三处旧句（"another client is already attached…"）→ UNREACHABLE_BODY 常量（R1）；default→C-5（"The connection closed unexpectedly." + "The session may still be running. To reattach,"，R2）；1008/1009/1011→HINT_RESTART 常量（R3）；Session ended (1000) 零改动
- **ro 不发 RESIZE 第一闸**（D-09）：sendResize 在 !helloSent 门之后加 `if (isRO) return`——Hello 携首尺寸不受影响（helloSent 门先于 isRO 生效），服务端忽略为第二闸（05-04 已落）
- **WELCOME 升格 rw 分支五步**（§RO Mode & Promotion Contract）：isRO=false + disableStdin=false → setTitle() 去 [ro] 前缀（单一写口）→ fit.fit() 触发 onResize→sendResize 上报当前尺寸（owner 尺寸接管前端半侧，排队期窗口变化纠正）→ prefs 应用段照常（queryKeys 跳过幂等）→ beforeunload 条件重注册（addEventListener 同参去重幂等）；握手 Welcome 同分支无害，降级路径不存在不实现
- **OSC52 一次性门闩**（D-13）：模块级 osc52Loaded flag——升格 Welcome 重放 prefs 防 ClipboardAddon 二次注册 OSC52 handler；ro 端永远收不到 osc52:true → 永不加载（无需 ro 特判）
- **ro 一次性 console 反馈**（review #4 采纳项）：WELCOME ro 分支内 console.info 三要素（输入不发送 + 窗口小于会话尺寸输出可能裁剪 + reattach 恢复）——旁观者与递补者同一条，零新 UI 不违 D-07，串内零 token 零动态内容
- **dist 重建入库**：`time pnpm -C web build`（2.5s）→ mtime 2026-08-21 07:24:41 +0800 为本次构建时刻（时间戳验证约束）；.gz 按 .gitignore 既定策略不入库

## Task Commits

Each task was committed atomically:

1. **Task 1: /s/ token 进入流程 + fetch 响应分派矩阵 + onclose 三专版与文案清扫** - `4e1b9d9` (feat)
2. **Task 2: WELCOME 升格 rw 分支 + sendResize isRO 门 + OSC52 门闩 + dist 重建入库** - `5849335` (feat)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md）

## Files Created/Modified

- `web/src/main.ts` - shareMatch/shareToken 提取与 fetch body 携 token 分支；七行响应分派矩阵（C-3/C-2 专版）；UNREACHABLE_BODY/HINT_RESTART 同源常量；1013→C-1、default→C-5、1008/1009/1011→C-6；sendResize isRO 门；WELCOME 升格 rw 分支；osc52Loaded 门闩；ro console.info；R4 浮层注释修订
- `web/dist/index.html` - 重建后的单文件产物（含全部新文案与逻辑；minified）

## Decisions Made

- **C-4/C-6 文案常量化单写口**：验收断言要求 `If the problem persists…` grep ==1 且 UI-SPEC 要求 C-4『三处同源』——常量化（UNREACHABLE_BODY/HINT_RESTART）同时满足两者，且消除三处文案漂移面。
- **onerror 注释 409→WS 握手 503 同步**：05-01 拆 409 闸后原注释（"含第二客户端 409"）事实错误；C-4 通用面板按 OQ2 裁决注记正是 WS 握手阶段 503 的落点（早闸后竞态窗口浏览器不暴露握手状态码）——注释随 C-4 改写同点修正，零行为变化。
- **升格分支不区分握手/运行期 Welcome**：握手 Welcome（attach 即 rw）与升格 Welcome 走同一 rw 分支——isRO 本就 false、disableStdin 重复赋 false、fit.fit() 重复触发均幂等，零状态机分支（plan 注释授权逐字）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 验收红线 grep 与源码注释字面冲突**
- **Found during:** Task 1 验收断言阶段（`grep -c 'history.replaceState' == 0` 与 `grep -c 'another client is already attached' == 0` 双双失败，实测各为 1）
- **Issue:** 初版注释以引述形式写入被禁串字面——connect() 红线注释直书 `history.replaceState` API 名、常量注释逐字引用旧句 "another client is already attached (wesh currently allows a single client)"；机械 grep 对注释同样计数，断言无法通过
- **Fix:** 注释改为指代式表述——"禁经 URL 重写 API 剥离 URL token"（plan action 原文即此措辞）与"旧版『另一客户端已连接（单客户端）』表述事实错误"；红线语义与警示作用完整保留
- **Files modified:** web/src/main.ts
- **Verification:** 两条 ==0 断言通过，其余四条断言不受影响全绿
- **Committed in:** 4e1b9d9（Task 1 提交）

**2. [Rule 3 - Blocking] dist 产物 osc52Loaded 标识符 grep 断言机械不可达——以结构指纹替代**
- **Found during:** Task 2 验收断言阶段（`grep -c 'osc52Loaded' web/dist/index.html` 得 0，verify 链 grep -c 零匹配 exit 1 阻断）
- **Issue:** vite/esbuild 默认压缩重命名全部模块级标识符——实证 helloSent/isRO/queryKeys/showStatus/UNREACHABLE_BODY 在产物中均 0 次，仅属性名（disableStdin、osc52 prefs 键）幸存；plan 验收假设标识符入产物，在既定构建契约下结构性不可达（改构建配置关压缩属 Rule 4 架构变更，不可为满足 grep 而动）
- **Fix:** 源码断言保持（`grep -c 'osc52Loaded' web/src/main.ts` == 5 ≥ 1 ✓）；产物断言改以门闩结构指纹 `osc52===!0&&X&&!X&&(X=!0`（字符类通配任意压缩名）——锁定门闩逻辑本体（条件+否定检查+置位包裹 loadAddon 调用），证据强于裸标识符出现；实测产物指纹 ==1（压缩形态 `osc52===!0&&em&&!Hp&&(Hp=!0,$.loadAddon(…`）
- **Files modified:** 无（验证方式替代，非代码变更）
- **Verification:** 指纹断言通过；dist 其余检索串（Invalid share link/Server is full/Disconnected/read-only mode）各 ==1 全绿
- **Committed in:** 5849335（Task 2 提交，无额外代码改动）

---

**Total deviations:** 2 auto-fixed（均 Rule 3 - Blocking，验收断言层适配——生产代码行为零偏离）
**Impact on plan:** 两处均不改变 plan 锁定的行为/文案契约本体——第一处是注释措辞合规化，第二处是产物验证方式等价强化；构建契约（默认压缩）与全部行为断言逐字保持。

## Threat Model 处置

| Threat ID | 处置 | 证据 |
|-----------|------|------|
| T-05-09（1013/关闭帧伪造引导钓鱼，medium，mitigate） | **已落地** | onclose 只认 ev.code 不渲染 reason（1013 注释逐字登记 slow_consumer 机器串纪律）；C-1/C-2/C-3 文案全部硬编码英文（UI-SPEC 逐字）；Reload this page 链接仅 location.reload() 无外链（showStatus 既有形态零改动） |
| T-05-06b（token 经前端面泄露，high，mitigate） | **已落地** | shareToken 只存 connect() 闭包变量与 POST body（红线注释落码）；`history.replaceState` 源码零调用（验收断言 ==0 锁定）；console.info 调用行核读无 shareToken/ticket 引用（红线核读项）；禁止剥离 URL token 的 D-03/D-10 论证注释在案 |
| T-05-09b（升格 Welcome 伪造，low，accept） | **登记维持** | 传输层 TLS+Origin+ticket 链路既有（P3）；服务端 INPUT 门按注册表 mode 独立判定（05-03/05-05），前端 disableStdin 只是 UX 层——伪造帧无法让服务端接受 INPUT |

无新增威胁面——无新端点（/api/attach 与 /s/ 均为 05-06 既有）、无新协议帧（升格复用 'W' Welcome）、无新依赖、无新可视组件（D-07）。

## Known Stubs

None — 本 plan 无新增占位 stub（无硬编码空值/占位文案/TODO/FIXME；全部 verify 均已运行）。1013 占位文案（"The server asked this client to retry later."）在本 plan 被 C-1 专版正式替换，占位清零。

## Issues Encountered

- **验收断言与实现细节的两次机械冲突**（见 Deviations 1/2）：均以验证等价化收口，非生产代码缺陷；修正后全部断言绿。
- 无其他问题；tsc/build/既有套件一次通过。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的后续挂点）

- **05-09 UAT phase05.mjs 协议层验证**：ro/rw 链接全链（stdout 链接 → GET 页面 → fetch /api/attach 携 token → ticket → Welcome mode 断言）、满员 503 场景（--max-clients 小值 spawn）、1013 踢出关闭码与 slow_consumer reason 断言、双客户端 OUTPUT 一致性（UI-SPEC backstop 行）——本 plan 交付的前端分派矩阵与专版文案即其断言锚点
- **05-09 README**：旁观者裁剪行为说明（窗口 < PTY 尺寸看到裁剪画面、重新 attach 恢复——与 ro console.info 文案同义）；分享链接暴露面清单（T-05-06/T-05-06e 残余登记，05-06 移交）
- **Phase 6 CORE-05**：自动重连完整能力（指数退避+上限+手动入口）——本 plan 1013 零自动重连为 D-10 既定边界

## Next Phase Readiness

- 05-09：前端全部锚点就绪——/s/ 进入流程、三专版文案检索串、升格行为、门闩逻辑均可经协议层/产物 grep 断言；--max-clients flag 可 spawn 驱动满员场景
- 无阻塞项；tsc exit 0、`time pnpm -C web build` 2.5s 绿、既有 node --test 16/16 零回归、dist 时间戳验证通过

## Self-Check: PASSED

- FOUND: commit 4e1b9d9（Task 1）、5849335（Task 2）均在 git log；两提交均无意外文件删除（--diff-filter=D 检查通过）
- FOUND: web/src/main.ts 检索串——`JSON.stringify({ token` ==1、`history.replaceState` ==0、`Invalid share link|Server is full|Disconnected` ==3、`another client is already attached` ==0、`To reattach from the latest output,` ==1、`If the problem persists, restart wesh from your shell, then` ==1、`if (isRO) return` ==1、`disableStdin = false` ==1、`osc52Loaded` ==5、`read-only mode` ==1、`console.info` ==1（调用行零 shareToken/ticket 引用，红线核读通过）
- FOUND: web/dist/index.html 检索串——Invalid share link/Server is full/Disconnected/read-only mode 各 ==1；门闩结构指纹 `osc52===!0&&X&&!X&&(X=!0` ==1
- pnpm -C web exec tsc --noEmit exit 0；time pnpm -C web build exit 0（2.5s）；dist/index.html mtime 2026-08-21 07:24:41 +0800 为本次构建时刻；node --test src/lib/*.test.ts 16/16 pass

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
