package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	creackpty "github.com/creack/pty"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
)

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

// TestSessionDimsLocked 锁定 G-05-1 会话尺寸取值（05-10，sessionDimsLocked 白盒，
// arbitrate 表测同文件纪律）：arbiter.last 零值（从未有参与者——首个参与端 attach
// 前 PTY 保持 spawn 尺寸，recalcNow 零值分支不动 PTY 的既有语义）→ 回落 spawn
// 尺寸 dims{pty.SpawnCols, pty.SpawnRows}（与真实 PTY spawn 尺寸同源的单一事实源
// 锁）；last 非零 → 原样返回（恒等锁——参与期会话尺寸 = 仲裁器 last = PTY 实际
// 尺寸）。Server 零值直构造即可（sessionDimsLocked 只读 arbiter.last，无 sess/注册表
// 依赖，TestClientCountInvariant 零值可用先例）。
func TestSessionDimsLocked(t *testing.T) {
	s := &Server{}
	if got := s.sessionDimsLocked(); got != (dims{cols: pty.SpawnCols, rows: pty.SpawnRows}) {
		t.Errorf("零值 arbiter sessionDimsLocked() = %+v, want %+v（spawn 尺寸回落——last 零值 ≠ 会话尺寸未知，PTY 实际尺寸 = spawn 值）",
			got, dims{cols: pty.SpawnCols, rows: pty.SpawnRows})
	}
	s.arbiter.last = dims{cols: 100, rows: 30}
	if got := s.sessionDimsLocked(); got != (dims{cols: 100, rows: 30}) {
		t.Errorf("last={100,30} 时 sessionDimsLocked() = %+v, want {100 30}（参与期恒等锁）", got)
	}
}

