---
phase: 12-per-client
plan: 03
subsystem: server-backpressure
tags: [backpressure, slow-consumer, dwell-watchdog, stall-hold, outbox-signal, per-client, ttyd-parity, tdd]

# Dependency graph
requires:
  - phase: 12-per-client plan 02
    provides: cl.pc != nil 分岔形态先例（shared 半侧逐字保留）+ debouncer AfterFunc 纪律邻件 + perclient_test.go harness 扩展面（mutate 覆写通道）
  - phase: 11-per-client
    provides: pcSession 结构与 hubMu 字段组纪律 + kickSlowConsumerLocked 零改动复用面 + teardownPCLocked 序列 + slowclient_test.go stall 夹具/seqFlood/assertKicked1013/readUntilError
  - phase: 05-multi-client
    provides: outbox/trySend/drain/writer 机械与 cap-1 信号量形态（notEmpty）+ maybeExitWhenEmptyLocked AfterFunc 三件套母本 + gateTransitions 计数器（D-05 递增点）
provides:
  - outbox.notFull cap-1 恢复信号量 + drain() 尾部非阻塞发送（「由满转非满」通知通道，shared 路径零消费者无害滞留）
  - ReadLoop 输出闭包 D-01 阻塞持帧形态（停读点递增+武装 → 零锁 select notFull/cl.done → 重试/逃逸 → 续读点递增+Stop 置 nil）+ armSlowDwellLocked 回调（AfterFunc 三件套 + cl.done 早退 → kickSlowConsumerLocked 既有序列）
  - pcSession.dwellTimer 字段（hubMu 保护，exitCode/reaped 同锁先例）
  - defaultSlowDwell=10s 内部常量（D-03 零 CLI/TOML，注释回指 shared 侧 defaultGateDwell 废弃记录反面教材段）
  - Options.SlowDwell 测试可覆写通道 + New 零值兜底 + Server.slowDwell 装配（sessionMode 同位）
  - export_test.go GateTransitionsForTest 观测出口（ForTest 四件套纪律）
  - 四 Go 行为测（停读续读字节连续 / dwell 到期 1013 / 慢但前进不踢 / gateTransitions 配对计数）+ waitGateTransitions/assertSeqContinuity helper
affects: [12-04 (phase12.mjs S5 停读期输出不丢恢复后完整到达 / S6 dwell 到期 1013 端到端——服务端语义全部就位), 12-05 (PC-10/PC-11 勾选证据链 = 本 plan 四测 + 12-04 协议层), 13 (metrics 17→N 镜像扩展时 gateTransitions 双模式聚合口径已就绪)]

# Actuals (#2632) — 与 plan estimate (70000 tokens) 同标尺。
# 口径注记：源码 diff chars/4（internal/server/），排除 .planning。
# 大幅低于 estimate 的结构性原因沿用 12-01/12-02 口径：四测大量复用同包既有夹具
# （seqFlood/assertKicked1013/readUntilError/stall 纪律）；机制侧零件全部既有
# （cap-1 信号量/AfterFunc 三件套/kick 序列零改动复用）。
# 诚实注记：diff 标尺不覆盖执行期调试成本——慢但前进测的滴漏形态经三轮 TCP 层
# 实证（时间线/双端 ss/goroutine 栈/服务端临时插桩）才收敛为事件驱动 duty-cycle，
# 该调查成本未计入 tokens 口径（标尺定义如此，非隐瞒）。
actuals:
  tokens: 8808
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: [] # 零新依赖红线保持（T-12-SC）：go.mod/go.sum 零 diff
  patterns:
    - 事件驱动 duty-cycle 断言形态（停读/续读事件本身作节拍——单轮间隔 < dwell 且累计 > dwell，判别力内建且机器无关；配额/时间驱动滴漏经 TCP 层实证否决）
    - 停读态观测三通道：gateTransitions 轮询（服务端状态机）+ /healthz clients 归零（踢出事实，只读 HTTP 不打扰 WS stall 面）+ wire CloseError（1013 机器串）
    - 观测出口差值断言的配对论证形态（per-client 单模式实例下计数只有闭包两递增点——「+1 于恒不读窗口」与「再 +1 于读取启动后」蕴含停读/续读配对，精确 ==2 断言在瞬态二次停读下 flake 故取下界）
