// wesh — share terminal over web。
//
// CLI 形态：wesh [flags] -- <cmd> [args...]（D-02）；`--` 后原样以 exec 数组传递，
// 绝不经 shell。Phase 1 单次语义：WS 断开即退出（D-11），断线重连在 Phase 6。
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// version 由发布构建注入；开发构建为 dev。
var version = "dev"

type config struct {
	port         int
	bind         string
	showVersion  bool
	writable     bool
	pingInterval time.Duration
	// Phase 5 写权限体系（D-05，one-way 公开契约，P2 D-15 同纪律）：
	writePolicy    string // --write-policy=owner|all（默认 owner；仅在 --writable 总闸开启时有意义）
	writePolicySet bool   // --write-policy 是否被显式设置（parseArgs 经 fs.Visit 填充，validateStartup 组合校验消费）
	// Phase 5 容量策略（D-08，one-way 公开契约，P2 D-15 同纪律）：
	maxClients int // --max-clients（默认 32；容量策略是部署关切开 flag，与 P2 D-10 攻击面上限常量不同类）
	// Phase 6 会话生命周期（D-12/D-14，one-way 公开契约，P2 D-15 同纪律）：
	once          bool           // --once（≡ --max-clients=1 --exit-when-empty=0 语法糖；展开见 parseArgs）
	exitEmpty     exitEmptyValue // --exit-when-empty[=duration]（可选值 flag，三形态见类型注释）
	maxClientsSet bool           // --max-clients 是否被显式设置（fs.Visit 填充；--once 展开与 validateStartup 冲突校验消费）
	exitEmptySet  bool           // --exit-when-empty 是否被显式设置（同上）
	// Phase 3 认证与传输安全（one-way 公开契约，P2 D-15 同纪律）：
	credentials  []server.Credential // D-01：--credential 逐组收集 / WESH_CREDENTIAL env 兜底
	tlsCert      string              // D-04：--tls-cert，与 tlsKey 成对才启用 TLS
	tlsKey       string              // D-04：--tls-key
	noAuth       bool                // D-03：--no-auth 逃生门（无凭据监听非 loopback）
	insecureHTTP bool                // D-05：--insecure-http 逃生门（非 loopback 明文携凭据）
	origins      []string            // D-12：--origin 规范化后的允许列表
	// Phase 4 客户端偏好下发（one-way 公开契约，P2 D-15 同纪律）：
	clientOptions []clientOption // P4 D-15：--client-option key=value 逐组收集（白名单 + JSON parse 期校验）
	osc52         bool           // P4 D-12：--osc52 OSC52 剪贴板写开关（默认关，只写不读；安全敏感项仅服务端可开启）
	// Phase 7 部署形态（D-13，one-way 公开契约，P2 D-15 同纪律）：
	basePath string // --base-path 反代子路径挂载前缀（parse 期 normalizeBasePath 严格校验；空串 = 未配置，根 "/" 归一为未配置）
	// Phase 7 监听形态（D-08/D-09，one-way 公开契约，P2 D-15 同纪律）：
	socket         string      // --socket unix socket 监听路径（空串 = TCP 形态现状；与显式 --port/--bind 互斥，validateStartup 消费）
	socketModeStr  string      // --socket-mode 原始八进制串（默认 "0660"；ParseUint 前形态——八进制语义见 flag 定义注释）
	socketMode     os.FileMode // --socket-mode 八进制解析产物（默认 0660；D-09：权限位确定性必须 listen 后显式 Chmod，不得由 umask 漂移决定）
	socketOwner    string      // --socket-owner 原始 user[:group]（仅随 --socket 有意义）
	socketUid      int         // --socket-owner parse 期解析产物 uid（未给 = -1 哨兵——uid/gid 0 是 root 合法值，零值不可作未给标记；listenSocket 据此跳过 Chown）
	socketGid      int         // 同上 gid
	portSet        bool        // --port 是否被显式设置（fs.Visit 填充；D-08 互斥判定锚定显式位而非终值——write-policy/max-clients/exit-empty 三先例同形态）
	bindSet        bool        // --bind 是否被显式设置（同上）
	socketModeSet  bool        // --socket-mode 是否被显式设置（D-09 单给矛盾判定）
	socketOwnerSet bool        // --socket-owner 是否被显式设置（同上）
	// Phase 7 反代信任（D-18，one-way 公开契约，P2 D-15 同纪律）：
	authHeader string // --auth-header 可信反代用户头名（空串 = 不信任——XFF 完全忽略、日志不出 remote_user 键，D-20 单一信任闸；值只做 logEvent 审计归因记录，不做任何认证决定，D-17 正交）
	// Phase 7 子进程管理（D-21/D-22/D-24，one-way 公开契约，P2 D-15 同纪律）：
	cwd  string // --cwd 子进程工作目录（空串 = 继承服务端 cwd 现状；非空时 validateStartup stat 预检 fail-fast，spawn 前零资源占用）
	term string // --term 子进程 TERM（空串 = xterm-256color 现状语义；--term="" 空串值按未配置处理）
	uid  int    // --uid 降权目标 uid（默认 -1 = 不降权；与 --gid 成对强制，validateStartup 消费；flag 注册与校验见 07-04 Task 3）
	gid  int    // --gid 降权目标 gid（同上）
	// D-22 stop-signal 序列（one-way 公开契约，P2 D-15 同纪律）：
	stopSignal    string         // --stop-signal 枚举名 HUP|TERM|INT|KILL（默认 HUP 现状语义；parse 期经 pty.StopSignalByName 枚举校验）
	stopTimeout   time.Duration  // --stop-timeout stop-signal 后补发 SIGKILL 的宽限（默认 0 = 不补 KILL 纯单信号现状；负值 parse 期拒绝）
	stopSignalSig syscall.Signal // --stop-signal 的 parse 期名→信号解析产物（StopSignalByName 命中；Options.StopSignal 接线源）
	// Phase 7 自动打开浏览器（D-26，one-way 公开契约，P2 D-15 同纪律）：
	open bool // --open 启动后以系统启动器打开分享链接（--writable 开 rw 链接否则 ro 链接，含 token 免交互；headless 跳过不阻断，--socket×--open 组合矛盾归 validateStartup）
	// Phase 9 自定义首页（09-04 OPS-03，D-07/D-08 one-way 公开契约，P2 D-15 同纪律）：
	index        string // D-07：--index 自定义首页路径（空串 = 未配置内建页现状；启动一次读入内存，运行期零磁盘依赖，改文件需重启生效）
	indexMaxSize int    // D-08：自定义首页读入上限（字节，默认 16MiB；TOML 纯配置键 index-max-size 可调——无 CLI flag，P7 D-03 纪律的明示例外）
	// Phase 10 会话模式（10-01 PC-01，one-way 公开契约，P2 D-15 同纪律）：
	sessionMode    string // --session-mode=shared|per-client（默认 shared——REQUIREMENTS 反特性 A5；per-client 装配中，当前版本与 shared 等价，10-CONTEXT D-05）
	sessionModeSet bool   // --session-mode 是否被显式设置（parseArgs 经 fs.Visit 填充；D-02 双源机制采集备用，write-policy×per-client warn 锚定归 10-02 消费）
	argv0          string // argv[0] 落定副本（validateStartup per-client LookPath 预检数据源，10-02 消费）
}

// clientOption 是 --client-option 的 parse 期产物：key 已过白名单（P4 D-14），
// value 已是合法 JSON（P4 D-15 parse 期 fail-fast）。
type clientOption struct {
	key   string
	value json.RawMessage
}

// exitEmptyValue 是 --exit-when-empty[=duration] 的 flag.Value 实现（D-14 三形态：
// 不写 = 不开启；裸写 = 最后一个客户端断开立即退出（grace 0）；=duration = 重连
// 宽限）。可选值惯例（GOROOT flag.go:350-356：实现该布尔方法的 Value 使命令行
// 解析器把 -name 等价于 -name=true，而非消费下一命令行参数）——空格分隔形态
// `--exit-when-empty 30s` 结构性不传值（30s 落入 argv），值只能经 = 号形态传入。
type exitEmptyValue struct {
	set   bool
	grace time.Duration
}

// String 实现 flag.Value 契约（PrintDefaults 取注册时零值串 ""，不显示 default
// 标注——不写即不开启的零配置形态）。
func (v *exitEmptyValue) String() string {
	if !v.set {
		return ""
	}
	return v.grace.String()
}

func (v *exitEmptyValue) IsBoolFlag() bool { return true } // GOROOT flag.go:350-356 逐字："If a Value has an IsBoolFlag() bool method returning true, the command-line parser makes -name equivalent to -name=true rather than using the next command-line argument."——裸写 ≡ =true 不消费下一参数

// Set 三形态：裸写（可选值惯例下送来 "true"）→ set + grace 0（立即退出）；
// =duration → time.ParseDuration 解析；err!=nil 或 d<0 报错——d<0 检查是负值
// 拒绝的唯一闸（time.ParseDuration("-5s") 解析成功，负 duration 是合法语法）。
// 错误上报纪律：duration 值非敏感（T-06-04a accept），直接 return error——
// flag 包会包装为 `invalid value %q for flag -exit-when-empty` 回显值，可接受，
// 非 SEC-01 面（credErr/clientOptErr 记录式仅用于值含敏感内容的 flag，既定注释）。
func (v *exitEmptyValue) Set(s string) error {
	v.set = true
	if s == "true" { // 裸写 = 最后一个客户端断开立即退出
		v.grace = 0
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return fmt.Errorf("invalid --exit-when-empty duration %q: must be a non-negative duration (e.g. 30s)", s)
	}
	v.grace = d
	return nil
}

