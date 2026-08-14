# Phase 2: 协议基线 - Context

**Gathered:** 2026-08-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 2 把 WS 协议层一次性设计到位：版本化子协议 `wesh.v1`、Hello/Welcome/Error 握手、类型化错误帧、三层资源上限（单帧/分片数/累积字节）、合规关闭码全集、默认只读、ping/pong 保活——预认证攻击面在结构上消除。**事后补洞要动协议的东西本 phase 定死**；纯实现细节留给 research/plan。

**In scope (from ROADMAP):** CORE-04（默认只读）、CORE-06（ping/pong 保活）、SEC-08（认证前零缓冲分配）、RES-01（三层上限）；`proto/` 单一事实源（帧类型、版本、错误码、close code 常量）、Hello/Welcome/Error 握手帧、coder/websocket SetReadLimit、5s 未认证超时、per-IP 未认证连接上限、permessage-deflate 默认关（D-17 已定）。

**Out of scope (本阶段不做):** 认证/ticket/Origin 白名单/TLS（Phase 3）、多客户端 fan-out 与 ro/rw 分享链接（Phase 5）、EXIT 终结帧与断线重连（Phase 6，类型字节本 phase 占住）、配置文件（Phase 7）、结构化日志（Phase 8，本 phase 仅 stderr 单行事件）、TITLE/PREFS 帧（Phase 4，类型字节本 phase 占住）。

</domain>

<decisions>
## Implementation Decisions

### 握手与帧编码
- **D-01:** 控制面与数据面统一 binary 帧 = 1 字节类型 + 载荷；控制帧载荷为 JSON。类型分配：`'H'` Hello（C→S）、`'W'` Welcome（S→C）、`'E'` Error（S→C）、`'0'` INPUT/OUTPUT、`'1'` RESIZE（D-16 沿用）；`'X'` EXIT、`'T'` TITLE、`'P'` PREFS 等字母空间为后续 phase 占住。前后端各一个 switch 分派，proto 包唯一事实源。**ARCHITECTURE.md §2.8 的"控制面 text JSON"方案作废**（调研期语言中立设计，proto.go D-16 预留路线更近、单一路径更简单） — **Reversibility:** one-way — 协议编码是前后端公开契约，上线后改编码即协议破坏，所有端须同步升级。
- **D-02:** Hello 最小 schema `{version, cols, rows}`；协议纪律钉死**未知字段必须忽略**（前后端同纪律）。Phase 3 加 `ticket`、Phase 5 加 `attach`/`mode` 只是加字段，向后兼容不算动协议 — **Reversibility:** one-way — "忽略未知字段"是演化承诺，任何一端变成严格校验即破坏整个扩展机制。
- **D-03:** 子协议协商在 HTTP 层拦截：`Accept` 前检查 `Sec-WebSocket-Protocol` 头，不含 `wesh.v1` 返回 HTTP 400（零 WS 资源分配，扫描器/旧客户端最早被拦）；`Accept` 后 `c.Subprotocol()` assert 兜底。前端 `new WebSocket(url, ['wesh.v1'])`。
- **D-04:** per-IP 未认证（Hello 未完成）半开连接上限 **8**，超限 Accept 前 HTTP 429；5s 未认证超时（ROADMAP 已定）；Hello 前收到任何数据帧（抢跑帧）按协议违规关闭（码值见 D-05）。正常浏览器秒发 Hello 不受限；NAT 多人场景 Hello 已完成不计入。

### 关闭码与错误码
- **D-05:** 关闭码全集 = `{1000 正常, 1001 服务端下线, 1002 协议错误(未知帧/抢跑/畸形), 1008 策略违反(认证/权限/版本), 1009 超限, 1011 内部错误, 1013 踢出可重试}`，1005/1006/1015 永不发送。ROADMAP 成功准则 2 的集合 `{1000,1008,1009,1011,1013}` 漏写 1002（Phase 1 已在用，server.go:114）——以本并集为准，ROADMAP 待校正 — **Reversibility:** one-way — 关闭码是客户端可观测契约（前端按码分派文案），集合语义变化影响所有端。
- **D-06:** Error 帧按受众分治——正常客户端会遇到的错误发 `Error{code,message}` + 关闭码：`version_mismatch`(1008)、`server_error`(1011)；攻击面路径直接关闭码不发 Error：`unknown_frame`/抢跑帧(1002)、超限(1009，库自动)——不给攻击者反馈面。
- **D-07:** Error JSON = `{code, message}`：code 机器可读 snake_case，message 英文人话（前端直接展示，与现有文案风格一致）；所有主动关闭的 close reason 带同名机器串（RFC ≤123 字节），抓包/devtools 可辨 — **Reversibility:** costly — 前端直接展示 message 并依赖 code 分派，code 语义变更牵动前后端两侧。
- **D-08:** 1001（Phase 7 优雅下线）与 1013（Phase 5 背压踢出）进 proto 常量表**占位不实现**，注释标注启用 phase；Phase 2 只产生 1000/1002/1008/1009/1011 发送路径。纪律：应用层超限检测（分片数/累积字节）复用 1009，不得发明新码或自定义 4000 段。

