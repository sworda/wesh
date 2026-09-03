# Phase 3: 认证与传输安全 - Context

**Gathered:** 2026-08-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 3 把认证与传输安全做到"敢暴露到公网"标准，修复 ttyd 已核实的 C6 认证连环错全套（strcmp 时序泄露、凭据 base64 进日志、/token 明文下发、AuthToken 与 Basic 复用、无节流）。ROADMAP 锁定项（不重复决策）：已认证 HTTP `POST /api/attach` 换一次性 ticket（单次使用、60s TTL、绑定权限级别、128bit 随机、与静态凭据独立 secret），WS Hello 首帧核销、重放拒绝；爆破 100 次触发指数退避；`crypto/subtle` 常数时间比较（先 SHA-256 哈希等长）；凭据/ticket/Authorization 头任何形态（含 base64）不进任何日志且有脱敏测试；Origin 不在白名单拒绝握手；TLS 仅 1.2+（默认 1.3）、安全响应头、testssl.sh 无弱项。

**In scope (from ROADMAP):** SEC-01（时序安全比较+日志红线）、SEC-02（一次性 ticket）、SEC-03（失败节流）、SEC-04（Origin 白名单）、SEC-05（TLS 加固+安全头）；`/api/attach` 端点、整站 Basic、Hello 增 ticket 字段、`auth_failed` Error code 兑现（P2 deferred 挂账）、5 个新 CLI flag。

**Out of scope (本阶段不做):** ro/rw 分别签发 ticket 与分享链接（Phase 5 MULTI-05）、auth-header 反代透传 SEC-07 与 X-Forwarded-For 信任（Phase 7）、配置文件收口（Phase 7 OPS-09）、metrics 失败计数与 slog 结构化日志（Phase 8）、ACME 自动证书（v2 V2-ACME）、多客户端 fan-out 与背压（Phase 5）。

</domain>

<decisions>
## Implementation Decisions

### 凭据形态与认证入口
- **D-01:** 凭据双通道：`--credential user:pass` 可重复 flag（多组凭据支持按人撤销，常数时间比较逐组轮询）+ `WESH_CREDENTIAL` env 兜底，flag 优先（systemd `EnvironmentFile=` 600 路径，PITFALLS 部署矩阵） — **Reversibility:** one-way — CLI flag/env 名是公开契约（P2 D-15 同纪律），发布后改名破坏脚本与 systemd unit
- **D-02:** 整站 Basic：`/` 与 `/api/attach` 均挂 401 challenge（WWW-Authenticate），浏览器原生弹窗输一次凭据后同源 fetch 自动带缓存凭据；前端零新 UI 组件，仅加"fetch ticket → Hello 携带"逻辑；静态页本身受保护 = 纵深防御（ttyd 同形态）
- **D-03:** 裸跑收口：bind loopback 且无凭据 → 放行免警告；bind 非 loopback（含默认 0.0.0.0）且无凭据 → 拒绝启动，需显式 `--no-auth` 放行。Phase 1 D-05 的临时取舍在此收口 — **Reversibility:** one-way — flag 契约同上；行为变更影响 Phase 1/2 既有用法（`wesh -- bash` 裸跑需 `--no-auth` 或凭据），README 必须明示

### TLS 形态与证书来源
- **D-04:** 显式证书才开 TLS：`--tls-cert`/`--tls-key` 成对给出才 ServeTLS（只给一个 = 启动报错）；否则明文 HTTP + stderr 醒目警告，bind 非 loopback 时警告升级。反代终止 TLS 部署（wesh 自身明文）是常态场景，零意外行为 — **Reversibility:** one-way — flag 契约同上
- **D-05:** 凭据×明文耦合：loopback + 明文 + 凭据 → 放行免警告（流量不出机）；非 loopback + 明文 + 凭据 → 拒绝启动，需显式 `--insecure-http` 放行（与 D-03 `--no-auth` 同构的显式逃生门） — **Reversibility:** one-way — flag 契约同上
- **D-06:** TLS 强度实现目标：MinVersion 1.2、默认协商 1.3、1.2 弱 cipher 剔除；安全响应头集合交 research 按 OWASP 基线定稿——HSTS 仅 TLS 时发送；单文件全内联现实下 CSP 实用化（script/style 'unsafe-inline' 不可避免，default-src/connect-src 收同源、ws/wss 限同源）；frame-ancestors 'none' 兼发 X-Frame-Options: DENY 兼容老浏览器
- **D-07:** TLS 验证形态：Go 测试自动化断言（crypto/tls client MaxVersion 1.1 必败、1.2 必成、弱 cipher 拒绝）+ testssl.sh 进手动 UAT 清单并文档化命令；不进 CI（外部依赖重、与现有轻量 CI 风格不搭）