// parseArgs 解析 flags。全名无短选项（P2 D-15），共 32 个：
// Phase 1/2：--port/--bind/--version/--writable（D-15）/--ping-interval（D-16）；
// Phase 3：--credential（D-01 可重复）、--tls-cert/--tls-key（D-04 成对）、
// --no-auth（D-03 逃生门）、--insecure-http（D-05 逃生门）、--origin（D-12 可重复）；
// Phase 4：--client-option（P4 D-15 可重复，白名单 + JSON parse 期校验）、
// --osc52（P4 D-12 OSC52 剪贴板写开关，默认关）；
// Phase 5：--write-policy（D-05，owner|all 默认 owner，parse 期枚举校验）、
// --max-clients（D-08，默认 32，≤0 经 validateStartup 拒绝）；
// Phase 6：--once（D-12 语法糖 ≡ --max-clients=1 --exit-when-empty=0）、
// --exit-when-empty[=duration]（D-14 可选值，裸写 = 立即退出，默认不开启）；
// Phase 7：--base-path（07-01 D-13 反代子路径，parse 期严格校验）、
// --socket/--socket-mode/--socket-owner（D-08/D-09 unix socket 监听与权限属主，
// 互斥/单给组合矛盾归 validateStartup fail-fast）、
// --auth-header（07-03 D-18 反代用户头名；裸信任 + D-16 暴露面警告归
// validateStartup，XFF 同闸采信 D-20）、
// --cwd/--term（07-04 D-21 子进程工作目录与 TERM，stat 预检归 validateStartup）、
// --stop-signal/--stop-timeout（07-04 D-22 停止信号进程组序列，枚举/负值 parse
// 期校验）、--uid/--gid（07-04 D-24 数字直通降权，值域 parse 期校验，成对强制
// 归 validateStartup）、--open（07-05 D-26 启动后自动开浏览器，headless 跳过
// 不阻断，--socket×--open 组合矛盾归 validateStartup）、
// --config（07-06 OPS-09 D-01 显式指定 TOML 配置文件——仅预扫显式路径，
// 零隐式默认路径搜索，裸启动行为零漂移）；
// Phase 9：--index（09-04 D-07 自定义首页整页替换，ttyd -i 同款——启动一次
// 读入；stat 级预检与读入校验归 validateStartup/loadCustomIndex，index-max-size
// 纯配置键无 flag——D-08）；
// Phase 10：--session-mode（10-01 PC-01，shared|per-client 默认 shared
// ——REQUIREMENTS 反特性 A5；parse 期枚举校验 D-04 文案；per-client 装配中，
// 当前版本行为与 shared 等价——10-CONTEXT D-05 注记）。
// 配置文件两阶段合并（07-06 OPS-09，D-01..D-07，07-RESEARCH Pattern 4）：
// prescanConfigPath 预扫 --config 路径 → loadFileConfig 严格加载铺底（文件级
// 错误 exit 2 现状通道，D-06；D-07 权限警告加载期 stderr 打印）→ fileConfig
// 标量键换算为 flag 注册默认值（CLI 未给自然落配置值、CLI 给则覆盖；内置
// 默认仅在配置键缺席时出现——flag > 配置 > 默认两档由默认值替换机制天然
// 成立）→ fs.Parse → fs.Visit 显式位 → 配置键存在即「已给定」补置
// portSet/bindSet/socketModeSet/socketOwnerSet/writePolicySet/sessionModeSet
// （socket 族
// 互斥/单给与 write-policy×writable 矩阵对配置来源值同档生效；sessionModeSet
// 为 D-02 双源采集位，本阶段备用、消费归 10-02）→ 配置
// exit-when-empty（exitEmptyValue.Set 单一解析路径，OQ4；在 --once 展开之前
// 应用——配置不算显式，展开覆盖配置值）→ 列表合并（credential/origin/
// client-option：CLI 给出则替换整个列表，D-02；CLI 未给则 env 夹层先行、
// 配置列表逐项经各自 parse 期校验函数应用——flag > env > 配置，D-05）→
// argv：fs.Args() 非空为 CLI argv，空且 command 键非空为配置 command（D-04）。
// WESH_CREDENTIAL env 兜底单组凭据（D-01：flag 非空时 env 整体忽略，flag 优先）。
// `--` 后参数原样收集为 argv（D-02）；argv 为空（且非 --version/--help）
// 返回错误（D-03：无命令不起登录 shell）。
func parseArgs(args []string) (cfg config, argv []string, err error) {
	// D-09：socket owner 未给哨兵——uid/gid 0 是 root 合法值，零值不可作
	// 未给标记；listenSocket 以 uid<0 判定跳过 Chown。
	cfg.socketUid, cfg.socketGid = -1, -1
	// D-24：降权未给哨兵同形态——uid/gid 0 是 root 合法值，-1 = 不降权
	//（pty.StartOptions Uid/Gid 消费；成对强制校验见 validateStartup）。
	cfg.uid, cfg.gid = -1, -1
	// D-01：--config 预扫（两阶段合并第一环——flag 注册前需路径加载 TOML
	// 铺底；仅显式指定零隐式默认路径搜索，未给时 fc=nil 全部行为与无
	// 配置文件时逐字节一致，裸启动零漂移）。
	configPath := prescanConfigPath(args)
	var fc *fileConfig
	if configPath != "" {
		var warn string
		fc, warn, err = loadFileConfig(configPath)
		if err != nil {
			// D-06 fail-fast：文件不存在/解析失败/未知键经现状错误通道
			// exit 2（错误文案只含类别+键名+行号，config.go 值剥离红线）。
			return cfg, nil, err
		}
		if warn != "" {
			// D-07 加载期警告（parseArgs 打印为最小签名扰动——fs.Usage
			// 打印先例同文件）；警告串不含凭据值。
			fmt.Fprintln(os.Stderr, warn)
		}
	}
	// 注册默认值铺底（D-05 优先级链 flag > 配置 > 内置默认的承载机制）：
	// fileConfig 标量键换算为 flag 注册默认值——CLI 未给自然落配置值，
	// CLI 给则覆盖（flag 包语义）；内置默认仅在配置键缺席时出现。指针
	// 标量区分「键缺席」与「显式零值」（fc.X != nil 即采用 *fc.X——
	// port = 0 等显式零值不被吞成内置默认）。duration 串配置键此处解析，
	// 解析失败经 configErr 类别报错（键名不含值；duration 非敏感本可含值
	// ——exitEmptyValue.Set 既定纪律——本实现取不含值的更严形态）。
	portDefault := 7681
	bindDefault := "0.0.0.0"
	writableDefault := false
	pingIntervalDefault := 5 * time.Second
	writePolicyDefault := server.WritePolicyOwner
	sessionModeDefault := server.SessionModeShared
	maxClientsDefault := 32
	onceDefault := false
	osc52Default := false
	tlsCertDefault := ""
	tlsKeyDefault := ""
	socketDefault := ""
	socketModeDefault := "0660"
	socketOwnerDefault := ""
	basePathDefault := ""
	authHeaderDefault := ""
	cwdDefault := ""
	termDefault := ""
	stopSignalDefault := "HUP"
	stopTimeoutDefault := time.Duration(0)
	uidDefault := -1
	gidDefault := -1
	openDefault := false
	// 09-04 D-07/D-08：--index 默认空串（未配置内建页现状）；index-max-size
	// 默认 16MiB（纯配置键默认值，直接赋值不经 flag 注册——D-08）。
	indexDefault := ""
	indexMaxSizeDefault := 16 * 1024 * 1024
	if fc != nil {
		if fc.Port != nil {
			portDefault = *fc.Port
		}
		if fc.Bind != nil {
			bindDefault = *fc.Bind
		}
		if fc.Writable != nil {
			writableDefault = *fc.Writable
		}
		if fc.PingInterval != nil {
			d, perr := time.ParseDuration(*fc.PingInterval)
			if perr != nil {
				return cfg, nil, configErr(configPath, "invalid duration", `key "ping-interval"`)
			}
			pingIntervalDefault = d
		}
		if fc.WritePolicy != nil {
			writePolicyDefault = *fc.WritePolicy
		}
		if fc.SessionMode != nil {
			sessionModeDefault = *fc.SessionMode
		}
		if fc.MaxClients != nil {
			maxClientsDefault = *fc.MaxClients
		}
		if fc.Once != nil {
			onceDefault = *fc.Once
		}
		if fc.Osc52 != nil {
			osc52Default = *fc.Osc52
		}
		if fc.TLSCert != nil {
			tlsCertDefault = *fc.TLSCert
		}
		if fc.TLSKey != nil {
			tlsKeyDefault = *fc.TLSKey
		}
		if fc.Socket != nil {
			socketDefault = *fc.Socket
		}
		if fc.SocketMode != nil {
			socketModeDefault = *fc.SocketMode
		}
		if fc.SocketOwner != nil {
			socketOwnerDefault = *fc.SocketOwner
		}
		if fc.BasePath != nil {
			basePathDefault = *fc.BasePath
		}
		if fc.AuthHeader != nil {
			authHeaderDefault = *fc.AuthHeader
		}
		if fc.Cwd != nil {
			cwdDefault = *fc.Cwd
		}
		if fc.Term != nil {
			termDefault = *fc.Term
		}
		if fc.StopSignal != nil {
			stopSignalDefault = *fc.StopSignal
		}
		if fc.StopTimeout != nil {
			d, perr := time.ParseDuration(*fc.StopTimeout)
			if perr != nil {
				return cfg, nil, configErr(configPath, "invalid duration", `key "stop-timeout"`)
			}
			stopTimeoutDefault = d
		}
		if fc.Uid != nil {
			uidDefault = *fc.Uid
		}
		if fc.Gid != nil {
			gidDefault = *fc.Gid
		}
		if fc.Open != nil {
			openDefault = *fc.Open
		}
		// 09-04：index 键换算 flag 注册默认值（配置铺底、CLI 覆盖）；index-max-size
		// 纯配置键直接落默认值变量（无 flag 注册——D-08 明示例外形态）。
		if fc.Index != nil {
			indexDefault = *fc.Index
		}
		if fc.IndexMaxSize != nil {
			indexMaxSizeDefault = *fc.IndexMaxSize
		}
	}
	fs := flag.NewFlagSet("wesh", flag.ContinueOnError)
	fs.IntVar(&cfg.port, "port", portDefault, "listen port (0 = random, actual port is printed)")
	fs.StringVar(&cfg.bind, "bind", bindDefault, "listen address")
	fs.BoolVar(&cfg.showVersion, "version", false, "print version and exit")
	fs.BoolVar(&cfg.writable, "writable", writableDefault, "allow client input (default read-only)")
	fs.DurationVar(&cfg.pingInterval, "ping-interval", pingIntervalDefault, "WS ping interval (0 = disable)")
	// D-05：写权限策略（one-way 公开契约）。--writable 保持总闸（不给 = 全员只读，
	// 现状语义零漂移）；write-policy 仅在总闸开启时有意义（组合校验见
	// validateStartup）。parse 期枚举校验在 Parse 返回处（值非敏感，直接 return
	// error 即可——client-option 的记录式上报仅用于值含敏感内容的场景）。
	fs.StringVar(&cfg.writePolicy, "write-policy", writePolicyDefault, "write policy when --writable is on: owner|all (default owner)")
	// 10-01 PC-01：会话模式（one-way 公开契约，P2 D-15 同纪律）——shared 为
	// 内置默认（REQUIREMENTS 反特性 A5：默认永不翻转）；per-client 装配中，
	// 当前版本行为与 shared 等价（10-CONTEXT D-05 注记随 help 文案同 PR——
	// 防用户开了发现无新行为误以为 bug）。parse 期枚举校验在 Parse 返回处
	//（write-policy 同位先例——值非敏感，直接 return error 即可）。
	fs.StringVar(&cfg.sessionMode, "session-mode", sessionModeDefault, "session mode: shared|per-client (default shared; per-client is being assembled and currently behaves as shared)")
	// D-08：最大并发客户端数（one-way 公开契约——容量策略是部署关切开 flag，
	// 与 P2 D-10 攻击面上限常量不同类）。默认 32（ARCHITECTURE §6『10–100 连接
	// =团队围观/教学』区间下沿；账面内存与 goroutine 开销微小），Phase 9 负载
	// 标定回填。满员行为：/ws Accept 前 HTTP 503（守卫区③位）+ /api/attach
	// 503 早闸（OQ2）；≤0 经 validateStartup 拒绝（exit 2 配置校验矩阵形态）。
	fs.IntVar(&cfg.maxClients, "max-clients", maxClientsDefault, "maximum simultaneous attached clients")
	// D-01：可重复凭据 flag，fs.Func 回调内 parse 期校验（畸形值即时报错——
	// systemd 配置错误零窗口暴露）。Pitfall 8：help 必须提示 ps 可见性。
	// 红线（SEC-01 启动面延伸，WR-01）：校验错误记入 credErr 而非直接
	// return——flag 包会把回调返回的错误包装为 `invalid value %q for flag
	// -credential: …` 并打印到 stderr，%q 处正是原始 flag 值全文（空 user
	// 形态的手误会把密码分量完整写进 stderr，systemd 部署下落 journald
	// 持久化）；记录后于 Parse 返回处统一上报，两通道（flag 自打印与返回
	// 错误串）均不含值。client-option 同款记录式先例见下方 clientOptErr 注释。
	var credErr error
	fs.Func("credential", "basic auth credential user:pass (repeatable; value visible to local users via ps, prefer WESH_CREDENTIAL env in production)", func(s string) error {
		c, err := server.ParseCredential(s)
		if err != nil {
			credErr = errors.New("invalid --credential: credential must be user:pass") // 只含错误类别，禁含值（SEC-01）
			return nil
		}
		cfg.credentials = append(cfg.credentials, c)
		return nil
	})
	fs.StringVar(&cfg.tlsCert, "tls-cert", tlsCertDefault, "TLS certificate file (must give both --tls-cert and --tls-key to enable TLS)")
	fs.StringVar(&cfg.tlsKey, "tls-key", tlsKeyDefault, "TLS private key file (must give both --tls-cert and --tls-key)")
	fs.BoolVar(&cfg.noAuth, "no-auth", false, "allow listening on non-loopback address without credentials (explicit escape hatch)")
	fs.BoolVar(&cfg.insecureHTTP, "insecure-http", false, "allow serving credentials over plaintext HTTP on non-loopback address (explicit escape hatch; typical behind a TLS-terminating reverse proxy)")
	// D-12：可重复 origin flag，回调内经 NormalizeOrigin parse 期规范化
	// （小写 host + 剥默认端口），规范化错误即时报错。
	fs.Func("origin", "allowed origin scheme://host[:port] (repeatable)", func(s string) error {
		n, err := server.NormalizeOrigin(s)
		if err != nil {
			return err
		}
		cfg.origins = append(cfg.origins, n)
		return nil
	})
	// P4 D-15：可重复客户端偏好 flag，回调内 parse 期校验（白名单 D-14 + 值须合法
	// JSON）——配置错误零窗口暴露，parseArgs 返回错误即 exit 2。
	// 红线（SEC-01 启动面延伸）：错误文案只含 key 名与错误类别，禁含值内容。
	// 为此校验错误记入 clientOptErr 而非直接 return——flag 包会把回调返回的
	// 错误包装为 `invalid value %q for flag -client-option: …` 并打印到 stderr，
	// %q 处正是原始 key=value 串（值内容随之泄露）；记录后于 Parse 返回处统一上报，
	// 两通道（flag 自打印与返回错误串）均不含值。
	var clientOptErr error
	fs.Func("client-option", "client preference key=value (repeatable; whitelisted keys, value is JSON)", func(s string) error {
		key, value, found := strings.Cut(s, "=")
		if !found {
			clientOptErr = fmt.Errorf("invalid --client-option %q: must be key=value", s)
			return nil
		}
		if !proto.ValidClientOptionKey(key) {
			clientOptErr = fmt.Errorf("invalid --client-option key %q", key)
			return nil
		}
		var v json.RawMessage
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			clientOptErr = fmt.Errorf("invalid --client-option value for %q: not valid JSON", key)
			return nil
		}
		cfg.clientOptions = append(cfg.clientOptions, clientOption{key: key, value: v})
		return nil
	})
	// P4 D-12：OSC52 剪贴板写开关（write-only，默认关）——安全敏感项只能经本
	// flag 由服务端开启，结构性排除出 --client-option 白名单与 URL query。
	fs.BoolVar(&cfg.osc52, "osc52", osc52Default, "enable OSC52 clipboard write (write-only; default off)")
	// D-12：--once 语法糖（one-way 公开契约）≡ --max-clients=1
	// --exit-when-empty=0——help 文案单行标明等价关系；展开见下方 fs.Visit 之后。
	// 第二客户端拒绝走既有 503 计数路径（D-12：409 单客户端门不复活）。
	fs.BoolVar(&cfg.once, "once", onceDefault, "accept only one client and exit when it disconnects (equivalent to --max-clients=1 --exit-when-empty=0)")
	// D-14：--exit-when-empty[=duration] 可选值 flag（one-way 公开契约）——
	// 三形态：不写 = 不开启（现状保持：无客户端时子进程继续运行）；裸写 =
	// 立即退出；=duration = 重连宽限。空格分隔形态不传值（可选值惯例，见类型
	// 注释），help 用法行明示 = 号形态。
	fs.Var(&cfg.exitEmpty, "exit-when-empty", "exit after all clients disconnect (optional grace: --exit-when-empty=30s; bare = exit immediately)")
	// D-13：--base-path 反代子路径挂载（one-way 公开契约）——严格模式校验在
	// Parse 返回处经 normalizeBasePath（NormalizeOrigin 先例形态）：合法值原样、
	// 根 "/" 归一为未配置、非法值 exit 2；绝不宽容自动修正（输入与生效值分叉
	// 是配置漂移隐蔽源）。
	fs.StringVar(&cfg.basePath, "base-path", basePathDefault, "serve under a sub-path (e.g. /wesh; must start with /, no trailing slash)")
	// D-08：--socket unix socket 监听（one-way 公开契约）——与显式 --port/--bind
	// 互斥（组合矛盾归 validateStartup fail-fast，分层纪律：parse = 形状，
	// validate = 组合矛盾）；空串 = TCP 形态现状。
	fs.StringVar(&cfg.socket, "socket", socketDefault, "listen on a unix socket at the given path (mutually exclusive with --port/--bind)")
	// D-09：--socket-mode 八进制权限位（one-way 公开契约）——socket 文件 mode
	// 由内核定为 0777&~umask（Go 不做任何 chmod，07-RESEARCH Pattern 1 GOROOT
	// 实证），0660 确定性只能 listen 后显式 Chmod 达成（T-07-02b：文件系统
	// 权限即认证边界，权限不得由 umask 漂移决定）；八进制串形态在 Parse 返回处
	// ParseUint 解析（非法值 parse 期 exit 2）。仅随 --socket 有意义
	//（单给 = 配置矛盾，validateStartup 消费 socketModeSet 显式位）。
	fs.StringVar(&cfg.socketModeStr, "socket-mode", socketModeDefault, "unix socket permission bits in octal (default 0660; only with --socket)")
	// D-09：--socket-owner user[:group]（one-way 公开契约）——parse 期经
	// os/user.Lookup[/LookupGroup] 解析为 uid/gid 数字对（未知用户/组 parse 期
	// exit 2）；仅随 --socket 有意义（单给 = 配置矛盾，同上）。
	fs.StringVar(&cfg.socketOwner, "socket-owner", socketOwnerDefault, "unix socket owner user[:group] (only with --socket)")
	// D-18：--auth-header 可配反代用户头名（one-way 公开契约）——配置即信任
	// 该头（裸信任，ttyd -H 同款；D-16 暴露面启动警告见 validateStartup）。
	// 值经 sanitize 后只做 logEvent remote_user 审计归因记录，不做任何认证
	// 决定（D-17 正交——伪造头不能绕过 Basic/ticket/share token 任一检查）；
	// X-Forwarded-For 同闸采信换 per-IP 键（D-20 单一信任闸，未配置时完全
	// 忽略）。parse 期校验（07-review CR-03）：凭据载体头名拒绝——配置即信任
	// 该头值逐事件进 logEvent，指向 Authorization/Cookie 等会把凭据（含
	// base64）写进 stderr 落 journald，直接击穿 D-03 红线（见 Parse 返回处
	// 校验段；头名合法性其余面由 HTTP 层 Header.Get 语义自然承载）。
	fs.StringVar(&cfg.authHeader, "auth-header", authHeaderDefault, "trusted reverse-proxy user header name (e.g. X-Remote-User); logged as remote_user, no auth effect; credential-carrying headers rejected")
	// D-21：--cwd/--term 子进程工作目录与 TERM（one-way 公开契约）——落
	// pty.StartOptions 的 Dir/Term（spawn.go 注释预留位 07-04 兑现）；空串 =
	// 继承服务端 cwd / xterm-256color 现状语义。--cwd 非空时 validateStartup
	// stat 预检 fail-fast（spawn 前零资源占用——spawn 后才发现 ENOENT 则资源
	// 已占用且错误面到客户端，RESEARCH Anti-Patterns）；--term="" 空串值按
	// 未配置处理（显式空 TERM 会使终端能力丢失）。
	fs.StringVar(&cfg.cwd, "cwd", cwdDefault, "working directory for the child process (default: inherit)")
	fs.StringVar(&cfg.term, "term", termDefault, "TERM for the child process (default: xterm-256color)")
	// D-22：--stop-signal/--stop-timeout（one-way 公开契约）——exit-when-empty
	// 收口路径向子进程进程组（负 pid，setsid pgid==pid 既定不变量）所发信号
	// 与 KILL 补发宽限；默认 HUP + 0 = 现状语义（纯单信号不补 KILL，06-02 D-13
	// 零漂移）。枚举校验与负值检查在 Parse 返回处（write-policy 同位先例；
	// --stop-timeout 取 DurationVar 直收形态——负 duration 是合法语法，负值
	// 检查是唯一闸，exitEmptyValue.Set 负值闸同纪律）。
	fs.StringVar(&cfg.stopSignal, "stop-signal", stopSignalDefault, "signal sent to the child process group on shutdown: HUP|TERM|INT|KILL (default HUP)")
	fs.DurationVar(&cfg.stopTimeout, "stop-timeout", stopTimeoutDefault, "grace before SIGKILL after stop-signal (0 = no escalation)")
	// D-24：--uid/--gid 数字直通降权（one-way 公开契约）——落 pty.StartOptions
	// Uid/Gid → SysProcAttr.Credential（fork 后 exec 前生效，spawn.go 注释登记
	// GOROOT forkExec 顺序）；数字直通不做名字解析（极简容器无 /etc/passwd 的
	// NSS 解析差异规避——名字解析场景运维先 id -u/id -g 查好）。成对强制
	//（只给一个 = 配置矛盾零窗口暴露，validateStartup——降权半配置静默放行 =
	// 子进程以原权运行，T-07-04b；exit 2 而非降级运行）；值域（-1 哨兵之外
	// < -1 或 > 4294967295）parse 期拒绝（uint32 转换安全）。
	fs.IntVar(&cfg.uid, "uid", uidDefault, "numeric uid to drop privileges to (must give both --uid and --gid; resolve names with id -u first)")
	fs.IntVar(&cfg.gid, "gid", gidDefault, "numeric gid to drop privileges to (must give both --uid and --gid; resolve names with id -g first)")
	// D-26：--open 自动开浏览器（one-way 公开契约）——operator 视角入口：
	// --writable 时开 rw 分享链接，否则开 ro 链接（含 token 免交互即打即用，
	// token 通道绕过 Basic 是 P5 D-01 既定语义；与启动打印消费同一拼串
	// shareURLRO/shareURLRW 单一事实源）。平台机制 D-27：xdg-open（Linux）/
	// open（macOS）；headless 检测（无 DISPLAY 且无 WAYLAND_DISPLAY）时 stderr
	// 提示后跳过不阻断启动（headless 服务器是常态部署形态——--open 本质是
	// 桌面便利功能）；Windows 不做（PROJECT Out of Scope）。--socket×--open
	// 组合矛盾归 validateStartup（unix socket 无 http URL 可开，OQ1）。
	fs.BoolVar(&cfg.open, "open", openDefault, "open the share link in a browser after startup (rw link when --writable, otherwise ro)")
	// D-07：--index 自定义首页（09-04 OPS-03，one-way 公开契约——ttyd -i 同款
	// 整页替换语义）：用户 HTML 完全替代内建终端页，/ 与 /s/{token}/ 全通道
	// 统一伺服（D-06）；终端功能由用户页自行实现（WS 端点照旧暴露，wesh 零
	// 注入零模板 D-05）。启动一次读入内存——运行期零磁盘依赖，改文件需重启
	// 生效（与 embed 静态伺服同语义）。校验分层：stat 级预检（不存在/非常规）
	// 归 validateStartup，读入与不可读/超限拒绝归 loadCustomIndex（run() 启动序，
	// exit 2 配置矩阵同语义）。
	fs.StringVar(&cfg.index, "index", indexDefault, "custom index HTML file served in place of the built-in page (full-page replacement; share links serve it too)")
	// D-08：index-max-size 纯配置键（TOML 整数字节，无对应 CLI flag——P2 D-15
	// flag 面紧缩与「设上限就必须可配置」用户裁决的调和形态，P7 D-03 纪律的
	// 明示例外，README 写明防蔓延）——默认值直接赋值，不经 flag 注册。
	cfg.indexMaxSize = indexMaxSizeDefault
	// D-01：--config TOML 配置文件（07-06 OPS-09，one-way 公开契约）——仅显式
	// 指定路径，零隐式默认路径搜索（裸启动行为零漂移）；函数首部
	// prescanConfigPath 已消费其值加载铺底，此处正式注册保持 help 可见与
	// flag 解析一致性（预扫与正式 Parse 双通道同值；值不再二次消费）。
	var configFileFlag string
	fs.StringVar(&configFileFlag, "config", "", "load TOML config file (CLI flags override config values)")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: wesh [flags] -- <cmd> [args...]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return cfg, nil, err
	}
	// D-05：fs.Visit 判定 --write-policy 是否被显式设置（Visit 只遍历已设置
	// flag）——validateStartup 组合校验消费：显式设置却未开 --writable 总闸
	// 属配置矛盾 fail-fast；默认 owner 未显式设置 + 无 --writable 是纯 ro
	// 会话正常形态不拒。
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "write-policy" {
			cfg.writePolicySet = true
		}
		// 10-01 PC-01：--session-mode 显式设置位（D-02 双源机制——本阶段采集
		// 备用，write-policy×per-client warn 锚定消费归 10-02）。
		if f.Name == "session-mode" {
			cfg.sessionModeSet = true
		}
		// D-12/D-14：--once 展开与 validateStartup 冲突校验消费的显式设置判定
		//（write-policy 同款形态——Visit 只遍历已设置 flag）。
		if f.Name == "max-clients" {
			cfg.maxClientsSet = true
		}
		if f.Name == "exit-when-empty" {
			cfg.exitEmptySet = true
		}
		// D-08/D-09：--socket 互斥与单给矛盾判定锚定显式设置位而非终值
		//（三先例同款形态——--socket 与默认 port/bind 同给不误判冲突）。
		if f.Name == "port" {
			cfg.portSet = true
		}
		if f.Name == "bind" {
			cfg.bindSet = true
		}
		if f.Name == "socket-mode" {
			cfg.socketModeSet = true
		}
		if f.Name == "socket-owner" {
			cfg.socketOwnerSet = true
		}
	})
	// D-08/D-09 + write-policy 配置来源显式位（07-06 合并收尾第一档）：配置键
	// 存在即「已给定」——fc.Port/fc.Bind/fc.SocketMode/fc.SocketOwner/
	// fc.WritePolicy/fc.SessionMode 非 nil 即置对应显式位，07-02 落地的
	// 互斥/单给校验矩阵与
	// write-policy×writable 组合校验对配置驱动与 CLI 驱动同档生效（不置位
	// 则配置同时写 socket+port 或单写 socket-mode 会静默绕过 D-08/D-09
	// fail-fast；write-policy 扩展同款模式；session-mode 置位为 D-02 双源
	// 采集位，消费归 10-02）。
	if fc != nil {
		if fc.Port != nil {
			cfg.portSet = true
		}
		if fc.Bind != nil {
			cfg.bindSet = true
		}
		if fc.SocketMode != nil {
			cfg.socketModeSet = true
		}
		if fc.SocketOwner != nil {
			cfg.socketOwnerSet = true
		}
		if fc.WritePolicy != nil {
			cfg.writePolicySet = true
		}
		if fc.SessionMode != nil {
			cfg.sessionModeSet = true
		}
	}
	// 配置内部矛盾检测（07-review WR-02，D-06 严格模式哲学）：fc.Once 为真时
	// 同文件内 max-clients 显式值 ≠1、或 exit-when-empty 解析 grace ≠0 即
	// configErr 拒绝——CLI 同组合（--once × 显式矛盾值）经 validateStartup
	// exit 2 拒绝，同一配置文件内的自相矛盾不得逃过 fail-fast 被 --once 展开
	// 静默改写。只锚定 fc 字段（文件内矛盾）：CLI --once × 配置值的既定覆盖
	// 语义（flag > 配置——配置 max-clients/exit-when-empty 不算显式，展开覆盖）
	// 不受影响（fc.Once 未给时本块不触发）。exit-when-empty 经
	// exitEmptyValue.Set 单一解析路径换算 grace（"true"/"0" = grace 0 与 once
	// 一致冗余放行，CLI --once + 显式裸 --exit-when-empty 放行同档）；值剥离
	// 红线：detail 只含键名，禁含配置值（credential/client-option 记录式同款）。
	if fc != nil && fc.Once != nil && *fc.Once {
		if fc.MaxClients != nil && *fc.MaxClients != 1 {
			return cfg, nil, configErr(configPath, "conflicting keys", `key "once" conflicts with key "max-clients"`)
		}
		if fc.ExitWhenEmpty != nil {
			var ev exitEmptyValue
			if serr := ev.Set(*fc.ExitWhenEmpty); serr != nil {
				return cfg, nil, configErr(configPath, "invalid duration", `key "exit-when-empty"`)
			}
			if ev.grace != 0 {
				return cfg, nil, configErr(configPath, "conflicting keys", `key "once" conflicts with key "exit-when-empty"`)
			}
		}
	}
	// D-14 配置 exit-when-empty（07-06，OQ4 字符串单形态）：CLI 未显式给且
	// 配置键非 nil → exitEmptyValue.Set 单一解析路径复用（"true"/"0"/"30s"
	// 全通零双写；bool 形态已被 go-toml 类型不符在加载层拒绝）。必须在下方
	// --once 展开之前应用——配置不算显式（exitEmptySet 不置位），--once 展开
	// 随后覆盖配置值（flag > 配置优先级直接推论）；CLI 显式给定时
	// exitEmptySet 为真，配置与展开双双让位。
	if fc != nil && fc.ExitWhenEmpty != nil && !cfg.exitEmptySet {
		if serr := cfg.exitEmpty.Set(*fc.ExitWhenEmpty); serr != nil {
			return cfg, nil, configErr(configPath, "invalid duration", `key "exit-when-empty"`)
		}
	}
	// D-12：--once 语法糖展开 ≡ --max-clients=1 --exit-when-empty=0——未显式给
	// --max-clients 则置 1、未显式给 --exit-when-empty 则置 set+grace 0；显式给定
	// 时不覆盖（用户值保持，矛盾检测归 validateStartup——分层纪律：parse = 形状
	// 与展开，validate = 组合矛盾）。
	if cfg.once {
		if !cfg.maxClientsSet {
			cfg.maxClients = 1
		}
		if !cfg.exitEmptySet {
			cfg.exitEmpty.set = true
			cfg.exitEmpty.grace = 0
		}
	}
	if cfg.showVersion {
		return cfg, nil, nil
	}
	// D-01：--credential 回调内校验错误统一上报点——插入点在 showVersion 早退
	// 之后（纯信息路径不被配置校验阻断，03-04 先例）；记录式原因见 flag 定义注释。
	if credErr != nil {
		return cfg, nil, credErr
	}
	// P4 D-15：--client-option 回调内校验错误统一上报点——插入点在 showVersion
	// 早退之后（纯信息路径不被配置校验阻断，03-04 先例）；记录式原因见 flag 定义注释。
	if clientOptErr != nil {
		return cfg, nil, clientOptErr
	}
	// D-05：--write-policy parse 期枚举校验（插入点同 03-04 先例——showVersion
	// 早退之后）。枚举值非敏感，直接 return error（不经 clientOptErr 记录式——
	// 该形态仅用于值含敏感内容的 --client-option）。
	if cfg.writePolicy != server.WritePolicyOwner && cfg.writePolicy != server.WritePolicyAll {
		return cfg, nil, fmt.Errorf("invalid --write-policy %q: must be owner or all", cfg.writePolicy)
	}
	// 10-01 PC-01：--session-mode parse 期枚举校验（插入点同 write-policy
	// 先例——showVersion 早退之后；D-04 定案文案回显值——枚举值非敏感豁免
	// 面，凭据/token/文件内容红线保持；精确匹配两枚举常量，绝不宽容归一
	// 大小写/空白近形值——输入与生效模式分叉是配置漂移隐蔽源）。TOML 源
	// 非法值经默认值替换机制落 cfg.sessionMode 同一终值——一闸双覆盖
	//（pingInterval 负值闸注释同款机制）。
	if cfg.sessionMode != server.SessionModeShared && cfg.sessionMode != server.SessionModePerClient {
		return cfg, nil, fmt.Errorf("invalid --session-mode %q: must be shared or per-client", cfg.sessionMode)
	}
	// D-18 安全闸（07-review CR-03，SEC-01 值剥离红线族）：--auth-header
	// 凭据载体头名拒绝——配置即裸信任该头（D-16），其值逐 attach 事件进
	// logEvent remote_user；指向 Authorization/Proxy-Authorization/Cookie/
	// Set-Cookie 会把 Basic 凭据（base64）或会话 Cookie 随每个认证事件写进
	// stderr 落 journald 持久化，直接击穿 D-03「凭据绝不出现在 logEvent」
	// 红线（proxy.go 注释的「结构性保证」只论证了 token/ticket 进不来，未
	// 覆盖凭据头名配置——本闸封闭该结构性缺口）。http.CanonicalHeaderKey
	// 归一后比较（HTTP 头名大小写不敏感，aUtHoRiZaTiOn 同拒）。项目内无
	// ticket 头可列——ticket 走 Hello 帧、share token 走 /s/ 路径段，结构
	// 上不经 HTTP 头。拒绝文案只含 flag 名与类别枚举（公开协议常量），不
	// 回显用户所给值（credErr/clientOptErr 记录式同纪律）。CLI 与配置文件
	// 来源同闸（配置值经默认值替换机制落 cfg.authHeader 同一终值）。
	if cfg.authHeader != "" {
		switch http.CanonicalHeaderKey(cfg.authHeader) {
		case "Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie":
			return cfg, nil, errors.New("invalid --auth-header: credential-carrying header names are not allowed (Authorization, Proxy-Authorization, Cookie, Set-Cookie)")
		}
	}
	// D-22：--stop-signal 枚举校验（插入点同 03-04 先例——showVersion 早退之后，
	// write-policy 枚举校验同位）。名→信号映射在 pty 平台文件集中
	//（signal_linux.go/signal_darwin.go 同签名同表——parse 期枚举校验的唯一
	// 事实源；与 server.go signalName 的 signal→name 方向相反，不复用错方向）。
	// 值非敏感，错误文案可回显并列合法枚举（exitEmptyValue.Set 同纪律，非
	// SEC-01 面）。解析产物入 cfg.stopSignalSig 供 run() Options 接线。
	sig, ok := pty.StopSignalByName(cfg.stopSignal)
	if !ok {
		return cfg, nil, fmt.Errorf("invalid --stop-signal %q: must be HUP, TERM, INT or KILL", cfg.stopSignal)
	}
	cfg.stopSignalSig = sig
	// D-22：--stop-timeout 负值拒绝（DurationVar 直收下 "-5s" 解析成功，负值
	// 检查是唯一闸——exitEmptyValue.Set 负值闸同纪律；值非敏感可回显）。
	if cfg.stopTimeout < 0 {
		return cfg, nil, fmt.Errorf("invalid --stop-timeout %v: must be a non-negative duration (e.g. 2s)", cfg.stopTimeout)
	}
	// D-16：--ping-interval 负值拒绝（08-review WR-02——0 = 禁用为唯一合法非正
	// 形态；DurationVar 直收负值语法合法，负值检查是唯一闸，exitEmptyValue.Set/
	// --stop-timeout 同纪律；配置来源负值经默认值替换机制落同一终值，一闸双覆盖；
	// 值非敏感可回显）。缺闸时 pinger 按 interval<=0 静默不启动——用户笔误把
	// 保活关了而零报错（反代空闲超时收割表现为「终端莫名掉线」）。
	if cfg.pingInterval < 0 {
		return cfg, nil, fmt.Errorf("invalid --ping-interval %v: must be a non-negative duration (0 = disable keepalive)", cfg.pingInterval)
	}
	// D-24：--uid/--gid 值域校验（插入点同 03-04 先例——showVersion 早退之后，
	// write-policy 枚举校验同位）：-1 哨兵之外 < -1 或 > 4294967295 即拒
	//（uint32 转换安全——越界值 uint32 截断会降权到非预期账号，T-07-04b；
	// 值非敏感可回显）。
	if cfg.uid < -1 || cfg.uid > 4294967295 {
		return cfg, nil, fmt.Errorf("invalid --uid %d: must be -1 (unset) or 0..4294967295", cfg.uid)
	}
	if cfg.gid < -1 || cfg.gid > 4294967295 {
		return cfg, nil, fmt.Errorf("invalid --gid %d: must be -1 (unset) or 0..4294967295", cfg.gid)
	}
	// D-09：--socket-mode 八进制解析（插入点同 03-04 先例——showVersion 早退
	// 之后，write-policy 枚举校验同位）。值非敏感可回显（exitEmptyValue.Set
	// 同纪律，非 SEC-01 面）；>0777 含特殊位拒绝（T-07-02b：权限位是认证
	// 边界，不接纳 setuid/sticky 等漂移面）。
	mode, err := strconv.ParseUint(cfg.socketModeStr, 8, 32)
	if err != nil || mode > 0o777 {
		return cfg, nil, fmt.Errorf("invalid --socket-mode %q: must be octal permission bits (e.g. 0660)", cfg.socketModeStr)
	}
	cfg.socketMode = os.FileMode(mode)
	// D-09：--socket-owner parse 期名字解析（user.Lookup 得 uid/gid，有 group
	// 分量再 LookupGroup 覆盖 gid——07-RESEARCH Pattern 1 形态）。未知用户/组
	// 即拒；uid/gid 数字转换失败归并同拒（os/user 双实现下 Uid/Gid 恒为十进制
	// 串，防御性归并）。错误文案只含错误类别与 flag 名（用户名非敏感可回显，
	// 但固定类别文案不泄露系统细节之外信息）。
	if cfg.socketOwner != "" {
		name, group, hasGroup := strings.Cut(cfg.socketOwner, ":")
		u, lerr := user.Lookup(name)
		if lerr != nil {
			return cfg, nil, errors.New("invalid --socket-owner: unknown user or group")
		}
		uid, uerr := strconv.Atoi(u.Uid)
		gid, gerr := strconv.Atoi(u.Gid)
		if uerr != nil || gerr != nil {
			return cfg, nil, errors.New("invalid --socket-owner: unknown user or group")
		}
		if hasGroup {
			g, lerr := user.LookupGroup(group)
			if lerr != nil {
				return cfg, nil, errors.New("invalid --socket-owner: unknown user or group")
			}
			gid, gerr = strconv.Atoi(g.Gid)
			if gerr != nil {
				return cfg, nil, errors.New("invalid --socket-owner: unknown user or group")
			}
		}
		cfg.socketUid = uid
		cfg.socketGid = gid
	}
	// D-13：--base-path parse 期规范化+严格校验（插入点同 03-04 先例——
	// showVersion 早退之后，write-policy 枚举校验同位）。值非敏感，错误文案
	// 可回显（exitEmptyValue.Set 同纪律，非 SEC-01 面）。
	bp, err := normalizeBasePath(cfg.basePath)
	if err != nil {
		return cfg, nil, err
	}
	cfg.basePath = bp
	// D-04：cert/key 必须成对——只给其一 parse 期报错（分层纪律：此处报配置
	// 形态错误，validateStartup 不重复此项）。
	if (cfg.tlsCert == "") != (cfg.tlsKey == "") {
		return cfg, nil, errors.New("must give both --tls-cert and --tls-key")
	}
	// D-01：WESH_CREDENTIAL env 兜底——仅当 flag 未给任何凭据时解析 env 值；
	// flag 非空时 env 整体忽略（flag 优先）。
	if len(cfg.credentials) == 0 {
		if env := os.Getenv("WESH_CREDENTIAL"); env != "" {
			c, err := server.ParseCredential(env)
			if err != nil {
				return cfg, nil, fmt.Errorf("WESH_CREDENTIAL: %w", err)
			}
			cfg.credentials = append(cfg.credentials, c)
		}
	}
	// D-02/D-05 配置列表合并（07-06；env 块之后——执行序：flag 列表 → env →
	// 配置列表，flag > env > 配置成立）：CLI 回调填充非空 = 显式给出 → 整个
	// 列表替换（配置不应用，D-02）；CLI 未给且配置键非 nil → 配置列表逐项经
	// 各自 parse 期校验函数应用（ParseCredential/NormalizeOrigin/client-option
	// 白名单+JSON——同一校验函数复用零双写）。len==0 守卫同时承载 env 夹层：
	// env 非空则配置 credential 不应用（D-05）。被替换/遮蔽（CLI 或 env 给出）
	// 的配置列表不解析不校验——「不应用」语义的字面落地。配置 credential 与
	// client-option 校验错误走 credErr/clientOptErr 同款记录式（类别 + 键名，
	// 禁含值——统一上报点在本段末尾、showVersion 早退之后，纯信息路径不被
	// 阻断）；origin 错误同取值剥离形态（07-review IN-01：oerr.Error() 含 %q
	// 原输入不透传，detail 只含键名——与 credential/client-option 记录式
	// 形态对齐；CLI --origin 回调通道不经 configErr，其值非敏感回显纪律不变）。
	var cfgCredErr, cfgClientOptErr error
	if fc != nil && fc.Credential != nil && len(cfg.credentials) == 0 {
		for _, s := range fc.Credential {
			c, cerr := server.ParseCredential(s)
			if cerr != nil {
				cfgCredErr = configErr(configPath, "credential must be user:pass", `key "credential"`)
				break
			}
			cfg.credentials = append(cfg.credentials, c)
		}
	}
	if fc != nil && fc.Origin != nil && len(cfg.origins) == 0 {
		for _, s := range fc.Origin {
			n, oerr := server.NormalizeOrigin(s)
			if oerr != nil {
				return cfg, nil, configErr(configPath, "invalid origin entry", `key "origin"`)
			}
			cfg.origins = append(cfg.origins, n)
		}
	}
	if fc != nil && fc.ClientOption != nil && len(cfg.clientOptions) == 0 {
		for _, s := range fc.ClientOption {
			key, value, found := strings.Cut(s, "=")
			if !found {
				cfgClientOptErr = configErr(configPath, "invalid client-option entry: must be key=value", `key "client-option"`)
				break
			}
			if !proto.ValidClientOptionKey(key) {
				cfgClientOptErr = configErr(configPath, fmt.Sprintf("invalid client-option key %q", key), `key "client-option"`)
				break
			}
			var v json.RawMessage
			if uerr := json.Unmarshal([]byte(value), &v); uerr != nil {
				cfgClientOptErr = configErr(configPath, fmt.Sprintf("invalid client-option value for %q: not valid JSON", key), `key "client-option"`)
				break
			}
			cfg.clientOptions = append(cfg.clientOptions, clientOption{key: key, value: v})
		}
	}
	// 配置列表校验错误统一上报点（credErr/clientOptErr 同款记录式——值剥离
	// 红线：类别 + 键名，禁含值）。
	if cfgCredErr != nil {
		return cfg, nil, cfgCredErr
	}
	if cfgClientOptErr != nil {
		return cfg, nil, cfgClientOptErr
	}
	argv = fs.Args()
	// D-04：CLI `--` 后 argv 非空则覆盖配置 command；空且 command 键非空
	//（空数组按缺席语义——plan flagged_assumptions，与 CLI `--` 空 argv
	// 同档）→ 配置 command。
	if len(argv) == 0 && fc != nil && len(fc.Command) > 0 {
		argv = fc.Command
	}
	if len(argv) == 0 {
		return cfg, nil, errors.New("missing command")
	}
	// 10-01 PC-01：argv0 落定副本（CLI/配置 command 两源汇合后的最终
	// argv[0]——validateStartup per-client LookPath 预检数据源，10-02 消费）。
	cfg.argv0 = argv[0]
	return cfg, argv, nil
}