### 三层资源上限
- **D-09:** 三层上限初始值：单帧 **16KiB** / 每消息分片数 **32** / 每消息累积字节 **16KiB**（C→S）。依据：合法流量极小（键盘 INPUT 字节级、RESIZE/Hello JSON <200B、粘贴几 KB，浏览器 WS API 不分片）；分片数 32 对空帧攻击（0 字节 continuation 不累积字节）是关键防线。ttyd 对照已核实无参考价值——其三层全零即两个预认证漏洞本体（protocol.c:288-298）。数值经 research + 负载测试标定（research flag），Phase 9 回填默认值。
- **D-10:** 上限全部**常量**进 proto/server 包（注释标定来源与依据），**不开 CLI flag**；Phase 7 配置文件（OPS-09）统一收口可配性。
- **D-11:** SetReadLimit 两档切换：`Accept` 后先 `SetReadLimit(4KiB)`（Hello JSON ~100 字节，余量两个数量级），Hello/Welcome 完成后 `SetReadLimit(16KiB)`——SEC-08 预认证窗口单连接可占内存最小化。

### 超限提示（用户明确要求：不得吞错误）
- **D-12:** 库自动 1009 无机会补 Error 帧（D-06 已定），提示走三条腿：① 前端 `onclose` 按 1009 分派人话文案（"超出服务端消息上限"类，不提 flag——本 phase 无可调 flag）；② 服务端 stderr 打单行事件（对端、码值、reason）——现状服务端除启动行外零输出，Phase 8 才升级为结构化日志（OPS-08）；③ close reason 带机器串（`message_too_big`/`fragment_limit`）。

### 只读模式与保活
- **D-13:** ro 边界 = **只丢弃 INPUT，RESIZE 放行**（单客户端窗口拖动仍同步；RESIZE 只改视图尺寸不改 shell 输入；与 ttyd `-R` 行为一致）。Phase 5 多客户端时 RESIZE 才收写权限门 + 最小公共矩形仲裁（MULTI-04）。
- **D-14:** Welcome 帧带 `mode`（`"ro"`/`"rw"`）；ro 时前端 `term.options.disableStdin = true`（键盘不产生 onData，前端层面即不发）+ `document.title` 加 `"[ro] "` 前缀。零新 UI 组件；Phase 4 TITLE 同步（CORE-03）接管标题时再融合。
- **D-15:** 可写 flag = `--writable`（布尔，help: "allow client input (default read-only)"），全名无短选项与现有 flag 风格一致 — **Reversibility:** one-way — CLI flag 是公开契约（同 D-02 理由），发布后改名破坏脚本与文档。
- **D-16:** 保活参数：ping 间隔默认 **5s**（ttyd 生产验证值，对 nginx 60s / Cloudflare 100s / 30s 型 ingress 均"显著小"）+ `--ping-interval` flag（0=禁用）+ pong 超时 **10s** 常量（发出 ping 后等 pong 的时长，正常 RTT 毫秒级，10s 极宽） — flag 名 **Reversibility:** one-way（同 D-15）；默认值 reversible。

### Claude's Discretion
- proto 包内部组织（常量分组、Error code 字符串常量表、JSON 编解码函数签名）以保持单一事实源原则为准。
- per-IP 半开计数器的数据结构（map + 计数 + 清理时机）与 5s 计时器的挂点由 planner 定。
- 前端 onclose 各码值的具体英文文案（沿用现有 showStatus 三态面板风格）。
- 服务端 stderr 单行事件的具体格式（Phase 8 会被 slog 结构化日志取代）。
- ping goroutine 与现有单 reader/单 writer 结构的装配方式（coder/websocket Ping API 的具体用法以 research 为准）。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 2 — 成功准则 3 条与 research flag（三层上限需实测标定）；准则 2 关闭码集合漏写 1002，以本文件 D-05 并集为准
- `.planning/REQUIREMENTS.md` — CORE-04（默认只读）、CORE-06（ping/pong 保活）、SEC-08（认证前零缓冲）、RES-01（三层上限）
- `.planning/PROJECT.md` — 约束（单静态二进制、Go、xterm.js）与 Key Decisions 表

### 调研结论
- `.planning/research/ARCHITECTURE.md` §2.8 — 协议设计（**text JSON 方案已作废**，以 D-01 binary 统一分派为准；版本化/ticket 演化方向仍有效）；§2.9 写权限与 resize 仲裁（Phase 5 语境，ro 边界以 D-13 为准）
- `.planning/research/PITFALLS.md` — Pitfall 1（手写分片重组/预认证无上限，三层限制齐下）、Pitfall 9（RFC6455 合规、关闭码映射、wasClean 判断）、§验证清单（资源上限/关闭路径验证条目：空帧/百万小帧/超限帧模糊测试、Autobahn 套件）

