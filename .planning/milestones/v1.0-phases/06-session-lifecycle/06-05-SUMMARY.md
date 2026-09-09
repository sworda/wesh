---
phase: 06-session-lifecycle
plan: 05
subsystem: testing
tags: [uat, jsdom, websocket, reconnect, xterm, typescript]

requires:
  - phase: 06-session-lifecycle
    provides: 06-03 重连状态机（startReconnect/scheduleAttempt/runAttempt/stopReconnect 四函数 + case 1006 分派 + 代际守卫×4 + online/offline 监听 + C-9 面板三件套 + WELCOME 成功点 term.clear()）与 06-01 EXIT 帧前端承接（lastExit → onclose 1000 正文）
provides:
  - web/uat/phase06-dom.mjs：CORE-05 重连状态机 jsdom 行为断言八场景（D1-D8）+ D9 真实断网栈豁免记录 + SEC 输出自净断言（30/30 PASS + 1 skipped）
  - SpyWebSocket synthClose(code) 合成 CloseEvent 能力（RESEARCH A2 兑现）+ 模块级构造计数 + beforeunload add/remove 记账包装
  - assertOutputClean() 运行时红线自证形态（review #7——token/链接形态串零进输出的可执行断言）
affects: [06-06, 06-07, phase-07]

actuals:
  tokens: 8400
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "合成 CloseEvent 驱动真实 bundle onclose 分派：synthClose 取存处理器并置 null（抑制随后真实 close 的 1000 混入断言面）→ try{close()}catch{} → 处理器同步调用合成事件；副本留存供代际场景二次驱动（staleClose）"
    - "重连行为断言三件套：模块级 SpyWebSocket 构造计数（基线快照取相对值）+ beforeunload 记账包装（转发原实现零漂移）+ 终端 DOM 文本 waitFor（term.clear() 可观测通道）"
    - "时序敏感断言的轮询+容差窗形态：立即 attempt 断言取 800ms 容差窗 ≪ 标称退避 1s；不触发边界取 2.5s 守候窗 > attempt 1 退避点 + 1.5s 抖动容差（review #6 吸收，精确时点断言零残留）"

key-files:
  created:
    - web/uat/phase06-dom.mjs
  modified: []

key-decisions:
  - "D1 清屏对照文本改 typeText echo 链路（Rule 3）——plan『spawn printf D1BANNER 先行』形态在 D-12 drain 语义下结构性不可观测（attach 前输出被丢弃无回放，phase05 D5c/05-09 登记同源语义）；typeText InputEvent 链是 phase04-dom 已验证先例，且恰为 must_have『终端经 echo 写入可观测文本』的字面形态"
  - "RESEARCH A2 兑现——jsdom 25 CloseEvent 构造器探针先证可用（Event+code 回退形态同码登记未触发）；synthClose 置 null 抑制真实 close 混入断言面"
  - "D7 会话建立可观测代理取 beforeunload 注册（bu.on==1）——sh -c 'sleep 2; exit 7' 无输出进程 waitReady（终端文本）不适用，WELCOME 处理完成注册点恰好是 waitFor 友好标记"

patterns-established:
  - "UAT 输出红线运行时自证：check/skip 包装收集已发 detail 入数组 + startWesh token 留闭包 + 汇总前 assertOutputClean() 遍历断言（注释纪律 → 可执行断言，phase04.mjs:6-9 红线的 regression guard 形态）"
  - "代际守卫直击断言形态：stale 实例引用留存 → 重连成功 → 对旧实例二次派发合成 onclose → 面板/构造计数/beforeunload 记账三零污染断言"

requirements-completed: [CORE-05]