key-files:
  created: [] # 全部为既有文件扩展，无新建
  modified:
    - internal/server/clients.go
    - internal/server/server.go
    - internal/server/perclient.go
    - internal/server/export_test.go
    - internal/server/perclient_test.go

key-decisions:
  - "kickSlowConsumerLocked 调用从 ReadLoop 闭包直踢改造为移入独立回调函数 armSlowDwellLocked（AfterFunc 三件套承载）——使「startSessionGoroutines 函数体内 kickSlowConsumerLocked 零命中」region grep 验收闸字面可满足，同时 dwell 到期路径（hubMu 内 kick 既有序列）与闭包可读性双双改善；武装挂点形态属 CONTEXT Claude's Discretion 既定范围"
  - "慢但前进测的滴漏形态按执行期 TCP 层实证从 plan 文本的「每 ~dwell/3 读一小批」演化为事件驱动 duty-cycle：PTY 行规程使 seq 逐行产出 ~50-500B 微帧、内核 send queue 自适应 ~3-4MiB，配额泵滴漏的窗口信用在亚秒级 dwell 内永不完成「管线排空→writer 解阻塞→drain→notFull→续读」链路（gt 恒 +1 → dwell 正常踢出，机制按 D-02 定义行为正确）；事件驱动形态以停读/续读事件为节拍，判别力内建（单轮 0.5×dwell < dwell、3 轮累计 1.56×dwell > dwell——非重置实现第 2 轮即翻车）且机器无关"
  - "停读续读主测与慢但前进测的静默窗/dwell 参数取 SlowDwell=30s / 1s 覆写（D-03/D-12 测试覆写通道）：默认 10s 下慢 CI 的洪水填充时长会侵蚀「静默窗 ≪ dwell」与「单轮间隔 < dwell」约束——覆写使约束在任意环境稳健；dwell 到期踢出语义由 500ms 短值覆写测独立承担"
  - "dwell 踢出观测经 /healthz clients 归零轮询（只读 HTTP 通道）替代越过到期点的固定 sleep——不打扰 WS stall 面（读取即破坏 stall），轮询替代固定 sleep 纪律（STATE Phase 9 教训）的通道面应用；wire 证据仍由 assertKicked1013 两合法终结形态承载"
  - "[Rule 1/Rule 3 偏差] 12-02 遗留 TestPerClientROInputDropped ro 半场 return 使 rw 对照半场不可达（go vet unreachable 暴露；go test 默认 vet 子集不含该项故 12-02 未现形，rw 对照证据从未执行）——独立 fix 提交 0602e0b 以 labeled break 修复，断言语义逐字不动，解锁本 plan mandated verify（go vet ./...）"
  - "PC-10/PC-11 需求勾选留 phase 末 12-05（ID 跨 12-03/04/05 共享，12-04 协议层证据未落——11-01/12-01/12-02 先例延续）"

patterns-established:
  - "PTY 洪水夹具的微帧形态认知：TTY 行规程下交互程序 stdout 行缓冲（seq 逐行 write），服务端帧粒度 ~50-500B 而非 32KiB 读块量级——后续背压/UAT 场景设计需以此为前提（12-04 phase12.mjs 停读场景直接受益）"
  - "TCP 层诊断四件套（时间线日志/双端 ss -tinm/SIGQUIT goroutine 栈/服务端临时插桩+还原）：停读类时序问题的归因路径，诊断件永不入提交"

requirements-completed: []  # PC-10/PC-11 跨 plan 共享（12-03/12-04/12-05），按既定先例留 phase 末 12-05 勾选

