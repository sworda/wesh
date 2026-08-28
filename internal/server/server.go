// Package server 提供 HTTP + WS 网关、数据泵与多客户端生命周期：
// 子进程退出是唯一终结路径（D-10——lifecycle 广播 1000 关闭全部客户端后 exitf）；
// 客户端断开只做注册表移除，不再触发 exitf/SIGHUP（Phase 5 多客户端必然推论，
// P1 D-11 单次语义终结）。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/web"
)

// Server 持有会话、客户端注册表与生命周期收口件。
// exitf 由 main 注入 os.Exit、测试注入捕获桩——生命周期必须可测，这是硬约束。
type Server struct {
	sess  *pty.Session
	exitf func(code int)

	// startedAt 为会话起点（08-02，D-22）：New 尾部记录（goroutine 启动之前），
	// 与 pty.Start 时刻差毫秒级、审计语义无碍；session_end 事件的
	// duration_seconds 数据源。New 写入后运行期只读。
	startedAt time.Time

	// shares 为分享 token 两条目 store（05-06 MULTI-05，D-01 独立第三认证通道）：
	// New 装配期固化、运行期只读；nil = 通道关闭（ShareTokenRO/RW 任一空串，
	// 本 phase main 恒生成——nil 仅测试可构造）。结构与校验形态见 sharetoken.go。
	shares *shareTokens

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

	// arbiter 为 MULTI-04 resize 仲裁器（05-04，D-09 参与集分层 + 50ms 防抖 +
	// 即时重算双通道）：sizes 仅参与集成员（owner 模式仅 owner / all 模式全部
	// rw 端 / 纯 ro 会话全部 ro 端 Hello 首尺寸），结构与方法见 resize.go。
	// 全部字段 hubMu 保护（注册表同锁 R-07）；timer 回调自有 goroutine 取 hubMu。
	arbiter arbiter

	// inputQ 会话级字节有界输入队列（CR-01 完整背压修复，05-05，RESEARCH
	// Pattern 8）：各客户端读循环 INPUT 帧经 mode 门 → limiter.AllowN →
	// tryEnqueue 入队，单 input-writer goroutine 独占 sess.Master.Write——
	// 读循环零同步写。结构见 clients.go。inputDone 为 input-writer 终结信号
	//（lifecycle 子进程退出路径 close，Drain→Close 解除在途写阻塞——D-12
	// 同款 runtime poller 机制）。inputDrops 为限速丢弃计数（atomic——INPUT
	// 门每击键热路径无锁递增；Phase 8 OPS-07 进 metrics，review #10 挂点；
	// 与 inputQ.droppedInputs 队列满丢弃计数成对）。
	inputQ     *inputQ
	inputDone  chan struct{}
	inputDrops atomic.Int64

	// 多客户端参数（New 装配期固化、运行期只读；零值兜底见 New，默认常量声明
	// 在 clients.go 并挂 Phase 9 标定注释）：outboxBytes/resizeDebounce/
	// inputRate/inputBurst 已消费（outbox 字节容量 / resize.go 仲裁器防抖
	// reset / Attach 升档 limiter 构造，05-04/05-05）；maxClients 消费点已落地
	//（05-07：Attach 守卫区③位 503 闸 + /api/attach 503 早闸，判定源
	// registry.n）。
	outboxBytes    int
	maxClients     int
	inputRate      int
	inputBurst     int
	resizeDebounce time.Duration

	// halfOpen 为 D-04 per-IP 半开（Hello 未完成）连接计数器；
	// acquire/release 恰好一次不变量见 halfOpenCounter 类型注释。
	halfOpen halfOpenCounter

	// Phase 6 断开退出装配（06-02，SESS-01/02）：exitWhenEmpty/exitWhenEmptyGrace
	// 为 New 装配期固化、运行期只读（D-14 set/grace 分离——grace=0 是合法显式值
	//「最后一个客户端断开立即退出」，set 位由 bool 承载，禁止 <=0 零值兜底吞掉
	// 显式 0）；exitEmptyTimer 为宽限计时器（hubMu 保护——启停全在 hubMu 内：
	// detach/kick 两移除点启动、registerLocked 成功后取消、置 nil 恰好一次，
	// resize.go initArbiter 计时器先例；回调自有 goroutine，入内先取 hubMu 复查
	//『仍空且未 exiting』）；exiting 为 lifecycle 终结广播门（hubMu 保护——
	// lifecycle 注册表快照前置位：广播 Close 引发的 detach 致空属正常终结序列，
	// 空触发检查该位抑制，不得再生 SIGHUP/计时器）。
	exitWhenEmpty      bool
	exitWhenEmptyGrace time.Duration
	exitEmptyTimer     *time.Timer
	exiting            bool
	// closeBroadcastCode 为终结广播关闭码（hubMu 保护，0=未广播）：lifecycle
	// 子进程退出广播窗口与 exiting 置位同点置 1000（StatusNormalClosure），
	// Shutdown 1001 优雅下线同点置 1001（StatusGoingAway）——08-02 D-21
	// detach 事件 reason=shutdown 的 code 数据源（与广播关闭码同源单写口）。
	closeBroadcastCode int

	// Phase 7 stop-signal 装配（07-04，OPS-04，D-22）：stopSignal 为 exit-when-empty
	// 收口路径向子进程进程组所发信号（New 装配期固化、运行期只读；零值经 New
	// 兜底 SIGHUP——零信号无语义，兜底保持默认 HUP 现状语义）；stopTimeout 为
	// stop-signal 后补发 SIGKILL 的宽限（零值 = 不补 KILL 纯单信号，合法现状
	// 语义无兜底；AfterFunc 异步补发不占 hubMu、ESRCH 幂等静默，Pitfall 8）。
	// 07-05 Shutdown（1001 优雅下线）复用同一字段——退出收口的信号配置必须
	// 经 Options 单一通道，双写即漂移。
	stopSignal  syscall.Signal
	stopTimeout time.Duration

	// Phase 7 部署形态装配（07-01，D-13/D-14）：basePath 为 New 装配期固化、
	// 运行期只读的反代子路径挂载前缀（空串 = 根挂载）；Handler() 注册点统一
	// 消费，StripPrefix 仅包静态伺服链。
	basePath string

	// Phase 7 反代信任装配（07-03，SEC-07 D-15..D-20）：proxy 为 New 装配期
	// 固化、运行期只读的信任配置（AuthHeader 非空 = 信任闸开——XFF 换键与
	// remote_user 提取共用同一开关，D-20 零双轨；零值 = 不信任，行为与现状
	// 逐字节一致）。结构与提取语义见 proxy.go。
	proxy proxyInfo

	// Phase 8 探活状态位（08-03，OPS-06，D-10/D-11 数据源）：draining 为优雅
	// 关停位（Shutdown 入口置 true——与 s.exiting 同源触发点，1001 广播开始
	// 前即翻转，关停全程探活器不再导新流）；sessionAlive 为 PTY 会话存活位
	//（New 尾部置 true，lifecycle sess.Wait 返回与退出码提取完成后置
	// false——与 session_end 事件同区段）。/healthz handler 在 hubMu 外读取，
	// 故为 atomic.Bool（与 registry.n 的「hubMu 外 atomic load」选型先例
	// 同构，clients.go registry 注释论证）。
	draining     atomic.Bool
	sessionAlive atomic.Bool

	// Phase 8 运维端点装配（08-04，OPS-07）：version 为 New 装配期固化、运行期
	// 只读的构建版本（Options.Version 单一通道——main var version 原样透传，
	// 零值兜底 "dev"，发布构建 ldflags 注入属 Phase 9 既定），消费点 =
	// /metrics 的 wesh_build_info{version} label（escLabel 转义）；mc 为
	// metrics 热路径计数器集（结构与递增点见 metrics.go——atomic.Int64
	// 无锁递增，inputDrops 先例形态；快照读取端 = snapshotMetrics）。
	version string
	mc      metricsCounters

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
// 负载标定回填注释见 clients.go）；ResizeDebounce/InputRate/InputBurst 消费点
// 已落地（05-04 resize.go 仲裁器防抖 / 05-05 Attach 升档 limiter 构造 +
// INPUT 门 AllowN），MaxClients 消费点已落地（05-07 ③位 503 闸 + /api/attach
// 早闸）。
// 05-03 新增：WritePolicy 为生产直传字段（main --write-policy flag 原样透传，
// D-05；零值兜底 WritePolicyOwner——安全默认；取值常量见 clients.go）。
// 05-06 新增：ShareTokenRO/ShareTokenRW 为生产直传字段（main 经
// GenerateShareToken 启动生成两明文原样透传，server 只存 SHA-256 预哈希——
// D-01 第三认证通道；任一空串 = 通道关闭，本 phase main 恒生成）。
// 06-02 新增：ExitWhenEmpty/ExitWhenEmptyGrace 为测试可覆写字段（SESS-01/02，
// D-14）：ExitWhenEmpty 为 set 位（所有客户端断开后退出——默认 false = 现状
// 保持，无客户端时子进程继续运行，P5『断开不退出』产品承诺）；ExitWhenEmptyGrace
// 为断开退出宽限（0 是合法显式值 = 最后一个客户端断开立即退出——set/grace
// 分离，禁止 <=0 零值兜底吞掉显式 0，PATTERNS §3 注意项；仅在 ExitWhenEmpty
// 为 true 时有意义；负值无语义，New 防御性钳 0）。
type Options struct {
	Writable           bool
	WritePolicy        string
	PingInterval       time.Duration
	HelloTimeout       time.Duration
	MaxHalfOpenPerIP   int
	PongTimeout        time.Duration
	Credentials        []Credential
	Origins            []string
	TLS                bool
	TicketTTL          time.Duration
	ThrottleBase       time.Duration
	ThrottleCap        time.Duration
	ClientPrefsRO      json.RawMessage
	ClientPrefsRW      json.RawMessage
	OutboxBytes        int
	MaxClients         int
	InputRate          int
	InputBurst         int
	ResizeDebounce     time.Duration
	ShareTokenRO       string
	ShareTokenRW       string
	ExitWhenEmpty      bool
	ExitWhenEmptyGrace time.Duration
	// BasePath 为生产直传字段（07-01 D-13，main --base-path flag 经 parse 期
	// normalizeBasePath 严格校验后原样透传；零值 = 无前缀根挂载，New 不做任何
	// 兜底改写——Handler() 按零值装配与现状逐字节一致的注册形态）。
	BasePath string
	// AuthHeader 为生产直传字段（07-03 D-18，main --auth-header flag 原样透传，
	// Credentials/Origins 先例形态；空串 = 不信任反代头——XFF 完全忽略、
	// logEvent 不出 remote_user 键，D-20 单一信任闸）。值只做审计归因记录，
	// 不做任何认证决定（D-17 正交）。
	AuthHeader string
	// StopSignal/StopTimeout 为生产直传字段（07-04 D-22，main --stop-signal/
	// --stop-timeout flag 经 parse 期枚举/负值校验后原样透传）：StopSignal 为
	// exit-when-empty 收口路径的进程组信号（零值无语义，New 显式兜底 SIGHUP
	// 保持默认 HUP 现状语义——06-02 注释分档先例：生产直传 + 零值兜底说明）；
	// StopTimeout 为 stop-signal 后补发 SIGKILL 的宽限（零值 = 不补 KILL 纯单
	// 信号，合法现状语义，无兜底）。07-05 Shutdown 复用同一字段（Options 单一
	// 通道，双写即漂移）。
	StopSignal  syscall.Signal
	StopTimeout time.Duration
	// Version 为生产直传字段（08-04 OPS-07，main var version 原样透传——
	// 发布构建 ldflags 注入属 Phase 9 既定，本 phase 只落 plumbing；零值兜底
	// "dev"）。消费点 = /metrics 的 wesh_build_info{version} 单 label
	//（escLabel 转义，metrics.go）。
	Version string
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

// New 装配服务端并钉死三个 goroutine 的启动点：
//   - sess.ReadLoop：自装配起持续 drain master（D-12），无客户端期间输出经 hub
//     空扇出自然丢弃，防 64KiB PTY 内核缓冲填满导致子进程写阻塞；attach 路径内
//     不得再新建读循环。
//   - inputWriter：会话级单 input-writer（05-05，CR-01 完整修复）——独占
//     sess.Master.Write，消费 inputQ；lifecycle 子进程退出路径 close(inputDone)
//     收口（clients.go）。
//   - lifecycle：sess.Wait → 带时限 drain → 广播 1000 关闭全部客户端 → exitf
//     （D-10 唯一终结路径的多客户端形态）。
//
// 装配契约：opts.Writable 是 per-client Welcome mode 与 INPUT 门的全局派生来源
// （D-13/D-14/D-15）；opts.HelloTimeout 零值时取 defaultHelloTimeout。
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
	// D-14 set/grace 分离：grace=0 是合法显式值（最后一个客户端断开立即退出），
	// 禁止 <=0 零值兜底吞掉显式 0（PATTERNS §3 注意项）；负值无语义，防御性钳 0。
	if opts.ExitWhenEmptyGrace < 0 {
		opts.ExitWhenEmptyGrace = 0
	}
	// D-22：零信号无语义——显式兜底 SIGHUP 保持默认 HUP 现状语义（WritePolicy
	// 零值兜底同档先例；StopTimeout 零值 = 不补 KILL 合法，不兜底）。
	if opts.StopSignal == 0 {
		opts.StopSignal = syscall.SIGHUP
	}
	// 08-04 OPS-07：version 零值兜底 "dev"（Options 注释分档先例——生产直传
	// + 零值兜底说明；main var version 恒非空，兜底服务测试直构造形态）。
	if opts.Version == "" {
		opts.Version = "dev"
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
		inputQ:           newInputQ(defaultInputQueueBytes),
		inputDone:        make(chan struct{}),
		halfOpen:         halfOpenCounter{n: make(map[string]int)},
		credentials:      opts.Credentials,
		tlsOn:            opts.TLS,
		clientPrefsRO:    opts.ClientPrefsRO,
		clientPrefsRW:    opts.ClientPrefsRW,
		// New 装配直传（06-02，D-14；set/grace 分离，grace 负值已在上方钳 0）。
		exitWhenEmpty:      opts.ExitWhenEmpty,
		exitWhenEmptyGrace: opts.ExitWhenEmptyGrace,
		// New 装配直传（07-04，D-22；StopSignal 零值已在上方兜底 SIGHUP）。
		stopSignal:  opts.StopSignal,
		stopTimeout: opts.StopTimeout,
		// New 装配直传（07-01，D-13；零值 = 根挂载，无兜底改写）。
		basePath: opts.BasePath,
		// New 装配直传（07-03，D-18/D-20）：AuthHeader 非空 = 信任闸开（XFF 换键
		// 与 remote_user 提取共用同一开关，零双轨）；空串 = 零值不信任。
		proxy: proxyInfo{trust: opts.AuthHeader != "", userHeader: opts.AuthHeader},
		// New 装配直传（08-04，OPS-07；零值已在上方兜底 "dev"）。
		version: opts.Version,
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
	// 05-06 分享通道（D-01/D-02）：share token 存在时 ticket store 必须构造——
	// token → /api/attach → 一次性 ticket 链路在无认证模式同样兑现（OQ1 正交：
	// mode 绑定在无密码演示场景有效）；throttle 仍仅凭据模式构造（无凭据无
	// Basic/401 面——checkTicket 对 nil throttle 有守卫；token 通道 128bit 空间
	// 使无节流枚举无意义，T-05-05）。
	s.shares = newShareTokens(opts.ShareTokenRO, opts.ShareTokenRW)
	if len(opts.Credentials) > 0 || s.shares != nil {
		s.tickets = newTicketStore(opts.TicketTTL)
	}
	if len(opts.Credentials) > 0 {
		s.throttle = newThrottleStore(opts.ThrottleBase, opts.ThrottleCap)
	}
	// 信用门 cond 挂 hubMu（R-07：信用门状态与注册表同锁；构造必须在 goroutine
	// 启动前——onChunk 的 Wait 与 detach/kick 的 Broadcast 均以它为挂点）。
	s.hubCond = sync.NewCond(&s.hubMu)
	// MULTI-04 仲裁器装配（resize.go）：timer 初始化为 stopped 态——首次
	// reportResize 的 Reset 才武装；必须在 ReadLoop/lifecycle 启动前完成。
	s.initArbiter()
	// 08-02 D-17/D-22：会话起点记录 + session_start 事件（进程级：pid 键 =
	// 子进程 PID；无 remote/code 键）——goroutine 启动之前 emit（审计事件
	// 先于任何连接/会话流量落流）。
	s.startedAt = time.Now()
	emitEvent(slog.String("event", "session_start"), slog.Int("pid", sess.Cmd.Process.Pid))
	// 08-03 D-10：会话存活位置位——goroutine 启动前（/healthz session_active
	// 数据源；置 false 挂点在 lifecycle sess.Wait 返回区段）。
	s.sessionAlive.Store(true)
	go sess.ReadLoop(s.onChunk)
	go s.inputWriter() // CR-01：input-writer 唯一装配点——master 写路径独占在专属 goroutine
	go s.lifecycle()
	return s
}

// Handler 挂四条路由：/ 走 go:embed 静态伺服，/ws 走 Attach，POST /api/attach
// 走 ticket 签发，GET /s/{token}/ 走分享链接页面门禁（05-06，sharetoken.go）。
// 认证模式（D-02 整站 Basic）：/ 与 /api/attach 挂 basicAuth（/ws 不挂——
// ticket 即其认证）；/api/attach 守卫链 = 非 POST 405（方法模式 + 显式同文
// fallback，见下）→ Origin 403 → 节流 429 → Basic 401 → 签发 200；分享 token
// 分支在守卫链之前 peek（D-01 第三通道——命中按 token 绑定 mode 直接签发，
// 绕过 Basic/throttle，capability 语义 R-03；未携/错 token 委托原链零改动）。
// 无认证模式 /api/attach 显式注册 404（前端探测信号：跳过 fetch 直连 WS；
// 显式注册避免依赖静态 handler 对 POST 的偶发行为，RESEARCH Pattern 1 决策）——
// 05-06 起仅当 body 携有效 token 时非 404（OQ1 正交：ro/rw mode 绑定在无密码
// 演示场景兑现）；G-05-7（2026-08-22 裁决）起携错 token 返 401（前端 C-3 承接）。
// /s/ 两路由凭据与无认证模式均注册（同 OQ1）。
// 最外层 securityHeaders 包裹全部路由（含 /ws，D-06）。
// 07-01 base-path 前缀装配（D-13/D-14，RESEARCH Pattern 2）：bp 为 parse 期已
// 校验值（/wesh 形态，空串 = 根挂载）。StripPrefix 仅包静态伺服链（wh 或
// basicAuth(wh)——全部 handler 中唯一路径敏感者）；sharePage 自改写
// r.URL.Path="/"（sharetoken.go:87-96）与 Attach/attachHandler/issueTicketJSON
// 均路径无关，注册模式串直接带 bp 前缀。注册 bp+"/" 子树后裸 {bp} 由 mux
// matchOrRedirect 307 补斜杠（GOROOT server.go:2687,2721-2745——D-14 尾斜杠
// 规范化零自写代码；StripPrefix 404 分支结构性不可达，T-07-01b）；bp==""
// 时注册形态与现状逐字节一致（零漂移兜底由 TestBasePathEmptyUnchanged 锁定）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	wh, err := web.Handler()
	if err != nil {
		// fs.Sub 仅在内嵌 FS 缺 dist 时失败（编译期 go:embed 已保证存在）；防御性 500。
		wh = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded assets unavailable", http.StatusInternalServerError)
		})
	}
	bp := s.basePath
	if len(s.credentials) > 0 {
		root := basicAuth(wh, s.credentials, s.throttle, s.proxy, &s.mc)
		if bp != "" {
			mux.Handle(bp+"/", http.StripPrefix(bp, root))
		} else {
			mux.Handle("/", root)
		}
		s.registerShareRoutes(mux, bp, wh, root)
		// 分享 token 分支包装（05-06 D-01）：先做 token peek——命中按绑定 mode
		// 签发（有效 token 优先于 throttle 直接放行，capability 语义：避免 NAT
		// 出口 IP 误伤持票旁观者，R-03）；未携/错 token 委托原链（401 同文同码
		// 无 oracle，失败经 recordFail 计入 D-08 统一 per-IP 计数器）。token
		// 分支先于 Origin 中间件是刻意排序：/ws Attach 守卫区 ⓪ 位的 Origin 检查
		// 对 ticket 核销后的 WS 握手依然生效（纵深不变），跨站表单无从获知
		// 128bit token，本面无 CSRF 增量。
		attachChain := originMiddleware(basicAuth(http.HandlerFunc(s.attachHandler), s.credentials, s.throttle, s.proxy, &s.mc), s.origins)
		mux.Handle("POST "+bp+"/api/attach", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.shareAttach(w, r) == shareHandled {
				return
			}
			attachChain.ServeHTTP(w, r)
		}))
		// 非 POST /api/attach → 405 + Allow: POST。方法模式的内建 405 回退仅在
		// 没有任何其它模式匹配时触发（GOROOT server.go:2699-2710 的 n==nil 分支）——
		// 会被 "/" 子树匹配吞掉，故显式注册同文 fallback 补齐守卫链第一闸；
		// "POST /api/attach" 比 "/api/attach" 更具体，POST 仍走上方完整链。
		//（bp 形态同理——内建 405 会被 bp+"/" 子树吞掉，fallback 同带前缀注册，
		// RESEARCH Pitfall 4 单侧定义防线。）
		mux.HandleFunc(bp+"/api/attach", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		})
		// 08-04 OPS-07（D-08/D-09）：/metrics 跟随认证闸（与 / 同一条 basicAuth
		// 链——无/错凭据 401 同文、节流 429 同 store；Prometheus scrape_config
		// 原生 basic_auth 可采集），根路径固定不带 bp（采集器直连后端端口，
		// 路径恒定可写死进 Prometheus 静态配置；拒绝双挂，单侧定义纪律）。
		mux.Handle("GET /metrics", basicAuth(http.HandlerFunc(s.metricsHandler), s.credentials, s.throttle, s.proxy, &s.mc))
	} else {
		if bp != "" {
			mux.Handle(bp+"/", http.StripPrefix(bp, wh))
		} else {
			mux.Handle("/", wh)
		}
		s.registerShareRoutes(mux, bp, wh, wh) // 无认证模式 page/root 同为 wh——给页无门
		mux.HandleFunc("POST "+bp+"/api/attach", func(w http.ResponseWriter, r *http.Request) {
			// OQ1 正交（用户 2026-08-19 裁决）：body 携有效 token → 按绑定 mode
			// 签发 ticket（ro/rw mode 绑定在无密码演示场景兑现）；未携 token →
			// 404 探测信号不变（前端直连 WS 既有形态）。
			// G-05-7（用户 2026-08-22 裁决）：携错 token → 401——前端既有
			//「携 token 401 → C-3 Invalid share link」分支直接承接（零前端改动）；
			// 无 WWW-Authenticate 挑战头（无认证模式无凭据可弹，挑战头只会诱导
			// 浏览器对导航式请求弹空框）；无 recordFail（throttle 语义锚定凭据
			// 失败，无认证模式本无节流面）。body 与凭据链 401 同一 authRequiredBody。
			switch s.shareAttach(w, r) {
			case shareHandled:
				return
			case shareInvalid:
				http.Error(w, authRequiredBody, http.StatusUnauthorized)
				return
			default: // shareAbsent
				http.NotFound(w, r) // 无认证模式探测信号（404 → 前端跳过 fetch 直连）
			}
		})
		// bp 形态单侧定义防线（07-01，RESEARCH Pitfall 4）：非 POST 打
		// {bp}/api/attach → 405 + Allow: POST——path-only fallback 显式注册，
		// 否则落入 bp+"/" 子树经 StripPrefix 漏进 embed FS 的 404（与凭据分支
		// 守卫链第一闸同文同码）。根挂载（bp==""）不注册本 fallback——无认证
		// 模式现状 404（embed FS）逐字节保持（零漂移红线），两形态差异由此
		// 注释锚定。
		if bp != "" {
			mux.HandleFunc(bp+"/api/attach", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Allow", http.MethodPost)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			})
		}
		// 08-04 OPS-07（D-08）：--no-auth 模式 /metrics 直通（无凭据无 Basic 面；
		// 暴露面 README 明示义务随 08-05，RESEARCH Pitfall 6）。
		mux.HandleFunc("GET /metrics", s.metricsHandler)
	}
	mux.HandleFunc(bp+"/ws", s.Attach)
	// 08-03 OPS-06（D-07/D-09）：/healthz 根路径固定注册——不带 bp 前缀
	//（探活/采集器直连后端端口，路径恒定可写死进 k8s probe/反代静态配置；
	// 拒绝双挂，单侧定义纪律），注册点在认证/无认证两分支之外唯一一处——
	// 免认证窄例外（D-07：整站 Basic 闸唯一例外，探活器结构性带不了凭据 +
	// 端点零敏感信息双前提；防例外蔓延：新端点不得以此为例外先例，README
	// 明示义务随 08-05）。方法模式 + path-only 405 fallback 成对注册
	//（sharetoken.go registerShareRoutes 先例——内建 405 会被 "/" 子树
	// 吞掉，RESEARCH Pitfall 7）。
	mux.HandleFunc("GET /healthz", s.healthzHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})
	// 08-04 OPS-07（D-09）：/metrics path-only 405 fallback（Allow: GET）——
	// 与 /healthz 成对同区（方法模式的内建 405 会被 "/" 子树吞掉，sharetoken.go
	// registerShareRoutes 先例，RESEARCH Pitfall 7）；注册在认证两分支之外——
	// POST /metrics 两模式同文 405，fallback 不包认证（/api/attach 405 先例
	// 同形态；GET 方法模式仍走上方各自分支的认证语义）。
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})
	return securityHeaders(mux, s.tlsOn)
}

