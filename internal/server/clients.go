// clients.go：多客户端注册表 + fan-out hub + 每客户端 outbox/writer（MULTI-01/03
// 主干，CONTEXT domain 必然推论——P1 D-11 单次语义终结；RESEARCH Pattern 1/2/5
// 定稿形态，P5-1 chunk 别名与 P5-2 踢出不内联两条红线纪律）+ 全局信用门
// （RES-04，RESEARCH Pattern 3：全体可写端 outbox 均满 → hub 持块停读 PTY，
// 任一可写端 drain 至半水位恢复）+ 写权限体系（MULTI-02，05-03：模式判定矩阵 +
// owner FIFO 递补升格，RESEARCH Pattern 5 逐字落地）+ 每客户端输入限速与会话级
// 输入队列/input-writer（RES-02 + CR-01 完整背压修复，05-05，RESEARCH Pattern 8：
// Attach 读循环零同步 Master.Write，单 goroutine 独占写 master）。
//
// 锁序纪律（R-07）：hubMu > outboxMu——writer 持 outboxMu drain 期间绝不取 hubMu；
// drain 完释放后才取 hubMu 做 afterDrain 恢复判定，绝不反序同持。信用门 cond
// 挂 hubMu（信用门状态与注册表同锁），outbox 自有 mu。
package server

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate" // 钉版 v0.15.0 防版本漂移：rate API 自 2015 年签名稳定（rate.go:100-117），升级经 go.sum 审计链显式进行（review MEDIUM 处置）

	"github.com/sworda/wesh/internal/proto"
)

