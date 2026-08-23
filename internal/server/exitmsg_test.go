package server

// exitmsg_test.go 白盒锁定 EXIT message 组文案两件套（06-01，review 吸收——
// signalName 映射表独立 helper + 兜底分支白盒直测；resize_test.go 白盒先例，
// 同目录 server/server_test 双包并存约束下与黑盒 exit_test.go 分文件）。

import (
	"errors"
	"syscall"
	"testing"
)

// TestSignalName 逐行锁定信号大写名映射表（RESEARCH Pitfall 3：Signal.String()
// 产出 "hangup" 式小写描述词——GOROOT zerrors 表，禁止裸用；显式映射是唯一
// 合法形态，D-09 服务端组文案唯一写口）。表内信号返回约定大写名且 ok==true；
// 表外信号（SIGWINCH——窗口变化信号无终结文案场景，刻意不入表）ok==false，
// 调用方回退数字形态。
func TestSignalName(t *testing.T) {
	mapped := []struct {
		sig  syscall.Signal
		want string
	}{
		{syscall.SIGHUP, "SIGHUP"},
		{syscall.SIGINT, "SIGINT"},
		{syscall.SIGQUIT, "SIGQUIT"},
		{syscall.SIGILL, "SIGILL"},
		{syscall.SIGABRT, "SIGABRT"},
		{syscall.SIGKILL, "SIGKILL"},
		{syscall.SIGSEGV, "SIGSEGV"},
		{syscall.SIGPIPE, "SIGPIPE"},
		{syscall.SIGALRM, "SIGALRM"},
		{syscall.SIGTERM, "SIGTERM"},
		{syscall.SIGUSR1, "SIGUSR1"},
		{syscall.SIGUSR2, "SIGUSR2"},
		{syscall.SIGCHLD, "SIGCHLD"},
	}
	for _, tt := range mapped {
		got, ok := signalName(tt.sig)
		if !ok || got != tt.want {
			t.Errorf("signalName(%d) = %q,%v, want %q,true", int(tt.sig), got, ok, tt.want)
		}
	}
	if got, ok := signalName(syscall.SIGWINCH); ok {
		t.Errorf("signalName(SIGWINCH) = %q,true, want ok==false（表外信号回退数字形态）", got)
	}
}

// TestExitMessage 白盒锁定 exitMessage 的两条可构造分支：
//   - err==nil → 退出码 0 文案（正常退出含 exit 0）；
//   - 非 ExitError 错误（Wait 返回其他错误）→ 兜底文案。
//
// ExitError 两分支（正常退出码 / 信号死亡）由 TestExitFrameBroadcast /
// TestExitFrameSignal 真实进程路径覆盖——exec.ExitError 无公开构造器不可合成。
func TestExitMessage(t *testing.T) {
	if got := exitMessage(nil, 0); got != "The process exited with code 0." {
		t.Errorf("exitMessage(nil, 0) = %q, want %q", got, "The process exited with code 0.")
	}
	if got := exitMessage(errors.New("synthetic wait error"), 0); got != "The process terminated." {
		t.Errorf("exitMessage(非 ExitError, 0) = %q, want %q（兜底分支）", got, "The process terminated.")
	}
}
