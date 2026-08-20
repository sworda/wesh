// clients.go：多客户端注册表 + fan-out hub + 每客户端 outbox/writer（MULTI-01/03
// 主干，CONTEXT domain 必然推论——P1 D-11 单次语义终结；RESEARCH Pattern 1/2/5
// 定稿形态，P5-1 chunk 别名与 P5-2 踢出不内联两条红线纪律）+ 全局信用门
//（RES-04，RESEARCH Pattern 3：全体可写端 outbox 均满 → hub 持块停读 PTY，
// 任一可写端 drain 至半水位恢复）+ 写权限体系（MULTI-02，05-03：模式判定矩阵 +
// owner FIFO 递补升格，RESEARCH Pattern 5 逐字落地）。
//
// 锁序纪律（R-07）：hubMu > outboxMu——writer 持 outboxMu drain 期间绝不取 hubMu；
// drain 完释放后才取 hubMu 做 afterDrain 恢复判定，绝不反序同持。信用门 cond
// 挂 hubMu（信用门状态与注册表同锁），outbox 自有 mu。
package server

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
)

// 多客户端五个测试可覆写参数的默认常量（R-01 初值；P2 D-10 常量纪律——一律常量
// 不开 CLI flag，全部挂 Phase 9 负载测试标定回填）。声明在本文件（hub/outbox/限速
// 均属多客户端关切面），server.go New 的零值兜底逐字段引用。
const (
	// defaultOutboxBytes 每客户端 outbox 字节容量默认值（512KiB：16×32KiB 读块，
	// 100KB/s 慢链路 ~5s 抖动容忍）。Phase 9 负载测试标定回填（P2 D-10 纪律）。
	defaultOutboxBytes = 512 * 1024
	// defaultMaxClients 最大并发客户端数默认值（32）。Phase 9 负载测试标定回填
	//（P2 D-10 纪律）。本 plan 仅立字段与常量，消费点（守卫区③位 503 闸）05-07 落地。
	defaultMaxClients = 32
	// defaultInputRate 每客户端输入限速速率默认值（32KiB/s：人类击键 ~10B/s，
	// 持续 32KiB/s 远超合法、远低于洪水）。Phase 9 负载测试标定回填（P2 D-10
	// 纪律）。本 plan 仅立字段与常量，消费点（INPUT 门 AllowN）05-05 落地。
	defaultInputRate = 32 * 1024
	// defaultInputBurst 每客户端输入限速突发默认值（64KiB：容纳一次快粘）。
	// Phase 9 负载测试标定回填（P2 D-10 纪律）。本 plan 仅立字段与常量，
	// 消费点 05-05 落地。
	defaultInputBurst = 64 * 1024
	// defaultResizeDebounce resize 仲裁防抖默认值（50ms，CONTEXT 已锁定；
	// PITFALLS Pitfall 10 SIGWINCH 风暴防线）。Phase 9 负载测试标定回填
	//（P2 D-10 纪律）。本 plan 仅立字段与常量，消费点（仲裁器单 time.Timer
	// reset）05-04 落地。
	defaultResizeDebounce = 50 * time.Millisecond
)

// WritePolicy 取值（D-05 公开 CLI 契约——--write-policy=owner|all，全名无短选项
// P2 D-15；main.go parse 期枚举校验用同一对常量，防双写漂移）：
// owner = 首个以 rw 身份 attach 的客户端为 owner、后续 rw 降级 ro 进 FIFO 递补
// 队列（D-06/D-07，安全默认哲学：旁观是被动场景、协作主动开启）；
// all = 全员可写（服务协作排障形态，无递补概念）。
const (
	WritePolicyOwner = "owner" // 默认（D-05 安全默认）
	WritePolicyAll   = "all"
)