// 多客户端五个测试可覆写参数的默认常量（R-01 初值；P2 D-10 常量纪律——一律常量
// 不开 CLI flag，全部挂 Phase 9 负载测试标定回填）。声明在本文件（hub/outbox/限速
// 均属多客户端关切面），server.go New 的零值兜底逐字段引用。
const (
	// defaultOutboxBytes 每客户端 outbox 字节容量默认值（512KiB：16×32KiB 读块，
	// 100KB/s 慢链路 ~5s 抖动容忍）。Phase 9 负载测试标定回填（P2 D-10 纪律）。
	defaultOutboxBytes = 512 * 1024
	// defaultMaxClients 最大并发客户端数默认值（32：ARCHITECTURE §6『10–100 连接
	// =团队围观/教学』区间下沿；账面内存与 goroutine 开销微小）。Phase 9 负载测试
	// 标定回填（P2 D-10 纪律）。消费点已落地（05-07：守卫区③位 503 闸 +
	// /api/attach 503 早闸，判定源 = registry.n）。
	defaultMaxClients = 32
	// defaultInputRate 每客户端输入限速速率默认值（32KiB/s：人类击键 ~10B/s，
	// 持续 32KiB/s 远超合法、远低于洪水）。Phase 9 负载测试标定回填（P2 D-10
	// 纪律）。消费点 = Attach 升档 limiter 构造 + INPUT 门 AllowN（05-05 已落地）。
	defaultInputRate = 32 * 1024
	// defaultInputBurst 每客户端输入限速突发默认值（64KiB：容纳一次快粘）。
	// Phase 9 负载测试标定回填（P2 D-10 纪律）。消费点同 defaultInputRate
	//（05-05 已落地）。
	defaultInputBurst = 64 * 1024
	// defaultInputQueueBytes 会话级输入队列字节容量默认值（256KiB：≥16 个
	// 16KiB 满帧 ReadLimitPostAuth；限速器在前，队列满本应罕见）。Phase 9
	// 负载测试标定回填（P2 D-10 纪律）。消费点 = server.go New 装配 inputQ
	//（05-05 已落地）。
	defaultInputQueueBytes = 256 * 1024
	// defaultResizeDebounce resize 仲裁防抖默认值（50ms，CONTEXT 已锁定；
	// PITFALLS Pitfall 10 SIGWINCH 风暴防线）。Phase 9 负载测试标定回填
	//（P2 D-10 纪律）。消费点 = resize.go 仲裁器单 time.Timer reset（05-04 已落地）。
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
	remote string // logEvent 三要素之对端（Attach 入口保存的 RemoteAddr；07-03 trust 开启时为 XFF 链首，proxy.go）
	// remoteUser 为 logEvent 可选第四字段 remote_user 的值（07-03，SEC-07 D-15
	// 审计归因）：Attach 入口经 s.proxy.remoteUser(r) 提取一次（sanitize 已在
	// 提取点完成），此后只读——并发读写面与 remote 字段既有形态相同（写一次
	// 发生在 client 构造、registerLocked 之前，全部读者在其后启动的
	// writer/pinger/读循环 goroutine 内，happens-before 由 goroutine 启动建立；
	// plain 字段无锁安全，-race 全量回归锁）。空串 = 未配置/头缺席（logEvent
	// 不出键）。share token 渠道进入的客户端同经 Attach 入口提取点赋值。
	remoteUser string
	// mode 生效模式（proto.ModeRO/ModeRW 字符串，atomic.Value 承载）：升档时由
	// 判定矩阵写入初始值，运行期唯一写者是 promoteNextLocked 的 ro→rw 升格翻转
	//（hubMu 内）。Attach 读循环的 INPUT 门每击键无锁 Load（热路径不得取 hubMu），
	// 与 hubMu 内的晋升写并发（05-03 -race 实测命中）；hubMu 侧读取（信用门/
	// 分工表）同经 Load 统一形态。
	mode atomic.Value
	// limiter 每客户端输入限速令牌桶（RES-02，x/time/rate）：Attach 升档时构造
	// rate.NewLimiter(rate.Limit(s.inputRate), s.inputBurst)（默认 32KiB/s + burst
	// 64KiB，R-01 参数表——人类击键 ~10B/s、快粘 ~50KB 瞬时由 burst 容纳、持续
	// 32KiB/s 远超合法远低于洪水；Options.InputRate/InputBurst 测试可覆写）。
	// 超限唯一动作 = 丢弃该帧（R-02——Allow godoc 官方 drop 语义逐字："Use this
	// method if you intend to drop / skip events that exceed the rate limit."
	// rate.go:113-115；all 模式下激进粘贴的合法用户被踢是 UX 灾难，INPUT 丢帧
	// 用户可感知可恢复——键不回显自然放慢——不同于 OUTPUT 丢帧的静默损坏）；
	// 禁止以限速为由 Close/踢出/降权任何客户端（must_haves prohibitions）。
	// 升档构造后运行期只读；包内并发安全（rate.go:56-57 "Limiter is safe for
	// simultaneous use by multiple goroutines"）。
	limiter *rate.Limiter
	// rwEligible 递补候选资格（D-06「以 rw 身份」）：rw ticket × owner 模式 → true
	//（含 owner 本人——语义为「持 rw 身份的资格」，owner 断线后该客户端若仍在队
	// 即可被递补）；ro ticket 与 all 模式恒 false（ro 永不递补；all 无递补概念）。
	// 升档时写入后运行期只读（promoteNextLocked 保留其值），plain 字段。
	rwEligible bool

	// dims 客户端最近登记尺寸（Hello 首尺寸登记，升档时写入后运行期不再更新——
	// 参与集成员的最新尺寸由 arbiter.sizes 承载，本字段只服务递补升格时新 owner
	// 的参与集切换：D-09「递补后新 owner 尺寸接管」）。plain 字段，读取恒在
	// hubMu 内（promoteNextLocked 调用点）。
	dims dims

	attachSeq int64 // attach FIFO 序号（registerLocked 分配；05-03 owner 递补取序）
	outbox    *outbox
	done      chan struct{}      // writer 终结信号——kick/detach 关闭，恰好一次由 hubMu 内注册表成员判定保证
	cancel    context.CancelFunc // pinger 所在 ctx 的 cancel（Attach 派生，随客户端生命周期）

	// pongTimedOut 为 pinger pong 超时置位（08-02，D-21）：pinger 判定超时后取
	// hubMu 写本字段、detach 同锁读（RESEARCH Pattern 4 形态 b——同步边 =
	// hubMu，-race 防线；禁止 plain 字段跨 goroutine 传递）。pong_timeout 不再
	// 单独打事件行，折入 detach reason=pong_timeout（code 1006）。
	pongTimedOut bool

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

// inputQ 是会话级字节有界输入队列（CR-01 完整背压修复，RESEARCH Pattern 8）：
// 存 INPUT payload（data[1:]，各客户端读循环逐帧入队），单 input-writer
// goroutine 顺序出队独占 sess.Master.Write——Attach 读循环内的同步 Master.Write
// 彻底消失（CR-01：慢子进程 stdin 反堵读循环的已知缺陷终结；Phase 2 的
// O_NONBLOCK 最小缓解从未落地且不再需要，RESEARCH State of the Art 核实——
// master fd 保持默认阻塞模式，阻塞被关进专属 goroutine，本队列有界 + 丢弃即
// 背压）。多写者输入交错不做排序承诺（ARCHITECTURE §2.9 screen 同款语义，
// all 模式 README 明示由 05-09 落地）。
//
// 别名安全（与 OUTPUT 方向 chunk 别名红线 P5-1 对照区分）：coder/websocket
// Read 每次返回新分配 payload（消息缓冲跨帧不复用），tryEnqueue 持引用无别名
// 风险——入队无需拷贝点。
//
// notEmpty 为 cap 1 信号量（outbox 同款形态）：tryEnqueue 非阻塞投递，
// inputWriter 阻塞消费。
type inputQ struct {
	mu       sync.Mutex
	q        [][]byte // INPUT payload（各 Read 新分配，持引用无别名风险，见上）
	bytes    int
	cap      int
	notEmpty chan struct{}
	// droppedInputs 队列满丢弃计数（atomic——读循环热路径递增，Phase 8 metrics
	// 读取端免锁；Phase 8 OPS-07 进 metrics，review #10 挂点）。与 server 的
	// inputDrops（限速丢弃）成对；限速器在前，队列满本应罕见。
	droppedInputs atomic.Int64
}

// newInputQ 构造字节容量为 cap 的 inputQ（cap 由 defaultInputQueueBytes 供给，
// Phase 9 标定回填）。
func newInputQ(cap int) *inputQ {
	return &inputQ{cap: cap, notEmpty: make(chan struct{}, 1)}
}

// tryEnqueue 非阻塞入队：满 → 丢 + droppedInputs 计数递增并返回 false（调用方
// continue——满则丢绝不阻塞读循环，T-05-03b 防线）；成功则 append + 记账 +
// 非阻塞信号（已有信号在飞则 writer 必会 drain 到本条，select default 不阻塞）。
func (q *inputQ) tryEnqueue(p []byte) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.bytes+len(p) > q.cap {
		q.droppedInputs.Add(1) // Phase 8 OPS-07 metrics 挂点（review #10）
		return false
	}
	q.q = append(q.q, p)
	q.bytes += len(p)
	select {
	case q.notEmpty <- struct{}{}:
	default:
	}
	return true
}

