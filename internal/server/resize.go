// resize.go：MULTI-04 resize 仲裁器（RESEARCH Pattern 4 逐字形态）——arbitrate
// 纯函数（0/1/N 参与者的 last-wins 与 min-rect）+ D-09 参与集分层（owner 模式
// 仅 owner / all 模式全部 rw 端 / 纯 ro 会话全部 ro 端 Hello 首尺寸 / 含可写端
// 会话的 ro 旁观者不参与）+ 50ms 防抖与即时重算双通道。
//
// 锁纪律（R-07 单锁延伸）：arbiter 全部字段（sizes/timer/last）由 hubMu 保护，
// 与注册表同锁——reportResize/addMember/removeMember/recalcNow 均须 hubMu 内调用；
// timer 回调自有 goroutine，入内先取 hubMu（锁序 hubMu > sess.fdMu，与升档序列
// 的 sess.Resize 调用同序，无反序路径）。
package server

import (
	"time"

	"github.com/sworda/wesh/internal/proto"
)

// dims 终端尺寸（cols×rows）。入仲裁前已经 proto.ClampDim 钳制 [1,1000]
//（DecodeHello/DecodeResize 既有钳制点，proto.go:165-182——纯函数不重复钳制），
// uint16 转换安全（sess.Resize 的 Winsize 转换既有纪律）。
type dims struct {
	cols, rows int
}

// arbitrate 是 MULTI-04 仲裁纯函数（RESEARCH Code Examples 逐字形态，D-09 矩阵
// 的算法内核）：members = 参与集成员的最新上报尺寸。
//
//	0 人 → 零值 dims{}（不动 PTY 保持现状的哨兵——调用方对零值不发起 Resize）；
//	1 人 → members[0]（last-wins：该成员尺寸）；
//	≥2 人 → min(cols)×min(rows) 最小公共矩形（min-rect 不变量：任何参与端窗口
//	        ≥ PTY 尺寸，各端按自己窗口渲染、多余面积留白——D-09 推论，无需
//	        S→C 尺寸下发帧；2→1 收缩后单成员分支自然恢复 last-wins）。
//
// 纯函数不读共享状态，resize_test.go 表测直接锁定（tickets_test.go 白盒先例）。
func arbitrate(members []dims) dims { // members = 参与集最新上报尺寸（已 ClampDim 钳制）
	switch len(members) {
	case 0:
		return dims{} // 无参与者：不动 PTY（保持现状）
	case 1:
		return members[0] // last-wins
	default:
		out := members[0]
		for _, m := range members[1:] {
			out.cols = min(out.cols, m.cols)
			out.rows = min(out.rows, m.rows)
		}
		return out
	}
}

// arbiter 仲裁器状态（全部字段 hubMu 保护）：sizes 仅参与集成员（D-09 分层——
// owner 模式仅 owner 一员 / all 模式全部生效 rw 端 / 无 --writable 纯 ro 会话
// 全部 ro 端；含可写端会话的 ro 旁观者永不入内，其 RESIZE 经 D-09 第二闸忽略）；
// timer 为 50ms 防抖单 time.Timer（RESIZE 上报 reset 合并风暴——PITFALLS
// Pitfall 10 SIGWINCH 风暴防线；attach/detach/递补升格走 recalcNow 即时重算
// 不防抖，无风暴风险，RESEARCH Pattern 4）；last 为当前 PTY 目标尺寸（变化
// 检测——目标尺寸不变则不发起 TIOCSWINSZ）。
type arbiter struct {
	sizes map[*client]dims
	timer *time.Timer
	last  dims
}

// initArbiter 装配仲裁器（New 在 hubCond 构造后、goroutine 启动前调用——timer
// 回调与 ReadLoop 都可能在装配返回后立刻触达 arbiter 字段）。timer 以
// AfterFunc 创建后立即 Stop 为 stopped 态：首次 reportResize 的 Reset 才武装
//（单 time.Timer reset 防抖形态，server.go helloTimeout AfterFunc 先例；
// 回调到期取 hubMu 做 recalcNow——Go 1.23+ timer 语义下 Reset 与回调并发安全，
// 重复触发只是幂等重算）。Don't Hand-Roll 纪律：除本装配点外无任何 goroutine
// 计时循环。
func (s *Server) initArbiter() {
	s.arbiter = arbiter{sizes: make(map[*client]dims)}
	s.arbiter.timer = time.AfterFunc(s.resizeDebounce, func() {
		s.hubMu.Lock()
		defer s.hubMu.Unlock()
		s.recalcNow()
	})
	s.arbiter.timer.Stop()
}

