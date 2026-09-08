# Phase 12: per-client 交互与背压语义 - Context

**Gathered:** 2026-09-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 12 交付 per-client 模式下四类交互语义各归各会话：① RESIZE 直通本会话 TIOCSWINSZ（[1,1000] 钳制与 50ms 防抖保留，无仲裁器、无 'W' 约束帧）；② ro 客户端独立进程照常 spawn，INPUT 服务端丢弃 + 限速保留（PC-07 服务端语义 Phase 11 已免费成立，本 phase 断言锁定）；③ 异常断线（1006）重连 = 全新进程，前端按 Welcome 模式位执行 terminal.reset() 清屏；④ 慢客户端先停读其 PTY（内核缓冲积压、子进程写阻塞不丢数据），恢复自动续读，持续过载 dwell 到期 1013 踢出。

**In scope (from ROADMAP):** PC-05/PC-06/PC-07/PC-10/PC-11——INPUT 零分支 / RESIZE 直通两 case（rw + ro 放行）；每会话 resize 防抖（共用 debouncer 组件防双写漂移）；前端重连分支按 Welcome 模式位 terminal.reset() + dist 重建（本里程碑唯一前端改动面）；per-PTY 停读/续读状态机（阻塞持帧形态）+ dwell 看门狗；WelcomePayload 加 session 字段（本 phase 唯一协议面变更，additive）。

**Out of scope (本阶段不做):** spawn 双令牌桶 / stop-timeout 默认值重议 / Shutdown N 进程组快照逐组信号 / --once·exit-when-empty 第二终结源（pcSupervisor/pcExitReq）/ metrics 与审计 per-client 粒度（session_start/end client_id 关联键、spawn_failures_total counter、healthz 语义）/ WESH_REMOTE_USER 注入（以上 Phase 13）；参数化测试 harness 与三维归类表 / 模式语义文档 PC-12 / herdr 场景 E2E UAT PC-13（以上 Phase 14）；Playwright 浏览器层实证（归 Phase 14 herdr 全链，本 phase 前端改动 jsdom+协议层锁定）。

**已锁定不重复决策（继承，下游直接执行）：** 装配期一次分岔运行期零分岔、不抽象 session 接口、零新依赖；INPUT case `cl.inQ.tryEnqueue` 一行切换 Phase 11 已落地（ro 丢 INPUT + AllowN 限速 + inputDrops 计数自然继承）；RESIZE 仲裁/owner 递补/fan-out/信用门在 per-client 分支不装配（arbiter 零值天然 no-op），'W' 帧不发（Welcome cols/rows 即自有尺寸，G-05-1）；重连=新进程服务端语义 Phase 11 已锁（phase11.mjs S7 双 pid）；1013 踢出机械 `kickSlowConsumerLocked` 零改动复用（arbiter 空集时 removeMember/recalcNow 天然 no-op）；metrics.go 零改动红线（17→N 镜像扩展归 Phase 13）；session_start/session_end 审计空白 Phase 11→13 窗口期已明示接受；每会话 50ms 防抖保留、共用 debouncer 组件（ROADMAP「含」）；ro 恒 `[ro] ` 标题前缀不变（FEATURES Keep 清单）；收口闸 = shared 全量 Go 测试原样绿 + phase02-11.mjs 默认模式零修改重跑 + 期望值逐字未动，禁止断言放宽成「两模式都接受」。

</domain>

<decisions>
## Implementation Decisions

