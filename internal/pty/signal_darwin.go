//go:build darwin

package pty

import "syscall"

// SignalHangup 向子进程进程组发 SIGHUP（06-02，SESS-01/02 断开退出统一收口
// 触发源，D-13；P1 git 历史 cc03c79~1 逐字形态复活）：负 pid = 进程组；
// setsid（creack/pty StartWithSize 内建，spawn.go 调用链）使子进程为组长，
// pgid == 子进程 pid。Start 成功后 Cmd.Process 必非 nil。错误全部静默
// （ESRCH 幂等——已死进程组重复发送无害，RESEARCH Pitfall 4）。不触 Master
// fd 故不取 fdMu（与 SignalForegroundGroup 的 TIOCGPGRP 路径不同——io.go
// 对照），调用方可在 hubMu 内调用（锁序 hubMu > sess.fdMu 不受影响）。
// 与 signal_linux.go 签名统一——调用点零平台分支（reap_darwin.go:105 同款
// 纪律注释逐字沿用）。
func (s *Session) SignalHangup() {
	_ = syscall.Kill(-s.Cmd.Process.Pid, syscall.SIGHUP)
}
