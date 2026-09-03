# Phase 6: 会话生命周期与重连 - Pattern Map

**Mapped:** 2026-08-23
**Files analyzed:** 14（10 修改 + 4 新建）
**Analogs found:** 14 / 14（全部有精确类比；SIGHUP 形态的类比在 git 历史，已逐字核实）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/proto/proto.go`（修改） | config（协议常量+组帧） | request-response | 同文件 `ErrorFrame`/`ErrorPayload`（165-170）+ 常量区（19-34） | exact |
| `internal/proto/proto_test.go`（修改） | test | unit | 同文件 `TestWelcomeFrameErrorFrame`（64-136）+ `TestProtocolConstants` 帧字节表（196-212） | exact |
| `internal/server/server.go`（修改） | service（lifecycle/Options） | event-driven | 同文件 `lifecycle()`（955-995）+ `terminate`（997-1004）+ `New` 零值兜底（199-226） | exact |
| `internal/server/clients.go`（修改） | service（注册表/宽限计时） | event-driven | 同文件 `detach`（677-696）+ `kickSlowConsumerLocked`（491-513）；timer 形态类比 `resize.go initArbiter`（72-80） | exact |
| `internal/pty/signal_linux.go` + `signal_darwin.go`（新建，或并入 io.go） | service（平台信号） | OS 级事件 | `reap_linux.go`/`reap_darwin.go` 构建标签纪律 + `io.go SignalForegroundGroup`（50-61）+ git `cc03c79~1` SIGHUP 形态 | exact |
| `cmd/wesh/main.go`（修改） | config（CLI parse） | batch（启动一次） | 同文件 `parseArgs`（69-204）+ `validateStartup`（290-322）+ `fs.Visit`（159-163） | exact |
| `cmd/wesh/main_test.go`（修改） | test | unit | 同文件 `TestParseArgs`（30-125）+ `TestStartupMatrix`（356-431）+ `TestTLSKeyPairError`（171-199） | exact |
| `internal/server/` 生命周期测试（扩既有，如 e2e_test.go 或新文件） | test | integration | `e2e_test.go startTestServerWith`（121-137，exitf 捕获桩）+ `keepalive_test.go assertNoExit`（61） | exact |
| `web/src/main.ts`（修改） | component（客户端控制器） | event-driven | 同文件 `connect()`（390-757）+ `showStatus`（365-381）+ 帧常量区（20-26） | exact |
| `web/src/lib/reconnect.ts`（新建） | utility（纯函数） | transform | `web/src/lib/prefs.ts`（全文 73 行——零 DOM 纯函数模块形态） | exact |
| `web/src/lib/reconnect.test.ts`（新建） | test | unit | `web/src/lib/prefs.test.ts`（全文 61 行——node --test 直跑 .ts 形态） | exact |
| `web/uat/phase06.mjs`（新建） | test（协议层 UAT） | request-response + event-driven | `web/uat/phase05.mjs`（全文 655 行——harness 五件套用） | exact |
| `web/uat/phase06-dom.mjs`（新建） | test（DOM 层 UAT） | event-driven | `web/uat/phase05-dom.mjs`（loadTerminal 124-151 + SpyWebSocket 注入） | exact |
| `README.md`（修改） | docs | — | 既有生命周期节/flag 表文风（无需代码类比） | n/a |

## Pattern Assignments

### 1. `internal/proto/proto.go` —— EXIT 帧常量 + ExitPayload + ExitFrame()

**Analog:** 同文件 `ErrorFrame`（165-170）与类型字节常量区（19-34）

**类型字节常量区 pattern**（proto.go:19-34，注意 32 行已预留 'X' 占位注释）：
```go
const (
	Input  = '0' // 0x30, C→S, raw bytes → 写 master
	Resize = '1' // 0x31, C→S, JSON {"cols":C,"rows":R} → 钳制 1..1000 后 Setsize
	Output = '0' // 0x30, S→C, master 读块直发

	Hello   = 'H' // 0x48, C→S, JSON {"version":V,"cols":C,"rows":R}
	Welcome = 'W' // 0x57, S→C, JSON {"mode":"ro"|"rw","cols":C,"rows":R,"prefs"?}——...
	Error = 'E' // 0x45, S→C, JSON {"code":C,"message":M}
	// 'X' EXIT / 'T' TITLE / 'P' PREFS —— 类型字节本 phase 占住，语义分属 Phase 6/4（D-01）；
	// ...
)
```
→ EXIT 落法：`Exit = 'X'` 常量补进常量区，同时更新 32 行占位注释（'X' 已兑现）与文件头 6 行「前端手工对齐」注释纪律不变。

**Payload struct pattern**（proto.go:112-117，显式 json tag 防字段名漂移）：
```go
// ErrorPayload 显式 json tag。Code 为 snake_case 机器串，Message 为英文人话
// （前端直接展示，D-07）。
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
```
→ ExitPayload 同构：`ExitCode int \`json:"exit_code"\`` + `Message string \`json:"message"\``（snake_case tag，D-09）。

