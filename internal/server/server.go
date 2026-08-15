// Package server 提供 HTTP + WS 网关、数据泵与 Phase 1 单次语义生命周期：
// 任意终结路径（子进程退出 / WS 断开）都使服务端经 exitf 整体退出（D-10/D-11）。
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/web"
)

// Server 持有会话、attach 原子门与生命周期收口件。
// exitf 由 main 注入 os.Exit、测试注入捕获桩——生命周期必须可测，这是硬约束。
type Server struct {
	sess  *pty.Session
	exitf func(code int)

	// New 装配期固化、运行期只读，故五个 plain 字段无需 atomic：
	// writable 为 D-13 ro 门读端；helloTimeout 为 D-04 未认证超时时长；
	// maxHalfOpenPerIP 为 D-04 per-IP 半开上限；pingInterval/pongTimeout 为
	// D-16 保活参数（均 Options 注入）。
	writable         bool
	helloTimeout     time.Duration
	maxHalfOpenPerIP int
	pingInterval     time.Duration
	pongTimeout      time.Duration

	attached atomic.Bool                    // D-09：单客户端原子门
	conn     atomic.Pointer[websocket.Conn] // 当前已完成握手的 WS 连接（onChunk 写端 / 1000 关闭用）
	frame    []byte                         // OUTPUT 组帧缓冲（仅 ReadLoop 单 goroutine 经 onChunk 访问，无竞争）

	// halfOpen 为 D-04 per-IP 半开（Hello 未完成）连接计数器；
	// acquire/release 恰好一次不变量见 halfOpenCounter 类型注释。
	halfOpen halfOpenCounter

	// childExited 区分两条终结路径的触发源：lifecycle 在 Wait 返回后先置位再关 conn，
	// Attach 读循环随服务端 1000 关闭帧终结时据此前置位识别"非客户端断开"，
	// 不得走 D-11 竞争 exitf——否则 D-10 退出码会被 terminate(true, 0) 抢跑覆盖
	// （plan 01-03 TestExitCodePropagation 暴露：exitf(0) 顶替 exitf(42)）。
	childExited atomic.Bool
	termOnce    sync.Once // 两条终结路径收口，exitf 只触发一次
}

// Options 为 New 的装配选项。
// Writable/PingInterval 为生产直传字段（main.go --writable/--ping-interval flag
// 原样透传，D-15/D-16；PingInterval 0 = 禁用保活）；
// HelloTimeout/MaxHalfOpenPerIP/PongTimeout 为测试可覆写字段（零值各取默认常量
// defaultHelloTimeout/defaultMaxHalfOpenPerIP/defaultPongTimeout，D-04/D-16）。
type Options struct {
	Writable         bool
	PingInterval     time.Duration
	HelloTimeout     time.Duration
	MaxHalfOpenPerIP int
	PongTimeout      time.Duration
}

// defaultHelloTimeout 未认证 Hello 超时默认值（D-04：5s）。
const defaultHelloTimeout = 5 * time.Second

// defaultMaxHalfOpenPerIP per-IP 半开（Hello 未完成）连接上限默认值（D-04：8）。
// 正常浏览器秒发 Hello 不受限；NAT 多人场景 Hello 已完成者不计入。
const defaultMaxHalfOpenPerIP = 8

// defaultPongTimeout 发出 ping 后等 pong 的时长默认值（D-16：10s——正常 RTT
// 毫秒级，10s 极宽）。只有 pong 超时才允许断开连接；读路径恒无 deadline
// （Pitfall 2），健康的长空闲会话永不因保活被误杀。
const defaultPongTimeout = 10 * time.Second

