---
phase: 05-multi-client
plan: 02
subsystem: api
tags: [go, websocket, backpressure, credit-gate, slow-consumer, sigwinch, pty, multi-client, race]

# Dependency graph
requires:
  - phase: 05-multi-client plan 01
    provides: 注册表/hub 共享帧扇出/outbox/writer/kickSlowConsumerLocked/detach 多客户端拓扑；五默认常量与 Options 覆写字段
provides:
  - 全局信用门（RES-04）：hubCond 挂 hubMu，全体可写端 creditBlocked → onChunk 持块停读 PTY；afterDrain 半水位（<cap/2）恢复 + Broadcast
  - R-08 踢出/信用分工表：ro 满即踢 / rw 满且非全体满即踢 / 全体满计信用；触发帧 creditPending 暂存重投（门转换零丢帧）
  - P5-7 四处统一 Broadcast（detach/kick/attach/lifecycle）+ pinger 结构性独立于 hubMu 的死锁免除链路
  - registry.gateTransitions 门开闭周期计数（Phase 8 OPS-07 挂点，review #10 stub）
  - pty.Session.SignalForegroundGroup（D-11：TIOCGPGRP → kill(-pgid, SIGWINCH)，静默降级）+ Attach 升档完成唯一调用点
  - TestSlowConsumerKick / TestGlobalCredit（两子场景）/ TestSigwinchOnAttach 三测试锁定行为
affects: [05-03 权限体系（write-policy=all 适配项已登记测试注释）, 05-04 resize 仲裁, 05-05 限速, 05-07 max-clients, Phase 8 OPS-07 metrics, Phase 9 dwell 计时器评估]

# Actuals (#2632)
actuals:
  tokens: 9215
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "全局信用门：sync.Cond 挂 hubMu——Wait 原子释放 hubMu，门闭合期间 detach/kick/attach/lifecycle 均可获锁重估（P5-7 死锁免除）"
    - "踢出/信用分工表（R-08）：ro 满即踢；rw 满看全体；全体满置 creditBlocked + 暂存触发帧（creditPending），afterDrain 半水位重投+清位+Broadcast"
    - "1013 帧可达性不变量：kick 的 cancel 推迟到异步 Close 落定后——否则 defer CloseNow 先赢 casClosing，stall 端只见 EOF"
    - "stall 夹具：dialHello 后不 Read；洪水必须超过 loopback 单连接最坏吸收量（本机实测 wmem 4MiB + rmem 6MiB）才传导到 outbox"
    - "确定性角色构造：先 attach 者领先 1s ⇒ 先满被踢、后满者持信用（黑盒测试可断言确定角色）"
    - "SIGWINCH 送达证据：helper stdin 读字节 → 装处理器报 READY → 收信号落盘标记（消除安装竞态 + stdout 不可观测纪律）"

key-files:
  created:
    - internal/server/slowclient_test.go
  modified:
    - internal/server/clients.go
    - internal/server/server.go
    - internal/pty/io.go
    - internal/server/multi_test.go
    - internal/server/e2e_test.go
    - go.mod

key-decisions:
  - "[Phase 05-02]: kick 路径 cancel 推迟到异步 Close 落定之后（goroutine 内 Close 先行赢 casClosing）——05-01 的同步 cancel 使 Attach defer CloseNow 先硬关 TCP，1013 关闭帧对 stall 端永不可达（TestSlowConsumerKick 实测只见 EOF）；重排后读端消化管道即可见 CloseError{1013, slow_consumer}"
  - "[Phase 05-02]: 信用路径触发帧暂存 creditPending 并在 afterDrain 清位前重投——plan/RESEARCH 机制（trySend 失败即置位）会丢当前帧，违反 plan 自身 prohibition『禁止丢帧保连接』；TestGlobalCredit 门转换字节精确断言实测抓到缺 1 帧（review #1 行为证据锁住的正是此窗口）"
  - "[Phase 05-02]: 背压测试参数推导——OutboxBytes 取 64KiB（cap ≥ 2×maxChunk 数学下限，plan 示例 8KiB 会使空 outbox 的整帧 trySend 恒败）；洪水 seq 1 5000000/4000000（38.9/30.9MB > loopback 最坏吸收）；测试客户端 SetReadLimit 4MiB（writer 合并段超 Go 库默认 32KiB 触发 1009）"

