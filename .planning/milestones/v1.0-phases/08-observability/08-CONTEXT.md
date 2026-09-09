# Phase 8: 可观测性 - Context

**Gathered:** 2026-08-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 8 补齐 ttyd 缺失的可运维性三面：`/healthz` 探活端点（反代/编排可用）；`/metrics` Prometheus 文本格式指标（连接数/会话存活/收发字节/outbox 深度/踢出计数 + 认证计数器 + 基础 runtime）；运行期事件全量迁移 slog JSON 结构化审计日志（认证失败/连接建立断开/会话生命周期可检索，凭据零入日志红线保持，用户可控字段剥离控制字符）。

**In scope (from ROADMAP):** OPS-06（/healthz）、OPS-07（/metrics：连接数、会话数、收发字节数、每客户端 outbox 深度、1013 踢出计数）、OPS-08（slog JSON 结构化日志 + 审计事件：认证失败、连接建立/断开、会话生命周期）。P5/P6/P7 deferred 兑现：每客户端 outbox 深度与踢出计数进 metrics（P5）、断开退出事件进审计（P6）、remote_user 进结构化事件 + XFF 下 per-IP 节流指标口径（P7）。

**Out of scope (本阶段不做):** 自定义首页（Phase 9 OPS-03）；单二进制发布与参数标定回填（Phase 9 OPS-10）；独立运维端口（D-12 裁决同端口）；liveness/readiness 双端点（D-10 裁决单端点 200+JSON，见 deferred）；--log-format/--log-level flag（D-15 裁决恒 JSON 零新 CLI 契约）；前端任何改动（纯服务端 phase）；滚动回放/会话保持（PROJECT 锁定）。

**已锁定不重复决策：** log/slog stdlib JSONHandler（STACK.md 定案，不引 zap/logrus）；stdlib net/http ServeMux（STACK.md 明确预期 /healthz/metrics 端点，不引框架）；凭据/token 永不入日志（SEC-01/P5 D-03 红线）；remote_user sanitize C0/C1/DEL+128 rune（P7 D-19）；exitf + sync.Once 单一终结收口（P1 硬约束）；整站 Basic 认证闸（P3 D-02，/healthz 为唯一例外见 D-07）；CLI flag 不轻易新增（P2 D-15——本 phase 零新 flag）；metrics 挂点已预埋（clients.go droppedInputs/inputDrops atomic、registry.kicks/gateTransitions hubMu）。

</domain>

<decisions>
## Implementation Decisions

### Metrics 端点（OPS-07）
- **D-01:** /metrics = **手写 Prometheus 文本 exposition 格式**（stdlib 零依赖，不引 prometheus/client_golang）——指标集小（~15 series 全 gauge/counter），文本格式手写几十行；直采/Prom 兼容栈/人工 curl 三形态全覆盖（采集栈前提经用户确认按推荐成立）；与 slog stdlib、stdlib mux 的 STACK.md 哲学一致 — **Reversibility:** costly — 端点格式与 series 命名是采集契约，改动破坏已部署的 scrape 配置与看板
- **D-02:** 每客户端 outbox 深度 = **聚合 gauge（max/sum），不带客户端 label**——客户端身份（IP/remote_user）进 label 是日志红线的 metrics 延伸（隐私），且 label 基数随连接翻转不可控；max 深度即慢客户端检测的运维信号
- **D-03:** 指标范围 = **业务指标 + 基础 runtime**（goroutine 数 + runtime.ReadMemStats 内存）——P5/P6 goroutine 生命周期纪律的回归可观测化（踢出/断开后泄漏一眼可见），手写仅一两行
- **D-04:** 收发字节 = **双指标分开**：`wesh_pty_output_bytes_total`（PTY 数据源单计）+ `wesh_ws_sent_bytes_total`/`wesh_ws_recv_bytes_total`（WS 网络流量，fan-out 下行 ×N 反映真实带宽）——两 series 相除即吞吐放大比
- **D-05:** 「会话数」口径 = **`wesh_session_active` gauge（0/1）+ 连接三件套**：`wesh_clients_connected`（当前 gauge）/ `wesh_clients_total`（累计 counter）/ `wesh_clients_kicked_total`（1013 counter）——共享进程模型下会话数恒 1 是退化指标，gauge 落探活语义，连接数才是运维真实信号（ROADMAP「会话数」按此裁决兑现）
- **D-06:** 认证计数器 = **`wesh_auth_failed_total` / `wesh_auth_throttled_total`（无 IP label）+ `wesh_build_info` gauge{version}**——P7 deferred 的 XFF 节流口径落为总量计数（per-IP 明细查日志事件，metrics 不进 label）；build_info 常规自检 series

