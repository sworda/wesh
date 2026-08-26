package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
