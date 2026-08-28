package server

import (
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

// proxy_test.go 白盒锁定 07-03 反代信任提取层（SEC-07，D-15..D-20）：
// TestSanitizeRemoteUser 锁 D-19 清洗纪律（C0/C1/DEL 剥离 + 128 rune 截断，
// title.ts 同款纪律的 Go 移植——差异点：空结果返回空串即「不出键」，无标题的
// 'wesh' 回退）；TestProxyClientIP 锁 proxyInfo 三提取方法（clientIP/remote/
// remoteUser）在 trust 开/关 × XFF 多形态 × 回退形态下的逐值行为。
// 黑盒全链测试（logEvent remote_user 行/XFF 节流键/认证正交）在
// proxy_e2e_test.go（server_test 包——startTrackedServerWith/captureStderr
// 同步纪律要求，05-04 resize 两测试分文件先例：Go 单文件单 package 约束）。

// TestSanitizeRemoteUser（D-19）：逐 rune 剥离 ch<=0x1f、ch==0x7f、
// 0x80<=ch<=0x9f；截断 128 rune；多字节 rune 不碎；控制字符不占截断预算；
// 空结果返回空串（缺席即不出键——与 title.ts 空串回退 'wesh' 的差异点）。
func TestSanitizeRemoteUser(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "alice", "alice"},
		{"empty", "", ""},
		{"C0 stripped", "al\x00ice\x1f", "alice"},
		{"tab stripped", "a\tb", "ab"},
		// 日志注入防线主证据：换行被剥离，伪造不出第二行日志。
		{"newline injection stripped", "alice\nwesh: close remote=evil", "alicewesh: close remote=evil"},
		{"CR stripped", "a\rb", "ab"},
		{"DEL stripped", "a\x7fb", "ab"},
		{"C1 stripped", "a\u0080b\u009fc", "abc"},
		{"NEL stripped", "a\u0085b", "ab"},
		{"all control empty", "\x00\x1f\x7f\u0080\u009f", ""},
		{"multibyte kept", "操作员-01", "操作员-01"},
		{"unicode space kept", "a b", "a b"},
		{"truncation 128 ascii", strings.Repeat("a", 200), strings.Repeat("a", 128)},
		{"truncation multibyte not broken", strings.Repeat("测", 200), strings.Repeat("测", 128)},
		{"exactly 128 kept", strings.Repeat("x", 128), strings.Repeat("x", 128)},
		// 剥离先于截断计数：控制字符不消耗 128 rune 预算。
		{"control chars cost no budget", "a\x01" + strings.Repeat("b", 200), "a" + strings.Repeat("b", 127)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeRemoteUser(tt.in); got != tt.want {
				t.Errorf("sanitizeRemoteUser(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
	// 截断边界不碎多字节 rune：输出恒为合法 UTF-8。
	if got := sanitizeRemoteUser(strings.Repeat("测", 200)); !utf8.ValidString(got) {
		t.Errorf("truncated output is not valid UTF-8: %q", got)
	}
}

// TestProxyClientIP（D-15/D-17/D-20 + 07-review CR-02）：proxyInfo 三提取
// 方法逐值锁定。信任闸单一开关 = trust（--auth-header 给定；D-20 零双轨——
// XFF 与 user 头共用同一闸，未配置时 XFF 完全忽略、user 头不提取，与现状
// 逐字节一致）；XFF 取链首 IP（strings.Cut(",") 首段 + TrimSpace + ParseIP
// 校验——空串/非法值/控制字符注入均回退 TCP 对端现状取值，CR-02 日志注入
// 防线；SplitHostPort host 部分，失败回退整串）；remote 在 trust 时换
// clientIP（logEvent remote 字段换键），非 trust 时 r.RemoteAddr 原样
// （host:port 现状形态）；remoteUser 仅在 trust 且头存在时出 sanitize 值。
func TestProxyClientIP(t *testing.T) {
	newReq := func(remoteAddr string, headers map[string]string) *http.Request {
		r := &http.Request{RemoteAddr: remoteAddr, Header: http.Header{}}
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return r
	}
	trust := proxyInfo{trust: true, userHeader: "X-Remote-User"}
	distrust := proxyInfo{} // 零值 = 不信任/无头名（行为与现状逐字节一致）
	tests := []struct {
		name       string
		p          proxyInfo
		remoteAddr string
		headers    map[string]string
		wantIP     string
		wantRemote string
		wantUser   string
	}{
		// —— trust off（未配置 --auth-header）：XFF/头完全忽略（D-20 单一信任闸）——
		{"trust off ignores XFF", distrust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "203.0.113.7"}, "10.1.2.3", "10.1.2.3:5555", ""},
		{"trust off ignores user header", distrust, "10.1.2.3:5555", map[string]string{"X-Remote-User": "alice"}, "10.1.2.3", "10.1.2.3:5555", ""},
		{"trust off no headers", distrust, "10.1.2.3:5555", nil, "10.1.2.3", "10.1.2.3:5555", ""},
		// —— trust on：XFF 链首换键，非法/缺席回退 TCP 对端现状取值 ——
		{"trust on XFF chain first", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.1"}, "203.0.113.7", "203.0.113.7", ""},
		{"trust on XFF single", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "203.0.113.7"}, "203.0.113.7", "203.0.113.7", ""},
		{"trust on XFF spaces trimmed", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "  203.0.113.7  , 10.0.0.1"}, "203.0.113.7", "203.0.113.7", ""},
		{"trust on XFF first empty falls back", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": " , 10.0.0.1"}, "10.1.2.3", "10.1.2.3", ""},
		{"trust on XFF absent falls back", trust, "10.1.2.3:5555", nil, "10.1.2.3", "10.1.2.3", ""},
		{"trust on XFF empty falls back", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": ""}, "10.1.2.3", "10.1.2.3", ""},
		{"fallback unparseable RemoteAddr whole string", trust, "unix-addr-no-port", nil, "unix-addr-no-port", "unix-addr-no-port", ""},
		// —— trust on：XFF 链首非法值回退（07-review CR-02：ParseIP 校验闸——
		// 未校验首段原样进 logEvent remote 字段可注入 C1/CSI 伪造日志行）——
		{"trust on XFF valid ipv6 kept", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "2001:db8::1"}, "2001:db8::1", "2001:db8::1", ""},
		{"trust on XFF garbage falls back", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "not-an-ip"}, "10.1.2.3", "10.1.2.3", ""},
		{"trust on XFF unknown token falls back", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "unknown"}, "10.1.2.3", "10.1.2.3", ""},
		{"trust on XFF trailing junk falls back", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "203.0.113.7 evil"}, "10.1.2.3", "10.1.2.3", ""},
		// 日志注入防线主证据：C1 CSI（U+009B）拼在合法 IP 尾 → ParseIP 拒 →
		// remote 字段回退 TCP 对端形态，转义序列无从进入 stderr 单行日志。
		{"trust on XFF csi injection falls back", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "1.2.3.4\u009b"}, "10.1.2.3", "10.1.2.3", ""},
		{"trust on XFF ipv6 zone rejected falls back", trust, "10.1.2.3:5555", map[string]string{"X-Forwarded-For": "fe80::1%eth0"}, "10.1.2.3", "10.1.2.3", ""},
		// —— trust on：user 头提取经 sanitize；缺席/空值/全控制字符 → 空串不出键 ——
		{"trust on user header", trust, "10.1.2.3:5555", map[string]string{"X-Remote-User": "alice"}, "10.1.2.3", "10.1.2.3", "alice"},
		{"trust on user header sanitized", trust, "10.1.2.3:5555", map[string]string{"X-Remote-User": "al\x01ce"}, "10.1.2.3", "10.1.2.3", "alce"},
		{"trust on user header absent", trust, "10.1.2.3:5555", nil, "10.1.2.3", "10.1.2.3", ""},
		{"trust on user header empty", trust, "10.1.2.3:5555", map[string]string{"X-Remote-User": ""}, "10.1.2.3", "10.1.2.3", ""},
		{"trust on user header all control empty", trust, "10.1.2.3:5555", map[string]string{"X-Remote-User": "\x01\x7f"}, "10.1.2.3", "10.1.2.3", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newReq(tt.remoteAddr, tt.headers)
			if got := tt.p.clientIP(r); got != tt.wantIP {
				t.Errorf("clientIP = %q, want %q", got, tt.wantIP)
			}
			if got := tt.p.remote(r); got != tt.wantRemote {
				t.Errorf("remote = %q, want %q", got, tt.wantRemote)
			}
			if got := tt.p.remoteUser(r); got != tt.wantUser {
				t.Errorf("remoteUser = %q, want %q", got, tt.wantUser)
			}
		})
	}
	// 多值头（重复头行）取 Header.Get 首值（EDGE_ABSENT 回退裁决，07-03 PLAN
	// flagged_assumptions 登记，07-08 人工 UAT 清单复核项）。
	t.Run("multi value user header first wins", func(t *testing.T) {
		r := &http.Request{RemoteAddr: "10.1.2.3:5555", Header: http.Header{"X-Remote-User": []string{"alice", "bob"}}}
		if got := trust.remoteUser(r); got != "alice" {
			t.Errorf("remoteUser multi-value = %q, want %q（Header.Get 首值）", got, "alice")
		}
	})
}