// dequeue swap 出整队并重置字节计数（outbox drain 同款形态）。
func (q *inputQ) dequeue() [][]byte {
	q.mu.Lock()
	defer q.mu.Unlock()
	batch := q.q
	q.q = nil
	q.bytes = 0
	return batch
}

// registry 客户端注册表（R-07）：set 供 hub 扇出遍历，order 保 attach FIFO 序
// （05-03 owner 递补队列的遍历序），seq 为 attachSeq 分配器。全部方法名以 Locked
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
	// n 已注册客户端计数（RES-03，05-07——守卫区③位 503 闸与 /api/attach 早闸的
	// 判定源）：atomic 承载——③位闸在 hubMu 外 atomic load（守卫区 Accept 前不得
	// 取 hubMu），故须 atomic 而非 hubMu 保护（与 kicks/gateTransitions 的 hubMu
	// 内 plain int 成场景化选型）。计数口径 R-06：registerLocked 成功后 +1、
	// removeLocked -1——半开连接不计入（与 halfOpenCounter 正交两闸：SEC-08 半开
	// 面 vs RES-03 稳态容量面）；并发握手竞态最坏瞬时超编 ≤ per-IP 半开帽 8
	//（容量策略非安全边界，可接受——RESEARCH A5 裁断，Phase 9 负载标定复核）。
	// 对称不变量（review #7）：运行期全部移除路径——reader-error detach /
	// kickOrCreditLocked 1013 踢出 / pinger CloseNow 后 reader 错误 detach——均
	// 收口于 removeLocked 单点，加减排它性对称，漂移结构上不可能；lifecycle 广播
	// 后进程即退出（计数无后续读者）、panic 按 Go 语义进程崩溃（无恢复面）——
	// 两路径无漂移窗口。TestClientCountInvariant 白盒逐步锁定 n == len(set)。
	n atomic.Int64

	kicks int // 1013 踢出计数（Phase 8 OPS-07 观测性挂点，review #10；hubMu 保护，单锁纪律 R-07 下无需 atomic）
	// gateTransitions 信用门开闭周期计数（kickOrCreditLocked 置位 creditBlocked 与
	// afterDrain 清位两点递增）：Phase 8 OPS-07 门开闭周期计数挂点（review #10）；
	// hubMu 保护，零 atomic 不违 R-07 单锁纪律。
	gateTransitions int
}

// registerLocked 登记新客户端：分配 attachSeq、入 set 与 FIFO order，计数 +1
// （R-06 口径：注册成功才计数——半开连接不计入，与 halfOpenCounter 正交）。
func (r *registry) registerLocked(c *client) {
	r.seq++
	c.attachSeq = r.seq
	if r.set == nil {
		r.set = make(map[*client]struct{}) // 零值可用（TestClientCountInvariant 直构造，无需 Server/pty 装配）
	}
	r.set[c] = struct{}{}
	r.order = append(r.order, c)
	r.n.Add(1) // 对称记账：唯一加计数点（review #7）
}

