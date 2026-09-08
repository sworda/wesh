# Phase 11: per-client 生命周期主干 - Pattern Map

**Mapped:** 2026-09-03
**Files analyzed:** 6（3 改 Go 生产文件 + 1 改 pty 平台文件 + 2 全新测试/UAT 文件）
**Analogs found:** 6 / 6（3 处形态缺口见「No Analog Found」：SpawnFunc 失败桩被真实调用、UAT pgid ESRCH 断言通道、reaped 栅栏锁归属）

> **行号勘误**：CONTEXT.md 行号为讨论快照，本文全部行号以 2026-09-03 源码实测为准（INPUT case 实际 :1094-1122、detach 实际 :775-815、kickSlowConsumerLocked 实际 :578-608、New goroutine 钉死点实际 :493-495、client 结构实际 :95-160）。另：`exitmsg.go` 不存在——退出码提取逻辑内联在 `server.go:1360-1365`（lifecycle），helper 为 `exitMessage`/`exitSignalNum`（server.go:1314-1354）；研究 §4.1 的 `exitCodeOf(err)` 需新建小 helper 或各行内联（Discretion）。

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/server/server.go`（Attach 升档序列 :951-1071） | server core / 握手升档 | request-response（WS 握手状态机） | 同文件 shared 升档 else 块（:951-1071）+ ③位容量闸（:851-856）+ version_mismatch Error+Close（:935-937） | exact（同文件同函数内模式分支） |
| `internal/server/server.go`（INPUT case :1094-1122） | server core / 读循环分派 | event-driven（逐帧热路径） | 同文件现状行 `s.inputQ.tryEnqueue`（:1120） | exact（一行切换，CR-01 纪律载体） |
| `internal/server/server.go`（New 尾部钉死点 :488-495 + ValidateOptions :336-348） | server core / 装配 | event-driven（goroutine 拓扑分岔） | 同文件现状三件套启动（:493-495）+ Phase 10 已落地互斥两规则（:341-346） | exact（同文件） |
| `internal/server/clients.go`（client 结构 :95-160 +inQ/pc） | registry model | —（状态结构，写一次后只读） | 同文件 remoteUser 字段注释先例（:98-105）+ pongTimedOut（:144-148） | exact（同文件） |
| `internal/server/clients.go`（detach :775-815 / kickSlowConsumerLocked :578-608 SIGHUP 挂点） | lifecycle hook | event-driven（注册表移除点触发） | 同文件 maybeExitWhenEmptyLocked 挂点位（:602/:813）+ stopChildLocked（:919-924） | exact（同文件同位同款） |
| `internal/server/perclient.go`（NEW） | service / 每会话生命周期 | event-driven（spawn/watcher/teardown） | 复合母本：lifecycle()（server.go:1359-1441）+ stopChildLocked（clients.go:919-924）+ onChunk（clients.go:399-417）+ inputWriter（clients.go:751-764）+ termOnce（server.go:1446-1450） | role-match（全部件有 1:1 母本，组合形态为新） |
| `internal/pty/reap_darwin.go`（watch() :42-62 dup-watch 防御） | platform reap | event-driven（kqueue 订阅注册） | 同文件注册失败摘除分支（:54-60）+ awaitExit 退化兜底（:109-120） | exact（同文件） |
| `internal/server/perclient_test.go`（NEW） | test | request-response（WS 端到端断言） | e2e_test.go harness（:128-277）+ exit_test.go（:25-139）+ stopseq_test.go（:47-134）+ options_test.go（:17-46） | role-match |
| `web/uat/phase11.mjs`（NEW） | UAT script | request-response（协议层全链八场景） | phase06.mjs 全文（生命周期 UAT 母本）+ phase06.mjs S6 readPid（:423-433） | role-match |

---

## Pattern Assignments

### 1. `internal/server/server.go` Attach 升档序列（per-client 分支插入）

**Analog A:** shared 升档序列本体（server.go:979-1041——插入点 = `close(helloDone)+release()` :979-980 之后、Welcome 入队 :1023 之前；研究 §3.1 时序：容量再闸（hubMu 读 len(pcSessions)）→ 放锁 spawnFn → 失败 Error+1011 / 成功再取锁构造 client{inQ: pc.inQ, pc} → Welcome 回显 → registerLocked + pcSessions 登记 + D-03 复检回收）:

```go
close(helloDone)
release()
s.hubMu.Lock()
effMode, rwEligible, becomeOwner := s.decideModeLocked(mode)
cl = &client{
	conn:       c,
	remote:     remote,
	remoteUser: remoteUser,
	rwEligible: rwEligible,
	dims:       dims{cols: h.Cols, rows: h.Rows}, // Hello 首尺寸（DecodeHello 已 ClampDim）
	outbox:     newOutbox(s.outboxBytes),
	done:       make(chan struct{}),
	cancel:     cancel,
	limiter: rate.NewLimiter(rate.Limit(s.inputRate), s.inputBurst),
}
cl.mode.Store(effMode)
```

→ per-client 分支的关键差异点：① spawnFn(h.Cols, h.Rows) 消费 **Hello 钳制尺寸**（:988 的 dims 登记同源值——DecodeHello 已 ClampDim，StartWithSize 契约「调用方已钳制」自然满足，spawn.go:73-75）；② mode 判定用单行门 `writable && ticketMode==rw → rw else ro`（decideModeLocked 不调用，CONTEXT code_context :95）；③ Welcome cols/rows 回显本端 Hello 钳制尺寸（不经 sessionDimsLocked——per-client 无仲裁，G-05-1 契约退化为恒等式）。

**Analog B:** Welcome 恒首帧时序纪律（server.go:1012-1024——Welcome 入队先于 registerLocked 且全程持 hubMu，onChunk 遍历注册表不触未登记者）:

```go
	sd := s.sessionDimsLocked()
	prefs := s.clientPrefsRO
	if effMode == proto.ModeRW {
		prefs = s.clientPrefsRW
	}
	cl.outbox.trySend(proto.WelcomeFrame(effMode, prefs, sd.cols, sd.rows))
	s.registry.registerLocked(cl)