### 背压踢出判据与停读机制（PC-10/PC-11，SC4/SC5 的 HOW）
- **D-01:** 停读形态 = **ReadLoop 闭包阻塞持帧**——trySend 失败 → 闭包 `select` 等待 outbox 恢复信号 / `cl.done` 逃逸，恢复后重试同一帧；读循环自然停摆 → 内核缓冲积压 → 子进程写阻塞（ttyd pty_pause 等效）。否决显式 pause/resume 状态机（pcSession paused 字段 + cond 的唯一消费方是 metrics 观测，归 Phase 13，本 phase 无消费方）；否决 outbox 上层中间缓冲（与「内核缓冲即缓冲」哲学冲突）。单消费者可阻塞是 per-client 独有自由度——shared fan-out 不能阻塞读循环（creditPending 形态的存在理由），per-client 帧就在闭包栈上即「暂存」 — **Reversibility:** costly — 机制被停读/续读/dwell 三层 Go 测试 + phase12.mjs 端到端锁定后，改形态需同步重写两 face 断言
- **D-02:** 踢出判据 = **dwell 计时器**——停读态连续无恢复 > T 秒 → 1013；每次续读重置计时。慢但在前进的客户端永不被踢；dwell 起止两点确定性可断言（SC5 可测）；单消费者无误判面（shared dwell 前科不适用）。否决无看门狗（浏览器协议栈自动回 pong，pinger 探测不到慢端，SC5「持续过载 1013」永不达成，需修 ROADMAP）；否决写超时（改 writer 语义，慢网络正常消费客户端误伤面）
- **D-03:** dwell 阈值 = **10s 内部常量**（`defaultSlowDwell` 类）+ **测试可覆写**（outboxBytes 先例）；不暴露 CLI flag/TOML 键——零调优需求，避免公开契约面膨胀。dwell 期间服务端资源占用确定（1 outbox + 1 闭包），无资源风险
- **D-04:** WR-01 闭合形态 = **dwell 涵盖，宽限门与 creditPending 均不复刻**——dwell 10s 从停读起点武装，结构性涵盖 500ms attach 宽限与一切瞬态；阻塞持帧即暂存（帧在闭包栈上），creditPending/afterDrain 重投是 shared fan-out 不能阻塞的特有形态，per-client 复刻即死代码面。WR-01（Phase 11 review 遗留，STATE.md:99）登记按此裁决形态闭合，规划期在 plan 中显式回指 — **Reversibility:** costly — 若实证推翻（瞬态满箱误踢案例），需补宽限门并回写 WR-01 闭合记录
- **D-05:** 观测口径 = **停读/续读两点递增 `registry.gateTransitions`**（与两模式 1013 kicks 同构 mode-agnostic 聚合）；metrics.go 零改动——既有计数器多两个递增点，非新增 series，不违 Phase 11 D-04（17→N 镜像扩展归 Phase 13）

### ro 端 RESIZE 语义（PC-05 的 ro case）
- **D-06:** 服务端第二闸 = **per-client 分支 ro RESIZE 放行直通自己 PTY**——D-09 第二闸（server.go:1183 `cl.mode.Load() == proto.ModeRO → continue`）在 per-client 下不生效，shared 逐字保留。ttyd parity（protocol.c 只门 INPUT 不门 RESIZE，FEATURES 裁决 6）；per-client 下 ro 独占自己进程、无旁观对象可保护，丢弃只是让 ro 端 PTY 尺寸恒停初始值；herdr 场景 ro 移动端转屏/拖窗后自身 area 渲染尺寸正确（PC-13 driving scenario 的前半） — **Reversibility:** costly — wire 行为被 Go 测试 + phase12.mjs 锁定，per-client ro 客户端将对 resize 生效形成依赖，回收即行为破坏
- **D-07:** 前端第一闸 = **同步放开**——per-client（按 Welcome 模式位）ro 端恢复 fit 变化时上报 RESIZE；shared 保持不发（05-08 逐字保留）。与 D-06 配套才生效——仅服务端放行则自家前端行为零变化，herdr ro 移动端场景问题依旧

### Welcome 模式位与前端 reset（PC-06）
- **D-08:** 模式位字段 = **`session: "shared"|"per-client"` 字符串枚举**——WelcomePayload 加字段（proto.go:111），与 CLI flag `--session-mode` 同词自描述；**恒序列化无 omitempty**（G-05-1 方向 A 先例，同 Cols/Rows）；旧服务端缺键 = shared（前端不 reset，行为零漂移，协议演化纪律）。否决 `per_client` 布尔（表达力弱、与 CLI flag 不同词需注释互指）。本 phase 唯一协议面变更，additive，其余帧/关闭码零改动 — **Reversibility:** one-way — 发布后前端按该键分支，删键/改词即破坏已发布契约（v1.1.0 起公开面）
- **D-09:** reset 时机 = **Welcome 分支内统一判断**：模式位=per-client → `terminal.reset()`。首连时屏幕本来为空，reset no-op 等价——代码零分支；shared 永不 reset（CORE-05 接回同进程，旧屏有效）。调用点只能在 Welcome 分支——onopen 时模式位未到手。per-connection 状态（isRO/welcomeDone 等）复位沿用 IN-01 既有 connect() 重置面，不新增
- **D-10:** 重连 reset = **静默无提示**——画面恢复交给子程序重绘/herdr attach（ttyd parity，FEATURES 裁决 3）；面板在 Welcome 后照常隐藏，无「新会话」文案（普通 shell 场景的理解成本可接受，herdr/tmux 场景提示即噪音）

