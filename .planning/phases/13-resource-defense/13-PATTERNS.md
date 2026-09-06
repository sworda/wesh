# Phase 13: resource-defense - Pattern Map

**Mapped:** 2026-09-05
**Files analyzed:** 19（7 产品代码修改 + 1 可能新增 + 9 测试修改/新增 + 2 文档）
**Analogs found:** 19 / 19（本 phase 全部改动为「既定先例的第 N 次同构应用」，无 analog 缺口）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/wesh/main.go`（改） | config/CLI 装配层 | 启动期一次性 batch parse | 自身区段（:244/:314-320/:536-595/:986-1031/:1280-1344） | exact（同文件同机制扩展） |
| `internal/server/perclient.go`（改） | service（会话生命周期） | event-driven（goroutine+hubMu+channel） | 自身（rejectCapacity :133-137 / upgradePerClient :143-271 / sessionWatcher :432-455 / teardownPCLocked :473-504） | exact |
| `internal/server/spawnthrottle.go`（或并入 perclient.go） | service（令牌桶存储） | event-driven（每 attach 判定一次） | `internal/server/throttle.go` 全文 + perclient.go:236 rate 用法 | role-match |
| `internal/server/server.go`（改） | service（core server） | event-driven | 自身（New 分岔 :535-538 / terminate :1548-1552 / Shutdown :1581-1622 / session_end schema :1473-1483） | exact |
| `internal/server/clients.go`（改） | service（注册表/终结触发） | event-driven | 自身（maybeExitWhenEmptyLocked :952-998 / stopChildLocked :1009-1014） | exact |
| `internal/server/metrics.go`（改） | controller（/metrics handler） | request-response | 自身全文（metricsCounters :52-58 / snapshotMetrics :87-110 / metricsHandler :118-147） | exact |
| `internal/server/health.go`（改，或零改动） | controller（/healthz handler） | request-response | 自身全文 54 行 | exact |
| `internal/pty/spawn.go`（改） | utility（env 白名单构造） | transform（纯函数） | 自身 whitelistEnv :115-164 | exact |
| `internal/server/perclient_test.go`（改） | test（外部包 server_test） | event-driven 断言 | 自身（:395-455 断言翻转点 + SpawnFunc 注入夹具） | exact |
| `internal/server/metrics_test.go`（改） | test | request-response 断言 | 自身 metricsSeries17 :40-58 + assertExpositionShape :109-129 | exact |
| `internal/server/emptyexit_test.go`（改） | test | 进程级退出码断言 | 自身 :29-46（accept-255 断言常量形态） | exact |
| `internal/server/shutdown_test.go`（改） | test | event-driven | 自身 startShutdownServerWith :45-61 | exact |
| `internal/server/load_test.go`（改） | test（build tag `load` 隔离） | batch 负载施压 | 自身 :1-80（首行硬纪律 + 夹具层） | exact |
| `internal/server/events_test.go`（改） | test | 事件 schema 断言 | 自身 TestSessionEnd :432-471 | exact |
| `internal/pty/spawn_test.go`（改） | test | transform 断言 | 自身 TestEnvWhitelist :84-124 | exact |
| `cmd/wesh/main_test.go`（改） | test | 纯函数 validateStartup 断言 | 自身 TestValidateStartupWarnMerge :882-903 | exact |
| `cmd/wesh/config_test.go`（改） | test | TOML 解析断言 | main.go 显式位机制 :576-595（测试断言形态同 main_test.go） | role-match |
| `web/uat/phase13.mjs`（新） | test（协议层 UAT，零依赖 Node 脚本） | event-driven（spawn 真实二进制 + WS 客户端） | `web/uat/phase12.mjs` 全文 762 行 | exact |
| `docs/CONFIGURATION.md` + `README.md`（改） | config 文档 | — | 现有行（CONFIGURATION.md:77/:160） | exact |

## Pattern Assignments

### 1. `cmd/wesh/main.go` — stop-timeout 双默认值 + 显式位 + warn（D-01/D-02）

**Analog:** 自身四区段（全部本会话 Read 逐字核实）

**默认值来源模式**（:244 + :314-320——双默认值判定必须在 TOML 合并与 fs.Visit 之后）:
```go
stopTimeoutDefault := time.Duration(0)   // :244 —— shared 字面 0 逐字不动（零回归红线）
// ...
if fc.StopTimeout != nil {                // :314-320 TOML 合并先例
	d, perr := time.ParseDuration(*fc.StopTimeout)
	if perr != nil {
		return cfg, nil, configErr(configPath, "invalid duration", `key "stop-timeout"`)
	}
	stopTimeoutDefault = d
}
```

**显式设置位模式**（:536-567 fs.Visit 块新增第八位 + :576-595 配置键存在即置位块追加——两处同档置位）:
```go
fs.Visit(func(f *flag.Flag) {
	// ...既有七位逐字不动...
	if f.Name == "stop-timeout" {   // 本 phase 新增第八位
		cfg.stopTimeoutSet = true
	}
})
// :576-595 块内追加：
if fc.StopTimeout != nil {
	cfg.stopTimeoutSet = true
}
```

**warn 通道模式**（:986/:1004-1019 modeWarns 累积形态——10-02 先例同构，新 warn  append 进同一 modeWarns 即自动获得全透出点合并能力）:
```go
var modeWarns []string
if cfg.writePolicySet && cfg.sessionMode == server.SessionModePerClient {
	modeWarns = append(modeWarns, "wesh: warning: --write-policy has no effect with ...")
}
// mergeWarn 闭包既有（:1011-1019）——新 warn 只需 append，零透出点改动。
// 红线（:985 注释）：warn/err 文案不得含凭据值。
```

**终值落定 + spawnFunc 闭包**（:1280-1287 warn 消费点 / :1327-1333 spawnFunc——WESH_REMOTE_USER 传递链扩展点）:
```go
if warn != "" {
	fmt.Fprintln(os.Stderr, warn)
}
// ...
startOpts := pty.StartOptions{Dir: cfg.cwd, Term: cfg.term, Uid: cfg.uid, Gid: cfg.gid}
var spawnFunc func(cols, rows int) (*pty.Session, error)
if cfg.sessionMode == server.SessionModePerClient {
	spawnFunc = func(cols, rows int) (*pty.Session, error) {
		return pty.StartWithSize(argv, startOpts, cols, rows)
	}
}
// D-01 终值落定插点：Options 装配前、sessionMode 与 stopTimeoutSet 均在手处：
//   if cfg.sessionMode == server.SessionModePerClient && !cfg.stopTimeoutSet {
//       cfg.stopTimeout = 5 * time.Second
//   }
// SEC-09 扩展点：闭包签名扩展携带 remoteUser（精确形态属 Claude's Discretion）。
```

---

### 2. `internal/server/perclient.go` — 双令牌桶挂点 + session_start/end + 计数器递增 + WR-02 栅栏

**Analog:** 自身（同一文件内四处机制先例直接复用）

**拒绝序列模式**（:133-137 rejectCapacity——D-04 节流拒绝的逐字母本，同码同串不同事件名）:
```go
func rejectCapacity(ctx context.Context, c *websocket.Conn, remote, remoteUser string) {
	_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrServerError, capacityMessage))
	logEvent(remote, websocket.StatusInternalError, "max_clients", remoteUser)
	_ = c.Close(websocket.StatusInternalError, proto.ErrServerError)
}
// spawn_throttled 复刻本三段序列：Error 帧（同 capacityMessage 或定值节流文案）
// → logEvent(..., "spawn_throttled", ...) → Close(1011) → s.mc.ptySpawnThrottled.Add(1)。
```

**闸挂点模式**（:162-168 pre-spawn 容量再闸——双令牌桶判定插在本闸之前/同位；hubMu 短临界区只读计数，绝不持锁 spawn）:
```go
s.hubMu.Lock()
full := len(s.pcSessions) >= s.maxClients
s.hubMu.Unlock()
if full {
	rejectCapacity(ctx, c, remote, remoteUser)
	return nil
}
// 桶判定是纯内存操作（Allow() 无阻塞），可放同位；spawnFunc 调用点 :174 位置不动。
// 桶键来源：s.proxy.clientIP(r)（D-05）——upgradePerClient 现状签名无 *http.Request，
// 键需在 Attach 上游（请求在手处）提取后随参数传入（RESEARCH A4，实现细节选型）。
```

**rate.NewLimiter 既有用法**（:236——每客户端输入限速器，同库零新依赖）:
```go
limiter: rate.NewLimiter(rate.Limit(s.inputRate), s.inputBurst),
```

**sessionWatcher 挂点模式**（:432-455——session_end emit 插点 = 退出码提取后、close(waitDone) 前后均可但须在 EXIT 组帧前；pcLastExitCode/pcHasExitCode 记录点 = hubMu 内置位区段；收割点补 hubCond.Broadcast() 唤醒 pcSupervisor）:
```go
err := pc.sess.Wait()
code := 0
var ee *exec.ExitError
if errors.As(err, &ee) {
	code = ee.ExitCode()
}
close(pc.waitDone)          // WR-02 修法基石：已在 reap 完成点，无需移动
s.hubMu.Lock()
pc.exitCode = code
pc.reaped = true
// 本 phase 新增：s.pcLastExitCode/s.pcHasExitCode 记录 + session_end emit
//   （schema 复制 server.go:1473-1483 + slog.Int64("client_id", cl.attachSeq)）
//   + s.hubCond.Broadcast()（pcSupervisor 唤醒）
s.teardownPCLocked(pc)
s.hubMu.Unlock()
```

**KILL 兜底分支模式**（:481-491 teardownPCLocked 快半段——D-01 默认 5s 的机制消费点，本 phase 零改动；wesh_pty_kills_total 递增点 = 复检通过后 SignalGroup 之前）:
```go
if !pc.reaped {
	pc.sess.SignalGroup(s.stopSignal)
	if s.stopTimeout > 0 {
		time.AfterFunc(s.stopTimeout, func() {
			s.hubMu.Lock()
			defer s.hubMu.Unlock()
			if !pc.reaped {
				pc.sess.SignalGroup(syscall.SIGKILL)
			}
		})
	}
}
```

**WR-02 修复点**（teardown 快半段对 `pc.waitDone` 做非阻塞 select——已关闭即视为 reaped 跳过信号；结构性栅栏零新同步件，STATE.md 登记修法）。

---

### 3. `internal/server/server.go` — pcSupervisor 钉死 + Shutdown N 组快照 + 新字段

**Analog:** 自身三区段

**New per-client 分支模式**（:535-538——pcSupervisor 钉死点；现状零全局 goroutine）:
```go
if s.sessionMode == SessionModePerClient {
	s.pcSessions = make(map[*pcSession]struct{})
	// 本 phase 新增：spawn 双令牌桶字段初始化 + go s.pcSupervisor()
	return s
}
```

**terminate 唯一收口件**（:1548-1552——pcSupervisor 逐字复用，「exitf 恰好一次」零漂移）:
```go
func (s *Server) terminate(code int) {
	s.termOnce.Do(func() {
		s.exitf(code)
	})
}
```

**session_end shared schema 母本**（:1473-1483——per-client session_end 复制本形态 + client_id 键；KILL 兜底经 signal 字段归因不另起事件）:
```go
endAttrs := []slog.Attr{
	slog.String("event", "session_end"),
	slog.Int("exit_code", code),
	slog.Float64("duration_seconds", time.Since(s.startedAt).Seconds()), // per-client 换 pc.startedAt
}
if sig, ok := exitSignalNum(err); ok {
	if name, ok := signalName(syscall.Signal(sig)); ok {
		endAttrs = append(endAttrs, slog.String("signal", name))
	}
}
emitEvent(endAttrs...)
```

**Shutdown N 组填充点**（:1614-1621 空 if 体——快照源必须是 pcSessions（含待收割残留者，Pitfall 6），不是 registry；快照在 hubMu 内取、信号在 hubMu 外发；shared else 分支逐字不动）:
```go
if s.sessionMode == SessionModePerClient {
	// 本 phase 填充：hubMu 内 pcs 快照 → 放锁 → 逐组 SignalGroup(s.stopSignal)
	//   + stopTimeout>0 时 AfterFunc 补 KILL（teardownPCLocked :484-491 同形态）
	//   → 有界 join（上界 = stopTimeout + 余量，Claude's Discretion 定值）
	//   → hubCond.Broadcast() 补行唤醒 pcSupervisor
} else {
	s.sess.SignalGroup(s.stopSignal)   // 现状逐字不动
	if s.stopTimeout > 0 {
		time.Sleep(s.stopTimeout)
		s.sess.SignalGroup(syscall.SIGKILL)
	}
}
```

**新字段落点**（Server struct :188-192 pcSessions 注释块之后——pcExitReq/pcLastExitCode/pcHasExitCode 三字段 hubMu 保护，注释形态沿用「hubMu 保护（pinger pongTimedOut 置位取 hubMu 写、detach 同锁读先例）」论证风格）。

---

### 4. `internal/server/clients.go` — 第二终结源触发端（maybeExitWhenEmptyLocked 分支替换）

**Analog:** 自身 :952-1014

**早退守卫替换点 + 三形态机械**（:957-959 守卫 + :964-997 立即/宽限两形态——per-client 分支复用同一 exitEmptyTimer 机械，三形态分支表逐行对照防 Pitfall 1）:
```go
// 现状（替换对象）:
if s.sessionMode == SessionModePerClient {
	return
}
// 替换后 per-client 分支：四守卫沿用（!exitWhenEmpty/exiting/非空/门闩）→
//   grace==0 立即形态：s.pcExitReq = true; s.hubCond.Broadcast()（不发信号——
//     末端断开的 teardown 已 SIGHUP 其会话）
//   grace>0 宽限到期回调内（身份比对+复查通过后，:990-994 位置）：同两行动作
//   宽限取消（cancelExitEmptyTimerLocked :1022-1029）：零改动
```

**stop-signal 序列统一出口母本**（:1009-1014 stopChildLocked——Shutdown N 组每组执行的形态来源；AfterFunc 异步补 KILL 不 sleep 阻塞 hubMu）:
```go
func (s *Server) stopChildLocked() {
	s.sess.SignalGroup(s.stopSignal)
	if s.stopTimeout > 0 {
		time.AfterFunc(s.stopTimeout, func() { s.sess.SignalGroup(syscall.SIGKILL) })
	}
}
```

---

### 5. `internal/server/metrics.go` — 四计数器 + session_active 模式分支 + HELP 双模式（D-07/D-08）

**Analog:** 自身全文（17 series 现状即扩展母本）

**计数器集扩展模式**（:52-58——四枚 atomic.Int64 追加，场景化选型注释先例保持）:
```go
type metricsCounters struct {
	authFailed     atomic.Int64
	authThrottled  atomic.Int64
	ptyOutputBytes atomic.Int64
	wsSentBytes    atomic.Int64
	wsRecvBytes    atomic.Int64
	// 本 phase 追加：ptySpawn / ptySpawnFailures / ptyKills / ptySpawnThrottled
}
```

**快照单趟模式**（:87-110——metricsSnap 新增 pcSessions 字段在 hubMu 单趟持有内填齐，Open Question 1 推荐形态；绝不第二趟取锁）:
```go
func (s *Server) snapshotMetrics() metricsSnap {
	var sn metricsSnap
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	sn.clientsConnected = s.registry.n.Load()
	// ...
	// 本 phase：sn.pcSessions = int64(len(s.pcSessions))（per-client 注册表计数）
	return sn
}
```

**series 输出 + session_active 分支点**（:118-147 + :125-130——D-07 同名 series 按模式分支取值；HELP 文案按模式生成；四计数器尾部追加 writeCounter 三行组）:
```go
// D-05：共享进程模型下会话数恒 1 退化——session_active gauge 落探活语义。
var sessionActive int64
if s.sessionAlive.Load() {
	sessionActive = 1
}
writeGauge(&b, "wesh_session_active", "Whether the PTY session is alive (1) or exited (0).", sessionActive)
// 本 phase 分支：per-client → sessionActive = snap.pcSessions，HELP 文案按模式分支。
// 四新 series shared 恒 0 出数不摘（credit_gate 恒 0 先例）。
```

**label 红线**（:15-19 注释逐字保持并扩到新 series）：全部 series 零身份 label（remote/remote_user/client_id/ticket 永不进 label）；per-client 明细一律查审计日志。

---

### 6. `internal/server/health.go` — session_active 恒 true（D-06）

**Analog:** 自身 :29-54

**分支点**（:44——最简形态二选一，Claude's Discretion）:
```go
SessionActive: s.sessionAlive.Load(),
// 形态①：handler 内模式分支 per-client → true；
// 形态②（倾向）：New per-client 分支（server.go:535-538）置 sessionAlive.Store(true)
//   ——per-client 下 lifecycle 不启动，:1487 Store(false) 永不到达，handler 零分支。
// 四字段键集红线（:35-45 匿名 struct status/clients/max_clients/session_active）不变。
```

---

### 7. `internal/pty/spawn.go` — whitelistEnv 扩展 WESH_REMOTE_USER（SEC-09）

**Analog:** 自身 :115-164

**白名单构造模式**（替换式注入纪律 :111 逐字保持——严禁 os.Environ() 全量追加；倾向选项 1 = 形参扩展，空串不出键）:
```go
func whitelistEnv(term string, uid int) []string {
	// ...
	env := []string{
		"TERM=" + term,
		"COLORTERM=truecolor",
	}
	// 本 phase 扩展（选项 1）：加 remoteUser string 形参，非空时 append
	//   "WESH_REMOTE_USER=" + remoteUser；值 = cl.remoteUser（提取点
	//   proxy.go:136-141 已 sanitize 清洗产物，下游零二次清洗）。
	// shared 路径调用点（:81 cmd.Env = whitelistEnv(opts.Term, opts.Uid)）
	// 传空串零漂移——键名白名单固定语义由 pty 包单侧定义。
	return env
}
```

---

### 8. `internal/server/spawnthrottle.go`（或并入 perclient.go）— per-IP 桶存储

**Analog:** `internal/server/throttle.go` 全文 108 行（mu+map+惰性过期最小形态）

**存储形态模式**（:36-53 结构 + 构造零值兜底）:
```go
type throttleStore struct {
	mu   sync.Mutex
	m    map[string]throttleEntry
	base time.Duration // 默认 1s；测试经 newThrottleStore 参数覆写提速
	cap  time.Duration
}