coverage:
  - id: D1
    description: "1006 重连全链：Reconnecting 面板 C-9 逐字要点（title/body attempt 1/hint 双要点）→ beforeunload 移除 → 退避自动重连（构造 +1）→ 新 WELCOME 面板隐藏 → term.clear() 可观测（echo 标记消失）→ beforeunload 重注册"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D1a-D1i 9/9 pass）"
        status: pass
    human_judgment: false
  - id: D2
    description: "1002 不触发边界：C-5 Connection lost 手动面板 + 2.5s 守候窗零新连接（default 桶残留语义）"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D2a-D2b pass）"
        status: pass
    human_judgment: false
  - id: D3
    description: "1013/1008 不触发边界：Disconnected（P5 D-10 手动刷新保持）/ Connection refused（1000/1008/1009/1011 集合抽样）专版 + 守候窗零构造"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D3a-D3d pass）"
        status: pass
    human_judgment: false
  - id: D4
    description: "双触发幂等（Pitfall 5）：offline + onclose(1006) 相继到达 → 单循环（attempt 不跳变、构造单调 +1 不并发翻倍）"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D4a-D4c pass）"
        status: pass
    human_judgment: false
  - id: D5
    description: "Reconnect now 手动入口：等待期点击 #status-hint a → 800ms 容差窗内构造 +1（倒计时未完即 attempt）→ 循环以成功终止"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D5a-D5c pass）"
        status: pass
    human_judgment: false
  - id: D6
    description: "代际守卫（Pitfall 6）：重连成功后对旧 socket 迟到派发合成 onclose(1006) → 面板隐藏保持 + 无第三连接 + beforeunload 记账不增"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D6a-D6c pass）"
        status: pass
    human_judgment: false
  - id: D7
    description: "EXIT 帧全链（真实服务端行为）：sh -c 'sleep 2; exit 7' → EXIT+1000 → Session ended 正文逐字 'The process exited with code 7.' + wesh 进程 exit 码 7 + 1000 不触发重连"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D7a-D7c pass）"
        status: pass
    human_judgment: false
  - id: D8
    description: "online 快路径（D-04）：重连等待期 dispatch online → 800ms 容差窗内构造 +1（清等待立即试一次）→ 重连成功面板隐藏"
    requirement: CORE-05
    verification:
      - kind: automated_ui
        ref: "node web/uat/phase06-dom.mjs（D8a-D8b pass）"
        status: pass
    human_judgment: false
  - id: D9
    description: "真实 OS 断网栈与浏览器原生 online/offline 事件时序（backstop 行）"
    requirement: CORE-05
    verification: []
    human_judgment: true
    rationale: "headless 硬约束（CODEBUDDY.md 平台原生行为豁免条款）——任何自动化（含 playwright）结构性不可测；等价逻辑面由本脚本合成事件驱动覆盖（D1-D8）、协议面由 06-06 phase06.mjs 真实 TCP 断连覆盖；真实栈场景以 skipped+reason 记录并指向 06-UAT.md（06-07 产出）人工清单"

duration: 21min
completed: 2026-08-23
status: complete
---

# Phase 06 Plan 05: CORE-05 重连状态机 jsdom 行为证据（phase06-dom.mjs）Summary

**CORE-05 前端重连逻辑面最终自动化证据落盘：SpyWebSocket synthClose 合成 CloseEvent（A2 兑现）+ 构造计数 + beforeunload 记账三夹具，jsdom 驱动真实 dist bundle 连真实 wesh 实例，八场景 30/30 全绿——1006 全链（面板/退避/清屏/重注册）、1002/1013/1008 不触发边界、双触发幂等、Reconnect now 手动入口、stale 代际守卫、EXIT 帧端到端逐字文案、online 快路径，真实断网栈以 skipped 豁免登记**

## Performance

- **Duration:** 21 min
- **Started:** 2026-08-23T07:21:22Z
- **Completed:** 2026-08-23T07:42:09Z
- **Tasks:** 2/2
- **Files modified:** 1（新建 web/uat/phase06-dom.mjs，571 行）

## Accomplishments

