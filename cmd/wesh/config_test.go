package main

import (
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// writeToml 在 t.TempDir 落 TOML 配置文件并显式 Chmod 到目标权限
// （os.WriteFile 的 perm 受 umask 掩蔽——D-07 权限警告断言要求确定性权限位，
// 显式 Chmod 消除 umask 漂移）。
func writeToml(t *testing.T, content string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wesh.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatalf("chmod toml: %v", err)
	}
	return path
}

// TestLoadFileConfig（OPS-09，D-01/D-03/D-06/D-07）表驱动锁定 TOML 加载层契约：
// 合法全键加载（27 键 = 26 flag 同名 + command，指针标量区分「键缺席」与「零值」）；
// D-06 严格模式三族拒绝（文件不存在 / TOML 解析失败含类型不符 / 未知键）；
// 值剥离红线（SEC-01 启动面延伸，RESEARCH Pitfall 5）：错误文案只含类别+键名+行号，
// 凭据值探针串运行时自证零出现；D-07 权限警告（含 credential 键且权限非 600/400
// 时 warn 非空，警告串不含凭据值）。
func TestLoadFileConfig(t *testing.T) {
	t.Run("all 27 keys load", func(t *testing.T) {
		path := writeToml(t, `port = 9999
bind = "127.0.0.1"
writable = true
ping-interval = "30s"
write-policy = "all"
max-clients = 7
once = true
exit-when-empty = "30s"
credential = ["alice:pw1", "bob:pw2"]
origin = ["https://example.com"]
client-option = ["fontSize=16"]
tls-cert = "/tmp/c.pem"
tls-key = "/tmp/k.pem"
osc52 = true
socket = "/run/wesh.sock"
socket-mode = "0600"
socket-owner = "root:root"
base-path = "/wesh"
auth-header = "X-Remote-User"
cwd = "/tmp"
term = "vt100"
stop-signal = "TERM"
stop-timeout = "2s"
uid = 1000
gid = 1001
open = true
command = ["bash", "-l"]
`, 0o600)
		fc, warn, err := loadFileConfig(path)
		if err != nil {
			t.Fatalf("loadFileConfig: %v", err)
		}
		if warn != "" {
			t.Errorf("warn = %q, want empty (mode 600)", warn)
		}
		// 指针标量逐键断言非 nil 且值正确——nil 即「键缺席」与零值混淆，
		// 合并正确性依赖指针区分（值拷贝会吞掉显式零值）。
		if fc.Port == nil || *fc.Port != 9999 {
			t.Errorf("Port = %v, want 9999", fc.Port)
		}
		if fc.Bind == nil || *fc.Bind != "127.0.0.1" {
			t.Errorf("Bind = %v, want 127.0.0.1", fc.Bind)
		}
		if fc.Writable == nil || *fc.Writable != true {
			t.Errorf("Writable = %v, want true", fc.Writable)
		}
		if fc.PingInterval == nil || *fc.PingInterval != "30s" {
			t.Errorf("PingInterval = %v, want 30s", fc.PingInterval)
		}
		if fc.WritePolicy == nil || *fc.WritePolicy != "all" {
			t.Errorf("WritePolicy = %v, want all", fc.WritePolicy)
		}
		if fc.MaxClients == nil || *fc.MaxClients != 7 {
			t.Errorf("MaxClients = %v, want 7", fc.MaxClients)
		}
		if fc.Once == nil || *fc.Once != true {
			t.Errorf("Once = %v, want true", fc.Once)
		}
		if fc.ExitWhenEmpty == nil || *fc.ExitWhenEmpty != "30s" {
			t.Errorf("ExitWhenEmpty = %v, want 30s", fc.ExitWhenEmpty)
		}
		if len(fc.Credential) != 2 || fc.Credential[0] != "alice:pw1" || fc.Credential[1] != "bob:pw2" {
			t.Errorf("Credential = %v, want [alice:pw1 bob:pw2]", fc.Credential)
		}
		if len(fc.Origin) != 1 || fc.Origin[0] != "https://example.com" {
			t.Errorf("Origin = %v, want [https://example.com]", fc.Origin)
		}
		if len(fc.ClientOption) != 1 || fc.ClientOption[0] != "fontSize=16" {
			t.Errorf("ClientOption = %v, want [fontSize=16]", fc.ClientOption)
		}
		if fc.TLSCert == nil || *fc.TLSCert != "/tmp/c.pem" {
			t.Errorf("TLSCert = %v, want /tmp/c.pem", fc.TLSCert)
		}
		if fc.TLSKey == nil || *fc.TLSKey != "/tmp/k.pem" {
			t.Errorf("TLSKey = %v, want /tmp/k.pem", fc.TLSKey)
		}
		if fc.Osc52 == nil || *fc.Osc52 != true {
			t.Errorf("Osc52 = %v, want true", fc.Osc52)
		}
		if fc.Socket == nil || *fc.Socket != "/run/wesh.sock" {
			t.Errorf("Socket = %v, want /run/wesh.sock", fc.Socket)
		}
		if fc.SocketMode == nil || *fc.SocketMode != "0600" {
			t.Errorf("SocketMode = %v, want 0600", fc.SocketMode)
		}
		if fc.SocketOwner == nil || *fc.SocketOwner != "root:root" {
			t.Errorf("SocketOwner = %v, want root:root", fc.SocketOwner)
		}
		if fc.BasePath == nil || *fc.BasePath != "/wesh" {
			t.Errorf("BasePath = %v, want /wesh", fc.BasePath)
		}
		if fc.AuthHeader == nil || *fc.AuthHeader != "X-Remote-User" {
			t.Errorf("AuthHeader = %v, want X-Remote-User", fc.AuthHeader)
		}
		if fc.Cwd == nil || *fc.Cwd != "/tmp" {
			t.Errorf("Cwd = %v, want /tmp", fc.Cwd)
		}
		if fc.Term == nil || *fc.Term != "vt100" {
			t.Errorf("Term = %v, want vt100", fc.Term)
		}
		if fc.StopSignal == nil || *fc.StopSignal != "TERM" {
			t.Errorf("StopSignal = %v, want TERM", fc.StopSignal)
		}
		if fc.StopTimeout == nil || *fc.StopTimeout != "2s" {
			t.Errorf("StopTimeout = %v, want 2s", fc.StopTimeout)
		}
		if fc.Uid == nil || *fc.Uid != 1000 {
			t.Errorf("Uid = %v, want 1000", fc.Uid)
		}
		if fc.Gid == nil || *fc.Gid != 1001 {
			t.Errorf("Gid = %v, want 1001", fc.Gid)
		}
		if fc.Open == nil || *fc.Open != true {
			t.Errorf("Open = %v, want true", fc.Open)
		}
		if len(fc.Command) != 2 || fc.Command[0] != "bash" || fc.Command[1] != "-l" {
			t.Errorf("Command = %v, want [bash -l]", fc.Command)
		}
	})
	t.Run("absent keys stay nil", func(t *testing.T) {
		// 键缺席 = 指针 nil（内置默认档生效锚）；只给 port 时其余 26 键全 nil。
		path := writeToml(t, "port = 1\n", 0o600)
		fc, warn, err := loadFileConfig(path)
		if err != nil {
			t.Fatalf("loadFileConfig: %v", err)
		}
		if warn != "" {
			t.Errorf("warn = %q, want empty", warn)
		}
		if fc.Port == nil || *fc.Port != 1 {
			t.Errorf("Port = %v, want 1", fc.Port)
		}
		if fc.Bind != nil || fc.Writable != nil || fc.MaxClients != nil || fc.Once != nil ||
			fc.ExitWhenEmpty != nil || fc.Credential != nil || fc.Origin != nil ||
			fc.ClientOption != nil || fc.Osc52 != nil || fc.Socket != nil ||
			fc.SocketMode != nil || fc.SocketOwner != nil || fc.BasePath != nil ||
			fc.AuthHeader != nil || fc.Cwd != nil || fc.Term != nil ||
			fc.StopSignal != nil || fc.StopTimeout != nil || fc.Uid != nil ||
			fc.Gid != nil || fc.Open != nil || fc.Command != nil ||
			fc.TLSCert != nil || fc.TLSKey != nil || fc.PingInterval != nil || fc.WritePolicy != nil {
			t.Errorf("absent keys must stay nil, got %+v", fc)
		}
	})
	t.Run("explicit zero values decode", func(t *testing.T) {
		// 显式零值 ≠ 键缺席：port = 0 / writable = false 解码为非 nil 指针
		//（值拷贝会把显式零值吞成「缺席」，合并语义随之漂移）。
		path := writeToml(t, "port = 0\nwritable = false\nmax-clients = 0\n", 0o600)
		fc, _, err := loadFileConfig(path)
		if err != nil {
			t.Fatalf("loadFileConfig: %v", err)
		}
		if fc.Port == nil || *fc.Port != 0 {
			t.Errorf("Port = %v, want non-nil 0", fc.Port)
		}
		if fc.Writable == nil || *fc.Writable != false {
			t.Errorf("Writable = %v, want non-nil false", fc.Writable)
		}
		if fc.MaxClients == nil || *fc.MaxClients != 0 {
			t.Errorf("MaxClients = %v, want non-nil 0", fc.MaxClients)
		}
	})
	t.Run("file not found", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no-such.toml")
		_, _, err := loadFileConfig(missing)
		if err == nil {
			t.Fatal("loadFileConfig(missing) = nil error, want error (D-06)")
		}
		if !strings.Contains(err.Error(), "invalid config file") {
			t.Errorf("error = %q, want configErr wrapping", err)
		}
	})
	t.Run("unknown key rejected, value stripped", func(t *testing.T) {
		// D-06 严格模式：未知键拒绝；红线运行时自证——凭据值探针串
		// "s3cr3t-probe" 写入 TOML 后断言错误输出零出现（RESEARCH Pitfall 5：
		// go-toml 错误上下文可能回显源行，错误文案只含类别+键名）。
		path := writeToml(t, "credential = [\"alice:s3cr3t-probe\"]\nno-auth = true\n", 0o600)
		_, _, err := loadFileConfig(path)
		if err == nil {
			t.Fatal("loadFileConfig with unknown key = nil error, want error (D-06)")
		}
		if !strings.Contains(err.Error(), "unknown keys") {
			t.Errorf("error = %q, want unknown-keys category", err)
		}
		if !strings.Contains(err.Error(), "no-auth") {
			t.Errorf("error = %q, want unknown key name %q", err, "no-auth")
		}
		if strings.Contains(err.Error(), "s3cr3t-probe") {
			t.Errorf("error = %q, must not contain credential value probe", err)
		}
	})
	t.Run("escape-hatch keys rejected as unknown", func(t *testing.T) {
		// D-04 逃生门边界：五键写入配置文件即「未知键」拒绝（逃生门必须显式
		// 说出口——配置文件里写出来等于没说；fileConfig 无此五字段，
		// DisallowUnknownFields 自然拒绝）。
		path := writeToml(t, "no-auth = true\ninsecure-http = true\nversion = true\nhelp = true\nconfig = \"x\"\nport = 1\n", 0o600)
		_, _, err := loadFileConfig(path)
		if err == nil {
			t.Fatal("loadFileConfig with escape-hatch keys = nil error, want error")
		}
		for _, key := range []string{"no-auth", "insecure-http", "version", "help", "config"} {
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error = %q, want containing unknown key name %q", err, key)
			}
		}
		// 红线：未知键类别的错误文案不含值（true/x 均不得出现）。
		if strings.Contains(err.Error(), "true") {
			t.Errorf("error = %q, must not contain value %q", err, "true")
		}
	})
	t.Run("type mismatch rejected, value stripped", func(t *testing.T) {
		// TOML 类型不符 → error 只含键名与行号（go-toml DecodeError 行列
		// 上下文提取，禁含值）。
		path := writeToml(t, "port = \"zz-not-a-number-probe\"\n", 0o600)
		_, _, err := loadFileConfig(path)
		if err == nil {
			t.Fatal("loadFileConfig with port string = nil error, want type mismatch error")
		}
		if !strings.Contains(err.Error(), "port") {
			t.Errorf("error = %q, want containing key name %q", err, "port")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("error = %q, want containing line number", err)
		}
		if strings.Contains(err.Error(), "zz-not-a-number-probe") {
			t.Errorf("error = %q, must not contain value probe", err)
		}
	})
	t.Run("credential non-list rejected, value stripped", func(t *testing.T) {
		path := writeToml(t, "credential = \"s3cr3t-notalist-probe\"\n", 0o600)
		_, _, err := loadFileConfig(path)
		if err == nil {
			t.Fatal("loadFileConfig with scalar credential = nil error, want type mismatch error")
		}
		if !strings.Contains(err.Error(), "credential") {
			t.Errorf("error = %q, want containing key name %q", err, "credential")
		}
		if strings.Contains(err.Error(), "s3cr3t-notalist-probe") {
			t.Errorf("error = %q, must not contain value probe", err)
		}
	})
	t.Run("toml syntax error rejected", func(t *testing.T) {
		path := writeToml(t, "port = \n", 0o600)
		_, _, err := loadFileConfig(path)
		if err == nil {
			t.Fatal("loadFileConfig with syntax error = nil error, want error (D-06)")
		}
		if !strings.Contains(err.Error(), "invalid config file") {
			t.Errorf("error = %q, want configErr wrapping", err)
		}
	})
	t.Run("D-07 warns on group/world readable with credential", func(t *testing.T) {
		path := writeToml(t, "credential = [\"alice:s3cr3t-probe\"]\n", 0o644)
		_, warn, err := loadFileConfig(path)
		if err != nil {
			t.Fatalf("loadFileConfig: %v", err)
		}
		if warn == "" {
			t.Fatal("warn empty, want D-07 permission warning (credential + mode 644)")
		}
		if !strings.Contains(warn, "wesh: warning:") {
			t.Errorf("warn = %q, want wesh: warning: prefix", warn)
		}
		if !strings.Contains(warn, "0644") {
			t.Errorf("warn = %q, want containing mode 0644", warn)
		}
		if !strings.Contains(warn, "chmod 600") {
			t.Errorf("warn = %q, want chmod 600 recommendation", warn)
		}
		// 红线：警告串不含凭据值（D-07 不阻断但绝不泄露）。
		if strings.Contains(warn, "s3cr3t-probe") {
			t.Errorf("warn = %q, must not contain credential value probe", warn)
		}
	})
	t.Run("D-07 silent on 600 and 400", func(t *testing.T) {
		for _, perm := range []os.FileMode{0o600, 0o400} {
			path := writeToml(t, "credential = [\"alice:pw\"]\n", perm)
			_, warn, err := loadFileConfig(path)
			if err != nil {
				t.Fatalf("loadFileConfig(perm %o): %v", perm, err)
			}
			if warn != "" {
				t.Errorf("perm %o: warn = %q, want empty (600/400 安全档)", perm, warn)
			}
		}
	})
	t.Run("D-07 silent without credential key", func(t *testing.T) {
		path := writeToml(t, "port = 1\n", 0o644)
		_, warn, err := loadFileConfig(path)
		if err != nil {
			t.Fatalf("loadFileConfig: %v", err)
		}
		if warn != "" {
			t.Errorf("warn = %q, want empty (无 credential 键不触发 D-07)", warn)
		}
	})
}

