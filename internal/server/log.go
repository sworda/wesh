package server

// log.go —— 运行期事件日志基础设施（08-01，OPS-08，D-13/D-15/D-18）。
// 决策依据登记（origin.go/proxy.go 同位文件组织纪律：包级 + 注释头登记决策号）：
//
//   - D-13：logEvent 全量原子迁移 slog JSONHandler——唯一事件出口内部换实现，
//     全部 18 个调用点零改动；stderr 运行期事件从单行文本变 JSON 单行。
//     拒绝双轨（同一事件既出旧文本行又出 JSON 行的漂移面不存在——单点切换）。
//     迁移来源：server.go 原 L1047-1077 的 fmt.Fprintf 文本实现（本期已删除）。
//   - D-15：运行期事件恒 JSON 恒 INFO，无 --log-format/--log-level（零新 CLI
//     契约）；time/level/msg 用 slog 默认键，time 为 RFC3339Nano 携进程本地
//     时区偏移（stdlib 固定不可配；毫秒截断仅 TextHandler 路径，JSONHandler
//     为纳秒——GOROOT json_handler.go appendJSONTime 实证，08-REVIEW WR-01
//     修正）。人读检索走 jq。
//   - D-18：schema = msg 恒 "event" + 事件名走独立 event 字段，其余字段平铺
//     （remote/code/remote_user...）——jq/Loki 检索 event=="x" 直打字段索引。
//   - 启动行/分享链接行（含 token）与 wesh: warning: 警告行保持人读文本不变
//     （D-14/D-16）——operator 交互输出与机器审计事件两通道分流，不经本文件。
//
// 装配形态（08-RESEARCH Pattern 1/Open Question 倾向）：每个 Server 实例持有
// 自己的 slog.Logger（New 装配期构造，writer 于同一刻快照），不调
// slog.SetDefault（不污染全局默认 logger；D-15 恒 JSON 恒 INFO 无配置面）。
// per-Server 而非包级单例是测试隔离的结构性要求：跨测试遗留 goroutine 的
// 迟到事件只能写入它所属实例的 writer，不会串入下一测试的捕获管道。
// JSONHandler 内部 mu 串行化记录（GOROOT json_handler.go:36），并发 emit
// 行级原子——每记录恒完整一行。

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/coder/websocket"
)

// stderrMu 只负责「装配期读 os.Stderr」与「测试置换 os.Stderr」之间的
// data race 同步：写入侧各自持有装配期快照的 writer，不再在每次 Write 时
// 反查全局变量。读侧 RLock、置换侧（export_test.go LockStderr）Lock。
//
// 历史教训：旧实现每次 Write 动态读 os.Stderr，使跨测试遗留 goroutine 的
// 事件写出会落进「下一个测试刚置换的捕获管道」，表现为 TestRemoteUserLogging
// 等断言读到前序测试的 detach/session_end（CI 169.196s 形态）。
var stderrMu sync.RWMutex

// newEventLogger 在 Server 装配期快照当前 os.Stderr 作为事件 writer。
// 快照点是根因修复：captureStderr 在装配前置换 os.Stderr，故本测试的
// server 固定写入自己的捕获管道；此后其他测试再置换全局变量，也不会
// 把本 server 遗留 goroutine 的输出导向新测试的管道。
func newEventLogger() *slog.Logger {
	stderrMu.RLock()
	w := os.Stderr
	stderrMu.RUnlock()
	return slog.New(slog.NewJSONHandler(w, nil))
}

// emitWith 是全部运行期事件的底层出口：msg 恒 "event"（D-18），属性集由
// 调用方组好传入。08-02 扩展字段事件（attach/detach+reason/session_* 等）
// 的挂点即本函数；本 plan（08-01）仅 logEvent 消费。
// 禁止 kv 交替参数形态（奇数/非 string 键产出 !BADKEY，GOROOT logger.go:187）
// ——一律 slog.String/Int 类型化 attr 经 LogAttrs 发出。
func emitWith(l *slog.Logger, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelInfo, "event", attrs...)
}

// eventLogger 返回实例 logger；零值 Server（白盒测试 &Server{} 直构造的
// 既有纪律，见 clients_test.go/resize_test.go）回退到装配期语义的默认
// logger——不写回字段，故并发调用无数据竞争。
func (s *Server) eventLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return newEventLogger()
}

// emitEvent 为 Server 实例出口（per-Server logger，不与进程全局共享）。
func (s *Server) emitEvent(attrs ...slog.Attr) {
	emitWith(s.eventLogger(), attrs...)
}

// logEvent 打 D-12② stderr 单行事件，三要素齐全：对端 remote、码值 code、
// reason 机器串。本期覆盖 hello_timeout/empty_frame/frame_before_hello/
// malformed_hello/version_mismatch/subprotocol_required（assert 兜底）/
// pong_timeout（02-04 保活）/message_too_big（02-05 超限，经 logIfMessageTooBig
// 挂预认证首读与稳态读循环两处）/auth_failed（03-03 ticket 核销失败）/
// throttled（03-03 HTTP 层 429 节流闸，basicAuth）/slow_consumer（05-01 背压
// 1013 踢出，clients.go kickSlowConsumerLocked）。08-01 起经 slog JSONHandler
// 输出单行 JSON（D-13 原子迁移完成，D-18 schema：event=<reason 串> 独立字段）。
//
// 07-03（SEC-07，D-15/D-19/D-20）：可选第四字段 remote_user——variadic 末参
// 非空时追加 remote_user 键（空串/缺省不出键，全部既有调用点零改动编译通过，
// 未配置 --auth-header 时事件行无该键）。值必须经 sanitizeRemoteUser 清洗
// （proxy.go：C0/C1/DEL 剥离 + 128 rune 截断，T-07-03b 日志注入防线）且来源
// 只能是 --auth-header 配置头名对应的 HTTP 头——本函数不做二次清洗（清洗在
// 提取点完成，单一写口纪律）。
//
// 红线（SEC-01）：凭据、ticket、Authorization 头任何形态（含 base64）禁止作为
// 任何参数传入（ttyd server.c:142 反例）——三要素只有 remote/code/reason；
// D-03 红线随第四字段延伸：token/ticket/凭据同样禁止作为 remote_user 传入
// （结构性保证：remote_user 提取源只能是配置头名的 HTTP 头，/s/ 路径 token
// 与 Hello ticket 不可能进入该提取路径，T-07-03c——见 proxy.go 注释头）。
// logEventWith 供无 Server 接收者的装配期辅助函数（basicAuth 中间件、
// rejectCapacity）使用；Server 方法形态见下方 logEvent。HTTP 层事件 code
// 复用 HTTP 状态码值（websocket.StatusCode 底层 int，PATTERNS 裁决）。
func logEventWith(l *slog.Logger, remote string, code websocket.StatusCode, reason string, remoteUser ...string) {
	attrs := []slog.Attr{
		slog.String("event", reason),
		slog.String("remote", remote),
		slog.Int("code", int(code)),
	}
	if len(remoteUser) > 0 && remoteUser[0] != "" {
		attrs = append(attrs, slog.String("remote_user", remoteUser[0]))
	}
	emitWith(l, attrs...)
}

// logEvent 为 Server 实例出口（per-Server logger）。
func (s *Server) logEvent(remote string, code websocket.StatusCode, reason string, remoteUser ...string) {
	logEventWith(s.eventLogger(), remote, code, reason, remoteUser...)
}
