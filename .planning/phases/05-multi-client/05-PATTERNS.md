# Phase 5: 多客户端共享 - Pattern Map

**Mapped:** 2026-08-19
**Files analyzed:** 16（新文件 8 + 修改文件 8）
**Analogs found:** 16 / 16（全部命中，本仓单进程代码库内全部有直接先例）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/server/clients.go` 【新】 | service（注册表+hub+outbox+writer） | event-driven fan-out / streaming | `internal/server/server.go`（halfOpenCounter/onChunk/pinger/frame/lifecycle） | role-match（同包同纪律） |
| `internal/server/resize.go` 【新】 | service（仲裁器） | event-driven（防抖/即时重算） | `server.go` AfterFunc timer + `pty/io.go` Resize + RESEARCH Pattern 4 纯函数 | partial（机制先例全，拓扑新） |
| `internal/server/sharetoken.go` 【新】 | service（store）+ controller（/s/ handler） | request-response | `internal/server/tickets.go` + `auth.go` matchCredential + `server.go` Handler mux + `web/embed.go` | exact（复合先例） |
| `internal/server/server.go` 【改】 | controller（Attach/HTTP 路由） | request-response + streaming | 自身现状（改造点已锁定） | self |
| `internal/proto/proto.go` 【改】 | config（协议常量/组帧） | — | 自身现状 | self |
| `internal/server/tickets.go` 【改】 | service | CRUD（map 签发/核销） | 自身（注释兑现，结构零改动） | self |
| `internal/server/auth.go` 【改】 | middleware | request-response | 自身（matchCredential 形态复用） | self |
| `internal/pty/io.go` 【改】 | model（Session 方法） | file-I/O（ioctl/signal） | 自身 Resize/Close fdMu 纪律 | self |
| `cmd/wesh/main.go` 【改】 | config/entrypoint | request-response | 自身 parseArgs/validateStartup/启动打印 | self |
| `web/src/main.ts` 【改】 | component（连接流程） | request-response + streaming | 自身 connect/WELCOME/onclose/sendResize | self |
| `internal/server/e2e_test.go` 【改】 | test | integration（Wave 0 迁移） | 自身（断言形态保留，生命周期断言改写） | self |
| `internal/server/multi_test.go` 【新】 | test | integration | `e2e_test.go` startTestServerWith/dialHello/dialHelloTicket/waitExit | exact |
| `internal/server/slowclient_test.go` 【新】 | test | integration（真 stall 夹具） | `e2e_test.go` + TestDrainBeforeAttach 输出洪水夹具 | exact |
| `internal/server/resize_arb_test.go` 【新】 | test | unit（纯函数表测） | `tickets_test.go` 同包白盒 t.Run 形态 | exact |
| `internal/server/sharetoken_test.go` 【新】 | test | unit + handler 集成 | `tickets_test.go`（白盒 store 测） | exact |
| `web/uat/phase05.mjs` 【新】 | test | protocol integration | `web/uat/phase04.mjs` 全骨架 | exact |

## Pattern Assignments

### `internal/server/clients.go`（新，service，event-driven fan-out + streaming）

**Analog:** `internal/server/server.go`（同包，五处先例拼装）+ RESEARCH Pattern 1/2/3/5 目标形态

**mu+map 最小 store 形态**（参照 halfOpenCounter，server.go:238-264）：
```go
type halfOpenCounter struct {
	mu sync.Mutex
	n  map[string]int
}

func (h *halfOpenCounter) acquire(ip string, max int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n[ip] >= max {
		return false
	}
	h.n[ip]++
	return true
}

// release 到 0 删除 map key——防 map 随历史连接数单调增长（Pitfall 4 泄漏面）
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
→ 注册表沿用：单 mutex（=hubMu）护 set/order/owner/信用门状态；detach 必须清理 map 项与 slice 项（Pitfall 4 双容器防单调增长）。

**chunk 拷贝纪律 + 组帧先例**（server.go:63, 142, 166, 520-529）：
```go
frame    []byte  // OUTPUT 组帧缓冲（仅 ReadLoop 单 goroutine 经 onChunk 访问，无竞争）
// New 内：
frame: make([]byte, 1+32*1024),
// ...
s.frame[0] = proto.Output
// onChunk 现状（将被 hub 取代）：
n := copy(s.frame[1:], chunk)
if err := c.Write(context.Background(), websocket.MessageBinary, s.frame[:1+n]); err != nil {
	return // 写失败（连接已死）：终结由 reader 路径收口（D-11），本块丢弃
}
```
→ hub 改造后：`frame := make([]byte, 1+len(chunk))` 每 chunk 一次拷贝、全部客户端共享只读帧（pty/io.go:13-14 注释红线："onChunk 在读循环 goroutine 内同步调用、复用底层缓冲——回调方如需跨帧持有须自行拷贝"）。**P5-1：outbox 只存共享帧引用，绝不存 chunk 本身。**