// New 装配服务端并钉死两个 goroutine 的启动点：
//   - sess.ReadLoop：自装配起持续 drain master（D-12），attach 前输出直接丢弃，
//     防 64KiB PTY 内核缓冲填满导致子进程写阻塞；attach 路径内不得再新建读循环。
//   - lifecycle：sess.Wait → 带时限 drain → 1000 关闭当前客户端 → exitf（D-10 触发源）。
//
// 装配契约：opts.Writable 决定 Welcome mode 与 INPUT 门（D-13/D-14/D-15）；
// opts.HelloTimeout 零值时取 defaultHelloTimeout。
func New(sess *pty.Session, exitf func(int), opts Options) *Server {
	if opts.HelloTimeout <= 0 {
		opts.HelloTimeout = defaultHelloTimeout
	}
	if opts.MaxHalfOpenPerIP <= 0 {
		opts.MaxHalfOpenPerIP = defaultMaxHalfOpenPerIP
	}
	if opts.PongTimeout <= 0 {
		opts.PongTimeout = defaultPongTimeout
	}
	s := &Server{
		sess:             sess,
		exitf:            exitf,
		writable:         opts.Writable,
		helloTimeout:     opts.HelloTimeout,
		maxHalfOpenPerIP: opts.MaxHalfOpenPerIP,
		pingInterval:     opts.PingInterval,
		pongTimeout:      opts.PongTimeout,
		frame:            make([]byte, 1+32*1024),
		halfOpen:         halfOpenCounter{n: make(map[string]int)},
	}
	s.frame[0] = proto.Output
	go sess.ReadLoop(s.onChunk)
	go s.lifecycle()
	return s
}

// Handler 挂两条路由：/ 走 go:embed 静态伺服，/ws 走 Attach。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	wh, err := web.Handler()
	if err != nil {
		// fs.Sub 仅在内嵌 FS 缺 dist 时失败（编译期 go:embed 已保证存在）；防御性 500。
		wh = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded assets unavailable", http.StatusInternalServerError)
		})
	}
	mux.Handle("/", wh)
	mux.HandleFunc("/ws", s.Attach)
	return mux
}

// halfOpenCounter 是 per-IP 半开（Hello 未完成）连接计数器（D-04）。
// 不变量（Pitfall 4）：acquire 成功后 release 恰好一次，发生在 Hello 完成或
// 任一拒绝/失败路径（Accept 失败 / assert 失败 / 409 拒绝 / 连接终结，先到为准）——
// 不泄漏（计数单调上涨最终正常用户全被 429）也不双重释放（计数归零后后续连接被误放行）。
type halfOpenCounter struct {
	mu sync.Mutex
	n  map[string]int
}

// acquire 在 ip 的半开计数未达 max 时 +1 并返回 true，否则返回 false（429 闸）。
func (h *halfOpenCounter) acquire(ip string, max int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n[ip] >= max {
		return false
	}
	h.n[ip]++
	return true
}

// release 将 ip 的半开计数 -1；到 0 删除 map key——防 map 随历史连接数单调增长
// （Pitfall 4 泄漏面）。恰好一次不变量由调用方（Attach 内 sync.Once）保证。
func (h *halfOpenCounter) release(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n[ip] <= 1 {
		delete(h.n, ip)
		return
	}
	h.n[ip]--
}

