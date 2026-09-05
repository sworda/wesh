# Phase 13: 资源防线与终结语义 - Context

**Gathered:** 2026-09-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 13 交付 per-client 模式的资源防线与终结语义：① churn 防线——spawn 双令牌桶（全局防惊群 + per-IP 防单点），已认证高频断开重连打不垮服务端；② HUP 免疫进程必被收割——per-client 下 --stop-timeout 默认值重议为 5s（公开契约变更）；③ 优雅关停覆盖全部存活 per-client 进程组（N 组快照逐组 stop-signal 序列 + 有界 join）；④ --once/--exit-when-empty 第二终结源（pcSupervisor/pcExitReq——「注册表已空且无子进程可等」形态仍能退出），退出码 last-reaped-code 规则与 shared 逐位对齐；⑤ metrics/审计 per-client 粒度（四个 OQ 逐项裁决落地）；⑥ WESH_REMOTE_USER 注入子进程环境（per-client only）。

**In scope (from ROADMAP):** PC-08（spawn 双令牌桶本体——容量硬帽三道闸 Phase 11 D-02/D-03 已落地，硬不变量已成立）、PC-09（--once/exit-when-empty 第二终结源 + Shutdown N 进程组 + 退出码对齐）、SEC-09（WESH_REMOTE_USER 注入）、OPS-12（metrics/审计 per-client 粒度）；WR-02 修复（reaped 栅栏 Wait-return→hubMu-acquire 微窗口零成本严格修法，STATE.md 登记）；pinger/dwell 竞态裁决闭合（Phase 12-04 发现）；churn 负载测试（合法票据 10rps×30s 断言 RSS/goroutine/fd 有界）。

**Out of scope (本阶段不做):** 参数化测试 harness 与三维归类表 / 模式语义文档 PC-12 / herdr 场景 E2E UAT PC-13 / Playwright 浏览器层 / 并发进程负载矩阵标定回填（以上 Phase 14 既定范围）；容量硬帽机制本体（Phase 11 已落地）；KILL 兜底机制本体（Phase 11 D-01 已就位，本 phase 仅改默认值）。

**已锁定不重复决策（继承，下游直接执行）：** 容量三道闸（503 / pre-spawn 再闸 / 注册点复检回收）与「并发子进程数 ≤ maxClients」硬不变量 Phase 11 已成立；teardown 固定序列恰好一次含 KILL 兜底分支（Phase 11 D-01）；spawn 失败/容量拒绝 wire 面 = Error{server_error} + 1011（Phase 11 D-02）；EXIT 直写纪律 / 唯一收割者 / Welcome 恒首帧 / exitf 恰好一次（termOnce 单点）四大不变量；零新依赖（x/time/rate 已在依赖——输入限速既有）；零身份 label 红线；零回归双证据口径（shared 全量原样绿 + 期望值逐字未动，禁止断言放宽成「两模式都接受」）。

</domain>

<decisions>
## Implementation Decisions

### stop-timeout 默认值重议（公开契约变更，登记 Key Decisions）
- **D-01:** per-client 模式下 `--stop-timeout` **未显式设置时默认 5s**（非 0）——HUP 免疫泄漏防线默认开启（STATE.md Blockers ① 裁决落定；Phase 11 post-merge 已实证泄漏窗真实存在：bash 4.4 交互模式在「提示符 pselect + 竞态输入行待读」窗口内可无声吸收 SIGHUP）；shared 模式默认 0 逐字不变（v1.0 零回归红线）。机制 Phase 11 D-01 已就位（teardown 序列 `stopTimeout > 0` 则 AfterFunc 补 SIGKILL），本 phase 仅改默认值来源 — **Reversibility:** one-way — 公开契约变更：发布后 per-client 用户将对默认 5s KILL 兜底形成依赖，改回 0 即泄漏防线默认关闭的行为破坏
- **D-02:** 显式设置位区分「未设置」vs「显式 0」（fs.Visit 显式位 + TOML 键存在即置位，07-06 合并收尾先例同形态）；用户**显式 `--stop-timeout=0`（或 TOML 显式 `stop-timeout = "0s"`）+ per-client 时尊重用户意图**，validateStartup 经 warn 通道一行明示泄漏风险（--auth-header 暴露面警告先例；「不静默改写用户输入」纪律——--once 语法糖展开先例同构）

