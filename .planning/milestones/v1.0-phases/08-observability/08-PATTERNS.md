# Phase 8: 可观测性 - Pattern Map

**Mapped:** 2026-08-27
**Files analyzed:** 12（4 新建 + 8 修改/迁移）
**Analogs found:** 12 / 12（其中 2 个新建文件的核心写法无代码库先例，以 RESEARCH.md 骨架为准——见「无完全先例」节）

> **纠错提示（RESEARCH Summary ⑤）：** CONTEXT.md 把 `logEvent` 定位在 auth.go 有误——实际在 `internal/server/server.go:1071-1077`（包级函数、全部 18 个调用点唯一出口的事实成立）。planner 不要找错文件。

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/server/metrics.go`（新） | handler + 快照采集 | request-response（读快照→文本） | `server.go` Handler() + `clients.go` afterDrain 锁序 + `sharetoken.go` 405 fallback | role-match（exposition 写法无先例，用 RESEARCH Pattern 2） |
| `internal/server/health.go`（新，或并入 metrics.go） | handler | request-response（读 atomic→JSON） | `server.go` issueTicketJSON（JSON body 写法） | role-match |
| `internal/server/log.go`（新，logEvent 迁入；或就地 server.go） | logging 基础设施 | event-driven（多调用点→单行 emit） | `server.go` logEvent（1047-1077）+ `proxy.go` 文件组织纪律 | exact（是同一函数的迁移） |
| `web/uat/phase08.mjs`（新） | test（协议层 UAT） | spawn 真实二进制 + fetch/WS 断言 | `web/uat/phase07.mjs` | exact |
| `internal/server/server.go`（改） | handler 装配 + 生命周期 | request-response + event-driven | 自身（Handler/Attach/lifecycle/Shutdown 既有形态） | exact |
| `internal/server/clients.go`（改） | service（注册表/踢出/断开） | event-driven + CRUD（计数器） | 自身（kickSlowConsumerLocked/detach/registerLocked） | exact |
| `internal/server/auth.go`（改） | middleware | request-response | 自身（basicAuth 101-123） | exact |
| `internal/server/proxy.go`（改） | utility（提取/清洗） | transform | 自身（sanitizeRemoteUser/clientIP/remote） | exact |
| `cmd/wesh/main.go`（改） | config/装配 | 启动直传 | 自身（version var + server.New Options 调用） | exact |
| `internal/server/limits_test.go`（改）+ emptyexit/auth_e2e/proxy_e2e/multi_test.go（改） | test | event-driven 断言 | `limits_test.go` captureStderr + TestOversize1009 | exact |
| `web/uat/phase05.mjs` S6 / `phase07.mjs` S4（改） | test | event-driven 断言 | `phase07.mjs` S4 现状形态 | exact |
| `README.md`（改） | doc | — | README §部署与配置（Phase 7）节风格 | exact |

## Pattern Assignments

### `internal/server/log.go`（新）——slog 原子迁移（D-13/D-15/D-18，08-01 plan 核心）

**Analog 1（迁移对象现状）：** `internal/server/server.go:1071-1077`

```go
func logEvent(remote string, code websocket.StatusCode, reason string, remoteUser ...string) {
	if len(remoteUser) > 0 && remoteUser[0] != "" {
		fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s remote_user=%s\n", remote, code, reason, remoteUser[0])
		return
	}
	fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
}
```

**关键语义必须保持：** 现状每次调用时读 `os.Stderr` **变量**（调用时解析）——captureStderr 测试依赖该语义。slog handler 构造时捕获 writer（GOROOT json_handler.go:30-41），故必须用动态 writer（RESEARCH Pattern 1 逐字形态）：

```go
// Source: 08-RESEARCH.md Pattern 1（无代码库先例，research 设计 + GOROOT 实证）
type stderrW struct{}

func (stderrW) Write(p []byte) (int, error) { return os.Stderr.Write(p) }

// 包级单例：JSONHandler 内部 mu 串行化记录，并发 emit 安全且每记录恒完整一行。
var eventLog = slog.New(slog.NewJSONHandler(stderrW{}, nil))

// logEvent 签名不变（D-13：18 个调用点 16 个零改动）；schema = D-18 msg="event" + event 字段。
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