coverage:
  - id: T1
    description: "PC-11 停读续读不丢数据（ttyd pty_pause/resume parity）：停读（闭包阻塞持帧→读循环停摆→PTY 积压→子进程写阻塞）→ 恢复自动续读 → seq 字节严格 +1 连续零缺口"
    requirement: PC-11
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestPerClientStallBlocksAndResumes（-race 绿：停读 +1 轮询 → 静默窗不被踢 → 续读 +1 → 1000 终结 + 首值恒 1/末位 floodLast 连续）"
        status: pass
      - kind: static
        ref: "perclient.go 闭包 make+copy 逐字保持 + 持帧期间零锁（select 等待段零 hubMu）+ creditPending 源码零命中（D-04）"
        status: pass
    human_judgment: false
  - id: T2
    description: "PC-10 持续过载 1013：停读态连续无恢复超过 dwell → kickSlowConsumerLocked 既有序列（1013 slow_consumer / kicks++ / detach 事件 / teardown 挂点零改动复用）"
    requirement: PC-10
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestPerClientDwellKick（-race 绿：SlowDwell=500ms 覆写 → /healthz 归零轮询 → assertKicked1013 两合法终结形态）"
        status: pass
      - kind: static
        ref: "armSlowDwellLocked 回调 hubMu 内身份比对 + cl.done 早退 + 闭包 defer Stop 双防线（T-12-08）；metrics.go/main.go 零 diff（D-05/D-03）"
        status: pass
    human_judgment: false
  - id: T3
    description: "PC-11 慢但在前进永不踢（D-02 判据核心）：每次续读重置 dwell 计时——单轮停读 0.5×dwell < dwell、3 轮累计 1.56×dwell > dwell，全程零 1013"
    requirement: PC-11
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestPerClientDwellNoKickWhileProgressing（-race 绿：3 对停读/续读 + 1000 终结 + seq 连续——非重置实现第 2 轮即翻车的判别力内建）"
        status: pass
    human_judgment: false
  - id: T4
    description: "D-05 观测计数：停读/续读两点递增 registry.gateTransitions（既有 series wesh_credit_gate_transitions_total 多两个递增点，metrics.go 零改动）"
    requirement: PC-10
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestPerClientStallGateTransitions（起止快照差值 ≥2 + 配对论证：per-client 单模式实例下计数只有闭包两递增点）"
        status: pass
      - kind: static
        ref: "git diff --quiet HEAD~2 HEAD -- internal/server/metrics.go（D-05 红线零改动）"
        status: pass
    human_judgment: false

# Metrics
duration: 42min
completed: 2026-09-04
status: complete
---

# Phase 12 Plan 03: PC-10/PC-11 背压语义 Summary

**per-client 输出闭包从「trySend 失败直踢 1013」改造为「outbox notFull 恢复信号 + 零锁阻塞持帧 + dwell 看门狗」——停读不丢数据（帧在闭包栈上，内核缓冲积压子进程写阻塞，ttyd pty_pause parity）→ 恢复自动续读（每次续读重置计时）→ 持续过载 dwell 到期才 1013；WR-01（STATE.md:99）按 D-04「dwell 涵盖不复刻」形态在代码与注释双侧闭合**

## Performance

- **Duration:** 42 min
- **Started:** 2026-09-04T13:27:22Z
- **Completed:** 2026-09-04T14:09:55Z
- **Tasks:** 2（Task 1 机制装配 + Task 2 Go 断言组四测）
- **Files modified:** 5（全 internal/server/，无前端变更）

## Accomplishments

