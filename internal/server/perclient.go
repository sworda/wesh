package server

// perclient.go —— per-client 会话生命周期主干（11-01/11-03，PC-02/PC-03/PC-04，
// D-01/D-02/D-03/D-04；研究 ARCHITECTURE §2.1/§3.1/§3.5/§4.1/§5 与 PITFALLS
// Pitfall 2/3/4/5 锚点登记）：
//
//   - pcSession：单客户端会话的全部服务端状态（pty.Session + 每会话输入
//     队列 + 收口信号集 + 收割/终结同步边）；
//   - upgradePerClient：Attach 升档 per-client 分支——spawn 点 = ticket
//     核销之后 / Welcome 组帧之前 / hubMu 之外（SEC-08 + Anti-Pattern 1/3）；
//     11-03 落地 D-02 pre-spawn 容量再闸（1011+容量文案 wire 形态）与
//     D-03 注册点复检回收（「并发子进程数 ≤ maxClients」硬不变量）；
//   - startSessionGoroutines：五 goroutine 装配（writer/pinger/ReadLoop
//     闭包/inputWriter/sessionWatcher——注册后启动，Welcome 恒首帧纪律）；
//   - sessionWatcher：每会话唯一收割者（cmd.Wait 仅此处调用——断开路径
//     绝不自己 Wait）+ EXIT 私有化直写（组帧一次 → 同步 Write 2s ctx →
//     Close(1000)，禁 outbox 异步——Anti-Pattern 6）；
//   - teardownPCLocked：D-01 固定序列恰好一次（sync.Once）——SIGHUP（经
//     reaped 栅栏，Pitfall 2）→ stopTimeout>0 则 AfterFunc 补 SIGKILL
//    （栅栏同构覆盖补 KILL 发信号点，planner 裁定：Pitfall 2「信号与 reap
//     锁内序列化」语义对一切 kill(-pgid) 同构适用，不只 SIGHUP）→
//     Drain(200ms) → Close(master) → 等待收割返回（waitDone）→ pcSessions
//     单点移除；慢半段绝不占 hubMu（Pitfall 3 行头阻塞红线）。
//   - reapOrphanSession：D-03 复检淘汰孤儿会话的异步回收（SignalGroup(HUP)
//     → AfterFunc 补 KILL（局部 reaped 原子闸）→ Drain → Close → Wait——
//     该会话的唯一收割者，11-03）。
//
// 窗口期登记（D-04 / 11→13 已知中间态，均明示接受非缺口）：watcher 不 emit
// session_start/session_end（per-client 粒度审计归 Phase 13 一次补齐，
// client_id 关联键）；--once/--exit-when-empty 本阶段永不退出（第二终结源
// pcSupervisor/pcExitReq 归 Phase 13，Pitfall 1——maybeExitWhenEmptyLocked
// 早退守卫见 clients.go）。

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
)

