# Phase 2: 协议基线 - Pattern Map

**Mapped:** 2026-08-15
**Revised:** 2026-08-15（D-09 修订外科清理：readCounted/maxFragments 常量/rawws 裸帧 helper/分片计数矩阵行作废；RESEARCH 行号索引修正为现行 Pattern 1-3；守卫区 per-IP/409 顺序与升档序列按 planner 裁决对齐；onopen 补 helloSent 门）
**Files analyzed:** 10（6 个修改既有文件 + 4 个新建测试文件——rawws_test.go 已随 D-09 修订作废移除）
**Analogs found:** 10 / 10（9 个有库内 analog——其中 6 个为被改文件自身；全部新逻辑另以 02-RESEARCH.md 的 VERIFIED 模式为源）

## 0. 本地 Analog 核查结论（已实证）

与 Phase 1（greenfield、零本地 analog）根本不同：**Phase 2 的全部修改对象都存在于库内，且每个文件的最佳 analog 就是它自己的 Phase 1 现状**——本 phase 是"在既有骨架上扩展/重构"，不是从零新建。

| 候选来源 | 状态 | 结论 |
|----------|------|------|
| `internal/proto/proto.go`（48 行） | 存在，已读 | **proto 扩展的 exact analog**：常量块/显式 json tag/Decode+ok 惯例/包doc 纪律直接沿用 |
| `internal/server/server.go`（174 行） | 存在，已读 | **Attach 重构的 exact analog**：守卫区（409 门）/单 reader switch/原子态/sync.Once 收口全部在位 |
| `cmd/wesh/main.go`（89 行） | 存在，已读 | **flag 扩展的 exact analog**：config 结构体 + parseArgs + run() 错误路径风格 |
| `web/src/main.ts`（169 行） | 存在，已读 | **前端扩展的 exact analog**：concat/buf[0] 分派/showStatus 三态/onclose 骨架 |
| `internal/server/e2e_test.go`（358 行） | 存在，已读 | **全部新服务端测试的 exact analog**：startTestServer/waitExit/CloseError 断言/负例 HTTP 断言 |
| `cmd/wesh/main_test.go`（92 行） | 存在，已读 | **proto_test.go 与 flag 测试的 role-match analog**：表驱动 + captureFd |
| `02-RESEARCH.md` Pattern 1-3 | 全部经 coder/websocket v1.8.15 源码逐行核实 | **新逻辑（守卫链/握手状态机/ping/前端握手）的唯一模式源**——库内无对应现状代码 |
| ttyd 源码 | 存在 | **仅缺陷对照面，禁止参考实现**（CONTEXT canonical_refs 明示；protocol.c:288-298 为三层上限的反面教材） |

**结论：** planner 写 PLAN.md 时，"骨架/纪律/风格"抄库内现状文件行号，"新逻辑"抄 02-RESEARCH.md 行号；两者缺一不可。

## 0.1 模式来源行号索引

