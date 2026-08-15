# Phase 2: 协议基线 - Pattern Map

**Mapped:** 2026-08-15
**Files analyzed:** 11（6 个修改既有文件 + 5 个新建测试文件）
**Analogs found:** 11 / 11（10 个有库内 analog——其中 6 个为被改文件自身；全部新逻辑另以 02-RESEARCH.md 的 VERIFIED 模式为源）

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
| `02-RESEARCH.md` Pattern 1-5 | 全部经 coder/websocket v1.8.15 源码逐行核实 | **新逻辑（守卫链/握手状态机/分片计数/ping/前端握手）的唯一模式源**——库内无对应现状代码 |
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
| `02-RESEARCH.md` | Pattern 1 守卫链（176-224）；Pattern 2 握手状态机（226-269）；Pattern 3 分片计数读循环（271-317）；Pattern 4 pinger（319-349）；Pattern 5 前端握手/ro/onclose（351-404）；Anti-Patterns（406-415）；proto 骨架（485-514）；裸帧 helper（516-557）；验证用例矩阵（559-573）；Pitfall 1-8（431-481）；测试映射（647-668） |
| `02-CONTEXT.md` | D-01~D-16 决策（17-44）；Integration Points（96-102） |

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/proto/proto.go`（扩展） | utility（协议单一事实源） | transform | **自身现状**（proto.go:1-48）+ RESEARCH proto 骨架（485-514） | exact（自扩展）+ external-doc |
| `internal/server/server.go`（重构 Attach） | controller（HTTP/WS 网关） | request-response + streaming | **自身现状**（server.go:72-173）+ RESEARCH Pattern 1-4（176-349） | exact（骨架沿用）+ external-doc（新逻辑） |
| `cmd/wesh/main.go`（扩展） | controller（入口/装配/生命周期） | event-driven | **自身现状**（main.go:22-84） | exact（自扩展） |
| `web/src/main.ts`（扩展） | component | streaming（WS 客户端） | **自身现状**（main.ts:6-169）+ RESEARCH Pattern 5（351-404） | exact + external-doc |
| `internal/proto/proto_test.go`（新建） | test | — | `cmd/wesh/main_test.go` 表驱动（15-44）+ e2e 注释惯例 | role-match |
| `internal/server/handshake_test.go`（新建） | test | — | `internal/server/e2e_test.go`（同包，helper 直接复用） | exact（同包） |
| `internal/server/limits_test.go`（新建） | test | — | e2e_test.go + RESEARCH 裸帧 helper（516-557） | exact + external-doc |
| `internal/server/keepalive_test.go`（新建） | test | — | e2e_test.go + RESEARCH 裸帧 helper（516-557） | exact + external-doc |
| `internal/server/rawws_test.go`（新建 helper） | test（helper） | — | **库内无 analog**；RESEARCH §Code Examples（516-557，帧格式经 frame.go:52-102 核实） | external-doc |
| `internal/server/e2e_test.go`（改造） | test | — | **自身** + 新写 `dialAndHandshake` helper（RESEARCH 测试映射 line 668） | exact（自改造） |
| `cmd/wesh/main_test.go`（扩展） | test | — | **自身**表驱动（15-44） | exact（自扩展） |

## Pattern Assignments

### `internal/proto/proto.go`（utility, transform）——扩展

**Analog:** 自身现状 + `02-RESEARCH.md` proto 骨架（lines 485-514）

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

**新增常量/错误码表（RESEARCH lines 489-511，照抄为底）：**
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

**关闭码表纪律（D-05/D-08）：** 全集 `{1000,1001,1002,1008,1009,1011,1013}` 进常量/注释表；**1001/1013 占位不实现**（注释标启用 phase）；发送侧值直接复用 `websocket.StatusXxx` 常量，本表用于前端对齐文档与断言测试（RESEARCH lines 510-511）。上限常量（16KiB/32/4KiB/5s/10s）按 D-10 进 proto 或 server 包级常量，注释标定来源与依据，**不开 CLI flag**。

---

### `internal/server/server.go`（controller, request-response + streaming）——重构 Attach

**Analog:** 自身现状（守卫门/读循环/收口骨架）+ `02-RESEARCH.md` Pattern 1-4（新逻辑逐段照抄）

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

**守卫区现状（lines 72-85）→ 扩展为三道预检（RESEARCH Pattern 1，lines 184-218，照抄为骨架）：**
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
	// ...失败路径全部 release(ip)——不变量：acquire 成功 → release 恰好一次（RESEARCH line 222）
	if c.Subprotocol() != proto.Subprotocol { // D-03 assert 兜底（理论不可达）
		c.CloseNow(); /* +释放门位与计数 */ return
	}
}
```

