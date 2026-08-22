// Package proto 是 wesh 数据面协议的单一事实源。
//
// 帧格式：binary frame = 1 字节 ASCII 类型 + 载荷；控制帧（Hello/Welcome/Error）
// 载荷为 JSON，数据帧（INPUT/OUTPUT/RESIZE）沿用现状形状（D-01）。
//
// 前端 web/src/main.ts 的帧常量与本文件手工对齐，两侧注释互相指路（D-16）。
//
// 关闭码纪律（D-05 全集 {1000,1001,1002,1008,1009,1011,1013}）：
// 发送侧取值直接用库常量 websocket.StatusNormalClosure/StatusProtocolError/
// StatusPolicyViolation/StatusMessageTooBig/StatusInternalError；
// 1013 背压踢出已于 Phase 5 启用（D-08 占位兑现——发送路径 =
// server/clients.go kickSlowConsumerLocked，库常量 websocket.StatusTryAgainLater，
// close reason 机器串 slow_consumer）；1001 优雅下线 Phase 7 启用，本期占位不实现；
// 1005/1006/1015 永不发送（库层 validWireCloseCode 兜底）；禁止自定义 4000 段私码。
package proto

import "encoding/json"

const (
	Input  = '0' // 0x30, C→S, raw bytes → 写 master
	Resize = '1' // 0x31, C→S, JSON {"cols":C,"rows":R} → 钳制 1..1000 后 Setsize
	Output = '0' // 0x30, S→C, master 读块直发

	Hello   = 'H' // 0x48, C→S, JSON {"version":V,"cols":C,"rows":R}
	Welcome = 'W' // 0x57, S→C, JSON {"mode":"ro"|"rw","cols":C,"rows":R,"prefs"?}——P4 起可携可选
	// prefs（D-13 一次性下发）；G-05-1（2026-08-22 方向 A 裁决）起恒携会话尺寸 cols/rows 键
	//（P2 D-02 加键兼容增量：旧前端忽略未知键，新前端对缺席键的旧服务端不约束渲染）；
	// 05-03 起运行期再推送用于 owner 递补升格通知（R-09——P2 D-01/D-02 纪律：
	// 既有帧类型的运行期再推送不算动协议，零新类型字节）；G-05-1 起运行期再推送同为
	// 尺寸下发通道（recalcNow 检测到会话尺寸变化时按各端当前 mode 组帧广播）
	Error = 'E' // 0x45, S→C, JSON {"code":C,"message":M}
	// 'X' EXIT / 'T' TITLE / 'P' PREFS —— 类型字节本 phase 占住，语义分属 Phase 6/4（D-01）；
	// 'P' 帧运行期推送仍 v2 再议——P4 prefs 仅经 Welcome 内嵌一次性下发（D-13）。
)

// Subprotocol 子协议 token：HTTP 预检（Sec-WebSocket-Protocol 头）、
// AcceptOptions.Subprotocols 协商值、Hello.version 期望值三处复用同一常量
// （D-03，Open Question 3 裁决，双写漂移面最小）。
const Subprotocol = "wesh.v1"

// Welcome 帧 mode 取值（D-14，前后端对齐字符串入 proto）。
const (
	ModeRO = "ro" // 只读：服务端丢弃 INPUT，前端 disableStdin + 标题 [ro] 前缀
	ModeRW = "rw" // 可写：--writable 开启
)

// Error codes（D-06 受众分治：仅正常客户端可见的错误发 Error 帧 + 关闭码；
// 攻击面路径 unknown_frame/抢跑帧/超限/hello_timeout 只发关闭码不发 Error 帧——
// 不给攻击者反馈面）。code 为 snake_case 机器串，主动关闭的 close reason 带同名
// 机器串（RFC6455 ≤123 字节，D-07）。auth_failed 已于 Phase 3 兑现；
// permission_denied 保持占位不硬用（05-03 CONTEXT 裁断：owner 模式降级走
// Welcome{mode:"ro"} 而非 Error 帧，无真实使用场景——P3 deferred 入表挂账纪律）。
const (
	ErrVersionMismatch = "version_mismatch" // 正常客户端可见，发 Error 帧 + 1008
	ErrServerError     = "server_error"     // 发 Error 帧 + 1011
	// ticket 核销失败统一口径（Phase 3 D-10）：过期/非法/重放/节流中同口径，
	// 无 oracle；发 Error 帧 + 1008，正常客户端可见码（D-06 受众分治）。
	ErrAuthFailed = "auth_failed"
)