**红线注释随函数迁移（server.go:1063-1067，逐字保留语义）：** 凭据/ticket/Authorization 头任何形态（含 base64）禁止作为任何参数传入；remote_user 值必须经 sanitizeRemoteUser 清洗且来源只能是配置头名 HTTP 头（清洗在提取点完成，单一写口纪律）。

**装配点（Discretion，RESEARCH 倾向）：** server 包包级 `eventLog`，**不调** `slog.SetDefault`（不污染全局默认 logger，测试隔离性更好；D-15 恒 JSON 恒 INFO 无配置面）。main.go 零改动。

**禁止：** kv 交替参数（奇数/非 string 键产出 `!BADKEY`，GOROOT logger.go:187）——一律 `LogAttrs` + 类型化 attr。

**文件组织（proxy.go:3-39 / origin.go 先例）：** 新文件包声明后注释头登记决策号（D-13/D-15/D-18 + 迁移来源），包级纯函数/单例内聚。

---

### `internal/server/metrics.go`（新）——/metrics handler + 手写 exposition（D-01/D-02/D-03/D-04/D-05/D-06）

**Analog 1（mux 注册 + 认证闸装配）：** `internal/server/server.go:401-416`（basicAuth 包装先例）+ `sharetoken.go:122-128`（405 fallback 成对注册先例）

```go
// server.go:402——basicAuth 包装形态（D-08 /metrics 跟随认证闸直接复用）
root := basicAuth(wh, s.credentials, s.throttle, s.proxy)
```

```go
// sharetoken.go:123-128——方法模式 + path-only 405 fallback 成对注册（/healthz、/metrics 各一对；
// 不注册 fallback 则 POST /metrics 被 "/" 子树吞进静态伺服，RESEARCH Pitfall 7）
mux.Handle("GET "+bp+"/s/{token}/", s.sharePage(page, root))
mux.HandleFunc(bp+"/s/{token}/", func(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
})
```

**D-09 根路径固定：** 注册时**不带 bp 前缀**（`mux.Handle("GET /metrics", ...)`），与上方 bp 前缀形态正交——探活/采集器直连后端端口，路径恒定。拒绝双挂。

**Analog 2（handler 骨架形态——状态读取 + 响应写出）：** `internal/server/server.go:536-550`（issueTicketJSON）

```go
// server.go:537-549——容量闸判定源 registry.n + 响应头/状态码/写body 的既有纪律
if s.registry.n.Load() >= int64(s.maxClients) {
	logEvent(s.proxy.remote(r), websocket.StatusCode(http.StatusServiceUnavailable), "max_clients", s.proxy.remoteUser(r))
	http.Error(w, "server is full", http.StatusServiceUnavailable)
	return
}
...
w.Header().Set("Content-Type", "application/json")
w.Header().Set("Cache-Control", "no-store")
w.WriteHeader(http.StatusOK)
_, _ = w.Write(body)
```

**Analog 3（registry 快照并发形态——锁序 R-07）：** `internal/server/clients.go:451-456`（afterDrain 的 hubMu > outbox.mu 同序同持先例）

```go
// clients.go:451-456——metrics handler 读取 registry 的唯一合法形态：
// hubMu 一趟内逐 outbox.mu 读深度（绝不反序同持，RESEARCH Pitfall 3 死锁防线）
s.hubMu.Lock()
defer s.hubMu.Unlock()
c.outbox.mu.Lock()
cur := c.outbox.bytes
c.outbox.mu.Unlock()
```

**读取源（全部预埋就位，逐字核实）：**

| 数据源 | 位置 | 形态 |
|--------|------|------|
| `s.registry.n.Load()` | clients.go:262 | atomic.Int64 |
| `s.registry.kicks` | clients.go:264 | hubMu 内 plain int（注释逐字「Phase 8 OPS-07 观测性挂点」） |
| `s.registry.gateTransitions` | clients.go:268 | hubMu 内 plain int |
| `s.inputDrops.Load()` | server.go:103 | atomic.Int64 |
| `s.inputQ.droppedInputs.Load()` | clients.go:200 | atomic.Int64 |
| `c.attachSeq`（client_id 来源） | clients.go:274-275 | registerLocked 分配，从 1 起单调递增 |

