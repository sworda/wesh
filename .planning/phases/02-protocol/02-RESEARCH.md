# Phase 2: 协议基线 - Research

**Researched:** 2026-08-14
**Domain:** WebSocket 协议层（握手/版本化/资源上限/关闭码合规/保活）——Go coder/websocket v1.8.15 + RFC 6455
**Confidence:** HIGH（coder/websocket 全部关键行为——SetReadLimit 语义、分片重组路径、Ping 语义、子协议协商、关闭码线值校验、压缩默认值——均经模块缓存 v1.8.15 源码逐行核实并注出行号；Bandit CVE 经官方 advisory 原文核实；唯一 MEDIUM 区是少量实现参数推荐值）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**握手与帧编码**
- **D-01:** 控制面与数据面统一 binary 帧 = 1 字节类型 + 载荷；控制帧载荷为 JSON。类型分配：`'H'` Hello（C→S）、`'W'` Welcome（S→C）、`'E'` Error（S→C）、`'0'` INPUT/OUTPUT、`'1'` RESIZE（D-16 沿用）；`'X'` EXIT、`'T'` TITLE、`'P'` PREFS 等字母空间为后续 phase 占住。前后端各一个 switch 分派，proto 包唯一事实源。**ARCHITECTURE.md §2.8 的"控制面 text JSON"方案作废**（调研期语言中立设计，proto.go D-16 预留路线更近、单一路径更简单） — **Reversibility:** one-way
- **D-02:** Hello 最小 schema `{version, cols, rows}`；协议纪律钉死**未知字段必须忽略**（前后端同纪律）。Phase 3 加 `ticket`、Phase 5 加 `attach`/`mode` 只是加字段，向后兼容不算动协议 — **Reversibility:** one-way
- **D-03:** 子协议协商在 HTTP 层拦截：`Accept` 前检查 `Sec-WebSocket-Protocol` 头，不含 `wesh.v1` 返回 HTTP 400（零 WS 资源分配，扫描器/旧客户端最早被拦）；`Accept` 后 `c.Subprotocol()` assert 兜底。前端 `new WebSocket(url, ['wesh.v1'])`。
- **D-04:** per-IP 未认证（Hello 未完成）半开连接上限 **8**，超限 Accept 前 HTTP 429；5s 未认证超时（ROADMAP 已定）；Hello 前收到任何数据帧（抢跑帧）按协议违规关闭（码值见 D-05）。正常浏览器秒发 Hello 不受限；NAT 多人场景 Hello 已完成不计入。

**关闭码与错误码**
- **D-05:** 关闭码全集 = `{1000 正常, 1001 服务端下线, 1002 协议错误(未知帧/抢跑/畸形), 1008 策略违反(认证/权限/版本), 1009 超限, 1011 内部错误, 1013 踢出可重试}`，1005/1006/1015 永不发送。ROADMAP 成功准则 2 的集合 `{1000,1008,1009,1011,1013}` 漏写 1002（Phase 1 已在用，server.go:114）——以本并集为准，ROADMAP 待校正 — **Reversibility:** one-way
- **D-06:** Error 帧按受众分治——正常客户端会遇到的错误发 `Error{code,message}` + 关闭码：`version_mismatch`(1008)、`server_error`(1011)；攻击面路径直接关闭码不发 Error：`unknown_frame`/抢跑帧(1002)、超限(1009，库自动)——不给攻击者反馈面。
- **D-07:** Error JSON = `{code, message}`：code 机器可读 snake_case，message 英文人话（前端直接展示，与现有文案风格一致）；所有主动关闭的 close reason 带同名机器串（RFC ≤123 字节），抓包/devtools 可辨 — **Reversibility:** costly
- **D-08:** 1001（Phase 7 优雅下线）与 1013（Phase 5 背压踢出）进 proto 常量表**占位不实现**，注释标注启用 phase；Phase 2 只产生 1000/1002/1008/1009/1011 发送路径。纪律：应用层超限检测（分片数/累积字节）复用 1009，不得发明新码或自定义 4000 段。

**三层资源上限**
- **D-09:** 三层上限初始值：单帧 **16KiB** / 每消息分片数 **32** / 每消息累积字节 **16KiB**（C→S）。依据：合法流量极小（键盘 INPUT 字节级、RESIZE/Hello JSON <200B、粘贴几 KB，浏览器 WS API 不分片）；分片数 32 对空帧攻击（0 字节 continuation 不累积字节）是关键防线。ttyd 对照已核实无参考价值——其三层全零即两个预认证漏洞本体（protocol.c:288-298）。数值经 research + 负载测试标定（research flag），Phase 9 回填默认值。
- **D-10:** 上限全部**常量**进 proto/server 包（注释标定来源与依据），**不开 CLI flag**；Phase 7 配置文件（OPS-09）统一收口可配性。
- **D-11:** SetReadLimit 两档切换：`Accept` 后先 `SetReadLimit(4KiB)`（Hello JSON ~100 字节，余量两个数量级），Hello/Welcome 完成后 `SetReadLimit(16KiB)`——SEC-08 预认证窗口单连接可占内存最小化。

**超限提示（用户明确要求：不得吞错误）**
- **D-12:** 库自动 1009 无机会补 Error 帧（D-06 已定），提示走三条腿：① 前端 `onclose` 按 1009 分派人话文案（"超出服务端消息上限"类，不提 flag——本 phase 无可调 flag）；② 服务端 stderr 打单行事件（对端、码值、reason）——现状服务端除启动行外零输出，Phase 8 才升级为结构化日志（OPS-08）；③ close reason 带机器串（`message_too_big`/`fragment_limit`）。

**只读模式与保活**
- **D-13:** ro 边界 = **只丢弃 INPUT，RESIZE 放行**（单客户端窗口拖动仍同步；RESIZE 只改视图尺寸不改 shell 输入；与 ttyd `-R` 行为一致）。Phase 5 多客户端时 RESIZE 才收写权限门 + 最小公共矩形仲裁（MULTI-04）。
- **D-14:** Welcome 帧带 `mode`（`"ro"`/`"rw"`）；ro 时前端 `term.options.disableStdin = true`（键盘不产生 onData，前端层面即不发）+ `document.title` 加 `"[ro] "` 前缀。零新 UI 组件；Phase 4 TITLE 同步（CORE-03）接管标题时再融合。
- **D-15:** 可写 flag = `--writable`（布尔，help: "allow client input (default read-only)"），全名无短选项与现有 flag 风格一致 — **Reversibility:** one-way
- **D-16:** 保活参数：ping 间隔默认 **5s**（ttyd 生产验证值，对 nginx 60s / Cloudflare 100s / 30s 型 ingress 均"显著小"）+ `--ping-interval` flag（0=禁用）+ pong 超时 **10s** 常量（发出 ping 后等 pong 的时长，正常 RTT 毫秒级，10s 极宽） — flag 名 **Reversibility:** one-way；默认值 reversible。

### Claude's Discretion
- proto 包内部组织（常量分组、Error code 字符串常量表、JSON 编解码函数签名）以保持单一事实源原则为准。
- per-IP 半开计数器的数据结构（map + 计数 + 清理时机）与 5s 计时器的挂点由 planner 定。
- 前端 onclose 各码值的具体英文文案（沿用现有 showStatus 三态面板风格）。
- 服务端 stderr 单行事件的具体格式（Phase 8 会被 slog 结构化日志取代）。
- ping goroutine 与现有单 reader/单 writer 结构的装配方式（coder/websocket Ping API 的具体用法以 research 为准）。

### Deferred Ideas (OUT OF SCOPE)
- **1001 发送路径**（优雅下线）— Phase 7 信号处理落地时实现（本 phase 常量占位）。
- **1013 发送路径**（慢客户端背压踢出）— Phase 5 outbox/背压落地时实现（本 phase 常量占位）。
- **三层上限可配性** — Phase 7 配置文件（OPS-09）统一收口；本 phase 常量。
- **Error code 扩展** — `auth_failed`/`permission_denied`（Phase 3 认证、Phase 5 权限）随各自 phase 加入 code 表。
- **EXIT/TITLE/PREFS 帧实现** — 类型字节本 phase 占住（D-01），语义实现分属 Phase 6/4。
- **ROADMAP.md 准则 2 校正** — 关闭码集合补上 1002（与 PITFALLS 映射表对齐），下次 roadmap 维护时处理。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-04 | 默认只读模式（丢弃客户端输入），显式开启可写后才接受输入 | 双层防线：服务端 ro 时 INPUT 帧丢弃（安全边界，server.go switch 加分支）+ 前端 `term.options.disableStdin = true`（UX 层，xterm.d.ts:92 VERIFIED）；RESIZE 放行（D-13）；`--writable` flag（D-15）；Welcome `mode` 字段下发（D-14） |
| CORE-06 | WS ping/pong 保活，间隔可配置 | `c.Ping(ctx)` 源码核实语义：发 ping 阻塞等 pong，须与 Reader 并发（conn.go:216-259 VERIFIED）；pong 超时经 `context.WithTimeout(10s)` 传入（D-16）；超时触发库内 `c.close()` 自动收口现有 reader 终结路径 |
| SEC-08 | 认证完成前零缓冲分配 | 结构性达成：子协议 400 预检/per-IP 429 均在 `Accept` 前（零 WS 对象）；`SetReadLimit(4KiB)` 预认证档（D-11）；库重组为流式 O(1)/帧零累积缓冲（read.go:457-498 VERIFIED）；预认证期唯一分配 = 库固定缓冲（8B 头 buf + 125B 控制 buf + bufio） |
| RES-01 | WS 消息三层上限：单帧长度、分片数量、累积字节数 | 层1+层3 由 `SetReadLimit` 结构覆盖（per-message 跨分片累积限，read.go:88-105 VERIFIED；单帧可读字节隐含 ≤ limit）；层2 分片数库不暴露 → 应用层 `c.Reader` + 全限缓冲计数循环实现（精确对非空分片）+ 每消息完成时限兜底空分片洪水（read.go:31-33 官方指引模式） |
</phase_requirements>