### Phase 1 决策与现状代码
- `.planning/phases/01-pty/01-CONTEXT.md` — D-16（帧形状与类型空间预留）、D-17（SetReadLimit 基线、permessage-deflate 关）
- `internal/proto/proto.go` — 帧常量现状（'0'/'1'）与 'E'/'X' 预留注释、ClampDim；本 phase 在此扩展
- `internal/server/server.go` — Attach 现状：409 原子门（D-09，Phase 5 才改）、1002 未知帧、SetReadLimit 库默认 32768 待显式化、单 reader 循环
- `cmd/wesh/main.go` — parseArgs 现状（--port/--bind/--version），本 phase 加 --writable/--ping-interval
- `web/src/main.ts` — 前端 WS 现状：arraybuffer + buf[0] switch、onclose 1000/其他两分法待按码分派扩展

### ttyd 源码（缺陷对照面，不参考实现）
- `~/open_src/ttyd/src/protocol.c:288-298` — 预认证分片无限累积 + 空帧空指针（本 phase 三层上限的反面教材，已核实无参考价值）
- `~/open_src/ttyd/src/server.c:331` — max_http_header_data 65535（lws 头上限对照）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/proto/proto.go` — 常量与编解码集中点；D-01 的类型字节（'H'/'W'/'E'）与 Error code 表落此文件，前端 TS 常量手工对齐纪律沿用（D-16 注释模式）
- `internal/server/server.go` Attach — 单 reader 循环与 switch 分派骨架直接扩展：Hello 首帧校验、ro 模式 INPUT 丢弃分支、抢跑帧关闭
- `web/src/main.ts` — `concat()`、`buf[0]` switch、`showStatus()` 三态面板复用；onclose 扩展为按码分派
- `s.frame` 组帧缓冲模式（server.go:48）— OUTPUT 外的新 S→C 帧（Welcome/Error）组帧可复用同款 1+payload 模式

### Established Patterns
- **exitf 注入 + sync.Once 收口**（server.go:37,165）— 生命周期必须可测的硬约束；新增的 5s 未认证超时、ping goroutine 的终结也要进同一生收口
- **库默认可靠即不自建** — SetReadLimit 自动 1009、UTF-8/mask 合规由 coder/websocket 保证；应用层只补库没有的（分片数上限、子协议预检、per-IP 计数）
- **原子门 + atomic.Pointer 连接持有** — attached/conn/childExited 现状模式；ro 标志、握手完成标志沿用 atomic

### Integration Points
- `websocket.AcceptOptions{Subprotocols: []string{"wesh.v1"}}` — 一行开启子协议协商（D-03 的 assert 兜底挂点）；预检在 Accept 前的 HTTP 层（Attach handler 开头）
- SetReadLimit 两档挂点：Accept 成功后 4KiB → Welcome 发出后 16KiB（D-11）
- per-IP 半开计数 + HTTP 429 在 Accept 前；与子协议 400 预检同属 Attach 前置守卫区
- `cmd/wesh/main.go` parseArgs — 加 `--writable`/`--ping-interval`，config 结构体扩展
- 前端 `new WebSocket(url, ['wesh.v1'])`、发送 Hello 首帧、Welcome 处理（mode → disableStdin/title）、onclose 码值分派

</code_context>

<specifics>
## Specific Ideas

- **ttyd 三层全零对照结论**：用户要求查 ttyd 数值作参考——核实结果是 ttyd C→S 无任何上限（protocol.c:288-296 无限 xrealloc 累积、298 空帧空指针），"数值"即漏洞本体，反面教材价值大于参考价值。wesh 取值逻辑从"合法流量有多大"出发（C→S 全小消息）。
- **超限不得吞错误**（用户原话要求）：三腿提示（D-12）是硬要求，前端文案、stderr 事件、reason 机器串缺一不可；测试要断言 1009 路径的可见性。
- **分歧裁决记录**：ARCHITECTURE §2.8 text JSON 方案作废（D-01）；ROADMAP 准则 2 关闭码集合漏写 1002，以 D-05 并集为准（建议后续校正 ROADMAP 原文）。
- 前端文案继续全英文（现状一致）。

</specifics>

<deferred>
## Deferred Ideas

- **1001 发送路径**（优雅下线）— Phase 7 信号处理落地时实现（本 phase 常量占位）。
- **1013 发送路径**（慢客户端背压踢出）— Phase 5 outbox/背压落地时实现（本 phase 常量占位）。
- **三层上限可配性** — Phase 7 配置文件（OPS-09）统一收口；本 phase 常量。
- **Error code 扩展** — `auth_failed`/`permission_denied`（Phase 3 认证、Phase 5 权限）随各自 phase 加入 code 表。
- **EXIT/TITLE/PREFS 帧实现** — 类型字节本 phase 占住（D-01），语义实现分属 Phase 6/4。
- **ROADMAP.md 准则 2 校正** — 关闭码集合补上 1002（与 PITFALLS 映射表对齐），下次 roadmap 维护时处理。

</deferred>

---

*Phase: 2-protocol*
*Context gathered: 2026-08-15*