// aggregateClientPrefs 把 --client-option 收集项与 --osc52 合成 prefs 双档 blob
// （05-03 D-13 + P5-6）：ro 档（旁观者 + 降级递补者）= 聚合结果剔除 osc52 键——
// 即使全局 --osc52 开启也永不下发（osc52 是服务端专有键，聚合期已知，结构性排除
// 出 --client-option 白名单）；rw 档 = 聚合结果原样（按全局 --osc52 并入）。
// 产双 blob 保持服务端不透明透传纪律——不做运行期 JSON 手术（P5-6）。
// 零配置（无 client-option 且 osc52=false）两档均返回 nil——Welcome JSON 不出
// prefs 键（旧前端零漂移，P2 D-02）。同 key 重复给出时 last-wins（后者覆盖前者）。
func aggregateClientPrefs(opts []clientOption, osc52 bool) (ro, rw json.RawMessage) {
	if len(opts) == 0 && !osc52 {
		return nil, nil
	}
	m := make(map[string]json.RawMessage, len(opts)+1)
	for _, o := range opts {
		m[o.key] = o.value // last-wins：同 key 后者覆盖前者
	}
	roBlob, _ := json.Marshal(m) // ro 档：永不含 osc52 键（D-13）；固定值类型 marshal 不会失败
	if osc52 {
		m["osc52"] = json.RawMessage("true") // D-12：osc52 只能经服务端 flag 开启
	}
	rwBlob, _ := json.Marshal(m) // rw 档：按全局 --osc52 下发
	return roBlob, rwBlob
}