// reportResize 是 RESIZE 上报入口（调用方 = server.go RESIZE case，已过 ro
// 忽略闸，hubMu 内调用）：c 在参与集 → 更新其最新尺寸 → timer.Reset 防抖
//（s.resizeDebounce，默认 50ms，Options.ResizeDebounce 测试可覆写——高频
// 上报合并为窗口末一次重算，防 SIGWINCH 风暴）；c 不在参与集 → 直接忽略
//（D-09 第二闸的兜底层——ro 旁观者与纯 ro 会话运行期 RESIZE 均落此分支；
// 第一闸为 RESIZE case 的 mode 判定，第二闸此处成员判定，纵深防御）。
func (s *Server) reportResize(c *client, cols, rows int) {
	if _, ok := s.arbiter.sizes[c]; !ok {
		return
	}
	s.arbiter.sizes[c] = dims{cols: cols, rows: rows}
	s.arbiter.timer.Reset(s.resizeDebounce)
}

// addMember 参与集登记（hubMu 内调用）：attach 升档按 D-09 矩阵供给（rw 端
// 一律参与——owner 模式仅 owner 能为 rw，all 模式全部 rw 端；无 --writable
// 纯 ro 会话全部 ro 端以 Hello 首尺寸参与——其运行期 RESIZE 被 reportResize
// 忽略，运行期窗口缩放不上报，缩到小于 PTY 尺寸者看到裁剪画面、重新 attach
// 恢复，README 明示由 05-09 落地，RESEARCH Pattern 4 行为推论/A3）；递补升格
// 由 promoteNextLocked 调用（新 owner 尺寸接管）。
func (s *Server) addMember(c *client, d dims) {
	s.arbiter.sizes[c] = d
}

// removeMember 参与集移除（hubMu 内调用；非成员幂等 no-op）：detach/kick 的
// 注册表移除点同步调用——all 模式被踢 rw 端若滞留 sizes，其陈旧尺寸将永久
// 拖累 min-rect（幽灵成员把 PTY 压在离群者小窗口），成员移除与注册表移除
// 必须同点恰好一次。
func (s *Server) removeMember(c *client) {
	delete(s.arbiter.sizes, c)
}

// recalcNow 即时重算（hubMu 内调用，不防抖——attach/detach/递补升格调用点，
// 无风暴风险）：收集参与集成员 → arbitrate → 结果非零（零值 = 0 人哨兵，
// 不动 PTY 保持现状）且 ≠ last → sess.Resize 并更新 last。
//
// 目标尺寸变化才调 sess.Resize（P5-3：Linux 同尺寸 TIOCSWINSZ 内核不发
// SIGWINCH，且避免无谓 ioctl）。参数序 (cols, rows)——io_test.go:24-25
// 注释锁定，切勿按 (rows, cols) 序误传；会话 closed 时 Resize 返回
// os.ErrClosed，忽略（Attach 读循环既有纪律同款）。
func (s *Server) recalcNow() {
	members := make([]dims, 0, len(s.arbiter.sizes))
	for _, d := range s.arbiter.sizes {
		members = append(members, d)
	}
	target := arbitrate(members)
	if target == (dims{}) || target == s.arbiter.last {
		return
	}
	s.arbiter.last = target
	_ = s.sess.Resize(target.cols, target.rows)
}

// 参与集判定（D-09 矩阵逐字）：升档生效 mode 为 rw（owner 模式仅 owner /
// all 模式全部 rw 端），或无 --writable 纯 ro 会话的全部 ro 端（Hello 首
// 尺寸参与，否则会话冻结 80x24）；含可写端会话的 ro 旁观者不参与（其
// RESIZE 经 D-09 第二闸忽略，尺寸永不影响可写端 PTY 尺寸）。
func (s *Server) participates(effMode string) bool {
	return effMode == proto.ModeRW || !s.writable
}
