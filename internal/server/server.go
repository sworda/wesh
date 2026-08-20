// Package server 提供 HTTP + WS 网关、数据泵与多客户端生命周期：
// 子进程退出是唯一终结路径（D-10——lifecycle 广播 1000 关闭全部客户端后 exitf）；
// 客户端断开只做注册表移除，不再触发 exitf/SIGHUP（Phase 5 多客户端必然推论，
// P1 D-11 单次语义终结）。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/web"
)

// Server 持有会话、客户端注册表与生命周期收口件。
// exitf 由 main 注入 os.Exit、测试注入捕获桩——生命周期必须可测，这是硬约束。
type Server struct {
	sess  *pty.Session
	exitf func(code int)

	// New 装配期固化、运行期只读，故七个 plain 字段无需 atomic：
	// writable 为 checkTicket/attachHandler 派生 per-client ticket mode 的全局
	// 来源（D-13/D-14；生效 mode 由 clients.go 判定矩阵落地，INPUT/RESIZE 门
	// 已多客户端化为 per-client c.mode 判定）；
	// writePolicy 为 D-05 写权限策略（owner|all，矩阵输入之一，值域常量
	// WritePolicyOwner/WritePolicyAll 见 clients.go）；
	// helloTimeout 为 D-04 未认证超时时长；
	// maxHalfOpenPerIP 为 D-04 per-IP 半开上限；pingInterval/pongTimeout 为
	// D-16 保活参数（均 Options 注入）；
	// clientPrefsRO/clientPrefsRW 为客户端偏好双档 blob（P4 D-13 Welcome 内嵌
	// 一次性下发，空 = 不出 prefs 键；05-03 D-13 双档分化：ro 档永不含 osc52
	// 键——即使全局 --osc52 开启，rw 档按全局 --osc52 下发——聚合期由 main
	// 产双 blob，服务端不透明透传不解析、不做运行期 JSON 手术，P5-6 纪律）。
	writable         bool
	writePolicy      string
	helloTimeout     time.Duration
	maxHalfOpenPerIP int
	pingInterval     time.Duration
	pongTimeout      time.Duration
	clientPrefsRO    json.RawMessage
	clientPrefsRW    json.RawMessage

	// 认证与传输安全装配（Phase 3，均 New 装配期固化、运行期只读）：
	// credentials 为 D-02 整站 Basic 凭据集（空 = 无认证模式）；
	// origins/originList 为 D-12 Origin 白名单（规范化集合查 originAllowed、
	// 规范化串切片喂 AcceptOptions.OriginPatterns，二者同源于 opts.Origins）；
	// tickets/throttle 为 SEC-02 一次性 ticket 表与 SEC-03 per-IP 退避计数器
	//（仅认证模式构造，无认证模式为 nil——核销分支整体跳过）；
	// tlsOn 仅驱动 securityHeaders 的 HSTS 分支（D-06）。
	credentials []Credential
	origins     map[string]struct{}
	originList  []string
	tickets     *ticketStore
	throttle    *throttleStore
	tlsOn       bool

	// hubMu 护注册表与 hub 临界区（R-07 单锁纪律；锁序 hubMu > outbox.mu——
	// writer drain 不持 hubMu，绝不反序同持）。registry 结构见 clients.go。
	// hubCond 为全局信用门 cond（RES-04）：挂 hubMu——信用门状态与注册表同锁
	//（R-07），New 内以 &s.hubMu 构造；Wait 原子释放 hubMu，故门闭合期间
	// detach/kick/attach/lifecycle 均可获锁做门重估（P5-7 死锁免除链路）。
	hubMu    sync.Mutex
	hubCond  *sync.Cond
	registry registry

	// 多客户端参数（New 装配期固化、运行期只读；零值兜底见 New，默认常量声明
	// 在 clients.go 并挂 Phase 9 标定注释）：outboxBytes 本 plan 消费（outbox
	// 字节容量）；maxClients/inputRate/inputBurst/resizeDebounce 本 plan 仅立
	// 字段，消费点分别在 05-07（503 闸）/05-05（INPUT 限速）/05-04（仲裁防抖）。
	outboxBytes    int
	maxClients     int
	inputRate      int
	inputBurst     int
	resizeDebounce time.Duration

	// halfOpen 为 D-04 per-IP 半开（Hello 未完成）连接计数器；
	// acquire/release 恰好一次不变量见 halfOpenCounter 类型注释。
	halfOpen halfOpenCounter

	termOnce sync.Once // 终结路径收口，exitf 只触发一次（唯一触发源 = lifecycle 子进程退出，D-10）
}

