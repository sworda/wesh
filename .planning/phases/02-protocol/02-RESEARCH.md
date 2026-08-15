# Phase 2: 协议基线 - Research

**Researched:** 2026-08-15
**Domain:** WebSocket 协议层（coder/websocket v1.8.15 服务端 + 浏览器 WS API 前端）
**Confidence:** HIGH（核心 API 语义全部经 v1.8.15 源码逐行核实；CONTEXT.md 决策与库现实的一处冲突已定位并升级）

## Summary

本 phase 的全部库级技术问题已通过 **module cache 中 coder/websocket v1.8.15 的源码精读**闭环（该版本即 go.mod 钉版、亦经 proxy.golang.org 确认为最新版，2026-06-15 发布）。CONTEXT.md 的 D-01~D-16 设计与库能力高度吻合：SetReadLimit 自动 1009、Ping/Pong、子协议协商、关闭码合规、压缩默认关，全部有源码级证据。D-03 的"HTTP 层预检 + Accept 后 assert"双重闸不仅可行，而且是库官方注释明示的正确姿势（空子协议永远协商成功，Accept 不会替你拒绝）。

**一处硬冲突须升级用户裁决（Open Question 1）：D-09 三层上限中的"分片数 32"在 coder/websocket 上无任何 API 可落地。** 库内部流式重组分片，空 continuation 帧在 `mr.read` 内部循环被吞掉，对应用完全不可见。深入分析后结论是：分片数层的防御价值被库的设计结构性覆盖——Bandit CVE-2026-65623 的根因是 O(n²) 重组（官方修复是 running byte count 而非分片计数），coder/websocket 重组为流式 O(1)/帧，1 字节×N 帧洪水在 16KiB 累积字节硬顶处即被 1009 切断；仅剩 0 字节空帧洪水（纯 CPU、带宽受限、无内存增长）无应用层钩子。等效防线与残余风险见"Common Pitfalls > Pitfall 1"。

其余关键发现：① 带 deadline 的 ctx 传给 Read/Write 时，到期会经 AfterFunc **关闭整条连接**（非仅中断当次调用）——读循环 ctx 必须永无 deadline，5s 未认证超时应用 time.AfterFunc + Close(1008) 实现；② Ping(ctx) 超时只返回错误不关连接，pong 超时路径须应用自行 CloseNow；③ 库自动 1009 的 close reason 是库内字符串（`read limited at %d bytes`），D-12 的机器串 reason 只覆盖应用主动关闭路径，前端分派必须以 **code 而非 reason** 为准；④ close reason 上限 123 字节，库层拒发 1005/1006/1015（D-05 纪律由库兜底）。

**Primary recommendation:** 按 CONTEXT.md 决策实施，唯"分片数 32"一层降级为"累积字节硬顶（等效拦截非空帧洪水）+ 结构性风险披露"，其余照 D-01~D-16 落地；测试以 Go e2e（stdlib testing + -race）为主，前端经 tsc + UAT 收口。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 握手与帧编码
- **D-01:** 控制面与数据面统一 binary 帧 = 1 字节类型 + 载荷；控制帧载荷为 JSON。类型分配：`'H'` Hello（C→S）、`'W'` Welcome（S→C）、`'E'` Error（S→C）、`'0'` INPUT/OUTPUT、`'1'` RESIZE（D-16 沿用）；`'X'` EXIT、`'T'` TITLE、`'P'` PREFS 等字母空间为后续 phase 占住。前后端各一个 switch 分派，proto 包唯一事实源。**ARCHITECTURE.md §2.8 的"控制面 text JSON"方案作废** — **Reversibility:** one-way
- **D-02:** Hello 最小 schema `{version, cols, rows}`；协议纪律钉死**未知字段必须忽略**（前后端同纪律）。Phase 3 加 `ticket`、Phase 5 加 `attach`/`mode` 只是加字段 — **Reversibility:** one-way
- **D-03:** 子协议协商在 HTTP 层拦截：`Accept` 前检查 `Sec-WebSocket-Protocol` 头，不含 `wesh.v1` 返回 HTTP 400（零 WS 资源分配）；`Accept` 后 `c.Subprotocol()` assert 兜底。前端 `new WebSocket(url, ['wesh.v1'])`。
- **D-04:** per-IP 未认证（Hello 未完成）半开连接上限 **8**，超限 Accept 前 HTTP 429；5s 未认证超时；Hello 前收到任何数据帧（抢跑帧）按协议违规关闭（码值见 D-05）。正常浏览器秒发 Hello 不受限；NAT 多人场景 Hello 已完成不计入。

#### 关闭码与错误码
- **D-05:** 关闭码全集 = `{1000 正常, 1001 服务端下线, 1002 协议错误(未知帧/抢跑/畸形), 1008 策略违反(认证/权限/版本), 1009 超限, 1011 内部错误, 1013 踢出可重试}`，1005/1006/1015 永不发送。ROADMAP 成功准则 2 的集合 `{1000,1008,1009,1011,1013}` 漏写 1002（Phase 1 已在用，server.go:114）——以本并集为准 — **Reversibility:** one-way
- **D-06:** Error 帧按受众分治——正常客户端会遇到的错误发 `Error{code,message}` + 关闭码：`version_mismatch`(1008)、`server_error`(1011)；攻击面路径直接关闭码不发 Error：`unknown_frame`/抢跑帧(1002)、超限(1009，库自动)——不给攻击者反馈面。
- **D-07:** Error JSON = `{code, message}`：code 机器可读 snake_case，message 英文人话（前端直接展示）；所有主动关闭的 close reason 带同名机器串（RFC ≤123 字节） — **Reversibility:** costly
- **D-08:** 1001（Phase 7 优雅下线）与 1013（Phase 5 背压踢出）进 proto 常量表**占位不实现**，注释标注启用 phase；Phase 2 只产生 1000/1002/1008/1009/1011 发送路径。纪律：应用层超限检测（分片数/累积字节）复用 1009，不得发明新码或自定义 4000 段。

#### 三层资源上限
- **D-09:** 三层上限初始值：单帧 **16KiB** / 每消息分片数 **32** / 每消息累积字节 **16KiB**（C→S）。依据：合法流量极小（键盘 INPUT 字节级、RESIZE/Hello JSON <200B、粘贴几 KB，浏览器 WS API 不分片）；分片数 32 对空帧攻击是关键防线。数值经 research + 负载测试标定（research flag），Phase 9 回填默认值。
- **D-10:** 上限全部**常量**进 proto/server 包（注释标定来源与依据），**不开 CLI flag**；Phase 7 配置文件（OPS-09）统一收口可配性。
- **D-11:** SetReadLimit 两档切换：`Accept` 后先 `SetReadLimit(4KiB)`（Hello JSON ~100 字节，余量两个数量级），Hello/Welcome 完成后 `SetReadLimit(16KiB)`——SEC-08 预认证窗口单连接可占内存最小化。

