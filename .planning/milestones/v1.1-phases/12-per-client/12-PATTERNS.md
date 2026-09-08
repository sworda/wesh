# Phase 12: per-client 交互与背压语义 - Pattern Map

**Mapped:** 2026-09-04
**Files analyzed:** 9（7 修改/扩展 + 2 新增）
**Analogs found:** 9 / 9（全部命中；阻塞持帧恢复信号为「组合形态」，见 No Analog 节注记）

本 phase 的特殊性：几乎全部改动面的最近 analog 就是**目标文件自身**（CONTEXT.md 的 `<code_context>` 已带 file:line 实证）。本图把每条 analog 的精确摘录收敛到一处，planner 可直接拷入 plan 的 action 段。

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/proto/proto.go`（改：WelcomePayload +session） | protocol/model | serialization | 同文件 Cols/Rows G-05-1 恒序列化先例（:111-116） | exact |
| `internal/server/server.go`（改：RESIZE case per-client 直通分支） | request handler（读循环 case） | request-response | 同文件 RESIZE case 现状（:1181-1194）+ 升档分岔（:1026-1031） | exact |
| `internal/server/perclient.go`（改：输出闭包阻塞持帧 + dwell + Welcome 组帧） | service（会话生命周期） | streaming（PTY→WS 输出泵） | 同文件现有闭包（:253-268）+ clients.go 信用门 Wait（:420-422） | exact |
| `internal/server/clients.go`（改：outbox 恢复信号挂点 + gateTransitions 两递增点） | service（hub/outbox 机械） | streaming | 同文件 afterDrain/writer（:543-570, :738-761） | exact |
| `internal/server/resize.go`（改：debouncer 抽取共用件） | utility（防抖组件） | event-driven（timer） | 同文件 arbiter timer 形态（:59-94） | exact |
| `web/src/main.ts`（改：Welcome 分支 reset + 模式位 + ro RESIZE 第一闸放开） | component（前端主控） | event-driven（WS 消息分派） | 同文件 WELCOME 分支/sessionDims 容错/connect() 重置面 | exact |
| `internal/server/perclient_test.go`（扩：8-12 新测） | test（Go，server_test 包） | request-response | 同文件 harness（:56-102）+ slowclient_test.go stall 夹具 | exact |
| `web/uat/phase12.mjs`（新） | test（协议层 UAT） | request-response（真实二进制） | `web/uat/phase11.mjs`（同构母本，逐字可抄骨架） | exact |
| `web/uat/phase12-dom.mjs`（新，jsdom；文件名归 planner 定） | test（前端 DOM 逻辑面） | event-driven（jsdom + SpyWebSocket） | `web/uat/phase06-dom.mjs`（D1/D8 先例） | exact |

**只读参照（不修改）：** `internal/pty/io.go`（P5-1 别名红线与 fdMu 纪律的纪律源）、`cmd/wesh/main.go`（零改动——`--session-mode` flag 已存在，dwell 不暴露 flag/TOML，D-03）。dist 重建与 embed 链为 pnpm build 机械步骤（Claude's Discretion 已载）。

## Pattern Assignments

### 1. `internal/proto/proto.go` — WelcomePayload 加 session 字段（D-08）

**Analog:** 同文件 G-05-1 Cols/Rows 恒序列化先例（:104-116）+ WelcomeFrame 组帧（:146-157）

**现状结构**（proto.go:100-116）——session 字段按 Cols/Rows 同款形态插入：**显式 json tag、刻意不加 omitempty、注释载「缺席 = 旧服务端」识别契约**：

```go
// Cols/Rows 为当前会话尺寸（G-05-1，2026-08-22 用户裁决方向 A）：刻意不加
// omitempty——会话尺寸恒在（含零参与者期间的 80x24 spawn 回落），新前端靠
// 「缺席 = 旧服务端」识别遗留形态。…
type WelcomePayload struct {
	Mode  string          `json:"mode"`
	Cols  int             `json:"cols"` // G-05-1：会话 cols（恒序列化，无 omitempty）
	Rows  int             `json:"rows"` // G-05-1：会话 rows（恒序列化，无 omitempty）
	Prefs json.RawMessage `json:"prefs,omitempty"`
}
```

**组帧函数**（proto.go:154-157）——加参形态的直接母本；`json.Marshal` 固定 schema 不失败纪律保持：

```go
func WelcomeFrame(mode string, prefs json.RawMessage, cols, rows int) []byte {
	b, _ := json.Marshal(WelcomePayload{Mode: mode, Cols: cols, Rows: rows, Prefs: prefs})
	return append([]byte{Welcome}, b...)
}
```

**取值常量同词自描述**（D-08：与 CLI flag `--session-mode` 同词）：复用 `clients.go:88-92` 既有常量，**不在 proto 包新增字符串**——

```go
const (
	SessionModeShared = "shared" // 默认（REQUIREMENTS 反特性 A5）
	SessionModePerClient = "per-client"
)
```

**Planner 注意 — WelcomeFrame 全部 5 调用点**（改签名需同步）：`perclient.go:217`（per-client attach Welcome）、`server.go:1078`（shared attach Welcome）、`clients.go:680`（promoteNextLocked 升格 Welcome）、`clients.go:568`（afterDrain WR-02 补发）、`resize.go:172`（pushSessionDimsLocked 运行期推送）。shared-only 路径（后 4 处中的 clients/resize 三处）在 per-client 运行期结构性不可达，但编译面必须同参。帧常量注释互指纪律：proto.go:6「前端 web/src/main.ts 的帧常量与本文件手工对齐，两侧注释互相指路（D-16）」——main.ts:27-33 的 Welcome 注释块需同步加 session 键互指。

---

### 2. `internal/server/server.go` — RESIZE case per-client 直通分支（D-06，§3.4 参考实现）

**Analog:** 同文件 RESIZE case 现状（:1181-1194）+ Attach 升档模式分岔形态（:1026-1031）

**RESIZE case 现状逐字**（server.go:1181-1194）——per-client 直通分支的插入点即 D-09 第二闸之前/之中；**shared 分支逐字保留，ro 丢弃闸 (:1184) 只在 shared 生效**：

```go
	case proto.Resize:
		// D-09 第二闸：ro 端 RESIZE 服务端直接忽略（『P2 D-13 ro 放行 RESIZE 为单
		// 客户端语境，已被 D-09 修订』逐字登记；第一闸 = 前端 ro 不发，05-08 落地）。
		if cl == nil || cl.mode.Load() == proto.ModeRO {
			continue // 旁观者永不影响可写端 PTY 尺寸；cl == nil 为握手违规落入路径，同形静默丢
		}
		// JSON 解码失败静默丢弃（不关连接）；成功时已钳制 [1,1000]（D-16）。
		// rw 端上报入 arbiter（hubMu 内：sizes 更新 + 50ms 防抖 reset——
		// 到期重算，目标尺寸变化才 sess.Resize，resize.go reportResize）。
		if cols, rows, ok := proto.DecodeResize(data[1:]); ok {
			s.hubMu.Lock()
			s.reportResize(cl, cols, rows)
			s.hubMu.Unlock()
		}