**组帧函数 pattern**（proto.go:165-170）：
```go
// ErrorFrame 组 Error 帧：1 字节类型 + JSON {"code":C,"message":M}，调用方直接
// c.Write。固定 schema 下 json.Marshal 不会失败。
func ErrorFrame(code, message string) []byte {
	b, _ := json.Marshal(ErrorPayload{Code: code, Message: message})
	return append([]byte{Error}, b...)
}
```
→ `ExitFrame(exitCode int, message string) []byte` 逐字同构（`append([]byte{Exit}, b...)`）。

---

### 2. `internal/proto/proto_test.go` —— EXIT 常量行 + round-trip

**Analog:** 同文件 `TestProtocolConstants`（173-229）与 `TestWelcomeFrameErrorFrame`（64-96）

**帧字节逐字钉死表 pattern**（proto_test.go:196-212）：
```go
	// 帧类型字节逐字断死
	frameBytes := []struct {
		name string
		got  rune
		want rune
	}{
		{"Hello", Hello, 'H'},
		{"Welcome", Welcome, 'W'},
		{"Error", Error, 'E'},
		{"Input", Input, '0'},
		{"Resize", Resize, '1'},
		{"Output", Output, '0'},
	}
```
→ 加一行 `{"Exit", Exit, 'X'}`。

**round-trip 断言 pattern**（proto_test.go:82-96）：
```go
	const msg = "protocol version wesh.v1 required"
	ef := ErrorFrame(ErrVersionMismatch, msg)
	if len(ef) == 0 || ef[0] != Error {
		t.Fatalf("ErrorFrame[0] = %#x, want 'E'(%#x)", ef[0], Error)
	}
	var ep ErrorPayload
	if err := json.Unmarshal(ef[1:], &ep); err != nil {
		t.Fatalf("ErrorFrame payload unmarshal: %v", err)
	}
	if ep.Code != ErrVersionMismatch { ... }
```
→ ExitFrame round-trip 同形态：组帧 → `ef[0] != Exit` 断言 → Unmarshal → ExitCode/Message 逐字段相等。

---

### 3. `internal/server/server.go` —— lifecycle() 插 EXIT 广播 + Options.ExitWhenEmpty

**Analog:** 同文件 `lifecycle()`（955-995）——快照 + 并行 Close 既有形态

**现状 lifecycle 全文**（955-1004，EXIT 广播挂点 = Drain 后、并行 Close 循环内）：
```go
func (s *Server) lifecycle() {
	err := s.sess.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	s.sess.Drain(200 * time.Millisecond)
	close(s.inputDone)
	// 广播 1000：hubMu 下取注册表快照后并行 Close 全部客户端（Close 自带 5s+5s
	// 上界——close.go:87-89，并行等待自然有界）。...
	s.hubMu.Lock()
	clients := make([]*client, 0, len(s.registry.set))
	for c := range s.registry.set {
		clients = append(clients, c)
	}
	s.hubMu.Unlock()
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.conn.Close(websocket.StatusNormalClosure, "")
		}()
	}
	wg.Wait()
	s.hubMu.Lock()
	s.hubCond.Broadcast()
	s.hubMu.Unlock()
	s.terminate(code)
}
```
→ 改法：组帧一次 `exitFrame := proto.ExitFrame(code, msg)`（快照前），每客户端 goroutine 内**先同步 `conn.Write(超时 ctx, MessageBinary, exitFrame)` 再 `Close(1000)`**——写序靠同 goroutine 帧级串行化保证（RESEARCH Pitfall 1：禁止走 outbox.trySend 异步入队）。