patterns-established:
  - "stall 场景黑盒观测三件套：对端 CloseError 1013 取证 / exitCh 静默窗口证门闭合（子进程写阻塞）/ waitExit(0) 有界开门证据"
  - "字节精确断言形态：strings.Fields 切分免疫 ONLCR + seq 字段连续 +1 + 末字段 == N——门转换/合并写出路径的流完整性回归锁"
  - "outbox 容量数学不变量注释：cap ≥ 2×（32KiB+1）是 trySend/重投必成的前提（默认 512KiB，测试覆写 ≥64KiB）"

requirements-completed: [MULTI-03, RES-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "stall 客户端（建连后不 Read）outbox 写满后被 1013 踢出，close reason 逐字 slow_consumer，stderr 落 logEvent(remote,1013,slow_consumer)；同实例第二客户端 fan-out 采样单调增长无卡顿"
    requirement: MULTI-03
    verification:
      - kind: integration
        ref: "internal/server/slowclient_test.go#TestSlowConsumerKick"
        status: pass
    human_judgment: false
  - id: D2
    description: "全体可写端 stall → 信用门闭合（子进程写阻塞不退出，exitCh 静默反证）；一端恢复 Read → 半水位开门，seq 字段序列连续 +1 无重复无乱序、末字段==4000000（门转换字节精确）"
    requirement: RES-04
    verification:
      - kind: integration
        ref: "internal/server/slowclient_test.go#TestGlobalCredit/恢复Read开门_字节精确"
        status: pass
    human_judgment: false
  - id: D3
    description: "门闭合期间持信用端死亡（dead owner）→ detach → Broadcast → 注册表空 → 门 5s 内有界重开（waitExit(0) 证据）"
    requirement: RES-04
    verification:
      - kind: integration
        ref: "internal/server/slowclient_test.go#TestGlobalCredit/CloseNow有界开门"
        status: pass
    human_judgment: false
  - id: D4
    description: "新客户端 attach 完成时服务端向 PTY 前台进程组显式发一次 SIGWINCH（同尺寸 80x24 排除内核 resize 信号干扰），helper 收信号落盘 GOT_WINCH 标记"
    requirement: MULTI-03
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestSigwinchOnAttach"
        status: pass
    human_judgment: false

# Metrics
duration: 1h 17m
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 02: 全局信用门 + SIGWINCH Summary

**在 05-01 hub 骨架上落地 RES-04 全局信用门（全体可写端满 → cond 持块停读 PTY，半水位恢复）与 R-08 踢出/信用分工表，并修复两处实测发现的正确性缺陷（1013 帧可达性、门转换丢帧）；附带 D-11 新客首屏 SIGWINCH（TIOCGPGRP→kill(-pgid)），三测试全量 -race 绿。**

## Performance

- **Duration:** 1h 17m
- **Started:** 2026-08-20T08:15:29Z
- **Completed:** 2026-08-20T09:32:10Z
- **Tasks:** 3
- **Files modified:** 7（1 新建 + 6 修改）

## Accomplishments

- RES-04 信用门完整落码：`hubCond`（挂 hubMu，R-07 单锁纪律）+ `allWritableBlockedLocked`（≥1 可写端且全部 creditBlocked 才闭合；纯 ro/空会话门永不闭合）+ onChunk 门 Wait 循环恒在组帧之前（review #1 行号顺序断言锁定）；持块 = ReadLoop 停读 = PTY 64KiB 内核缓冲填满 = 子进程 write 自然阻塞（唯一合法反压路径）
- R-08 分工表 + 迟滞带：`kickOrCreditLocked`（ro 满即踢 / rw 满看全体 / 全体满计信用）+ `afterDrain` 半水位（<cap/2 = 默认 256KiB）恢复——关闭阈值 100% 写满、恢复阈值 <50% 的 2:1 迟滞带注释逐字落码（review #2）；`registry.gateTransitions` 置位/清位两点递增（Phase 8 OPS-07 挂点，review #10 stub）
- P5-7 死锁免除：detach/kick/attach/lifecycle 四处统一 Broadcast；pinger 仅触达 conn 级 writeFrameMu（mu.lock ctx 5s 超时）从不取 hubMu——门闭合期间死连接 ≤(ping 间隔+5s) 经 pong_timeout+CloseNow→detach 收口
- D-11 落地：`pty.Session.SignalForegroundGroup`（fdMu 纪律与 Resize 同款；TIOCGPGRP 失败/无前台进程组/closed 静默降级）+ Attach 升档完成（注册表登记+writer/pinger 启动后）唯一调用点；go.mod x/sys 转 direct（go.sum 零新增）
- 三测试锁定行为：TestSlowConsumerKick（1013 slow_consumer + 正常端单调增长）、TestGlobalCredit 两子场景（恢复开门字节精确 + CloseNow 5s 有界开门）、TestSigwinchOnAttach（GOT_WINCH 落盘证据）；server+pty+proto 三包 -race 全绿，新测试 3 连跑零失败

## Task Commits

Each task was committed atomically:

1. **Task 1: 全局信用门 + 踢出/信用分工表 + Broadcast 统一挂点** - `a25eafb` (feat)
2. **Task 2: pty.Session.SignalForegroundGroup（D-11）+ attach 完成调用点 + x/sys direct** - `f8b6646` (feat)
3. **Task 3: slowclient 两测试 + SIGWINCH 送达集成断言（含两处 Rule 1 修正）** - `335625c` (test)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `internal/server/slowclient_test.go`（新）- stall 夹具（dialHello 后不 Read）+ TestSlowConsumerKick + TestGlobalCredit 两子场景；readUntilError/assertKicked1013/assertExitSilent 三 helper（Pitfall 2：客户端 Read 永不带 deadline ctx）
- `internal/server/clients.go` - creditBlocked/creditPending 字段、gateTransitions 计数器、onChunk 信用门 Wait（顺序不变量注释）、allWritableBlockedLocked、kickOrCreditLocked（分工表+触发帧暂存）、afterDrain（重投+半水位清位+Broadcast）、kick/detach 两处 Broadcast、锁序头部注释（hubMu > outboxMu 逐字）
- `internal/server/server.go` - hubCond 字段与 New 构造、registerLocked 后 Broadcast（attach 挂点）、lifecycle 广播关闭后 Broadcast（第四挂点）、Attach 尾段 SignalForegroundGroup 调用
- `internal/pty/io.go` - SignalForegroundGroup（TIOCGPGRP → unix.Kill(-pgid, SIGWINCH)，静默降级注释要点逐字）
- `internal/server/multi_test.go` - TestSigwinchOnAttach（READY 同步 + 同尺寸排除内核信号 + 5s 轮询标记；CI macos runner 覆盖注释——A1 验证通道）
- `internal/server/e2e_test.go` - TestHelperProcess 新增 wesh-helper-winch 分支（stdin 读字节武装 → READY → 收信号落盘 GOT_WINCH）+ markerArg 解析扩展
- `go.mod` - golang.org/x/sys v0.47.0 indirect→direct（go mod tidy 顺带修正 coder/websocket、creack/pty 的误标 indirect；go.sum 零新增）

## Decisions Made

- **1013 帧可达性不变量**：05-01 的 kick 路径同步 cancel → Attach 读循环终结 → defer CloseNow 立刻硬关 TCP；Close/CloseNow 竞态由先到的 casClosing 取胜（close.go:101/134），CloseNow 先胜则 1013 关闭帧永不上 wire——stall 端只见 EOF（TestSlowConsumerKick 实测命中）。修正：cancel 移入异步 goroutine 内 Close 落定之后，Close 先赢 casClosing → 读端消化管道后必见 CloseError{1013, slow_consumer}；对永不读取的真死连接 writeClose 5s 超时照常收口，pinger 在关闭窗口期继续 ping 属无害（精确分类静默返回）。
- **触发帧暂存重投（门转换零丢帧）**：plan/RESEARCH 的机制（trySend 失败即置 creditBlocked）把被拒的当前帧丢弃——恢复端流缺一段，正是 plan 自身 prohibition『输出帧不得被静默丢弃』禁止的『丢帧保连接』形态。修正：client.creditPending 暂存触发帧（共享只读堆帧，P5-1 纪律不变），afterDrain 在清位/Broadcast 之前重投（重投必成的数学保证：cur < cap/2 ⇒ 余量 ≥ cap/2+1 ≥ 32KiB+1，前提 cap ≥ 64KiB 已注释登记）；门仍闭合期间 onChunk 无法夹入新帧，有序性成立。
- **背压测试参数推导**：本机实测 net.ipv4.tcp_wmem/rmem 上限 4MiB/6MiB——plan 沿用的 seq 1 200000（~1.3MB）会被 loopback 管道完全吸收，stall 永不传导到 outbox；洪水取 38.9MB（踢出测试，>单连接吸收+采样窗口余量）与 30.9MB（信用门测试，>双连接 ~20MiB + 2×64KiB outbox + 64KiB 内核缓冲）。OutboxBytes 取 64KiB 而非 plan 示例 8KiB：8KiB < 最大单帧 32KiB+1 会使空 outbox 的 trySend 恒败（健康端也被踢，违反 prohibition）。测试客户端 SetReadLimit 4MiB：writer 合并段可达 outbox 容量量级，超 Go 库默认 32KiB 触发 1009（实测命中；浏览器前端无此上限）。
- **确定性角色构造**：两客户端同时 stall 时分工表下先满者必被踢（剔除后仍有未 blocked 端）、后满者独自持信用——利用此性质以 1s attach 领先窗构造确定角色（c1=被踢、c2=持信用），黑盒测试无需窥探注册表内部状态。
- **SIGWINCH 测试同步纪律**：helper 先 stdin 读字节（INPUT 帧驱动）再装处理器报 READY——attach 信号若先于处理器安装会被默认忽略（默认 disposition = Ignore），READY 回读确认后第二客户端 attach 触发第二次信号，竞态消除；两端同尺寸 80x24 排除内核 TIOCSWINSZ 异尺寸信号干扰（P5-3 本机实证同尺寸不发）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] kick 路径 cancel 时序修正：1013 关闭帧可达性**
- **Found during:** Task 3（TestSlowConsumerKick 首跑：stall 端收到 EOF 而非 CloseError 1013）
- **Issue:** 05-01 kick 路径 `close(done); c.cancel(); go Close(1013)`——cancel 使 Attach 读循环终结、`defer c.CloseNow()` 立刻硬关 TCP；CloseNow 先赢 casClosing 后 1013 关闭帧永不上 wire，plan 验收『stall 端 CloseError.Code == 1013』结构性不可达
- **Fix:** cancel 移入异步 goroutine 内 `_ = c.conn.Close(...)` 之后（Close 先赢 casClosing → CloseNow 退化为 wait-only no-op）；clients.go 注释落码不变量与库源码行号证据
- **Files modified:** internal/server/clients.go
- **Verification:** TestSlowConsumerKick/TestGlobalCredit 三处 1013 断言全绿；既有套件零回归
- **Committed in:** 335625c