// clientIP 取对端 IP 作 per-IP 计数键：net.SplitHostPort 取主机部分（含端口直接
// 当键会使每连接一个"新 IP"，上限形同虚设——Pitfall 6），失败回退 RemoteAddr 整串。
// 反代部署下同键聚合为代理 IP 是已知限制（Pitfall 6）；X-Forwarded-For 信任属
// Phase 3 SEC-07，本 phase 不解析。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// headerHasToken 按 token 拆分比较逗号分隔头（Split "," + TrimSpace + EqualFold
// 逐 token），禁止 strings.Contains 整头匹配——防 wesh.v1.evil 前缀绕过
// （Pitfall 5 硬纪律；库 accept.go:357-368 headerTokens 同语义）。
func headerHasToken(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// Attach 是 /ws 的 attach handler（PATTERNS 称 serveWS）。
//
// 守卫区（Accept 前，HTTP 层零 WS 资源分配，顺序敏感）：
//
//	① D-03 子协议预检 400（最廉价无状态，扫描器/旧客户端最早被拦）；
//	② D-04 per-IP 半开上限 429（默认 8）——必须在 409 之前：409 在前则 429 在
//	   单客户端模型下结构性不可达（planner 裁决的 D-04 可触达性形态，RESEARCH 的
//	   TestHalfOpenPerIP429 映射亦无法构造）；被 409 拒的连接 acquire→release
//	   恰好一次，不残留计数；
//	③ D-09 409 单客户端原子门（Phase 5 才改）。
func (s *Server) Attach(w http.ResponseWriter, r *http.Request) {
	// ① D-03：子协议预检——按 token 拆分精确比较，拒绝整头匹配。
	if !headerHasToken(r.Header, "Sec-WebSocket-Protocol", proto.Subprotocol) {
		http.Error(w, "subprotocol wesh.v1 required", http.StatusBadRequest)
		return
	}
	// ② D-04：per-IP 半开上限。不变量：acquire 成功 → release 恰好一次，发生在
	// Hello 完成或任一拒绝/失败路径（先到为准，Pitfall 4）——局部 sync.Once +
	// defer 兜底覆盖一切 return 路径（含违规落读循环后的 reader 终结与正常会话终结），
	// 显式提前调用处理 409/Accept/assert 失败与握手成功升档点。
	ip := clientIP(r)
	if !s.halfOpen.acquire(ip, s.maxHalfOpenPerIP) {
		http.Error(w, "too many pending connections", http.StatusTooManyRequests)
		return
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { s.halfOpen.release(ip) }) }
	defer release()
	// ③ D-09：第二连接在 Accept 之前以 HTTP 409 拒绝。
	if !s.attached.CompareAndSwap(false, true) {
		release() // 被 409 拒的连接已持有半开名额，先释放不残留计数
		http.Error(w, "another client is already attached", http.StatusConflict)
		return
	}
	// logEvent 对端取值（D-12②）：Attach 入口保存 RemoteAddr。反代部署下聚合为
	// 代理 IP 是已知限制（Pitfall 6）；X-Forwarded-For 信任属 Phase 3 SEC-07，本 phase 不解析。
	remote := r.RemoteAddr
	// AcceptOptions：Subprotocols 一行开启协商回显（D-03）；压缩默认禁用（终端高熵
	// 数据无收益，D-17）；不跳过 Origin 校验——库默认同源校验（同 Host 放行、跨源拒绝）。
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{proto.Subprotocol},
	})
	if err != nil {
		release()               // Accept 失败即未 attach，半开名额随拒绝释放
		s.attached.Store(false) // 释放门位允许后续客户端
		return                  // Accept 失败已自动写 HTTP 错误响应
	}
	defer c.CloseNow()
	// D-03 双闸之二：Accept 后 assert 兜底（理论不可达——预检已拦正常路径；
	// logEvent 埋点在此建立，02-05 复核清单以此为准）。防御性退化：释放门位直返，
	// 不消耗单次生命周期。
	if c.Subprotocol() != proto.Subprotocol {
		s.logEvent(remote, websocket.StatusPolicyViolation, "subprotocol_required")
		_ = c.Close(websocket.StatusPolicyViolation, "subprotocol_required")
		release()
		s.attached.Store(false)
		return
	}
	// ctx 由 context.Background() 派生——禁止 r.Context()（hijack 后行为意外，官方
	// README 明示）；读路径永不带 deadline（Pitfall 2：deadline ctx 到期经库内
	// AfterFunc 关整条连接且无关闭帧，conn.go:188-199——长 idle 终端会话会被误杀）。
	// WithCancel 唯一用途：Attach 返回时 defer cancel 终结 pinger goroutine——
	// pinger 随本 handler 生命周期同生灭，进既有 wsDisconnected/terminate 单一
	// 收口，零新 exitf 分支（CONTEXT L92 硬约束）。cancel 只在读循环终结后触发，
	// 不打断在途读写。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// D-11 预认证窗口第一档：4KiB 读上限（Hello JSON ~100B，余量两个数量级）；
	// 库 limitReader 流式执行，SEC-08 预认证窗口单连接可占内存最小化。
	c.SetReadLimit(proto.ReadLimitPreAuth)
	// D-04 5s 未认证超时：time.AfterFunc + Close(1008)——只有 AfterFunc 才能把码值
	// 送上 wire（deadline ctx 无关闭帧，见上）。Close 是 5s+5s 握手阻塞调用，
	// 在 AfterFunc 自有 goroutine 里跑，不阻塞 reader（close.go:86-99）。
	helloDone := make(chan struct{})
	timer := time.AfterFunc(s.helloTimeout, func() {
		select {
		case <-helloDone:
		default:
			s.logEvent(remote, websocket.StatusPolicyViolation, "hello_timeout")
			_ = c.Close(websocket.StatusPolicyViolation, "hello_timeout")
		}
	})
	defer timer.Stop()

	// 握手状态机（RESEARCH Pattern 2）：首帧必须 Hello。违规路径只关 conn
	// （攻击面不发 Error 帧，D-06 零反馈）并落入数据面读循环——下一拍 c.Read
	// 错误经既有 wsDisconnected→terminate 单一路径收口（CONTEXT L92 硬约束：
	// 禁止新增 exitf 分支；与 Phase 1 unknown-frame 关闭路径同形）。
	_, data, err := c.Read(ctx)
	if err != nil {
		// 对端断开与 hello_timeout 关闭后的 reader 终结：走既有 D-11 收口。
		s.wsDisconnected()
		return
	}
	if len(data) == 0 {
		// OQ2 裁决：Hello 前空消息按畸形处理（1002 桶）。
		s.logEvent(remote, websocket.StatusProtocolError, "empty_frame")
		_ = c.Close(websocket.StatusProtocolError, "empty_frame")
	} else if data[0] != proto.Hello {
		// D-04 抢跑帧：1002 直关，不发 Error 帧。
		s.logEvent(remote, websocket.StatusProtocolError, "frame_before_hello")
		_ = c.Close(websocket.StatusProtocolError, "frame_before_hello")
	} else if h, ok := proto.DecodeHello(data[1:]); !ok {
		// D-05 1002 桶含畸形 Hello（DecodeHello 的未知字段忽略纪律不受影响）。
		s.logEvent(remote, websocket.StatusProtocolError, "malformed_hello")
		_ = c.Close(websocket.StatusProtocolError, "malformed_hello")
	} else if h.Version != proto.Subprotocol {
		// D-06 正常客户端路径：先 Error 帧后 1008；close reason 与 code 同名机器串
		// （D-07）。Error 写失败不补救——连接已死，Close 仍把码值送上。
		_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrVersionMismatch, "protocol version wesh.v1 required"))
		s.logEvent(remote, websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
		_ = c.Close(websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
	} else {
		// 升档序列（顺序敏感，PATTERNS 注意 5/6）：停 5s 计时器 → Hello 携首尺寸
		// Resize（消除 80x24 首帧窗口）→ per-IP release（Hello 完成即不计半开，
		// D-04：NAT 场景正常浏览器不受限）→ Welcome 下发 mode（D-14）→ 16KiB 稳态档
		// （SetReadLimit 经库 atomic store 下一条消息起生效，read.go:97-105）→
		// pinger 保活（D-16）→ conn 上线。conn 刻意在握手完成后才上线：此前 OUTPUT 一律 drain——
		// Welcome 恒为 S→C 首帧（dialHello 首帧断言无时序竞态），且未认证客户端
		// 在预认证窗口内收不到任何 PTY 输出。
		close(helloDone)
		s.sess.Resize(h.Cols, h.Rows)
		release()
		mode := proto.ModeRO
		if s.writable {
			mode = proto.ModeRW
		}
		// Welcome 写失败不补救——连接已死，读循环下一拍收口。
		_ = c.Write(ctx, websocket.MessageBinary, proto.WelcomeFrame(mode))
		c.SetReadLimit(proto.ReadLimitPostAuth)
		// CORE-06 保活：pinger 挂升档序列尾段（PATTERNS 注意 5），与既有单 reader
		// 循环并发装配——库硬性要求 Ping 必须与 Reader 并发（conn.go:218-220），
		// 不得为 ping 再开 reader；pong 由读循环 handleControl 自动处理
		// （read.go:317-337）；ping 与 onChunk 的 OUTPUT 写并发安全、无帧交错
		// （库 writeFrameMu 串行化所有帧，write.go:288-293）。
		go s.pinger(ctx, c, remote, s.pingInterval)
		s.conn.Store(c)
		defer s.conn.Store(nil)
	}

	// C→S：单 reader 循环（c.Read 不可并发，Pitfall 7）。
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			// 对端关闭（errors.As 可取出 CloseError）与网络断开同等处理：
			// 单次语义，任何 reader 终结都走 D-11 路径。
			s.wsDisconnected()
			return
		}
		if len(data) == 0 {
			continue // OQ2：Hello 完成后空消息维持静默跳过
		}
		switch data[0] {
		case proto.Input:
			if !s.writable {
				continue // D-13：ro 安全边界在服务端——静默丢弃（不打日志防按键洪水）
			}
			s.sess.Master.Write(data[1:])
		case proto.Resize:
			// JSON 解码失败静默丢弃（不关连接）；成功时已钳制 [1,1000]（D-16）。
			// D-13：ro 同放行——RESIZE 只改视图尺寸不改 shell 输入。
			if cols, rows, ok := proto.DecodeResize(data[1:]); ok {
				s.sess.Resize(cols, rows)
			}
		default:
			_ = c.Close(websocket.StatusProtocolError, "unknown frame type") // 1002，协议演化无歧义
		}
	}
}