// 读上限两档（D-10 一律常量不开 CLI flag；单帧与累积字节同由 SetReadLimit 库执行，
// 超限库自动 1009）。标定依据：合法 C→S 流量极小（键盘 INPUT 字节级、
// RESIZE/Hello JSON <200B、粘贴几 KB），浏览器 WS API 不产生分片。
const (
	// ReadLimitPreAuth 预认证读上限（D-11：Accept 后 Hello 完成前）。
	// Hello JSON 实测 ~45-100 字节，4KiB 余量两个数量级——SEC-08 预认证窗口
	// 单连接可占内存最小化。
	ReadLimitPreAuth = 4 * 1024
	// ReadLimitPostAuth 稳态读上限（D-09 修订：Hello 完成后切换）。
	ReadLimitPostAuth = 16 * 1024
)

// 分片数上限注释位（值 32，D-09 2026-08-15 用户裁决修订）：
// coder/websocket 不暴露分片计数 API（read.go:457-479 空 continuation 帧在
// mr.read 内部循环被吞掉，应用层数不到分片），本层由等效防线覆盖——
// 两层字节硬顶（ReadLimitPreAuth/ReadLimitPostAuth）+ 预认证三道闸
//（D-03 400 / D-04 429 / 5s 超时）；409 单客户端门已于 Phase 5 拆除（多客户端
// 注册表取代，max-clients 503 闸由 05-07 重建）；
// Bandit CVE-2026-65623 官方修复同为 running byte count 而非分片计数。
// 残余风险：0 字节空帧洪水（纯 CPU、带宽受限、内存平坦）无应用层钩子。

