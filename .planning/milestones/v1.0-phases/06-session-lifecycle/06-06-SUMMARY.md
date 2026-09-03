---
phase: 06-session-lifecycle
plan: 06
subsystem: testing
tags: [uat, websocket, protocol, lifecycle, reconnect, node, headless]

requires:
  - phase: 06-session-lifecycle/06-01
    provides: EXIT 帧契约（'X'=0x58/ExitPayload/ExitFrame）+ lifecycle 写序安全广播 + exitMessage 三形态文案唯一写口（S1/S2 断言面来源）
  - phase: 06-session-lifecycle/06-02
    provides: SignalHangup + Options.ExitWhenEmpty + 空触发/宽限计时器 + exiting 门 + OQ1 门裁决 accept-255（S3/S4/S5 进程退出状态断言常量）
  - phase: 06-session-lifecycle/06-04
    provides: --once 语法糖（≡ --max-clients=1 --exit-when-empty=0）+ --exit-when-empty[=duration] IsBoolFlag 三形态（S3/S4/S5 驱动的 CLI 面）
provides:
  - web/uat/phase06.mjs：phase 6 协议层 UAT 七场景（EXIT 双端广播/信号死亡/--once 全链/--exit-when-empty 立即与宽限/断连重接同一 PTY）+ S7 豁免记录（23/23 PASS + 1 skipped）
  - waitExit/exitOf/collectUntilClose 三 helper（进程退出与帧序断言通道）+ EXIT=0x58 帧常量对齐位
  - assertOutputClean() 运行时红线自证在协议层脚本的第二个落地（review #7 形态复用）
affects: [06-07, phase-07]

actuals:
  tokens: 11700
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "进程退出断言通道：waitExit(child, timeoutMs) 恒带护栏超时——child 'exit' 事件决议 {code,signal}，挂死=护栏到期 FAIL 而非无限等待；监听先挂于触发动作前防事件丢失"
    - "EXIT 帧序断言形态：collectUntilClose(ws) 换装 onmessage/onclose 收集握手后增量帧——末帧 [0]===EXIT 且 close.code===1000 即『EXIT 先于 1000』的协议层证据"
    - "时序容差规格（review #6）：取消窗 400ms ≪ 标称宽限 1500ms（1100ms 调度余量）；到期断言 5s 护栏轮询而非精确时点；时钟不 mock、等待不缩短"

key-files:
  created:
    - web/uat/phase06.mjs
  modified: []

key-decisions:
  - "S5① 宽限计时起点锚定服务端 detach：c1 close 后先 waitClose（close 握手完成 ⇒ reader 已收关闭帧触发 detach）再 sleep 400ms——取消窗相对真实计时起点起算，1100ms 余量论证成立"
  - "sensitiveTokens 收集自全部 startWesh 实例（plan 字面『S1/S3 两条』的严格超集）——assertOutputClean 断言面不随场景增删漂移"
  - "S2a 补负向断言 !message.includes('hangup')——plan must_have 括号内点名的小写描述词回归形态转可执行断言（Pitfall 3 双向锁）"
  - "S6 pid 解析 helper 统一 readPid(frames,ws)：/S6PID=(\\d+)/ 数字锚定只命中结果行（终端回显含命令原文 'echo S6PID=$$' 无数字不命中），pidPre/pidPost 两调用点同形态"

patterns-established:
  - "单次语义场景的 UAT 纪律注释：--once/--exit-when-empty 下服务端进程退出是特性不是回归——child 'exit' 事件为断言通道，SIGKILL 仅作未退出场景清理"
  - "断连重接同一 PTY 的协议层证据形态：echo S6PID=$$ 进程 ID 相等为主证据（新 bash 必持不同 pid）+ shell 变量跨断连存活为次级佐证 + 首连接全程无 EXIT 帧"

requirements-completed: [SESS-01, SESS-02, SESS-03, CORE-05]