**禁止：** 为 metrics 读取把 kicks/gateTransitions 改 atomic（为读改写的反向耦合，破坏 R-07 选型——hubMu 快照即可，RESEARCH Pattern 3 裁决建议）。

**exposition writer（无代码库先例——RESEARCH Pattern 2 骨架为准）：**

```go
// Source: 08-RESEARCH.md Pattern 2（规范 CITED prometheus/docs exposition_formats.md）
w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
var b strings.Builder
writeGauge(&b, "wesh_clients_connected", "Currently attached WebSocket clients.", snap.clientsConnected)
writeCounter(&b, "wesh_clients_kicked_total", "Clients kicked with 1013 (slow consumer).", snap.kicks)
// ...17 series 清单见 RESEARCH Pattern 2 表（wesh_ 前缀、_total/_bytes 后缀惯例）
fmt.Fprint(w, b.String()) // 每行 \n 收尾，末行恒带 \n（规范硬性要求）

func writeCounter(b *strings.Builder, name, help string, v int64) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, v)
}
```

**escLabel helper（build_info version label 唯一消费点）：** `\`→`\\`、`"`→`\"`、`\n`→`\\n`（反斜杠先行，顺序敏感）——规范逐字要求，单侧定义（RESEARCH Don't Hand-Roll 表）。

**version 来源（Options 单一通道）：** `main.go:32` `var version = "dev"` → 新增 `Options.Version` 生产直传字段 → `s.version`。先例形态见下方 Shared Patterns「Options 单一通道」。

---

### `internal/server/health.go`（新，或并入 metrics.go）——/healthz handler（D-07/D-09/D-10/D-11）

**Analog：** `server.go:536-550`（issueTicketJSON 的 JSON body + 响应头纪律）+ registry.n atomic 读（clients.go:262）

**RESEARCH 给出的锁定骨架（D-10/D-11）：**