```

**运行期零分岔的既有分岔形态**（server.go:1026-1031）——`cl.pc != nil` 判定与「装配期一次分岔」哲学的衔接先例（11-01 Pattern 2 间接字段注释见 clients.go:151-162：INPUT case 经 `cl.inQ` 逐行不分支；RESIZE 直通因锁序差异**必须**分支，`cl.pc != nil` 为门——detach/kick 的 teardown 触发同门先例，clients.go:619-621/846-848）：

```go
	if s.sessionMode == SessionModePerClient {
		cl = s.upgradePerClient(ctx, c, remote, remoteUser, h, mode, cancel)
		if cl == nil {
			return
		}
	} else {
```

**直通目标机械**（pty/io.go:34-41）——`sess.Resize` 仅取 `fdMu`、**不持 hubMu**（锁序三规则 §5；resize.go:8-9「锁序 hubMu > sess.fdMu」同序），closed 后返回 `os.ErrClosed` 忽略（Attach 读循环既有纪律）：

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

**解码层零改动**（proto.go:203-209）：`DecodeResize` 钳制 [1,1000] 在 Decode 内既有，直通路径直接消费。**每会话 50ms 防抖**（ROADMAP「含」）挂点在直通分支与 `sess.Resize` 之间——debouncer 共用件形态见下条 §5。

---

### 3. `internal/server/perclient.go` — 输出闭包阻塞持帧 + dwell 武装（D-01/D-02/D-03）

**Analog:** 同文件 ReadLoop 闭包现状（:253-268，「直踢」改造点）；阻塞/等待形态参照 clients.go 信用门 Wait 与 maybeExitWhenEmptyLocked AfterFunc

**现状闭包逐字**（perclient.go:253-268）——改造点 = `trySend` 失败分支从「hubMu 内直踢」改为「阻塞持帧 + dwell 武装」；**detach 门提前返回与每帧 make+copy 两行纪律逐字保持**（P5-1：持帧期间帧在闭包栈上，下一帧读取被阻塞自然无别名窗口）：

```go
	go pc.sess.ReadLoop(func(chunk []byte) {
		select {
		case <-cl.done:
			return
		default:
		}
		s.mc.ptyOutputBytes.Add(int64(len(chunk)))
		frame := make([]byte, 1+len(chunk))
		frame[0] = proto.Output
		copy(frame[1:], chunk)
		if !cl.outbox.trySend(frame) {
			s.hubMu.Lock()
			s.kickSlowConsumerLocked(cl)
			s.hubMu.Unlock()
		}
	})
```

**阻塞等待的锁序红线先例**（clients.go:420-422，信用门持块）——`hubCond.Wait` 原子释放 hubMu 是「持锁等待」的唯一合法形态；**per-client 闭包阻塞绝不可持 hubMu**（单消费者自由度 = 阻塞点在闭包栈上、锁全放）。恢复信号通道（cap 1 信号量 vs cond）归 Claude's Discretion，锁序约束：不得引入 hubMu 反向依赖（CONTEXT code_context §Established Patterns）：

```go
	for s.allWritableBlockedLocked() {
		s.hubCond.Wait() // Wait 原子释放 hubMu；持块即停读 PTY（RES-04）；chunk 停留 ReadLoop 缓冲，无别名窗口（review #1）
	}
```

**dwell 计时器形态母本**（clients.go:934-953，maybeExitWhenEmptyLocked 的 AfterFunc 纪律）——自引用闭包预声明 + 回调首句取 hubMu + 身份比对/状态复查三点逐字可抄（dwell 武装=停读起点、重置=每次续读、到期=hubMu 内 `kickSlowConsumerLocked` 既有序列）：

```go
	// 自引用闭包预声明（:= 短声明作用域起始于声明结束之后，闭包内引用 t
	// 将编译失败 undefined）——var 先声明、AfterFunc 后赋值；回调首句取
	// hubMu（武装方持锁中），取锁成功时赋值必然已完成且可见（锁同步边）。
	var t *time.Timer
	t = time.AfterFunc(s.exitWhenEmptyGrace, func() {
		s.hubMu.Lock()
		defer s.hubMu.Unlock()
		// 复查『计时器身份未易主且仍空且未 exiting』（Pitfall 4 恰好一次兜底；…
		if s.exitEmptyTimer != t || s.exiting || len(s.registry.set) != 0 {
			return
		}
		…
	})
	s.exitEmptyTimer = t
```

**踢出机械零改动复用**（clients.go:592-631，`kickSlowConsumerLocked`）——1013 wire 形态（`Close(1013, "slow_consumer")`）、`kicks` 计数、`emitDetachLocked(reason="kick")`、`removeMember/recalcNow`（arbiter 空集天然 no-op）、`c.pc != nil → teardownPCLocked` 挂点全部既有。 dwell 到期路径 = 取 hubMu → 调用本函数，调用序列与现状闭包直踢分支同形。

**dwell 常量纪律**（D-03 + outboxBytes 先例）：内部常量 + Options 测试可覆写 + 零值兜底三段式，逐字先例 = clients.go:33-36 常量声明区 + server.go:382-384：

```go
if opts.OutboxBytes <= 0 {
	opts.OutboxBytes = defaultOutboxBytes
}
```

**反面教材（勿抄）**：shared 侧 dwell 计时器第 5 轮实证废弃记录（clients.go:473-475 kickOrCreditLocked 注释）——「与 R-08『全体可写端均满 → 置信用保护』语义根本冲突」。per-client 单消费者无信用集、无误判面，该冲突结构性不存在（D-02 论证），但 plan 注释应显式回指此段说明为何 per-client 可 dwell 而 shared 不可。

---

### 4. `internal/server/clients.go` — outbox 恢复信号挂点 + gateTransitions 递增点（D-05）

**Analog:** 同文件 outbox/writer/afterDrain 现状

**outbox 信号量形态**（clients.go:179-209）——cap 1 `notEmpty` 信号量是恢复信号通道的直接选型先例；**drain 至非满 = 恢复信号的 natural 挂点**（CONTEXT code_context）：

```go
type outbox struct {
	mu       sync.Mutex
	q        [][]byte // 共享帧（hub 分配、只读，引用计数靠 GC 自然回收）
	bytes    int
	cap      int
	notEmpty chan struct{}
}

func newOutbox(cap int) *outbox {
	return &outbox{cap: cap, notEmpty: make(chan struct{}, 1)}
}

func (o *outbox) trySend(frame []byte) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.bytes+len(frame) > o.cap {
		return false
	}
	o.q = append(o.q, frame)
	o.bytes += len(frame)
	select {
	case o.notEmpty <- struct{}{}:
	default:
	}
	return true
}
```

**writer drain 恢复点**（clients.go:738-761）——整批写出成功后调 `afterDrain` 的既有挂点行（:759 `s.afterDrain(c)`）即「drain 至非满通知持帧闭包」的同位挂点；锁序纪律保持「drain/写出完才取 hubMu，绝不反序同持」（:757-758 注释）。

**gateTransitions 两递增点先例**（clients.go:515-519 置位点 / :558-559 清位点）——D-05 停读/续读两点递增照此同构（hubMu 保护 plain int，零 atomic；metrics.go 零改动红线：既有计数器多两个递增点，非新增 series）：

```go
	if !c.creditBlocked {
		c.creditBlocked = true
		c.creditPending = frame      // 触发帧暂存，afterDrain 重投（触发帧不丢）
		s.registry.gateTransitions++ // Phase 8 OPS-07 门开闭周期计数挂点（review #10）
	}