// Options 为 New 的装配选项。
// Writable/PingInterval 为生产直传字段（main.go --writable/--ping-interval flag
// 原样透传，D-15/D-16；PingInterval 0 = 禁用保活）；
// HelloTimeout/MaxHalfOpenPerIP/PongTimeout 为测试可覆写字段（零值各取默认常量
// defaultHelloTimeout/defaultMaxHalfOpenPerIP/defaultPongTimeout，D-04/D-16）。
// Phase 3 新增：Credentials/Origins/TLS 为生产直传字段（main 经 ParseCredential/
// NormalizeOrigin 构造，D-01/D-12；Credentials 空 = 无认证模式，Origins 与凭据
// 正交——--origin 无凭据也生效；TLS 仅驱动 HSTS 分支，D-06）；
// TicketTTL/ThrottleBase/ThrottleCap 为测试可覆写字段（零值各取
// defaultTicketTTL/defaultThrottleBase/defaultThrottleCap）。
// Phase 4 新增：ClientPrefsRO/ClientPrefsRW 为生产直传字段（main 经
// aggregateClientPrefs 聚合 --client-option + --osc52 产双档 blob，P4 D-13
// Welcome 内嵌一次性下发；空 = 不下发——服务端对 prefs 不透明透传不解析，
// 白名单/JSON 校验已在 --client-option parse 期完成。05-03 D-13 双档：attach
// Welcome 按生效 mode 选档（ro → ClientPrefsRO，rw → ClientPrefsRW），递补升格
// Welcome 必携 ClientPrefsRW——ro 档永不含 osc52 键，即使全局 --osc52 开启）。
// Phase 5 新增：OutboxBytes/MaxClients/InputRate/InputBurst/ResizeDebounce 为
// 测试可覆写字段（HelloTimeout 先例形态——零值各取默认常量，常量声明与 Phase 9
// 负载标定回填注释见 clients.go）；MaxClients/InputRate/InputBurst/ResizeDebounce
// 本 plan 仅立字段，消费点分别在 05-07/05-05/05-04。
// 05-03 新增：WritePolicy 为生产直传字段（main --write-policy flag 原样透传，
// D-05；零值兜底 WritePolicyOwner——安全默认；取值常量见 clients.go）。
type Options struct {
	Writable         bool
	WritePolicy      string
	PingInterval     time.Duration
	HelloTimeout     time.Duration
	MaxHalfOpenPerIP int
	PongTimeout      time.Duration
	Credentials      []Credential
	Origins          []string
	TLS              bool
	TicketTTL        time.Duration
	ThrottleBase     time.Duration
	ThrottleCap      time.Duration
	ClientPrefsRO    json.RawMessage
	ClientPrefsRW    json.RawMessage
	OutboxBytes      int
	MaxClients       int
	InputRate        int
	InputBurst       int
	ResizeDebounce   time.Duration
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
//   - sess.ReadLoop：自装配起持续 drain master（D-12），无客户端期间输出经 hub
//     空扇出自然丢弃，防 64KiB PTY 内核缓冲填满导致子进程写阻塞；attach 路径内
//     不得再新建读循环。
//   - lifecycle：sess.Wait → 带时限 drain → 广播 1000 关闭全部客户端 → exitf
//     （D-10 唯一终结路径的多客户端形态）。
//
// 装配契约：opts.Writable 是 per-client Welcome mode 与 INPUT 门的全局派生来源
//（D-13/D-14/D-15）；opts.HelloTimeout 零值时取 defaultHelloTimeout。
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
	if opts.OutboxBytes <= 0 {
		opts.OutboxBytes = defaultOutboxBytes
	}
	if opts.MaxClients <= 0 {
		opts.MaxClients = defaultMaxClients
	}
	if opts.InputRate <= 0 {
		opts.InputRate = defaultInputRate
	}
	if opts.InputBurst <= 0 {
		opts.InputBurst = defaultInputBurst
	}
	if opts.ResizeDebounce <= 0 {
		opts.ResizeDebounce = defaultResizeDebounce
	}
	if opts.WritePolicy == "" {
		opts.WritePolicy = WritePolicyOwner // D-05 安全默认
	}
	s := &Server{
		sess:             sess,
		exitf:            exitf,
		writable:         opts.Writable,
		writePolicy:      opts.WritePolicy,
		helloTimeout:     opts.HelloTimeout,
		maxHalfOpenPerIP: opts.MaxHalfOpenPerIP,
		pingInterval:     opts.PingInterval,
		pongTimeout:      opts.PongTimeout,
		registry:         registry{set: make(map[*client]struct{})},
		outboxBytes:      opts.OutboxBytes,
		maxClients:       opts.MaxClients,
		inputRate:        opts.InputRate,
		inputBurst:       opts.InputBurst,
		resizeDebounce:   opts.ResizeDebounce,
		halfOpen:         halfOpenCounter{n: make(map[string]int)},
		credentials:      opts.Credentials,
		tlsOn:            opts.TLS,
		clientPrefsRO:    opts.ClientPrefsRO,
		clientPrefsRW:    opts.ClientPrefsRW,
	}
	// D-12：Origin 白名单与凭据正交（--origin 无凭据也生效）；opts.Origins 为
	// main 已规范化的串（小写 host + 剥默认端口），集合供 originAllowed 精确查找、
	// 切片供 AcceptOptions.OriginPatterns。零配置时两字段为 nil——库默认同源
	// 校验与无 Origin 放行行为零漂移（D-12）。
	if len(opts.Origins) > 0 {
		s.origins = make(map[string]struct{}, len(opts.Origins))
		s.originList = make([]string, 0, len(opts.Origins))
		for _, o := range opts.Origins {
			s.origins[o] = struct{}{}
			s.originList = append(s.originList, o)
		}
	}
	// 认证模式（len(Credentials)>0）才构造 ticket/throttle 两 store；无认证模式
	// 两 store 为 nil——checkTicket 核销分支整体跳过（既有行为零漂移）。
	if len(opts.Credentials) > 0 {
		s.tickets = newTicketStore(opts.TicketTTL)
		s.throttle = newThrottleStore(opts.ThrottleBase, opts.ThrottleCap)
	}
	// 信用门 cond 挂 hubMu（R-07：信用门状态与注册表同锁；构造必须在 goroutine
	// 启动前——onChunk 的 Wait 与 detach/kick 的 Broadcast 均以它为挂点）。
	s.hubCond = sync.NewCond(&s.hubMu)
	go sess.ReadLoop(s.onChunk)
	go s.lifecycle()
	return s
}

