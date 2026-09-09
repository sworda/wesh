---
phase: 06-session-lifecycle
plan: 03
subsystem: frontend
tags: [typescript, websocket, reconnect, xterm, state-machine]

requires:
  - phase: 06-session-lifecycle
    provides: 06-01 前端 EXIT 承接（lastExit 暂存 + onclose 1000 正文）与 per-connection 重置块（IN-01）、P3 connect() 可重入入口、P5 页面级门闩先例
provides:
  - web/src/lib/reconnect.ts：backoffMs(attempt)=min(1000*2**attempt,30000) + shouldReconnect(code)=code===1006 纯函数（node --test 锁定，零 DOM）
  - main.ts 重连状态机：startReconnect/scheduleAttempt/runAttempt/stopReconnect 四函数 + 页面级状态（reconnecting/attempt/reconnectTimer/retryAt/countdownTimer）
  - showStatus 第四可选参 action?:{label,onClick}（R3/OQ2 定稿形态——缺省 Reload 零漂移）
  - onclose case 1006 显式触发分派（D-01）+ 四 handler 代际守卫（Pitfall 6）+ online/offline 双触发（D-04）
  - Reconnecting 面板（C-9 逐字文案常量 RECONNECTING_TITLE/RECONNECTING_HINT + 等待期/在途双模板函数，1Hz 倒计时只写 #status-body）
  - 重连成功点：WELCOME 到达且 reconnecting → stopReconnect + term.clear()（D-05）
affects: [06-05, 06-06, phase-07]

actuals:
  tokens: 9000
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "重连循环单例门闩：三触发源（onclose(1006)/offline/online）全经『已在循环则幂等返回』入口；scheduleAttempt 入口清双 timer 保恰好一次（Pitfall 5 机械核心）"
    - "代际守卫：connect() 内四 socket handler 入口 if (sock !== ws) return（const sock = ws 闭包承载）；fetch 通道无 sock 可守卫——以 welcomeDone 作『新会话已建立』代际标记收口陈旧迟到失败"
    - "面板保护分层：循环内失败只更新 Reconnecting 计数（fetch throw→scheduleAttempt / onerror-!opened→return / 再 1006→scheduleAttempt）；终态分支（401/429/503/通用认证/带码关闭）先 stopReconnect 再落既有专版面板逐字"
    - "showStatus 动作链接参数化：第四可选参缺省保持 'Reload this page'+location.reload() 逐字——13 个既有调用点零改动（R3）"

key-files:
  created:
    - web/src/lib/reconnect.ts
    - web/src/lib/reconnect.test.ts
  modified:
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "fetch catch 补 welcomeDone 代际标记守卫（Rule 2）——D-04 既定形态（online/手动点击在 fetch 飞行中再启 attempt）引入双在飞 connect，较慢者迟到失败在重连成功后不得用 'Unable to connect' 覆盖健康会话（Pitfall 6 同族代际污染，fetch 通道无 sock 可守卫）"
  - "scheduleAttempt 入口清双 timer（Rule 2）——双在飞 attempt 先后失败重入 scheduleAttempt 不叠加定时器，Pitfall 5『恰好一次』从门闩语义落到定时器机械层"
  - "404 探测直连分支不设 stopReconnect——无认证模式重连链路继续走 WS，循环终止唯一挂点 = WELCOME 到达（契约：成功判定恒为 WELCOME，fetch 收到响应不等于接回）"
  - "shouldReconnect 谓词落 lib 纯函数（单测锁定 D-01 全集正反断言），main.ts 主分派按 plan 逐字取 case 1006/ev.code === 1006 字面形态——06-05 jsdom 断言共享同一事实源"

patterns-established:
  - "页面级重连状态声明于门闩区（osc52Loaded/retriedAuth 邻位）+ connect() per-connection 重置块刻意不重置的注释化纪律（IN-01 延伸）"
  - "面板文案三处同源单写口：常量 + 模板函数（RECONNECTING_TITLE/HINT + reconnectingWaitBody/reconnectingNowBody）——05-08 C-4/C-6 常量化先例第三次沿用"

requirements-completed: [CORE-05]