**`headerHasToken` 硬纪律（RESEARCH line 224）：** `Sec-WebSocket-Protocol` 是逗号分隔 token 列表，**必须按 token 拆分精确比较**，禁止 `strings.Contains`（防 `wesh.v1.evil` 前缀绕过）。

**握手状态机（RESEARCH Pattern 2，lines 230-266，照抄；两档 SetReadLimit + 5s 计时器 + 抢跑 1002 + version_mismatch 1008）：**
```go
const (
	readLimitPreAuth  = 4 * 1024      // D-11：Hello JSON ~100B，余量两个数量级
	readLimitPostAuth = 16 * 1024     // D-09/D-11
	helloTimeout      = 5 * time.Second // D-04
	maxFragments      = 32            // D-09
)
// c.SetReadLimit(readLimitPreAuth) → time.AfterFunc(helloTimeout, Close(1008,"hello_timeout"))
// → readCounted 等首消息 → 非 'H' 即 Close(1002,"unexpected_frame") 无 Error 帧（D-04/D-06）
// → proto.DecodeHello（未知字段忽略）→ version≠wesh.v1 则 Write(Error)+Close(1008)
```

**Welcome 后升档序列（RESEARCH line 269，顺序敏感）：** `sess.Resize(h.Cols, h.Rows)`（Hello 携首尺寸，消除 80x24 首帧窗口）→ `writeWelcome(mode)` → `SetReadLimit(16KiB)` → 停 hello 计时器 → per-IP release → `go s.pinger(...)`。`SetReadLimit` 内部 atomic，读循环进行中调档安全。

**分片计数读循环（RESEARCH Pattern 3，lines 277-314，照抄——本 phase 唯一非常规实现点；现状 line 95 的 `c.Read` 必须整体替换为 `c.Reader` 手动循环，`io.ReadAll` 吞分片边界不可用）：**
```go
func readCounted(ctx context.Context, c *websocket.Conn, buf []byte, maxFragments int) (websocket.MessageType, []byte, error) {
	rctx, rcancel := context.WithCancel(ctx)
	defer rcancel()
	typ, r, err := c.Reader(rctx)
	if err != nil { return 0, nil, err }
	timer := time.AfterFunc(msgCompleteTimeout, rcancel) // 首帧到达后才武装完成时限
	defer timer.Stop()
	msg := buf[:0]
	fragments := 0
	for {
		n, rerr := r.Read(buf[len(msg):cap(msg)]) // 追加读进同一缓冲，零额外分配
		if n > 0 {
			msg = msg[:len(msg)+n]
			fragments++
			if fragments > maxFragments {
				return 0, nil, errFragmentLimit // 调用方 Close(1009, "fragment_limit")
			}
		}
		if errors.Is(rerr, io.EOF) { return typ, msg, nil }
		if rerr != nil { return 0, nil, rerr } // 含 ErrMessageTooBig（库已发 1009）
		// ...
	}
}
```
**buf 必须 ≥ 当前 SetReadLimit**（否则单帧被缓冲截断成伪分片误杀合法消息——RESEARCH Pitfall 1，lines 433-438）。

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

**ping goroutine（RESEARCH Pattern 4，lines 325-346，照抄；Welcome 升档后启动，ctx 随 Attach defer cancel——禁止 r.Context()，现状 line 90 注释同源）：**
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

**stderr 单行事件（D-12②，格式属 discretion）：** 对端、码值、reason 单行；`errors.Is(err, websocket.ErrMessageTooBig)` 归一化为 `message_too_big` 机器串（RESEARCH Pitfall 4，lines 453-457——库自动 1009 的 reason 不可定制，机器串落点在 stderr 而非线上 reason）。

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

**http.Serve 改造点（现状 line 79 → http.Server + ReadHeaderTimeout，RESEARCH Pitfall 8 lines 477-481——http.Serve 零超时是预认证 slowloris 面，新发现）：**
```go
// 现状：if err := http.Serve(ln, srv.Handler()); err != nil {
// 改为（ReadTimeout/WriteTimeout 不要设——误伤长连接语义边界）：
hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
if err := hs.Serve(ln); err != nil {
```