### spawn 双令牌桶（churn 防线，PITFALLS P4 / PC-08 churn 语境操作化）
- **D-03:** 参数 = **内部常量：全局 8/s burst 16（防断网惊群）+ per-IP 1/s burst 4（防单点 churn）**；Options 覆写仅供测试注入（dwell 10s `defaultSlowDwell` 先例同构）；**不暴露 CLI flag/TOML 键**——零调优需求证据，公开契约面不膨胀；Phase 14 负载矩阵实测后如需调整，常量改值非公开契约变更
- **D-04:** 取不到令牌的拒绝 wire 形态 = **Error{server_error, 定值常量文案} + Close(1011)**——与容量再闸/spawn 失败同码同串（Phase 11 D-02「wire 聚合、日志细分」先例同构）；分辨率由 logEvent 事件名 `spawn_throttled` 承担；关闭码避开 1006 硬约束满足（前端仅对 1006 自动重连，Phase 6 shouldReconnect 裁决——1008/1011 均不在触发面，无重连放大循环）。否决 PITFALLS 原推荐 1008+spawn_throttled 新串：1008 受众 = 认证/版本策略违反（version_mismatch/auth_failed），容量/节流策略混入污染受众分治；新机器串需动 proto 错误码表，协议零改动红线优先
- **D-05:** per-IP 桶键 = **XFF 换键**（--auth-header 信任闸开启时以 XFF 值作键，未开启用 TCP remote IP——throttleStore XFF 换键先例同闸采信）；不采信则反代部署下全部用户共享一桶，per-IP 1/s burst 4 形同虚设误伤合法多用户

### 观测面四 OQ 逐项落地（OPS-12）
- **D-06:** OQ① healthz `session_active` per-client 语义 = **恒 true**（「会话服务可用」语义诚实——per-client 无「会话死亡=服务终结」态；编排探活只看 200/503 status 不受影响）；四字段键集红线不变；perclient_test.go:421 窗口期断言随之翻转
- **D-07:** OQ② `wesh_session_active` **同名 series 按模式分支取值**——shared = 0/1 探活语义（现状逐字不动），per-client = `len(pcSessions)` 活跃会话计数（快照内读）；HELP 文案按模式生成；否决另起 `wesh_sessions_active`（series 清单膨胀且 shared 侧恒 0 徒增噪音）
- **D-08:** 新计数器**四件全入**：`wesh_pty_spawn_total`（成功）/ `wesh_pty_spawn_failures_total` / `wesh_pty_kills_total`（SIGKILL 兜底次数）/ `wesh_pty_spawn_throttled_total`（节流拒绝——churn 防线「生效过」的唯一 metrics 面信号，否则只能靠日志检索）；per-client-only 出数，shared 恒 0 series 保留不摘（credit_gate 恒 0 先例）；metricsSeries17 测试侧镜像一次性扩展 17→N（Phase 11 D-04 既定窗口）；零身份 label 红线扩到新 series
- **D-09:** 审计事件按研究 §7.1 schema：session_start（每次 spawn 成功 emit：pid + **client_id** + startedAt——pc.startedAt 字段 Phase 11 已预留本 phase 消费）/ session_end（watcher emit：同 shared schema exit_code/duration/signal + **client_id** 关联键；KILL 兜底经 signal 字段归因，不另起独立事件）；spawn_failed/spawn_throttled 走 logEvent 既有四段通道（remote/code/reason/remoteUser）；零敏感值红线（token/ticket/凭据永不入参）