// TestPushSessionDimsKickRecalc（WR-01 白盒回归，05-13）：复现「推送循环内踢出
// 改变仲裁结果」交织并锁定留存端终值 == 最新仲裁尺寸。白盒必要性：缺陷窗口 =
// map 遍历序 × 推送循环内踢出 × 仲裁改变三者交织，黑盒不可控——白盒直调
// pushSessionDimsLocked 复现（TestSessionDimsLocked 同文件白盒纪律）。
//
// 交织链（每轮迭代全新装配）：B 被推送循环访问时 trySend 必败（outbox cap=1）
// → kickOrCreditLocked(B) → B 为 rw 且 A 为未 blocked rw → 离群即踢 →
// removeMember(B) → 嵌套 recalcNow：sizes 余 {A:60x50} → T2=60x50 ≠ last=60x24
// → last=T2 + Resize(60,50) + 嵌套推送 W(60x50) 给 A → 外层复检命中（修复后）
// → return。遍历序两态判据：A.outbox 恰 1 帧 ⟺ B-first 危险序（外层被复检中止，
// A 只收嵌套推送）；恰 2 帧 ⟺ A-first（[W(60x24), W(60x50)]）。普适不变量：
// 末帧 == arbiter.last（60x50）——修复前 B-first 态为 [W(60x50), W(60x24)]、
// 末帧 60x24 ≠ last，断言必败（测试牙齿）。
//
// conn 必须真实（httptest + websocket.Dial）：kickSlowConsumerLocked 的异步
// goroutine 调 c.conn.Close(1013, ...)，nil conn 会 panic 炸掉整个测试进程；
// handler Accept 后读至出错即返回，承接踢出 goroutine 的 1013 Close 握手，
// 防 goroutine 泄漏。map 遍历随机性是覆盖交织的来源（非缺陷）：至多 32 轮迭代
// 直至命中 B-first（2^-32 未命中概率等同确定性——不静默 skip，防测试形同虚设）。
func TestPushSessionDimsKickRecalc(t *testing.T) {
	// 测试起点单个 echo 丢弃服务端：Accept 后读至出错即返回（对端 1013 握手或
	// CloseNow 均使 Read 返回错误），承接全部迭代 conn 的生命周期。
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			if _, _, err := c.Read(context.Background()); err != nil {
				return
			}
		}
	}))
	defer echo.Close()

	hitBFirst := false
	for iter := 0; iter < 32 && !hitBFirst; iter++ {
		func() {
			// 每轮迭代全新装配（B 被踢后注册表/仲裁态已变，夹具一次性）。
			master, tty, err := creackpty.Open()
			if err != nil {
				t.Fatalf("iter %d: creack/pty.Open: %v", iter, err)
			}
			sess := &pty.Session{Master: master} // recalcNow→sess.Resize 需活 master fd 吸收 TIOCSWINSZ；无子进程即无收割负担
			defer func() {
				_ = sess.Close()
				_ = tty.Close()
			}()

			s := &Server{}
			s.sess = sess
			s.hubCond = sync.NewCond(&s.hubMu) // kick 链 Broadcast 需要非 nil cond
			s.arbiter = arbiter{sizes: make(map[*client]dims)}

			dial := func() *websocket.Conn {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				conn, _, err := websocket.Dial(ctx, "ws://"+echo.Listener.Addr().String(), nil)
				if err != nil {
					t.Fatalf("iter %d: websocket.Dial: %v", iter, err)
				}
				return conn
			}
			newClient := func(remote string, outboxCap int, conn *websocket.Conn) *client {
				c := &client{
					conn:   conn,
					remote: remote,
					outbox: newOutbox(outboxCap),
					done:   make(chan struct{}),
					cancel: func() {},
				}
				c.mode.Store(proto.ModeRW)
				return c
			}
			connA, connB := dial(), dial()
			defer connA.CloseNow() // 幂等
			defer connB.CloseNow() // 幂等（B 的 conn 已被踢出 goroutine 关过）

			a := newClient("A", 1<<20, connA) // 留存端：trySend 恒成
			b := newClient("B", 1, connB)     // 被踢端：~100B Welcome trySend 恒败
			s.registry.registerLocked(a)
			s.registry.registerLocked(b)

			s.arbiter.sizes[a] = dims{cols: 60, rows: 50}
			s.arbiter.sizes[b] = dims{cols: 80, rows: 24} // B 持 rows 轴最小值
			s.arbiter.last = dims{cols: 60, rows: 24}     // = min-rect T1（外层 recalcNow 已置 last 并 Resize 完毕）

			s.pushSessionDimsLocked(dims{cols: 60, rows: 24})

			batch, _ := a.outbox.drain()
			if len(batch) != 1 && len(batch) != 2 {
				t.Fatalf("iter %d: A.outbox 得 %d 帧, want 1（B-first）或 2（A-first）", iter, len(batch))
			}
			got := make([][2]float64, len(batch))
			for i, frame := range batch {
				if frame[0] != proto.Welcome {
					t.Fatalf("iter %d frame[%d] 类型字节 = %q, want 'W'（推送帧恒为 Welcome）", iter, i, frame[0])
				}
				var m map[string]any
				if err := json.Unmarshal(frame[1:], &m); err != nil {
					t.Fatalf("iter %d frame[%d] Welcome JSON 解码: %v", iter, i, err)
				}
				cols, okC := m["cols"].(float64)
				rows, okR := m["rows"].(float64)
				if !okC || !okR {
					t.Fatalf("iter %d frame[%d] cols/rows 缺失或非数值: %v", iter, i, m)
				}
				got[i] = [2]float64{cols, rows}
			}
			// 普适不变量：末帧 == arbiter.last == 60x50（留存端终值恒为最新仲裁尺寸）。
			if last := got[len(got)-1]; last != [2]float64{60, 50} {
				t.Fatalf("iter %d: 末帧尺寸 = %vx%v, want 60x50（== arbiter.last，WR-01 收敛）", iter, last[0], last[1])
			}
			if s.arbiter.last != (dims{cols: 60, rows: 50}) {
				t.Fatalf("iter %d: arbiter.last = %+v, want {60 50}（嵌套 recalcNow 已推进）", iter, s.arbiter.last)
			}
			switch len(batch) {
			case 1:
				hitBFirst = true // B-first 危险序命中：外层被复检中止，A 仅收嵌套推送的 W(60x50)
			case 2:
				// A-first：[外层捕获的 T1, 嵌套推送的 T2]——有意义的回归样本。
				if got[0] != [2]float64{60, 24} {
					t.Fatalf("iter %d: A-first 首帧 = %vx%v, want 60x24（外层捕获的 T1）", iter, got[0][0], got[0][1])
				}
			}
		}()
	}
	if !hitBFirst {
		t.Fatal("32 轮迭代未观测到 B-first 交织（map 随机序 2^-32 概率，等同确定性——不放行空转绿）")
	}
}