**config → server 装配传递（现状 line 76 `server.New(sess, os.Exit)`）：** writable/pingInterval 经 New 参数或 Server 字段注入（planner 定签名）；启动单行打印（D-07 现状 line 78）不变。

---

### `web/src/main.ts`（component, streaming）——扩展

**Analog:** 自身现状 + `02-RESEARCH.md` Pattern 5（lines 353-404）

**帧常量对齐（现状 lines 6-9，新增 HELLO/WELCOME/ERROR 入同一块并保持对齐注释）：**
```typescript
// 帧常量与 internal/proto/proto.go 手工对齐（D-16）：'0' INPUT / '1' RESIZE / '0' OUTPUT
const OUTPUT = 0x30,
  INPUT = 0x30,
  RESIZE = 0x31;
// 新增：const HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45; // 'H' 'W' 'E'
```

**WS 构造加子协议（现状 line 65 → RESEARCH line 355）：**
```typescript
const ws = new WebSocket('ws://' + location.host + '/ws', ['wesh.v1']); // D-03
ws.binaryType = 'arraybuffer';
```

**onopen 发 Hello 首帧（现状 lines 111-116 扩展；Hello 必须是首帧且携带首尺寸——RESEARCH lines 361-368；原 `sendResize` 首发被 Hello 的 cols/rows 取代）：**
```typescript
ws.onopen = () => {
  opened = true;
  fit.fit();
  ws.send(concat(new Uint8Array([HELLO]),
    enc.encode(JSON.stringify({ version: 'wesh.v1', cols: term.cols, rows: term.rows }))));
  term.focus();
};
```

**onmessage 扩展为 switch 分派（现状 lines 79-84 单 if → RESEARCH lines 370-388；复用 concat/TextDecoder 现状件）：**
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

**onData 发送门（现状 lines 89-93 的 readyState 门保留，叠加 `mode === 'rw'` 第二道闸——RESEARCH line 404；真防线在服务端丢 INPUT）：**
```typescript
term.onData((s) => {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(concat(new Uint8Array([INPUT]), enc.encode(s)));
  }
});
```

**onclose 按码分派（现状 lines 149-169 的三分法扩展为 per-code switch——RESEARCH lines 390-401；文案沿用 showStatus 三态面板风格与全英文纪律，具体文案属 discretion）：**
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

**Analog:** `internal/server/e2e_test.go`（同包 helper 直接复用）+ `cmd/wesh/main_test.go`（表驱动风格）+ RESEARCH 裸帧 helper（516-557）

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

**改造点（RESEARCH line 668，重要）：** Phase 1 全部 e2e（TestEchoPTY/TestSecondClient409/TestExitCodePropagation/TestUnknownFrame1002/TestClientDisconnectSIGHUP）Dial 后直接发 INPUT——握手上线后全部需先过 Hello。做 `dialAndHandshake(t, addr)` helper 统一收口（Dial options 带 `Subprotocols: []string{"wesh.v1"}` → 发 Hello → 等 Welcome），避免逐测试散落握手代码。注意 `server.New` 签名若变（writable 注入），startTestServer 同步改。

**裸帧 helper（`rawws_test.go`，RESEARCH lines 519-556 照抄——库客户端发不出畸形帧，攻击面用例必须裸写）：**
```go
func dialRawWS(t *testing.T, addr string, extraHeaders ...string) net.Conn {
	// 裸 TCP + 手写 Upgrade 请求；extraHeaders 例: "Sec-WebSocket-Protocol: wesh.v1"
	// 读响应头至 \r\n\r\n；断言首行（101 或 400/429——负例测试的断言点）
}
func writeRawFrame(t *testing.T, conn net.Conn, fin bool, opcode byte, payload []byte) {
	// mask 强制（服务端对未 mask 帧直接报错）；payload 逐字节 XOR maskKey
}
```

**proto_test.go（表驱动风格仿 main_test.go:15-44）：** Hello/Welcome/Error 编解码往返；Hello 混未知字段解析成功（D-02）；关闭码常量表 ⊆ D-05 集合、无 1005/1006/1015/4000 段字面量（静态断言，RESEARCH line 664）。