### pinger/dwell 竞态闭合（Phase 12-04 发现 → 本 phase 裁决）
- **D-10:** **接受 1006 语义**——「连 pong 都不回的真死/全停读连接被 pong_timeout 先杀（停读+5s~10s 窗口）」是正确收口（比 dwell 10s 更早回收）；dwell 1013 面 = 回 pong 的活慢端（真实浏览器形态——浏览器网络栈自动回 pong 不受 JS 节流影响，该竞态对浏览器结构性不可达；herdr 类自管 socket 客户端理论可达但同为「不回 pong」死连接语义）；**不匹配 coder/websocket 库内部错误文案区分两类超时**（"failed to acquire lock" vs "failed to wait for pong"——库升级改文案即失效的脆弱面）。phase12.mjs S6 的 `--ping-interval=0` 隔离 dwell 路径测试纪律固化为既定形态；README/CONFIGURATION 相应段明示该时序语义 — **Reversibility:** reversible — 接受语义不改代码路径；若未来实证 herdr 类客户端受害，区分方案可后补（pinger 判读处分支为局部变更）

### Claude's Discretion
- pcSupervisor 精确形态（hubCond 等 `(pcExitReq || exiting) && len(pcSessions) == 0`，研究 §4.1 参考实现；terminate/termOnce 逐字复用，「exitf 恰好一次、唯一收口」零漂移）
- maybeExitWhenEmptyLocked per-client 分支精确形态（`pcExitReq = true; hubCond.Broadcast()`；grace 0 立即/grace>0 宽限/宽限取消三形态复用同一计时器机械，研究 §4.3 表）
- Shutdown N 进程组快照逐组信号的有界 join 上界值与序列细节（研究 §4.4：pcSessions 快照逐组 stop-signal 序列 + exiting 置位 + hubCond.Broadcast() 补行；不等 D-state、不丢 session_end 事件）
- 退出码 last-reaped-code 规则实现（pcLastExitCode/pcHasExitCode 字段消费点；研究 §4.3 --once 两时序逐位对齐证明已备：子先死 exit 0 / 客户端先断 255）
- WESH_REMOTE_USER 注入接入点（whitelistEnv 白名单扩展 vs StartOptions.Env 通道扩展；键名固定 `WESH_REMOTE_USER`、值沿用 SEC-07 sanitize 清洗产物、仅 per-client 分支注入、spawn 时 env 组装一次到位；client.remoteUser 写一次字段既有）
- per-IP 桶存储形态（map + 惰性过期防无界增长，throttleStore 先例）
- WR-02 修复精确形态（waitDone 在 reap 完成点关闭 + teardown 快半段非阻塞 select——STATE.md 登记零成本严格修法）
- churn 压测精确断言与挂接（合法票据 10rps×30s；RSS/goroutine/fd 有界——wesh_goroutines/wesh_mem_alloc_bytes 既有观测钩子；load_test.go 扩展 vs phase13.mjs 场景的分配）
- phase13.mjs 协议层 UAT 场景集编号与断言颗粒度（建议面：churn 节流 1011+XFF 换键 / KILL 兜底默认 5s 生效（trap '' HUP 零配置）/ --once·exit-when-empty 三形态 per-client 退 255 / Shutdown N 组 pgid ESRCH + session_end 数=N / metrics 四计数器 + session_active 恒 true / WESH_REMOTE_USER env 可见）
- metricsSeries17 镜像测试扩展形态与 HELP 双模式文案断言
- 文档落点精确措辞（README/CONFIGURATION.md 默认值变更行 + 1006 先杀时序明示段——文档即被测物纪律）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 13 — 成功准则 9 条（满员 503+硬不变量 / churn 双桶限速避开 1006 / KILL 兜底+默认值重议 / Shutdown N 组有界 join / 第二终结源 255 对齐 / 退出码逐位对齐 / metrics per-client 粒度零身份 label / 审计 pid+client_id 关联 / WESH_REMOTE_USER 注入）与「含」清单
- `.planning/REQUIREMENTS.md` §PC-08/PC-09/SEC-09/OPS-12 — 需求原文（:87-100）
- `.planning/PROJECT.md` §Current Milestone v1.1 — 里程碑目标与架构形态锁定
- `.planning/STATE.md` §Blockers — 裁决项①（stop-timeout 默认值，D-01/D-02 本讨论闭合）③（四个 OQ，D-06/D-07/D-08 闭合）与 pinger/dwell 竞态条目（D-10 闭合）；WR-02 登记（[Phase 11 REVIEW WR-02 → Phase 13]）