**per-conn goroutine 装配先例**（pinger，server.go:550-583 头尾）：
```go
go s.pinger(ctx, c, remote, s.pingInterval)
// ...
func (s *Server) pinger(ctx context.Context, c *websocket.Conn, remote string, interval time.Duration) {
	if interval <= 0 {
		return // D-16：--ping-interval 0 = 禁用
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		// ...
	}
}
```
→ 每客户端 writer goroutine 同款：ctx 挂点（client.cancel 触发 `<-c.done` 返回）、终结收口纪律（不写事件、连接终结由该客户端 reader 路径收口）。

**踢出形态（RESEARCH Pattern 1 关键纪律，planner 直接采用）：**
```go
// 注册表移除与 cancel() 同步内联（非阻塞）；Close 异步独立 goroutine
s.registry.removeLocked(c)
c.cancel()
logEvent(c.remote, websocket.StatusTryAgainLater, "slow_consumer") // R-10 命名
go func() {
	_ = c.conn.Close(websocket.StatusTryAgainLater, "slow_consumer") // 幂等自界；永不内联（P5-2）
}()
```
→ **P5-2 红线：hub 绝不内联 `c.Close()`**（close.go:87-89：5s 写超时 + 5s 等对端，stall 客户端阻塞 ReadLoop ~10s）。`websocket.StatusTryAgainLater` = 1013 库常量。

**锁序纪律（RESEARCH Pattern 5 裁断 R-07）：** hubMu > outboxMu（writer drain 完才取 hubMu 做恢复判定，绝不反序同持）；outbox 自有 mu；信用门 `sync.Cond` 挂 hubMu。

**Options 测试可覆写字段先例**（server.go:85-103 + New:124-133）：
```go
// HelloTimeout/MaxHalfOpenPerIP/PongTimeout 为测试可覆写字段（零值各取默认常量...）
if opts.HelloTimeout <= 0 {
	opts.HelloTimeout = defaultHelloTimeout
}
```
→ OutboxBytes/MaxClients/InputRate/InputBurst/ResizeDebounce 同款零值兜底 + 常量纪律（P2 D-10：一律常量不开 CLI flag——outbox/限速参数；--max-clients 例外是 flag，见 main.go 节）。

---

### `internal/server/resize.go`（新，service，event-driven）

**Analog:** `server.go` helloTimeout AfterFunc（timer 先例）+ `pty/io.go` Resize（fdMu 纪律）+ `proto.go` ClampDim + RESEARCH Pattern 4

**time.AfterFunc 防抖先例**（server.go:377-386）：
```go
helloDone := make(chan struct{})
timer := time.AfterFunc(s.helloTimeout, func() {
	select {
	case <-helloDone:
	default:
		logEvent(remote, websocket.StatusPolicyViolation, "hello_timeout")
		_ = c.Close(websocket.StatusPolicyViolation, "hello_timeout")
	}
})
defer timer.Stop()
```
→ 仲裁器 50ms 防抖同款：单 `time.Timer` reset（RESIZE 上报 → 更新 sizes → 重置 timer）；detach/递补**即时**重算不防抖。Don't Hand-Roll 表："resize 防抖 → 单 time.Timer reset（AfterFunc 先例）"。

**PTY Resize 调用纪律**（pty/io.go:33-40）：
```go
func (s *Session) Resize(cols, rows int) error {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	return pty.Setsize(s.Master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}
```
→ 参数序 (cols, rows) 陷阱已有 io_test.go:24-25 注释锁定；目标尺寸变化才调用（同尺寸 TIOCSWINSZ 不发 SIGWINCH，P5-3）；尺寸已钳 [1,1000]（proto.ClampDim，proto.go:168-176，uint16 转换安全）。

**arbitrate 纯函数形态（RESEARCH Code Examples，planner 直接采用）：**
```go
func arbitrate(members []dims) dims { // members = 参与集最新上报尺寸（已 ClampDim 钳制）
	switch len(members) {
	case 0:
		return dims{} // 无参与者：不动 PTY（保持现状）
	case 1:
		return members[0] // last-wins
	default:
		out := members[0]
		for _, m := range members[1:] {
			out.cols = min(out.cols, m.cols)
			out.rows = min(out.rows, m.rows)
		}
		return out
	}
}
```
→ 纯函数可单测（resize_arb_test.go 表测）；参与集分层（D-09 矩阵）由注册表侧供给 members。