// onChunk 是 S→C 数据泵（ReadLoop 回调，独占 WS 写端）：
// 未 attach 期间直接丢弃（D-12 drain 语义）；已 attach 组 OUTPUT 帧写 WS。
// 仅由 ReadLoop 单 goroutine 调用，故复用 s.frame 无竞争。
func (s *Server) onChunk(chunk []byte) {
	c := s.conn.Load()
	if c == nil {
		return // D-12：attach 前输出丢弃，防 PTY 内核缓冲写阻塞
	}
	n := copy(s.frame[1:], chunk)
	if err := c.Write(context.Background(), websocket.MessageBinary, s.frame[:1+n]); err != nil {
		return // 写失败（连接已死）：终结由 reader 路径收口（D-11），本块丢弃
	}
}

// pinger 是 CORE-06 保活 goroutine（D-16，RESEARCH Pattern 3 照抄语义）：
// 按 interval 周期发 WS ping——反代空闲超时（nginx 60s / Cloudflare 100s /
// 30s 型 ingress）看的是应用层流量，TCP keepalive 多数反代不计入，WS ping 才是对症解。
// interval <= 0 直接返回（--ping-interval 0 禁用，D-16：不发任何 ping，长空闲
// 连接保持——用户显式选择）。
//
// 三条源码核实纪律：
//  1. Ping 必须与 Reader 并发（conn.go:218-220 库硬性要求）——现有单 reader
//     循环天然满足，本 goroutine 与读循环并行，不得为 ping 再开 reader；
//  2. Ping 的 ctx 超时只返回错误、不关连接（conn.go:251-258 select 路径无
//     close）——应用须自行 CloseNow；对端已不应答，关闭握手无意义，客户端见
//     1006 属本地合成码，不违反 D-05（D-05 约束服务端 wire 发送）；
//  3. 写并发安全由库 writeFrameMu 串行化所有帧保证（write.go:288-293）——
//     ping 与 onChunk 的 OUTPUT 写无帧交错；pong 由读循环 handleControl
//     自动处理（read.go:317-337）。
//
// 终结挂点：Attach 返回时 defer cancel 触发 ctx.Done（或在途 Ping 的 pctx 随
// ctx 取消）——随既有 wsDisconnected/terminate 单一路径同生灭，零新 exitf 分支。
// ctx 已取消时的在途 Ping 错误是正常终结而非 pong 超时，直接返回不打事件。
func (s *Server) pinger(ctx context.Context, c *websocket.Conn, remote string, interval time.Duration) {
	if interval <= 0 {
		return // D-16：--ping-interval 0 = 禁用
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, s.pongTimeout)
		err := c.Ping(pctx)
		cancel()
		if err != nil {
			// 只有真正的 pong 超时才打事件 + CloseNow：Ping 对 pong 等待的 ctx 到期
			// 返回包装后的 DeadlineExceeded（conn.go:251-258）；父 ctx 是 WithCancel
			// 无 deadline，DeadlineExceeded 唯一来源即 pctx 到期。其余错误（连接已被
			// 对端关闭/写失败/Attach 返回 cancel 级联取消）都是正常终结路径，
			// 静默返回即可——误报 pong_timeout 会污染 stderr 事件流（D-12② 三要素
			// 语义失真），连接终结由既有 reader 路径收口，pinger 无需也不应补刀。
			if !errors.Is(err, context.DeadlineExceeded) {
				return
			}
			// pong 超时（pongTimeout 内未收到应答）：stderr 单行事件（D-12② 三要素；
			// code 记 1006 = 客户端将观测到的本地合成码，CloseNow 无关闭帧）+ CloseNow。
			// 断开后的 reader 终结走既有 wsDisconnected→terminate 收口。
			s.logEvent(remote, websocket.StatusAbnormalClosure, "pong_timeout")
			c.CloseNow()
			return
		}
	}
}