// removeLocked 移除客户端：同时清理 map 项与 slice 项（Pitfall 4 双容器防单调
// 增长），计数 -1（对称记账：唯一减计数点——运行期全部移除路径收口于此，
// review #7）。返回是否移除成功——非成员幂等 false 且计数不变（减计数不得越过
// 实际移除，Pitfall 4 防线；kick 与 detach 互斥恰好一次的判定依据：先完成移除
// 的一方负责 close(done)/cancel）。
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
	r.n.Add(-1)
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
// 「客户端发帧申请写权限」的机制都是越权面，本矩阵不提供也不接受此类输入。
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
// （阻塞即无下次读，无别名窗口）；帧拷贝只发生在门开之后、trySend 之前，outbox
// 绝无持有跨门 chunk 的窗口。
//
// 全局信用门（RES-04，RESEARCH Pattern 3）：全体可写端 creditBlocked → hub 在
// hubCond 上 Wait——Wait 原子释放 hubMu；持块即停读 PTY，反压经 64KiB 内核缓冲
// 传导至子进程 write（RES-04 唯一合法反压路径，ARCHITECTURE §2.6）。任一可写端
// writer drain 至半水位 → afterDrain 清位 + Broadcast 恢复。
//
// 门之外的临界区只含非阻塞 trySend 遍历：无锁等待、cond 等待或逐客户端帧拷贝
// （单客户端内存与延迟形态与 Phase 4 等价）。
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
// （纯 ro 会话/无客户端）→ false（信用集为空 → 门永不闭合 → ro 满即踢，R-08
// 分工表前提）。调用方必须已持 hubMu。O(n) 遍历每 chunk 一次，规模 ≤ max-clients
// 32，可接受（review LOW 项登记）。
func (s *Server) allWritableBlockedLocked() bool {
	writable := 0
	for c := range s.registry.set {
		if c.mode.Load() != proto.ModeRW {
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
// 不暂存则恢复端流缺一段——review #1 行为证据锁住的正是此窗口）。首帧暂存边界
// （WR-02，05-13）：幂等置位守卫（`if !c.creditBlocked`）下已 blocked 的后续帧
// 不覆写暂存——防二次暂存覆写首帧的既有语义；尺寸推送类帧的收敛出口 =
// afterDrain 开门时补发当前 sessionDimsLocked() 的 Welcome（见 afterDrain 注释）。
//
// 迟滞带论证（review #2）：门关闭阈值 = outbox 写满（100% 字节上界才置
// creditBlocked）、恢复阈值 = drain 至 <50%——2:1 迟滞带内建于分工表，非
// 『50% 单点抖动』；残余振荡（滴漏读者每次 drain 开门、下一 chunk 可能再闭门）
// 经 RESEARCH Open Question 3 裁决接受（各端持续前进；dwell 计时器 Phase 9
// 负载标定时评估）。调用方必须已持 hubMu。
func (s *Server) kickOrCreditLocked(c *client, frame []byte) {
	if c.mode.Load() == proto.ModeRO {
		s.kickSlowConsumerLocked(c)
		return
	}
	// rw：剔除 c 后仍存在未 blocked 的可写端 → c 是离群慢端，立即踢出。
	for oc := range s.registry.set {
		if oc != c && oc.mode.Load() == proto.ModeRW && !oc.creditBlocked {
			s.kickSlowConsumerLocked(c)
			return
		}
	}
	// 全体可写端均满 → 不踢，计入信用（保护 owner 演示者；幂等置位防重复计数、
	// 防触发帧被二次暂存覆写）。
	if !c.creditBlocked {
		c.creditBlocked = true
		c.creditPending = frame      // 触发帧暂存，afterDrain 重投（触发帧不丢）
		s.registry.gateTransitions++ // Phase 8 OPS-07 门开闭周期计数挂点（review #10）
	}
}

// afterDrain 在一次成功批量写出后做信用门恢复判定（R-01 半水位迟滞）：outbox 当前
// 字节 < cap/2（半水位，defaultOutboxBytes 的 50% = 256KiB）且 c.creditBlocked →
// 重投触发帧 + 清位 + 补发当前会话尺寸 Welcome（WR-02）+ hubCond.Broadcast
// 开门。锁序 R-07：drain 完才取 hubMu
// （本函数），绝不反序同持；hubMu > outboxMu 的同序同持与 onChunk→trySend 同款。
//
// 重投有序性：暂存帧在清位/Broadcast 之前入队——门仍闭合（flag 未清），onChunk
// 无法夹入新帧；清位开门后新帧经门判定排在暂存帧之后，客户端字节流严格有序。
// 重投必成的数学保证：cur < cap/2 ⇒ 余量 ≥ cap/2+1 ≥ 32KiB+1（最大帧 = 32KiB 读块
// + 类型字节）——要求 outbox cap ≥ 64KiB（默认 512KiB，测试覆写下限同此；cap
// 小于单帧的场景下 trySend 本就恒败，属配置错误不在此兜底）；防御性失败分支保位
// 保帧，下次 drain 再试。
//
// WR-02 补发（05-13，用户裁决 option (a)）：恢复开门时向该端补发一帧当前
// sessionDimsLocked() 的 Welcome——creditBlocked 期间被 kickOrCreditLocked 的
// `if !c.creditBlocked` 守卫跳过的尺寸推送帧由此收敛，该端 sessionDims 不过期至
// 下次尺寸事件。补发帧在清位后、Broadcast 前入队：afterDrain 全程持有 hubMu，
// onChunk 无法进入临界区夹入新帧；补发帧经 outbox FIFO 排在重投的 creditPending
// 之后，客户端字节流严格有序保持。入队必成沿用重投的数学保证（余量 ≥ cap/2+1
// ≫ ~100B Welcome），`_ =` 不兜底（失败属配置错误）；prefs 按 c.mode 选档
// （pushSessionDimsLocked 逐字同形态，D-13 双档纪律在补发通道不漂移）。
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
	// WR-02（05-13，option (a)）：补发一帧当前会话尺寸的 Welcome——收敛 creditBlocked
	// 期间被守卫静默丢弃的尺寸推送帧（选档同 pushSessionDimsLocked，D-13 不漂移）。
	sd := s.sessionDimsLocked()
	mode := c.mode.Load().(string)
	prefs := s.clientPrefsRO
	if mode == proto.ModeRW {
		prefs = s.clientPrefsRW
	}
	_ = c.outbox.trySend(proto.WelcomeFrame(mode, prefs, sd.cols, sd.rows)) // 补发阻塞期错过的尺寸推送
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
	// 08-02 D-21：slow_consumer 独立事件行折入 detach 单事件（reason=kick，
	// code=1013）——「连接断开」检索单入口；wire 关闭帧
	// Close(1013, "slow_consumer") 与 kicks 计数逐字不动（客户端可见形态
	// 由 slowclient_test.go 既有断言锁定）。
	s.emitDetachLocked(c, "kick", websocket.StatusTryAgainLater)
	// arbiter 参与集移除 + 即时重算（05-04，D-09）：all 模式被踢的 rw 端是参与集
	// 成员——滞留 sizes 则其陈旧尺寸永久拖累 min-rect（幽灵成员把 PTY 压在离群者
	// 小窗口）；owner 模式被踢者为 ro 旁观者（非成员，removeMember 幂等 no-op）。
	s.removeMember(c)
	s.recalcNow()
	// review #3 时序闭合（第二调用点）：被踢的若是 owner，晋升在异步 go Close
	// 之前、同一 hubMu 持有内同步完成——见 promoteNextLocked 注释。
	if s.registry.owner == c {
		s.promoteNextLocked()
	}
	// 注册表空触发断开退出（06-02，SESS-01/02）：非空→空迁移事件挂点之二——
	// kick 移除路径同挂点（removeLocked 返回 true 之后、hubCond.Broadcast 之前，
	// 与 detach 调用点同位同款注释纪律）。
	s.maybeExitWhenEmptyLocked(c)
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
// 仲裁参与集切换（05-04 resize.go 已落地）：owner 模式下仅 owner 尺寸参与仲裁
// （D-09）——升格后 addMember(新 owner, Hello 登记尺寸) + recalcNow 即时重算，
// 新 owner 尺寸接管。
//
// 升格 Welcome 尺寸取值论证（G-05-1，05-10）：组帧携 cand.dims（Hello 登记尺寸）
// 而非重算后的 arbiter.last——owner 模式升格后参与集 = {cand} 单员（旧 owner 已在
// detach/kick 的 removeMember 移除，递补链上被同义踢出的候选亦非成员），arbitrate
// 单员 = cand.dims，故升格 Welcome 尺寸恒等于升格后 recalcNow 的 last。保持
// trySend 在前、失败踢出重扫的既有形态——不得为重算尺寸把 addMember/recalcNow
// 前移（trySend 失败路径要回滚参与集，复杂度换取的值相同）。cand.dims 运行期不
// 更新（clients.go dims 字段注释既定）：旁观期缩窗的瞬态偏差由前端升格分支
// refit→RESIZE 上报纠正（05-08 纠正链，05-11 保持），随后 recalcNow 推送收口。
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
		if !cand.outbox.trySend(proto.WelcomeFrame(proto.ModeRW, s.clientPrefsRW, cand.dims.cols, cand.dims.rows)) {
			s.kickSlowConsumerLocked(cand) // 升格通知不可达 = stalled，同义踢出后重扫
			continue
		}
		s.registry.owner = cand
		cand.mode.Store(proto.ModeRW) // ro→rw 升格翻转（atomic 承载：INPUT 门无锁读者）
		// arbiter 参与集切换（05-04，D-09：owner 模式仅 owner 尺寸参与——sizes
		// 只保留新 owner；旧 owner 已在 detach/kick 的 removeMember 完成移除，
		// 递补链上被同义踢出的候选亦非成员，此处 addMember 后 sizes 恒为
		// {新 owner} 单员）+ 即时重算不防抖——递补后新 owner 尺寸接管。
		s.addMember(cand, cand.dims)
		s.recalcNow()
		s.hubCond.Broadcast() // P5-7 升格挂点：新 rw 端进信用集
		return
	}
}

// mergeBatch 把 drain 出的整批帧折叠为待写消息序列（ARCHITECTURE §2.5 写合并）：
// 同类型连续段合并成单条消息 = 类型字节一次 + 载荷顺序拼接，减少帧数与小包。
//
// 合并仅限 OUTPUT 数据帧（WR-02）：合并后 OUTPUT 字节流与逐帧接收完全相同
// （有序 delta 流语义不变，前端 buf[0] 分派零改动，must_have「扇出对前端透明」）。
// 控制帧（W/E）恒单发——其载荷是独立 JSON 文档，拼接产物（W{...}{...}）前端
// JSON.parse 抛错整帧丢弃（main.ts "discard malformed WELCOME"）。可达时序：
// attach Welcome 先入队（server.go 升档，hubMu 内），go writer 在 hubMu 释放后
// 才启动；间隙内 owner reader 终结 → detach → promoteNextLocked 命中同端 →
// 第二帧（升格 rw）Welcome 入队同一 outbox——静默 shell 无输出时批内即
// [W1(ro), W2(rw)] 相邻，合并将使被升格端丢失该 Welcome 的全部应用（prefs
// 不应用、welcomeDone 永不置位）。
// 单帧零拷贝直写（共享帧只读纪律不受影响）；合并帧先拷贝再拼接（防共享帧原地
// append，P5-1）。
func mergeBatch(batch [][]byte) [][]byte {
	var msgs [][]byte
	for i := 0; i < len(batch); {
		j := i + 1
		for j < len(batch) && len(batch[j]) > 0 && batch[j][0] == batch[i][0] && batch[i][0] == proto.Output {
			j++
		}
		msg := batch[i] // 单帧零拷贝直写（共享帧只读纪律不受影响）
		if j > i+1 {
			msg = append([]byte(nil), batch[i]...) // 拷贝防共享帧原地 append（P5-1）
			for k := i + 1; k < j; k++ {
				msg = append(msg, batch[k][1:]...)
			}
		}
		msgs = append(msgs, msg)
		i = j
	}
	return msgs
}

// writer 是每客户端专属 WS 写端 goroutine（pinger 装配先例，server.go 既有
// per-conn goroutine 模式）：阻塞消费 notEmpty 信号，drain 整队后经 mergeBatch
// 折叠写出（ARCHITECTURE §2.5 写合并——合并仅限 OUTPUT 数据帧，控制帧 W/E 恒
// 单发，WR-02；合并语义与红线论证见 mergeBatch 注释）。1 WS 消息 = 1 帧的线上
// 纪律对控制帧不变（前端 buf[0] 分派零改动，must_have「扇出对前端透明」）。
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
		for _, msg := range mergeBatch(batch) {
			if err := c.conn.Write(context.Background(), websocket.MessageBinary, msg); err != nil {
				return
			}
		}
		// 整批写出成功 → 信用门恢复判定（R-01 半水位；R-07 锁序：drain/写出完才取
		// hubMu，绝不反序同持）。写失败路径已 return，不触达本行。
		s.afterDrain(c)
	}
}