| 来源 | 关键区段（行号） |
|------|------------------|
| `internal/proto/proto.go` | 包 doc 纪律（1-11）；帧常量块（16-20）；显式 json tag struct（22-26）；Decode+ok 惯例（28-37）；ClampDim（39-48） |
| `internal/server/server.go` | imports（5-20）；Server 结构体原子态（24-38）；New 装配+goroutine 启动点（40-54）；Attach 守卫+Accept+读循环（72-117）；onChunk 组帧（122-131）；lifecycle/wsDisconnected/terminate 收口（135-173） |
| `cmd/wesh/main.go` | config 结构体（22-26）；parseArgs（31-51）；run() 错误/退出码路径（53-84）；http.Serve 改造点（79） |
| `web/src/main.ts` | 帧常量对齐注释（6-9）；concat（68-76）；onmessage 分派（79-84）；onData 发送门（86-93）；sendResize 防护（95-102）；showStatus（118-136）；onerror/onclose（138-169） |
| `internal/server/e2e_test.go` | startTestServer（106-122）；waitExit（125-135）；helperArgv/TestHelperProcess（142-199）；负例 HTTP 状态断言（213-223）；CloseError 读取循环（267-278, 298-312） |
| `cmd/wesh/main_test.go` | 表驱动 TestParseArgs（15-44）；captureFd（46-65） |
| `02-RESEARCH.md` | Pattern 1 守卫链（180-224）；Pattern 2 预认证窗口（226-263）；Pattern 3 pinger（265-298）；Anti-Patterns（300-306）；Don't Hand-Roll（308-320）；Pitfall 1-7（322-371）；Code Examples（帧常量 376-402 / Hello·Welcome·Error JSON 404-421 / 超限钩子 423-436 / 前端握手·ro·onclose 438-475）；Open Questions (RESOLVED)（501-513）；Validation Architecture（527-569：测试映射 539-559、Wave 0 566-569）；Security Domain（571-600） |
| `02-CONTEXT.md` | D-01~D-16 决策（17-44）；Integration Points（96-102） |

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/proto/proto.go`（扩展） | utility（协议单一事实源） | transform | **自身现状**（proto.go:1-48）+ RESEARCH §Code Examples 帧常量/JSON（376-421） | exact（自扩展）+ external-doc |
| `internal/server/server.go`（重构 Attach） | controller（HTTP/WS 网关） | request-response + streaming | **自身现状**（server.go:72-173）+ RESEARCH Pattern 1-3（180-298） | exact（骨架沿用）+ external-doc（新逻辑） |
| `cmd/wesh/main.go`（扩展） | controller（入口/装配/生命周期） | event-driven | **自身现状**（main.go:22-84） | exact（自扩展） |
| `web/src/main.ts`（扩展） | component | streaming（WS 客户端） | **自身现状**（main.ts:6-169）+ RESEARCH §Code Examples 前端段（438-475） | exact + external-doc |
| `internal/proto/proto_test.go`（新建） | test | — | `cmd/wesh/main_test.go` 表驱动（15-44）+ e2e 注释惯例 | role-match |
| `internal/server/handshake_test.go`（新建） | test | — | `internal/server/e2e_test.go`（同包，helper 直接复用） | exact（同包） |
| `internal/server/limits_test.go`（新建） | test | — | e2e_test.go + RESEARCH §测试可注射性注意（559：库客户端 Writer 构造分片流） | exact + external-doc |
| `internal/server/keepalive_test.go`（新建） | test | — | e2e_test.go + RESEARCH Pattern 3（265-298） | exact + external-doc |
| `internal/server/e2e_test.go`（改造） | test | — | **自身** + 新写 `dialHello` helper（RESEARCH §Wave 0 Gaps 566-569；尺寸参数化 cols/rows） | exact（自改造） |
| `cmd/wesh/main_test.go`（扩展） | test | — | **自身**表驱动（15-44） | exact（自扩展） |

（注：`internal/server/rawws_test.go` 裸帧 helper 已随 D-09 2026-08-15 修订作废并从本表移除——攻击面用例全部库客户端可构造，见 §测试文件段与 02-VALIDATION.md Wave 0 节。）

## Pattern Assignments

### `internal/proto/proto.go`（utility, transform）——扩展

**Analog:** 自身现状 + `02-RESEARCH.md` §Code Examples 帧类型与关闭码常量（376-402）+ Hello/Welcome/Error JSON（404-421）

**包 doc 纪律（现状 lines 1-11，扩展时改写保留三段式：帧格式 / 前端对齐 / 关闭码纪律）：**
```go
// Package proto 是 wesh 数据面协议的单一事实源。
//
// 帧格式：binary frame = 1 字节 ASCII 类型 + 载荷（ttyd 同构形状、wesh 自定义取值）。
// 前端 web/src/main.ts 的帧常量与本文件手工对齐（D-16）。
```

**帧常量块（现状 lines 16-20，新增 'H'/'W'/'E' 直接入此块；'X'/'T'/'P' 占位注释沿用现状 lines 6-7 的预留写法）：**
```go
const (
	Input  = '0' // 0x30, C→S, raw bytes → 写 master
	Resize = '1' // 0x31, C→S, JSON {"cols":C,"rows":R} → 钳制 1..1000 后 Setsize
	Output = '0' // 0x30, S→C, master 读块直发
)
```

**显式 json tag struct + Decode 返回 ok 惯例（现状 lines 22-37，Hello/Welcome/Error 编解码照此形态；Hello 必须保持 Unmarshal 默认行为=未知字段忽略，禁止 DisallowUnknownFields——D-02）：**
```go
// resizePayload 显式 json tag，防字段名漂移。
type resizePayload struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// DecodeResize 解码 RESIZE 帧载荷 {"cols":C,"rows":R}。
// 解码失败返回 ok=false（调用方静默丢弃，不关连接）；
func DecodeResize(payload []byte) (cols, rows int, ok bool) {
	var rp resizePayload
	if err := json.Unmarshal(payload, &rp); err != nil {
		return 0, 0, false
	}
	return ClampDim(rp.Cols), ClampDim(rp.Rows), true
}
```

**新增常量/错误码表（RESEARCH §Code Examples 376-421 行，照抄为底）：**
```go
const Subprotocol = "wesh.v1" // D-03：子协议名 = Hello.version 期望值（同一字符串，单一常量）

// 帧类型字节（D-01；'X' EXIT / 'T' TITLE / 'P' PREFS 为 Phase 4/6 占住，不实现）
const (
	Hello   = 'H' // 0x48, C→S, JSON {version, cols, rows}
	Welcome = 'W' // 0x57, S→C, JSON {version, mode}
	Error   = 'E' // 0x45, S→C, JSON {code, message}
)