- **outbox 恢复信号（D-01 组合形态零件一）**：`notFull` cap-1 信号量（notEmpty 同形）+ drain() 尾部非阻塞发送——「由满转非满」通知在 outbox 自有 mu 内发出，绝不触 hubMu（恢复通道零 hubMu 反向依赖，锁序三规则 §5）；shared 路径零消费者（token 滞留至多一次伪唤醒，持帧重试失败继续 select 无死锁面——drain 每次发新 token）
- **闭包阻塞持帧（D-01 核心形态）**：trySend 失败分支从「hubMu 内直踢」重写为「停读点（hubMu 内 gateTransitions++ + 武装 dwell）→ 放锁后 select notFull/cl.done（持帧期间零锁——单消费者自由度）→ notFull 唤醒重试同一帧（P5-1：帧在闭包栈上，读循环停摆无别名窗口）→ 续读点（gateTransitions++ + dwell Stop 置 nil——每次续读重置，再停重新武装完整 dwell）」；cl.done 逃逸丢帧合法 + 闭包 defer Stop 兜底
- **dwell 看门狗（D-02/D-03）**：armSlowDwellLocked 独立回调承载 AfterFunc 三件套（var 预声明自引用/回调首句取 hubMu/身份比对 pc.dwellTimer != t 陈旧不动作）+ cl.done 早退 → kickSlowConsumerLocked 既有序列（1013 wire 形态/kicks 计数/detach 事件/teardown 挂点全部零改动复用）；defaultSlowDwell=10s 内部常量（量级依据 ×20 attach 宽限余量 + 后台标签页风险接受段注释载明；反面教材回指 defaultGateDwell 第 5 轮废弃记录——其「信用集冲突」死因在单消费者下结构性不存在）
- **WR-01 闭合回指（D-04）**：dwell 从停读起点武装结构性涵盖 500ms attach 宽限；阻塞持帧即暂存（帧在闭包栈上 ≡ shared 触发帧暂存字段语义等价）——宽限门与 creditPending/afterDrain 重投一律不复刻，注释双侧回指落码（perclient.go 闭包注释段 + clients.go 常量注释段）
- **观测面（D-05）**：停读/续读两点递增 registry.gateTransitions（mode-agnostic 聚合）；metrics.go 零 diff（既有 series 多两个递增点，非新增 series）；export_test.go 增 GateTransitionsForTest（ForTest 四件套：hubMu 内读/调用方不得持锁/仅服务测试）
- **Go 断言组四测（D-11 单一家）**：停读续读主测（静默窗不被踢 + seq 首值恒 1/严格 +1/末位 floodLast 连续——停读期零丢失）/ dwell 踢出测（500ms 覆写 + /healthz 归零轮询 + assertKicked1013 逐字复用）/ 慢但前进不踢测（事件驱动 duty-cycle：单轮 0.5×dwell < dwell、3 轮累计 > dwell——非重置实现必翻车的判别力内建）/ 计数配对测（起止快照差值 ≥2 + per-client 单模式只有闭包两递增点的配对论证）
- **零回归三证据**：`go test -race ./internal/...` 三包全绿（server 81.3s——新增四测 ~12s）；cmd/wesh/main.go 与 internal/server/metrics.go 零 diff（D-03/D-05 红线）；pnpm-lock/go.mod 零 diff（T-12-SC）

## Task Commits

Each task was committed atomically:

1. **Task 1: 机制——outbox 恢复信号 + 闭包阻塞持帧 + dwell 看门狗 + gateTransitions 两递增点 + Options.SlowDwell（D-01/D-02/D-03/D-04/D-05）** - `667ca64` (feat)
2. **Task 2: Go 断言组——停读续读不丢帧 / dwell 到期 1013 / 慢但前进不踢 / gateTransitions 计数（D-11/D-12，PC-10/PC-11 收口）** - `acc02b7` (test)

**Rule 1/3 偏差前置修复**：`0602e0b` (fix)——12-02 遗留 TestPerClientROInputDropped rw 对照半场不可达

**Plan metadata:** docs commit（本 SUMMARY + STATE/ROADMAP 更新）

## Files Created/Modified

