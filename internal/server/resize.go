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
	"github.com/sworda/wesh/internal/pty"
)

// dims 终端尺寸（cols×rows）。入仲裁前已经 proto.ClampDim 钳制 [1,1000]
// （DecodeHello/DecodeResize 既有钳制点，proto.go:165-182——纯函数不重复钳制），
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

// debouncer 是「单 time.Timer reset 防抖」共用件（12-02 抽取，ROADMAP「含」：
// arbiter 与每会话 RESIZE 直通（perclient.go resizeDeb）共用同一组件同一形态，
// 防两份防抖实现漂移——Pitfall 7 SIGWINCH 风暴防线的组件半侧）。形态沿用
// initArbiter 既有注释论证：AfterFunc 创建即 Stop 为 stopped 态，首次 Reset 才
// 武装；Go 1.23+ timer 语义下 Reset/Stop 与回调并发安全，重复触发幂等（回调方
// 自持锁序与身份复查）。时长恒由调用方传入：arbiter 与每会话实例同源消费
// s.resizeDebounce（defaultResizeDebounce/Options.ResizeDebounce，clients.go
// :55-58 唯一时长源）——不新增第二份常量，防双写漂移。
type debouncer struct {
	timer *time.Timer
}

// newDebouncer 构造即 stopped 态（AfterFunc 创建后立即 Stop——到点前永不触发，
// 首次 Reset 才武装）。fn 为到期回调：arbiter 实例入内先取 hubMu（锁序
// hubMu > sess.fdMu，recalcNow 既有序列）；per-session 实例只取 resizeMu 叶锁
// 取 pendingResize 后放锁再 sess.Resize（仅 fdMu，回调函数体绝不取 hubMu——
// 锁序三规则 §5，perclient.go 12-02）。
func newDebouncer(d time.Duration, fn func()) *debouncer {
	db := &debouncer{}
	db.timer = time.AfterFunc(d, fn)
	db.timer.Stop()
	return db
}

// Reset 重置防抖窗（d 后触发回调；窗口内重复 Reset 合并为窗口末一次）。
func (db *debouncer) Reset(d time.Duration) {
	db.timer.Reset(d)
}

// Stop 停止防抖（计时器随属主消亡——per-session 实例的 teardown 挂点；
// stopped 态下 Reset 可再武装，Go 1.23+ timer 语义）。
func (db *debouncer) Stop() {
	db.timer.Stop()
}

// arbiter 仲裁器状态（全部字段 hubMu 保护）：sizes 仅参与集成员（D-09 分层——
// owner 模式仅 owner 一员 / all 模式全部生效 rw 端 / 无 --writable 纯 ro 会话
// 全部 ro 端；含可写端会话的 ro 旁观者永不入内，其 RESIZE 经 D-09 第二闸忽略）；
// timer 为 50ms 防抖（共用件 debouncer，RESIZE 上报 reset 合并风暴——PITFALLS
// Pitfall 10 SIGWINCH 风暴防线；attach/detach/递补升格走 recalcNow 即时重算
// 不防抖，无风暴风险，RESEARCH Pattern 4）；last 为当前 PTY 目标尺寸（变化
// 检测——目标尺寸不变则不发起 TIOCSWINSZ）。
type arbiter struct {
	sizes map[*client]dims
	timer *debouncer
	last  dims
}

// initArbiter 装配仲裁器（New 在 hubCond 构造后、goroutine 启动前调用——timer
// 回调与 ReadLoop 都可能在装配返回后立刻触达 arbiter 字段）。防抖经共用件
// newDebouncer 构造即 stopped 态：首次 reportResize 的 Reset 才武装（单
// time.Timer reset 防抖形态，server.go helloTimeout AfterFunc 先例；回调到期
// 取 hubMu 做 recalcNow）。Don't Hand-Roll 纪律：除本装配点外无任何 goroutine
// 计时循环。
func (s *Server) initArbiter() {
	s.arbiter = arbiter{sizes: make(map[*client]dims)}
	s.arbiter.timer = newDebouncer(s.resizeDebounce, func() {
		s.hubMu.Lock()
		defer s.hubMu.Unlock()
		s.recalcNow()
	})
}

// reportResize 是 RESIZE 上报入口（调用方 = server.go RESIZE case，已过 ro
// 忽略闸，hubMu 内调用）：c 在参与集 → 更新其最新尺寸 → timer.Reset 防抖
// （s.resizeDebounce，默认 50ms，Options.ResizeDebounce 测试可覆写——高频
// 上报合并为窗口末一次重算，防 SIGWINCH 风暴）；c 不在参与集 → 直接忽略
// （D-09 第二闸的兜底层——ro 旁观者与纯 ro 会话运行期 RESIZE 均落此分支；
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
// 不动 PTY 保持现状）且 ≠ last → sess.Resize 并更新 last + 向全部在线客户端
// 推送新会话尺寸（G-05-1，pushSessionDimsLocked）。
//
// 目标尺寸变化才调 sess.Resize（P5-3：Linux 同尺寸 TIOCSWINSZ 内核不发
// SIGWINCH，且避免无谓 ioctl）；推送挂点同此唯一变化检测点——目标尺寸不变
// （含零值哨兵提前返回）零推送（无放大无循环），attach/detach/kick/升格/防抖
// 五调用点自动全覆盖。参数序 (cols, rows)——io_test.go:24-25
// 注释锁定，切勿按 (rows, cols) 序误传；会话 closed 时 Resize 返回
// os.ErrClosed，忽略（Attach 读循环既有纪律同款）。
//
// attach 调用点的天然性质（05-10 Task 1 升档重排后）：recalcNow 执行时新客户端
// 尚未 registerLocked，推送循环遍历注册表不触达 attach 者自身——其会话尺寸由
// Welcome 承载（sessionDimsLocked 取值组帧），零重复帧。
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
	s.pushSessionDimsLocked(target)
}