**2. [Rule 1 - Bug] 信用路径触发帧暂存重投：门转换零丢帧**
- **Found during:** Task 3（TestGlobalCredit 字节精确断言：`seq discontinuity at field 507469: 1038423 -> 10384241038467`——门转换点缺 1 帧 ~340B）
- **Issue:** plan/RESEARCH 机制在 trySend 失败时置 creditBlocked 即罢，被拒的当前帧对持信用端永久丢失——违反 plan 自身 prohibition『输出帧不得被静默丢弃……任何「丢帧保连接」形态都是有序流画面静默损坏，禁止出现』（review #1 要求的行为证据断言正是为此而设）
- **Fix:** client.creditPending 暂存触发帧；afterDrain 半水位恢复时在清位/Broadcast 之前重投（重投必成数学保证与 cap ≥ 64KiB 前提注释落码；防御性失败分支保位保帧下次再试）
- **Files modified:** internal/server/clients.go
- **Verification:** 字节精确断言绿（连续 +1、末字段 == 4000000）；新测试 3 连跑零失败
- **Committed in:** 335625c

**3. [Rule 3 - Blocking] stall 夹具洪水尺寸与测试参数重推导**
- **Found during:** Task 3 夹具设计（本机 /proc 实测 net.ipv4.tcp_wmem 上限 4MiB、tcp_rmem 上限 6MiB）
- **Issue:** plan 沿用的 seq 1 200000（~1.3MB）洪水会被 loopback 单连接最坏 ~10MiB 吸收量完全吞掉，stall 永不传导到 outbox 写满，踢出/信用门测试结构性不触发；plan 示例 OutboxBytes 8KiB < 最大单帧 32KiB+1，空 outbox trySend 恒败会误踢健康端（违反 prohibition）；writer 合并段超 Go 客户端库默认 32KiB 读上限触发 1009（实测命中）
- **Fix:** 洪水 38.9MB/30.9MB（超吸收量 1.5-2 倍余量）、OutboxBytes 64KiB（cap ≥ 2×maxChunk 下限）、测试客户端 SetReadLimit 4MiB（注释登记浏览器无此上限）、1s 领先窗确定性角色构造
- **Files modified:** internal/server/slowclient_test.go
- **Verification:** 三测试 3 连跑 -race 零失败
- **Committed in:** 335625c