// client 是一个已注册 WS 客户端的全部服务端状态。writer goroutine 是该连接全程
// 唯一 WS 写端（pinger 的控制帧经库 writeFrameMu 与数据写串行化，既有 02-04 纪律）。
type client struct {
	conn   *websocket.Conn
	remote string // logEvent 三要素之对端（Attach 入口保存的 RemoteAddr）
	mode   string // proto.ModeRO/ModeRW——INPUT/RESIZE 门 per-client 判定（P2 D-13/D-14 的多客户端映射）
	// rwEligible 递补候选资格（D-06「以 rw 身份」）：rw ticket × owner 模式 → true
	//（含 owner 本人——语义为「持 rw 身份的资格」，owner 断线后该客户端若仍在队
	// 即可被递补）；ro ticket 与 all 模式恒 false（ro 永不递补；all 无递补概念）。
	rwEligible bool

	attachSeq int64 // attach FIFO 序号（registerLocked 分配；05-03 owner 递补取序）
	outbox    *outbox
	done      chan struct{}      // writer 终结信号——kick/detach 关闭，恰好一次由 hubMu 内注册表成员判定保证
	cancel    context.CancelFunc // pinger 所在 ctx 的 cancel（Attach 派生，随客户端生命周期）

	// creditBlocked 信用门阻塞位（hubMu 保护）：仅可写端参与信用集，ro 客户端
	// 此位恒 false——ro 满即踢永不持信用（R-08 分工表）。kickOrCreditLocked 置位、
	// afterDrain 清位，两点各递增 registry.gateTransitions。
	creditBlocked bool
	// creditPending 计入信用时被拒的触发帧（hubMu 保护）：trySend 失败 ≠ 丢帧
	//（must_haves prohibitions——丢帧保连接 = 有序流画面静默损坏）——该帧暂存于此，
	// afterDrain 半水位恢复时先于清位/Broadcast 重投，门重开后的一切新帧经 onChunk
	// 门判定排在其后（有序性成立）。共享只读堆帧引用（P5-1 纪律不变——非 ReadLoop
	// chunk）。nil = 无待重投帧；kick/detach 随客户端消亡自然作废。
	creditPending []byte
}

// outbox 是每客户端字节有界输出队列（RESEARCH Pattern 2 逐字形态）：存 hub 组好的
// 共享只读帧引用，逐客户端只记字节账——共享帧使全局 WS 出站内存 ≈ 最慢者滞后量
// 而非 Σ。notEmpty 为 cap 1 信号量：trySend 非阻塞投递，writer 阻塞消费。
type outbox struct {
	mu       sync.Mutex
	q        [][]byte // 共享帧（hub 分配、只读，引用计数靠 GC 自然回收）
	bytes    int
	cap      int
	notEmpty chan struct{}
}

// newOutbox 构造字节容量为 cap 的 outbox（cap 由 s.outboxBytes 供给，测试可覆写）。
func newOutbox(cap int) *outbox {
	return &outbox{cap: cap, notEmpty: make(chan struct{}, 1)}
}

// trySend 非阻塞投递共享帧：超容量返回 false（调用方唯一处置 = 1013 踢出该客户端，
// 绝不丢帧保连接——有序字节流丢一段转义序列 = 客户端画面静默损坏，RESEARCH
// Anti-Pattern 2）；成功则 append + 记账 + 非阻塞信号（已有信号在飞则 writer 必会
// drain 到本帧，select default 不阻塞）。
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

// drain swap 出整队并重置字节计数。信用门半水位恢复判定（afterDrain）在写出成功
// 后重新读当前字节——drain 返回的预重置计数不参与门判定（写出期间新入队的部分
// 才是迟滞带语义的承载）。
func (o *outbox) drain() (batch [][]byte, bytes int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	batch, bytes = o.q, o.bytes
	o.q = nil
	o.bytes = 0
	return batch, bytes
}