---

### `internal/server/sharetoken.go`（新，service + controller，request-response）

**Analog:** `tickets.go`（store 生成形态）+ `auth.go` matchCredential（subtle 比较）+ `server.go` Handler（mux 装配纪律）+ `web/embed.go`（页面伺服）

**token 生成形态**（tickets.go:45-49，逐字复用）：
```go
var b [16]byte
_, _ = rand.Read(b[:])                          // crypto/rand 失败即进程级问题，沿用 Go 惯例可读性处理
t := base64.RawURLEncoding.EncodeToString(b[:]) // 16B → 22 字符
```
→ 启动时生成 ro/rw 两 token（各 128bit）；打印是产品行为（MULTI-05 授权），token 值**永不入 logEvent 参数**（D-03 红线）。

**subtle 常数时间比较形态**（auth.go:56-65 修正形态 + RESEARCH Code Examples 目标形）：
```go
// matchCredential 先例：SHA-256 等长化消除长度侧信道 + 位或累积不短路
func matchCredential(creds []Credential, user, pass string) bool {
	uh := sha256.Sum256([]byte(user))
	ph := sha256.Sum256([]byte(pass))
	matched := 0
	for _, c := range creds {
		matched |= subtle.ConstantTimeCompare(uh[:], c.userHash[:]) &
			subtle.ConstantTimeCompare(ph[:], c.passHash[:])
	}
	return matched == 1
}
```
→ shareTokens 仅两条目（ro/rw），**不用 map**（RESEARCH R-04：仅两条目、生命周期=进程，map/janitor 全是过度设计）：`struct{ ro, rw [sha256.Size]byte }` 启动生成即预哈希，校验走同款位或不短路，返回命中 mode。

**mux 装配 + 405 fallback 纪律**（server.go:190-198 先例）：
```go
mux.Handle("POST /api/attach", originMiddleware(basicAuth(http.HandlerFunc(s.attachHandler), s.credentials, s.throttle), s.origins))
// 非 POST /api/attach → 405 + Allow: POST。方法模式的内建 405 回退仅在
// 没有任何其它模式匹配时触发——会被 "/" 子树匹配吞掉，故显式注册同文 fallback
mux.HandleFunc("/api/attach", func(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodPost)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
})
```
→ 同款装配：
```go
mux.Handle("GET /s/{token}/", s.sharePage)           // 页面门禁；r.PathValue("token") 取值
mux.HandleFunc("/s/{token}/", func(w, r) {            // path-only 405 fallback（同纪律）
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
})
```
注意 GOROOT 核实三坑（RESEARCH Pattern 6）：尾斜杠 = 匿名多段通配；`/s/abc` 无尾斜杠 → 301 到 `/s/abc/`；内建 405 会被 "/" 子树吞掉。

**页面伺服委托形态**（embed.go:24-51 + server.go:188-204）：
```go
// Handler 内既有装配：
wh, err := web.Handler()  // go:embed 静态伺服（gzip 旁路 + Vary 头）
if len(s.credentials) > 0 {
	mux.Handle("/", basicAuth(wh, s.credentials, s.throttle))
} else {
	mux.Handle("/", wh)
}
```
→ sharePage handler（RESEARCH R-05 裁断）：有效 token → 改写 `r.URL.Path = "/"` 后调既有 embed handler（保留 gzip 旁路与 Vary 语义，dist 单文件全内联无相对资源问题）；无效/缺席 token → **原样委托 `/` 处理链**（凭据模式 basicAuth → 401 + recordFail 自动计入 D-08 统一计数器；无认证模式直接给页）。零新响应形态、不加新 Cache-Control 头。

**throttle 集成（RESEARCH R-03）：** 无效 token 经 401 路径自然计入既有 per-IP 计数器——零新代码；有效 token 优先于 throttle 直接放行（capability 语义）。

---

### `internal/server/server.go`（改，controller）

**Analog:** 自身（改造点 = CONTEXT.md Integration Points 逐行）

改造映射表（现状 → 目标，行号为现状）：

