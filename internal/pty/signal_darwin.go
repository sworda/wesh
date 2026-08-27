//go:build darwin

package pty

import "syscall"

// SignalGroup 向子进程进程组发指定信号（07-04 泛化自 06-02 的 HUP 专用形态——
// SESS-01/02 断开退出统一收口触发源，D-13；D-22 stop-signal 可配）：负 pid =
// 进程组；setsid（creack/pty StartWithSize 内建，spawn.go 调用链）使子进程
// 为组长，pgid == 子进程 pid。Start 成功后 Cmd.Process 必非 nil。错误全部静默
// （ESRCH 幂等——已死进程组重复发送无害，RESEARCH Pitfall 4；stop-timeout 后
// KILL 补发到达空 pgid 同款静默，Pitfall 8）。不触 Master fd 故不取 fdMu
// （与 SignalForegroundGroup 的 TIOCGPGRP 路径不同——io.go 对照），调用方可
// 在 hubMu 内调用（锁序 hubMu > sess.fdMu 不受影响）。
// 与 signal_linux.go 签名统一——调用点零平台分支（reap_darwin.go:105 同款
// 纪律注释逐字沿用）。
func (s *Session) SignalGroup(sig syscall.Signal) {
	_ = syscall.Kill(-s.Cmd.Process.Pid, sig)
}

// StopSignalByName 是 --stop-signal 的名→信号映射（07-04，OPS-04，D-22）：
// HUP/TERM/INT/KILL 四枚举命中返回 (信号, true)；其他（小写/未知名/空串/
// SIG 前缀全名）一律 false——parse 期枚举校验的唯一事实源（cmd/wesh 消费）。
// 与 server.go signalName 方向相反（那是 signal→name 的日志展示表）——名→
// 信号反向表不复用错方向。与 signal_linux.go 同签名同表（平台
// 对件纪律逐字沿用）。
func StopSignalByName(name string) (syscall.Signal, bool) {
	switch name {
	case "HUP":
		return syscall.SIGHUP, true
	case "TERM":
		return syscall.SIGTERM, true
	case "INT":
		return syscall.SIGINT, true
	case "KILL":
		return syscall.SIGKILL, true
	}
	return 0, false
}
