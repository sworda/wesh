// Package proto 是 wesh 数据面协议的单一事实源。
//
// 帧格式：binary frame = 1 字节 ASCII 类型 + 载荷（ttyd 同构形状、wesh 自定义取值）。
// 前端 web/src/main.ts 的帧常量与本文件手工对齐（D-16）。
//
// Phase 2 预留（现在不实现；Phase 1 收到未知类型由 server 以 1002 关闭）：
//   - 类型字节 '2' PAUSE / '3' RESUME / 'E' ERROR+JSON / 'X' EXIT+退出码
//   - 子协议 wesh.v1 协商（AcceptOptions.Subprotocols 一行开启）
//
// 关闭码纪律：主动发送仅 1000（子进程退出正常关）与 1002（未知帧）；
// 1009 由 SetReadLimit 默认自动产生；1006 永不写入（RFC6455 §7.4，PITFALLS C9）。
package proto

import "encoding/json"

const (
	Input  = '0' // 0x30, C→S, raw bytes → 写 master
	Resize = '1' // 0x31, C→S, JSON {"cols":C,"rows":R} → 钳制 1..1000 后 Setsize
	Output = '0' // 0x30, S→C, master 读块直发
)

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