// pcSession 是 per-client 模式单客户端会话的全部服务端状态（研究 §2.1 参考
// 实现 + D-01/Pitfall 2 要求的 waitDone/teardownDone/reaped 三件——研究 §2.1
// 未含，本阶段新增）。
//
// 写一次纪律（client 结构 remoteUser 注释形态同构）：sess/inQ/inputDone/
// waitDone/teardownDone/startedAt 写一次于 attach 升档（registerLocked 与
// pcSessions 登记之前），此后读者为该会话自有 goroutine 群（ReadLoop 闭包/
// inputWriter/sessionWatcher/teardown 慢半段）与 hubMu 内读者——
// happens-before 由 goroutine 启动 + hubMu 建立。exitCode/reaped 为运行期
// 可写字段，hubMu 保护（pinger pongTimedOut「置位取 hubMu 写、detach 同锁读」
// 先例同形态——与 hubMu 同锁免新锁类型，S6 纪律）。
type pcSession struct {
	sess *pty.Session
	inQ  *inputQ // 每会话独享（newInputQ(defaultInputQueueBytes)——容量常量复用）
	// inputDone 为该会话 inputWriter 的收口信号（shared s.inputDone 同构）：
	// sessionWatcher 在 teardown 落定后 close——master 已经 Drain→Close，在途
	// Master.Write 经 runtime poller 解除阻塞返回错误（D-12 同款语义）。
	inputDone chan struct{}
	// waitDone 为 watcher 的 Wait 返回即 close 的同步边——teardown 慢半段经它
	// 等待收割完成（唯一收割者纪律每会话粒度保持：断开路径绝不自己 Wait，
	// 只等本信号）。
	waitDone chan struct{}
	// teardownDone 为 teardown 慢半段落定信号（Drain/Close/pcSessions 移除
	// 完成后 close）——sessionWatcher 等它落定再发 EXIT（终结输出先经 outbox
	// 送达的时序与 lifecycle 同序）。
	teardownDone chan struct{}
	// startedAt 为会话起点（spawn 成功时刻）：Phase 13 per-client session_end
	// duration_seconds 数据源预留（shared s.startedAt 同构；D-04 窗口期本
	// 阶段零消费方）。
	startedAt time.Time
	// exitCode 为 watcher 收割的退出码（hubMu 保护）：信号死亡 = -1
	//（exec.ExitError ExitCode 语义，lifecycle 退出码提取同形）；本阶段
	// 零读者——supervisor 消费归 Phase 13。
	exitCode int
	// reaped 为「已被 Wait 收割」栅栏位（hubMu 保护，Pitfall 2）：一切
	// kill(-pgid) 发信号点（SIGHUP 与 AfterFunc 补 KILL 两处）只在本位为
	// false 时发送——信号与 reap 经 hubMu 内标志位序列化，kill-after-reap
	// 误杀复用 pgid 结构性不可能。
	reaped bool
	// teardownOnce 收口双触发路径（detach/kick 断开挂点 × watcher 子死
	// 路径）——两路径都只「触发」，执行序列只有一个（Pitfall 3 恰好一次）。
	teardownOnce sync.Once
	// 12-02（PC-05）RESIZE 直通三字段：resizeMu 为 pendingResize 的叶锁
	// （锁序三规则 §5——持有期间不取任何其他锁；读循环写 pendingResize 与
	// 防抖回调读 pendingResize 经它互斥，Reset 与回调执行的并发安全由
	// Go 1.23+ timer 语义承接）；pendingResize 为最新上报尺寸（防抖窗内
	// 合并，到期只应用最后值——arbiter 同款语义，Pitfall 7 SIGWINCH 风暴
	// 防线每会话粒度）；resizeDeb 为每会话防抖（共用件 debouncer，时长源
	// s.resizeDebounce 与 arbiter 同源——不新增第二份常量，防双写漂移）。
	// 回调锁序可证：resizeMu 内取 pendingResize → 放锁 → sess.Resize 仅
	// fdMu（hubMu > sess.fdMu 全序不受影响，回调函数体零 hubMu）；closed
	// 会话 Resize 返回 os.ErrClosed 静默（Attach 读循环既有纪律同款）。
	resizeMu      sync.Mutex
	pendingResize dims
	resizeDeb     *debouncer
}

// capacityMessage 是 D-02 容量拒绝的统一文案（11-03）：pre-spawn 闸与注册点
// 复检回收两调用点共用同一字面量单点——wire 面两种容量拒绝不可区分是有意为之
// （D-02「wire 聚合」）；定值常量纪律与 spawn 失败文案同款（绝不携带路径/
// errno/argv/计数器快照等内部状态，Pitfall 5 + Security Mistakes 表红线，
// T-11-03b）。
const capacityMessage = "server is at capacity"

// rejectCapacity 执行容量拒绝序列（D-02 wire 形态单点，11-03）：
// Error{server_error, 容量文案} 直写（握手期 Error 直写先例——server.go
// version_mismatch 形态，注册前维持直写不变）→ max_clients 单行审计（logEvent
// 既有四段 schema：remote/code/reason/remoteUser，零敏感值，S3）→
// Close(1011, server_error)。与 spawn 失败同码 1011 同机器串 server_error
// （协议零改动红线），分辨率由 logEvent 事件名承担（max_clients vs
// spawn_failed——「wire 聚合、日志细分」，SEC-07 先例同构，D-04）。
// 两调用点：upgradePerClient 的 pre-spawn 闸与注册点复检回收分支。
func rejectCapacity(ctx context.Context, c *websocket.Conn, remote, remoteUser string) {
	_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrServerError, capacityMessage))
	logEvent(remote, websocket.StatusInternalError, "max_clients", remoteUser)
	_ = c.Close(websocket.StatusInternalError, proto.ErrServerError)
}

