package server_test

// events_test.go —— 08-02（OPS-08）审计事件目录行为锁：
//
//   - D-17 全量目录：连接面 attach/detach 两事件落地（认证面 auth_failed/
//     throttled 与会话面 session_*/shutdown 的边界锁见 Task 2/3 与本文件
//     头部决策登记延续）。
//   - D-20：attach/detach 携 client_id（= c.attachSeq，进程内单调递增从 1
//     起）——同一连接的 attach→detach 事件流可按 client_id 关联检索。
//   - D-21：client 断开 = detach 单事件 + reason 四值（normal/kick/
//     pong_timeout/shutdown）——kick/pong_timeout 不再单独打事件行（折入
//     detach reason）；wire 关闭码与 close reason（1013 slow_consumer 等
//     客户端可见形态）逐字节不变（slowclient_test.go 既有断言承接）。
//
// 同步纪律（05-01 + 08-01 门禁修正）：captureStderr 置换/恢复经
// server.LockStderr 持写锁；事件 emit 与 restore() 的 happens-before 边 =
// waitHandlers（Attach handler 内程序序）或 wire 观测（kick 事件的 1013
// 关闭帧在 emit 之后异步发出——观测到帧即 emit 已完成）。
//
// 断言纪律（08-01 定案）：parseEvents 字段断言；JSON 数字解进 float64
// （Pitfall 4）；恰一次 = 事件名+reason 精确计数。

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// eventsNamed 过滤事件集为指定事件名子集（保序——D-18：事件名走独立
// event 字段，精确相等无前缀歧义）。
func eventsNamed(evs []map[string]any, name string) []map[string]any {
	var out []map[string]any
	for _, m := range evs {
		if m["event"] == name {
			out = append(out, m)
		}
	}
	return out
}

// singleDetachReason 断言事件集中 event=="detach" 且 reason==want 的事件
// 恰好一条并返回之（D-21 恰好一次：kick 与 reader detach 互斥由 removeLocked
// 所有权保证，同连接绝无两条 detach）。
func singleDetachReason(t *testing.T, evs []map[string]any, want string, out string) map[string]any {
	t.Helper()
	var hit map[string]any
	n := 0
	for _, m := range evs {
		if m["event"] == "detach" && m["reason"] == want {
			n++
			hit = m
		}
	}
	if n != 1 {
		t.Fatalf("detach reason=%q event count = %d, want exactly 1 (out=%q)", want, n, out)
	}
	return hit
}

// startEventsServerWith 是 startTrackedServerWith 的全量返回变体：同装配序列
//（wg 包裹 handler——05-01 同步边纪律）+ 额外返回 srv 与 sess 句柄
//（srv = Shutdown 直调面；sess = session_start pid 断言面——sess.Cmd.Process.Pid
// 与事件 pid 字段相等性断言的数据源）。进程级事件（session_*/shutdown）的
// happens-before 边：session_start 在 New 内 emit（先于本函数返回，程序序）；
// session_end 在 lifecycle 内 emit 先于 terminate→exitf（waitExit 收码即
// 同步）；shutdown 由测试 goroutine 直调 srv.Shutdown() emit（程序序）。
func startEventsServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, srv *server.Server, sess *pty.Session, waitHandlers func()) {
	t.Helper()
	sess, err := pty.Start(argv, pty.StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh = make(chan int, 1)
	srv = server.New(sess, func(code int) { exitCh <- code }, opts)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { killServer(ln, sess) })
	var wg sync.WaitGroup
	h := srv.Handler()
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()
		h.ServeHTTP(w, r)
	}))
	return exitCh, "ws://" + ln.Addr().String() + "/ws", srv, sess, wg.Wait
}

// startEventsShutdownServerWith 是 startShutdownServerWith（shutdown_test.go）
// 的 handler 追踪变体（startEventsServerWith 的 sess 丢弃包装——Shutdown
// 直调面 + waitHandlers 同步边）。
func startEventsShutdownServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, srv *server.Server, waitHandlers func()) {
	t.Helper()
	exitCh, wsURL, srv, _, waitHandlers = startEventsServerWith(t, argv, opts)
	return exitCh, wsURL, srv, waitHandlers
}

