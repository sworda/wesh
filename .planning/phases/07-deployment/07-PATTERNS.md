# Phase 7: 部署与配置 - Pattern Map

**Mapped:** 2026-08-25
**Files analyzed:** 12（3 新建 + 9 修改）
**Analogs found:** 11 / 12（config.go 的 TOML 加载面无仓内先例，用 RESEARCH Pattern 4 配方 + 仓内纪律类比）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/wesh/config.go` 【新】 | config loader | file-I/O + transform | `cmd/wesh/main.go`（credErr 记录式）+ `internal/server/origin.go`（parse 期校验） | partial（RESEARCH Pattern 4 配方承载主体形态） |
| `internal/server/proxy.go` 【新】 | middleware/utility | request-response transform | `internal/server/headers.go` + `internal/server/origin.go` | role-match（中间件+纯函数同位文件组织先例） |
| `web/uat/phase07.mjs` 【新】 | test (UAT harness) | request-response + event-driven | `web/uat/phase06.mjs` | exact（同款零依赖协议脚本 harness） |
| `cmd/wesh/main.go` | entrypoint/controller | request-response（启动装配） | 自身（config struct/fs.Visit/validateStartup/run 分段） | exact（扩展既有分段） |
| `internal/pty/spawn.go` | service（进程 spawn） | process spawn（event source） | 自身（Start/whitelistEnv） | exact（预留挂点兑现） |
| `internal/pty/signal_linux.go` | utility（平台信号） | event-driven（OS signal） | 自身 SignalHangup | exact（泛化为 SignalGroup） |
| `internal/pty/signal_darwin.go` | utility（平台信号） | event-driven | `signal_linux.go`（平台对件） | exact（同签名纪律） |
| `internal/server/server.go` | controller/service | request-response + event-driven | 自身（clientIP/logEvent/lifecycle/Handler） | exact（扩展点逐行锚定） |
| `internal/server/sharetoken.go` | controller（路由装配） | request-response | 自身 registerShareRoutes | exact（base-path 前缀注入点） |
| `web/src/main.ts` | component（前端主件） | event-driven | 自身（share 正则/fetch/WS URL/onclose switch） | exact（三处改造点逐行锚定） |
| `internal/proto/proto.go` | config（协议常量） | — | 自身（关闭码纪律注释块） | exact（1001 占位翻正） |
| `cmd/wesh/main_test.go` | test（表驱动） | — | 自身（TestParseArgs/TestStartupMatrix） | exact（表结构沿用，03-04 先例） |

## Pattern Assignments

### `cmd/wesh/config.go` 【新】（config loader，file-I/O + transform）

**Analog:** 仓内无 TOML 加载先例——主体形态用 RESEARCH.md Pattern 4（fileConfig 指针标量 struct + `DisallowUnknownFields` 严格模式 + 两阶段合并算法）；**纪律类比**从下列仓内先例复制：

**敏感值记录式错误上报**（`cmd/wesh/main.go:143-152`）：
```go
var credErr error
fs.Func("credential", "...", func(s string) error {
	c, err := server.ParseCredential(s)
	if err != nil {
		credErr = errors.New("invalid --credential: credential must be user:pass") // 只含错误类别，禁含值（SEC-01）
		return nil
	}
	cfg.credentials = append(cfg.credentials, c)
	return nil
})
```
配置文件里 credential 等敏感值校验错误走同款纪律：先记录、parseArgs 统一上报点（main.go:247-254 的 showVersion 早退之后插入位）上报，错误文案 = 类别 + 键名 + 行号三要素，禁含值（RESEARCH Pitfall 5）。

**parse 期规范化+校验形态**（`internal/server/origin.go:28-48`，NormalizeOrigin 先例）：
```go
func NormalizeOrigin(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("origin must be scheme://host[:port]: %q", s)
	}
	// ...逐项拒绝 + 规范化...
	return u.Scheme + "://" + host, nil
}
```
`loadFileConfig` 同款形态：纯函数 `(path string) (*fileConfig, error)`，错误即拒（D-06 exit 2 fail-fast），无副作用。

**D-07 权限检查**：`os.Stat(path)` → `info.Mode().Perm()`；含 credential 键且 perm 非 0600/0400 → stderr 警告放行（警告串形态照 validateStartup 的 `wesh: warning:` 前缀先例，main.go:403,409），不含凭据值。

---

### `internal/server/proxy.go` 【新】（middleware/utility，request-response transform）

**Analog:** `internal/server/headers.go`（中间件形态）+ `internal/server/origin.go`（同位文件组织：包级纯函数 + middleware + 注释头登记决策依据）

**文件组织形态**（`internal/server/origin.go:1-9, 84-96`）：
```go
package server