- `web/uat/phase06-dom.mjs`（新建）：startWesh 复用 phase05.mjs 形态（--port 0 + stdout 解析 + stderr 捕获 + SIGKILL 收口 + token 留闭包）；loadTerminal 逐字复用 phase05-dom 注入形态并延伸三件——SpyWebSocket `synthClose(code)`（合成 CloseEvent，jsdom 25 构造器探针先证可用，Event+code 回退同码登记）+ 模块级 `constructed` 构造计数 + window addEventListener/removeEventListener 'beforeunload' 记账包装（on/off 计数，转发原实现零漂移）
- D1 重连全链（9 断言）：echo 标记串写入终端 DOM → synthClose(1006) → Reconnecting 面板 C-9 逐字要点（title=='Reconnecting'、body 含 'attempt 1'、hint 含 'Reconnect now' 与 'restart it from your shell'）→ beforeunload 移除（off+1）→ 退避自动重连（构造 +1，3s 容差窗）→ 新 WELCOME 面板隐藏 → D1BANNER 从终端 DOM 消失（term.clear() 可观测证据）→ beforeunload 重注册（on+1）
- D2/D3 不触发边界（6 断言）：1002 → C-5 'Connection lost'、1013 → 'Disconnected'（P5 D-10 保持）、1008 → 'Connection refused'（专版集合抽样），各配 2.5s 守候窗零新连接构造（> attempt 1 退避标称 1s + 1.5s 抖动容差，review #6 论证注释位）
- D4 双触发幂等（3 断言，Pitfall 5）：synthClose(1006) 紧随 dispatchEvent(offline) → 单循环（attempt 1 不跳变、构造单调 +1、3s 窗不并发翻倍）
- D5 手动入口（3 断言）：等待期点击 `#status-hint a`（'Reconnect now'）→ 800ms 容差窗内构造 +1（≪ 标称退避 1s——倒计时未完即 attempt）→ 循环以成功终止
- D6 代际守卫（3 断言，Pitfall 6）：staleClose 对旧实例二次派发合成 onclose(1006) → 面板隐藏保持 + 无第三连接 + beforeunload 记账不增（stale 不拆新连接监听）
- D7 EXIT 全链（3 断言，真实服务端行为）：spawn `sh -c 'sleep 2; exit 7'` → 真实 EXIT 帧 + 1000 → Session ended 正文逐字 == 'The process exited with code 7.'（06-01 服务端组文案端到端直显）+ wesh 进程 exit 码==7 + 1000 零新连接
- D8 online 快路径（2 断言，D-04）：等待期 dispatchEvent(online) → 800ms 容差窗内构造 +1（清等待立即试一次）→ 重连成功
- D9 豁免登记：真实 OS 断网栈/浏览器原生事件时序 skipped+reason（headless 硬约束，指向 06-UAT.md 人工清单）——backstop 行 UAT 落点
- SEC 输出自净（review #7）：check/skip 收集已发 detail + startWesh token 留闭包 + 汇总前 assertOutputClean() 遍历断言零 token 值零 '/s/' 链接形态串——红线由注释纪律升级为运行时自证

## Task Commits

1. **Task 1: 骨架（loadTerminal + synthClose + 记账）+ D1-D4** — `24a2ac2` (feat)
2. **Task 2: D5-D8 + 豁免场景 + assertOutputClean 汇总收口** — `6d35cb7` (feat)

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS）

## Files Created/Modified

- `web/uat/phase06-dom.mjs`（新建）— 八场景 + D9 豁免 + SEC 自净断言，30/30 PASS + 1 skipped

## Verification Evidence

- `go build -o /tmp/wesh-uat/wesh ./cmd/wesh` 退出 0（0.5s）
- `node web/uat/phase06-dom.mjs` 退出 0：**30/30 DOM 断言通过 + 1 skipped（豁免）**（D1a-D1i / D2a-b / D3a-d / D4a-c / D5a-c / D6a-c / D7a-c / D8a-b / SEC 全 PASS，D9 SKIP）
- Task 1 验收 grep：`synthClose`=13≥2、`new window.CloseEvent('close'`=1≥1、`'beforeunload'`=3≥2
- Task 2 验收 grep：`skip(`=1≥1、`assertOutputClean`=7≥2
- 回归门（本 plan 零生产代码改动，跨 phase 佐证）：`go test -race -count=1` proto/server/pty/cmd 四包全 ok；`node --test web/src/lib/*.test.ts` 0 fail；phase04-dom 37/37、phase05-dom 19/19 零漂移
- A2 探针先证：jsdom 25 `new window.CloseEvent('close', {code:1006})` 构造可用、addEventListener 包装转发可行（回退形态未触发）

## Decisions Made