func newThrottleStore(base, cap time.Duration) *throttleStore {
	if base <= 0 {
		base = defaultThrottleBase
	}
	// ...
	return &throttleStore{m: make(map[string]throttleEntry), base: base, cap: cap}
}
```

**惰性过期模式**（:70-85 recordFail 内 15min 重置——无常驻 janitor goroutine「零新 exitf 分支」纪律；桶条目 = *rate.Limiter + lastSeen）:
```go
func (t *throttleStore) recordFail(ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.m[ip]
	if now.Sub(e.lastSeen) > 15*time.Minute {
		e = throttleEntry{} // 惰性过期重置（map 上界纪律）
	}
	// ...
}
```

**内部常量 + Options 测试覆写形态**（:11-14 + :39——D-03 双桶常量 defaultSpawnGlobalRate=8/burst=16/perIP=1/burst=4 同形态声明；Options 覆写仅供测试注入，12-03 defaultSlowDwell 先例）。

**桶键来源**: `proxy.go:89-103 clientIP`——trust 且 XFF 非空 → 链首 IP（ParseIP 校验闸）→ 回退 TCP 对端 host；**同源调用，禁止新写 XFF 解析**（键分叉 = 同 IP 两配额）。

---

### 9. 测试文件组（九件）

| 测试文件 | Analog 与关键摘录 |
|---------|-------------------|
| `perclient_test.go` | **断言翻转点** :420-422 逐字：`if body.SessionActive { t.Fatal("...Phase 13 OQ①② 裁决落地时本断言随之翻转") }`——本 phase 翻转为恒 true。**SpawnFunc 注入夹具** :441-452（atomic 失败开关 + startPerClientServerWithSpawn）——双桶单元测同形态注入覆写桶参数。 |
| `metrics_test.go` | **镜像契约** :40-58 metricsSeries17（17 条 name/typ 对）+ :115-116 行数闸 `len(lines) != 3*len(metricsSeries17)`——17→21 一次性扩展，新四条追加尾部；HELP 双模式文案断言随模式分支新增。 |
| `emptyexit_test.go` | **accept-255 门** :9-13 头注释 + :29-46 形态：exitf 捕获桩收 `-1` 恰好一次（Go 内断言常量 -1；真实二进制 255 由 UAT 进程级断言承接）；per-client 断言分叉走「两时序逐位对齐证明」（子先死 exit 0 / 客户端先断 255），**禁止断言放宽成「两模式都接受」**。 |
| `shutdown_test.go` | **夹具** :45-61 startShutdownServerWith（返回 srv 句柄直接调 Shutdown；pty.Start → New → Listen → Cleanup → Serve 序列）；TERM 忽略形态复用 stopseq 夹具（trap 安装与关停信号竞态经落盘标记文件同步）。per-client Shutdown N 组测需补「断开未收割残留会话」形态（Pitfall 6）。 |
| `load_test.go` | **首行硬纪律** :1 `//go:build load` 必须首行 + package server_test 外部包；**夹具层** :51-80（drainClient Read 永不带 deadline ctx、select time.After 竞速收口）；churn 负载格断言形态 = 起点/终点双采样 + 差值上界 + 多次轮询（Pitfall 7——禁单个硬编码绝对上限）；观测钩子 = /metrics 黑盒 scrape（getMetrics/metricSample 先例）+ /proc/self/fd。 |
| `events_test.go` | **schema 断言母本** :432-471 TestSessionEnd：captureStderr → parseEvents → eventsNamed 计数锁 → 键存在/缺席双向断言（「不应出 signal 键（非信号死亡）」形态）；per-client 扩展组同形态 + client_id 关联键断言（:420-422 attach/detach client_id 关联先例）。 |
| `spawn_test.go` | **TestEnvWhitelist** :84-124 双层形态：(a) 单元层 whitelistEnv 返回值断言（泄露键零出现 + 固定键在场合）+ (b) e2e 层 `/usr/bin/env` 子进程真实输出 + 阳性对照（防空串假绿）；WESH_REMOTE_USER 扩展同形态双断言（per-client 注入可见 / 空串不出键）。 |
| `main_test.go` | **warn 断言形态** :882-903 TestValidateStartupWarnMerge：直调 validateStartup 纯函数 + `strings.Contains(warn, sub)` 子串断言 + socket 早退透出锁；stop-timeout=0 warn 同形态新增 t.Run 分支。 |
| `config_test.go` | 显式位双源置位（CLI fs.Visit + TOML 键存在）测试断言——形态同 main_test.go 矩阵测（config struct 直构 + parseArgs 断言 stopTimeoutSet）。 |