```go
// Source: 08-RESEARCH.md Code Examples（数据源全部已核实）
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

**两枚新 atomic.Bool 的挂点：**
- `draining`：Shutdown() 入口置位——与 `server.go:1265-1266` `s.exiting = true` 同源同位（hubMu 外 atomic 读故须 atomic.Bool，与 registry.n 的「hubMu 外 atomic load」选型先例同构，clients.go:250-256 注释论证）
- `sessionAlive`：New 尾部置 true，lifecycle `sess.Wait()` 返回后（server.go:1168 之后）置 false

**D-07 免认证：** 该路由**不包** basicAuth——整站闸唯一窄例外（装配时直接 `mux.Handle("GET /healthz", ...)` 注册在认证分支之外，双模式（凭据/无认证）均注册）。405 fallback 成对注册同上。

---

### `internal/server/server.go`（改）——事件打点 + Handler 注册 + atomic 字段

**logEvent 迁移：** 1071-1077 函数体按上方 log.go 节换实现（或整体迁入 log.go——Open Question 1，两形态零语义差异，倾向新文件）；注释头同步更新（现注释 1047-1070 明写「Phase 8 升级 slog 结构化日志（OPS-08），本期为过渡形态」——迁移后删除过渡表述）。

**attach 事件（D-17/D-20）：** 升档完成点（registerLocked 之后）emit `event=attach`，携 client_id=`c.attachSeq`。现有调用点形态先例（server.go:684）：

```go
logEvent(remote, websocket.StatusCode(http.StatusServiceUnavailable), "max_clients", remoteUser)
```

**session_start/session_end（D-17/D-22）：** lifecycle（server.go:1167-1226）内打点。session_end 字段源现成（server.go:1168-1173 + 1154-1157）：

```go
// server.go:1168-1173——exit code 提取（sess.Wait 返回 + ExitError）
err := s.sess.Wait()
code := 0
var ee *exec.ExitError
if errors.As(err, &ee) {
	code = ee.ExitCode()
}
```

```go
// server.go:1154-1157——信号死亡 WaitStatus 提取（exitMessage 内联现状；
// 抽为包级小 helper 供 exitMessage 与 session_end 两消费点共用，单侧定义纪律）
sig := code
if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
	sig = int(ws.Signal())
}
// 信号名：signalName(syscall.Signal(sig))（server.go:1099-1129 映射表现存，
// 未命中返回 ("", false) → D-22「仅信号死亡出键」）
```

`duration_seconds`：lifecycle 内 `time.Since(s.startedAt).Seconds()`（startedAt 在 New 尾部记录）。

**shutdown 事件 + draining 置位：** Shutdown()（server.go:1264-1286）入口——`s.exiting = true`（1266 行）同位置 `s.draining.Store(true)`。

**detach reason 传递（D-21，RESEARCH Pattern 4 三机制表——-race 关键）：**

| reason | 机制 | 先例/依据 |
|--------|------|-----------|
| `kick` | hubMu 内天然同步——kickSlowConsumerLocked（clients.go:500-526）就地 emit detach reason=kick，替换现 slow_consumer 行 | clients.go:506 现 `logEvent(c.remote, websocket.StatusTryAgainLater, "slow_consumer", c.remoteUser)` |
| `pong_timeout` | pinger 置位时取 hubMu 写 c 字段（冷路径，与 kick 同锁语义统一）；detach 在 hubMu 内读 | pinger 现站点 server.go:1040；**禁止 plain 字段跨 goroutine 传递**（RESEARCH Pitfall 2，-race 必中） |
| `shutdown` | 复用 `s.exiting` 判定——detach 在 hubMu 内读，exiting 同为 hubMu 字段 | server.go:1198/1266 两置位点 |
| `normal` | 默认分支（三者均不命中） | — |

**恰好一次归属：** detach 事件由 removeLocked 返回 true 的路径 emit（reader 路径 detach() clients.go:690 或 kick 路径 kickSlowConsumerLocked()）——与既有「removeLocked 胜出方负责 close(done)/cancel」同一所有权规则（clients.go:693-694）。

**pinger 现状调用点（server.go:1040，迁移后折入 detach reason，不再单独打行）：**

```go
logEvent(remote, websocket.StatusAbnormalClosure, "pong_timeout", remoteUser)
c.CloseNow()
```

---

### `internal/server/clients.go`（改）——detach/kick 打点 + clientsTotal 计数器

**Analog：** 自身既有形态，全部挂点逐字就位。

**detach 收口（clients.go:690-713）——事件 emit 插入点 = removeLocked 返回 true 之后、hubCond.Broadcast 之前：**

```go
// clients.go:690-697——现状形态（emit 插在第 696 行 close(c.done) 之后的 hubMu 持有内）
func (s *Server) detach(c *client) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	if !s.registry.removeLocked(c) {
		return // 已被 kick 移除——close(done)/cancel 恰好一次由成员判定保证
	}
	close(c.done)
	c.cancel()