**Options 字段 + New 零值兜底 pattern**（server.go:151-173 / 199-226）：
```go
type Options struct {
	Writable         bool
	// ...
	MaxClients       int
	// ...
}
// New 内：
	if opts.MaxClients <= 0 {
		opts.MaxClients = defaultMaxClients
	}
```
→ `ExitWhenEmpty` 字段按同形态加入（注意：grace=0 立即退出是**合法显式值**，不能用 `<=0` 零值兜底——需要 set/grace 分离或指针/布尔对，参照 main.go `writePolicySet` 显式设置判定形态）。

**terminate 收口纪律**（server.go:997-1004，不可动的硬约束）：
```go
func (s *Server) terminate(code int) {
	s.termOnce.Do(func() {
		s.exitf(code)
	})
}
```
→ 宽限到期/once 断开只发 SIGHUP（触发源），**不调 terminate、不加 exitf 分支**——终结仍由 lifecycle 单一路径到达（D-13）。

---

### 4. `internal/server/clients.go` —— 注册表空触发点 + 宽限计时器启停

**Analog A（移除点挂点）:** `detach`（677-696）与 `kickSlowConsumerLocked`（491-513）——注册表移除恰好两调用点，均持 hubMu

**detach 形态**（677-696）：
```go
func (s *Server) detach(c *client) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	if !s.registry.removeLocked(c) {
		return // 已被 kick 移除——close(done)/cancel 恰好一次由成员判定保证
	}
	close(c.done)
	c.cancel()
	s.removeMember(c)
	s.recalcNow()
	if s.registry.owner == c {
		s.promoteNextLocked()
	}
	s.hubCond.Broadcast() // P5-7 统一挂点：detach 后门重估
}
```
→ 空调检测挂在 `removeLocked` 返回 true 之后（`len(s.registry.set)==0` 判定）；kick 路径同点。**事件 = 非空→空迁移**（RESEARCH Pitfall 2：启动期恒空天然免疫，绝不挂轮询）。

**Analog B（取消点挂点）:** `registerLocked`（264-273）/ Attach 升档 hubMu 段（server.go:689-738）——attach 登记后取消 timer。

**Analog C（timer 形态）:** `resize.go initArbiter`（72-80）——AfterFunc + Stop 初始态 + 回调取 hubMu：
```go
func (s *Server) initArbiter() {
	s.arbiter = arbiter{sizes: make(map[*client]dims)}
	s.arbiter.timer = time.AfterFunc(s.resizeDebounce, func() {
		s.hubMu.Lock()
		defer s.hubMu.Unlock()
		s.recalcNow()
	})
	s.arbiter.timer.Stop()
}
```
→ 宽限 timer 同款：字段挂 Server（hubMu 保护），回调内取 hubMu 复查仍空 → 调 SIGHUP 方法（不调 exitf）。timer 启停全部在 hubMu 内，零新锁（R-07 纪律延伸）。

---

### 5. `internal/pty/signal_linux.go` + `signal_darwin.go` —— Session SIGHUP 进程组方法

**Analog A（构建标签纪律）:** `reap_linux.go`（全文 16 行）/ `reap_darwin.go`（1-2 行）
```go
//go:build linux

package pty

import "os/exec"

// Linux 收割 = 每会话一个 goroutine 阻塞在 cmd.Wait()。...
func awaitExit(cmd *exec.Cmd) error {
	return cmd.Wait()
}
```
→ 两平台文件同一签名（`reap_darwin.go:105` 注释：「与 reap_linux.go 签名统一——调用点零平台分支」），SIGHUP 方法同纪律。