// shareResult 是 shareAttach 的三态结果（G-05-7 拆分「未携」与「携错」——
// 无认证分支对两者响应不同：未携 404 探测信号不变，携错 401 供前端 C-3 承接）。
type shareResult int

const (
	shareAbsent  shareResult = iota // 未携 token（含解析失败/超体/通道关闭）
	shareInvalid                    // 携 token 但 lookup 未命中
	shareHandled                    // 命中，已按绑定 mode 签发 ticket
)

// shareAttach 是 POST /api/attach 的分享 token peek 分支（05-06 D-01 第三认证
// 通道）：body JSON{"token":...}（MaxBytesReader 4KiB 防御放大）解析且 lookup
// 命中 → 按 token 绑定 mode 签发一次性 ticket 并返回 shareHandled（/api/attach
// 不收 mode 参数——mode 由 token 绑定，P3 D-11 可选 mode 参数预期细化作废）。
// 未携 token → shareAbsent、携错 token → shareInvalid；两者均恢复 body 供委托
// 链重读（凭据模式调用方不区分——委托原链：Origin→Basic 401 同文同码无 oracle；
// 无认证模式调用方按 G-05-7 分派 404/401）。
// 解析失败一律按未携 token 处理——不回显 body 内容（T-05-06：错误响应面零泄露）。
// 红线（D-03/SEC-01）：token 值永不入 logEvent/错误响应。
func (s *Server) shareAttach(w http.ResponseWriter, r *http.Request) shareResult {
	if s.shares == nil {
		return shareAbsent // 通道关闭：零读取零行为变化（本 phase main 恒生成，防御兜底）
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	_ = r.Body.Close()
	// 恢复 body 供委托链重读（attachHandler 自携 1KiB MaxBytesReader 重包装——
	// 超限请求仍 413 同文）；err != nil（body >4KiB）按未携 token 处理。
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return shareAbsent
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Token == "" {
		return shareAbsent // 解析失败 = 未携 token（未知字段忽略纪律同 Hello）
	}
	mode, ok := s.shares.lookup(req.Token)
	if !ok {
		return shareInvalid
	}
	s.issueTicketJSON(w, r, mode) // OQ2 早闸在签发点内（满员 → 503 而非 ticket）
	return shareHandled
}

// issueTicketJSON 签发绑定 mode 的一次性 ticket 并以 {"ticket":...} JSON 响应
// （Cache-Control: no-store——ticket 不可落缓存，RESEARCH Pattern 6 表）。
// 两签发通道共用（tickets.go mode 注释兑现）：Basic 通道 = 全局 --writable 派生
// mode（attachHandler）；分享 token 通道 = token 绑定 mode（shareAttach）。
// 红线（SEC-01）：ticket 值禁止作为任何日志参数。
//
// OQ2 容量早闸（05-07，用户 2026-08-19 裁决）：签发 ticket 前 atomic load 满员
// 即 503——本函数是 Basic 链与 token 分支的唯一共享签发点，一处检查两通道同查；
// 前端 fetch 阶段即可给 Server is full 专版文案（05-08 UI-SPEC C-2）。一处检查
// 两处用：WS 侧③位 503 兜底竞态窗口（早闸后 ticket 已签但握手时满员 → WS 侧
// 503，语义无害）。logEvent 与③位同 reason 串 max_clients（P3 纪律：HTTP 层
// 401/429 经 basicAuth/throttle 内 logEvent 落事件，503 早闸同款保持可观测
// 一致性；双点位同串——同一容量事件的两个观测面）。07-03：remote 走
// s.proxy.remote（D-20 XFF 换键）并携 s.proxy.remoteUser 第四字段（D-15 审计
// 归因——两签发通道经同一提取点天然同口径，07-03 Task 2 双通道测试锁定）。
func (s *Server) issueTicketJSON(w http.ResponseWriter, r *http.Request, mode string) {
	if s.registry.n.Load() >= int64(s.maxClients) {
		logEvent(s.proxy.remote(r), websocket.StatusCode(http.StatusServiceUnavailable), "max_clients", s.proxy.remoteUser(r))
		http.Error(w, "server is full", http.StatusServiceUnavailable)
		return
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

// attachHandler 是 POST /api/attach 的 ticket 签发端点（SEC-02）：Basic 认证
// 通过后签发一次性 ticket（60s TTL、单次使用、绑定全局 --writable 模式，D-11）。
// 分享 token 通道的 body peek 在本函数之前的 shareAttach 完成（05-06——命中即
// 按 token 绑定 mode 直接签发，不进本函数）；走到此处 body 一律按 D-11 空体
// 预期丢弃——MaxBytesReader 1KiB 上限纯防御，超限 413。
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
	s.issueTicketJSON(w, r, mode)
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

// 07-03：包级 clientIP(r) 自由函数已删除——per-IP 键与 logEvent remote 取值
// 统一收编到 proxyInfo 方法（proxy.go：clientIP/remote/remoteUser），XFF 信任
// 闸由 --auth-header 承载（D-20，SEC-07 兑现）；反代部署下同键聚合为代理 IP
// 的旧限制（Pitfall 6）随 trust 开启解除。

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
//	③ RES-03 max-clients 容量闸 503（05-07，D-08 公开契约，默认 32）——原 D-09
//	   409 单客户端原子门已随多客户端注册表拆除，本位重建为容量闸。必须在
//	   ② halfOpen acquire 之后：满员时攻击者不得占半开名额（P5-5）；计数口径
//	   R-06（注册成功后计数，clients.go registry.n atomic load 免锁判定）；
//	   拒绝路径 release() 恰好一次（02-03 sync.Once + defer 兜底先例）。
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
	// 07-03（D-20）：ip 键走 s.proxy.clientIP——trust 开启时换 XFF 链首（反代后
	// per-IP 计数聚合回代理 IP 的旧限制解除）；未配置与现状逐字节一致。
	ip := s.proxy.clientIP(r)
	if !s.halfOpen.acquire(ip, s.maxHalfOpenPerIP) {
		http.Error(w, "too many pending connections", http.StatusTooManyRequests)
		return
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { s.halfOpen.release(ip) }) }
	defer release()
	// logEvent 对端取值（D-12② + 07-03 D-15/D-20）：Attach 入口统一提取一次——
	// trust 开启时 remote 换 XFF 链首（与上方 halfOpen/checkTicket 的 ip 键同源，
	// 日志归因与节流计数不分叉）、remoteUser 取配置头 sanitize 值；未配置与现状
	// 逐字节一致（XFF 完全忽略、不出 remote_user 键，D-20 单一信任闸）。
	// share token 渠道进入的 WS attach 同样经本提取点——两签发通道（Basic 链
	// attachHandler / token 分支 shareAttach）共享同一 Attach 入口，token 渠道
	// 客户端的 remote/remoteUser 来自同一行代码（07-03 Task 2 双通道测试锁定）。
	remote := s.proxy.remote(r)
	remoteUser := s.proxy.remoteUser(r)
	// ③ RES-03 max-clients 容量闸（05-07，D-08）：满员在 Accept 前以 HTTP 503
	// 拒绝（守卫区既有形态延伸，零 WS 资源分配）。计数口径 R-06：注册成功后计数
	//（registry.n atomic load——本闸在 hubMu 外，不得取锁），半开连接不计入
	//（与 ② halfOpen 正交两闸）；并发握手竞态最坏瞬时超编 ≤ per-IP 半开帽 8
	//（容量策略非安全边界，RESEARCH A5 裁断接受，注释明示）。拒绝路径 release()
	// 恰好一次（P5-5：被 503 拒的连接已持有半开名额，先释放不残留计数——
	// sync.Once 幂等 + defer 兜底，02-03 先例）。logEvent code 复用 HTTP 状态码
	// 值（R-10/P3 HTTP 层事件裁决——logEvent 签名 code 为 websocket.StatusCode，
	// 强转同 auth.go 401/429 先例）。
	if s.registry.n.Load() >= int64(s.maxClients) {
		release()
		logEvent(remote, websocket.StatusCode(http.StatusServiceUnavailable), "max_clients", remoteUser)
		http.Error(w, "server is full", http.StatusServiceUnavailable)
		return
	}
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
		logEvent(remote, websocket.StatusPolicyViolation, "subprotocol_required", remoteUser)
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
			logEvent(remote, websocket.StatusPolicyViolation, "hello_timeout", remoteUser)
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
		s.logIfMessageTooBig(remote, err, remoteUser)
		// 对端断开与 hello_timeout 关闭后的 reader 终结：无客户端注册，直接
		// return（多客户端推论：不进 exitf）。
		return
	}
	// 08-04 OPS-07（D-04）：WS 上行流量站点一——Hello 首读计入（RESEARCH A4：
	// 两站点计入，忠实「WS 网络流量」字面；~100B/连接，运维语义无碍）。
	s.mc.wsRecvBytes.Add(int64(len(data)))
	if len(data) == 0 {
		// OQ2 裁决：Hello 前空消息按畸形处理（1002 桶）。
		logEvent(remote, websocket.StatusProtocolError, "empty_frame", remoteUser)
		_ = c.Close(websocket.StatusProtocolError, "empty_frame")
	} else if data[0] != proto.Hello {
		// D-04 抢跑帧：1002 直关，不发 Error 帧。
		logEvent(remote, websocket.StatusProtocolError, "frame_before_hello", remoteUser)
		_ = c.Close(websocket.StatusProtocolError, "frame_before_hello")
	} else if h, ok := proto.DecodeHello(data[1:]); !ok {
		// D-05 1002 桶含畸形 Hello（DecodeHello 的未知字段忽略纪律不受影响）。
		logEvent(remote, websocket.StatusProtocolError, "malformed_hello", remoteUser)
		_ = c.Close(websocket.StatusProtocolError, "malformed_hello")
	} else if h.Version != proto.Subprotocol {
		// D-06 正常客户端路径：先 Error 帧后 1008；close reason 与 code 同名机器串
		// （D-07）。Error 写失败不补救——连接已死，Close 仍把码值送上。
		_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrVersionMismatch, "protocol version wesh.v1 required"))
		logEvent(remote, websocket.StatusPolicyViolation, proto.ErrVersionMismatch, remoteUser)
		_ = c.Close(websocket.StatusPolicyViolation, proto.ErrVersionMismatch)
	} else if mode, tok := s.checkTicket(ip, h.Ticket); !tok {
		// D-10 统一口径：节流中/过期/非法/重放 ticket → 同 Error{auth_failed}+1008，
		// close reason 与 code 同名机器串（D-07），各形态响应不可区分（无 oracle）。
		// 分支位置：version 检查之后、升档之前（Open Question 2 裁决——version 为
		// 公开协议信息先查，核销紧随其后）；核销全部在预认证 4KiB 读上限档内完成
		// （Hello JSON +ticket ~120B，D-11 两档纪律不变）。
		_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrAuthFailed, "authentication failed"))
		logEvent(remote, websocket.StatusPolicyViolation, proto.ErrAuthFailed, remoteUser)
		// 08-04 OPS-07（D-06）：WS 侧 auth_failed 计数与事件同址递增
		// （401/429 事件行既有，metrics 只加计数不打行；无 IP label——
		// per-IP 明细查日志事件）。
		s.mc.authFailed.Add(1)
		_ = c.Close(websocket.StatusPolicyViolation, proto.ErrAuthFailed)
	} else {
		// 升档序列（顺序敏感，PATTERNS 注意 5/6；G-05-1 重排后时序）：停 5s 计时器 →
		// per-IP release（Hello 完成即不计半开，D-04：NAT 场景正常浏览器不受限）→
		// hubMu 内：模式判定矩阵（ticket 绑定 mode × --writable × write-policy ×
		// owner 在位 → 生效 mode + rwEligible + 是否立 owner，clients.go
		// decideModeLocked；ticket 绑定值 = 认证模式 checkTicket 核销返回（D-11），
		// 无认证模式为其 s.writable 派生值）→ 构造 client（Hello 首尺寸登记 dims
		// 字段——递补升格时新 owner 参与集切换的尺寸来源）→ Hello 首尺寸按 D-09
		// 参与集规则入 arbiter 并即时重算（05-04：消除 80x24 首帧窗口——参与集
		// attach 即时重算不防抖，新 owner/新 rw 端立即参与仲裁；ro 旁观者不参与、
		// 不重算）→ sessionDimsLocked 取重算后的会话尺寸（G-05-1：Welcome 恒携
		// attach 完成后生效的会话尺寸——重排前 Welcome 组帧在 addMember/recalcNow
		// 之前，会携带过时的 pre-attach 尺寸）→ Welcome 帧按生效 mode 与 prefs 双档
		// 选档（P4 D-13 空则不出键——旧前端零漂移；05-03 D-13：ro 档永不含 osc52）
		// 携会话尺寸经 outbox 首条入队 → 注册表登记（+ owner 指针登记）→ 16KiB
		// 稳态档（SetReadLimit 经库 atomic store 下一条消息起生效，read.go:97-105）
		// → writer + pinger 启动。
		// 不变量保持论证（G-05-1 重排）：
		//   - Welcome 恒首帧：Welcome 入队仍先于 registerLocked——onChunk 遍历注册表
		//     扇出，cl 未登记前绝无 OUTPUT 夹入（P2 D-02 时序纪律不受影响）；
		//     addMember/recalcNow 不触注册表，提前执行无扇出面（其运行期推送循环
		//     遍历注册表，attach 者自身尚未登记故不触达——其会话尺寸由 Welcome
		//     承载，零重复帧）。
		//   - 锁序保持：recalcNow → sess.Resize 取 sess.fdMu，hubMu > sess.fdMu
		//     同序（resize.go:8-9 纪律）不变。
		// 注册刻意在握手完成后才发生：此前 OUTPUT 一律经 hub 空扇出丢弃——
		// Welcome 恒为 S→C 首帧（writer 全程唯一写端，FIFO 保证首帧无时序竞态，
		// P2 D-02 时序纪律），且未认证客户端在预认证窗口内收不到任何 PTY 输出。
		close(helloDone)
		release()
		s.hubMu.Lock()
		effMode, rwEligible, becomeOwner := s.decideModeLocked(mode)
		cl = &client{
			conn:       c,
			remote:     remote,
			remoteUser: remoteUser, // 07-03：Attach 入口提取一次，此后只读（clients.go 字段注释）
			rwEligible: rwEligible,
			dims:       dims{cols: h.Cols, rows: h.Rows}, // Hello 首尺寸（DecodeHello 已 ClampDim）
			outbox:     newOutbox(s.outboxBytes),
			done:       make(chan struct{}),
			cancel:     cancel,
			// 每客户端输入限速令牌桶（RES-02，05-05）：rate 32KiB/s + burst 64KiB
			// 默认（R-01 参数表——击键 ~10B/s、快粘 ~50KB 瞬时由 burst 容纳）；
			// 超限唯一动作 = 丢弃该帧（R-02，不断开不打逐次日志）。ro 客户端同样
			// 构造（无害——INPUT 先过 mode 门）；字段注释见 clients.go。
			limiter: rate.NewLimiter(rate.Limit(s.inputRate), s.inputBurst),
		}
		cl.mode.Store(effMode) // 生效模式初始值（atomic 承载：INPUT 门无锁读者，见 clients.go）
		// D-09 参与集登记（resize.go participates 矩阵逐字）：rw 端（owner 模式
		// 仅 owner / all 模式全部 rw 端）与无 --writable 纯 ro 会话的全部 ro 端
		// 以 Hello 首尺寸参与仲裁；含可写端会话的 ro 旁观者不参与（其 RESIZE 经
		// D-09 第二闸忽略，尺寸永不影响可写端 PTY 尺寸）。attach 即时重算不防抖
		//（RESEARCH Pattern 4：无风暴风险）；旁观者 attach 不改变参与集，不重算。
		// G-05-1 重排：参与集登记/重算前移至 Welcome 组帧之前——Welcome 恒携
		// attach 完成后生效的会话尺寸（重排前组帧在前，会携带过时的 pre-attach
		// 尺寸）。recalcNow 的运行期推送不触达 attach 者自身（尚未 registerLocked，
		// 推送循环遍历注册表）——其会话尺寸由下方 Welcome 承载，零重复帧。
		if s.participates(effMode) {
			s.addMember(cl, cl.dims)
			s.recalcNow()
		}
		sd := s.sessionDimsLocked() // G-05-1：重算后的会话尺寸（零参与者期间 = spawn 80x24 回落）
		// Welcome 帧作为 outbox 首条入队（组帧函数零改动复用；空队列首帧
		// trySend 恒成功——Welcome ≪ cap），按生效 mode 选 prefs 档（ro 档永不
		// 含 osc52，D-13/P5-6）并携会话尺寸（G-05-1 恒在键）。握手期 Error 帧
		//（version_mismatch/auth_failed）发生在注册前，维持直写不变。入队先于
		// 登记且全程持 hubMu——hub 扇出遍历注册表，若先登记 onChunk 可在 Welcome
		// 前夹入 OUTPUT（首帧时序竞态）。
		prefs := s.clientPrefsRO
		if effMode == proto.ModeRW {
			prefs = s.clientPrefsRW
		}
		cl.outbox.trySend(proto.WelcomeFrame(effMode, prefs, sd.cols, sd.rows))
		s.registry.registerLocked(cl)
		if becomeOwner {
			s.registry.owner = cl // D-06：首个 rw attach 立 owner
		}
		// 宽限取消点（06-02，D-14）：attach 登记成功即取消断开退出宽限计时——
		// 宽限内任一端 attach 成功则退出取消、会话继续；恰好一次（Stop + 置 nil
		// 防重复，实现见 clients.go cancelExitEmptyTimerLocked）。plan 字面
		//「registerLocked 尾部」的调和：registerLocked 是 registry 方法无 Server
		// 视角，取消点落同一 hubMu 持有内的登记之后。
		s.cancelExitEmptyTimerLocked(cl.remote, cl.remoteUser)
		// P5-7 统一挂点：attach 后门重估——新可写端加入信用集（其 creditBlocked
		// 恒 false），可能使「全体可写端均满」不再成立，等待中的信用门必须重估。
		s.hubCond.Broadcast()
		s.hubMu.Unlock()
		// 08-02 D-17/D-20：attach 事件（升档完成、注册表登记后）——client_id =
		// attachSeq（registerLocked 分配，从 1 起单调递增；同一 goroutine 内其写
		// happens-before 本读），携 remote/mode（RESEARCH A6 增强字段）键；
		// 无 code 键（连接事件非关闭事件）；remote_user 非空出键（07-03 同口径）。
		// 同一连接的 detach 事件经 client_id 与本事件关联检索。
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
		// writer 是该连接全程唯一 WS 写端（clients.go）；pinger 保活（D-16）挂
		// 升档序列尾段（PATTERNS 注意 5），与既有单 reader 循环并发装配——库硬性
		// 要求 Ping 必须与 Reader 并发（conn.go:218-220），不得为 ping 再开
		// reader；pong 由读循环 handleControl 自动处理（read.go:317-337）；ping
		// 与 writer 的数据写并发安全、无帧交错（库 writeFrameMu 串行化所有帧，
		// write.go:288-293）。
		go s.writer(cl)
		go s.pinger(ctx, cl, s.pingInterval)
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
			s.logIfMessageTooBig(remote, err, remoteUser)
			// 对端关闭（errors.As 可取出 CloseError）与网络断开同等处理：reader
			// 终结 → detach（注册表移除 + writer/pinger 终结），不进 exitf、不发
			// 任何信号（CONTEXT domain 必然推论：P1 D-11 单次语义终结）。
			if cl != nil {
				s.detach(cl)
			}
			return
		}
		// 08-04 OPS-07（D-04）：WS 上行流量站点二——稳态读循环每消息计入。
		s.mc.wsRecvBytes.Add(int64(len(data)))
		if len(data) == 0 {
			continue // OQ2：Hello 完成后空消息维持静默跳过
		}
		switch data[0] {
		case proto.Input:
			// per-client mode 门（P2 D-13/D-14 的多客户端映射：per-client 判定
			// 替代全局 s.writable——05-03 起 owner 降级/递补升格翻转 per-client
			// mode，mode 经 atomic.Value 承载：本门每击键无锁 Load，与 hubMu 内
			// promoteNextLocked 的升格写并发安全）：ro 静默丢（不打日志防按键
			// 洪水）。cl == nil 为握手违规落入路径（连接已在关闭握手）——同样
			// 静默丢，绝不对未注册连接写 PTY。
			if cl == nil || cl.mode.Load() == proto.ModeRO {
				continue
			}
			// 限速门（RES-02，05-05，R-02 丢弃语义）：超限唯一动作 = 丢弃该帧 +
			// inputDrops 计数——不断开、不打逐次日志、不踢出不降权（Allow godoc
			// 官方 drop 语义 rate.go:113-115；all 模式激进粘贴的合法用户被踢是
			// UX 灾难，INPUT 丢帧用户可感知可恢复——键不回显自然放慢——不同于
			// OUTPUT 丢帧的静默损坏）。计数器 Phase 8 OPS-07 进 metrics（review
			// #10）；用户侧可见性通道 = README 明示（05-09）+ Phase 8 metrics，
			// 无协议反馈帧（P2 D-01 类型空间纪律，review #5 延期处置）。
			if !cl.limiter.AllowN(time.Now(), len(data)-1) {
				s.inputDrops.Add(1)
				continue
			}
			// CR-01 完整背压修复（05-05，RESEARCH Pattern 8）：payload 入会话级
			// inputQ，单 input-writer goroutine 独占 Master.Write——读循环零同步
			// 写（本 case 的 s.sess.Master.Write 直写已删除；Phase 2 的 O_NONBLOCK
			// 最小缓解从未落地且不再需要，master fd 保持默认阻塞模式——阻塞被关进
			// 专属 goroutine，队列有界 + 丢弃即背压）。
			if !s.inputQ.tryEnqueue(data[1:]) {
				continue // 满则丢（droppedInputs 计数已在 tryEnqueue 内递增）；限速器在前，队列满本应罕见
			}
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
		default:
			_ = c.Close(websocket.StatusProtocolError, "unknown_frame") // 1002，协议演化无歧义
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
// 05-06 分享通道适配（OQ1 正交）：无认证模式但 token 通道开启时（s.tickets 非
// nil 而 s.throttle 为 nil）——Hello 未携 ticket 原样放行（s.writable 派生
// mode，前端探测直连链路不变）；Hello 携 ticket 则必须核销成功才放行（token
// 签发的 ro ticket 若过期/重放后落入 writable 派生 mode 等于降权闸门失效——
// 携票即走核销语义与认证模式一致，throttle 面不存在故守卫跳过）。
//
// 红线（SEC-01）：ticket 值禁止作为任何日志参数——本方法与调用方均不打印。
func (s *Server) checkTicket(ip, ticket string) (string, bool) {
	mode := proto.ModeRO
	if s.writable {
		mode = proto.ModeRW
	}
	if s.tickets == nil {
		return mode, true // 无认证模式且无分享通道：核销分支整体跳过
	}
	if ticket == "" && len(s.credentials) == 0 {
		return mode, true // 无认证模式未携 ticket：原样放行（OQ1，直连链路不变）
	}
	now := time.Now()
	if s.throttle != nil && !s.throttle.allow(ip, now) {
		return "", false // 节流命中：不 recordFail（不延长窗口），ticket 不核销
	}
	m, ok := s.tickets.redeem(ticket, now)
	if !ok {
		if s.throttle != nil {
			s.throttle.recordFail(ip, now) // D-08 统一计数器
		}
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
// 08-02（D-21）：签名由 (ctx, c, remote, remoteUser, interval) 收窄为
// (ctx, cl, interval)——pong_timeout 不再单独打事件行，折入 detach 单事件
// reason=pong_timeout（「连接断开」检索单入口）；cl 承载 conn 与 pongTimedOut
// 置位面（remote/remoteUser 随 cl 只读字段天然同值，参数随签名删除）。
func (s *Server) pinger(ctx context.Context, cl *client, interval time.Duration) {
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
		err := cl.conn.Ping(pctx)
		cancel()
		if err != nil {
			// 只有真正的 pong 超时才置位 + CloseNow：Ping 对 pong 等待的 ctx 到期
			// 返回包装后的 DeadlineExceeded（conn.go:251-258）；父 ctx 是 WithCancel
			// 无 deadline，DeadlineExceeded 唯一来源即 pctx 到期。其余错误（连接已被
			// 对端关闭/写失败/Attach 返回 cancel 级联取消）都是正常终结路径，
			// 静默返回即可——误置 pongTimedOut 会使 detach 误报 pong_timeout
			//（D-21 reason 语义失真），连接终结由既有 reader 路径收口，pinger
			// 无需也不应补刀。
			if !errors.Is(err, context.DeadlineExceeded) {
				return
			}
			// pong 超时（pongTimeout 内未收到应答）：置 cl.pongTimedOut 后
			// CloseNow——reader 终结走 detach 收口 emit detach reason=pong_timeout
			//（code 1006 = 客户端观测的本地合成码，CloseNow 无关闭帧，wire 零
			// 改动）。置位取 hubMu 写、detach 同锁读（08-RESEARCH Pattern 4
			// 形态 b——同步边 = hubMu，禁止 plain 字段跨 goroutine 传递，-race
			// 防线；冷路径每 interval 才一次取锁，无热路径影响）。
			s.hubMu.Lock()
			cl.pongTimedOut = true
			s.hubMu.Unlock()
			cl.conn.CloseNow()
			return
		}
	}
}

// logIfMessageTooBig 是 D-12② 超限可见性三腿之二的服务端钩子：库 limitReader
// 流式截断超限后自动把 1009 送上 wire 且 Read 返回包装 ErrMessageTooBig 的错误
// （read.go:521-541）——应用在此补 stderr 单行事件。库的 close reason 是库内
// 字符串 "read limited at N bytes" 不可定制（PATTERNS 注意 7），message_too_big
// 机器串落点在 stderr 而非线上 reason；禁止包装库或包装 conn 数帧（D-09 修订
// 反模式清单）。非超限错误（对端关闭/网络断开）不产生事件。稳态 16KiB 与预认证
// 4KiB 两档共用同一错误标识（SetReadLimit 仅数值不同），两处埋点同一调用形态。
// 07-03：remoteUser variadic 透传（Attach 入口提取值——预认证档即 attach 链路
// 事件，稳态档即该连接会话事件；空串/缺省不出键，与现状逐字节一致）。
func (s *Server) logIfMessageTooBig(remote string, err error, remoteUser ...string) {
	if errors.Is(err, websocket.ErrMessageTooBig) {
		logEvent(remote, websocket.StatusMessageTooBig, "message_too_big", remoteUser...)
	}
}

// signalName 返回信号的约定大写名（"SIGHUP" 形态）供 exitMessage 消费
// （06-01 review 吸收——映射表抽为独立 helper，exitmsg_test.go 白盒逐行锁定）。
// RESEARCH Pitfall 3：Signal.String() 产出小写描述词（"hangup"——GOROOT
// zerrors 表），禁止裸用——显式映射是唯一合法形态（D-09 服务端组文案唯一
// 写口）。未在映射表的信号返回 ("", false)，调用方回退数字形态。
func signalName(sig syscall.Signal) (string, bool) {
	switch sig {
	case syscall.SIGHUP:
		return "SIGHUP", true
	case syscall.SIGINT:
		return "SIGINT", true
	case syscall.SIGQUIT:
		return "SIGQUIT", true
	case syscall.SIGILL:
		return "SIGILL", true
	case syscall.SIGABRT:
		return "SIGABRT", true
	case syscall.SIGKILL:
		return "SIGKILL", true
	case syscall.SIGSEGV:
		return "SIGSEGV", true
	case syscall.SIGPIPE:
		return "SIGPIPE", true
	case syscall.SIGALRM:
		return "SIGALRM", true
	case syscall.SIGTERM:
		return "SIGTERM", true
	case syscall.SIGUSR1:
		return "SIGUSR1", true
	case syscall.SIGUSR2:
		return "SIGUSR2", true
	case syscall.SIGCHLD:
		return "SIGCHLD", true
	}
	return "", false
}

// exitMessage 组装 EXIT 帧 message（D-09 三形态，文案与 06-UI-SPEC §Session
// Ended Contract 文案表逐字一致；服务端组文案唯一写口——前端不自维护信号文案
// 表、textContent 直显）：
//  1. err==nil（正常退出含 exit 0）→ "The process exited with code 0."
//  2. ExitError 且 code>=0 → "The process exited with code N."
//  3. ExitError 且 code<0（信号死亡，ExitCode()=-1——GOROOT exec_posix.go 语义）
//     → signalName 命中组大写名形态 "…killed by signal SIGNAME."；未命中/
//     WaitStatus 断言失败回退数字形态（断言失败为防御性兜底——unix 上
//     ExitError.Sys() 恒为 WaitStatus，真实进程不可达）
//  4. 非 ExitError（Wait 返回其他错误）→ 兜底 terminated 文案
func exitMessage(err error, code int) string {
	if err == nil {
		return "The process exited with code 0."
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return "The process terminated."
	}
	if code >= 0 {
		return fmt.Sprintf("The process exited with code %d.", code)
	}
	// 信号死亡分支：信号号提取单侧定义 = exitSignalNum（08-02 D-22——session_end
	// 事件 signal 字段与本分支同源，纯重构抽取行为逐字节不变）。未命中时
	//（WaitStatus 断言失败/未 Signaled——unix 上真实进程不可达的防御兜底）
	// sig 保持占位值 code（恒 -1）——回退数字形态产出 "signal -1"，语义即
	//「信号号未知」。
	sig := code
	if n, ok := exitSignalNum(err); ok {
		sig = n
	}
	if name, ok := signalName(syscall.Signal(sig)); ok {
		return fmt.Sprintf("The process was killed by signal %s.", name)
	}
	return fmt.Sprintf("The process was killed by signal %d.", sig)
}

// exitSignalNum 从 sess.Wait 返回的错误提取信号号（08-02 D-22 单侧定义——
// exitMessage 信号分支与 session_end 事件 signal 字段共用，抽取前为
// exitMessage 内联逻辑）：err 为 *exec.ExitError 且其 WaitStatus Signaled()
// → (信号号, true)；其余一切形态（nil/非 ExitError/非 WaitStatus/未
// Signaled 正常退出）→ (0, false)。exitmsg_test.go 白盒四形态锁定。
func exitSignalNum(err error) (int, bool) {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return 0, false
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return int(ws.Signal()), true
	}
	return 0, false
}

// lifecycle 是 D-10 唯一终结路径触发源（多客户端形态）：子进程退出 → 带时限
// drain（Pitfall 4）→ 广播 EXIT 帧 + 1000 关闭全部已注册客户端（D-10 序列，
// Phase 6 SESS-03）→ exitf（退出码 = 子进程退出码，退出码传递语义不变）。
func (s *Server) lifecycle() {
	err := s.sess.Wait()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	// 08-02 D-17/D-22：session_end 事件（sess.Wait 返回与退出码提取完成后、
	// EXIT 帧组帧之前）——exit_code 与 EXIT 帧同源（信号死亡 -1）；signal 键
	// 仅信号死亡且 signalName 映射命中出键（RESEARCH A7 裁决：未命中不出键，
	// 键在时类型恒 string）；duration_seconds = PTY spawn 到退出（startedAt
	// 起点，D-22）。信号号提取单侧定义 = exitSignalNum（与 exitMessage 同源）。
	endAttrs := []slog.Attr{
		slog.String("event", "session_end"),
		slog.Int("exit_code", code),
		slog.Float64("duration_seconds", time.Since(s.startedAt).Seconds()),
	}
	if sig, ok := exitSignalNum(err); ok {
		if name, ok := signalName(syscall.Signal(sig)); ok {
			endAttrs = append(endAttrs, slog.String("signal", name))
		}
	}
	emitEvent(endAttrs...)
	// 08-03 D-10：PTY 会话死亡即探活语义翻转——与 session_end 事件同区段
	//（sess.Wait 返回与退出码提取完成后）；/healthz 的 session_active 数据源，
	// 先于 terminate→exitf（waitExit 收码即测试侧同步边）。
	s.sessionAlive.Store(false)
	s.sess.Drain(200 * time.Millisecond)
	// input-writer 收口（05-05，CR-01 修复的生命周期半侧）：Drain→Close 已关闭
	// master fd，在途 Master.Write 经 runtime poller 解除阻塞返回错误（与 Read
	// 同机制，D-12 语义）；close(inputDone) 使 select 等待中的 inputWriter 退出。
	close(s.inputDone)
	// EXIT 帧组帧一次（D-09 载荷：exit_code + 服务端组文案 message）——全客户端
	// 共享只读引用（P5-1 纪律）；ro/rw 全员同帧（终结无权限语义）。
	exitFrame := proto.ExitFrame(code, exitMessage(err, code))
	// 广播 EXIT 帧 + 1000（D-10 序列）：hubMu 下取注册表快照后，每客户端
	// goroutine 内【先同步 Write(EXIT) 再 Close(1000)】——写序论证（RESEARCH
	// Pattern 2 / Pitfall 1）：库帧级写串行化保同 goroutine 先写先发，wire 序恒
	// EXIT 在 1000 前；禁止经 outbox.trySend 异步入队（writer drain 与 Close
	// 关闭帧竞态，关闭帧超车则客户端收 1000 却无退出码）。Write 失败不补救直接
	// Close——进程已退出场景无需保帧（CONTEXT discretion 授权）。Close 自带
	// 5s+5s 上界（close.go:87-89，并行等待自然有界）。广播期间新 attach 或断开
	// 的路径均经 detach/reader 收口，不影响本快照的关闭语义；被关闭客户端的读
	// 循环随 CloseError 终结走 detach——detach 不进 exitf，终结由本路径独占
	//（D-10）。
	s.hubMu.Lock()
	// exiting 门（06-02，D-13 防线；review #5 吸收——置位必须先于注册表快照）：
	// 广播 Close(1000) 引发的 detach 致空属正常终结序列，空触发
	//（clients.go maybeExitWhenEmptyLocked）检查该位抑制，不得再生 SIGHUP/宽限
	// 计时器（事件流可信度与信号竞态防线；无此门则自然退出 exit 42 路径有
	// SIGHUP 翻码竞态）。
	s.exiting = true
	// 08-02 D-21：广播关闭码同点登记（hubMu 保护，detach 同锁读）——lifecycle
	// 子进程退出广播窗口 = 1000；detach reason=shutdown 的 code 数据源。
	s.closeBroadcastCode = int(websocket.StatusNormalClosure)
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
			// 2s Write 超时（RESEARCH OQ3 定值，Phase 9 标定挂账；拒绝可配化——
			// P2 D-10 常量纪律）：stall/慢链路 2s 未写完 ~100B EXIT 帧即放弃
			// 直写，该端退化为 1000 + 前端硬编码回退文案（R2 回退路径既有，
			// 非致命）；2s ≪ Close 内建 5s+5s 上界。
			wctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = c.conn.Write(wctx, websocket.MessageBinary, exitFrame)
			cancel()
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

// Shutdown 是 D-23 1001 优雅下线的触发源（07-05，P6 deferred 兑现；调用方 =
// main 的 SIGTERM/SIGINT NotifyContext goroutine）：exiting 门置位 → 注册表
// 快照 → 每客户端 goroutine 同步 Close(1001 Going Away) → wg.Wait() →
// stop-signal 序列（07-04 字段复用）→ 返回。纪律要点：
//
//   - 无 EXIT 帧前置——进程未退出，终结语义由关闭码承载（EXIT 帧语义 = 子
//     进程退出，与 1001「服务关停、进程将被信号终结」不同源）；Close 内建
//     5s+5s 上界（close.go:87-89）不再盒一层（RESEARCH OQ3 裁决——stall 端
//     最坏 10s 不阻塞进程退出：exitf 由 lifecycle 子进程路径收口，与本
//     goroutine 并发，T-07-05a）。
//   - exiting 门复用 lifecycle 先例（上方 lifecycle 注释）：广播 Close(1001)
//     引发的 detach 致空属正常终结序列，空触发（clients.go
//     maybeExitWhenEmptyLocked）检查该位抑制——不得再生 stop-signal/宽限
//     计时器（06-02 D-13 防线同族）。
//   - Shutdown 不调 exitf（P1 硬约束：触发源非分支）——返回后子进程死亡 →
//     sess.Wait 返回 → lifecycle 的 EXIT+1000 广播在已空注册表上零循环 →
//     terminate(code) 单一收口（exitf + sync.Once，零新 exit 分支）。
//   - 1001×EXIT 竞态论证（RESEARCH Pattern 7，T-07-05d 风险接受登记）：
//     Shutdown 置 exiting 后子进程才死亡，lifecycle 快照在 Shutdown 之后
//     注册表已空，零重复关闭；子进程在 1001 广播完成前自然死亡时两端可能
//     先后收到 1000/1001——coder/websocket Close 幂等（重复 Close 返回错误
//     静默），前端以先到码分派，两语义都正确（进程死 vs 服务关停）。
//   - stop-signal 序列复用 07-04 落地的 s.stopSignal/s.stopTimeout（Options
//     单一通道，双写即漂移）：SignalGroup 负 pid 进程组（setsid pgid==pid
//     既定不变量）+ ESRCH 幂等静默（已死进程组重复发送无害）；Shutdown 不在
//     hubMu 内等待（锁序 hubMu > sess.fdMu 不受影响），与 lifecycle 并发
//     安全——stopTimeout 期间进程若已退出，补发 KILL 到达空 pgid 静默。
func (s *Server) Shutdown() {
	// 08-03 D-11：draining 置位 = Shutdown 入口首行（hubMu 锁定之前，与
	// s.exiting 同源触发点）——1001 广播开始前 /healthz 即翻转为 503
	// draining，关停全程探活器/反代不再向将死实例导新流；hubMu 外 atomic
	// 读故 atomic.Bool（registry.n 先例同构）。无网络可达置位路径（T-08-03d：
	// 只能经 SIGTERM/INT → Shutdown 触发）。
	s.draining.Store(true)
	// 08-02 D-17：shutdown 事件（进程级——无 remote/code 键），hubMu 锁定前
	// emit；draining 置位与本事件同函数不同点。
	emitEvent(slog.String("event", "shutdown"))
	s.hubMu.Lock()
	s.exiting = true
	// 08-02 D-21：1001 优雅下线广播码同点登记（hubMu 保护，detach 同锁读）——
	// 本路径广播窗口内的 detach reason=shutdown code=1001。
	s.closeBroadcastCode = int(websocket.StatusGoingAway)
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
			_ = c.conn.Close(websocket.StatusGoingAway, "server_shutting_down")
		}()
	}
	wg.Wait()
	s.sess.SignalGroup(s.stopSignal)
	if s.stopTimeout > 0 {
		time.Sleep(s.stopTimeout)
		s.sess.SignalGroup(syscall.SIGKILL)
	}
}