// registry 客户端注册表（R-07）：set 供 hub 扇出遍历，order 保 attach FIFO 序
//（05-03 owner 递补队列的遍历序），seq 为 attachSeq 分配器。全部方法名以 Locked
// 结尾——调用方必须已持 hubMu（注册表与 hub 共用单锁，R-07 单锁纪律）。
type registry struct {
	set   map[*client]struct{}
	order []*client
	seq   int64
	// owner 当前写权限持有者（D-06：首个以 rw 身份完成 attach 的客户端）；
	// 仅 owner 模式使用——all 模式恒 nil（全员可写无递补概念），无 --writable
	// 纯 ro 会话恒 nil。nil = 无 owner（下一个 rw attach 按矩阵成为新 owner）。
	// hubMu 保护（注册表同锁，R-07）。
	owner *client
	kicks int // 1013 踢出计数（Phase 8 OPS-07 观测性挂点，review #10；hubMu 保护，单锁纪律 R-07 下无需 atomic）
	// gateTransitions 信用门开闭周期计数（kickOrCreditLocked 置位 creditBlocked 与
	// afterDrain 清位两点递增）：Phase 8 OPS-07 门开闭周期计数挂点（review #10）；
	// hubMu 保护，零 atomic 不违 R-07 单锁纪律。
	gateTransitions int
}

// registerLocked 登记新客户端：分配 attachSeq、入 set 与 FIFO order。
func (r *registry) registerLocked(c *client) {
	r.seq++
	c.attachSeq = r.seq
	r.set[c] = struct{}{}
	r.order = append(r.order, c)
}