// Handler 挂三条路由：/ 走 go:embed 静态伺服，/ws 走 Attach，POST /api/attach
// 走 ticket 签发。认证模式（D-02 整站 Basic）：/ 与 /api/attach 挂 basicAuth
// （/ws 不挂——ticket 即其认证）；/api/attach 守卫链 = 非 POST 405（方法模式 +
// 显式同文 fallback，见下）→ Origin 403 → 节流 429 → Basic 401 → 签发 200。
// 无认证模式 /api/attach 显式注册 404（前端探测信号：跳过 fetch 直连 WS；
// 显式注册避免依赖静态 handler 对 POST 的偶发行为，RESEARCH Pattern 1 决策）。
// 最外层 securityHeaders 包裹全部路由（含 /ws，D-06）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	wh, err := web.Handler()
	if err != nil {
		// fs.Sub 仅在内嵌 FS 缺 dist 时失败（编译期 go:embed 已保证存在）；防御性 500。
		wh = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded assets unavailable", http.StatusInternalServerError)
		})
	}
	if len(s.credentials) > 0 {
		mux.Handle("/", basicAuth(wh, s.credentials, s.throttle))
		mux.Handle("POST /api/attach", originMiddleware(basicAuth(http.HandlerFunc(s.attachHandler), s.credentials, s.throttle), s.origins))
		// 非 POST /api/attach → 405 + Allow: POST。方法模式的内建 405 回退仅在
		// 没有任何其它模式匹配时触发（GOROOT server.go:2699-2710 的 n==nil 分支）——
		// 会被 "/" 子树匹配吞掉，故显式注册同文 fallback 补齐守卫链第一闸；
		// "POST /api/attach" 比 "/api/attach" 更具体，POST 仍走上方完整链。
		mux.HandleFunc("/api/attach", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		})
	} else {
		mux.Handle("/", wh)
		mux.HandleFunc("POST /api/attach", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r) // 无认证模式探测信号（404 → 前端跳过 fetch 直连）
		})
	}
	mux.HandleFunc("/ws", s.Attach)
	return securityHeaders(mux, s.tlsOn)
}

