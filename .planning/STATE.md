---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: per-client 会话模式
current_phase: 12
current_phase_name: per-client
status: "Phase 12 shipped — PR #16"
stopped_at: Completed 12-per-client 12-05-PLAN.md（Phase 12 收口：五需求勾选 + WR-01 闭合登记——phase 5/5 ready for verification）
last_updated: "2026-09-05T05:57:03.849Z"
last_activity: 2026-09-05
progress:
  total_phases: 3
  completed_phases: 3
  total_plans: 17
  completed_plans: 17
last_activity_desc: Phase 11 gap closure complete
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-04)

**Core value:** 浏览器里获得一个可靠、安全、可多人共享的远程终端
**Current focus:** Phase 12 — per-client

## Current Position

Phase: 12 (per-client) — EXECUTING
Plan: 5 of 5
Status: Phase 12 shipped — PR #16
Last activity: 2026-09-05

Progress: [██████████] 100%（v1.1；v1.0 已 9/9 阶段 70/70 计划收口，v1.0.0 已发布）

## Performance Metrics

**Velocity:**

- Total plans completed: 70（v1.0）
- Average duration: -
- Total execution time: -

**By Phase (v1.0):**

| Phase | Plans | Phase | Plans |
|-------|-------|-------|-------|
| 01 | 5 | 06 | 7 |
| 02 | 6 | 07 | 10 |
| 03 | 7 | 08 | 6 |
| 04 | 6 | 09 | 10 |
| 05 | 13 | | |

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 10 P01 | 32min | 2 tasks | 6 files |
| Phase 10 P02 | 35 min | 2 tasks | 4 files |
| Phase 10 P03 | 18 min | 2 tasks | 2 files |
| Phase 10 P04 | 25 min | 2 tasks | 2 files |
| Phase 10 P05 | 31min | 3 tasks | 3 files |
| Phase 11-per-client P01 | 40min | 2 tasks | 5 files |
| Phase 11-per-client P02 | 9min | 2 tasks | 2 files |
| Phase 11-per-client P03 | 19min | 2 tasks | 3 files |
| Phase 11-per-client P04 | 30min | 2 tasks | 1 files |
| Phase 11-per-client P05 | 18min | 2 tasks | 1 files |
| Phase 11-per-client P06 | 11min | 2 tasks | 0 files |
| Phase 11-per-client P07 | 1h50min | 3 tasks | 1 files |
| Phase 12 P01 | 38min | 3 tasks | 11 files |
| Phase 12 P02 | 20min | 3 tasks | 7 files |
| Phase 12 P03 | 42min | 2 tasks | 5 files |
| Phase 12 P04 | 22min | 2 tasks | 1 files |
| Phase 12-per-client P05 | 22min | 2 tasks | 2 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Milestone]: v1.0 全量收口（44/44 需求），v1.0.0 于 2026-08-30 实发布上架（四平台 + checksums 核验全 OK）
- [Roadmap v1.1]: 五阶段沿研究骨架与依赖链——装配阀门(10) ≺ 生命周期主干(11) ≺ 交互背压(12) ≺ 资源防线与终结语义(13) ≺ 标定/UAT(14)；2026-09-03 原 13/14 合并（原 13 经 Phase 11 D-01/D-03 机制先行收窄后独立 phase 开销过重，合并同时提前闭合 --once 窗口期缺口）、原 15 重编号 14
- [Research v1.1]: 零新增依赖；「装配期一次分岔、运行期零分岔」不抽象 session 接口（6-7 显式分支点）；最大风险=破坏既有不变量而不自知（D-10 唯一终结/D-13 零新 exitf/唯一收割者/Welcome 恒首帧/零身份 label）
- [Requirements v1.1]: D5 裁决落定——SEC-09 per-client 下 WESH_REMOTE_USER 注入子进程 env（D-15 收窄理由结构性消失），shared 保持收窄语义；反特性五条入 Out of Scope（reattach/linger/运行期切模式/默认 per-client/ro 共享进程）
- [Phase ?]: [Phase 10-01] run() 两模式均经启动期 pty.Start（sess=nil 与 New 体 sess.Cmd.Process.Pid 取引用冲突，归 Phase 11）；SpawnFunc 闭包 inert 零调用方
- [Phase ?]: [Phase 10-01] ValidateOptions 包级互斥校验 option (b) 落地：per-client×SpawnFunc=nil / shared×SpawnFunc≠nil fail-fast，零值归一 shared 与 New 兜底同口径
- [Phase ?]: [Phase 10-05] GOROOT gofmt（go1.26.3 现代 doc-comment 规则）定为收口闸工具：10-01 遗留两行 CJK 标点接续注释补空格归一（a412a87），新旧 gofmt 双 clean；历史闸用 PATH 旧版 gofmt 故未报
- [Phase ?]: [Phase 11-01] perclient_test.go 落 package server_test（plan 文本 package server 与「同包复用 e2e_test.go helper」矛盾，按后者裁决）
- [Phase ?]: [Phase 11-01] Task 1 TDD 以 plan 显式单 feat 提交收口；PC-02/03/04 需求勾选留给 phase 末 plan 11-06（ID 跨 6 plan 共享）
- [Phase ?]: [Phase 11-02] darwin exit watcher dup-watch fail-closed 落地（Pitfall 9 挂账兑现）：errDupWatch 包级错误值 + watch() w.mu 内 dup 检查；awaitExit 既有分支退化 cmd.Wait() 兜底零新面；TestWatchDupPidFailClosed 由 CI macOS leg 承担运行
- [Phase ?]: [Phase 11-03] D-02 容量再闸落地：capacityMessage 常量 + rejectCapacity 单点（两容量拒绝路径 wire 不可区分是有意为之）；Task 2 TDD 单 test 提交延续 11-01 先例
- [Phase ?]: [Phase 11-03] D-03 复检回收落地：硬不变量「并发子进程数 ≤ maxClients」Phase 11 即成立——Phase 13 裁决项④提前消解，Phase 13 规划时移除 STATE Blockers ④
- [Phase ?]: [Phase 11-04] plan 文本 kill -TERM $$ 勘误为 kill -HUP $$：交互 shell 无 trap 忽略 SIGTERM（实测不致死），HUP 致死且与 exit_test.go 信号夹具同款——后续 plan 信号夹具选型应直接用 HUP/trap 形态
- [Phase ?]: [Phase 11-04] 竞态注入测关闭观测形态：客户端主动 Close 后 Read 恒 net.ErrClosed（库 prepareRead 语义），「读至 CloseError」经并发泵 + 1000 证据双通道（泵 CloseError / Close nil 返回）实现
- [Phase ?]: [Phase 11-04] PC-03/PC-04 需求勾选延续既定归 phase 末 11-06（跨 6 plan 共享 ID）
- [Phase ?]: [Phase 11-05] S5d 自杀信号沿用 11-04 勘误（kill -TERM→-HUP，STATE 裁决既定）；pid 数值纳入 SEC 运行时自净扫描（sensitivePids 三通道）
- [Phase ?]: [Phase 11-05] PC-02/03/04 需求勾选延续既定归 phase 末 11-06（跨 6 plan 共享 ID + plan flagged_assumptions 明示保持 flagged-unverified）
- [Phase ?]: [Phase 11-06] Phase 11 收口闸六段式全绿：静态面+全量-race(5包1m5.6s)+darwin双闸+八脚本两轮基线一致(12/18/10/28/23/34/21/18)+phase11.mjs 21/21+1skip+diff四件套；prohibitions 19/19人工确认零违反
- [Phase ?]: [Phase 11-06] phase基点口径：branching_strategy=none下merge-base退化为HEAD——以phase首提交父提交954da7c为等价基点；PC-02/03/04勾选承载兑现（十四测+八场景+diff审查三证据链）
- [Phase 11 REVIEW WR-01 → Phase 12]: per-client 输出闭包 trySend 失败直踢 1013（kickSlowConsumerLocked），丢失 05-13 attach 宽限与信用门暂存层——慢链路新端瞬态满箱即循环丢会话；PATTERNS:218 母本为 kickOrCreditLocked。Phase 12（1013/背压语义主场）规划时收口：补宽限门 + creditPending/afterDrain 重投
- [Phase 11 REVIEW WR-02 → Phase 13]: reaped 栅栏 Wait-return→hubMu-acquire 微窗口（kill-after-reap 理论面，实际不可达=pid 回绕+µs 窗）——零成本严格修法：waitDone 在 reap 完成点关闭 + 快半段非阻塞 select 即结构性栅栏。随 Phase 13 终结语义一并处置
- [Phase 11-per-client]: [Phase 11-07 CI 复验副产] ubuntu flaky 根因 = CI 慢 runner shell 冷启动慢，PS1 打印晚于 tty 回显落在回显行与结果行之间（"$ MARKER" 交错形态，CI run 33843785651 实证）——9936f2b 以 (?:\$ )? 容忍修正三处（InputEcho/echoMarker/ExitPrivate42 B 端），结果行锚定与回显行排除语义不变（六案例自检）；后续终端类测试断言沿用该容忍形态
- [Phase 11-per-client]: [Phase 11 secure-phase] 21 威胁全 closed（threats_open: 0，L1 grep 深度 + register_authored_at_plan_time 短路）；accepted risks 三条登记（AR-1 审计空白→Phase 13 / AR-2 持续 EPERM 语义=护栏正确翻车 / AR-3 零新依赖供应链窗口）
- [Phase 11-per-client]: [Phase 11-07] 单一文件门基点按 plan 规则以实际起始 HEAD 975af23 替换假设基点 f55c1ea（两者间仅 7358b82/975af23 两个 .planning-only 提交，f55c1ea 交叉核对同结果：恰 internal/server/perclient_test.go 一文件）；waitPgroupESRCH EPERM 容忍语义经探针参数化（waitPgroupESRCHWithProbe 四子测）确定性锁定，护栏保留与他错立即 Fatal 两半边零弱化
- [Phase ?]: [Phase 12-01] D-08 one-way 门 option-a 用户派发确认落定：session 字符串枚举恒序列化（G-05-1 同形态），五 Welcome 组帧调用点统一恒传 s.sessionMode
- [Phase ?]: [Phase 12-01] reset 判别通道以 @xterm/headless 探针实证升级：clear() 不退 alt screen 且不清其背后 normal buffer——phase12-dom D1 以 1049l 残影复活链路锁定 reset 效应（plan 原 DOM 空白断言两态皆过无判别力）
- [Phase ?]: [Phase 12-01] 模式位解析缺键语义按 sessionDims :655-664 容错同构：缺键（旧服务端）静默 shared，键在场值非法 warn；PC-06 勾选留 phase 末 12-05（跨 plan 共享，11-01 先例）
- [Phase ?]: [Phase 12-02] winsize 观测面复用同包 ptySize/pollSize（creack/pty Getsize 即 TIOCSWINSZ 直读）——plan 原文 unix.IoctlGetWinsize 同语义既有件，零新代码零新导入
- [Phase ?]: [Phase 12-02] D2 判别面经 onopen lastReported 同步语义（Hello 即首报）收敛：握手后基线恒零 RESIZE，布局桩突变+resize 事件才产生新帧——消除 WELCOME refit 假阳面
- [Phase ?]: [Phase 12-02] PC-05/PC-07 需求勾选留 phase 末 12-05（ID 跨 12-02/04/05 共享，12-04 协议层证据未落——11-01/12-01 先例延续）
- [Phase ?]: [Phase 12-03] kickSlowConsumerLocked 调用移入独立回调 armSlowDwellLocked（AfterFunc 三件套承载）——startSessionGoroutines 函数体 kick 零命中验收闸字面可满足 + 闭包可读性；武装挂点形态属 Claude's Discretion 范围
- [Phase ?]: [Phase 12-03] 慢但前进测滴漏形态经三轮 TCP 层实证（滴漏时间线/双端 ss/SIGQUIT 栈/服务端临时插桩）从 plan 文本「每 dwell/3 读一小批」演化为事件驱动 duty-cycle：PTY 行规程微帧 ~50-500B + 内核 send queue 自适应 ~3-4MiB 使亚秒级配额滴漏永不触发服务端续读（机制按 D-02 定义正确）；事件形态判别力内建（单轮 0.5×dwell < dwell、3 轮累计 > dwell，非重置实现必翻车）且机器无关
- [Phase ?]: [Phase 12-03] dwell 踢出观测经 /healthz clients 归零轮询（只读 HTTP 不打扰 WS stall 面）替代固定 sleep 越点等待；gateTransitions 差值断言取下界（≥2/≥6）——瞬态二次停读使精确计数 flake（Phase 9 教训），两递增点存在性由配对论证锁定
- [Phase ?]: [Phase 12-03] Rule 1/3 偏差：12-02 遗留 TestPerClientROInputDropped ro 半场 return 使 rw 对照半场不可达（go vet unreachable 暴露，go test 默认 vet 子集不含故 12-02 未现形）——labeled break 修复（0602e0b），解锁本 plan mandated verify；PC-10/PC-11 勾选留 12-05
- [Phase ?]: [Phase 12-03] WR-01（Phase 11 REVIEW 遗留）按 D-04「dwell 涵盖不复刻」形态闭合：dwell 10s 从停读起点武装结构性涵盖 500ms attach 宽限（×20 余量）与一切瞬态满箱；阻塞持帧即暂存（帧在闭包栈上 ≡ shared 暂存字段语义等价，单消费者下复刻即死代码面）——宽限门与 creditPending/afterDrain 重投均不复刻，代码与注释双侧回指（perclient.go 闭包注释 + clients.go defaultSlowDwell 注释）；若瞬态满箱误踢案例实证出现，回写重开（CONTEXT deferred 既定）
- [Phase ?]: [Phase 12-04] S6 场景形态裁决（Rule 3 实证驱动）：默认 --ping-interval=5s 下 dwell 1013 被 1006 pong_timeout 结构性先杀——coder/websocket writeControl 内层 5s 写超时（write.go:277-279）使 ping tick 落在 writer 持锁阻塞于满 TCP 窗口时 mu.lock 超时返回 DeadlineExceeded，被 pinger 单一判读误认为 pong 超时（实测 detach 恰于 attach+10.0007s）；S6 以生产 CLI flag --ping-interval=0（D-16「0 = 禁用保活」，Go harness 零值同构）隔离 dwell 看门狗，dwell 本身生产 10s 零覆写真实等待（两轮实测 10.6s/10.4s）
- [Phase ?]: [Phase 12-04] 洪水量修正：plan 文本「seq 1 400000 级，超 outbox 512KiB 即足量」与 TCP 吸收带事实不符（2.7MB < ~10.6MiB 吸收带 → 停读永不形成、场景空转假绿）——按 slowclient_test.go 吸收带纪律上调至 seq 1 4000000（30.9MB ≈ 3× 余量，Go seqFlood Linux 分支同款）；S5 恢复期零输入纪律（tty 回显与洪水共用输出流，发标记会破坏连续性校验面——收齐信号 = 尾窗 '3999999\r\n4000000\r\n' 终态联合形态）
- [Phase ?]: [Phase 12-04] phase12.mjs 六场景两轮全绿（20/20×2）：Welcome.session 双模式 / resize 直通隔离+零 W 帧 / ro RESIZE 直通+shared 对照 / ro INPUT 丢弃+rw 限速 / 停读续读 34.9MB 字节级连续 / 真实 dwell 1013+ESRCH；RawStallClient raw socket 停读夹具（phase05 rawStallClient 一般化）为后续 phase 可复用件；PC-05/06/07/10/11 勾选留 12-05（既定先例）
- [Phase ?]: [Phase 12-05] Phase 12 收口闸六段式全绿：静态面（gofmt/vet 零输出）+ 全量 -race 5 包 1m21s（新测 12/12 逐名）+ darwin 双编译闸 + dist byte-identical + UAT 矩阵 16 轮（既有 10 协议脚本默认 shared 零修改与基线逐脚本一致 + 3 jsdom + phase12 两轮 20/20 + phase12-dom 14/14）+ diff 白名单审查（放宽形态零命中/红线文件零 diff/零新依赖 0 行）；PC-05/06/07/10/11 五需求勾选收口（三证据链映射表）
- [Phase ?]: [Phase 12-05] WR-01（Phase 11 REVIEW 遗留）闭合回指登记（D-04「dwell 涵盖不复刻」形态）：dwell 10s 从停读起点武装结构性涵盖 500ms attach 宽限（×20 余量）；阻塞持帧即暂存（帧在闭包栈上 ≡ creditPending 语义等价）——宽限门与 creditPending/afterDrain 重投均不复刻；若瞬态满箱误踢案例实证出现则回写重开（CONTEXT deferred 口径）；登记项 STATE.md 规划期 :99 现位 :103（12-01..04 决策追加行移，内容逐字核对）
- [Phase ?]: [Phase 12-05] diff 审查白名单补充项①：export_test.go M（+17/-0 GateTransitionsForTest 观测出口）为 12-03 plan 明示落地项，零断言纯观测出口文件——12-05 plan 白名单枚举未列属 plan 文本枚举缺口而非回归，三轴裁决（plan 授权/append-only/零断言）如实登记（WINDOWS #33）不判收口失败；phase 基点 = e8b39c0（86433a6^ Phase 12 首提交父提交，11-06 先例同构）

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 9 遗留]: TestResize CI 时序 flake（CI 观察一次，重载 runner 调度延迟所致，非产品缺陷）——择机以轮询替代固定 sleep 修复
- [Phase 9 遗留]: README.md:96「及其 `.gz`」Phase 1 遗留文档债——随 WR 清单择机处置
- [v1.1 规划期裁决项]: ① per-client stop-timeout 默认值重议（0=不补 KILL 在新模式下=HUP 免疫泄漏，公开契约变更，Pitfall 8）→ Phase 13——**Phase 11 post-merge 调查已实证泄漏窗真实存在**（2026-09-04）：本机 bash 4.4 交互模式在「提示符 pselect + 竞态输入行待读」窗口内可无声吸收 SIGHUP（kill 成功发出、非阻塞非 pending、进程存活；cat 对照组 50/50 全收，服务端信号面零缺陷），11-04 竞态测试经 StopTimeout=1s 覆写走 KILL 兜底确定性收口（14143fe）；③ healthz/metrics 四个 OQ（session_alive 语义/series 双语义/1013 vs 阻塞/spawn 失败 wire 面，研究均有推荐答案）→ Phase 13（② write-policy×per-client 经 Phase 10 D-01/D-02 闭合；④ spawn-intent 口径经 Phase 11 D-03 复检回收提前消解）
- [v1.1 测试拓扑]: 协议层 UAT 在 Linux 开发机（headless 禁浏览器/禁 playwright）；Playwright 浏览器全链在 Windows 工作站（TCP 转发器 kill/restore 模拟断网）——见 CODEBUDDY.md 双机拓扑
- [Phase 12-04 发现 → Phase 13 裁决] pinger/dwell 竞态：默认 --ping-interval=5s 下 TCP 级停读客户端在 (停读+5s, 停读+10s] 被 1006 pong_timeout 先杀，PC-10 dwell 1013 结构性后到（writeControl 5s 写超时 × pinger 单一 DeadlineExceeded 判读；Go 测 harness PingInterval 零值未暴露，phase12.mjs S6 以 --ping-interval=0 隔离取证）。真实浏览器端网络栈自动回 pong 不触发；herdr 类自管 socket 客户端可触发。裁决面：pinger 区分「写阻塞超时」与「pong 等待超时」（lib 错误链 failed to acquire lock vs failed to wait for pong 可区分）或接受 1006 语义（死连接更早收口）

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-09-04T15:24:04.860Z
Stopped at: Completed 12-per-client 12-05-PLAN.md（Phase 12 收口：五需求勾选 + WR-01 闭合登记——phase 5/5 ready for verification）
Resume file: None