import (
	"fmt"
	"net"
	"net/http"
	// ...
)

// originMiddleware 是 /api/attach 守卫链的 Origin 闸 ...
func originMiddleware(next http.Handler, allowed map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !originAllowed(r, allowed) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```
proxy.go 同款：包级 `sanitizeRemoteUser` 纯函数 + 提取 helper；注释头登记 D-15..D-20 决策依据与红线（token 永不入参）。

**remote_user sanitize**（D-19）——`web/src/lib/title.ts:10-18` 同款纪律的 Go 移植（RESEARCH Pattern 5 给出逐字 Go 形态）：
```typescript
// title.ts 先例（纪律来源，Go 侧按 rune 迭代等价）：
const stripped = Array.from(raw).filter((ch) => {
  const cp = ch.codePointAt(0)!;
  return !(cp <= 0x1f || cp === 0x7f || (cp >= 0x80 && cp <= 0x9f) || ...);
});
return stripped.slice(0, 128).join('') || 'wesh';
```
Go 侧按 RESEARCH Pattern 5 的 `sanitizeRemoteUser`：C0/DEL/C1 剥离 + 128 rune 截断（无空串回退——缺席即不出键，与标题纪律差异点）。

**clientIP XFF 改造**（D-20）——现状函数（`internal/server/server.go:519-529`）：
```go
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```
改造为 Server 方法，`--auth-header` 给定 = 信任闸总开关，XFF 取链首 IP + TrimSpace，非法/缺席回退现状（RESEARCH Pattern 5 逐字形态）；调用点 Attach（server.go:577 `ip := clientIP(r)`）与 remote 取值点（server.go:587）同换。

---

### `web/uat/phase07.mjs` 【新】（test，request-response + event-driven）

**Analog:** `web/uat/phase06.mjs`（exact——同款零依赖 Node 原生 WS/fetch 协议层 harness，逐结构复制）

**文件头纪律**（phase06.mjs:1-19）：覆盖范围段 + 红线段（token/凭据只作断言材料永不进 check detail）+ 运行方式行。

**核心 harness 五件**（phase06.mjs:51-198，全部照搬）：
```javascript
const check = (id, name, ok, detail = '') => { ... };   // PASS/FAIL 收集
const skip = (id, name, reason) => { ... };              // 平台豁免不计失败
function startWesh(args) { ... }   // spawn --bind 127.0.0.1 --port 0，stdout 三行解析，
                                   // redactArgs 脱敏（--credential 值 → <redacted>），
                                   // token 留 sensitiveTokens 闭包数组
function dialHello(port, { ticket, cols, rows } = {}) { ... }  // WS + Hello + Welcome 等待 + 10s watchdog
function waitExit(child, timeoutMs) { ... }  // child 'exit' 决议 {code, signal} 恒带超时护栏
function collectUntilClose(ws, timeoutMs = 10000) { ... }      // 帧收集器 + close {code,reason}
function rawUpgrade(port, headers) { ... }   // 手构 WS Upgrade 请求 resolve HTTP 状态码
```

**收尾纪律**（phase06.mjs:483-509）：
```javascript
function assertOutputClean() {  // 遍历 emittedDetails 断言零凭据/token/'/s/' 串（红线运行时自证）
	const leaked = emittedDetails.some((d) =>
		d.includes(UAT_CREDENTIAL) || d.includes('/s/') || sensitiveTokens.some((t) => t !== null && d.includes(t)));
	check('SEC', "输出自净：...", !leaked, `details=${emittedDetails.length} 命中=${leaked}`);
}
// 场景数组循环 + 场景异常消息纳入 emittedDetails + 汇总行 + process.exit(failedN===0 && failed===0 ? 0 : 1)
```

**本 phase 新增夹具**（RESEARCH Pattern 7 探针实证，unix socket 场景）：
```javascript
import net from 'node:net';
const relay = net.createServer((c) => {
  const u = net.createConnection(SOCK);
  c.pipe(u).pipe(c);
});
relay.listen(0, '127.0.0.1', () => { /* 原生 WebSocket/fetch 连 127.0.0.1:port，既有断言零改动 */ });
```
场景矩阵（CONTEXT discretion）：配置文件合并 / unix socket 全链（经 relay）/ base-path 页面+WS 升级 / auth-header 记录 / XFF / stop-signal 宽限 / 降权 / 1001 关停序列（SIGTERM → 客户端收 1001 → waitExit）。xdg-open 场景用 fake xdg-open（PATH 前置）断言 argv，真实弹浏览器列 skip+reason。

---

### `cmd/wesh/main.go`（entrypoint，启动装配扩展）

**Analog:** 自身——全部 13 个新 flag 的宿主分段已成型，逐段复制：

**config struct 扩展形态**（main.go:28-54，含分组注释纪律）：
```go
type config struct {
	// Phase 5 写权限体系（D-05，one-way 公开契约，P2 D-15 同纪律）：
	writePolicy    string
	writePolicySet bool   // fs.Visit 显式设置位（validateStartup 组合校验消费）
	// ...
}
```
新字段按 Phase 7 分组（D-08 socket 组 / D-13 base-path / D-18 auth-header / D-21..D-22 子进程 / D-24 uid/gid / D-26 open），每组带决策号注释；新增需显式设置位的 flag 补 `xxxSet bool` 字段。

**flag 注册 + fs.Visit 显式设置位**（main.go:118-134, 216-228）：
```go
fs.StringVar(&cfg.writePolicy, "write-policy", server.WritePolicyOwner, "...")
// ...Parse 之后：
fs.Visit(func(f *flag.Flag) {
	if f.Name == "write-policy" { cfg.writePolicySet = true }
	if f.Name == "max-clients" { cfg.maxClientsSet = true }
	if f.Name == "exit-when-empty" { cfg.exitEmptySet = true }
})
```
D-02 合并语义承载：配置文件合并后、CLI 显式设置位判定照旧经 fs.Visit（装配顺序 = --config 预扫 → TOML 铺底 → flag 注册解析 → Visit，planner discretion）。

**可选值 flag 自定义 Value 先例**（exitEmptyValue，main.go:68-102）：`String()/IsBoolFlag()/Set()` 三方法形态；`--stop-timeout` 若取自定义值形态照此（CONTEXT discretion）。

**validateStartup 校验矩阵扩展**（main.go:368-412）：
```go
func validateStartup(cfg config) (warn string, err error) {
	// 纯配置矛盾先行（loopback 早退之前判定——write-policy 行同位先例）：
	if cfg.writePolicySet && !cfg.writable {
		return "", errors.New("--write-policy is set but --writable is not; ...")
	}
	if isLoopbackBind(cfg.bind) {
		return "", nil // loopback：流量不出机，有无凭据/TLS 均放行免警告（D-03/D-05）
	}
	// ...
	return "wesh: warning: listening on non-loopback address with NO authentication (--no-auth); ...", nil
}
```
新落点：--socket×--port/--bind 互斥（D-08）、--socket-mode/--socket-owner 单给（D-09）、--uid/--gid 单给（D-24）、--socket×--open（OQ1 建议行）全部在 loopback 早退**之前**（纯配置矛盾同位纪律）；D-11 unix socket 跳过 bind 矩阵 = loopback 早退同款形态加分支；D-16 auth-header 暴露面警告走 warn 返回通道（`wesh: warning:` 前缀先例）。

**run() net.Listen 分岔点**（main.go:459-466，含失败回滚先例）：
```go
ln, err := net.Listen("tcp", net.JoinHostPort(cfg.bind, strconv.Itoa(cfg.port)))
if err != nil {
	// 启动失败路径回滚已 spawn 资源：Close master 后子进程（setsid 组长）收 SIGHUP 退出
	_ = sess.Close()
	fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
	return 1
}
```
unix 分岔（D-08/D-10/D-09）：`_ = os.Remove(sockPath)` → `net.Listen("unix", sockPath)` → `os.Chmod` → `os.Chown`，每步失败 `_ = ln.Close()`（UnixListener 默认 unlink-on-close 回滚零残留）+ `return 1` 同档。

**启动打印 + 分享链接**（main.go:483-505）：unix 形态 `listening on unix:///path`（D-12）；`ln.Addr().(*net.TCPAddr)` 断言（main.go:499-501）在 unix 形态天然不拼 TCP 端口，保留该防御；分享链接拼串点统一注入 base-path（D-14），--open 消费同一 URL（单一事实源，RESEARCH Pattern 8）。--open 调用 = 启动打印后 goroutine，headless 检测（无 DISPLAY/WAYLAND_DISPLAY）stderr 提示跳过不阻断（D-27）。

**SIGTERM/SIGINT 捕获挂点**（RESEARCH Pattern 7）：`signal.NotifyContext` + goroutine 内 `srv.Shutdown(...)`——**不调 exitf**（P1 硬约束：exitf + sync.Once 单一收口由 lifecycle 子进程路径承载，只加触发源）。

---

### `internal/pty/spawn.go`（service，Start 选项化 + 降权）

**Analog:** 自身——Start（spawn.go:43-58）与 whitelistEnv（spawn.go:63-85），注释预留位逐字兑现：

**现状 Start**（spawn.go:43-58）：
```go
func Start(argv []string) (*Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("pty: empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...) // exec 数组，绝不经 shell
	cmd.Env = whitelistEnv()                  // SEC-06：替换式注入，非追加
	// 不设 cmd.Stdin/Stdout/Stderr（...）与
	// cmd.Dir（Phase 1 继承服务端 cwd；OPS-04 可配留 Phase 7）。   ← D-21 预留注释
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: SpawnRows, Cols: SpawnCols})
	// ...
}
```
改造形态（RESEARCH Pattern 6）：`StartOptions{Dir, Term string; Uid, Gid int}`，`cmd.Dir = opts.Dir`（空串 = 继承，exec.Cmd 零值语义），`opts.Uid >= 0` 时 `cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(...), Gid: uint32(...)}}`——creack/pty StartWithSize 只补 Setsid/Setctty 不覆盖（GOMODCACHE start.go:18-25 核实兼容）。

**whitelistEnv 身份改写挂点**（spawn.go:63-85）：
```go
func whitelistEnv() []string {
	env := []string{
		"TERM=xterm-256color", // ← D-21 --term 落点（参数化）
		"COLORTERM=truecolor",
	}
	for _, k := range []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL"} {  // ← D-25 改写挂点
		if v, ok := os.LookupEnv(k); ok && v != "" {
			env = append(env, k+"="+v)
		}
	}
	// ...
}
```
D-25：uid>=0 时 `user.LookupId` 查 passwd 改写 HOME/USER/LOGNAME 三键，查不到剔除三键；**替换式注入纪律不变——严禁 os.Environ() 追加**（SEC-06 红线，spawn.go:62 注释）。

---

### `internal/pty/signal_linux.go` + `signal_darwin.go`（utility，信号泛化）

**Analog:** 自身 SignalHangup（signal_linux.go:15-17 逐字）：
```go
// SignalHangup 向子进程进程组发 SIGHUP（...）：负 pid = 进程组；setsid ... pgid == 子进程 pid。
// 错误全部静默（ESRCH 幂等——已死进程组重复发送无害 ...）。不触 Master fd 故不取 fdMu ...
// 与 signal_darwin.go 同签名——调用点零平台分支（reap_* 同款纪律）。
func (s *Session) SignalHangup() {
	_ = syscall.Kill(-s.Cmd.Process.Pid, syscall.SIGHUP)
}
```
泛化形态：`SignalGroup(sig syscall.Signal)` 同注释纪律同 ESRCH 静默；两平台文件同签名（调用点零平台分支纪律）；信号名→常量映射（--stop-signal HUP|TERM|INT|KILL parse 期枚举校验）集中平台文件——注意 server.go:997-1027 signalName 是 signal→name 方向，此处需 name→signal 反向表，不复用错方向（RESEARCH Pattern 6 明示）。

---

### `internal/server/server.go`（controller/service，四扩展点）

**Analog:** 自身——四扩展点逐行锚定：

**① Options/New 装配直传形态**（server.go:172-196, 222-280）：
```go
type Options struct {
	// ...生产直传字段（main.go flag 原样透传）与测试可覆写字段（零值取默认常量）分档注释
}
func New(sess *pty.Session, exitf func(int), opts Options) *Server {
	// 零值兜底先例：
	if opts.WritePolicy == "" {
		opts.WritePolicy = WritePolicyOwner // D-05 安全默认
	}
	// ...
}
```
新增 `AuthHeader string`（生产直传）；base-path 若入 Options 同通道。

**② Handler() mux 装配 + base-path 前缀**（server.go:332-392 现状 + RESEARCH Pattern 2 配方）：
```go
// 现状装配形态（扩展点逐字）：
mux.Handle("/", root)
s.registerShareRoutes(mux, wh, root)
mux.Handle("POST /api/attach", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if s.shareAttach(w, r) == shareHandled { return }
	attachChain.ServeHTTP(w, r)
}))
mux.HandleFunc("/api/attach", func(w http.ResponseWriter, _ *http.Request) { /* 405 fallback */ })
mux.HandleFunc("/ws", s.Attach)
return securityHeaders(mux, s.tlsOn)
```
改造 = 注册点统一拼 bp 前缀 + StripPrefix 仅包静态伺服（wh 是唯一路径敏感 handler——sharePage 自改写 `r.URL.Path = "/"` 前缀无关，sharetoken.go:87-96）；注册 `bp+"/"` 子树得 mux 内建 307 尾斜杠规范化免费（GOROOT 核实）；405 fallback 四条同带前缀（RESEARCH Pitfall 4 单侧定义防线）。

**③ logEvent 三要素扩字段**（server.go:975-977 现状）：
```go
func logEvent(remote string, code websocket.StatusCode, reason string) {
	fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
}
```
remote_user 作第四字段追加（` remote_user=alice`，缺席不出键）；**红线保持：token/ticket/凭据永不入参**（server.go:970-974 注释逐字纪律）；sanitize 在提取点（proxy.go）完成，logEvent 不做二次清洗。

**④ Shutdown（1001 广播）——lifecycle EXIT 帧先例**（server.go:1090-1124 逐字形态）：
```go
s.hubMu.Lock()
s.exiting = true                     // 空触发抑制门复用（lifecycle:1096 先例）
clients := make([]*client, 0, len(s.registry.set))  // hubMu 下快照
for c := range s.registry.set { clients = append(clients, c) }
s.hubMu.Unlock()
var wg sync.WaitGroup
for _, c := range clients {
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = c.conn.Close(websocket.StatusGoingAway, "server_shutting_down") // Close 内建 5s+5s 上界
	}()
}
wg.Wait()
s.sess.SignalGroup(sig)              // D-22 stop-signal 序列
if timeout > 0 { /* sleep → SignalGroup(SIGKILL) ESRCH 幂等 */ }
```
差别点（RESEARCH Pattern 7）：1001 无 EXIT 帧前置（进程未退出，终结语义由关闭码承载）；Shutdown 是**触发源非 exitf 分支**——返回后子进程死亡 → lifecycle EXIT+1000 广播在空注册表上零循环 → terminate 收口。

---

### `internal/server/sharetoken.go`（controller，base-path 前缀注入）

**Analog:** 自身 registerShareRoutes（sharetoken.go:116-122）：
```go
func (s *Server) registerShareRoutes(mux *http.ServeMux, page, root http.Handler) {
	mux.Handle("GET /s/{token}/", s.sharePage(page, root))
	mux.HandleFunc("/s/{token}/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})
}
```
改造 = 两注册模式串加 bp 前缀参数（sharePage 路径无关零改动——自改写 "/" 语义，sharetoken.go:87-96）；GOROOT 通配语义三坑注释（sharetoken.go:106-115）保持。

---

### `web/src/main.ts`（component，相对 URL 三改 + 1001 分派）

**Analog:** 自身——改造点逐行锚定（RESEARCH Pattern 3 完整配方）：

**① share 正则不锚 ^ 兼作挂载点检测**（main.ts:500 现状）：
```typescript
const shareMatch = location.pathname.match(/^\/s\/([^/]+)\/$/);  // 现状
// → const shareMatch = location.pathname.match(/\/s\/([^/]+)\/$/);  // 不锚 ^
const up = shareMatch ? '../../' : '';  // 升级前缀：share 页深两段
```

**② fetch 相对路径**（main.ts:509-510 现状 `'/api/attach'`）：
```typescript
const resp = await fetch(up + 'api/attach', shareToken === undefined ? { method: 'POST' } : { ... });
```

**③ WS URL 构造**（main.ts:601 现状）：
```typescript
// 现状：ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws', [SUBPROTOCOL]);
const wsUrl = new URL(up + 'ws', location.href);
wsUrl.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';  // 必换——URL 继承 http(s) scheme
ws = new WebSocket(wsUrl, [SUBPROTOCOL]);
```
**Anti-pattern 防线**（RESEARCH Pitfall 3，本 phase 最高回归风险）：只去前导斜杠不做升级前缀 → share 页 `fetch('api/attach')` 解析为 `/s/{token}/api/attach` → 401 → 分享链接全灭；UAT 必含 share token × base-path 交叉场景。

**④ onclose 1001 case**（main.ts:873-929 switch 加 case）：
```typescript
switch (ev.code) {
	case 1000: showStatus('Session ended', ...); break;
	// 新增（D-23：1001 不在 CORE-05 重连触发集——仅 1006，main.ts:914-918 现状）：
	case 1001:
		showStatus('Server shutting down', '...', 'Start wesh again from your shell, then');
		break;
	case 1006: startReconnect(); break;
	// ...
}
```
showStatus 三态面板复用零新 UI 组件（main.ts:451-471）；`reconnecting && ev.code === 1006` 分支（main.ts:862-865）对 1001 自然落 stopReconnect + 面板分派，零冲突。dist 重建：`time pnpm -C web build`。

---

### `internal/proto/proto.go`（config，1001 注释翻正）

**Analog:** 自身关闭码纪律块（proto.go:8-14 逐字）：
```go
// 关闭码纪律（D-05 全集 {1000,1001,1002,1008,1009,1011,1013}）：
// ...
// 1013 背压踢出已于 Phase 5 启用（D-08 占位兑现——发送路径 = server/clients.go
// kickSlowConsumerLocked，库常量 websocket.StatusTryAgainLater，close reason 机器串
// slow_consumer）；1001 优雅下线 Phase 7 启用，本期占位不实现；   ← 本行翻正为启用态
// 1005/1006/1015 永不发送（库层 validWireCloseCode 兜底）；禁止自定义 4000 段私码。
```
1013 占位兑现的翻正形态即先例：注明启用 phase + 发送路径指针（server.go Shutdown）+ 库常量（`websocket.StatusGoingAway`）+ close reason 机器串（`server_shutting_down`）；main.ts 注释互相指路纪律（proto.go:6 既定）同步。

---

### `cmd/wesh/main_test.go`（test，表驱动扩展）

**Analog:** 自身 TestParseArgs（main_test.go:32-85+）与 TestStartupMatrix（main_test.go:402-484）：

**表行形态**（03-04 先例：命名字段转换，既有行零改动）：
```go
{name: "defaults", args: []string{"--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
```

**TestStartupMatrix 三通道断言形态**（main_test.go:408-484）：
```go
tests := []struct {
	name        string
	cfg         config
	wantErrSub  string // 非空 = 拒绝启动，文案须含此子串
	wantErrSub2 string // 组合校验双 flag 名断言
	wantWarnSub string // 逃生门警告须含 flag 名
}{
	{"non-loopback no creds refused", config{bind: "0.0.0.0", maxClients: 32}, "refusing to listen on non-loopback ...", "", ""},
	// ...
}
// 行尾统一红线断言（main_test.go:478-481）：warn/err 文案不得含凭据值
```
新组合校验行（--socket×--port / --socket-owner 单给 / --uid 单给 / --socket×--open）同款表行追加；配置文件合并用例入 TestParseArgs 表（t.TempDir 落 TOML 文件）；TestStartupRefusalNoResource（main_test.go:490+，拒绝路径零资源占用）同款纪律覆盖新拒绝路径。

---

## Shared Patterns

### 敏感值两通道记录式错误上报（SEC-01 启动面红线）
**Source:** `cmd/wesh/main.go:143-152, 174-192, 247-254`
**Apply to:** config.go（credential 键校验错误）、main.go 新 flag 中含敏感值者
```go
var credErr error
fs.Func("credential", "...", func(s string) error {
	c, err := server.ParseCredential(s)
	if err != nil {
		credErr = errors.New("invalid --credential: credential must be user:pass") // 只含类别，禁含值
		return nil
	}
	// ...
	return nil
})
// Parse 返回处统一上报（showVersion 早退之后，03-04 先例插入位）
if credErr != nil { return cfg, nil, credErr }
```

### fs.Visit 显式设置位（D-02/D-05 合并宿主）
**Source:** `cmd/wesh/main.go:216-228`
**Apply to:** main.go 全部需「CLI 显式设置则覆盖配置文件」判定的 flag
```go
fs.Visit(func(f *flag.Flag) {
	if f.Name == "write-policy" { cfg.writePolicySet = true }
	// ...三先例形态逐字
})
```

### validateStartup 纯函数校验矩阵（分层纪律：parse = 形状，validate = 组合矛盾）
**Source:** `cmd/wesh/main.go:368-412`
**Apply to:** 全部新组合矛盾（--socket×--port / owner 单给 / uid 单给 / socket×open / auth-header 暴露面警告）
```go
// 纯配置矛盾在 loopback 早退之前判定（write-policy 行同位）；warn/err 文案不得含凭据值；
// 警告形态：return "wesh: warning: ...", nil（逃生门 flag 名进文案）
```

### exitf + sync.Once 单一终结收口（P1 硬约束）
**Source:** `internal/server/server.go:1129-1133` + `cmd/wesh/main.go`（os.Exit 注入点 476）
**Apply to:** Shutdown（1001+stop-signal 序列只加触发源不加 exitf 分支）、SIGTERM/INT 捕获
```go
func (s *Server) terminate(code int) {
	s.termOnce.Do(func() {
		s.exitf(code)
	})
}
```

### 进程组信号负 pid + ESRCH 幂等静默（平台对件同签名）
**Source:** `internal/pty/signal_linux.go:15-17`（darwin 对件 signal_darwin.go:16-18）
**Apply to:** SignalGroup 泛化、stop-signal 序列、KILL 补发
```go
_ = syscall.Kill(-s.Cmd.Process.Pid, sig)  // 负 pid = 进程组；setsid 使 pgid == pid；错误全静默
```

### logEvent 三要素单行 + token 永不入参红线
**Source:** `internal/server/server.go:959-977`
**Apply to:** remote_user 第四字段、XFF remote 取值、全部新记录点
```go
fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
// 红线：凭据/ticket/Authorization 头任何形态禁作参数；remote_user 须先经 sanitize（D-19）
```

### 前端帧常量/关闭码前后端手工对齐
**Source:** `internal/proto/proto.go:6,8-14` ↔ `web/src/main.ts:12-30`
**Apply to:** 1001 启用（proto.go 注释翻正 + main.ts onclose case 同批，两侧注释互指）

### UAT 红线运行时自净 + 平台豁免形态
**Source:** `web/uat/phase06.mjs:54-64, 483-509`
**Apply to:** phase07.mjs
```javascript
const skip = (id, name, reason) => { results.push({ id, name, ok: null }); ... };  // 豁免不计失败
// assertOutputClean：遍历 emittedDetails 断言零凭据/token 值（含场景异常通道）
```

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/wesh/config.go`（TOML 加载主体） | config loader | file-I/O | 仓内无配置文件加载先例——`fileConfig` 指针标量 struct + `toml.NewDecoder(f).DisallowUnknownFields().Decode(&fc)` 严格模式 + 两阶段合并算法用 RESEARCH.md Pattern 4 配方（go-toml v2.4.3 官方 API，CITED pkg.go.dev）；纪律面（敏感值上报/parse 期校验/警告形态）由上方 Shared Patterns 逐字承载 |

## Metadata

**Analog search scope:** `cmd/wesh/`、`internal/server/`、`internal/pty/`、`internal/proto/`、`web/src/`、`web/uat/`、`web/embed.go`
**Files scanned:** 12（main.go、main_test.go、spawn.go、signal_linux.go、signal_darwin.go、proto.go、server.go、headers.go、origin.go、sharetoken.go、throttle.go、embed.go、main.ts、lib/title.ts、phase06.mjs 全量/分段精读）
**Pattern extraction date:** 2026-08-25
