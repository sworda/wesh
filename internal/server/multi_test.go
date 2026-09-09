package server_test

// multi_test.go 锁定 MULTI-01 主干与多客户端生命周期推论（VALIDATION
// 05-W0-01/05-01-01）：fan-out 两端逐字节一致 / 断开不退出且再 attach 成功 /
// 子进程退出广播 1000 + exitf。helper 复用 e2e_test.go 零改动（PATTERNS exact
// 先例：startTestServerWith/dialHello/waitExit/assertNoExit 同包直接复用）。
//
// 14-02 双模式断言分叉表改造（D-01/D-02/D-03，蓝本 PITFALLS :386 multi 行
// 「fanout→各自进程输出；容量→spawn-intent 口径（P4）」+ :392 shared-only 行）：
//   - fanout/EXIT 广播两族真值两模式相反（shared=全员同帧 / per-client=各自
//     进程输出、属主私有化）——分叉表显式成表，严禁和稀泥（P11 警告征象）。
//   - owner 四测（TestOwnerPolicy/TestAllPolicy/TestSuccession/
//     TestSuccessionKickRace）per-client 列走 D-03 显式断言未装配——各端权限由
//     自身 ticket/writable 决定（upgradePerClient 单行门 perclient.go:159-164
//     不查 write-policy），owner 断开/移除不触发任何递补行为（他端权限/角色
//     零变化、零升格 Welcome 帧），可证伪（误装配递补即静默窗断言翻红），否决
//     t.Skip。蓝本 shared-only 行「owner 递补（clients_test 大部）」的实测归属
//     修正：四测实在本文件（非 clients_test）——D-02 偏差登记进 SUMMARY。
//   - MaxClients per-client 列补 spawn-intent 口径（P4/D-03 复检回收）：503 闸
//     先于 spawn——被拒 dial 零 spawn，wesh_pty_spawn_total 与成功 attach 数
//     程序序精确对照（metrics 通道，churn 格先例形态）。
//   - 同断言测（TestDetach/TestInputRateLimit/TestMaxClients503 早闸双通道）两列
//     同值双跑，测试体内注释载明归属依据（蓝本行号）。
// shared 列期望值与改造前逐字一致（D-02 红线；git diff 自审白名单 = 结构包裹
// 与分叉表新增）。夹具纪律：客户端 Read 永不带 per-read deadline——静默窗一律
// select + time.After 竞速（perclient_test.go:13-14 头注释纪律）。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
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
//
// 14-02 双模式断言分叉表（D-01/D-02，蓝本 :386 multi 行「fanout→各自进程输出」
// ——MULTI-01 语义的反转面，和稀泥即证据毁灭）：
//
//	fanout shared（v1.0 逐字）= 输出逐字节一致扇出（gotA == gotB == payload）。
//	fanout per-client = 各端输出即自身进程输出、互不串台——双标记串交叉断言
//	  （A 发 FANOUT_MARK_A / B 发 FANOUT_MARK_B，各自回读自身标记逐字节精确，
//	  对方标记缺席：A 的标记先发，若串台必落在 B 累积流前缀使精确比对翻红；
//	  B 的标记后发，若串台必落在 A 的尾窗帧——双泵 1s 静默窗帧级累积交叉断言，
//	  免疫标记跨帧切分）。
func TestMultiClientFanout(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			// WritePolicy=all 两列同 mutate 镜像（per-client 无仲裁面，
			// upgradePerClient 单行门不查 write-policy——行为无差，14-01
			// TestSlowConsumerKick 同款注记）。
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.WritePolicy = server.WritePolicyAll
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
			cB, modeB := dialHello(t, ctx, wsURL, 132, 43)
			// 两端各自 Welcome mode 一致（无认证 --writable 均 rw；per-client
			// 同值——各端权限由自身 writable 决定，无 owner 降级面）。
			if modeA != proto.ModeRW || modeB != proto.ModeRW {
				t.Fatalf("welcome modes = %q / %q, want both %q", modeA, modeB, proto.ModeRW)
			}

			if mode == server.SessionModePerClient {
				// per-client 列：双标记串交叉断言（分叉表见上）。
				markerA := []byte("FANOUT_MARK_A_7q2")
				markerB := []byte("FANOUT_MARK_B_4k8")
				sendInput(t, ctx, cA, markerA)
				accumPayload(t, ctx, cA, markerA) // A 自有进程回显（逐字节精确比对）
				// B 发自身标记并自收：A 的标记若串台到 B，B 的累积流以其为前缀
				// 必然失配翻红（accumPayload 逐字节精确比对）。
				sendInput(t, ctx, cB, markerB)
				accumPayload(t, ctx, cB, markerB)
				// 双泵尾窗交叉断言：任何一方标记串台到对方，必落在其后续帧——
				// 1s 静默窗（select + time.After 竞速，夹具纪律红线）双泵并发
				// 收集，帧级累积后 Contains（免疫标记跨帧切分）。
				resA := make(chan frameRes, 8)
				resB := make(chan frameRes, 8)
				quit := make(chan struct{})
				defer close(quit)
				go readPump(ctx, cA, resA, quit)
				go readPump(ctx, cB, resB, quit)
				var accA, accB []byte
				window := time.After(1 * time.Second)
			crossWin:
				for {
					select {
					case r := <-resA:
						if r.err != nil {
							t.Fatalf("A 静默窗内 read error: %v（连接应存活）", r.err)
						}
						if len(r.data) > 0 && r.data[0] == proto.Output {
							accA = append(accA, r.data[1:]...)
						}
					case r := <-resB:
						if r.err != nil {
							t.Fatalf("B 静默窗内 read error: %v（连接应存活）", r.err)
						}
						if len(r.data) > 0 && r.data[0] == proto.Output {
							accB = append(accB, r.data[1:]...)
						}
					case <-window:
						break crossWin
					}
				}
				if bytes.Contains(accA, markerB) {
					t.Fatalf("A 静默窗收到 B 标记 %q——per-client fanout 串台（共享扇出误装配）: %q", markerB, accA)
				}
				if bytes.Contains(accB, markerA) {
					t.Fatalf("B 静默窗收到 A 标记 %q——per-client fanout 串台（共享扇出误装配）: %q", markerA, accB)
				}
				cA.Close(websocket.StatusNormalClosure, "")
				cB.Close(websocket.StatusNormalClosure, "")
				assertNoExit(t, exitCh)
				return
			}

			// shared 列（v1.0 期望值逐字搬入）：
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
		})
	}
}

// TestDetach（VALIDATION 05-W0-01，多客户端生命周期推论）：任一客户端断开不再触发
// exitf——exitCh 200ms 静默 + 其他客户端继续 echo 正常 + 断开者立即重新 attach
// 成功（注册表移除断言的行为化；P1 D-11 单次语义终结，服务端生命周期只随子进程）。
// 05-03 适配：显式 WritePolicy=all——本测试锁定的是断开生命周期语义而非权限语义；
// owner 默认策略下 B 会被降级 ro（INPUT 静默丢）且 A 再 attach 归队 ro，两处断言
// 前提不再成立（owner 降级/递补/归队行为由 TestOwnerPolicy/TestSuccession 专测）。
// 14-02 双模式（蓝本 :386 multi 行 fanout 面的断开推论——同断言判定）：本测锁定
// 「断开者对在线客户端零影响 + 断开者可再 attach」两模式同构——per-client 下 A
// 断开仅 SIGHUP 其自有会话（断开语义分叉本体由 e2e_test.go TestEchoPTY
// per-client 列承载）、B 自有会话零影响、A 再 attach 为全新进程且 mode 判定
// 同值 rw——全部原断言两模式同值，不双写（T-14-04 防双写漂移）。
func TestDetach(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.WritePolicy = server.WritePolicyAll
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
		})
	}
}