## Summary

**最重要的发现：三层上限中有两层半是 coder/websocket 结构性白送的，但"分片数"这层库的公开 API 根本不暴露——需要用 `c.Reader` 手动读循环来计数，这是本 phase 唯一的非常规实现点。** 源码核实结论：`SetReadLimit` 限的是**单条消息跨分片累积字节**（read.go:88-89 "sets the max number of bytes to read for a single message"），`limitReader` 包在分片循环 `mr.read` 外层（read.go:116,521-541），百万个 1 字节 continuation 帧在累积到 16385 字节时精确触发 1009——ROADMAP 成功准则 1 的核心用例由库原生覆盖。库的重组是**流式**的：每个分片直接流进调用方缓冲，无累积副本、无重遍历，每帧 O(1)——Bandit GHSA-vg8x-66vg-5pxh 的 O(n²) 模式（每帧对全量累积缓冲跑 `IO.iodata_length/1`）在本库**结构上不存在**，空分片洪水下内存天然平坦。单帧上限同理隐含覆盖：帧声明长度只解析不分配，超限字节永远不被读取。

剩余缺口与对应手段（全部源码核实）：① **非空分片计数**——`mr.read` 每次调用至多消费一个非空帧（read.go:457-498），用 `c.Reader` + ≥limit 的缓冲手动循环，数 `n>0` 的 Read 次数即精确分片数，超 32 主动 `Close(1009, "fragment_limit")`；② **空分片慢滴洪水**——空分片在 `mr.read` 内部循环被吞（应用不可见），功能等价防线是"首帧到达后武装每消息完成时限"（`time.AfterFunc` 取消 ctx，read.go:31-33 官方注释指引的模式）；③ **子协议强制**——`AcceptOptions.Subprotocols` 不匹配时库**不拒绝**（selectSubprotocol 返回 "" 后照常 101，accept.go:141-144,266-275 VERIFIED），D-03 的 HTTP 层 400 预检因此是**必需**而非可选；④ **Ping** 阻塞等 pong 且必须由并发 Reader 交付（conn.go:216-220），D-16 的 10s pong 超时经 ctx 传入，超时经库内 AfterFunc 自动 `c.close()` 收口；⑤ **关闭码合规有库层护栏**——`validWireCloseCode` 线上拒发 1004/1005/1006/1015（close.go:279-293 VERIFIED），1006 永不发送由库强制；⑥ **新发现**：`http.Serve` 零超时（GOROOT server.go:2940-2943 VERIFIED），HTTP 握手层 slowloris 无防护，建议加 `ReadHeaderTimeout: 5s` 与 5s Hello 超时对齐。

**Primary recommendation:** 服务端 Attach 重构为"前置守卫链（400 子协议预检 → 429 per-IP → 409 原子门 → Accept → SetReadLimit 4KiB → 5s Hello 超时）→ 计数读循环握手（Hello 校验/Welcome 下发/SetReadLimit 16KiB 升档）→ 数据面循环（ro 丢 INPUT、分片计数、抢跑 1002）"；ping goroutine 在 Welcome 后启动；前端 `['wesh.v1']` 子协议 + Hello 首帧 + Welcome 驱动 ro/rw + onclose 按码分派。零新增第三方依赖。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 子协议/429/409 预检 | 服务端 `internal/server`（Attach 前置区，Accept 前） | — | D-03/D-04 要求零 WS 资源分配拦截；纯 HTTP 层检查 |
| 帧类型/版本/错误码/关闭码常量 | 服务端 `internal/proto` | 前端 TS 常量手工对齐 | proto 单一事实源（D-01，沿用 Phase 1 D-16 注释模式） |
| 三层上限执行 | 服务端（SetReadLimit + server 读循环计数） | — | 库限字节、应用限帧数；预认证 4KiB/认证后 16KiB 两档（D-11） |
| per-IP 半开计数 + 5s 未认证计时 | 服务端 `internal/server` | — | Accept 前计数、Hello 完成或连接终结时 exactly-once 释放 |
| ro/rw 权限边界 | 服务端（丢 INPUT，安全边界） | 前端（disableStdin，UX 层） | 服务端强制是真防线；前端层面不发只是体验（D-13/D-14） |
| ping/pong 保活 | 服务端（ping goroutine） | 浏览器自动回 pong | 浏览器 WS API 无 ping 控制，自动回 pong；保活流量防反代空闲超时 |
| 关闭码分派文案 | 前端 `web/src/main.ts`（onclose 按码 + Error 帧 message） | 服务端 close reason 机器串 | D-07/D-12：码值驱动文案，message 直接展示 |
| PTY 尺寸初始同步 | 服务端（Hello cols/rows → `sess.Resize`） | 前端 onopen fit 后随 Hello 上报 | Hello 携带首尺寸，消除 Phase 1 的 80x24 首帧窗口 |
| HTTP 握手层超时 | 服务端 `cmd/wesh/main.go`（http.Server.ReadHeaderTimeout） | — | 新发现：http.Serve 零默认超时，预认证 slowloris 面 |

## Standard Stack

**本 phase 零新增第三方依赖。** Phase 1 审计过的栈完整覆盖全部需求。

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.x（本机 1.26.3） | 后端语言/工具链 | 既定 [VERIFIED: 本机 `go version` 输出 `go version go1.26.3 linux/amd64`] |
| github.com/coder/websocket | v1.8.15（go.mod 已钉） | WS 服务端 + 测试客户端 | 本 phase 全部 API 行为经模块缓存源码逐行核实（read.go/close.go/conn.go/accept.go/frame.go/write.go，行号见正文）；零外部依赖 [VERIFIED: 模块缓存源码] |
| encoding/json（stdlib） | 随工具链 | Hello/Welcome/Error 控制帧编解码 | **未知字段忽略是 Unmarshal 进 struct 的默认行为**——D-02 演化纪律由 stdlib 天然满足，只需纪律性不加 DisallowUnknownFields |
| net/http（stdlib） | 随工具链 | 前置守卫链 + ReadHeaderTimeout | `http.Serve` 零超时已核实（GOROOT server.go:2940-2943）[VERIFIED: GOROOT 源码] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| testing + `go test -race`（stdlib） | 随工具链 | 全部自动化测试 | 攻击面模糊测试用裸 TCP 手写帧客户端（测试 helper，非依赖）；正常流程用 `websocket.Dial` |
| @xterm/xterm | 6.0.0（已装） | `disableStdin` 选项 | ro 模式前端层（typings xterm.d.ts:92 `disableStdin?: boolean` [VERIFIED: node_modules 安装产物]） |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `c.Reader` 计数读循环（分片数上限） | `c.Read`/`io.ReadAll` | **不可用**——io.ReadAll 吞掉分片边界，分片数不可见；必须用 Reader 手动循环（唯一非常规点，见 Pattern 3） |
| 库原生分片数上限 | gorilla/tungstenite/自写帧层 | gorilla 同样无分片计数且默认无读限（PITFALLS 已核）；tungstenite 是 Rust；自写帧层 = Pitfall 1 红线。**结论：应用层计数是唯一正解** |
| 每消息完成时限（空分片洪水兜底） | 不设时限 | 空分片 O(1)/帧、内存平坦、单连接门+per-IP 帽已限定暴露面；不设时限的残余风险是攻击者以 ≥6 字节/帧的线上代价换服务端一核比例性 CPU。推荐设时限（~10 行），planner 裁决（Open Questions Q1） |
| `ReadHeaderTimeout: 5s` | 不设 / 依赖反代 | 裸部署场景 http.Serve 零超时是真实 slowloris 面；一行修复，与 5s Hello 超时语义对齐 |

**Installation:**

```bash
# 无新增依赖。go.mod 已含 coder/websocket v1.8.15（Phase 1 钉定）。
# 若执行期 go.sum 漂移：go mod tidy && go mod verify
```

**Version verification:** coder/websocket v1.8.15 于 2026-08-13（Phase 1 research）经 `go mod download` 拉取并源码精读；本 phase 全部所需 API（`Accept`/`AcceptOptions.Subprotocols`/`SetReadLimit`/`Reader`/`Ping`/`Close`/`CloseNow`/`Subprotocol`/`CloseError`/`ErrMessageTooBig`/`StatusXxx` 常量集）在该版本源码中逐一核实存在且语义明确（行号见 §Code Examples 与 §Common Pitfalls）。@xterm/xterm 6.0.0 为 web/node_modules 当前安装版，`disableStdin` 存在于其 typings。