#### 超限提示（不得吞错误）
- **D-12:** ① 前端 `onclose` 按 1009 分派人话文案（"超出服务端消息上限"类，不提 flag）；② 服务端 stderr 打单行事件（对端、码值、reason）——Phase 8 才升级为结构化日志（OPS-08）；③ close reason 带机器串（`message_too_big`/`fragment_limit`）。

#### 只读模式与保活
- **D-13:** ro 边界 = **只丢弃 INPUT，RESIZE 放行**（与 ttyd `-R` 行为一致）。Phase 5 多客户端时 RESIZE 才收写权限门 + 最小公共矩形仲裁（MULTI-04）。
- **D-14:** Welcome 帧带 `mode`（`"ro"`/`"rw"`）；ro 时前端 `term.options.disableStdin = true` + `document.title` 加 `"[ro] "` 前缀。零新 UI 组件。
- **D-15:** 可写 flag = `--writable`（布尔，help: "allow client input (default read-only)"），全名无短选项 — **Reversibility:** one-way
- **D-16:** 保活参数：ping 间隔默认 **5s** + `--ping-interval` flag（0=禁用）+ pong 超时 **10s** 常量 — flag 名 **Reversibility:** one-way

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
- **ROADMAP.md 准则 2 校正** — 关闭码集合补上 1002，下次 roadmap 维护时处理。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-04 | 默认只读模式（丢弃客户端输入），显式开启可写后才接受输入 | D-13/D-14/D-15 落地路径已验证：服务端 Attach 循环 INPUT 分支按 atomic ro 标志丢弃；前端 `disableStdin` 选项经 xterm.d.ts:92 核实存在；`--writable` flag 接入现有 parseArgs（main.go:31-51） |
| CORE-06 | WS ping/pong 保活，间隔可配置，防止反代空闲超时断连 | `Conn.Ping(ctx)` 语义经 conn.go:216-259 源码核实：发 ping 等 pong、必须与 Reader 并发（pong 由读循环处理）、超时不关连接（应用自行 CloseNow）；浏览器自动回 pong（MDN）；`--ping-interval` flag 接 parseArgs |
| SEC-08 | 认证完成前零缓冲分配（防 ttyd 式预认证内存放大/崩溃） | D-03/D-04/D-11 三道预认证闸全部零缓冲：子协议 400 在 HTTP 层（Accept 前）、per-IP 429 在 Accept 前、SetReadLimit(4KiB) 两档切换经 read.go:97-105 核实可运行期原子生效（下一条消息起）；库 limitReader 流式执行无预分配 |
| RES-01 | WS 消息三层上限：单帧长度、分片数量、累积字节数 | **部分可落地，一处冲突**：单帧+累积字节由 SetReadLimit 覆盖（超限库自动 1009，read.go:521-541）；分片数层无 API（read.go:457-498 空帧内部吞掉）——见 Open Question 1 与 Pitfall 1 的等效防线分析 |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 子协议 400 预检 / per-IP 429 / 409 门 | Go server HTTP 层（Attach 前置守卫区） | — | Accept 前拦截零 WS 资源分配（D-03/D-04）；库 Accept 内嵌 Origin/版本校验在其后 |
| 帧类型/版本/错误码/关闭码常量 | `internal/proto`（单一事实源） | 前端 TS 常量手工对齐 | D-01/D-16 纪律；前后端各一个 switch 分派 |
| Hello/Welcome/Error 编解码 | `internal/proto` | — | encoding/json stdlib；未知字段忽略由 json.Unmarshal 默认行为保证 |
| 三层上限执行（单帧/累积字节） | coder/websocket 库（SetReadLimit） | server 包常量声明 | 库默认可靠即不自建（Established Pattern）；limitReader 流式执行 |
| 5s 未认证超时 / 抢跑帧关闭 | Go server（Attach 握手段） | — | time.AfterFunc + Close(1008)；首帧非 'H' → 1002 |
| ro/rw 输入门 | Go server（INPUT 分支丢弃） | 前端 disableStdin（不产生输入） | 双保险：服务端丢弃是安全边界，前端禁输入是 UX |
| ping/pong 保活 | Go server（独立 goroutine + Ping API） | 浏览器自动回 pong | 库 Ping 必须与 Reader 并发（conn.go:218-220）——正好匹配现有单 reader 结构 |
| onclose 码值分派文案 | 前端 main.ts（showStatus 面板） | — | D-12①；code 驱动非 reason 驱动（库 1009 reason 不可控） |
| stderr 单行事件 | Go server | — | D-12②；Phase 8 升级 slog 前的过渡形态 |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/coder/websocket | v1.8.15（已钉，go.mod:6） | WS 服务端全部帧语义 | [VERIFIED: proxy.golang.org 2026-06-15 最新版] 零依赖、context-first、默认 32KiB 读上限、Autobahn 合规（上游 CI）；Phase 1 已落地 |
| Go stdlib encoding/json | go 1.26.3 | 控制帧 JSON 编解码 | D-01 控制帧载荷 JSON；未知字段忽略是 Unmarshal 默认行为（D-02 纪律零成本） |
| Go stdlib flag | go 1.26.3 | `--writable`/`--ping-interval` | 沿用 main.go:31-51 parseArgs 现有模式（全名无短选项） |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @xterm/xterm | 6.0.0（已钉，web/package.json:10） | `term.options.disableStdin` ro 模式 | [VERIFIED: web/node_modules/@xterm/xterm/typings/xterm.d.ts:92] Welcome mode=ro 时运行期置位 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| coder/websocket | gorilla/websocket | gorilla 默认**无读上限**（PITFALLS Pitfall 1 明示裸奔）；无 context-first；项目曾因归档风波迁移。本项目 Phase 1 已选 coder/websocket，无切换理由 |
| time.AfterFunc + Close(1008) 实现 5s 超时 | 首读传 5s deadline ctx | deadline ctx 到期经 AfterFunc 直接 `c.close()`（TCP 层关闭，**无关闭帧**，客户端只见 1006 等效）[VERIFIED: conn.go:188-199]；AfterFunc + Close 能在线上发出 1008 码与机器串 reason（D-07 抓包可辨） |

**Installation:**

```bash
# 本 phase 零新增依赖——Go 侧 coder/websocket v1.8.15 与 web 侧 @xterm/xterm 6.0.0
# 均于 Phase 1 钉版落地，go.mod / pnpm-lock.yaml 无需变更
```

**Version verification:** coder/websocket v1.8.15 经 `proxy.golang.org/.../@latest` 核实即最新版（2026-06-15 发布，commit 9c8faadc）[VERIFIED: proxy.golang.org]；@xterm/xterm 6.0.0 经 web/package.json:10 + node_modules typings 核实[VERIFIED: web/package.json:10]。

## Package Legitimacy Audit

**本 phase 不安装任何新外部包**（零新增 Go module / npm 依赖——协议层全部能力由 Phase 1 已钉依赖 + stdlib 覆盖）。Legitimacy Gate 不适用。