### 端点认证与挂载（OPS-06/07）
- **D-07:** /healthz = **免认证**（整站 Basic 闸唯一窄例外，README 明示）——探活器（k8s liveness/nginx/反代健康检查）结构性带不了凭据；healthz 只暴露「进程活着」无敏感信息 — **Reversibility:** one-way — 免认证端点是公开暴露面契约，收紧会让已部署探活配置失效
- **D-08:** /metrics = **跟随认证闸**（认证开启时过 Basic，Prometheus scrape_config 原生支持 basic_auth；--no-auth 模式自然免）——metrics 泄漏服务行为轮廓（连接数/失败计数），与「默认安全」哲学一致 — **Reversibility:** one-way — 同上暴露面契约
- **D-09:** /healthz、/metrics = **根路径固定，不受 --base-path 影响**——探活/采集器直连后端端口（不经反代），路径恒定可写进 k8s probe/Prometheus 静态配置；base-path 是浏览器用户面挂载形态，与运维面正交；拒绝双挂（单侧定义纪律）
- **D-10:** /healthz 返回 = **200 + 状态 JSON**（`{"status":"ok","clients":N,"max_clients":M,"session_active":bool}`）——liveness 语义由进程存活保证，body 顺手给运维一眼状态；不引 readiness 双端点（满员 503 是业务容量闸非健康闸，引 /readyz 误导编排摘流，记 deferred）
- **D-11:** 优雅关停进行中（SIGTERM/INT → 1001 广播后、进程退出前）/healthz 返回 **503 draining**——反代/编排健康检查不再向将死实例导新流；实现 = atomic bool 与 1001 广播同源挂点
- **D-12:** 运维端点与主服务 **同端口**——个人运维工具零额外配置；暴露面控制由 D-07/D-08 承担；独立运维端口未采纳（不按 deferred 挂账）

### 日志迁移形态（OPS-08）
- **D-13:** logEvent **全量原子迁移 slog JSONHandler**——logEvent 是包级函数唯一出口（auth.go），内部换实现、全部调用点零改动；stderr 运行期事件从单行文本变 JSON 一行；拒绝双轨（无双轨漂移面，wesh 无既有日志消费者需兼容） — **Reversibility:** costly — 全部 captureStderr 断言的测试随迁移改写（helper 形态见 Discretion），回退需全量还原
- **D-14:** 启动行/分享链接行（含 token）= **保持人读文本不变**——01-05 冒烟与 UAT 以启动行解析实际端口（既有消费者零破坏）；operator 交互输出与机器审计事件分流；token 不上结构化字段（P5 D-04 现状保持）
- **D-15:** 运行期事件 **恒 JSON 恒 INFO，不加 --log-format/--log-level**——零新 CLI 契约（P2 D-15 纪律）；wesh 事件量小 DEBUG 需求低，人读走 jq；拒绝 TTY 自动检测（隐式行为与显式哲学相悖）
- **D-16:** 启动期警告行（wesh: warning: 前缀）= **保持文本**——与启动行同分类（operator 交互输出）；07 落地的「结构性不含 URL/凭据」断言零改动；运行期事件才进 slog