// removeLocked 移除客户端：同时清理 map 项与 slice 项（Pitfall 4 双容器防单调
// 增长）。返回是否移除成功——非成员幂等 false（kick 与 detach 互斥恰好一次的
// 判定依据：先完成移除的一方负责 close(done)/cancel）。
func (r *registry) removeLocked(c *client) bool {
	if _, ok := r.set[c]; !ok {
		return false
	}
	delete(r.set, c)
	for i, oc := range r.order {
		if oc == c {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return true
}

// decideModeLocked 模式判定矩阵（MULTI-02，RESEARCH Pattern 5 表逐字落地）：
// 输入 = ticket 绑定 mode（认证模式为 ticket 绑定值 D-11，无认证模式为 s.writable
// 派生值）× --writable × write-policy × owner 是否在位；输出 = 生效 mode +
// rwEligible + 是否成为 owner。调用方必须已持 hubMu（读 s.registry.owner）。
//
//	rw ticket × owner 模式 × owner 不在位 → ModeRW + 立 owner（D-06）
//	rw ticket × owner 模式 × owner 在位   → ModeRO + rwEligible=true（D-07 降级
//	                                        进递补队列——registry.order 切片天然 FIFO 尾）
//	rw ticket × all                       → ModeRW（全员可写，无递补概念）
//	ro ticket × 任意                      → ModeRO（rwEligible=false——ro ticket
//	                                        永不递补，D-06「以 rw 身份」）
//	无 --writable × 任意                  → ModeRO（D-05 总闸，现状语义零漂移）
//
// 权限不得由客户端请求获得（must_haves prohibitions——T-05-08 越权面）：ro→rw
// 唯一通道是服务端 FIFO 递补后的 Welcome 推送（promoteNextLocked），任何
//「客户端发帧申请写权限」的机制都是越权面，本矩阵不提供也不接受此类输入。
// writePolicy 取值在 main.go parse 期已枚举校验（owner|all）；非 "all" 一律按
// owner 语义收口（安全默认方向兜底）。
func (s *Server) decideModeLocked(ticketMode string) (mode string, rwEligible bool, becomeOwner bool) {
	if !s.writable || ticketMode != proto.ModeRW {
		return proto.ModeRO, false, false // D-05 总闸 / ro ticket 永不递补
	}
	if s.writePolicy == WritePolicyAll {
		return proto.ModeRW, false, false // all：全员可写，无递补概念
	}
	if s.registry.owner == nil {
		return proto.ModeRW, true, true // D-06：首个 rw attach 成为 owner
	}
	return proto.ModeRO, true, false // D-07：降级 ro + 进递补队列（order FIFO 尾）
}

// onChunk 是 S→C fan-out hub（ReadLoop 唯一读者经 sess.ReadLoop 回调，D-12 drain
// 语义不变——无客户端时遍历空集自然丢弃，防 PTY 内核缓冲写阻塞）。
//
// 别名红线（P5-1，pty/io.go:13-14 注释明示"onChunk 复用底层缓冲——回调方如需跨帧
// 持有须自行拷贝"）：每 chunk 组帧一次（make+copy 是唯一拷贝点），全部客户端
// outbox 共享该只读帧引用，逐客户端零拷贝；outbox 绝不直接存 chunk。
//
// 代码顺序不变量（review #1，frame 别名安全双锁定之一）：门 Wait 循环必须位于
// 组帧语句（make + copy 组共享帧）之前——门持块期间 chunk 停留于 ReadLoop 缓冲
//（阻塞即无下次读，无别名窗口）；帧拷贝只发生在门开之后、trySend 之前，outbox
// 绝无持有跨门 chunk 的窗口。
//
// 全局信用门（RES-04，RESEARCH Pattern 3）：全体可写端 creditBlocked → hub 在
// hubCond 上 Wait——Wait 原子释放 hubMu；持块即停读 PTY，反压经 64KiB 内核缓冲
// 传导至子进程 write（RES-04 唯一合法反压路径，ARCHITECTURE §2.6）。任一可写端
// writer drain 至半水位 → afterDrain 清位 + Broadcast 恢复。
//
// 门之外的临界区只含非阻塞 trySend 遍历：无锁等待、cond 等待或逐客户端帧拷贝
//（单客户端内存与延迟形态与 Phase 4 等价）。
func (s *Server) onChunk(chunk []byte) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	for s.allWritableBlockedLocked() {
		s.hubCond.Wait() // Wait 原子释放 hubMu；持块即停读 PTY（RES-04）；chunk 停留 ReadLoop 缓冲，无别名窗口（review #1）
	}
	frame := make([]byte, 1+len(chunk))
	frame[0] = proto.Output
	copy(frame[1:], chunk)
	for c := range s.registry.set {
		if !c.outbox.trySend(frame) {
			s.kickOrCreditLocked(c, frame)
		}
	}
}

// allWritableBlockedLocked 判定信用门是否应闭合（RESEARCH Pattern 3）：遍历注册表
// 统计生效 mode==rw 的客户端；≥1 个且全部 creditBlocked → true；无可写端
//（纯 ro 会话/无客户端）→ false（信用集为空 → 门永不闭合 → ro 满即踢，R-08
// 分工表前提）。调用方必须已持 hubMu。O(n) 遍历每 chunk 一次，规模 ≤ max-clients
// 32，可接受（review LOW 项登记）。
func (s *Server) allWritableBlockedLocked() bool {
	writable := 0
	for c := range s.registry.set {
		if c.mode != proto.ModeRW {
			continue
		}
		writable++
		if !c.creditBlocked {
			return false
		}
	}
	return writable > 0
}

// kickOrCreditLocked 是 trySend 写满后的踢出/信用分工判定（R-08 逐字）：
//   - c.mode == ro → 立即 1013 踢出（旁观者可弃，永不持信用）；
//   - c 为 rw 且剔除 c 后仍存在未 blocked 的可写端 → 立即 1013 踢出（慢的是
//     离群者，MULTI-03 本义）；
//   - c 为 rw 且全体可写端均满 → 不踢，置 c.creditBlocked = true（保护 owner
//     演示者，离群慢端才踢；onChunk 下轮门判定闭合 → 持块停读 PTY）。
//
// 触发帧不丢（must_haves prohibitions 硬约束——丢帧保连接 = 有序流画面静默损坏，
// RESEARCH Anti-Pattern 2）：信用路径把被拒的当前帧暂存 c.creditPending，
// afterDrain 半水位恢复时重投（TestGlobalCredit 门转换字节精确断言实测发现：
// 不暂存则恢复端流缺一段——review #1 行为证据锁住的正是此窗口）。
//
// 迟滞带论证（review #2）：门关闭阈值 = outbox 写满（100% 字节上界才置
// creditBlocked）、恢复阈值 = drain 至 <50%——2:1 迟滞带内建于分工表，非
//『50% 单点抖动』；残余振荡（滴漏读者每次 drain 开门、下一 chunk 可能再闭门）
// 经 RESEARCH Open Question 3 裁决接受（各端持续前进；dwell 计时器 Phase 9
// 负载标定时评估）。调用方必须已持 hubMu。
func (s *Server) kickOrCreditLocked(c *client, frame []byte) {
	if c.mode == proto.ModeRO {
		s.kickSlowConsumerLocked(c)
		return
	}
	// rw：剔除 c 后仍存在未 blocked 的可写端 → c 是离群慢端，立即踢出。
	for oc := range s.registry.set {
		if oc != c && oc.mode == proto.ModeRW && !oc.creditBlocked {
			s.kickSlowConsumerLocked(c)
			return
		}
	}
	// 全体可写端均满 → 不踢，计入信用（保护 owner 演示者；幂等置位防重复计数、
	// 防触发帧被二次暂存覆写）。
	if !c.creditBlocked {
		c.creditBlocked = true
		c.creditPending = frame // 触发帧暂存，afterDrain 重投（触发帧不丢）
		s.registry.gateTransitions++ // Phase 8 OPS-07 门开闭周期计数挂点（review #10）
	}
}

// afterDrain 在一次成功批量写出后做信用门恢复判定（R-01 半水位迟滞）：outbox 当前
// 字节 < cap/2（半水位，defaultOutboxBytes 的 50% = 256KiB）且 c.creditBlocked →
// 重投触发帧 + 清位 + hubCond.Broadcast 开门。锁序 R-07：drain 完才取 hubMu
//（本函数），绝不反序同持；hubMu > outboxMu 的同序同持与 onChunk→trySend 同款。
//
// 重投有序性：暂存帧在清位/Broadcast 之前入队——门仍闭合（flag 未清），onChunk
// 无法夹入新帧；清位开门后新帧经门判定排在暂存帧之后，客户端字节流严格有序。
// 重投必成的数学保证：cur < cap/2 ⇒ 余量 ≥ cap/2+1 ≥ 32KiB+1（最大帧 = 32KiB 读块
// + 类型字节）——要求 outbox cap ≥ 64KiB（默认 512KiB，测试覆写下限同此；cap
// 小于单帧的场景下 trySend 本就恒败，属配置错误不在此兜底）；防御性失败分支保位
// 保帧，下次 drain 再试。
func (s *Server) afterDrain(c *client) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	c.outbox.mu.Lock()
	cur := c.outbox.bytes
	c.outbox.mu.Unlock()
	if !c.creditBlocked || cur >= c.outbox.cap/2 {
		return
	}
	if c.creditPending != nil {
		if !c.outbox.trySend(c.creditPending) {
			return // 防御（数学上不可达，见上）：保位保帧，下次 drain 再试
		}
		c.creditPending = nil
	}
	c.creditBlocked = false
	s.registry.gateTransitions++ // Phase 8 OPS-07 门开闭周期计数挂点（review #10）
	s.hubCond.Broadcast()
}