// normalizeBasePath 是 --base-path 的 parse 期规范化+严格校验（D-13，one-way
// 公开契约；isLoopbackBind 同位纯函数，NormalizeOrigin 先例形态）：空串 → 空串
// （未配置）；"/" → 空串（根视为未配置，D-13 显式裁决）；合法值原样返回。
// 拒绝形态（exit 2，绝不宽容自动修正——输入与生效值分叉是配置漂移隐蔽源）：
//   - 不以 / 开头（wesh）；
//   - 以 / 结尾（/wesh/——尾斜杠规范化由 mux 307 承担，配置侧不接受）；
//   - 含 ".."（/wesh/../x）；
//   - 含连续 "//"（//wesh、/w//esh）；
//   - 含 [A-Za-z0-9\-._~/] 之外字符（空格/?/#/% 等——% 拒绝使转义序列无从进入，
//     StripPrefix 精确前缀匹配语义保持）。
//
// 错误文案含原输入（值非敏感可回显，exitEmptyValue.Set 同纪律）：parse 期报错
// 面向部署者，需可定位是哪条值出问题。
func normalizeBasePath(s string) (string, error) {
	if s == "" || s == "/" {
		return "", nil // 未配置 / 根视为未配置（D-13）
	}
	if !strings.HasPrefix(s, "/") {
		return "", fmt.Errorf("invalid --base-path %q: must start with /", s)
	}
	if strings.HasSuffix(s, "/") {
		return "", fmt.Errorf("invalid --base-path %q: must not end with / (root / alone means unset)", s)
	}
	if strings.Contains(s, "..") {
		return "", fmt.Errorf("invalid --base-path %q: must not contain ..", s)
	}
	if strings.Contains(s, "//") {
		return "", fmt.Errorf("invalid --base-path %q: must not contain repeated slashes", s)
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_' || r == '~' || r == '/' {
			continue
		}
		return "", fmt.Errorf("invalid --base-path %q: character %q outside URL path safe set [A-Za-z0-9-._~/]", s, r)
	}
	return s, nil
}