// inputWriter 是会话级单 input-writer goroutine（CR-01 完整背压修复核心，
// RESEARCH Pattern 8）：独占 sess.Master.Write——阻塞等队列信号 → 顺序出队 →
// 逐 payload 写 master。Attach 读循环内的同步 Master.Write 彻底消失（读循环
// 零同步写，must_haves；禁止为输入方向恢复任何形式的读循环内同步 Master.Write
// 或给读路径加 deadline——prohibitions，Pitfall 2/CR-01 老路禁止回潮）。
//
// 生命周期挂服务端：New 内启动（与 ReadLoop/lifecycle 同批装配）；终结 =
// lifecycle 子进程退出路径 close(inputDone)——sess.Drain→Close 先关闭 master
// fd，经 runtime poller 解除本 goroutine 的在途写阻塞（与 Read 同机制，既有
// D-12 语义），写失败即 return（子进程退出路径由 lifecycle 收口）；select 收
// inputDone 亦 return。队列残余随会话消亡（子进程已退出，输入无意义）。
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
	// 08-02 D-17/D-21：detach 单事件 emit（removeLocked 返回 true 之后、
	// close(done) 之前——事件即「连接断开」检索的唯一入口）。reason 判定序：
	//   - c.pongTimedOut（pinger 同锁置位，Pattern 4 形态 b）→ pong_timeout，
	//     code=1006（对端观测的本地合成异常关闭码语义，wire 永不发送 1006）；
	//   - s.exiting（lifecycle/Shutdown 终结广播窗口，既有不变量：置位先于
	//     广播）→ shutdown，code 取 s.closeBroadcastCode（lifecycle 置 1000 /
	//     Shutdown 置 1001——与广播关闭码同源）；
	//   - 否则 → normal，code=1000（对端主动关闭/网络断开）。
	// kick 路径不到本函数（kickSlowConsumerLocked 就地 emit reason=kick）。
	reason, code := "normal", websocket.StatusNormalClosure
	switch {
	case c.pongTimedOut:
		reason, code = "pong_timeout", websocket.StatusAbnormalClosure
	case s.exiting:
		reason, code = "shutdown", websocket.StatusCode(s.closeBroadcastCode)
	}
	s.emitDetachLocked(c, reason, code)
	close(c.done)
	c.cancel()
	// arbiter 参与集移除 + 即时重算（05-04，D-09，不防抖——参与集收缩立即反映，
	// all 模式 2→1 恢复剩余者 last-wins；非成员幂等 no-op，0 人时 arbitrate
	// 零值哨兵不动 PTY 保持现状）。
	s.removeMember(c)
	s.recalcNow()
	// review #3 时序闭合（第一调用点）：断开的若是 owner，FIFO 递补晋升在同一
	// hubMu 持有内同步完成——晋升必然先于该 owner 任何重连的 registerLocked。
	if s.registry.owner == c {
		s.promoteNextLocked()
	}
	// 注册表空触发断开退出（06-02，SESS-01/02）：非空→空迁移事件挂点之一——
	// removeLocked 返回 true 之后、hubCond.Broadcast 之前（与 kick 调用点同位
	// 同款注释纪律）。
	s.maybeExitWhenEmptyLocked(c)
	s.hubCond.Broadcast() // P5-7 统一挂点：detach 后门重估
}