```

---

### 5. `internal/server/resize.go` — 每会话 debouncer 共用件抽取（ROADMAP「含」，Pitfall 7 防线）

**Analog:** 同文件 arbiter timer 形态（:59-94）

**防抖机械逐字母本**（resize.go:59-94）——`time.AfterFunc` 创建即 Stop 为 stopped 态 + 首次上报 `Reset` 才武装 + 回调入内先取锁（Go 1.23+ timer 语义下 Reset 与回调并发安全，重复触发幂等）。**抽取方向**（Claude's Discretion）：把「单 time.Timer reset 防抖」从 arbiter 字段耦合中抽为共用件，arbiter 与每会话直通路径各持一实例防双写漂移；per-client 实例的回调**不取 hubMu**（直通 Resize 仅取 sess.fdMu，锁序三规则）：

```go
type arbiter struct {
	sizes map[*client]dims
	timer *time.Timer
	last  dims
}

func (s *Server) initArbiter() {
	s.arbiter = arbiter{sizes: make(map[*client]dims)}
	s.arbiter.timer = time.AfterFunc(s.resizeDebounce, func() {
		s.hubMu.Lock()
		defer s.hubMu.Unlock()
		s.recalcNow()
	})
	s.arbiter.timer.Stop()
}

func (s *Server) reportResize(c *client, cols, rows int) {
	if _, ok := s.arbiter.sizes[c]; !ok {
		return
	}
	s.arbiter.sizes[c] = dims{cols: cols, rows: rows}
	s.arbiter.timer.Reset(s.resizeDebounce)
}
```

**防抖时长来源**：`defaultResizeDebounce = 50 * time.Millisecond`（clients.go:55-58）+ `Options.ResizeDebounce` 测试可覆写——每会话实例同源消费，**不新增第二份时长常量**（双写漂移防线）。

---

### 6. `web/src/main.ts` — Welcome 分支 reset + 模式位 + ro RESIZE 第一闸放开（D-07/D-09/D-10）

**Analog:** 同文件 WELCOME 分支（:639-807）+ connect() per-connection 重置面（:496-512）+ sendResize isRO 闸（:325-341）

**模式位解析形态母本**（main.ts:655-664，sessionDims 容错段）——「缺键 = 旧服务端 = shared，行为零漂移」的前端防御性缺省逐字同构（D-08 协议演化纪律的前端半侧）：

```ts
          if ('cols' in w || 'rows' in w) {
            if (
              typeof w.cols === 'number' && Number.isInteger(w.cols) && w.cols >= 1 && w.cols <= 1000 &&
              typeof w.rows === 'number' && Number.isInteger(w.rows) && w.rows >= 1 && w.rows <= 1000
            ) {
              sessionDims = { cols: w.cols, rows: w.rows };
            } else {
              console.warn('ignoring invalid session dims in WELCOME frame');
            }
          }