**Analog B（SIGHUP 逐字形态）:** git `cc03c79~1:internal/server/server.go:649-657`（本 session 逐字核实）：
```go
// terminate 以 sync.Once 收口两条终结路径，exitf 只触发一次。
// sighup 为真时先 SIGHUP 子进程进程组：负 pid = 进程组；setsid 使子进程为组长，
// pgid = 子进程 pid（D-11）。Start 成功后 Cmd.Process 必非 nil。
func (s *Server) terminate(sighup bool, code int) {
	s.termOnce.Do(func() {
		sess := s.sess
		if sighup {
			syscall.Kill(-sess.Cmd.Process.Pid, syscall.SIGHUP)
		}
		s.exitf(code)
	})
}
```
→ 落 pty.Session 方法（如 `SignalHangup()`），`syscall.Kill(-s.Cmd.Process.Pid, syscall.SIGHUP)` 形态不变；setsid 由 creack/pty StartWithSize 内建（spawn.go:51），pgid == 子进程 pid。

**Analog C（幂等/降级纪律）:** `io.go SignalForegroundGroup`（50-61）：
```go
func (s *Session) SignalForegroundGroup() {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return
	}
	pgid, err := unix.IoctlGetInt(int(s.Master.Fd()), unix.TIOCGPGRP)
	if err != nil || pgid <= 0 {
		return // 静默降级（D-11 授权）
	}
	_ = unix.Kill(-pgid, unix.SIGWINCH) // 负 pid = 进程组；失败静默
}
```
→ SIGHUP 幂等同款（kill 已死 pgid 返回 ESRCH 静默忽略——RESEARCH Pitfall 4）。

---

### 6. `cmd/wesh/main.go` —— --once / --exit-when-empty flag + 校验矩阵

**Analog A（flag 定义）:** `parseArgs`（71-86）——全名无短选项、help 文案单行：
```go
	fs.BoolVar(&cfg.writable, "writable", false, "allow client input (default read-only)")
	fs.DurationVar(&cfg.pingInterval, "ping-interval", 5*time.Second, "WS ping interval (0 = disable)")
	fs.IntVar(&cfg.maxClients, "max-clients", 32, "maximum simultaneous attached clients (default 32)")
```
→ `--once` BoolVar 同款；`--exit-when-empty` 需自定义 `flag.Value` + `IsBoolFlag() bool`（GOROOT flag.go:350-356 惯例，RESEARCH Pattern 5 骨架可直接用）。值非敏感（duration）→ 错误直接 return，**不走 credErr/clientOptErr 记录式**（96-104/126-144 注释明示该形态仅用于值含敏感内容的 flag）。

**Analog B（显式设置判定）:** `fs.Visit`（159-163）：
```go
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "write-policy" {
			cfg.writePolicySet = true
		}
	})
```
→ `--once`/`--max-clients`/`--exit-when-empty` 显式设置判定同形态（语法糖展开 + 冲突校验消费）。

**Analog C（parse 期枚举校验）:** write-policy（180-182）：
```go
	if cfg.writePolicy != server.WritePolicyOwner && cfg.writePolicy != server.WritePolicyAll {
		return cfg, nil, fmt.Errorf("invalid --write-policy %q: must be owner or all", cfg.writePolicy)
	}
```

**Analog D（校验矩阵）:** `validateStartup`（290-322）——纯函数零副作用、组合矛盾 fail-fast、loopback 早退前判定：
```go
	if cfg.writePolicySet && !cfg.writable {
		return "", errors.New("--write-policy is set but --writable is not; write policy only applies when client input is enabled")
	}
	if cfg.maxClients <= 0 {
		return "", errors.New("--max-clients must be positive")
	}
```
→ `--once` × 显式 `--max-clients≠1` / `--exit-when-empty≠0` 冲突行进矩阵同款（双 flag 名进文案，`wantErrSub2` 断言位已备）。

**config struct 扩展点**（28-49）：新字段进 config，注释按 Phase 分组成段（34-48 行既有形态）。

---

### 7. `cmd/wesh/main_test.go` —— TestParseArgs 表行 + TestStartupMatrix 矩阵行

**Analog A:** `TestParseArgs`（30-125）——表驱动 + `t.Setenv("WESH_CREDENTIAL", "")` 表头隔离 + 逐字段 want 断言；零值语义断言先例（89-92：`wantWritePolicy == "" → 期望默认 owner`）。
→ 新行：`--once`（断言展开后 maxClients==1 且 exitWhenEmpty set+grace==0）、`--exit-when-empty` 三形态（不写/裸写/=30s）、非法 duration 进错误表。

