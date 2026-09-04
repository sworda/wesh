# Phase 11: per-client 生命周期主干 - Context

**Gathered:** 2026-09-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 11 交付 per-client 模式的核心 E2E 最长链：attach 升档即 spawn 独立 PTY 子进程（Hello 上报尺寸经钳制后作初始 winsize，无 80×24 中间态闪烁）→ 客户端断开（正常关闭或异常 1006）其进程组立即收 SIGHUP（随 --stop-signal 可配，无宽限、无僵尸）→ 子进程退出仅该客户端收私有 EXIT 帧（含 exit_code，信号死亡 -1）并以 1000 关闭。其生死只影响自己：spawn 失败该客户端收类型化 Error 帧 + 1011，服务端与其他在线客户端不受影响。

**In scope (from ROADMAP):** PC-02/PC-03/PC-04——client.inQ/pc 两字段；升档 per-client 分支（容量再闸 → hubMu 外 spawn → 失败 Error+1011 → Welcome 回显 → 注册+登记）；五 goroutine 装配（ReadLoop 闭包 / inputWriter 参数化 / writer / pinger / sessionWatcher）；EXIT 私有化直写；detach/kick SIGHUP 挂点（注册表移除点覆盖一切断开形态）；每会话 teardown sync.Once 固定序列 + reaped 栅栏；darwin watcher dup-watch fail-closed 防御。

**Out of scope (本阶段不做):** RESIZE 直通 case / ro 门控断言 / 前端重连 terminal.reset() / per-PTY 停读续读（Phase 12——INPUT case 换 cl.inQ 一行切换属本阶段 inQ 字段装配的必然结果，RESIZE 仲裁路径在 arbiter 未装配下天然 no-op 即可）；spawn 双令牌桶 / stop-timeout 默认值重议（裁决项①）/ Shutdown N 进程组快照逐组信号（Phase 13）；--once/--exit-when-empty 第二终结源（pcSupervisor/pcExitReq）/ metrics 与审计 per-client 粒度（session_start/end 的 client_id 关联键、spawn_failures_total counter、healthz 语义）/ WESH_REMOTE_USER 注入（Phase 13）；参数化测试 harness 与三维归类表（Phase 14）；前端任何改动（本里程碑唯一前端改动面在 Phase 12）。

**已锁定不重复决策（继承，下游直接执行）：** 架构形态「装配期一次分岔、运行期零分岔」不抽象 session 接口（6-7 显式分支点，每处 ≤10 行）；spawn 点 = ticket 核销之后 / Welcome 组帧之前 / hubMu 之外（SEC-08 预认证零资源 + Anti-Pattern 1）；spawn 失败 wire 面 = Error{server_error} + 1011（PC-02/ROADMAP SC2 已锁，复用 proto.ErrServerError 已备未启用常量，协议零改动）；失败文案绝不拼 err.Error()（启动面红线延伸）；Welcome 恒首帧 / exitf 恰好一次（termOnce 单点）/ 唯一收割者（cmd.Wait）三大不变量保持；EXIT 直写纪律（组帧一次 → 同步 Write 2s ctx → Close(1000)，禁 outbox 异步，Anti-Pattern 6）；CR-01 纪律（读循环零同步 Master.Write，每会话 inputQ+inputWriter，Anti-Pattern 5）；零新依赖；每阶段收口闸 = shared 全量测试原样绿 + 期望值逐字未动（禁止断言放宽成「两模式都接受」）。

</domain>

<decisions>
## Implementation Decisions

### KILL 兜底装配时机（STATE 裁决项①的边界划分，本讨论闭合机制侧）
- **D-01:** 每会话 teardown 固定序列**本阶段一次定型含 KILL 兜底分支**：SIGHUP（经 reaped 栅栏）→ `stopTimeout > 0` 则 AfterFunc 补 SIGKILL → Drain(200ms) → Close(master) → Wait 返回 → 注册表/容量记账单点移除（Pitfall 3 序列 + Pitfall 2 栅栏）。`--stop-timeout` 默认值 0 语义不变（不补 KILL），per-client 默认值重议（公开契约变更）仍归 Phase 13 裁决项①——本阶段只锁机制不锁默认值。机制先行的理据：teardown 序列是 Pitfall 3「恰好一次」的载体，二期再开序列口比重装机制更危险；用户显式 `--stop-timeout=5s` 时 per-client 立即获得正确行为（HUP 免疫进程有界死亡）；严格切片会在 Phase 11→13 窗口期留 `trap '' HUP` 泄漏（pcSessions 持续登记至自然死亡，Shutdown 暂不能覆盖残留者） — **Reversibility:** costly — teardown 序列被 Go 测试（断开/子死竞态注入）与 phase11.mjs S8（HUP 免疫 KILL 兜底）双重锁定后，拆除 KILL 分支需同步改写两 face 断言