```

**reset 调用点的落位约束**（D-09）：只能在 Welcome 分支内（onopen 时模式位未到手）；统一判断无分支——首连时屏幕本空，reset no-op 等价。既有「重连成功点」段（main.ts:782-793）是顺序锚——`stopReconnect()` → 清屏的既有形态中，`term.clear()` 是**清屏先例**；per-client 的 `terminal.reset()`（xterm API，比 clear 更深——清 buffer + 状态复位）按 D-09 在 Welcome 分支统一判断，与 reconnecting 门闩**无耦合**（首连也执行）：

```ts
          if (reconnecting) {
            stopReconnect();
            term.clear();
          }
```

**模式位存储形态**（Claude's Discretion，IN-01 登记口径）：per-connection 模块级 `let`，入 connect() 重置块——既有重置面逐字（main.ts:498-512），新变量同批清零：

```ts
  // 每次尝试重置 per-connection 状态——auth_failed 重试不携带上次连接残留
  opened = false;
  helloSent = false;
  lastError = null;
  lastExit = null; // IN-01 防漂移登记同款——auth_failed 重试/重连不携带上连接的 EXIT 暂存
  // isRO/welcomeDone 同属 per-connection（IN-01 防漂移登记，Phase 6 自动重连落地前提）；
  // osc52Loaded/retriedAuth 与 reconnecting/attempt 等重连循环状态为页面级门闩，刻意不重置
  isRO = false;
  welcomeDone = false;