| Package | Registry | 状态 | Disposition |
|---------|----------|------|-------------|
| github.com/coder/websocket v1.8.15 | proxy.golang.org | Phase 1 已装；本期核实为最新版、上游 Autobahn CI 在跑 | 沿用，无需操作 |
| @xterm/xterm 6.0.0 | npm（pnpm-lock） | Phase 1 已装；disableStdin  typings 核实存在 | 沿用，无需操作 |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
## Architecture Patterns

### System Architecture Diagram

```mermaid
flowchart LR
    subgraph Browser["浏览器 (web/src/main.ts)"]
        UI[xterm.js 6<br/>disableStdin 按 mode 置位]
        WSAPI["new WebSocket(url, ['wesh.v1'])<br/>onopen → 发 Hello 首帧"]
    end

    subgraph Server["Go server (internal/server + internal/proto)"]
        direction TB
        GATE["Attach 前置守卫区（Accept 前，零 WS 资源）<br/>① 子协议 400 → ② 409 原子门 → ③ per-IP 半开 429"]
        ACCEPT["websocket.Accept<br/>Subprotocols=['wesh.v1'] / Origin 同源默认 / 压缩关"]
        PREAUTH["预认证窗口<br/>SetReadLimit(4KiB) · 5s 超时(1008)<br/>首帧必须 Hello，抢跑 → 1002"]
        SESSION["已认证会话<br/>SetReadLimit(16KiB) · ro 丢 INPUT<br/>ping 5s / pong 超时 10s → CloseNow"]
        PROTO["internal/proto<br/>帧类型/版本/错误码/关闭码常量<br/>Hello·Welcome·Error JSON 编解码"]
    end

    PTY[(PTY Session<br/>Phase 1 已有)]

    UI --> WSAPI
    WSAPI -- "GET /ws + Sec-WebSocket-Protocol: wesh.v1" --> GATE
    GATE -- 全过 --> ACCEPT --> PREAUTH
    PREAUTH -- "Hello{version,cols,rows}" --> SESSION
    SESSION -- "Welcome{mode} / OUTPUT / ping" --> WSAPI
    SESSION -- "INPUT(rw 时) / RESIZE" --> PTY
    PTY -- "输出块 → OUTPUT 帧" --> SESSION
    PREAUTH -.->|违规| CLOSE1["1002/1008/1009<br/>+ stderr 单行事件"]
    SESSION -.->|超限(库自动 1009)/错误| CLOSE1
```

**数据流主线**（attach 成功路径）：HTTP 升级请求 → 三道前置闸 → Accept（子协议回显 wesh.v1）→ 4KiB 读上限窗口 → 首帧 Hello 校验（版本不符 → Error{version_mismatch}+1008）→ Welcome 下发 mode → 16KiB 稳态 → INPUT/RESIZE 入 PTY、OUTPUT 出浏览器；ping goroutine 与单 reader/单 writer 并行。

### Recommended Project Structure

```
internal/
├── proto/
│   └── proto.go        # 本期扩展主场：'H'/'W'/'E' 类型字节、Subprotocol 常量、
│                       # Error code 表、关闭码常量表（1001/1013 占位注释）、
│                       # Hello/Welcome/Error 编解码、上限常量（注释标定依据）
├── server/
│   ├── server.go       # Attach 扩展：前置守卫区、握手段、ro 门、ping goroutine、stderr 事件
│   └── e2e_test.go     # 本期新增协议测试（沿用 startTestServer/waitExit/helperArgv 模式）
└── pty/                # 本 phase 不动
cmd/wesh/main.go        # parseArgs 加 --writable/--ping-interval；config 透传 server.New
web/src/main.ts         # 子协议、Hello 首帧、Welcome 处理（mode→disableStdin/title）、onclose 按码分派
```

### Pattern 1: Attach 前置守卫区（Accept 前三道闸，顺序敏感）

**What:** 在 `websocket.Accept` 之前按固定顺序执行三个拒绝路径——全部 HTTP 层响应，零 WS 资源分配。
**When to use:** D-03/D-04 要求的预认证攻击面收敛。
**Why this order:** ① 子协议 400 无状态、最便宜、拦截扫描器最早（D-03 明示"最早被拦"）；② 409 原子门失败时不消耗 per-IP 配额；③ per-IP 429 需要 increment+defer decrement 生命周期，放最后使被拒绝的连接不产生计数器 bookkeeping。

```go
// Source: 依据 accept.go:141-144,266-276 核实行为设计（空子协议永远协商成功，
// 故 400 预检不可省略）；闸顺序为本 research 推荐，具体实现 planner discretion
func (s *Server) Attach(w http.ResponseWriter, r *http.Request) {
	// ① 子协议预检（D-03）：headerTokens 语义——逗号分隔多值都要查
	if !hasSubprotocol(r.Header, proto.Subprotocol) { // "wesh.v1"
		http.Error(w, "subprotocol wesh.v1 required", http.StatusBadRequest)
		return
	}
	// ② 409 原子门（Phase 1 现状，D-09；Phase 5 才改）
	if !s.attached.CompareAndSwap(false, true) {
		http.Error(w, "another client is already attached", http.StatusConflict)
		return
	}
	// ③ per-IP 半开上限（D-04）：Accept 前 HTTP 429
	ip := remoteIP(r) // net.SplitHostPort(r.RemoteAddr)；反代场景见 Pitfall 6
	if !s.halfOpen.acquire(ip, 8) {
		s.attached.Store(false)
		http.Error(w, "too many pending connections", http.StatusTooManyRequests)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{proto.Subprotocol}, // 回显 wesh.v1
	})
	if err != nil {
		s.halfOpen.release(ip)
		s.attached.Store(false)
		return
	}
	// Accept 后 assert 兜底（D-03 双闸）：预检挡正常路径，assert 挡手工构造
	if c.Subprotocol() != proto.Subprotocol {
		c.Close(websocket.StatusPolicyViolation, "subprotocol_required") // 1008
		s.halfOpen.release(ip)
		s.attached.Store(false)
		return
	}
	// ...握手段（Pattern 2）
}
```

### Pattern 2: 预认证窗口（4KiB 上限 + 5s 超时 + 抢跑帧 1002）

**What:** Accept 成功到 Hello 校验通过之间的最小攻击面窗口；Hello 完成后才放大上限、释放半开计数。
**When to use:** SEC-08 / D-04 / D-11。
**关键纪律（源码核实）：**
- `SetReadLimit` 是 atomic store，**下一条消息起生效**（read.go:97-105 + 516-519：`lr.reset` 每条消息 `n = limit.Load()`），运行期从 4KiB 切 16KiB 安全。
- **读循环 ctx 永不带 deadline**：带 deadline 的 ctx 到期经 AfterFunc 直接 `c.close()`（TCP 关闭、无关闭帧）[VERIFIED: conn.go:188-199 `setupReadTimeout`]。5s 超时必须用 `time.AfterFunc` + `c.Close(1008, "hello_timeout")`，才能把码值送上 wire。
- Hello 超时的 Close 是 5s+5s 握手阻塞调用（close.go:86-99），在 AfterFunc 自有 goroutine 里跑，不阻塞 reader。