// attachHandler 是 POST /api/attach 的 ticket 签发端点（SEC-02）：Basic 认证
// 通过后签发一次性 ticket（60s TTL、单次使用、绑定全局 --writable 模式，D-11）。
// D-11 请求体为空——MaxBytesReader 1KiB 上限纯防御，超限 413；响应
// Cache-Control: no-store（ticket 不可落缓存，RESEARCH Pattern 6 表）。
// 红线（SEC-01）：ticket 值禁止作为任何日志参数。
func (s *Server) attachHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	mode := proto.ModeRO
	if s.writable {
		mode = proto.ModeRW
	}
	ticket := s.tickets.issue(mode, time.Now())
	body, _ := json.Marshal(struct {
		Ticket string `json:"ticket"`
	}{Ticket: ticket}) // 固定 schema，json.Marshal 不会失败
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
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
// Phase 7 SEC-07，本 phase 不解析。
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
//	⓪ D-13 Origin 白名单 403——与库 Accept 内检查同语义前置，拒绝形态与
//	   /api/attach 一致且 HTTP 层可测（AcceptOptions.OriginPatterns 为库内
//	   二次校验，纵深防御，SEC-04）；
//	① D-03 子协议预检 400（最廉价无状态，扫描器/旧客户端最早被拦）；
//	② D-04 per-IP 半开上限 429（默认 8）——必须在容量闸之前：容量闸在前则 429
//	   结构性不可达（planner 裁决的 D-04 可触达性形态沿用；被拒连接 acquire→release
//	   恰好一次，不残留计数）；
//	③ max-clients 503 闸由 05-07 落地（原 D-09 409 单客户端原子门已随多客户端
//	   注册表拆除——P5-5：503 闸沿用本区顺序与 release 恰好一次纪律）。
func (s *Server) Attach(w http.ResponseWriter, r *http.Request) {
	// ⓪ D-13：Origin 白名单检查（Accept 前拒绝，HTTP 层可测）——通用文案不回显
	// Origin 值（无反射面）；无 Origin 头放行（非浏览器客户端零摩擦）。
	if !originAllowed(r, s.origins) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	// ① D-03：子协议预检——按 token 拆分精确比较，拒绝整头匹配。
	if !headerHasToken(r.Header, "Sec-WebSocket-Protocol", proto.Subprotocol) {
		http.Error(w, "subprotocol wesh.v1 required", http.StatusBadRequest)
		return
	}
	// ② D-04：per-IP 半开上限。不变量：acquire 成功 → release 恰好一次，发生在
	// Hello 完成或任一拒绝/失败路径（先到为准，Pitfall 4）——局部 sync.Once +
	// defer 兜底覆盖一切 return 路径（含违规落读循环后的 reader 终结与正常会话终结），
	// 显式提前调用处理 Accept/assert 失败与握手成功升档点。
	ip := clientIP(r)
	if !s.halfOpen.acquire(ip, s.maxHalfOpenPerIP) {
		http.Error(w, "too many pending connections", http.StatusTooManyRequests)
		return
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { s.halfOpen.release(ip) }) }
	defer release()
	// logEvent 对端取值（D-12②）：Attach 入口保存 RemoteAddr。反代部署下聚合为
	// 代理 IP 是已知限制（Pitfall 6）；X-Forwarded-For 信任属 Phase 7 SEC-07，本 phase 不解析。
	remote := r.RemoteAddr
	// AcceptOptions：Subprotocols 一行开启协商回显（D-03）；压缩默认禁用（终端高熵
	// 数据无收益，D-17）；OriginPatterns 为 D-12 白名单的库内二次校验（⓪ 已前置
	// 同语义检查，纵深防御）——nil 时保持库默认同源校验（同 Host 放行、跨源拒绝，
	// 零配置零漂移）。
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:   []string{proto.Subprotocol},
		OriginPatterns: s.originList,
	})
	if err != nil {
		release() // Accept 失败即未 attach，半开名额随拒绝释放
		return    // Accept 失败已自动写 HTTP 错误响应
	}
	defer c.CloseNow()
	// D-03 双闸之二：Accept 后 assert 兜底（理论不可达——预检已拦正常路径；
	// logEvent 埋点在此建立，02-05 复核清单以此为准）。防御性退化：直接 return，
	// 不进注册表。
	if c.Subprotocol() != proto.Subprotocol {
		logEvent(remote, websocket.StatusPolicyViolation, "subprotocol_required")
		_ = c.Close(websocket.StatusPolicyViolation, "subprotocol_required")
		release()
		return
	}
	// ctx 由 context.Background() 派生——禁止 r.Context()（hijack 后行为意外，官方
	// README 明示）；读路径永不带 deadline（Pitfall 2：deadline ctx 到期经库内
	// AfterFunc 关整条连接且无关闭帧，conn.go:188-199——长 idle 终端会话会被误杀）。
	// WithCancel 用途：终结该客户端的 pinger goroutine——升档后 cancel 归 client
	// 持有，detach/kick 路径触发（clients.go），Attach 返回时 defer cancel 幂等
	// 兜底（含未升档的违规路径）。cancel 只在读循环终结后触发，不打断在途读写。
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
			logEvent(remote, websocket.StatusPolicyViolation, "hello_timeout")
			_ = c.Close(websocket.StatusPolicyViolation, "hello_timeout")
		}
	})
	defer timer.Stop()

	// 握手状态机（RESEARCH Pattern 2）：首帧必须 Hello。违规路径只关 conn
	// （攻击面不发 Error 帧，D-06 零反馈）并落入数据面读循环——下一拍 c.Read
	// 错误直接 return 收口（未升档无客户端注册；多客户端推论：单次语义终结，
	// 违规终结不再触发 exitf，与 unknown-frame 关闭路径同形）。
	var cl *client // 升档后非 nil——读循环终结时 detach 的判定依据
	_, data, err := c.Read(ctx)
	if err != nil {
		// 预认证 4KiB 档超限（D-11）：库已自动 1009，补 stderr 事件（D-12②）。
		s.logIfMessageTooBig(remote, err)
		// 对端断开与 hello_timeout 关闭后的 reader 终结：无客户端注册，直接
		// return（多客户端推论：不进 exitf）。
		return
	}
	if len(data) == 0 {
		// OQ2 裁决：Hello 前空消息按畸形处理（1002 桶）。
		logEvent(remote, websocket.StatusProtocolError, "empty_frame")
		_ = c.Close(websocket.StatusProtocolError, "empty_frame")
	} else if data[0] != proto.Hello {
		// D-04 抢跑帧：1002 直关，不发 Error 帧。
		logEvent(remote, websocket.StatusProtocolError, "frame_before_hello")
		_ = c.Close(websocket.StatusProtocolError, "frame_before_hello")
	} else if h, ok := proto.DecodeHello(data[1:]); !ok {
		// D-05 1002 桶含畸形 Hello（DecodeHello 的未知字段忽略纪律不受影响）。
		logEvent(remote, websocket.StatusProtocolError, "malformed_hello")
		_ = c.Close(websocket.StatusProtocolError, "malformed_hello")
	} else if h.Version != proto.Subprotocol {
		// D-06 正常客户端路径：先 Error 帧后 1008；close reason 与 code 同名机器串
		// （D-07）。Error 写失败不补救——连接已死，Close 仍把码值送上。
		_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrVersionMismatch, "protocol version wesh.v1 required"))
		logEvent(remote, websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
		_ = c.Close(websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
	} else if mode, tok := s.checkTicket(ip, h.Ticket); !tok {
		// D-10 统一口径：节流中/过期/非法/重放 ticket → 同 Error{auth_failed}+1008，
		// close reason 与 code 同名机器串（D-07），各形态响应不可区分（无 oracle）。
		// 分支位置：version 检查之后、升档之前（Open Question 2 裁决——version 为
		// 公开协议信息先查，核销紧随其后）；核销全部在预认证 4KiB 读上限档内完成
		// （Hello JSON +ticket ~120B，D-11 两档纪律不变）。
		_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrAuthFailed, "authentication failed"))
		logEvent(remote, websocket.StatusPolicyViolation, proto.ErrAuthFailed)
		_ = c.Close(websocket.StatusPolicyViolation, proto.ErrAuthFailed)
	} else {
		// 升档序列（顺序敏感，PATTERNS 注意 5/6）：停 5s 计时器 → Hello 携首尺寸
		// Resize（消除 80x24 首帧窗口；多客户端仲裁与 ro 忽略闸 05-04 落地，本 plan
		// 为 last-wins 延续）→ per-IP release（Hello 完成即不计半开，
		// D-04：NAT 场景正常浏览器不受限）→ hubMu 内：模式判定矩阵（ticket 绑定
		// mode × --writable × write-policy × owner 在位 → 生效 mode + rwEligible
		// + 是否立 owner，clients.go decideModeLocked；ticket 绑定值 = 认证模式
		// checkTicket 核销返回（D-11），无认证模式为其 s.writable 派生值）→
		// 构造 client → Welcome 帧按生效 mode 与 prefs 双档选档（P4 D-13 空则不
		// 出键——旧前端零漂移；05-03 D-13：ro 档永不含 osc52）经 outbox 首条入队
		// → 注册表登记（+ owner 指针登记）→ 16KiB 稳态档（SetReadLimit 经库
		// atomic store 下一条消息起生效，read.go:97-105）→ writer + pinger 启动。
		// 注册刻意在握手完成后才发生：此前 OUTPUT 一律经 hub 空扇出丢弃——
		// Welcome 恒为 S→C 首帧（writer 全程唯一写端，FIFO 保证首帧无时序竞态，
		// P2 D-02 时序纪律），且未认证客户端在预认证窗口内收不到任何 PTY 输出。
		close(helloDone)
		s.sess.Resize(h.Cols, h.Rows)
		release()
		s.hubMu.Lock()
		effMode, rwEligible, becomeOwner := s.decideModeLocked(mode)
		cl = &client{
			conn:       c,
			remote:     remote,
			mode:       effMode,
			rwEligible: rwEligible,
			outbox:     newOutbox(s.outboxBytes),
			done:       make(chan struct{}),
			cancel:     cancel,
		}
		// Welcome 帧作为 outbox 首条入队（组帧函数零改动复用；空队列首帧
		// trySend 恒成功——Welcome ≪ cap），按生效 mode 选 prefs 档（ro 档永不
		// 含 osc52，D-13/P5-6）。握手期 Error 帧（version_mismatch/auth_failed）
		// 发生在注册前，维持直写不变。入队先于登记且全程持 hubMu——hub 扇出
		// 遍历注册表，若先登记 onChunk 可在 Welcome 前夹入 OUTPUT（首帧时序竞态）。
		prefs := s.clientPrefsRO
		if effMode == proto.ModeRW {
			prefs = s.clientPrefsRW
		}
		cl.outbox.trySend(proto.WelcomeFrame(effMode, prefs))
		s.registry.registerLocked(cl)
		if becomeOwner {
			s.registry.owner = cl // D-06：首个 rw attach 立 owner
		}
		// P5-7 统一挂点：attach 后门重估——新可写端加入信用集（其 creditBlocked
		// 恒 false），可能使「全体可写端均满」不再成立，等待中的信用门必须重估。
		s.hubCond.Broadcast()
		s.hubMu.Unlock()
		c.SetReadLimit(proto.ReadLimitPostAuth)
		// writer 是该连接全程唯一 WS 写端（clients.go）；pinger 保活（D-16）挂
		// 升档序列尾段（PATTERNS 注意 5），与既有单 reader 循环并发装配——库硬性
		// 要求 Ping 必须与 Reader 并发（conn.go:218-220），不得为 ping 再开
		// reader；pong 由读循环 handleControl 自动处理（read.go:317-337）；ping
		// 与 writer 的数据写并发安全、无帧交错（库 writeFrameMu 串行化所有帧，
		// write.go:288-293）。
		go s.writer(cl)
		go s.pinger(ctx, c, remote, s.pingInterval)
		// D-11：attach 完成向 PTY 前台进程组显式发一次 SIGWINCH 强制全屏程序重绘
		//（TIOCGPGRP → kill(-pgid)）——与仲裁 resize 是否发生无关（P5-3 本机实证：
		// Linux 同尺寸 TIOCSWINSZ 不发信号）；新客秒见画面，行内 shell 下次输出
		// 自然追上。TIOCGPGRP 失败/无前台进程组静默降级（pty/io.go）。
		s.sess.SignalForegroundGroup()
	}

	// C→S：单 reader 循环（c.Read 不可并发，Pitfall 7）。
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			// 稳态 16KiB 档超限（D-09 修订两层硬顶）：库已自动 1009，补 stderr
			// 事件（D-12②）。
			s.logIfMessageTooBig(remote, err)
			// 对端关闭（errors.As 可取出 CloseError）与网络断开同等处理：reader
			// 终结 → detach（注册表移除 + writer/pinger 终结），不进 exitf、不发
			// 任何信号（CONTEXT domain 必然推论：P1 D-11 单次语义终结）。
			if cl != nil {
				s.detach(cl)
			}
			return
		}
		if len(data) == 0 {
			continue // OQ2：Hello 完成后空消息维持静默跳过
		}
		switch data[0] {
		case proto.Input:
			// per-client mode 门（P2 D-13/D-14 的多客户端映射：per-client 判定
			// 替代全局 s.writable——本 plan 无降级路径故语义等价，owner 降级在
			// 05-03 落地）：ro 静默丢（不打日志防按键洪水）。cl == nil 为握手违规
			// 落入路径（连接已在关闭握手）——同样静默丢，绝不对未注册连接写 PTY。
			if cl == nil || cl.mode == proto.ModeRO {
				continue
			}
			s.sess.Master.Write(data[1:]) // 直写本 plan 保持（CR-01 输入队列在 05-05）
		case proto.Resize:
			// JSON 解码失败静默丢弃（不关连接）；成功时已钳制 [1,1000]（D-16）。
			// 多客户端仲裁与 ro 忽略闸在 05-04 落地，本 plan 为 last-wins 延续
			//（D-13 单客户端语境的 ro 放行语义）。
			if cols, rows, ok := proto.DecodeResize(data[1:]); ok {
				s.sess.Resize(cols, rows)
			}
		default:
			_ = c.Close(websocket.StatusProtocolError, "unknown frame type") // 1002，协议演化无歧义
		}
	}
}