**关键攻击面用例矩阵（RESEARCH lines 559-573 逐条落成测试；测试名与命令已定于 lines 649-665）：**

| 用例 | 断言要点 |
|------|----------|
| TestMessageTooBig | 库客户端发 17KiB 单帧 → CloseError.Code==1009 |
| TestFragmentByteFlood | 裸帧百万 1B continuation → ≤16385 帧内收 1009；随后新连接握手成功（存活代理断言） |
| TestFragmentCountLimit | 裸帧 33 个 1B 分片（33B<16KiB）→ 1009 + reason `fragment_limit`（层2 独立于层3 的最小反例） |
| TestEmptyFrame | 空 binary 消息 → 连接保持，后续 Hello 正常完成 |
| TestSubprotocolRequired | 裸握手不带 Sec-WebSocket-Protocol → HTTP 400 未升级 |
| TestHalfOpenCap | 同 IP 8 连接升级不发 Hello，第 9 个 → HTTP 429 |
| TestHelloTimeout | 升级后静默 → 4-8s 容忍窗内收 1008（reason `hello_timeout`） |
| TestPrematureFrame | 握手后首帧直接 INPUT → 1002 且线上无 'E' Error 帧（D-06） |
| TestReadOnlyDropsInput | 默认握手后发 INPUT → 无回显且连接存活；Welcome.mode=="ro" |
| TestReadOnlyAllowsResize | ro 下发 RESIZE → spawn `stty size` 断言尺寸跟随（D-13） |
| TestWritableEcho | writable=true，INPUT 回显（TestEchoPTY 改造加握手） |
| TestPingKeepalive | interval=200ms，裸帧断言 1s 内 ≥2 个 ping（op=0x9），回 pong 后存活 |
| TestPongTimeout | 裸帧不回 pong → interval+timeout 内连接被关（Open Questions Q2：pongTimeout 用包级私有变量注入，生产恒 10s） |

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
5s 未认证超时、pong 超时、分片超限等一切新终结**不得新辟 exitf 分支**——全部经"关 conn → reader 终结 → wsDisconnected → terminate"既有单一路径收口（CONTEXT line 92 硬约束）。

### 原子态惯例
**Source:** server.go:28-36 · **Apply to:** `internal/server/server.go`
ro 标志、握手完成标志沿用 `atomic.Bool`；连接持有沿用 `atomic.Pointer[websocket.Conn]`（CONTEXT line 94）。

### 错误处理（Go 惯用法，无集中错误包）
**Source:** server.go:96-101, e2e_test.go:267-278 · **Apply to:** 全部 Go 文件
`errors.As(err, &websocket.CloseError)` 区分对端关闭；`errors.Is(err, websocket.ErrMessageTooBig)` 归一化 1009 事件（RESEARCH Pitfall 4）；JSON 解码失败静默丢弃不关连接（现状 server.go:109-110 模式，Hello 之外沿用）；需要对端看到的关闭一律 graceful `c.Close(code, reason)`，`defer c.CloseNow()` 只做兜底（RESEARCH Pitfall 5——CloseNow 会丢未刷出的 Error 帧）。

### ctx 纪律
**Source:** server.go:90-91 + RESEARCH Pitfall 2（440-445） · **Apply to:** `internal/server/server.go`
ctx 一律 `context.Background()` 派生，**禁止 r.Context()**（hijack 后行为意外）；读路径只传无 deadline 的可取消 ctx（ctx 到期 = 库内 AfterFunc 整连接关闭，不是"本次读返回"）；ping 的 10s ctx 独立短命；完成时限用"Reader 返回后 time.AfterFunc(rcancel)"形态。