// kickSlowConsumerLocked 踢出 outbox 写满的客户端（R-10 命名族）：注册表同步移除
// + close(done)（非阻塞）+ logEvent + 异步 Close。调用方必须已持 hubMu。
// 踢出/信用的分工判定在 kickOrCreditLocked（R-08），本函数只执行踢出。
//
// Close 永不内联（P5-2）：Close 对 stall 客户端最长阻塞 ~10s（close.go:87-89，
// 5s 写超时 + 5s 等对端关闭帧），内联会把行头阻塞还魂；Close 幂等且完成后解除
// 全部 goroutine 阻塞（close.go:92-96），被踢客户端卡死的 writer/reader 随其收口。
// 踢出即唯一处置——绝不丢帧保连接（must_haves prohibitions）。
//
// 1013 关闭帧可达性不变量（05-02 Task 3 实测锁定）：cancel 必须推迟到异步 Close
// 落定之后——Attach 读循环随 cancel 终结 → defer CloseNow 立刻硬关 TCP；而
// Close/CloseNow 竞态由先到的 casClosing 取胜（close.go:101/134），CloseNow 先胜
// 则 1013 关闭帧永不上 wire（stall 客户端只见 EOF，TestSlowConsumerKick 实测
// 命中）。cancel 置于异步 goroutine 内 Close 之后：Close 先赢 casClosing → 客户端
// 消化管道后收到 1013 关闭帧 → 握手落定 → cancel 收口 pinger/reader，Attach
// defer 的 CloseNow 退化为 wait-only no-op（幂等自界）。对永不读取的真死连接，
// writeClose 5s 超时照常收口（close.go:168-183），时序不变量不受影响。
//
// 移除后 hubCond.Broadcast（P5-7 统一挂点）：踢出改变可写端信用集构成，等待中的
// 信用门必须重估（死锁免除链路——detach/kick/attach/子进程退出四处统一 Broadcast）。
func (s *Server) kickSlowConsumerLocked(c *client) {
	if !s.registry.removeLocked(c) {
		return // 已被 detach 移除（防御幂等；hubMu 下正常不会发生）
	}
	close(c.done)
	s.registry.kicks++ // Phase 8 OPS-07 1013 踢出计数挂点（review #10）
	logEvent(c.remote, websocket.StatusTryAgainLater, "slow_consumer")
	// review #3 时序闭合（第二调用点）：被踢的若是 owner，晋升在异步 go Close
	// 之前、同一 hubMu 持有内同步完成——见 promoteNextLocked 注释。
	if s.registry.owner == c {
		s.promoteNextLocked()
	}
	s.hubCond.Broadcast() // P5-7 统一挂点：kick 后门重估
	go func() {
		_ = c.conn.Close(websocket.StatusTryAgainLater, "slow_consumer")
		c.cancel() // Close 落定后终结 pinger/reader ctx（1013 帧可达性不变量，见上）
	}()
}