// TestRemoteSanitize（08-02 D-19 纵深第二道）：trust 模式 remote() 返回值经
// sanitizeRemoteUser 清洗——白盒属性断言：任意构造 XFF 输入下 remote() 返回
// 值逐 rune 无 C0（ch<=0x1f）/DEL（ch==0x7f）/C1（0x80<=ch<=0x9f）。ParseIP
// 第一道结构性闸保持（注入值回退 TCP 对端键——07-review CR-02），本测构造
// 含 C1/NEL 的 XFF 首段与边界输入覆盖「第一道闸若放宽即穿透」面——encoding/json
// 只转义 C0、C1 原样穿透（08-RESEARCH Pitfall 5 GOROOT 实证），清洗是唯一
// 防线；两闸并存不冲突。
func TestRemoteSanitize(t *testing.T) {
	trust := proxyInfo{trust: true, userHeader: "X-Remote-User"}
	newReq := func(remoteAddr, xff string) *http.Request {
		r := &http.Request{RemoteAddr: remoteAddr, Header: http.Header{}}
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		// XFF 干净值透传相等（sanitize 对合法值零改动——幂等面）。
		{"clean ipv4 passthrough", "10.1.2.3:5555", "203.0.113.7", "203.0.113.7"},
		{"clean ipv6 passthrough", "10.1.2.3:5555", "2001:db8::1", "2001:db8::1"},
		{"clean chain first", "10.1.2.3:5555", "203.0.113.7, 10.0.0.1", "203.0.113.7"},
		// 含 C1/NEL 的 XFF 首段：ParseIP 闸拒 → 回退 TCP 对端键（CR-02 既有
		// 行为），sanitize 第二道对回退值幂等。
		// 注入位置取中段——尾段 NEL 会被 clientIP 的 TrimSpace 先吃掉（unicode.IsSpace(NEL)=true 本机实证），
		// 中段注入才是 ParseIP 闸的真实受力面。
		{"C1 mid-segment falls back", "10.1.2.3:5555", "1.2.3\u00854", "10.1.2.3"},
		{"NEL only first segment falls back", "10.1.2.3:5555", "\u0085", "10.1.2.3"},
		{"CSI suffix falls back", "10.1.2.3:5555", "1.2.3.4\u009b", "10.1.2.3"},
		// XFF 缺席/空值边界：回退 TCP 对端 host。
		{"xff absent falls back", "10.1.2.3:5555", "", "10.1.2.3"},
		// 回退链末端：RemoteAddr 不可 SplitHostPort 时整串回退（含端口缺失
		// 形态——sanitize 对该路径同样生效）。
		{"unparseable remoteaddr whole string", "unix-addr-no-port", "", "unix-addr-no-port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trust.remote(newReq(tt.remoteAddr, tt.xff))
			if got != tt.want {
				t.Errorf("remote() = %q, want %q", got, tt.want)
			}
			// 属性断言：返回值逐 rune 无 C0/C1/DEL（注入面防线本体）。
			for _, ch := range got {
				if ch <= 0x1f || ch == 0x7f || (ch >= 0x80 && ch <= 0x9f) {
					t.Errorf("remote() = %q 含控制 rune %#U（C0/C1/DEL 穿透）", got, ch)
				}
			}
		})
	}
}