// checkTicket 收口 Hello 携 ticket 的核销闸（D-08/D-10）：返回升档 mode 与是否
// 放行。无认证模式（s.tickets == nil）核销分支整体跳过——返回 s.writable 派生
// mode 且恒放行（既有行为零漂移）。认证模式顺序敏感：
//  1. 节流窗口内（throttle.allow false）→ 拒绝且不 recordFail——节流命中不延长
//     窗口，ticket 也不核销（wire 上与过期/非法同口径，D-10）；
//  2. redeem 失败（过期/非法/重放同归 false）→ throttle.recordFail 计入 D-08
//     统一计数器（与 /api/attach 凭据失败同一 per-IP store）后拒绝；
//  3. 核销成功 → 返回 ticket 绑定的 mode（D-11）。
//
// 红线（SEC-01）：ticket 值禁止作为任何日志参数——本方法与调用方均不打印。
func (s *Server) checkTicket(ip, ticket string) (string, bool) {
	mode := proto.ModeRO
	if s.writable {
		mode = proto.ModeRW
	}
	if s.tickets == nil {
		return mode, true // 无认证模式：核销分支整体跳过
	}
	now := time.Now()
	if !s.throttle.allow(ip, now) {
		return "", false // 节流命中：不 recordFail（不延长窗口），ticket 不核销
	}
	m, ok := s.tickets.redeem(ticket, now)
	if !ok {
		s.throttle.recordFail(ip, now) // D-08 统一计数器
		return "", false
	}
	return m, true
}

