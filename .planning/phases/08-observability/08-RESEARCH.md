# Phase 8: 可观测性 - Research

**Researched:** 2026-08-27
**Domain:** Go 1.26.3 服务端可观测性——stdlib net/http 运维端点（/healthz、/metrics 手写 Prometheus text 0.0.4 exposition）+ log/slog JSONHandler 结构化审计日志迁移；纯服务端 phase，前端零改动
**Confidence:** HIGH（核心机制全部本 session 源码级核实：GOROOT go1.26.3 的 slog/encoding/json/runtime 逐行、现状 server/clients/proxy/main 代码逐行；Prometheus 规范 CITED 官方 prometheus/docs 仓原文）

## Summary

本 phase 三大件全部落在**既有架构的预留挂点与已定案选型**上，**零新外部依赖**（log/slog、runtime、sync/atomic、encoding/json 全 stdlib，与 STACK.md log/slog 定案和 D-01 手写 exposition 决策一致）。预埋挂点已就位：`registry.kicks`/`gateTransitions`（hubMu plain int）、`inputQ.droppedInputs`/`s.inputDrops`（atomic）、`registry.n`（atomic.Int64）、`c.attachSeq`（D-20 client_id 现成来源，registerLocked 单调递增从 1 起）。研究发现的**五个计划期必须处理的机制细节**（全部源码级实证）：

① **slog JSONHandler 在构造时捕获 io.Writer**——若直接 `slog.NewJSONHandler(os.Stderr, ...)` 装配，测试的 `captureStderr`（置换 `os.Stderr` 变量）将捕获不到事件行。现状 `logEvent` 是**调用时解析** `os.Stderr`（`fmt.Fprintf(os.Stderr, ...)`，server.go:1073/1076），迁移必须用一个 Write 时再解析 `os.Stderr` 的动态 writer 保持该语义，否则 05-01 登记的 `-race` 同步纪律（startTrackedServerWith + waitHandlers）下全部 stderr 断言测试结构性失效。② **encoding/json 只转义 C0（<0x20），C1（0x80-0x9F，如 NEL U+0085）原样穿透**且 slog JSONHandler 显式 `SetEscapeHTML(false)`（GOROOT encode.go:1023、json_handler.go:152 逐行核实）——JSON 化消除了换行伪造成员（C0 的 \n→\\n），但 C1 伪造日志行风险**迁移后依然存在**，D-19 的「XFF 链首 remote 字段过 sanitizeRemoteUser 同款清洗」是**必需**而非冗余。③ **detach reason 的跨 goroutine 传递有 happens-before 陷阱**：pinger（pong_timeout）在独立 goroutine，reader 路径 detach 读 reason 时若无同步边即数据竞争（-race 必中）；kick 路径在 hubMu 内天然同步，shutdown 路径可复用 `s.exiting` 判定——三种 reason 三个机制，不能一刀切。④ **metrics handler 读 registry 状态的锁序必须是 hubMu > outbox.mu**（afterDrain 先例 clients.go:451-454，onChunk→trySend 同序）——R-07 单锁纪律内，hubMu 短暂快照 + 逐客户端 outbox.mu 读深度是唯一合法形态，反序同持即死锁。⑤ **CONTEXT canonical_refs 把 logEvent 定位在 auth.go 是错的**——实际在 `internal/server/server.go:1071-1077`（包级函数、全部 18 个调用点唯一出口这一事实成立，只是文件标错；auth.go 只是其中 2 个调用点所在）。planner 不要找错文件。

**Primary recommendation:** 按五条主线拆 plan——(1) slog 原子迁移（logEvent 内部换实现 + 动态 stderr writer + msg="event"/event schema + 5 个 Go 测试文件与 phase05/07 两个 UAT 脚本的断言改 JSON 行解析，D-13 costly 级单点）；(2) 审计事件目录补全（attach/detach+reason/session_start/session_end/shutdown + client_id + throttled retry_after + remote 字段 sanitize 推广）；(3) /healthz（免认证根路径 + 200 JSON + draining 503，两枚新 atomic.Bool）；(4) /metrics（手写 exposition + 计数器挂点 + registry 快照 + version 经 Options 单一通道 + 认证闸跟随）；(5) phase08.mjs UAT + README 运维节 + 收口。依赖序：08-01 先行（日志基座），08-02 次之（事件进 slog 才有形态），08-03/08-04 可并行但 08-04 与 08-02 同触 auth.go 调用点建议顺序执行，08-05 收尾。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Metrics 端点（OPS-07）**
- **D-01:** /metrics = **手写 Prometheus 文本 exposition 格式**（stdlib 零依赖，不引 prometheus/client_golang）——指标集小（~15 series 全 gauge/counter），文本格式手写几十行；直采/Prom 兼容栈/人工 curl 三形态全覆盖（采集栈前提经用户确认按推荐成立）；与 slog stdlib、stdlib mux 的 STACK.md 哲学一致 — **Reversibility:** costly — 端点格式与 series 命名是采集契约，改动破坏已部署的 scrape 配置与看板
- **D-02:** 每客户端 outbox 深度 = **聚合 gauge（max/sum），不带客户端 label**——客户端身份（IP/remote_user）进 label 是日志红线的 metrics 延伸（隐私），且 label 基数随连接翻转不可控；max 深度即慢客户端检测的运维信号
- **D-03:** 指标范围 = **业务指标 + 基础 runtime**（goroutine 数 + runtime.ReadMemStats 内存）——P5/P6 goroutine 生命周期纪律的回归可观测化（踢出/断开后泄漏一眼可见），手写仅一两行
- **D-04:** 收发字节 = **双指标分开**：`wesh_pty_output_bytes_total`（PTY 数据源单计）+ `wesh_ws_sent_bytes_total`/`wesh_ws_recv_bytes_total`（WS 网络流量，fan-out 下行 ×N 反映真实带宽）——两 series 相除即吞吐放大比
- **D-05:** 「会话数」口径 = **`wesh_session_active` gauge（0/1）+ 连接三件套**：`wesh_clients_connected`（当前 gauge）/ `wesh_clients_total`（累计 counter）/ `wesh_clients_kicked_total`（1013 counter）——共享进程模型下会话数恒 1 是退化指标，gauge 落探活语义，连接数才是运维真实信号（ROADMAP「会话数」按此裁决兑现）
- **D-06:** 认证计数器 = **`wesh_auth_failed_total` / `wesh_auth_throttled_total`（无 IP label）+ `wesh_build_info` gauge{version}**——P7 deferred 的 XFF 节流口径落为总量计数（per-IP 明细查日志事件，metrics 不进 label）；build_info 常规自检 series

**端点认证与挂载（OPS-06/07）**
- **D-07:** /healthz = **免认证**（整站 Basic 闸唯一窄例外，README 明示）——探活器（k8s liveness/nginx/反代健康检查）结构性带不了凭据；healthz 只暴露「进程活着」无敏感信息 — **Reversibility:** one-way — 免认证端点是公开暴露面契约，收紧会让已部署探活配置失效
- **D-08:** /metrics = **跟随认证闸**（认证开启时过 Basic，Prometheus scrape_config 原生支持 basic_auth；--no-auth 模式自然免）——metrics 泄漏服务行为轮廓（连接数/失败计数），与「默认安全」哲学一致 — **Reversibility:** one-way — 同上暴露面契约
- **D-09:** /healthz、/metrics = **根路径固定，不受 --base-path 影响**——探活/采集器直连后端端口（不经反代），路径恒定可写进 k8s probe/Prometheus 静态配置；base-path 是浏览器用户面挂载形态，与运维面正交；拒绝双挂（单侧定义纪律）
- **D-10:** /healthz 返回 = **200 + 状态 JSON**（`{"status":"ok","clients":N,"max_clients":M,"session_active":bool}`）——liveness 语义由进程存活保证，body 顺手给运维一眼状态；不引 readiness 双端点（满员 503 是业务容量闸非健康闸，引 /readyz 误导编排摘流，记 deferred）
- **D-11:** 优雅关停进行中（SIGTERM/INT → 1001 广播后、进程退出前）/healthz 返回 **503 draining**——反代/编排健康检查不再向将死实例导新流；实现 = atomic bool 与 1001 广播同源挂点
- **D-12:** 运维端点与主服务 **同端口**——个人运维工具零额外配置；暴露面控制由 D-07/D-08 承担；独立运维端口未采纳（不按 deferred 挂账）

