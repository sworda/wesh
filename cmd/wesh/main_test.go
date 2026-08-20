package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sworda/wesh/internal/server"
)

// TestParseArgs 表驱动锁定 CLI 解析契约：
// D-05/D-06 默认值（0.0.0.0:7681）；D-02 `--` 后参数（含以 `-` 开头者）原样进
// exec 数组不被 flag 包吞掉；子命令自身 flag 不被 wesh 解析；
// D-15 --writable 默认 false（默认只读），显式传旗标后为 true；
// D-16 --ping-interval 默认 5s，显式传值解析 Duration，0 = 禁用保活；
// Phase 3：D-01 --credential 可重复（计数断言——摘要不导出，计数即契约）、
// D-04 --tls-cert/--tls-key 成对装配、D-12 --origin parse 期规范化
// （小写 host + 剥默认端口）、D-03/D-05 两逃生门默认 false；
// Phase 4：P4 D-15 --client-option 可重复（计数断言）、P4 D-12 --osc52 默认 false；
// Phase 5：D-05 --write-policy 默认 owner，显式传 owner/all 原样解析。
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
		wantWritePolicy  string // D-05：零值 = 期望默认 owner
		wantCredentials  int    // D-01：凭据组数（不断言哈希值——Credential 字段不导出）
		wantTLSCert      string
		wantTLSKey       string
		wantNoAuth       bool
		wantInsecureHTTP bool
		wantOrigins      []string // D-12：断言规范化后形态
		// P4：照 wantCredentials 先例的计数断言（clientOption.value 是
		// json.RawMessage，计数即契约）；wantOSC52 直断言布尔。
		wantClientOptions int  // P4 D-15：--client-option 组数
		wantOSC52         bool // P4 D-12：--osc52 默认 false
		wantArgv          []string
	}{
		{name: "defaults", args: []string{"--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
		{name: "flags before dashdash", args: []string{"--port", "0", "--bind", "127.0.0.1", "--", "ls", "-la"}, wantBind: "127.0.0.1", wantPort: 0, wantPingInterval: 5 * time.Second, wantArgv: []string{"ls", "-la"}},
		// WR-01：IPv6 bind 值 parse 期原样接收（listen 侧由 net.JoinHostPort 加方括号）。
		{name: "ipv6 bind", args: []string{"--bind", "::1", "--", "bash"}, wantBind: "::1", wantPort: 7681, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
		{name: "subcommand flag passthrough", args: []string{"--port", "7682", "--", "/bin/echo", "--version"}, wantBind: "0.0.0.0", wantPort: 7682, wantPingInterval: 5 * time.Second, wantArgv: []string{"/bin/echo", "--version"}},
		{name: "writable flag", args: []string{"--writable", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantWritable: true, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
		{name: "ping interval", args: []string{"--ping-interval", "30s", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 30 * time.Second, wantArgv: []string{"bash"}},
		{name: "ping disabled", args: []string{"--ping-interval", "0", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 0, wantArgv: []string{"bash"}},
		{name: "two credentials", args: []string{"--credential", "alice:pw1", "--credential", "bob:pw2", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantCredentials: 2, wantArgv: []string{"bash"}},
		{name: "tls cert key pair", args: []string{"--tls-cert", "/tmp/cert.pem", "--tls-key", "/tmp/key.pem", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantTLSCert: "/tmp/cert.pem", wantTLSKey: "/tmp/key.pem", wantArgv: []string{"bash"}},
		{name: "origin normalized", args: []string{"--origin", "https://EXAMPLE.com:443", "--origin", "http://Foo.bar:8080", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantOrigins: []string{"https://example.com", "http://foo.bar:8080"}, wantArgv: []string{"bash"}},
		{name: "escape hatches", args: []string{"--no-auth", "--insecure-http", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantNoAuth: true, wantInsecureHTTP: true, wantArgv: []string{"bash"}},
		{name: "client options", args: []string{"--client-option", "fontSize=16", "--client-option", "cursorBlink=false", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantClientOptions: 2, wantArgv: []string{"bash"}},
		{name: "osc52 flag", args: []string{"--osc52", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantOSC52: true, wantArgv: []string{"bash"}},
		// D-05：--write-policy 显式传值原样解析（默认值由零值语义统一断言 = owner）。
		{name: "write policy all", args: []string{"--writable", "--write-policy", "all", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantWritable: true, wantPingInterval: 5 * time.Second, wantWritePolicy: "all", wantArgv: []string{"bash"}},
		{name: "write policy owner explicit", args: []string{"--writable", "--write-policy", "owner", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantWritable: true, wantPingInterval: 5 * time.Second, wantWritePolicy: "owner", wantArgv: []string{"bash"}},
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
			// D-05：零值 wantWritePolicy = 期望默认 owner（含全部既有行）。
			wantWritePolicy := tt.wantWritePolicy
			if wantWritePolicy == "" {
				wantWritePolicy = "owner"
			}
			if cfg.writePolicy != wantWritePolicy {
				t.Errorf("writePolicy = %q, want %q", cfg.writePolicy, wantWritePolicy)
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
			if len(cfg.clientOptions) != tt.wantClientOptions {
				t.Errorf("clientOptions count = %d, want %d", len(cfg.clientOptions), tt.wantClientOptions)
			}
			if cfg.osc52 != tt.wantOSC52 {
				t.Errorf("osc52 = %v, want %v", cfg.osc52, tt.wantOSC52)
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
// 回调内 parse 期报错（配置错误零窗口暴露）；--write-policy 非枚举值在 Parse
// 返回处报错（D-05，值非敏感直接 return error 形态）。
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
		{"malformed write-policy", []string{"--write-policy", "sometimes", "--", "bash"}, "must be owner or all"},
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

// TestClientOptionError（P4 D-14/D-15 + SEC-01 启动面红线）：--client-option 的
// 非白名单 key（含 osc52——D-12 安全不对称，安全敏感项只能经服务端 --osc52 开启）、
// 非法 JSON 值、缺 '=' 均在 parse 期报错（照 TestTLSKeyPairError 错误子串表形态）。
// 红线断言：err.Error() 只含 key 名与错误类别，不含值内容（值内容禁入错误串——
// "not valid JSON" 子串本身合法，禁入的是用户给的值）。missing-equals 行无值可禁
// （回显的 s 即 key 位），forbiddenSub 置空跳过。
func TestClientOptionError(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	tests := []struct {
		name         string
		args         []string
		wantSub      string
		forbiddenSub string // 值内容禁入错误串（SEC-01 启动面红线延伸）
	}{
		{"bad key", []string{"--client-option", "allowProposedApi=true", "--", "bash"}, "invalid --client-option key", "true"},
		{"bad JSON", []string{"--client-option", "fontSize=abc", "--", "bash"}, "not valid JSON", "abc"},
		{"osc52 key rejected (D-12)", []string{"--client-option", "osc52=true", "--", "bash"}, "invalid --client-option key", "true"},
		{"missing equals", []string{"--client-option", "fontSize", "--", "bash"}, "must be key=value", ""},
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
			if tt.forbiddenSub != "" && strings.Contains(err.Error(), tt.forbiddenSub) {
				t.Errorf("parseArgs(%v) error = %q, must not contain value content %q", tt.args, err, tt.forbiddenSub)
			}
		})
	}
}

// TestAggregateClientPrefs（P4 D-13 聚合语义 + 05-03 D-13 双档分化表）：零配置 →
// 两档均 nil（Welcome JSON 不出 prefs 键，旧前端零漂移）；keys + osc52=true →
// rw 档含各 key 值与 osc52:true，ro 档含各 key 值但永不含 osc52 键（D-13：旁观者
// 强制不下发，即使全局 --osc52 开启）；同 key 重复 → 两档均 last-wins。
// 断言经 json.Unmarshal 为 map 进行（不逐字节——map marshal 键序确定性不依赖
// 断言顺序）。各语义独立 subtest，回归可定位。
func TestAggregateClientPrefs(t *testing.T) {
	t.Run("zero config returns nil", func(t *testing.T) {
		ro, rw := aggregateClientPrefs(nil, false)
		if ro != nil || rw != nil {
			t.Errorf("aggregateClientPrefs(nil, false) = (%s, %s), want both nil (no prefs key on wire)", ro, rw)
		}
	})
	t.Run("keys and osc52 merged", func(t *testing.T) {
		opts := []clientOption{
			{key: "fontSize", value: json.RawMessage("18")},
			{key: "theme", value: json.RawMessage(`{"background":"#000"}`)},
		}
		ro, rw := aggregateClientPrefs(opts, true)
		var m map[string]json.RawMessage
		if err := json.Unmarshal(rw, &m); err != nil {
			t.Fatalf("aggregate rw result unmarshal: %v", err)
		}
		if len(m) != 3 {
			t.Errorf("rw prefs keys = %d, want 3 (fontSize, theme, osc52)", len(m))
		}
		if string(m["fontSize"]) != "18" {
			t.Errorf("rw fontSize = %s, want 18", m["fontSize"])
		}
		if string(m["theme"]) != `{"background":"#000"}` {
			t.Errorf("rw theme = %s, want %s", m["theme"], `{"background":"#000"}`)
		}
		if string(m["osc52"]) != "true" {
			t.Errorf("rw osc52 = %s, want true (D-12 并入)", m["osc52"])
		}
		// D-13 双档断言：ro 档（旁观者+降级递补者）永不含 osc52 键——即使全局
		// --osc52 开启（PITFALLS C5 对策另一半：OSC52 不劫持旁观者剪贴板）。
		var mRO map[string]json.RawMessage
		if err := json.Unmarshal(ro, &mRO); err != nil {
			t.Fatalf("aggregate ro result unmarshal: %v", err)
		}
		if len(mRO) != 2 {
			t.Errorf("ro prefs keys = %d, want 2 (fontSize, theme — osc52 强制缺席)", len(mRO))
		}
		if _, present := mRO["osc52"]; present {
			t.Errorf("ro prefs must not contain osc52 key (D-13), got %s", mRO["osc52"])
		}
		if string(mRO["fontSize"]) != "18" {
			t.Errorf("ro fontSize = %s, want 18", mRO["fontSize"])
		}
	})
	t.Run("same key last-wins", func(t *testing.T) {
		opts := []clientOption{
			{key: "fontSize", value: json.RawMessage("14")},
			{key: "fontSize", value: json.RawMessage("22")},
		}
		ro, rw := aggregateClientPrefs(opts, false)
		for _, blob := range []json.RawMessage{ro, rw} {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(blob, &m); err != nil {
				t.Fatalf("aggregate result unmarshal: %v", err)
			}
			if len(m) != 1 {
				t.Errorf("prefs keys = %d, want 1 (collapsed)", len(m))
			}
			if string(m["fontSize"]) != "22" {
				t.Errorf("fontSize = %s, want 22 (last-wins)", m["fontSize"])
			}
		}
	})
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

// TestStartupMatrix（D-03/D-05，RESEARCH Pattern 7 八行矩阵 + 05-03 D-05 组合
// 校验两行）：直调 validateStartup 纯函数全覆盖。wantErr 行断言与 RESEARCH 逐字
// 一致的拒绝文案；wantWarnSub 行断言警告非空且含对应逃生门 flag 名（D-03/D-05
// 显式确认语义）；全部行断言 warn/err 不含凭据值——SEC-01 日志红线延伸到启动面
// （启动输出任何形态不得泄露凭据）。05-03 新增：write-policy × writable 组合校验
// （配置矛盾 fail-fast——显式设置 write-policy 却未开 --writable 总闸即拒，与 bind
// 安全形态无关；默认 owner 未显式设置 + 无 --writable 是纯 ro 会话正常形态放行）。
func TestStartupMatrix(t *testing.T) {
	cred, err := server.ParseCredential("matrix-user:matrix-secret-7d1f")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	creds := []server.Credential{cred}
	tests := []struct {
		name        string
		cfg         config
		wantErrSub  string // 非空 = 拒绝启动，文案须含此子串
		wantErrSub2 string // 非空 = 拒绝文案须同时含此第二子串（组合校验双 flag 名断言）
		wantWarnSub string // 非空 = 放行但 stderr 醒目警告须含此子串（逃生门 flag 名）
	}{
		// loopback：流量不出机，有无凭据/TLS 均放行免警告（D-03/D-05 现状保持）。
		{"loopback no creds plaintext", config{bind: "127.0.0.1"}, "", "", ""},
		{"loopback creds plaintext", config{bind: "127.0.0.1", credentials: creds}, "", "", ""},
		{"loopback creds TLS", config{bind: "localhost", credentials: creds, tlsCert: "/tmp/c.pem", tlsKey: "/tmp/k.pem"}, "", "", ""},
		// WR-01：IPv6 loopback（::1）与 IPv4 loopback 同等待遇——无凭据明文放行免警告。
		{"loopback ipv6 no creds plaintext", config{bind: "::1"}, "", "", ""},
		// 非 loopback + 无凭据：拒绝（D-03 逐字文案），TLS 不救无凭据。
		{"non-loopback no creds refused", config{bind: "0.0.0.0"}, "refusing to listen on non-loopback address without credentials; pass --no-auth to disable authentication", "", ""},
		{"non-loopback no creds no-auth escape", config{bind: "0.0.0.0", noAuth: true}, "", "", "--no-auth"},
		// 非 loopback + 凭据 + 明文：拒绝（D-05 逐字文案）；逃生门放行 + 醒目警告。
		{"non-loopback creds plaintext refused", config{bind: "0.0.0.0", credentials: creds}, "refusing to serve credentials over plaintext HTTP on non-loopback address; pass --insecure-http or provide --tls-cert/--tls-key", "", ""},
		{"non-loopback creds plaintext insecure-http escape", config{bind: "0.0.0.0", credentials: creds, insecureHTTP: true}, "", "", "--insecure-http"},
		// 非 loopback + 凭据 + TLS：最强形态免警告。
		{"non-loopback creds TLS", config{bind: "0.0.0.0", credentials: creds, tlsCert: "/tmp/c.pem", tlsKey: "/tmp/k.pem"}, "", "", ""},
		// 05-03 D-05 组合校验：显式 write-policy 无 --writable → 拒绝（配置矛盾
		// fail-fast，文案须含双 flag 名；loopback 安全形态也不豁免——纯配置矛盾
		// 与 bind 无关）；默认 owner 未显式设置 + 无 --writable → 纯 ro 会话放行。
		{"explicit write-policy without writable refused", config{bind: "127.0.0.1", writePolicy: "all", writePolicySet: true}, "--write-policy", "--writable", ""},
		{"default owner without writable allowed", config{bind: "127.0.0.1", writePolicy: "owner"}, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warn, err := validateStartup(tt.cfg)
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			if tt.wantErrSub == "" && err != nil {
				t.Errorf("validateStartup = err %v, want nil", err)
			}
			if tt.wantErrSub != "" {
				if err == nil {
					t.Errorf("validateStartup = nil err, want containing %q", tt.wantErrSub)
				} else if !strings.Contains(errStr, tt.wantErrSub) {
					t.Errorf("err = %q, want containing %q", errStr, tt.wantErrSub)
				} else if tt.wantErrSub2 != "" && !strings.Contains(errStr, tt.wantErrSub2) {
					t.Errorf("err = %q, want also containing %q", errStr, tt.wantErrSub2)
				}
				if warn != "" {
					t.Errorf("refusal path warn = %q, want empty", warn)
				}
			}
			if tt.wantWarnSub == "" && warn != "" {
				t.Errorf("warn = %q, want empty", warn)
			}
			if tt.wantWarnSub != "" {
				if warn == "" {
					t.Errorf("warn empty, want containing escape-hatch flag %q", tt.wantWarnSub)
				} else if !strings.Contains(warn, tt.wantWarnSub) {
					t.Errorf("warn = %q, want containing %q", warn, tt.wantWarnSub)
				}
			}
			// 启动面红线：警告/拒绝文案任何形态不得含凭据值。
			if strings.Contains(warn, "matrix-secret-7d1f") || strings.Contains(errStr, "matrix-secret-7d1f") {
				t.Errorf("startup output leaks credential value: warn=%q err=%q", warn, errStr)
			}
		})
	}
}

// TestStartupRefusalNoResource（D-03 拒绝路径零资源占用）：默认 bind 0.0.0.0 无
// 凭据时 run 必须先于 pty.Start/net.Listen 返回非零 + stderr 含拒绝文案——正常
// 快速返回即证明未 spawn 未 listen（误启动监听会 hang 或经 lifecycle os.Exit，
// TestNoCommandError 同构纪律）。t.Setenv 隔离宿主 WESH_CREDENTIAL 兜底干扰。
func TestStartupRefusalNoResource(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	code, out := captureFd(t, &os.Stderr, func() int { return run([]string{"--", "true"}) })
	if code == 0 {
		t.Error("run(-- true) = 0, want non-zero (startup refusal)")
	}
	if !strings.Contains(out, "refusing to listen on non-loopback address without credentials") {
		t.Errorf("run(-- true) stderr = %q, want D-03 refusal text", out)
	}
}

// TestBadCertPreflight（G-03-5 根因①回归锁）：坏 --tls-cert/--tls-key 路径时
// run 必须在零资源占用阶段报错退出（exit 1）——先于 pty.Start/net.Listen/
// listening 打印三者，与 validateStartup「拒绝路径零资源占用」纪律一致
// （exit 1 档位：运行时 I/O 错误，非 validateStartup 的 exit 2 配置矩阵错误）。
// 两子场景共用：--bind 127.0.0.1 loopback 免 validateStartup 拦截、证书路径
// 指向不存在文件、命令 `-- true`；t.Setenv 清空 WESH_CREDENTIAL 隔离宿主环境。
// 两场景均即时返回即证明无 spawn/无 listen 挂起（TestStartupRefusalNoResource
// 同构纪律——误启动监听会 hang 或经 lifecycle os.Exit，正常返回即未占用）。
// G-03-5 根因②（serve 失败 sess.Close() 回滚）无单测故障注入手段（Serve
// 阻塞语义 + lifecycle os.Exit 不可在单测驱动），以与 listen 失败路径
// （main.go net.Listen 错误分支）逐字对称 + 代码评审锁定。
func TestBadCertPreflight(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	args := func(port string) []string {
		return []string{"--bind", "127.0.0.1", "--port", port,
			"--tls-cert", "/nonexistent/cert.pem", "--tls-key", "/nonexistent/key.pem",
			"--", "true"}
	}
	// 场景 A（free port，--port 0）：print-then-die 回归锁——stdout 不得含
	// "listening on"（坏证书必须先于 listening 打印报错）；stderr 含所给
	// cert 路径（错误文案红线：只含路径与 OS/tls 错误，SEC-01 启动面同纪律）。
	t.Run("free port no print-then-die", func(t *testing.T) {
		code, stdout := captureFd(t, &os.Stdout, func() int { return run(args("0")) })
		if code != 1 {
			t.Errorf("run = %d, want 1 (bad cert preflight refusal)", code)
		}
		if strings.Contains(stdout, "listening on") {
			t.Errorf("stdout = %q, must not contain %q (print-then-die, G-03-5)", stdout, "listening on")
		}
		code, stderr := captureFd(t, &os.Stderr, func() int { return run(args("0")) })
		if code != 1 {
			t.Errorf("run = %d, want 1 (bad cert preflight refusal)", code)
		}
		if !strings.Contains(stderr, "/nonexistent/cert.pem") {
			t.Errorf("stderr = %q, want containing cert path %q", stderr, "/nonexistent/cert.pem")
		}
	})
	// 场景 B（occupied port）：锁预检先于 net.Listen 的顺序——测试内占住
	// 127.0.0.1:0 取实际端口，--port 传该端口；run 仍须报证书错（stderr 含
	// cert 路径、不含 "already in use"）。若预检晚于 listen，此处先撞地址冲突。
	t.Run("occupied port preflight precedes listen", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen: %v", err)
		}
		defer func() { _ = ln.Close() }()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		code, stderr := captureFd(t, &os.Stderr, func() int { return run(args(port)) })
		if code != 1 {
			t.Errorf("run = %d, want 1 (bad cert preflight refusal)", code)
		}
		if !strings.Contains(stderr, "/nonexistent/cert.pem") {
			t.Errorf("stderr = %q, want containing cert path %q", stderr, "/nonexistent/cert.pem")
		}
		if strings.Contains(stderr, "already in use") {
			t.Errorf("stderr = %q, must not contain %q (preflight must precede net.Listen)", stderr, "already in use")
		}
	})
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