// isLoopbackBind 判定 bind 地址是否仅本机可达（RESEARCH Pattern 7 裁决）：
// 空串视为全网卡（非 loopback）；net.ParseIP 成功取 IsLoopback()（127.0.0.0/8
// 与 ::1）；localhost 特判 loopback；0.0.0.0/:: 与其他主机名保守按非 loopback——
// 保守方向出错的代价是多要一个显式逃生门，不削弱安全。
func isLoopbackBind(bind string) bool {
	if bind == "localhost" {
		return true
	}
	ip := net.ParseIP(bind)
	return ip != nil && ip.IsLoopback()
}

// outboundIPv4 取本机出站 IPv4（05-06 分享链接 host 回填，D-04/R-04，RESEARCH
// Pattern 7 形态）：UDP-dial 路由感知优先——net.Dial("udp", "192.0.2.1:80")
// （RFC 5737 TEST-NET-1 文档地址；UDP dial 无握手零流量，仅让内核按路由表选出
// 站接口的本地地址——结构性避开 docker0/bridge：朴素接口扫描在多 docker 桥接口
// 机器上必中 docker0，2026-08-19 本机实证）；失败 fallback net.Interfaces()
// 索引序首个 up && !loopback 接口的首个 IPv4；全失败返回 ""（调用方兜底打印
// bind 原样，不阻断启动——失败退化为次优展示地址而非功能故障，operator 可从
// listening on 行获得真实地址）。
func outboundIPv4() string {
	if conn, err := net.Dial("udp", "192.0.2.1:80"); err == nil { // RFC 5737，永不真实路由
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			if ip4 := addr.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	if ifaces, err := net.Interfaces(); err == nil {
		for _, ifa := range ifaces {
			if ifa.Flags&net.FlagUp == 0 || ifa.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := ifa.Addrs()
			if err != nil {
				continue
			}
			for _, a := range addrs {
				var ip net.IP
				switch v := a.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip4 := ip.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return ""
}

// validateStartup 是 D-03/D-05 启动校验矩阵（RESEARCH Pattern 7 八行）的纯函数
// 落地——无副作用：禁止 listen/spawn/写文件；os.Stat 只读探测允许（07-04 D-21
// --cwd 预检），必须先于 pty.Start/net.Listen 执行：拒绝路径零资源占用，测试
// 不挂死（main_test.go captureFd 纪律）。
// 返回 warn（放行但需 stderr 醒目警告的逃生门/明文场景）或 err（拒绝启动）。
// cert/key 成对校验在 parseArgs 已落 parse 期（D-04 分层，此处不重复）。
// 红线：warn/err 文案不得含凭据值（SEC-01 日志红线延伸到启动面）。
func validateStartup(cfg config) (warn string, err error) {
	// D-05 组合校验（配置矛盾 fail-fast，P3 校验矩阵纪律）：显式设置
	// --write-policy 却未开 --writable 总闸 → 拒绝（写策略只在可写会话有意义，
	// 显式要求写策略却没开写总闸是用户配置错误，exit 2 零窗口暴露）。
	// 默认 owner 未显式设置 + 无 --writable 是纯 ro 会话正常形态不拒。
	// 与 bind 安全形态无关（纯配置矛盾），故在 loopback 早退之前判定。
	if cfg.writePolicySet && !cfg.writable {
		return "", errors.New("--write-policy is set but --writable is not; write policy only applies when client input is enabled")
	}
	// D-01/D-02 组合 warn（10-02，放行但 stderr 醒目警告——静默永不接受，
	// ROADMAP 锁定）：writePolicySet 显式设置位（owner|all 任一，CLI/TOML
	// 双源经 07-06 显式位机制同档置位）× sessionMode=per-client → owner/all
	// 仲裁与递补语义在 per-client 下不装配（ro/rw 权限级别仍按 ticket 生效
	// ——D-01 否决 exit 2 的理据），警告行含双 flag 名。锚定模式终值而非
	// sessionModeSet——终值判定即双源覆盖。本 warn 在 socket/loopback 早退
	// 也可达（纯配置语义与 bind 安全形态无关），故以累积变量 + 各透出点
	// 拼接落地（合并形态保证下方既有非 loopback 安全警告不被遮蔽——两类
	// 文案均达 stderr，安全警告在前显著性优先；既有 warn 文案逐字未动）。
	var modeWarns []string
	if cfg.writePolicySet && cfg.sessionMode == server.SessionModePerClient {
		modeWarns = append(modeWarns, "wesh: warning: --write-policy has no effect with --session-mode=per-client; owner/all arbitration and succession are not assembled in per-client mode (ro/rw permission levels still apply per ticket)")
	}
	// mergeWarn 把累积 modeWarns 拼到各透出点：sec 为既有安全警告原文
	//（在前，显著性优先）；sec 为空（socket/loopback/最强形态早退）时透出
	// 累积 warn；两者皆空返回 ""（零漂移——strings.Join(nil) == ""）。
	mergeWarn := func(sec string) string {
		if sec == "" {
			return strings.Join(modeWarns, "\n")
		}
		if len(modeWarns) == 0 {
			return sec
		}
		return sec + "\n" + strings.Join(modeWarns, "\n")
	}
	// D-12 组合校验（配置矛盾 fail-fast，write-policy 行同位——纯配置矛盾与
	// bind 安全形态无关，loopback 早退之前判定）：--once 与显式矛盾值同给即拒，
	// 双 flag 名进文案。判定锚定显式设置位而非展开后终值（review #3 吸收——
	// 展开只填未显式位时两形态逻辑等价，但显式设置位判定不依赖该不变量，
	// 自证性更强）；--once + 显式 --max-clients=1 / 显式裸 --exit-when-empty
	// 为一致冗余放行（设置位为真但值一致）。
	if cfg.once && cfg.maxClientsSet && cfg.maxClients != 1 {
		return "", errors.New("--once conflicts with --max-clients: --once implies --max-clients=1")
	}
	if cfg.once && cfg.exitEmptySet && cfg.exitEmpty.grace != 0 {
		return "", errors.New("--once conflicts with --exit-when-empty grace: --once implies immediate exit (--exit-when-empty=0)")
	}
	// D-08 数值校验（配置错误 fail-fast，P3 校验矩阵纪律）：--max-clients ≤0
	// 无意义（容量必须为正——0/负值会使③位 503 闸恒触发，全员被拒）。纯配置
	// 有效性与 bind 安全形态无关，故在 loopback 早退之前判定（write-policy
	// 组合校验同款落点）。
	if cfg.maxClients <= 0 {
		return "", errors.New("--max-clients must be positive")
	}
	// D-21 预检（配置错误 fail-fast，write-policy 行同位——纯配置有效性与
	// bind 安全形态无关，loopback 早退之前判定）：--cwd 非空时 os.Stat 只读
	// 探测（纯函数纪律允许只读探测，见函数头注释）；不存在或非目录即拒
	//（spawn 前零资源占用——spawn 后才发现 ENOENT 则资源已占用且错误面到
	// 客户端，RESEARCH Anti-Patterns）。值非敏感，错误文案可回显路径
	//（exitEmptyValue.Set 同纪律，非 SEC-01 面）。
	if cfg.cwd != "" {
		if fi, serr := os.Stat(cfg.cwd); serr != nil || !fi.IsDir() {
			return "", fmt.Errorf("invalid --cwd %q: not an existing directory", cfg.cwd)
		}
	}
	// SC4 预检（10-02，--cwd 行同位——纯配置有效性与 bind 安全形态无关，
	// loopback 早退之前判定；exec.LookPath/os.Stat 为只读探测，零资源占用，
	// 纯函数纪律内——os.Stat 只读探测先例）：per-client 把 spawn 推迟到首个
	// 客户端 attach（Phase 11），命令缺失若不在启动期暴露则推迟为 attach 期
	// 故障——启动期 fail-fast 是其结构性补偿。仅 per-client × argv0 非空
	// 触发：shared（含零值模式）与 argv0 空串不预检——shared 启动行为零漂移
	//（spawn 失败仍走 pty.Start exit 1 现状通道）。命令名非敏感可 %q 回显
	//（--cwd 路径回显先例，非 SEC-01 面）。
	// 10-review WR-01：预检与 spawn 语义对齐——argv0 含 '/' 时不经 PATH
	// 解析（child 在 chdir(cfg.cwd) 之后 execve，相对路径按 --cwd 解析，
	// spawn.go cmd.Dir 注释同款语义；LookPath 在服务端进程 cwd 下解析且对
	// 相对 slash 路径返回 ErrDot，双向发散），改为 --cwd 感知的可执行 stat
	// 探测（不存在/目录/无执行位同归「not executable」——带斜杠路径不经
	// PATH，文案不再称 not found in PATH）；不含 '/' 才走 LookPath（PATH
	// 解析与父子进程 cwd 无关，无 cwd 错位面）。
	if cfg.sessionMode == server.SessionModePerClient && cfg.argv0 != "" {
		probe := cfg.argv0
		if strings.ContainsRune(probe, '/') && cfg.cwd != "" && !filepath.IsAbs(probe) {
			probe = filepath.Join(cfg.cwd, probe) // 与 child chdir 后 execve 的解析对齐
		}
		if strings.ContainsRune(probe, '/') {
			if fi, serr := os.Stat(probe); serr != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
				return "", fmt.Errorf("invalid command %q: not executable (per-client startup preflight)", cfg.argv0)
			}
		} else if _, lerr := exec.LookPath(probe); lerr != nil {
			return "", fmt.Errorf("invalid command %q: not found in PATH (per-client startup preflight)", cfg.argv0)
		}
	}
	// D-07 预检（09-04，--cwd 行同位——纯配置有效性与 bind 安全形态无关，
	// loopback 早退之前判定）：--index 非空时 os.Stat 只读探测（纯函数纪律
	// 允许只读探测，见函数头注释）——不存在或非常规文件（目录/设备/socket
	// 同归此类）即拒（spawn 前零资源占用，loadCustomIndex 读入前的第一闸）。
	// 错误行只含路径与原因类别，绝不含文件内容任何字节（D-08 红线——自定义页
	// 可能含 operator 私有信息；路径非敏感可回显，--cwd 同纪律）。
	if cfg.index != "" {
		if fi, serr := os.Stat(cfg.index); serr != nil {
			return "", fmt.Errorf("invalid --index %q: file does not exist", cfg.index)
		} else if !fi.Mode().IsRegular() {
			return "", fmt.Errorf("invalid --index %q: not a regular file", cfg.index)
		}
	}
	// D-08：index-max-size 纯配置键数值校验（TOML 整数字节，无 CLI flag）——
	// ≤0 拒绝（0/负值使 LimitReader 读入上限失义；「显式」哲学：给的键值无效
	// 即 fail-fast）。配置键名入文案合法（键名非值）。
	if cfg.indexMaxSize <= 0 {
		return "", errors.New("invalid index-max-size: must be positive")
	}
	// 09-review WR-05：上限上界钳制（2GiB 硬顶，与 ≤0 拒绝同位 fail-fast）——
	// MaxInt64 一类「实际无限大」笔误会使 loadCustomIndex 的 int64(max)+1 回绕
	// 为负，LimitReader 对 N≤0 立即 EOF → ReadAll 得 0 字节、len(data)>max 不
	// 成立 → wesh 正常启动并对全部页通道伺服 200 空 body（无任何错误行的
	// 静默失败，最难排查形态）。2GiB 为自定义页合理尺寸的远上界（默认 16MiB
	// 的 128 倍），兼防无界读入；键名与上限数值入文案合法（非值）。
	if cfg.indexMaxSize > 1<<31-1 {
		return "", errors.New("invalid index-max-size: exceeds 2GiB cap")
	}
	// D-24 组合校验（配置矛盾 fail-fast，write-policy 行同位——纯配置矛盾与
	// bind 安全形态无关，loopback 早退之前判定）：--uid/--gid 成对强制——只给
	// 一个 = 配置矛盾零窗口暴露（降权半配置静默放行 = 子进程以原权运行，
	// T-07-04b Elevation of Privilege；exit 2 而非降级运行），双 flag 名进文案。
	if (cfg.uid == -1) != (cfg.gid == -1) {
		return "", errors.New("--uid and --gid must be given together")
	}
	// D-08 组合校验（配置矛盾 fail-fast，write-policy 行同位——纯配置矛盾与
	// bind 安全形态无关，loopback 早退之前判定）：--socket 与显式 --port/--bind
	// 同给即拒，双 flag 名进文案。判定锚定显式设置位而非终值（write-policy/
	// max-clients/exit-empty 三先例同形态）——--socket 与默认 port/bind 同给
	// 是纯 unix 形态，不误判冲突。
	if cfg.socket != "" && (cfg.portSet || cfg.bindSet) {
		return "", errors.New("--socket conflicts with --port/--bind: unix socket listen and TCP listen are mutually exclusive")
	}
	// D-09 组合校验（配置矛盾 fail-fast，同位纪律）：--socket-mode/--socket-owner
	// 仅随 --socket 有意义——单独给出 = 配置矛盾零窗口暴露（给了无法兑现的
	// flag = 配置错误，「显式」哲学一贯性）。
	if (cfg.socketModeSet || cfg.socketOwnerSet) && cfg.socket == "" {
		return "", errors.New("--socket-mode/--socket-owner require --socket: socket permission flags only apply to unix socket listen")
	}
	// D-26/OQ1 组合校验（配置矛盾 fail-fast，同位纪律——纯配置矛盾与 bind 安全
	// 形态无关，且必须在下方 D-11 socket 早退之前判定否则结构性不可达）：
	// --socket × --open 同给即拒——unix socket 形态无 host:port 可拼（D-12 分享
	// 链接已退化为提示行），--open 需要 http(s) URL；给了无法兑现的 flag 组合
	// = 配置错误（「显式」哲学一贯性，RESEARCH OQ1 建议行落地），双 flag 名进
	// 文案。值非敏感可回显（exitEmptyValue.Set 同纪律，非 SEC-01 面）。
	if cfg.open && cfg.socket != "" {
		return "", errors.New("--open conflicts with --socket: no http URL to open on unix socket listen")
	}
	// D-11：unix socket 形态跳过 bind 安全矩阵（isLoopbackBind 早退及其后全部）——
	// 文件系统权限即认证边界，--socket-mode/--socket-owner 就是访问控制，
	// loopback 早退同款信任档位；流量不出机，有无凭据/TLS 均放行免警告。
	if cfg.socket != "" {
		return mergeWarn(""), nil
	}
	if isLoopbackBind(cfg.bind) {
		return mergeWarn(""), nil // loopback：流量不出机，有无凭据/TLS 均放行免警告（D-03/D-05）
	}
	if len(cfg.credentials) == 0 {
		if !cfg.noAuth {
			return "", errors.New("refusing to listen on non-loopback address without credentials; pass --no-auth to disable authentication") // D-03
		}
		// D-16 暴露面警告（07-03）：--auth-header 非空 + bind 非 loopback + 无凭据
		// ——配置即裸信任该头，直连客户端可自设伪造（审计归因失真；D-17 正交下
		// 伪造头不能越权，但日志归因会被污染）。警告文案含 --auth-header flag 名、
		// 不含任何头值（启动面红线同 TestStartupMatrix 纪律）；socket 形态 bind
		// 矩阵已跳过（上方 D-11 早退），同行跳过本警告——unix socket 信任边界
		// 同 D-11 逻辑。无凭据裸奔语义（--no-auth）随同行保持不丢。
		if cfg.authHeader != "" {
			return mergeWarn("wesh: warning: listening on non-loopback address with NO authentication (--no-auth) and --auth-header enabled; anyone who can reach this port gets a terminal, and directly connecting clients can forge the auth header — ensure wesh is not directly exposed (front it with a reverse proxy that sets the header)"), nil
		}
		return mergeWarn("wesh: warning: listening on non-loopback address with NO authentication (--no-auth); anyone who can reach this port gets a terminal"), nil
	}
	if cfg.tlsCert == "" {
		if !cfg.insecureHTTP {
			return "", errors.New("refusing to serve credentials over plaintext HTTP on non-loopback address; pass --insecure-http or provide --tls-cert/--tls-key") // D-05
		}
		return mergeWarn("wesh: warning: serving credentials over plaintext HTTP on non-loopback address (--insecure-http); prefer --tls-cert/--tls-key or a TLS-terminating reverse proxy"), nil
	}
	return mergeWarn(""), nil // 非 loopback + 凭据 + TLS：最强形态免警告
}

// listenSocket 是 --socket 形态的 unix socket listen 序列（D-08/D-09/D-10；
// run() 上方的纯 helper，isLoopbackBind 同位纪律）：
// Lstat 类型闸 → os.Remove → net.Listen("unix") → os.Chmod → uid>=0 时
// os.Chown。任一步失败即 ln.Close() 回滚并返回 error——UnixListener 默认
// unlink:true（GOROOT unixsock_posix.go:210-216,230），Close 自动删文件，
// 回滚零残留（T-07-02a/b：Chmod/Chown 失败必须回滚退出而非带病放行）。
//
// 顺序敏感依据（07-RESEARCH Pattern 1，全部 GOROOT 实证）：
//   - D-10：Go 的 listenStream 直接 syscall.Bind，无 bind 前 unlink——残留
//     socket 必收 EADDRINUSE，os.Remove 是必需而非保险（systemd Restart= 场景
//     零人工干预）；文件不存在时忽略 Remove 错误。
//   - D-10 边界收窄（07-review CR-01）：Remove 前 Lstat 判定类型——存在且
//     非 socket（普通文件/目录/FIFO/symlink 等）拒绝启动而非删除：D-10 意图
//     仅为清理残留 IPC 端点，operator 手误指向普通文件（root/systemd 部署下
//     有权限删除）即静默丢数据，超出决策面；Lstat 不跟随符号链接，symlink
//     同按非 socket 拒绝（保守方向：不理解的类型一律不删）。错误文案只含
//     路径与类别（--cwd 预检同纪律，路径非敏感可回显）。
//   - D-10 收窄链第二环（07-10 G-07-3）：类型闸之后按活性再分——存活 socket
//     与残留 socket 在文件类型上不可区分，以能否建连区分（net.Dial unix：
//     连通 = 存活实例占用 → 拒绝启动，07-02 OPS-01 设计答案；ECONNREFUSED =
//     残留 → 照旧清理）。CR-01 前的无条件 Remove 会把存活实例孤儿化（进程
//     在跑但端点被夺走，第二实例 unlink 后 listen 成功成静默赢者）——
//     「存活实例被孤儿化」正是本 gap 修复对象。
//   - D-09：socket 文件 mode 由内核定为 0777&~umask，Go 不做任何 chmod——
//     0660 确定性必须 listen 后显式 Chmod 达成（文件系统权限即认证边界，
//     权限不得由 umask 漂移决定）；uid<0（owner 未给，-1 哨兵）跳过 Chown。
//   - Chmod/Chown 与 listening 打印之间的 umask 窗口风险接受（RESEARCH A5）：
//     调用方在本函数返回后才打地址行，窗口内无客户端被指引。
func listenSocket(path string, mode os.FileMode, uid, gid int) (net.Listener, error) {
	// D-10 类型闸（07-review CR-01）：仅残留 socket 端点可 Remove；其他现存
	// 类型拒绝启动——拒绝经 run() listen 失败通道落地（net.Listen 失败同档，
	// exit 1 运行时错误 tier），文件内容零触碰。
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("%s exists and is not a socket", path)
		}
		// G-07-3 活性探测（D-10 收窄链第二环）：Dial 连通 = 存活实例占用 →
		// 拒绝启动，错误文案与 net.Listen EADDRINUSE 逐字全等（经 run() listen
		// 失败通道落地 exit 1，07-02 OPS-01 设计答案，静默赢者结构性消除）。
		// Dial 失败全形态按残留处理落 Remove：ECONNREFUSED = 无进程监听；
		// EACCES 等按「不可服务即残留」（跨用户活体误删由目录写权限/sticky 位
		// 结构性抑制；D-10 systemd Restart= 零人工干预优先——07-10
		// flagged_assumptions 登记）。TOCTOU 窗口两向安全降级：探测后实例
		// 死亡 → 本次拒绝、下次启动清理；清理后对手抢绑 → 下方 net.Listen
		// 真 EADDRINUSE 兜底——两向均无静默赢者。
		conn, derr := net.Dial("unix", path)
		if derr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("listen unix %s: bind: address already in use", path)
		}
		_ = os.Remove(path) // D-10：残留 socket 即垃圾；Remove 失败由下方 Listen 报错承载
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, mode); err != nil {
		_ = ln.Close() // Close 自动 unlink（unlink:true 默认）——回滚零残留
		return nil, err
	}
	if uid >= 0 {
		if err := os.Chown(path, uid, gid); err != nil {
			_ = ln.Close() // 同上——回滚零残留
			return nil, err
		}
	}
	return ln, nil
}