```go
// Source: conn.go:188-199（deadline ctx 关闭整条连接）、close.go:86-128（Close 语义）、
// read.go:88-105（SetReadLimit 语义）核实后设计
c.SetReadLimit(proto.PreHelloReadLimit) // 4KiB，D-11
helloDone := make(chan struct{})
timer := time.AfterFunc(proto.HelloTimeout /* 5s */, func() {
	select {
	case <-helloDone:
	default:
		c.Close(websocket.StatusPolicyViolation, "hello_timeout") // 1008，D-07 机器串
	}
})
defer timer.Stop()

_, data, err := c.Read(context.Background()) // ctx 无 deadline——硬约束
if err != nil { /* 对端断开/超时关闭：走 D-11 收口 */ }
mt := data[0] // 先查 len(data)==0：空消息 → 1002 "empty_frame"（见 Open Question 2）
if mt != proto.Hello {
	c.Close(websocket.StatusProtocolError, "frame_before_hello") // 1002 抢跑帧，D-04
}
h, ok := proto.DecodeHello(data[1:]) // json.Unmarshal 默认忽略未知字段（D-02）
if !ok || h.Version != proto.Subprotocol {
	proto.WriteError(c, proto.ErrVersionMismatch, "...") // 'E' 帧 + 1008，D-06
}
close(helloDone)
s.halfOpen.release(ip) // Hello 完成即不计半开（D-04：NAT 场景正常浏览器不受限）
c.SetReadLimit(proto.MaxMessageBytes) // 16KiB，稳态
proto.WriteWelcome(c, mode) // 'W' {"mode":"ro"|"rw"}，D-14
```

### Pattern 3: ping 保活 goroutine（与单 reader 并行）

**What:** 独立 goroutine 按 ticker 调 `c.Ping(ctx10s)`；pong 超时即 CloseNow。
**When to use:** CORE-06 / D-16。
**源码核实的三条纪律：**
1. **"Ping must be called concurrently with Reader"**（conn.go:218-220 原话）——pong 由读循环 `handleControl` 处理（read.go:324-337），现有单 reader 循环天然满足，**不得为 ping 再开 reader**。
2. Ping ctx 超时只返回错误、**不关连接**（conn.go:251-258 select 路径无 close）——应用须自行 `CloseNow()`（对端已不应答，关闭握手无意义；客户端见 1006 属正常合成码，不违反 D-05——D-05 约束的是服务端 wire 发送）。
3. 写路径 `writeFrameMu` 串行化所有帧（write.go:288-293），`Write` 非压缩时单帧完成（write.go:110-111），故 Ping 与 onChunk 的 OUTPUT 写并发安全、无帧交错。

```go
// Source: conn.go:216-259（Ping 语义）、read.go:317-337（pong 处理）、write.go:288-293（写串行化）
func (s *Server) keepalive(ctx context.Context, c *websocket.Conn, interval time.Duration) {
	if interval == 0 {
		return // --ping-interval 0 = 禁用，D-16
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done(): // Attach 返回时 cancel——进 sync.Once 同一生收口（Established Pattern）
			return
		case <-t.C:
			pctx, cancel := context.WithTimeout(context.Background(), proto.PongTimeout) // 10s，D-16
			err := c.Ping(pctx)
			cancel()
			if err != nil {
				s.logEvent("pong_timeout", c) // stderr 单行，D-12②
				c.CloseNow()
				return
			}
		}
	}
}
```

### Anti-Patterns to Avoid
- **给读循环 ctx 加 deadline 当"读取超时"用：** deadline 到期关闭整条连接且空闲期同样触发（长 idle 终端会话会被误杀）[VERIFIED: conn.go:188-199]。读循环 ctx 永远 `context.Background()`（Phase 1 现状 server.go:91 已正确）。
- **试图应用层数分片：** 库内部吞掉空帧（read.go:457-498），应用数不到；用 `c.Reader()` 手数只能数到非空帧，而那些已被 16KiB 字节顶拦截——白做工。见 Pitfall 1。
- **fork/patch 库或在 listener 层包 net.Conn 解析帧头：** 前者违反"库默认可靠即不自建"；后者 hijack 后 TLS（Phase 3）之下只见密文，且等于手写帧解析（PITFALLS C9 预警）。
- **前端按 reason 文本分派：** 库自动 1009 的 reason 是库内字符串 `read limited at %d bytes`（read.go:526-529）不可控；onclose 分派只认 `ev.code`。
- **Close reason 超 123 字节：** `maxCloseReason = maxControlPayload - 2 = 123`（close.go:307）；超长时库降级发 1011 且返回错误（close.go:295-305）——reason 机器串全部短 snake_case。
- **手写 `Sec-WebSocket-Protocol` 解析用 `==` 比整头：** 头部可多值逗号分隔；应 split+trim+EqualFold（库 accept.go:357-368 `headerTokens` 同语义）。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WS 分片重组/累积上限 | 自写 continuation 追加循环 | `SetReadLimit`（limitReader 流式，read.go:500-541） | ttyd 两大预认证 CVE 本体；Bandit O(n²) 同款教训 |
| 超限关闭 | 自写 1009 关闭帧 | 库自动 `writeError(StatusMessageTooBig)` | read.go:526-529；应用从 Read 错误 `errors.Is(err, websocket.ErrMessageTooBig)` 拿钩子打 stderr 事件 |
| ping/pong 帧处理 | 自写 ping 帧/pong 等待 | `c.Ping(ctx)` + 库自动回 pong | conn.go:216-259；read.go:317-323（收 ping 自动回 pong） |
| 关闭握手/码合规 | 自写 close 帧 | `c.Close(code, reason)` / `CloseNow()` | 库拒发 1005/1006/1015（close.go:279-293），握手 5s+5s 超时内置 |
| mask/UTF-8/控制帧合规 | 自校验 | 库 readLoop | read.go:190-216：rsv 位、未 mask、控制帧 >125B、分片控制帧全部自动 1002 |
| 子协议/Origin 协商 | 自写 Sec-* 头处理 | `AcceptOptions{Subprotocols}` + HTTP 预检 | accept.go 全文；注意空子协议永远协商成功（accept.go:26-27），预检不能省 |
| CLI flag 解析 | 自写 argv 扫描 | stdlib `flag`（沿用 parseArgs） | main.go:31-51 现有模式；DurationVar 原生支持 "5s" 语法 |

**Key insight:** 本 phase 的应用层代码只补库没有的四样：**子协议 HTTP 预检、per-IP 半开计数、5s 未认证超时、ro INPUT 门**——其余全部是"调库 + 常量钉版"。这是 CONTEXT.md"库默认可靠即不自建"原则的直接推论，也是 SEC-08 的结构保证：库的 limitReader 在认证前只做流式截断，无任何消息级缓冲预分配。

## Common Pitfalls

### Pitfall 1: 分片数上限无 API——D-09 第三层的现实落差（本 phase 最重要发现）

**What goes wrong:** D-09 要求"每消息分片数 32"作为空帧攻击关键防线；ROADMAP 成功准则 1 预期空帧/百万小帧洪水被 1009 关闭。**源码核实结论：coder/websocket v1.8.15 无任何分片计数钩子**——`mr.read` 内部循环跳过空 continuation 帧（read.go:457-479），应用经 `Read`/`Reader` 拿到的只有重组后的完整消息，空帧对应用不可见。