### v1.1 调研结论（设计蓝本）
- `.planning/research/ARCHITECTURE.md` §4.1（sessionWatcher/pcSupervisor 参考实现 + exitf 语义分裂）§4.3（--once/exit-when-empty 三形态分支表 + 退出码两时序逐位对齐证明）§4.4（Shutdown 两处分支：pcSessions 快照逐组信号 + hubCond.Broadcast 补行）§5（锁序三规则 + goroutine 拓扑）§7（审计事件 schema 表 + metrics series 双语义表 + healthz OQ）§13（Open Questions 原文——OQ①②③④ 本讨论全部闭合）
- `.planning/research/PITFALLS.md` P1（空即不死/第二终结源）P4（fork bomb/双令牌桶参数推荐与 1006 避让/churn 压测形态）P8（HUP 免疫泄漏/默认值重议动机/trap '' HUP UAT 必做项）P10（Shutdown N 组/观测面）+ 验证清单（:342-349）与既有测试双模式归属表（:386-398）
- `.planning/research/FEATURES.md` §T8（maxClients 兼任进程上限）§D4（metrics/审计粒度）§D5（WESH_REMOTE_USER 裁决记录——D-15 收窄理由结构性消失）

### 前序 phase 决策（机制先例与边界）
- `.planning/phases/11-per-client/11-CONTEXT.md` — D-01 teardown 序列含 KILL 兜底（本 phase 默认值重议的机制基座）/ D-02 容量再闸 wire 形态（D-04 的同构母本）/ D-03 复检回收（裁决项④已消解）/ D-04 观测面窗口期登记（session_start/end 空白本 phase 一次补齐）
- `.planning/phases/12-per-client/12-CONTEXT.md` — D-03 dwell 内部常量+测试覆写先例（D-03 令牌桶同形态依据）；STATE.md Phase 12-04 pinger/dwell 竞态发现原文（D-10 裁决对象）
- `.planning/phases/10-mode-assembly/10-CONTEXT.md` — 显式设置位先例与「值域/枚举非敏感可回显」口径
- `.planning/milestones/v1.0-phases/06-lifecycle/06-CONTEXT.md` — accept-255 裁决（OQ1；per-client --once 255 对齐的语义源）与 EXIT 直写纪律
- `.planning/milestones/v1.0-phases/07-deployment/07-CONTEXT.md` — stop-signal/stop-timeout 序列机制母本（OPS-04）与 warn 通道先例

