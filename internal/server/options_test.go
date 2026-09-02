package server

// options_test.go（10-02 PC-01 装配契约行为锁）：ValidateOptions 三态契约的
// 表驱动锁定——ROADMAP「含」两互斥规则（SessionMode=per-client × SpawnFunc=nil
// 拒绝；SessionMode=shared × SpawnFunc≠nil 拒绝）+ 合法两态与零值归一
//（""→shared，New 零值兜底同语义）放行。
// SpawnFunc 非 nil 形态用空闭包占位——只断言校验分派，不调用闭包（inert
// 纪律保持，T-10-01c：SpawnFunc 被调用即未审计 spawn 路径提前暴露）。

import (
	"strings"
	"testing"

	"github.com/sworda/wesh/internal/pty"
)

func TestValidateOptions(t *testing.T) {
	spawnFunc := func(cols, rows int) (*pty.Session, error) { return nil, nil }
	tests := []struct {
		name       string
		opts       Options
		wantErrSub string // 非空 = 拒绝，文案须含此子串（两拒绝分支以互斥子串分锁）
	}{
		{"per-client nil SpawnFunc refused", Options{SessionMode: SessionModePerClient}, "requires SpawnFunc"},
		{"shared non-nil SpawnFunc refused", Options{SessionMode: SessionModeShared, SpawnFunc: spawnFunc}, "must not set SpawnFunc"},
		{"per-client with SpawnFunc allowed", Options{SessionMode: SessionModePerClient, SpawnFunc: spawnFunc}, ""},
		{"shared nil SpawnFunc allowed", Options{SessionMode: SessionModeShared}, ""},
		{"zero mode normalizes to shared", Options{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOptions(tt.opts)
			if tt.wantErrSub == "" {
				if err != nil {
					t.Errorf("ValidateOptions = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Errorf("ValidateOptions = nil, want containing %q", tt.wantErrSub)
			} else if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("err = %q, want containing %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}
