//go:build linux

package pty

import "os/exec"

// Linux 收割 = 每会话一个 goroutine 阻塞在 cmd.Wait()。
// Go ≥1.23 的 os/exec 在 Linux 5.3+ 自动以 CLONE_PIDFD fork、以 waitid(P_PIDFD) 等待
// （GOROOT syscall/exec_linux.go:310-312、os/pidfd_linux.go），即"pidfd 收割"的正确实现：
// 零额外线程、无 PID 复用竞态、无僵尸、*exec.ExitError 带退出码。
// 禁止手写 pidfd_open / SIGCHLD+waitpid——会引入第二个收割者与 Wait 竞争，丢退出码
// （D-14，RESEARCH Anti-Patterns）。
func awaitExit(cmd *exec.Cmd) error {
	return cmd.Wait()
}