// TestAttachDetachEvents（D-17/D-20）：dialHello 成功 → 恰 1 条 event=="attach"
// （remote/client_id/mode 键齐备、无 code 键、未配置 --auth-header 不出
// remote_user 键）；客户端主动关闭 → 恰 1 条 event=="detach"（reason=normal、
// code=1000）；两事件 client_id 同值（关联检索锁）且为 1（本实例首连接——
// attachSeq 从 1 起单调递增，D-20）。
func TestAttachDetachEvents(t *testing.T) {
	restore := captureStderr(t)
	defer restore() // 失败路径兜底恢复 os.Stderr（幂等）

	exitCh, wsURL, waitHandlers := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, mode := dialHello(t, ctx, wsURL, 80, 24)
	if mode != proto.ModeRW {
		t.Fatalf("welcome mode = %q, want %q (Writable:true)", mode, proto.ModeRW)
	}
	c.Close(websocket.StatusNormalClosure, "")

	// 同步边：attach emit 在 Attach 升档序列、detach emit 在读循环 detach——
	// 均先于 handler 返回（程序序），WaitGroup happens-before 使 restore()
	// 的 os.Stderr 写与该读同步。
	waitHandlers()
	assertNoExit(t, exitCh) // 多客户端推论：客户端断开不触发 exitf

	out := restore()
	evs := parseEvents(t, out)
	atts := eventsNamed(evs, "attach")
	if len(atts) != 1 {
		t.Fatalf("attach event count = %d, want exactly 1 (out=%q)", len(atts), out)
	}
	dets := eventsNamed(evs, "detach")
	if len(dets) != 1 {
		t.Fatalf("detach event count = %d, want exactly 1 (out=%q)", len(dets), out)
	}
	att, det := atts[0], dets[0]

	// attach 键齐备：remote（loopback host:port 现状形态）/client_id/mode。
	remote, _ := att["remote"].(string)
	if !strings.HasPrefix(remote, "127.0.0.1:") {
		t.Fatalf("attach remote = %q, want 127.0.0.1: 前缀", remote)
	}
	if att["mode"] != proto.ModeRW {
		t.Fatalf("attach mode = %v, want %q（RESEARCH A6 增强字段）", att["mode"], proto.ModeRW)
	}
	// client_id 从 1 起（D-20）——本测试该实例首连接，attachSeq==1 确定性成立。
	if att["client_id"] != float64(1) {
		t.Fatalf("attach client_id = %v (%T), want float64(1)（从 1 起单调递增）", att["client_id"], att["client_id"])
	}
	// attach 无 code 键（连接事件非关闭事件）；未配置 --auth-header 不出
	// remote_user 键（空串/缺省不出键语义，07-03 同口径）。
	if _, ok := att["code"]; ok {
		t.Fatalf("attach 不应出 code 键: %v", att)
	}
	if _, ok := att["remote_user"]; ok {
		t.Fatalf("attach 不应出 remote_user 键（未配置 --auth-header）: %v", att)
	}

	// detach：reason=normal、code=1000、client_id 与 attach 同值（关联检索锁）。
	if det["reason"] != "normal" {
		t.Fatalf("detach reason = %v, want %q", det["reason"], "normal")
	}
	if det["code"] != float64(websocket.StatusNormalClosure) {
		t.Fatalf("detach code = %v (%T), want float64(1000)", det["code"], det["code"])
	}
	if det["client_id"] != att["client_id"] {
		t.Fatalf("detach client_id = %v, want 与 attach 同值 %v（D-20 关联检索）", det["client_id"], att["client_id"])
	}
	if det["remote"] != att["remote"] {
		t.Fatalf("detach remote = %v, want 与 attach 同值 %v", det["remote"], att["remote"])
	}
	if _, ok := det["remote_user"]; ok {
		t.Fatalf("detach 不应出 remote_user 键（未配置 --auth-header）: %v", det)
	}
}