### 审计事件目录（OPS-08）
- **D-17:** 事件集 = **全量目录**：认证（auth_failed/throttled）+ 连接（attach/detach）+ 会话生命周期（session_start=PTY spawn/session_end/shutdown）+ exit_when_empty 族（wait/cancel/fire）——ROADMAP「认证失败、连接建立/断开、会话生命周期」三面全覆盖，现状事件名沿用
- **D-18:** slog JSON 字段 = **msg 恒 "event"，事件名走独立 event 字段**（event="attach"），其余字段平铺（remote/code/remote_user/client_id 等）——jq/Loki 检索 event=="x" 直打字段索引
- **D-19:** XFF 链首 IP（trust 模式 remote 字段）**过 P7 D-19 同款 sanitize**（C0/C1/DEL 剥离+截断）——反代对 XFF 是追加非复写，客户端可注入任意首段，控制字符伪造 JSON 日志行风险当期存在；ROADMAP SC3「用户可控字段剥离控制字符」落此
- **D-20:** 事件携 **client_id（进程内单调递增序号，从 1 起，重启归零）**——同一连接的 attach/detach 事件流可关联检索；序号无隐私面（非 IP）
- **D-21:** client 断开 = **detach 单事件 + reason 字段**（normal/kick/pong_timeout/shutdown）——「连接断开」检索单入口；kick/pong_timeout 不再单独打行（计数走 metrics counter）；exit_when_empty 族属会话生命周期事件不归 detach reason
- **D-22:** session_end 字段 = **exit_code（与 EXIT 帧 D-09 同源：信号死亡 -1）+ signal 名（仅信号死亡出键）+ duration_seconds**（PTY spawn 到退出存活时长）——「会话活了多久、怎么死的」单事件齐备
- **D-23:** 认证事件字段边界 = **throttled 携 retry_after 秒数**（排查爆破节奏）；**auth_failed 不含用户名**（SEC-01 红线重申：凭据任何形态含用户名永不入日志）

### Claude's Discretion
- series 命名细节（wesh_ 前缀统一、_bytes/_total 后缀惯例）与预埋挂点兑现（droppedInputs/inputDrops/gateTransitions 的 series 名与读取形态）
- metrics handler 读取 registry 状态的并发形态（hubMu 内 plain int 的读取挂点 vs atomic 化，R-07 单锁纪律内选择）
- exposition 格式版本（Prometheus text 0.0.4 vs OpenMetrics——倾 0.0.4 最宽兼容）与 Content-Type
- slog JSONHandler 装配点（main.go 早期 SetDefault vs server 注入）与字段命名（time/level/msg 默认）
- 测试断言迁移 helper 形态（captureStderr 后按行 json.Unmarshal 到 map 断言字段；不用子串正则——JSON 转义/键序下正则脆；05-01 同步纪律保持）
- /healthz 503 draining 的 atomic bool 具体挂点（与 1001 广播同源）
- phase08.mjs UAT 场景矩阵（healthz 免认证+状态 JSON/metrics 认证闸两态/根路径固定不受 bp 影响/关停中 503/审计事件 JSON 行可检索/控制字符剥离回归）
- README 运维节写法（/healthz 免认证例外明示、反代暴露 /metrics 的 basic_auth 配方）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 8 — 成功准则 3 条（/healthz 探活可用 / /metrics 五类指标 / JSON 结构化日志可检索+凭据零入日志+控制字符剥离）
- `.planning/REQUIREMENTS.md` — OPS-06/07/08 原文
- `.planning/PROJECT.md` — Constraints（单静态二进制——D-01 零依赖手写依据）、Key Decisions（SEC-01 红线、GoTTY 共享进程模型——D-05 会话数裁决根因）

### 调研结论
- `.planning/research/STACK.md` — log/slog stdlib 定案（D-13 依据）；stdlib net/http ServeMux 明确预期 /healthz、metrics 端点（D-01/D-09 依据）
- `.planning/research/PITFALLS.md` — 计数器/map 防单调增长（metrics 计数器纪律）；R-07 单锁纪律（registry 状态读取形态约束）