---

**Total deviations:** 3 auto-fixed（2 Rule 1 - Bug，1 Rule 3 - Blocking）
**Impact on plan:** 两处 Rule 1 修正是 plan 自身 truths/prohibitions/验收断言的直接要求（1013 可达、零丢帧），未改动 plan 锁定的机制形态（cond 门/分工表/半水位/Broadcast 挂点全部逐字保持）；Rule 3 为测试夹具参数推导，无生产代码影响。

## Known Stubs

- `registry.gateTransitions` 门开闭周期计数器：plan 授权的观测性 stub（review #10），仅递增无读取方——Phase 8 OPS-07/OPS-08 进 metrics/结构化日志时消费（与 05-01 的 registry.kicks 同批）。
- 05-03 适配登记：TestGlobalCredit 以 Options{Writable:true} 构造两 rw 前提——05-03 落地 write-policy=owner 默认后第二客户端会降级 ro，届时本测试须显式传 write-policy=all（测试注释已登记）。

## Issues Encountered

- **plan/RESEARCH 机制级缺陷两处**（1013 可达性、门转换丢帧）——均由 plan 要求的实测断言捕获并修复，见 Deviations 1/2。教训已转化为不变量注释（casClosing 竞态、触发帧重投有序性）。
- **stderr 1013 事件未做捕获断言**：logEvent 由 ReadLoop goroutine 写出，captureStderr restore() 与其无 happens-before 边（05-01 deviation #3 同款竞态）；wire 级 CloseError.Reason == "slow_consumer" 与 logEvent 机器串同源同值（kickSlowConsumerLocked 单调用点），wire 断言已锁定逐字。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的占位与后续挂点）

