package server_test

// limits_test.go 锁定 RES-01 三层上限（D-09 修订后的等效防线形态）与 D-12② 超限
// 可见性：TestOversize1009 / TestReadLimitBoundary / TestFragmentedFlood1009 /
// TestEmptyFragmentFloodResilience / TestPreHelloReadLimit，与 RESEARCH
// §Validation Architecture RES-01 五行映射一一对应。
//
// 库行为锚点（coder/websocket v1.8.15 源码核实）：
//   - SetReadLimit 内部 +1 余量供 fin 帧收尾读（read.go:97-105）——边界恰为
//     "limit 字节通过 / limit+1 字节在收尾读时被 1009"；
//   - 超限由 limitReader 流式触发：writeError(1009) 后 Read 返回包装
//     ErrMessageTooBig 的错误（read.go:521-541），stderr 事件钩子即挂此处；
//   - 客户端 Writer 每次 Write 产生一个非 fin continuation 帧（write.go:223-264）
//     ——1 字节分片洪水精确构造，无需裸帧 helper（D-09 修订：rawws 作废）；
//   - 空消息不经字节累积（0 字节载荷不递减计数）——D-09 修订残余风险，断言
//     "存活+内存平坦"而非 1009。

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)

// startRawCatServer 装配 PTY 已切 raw 模式的 /bin/cat 测试服务端（仅
// TestReadLimitBoundary 使用）：termios 由父进程侧 stty 经 master fd 同步
// 设置（stty 默认操作 stdin 所指终端；tcsetattr 对 master 与 slave 等效），
// 且在 net.Listen 之前完成——结构上消除子进程内 stty 的启动窗口（窗口内
// 到达的字节会被行规程回显一次、模式切换后又被 cat 输出一次，双重回显）。
// cat 不改 termios，与 stty 并发无冲突；Listen 前无任何客户端输入可达。
func startRawCatServer(t *testing.T) (exitCh chan int, wsURL string) {
	t.Helper()
	// 零值等价形态（07-04 选项化适配：Uid/Gid -1 = 不降权，Dir/Term 空 = 现状）。
	sess, err := pty.Start([]string{"/bin/cat"}, pty.StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	stty := exec.Command("stty", "raw", "-echo")
	stty.Stdin = sess.Master
	if out, err := stty.CombinedOutput(); err != nil {
		t.Fatalf("stty raw -echo: %v (%s)", err, out)
	}
	exitCh = make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, server.Options{Writable: true})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { killServer(ln, sess) })
	go http.Serve(ln, srv.Handler())
	return exitCh, "ws://" + ln.Addr().String() + "/ws"
}

// readCloseErr 读至连接关闭并返回 CloseError（五测共用的关闭码收口，沿用
// e2e_test.go 各测的 CloseError 读取循环模式）。非 CloseError 终结按失败处理。
func readCloseErr(t *testing.T, c *websocket.Conn, ctx context.Context) websocket.CloseError {
	t.Helper()
	var ce websocket.CloseError
	for {
		if _, _, err := c.Read(ctx); err != nil {
			if !errors.As(err, &ce) {
				t.Fatalf("read terminated without CloseError: %v", err)
			}
			return ce
		}
	}
}

// captureStderr 将进程 os.Stderr 替换为管道写端，返回恢复函数：调用即恢复
// os.Stderr 并返回捕获全文（幂等，可 defer 兜底）。
//
// 进程全局替换不并行——同包测试默认串行（无 t.Parallel），且五测中仅
// TestOversize1009 一处断言 stderr（plan 裁决：避免 fd 替换竞争，其余以
// CloseError 为准）。恢复时序安全（多客户端形态）：logEvent 在服务端读循环内、
// 客户端观测 CloseError 之后数微秒写出——TestOversize1009 以 assertNoExit 的
// 200ms 静默窗口覆盖该余量（远超读循环调度延迟），恢复无竞态。
func captureStderr(t *testing.T) func() string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	restored := false
	return func() string {
		if restored {
			return ""
		}
		restored = true
		os.Stderr = old
		_ = w.Close()
		out, _ := io.ReadAll(r) // 写端已关，读至 EOF（单行事件远小于管道缓冲）
		_ = r.Close()
		return string(out)
	}
}