**Analog B（错误文案表）:** `TestTLSKeyPairError`（171-199）：
```go
	tests := []struct {
		name         string
		args         []string
		wantSub      string
		forbiddenSub string // 值内容禁入错误串（SEC-01 启动面红线，WR-01；TestClientOptionError 先例）
	}{
		{"malformed write-policy", []string{"--write-policy", "sometimes", "--", "bash"}, "must be owner or all", ""},
	}
```

**Analog C（矩阵行）:** `TestStartupMatrix`（356-431）——既有行基线同步先例（354-355 注释：新增校验后既有行注入 `maxClients: 32` 基值）；双 flag 名断言位 `wantErrSub2`（386 行使用先例）。

---

### 8. `internal/server/` 生命周期/注册表空测试 —— exitf 捕获桩

**Analog:** `e2e_test.go startTestServerWith`（121-137）+ `keepalive_test.go assertNoExit`（61）
```go
func startTestServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string) {
	t.Helper()
	sess, err := pty.Start(argv)
	// ...
	exitCh = make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, opts)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	// ...
	go http.Serve(ln, srv.Handler())
	return exitCh, "ws://" + ln.Addr().String() + "/ws"
}
```
→ 宽限/once 测试：`Options` 注入短 grace（HelloTimeout 先例形态），exitCh 断言**收到**退出码（与 assertNoExit 反向——exitf 必然触发且恰好一次）；服务端注释硬约束「exitf 由 main 注入 os.Exit、测试注入捕获桩——生命周期必须可测」（server.go:32）。

---

### 9. `web/src/main.ts` —— 重连状态机 + EXIT 帧承接

**Analog A（帧常量区）:** main.ts:20-26（与 proto.go 手工对齐，注释互相指路）：
```typescript
const OUTPUT = 0x30,
  INPUT = 0x30,
  RESIZE = 0x31,
  HELLO = 0x48,
  WELCOME = 0x57,
  ERROR = 0x45,
  SUBPROTOCOL = 'wesh.v1';
```
→ 加 `EXIT = 0x58`（proto.go 'X' 对齐，注释更新两侧互指）。

**Analog B（可重入入口 + per-connection 重置块）:** connect()（390-404）：
```typescript
async function connect(): Promise<void> {
  // 每次尝试重置 per-connection 状态——auth_failed 重试不携带上次连接残留
  opened = false;
  helloSent = false;
  lastError = null;
  // isRO/welcomeDone 同属 per-connection（IN-01 防漂移登记，Phase 6 自动重连落地前提）；
  // osc52Loaded/retriedAuth 为页面级门闩，刻意不重置
  isRO = false;
  welcomeDone = false;
  sessionDims = null;
  lastReported = null;
  prevFit = null;
  roNotified = false;
```
→ 重连循环直接 `void connect()` 重入（auth_failed 先例 699 行）；`lastExit` 暂存变量加进重置块。

**Analog C（代际守卫）:** `const sock = ws`（476）：
```typescript
  ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws', [SUBPROTOCOL]);
  const sock = ws; // 闭包内引用本连接的确定句柄（TS 对模块级可空 let 不做闭包收窄）
```
→ 全部 handler 入口加 `if (sock !== ws) return;`（今日恒真，重连落地后成为必需闸——RESEARCH Pitfall 6）。

**Analog D（ERROR 帧暂存通道）:** 643-648：
```typescript
      case ERROR: // D-06/D-07：暂存 {code,message}，onclose 按码分派时展示 message
        try {
          lastError = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
        } catch {
          console.warn('discard malformed ERROR frame');
        }
        break;
```
→ `case EXIT:` 同款暂存 `lastExit`（D-10）；onclose case 1000（707-713）body 改 `lastExit?.message ?? 既有硬编码文案`。