## Package Legitimacy Audit

**本 phase 不安装任何新外部包**——后端零新增（go.mod 三个 require 均为 Phase 1 审计通过项），前端零新增（disableStdin 是已装 @xterm/xterm 6.0.0 的内建选项）。Gate 协议无适用对象。

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| coder/websocket v1.8.15 | Go proxy | 2026-06-15 发布 | Coder 生产使用 | github.com/coder/websocket | OK | Approved（Phase 1 审计沿用，本 phase 源码精读再加固） |
| @xterm/xterm 6.0.0 | npm | 2025-12-22 发布 | 3.70M/wk | github.com/xtermjs/xterm.js | OK | Approved（Phase 1 审计沿用） |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
HTTP GET /ws
   │
   ▼
┌─ Attach 前置守卫区（Accept 之前，零 WS 对象分配）──────────────┐
│ ① Sec-WebSocket-Protocol 含 wesh.v1？──否──> HTTP 400（D-03）│
│ ② per-IP 半开计数 ≥8？──是──> HTTP 429（D-04）  否则 +1       │
│ ③ attached CAS 原子门──占用──> HTTP 409（Phase 1 既有，-1 释放）│
└──────────────────────────────┼───────────────────────────────┘
                               ▼
              websocket.Accept(Subprotocols: ["wesh.v1"])
              （库自动完成：Origin 同源校验 / 压缩默认关 / 400/403/426）
                               ▼
              assert c.Subprotocol()=="wesh.v1"（D-03 兜底）
              SetReadLimit(4KiB)          ← D-11 预认证档
              time.AfterFunc(5s, 未 Hello 则 Close(1008))  ← D-04
                               ▼
              计数读循环等首消息（分片计数 + 完成时限）
                 ├─ 非 'H' Hello ──> Close(1002,"抢跑") 无 Error 帧（D-04/D-06）
                 ├─ JSON 坏/version≠wesh.v1 ──> Write(Error) + Close(1008)（D-06）
                 └─ Hello OK ──> sess.Resize(cols,rows)
                               ▼
              Write(Welcome{version,mode}) → SetReadLimit(16KiB) 升档
              停 5s 计时器 / per-IP 计数 -1 / 启动 ping goroutine（D-16）
                               ▼
┌─ 数据面（单 reader 循环，沿用 Phase 1）────────────────────────┐
│ '0' INPUT：ro 丢弃（D-13）/ rw 写 master                      │
│ '1' RESIZE：放行（ro 同）→ 钳制 [1,1000] → Setsize            │
│ 未知类型 ──> Close(1002)；分片 >32 ──> Close(1009,"fragment_limit")
│ 累积 >16KiB ──> 库自动 1009（read.go:526-530）                │
│ 读循环终结（含 pong 超时/对端关闭）──> D-11 终结路径收口        │
└──────────────────────────────────────────────────────────────┘
S→C（onChunk 独占写端，Phase 1 既有）：OUTPUT 帧 / Welcome / Error / 1000 关闭
```

### Pattern 1: Attach 前置守卫链（Accept 前零分配拦截）

**What:** 把 D-03/D-04 的三道预检全部放在 `websocket.Accept` 调用之前——此刻 `w,r` 还是普通 HTTP 请求，拒绝 = `http.Error`，不创建任何 WS 对象（hijack/bufio/Conn 全不发生）。SEC-08"认证前零缓冲分配"的第一道结构防线。
**When:** 每个 /ws 请求入口。
**Why（源码依据）:** coder/websocket 的子协议协商**不在不匹配时拒绝连接**：`selectSubprotocol` 找不到匹配返回 `""`，`accept` 仅在非空时写响应头，随后照常 `WriteHeader(101)`（accept.go:141-144,266-275 VERIFIED 原文）。浏览器侧对"请求了子协议但服务端未回选"的响应是**照常建立连接**（`ws.protocol === ""`）[CITED: RFC 6455 §4.1 服务端握手规则]。因此"不含 wesh.v1 即 400"只能自己在 HTTP 层做——D-03 的必要性经源码证实。

```go
// Source: 本研究设计（基于 accept.go:102-182 Accept 流程与 server.go 现状）
func (s *Server) Attach(w http.ResponseWriter, r *http.Request) {
	// ① D-03：子协议预检（最廉价无状态，扫描器/旧客户端最早被拦）
	if !headerHasToken(r.Header, "Sec-WebSocket-Protocol", proto.Subprotocol) {
		http.Error(w, "subprotocol wesh.v1 required", http.StatusBadRequest)
		return
	}
	// ② D-04：per-IP 半开上限（map[string]int + Mutex；RemoteAddr 取 IP 部分）
	ip := clientIP(r) // net.SplitHostPort(r.RemoteAddr)；反代后按代理 IP 聚合（见 Pitfall 7）
	if !s.halfOpen.acquire(ip, maxHalfOpenPerIP /*8*/) {
		http.Error(w, "too many half-open connections", http.StatusTooManyRequests)
		return
	}
	// ③ 409 原子门（Phase 1 既有，保持第一位的是子协议预检——见下"顺序理由"）
	if !s.attached.CompareAndSwap(false, true) {
		s.halfOpen.release(ip) // exactly-once 释放
		http.Error(w, "another client is already attached", http.StatusConflict)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{proto.Subprotocol}, // "wesh.v1"：一行开启协商 + 写响应头
		// CompressionMode 默认 CompressionDisabled（accept.go:55-59 VERIFIED）——D-17 成立
		// 不设 InsecureSkipVerify：默认同源放行（accept.go:228-260 VERIFIED），Origin 白名单属 Phase 3
	})
	if err != nil {
		s.attached.Store(false)
		s.halfOpen.release(ip)
		return
	}
	if c.Subprotocol() != proto.Subprotocol { // D-03 assert 兜底（理论不可达）
		c.CloseNow()
		s.attached.Store(false)
		s.halfOpen.release(ip)
		return
	}
	// …进入 Pattern 2 握手状态机
}
```

**顺序理由：** 子协议预检无状态最廉价排最前；per-IP 在 409 之前是因为被拒的第二客户端不该消耗半开名额（409 后立即释放）；所有拒绝路径都必须 `release(ip)` 或从未 `acquire`——**不变量：acquire 成功 → Hello 完成或连接/拒绝终结时 release 恰好一次**（Hello 完成即释放，D-04"Hello 已完成不计入"）。

**`headerHasToken` 注意：** `Sec-WebSocket-Protocol` 是逗号分隔 token 列表，须按 token 拆分比较（可参照库内 `headerTokens` 的做法，accept.go:267），不能 `strings.Contains`——防 `wesh.v1.evil` 之类前缀绕过。

### Pattern 2: 握手状态机（Hello → Welcome，两档 ReadLimit）

```go
// Source: 本研究设计（SetReadLimit 语义 read.go:88-105 VERIFIED；Close 语义 close.go:86-128 VERIFIED）
const (
	readLimitPreAuth  = 4 * 1024   // D-11：Hello JSON ~100B，余量两个数量级
	readLimitPostAuth = 16 * 1024  // D-09/D-11
	helloTimeout      = 5 * time.Second // D-04
	maxFragments      = 32         // D-09
)

func (s *Server) handshake(c *websocket.Conn, ctx context.Context) (hello *proto.Hello, ok bool) {
	c.SetReadLimit(readLimitPreAuth) // 预认证档：单连接预认证窗口可占内存最小化（SEC-08）
	helloDone := make(chan struct{})
	timer := time.AfterFunc(helloTimeout, func() {
		select {
		case <-helloDone:
		default:
			c.Close(websocket.StatusPolicyViolation, "hello_timeout") // 1008，机器串 reason（D-07）
		}
	})
	defer func() { close(helloDone); timer.Stop() }()

	buf := make([]byte, readLimitPreAuth) // 复用缓冲 ≥ limit：Pattern 3 的计数前提
	typ, data, err := readCounted(ctx, c, buf, maxFragments) // 见 Pattern 3
	if err != nil {
		// ErrMessageTooBig：库已自动发 1009（read.go:526-530 VERIFIED），stderr 单行事件（D-12②）
		return nil, false
	}
	if typ != websocket.MessageBinary || len(data) == 0 || data[0] != proto.Hello {
		c.Close(websocket.StatusProtocolError, "unexpected_frame") // 1002 抢跑帧，无 Error 帧（D-04/D-06）
		return nil, false
	}
	h, err := proto.DecodeHello(data[1:]) // JSON；未知字段忽略（D-02，stdlib 默认行为）
	if err != nil || h.Version != proto.Subprotocol {
		writeErrorFrame(c, proto.ErrVersionMismatch, "Protocol version mismatch. This server speaks wesh.v1.")
		c.Close(websocket.StatusPolicyViolation, proto.ErrVersionMismatch) // 1008（D-06）
		return nil, false
	}
	return h, true
}
```

Welcome 发出后的升档序列（顺序敏感）：`sess.Resize(h.Cols, h.Rows)`（Hello 携带首尺寸，消除 Phase 1 的 80x24 首帧窗口）→ `writeWelcome(mode)` → `c.SetReadLimit(readLimitPostAuth)` → 停 hello 计时器 → per-IP release → 启动 ping goroutine（Pattern 4）。`SetReadLimit` 内部是 `atomic.Int64.Store`（read.go:104 VERIFIED），读循环进行中调档安全。

### Pattern 3: 分片计数读循环（RES-01 层2 的唯一实现路径）

**What:** 库公开 API 不暴露分片边界（`c.Read` = `io.ReadAll` 吞掉边界，read.go:41-49），但 `msgReader.read` 每次调用**至多消费一个非空帧**（read.go:457-498 VERIFIED：读满当前帧即返回；空分片在 `payloadLength==0` 分支内部循环吞掉）。因此用 `c.Reader` + **容量 ≥ ReadLimit 的缓冲**手动循环时，**每次 `n>0` 的 Read 恰好对应一个非空分片**——计数即分片数上限。

```go
// Source: 本研究设计（行为依据 read.go:435-498 逐行核实；完成时限模式依据 read.go:31-33 官方注释）
var errFragmentLimit = errors.New("fragment limit exceeded")