### 失败节流与核销语义
- **D-08:** 两处失败统一 per-IP 指数退避计数器：`/api/attach` 凭据失败与 Hello ticket 核销失败计入同一 per-IP 计数器；认证成功清零
- **D-09:** 退避参数（初始值/倍率/封顶/是否临时锁定）交 research 标定（参照 fail2ban 生态与同类工具）；验收锚点 = ROADMAP 准则 2"爆破 100 次触发可观测退避"
- **D-10:** 核销失败统一 `Error{code: auth_failed}` + 1008 关闭（close reason 同名机器串，P2 D-07 纪律）——过期/非法/重放同口径，不给攻击者区分 oracle；前端拿 `auth_failed` 机器码自动重取 ticket 静默重试一次（ticket 过期是正常场景：页面放置超 60s）。`auth_failed` 进 proto Error code 表，兑现 P2 deferred 挂账 — **Reversibility:** costly — Error code 入表即前后端公开契约（P2 D-07 同评级），更名/改义牵动两侧
- **D-11:** ticket 绑权限：`/api/attach` 请求体为空，ticket 内部携带 mode 字段（值 = 全局 `--writable` 模式）；Phase 5 分签发时给请求加可选 mode 参数即向后兼容，本 phase 不提前收参

### Origin 白名单语义
- **D-12:** `--origin` 可重复 flag，值为完整 origin（scheme://host[:port]），解析规范化后精确比较；不配 flag 时维持库默认同源校验（零配置场景行为不变）；反代场景显式加公网 origin 解 FEATURES 张力 — **Reversibility:** one-way — flag 契约同上
- **D-13:** 无 Origin 头放行（非浏览器客户端：curl/Node e2e/UAT 零摩擦；CSWSH 威胁模型只约束浏览器，浏览器必发 Origin）；有 Origin 必查（同源或在 `--origin` 列表内）；`/ws` 与 `/api/attach` 均查，静态页 GET 不查（无副作用）

### Claude's Discretion
- ticket 存储数据结构与核销原子性（进程内 map + O(1) 查表删除，ARCHITECTURE 已定方向；128bit crypto/rand、60s TTL、单次核销、独立 secret 为 ROADMAP/PITFALLS 锁定项）
- 无认证模式（--no-auth/loopback 裸跑）下 `/api/attach` 的行为与前端探测顺序（如 404 → 前端跳过 fetch 直连 WS）
- `/api/attach` 实现细节：POST-only（其他方法 405）、body 上限、响应 JSON 字段名
- 退避计数器内存界与清理时机（map + 计数 + 过期清理，防单调增长——halfOpenCounter 同款 Pitfall 4 纪律）
- Origin 规范化比较细节（默认端口省略/大小写/尾斜杠）、Basic realm 字符串、401/403 body 文案
- 日志脱敏测试形态（CI grep base64/凭据样例扫描）、节流/认证失败事件 stderr 单行形态（沿用 logEvent 三要素，Phase 8 升级 slog）
- TLS 1.2 cipher 具体清单（Go stdlib 安全集基础上去弱，以 research 为准）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 3 — 成功准则 3 条（ticket 流程与重放拒绝 / 爆破节流+subtle+日志红线 / Origin+TLS+安全头+testssl.sh）
- `.planning/REQUIREMENTS.md` — SEC-01..SEC-05 原文
- `.planning/PROJECT.md` — Context 节 ttyd C6 认证缺陷清单（行号证据）、Key Decisions 表

### 调研结论
- `.planning/research/ARCHITECTURE.md` §2.8 — ticket 握手认证设计（POST /api/attach → 一次性 ticket → Hello 核销；ticket 不走 URL query、不塞子协议头）；§2.2 Auth Service 职责表（内存票表 + 核销原子性）
- `.planning/research/PITFALLS.md` — C6 认证连环错全套对策（常数时间比较先哈希等长 / 令牌独立 secret / 节流 / 日志红线 / Basic 只走 TLS）、Origin 规范化比较（ttyd -O 反例）、TLS 加固与部署矩阵（systemd EnvironmentFile 600）、§验证清单认证与 TLS 条目
- `.planning/research/FEATURES.md` §功能张力 — base-path/auth-header × Origin 校验张力（反代白名单必须支持显式可信源，D-12 已解）

### 前序 phase 决策
- `.planning/phases/01-pty/01-CONTEXT.md` — D-05（默认 0.0.0.0 的临时取舍，本 phase D-03/D-05 收口）
- `.planning/phases/02-protocol/02-CONTEXT.md` — D-02（Hello 加 ticket 字段 = 纯加字段不破坏协议）、D-04（per-IP 半开帽与 5s 未认证超时——ticket 核销窗口复用）、D-06/D-07（错误分治与 Error JSON 形状，auth_failed 落此规范）、D-11（两档 SetReadLimit——核销在预认证 4KiB 档内完成）、deferred（auth_failed 进 code 表挂账，本 phase 兑现）