### 前序 phase 决策
- `.planning/phases/07-deployment/07-CONTEXT.md` — D-19（remote_user sanitize 同款纪律——D-19 推广到 remote 字段）、D-20（XFF 链首消费范围）、deferred（remote_user 进 slog 结构化事件 + XFF 节流指标口径——D-06/D-17 兑现）
- `.planning/phases/06-session-lifecycle/06-CONTEXT.md` — D-09（EXIT 帧 exit_code 语义——D-22 session_end 字段同源）、deferred（断开退出事件进 metrics/审计——D-06/D-21 兑现）
- `.planning/phases/05-multi-client/05-CONTEXT.md` — D-03（token 永不入日志——D-14 启动行文本保持依据）、deferred（每客户端 outbox 深度与 1013 踢出计数进 metrics——D-02/D-05 兑现）
- `.planning/phases/03-auth/03-CONTEXT.md` — D-02（整站 Basic 闸——D-07/D-08 例外与跟随的基准）、SEC-01（凭据不明文进日志——D-23 红线）

### 现状代码（扩展点）
- `internal/server/auth.go` — logEvent 包级函数（D-13 原子迁移唯一改动点）；basicAuth（throttled/auth_failed 事件现状 call site，retryAfter 返回值是 D-23 字段来源）
- `internal/server/clients.go` — 预埋 metrics 挂点（droppedInputs/inputDrops atomic、registry.kicks/gateTransitions hubMu——D-02/D-05/D-06 读取源）；kickOrCreditLocked（detach reason=kick 挂点）
- `internal/server/server.go` — Handler() mux 装配（Go 1.22 模式路由 + bp 前缀先例——D-09 根路径注册挂点；basicAuth 装配先例——D-08 /metrics 过闸挂点）；Attach/detach 路径（D-17/D-20/D-21 事件挂点）；lifecycle/exitf（session_start/session_end/shutdown 事件与 D-11 draining bool 挂点）；pinger（pong_timeout 归 detach reason 挂点）
- `internal/server/proxy.go` — sanitizeRemoteUser（D-19 sanitize 复用实现）；clientIP/XFF 链首提取（remote 字段来源）
- `cmd/wesh/main.go` — 启动行/警告行文本保持面（D-14/D-16 零改动区）；slog 装配点候选
- `web/uat/phaseNN.mjs` — UAT harness 模式（phase08.mjs 同款；JSON 日志行解析断言形态）

### ttyd 源码（缺陷对照面，不参考实现）
- `~/open_src/ttyd/` — 无 /healthz、metrics、结构化日志（PROJECT Context 已核实缺陷，本 phase 补齐动机的对照面）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `logEvent 包级函数`（auth.go）— 全部运行期事件的唯一出口；D-13 原子迁移只改这一处实现，调用点零改动
- `sanitizeRemoteUser`（proxy.go）— P7 D-19 sanitize 实现，D-19 remote 字段同款处理直接复用
- `预埋 metrics 挂点`（clients.go）— droppedInputs/inputDrops（atomic，读循环热路径递增）、registry.kicks/gateTransitions（hubMu 内 plain int）——OPS-07 读取源已就位，本 phase 只加读取端
- `throttleStore.retryAfter` 返回值 — D-23 throttled 事件 retry_after 字段现成来源
- `EXIT 帧 exit_code 提取`（lifecycle，sess.Wait 返回 + ExitError 信号名提取）— D-22 session_end 字段同源复用
- `Go 1.22 mux 模式路由 + basicAuth 装配先例`（server.go Handler）— D-08 /metrics 过闸与 D-09 根路径注册的既有形态
- `captureStderr + startTrackedServerWith 同步纪律`（05-01 决策）— 测试断言迁移 helper 的宿主