// upgradePerClient 是 Attach 升档的 per-client 分支（研究 §3.1 时序 + PATTERNS
// §1/§6b；调用点 = server.go Attach 分岔，close(helloDone)+release() 已在其前
// 执行——半开名额不泄漏）。拒绝/失败路径自行写 Error+Close 并返回 nil，调用方
// 据此直接 return；成功路径完成装配并启动五 goroutine 后返回 cl。
func (s *Server) upgradePerClient(ctx context.Context, c *websocket.Conn, remote, remoteUser string, h proto.HelloPayload, ticketMode string, cancel context.CancelFunc) *client {
	// 单行门算 effMode（decideModeLocked 不调用——write-policy 仲裁矩阵在
	// per-client 不装配，owner 递补语义不存在）。
	effMode := proto.ModeRO
	if s.writable && ticketMode == proto.ModeRW {
		effMode = proto.ModeRW
	}
	// D-02 pre-spawn 容量再闸（11-03）：hubMu 短临界区只读 len(pcSessions)
	// 计数——绝不持锁 spawn（Anti-Pattern 1：fork/exec 阻塞不得冻结全控制面；
	// 闸内读 → 放锁 spawn → 再取锁注册）。满员即拒（rejectCapacity 序列）。
	// 1013 否决：main.ts:946 前端 1013 分派只认 code 不渲染 reason，固定显示
	// 慢消费者文案（"The session itself is unaffected"——per-client 满员语义
	// 双重错位，前端改动窗口在 Phase 12）；1008 否决：既有语义为认证/版本策略
	// 违反（version_mismatch/auth_failed），容量策略混入污染 1008 受众分治。
	// 与 ③位 HTTP 503 闸（server.go 守卫区，registry.n 计数源）分工不变：
	// 满员且注册表满 → 既有 503（零改动）；注册表空出但 pcSessions 满（断开
	// 待收割 linger 形态）或并发竞态窗口 → 本闸 1011 容量文案。半开名额已在
	// 升档分岔前 release()（server.go Attach close(helloDone)+release() 共用
	// 行结构性保证恰好一次），本拒绝路径零注册零登记零残留。
	s.hubMu.Lock()
	full := len(s.pcSessions) >= s.maxClients
	s.hubMu.Unlock()
	if full {
		rejectCapacity(ctx, c, remote, remoteUser)
		return nil
	}
	// spawn 在 hubMu 之外（Anti-Pattern 1：fork/exec 阻塞不得冻结全控制面）；
	// h.Cols/h.Rows 已经 DecodeHello ClampDim 钳制，满足 StartWithSize
	//「调用方已钳制」契约——直通出生即正确尺寸（无 80x24 中间态，SC1 后半）。
	// SEC-08 + Anti-Pattern 3：checkTicket 成功是 spawn 唯一前置，本调用点
	// 在升档分岔内结构性保证。
	sess, err := s.spawnFunc(h.Cols, h.Rows)
	if err != nil {
		// 失败路径（D-04/Pitfall 5 清理清单）：Error 帧直写（握手期 Error 直写
		// 先例——注册前维持直写不变；message 为定值常量，绝不携带底层错误
		// 细节，Pitfall 5 + Security Mistakes 表红线）→ spawn_failed 单行审计
		//（logEvent 既有通道，S3：四段 schema 零敏感值，无 pid/client_id——
		// 失败点在注册之前，无关联键可挂）→ Close(1011)（D-07 code 与 reason
		// 同名机器串）。收口 = 关连接零残留：零 client 构造、零注册、零
		// pcSessions 登记；spawn 失败由 pty 包保证 fd 完好（Pitfall 5）。
		_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrServerError, "failed to start process"))
		logEvent(remote, websocket.StatusInternalError, "spawn_failed", remoteUser)
		_ = c.Close(websocket.StatusInternalError, proto.ErrServerError)
		return nil
	}
	// 成功路径：构造 pcSession（写一次字段集——见 pcSession 注释）。
	pc := &pcSession{
		sess:         sess,
		inQ:          newInputQ(defaultInputQueueBytes),
		inputDone:    make(chan struct{}),
		waitDone:     make(chan struct{}),
		teardownDone: make(chan struct{}),
		startedAt:    time.Now(),
	}
	// 12-02（PC-05）每会话 RESIZE 防抖装配（共用件 debouncer，构造即 stopped：
	// 首次 RESIZE 上报才武装——server.go RESIZE case per-client 直通分支在
	// resizeMu 内写 pendingResize 后 Reset）。回调：resizeMu 内取 pendingResize
	// → 放锁 → sess.Resize 仅 fdMu（锁序三规则 §5——回调函数体绝不取 hubMu；
	// closed 会话返回 os.ErrClosed 静默）。
	pc.resizeDeb = newDebouncer(s.resizeDebounce, func() {
		pc.resizeMu.Lock()
		d := pc.pendingResize
		pc.resizeMu.Unlock()
		_ = pc.sess.Resize(d.cols, d.rows)
	})
	s.hubMu.Lock()
	// D-03 注册点复检（11-03）：spawn 成功后在注册段的同一 hubMu 持有内复检
	// 容量——竞态窗口 = 两并发升档同时过上方 pre-spawn 闸（Pitfall 4 超编形态：
	// 并发握手各自通过容量检查后同时 spawn，瞬时进程数可超 maxClients）。
	// 复检使「并发子进程数 ≤ maxClients」硬不变量 Phase 11 即成立；Phase 13
	// 裁决项④（spawn-intent 预占/回滚口径）由此提前消解——STATE.md Blockers
	// ④ 已标注消解，Phase 13 规划时移除该开放项。超编者放锁后异步回收
	// （reapOrphanSession——该孤儿会话的唯一收割者），客户端收与 pre-spawn
	// 闸完全相同的容量拒绝序列（同文案同事件名同关闭码——wire 面两种容量
	// 拒绝不可区分是有意为之，D-02「wire 聚合」）。
	if len(s.pcSessions) >= s.maxClients {
		s.hubMu.Unlock()
		s.reapOrphanSession(sess)
		rejectCapacity(ctx, c, remote, remoteUser)
		return nil
	}
	cl := &client{
		conn:       c,
		remote:     remote,
		remoteUser: remoteUser, // 07-03：Attach 入口提取一次，此后只读（clients.go 字段注释）
		// rwEligible 零值 false——per-client 无 owner 递补语义
		//（promoteNextLocked 永不可达：registry.owner 恒 nil）。
		dims:   dims{cols: h.Cols, rows: h.Rows}, // Hello 首尺寸（DecodeHello 已 ClampDim）
		outbox: newOutbox(s.outboxBytes),
		done:   make(chan struct{}),
		cancel: cancel,
		// 每客户端输入限速令牌桶（与 shared 升档同形同值；ro 端同样构造，
		// 无害——INPUT 先过 mode 门）。
		limiter: rate.NewLimiter(rate.Limit(s.inputRate), s.inputBurst),
		inQ:     pc.inQ, // Pattern 2 间接字段：读循环零分支的关键（shared = s.inputQ）
		pc:      pc,     // per-client 会话绑定（detach/kick 的 teardown 触发以 pc != nil 为门）
	}
	cl.mode.Store(effMode) // 生效模式初始值（atomic 承载：INPUT 门无锁读者）
	// Welcome 恒首帧（S5 时序纪律，P2 D-02 同构）：入队先于 registerLocked +
	// pcSessions 登记且全程持 hubMu——本分支 ReadLoop 闭包只投本端 outbox，
	// 注册前绝无帧夹入；goroutine 群注册后启动，spawn→ReadLoop 启动间输出由
	// 64KiB PTY 内核缓冲承接（研究 §3.1 约束 2）。cols/rows 回显本端 Hello
	// 钳制尺寸——不经 sessionDimsLocked（per-client 无仲裁，G-05-1 契约退化
	// 为恒等式）。prefs 双档选档（ro 档永不含 osc52，D-13/P5-6 纪律不动）。
	prefs := s.clientPrefsRO
	if effMode == proto.ModeRW {
		prefs = s.clientPrefsRW
	}
	cl.outbox.trySend(proto.WelcomeFrame(effMode, prefs, h.Cols, h.Rows, s.sessionMode))
	s.registry.registerLocked(cl)
	s.pcSessions[pc] = struct{}{}
	s.hubMu.Unlock()
	// attach 事件（shared 升档同形态同字段集：event=attach + remote +
	// client_id=attachSeq + mode + remote_user 非空出键；registerLocked 分配
	// attachSeq 的写 happens-before 本读）。
	attachAttrs := []slog.Attr{
		slog.String("event", "attach"),
		slog.String("remote", cl.remote),
		slog.Int64("client_id", cl.attachSeq),
		slog.String("mode", effMode),
	}
	if cl.remoteUser != "" {
		attachAttrs = append(attachAttrs, slog.String("remote_user", cl.remoteUser))
	}
	emitEvent(attachAttrs...)
	c.SetReadLimit(proto.ReadLimitPostAuth)
	s.startSessionGoroutines(ctx, cl, pc)
	return cl
}