// TestExitBroadcast（VALIDATION 05-W0-01，D-10 唯一终结路径的多客户端形态）：
// 子进程退出是唯一终结路径——lifecycle Wait → Drain → 并行广播 1000 关闭全部
// 客户端（双端均收 CloseError 1000）→ exitf 以子进程退出码收口（退出码传递
// 语义不变）。
//
// 14-02 双模式断言分叉表（D-01/D-02，蓝本 :385 exit 行「shared=全员广播；
// per-client=属主私有化 + 他端零感知（PC-04）」的 MULTI 形态）：
//
//	broadcast shared（v1.0 逐字）= 双端均收 1000 + exitf(3)。
//	broadcast per-client = A（属主）收 1000（自有会话终结），B 静默窗零帧
//	  零错误（他端零感知），窗后 B 发一行触发自身 sh 退出 → B 收自己的 1000
//	  （自有会话独立存活的行为化证明——shared 列无此面）；exitf 静默（服务端
//	  续跑）。EXIT 帧细节（exit_code/文案/信号名）由 perclient_test.go
//	  TestPerClientExitPrivate42 单一承载，本列不双写（T-14-04）。
func TestExitBroadcast(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			// sh 读一行后以 3 退出：保证广播时两客户端均已 attach（消除子进程抢在 Dial
			// 完成前退出的竞态——wesh-helper-exit42 同款编排，此处无需专用 helper）。
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/sh", "-c", "read x; exit 3"}, nil)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cA, _ := dialHello(t, ctx, wsURL, 80, 24)
			cB, _ := dialHello(t, ctx, wsURL, 80, 24)

			// 触发子进程退出：A 发一行（INPUT 经 master 送达 sh stdin，规范模式行缓冲）。
			if err := cA.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
				t.Fatalf("write INPUT on A: %v", err)
			}

			if mode == server.SessionModePerClient {
				// per-client 列：A 属主私有收口（读取循环原形态——途中帧丢弃）。
				var ceA websocket.CloseError
				for {
					if _, _, rerr := cA.Read(ctx); rerr != nil {
						if !errors.As(rerr, &ceA) {
							t.Fatalf("A read terminated without CloseError: %v", rerr)
						}
						break
					}
				}
				if ceA.Code != websocket.StatusNormalClosure {
					t.Fatalf("A close code = %d, want %d (1000)", ceA.Code, websocket.StatusNormalClosure)
				}

				// B 端 1.5s 静默窗（select + time.After 竞速，夹具纪律红线）：
				// 任何帧/错误到达即 FAIL（他端零感知强形态——A 的终结余波连
				// OUTPUT 级都不得泄漏到 B，PC-04）。
				resCh := make(chan frameRes, 8)
				quit := make(chan struct{})
				defer close(quit)
				go readPump(ctx, cB, resCh, quit)
				silent := time.After(1500 * time.Millisecond)
			bQuiet:
				for {
					select {
					case r := <-resCh:
						t.Fatalf("B 端在 A 终结后静默窗内收到帧/错误——EXIT 广播串台（per-client 私有化违反，PC-04）: data=%q err=%v", r.data, r.err)
					case <-silent:
						break bQuiet
					}
				}

				// B 自有会话独立存活的行为化证明：B 发一行触发自身 sh exit 3 →
				// 经泵源读至 CloseError 1000（会话无扰动 + 私有收口同链）。
				if err := cB.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'y', '\n'}); err != nil {
					t.Fatalf("write INPUT on B: %v", err)
				}
				for r := range resCh {
					if r.err != nil {
						var ceB websocket.CloseError
						if !errors.As(r.err, &ceB) {
							t.Fatalf("B read terminated without CloseError: %v（自身会话退出应收口帧）", r.err)
						}
						if ceB.Code != websocket.StatusNormalClosure {
							t.Fatalf("B close code = %d, want %d (1000)", ceB.Code, websocket.StatusNormalClosure)
						}
						break
					}
				}
				assertNoExit(t, exitCh) // 服务端续跑（exitf 唯二触发源不在场）
				return
			}

			// shared 列（v1.0 期望值逐字搬入）：
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
		})
	}
}

// TestSigwinchOnAttach（D-11 送达证据）：新客户端 attach 完成时服务端向 PTY 前台
// 进程组显式发一次 SIGWINCH——helper 收到信号后落盘标记文件（GOT_WINCH）。
// 两端同尺寸 80x24：排除内核 TIOCSWINSZ 异尺寸发信号的干扰（P5-3 本机实证 Linux
// 同尺寸不发信号）——标记出现即显式 SignalForegroundGroup 送达的证据，而非
// resize 副作用。同步纪律：helper 先从 stdin 读一字节再装处理器报 READY，c1 发
// INPUT 驱动并回读 READY 确认处理器就位，c2 attach 触发第二次信号——消除
// 「attach 信号先于处理器安装被默认忽略」的竞态。
// 本测试在 CI macos runner 同样执行（.github/workflows/ci.yml 双平台矩阵既定）——
// darwin 同尺寸行为假设 A1 的验证通道（review MEDIUM 项处置：以 CI 双平台运行
// 实证替代本机平台断言；即便 darwin 同尺寸发信号，显式 SIGWINCH 也只是冗余无害）。
//
// 14-02 双模式断言分叉表（D-03）：attach 期显式 SIGWINCH 为 shared-only 装配
// （server.go:1098-1104——per-client 升档分支不调用序列尾部 SignalForegroundGroup；
// 子进程以 Hello 钳制尺寸出生即正确，无重绘需求，研究 §1.2）。shared 列 =
// 上述 v1.0 断言逐字；per-client 列 = 显式断言未装配——c2 attach 后 c1 已武装
// 的处理器不得收到信号（标记文件 2s 窗口内缺席；可证伪面 = shared 尾段（含向
// 既有会话发信号）被误复制进 per-client 升档或误向既有会话补发信号，标记即
// 落盘翻红；直接 s.sess 引用的整段复制在 per-client 会 nil deref 被任意测捕获）
// + c2 新会话功能性自证（INPUT 携 '\n' → READY 回读——出生即正确尺寸 ≠ 会话
// 不可用）。
func TestSigwinchOnAttach(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "got_winch")
			_, wsURL := newTestServer(t, mode, helperArgv(t, "wesh-helper-winch", marker), nil)

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

			if mode == server.SessionModePerClient {
				// per-client 列（D-03 显式断言未装配）：c2 attach（自有新会话出生
				// 即正确尺寸）→ c1 已武装的处理器不得收到任何 SIGWINCH——标记文件
				// 2s 窗口内缺席（误装配面翻红论证见分叉表注释）。
				c2, _ := dialHello(t, ctx, wsURL, 80, 24)
				defer c2.CloseNow()
				deadline := time.Now().Add(2 * time.Second)
				for {
					if _, err := os.Stat(marker); err == nil {
						t.Fatal("GOT_WINCH marker created after second attach——per-client 不得向既有会话发 SIGWINCH（attach 期显式信号为 shared-only 装配红线，D-03）")
					}
					if time.Now().After(deadline) {
						break
					}
					time.Sleep(50 * time.Millisecond)
				}
				// c2 新会话功能性自证：INPUT 携 '\n' 武装 c2 自身 helper → READY 回读
				//（spawn + 输入输出链全通；同 c1 的 READY 读取形态）。
				if err := c2.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
					t.Fatalf("write INPUT to arm c2 helper: %v", err)
				}
				ready2 := make(chan struct{}, 1)
				go func() {
					var acc []byte
					for {
						_, data, err := c2.Read(context.Background())
						if err != nil {
							return
						}
						if len(data) > 0 && data[0] == proto.Output {
							acc = append(acc, data[1:]...)
							if bytes.Contains(acc, []byte("READY")) {
								ready2 <- struct{}{}
								return
							}
						}
					}
				}()
				select {
				case <-ready2:
				case <-time.After(5 * time.Second):
					t.Fatal("c2 helper READY not observed within 5s——新会话输入输出链不可用")
				}
				return
			}

			// shared 列（v1.0 期望值逐字搬入）：
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
		})
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
//
// 14-02 双模式断言分叉表（D-03，蓝本 :392 shared-only 行「owner 递补」的实测
// 归属修正：本测实在 multi_test.go 而非蓝本所记 clients_test——D-02 偏差登记）：
// owner 指派/降级组件在 per-client 不装配（upgradePerClient 单行门
// perclient.go:159-164 不查 write-policy，registry.owner 恒 nil）。shared 列 =
// 上述 v1.0 断言逐字；per-client 列 = 显式断言未装配——B 恒 rw（无 D-07 降级，
// 可证伪：降级逻辑误装配即 mode 断言翻红）+ rw 档 prefs 照发（osc52 在场——
// 档位由生效 mode 选，与降级无关）+ B 的 INPUT 生效（无 per-client mode 门降级
// 面；shared 列为服务端真边界丢弃——分叉）。
func TestOwnerPolicy(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			// prefs 双档注入模拟 main aggregateClientPrefs 在全局 --osc52 开启下的产出
			//（ro 档永不含 osc52 键 / rw 档按全局下发——D-13 + P5-6；两列同 mutate）。
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.WritePolicy = server.WritePolicyOwner
				o.ClientPrefsRO = json.RawMessage(`{"fontSize":14}`)
				o.ClientPrefsRW = json.RawMessage(`{"fontSize":14,"osc52":true}`)
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// A：首个 rw attach → 立 owner（D-06）：Welcome mode=rw + rw 档 prefs。
			//（per-client 同值——各端权限由自身 writable 决定，A 亦 rw。）
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
			// B：分叉表——shared（v1.0 逐字）= D-07 降级 ro 进递补队列（Welcome
			// mode=ro + ro 档 prefs，osc52 键强制缺席）；per-client（D-03）= 恒 rw
			// + rw 档 prefs（osc52 在场——无降级面）。
			cB, wmB := dialHelloPayload(t, ctx, wsURL, 132, 43)
			if mode == server.SessionModePerClient {
				if wmB["mode"] != proto.ModeRW {
					t.Fatalf("B welcome mode = %v, want %q（per-client 无 owner 降级——各端权限由自身 ticket 决定，D-03）", wmB["mode"], proto.ModeRW)
				}
				prefsB, ok := wmB["prefs"].(map[string]any)
				if !ok {
					t.Fatalf("B welcome prefs = %v, want JSON object（rw 档注入——生效 mode=rw）", wmB["prefs"])
				}
				if _, present := prefsB["osc52"]; !present {
					t.Errorf("B(rw) prefs = %v, want osc52 key present（per-client 无 ro 降级档——D-03 未装配红线）", prefsB)
				}
				// B 的 INPUT 生效（无降级面——各端写自有会话；shared 列为真边界
				// 丢弃——分叉）；A 的 INPUT 生效（各端自有进程回显）。
				sendInput(t, ctx, cB, []byte("b-own-ticket-writes"))
				accumPayload(t, ctx, cB, []byte("b-own-ticket-writes"))
				sendInput(t, ctx, cA, []byte("a-own-ticket-writes"))
				accumPayload(t, ctx, cA, []byte("a-own-ticket-writes"))
				cA.Close(websocket.StatusNormalClosure, "")
				cB.Close(websocket.StatusNormalClosure, "")
				assertNoExit(t, exitCh)
				return
			}
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
		})
	}
}