### 验证面切片
- **D-11:** Go 测试 = **扩展 `internal/server/perclient_test.go`**——resize 直通两 case / ro INPUT 门控 / 停读续读 / dwell 踢出 8-12 新测同模式单一家；Phase 14 参数化 harness（newTestServer(t, mode)）收编时只动一个文件
- **D-12:** dwell 协议层验证 = **真实 10s+ 等待**——Go 测用测试覆写短 dwell（outboxBytes 先例）做确定性断言；phase12.mjs 等真实 dwell 到期做一次端到端 1013 证据（phase06 保活测等 11s+ 先例）；**零测试钩子进二进制**（否决 WESH_SLOW_DWELL 类 env 注入——隐藏调优面与 D-03 裁决精神冲突）
- **D-13:** 前端验证 = **jsdom + 协议层，Playwright 归 Phase 14**——jsdom 断言模式位→reset 调用 / ro RESIZE 发送恢复 / 旧服务端缺键不 reset（phase06-dom D1/D8 先例）；phase12.mjs 断言 Welcome.session 字段存在与取值；浏览器观感（清屏重绘）归 Phase 14 herdr 全链一并实证，本 phase 不建 phase12-pw（避免与 Phase 14 重复建设）

### Claude's Discretion
- 阻塞持帧的恢复信号精确形态（outbox drain 至非满时的通知通道——cap 1 信号量 vs cond，锁序保持 hubMu > outbox.mu 全序）
- dwell 计时器的武装/重置挂点（停读起点 AfterFunc vs 闭包内 Timer）与 1013 踢出路径复用 kickSlowConsumerLocked 的调用序列
- RESIZE 直通 case 的精确分支形态（§3.4 参考实现：cl.pc != nil → sess.Resize 仅取 fdMu 不持 hubMu；ro 放行后两 case 归一直通）
- 每会话 debouncer 的组件复用形态（从 arbiter 抽取共用件防双写漂移，ROADMAP「含」已锁方向）
- 前端模式位的存储形态（per-connection 变量，IN-01 登记口径）与 RESIZE 发送门的分支点
- phase12.mjs 场景集精确编号与断言颗粒度（建议面：resize 直通双端互不影响 / ro RESIZE 直通 stty 证据 / ro INPUT 丢弃+rw 限速 / Welcome.session 字段 / 停读期输出不丢恢复后完整到达 / dwell 到期 1013）
- jsdom 测试文件归属与 mock 形态（terminal.reset 调用计数断言）
- dist 重建与 embed 链（pnpm build 机械步骤）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 12 — 成功准则 5 条（resize 直通无仲裁 / ro 独立进程 INPUT 门控 / 重连新进程+前端 reset / 停读续读 ttyd parity / 持续过载 1013）与「含」清单（每会话防抖共用 debouncer、本里程碑唯一前端改动面）
- `.planning/REQUIREMENTS.md` §PC-05/PC-06/PC-07/PC-10/PC-11 — 需求原文（:84-90）
- `.planning/PROJECT.md` §Current Milestone v1.1 — 里程碑目标与架构形态锁定

### v1.1 调研结论
- `.planning/research/ARCHITECTURE.md` §3.3（INPUT 零分支）§3.4（RESIZE 直通参考实现 + 无 'W' 帧论证）§3.5（输出闭包现状 + 慢客户端策略 OQ 背景——kick-on-full 推荐已被 ROADMAP SC4/SC5 的停读续读裁决取代）§5（锁序三规则）§9（Anti-Pattern 红线）
- `.planning/research/FEATURES.md` §七项裁决 3（重连=新进程+前端 reset，ttyd index.ts:267 parity）§6（ro=自有进程输入门控，ttyd 只门 INPUT）+ keep/degrade/vanish 映射（vanish 清单=本 phase 不装配项的红线依据）
- `.planning/research/PITFALLS.md` P7（per-client 分支丢失防抖 → SIGWINCH 风暴回潮——每会话 debouncer 共用的动机）+ Integration Gotchas / Performance Traps

### 前序 phase 决策（机制先例与边界）
- `.planning/phases/11-per-client/11-CONTEXT.md` — perclient.go 装配形态（五 goroutine/ReadLoop 闭包/teardown Once）；D-04 metrics 零改动红线（D-05 的约束源）；Reusable Assets（kickSlowConsumerLocked 零改动复用）；Deferred（本 phase 全部范围的来源清单）
- `.planning/phases/10-mode-assembly/10-CONTEXT.md` — SessionMode/SpawnFunc 接缝与收口闸口径
- `.planning/STATE.md` §Learnings [Phase 11 REVIEW WR-01 → Phase 12]（:99）— 宽限门/creditPending 丢失的原始登记，D-04 按「dwell 涵盖不复刻」形态闭合，plan 中须显式回指