**Why it happens:** 库把分片重组作为内部实现细节（这正是 Phase 1 选它的理由——不手写重组）。Go 生态两个主流库（coder/gorilla）均无分片计数 API；WebSearch 未发现任何公开的分片数限制模式。

**等效性分析（为什么这不削弱 RES-01 的防御目标）:**
- Bandit CVE-2026-65623 的官方修复（GHSA-vg8x-66vg-5pxh 核实）是**连接结构体内维护 running byte count**——即流式字节计数，恰恰不是分片数上限。该 CVE 的根因是每帧全遍历累积缓冲的 O(n²) 重组；coder/websocket 为流式 O(1)/帧（每帧 payload 一次性流入调用方缓冲，read.go:481-496），结构性免疫。
- **1 字节 × N 帧洪水**：累积字节硬顶 16KiB 在第 16385 字节处自动 1009，内存 ≤16KiB、CPU ≈ 16K 次帧头解析（毫秒级）。字节层完整拦截 ✓
- **0 字节 × ∞ 帧洪水**：字节层永不触发（不累积）。残余 = 单 goroutine 帧头解析 CPU 消耗，受攻击者带宽约束（客户端 mask 帧 ≥6 字节/帧），无内存增长（SEC-08 内存平坦目标不受影响）。爆破解面：预认证窗口被 5s 超时 + per-IP 8 上限盒住；认证后须先成为唯一 attach 客户端（409 门）。
- ROADMAP 准则 1 的"空帧→1009 关闭"与库现实的措辞冲突，建议随已 defer 的"准则 2 补 1002"一并校正。

**How to avoid:** 接受"两层硬顶（单帧/累积字节，库执行）+ 预认证三道闸（应用执行）"的等效防线；空帧洪水残余风险显式记录。**须用户确认**（Open Question 1）。
**Warning signs:** planner/executor 试图引入 `unsafe`、反射、listener 层 conn 包装去数帧——全是反模式（见 Anti-Patterns）。

### Pitfall 2: 带 deadline 的 Read ctx 静默杀连接

**What goes wrong:** 直觉地想给 `c.Read` 传超时 ctx 实现"读取超时"，结果连接在第一个空闲间隔就被库 AfterFunc 关掉（conn.go:188-199：`c.close()` = TCP 关闭，无关闭帧），且空闲期与洪水期无法区分——长 idle 终端会话必被误杀。
**Why it happens:** gorilla 的 `SetReadDeadline` 心智模型迁移；coder/websocket 的 ctx 语义是"操作边界"而非"deadline 设置"，到期即连接级关闭。
**How to avoid:** 读循环 ctx 恒 `context.Background()`；一切超时（5s 未认证、pong 10s）走 AfterFunc/Ping-ctx + 显式 Close/CloseNow。

### Pitfall 3: Ping 实现了但 pong 永远等不到

**What goes wrong:** 把 ping 塞进读循环顺序执行（读循环阻塞在 Read 上等输入，ticker 饿死），或 ping 后不处理错误。
**Why it happens:** 忽视 conn.go:218-220 的并发要求；忽视 Ping 超时不关连接（conn.go:251-258）。
**How to avoid:** 独立 goroutine + ticker（Pattern 3）；Ping 错误 → stderr 事件 + CloseNow + goroutine 退出；goroutine 终结挂进 Attach 的 ctx cancel，进 sync.Once 同一生收口（server.go:37,165 模式）。

### Pitfall 4: per-IP 计数器泄漏/双重释放

**What goes wrong:** Accept 失败、子协议 assert 失败、Hello 完成、连接异常断开四条路径的 release 时机不齐——计数器单调上涨，最终正常用户全被 429。
**Why it happens:** acquire/release 跨 HTTP 层与 WS 层两个生命周期。
**How to avoid:** acquire 只在 Accept 前一次；release 恰好一次——用 `sync.Once` 或"Hello 完成 or Attach 返回"二选一的 defer 结构（planner discretion）；测试覆盖四条路径的计数断言。
**Warning signs:** 压测 9+ 短连接后正常浏览器 429。

### Pitfall 5: 子协议预检把合法多值头误判

**What goes wrong:** `r.Header.Get("Sec-WebSocket-Protocol") == "wesh.v1"` 整头比对——客户端发 `wesh.v1, other` 时被误拒（或大小写差异）。
**How to avoid:** split(",")+TrimSpace+EqualFold 逐 token 比（库 accept.go:357-368 同语义）；Accept 后 `c.Subprotocol()` assert 兜底（D-03 第二闸）。

### Pitfall 6: per-IP 键取错（带端口 / 反代后全是一个 IP）

**What goes wrong:** `r.RemoteAddr` 含端口（"1.2.3.4:5678"）直接当键——每连接一个"新 IP"，上限形同虚设；反代部署时所有客户端同为代理 IP，8 上限误杀 NAT 后正常用户。
**How to avoid:** `net.SplitHostPort(r.RemoteAddr)` 取主机部分[标准库行为]；反代场景的 X-Forwarded-For 信任属 Phase 3+（认证/SEC-07）范畴，本期文档注明"per-IP 上限在直连部署下有效"即可[ASSUMED——部署形态决策非本期范围]。

### Pitfall 7: ro 模式只在前端禁输入

**What goes wrong:** 只设 `disableStdin`——非浏览器客户端（wscat/脚本）照常发 INPUT 写进 PTY，"只读"名存实亡。
**How to avoid:** 服务端 INPUT 分支按 ro 标志丢弃是安全边界（D-13），前端 disableStdin 只是 UX 层；e2e 必须覆盖"ro 下裸 WS 客户端发 INPUT 不进 PTY"。
## Code Examples

Verified patterns from official sources（所有库行为断言均附 v1.8.15 源码行号）:

### 帧类型与关闭码常量（proto 包扩展现状锚点）

现状 proto.go:16-20（本 phase 在此扩展，**逐字引用**）:
```go
const (
	Input  = '0' // 0x30, C→S, raw bytes → 写 master
	Resize = '1' // 0x31, C→S, JSON {"cols":C,"rows":R} → 钳制 1..1000 后 Setsize
	Output = '0' // 0x30, S→C, master 读块直发
)
```
[VERIFIED: internal/proto/proto.go:16-20]（本会话 Read）

新增常量形状（planner 按 D-01/D-05/D-06 填充，组织方式 discretion）：
```go
// Source: D-01/D-05/D-06 决策 + proto.go 现有常量风格
const (
	Hello   = 'H' // C→S, JSON {"version","cols","rows"}
	Welcome = 'W' // S→C, JSON {"mode":"ro"|"rw"}
	Error   = 'E' // S→C, JSON {"code","message"}
	// 'X' EXIT / 'T' TITLE / 'P' PREFS —— 类型字节占住，语义 Phase 6/4（D-01）
	Subprotocol = "wesh.v1" // D-03；HTTP 预检、AcceptOptions、Hello.version 三处复用同一常量
)
// 关闭码常量表：1000/1002/1008/1009/1011 本期产生发送路径；
// 1001（Phase 7）/1013（Phase 5）占位不实现（D-08）。库常量直接用：
// websocket.StatusNormalClosure / StatusProtocolError / StatusPolicyViolation /
// StatusMessageTooBig / StatusInternalError [VERIFIED: close.go:28-52]
```