// emitDetachLocked 是 detach 事件的唯一 emit 形态（08-02，D-18 schema 单侧
// 定义）：event=detach + remote/client_id（= c.attachSeq，D-20 关联检索键）
// + code/reason（D-21 四值）+ remote_user 非空追加（07-03 同口径——提取点
// 已 sanitize）。两调用点（detach 与 kickSlowConsumerLocked）共用本函数——
// 调用方必须已持 hubMu 且 c 刚被 removeLocked 移除成功（恰好一次 emit 由
// 移除所有权保证：reader detach 与 kick 互斥）。
func (s *Server) emitDetachLocked(c *client, reason string, code websocket.StatusCode) {
	attrs := []slog.Attr{
		slog.String("event", "detach"),
		slog.String("remote", c.remote),
		slog.Int64("client_id", c.attachSeq),
		slog.Int("code", int(code)),
		slog.String("reason", reason),
	}
	if c.remoteUser != "" {
		attrs = append(attrs, slog.String("remote_user", c.remoteUser))
	}
	emitEvent(attrs...)
}

// maybeExitWhenEmptyLocked 是注册表空触发断开退出的判定与执行（06-02，
// SESS-01/02，D-13/D-14；07-04 D-22 换入可配 stop-signal 序列）。调用方必须
// 已持 hubMu 且刚 removeLocked(c) 成功——事件 = 非空→空迁移（RESEARCH
// Pitfall 2：启动期恒空天然免疫，检测只挂 detach/kickSlowConsumerLocked 两
// 移除点，严禁轮询/状态式检测）。
//
// 三守卫任一成立即返回：!exitWhenEmpty（默认不开启 = 现状保持——无客户端时
// 子进程继续运行，P5『断开不退出』产品承诺，D-14）|| exiting（lifecycle 终结
// 广播门——广播 Close(1000) 引发的 detach 致空属正常终结序列，不得再生
// 信号/计时器）|| 注册表非空。
//
// grace==0（立即形态——0 是合法显式值，D-14 set/grace 分离）：logEvent
// exit_when_empty + stop-signal 序列（stopChildLocked——默认 SIGHUP 与 06-02
// 现状语义一致，D-22）。只发信号，不调 exitf、不经旁路 terminate——零新
// exitf 分支（D-13 硬约束），终结由既有 lifecycle 单一路径收口（信号 →
// 子进程死亡 → sess.Wait 返回 → exitf 以子进程退出码收口，两模式零分支差异）。
//
// grace>0（宽限形态）：既有 timer 先 Stop（幂等重启——再次断开重新计时）→
// AfterFunc 启动 exitEmptyTimer（回调捕获最后离开者 remote）→ logEvent
// exit_when_empty_wait。回调到期取 hubMu 复查『仍空且未 exiting』才发
// stop-signal 序列（RESEARCH Pitfall 4：复查是恰好一次的兜底——宽限内
// attach 已由取消点 Stop+置 nil，本回调能进入临界区即取消点未覆盖的残余
// 窗口）；信号幂等（kill 已死 pgid 收 ESRCH 静默忽略）；timer 随会话消亡。
//
// logEvent 三要素纪律（D-12② 延伸）：code 恒 websocket.StatusNormalClosure
// （1000 收口桶，reason 区分语义）；token/ticket/凭据值永不入参（SEC-01 红线）。
func (s *Server) maybeExitWhenEmptyLocked(c *client) {
	if !s.exitWhenEmpty || s.exiting || len(s.registry.set) != 0 {
		return
	}
	if s.exitWhenEmptyGrace == 0 {
		// 立即形态：无计时器，迁移点直接发 stop-signal 序列（D-22）。
		logEvent(c.remote, websocket.StatusNormalClosure, "exit_when_empty", c.remoteUser)
		s.stopChildLocked()
		return
	}
	if s.exitEmptyTimer != nil {
		s.exitEmptyTimer.Stop() // 幂等重启——再次断开重新计时（取消点置 nil 后的防御兜底）
	}
	// 回调捕获最后离开者对端与 remote_user——回调触发时 c 已随 detach 消亡
	//（remoteUser 与 remote 同为 Attach 入口写一次的只读字段，捕获同值）。
	remote, remoteUser := c.remote, c.remoteUser
	s.exitEmptyTimer = time.AfterFunc(s.exitWhenEmptyGrace, func() {
		s.hubMu.Lock()
		defer s.hubMu.Unlock()
		// 复查『仍空且未 exiting』（Pitfall 4 恰好一次兜底；arbiter timer 同款
		// 取锁纪律，resize.go initArbiter 先例）。不调 exitf——零新 exitf 分支
		//（D-13 硬约束）；信号幂等（已死 pgid ESRCH 静默）。
		if s.exiting || len(s.registry.set) != 0 {
			return
		}
		logEvent(remote, websocket.StatusNormalClosure, "exit_when_empty", remoteUser)
		s.stopChildLocked()
	})
	logEvent(c.remote, websocket.StatusNormalClosure, "exit_when_empty_wait", c.remoteUser)
}