- `internal/server/clients.go` - outbox.notFull 字段 + newOutbox 初始化 + drain() 尾部非阻塞信号；defaultSlowDwell=10s 常量（量级依据/反面教材回指/D-03 不暴露理由全注释）；gateTransitions 注释补 per-client 两递增点（D-05）
- `internal/server/server.go` - Options.SlowDwell 测试可覆写字段 + New 零值兜底 + Server.slowDwell 字段声明与装配（sessionMode 同位）
- `internal/server/perclient.go` - pcSession.dwellTimer（hubMu 保护）；ReadLoop 闭包 D-01 形态重写（停读点/select 持帧/续读点 + defer Stop 兜底）；armSlowDwellLocked 回调（AfterFunc 三件套 + cl.done 早退 → kick 既有序列）；WR-01 闭合注释双侧回指
- `internal/server/export_test.go` - GateTransitionsForTest 观测出口
- `internal/server/perclient_test.go` - 四测 + waitGateTransitions/assertSeqContinuity helper（+279 纯新增，既有断言行零改动）；12-02 rw 对照半场可达性修复（独立 fix 提交）

## Decisions Made

（全部登记于 frontmatter key-decisions，此处列执行要点）

- **armSlowDwellLocked 独立回调函数**：kickSlowConsumerLocked 调用移出 startSessionGoroutines 函数体——验收 grep 闸字面可满足 + 闭包可读性；属 CONTEXT Claude's Discretion（武装挂点形态）范围
- **慢但前进测的事件驱动 duty-cycle 演化**（plan 文本「每 ~dwell/3 读一小批」的实证替代）：三轮 TCP 层诊断（滴漏时间线 / 双端 ss -tinm 快照 / SIGQUIT goroutine 栈 / 服务端临时插桩）证明配额泵滴漏在本机永不触发服务端续读——PTY 行规程微帧（~50-500B）+ 内核 send queue 自适应（~3-4MiB，实测 Send-Q 3.2-3.8MB）使 writer 阻塞写在多 MB 管线之后；机制按 D-02 定义行为正确（该消费形态确属停读态连续无恢复），测试形态改为以停读/续读事件为节拍。**该形态机器无关且判别力内建**（单轮间隔 < dwell、累计 > dwell——非重置实现第 2 轮即 1013）
- **dwell 踢出观测通道**：/healthz clients 归零轮询（只读 HTTP 不打扰 WS stall 面）替代固定 sleep 越点等待——轮询替代固定 sleep 纪律的通道面应用
- **gateTransitions 差值断言取下界 ≥2/≥6 而非精确值**：恢复读取后的瞬态二次停读（慢 CI 下 TCP 写瞬断 + outbox 亚毫秒回填）会追加成对增量，精确计数断言有 flake 面（STATE Phase 9 教训同源，「禁精确时点断言」纪律的计数面同构）；两递增点的存在性由配对论证锁定（恒不读窗口的 +1 只能是停读点，读取启动后的 +1 只能是续读点——per-client 单模式实例下无第三递增源）

## TDD Gate Compliance

- Task 1（tdd=true，机制先行形态）：plan Task 2 action 2 显式「测试与机制同 phase 两提交：Task 1 feat 提交 + 本任务 test 提交」（11-01/11-03/12-02 断言收口先例）——机制由既有全量 -race 组为回归网（69.3s 绿 = shared 信用门/slowclient/multi 零回归证据），单 feat 提交
- Task 2（tdd=true，断言收口形态）：四测对已落地机制编写，聚焦组 -race -v 全绿（10.9s）+ 全量 ./internal/... 三包 -race 绿；单 test 提交。RED 证据以调试形态呈现：慢但前进测首版（配额泵）FAIL（gt=+1 < 4）驱动 TCP 层根因调查与形态重设计——测试驱动出的不是实现缺陷而是测试形态缺陷（机制经主测/踢出测/计数测三面锁定为正确），该 RED→修正→GREEN 全程记录于 commit acc02b7 与本文 Decisions 段

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 + Rule 3] 12-02 遗留：TestPerClientROInputDropped rw 对照半场不可达**
- **Found during:** Task 1（go vet ./... 验收）
- **Issue:** ro 半场静默窗 case 内 `return` 使半场二（rw 对照 echo）为死代码——「丢弃是 mode 闸语义而非链路故障」的对照证据从未执行过；go test 默认 vet 子集不含 unreachable 检查故 12-02 未暴露，本 plan mandated verify（go build && go vet && go test -race）被其阻塞
- **Fix:** labeled break roSilent 退出静默窗循环，断言语义逐字不动
- **Files modified:** internal/server/perclient_test.go
- **Commit:** 0602e0b（独立 fix 提交，先于 Task 1 feat）