// TestOversize1009（RES-01 + D-12②）：稳态 16KiB 档——总长 16385 字节的消息
// （ReadLimitPostAuth+1，首字节随意：超限在协议解析前由库拦截）触发库自动 1009；
// 服务端 stderr 留下恰一行 message_too_big 事件（remote/code/reason 三要素），
// 随后经 detach 收口（多客户端推论：不触发 exitf）。
func TestOversize1009(t *testing.T) {
	restore := captureStderr(t)
	defer restore() // 失败路径兜底恢复 os.Stderr（幂等，成功路径显式调用后空转）

	// handler 追踪变体：restore() 写 os.Stderr 前需与 handler 内 logEvent 的读
	// 建立同步边（waitExit 消亡后的替代形态，见 startTrackedServerWith 注释）。
	exitCh, wsURL, waitHandlers := startTrackedServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	if err := c.Write(ctx, websocket.MessageBinary, make([]byte, proto.ReadLimitPostAuth+1)); err != nil {
		t.Fatalf("write oversize message: %v", err)
	}
	ce := readCloseErr(t, c, ctx)
	if ce.Code != websocket.StatusMessageTooBig {
		t.Fatalf("close code = %d, want %d (1009)", ce.Code, websocket.StatusMessageTooBig)
	}
	// 同步边：等该连接的 Attach handler 返回——logEvent 在 handler 内先于返回
	// 执行，WaitGroup happens-before 使 restore() 的 os.Stderr 写与该读同步。
	waitHandlers()
	// 多客户端推论：超限断开不再触发 exitf——200ms 静默反证。
	assertNoExit(t, exitCh)

	// D-12② 超限可见性三腿之二：stderr 恰一条 message_too_big JSON 事件，
	// 三要素齐全（event 名精确相等；code 按 float64 比——08-RESEARCH Pitfall 4）。
	out := restore()
	evs := parseEvents(t, out)
	if n := countByEvent(evs, "message_too_big"); n != 1 {
		t.Fatalf("stderr message_too_big event count = %d, want exactly 1 (out=%q)", n, out)
	}
	var ev map[string]any
	for _, m := range evs {
		if m["event"] == "message_too_big" {
			ev = m
		}
	}
	remote, _ := ev["remote"].(string)
	if ev["code"] != float64(websocket.StatusMessageTooBig) || !strings.HasPrefix(remote, "127.0.0.1:") {
		t.Fatalf("message_too_big 事件三要素不符（code/remote）: %v (out=%q)", ev, out)
	}
}