// pushSessionDimsLocked 向全部在线客户端推送新会话尺寸（G-05-1 运行期尺寸下发
// 通道；hubMu 内调用，唯一挂点 = recalcNow 的 last 变化分支）：遍历注册表，每
// 客户端按其当前生效 mode 组帧（c.mode.Load().(string)——atomic.Value 恒存
// proto.ModeRO/ModeRW 字符串，allWritableBlockedLocked 同款读法先例）；prefs 按
// mode 选档（rw → clientPrefsRW / ro → clientPrefsRO——D-13 双档纪律在推送通道
// 不漂移：ro 端推送永不含 osc52）。trySend 失败走 kickOrCreditLocked（R-08 分工
// 复用：连 ~100B 推送都容不下的端 = 事实 stalled；首触发帧暂存 creditPending
// 不丢——已 blocked 期间的后续尺寸推送帧不再暂存，由 afterDrain 开门时按当前
// sessionDimsLocked() 补发收敛，WR-02 见 clients.go afterDrain）。
//
// range 内踢出安全性：removeLocked 的 map delete 在 range 期间为 Go spec 安全
// （未到达的被删条目不再产出），onChunk → kickOrCreditLocked（clients.go:354-358）
// 同形态先例。循环内踢出经 clients.go:501-502 removeMember → 嵌套 recalcNow
// 真实可达——被踢者是参与集成员且其持有某轴最小值时仲裁结果改变（纯 ro 会话
// 全部 ro 端均为成员、all 模式被踢 rw 端亦为成员）：嵌套 recalcNow 把
// arbiter.last 推进到 T2，嵌套推送把 W(T2) 送达全部留存端，外层捕获的 target
// （T1）已 stale——kickOrCreditLocked 返回后的 arbiter.last != target 复检即
// 防旧值反超的防线（嵌套推送已送达更新值，stale 外层扇出直接中止）。踢出不改
// 仲裁（嵌套 recalcNow 因 target==last 提前返回、last 不变）或走信用路径（无
// 成员变动）时 last==target，外层循环正确继续。每次踢出永久移除一端保证嵌套
// 有界终止（≤ max-clients）。promoteNextLocked 自推送循环内不可达（owner 模式
// 唯一可写端恒走信用、all 模式 owner 恒 nil），不纳入主论证。
func (s *Server) pushSessionDimsLocked(target dims) {
	for c := range s.registry.set {
		mode := c.mode.Load().(string)
		prefs := s.clientPrefsRO
		if mode == proto.ModeRW {
			prefs = s.clientPrefsRW
		}
		frame := proto.WelcomeFrame(mode, prefs, target.cols, target.rows, s.sessionMode)
		if !c.outbox.trySend(frame) {
			s.kickOrCreditLocked(c, frame)
			// 踢出可能经 removeMember→嵌套 recalcNow 把 arbiter.last 推进到更新值，
			// 嵌套推送已向全部留存客户端送达新值——本循环的 target 已过时，中止防旧值反超。
			if s.arbiter.last != target {
				return
			}
		}
	}
}

// sessionDimsLocked 返回当前会话尺寸（G-05-1，05-10；hubMu 内调用——读
// arbiter.last 与同锁字段一致）：会话尺寸恒等于 PTY 实际尺寸，两个分支同源论证——
//   - arbiter.last 非零（曾有参与者）：last 即最近一次 recalcNow 应用到 PTY 的
//     目标尺寸（重算后 min-rect/last-wins）；全部 detach 后 last 遗留 = PTY 保持
//     的最后一次应用尺寸（0 人零值哨兵不动 PTY），同源成立。
//   - arbiter.last 零值（从未有参与者）：不等于「会话尺寸未知」——首个参与端
//     attach 前 PTY 保持 spawn 尺寸（spawn.go StartWithSize 钉死 SpawnCols×
//     SpawnRows 80x24，且零参与者期间无任何 Resize 路径触达：recalcNow 零值
//     分支提前返回不发起 Resize），故回落 dims{pty.SpawnCols, pty.SpawnRows}
//     与 PTY 实际尺寸同源（spawn 常量单一事实源，防双写漂移）。
func (s *Server) sessionDimsLocked() dims {
	if s.arbiter.last == (dims{}) {
		return dims{cols: pty.SpawnCols, rows: pty.SpawnRows}
	}
	return s.arbiter.last
}

// 参与集判定（D-09 矩阵逐字）：升档生效 mode 为 rw（owner 模式仅 owner /
// all 模式全部 rw 端），或无 --writable 纯 ro 会话的全部 ro 端（Hello 首
// 尺寸参与，否则会话冻结 80x24）；含可写端会话的 ro 旁观者不参与（其
// RESIZE 经 D-09 第二闸忽略，尺寸永不影响可写端 PTY 尺寸）。
func (s *Server) participates(effMode string) bool {
	return effMode == proto.ModeRW || !s.writable
}