**Analog E（onclose 按码分派 + auth_failed 重试先例）:** 688-754：
```typescript
  sock.onclose = (ev) => {
    window.removeEventListener('beforeunload', onBeforeUnload);
    if (lastError?.code === 'auth_failed' && !retriedAuth) {
      retriedAuth = true;
      lastError = null;
      void connect();
      return;
    }
    if (!opened) { ...return; }
    switch (ev.code) {
      case 1000: ... case 1008: ... case 1009: ... case 1011: ... case 1013: ...
      default: // C-5（含 1002 协议错误与无码异常断开）
```
→ `ev.code === 1006` **显式判定**抽出进重连分支（不用 default 桶——1002 落 default，745 行注释既定）；686-687「1006 永不作为分派依据」注释随实现改写（RESEARCH State of the Art 登记）。

**Analog F（showStatus 三态面板）:** 365-381——hint 链接当前硬编码 "Reload this page"（370-377）；幂等纪律（textContent 先清后建，364 注释）：
```typescript
function showStatus(title: string, body: string, hintPrefix: string): void {
  document.getElementById('status-title')!.textContent = title;
  document.getElementById('status-body')!.textContent = body;
  const hint = document.getElementById('status-hint')!;
  hint.textContent = hintPrefix + ' ';
  const a = document.createElement('a');
  a.href = '';
  a.textContent = 'Reload this page';
  a.addEventListener('click', (e) => {
    e.preventDefault();
    location.reload();
  });
  hint.appendChild(a);
  hint.appendChild(document.createTextNode('.'));
  document.getElementById('status')!.hidden = false;
}
```
→ 参数化动作链接（label + onClick 可选参数，默认保持 Reload 零漂移）——「Reconnect now」落此（RESEARCH OQ2 推荐形态）。

**Analog G（WELCOME 成功点 + beforeunload 重注册）:** 634-637：
```typescript
          welcomeDone = true;
          if (confirmBeforeUnloadOn) {
            window.addEventListener('beforeunload', onBeforeUnload);
          }
```
→ 重连成功判定 = WELCOME 到达：此处退避清零 + `term.clear()` + 面板隐藏。

**Analog H（文案常量化）:** 356-360（UNREACHABLE_BODY/HINT_RESTART 单写口防漂移）→ Reconnecting 面板文案常量化同款。

---

### 10. `web/src/lib/reconnect.ts`（新建）—— backoffMs 纯函数

**Analog:** `web/src/lib/prefs.ts`（全文 73 行）——零 DOM 依赖纯函数模块：
```typescript
// FE-07 偏好白名单与 query/prefs 解析纯函数（node --test 可测——零 DOM 依赖，RESEARCH §A3）。
// ...
export function parseQueryPrefs(search: string): { ... } { ... }
```
→ 文件头注释同款（「node --test 可测——零 DOM 依赖」）；`export function backoffMs(attempt: number): number { return Math.min(1000 * 2 ** attempt, 30000); }`（参数族 1s×2 封顶 30s，throttle.go:12-13 同族）。

---

### 11. `web/src/lib/reconnect.test.ts`（新建）—— node --test 单测

**Analog:** `web/src/lib/prefs.test.ts`（全文 61 行）：
```typescript
// prefs 纯函数回归锁（node --test 直跑 .ts——Node 24 内建 type stripping 零新依赖，RESEARCH §A3；
// 相对导入必须带 .ts 扩展名）。本文件只经 node --test 执行，不参与 tsc（tsconfig exclude）。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseQueryPrefs, splitPrefs, mergeTheme } from './prefs.ts';

test('parseQueryPrefs: 合法白名单键解析并记入 keys', () => {
  const r = parseQueryPrefs('?fontSize=16&cursorBlink=false');
  assert.equal(r.prefs.fontSize, 16);
});
```
→ `import { backoffMs } from './reconnect.ts';`（必须带 .ts 扩展名）；断言序列 1s/2s/4s/8s/16s/30s/30s…（封顶截断）+ attempt 0 起点。

---

### 12. `web/uat/phase06.mjs`（新建）—— 协议层 UAT

**Analog:** `web/uat/phase05.mjs`（全文 655 行）——五件套逐字沿用：

