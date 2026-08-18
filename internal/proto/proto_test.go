package proto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
)

// TestDecodeHello 表驱动锁定 Hello 解码契约：
// 标准载荷精确解码；未知字段必须忽略（D-02 演化纪律的自动化回归——若未来
// 有人加 DisallowUnknownFields，unknown-fields 组即红）；畸形 JSON ok=false；
// cols/rows 经 ClampDim 钳制到 [1,1000]。
func TestDecodeHello(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantOK      bool
		wantVersion string
		wantCols    int
		wantRows    int
		// ticket 专项断言：仅 checkTicket=true 的行断言 hp.Ticket == wantTicket。
		// unknown-fields 行载荷含 ticket:"secret"（Phase 3 起解码入 Ticket），
		// 该行只承担 D-02 未知字段（attach）忽略回归，零值补位不参与 ticket 断言。
		wantTicket  string
		checkTicket bool
	}{
		{"standard", `{"version":"wesh.v1","cols":80,"rows":24}`, true, "wesh.v1", 80, 24, "", false},
		{"unknown fields ignored (D-02)", `{"version":"wesh.v1","cols":100,"rows":40,"ticket":"secret","attach":2}`, true, "wesh.v1", 100, 40, "", false},
		{"malformed JSON", `{not json`, false, "", 0, 0, "", false},
		{"clamp lower bound", `{"version":"wesh.v1","cols":0,"rows":0}`, true, "wesh.v1", 1, 1, "", false},
		{"clamp upper bound", `{"version":"wesh.v1","cols":9999,"rows":9999}`, true, "wesh.v1", 1000, 1000, "", false},
		{"ticket round-trip", `{"version":"wesh.v1","cols":80,"rows":24,"ticket":"abc123"}`, true, "wesh.v1", 80, 24, "abc123", true},
		{"ticket omitted", `{"version":"wesh.v1","cols":80,"rows":24}`, true, "wesh.v1", 80, 24, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp, ok := DecodeHello([]byte(tt.payload))
			if ok != tt.wantOK {
				t.Fatalf("DecodeHello(%q) ok = %v, want %v", tt.payload, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if hp.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", hp.Version, tt.wantVersion)
			}
			if hp.Cols != tt.wantCols {
				t.Errorf("Cols = %d, want %d", hp.Cols, tt.wantCols)
			}
			if hp.Rows != tt.wantRows {
				t.Errorf("Rows = %d, want %d", hp.Rows, tt.wantRows)
			}
			if tt.checkTicket && hp.Ticket != tt.wantTicket {
				t.Errorf("Ticket = %q, want %q", hp.Ticket, tt.wantTicket)
			}
		})
	}
}

// TestWelcomeFrameErrorFrame 锁定 S→C 控制帧组帧形状：
// 1 字节类型 + JSON 载荷，解码往返后字段精确相等。
func TestWelcomeFrameErrorFrame(t *testing.T) {
	wf := WelcomeFrame(ModeRO, nil)
	if len(wf) == 0 || wf[0] != Welcome {
		t.Fatalf("WelcomeFrame[0] = %#x, want 'W'(%#x)", wf[0], Welcome)
	}
	var wp WelcomePayload
	if err := json.Unmarshal(wf[1:], &wp); err != nil {
		t.Fatalf("WelcomeFrame payload unmarshal: %v", err)
	}
	if wp.Mode != ModeRO {
		t.Errorf("WelcomeFrame mode = %q, want %q", wp.Mode, ModeRO)
	}

	const msg = "protocol version wesh.v1 required"
	ef := ErrorFrame(ErrVersionMismatch, msg)
	if len(ef) == 0 || ef[0] != Error {
		t.Fatalf("ErrorFrame[0] = %#x, want 'E'(%#x)", ef[0], Error)
	}
	var ep ErrorPayload
	if err := json.Unmarshal(ef[1:], &ep); err != nil {
		t.Fatalf("ErrorFrame payload unmarshal: %v", err)
	}
	if ep.Code != ErrVersionMismatch {
		t.Errorf("ErrorFrame code = %q, want %q", ep.Code, ErrVersionMismatch)
	}
	if ep.Message != msg {
		t.Errorf("ErrorFrame message = %q, want %q", ep.Message, msg)
	}

	// P4 D-13：prefs 往返与 omitempty 缺席两回归锁（各自独立 subtest，回归可定位）。
	t.Run("prefs round-trip", func(t *testing.T) {
		wf := WelcomeFrame(ModeRW, json.RawMessage(`{"fontSize":16,"resizeOverlay":false}`))
		var wp WelcomePayload
		if err := json.Unmarshal(wf[1:], &wp); err != nil {
			t.Fatalf("WelcomeFrame prefs payload unmarshal: %v", err)
		}
		if string(wp.Prefs) != `{"fontSize":16,"resizeOverlay":false}` {
			t.Errorf("WelcomeFrame prefs = %s, want %s", wp.Prefs, `{"fontSize":16,"resizeOverlay":false}`)
		}
	})
	t.Run("prefs omitted when nil (omitempty)", func(t *testing.T) {
		// 旧前端零漂移回归锁：nil prefs 组帧后帧体无 "prefs" 键（P2 D-02 加字段纪律）。
		wf := WelcomeFrame(ModeRO, nil)
		if bytes.Contains(wf, []byte("prefs")) {
			t.Errorf("WelcomeFrame(ModeRO, nil) = %s, must not contain %q key", wf[1:], "prefs")
		}
	})
}