// TestReadLimitBoundary（RES-01）：两层硬顶边界精确——总长恰 16384 的 INPUT 帧
// 被正常接受（不触发 1009、连接存活、数据通路有序完好），总长 16385 的 INPUT
// 帧被 1009 切断。
//
// 载荷用 'A' 不用零字节（plan 字面 zeros 的实测偏差修正）：NUL 是控制字符，
// PTY 规范模式 ECHOCTL 将其回显为 "^@" 两字节；可打印字符回显语义干净。
// 边界值一律引用 proto 常量而非硬编码数字。
//
// 子进程用裸 /bin/cat（D-02 不经 shell），raw 模式由父进程侧 stty 经 master
// fd 同步设置（startRawCatServer）：tcsetattr 对 master 与 slave 等效（同一
// 终端），且在 net.Listen 之前完成——结构上消除"子进程内 stty 生效前客户端
// 输入已到"的启动窗口：规范模式窗口内到达的字节会被行规程 ECHO 回显一次，
// 模式切换时又随 canon 缓冲倒入 rawq 被 cat 再输出一次（双重回显，全量跑
// 实测 ~2KB 竞态命中）。规范模式下无换行的输入永远到不了 cat（回显全来自
// 行规程且受 canon 缓冲 Darwin 1024 截断）；raw 模式无 canon 上限、无行规程
// 回显，输入由 cat 本体 1:1 输出。
//
// 大回显不做精确字节数断言（macOS CI 实测修正）：Darwin ptmx 写路径
// ptcwrite() 没有任何反压——slave 输入队列（rawq+canq）达到 TTYHOG(2048)
// 后字符被内核静默丢弃且 write 假报全部成功（xnu bsd/kern/tty_ptmx.c 的
// kqueue 可写判定即按 TTYHOG-2 计算；tty.c ttyinput 溢出路径直接丢字符），
// 故 macOS 上 16383 字节载荷仅前 2048 字节进入 slave，回显恒 2048；Linux
// master 写真正阻塞等待、不丢字节，回显恰 16383。断言改为平台无关形态：
// 边界消息后追加独立小标记，标记回显完整即证明恰达上限的消息被接受
// （无 1009、连接存活、数据通路有序），前缀全 'A' 且长度 ∈ [1,16383]。
// macOS 宿主机 >2KB 输入丢字节为内核行为的已知平台限制（产品侧修复需
// darwin kqueue pacing 写路径），本期不覆盖。
func TestReadLimitBoundary(t *testing.T) {
	exitCh, wsURL := startRawCatServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 边界内：'0' + 16383 = 总长恰 16384。
	payload := bytes.Repeat([]byte("A"), proto.ReadLimitPostAuth-1)
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write boundary INPUT: %v", err)
	}
	// 紧随的独立小标记：WS 消息有序处理、PTY 字节流有序、回显有序，故收到
	// 完整标记即证明边界消息已被接受且数据通路完好（与回显多少 'A' 无关）。
	marker := []byte("boundary-tail-marker")
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, marker...)); err != nil {
		t.Fatalf("write marker INPUT: %v", err)
	}
	// /bin/cat 回显大块可能分块到达（沿用累积模式），读 ctx 放宽 10s。
	rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer rcancel()
	got := make([]byte, 0, len(payload)+len(marker))
	for !bytes.HasSuffix(got, marker) {
		_, data, err := c.Read(rctx)
		if err != nil {
			t.Fatalf("read OUTPUT: %v (got %d bytes so far)", err, len(got))
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame head: %v", data[:min(len(data), 16)])
		}
		got = append(got, data[1:]...)
		if len(got) > len(payload)+len(marker) {
			t.Fatalf("echo overflow: got %d bytes, want <= %d", len(got), len(payload)+len(marker))
		}
	}
	// 回显前缀（标记前）必须全为 'A' 且长度 ∈ [1,16383]：Linux 恰 16383，
	// Darwin 恰 2048（见函数头注释），其余值均为异常。
	prefix := got[:len(got)-len(marker)]
	if len(prefix) == 0 || len(prefix) > len(payload) {
		t.Fatalf("echo prefix len = %d, want in [1, %d]", len(prefix), len(payload))
	}
	for i, b := range prefix {
		if b != 'A' {
			t.Fatalf("echo prefix byte #%d = %q, want 'A'", i, b)
		}
	}

	// 边界外：'0' + 16384 = 总长 16385 → 库累积字节硬顶在收尾读时切断（1009）。
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, make([]byte, proto.ReadLimitPostAuth)...)); err != nil {
		t.Fatalf("write oversize INPUT: %v", err)
	}
	ce := readCloseErr(t, c, ctx)
	if ce.Code != websocket.StatusMessageTooBig {
		t.Fatalf("close code = %d, want %d (1009)", ce.Code, websocket.StatusMessageTooBig)
	}
	// 多客户端推论：断开不触发 exitf（静默反证）。
	assertNoExit(t, exitCh)
}

// TestFragmentedFlood1009（RES-01 等效防线主证据）：1 字节 × N 分片洪水在累积
// 16385 字节处被库自动 1009 切断——内存 ≤16KiB、CPU ≈ 16K 次帧头解析（毫秒级）。
//
// 客户端 Writer 精确构造分片流：每次 Write 产生一个非 fin continuation 帧
// （write.go:223-264），无裸帧 helper、无手写帧字节（D-09 修订后测试矩阵全部
// 库客户端可覆盖）。断言放宽形态：服务端切断后连接随 Attach 终结关闭，Write
// 循环中出错即 break，最终 CloseError 必须是 1009。
func TestFragmentedFlood1009(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	w, err := c.Writer(ctx, websocket.MessageBinary)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	for i := 0; i < proto.ReadLimitPostAuth+100; i++ {
		if _, err := w.Write([]byte{0x41}); err != nil {
			break // 服务端已切断：连接死亡后的写错误属预期
		}
	}
	_ = w.Close() // 连接已死，忽略错误

	ce := readCloseErr(t, c, ctx)
	if ce.Code != websocket.StatusMessageTooBig {
		t.Fatalf("close code = %d, want %d (1009)", ce.Code, websocket.StatusMessageTooBig)
	}
	// 多客户端推论：断开不触发 exitf（静默反证）。
	assertNoExit(t, exitCh)
}