// Error codes（D-06/D-07：snake_case 机器串；Error 帧 code 与 close reason 同名）
const (
	ErrVersionMismatch = "version_mismatch" // +1008（正常客户端可见，发 Error 帧）
	ErrServerError     = "server_error"     // +1011（发 Error 帧）
)
```

**关闭码表纪律（D-05/D-08）：** 全集 `{1000,1001,1002,1008,1009,1011,1013}` 进常量/注释表；**1001/1013 占位不实现**（注释标启用 phase）；发送侧值直接复用 `websocket.StatusXxx` 常量，本表用于前端对齐文档与断言测试（RESEARCH 398-402 行）。上限常量（16KiB/4KiB/5s/10s）按 D-10 进 proto 或 server 包级常量，注释标定来源与依据，**不开 CLI flag**；分片数 32 仅注释位（D-09 修订，见 §server.go 段作废声明）。

---

### `internal/server/server.go`（controller, request-response + streaming）——重构 Attach

**Analog:** 自身现状（守卫门/读循环/收口骨架）+ `02-RESEARCH.md` Pattern 1-3（新逻辑逐段照抄）

**Imports（现状 lines 5-20，新增 `net`（SplitHostPort）/`time`/`encoding/json` 按现状分组风格：stdlib → 第三方 → 本地）：**
```go
import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/web"
)
```

**Server 结构体原子态惯例（现状 lines 24-38，ro 标志/握手完成标志沿用 atomic——CONTEXT line 94；per-IP 半开计数器挂为本结构体字段，数据结构属 discretion）：**
```go
type Server struct {
	sess  *pty.Session
	exitf func(code int)

	attached atomic.Bool                    // D-09：单客户端原子门
	conn     atomic.Pointer[websocket.Conn] // 当前已 attach 的 WS 连接（onChunk 写端 / 1000 关闭用）
	// ...
	childExited atomic.Bool
	termOnce    sync.Once // 两条终结路径收口，exitf 只触发一次
}
```

**守卫区现状（lines 72-85）→ 扩展为三道预检（RESEARCH Pattern 1，180-224，照抄为骨架；本块 ②③ 顺序即 planner 裁决形态——per-IP 必须在 409 之前：409 在前则 429 在单客户端模型下结构性不可达（D-04 可触达性），RESEARCH Pattern 1 代码块的 409→per-IP 顺序与其自身 TestHalfOpenPerIP429 映射矛盾，以本块为准）：**
```go
func (s *Server) Attach(w http.ResponseWriter, r *http.Request) {
	// ① D-03：子协议预检（最廉价无状态，扫描器/旧客户端最早被拦）
	if !headerHasToken(r.Header, "Sec-WebSocket-Protocol", proto.Subprotocol) {
		http.Error(w, "subprotocol wesh.v1 required", http.StatusBadRequest)
		return
	}
	// ② D-04：per-IP 半开上限（map[string]int + Mutex；RemoteAddr 取 IP 部分）
	ip := clientIP(r)
	if !s.halfOpen.acquire(ip, maxHalfOpenPerIP /*8*/) {
		http.Error(w, "too many half-open connections", http.StatusTooManyRequests)
		return
	}
	// ③ 409 原子门（Phase 1 既有，保持第一位的是子协议预检）
	if !s.attached.CompareAndSwap(false, true) {
		s.halfOpen.release(ip) // exactly-once 释放
		http.Error(w, "another client is already attached", http.StatusConflict)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{proto.Subprotocol}, // 一行开启协商 + 写响应头
	})
	// ...失败路径全部 release(ip)——不变量：acquire 成功 → release 恰好一次（RESEARCH Pitfall 4，351-356 行）
	if c.Subprotocol() != proto.Subprotocol { // D-03 assert 兜底（理论不可达）
		c.Close(websocket.StatusPolicyViolation, "subprotocol_required"); /* +logEvent +释放门位与计数 */ return
	}
}
```

**`headerHasToken` 硬纪律（RESEARCH Pitfall 5，358-361 行 + Anti-Patterns 306 行）：** `Sec-WebSocket-Protocol` 是逗号分隔 token 列表，**必须按 token 拆分精确比较**，禁止 `strings.Contains`（防 `wesh.v1.evil` 前缀绕过）。

**握手状态机（RESEARCH Pattern 2，226-263 行，照抄；两档 SetReadLimit + 5s 计时器 + 抢跑 1002 + version_mismatch 1008）：**
```go
const (
	readLimitPreAuth  = 4 * 1024      // D-11：Hello JSON ~100B，余量两个数量级
	readLimitPostAuth = 16 * 1024     // D-09 修订/D-11
	helloTimeout      = 5 * time.Second // D-04
	// 分片数上限 32 仅注释位（D-09 2026-08-15 修订）：库不暴露分片计数 API
	// （read.go:457-479 空帧内部吞掉），本层由等效防线覆盖——无可执行常量
)
// c.SetReadLimit(readLimitPreAuth) → time.AfterFunc(helloTimeout, Close(1008,"hello_timeout"))
// → c.Read 等首消息（ctx 恒无 deadline，Pitfall 2）→ 空消息 → 1002 "empty_frame"（OQ2 裁决）
// → 非 'H' 即 Close(1002,"frame_before_hello") 无 Error 帧（D-04/D-06）
// → proto.DecodeHello（未知字段忽略）→ version≠wesh.v1 则 Write(Error)+Close(1008)
// （常量归属以 plans 为准：ReadLimitPreAuth/ReadLimitPostAuth 进 proto 包——02-01；
//   helloTimeout 为 server 包默认常量 defaultHelloTimeout、经 Options 可覆写——02-02）
```

**Welcome 后升档序列（顺序敏感；planner 裁决形态，落 02-02/02-03 action）：** `close(helloDone)` 停 5s 计时器 → `sess.Resize(h.Cols, h.Rows)`（Hello 携首尺寸，消除 80x24 首帧窗口）→ per-IP release（02-03：Hello 校验通过后、Welcome 发出前，恰好一次）→ `writeWelcome(mode)` → `SetReadLimit(16KiB)` → `go s.pinger(...)`（02-04）。`SetReadLimit` 内部 atomic，读循环进行中调档安全（下一条消息起生效，read.go:97-105）。

**~~分片计数读循环~~（D-09 2026-08-15 修订作废）：** 原 readCounted 方案（`c.Reader` 手数分片 + `maxFragments=32`）无库 API 可落地——库内部流式重组吞掉空 continuation 帧（read.go:457-479），应用层数不到分片；fork 库/包装 conn 逐帧计数均判反模式（RESEARCH Anti-Patterns 302-303 行）。等效防线：两层字节硬顶（SetReadLimit 库执行，超限自动 1009）+ 预认证三道闸 + 409 单客户端门；0 字节空帧洪水残余风险显式记录（RESEARCH Pitfall 1，324-337 行；e2e 断言"存活+内存平坦"而非 1009——02-05 TestEmptyFragmentFloodResilience）。**现状 server.go:95 的 `c.Read` 读循环保持不动——本 phase 无破坏性读循环改造点。**

**数据面 switch（现状 lines 105-115 骨架直接扩展——ro 分支进 `case proto.Input`）：**
```go
switch data[0] {
case proto.Input:
	s.sess.Master.Write(data[1:])     // rw
case proto.Resize:
	if cols, rows, ok := proto.DecodeResize(data[1:]); ok {
		s.sess.Resize(cols, rows)      // ro 同——RESIZE 放行（D-13）
	}
default:
	c.Close(websocket.StatusProtocolError, "unknown frame type") // 1002 现状既有
}
```
ro 时 INPUT 丢弃（D-13 安全边界在服务端）；ro 标志沿用 atomic（CONTEXT line 94）。

**ping goroutine（RESEARCH Pattern 3，265-298 行，照抄；Welcome 升档后启动，ctx 随 Attach defer cancel——禁止 r.Context()，现状 line 90 注释同源）：**
```go
const pongTimeout = 10 * time.Second // D-16 常量；正常 RTT 毫秒级，10s 极宽

