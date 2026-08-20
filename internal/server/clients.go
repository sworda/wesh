// clients.go：多客户端注册表 + fan-out hub + 每客户端 outbox/writer（MULTI-01/03
// 主干，CONTEXT domain 必然推论——P1 D-11 单次语义终结；RESEARCH Pattern 1/2/5
// 定稿形态，P5-1 chunk 别名与 P5-2 踢出不内联两条红线纪律）。
package server

import (
	"bytes"
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

// client 是一个已注册 WS 客户端的全部服务端状态。writer goroutine 是该连接全程
// 唯一 WS 写端（pinger 的控制帧经库 writeFrameMu 与数据写串行化，既有 02-04 纪律）。
type client struct {
	conn   *websocket.Conn
	remote string // logEvent 三要素之对端（Attach 入口保存的 RemoteAddr）
	mode   string // proto.ModeRO/ModeRW——INPUT/RESIZE 门 per-client 判定（P2 D-13/D-14 的多客户端映射）

	attachSeq int64 // attach FIFO 序号（registerLocked 分配；05-03 owner 递补取序）
	outbox    *outbox
	done      chan struct{}      // writer 终结信号——kick/detach 关闭，恰好一次由 hubMu 内注册表成员判定保证
	cancel    context.CancelFunc // pinger 所在 ctx 的 cancel（Attach 派生，随客户端生命周期）
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

// drain swap 出整队并重置字节计数（返回的 bytes 供 05-02 信用门半水位恢复判定，
// 本 plan 调用方不使用）。
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
	kicks int // 1013 踢出计数（Phase 8 OPS-07 观测性挂点，review #10；hubMu 保护，单锁纪律 R-07 下无需 atomic）
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

// onChunk 是 S→C fan-out hub（ReadLoop 唯一读者经 sess.ReadLoop 回调，D-12 drain
// 语义不变——无客户端时遍历空集自然丢弃，防 PTY 内核缓冲写阻塞）。
//
// 别名红线（P5-1，pty/io.go:13-14 注释明示"onChunk 复用底层缓冲——回调方如需跨帧
// 持有须自行拷贝"）：每 chunk 组帧一次（make+copy 是唯一拷贝点），全部客户端
// outbox 共享该只读帧引用，逐客户端零拷贝；outbox 绝不直接存 chunk。05-02 信用门
// 不变量（review #1）：门的 Wait 循环必须插入在下方组帧语句之前——门持块期间
// chunk 停留于 ReadLoop 缓冲（阻塞即无下次读，无别名窗口），帧拷贝在门开之后发生。
//
// 临界区只含非阻塞 trySend 遍历：无锁等待、cond 等待或逐客户端帧拷贝（单客户端
// 内存与延迟形态与 Phase 4 等价）。
func (s *Server) onChunk(chunk []byte) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	frame := make([]byte, 1+len(chunk))
	frame[0] = proto.Output
	copy(frame[1:], chunk)
	for c := range s.registry.set {
		if !c.outbox.trySend(frame) {
			s.kickSlowConsumerLocked(c)
		}
	}
}

// kickSlowConsumerLocked 踢出 outbox 写满的客户端（R-10 命名族）：注册表同步移除
// + close(done)/cancel（均非阻塞）+ logEvent + 异步 Close。调用方必须已持 hubMu。
//
// Close 永不内联（P5-2）：Close 对 stall 客户端最长阻塞 ~10s（close.go:87-89，
// 5s 写超时 + 5s 等对端关闭帧），内联会把行头阻塞还魂；Close 幂等且完成后解除
// 全部 goroutine 阻塞（close.go:92-96），被踢客户端卡死的 writer/reader 随其收口。
// 踢出即唯一处置——绝不丢帧保连接（must_haves prohibitions）。
func (s *Server) kickSlowConsumerLocked(c *client) {
	if !s.registry.removeLocked(c) {
		return // 已被 detach 移除（防御幂等；hubMu 下正常不会发生）
	}
	close(c.done)
	c.cancel()
	s.registry.kicks++ // Phase 8 OPS-07 1013 踢出计数挂点（review #10）
	logEvent(c.remote, websocket.StatusTryAgainLater, "slow_consumer")
	go func() {
		_ = c.conn.Close(websocket.StatusTryAgainLater, "slow_consumer")
	}()
}

// writer 是每客户端专属 WS 写端 goroutine（pinger 装配先例，server.go 既有
// per-conn goroutine 模式）：阻塞消费 notEmpty 信号，drain 批量合并单帧写出
//（ARCHITECTURE §2.5 写合并；bytes.Join 逐帧拷进新缓冲，共享帧本体只读零改动）。
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
		if err := c.conn.Write(context.Background(), websocket.MessageBinary, bytes.Join(batch, nil)); err != nil {
			return
		}
	}
}

// detach 收口客户端断开（reader 路径唯一终结点）：注册表移除 + close(done) +
// cancel pinger ctx。不进 exitf、不发任何信号——多客户端必然推论（CONTEXT
// domain：P1 D-11 单次语义终结；服务端生命周期只随子进程，无客户端时子进程
// 继续运行且新客户端仍可 attach）。
func (s *Server) detach(c *client) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	if !s.registry.removeLocked(c) {
		return // 已被 kick 移除——close(done)/cancel 恰好一次由成员判定保证
	}
	close(c.done)
	c.cancel()
}