// TestDetachReason（D-21）：client 断开 = detach 单事件 + reason 字段四值——
// normal（对端主动关闭）/kick（outbox 写满 1013）/pong_timeout（保活超时
// 1006）/shutdown（Shutdown 1001 与 lifecycle 终结广播窗口 1000 两形态）。
// kick 与 pong_timeout 的独立事件行已折入 detach（零残留断言随子测锁定）。
func TestDetachReason(t *testing.T) {
	// normal：客户端主动 Close(1000) → detach reason=normal code=1000。
	t.Run("normal", func(t *testing.T) {
		restore := captureStderr(t)
		defer restore()

		exitCh, wsURL, waitHandlers := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c, _ := dialHello(t, ctx, wsURL, 80, 24)
		c.Close(websocket.StatusNormalClosure, "")

		waitHandlers()
		assertNoExit(t, exitCh)

		out := restore()
		evs := parseEvents(t, out)
		det := singleDetachReason(t, evs, "normal", out)
		if det["code"] != float64(websocket.StatusNormalClosure) {
			t.Fatalf("detach code = %v, want float64(1000)", det["code"])
		}
		// client_id 关联检索锁：detach 与本实例唯一 attach 同值。
		atts := eventsNamed(evs, "attach")
		if len(atts) != 1 || det["client_id"] != atts[0]["client_id"] {
			t.Fatalf("client_id 关联失败：attach=%v detach.client_id=%v (out=%q)", atts, det["client_id"], out)
		}
	})

	// kick：stall 客户端 outbox 写满被踢（slowclient_test.go 洪水夹具形态复用）
	// → detach reason=kick code=1013 恰 1 条；slow_consumer 独立事件行零残留
	//（wire 关闭帧 CloseError{1013, "slow_consumer"} 逐字节不变由
	// assertKicked1013 既有断言承接）。
	t.Run("kick", func(t *testing.T) {
		restore := captureStderr(t)
		defer restore()

		// seq 1 5000000 ≈ 38.9MB 洪水（> 单连接最坏吸收 ~10MiB + 64KiB
		// outbox——stall 必然传导到 outbox 写满）；WritePolicy=all 双 rw 前提
		//（05-03 适配先例）；OutboxBytes 小值覆写加速触发。
		exitCh, wsURL, waitHandlers := startTrackedServerWith(t, []string{"seq", "1", "5000000"}, server.Options{
			Writable:    true,
			WritePolicy: "all",
			OutboxBytes: 64 * 1024,
		})
		_ = exitCh // 本测试不断言子进程退出

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		stall, _ := dialHello(t, ctx, wsURL, 80, 24)
		normal, _ := dialHello(t, ctx, wsURL, 80, 24)
		// 放宽读上限：writer 合并段使积压后的单条 WS 消息超 Go 客户端库默认
		// 32KiB 读上限会被 1009 误关（slowclient_test 实测先例）。
		stall.SetReadLimit(4 * 1024 * 1024)
		normal.SetReadLimit(4 * 1024 * 1024)

		// 正常端读取 goroutine 自始运行（只计数不缓存）；stall 端不 Read。
		var normalBytes atomic.Int64
		go func() {
			for {
				_, data, err := normal.Read(context.Background())
				if err != nil {
					return
				}
				if len(data) > 0 && data[0] == proto.Output {
					normalBytes.Add(int64(len(data) - 1))
				}
			}
		}()

		// 等正常端累积超 12MiB：stall 端管道必然已满、outbox 已写满、踢出已触发。
		deadline := time.Now().Add(15 * time.Second)
		for normalBytes.Load() < 12*1024*1024 {
			if time.Now().After(deadline) {
				t.Fatalf("normal client received %d bytes in 15s, want >= 12MiB (flood not flowing)", normalBytes.Load())
			}
			time.Sleep(50 * time.Millisecond)
		}
		// 踢出证据 = 1013 关闭帧到达（wire 形态逐字不变的既有断言）；emit 在
		// 异步 Close 之前（kickSlowConsumerLocked 程序序）——观测到帧即事件已落。
		assertKicked1013(t, stall, 10*time.Second, "stall client")

		// 收口两端使 Attach handler 返回（waitHandlers 前置——覆盖 normal 端的
		// detach emit 同步边）。
		normal.CloseNow()
		stall.CloseNow()
		waitHandlers()

		out := restore()
		evs := parseEvents(t, out)
		det := singleDetachReason(t, evs, "kick", out)
		if det["code"] != float64(websocket.StatusTryAgainLater) {
			t.Fatalf("detach(kick) code = %v, want float64(1013)", det["code"])
		}
		// client_id 关联：stall 端先 attach → attachSeq==1（顺序 dial 确定性），
		// kick 事件与该连接 attach 事件同值。
		atts := eventsNamed(evs, "attach")
		if len(atts) != 2 || atts[0]["client_id"] != float64(1) || atts[1]["client_id"] != float64(2) {
			t.Fatalf("attach client_id 序列异常（want 1,2）: %v (out=%q)", atts, out)
		}
		if det["client_id"] != atts[0]["client_id"] {
			t.Fatalf("detach(kick) client_id = %v, want 与 stall 端 attach 同值 %v", det["client_id"], atts[0]["client_id"])
		}
		// D-21 折入锁：不再有独立踢出事件行。
		if n := countByEvent(evs, "slow_consumer"); n != 0 {
			t.Fatalf("slow_consumer 独立事件行残留 %d 条（D-21 应折入 detach reason=kick）: %q", n, out)
		}
		// 同连接绝无两条 detach（恰好一次互斥）：stall 端（client_id==1）的
		// detach 总数 == 1（即 kick 那条，reader 路径 removeLocked 返回 false）。
		stallDetach := 0
		for _, m := range eventsNamed(evs, "detach") {
			if m["client_id"] == float64(1) {
				stallDetach++
			}
		}
		if stallDetach != 1 {
			t.Fatalf("stall 端 detach 事件数 = %d, want 1（kick/reader 互斥恰好一次）: %q", stallDetach, out)
		}
	})

	// pong_timeout：PingInterval=100ms + PongTimeout=300ms + 客户端不 Read
	//（keepalive_test.go TestPongTimeout 同款注射形态）→ pinger 置
	// pongTimedOut（hubMu 内写）→ CloseNow → reader detach 同锁读 →
	// detach reason=pong_timeout code=1006；pinger 内独立事件行已除。
	t.Run("pong_timeout", func(t *testing.T) {
		restore := captureStderr(t)
		defer restore()

		exitCh, wsURL, waitHandlers := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:     true,
			PingInterval: 100 * time.Millisecond,
			PongTimeout:  300 * time.Millisecond,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c, _ := dialHello(t, ctx, wsURL, 80, 24)

		// 停止 Read 600ms（> interval 100ms + timeout 300ms）：库只在读路径回
		// pong，不 Read 即不应答 → 服务端必然 pong 超时。此窗内客户端任何
		// Read 都会回 pong 破坏负例语义，故只能 sleep 等待（既有纪律）。
		time.Sleep(600 * time.Millisecond)

		// 下一次 Read 必然返回错误（CloseNow 无关闭帧）；2s 护栏到期
		//（DeadlineExceeded）= 服务端未在 interval+timeout 内断开，按失败处理。
		rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _, rerr := c.Read(rctx)
		rcancel()
		if rerr == nil {
			t.Fatal("read after pong timeout unexpectedly succeeded — server kept dead connection")
		}
		if errors.Is(rerr, context.DeadlineExceeded) {
			t.Fatalf("read hit 2s guard — server did not CloseNow within interval+timeout: %v", rerr)
		}

		// 同步边：detach 在 Attach 读循环 handler 内先于返回执行。
		waitHandlers()
		assertNoExit(t, exitCh)

		out := restore()
		evs := parseEvents(t, out)
		det := singleDetachReason(t, evs, "pong_timeout", out)
		if det["code"] != float64(websocket.StatusAbnormalClosure) {
			t.Fatalf("detach(pong_timeout) code = %v, want float64(1006)", det["code"])
		}
		atts := eventsNamed(evs, "attach")
		if len(atts) != 1 || det["client_id"] != atts[0]["client_id"] {
			t.Fatalf("client_id 关联失败：attach=%v detach.client_id=%v (out=%q)", atts, det["client_id"], out)
		}
		// D-21 折入锁：pinger 独立事件行零残留。
		if n := countByEvent(evs, "pong_timeout"); n != 0 {
			t.Fatalf("pong_timeout 独立事件行残留 %d 条（D-21 应折入 detach reason）: %q", n, out)
		}
	})

	// shutdown 两形态（code 数据源 = closeBroadcastCode，与终结广播码同源）：
	//  A. srv.Shutdown() 广播 1001 → 已注册客户端 detach reason=shutdown
	//     code=1001（客户端读循环先行应答关闭帧，shutdown_test.go 夹具纪律）；
	//  B. 子进程自然退出（exit 42）→ lifecycle 广播 1000 窗口 → detach
	//     reason=shutdown code=1000（exiting 置位先于广播，既有不变量）。
	t.Run("shutdown", func(t *testing.T) {
		// —— A：Shutdown() 1001 形态 ——
		restore := captureStderr(t)
		exitCh, wsURL, srv, waitHandlers := startEventsShutdownServerWith(t, []string{"sh", "-c", "sleep 100"}, server.Options{Writable: true})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c, _ := dialHello(t, ctx, wsURL, 80, 24)
		errCh := readCloseAsync(ctx, c) // 客户端读循环先行——Close 握手应答面
		srv.Shutdown()
		if ce := awaitCloseError(t, errCh); ce.Code != websocket.StatusGoingAway {
			t.Fatalf("close code = %d, want %d (1001)", ce.Code, websocket.StatusGoingAway)
		}
		waitExit(t, exitCh, -1) // 默认 HUP stop-signal 收口（07-05 同形态）
		waitHandlers()
		out := restore()
		evs := parseEvents(t, out)
		det := singleDetachReason(t, evs, "shutdown", out)
		if det["code"] != float64(websocket.StatusGoingAway) {
			t.Fatalf("detach(shutdown/Shutdown) code = %v, want float64(1001)", det["code"])
		}
		atts := eventsNamed(evs, "attach")
		if len(atts) != 1 || det["client_id"] != atts[0]["client_id"] {
			t.Fatalf("client_id 关联失败：attach=%v detach.client_id=%v (out=%q)", atts, det["client_id"], out)
		}

		// —— B：lifecycle 子进程退出广播窗口 1000 形态 ——
		restore2 := captureStderr(t)
		exitCh2, wsURL2, waitHandlers2 := startTrackedServerWith(t, []string{"sh", "-c", "sleep 1; exit 42"}, server.Options{Writable: true})
		c2, _ := dialHello(t, ctx, wsURL2, 80, 24)
		frames, code := readExitClose(t, ctx, c2) // EXIT 帧 + 1000（06-01 广播序列）
		if code != websocket.StatusNormalClosure {
			t.Fatalf("close code = %d, want %d (1000)", code, websocket.StatusNormalClosure)
		}
		if ep := decodeExitFrame(t, frames[len(frames)-1]); ep.ExitCode != 42 {
			t.Fatalf("EXIT exit_code = %d, want 42", ep.ExitCode)
		}
		waitExit(t, exitCh2, 42)
		waitHandlers2()
		out2 := restore2()
		evs2 := parseEvents(t, out2)
		det2 := singleDetachReason(t, evs2, "shutdown", out2)
		if det2["code"] != float64(websocket.StatusNormalClosure) {
			t.Fatalf("detach(shutdown/lifecycle) code = %v, want float64(1000)", det2["code"])
		}
		atts2 := eventsNamed(evs2, "attach")
		if len(atts2) != 1 || det2["client_id"] != atts2[0]["client_id"] {
			t.Fatalf("client_id 关联失败：attach=%v detach.client_id=%v (out=%q)", atts2, det2["client_id"], out2)
		}
	})
}