// TestValidClientOptionKey 表驱动锁定客户端偏好白名单（P4 D-14）：恰 10 键通过；
// osc52（D-12 安全不对称——只能经服务端 --osc52 开启）、allowProposedApi（危险面
// 注入）、空串、大小写变体（fontsize——大小写敏感）与任意未知键一律拒绝。
func TestValidClientOptionKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"fontSize", true},
		{"fontFamily", true},
		{"cursorBlink", true},
		{"cursorStyle", true},
		{"scrollback", true},
		{"lineHeight", true},
		{"letterSpacing", true},
		{"theme", true},
		{"resizeOverlay", true},
		{"confirmBeforeUnload", true},
		{"osc52", false},            // D-12：安全敏感项只能经服务端 --osc52 开启
		{"allowProposedApi", false}, // D-14：危险面结构性排除
		{"", false},
		{"fontsize", false}, // 大小写敏感
		{"fontWeight", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("key=%q", tt.key), func(t *testing.T) {
			if got := ValidClientOptionKey(tt.key); got != tt.want {
				t.Errorf("ValidClientOptionKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// TestProtocolConstants 逐字锁定协议常量——这些是前后端公开契约（D-01/D-03/D-14），
// 手滑改码即协议破坏，本测试即红（T-02-01 缓解）。
func TestProtocolConstants(t *testing.T) {
	if Subprotocol != "wesh.v1" {
		t.Errorf("Subprotocol = %q, want %q", Subprotocol, "wesh.v1")
	}

	// Error code 形状：snake_case 机器串（D-06/D-07）
	snakeCase := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for name, code := range map[string]string{
		"ErrVersionMismatch": ErrVersionMismatch,
		"ErrServerError":     ErrServerError,
		"ErrAuthFailed":      ErrAuthFailed,
	} {
		if !snakeCase.MatchString(code) {
			t.Errorf("%s = %q, want snake_case ^[a-z][a-z0-9_]*$", name, code)
		}
	}
	// ErrAuthFailed 逐字钉死（Phase 3 D-10 兑现 P2 deferred 挂账；
	// 前后端公开契约，close reason 同名机器串）。
	if ErrAuthFailed != "auth_failed" {
		t.Errorf("ErrAuthFailed = %q, want %q", ErrAuthFailed, "auth_failed")
	}

	// 帧类型字节逐字断死
	frameBytes := []struct {
		name string
		got  rune
		want rune
	}{
		{"Hello", Hello, 'H'},
		{"Welcome", Welcome, 'W'},
		{"Error", Error, 'E'},
		{"Input", Input, '0'},
		{"Resize", Resize, '1'},
		{"Output", Output, '0'},
	}
	for _, fb := range frameBytes {
		if fb.got != fb.want {
			t.Errorf("%s = %#x(%q), want %#x(%q)", fb.name, fb.got, fb.got, fb.want, fb.want)
		}
	}

	// Welcome mode 对齐字符串（D-14）
	if ModeRO != "ro" {
		t.Errorf("ModeRO = %q, want %q", ModeRO, "ro")
	}
	if ModeRW != "rw" {
		t.Errorf("ModeRW = %q, want %q", ModeRW, "rw")
	}

	// 读上限两档（D-09 修订/D-11）
	if ReadLimitPreAuth != 4096 {
		t.Errorf("ReadLimitPreAuth = %d, want 4096", ReadLimitPreAuth)
	}
	if ReadLimitPostAuth != 16384 {
		t.Errorf("ReadLimitPostAuth = %d, want 16384", ReadLimitPostAuth)
	}
}