**startWesh**（68-93）：spawn `--bind 127.0.0.1 --port 0` + stdout 三行解析 + 8s 启动超时 + stderr 持续捕获（logEvent 断言通道）+ `kill: () => child.kill('SIGKILL')` 与 `child` 句柄（'exit' 事件断言进程退出——phase06 --once 场景关键）。
**dialHello**（98-117）：WELCOME 轮询决议 + 10s watchdog（IN-04）。
**waitClose**（119-122）：
```javascript
const waitClose = (ws, timeoutMs) => new Promise((resolve) => {
  const to = setTimeout(() => resolve(null), timeoutMs);
  ws.onclose = (ev) => { clearTimeout(to); resolve({ code: ev.code, reason: ev.reason }); };
});
```
**check/skip**（52-60）：PASS/FAIL/SKIP 三态 + skip 记豁免（headless 硬约束条款）。
**场景驱动骨架**（640-655）：scenarios 数组顺序执行 + try/catch 场景异常计数 + 300ms 间隔 + 汇总行 + `process.exit(failedN === 0 && failed === 0 ? 0 : 1)`。

**帧常量区**（27-29）：加 `EXIT = 0x58`。
**红线沿用**（11-13）：token 值永不进 check detail/控制台输出。
**EXIT 断言形态**（RESEARCH Code Examples 已给）：`exitOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === EXIT).subarray(1)))`；帧序断言 = EXIT 必先于 onclose 到达；`sh -c 'exit 42'` / 外部 kill 夹具；--once 场景用 S5 双点位先例（380-407）逐字可抄。

---

### 13. `web/uat/phase06-dom.mjs`（新建）—— jsdom 重连逻辑面

**Analog:** `web/uat/phase05-dom.mjs`（124-151 + 头部注释 1-24）

