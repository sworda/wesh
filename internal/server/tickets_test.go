package server

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/sworda/wesh/internal/proto"
)

// TestTicketStore 锁定一次性 ticket 存储语义（SEC-02）：单次使用（查即删）/
// mode 签发时绑定（D-11）/ 22 字符 base64url 独立 secret 形态（C6）/
// 重放·过期·非法同归 false（D-10 无 oracle）/ 签发惰性清扫（Pitfall 4）。
// 全部经 now 手工注入推进，零真实 sleep；同包白盒（内部类型不导出，
// 不走 server_test 黑盒——与既有测试文件包名不同属有意选择，go test 兼容同目录双包）。
func TestTicketStore(t *testing.T) {
	base := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	t.Run("issue 形态与登记精确", func(t *testing.T) {
		ts := newTicketStore(time.Minute)
		now := base
		t1 := ts.issue(proto.ModeRW, now)
		t2 := ts.issue(proto.ModeRO, now)
		if len(t1) != 22 {
			t.Errorf("len(issue) = %d, want 22（16B → base64.RawURLEncoding）", len(t1))
		}
		raw, err := base64.RawURLEncoding.DecodeString(t1)
		if err != nil || len(raw) != 16 {
			t.Errorf("issue 输出非 base64url(16B)：raw=%v err=%v", raw, err)
		}
		if t1 == t2 {
			t.Errorf("两次签发相同 %q——随机源失效（C6：可预测 ticket = 认证绕过）", t1)
		}
		e, ok := ts.m[t1]
		if !ok {
			t.Fatalf("签发的 ticket 未登记入 map")
		}
		if e.mode != proto.ModeRW {
			t.Errorf("mode = %q, want %q（D-11 签发时精确绑定）", e.mode, proto.ModeRW)
		}
		if !e.exp.Equal(now.Add(time.Minute)) {
			t.Errorf("exp = %v, want %v（now+ttl 精确）", e.exp, now.Add(time.Minute))
		}
	})

	t.Run("redeem 单次使用与 mode 返回", func(t *testing.T) {
		ts := newTicketStore(time.Minute)
		now := base
		tk := ts.issue(proto.ModeRW, now)
		mode, ok := ts.redeem(tk, now.Add(time.Second))
		if !ok || mode != proto.ModeRW {
			t.Errorf("首次 redeem = (%q, %v), want (%q, true)", mode, ok, proto.ModeRW)
		}
		if _, ok := ts.m[tk]; ok {
			t.Errorf("核销后 ticket 仍在 map——查即删（单次使用）不变量破坏")
		}
		mode, ok = ts.redeem(tk, now.Add(2*time.Second))
		if ok || mode != "" {
			t.Errorf("重放 redeem = (%q, %v), want (\"\", false)（D-10 同口径无 oracle）", mode, ok)
		}
	})

	t.Run("过期核销同归 false 且已删除", func(t *testing.T) {
		ts := newTicketStore(time.Minute)
		now := base
		tk := ts.issue(proto.ModeRO, now)
		mode, ok := ts.redeem(tk, now.Add(2*time.Minute)) // now 推过 ttl
		if ok || mode != "" {
			t.Errorf("过期 redeem = (%q, %v), want (\"\", false)（D-10 过期按不存在处理）", mode, ok)
		}
		if _, ok := ts.m[tk]; ok {
			t.Errorf("过期核销后 ticket 仍在 map——查即删不变量破坏")
		}
	})

	t.Run("未签发与畸形串同归 false", func(t *testing.T) {
		ts := newTicketStore(time.Minute)
		for _, tk := range []string{
			"AAAAAAAAAAAAAAAAAAAAAA", // 合法 22 字符形态但从未签发
			"not-a-ticket",           // 畸形串
			"",                       // 空串
		} {
			if mode, ok := ts.redeem(tk, base); ok || mode != "" {
				t.Errorf("redeem(%q) = (%q, %v), want (\"\", false)", tk, mode, ok)
			}
		}
	})

	t.Run("签发惰性清扫过期项", func(t *testing.T) {
		ts := newTicketStore(time.Minute)
		now := base
		ts.issue(proto.ModeRO, now)
		ts.issue(proto.ModeRO, now)
		if len(ts.m) != 2 {
			t.Fatalf("签发两条后 len(m) = %d, want 2", len(ts.m))
		}
		now = now.Add(2 * time.Minute) // 两条均过期
		ts.issue(proto.ModeRO, now)    // 签发顺手机会性清扫
		if len(ts.m) != 1 {
			t.Errorf("惰性清扫后 len(m) = %d, want 1（map 不随历史签发单调增长，Pitfall 4）", len(ts.m))
		}
	})

	t.Run("零值 ttl 兜底 60s", func(t *testing.T) {
		for _, ttl := range []time.Duration{0, -time.Second} {
			ts := newTicketStore(ttl)
			if ts.ttl != defaultTicketTTL {
				t.Errorf("newTicketStore(%v).ttl = %v, want %v（零值可用纪律）", ttl, ts.ttl, defaultTicketTTL)
			}
		}
		if defaultTicketTTL != 60*time.Second {
			t.Errorf("defaultTicketTTL = %v, want 60s（ROADMAP 锁定）", defaultTicketTTL)
		}
	})
}