// onChunk（S→C fan-out hub）已迁至 clients.go——每 chunk 组一次共享帧，逐客户端
// outbox 非阻塞 trySend；未 attach 期间注册表为空，输出自然丢弃（D-12 drain 语义不变）。

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
// 终结挂点：detach/kick 触发 client.cancel（clients.go；Attach 返回时 defer
// cancel 幂等兜底），ctx.Done（或在途 Ping 的 pctx 随 ctx 取消）使本 goroutine
// 随该客户端生命周期同生灭——零新 exitf 分支（CONTEXT L92 硬约束）。
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
			// 断开后的 reader 终结经 detach 收口（注册表移除，不进 exitf）。
			logEvent(remote, websocket.StatusAbnormalClosure, "pong_timeout")
			c.CloseNow()
			return
		}
	}
}

// logEvent 打 D-12② stderr 单行事件，三要素齐全：对端 remote、码值 code、
// reason 机器串。本期覆盖 hello_timeout/empty_frame/frame_before_hello/
// malformed_hello/version_mismatch/subprotocol_required（assert 兜底）/
// pong_timeout（02-04 保活）/message_too_big（02-05 超限，经 logIfMessageTooBig
// 挂预认证首读与稳态读循环两处）/auth_failed（03-03 ticket 核销失败）/
// throttled（03-03 HTTP 层 429 节流闸，basicAuth）/slow_consumer（05-01 背压
// 1013 踢出，clients.go kickSlowConsumerLocked）。Phase 8 升级 slog 结构化日志
// （OPS-08），本期为过渡形态。remote 由调用方传 Attach 入口保存的 r.RemoteAddr——
// 反代部署下同键聚合为代理 IP 是已知限制（Pitfall 6），X-Forwarded-For 属
// Phase 7 SEC-07，本 phase 不解析。
//
// 红线（SEC-01）：凭据、ticket、Authorization 头任何形态（含 base64）禁止作为
// 任何参数传入（ttyd server.c:142 反例）——三要素只有 remote/code/reason。
// 包级函数（无 Server 状态依赖）：HTTP 层中间件（basicAuth）与 WS 握手段共用
// 唯一出口；HTTP 层事件 code 复用 HTTP 状态码值（websocket.StatusCode 底层 int，
// PATTERNS Shared Patterns 裁决）。
func logEvent(remote string, code websocket.StatusCode, reason string) {
	fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)
}