### 容量再闸（maxClients 进程帽第三道闸）
- **D-02:** 满员拒绝 wire 形态 = **Error{server_error, "server is at capacity…" 容量文案} + Close(1011)**。否决 1013"max_clients"：前端 1013 分派只认 code 不渲染 reason（main.ts:946），固定显示「慢消费者背压踢出」文案（"could not keep up with the session output. The session itself is unaffected"）——per-client 满员场景语义双重错位，而前端改动窗口在 Phase 12；否决 1008：既有语义为认证/版本策略违反（version_mismatch/auth_failed），容量策略混入污染 1008 受众分治。1011 分派「Error 帧 message 优先展示」（main.ts:939-944，D-07）使容量文案准确可达；与 spawn 失败同码同机器串（协议零改动红线保持），服务端侧以 logEvent 事件名区分（容量=max_clients 语义事件名 / 失败=spawn_failed，D-04） — **Reversibility:** one-way — wire 行为被 phase11.mjs S6 与 Go 测试锁定；且 per-client 用户将对 1011+容量文案形成依赖，改码即行为破坏
- **D-03:** 闸后竞态窗口**本阶段装注册点复检+回收**：spawn 成功后注册点 hubMu 内复检 `len(pcSessions) >= maxClients`，超编者 SignalGroup(HUP)+Drain 回收（研究 §5 规则 1 建议形态，≤5 行）——「并发子进程数 ≤ maxClients」硬不变量 Phase 11 即成立。效果：Phase 13 裁决项④（spawn-intent 预占/回滚 vs 超编 ≤8 登记接受）**提前消解**——Phase 13 不再需要容量记账口径裁决，spawn 令牌桶（churn 限速）与 Shutdown N 组仍为 Phase 13 本体

### 观测面最小钩子（Phase 13 粒度的边界划分）
- **D-04:** 仅 **spawn_failed 单行审计事件**本阶段先行（`logEvent(remote, 1011, "spawn_failed", remoteUser)` 形态，零敏感值——Pitfall 5 失败清理清单的测试锁定项）；容量再闸拒绝同通道以区分事件名记录（D-02）。`wesh_pty_spawn_failures_total` counter 归 Phase 13——metricsSeries17 镜像契约保持 Phase 13 一次性扩展（17→N），本阶段 metrics.go 零改动；session_start/session_end 的 per-client 粒度（client_id 关联键）归 Phase 13——Phase 11→13 窗口期 per-client 会话生命周期审计为空白（已知且接受：会话事件对在 Phase 13 一次补齐，避免两半套 schema）

### 验证面切片
- **D-05:** Go 测试 = **新增 `internal/server/perclient_test.go` per-client-only 独立文件**（研究 §11 文件清单形态）；既有 shared 测试零改动原样跑（零回归证据不动）；`newTestServer(t, mode)` 参数化 harness 与 Pitfall 11 三维归类表归 Phase 14 统一收编——本阶段不碰任何既有测试文件装配点
- **D-06:** 新建 **`web/uat/phase11.mjs` 协议层 UAT 全链八场景**（phase02-09.mjs 先例：Node 原生 WebSocket/fetch 零依赖，spawn 真实二进制断言；兑现 10-CONTEXT deferred 项「per-client 真实协议行为 UAT 随 Phase 11+ 建设」）：①双端 attach 各得独立 pid、输出互不串台 ②首帧 winsize = Hello 钳制尺寸（spawn 后 stty size 断言，无 80×24 中间态）③spawn 失败 Error 通用文案 + 1011（运行期删命令注入——Phase 10 SC4 LookPath 预检只覆盖启动期，运行期删除经 exec 失败路径触发）④正常关闭与 1006 异常两形态断开 → pgid ESRCH 无僵尸 ⑤exit 42 / 信号死亡 → 仅本端 EXIT（exit_code 42 / -1）+ 1000，他端逐字节无扰动 ⑥`--max-clients=1` 容量再闸 → 第二客户端 1011 + 容量文案 ⑦断开后重连 = 全新进程新 pid（服务端语义本阶段成立；前端 reset 归 Phase 12 不在此断言）⑧`trap '' HUP` 命令 + `--stop-timeout=1s` → KILL 兜底后 pgid ESRCH（D-01 机制先行裁决的端到端证据）

