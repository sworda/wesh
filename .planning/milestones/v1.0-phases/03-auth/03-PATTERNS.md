# Phase 3: 认证与传输安全 - Pattern Map

**Mapped:** 2026-08-17
**Files analyzed:** 13（4 修改 + 5 新建源文件 + 4 新建/扩展测试文件）
**Analogs found:** 13 / 13（本 phase 为纯增量，全部文件均有同仓模拟；新建中间件类文件无现成中间件模拟，取最近邻模式）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/server/auth.go` 【新】 | middleware（Basic 认证+常数时间比较） | request-response | `server.go` 守卫区 + `halfOpenCounter` | partial（仓内无中间件先例，取守卫/http.Error 模式） |
| `internal/server/tickets.go` 【新】 | service（内存票表 CRUD） | CRUD（签发/核销） | `halfOpenCounter`（server.go:136-162） | exact（mutex+map+删 key 纪律同构） |
| `internal/server/throttle.go` 【新】 | service（per-IP 退避计数器） | CRUD（计数/过期） | `halfOpenCounter`（server.go:136-162） | exact |
| `internal/server/origin.go` 【新】 | utility + middleware（Origin 规范化/检查） | request-response | `headerHasToken`（server.go:179-188） | role-match（头解析精确比较纪律） |
| `internal/server/headers.go` 【新】 | middleware（安全响应头） | request-response | `Server.Handler()` 装配（server.go:118-130） | partial |
| `internal/server/server.go` 【改】 | controller（/ws handler + 握手段） | request-response / streaming | 自身现状（守卫区+升档序列） | exact（增量插入） |
| `internal/proto/proto.go` 【改】 | model（协议常量/载荷） | request-response | 自身现状（Error 表/HelloPayload） | exact（纯加字段/常量） |
| `cmd/wesh/main.go` 【改】 | config/entrypoint（flag+启动矩阵+TLS 分岔） | startup / request-response | 自身现状（parseArgs/run） | exact |
| `web/src/main.ts` 【改】 | component（前端连接流程） | event-driven | 自身现状（onopen/onclose 分派） | exact |
| `internal/server/auth_test.go` 【新】 | test（integration+unit） | request-response | `handshake_test.go` + `main_test.go:captureFd` | exact |
| `internal/server/throttle_test.go` 【新】 | test（integration） | request-response | `handshake_test.go`（Options 注入模式） | exact |
| `internal/server/origin_test.go` 【新】 | test（integration） | request-response | `handshake_test.go`（dialWantStatus 负例断言） | exact |
| `internal/server/tls_test.go` 【新】 | test（integration） | request-response（TLS 握手矩阵） | `e2e_test.go:startTestServerWith` | exact |
| `cmd/wesh/main_test.go` 【改】 | test（unit 表驱动） | startup | 自身现状（TestParseArgs/captureFd） | exact |
| `web/uat/phase03.mjs` 【新】 | test（协议 UAT） | event-driven | `web/uat/phase02.mjs` | exact |

## Pattern Assignments

### `internal/server/tickets.go`【新】（service, CRUD）

**Analog:** `internal/server/server.go` — `halfOpenCounter`（行 132-162）

**核心模式：mutex + map + 防单调增长纪律**（行 136-162）：
```go
type halfOpenCounter struct {
	mu sync.Mutex
	n  map[string]int
}

// acquire 在 ip 的半开计数未达 max 时 +1 并返回 true，否则返回 false（429 闸）。
func (h *halfOpenCounter) acquire(ip string, max int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n[ip] >= max {
		return false
	}
	h.n[ip]++
	return true
}