coverage:
  - id: D1
    description: "S1 EXIT 双端广播（SESS-03 协议层终证）：--writable + --credential 实例，A 经 Basic ticket（rw）+ B 经 shareRO token（ro）双 attach → A 写 'exit 42' → 双端同收 EXIT{exit_code:42, message 逐字 'The process exited with code 42.'} 且帧体逐字节一致（ro/rw 全员同帧）→ 帧序 EXIT 先于 close 1000 → wesh 进程 exit 码 42"
    requirement: SESS-03
    verification:
      - kind: integration
        ref: "node web/uat/phase06.mjs（S1a-S1e 5/5 pass，含复跑）"
        status: pass
    human_judgment: false
  - id: D2
    description: "S2 EXIT 信号死亡：sh -c 'sleep 1; kill -HUP $$' → EXIT{exit_code:-1} 且 message 含大写 'SIGHUP'（'killed by signal hangup' 小写描述词回归负向断言同锁）→ close 1000"
    requirement: SESS-03
    verification:
      - kind: integration
        ref: "node web/uat/phase06.mjs（S2a-S2b pass）"
        status: pass
    human_judgment: false
  - id: D3
    description: "S3 --once 全链（SESS-01 协议层终证）：首客户端 Basic ticket attach → 第二客户端双点位 503（POST /api/attach 早闸 + WS 直连升级 ③位——503=既有 max-clients 计数路径，409 不复活）→ 唯一客户端断开后 wesh 进程退出状态==255（OQ1 accept-255 门裁决值兑现）+ stderr 无 panic"
    requirement: SESS-01
    verification:
      - kind: integration
        ref: "node web/uat/phase06.mjs（S3a-S3e 5/5 pass）"
        status: pass
    human_judgment: false
  - id: D4
    description: "S4 --exit-when-empty 立即形态（SESS-02）：裸 flag（IsBoolFlag 形态实证）→ attach 前 400ms 守候窗进程无 exit 事件（启动期注册表恒空不触发，Pitfall 2 协议层显式证据）→ attach → close → 进程退出状态 255"
    requirement: SESS-02
    verification:
      - kind: integration
        ref: "node web/uat/phase06.mjs（S4a-S4b pass）"
        status: pass
    human_judgment: false
  - id: D5
    description: "S5 --exit-when-empty=1500ms 宽限两子场景（时序容差规格：取消窗 400ms ≪ 标称 1500ms 留 1100ms 余量；到期 5s 护栏轮询）：① attach → close → 400ms 后再 attach 成功（宽限取消）+ INPUT 唯一标记回读 OUTPUT 含标记（会话存活）→ 再断开到期退出 255；② 独立实例 attach → close → 无人归到期退出 255"
    requirement: SESS-02
    verification:
      - kind: integration
        ref: "node web/uat/phase06.mjs（S5a-S5d 4/4 pass，含复跑稳定）"
        status: pass
    human_judgment: false
  - id: D6
    description: "S6 断连重接同一 PTY（CORE-05 协议层等价物）：--writable 无认证实例 attach → 写 X=weshmark42 回读落账 + echo S6PID=$$ 解析 pidPre → ws.close() → 500ms 后新 attach 成功（服务端存活=默认不开启退出顺带锁定，D-14）→ pidPost==pidPre（进程 ID 相等=同一进程强证据）+ echo $X 含 weshmark42（变量跨断连存活佐证）+ 首连接全程无 EXIT 帧"
    requirement: CORE-05
    verification:
      - kind: integration
        ref: "node web/uat/phase06.mjs（S6a-S6d 4/4 pass）"
        status: pass
    human_judgment: false
  - id: D7
    description: "S7 真实断网栈/浏览器原生断网恢复事件序列与 tmux/herdr 屏幕重绘观感（backstop 行）"
    requirement: CORE-05
    verification: []
    human_judgment: true
    rationale: "headless 硬约束（CODEBUDDY.md 平台原生行为豁免条款）——真实断网栈任何自动化（含 playwright）结构性不可测；协议层等价物已由 S6 覆盖（真实 TCP 断连重接同一 PTY），DOM 逻辑面由 phase06-dom.mjs D1-D8 合成事件覆盖；以 skipped+reason 记录并指向 .planning/phases/06-session-lifecycle/06-UAT.md 人工清单（06-07 产出）"
  - id: D8
    description: "SEC 输出自净断言（review #7）：check/skip 收集全部已发 detail，汇总前 assertOutputClean() 遍历断言零 UAT_CREDENTIAL 值、零 share token 值、零 '/s/' 链接形态串——phase04.mjs:6-9 红线的运行时自证形态"
    requirement: SESS-03
    verification:
      - kind: other
        ref: "node web/uat/phase06.mjs（SEC pass：details=23 命中=false）"
        status: pass
    human_judgment: false

