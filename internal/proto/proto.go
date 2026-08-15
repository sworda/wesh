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
// 1001 优雅下线 Phase 7 启用、1013 背压踢出 Phase 5 启用，本期占位不实现（D-08）；
// 1005/1006/1015 永不发送（库层 validWireCloseCode 兜底）；禁止自定义 4000 段私码。
package proto

import "encoding/json"

const (
	Input  = '0' // 0x30, C→S, raw bytes → 写 master
	Resize = '1' // 0x31, C→S, JSON {"cols":C,"rows":R} → 钳制 1..1000 后 Setsize
	Output = '0' // 0x30, S→C, master 读块直发

	Hello   = 'H' // 0x48, C→S, JSON {"version":V,"cols":C,"rows":R}
	Welcome = 'W' // 0x57, S→C, JSON {"mode":"ro"|"rw"}
	Error   = 'E' // 0x45, S→C, JSON {"code":C,"message":M}
	// 'X' EXIT / 'T' TITLE / 'P' PREFS —— 类型字节本 phase 占住，语义分属 Phase 6/4（D-01）
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
// 机器串（RFC6455 ≤123 字节，D-07）。auth_failed/permission_denied 属 Phase 3/5（deferred）。
const (
	ErrVersionMismatch = "version_mismatch" // 正常客户端可见，发 Error 帧 + 1008
	ErrServerError     = "server_error"     // 发 Error 帧 + 1011
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
//（D-03 400 / D-04 429 / 5s 超时）+ 409 单客户端门；
// Bandit CVE-2026-65623 官方修复同为 running byte count 而非分片计数。
// 残余风险：0 字节空帧洪水（纯 CPU、带宽受限、内存平坦）无应用层钩子。

// HelloPayload 显式 json tag，防字段名漂移。
// 未知字段由 json.Unmarshal 默认忽略——D-02 演化纪律的零成本实现
// （禁止 DisallowUnknownFields；Phase 3 加 ticket、Phase 5 加 attach/mode 只是加字段）。
type HelloPayload struct {
	Version string `json:"version"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
}

// WelcomePayload 显式 json tag。Mode 取值见 ModeRO/ModeRW（D-14）。
type WelcomePayload struct {
	Mode string `json:"mode"`
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

// WelcomeFrame 组 Welcome 帧：1 字节类型 + JSON {"mode":M}，调用方直接 c.Write
// （与 onChunk 的 1+payload 组帧模式同构）。固定 schema 下 json.Marshal 不会失败。
func WelcomeFrame(mode string) []byte {
	b, _ := json.Marshal(WelcomePayload{Mode: mode})
	return append([]byte{Welcome}, b...)
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