// release 将 ip 的半开计数 -1；到 0 删除 map key——防 map 随历史连接数单调增长
// （Pitfall 4 泄漏面）。
func (h *halfOpenCounter) release(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n[ip] <= 1 {
		delete(h.n, ip)
		return
	}
	h.n[ip]--
}
```

**复制要点：**
- 类型注释写清不变量（halfOpenCounter 行 132-135 注释风格：先写"是什么"，再写"不泄漏也不双重释放"的纪律）
- 每个方法 `mu.Lock(); defer mu.Unlock()` 成对
- **map 必须删 key 防单调增长**——ticketStore 对应物：核销即删（单次使用）、签发顺手机会性清扫过期项（RESEARCH Pattern 1 已定稿代码，直接照抄）
- 零值默认 + Options 覆写先例见下方 Shared Patterns「Options 注入」

---

### `internal/server/throttle.go`【新】（service, CRUD）

**Analog:** 同上 `halfOpenCounter`（server.go:136-162）+ `clientIP`（server.go:164-174）

**per-IP 键提取模式**（行 164-174，直接复用此函数，不重写）：
```go
// clientIP 取对端 IP 作 per-IP 计数键：net.SplitHostPort 取主机部分（含端口直接
// 当键会使每连接一个"新 IP"，上限形同虚设——Pitfall 6），失败回退 RemoteAddr 整串。
// 反代部署下同键聚合为代理 IP 是已知限制（Pitfall 6）；X-Forwarded-For 信任属
// Phase 3 SEC-07，本 phase 不解析。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

**复制要点：**
- throttleStore 结构（mu + map + base/cap 时长字段）直接采用 RESEARCH Pattern 3 定稿代码（1s×2 封顶 30s、15min 惰性过期、成功 delete 清零）
- `base`/`cap` 走 Options 可覆写（HelloTimeout 先例，server.go:91-93）
- **顺手校正注释**：行 166-167 与行 224-225 "Phase 3 SEC-07" 误写，本 phase 改为 "Phase 7 SEC-07"（CONTEXT deferred 节明示）

---

### `internal/server/auth.go`【新】（middleware, request-response）

**Analog:** `server.go` Attach 守卫区（行 200-223）+ `headerHasToken`（行 176-188）

**守卫拒绝模式：http.Error + 通用文案 + 提前 return**（行 201-205）：
```go
if !headerHasToken(r.Header, "Sec-WebSocket-Protocol", proto.Subprotocol) {
	http.Error(w, "subprotocol wesh.v1 required", http.StatusBadRequest)
	return
}
```

**头解析精确比较纪律**（行 176-188，Pitfall 5 硬纪律）：
```go
// headerHasToken 按 token 拆分比较逗号分隔头（Split "," + TrimSpace + EqualFold
// 逐 token），禁止 strings.Contains 整头匹配——防 wesh.v1.evil 前缀绕过。
func headerHasToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}
```

**复制要点：**
- Basic 解析用 `r.BasicAuth()`（stdlib），禁止手拆 base64（RESEARCH Don't Hand-Roll 表）
- 凭据比较代码直接采用 RESEARCH Pattern 2 定稿（SHA-256 等长化 + subtle + 逐组位与 `&` 累积不短路）
- 401 同文纪律：`WWW-Authenticate: Basic realm="wesh", charset="UTF-8"` + 通用 body，无/错凭据完全同文（无枚举 oracle）
- 认证失败/成功事件出口只走 `logEvent` 三要素（见 Shared Patterns），**凭据/Authorization 永不入参**

---

### `internal/server/origin.go`【新】（utility + middleware, request-response）

**Analog:** `headerHasToken`（server.go:176-188）+ Attach 守卫区拒绝形态