// loadCustomIndex 是 --index 自定义首页的启动读入（09-04，D-07/D-08；run()
// 上方的纯 helper——listenSocket 同位纪律）：io.LimitReader(max+1) 读入后按
// len(data) > max 判定超限（+1 使恰顶文件可读全、超顶文件多读一字节即拒——
// 防 io.ReadAll 无顶读入把误指的巨大文件吃光内存，Pitfall 9/T-09-04b）。
// 错误行只含路径 + 原因类别 + 上限数值，绝不含文件内容任何字节（D-08 红线：
// 自定义页可能含 operator 私有信息，超限/不可读场景尤其不得回显文件头字节，
// P3/P4 启动面红线延伸——测试以内容探针串反断言）。0 字节文件合法（D-07
// 拒绝列表 = 不存在/不可读/非常规/超限，不含空文件——伺服空白页是用户明示
// 的整页替换语义，「验证为主」纪律不过度校验）。
func loadCustomIndex(path string, max int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("invalid --index %q: cannot read", path)
	}
	defer func() { _ = f.Close() }()
	data, rerr := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if rerr != nil {
		// ReadAll 失败（生产路径不可达——stat 预检已过且目录已被拒；防御性
		// 归并 cannot read 类别，不透传 OS 错误文本）。
		return nil, fmt.Errorf("invalid --index %q: cannot read", path)
	}
	if len(data) > max {
		return nil, fmt.Errorf("invalid --index %q: exceeds index-max-size (%d bytes)", path, max)
	}
	return data, nil
}