- **D1 清屏对照文本改 typeText echo 链路**（Rule 3，见 Deviations #1）——plan printf 先行形态结构性不可观测
- **D7 会话建立代理取 beforeunload 注册**——无输出进程 waitReady 不适用；WELCOME 处理完成注册点（bu.on==1）是 waitFor 友好的精确标记
- **D6 stale 驱动取 staleClose 副本直调形态**（review D6 简化建议同款）——synthClose 置 null 后二次调用空转，代际事件须以 `_savedClose` 捕获副本驱动

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] D1 清屏对照文本机制由 spawn printf 改 typeText echo**
- **Found during:** Task 1（D1 首跑 waitFor 超时：D1BANNER 永不出现）
- **Issue:** plan 指定 spawn `sh -c 'printf D1BANNER; exec bash …'`——banner 在进程启动即打印，而 jsdom 页面 ~0.5s 后才 attach；服务端 D-12 drain 语义（无 ring 回放，phase05 D5c/05-09 S6 登记同源）使 attach 前输出被丢弃，banner 结构性不可观测。plan 行动项同时误判「jsdom 侧无法直接驱动键盘」——phase04-dom.mjs typeText（InputEvent 链 → onData → INPUT 帧）恰是已验证的键盘驱动先例（phase05-dom D2c 沿用）
- **Fix:** spawn 改回 `--writable -- bash --norc --noprofile`，会话建立后 `typeText(ctx.window, 'echo D1BANNER\n')` 经真实键盘链路写入标记串（must_have「终端经 echo 写入可观测文本」的字面形态），waitFor 在场后再驱动断连——「消失」断言因前置存在性而有意义的 review 吸收形态保持
- **Files modified:** web/uat/phase06-dom.mjs
- **Verification:** D1a-D1i 9/9 PASS（含 D1c 标记在场基线 + D1h 重连后消失）；断言面与 plan 验收逐字一致（面板文案要点/构造 +1/面板隐藏/清屏消失/beforeunload 记账五项全在）
- **Committed in:** `24a2ac2`（Task 1 提交内）

---

**Total deviations:** 1 auto-fixed（Rule 3——plan 机制在既有 drain 语义下不可达，同意图等价机制替换；断言面零削弱）
**Impact on plan:** 无范围蔓延；八场景断言面与 must_haves 逐字对应，既有套件零适配。

## Threat Flags

None——T-06-05a（token 进输出：红线注释 + assertOutputClean 运行时自证双保险，已落地且超出 plan 登记强度的部分为零）、T-06-05b（测试挂死：waitFor 全带上限 + 2.5s/3s 守候窗显式值 + 全实例 SIGKILL 收口，已落地）、T-06-SC（零新依赖：jsdom 25.0.1 既有锁定）均在 plan threat_model 登记内，无新增未登记面。

## Known Stubs

None——八场景全链 wired：真实 dist bundle + 真实 spawn 实例 + 真实 WS/EXIT 帧，零占位零 mock。D9 为显式豁免（skipped+reason），非 stub；已登记 .planning/WINDOWS.md（kind=unrun-verify，指向 06-UAT.md 人工清单）。

## Issues Encountered

仅 Deviations #1 一单（printf banner 不可观测——首跑即暴露，探针/先例驱动的一次性修正，无反复）。

## Next Phase Readiness

- 06-06（phase06.mjs 协议层）：本脚本锁定的前端行为面（1006 触发/退避/手动入口/代际守卫）与协议面『杀 WS → 重连接回原 PTY』分工互补；D7 已顺带锁定进程级退出码传递（exit 7），S1/S2 断言面可直接引用
- 06-07（README/06-UAT.md）：D9 豁免场景的人工清单指针已落（skipped reason 逐字指向 06-UAT.md）；重连语义文档化（D-07「重连目标 = 同一 URL 当前进程」）所需行为事实全部由本脚本证据化
- 关注点：无阻塞；assertOutputClean 形态可被后续 UAT 脚本复用（红线 regression guard）

## Self-Check: PASSED

- 产物文件 web/uat/phase06-dom.mjs + 本 SUMMARY 落盘核实（FOUND ×2）
- 任务提交 24a2ac2 / 6d35cb7 在 git log 核实（FOUND ×2）；两提交 post-commit 删除检查均无文件删除（第二提交 -5 行为 Task 1 骨架的就地演进改写，非文件删除）、无遗留 untracked