// HelloPayload 显式 json tag，防字段名漂移。
// 未知字段由 json.Unmarshal 默认忽略——D-02 演化纪律的零成本实现
// （禁止 DisallowUnknownFields；ticket 已于 Phase 3 落地，Phase 5 加 attach/mode
// 仍只是加字段）。Ticket 为一次性认证票（omitempty：无认证模式前端省略字段，
// JSON 不出 ticket 键；唯一传输通道，禁止走 URL query/子协议头——ARCHITECTURE §2.8）。
type HelloPayload struct {
	Version string `json:"version"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	Ticket  string `json:"ticket,omitempty"`
}

// WelcomePayload 显式 json tag。Mode 取值见 ModeRO/ModeRW（D-14）。
// Prefs 为 P4 D-13 客户端偏好一次性下发通道（--client-option 聚合 + osc52 并入），
// 服务端不透明透传不解析；omitempty：nil/空时 JSON 不出 prefs 键——旧前端零漂移
// （P2 D-02 加字段不动协议纪律，与 HelloPayload.Ticket 同形态）。
// Cols/Rows 为当前会话尺寸（G-05-1，2026-08-22 用户裁决方向 A）：刻意不加
// omitempty——会话尺寸恒在（含零参与者期间的 80x24 spawn 回落），新前端靠
// 「缺席 = 旧服务端」识别遗留形态。根因登记：D-09 min-rect 不变量「无需 S→C 尺寸
// 下发」的假设只覆盖绝对寻址流（光标定位）与纯文本流，不覆盖 readline 行编辑等
// 相对寻址流——行编辑按终端宽度生成环绕点/光标上行，40 列 PTY 字节流在 120 列端
// 换行点分叉产生异尺寸双端叠写；会话尺寸下发使各端按同尺寸约束渲染（05-11 前端
// 消费），同 cols 渲染同字节流 = 两端逐屏严格一致。
type WelcomePayload struct {
	Mode  string          `json:"mode"`
	Cols  int             `json:"cols"` // G-05-1：会话 cols（恒序列化，无 omitempty）
	Rows  int             `json:"rows"` // G-05-1：会话 rows（恒序列化，无 omitempty）
	Prefs json.RawMessage `json:"prefs,omitempty"`
}

// ErrorPayload 显式 json tag。Code 为 snake_case 机器串，Message 为英文人话
// （前端直接展示，D-07）。
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DecodeHello 解码 Hello 帧载荷 {"version":V,"cols":C,"rows":R}。
// 解码失败返回 ok=false；成功时 Cols/Rows 经 ClampDim 钳制到 [1,1000] 后返回。
// 不做 version 校验——校验语义在 server 握手段（02-02）。
func DecodeHello(payload []byte) (HelloPayload, bool) {
	var hp HelloPayload
	if err := json.Unmarshal(payload, &hp); err != nil {
		return HelloPayload{}, false
	}
	hp.Cols = ClampDim(hp.Cols)
	hp.Rows = ClampDim(hp.Rows)
	return hp, true
}

// WelcomeFrame 组 Welcome 帧：1 字节类型 + JSON {"mode":M,"cols":C,"rows":R,"prefs"?}，
// 调用方直接 c.Write（与 onChunk 的 1+payload 组帧模式同构）。prefs 为 P4 D-13 内嵌
// 下发的客户端偏好 blob——nil/空时 omitempty 使 JSON 不出 prefs 键（旧前端零漂移）。
// cols/rows 为当前会话尺寸（G-05-1）：恒序列化（无 omitempty——「缺席 = 旧服务端」
// 识别契约），调用方取值须与 PTY 实际尺寸同源（server.sessionDimsLocked——参与期为
// 仲裁器 last，零参与者期间为 spawn 80x24 回落）。三条组帧通道：attach 升档 Welcome
// / 递补升格 Welcome（R-09）/ 运行期尺寸变化再推送（G-05-1，recalcNow 唯一挂点）。
// 固定 schema 下 json.Marshal 不会失败。
func WelcomeFrame(mode string, prefs json.RawMessage, cols, rows int) []byte {
	b, _ := json.Marshal(WelcomePayload{Mode: mode, Cols: cols, Rows: rows, Prefs: prefs})
	return append([]byte{Welcome}, b...)
}

// ValidClientOptionKey 判定 key 是否在客户端偏好白名单内（P4 D-14 白名单制防任意
// option 注入——allowProposedApi 等危险面结构性排除）：恰 10 键 = 8 个 xterm 视觉键
// + resizeOverlay/confirmBeforeUnload 2 个前端行为键（FE-06 开关）。osc52 刻意不在
// 内（D-12 安全不对称——安全敏感项只能经服务端 --osc52 开启，--client-option 与
// URL query 均不得触碰）。刻意用直白 switch，不引入新类型/注册表抽象（反过度设计）。
func ValidClientOptionKey(key string) bool {
	switch key {
	case "fontSize", "fontFamily", "cursorBlink", "cursorStyle",
		"scrollback", "lineHeight", "letterSpacing", "theme",
		"resizeOverlay", "confirmBeforeUnload":
		{
			return true
		}
	default:
		{
			return false
		}
	}
}

// ErrorFrame 组 Error 帧：1 字节类型 + JSON {"code":C,"message":M}，调用方直接
// c.Write。固定 schema 下 json.Marshal 不会失败。
func ErrorFrame(code, message string) []byte {
	b, _ := json.Marshal(ErrorPayload{Code: code, Message: message})
	return append([]byte{Error}, b...)
}

// resizePayload 显式 json tag，防字段名漂移。
type resizePayload struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// DecodeResize 解码 RESIZE 帧载荷 {"cols":C,"rows":R}。
// 解码失败返回 ok=false（调用方静默丢弃，不关连接）；
// 成功时 cols/rows 经 ClampDim 钳制到 [1,1000] 后返回（D-16）。
func DecodeResize(payload []byte) (cols, rows int, ok bool) {
	var rp resizePayload
	if err := json.Unmarshal(payload, &rp); err != nil {
		return 0, 0, false
	}
	return ClampDim(rp.Cols), ClampDim(rp.Rows), true
}

// ClampDim 将终端尺寸钳制到 [1,1000]（PITFALLS C10：0/NaN/超大尺寸防御）。
func ClampDim(v int) int {
	if v < 1 {
		return 1
	}
	if v > 1000 {
		return 1000
	}
	return v
}