// startSessionGoroutines 启动 per-client 会话的五件 goroutine（研究 §5 拓扑
// 表：每客户端 6 = reader + writer + pinger + ReadLoop 闭包 + inputWriter +
// sessionWatcher；reader 即 Attach 返回后的读循环本体）。调用点 = 注册完成
// 之后（Welcome 恒首帧论证见 upgradePerClient）。
func (s *Server) startSessionGoroutines(ctx context.Context, cl *client, pc *pcSession) {
	go s.writer(cl)
	go s.pinger(ctx, cl, s.pingInterval)
	// ReadLoop 闭包（研究 §3.5/PATTERNS §6c，每会话输出路径——1:1 直投属主
	// outbox，不复用全局 onChunk 扇出，Pitfall 6 红线）：detach 门提前返回
	//（SIGHUP→死亡窗口期不做无谓组帧）；同一 mc.ptyOutputBytes 计数器聚合
	// 口径不变；每帧 make+copy（P5-1 别名红线——ReadLoop 缓冲复用，
	// pty/io.go:14）；trySend 失败 → hubMu 内 kickSlowConsumerLocked（1013
	// 既有机械零改动复用——arbiter 零值下 removeMember/recalcNow 天然
	// no-op，Go nil map delete 安全，研究 §1.1）。
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
	go inputWriter(pc.sess, pc.inQ, pc.inputDone) // CR-01：读循环永不直写 master（Anti-Pattern 5）
	go s.sessionWatcher(cl, pc)
}