// readCounted 读取一条完整消息并计数非空分片；超 maxFragments 返回 errFragmentLimit。
// buf 必须 ≥ 当前 SetReadLimit 值（保证单 Read 不被缓冲截断成"伪分片"）。
func readCounted(ctx context.Context, c *websocket.Conn, buf []byte, maxFragments int) (websocket.MessageType, []byte, error) {
	rctx, rcancel := context.WithCancel(ctx)
	defer rcancel() // 消息读完后 cancel 安全：每帧读结束 finishRead 已停内部 AfterFunc（read.go:244-258）
	typ, r, err := c.Reader(rctx) // 返回即首帧头已读（read.go:382-395）
	if err != nil {
		return 0, nil, err
	}
	// 首帧到达后才武装完成时限——空分片慢滴洪水（mr.read 内部循环不可见）的功能等价防线。
	// ctx 取消 → 库内 AfterFunc → c.close() 整连接关闭（conn.go:188-199 VERIFIED）——正是预期。
	timer := time.AfterFunc(msgCompleteTimeout, rcancel)
	defer timer.Stop()

	msg := buf[:0]
	fragments := 0
	for {
		n, rerr := r.Read(buf[len(msg):cap(msg)]) // 追加读进同一缓冲，零额外分配
		if n > 0 {
			msg = msg[:len(msg)+n]
			fragments++
			if fragments > maxFragments {
				return 0, nil, errFragmentLimit // 调用方 Close(1009, "fragment_limit")（D-08 复用 1009）
			}
		}
		if errors.Is(rerr, io.EOF) {
			return typ, msg, nil
		}
		if rerr != nil {
			return 0, nil, rerr // 含 ErrMessageTooBig（库已发 1009）与超时关闭
		}
		if len(msg) == cap(msg) { // 防御：理论上 limitReader 先触发 1009，此分支不可达
			return 0, nil, websocket.ErrMessageTooBig
		}
	}
}
```

**三个已核实的边界行为：** ① 恰等于 limit 的消息正常读完（库内部存 limit+1，"多读一字节留给 fin 帧探测"，read.go:98-102 VERIFIED）；② 超限消息在 `lr.n==0` 的下一次 Read 触发 `writeError(StatusMessageTooBig,…)` 并返回 `ErrMessageTooBig`（read.go:526-530 VERIFIED）——库自动 1009 的 reason 固定为 `"read limited at N bytes"`，**不可定制**（对 D-12③ 的影响见 Pitfall 4）；③ continuation 无 opener、RSV 非法、opcode 保留值、控制帧分片、控制帧 >125B 均由库自动 1002（read.go:190-194,213-216,291-301,387-391 VERIFIED）——畸形帧面零应用代码。

### Pattern 4: ping 保活 goroutine（CORE-06）

**库语义（conn.go:216-259 VERIFIED 原文要点）：** `Ping(ctx)` 发自增计数器为载荷的 ping，注册进 `activePings`，**阻塞**至 ①连接关闭 ②ctx 终结 ③匹配 pong 到达；"Ping must be called concurrently with Reader as it does not read from the connection"——本 Attach 读循环常阻塞在 Read 上，pong 交付天然满足。pong 匹配按载荷（handleControl read.go:324-337），未匹配的多余 pong 静默忽略。pong 超时：ctx 带 10s deadline（D-16），超时后 writeControl 的超时 AfterFunc 直接 `c.close()`（conn.go:171-182 VERIFIED）→ 读循环 `c.Read` 返回错误 → **沿 Phase 1 既有 wsDisconnected 路径收口，零新终结分支**。

```go
// Source: 本研究设计（Ping 语义 conn.go:216-259 VERIFIED）
const pongTimeout = 10 * time.Second // D-16 常量；正常 RTT 毫秒级，10s 极宽

func (s *Server) pinger(ctx context.Context, c *websocket.Conn, interval time.Duration) {
	if interval <= 0 {
		return // --ping-interval=0 禁用（D-16）
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done(): // Attach 返回时 cancel，goroutine 随连接生命周期终结
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, pongTimeout)
		err := c.Ping(pctx)
		cancel()
		if err != nil {
			return // 连接已死/对端无响应：终结由 reader 路径收口（D-11 既有）
		}
	}
}
```

**装配：** Welcome 升档后 `go s.pinger(ctx, c, cfg.pingInterval)`；`ctx` 从 `context.Background()` 派生并随 Attach defer cancel（**禁止 r.Context()**，hijack 后行为意外——Phase 1 已记，官方 README 同源警告在 accept.go:100-101）。写路径并发安全：`Ping`→`writeControl` 与 `onChunk` 的 `c.Write` 共用写互斥（库注释 "All methods may be called concurrently except for Reader and Read"）。保活有效性：ping/pong 帧是线上真实流量，5s 间隔 ≪ nginx `proxy_read_timeout` 60s / Cloudflare ~100s / 30s 型 ingress（D-16 已论证；PITFALLS 集成表交叉确认）。

### Pattern 5: 前端握手 + ro/rw + onclose 按码分派

```typescript
// Source: 本研究设计（disableStdin: xterm.d.ts:92 VERIFIED；其余沿用 main.ts 现状骨架）
const ws = new WebSocket('ws://' + location.host + '/ws', ['wesh.v1']); // D-03
const HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45; // 'H' 'W' 'E'，与 proto 手工对齐（D-01/D-16）

let mode: 'ro' | 'rw' = 'rw';
let lastError: { code: string; message: string } | null = null; // Error 帧暂存，onclose 展示

ws.onopen = () => {
  opened = true;
  fit.fit();
  // Hello 必须是首帧（D-02/D-04）；携带首尺寸消除 80x24 首帧窗口
  ws.send(concat(new Uint8Array([HELLO]),
    enc.encode(JSON.stringify({ version: 'wesh.v1', cols: term.cols, rows: term.rows }))));
  term.focus();
};

ws.onmessage = (ev) => {
  const buf = new Uint8Array(ev.data as ArrayBuffer);
  switch (buf[0]) {
    case OUTPUT: term.write(buf.subarray(1)); break;
    case WELCOME: {
      const w = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
      mode = w.mode === 'ro' ? 'ro' : 'rw';
      if (mode === 'ro') {
        term.options.disableStdin = true;        // D-14：键盘层面即不产生 onData
        document.title = '[ro] ' + document.title; // 零新 UI 组件
      }
      break;
    }
    case ERROR:
      lastError = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
      break;
    default: break; // 未知 S→C 类型忽略（前向兼容，D-02 同纪律）
  }
};