### 现状代码（扩展点）
- `internal/proto/proto.go` — Error code 表加 `auth_failed`；Hello 解码加 `ticket` 可选字段（未知字段忽略纪律不受影响）
- `internal/server/server.go` — Attach 守卫区（①子协议 400 ②per-IP 429 ③409）的 Origin 检查挂点；`halfOpenCounter` 为节流计数器参照模式；`logEvent` 三要素为认证失败/节流事件出口
- `cmd/wesh/main.go` — parseArgs 加新 flag；http.Server 分岔 ServeTLS；启动校验矩阵（凭据×bind×TLS）与警示打印
- `web/src/main.ts` — WS 连接前 fetch POST /api/attach 取 ticket（整站 Basic 下同源自动带凭据）；Hello 加 ticket；auth_failed → 自动重取重试一次

### ttyd 源码（缺陷对照面，不参考实现）
- `~/open_src/ttyd/src/server.c:142` — 凭据 base64 明文进日志（日志红线反例）
- `~/open_src/ttyd/src/protocol.c:51-71` — Origin 弱字符串比对可绕过（D-12 规范化精确比较的反例）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `halfOpenCounter`（server.go:136-162）— per-IP 计数器 + 恰好一次不变量模式，节流计数器直接参照（含 map 到 0 删 key 防单调增长纪律）
- `headerHasToken`（server.go:179-188）— 逗号分隔头逐 token 精确比较（Pitfall 5 纪律），Authorization/WWW-Authenticate 头解析同款要求
- `logEvent`（server.go:441-443）— stderr 单行事件三要素（remote/code/reason）统一出口；**凭据/ticket/Authorization 永不入参**
- `clientIP`（server.go:168-174）— per-IP 键提取现状（反代下聚合为代理 IP 是已知限制，X-Forwarded-For 属 Phase 7）

### Established Patterns
- **守卫区顺序敏感**（Attach ①子协议 400 → ②per-IP 429 → ③409，Accept 前零 WS 资源分配）— Origin 检查加入此守卫区；`/api/attach` 独立小守卫链（POST-only → Origin → 节流 → Basic 校验）
- **库默认可靠即不自建** — TLS 用 stdlib crypto/tls 声明式 tls.Config（版本/cipher 下限），不手写握手；随机数用 crypto/rand
- **exitf 注入 + sync.Once 收口**（P1 硬约束）— 新端点/新计时器不得新增 exitf 分支
- **CLI flag 全名无短选项**（P2 D-15）— 新 flag 同纪律

### Integration Points
- `proto.go` — Hello 解码加 `ticket` 可选字段；Error code 表加 `auth_failed`；前端 TS 常量手工对齐纪律沿用
- `Attach` 升档序列（server.go:304-330）— ticket 核销插入 Hello 校验段（version 检查之前/之后由 planner 定，失败统一 D-10 口径）；核销成功才走既有升档
- `main.go run()` — TLS 分岔（ServeTLS vs Serve）、启动校验矩阵（D-03/D-05 拒绝路径+错误文案）、启动行/警示打印
- `web/src/main.ts` — 连接流程改造：fetch ticket → new WebSocket → Hello{version,ticket,cols,rows}；onclose/Error 按 `auth_failed` 分派自动重试一次

</code_context>

<specifics>
## Specific Ideas

- **显式逃生门风格**：D-03 `--no-auth` 与 D-05 `--insecure-http` 同构——"我知道我在裸奔/明文"必须显式说出口；默认 bind 0.0.0.0 下 Phase 1/2 的裸跑用法行为变更，README 与 help 文案必须明示（延续 P1"单次语义不是产品形态"的明示传统）
- **auth_failed 自动重试仅一次**：前端拿机器码自动重取 ticket 静默重试一次，失败才展示人话；非无限循环
- **日志红线**：凭据、ticket、Authorization 头任何形态（含 base64）永不进任何日志；脱敏测试是 ROADMAP 准则 2 的硬验收
- **反代张力已闭合**：FEATURES.md 标记的 base-path/auth-header × Origin 张力由 D-12 显式可信源解决；Phase 7 base-path 落地时不得推翻

</specifics>

<deferred>
## Deferred Ideas

- **ro/rw 分别签发 ticket 与分享链接** — Phase 5 MULTI-05；本 phase ticket 内部 mode 字段已占位（D-11）
- **permission_denied Error code** — Phase 5 权限场景加入 code 表（P2 deferred 同批挂账）
- **X-Forwarded-For/可信代理解析与 auth-header 透传（SEC-07）** — Phase 7；server.go 现行注释误写"Phase 3 SEC-07"，本 phase 顺手校正注释
- **节流失败计数进 metrics / 审计事件 slog 结构化** — Phase 8（PITFALLS"失败计数进 metrics"挂账）
- **ACME/Let's Encrypt 自动证书** — v2（V2-ACME）
- **flag 可配性收口进配置文件** — Phase 7（OPS-09）

</deferred>

---

*Phase: 3-auth*
*Context gathered: 2026-08-17*
