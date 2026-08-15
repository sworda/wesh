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
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

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
// CloseError 为准）。恢复时序安全：logEvent 与 exitf 同 goroutine 程序序，
// waitExit 返回即 stderr 行已落盘（channel happens-before），恢复无竞态。
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
// 随后 exitf(0) 经既有 D-11 单一路径收口。
func TestOversize1009(t *testing.T) {
	restore := captureStderr(t)
	defer restore() // 失败路径兜底恢复 os.Stderr（幂等，成功路径显式调用后空转）

	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})

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
	// logEvent 与 exitf 同 goroutine 程序序——waitExit 返回即 stderr 行必然已写出。
	waitExit(t, exitCh, 0)

	// D-12② 超限可见性三腿之二：stderr 恰一行 message_too_big 事件，三要素齐全。
	out := restore()
	if n := strings.Count(out, "message_too_big"); n != 1 {
		t.Fatalf("stderr message_too_big count = %d, want exactly 1 (out=%q)", n, out)
	}
	if !strings.Contains(out, "remote=127.0.0.1:") ||
		!strings.Contains(out, "code=1009") ||
		!strings.Contains(out, "reason=message_too_big") {
		t.Fatalf("stderr event missing 三要素 (remote/code/reason): %q", out)
	}
}

// TestReadLimitBoundary（RES-01）：两层硬顶边界精确——总长恰 16384 的 INPUT 帧
// 正常处理（cat 回显 16383 字节载荷收齐），总长 16385 的 INPUT 帧被 1009 切断。
//
// 载荷用 'A' 不用零字节（plan 字面 zeros 的实测偏差修正）：NUL 是控制字符，
// PTY 规范模式 ECHOCTL 将其回显为 "^@" 两字节（实测 16383 零字节回显 32766
// 字节），回显计数断言语义失真；可打印字符回显 1:1（实测 16383 'A' 回显恰
// 16383 字节、~22ms 收齐）。边界值一律引用 proto 常量而非硬编码数字。
func TestReadLimitBoundary(t *testing.T) {
	exitCh, wsURL := startTestServerWith(t, []string{"/bin/cat"}, server.Options{Writable: true})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := dialHello(t, ctx, wsURL, 80, 24)

	// 边界内：'0' + 16383 = 总长恰 16384。
	payload := bytes.Repeat([]byte("A"), proto.ReadLimitPostAuth-1)
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, payload...)); err != nil {
		t.Fatalf("write boundary INPUT: %v", err)
	}
	// /bin/cat 回显大块可能分块到达（沿用累积模式），读 ctx 放宽 10s。
	rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer rcancel()
	got := make([]byte, 0, len(payload))
	for len(got) < len(payload) {
		_, data, err := c.Read(rctx)
		if err != nil {
			t.Fatalf("read OUTPUT: %v (got %d/%d bytes so far)", err, len(got), len(payload))
		}
		if len(data) == 0 || data[0] != proto.Output {
			t.Fatalf("unexpected frame head: %v", data[:min(len(data), 16)])
		}
		got = append(got, data[1:]...)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("echo payload mismatch: got %d bytes, want %d", len(got), len(payload))
	}

	// 边界外：'0' + 16384 = 总长 16385 → 库累积字节硬顶在收尾读时切断（1009）。
	if err := c.Write(ctx, websocket.MessageBinary, append([]byte{proto.Input}, make([]byte, proto.ReadLimitPostAuth)...)); err != nil {
		t.Fatalf("write oversize INPUT: %v", err)
	}
	ce := readCloseErr(t, c, ctx)
	if ce.Code != websocket.StatusMessageTooBig {
		t.Fatalf("close code = %d, want %d (1009)", ce.Code, websocket.StatusMessageTooBig)
	}
	waitExit(t, exitCh, 0)
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
	waitExit(t, exitCh, 0)
}

// TestEmptyFragmentFloodResilience（RES-01，D-09 修订残余风险的显式断言形态）：
// 0 字节空消息洪水 ×5000 下服务存活、连接不断、echo 功能正常、内存平坦。
//
// D-09 修订：空帧无应用层钩子（库 read.go:457-479 内部吞掉空 continuation 帧，
// 字节计数不递减故字节硬顶永不触发）——本测试断言"存活+内存平坦"而非 1009；
// 残余 CPU 消耗受攻击者带宽约束（客户端 mask 帧 ≥6 字节/帧），预认证窗口另有
// 5s 超时 + per-IP 8 + 409 门盒住。内存断言为宽松参考防线（backstop 非精确门禁）：
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
	waitExit(t, exitCh, 0)
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
	waitExit(t, exitCh, 0)
}