| 现状位置 | 现状形态 | 改造方向 |
|----------|----------|----------|
| server.go:61-62 | `attached atomic.Bool` + `conn atomic.Pointer` | **整体退役** → clients.go 注册表（set + FIFO 递补队列 + owner 指针） |
| server.go:329-334 | ③ 409 单客户端门（CompareAndSwap） | ③ 位换 **max-clients 503 闸**（atomic int 计数，R-06 注册成功后计数、半开不计入）；拒绝路径 `release()` 恰好一次纪律不变（P5-5：沿用 ⓪Origin→①子协议→②halfOpen 429→③503 顺序） |
| server.go:418-451 | 升档序列（checkTicket → Welcome 直写 → pinger → conn.Store） | mode 判定矩阵（RESEARCH Pattern 5 表：ticket mode × --writable × write-policy × owner 在位？）→ 注册表登记（per-client conn/mode/rwEligible/attachSeq/outbox/limiter/cancel 装配）→ **Welcome 走 outbox 首条入队**（writer 全程唯一写端，FIFO 保证首帧）→ pinger 照旧 |
| server.go:469-483 | 读循环 `case proto.Input: if !s.writable { continue }; s.sess.Master.Write(data[1:])` | per-client mode 门（ro 静默丢，per-client 判定替代全局 s.writable）→ `c.limiter.AllowN(time.Now(), len(data)-1)` 超限静默丢（R-02）→ 会话级有界输入队列 tryEnqueue（满则丢，CR-01 完整修复）→ 单 input-writer goroutine 独占 Master.Write |
| server.go:475-480 | `case proto.Resize:` 直接 sess.Resize | ro 端 RESIZE **直接忽略**（D-09 第二闸）；参与集成员 → resize.go 仲裁器上报 |
| server.go:520-529 | onChunk 单 conn 直写 | **hub**（clients.go）：信用门 → 组共享帧 → 逐客户端 trySend → 满则踢出/信用 |
| server.go:619-634 | lifecycle：Wait → Drain → 关单 conn → terminate | 子进程退出 → Drain → **广播 1000**（并行 Close 全部客户端 + 有界等待）→ terminate 唯一终结路径（D-10 推论） |
| server.go:636-644 | wsDisconnected → SIGHUP + exitf(0) | **detach**：注册表移除 + 递补升格（owner 断线 → FIFO 首个 rwEligible 在线者 → outbox 入队 Welcome{mode:"rw", prefs:rw档}）+ 仲裁重算 + 信用门 Broadcast 重估（P5-7：detach/kick/attach/子进程退出全部路径统一 Broadcast）——**不进 exitf**；`terminate(true, 0)` SIGHUP 路径消亡 |
| server.go:214-232 | attachHandler（空 body + 全局 writable mode） | body 携 `{"token":...}` 分支：token 有效 → 按 token 绑定 mode 签 ticket（D-01：/api/attach 不收 mode 参数）；无/错 token → 既有 Basic 链不变（R-03 同 401 计数器）；Open Question 2：可加 503 容量早闸（planner 自决） |
| server.go:179-207 | Handler 三路装配 | 加 `GET /s/{token}/` + 405 fallback 两条（见 sharetoken.go 节） |

**保留不变的纪律：** 守卫区零 WS 资源分配；Accept 后 assert 兜底；helloTimeout AfterFunc；预认证 4KiB/稳态 16KiB 两档 SetReadLimit；logEvent 三要素唯一出口（token/ticket 永不入参）；`logIfMessageTooBig` 两处埋点。

---

### `internal/proto/proto.go`（改，config）

**Analog:** 自身

- proto.go:11 注释更新：`1013 背压踢出 Phase 5 启用，本期占位不实现（D-08）` → 启用说明（库常量 `websocket.StatusTryAgainLater`，组帧/关闭码表注释）。
- proto.go:23 Welcome 注释补充：运行期再推送用于递补升格（P2 D-01/D-02 纪律：既有帧类型再推送 + 加字段均不算动协议）。
- proto.go:44 `permission_denied 属 Phase 5（deferred）` → CONTEXT 裁断：无真实使用场景则保持占位注释不硬用（owner 模式降级走 Welcome 非 Error）。
- WelcomeFrame（proto.go:118-121）组帧形态零改动复用——升格推送与握手 Welcome 同一构造函数：
```go
func WelcomeFrame(mode string, prefs json.RawMessage) []byte {
	b, _ := json.Marshal(WelcomePayload{Mode: mode, Prefs: prefs})
	return append([]byte{Welcome}, b...)
}
```

---

### `internal/server/tickets.go`（改，service）

**Analog:** 自身（结构零改动）

- tickets.go:16 `mode string // ... Phase 5 ro/rw 分签发的占位字段` → 注释兑现（签发方由"全局 --writable 派生"变"token 绑定 / Basic 全局模式"两通道）；issue/redeem 签名与不变量全部不动。
- ticket 生成形态（tickets.go:45-49）同时是 sharetoken.go 的生成先例（见上）。