### 现状代码（扩展点，file:line 实证）
- `internal/server/perclient.go` — upgradePerClient（:143，令牌桶挂点 = D-02 pre-spawn 闸 :162-168 之前/同位）/ teardownPCLocked（:473，KILL 兜底分支 :483-491——D-01 默认值消费点）/ sessionWatcher（:432，session_end emit 挂点 + pcLastExitCode 记录点）/ pcSession.startedAt（:80，session_start/end duration 数据源已预留）
- `internal/server/server.go` — New per-client 分支（:535-538，pcSupervisor 钉死点）/ Shutdown stop-signal 段模式守卫（:1614-1621，per-client 空操作现状——N 组快照信号替换点）/ sessionAlive 置位点（:550，D-06 恒 true 分支点）/ hubCond（:89/:525，pcSupervisor 等待挂点既有）
- `internal/server/clients.go` — maybeExitWhenEmptyLocked per-client 早退守卫（:952-959，第二终结源分支替换点）/ stopChildLocked（SignalGroup+AfterFunc KILL 母本）/ throttleStore（XFF 换键与惰性过期先例）
- `internal/server/metrics.go` — metricsHandler 17 series（:118-147，四计数器+session 计数 gauge 扩展点）/ metricsCounters（:52）/ snapshotMetrics（:87，pcSessions 计数快照读取点）
- `internal/server/health.go` — session_active 四字段端点（D-06 分支点；键集白名单红线）
- `internal/server/log.go` — logEvent/emitEvent 既有通道（session_start/end/spawn_throttled 挂点）
- `internal/pty/spawn.go` — whitelistEnv（:115，WESH_REMOTE_USER 白名单扩展点）/ StartWithSize（:76，spawnFunc 闭包捕获链——attach 期 remoteUser 注入路径）
- `cmd/wesh/main.go` — stopTimeout 默认值（:244）与 TOML 合并显式位（:314-319，D-01/D-02 落地双点）/ validateStartup warn 通道（--auth-header 暴露面警告先例）
- `web/uat/phase11.mjs` / `phase12.mjs` — 协议层 UAT 母本（phase13.mjs 同构；RawStallClient 停读夹具可复用）
- `internal/server/perclient_test.go:421` — D-06 窗口期断言翻转点

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `x/time/rate`（go.mod 既有依赖，输入限速在用）— spawn 双令牌桶同一库零新依赖
- `throttleStore`（throttle.go）— per-IP 键管理 + XFF 换键 + 惰性过期的完整先例（D-05 直接同形态）
- `hubCond`（server.go:525 已装配挂 hubMu）— pcSupervisor 等待条件既有，零新同步件
- `terminate`/`termOnce`（server.go:1388-1392 区）— exitf 唯一收口件逐字复用（研究 §4.1 明示）
- `pc.startedAt` / `pc.exitCode`（perclient.go:80/84 已预留零消费方字段）— session_start/end 与 last-reaped-code 的数据源本 phase 激活
- `stopChildLocked` SignalGroup+AfterFunc 形态（clients.go:893-898）— Shutdown N 组快照信号的每组执行母本
- `whitelistEnv`（spawn.go:115）— WESH_REMOTE_USER 白名单扩展宿主；替换式注入纪律（非追加）保持
- `logEvent`/`emitEvent`（log.go）— 全部新事件既有通道；remote 字段 sanitize 与红线纪律不变
- `registry.gateTransitions` 计数器先例 — 新四计数器的 atomic vs hubMu plain int 选型参照系

### Established Patterns
- **显式设置位 + warn 通道分档**（07-06/--auth-header 先例）— D-02「显式 0 尊重+warn」的机制基座
- **wire 聚合、日志细分**（Phase 11 D-02 / SEC-07 先例）— D-04 节流拒绝同码同串、事件名细分的直接依据
- **内部常量 + Options 测试覆写**（12-03 defaultSlowDwell 先例）— D-03 令牌桶参数形态
- **零值等价纪律**（SessionMode 零值=shared 先例）— shared 分支一切默认值/行为逐字不动，per-client 新默认只在 per-client 分支生效
- **零回归双证据口径**— shared 全量 Go 测试原样绿 + phase02-12 各 UAT 默认模式零修改重跑 + 期望值逐字未动 diff 审查；禁止断言放宽成「两模式都接受」
- **metricsSeries17 镜像契约**（metrics_test.go）— series 清单变更必须测试镜像同步扩展（17→N 一次性，Phase 11 D-04 既定窗口）