// TestSessionEnd（D-17/D-22 会话生命周期事件）：进程级事件——零客户端即全量
// 事件面（恰 2 条：session_start + session_end，无 attach/detach 混入的强锁）。
// session_start：pid 键 = sess.Cmd.Process.Pid（数字），无 remote/code 键；
// session_end：exit_code 与 EXIT 帧同源（信号死亡 -1）+ duration_seconds>0
//（startedAt 起点 = New 尾部记录）+ signal 键仅信号死亡且 signalName 映射
// 命中出键（A7 裁决：未命中不出键，类型恒 string）。
func TestSessionEnd(t *testing.T) {
	// exit 42 形态：正常退出码传递——session_end exit_code==42、无 signal 键。
	t.Run("exit_code_42", func(t *testing.T) {
		restore := captureStderr(t)
		defer restore()

		exitCh, _, _, sess, _ := startEventsServerWith(t, []string{"sh", "-c", "exit 42"}, server.Options{Writable: true})
		waitExit(t, exitCh, 42) // session_end emit 先于 terminate→exitf（lifecycle 程序序）——收码即同步边

		out := restore()
		evs := parseEvents(t, out)
		if len(evs) != 2 {
			t.Fatalf("事件总数 = %d, want 恰 2（session_start+session_end，零客户端进程级事件面）: %q", len(evs), out)
		}
		starts := eventsNamed(evs, "session_start")
		ends := eventsNamed(evs, "session_end")
		if len(starts) != 1 || len(ends) != 1 {
			t.Fatalf("session_start=%d session_end=%d, want 各 1: %q", len(starts), len(ends), out)
		}
		// session_start：pid 键存在且与 sess.Cmd.Process.Pid 相等；无 remote/code 键。
		if starts[0]["pid"] != float64(sess.Cmd.Process.Pid) {
			t.Fatalf("session_start pid = %v, want %d（sess.Cmd.Process.Pid）", starts[0]["pid"], sess.Cmd.Process.Pid)
		}
		if _, ok := starts[0]["remote"]; ok {
			t.Fatalf("session_start 不应出 remote 键（进程级事件）: %v", starts[0])
		}
		if _, ok := starts[0]["code"]; ok {
			t.Fatalf("session_start 不应出 code 键（进程级事件）: %v", starts[0])
		}
		// session_end：exit_code==42（EXIT 帧同源）；duration_seconds>0；无 signal 键。
		if ends[0]["exit_code"] != float64(42) {
			t.Fatalf("session_end exit_code = %v, want 42", ends[0]["exit_code"])
		}
		if dur, _ := ends[0]["duration_seconds"].(float64); dur <= 0 {
			t.Fatalf("session_end duration_seconds = %v, want >0", ends[0]["duration_seconds"])
		}
		if _, ok := ends[0]["signal"]; ok {
			t.Fatalf("session_end 不应出 signal 键（非信号死亡）: %v", ends[0])
		}
	})

	// kill -HUP 形态（exit_test.go TestExitFrameSignal argv 先例）：信号死亡
	// exit_code==-1 + signal=="SIGHUP"（显式大写名映射命中出键）+
	// duration_seconds>0。
	t.Run("signal_sighup", func(t *testing.T) {
		restore := captureStderr(t)
		defer restore()

		exitCh, _, _, _, _ := startEventsServerWith(t, []string{"sh", "-c", "kill -HUP $$"}, server.Options{Writable: true})
		waitExit(t, exitCh, -1) // SIGHUP 致死 ExitCode()=-1（accept-255 断言常量同源）

		out := restore()
		evs := parseEvents(t, out)
		ends := eventsNamed(evs, "session_end")
		if len(ends) != 1 {
			t.Fatalf("session_end count = %d, want 1: %q", len(ends), out)
		}
		if ends[0]["exit_code"] != float64(-1) {
			t.Fatalf("session_end exit_code = %v, want -1（信号死亡不得粉饰为正常退出码）", ends[0]["exit_code"])
		}
		if ends[0]["signal"] != "SIGHUP" {
			t.Fatalf("session_end signal = %v, want %q（signalName 映射命中出键）", ends[0]["signal"], "SIGHUP")
		}
		if dur, _ := ends[0]["duration_seconds"].(float64); dur <= 0 {
			t.Fatalf("session_end duration_seconds = %v, want >0", ends[0]["duration_seconds"])
		}
	})
}

// TestShutdownEvent（D-17）：srv.Shutdown() 入口 → 恰 1 条 event=="shutdown"
//（进程级事件——无 remote/code 键）。
func TestShutdownEvent(t *testing.T) {
	restore := captureStderr(t)
	defer restore()

	exitCh, _, srv, _, _ := startEventsServerWith(t, []string{"sh", "-c", "sleep 100"}, server.Options{Writable: true})
	srv.Shutdown()        // shutdown emit 在本 goroutine 内（Shutdown 入口程序序）
	waitExit(t, exitCh, -1) // 默认 HUP stop-signal 收口（session_end 同步边同 waitExit）

	out := restore()
	evs := parseEvents(t, out)
	shutdowns := eventsNamed(evs, "shutdown")
	if len(shutdowns) != 1 {
		t.Fatalf("shutdown event count = %d, want exactly 1: %q", len(shutdowns), out)
	}
	if _, ok := shutdowns[0]["remote"]; ok {
		t.Fatalf("shutdown 不应出 remote 键（进程级事件）: %v", shutdowns[0])
	}
	if _, ok := shutdowns[0]["code"]; ok {
		t.Fatalf("shutdown 不应出 code 键（进程级事件）: %v", shutdowns[0])
	}
}