ws.onclose = (ev) => {
  // 1006 永不到达线上（RFC §7.4 MUST NOT + 库 validWireCloseCode 强制）；异常断开 = !ev.wasClean
  if (!opened) { showStatus('Unable to connect', /* 现有文案（含子协议缺失/429/409 场景） */, '…'); return; }
  switch (ev.code) {
    case 1000: showStatus('Session ended', /* 现有文案 */, '…'); break;
    case 1008: showStatus('Connection refused', lastError?.message ?? 'The server refused this connection.', '…'); break;
    case 1009: showStatus('Message too large', 'Input exceeded the server message size limit and the connection was closed.', '…'); break; // D-12① 不提 flag
    case 1011: showStatus('Server error', lastError?.message ?? 'The server hit an internal error.', '…'); break;
    case 1013: showStatus('Disconnected', 'The server asked this client to retry later.', '…'); break; // Phase 5 占位路径
    default:   showStatus('Connection lost', /* 现有文案 */, '…'); break; // 含 1002 与无码异常断开
  }
};
```

ro 模式 `term.onData` 里保留 `mode === 'rw'` 的发送门（disableStdin 之外的第二道前端闸），但**真防线在服务端丢 INPUT**（D-13）。

### Anti-Patterns to Avoid

- **用 `c.Read`/`io.ReadAll` 读消息还想限分片数：** 边界被吞，分片计数不可实现；必须 `c.Reader` 手动循环（Pattern 3）。
- **给 `c.Read` 一个带 deadline 的 ctx 当"空闲超时"：** ctx 到期 = `c.close()` 整连接关闭（conn.go:188-199 VERIFIED），不是"本次读失败返回"——空闲 N 秒的合法终端连接会被杀。读路径超时的唯一安全形态是 Pattern 3 的"首帧到达后武装完成时限"。
- **指望 `AcceptOptions.Subprotocols` 强制子协议：** 库不匹配时照常 101（accept.go:141-144 VERIFIED）；必须在 Accept 前自查 400（D-03）。
- **先 `Write(Error 帧)` 后 `CloseNow()`：** CloseNow 跳过关闭握手直接关 TCP，bufio 里未刷出的 Error 帧可能丢失；先 graceful `c.Close(code, reason)`（5s+5s 有界握手，close.go:86-99 VERIFIED），`defer c.CloseNow()` 只做兜底（Phase 1 既有模式）。
- **给攻击面路径发 Error 帧：** 抢跑/未知帧/超限只发关闭码（D-06）——Error 帧给攻击者反馈面，且库自动 1009 路径根本没机会写。
- **应用层自造心跳帧：** 库内建 ping/pong（`c.Ping` + 自动 pong，read.go:317-323 VERIFIED）；CORE-06 用原生 ping（Phase 1 Anti-Pattern 已记，沿用）。
- **`strings.Contains` 查子协议头：** token 列表须拆分精确匹配，防前缀伪造。
- **为 1001/1013 提前写发送路径：** D-08 明确占位不实现；超前实现 = 动协议语义。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 分片重组 / mask 解析 / UTF-8 校验 / RSV-opcode 合规 | 任何手写帧层代码 | coder/websocket 读路径 | ttyd 两个预认证严重漏洞 + Bandit CVE 全在手写重组（PITFALLS C1）；库流式重组 O(1)/帧（read.go:457-498 VERIFIED） |
| 累积字节上限 / 单帧上限 | 应用层计数器 | `SetReadLimit`（per-message 跨分片，超限自动 1009） | read.go:88-105,521-541 VERIFIED；库的路径比应用层早拦截、零拷贝 |
| 关闭码线值合法性 | 应用层白名单自查 | 库 `validWireCloseCode` | 1004/1005/1006/1015 线上拒发（close.go:279-293 VERIFIED）——1006 永不发送是库强制，应用层只需"不发明新码"（D-08） |
| ping/pong 保活 | 应用层心跳帧 | `c.Ping(ctx)` + 库自动 pong | conn.go:216-259 / read.go:317-323 VERIFIED |
| 压缩协商拒绝 | 手查 `Sec-WebSocket-Extensions` | `CompressionDisabled` 默认值 | accept.go:55-59 默认即关（VERIFIED）；rsv1 帧自动 1002（read.go:171-181） |
| 测试用畸形帧构造 | 修改库 / 反射内部 API | 裸 TCP 手写帧测试 helper（~60 行） | 库客户端不发畸形帧；测试 helper 独立于被测库实现，见 §Code Examples |
| Hello/Welcome JSON 编解码 | 手写解析 / 严格 schema 校验器 | `encoding/json` struct Unmarshal | 未知字段忽略是默认行为 = D-02 演化承诺的天然实现 |

**Key insight:** 本 phase 的自主代码集中在**库明确不提供的三件事**：分片数计数（Reader 循环）、子协议强制（HTTP 预检）、per-IP/计时器守卫。其余全部是"调用 + 常量表"。每多写一行帧层代码都是在 Bandit/ttyd 坟场上蹦迪。

## Common Pitfalls

### Pitfall 1: 误用 `c.Read` 导致分片数上限无法落地

**What goes wrong:** 沿用 Phase 1 的 `c.Read(ctx)`（内部 `io.ReadAll`），分片边界丢失，RES-01 层2 只能放弃或错误地"用 bufio 包 conn 数字节"。
**Why it happens:** `Read` 的便利签名隐藏了 `Reader` 的存在；分片计数需求在库 API 文档里没有直接答案。
**How to avoid:** 数据面统一走 Pattern 3 的 `readCounted`；缓冲 ≥ 当前 ReadLimit（否则单帧被缓冲截断成多次 Read → 伪分片误杀合法大消息）。
**Warning signs:** 代码里 `io.ReadAll(r)`；或 buffer 小于 SetReadLimit 值。

### Pitfall 2: ctx 到期 = 整连接关闭（读超时语义陷阱）

**What goes wrong:** 给读循环的 ctx 加 deadline 想实现"读超时"，结果 conn.go:188-199 的 `setupReadTimeout` 在 ctx 终结时直接 `c.close()`——空闲终端（用户不敲键盘）被误杀；或 ping 的 10s ctx 复用到读路径，一次 pong 超时变成全连接重置。
**Why it happens:** 直觉以为 ctx 超时只让当前 Read 返回 `context.DeadlineExceeded`；库的实现选择是"超时即关连接"（AfterFunc → close，VERIFIED）。
**How to avoid:** 读路径只传无 deadline 的可取消 ctx；完成时限用 Pattern 3 的"Reader 返回后 `time.AfterFunc(rcancel)`"形态；ping 的 10s ctx 独立短命（Pattern 4）。
**Warning signs:** 空闲一段时间后连接神秘断开且 close code 缺失；测试里"等 10s 后连接死了"。

### Pitfall 3: 子协议"协商了"但旧客户端照样连上

**What goes wrong:** 只设 `AcceptOptions.Subprotocols: ["wesh.v1"]`，不带子协议的扫描器/旧客户端收到无 `Sec-WebSocket-Protocol` 头的 101 并**成功建立连接**（accept.go:141-144 VERIFIED；浏览器对未回选行为 = 照常连通 [CITED: RFC 6455 §4.1]），D-03 的最早拦截目标落空。
**How to avoid:** Pattern 1 的 HTTP 预检是必须路径；`c.Subprotocol()` assert 仅作兜底。
**Warning signs:** 无 `Sec-WebSocket-Protocol` 的 curl 升级请求拿到 101。

### Pitfall 4: 库自动 1009 的 close reason 不可定制（D-12③ 落点修正）

**What goes wrong:** D-12③ 要求 close reason 带机器串 `message_too_big`，但累积字节超限走库自动路径，reason 固定为 `"read limited at 16385 bytes"`（read.go:527 VERIFIED），应用无介入点。
**How to avoid:** 三腿提示的精确落点——① 前端按 **code** 1009 分派文案（不依赖 reason，D-12① 不变）；② stderr 单行事件用 `errors.Is(err, websocket.ErrMessageTooBig)` 归一化为 `message_too_big` 机器串；③ 应用层主动关闭（分片超限）reason = `fragment_limit` 完全可控。库 reason 文本 "read limited at …" 在 devtools 里同样可辨，不构成信息缺口。**此条请 planner/d discuss 知悉：D-12③ 的 `message_too_big` 落点在 stderr 事件而非线上 reason。**
**Warning signs:** 模糊测试里抓到的 1009 reason 不是预期机器串——这是正常的，别去包装库。

### Pitfall 5: Error 帧写了但对端没收到

**What goes wrong:** `Write(Error)` 紧跟 `CloseNow()`：CloseNow 不走关闭握手直接关 TCP（close.go:130-155 VERIFIED），写缓冲里的 Error 帧可能随 RST 丢失。
**How to avoid:** 需要对方看到的关闭一律 graceful `c.Close(code, reason)`（写关闭帧 5s 超时 + 等对端回显 5s，close.go:86-99 VERIFIED）；`defer c.CloseNow()` 保留为路径兜底（幂等，Phase 1 既有）。
**Warning signs:** e2e 里客户端偶发收不到 Error 帧直接见 1006。

### Pitfall 6: per-IP 计数泄漏或双重释放

**What goes wrong:** 拒绝路径多（400/429/409/Accept 失败/5s 超时/抢跑关闭/正常 Hello），漏一条 release → 计数只涨不降 → 合法用户被 429；或同一连接 release 两次 → 计数为负，帽失效。
**How to avoid:** 不变量一句话——**acquire 成功后，release 恰好一次，发生在 Hello 完成或连接终结（先到为准）**；实现上用 `sync.Once` 或"defer 释放 + Hello 完成时显式释放并置空"二选一，测试覆盖全部退出路径（NAT 场景同 IP 多连接并发用例）。
**Warning signs:** 压测后新连接持续 429；`-race` 报计数器竞态。

### Pitfall 7: 反代部署下 per-IP 按代理 IP 聚合

**What goes wrong:** wesh 挂在 nginx/Cloudflare 后，`r.RemoteAddr` 全是反代 IP → 全部用户共享 8 个半开名额，突发合法流量互相 429；攻击者打满 8 个名额 = 对所有用户拒绝服务。
**How to avoid:** Phase 2 文档明示该限制（本 phase 无可信 X-Forwarded-For 处理——SEC-04/SEC-07 在 Phase 3/7）；缓解事实是"半开"窗口极短（5s 超时 + Hello 完成即释放），正常浏览器秒过窗口。**不**在本 phase 解析 X-Forwarded-For（不可信头直接信 = 伪造 IP 绕过计数器，更糟）。
**Warning signs:** 反代后多人同时首开页面出现 429。

### Pitfall 8: HTTP 握手层 slowloris 无防护（新发现）

**What goes wrong:** `http.Serve(ln, handler)` = `&Server{Handler: handler}` 零超时（GOROOT server.go:2940-2943 VERIFIED）；攻击者慢滴 HTTP 头可无限占用连接 goroutine——这是 /ws 预检代码都到不了的**更前置**预认证面。
**How to avoid:** `http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}`（与 5s Hello 超时语义对齐）；WS hijack 后该超时不再作用于已升级连接（只影响握手前，VERIFIED: GOROOT server.go ReadHeaderTimeout 注释路径）。`ReadTimeout`/`WriteTimeout` **不要设**（会误伤长连接语义边界，hijack 后虽不生效但避免误配）。
**Warning signs:** `slowhttptest` 类工具挂几百连接后服务 fd 耗尽。

## Code Examples

### proto 包扩展骨架（D-01/D-05/D-06/D-07/D-08 单一事实源）

```go
// Source: 本研究设计（常量值 D-01 锁定；关闭码与库常量一一对应 close.go:27-59 VERIFIED）
package proto