---

### 10. `web/uat/phase13.mjs` — 协议层 UAT（新增）

**Analog:** `web/uat/phase12.mjs` 全文 762 行（同构母本——phase11.mjs 先例的第三代）

**文件头纪律模式**（:1-28——红线声明 + 时序纪律 + 运行方式注释三段式，逐形态复刻）:
```js
// Phase 12 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch/child_process/net）。
// ...
// 红线（phase11.mjs:27-30 纪律逐字沿用）：token/凭据/pid 数值只作断言材料，永不
// 进入 check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/文案常量
// 时序纪律：宽限/退避/免疫/dwell 类场景真实等待，超时上限只做护栏，禁精确时点断言；
// 时钟不 mock、服务端等待不缩短。
```

**check/skip 记录器模式**（:55-68——emittedDetails 收集供 assertOutputClean 自净）:
```js
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
const skip = (id, name, reason) => { /* ok: null，CODEBUDDY.md §5 显式豁免形态 */ };
```

**startWesh 实例夹具**（:94-121——spawn 真实二进制 + stdout 两行解析 + 8s 启动超时 + kill 恒 SIGKILL + redactArgs 脱敏；phase13.mjs 逐字复用）。

**场景函数模式**（:676-726 s6DwellKick1013——「console.log 场景头 → startWesh → try{check 序列}catch/finally{夹具收口}」；KILL 兜底 trap '' HUP 场景与 churn 节流场景同构）:
```js
async function s6DwellKick1013() {
  console.log('S6: 真实 dwell 到期 1013（...）');
  const inst = await startWesh(['--session-mode=per-client', '--writable', '--ping-interval=0', '--', 'sh']);
  // ...check('S6b', '...', ok, `detail 只含布尔/计数/码值`)
  } finally {
    if (pid !== null) { try { process.kill(-pid, 'SIGKILL'); } catch { /* 已消亡 */ } }
    a.destroy();
    inst.kill();
  }
}
```

