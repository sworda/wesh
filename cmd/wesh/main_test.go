package main

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestParseArgs 表驱动锁定 CLI 解析契约：
// D-05/D-06 默认值（0.0.0.0:7681）；D-02 `--` 后参数（含以 `-` 开头者）原样进
// exec 数组不被 flag 包吞掉；子命令自身 flag 不被 wesh 解析；
// D-15 --writable 默认 false（默认只读），显式传旗标后为 true；
// D-16 --ping-interval 默认 5s，显式传值解析 Duration，0 = 禁用保活；
// Phase 3：D-01 --credential 可重复（计数断言——摘要不导出，计数即契约）、
// D-04 --tls-cert/--tls-key 成对装配、D-12 --origin parse 期规范化
// （小写 host + 剥默认端口）、D-03/D-05 两逃生门默认 false。
// 表头 t.Setenv 清空 WESH_CREDENTIAL：隔离宿主环境，防宿主已设该变量时
// D-01 env 兜底改变各行 credentials 计数（env 专属用例在 TestCredentialFlagEnv）。
func TestParseArgs(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	tests := []struct {
		name             string
		args             []string
		wantBind         string
		wantPort         int
		wantWritable     bool
		wantPingInterval time.Duration
		wantCredentials  int // D-01：凭据组数（不断言哈希值——Credential 字段不导出）
		wantTLSCert      string
		wantTLSKey       string
		wantNoAuth       bool
		wantInsecureHTTP bool
		wantOrigins      []string // D-12：断言规范化后形态
		wantArgv         []string
	}{
		{name: "defaults", args: []string{"--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
		{name: "flags before dashdash", args: []string{"--port", "0", "--bind", "127.0.0.1", "--", "ls", "-la"}, wantBind: "127.0.0.1", wantPort: 0, wantPingInterval: 5 * time.Second, wantArgv: []string{"ls", "-la"}},
		{name: "subcommand flag passthrough", args: []string{"--port", "7682", "--", "/bin/echo", "--version"}, wantBind: "0.0.0.0", wantPort: 7682, wantPingInterval: 5 * time.Second, wantArgv: []string{"/bin/echo", "--version"}},
		{name: "writable flag", args: []string{"--writable", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantWritable: true, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
		{name: "ping interval", args: []string{"--ping-interval", "30s", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 30 * time.Second, wantArgv: []string{"bash"}},
		{name: "ping disabled", args: []string{"--ping-interval", "0", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 0, wantArgv: []string{"bash"}},
		{name: "two credentials", args: []string{"--credential", "alice:pw1", "--credential", "bob:pw2", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantCredentials: 2, wantArgv: []string{"bash"}},
		{name: "tls cert key pair", args: []string{"--tls-cert", "/tmp/cert.pem", "--tls-key", "/tmp/key.pem", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantTLSCert: "/tmp/cert.pem", wantTLSKey: "/tmp/key.pem", wantArgv: []string{"bash"}},
		{name: "origin normalized", args: []string{"--origin", "https://EXAMPLE.com:443", "--origin", "http://Foo.bar:8080", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantOrigins: []string{"https://example.com", "http://foo.bar:8080"}, wantArgv: []string{"bash"}},
		{name: "escape hatches", args: []string{"--no-auth", "--insecure-http", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantNoAuth: true, wantInsecureHTTP: true, wantArgv: []string{"bash"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, argv, err := parseArgs(tt.args)
			if err != nil {
				t.Fatalf("parseArgs(%v): %v", tt.args, err)
			}
			if cfg.bind != tt.wantBind {
				t.Errorf("bind = %q, want %q", cfg.bind, tt.wantBind)
			}
			if cfg.port != tt.wantPort {
				t.Errorf("port = %d, want %d", cfg.port, tt.wantPort)
			}
			if cfg.writable != tt.wantWritable {
				t.Errorf("writable = %v, want %v", cfg.writable, tt.wantWritable)
			}
			if cfg.pingInterval != tt.wantPingInterval {
				t.Errorf("pingInterval = %v, want %v", cfg.pingInterval, tt.wantPingInterval)
			}
			if len(cfg.credentials) != tt.wantCredentials {
				t.Errorf("credentials count = %d, want %d", len(cfg.credentials), tt.wantCredentials)
			}
			if cfg.tlsCert != tt.wantTLSCert {
				t.Errorf("tlsCert = %q, want %q", cfg.tlsCert, tt.wantTLSCert)
			}
			if cfg.tlsKey != tt.wantTLSKey {
				t.Errorf("tlsKey = %q, want %q", cfg.tlsKey, tt.wantTLSKey)
			}
			if cfg.noAuth != tt.wantNoAuth {
				t.Errorf("noAuth = %v, want %v", cfg.noAuth, tt.wantNoAuth)
			}
			if cfg.insecureHTTP != tt.wantInsecureHTTP {
				t.Errorf("insecureHTTP = %v, want %v", cfg.insecureHTTP, tt.wantInsecureHTTP)
			}
			if !reflect.DeepEqual(cfg.origins, tt.wantOrigins) {
				t.Errorf("origins = %v, want %v", cfg.origins, tt.wantOrigins)
			}
			if !reflect.DeepEqual(argv, tt.wantArgv) {
				t.Errorf("argv = %v, want %v", argv, tt.wantArgv)
			}
		})
	}
}

// TestCredentialFlagEnv（D-01）：WESH_CREDENTIAL env 兜底单组凭据；flag 非空时
// env 整体忽略（flag 优先）；env 畸形值 parse 期报错且文案注明来源 env 名。
func TestCredentialFlagEnv(t *testing.T) {
	t.Run("env only", func(t *testing.T) {
		t.Setenv("WESH_CREDENTIAL", "env-user:env-pass")
		cfg, _, err := parseArgs([]string{"--", "bash"})
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if len(cfg.credentials) != 1 {
			t.Errorf("credentials count = %d, want 1 (env fallback)", len(cfg.credentials))
		}
	})
	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv("WESH_CREDENTIAL", "env-user:env-pass")
		cfg, _, err := parseArgs([]string{"--credential", "flag-user:flag-pass", "--", "bash"})
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		// flag 优先：env 被整体忽略——若实现错误地把 env 追加进来则计数为 2。
		if len(cfg.credentials) != 1 {
			t.Errorf("credentials count = %d, want 1 (flag wins, env ignored)", len(cfg.credentials))
		}
	})
	t.Run("malformed env rejected", func(t *testing.T) {
		t.Setenv("WESH_CREDENTIAL", "no-colon-here")
		_, _, err := parseArgs([]string{"--", "bash"})
		if err == nil {
			t.Fatal("parseArgs with malformed WESH_CREDENTIAL = nil error, want error")
		}
		if !strings.Contains(err.Error(), "WESH_CREDENTIAL") {
			t.Errorf("error = %q, want containing env source name %q", err, "WESH_CREDENTIAL")
		}
	})
}

// TestTLSKeyPairError（D-04 + parse 期校验纪律）：--tls-cert/--tls-key 单给报错
// （文案含 both 双旗名）；--credential 畸形与 --origin 含 glob 字符在 fs.Func
// 回调内 parse 期报错（配置错误零窗口暴露）。
func TestTLSKeyPairError(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	tests := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{"tls-cert without key", []string{"--tls-cert", "/tmp/c.pem", "--", "bash"}, "both --tls-cert and --tls-key"},
		{"tls-key without cert", []string{"--tls-key", "/tmp/k.pem", "--", "bash"}, "both --tls-cert and --tls-key"},
		{"malformed credential", []string{"--credential", "no-colon-here", "--", "bash"}, "credential must be user:pass"},
		{"origin glob rejected", []string{"--origin", "https://*.example.com", "--", "bash"}, "glob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseArgs(tt.args)
			if err == nil {
				t.Fatalf("parseArgs(%v) = nil error, want containing %q", tt.args, tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("parseArgs(%v) error = %q, want containing %q", tt.args, err, tt.wantSub)
			}
		})
	}
}

// captureFd 临时替换目标 *os.File（os.Stdout/os.Stderr）执行 f 并捕获其输出。
// 不并行使用（fd 替换是进程全局的）。
func captureFd(t *testing.T, target **os.File, f func() int) (code int, out string) {
	t.Helper()
	old := *target
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	*target = w
	code = f()
	_ = w.Close()
	*target = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	_ = r.Close()
	return code, buf.String()
}

// TestNoCommandError（D-03）：无 `-- <cmd>` 时 run 返回非零且 stderr 含 usage 行。
// 解析失败先于 net.Listen——若误启动监听，http.Serve 会阻塞使本测试挂死变红，
// 故正常返回即证明两条路径都未占用端口。
func TestNoCommandError(t *testing.T) {
	for _, args := range [][]string{{}, {"--port", "0"}} {
		code, out := captureFd(t, &os.Stderr, func() int { return run(args) })
		if code == 0 {
			t.Errorf("run(%v) = 0, want non-zero", args)
		}
		if !strings.Contains(out, "usage: wesh [flags] -- <cmd> [args...]") {
			t.Errorf("run(%v) stderr = %q, want usage line", args, out)
		}
	}
}

// TestVersionFlag：`--version` 返回 0 且 stdout 含 wesh 与版本字符串
// （version 为包内 var，发布构建注入，开发构建为 dev，不强制构建期注入）。
func TestVersionFlag(t *testing.T) {
	code, out := captureFd(t, &os.Stdout, func() int { return run([]string{"--version"}) })
	if code != 0 {
		t.Fatalf("run(--version) = %d, want 0", code)
	}
	if !strings.Contains(out, "wesh") || !strings.Contains(out, version) {
		t.Errorf("run(--version) stdout = %q, want containing %q and version %q", out, "wesh", version)
	}
}
