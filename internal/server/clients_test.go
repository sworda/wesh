package server

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/sworda/wesh/internal/proto"
)

// clients_test.go —— TestClientCountInvariant（VALIDATION 05-01-08，RES-03 /
// T-05-04b 计数器泄漏防线，review #7）：registry 内嵌计数器 n 与成员集 set 的
// 对称不变量白盒锁定。同包白盒（tickets_test.go 先例——内部类型不导出，
// 直构造 registry 零值可用，无需 Server/pty 装配）。
//
// 不变量论证的结构半侧（clients.go registry.n 字段注释逐字登记）：运行期全部
// 移除路径——reader-error detach / kickOrCreditLocked 1013 踢出 / pinger
// CloseNow 后 reader 错误 detach——均收口于 removeLocked 单点，加减排它性对称，
// 漂移结构上不可能；lifecycle 广播后进程即退出（计数无后续读者）、panic 按 Go
// 语义进程崩溃（无恢复面）——两路径无漂移窗口。本测试锁定机械半侧：交错序列
// 逐步断言 + 幂等防御。

// TestClientCountInvariant：N 次 registerLocked 与交错 removeLocked 序列，每步
// 断言 r.n.Load() == int64(len(r.set))（逐步不变量——计数与成员集恒同步的机械
// 证据）；非成员 removeLocked 幂等 no-op 且计数不变（Pitfall 4 防线：减计数不得
// 越过实际移除——重复移除/幽灵移除若穿透减计数，运行时间增长计数静默低于实际
// 占用，③位 503 闸形同虚设：全员被拒而注册表实际为空）。
func TestClientCountInvariant(t *testing.T) {
	var r registry // 零值可用（registerLocked 惰性建 map）
	step := 0
	assertInvariant := func() {
		t.Helper()
		step++
		if got, want := r.n.Load(), int64(len(r.set)); got != want {
			t.Fatalf("step %d: n = %d, len(set) = %d——计数对称不变量破坏（review #7）", step, got, want)
		}
	}
	assertInvariant() // 零值起点：0 == 0

	// N 次注册，逐步断言。
	const n = 16
	clients := make([]*client, n)
	for i := range clients {
		clients[i] = &client{}
		r.registerLocked(clients[i])
		assertInvariant()
	}
	// 交错移除（奇数下标），逐步断言。
	for i := 1; i < n; i += 2 {
		if !r.removeLocked(clients[i]) {
			t.Fatalf("removeLocked(成员 #%d) = false, want true", i)
		}
		assertInvariant()
	}
	// 幂等防御：重复移除（刚移除的奇数成员）与幽灵移除（从未注册者）均为
	// no-op 且计数不变。
	if r.removeLocked(clients[1]) {
		t.Fatal("重复移除返回 true——幂等不变量破坏（Pitfall 4：减计数越过实际移除）")
	}
	assertInvariant()
	if r.removeLocked(&client{}) {
		t.Fatal("幽灵移除（从未注册）返回 true——幂等不变量破坏")
	}
	assertInvariant()
	// 再注册补充（先前移除的奇数位重新入册）+ 全量移除至空，逐步断言。
	for i := 1; i < n; i += 2 {
		r.registerLocked(clients[i])
		assertInvariant()
	}
	for i := 0; i < n; i++ {
		if !r.removeLocked(clients[i]) {
			t.Fatalf("removeLocked(成员 #%d) = false, want true", i)
		}
		assertInvariant()
	}
	if r.n.Load() != 0 {
		t.Fatalf("全量移除后 n = %d, want 0（计数泄漏，T-05-04b）", r.n.Load())
	}
}