**收口模式**（:733-762——assertOutputClean 自净 + 场景串行 + 退出码）:
```js
const scenarios = [s1..., s2..., /* ... */];
let failed = 0;
for (const s of scenarios) {
  try { await s(); } catch (e) { failed++; emittedDetails.push(String(e.message)); /* ... */ }
  await sleep(300);
}
assertOutputClean();
// ...
process.exit(failedN === 0 && failed === 0 ? 0 : 1);
```

**可复用夹具**: `RawStallClient`（phase12.mjs:221 raw net.Socket 手工 WS 客户端——churn 场景合法票据高频重连与停读夹具直接复用/裁剪）。

---

### 11. 文档（`docs/CONFIGURATION.md` + `README.md`）

**Analog:** 现有行（CONFIGURATION.md:77 TOML 键表 / :160 flag 表——双处 `stop-timeout` 行同步改默认值列 + per-client 双默认值语义注记；1006 先杀时序明示段随 D-10 落 README/CONFIGURATION 相应段——文档即被测物纪律，精确措辞属 Claude's Discretion）。

---

## Shared Patterns

### 审计事件出口（全部新事件唯一通道）
**Source:** `internal/server/log.go:65-103`
**Apply to:** session_start / session_end / spawn_throttled 全部新事件
```go
func emitEvent(attrs ...slog.Attr) {
	eventLog.LogAttrs(context.Background(), slog.LevelInfo, "event", attrs...)
}

func logEvent(remote string, code websocket.StatusCode, reason string, remoteUser ...string) {
	attrs := []slog.Attr{
		slog.String("event", reason),
		slog.String("remote", remote),
		slog.Int("code", int(code)),
	}
	if len(remoteUser) > 0 && remoteUser[0] != "" {
		attrs = append(attrs, slog.String("remote_user", remoteUser[0]))
	}
	emitEvent(attrs...)
}
```
**红线（:85-89 注释逐字保持）:** 凭据/ticket/Authorization 任何形态（含 base64）禁止入参；spawn_failed/spawn_throttled 定值常量文案，绝不携带 err.Error()/路径/errno。