---

### `internal/server/auth.go`（改，middleware）

**Analog:** 自身

- matchCredential（auth.go:56-65）形态被 sharetoken.go 复用（见上）——本文件预期**零或极小改动**（token 校验逻辑落 sharetoken.go，/api/attach token 分支落 server.go attachHandler）。RESEARCH 推荐结构如是；若 planner 判定 token 比较函数归 auth.go 更内聚，形态同上不另列。

---

### `internal/pty/io.go`（改，model，file-I/O）

**Analog:** 自身 Resize/Close fdMu 纪律（io.go:33-55）

新增方法同款（RESEARCH Code Examples，planner 直接采用）：
```go
// SignalForegroundGroup 向 PTY 前台进程组发 SIGWINCH（D-11 新客强制重绘）；
// TIOCGPGRP 失败/无前台进程组静默降级。fdMu 与 Resize/Close 互斥（既有纪律）。
func (s *Session) SignalForegroundGroup() {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return
	}
	pgid, err := unix.IoctlGetInt(int(s.Master.Fd()), unix.TIOCGPGRP)
	if err != nil || pgid <= 0 {
		return // 静默降级
	}
	_ = unix.Kill(-pgid, unix.SIGWINCH) // 负 pid = 进程组；失败静默
}
```
→ 纪律：fdMu 持锁范围与 Resize 同款（io.go:22-26 注释：Read 绝不可入此锁）；`golang.org/x/sys` indirect→direct（go.mod 已有 v0.47.0 在 go.sum）；调用点 = 注册表 attach 完成后（无条件执行，与仲裁 resize 是否发生无关——P5-3 同尺寸不发信号实证）。

---

### `cmd/wesh/main.go`（改，config/entrypoint）

**Analog:** 自身 parseArgs/validateStartup/启动打印

**新 flag 形态**（parseArgs 现状纪律，main.go:53-68 + 71-78）：
```go
fs.BoolVar(&cfg.writable, "writable", false, "allow client input (default read-only)")
fs.DurationVar(&cfg.pingInterval, "ping-interval", 5*time.Second, "WS ping interval (0 = disable)")
```
→ `--write-policy=owner|all`（StringVar + parse 期枚举校验，畸形值即时报错——client-option 先例 main.go:100-118 的"记录式上报"仅在值含敏感内容时需要，此处枚举值非敏感可直接 return error）；`--max-clients`（IntVar，默认 32）。全名无短选项（P2 D-15）。

**启动校验矩阵扩展点**（validateStartup，main.go:199-216）：
```go
func validateStartup(cfg config) (warn string, err error) {
	if isLoopbackBind(cfg.bind) {
		return "", nil
	}
	// ... D-03/D-05 矩阵
}
```
→ `--write-policy` 与 `--writable` 组合校验（如 write-policy=all 但未给 --writable 的语义裁决）；纯函数零副作用纪律不变。

**启动打印形态**（main.go:272-278）：
```go
scheme := "http"
if cfg.tlsCert != "" {
	scheme = "https"
}
fmt.Printf("listening on %s://%s\n", scheme, ln.Addr())
```
→ `listening on ...` 行不动，**追加两行**（RESEARCH Pattern 7）：
```
share read-only:  http://9.134.229.124:7681/s/<ro-token>/
share read-write: http://9.134.229.124:7681/s/<rw-token>/   # 仅 --writable 时打印（D-05）
```

**host 回填（RESEARCH Pattern 7 裁断 R-04，planner 直接采用）：**
```go
// bind 为 0.0.0.0/:: 时：UDP-dial 技巧优先（路由表感知，零流量），接口扫描兜底
func outboundIPv4() string {
	if conn, err := net.Dial("udp", "192.0.2.1:80"); err == nil { // RFC 5737 TEST-NET-1
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil {
			return addr.IP.String()
		}
	}
	// fallback：net.Interfaces() 索引序首个 up && !loopback 接口的首个 IPv4
	// 全失败兜底：打印 bind 原样（不阻断启动）
}
```
→ 具体 bind 地址原样使用（D-04）；loopback bind 打印 loopback；端口取 `ln.Addr()`；scheme 随 TLS 分岔。**Anti-pattern：朴素接口扫描当唯一手段**（本机实证 docker0/bridge 抢先）。