// TestWriterMergeControlFramesOnly（WR-02 回归锁）：writer 批内合并仅限 OUTPUT
// 数据帧——控制帧（W/E）载荷是独立 JSON 文档，同类型相邻合并的拼接产物
// （W{...}{...}）前端 JSON.parse 抛错整帧丢弃（main.ts "discard malformed
// WELCOME"）。attach Welcome 与升格 Welcome 相邻同批的可达时序（server.go 升档
// 入队 → go writer 启动间隙 promoteNextLocked 再入队同一 outbox）见 05-REVIEW
// WR-02；合并将使被升格端丢失该 Welcome 的全部应用（prefs 不应用、welcomeDone
// 永不置位）。表测 mergeBatch（writer 合并段抽出形态，writer 对返回序列逐条
// conn.Write——msgs 元素数即线上 WS 消息数）：
//   - 两帧 Welcome 相邻 → 两条独立消息、逐字节原样（WR-02 核心回归）；
//   - 两帧 Error 相邻 → 两条独立消息（同纪律）；
//   - OUTPUT 连续段 → 合并单条（类型字节一次 + 载荷顺序拼接，§2.5 既有行为不变）；
//   - 混合 [O,O,W,W,O] → [合并 O, W, W, 单 O]（控制帧既不被合也不被并入）；
//   - Welcome–OUTPUT 相邻（类型不同）→ 各自单发；
//   - 空批 → nil（writer len(batch)==0 分支前置，双保险）。
func TestWriterMergeControlFramesOnly(t *testing.T) {
	// G-05-1（05-10）：WelcomeFrame 签名携会话尺寸——本测试锁定 mergeBatch 形状，
	// 尺寸值不参与断言，任取 80x24。第 5 参 session（D-08）传 SessionModeShared
	// 常量（本包白盒可引，clients.go:88-92）。
	wRO := proto.WelcomeFrame(proto.ModeRO, json.RawMessage(`{"fontSize":14}`), 80, 24, SessionModeShared)
	wRW := proto.WelcomeFrame(proto.ModeRW, json.RawMessage(`{"fontSize":14,"osc52":true}`), 80, 24, SessionModeShared)
	e1 := proto.ErrorFrame(proto.ErrServerError, "boom-1")
	e2 := proto.ErrorFrame(proto.ErrServerError, "boom-2")
	out := func(payload string) []byte { return append([]byte{proto.Output}, payload...) }

	tests := []struct {
		name  string
		batch [][]byte
		want  [][]byte // 期望写出的消息序列（1 元素 = 1 WS 消息）
	}{
		{"two welcomes stay separate", [][]byte{wRO, wRW}, [][]byte{wRO, wRW}},
		{"two errors stay separate", [][]byte{e1, e2}, [][]byte{e1, e2}},
		{"output run merges", [][]byte{out("hello "), out("wor"), out("ld")}, [][]byte{out("hello world")}},
		{"mixed control breaks merge", [][]byte{out("ab"), out("cd"), wRO, wRW, out("z")}, [][]byte{out("abcd"), wRO, wRW, out("z")}},
		{"welcome then output", [][]byte{wRW, out("x")}, [][]byte{wRW, out("x")}},
		{"single welcome passthrough", [][]byte{wRO}, [][]byte{wRO}},
		{"empty batch", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := mergeBatch(tt.batch)
			if len(msgs) != len(tt.want) {
				t.Fatalf("mergeBatch produced %d messages, want %d（控制帧被合并 = WR-02 回归）", len(msgs), len(tt.want))
			}
			for i := range tt.want {
				if !bytes.Equal(msgs[i], tt.want[i]) {
					t.Errorf("msg[%d] = %q, want %q（帧内容须逐字节原样）", i, msgs[i], tt.want[i])
				}
			}
		})
	}
}