### wire 聚合、日志细分（第三次应用）
**Source:** `internal/server/perclient.go:133-137`（rejectCapacity）+ :183-186（spawn 失败）
**Apply to:** spawn_throttled 拒绝路径
同一 `Error{server_error, 定值文案}` + `Close(1011)` wire 面三拒绝不可区分是有意为之；分辨率全部由 logEvent 事件名（max_clients / spawn_failed / spawn_throttled）承担。关闭码纪律：`websocket.StatusInternalError` 常量（禁魔数 1011）；1011 不在前端 shouldReconnect 触发集（main.ts:1023 仅 1006），无重连放大循环。

### hubMu 单锁纪律 + 锁序
**Source:** `internal/server/server.go:83-90`（hubMu/hubCond 注释）+ metrics.go:78-86（R-07 锁序论证）
**Apply to:** pcSupervisor / sessionWatcher 记录点 / Shutdown 快照 / snapshotMetrics
hubCond 既有（server.go:525 `sync.NewCond(&s.hubMu)`）——pcSupervisor 等待挂点零新同步件；快照在 hubMu 内取、信号/spawn/阻塞等待一律 hubMu 外；锁序 hubMu > outbox.mu / hubMu > sess.fdMu 全序不动。

### 内部常量 + Options 测试覆写（零公开契约面）
**Source:** throttle.go:11-14 + :39（newThrottleStore 参数覆写注释「测试经 newThrottleStore 参数覆写提速（HelloTimeout 先例）」）
**Apply to:** 双令牌桶四常量（defaultSpawnGlobalRate=8 / defaultSpawnGlobalBurst=16 / defaultSpawnPerIPRate=1 / defaultSpawnPerIPBurst=4，D-03）
不暴露 CLI flag/TOML 键；Options 覆写仅供测试注入；零值兜底纪律（`if base <= 0 { base = defaultThrottleBase }` 形态）。