duration: 23min
completed: 2026-08-23
status: complete
---

# Phase 06 Plan 06: phase06.mjs 协议层 UAT 七场景（SESS-01/02/03 + CORE-05 终证）Summary

**phase 6 服务端行为最终端到端证据落盘：EXIT 双端同帧逐字节一致（exit 42 逐字文案 + 帧序先于 1000 + 进程退出码 42）、SIGHUP 信号死亡大写名形态、--once 双点位 503 + 断开退出 255（OQ1 accept-255 兑现）、--exit-when-empty 立即/宽限取消/宽限到期三形态、断连重接同一 PTY（pidPre==pidPost 进程 ID 主证据）——真实二进制七场景 23/23 PASS + S7 headless 豁免登记**

## Performance

- **Duration:** 23 min
- **Started:** 2026-08-23T07:55:07Z
- **Completed:** 2026-08-23T08:18:07Z
- **Tasks:** 2/2（Task 1 骨架+S1/S2；Task 2 S3-S7+汇总收口）
- **Files modified:** 1（新建 web/uat/phase06.mjs，496 行）

## Accomplishments

- `web/uat/phase06.mjs`（新建，零依赖 Node 原生 WebSocket/fetch）：phase05.mjs 五件套逐字复用（startWesh/dialHello/waitClose/check-skip/场景驱动骨架）+ 帧常量区 `EXIT = 0x58` 对齐位 + 三新 helper——`waitExit`（child 'exit' 事件恒带护栏超时）、`exitOf`（RESEARCH 逐字形态）、`collectUntilClose`（帧序断言通道）
- S1 EXIT 双端广播（5 断言）：A=Basic ticket（rw）+ B=shareRO token（ro）→ 'exit 42' → 双端 exit_code==42 且 message 逐字 'The process exited with code 42.'（06-01 服务端唯一写口）+ 帧体逐字节一致（62 字节，终结无权限语义）+ 末帧 EXIT 且 close 1000（帧序）+ wesh 进程 exit 码==42（退出码传递进程级锁定）
- S2 信号死亡（2 断言）：sh 自杀 SIGHUP → exit_code==-1 + message 含大写 'SIGHUP' 且负向锁定无 'hangup' 小写描述词回归（Pitfall 3）+ close 1000
- S3 --once 全链（5 断言）：单客户端占槽 → 第二客户端双点位 503（/api/attach 早闸 + WS ③位——既有 max-clients 计数路径，409 不复活，D-12）→ 断开 → 进程退出状态==255（**OQ1 accept-255 门裁决值兑现**：SIGHUP 收口 ExitCode=-1 → os.Exit(-1) Unix 截断）+ stderr 无 panic
- S4 立即形态（2 断言）：裸 flag `--exit-when-empty`（IsBoolFlag 实证——不消费下一参数 '--'）+ attach 前 400ms 守候窗无 exit 事件（启动期恒空不触发，Pitfall 2 协议层显式证据）+ close 后退出 255
- S5 宽限两子场景（4 断言，时序容差规格）：① close 后 400ms（≪ 标称 1500ms，1100ms 余量）再 attach 成功（宽限取消）+ echo 唯一标记回读存活 + 再断开 5s 护栏内到期退出 255；② 独立实例无人归 5s 护栏内到期退出 255
- S6 断连重接同一 PTY（4 断言）：pidPost==pidPre（echo S6PID=$$ 进程 ID 相等=同一进程强证据，review #4 吸收）+ weshmark42 变量跨断连存活（次级佐证）+ 首连接全程无 EXIT 帧 + 断开期间服务端存活（默认不开启顺带锁定，D-14 零漂移）
- S7 豁免登记：真实断网栈/浏览器原生事件序列 skipped+reason（headless 硬约束，指向 06-UAT.md 人工清单）；SEC 输出自净断言（review #7）——23 条 detail 零凭据/token/'/s/' 形态串