func (s *Server) pinger(ctx context.Context, c *websocket.Conn, interval time.Duration) {
	if interval <= 0 { return } // --ping-interval=0 禁用（D-16）
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, pongTimeout)
		err := c.Ping(pctx)
		cancel()
		if err != nil { return } // 终结由 reader 路径收口（D-11 既有），零新终结分支
	}
}
```

**onChunk 组帧缓冲（现状 lines 30,48-50,122-131）——Welcome/Error 等 S→C 控制帧组帧复用同款 1+payload 模式（CONTEXT line 89）：**
```go
n := copy(s.frame[1:], chunk)
if err := c.Write(context.Background(), websocket.MessageBinary, s.frame[:1+n]); err != nil {
	return // 写失败：终结由 reader 路径收口（D-11），本块丢弃
}
```
注意：Welcome/Error 的写方是 Attach 握手路径而非 onChunk，写并发安全由库保证（"All methods may be called concurrently except Reader and Read"）。

**生命周期收口（现状 lines 135-173，不改）：** 5s 未认证超时、ping goroutine 的终结都必须沿既有 wsDisconnected/terminate 单一路径收口，**禁止新增 exitf 触发分支**（CONTEXT line 92 硬约束）。

**stderr 单行事件（D-12②，格式属 discretion）：** 对端、码值、reason 单行；`errors.Is(err, websocket.ErrMessageTooBig)` 归一化为 `message_too_big` 机器串（RESEARCH §Code Examples 423-436 行——库自动 1009 的 reason 不可定制，机器串落点在 stderr 而非线上 reason）。

---

### `cmd/wesh/main.go`（controller, event-driven）——扩展

**Analog:** 自身现状（全部模式在位，仅三处插入）

**config 结构体扩展（现状 lines 22-26，加 `writable bool` / `pingInterval int` 或 time.Duration——planner 定单位）：**
```go
type config struct {
	port        int
	bind        string
	showVersion bool
}
```

**flag 注册（现状 lines 32-39 风格照抄——全名无短选项，D-15/D-16）：**
```go
fs.IntVar(&cfg.port, "port", 7681, "listen port (0 = random, actual port is printed)")
fs.StringVar(&cfg.bind, "bind", "0.0.0.0", "listen address")
fs.BoolVar(&cfg.showVersion, "version", false, "print version and exit")
// 新增：
// fs.BoolVar(&cfg.writable, "writable", false, "allow client input (default read-only)")  // D-15 逐字 help
// fs.IntVar(&cfg.pingInterval, "ping-interval", 5, "WS ping interval in seconds (0 = disable)") // D-16
```

**http.Serve 改造点（现状 line 79 → http.Server + ReadHeaderTimeout——planner 裁决落 02-03 Task 1：http.Serve 零超时是预认证 slowloris 面，与 helloTimeout 同 5s 量级）：**
```go
// 现状：if err := http.Serve(ln, srv.Handler()); err != nil {
// 改为（ReadTimeout/WriteTimeout 不要设——误伤长连接语义边界）：
hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
if err := hs.Serve(ln); err != nil {
```

**config → server 装配传递（现状 line 76 `server.New(sess, os.Exit)`）：** writable/pingInterval 经 New 参数或 Server 字段注入（planner 定签名）；启动单行打印（D-07 现状 line 78）不变。

---

### `web/src/main.ts`（component, streaming）——扩展

**Analog:** 自身现状 + `02-RESEARCH.md` §Code Examples 前端段（438-475）

**帧常量对齐（现状 lines 6-9，新增 HELLO/WELCOME/ERROR 入同一块并保持对齐注释）：**
```typescript
// 帧常量与 internal/proto/proto.go 手工对齐（D-16）：'0' INPUT / '1' RESIZE / '0' OUTPUT
const OUTPUT = 0x30,
  INPUT = 0x30,
  RESIZE = 0x31;
// 新增：const HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45; // 'H' 'W' 'E'
```

**WS 构造加子协议（现状 line 65 → RESEARCH 444 行）：**
```typescript
const ws = new WebSocket('ws://' + location.host + '/ws', ['wesh.v1']); // D-03
ws.binaryType = 'arraybuffer';
```

**onopen 发 Hello 首帧（现状 lines 111-116 扩展；Hello 必须是线上首帧且携带首尺寸——RESEARCH 446-452 行；原 `sendResize` 首发被 Hello 的 cols/rows 取代）：**

**helloSent 门（顺序硬约束，02-02 Task 2）：** `term.onResize` 常驻接线（现状 line 103）在首次 `fit.fit()` 几乎必然改尺寸触发 `sendResize`——不门住则 RESIZE 抢跑 Hello 首帧，服务端握手段以 1002 `frame_before_hello` 直关（真实浏览器主路径断连，Go e2e 覆盖不到此前端时序）。`sendResize` 首行加 `if (!helloSent) return;`，onopen 内 Hello 发出后才置位：

```typescript
let helloSent = false; // 近现状 line 63 opened 声明处
ws.onopen = () => {
  opened = true;
  fit.fit(); // 此间 onResize 触发的 sendResize 被 helloSent 门吞掉；fit 后尺寸由 Hello 承载
  ws.send(concat(new Uint8Array([HELLO]),
    enc.encode(JSON.stringify({ version: 'wesh.v1', cols: term.cols, rows: term.rows }))));
  helloSent = true; // 此后窗口拖动的 onResize → sendResize 正常发送（握手已完成，协议合法）
  term.focus();
};
```

**onmessage 扩展为 switch 分派（现状 lines 79-84 单 if → RESEARCH 454-468 行；复用 concat/TextDecoder 现状件）：**
```typescript
ws.onmessage = (ev) => {
  const buf = new Uint8Array(ev.data as ArrayBuffer);
  switch (buf[0]) {
    case OUTPUT: term.write(buf.subarray(1)); break;
    case WELCOME: {
      const w = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
      mode = w.mode === 'ro' ? 'ro' : 'rw';
      if (mode === 'ro') {
        term.options.disableStdin = true;          // D-14：键盘层面即不产生 onData
        document.title = '[ro] ' + document.title; // 零新 UI 组件
      }
      break;
    }
    case ERROR:
      lastError = JSON.parse(new TextDecoder().decode(buf.subarray(1))); // 暂存，onclose 展示
      break;
    default: break; // 未知 S→C 类型忽略（前向兼容，D-02 同纪律）
  }
};
```

**onData 发送门（现状 lines 89-93 的 readyState 门保留，**不叠加 mode 门**——planner 裁决落 02-02 Task 2：disableStdin 已使 ro 下不产生 onData，服务端丢 INPUT 是真防线，避免冗余，D-13/D-14 足够）：**
```typescript
term.onData((s) => {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(concat(new Uint8Array([INPUT]), enc.encode(s)));
  }
});
```

**onclose 按码分派（现状 lines 149-169 的三分法扩展为 per-code switch——RESEARCH 470-474 行；文案沿用 showStatus 三态面板风格与全英文纪律，具体文案属 discretion）：**
```typescript
ws.onclose = (ev) => {
  if (!opened) { showStatus('Unable to connect', /* 现有文案 */, '…'); return; }
  switch (ev.code) {
    case 1000: showStatus('Session ended', /* 现有文案 */, '…'); break;
    case 1008: showStatus('Connection refused', lastError?.message ?? 'The server refused this connection.', '…'); break;
    case 1009: showStatus('Message too large', 'Input exceeded the server message size limit and the connection was closed.', '…'); break; // D-12① 不提 flag
    case 1011: showStatus('Server error', lastError?.message ?? 'The server hit an internal error.', '…'); break;
    case 1013: showStatus('Disconnected', 'The server asked this client to retry later.', '…'); break; // Phase 5 占位路径
    default:   showStatus('Connection lost', /* 现有文案 */, '…'); break; // 含 1002 与无码异常断开（1006 线上永不到达，用 !ev.wasClean/无码分支）
  }
};
```

---

### 测试文件（role: test）

**Analog:** `internal/server/e2e_test.go`（同包 helper 直接复用）+ `cmd/wesh/main_test.go`（表驱动风格）——D-09 修订后无裸帧 helper（见下）

**既有测试基建（全部新测试直接复用，禁止另造）：**
```go
// startTestServer（e2e_test.go:106-122）：sess + New(sess, exitf 捕获桩) + 127.0.0.1:0 + go http.Serve
// waitExit（e2e_test.go:125-135）：exitf 5s 超时断言
// CloseError 读取循环（e2e_test.go:267-278）：
var ce websocket.CloseError
for {
	if _, _, rerr := c.Read(ctx); rerr != nil {
		if !errors.As(rerr, &ce) {
			t.Fatalf("read terminated without CloseError: %v", rerr)
		}
		break
	}
}
if ce.Code != websocket.StatusNormalClosure { /* ... */ }
// 负例 HTTP 状态断言（e2e_test.go:213-223）：Dial 失败时 resp.StatusCode 断言（400/429 用例同款）
```

**改造点（RESEARCH §Wave 0 Gaps 566-569 行，重要）：** Phase 1 全部 e2e（TestEchoPTY/TestSecondClient409/TestExitCodePropagation/TestUnknownFrame1002/TestClientDisconnectSIGHUP）Dial 后直接发 INPUT——握手上线后全部需先过 Hello。做 `dialHello(t, ctx, wsURL, cols, rows)` helper 统一收口（Dial options 带 `Subprotocols: []string{proto.Subprotocol}` → 发 Hello → 等 Welcome 校验 mode），避免逐测试散落握手代码；**cols/rows 参数化是签名硬要求**——02-03 TestReadOnlyAllowsResize 以 (111, 44) 复用同一签名，既有测试统一传 (80, 24)，禁止硬编码。`server.New` 签名变更（Options 注入，02-02）后 startTestServer 同步改；**TestDrainBeforeAttach（e2e_test.go:84-100，无 WS 客户端）直调 New 处加第三参 `server.Options{}` 零值适配——否则编译即破**。

**~~裸帧 helper（`rawws_test.go`）~~——D-09 2026-08-15 修订作废：** 原计划因"库客户端发不出畸形帧"引入裸写帧 helper；修订后攻击面用例全部库客户端可构造——分片流用 `c.Writer(ctx, MessageBinary)` 逐 `Write` 产生非 fin 帧、`Close()` 补 fin 帧（write.go:223-264，RESEARCH §测试可注射性注意 559 行）；子协议负例用 `DialOptions{Subprotocols}` 有无/错值/多值；空帧洪水用 0 字节空消息近似（02-05 TestEmptyFragmentFloodResilience）。**禁止手写帧字节/裸 TCP Upgrade。**

**proto_test.go（表驱动风格仿 main_test.go:15-44）：** Hello/Welcome/Error 编解码往返；Hello 混未知字段解析成功（D-02）。关闭码集合的源码 grep 静态断言经 planner 裁决不做（02-01 Task 2：comment-text 反模式）——由 02-03/02-05 的 e2e CloseError 码值断言行为层覆盖。

**关键攻击面用例矩阵（D-09 修订后形态——全部库客户端构造；测试名与命令的唯一事实源为 02-VALIDATION.md Per-Task Verification Map，本表仅作 pattern 级导航）：**

| 用例 | 断言要点 |
|------|----------|
| TestOversize1009（02-05） | 库客户端发 16KiB+1 消息 → CloseError.Code==1009 + stderr `message_too_big` 单行（D-12②） |
| TestReadLimitBoundary（02-05） | 16384 正常 echo / 16385 → 1009（边界精确） |
| TestFragmentedFlood1009（02-05） | 库客户端 `Writer()` 逐 Write 写 1 字节非 fin 帧流 → 累积 16385 处 1009（等效防线主证据） |
| TestEmptyFragmentFloodResilience（02-05） | 5000 空消息洪水 → 服务存活、echo 正常、HeapAlloc 增量 <8MiB（**不断言 1009**——空帧无应用层钩子，D-09 修订残余风险显式形态） |
| TestPreHelloReadLimit（02-05） | Hello 前 >4KiB 消息 → 1009（预认证档生效） |
| TestSubprotocolRequired（02-03） | DialOptions 无子协议 → 400；错子协议 → 400；多值头含 wesh.v1 → 放行（Pitfall 5） |
| TestHalfOpenPerIP429（02-03） | MaxHalfOpenPerIP=1 注入：c1 半开 → c2 收 429；c1 随后握手成功（不误伤，acquire/release 恰好一次） |
| TestHelloTimeout（02-03） | HelloTimeout=200ms 注入：静默 → 1008 且 reason `hello_timeout` |
| TestPrematureFrame（02-03） | 握手后首帧直接 INPUT → 1002 且全程无 'E' Error 帧（D-06 零反馈） |
| TestVersionMismatch（02-03） | 先收 Error{version_mismatch} 帧再收 1008，两处机器串同名（D-07） |
| TestReadOnlyDropsInput（02-03） | ro 下发 INPUT → 200ms 静默窗口无回读（goroutine Read + select 竞速，**禁 deadline ctx**——Pitfall 2）且连接存活 |
| TestReadOnlyAllowsResize（02-03） | 夹具 `sh -c 'sleep 0.5; stty size; sleep 1; stty size'`（前导 sleep 防 attach 前 drain，server.go:124）：Hello(111,44) → "44 111"，RESIZE(120,50) → "50 120" |
| TestHelloWelcome（02-02） | Welcome mode ro/rw 断言 + rw 下 INPUT echo（tracer 端到端） |
| TestPingKeepalive / TestPongTimeout / TestPingDisabled（02-04） | Options{PingInterval/PongTimeout} 注入：空闲 >3 间隔存活 / 客户端停 Read 被 CloseNow（库只在读路径回 pong，read.go:317-323）/ 0 禁用反证存活 |

**测试纪律：** stdlib `testing` + `-race`，零新增测试依赖；quick `go test ./... -count=1`；full `go test -race -count=1 ./... && pnpm -C web build`。

---

## Shared Patterns

以下为跨文件横切纪律，planner 应写进每个相关 PLAN.md 的 action 段：

### 协议单一事实源 + 前端手工对齐
**Source:** proto.go:1-11 包 doc · **Apply to:** `internal/proto/proto.go`、`web/src/main.ts`、`internal/server/server.go`
帧类型/版本/错误码/关闭码全部落 proto 包；前端常量手工对齐并在两侧注释互相指路（现状 proto.go:4 ↔ main.ts:6 模式）。Subprotocol 字符串 `"wesh.v1"` = Hello.version 期望值，同一常量（RESEARCH A2）。

### 关闭码纪律（D-05 全集，覆盖 Phase 1 版）
**Source:** CONTEXT D-05/D-06/D-08 · **Apply to:** `internal/proto/proto.go`、`internal/server/server.go`、`web/src/main.ts`
主动发送仅 {1000,1002,1008,1009,1011}；1001/1013 占位不实现；1005/1006/1015 永不发送（库 validWireCloseCode 线上强制）；**禁止发明新码或自定义 4000 段**。Error 帧按受众分治：正常客户端路径（version_mismatch/server_error）发 Error+关闭码；攻击面路径（unknown_frame/抢跑/超限/hello_timeout）只发关闭码无 Error 帧（D-06，不给攻击者反馈面）。

### exitf 注入 + sync.Once 收口
**Source:** server.go:37,162-173 · **Apply to:** `internal/server/server.go` 全部新终结路径
5s 未认证超时、pong 超时等一切新终结**不得新辟 exitf 分支**——全部经"关 conn → reader 终结 → wsDisconnected → terminate"既有单一路径收口（CONTEXT line 92 硬约束）。

### 原子态惯例
**Source:** server.go:28-36 · **Apply to:** `internal/server/server.go`
ro 标志、握手完成标志沿用 `atomic.Bool`；连接持有沿用 `atomic.Pointer[websocket.Conn]`（CONTEXT line 94）。

### 错误处理（Go 惯用法，无集中错误包）
**Source:** server.go:96-101, e2e_test.go:267-278 · **Apply to:** 全部 Go 文件
`errors.As(err, &websocket.CloseError)` 区分对端关闭；`errors.Is(err, websocket.ErrMessageTooBig)` 归一化 1009 事件（RESEARCH §Code Examples 423-436 行）；JSON 解码失败静默丢弃不关连接（现状 server.go:109-110 模式，Hello 之外沿用）；需要对端看到的关闭一律 graceful `c.Close(code, reason)`，`defer c.CloseNow()` 只做兜底（CloseNow 立即关 TCP 不发关闭帧，会丢未刷出的 Error 帧）。

### ctx 纪律
**Source:** server.go:90-91 + RESEARCH Pitfall 2（339-343 行） · **Apply to:** `internal/server/server.go`
ctx 一律 `context.Background()` 派生，**禁止 r.Context()**（hijack 后行为意外）；读路径只传无 deadline 的可取消 ctx（ctx 到期 = 库内 AfterFunc 整连接关闭，不是"本次读返回"，conn.go:188-199）；ping 的 10s ctx 独立短命；测试侧同样禁止给客户端 Read 传 deadline ctx（静默窗口用 goroutine + select 竞速，02-03）。

### per-IP 计数不变量
**Source:** RESEARCH Pitfall 4（351-356 行）+ Pitfall 6（363-366 行） · **Apply to:** `internal/server/server.go`
acquire 成功后 release 恰好一次，发生在 Hello 完成或连接/拒绝终结（先到为准）；实现用 `sync.Once` 或"defer 释放 + Hello 完成时显式释放置空"二选一；测试须覆盖全部退出路径。反代部署按代理 IP 聚合是已知限制（Pitfall 6，文档明示即可，**本 phase 不解析 X-Forwarded-For**）。

### 注释风格（决策追溯）
**Source:** 现状全部文件 · **Apply to:** 全部新代码
注释引用决策号（D-xx）/Pitfall 号/库源码行号（如 `read.go:107`）；包 doc 与结构体字段注释说"为什么"而非"是什么"；关闭路径注释标清触发源与收口关系（server.go:32-36 的 childExited 注释为范本）。

### 前端文案纪律
**Source:** CONTEXT specifics line 111 + main.ts:138-169 · **Apply to:** `web/src/main.ts`
全英文；onclose 按码分派文案，Error 帧 message 直接展示（D-07）；1009 文案不提 flag（本 phase 无可调 flag，D-12①）；1006 永不作为分派依据，异常断开用 `!ev.wasClean`/default 分支。

## No Analog Found

以下逻辑在库内与 RESEARCH.md 中均**无现成代码**，planner 需从 CONTEXT.md 决策 + RESEARCH 行为描述直接翻译（不得臆造"既有模式"）：

| 逻辑 | 归属文件 | 缺口说明 | 组装来源 |
|------|----------|----------|----------|
| per-IP 半开计数器（map+Mutex、acquire/release、清理时机） | `internal/server/server.go` | 库内无；RESEARCH 只定不变量与顺序（Pattern 1 注释 + Pitfall 4），数据结构明示为 discretion | CONTEXT D-04 + RESEARCH Pattern 1 注释 + Pitfall 4 不变量 |
| `clientIP`（RemoteAddr 取 IP 部分） | `internal/server/server.go` | 无 | `net.SplitHostPort(r.RemoteAddr)`（RESEARCH Pitfall 6）；反代聚合限制按 Pitfall 6 文档化 |
| stderr 单行事件具体格式 | `internal/server/server.go` | 现状服务端除启动行外零输出，无格式可沿 | CONTEXT D-12②（对端/码值/reason 三要素）+ discretion（Phase 8 才升级 slog） |
| `dialHello` 测试 helper（参数化 cols/rows） | `internal/server/e2e_test.go` | 新写 | RESEARCH §Wave 0 Gaps（566-569 行）职责描述 + startTestServer 现状装配模式 |
| 前端各码值英文文案细节 | `web/src/main.ts` | RESEARCH §Code Examples 前端段（438-475 行）给骨架与必备要点，措辞为 discretion | 现状 showStatus 三态面板文案风格（main.ts:141-168） |

## Planner 注意事项（跨文档冲突与覆盖）

1. **ARCHITECTURE.md §2.8"控制面 text JSON"方案已作废**（D-01）——一切帧编码以统一 binary 1 字节类型分派为准，勿引用旧案。
2. **ROADMAP 成功准则 2 关闭码集合漏写 1002**——以 D-05 并集 `{1000,1001,1002,1008,1009,1011,1013}` 为准（现状 server.go:114 已在用 1002）。
3. **~~`c.Read` 替换为 `readCounted`~~——D-09 2026-08-15 修订作废：** 现状 server.go:95 读循环保持不动；分片数层无库 API（read.go:457-479 空帧内部吞掉），由两层字节硬顶 + 预认证三道闸 + 409 门等效覆盖；本 phase 无破坏性读循环改造点，既有 e2e 全部经 `dialHello` 收口回归。
4. **守卫链顺序敏感**（planner 裁决，覆盖 RESEARCH Pattern 1 代码块的 409→per-IP 顺序）：子协议 400 → per-IP 429 → 409 原子门 → Accept → assert → SetReadLimit(4KiB) → 5s 计时器；409 在前则 429 在单客户端模型下结构性不可达（D-04 可触达性）；被 409 拒的第二客户端不得残留半开计数。
5. **升档序列顺序敏感**（planner 裁决形态，落 02-02/02-03 action）：close(helloDone) 停计时器 → Resize → per-IP release → Welcome → SetReadLimit(16KiB) → 启动 pinger（02-04）。
6. **Hello 语义改动影响面：** Hello 携带 cols/rows 后，前端 onopen 不再单独发首帧 RESIZE（被 Hello 取代）；服务端握手完成即 `sess.Resize`，消除 Phase 1 的 80x24 首帧窗口（RESEARCH 数据流主线 162 行）。**前端侧配套硬约束：helloSent 门住 sendResize**（term.onResize 常驻接线在首次 fit 必触发，不门则 RESIZE 抢跑首帧被 1002 frame_before_hello 直关——见 §main.ts 段）。
7. **D-12③ 落点修正**（RESEARCH §Code Examples 423-436 行）：库自动 1009 的 close reason 不可定制（固定 "read limited at N bytes"），`message_too_big` 机器串落在 stderr 事件而非线上 reason——planner/discuss 知悉即可，禁止包装库。
8. **测试可注入性**（RESEARCH §测试可注射性注意 559 行）：超时与上限经 `server.Options` 字段注入（`PingInterval` 生产直传（0=禁用，D-16）；`HelloTimeout`/`MaxHalfOpenPerIP`/`PongTimeout` 零值取默认常量：HelloTimeout 02-02 / MaxHalfOpenPerIP 02-03 / PingInterval+PongTimeout 02-04）——planner 已裁决为 Options 注入形态（沿用 exitf 注入模式，server.go:44 先例），替代包级私有变量测缝（e2e 为 `package server_test` 外包且包级变量改写有 -race 并行风险）；不违反 D-10/D-16"常量不开 CLI flag"（可配性指用户面）。
9. **ttyd 源码仅作缺陷对照**——可在注释/测试名中引证行号（protocol.c:288-298 作为三层上限的反面教材），禁止参考其实现方式。

## Metadata

**Analog search scope:** 仓库全量（cmd/ internal/ web/ 全部 .go/.ts 源文件逐一读取）；`02-RESEARCH.md` / `02-CONTEXT.md` 全量读取；Phase 1 的 01-PATTERNS.md 格式参照
**Files scanned:** 库内源文件 6（proto.go/server.go/main.go/main.ts/e2e_test.go/main_test.go/io.go）+ 阶段文档 3 份
**Pattern extraction date:** 2026-08-15
**模式置信度:** 库内骨架/纪律/测试基建为现状代码直引（HIGH）；RESEARCH Pattern 1-3 全部经 coder/websocket v1.8.15 模块缓存源码逐行核实（HIGH）；per-IP 计数器/stderr 格式/文案措辞为决策直译+discretion（缺口已显式标注）
