---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: per-client 会话模式
current_phase: 14
status: completed
stopped_at: Phase 14 verify-work 全收口（UAT 34/34 + SECURITY threats_open: 0 + verification passed）——v1.1 milestone 5/5 phases，ready for complete-milestone
last_updated: "2026-09-08T07:10:00Z"
last_activity: 2026-09-08
last_activity_desc: Phase 14 execution started
progress:
  total_phases: 5
  completed_phases: 5
  total_plans: 38
  completed_plans: 38
current_phase_name: herdr-uat
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-08)

**Core value:** 浏览器里获得一个可靠、安全、可多人共享的远程终端
**Current focus:** Milestone v1.1 complete — 5/5 phases, 15/15 requirements closed（Phase 14 经 verify-work 全收口：UAT 34/34 + SECURITY threats_open: 0）

## Current Position

Phase: 14
Plan: Not started
Status: All phases complete
Last activity: 2026-09-08 — Phase 14 complete

Progress: [██████████] 100%

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
| Phase 13 P01 | 30min | 3 tasks | 5 files |
| Phase 13 P02 | 50min | 2 tasks | 6 files |
| Phase 13 P03 | 50min | 2 tasks | 7 files |
| Phase 13 P04 | 13min | 2 tasks | 2 files |
| Phase 13 P05 | 19min | 2 tasks | 6 files |
| Phase 13 P06 | 23min | 2 tasks | 11 files |
| Phase 13 P07 | 27min | 2 tasks | 2 files |
| Phase 13 P08 | 37min | 2 tasks | 11 files |
| Phase 14 P01 | 43min | 2 tasks | 7 files |
| Phase 14 P07 | 40min | 2 tasks | 1 files |
| Phase 14 P08 | 23min | 2 tasks | 1 files |
| Phase 14 P02 | 28min | 2 tasks | 2 files |
| Phase 14 P03 | 25min | 3 tasks | 3 files |
| Phase 14 P04 | 21min | 2 tasks | 4 files |
| Phase 14 P05 | 25min | 2 tasks | 8 files |
| Phase 14 P06 | 28min | 2 tasks | 8 files |
| Phase 14 P09 | 19h50min | 2 tasks | 6 files |
| Phase 14 P10 | 16min | 2 tasks | 1 files |
| Phase 14 P11 | 15min | 2 tasks | 3 files |
| Phase 14 P12 | 9h20min | 3 tasks | 6 files |
| Phase 14 P13 | 35min | 3 tasks | 5 files |

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
- [Phase ?]: [Phase 13-01] D-01 one-way 门 option-a 用户派发确认落定：per-client --stop-timeout 未显式设置默认 5s（HUP 免疫泄漏防线默认开启）+ 显式 0 经 stopTimeoutSet 显式位尊重并 warn 泄漏风险；shared 字面 0 逐字不动（12-01 D-08 one-way 门先例同形态）
- [Phase ?]: [Phase 13-01] Task 3 Rule 3 可测性提取：run() 内联双默认值覆写提取为纯函数 resolveStopTimeout（loadCustomIndex 同位纪律）——plan「终值落定直调」的直调点；TestValidateStartupWarnMerge 加第四枚负例（per-client 未设无 warn，Rule 2 判别力——锚定显式位而非终值的过宽实现必翻车）
- [Phase ?]: [Phase 13-02] 双桶判序 per-IP 先/全局后（短路）：单 IP churn 过量尝试在 per-IP 桶即拒（AllowN 失败零消耗）不耗全局预算——反代后合法多用户共享全局配额不被单一 churning IP 占干（plan/RESEARCH 蓝本均未定序，实现 latitude 裁决）
- [Phase ?]: [Phase 13-02] Rule 3：TestPerClientTeardownRaceOnce mutate 放宽 per-IP 桶（10 轮同 IP 连续 attach 超默认 burst 4，plan 对既有测试 attach 密度枚举缺口；测试对象 teardown 竞态非 churn 防线，断言行零改动）
- [Phase ?]: [Phase 13-02] 惰性过期判别面：perIPRate=1/s burst=1200 > 15min TTL 补给上限 900——正补给使小 burst 的 allow-结果无判别力（重置与补给同满额），三相位精确计数 1200/120/1200 双向判别（无重置 960 翻车/过早重置相位二 1200 翻车）
- [Phase ?]: [Phase 13-02] 事件 schema 键集白名单含日志封套（time/level/msg）+ 四段 schema——parseEvents 解析整行 JSON，纯四段白名单必翻车（Task 2 内自愈的 Rule 1 测试 bug）
- [Phase ?]: [Phase 13-02] PC-08 勾选留 phase 末收口 plan（11-01/12-01/13-01 先例：ID 跨 plan 共享——机制本体已落地，13-03 场景面/13-07 压测面证据未齐）
- [Phase ?]: [Phase 13-02] GOROOT gofmt（go1.26.3）存量命中两处（cmd/wesh/main_test.go 13-01 遗留 + perclient_test.go:1535 Phase 12 遗留，均为 //（ CJK 标点接续行）登记 deferred-items.md——范围外不修，13-08 收口闸应知悉
- [Phase ?]: [Phase 13-03] session_end emit 位置取 close(waitDone) 前（plan「hubMu 解锁后」的 Rule 1 偏差）：与 shared lifecycle 母本同位 + 建立 emit→close(waitDone)→delete+Broadcast→exitf 的 happens-before 链——plan 位置与慢半段 Broadcast 无同步边，调度停摆下 exitf(os.Exit) 可先于事件落流
- [Phase ?]: [Phase 13-03] Rule 1 关键补齐：upgradePerClient registerLocked 后补宽限取消点 + 空纪元门闩清零（11-01 早退守卫期两挂点 per-client 从未装配；触发端激活后缺失 = 宽限取消仅靠复查兜底 + exit-when-empty 单发缺陷）——GraceCancel 测试首跑实测暴露，单独 fix 提交
- [Phase ?]: [Phase 13-03] WR-02 栅栏同构覆盖补 KILL 回调（planner 裁定「Pitfall 2 语义对一切 kill(-pgid) 同构适用」）；子先死断言值 42（判别力收紧）；PC-09/OPS-12 勾选留 phase 末（13-07 phase13.mjs S3 进程级 255 断言未齐）
- [Phase ?]: [Phase 13-04] Shutdown 侧 D-state 兜底 terminate 落地（Rule 2——T-13-13 mitigation 为 threat register 硬要求而 plan behavior 四锚点未列 terminate 调用）：drained 形态终结仍归 pcSupervisor 零漂移，仅 join 到期未清零分支 Shutdown 直调 terminate（termOnce 交汇恰好一次，退出码同 last-reaped-code 规则）；形态四测试（stopTimeout=0+免疫=D-state 代理）锁定 exitf(0)+存活探针双观测
- [Phase ?]: [Phase 13-04] 快照信号循环加 WR-02 waitDone 栅栏（研究明示 Discretion——按 13-03 planner 裁定「Pitfall 2 语义对一切 kill(-pgid) 同构适用」选栅栏形态）；join 实现形态选 hubCond.Wait + AfterFunc 兜底 Broadcast（零新同步件）；join 上界余量定值 shutdownJoinMargin=2s（研究 A1 保守形态）；测试夹具复用 startPerClientServerWithSpawn（11-03 已参数化，零新装配）；PC-09/OPS-12 勾选留 phase 末（先例延续）
- [Phase ?]: [Phase 13-05] TestSpawnEventsSchema 零成功 spawn 构造（per-IP burst=1 + 恒败注入：dial ① spawn_failed + dial ② spawn_throttled 一窗捕获）——零会话零 watcher 使迟到 emit 面结构性不存在（13-03 跨测试迟写教训前置规避）；wire 定值文案逐 dial 绑定 + 注入敏感值三形态（err.Error()/路径/errno）零出现负断言
- [Phase ?]: [Phase 13-05] ptyKills 恰按 plan 枚举两路径（teardown + 孤儿回收 AfterFunc），13-04 Shutdown 路径补 KILL 不在计数面（plan 白名单明示两路径；series HELP 文案已如实限定 teardown and orphan reaping）——运维面若需 Shutdown KILL 计数属 series 语义扩展，13-07/13-08 复核知悉项
- [Phase ?]: [Phase 13-06] Task 1/2 TDD 按先例单 feat 提交收口（RED=编译红任务内观察即转 GREEN）；darwin 双编译闸前移抓出 reap_darwin_test.go whitelistEnv 两参遗漏（build-tag 文件 Linux 编译面不含，独立 fix 提交）——签名扩散类改动 darwin 闸应随任务即跑
- [Phase ?]: [Phase 13-06] SEC-09 WESH_REMOTE_USER 落地：whitelistEnv 第三参空串不出键（键名白名单固定代码常量由 pty 包单侧定义）+ SpawnFunc 三参签名全链 + main.go 闭包 startOpts 局部复制防串台（T-13-21）；startPerClientServer 默认 spawnFn 升级生产镜像完整形态（注释契约真值优先）；签名扩散波及 shutdown/metrics/events 三测试文件（plan files_modified 枚举缺口，WINDOWS #36）；SEC-09 勾选留 phase 末（13-07 S6 进程级 env 回读未齐）
- [Phase ?]: [Phase 13-07] phase13.mjs 六场景两轮 29/29（phase12 同构第三代 + dialAttach 双形态 dial 合流）；S2 KILL 兜底默认 5s 三面判别（~2s 存活 + ESRCH + elapsed≥4s 下界，实测 5.0s）；S1 事件/计数器/拒绝数三方精确相等 + XFF 换键双态（事件 remote==XFF 链首）；S6c printenv 缺席断言以 echo 标记程序序锚定
- [Phase ?]: [Phase 13-07] TestChurn churn 负载格：10rps×30s（300 次）生产默认桶参数零覆写——attached=33/rejected=267/throttled=267、gor 8→8/fd 11→11/mem +110KB 精确回落基线；断言全部基线差值形态（gor/mem/fd 双采样 + 回收轮询 + 容差标定注释——Pitfall 7）；spawn_total==attached 程序序精确对照；PC-08/09/SEC-09/OPS-12 勾选留 13-08（Task 2 承载）
- [Phase ?]: [Phase 13-08] 收口闸六段式全绿（零回归双证据 + phase13 两轮 29/29 + churn 格 + diff 白名单 23 文件零外改）；phase 基点 = e0ae66b^（首提交父提交，12-05 先例同构）；PC-08/09/SEC-09/OPS-12 四需求勾选收口（v1.1 10/15）
- [Phase ?]: [Phase 13-08] WR-02 闭合回指：waitDone 非阻塞 select 结构性栅栏落地且同构覆盖三处 kill(-pgid) 面（teardown 快半段/补 KILL 回调/Shutdown 快照循环）——结构性消除非风险接受；D-10 文档段落地（README 保活先杀时序 + CONFIGURATION ping-interval 小节）；gofmt 三处 deferred 存量与 README「per-client 装配中」失实残留收口期归一
- [Phase ?]: [Phase 14-01] D-01 小族四形态落地并双模式实证：newTestServer/newTracked/newHandle/newSess 两分支直传两族母本零改写；tracked 形态经 TestOversize1009、handle 形态经 TestHealthzDraining 获运行期首证，sess 形态编译期在场（14-04 首证）——后续 14-02..06 改造 plan 直接复用
- [Phase ?]: [Phase 14-01] TestSlowConsumerKick per-client 列同步边勘误（Rule 1）：独立洪水拓扑下 shared 的「正常端 12MiB 等待」不构成踢出同步——首次 Read 过早续读重置 dwell 使会话 1000 收尾（全量 -race 负载实测命中）；kick 观测统一走 /healthz clients 计数轮询（TestPerClientDwellKick 先例通道），Read 推迟到踢出可观测后
- [Phase ?]: [Phase 14-01] CI 时长增量实测：双跑使 internal/server -race 全量从 ~98.5s 增至 ~104-106s（+6-8s，洪水类测双跑为主贡献）；14-02..06 批量推开持续累积——14-12 收口闸知悉 CI 时长预算（蓝本估算 3-4 min/leg）
- [Phase ?]: [Phase 14-01] TestReadLimitBoundary D-02 偏差登记：startRawCatServer 的 pre-listen stty 装配与 per-client attach 期 spawn 结构性不等价（无 pre-listen 窗口）——蓝本 limits 同断言双跑行的例外，保持 shared 单跑（边界值断言与进程模型无关，per-client 同值面由 TestOversize1009/PreHelloReadLimit 双跑承载）
- [Phase ?]: [Phase 14-07] D-11 判定证真：32 会话实测（wesh 侧 Alloc 增量 2.2MB ≈ 24MiB 账面 9% / 子进程 VmRSS 合计 120MB < 160MB 账面 / fd 差值精确 4N / gor 5N+1）全部在可接受界线内——maxClients=32 默认值不动（零公开契约变更），one-way 确认门不触发；LOADDATA pc_flood/pc_resident 八行供 14-11 README 标定表回填
- [Phase ?]: [Phase 14-07] Rule 1 夹具修正：loadDrain.note() 流尾采样帧级重置改跨帧滚动窗——tty ONLCR 行尾拆分使末帧载荷仅 \r\n（探针实证），帧级重置把帧边界误当流边界误判流截断（洪水格 sessions_16 复现率 4/6）；滚动窗保流截断判别力且内存恒 ≤128B，既有六负载格全套件回归绿
- [Phase ?]: [Phase 14-07] 驻留格 fd 账面 4N 推导归类：master+pidfd+accepted 为服务端生产账面（ARCHITECTURE §10）+ dial socket 为 in-process harness 成本单独注释归类——实测差值精确 4N 逐项证实；gor 按生产账面 6N 收口（harness pinger 退场实测 5N+1）；PC-12 勾选留 14-11 文档承载（shared-ID 门先例）
- [Phase ?]: [Phase 14-08] herdr UAT 就绪门 Rule 1 实测修正：「layouts 非空」不充分——server 启动期 area 先 {0,0,0,0} 再默认布局瞬态 {x>0,width≈默认宽-侧栏}（后者同样满足 isFullFor 谓词）且移动端过早 attach 使桌面 size 上报成 last-activity（compact-40 永不现 + S1d 假绿）；修正为桌面初始全量帧到达落定（OUTPUT>0 且 150ms 双采样相等）+ area 桌面全量几何双条件门——后续 herdr 断言类 UAT 沿用
- [Phase ?]: [Phase 14-08] herdr 清理序列补 session delete：stop 后 session list 行保留 stopped 态（探针实证），delete 才清册+状态目录——「list 零残留」完整序列 = stop → delete → list 核验；session stop 以会话名为准（ambient HERDR_SOCKET_PATH 不干扰，执行 shell 位于 herdr pane 内的运行期再实证）
- [Phase ?]: [Phase 14-08] phase14.mjs 两轮 18/18（S1 driving 三通道：area 翻转链四步 + 流层 254B/55B<<97049B + wesh 层双 pid/双 Welcome/流几何 120vs40；S2 ro 汇聚：ticket 全链 + pane read 门控实证 + rw 对照）；PC-13 勾选留 14-09（pw 观感层承载，共享 ID 先例）
- [Phase ?]: [Phase 14-08] maxCursorCol 流层几何特征通道：CUP/HVP/CHA 三形态正则扫描取列坐标最大值（spike 只标定 CUP，实现扩展）——移动端几何只能寻址 ≤cols 列的关系断言材料，禁绝对常量
- [Phase ?]: [Phase 14-02] e2e/multi 双模式分叉表落地：断开/重连（CORE-05 反转面经 newSessTestServer spawned 访问器取 pid 锚点，argv 零漂移）、fanout 双标记串交叉断言、owner 四测 D-03 四段式未装配列（零 t.Skip）、MaxClients spawn-intent 经 wesh_pty_spawn_total 程序序对照——50 个 mode= 子测试 -race 逐名绿，shared 87 条期望文案逐字零漂移
- [Phase ?]: [Phase 14-03] 三文件归一映射落定：TestShutdown1001 per-client 列=TwoGroups 归位（N=2 组，plan action 3 明示 shared 保持单组语义）、StopTimeout 列=JoinBounded 归位（stop-timeout KILL 兜底唯一同需求映射）；ResidualGroup/DeadlineExits 判 per-client-only 保持原样（pcSessions 残留/D-state 收口无 shared 对照面）；emptyexit/metrics 混入测全归位吸收
- [Phase ?]: [Phase 14-03] TestExitWhenEmptyPromoteKickOnce per-client 列 D-03 四段式 + 尾部 Shutdown 收口（1min 悬挂宽限计时器在 per-client 无 lifecycle-exiting 免疫——不收口即 +1min 迟触发 exit_when_empty 事件污染后继捕获窗；Pattern 7 供后续长 grace per-client 测试沿用）
- [Phase ?]: [Phase 14-03] series 镜像口径以 metrics.go 现状核定：双模式同持 21 series（shared 四 spawn 计数器恒 0 不摘），plan 文本「17 shared/21 per-client」与 13-05 落地不符——WINDOWS #42 登记（14-12 diff 白名单审查知悉）；TestMetricsValues 放大比分叉 shared ×2 / per-client 1:1（R-08）
- [Phase ?]: [Phase 14-04] resize_arb per-client 列三重可证伪面：双端各自 RESIZE（A→110x40/B→70x25）+ B 落定后 A 保持 110x40 负向余量断言（互不压缩为 Isolation 测外独有面）+ 双泵静默窗零 'W' 帧——比 TestGlobalCredit 单面形态更完整的三观测面（Pattern 8）
- [Phase ?]: [Phase 14-04] startResizeServer 收编内联终态：装配体逐字内联至 newSessTestServer shared 分支（helper 名消失满足全仓零引用字面闸；e2e_test.go:36 注释同步）——14-02 startShutdownServerWith 后第二例专用 helper 收编闭环；Rule 3 枚举缺口（harness/e2e 不在 files_modified）WINDOWS 登记
- [Phase ?]: [Phase 14-04] 蓝本三则归属偏差登记 CONTEXT 落地（D-02 偏差登记小节——14-06 续登 events/log 的既定通道）：owner 四测实在 multi_test.go:541/647/746/850（plan 行号 :301/373/428/497 为规划期陈旧值）/ resize wire 面实在 resize_arb_test.go / 计数不变量白盒面 clients_test:29 + wire 面 TestMaxClients503:1222；clients_test/resize_test 六测纯白盒单跑判定
- [Phase ?]: [Phase 14-05] TestReadOnlyAllowsResize 为 handshake 行唯一分叉面：ro 运行期 RESIZE 两模式真值相反（shared D-09 忽略/per-client D-06 直通）——按 14-01 断言分叉表落地（shared 逐字 + per-client 断言直通真值 50/120），exitf 面同 TestExitFrameSignal 分叉；WINDOWS #44
- [Phase ?]: [Phase 14-05] 零值/Writable:false Options 经 mutate 显式回写（Pattern 9）：小族基线 Writable:true 下 TestNoAuthMode/TestReadOnly* 覆写回 false 保期望值零改写；TestPingDisabled 显式 PingInterval=0 锁定禁用语义防基线漂移
- [Phase ?]: [Phase 14-05] auth/origin/throttle/tickets 四文件 6 测纯函数白盒形态判定保持单跑（无装配可参数化，非覆盖缺口）；auth_e2e :403 收编 newTrackedTestServer（tracked 形态第二消费方）；startTestServer 零引用收编删除（14-02/04 闭环第三例，WINDOWS #45）
- [Phase ?]: [Phase 14-06] sharetoken 白盒镜像双分支落地：package server 白盒文件经本地 startShareServer(t,mode) 镜像小族两分支（per-client = New(nil)+SpawnFunc 生产闭包镜像+spawned Kill+Close Cleanup）——包墙结构性不可达 server_test 小族的 D-01 同构形态；部署面 9 测双跑收口（sharetoken 2+customindex 8+proxy_e2e 4+basepath 3，proxy_e2e 5 调用点按 waitHandlers 用途分流）
- [Phase ?]: [Phase 14-06] 三维归类收口核对：server 包实测 34 文件（plan 33+harness_test.go 小族本体零测桶）= ① 18 文件 81 测双跑（-v mode= 子测 216 PASS）+② perclient 36 测单模式+③ load 8 测+④ 纯白盒/纯函数 10 文件+⑤ events 9/log 1 单跑（D-02 续登第 4/5 则）+⑥ export 零测桶；蓝本 :378-399 十七行全命中或偏差登记，零静默漏网——SC1 单 step 双模式 CI 门形态达成（ci.yml 零 diff）
- [Phase ?]: [Phase 14-06] TestMaxClients503/mode=per-client 隔离复跑 flake 判 14-02 遗留（基线 9af7ce1 worktree 复现 3/3，pcSessions linger 窗口竞态，轮询仅覆盖 HTTP 503 形态）——与本 plan 无关（全量 -race 两轮全绿），deferred-items.md 登记三修复方向，14-12 知悉
- [Phase ?]: [Phase 14-09] pw 层海森bug 架构裁决（Rule 3 实证驱动，承重）：浏览器 tab 首键入后输出流非确定性死亡为载具级（10 轮实跑 + 23 探针 + minrepro 双形态均不复现——herdr/wesh 服务端无罪）；键入通道改 Windows Node 裸 WS 驱动端（ticket 认证），浏览器双 tab 退化为纯被动渲染观测面，D-07 断言面不变（桌面 tab 渲染观测 + 结构行对齐比较）
- [Phase ?]: [Phase 14-09] 结构行幸存前缀判别面：目标行 = 含 │ 且边框右侧空白的结构行（pane 内容行排除——前台 reflow 属 herdr 正确行为）；pane 内容向下增长只从尾部吞 blank 行使幸存前缀序位不变，shared 压缩会整体改写边框列——herdr 类 TUI 布局稳定性断言的判别面形态
- [Phase ?]: [Phase 14-09] minrepro 双件套交付（Track2 偏差副产）：minrepro-p14.mjs（Linux loopback）+ minrepro-win.mjs（Windows 过转发器）——两形态均未复现停摆本身即服务端无罪的反向证据，供 herdr 上游定位浏览器输入链停摆
- [Phase ?]: [Phase 14-09] PC-13 勾选收口（14-08 既定裁决兑现：协议层 18/18 + 浏览器面 Windows 两轮 4/4 + 截图六帧人工复核——共享 ID 先例 11-01/12-01/13-01）
- [Phase ?]: [Phase 14-10] run-all.mjs 17 项矩阵 runner 落地（D-04 形式化）：串行 spawn 聚合 + 10min 超时护栏（detached 进程组 SIGTERM→2s→SIGKILL）+ argv[3:] 过滤子集（未知名 exit 2 防静默跑空 + 脚本名误传二进制位守卫）+ pw 独立入口注记；全矩阵首跑 17/17 全绿 213.4s，逐脚本计数与 13-08 基线完全一致（phase14 实测 5.6s 远快于 A6 预估 1-2min）——SC2 三脚本重跑证据成型，14-12 收口闸单命令复用载具就位
- [Phase ?]: [Phase 14-10] Rule 3 清障：残留核验命中的 2 wesh 进程 + 2 herdr 会话经 ps 起始时间实证为 14-09 minrepro/pw 诊断陈旧残留（早于本次矩阵运行 2.5h+，非本次泄漏——矩阵自身 phase14 S1j/S2g 零残留 PASS），SIGTERM + session stop/delete 清障后 default 外零残留——14-12 收口闸从清洁基线起跑
- [Phase ?]: [Phase 14-11] PC-12 三件套落地：README「会话模式」节同位扩展（:96 段字节级不动纯插入——shared 表述零削弱以 diff 纯新增自证）+ ARCHITECTURE 七分支点/goroutine 拓扑 mermaid 段 + :7 GoTTY 误记修正（实为 per-connection spawn）+ CONFIGURATION max-clients 兼任进程上限行；herdr 配方取 phase14.mjs argv 逐字形态（--writable 在前——T-14-23 实证优先于 plan 文本 flag 序）；PC-12 勾选留 14-12（共享 ID 门）
- [Phase ?]: [Phase 14-11] D-11 建议值表三档分档：默认 32「实测可承载，保持不变」明示 / 低配 VPS·内存受限 8 / 个人多端 4——依据列全部锚定 14-07 LOADDATA 实测值（每会话 bash ~3.7MiB + wesh 侧 ~61KiB），标注资源画像分档非硬性门槛；标定表 30 项程序化数据核对全过（N=16 取整 959→960KiB 自审修正）
- [Phase ?]: [Phase 14-12] Phase 14 收口闸六段式全绿：静态面（gofmt/vet 含 -tags=load 零输出）+ 全量 -race 五包 2m37.986s（mode= 子测 216 RUN/216 PASS 与 14-06 收口审计精确一致）+ darwin amd64/arm64 双编译闸四命令零错 + dist byte-identical（md5 d5c25e27 复建一致）+ UAT 矩阵 17/17 零修改重跑（210.9s 逐脚本计数与 13-08 基线一致）+ load 双剖面八格 + diff 白名单终审零外改动
- [Phase ?]: [Phase 14-12] phase 基点 = 03268f0^（Phase 14 首提交 docs(14): capture phase context 的父提交 = 96a188a Phase 13 PR #17 合并点）——branching_strategy=none 平直 main 下 merge-base 退化，11-06/12-05/13-08 先例同构
- [Phase ?]: [Phase 14-12] diff 白名单 35 文件终审零外改动：27 internal/server 测试文件恰=各 plan files_modified 并集（26 M + harness_test.go A）+ 3 文档（14-11 三件套）+ 5 web/uat 新增（3 plan 声明 + minrepro 双件套 14-09 Track2 偏差副产经 #33 三轴裁决先例纳入白名单）；565 真删除行全归类零期望值漂移（断言文案字面量全保留，仅结构包裹/装配点收编/变量名形态变化）
- [Phase ?]: [Phase 14-12] PC-12/PC-13 勾选收口——v1.1 全部 15/15 需求闭合（PC-12 证据链 = 三文档段 + 14-07 LOADDATA 八格回填 + diff 审查；PC-13 证据链 = phase14.mjs 两轮 18/18 + phase14-pw Windows 两轮 4/4 截图六帧 + run-all 三脚本重跑）；flagged_assumptions 终验：14-08 PC-13 unclassified 三边界形态检索结论登记，µs 级/版本漂移类保持 flagged-unverified（主面覆盖 + 同构论证口径，13-08 先例同构）
- [Phase ?]: [Phase 14-12] Key Decisions 三行登记 PROJECT.md：三维归类执行机械（newTestServer 小族 + t.Run 双跑，ci.yml 零 diff 单 step 天然覆盖）/ maxClients=32 经负载矩阵实测成立不动（D-11 两段式——零公开契约变更 one-way 门不触发，README 三档建议值表）/ herdr 观测通道钉定（api snapshot 翻转链 + pane read + 版本钉定 0.8.100）
- [Phase ?]: [Phase 14-13] G-14-34 闭合：ARCHITECTURE 组件图 mermaid 词法修复（4 subgraph 合法 id + 引号标题 + 12 边标签引号化，含 L41 {token} DIAMOND_START 主修复点）红转绿 + check-mermaid.mjs 词法校验载具常驻 scripts/（负对照自证堵 14-11 结构化校验不查词法的漏检面）+ 用户渲染目检 approved——UAT 34/34 全过 gap 清零

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 9 遗留]: TestResize CI 时序 flake（CI 观察一次，重载 runner 调度延迟所致，非产品缺陷）——择机以轮询替代固定 sleep 修复
- [Phase 9 遗留]: README.md:96「及其 `.gz`」Phase 1 遗留文档债——随 WR 清单择机处置
- [v1.1 测试拓扑]: 协议层 UAT 在 Linux 开发机（headless 禁浏览器/禁 playwright）；Playwright 浏览器全链在 Windows 工作站（TCP 转发器 kill/restore 模拟断网）——见 CODEBUDDY.md 双机拓扑
- [14-06 执行期发现→14-12 知悉] TestMaxClients503/mode=per-client 隔离复跑（-run 过滤非 -race 形态）高概率 flake：pcSessions linger 窗口竞态（detach 槽位释放早于收割，pre-spawn 容量再闸 1011；轮询仅重试 HTTP 503）——14-02 遗留（基线 9af7ce1 复现 3/3），全量 -race CI 同款命令不受影响；deferred-items.md 已登记修复方向

<!-- v1.1 收口清理（2026-09-08，Phase 14 verify-work 转场时）：
  - [v1.1 规划期裁决项] 移除：① stop-timeout 默认 5s 经 13-01 落地（27909f8 三态断言组）；③ healthz/metrics 四 OQ 经 13-03/13-05 落地；②④ 原文已注明闭合
  - [Phase 12-04 发现 → Phase 13 裁决] pinger/dwell 竞态 移除：13-08 D-10 裁决=接受 1006 语义（死连接更早收口）+ README.md:155「保活先杀时序」文档化收口
-->

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-09-08T07:10:00Z
Stopped at: Phase 14 全收口（verify-work：UAT 34/34 + gap G-14-34 对账 resolved + SECURITY.md threats_open: 0 + verification passed + 转场完成）——v1.1 milestone 5/5 phases ready for /gsd-complete-milestone v1.1
Resume file: None
