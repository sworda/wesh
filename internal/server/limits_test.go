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
	"context"
	"errors"
	"io"
	"os"
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