```

**kick 路径（clients.go:500-526）：** 现 slow_consumer logEvent（506 行）替换为 detach reason=kick 事件；`s.registry.kicks++`（505 行）已是 metrics 读取源，不动。

**clientsTotal 新增：** hubMu 内 plain int64，registerLocked（clients.go:273-282）唯一加点——与 kicks 同形态（hubMu 保护，R-07 下无需 atomic），metrics 快照在锁内读。

**exit_when_empty 族事件（D-17 沿用现状事件名）：** maybeExitWhenEmptyLocked（clients.go:741-770）与 cancelExitEmptyTimerLocked（794-801）的 logEvent 调用点零改动（签名不变、事件名不变）——迁移后自动出 JSON 形态。

---

### `internal/server/auth.go`（改）——throttled 携 retry_after + auth 计数器（D-06/D-23）

**Analog：** 自身 basicAuth（101-123）。retry_after 字段来源现成（106 行已算出 `retry int64`）：

```go
// auth.go:104-110——throttled 站点现状；D-23：事件携 retry_after 秒数（值 = 106 行 retry）
if wait, throttled := th.retryAfter(ip, time.Now()); throttled {
	retry := int64((wait + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.FormatInt(retry, 10))
	logEvent(p.remote(r), websocket.StatusCode(http.StatusTooManyRequests), "throttled", p.remoteUser(r))
	http.Error(w, "too many requests", http.StatusTooManyRequests)
	return
}
```

实现形态：throttled/auth_failed 两事件属「全量目录」扩展字段——logEvent 签名不变（D-13），扩展字段走新增的 emit 变体或 attrs 追加 helper（planner 定形；约束：auth_failed 事件**不含用户名**，SEC-01 红线重申；两计数器 `wesh_auth_failed_total`/`wesh_auth_throttled_total` 递增点 = auth.go:108/116 与 server.go:773 三站点，无 IP label）。

**红线注释保持（auth.go:89-93）：** 凭据/Authorization 头任何形态永不入日志参数；401/429 body 恒通用文案。

---

### `internal/server/proxy.go`（改）——remote 字段 sanitize 推广（D-19）

**Analog：** 自身 sanitizeRemoteUser（55-67）+ clientIP（89-103）+ remote（108-113）。

```go
// proxy.go:55-67——D-19 同款清洗实现（直接复用，不重写）：
// 逐 rune 剥离 ch<=0x1f（C0）、ch==0x7f（DEL）、0x80<=ch<=0x9f（C1），截断 128 rune
func sanitizeRemoteUser(s string) string {
	r := make([]rune, 0, len(s))
	for _, ch := range s {
		if ch <= 0x1f || ch == 0x7f || (ch >= 0x80 && ch <= 0x9f) {
			continue
		}
		r = append(r, ch)
		if len(r) >= 128 {
			break
		}
	}
	return string(r)
}
```

```go
// proxy.go:89-103——clientIP 现状：链首经 net.ParseIP 校验（07-review CR-02），
// ParseIP 通过值字符集恒为 [0-9a-fA-F:.]，结构性排除注入
if ip := strings.TrimSpace(first); ip != "" && net.ParseIP(ip) != nil {
	return ip
}
```

**D-19 落地要点：** trust 模式 remote 字段（XFF 链首）过 sanitizeRemoteUser 同款清洗——挂点在 `remote()`（108-113）或 clientIP 返回值；清洗在提取点完成（单一写口纪律，与 remote_user 同款）。注意这是**纵深第二道**（ParseIP 已是第一道结构性闸），两道并存不冲突；sanitize 必需因 encoding/json 只转义 C0、C1（如 NEL U+0085）原样穿透（RESEARCH Pitfall 5，GOROOT encode.go:1023 实证）。

**红线注释头保持（proxy.go:30-39）**：token/ticket/凭据永不作为 remote_user 或任何 logEvent 字段出现。

---

### `cmd/wesh/main.go`（改）——version → Options.Version 透传

**Analog：** 自身既有形态。

```go
// main.go:31-32——version 现状（发布构建 ldflags 注入；Phase 9 goreleaser 接线是既定后续）
// version 由发布构建注入；开发构建为 dev。
var version = "dev"
```

```go
// main.go:1134——server.New Options 直传先例（生产直传字段原样透传形态，
// Version 新增同通道同注释纪律；改动 = 尾部追加 Version: version 一个 kv）
srv := server.New(sess, os.Exit, server.Options{Writable: cfg.writable, ..., StopTimeout: cfg.stopTimeout})
```

**零改动区（D-14/D-16 红线）：** 启动行（1156 `listening on ...`）、分享链接行（1180-1183）、警告行（`wesh: warning:` 前缀）全部保持人读文本——01-05 冒烟与 UAT 以启动行解析实际端口，既有消费者零破坏。slog 装配倾向不进 main（server 包包级 eventLog，见 log.go 节）。

---

### `web/uat/phase08.mjs`（新）——协议层 UAT

**Analog：** `web/uat/phase07.mjs`（exact match，逐字复制 harness 后换场景矩阵）。

**harness 复制清单（phase07.mjs 逐字沿用）：**

1. **文件头纪律块（1-25 行同款）：** 覆盖矩阵注释 + 红线（token/凭据值只作断言材料永不进 check detail）+ 单次语义纪律 + 运行/调试方式
2. **check/skip/emittedDetails（62-75 行）：** 结果收集 + 平台豁免记录形态
3. **sensitiveTokens + redactArgs（77-92 行）：** 敏感值闭包数组 + argv 脱敏
4. **startWesh（103-145 行）：** spawn + 启动行解析 + stderr 持续捕获（`stderrText()` 是 JSON 事件行断言通道——本 phase 核心消费点）
5. **dialHello/waitClose/waitExit/collectUntilClose（171-219 行）：** WS 握手/关闭/进程退出断言通道
6. **assertOutputClean（759-764 行）：** 输出自净运行时自证
7. **场景表 + ONLY 过滤 + 汇总退出码（766-797 行）：** `PHASE08_ONLY=S1,S3` 同款调试形态

**本 phase 特有场景夹具先例：**

```javascript
// phase07.mjs:677-690（S7）——SIGTERM 驱动真实关停序列（503 draining 场景同形态：
// 用 --stop-timeout 3s 拉长 draining 窗口后 SIGTERM 立即轮询 /healthz，08-RESEARCH OQ3 建议）
const inst = await startWesh(['--', 'bash', '--norc', '--noprofile']);
const c = await dialHello(inst.port, {});
process.kill(inst.child.pid, 'SIGTERM');
const proc = await waitExit(inst.child, 15000);
```

```javascript
// phase07.mjs:570-579（S4c）——C1 NEL 控制字符线形构造探针（控制字符剥离回归场景同款）：
// undici latin1 上线——发 JS 'ali\u00C2\u0085ce' 两码点 = UTF-8 客户端发 'ali\u0085ce' 等价线形
const NEL_WIRE = 'ali\u00C2\u0085ce';
```

```javascript
// phase07.mjs:319-321——凭据模式 fetch 断言形态（/metrics 认证闸两态场景同款）
const respA = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
	method: 'POST', headers: { Authorization: basicHeader(CRED1) },
});
```

**JSON 事件行断言新形态（替代 S4 的 550-552 行子串断言）：** `stderrText()` 按行 split → 滤 `{` 起始行 → `JSON.parse` → 按 `event=="x"` 过滤断言字段（与 Go 侧 parseEvents 同构；**禁止子串/正则断言 JSON 行**——CONTEXT Discretion 逐字纪律）。

**UAT 场景矩阵（CONTEXT Discretion 授权面）：** healthz 免认证+状态 JSON 四字段 / metrics 认证闸两态（凭据模式 401/200、--no-auth 直通）/ 根路径固定不受 bp 影响（bp=/wesh 实例下 /healthz 可达、/wesh/healthz 不可达）/ 关停中 503 draining / 审计事件 JSON 行可检索 / 控制字符剥离回归。

---

### 测试断言迁移（5 Go 文件 + 2 UAT 脚本）

**Analog 1（captureStderr 本体——零改动，05-01 同步纪律保持）：** `limits_test.go:91-111`

```go
// limits_test.go:91-111——os.Pipe 置换 + 幂等恢复；迁移后原样保留
func captureStderr(t *testing.T) func() string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	...
	os.Stderr = w
	...
}
```

**Analog 2（同步边——waitHandlers 纪律保持）：** `e2e_test.go:171-194`（startTrackedServerWith）+ `limits_test.go:136-138`

```go
// limits_test.go:136-138——restore() 前的 happens-before 边：等 Attach handler 返回
//（logEvent 在 handler 内先于返回执行，WaitGroup happens-before 使 restore 的写与该读同步）
waitHandlers()
```

**Analog 3（现状断言形态——迁移对象）：** `limits_test.go:144-151`

```go
// 现状：子串断言（迁移后替换为 parseEvents 字段断言）
if n := strings.Count(out, "message_too_big"); n != 1 { ... }
if !strings.Contains(out, "remote=127.0.0.1:") ||
	!strings.Contains(out, "code=1009") ||
	!strings.Contains(out, "reason=message_too_big") { ... }