**prefs 双档分化（P5-6/D-13）：** aggregateClientPrefs（main.go:166-179）产**双变体**——ro 档不含 osc52、rw 档按全局 --osc52（osc52 是服务端专有键，产双 blob 保持不透明透传纪律，不做运行期 JSON 手术）；Options.ClientPrefs 分裂为两字段，attach Welcome 按 mode 选档、升格 Welcome 携 rw 档。

---

### `web/src/main.ts`（改，component）

**Analog:** 自身 connect/WELCOME/onclose/sendResize

改造映射表（现状 → 目标，行号为现状）：

| 现状位置 | 改造 |
|----------|------|
| main.ts:330-372 connect() 开头 | `location.pathname` 匹配 `^/s/([^/]+)/$` 提取 token → `fetch('/api/attach', {method:'POST', body: JSON.stringify({token})})`；无 token 时 body 空（现状形态）；ticket→Hello 链路不变 |
| main.ts:340 | `fetch('/api/attach', { method: 'POST' })` | 携 token 时加 body；503 分支（若服务端加容量早闸）→ "Server is full" 专版文案 |
| main.ts:365-371, 519-525, 544-549 | 三处 "another client is already attached (wesh currently allows a single client)" 文案 | **改写**（多客户端后事实错误）→ "server unreachable or at capacity" 语义 |
| main.ts:386-397 WELCOME 分支 | 现状仅 `if (w.mode === 'ro')` 单分支 | 补 **rw 分支**（升格处理）：`isRO=false; term.options.disableStdin=false; setTitle()`（去 `[ro] ` 前缀，经单一写口 main.ts:209-211）+ `fit.fit()`（触发 onResize→sendResize 纠正尺寸）；prefs 应用段（main.ts:401-473）幂等可重入（queryKeys 跳过机制），升格 Welcome 重跑无泄漏 |
| main.ts:235-242 sendResize | `if (!helloSent) return;` 后 | 加 `if (isRO) return;`（D-09 第一闸；Hello 携首尺寸不受影响——helloSent 门先于 isRO 生效） |
| main.ts:581-587 onclose 1013 分支 | 占位文案 "The server asked this client to retry later." | 更新为 slow consumer 被断开 + 手动刷新英文语义（D-10）；维持 showStatus + "Reload this page" 链接形态（main.ts:309-325），不做任何自动重连 |

**保留纪律：** ticket 只存闭包变量与 Hello 载荷（main.ts:328-329 红线：禁 URL query/localStorage/console）——**token 同样适用**；onclose 只认 code 不认 reason（main.ts:528-530）。

---

### `internal/server/e2e_test.go`（改，Wave 0 迁移）

**Analog:** 自身（P5-4 逐文件核实清单）

- `waitExit` 收口断言 ≥8 处改写：断开不再 exitf → 改注册表移除断言（exitCh 不触发 + 会话仍存活可 echo）。
- `TestSecondClient409`（e2e_test.go:326-372）整测替换 → 双 attach 成功 + 第三人 503。
- SIGHUP 两测（e2e_test.go:436-473 + helperArgv "wesh-helper-sighup" 分支）删除——路径消亡。
- `TestExitCodePropagation`（374-402）保留核心（子进程退出 → 1000 → exitf(42)），适配广播形态。
- **helper/dialHello/startTestServerWith/waitExit 形态全部保留**——新测试文件直接复用（见下）。

---

### `internal/server/multi_test.go`（新，test，integration）

**Analog:** `e2e_test.go`（exact——同包 server_test，helper 直接复用零改动）

**装配形态**（e2e_test.go:105-127）：
```go
func startTestServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string) {
	t.Helper()
	sess, err := pty.Start(argv)
	// ...
	exitCh = make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, opts)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	// ...
	t.Cleanup(func() { ln.Close() })
	go http.Serve(ln, srv.Handler())
	return exitCh, "ws://" + ln.Addr().String() + "/ws"
}
```

**握手形态**（e2e_test.go:133-158 dialHello / 177-202 dialHelloTicket）：
```go
func dialHello(t *testing.T, ctx context.Context, wsURL string, cols, rows int) (*websocket.Conn, string) {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	// ... 发 Hello → 读首帧断言 Welcome → 返回 (conn, mode)
}
```
→ 双客户端场景：两次 dialHello（可变 cols/rows 参数——签名参数化正是为此，e2e_test.go:131-132 注释）；Welcome mode 断言矩阵（owner/all/ro）；fan-out 一致性断言 = 两端各累积收齐同一 payload（TestEchoPTY 累积模式 e2e_test.go:52-66 双份）；**Options.OutboxBytes 覆写加速触发**（测试覆写先例）。

---