// promoteNextLocked 在 owner 被移除后做 FIFO 递补升格（MULTI-02，D-06）：遍历
// registry.order（attach FIFO 序）取首个 rwEligible 且仍在线者（order 成员即
// 在线）→ 置 owner + c.mode = ModeRW + c.rwEligible 保留（语义：rw 身份资格不随
// 升格消失，其后再断开递补链按同一规则继续）→ 其 outbox 入队
// Welcome{mode:"rw", prefs: rw 档}（R-09 升格推送复用 'W' 帧——P2 D-01/D-02：
// 既有帧类型运行期再推送不算动协议，零新类型字节；P5-6：升格必携 rw 档
// prefs——升格即获 osc52，D-13 的另一半）→ hubCond.Broadcast()（P5-7 升格挂点：
// 新 rw 端进信用集，等待中的信用门必须重估）。无可递补者 → owner=nil（下一个
// rw attach 按矩阵成为新 owner）。
//
// 升格 Welcome 入队失败（该端 outbox 连 ~100B 余量都没有 = 事实上 stalled）按
// R-08 分工表同义踢出（ro 满即踢）并继续扫描下一位——绝不立一个无法送达升格
// 通知的 owner（T-05-08 权限真空防线；该端下一轮 onChunk 本就必被踢，此处只是
// 提前一拍收口）。
//
// 仲裁参与集切换挂点（05-04 resize.go 落地）：owner 模式下仅 owner 尺寸参与仲裁
//（D-09）——升格后新 owner 尺寸接管，挂点登记于此。
//
// 调用时序闭合（review #3）：本函数在 detach 与 kickSlowConsumerLocked 两路径的
// removeLocked 之后同步调用（同一 hubMu 持有内、异步 go Close 之前）——晋升恒在
// hubMu 内完成；被移除 owner 的重连必须经 HTTP→ticket→WS→Hello→registerLocked
// 取同一 hubMu，故晋升必然先于任何重连登记；重连的旧 owner 按矩阵作为新 client
// 追加 order 尾（rwEligible=true、mode=ro 归队重排），绝不出现双 owner 或晋升
// 丢失。调用方必须已持 hubMu，且注册表须已完成移除（owner 指针仍指向被移除者）。
func (s *Server) promoteNextLocked() {
	for {
		var cand *client
		for _, c := range s.registry.order {
			if c.rwEligible {
				cand = c
				break
			}
		}
		if cand == nil {
			s.registry.owner = nil // 无可递补者：下一个 rw attach 按矩阵成为新 owner
			return
		}
		if !cand.outbox.trySend(proto.WelcomeFrame(proto.ModeRW, s.clientPrefsRW)) {
			s.kickSlowConsumerLocked(cand) // 升格通知不可达 = stalled，同义踢出后重扫
			continue
		}
		s.registry.owner = cand
		cand.mode = proto.ModeRW
		s.hubCond.Broadcast() // P5-7 升格挂点：新 rw 端进信用集
		return
	}
}