// TestPrescanConfigPath（D-01 装配前提）：手工扫描 `--config=<v>` 与 `--config <v>`
// 两形态，last-wins；未给返回 ""；`--` 之后的 --config 属子命令 argv 不扫描；
// 扫描器不解析其他 flag 不报错。
func TestPrescanConfigPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"space form", []string{"--config", "/x.toml", "--", "bash"}, "/x.toml"},
		{"equals form", []string{"--config=/y.toml", "--", "bash"}, "/y.toml"},
		{"single dash space", []string{"-config", "/s.toml", "--", "bash"}, "/s.toml"},
		{"single dash equals", []string{"-config=/s2.toml", "--", "bash"}, "/s2.toml"},
		{"last wins", []string{"--config", "/a.toml", "--config=/b.toml", "--", "bash"}, "/b.toml"},
		{"absent", []string{"--port", "1", "--", "bash"}, ""},
		{"after dashdash ignored", []string{"--", "bash", "--config", "/z.toml"}, ""},
		{"config value of another flag untouched", []string{"--credential", "u:p", "--", "bash"}, ""},
		{"empty args", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := prescanConfigPath(tt.args); got != tt.want {
				t.Errorf("prescanConfigPath(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

// parseConfigArgs 测试夹具：writeToml + parseArgs(--config path + extra + trailing)。
// trailing 为 `--` 后 argv（nil = 不给 CLI argv——command 键用例的断言面）。
func parseConfigArgs(t *testing.T, tomlContent string, extra []string, trailing ...string) (config, []string, error) {
	t.Helper()
	path := writeToml(t, tomlContent, 0o600)
	args := []string{"--config", path}
	args = append(args, extra...)
	args = append(args, trailing...)
	return parseArgs(args)
}

// TestParseArgsWithConfig（OPS-09 Task 2 验收锚）——配置文件两阶段合并矩阵伞测试，
// 子测分组落三具名测试（must_have artifacts 锁名）：merge（D-02 列表替换与
// 标量/argv 合并）、precedence（D-05 flag > env > config > default 链）、
// redlines（D-04 逃生门键 + 值剥离红线 + D-07 警告通道 + 终值走同一校验）。
func TestParseArgsWithConfig(t *testing.T) {
	t.Run("merge", TestConfigMerge)
	t.Run("precedence", TestConfigPrecedence)
	t.Run("redlines", TestConfigRedLines)
}

// TestConfigMerge（D-02/D-04 + socket 族配置同档）：标量默认值替换机制
// （CLI 未给落配置值，CLI 给则覆盖）；列表 CLI 给出则整个替换、未给则配置列表
// 经各自 parse 期校验函数逐项解析；argv/command 覆盖与空数组缺席语义；
// exit-when-empty 字符串三形态（OQ4）；--once 展开覆盖配置 max-clients/
// exit-when-empty（flag > 配置直接推论）；write-policy×writable 与 socket 族
// 互斥/单给矩阵对配置来源值同档生效（fc.X 非 nil 即置对应显式位）。
func TestConfigMerge(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	t.Run("scalar from config only", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "port = 9999\nbind = \"127.0.0.1\"\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.port != 9999 {
			t.Errorf("port = %d, want 9999 (config 铺底)", cfg.port)
		}
		if cfg.bind != "127.0.0.1" {
			t.Errorf("bind = %q, want 127.0.0.1", cfg.bind)
		}
	})
	t.Run("scalar CLI overrides config", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "port = 9999\n", []string{"--port", "1234"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.port != 1234 {
			t.Errorf("port = %d, want 1234 (flag > config)", cfg.port)
		}
	})
	t.Run("explicit zero scalar from config", func(t *testing.T) {
		// port = 0（随机端口）经配置给出——指针标量保证显式零值不被吞成内置默认。
		cfg, _, err := parseConfigArgs(t, "port = 0\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.port != 0 {
			t.Errorf("port = %d, want 0 (显式零值 ≠ 键缺席)", cfg.port)
		}
	})
	t.Run("bool scalars from config", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "writable = true\nosc52 = true\nopen = true\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if !cfg.writable || !cfg.osc52 || !cfg.open {
			t.Errorf("writable/osc52/open = %v/%v/%v, want all true", cfg.writable, cfg.osc52, cfg.open)
		}
	})
	t.Run("duration scalars from config", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "ping-interval = \"30s\"\nstop-timeout = \"2s\"\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.pingInterval != 30*time.Second {
			t.Errorf("pingInterval = %v, want 30s", cfg.pingInterval)
		}
		if cfg.stopTimeout != 2*time.Second {
			t.Errorf("stopTimeout = %v, want 2s", cfg.stopTimeout)
		}
	})
	t.Run("string and enum scalars from config", func(t *testing.T) {
		me, uerr := user.Current()
		if uerr != nil {
			t.Fatalf("user.Current: %v", uerr)
		}
		cfg, argv, err := parseConfigArgs(t, "tls-cert = \"/tmp/c.pem\"\ntls-key = \"/tmp/k.pem\"\nbase-path = \"/wesh\"\nauth-header = \"X-Remote-User\"\ncwd = \"/tmp\"\nterm = \"vt100\"\nstop-signal = \"TERM\"\nuid = 1000\ngid = 1001\nsocket = \"/tmp/wesh.sock\"\nsocket-mode = \"0600\"\nsocket-owner = \""+me.Username+"\"\ncommand = [\"bash\"]\n", nil)
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.tlsCert != "/tmp/c.pem" || cfg.tlsKey != "/tmp/k.pem" {
			t.Errorf("tlsCert/tlsKey = %q/%q, want pair", cfg.tlsCert, cfg.tlsKey)
		}
		if cfg.basePath != "/wesh" {
			t.Errorf("basePath = %q, want /wesh", cfg.basePath)
		}
		if cfg.authHeader != "X-Remote-User" {
			t.Errorf("authHeader = %q, want X-Remote-User", cfg.authHeader)
		}
		if cfg.cwd != "/tmp" || cfg.term != "vt100" {
			t.Errorf("cwd/term = %q/%q, want /tmp vt100", cfg.cwd, cfg.term)
		}
		if cfg.stopSignal != "TERM" || cfg.stopSignalSig != syscall.SIGTERM {
			t.Errorf("stopSignal = %q sig %v, want TERM/SIGTERM（配置值走同一名→信号解析）", cfg.stopSignal, cfg.stopSignalSig)
		}
		if cfg.uid != 1000 || cfg.gid != 1001 {
			t.Errorf("uid/gid = %d/%d, want 1000/1001", cfg.uid, cfg.gid)
		}
		if cfg.socket != "/tmp/wesh.sock" || cfg.socketMode != 0o600 {
			t.Errorf("socket/socketMode = %q/%o, want /tmp/wesh.sock 600", cfg.socket, cfg.socketMode)
		}
		if cfg.socketUid != os.Getuid() || cfg.socketGid != os.Getgid() {
			t.Errorf("socketUid/socketGid = %d/%d, want self %d/%d（配置值走同一名字解析）", cfg.socketUid, cfg.socketGid, os.Getuid(), os.Getgid())
		}
		if !reflect.DeepEqual(argv, []string{"bash"}) {
			t.Errorf("argv = %v, want [bash]（command 键）", argv)
		}
	})
	t.Run("credential list replaced by CLI", func(t *testing.T) {
		// D-02：CLI 给出则替换整个列表——配置 2 组 + CLI 1 组 → 1。
		cfg, _, err := parseConfigArgs(t, "credential = [\"alice:pw1\", \"bob:pw2\"]\n", []string{"--credential", "carol:pw3"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if len(cfg.credentials) != 1 {
			t.Errorf("credentials count = %d, want 1 (CLI 替换整个列表)", len(cfg.credentials))
		}
	})
	t.Run("credential list from config", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "credential = [\"alice:pw1\", \"bob:pw2\"]\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if len(cfg.credentials) != 2 {
			t.Errorf("credentials count = %d, want 2 (配置列表逐项 ParseCredential)", len(cfg.credentials))
		}
	})
	t.Run("origin list from config and CLI replace", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "origin = [\"https://EXAMPLE.com:443\", \"http://foo.bar:8080\"]\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if !reflect.DeepEqual(cfg.origins, []string{"https://example.com", "http://foo.bar:8080"}) {
			t.Errorf("origins = %v, want normalized pair（配置列表逐项 NormalizeOrigin）", cfg.origins)
		}
		cfg2, _, err := parseConfigArgs(t, "origin = [\"https://example.com\", \"http://foo.bar:8080\"]\n", []string{"--origin", "https://cli.example"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if !reflect.DeepEqual(cfg2.origins, []string{"https://cli.example"}) {
			t.Errorf("origins = %v, want CLI 单组替换", cfg2.origins)
		}
	})
	t.Run("client-option list from config and CLI replace", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "client-option = [\"fontSize=16\", \"cursorBlink=false\"]\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if len(cfg.clientOptions) != 2 {
			t.Errorf("clientOptions count = %d, want 2（配置列表逐项白名单+JSON 校验）", len(cfg.clientOptions))
		}
		cfg2, _, err := parseConfigArgs(t, "client-option = [\"fontSize=16\", \"cursorBlink=false\"]\n", []string{"--client-option", "fontSize=20"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if len(cfg2.clientOptions) != 1 {
			t.Errorf("clientOptions count = %d, want 1 (CLI 替换)", len(cfg2.clientOptions))
		}
	})
	t.Run("command from config", func(t *testing.T) {
		_, argv, err := parseConfigArgs(t, "command = [\"bash\", \"-l\"]\n", nil)
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if !reflect.DeepEqual(argv, []string{"bash", "-l"}) {
			t.Errorf("argv = %v, want [bash -l]（D-04 command exec 数组）", argv)
		}
	})
	t.Run("command overridden by CLI argv", func(t *testing.T) {
		_, argv, err := parseConfigArgs(t, "command = [\"bash\", \"-l\"]\n", nil, "--", "ls")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if !reflect.DeepEqual(argv, []string{"ls"}) {
			t.Errorf("argv = %v, want [ls]（CLI `--` argv 非空则覆盖 command 键）", argv)
		}
	})
	t.Run("empty command array treated as absent", func(t *testing.T) {
		// flagged_assumption 锁定：command = [] 等价缺席（与 CLI `--` 空 argv 同档）。
		_, _, err := parseConfigArgs(t, "command = []\n", nil)
		if err == nil || !strings.Contains(err.Error(), "missing command") {
			t.Errorf("err = %v, want missing command（空数组按缺席处理）", err)
		}
	})
	t.Run("exit-when-empty config forms", func(t *testing.T) {
		// OQ4 字符串单形态：exitEmptyValue.Set 单一解析路径（true/0/30s 全通）。
		for _, row := range []struct {
			value string
			grace time.Duration
		}{
			{"true", 0},
			{"0", 0},
			{"30s", 30 * time.Second},
		} {
			cfg, _, err := parseConfigArgs(t, "exit-when-empty = \""+row.value+"\"\n", nil, "--", "bash")
			if err != nil {
				t.Fatalf("parseArgs(exit-when-empty=%q): %v", row.value, err)
			}
			if !cfg.exitEmpty.set || cfg.exitEmpty.grace != row.grace {
				t.Errorf("exit-when-empty=%q: set/grace = %v/%v, want true/%v", row.value, cfg.exitEmpty.set, cfg.exitEmpty.grace, row.grace)
			}
		}
	})
	t.Run("exit-when-empty CLI overrides config", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "exit-when-empty = \"30s\"\n", []string{"--exit-when-empty=10s"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if !cfg.exitEmpty.set || cfg.exitEmpty.grace != 10*time.Second {
			t.Errorf("exitEmpty = %v/%v, want true/10s (CLI 显式位锚定)", cfg.exitEmpty.set, cfg.exitEmpty.grace)
		}
	})
	t.Run("once overrides config max-clients", func(t *testing.T) {
		// 配置 max-clients 不算显式 → --once 展开覆盖配置值（flag > 配置推论）。
		cfg, _, err := parseConfigArgs(t, "max-clients = 5\n", []string{"--once"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.maxClients != 1 {
			t.Errorf("maxClients = %d, want 1 (--once 展开覆盖配置)", cfg.maxClients)
		}
	})
	t.Run("once overrides config exit-when-empty", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "exit-when-empty = \"30s\"\n", []string{"--once"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if !cfg.exitEmpty.set || cfg.exitEmpty.grace != 0 {
			t.Errorf("exitEmpty = %v/%v, want true/0 (--once 展开覆盖配置)", cfg.exitEmpty.set, cfg.exitEmpty.grace)
		}
	})
	t.Run("write-policy config-driven combo refusal", func(t *testing.T) {
		// 配置 write-policy 键非 nil 即「已给定」（writePolicySet 置位）——配置驱动
		// 的组合矛盾与 CLI 同档：write-policy × 无 writable → 拒绝且文案同 CLI 形态。
		cfg, _, err := parseConfigArgs(t, "write-policy = \"all\"\nbind = \"127.0.0.1\"\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		warn, verr := validateStartup(cfg)
		if verr == nil {
			t.Fatal("validateStartup = nil, want write-policy×writable 组合拒绝")
		}
		if !strings.Contains(verr.Error(), "--write-policy") || !strings.Contains(verr.Error(), "--writable") {
			t.Errorf("err = %q, want 双 flag 名（文案同 CLI 驱动）", verr)
		}
		if warn != "" {
			t.Errorf("warn = %q, want empty on refusal", warn)
		}
	})
	t.Run("write-policy config allowed with writable", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "write-policy = \"all\"\nwritable = true\nbind = \"127.0.0.1\"\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if warn, verr := validateStartup(cfg); verr != nil || warn != "" {
			t.Errorf("validateStartup = warn %q err %v, want clean", warn, verr)
		}
	})
	t.Run("socket and port both in config refused", func(t *testing.T) {
		// fc.Port 非 nil 即置 portSet（配置键存在 =「已给定」）——D-08 互斥矩阵对
		// 配置来源值同档生效，拒绝文案与 CLI 驱动逐字同。
		cfg, _, err := parseConfigArgs(t, "socket = \"/tmp/wesh.sock\"\nport = 9999\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		_, verr := validateStartup(cfg)
		if verr == nil {
			t.Fatal("validateStartup = nil, want D-08 互斥拒绝")
		}
		if !strings.Contains(verr.Error(), "--socket") || !strings.Contains(verr.Error(), "--port") {
			t.Errorf("err = %q, want 文案同 D-08（--socket × --port/--bind）", verr)
		}
	})
	t.Run("socket CLI with config port refused", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "port = 9999\n", []string{"--socket", "/tmp/wesh.sock"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		_, verr := validateStartup(cfg)
		if verr == nil {
			t.Fatal("validateStartup = nil, want D-08 互斥拒绝（CLI socket × 配置 port）")
		}
		if !strings.Contains(verr.Error(), "--socket") || !strings.Contains(verr.Error(), "--port") {
			t.Errorf("err = %q, want 文案同 D-08", verr)
		}
	})
	t.Run("socket-mode alone in config refused", func(t *testing.T) {
		// fc.SocketMode 非 nil 即置 socketModeSet——D-09 单给矛盾配置同档。
		cfg, _, err := parseConfigArgs(t, "socket-mode = \"0600\"\nbind = \"127.0.0.1\"\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		_, verr := validateStartup(cfg)
		if verr == nil {
			t.Fatal("validateStartup = nil, want D-09 单给矛盾拒绝")
		}
		if !strings.Contains(verr.Error(), "--socket-mode") || !strings.Contains(verr.Error(), "--socket") {
			t.Errorf("err = %q, want 文案同 D-09", verr)
		}
	})
	t.Run("socket family from config allowed", func(t *testing.T) {
		me, uerr := user.Current()
		if uerr != nil {
			t.Fatalf("user.Current: %v", uerr)
		}
		cfg, _, err := parseConfigArgs(t, "socket = \"/tmp/wesh.sock\"\nsocket-mode = \"0660\"\nsocket-owner = \""+me.Username+"\"\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if warn, verr := validateStartup(cfg); verr != nil || warn != "" {
			t.Errorf("validateStartup = warn %q err %v, want clean（D-11 socket 跳过矩阵）", warn, verr)
		}
	})
}

// TestConfigPrecedence（D-05 优先级链）：flag > env > 配置文件 > 内置默认。
// 标量两档由默认值替换机制天然成立；WESH_CREDENTIAL env 夹层保持在配置列表
// 应用之前（flag 列表 → env → 配置列表——env 非空则配置 credential 不应用）。
func TestConfigPrecedence(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	t.Run("flag over config", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "port = 9999\nwrite-policy = \"all\"\nwritable = true\n", []string{"--port", "1234", "--write-policy", "owner"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.port != 1234 || cfg.writePolicy != "owner" {
			t.Errorf("port/writePolicy = %d/%q, want 1234/owner (flag 最高优先)", cfg.port, cfg.writePolicy)
		}
	})
	t.Run("env over config credential", func(t *testing.T) {
		t.Setenv("WESH_CREDENTIAL", "env-user:env-pass")
		cfg, _, err := parseConfigArgs(t, "credential = [\"alice:pw1\", \"bob:pw2\"]\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if len(cfg.credentials) != 1 {
			t.Errorf("credentials count = %d, want 1（env 胜——配置 credential 不应用）", len(cfg.credentials))
		}
	})
	t.Run("flag over env over config", func(t *testing.T) {
		t.Setenv("WESH_CREDENTIAL", "env-user:env-pass")
		cfg, _, err := parseConfigArgs(t, "credential = [\"alice:pw1\", \"bob:pw2\"]\n", []string{"--credential", "carol:pw3"}, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if len(cfg.credentials) != 1 {
			t.Errorf("credentials count = %d, want 1（flag 胜 env 胜配置）", len(cfg.credentials))
		}
	})
	t.Run("config over builtin default", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "port = 9999\nmax-clients = 7\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.port != 9999 || cfg.maxClients != 7 {
			t.Errorf("port/maxClients = %d/%d, want 9999/7（配置 > 内置默认 7681/32）", cfg.port, cfg.maxClients)
		}
	})
	t.Run("builtin default when config key absent", func(t *testing.T) {
		cfg, _, err := parseConfigArgs(t, "bind = \"127.0.0.1\"\n", nil, "--", "bash")
		if err != nil {
			t.Fatalf("parseArgs: %v", err)
		}
		if cfg.port != 7681 || cfg.maxClients != 32 || cfg.pingInterval != 5*time.Second {
			t.Errorf("port/maxClients/pingInterval = %d/%d/%v, want 7681/32/5s（配置键缺席才落内置默认）", cfg.port, cfg.maxClients, cfg.pingInterval)
		}
	})
}

// TestConfigRedLines（D-04 逃生门边界 + SEC-01 启动面红线延伸 + D-07 通道 +
// 「合并后终值走同一校验」回归锁）：五逃生门键拒绝为未知键；配置来源的
// credential/client-option 校验错误走记录式（类别 + 键名，禁含值——探针串
// 运行时自证）；配置终值消费既有 parse 期校验段（stop-signal/socket-mode/
// uid/write-policy 零新代码路径）；D-07 警告经 parseArgs stderr 通道。
func TestConfigRedLines(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	t.Run("escape-hatch key rejected as unknown", func(t *testing.T) {
		// D-04：no-auth 写入 TOML 即「未知键」拒绝；文案为未知键类别不含 "true" 值。
		_, _, err := parseConfigArgs(t, "no-auth = true\n", nil, "--", "bash")
		if err == nil {
			t.Fatal("parseArgs = nil error, want 未知键拒绝（D-06 严格模式）")
		}
		if !strings.Contains(err.Error(), "unknown keys") || !strings.Contains(err.Error(), "no-auth") {
			t.Errorf("err = %q, want 未知键类别 + 键名", err)
		}
		if strings.Contains(err.Error(), "true") {
			t.Errorf("err = %q, must not contain value %q", err, "true")
		}
	})
	t.Run("invalid credential in config, value stripped", func(t *testing.T) {
		_, _, err := parseConfigArgs(t, "credential = [\"nocolon-probe\"]\n", nil, "--", "bash")
		if err == nil {
			t.Fatal("parseArgs = nil error, want credential 校验拒绝")
		}
		if !strings.Contains(err.Error(), "credential must be user:pass") {
			t.Errorf("err = %q, want 类别文案（credErr 同款记录式）", err)
		}
		if strings.Contains(err.Error(), "nocolon-probe") {
			t.Errorf("err = %q, must not contain credential 值探针", err)
		}
	})
	t.Run("invalid client-option entries in config, value stripped", func(t *testing.T) {
		for _, row := range []struct {
			name         string
			entry        string
			wantSub      string
			forbiddenSub string
		}{
			{"bad key", "allowProposedApi=secretprobe", "invalid client-option key", "secretprobe"},
			{"bad JSON", "fontSize=secretprobe", "not valid JSON", "secretprobe"},
			{"missing equals", "fontSize", "must be key=value", ""},
		} {
			t.Run(row.name, func(t *testing.T) {
				_, _, err := parseConfigArgs(t, "client-option = [\""+row.entry+"\"]\n", nil, "--", "bash")
				if err == nil {
					t.Fatal("parseArgs = nil error, want client-option 校验拒绝")
				}
				if !strings.Contains(err.Error(), row.wantSub) {
					t.Errorf("err = %q, want containing %q", err, row.wantSub)
				}
				if row.forbiddenSub != "" && strings.Contains(err.Error(), row.forbiddenSub) {
					t.Errorf("err = %q, must not contain 值探针 %q", err, row.forbiddenSub)
				}
			})
		}
	})
	t.Run("invalid origin in config rejected", func(t *testing.T) {
		_, _, err := parseConfigArgs(t, "origin = [\"https://*.example.com\"]\n", nil, "--", "bash")
		if err == nil {
			t.Fatal("parseArgs = nil error, want origin 校验拒绝")
		}
		if !strings.Contains(err.Error(), "glob") {
			t.Errorf("err = %q, want NormalizeOrigin 同款拒绝（值非敏感先例）", err)
		}
	})
	t.Run("invalid exit-when-empty duration in config, value stripped", func(t *testing.T) {
		_, _, err := parseConfigArgs(t, "exit-when-empty = \"zz-probe\"\n", nil, "--", "bash")
		if err == nil {
			t.Fatal("parseArgs = nil error, want duration 校验拒绝")
		}
		if !strings.Contains(err.Error(), "exit-when-empty") {
			t.Errorf("err = %q, want 键名 exit-when-empty", err)
		}
		if strings.Contains(err.Error(), "zz-probe") {
			t.Errorf("err = %q, must not contain 值探针", err)
		}
	})
	t.Run("exit-when-empty bool form rejected at load", func(t *testing.T) {
		// OQ4：bool 形态由 go-toml 类型不符自然拒绝（配置键字符串单形态）。
		_, _, err := parseConfigArgs(t, "exit-when-empty = true\n", nil, "--", "bash")
		if err == nil {
			t.Fatal("parseArgs = nil error, want 类型不符拒绝")
		}
		if !strings.Contains(err.Error(), "exit-when-empty") {
			t.Errorf("err = %q, want 键名 exit-when-empty", err)
		}
		if strings.Contains(err.Error(), "true") {
			t.Errorf("err = %q, must not contain value %q", err, "true")
		}
	})
	t.Run("invalid ping-interval duration in config, value stripped", func(t *testing.T) {
		_, _, err := parseConfigArgs(t, "ping-interval = \"zz-probe\"\n", nil, "--", "bash")
		if err == nil {
			t.Fatal("parseArgs = nil error, want duration 校验拒绝")
		}
		if !strings.Contains(err.Error(), "invalid duration") || !strings.Contains(err.Error(), "ping-interval") {
			t.Errorf("err = %q, want 类别 + 键名", err)
		}
		if strings.Contains(err.Error(), "zz-probe") {
			t.Errorf("err = %q, must not contain 值探针", err)
		}
	})
	t.Run("config values flow through existing parse validation", func(t *testing.T) {
		// 合并后终值消费既有 parse 期校验段（零新代码路径回归锁）——错误文案
		// 与 CLI 驱动同形态（既有渠道值非敏感可回显纪律不变）。
		for _, row := range []struct {
			name    string
			toml    string
			wantSub string
		}{
			{"stop-signal enum", "stop-signal = \"bogus\"\n", "invalid --stop-signal"},
			{"socket-mode octal", "socket = \"/tmp/x.sock\"\nsocket-mode = \"0888\"\n", "invalid --socket-mode"},
			{"uid range", "uid = -5\ngid = 1000\n", "invalid --uid"},
			{"write-policy enum", "write-policy = \"sometimes\"\n", "must be owner or all"},
			{"base-path strict", "base-path = \"wesh\"\n", "invalid --base-path"},
			{"tls pair", "tls-cert = \"/tmp/c.pem\"\n", "both --tls-cert and --tls-key"},
		} {
			t.Run(row.name, func(t *testing.T) {
				_, _, err := parseConfigArgs(t, row.toml, nil, "--", "bash")
				if err == nil {
					t.Fatalf("parseArgs = nil error, want %q", row.wantSub)
				}
				if !strings.Contains(err.Error(), row.wantSub) {
					t.Errorf("err = %q, want containing %q（CLI 驱动同文案）", err, row.wantSub)
				}
			})
		}
	})
	t.Run("D-07 warning printed via parseArgs stderr", func(t *testing.T) {
		path := writeToml(t, "credential = [\"alice:s3cr3t-probe\"]\nbind = \"127.0.0.1\"\n", 0o644)
		var perr error
		_, out := captureFd(t, &os.Stderr, func() int {
			_, _, perr = parseArgs([]string{"--config", path, "--", "bash"})
			return 0
		})
		if perr != nil {
			t.Fatalf("parseArgs: %v（D-07 警告放行不阻断）", perr)
		}
		if !strings.Contains(out, "wesh: warning:") || !strings.Contains(out, "0644") {
			t.Errorf("stderr = %q, want D-07 权限警告（wesh: warning: 前缀 + mode）", out)
		}
		if strings.Contains(out, "s3cr3t-probe") {
			t.Errorf("stderr = %q, must not contain credential 值探针", out)
		}
	})
	t.Run("config refusal exit 2 end-to-end", func(t *testing.T) {
		// run() 通道：未知键拒绝 exit 2（D-06 与启动校验矩阵同档）。
		path := writeToml(t, "no-auth = true\n", 0o600)
		code, out := captureFd(t, &os.Stderr, func() int {
			return run([]string{"--config", path, "--", "true"})
		})
		if code != 2 {
			t.Errorf("run = %d, want 2 (D-06 fail-fast)", code)
		}
		if !strings.Contains(out, "unknown keys") {
			t.Errorf("stderr = %q, want 未知键类别文案", out)
		}
	})
	t.Run("missing config file exit 2 end-to-end", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no-such.toml")
		code, out := captureFd(t, &os.Stderr, func() int {
			return run([]string{"--config", missing, "--", "true"})
		})
		if code != 2 {
			t.Errorf("run = %d, want 2 (D-06 文件不存在 fail-fast)", code)
		}
		if !strings.Contains(out, "invalid config file") {
			t.Errorf("stderr = %q, want configErr 包装文案", out)
		}
	})
}
