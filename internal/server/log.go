package server

// log.go —— 运行期事件日志基础设施（08-01，OPS-08，D-13/D-15/D-18）。
// 决策依据登记（origin.go/proxy.go 同位文件组织纪律：包级 + 注释头登记决策号）：
//
//   - D-13：logEvent 全量原子迁移 slog JSONHandler——唯一事件出口内部换实现，
//     全部 18 个调用点零改动；stderr 运行期事件从单行文本变 JSON 单行。
//     拒绝双轨（同一事件既出旧文本行又出 JSON 行的漂移面不存在——单点切换）。
//     迁移来源：server.go 原 L1047-1077 的 fmt.Fprintf 文本实现（本期已删除）。
//   - D-15：运行期事件恒 JSON 恒 INFO，无 --log-format/--log-level（零新 CLI
//     契约）；time/level/msg 用 slog 默认键，time 为 RFC3339 毫秒 UTC（stdlib
//     固定不可配）。人读检索走 jq。
//   - D-18：schema = msg 恒 "event" + 事件名走独立 event 字段，其余字段平铺
//     （remote/code/remote_user...）——jq/Loki 检索 event=="x" 直打字段索引。
//   - 启动行/分享链接行（含 token）与 wesh: warning: 警告行保持人读文本不变
//     （D-14/D-16）——operator 交互输出与机器审计事件两通道分流，不经本文件。
//
// 装配形态（08-RESEARCH Pattern 1/Open Question 倾向）：server 包包级 eventLog
// 单例，不调 slog.SetDefault（不污染全局默认 logger，测试隔离性更好；D-15
// 恒 JSON 恒 INFO 无配置面）。JSONHandler 内部 mu 串行化记录（GOROOT
// json_handler.go:36），并发 emit 行级原子——每记录恒完整一行。

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/coder/websocket"
)

// stderrW 动态解析 os.Stderr——保持迁移前 logEvent「调用时解析 os.Stderr
// 变量」语义（08-RESEARCH Pitfall 1：NewJSONHandler 构造时捕获 io.Writer，
// 若直接传 os.Stderr 则 captureStderr 置换后事件行写进旧 writer，全部
// stderr 断言测试结构性失明）。每次 Write 调用时读 os.Stderr 变量是
// 捕获语义的唯一保真形态。
//
// stderrMu 串行化「读 os.Stderr」与「测试置换 os.Stderr」（08-01 门禁修正）：
// 动态 writer 的读与 captureStderr 的写都是对同一包级变量的裸访问，跨测试
// 遗留 handler goroutine 的事件写出会与下一测试的置换构成 data race——
// 读侧 RLock、置换侧（export_test.go LockStderr）Lock，两侧经同一互斥锁
// 建立 happens-before，race detector 认可。仅在读/置换瞬间持锁，不在整个
// 捕获期持写锁（否则事件写出全阻塞，waitHandlers 死锁）。
var stderrMu sync.RWMutex

type stderrW struct{}

func (stderrW) Write(p []byte) (int, error) {
	stderrMu.RLock()
	defer stderrMu.RUnlock()
	return os.Stderr.Write(p)
}

// eventLog 包级单例：JSONHandler(stderrW{}, nil)——nil opts 即默认键
// time/level/msg + LevelInfo 阈值（D-15 恒 INFO 无配置面）。
var eventLog = slog.New(slog.NewJSONHandler(stderrW{}, nil))

// emitEvent 是全部运行期事件的底层出口：msg 恒 "event"（D-18），属性集由
// 调用方组好传入。08-02 扩展字段事件（attach/detach+reason/session_* 等）
// 的挂点即本函数；本 plan（08-01）仅 logEvent 消费。
// 禁止 kv 交替参数形态（奇数/非 string 键产出 !BADKEY，GOROOT logger.go:187）
// ——一律 slog.String/Int 类型化 attr 经 LogAttrs 发出。
func emitEvent(attrs ...slog.Attr) {
	eventLog.LogAttrs(context.Background(), slog.LevelInfo, "event", attrs...)
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
// 包级函数（无 Server 状态依赖）：HTTP 层中间件（basicAuth）与 WS 握手段共用
// 唯一出口；HTTP 层事件 code 复用 HTTP 状态码值（websocket.StatusCode 底层 int，
// PATTERNS Shared Patterns 裁决）。
func logEvent(remote string, code websocket.StatusCode, reason string, remoteUser ...string) {
	attrs := []slog.Attr{
		slog.String("event", reason),
		slog.String("remote", remote),
		slog.Int("code", int(code)),
	}
	if len(remoteUser) > 0 && remoteUser[0] != "" {
		attrs = append(attrs, slog.String("remote_user", remoteUser[0]))
	}
	emitEvent(attrs...)
}