coverage:
  - id: D1
    description: "退避纯函数：backoffMs 序列 1000/2000/4000/8000/16000/30000/30000（封顶截断）+ 深尝试 backoffMs(10)=30000 + attempt 0 起点；shouldReconnect(1006)=true，1000/1002/1008/1009/1011/1013 全 false（D-01/D-02 逐字）"
    requirement: CORE-05
    verification:
      - kind: unit
        ref: "web/src/lib/reconnect.test.ts（node --test 3/3；全量 lib 套件 19/19 零回归）"
        status: pass
    human_judgment: false
  - id: D2
    description: "重连状态机落码：四函数 + 1006 显式分派（case 1006→startReconnect）+ 重连上下文分派（再 1006→scheduleAttempt 留循环；带码关闭→stopReconnect 落专版面板）+ 面板保护（fetch throw/onerror-!opened 不覆盖 Reconnecting）+ 代际守卫四 handler + online/offline 双触发 + WELCOME 成功点（stopReconnect+term.clear()）"
    requirement: CORE-05
    verification:
      - kind: other
        ref: "time pnpm -C web build 退出 0（tsc 严格+vite+gzip，2.3s）；验收 grep 全过（ev.code===1006×1 / sock!==ws×4 / online×1 / offline×1 / Reconnect now×4 / Reconnecting×4 / term.clear()×1 / 状态机四函数×4 / action?:×1）；git diff 核读只增不改"
        status: pass
    human_judgment: false
  - id: D3
    description: "既有面板语义零漂移：13 个既有 showStatus 调用点零改动、渲染逐字节不变（第四参可选）；1000/1008/1009/1011/1013/default 分派文案逐字不动；1013 维持手动刷新；首连失败（!opened）不进重连"
    requirement: CORE-05
    verification:
      - kind: integration
        ref: "web/uat/phase04-dom.mjs 37/37 + phase05-dom.mjs 19/19（真实 dist bundle jsdom 驱动，含 Invalid share link/Disconnected 1013/Session ended 面板断言）；git diff 核读确认既有调用行逐字不变"
        status: pass
    human_judgment: false
  - id: D4
    description: "跨 phase 回归门：Go 四包 -race 全绿（本 plan 零 Go 改动，dist 嵌入二进制一并验证）；行为证据（八场景 jsdom 合成 CloseEvent/online/offline 断言）由 06-05 phase06-dom.mjs 补齐，协议层『接回同一 PTY』由 06-06 S6 实证"
    requirement: CORE-05
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./internal/proto/ ./internal/server/ ./internal/pty/ ./cmd/wesh/ 全 ok"
        status: pass
    human_judgment: false

duration: 31min
completed: 2026-08-23
status: complete
---

# Phase 06 Plan 03: CORE-05 前端重连状态机 Summary

**CORE-05 前端主链端到端落码：1006 显式触发 → Reconnecting 面板（1s×2 封顶 30s 退避 + 1Hz 倒计时 + Reconnect now 手动入口）→ connect() 完整认证链重入 → WELCOME 到达清零+清屏接回原 PTY 进程——Pitfall 5/6/7 三防线（单例门闩/代际守卫/面板保护）全部落码，既有六类面板语义经双 DOM 套件实证零漂移**

## Performance

- **Duration:** 31 min
- **Started:** 2026-08-23T06:02:42Z
- **Completed:** 2026-08-23T06:33:47Z
- **Tasks:** 2/2
- **Files modified:** 4（2 新建 + 2 修改）

## Accomplishments

- `web/src/lib/reconnect.ts`（新建 32 行）：`backoffMs`/`shouldReconnect` 纯函数，prefs.ts 同款零 DOM 形态，D-02 参数族注释引 throttle.go:12-13 同族（1s/30s）
- `web/src/lib/reconnect.test.ts`（新建）：序列数组逐项等值断言 + 谓词正反两行，node --test 直跑 .ts（.ts 扩展名相对导入纪律）
- `main.ts` 状态机四函数（页面级门闩区声明，osc52Loaded/retriedAuth 邻位）：
  - `startReconnect()`——reconnecting 幂等门闩（三触发源共用入口，Pitfall 5）
  - `scheduleAttempt()`——backoffMs(attempt) 退避 + `retryAt` 记账 + Reconnecting 面板等待期正文（attempt+1）+ setTimeout(runAttempt) + 1Hz setInterval 只写 #status-body textContent（面板不闪隐，UI-SPEC §Reconnect Panel Contract）
  - `runAttempt()`——清双 timer → attempt++ → 在途正文 → `void connect()`（完整 fetch ticket → Hello 核销链，认证不绕行 T-06-03c）
  - `stopReconnect()`——清零 + 清双 timer + `#status` 隐藏（终态分派与重连成功两调用点）