// TestAfterDrainResendsDims（WR-02 白盒回归，05-13，用户裁决 option (a)）：锁定
// 「blocked 丢推送 → 恢复补发收敛」全链。两子测（TestClientCountInvariant 同文件
// 白盒纪律——&Server{} 直构造 + hubCond 装配，零值注册表可用）：
//
//  1. 「blocked 期后续推送帧不覆写暂存」（守卫语义锁——WR-02 缺陷链前半的行为
//     固定）：已 creditBlocked 端再触发 kickOrCreditLocked（尺寸推送帧），幂等
//     置位守卫 `if !c.creditBlocked` 跳过二次暂存——creditPending 仍逐字节 ==
//     首触发帧（防二次暂存覆写首帧的既有语义），creditBlocked 仍 true，kicks==0
//     （注册表仅 c 一端——剔除 c 后无其他未 blocked rw，结构上必走信用分支，
//     不触发踢出路径）。
//  2. 「afterDrain 开门补发当前会话尺寸」（WR-02 收敛锁）：恢复后 outbox 恰 2 帧
//     ——[0] 逐字节 == 原 creditPending（重投有序性：补发帧排在暂存帧之后），
//     [1] Welcome 携当前 sessionDimsLocked()（arbiter.last=100x30）且 prefs 按
//     c.mode 选档（rw→14/ro→13，D-13 双档在补发通道不漂移的机械证据）。ro 行
//     同构验证旁观者恢复路径——ro 永不持信用（R-08 分工表），ro 行进信用集是
//     防御性夹具驱动，非生产可达路径，断言补发选档正确性即可。
func TestAfterDrainResendsDims(t *testing.T) {
	t.Run("blocked期后续推送帧不覆写暂存", func(t *testing.T) {
		s := &Server{}
		s.hubCond = sync.NewCond(&s.hubMu)

		first := append([]byte{proto.Output}, []byte("first-trigger-output")...)
		c := &client{
			outbox:        newOutbox(4096),
			done:          make(chan struct{}),
			cancel:        func() {},
			remote:        "c",
			creditBlocked: true,
			creditPending: first,
		}
		c.mode.Store(proto.ModeRW)
		s.registry.registerLocked(c) // 注册表仅 c 一端——剔除 c 后无其他未 blocked rw，必走信用分支

		s.kickOrCreditLocked(c, proto.WelcomeFrame(proto.ModeRW, nil, 100, 30, SessionModeShared))

		if !bytes.Equal(c.creditPending, first) {
			t.Fatalf("creditPending = %q, want 首触发帧 %q（幂等置位守卫下后续推送帧不覆写暂存）", c.creditPending, first)
		}
		if !c.creditBlocked {
			t.Fatal("creditBlocked = false, want true（守卫不清位）")
		}
		if s.registry.kicks != 0 {
			t.Fatalf("registry.kicks = %d, want 0（信用分支不触发踢出）", s.registry.kicks)
		}
	})

	t.Run("afterDrain开门补发当前会话尺寸", func(t *testing.T) {
		tests := []struct {
			name         string
			mode         string
			wantFontSize float64
		}{
			{"rw端补发携rw档prefs", proto.ModeRW, 14},
			{"ro旁观者同构验证（防御夹具非生产可达）", proto.ModeRO, 13},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := &Server{}
				s.hubCond = sync.NewCond(&s.hubMu)
				s.arbiter.last = dims{cols: 100, rows: 30} // sessionDimsLocked 非零分支
				s.clientPrefsRW = json.RawMessage(`{"fontSize":14}`)
				s.clientPrefsRO = json.RawMessage(`{"fontSize":13}`) // 选档区分度夹具

				pending := append([]byte{proto.Output}, []byte("stalled-output-frame")...)
				c := &client{
					outbox:        newOutbox(4096), // bytes=0 < cap/2 → 恢复判定必过
					done:          make(chan struct{}),
					cancel:        func() {},
					remote:        "c",
					creditBlocked: true,
					creditPending: pending,
				}
				c.mode.Store(tt.mode)
				s.registry.registerLocked(c)

				s.afterDrain(c)

				if c.creditBlocked {
					t.Fatal("afterDrain 后 creditBlocked 仍 true, want false（既有恢复语义不变）")
				}
				if c.creditPending != nil {
					t.Fatalf("afterDrain 后 creditPending = %q, want nil（重投后清零）", c.creditPending)
				}
				batch, _ := c.outbox.drain()
				if len(batch) != 2 {
					t.Fatalf("drain 得 %d 帧, want 2（重投暂存帧在前、Welcome 补发帧在后）", len(batch))
				}
				if !bytes.Equal(batch[0], pending) {
					t.Fatalf("frame[0] = %q, want 原 creditPending %q（重投有序性：补发帧排在暂存帧之后）", batch[0], pending)
				}
				if batch[1][0] != proto.Welcome {
					t.Fatalf("frame[1] 类型字节 = %q, want 'W'（Welcome 补发帧）", batch[1][0])
				}
				var m map[string]any
				if err := json.Unmarshal(batch[1][1:], &m); err != nil {
					t.Fatalf("Welcome 补发帧 JSON 解码: %v", err)
				}
				if m["mode"] != tt.mode {
					t.Fatalf("补发帧 mode = %v, want %q（按各端当前生效 mode 组帧）", m["mode"], tt.mode)
				}
				if m["cols"] != float64(100) || m["rows"] != float64(30) {
					t.Fatalf("补发帧尺寸 = %vx%v, want 100x30（当前会话尺寸 = arbiter.last）", m["cols"], m["rows"])
				}
				prefs, ok := m["prefs"].(map[string]any)
				if !ok {
					t.Fatalf("补发帧 prefs 缺失或非对象: %v", m["prefs"])
				}
				if prefs["fontSize"] != tt.wantFontSize {
					t.Fatalf("补发帧 prefs.fontSize = %v, want %v（按 c.mode 选档，D-13 在补发通道不漂移）", prefs["fontSize"], tt.wantFontSize)
				}
			})
		}
	})
}