### per-IP 计数不变量
**Source:** RESEARCH line 222 + Pitfall 6（465-469） · **Apply to:** `internal/server/server.go`
acquire 成功后 release 恰好一次，发生在 Hello 完成或连接/拒绝终结（先到为准）；实现用 `sync.Once` 或"defer 释放 + Hello 完成时显式释放置空"二选一；测试须覆盖全部退出路径。反代部署按代理 IP 聚合是已知限制（Pitfall 7，文档明示即可，**本 phase 不解析 X-Forwarded-For**）。

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
| per-IP 半开计数器（map+Mutex、acquire/release、清理时机） | `internal/server/server.go` | 库内无；RESEARCH 只定不变量与顺序（line 191-222），数据结构明示为 discretion | CONTEXT D-04 + RESEARCH Pattern 1 注释 + Pitfall 6 不变量 |
| `clientIP`（RemoteAddr 取 IP 部分） | `internal/server/server.go` | 无 | `net.SplitHostPort(r.RemoteAddr)`（RESEARCH line 191 注释）；反代聚合限制按 Pitfall 7 文档化 |
| stderr 单行事件具体格式 | `internal/server/server.go` | 现状服务端除启动行外零输出，无格式可沿 | CONTEXT D-12②（对端/码值/reason 三要素）+ discretion（Phase 8 才升级 slog） |
| `dialAndHandshake` 测试 helper | `internal/server/e2e_test.go` | 新写 | RESEARCH line 668 职责描述 + startTestServer 现状装配模式 |
| 前端各码值英文文案细节 | `web/src/main.ts` | RESEARCH Pattern 5 给骨架与必备要点，措辞为 discretion | 现状 showStatus 三态面板文案风格（main.ts:141-168） |
| `msgCompleteTimeout` 常量取值 | `internal/server/server.go` | RESEARCH A5 推荐 10s（与 pong 超时同值），Open Questions Q1/Q3 待 planner 裁决 | RESEARCH lines 604-619；若从简须在 PLAN 风险节明示接受空分片慢滴残余风险 |

## Planner 注意事项（跨文档冲突与覆盖）

1. **ARCHITECTURE.md §2.8"控制面 text JSON"方案已作废**（D-01）——一切帧编码以统一 binary 1 字节类型分派为准，勿引用旧案。
2. **ROADMAP 成功准则 2 关闭码集合漏写 1002**——以 D-05 并集 `{1000,1001,1002,1008,1009,1011,1013}` 为准（现状 server.go:114 已在用 1002）。
3. **`c.Read` 必须整体替换为 `readCounted`**（RESEARCH Pitfall 1）——现状 server.go:95 的读法吞分片边界，RES-01 层2 无法实现；这是本 phase 唯一的破坏性改造点，既有 e2e 全部经 `dialAndHandshake` 收口回归。
4. **守卫链顺序敏感**（RESEARCH line 222）：子协议 400 → per-IP 429 → 409 原子门 → Accept → assert → SetReadLimit(4KiB) → 5s 计时器；被 409 拒的第二客户端不得消耗半开名额。
5. **升档序列顺序敏感**（RESEARCH line 269）：Resize → Welcome → SetReadLimit(16KiB) → 停计时器 → per-IP release → 启动 pinger。
6. **Hello 语义改动影响面：** Hello 携带 cols/rows 后，前端 onopen 不再单独发首帧 RESIZE（被 Hello 取代）；服务端握手完成即 `sess.Resize`，消除 Phase 1 的 80x24 首帧窗口（RESEARCH 架构图 line 161）。
7. **D-12③ 落点修正**（RESEARCH Pitfall 4）：库自动 1009 的 close reason 不可定制（固定 "read limited at N bytes"），`message_too_big` 机器串落在 stderr 事件而非线上 reason——planner/discuss 知悉即可，禁止包装库。
8. **测试可注入性**（Open Questions Q2）：pongTimeout 用包级私有变量（非 export、非 flag，生产恒 10s）供测试改写，不违反 D-10/D-16"常量不开 CLI flag"（可配性指用户面）。
9. **ttyd 源码仅作缺陷对照**——可在注释/测试名中引证行号（protocol.c:288-298 作为三层上限的反面教材），禁止参考其实现方式。

## Metadata

**Analog search scope:** 仓库全量（cmd/ internal/ web/ 全部 .go/.ts 源文件逐一读取）；`02-RESEARCH.md` / `02-CONTEXT.md` 全量读取；Phase 1 的 01-PATTERNS.md 格式参照
**Files scanned:** 库内源文件 6（proto.go/server.go/main.go/main.ts/e2e_test.go/main_test.go/io.go）+ 阶段文档 3 份
**Pattern extraction date:** 2026-08-15
**模式置信度:** 库内骨架/纪律/测试基建为现状代码直引（HIGH）；RESEARCH Pattern 1-5 全部经 coder/websocket v1.8.15 模块缓存源码逐行核实（HIGH）；per-IP 计数器/stderr 格式/文案措辞为决策直译+discretion（缺口已显式标注）