- `showStatus` 参数化（R3/OQ2）：第四可选参 `action?: { label: string; onClick: () => void }`——缺省 'Reload this page' + location.reload() 逐字现状；传参时 label 替换 + click preventDefault 后调 onClick；幂等纪律（textContent 先清后建）保持
- C-9 文案常量化（UNREACHABLE_BODY/HINT_RESTART 单写口先例）：`RECONNECTING_TITLE`/`RECONNECTING_HINT` 常量 + `reconnectingWaitBody`/`reconnectingNowBody` 模板函数（初显/1Hz 更新/在途三处同源）
- 代际守卫（Pitfall 6）：onmessage/onopen/onerror/onclose 四 handler 入口 `if (sock !== ws) return;`；onclose 的 beforeunload 移除在守卫保护下（stale socket 不拆新连接监听，R4）
- onclose 分派改造：auth_failed 重试块保持原位 → 新增重连上下文分派（再 1006 `scheduleAttempt()` 留循环；非 1006 `stopReconnect()` 终止循环落下方分派）→ `!opened` Unable to connect 保持（首连失败不进重连，D-01 触发范围）→ switch 新增 `case 1006: startReconnect(); break;`（default 前）；685-687 区注释按 State of the Art 登记改写为「1006 = 重连唯一触发码（浏览器本地合成码，永不出现于线上——RFC6455 §7.4，proto.go 关闭码纪律不变）；其余码分派语义不变」
- 面板保护（Pitfall 7）：fetch catch 重连上下文走 `scheduleAttempt(); return;`（D-11 网络不可达/服务端已退出不可区分）；onerror-!opened 重连上下文直接 return（onclose 随后分派）；fetch 四条终态专版面板分支（401+token/429/503/通用认证）统一 `if (reconnecting) stopReconnect();` 前置——404 探测直连不设（链路继续走 WS，成功终止唯一挂点 = WELCOME）
- WELCOME 重连成功点（D-05）：`if (reconnecting) { stopReconnect(); term.clear(); }` 于 welcomeDone 置位前——清零+隐藏面板 → 清屏（不保留旧 buffer，G-05-1 同源花屏风险裁决；服务端 SIGWINCH 随 attach 恒触发重绘，server.go:752 零改动）→ beforeunload 按开关重注册既有代码照常（P4 D-18）；标题保持最后 remoteTitle（P5 D-12 不主动重置）；osc52Loaded 无耦合注释登记
- online/offline 监听（模块级）：offline——`reconnecting || !welcomeDone` 幂等/初连窗口返回 + ws OPEN 健康等 onclose 权威信号，否则 startReconnect（D-04 快路径；黑洞场景退化为 TCP 超时后重连，风险接受）；online——`if (reconnecting) runAttempt()`（清等待立即试一次，不是新循环）
- `web/dist/index.html` 重建入库（499.22 kB / gzip 134.10 kB；.gz 不入库既定纪律）

## showStatus 前置影响面清点（review #2 闭合动作）

执行 `grep -n 'showStatus(' web/src/main.ts`，改造前 13 个既有调用点全部确认三参形态、第四参可选下零改动：

| 行号（改造前） | 调用点 | 面板 |
|---|---|---|
| 442 | fetch 401+token | Invalid share link（C-3） |
| 449 | fetch 429 | Too many attempts |
| 457 | fetch 503 | Server is full（C-2） |
| 467 | fetch 通用认证失败 | Authentication failed（C-8） |
| 475 | fetch catch | Unable to connect（C-4） |
| 693 | onerror !opened | Unable to connect（C-4） |
| 715 | onclose !opened | Unable to connect（C-4） |
| 720 | onclose 1000 | Session ended（C-7，R2 正文） |
| 729 | onclose 1008 | Connection refused |
| 736 | onclose 1009 | Message too large |
| 743 | onclose 1011 | Server error |
| 753 | onclose 1013 | Disconnected（C-1） |
| 761 | onclose default | Connection lost（C-5） |

git diff 核读：全部 13 个调用行逐字不变（仅四 fetch 终态分支前置 stopReconnect 行新增）；缺省渲染路径（'Reload this page' + location.reload() + 尾部 '.'）逐字节不变，由 phase04-dom 37/37 与 phase05-dom 19/19（含各专版面板三件套断言）实证。新增第 14/15 调用点 = scheduleAttempt/runAttempt 的 Reconnecting 面板（传第四参）。