// sessionWatcher 是 per-client 会话的终结者（每会话一个，子进程死亡的唯一
// 感知者——cmd.Wait 唯一收割者纪律每会话粒度保持，断开路径绝不自己 Wait；
// darwin 共享 kqueue watcher 天然 N 会话安全，reap_darwin.go 设计注释逐字）。
//
// 序列（研究 §4.1）：Wait 返回 → 退出码提取（lifecycle 内联三行同形，信号
// 死亡 ExitCode()=-1 语义免费）→ close(waitDone)（teardown 慢半段的收割
// 同步边）→ hubMu 内置 exitCode+reaped 并触发 teardown（同锁持有内——若
// 断开路径已先触发则 Once 去重无操作；若本路径先触发，reaped 已在同临界区
// 置位，快半段栅栏成立）→ 等 teardownDone 落定（Drain/Close/注册表移除
// 完成后再发 EXIT——终结输出先经 outbox 送达的时序与 lifecycle 同序）→
// close(inputDone)（inputWriter 收口，shared close(s.inputDone) 同构）→
// EXIT 私有化直写（S1 直写纪律逐字形态：组帧一次共享只读 → 同步 Write
// 2s ctx → Close(1000)——禁 outbox 异步入队，Anti-Pattern 6 关闭帧超车
// 防线；Write 失败不补救直接 Close，server.go lifecycle 授权先例；与
// Shutdown 1001 竞态由库 Close 幂等承接）。
//
// D-04 窗口期：watcher 不 emit session_end（per-client 粒度审计归 Phase 13
// 一次补齐，client_id 关联键）。
func (s *Server) sessionWatcher(cl *client, pc *pcSession) {
	err := pc.sess.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	close(pc.waitDone)
	s.hubMu.Lock()
	pc.exitCode = code
	pc.reaped = true
	s.teardownPCLocked(pc)
	s.hubMu.Unlock()
	<-pc.teardownDone
	close(pc.inputDone)
	exitFrame := proto.ExitFrame(code, exitMessage(err, code))
	// 2s Write 超时（RESEARCH OQ3 定值，常量纪律同 lifecycle）：stall/慢链路
	// 2s 未写完 ~100B EXIT 帧即放弃直写，该端退化为 1000 + 前端硬编码回退
	// 文案（R2 回退路径既有，非致命）。
	wctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = cl.conn.Write(wctx, websocket.MessageBinary, exitFrame)
	cancel()
	_ = cl.conn.Close(websocket.StatusNormalClosure, "")
}

