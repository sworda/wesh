package server_test

// multi_test.go 锁定 MULTI-01 主干与多客户端生命周期推论（VALIDATION
// 05-W0-01/05-01-01）：fan-out 两端逐字节一致 / 断开不退出且再 attach 成功 /
// 子进程退出广播 1000 + exitf。helper 复用 e2e_test.go 零改动（PATTERNS exact
// 先例：startTestServerWith/dialHello/waitExit/assertNoExit 同包直接复用）。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// sendInput 发送 INPUT 帧（payload 原样拼在类型字节后）。
func sendInput(t *testing.T, ctx context.Context, c *websocket.Conn, payload []byte) {
	t.Helper()
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT: %v", err)
	}
}

// accumPayload 累积读 OUTPUT 帧直到收齐 want 载荷并逐字节比对（TestEchoPTY 累积
// 模式的包级复用形态）；途中收到任何非 OUTPUT 帧即 fatal——升格 Welcome 推送在
// 此形态下天然构成「窗口内无第二帧 Welcome」断言（单 owner 证据）。
func accumPayload(t *testing.T, ctx context.Context, c *websocket.Conn, want []byte) {
	t.Helper()
	got := make([]byte, 0, len(want))
	for len(got) < len(want) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(want) {
		t.Fatalf("echo payload = %q, want %q", got, want)
	}
}

// readUntilWelcome 在 goroutine 中持续读 conn 直到出现 Welcome 帧，返回其解码
// 载荷（Pitfall 2 竞速形态——客户端 Read 永不带 deadline ctx，调用方以 select
// time.After 收口）。读终结（连接关闭）时关闭 channel——接收方得 nil 载荷。
func readUntilWelcome(c *websocket.Conn) <-chan map[string]any {
	ch := make(chan map[string]any, 1)
	go func() {
		defer close(ch)
		for {
			_, data, err := c.Read(context.Background())
			if err != nil {
				return
			}
			if len(data) > 0 && data[0] == proto.Welcome {
				var wm map[string]any
				if err := json.Unmarshal(data[1:], &wm); err == nil {
					ch <- wm
					return
				}
			}
		}
	}()
	return ch
}

// TestMultiClientFanout（VALIDATION 05-01-01，MULTI-01 主干断言）：双客户端 attach
// 同一服务端各自收到 Welcome，且实时收到同一 OUTPUT 字节流——两端累积 payload
// 逐字节一致（hub 每 chunk 组一次共享只读帧的行为证据，P5-1）。
// 异尺寸参数化（80x24 与 132x43——dialHello 签名参数化的既定用法，e2e_test.go
// 注释：禁止硬编码 80x24）。
// 05-03 适配：显式 WritePolicy=all——fan-out 语义隔离（owner 默认策略下第二客户
// 端会降级 ro，双 rw 断言前提不再成立；owner 降级行为由 TestOwnerPolicy 专测）。
func TestMultiClientFanout(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:    true,
		WritePolicy: "all",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
	cB, modeB := dialHello(t, ctx, wsURL, 132, 43)
	// 两端各自 Welcome mode 一致（无认证 --writable 均 rw）。
	if modeA != proto.ModeRW || modeB != proto.ModeRW {
		t.Fatalf("welcome modes = %q / %q, want both %q", modeA, modeB, proto.ModeRW)
	}

	// 子进程输出驱动：A 发 INPUT，行规程回显经 hub 扇出到 A/B。
	payload := []byte("fanout-payload")
	if err := cA.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT on A: %v", err)
	}
	// 两端各自累积收齐同一 payload（TestEchoPTY 累积模式双份，e2e_test.go 既定形态；
	// 载荷无空白字符故直接比较——strings.Fields 切分免疫 ONLCR 纪律本例无需动用）。
	accum := func(c *websocket.Conn) []byte {
		t.Helper()
		got := make([]byte, 0, len(payload))
		for len(got) < len(payload) {
			_, data, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("read OUTPUT: %v (got %q so far)", err, got)
			}
			if len(data) == 0 || data[0] != proto.Output {
				t.Fatalf("unexpected frame: %v", data)
			}
			got = append(got, data[1:]...)
		}
		return got
	}
	gotA := accum(cA)
	gotB := accum(cB)
	// 两端 payload 逐字节一致断言（string 比较即逐字节）。
	if string(gotA) != string(gotB) {
		t.Fatalf("fan-out payload mismatch: A=%q B=%q", gotA, gotB)
	}
	if string(gotA) != string(payload) {
		t.Fatalf("fan-out payload = %q, want %q", gotA, payload)
	}

	cA.Close(websocket.StatusNormalClosure, "")
	cB.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestDetach（VALIDATION 05-W0-01，多客户端生命周期推论）：任一客户端断开不再触发