### 显式设置位双源置位（CLI + TOML 同档）
**Source:** `cmd/wesh/main.go:536-567`（fs.Visit）+ :576-595（fc 键非 nil 置位）
**Apply to:** stopTimeoutSet 第八位
「未设置」vs「显式 0」区分是 D-01/D-02 的机制基座；判定锚定显式位而非终值（review #3 吸收先例——自证性更强）。

### 写一次字段 + happens-before 论证注释
**Source:** perclient.go:55-61（pcSession 写一次纪律注释）+ :227（`remoteUser: remoteUser, // 07-03：Attach 入口提取一次，此后只读`）
**Apply to:** 新 Server 字段（pcExitReq/pcLastExitCode/pcHasExitCode）与 spawnFunc 闭包扩展签名
新字段注释须载明锁保护归属（hubMu 保护）与同步边论证（goroutine 启动 + hubMu 建立 happens-before）——本仓注释即契约的风格纪律。

### 零回归双证据口径（收口闸）
**Source:** 13-CONTEXT.md 继承决策 + RESEARCH §Validation Architecture
**Apply to:** 全部改动
shared 全量 Go 测试原样绿 + phase02-12 UAT 默认模式零修改重跑 + 期望值逐字未动 diff 审查；**禁止断言放宽成「两模式都接受」**；全部新行为断言走 per-client 分支。per-client-only 新 metrics series 在 shared 恒 0 出数不摘（credit_gate 恒 0 先例）。

## No Analog Found

无。本 phase 全部 19 件文件均在仓内找到 exact/role-match analog——研究结论「无新机制发明，全部是既定先例的第 N 次同构应用」经本会话逐文件核实成立。唯一「新建」件 `web/uat/phase13.mjs` 有 phase12.mjs 逐段同构母本；可能新增件 `internal/server/spawnthrottle.go` 有 throttle.go 全文母本。

## Metadata

**Analog search scope:** `internal/server/`（perclient.go / server.go / clients.go / metrics.go / health.go / throttle.go / proxy.go / log.go + 六测试文件）、`internal/pty/`（spawn.go / spawn_test.go）、`cmd/wesh/`（main.go / main_test.go）、`web/uat/phase12.mjs`、`docs/CONFIGURATION.md`、`README.md`
**Files scanned:** 20（全文读 8 件：throttle/metrics/health/log/spawn/proxy/perclient.go/phase12.mjs 区段；目标区段读 12 件）
**Pattern extraction date:** 2026-09-05