## Task Commits

1. **Task 1 (tdd): reconnect.ts 纯函数 + 回归锁** — `d83a3de` (feat)——RED（测试文件先行，模块不存在即红实证）→ GREEN（3/3 转绿，lib 全量 19/19）；单提交含两件（plan action 3「一次提交含两件」字面收口，06-01 同款纪律）
2. **Task 2: main.ts 状态机 + dist 重建** — `615cf23` (feat)

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS）

## Files Created/Modified

- `web/src/lib/reconnect.ts`（新建）— backoffMs + shouldReconnect
- `web/src/lib/reconnect.test.ts`（新建）— 序列与谓词回归锁
- `web/src/main.ts` — 状态机四函数 + 页面级状态 + showStatus 参数化 + C-9 常量 + 代际守卫×4 + 重连分派 + case 1006 + WELCOME 成功点 + online/offline 监听 + 注释改写（+176/-17）
- `web/dist/index.html` — 重建产物

## Verification Evidence

- `node --test web/src/lib/reconnect.test.ts` 3/3；`node --test web/src/lib/*.test.ts` 19/19（prefs 8 + title 8 + reconnect 3 零回归）
- `time pnpm -C web build` 退出 0（tsc 严格模式 + vite + gzip，2.3s）
- 验收 grep 全过：`ev.code === 1006`==1、`sock !== ws`==4（四 handler）、`addEventListener('online'`==1、`addEventListener('offline'`==1、`Reconnect now`==4、`Reconnecting`==4、`term.clear()`==1（唯一调用点）、状态机四函数==4、`action?:`==1
- git diff 核读：既有面板分派（1000/1008/1009/1011/1013/default）与各 showStatus 调用行逐字不动，只增不改（R1：default 桶残留语义不变——1002 等带码关闭仍落 C-5）
- 既有 DOM 套件实证零漂移：phase04-dom.mjs 37/37、phase05-dom.mjs 19/19（真实新 dist bundle 驱动）
- 回归门：`go test -race -count=1` proto/server/pty/cmd 四包全 ok
- 行为证据分工（plan 既定）：八场景 jsdom 状态机断言（合成 CloseEvent{1006}/online/offline、代际守卫 D6 场景）由 06-05 phase06-dom.mjs 锁定；协议层断线重连接回原 PTY 由 06-06 phase06.mjs S6 实证

## Decisions Made

- **fetch catch 补 welcomeDone 代际标记守卫**（Rule 2，见 Deviations #1）——Pitfall 6 在 fetch 通道的同族收口
- **scheduleAttempt 入口清双 timer**（Rule 2，见 Deviations #2）——定时器恰好一次的机械保证
- **404 探测直连分支不设 stopReconnect**——UI-SPEC 终态表的「404 探测直连」行机械调和：无认证模式重连链路继续走 WS，循环终止唯一挂点 = WELCOME 到达（「重连成功判定恒为 WELCOME」prohibition 的直接推论）；401/429/503/通用认证四条真终态分支按 plan 逐字设 stopReconnect
- **TDD 提交纪律按 plan 字面收口**：RED→GREEN 顺序实证，单提交含测试+实现（plan action 3 明示，06-01 先例第二次沿用）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] fetch catch 补 welcomeDone 代际守卫**
- **Found during:** Task 2 面板保护落码（Pitfall 7 catch 分支改造时推演 D-04 既定形态的全时序）
- **Issue:** plan 规格 `if (reconnecting) { scheduleAttempt(); return; }` 未覆盖『双在飞 attempt 较慢者迟到失败』窗口——online 事件/「Reconnect now」点击可在前一 attempt 的 fetch 飞行中再启 attempt（D-04 明文既定形态）；若后者成功（WELCOME → stopReconnect → welcomeDone=true），前者的 fetch 迟到失败（黑洞场景 TCP 超时可迟至分钟级）走到 catch 时 reconnecting 已为 false，plan 字面将落既有 `showStatus('Unable to connect')`——用失败面板覆盖健康会话（Pitfall 6 同族代际污染，但 fetch 通道无 sock 可守卫，四 handler 代际守卫不覆盖此通道）
- **Fix:** catch 分支在 reconnecting 检查之后补 `if (welcomeDone) return;`——welcomeDone 即「新会话已建立」代际标记（connect() 入口重置块清零、WELCOME 置位，时序恰好区分三场景：初连失败=false 落面板 / 循环中=true 走 scheduleAttempt / 陈旧迟到=true 静默丢弃）
- **Files modified:** web/src/main.ts
- **Verification:** tsc 过；既有 DOM 套件（含 fetch 失败/专版面板场景）37/37 + 19/19 零回归；06-05 D6 代际场景将补 jsdom 层断言
- **Committed in:** `615cf23`（Task 2 提交内）