// logEvent 打 D-12② stderr 单行事件，三要素齐全：对端 remote、码值 code、
// reason 机器串。本期覆盖 hello_timeout/empty_frame/frame_before_hello/
// malformed_hello/version_mismatch/subprotocol_required（assert 兜底）/
// pong_timeout（02-04 保活）七处埋点；
// 库自动 1009 的 ErrMessageTooBig 钩子属 02-05。Phase 8 升级 slog 结构化日志
// （OPS-08），本期为过渡形态。remote 由调用方传 Attach 入口保存的 r.RemoteAddr——
// 反代部署下同键聚合为代理 IP 是已知限制（Pitfall 6），X-Forwarded-For 属
// Phase 3 SEC-07，本 phase 不解析。
func (s *Server) logEvent(remote string, code websocket.StatusCode, reason string) {
	fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
}

// lifecycle 是 D-10 路径触发源：子进程退出 → 带时限 drain（Pitfall 4）→
// 当前已 attach 客户端收 1000 正常关闭帧 → exitf（退出码 = 子进程退出码）。
func (s *Server) lifecycle() {
	err := s.sess.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	// 置位必须先于关 conn：随后的 1000 关闭帧必然终结 Attach 读循环（c.Read 返回
	// CloseError），wsDisconnected 凭此标志识别该路径并放弃 exitf 竞争（D-10 优先）。
	s.childExited.Store(true)
	s.sess.Drain(200 * time.Millisecond)
	if c := s.conn.Load(); c != nil {
		c.Close(websocket.StatusNormalClosure, "") // D-10：1000
	}
	s.terminate(false, code) // 子进程已退出，无需 SIGHUP
}

// wsDisconnected 是 D-11 路径：WS 断开（reader 返回错误）→ SIGHUP 子进程进程组 → exitf(0)。
// 若 childExited 已置位，说明读循环终结是 D-10 服务端 1000 关闭帧的必然结果，
// 而非客户端断开——直接返回，exitf 由 lifecycle 以子进程退出码收口。
func (s *Server) wsDisconnected() {
	if s.childExited.Load() {
		return
	}
	s.terminate(true, 0)
}

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