**2. [Rule 3] 慢但前进测滴漏形态重设计（plan 文本形态的实证替代）**
- **Found during:** Task 2（首版 FAIL：滴漏窗内 gateTransitions 差值 1/3 < 4）
- **Issue:** plan 文本「每 ~dwell/3 读一小批」在真实 TCP/PTY 管线下不构成服务端可观测的「前进」——单条消息/128KiB 配额泵释放的窗口信用不足以完成多 MB 内核管线排空（详见 Decisions 段诊断记录）
- **Fix:** 事件驱动 duty-cycle（停读形成 +1 → 刻意停读 0.5×dwell → 全速读到续读 +1，循环 3 轮）——判别力内建且机器无关
- **Files modified:** internal/server/perclient_test.go（仅新增函数体内，既有断言零触碰）
- **Commit:** acc02b7（含于 Task 2 test 提交，形态演化全程记录于测试 doc 注释）

## Issues Encountered

- gofmt（go1.26.3 doc-comment 规则）对两处行首全角括号接续注释要求加空格——应用即过（10-05 收口闸工具既定）
- 慢但前进测的 TCP 层调查消耗约 25min（三轮诊断：滴漏时间线 / ss -tinm 双端快照 / SIGQUIT 全栈 / 服务端临时插桩+还原）——诊断件全部未入提交；产出固化为「PTY 微帧 + 内核管线量级」认知（patterns-established）供 12-04 UAT 场景设计消费
- 四测加入后全量 server 包 -race 从 69.3s → 81.3s（+12s），可接受

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **12-04（phase12.mjs）**：S5 停读期输出不丢（协议层 seq 连续性 + 真实 10s dwell 到期 1013 一次端到端证据——D-12「真实 10s+ 等待」纪律）可直接消费本 plan 服务端语义；本 plan 的 PTY 微帧/内核管线量级认知（patterns-established）为 UAT 场景时序设计提供实证前提
- **12-05（收口）**：PC-10/PC-11 勾选证据链 = 本 plan 四测（Go 侧）+ 12-04 协议层六场景
- **Phase 13**：gateTransitions 双模式聚合口径就绪（metrics 17→N 镜像扩展时停读/续读递增点已入既有 series）；pcSession.dwellTimer 为停读态 gauge（OPS-12 若需）预留观测挂点

## Threat Flags

无新增威胁面——T-12-07（dwell 踢出 + 资源上界：每 stall 端 1 outbox ≤512KiB + 1 闭包，maxClients 硬顶）、T-12-08（AfterFunc 三件套 + cl.done 早退 + defer Stop 双防线 + removeLocked 幂等三层防线全部落码）、T-12-09（cap-1 信号量 select default 非阻塞，信号频率上界 = writer drain 频率）、T-12-SC（go.mod/go.sum 零 diff）四项 mitigate 处置全部落地验收；T-12-10（detach reason=kick code=1013 既有 schema）accept 既定。

---

*Phase: 12-per-client*
*Completed: 2026-09-04*

## Self-Check: PASSED

- 5/5 关键文件在场（clients.go/server.go/perclient.go/export_test.go/perclient_test.go）
- 3/3 任务提交在场（0602e0b / 667ca64 / acc02b7）
- 锚点 grep（实测值，含注释引用）：clients.go notFull ×5 + defaultSlowDwell ×1；server.go SlowDwell 三锚点（Options 字段/New 兜底/装配）；perclient.go dwellTimer ×14 + armSlowDwellLocked ×6；export_test.go GateTransitionsForTest ×1；四测函数名各 ×1（perclient_test.go）；perclient.go creditPending 零命中（D-04）