**loadTerminal 注入形态**（124-151）：
```javascript
async function loadTerminal(srv, opts = {}) {
  const html = readFileSync(DIST, 'utf8');
  const js = /<script type="module" crossorigin>([\s\S]*?)<\/script>/.exec(html)[1];
  const origin = `${srv.scheme}://${srv.host ?? '127.0.0.1'}:${srv.port}`;
  const url = `${origin}${opts.path ?? '/'}`;
  const dom = new JSDOM('', { url, pretendToBeVisual: true, runScripts: 'outside-only' });
  const { window } = dom;
  // SpyWebSocket：记录全部上行帧首字节（INPUT/RESIZE/HELLO 分派断言材料）
  const sentFrames = [];
  window.WebSocket = class extends WebSocket {
    constructor(...a) { super(...a); this.binaryType = 'arraybuffer'; }
    send(data) { ...; return super.send(data); }
  };
  window.fetch = opts.auth ? authFetch : ((u, o) => fetch(new URL(u, origin), o));
  window.TextEncoder = TextEncoder;
  window.TextDecoder = TextDecoder;
  // ...
}
```
→ phase06-dom 延伸点：SpyWebSocket 需能**合成 CloseEvent{code:1006}** 驱动重连循环（RESEARCH A2：若注入受阻可用真实断连 destroy socket 替代）；`window.dispatchEvent(new window.Event('offline'))`/`('online')` 驱动双触发断言；waitFor 轮询断言（48-55）检查 Reconnecting 面板文案/attempt 计数。

---

### 14. `README.md` —— 生命周期节改写 + flag 表两行

无代码类比需求。照既有文风：flag 表逐行「flag 名 + 默认 + 语义」；D-07 重连语义明示段（「重连目标 = 同一 URL 的当前进程，服务端重启后是全新会话」）+ --once 等价关系（`--max-clients=1 --exit-when-empty=0`）+ 无滚动回放明示（ROADMAP 既定分工 tmux/herdr）。

---

## Shared Patterns

### 帧常量三侧手工对齐（P2 D-01/D-16）
**Source:** `internal/proto/proto.go:19-34` ↔ `web/src/main.ts:20-26` ↔ `web/uat/phase05.mjs:28`
**Apply to:** EXIT 'X'/0x58 三侧同步 + 注释互相指路（proto.go:6「前端 web/src/main.ts 的帧常量与本文件手工对齐，两侧注释互相指路」；main.ts:11 同文反向）。
```go
// proto.go:6
// 前端 web/src/main.ts 的帧常量与本文件手工对齐，两侧注释互相指路（D-16）。
```

### exitf + sync.Once 单一收口（D-13 硬约束）
**Source:** `internal/server/server.go:997-1004`
**Apply to:** 全部新终结触发源（--once 断开 / 宽限到期）——只发 SIGHUP，绝不调 terminate/exitf。
```go
func (s *Server) terminate(code int) {
	s.termOnce.Do(func() {
		s.exitf(code)
	})
}
```

### hubMu 单锁 + timer 回调取锁（R-07 延伸）
**Source:** `internal/server/resize.go:6-9`（锁纪律注释）+ 72-80（initArbiter AfterFunc 形态）
**Apply to:** 宽限计时器——启停全在 hubMu 内（detach/kick 移除点启动，registerLocked 后取消），回调自有 goroutine 入内先取 hubMu 复查仍空。

### logEvent 三要素单行（D-12②）
**Source:** `internal/server/server.go:938-940`
**Apply to:** 断开退出触发（once/empty）、宽限计时启动/取消新事件——remote/code/reason 三要素，token/ticket/凭据永不入参。
```go
func logEvent(remote string, code websocket.StatusCode, reason string) {
	fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
}
```

### Options 注入 + 零值兜底（可测性硬约束）
**Source:** `internal/server/server.go:199-226`（New 逐字段兜底）+ `throttle.go:45-53`（newThrottleStore 同款）
**Apply to:** ExitWhenEmpty 装配字段——注意 grace=0 是合法显式值，需 set 位分离（main.go writePolicySet 先例 159-163），不能 `<=0` 兜底吞掉。

### 写序安全：同步 Write 后接 Close（EXIT↔1000 保序）
**Source:** 类比 `kickSlowConsumerLocked` 的「Close 永不内联」（clients.go:475-478 P5-2 注释）的反向应用 + `lifecycle` 快照循环（server.go:974-988）
**Apply to:** lifecycle EXIT 广播——每客户端 goroutine 内**同步** `conn.Write(2s 超时 ctx, exitFrame)` 后接 `conn.Close(1000)`；禁止 outbox.trySend 异步入队（关闭帧超车竞态，RESEARCH Pitfall 1）。

### 前端面板/暂存通道纪律
**Source:** `web/src/main.ts:365-381`（showStatus 幂等 + textContent 无 innerHTML）+ 643-648（暂存 + malformed 容错）
**Apply to:** Reconnecting 面板复用 showStatus（幂等 textContent 先清后建）；EXIT 帧暂存 lastExit 同款 try/catch + console.warn 容错；服务端 message 经 textContent 渲染（无 HTML 注入面）。

### UAT harness 纪律
**Source:** `web/uat/phase05.mjs:11-13`（token 红线）+ 56-60（skip 豁免形态）
**Apply to:** phase06.mjs/phase06-dom.mjs——敏感值只作断言材料永不进 detail；平台原生行为（真实断网栈/权限弹窗）记 `skip(id, name, reason)` 不阻塞。

### CLI flag 纪律（P2 D-15）
**Source:** `cmd/wesh/main.go:71-86`（全名无短选项）+ 290-322（validateStartup 纯函数零副作用、先于 pty.Start/net.Listen、exit 2）
**Apply to:** --once/--exit-when-empty——parse 期校验（值非敏感直接 return error；敏感才用记录式 credErr/clientOptErr 形态）+ 组合冲突进 validateStartup 矩阵。

## No Analog Found

无。全部 14 个文件均有精确类比（含 git 历史逐字核实的 SIGHUP 形态）。

## Metadata

**Analog search scope:** `internal/proto/`、`internal/server/`、`internal/pty/`、`cmd/wesh/`、`web/src/`、`web/src/lib/`、`web/uat/`、git 历史（cc03c79~1）
**Files scanned:** 16（proto.go / proto_test.go / server.go / clients.go / resize.go / throttle.go / io.go / spawn.go / reap_linux.go / reap_darwin.go / main.go / main_test.go / e2e_test.go / keepalive_test.go / main.ts / prefs.ts / prefs.test.ts / phase05.mjs / phase05-dom.mjs 头 150 行 + git show 一段）
**Pattern extraction date:** 2026-08-23