// logIfMessageTooBig 是 D-12② 超限可见性三腿之二的服务端钩子：库 limitReader
// 流式截断超限后自动把 1009 送上 wire 且 Read 返回包装 ErrMessageTooBig 的错误
// （read.go:521-541）——应用在此补 stderr 单行事件。库的 close reason 是库内
// 字符串 "read limited at N bytes" 不可定制（PATTERNS 注意 7），message_too_big
// 机器串落点在 stderr 而非线上 reason；禁止包装库或包装 conn 数帧（D-09 修订
// 反模式清单）。非超限错误（对端关闭/网络断开）不产生事件。稳态 16KiB 与预认证
// 4KiB 两档共用同一错误标识（SetReadLimit 仅数值不同），两处埋点同一调用形态。
func (s *Server) logIfMessageTooBig(remote string, err error) {
	if errors.Is(err, websocket.ErrMessageTooBig) {
		logEvent(remote, websocket.StatusMessageTooBig, "message_too_big")
	}
}

// lifecycle 是 D-10 唯一终结路径触发源（多客户端形态）：子进程退出 → 带时限
// drain（Pitfall 4）→ 广播 1000 关闭全部已注册客户端 → exitf（退出码 = 子进程
// 退出码，退出码传递语义不变）。
func (s *Server) lifecycle() {
	err := s.sess.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	s.sess.Drain(200 * time.Millisecond)
	// 广播 1000：hubMu 下取注册表快照后并行 Close 全部客户端（Close 自带 5s+5s
	// 上界——close.go:87-89，并行等待自然有界）。广播期间新 attach 或断开的路径
	// 均经 detach/reader 收口，不影响本快照的关闭语义；被关闭客户端的读循环随
	// CloseError 终结走 detach——detach 不进 exitf，终结由本路径独占（D-10）。
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
	// P5-7 统一挂点：lifecycle 广播关闭后补 Broadcast——被关闭客户端的 detach 已
	// 各自重估门，此处兜底（注册表空 → 门开 → ReadLoop 续 drain，D-12 语义不变）。
	s.hubMu.Lock()
	s.hubCond.Broadcast()
	s.hubMu.Unlock()
	s.terminate(code)
}

// terminate 以 sync.Once 收口终结路径，exitf 只触发一次（唯一调用方 =
// lifecycle——P1 D-11 单次语义终结后，wsDisconnected/SIGHUP 路径已消亡，
// CONTEXT domain 必然推论）。
func (s *Server) terminate(code int) {
	s.termOnce.Do(func() {
		s.exitf(code)
	})
}