- **darwin SIGWINCH 行为假设 A1**：同尺寸 TIOCSWINSZ 不发信号系 Linux 本机实证（P5-3），darwin 外推未本机验证——TestSigwinchOnAttach 在 CI macos runner 双平台矩阵执行为验证通道；即便 darwin 同尺寸发信号，显式 SIGWINCH 也只是冗余无害
- **残余振荡接受**：滴漏读者每次 drain 开门、下一 chunk 可能再闭门——RESEARCH Open Question 3 裁决接受（各端持续前进），dwell 计时器 Phase 9 负载标定时评估
- **allWritableBlockedLocked O(n) 遍历**：每 chunk 一次、规模 ≤ max-clients 32，review LOW 项接受，代码注释已登记
- **05-03 适配项**：TestGlobalCredit 两 rw 前提届时须显式 write-policy=all（Known Stubs 已登记）

## Next Phase Readiness

- 信用门/分工表/Broadcast 挂点齐备——05-03 owner 递补（owner 断线 → detach → 门重估链路已含递补重算挂点位）、05-04 仲裁重算均可直接在四处 Broadcast 挂点旁扩展
- SignalForegroundGroup 已由 attach 路径无条件触发，05-04 仲裁 resize 落地后新客首屏双保险（显式信号 + 异尺寸内核信号）
- 无阻塞项；全量 -race 绿，新测试 3 连跑稳定

## Self-Check: PASSED

- FOUND: internal/server/slowclient_test.go（func TestSlowConsumerKick/TestGlobalCredit，grep == 2）
- FOUND: internal/pty/io.go SignalForegroundGroup（grep == 1）、internal/server/server.go 调用点（grep == 1）
- FOUND: commit a25eafb / f8b6646 / 335625c 均在 git log
- 验收 grep 全过：hubCond == 8（≥4）、三分工函数 == 3、creditBlocked == 14（≥3）、gateTransitions == 5（≥3）、门行 179 < 组帧行 182、迟滞带 == 3、hubMu > outboxMu == 2、wesh-helper-winch == 1、go.mod x/sys 无 indirect
- 全量 go test -race -count=1 ./internal/server/ ./internal/pty/ ./internal/proto/ 绿；新测试 -race 3 连跑零失败

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