```

**新 helper（RESEARCH Pattern 5 逐字授权形态，落 limits_test.go 或新 log 测试 helper 文件）：**

```go
// parseEvents 按行解析为事件 map 集——跳过非 '{' 起始行
//（D-16 启动警告行保持文本 + panic 栈等混合流成员，不得因非 JSON 行 FAIL）
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
```

**两个断言陷阱（迁移必查）：** ① JSON 数字解进 `map[string]any` 是 `float64`——`m["code"] == float64(401)` 或写 intOf helper（RESEARCH Pitfall 4）；② 行尾锚定断言（`reason=exit_when_empty\n` 区分 `_wait` 后缀）在 JSON 字段语义下天然消解——`m["event"] == "exit_when_empty"` 精确相等。

**迁移清单（RESEARCH Runtime State Inventory 全量盘点）：**
- Go：`limits_test.go:144-149`、`emptyexit_test.go:275-281`、`auth_e2e_test.go:457-474`、`proxy_e2e_test.go:86-95/325-347`、`multi_test.go:837`
- UAT：`phase05.mjs` S6（`code=1013 && reason=slow_consumer` 行断言 → JSON detach 事件 reason=kick；1013 关闭帧既有断言保持）、`phase07.mjs` S4（550-552/575/594-596 行子串 → JSON 字段解析）
- **凭据红线负断言保持子串形态不变**（auth_e2e_test.go:457-463、proxy_e2e_test.go:344-347——断言「全文不含敏感串」，与 JSON 化正交，逐字保留）
- 零改动：phase02/03/04/06.mjs（不消费事件行，已逐脚本核实）

---

### `README.md`（改）——运维节新增

**Analog：** `README.md:250-`（## 部署与配置（Phase 7）节风格——节首一段总览 + `###` 小节分专题 + 配方代码块）。