// TestAllPolicy（VALIDATION 05-01-02，all 模式矩阵行）：--writable +
// write-policy=all → A/B 均 Welcome mode=rw（全员可写，协作排障形态）；两端
// INPUT 均生效（cat 回显双端扇出收齐）；无递补概念——A 断开后 B 保持 rw，
// 无升格帧（B 端读到的下一帧是 OUTPUT 而非 Welcome）。
//
// 14-02 双模式断言分叉表（D-03）：shared 列 = 上述 v1.0 断言逐字；per-client 列
// 两处分叉——①INPUT 生效面：各端只自收自身标记（shared 双端互收——fanout 分叉
// 本体由 TestMultiClientFanout per-client 列承载，此处锁 all 策略语境）；②A 断开
// 后 B 零 dims-push Welcome（per-client 无仲裁无参与集收缩——G-05-1 契约退化为
// 恒等式，recalcNow 运行期推送挂点不存在；可证伪：仲裁误装配则 B 必收推送帧即
// 静默窗断言翻红）+ B 保持 rw（INPUT 生效）。
func TestAllPolicy(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.WritePolicy = server.WritePolicyAll
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
			cB, modeB := dialHello(t, ctx, wsURL, 132, 43)
			if modeA != proto.ModeRW || modeB != proto.ModeRW {
				t.Fatalf("welcome modes = %q / %q, want both %q（all 模式全员可写）", modeA, modeB, proto.ModeRW)
			}

			// 两端 INPUT 均生效——分叉表：shared = 各自标记串经 cat 回显扇出，
			// 双端各自收齐（v1.0 逐字）；per-client = 各端只自收（无扇出面）。
			if mode == server.SessionModePerClient {
				sendInput(t, ctx, cA, []byte("a-all-writes"))
				accumPayload(t, ctx, cA, []byte("a-all-writes"))
				sendInput(t, ctx, cB, []byte("b-all-writes"))
				accumPayload(t, ctx, cB, []byte("b-all-writes"))
			} else {
				sendInput(t, ctx, cA, []byte("a-all-writes"))
				accumPayload(t, ctx, cA, []byte("a-all-writes"))
				accumPayload(t, ctx, cB, []byte("a-all-writes"))
				sendInput(t, ctx, cB, []byte("b-all-writes"))
				accumPayload(t, ctx, cA, []byte("b-all-writes"))
				accumPayload(t, ctx, cB, []byte("b-all-writes"))
			}

			// 无递补概念 + G-05-1 尺寸推送（05-10 适配）：A 断开后 B 保持 rw——all 模式
			// 无升格通道，但 detach 使参与集 2→1 收缩（min-rect {80,24} → B 单员 {132,43}
			// last-wins），recalcNow 的 last 变化分支（运行期尺寸下发唯一挂点）向 B 推送
			// 携新会话尺寸的 Welcome。断言该帧 mode 恒为 rw（无 mode 翻转 = 无升格语义的
			// 行为证据）且 cols/rows == 132/43（2→1 新会话尺寸），随后 B 的 INPUT 生效
			//（accumPayload 对任何非 OUTPUT 帧 fatal——消费推送帧后回显路径断言形态不变）。
			cA.CloseNow()
			if mode == server.SessionModePerClient {
				// per-client 列（D-03）：无仲裁无参与集收缩——B 静默窗零 Welcome 帧
				//（可证伪：仲裁/推送误装配即翻红），窗后 B 保持 rw（INPUT 生效）。
				resCh := make(chan frameRes, 8)
				quit := make(chan struct{})
				defer close(quit)
				go readPump(ctx, cB, resCh, quit)
				window := time.After(2 * time.Second)
			noPush:
				for {
					select {
					case r := <-resCh:
						if r.err != nil {
							t.Fatalf("B 静默窗内 read error: %v（连接应存活）", r.err)
						}
						if len(r.data) > 0 && r.data[0] == proto.Welcome {
							t.Fatalf("B 收到 dims-push Welcome——per-client 无仲裁无运行期尺寸推送（G-05-1 恒等式红线，D-03）: %q", r.data)
						}
					case <-window:
						break noPush
					}
				}
				sendInput(t, ctx, cB, []byte("b-still-rw"))
				accumFramesUntil(t, resCh, regexp.MustCompile("b-still-rw"))
				cB.Close(websocket.StatusNormalClosure, "")
				assertNoExit(t, exitCh)
				return
			}
			select {
			case wm := <-readUntilWelcome(cB):
				if wm == nil {
					t.Fatal("B read terminated before dims-push Welcome（G-05-1 运行期推送丢失）")
				}
				if wm["mode"] != proto.ModeRW {
					t.Fatalf("dims-push Welcome mode = %v, want %q（all 模式无升格——mode 不翻转）", wm["mode"], proto.ModeRW)
				}
				if wm["cols"] != float64(132) || wm["rows"] != float64(43) {
					t.Fatalf("dims-push Welcome dims = %vx%v, want 132x43（2→1 last-wins 会话尺寸推送）", wm["cols"], wm["rows"])
				}
			case <-time.After(5 * time.Second):
				t.Fatal("B 未在 5s 内收到 dims-push Welcome——A detach 2→1 会话尺寸变化未推送（G-05-1）")
			}
			sendInput(t, ctx, cB, []byte("b-still-rw"))
			accumPayload(t, ctx, cB, []byte("b-still-rw"))

			cB.Close(websocket.StatusNormalClosure, "")
			assertNoExit(t, exitCh)
		})
	}
}