### Integration Points
- `upgradePerClient pre-spawn 段`（perclient.go:162 前）— 双令牌桶取闸挂点：取不到 → Error+1011+spawn_throttled 日志（D-04）；取得 → 放行进入既有容量再闸
- `New per-client 分支`（server.go:535）— +pcSupervisor goroutine 钉死（per-client 唯一全局 goroutine，研究 §5 拓扑表）
- `maybeExitWhenEmptyLocked 早退守卫`（clients.go:957-959）— 替换为 pcExitReq 置位 + Broadcast（第二终结源触发端）
- `Shutdown stop-signal 段`（server.go:1614 空 if 体）— pcSessions 快照逐组 stop-signal 序列 + 有界 join 填充点
- `sessionWatcher`（perclient.go:432）— session_end emit + pcLastExitCode/pcHasExitCode 记录（hubMu 内，研究 §4.1 序列）
- `metricsHandler`/`snapshotMetrics`（metrics.go:87/118）— session_active 模式分支 + 四计数器读出
- `health.go session_active`— D-06 恒 true 分支
- `main.go stopTimeout 默认值 :244 + TOML 合并 :314` — D-01/D-02 双默认值与显式位落地
- `whitelistEnv`（spawn.go:115）或 StartOptions 通道 — WESH_REMOTE_USER 注入点（remoteUser 经 spawnFunc 闭包/StartOptions 传递，attach 期 HTTP 上下文在手——D-15 收窄理由结构性消失的兑现点）

</code_context>

<specifics>
## Specific Ideas

- **「双默认值」而非「统一改默认」是 D-01 的核心边界**——shared 的 0 默认是 v1.0 产品承诺（断开不退出、子进程继续运行）的组成部分，动它即零回归红线破坏；per-client 的产品语义相反（ttyd 语义：断开=进程该死），默认值的产品前提翻转了（PITFALLS P8 原文论证）。5s 取值 = PITFALLS 推荐值：正常程序收 HUP 后的清理窗口（shell 写 history 等）足够，泄漏进程存活上界有界
- **「wire 聚合、日志细分」第三次应用**（SEC-07 → Phase 11 D-02 → 本 D-04）——容量拒绝/spawn 失败/节流拒绝三者在 wire 面不可区分是有意为之；运维分辨率全部在日志事件名（max_clients/spawn_failed/spawn_throttled）。该家族决策已成型，后续同类面默认沿用
- **spawn_throttled 计数器的存在理由**——churn 防线若无量化信号，「防线是否生效过」只能靠事后日志检索；四件计数器中它是唯一「防线动作」指标（其余三件是结果指标）
- **D-10 的「真实浏览器结构性不可达」论证**——浏览器 WS 实现在网络进程自动回 pong，JS 主线程节流/停读不影响 pong 回复；能触发 1006 先杀的只有「连 pong 都不回」的连接（真死或 raw socket 全停读夹具）——该场景 1006 语义本就正确。这是「接受语义」而非「修复」的裁决依据
- **1006 先杀对 per-client 重连语义的叠加效应**——1006 触发前端自动重连 → per-client 下重连=全新进程。真死连接场景下这是合理恢复路径；文档明示该时序（停读 5-10s 内 1006 收口 → 自动重连新进程）防误判为 dwell 失效

</specifics>

<deferred>
## Deferred Ideas

- **令牌桶参数调优入口**（flag/TOML 或常量改值）— D-03 已锁内部常量；Phase 14 负载矩阵实测后如需调整，常量改值不属公开契约变更可直接调；暴露公开调优面需真实运维需求支撑
- **pinger 区分写阻塞/pong 超时**— D-10 接受 1006 语义；若 herdr 类自管 socket 客户端实证受害（不回 pong 的慢端被当死连接杀），pinger 判读处分支可后补（局部变更，需钉库版本+升级回归）
- **session_killed 独立审计事件**— D-09 按 signal 字段归因；若运维实证「KILL 兜底历史」检索需求强烈再评
- **参数化测试 harness（newTestServer(t, mode)）与三维归类表**— Phase 14 既定（Pitfall 11）
- **模式语义文档 PC-12 / herdr E2E UAT PC-13 / Playwright 浏览器层 / 负载矩阵标定回填**— Phase 14 既定范围
- **per-client 默认 dwell 与 ping-interval 组合语义文档化**— 随 Phase 14 PC-12 文档段一并落地（本 phase D-10 仅 README/CONFIGURATION 最小明示）

</deferred>

---

*Phase: 13-resource-defense*
*Context gathered: 2026-09-05*