## Task Commits

1. **Task 1: phase06.mjs 骨架（五件套 + EXIT 常量 + waitExit）+ S1/S2** — `e6a46ab` (feat)
2. **Task 2: S3-S7 + assertOutputClean + 汇总收口** — `2940c66` (feat)

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS）

## Files Created/Modified

- `web/uat/phase06.mjs`（新建）— phase 6 协议层 UAT 七场景 + S7 豁免 + SEC 自净断言，23/23 PASS + 1 skipped

## Verification Evidence

- `go build -o /tmp/wesh-uat/wesh ./cmd/wesh` 退出 0（0.4s）
- `node web/uat/phase06.mjs` 退出 0：**23/23 协议断言通过 + 1 skipped（豁免）**——首跑与复跑两连绿（S4/S5 时序场景无 flake）
- Task 1 验收 grep：`EXIT = 0x58`==1（帧常量对齐位唯一）、`waitExit`==8≥2（helper 定义+多场景调用）
- Task 2 验收 grep：`'--once'`==1≥1、`'--exit-when-empty`==3≥2（裸写与 =1500ms 两形态）、`weshmark42`==6≥2（写入/回读/跨断连三断言点）、`S6PID`==5≥2（pidPre/pidPost 两解析点）、`assertOutputClean`==5≥2（定义+汇总前调用）、`skip(`==1≥1（S7 豁免记录）
- 红线人工核读：全部 detail 只含状态码/布尔/退出码/文案常量（message 逐字为服务端组文案常量，非敏感值）；token/凭据值仅出现在常量定义与请求构造材料位——SEC 运行时自证同锁

## 协议层证据面与 Go 测试层分工（plan output 指定记录）

**只在协议层存在的断言**（Go 层结构性不可达）：
- **进程级退出状态 255**（S3/S4/S5）——Go 测试的 exitf 捕获桩收 ExitError ExitCode 原值 -1；os.Exit(-1)→255 的 Unix 截断只在真实二进制出现。本 plan 按 OQ1 accept-255 门裁决值逐字兑现三处进程级断言（06-02 登记的下游消费点闭合）
- **真实 WS 线上的帧序**（S1d/S2b：末帧 EXIT 先于 close 1000）——Go 集成测试经进程内 client 断言同语义；协议层以 collectUntilClose 收集真实线帧，末帧 [0]===EXIT 且 close.code===1000 为写序安全（Pitfall 1）的端到端证据
- **双端帧体逐字节一致**（S1c，62 字节）——组帧一次共享只读引用的线上证据
- **双点位 503 的真实 HTTP 面**（S3b/S3c）——/api/attach 早闸与 WS ③位 Accept 前拒绝在真实监听器上的全链（phase05.mjs S5 先例形态在 --once 语法糖展开路径的复证）
- **启动期恒空不触发的守候窗证据**（S4a 400ms 无 exit 事件）——Pitfall 2 在真实进程生命周期的显式证据（Go 层由空触发只挂两移除点的结构性论证覆盖）
- **断连重接同一 PTY 的真实进程面**（S6 pidPre==pidPost）——共享进程模型在真实二进制上的同一进程强证据

**Go 测试层已锁定、协议层不重复的面**：空触发恰好一次/宽限计时器启停/exiting 门（emptyexit_test.go 六测）、SignalHangup 送达语义（TestSignalHangup）、CLI 解析三形态与冲突矩阵（cmd/wesh 表测试）、EXIT 组帧 round-trip（proto_test.go）。

**余量关系登记**（prohibition 合规：真实等待，时钟不 mock）：S5 标称宽限 1500ms vs 取消窗 400ms（1100ms 调度余量）；到期断言 5s 护栏轮询（≈1500ms 到期 + 3.5s 抖动余量）；S4 守候窗 400ms ≪ 任何合理误触发时延；waitExit 全带超时护栏（挂死=FAIL 非悬挂）。