### Hello/Welcome/Error JSON（未知字段忽略纪律）

```go
// Source: proto.go:22-37 DecodeResize 现有模式（显式 json tag + ok 返回值）扩展；stdlib encoding/json
type HelloPayload struct {
	Version string `json:"version"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	// 未知字段由 json.Unmarshal 默认忽略——D-02 演化纪律的零成本实现
}
type WelcomePayload struct {
	Mode string `json:"mode"` // "ro" | "rw"，D-14
}
type ErrorPayload struct {
	Code    string `json:"code"`    // snake_case 机器串：version_mismatch / server_error，D-06/D-07
	Message string `json:"message"` // 英文人话，前端直接展示
}
```

### 超限可见性三腿（D-12）的服务端钩子

```go
// Source: read.go:526-529（ErrMessageTooBig 包装）+ D-12②
_, data, err := c.Read(ctx)
if err != nil {
	if errors.Is(err, websocket.ErrMessageTooBig) {
		// 库已自动发 1009（reason 为库内字符串，前端按 code 分派）；
		// 应用补 stderr 单行事件——三腿之②
		fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=1009 reason=message_too_big\n", ip)
	}
	// ...
}
```

### 前端：子协议 + Hello 首帧 + Welcome 处理 + onclose 按码分派

```typescript
// Source: MDN WebSocket() constructor（protocols 参数）+ D-03/D-14/D-12①；
// 帧常量与 proto.go 手工对齐（D-16 注释模式沿用）
const HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45; // 'H'/'W'/'E'
const ws = new WebSocket('ws://' + location.host + '/ws', ['wesh.v1']);

ws.onopen = () => {
  opened = true;
  fit.fit();
  // Hello 必须是第一帧（抢跑 = 1002）：version/cols/rows，D-02
  ws.send(concat(new Uint8Array([HELLO]),
    enc.encode(JSON.stringify({ version: 'wesh.v1', cols: term.cols, rows: term.rows }))));
};

ws.onmessage = (ev) => {
  const buf = new Uint8Array(ev.data as ArrayBuffer);
  switch (buf[0]) { // 与 server switch 对称（D-01）
    case OUTPUT: term.write(buf.subarray(1)); break;
    case WELCOME: {
      const w = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
      if (w.mode === 'ro') { // D-14
        term.options.disableStdin = true; // [VERIFIED: xterm.d.ts:92]
        document.title = '[ro] ' + document.title;
      }
      break;
    }
    case ERROR: /* JSON {code,message} → showStatus 展示 message */ break;
  }
};

ws.onclose = (ev) => {
  // 按 code 分派（D-12①）：1000 Session ended / 1009 超限人话 /
  // 1008 策略（version_mismatch）/ 1011 服务端错误 / 其余 Connection lost
  // 1006 永不来自 wire（RFC6455 §7.4）——网络断开时浏览器本地合成 1006，wasClean=false
};
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| gorilla/websocket（默认无读上限） | coder/websocket（默认 32KiB、context-first） | Phase 1 已选定 | 本 phase 全部读上限语义基于后者源码核实 [VERIFIED: read.go:107] |
| nhooyr.io/websocket 旧 import path | github.com/coder/websocket | 库 2022 年迁 org | go.mod:6 已是新路径 [VERIFIED: go.mod:6] |
| ttyd 手写 lws 分片重组 | 库内流式重组 + SetReadLimit | Phase 1 架构决策 | 两大预认证 CVE 的结构性消除；本 phase 补三层上限 |
| ping 保活靠 TCP keepalive | 应用层 WS ping（5s） | 本 phase（D-16） | conn.go:222 注释 "TCP Keepalives should suffice for most use cases"——但反代空闲超时看的是**应用层流量**，TCP keepalive 多数反代不计入，WS ping 才是对nginx/Cloudflare 空闲切断的对症解 [CITED: conn.go:222 + PITFALLS nginx 节] |

**Deprecated/outdated:**
- 自写 WS 帧层任何部分：本项目的库选型已使其成为反模式（PITFALLS C1/C9）。
- ARCHITECTURE.md §2.8 控制面 text JSON：D-01 已作废，binary 统一分派。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | nginx `proxy_read_timeout` 默认 60s、Cloudflare 免费计划 WS 空闲 ~100s 切断——ping 5s 默认值的"显著小"论证 | D-16 / State of the Art | 低：5s 对任何主流反代空闲阈值都安全两个数量级以上；数值仅影响文档论证不影响实现。来源为 PITFALLS（MEDIUM），本期未重新核实 |
| A2 | Hello JSON 载荷 ~100 字节（4KiB 预认证上限"余量两个数量级"的 sizing 依据） | D-11 / Pattern 2 | 低：`{"version":"wesh.v1","cols":220,"rows":50}` 实测约 45 字节量级；即使字段名变长也远在 4KiB 内 |
| A3 | 浏览器 WS API 不产生分片（`send()` 一次一消息，JS 无分片控制面） | D-09 依据 / Open Question 1 | 低：MDN send() 文档无分片 API；即使浏览器内部分片巨帧，16KiB 字节顶同样拦截。本轮未逐字核实 MDN send 页 |
| A4 | 反代部署下 `r.RemoteAddr` 为代理 IP（per-IP 计数误伤 NAT 用户的可能性） | Pitfall 6 | 低：标准 net/http 行为；本期直连部署定位下不影响，Phase 3 SEC-07 再处理可信头 |

**若上表为空则无需用户确认**——本期 4 条均为低风险论证性假设，无阻塞实现者；唯一需用户确认项在 Open Question 1（D-09 分片层）。

## Open Questions

1. **D-09 分片数上限 32 在 coder/websocket 上无 API——是否接受等效防线替代？**（须用户裁决，建议在 plan 前确认）
   - What we know: 库流式重组吞掉空帧（read.go:457-479），应用层任何手段数不到分片；1 字节×N 帧洪水被 16KiB 累积字节硬顶完整拦截（1009 自动）；Bandit 官方修复同为字节计数非分片计数；fork 库/包 conn 数帧均已被判反模式。
   - What's unclear: 用户对"三层上限"字面完整性的坚持程度；ROADMAP 准则 1"空帧→1009 关闭"措辞是否接受校正为"空帧洪水不崩溃、内存平坦（残余 CPU 消耗带宽受限）"。
   - Recommendation: **接受等效防线**——两层硬顶（单帧/累积字节）+ 预认证三道闸（400/429/5s 超时）+ 409 单客户端门；`proto` 常量表保留分片上限值的注释位但标注"库不暴露、依赖等效防线"，残余风险写入文档。e2e 测试相应断言"1M 空帧洪水下服务存活、内存平坦、其他功能不受影响"而非"连接被 1009 关闭"。