风格要点（250-310 行实证）：
- 节标题带 phase 号：`## 运维（Phase 8）`
- 每个特性一个 `###` 小节：`### 健康检查（/healthz）` / `### 指标（/metrics）` / `### 结构化日志`
- 粗体决策点 + 配置配方代码块（toml/ini/nginx 先例——本 phase 为 Prometheus `scrape_config` basic_auth 配方 + journald/jq 检索示例）
- **D-07 明示义务：** /healthz 免认证是整站 Basic 闸唯一例外，必须明示防「例外蔓延」预期
- **Pitfall 6 明示义务：** Prometheus 凭据错误会触发全站节流（429），配方节必须写明

---

## Shared Patterns

### 认证闸装配（D-08 /metrics 跟随；D-07 /healthz 唯一例外）
**Source:** `internal/server/server.go:401-416`
**Apply to:** Handler() 新路由注册
```go
if len(s.credentials) > 0 {
	root := basicAuth(wh, s.credentials, s.throttle, s.proxy)
	...
	attachChain := originMiddleware(basicAuth(http.HandlerFunc(s.attachHandler), s.credentials, s.throttle, s.proxy), s.origins)
```
/metrics 认证模式包 basicAuth（无认证模式直通）；/healthz 两模式均不包。

### 405 fallback 成对注册（方法模式 405 会被 "/" 子树吞掉）
**Source:** `internal/server/sharetoken.go:122-128` + `server.go:423-432`
**Apply to:** /healthz、/metrics 各一对
```go
mux.Handle("GET "+bp+"/s/{token}/", s.sharePage(page, root))
mux.HandleFunc(bp+"/s/{token}/", func(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
})
```
（机制依据：GOROOT server.go:2699-2710 n==nil 分支——内建 405 仅在无任何其他模式匹配时触发；RESEARCH Pitfall 7。）

### 锁序 R-07：hubMu > outbox.mu
**Source:** `internal/server/clients.go:433-434`（afterDrain 注释）+ `451-456`（实现）
**Apply to:** metrics 快照、detach reason 的 hubMu 内读写
```go
// 锁序 R-07：drain 完才取 hubMu（本函数），绝不反序同持；
// hubMu > outboxMu 的同序同持与 onChunk→trySend 同款。
s.hubMu.Lock()
defer s.hubMu.Unlock()
c.outbox.mu.Lock()
cur := c.outbox.bytes
c.outbox.mu.Unlock()
```
hubMu 内 plain int（kicks/gateTransitions/clientsTotal）读取只在持锁快照内；**绝不**为读取把字段改 atomic（为读改写的反向耦合）。

### atomic 计数器/状态位选型（hubMu 外读取才 atomic）
**Source:** `clients.go:250-262`（registry.n 注释论证）、`server.go:98-103`（inputDrops）、`clients.go:197-200`（droppedInputs）
**Apply to:** draining/sessionAlive 两枚新 atomic.Bool、三枚新字节计数器（pty_output/ws_sent/ws_recv）
```go
// server.go:103——热路径无锁递增先例（atomic.Int64；注释登记 Phase 8 OPS-07 挂点）
inputDrops atomic.Int64
```
判定纪律：读取点在 hubMu 外（/healthz、Attach 守卫区③位先例）→ atomic；读写全在 hubMu 内 → plain int。

