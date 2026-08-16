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
// D-16 --ping-interval 默认 5s，显式传值解析 Duration，0 = 禁用保活。
func TestParseArgs(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		wantBind         string
		wantPort         int
		wantWritable     bool
		wantPingInterval time.Duration
		wantArgv         []string
	}{
		{"defaults", []string{"--", "bash"}, "0.0.0.0", 7681, false, 5 * time.Second, []string{"bash"}},
		{"flags before dashdash", []string{"--port", "0", "--bind", "127.0.0.1", "--", "ls", "-la"}, "127.0.0.1", 0, false, 5 * time.Second, []string{"ls", "-la"}},
		{"subcommand flag passthrough", []string{"--port", "7682", "--", "/bin/echo", "--version"}, "0.0.0.0", 7682, false, 5 * time.Second, []string{"/bin/echo", "--version"}},
		{"writable flag", []string{"--writable", "--", "bash"}, "0.0.0.0", 7681, true, 5 * time.Second, []string{"bash"}},
		{"ping interval", []string{"--ping-interval", "30s", "--", "bash"}, "0.0.0.0", 7681, false, 30 * time.Second, []string{"bash"}},
		{"ping disabled", []string{"--ping-interval", "0", "--", "bash"}, "0.0.0.0", 7681, false, 0, []string{"bash"}},
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
			if !reflect.DeepEqual(argv, tt.wantArgv) {
				t.Errorf("argv = %v, want %v", argv, tt.wantArgv)
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