const Subprotocol = "wesh.v1" // D-03：子协议名 = Hello.version 期望值（同一字符串，单一常量）

// 帧类型字节（D-01；'X' EXIT / 'T' TITLE / 'P' PREFS 为 Phase 4/6 占住，不实现）
const (
	Hello   = 'H' // 0x48, C→S, JSON {version, cols, rows}
	Welcome = 'W' // 0x57, S→C, JSON {version, mode}
	Error   = 'E' // 0x45, S→C, JSON {code, message}
	// Input/Resize/Output 沿用 Phase 1 既有常量
)

// Error codes（D-06/D-07：snake_case 机器串；Error 帧 code 与 close reason 同名）
const (
	ErrVersionMismatch = "version_mismatch" // +1008（正常客户端可见，发 Error 帧）
	ErrServerError     = "server_error"     // +1011（发 Error 帧）
	// 攻击面路径无 Error 帧，仅关闭码 + reason 机器串：
	// "unexpected_frame"(1002 抢跑/未知帧) / "hello_timeout"(1008) /
	// "fragment_limit"(1009 应用层分片超限) / 库自动 1009 reason 不可定制（Pitfall 4）
)

// 关闭码常量表 = D-05 全集；1001/1013 占位不实现（D-08，注释标启用 phase）。
// 发送侧值直接复用 websocket.StatusXxx 常量，本表用于前端对齐文档与断言测试。
```

Hello/Welcome 的 struct 定义沿用 `resizePayload` 模式（显式 json tag）；`DecodeHello` 返回 `(Hello, error)`，**不**调 `DisallowUnknownFields`（D-02 演化承诺，stdlib 默认忽略未知字段）。

### 攻击面模糊测试 helper（裸 TCP 手写帧客户端）

```go
// Source: 本研究设计；线格式依据 RFC 6455 §5.2 + 库解析端 frame.go:52-102 逐字段核实（VERIFIED）
// 用途：库客户端发不出畸形帧——空帧/百万分片/抢跑/无子协议握手等用例必须裸写。
func dialRawWS(t *testing.T, addr string, extraHeaders ...string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil { t.Fatal(err) }
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")) // 16B 即合规
	req := "GET /ws HTTP/1.1\r\nHost: " + addr +
		"\r\nUpgrade: websocket\r\nConnection: Upgrade" +
		"\r\nSec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13"
	for _, h := range extraHeaders { req += "\r\n" + h } // 例: "Sec-WebSocket-Protocol: wesh.v1"
	req += "\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil { t.Fatal(err) }
	// 读响应头至 \r\n\r\n；断言首行（101 或 400/429——负例测试的断言点）
	// …
	return conn
}

// writeRawFrame 构造 C→S 帧：mask 强制（服务端对未 mask 帧直接报错，read.go:196-198 VERIFIED）
func writeRawFrame(t *testing.T, conn net.Conn, fin bool, opcode byte, payload []byte) {
	t.Helper()
	b0 := opcode
	if fin { b0 |= 0x80 }
	head := []byte{b0}
	switch n := len(payload); {
	case n < 126:
		head = append(head, 0x80|byte(n))
	case n <= 65535:
		head = append(head, 0x80|126, byte(n>>8), byte(n))
	default:
		head = append(head, 0x80|127, 0,0,0,0, byte(uint64(n)>>24), byte(uint64(n)>>16), byte(uint64(n)>>8), byte(n))
	}
	maskKey := [4]byte{0xde, 0xad, 0xbe, 0xef}
	head = append(head, maskKey[:]...)
	masked := make([]byte, len(payload))
	for i, b := range payload { masked[i] = b ^ maskKey[i%4] }
	if _, err := conn.Write(append(head, masked...)); err != nil { t.Fatal(err) }
}
```

### 关键验证用例矩阵（对应 Validation Architecture）

```go
// Source: 本研究设计
// 百万 1 字节 continuation → 库在累积 16385B 时自动 1009：
//   opener: writeRawFrame(fin=false, op=0x2, payload=1B)；随后 continuation(fin=false, op=0x0, 1B) 循环；
//   断言：服务端在 ≤16385 帧内发来 1009 关闭帧（直接解析线上关闭帧：fin=1, op=0x8, 2B 码值）。
// 33 个非空分片（33B 总字节 < 16KiB 字节限）→ 应用层 1009 "fragment_limit"：
//   证明层2 独立于层3 生效——这是 RES-01 "三层缺一不可"的最小反例用例。
// 空帧：writeRawFrame(fin=true, op=0x2, payload=nil) → 连接保持（空消息被服务端 continue 跳过，不崩溃不关）。
// 抢跑：握手成功后直接发 INPUT 帧（无 Hello）→ 1002 关闭帧，且此前线上无 'E' Error 帧（D-06）。
// 无子协议：dialRawWS 不带 Sec-WebSocket-Protocol → HTTP 400（首行断言），连接未升级。
// 半开帽：同 IP 8 连接完成升级不发 Hello，第 9 个 → HTTP 429。
// 5s 超时：升级成功静默 → 5s 左右收到 1008 关闭帧（reason "hello_timeout"）。
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 手写 WS 分片重组（ttyd/Bandit 模式） | 成熟库流式重组 + 三层上限 | 持续教训（Bandit CVE 2026-07） | GHSA-vg8x-66vg-5pxh：8MB 字节限不限帧数 → O(n²) CPU，CVSS 8.7；wesh 层2 分片计数正对此 |
| gorilla/websocket | coder/websocket | Phase 1 已定案 | gorilla 默认无读限需显式 SetReadLimit；coder 默认 32768 且 API 语义本 phase 已全量源码核实 |
| 应用层心跳帧 | WS 原生 ping/pong | ARCHITECTURE §2.8 / Phase 1 沿用 | 少一条协议帧类型；代理对控制帧同样刷新空闲计时 |
| permessage-deflate 默认开（部分库） | 默认关（coder/websocket CompressionDisabled） | 库默认（accept.go:55-59） | 终端高熵数据无收益 + 每连接 zlib 状态内存——D-17 零代码成立 |
| ARCHITECTURE §2.8 控制面 text JSON | 统一 binary 1 字节类型分派 | D-01（本 phase 作废旧案） | 单一解码路径；proto.go 预留路线落地 |

**Deprecated/outdated:**
- ARCHITECTURE.md §2.8 "控制面 text JSON" 方案：D-01 作废，勿再引用。
- ROADMAP 成功准则 2 关闭码集合 `{1000,1008,1009,1011,1013}`：漏 1002，以 D-05 并集 `{1000,1001,1002,1008,1009,1011,1013}` 为准（CONTEXT 已记，ROADMAP 校正已 defer）。
- "前端等待 1006 判断异常断开"：线上永无 1006（RFC §7.4 MUST NOT + 库 close.go:279-293 强制）；用 `!ev.wasClean` / 无码分支（前端 Pattern 5 已按此写）。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 浏览器 WS API 每次 `send()` 产生单条完整消息、实际不分片；RFC 允许中间代理分片，故 32 分片上限须留余量（16KiB 消息按 ~1.5KiB MSS 分片 ≈11 帧 < 32） | D-09 依据 / Pattern 3 | 低：代理分片场景余量分析覆盖；若某代理分片 >32 片/16KiB，合法大 INPUT 被 1009——但浏览器 INPUT 本身字节级，16KiB 消息仅粘贴场景 |
| A2 | Hello.version 期望值 = `"wesh.v1"`（与子协议同串同常量） | Code Examples | 低：CONTEXT 未钉 version 字段取值，属 proto 组织 discretion；取同串最少常量 |
| A3 | `hello_timeout` 用 1008 + 无 Error 帧 | Pattern 2 | 低：D-06 未枚举该路径；定性为"攻击/异常路径"（正常浏览器秒发 Hello），与"不给攻击者反馈面"一致；若 planner 判为正常客户端路径则补 Error 帧即可 |
| A4 | reason 机器串具体拼写（`unexpected_frame`/`hello_timeout`/`fragment_limit`） | proto 骨架 | 低：D-07 只钉"同名机器串"原则，拼写属 discretion |
| A5 | 每消息完成时限推荐 10s（与 pong 超时同值） | Pattern 3 | 低：合法消息浏览器原子送达，毫秒级完成；10s 对 56kbit/s 极端链路传 16KiB（~2.3s）仍有 4x 余量 |
| A6 | Bandit 修复版 1.12.1 的具体修复手段（推断为加帧数上限） | Summary | 极低：advisory 原文只给出漏洞机制与补丁版本号，修复细节未逐字引用；wesh 不依赖该结论（三层设计由漏洞机制直接推出） |
| A7 | stderr 单行事件含对端 IP | Pattern 2/D-12 | 极低：运维标准实践；Phase 8 结构化日志统一收口 |