### `internal/server/slowclient_test.go`（新，test，integration）

**Analog:** `e2e_test.go` + TestDrainBeforeAttach 输出洪水夹具

**输出洪水子进程夹具**（e2e_test.go:83-84）：
```go
sess, err := pty.Start([]string{"seq", "1", "200000"}) // 约 1.3MB 输出
```

**stall 客户端夹具（RESEARCH Validation 裁决）：** 测试客户端建连完成 Hello 后**不再调用 Read**——TCP 接收缓冲填满 → 服务端 writer 阻塞 → outbox 涨满 → 1013 踢出断言（CloseError.Code == websocket.StatusTryAgainLater）；同实例第二客户端正常 Read 断言无卡顿（fan-out 一致性 + 时延上界）；Options.OutboxBytes 调小加速触发。信用门测试：全体可写端 stall → 子进程输出暂停可观测（hub 持块）；一端 CloseNow → 门开（P5-7 验证序列）。

---

### `internal/server/resize_arb_test.go` + `sharetoken_test.go`（新，test，unit）

**Analog:** `tickets_test.go`（exact——同包白盒单测形态）

**同包白盒 + t.Run 子测 + now 手工注入形态**（tickets_test.go:1-20, 104-114）：
```go
package server  // 同包白盒（内部类型不导出，不走 server_test 黑盒——有意选择）

func TestTicketStore(t *testing.T) {
	base := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	t.Run("零值 ttl 兜底 60s", func(t *testing.T) {
		for _, ttl := range []time.Duration{0, -time.Second} {
			ts := newTicketStore(ttl)
			if ts.ttl != defaultTicketTTL {
				t.Errorf("newTicketStore(%v).ttl = %v, want %v（零值可用纪律）", ttl, ts.ttl, defaultTicketTTL)
			}
		}
	})
}
```
→ resize_arb_test.go：arbitrate 纯函数表测（0/1/N 参与者、min-rect、2→1 恢复、ClampDim 已钳输入）；sharetoken_test.go：subtle 比较（ro/rw 命中各自 mode、错 token 同归 false 无 oracle、22 字符形态）、/s/ 路由门禁（有效 → 200、无效 → 委托 `/` 链 401 challenge/无认证给页、405 fallback + Allow 头）、/api/attach token 分支。**零真实 sleep**（now 注入纪律）；Go 集成侧仲裁断言通道 `pty.Getsize`（RESEARCH 已核实 winsize.go:18）。

---

### `web/uat/phase05.mjs`（新，test，protocol integration）

**Analog:** `web/uat/phase04.mjs`（exact——全骨架复用）

**复用件（phase04.mjs 逐行形态）：**
- `startWesh(args)`（:44-60）：spawn + `--bind 127.0.0.1 --port 0` + stdout 正则解析端口/scheme + 8s 超时 SIGKILL——**stdout 解析正则需扩展**：除 `listening on` 行外解析两行 `share read-only:`/`share read-write:` 链接（MULTI-05 断言锚点）。
- `spawnExpectExit(args)`（:64-76）：启动校验拒绝路径（exit 2 + stderr 文案）。
- `dialHello(port)`（:82-96）：WS + Hello → Welcome 轮询收口——双客户端场景同 port 调两次。
- `waitClose(ws, timeoutMs)`（:98-101）、`check(id, name, ok, detail)`（:37-40）、场景数组 + 串行循环 + 结果汇总（:251-264）。
- **红线同守**（:6-9）：token 值只作断言材料，永不进 check detail/控制台输出——detail 只打状态码/布尔/形状。

**phase05 特有场景：** 双客户端输出一致（两 dialHello 收同一 OUTPUT 流）；ro/rw 链接全链（stdout 链接 → GET 页面 200 无 Basic → fetch /api/attach body 携 token → ticket → dialHelloTicket mode 正确）；错 token → Basic 矩阵；满员 503（--max-clients 小值 spawn）；1013 踢出文案面（协议层只验关闭码与 reason 机器串）。**单次语义纪律放松**（phase04.mjs:11-12 注释"每个需 WS 的场景独立 spawn"）——多客户端下同进程多 WS 建连正是本 phase 特性，但生命周期断言必须适配新形态。

## Shared Patterns