// TestSuccession（VALIDATION 05-01-02，owner FIFO 递补升格）：owner 模式
// A(owner rw)/B(降级 ro) → A CloseNow → B 在轮询窗口内收到第二帧 Welcome
// mode=rw（R-09 升格推送复用 'W' 帧）且 prefs 含 osc52 键（P5-6：升格必携
// rw 档——升格即获 osc52，D-13 的另一半）→ B 的 INPUT 此后生效。
// C 场景：owner 断开且无可递补者（注册表空）→ 后续新 rw attach 直接成为 owner。
//
// 14-02 双模式断言分叉表（D-03）：shared 列 = 上述 v1.0 断言逐字；per-client 列
// 三处分叉——①B 恒 rw（无 D-07 降级）；②A 断开后 B 零升格 Welcome（静默窗可证
// 伪：递补误装配即翻红）+ B 的 INPUT 生效（本就 rw，权限零变化）；③C 场景两列
// 同值（注册表空 → 新 rw attach 直接 rw——per-client 无 owner 语义亦同值）。
func TestSuccession(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.WritePolicy = server.WritePolicyOwner
				o.ClientPrefsRO = json.RawMessage(`{"fontSize":14}`)
				o.ClientPrefsRW = json.RawMessage(`{"fontSize":14,"osc52":true}`)
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
			if modeA != proto.ModeRW {
				t.Fatalf("A welcome mode = %q, want %q", modeA, proto.ModeRW)
			}
			cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
			if mode == server.SessionModePerClient {
				if modeB != proto.ModeRW {
					t.Fatalf("B welcome mode = %q, want %q（per-client 无 owner 降级——各端权限由自身 writable 决定，D-03）", modeB, proto.ModeRW)
				}
			} else if modeB != proto.ModeRO {
				t.Fatalf("B welcome mode = %q, want %q（D-07 降级进递补队列）", modeB, proto.ModeRO)
			}

			// owner 断开 → 分叉表：shared（v1.0 逐字）= B 在 5s 轮询窗口内收升格
			// Welcome mode=rw 且携 rw 档 prefs（osc52 键存在性断言），升格后 B 的
			// INPUT 生效；per-client（D-03）= B 静默窗零升格帧（权限/角色零变化、
			// 无递补事件帧——可证伪）+ B 的 INPUT 生效（本就 rw）。
			cA.CloseNow()
			if mode == server.SessionModePerClient {
				resCh := make(chan frameRes, 8)
				quit := make(chan struct{})
				defer close(quit)
				go readPump(ctx, cB, resCh, quit)
				window := time.After(2 * time.Second)
			noPromo:
				for {
					select {
					case r := <-resCh:
						if r.err != nil {
							t.Fatalf("B 静默窗内 read error: %v（连接应存活）", r.err)
						}
						if len(r.data) > 0 && r.data[0] == proto.Welcome {
							t.Fatalf("B 收到升格 Welcome——owner 递补被误装配进 per-client（D-03 红线）: %q", r.data)
						}
					case <-window:
						break noPromo
					}
				}
				sendInput(t, ctx, cB, []byte("b-rw-no-promotion"))
				accumFramesUntil(t, resCh, regexp.MustCompile("b-rw-no-promotion"))
			} else {
				// cat 无输出，B 端首帧即升格帧。
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
			}

			// C 场景：B（现任 owner，无可递补者——A 已走）断开 → 注册表空、owner=nil
			// → 新 rw attach 按矩阵直接成为 owner（Welcome mode=rw；per-client 下
			// 同值 rw——无 owner 语义亦然）。
			cB.CloseNow()
			cC, modeC := dialHello(t, ctx, wsURL, 80, 24)
			if modeC != proto.ModeRW {
				t.Fatalf("C welcome mode = %q, want %q（无可递补者时新 rw attach 立为 owner）", modeC, proto.ModeRW)
			}

			cC.Close(websocket.StatusNormalClosure, "")
			assertNoExit(t, exitCh)
		})
	}
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
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
				o.WritePolicy = server.WritePolicyOwner
				// 短 ping/pong 参数加速 stall owner 的 pong_timeout 收口（生产 5s/10s
				// 在测试窗口内不可行；2s PongTimeout 同时保证正常读取的 cB/cA2 不被误伤——
				// 测试内读间隔均为毫秒级，远小于 pong 窗口；per-client pinger 同样装配
				// perclient.go:350-351，短值语义两列同构）。
				o.PingInterval = 100 * time.Millisecond
				o.PongTimeout = 2 * time.Second
			})
			_ = exitCh // 本测试不断言子进程退出

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cA, modeA := dialHello(t, ctx, wsURL, 80, 24)
			if modeA != proto.ModeRW {
				t.Fatalf("A welcome mode = %q, want %q", modeA, proto.ModeRW)
			}
			cB, modeB := dialHello(t, ctx, wsURL, 80, 24)
			if mode == server.SessionModePerClient {
				if modeB != proto.ModeRW {
					t.Fatalf("B welcome mode = %q, want %q（per-client 无 owner 降级——各端恒 rw，D-03）", modeB, proto.ModeRW)
				}
			} else if modeB != proto.ModeRO {
				t.Fatalf("B welcome mode = %q, want %q（D-07 降级）", modeB, proto.ModeRO)
			}
			defer cB.CloseNow()

			// cA 自此不再 Read（stall owner）：不答 pong → ~2.1s 内 pinger pong_timeout
			// CloseNow → 服务端 reader 终结 → detach（per-client 下另有自有会话
			// teardown SIGHUP——不影响 cB）。
			//
			// per-client 列（D-03 显式断言未装配——owner 递补/归队/再递补三概念均
			// 不装配，promoteNextLocked 永不可达：registry.owner 恒 nil）：四断言的
			// shared 真值全部反转为「零变化」形态——断言一：cA 移除后 cB 零升格
			// Welcome（3.5s 窗口覆盖 pong_timeout 2s + 余量；可证伪：递补误装配即
			// 翻红）+ cB INPUT 生效（本就 rw）；断言二：旧 owner 重连直接 rw（无
			// FIFO 归队降级）；断言三：cB 发标记只有 cB 自收——cA2 静默窗零 cB 标记
			//（shared 列 cA2 收 fan-out——反转面；输出隔离本体由 fanout 测承载）；
			// 断言四：cB 断开后 cA2 零升格 Welcome + INPUT 生效。
			if mode == server.SessionModePerClient {
				resChB := make(chan frameRes, 16)
				quitB := make(chan struct{})
				defer close(quitB)
				go readPump(ctx, cB, resChB, quitB)
				window := time.After(3500 * time.Millisecond)
			noPromo:
				for {
					select {
					case r := <-resChB:
						if r.err != nil {
							t.Fatalf("cB 窗口内 read error: %v（连接应存活）", r.err)
						}
						if len(r.data) > 0 && r.data[0] == proto.Welcome {
							t.Fatalf("cB 收到升格 Welcome——owner 递补被误装配进 per-client（D-03 红线）: %q", r.data)
						}
					case <-window:
						break noPromo
					}
				}
				// cB 全程 rw：INPUT 生效（无升格语义——权限由自身 writable 决定）。
				sendInput(t, ctx, cB, []byte("b-rw-throughout"))
				accumFramesUntil(t, resChB, regexp.MustCompile("b-rw-throughout"))

				// 断言二（重连零归队）：旧 owner 立即重连——无 FIFO 递补队列，
				// cA2 直接 rw（shared 列为降级 ro 归队尾——分叉）。
				cA.CloseNow()
				cA2, modeA2 := dialHello(t, ctx, wsURL, 80, 24)
				if modeA2 != proto.ModeRW {
					t.Fatalf("re-connected old owner welcome mode = %q, want %q（per-client 无 owner 概念，重连直接 rw）", modeA2, proto.ModeRW)
				}
				defer cA2.CloseNow()
				resChA2 := make(chan frameRes, 16)
				quitA2 := make(chan struct{})
				defer close(quitA2)
				go readPump(ctx, cA2, resChA2, quitA2)

				// 断言三（零扇出给旁观端）：cB 发标记只有 cB 自收——cA2 静默窗
				// 零 cB 标记（帧级累积交叉断言，免疫标记跨帧切分）。
				sendInput(t, ctx, cB, []byte("b-own-output"))
				accumFramesUntil(t, resChB, regexp.MustCompile("b-own-output"))
				var a2Acc []byte
				win3 := time.After(1 * time.Second)
			noFanout:
				for {
					select {
					case r := <-resChA2:
						if r.err != nil {
							t.Fatalf("cA2 静默窗内 read error: %v（连接应存活）", r.err)
						}
						if len(r.data) > 0 && r.data[0] == proto.Output {
							a2Acc = append(a2Acc, r.data[1:]...)
						}
					case <-win3:
						break noFanout
					}
				}
				if bytes.Contains(a2Acc, []byte("b-own-output")) {
					t.Fatalf("cA2 收到 cB 标记——per-client 输出串台（共享扇出误装配）: %q", a2Acc)
				}

				// 断言四（再递补不存在）：cB CloseNow → cA2 静默窗零升格 Welcome
				// + INPUT 生效（shared 列为再递补升格——反转面）。
				cB.CloseNow()
				win4 := time.After(2 * time.Second)
			noRePromo:
				for {
					select {
					case r := <-resChA2:
						if r.err != nil {
							t.Fatalf("cA2 窗口内 read error: %v（连接应存活）", r.err)
						}
						if len(r.data) > 0 && r.data[0] == proto.Welcome {
							t.Fatalf("cA2 收到升格 Welcome——再递补被误装配进 per-client（D-03 红线）: %q", r.data)
						}
					case <-win4:
						break noRePromo
					}
				}
				sendInput(t, ctx, cA2, []byte("a2-rw-throughout"))
				accumFramesUntil(t, resChA2, regexp.MustCompile("a2-rw-throughout"))
				return
			}

			// shared 列（v1.0 期望值逐字搬入）：
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
		})
	}
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
//
// 对照子测：InputRate/InputBurst 调大（1MiB/1MiB）下同量 INPUT 全量送达（'x'
// 计数精确 == 发送量——证明丢弃确由限速器而非队列/其他路径）。
//
// 回显计数模型：/bin/cat 默认 canonical+ECHO——每送达一帧产生双份 'x'（行规
// ECHO 即时回显 + cat 读后 stdout 拷贝），ONLCR 只把 '\n' 展开为 \r\n、不影响
// 'x' 计数；每帧 511 'x' × 2 份 = 1022。
//
// 14-02 双模式（蓝本 :379 limits/协议守卫行「与进程模型无关」口径）：RES-02
// 输入限速是每客户端连接面（limiter 在两模式升档同形同值构造，perclient.go
// 281-283 / server.go:1130），两列同断言双跑——per-client 下洪水入自有会话，
// 丢弃/存活/对照三断言真值不变。
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

	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			t.Run("超限丢弃且连接存活", func(t *testing.T) {
				exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.InputRate = 1024  // 1KiB/s
					o.InputBurst = 1024 // 1KiB burst——恰容纳首批 2 帧（512B/帧）
				})

				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				// effMode 断言两列同值 rw：shared 由 owner 默认策略单客户端立 owner；
				// per-client 由自身 writable 决定（无 owner 面，同值）。
				c, effMode := dialHello(t, ctx, wsURL, 80, 24)
				if effMode != proto.ModeRW {
					t.Fatalf("welcome mode = %q, want %q（owner 默认策略单客户端立 owner）", effMode, proto.ModeRW)
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
				_, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.InputRate = 1 << 20  // 1MiB/s
					o.InputBurst = 1 << 20 // 1MiB burst——整批 64KiB 洪水一次性容纳
				})

				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				c, _ := dialHello(t, ctx, wsURL, 80, 24)
				defer c.CloseNow()
				snap := accum(c)

				// 帧间 2ms 节流：压平 64KiB 瞬时突发——无节流灌入远超 tty input queue
				// 消化速率，macos-latest CI 实测 XNU TTYHOG 丢弃 85.5%（18885/130816），
				// cat 慢时 write 阻塞路径同样受益。发送窗口 ~256ms ≪ 10s deadline。
				for i := 0; i < frames; i++ {
					sendInput(t, ctx, c, frame)
					time.Sleep(2 * time.Millisecond)
				}
				// 全量送达断言（对照组：证明限速子测的丢弃确由限速器而非 inputQ/其他
				// 路径，队列 256KiB ≫ 64KiB 洪水上限）。
				// 全平台放宽为 ≥95%：PTY 输入方向 line discipline input queue 高水位
				// 零星丢字节是平台固有行为，非整帧、与服务端路径无关——darwin XNU
				// TTYHOG 直接丢弃（CI 实测丢 ~2.6KB，128133/130816 ≈ 98%）；Linux
				// n_tty input queue（N_TTY_BUF_SIZE 4096B）在 CI 繁忙 cat 调度延迟
				// 顶穿后逐字符丢弃（ubuntu-latest 实测丢 6B，130810/130816 ≈
				// 99.995%——"Linux master write 阻塞不丢"仅 cat 读得快时成立）。
				// 5% 容差不动摇对照语义：限速子测丢弃 ≥75%，与 PTY 层 ~2% 丢失差
				// 两个数量级，归因判别力不变。
				deadline := time.Now().Add(10 * time.Second)
				for {
					got := bytes.Count(snap(), []byte{'x'})
					if got >= sentX*95/100 {
						if got != sentX {
							t.Logf("PTY input 高水位丢弃容忍：'x' = %d/%d (%.1f%%)", got, sentX, float64(got)/float64(sentX)*100)
						}
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("回显 'x' = %d, want >= %d（95%%；大限额下同量 INPUT 应近全量送达，PTY 层高水位零星丢失容忍）", got, sentX*95/100)
					}
					time.Sleep(20 * time.Millisecond)
				}
			})
		})
	}
}