```

→ per-client 下同构成立：Welcome 入队先于 registerLocked + pcSessions 登记，ReadLoop 闭包 goroutine 注册后启动；spawn→ReadLoop 启动间输出由 64KiB 内核缓冲承接（研究 §3.1 约束 2）。

**Analog C:** ③位容量闸的 logEvent 形态（server.go:851-856——`max_clients` 机器串与 remote/remoteUser 四段 schema 即 D-04 容量再闸事件名的直接通道）:

```go
if s.registry.n.Load() >= int64(s.maxClients) {
	release()
	logEvent(remote, websocket.StatusCode(http.StatusServiceUnavailable), "max_clients", remoteUser)
	http.Error(w, "server is full", http.StatusServiceUnavailable)
	return
}
```

→ per-client 容量再闸（D-02）改走 WS 面：`proto.ErrorFrame(proto.ErrServerError, "server is at capacity…")` 直写 + `Close(1011)` + 同通道 logEvent（事件名 `max_clients` 与 spawn 失败的 `spawn_failed` 区分，D-04）。1011 前端分派 = Error message 优先展示（main.ts:939-944——D-02 裁决依据）。

**Analog D:** Error 帧 + Close 机器串同名的既有形态（server.go:935-937，version_mismatch 先例——spawn 失败 wire 面的逐字母本；`proto.ErrServerError` = proto.go:61 已备未启用常量，协议零改动）:

```go
	_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrVersionMismatch, "protocol version wesh.v1 required"))
	logEvent(remote, websocket.StatusPolicyViolation, proto.ErrVersionMismatch, remoteUser)
	_ = c.Close(websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
```

→ spawn 失败形态：`ErrorFrame(proto.ErrServerError, "failed to start process")` → `logEvent(remote, websocket.StatusInternalError, "spawn_failed", remoteUser)` → `Close(websocket.StatusInternalError, "server_error")`。**红线**：message 绝不拼 `err.Error()`（Security Mistakes 表——路径/errno 泄露面）；失败点在注册之前，收口 = 关连接零残留（Pitfall 5 清理清单）。

**不装配警示**：per-client 分支**不调用** `s.sess.SignalForegroundGroup()`（server.go:1070——子进程以正确尺寸 spawn，无重绘需求，研究 §1.2）；`SignalForegroundGroup` 与 `s.participates/addMember/recalcNow/sessionDimsLocked`（:1008-1012）整段为 shared-only。

---

### 2. `internal/server/server.go` INPUT case（一行切换）

**Analog:** 现状 case 本体（server.go:1094-1122）——唯一改动行 :1120:

```go
		case proto.Input:
			if cl == nil || cl.mode.Load() == proto.ModeRO {
				continue
			}
			if !cl.limiter.AllowN(time.Now(), len(data)-1) {
				s.inputDrops.Add(1)
				continue
			}
			if !s.inputQ.tryEnqueue(data[1:]) {
				continue // 满则丢（droppedInputs 计数已在 tryEnqueue 内递增）；限速器在前，队列满本应罕见
			}
```

→ `s.inputQ.tryEnqueue` → `cl.inQ.tryEnqueue`（Pattern 2 间接字段：升档构造时 shared = `s.inputQ`、per-client = `pc.inQ`，读循环逐行不分支）。mode 门（:1101）与限速门（:1111）逐字不动；`cl == nil` 守卫（握手违规落入路径）对两模式同形保持——`cl.inQ` 在升档构造赋值前不存在读者（cl 非 nil 即已升档）。

---

### 3. `internal/server/server.go` New 尾部 goroutine 钉死点 + ValidateOptions

**Analog A:** 现状三件套钉死段（server.go:488-495）:

```go
	s.startedAt = time.Now()
	emitEvent(slog.String("event", "session_start"), slog.Int("pid", sess.Cmd.Process.Pid))
	s.sessionAlive.Store(true)
	go sess.ReadLoop(s.onChunk)
	go s.inputWriter() // CR-01：input-writer 唯一装配点——master 写路径独占在专属 goroutine
	go s.lifecycle()
```

→ **模式分岔必须覆盖 :488-495 整段**（关键陷阱：per-client 下 `sess` 恒 nil——:489 的 `sess.Cmd.Process.Pid` 引用与 :493 的 `sess.ReadLoop` 不分支即 nil deref panic）。per-client 分支形态：`pcSessions` map 初始化（hubMu 保护字段，research §2.1）+ **本阶段零全局 goroutine**（pcSupervisor/pcExitReq 归 Phase 13，CONTEXT Out-of-scope 与 Integration Point :115 明示）+ session_start 进程级事件不 emit（D-04 窗口期空白已明示接受，Phase 13 以 per-client 粒度一次补齐）。shared 分支 = 现状五行逐字不动（零回归红线）。另：`go s.inputWriter()` 随 §6d 参数化同步改签名。

**Analog B:** ValidateOptions 互斥两规则（server.go:341-346，Phase 10 已落地）+ 注释预言（:327-328「sess 维度规则归 Phase 11 生命周期落地——本阶段不加第三规则防双写」）:

```go
	if mode == SessionModePerClient && opts.SpawnFunc == nil {
		return errors.New("server: session-mode per-client requires SpawnFunc")
	}
	if mode == SessionModeShared && opts.SpawnFunc != nil {
		return errors.New("server: session-mode shared must not set SpawnFunc")
	}
```

→ 本阶段兑现注释预言：sess 维度收编（per-client 要求 `sess == nil`、shared 要求 `sess != nil`——New 签名 :362 收 sess 为首参，校验在 New 前由 main 调用）。测试锁 = options_test.go 表加行（§8 Analog D）。

---

### 4. `internal/server/clients.go` client 结构（+inQ/pc 两字段）

**Analog:** remoteUser 字段的「写一次 + happens-before 论证」注释形态（clients.go:98-105——新字段注释的直接母本）:

```go
	// remoteUser 为 logEvent 可选第四字段 remote_user 的值（07-03，SEC-07 D-15
	// 审计归因）：Attach 入口经 s.proxy.remoteUser(r) 提取一次（sanitize 已在
	// 提取点完成），此后只读——并发读写面与 remote 字段既有形态相同（写一次
	// 发生在 client 构造、registerLocked 之前，全部读者在其后启动的
	// writer/pinger/读循环 goroutine 内，happens-before 由 goroutine 启动建立；
	// plain 字段无锁安全，-race 全量回归锁）。空串 = 未配置/头缺席（logEvent
	// 不出键）。share token 渠道进入的客户端同经 Attach 入口提取点赋值。
	remoteUser string
```

→ 追加两字段（研究 §2.1，写一次于升档、registerLocked 之前，读者为该会话 goroutine 群与 hubMu 内读者——happens-before 由 goroutine 启动 + hubMu 建立，同一论证形态）:

```go
	inQ *inputQ    // 输入队列挂点：shared = s.inputQ；per-client = pc.inQ（读循环零分支的关键）
	pc  *pcSession // per-client 会话绑定；shared 恒 nil
```

并发位纪律对照先例：`pongTimedOut`（clients.go:144-148——「置位取 hubMu 写、detach 同锁读，禁止 plain 字段跨 goroutine 传递」）是 `pc.exitCode` 类运行期可写字段的注释母本；`inQ/pc` 为写一次只读字段，对齐 remoteUser 形态即可。

---

### 5. `internal/server/clients.go` detach / kickSlowConsumerLocked（per-client SIGHUP 挂点）

**Analog A:** 两移除点的现状序列（detach :775-815 与 kick :578-608——`removeLocked` 返回 true 之后、`hubCond.Broadcast` 之前即 `maybeExitWhenEmptyLocked` 挂点位；per-client SIGHUP 挂点同位插入，Anti-Pattern 7：挂注册表移除点覆盖一切断开形态——正常关闭/1006/pong 超时/Shutdown 1001 广播引发的 detach 全部经此）:

```go
func (s *Server) detach(c *client) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	if !s.registry.removeLocked(c) {
		return // 已被 kick 移除——close(done)/cancel 恰好一次由成员判定保证
	}
	// …… emitDetachLocked / close(done) / cancel / removeMember / recalcNow / promote ……
	s.maybeExitWhenEmptyLocked(c)
	s.hubCond.Broadcast() // P5-7 统一挂点：detach 后门重估
}
```

**Analog B:** stopChildLocked 的 SignalGroup + AfterFunc KILL 形态（clients.go:919-924——D-01 每会话化的直接母本；SignalGroup 不取 fdMu，hubMu 内发送安全，锁序 hubMu > sess.fdMu 不受影响）:

```go
func (s *Server) stopChildLocked() {
	s.sess.SignalGroup(s.stopSignal)
	if s.stopTimeout > 0 {
		time.AfterFunc(s.stopTimeout, func() { s.sess.SignalGroup(syscall.SIGKILL) })
	}
}
```

→ 每会话形态（研究 §4.2 + D-01 机制定型）：挂点处 `if cl.pc != nil { pc.teardown() }` 触发——**执行序列只有一个**（Pitfall 3：sync.Once + 固定序列 SIGHUP（经 reaped 栅栏）→ stopTimeout>0 则 AfterFunc 补 SIGKILL → Drain(200ms) → Close(master) → Wait 返回 → pcSessions 移除）；SIGHUP 可在 hubMu 内发，**Drain/Close/Wait 绝不能在 hubMu 内**（200ms 级阻塞 = 全服务端行头阻塞，Pitfall 3 红线——慢半段放会话自有 goroutine）。两移除点挂点注释与 maybeExitWhenEmptyLocked 同位同款纪律（:599-601/:810-812 注释形态）。

---

### 6. `internal/server/perclient.go`（NEW——pcSession / 升档分支 / ReadLoop 闭包 / sessionWatcher / teardown）

**文件组织纪律母本**：log.go:1-23 文件头注释形态（包级 + 决策号/研究锚点登记）；全部件注释密度对齐 server.go/clients.go 现状（论证链锚定 file:line + 决策号）。

**6a. pcSession 结构**——母本 = client 结构注释纪律（§4）+ pty.Session 的锁归属注释形态（spawn.go:25-31 fdMu 注释）。字段集 = 研究 §2.1 参考实现（sess/inQ/inputDone/startedAt/exitCode + teardownOnce + reaped 栅栏位——后两者为本阶段新增，研究 §2.1 未含，D-01/Pitfall 2 要求）。

**6b. 升档 per-client 分支**——全部件母本在 §1（容量再闸 C、spawn 失败 D、Welcome 时序 B）；落点与行序 = 研究 §3.1 时序图。D-03 注册点复检回收：registerLocked + pcSessions 登记后在**同一 hubMu 持有内**复检 `len(s.pcSessions) >= s.maxClients`，超编者 `SignalGroup(HUP)+Drain` 回收（research §5 规则 1 建议形态，≤5 行）——「并发子进程数 ≤ maxClients」硬不变量。

**6c. ReadLoop 闭包（每会话输出路径）**——母本 = onChunk（clients.go:399-417，P5-1 别名红线与 trySend→kick 机械）+ writer 的 done 门形态（clients.go:715-720）:

```go
	frame := make([]byte, 1+len(chunk))
	frame[0] = proto.Output
	copy(frame[1:], chunk)
	for c := range s.registry.set {
		if !c.outbox.trySend(frame) {
			s.kickOrCreditLocked(c, frame)
		}
	}