**日志迁移形态（OPS-08）**
- **D-13:** logEvent **全量原子迁移 slog JSONHandler**——logEvent 是包级函数唯一出口（auth.go），内部换实现、全部调用点零改动；stderr 运行期事件从单行文本变 JSON 一行；拒绝双轨（无双轨漂移面，wesh 无既有日志消费者需兼容） — **Reversibility:** costly — 全部 captureStderr 断言的测试随迁移改写（helper 形态见 Discretion），回退需全量还原
- **D-14:** 启动行/分享链接行（含 token）= **保持人读文本不变**——01-05 冒烟与 UAT 以启动行解析实际端口（既有消费者零破坏）；operator 交互输出与机器审计事件分流；token 不上结构化字段（P5 D-04 现状保持）
- **D-15:** 运行期事件 **恒 JSON 恒 INFO，不加 --log-format/--log-level**——零新 CLI 契约（P2 D-15 纪律）；wesh 事件量小 DEBUG 需求低，人读走 jq；拒绝 TTY 自动检测（隐式行为与显式哲学相悖）
- **D-16:** 启动期警告行（wesh: warning: 前缀）= **保持文本**——与启动行同分类（operator 交互输出）；07 落地的「结构性不含 URL/凭据」断言零改动；运行期事件才进 slog

**审计事件目录（OPS-08）**
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

### Deferred Ideas (OUT OF SCOPE)
- **liveness/readiness 双端点**（/readyz：满员/关停中 503）— D-10 裁决单端点 200+JSON；wesh 满员 503 是业务容量闸非健康闸，引 readiness 会误导编排摘流；若未来真实 k8s 编排部署反馈需要再评估
- **OpenMetrics 格式升级** — D-01 手写 text 0.0.4 为最宽兼容定案；OpenMetrics（ exemplar/直方图增强）待真实需求出现
- **每客户端 label 细化指标**（按 remote 定位慢客户端）— D-02 隐私/基数纪律否决；运维定位走日志事件 client_id 关联
- **独立运维端口** — D-12 裁决同端口且用户明确不挂 deferred；此处仅记录否决事由（个人运维零额外配置优先）
- **--log-format/--log-level flag** — D-15 裁决恒 JSON 零新契约；DEBUG 需求出现再评（届时连配置文件收口同批）
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OPS-06 | /healthz 健康检查端点 | stdlib mux 根路径注册（`Handler()` server.go:391-474 既有装配形态；405 fallback 惯例 sharetoken.go:122-128 先例）+ 免认证窄例外（basicAuth 不挂该路径即可，整站闸 server.go:401-471 逐行核实）+ 状态 JSON 数据源全现成（`registry.n.Load()` atomic、`s.maxClients` 只读字段；session_active/draining 需两枚新 atomic.Bool，挂点 = lifecycle/Shutdown 置 `s.exiting` 同位 server.go:1198/1266）——Pattern 3 |
| OPS-07 | /metrics 监控端点（连接数、会话数、收发字节数） | 手写 text 0.0.4 exposition（官方规范原文已核实——Pattern 2 给出 writer 骨架）；预埋挂点逐字就位：`registry.kicks`/`gateTransitions`（clients.go:264-268 注释明写「Phase 8 OPS-07 挂点」）、`droppedInputs`/`inputDrops`（clients.go:197-200、server.go:98-103）、`registry.n`（clients.go:250-262）；收发字节新挂点 = onChunk 入口（PTY 源单计）+ writer 成功 Write（WS 下行 ×N）+ Attach 读循环（WS 上行）；version 经 Options 单一通道（main.go:32 `var version = "dev"` → Options 生产直传先例）；认证闸跟随 = basicAuth 包装（server.go:402/416 先例） |
| OPS-08 | 结构化日志（JSON），含审计事件 | log/slog stdlib（STACK.md 定案；GOROOT go1.26.3 逐行核实 API 与转义行为——Pattern 1）；logEvent 唯一出口 server.go:1071-1077（CONTEXT 标 auth.go 有误，见 Summary ⑤）内部换实现、18 个调用点中 16 个零改动（slow_consumer/pong_timeout 两站点按 D-21 折入 detach reason）；事件 schema msg="event"+event 字段（D-18）；client_id 现成 = `c.attachSeq`（clients.go:274-275）；session_end 字段源 = lifecycle 既有 exit code/WaitStatus 提取（server.go:1154-1157）+ signalName 映射表（server.go:1099-1129）；throttled 的 retry_after 现成 = auth.go:106 已算出的 `retry int64`；测试迁移 helper = captureStderr 后按行 JSON 解析（limits_test.go:91-111 改造） |
</phase_requirements>

## Project Constraints (from CODEBUDDY.md)

| 约束 | 对本 phase 的影响 |
|------|------------------|
| 双机拓扑：Linux 开发机构建/运行 + Windows 工作站跑 Playwright；Linux 侧禁装 GUI/浏览器/playwright | 本 phase 纯服务端、零前端改动——**无需 pw 场景**；协议层 UAT（phase08.mjs 零依赖 Node 脚本）与 Go 单测全在 Linux 侧 |
| 测试分层：① `web/uat/phaseNN.mjs` 零依赖协议脚本；② `@xterm/headless`；③ jsdom；④ Windows pw 实测；⑤ 平台原生行为显式豁免 | phase08.mjs 同款零依赖纪律（Node 原生 fetch/WebSocket spawn 真实二进制）；/healthz 的 503 draining 用 SIGTERM 驱动真实关停序列（phase07.mjs S7 先例）；无平台豁免场景 |
| 不要在本机启动 wesh 实例等待人工浏览器访问 | UAT 全自动：健康检查/metrics 用 Node fetch 断言，JSON 日志行用 stderr 捕获解析断言 |
| pnpm 而非 npm；构建命令带 `time` 前缀 | 本 phase 不动 web/——但 UAT 脚本归 web/uat/ 管；若需跑前端构建验证零漂移：`time pnpm -C web build` |
| Go 单测全量 + `-race`（CI ci.yml 既定：`go vet ./...` + `go test -race -count=1 -v ./...`） | detach reason 传递、metrics 快照读取、atomic 挂点全部必须 -race 干净（本 research Pitfall 1/2 是 -race 实测高危点） |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| /healthz 端点（免认证探活 + 状态 JSON + draining 503） | API / Backend (server.go Handler 装配) | — | HTTP 端点纯服务端；数据源 registry.n/maxClients/atomic.Bool 全在 server 包 |
| /metrics 端点（手写 exposition + 认证闸跟随） | API / Backend (新 metrics.go + Handler 装配) | — | 指标数据源全部在 server 包（registry/atomic 计数器/runtime）；Prometheus 采集器是外部消费者，不进本仓 |
| slog JSONHandler 迁移（logEvent 换实现） | API / Backend (server.go logEvent 单点) | — | 包级函数唯一出口；main.go 只动 SetDefault 装配一行（Discretion 倾向，见 Pattern 1） |
| 审计事件打点（attach/detach/session_*/shutdown/exit_when_empty 族） | API / Backend (server.go/clients.go 既有事件站点) | — | 事件源全部在服务端 goroutine（Attach 升档/detach/kick/pinger/lifecycle/Shutdown） |
| 控制字符 sanitize 推广（remote 字段） | API / Backend (proxy.go sanitizeRemoteUser 复用) | — | D-19 同款纪律的提取点复用；清洗在提取点完成（单一写口纪律） |
| version 进 build_info | API / Backend (main.go `var version` → Options → metrics.go) | 发布构建（goreleaser ldflags 注入，Phase 9） | Options 单一通道纪律；Phase 8 只需 plumbing，发布期注入是 Phase 9 既定 |
| phase08.mjs 协议层 UAT | API / Backend 测试面 (web/uat/) | — | 零依赖 Node 脚本 spawn 真实二进制（CODEBUDDY.md 测试分层第 ① 层） |

（纯服务端 phase——Browser/Frontend Server/CDN/Database 各层零责任。）

## Standard Stack