### 现状代码（扩展点，file:line 实证）
- `internal/server/server.go:1181-1196` — RESIZE case 现状（D-09 第二闸 ro 丢弃 :1183；rw 入 arbiter hubMu 路径 :1191-1194——per-client 直通分支插入点，§3.4 参考实现）
- `internal/server/perclient.go` — 输出闭包 trySend 失败直踢现状（阻塞持帧改造点）；pcSession 结构
- `internal/server/clients.go` — outbox trySend（:192-196）/ kickSlowConsumerLocked（1013 机械零改动复用）/ kickOrCreditLocked + creditPending + afterDrain（shared 信用门形态，D-04 不复刻的对照母本）/ gateTransitions 计数器（D-05 递增点）
- `internal/pty/io.go` — ReadLoop 缓冲复用（P5-1 别名红线：阻塞持帧期间帧在闭包栈上，持帧帧不得复用缓冲——每帧 make+copy 纪律保持）/ Resize fdMu 纪律
- `internal/proto/proto.go:111-116` — WelcomePayload（D-08 session 字段加点；G-05-1 恒序列化先例 Cols/Rows）
- `web/src/main.ts` — Welcome 处理分支（reset 调用点 + 模式位存储）/ ro RESIZE 第一闸现状（05-08，D-07 放开点）/ IN-01 per-connection 状态重置面（:503-506）/ reconnect 循环（:199-251）
- `web/uat/phase11.mjs` — 协议层 UAT 母本（phase12.mjs 同构）；`web/uat/phase06-dom*.mjs` — jsdom 前端断言先例
- `internal/server/perclient_test.go` — Phase 12 新测归属文件（D-11）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `kickSlowConsumerLocked`（clients.go:564-606）— dwell 到期 1013 踢出的执行机械零改动复用（arbiter 空集时 removeMember/recalcNow 天然 no-op，11-CONTEXT 已验）
- `outbox.trySend / drain / writer`（clients.go:192-211）— 阻塞持帧的信号源与恢复点；drain 至非满是恢复信号的 natural 挂点
- `resize.go debouncer` — 每会话防抖的共用组件来源（ROADMAP「含」：共用 debouncer 防双写漂移；Pitfall 7 SIGWINCH 风暴防线）
- `proto.DecodeResize` 钳制 [1,1000]（D-16）— 直通路径解码层零改动，钳制在 Decode 内既有
- `registry.gateTransitions`（clients.go:312-313）— D-05 停读/续读计数递增点，OPS-07 既有 series 零扩展
- `terminal.reset()`（xterm.js API）— PC-06 前端唯一新调用；IN-01 per-connection 重置面（main.ts:503-506）承载模式位存储

### Established Patterns
- **CR-01 读循环零同步写**（Anti-Pattern 5）— 阻塞持帧绝不演化为「读循环直写 master 等 outbox」；阻塞点只在输出闭包（PTY→WS 方向），INPUT 路径零改动
- **P5-1 别名红线**— ReadLoop 缓冲复用（pty/io.go:14）下每帧 make+copy；阻塞持帧期间帧持有在闭包栈上，下一帧读取被阻塞自然无别名窗口——纪律保持逐字
- **锁序三规则**（§5）— 直通 Resize 仅取 sess.fdMu 不持 hubMu；dwell 到期踢出经 hubMu → kickSlowConsumerLocked 既有序列；恢复信号通道不得引入 hubMu 反向依赖
- **Welcome 恒首帧 + G-05-1 恒序列化**— session 字段同 Cols/Rows 先例恒在键；旧服务端缺键 = shared 的协议演化纪律（前端防御性缺省）
- **零回归双证据口径**— shared 全量原样绿 + 期望值逐字未动；ro RESIZE 放行/模式位下发只在 per-client 分支生效，shared 路径逐字不动
- **EXIT 直写/唯一收割者/Welcome 恒首帧三大不变量**— 本 phase 不触碰，停读/续读/dwell 全部落在输出闭包与踢出机械内

