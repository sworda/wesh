package server_test

// log_test.go —— 08-01（OPS-08，D-13/D-15/D-18）slog JSON 事件断言面：
// parseEvents/countByEvent 是全部 stderr 事件断言迁移后的唯一消费形态
//（按行 JSON 解析，禁止子串/正则断言 JSON 行——JSON 转义/键序下正则脆，
// 08-CONTEXT Discretion 逐字纪律）；TestLogEventJSON 端到端锁定 D-13 迁移
// 基座（动态 stderr writer 保 captureStderr 语义 + D-18 schema 六键）。
//
// 断言纪律（08-RESEARCH Pattern 5 / Pitfall 4）：
//   - JSON 数字解进 map[string]any 是 float64——code 等数字字段按 float64 比；
//   - 行尾锚定在 JSON 字段精确相等语义下天然消解（exit_when_empty 与
//     exit_when_empty_wait 是两个独立 event 名，无前缀歧义）；
//   - 凭据/ticket/token 红线负断言（「全文不含敏感串」）与 JSON 化正交，
//     各测试文件内逐字保留子串形态，不经本 helper。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/server"
)

// parseEvents 把捕获的 stderr 按行解析为事件 map 集——跳过非 '{' 起始行
//（D-16 启动警告行保持文本 + panic 栈等混合流成员，不得因非 JSON 行 FAIL）；
// '{' 起始行单行非法 JSON 即 FAIL 并带行内容（格式完整性即审计完整性，
// T-08-01c 无双轨漂移面的断言侧防线）。
func parseEvents(t *testing.T, captured string) []map[string]any {
	t.Helper()
	var evs []map[string]any
	for _, line := range strings.Split(captured, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("事件行非合法 JSON: %q: %v", line, err)
		}
		evs = append(evs, m)
	}
	return evs
}

// countByEvent 统计事件名精确相等（D-18：事件名走独立 event 字段）的条数。
func countByEvent(evs []map[string]any, name string) int {
	n := 0
	for _, m := range evs {
		if m["event"] == name {
			n++
		}
	}
	return n
}

// TestLogEventJSON（OPS-08，D-13/D-18）：logEvent 迁移 slog JSONHandler 后
// 端到端形态——captureStderr（动态 writer 语义：事件行在 os.Stderr 置换后
// 仍入管道）→ 稳态超限触发 1009 message_too_big → waitHandlers 同步边 →
// parseEvents 断言恰一条事件、六键齐备（time/level/msg/event/remote/code）、
// 未配置 --auth-header 时不出 remote_user 键（空串/缺省不出键语义）。
func TestLogEventJSON(t *testing.T) {
	restore := captureStderr(t)
	defer restore()

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
	assertNoExit(t, exitCh)

	out := restore()
	evs := parseEvents(t, out)
	if n := countByEvent(evs, "message_too_big"); n != 1 {
		t.Fatalf("message_too_big event count = %d, want exactly 1 (out=%q)", n, out)
	}
	var ev map[string]any
	for _, m := range evs {
		if m["event"] == "message_too_big" {
			ev = m
		}
	}
	// D-18 schema：msg 恒 "event"、level 恒 "INFO"（D-15）、事件名走 event 字段。
	if ev["msg"] != "event" {
		t.Fatalf(`msg = %v, want "event"（D-18）`, ev["msg"])
	}
	if ev["level"] != "INFO" {
		t.Fatalf(`level = %v, want "INFO"（D-15）`, ev["level"])
	}
	// JSON 数字解码为 float64（Pitfall 4）——code 按 float64 比 1009。
	if ev["code"] != float64(websocket.StatusMessageTooBig) {
		t.Fatalf("code = %v (%T), want float64(1009)", ev["code"], ev["code"])
	}
	remote, _ := ev["remote"].(string)
	if !strings.HasPrefix(remote, "127.0.0.1:") {
		t.Fatalf("remote = %q, want 127.0.0.1: 前缀", remote)
	}
	if tm, _ := ev["time"].(string); tm == "" {
		t.Fatalf("time 键缺失或为空（slog 默认键，D-15）: %v", ev)
	}
	// 空串/缺省不出键（与迁移前文本行语义一致）：未配置 --auth-header。
	if _, ok := ev["remote_user"]; ok {
		t.Fatalf("remote_user 键不应出现（未配置 --auth-header）: %v", ev)
	}
}
