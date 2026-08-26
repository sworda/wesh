package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
// Phase 5：D-05 --write-policy 默认 owner，显式传 owner/all 原样解析；
// Phase 6：D-12 --once 语法糖展开（maxClients=1 + exit-when-empty set/grace 0）、
// D-14 --exit-when-empty 三形态（不写 = 不开启 / 裸写 = 立即退出 / =duration = 宽限）；
// Phase 7：D-13 --base-path parse 期规范化（合法原样 / 根 / 归一空串；非法形态
// 五族拒绝断言在 TestTLSKeyPairError 错误表——parse 期拒绝的既定归属）；
// D-08/D-09 --socket 三 flag（路径原样 / --socket-mode 默认 0660 与自定义八进制 /
// --socket-owner parse 期解析为 self 数字对；非法 mode/owner 拒绝断言同在错误表）；
// D-21 --cwd/--term（原样入 cfg；--term="" 空串值按未配置处理；--cwd stat 预检
// 归 TestStartupMatrix）。
// 表头 t.Setenv 清空 WESH_CREDENTIAL：隔离宿主环境，防宿主已设该变量时
// D-01 env 兜底改变各行 credentials 计数（env 专属用例在 TestCredentialFlagEnv）。
func TestParseArgs(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	// D-09 owner 解析断言材料：self 用户名与主组名（免 root；开发/CI 环境
	// /etc/passwd、/etc/group 完备——osusergo 纯 Go 解析同路径）。
	me, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	grp, err := user.LookupGroupId(me.Gid)
	if err != nil {
		t.Fatalf("user.LookupGroupId(%s): %v", me.Gid, err)
	}
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
		// P6：D-12/D-14 CLI 契约断言位；零值语义照 wantWritePolicy 先例
		//（wantMaxClients 零值 = 期望默认 32，wantOnce/wantExitEmptySet 零值 =
		// 期望 false——defaults 行与全部既有行经此扩展零值断言覆盖）。
		wantOnce           bool          // D-12：--once 默认 false
		wantMaxClients     int           // D-08/D-12：零值 = 期望默认 32
		wantExitEmptySet   bool          // D-14：exitEmpty.set（不写 = 不开启）
		wantExitEmptyGrace time.Duration // D-14：exitEmpty.grace（裸写/默认 = 0）
		// P7：D-13 --base-path parse 期规范化断言位（零值 = 期望空串未配置，
		// 既存行经此扩展零值断言覆盖——命名字段扩展纪律 03-04 先例）。
		wantBasePath string // D-13：--base-path 规范化后期望值（根 "/" 归一为空串）
		// P7：D-08/D-09 --socket 组断言位（零值语义照 wantBasePath/wantMaxClients
		// 先例：wantSocket 零值 = 期望空串 TCP 形态；wantSocketMode 零值 = 期望
		// 默认 0660；wantSocketOwnerSelf=false = 期望 uid/gid 为 -1 未给哨兵）。
		wantSocket          string      // D-08：--socket 路径原样
		wantSocketMode      os.FileMode // D-09：--socket-mode 八进制解析产物
		wantSocketOwnerSelf bool        // D-09：--socket-owner 解析为 self 数字对
		// P7：D-18 --auth-header 断言位（零值 = 期望空串未配置——信任闸关闭，
		// 既存行经此扩展零值断言覆盖，命名字段扩展纪律 03-04 先例）。
		wantAuthHeader string // D-18：--auth-header 头名原样入 cfg
		// P7：D-21 --cwd/--term 断言位（零值 = 期望空串未配置——cwd 继承服务端、
		// term 回落 xterm-256color 现状语义，既存行经此扩展零值断言覆盖，命名
		// 字段扩展纪律 03-04 先例）。
		wantCwd  string // D-21：--cwd 路径原样入 cfg
		wantTerm string // D-21：--term 原样入 cfg（空串值按未配置处理）
		// P7：D-22 --stop-signal/--stop-timeout 断言位（零值语义照 wantWritePolicy/
		// wantSocketMode 先例：wantStopSignal 零值 = 期望默认 "HUP"；
		// wantStopSignalSig 零值 = 期望 SIGHUP；wantStopTimeout 零值 = 期望 0
		// 不补 KILL 纯单信号现状）。
		wantStopSignal    string         // D-22：--stop-signal 枚举名原样入 cfg
		wantStopSignalSig syscall.Signal // D-22：parse 期名→信号解析产物
		wantStopTimeout   time.Duration  // D-22：--stop-timeout 原样入 cfg
		// P7：D-24 --uid/--gid 断言位。零值 = 期望 -1 未给哨兵（root uid/gid 0
		// 是合法值，不在本表零值断言面——0 与 -1 的区分由值域拒绝行
		//（TestTLSKeyPairError）与成对校验行（TestStartupMatrix）承载）。
		wantUid int // D-24：--uid 解析产物（默认 -1 不降权）
		wantGid int // D-24：--gid 解析产物（默认 -1 不降权）
		// P7：D-26 --open 断言位（零值 = 期望 false 默认不开——既存行经此扩展
		// 零值断言覆盖，命名字段扩展纪律 03-04 先例）。
		wantOpen bool // D-26：--open 默认 false
		wantArgv []string
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
		// D-12：--once 语法糖展开 ≡ --max-clients=1 --exit-when-empty=0（裸写形态；
		// grace 零值 0 由共享断言覆盖）。
		{name: "once sugar expands", args: []string{"--once", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantOnce: true, wantMaxClients: 1, wantExitEmptySet: true, wantArgv: []string{"bash"}},
		// D-14：裸写 = 最后一个客户端断开立即退出（set, grace 0），max-clients
		// 默认 32 不受影响（零值断言）。
		{name: "exit-when-empty bare", args: []string{"--exit-when-empty", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantExitEmptySet: true, wantArgv: []string{"bash"}},
		// D-14：=duration 形态 = 重连宽限。
		{name: "exit-when-empty grace", args: []string{"--exit-when-empty=30s", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantExitEmptySet: true, wantExitEmptyGrace: 30 * time.Second, wantArgv: []string{"bash"}},
		// D-13：--base-path 合法值原样接收（单级与多级形态）；根 "/" 归一为
		// 未配置（空串——「根视为未配置」是 D-13 显式裁决，非默认值兜底）。
		{name: "base-path subpath", args: []string{"--base-path", "/wesh", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantBasePath: "/wesh", wantArgv: []string{"bash"}},
		{name: "base-path nested", args: []string{"--base-path", "/a/b", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantBasePath: "/a/b", wantArgv: []string{"bash"}},
		{name: "base-path root normalized", args: []string{"--base-path", "/", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
		// D-08/D-09：--socket 三 flag——路径原样入 cfg（互斥/单给组合矛盾归
		// validateStartup，TestStartupMatrix 锁定）；--socket-mode 自定义八进制
		// 解析（默认 0660 由零值语义统一断言覆盖全部既存行）；--socket-owner
		// parse 期经 os/user.Lookup[/LookupGroup] 解析为数字对（self 免 root；
		// user:group 形态 gid 经 LookupGroup 覆盖——self 主组同值，分支仍被走到）。
		{name: "socket path", args: []string{"--socket", "/run/wesh.sock", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantSocket: "/run/wesh.sock", wantArgv: []string{"bash"}},
		{name: "socket-mode custom", args: []string{"--socket", "/run/wesh.sock", "--socket-mode", "0600", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantSocket: "/run/wesh.sock", wantSocketMode: 0o600, wantArgv: []string{"bash"}},
		{name: "socket owner self", args: []string{"--socket", "/tmp/wesh.sock", "--socket-owner", me.Username, "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantSocket: "/tmp/wesh.sock", wantSocketOwnerSelf: true, wantArgv: []string{"bash"}},
		{name: "socket owner self group", args: []string{"--socket", "/tmp/wesh.sock", "--socket-owner", me.Username + ":" + grp.Name, "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantSocket: "/tmp/wesh.sock", wantSocketOwnerSelf: true, wantArgv: []string{"bash"}},
		// D-18：--auth-header 头名原样入 cfg（无 parse 期校验——头名合法性由
		// HTTP 层 Header.Get 语义自然承载；暴露面组合警告归 validateStartup，
		// TestStartupMatrix D-16 行锁定）；默认值空串由零值语义统一断言。
		{name: "auth-header flag", args: []string{"--auth-header", "X-Remote-User", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantAuthHeader: "X-Remote-User", wantArgv: []string{"bash"}},
		// D-21：--cwd/--term 原样入 cfg（--cwd stat 预检归 validateStartup，
		// TestStartupMatrix 锁定）；--term="" 空串值按未配置处理（显式空 TERM
		// 会使终端能力丢失——空 = 默认 xterm-256color 现状语义）。
		{name: "cwd and term", args: []string{"--cwd", "/tmp", "--term", "vt100", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantCwd: "/tmp", wantTerm: "vt100", wantArgv: []string{"bash"}},
		{name: "term empty treated as unset", args: []string{"--term=", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantArgv: []string{"bash"}},
		// D-22：--stop-signal 四枚举逐一断言名→信号解析产物（默认 HUP+SIGHUP 由
		// 零值语义统一断言覆盖全部既存行）；--stop-timeout 正值原样解析（非法
		// 枚举名与负值 duration 的 parse 期拒绝在 TestTLSKeyPairError 错误表——
		// parse 期拒绝既定归属）。
		{name: "stop-signal HUP explicit", args: []string{"--stop-signal", "HUP", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantStopSignal: "HUP", wantStopSignalSig: syscall.SIGHUP, wantArgv: []string{"bash"}},
		{name: "stop-signal TERM", args: []string{"--stop-signal", "TERM", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantStopSignal: "TERM", wantStopSignalSig: syscall.SIGTERM, wantArgv: []string{"bash"}},
		{name: "stop-signal INT", args: []string{"--stop-signal", "INT", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantStopSignal: "INT", wantStopSignalSig: syscall.SIGINT, wantArgv: []string{"bash"}},
		{name: "stop-signal KILL", args: []string{"--stop-signal", "KILL", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantStopSignal: "KILL", wantStopSignalSig: syscall.SIGKILL, wantArgv: []string{"bash"}},
		{name: "stop-timeout grace", args: []string{"--stop-timeout", "2s", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantStopTimeout: 2 * time.Second, wantArgv: []string{"bash"}},
		// D-24：--uid/--gid 数字直通原样入 cfg（成对强制归 validateStartup
		// TestStartupMatrix 锁定；值域拒绝在 TestTLSKeyPairError——parse 期
		// 拒绝既定归属；默认 -1/-1 哨兵由零值语义统一断言覆盖全部既存行）。
		{name: "uid gid pair", args: []string{"--uid", "1000", "--gid", "1000", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantUid: 1000, wantGid: 1000, wantArgv: []string{"bash"}},
		// D-26：--open 布尔 flag 原样入 cfg（--socket×--open 组合矛盾归
		// validateStartup，TestStartupMatrix 锁定）；默认 false 由零值语义统一断言。
		{name: "open flag", args: []string{"--open", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantOpen: true, wantArgv: []string{"bash"}},
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
			if cfg.once != tt.wantOnce {
				t.Errorf("once = %v, want %v", cfg.once, tt.wantOnce)
			}
			// D-08/D-12：零值 wantMaxClients = 期望默认 32（wantWritePolicy 零值
			// 语义同款）；--once 行显式给 1 锁语法糖展开。
			wantMaxClients := tt.wantMaxClients
			if wantMaxClients == 0 {
				wantMaxClients = 32
			}
			if cfg.maxClients != wantMaxClients {
				t.Errorf("maxClients = %d, want %d", cfg.maxClients, wantMaxClients)
			}
			if cfg.exitEmpty.set != tt.wantExitEmptySet {
				t.Errorf("exitEmpty.set = %v, want %v", cfg.exitEmpty.set, tt.wantExitEmptySet)
			}
			if cfg.exitEmpty.grace != tt.wantExitEmptyGrace {
				t.Errorf("exitEmpty.grace = %v, want %v", cfg.exitEmpty.grace, tt.wantExitEmptyGrace)
			}
			// D-13：零值 wantBasePath = 期望空串未配置（含全部既存行）。
			if cfg.basePath != tt.wantBasePath {
				t.Errorf("basePath = %q, want %q", cfg.basePath, tt.wantBasePath)
			}
			// D-08：零值 wantSocket = 期望空串 TCP 形态（含全部既存行）。
			if cfg.socket != tt.wantSocket {
				t.Errorf("socket = %q, want %q", cfg.socket, tt.wantSocket)
			}
			// D-09：零值 wantSocketMode = 期望默认 0660（wantMaxClients 零值语义同款）。
			wantSocketMode := tt.wantSocketMode
			if wantSocketMode == 0 {
				wantSocketMode = 0o660
			}
			if cfg.socketMode != wantSocketMode {
				t.Errorf("socketMode = %o, want %o", cfg.socketMode, wantSocketMode)
			}
			// D-09：owner 未给 = -1/-1 哨兵（uid/gid 0 是 root 合法值，零值不可作
			// 未给标记——含全部既存行）；wantSocketOwnerSelf 行断言 parse 期名字
			// 解析为 self 数字对。
			if tt.wantSocketOwnerSelf {
				if cfg.socketUid != os.Getuid() || cfg.socketGid != os.Getgid() {
					t.Errorf("socketUid/socketGid = %d/%d, want self %d/%d", cfg.socketUid, cfg.socketGid, os.Getuid(), os.Getgid())
				}
			} else if cfg.socketUid != -1 || cfg.socketGid != -1 {
				t.Errorf("socketUid/socketGid = %d/%d, want -1/-1 sentinel (owner unset)", cfg.socketUid, cfg.socketGid)
			}
			// D-18：--auth-header 原样解析（零值 = 空串未配置，既存行零值断言覆盖）。
			if cfg.authHeader != tt.wantAuthHeader {
				t.Errorf("authHeader = %q, want %q", cfg.authHeader, tt.wantAuthHeader)
			}
			// D-21：--cwd/--term 原样解析（零值 = 空串未配置，既存行零值断言覆盖）。
			if cfg.cwd != tt.wantCwd {
				t.Errorf("cwd = %q, want %q", cfg.cwd, tt.wantCwd)
			}
			if cfg.term != tt.wantTerm {
				t.Errorf("term = %q, want %q", cfg.term, tt.wantTerm)
			}
			// D-22：零值 wantStopSignal = 期望默认 "HUP"、零值 wantStopSignalSig =
			// 期望 SIGHUP（wantWritePolicy/wantSocketMode 零值语义同款）；
			// wantStopTimeout 零值 = 期望 0（不补 KILL 纯单信号现状）。
			wantStopSignal := tt.wantStopSignal
			if wantStopSignal == "" {
				wantStopSignal = "HUP"
			}
			if cfg.stopSignal != wantStopSignal {
				t.Errorf("stopSignal = %q, want %q", cfg.stopSignal, wantStopSignal)
			}
			wantStopSignalSig := tt.wantStopSignalSig
			if wantStopSignalSig == 0 {
				wantStopSignalSig = syscall.SIGHUP
			}
			if cfg.stopSignalSig != wantStopSignalSig {
				t.Errorf("stopSignalSig = %v, want %v", cfg.stopSignalSig, wantStopSignalSig)
			}
			if cfg.stopTimeout != tt.wantStopTimeout {
				t.Errorf("stopTimeout = %v, want %v", cfg.stopTimeout, tt.wantStopTimeout)
			}
			// D-24：零值 wantUid/wantGid = 期望 -1 未给哨兵（root 0 不在本表断言面，
			// 见字段注释）。
			wantUid := tt.wantUid
			if wantUid == 0 {
				wantUid = -1
			}
			if cfg.uid != wantUid {
				t.Errorf("uid = %d, want %d", cfg.uid, wantUid)
			}
			wantGid := tt.wantGid
			if wantGid == 0 {
				wantGid = -1
			}
			if cfg.gid != wantGid {
				t.Errorf("gid = %d, want %d", cfg.gid, wantGid)
			}
			// D-26：--open 原样解析（零值 = false 默认不开，既存行零值断言覆盖）。
			if cfg.open != tt.wantOpen {
				t.Errorf("open = %v, want %v", cfg.open, tt.wantOpen)
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
// （文案含 both 双旗名）；--credential 畸形（fs.Func 回调内 parse 期校验、
// credErr 记录式于 Parse 返回处统一报错，WR-01）与 --origin 含 glob 字符
// （回调内即时报错）均为配置错误零窗口暴露；--write-policy 非枚举值在 Parse
// 返回处报错（D-05，值非敏感直接 return error 形态）；--exit-when-empty 非法/
// 负值 duration 报错（D-14，值非敏感——flag 包 invalid value %q 包装回显
// duration 值可接受，T-06-04a 论证登记在 main.go Set 注释）；
// Phase 7：D-13 --base-path 非法形态五族 parse 期拒绝（严格模式，绝不宽容
// 自动修正——输入与生效值分叉是配置漂移隐蔽源；值非敏感，错误文案可回显，
// exitEmptyValue.Set 同纪律）；D-09 --socket-mode 非法值（非八进制数字 / >0777
// 含特殊位）与 --socket-owner 未知用户/未知组 parse 期拒绝（错误文案只含错误
// 类别与 flag 名——用户名非敏感可回显，但不泄露系统细节之外信息）。
// WR-01 红线断言：malformed credential 行的 err 只含错误类别，不含 flag 值
// 内容（记录式上报杜绝 flag 包 invalid value %q 包装回显——TestClientOptionError
// forbiddenSub 同款先例；凭据值敏感度高于 prefs 值，同红线更须锁定）。
func TestTLSKeyPairError(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	tests := []struct {
		name         string
		args         []string
		wantSub      string
		forbiddenSub string // 值内容禁入错误串（SEC-01 启动面红线，WR-01；TestClientOptionError 先例）
	}{
		{"tls-cert without key", []string{"--tls-cert", "/tmp/c.pem", "--", "bash"}, "both --tls-cert and --tls-key", ""},
		{"tls-key without cert", []string{"--tls-key", "/tmp/k.pem", "--", "bash"}, "both --tls-cert and --tls-key", ""},
		{"malformed credential", []string{"--credential", "no-colon-here", "--", "bash"}, "credential must be user:pass", "no-colon-here"},
		{"origin glob rejected", []string{"--origin", "https://*.example.com", "--", "bash"}, "glob", ""},
		{"malformed write-policy", []string{"--write-policy", "sometimes", "--", "bash"}, "must be owner or all", ""},
		// D-14：非法/负值 duration parse 期报错（值非敏感——flag 包装串回显
		// duration 值可接受，T-06-04a；forbiddenSub 置空）。负值闸依赖 Set 的
		// d<0 检查（time.ParseDuration("-5s") 解析成功，负 duration 是合法语法）。
		{"exit-when-empty bad duration", []string{"--exit-when-empty=abc", "--", "bash"}, "exit-when-empty", ""},
		{"exit-when-empty negative duration", []string{"--exit-when-empty=-5s", "--", "bash"}, "exit-when-empty", ""},
		// D-13：--base-path 非法五族——无前导斜杠 / 尾斜杠 / 重复斜杠（头与
		// 中间两形态）/ ".." / 非 path 安全字符（空格/?/#/% 抽样）。
		{"base-path no leading slash", []string{"--base-path", "wesh", "--", "bash"}, "invalid --base-path", ""},
		{"base-path trailing slash", []string{"--base-path", "/wesh/", "--", "bash"}, "invalid --base-path", ""},
		{"base-path repeated slash head", []string{"--base-path", "//wesh", "--", "bash"}, "invalid --base-path", ""},
		{"base-path repeated slash inner", []string{"--base-path", "/w//esh", "--", "bash"}, "invalid --base-path", ""},
		{"base-path dotdot", []string{"--base-path", "/wesh/../x", "--", "bash"}, "invalid --base-path", ""},
		{"base-path space char", []string{"--base-path", "/we sh", "--", "bash"}, "invalid --base-path", ""},
		{"base-path query char", []string{"--base-path", "/wesh?x", "--", "bash"}, "invalid --base-path", ""},
		{"base-path fragment char", []string{"--base-path", "/wesh#x", "--", "bash"}, "invalid --base-path", ""},
		{"base-path percent char", []string{"--base-path", "/wesh%x", "--", "bash"}, "invalid --base-path", ""},
		// D-09：--socket-mode 非法两族（非八进制数字 / >0777 含特殊位——权限位
		// 是认证边界，不接纳 setuid/sticky 漂移面，T-07-02b）与 --socket-owner
		// 未知用户/未知组（拒绝串为固定类别文案，不含系统细节之外信息）。
		{"socket-mode non-octal", []string{"--socket", "/tmp/x.sock", "--socket-mode", "0888", "--", "bash"}, "invalid --socket-mode", ""},
		{"socket-mode special bits", []string{"--socket", "/tmp/x.sock", "--socket-mode", "1777", "--", "bash"}, "invalid --socket-mode", ""},
		{"socket-owner unknown user", []string{"--socket", "/tmp/x.sock", "--socket-owner", "wesh-no-such-user-7f3a", "--", "bash"}, "invalid --socket-owner", ""},
		{"socket-owner unknown group", []string{"--socket", "/tmp/x.sock", "--socket-owner", "root:wesh-no-such-group-7f3a", "--", "bash"}, "invalid --socket-owner", ""},
		// D-22：--stop-signal 非法枚举名两族（小写/未知名——文案列合法枚举）与
		// --stop-timeout 负值（DurationVar 直收下 "-5s" 解析成功，负值检查是
		// 唯一闸，exitEmptyValue.Set 负值闸同纪律；值非敏感可回显）。
		{"stop-signal lowercase rejected", []string{"--stop-signal", "term", "--", "bash"}, "invalid --stop-signal", ""},
		{"stop-signal unknown rejected", []string{"--stop-signal", "USR1", "--", "bash"}, "invalid --stop-signal", ""},
		{"stop-timeout negative rejected", []string{"--stop-timeout=-5s", "--", "bash"}, "invalid --stop-timeout", ""},
		// D-24：--uid/--gid 值域拒绝——-1 哨兵之外 < -1 或 > 4294967295 即拒
		//（uint32 转换安全；值非敏感可回显）。
		{"uid below range rejected", []string{"--uid", "-2", "--gid", "1000", "--", "bash"}, "invalid --uid", ""},
		{"uid above range rejected", []string{"--uid", "4294967296", "--gid", "1000", "--", "bash"}, "invalid --uid", ""},
		{"gid below range rejected", []string{"--uid", "1000", "--gid", "-2", "--", "bash"}, "invalid --gid", ""},
		{"gid above range rejected", []string{"--uid", "1000", "--gid", "4294967296", "--", "bash"}, "invalid --gid", ""},
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
// 校验两行 + 05-07 D-08 数值校验两行）：直调 validateStartup 纯函数全覆盖。
// wantErr 行断言与 RESEARCH 逐字
// 一致的拒绝文案；wantWarnSub 行断言警告非空且含对应逃生门 flag 名（D-03/D-05
// 显式确认语义）；全部行断言 warn/err 不含凭据值——SEC-01 日志红线延伸到启动面
// （启动输出任何形态不得泄露凭据）。05-03 新增：write-policy × writable 组合校验
// （配置矛盾 fail-fast——显式设置 write-policy 却未开 --writable 总闸即拒，与 bind
// 安全形态无关；默认 owner 未显式设置 + 无 --writable 是纯 ro 会话正常形态放行）。
// 05-07 新增：--max-clients ≤0 拒绝（D-08 数值校验——容量必须为正，0/负值会使
// ③位 503 闸恒触发全员被拒；纯配置有效性与 bind 无关，loopback 行也不豁免）。
// 既有行基线同步：validateStartup 新增 ≤0 拒绝后，既有行 config 零值 maxClients=0
// 会被误拒——全部既有行注入 maxClients: 32 基值（wantErr/wantWarn 断言语义不变）。
// 06-04 新增：--once × 显式矛盾值组合校验（D-12）——显式 --max-clients≠1 或显式
// --exit-when-empty grace≠0 与 --once 同给即拒（双 flag 名进文案，wantErrSub2
// 断言位；判定锚定显式设置位而非展开后终值，review #3）；--once + 显式
// --max-clients=1 / 显式裸 --exit-when-empty 为一致冗余放行两行为其行为锁。
// 07-02 新增：D-08/D-09/D-11 --socket 三行——互斥（--socket × 显式 --port/--bind
// 同给即拒，双 flag 名进文案；判定锚定显式设置位而非终值，--socket + 默认
// port/bind 不误判冲突）、单给矛盾（--socket-mode/--socket-owner 无 --socket
// 即拒）、D-11 跳过（unix 形态 bind 安全矩阵不可达——config 零值 bind 按非
// loopback 保守判定、无凭据本应收 D-03 拒绝，socket 给定时不拒不警告；文件
// 系统权限即认证边界，loopback 早退同款信任档位）。
// 07-03 新增：D-16 --auth-header 暴露面警告四行——触发（非 loopback + 无凭据
// + --no-auth + auth-header → 警告含 flag 名，文案不含头值）；D-03 拒绝不削弱
// （无 --no-auth 时 auth-header 照样拒）；不触发（loopback + auth-header；非
// loopback + 凭据 + TLS + auth-header）；socket 形态同 D-11 逻辑跳过本警告。
// 07-04 新增：D-21 --cwd stat 预检两行——不存在拒绝（fail-fast spawn 前零资源
// 占用，纯配置有效性 loopback 也不豁免）；真实目录放行（t.TempDir 材料）。
// 07-04 新增：D-24 --uid/--gid 成对强制三行——单给拒绝（双 flag 名进文案，
// 纯配置矛盾 loopback 不豁免）；成对给出放行。
func TestStartupMatrix(t *testing.T) {
	cred, err := server.ParseCredential("matrix-user:matrix-secret-7d1f")
	if err != nil {
		t.Fatalf("ParseCredential: %v", err)
	}
	creds := []server.Credential{cred}
	// 07-04 D-21：--cwd 合法放行行的运行时材料（t.TempDir 真实存在目录——stat
	// 预检放行分支需要一个真实目录路径）。
	cwdOK := t.TempDir()
	tests := []struct {
		name        string
		cfg         config
		wantErrSub  string // 非空 = 拒绝启动，文案须含此子串
		wantErrSub2 string // 非空 = 拒绝文案须同时含此第二子串（组合校验双 flag 名断言）
		wantWarnSub string // 非空 = 放行但 stderr 醒目警告须含此子串（逃生门 flag 名）
	}{
		// loopback：流量不出机，有无凭据/TLS 均放行免警告（D-03/D-05 现状保持）。
		{"loopback no creds plaintext", config{bind: "127.0.0.1", maxClients: 32}, "", "", ""},
		{"loopback creds plaintext", config{bind: "127.0.0.1", maxClients: 32, credentials: creds}, "", "", ""},
		{"loopback creds TLS", config{bind: "localhost", maxClients: 32, credentials: creds, tlsCert: "/tmp/c.pem", tlsKey: "/tmp/k.pem"}, "", "", ""},
		// WR-01：IPv6 loopback（::1）与 IPv4 loopback 同等待遇——无凭据明文放行免警告。
		{"loopback ipv6 no creds plaintext", config{bind: "::1", maxClients: 32}, "", "", ""},
		// 非 loopback + 无凭据：拒绝（D-03 逐字文案），TLS 不救无凭据。
		{"non-loopback no creds refused", config{bind: "0.0.0.0", maxClients: 32}, "refusing to listen on non-loopback address without credentials; pass --no-auth to disable authentication", "", ""},
		{"non-loopback no creds no-auth escape", config{bind: "0.0.0.0", maxClients: 32, noAuth: true}, "", "", "--no-auth"},
		// 非 loopback + 凭据 + 明文：拒绝（D-05 逐字文案）；逃生门放行 + 醒目警告。
		{"non-loopback creds plaintext refused", config{bind: "0.0.0.0", maxClients: 32, credentials: creds}, "refusing to serve credentials over plaintext HTTP on non-loopback address; pass --insecure-http or provide --tls-cert/--tls-key", "", ""},
		{"non-loopback creds plaintext insecure-http escape", config{bind: "0.0.0.0", maxClients: 32, credentials: creds, insecureHTTP: true}, "", "", "--insecure-http"},
		// 非 loopback + 凭据 + TLS：最强形态免警告。
		{"non-loopback creds TLS", config{bind: "0.0.0.0", maxClients: 32, credentials: creds, tlsCert: "/tmp/c.pem", tlsKey: "/tmp/k.pem"}, "", "", ""},
		// 05-03 D-05 组合校验：显式 write-policy 无 --writable → 拒绝（配置矛盾
		// fail-fast，文案须含双 flag 名；loopback 安全形态也不豁免——纯配置矛盾
		// 与 bind 无关）；默认 owner 未显式设置 + 无 --writable → 纯 ro 会话放行。
		{"explicit write-policy without writable refused", config{bind: "127.0.0.1", maxClients: 32, writePolicy: "all", writePolicySet: true}, "--write-policy", "--writable", ""},
		{"default owner without writable allowed", config{bind: "127.0.0.1", maxClients: 32, writePolicy: "owner"}, "", "", ""},
		// 05-07 D-08 数值校验：--max-clients ≤0 → 拒绝（容量必须为正；bind
		// 127.0.0.1 隔离其他校验维度——纯配置有效性 loopback 也不豁免）。
		{"max-clients zero refused", config{bind: "127.0.0.1", maxClients: 0}, "--max-clients", "", ""},
		{"max-clients negative refused", config{bind: "127.0.0.1", maxClients: -1}, "--max-clients", "", ""},
		// 06-04 D-12 组合校验：--once × 显式矛盾值 → 拒绝（双 flag 名进文案）；
		// --once × 显式一致值 = 一致冗余放行。bind 127.0.0.1 隔离其他校验维度
		//（max-clients 拒绝行同款）；放行行注入 maxClients: 32 基值避开 ≤0 维度。
		{"once with explicit max-clients=2 refused", config{bind: "127.0.0.1", once: true, maxClients: 2, maxClientsSet: true}, "--once", "--max-clients", ""},
		{"once with explicit exit-when-empty grace refused", config{bind: "127.0.0.1", once: true, maxClients: 32, exitEmptySet: true, exitEmpty: exitEmptyValue{set: true, grace: 5 * time.Second}}, "--once", "--exit-when-empty", ""},
		{"once with explicit max-clients=1 allowed", config{bind: "127.0.0.1", once: true, maxClients: 1, maxClientsSet: true}, "", "", ""},
		{"once with explicit bare exit-when-empty allowed", config{bind: "127.0.0.1", once: true, maxClients: 32, exitEmptySet: true, exitEmpty: exitEmptyValue{set: true}}, "", "", ""},
		// 07-02 D-08 互斥：--socket × 显式 --port/--bind 同给即拒（双 flag 名进
		// 文案；显式位锚定——下方 D-11 跳过行的 socket + 默认 port/bind 不误判
		// 即其行为锁）。
		{"socket with explicit port refused", config{socket: "/run/wesh.sock", maxClients: 32, port: 7682, portSet: true}, "--socket", "--port", ""},
		{"socket with explicit bind refused", config{socket: "/run/wesh.sock", maxClients: 32, bind: "127.0.0.1", bindSet: true}, "--socket", "--bind", ""},
		// 07-02 D-09 单给矛盾：--socket-mode/--socket-owner 仅随 --socket 有意义，
		// 单独给出 = 配置矛盾（bind 127.0.0.1 隔离其他校验维度）。
		{"socket-mode without socket refused", config{bind: "127.0.0.1", maxClients: 32, socketModeSet: true}, "--socket-mode", "--socket", ""},
		{"socket-owner without socket refused", config{bind: "127.0.0.1", maxClients: 32, socketOwnerSet: true}, "--socket-owner", "--socket", ""},
		// 07-02 D-11 跳过：unix 形态下 bind 安全矩阵不可达——config 零值 bind ""
		// 按非 loopback 保守判定，无凭据本应收 D-03 拒绝；socket 给定时不拒不警告。
		{"socket skips bind matrix (D-11)", config{socket: "/run/wesh.sock", maxClients: 32}, "", "", ""},
		// 07-03 D-16 暴露面警告：--auth-header 非空 + bind 非 loopback + 无凭据
		//（--no-auth 放行形态）→ wesh: warning: 前缀警告含 --auth-header flag 名
		//（文案不含任何头值）；无 --no-auth 时 D-03 拒绝照旧（auth-header 不削弱
		// 拒绝语义）；loopback + auth-header 与 非 loopback + 凭据 + TLS +
		// auth-header 不触发（矩阵其余行语义不变）；socket 形态 bind 矩阵已跳过，
		// 同行跳过本警告（unix socket 信任边界同 D-11 逻辑）。
		{"auth-header non-loopback no creds warns (D-16)", config{bind: "0.0.0.0", maxClients: 32, noAuth: true, authHeader: "X-Remote-User"}, "", "", "--auth-header"},
		{"auth-header does not bypass D-03 refusal", config{bind: "0.0.0.0", maxClients: 32, authHeader: "X-Remote-User"}, "refusing to listen on non-loopback address without credentials", "", ""},
		{"auth-header loopback silent", config{bind: "127.0.0.1", maxClients: 32, authHeader: "X-Remote-User"}, "", "", ""},
		{"auth-header non-loopback creds TLS silent", config{bind: "0.0.0.0", maxClients: 32, credentials: creds, tlsCert: "/tmp/c.pem", tlsKey: "/tmp/k.pem", authHeader: "X-Remote-User"}, "", "", ""},
		{"socket skips auth-header warning (D-16/D-11)", config{socket: "/run/wesh.sock", maxClients: 32, authHeader: "X-Remote-User"}, "", "", ""},
		// 07-04 D-21 预检：--cwd 非空且 stat 失败（不存在）即拒（fail-fast spawn
		// 前零资源占用——spawn 后才发现 ENOENT 是资源已占用且错误面到客户端；
		// 值非敏感可回显路径）。纯配置有效性与 bind 安全形态无关，loopback 早退
		// 之前判定（write-policy 行同位）；合法目录放行（bind 127.0.0.1 隔离
		// 其他校验维度）。
		{"cwd nonexistent refused", config{bind: "127.0.0.1", maxClients: 32, cwd: "/nonexistent-wesh-07-04/x"}, "--cwd", "", ""},
		{"cwd existing allowed", config{bind: "127.0.0.1", maxClients: 32, cwd: cwdOK}, "", "", ""},
		// 07-04 D-24 成对强制：--uid/--gid 只给一个 = 配置矛盾零窗口暴露（降权
		// 半配置静默放行 = 子进程以原权运行，T-07-04b；exit 2 而非降级运行），
		// 双 flag 名进文案；纯配置矛盾 loopback 也不豁免（write-policy 行同位）。
		// 成对给出放行（bind 127.0.0.1 隔离其他校验维度）。
		{"uid without gid refused", config{bind: "127.0.0.1", maxClients: 32, uid: 1000, gid: -1}, "--uid", "--gid", ""},
		{"gid without uid refused", config{bind: "127.0.0.1", maxClients: 32, uid: -1, gid: 1000}, "--uid", "--gid", ""},
		{"uid gid pair allowed", config{bind: "127.0.0.1", maxClients: 32, uid: 1000, gid: 1000}, "", "", ""},
		// 07-05 D-26/OQ1 组合校验：--socket × --open 同给即拒（unix socket 无
		// host:port 可拼——D-12 分享链接已退化为提示行，--open 需要 http(s) URL；
		// 给了无法兑现的 flag 组合 = 配置错误，「显式」哲学一贯性，双 flag 名进
		// 文案；纯配置矛盾在 D-11 socket 早退之前判定，socket 形态不豁免）。
		// --open 单独给（TCP 形态）放行（bind 127.0.0.1 隔离其他校验维度）。
		{"socket with open refused", config{socket: "/run/wesh.sock", maxClients: 32, open: true}, "--open", "--socket", ""},
		{"open without socket allowed", config{bind: "127.0.0.1", maxClients: 32, open: true}, "", "", ""},
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
// 07-02 同款纪律覆盖 D-08/D-09 新拒绝路径：--socket × 显式 --port 组合矛盾与
// --socket-mode 单给矛盾均 validateStartup exit 2（零 listen 零 spawn——socket
// 路径指向不存在目录，误 listen 会 exit 1 而非 2，退出码档位区分即证据）。
func TestStartupRefusalNoResource(t *testing.T) {
	t.Setenv("WESH_CREDENTIAL", "")
	t.Run("non-loopback no creds", func(t *testing.T) {
		code, out := captureFd(t, &os.Stderr, func() int { return run([]string{"--", "true"}) })
		if code == 0 {
			t.Error("run(-- true) = 0, want non-zero (startup refusal)")
		}
		if !strings.Contains(out, "refusing to listen on non-loopback address without credentials") {
			t.Errorf("run(-- true) stderr = %q, want D-03 refusal text", out)
		}
	})
	// D-08：--socket × 显式 --port 组合矛盾 exit 2（双 flag 名进文案）。
	t.Run("socket with explicit port", func(t *testing.T) {
		code, out := captureFd(t, &os.Stderr, func() int {
			return run([]string{"--socket", "/nonexistent-dir-7f3a/wesh.sock", "--port", "7682", "--", "true"})
		})
		if code != 2 {
			t.Errorf("run(--socket ... --port 7682) = %d, want 2 (validateStartup refusal, zero listen/spawn)", code)
		}
		if !strings.Contains(out, "--socket") || !strings.Contains(out, "--port") {
			t.Errorf("stderr = %q, want containing both --socket and --port", out)
		}
	})
	// D-09：--socket-mode 单给（无 --socket）配置矛盾 exit 2（双 flag 名进文案）。
	t.Run("socket-mode without socket", func(t *testing.T) {
		code, out := captureFd(t, &os.Stderr, func() int {
			return run([]string{"--bind", "127.0.0.1", "--socket-mode", "0660", "--", "true"})
		})
		if code != 2 {
			t.Errorf("run(--socket-mode 0660) = %d, want 2 (validateStartup refusal, zero listen/spawn)", code)
		}
		if !strings.Contains(out, "--socket-mode") || !strings.Contains(out, "--socket") {
			t.Errorf("stderr = %q, want containing both --socket-mode and --socket", out)
		}
	})
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

// TestListenSocket（07-02，D-08/D-09/D-10 + T-07-02a/b）unix socket listen 序列
// 五子测：
//   - 残留 socket 清理+可拨通：路径上预建真实残留 socket 不阻 bind（D-10
//     listen 前 os.Remove——Go listenStream 直接 syscall.Bind 无 unlink，
//     GOROOT 实证残留必收 EADDRINUSE，Remove 是必需而非保险）；listen 后
//     net.Dial("unix") 可建连。
//   - 非 socket 文件拒绝且内容保留（07-review CR-01）：普通文件占位 → 拒绝
//     启动，文件内容零触碰（D-10 意图仅为清理残留 socket 端点——operator
//     手误指向普通文件不得静默删除，拒绝文案只含路径与类别）。
//   - 权限位恰为 0660：stat mode.Perm() 断言（D-09 显式 Chmod 达成确定性，
//     不靠 umask 漂移——文件系统权限即认证边界，T-07-02b）。
//   - owner=self 属主正确：uid/gid 数字对经 Chown 落到 stat（self 免 root）。
//   - 失败回滚零残留：Listen 失败注入（父目录不存在，mode 合法）→ error 且
//     路径无残留；Chown 失败注入（非 root 限定——chown 他人 uid EPERM）→
//     error 且已建 socket 文件被 Close 自动 unlink 删除（UnixListener 默认
//     unlink:true，GOROOT unixsock_posix.go:210-216,230；T-07-02a 回滚纪律：
//     Chmod/Chown 失败必须回滚退出而非带病放行）。
func TestListenSocket(t *testing.T) {
	t.Run("stale socket removed and dialable", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "wesh.sock")
		// 制造真实残留 socket 端点：listen 后 SetUnlinkOnClose(false) 再 Close——
		// 文件遗留但无进程监听（systemd Restart= 场景 D-10 的清理对象；普通文件
		// 不再是合法残留——CR-01 起非 socket 类型一律拒绝，见下方子测）。
		ln0, err := net.Listen("unix", path)
		if err != nil {
			t.Fatalf("pre-create stale socket: %v", err)
		}
		ln0.(*net.UnixListener).SetUnlinkOnClose(false)
		if err := ln0.Close(); err != nil {
			t.Fatalf("close stale listener: %v", err)
		}
		ln, err := listenSocket(path, 0o660, -1, -1)
		if err != nil {
			t.Fatalf("listenSocket over stale socket: %v (D-10 os.Remove must clear residue)", err)
		}
		defer func() { _ = ln.Close() }()
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("net.Dial unix %s: %v", path, err)
		}
		_ = conn.Close()
	})
	t.Run("non-socket file refused and preserved", func(t *testing.T) {
		// 07-review CR-01：普通文件占位 → listenSocket 拒绝且文件内容原样保留
		//（拒绝经 run() listen 失败通道 exit 1——net.Listen 失败同档运行时
		// 错误 tier；文案只含路径与 "not a socket" 类别）。
		path := filepath.Join(t.TempDir(), "wesh.sock")
		content := []byte("precious-operator-data")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("pre-create regular file: %v", err)
		}
		ln, err := listenSocket(path, 0o660, -1, -1)
		if err == nil {
			_ = ln.Close()
			t.Fatal("listenSocket over regular file = nil error, want refusal (CR-01)")
		}
		if !strings.Contains(err.Error(), "not a socket") {
			t.Errorf("err = %v, want containing %q", err, "not a socket")
		}
		got, rerr := os.ReadFile(path)
		if rerr != nil || !bytes.Equal(got, content) {
			t.Errorf("file after refusal = %q/%v, want 内容原样保留（未被删除）", got, rerr)
		}
	})
	t.Run("mode exactly 0660", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "wesh.sock")
		ln, err := listenSocket(path, 0o660, -1, -1)
		if err != nil {
			t.Fatalf("listenSocket: %v", err)
		}
		defer func() { _ = ln.Close() }()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat: %v", err)
		}
		if info.Mode().Perm() != 0o660 {
			t.Errorf("socket mode = %o, want 660 (explicit Chmod, not umask drift)", info.Mode().Perm())
		}
	})
	t.Run("owner self applied", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "wesh.sock")
		ln, err := listenSocket(path, 0o660, os.Getuid(), os.Getgid())
		if err != nil {
			t.Fatalf("listenSocket with self owner: %v", err)
		}
		defer func() { _ = ln.Close() }()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat: %v", err)
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("Stat Sys() is %T, want *syscall.Stat_t", info.Sys())
		}
		if int(st.Uid) != os.Getuid() || int(st.Gid) != os.Getgid() {
			t.Errorf("socket owner = %d/%d, want self %d/%d", st.Uid, st.Gid, os.Getuid(), os.Getgid())
		}
	})
	t.Run("failure rollback leaves no residue", func(t *testing.T) {
		// Listen 失败注入：父目录不存在（mode 合法）——error 且零残留。
		path := filepath.Join(t.TempDir(), "nonexistent", "wesh.sock")
		ln, err := listenSocket(path, 0o660, -1, -1)
		if err == nil {
			_ = ln.Close()
			t.Fatal("listenSocket with nonexistent parent dir = nil error, want error")
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Errorf("after failed listenSocket, stat err = %v, want NotExist (zero residue)", statErr)
		}
		// Chown 失败注入（非 root 限定）：chown 他人 uid EPERM——序列已建 socket
		// 文件，失败回滚必须经 Close 自动 unlink 删除（T-07-02a 零残留纪律）。
		if os.Getuid() != 0 {
			p2 := filepath.Join(t.TempDir(), "wesh.sock")
			ln2, err2 := listenSocket(p2, 0o660, 1, 1)
			if err2 == nil {
				_ = ln2.Close()
				t.Fatal("listenSocket chown to uid 1 as non-root = nil error, want EPERM")
			}
			if _, statErr := os.Stat(p2); !os.IsNotExist(statErr) {
				t.Errorf("rollback residue: stat err = %v, want NotExist (Close auto-unlink)", statErr)
			}
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

// TestOpenBrowser（07-05，OPS-11，D-26/D-27）两场景：
//   - headless 跳过：linux 且 DISPLAY/WAYLAND_DISPLAY 均空 → openBrowser 直接
//     返回且 stderr 捕获提示行（不调用任何启动器——headless 服务器是常态部署
//     形态，跳过不阻断启动）；
//   - fake xdg-open：t.TempDir 写可执行脚本记录 argv 到文件 + PATH 前置 +
//     DISPLAY=:99 → 断言 argv[1] == 预期分享 URL（exec 的真实调用链不打桩，
//     与 07-VALIDATION「fake xdg-open PATH 前置断言 argv」策略一致）。
//
// darwin 分支不做运行时断言（构建标签差异——openBrowser 的 headless 检测只在
// linux 分支判定，darwin 直接 open；CI macOS 跑同款测试形态即本测试整体
// Skip，main.go openBrowser 注释登记）。URL 用占位 host:port 串——只作 argv
// 断言材料，不发起任何网络连接。
func TestOpenBrowser(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fake xdg-open 场景仅 linux 形态（darwin 分支不做运行时断言，见函数头注释）")
	}
	t.Run("headless skips browser launch", func(t *testing.T) {
		t.Setenv("DISPLAY", "")
		t.Setenv("WAYLAND_DISPLAY", "")
		_, out := captureFd(t, &os.Stderr, func() int {
			openBrowser("http://127.0.0.1:7681/s/headless-placeholder/")
			return 0
		})
		if !strings.Contains(out, "wesh: --open: no display detected (headless), skipping browser launch") {
			t.Errorf("stderr = %q, want headless skip line（D-27 提示行逐字）", out)
		}
	})
	t.Run("fake xdg-open receives share url", func(t *testing.T) {
		dir := t.TempDir()
		argvLog := filepath.Join(dir, "argv.log")
		script := filepath.Join(dir, "xdg-open")
		// 记录全部参数（每行一个）——断言材料；分享 URL 作 argv[1]（exec.Command
		// argv 分离不经 shell，T-07-05b）。
		content := "#!/bin/sh\nprintf '%s\n' \"$@\" > " + argvLog + "\n"
		if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
			t.Fatalf("write fake xdg-open: %v", err)
		}
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
		t.Setenv("DISPLAY", ":99")
		want := "http://127.0.0.1:7681/s/fake-xdg-open-placeholder/"
		openBrowser(want)
		// exec.Command(...).Start() 不等待（D-27 goroutine 调用形态）——fake 脚本
		// 落盘极快但仍需小窗轮询同步。
		deadline := time.Now().Add(2 * time.Second)
		var got []byte
		for {
			b, err := os.ReadFile(argvLog)
			if err == nil {
				got = b
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("fake xdg-open argv.log 2s 内未出现: %v", err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		lines := strings.Split(strings.TrimRight(string(got), "\n"), "\n")
		if len(lines) != 1 || lines[0] != want {
			t.Errorf("xdg-open argv = %v, want exactly [%q]", lines, want)
		}
	})
}
