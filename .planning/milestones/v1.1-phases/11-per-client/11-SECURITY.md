---
phase: 11
slug: per-client
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-04
---

# Phase 11 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 已认证 WS 客户端 → attach 升档 spawn 路径 | ticket 核销是唯一资源创建前置；升档后每连接 = 一个 PTY 子进程 + 一套 goroutine（昂贵资源） | Hello 尺寸（钳制后）；spawn 结果（定值文案） |
| 服务端 → 宿主机进程组（kill(-pgid) 信号面） | 断开/超时路径向子进程组发信号；pgid 复用窗口内误发 = 杀宿主机无关进程组 | SIGHUP/SIGKILL 信号（作用域锁定） |
| 子进程 → 客户端（OUTPUT/EXIT 数据面） | per-client 输出与退出码只许到达属主客户端（会话状态越权可见 = 信息泄露） | PTY 输出流；exit_code/信号语义 |
| goroutine 拓扑（teardown × watcher × detach 竞态面） | 双路终结触发并发到达；锁序与恰好一次语义是结构性防线 | 控制面状态（hubMu 保护） |
| 服务端内部 → darwin 内核 kqueue 订阅面 | (ident,filter) 唯一键语义下重复注册 = 替换；影子化使收割挂死（可用性事故） | kqueue 事件订阅（进程内） |
| 已认证客户端 → spawn 路径容量面 | per-client 下每连接 = 一个子进程；容量闸是昂贵资源的唯一硬帽 | 并发升档请求计数 |
| 并发升档竞态窗口（两 attach 同过 pre-spawn 闸） | 注册点复检是唯一兜底；回收路径的信号/收割序列化直面 pgid 复用风险 | hubMu 临界区内计数 |
| 子进程退出/断开事件 → 客户端 wire 面 | 退出码与信号信息只许到达属主客户端（越权可见 = 信息泄露） | EXIT 帧（exit_code/信号语义） |
| 断开/子死双路并发 → teardown 序列 | 竞态下双重信号/双重收割/记账漂移 = 完整性事故面 | teardown 状态机 |
| UAT 脚本 → 真实 wesh 二进制（进程级断言面） | spawn 真实子进程与进程组信号探测——误伤面限于测试自身 spawn 的进程组 | 进程组信号探测（测试自管） |
| UAT 控制台输出 → 审计/日志面 | token/凭据/pid 泄漏进 CI 日志 = 红线事故 | 测试输出（状态码/布尔/形状白名单） |
| 收口证据面 → 零回归承诺 | 收口闸是「shared 逐字节不变」的最终证明载体——闸被削弱 = 承诺失效 | diff 审查证据（机器闸） |
| CI runner 运行态 → 人工确认门 | darwin 运行态为外部服务状态（本机结构性不可观测），经人工确认门承载——非代码攻击面 | CI 测试结果（只读核对） |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-11-01a | Elevation of Privilege | 预认证 spawn（绕过 ticket 闸的匿名 fork 面） | critical | mitigate | spawn 点锁定 checkTicket 核销后（server.go:1026-1030 升档分岔内、:1016 helloDone 核销后、hubMu 之外）；UAT S1a 启动期零子进程 pgrep 实证（VERIFICATION Truth #7） | closed |
| T-11-01b | Information Disclosure | spawn 失败文案携带路径/errno/系统调用细节 | high | mitigate | perclient.go:160 定值常量 "failed to start process"；`err.Error()` 拼接计数 0（L1 grep 复核 2026-09-04）；logEvent spawn_failed 四段 schema 零敏感值 | closed |
| T-11-01c | Tampering | kill-after-reap：pgid 复用窗口内 SIGHUP/SIGKILL 打中无关进程组 | high | mitigate | reaped 栅栏（perclient.go:85-89，hubMu 内标志位）覆盖 SIGHUP 与 AfterFunc 补 KILL 两发信号点；唯一收割者纪律；TestPerClientTeardownRaceOnce 10 轮竞态注入 PASS | closed |
| T-11-01d | Denial of Service | hubMu 横跨 spawn/Drain/Wait 阻塞冻结全控制面 | high | mitigate | Anti-Pattern 1 锁序：闸内读计数 → 放锁 spawn → 再取锁注册（perclient.go:139-151）；teardown 慢半段独立 goroutine（:349-357，Drain/Close/waitDone 均不占 hubMu）；全量 -race 5 包 ok | closed |
| T-11-01e | Repudiation | per-client 会话生命周期审计空白（无 session_start/session_end） | low | accept | 见 Accepted Risks Log AR-1 | closed |
| T-11-02a | Denial of Service | 重复 pid 注册影子化先注册者 → 该会话收割挂死（goroutine 泄漏 + EXIT 永不送达） | medium | mitigate | watch() fail-closed dup 检查（reap_darwin.go:30 errDupWatch、:55-60 w.mu 内检查）+ awaitExit 退化 cmd.Wait() 既有兜底（:132-134）；CI run 33832096581 macOS leg TestWatchDupPidFailClosed 实际运行 PASS（UAT 测试 1 证据） | closed |
| T-11-03a | Denial of Service | 并发 attach 竞态超编（两升档同过闸 → 瞬时进程数 > maxClients） | high | mitigate | D-03 注册点复检回收：perclient.go:140 pre-spawn 闸 + :184-189 同一 hubMu 持有内复检，超编者 SignalGroup(HUP)+Drain+Close+Wait 完整回收（:381-397 reapOrphanSession）；TestPerClientCapacityRecheckRace「恰一胜一负 + 终态==1 + 败者 ESRCH」PASS | closed |
| T-11-03b | Information Disclosure | 容量/失败拒绝文案或事件携带内部状态（路径/errno/argv） | high | mitigate | 文案定值常量（perclient.go:100 "server is at capacity" / :160 "failed to start process"）；事件四段 schema 零敏感值；TestPerClientSpawnFailure 断言事件不含注入错误文本与 argv 路径 | closed |
| T-11-03c | Tampering | 回收路径 kill-after-reap（AfterFunc 补 KILL 打中复用 pgid） | high | mitigate | reapOrphanSession 局部 reaped 原子闸（Wait 返回置位后才放行 KILL 豁免，perclient.go:381-397）——Pitfall 2 同构；唯一收割者纪律 | closed |
| T-11-04a | Information Disclosure | EXIT 帧（含 exit_code/信号语义）串台到非属主客户端 | high | mitigate | sessionWatcher 直写序（perclient.go:291-314，组帧一次→Write 2s ctx→Close 1000，禁 outbox 广播）；TestPerClientExitPrivate42 他端静默窗零帧断言 + phase11.mjs S5 协议层对照（21/21 PASS） | closed |
| T-11-04b | Tampering | teardown 双路并发 → 双重 SIGHUP（第二发打中 reap 后复用 pgid）/ 双 close panic / 记账漂移 | high | mitigate | teardownOnce + reaped 栅栏 + 慢半段单 goroutine（11-01 机制）；TestPerClientTeardownRaceOnce 10 轮竞态注入（quiescent 四件套 + exitf 零调用 + 零 panic）+ 全量 -race | closed |
| T-11-04c | Denial of Service | HUP 免疫子进程断开后永生（nohup/trap 泄漏 ×N——Pitfall 8） | high | mitigate | D-01 KILL 兜底分支（stopTimeout>0 时 AfterFunc 补 SIGKILL 经 reaped 闸）；TestPerClientStopTimeoutKillFallback 时序双断言 PASS；stop-timeout=0 默认值语义不变 | closed |
| T-11-05a | Information Disclosure | UAT detail/控制台打印 token/凭据/pid | medium | mitigate | phase06.mjs:11-13 红线逐字沿用（detail 只打印状态码/布尔/形状/退出码/文案常量）；phase11.mjs 21/21 PASS 输出零敏感值 | closed |
| T-11-05b | Denial of Service | trap 免疫滞留进程组随脚本泄漏（CI 级联减速实证） | medium | mitigate | S6/S8 场景尾部显式 SIGKILL 清场 + startWesh 子进程追踪收口；VERIFICATION 跑后 ps 排查零滞留实证 | closed |
| T-11-05c | Tampering | UAT 断言放宽（「503 或 1011 都接受」形态使 D-02 wire 锁定失效） | high | mitigate | S6 linger 注入使 1011 面确定性可达；两关闭面各自逐字断言（"server is at capacity" + close 1011）；acceptance 末条负向复核 | closed |
| T-11-06a | Tampering | 收口阶段为转绿而改动既有期望值/既有 UAT 脚本（零回归证据证伪） | high | mitigate | 11-06 零代码改动（files_modified 空集）；diff 四件套审查（删除文件 0、新增恰 3、修改恰 6、红线路径零出现、白名单文件删除行==0）机器闸 + 逐条人工复核（VERIFICATION Truth #11） | closed |
| T-11-06b | Denial of Service | UAT 连跑资源滞留（各脚本子进程未收口叠加） | low | mitigate | 各脚本自带追踪收口 + 11-05 prohibitions 清场纪律；收口顺序串行执行 | closed |
| T-11-07-01 | Tampering | waitPgroupESRCH 断言强度（EPERM 容忍被顺手扩大为检测弱化） | high | mitigate | 容忍严格限定「存在形态」归类：护栏到期未 ESRCH 仍 Fatal + 其余错误立即 Fatal，两半边由 TestWaitPgroupESRCHProbeSemantics 四子测反向锁定（持续 EPERM 翻车 / 他错立即翻车两负例，2026-09-04 T1 复核 4/4 PASS）；prohibitions 明令禁止 t.Skip/删调用点/放大护栏 | closed |
| T-11-07-02 | Denial of Service | （理论）持续 EPERM 误报致护栏翻车误报 | low | accept | 见 Accepted Risks Log AR-2 | closed |
| T-11-SC（11-01…11-06 各 plan 重复登记，同一处置合并） | Tampering | 依赖供应链（go get/包安装） | high | accept | 见 Accepted Risks Log AR-3 | closed |
| T-11-07-SC | Tampering | npm/pip/cargo 安装 | — | not applicable | 本 plan 零包安装（go.mod/go.sum 一行不动）——Package Legitimacy Gate 不触发 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-1 | T-11-01e | per-client 会话生命周期审计事件（session_start/session_end）空白——D-04 明示接受的 11→13 窗口期：Phase 13 以 per-client 粒度一次补齐（client_id 关联键）。本阶段 spawn_failed/max_clients 两事件先行；TestPerClientNoSessionStartEvent（perclient_test.go:366）显式锁定空白语义防漂移 | GSD secure-phase (L1) | 2026-09-04 |
| AR-2 | T-11-07-02 | 持续 EPERM 意味着进程组真实存在（POSIX 语义权限拒绝 ≠ 消失）——护栏翻车是该语义下的正确行为（僵尸/卡死检测保留）；macOS 瞬态 µs-ms 级，2s/5s 护栏余量充足（CI 复跑确认门兜底异常形态） | GSD secure-phase (L1) | 2026-09-04 |
| AR-3 | T-11-SC | 零新依赖策略下供应链威胁以 accept 处置：Phase 11 全部机制由既有六依赖覆盖（11-01 研究 STACK.md 锁定），go.mod/go.sum 零漂移经 11-06 diff 白名单审查机器闸实证；UAT 脚本 Node 原生 WebSocket/fetch 零三方包。后续引入新依赖时需重新评估本条 | GSD secure-phase (L1) | 2026-09-04 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-04 | 21 | 21 | 0 | gsd secure-phase orchestrator（L1 grep 深度；short-circuit：register_authored_at_plan_time=true + asvs_level=1 + threats_open=0） |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-04