### 认证/比较（crypto 纪律）
**Source:** `internal/server/auth.go:56-65`（matchCredential）+ `internal/server/tickets.go:45-49`（生成）
**Apply to:** sharetoken.go（token 生成+比较）、auth.go（不变）
- 生成：crypto/rand 16B → base64.RawURLEncoding 22 字符（128bit，独立 secret 不从凭据派生，C6）
- 比较：SHA-256 等长化 + subtle.ConstantTimeCompare 位或累积不短路（Pitfall 1 长度侧信道 + 组序号时序泄露双防）
- 红线（SEC-01）：token/ticket/凭据/Authorization 头任何形态（含 base64）永不入 logEvent 参数——三要素只有 remote/code/reason

### stderr 单行事件（logEvent）
**Source:** `internal/server/server.go:600-602`
**Apply to:** 1013 踢出（reason="slow_consumer"，code=1013）、503 满员（reason="max_clients"，code=503——HTTP 层事件 code 复用 HTTP 状态码值既有裁决）
```go
func logEvent(remote string, code websocket.StatusCode, reason string) {
	fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
}
```
命名族同构 snake_case（hello_timeout/pong_timeout/message_too_big 同族，R-10）。

### 错误处理与连接终结收口
**Source:** `internal/server/server.go:526-528, 455-465, 550-583`
**Apply to:** clients.go writer goroutine、server.go 读循环改造
- 写失败不补救——连接已死，终结由该客户端 reader 路径收口（每客户端映射既有 D-11 纪律）
- 读循环 `c.Read` 错误 → detach（注册表移除 + 递补 + 仲裁重算 + Broadcast）——不再进 exitf
- goroutine 终结挂点 ctx（cancel 随客户端生命周期）；pinger 精确分类纪律保留（仅 DeadlineExceeded → pong_timeout + CloseNow）

### 常量与 Options 纪律
**Source:** `internal/server/server.go:85-133`（Options 零值兜底）+ `internal/proto/proto.go:53-63`（读上限常量注释形态）
**Apply to:** clients.go（OutboxBytes 512KiB/resume 50%/InputRate 32KiB/s/InputBurst 64KiB/输入队列 256KiB/ResizeDebounce 50ms）、main.go（--max-clients 默认 32）
- 攻击面上限一律常量不开 flag（P2 D-10）；容量策略（--max-clients）与模式（--write-policy）是部署关切开 flag（D-08 裁决分类）
- 全部参数初值挂"Phase 9 负载标定回填"注释；测试经 Options 覆写加速

### 节流计数器
**Source:** `internal/server/throttle.go:70-92`（recordFail/recordSuccess）+ `server.go:497-515`（checkTicket 顺序敏感）
**Apply to:** sharetoken.go /s/ 门禁、/api/attach token 分支
- token 失败经既有 401 路径自然计入 D-08 统一 per-IP 计数器（R-03 零新代码）；有效 token 优先于 throttle 放行
- 失败响应与 Basic 失败同文同码（无 oracle）

### 前后端帧常量手工对齐
**Source:** `internal/proto/proto.go:6` ↔ `web/src/main.ts:11-24`（两侧注释互相指路）
**Apply to:** 1013 注释启用、升格 Welcome 说明、main.ts 帧处理
- 不加新帧类型字节（P2 D-01 类型空间经济）；升格复用 'W' Welcome 运行期推送（R-09）

### UAT harness
**Source:** `web/uat/phase04.mjs` 全骨架
**Apply to:** phase05.mjs（见上节）；测试输出红线（secret 只作断言材料永不进 detail）

## No Analog Found

无。全部 16 个文件在仓内找到直接或复合先例——本 phase 的设计增量在**拓扑与规则**（hub/信用/仲裁/递补四件套，RESEARCH Pattern 1-5 已给定稿代码形态），底层机制（令牌桶/通配路由/ioctl/关闭握手/防抖/subtle 比较）全部有库或仓内先例。

唯一"新拓扑"成分（fan-out hub + 全局信用门 + 仲裁器）无仓内运行先例，但其组成件全部有先例：trySend 非阻塞投递 = halfOpenCounter 非阻塞判定同构；cond 门 = stdlib 原语；writer goroutine = pinger 装配先例；目标形态代码 RESEARCH Pattern 1/2/3/4 已逐行给出并经库源码验证，planner 直接采用即可。

## Metadata

**Analog search scope:** `internal/server/`（全部 19 个 go 文件）、`internal/proto/`、`internal/pty/`、`cmd/wesh/`、`web/src/`、`web/uat/`、`web/embed.go`
**Files scanned:** 16 个文件完整读取（server.go/tickets.go/throttle.go/auth.go/origin.go/proto.go/io.go/spawn.go/main.go/main.ts/e2e_test.go/tickets_test.go/phase04.mjs/embed.go + 2 个上游文档）
**Pattern extraction date:** 2026-08-19