func run(args []string) int {
	cfg, argv, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // --help：flag 包已打印用法
		}
		fmt.Fprintf(os.Stderr, "wesh: %v; usage: wesh [flags] -- <cmd> [args...]\n", err)
		return 2
	}
	if cfg.showVersion {
		fmt.Printf("wesh %s\n", version)
		return 0
	}
	// D-03/D-05 启动校验矩阵：先于 pty.Start/net.Listen——拒绝路径零资源占用；
	// 逃生门放行路径 stderr 醒目警告（裸奔/明文语义，警告不含凭据值）。
	warn, err := validateStartup(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 2
	}
	if warn != "" {
		fmt.Fprintln(os.Stderr, warn)
	}
	// G-03-5 根因①：TLS 证书启动预检——先于 pty.Start/net.Listen/listening 打印，
	// 坏证书路径在零资源占用阶段报错退出（无 spawn/无 listen/无 listening 打印），
	// 与 validateStartup「拒绝路径零资源占用」纪律一致；exit 1 与 pty.Start/
	// net.Listen 失败路径同档（运行时 I/O 错误，非 validateStartup 的 exit 2
	// 配置矩阵错误）。预加载的证书对留存供下方 ServeTLS 复用（单一事实源）。
	// 红线：错误文案只含证书路径与 OS/tls 包错误（fmt %v 透传），绝不含凭据值
	// （SEC-01 日志红线延伸到启动面，与 TestStartupMatrix 同纪律）。
	var cert tls.Certificate
	if cfg.tlsCert != "" {
		cert, err = tls.LoadX509KeyPair(cfg.tlsCert, cfg.tlsKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
			return 1
		}
	}
	// D-07 启动读入（09-04）：--index 给定时 loadCustomIndex 一次读入内存——
	// validateStartup warn 打印之后、pty.Start 之前（零资源占用拒绝纪律与
	// TLS 预检同位；exit 2 与校验矩阵同语义，spawn 前失败零资源占用）。
	// 未配置 --index 时 customIndex 保持 nil（Options 零值兜底——内建页伺服
	// 现状逐字节一致）。运行期零磁盘依赖：读入字节常驻内存经 Options 透传
	// 伺服层，改文件需重启生效。
	var customIndex []byte
	if cfg.index != "" {
		customIndex, err = loadCustomIndex(cfg.index, cfg.indexMaxSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
			return 2
		}
	}
	// 10-01 PC-01 装配期一次分岔：per-client 模式装配 SpawnFunc 闭包（捕获
	// argv+StartOptions，函数体为 pty.StartWithSize 直通——Phase 11 attach 期
	// spawn 的装配挂点，本阶段零调用方 inert，T-10-01c；闭包内 StartOptions
	// 字面量与下方 pty.Start 内联字面量为 Phase 11 重写前的临时重复形态，
	// 切换 sess=nil + attach 期 spawn 时收编）。shared 模式 spawnFunc 保持
	// 零值 nil（ValidateOptions 互斥契约锚定）。
	// planner 裁定（10-01-PLAN Task 1 ⑥，executor 不得回改）：两模式本阶段
	// 均经启动期 pty.Start 创建 sess——PATTERNS §3「sess = nil」建议与 New
	// 体 server.go session_start emit 的 sess.Cmd.Process.Pid 取引用冲突
	//（nil 即 panic），且违反 D-05「与 shared 等价」注记与全部 inert 约束；
	// sess=nil 形态归 Phase 11 生命周期主干。
	var spawnFunc func(cols, rows int) (*pty.Session, error)
	if cfg.sessionMode == server.SessionModePerClient {
		startOpts := pty.StartOptions{Dir: cfg.cwd, Term: cfg.term, Uid: cfg.uid, Gid: cfg.gid}
		spawnFunc = func(cols, rows int) (*pty.Session, error) {
			return pty.StartWithSize(argv, startOpts, cols, rows)
		}
	}
	// 10-01 PC-01：装配契约 fail-fast——ValidateOptions 前移至资源获取之前
	//（10-review WR-02：两输入 cfg.sessionMode 与分岔产物 spawnFunc 在分岔块
	// 尾部即已完全确定，校验只读该两字段，最小字面量与完整 opts 语义等价）。
	// 守卫触发时零资源占用——spawn/listen 均未发生，无 sess/ln 可回滚，与
	// validateStartup「拒绝路径零资源占用」纪律同构（原位调用在 pty.Start 与
	// listen 之后，失败分支既无 sess.Close 也无 ln.Close，违反其注释自引
	// 纪律）。失败经 validateStartup 同款 exit 2 通道形态。
	if verr := server.ValidateOptions(server.Options{SessionMode: cfg.sessionMode, SpawnFunc: spawnFunc}); verr != nil {
		fmt.Fprintf(os.Stderr, "wesh: %v\n", verr)
		return 2
	}
	// D-21/D-24 接线（07-04）：--cwd/--term 落 StartOptions Dir/Term；--uid/--gid
	// 落 Uid/Gid（-1 哨兵 = 不降权，Task 3 完成 flag 注册与成对校验）。
	sess, err := pty.Start(argv, pty.StartOptions{Dir: cfg.cwd, Term: cfg.term, Uid: cfg.uid, Gid: cfg.gid})
	if err != nil {
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	// D-08：listen 分岔——--socket 给定时走 unix socket（Remove→Listen→Chmod→
	// Chown 序列见 listenSocket 注释），否则现状 TCP 一行；失败回滚块两分岔
	// 共用现状形态（sess.Close + 打印 + return 1，net.Listen 失败路径逐字对称）。
	var ln net.Listener
	if cfg.socket != "" {
		ln, err = listenSocket(cfg.socket, cfg.socketMode, cfg.socketUid, cfg.socketGid)
	} else {
		// JoinHostPort 对 IPv4/主机名输出与 %s:%d 逐字相同，仅对 IPv6 字面量
		// 加方括号（--bind ::1 拼出 [::1]:7681）——WR-01：IPv6 行为零漂移修复。
		ln, err = net.Listen("tcp", net.JoinHostPort(cfg.bind, strconv.Itoa(cfg.port)))
	}
	if err != nil {
		// 启动失败路径回滚已 spawn 资源：Close master 后子进程（setsid 组长）
		// 收 SIGHUP 退出，不留孤儿进程。
		_ = sess.Close()
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	prefsRO, prefsRW := aggregateClientPrefs(cfg.clientOptions, cfg.osc52)
	// MULTI-05 分享链接（05-06，D-01/D-02）：启动时生成 ro/rw 两明文 token——
	// 每轮启动重新随机（重启即废全部旧链接，吊销语义 = 重启）；main 持明文供
	// 启动打印，server 只存 SHA-256 预哈希（Options 注释）。
	shareRO := server.GenerateShareToken()
	shareRW := server.GenerateShareToken()
	// D-12/D-14 接线：ExitWhenEmpty 两键直传解析产物（--once 展开后同通道——
	// 服务端无 --once 概念，SESS-01 = maxClients=1 + ExitWhenEmpty grace 0 的
	// 组合语义，06-02 空触发机制消费）。
	// 10-01 PC-01：字面量尾部只追加 SessionMode/SpawnFunc 两键（既有键序
	// 不重排——shared 路径逐字节零回归）；提取为命名 opts 供 New 消费。
	// 装配契约校验已前移至 pty.Start 之前（10-review WR-02——守卫触发时
	// 零资源占用），此处不再重复调用。
	opts := server.Options{Writable: cfg.writable, WritePolicy: cfg.writePolicy, PingInterval: cfg.pingInterval, Credentials: cfg.credentials, Origins: cfg.origins, TLS: cfg.tlsCert != "", ClientPrefsRO: prefsRO, ClientPrefsRW: prefsRW, MaxClients: cfg.maxClients, ExitWhenEmpty: cfg.exitEmpty.set, ExitWhenEmptyGrace: cfg.exitEmpty.grace, ShareTokenRO: shareRO, ShareTokenRW: shareRW, BasePath: cfg.basePath, AuthHeader: cfg.authHeader, StopSignal: cfg.stopSignalSig, StopTimeout: cfg.stopTimeout, Version: version, CustomIndex: customIndex, SessionMode: cfg.sessionMode, SpawnFunc: spawnFunc}
	srv := server.New(sess, os.Exit, opts)
	// shareURLRO/shareURLRW 拼串单一事实源（07-01 D-14 既定注释）：启动打印与
	// 07-05 --open 两消费点共用（两消费点不得各自重拼）；socket 分支保持零值
	// 空串（该形态 --open 已被 validateStartup 拒绝，下方消费点结构性不可达）。
	var shareURLRO, shareURLRW string
	if cfg.socket != "" {
		// D-12：unix 形态启动打印——地址行打 unix:// 前缀 + cfg.socket 原样
		//（/run/wesh.sock 即得三斜杠形态）；分享链接两行退化为单行提示
		//（无 host:port 可拼时绝不拼误导性 TCP 链接；反代后链接由反代 URL
		// 决定）。本分支位于 hs 装配之前——打印时 listenSocket 的 Chmod/Chown
		// 已完成（umask 窗口内不指引客户端，RESEARCH A5 风险接受）。
		// ln.Addr().(*net.TCPAddr) 断言防御留在 TCP 分支——unix 形态天然
		// 不拼 TCP 端口（RESEARCH Pattern 1 关键事实末条）。
		fmt.Printf("listening on unix://%s\n", cfg.socket)
		fmt.Println("share links: unavailable on unix socket (front with a reverse proxy to share)")
	} else {
		// D-07：启动仅打印单行（无 banner/emoji）；port 0 时 Addr 已是实际端口（D-06）。
		// scheme 分支感知（D-04）：TLS 启用时打印 https://。
		scheme := "http"
		if cfg.tlsCert != "" {
			scheme = "https"
		}
		fmt.Printf("listening on %s://%s\n", scheme, ln.Addr())
		// MULTI-05 分享链接两行（05-06，D-03/D-04/D-05）：启动打印是产品行为
		// （MULTI-05 明确授权），token 永不入 logEvent/stderr 事件流（D-03 红线——
		// 这两处 stdout Printf 与 URL 路径本身是 token 的全部合法输出面）。
		// host 回填（D-04/R-04）：bind 为全网卡形态（0.0.0.0/::/空串）→ outboundIPv4
		// 路由感知回填（空则兜底 bind 原样，不阻断启动）；具体 bind 原样使用
		// （loopback bind 打印 loopback——链接本机自用）；端口取 ln.Addr() 实际值
		// （--port 0 随机端口形态，D-06）；scheme 随 TLS 分岔（上方既有分支）。
		// rw 行仅 --writable 时打印（D-05 总闸——不给 --writable 只打印 ro 行）。
		shareHost := cfg.bind
		if cfg.bind == "" || cfg.bind == "0.0.0.0" || cfg.bind == "::" {
			if ip := outboundIPv4(); ip != "" {
				shareHost = ip
			}
		}
		sharePort := cfg.port
		if ta, ok := ln.Addr().(*net.TCPAddr); ok {
			sharePort = ta.Port
		}
		// D-14：分享链接路径含 base-path 前缀（拼串在 hostport 与 /s/ 之间注入
		// cfg.basePath——空串时与现状逐字节一致）。shareURLRO/shareURLRW 是本打印点
		// 与 07-05 --open 消费的单一事实源（拼串唯一出口，两消费点不得各自重拼）。
		shareURLRO = fmt.Sprintf("%s://%s%s/s/%s/", scheme, net.JoinHostPort(shareHost, strconv.Itoa(sharePort)), cfg.basePath, shareRO)
		shareURLRW = fmt.Sprintf("%s://%s%s/s/%s/", scheme, net.JoinHostPort(shareHost, strconv.Itoa(sharePort)), cfg.basePath, shareRW)
		fmt.Printf("share read-only:  %s\n", shareURLRO)
		if cfg.writable {
			fmt.Printf("share read-write: %s\n", shareURLRW)
		}
	}
	// 显式 http.Server：ReadHeaderTimeout=5s 盒住预认证 HTTP 层慢 loris（与
	// helloTimeout 同 5s 量级，D-04）；ReadTimeout/WriteTimeout 不设——会误伤
	// WS 长连接语义（升级后的连接读写在握手后长期空闲/突发均属正常）。
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	// D-23：SIGTERM/SIGINT 捕获 → srv.Shutdown()（1001 优雅下线广播 + stop-signal
	// 序列，server.go Shutdown 注释）。不调 exitf：Shutdown 是触发源不是 exitf
	// 分支——进程终结仍由 lifecycle 子进程死亡路径收口（P1 硬约束，零新 exit
	// 分支）；goroutine 内先等 Done → stopSignals 恢复默认处置 → Shutdown
	//（NotifyContext 官方推荐形态——07-review WR-01：首次信号后若不恢复默认，
	// Shutdown 全程（Close 内建最长 10s + stopTimeout）后续 SIGTERM/SIGINT 被
	// 转发进无人读取的 channel 丢弃，operator 双击 Ctrl+C 强杀失效只能 kill -9；
	// stopSignals 后第二次信号即按默认动作立即终结进程）。defer 的 stopSignals
	// 与 goroutine 内调用幂等共存（signal_stop map 删除 + cancel 均可重入，
	// 正常返回路径双调用无害）。挂点在 hs 装配后、Serve 前——监听已就绪，
	// 信号随时到达均走同一关停序列。
	sigCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	go func() {
		<-sigCtx.Done()
		stopSignals() // WR-01：恢复默认处置——关停期间第二次 SIGTERM/SIGINT 立即强杀
		srv.Shutdown()
	}()
	// D-26：--open 在启动打印完成之后、Serve 之前 goroutine 拉起浏览器（不阻塞
	// Serve——xdg-open/open 失败只警告，openBrowser 注释）；URL 与启动打印同一
	// 拼串单一事实源（上方 hoisted 局部变量）。--writable 开 rw 链接否则 ro
	// 链接（operator 视角入口，含 token 免交互，D-26）；--socket×--open 已被
	// validateStartup 拒绝，unix 分支下本 if 恒 false（shareURL 零值空串不消费）。
	if cfg.open {
		url := shareURLRO
		if cfg.writable {
			url = shareURLRW
		}
		go openBrowser(url)
	}
	// D-04/D-06：显式证书才 TLS（parseArgs 已保证成对）——TLSConfig 声明式下限
	//（MinVersion 1.2 + 显式 AEAD 清单，03-02 组件复用，无二手配置路径）；
	// 否则明文 Serve（D-03/D-05 矩阵已先行约束 + 上方 stderr 醒目警告）。
	if cfg.tlsCert != "" {
		hs.TLSConfig = server.TLSConfig()
		// G-03-5：复用上方预检加载的证书对——证书加载单一事实源，ServeTLS 不再
		// 二次读盘；stdlib 约定 TLSConfig.Certificates 非空时 certFile/keyFile
		// 传空串即可。
		hs.TLSConfig.Certificates = []tls.Certificate{cert}
		err = hs.ServeTLS(ln, "", "")
	} else {
		err = hs.Serve(ln)
	}
	if err != nil {
		// G-03-5 根因②：serve 失败路径与 net.Listen 失败路径（上方）逐字对称
		// 回滚——Close master 后 setsid 组长子进程收 SIGHUP 退出，不留孤儿进程。
		// 该路径无单测故障注入手段（Serve 阻塞语义 + lifecycle os.Exit 不可在
		// 单测驱动），以逐字对称 + 代码评审锁定（TestBadCertPreflight 注释同述）。
		_ = sess.Close()
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	return 0
}

// openBrowser 以系统启动器打开分享链接（07-05，OPS-11，D-26/D-27，RESEARCH
// Pattern 8 配方）：desktop（Linux 有 DISPLAY/WAYLAND_DISPLAY，或 darwin）→
// exec.Command(tool, url) Start——tool = linux "xdg-open" /
// darwin "open"；headless（linux 且 DISPLAY 与 WAYLAND_DISPLAY 均空）→ stderr
// 提示后跳过不阻断启动（headless 服务器是常态部署形态，--open 本质是桌面便利
// 功能，D-27）；启动失败仅 stderr 警告不阻断。URL 由 wesh 自构
// （scheme+host:port+base-path+自生成 token，run() 拼串单一事实源），
// exec.Command argv 分离不经 shell（T-07-05b 注入面结构性排除——goroutine
// Wait 改造后该不变量保持）。headless 检测只在 linux 分支判定，darwin 直接
// open——darwin 分支无本机运行时断言（构建标签差异；CI macOS 跑
// TestOpenBrowser 同款测试形态即整体 Skip，真实弹窗列 07-08 人工 UAT 清单）。
//
// Pattern 8 配方偏差登记（07-10 G-07-8 选项 A）：原配方 fire-and-forget
// .Start() 不等待——Start 成功后改起 goroutine Wait() 收割 opener 子进程
// （fire-and-forget 从不 Wait 会让 opener 退出后驻留僵尸至服务终结，每次
// --open 一个），且非零退出补 stderr 警告行；D-27「xdg-open 存在但返回非零
// （桌面异常）只警告不阻断」由仅覆盖启动失败延伸覆盖运行期非零退出——
// 「不阻断」是不变量，「须可见」是补齐（headless 跳过尚有提示行，桌面异常
// 非零退出不应反而静默）。警告行不含 URL（Wait err 仅 exit status N，结构性
// 无 argv——share token 红线 P5 D-03）。
func openBrowser(url string) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "wesh: --open: no display detected (headless), skipping browser launch")
		return
	}
	tool := "xdg-open"
	if runtime.GOOS == "darwin" {
		tool = "open"
	}
	cmd := exec.Command(tool, url)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "wesh: --open: failed to launch browser: %v\n", err) // 只警告不阻断（D-27）
		return
	}
	// Start 成功：goroutine Wait 收割（防僵尸）+ 非零退出补警告行（D-27 运行期
	// 覆盖）；异步执行——不阻断、零退出码影响、启动打印序列不变。
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "wesh: warning: --open: browser launcher exited with error: %v\n", err)
		}
	}()
}

func main() {
	os.Exit(run(os.Args[1:]))
}