**复制要点：**
- `normalizeOrigin` 直接采用 RESEARCH Pattern 4 定稿代码（url.Parse + 拒绝 path/query/fragment/userinfo + 拒绝 glob 字符 `*?[\` + 小写 host + 剥离默认端口）
- `/api/attach` 侧检查语义与库对齐（RESEARCH 一手核实 accept.go:228-264）：空 Origin 放行 → `strings.EqualFold(r.Host, u.Host)` 同源放行 → 规范化集合精确查找 → 否则 403
- 拒绝形态沿用守卫区风格：`http.Error(w, "origin not allowed", http.StatusForbidden)` + 提前 return（文案为通用文案，不回显 Origin 值——防日志/响应泄露面）
- `/ws` 侧：规范化结果喂 `AcceptOptions.OriginPatterns`（挂点 server.go:229-231），另在守卫区 ⓪ 位加显式检查（Accept 前拒绝，HTTP 层可测）

---

### `internal/server/headers.go`【新】（middleware, request-response）

**Analog:** `Server.Handler()` mux 装配（server.go:117-130）

**装配模式**（行 117-130）：
```go
// Handler 挂两条路由：/ 走 go:embed 静态伺服，/ws 走 Attach。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	wh, err := web.Handler()
	if err != nil {
		wh = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded assets unavailable", http.StatusInternalServerError)
		})
	}
	mux.Handle("/", wh)
	mux.HandleFunc("/ws", s.Attach)
	return mux
}
```

**复制要点：**
- 安全头中间件代码直接采用 RESEARCH Pattern 6 定稿（`securityHeaders(next http.Handler, tlsOn bool) http.Handler`，CSP/XFO/nosniff/Referrer-Policy/COOP/CORP 恒在，HSTS 仅 `tlsOn` 分支）
- 中间件包装在 `Handler()` 装配层完成（mux 外层包 securityHeaders；`/` 与 `/api/attach` 再包 basicAuth；`/ws` 不挂 Basic——ticket 即其认证）
- 本仓此前无 `func(http.Handler) http.Handler` 中间件先例——此文件即先例，保持最小形态，不引入框架
- `/api/attach` 路由用 ServeMux 方法模式 `mux.HandleFunc("POST /api/attach", h)` 白拿 405 + Allow 头（Go 1.22+，RESEARCH 已核实 GOROOT server.go:2699-2710）

---

### `internal/server/server.go`【改】（controller, request-response/streaming）

**Analog:** 自身现状——三处增量插入点

**插入点 1：守卫区加 ⓪ Origin（行 200-223 之前）**——顺序敏感注释块（行 192-199）同步更新：
```go
// 守卫区（Accept 前，HTTP 层零 WS 资源分配，顺序敏感）：
//	① D-03 子协议预检 400（最廉价无状态，扫描器/旧客户端最早被拦）；
//	② D-04 per-IP 半开上限 429（默认 8）……
//	③ D-09 409 单客户端原子门（Phase 5 才改）。
```

**插入点 2：AcceptOptions 加 OriginPatterns**（行 227-231）：
```go
// AcceptOptions：Subprotocols 一行开启协商回显（D-03）；压缩默认禁用（终端高熵
// 数据无收益，D-17）；不跳过 Origin 校验——库默认同源校验（同 Host 放行、跨源拒绝）。
c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
	Subprotocols: []string{proto.Subprotocol},
})
```

**插入点 3：握手段加 ticket 核销**（version 检查之后、升档序列之前，行 298-304 之间）——照抄 version_mismatch 既有桶形态（行 298-303）：
```go
} else if h.Version != proto.Subprotocol {
	// D-06 正常客户端路径：先 Error 帧后 1008；close reason 与 code 同名机器串（D-07）。
	_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrVersionMismatch, "protocol version wesh.v1 required"))
	s.logEvent(remote, websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
	_ = c.Close(websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
}
```
新增 `else if` 分支形态与此完全一致：`ErrorFrame(proto.ErrAuthFailed, ...)` + `logEvent` + `Close(1008, proto.ErrAuthFailed)`（D-10 统一口径；RESEARCH Open Question 2 建议 version 先查、核销紧随其后）。

**Options 新字段挂点**（行 63-69）——ticket TTL/throttle base/credentials/origins/tlsOn 按同一注释纪律添加（生产直传 vs 测试覆写分组说明）。

---

### `internal/proto/proto.go`【改】（model, request-response）

**Analog:** 自身现状——两处纯增量

**Error code 表**（行 39-46）——注释块已预告本 phase 挂账，照格式加行：
```go
// Error codes（D-06 受众分治……）。code 为 snake_case 机器串，主动关闭的 close
// reason 带同名机器串（RFC6455 ≤123 字节，D-07）。auth_failed/permission_denied
// 属 Phase 3/5（deferred）。
const (
	ErrVersionMismatch = "version_mismatch" // 正常客户端可见，发 Error 帧 + 1008
	ErrServerError     = "server_error"     // 发 Error 帧 + 1011
)
```
增量：`ErrAuthFailed = "auth_failed"` + 行尾注释（核销失败统一口径，发 Error 帧 + 1008）；同时把注释块里 "auth_failed…属 Phase 3/5（deferred）" 改为已兑现表述（permission_denied 仍 Phase 5）。

**HelloPayload 加字段**（行 68-75）——显式 json tag 纪律 + 未知字段忽略注释已预告 ticket：
```go
// HelloPayload 显式 json tag，防字段名漂移。
// 未知字段由 json.Unmarshal 默认忽略——D-02 演化纪律的零成本实现
// （禁止 DisallowUnknownFields；Phase 3 加 ticket、Phase 5 加 attach/mode 只是加字段）。
type HelloPayload struct {
	Version string `json:"version"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
}
```
增量：`Ticket string \`json:"ticket,omitempty"\``（omitempty：无认证模式前端省略字段，JSON 不出 ticket 键）。

**前端对齐纪律**：proto.go 行 6 注释「前端 web/src/main.ts 的帧常量与本文件手工对齐，两侧注释互相指路（D-16）」——改 proto.go 必须同步 main.ts 常量区。

---

### `cmd/wesh/main.go`【改】（config/entrypoint, startup）

**Analog:** 自身现状

**flag 注册模式**（行 36-46，全名无短选项、连续 *Var 调用、Usage 覆写）：
```go
fs := flag.NewFlagSet("wesh", flag.ContinueOnError)
fs.IntVar(&cfg.port, "port", 7681, "listen port (0 = random, actual port is printed)")
fs.StringVar(&cfg.bind, "bind", "0.0.0.0", "listen address")
fs.BoolVar(&cfg.showVersion, "version", false, "print version and exit")
fs.BoolVar(&cfg.writable, "writable", false, "allow client input (default read-only)")
fs.DurationVar(&cfg.pingInterval, "ping-interval", 5*time.Second, "WS ping interval (0 = disable)")
```
- 可重复 flag（`--credential`/`--origin`）用 `fs.Func(name, usage, func(s string) error { ... })` 收集——解析校验（parseCredential/normalizeOrigin）直接挂在 Func 回调里，错误提前到 parse 期
- `--credential` help 文案须注明"flag 值对同机用户可见（ps），生产建议 WESH_CREDENTIAL env"（Pitfall 8）

**错误出口模式**（行 60-68，exit code 2 = 用法/校验错误）：
```go
cfg, argv, err := parseArgs(args)
if err != nil {
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintf(os.Stderr, "wesh: %v; usage: wesh [flags] -- <cmd> [args...]\n", err)
	return 2
}
```
启动校验矩阵（D-03/D-05 拒绝路径、D-04 cert/key 成对）走同一出口：纯函数（RESEARCH Pattern 7 定稿 8 行矩阵）返回 error，run 里 `Fprintf(os.Stderr, "wesh: %v\n", err)` + 非零返回。

**http.Server 分岔挂点**（行 86-96）：
```go
// 显式 http.Server：ReadHeaderTimeout=5s 盒住预认证 HTTP 层慢 loris……
hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
if err := hs.Serve(ln); err != …
```
增量：`if cfg.tlsCert != "" { hs.TLSConfig = tlsConfig(); err = hs.ServeTLS(ln, cert, key) } else { err = hs.Serve(ln) }`（tlsConfig 用 RESEARCH Pattern 5 定稿：MinVersion 1.2 + 显式 6 AEAD 清单）；启动行打印（行 88）分支感知 scheme（http/https）+ 明文警告 stderr 打印。

---

### `web/src/main.ts`【改】（component, event-driven）

**Analog:** 自身现状

**连接建立现状**（行 73-74）+ **Hello 发送现状**（行 150-161）：
```ts
const ws = new WebSocket('ws://' + location.host + '/ws', [SUBPROTOCOL]); // D-03：wesh.v1 子协议建连
ws.binaryType = 'arraybuffer';
...
ws.onopen = () => {
  opened = true;
  fit.fit();
  ws.send(concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols: term.cols, rows: term.rows }))));
  helloSent = true;
  term.focus();
};
```
改造为 `connect()` 函数包裹（RESEARCH Code Examples 定稿）：先 `fetch('/api/attach', {method:'POST'})` 按状态分派（200 取 ticket / 404 无认证直连 / 401·429 showStatus），WS URL scheme 按 `location.protocol` 选 ws/wss，Hello JSON 加 `ticket` 字段（undefined 时 JSON 自动省略）。

**onclose 按码分派现状**（行 197-249，switch(ev.code) + lastError?.message 优先）：
```ts
case 1008: // 策略违反（version_mismatch 等）——Error 帧 message 优先展示（D-07）
  showStatus('Connection refused', lastError?.message ?? 'The server refused this connection.', ...);
```
增量：switch 前加 auth_failed 静默重试守卫（`lastError?.code === 'auth_failed' && !retriedAuth` → 置位重试一次 `void connect()`，非无限循环）；1008 分支文案兼容 auth_failed（重试一次失败后才落此分支展示）。

**纪律沿用：** 帧常量与 proto.go 手工对齐注释（行 6-8）；showStatus 三态面板复用（行 165-181），零新 UI 组件（D-02）。

---

### 测试文件组（auth_test / throttle_test / origin_test / tls_test）【新】

**Analog:** `handshake_test.go`（守卫负例断言）+ `e2e_test.go`（装配收口）+ `main_test.go`（captureFd）

**装配复用**（e2e_test.go:105-121，同包直接复用，注释行 18-21 明示纪律）：
```go
// 测试装配统一经 e2e_test.go 的 startTestServerWith/startTestServer/dialHello/waitExit
// 收口（同包直接复用）；超时护栏一律 10s ctx。
```

**Options 注入提速模式**（handshake_test.go:140-141，HelloTimeout=200ms 先例）：
```go
exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true, HelloTimeout: 200 * time.Millisecond})
```
throttle/ticket TTL 同此注入（ms 级 base/TTL 覆写）。

**HTTP 负例断言模式**（handshake_test.go:104-116，dialWantStatus 三段式：dial 失败 + resp 非 nil + 状态码相等）——401/403/405/429 断言直接复用此形；`/api/attach` 非 WS 端点用 `http.Post`/`http.DefaultClient.Do` 断言。

**关闭码+reason 双断言**（handshake_test.go:163-168）：`ce.Code == 1008` 且 `ce.Reason == proto.ErrAuthFailed`（D-07/D-10 机器串同名纪律）。

**stderr 捕获**（main_test.go:62-79 captureFd：os.Pipe 置换 *os.File，进程全局不并行）——日志脱敏测试（SEC-01 红线）直接复用此 helper 形态到 server 包测试，断言输出不含 base64 凭据/明文 pass/ticket 值/"authorization"（大小写不敏感）。

**TLS 测试**：无仓内先例，按 RESEARCH Code Examples 定稿（测试内 crypto/x509+ecdsa 自签 localhost 证书 ~40 行 helper，tls.Dial 矩阵断言 1.1 必败/1.2 必成/1.3 默认/CBC-only 必败），装配层仿 startTestServerWith 但 `tls.NewListener` 替换 `net.Listen` + `http.Serve`。

---

### `cmd/wesh/main_test.go`【改】（test, startup）

**Analog:** 自身现状

**表驱动断言模式**（行 18-58）——新 flag 默认值/显式传值直接加表行；启动矩阵 8 行用独立表驱动测试（wantErr + 错误文案子串断言）。

**run 级捕获模式**（行 84-94，captureFd + run(args) 返回码 + stderr 子串断言）：
```go
code, out := captureFd(t, &os.Stderr, func() int { return run(args) })
if code == 0 { t.Errorf(...) }
if !strings.Contains(out, "usage: wesh [flags] -- <cmd> [args...]") { ... }
```
D-03/D-05 拒绝路径文案断言同此（注意：校验必须先于 pty.Start/net.Listen，否则测试挂死——行 81-83 注释已锁此纪律）。

---

### `web/uat/phase03.mjs`【新】（test, event-driven）

**Analog:** `web/uat/phase02.mjs`（exact match——整文件即模板）

**复制要点（逐段对应）：**
- 零依赖 Node 原生 WebSocket/fetch（行 1-2 头注释格式：phase 名 + 运行命令）
- `startWesh` spawn + stdout 正则解析端口（行 32-47）——原样复用；认证场景 args 加 `--credential`（注意凭据不出现在 check 输出里，日志红线延伸到 UAT 输出）
- `dialHello` 握手 Promise 封装（行 52-66）——扩展支持 ticket 参数
- `check(id, name, ok, detail)` PASS/FAIL 累积 + 末尾汇总 exit code（行 26-29、230-232）——原样复用
- 场景函数 + `finally { inst.kill() }` 清理（行 77-113 形态）——新场景：完整链路（Basic→ticket→Hello→Welcome）、重放拒绝（同 ticket 二次 Hello → auth_failed+1008）、无凭据 401、节流 429
- HTTP 层断言用 `node:http` request（行 196-211 形态：手构 Upgrade 头）——401/403/405 断言同此

## Shared Patterns

### 日志出口（红线纪律）
**Source:** `internal/server/server.go:433-443`
**Apply to:** auth.go（认证失败/成功）、throttle.go（429 事件）、server.go 核销分支、`/api/attach` handler
```go
// logEvent 打 D-12② stderr 单行事件，三要素齐全：对端 remote、码值 code、reason 机器串。
func (s *Server) logEvent(remote string, code websocket.StatusCode, reason string) {
	fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
}
```
**红线：** 凭据、ticket、Authorization 头任何形态（含 base64）**永不作为任何 log 参数**（ttyd server.c:142 反例）。HTTP 层事件（非 WS 关闭）沿用同函数签名风格，code 用 HTTP 状态码或约定值——具体形态 planner 定，但三要素结构不变。HTTP 层新事件若要复用，注意当前签名 code 类型是 `websocket.StatusCode`（int 底层，可传 HTTP 码值但注释需说明）。

### Options 注入（测试可覆写纪律）
**Source:** `internal/server/server.go:63-69 + 90-99`
**Apply to:** tickets.go（ttl）、throttle.go（base/cap）、server.go（新字段）
```go
type Options struct {
	Writable         bool
	PingInterval     time.Duration
	HelloTimeout     time.Duration // 测试可覆写
	...
}
// New 内零值兜底：
if opts.HelloTimeout <= 0 {
	opts.HelloTimeout = defaultHelloTimeout
}
```
纪律：生产直传字段与测试覆写字段在注释中分组明示；默认值为包级 `defaultXxx` 常量（行 71-81 形态）。

### exitf 注入 + sync.Once 收口（P1 硬约束）
**Source:** `internal/server/server.go:28-30, 490-498`
**Apply to:** 全部新文件——**新端点/新计时器/新 goroutine 不得新增 exitf 分支**；`/api/attach` handler 纯 HTTP 请求响应，无 goroutine、无计时器；ticket/throttle 惰性清理不开常驻 janitor goroutine。

### 错误帧 + 关闭码成对（D-06/D-07）
**Source:** `internal/server/server.go:298-303`
**Apply to:** server.go 核销失败分支（唯一新增正常客户端可见错误路径）
形态固定：先 `ErrorFrame(code, 英文人话)` → `logEvent(remote, 1008, code)` → `Close(1008, code)`；Error code 与 close reason 同名机器串。攻击面路径（节流命中在核销前置闸时）不发 Error 帧只关闭——是否发帧的受众分治由 planner 按 D-06 定（RESEARCH 建议核销/节流统一 auth_failed 同口径，D-10 已裁决发 Error 帧）。

### CLI flag 契约纪律（P2 D-15）
**Source:** `cmd/wesh/main.go:38-42`
**Apply to:** 6 个新 flag
全名无短选项；help 文案小写开头短语式；行为变更（D-03 裸跑收口）必须在 README 与 help 文案明示。

### 中文注释 + 决策编号引用
**Source:** 全仓（如 server.go:176-178 "Pitfall 5 硬纪律"、proto.go:42 "属 Phase 3/5（deferred）"）
**Apply to:** 全部新文件——注释中文；每个非显然决策引用 D-xx/SEC-xx/Pitfall N/源码行号锚点；类型注释写清不变量。

## No Analog Found

无。全部 15 个文件均有同仓模拟或可逐字采用的 RESEARCH 定稿代码。两点说明：

| 事项 | 说明 |
|------|------|
| HTTP 中间件形态（`func(http.Handler) http.Handler`） | 仓内无先例，headers.go/auth.go 即首例；取 `Server.Handler()` 装配 + RESEARCH Pattern 2/6 定稿代码，不引入框架 |
| TLS 测试（自签证书 + tls.Dial 矩阵） | 仓内无先例；RESEARCH Code Examples 已定稿形态（stdlib crypto/x509 测试内生成，无 fixture 文件），按 `e2e_test.go` 装配风格落地 |

## Metadata

**Analog search scope:** `internal/server/`（全部 5 个 .go）、`internal/proto/proto.go`、`cmd/wesh/`（main.go + main_test.go）、`web/src/main.ts`、`web/uat/phase02.mjs`
**Files scanned:** 10（全部直接 Read；无 >2000 行文件，无需 Grep 定位）
**Pattern extraction date:** 2026-08-17