// ====== plan 05-07 增量：RES-03 max-clients 容量闸测试（VALIDATION 05-02-02）======

// TestMaxClients503（VALIDATION 05-02-02，RES-03 + T-05-04/T-05-04b 缓解锁定）三面
// 行为断言：
//  1. WS 满员 503 + halfOpen 无泄漏 + detach 槽位释放：MaxClients=2 装配，A/B 两
//     dialHello 成功（计数=2）后第三人 WS 握手在 Accept 前收 HTTP 503（Dial 返回
//     的 *http.Response 状态码断言，handshake_test.go HTTP 层拒绝形态）；③位拒绝
//     后 halfOpen 不泄漏——MaxHalfOpenPerIP=1 装配下第四人仍达③位收 503 而非被
//     ②位 429 截（release() 恰好一次的行为化证据：泄漏一个名额即触 429）；stderr
//     存在 max_clients/code=503 JSON 事件（R-10 命名族，captureStderr +
//     startTrackedServerWith 同步边先例）。A CloseNow → detach -1 → 第三人
//     attach 成功（计数对称断言的行为化——Pitfall 4 防线：计数只增不减 = 运行时
//     间增长静默降低可用容量直至全员 503）。
//  2. /api/attach 早闸（OQ2）：凭据模式 MaxClients=1 占满后 POST（Basic 正确）
//     → 503；token 通道实例同款（body 携 rw token → 503）——Basic 链与 token
//     分支两签发路径同查（issueTicketJSON 唯一共享签发点）。
//  3. kick 路径槽位释放（review #7 第二移除路径）：stall 端被 1013 踢出
//     （removeLocked -1）→ 第三人 attach 成功。
//
// 14-02 双模式断言分叉表（蓝本 :386 multi 行「容量→spawn-intent 口径（P4）」）：
// 503 闸/halfOpen/槽位释放为 HTTP 层与注册表面（两列同断言，计数与进程模型
// 无关）；per-client 列补 spawn-intent 口径（P4/D-03 复检回收）：③位闸在
// spawn 之前——被拒 dial 零 spawn，wesh_pty_spawn_total 与成功 attach 数程序
// 序精确对照（metrics 通道，churn 格 load_test.go 先例形态——可读性择一注释
// 载明）；kick 子测 per-client 列分叉：各端自有洪水，B stall 满箱后 dwell 到期
// 踢（SlowDwell=500ms 短值覆写，TestSlowConsumerKick per-client 列同款——默认
// 10s 恒在断言窗外），kick 观测走 /healthz clients 计数轮询（14-01 勘误既定：
// A 的字节进度与 B 的踢出结构性解耦，12MiB 等待不构成同步边）。
func TestMaxClients503(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			t.Run("WS 满员 503 与 halfOpen 无泄漏与槽位释放", func(t *testing.T) {
				restore := captureStderr(t)
				defer restore() // 失败路径兜底恢复 os.Stderr（幂等）

				// handler 追踪变体（:756 startTrackedServerWith 收编 newTrackedTestServer
				// ——14-01 小族 tracked 形态，waitHandlers 同步边保留）。MaxHalfOpenPerIP=1
				// 使 halfOpen 泄漏可观测（见下）。容量计数与 mode 无关（③位只看 registry.n），
				// owner 默认策略即可——B 降级 ro 同样占注册名额（per-client 下各端恒 rw
				// 亦同占名额）。
				exitCh, wsURL, waitHandlers := newTrackedTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.MaxClients = 2
					o.MaxHalfOpenPerIP = 1
				})

				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cA, _ := dialHello(t, ctx, wsURL, 80, 24)
				cB, _ := dialHello(t, ctx, wsURL, 80, 24)

				// 满员拒绝：第三人 WS 握手 → Accept 前 HTTP 503（零 WS 资源分配）。
				dialWant503 := func(what string) {
					t.Helper()
					_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
					if err == nil {
						t.Fatalf("%s dial unexpectedly succeeded — max-clients ③位 503 闸缺失", what)
					}
					if resp == nil {
						t.Fatalf("%s dial failed without HTTP response: %v", what, err)
					}
					if resp.StatusCode != http.StatusServiceUnavailable {
						t.Fatalf("%s dial status = %d, want %d (503 Accept 前拒绝)", what, resp.StatusCode, http.StatusServiceUnavailable)
					}
				}
				dialWant503("第三人")
				// halfOpen 无泄漏：③位拒绝路径 release() 恰好一次——第四人仍达③位收
				// 503；若第三人残留半开计数（MaxHalfOpenPerIP=1），本次必被 ②位 429 截。
				dialWant503("第四人（③位拒绝后 halfOpen 名额已释放）")

				// per-client 列 spawn-intent 口径（P4/D-03）：③位闸在 spawn 之前——
				// 两次被拒 dial 零 spawn，spawn_total == 成功 attach 数（== 2 ≤
				// maxClients 程序序精确对照；若闸被误装配在 spawn 之后，被拒 dial
				// 计入计数 > 2 即翻红——「并发 attach 后实际 spawn 数 ≤ max-clients」
				// 的注册点计数器观测面，upgradePerClient :311-315 递增点）。
				if mode == server.SessionModePerClient {
					body := getMetrics(t, httpBaseOf(wsURL)+"/metrics")
					if v, ok := metricSample(t, body, "wesh_pty_spawn_total"); !ok || v != 2 {
						t.Fatalf("wesh_pty_spawn_total = %d (present=%v), want 2（spawn-intent 口径：503 闸先于 spawn——被拒 dial 零 spawn，成功 attach 恰 A/B 两次）", v, ok)
					}
				}

			// 槽位释放（detach 路径）：A CloseNow → detach -1 → 第三人 attach 成功。
			// 「attach 成功」判定 = Welcome 到手，非 dial 放行：per-client 下 detach
			// 只移除 registry（HTTP 503 闸即放行），pcSessions 移除滞后于 teardown
			// 全链（Drain + 子进程收割）——Hello 后 pre-spawn 容量再闸在 linger 窗口
			// 内 capacity 拒绝（perclient.go D-02「断开待收割 linger」登记行为，
			// run 34225422898 macos 实证）。拒绝/读错连接弃置重试（半开名额在升档
			// 分岔前已 release，503 重试与 capacity 重试均不污染 halfOpen 计数），
			// 轮询消除到达序竞态。
			cA.CloseNow()
			var cE *websocket.Conn
			deadline := time.Now().Add(5 * time.Second)
			hello, herr := json.Marshal(proto.HelloPayload{Version: proto.Subprotocol, Cols: 80, Rows: 24})
			if herr != nil {
				t.Fatalf("marshal Hello: %v", herr)
			}
			for cE == nil {
				c, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
				if err != nil {
					if resp != nil && resp.StatusCode == http.StatusServiceUnavailable {
						if time.Now().After(deadline) {
							t.Fatal("A CloseNow 后 5s 内仍满员——detach 槽位释放失败（计数只增不减，Pitfall 4）")
						}
						time.Sleep(20 * time.Millisecond)
						continue // A 的 detach 尚未落账——重试
					}
					t.Fatalf("槽位释放后轮询 dial 非 503 失败: err=%v resp=%v", err, resp)
				}
				// Hello → Welcome 全链判定；capacity 拒绝（teardown linger 窗口）弃置重试。
				if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Hello}, hello...)); err != nil {
					t.Fatalf("cE write Hello: %v", err)
				}
				_, data, err := c.Read(ctx)
				if err == nil && len(data) > 0 && data[0] == proto.Welcome {
					cE = c // attach 成功全链，计数=2 的第三人
					break
				}
				_ = c.CloseNow() // capacity 拒绝序列/异常帧/读错——连接弃置，重试
				if time.Now().After(deadline) {
					t.Fatalf("A CloseNow 后 5s 内 attach 未成功（最后首帧=%v err=%v）——槽位释放或 teardown 收割未在护栏内落定", data, err)
				}
				time.Sleep(20 * time.Millisecond)
			}

				// per-client 列对照延续：cE attach 成功恰一次新 spawn（程序序精确 == 3）。
				if mode == server.SessionModePerClient {
					body := getMetrics(t, httpBaseOf(wsURL)+"/metrics")
					if v, ok := metricSample(t, body, "wesh_pty_spawn_total"); !ok || v != 3 {
						t.Fatalf("cE attach 后 wesh_pty_spawn_total = %d (present=%v), want 3（槽位释放后的第三人恰一次新 spawn）", v, ok)
					}
				}

				cB.Close(websocket.StatusNormalClosure, "")
				cE.Close(websocket.StatusNormalClosure, "")
				// 同步边：等全部 Attach handler 返回——③位 logEvent 在 handler 内先于
				// 返回执行，WaitGroup happens-before 使 restore() 的 os.Stderr 写与该读
				// 同步（05-01 startTrackedServerWith 先例）。
				waitHandlers()
				// 容量闸不改生命周期：断开不触发 exitf（多客户端推论）。
				assertNoExit(t, exitCh)

				// R-10 命名族：stderr 存在 event=="max_clients" 且 code==503 的 JSON 事件
				//（HTTP 层事件 code 复用 HTTP 状态码值；JSON 数字按 float64 比——Pitfall 4）。
				// 次数不断言——轮询重试次数不确定（每次 503 均落事件），存在性即锁定事件落点。
				out := restore()
				evs := parseEvents(t, out)
				found := false
				for _, m := range evs {
					if m["event"] == "max_clients" && m["code"] == float64(http.StatusServiceUnavailable) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("stderr 缺 max_clients/code=503 事件（R-10 命名族）：out=%q", out)
				}
			})

			t.Run("api attach 早闸双通道", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				// —— Basic 通道（凭据模式，MaxClients=1）——
				cred, err := server.ParseCredential("cap-op:cap-pass")
				if err != nil {
					t.Fatalf("ParseCredential: %v", err)
				}
				_, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.MaxClients = 1
					o.Credentials = []server.Credential{cred}
					o.ThrottleBase = 50 * time.Millisecond
				})
				url := attachURL(wsURL)
				// 占满唯一槽位：Basic → ticket → WS attach（计数=1）。
				resp := postAttach(t, url, "cap-op", "cap-pass", nil)
				if resp.StatusCode != http.StatusOK {
					_ = resp.Body.Close()
					t.Fatalf("占槽签发 status = %d, want %d (200)", resp.StatusCode, http.StatusOK)
				}
				var issued struct {
					Ticket string `json:"ticket"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
					_ = resp.Body.Close()
					t.Fatalf("decode ticket response: %v", err)
				}
				_ = resp.Body.Close()
				if issued.Ticket == "" {
					t.Fatal("ticket empty in 200 response")
				}
				c1, _ := dialHelloTicket(t, ctx, wsURL, issued.Ticket, 80, 24)
				defer c1.CloseNow()

				// 满员早闸：Basic 正确 → 503（签发 ticket 前 atomic load 满员，OQ2——
				// 前端 fetch 阶段即可给 Server is full 专版文案）。
				resp = postAttach(t, url, "cap-op", "cap-pass", nil)
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusServiceUnavailable {
					t.Fatalf("满员 Basic attach status = %d, want %d (503 早闸)", resp.StatusCode, http.StatusServiceUnavailable)
				}

				// —— token 通道（无认证 + shares，OQ1 正交，MaxClients=1）——
				// 红线纪律（D-03）：token 值只存局部变量作断言材料，断言只含状态码。
				roTok := server.GenerateShareToken()
				rwTok := server.GenerateShareToken()
				_, wsURL2 := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.MaxClients = 1
					o.ShareTokenRO = roTok
					o.ShareTokenRW = rwTok
				})
				url2 := attachURL(wsURL2)
				tokenBody := func(tok string) []byte {
					b, err := json.Marshal(struct {
						Token string `json:"token"`
					}{Token: tok})
					if err != nil {
						t.Fatalf("marshal token body: %v", err)
					}
					return b
				}
				// 占满唯一槽位：rw token → ticket → WS attach（计数=1）。
				resp2, err := http.Post(url2, "application/json", bytes.NewReader(tokenBody(rwTok)))
				if err != nil {
					t.Fatalf("token 签发 POST: %v", err)
				}
				if resp2.StatusCode != http.StatusOK {
					_ = resp2.Body.Close()
					t.Fatalf("token 通道占槽签发 status = %d, want %d (200)", resp2.StatusCode, http.StatusOK)
				}
				var issued2 struct {
					Ticket string `json:"ticket"`
				}
				if err := json.NewDecoder(resp2.Body).Decode(&issued2); err != nil {
					_ = resp2.Body.Close()
					t.Fatalf("decode token ticket response: %v", err)
				}
				_ = resp2.Body.Close()
				if issued2.Ticket == "" {
					t.Fatal("ticket empty in token 200 response")
				}
				c2, _ := dialHelloTicket(t, ctx, wsURL2, issued2.Ticket, 80, 24)
				defer c2.CloseNow()

				// 满员早闸：body 携 rw token → 503（token 分支与 Basic 链同查——
				// issueTicketJSON 唯一共享签发点，一处检查两通道）。
				resp3, err := http.Post(url2, "application/json", bytes.NewReader(tokenBody(rwTok)))
				if err != nil {
					t.Fatalf("满员 token POST: %v", err)
				}
				_ = resp3.Body.Close()
				if resp3.StatusCode != http.StatusServiceUnavailable {
					t.Fatalf("满员 token attach status = %d, want %d (503 早闸)", resp3.StatusCode, http.StatusServiceUnavailable)
				}
			})

			t.Run("kick 路径槽位释放", func(t *testing.T) {
				// darwin 跳过（macOS CI flake 实测）：kickOrCreditLocked 分工表
				//（clients.go:400-419）踢 B 的前提是「剔除 B 后仍存在未 blocked 可写端
				// A」；darwin loopback TCP buffer 仅 ~190KB（见下方 12MiB 等待跳过注释），
				// A 也易触 creditBlocked，前提不成立时 B 转为持信用闭门而非被踢，
				// assertKicked1013(B) 10s 超时。等价覆盖：Linux leg 同测试通过 +
				// TestSlowConsumerKick 单独锁定 1013 踢出全链（darwin 通过）。
				// 平台闸非模式闸——跳过覆盖两列（per-client darwin 面由
				// TestSlowConsumerKick per-client 列承载，14-01 已双平台绿）。
				if runtime.GOOS == "darwin" {
					t.Skip("darwin: A also prone to creditBlocked under small TCP buffers, B may hold gate instead of being kicked")
				}
				// MaxClients=2：A 正常读取 + B stall（OutboxBytes 覆写小值 + seq 输出
				// 洪水——slowclient_test.go 夹具形态）→ B 被 1013 踢出（removeLocked -1，
				// 第二移除路径）→ 第三人 attach 成功（踢出路径计数对称的行为化，
				// review #7）。显式 WritePolicy=all——双 rw 语义前提（owner 默认策略下
				// B 降级 ro：stall ro 端同样满即踢，但 A 为唯一可写端的语义组合与本
				// 子场景锁定的 R-08 离群慢端踢出路径不同，保持与 TestSlowConsumerKick
				// 同款前提；per-client 列 WritePolicy 无仲裁面——同 mutate 镜像）。
				// 洪水量论证（-count=3 实测驱动的修正）：seq 1 50000000 ≈ 389MB——踢出
				// 触发点在 B 的 loopback 吸收极限（~10MiB + 64KiB outbox），洪水须以
				// 数量级余量压过该点：若洪水先于踢出耗尽（38.9MB 形态实测命中），子进程
				// 退出 → lifecycle 广播 1000 与异步 Close(1013) 竞态（casClosing 先到
				// 先赢），stall B 可能观测到 1000 而非 1013。
				mutate := func(o *server.Options) {
					o.WritePolicy = server.WritePolicyAll
					o.MaxClients = 2
					o.OutboxBytes = 64 * 1024
					if mode == server.SessionModePerClient {
						// per-client 列：dwell 500ms 短值覆写（默认 10s 恒在断言窗外）
						// ——TestSlowConsumerKick per-client 列同款；dwell 语义本体由
						// perclient_test.go Phase 12 三测承载，本列只锁踢出→槽位释放链。
						o.SlowDwell = 500 * time.Millisecond
					}
				}
				_, wsURL := newTestServer(t, mode, []string{"seq", "1", "50000000"}, mutate)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cA, _ := dialHello(t, ctx, wsURL, 80, 24)
				cB, _ := dialHello(t, ctx, wsURL, 80, 24)
				defer cA.CloseNow()
				// writer 同类型连续段合并使单条 WS 消息可达 outbox 容量量级——超过 Go
				// 客户端库默认 32KiB 读上限会触发 1009 自动关闭（slowclient_test.go 实测
				// 注释），测试客户端显式放宽。
				cA.SetReadLimit(4 * 1024 * 1024)
				cB.SetReadLimit(4 * 1024 * 1024)

				// per-client 列分叉：各端自有洪水——B stall 满箱后 dwell(500ms) 踢；
				// A 持续读取保未踢（自有会话独立推进）。kick 观测 = /healthz clients
				// 2→1 轮询（TestSlowConsumerKick per-client 同步边先例——A 的字节进度
				// 与 B 的踢出结构性解耦，12MiB 等待不构成同步边；首次 Read 推迟到
				// 踢出可观测之后，过早续读会重置 dwell 使会话 1000 收尾——14-01 勘误）。
				if mode == server.SessionModePerClient {
					// A 持续读取保未踢（只计数不缓存——自有会话独立推进，字节数无断言）。
					var aBytes atomic.Int64
					go func() {
						for {
							_, data, err := cA.Read(context.Background())
							if err != nil {
								return
							}
							if len(data) > 0 && data[0] == proto.Output {
								aBytes.Add(int64(len(data) - 1))
							}
						}
					}()
					kickDeadline := time.Now().Add(15 * time.Second)
					for healthzClients(t, wsURL) != 1 {
						if time.Now().After(kickDeadline) {
							t.Fatalf("stall B 15s 内未被移除——dwell(500ms) 踢出未发生（per-client 满即踢分叉点）")
						}
						time.Sleep(15 * time.Millisecond)
					}
					// B 此刻首次 Read：消耗管道积存 OUTPUT 后必见 CloseError 1013
					// slow_consumer（先取踢出证据——关闭帧写出带 5s 超时，须在其
					// 窗口内开始消化管道）。
					assertKicked1013(t, cB, 10*time.Second, "stall B")
					// removeLocked 先于 1013 关闭帧发出——观察到踢出时计数已减（2→1），
					// 第三人 attach 成功（踢出路径计数对称的行为化）。
					cC, _ := dialHello(t, ctx, wsURL, 80, 24)
					cC.CloseNow()
					return
				}

				// shared 列（v1.0 期望值逐字搬入）：
				// A 持续读取保未 blocked（只计数不缓存——R-08 分工表：剔除 B 后仍存在
				// 未 blocked 的可写端 A → B 离群慢端立即踢；Pitfall 2：Read 永不带
				// deadline ctx）。
				var aBytes atomic.Int64
				go func() {
					for {
						_, data, err := cA.Read(context.Background())
						if err != nil {
							return
						}
						if len(data) > 0 && data[0] == proto.Output {
							aBytes.Add(int64(len(data) - 1))
						}
					}
				}()

				// stall 纪律（TestSlowConsumerKick 逐字形态，初版违例实测命中）：B 在
				// 踢出触发前绝不 Read——等 A 累积超 12MiB，此时 B 管道（最坏 ~10MiB）
				// 必然已满、outbox 已写满、1013 踢出已触发；提前读 B 会排空管道使踢出
				// 永不成立（assertKicked1013 的 readUntilError 即读者，调用时点纪律）。
				//
				// darwin 跳过 12MiB 等待（macOS CI 实测 A 15s 仅收 ~190KB）：darwin
				// loopback/PTY 管道 ~128KiB ≪ Linux ~10MiB，小 buffer 下 A 也易触
				// creditBlocked → 信用门反复开关，吞吐崩塌使「A 收 12MiB」不可达。
				// 但 B stall→outbox 满→1013 踢出是确定行为（macOS CI 日志实测
				// code=1013 slow_consumer），直接 assertKicked1013(B) 即可取踢出证据；
				// fan-out 持续推进断言由 TestSlowConsumerKick 同款形态覆盖（darwin 通过）。
				if runtime.GOOS != "darwin" {
					deadline := time.Now().Add(15 * time.Second)
					for aBytes.Load() < 12*1024*1024 {
						if time.Now().After(deadline) {
							t.Fatalf("A received %d bytes in 15s, want >= 12MiB（洪水未推进，stall 夹具失效）", aBytes.Load())
						}
						time.Sleep(50 * time.Millisecond)
					}
				}
				// B 此刻首次 Read：消耗管道积存 OUTPUT 后必见 CloseError 1013
				// slow_consumer。先取踢出证据再继续——关闭帧写出带 5s 超时
				//（close.go:168-183），须在其窗口内开始消化管道。
				assertKicked1013(t, cB, 10*time.Second, "stall B")
				// removeLocked 先于 1013 关闭帧发出（kickSlowConsumerLocked 同步段内）——
				// 观察到踢出时计数已减（2→1），第三人 attach 成功，无需轮询。
				cC, _ := dialHello(t, ctx, wsURL, 80, 24)
				cC.CloseNow()
			})
		})
	}
}

// ====== plan 05-10 增量：G-05-1 会话尺寸下发测试（Welcome 三通道 carriage 断言）======

// TestWelcomeSessionDims（G-05-1 核心 carriage 断言组）：Welcome 帧恒携会话
// cols/rows——attach Welcome（升档）携 attach 完成后生效的会话尺寸；旁观者 Welcome
// 携会话尺寸而非自身窗口尺寸；递补升格 Welcome 携新 owner 尺寸（cand.dims）。
// map[string]any JSON 解码数值为 float64（dialHelloPayload 返回完整 Welcome map，
// e2e_test.go 既有 helper 零新造）。
//
// 14-02 双模式断言分叉表（蓝本 :387 resize 行「per-client=直通……无 min-rect」
// 的 Welcome carriage 面）：per-client 无仲裁，G-05-1 契约退化为恒等式
// （upgradePerClient :291-298——cols/rows 回显本端 Hello 钳制尺寸，不经
// sessionDimsLocked）。分叉点：①旁观者（B）Welcome 尺寸——shared=会话尺寸
// （40x10）/ per-client=自身 Hello 尺寸（120x40）；②all 模式 B 的 Welcome——
// shared=min-rect 重算（60x43）/ per-client=自身（60x50）；③A 断开后的升格
// Welcome——shared=递补推送（120x40 cand.dims）/ per-client=零 Welcome 帧
// （D-03 静默窗可证伪）+ B INPUT 生效（会话无扰动）。A 单员 last-wins 与
// per-client 恒等式数值巧合同值（40x10/132x43），以 B 端分叉产生判别力。
func TestWelcomeSessionDims(t *testing.T) {
	for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
		t.Run("mode="+mode, func(t *testing.T) {
			// owner 模式（Writable: true 默认策略）：A(owner rw 40x10)/B(降级 ro 120x40)。
			t.Run("owner模式attach与升格携会话尺寸", func(t *testing.T) {
				exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, nil)

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				// A：首个 rw attach 立 owner——参与集单员 last-wins，会话尺寸 = A 自身
				// Hello 尺寸（40x10），Welcome 携之（attach 升档序列已重排：addMember/
				// recalcNow 先于 Welcome 组帧）。per-client 恒等式数值巧合同值。
				cA, wmA := dialHelloPayload(t, ctx, wsURL, 40, 10)
				if wmA["mode"] != proto.ModeRW {
					t.Fatalf("A welcome mode = %v, want %q（首个 rw attach 立 owner）", wmA["mode"], proto.ModeRW)
				}
				if wmA["cols"] != float64(40) || wmA["rows"] != float64(10) {
					t.Fatalf("A welcome dims = %vx%v, want 40x10（owner 单员 last-wins 会话尺寸）", wmA["cols"], wmA["rows"])
				}
				// B：分叉表——shared = D-07 降级 ro，旁观者 Welcome 携会话尺寸（40x10）
				// 而非自身窗口尺寸（120x40）：G-05-1 核心 carriage 断言；per-client =
				// 恒 rw + Welcome 携自身 Hello 尺寸（120x40）——无仲裁无降级。
				cB, wmB := dialHelloPayload(t, ctx, wsURL, 120, 40)
				if mode == server.SessionModePerClient {
					if wmB["mode"] != proto.ModeRW {
						t.Fatalf("B welcome mode = %v, want %q（per-client 无 owner 降级，D-03）", wmB["mode"], proto.ModeRW)
					}
					if wmB["cols"] != float64(120) || wmB["rows"] != float64(40) {
						t.Fatalf("B welcome dims = %vx%v, want 120x40（per-client 恒等式：Welcome 回显自身 Hello 尺寸，无 min-rect）", wmB["cols"], wmB["rows"])
					}
				} else {
					if wmB["mode"] != proto.ModeRO {
						t.Fatalf("B welcome mode = %v, want %q（D-07 降级）", wmB["mode"], proto.ModeRO)
					}
					if wmB["cols"] != float64(40) || wmB["rows"] != float64(10) {
						t.Fatalf("B(ro) welcome dims = %vx%v, want 40x10（旁观者 Welcome 携会话尺寸而非自身窗口尺寸）", wmB["cols"], wmB["rows"])
					}
				}

				// A 断开 → 分叉表：shared = B 递补升格（升格 Welcome 携新 owner 尺寸
				// = cand.dims，Hello 登记尺寸 120x40——owner 模式升格后参与集 = {B}
				// 单员，arbitrate 单员 = cand.dims，恒等于升格后 recalcNow 的 last）；
				// per-client = 零升格 Welcome（D-03 静默窗可证伪）+ B INPUT 生效。
				cA.CloseNow()
				if mode == server.SessionModePerClient {
					resCh := make(chan frameRes, 8)
					quit := make(chan struct{})
					defer close(quit)
					go readPump(ctx, cB, resCh, quit)
					window := time.After(2 * time.Second)
				noPromo:
					for {
						select {
						case r := <-resCh:
							if r.err != nil {
								t.Fatalf("B 静默窗内 read error: %v（连接应存活）", r.err)
							}
							if len(r.data) > 0 && r.data[0] == proto.Welcome {
								t.Fatalf("B 收到升格 Welcome——owner 递补被误装配进 per-client（D-03 红线）: %q", r.data)
							}
						case <-window:
							break noPromo
						}
					}
					sendInput(t, ctx, cB, []byte("b-dims-own"))
					accumFramesUntil(t, resCh, regexp.MustCompile("b-dims-own"))
				} else {
					select {
					case wm := <-readUntilWelcome(cB):
						if wm == nil {
							t.Fatal("B read terminated before promotion Welcome（升格推送丢失）")
						}
						if wm["mode"] != proto.ModeRW {
							t.Fatalf("promotion Welcome mode = %v, want %q（R-09 升格推送）", wm["mode"], proto.ModeRW)
						}
						if wm["cols"] != float64(120) || wm["rows"] != float64(40) {
							t.Fatalf("promotion Welcome dims = %vx%v, want 120x40（升格携新 owner Hello 登记尺寸 cand.dims）", wm["cols"], wm["rows"])
						}
					case <-time.After(5 * time.Second):
						t.Fatal("B 未在 5s 内收到升格 Welcome——owner 断开后 FIFO 递补失败")
					}
				}

				cB.Close(websocket.StatusNormalClosure, "")
				assertNoExit(t, exitCh)
			})

			// all 模式（WritePolicyAll）：A(132x43) 单员 last-wins；B(60x50) attach 后
			// min-rect 即时重算 min(132,60)xmin(43,50) = 60x43 ≠ B 自身 60x50——rows 维
			// 产生区分度：Welcome 携重算后会话尺寸（60x43）而非 B 自身窗口尺寸（60x50）。
			//（plan 字面 B(60,20)→60/24 算术自相矛盾——min(43,20)=20 恰等于 B 自身尺寸
			// 无区分度；按 plan 明示意图「rows 维产生区分度断言」取 B(60,50)，SUMMARY
			// 登记 Rule 1 修正。）
			// 14-02 分叉：per-client 无 min-rect——B 的 Welcome 携自身 Hello 尺寸
			//（60x50，恒等式；可证伪：仲裁误装配则 60x43 翻红）。
			t.Run("all模式attach携min-rect重算尺寸", func(t *testing.T) {
				exitCh, wsURL := newTestServer(t, mode, []string{"/bin/cat"}, func(o *server.Options) {
					o.WritePolicy = server.WritePolicyAll
				})

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cA, wmA := dialHelloPayload(t, ctx, wsURL, 132, 43)
				if wmA["mode"] != proto.ModeRW {
					t.Fatalf("A welcome mode = %v, want %q（all 模式全员可写）", wmA["mode"], proto.ModeRW)
				}
				if wmA["cols"] != float64(132) || wmA["rows"] != float64(43) {
					t.Fatalf("A welcome dims = %vx%v, want 132x43（单员 last-wins）", wmA["cols"], wmA["rows"])
				}
				// B attach 触发 min-rect 即时重算（60x43），B 的 Welcome 携重算后会话尺寸
				//（rows 维 43 ≠ B 自身 50——区分度断言）；重算推送落 A（未被读流，不影响）。
				// per-client 分叉：B 的 Welcome 携自身尺寸（60x50）——无重算。
				cB, wmB := dialHelloPayload(t, ctx, wsURL, 60, 50)
				if wmB["mode"] != proto.ModeRW {
					t.Fatalf("B welcome mode = %v, want %q（all 模式全员可写）", wmB["mode"], proto.ModeRW)
				}
				if mode == server.SessionModePerClient {
					if wmB["cols"] != float64(60) || wmB["rows"] != float64(50) {
						t.Fatalf("B welcome dims = %vx%v, want 60x50（per-client 恒等式：自身 Hello 尺寸直通，无 min-rect）", wmB["cols"], wmB["rows"])
					}
				} else if wmB["cols"] != float64(60) || wmB["rows"] != float64(43) {
					t.Fatalf("B welcome dims = %vx%v, want 60x43（min-rect 重算后会话尺寸，rows 维 ≠ B 自身 50 产生区分度）", wmB["cols"], wmB["rows"])
				}

				cA.Close(websocket.StatusNormalClosure, "")
				cB.Close(websocket.StatusNormalClosure, "")
				assertNoExit(t, exitCh)
			})
		})
	}
}