2. **空完整消息（0 字节 binary message）的处置码**
   - What we know: `c.Read` 对空消息返回 `([]byte{}, nil)` 无错误；Phase 1 现状 `len(data)==0 → continue` 静默跳过（server.go:102-104）[VERIFIED: server.go:102-104]；D-05 的 1002 桶含"畸形"。
   - What's unclear: 空消息算"畸形"（1002 关）还是无害噪声（继续静默跳过）。
   - Recommendation: Hello 前的空消息按抢跑/畸形 1002（`empty_frame`）；Hello 后维持静默跳过——浏览器永不发空消息，发了也无害，收紧只为协议洁癖。此为 planner 可定级，无需用户。
3. **Hello.version 取值形状**
   - What we know: D-02 只定 schema `{version, cols, rows}`；子协议 token 已是 `wesh.v1`。
   - Recommendation: version 取字符串 `"wesh.v1"` 与子协议常量同源复用（`proto.Subprotocol`），前端 Hello 与 `new WebSocket` 第二参引用同一 TS 常量——双写漂移面最小。planner 可定级。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | 全部 Go 实现/测试 | ✓ | go1.26.3 linux/amd64（与 go.mod:3 钉版一致）[VERIFIED: `go version`] | — |
| Node.js | web 构建（tsc+vite） | ✓ | v24.13.0（CI 钉 24 一致）[VERIFIED: `node --version`] | — |
| pnpm | web 依赖/构建 | ✓ | 11.21.0（CI 钉版一致）[VERIFIED: `pnpm --version`] | — |
| CGO（-race 需要） | go test -race | ✓ | 默认启用（CI 注释明示不设 CGO_ENABLED）[VERIFIED: .github/workflows/ci.yml:15-17] | — |
| Docker（Autobahn 套件可选加强项） | PITFALLS C9 验证清单 | 未探测 | — | 可跳过：coder/websocket 上游 CI 已跑 Autobahn（autobahn_test.go 排除项文档化），wesh 不手写帧层，边际价值低；本期以应用层模糊测试（空帧/百万小帧/超限帧）替代 |

**Missing dependencies with no fallback:** 无
**Missing dependencies with fallback:** Docker/Autobahn（以上游库合规 + 应用层模糊测试替代）
## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `-race`（CI 固化：ubuntu + macOS 双平台）[VERIFIED: .github/workflows/ci.yml:3-17] |
| Config file | 无（零配置；go.mod 即版本源） |
| Quick run command | `go test -race -count=1 ./internal/server/ ./internal/proto/` |
| Full suite command | `go vet ./... && go test -race -count=1 -v ./... && pnpm -C web build`（与 CI 两腿一致） |

前端无单测 runner（web/package.json 无 test script）[VERIFIED: web/package.json:5-8]——前端改动由 `tsc` 类型检查（build 内含）+ Go e2e（裸 WS 客户端驱动）+ 期末人工 UAT（config human_verify_mode: end-of-phase）三层收口，与 Phase 1 相同。**不引入 vitest**——超出本 phase 范围。

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RES-01 | 超限消息（>16KiB）→ 1009 + 连接关闭 + stderr 事件 | e2e | `go test -race -run TestOversize1009 ./internal/server/` | ❌ 本期新建 |
| RES-01 | 16KiB 边界：16384B 消息正常 echo，16385B → 1009 | e2e | `go test -race -run TestReadLimitBoundary ./internal/server/` | ❌ 本期新建 |
| RES-01 | 分片消息（1 字节 × N，经客户端 `Writer()` 流式逐帧写）在 16KiB 顶被 1009 | e2e | `go test -race -run TestFragmentedFlood1009 ./internal/server/` | ❌ 本期新建 |
| RES-01 | 空帧洪水（`Writer().Write(nil)` × 大量）服务存活、内存平坦、回声功能正常（**不断言 1009**，见 Open Question 1） | e2e | `go test -race -run TestEmptyFragmentFloodResilience ./internal/server/` | ❌ 本期新建 |
| RES-01 | 预认证 4KiB 档：Hello 前 >4KiB 消息 → 1009 | e2e | `go test -race -run TestPreHelloReadLimit ./internal/server/` | ❌ 本期新建 |
| SEC-08 | 无子协议握手 → HTTP 400（零 WS 分配）；错子协议 → 400 | e2e | `go test -race -run TestSubprotocolRequired ./internal/server/` | ❌ 本期新建 |
| SEC-08 | 5s 内无 Hello → 1008 `hello_timeout`（测试用短超时注入或真实 5s，planner 定） | e2e | `go test -race -run TestHelloTimeout ./internal/server/` | ❌ 本期新建 |
| SEC-08 | per-IP 第 9 条半开连接 → HTTP 429；Hello 完成后不计 | e2e | `go test -race -run TestHalfOpenPerIP429 ./internal/server/` | ❌ 本期新建 |
| SEC-08 | 抢跑帧（首帧非 'H'）→ 1002 `frame_before_hello` | e2e | `go test -race -run TestFrameBeforeHello ./internal/server/` | ❌ 本期新建 |
| CORE-04 | 默认 ro：Hello→Welcome{mode:"ro"}；INPUT 帧被丢弃（PTY 无回显）；RESIZE 放行 | e2e | `go test -race -run TestReadOnlyDefault ./internal/server/` | ❌ 本期新建 |
| CORE-04 | `--writable`：Welcome{mode:"rw"}；INPUT 正常写 PTY（echo 断言） | e2e | `go test -race -run TestWritableMode ./internal/server/` + parseArgs 单测 | ❌ 本期新建 |
| CORE-06 | ping 保活：连接空闲 >2 个间隔仍存活（库自动回 pong） | e2e | `go test -race -run TestPingKeepalive ./internal/server/`（短间隔注入） | ❌ 本期新建 |
| CORE-06 | pong 超时：客户端 Dial 后停止 Read（库只在读路径回 pong，read.go:317-323）→ 服务端 10s 后断开 | e2e | `go test -race -run TestPongTimeout ./internal/server/`（短超时注入） | ❌ 本期新建 |
| D-06 | version_mismatch → 先收 'E' Error 帧再收 1008 | e2e | `go test -race -run TestVersionMismatch ./internal/server/` | ❌ 本期新建 |
| D-05 | 未知帧（Hello 后）→ 1002；全程无 1006（沿用 TestUnknownFrame1002 模式） | e2e | 现有 `TestUnknownFrame1002` 扩展 | ✅ 扩展（e2e_test.go:285-316） |
| D-03 | 前端 `new WebSocket(url, ['wesh.v1'])` + Hello 首帧 + Welcome 处理 + onclose 分派 | 人工 UAT | `pnpm -C web build` 后浏览器实测清单 | manual-only（无前端 runner，期末 UAT 收口） |

