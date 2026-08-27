package pty

// signal_test.go 锁定 07-04（OPS-04，D-22）进程组信号泛化面：SignalGroup(sig)
// 任意信号送达与 ESRCH 幂等；StopSignalByName 名→信号映射四枚举与拒绝面
//（--stop-signal parse 期枚举校验的唯一事实源，两平台文件同签名同表）。

import (
	"errors"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestStopSignalByName（07-04，D-22）：HUP/TERM/INT/KILL 四枚举命中；小写、未知
// 名、空串、SIG 前缀全名均 false（枚举是 CLI 公开契约——值面严格大写简名）。
func TestStopSignalByName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want syscall.Signal
		ok   bool
	}{
		{"HUP", "HUP", syscall.SIGHUP, true},
		{"TERM", "TERM", syscall.SIGTERM, true},
		{"INT", "INT", syscall.SIGINT, true},
		{"KILL", "KILL", syscall.SIGKILL, true},
		{"lowercase rejected", "term", 0, false},
		{"unknown rejected", "USR1", 0, false},
		{"empty rejected", "", 0, false},
		{"sig-prefixed rejected", "SIGTERM", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := StopSignalByName(tt.in)
			if ok != tt.ok || (tt.ok && got != tt.want) {
				t.Errorf("StopSignalByName(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// TestSignalGroup（07-04，D-22）：SignalGroup(sig) 向子进程进程组（负 pid，
// setsid 使 pgid == 子进程 pid 既定不变量）发指定信号——SIGTERM 终结 sleep
// 子进程，sess.Wait 返回 *exec.ExitError，WaitStatus 断言 Signaled()==true 且
// Signal()==syscall.SIGTERM（任意信号送达的精确证据，io_test.go
// TestSignalGroupHangup 同款断言形态）；已死进程组重复发送静默（ESRCH 幂等——
// 不 panic 不阻塞即证，signal_linux.go 注释纪律）。
func TestSignalGroup(t *testing.T) {
	sess, err := Start([]string{"sleep", "600"}, StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	sess.SignalGroup(syscall.SIGTERM)
	waitCh := make(chan error, 1)
	go func() { waitCh <- sess.Wait() }()
	select {
	case werr := <-waitCh:
		var ee *exec.ExitError
		if !errors.As(werr, &ee) {
			t.Fatalf("Wait err = %v, want *exec.ExitError（SIGTERM 致死）", werr)
		}
		ws, ok := ee.Sys().(syscall.WaitStatus)
		if !ok {
			t.Fatalf("ExitError.Sys() = %T, want syscall.WaitStatus", ee.Sys())
		}
		if !ws.Signaled() {
			t.Fatal("WaitStatus.Signaled() = false, want true（信号致死）")
		}
		if ws.Signal() != syscall.SIGTERM {
			t.Fatalf("WaitStatus.Signal() = %v, want SIGTERM（SignalGroup(TERM) 送达语义）", ws.Signal())
		}
	case <-time.After(testGuard):
		t.Fatal("SignalGroup(SIGTERM) 后子进程未在 10s 内死亡（10s 护栏）")
	}
	// ESRCH 幂等：已死进程组重复发送静默（不 panic 即证）。
	sess.SignalGroup(syscall.SIGKILL)
	if err := sess.Close(); err != nil {
		t.Fatalf("Close master: %v", err)
	}
}