// teardownPCLocked 是 per-client 会话的 D-01 固定 teardown 序列，恰好一次
// （sync.Once；三调用点：detach 挂点、kick 挂点、sessionWatcher 同锁持有
// 内——两触发路径都只「触发」，执行序列只有一个，Pitfall 3）。调用方必须
// 已持 hubMu（Locked 后缀纪律）。
//
// 快半段（hubMu 持有内同步执行）：reaped 栅栏内发 s.stopSignal（默认 HUP =
// milestone 字面语义，OPS-04 --stop-signal 可配通道自然继承；SignalGroup
// 不取 fdMu，hubMu 内发送安全，锁序 hubMu > sess.fdMu 不受影响）且
// stopTimeout>0 时 AfterFunc 补 SIGKILL——闭包内先取 hubMu 复检 !reaped
// 才发送（planner 裁定：栅栏覆盖补 KILL 发信号点；pinger pongTimedOut
// 置位取 hubMu 先例同形态；timer 随会话消亡，ESRCH 幂等静默）。
//
// 慢半段（独立 goroutine——Drain/Close/waitDone 等待绝不在 hubMu 内，
// Pitfall 3 行头阻塞红线）：Drain(200ms) → Close(master) → <-waitDone
// （watcher Wait 返回信号）→ hubMu 内 delete(pcSessions)（活性记账单点
// 移除——map delete 幂等）→ close(teardownDone)。
func (s *Server) teardownPCLocked(pc *pcSession) {
	pc.teardownOnce.Do(func() {
		// 快半段：每会话 RESIZE 防抖停摆（12-02——计时器随会话消亡；teardown
		// 后在途 RESIZE 再武装亦无害：AfterFunc 回调对 closed 会话 Resize 返回
		// os.ErrClosed 静默，双防线）。
		pc.resizeDeb.Stop()
		// 快半段：信号面（Pitfall 2 reaped 栅栏——收割完成后一切 kill(-pgid)
		// 禁发，kill-after-reap 误杀复用 pgid 结构性不可能）。
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
		// 慢半段：阻塞面移出 hubMu。
		go func() {
			pc.sess.Drain(200 * time.Millisecond)
			_ = pc.sess.Close()
			<-pc.waitDone
			s.hubMu.Lock()
			delete(s.pcSessions, pc)
			s.hubMu.Unlock()
			close(pc.teardownDone)
		}()
	})
}

// reapOrphanSession 异步回收 D-03 注册点复检淘汰的孤儿会话（11-03）。唯一收割
// 者纪律：该会话从未注册 pcSessions、从未装配 goroutine 群（无 sessionWatcher）
// ——本 goroutine 是其 Wait 的唯一调用方（每会话恰好一个 Wait 调用方：正常路径
// = sessionWatcher，回收路径 = 本 goroutine，两路径互斥不可同时装配）。
//
// 序列参照 teardownPCLocked 的信号纪律（SignalGroup → stopTimeout>0 则
// AfterFunc 补 SIGKILL → Drain(200ms) → Close(master) → Wait 收割），两处
// 差异锚定：①信号恒 syscall.SIGHUP 字面（D-03——容量回收路径非断开语义，
// 不走 --stop-signal 通道）；②reaped 闸用局部 atomic.Bool 而非 hubMu 内标志位
// ——该会话无 watcher 无注册，无第二置位/读取方，局部原子量即达到 Pitfall 2
// 「信号与 reap 序列化」同构语义，且避免回收 goroutine 与 hubMu 的无谓耦合
// （11-03 flagged_assumptions 登记形态，与主 teardown 路径 hubMu 内标志位同档）。
//
// AfterFunc 回调先查 reaped 才补 KILL——Wait 返回后补 KILL 可能打中复用 pgid
// （Pitfall 2 kill-after-reap）。窄窗口论证：Wait 返回与 reaped.Store(true)
// 相邻两指令，窗口内 AfterFunc 恰到期且 pgid 已被内核整轮复用分配，实际不可达
// （与主 teardown 路径 hubMu 置位同档形态）。
//
// 全程异步（单 goroutine）——Drain/Close/Wait 阻塞面绝不占 hubMu（Pitfall 3
// 行头阻塞红线）；调用点 = 注册点复检放锁之后（upgradePerClient）。
func (s *Server) reapOrphanSession(pcSess *pty.Session) {
	go func() {
		var reaped atomic.Bool
		pcSess.SignalGroup(syscall.SIGHUP)
		if s.stopTimeout > 0 {
			time.AfterFunc(s.stopTimeout, func() {
				if !reaped.Load() {
					pcSess.SignalGroup(syscall.SIGKILL)
				}
			})
		}
		pcSess.Drain(200 * time.Millisecond)
		_ = pcSess.Close()
		_ = pcSess.Wait()
		reaped.Store(true)
	}()
}