**S6 证据形态**（review #4 吸收的双证据互证）：主证据 = 进程 ID 相等（pidPost==pidPre——新 bash 进程必持不同 pid，/S6PID=(\d+)/ 数字锚定防回显误命中）；次级佐证 = shell 变量 weshmark42 跨断连存活；附带锁定 = 首连接全程无 EXIT 帧 + 断开期间服务端存活（默认不开启，D-14）。分工登记：abrupt/正常断开在服务端同归 reader 终结 → detach；1006 触发面由 phase06-dom.mjs 合成事件覆盖。

## Decisions Made

- **S5① 宽限计时起点锚定**：c1 close 后先 waitClose 再 sleep 400ms——close 握手完成 ⇒ 服务端 reader 已收关闭帧触发 detach（宽限计时真实起点），取消窗相对锚点起算使 1100ms 余量论证严格成立
- **sensitiveTokens 全实例收集**（plan 字面『S1/S3 两条 share token』的严格超集）：startWesh 统一留闭包，assertOutputClean 断言面不随场景增删漂移
- **S2a 负向断言补强**：`!message.includes('hangup')`——plan must_have 括号点名的『killed by signal hangup 小写描述词为回归』形态转可执行双向锁（断言面增强，非范围蔓延）
- **S6 readPid 统一解析 helper**：pidPre/pidPost 两调用点同形态，正则数字锚定只命中结果行（终端回显含命令原文 'echo S6PID=$$' 无数字不命中）

## Deviations from Plan

None——plan 逐字执行：read_first 先例（phase05.mjs 五件套/S5 双点位/phase03.mjs 凭据链路/RESEARCH Code Examples）全部精确可抄，两任务首跑即全绿，零修复零调和。

## Threat Flags

None——威胁登记全部 plan threat_model 内闭环：T-06-06a（token/凭据值进输出：红线注释 + SEC 运行时自净双保险落地，23 条 detail 零命中）、T-06-06b（时序假绿/挂死：余量关系注释明示 + waitExit 恒带护栏 + 全实例 SIGKILL 收口 + 复跑无 flake）、T-06-SC（零新依赖零安装）。prohibitions 两项保持：token 值零进输出；宽限/退避场景真实等待（时钟不 mock、服务端等待不缩短、超时上限只做护栏且注释写明余量关系）。

## Known Stubs

None——七场景全链 wired：真实二进制 spawn + 真实 WS/EXIT 帧 + 真实进程退出事件，零占位零 mock。S7 为显式豁免（skipped+reason）非 stub，已登记 .planning/WINDOWS.md（kind=unrun-verify，指向 06-UAT.md 人工清单——06-07 产出）。

## Issues Encountered

None——plan 的 read_first 与逐字先例完备，S1-S6 首跑全绿；复跑确认时序场景（S4 守候窗/S5 宽限）无 flake。

## Next Phase Readiness

- **06-07（收口）**：本 plan 提供 README 文案所需全部行为事实——--once 等价关系（≡ --max-clients=1 --exit-when-empty=0）协议层实证、accept-255 收口文案（「--once/--exit-when-empty 收口 = 子进程被 SIGHUP 终结，wesh 退出状态 255」）三处进程级断言兑现、--exit-when-empty 三形态（不写/裸写/=duration）行为证据；S7 豁免 reason 已逐字指向 06-UAT.md 人工清单落点
- **phase 验收**：SESS-01/02/03 与 CORE-05 协议面端到端证据齐备——ROADMAP 成功准则 1（--once 断开退出 + --exit-when-empty 三形态）、准则 2（EXIT 帧含退出码 + 1000）、准则 3 服务端半侧（重连接回同一 PTY）全部协议层锁定；六段式回归由 06-07 统一执行
- 关注点：无阻塞；assertOutputClean 形态已在 phase06-dom/phase06 两脚本落地，后续 UAT 脚本可直接复用

## Self-Check: PASSED

- 产物文件 web/uat/phase06.mjs + 本 SUMMARY 落盘核实（FOUND ×2）
- 任务提交 e6a46ab / 2940c66 在 git log 核实（FOUND ×2）；两提交 post-commit 删除检查均无文件删除、无遗留 untracked

---

*Phase: 06-session-lifecycle*
*Completed: 2026-08-23*
