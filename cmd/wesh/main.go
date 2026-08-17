// wesh — share terminal over web。
//
// CLI 形态：wesh [flags] -- <cmd> [args...]（D-02）；`--` 后原样以 exec 数组传递，
// 绝不经 shell。Phase 1 单次语义：WS 断开即退出（D-11），断线重连在 Phase 6。
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

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
	// Phase 3 认证与传输安全（one-way 公开契约，P2 D-15 同纪律）：
	credentials  []server.Credential // D-01：--credential 逐组收集 / WESH_CREDENTIAL env 兜底
	tlsCert      string              // D-04：--tls-cert，与 tlsKey 成对才启用 TLS
	tlsKey       string              // D-04：--tls-key
	noAuth       bool                // D-03：--no-auth 逃生门（无凭据监听非 loopback）
	insecureHTTP bool                // D-05：--insecure-http 逃生门（非 loopback 明文携凭据）
	origins      []string            // D-12：--origin 规范化后的允许列表
}

// parseArgs 解析 flags。全名无短选项（P2 D-15），共 11 个：
// Phase 1/2：--port/--bind/--version/--writable（D-15）/--ping-interval（D-16）；
// Phase 3：--credential（D-01 可重复）、--tls-cert/--tls-key（D-04 成对）、
// --no-auth（D-03 逃生门）、--insecure-http（D-05 逃生门）、--origin（D-12 可重复）。
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
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: wesh [flags] -- <cmd> [args...]\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return cfg, nil, err
	}
	if cfg.showVersion {
		return cfg, nil, nil
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
	sess, err := pty.Start(argv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.bind, cfg.port))
	if err != nil {
		// 启动失败路径回滚已 spawn 资源：Close master 后子进程（setsid 组长）
		// 收 SIGHUP 退出，不留孤儿进程。
		_ = sess.Close()
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	srv := server.New(sess, os.Exit, server.Options{Writable: cfg.writable, PingInterval: cfg.pingInterval})
	// D-07：启动仅打印单行（无 banner/emoji）；port 0 时 Addr 已是实际端口（D-06）。
	fmt.Printf("listening on http://%s\n", ln.Addr())
	// 显式 http.Server：ReadHeaderTimeout=5s 盒住预认证 HTTP 层慢 loris（与
	// helloTimeout 同 5s 量级，D-04）；ReadTimeout/WriteTimeout 不设——会误伤
	// WS 长连接语义（升级后的连接读写在握手后长期空闲/突发均属正常）。
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	if err := hs.Serve(ln); err != nil {
		fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:]))
}
