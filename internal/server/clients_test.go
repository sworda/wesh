package server

import (
	"bytes"
	"encoding/json"
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
//（W{...}{...}）前端 JSON.parse 抛错整帧丢弃（main.ts "discard malformed
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
	// 尺寸值不参与断言，任取 80x24。
	wRO := proto.WelcomeFrame(proto.ModeRO, json.RawMessage(`{"fontSize":14}`), 80, 24)
	wRW := proto.WelcomeFrame(proto.ModeRW, json.RawMessage(`{"fontSize":14,"osc52":true}`), 80, 24)
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