// stopChildLocked 是 D-22 stop-signal 序列的统一出口（07-04，OPS-04）：向子进程
// 进程组发 s.stopSignal（pty.Session.SignalGroup——负 pid = 进程组，setsid 使
// pgid == 子进程 pid 既定不变量）；s.stopTimeout > 0 时 AfterFunc 异步补发
// SIGKILL（RESEARCH Pitfall 8 纪律：不用 sleep 阻塞 hubMu——补发不与 lifecycle
// 协调，ESRCH 幂等使子进程早死无害、KILL 到达空 pgid 静默；timer 随会话消亡）。
// 调用方必须已持 hubMu（本函数自身不取锁——SignalGroup 不触 Master fd 不取
// fdMu，锁序 hubMu > sess.fdMu 不受影响）。exit-when-empty 立即/宽限到期两
// 触发点与 07-05 Shutdown（1001 优雅下线）共用本序列——退出收口的信号配置
// 经 Options 单一通道（StopSignal/StopTimeout），双写即漂移。
func (s *Server) stopChildLocked() {
	s.sess.SignalGroup(s.stopSignal)
	if s.stopTimeout > 0 {
		time.AfterFunc(s.stopTimeout, func() { s.sess.SignalGroup(syscall.SIGKILL) })
	}
}

// cancelExitEmptyTimerLocked 是宽限取消点（06-02，D-14）：宽限内任一端 attach
// 成功（registerLocked 登记后由 Attach 升档序列在同一 hubMu 持有内调用）即取消
// 退出——恰好一次：置 nil 防重复 Stop 与重复 exit_when_empty_cancel 事件。
// 调用方必须已持 hubMu。code 恒 1000 收口桶（reason 区分语义，D-12② 延伸）；
// SEC-01 红线保持——token/ticket/凭据值永不入参。07-03：remoteUser 为新 attach
// 端的提取值（与 remote 同源传入），携 remote_user 第四字段同口径（D-15）。
func (s *Server) cancelExitEmptyTimerLocked(remote, remoteUser string) {
	if s.exitEmptyTimer == nil {
		return
	}
	s.exitEmptyTimer.Stop()
	s.exitEmptyTimer = nil
	logEvent(remote, websocket.StatusNormalClosure, "exit_when_empty_cancel", remoteUser)
}