// TestEmptyFragmentFloodResilience（RES-01，D-09 修订残余风险的显式断言形态）：
// 0 字节空消息洪水 ×5000 下服务存活、连接不断、echo 功能正常、内存平坦。
//
// D-09 修订：空帧无应用层钩子（库 read.go:457-479 内部吞掉空 continuation 帧，
// 字节计数不递减故字节硬顶永不触发）——本测试断言"存活+内存平坦"而非 1009；
// 残余 CPU 消耗受攻击者带宽约束（客户端 mask 帧 ≥6 字节/帧），预认证窗口另有
// 5s 超时 + per-IP 8 盒住（409 单客户端门已随 Phase 5 注册表拆除，max-clients
// 503 闸 05-07 重建）。内存断言为宽松参考防线（backstop 非精确门禁）：
// HeapAlloc 增量 < 8MiB，防回归性分配（SEC-08 结构目标：无消息级缓冲预分配）。
func TestEmptyFragmentFloodResilience(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 采样前 GC：HeapAlloc 近似活堆（sync.Pool 等缓存随 GC 清空，增量读数稳定）。
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	for i := 0; i < 5000; i++ {
		if err := c.Write(ctx, websocket.MessageBinary, []byte{}); err != nil {
			t.Fatalf("empty flood write #%d: %v — 空帧洪水下连接被切断", i, err)
		}
	}

	// 存活证据一：洪水后 INPUT echo 全链路功能正常（累积收齐）。
	payload := []byte("alive-after-empty-flood")
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write INPUT after flood: %v", err)
	}
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read OUTPUT after flood: %v (got %q so far)", err, got)
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame: %v", data)
		}
		got = append(got, data[1:]...)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo payload = %q, want %q", got, payload)
	}

	// 存活证据二：exitf 未被提前触发（连接存活、单次生命周期未被消耗）。
	select {
	case code := <-exitCh:
		t.Fatalf("exitf called during empty flood (code=%d) — 空帧洪水触发终结", code)
	default:
	}

	// 内存平坦宽松防线：采样前再 GC，HeapAlloc 增量断言 < 8MiB。
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	delta := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)
	t.Logf("HeapAlloc delta after 5000 empty messages: %d bytes", delta)
	if delta > 8*1024*1024 {
		t.Fatalf("HeapAlloc grew by %d bytes (> 8MiB) — 空帧洪水存在回归性分配", delta)
	}

	c.Close(websocket.StatusNormalClosure, "")
	assertNoExit(t, exitCh)
}

// TestPreHelloReadLimit（SEC-08/D-11）：预认证 4KiB 档——Dial（带 wesh.v1 子协议）
// 成功但不发 Hello，直接发 >4KiB 消息，库在预认证窗口首读处自动 1009（ReadLimitPreAuth
// 档生效；此路径同时命中 02-05 Task 1 预认证首读的 message_too_big 埋点）。
func TestPreHelloReadLimit(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{Subprotocols: []string{proto.Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	if err := c.Write(ctx, websocket.MessageBinary, make([]byte, proto.ReadLimitPreAuth+1)); err != nil {
		t.Fatalf("write pre-auth oversize: %v", err)
	}
	ce := readCloseErr(t, c, ctx)
	if ce.Code != websocket.StatusMessageTooBig {
		t.Fatalf("close code = %d, want %d (1009)", ce.Code, websocket.StatusMessageTooBig)
	}
	// 多客户端推论：断开不触发 exitf（静默反证）。
	assertNoExit(t, exitCh)
}