### Integration Points
- `server.go:1181 RESIZE case` — per-client 直通分支插入点（cl.pc != nil → DecodeResize 后 sess.Resize 仅 fdMu；ro 放行经 D-06 闸形态调整，shared 分支逐字保留）
- `perclient.go 输出闭包` — trySend 失败处置从「直踢」改为「阻塞持帧 + dwell 武装」（D-01/D-02 改造点）；cl.done 逃逸保持 detach 后不做无谓组帧语义
- `clients.go outbox drain/writer` — 恢复信号挂点（drain 至非满通知持帧闭包）；gateTransitions 两个新递增点（停读/续读）
- `proto.go WelcomePayload` + `server.go Welcome 组帧点` — session 字段填充（Options.SessionMode 直读，10-CONTEXT 接缝在手）
- `web/src/main.ts Welcome 分支` — 模式位解析（缺键=shared）+ terminal.reset() 统一判断 + ro RESIZE 发送门按模式位放开
- `internal/server/perclient_test.go`（扩展）/ `web/uat/phase12.mjs`（新增）/ jsdom 测试（新增）

</code_context>

<specifics>
## Specific Ideas

- **「浏览器自动回 pong」是无看门狗方案的死因**——SC5 要求「持续过载 1013」，但 ws ping/pong 由浏览器协议栈自动应答（与应用层消费速率无关），pinger 探测不到慢端；trySend 失败在阻塞持帧后不再发生（outbox 恒满但无新失败事件）——dwell 计时器是踢出信号的唯一来源。这是 D-02 的核心论证
- **「阻塞持帧即暂存」是 D-04 的核心论证**——shared 的 creditPending 字段存在理由是 fan-out 不能为单端阻塞读循环（其他端要等帧），per-client 单消费者下「帧在闭包栈上」与「帧在 creditPending 字段」语义等价，复刻后者是纯死代码
- **D-06/D-07 必须配对生效**——仅服务端放行（D-06）而前端不放（D-07 否决项）则 wesh 自家 ro 端拖窗仍不生效，herdr ro 移动端场景问题依旧；仅前端放开而服务端丢则消息空转。两决策在 plan 中应同 plan 落地或显式标注配对关系
- **session 字段命名与 CLI flag 同词是刻意的**——`--session-mode=shared|per-client` 与 Welcome `session` 键同词同值域，文档（Phase 14 PC-12）与调试时无双写漂移；proto.go 注释与 main.ts 帧常量注释按 D-16 先例互指
- **dwell 10s 的量级依据**——涵盖：attach 初期 outbox 瞬态满（WR-01 的 500ms 宽限场景 ×20 余量）、咖啡歇级阅读暂停（分钟级内的真慢不踢需要「在前进」证据——dwell 只保护「完全停」）、浏览器后台标签页节流（Chrome 定时器节流至 1Hz 级，ws 消费暂停可超 60s——这是已知风险接受：后台标签页超 10s 无消费将被 1013，重连即新进程语义下恢复成本一次 F5；ttyd 同场景直接丢数据无提示，wesh 语义严格更强）。如 UAT 实证该场景成为痛点，调值归后续 phase 裁决（常量改值非公开契约变更）

</specifics>

<deferred>
## Deferred Ideas

- **显式 pause/resume 状态位观测**（pcSession paused 字段 + cond）— Phase 13 OPS-12 metrics 粒度落地时若需停读态 gauge 再评；本 phase gateTransitions 计数已够（D-01/D-05）
- **dwell 阈值调优入口**（flag/TOML 或常量改值）— 仅当后台标签页节流场景经 UAT/实测成为痛点；D-03 已锁本 phase 不暴露（常量改值不属公开契约变更，可后续直接调）
- **WR-01 宽限门/creditPending 复刻**— D-04 按「dwell 涵盖」闭合；若瞬态满箱误踢案例实证出现，回写 WR-01 并重开本项
- **重连「新会话」提示文案**— D-10 静默裁决；普通 shell 用户理解成本若经实测成为问题，归 UX 迭代候选（非本里程碑）
- **后台标签页 1013 后自动重连的体验**（reset+新进程+F5 等效）— PC-06 语义的自然延伸，非缺陷；herdr/tmux 场景经子程序恢复，普通 shell 场景接受
- **spawn 双令牌桶 / Shutdown N 组 / 第二终结源 / stop-timeout 默认值重议 / metrics 审计 per-client 粒度 / WESH_REMOTE_USER** — Phase 13 既定范围（11-CONTEXT deferred 原样）
- **参数化 harness / 模式文档 PC-12 / herdr E2E UAT PC-13 / Playwright 浏览器层** — Phase 14 既定范围

</deferred>

---

*Phase: 12-per-client*
*Context gathered: 2026-09-04*