// exitf——exitCh 200ms 静默 + 其他客户端继续 echo 正常 + 断开者立即重新 attach
// 成功（注册表移除断言的行为化；P1 D-11 单次语义终结，服务端生命周期只随子进程）。
// 05-03 适配：显式 WritePolicy=all——本测试锁定的是断开生命周期语义而非权限语义；
// owner 默认策略下 B 会被降级 ro（INPUT 静默丢）且 A 再 attach 归队 ro，两处断言
// 前提不再成立（owner 降级/递补/归队行为由 TestOwnerPolicy/TestSuccession 专测）。
func TestDetach(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:    true,
		WritePolicy: "all",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	cB, _ := dialHello(t, ctx, wsURL, 80, 24)

	// A 断开（CloseNow 不发关闭帧，覆盖网络断开路径）→ exitCh 200ms 静默反证。
	cA.CloseNow()
	assertNoExit(t, exitCh)

	// B 继续 echo 正常（断开者对在线客户端零影响）。
	payload := []byte("b-survives-detach")
	if err := cB.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT on B: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := cB.Read(ctx)
		if err != nil {
			t.Fatalf("B read OUTPUT after A detach: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("B echo payload = %q, want %q", got, payload)
	}

	// A 立即重新 attach 成功（注册表移除断言的行为化）。
	cA2, modeA2 := dialHello(t, ctx, wsURL, 80, 24)
	if modeA2 != proto.ModeRW {
		t.Fatalf("re-attach welcome mode = %q, want %q", modeA2, proto.ModeRW)
	}
	cB.Close(websocket.StatusNormalClosure, "")
	cA2.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestExitBroadcast（VALIDATION 05-W0-01，D-10 唯一终结路径的多客户端形态）：
// 子进程退出是唯一终结路径——lifecycle Wait → Drain → 并行广播 1000 关闭全部
// 客户端（双端均收 CloseError 1000）→ exitf 以子进程退出码收口（退出码传递
// 语义不变）。
func TestExitBroadcast(t *testing.T) {
	// sh 读一行后以 3 退出：保证广播时两客户端均已 attach（消除子进程抢在 Dial
	// 完成前退出的竞态——wesh-helper-exit42 同款编排，此处无需专用 helper）。
	exitCh, wsURL := startTestServer(t, []string{"/bin/sh", "-c", "read x; exit 3"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, _ := dialHello(t, ctx, wsURL, 80, 24)
	cB, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 触发子进程退出：A 发一行（INPUT 经 master 送达 sh stdin，规范模式行缓冲）。
	if err := cA.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
		t.Fatalf("write INPUT on A: %v", err)
	}

	// A/B 两端均收 1000 关闭（读到 CloseError 为止，途中回显等 OUTPUT 帧丢弃——
	// TestExitCodePropagation 既定读取循环形态双份）。
	for i, c := range []*websocket.Conn{cA, cB} {
		var ce websocket.CloseError
		for {
			if _, _, rerr := c.Read(ctx); rerr != nil {
				if !errors.As(rerr, &ce) {
					t.Fatalf("client %d read terminated without CloseError: %v", i, rerr)
				}
				break
			}
		}
		if ce.Code != websocket.StatusNormalClosure {
			t.Fatalf("client %d close code = %d, want %d (1000)", i, ce.Code, websocket.StatusNormalClosure)
		}
	}
	// 广播后 exitf 以子进程退出码收口（D-10 退出码传递语义不变）。
	waitExit(t, exitCh, 3)
}

// TestSigwinchOnAttach（D-11 送达证据）：新客户端 attach 完成时服务端向 PTY 前台
// 进程组显式发一次 SIGWINCH——helper 收到信号后落盘标记文件（GOT_WINCH）。
// 两端同尺寸 80x24：排除内核 TIOCSWINSZ 异尺寸发信号的干扰（P5-3 本机实证 Linux
// 同尺寸不发信号）——标记出现即显式 SignalForegroundGroup 送达的证据，而非
// resize 副作用。同步纪律：helper 先从 stdin 读一字节再装处理器报 READY，c1 发
// INPUT 驱动并回读 READY 确认处理器就位，c2 attach 触发第二次信号——消除
//「attach 信号先于处理器安装被默认忽略」的竞态。
// 本测试在 CI macos runner 同样执行（.github/workflows/ci.yml 双平台矩阵既定）——
// darwin 同尺寸行为假设 A1 的验证通道（review MEDIUM 项处置：以 CI 双平台运行
// 实证替代本机平台断言；即便 darwin 同尺寸发信号，显式 SIGWINCH 也只是冗余无害）。
func TestSigwinchOnAttach(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "got_winch")
	_, wsURL := startTestServer(t, helperArgv(t, "wesh-helper-winch", marker))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c1, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c1.CloseNow()
	// 驱动 helper 装处理器：INPUT 携 '\n'（PTY 规范模式行缓冲）；回读 READY 确认
	// 就位（Pitfall 2：Read 永不带 deadline ctx——goroutine + select time.After）。
	if err := c1.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
		t.Fatalf("write INPUT to arm helper: %v", err)
	}
	ready := make(chan struct{}, 1)
	go func() {
		var acc []byte
		for {
			_, data, err := c1.Read(context.Background())
			if err != nil {
				return
			}
			if len(data) > 0 && data[0] == proto.Output {
				acc = append(acc, data[1:]...)
				if bytes.Contains(acc, []byte("READY")) {
					ready <- struct{}{}
					return
				}
			}
		}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("helper READY not observed within 5s — winch helper not armed")
	}

	// 处理器就位后 attach c2（同尺寸 80x24）：其 attach 完成的显式 SIGWINCH 必然
	// 送达前台进程组 → helper 落盘标记。轮询 5s 断言标记文件出现。
	c2, _ := dialHello(t, ctx, wsURL, 80, 24)
	defer c2.CloseNow()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return // GOT_WINCH 落盘——D-11 送达证据齐全
		}
		if time.Now().After(deadline) {
			t.Fatal("GOT_WINCH marker not created within 5s of second attach — SignalForegroundGroup not delivered")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ====== plan 05-03 增量：MULTI-02 写权限体系测试组（VALIDATION 05-01-02/05-01-07）======

// TestOwnerPolicy（VALIDATION 05-01-02，owner 模式升降级矩阵行）：--writable +
// write-policy=owner → 首个 rw attach（A）成为 owner（Welcome mode=rw）；后续 rw
// attach（B）D-07 降级 ro 进递补队列（Welcome mode=ro）；B 的 INPUT 被服务端真
// 边界丢弃、A 的 INPUT 生效；D-13 prefs 双档：A 的 Welcome prefs 含 osc52 键、
// B 的不含（prefs JSON 键存在性断言，不解析值——T-05-07 旁观者剪贴板防线）。
// 无认证模式即可覆盖矩阵核心行（ticket mode 由全局 writable 派生；认证模式 token
// 绑定通道由 05-06 sharetoken_test.go 与既有 dialHelloTicket 覆盖）。
func TestOwnerPolicy(t *testing.T) {
	// prefs 双档注入模拟 main aggregateClientPrefs 在全局 --osc52 开启下的产出
	//（ro 档永不含 osc52 键 / rw 档按全局下发——D-13 + P5-6）。
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:      true,
		WritePolicy:   server.WritePolicyOwner,
		ClientPrefsRO: json.RawMessage(`{"fontSize":14}`),
		ClientPrefsRW: json.RawMessage(`{"fontSize":14,"osc52":true}`),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// A：首个 rw attach → 立 owner（D-06）：Welcome mode=rw + rw 档 prefs。
	cA, wmA := dialHelloPayload(t, ctx, wsURL, 80, 24)
	if wmA["mode"] != proto.ModeRW {
		t.Fatalf("A welcome mode = %v, want %q (首个 rw attach 立 owner)", wmA["mode"], proto.ModeRW)
	}
	prefsA, ok := wmA["prefs"].(map[string]any)
	if !ok {
		t.Fatalf("A welcome prefs = %v, want JSON object（rw 档注入）", wmA["prefs"])
	}
	if _, present := prefsA["osc52"]; !present {
		t.Errorf("A(rw owner) prefs = %v, want osc52 key present（rw 档按全局 --osc52 下发）", prefsA)
	}
	// B：后续 rw attach → D-07 降级 ro 进递补队列：Welcome mode=ro + ro 档 prefs
	//（osc52 键强制缺席——即使全局 --osc52 开启）。
	cB, wmB := dialHelloPayload(t, ctx, wsURL, 132, 43)
	if wmB["mode"] != proto.ModeRO {
		t.Fatalf("B welcome mode = %v, want %q (D-07 降级)", wmB["mode"], proto.ModeRO)
	}
	prefsB, ok := wmB["prefs"].(map[string]any)
	if !ok {
		t.Fatalf("B welcome prefs = %v, want JSON object（ro 档注入）", wmB["prefs"])
	}
	if _, present := prefsB["osc52"]; present {
		t.Errorf("B(ro 降级) prefs = %v, must not contain osc52 key（D-13：旁观者强制不下发）", prefsB)
	}

	// B（ro 降级端）INPUT 被服务端真边界丢弃（per-client mode 门）：先发 B 的
	// 标记串并 pacing 300ms（保证服务端读循环已处理该帧——若泄漏，cat 回显必先于
	// A 的回显出现在扇出流），再发 A 的标记串；A 端输出视角断言。
	sendInput(t, ctx, cB, []byte("b-must-be-dropped"))
	time.Sleep(300 * time.Millisecond)
	sendInput(t, ctx, cA, []byte("a-owner-writes"))
	want := []byte("a-owner-writes")
	got := make([]byte, 0, len(want))
	for len(got) < len(want) {
		_, data, err := cA.Read(ctx)
		if err != nil {
			t.Fatalf("A read OUTPUT: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("A unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
		if bytes.Contains(got, []byte("b-must-be-dropped")) {
			t.Fatalf("ro 降级端 INPUT 泄漏进 master：A 扇出流含 B 标记串 %q（per-client mode 门失效）", got)
		}
	}
	if string(got) != string(want) {
		t.Fatalf("A echo payload = %q, want %q（owner INPUT 生效）", got, want)
	}

	cA.Close(websocket.StatusNormalClosure, "")
	cB.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestAllPolicy（VALIDATION 05-01-02，all 模式矩阵行）：--writable +
// write-policy=all → A/B 均 Welcome mode=rw（全员可写，协作排障形态）；两端
// INPUT 均生效（cat 回显双端扇出收齐）；无递补概念——A 断开后 B 保持 rw，
// 无升格帧（B 端读到的下一帧是 OUTPUT 而非 Welcome）。
func TestAllPolicy(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:    true,
		WritePolicy: server.WritePolicyAll,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
	cB, modeB := dialHello(t, ctx, wsURL, 132, 43)
	if modeA != proto.ModeRW || modeB != proto.ModeRW {
		t.Fatalf("welcome modes = %q / %q, want both %q（all 模式全员可写）", modeA, modeB, proto.ModeRW)
	}

	// 两端 INPUT 均生效：各自标记串经 cat 回显扇出，双端各自收齐。
	sendInput(t, ctx, cA, []byte("a-all-writes"))
	accumPayload(t, ctx, cA, []byte("a-all-writes"))
	accumPayload(t, ctx, cB, []byte("a-all-writes"))
	sendInput(t, ctx, cB, []byte("b-all-writes"))
	accumPayload(t, ctx, cA, []byte("b-all-writes"))
	accumPayload(t, ctx, cB, []byte("b-all-writes"))

	// 无递补概念：A 断开后 B 保持 rw 且无升格帧——B 发 INPUT 驱动输出，其读到
	// 的下一帧须为 OUTPUT 回显（accumPayload 对任何非 OUTPUT 帧 fatal，升格
	// Welcome 在此天然被断言缺席）。
	cA.CloseNow()
	sendInput(t, ctx, cB, []byte("b-still-rw"))
	accumPayload(t, ctx, cB, []byte("b-still-rw"))

	cB.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestSuccession（VALIDATION 05-01-02，owner FIFO 递补升格）：owner 模式
// A(owner rw)/B(降级 ro) → A CloseNow → B 在轮询窗口内收到第二帧 Welcome
// mode=rw（R-09 升格推送复用 'W' 帧）且 prefs 含 osc52 键（P5-6：升格必携
// rw 档——升格即获 osc52，D-13 的另一半）→ B 的 INPUT 此后生效。
// C 场景：owner 断开且无可递补者（注册表空）→ 后续新 rw attach 直接成为 owner。
func TestSuccession(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:      true,
		WritePolicy:   server.WritePolicyOwner,
		ClientPrefsRO: json.RawMessage(`{"fontSize":14}`),
		ClientPrefsRW: json.RawMessage(`{"fontSize":14,"osc52":true}`),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
	if modeA != proto.ModeRW {
		t.Fatalf("A welcome mode = %q, want %q", modeA, proto.ModeRW)
	}
	cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
	if modeB != proto.ModeRO {
		t.Fatalf("B welcome mode = %q, want %q（D-07 降级进递补队列）", modeB, proto.ModeRO)
	}

	// owner 断开 → B 在 5s 轮询窗口内收升格 Welcome mode=rw 且携 rw 档 prefs
	//（osc52 键存在性断言）。cat 无输出，B 端首帧即升格帧。
	cA.CloseNow()
	select {
	case wm := <-readUntilWelcome(cB):
		if wm == nil {
			t.Fatal("B read terminated before promotion Welcome（升格推送丢失）")
		}
		if wm["mode"] != proto.ModeRW {
			t.Fatalf("promotion Welcome mode = %v, want %q（R-09 升格推送）", wm["mode"], proto.ModeRW)
		}
		prefs, ok := wm["prefs"].(map[string]any)
		if !ok {
			t.Fatalf("promotion Welcome prefs = %v, want JSON object（P5-6 升格携 rw 档）", wm["prefs"])
		}
		if _, present := prefs["osc52"]; !present {
			t.Errorf("promotion Welcome prefs = %v, want osc52 key present（升格即获 osc52，D-13 另一半）", prefs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("B 未在 5s 内收到升格 Welcome——owner 断开后 FIFO 递补失败")
	}
	// 升格后 B 的 INPUT 生效（per-client mode 已翻转 rw）。
	sendInput(t, ctx, cB, []byte("b-promoted-writes"))
	accumPayload(t, ctx, cB, []byte("b-promoted-writes"))

	// C 场景：B（现任 owner，无可递补者——A 已走）断开 → 注册表空、owner=nil
	// → 新 rw attach 按矩阵直接成为 owner（Welcome mode=rw）。
	cB.CloseNow()
	cC, modeC := dialHello(t, ctx, wsURL, 80, 24)
	if modeC != proto.ModeRW {
		t.Fatalf("C welcome mode = %q, want %q（无可递补者时新 rw attach 立为 owner）", modeC, proto.ModeRW)
	}

	cC.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestSuccessionKickRace（VALIDATION 05-01-07，review #3 时序闭合竞态锁定）：
// stall owner 被服务端移除后的晋升/重连时序闭合——晋升恒在 hubMu 内同步完成，
// 必然先于旧 owner 任何重连的 registerLocked；重连旧 owner 归队 FIFO 尾；
// 全程单 owner；再递补链完整。
//
// 触发形态（SUMMARY Deviation 登记）：plan 字面「owner stall 被 1013 踢出」在
// R-08 分工表下结构性不可达——owner 模式 owner 恒为唯一可写端，其 outbox 写满
// 走信用门（creditBlocked 闭门）而非踢出（『剔除 c 后仍存在未 blocked 的可写端』
// 对唯一可写端恒假）。服务端主动移除 stall owner 的唯一可达路径是 pinger
// pong_timeout 收口（stall = 不 Read = 不答 pong）→ detach → promoteNextLocked
// 第一调用点——与 kick 路径第二调用点共享同一 hubMu 时序闭合论证（两路径均在
// removeLocked 后同一 hubMu 持有内同步晋升），四断言同款锁定；kick 路径调用点
// 保留为防御性挂点（未来策略形态变化的防线）。
func TestSuccessionKickRace(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
		Writable:    true,
		WritePolicy: server.WritePolicyOwner,
		// 短 ping/pong 参数加速 stall owner 的 pong_timeout 收口（生产 5s/10s
		// 在测试窗口内不可行；2s PongTimeout 同时保证正常读取的 cB/cA2 不被误伤——
		// 测试内读间隔均为毫秒级，远小于 pong 窗口）。
		PingInterval: 100 * time.Millisecond,
		PongTimeout:  2 * time.Second,
	})
	_ = exitCh // 本测试不断言子进程退出

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
	if modeA != proto.ModeRW {
		t.Fatalf("A welcome mode = %q, want %q", modeA, proto.ModeRW)
	}
	cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
	if modeB != proto.ModeRO {
		t.Fatalf("B welcome mode = %q, want %q（D-07 降级）", modeB, proto.ModeRO)
	}
	defer cB.CloseNow()

	// cA 自此不再 Read（stall owner）：不答 pong → ~2.1s 内 pinger pong_timeout
	// CloseNow → 服务端 reader 终结 → detach → promoteNextLocked 同步晋升 cB。
	// 断言一（晋升）：cB 在 5s 轮询窗口内收升格 Welcome mode=rw。
	select {
	case wm := <-readUntilWelcome(cB):
		if wm == nil {
			t.Fatal("B read terminated before promotion Welcome（stall owner 移除后升格推送丢失）")
		}
		if wm["mode"] != proto.ModeRW {
			t.Fatalf("promotion Welcome mode = %v, want %q", wm["mode"], proto.ModeRW)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("B 未在 5s 内收到升格 Welcome——stall owner 移除后晋升丢失")
	}
	// cB 保持 rw：INPUT 生效（cat 回显）。
	sendInput(t, ctx, cB, []byte("b-promoted-after-stall"))
	accumPayload(t, ctx, cB, []byte("b-promoted-after-stall"))

	// 断言二（重连归队）：旧 owner 立即重连——晋升已在 hubMu 内先于任何
	// registerLocked 完成（时序闭合），cA2 按矩阵降级 ro 归队 FIFO 尾。
	cA.CloseNow()
	cA2, modeA2 := dialHello(t, ctx, wsURL, 80, 24)
	if modeA2 != proto.ModeRO {
		t.Fatalf("re-connected old owner welcome mode = %q, want %q（新 owner 在位，归队 FIFO 尾）", modeA2, proto.ModeRO)
	}
	defer cA2.CloseNow()

	// 断言三（全程单 owner）：cA2 在窗口内不再收第二帧 Welcome——cB 发 INPUT
	// 驱动扇出，cA2 收齐回显期间任何升格 Welcome 都会使 accumPayload fatal；
	// 同时证明归队 ro 端正常收 fan-out 流。
	sendInput(t, ctx, cB, []byte("b-still-owner"))
	accumPayload(t, ctx, cA2, []byte("b-still-owner"))

	// 断言四（再递补）：cB CloseNow → cA2（rwEligible 归队者）收升格 Welcome
	// mode=rw——归队重排后递补链完整。
	cB.CloseNow()
	select {
	case wm := <-readUntilWelcome(cA2):
		if wm == nil {
			t.Fatal("A2 read terminated before re-succession Welcome（递补链断裂）")
		}
		if wm["mode"] != proto.ModeRW {
			t.Fatalf("re-succession Welcome mode = %v, want %q", wm["mode"], proto.ModeRW)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("A2 未在 5s 内收到升格 Welcome——归队重排后递补链断裂")
	}
	// 升格后 cA2 INPUT 生效。
	sendInput(t, ctx, cA2, []byte("a2-promoted"))
	accumPayload(t, ctx, cA2, []byte("a2-promoted"))
}

// ====== plan 05-05 增量：RES-02 每客户端输入限速测试（VALIDATION 05-01-05）======

// TestInputRateLimit（VALIDATION 05-01-05，RES-02 + 裁断 R-02 丢弃语义 + CR-01
// 队列化行为锁定）：Options 覆写 InputRate=1024（1KiB/s）+ InputBurst=1024（1KiB）
// 加速；Hello 完成后连续发送总计 64KiB INPUT（128 帧 × 512B 载荷——帧长 ≤ burst
// 才有帧可通过：AllowN 对 n > burst 恒 false，rate.go godoc）→ 读 OUTPUT 累积
// 回显 → 断言三件套：
//  1. 送达回显 'x' 字节数 > 0（未超限部分送达——burst 内容纳的首批 2 帧）；
//  2. 送达字节数显著小于发送量（< 发送量的 25%——超限部分被丢弃。宽界时序论证：
//     令牌桶 1KiB/s refill，送达达 25%（32 帧）需 ~15s 连续 refill，而收集窗口
//     仅 1.5s（窗内至多 ~5 帧）——窗口滑动与调度迟滞的时序不确定性由宽界免疫）；
//  3. 连接存活（洪水后 Ping 收 pong——未被踢未 1011；refill 1.2s 后发小量
//     marker INPUT 有回显——输入路径未降权）。
// 对照子测：InputRate/InputBurst 调大（1MiB/1MiB）下同量 INPUT 全量送达（'x'
// 计数精确 == 发送量——证明丢弃确由限速器而非队列/其他路径）。
//
// 回显计数模型：/bin/cat 默认 canonical+ECHO——每送达一帧产生双份 'x'（行规
// ECHO 即时回显 + cat 读后 stdout 拷贝），ONLCR 只把 '\n' 展开为 \r\n、不影响
// 'x' 计数；每帧 511 'x' × 2 份 = 1022。
func TestInputRateLimit(t *testing.T) {
	// 帧载荷：511 'x' + '\n' = 512B（'\n' 使 canonical 行完结、cat 可读后拷贝；
	// 行长 512B ≪ MAX_CANON 4096 不触顶）。
	frame := append(bytes.Repeat([]byte{'x'}, 511), '\n')
	const frames = 128        // 总计 64KiB INPUT
	const xPerFrame = 2 * 511 // 行规 ECHO + cat 拷贝双份
	const sentX = frames * xPerFrame

	// outputAccumulator 形态（Pitfall 2：客户端 Read 永不带 deadline ctx）：
	// goroutine 持续读 OUTPUT 把载荷累积进互斥锁保护的缓冲，主测试经快照轮询
	// 收口；conn 关闭后 Read 出错 goroutine 自终结。
	accum := func(c *websocket.Conn) (snapshot func() []byte) {
		var mu sync.Mutex
		var buf []byte
		go func() {
			for {
				_, data, err := c.Read(context.Background())
				if err != nil {
					return
				}
				if len(data) > 0 && data[0] == proto.Output {
					mu.Lock()
					buf = append(buf, data[1:]...)
					mu.Unlock()
				}
			}
		}()
		return func() []byte {
			mu.Lock()
			defer mu.Unlock()
			return append([]byte(nil), buf...)
		}
	}

	t.Run("超限丢弃且连接存活", func(t *testing.T) {
		exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:   true,
			InputRate:  1024, // 1KiB/s
			InputBurst: 1024, // 1KiB burst——恰容纳首批 2 帧（512B/帧）
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c, mode := dialHello(t, ctx, wsURL, 80, 24)
		if mode != proto.ModeRW {
			t.Fatalf("welcome mode = %q, want %q（owner 默认策略单客户端立 owner）", mode, proto.ModeRW)
		}
		defer c.CloseNow()
		snap := accum(c)

		// Hello 完成后连续发送 64KiB INPUT 洪水（128 帧 × 512B）。
		for i := 0; i < frames; i++ {
			sendInput(t, ctx, c, frame)
		}

		// 1.5s 收集窗口：窗内可送达 = burst 首批 2 帧 + refill（1 帧/0.5s × 1.5s
		// ≈ 3 帧）≈ 5 帧（'x' 上限 5×1022=5110，占发送量 ~3.9%）。
		time.Sleep(1500 * time.Millisecond)
		got := bytes.Count(snap(), []byte{'x'})
		if got == 0 {
			t.Fatal("未收到任何回显——burst 内容纳的首批帧未送达（限速器误杀合法输入）")
		}
		if got >= sentX/4 {
			t.Fatalf("回显 'x' = %d, want < %d（发送量 25%%）——超限帧未被限速器丢弃", got, sentX/4)
		}

		// 存活断言一：洪水后 Ping 收 pong（未被踢未 1011；库硬性要求 Ping 与
		// Reader 并发——accum goroutine 即并发 reader，conn.go:218-220）。
		if err := c.Ping(ctx); err != nil {
			t.Fatalf("洪水后 Ping: %v——连接被限速路径断开（R-02：超限唯一动作是丢弃）", err)
		}
		// 存活断言二：refill 1.2s（令牌回填 ≥1KiB）后发小量 marker INPUT，回显
		// 送达——输入路径未降权。
		time.Sleep(1200 * time.Millisecond)
		sendInput(t, ctx, c, []byte("tail-ok\n"))
		deadline := time.Now().Add(5 * time.Second)
		for {
			if bytes.Contains(snap(), []byte("tail-ok")) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("marker 回显未达——洪水后输入路径未恢复（连接存活但输入失效）")
			}
			time.Sleep(20 * time.Millisecond)
		}
		// 洪水不拖死会话：服务端零退出（RES-02 是资源保护非策略违例）。
		assertNoExit(t, exitCh)
	})

	t.Run("对照大限额全量送达", func(t *testing.T) {
		_, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{
			Writable:   true,
			InputRate:  1 << 20, // 1MiB/s
			InputBurst: 1 << 20, // 1MiB burst——整批 64KiB 洪水一次性容纳
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c, _ := dialHello(t, ctx, wsURL, 80, 24)
		defer c.CloseNow()
		snap := accum(c)

		for i := 0; i < frames; i++ {
			sendInput(t, ctx, c, frame)
		}
		// 全量送达精确断言：'x' 计数 == 发送量（128 帧 × 1022）；证明限速子测的
		// 丢弃确由限速器而非 inputQ/其他路径（队列 256KiB ≫ 64KiB 洪水上限）。
		deadline := time.Now().Add(10 * time.Second)
		for {
			got := bytes.Count(snap(), []byte{'x'})
			if got == sentX {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("回显 'x' = %d, want %d（大限额下同量 INPUT 应全量送达）", got, sentX)
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
}