### logEvent 红线（SEC-01/P5 D-03）
**Source:** `server.go:1063-1067`、`auth.go:89-93`、`sharetoken.go:11-14`
**Apply to:** 全部新事件打点与 metrics series
- 凭据/ticket/token/Authorization 头任何形态（含 base64、含用户名）永不入日志参数
- auth_failed 事件不含用户名（D-23）
- metrics label 零身份面（remote/remote_user/client_id 永不进 label——D-02/D-06，日志红线的 metrics 延伸；per-IP 明细查日志事件）
- build_info 仅 version 单 label（过 escLabel）

### 用户可控字段清洗（D-19 + P7 D-19）
**Source:** `proxy.go:55-67`（sanitizeRemoteUser）
**Apply to:** remote 字段（XFF 链首）推广；remote_user 既有
清洗在提取点完成（单一写口纪律）；C0/C1/DEL 剥离 + 128 rune 截断；JSON 化不消除 C1 穿透（RESEARCH Pitfall 5）——清洗是唯一防线。

### Options 单一通道（配置直传纪律）
**Source:** `server.go:159-234`（Options 注释分档先例）+ `main.go:1134`
**Apply to:** Options.Version 新增
```go
// Options 注释分档形态（server.go:225-231 先例）：生产直传字段写明 main 来源 flag
// 与零值语义；New 内直传或显式兜底（server.go:285-297 先例）
StopSignal  syscall.Signal
StopTimeout time.Duration
```
version 走 `var version = "dev"`（main.go:32）→ `Options.Version` → `s.version` 单通道；发布构建 ldflags 注入属 Phase 9 既定。

### 文件组织纪律（一关注点一文件 + 注释头登记决策号）
**Source:** `proxy.go:3-39`、`origin.go`、`sharetoken.go:3-14`
**Apply to:** metrics.go/health.go/log.go 新文件
包声明后注释头：决策号清单（D-xx）+ 红线声明 + 与既有决策的关系论证；包级纯函数优先。

### 测试同步纪律（05-01）
**Source:** `limits_test.go:91-111`（captureStderr）+ `e2e_test.go:171-194`（startTrackedServerWith waitHandlers）
**Apply to:** 全部新事件断言测试
stderr 断言测试必须：captureStderr 置换 → 驱动事件 → waitHandlers() 建立 happens-before → restore() 读捕获。进程全局替换不并行（无 t.Parallel）。

### UAT 红线与豁免纪律
**Source:** `phase07.mjs:14-25、62-92、755-797`
**Apply to:** phase08.mjs + phase05/07 迁移
token/凭据值只作断言材料（sensitiveTokens 闭包），detail 只打状态码/布尔/形状/退出码；assertOutputClean 运行时自证；平台原生行为 skip+reason 记录；`PHASE08_ONLY` 调试过滤提交形态恒全场景。

## 无完全先例（用 RESEARCH.md 骨架）

以下两块写法代码库无既有 analog——不是「找不到」，而是本 phase 首次引入的机制。以 08-RESEARCH.md 的实证骨架为准（均附 GOROOT/规范 CITED 依据）：

| 文件/写法 | Role | 原因 | 依据 |
|-----------|------|------|------|
| slog JSONHandler + 动态 stderr writer（log.go） | logging | 项目首次引入 slog；动态 writer 是 captureStderr 语义保持的必需形态 | 08-RESEARCH Pattern 1（GOROOT json_handler.go:30-41 构造语义实证） |
| 手写 Prometheus text 0.0.4 exposition（metrics.go） | handler 输出格式 | D-01 裁决手写零依赖，项目首个文本 exposition | 08-RESEARCH Pattern 2（官方 exposition_formats.md 逐字 CITED）+ Series 清单表 |

## Metadata

**Analog search scope:** `internal/server/`（全部 18 个 .go 源文件 + 关键测试）、`cmd/wesh/main.go`、`web/uat/`（phase05/07.mjs）、`README.md`
**Files scanned:** 30+（wc 全量统计）；精读 12 个 analog 文件的目标区段
**Pattern extraction date:** 2026-08-27