### Claude's Discretion
- `pcSession` 结构精确字段集与 `pcSessions` 登记/移除的 hubMu 临界区形态（研究 §2.1 参考实现）
- 升档 per-client 分支的精确落点与行序（容量再闸 → spawn → 失败帧 → Welcome 回显 → 注册+登记——研究 §3.1 时序图）
- ReadLoop 闭包的 detach 门形态（`select <-cl.done` 提前 return，研究 §3.5）与 P5-1 别名红线保持（每帧 make+copy）
- inputWriter 参数化签名形态（研究 Pattern 3：1 份代码 N 实例）
- sessionWatcher 内 EXIT 私有化直写的精确 ctx/关闭序列（研究 §4.1 参考实现；Drain(200ms) 与 close(inputDone) 次序）
- reaped 栅栏的锁归属（每会话状态锁 vs hubMu 内标志位——Pitfall 2 只锁「信号与 reap 序列化」语义，实现选型自由）
- detach/kick 两处 SIGHUP 挂点的精确插入行（removeLocked 返回 true 之后，与 maybeExitWhenEmptyLocked 同位——Anti-Pattern 7）
- darwin watcher dup-watch fail-closed 防御的精确形态（重复 pid 注册返回错误 → 调用方退化 cmd.Wait() 兜底，Pitfall 9）
- 容量再闸/复检回收的容量文案精确措辞（server_error 机器串 + 人类可读 message，零路径/errno）
- spawn_failed 事件的字段集（remote/code/reason/remoteUser 四段既有 schema 内）
- perclient_test.go 内部测试拆分与 export_test.go 暴露面
- phase11.mjs 八场景的脚本内编号与断言颗粒度

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 11 — 成功准则 4 条（双 pid 独立+首帧 winsize / spawn 失败 1011 / 断开 SIGHUP 无宽限无僵尸 / EXIT 私有化）与「含」清单（本 phase 全部交付物枚举）
- `.planning/REQUIREMENTS.md` §PC-02/PC-03/PC-04 — 需求原文（:81-83）
- `.planning/PROJECT.md` §Current Milestone v1.1 — 里程碑目标与架构形态锁定（装配期一次分岔、运行期零分岔，不抽象 session 接口，共享面 ≥90%）
- `.planning/STATE.md` §Blockers — v1.1 裁决项归属（① stop-timeout 默认值 → Phase 13；③ healthz/metrics OQ → Phase 13；④ spawn-intent 口径 → 本 phase D-03 提前消解）

### v1.1 调研结论（2026-09-01/02，HIGH 置信）
- `.planning/research/ARCHITECTURE.md` §2（Server 结构形态/pcSession/client 增量）§3（Attach 时序/容量再闸/INPUT/RESIZE/输出闭包）§4（sessionWatcher/pcSupervisor/断开 SIGHUP 挂点）§5（锁序三规则/goroutine 拓扑/终结恰好一次）§6（StartWithSize）§9（Anti-Pattern 1/2/3/5/6/7 红线）§11（新增 vs 修改文件清单）——本 phase 的主设计文档
- `.planning/research/PITFALLS.md` P2（kill-after-reap 误杀/reaped 栅栏）P3（teardown 恰好一次固定序列）P5（spawn 失败四连坑/清理清单）P9（darwin dup-watch fail-closed）+ Technical Debt Patterns 表 + Security Mistakes 表（EXIT 广播习惯禁带过来）
- `.planning/research/SUMMARY.md` §Phase 2（本 phase 交付物枚举）+ 方法论警告（最大风险 = 破坏既有不变量而不自知；禁止断言放宽）
- `.planning/research/FEATURES.md` §T2/T3/T4（attach spawn/断开 SIGHUP/EXIT 私有化的 ttyd 语义锚定）+ Anti-Features A1/A2/A3/A6/A7