**除 A1-A5 的参数推荐外无实质假设。** 全部库行为、CVE 事实、RFC 规则、xterm 选项均 VERIFIED/CITED。A1-A5 是 planner 可定稿的实现参数，不需要用户额外确认。

## Open Questions

1. **空分片慢滴洪水：完成时限（推荐）还是接受残余风险？**
   - What we know: 空分片在 `mr.read` 内部循环被吞（应用不可计数）；库重组 O(1)/帧、内存天然平坦；残余风险 = 攻击者以 ≥6 线上字节/帧换服务端比例性 CPU（无放大），且单连接门（409）+ per-IP 帽（8）+ 5s 预认证窗口把暴露面压到最小。
   - What's unclear: 无技术盲点——纯粹是"要不要多 10 行代码换确定性的连接生命周期上限"的取舍。
   - Recommendation: **实现**（Pattern 3 已含，`time.AfterFunc(msgCompleteTimeout, rcancel)` 三行 + 常量），与 Bandit advisory 教训 #4"idle-only 超时不覆盖重组期"的教训对齐；planner 若从简，须在 PLAN 风险节明示接受该残余风险。

2. **pong 超时 10s 常量的测试可注入性**
   - What we know: D-16 钉死 10s 常量（不开 flag）；`TestPongTimeout` 若用真 10s 常量，单测耗时 ~11s（interval 1s + timeout 10s），拖慢 quick-run 采样。
   - What's unclear: 包级私有变量（测试内改写）vs 常量 + 接受慢测试。
   - Recommendation: 包级私有变量 `pongTimeout`（非 export、非 flag，生产值恒 10s）——不违反 D-10/D-16"常量不开 CLI flag"的语义（可配性指用户面），planner 定。

3. **`msgCompleteTimeout` 与 Hello 5s 超时的关系**
   - What we know: 预认证期 5s Hello 超时（D-04）已覆盖"首帧迟迟不到"；完成时限管"首帧到了但消息永远不完"。
   - What's unclear: 预认证期是否也武装完成时限（Hello 消息被空分片慢滴吊住时，5s hello 计时器已覆盖——是，helloTimeout 从 Accept 起算，覆盖整个握手窗）。
   - Recommendation: 完成时限仅数据面（Welcome 后）；预认证由 5s helloTimeout 单点覆盖，不叠加。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 工具链 | 全部后端工作与测试 | ✓ | 1.26.3 linux/amd64 | — |
| coder/websocket 模块缓存（源码精读） | 本研究全部 VERIFIED 结论 | ✓ | v1.8.15 | — |
| Node.js | 前端构建验证 | ✓ | v24.13.0 | — |
| pnpm | 前端包管理 | ✓ | 11.21.0 | — |
| 外部服务（DB/Redis/Docker 等） | — | 无依赖 | — | — |

**Missing dependencies with no fallback:** 无。
**Missing dependencies with fallback:** 无。

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `-race`；攻击面用例用裸 TCP 帧 helper（§Code Examples）；正常 WS 流程用 coder/websocket `Dial`（既有惯例，e2e_test.go:43） |
| Config file | none |
| Quick run command | `go test ./... -count=1` |
| Full suite command | `go test -race -count=1 ./... && pnpm -C web build` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RES-01 | 超限消息 → 1009 | e2e：库客户端发 17KiB 单帧，断言 CloseError.Code==1009 | `go test ./internal/server -run TestMessageTooBig -count=1` | ❌ Wave 0 |
| RES-01 | 百万 1 字节 continuation → 1009 且内存平坦 | e2e（裸帧）：断言 ≤16385 帧内收到 1009；随后新连接握手成功（存活/无崩溃代理断言） | `go test ./internal/server -run TestFragmentByteFlood -count=1` | ❌ Wave 0 |
| RES-01 | 分片数 32 硬顶（独立于字节限） | e2e（裸帧）：33 个 1B 分片（33B<16KiB）→ 1009 + reason `fragment_limit` | `go test ./internal/server -run TestFragmentCountLimit -count=1` | ❌ Wave 0 |
| RES-01 | 空帧不崩溃不关连接 | e2e（裸帧）：空 binary 消息后连接存活，后续 Hello 正常完成 | `go test ./internal/server -run TestEmptyFrame -count=1` | ❌ Wave 0 |
| RES-01 | 空分片慢滴 → 完成时限关闭（若实现，Open Questions Q1） | e2e（裸帧）：opener 后每 100ms 滴空 continuation，断言 ~10s 内连接被关 | `go test ./internal/server -run TestFragmentTrickle -count=1` | ❌ Wave 0 |
| SEC-08 | 无 wesh.v1 → HTTP 400 未升级 | e2e（裸 HTTP）：断言响应首行 400 | `go test ./internal/server -run TestSubprotocolRequired -count=1` | ❌ Wave 0 |
| SEC-08 | per-IP 半开帽 8 → 第 9 个 429 | e2e（裸帧×9） | `go test ./internal/server -run TestHalfOpenCap -count=1` | ❌ Wave 0 |
| SEC-08 | 5s 无 Hello → 1008 `hello_timeout` | e2e（裸帧）：计时断言（容忍窗 4-8s） | `go test ./internal/server -run TestHelloTimeout -count=1` | ❌ Wave 0 |
| SEC-08 | 抢跑帧 → 1002 且无 Error 帧 | e2e（裸帧）：首帧发 INPUT，断言 1002 且线上无 'E' 帧 | `go test ./internal/server -run TestPrematureFrame -count=1` | ❌ Wave 0 |
| SEC-08 | 预认证内存平坦 | 代码走查（Accept 前守卫区零分配）+ flood 测试内存采样（脆弱断言，作参考不门禁） | 走查 + 上列 flood 用例 | — |
| CORE-04 | ro 模式 INPUT 被丢弃 | e2e：默认启动握手后发 INPUT，断言无回显且连接存活；Welcome.mode=="ro" | `go test ./internal/server -run TestReadOnlyDropsInput -count=1` | ❌ Wave 0 |
| CORE-04 | ro 下 RESIZE 放行（D-13） | e2e：ro 握手后发 RESIZE，spawn `stty size` 断言尺寸跟随 | `go test ./internal/server -run TestReadOnlyAllowsResize -count=1` | ❌ Wave 0 |
| CORE-04 | --writable 后 INPUT 生效 | e2e：cfg.writable=true，INPUT 回显（现有 TestEchoPTY 改造加握手） | `go test ./internal/server -run TestWritableEcho -count=1` | ❌ Wave 0 |
| CORE-06 | ping 按间隔到达且连接保活 | e2e：interval=200ms，裸帧客户端断言 1s 内收到 ≥2 个 ping（op=0x9），回 pong 后连接存活 | `go test ./internal/server -run TestPingKeepalive -count=1` | ❌ Wave 0 |
| CORE-06 | pong 超时 → 连接关闭 | e2e（裸帧，不回 pong）：断言连接在 interval+timeout 内被关（Open Questions Q2 的注入点） | `go test ./internal/server -run TestPongTimeout -count=1` | ❌ Wave 0 |
| （D-05） | 关闭码全集静态合规 | unit：proto 常量表 ⊆ D-05 集合；全部 `c.Close` 调用点 grep 断言无 1005/1006/1015/4000 段字面量 | `go test ./internal/proto -run TestCloseCodeSet -count=1` | ❌ Wave 0 |
| （D-02） | Hello 未知字段忽略 | unit：Hello JSON 混入未知字段解析成功 | `go test ./internal/proto -run TestHelloIgnoresUnknown -count=1` | ❌ Wave 0 |
| 既有回归 | Phase 1 生命周期五测 + echo | 既有测试改造：全部 Dial 后补 Hello 握手（helper 函数统一收口） | `go test ./internal/server -count=1` | ✅ 需改造 |

**既有测试改造点（重要）：** Phase 1 的 e2e（TestEchoPTY 等，e2e_test.go 358 行）Dial 后直接发 INPUT——Phase 2 上线握手后这些测试全部需要先过 Hello。做一个 `dialAndHandshake(t, addr, mode)` 测试 helper 统一收口，避免逐测试散落握手代码。

### Sampling Rate

