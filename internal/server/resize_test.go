package server

import "testing"

// TestArbitrate 锁定 MULTI-04 仲裁纯函数行为（VALIDATION 05-01-04，RESEARCH
// Code Examples 逐字形态）：0 人 → 零值 dims 哨兵（不动 PTY 保持现状，调用方
// recalcNow 对零值不发起 Resize）；1 人 → last-wins 原样返回；≥2 人 →
// min(cols)×min(rows) 最小公共矩形；2→1 恢复 last-wins（剩余者尺寸）。
// 输入语义：尺寸在上游已经 proto.ClampDim 钳制 [1,1000]（DecodeHello/
// DecodeResize），纯函数不重复钳制。同包白盒（内部类型不导出，tickets_test.go
// 先例——go test 兼容同目录 server 与 server_test 双包并存）。
func TestArbitrate(t *testing.T) {
	t.Run("0人零值哨兵", func(t *testing.T) {
		if got := arbitrate(nil); got != (dims{}) {
			t.Errorf("arbitrate(nil) = %+v, want 零值 dims（0 人不动 PTY 哨兵）", got)
		}
		if got := arbitrate([]dims{}); got != (dims{}) {
			t.Errorf("arbitrate(空切片) = %+v, want 零值 dims", got)
		}
	})

	t.Run("1人last-wins原样返回", func(t *testing.T) {
		m := dims{cols: 132, rows: 43}
		if got := arbitrate([]dims{m}); got != m {
			t.Errorf("arbitrate(单成员) = %+v, want %+v（last-wins 原样返回）", got, m)
		}
	})

	t.Run("2人min-rect", func(t *testing.T) {
		got := arbitrate([]dims{{cols: 132, rows: 43}, {cols: 80, rows: 24}})
		if want := (dims{cols: 80, rows: 24}); got != want {
			t.Errorf("arbitrate({132,43},{80,24}) = %+v, want %+v（min(cols)×min(rows)）", got, want)
		}
		// 逐维独立取 min（非整体取小者尺寸）：{80,43}×{132,24} → {80,24}——
		// min-rect 不变量（任何参与端窗口 ≥ PTY 尺寸）依赖逐维语义。
		got = arbitrate([]dims{{cols: 80, rows: 43}, {cols: 132, rows: 24}})
		if want := (dims{cols: 80, rows: 24}); got != want {
			t.Errorf("arbitrate({80,43},{132,24}) = %+v, want %+v（逐维独立取 min）", got, want)
		}
	})

	t.Run("3人含极小值得极小矩形", func(t *testing.T) {
		got := arbitrate([]dims{{cols: 132, rows: 43}, {cols: 80, rows: 24}, {cols: 40, rows: 10}})
		if want := (dims{cols: 40, rows: 10}); got != want {
			t.Errorf("arbitrate(三成员含极小值) = %+v, want %+v（极小矩形）", got, want)
		}
		// 交换序无关：极小值在首位结果一致。
		got = arbitrate([]dims{{cols: 40, rows: 10}, {cols: 132, rows: 43}, {cols: 80, rows: 24}})
		if want := (dims{cols: 40, rows: 10}); got != want {
			t.Errorf("arbitrate(极小值在首位) = %+v, want %+v（交换序无关）", got, want)
		}
	})

	t.Run("2to1恢复last-wins", func(t *testing.T) {
		a, b := dims{cols: 132, rows: 43}, dims{cols: 80, rows: 24}
		if got := arbitrate([]dims{a, b}); got != (dims{cols: 80, rows: 24}) {
			t.Fatalf("两人阶段 = %+v, want min-rect {80 24}（前置）", got)
		}
		// B 离开（detach）后参与集仅余 A：单成员分支恢复 last-wins（剩余者尺寸）。
		if got := arbitrate([]dims{a}); got != a {
			t.Errorf("B 离开后 = %+v, want %+v（2→1 恢复 last-wins 剩余者尺寸）", got, a)
		}
	})
}
