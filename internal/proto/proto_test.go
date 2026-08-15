package proto

import (
	"encoding/json"
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
	}{
		{"standard", `{"version":"wesh.v1","cols":80,"rows":24}`, true, "wesh.v1", 80, 24},
		{"unknown fields ignored (D-02)", `{"version":"wesh.v1","cols":100,"rows":40,"ticket":"secret","attach":2}`, true, "wesh.v1", 100, 40},
		{"malformed JSON", `{not json`, false, "", 0, 0},
		{"clamp lower bound", `{"version":"wesh.v1","cols":0,"rows":0}`, true, "wesh.v1", 1, 1},
		{"clamp upper bound", `{"version":"wesh.v1","cols":9999,"rows":9999}`, true, "wesh.v1", 1000, 1000},
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
		})
	}
}

// TestWelcomeFrameErrorFrame 锁定 S→C 控制帧组帧形状：
// 1 字节类型 + JSON 载荷，解码往返后字段精确相等。
func TestWelcomeFrameErrorFrame(t *testing.T) {
	wf := WelcomeFrame(ModeRO)
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
}