### Core（全部 stdlib，零新依赖）

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| log/slog | 随 Go 1.26.3 工具链 | JSON 结构化日志 | STACK.md 定案（不引 zap/logrus）；`slog.NewJSONHandler` 一行 JSON 一行 [VERIFIED: GOROOT log/slog/json_handler.go:30-41] |
| net/http ServeMux | 随工具链（Go 1.22+ 模式路由） | /healthz、/metrics 注册 | STACK.md 明确预期「wesh 只有少量端点（index、WS 握手、/token、/healthz、metrics），引入 gin/echo/chi 是负资产」；方法模式 + path-only 405 fallback 惯例已在 sharetoken.go:122-128 落地 |
| runtime | 随工具链 | `runtime.NumGoroutine()` / `runtime.ReadMemStats()` | D-03 锁定；`func NumGoroutine() int`（debug.go:179）、`func ReadMemStats(m *MemStats)`（mstats.go:356）[VERIFIED: GOROOT runtime/debug.go:179、mstats.go:356] |
| sync/atomic | 随工具链 | 计数器与状态位 | 既有先例：registry.n（atomic.Int64）、client.mode（atomic.Value）、inputDrops/droppedInputs（atomic.Int64） |
| encoding/json | 随工具链 | /healthz 状态 JSON 编码 | /healthz body 用 `json.Marshal` 或手写——结构固定四字段，手写 fmt.Fprintf 亦可；**禁止**手写 JSON 字符串转义（见 Don't Hand-Roll） |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| （无） | — | — | 本 phase 不引入任何新依赖；Prometheus 客户端库被 D-01 显式排除 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 手写 exposition（D-01） | prometheus/client_golang | 官方库自带 registry/采集/格式演进跟进，但 ~15 series 全 gauge/counter 的场景下是负资产：违反 PROJECT 单静态二进制最小依赖哲学，且与 slog stdlib、stdlib mux 的 STACK.md 定案不一致。**用户已在 D-01 裁决手写** |
| slog（STACK 定案） | zap / logrus / zerolog | 高性能但引入依赖决策（encore.dev 等第三方）；wesh 事件量极小（每连接个位数事件），stdlib JSONHandler 足够且零依赖 |
| runtime.ReadMemStats（D-03） | runtime/metrics 包（/memory/classes/*、/sched/goroutines） | runtime/metrics 更细粒度且是官方推荐新 API，但 D-03 已锁定 ReadMemStats（一两个 gauge 够用）；若未来要 GC 暂停指标再迁移 |

**Installation:** 无（零新依赖；`go.mod` 不动）

## Package Legitimacy Audit

**本 phase 不安装任何外部包**——D-01（手写 exposition）与 STACK.md（slog stdlib）双定案使 `go.mod` 零变更。legitimacy gate 无对象，显式记录为 N/A。

## Architecture Patterns

### System Architecture Diagram

```mermaid
flowchart LR
    subgraph 数据面挂点
        RL[ReadLoop PTY 读] --> OC[onChunk<br/>+pty_output_bytes]
        OC --> OB[每客户端 outbox<br/>深度 gauge 源]
        OB --> WR[writer goroutine<br/>+ws_sent_bytes]
        RD[Attach 读循环 INPUT<br/>+ws_recv_bytes/+inputDrops]
    end
    subgraph 控制面挂点
        AA[Attach 升档<br/>registerLocked] -->|client_id=attachSeq| EV[事件流]
        DT[detach/kick/pinger/Shutdown] -->|reason| EV
        LC[lifecycle sess.Wait] -->|session_end<br/>exit_code/signal/duration| EV
        BA[basicAuth 401/429] -->|auth_failed/throttled<br/>+retry_after| EV
    end
    EV --> LE[logEvent 唯一出口<br/>slog JSONHandler<br/>动态 stderr writer]
    LE --> SE[[stderr JSON 行]]
    REG[registry 快照<br/>hubMu > outbox.mu] --> MH[/metrics handler<br/>手写 exposition]
    RT[runtime.NumGoroutine<br/>ReadMemStats] --> MH
    MH -->|text/plain; version=0.0.4| PROM[Prometheus/curl]
    HZ[/healthz handler<br/>免认证] -->|200 JSON / 503 draining| K8S[探活器/反代]
    SIG[SIGTERM/INT → Shutdown] -->|draining atomic=true| HZ
```

### Recommended Project Structure（增量，现有结构不动）

```
internal/server/
├── metrics.go        # 新：/metrics handler + exposition writer + 计数器快照（origin.go/proxy.go 同位文件组织纪律——包级 + 注释头登记决策号）
├── health.go         # 新：/healthz handler + draining/sessionAlive 判定（或并入 metrics.go 单文件 ops.go——planner 二选一）
├── log.go            # 新（或就地 server.go）：slog 装配 + 动态 stderr writer + 事件 emit helper（logEvent 从 server.go 迁入保持单文件内聚）
├── server.go         # logEvent 换实现（server.go:1071-1077）；attach/session_start/session_end/shutdown 打点；两枚 atomic.Bool 新字段
├── clients.go        # detach/kick 打点 + reason 传递；预埋计数器读取端
├── auth.go           # throttled 站点携 retry_after（D-23）；auth 计数器递增
└── proxy.go          # remote 字段 sanitize 推广挂点（D-19，clientIP 返回值过 sanitizeRemoteUser 同款）
cmd/wesh/main.go      # slog SetDefault 装配（Discretion：main 早期）+ Options.Version 透传；启动行/警告行零改动（D-14/D-16）
web/uat/phase08.mjs   # 新：协议层 UAT
```

### Pattern 1: logEvent 的 slog 原子迁移 + 动态 stderr writer（D-13 核心形态）

**What:** logEvent 保持现有签名 `(remote string, code websocket.StatusCode, reason string, remoteUser ...string)` 不变，内部从 `fmt.Fprintf(os.Stderr, ...)` 换成 slog JSONHandler 调用；handler 的 writer 必须是**调用时解析 os.Stderr 的动态 writer**，否则 captureStderr 测试全部失明。

**Why（关键实证）:** slog handler 在构造时捕获 io.Writer——`NewJSONHandler` 把 `w` 存进 commonHandler [VERIFIED: GOROOT json_handler.go:30-41，逐字：`return &JSONHandler{&commonHandler{json: true, w: w, opts: *opts, mu: &sync.Mutex{}}}`]。现状 logEvent 每次调用时读 `os.Stderr` 变量（server.go:1073/1076），captureStderr 依赖的正是这个调用时解析语义（limits_test.go:91-111 置换 `os.Stderr` 为管道）。若 handler 构造在包级 init 或 server.New（早于测试置换），事件行会写进旧 stderr 永远不可见。

**Example:**
```go
// Source: 本 research 设计（基于 GOROOT json_handler.go:30-41 构造语义 + limits_test.go:91-111 捕获纪律）
// stderrW 动态解析 os.Stderr——保持 logEvent 现状「调用时解析」语义（05-01 同步纪律不变）。
type stderrW struct{}

func (stderrW) Write(p []byte) (int, error) { return os.Stderr.Write(p) }

// 包级单例：JSONHandler 内部 mu 串行化记录（json_handler.go:36），并发 emit 安全且
// 每记录恒完整一行——比现状 fmt.Fprintf（理论可交错）更强。
var eventLog = slog.New(slog.NewJSONHandler(stderrW{}, nil))

// logEvent 签名不变（D-13：18 个调用点 16 个零改动）；schema = D-18。
func logEvent(remote string, code websocket.StatusCode, reason string, remoteUser ...string) {
	attrs := []slog.Attr{
		slog.String("event", reason),
		slog.String("remote", remote),
		slog.Int("code", int(code)),
	}
	if len(remoteUser) > 0 && remoteUser[0] != "" {
		attrs = append(attrs, slog.String("remote_user", remoteUser[0]))
	}
	eventLog.LogAttrs(context.Background(), slog.LevelInfo, "event", attrs...)
}
```

输出形态（time=RFC3339 毫秒、level="INFO" [VERIFIED: GOROOT handler.go:177-185 默认键 `TimeKey = "time"`/`LevelKey = "level"`/`MessageKey = "msg"`；毫秒格式 handler.go:622 `appendRFC3339Millis`]）：
```json
{"time":"2026-08-27T09:40:00.123Z","level":"INFO","msg":"event","event":"auth_failed","remote":"127.0.0.1:51234","code":401}
```

**装配点（Discretion 倾向）:** main.go 不需要动——包级 `eventLog` 直接可用。D-15 恒 JSON 恒 INFO 意味着无配置面，`slog.SetDefault` 仅在想让 `slog.Info` 便捷函数可用时才需要；server 包内用私有 `eventLog` 更内聚（不污染全局默认 logger，测试隔离性更好）。倾向：**server 包包级 `eventLog`，不调 SetDefault**。

**注意:** 用 `LogAttrs`/`slog.String/Int` 类型化 attr，**不用** kv 交替参数——kv 奇数个或非 string 键会产出 `!BADKEY` 键 [VERIFIED: GOROOT logger.go:187，逐字注释 `// - Otherwise, the argument is treated as a value with key "!BADKEY".`]。

### Pattern 2: 手写 Prometheus text 0.0.4 exposition writer（D-01）

**What:** metrics handler 采集快照 → 按规范拼文本 → `text/plain; version=0.0.4; charset=utf-8` 写出。

**规范要点**（全部 CITED 官方 prometheus/docs 仓 exposition_formats.md，本 session curl 取得原文）：
- Content-Type：`text/plain` with parameters `version=0.0.4`（逐字：「A missing `version` value will lead to a fall-back to the most recent text format version.」）——wesh 显式带 `version=0.0.4; charset=utf-8`（client_golang 同款串）
- 编码：「UTF-8, `\n` line endings」；「The last line must end with a line feed character.」
- HELP：「If the token is `HELP`, at least one more token is expected, which is the metric name…Only one `HELP` line may exist for any given metric name.」
- TYPE：「the second is either `counter`, `gauge`, `histogram`, `summary`, or `untyped`…The `TYPE` line for a metric name must appear before the first sample is reported for that metric name.」
- label 值转义：「the backslash (`\`), double-quote (`"`), and line feed (`\n`) characters have to be escaped as `\\`, `\"`, and `\n`, respectively.」（仅 build_info 的 version label 需要——version 由 ldflags 注入，理论可控但转义 helper 必须写，几行）
- 分组：「All lines for a given metric must be provided as one single group, with the optional `HELP` and `TYPE` lines first」

**Example:**
```go
// Source: 本 research 设计（规范逐字依据上文 CITED 条款）
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	snap := s.snapshotMetrics() // Pattern 3：hubMu 内一次快照
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	var b strings.Builder
	writeGauge(&b, "wesh_clients_connected", "Currently attached WebSocket clients.", snap.clientsConnected)
	writeCounter(&b, "wesh_clients_total", "Total attached clients since process start.", snap.clientsTotal)
	writeCounter(&b, "wesh_clients_kicked_total", "Clients kicked with 1013 (slow consumer).", snap.kicks)
	writeGauge(&b, "wesh_session_active", "Whether the PTY session is alive (1) or exited (0).", snap.sessionActive)
	// ... outbox max/sum、字节三件套、auth 两计数器、dropped 两件、gateTransitions、goroutines、mem
	writeBuildInfo(&b, "wesh_build_info", "wesh build metadata.", s.version) // label 值过 escLabel()
	fmt.Fprint(w, b.String()) // 末行恒带 \n（builder 每行 \n 收尾——规范要求）
}

func writeCounter(b *strings.Builder, name, help string, v int64) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, v)
}
```

**Series 清单建议（Discretion 内，命名遵循官方 naming.md**——CITED：「SHOULD have a (single-word) application prefix…usually the application name itself」「an accumulating count has `total` as a suffix」「`foobar_build_info` (for a pseudo-metric that provides metadata about the running binary)」**）：**

| Series | Type | 数据源（全部已核实存在） |
|--------|------|--------------------------|
| `wesh_clients_connected` | gauge | `s.registry.n.Load()` [VERIFIED: internal/server/clients.go:262 `n atomic.Int64`] |
| `wesh_clients_total` | counter | registry 新增 plain int64，registerLocked 唯一加点（hubMu 内，与 kicks 同形态） |
| `wesh_clients_kicked_total` | counter | `s.registry.kicks` [VERIFIED: clients.go:264，hubMu plain int，逐字注释「Phase 8 OPS-07 观测性挂点」] |
| `wesh_session_active` | gauge | 新 atomic.Bool（New 置 true，lifecycle sess.Wait 返回后置 false） |
| `wesh_outbox_depth_bytes_max` / `_sum` | gauge | hubMu 内遍历 registry.set + 逐 outbox.mu 读 `bytes`（Pattern 3） |
| `wesh_pty_output_bytes_total` | counter | onChunk 入口 `len(chunk)`（PTY 源单计，D-04） |
| `wesh_ws_sent_bytes_total` | counter | writer 成功 Write 后 `len(msg)`（fan-out ×N 真实带宽，D-04） |
| `wesh_ws_recv_bytes_total` | counter | Attach 读循环每消息 `len(data)`（两站点：Hello 首读 server.go:740 + 稳态循环 :880） |
| `wesh_auth_failed_total` | counter | auth.go:116 + server.go:773 两站点递增 |
| `wesh_auth_throttled_total` | counter | auth.go:108 站点递增 |
| `wesh_input_rate_dropped_total` | counter | `s.inputDrops.Load()` [VERIFIED: server.go:103 `inputDrops atomic.Int64`] |
| `wesh_input_queue_dropped_total` | counter | `s.inputQ.droppedInputs.Load()` [VERIFIED: clients.go:200] |
| `wesh_credit_gate_transitions_total` | counter | `s.registry.gateTransitions` [VERIFIED: clients.go:268] |
| `wesh_goroutines` | gauge | `runtime.NumGoroutine()`（D-03） |
| `wesh_mem_alloc_bytes` | gauge | `runtime.ReadMemStats(&m); m.Alloc`（D-03；Alloc==HeapAlloc，GOROOT mstats.go:58-61 注释逐字） |
| `wesh_build_info{version="..."}` | gauge(=1) | `s.version`（Options.Version，main.go:32 `var version = "dev"` 透传；label 值过 escLabel） |

共 17 series（outbox max/sum 拆两条计），与 D-01「~15 series」口径吻合。

### Pattern 3: registry 状态快照的并发形态（Discretion 裁决建议）

**What:** metrics handler 读取 hubMu 保护状态（kicks/gateTransitions/outbox 深度/clientsTotal）的合法形态。

**锁序实证（R-07 单锁纪律）:** 既定锁序 **hubMu > outbox.mu**——afterDrain 逐字注释 [VERIFIED: clients.go:433-434：「锁序 R-07：drain 完才取 hubMu（本函数），绝不反序同持；hubMu > outboxMu 的同序同持与 onChunk→trySend 同款」]，实现 [VERIFIED: clients.go:451-454：`s.hubMu.Lock(); defer s.hubMu.Unlock(); c.outbox.mu.Lock()`]。

**Example:**
```go
// Source: 本 research 设计（锁序依据 clients.go:433-434/451-454 既定先例）
type metricsSnap struct {
	clientsConnected, clientsTotal, kicks, gateTransitions int64
	outboxMax, outboxSum                                   int
}

func (s *Server) snapshotMetrics() metricsSnap {
	var sn metricsSnap
	s.hubMu.Lock()
	sn.clientsConnected = s.registry.n.Load() // atomic 与 hubMu 双通道读均可——锁内读 plain 化
	sn.clientsTotal = s.registry.clientsTotal
	sn.kicks = int64(s.registry.kicks)
	sn.gateTransitions = int64(s.registry.gateTransitions)
	for c := range s.registry.set {
		c.outbox.mu.Lock()
		d := c.outbox.bytes
		c.outbox.mu.Unlock()
		sn.outboxSum += d
		if d > sn.outboxMax {
			sn.outboxMax = d
		}
	}
	s.hubMu.Unlock()
	return sn
}
```

**为何取 hubMu 而非把 kicks/gateTransitions 改 atomic:** R-07 单锁纪律下 hubMu 内 plain int 是**刻意选型**（clients.go:264 注释逐字「hubMu 保护，单锁纪律 R-07 下无需 atomic」）；为 metrics 读取把它们 atomic 化是**为读改写的反向耦合**，且破坏「计数与状态变更同锁原子」的记账不变量。metrics 采集是低频（Prometheus 默认 15s 间隔）、快照临界区是 O(客户端数≤32) 的纯读，持锁微秒级——onChunk 每 chunk 持同锁做 fan-out，采集不构成可观测竞争。**建议：hubMu 快照（如上），不改任何既有字段的承载形态。**

### Pattern 4: detach reason 的 happens-before 传递（D-21 实现关键）

**What:** detach 事件携 `reason`（normal/kick/pong_timeout/shutdown），三种非默认 reason 三条传递路径：

| reason | 设置点 | 同步机制（-race 关键） |
|--------|--------|------------------------|
| `kick` | kickSlowConsumerLocked（hubMu 内，clients.go:500） | **天然同步**——kick 与 detach 同在 hubMu 内；kick 路径自身就是 removeLocked==true 的移除方，直接就地 emit detach reason=kick（替换现 slow_consumer 行，D-21「不再单独打行」） |
| `pong_timeout` | pinger goroutine（server.go:1040 现站点） | **需新同步边**——pinger 独立于 reader；naive 方案「pinger 写 c.detachReason plain 字段 + detach 读」是数据竞争（-race 必中）。两合法形态：(a) `atomic.Value`（client.mode 先例，05-03「atomic 是热路径无锁读的唯一合理形态」——但本路径是冷路径）；(b) pinger 置位时取 hubMu 写 c 字段（detach 在 hubMu 内读）。**倾向 (b)**：冷路径、与 kick 同锁语义统一、pinger 每 interval 才一次取锁无热路径影响 |
| `shutdown` | Shutdown()/lifecycle 广播窗口 | **复用 `s.exiting` 判定**——detach 在 hubMu 内，exiting 同为 hubMu 字段；两广播窗口（1001 Shutdown server.go:1266、1000 lifecycle server.go:1198）置位先于 Close 广播， detach 时 `s.exiting==true` 即 reason=shutdown（服务端主动收口语义覆盖两窗口） |
| `normal` | detach 默认分支 | 上述三者均不命中即 normal（对端主动关闭/网络断开） |

**恰好一次归属:** 与既有「removeLocked 胜出方负责 close(done)/cancel」同一所有权规则（clients.go:693-694 注释「恰好一次由成员判定保证」）——detach 事件由 **removeLocked 返回 true 的路径** emit：reader 路径 detach()（clients.go:690）或 kick 路径 kickSlowConsumerLocked()（clients.go:500）。两路径都在 hubMu 内，emit 本身不取新锁。

### Pattern 5: 测试断言迁移 helper（Discretion 定形建议）

**What:** captureStderr 保持不动（os.Pipe 置换 + startTrackedServerWith 的 waitHandlers 同步边，05-01 纪律），断言层从子串匹配改为**按行 JSON 解析**。

**形态:**
```go
// Source: 本 research 设计（CONTEXT Discretion 逐字授权形态）
// parseEvents 把捕获的 stderr 按行解析为事件 map 集——跳过非 '{' 起始行
// （D-16 启动警告行保持文本 + panic 栈等混合流成员，不得因非 JSON 行 FAIL）。
func parseEvents(t *testing.T, captured string) []map[string]any {
	t.Helper()
	var evs []map[string]any
	for _, line := range strings.Split(captured, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("事件行非合法 JSON: %q: %v", line, err)
		}
		evs = append(evs, m)
	}
	return evs
}
// 断言形态：过滤 event=="x" 后断言字段——countByEvent(evs, "auth_failed") == 3、
// evs[0]["remote_user"] == "alice"、evs[0]["code"] == float64(401)（JSON 数字解码为 float64）。
```

**两个断言陷阱（迁移时必查）:** ① JSON 数字解进 `map[string]any` 是 `float64`——`code` 断言须按 float64 比或写 intOf helper；② 行尾锚定断言（`reason=exit_when_empty\n` 区分 `_wait` 后缀）在 JSON 字段语义下天然消解——`m["event"] == "exit_when_empty"` 精确相等，无需锚定。**禁止子串/正则断言 JSON 行**（CONTEXT Discretion 逐字：「JSON 转义/键序下正则脆」）。

**凭据红线反断言迁移:** auth_e2e_test.go:457-463 与 proxy_e2e_test.go:344-347 的负断言（stderr 不含 b64 凭据/ticket/roTok/rwTok）**保持子串形态不变**——它们断言的是「全文不含敏感串」，与 JSON 化正交，逐字保留。

### Anti-Patterns to Avoid

- **handler 构造时直接传 os.Stderr:** 见 Pattern 1——captureStderr 失明，05-01 同步纪律下 -race 测试结构性失效；必须动态 writer。
- **kv 交替参数调 slog:** 奇数参数/非 string 键产出 `!BADKEY`（GOROOT logger.go:187）；一律 `LogAttrs` + 类型化 attr。
- **为 metrics 读取把 kicks/gateTransitions 改 atomic:** 见 Pattern 3——为读改写的反向耦合，破坏 R-07 选型意图；hubMu 快照即可。
- **detach reason 用 plain 字段跨 goroutine 传递:** pinger→detach 无同步边即数据竞争（Pattern 4 表）。
- **metrics label 携 remote/remote_user/client_id:** D-02/D-06 隐私+基数纪律（日志红线的 metrics 延伸）；per-IP/每连接明细查日志事件。
- **手写 JSON 字符串拼接 /healthz body 时自写转义:** 见 Don't Hand-Roll——`json.Marshal` 或固定结构 fmt.Fprintf（四字段全非用户可控，无转义面）；真正的用户可控值（version label）转义走统一 escLabel helper。
- **在 healthz/metrics 上挂 bp 前缀双挂:** D-09 拒绝双挂（单侧定义纪律）；根路径唯一注册。
- **对 /healthz、/metrics 省略 path-only 405 fallback 又用方法模式注册:** 方法模式的内建 405 会被 `"/"` 子树吞掉（03-03 GOROOT 实证，07-01 登记为 RESEARCH Pitfall 4）——POST /healthz 会落进静态伺服返回 index 而非 405。惯例形态：`mux.HandleFunc("GET /healthz", h)` + path-only fallback（sharetoken.go:122-128 逐字先例）；或干脆不划方法（读端点任意方法同响应，亦可，但偏离既有惯例）。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON 日志编码/转义 | 手写 `{"event":"` 拼接 | log/slog JSONHandler | 转义规则细节多（C0/U+2028/U+2029/无效 UTF-8→\ufffd，GOROOT encode.go 逐行）；slog 还带来行级原子性（内部 mutex） |
| JSON body 编码（/healthz） | 字符串拼接含动态值 | encoding/json `json.Marshal` | 固定四字段结构虽小，但 Marshal 一行且无转义盲区 |
| 时间格式 | time.Format 手写 | slog 内建（RFC3339 millis） | handler.go:622 appendRFC3339Millis 内建；自建即漂移 |
| 并发计数 | mutex+int 自封装 | sync/atomic.Int64 | 既有先例（inputDrops/droppedInputs/registry.n）；热路径无锁 |
| exposition label 转义 | 逐站点 ad-hoc ReplaceAll | 单一 escLabel helper（`\\`→`\\\\`、`"`→`\"`、`\n`→`\\n`，顺序敏感——反斜杠先行） | 规范逐字要求（Pattern 2 CITED）；只 build_info version 一个消费点但 helper 必须单侧定义 |
| goroutine/内存采样 | 自读 /proc/self/status | runtime.NumGoroutine/ReadMemStats | 跨平台（darwin 同代码路径）、零解析 |

**Key insight:** 本 phase 的「手写」只发生在** exposition 文本拼装**（D-01 裁决域，~40 行 builder 调用）与**动态 stderr writer**（3 行）两处；其余一切（JSON 编码、时间、转义、并发）全部 stdlib 现成——手写面被决策压缩到最小，这正是 D-01「文本格式手写几十行」成立的前提。

## Runtime State Inventory

> 本 phase 涉及**日志格式迁移**（stderr 文本行 → JSON 行），按 rename/migration 纪律逐类显式回答。

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | **None**——wesh 日志只写 stderr 流，无落盘文件、无数据库、无 key 引用事件名/日志格式 | 无（进程重启即切换格式） |
| Live service config | **None**——wesh 无外部服务注册（无 n8n/Datadog/Tailscale 类配置面）；D-13 明确「wesh 无既有日志消费者需兼容」 | 无 |
| OS-registered state | **None**——无 systemd unit/任务计划随仓库分发（README §部署与配置 给出的是示例 unit，其中 ExecStart 不引用日志格式；systemd 的 stderr→journald 通道对格式透明） | 无（示例 unit 无需变更；journald 侧 JSON 行照常 ingest） |
| Secrets/env vars | **None**——无凭据名/env 键引用日志格式；红线反断言（凭据不入日志）在新格式下语义不变 | 无 |
| Build artifacts | **None**——无已安装产物携带旧格式（单二进制每次构建全量替换） | 无 |
| **测试断言消费者（本仓内）** | 5 个 Go 测试文件（limits/emptyexit/auth_e2e/proxy_e2e/multi_test.go:837）+ 2 个 UAT 脚本（phase05.mjs S6、phase07.mjs S4）断言旧文本行格式——**这是格式迁移的真实消费者面** | **代码编辑**：断言改 JSON 行解析（Pattern 5）；phase02/03/04/06.mjs 不消费事件行（已逐脚本核实——phase06 仅断言 panic 缺席，phase03 仅 wire close reason），零改动 |

## Common Pitfalls

### Pitfall 1: slog handler 捕获旧 os.Stderr，测试捕获失明
**What goes wrong:** 包级 `slog.New(slog.NewJSONHandler(os.Stderr, nil))` 装配后，captureStderr 置换 `os.Stderr` 变量对 handler 不可见——事件行写进程启动时的 stderr，测试管道永远读不到，全部事件断言 FAIL 且 -race 下出现「写已关闭 fd」类诡异竞态。
**Why it happens:** NewJSONHandler 构造时存 `w`（GOROOT json_handler.go:30-41 逐行核实）；Go 的 `os.Stderr` 是变量不是常量，置换只影响后续读取者。
**How to avoid:** Pattern 1 动态 writer（`func (stderrW) Write(p []byte) { return os.Stderr.Write(p) }`）——保持现状 logEvent「调用时解析」语义逐字。
**Warning signs:** 迁移后首个 captureStderr 测试事件计数恒 0。

### Pitfall 2: detach reason 的 pinger→detach 数据竞争
**What goes wrong:** pinger goroutine 置 `c.detachReason = "pong_timeout"`（plain 字段），reader 路径 detach 读——无同步边，-race 必中（本仓 05-03 client.mode、05-05 inputDrops 均有过同类实测命中记录）。
**Why it happens:** CloseNow 引发的 reader 错误与 pinger 的写字段之间没有 Go 内存模型边（close 传播是副作用不是同步原语）。
**How to avoid:** Pattern 4 表——pinger 取 hubMu 写字段（冷路径，与 kick/shutdown 判定同锁），或 atomic.Value。**禁止**「反正 -race 不一定中」的侥幸 plain 字段。
**Warning signs:** `go test -race` 在 TestPingKeepalive/TestSuccessionKickRace 类测试报 WARNING: DATA RACE。

### Pitfall 3: metrics handler 反序同持 hubMu/outbox.mu 死锁
**What goes wrong:** handler 先锁某 outbox.mu 再取 hubMu（如想「先算深度再算计数」分两趟），与 onChunk（hubMu→trySend 的 outbox.mu）或 afterDrain（hubMu→outbox.mu）构 ABBA 死锁——采集一发即全站 fan-out 冻结。
**Why it happens:** 锁序注释分散（clients.go:433-434），新代码不查先例。
**How to avoid:** Pattern 3 单快照形态：hubMu 一趟内逐 outbox.mu（既定同序）；hubMu 外绝不触 outbox.mu。
**Warning signs:** curl /metrics 后终端输出冻结（stage 环境一发即现）。

### Pitfall 4: JSON 数字断言按 int 比较
**What goes wrong:** `json.Unmarshal` 到 `map[string]any` 后 `m["code"]` 是 `float64(401)`——`m["code"] == 401` 恒 false（untyped constant 不跨界），断言全红。
**Why it happens:** Go encoding/json 数字默认 float64 语义。
**How to avoid:** helper 内 `intOf(m["code"])`（`int(m["code"].(float64))`）或按 float64 字面量比；client_id/exit_code/duration_seconds 同纪律。
**Warning signs:** 字段存在（`m["code"] != nil` 通过）但等值断言失败。

### Pitfall 5: C1 控制字符在 JSON 日志中「看起来已转义」
**What goes wrong:** 以为迁移 JSON 后 sanitize 可省——C0（\n→\\n）确实被 encoding/json 转义，但 **C1（0x80-0x9F）原样穿透**（GOROOT encode.go:1023 逐行：转义只覆盖 `bytes < 0x20`；slog 还显式 SetEscapeHTML(false)，json_handler.go:152）。NEL（U+0085）在多数 pager/journald viewer 里是换行——伪造日志行通道迁移后**仍在**。
**Why it happens:** JSON 转义的心理模型是「控制字符全转义」，实际只覆盖 C0 + 引号 + 反斜杠（+ U+2028/2029）。
**How to avoid:** D-19 逐字执行——trust 模式 remote 字段（XFF 链首）过 sanitizeRemoteUser 同款清洗（proxy.go:55-67 复用）；**且注意现状 clientIP 已有 net.ParseIP 校验**（proxy.go:93——ParseIP 通过值字符集恒为 [0-9a-fA-F:.]，结构性排除注入，07-review CR-02），D-19 的 sanitize 是**纵深第二道**（防未来取值路径放宽），两道并存不冲突。
**Warning signs:** UAT 控制字符探针（phase07.mjs S4c 的 NEL 线形构造先例：UTF-8 双码点上线）在 remote 字段 JSON 行里解出原始 NEL。

### Pitfall 6: /metrics 认证闸把 Prometheus 目标打成 429
**What goes wrong:** scrape 凭据配错/过期 → basicAuth ① 429 节流闸（auth.go:104-110）命中后 Prometheus 恒 429，且 per-IP 计数把真实监控源 IP 锁进退避窗口。
**Why it happens:** D-08 的「跟随认证闸」包含节流语义（与 / 同链）；scrape 是高频自动客户端，凭据错误时失败计数涨得比人工快得多。
**How to avoid:** 不是代码问题——README 运维节必须给 Prometheus `basic_auth` 配方并明示「凭据错误会触发全站节流」；测试覆盖「错凭据 scrape → 401/429 两态」。
**Warning signs:** Prometheus target down 且 wesh 日志出现 throttled 事件 remote=采集器 IP。

### Pitfall 7: healthz/metrics 方法模式 405 被 "/" 子树吞掉
**What goes wrong:** 只注册 `"GET /healthz"` 时 POST /healthz 落入 `"/"` 子树静态伺服（返回 index 200/404）而非 405——探活器配置错误被静默掩盖。
**Why it happens:** Go 1.22+ mux 方法模式的内建 405 仅在无任何其他模式匹配时触发（03-03 GOROOT server.go:2699-2710 n==nil 分支实证）；`"/"` 子树恒匹配。
**How to avoid:** sharetoken.go:122-128 逐字先例——方法模式 + path-only 405 fallback（`Allow: GET`）成对注册；/healthz、/metrics 各一对。
**Warning signs:** `curl -X POST localhost:PORT/healthz` 返回 HTML 而非 405。

## Code Examples

### 事件行（迁移后）与 jq 检索

```json
{"time":"2026-08-27T09:40:01.013Z","level":"INFO","msg":"event","event":"attach","remote":"198.51.100.7","remote_user":"alice","client_id":1,"code":0}
{"time":"2026-08-27T09:41:12.554Z","level":"INFO","msg":"event","event":"detach","remote":"198.51.100.7","remote_user":"alice","client_id":1,"code":1000,"reason":"normal"}
{"time":"2026-08-27T09:41:12.601Z","level":"INFO","msg":"event","event":"throttled","remote":"203.0.113.9","code":429,"retry_after":4}
{"time":"2026-08-27T09:42:00.000Z","level":"INFO","msg":"event","event":"session_end","exit_code":-1,"signal":"SIGHUP","duration_seconds":119.6}
```

```bash
# 检索认证失败（D-18 设计意图：event 字段直打索引）
journalctl -u wesh -o cat | jq -c 'select(.event=="auth_failed")'
# 关联单连接生命周期（D-20）
journalctl -u wesh -o cat | jq -c 'select(.client_id==7)'
```

### session_end 字段提取（D-22 与 EXIT 帧同源）

```go
// Source: 现状 exitMessage 内联逻辑（internal/server/server.go:1154-1157，逐字）——
// 抽为包级小 helper 供 exitMessage 与 session_end 事件两消费点共用（单侧定义纪律）。
	sig := code
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		sig = int(ws.Signal())
	}
// 信号名：signalName(syscall.Signal(sig))（server.go:1099-1129 映射表现存——
// SIGHUP/SIGINT/SIGQUIT/SIGILL/SIGABRT/SIGKILL/SIGSEGV/SIGPIPE/SIGALRM/SIGTERM/SIGUSR1/SIGUSR2/SIGCHLD，
// 未命中返回 ("", false) → D-22「仅信号死亡出键」：ok==false 时不出 signal 键或回退数字形态由 planner 定）。
// duration_seconds：lifecycle 内 time.Since(s.startedAt).Seconds()——
// s.startedAt 在 New 尾部记录（与 pty.Start 的时刻差为毫秒级，审计语义无碍）。
```

### /healthz handler 骨架

```go
// Source: 本 research 设计（D-10/D-11 锁定形态；数据源全部已核实）
func (s *Server) healthzHandler(w http.ResponseWriter, _ *http.Request) {
	status := "ok"
	code := http.StatusOK
	if s.draining.Load() { // D-11：Shutdown 入口置位（与 server.go:1266 s.exiting=true 同源挂点）
		status, code = "draining", http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"status":%q,"clients":%d,"max_clients":%d,"session_active":%t}`+"\n",
		status, s.registry.n.Load(), s.maxClients, s.sessionAlive.Load())
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| log/slog 出现前的 log 包 + 手写 JSON | stdlib slog JSONHandler | Go 1.21（2023-08） | 本 phase 正是该演进的兑现——STACK.md 定案即基于此 |
| prometheus/client_golang 是唯一现实选择 | 小指标集手写 text 0.0.4 可行 | 规范自 2014-04 稳定（CITED 官方 Basic info 表「Inception: April 2014」） | D-01 裁决的前提：格式十年未变，手写无跟进负担 |
| runtime.ReadMemStats | runtime/metrics（/memory/classes、/sched/goroutines 命名空间） | Go 1.16+ | D-03 锁定 ReadMemStats（够用）；runtime/metrics 是未来细化路径（本 phase 不用） |
| OpenMetrics | 仍是 text 0.0.4 最宽兼容 | OpenMetrics 2021 起推进但未取代 | Deferred（CONTEXT 明确：待真实需求） |

**Deprecated/outdated:**
- zap/logrus/zerolog 对本项目：非 deprecated 但被 STACK.md 定案排除（stdlib slog 零依赖哲学），不再评估。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `Content-Type: text/plain; version=0.0.4; charset=utf-8` 中追加 `; charset=utf-8` 与 client_golang 同款且采集器兼容 | Pattern 2 | 极低——charset 参数在 text/plain 上合法且常见；若某采集器严格匹配 `version=0.0.4` 后的精确串尾，理论可去掉 charset 段（规范只要求 version 参数） |
| A2 | `wesh_outbox_depth_bytes_max/_sum` 拆两 series（而非单 series 带 aggregate label） | Pattern 2 series 表 | 低——D-02 只锁「max/sum 聚合、无客户端 label」，拆 series 与 label 两形态都合规；改名成本在采集配置侧（未部署前零成本） |
| A3 | `wesh_clients_total` 用 registry 内 hubMu plain int64（与 kicks 同形态）而非 atomic | Pattern 2/3 | 低——读取端已取 hubMu 快照，plain 即同步；若 planner 选 atomic 亦等价 |
| A4 | ws_recv_bytes 计数含 Hello 首读（server.go:740）与稳态循环（:880）两站点 | Pattern 2 series 表 | 低——每连接 Hello ~100B，省略不影响运维语义；计入更忠实「WS 网络流量」字面 |
| A5 | draining 503 的 body 沿用同构 JSON（`{"status":"draining",...}`） | Code Examples /healthz | 低——D-11 只锁「503 draining」语义，body 形态 Discretion |
| A6 | attach 事件建议携 `mode` 字段（ro/rw） | Code Examples 事件行示例 | 低——D-17/D-20 未锁 attach 附加字段；不加 mode 不影响任何锁定验收 |
| A7 | session_end 的 signal 名未命中映射表时的回退形态（数字 or 不出键） | Code Examples session_end | 低——signalName 未命中仅覆盖罕见信号；exitMessage 的数字回退先例可沿用 |

**除上表外，本 research 全部关键机制声明均为 [VERIFIED: GOROOT/本仓源码 本 session 逐行] 或 [CITED: 官方文档原文]。**

## Open Questions (RESOLVED)

1. **log.go 新文件还是就地 server.go 改？**
   - What we know: logEvent 现居 server.go:1071-1077；origin.go/proxy.go/headers.go 的「一关注点一文件」纪律支持独立 log.go（slog 装配 + stderrW + emit helper 内聚）。
   - What's unclear: D-13 的「内部换实现」字面最小改动是就地改 server.go；独立文件是纯组织偏好。
   - Recommendation: 倾向新 `log.go`（logEvent 迁入，注释头登记 D-13/D-15/D-18 决策号，与 proxy.go 先例同构）——但属 planner 自由裁量，两形态零语义差异。
   - **Resolution (planning 定案）:** 新建 `log.go`——由 08-01-PLAN Task 1 落地（slog 基座 + 动态 stderr writer + logEvent 迁入）。

2. **metrics handler 与 healthz handler 同文件（ops.go）还是分文件（metrics.go/health.go）？**
   - What we know: 两 handler 数据源不同（registry 快照 vs atomic 状态位），但同属「运维端点」关注点。
   - Recommendation: 单 `metrics.go` 装 exposition 三件套 + 独立小文件或同文件装 healthz——planner 定；验收不锚文件名。
   - **Resolution (planning 定案）:** 分文件——`health.go`（08-03-PLAN）/ `metrics.go`（08-04-PLAN）。

3. **UAT 中 503 draining 的断言窗口有多宽？**
   - What we know: Shutdown 全程 = 1001 广播（Close 内建 5s+5s 上界）+ stop-signal 序列（默认 HUP 无 timeout → 子进程速死 → 进程退出）；draining 窗口 = SIGTERM 到进程退出，默认配置下可能 <1s。
   - What's unclear: phase08.mjs 需要在窗口内完成一次 /healthz 请求——用 `--stop-timeout 3s` 拉长窗口（07 落地的 flag，延迟 KILL 补发）是最稳的夹具形态。
   - Recommendation: UAT 场景用 `--stop-timeout 3` spawn，SIGTERM 后立即轮询 /healthz 断言 503（phase07.mjs S5 stop-signal 宽限夹具先例同思路）。
   - **Resolution (planning 定案）:** 采纳推荐——phase08.mjs S4 用 `--stop-timeout 3` 拉宽断言窗口（08-05-PLAN 落地）。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 工具链 | 全部实现与测试 | ✓ | go1.26.3 linux/amd64（go.mod `go 1.26.3` 钉死一致） | — |
| Node.js | phase08.mjs UAT | ✓ | v24.13.0（≥22 原生 WebSocket/fetch 需求满足） | — |
| pnpm | web/ 构建（本 phase 不动 web/，仅兜底验证） | ✓ | 11.21.0（CI 同钉） | — |
| jq | README 运维节检索示例 + 人工日志检查 | ✓ | jq-1.6（/usr/bin/jq） | 无亦可（非交付依赖） |
| Prometheus 实例 | /metrics 真实采集验证 | ✗ | — | UAT 用 Node fetch 解析 exposition 文本断言（不依赖真实 Prometheus）；D-01 前提已确认直采/curl 形态 |
| 浏览器/Playwright | 无（纯服务端 phase，CODEBUDDY.md 双机拓扑下半侧本就不在本机） | — | — | — |

**Missing dependencies with no fallback:** 无（不阻塞执行）。
**Missing dependencies with fallback:** Prometheus 实例 → exposition 文本断言（上面已列）。

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`（-race 强制，CI ci.yml 既定） |
| Config file | 无独立配置——`go.mod` + CI `go test -race -count=1 -v ./...` |
| Quick run command | `go test ./internal/server/ -run 'TestHealth|TestMetrics|TestEvent|TestLogEvent|TestAttach|TestDetach|TestSession|TestShutdown|TestThrottled|TestAuthFailed' -count=1` |
| Full suite command | `go vet ./... && go test -race -count=1 ./...` |
| UAT command | `node web/uat/phase08.mjs [wesh 二进制路径]`（惯例：默认 /tmp/wesh-uat/wesh） |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OPS-06 | /healthz 200+JSON 四字段（status/clients/max_clients/session_active） | Go 黑盒（httptest 或 startTestServerWith + http.Get） | `go test ./internal/server/ -run TestHealthz -count=1` | ❌ Wave 0 |
| OPS-06 | /healthz 免认证（凭据模式下无 Authorization 头仍 200） | Go 黑盒（凭据实例 + 无头请求） | 同上表驱动一行 | ❌ Wave 0 |
| OPS-06 | /healthz 根路径固定（bp=/wesh 实例下 /healthz 仍可达、/wesh/healthz 不可达） | Go 黑盒 + UAT | `go test ./internal/server/ -run TestHealthzBasePath -count=1` | ❌ Wave 0 |
| OPS-06 | 关停中 503 draining（Shutdown 置位后） | Go 集成（调 srv.Shutdown() 后请求）+ UAT（SIGTERM + --stop-timeout 窗口） | `go test ./internal/server/ -run TestHealthzDraining -count=1` | ❌ Wave 0 |
| OPS-07 | /metrics exposition 格式合法（HELP/TYPE/样本行/末行 \n/Content-Type） | Go 白盒（snapshotMetrics + writer 输出逐行断言） | `go test ./internal/server/ -run TestMetricsExposition -count=1` | ❌ Wave 0 |
| OPS-07 | 五类指标数值正确（连接 gauge/累计/踢出/收发字节/outbox max·sum） | Go 黑盒（起两客户端 + 踢一 + 灌输出后 GET /metrics 解析） | `go test ./internal/server/ -run TestMetricsValues -count=1` | ❌ Wave 0 |
| OPS-07 | /metrics 认证闸两态（凭据模式 401/200、无认证模式直通） | Go 黑盒 | `go test ./internal/server/ -run TestMetricsAuth -count=1` | ❌ Wave 0 |
| OPS-07 | build_info 携 version label | Go 白盒（escLabel 转义表驱动） | `go test ./internal/server/ -run TestBuildInfo -count=1` | ❌ Wave 0 |
| OPS-08 | logEvent 输出合法 JSON 单行（msg/event/remote/code 四键 + time/level 默认键） | Go（captureStderr + parseEvents，Pattern 5） | `go test ./internal/server/ -run TestLogEventJSON -count=1` | ❌ Wave 0（helper 新写） |
| OPS-08 | attach/detach 事件携 client_id 且同连接可关联（attachSeq 一致） | Go 黑盒（dialHello → close → parseEvents 关联断言） | `go test ./internal/server/ -run TestAttachDetachEvents -count=1` | ❌ Wave 0 |
| OPS-08 | detach reason 四值（normal/kick/pong_timeout/shutdown） | Go 黑盒 ×4（kick 复用 slowclient 夹具、pong_timeout 复用 keepalive 夹具、shutdown 复用 shutdown_test 夹具） | `go test ./internal/server/ -run TestDetachReason -count=1` | ❌ Wave 0 |
| OPS-08 | session_end 三字段（exit_code/signal/duration_seconds） | Go（exitf 桩实例 + 子进程 exit 42 / kill -HUP 两形态） | `go test ./internal/server/ -run TestSessionEnd -count=1` | ❌ Wave 0 |
| OPS-08 | throttled 携 retry_after；auth_failed 不含用户名（SEC-01 回归） | Go（auth_e2e 既有场景迁移） | `go test ./internal/server/ -run TestAuthEvents -count=1` | ❌ Wave 0（迁移既有） |
| OPS-08 | 用户可控字段控制字符剥离（remote 经 XFF 注入 NEL/C1 探针） | Go + UAT（phase07 S4c 探针构造先例迁移） | `go test ./internal/server/ -run TestRemoteSanitize -count=1` | ❌ Wave 0 |
| OPS-08 | 凭据/ticket/token 全文缺席（红线反断言） | Go（auth_e2e/proxy_e2e 既有负断言，逐字保留子串形态） | 既有测试迁移后保持 | ✅（迁移，非新建） |
| OPS-06/07/08 | 全链 UAT（场景矩阵见 CONTEXT Discretion） | UAT phase08.mjs | `node web/uat/phase08.mjs` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/server/ -count=1`（涉及包级快速回归）
- **Per wave merge:** `go vet ./... && go test -race -count=1 ./...`
- **Phase gate:** 全量 -race + `node web/uat/phase08.mjs` 全绿 + 既有 UAT 脚本（phase02..07）回归全绿（六段式纪律）→ `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/server/` 新测试文件（healthz/metrics/events 三组，命名随 plan）
- [ ] `parseEvents` helper（Pattern 5）——limits_test.go 或新 log_test helper 文件；captureStderr 本体零改动
- [ ] 既有断言迁移清单（本 research Runtime State Inventory 末行全量盘点：limits_test.go:144-149、emptyexit_test.go:275-281、auth_e2e_test.go:457-474、proxy_e2e_test.go:86-95/325-347、multi_test.go:837）
- [ ] UAT 脚本迁移：phase05.mjs S6（`code=1013 && reason=slow_consumer` 行断言 → JSON detach 事件 reason=kick + 1013 关闭帧既有断言保持）、phase07.mjs S4（remote=/remote_user=/reason= 子串 → JSON 字段解析）；phase08.mjs 新建
- [ ] 无框架安装需求（stdlib + 既有 Node 零依赖纪律）

## Security Domain

### Applicable ASVS Categories（Level 1 基线）

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | /metrics 跟随 basicAuth 闸（D-08；crypto/subtle 常数时间比较既有）；/healthz 免认证窄例外（D-07 双前提：探活器结构性带不了凭据 + 零敏感信息） |
| V3 Session Management | no | 无 HTTP 会话概念（WS ticket 一次性，P3 既定，本 phase 不触） |
| V4 Access Control | yes | D-07 窄例外不得蔓延（README 明示防例外蔓延）；D-09 根路径固定 + 拒绝双挂（单侧定义）；405 fallback 防方法面静默穿透（Pitfall 7） |
| V5 Input Validation | yes | XFF 链首/remote_user 的 C0/C1/DEL 剥离（D-19 + proxy.go:55-67 既有实现；JSON 化不消除 C1 穿透——Pitfall 5）；escLabel 处理 version label（build_info 唯一 label） |
| V6 Cryptography | no | 无新密码学面 |
| V7 Error Handling and Logging | yes | 凭据/token/ticket 任何形态（含 base64、含用户名）永不入日志（SEC-01/D-23；负断言测试迁移后逐字保留）；事件行零凭据面 + metrics label 零身份面（D-02/D-06 隐私纪律）；启动行 token 保持 stdout 人读文本（D-14 既有授权面，不进事件流） |
| V9 Communications | yes | /metrics 在 TLS 部署下同享 HSTS（securityHeaders 包裹全部路由，server.go:473 既定——新端点自动继承）；明文 HTTP 暴露 /metrics 的轮廓泄漏由 D-08 认证闸承担 |

### Known Threat Patterns for Go stdlib 运维端点 + JSON 日志栈

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 日志注入伪造事件行（C1/NEL 经 XFF/auth-header 首段注入） | Tampering | sanitizeRemoteUser 同款清洗落 remote 字段（D-19；encoding/json 只转义 C0 的实证见 Pitfall 5——清洗是唯一防线） |
| 日志成为凭据库（base64 凭据/ticket/token 进 stderr→journald 持久化） | Information Disclosure | SEC-01 红线 + 负断言测试（全文子串扫描，迁移后保留）+ auth_failed 不含用户名（D-23）+ sharetoken.go:12 注释既定「token 永不入 metrics」 |
| metrics label 基数爆炸/隐私泄漏（remote/client_id 进 label） | Information Disclosure | D-02/D-06：聚合 gauge + 无身份 label；build_info 仅 version 单 label |
| /metrics 行为轮廓泄漏（连接数/失败计数被未授权观测） | Information Disclosure | D-08 认证闸跟随 + --no-auth 模式的 README 明示 |
| /healthz 成为枚举 oracle | Information Disclosure | D-10 body 只含粗粒度容量状态（clients/max_clients/session_active），无版本无身份无错误细节（version 只在需认证的 /metrics build_info） |
| Slow loris 打运维端点 | Denial of Service | 既有 `ReadHeaderTimeout: 5s`（main.go:1188）盒住 HTTP 头读取；/metrics 响应体小（~2KB）无 WriteTimeout 需求（WS 长连接语义排斥全局 WriteTimeout，既定决策） |
| 采集器凭据错误触发自锁（429 节流打 Prometheus） | Denial of Service | README 配方明示 + throttled 事件可观测（Pitfall 6） |
| exposition 注入（version label 含 `"`/换行伪造 series） | Tampering | escLabel 三字符转义（Pattern 2；version 由发布构建注入理论可控，helper 是纵深） |

## Sources

### Primary (HIGH confidence)
- **GOROOT go1.26.3 本 session 逐行核实**：
  - `log/slog/json_handler.go:30-41`（NewJSONHandler 构造捕获 writer + 内部 mutex）、`:60-76`（Handle 单 line JSON + 默认键说明）、`:151-152`（SetEscapeHTML(false)）
  - `log/slog/handler.go:135-172`（HandlerOptions 三字段）、`:177-185`（TimeKey="time"/LevelKey="level"/MessageKey="msg"）、`:318`（行尾 \n）、`:614-622`（RFC3339 毫秒）
  - `log/slog/logger.go:187`（!BADKEY 语义）、`attr.go:18-44`（slog.String/Int/Int64/Bool 构造器）
  - `encoding/json/encode.go:1000-1060`（转义实现：C0→\u00XX、C1 穿透、U+2028/2029、无效 UTF-8→\ufffd）
  - `runtime/debug.go:179`（NumGoroutine）、`runtime/mstats.go:55-78,356`（MemStats.Alloc/Sys、ReadMemStats）
- **本仓源码本 session 逐行核实**（全部附行号于正文）：
  - server.go:1071-1077（logEvent 现状——CONTEXT 标 auth.go 有误）、391-474（Handler 装配）、638-800（Attach 事件站点）、1167-1286（lifecycle/Shutdown/exiting）、1099-1162（signalName/exitMessage）、98-103（inputDrops）
  - clients.go:132-138（outbox）、197-216（droppedInputs）、241-302（registry n/kicks/gateTransitions/attachSeq）、354-368（onChunk 锁序）、409-526（kickOrCreditLocked/afterDrain/kickSlowConsumerLocked）、633-679（writer/inputWriter）、690-713（detach）
  - proxy.go:55-125（sanitizeRemoteUser/proxyInfo）
  - auth.go:101-123（basicAuth 429/401 站点 + retry 现成值）
  - sharetoken.go:122-128（405 fallback 惯例）、main.go:31-32（version）、1102-1206（spawn/New/启动行/信号处理）、1188（ReadHeaderTimeout）
  - limits_test.go:91-111（captureStderr）、e2e_test.go:171-194（startTrackedServerWith 同步纪律）
- **官方规范原文（curl 取自 prometheus/docs 仓 main 分支，本 session 落盘 /tmp/prom_exposition.md、/tmp/prom_naming.md）**：exposition_formats.md（text 0.0.4 全条款，正文逐字引用）、practices/naming.md（前缀/单位/_total/build_info 惯例）

### Secondary (MEDIUM confidence)
- prometheus.io 官方站（WebSearch 结果 #2 确认 exposition_formats 文档存在与官方仓一致——因网络策略 WebFetch 被拦，以官方 GitHub 仓原文为准，等效权威）

### Tertiary (LOW confidence)
- 无（本 research 未采信任何仅 WebSearch 单源的机制声明）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH——零新依赖，全部 stdlib API GOROOT 逐行核实
- Architecture: HIGH——全部挂点/锁序/事件站点/测试迁移面本仓源码逐行核实；CONTEXT 的 logEvent 文件定位错误已纠正（auth.go→server.go:1071）
- Pitfalls: HIGH——Pitfall 1/2/3/5 均有 GOROOT 或本仓 -race 先例级实证；Pitfall 4/6/7 为机制推演但附检测信号
- Prometheus 规范: MEDIUM-HIGH——官方仓原文逐字引用（CITED），classify-confidence seam 对 webfetch 评级 LOW 已记录，以「官方文档一手原文 + 逐字可核对引用」上浮呈现

**Research date:** 2026-08-27
**Valid until:** 2026-09-26（30 天——stdlib 与十年稳定规范，快变面为零）