**2. [Rule 2 - Missing critical] scheduleAttempt 入口清双 timer**
- **Found during:** Task 2 状态机落码（Pitfall 5 恰好一次纪律的机械化推演）
- **Issue:** plan 规格 scheduleAttempt 直接赋值 `reconnectTimer = setTimeout(...)`——双在飞 attempt 先后失败各入 scheduleAttempt 时，第二次赋值覆盖变量但第一次的定时器仍在飞（id 丢失无法清除），定时器恰好一次不变量在机械层失守（双 timer → runAttempt 双触发 → attempt 跳变，Pitfall 5 Warning sign 逐字形态）
- **Fix:** scheduleAttempt 入口 `clearTimeout(reconnectTimer); clearInterval(countdownTimer);`——全部入路径（startReconnect 首入/失败重入/onclose 再 1006）都归一到『至多一个在飞等待定时器』
- **Files modified:** web/src/main.ts
- **Verification:** tsc 过；倒计时 1Hz 更新与 Reconnect now 点击路径同属 runAttempt/scheduleAttempt 双清形态，无叠加面
- **Committed in:** `615cf23`（Task 2 提交内）

---

**Total deviations:** 2 auto-fixed（均 Rule 2，Pitfall 5/6 规格在 plan 未枚举通道/窗口的机械化补全；契约文本零改动）
**Impact on plan:** 无范围蔓延——两处皆为 plan 自身 D-04/Pitfall 条款既定形态的正确性必要件；既有套件适配明细：**零适配**。

## Threat Flags

None——T-06-03a（流量放大：仅 1006 触发 + 30s 封顶 + 1013 不重连 + 终态终止，全落码）/ T-06-03b（面板钓鱼：C-9 全前端硬编码常量 + textContent 渲染）/ T-06-03c（认证绕行：结构性不存在，attempt 走完整 connect() 链）/ T-06-03d（代际污染：四 handler 守卫 + fetch 通道 welcomeDone 标记，已落地且比 plan 登记多收口一条 fetch 通道）均在 plan threat_model 登记内，无新增未登记面。

## Known Stubs

None——全链 wired：重连循环驱动真实 connect() 认证链，面板计数/倒计时由真实定时器驱动，清屏接真实 WELCOME 路径，零占位零 mock。

## Issues Encountered

None——plan 的 read_first 与 RESEARCH/PATTERNS/UI-SPEC 逐字契约完备（含行号级挂点），实现零探索弯路；仅有的两处规格外补全已按 Rule 2 登记。

## Next Phase Readiness

- 06-05（phase06-dom.mjs）：八场景断言所依赖的全部源码挂点已锁定——`startReconnect`/`scheduleAttempt`/`runAttempt`/`stopReconnect` 四函数、case 1006 分派、`sock !== ws` 守卫×4、online/offline 监听、Reconnecting 面板三件套（title='Reconnecting'/等待期与在途 body 模板/'Reconnect now' 链接）、WELCOME 成功点 term.clear()；代际守卫 D6 场景（旧 socket close 不得隐藏新会话面板）与 welcomeDone fetch 守卫同族可一并断言
- 06-06（phase06.mjs S6）：协议层『杀 WS → 自动重连接回原 PTY』的前端半侧已就绪——真实 dist bundle 已含状态机
- 关注点：无阻塞；backoffMs/shouldReconnect 已由 node --test 锁定，06-05 断言与主分派共享同一事实源

## Self-Check: PASSED

- 全部 4 个产物文件（reconnect.ts / reconnect.test.ts / main.ts / dist/index.html）落盘核实（FOUND ×4）
- 任务提交 d83a3de / 615cf23 在 git log 核实（FOUND ×2）；两提交 post-commit 删除检查均无文件删除、无遗留 untracked

---
*Phase: 06-session-lifecycle*
*Completed: 2026-08-23*