- **Per task commit:** `go test ./... -count=1`（秒级；TestPongTimeout 若保留 10s 真值则例外，见 Open Questions Q2）
- **Per wave merge:** `go test -race -count=1 ./...` + `pnpm -C web build`
- **Phase gate:** 全量绿 + 手动 checklist（浏览器 DevTools 观察：ro 标题前缀、键盘无响应、Network WS 帧面板可见 ping/pong、1009 文案）→ `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/proto/proto_test.go` — Hello/Welcome/Error 编解码 + 未知字段忽略 + 关闭码集合断言
- [ ] `internal/server/handshake_test.go` — 守卫链（400/429/409 顺序）+ Hello 超时 + 抢跑帧
- [ ] `internal/server/limits_test.go` — RES-01 五个攻击面用例 + 裸帧 helper（可拆 `rawws_test.go`）
- [ ] `internal/server/keepalive_test.go` — ping/pong 两用例
- [ ] 既有 `e2e_test.go` 改造 — 握手 helper 收口
- [ ] 框架安装：无 — stdlib + 既有依赖全覆盖

## Security Domain

### Applicable ASVS Categories（ASVS L1 基线）

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | partial | Phase 2 是协议握手（版本断言），无凭据（真认证 Phase 3）；补偿控制：per-IP 半开帽 8 + 5s 未认证超时 + 子协议 400 预检（D-03/D-04） |
| V3 Session Management | no | Phase 3 ticket / Phase 6 重连 |
| V4 Access Control | **yes** | ro/rw 边界服务端强制（INPUT 丢弃，D-13）；前端 disableStdin 仅 UX 层；测试断言 ro 下 INPUT 零效果（TestReadOnlyDropsInput） |
| V5 Input Validation | **yes** | 三层上限（RES-01）；Hello JSON 未知字段忽略但 version 严格等值；抢跑帧 1002；子协议 token 精确匹配；`ClampDim` 沿用 |
| V6 Cryptography | no | Phase 3 TLS；本 phase 文档维持"非 loopback 请套 TLS 反代"警示 |
| V9 Communications | **yes** | WS 原生 ping/pong 保活（CORE-06）；permessage-deflate 默认关（D-17，库默认 VERIFIED）；关闭码全集合规（D-05，库线值校验兜底） |
| V14 Configuration | **yes** | 上限全部常量不开 flag（D-10）；仅新增 `--writable`/`--ping-interval` 两 flag（D-15/D-16）；http.Server 显式 ReadHeaderTimeout（Pitfall 8） |

### Known Threat Patterns for WS 协议层（Go coder/websocket）

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 预认证分片洪水（Bandit O(n²) 模式） | DoS | 库流式重组结构上无 O(n²)（read.go VERIFIED）+ 层2 分片计数 + 层3 字节限 + 完成时限兜底空分片 |
| 预认证内存放大（ttyd protocol.c:288-296 模式） | DoS | Accept 前守卫链零分配 + SetReadLimit 4KiB 预认证档（D-11）+ 库固定缓冲（8B 头 + 125B 控制） |
| 半开连接洪泛（SYN 式） | DoS | per-IP 帽 8 + HTTP 429（D-04）+ ReadHeaderTimeout 5s（Pitfall 8，HTTP 层） |
| 慢滴（slowloris/slow-frame） | DoS | 5s Hello 超时（D-04）+ 每消息完成时限（Pattern 3）+ ReadHeaderTimeout |
| 关闭码不合规（1006 上线） | Tampering | 库 validWireCloseCode 线上拒发（close.go:279-293 VERIFIED）+ D-08 不发明新码 + 静态断言测试 |
| 子协议伪造/前缀绕过 | Elevation of Privilege | token 拆分精确匹配（非 Contains）+ Accept 后 assert 兜底（D-03） |
| CSWSH（跨站 WS 劫持） | Elevation of Privilege | 库默认同源校验（accept.go:228-260 VERIFIED）；禁用 InsecureSkipVerify；allow-list 属 Phase 3 |
| 扫描器协议指纹探测 | Information Disclosure | 攻击面路径不发 Error 帧（D-06）；400/429 在升级前返回，不暴露 WS 栈细节 |
| 反代后 IP 聚合绕过/误伤 | DoS | 不轻信 X-Forwarded-For（Pitfall 7）；半开窗口短（5s）缓解；可信头处理属 Phase 3/7 |

## Sources

### Primary（HIGH 置信——本机源码逐行核实，全部本会话完成）

- **coder/websocket v1.8.15 模块缓存**（~/go/pkg/mod/github.com/coder/websocket@v1.8.15/）：
  - `read.go:88-105` SetReadLimit per-message 语义 + limit+1 探测；`read.go:107` 默认 32768；`read.go:457-498` mr.read 流式分片循环（每调用至多一非空帧，空分片内部吞）；`read.go:521-541` limitReader 超限自动 writeError(1009)；`read.go:31-33` 官方 time.AfterFunc 超时指引；`read.go:190-194,213-216,291-301,317-323,387-391` RSV/opcode/控制帧/mask/自动 pong 合规路径；`read.go:41-49` Read=io.ReadAll
  - `close.go:27-59` StatusCode 全常量（含 1013 TryAgainLater）；`close.go:86-99` Close 5s+5s 有界握手；`close.go:130-155` CloseNow；`close.go:279-293` validWireCloseCode 拒发 1004/1005/1006/1015；`close.go:307` reason ≤123B
  - `conn.go:216-259` Ping 阻塞等 pong/"须与 Reader 并发"；`conn.go:171-199` ctx 到期 AfterFunc→c.close()；`conn.go:147-149` Subprotocol()
  - `accept.go:102-182` Accept 流程（hijack/101）；`accept.go:55-59` CompressionDisabled 默认；`accept.go:141-144,266-275` 子协议不匹配不拒绝（selectSubprotocol 返回 ""）；`accept.go:228-260` Origin 默认同源；`accept.go:184-226` verifyClientRequest 预检错误集
  - `frame.go:52-102,110-173` 帧线格式读写（测试 helper 对照面）；`frame.go:106` maxControlPayload=125
  - `write.go:428-430` writeError=writeClose
- **Bandit 官方 advisory 原文**（github.com/mtrudel/bandit/security/advisories/GHSA-vg8x-66vg-5pxh，本会话 WebFetch）：CVE-2026-65623，2026-07-24 发布，CVSS 8.7（CVSS:4.0），CWE-407；每帧对全量累积 iolist 跑 `IO.iodata_length/1` → O(n²)；`max_fragmented_message_size` 默认 8MB 限字节不限帧；"The zero-byte-frame guard does not help"；"Read timeout is idle-only"（重组回调内不触发）；影响 >=1.11.0 <1.12.1
- **wesh 现状代码**（本会话 Read）：`internal/proto/proto.go:16-48`（'0'/'1' 常量、DecodeResize、ClampDim）、`internal/server/server.go:72-117`（Attach 现状骨架）、`cmd/wesh/main.go:31-51`（parseArgs/flag 风格）、`web/src/main.ts`（concat/showStatus/onclose 现状）、`internal/server/e2e_test.go:26-79`（测试惯例）
- **GOROOT go1.26.3**：`net/http/server.go:2940-2943` http.Serve 零超时
- **web/node_modules/@xterm/xterm 6.0.0 typings**：`xterm.d.ts:92` `disableStdin?: boolean`（"Whether input should be disabled."）

### Secondary（MEDIUM 置信——标准/官方文档引用）

- RFC 6455 §7.4：1005/1006/1015 保留值 MUST NOT 写入 Close 帧；1004 仅 "Reserved"；1000-2999 协议保留；3000-3999 IANA；4000-4999 私有；1013 Try Again Later 为 IANA 后续注册（服务过载，客户端应在用户操作后重连）——与库 close.go 常量集及 validWireCloseCode 交叉一致 [CITED: datatracker.ietf.org/rfc6455 §7.4；正文 WebFetch 截断，逐字引用以库源码注释 + PITFALLS.md 既有 HIGH 引用为准]
- RFC 6455 §4.1/§5.2：握手子协议规则（服务端不回选 ≠ 拒绝）；帧线格式 [CITED]
- `.planning/research/PITFALLS.md` Pitfall 1/9 + 验证清单（既有 HIGH 一手审计）
- `.planning/phases/01-pty/01-RESEARCH.md`（栈版本与测试/CI 惯例沿用）

### Tertiary（LOW 置信——推断/惯例，planner 定稿）

- 浏览器不分片 + 代理分片余量分析（A1）；reason 机器串拼写（A4）；完成时限 10s 推荐值（A5）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH——零新增依赖；全部 API 行为本版本源码逐行核实
- Architecture: HIGH——守卫链/握手状态机/计数读循环/ping 装配全部基于 VERIFIED 库语义；完成时限为官方注释指引模式
- Pitfalls: HIGH——8 条中 7 条带本版本源码行号或官方 advisory 原文；Pitfall 7（反代聚合）为部署面分析
- 协议参数（A1-A5）: MEDIUM——有依据的推荐值，planner 定稿即锁定

**Research date:** 2026-08-14
**Valid until:** 2026-09-13（30 天；库版本已钉 v1.8.15，协议层结论不随生态快速漂移）
