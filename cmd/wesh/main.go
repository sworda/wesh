// wesh — share terminal over web。
//
// CLI 形态：wesh [flags] -- <cmd> [args...]（D-02）；`--` 后原样以 exec 数组传递，
// 绝不经 shell。Phase 1 单次语义：WS 断开即退出（D-11），断线重连在 Phase 6。
package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
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
}

// clientOption 是 --client-option 的 parse 期产物：key 已过白名单（P4 D-14），
// value 已是合法 JSON（P4 D-15 parse 期 fail-fast）。
type clientOption struct {
	key   string
	value json.RawMessage
}

// parseArgs 解析 flags。全名无短选项（P2 D-15），共 14 个：
// Phase 1/2：--port/--bind/--version/--writable（D-15）/--ping-interval（D-16）；
// Phase 3：--credential（D-01 可重复）、--tls-cert/--tls-key（D-04 成对）、
// --no-auth（D-03 逃生门）、--insecure-http（D-05 逃生门）、--origin（D-12 可重复）；
// Phase 4：--client-option（P4 D-15 可重复，白名单 + JSON parse 期校验）、
// --osc52（P4 D-12 OSC52 剪贴板写开关，默认关）；
// Phase 5：--write-policy（D-05，owner|all 默认 owner，parse 期枚举校验）。
// WESH_CREDENTIAL env 兜底单组凭据（D-01：flag 非空时 env 整体忽略，flag 优先）。
// `--` 后参数原样收集为 argv（D-02）；argv 为空（且非 --version/--help）
// 返回错误（D-03：无命令不起登录 shell）。
func parseArgs(args []string) (cfg config, argv []string, err error) {
	fs := flag.NewFlagSet("wesh", flag.ContinueOnError)
	fs.IntVar(&cfg.port, "port", 7681, "listen port (0 = random, actual port is printed)")
	fs.StringVar(&cfg.bind, "bind", "0.0.0.0", "listen address")
	fs.BoolVar(&cfg.showVersion, "version", false, "print version and exit")
	fs.BoolVar(&cfg.writable, "writable", false, "allow client input (default read-only)")
	fs.DurationVar(&cfg.pingInterval, "ping-interval", 5*time.Second, "WS ping interval (0 = disable)")
	// D-05：写权限策略（one-way 公开契约）。--writable 保持总闸（不给 = 全员只读，
	// 现状语义零漂移）；write-policy 仅在总闸开启时有意义（组合校验见
	// validateStartup）。parse 期枚举校验在 Parse 返回处（值非敏感，直接 return
	// error 即可——client-option 的记录式上报仅用于值含敏感内容的场景）。
	fs.StringVar(&cfg.writePolicy, "write-policy", server.WritePolicyOwner, "write policy when --writable is on: owner|all (default owner)")
	// D-01：可重复凭据 flag，fs.Func 回调内 parse 期校验（畸形值即时报错——
	// systemd 配置错误零窗口暴露）。Pitfall 8：help 必须提示 ps 可见性。
	fs.Func("credential", "basic auth credential user:pass (repeatable; value visible to local users via ps, prefer WESH_CREDENTIAL env in production)", func(s string) error {
		c, err := server.ParseCredential(s)
		if err != nil {
			return err
		}
		cfg.credentials = append(cfg.credentials, c)
		return nil
	})
	fs.StringVar(&cfg.tlsCert, "tls-cert", "", "TLS certificate file (must give both --tls-cert and --tls-key to enable TLS)")
	fs.StringVar(&cfg.tlsKey, "tls-key", "", "TLS private key file (must give both --tls-cert and --tls-key)")
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
	fs.BoolVar(&cfg.osc52, "osc52", false, "enable OSC52 clipboard write (write-only; default off)")
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
	})
	if cfg.showVersion {
		return cfg, nil, nil
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
	argv = fs.Args()
	if len(argv) == 0 {
		return cfg, nil, errors.New("missing command")
	}
	return cfg, argv, nil
}

// aggregateClientPrefs 把 --client-option 收集项与 --osc52 合成 prefs 双档 blob
//（05-03 D-13 + P5-6）：ro 档（旁观者 + 降级递补者）= 聚合结果剔除 osc52 键——
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

// validateStartup 是 D-03/D-05 启动校验矩阵（RESEARCH Pattern 7 八行）的纯函数
// 落地——无任何副作用（禁止 listen/spawn/写文件），必须先于 pty.Start/net.Listen
// 执行：拒绝路径零资源占用，测试不挂死（main_test.go captureFd 纪律）。
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
	if isLoopbackBind(cfg.bind) {
		return "", nil // loopback：流量不出机，有无凭据/TLS 均放行免警告（D-03/D-05）
	}
	if len(cfg.credentials) == 0 {
		if !cfg.noAuth {
			return "", errors.New("refusing to listen on non-loopback address without credentials; pass --no-auth to disable authentication") // D-03
		}
		return "wesh: warning: listening on non-loopback address with NO authentication (--no-auth); anyone who can reach this port gets a terminal", nil
	}
	if cfg.tlsCert == "" {
		if !cfg.insecureHTTP {
			return "", errors.New("refusing to serve credentials over plaintext HTTP on non-loopback address; pass --insecure-http or provide --tls-cert/--tls-key") // D-05
		}
		return "wesh: warning: serving credentials over plaintext HTTP on non-loopback address (--insecure-http); prefer --tls-cert/--tls-key or a TLS-terminating reverse proxy", nil
	}
	return "", nil // 非 loopback + 凭据 + TLS：最强形态免警告
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
	sess, err := pty.Start(argv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	// JoinHostPort 对 IPv4/主机名输出与 %s:%d 逐字相同，仅对 IPv6 字面量
	// 加方括号（--bind ::1 拼出 [::1]:7681）——WR-01：IPv6 行为零漂移修复。
	ln, err := net.Listen("tcp", net.JoinHostPort(cfg.bind, strconv.Itoa(cfg.port)))
	if err != nil {
		// 启动失败路径回滚已 spawn 资源：Close master 后子进程（setsid 组长）
		// 收 SIGHUP 退出，不留孤儿进程。
		_ = sess.Close()
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	prefsRO, prefsRW := aggregateClientPrefs(cfg.clientOptions, cfg.osc52)
	srv := server.New(sess, os.Exit, server.Options{Writable: cfg.writable, WritePolicy: cfg.writePolicy, PingInterval: cfg.pingInterval, Credentials: cfg.credentials, Origins: cfg.origins, TLS: cfg.tlsCert != "", ClientPrefsRO: prefsRO, ClientPrefsRW: prefsRW})
	// D-07：启动仅打印单行（无 banner/emoji）；port 0 时 Addr 已是实际端口（D-06）。
	// scheme 分支感知（D-04）：TLS 启用时打印 https://。
	scheme := "http"
	if cfg.tlsCert != "" {
		scheme = "https"
	}
	fmt.Printf("listening on %s://%s\n", scheme, ln.Addr())
	// 显式 http.Server：ReadHeaderTimeout=5s 盒住预认证 HTTP 层慢 loris（与
	// helloTimeout 同 5s 量级，D-04）；ReadTimeout/WriteTimeout 不设——会误伤
	// WS 长连接语义（升级后的连接读写在握手后长期空闲/突发均属正常）。
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
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

func main() {
	os.Exit(run(os.Args[1:]))
}