```

**ro RESIZE 第一闸放开点**（main.ts:325-341）——`if (isRO) return;`（:332）改为按模式位放行（per-client ro 恢复上报；shared ro 保持不发，05-08 逐字保留）。闸的前后注释（D-09 第一闸登记 + 服务端第二闸互指）需同步改写，两侧闸注释互指纪律与 proto.go:6 同款：

```ts
function sendResize(cols: number, rows: number): void {
  // Hello 完成前禁发任何数据帧：…
  if (!helloSent) return;
  // D-09 第一闸：ro 客户端不发 RESIZE。Hello 携首尺寸不受影响——helloSent 门先于
  // isRO 生效（isRO 仅在 WELCOME 到达后才可能为 true，彼时 Hello 已发出）；
  // 服务端忽略 ro RESIZE 为兜底第二闸（05-04 已落）
  if (isRO) return;
```

**Planner 注意**：D-06/D-07 必须同 plan 落地或显式标注配对关系（CONTEXT specifics——仅服务端放行则自家前端零变化，仅前端放开则消息空转）。`[ro] ` 标题前缀恒不变（FEATURES Keep 清单，main.ts:289 `setTitle` 单一写口不触碰）。静默 reset 无提示文案（D-10）。

---

### 7. `internal/server/perclient_test.go` — 扩展 8-12 新测（D-11/D-12）

**Analog:** 同文件 harness 与既有测 + slowclient_test.go stall 夹具

**harness 直通**（perclient_test.go:56-102）——`startPerClientServerWithSpawn(t, spawnFn, mutate)` 四返回值形态（exitCh/wsURL/srv/spawnedSessions）+ spawn 追踪 Cleanup 夹具，新测直接复用；**dwell 测试覆写经 `mutate` 参数注入**（Options 三段式先例，:1119 `func(o *server.Options) { o.StopTimeout = time.Second }` 同形）。

**stall 夹具 + 1013 断言通道**（slowclient_test.go:7-13 纪律 + :101-130）——dwell 踢出测试的客户端侧形态：dialHello 后不再 Read → TCP 缓冲填满 → outbox 涨满；`assertKicked1013` 的两种合法终结形态（CloseError{1013,"slow_consumer"} / CI 慢速 EOF 切面 + acc 阈值）逐字可抄或包内直接调用（同包 server_test）：

```go
// stall 夹具纪律（RESEARCH Validation 裁决 + 本机 /proc 实测）：dialHello 成功后
// 不再调用 Read——TCP 接收缓冲填满 → 服务端 writer 阻塞 → outbox 涨满。…
// 客户端 Read 永不带 deadline ctx（Pitfall 2）——一律 goroutine + 缓冲 channel +
// select time.After 竞速形态。
```

**时序双断言形态**（perclient_test.go:1115-1153，TestPerClientStopTimeoutKillFallback）——「到期前静默窗 / 到期后护栏内收码」镜像到 dwell 断言（停读起点武装 → dwell 到期 1013；续读重置计时 = 「慢但在前进永不踢」断言面）。**禁精确时点断言**（phase06.mjs:354-356 容差论证经 11-CONTEXT 继承）。

**输出不丢断言通道**（perclient_test.go:119-135 `readOutputUntil` + slowclient_test.go `readUntilError` acc 累积）：停读期输出积压 → 恢复后完整到达的字节连续性断言可抄 TestGlobalCredit 的 seq 洪水 + 字节连续判定（slowclient_test.go:31-49 `seqFlood` 平台分支）。

**夹具红线逐字保持**（文件头 :13-14）：客户端 Read 永不带 per-read deadline；静默窗口一律 select + time.After 竞速；统护 ctx 只做护栏。观测出口经 export_test.go `ForTest` 形态新增（如停读/续读计数断言需出口时——:26/:48 两先例形态：hubMu 内读写、调用方不得持 hubMu、注释载「故障注入语义仅服务测试」）。

---

### 8. `web/uat/phase12.mjs` — 协议层 UAT（新增，D-12）

**Analog:** `web/uat/phase11.mjs`（同构母本——骨架逐字可抄，场景函数 S 编号系列替换）

**骨架构成件**（phase11.mjs 全文即模板）：

- **头注释纪律**（:1-35）：覆盖需求编号清单 + 红线声明 + 时序纪律 + 运行方式，phase12 版替换为 PC-05/06/07/10/11 场景清单
- **check/skip 双通道**（:61-74）+ `emittedDetails` 收集 + `assertOutputClean()` 运行时自净（:563-573——token/pid 永不进 detail 的红线自证，**逐字保留**）
- **startWesh**（:100-128）：`--bind 127.0.0.1 --port 0` + stdout 两行解析 + 50ms 落定窗 + SIGKILL 收口
- **dialHello**（:135-152）：Welcome 到达 = 握手完成 + 10s watchdog
- **帧工具**（:44-59, :187-194）：帧常量与 proto.go 对齐注释、`helloFrame`/`sendInput`/`outputText`/`exitOf`
- **pid 工具**（:199-255）：`readPid`（正则只命中结果行）/`pgroupAlive`（fail-closed）/`pollESRCH`
- **场景收尾纪律**（:576-595）：scenarios 数组串行 + 场景间 300ms + 异常纳入 emittedDetails + skipped 不阻塞退出码

**dwell 真实等待先例**（D-12：phase12.mjs 做一次真实 10s+ dwell 到期 1013 端到端证据）：phase11.mjs:32-33 时序纪律逐字——「宽限/退避/免疫类场景真实等待，超时上限只做护栏，禁精确时点断言；时钟不 mock、服务端等待不缩短」（phase06 保活测 11s+ 先例经 CONTEXT 登记）。**零测试钩子进二进制**——Go 测用 Options 覆写短 dwell，UAT 用真实 10s。

**场景建议面**（CONTEXT Claude's Discretion 已载）：resize 直通双端互不影响 / ro RESIZE 直通 stty 证据 / ro INPUT 丢弃+rw 限速 / Welcome.session 字段 / 停读期输出不丢恢复后完整到达 / dwell 到期 1013。stty 证据形态先例 = phase11.mjs S2b（:331-340，`stty size` 回读 "44 111" 行命中纪律）。

---

### 9. `web/uat/phase12-dom.mjs` — jsdom 前端断言（新增，D-13；文件名归 planner）

**Analog:** `web/uat/phase06-dom.mjs`（loadTerminal 夹具 + D1 reset/clear 观测先例 + D8）

**loadTerminal 夹具逐字复用面**（phase06-dom.mjs:126-285）：jsdom 加载真实 `web/dist/index.html` → 提取 `<script type="module">` bundle `window.eval` 执行；SpyWebSocket（构造计数 + sentFrames 首字节记录 + `synthClose(code)` 合成 CloseEvent）；布局桩（720x408 恰 80x24）；console.info/warn 捕获与 unhandledrejection 通道。**phase12 新增 mock 面**（Claude's Discretion）：terminal.reset 调用计数断言——可经 SpyWebSocket 同族的包装注入（bundle 内 `term` 不可直触时的观测面：重置后的 DOM 可观测效应，D1h 先例）。

**reset 可观测证据先例**（phase06-dom.mjs:287-288 + :345-350）——`terminalText` 通道断言「断开前写入的文本在重连后从 DOM 消失」即 `term.clear()` 的 D1h 形态；`terminal.reset()` 断言同通道（reset ⊇ clear 的 DOM 效应），或经 mock 形态断言调用计数：

```js
// 终端 DOM 可见文本（清屏断言通道——term.clear() 后 buffer 行重渲染，异步由 waitFor 吸纳）
const terminalText = (document) => document.querySelector('.xterm-rows')?.textContent ?? '';
```

**三断言面**（D-13）：① 模式位=per-client → reset 调用（重连全链形态抄 D1：synthClose(1006) → 退避 → 新 WELCOME → reset 观测）；② per-client ro 端 RESIZE 发送恢复（sentFrames 首字节含 0x31——SpyWebSocket send 记录通道既有，:167-171）；③ 旧服务端缺 session 键 → 不 reset（**兼容形态注入面**：需向 jsdom 端喂一帧无 session 键的 Welcome——可经 SpyWebSocket 合成下行帧或共享模式真实服务端双实例，归 planner 定）。隔离纪律（:38）：每场景独立 spawn 实例 + 独立 jsdom。

**红线**：本 phase 不建 phase12-pw（Playwright 归 Phase 14，D-13 明示防重复建设）。

---

## Shared Patterns

### 锁序三规则（§5）——一切新代码的硬约束
**Source:** clients.go:10-12 文件头 + resize.go:6-9
**Apply to:** perclient.go 阻塞持帧/恢复信号/dwell 回调、resize.go debouncer 抽取、server.go RESIZE 直通分支

```go
// 锁序纪律（R-07）：hubMu > outboxMu——writer 持 outboxMu drain 期间绝不取 hubMu；
// drain 完释放后才取 hubMu 做 afterDrain 恢复判定，绝不反序同持。信用门 cond
// 挂 hubMu（信用门状态与注册表同锁），outbox 自有 mu。
```

直通 Resize 仅取 `sess.fdMu` 不持 hubMu（hubMu > sess.fdMu 同序）；dwell 到期踢出经 hubMu → `kickSlowConsumerLocked` 既有序列；恢复信号通道不得引入 hubMu 反向依赖；per-client 闭包阻塞时**不持任何锁**（帧在闭包栈上）。

### AfterFunc 计时器纪律（dwell 武装/重置/到期）
**Source:** clients.go:934-953（maybeExitWhenEmptyLocked）
**Apply to:** perclient.go dwell 计时器、resize.go 抽取件（既有形态保持）
要点三件套：var 预声明自引用闭包 → 回调首句取 hubMu → 身份比对/状态复查（陈旧回调不动作）。dwell 每次续读重置计时（`timer.Reset`，arbiter 防抖同款机械）。

### cap-1 信号量通道
**Source:** clients.go:189（`notEmpty: make(chan struct{}, 1)`）+ :204-207 非阻塞发送
**Apply to:** 阻塞持帧恢复信号（如选信号量形态——Claude's Discretion 两选项之一）

### 常量 + Options 测试覆写三段式
**Source:** clients.go:33-68 常量声明区 + server.go:249-317 Options + server.go:382-384 零值兜底
**Apply to:** `defaultSlowDwell`（10s 内部常量，D-03 不暴露 CLI flag/TOML——Options 字段为测试覆写通道，与 OutboxBytes/StopTimeout 先例同档）

### P5-1 别名红线（每帧 make+copy）
**Source:** pty/io.go:11-14 + perclient.go:260-262
**Apply to:** 阻塞持帧全程——帧在闭包栈上持有期间 ReadLoop 自然停摆无别名窗口；**持帧帧不得复用 ReadLoop 缓冲**（make+copy 后再持帧，现状已满足，逐字保持）

```go
// onChunk 在读循环 goroutine 内同步调用、复用底层缓冲——回调方如需跨帧持有须自行拷贝。
```

### CR-01 读循环零同步写
**Source:** CONTEXT code_context §Established Patterns + clients.go:763-780 inputWriter
**Apply to:** 阻塞持帧绝不演化为「读循环直写 master 等 outbox」；阻塞点只在输出闭包（PTY→WS 方向），INPUT 路径零改动

### 审计/事件 schema
**Source:** clients.go:862-874（emitDetachLocked）+ perclient.go:224-233（attach 事件）
**Apply to:** 本 phase 零新事件（session_start/end 归 Phase 13 窗口期已接受）；dwell 踢出走既有 `detach reason=kick code=1013` 单事件，零新增

### 测试期观测出口
**Source:** export_test.go（PCSationsLenForTest :48-52 / ShrinkOutboxForTest :26-39）
**Apply to:** 停读/续读/dwell 计数或状态断言如需观测口——`ForTest` 后缀 + hubMu 内读写 + 「调用方不得持 hubMu」注释 + 「仅服务测试」声明四件套

### UAT 红线（协议层与 jsdom 双侧）
**Source:** phase11.mjs:27-30 + phase06-dom.mjs:40-41
**Apply to:** phase12.mjs / phase12-dom.mjs——token/凭据/pid 数值只作断言材料，永不进 check detail/控制台输出；`assertOutputClean()` 运行时自净断言逐字保留

### 收口闸（零回归双证据）
**Source:** CONTEXT 已锁定不重复决策（15 行）
**Apply to:** 全部改动——shared 全量 Go 测试原样绿 + phase02-11.mjs 默认模式零修改重跑 + 期望值逐字未动；禁止断言放宽成「两模式都接受」；ro RESIZE 放行/模式位下发只在 per-client 分支生效，shared 路径逐字不动

## No Analog Found

无完全无 analog 的文件。两条**组合形态**注记（机制为新组合，零件全部既有）：

| 形态 | 组成零件来源 | 说明 |
|------|-------------|------|
| 阻塞持帧 + 恢复信号通道（D-01） | outbox cap-1 信号量（clients.go:189）+ 信用门 Wait 锁序先例（clients.go:420-422）+ drain 恢复挂点（clients.go:759） | 「ReadLoop 闭包 select 等恢复信号/cl.done 逃逸」在代码库无现成同构——shared 不能阻塞读循环正是 creditPending 形态的存在理由；per-client 单消费者自由度下由既有零件组合，精确选型归 Claude's Discretion（锁序约束见 Shared Patterns §1） |
| dwell 看门狗（D-02/D-03） | AfterFunc 三件套（clients.go:934-953）+ kickSlowConsumerLocked（clients.go:592-631）+ 常量三段式（clients.go:33-36 + server.go:382-384） | shared 侧 dwell 第 5 轮实证废弃（clients.go:473-475 注释）是**反面教材而非母本**——per-client 无信用集语义冲突（D-02/D-04 论证），plan 注释须显式回指该段 |

WR-01 闭合回指（D-04 要求 plan 中显式登记）：`.planning/STATE.md:99` 登记的宽限门/creditPending 丢失项，按「dwell 涵盖不复刻」形态闭合——dwell 10s 从停读起点武装结构性涵盖 500ms attach 宽限（specifics 量化论证：×20 余量 + 浏览器后台标签节流风险接受段），阻塞持帧即暂存（帧在闭包栈上 ≡ creditPending 字段语义等价）。

## Metadata

**Analog search scope:** `internal/server/`（server.go/clients.go/perclient.go/resize.go/export_test.go/slowclient_test.go/perclient_test.go）、`internal/pty/io.go`、`internal/proto/proto.go`、`web/src/main.ts`、`web/uat/phase11.mjs`、`web/uat/phase06-dom.mjs`
**Files scanned:** 12（全文读取 8，靶向区段读取 4：server.go :1-60/:160-219/:216-326/:1000-1259/:1555-1578，main.ts :176-375/:490-629/:630-809/:900-990）
**Pattern extraction date:** 2026-09-04
**注：** 本 phase 无 RESEARCH.md（research 显式跳过——CONTEXT.md `<canonical_refs>`/`<code_context>` 自带 file:line 实证，已全部并入上表）。ARCHITECTURE.md §3.4 的 RESIZE 直通参考实现为调研期伪码，落地时以上表 §2 的真实代码形态为准。