```

```go
	for {
		select {
		case <-c.done:
			return
		case <-c.outbox.notEmpty:
		}
```

→ 每会话闭包形态（研究 §3.5）：detach 门 `select { case <-cl.done: return; default: }` 提前 return（SIGHUP→死亡窗口期不做无谓组帧）→ `s.mc.ptyOutputBytes.Add`（同一计数器聚合口径不变）→ 每帧 make+copy（P5-1 红线——ReadLoop 缓冲复用 io.go:14）→ `cl.outbox.trySend` 失败 → hubMu 内 `kickSlowConsumerLocked`（arbiter 零值下 removeMember/recalcNow 天然 no-op——Go nil map delete 安全，研究 §1.1 安全性论证）。

**6d. inputWriter 参数化（Pattern 3：1 份代码 N 实例）**——母本 = clients.go:751-764 逐字:

```go
func (s *Server) inputWriter() {
	for {
		select {
		case <-s.inputDone:
			return
		case <-s.inputQ.notEmpty:
		}
		for _, p := range s.inputQ.dequeue() {
			if _, err := s.sess.Master.Write(p); err != nil {
				return // 子进程退出/master 已关——终结由 lifecycle 收口（D-12 同款）
			}
		}
	}
}
```

→ 签名参数化为 `inputWriter(sess *pty.Session, q *inputQ, done chan struct{})`；New 的 shared 装配点（server.go:494）同步改传参（行为逐字不变），per-client 每会话装配一次（q = pc.inQ 独享 `newInputQ(defaultInputQueueBytes)`，clients.go:236-240 容量常量复用）。CR-01 红线：读循环永不直写 master（Anti-Pattern 5）。

**6e. sessionWatcher（每会话终结者）**——母本 = lifecycle()（server.go:1359-1441）逐段映射:

```go
func (s *Server) lifecycle() {
	err := s.sess.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	// …… session_end emit :1371-1381（per-client 粒度归 Phase 13，D-04——本阶段 watcher 不 emit session_end）
	s.sessionAlive.Store(false)
	s.sess.Drain(200 * time.Millisecond)
	close(s.inputDone)
	exitFrame := proto.ExitFrame(code, exitMessage(err, code))
	// ……广播段 :1404-1434 → per-client 私有化 = 仅 cl 单端同序列
```

EXIT 直写纪律母本（server.go:1424-1431——组帧一次共享只读 + 同步 Write 2s ctx + Close(1000)，禁 outbox 异步入队，Anti-Pattern 6 关闭帧超车防线）:

```go
		wctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = c.conn.Write(wctx, websocket.MessageBinary, exitFrame)
		cancel()
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
```

→ watcher 序列（研究 §4.1）：`pc.sess.Wait()`（唯一收割者纪律每会话粒度保持——断开路径绝不自己 Wait）→ 退出码提取（**注意**：现状为 lifecycle 内联 :1361-1365，建议抽 `exitCodeOf` helper 两调用点共用，或 watcher 内联同 3 行——Discretion）→ hubMu 内 pcSessions 移除（活性记账单点）→ `Drain(200ms)` → `close(pc.inputDone)` → EXIT 组帧一次 → 直写 2s ctx → Close(1000)。watcher 的 Close(1000) 与 Shutdown 的 Close(1001) 竞态由库 Close 幂等承接（server.go:1469-1473 既有论证同形态）。退出文案/信号 -1 语义复用 `exitMessage`/`exitSignalNum`（server.go:1314-1354）逐字。

**6f. teardown Once + reaped 栅栏**——母本 = termOnce（server.go:1446-1450 sync.Once 收口形态）+ stopChildLocked（§5B）+ Session.Close 的 fdMu+closed 幂等形态（io.go:65-76，锁归属选型参考——Pitfall 2 只锁「信号与 reap 序列化」语义，每会话状态锁 vs hubMu 内标志位为 Discretion）:

```go
func (s *Server) terminate(code int) {
	s.termOnce.Do(func() {
		s.exitf(code)
	})
}
```

```go
func (s *Session) Close() error {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	// ……
}
```

→ D-01 固定序列（Pitfall 3 + Pitfall 2）：`teardownOnce.Do`：SIGHUP（**经 reaped 栅栏**——watcher 的 Wait 返回置位后发信号路径检查同位，kill-after-reap 误杀结构性不可能）→ `stopTimeout > 0` 则 AfterFunc 补 SIGKILL（ESRCH 幂等静默，不占 hubMu）→ Drain(200ms) → Close(master) → Wait 返回 → pcSessions 单点移除。两触发路径（断开 SIGHUP / 子死 watcher）都只「触发」，执行序列只有一个。

**6g. logEvent/emitEvent 消费点**——spawn_failed 单行事件 = log.go:93-103 既有通道（§S3）；红线：零敏感值、审计行无 pid/client_id（D-04 字段集 = remote/code/reason/remoteUser 四段既有 schema 内）。

---

### 7. `internal/pty/reap_darwin.go` watch()（dup-watch fail-closed 防御）

**Analog A:** 现状注册段（reap_darwin.go:42-62——`subs[pid] = ch` 赋值前无 dup 检查即 Pitfall 9 预警形态）:

```go
func (w *exitWatcher) watch(pid int) (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	w.mu.Lock()
	w.subs[pid] = ch
	w.mu.Unlock()
```

**Analog B:** 注册失败摘除分支（:54-60——fail-closed 的错误返回 + 清理形态母本）:

```go
	if _, err := unix.Kevent(w.kq, ev, nil, nil); err != nil { // 非阻塞注册
		// 注册失败须摘除已登记的订阅，避免 subs 泄漏（Rule 2 自动修复）
		w.mu.Lock()
		delete(w.subs, pid)
		w.mu.Unlock()
		return nil, err
	}
```

**Analog C:** 调用方退化兜底（awaitExit :109-120——watch 返回错误 → `cmd.Wait()` 直等的既有形态，零新兜底面）:

```go
	exited, err := w.watch(cmd.Process.Pid)
	if err != nil {
		return cmd.Wait()
	}
	<-exited
	return cmd.Wait()
```

→ 防御形态：`w.mu` 内 `if _, dup := w.subs[pid]; dup { w.mu.Unlock(); return nil, errDupWatch }`（kqueue (ident,filter) 唯一键语义——重复注册是**替换**而非叠加，先注册者 channel 被影子化则 awaitExit 永等 → 会话收割挂死；fail-closed 把「挂死」变「可观测错误」，调用方退化 cmd.Wait() 兜底）。每会话一 goroutine 消费共享 watcher 的 N 规模安全论证 = 文件头 :24-25 注释（「N 会话共用」设计逐字）。

---

### 8. `internal/server/perclient_test.go`（NEW——per-client-only 独立测试文件）

**Analog A:** 测试装配 harness（e2e_test.go:140-156——`startTestServerWith` 形态：pty.Start + New(sess, exitf 捕获桩, opts) + 127.0.0.1:0 监听 + t.Cleanup）:

```go
func startTestServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string) {
	t.Helper()
	sess, err := pty.Start(argv, pty.StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh = make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, opts)
	// …… net.Listen + t.Cleanup(killServer) + go http.Serve
}
```

→ per-client 变体（新文件内自建，**不碰既有 harness 装配点**，D-05）：`New(nil, exitf, Options{SessionMode: SessionModePerClient, SpawnFunc: func(cols, rows int) (*pty.Session, error) { return pty.StartWithSize(argv, pty.StartOptions{Uid: -1, Gid: -1}, cols, rows) }})`——SpawnFunc 闭包捕获 argv 的形态即 main 生产闭包的镜像（Options.SpawnFunc 注释 server.go:302-309 描述的消费形态）。

**Analog B:** 收口纪律（e2e_test.go:128-134 `killServer`——泄漏子进程在 CPU 受限 CI 上级联减速的实证注释必须随新 harness 重现；per-client 下 N 会话的收口需追踪全部已 spawn 子进程——新形态，见 No Analog ①）:

```go
func killServer(ln net.Listener, sess *pty.Session) {
	ln.Close()
	if sess.Cmd != nil && sess.Cmd.Process != nil {
		_ = sess.Cmd.Process.Kill()
	}
	_ = sess.Close()
}
```

**Analog C:** EXIT 断言三件套（exit_test.go:25-58——`readExitClose` 读至 CloseError 收集帧序 + `decodeExitFrame` 类型字节+JSON 解码；EXIT 私有化断言 = 本端 readExitClose 断言末帧 EXIT + close 1000，他端零感知断言 = 逐字节无扰动 + EXIT 类型字节缺席，phase06.mjs S6 :443 的 `noExitFrame` 形态在 Go 侧的同构）:

```go
func readExitClose(t *testing.T, ctx context.Context, c *websocket.Conn) (frames [][]byte, code websocket.StatusCode) {
	// …… Read 带调用方 10s 统护 ctx（永不带 per-read deadline，Pitfall 2 回归锁）
}
```

**Analog D:** HUP 免疫 KILL 兜底夹具（stopseq_test.go:97-134——`trap "" TERM` + while 循环恒活 + 落盘标记 waitMarker 同步 trap 安装 + 「stop-timeout 前静默 / 到期后 5s 内收码」时序双断言形态；per-client 版断言对象从 exitf(-1) 换为 watcher 收割后 EXIT{exit_code:-1} + pgid ESRCH——D-01 S8 的 Go 侧对应物）:

```go
	// stop-timeout 前 300ms 时点静默——TERM 被 trap 忽略，不存在自然死亡路径
	select {
	case code := <-exitCh:
		t.Fatalf("exitf(%d) 在 stop-timeout(%v) 前到达——TERM 应被 trap 忽略，无致死路径", code, stopTimeout)
	case <-time.After(300 * time.Millisecond):
	}
```

**Analog E:** ValidateOptions 表驱动行（options_test.go:17-46——sess 维度第三规则的测试锁加行位；`wantErrSub` 互斥子串分锁形态）:

```go
	{"per-client nil SpawnFunc refused", Options{SessionMode: SessionModePerClient}, "requires SpawnFunc"},
	{"shared non-nil SpawnFunc refused", Options{SessionMode: SessionModeShared, SpawnFunc: spawnFunc}, "must not set SpawnFunc"},
```

**夹具纪律红线**（stopseq_test.go:30-31 逐字）：客户端 Read 永不带 deadline ctx——静默窗口一律 select + time.After 竞速形态。断开/子死竞态注入（Pitfall 3 双路收口）与 reaped 栅栏白盒测（Pitfall 2）的内部件暴露面经 export_test.go 先例（:3 文件头纪律 + ShrinkOutboxForTest :26-39「调用方不得持 hubMu」注释形态）——新增出口全部走该文件。

---

### 9. `web/uat/phase11.mjs`（NEW——协议层 UAT 八场景）

**Analog A:** phase06.mjs 全文骨架（母本——生命周期 UAT 先例：头注释红线纪律 :1-19、check/skip 形态 :51-64、assertOutputClean 自净断言 :483-488、场景数组 + 串行 runner + 汇总退出码 :490-509）:

```js
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
```

**Analog B:** startWesh + dialHello + collectUntilClose 三件套（phase06.mjs:89-118 / :123-142 / :163-170——spawn 真实二进制 + stdout 启动行解析 + WS 握手 + 帧收集器；S1 双端 attach / S5 EXIT 私有化 / S6 容量再闸的失败面断言全部经 collectUntilClose 收 Error 帧 + close code）:

```js
function dialHello(port, { ticket, cols = 80, rows = 24 } = {}) {
  // …… ws.onopen = () => ws.send(helloFrame({ ticket, cols, rows }));
  // Welcome 到达即视为握手完成；10s watchdog 防挂死
}
```

**Analog C:** readPid 正则锚定（phase06.mjs:423-433——S1 双端独立 pid 断言的直接母本：per-client 下两端 `echo S1PID=$$` 各自回读，pid **不等** = 独立进程强证据——S6c 的判定方向恰好反转，注释须锚定语义差异防误抄）:

```js
    const readPid = async (frames, ws) => {
      const base = frames.length;
      ws.send(concat(new Uint8Array([INPUT]), enc.encode('echo S6PID=$$\r')));
      // …… /S6PID=(\d+)/ 正则数字锚定——回显含命令原文（无数字不命中），正则只命中结果行
    };
```

**Analog D:** echo 标记回读（phase06.mjs:371-381 S5b 唯一标记形态——S2 首帧 winsize = Hello 钳制尺寸的断言通道：`stty size` 回读 + 与 Hello cols/rows 逐字比对；无 80×24 中间态由「首帧即可断言正确尺寸」承载）。

**红线沿用**（phase06.mjs:11-13 逐字）：token/凭据值只作断言材料，永不进 check detail/控制台输出；detail 只打印状态码/布尔/形状/退出码/文案常量。S4 断开的 1006 形态按平台豁免条款登记 skipped + reason（CODEBUDDY.md 分层测试策略 §5——真实 OS 断网时序不列阻塞项；协议层可覆盖形态 = ws.close 正常关闭 + 服务端侧 pgid 断言）。运行纪律：宽限/退避类场景真实等待、超时上限只做护栏（phase06.mjs :354-356 时序容差论证先例）；node 运行入口 `node web/uat/phase11.mjs [wesh 二进制路径]` 与 argv 默认值形态照 :19/:24。

---

## Shared Patterns

### S1. EXIT 直写纪律（组帧一次 / 2s ctx / Close 1000 / 禁 outbox 异步）
**Source:** `internal/server/server.go:1391-1434` + `internal/proto/proto.go:186-192`（ExitFrame 注释自带红线）
**Apply to:** perclient.go sessionWatcher（EXIT 私有化）
- 组帧一次共享只读引用（P5-1）；同步 `Write(wctx 2s)` → `Close(1000)` 同 goroutine 内序（wire 序恒 EXIT 在 1000 前）；**绝不** `cl.outbox.trySend(exitFrame)`——writer drain 与 Close 关闭帧竞态超车 = 客户端收 1000 却无退出码（Anti-Pattern 6）。
- Write 失败不补救直接 Close（进程已退出场景无需保帧，server.go:1399-1400 授权先例）。

### S2. stop-signal 序列每会话化（Options 单一通道）
**Source:** `internal/server/clients.go:919-924`（stopChildLocked）+ `internal/server/server.go:1508-1512`（Shutdown 复用同字段先例）
**Apply to:** teardown helper、detach/kick SIGHUP 挂点
- `SignalGroup(s.stopSignal)` 默认 HUP = milestone 字面语义；`stopTimeout > 0` 则 `time.AfterFunc` 异步补 SIGKILL——不占 hubMu、ESRCH 幂等静默、timer 随会话消亡；配置经 Options.StopSignal/StopTimeout 单一通道（双写即漂移，07-04 纪律）。
- SignalGroup 不取 fdMu → hubMu 内发送安全；Drain/Close/Wait 慢半段**绝不**在 hubMu 内（Pitfall 3 行头阻塞红线）。

### S3. logEvent/emitEvent 事件 schema（四段 + 零敏感值）
**Source:** `internal/server/log.go:93-103` + 调用点先例 `server.go:853`（max_clients）/ `:874`（subprotocol_required）
**Apply to:** spawn_failed、容量再闸 max_clients 事件
- `logEvent(remote, code, reason, remoteUser)` 四段既有 schema；事件名细分分辨率（wire 面同码同串 server_error/1011，日志面 spawn_failed vs max_clients 区分——D-02/D-04 的「wire 聚合、日志细分」分工，SEC-07 先例同构）。
- 红线：token/ticket/凭据永不入参（log.go:85-89）；spawn 失败审计行零 pid/client_id（Pitfall 5 清理清单——失败点在注册之前，无关联键可挂）。

### S4. P5-1 别名红线 + outbox 踢出机械复用
**Source:** `internal/server/clients.go:399-417`（onChunk）+ `internal/pty/io.go:11-15`（ReadLoop 缓冲复用注释）
**Apply to:** ReadLoop 闭包（每会话输出路径）
- ReadLoop 回调同步复用底层缓冲——跨帧持有唯一拷贝点 = 组帧 `make([]byte, 1+len(chunk))` + copy；outbox 绝不直接存 chunk。
- trySend 失败唯一处置 = hubMu 内 kickSlowConsumerLocked（1013 既有机械零改动；arbiter 零值下 removeMember/recalcNow 天然 no-op——Go nil map delete 安全，研究 §1.1）。

### S5. Welcome 恒首帧 / 注册时序
**Source:** `internal/server/server.go:1012-1024` + `:968-978`（不变量论证注释块）
**Apply to:** 升档 per-client 分支
- Welcome 入队先于 registerLocked + pcSessions 登记，全程持 hubMu；会话 goroutine 群（ReadLoop 闭包/inputWriter/sessionWatcher/writer/pinger）注册后启动；spawn→ReadLoop 启动间输出由 64KiB PTY 内核缓冲承接（研究 §3.1 约束 2）。
- spawn 失败 = Error 帧直写（握手期 Error 直写先例 :935-937——注册前维持直写不变）+ 零注册零登记零残留（Pitfall 5）。

### S6. 锁序与 goroutine 纪律
**Source:** `internal/server/server.go:83-96`（hubMu/hubCond 注释块）+ `internal/pty/spawn.go:25-31`（fdMu 注释）+ 研究 §5
**Apply to:** 全部新代码
- 全序保持：`hubMu > outbox.mu`、`hubMu > sess.fdMu`，无新锁类型；per-client 新增字段（pcSessions/pc.exitCode 等）全部 hubMu 保护（pinger pongTimedOut 置位先例同形态，server.go:1243-1245）。
- **hubMu 绝不横跨 spawn**（Anti-Pattern 1：fork/exec 阻塞冻结全控制面）——闸内读计数 → 放锁 spawn → 再取锁注册 + D-03 复检回收兜底竞态窗口。
- goroutine 拓扑：New 全局 per-client 本阶段零钉死（pcSupervisor 归 Phase 13）；每会话 5 件随 attach 装配（ReadLoop 闭包/inputWriter/writer/pinger/sessionWatcher）+ reader 读循环 = 6/客户端。

### S7. 零回归收口闸 + 注释纪律
**Source:** 10-PATTERNS.md S4 + CONTEXT.md 已锁定不重复决策条
**Apply to:** 全部修改面
- shared 分支逐字不动（每处模式分岔 if/else 的 else 侧 = 现状字节序）；每阶段收口闸 = shared 全量 Go 测试原样绿 + phase02-09.mjs 默认模式零修改重跑 + 期望值逐字未动 diff 审查；**禁止断言放宽成「两模式都接受」**。
- 新代码注释密度对齐现状：决策号登记（D-01..D-06/PC-02..04）+ file:line 论证锚点 + 红线出处（Pitfall/Anti-Pattern 编号）；per-client 不分支的 shared 机制（maybeExitWhenEmptyLocked 等）补显式注释锚定「per-client 语义归 Phase 13」（CONTEXT specifics :128 缺口登记）。

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `perclient_test.go` SpawnFunc 失败桩被真实调用的装配形态 | test | — | 全代码库**无任何测试调用过 SpawnFunc**——Phase 10 options_test.go:18 空闭包只断言校验分派、从不执行（inert 纪律 T-10-01c）。本阶段首次装配 `New(nil, exitf, Options{SessionMode: per-client, SpawnFunc: 失败桩})` 且 stub 返回 error → 断言 Error{server_error} + 1011 + spawn_failed 事件 + 他端零感知（Pitfall 5 清理清单逐条）。建议形态 = §8A harness 变体（sess=nil）；失败桩签名对齐 `func(cols, rows int) (*pty.Session, error)`。连带缺口：killServer（e2e_test.go:128-134）以单 sess 为参，per-client N 会话收口需新夹具（追踪全部已 spawn *pty.Session 逐一 Kill+Close——泄漏实证注释 :123-127 必须随新夹具重现）。 |
| `phase11.mjs` pgid ESRCH 断言通道 + 运行期删命令注入 | UAT script | — | phase06.mjs 的进程级断言只有 waitExit 收码（:152-157，S3d :311-315）；「断开 → pgid ESRCH 无僵尸」（S4）与「`trap '' HUP` + stop-timeout → KILL 兜底后 pgid ESRCH」（S8）需 Node `process.kill(-pgid, 0)` catch ESRCH 轮询形态——无脚本先例（setsid 不变量使 pgid==pid，`echo $$` 回读的 pid 即 pgid 锚点，readPid §9C 复用）。S3 spawn 失败注入 = 启动期 LookPath 预检通过后、attach 前 unlink argv[0] 副本（tmp 目录 cp 后删）触发 exec 失败路径——亦无先例；最近似 = phase06.mjs S2 `sh -c` 内嵌命令夹具形态（:263）。Go 侧对应物 = stopseq_test.go:122-133 时序双断言形态可镜像到 UAT 层（KILL 兜底前静默窗 + 到期后护栏内 ESRCH）。 |
| `perclient.go` reaped 栅栏锁归属 | service | — | Pitfall 2 只锁「信号与 reap 锁内序列化」语义，实现选型 CONTEXT 已列 Discretion（每会话状态锁 vs hubMu 内标志位）。最近似形态 = pty.Session.Close 的 fdMu+closed 幂等（io.go:65-76，每会话锁 + 置位检查）与 pongTimedOut 的 hubMu 置位/同锁读（server.go:1243-1245 + clients.go:792-793）；pc.exitCode 已确定 hubMu 保护（研究 §2.1），reaped 与 exitCode 同锁（hubMu 内标志位）可免新锁类型——与 S6「无新锁类型」纪律自洽，建议形态以此为准。 |

---

## Metadata

**Analog search scope:** `internal/server/`（server.go / clients.go / log.go / export_test.go / e2e_test.go / exit_test.go / stopseq_test.go / options_test.go）、`internal/pty/`（spawn.go / io.go / reap_darwin.go / reap_linux.go）、`internal/proto/proto.go`、`web/uat/phase06.mjs`、`web/src/main.ts:900-975`、`.planning/phases/10-mode-assembly/10-PATTERNS.md`（文档形态先例）
**Files scanned:** 16（全文精读 12 + 定向段落 2 + 规划文档 2；CONTEXT 行号与实测偏差的全部落点已逐一复核）
**Pattern extraction date:** 2026-09-03