### Established Patterns
- **凭据/token 永不入日志红线**（SEC-01/P5 D-03）— 本 phase 延伸到 metrics label（D-02/D-06 无客户端 label 纪律）与 auth_failed 不含用户名（D-23）
- **启动面文本行与运行期事件分流**（D-14/D-16）— operator 交互输出（启动行/警告行）保持人读，机器事件（运行期审计）进 JSON；两通道并存于 stderr
- **exitf + sync.Once 单一终结收口**（P1）— shutdown 事件与 draining bool 只在收口路径加打点，不加 exitf 分支
- **CLI flag 不轻易新增**（P2 D-15）— 本 phase 零新 flag（D-15 恒 JSON）；/healthz、/metrics 无开关（默认安全：healthz 无敏感信息、metrics 跟随认证闸）
- **单侧定义/零双轨**（PITFALLS）— D-09 拒绝双挂、D-13 拒绝双轨、D-21 detach 单事件

### Integration Points
- `server.go Handler()` — /healthz、/metrics 根路径注册（bp 前缀之外）；metrics 在认证模式过 basicAuth 装配
- `auth.go logEvent` — slog JSONHandler 换实现（D-13）；msg="event" + event 字段 schema（D-18）
- `clients.go` 预埋挂点 — metrics 读取端接入；kick 路径 detach reason=kick 打点（D-21）
- `server.go Attach/detach/lifecycle` — attach/detach/session_start/session_end/shutdown 事件打点 + client_id 序号分配（D-17/D-20/D-22）
- `server.go 1001 广播处` — draining atomic bool 置位（D-11）
- `main.go` — slog SetDefault 装配（启动行/警告行文本保持零改动）
- `web/uat/phase08.mjs` — 新 UAT 脚本（协议层断言：端点行为 + JSON 日志行解析）

</code_context>

<specifics>
## Specific Ideas

- **采集栈前提的用户确认过程**：手写 Prometheus 文本格式的决策前提经用户确认——直采/Prom 兼容栈/人工 curl 三形态下文本格式均成立（人工 curl 时 exposition 格式人读友好），前提风险闭合
- **label 隐私纪律是日志红线的 metrics 延伸**：remote IP/remote_user 永不进 metrics label（D-02/D-06）——与凭据/token 永不入日志同一哲学在指标面的镜像；per-IP 明细查日志事件，metrics 只看总量
- **/healthz 免认证是唯一窄例外**：整站 Basic 闸（P3 D-02）第一个也是唯一一个例外端点——探活器结构性带不了凭据 + 端点零敏感信息双前提成立才开；README 明示防「例外蔓延」预期
- **「会话数」指标的模型纠偏**：ROADMAP 从通用视角写「会话数」，共享进程模型下恒 1 退化——D-05 以 session_active gauge 落探活语义 + 连接三件套承载真实运维信号，是对 ROADMAP 字面的忠实兑现而非缩水
- **draining 503 的编排价值**：wesh 关停窗口短（≤10s），但反代健康检查周期内 503 能挡住「向将死实例导新流」——atomic bool 一个字段换来 systemd restart 场景的零误路由

</specifics>

<deferred>
## Deferred Ideas

- **liveness/readiness 双端点**（/readyz：满员/关停中 503）— D-10 裁决单端点 200+JSON；wesh 满员 503 是业务容量闸非健康闸，引 readiness 会误导编排摘流；若未来真实 k8s 编排部署反馈需要再评估
- **OpenMetrics 格式升级** — D-01 手写 text 0.0.4 为最宽兼容定案；OpenMetrics（ exemplar/直方图增强）待真实需求出现
- **每客户端 label 细化指标**（按 remote 定位慢客户端）— D-02 隐私/基数纪律否决；运维定位走日志事件 client_id 关联
- **独立运维端口** — D-12 裁决同端口且用户明确不挂 deferred；此处仅记录否决事由（个人运维零额外配置优先）
- **--log-format/--log-level flag** — D-15 裁决恒 JSON 零新契约；DEBUG 需求出现再评（届时连配置文件收口同批）

</deferred>

---

*Phase: 8-observability*
*Context gathered: 2026-08-27*