**测试可注射性注意**：5s/10s 超时常量若硬编码将使 e2e 慢/脆——planner 应将超时做成包级变量（测试可覆写）或 `server.New` 参数注入（沿用 exitf 注入模式，server.go:44 先例）[VERIFIED: server.go:44-54]。**分片洪水客户端构造**：`c.Writer(ctx, MessageBinary)` 每次 `Write` 调用产生一个非 fin 帧、`Close()` 补 fin 帧（write.go:223-264）——测试客户端可精确构造分片流，无需手写帧。

### Sampling Rate
- **Per task commit:** `go test -race -count=1 ./internal/server/ ./internal/proto/`（+ 前端任务附 `pnpm -C web build`）
- **Per wave merge:** `go vet ./... && go test -race -count=1 ./... && pnpm -C web build`
- **Phase gate:** 全量绿 + 人工 UAT（ro 行为、onclose 文案、devtools 观察 ping/pong 帧与关闭码）后 `/gsd-verify-work`

### Wave 0 Gaps
- [ ] 测试辅助：`dialHello(t, wsURL, opts)`——封装"带子协议 Dial + 发 Hello + 收 Welcome"（现有 `startTestServer`/`waitExit`/`helperArgv` 复用，e2e_test.go:106-149）[VERIFIED: e2e_test.go:106-149]
- [ ] 超时注入点：包级变量或 New 参数（见上"可注射性注意"）
- [ ] 框架安装：无——stdlib testing 已覆盖全部需求

## Security Domain

security_enforcement: true，ASVS Level 1（config.json）。

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | proto 包单一事实源（D-01）；类型字节/关闭码常量一处定义，前后端手工对齐纪律（D-16 注释模式） |
| V2 Authentication | 部分（本期无凭据认证，属 Phase 3） | 本期交付"预认证面收敛"：Hello 握手段 + 三道前置闸；SEC-01/02/03 均为 Phase 3 |
| V3 Session Management | no | —（会话生命周期 Phase 6） |
| V4 Access Control | yes（模式级） | ro/rw 服务端 INPUT 门（D-13）：服务端丢弃是边界，前端 disableStdin 仅 UX（Pitfall 7） |
| V5 Input Validation | yes | Hello schema 校验 + 未知字段忽略（D-02）；ClampDim [1,1000]（proto.go:40-48 沿用）；空消息/抢跑帧 1002；库级 mask/rsv/控制帧合规（read.go:190-216） |
| V6 Cryptography | no | —（TLS Phase 3；本期无加密原语） |
| V7 Error Handling & Logging | yes | Error 帧 `{code,message}` 不泄露内部（D-06/D-07）；stderr 单行事件（D-12②）；库自动 1009 的 ErrMessageTooBig 钩子 |
| V9 Communications | yes | 子协议 wesh.v1 双重闸（D-03）；关闭码全集纪律（D-05，库层拒发 1005/1006/1015 兜底 close.go:279-293）；permessage-deflate 默认关（accept.go:278-294，D-17） |

### Known Threat Patterns for Go + coder/websocket + 浏览器 WS

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 预认证内存放大（ttyd 式分片累积） | DoS | SetReadLimit 两档（4KiB→16KiB，D-11）+ 库流式 limitReader 无预分配 [VERIFIED: read.go:500-541] |
| 1 字节 × N 帧洪水（Bandit 形） | DoS | 累积字节硬顶自动 1009；库重组 O(1)/帧，无 O(n²) [VERIFIED: read.go:481-496 + GHSA-vg8x-66vg-5pxh 修复对照] |
| 0 字节空帧洪水 | DoS（残余） | 无应用层钩子——5s 超时盒住预认证窗口；per-IP 8 上限；409 单客户端门；残余 CPU 带宽受限（Open Question 1 显式记录） |
| 半开连接慢 loris（不发言占坑） | DoS | 5s 未认证超时（1008 hello_timeout）+ per-IP 8（HTTP 429），D-04 |
| 抢跑帧/畸形帧探测 | Tampering / Info Disclosure | 1002 直关不发 Error 帧（D-06：不给攻击者反馈面） |
| 子协议扫描/旧客户端 | — | HTTP 400 最早拦截，零 WS 资源（D-03） |
| CSWSH（跨站 WS 劫持） | Elevation of Privilege | 库默认 Origin 同源校验（accept.go:228-260；Phase 1 已依赖）；白名单属 Phase 3 SEC-04 |
| 压缩侧信道/内存常驻 | Info Disclosure | permessage-deflate 不协商（CompressionDisabled 默认，D-17） |
| 关闭码信息泄露 | Info Disclosure | 攻击路径只给码不给 Error 帧（D-06）；reason 短机器串 ≤123B（D-07，库上限 close.go:307） |

## Sources

### Primary (HIGH confidence)
- **coder/websocket v1.8.15 源码**（module cache 精读，本会话）——read.go（SetReadLimit/limitReader/分片重组/控制帧合规）、conn.go（Ping/setupReadTimeout）、write.go（写串行化/Writer 分片）、accept.go（Subprotocols/Origin/压缩）、close.go（StatusCode 常量/Close/validWireCloseCode/123B reason 上限）、autobahn_test.go（上游合规排除项）
- **proxy.golang.org** `/github.com/coder/websocket/@latest` — v1.8.15 为最新版，2026-06-15 发布
- **MDN** CloseEvent.code（关闭码表、1005/1006/1015 保留值）与 WebSocket() constructor（protocols 参数）
- **@xterm/xterm 6.0.0 typings**（web/node_modules，本会话 Read）— xterm.d.ts:92 `disableStdin?: boolean`
- **本仓库源码**（本会话 Read）— proto.go / server.go / main.go / e2e_test.go / ci.yml / go.mod / web/package.json（所有 in-repo 离散值断言的出处）
- **.planning/research/PITFALLS.md**（2026-08-13，含 ttyd 源码审计一手行号）

### Secondary (MEDIUM confidence)
- **GHSA-vg8x-66vg-5pxh / CVE-2026-65623**（Bandit O(n²) 重组；官方修复为 running byte count）— WebSearch 命中官方 advisory + CNA 记录交叉验证
- Go 生态无分片计数 API 的负面断言——WebSearch 未发现反例 + 两库源码/API 面核实（负面断言已尽合理核查）

### Tertiary (LOW confidence)
- nginx 60s / Cloudflare 100s 空闲阈值（A1，沿用 PITFALLS，本期未重新核实）
- Context7：月度配额耗尽不可用；ctx7 CLI 未安装——库语义已全部改经源码一级核实覆盖，无信息缺口

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH——零新增依赖；现有两依赖经 registry/typings 核实
- Architecture: HIGH——守卫区顺序、握手段、ping 装配全部有源码行号支撑；唯一结构性偏差（分片层）已升级为 Open Question 1
- Pitfalls: HIGH——Pitfall 1/2/3 为本期源码精读的直接产物；Pitfall 4-7 为实现期纪律
- 验证架构： HIGH——框架与 CI 现状文件核实；测试命令均为拟定名（planner 可调整）

**Research date:** 2026-08-15
**Valid until:** 2026-09-14（库钉版 v1.8.15 不变则核心结论长期有效；升级库版本时须重核 read.go/conn.go 行号引用）