// writer 是每客户端专属 WS 写端 goroutine（pinger 装配先例，server.go 既有
// per-conn goroutine 模式）：阻塞消费 notEmpty 信号，drain 整队后按同类型连续段
// 合并写出（ARCHITECTURE §2.5 写合并——合并成单帧 = 类型字节一次 + 载荷顺序拼接，
// 减少帧数与小包）。1 WS 消息 = 1 帧的线上纪律不变（前端 buf[0] 分派零改动，
// must_have「扇出对前端透明」），合并后 OUTPUT 字节流与逐帧接收完全相同（有序
// delta 流语义不变）。
// 写允许阻塞——阻塞即"该客户端慢"，由 outbox 满 → 1013 踢出收口；写失败静默
// return（连接已死，终结由该客户端 reader 路径收口——D-11 纪律的多客户端映射）。
func (s *Server) writer(c *client) {
	for {
		select {
		case <-c.done:
			return
		case <-c.outbox.notEmpty:
		}
		batch, _ := c.outbox.drain()
		if len(batch) == 0 {
			continue
		}
		for i := 0; i < len(batch); {
			j := i + 1
			for j < len(batch) && len(batch[j]) > 0 && batch[j][0] == batch[i][0] {
				j++
			}
			msg := batch[i] // 单帧零拷贝直写（共享帧只读纪律不受影响）
			if j > i+1 {
				msg = append([]byte(nil), batch[i]...) // 拷贝防共享帧原地 append（P5-1）
				for k := i + 1; k < j; k++ {
					msg = append(msg, batch[k][1:]...)
				}
			}
			if err := c.conn.Write(context.Background(), websocket.MessageBinary, msg); err != nil {
				return
			}
			i = j
		}
		// 整批写出成功 → 信用门恢复判定（R-01 半水位；R-07 锁序：drain/写出完才取
		// hubMu，绝不反序同持）。写失败路径已 return，不触达本行。
		s.afterDrain(c)
	}
}

// detach 收口客户端断开（reader 路径唯一终结点）：注册表移除 + close(done) +
// cancel pinger ctx。不进 exitf、不发任何信号——多客户端必然推论（CONTEXT
// domain：P1 D-11 单次语义终结；服务端生命周期只随子进程，无客户端时子进程
// 继续运行且新客户端仍可 attach）。
//
// 移除后 hubCond.Broadcast（P5-7 统一挂点）：信用门闭合期间的可写端死亡是本路径
// 收口——注册表移除使门重估（全体可写端消失或被移出 → 门开），是死锁免除链路
// 的必经环节（review #2 dead-owner 场景：门闭合期间一端 CloseNow → 本路径 →
// 门有界重开）。
func (s *Server) detach(c *client) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	if !s.registry.removeLocked(c) {
		return // 已被 kick 移除——close(done)/cancel 恰好一次由成员判定保证
	}
	close(c.done)
	c.cancel()
	// review #3 时序闭合（第一调用点）：断开的若是 owner，FIFO 递补晋升在同一
	// hubMu 持有内同步完成——晋升必然先于该 owner 任何重连的 registerLocked。
	if s.registry.owner == c {
		s.promoteNextLocked()
	}
	s.hubCond.Broadcast() // P5-7 统一挂点：detach 后门重估
}