### 前序 phase 决策（机制先例）
- `.planning/phases/10-mode-assembly/10-CONTEXT.md` — Phase 10 接缝交付物（SessionMode/SpawnFunc/ValidateOptions/StartWithSize）与收口闸口径（shared 全量原样绿 + 期望值逐字未动）；D-06 零新 UAT 的边界注记（per-client UAT 归本 phase 起建）
- `.planning/milestones/v1.0-phases/06-lifecycle/06-CONTEXT.md` — EXIT 帧直写纪律（组帧一次/同步 Write 2s/Close 1000/禁 outbox 异步）与 accept-255 裁决（OQ1；per-client --once 对齐在 Phase 13，本 phase 不涉及）
- `.planning/milestones/v1.0-phases/07-deployment/07-CONTEXT.md` — stop-signal/stop-timeout 序列机制（OPS-04，SignalGroup + AfterFunc KILL 既有形态——D-01 每会话化的母本）

### 现状代码（扩展点，file:line 实证）
- `internal/server/server.go` — Options.SessionMode/SpawnFunc（:301-309，10-01 已装配）/ ValidateOptions 互斥校验（:337-345）/ New 零值兜底（:392-393）与字段直传（:445-446）/ Attach 升档序列（:807 起；守卫区⓪-③ :749-798；checkTicket :1159；Welcome 恒首帧论证 :961-1043；registerLocked :1024）/ pinger（:1211）/ lifecycle（:1359，per-client 不装配）/ terminate/termOnce（:1446，supervisor 归 Phase 13 复用）/ Shutdown（:1479，本阶段不分支）
- `internal/server/clients.go` — client 结构（:81-146，+inQ/pc 两字段）/ outbox trySend（:178）/ registerLocked/removeLocked（:311/:329，记账单点）/ kickSlowConsumerLocked（:564-606，1013 既有机械）/ detach（:761 附近，SIGHUP 挂点同位）/ maybeExitWhenEmptyLocked（:852，本阶段不分支——Phase 13）/ stopChildLocked（:893，每会话化母本）/ inputQ（defaultInputQueueBytes 容量复用）
- `internal/server/server.go:1036-1064` — INPUT case（s.inputQ.tryEnqueue → cl.inQ.tryEnqueue 一行切换）
- `internal/pty/spawn.go` — StartWithSize（:76，10-01 已导出）/ Start 委托（:68）/ SpawnCols-SpawnRows 80×24 单一事实源（:38-41；per-client 改为 Hello ClampDim 钳制值）
- `internal/pty/io.go` — ReadLoop 缓冲复用（:14，P5-1 别名红线）/ Resize fdMu 纪律 / Drain(200ms) / SignalGroup（不取 fdMu，hubMu 内安全）
- `internal/pty/reap_linux.go` / `reap_darwin.go` — 唯一收割者纪律（D-14）；darwin 共享 watcher dup-watch 防御点（Pitfall 9）
- `internal/proto/proto.go` — ErrServerError（:61，已备未启用）/ 关闭码纪律块（:8-11）/ ExitFrame/ExitPayload（:189-192）——零改动红线
- `web/src/main.ts:925-955` — 前端关闭码分派表现状（1013 只认 code 固定慢消费者文案 / 1011·1008 Error message 优先展示——D-02 裁决依据；本阶段前端零改动）
- `web/uat/phase02-09.mjs` — 协议 UAT 先例形态（phase11.mjs 母本；既有脚本默认模式原样重跑 = 零回归脚本级证据）
- `docs/ARCHITECTURE.md` §2.9 等 — v1.0 落地架构对照（双模式文档段归 Phase 14，本阶段不动）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pty.StartWithSize`（spawn.go:76，Phase 10 已导出）— per-client attach 期 spawn 的消费点：Hello cols/rows 经 ClampDim 钳制后直通，出生即正确尺寸，免 80×24 首帧窗口与 SignalForegroundGroup 强制重绘
- `outbox + writer + pinger 三件套`（clients.go:151-193/701-724，server.go:1211）— 逐字复用：per-client 客户端仍是 registry 成员，P5-1 别名红线/mergeBatch/1013 帧可达性不变量全部自然继承
- `kickSlowConsumerLocked`（clients.go:564-606）— arbiter 空集时 removeMember/recalcNow 天然 no-op，1013 踢出机械零改动复用（慢客户端 outbox 满 → 1013 的本阶段默认行为；停读续读层 Phase 12 再加）
- `输入限速器 + ro mode 门`（server.go:1036-1056）— 原样：per-client 单行门 `writable && ticketMode==rw → rw else ro`（decideModeLocked 不调用），ro 丢 INPUT 与 AllowN 限速自然继承（PC-06 服务端语义本阶段免费成立，断言归 Phase 12）
- `stopChildLocked 的 SignalGroup + AfterFunc KILL 形态`（clients.go:893-898）— D-01 每会话化的直接母本；SignalGroup 不取 fdMu，hubMu 内发送安全（锁序不变）
- `proto.ExitFrame/exitMessage/exitCodeOf`（proto.go:189-192，exitmsg.go）— EXIT 私有化组帧/文案/退出码提取逐字复用（三形态文案与信号死亡 -1 语义不变）
- `logEvent/emitEvent`（log.go）— D-04 spawn_failed 事件的既有通道；remote 字段 sanitize 与红线纪律不变
- `darwin 共享 kqueue exit watcher`（reap_darwin.go:24-119）— 包级单例设计上即 N 会话共用；awaitExit 每会话一 goroutine 形态直接消费，仅需 dup-watch fail-closed 防御（Pitfall 9）

### Established Patterns
- **Welcome 恒首帧时序纪律**（P2 D-02 同构）— Welcome 入队先于 registerLocked、ReadLoop goroutine 注册后启动；spawn 到 ReadLoop 启动间的子进程输出由 64KiB 内核缓冲承接——per-client 下同构成立
- **唯一收割者纪律**（D-14）— cmd.Wait() 仅 sessionWatcher 调用；断开路径绝不自己 Wait（经会话 done 等待收割完成）；Linux pidfd 每进程独立 N 规模天然成立
- **R-06/R-07 记账与锁序**— registry.n 加减排他对称（registerLocked/removeLocked 单点）；锁序 hubMu > outbox.mu、hubMu > sess.fdMu 全序保持，无新锁类型；hubMu 绝不横跨 spawn（Anti-Pattern 1）
- **CR-01 读循环零同步写**— cl.inQ 间接字段使 INPUT case 逐行不分支（Pattern 2）；每会话 inputWriter 参数化（Pattern 3）保持「读循环永不直写 master」
- **EXIT 直写纪律**（Phase 6 裁决）— 组帧一次共享只读 + 同步 Write(2s ctx) + Close(1000)；禁 outbox 异步入队（关闭帧超车防线）；watcher 的 Close(1000) 与 Shutdown 的 Close(1001) 竞态由库 Close 幂等承接
- **零回归双证据口径**（Phase 10 D-06 延伸）— 每阶段收口闸 = shared 全量 Go 测试原样绿 + phase02-09.mjs 默认模式零修改重跑 + 期望值逐字未动 diff 审查；禁止断言放宽成「两模式都接受」
- **生产直传 + New 零值兜底分档**— SessionMode 零值 = shared 已在 Phase 10 落地；per-client 新增运行期字段（pcSessions 等）全部 hubMu 保护（pinger pongTimedOut 置位/读取先例同形态）

### Integration Points
- `server.go Attach 升档序列`（ticket 核销后 / close(helloDone)+release() 后 / Welcome 组帧前）— per-client 分支插入点：容量再闸（hubMu 读 len(pcSessions)）→ spawnFn（hubMu 外）→ 失败 Error+1011+logEvent spawn_failed → 成功构造 client{inQ: pc.inQ, pc} → Welcome{mode,cols,rows}（本端 Hello 钳制尺寸回显，G-05-1 契约自然满足）→ registerLocked + pcSessions 登记 + 复检回收（D-03）
- `clients.go client 结构`（:81-146）— +inQ/pc 两字段（写一次于升档，happens-before 由 goroutine 启动 + hubMu 建立，remote/remoteUser plain 字段先例同形态）
- `server.go INPUT case`（:1036）— s.inputQ.tryEnqueue → cl.inQ.tryEnqueue 一行切换
- `clients.go detach / kickSlowConsumerLocked`（:761 / :564）— removeLocked 返回 true 之后的 per-client SIGHUP 挂点（D-01 teardown 序列触发端；两移除点覆盖一切断开形态，含 pinger 超时与 Shutdown 1001 广播引发的 detach）
- `server.go New 尾部 goroutine 钉死点`（:443-446 附近）— 模式分岔：shared 现状三件套逐字不动；per-client 本阶段零全局 goroutine（pcSupervisor 归 Phase 13），五 goroutine 全部随 attach 每会话装配
- `perclient.go（新增）`— pcSession 结构、升档分支、ReadLoop 闭包、sessionWatcher、teardown Once 序列 + reaped 栅栏、每会话 stop-signal helper（研究 §11 文件清单）
- `reap_darwin.go watch()` — dup-watch fail-closed 防御（重复 pid 注册返回错误，调用方退化 cmd.Wait() 兜底）
- `web/uat/phase11.mjs（新增）`— D-06 八场景协议 UAT

</code_context>

<specifics>
## Specific Ideas

- **「1013 文案错位」是 D-02 的关键裁决依据**——前端 1013 分派只认 ev.code 不渲染 reason（main.ts:946 注释明示「slow_consumer 是机器串，渲染远端内容是伪造钓鱼面」），固定文案含 "The session itself is unaffected"——per-client 满员场景这句语义直接错误。在前端改动窗口（Phase 12）之前，容量拒绝必须走 message 可渲染的 1011 通道
- **容量再闸与 spawn 失败同码同串是有意为之**——D-02 选择不在协议面区分两者（协议零改动红线 > 关闭码分辨率），分辨率由服务端日志事件名承担（SEC-07「日志归因 vs metrics 聚合」分工先例的同构：wire 面聚合、日志面细分）
- **D-03 复检回收使硬不变量提前成立的连锁效果**——Phase 13 规划时裁决项④应直接从 STATE.md Blockers 移除（不再是开放项）；Phase 13 本体收窄为 spawn 令牌桶 + stop-timeout 默认值重议 + Shutdown N 组
- **Phase 11→14 窗口期的两处已知语义缺口（明示接受，非疏漏）**：per-client 下 --once/--exit-when-empty 永不退出（Pitfall 1，第二终结源归 Phase 13——本阶段 maybeExitWhenEmptyLocked 不分支，per-client 下该路径显式注释锚定）；per-client 会话无 session_start/session_end 审计（D-04，Phase 13 一次补齐）
- **UAT S3 的 spawn 失败注入手法**——运行期删命令（启动期 LookPath 预检通过后、attach 前 unlink argv[0]），触发 exec 失败路径；这是 Phase 10 SC4 预检「启动期暴露」与运行期「per-request degrade」哲学分界的直接实证（Pitfall 5b）

</specifics>

<deferred>
## Deferred Ideas

- **wesh_pty_spawn_failures_total counter** — D-04 裁决归 Phase 13（metricsSeries17 镜像一次性扩展）；EMFILE 级联的 Prometheus 可见性随 Phase 13 落地
- **per-client 会话 session_start/session_end 审计（client_id 关联键）** — Phase 13 本体（D-04 窗口期空白已明示接受）
- **per-client stop-timeout 默认值重议**（裁决项①）— Phase 13：0=不补 KILL 在 per-client = HUP 免疫泄漏，公开契约变更需用户裁决；D-01 机制已就位，届时仅改默认值
- **pcSupervisor / 第二终结源 / --once·exit-when-empty per-client 语义** — Phase 13（Pitfall 1）；本阶段 maybeExitWhenEmptyLocked 不分支
- **newTestServer(t, mode) 参数化 harness 与三维归类表** — Phase 14（Pitfall 11）；perclient_test.go 届时按归类表收编
- **RESIZE 直通 / ro 断言 / 重连 reset / 停读续读** — Phase 12（PC-05/06/07/10/11）；本阶段 RESIZE 在 per-client 下经零值 arbiter 天然 no-op（静默无效为已知中间态）
- **spawn 双令牌桶 / Shutdown N 进程组快照** — Phase 13（PC-08）；本阶段 D-03 硬帽 + 断开 SIGHUP 已覆盖泄漏主面

</deferred>

---

*Phase: 11-per-client*
*Context gathered: 2026-09-03*
