//go:build linux

package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestReap（VALIDATION 1-01-05，成功准则 3）：短命令退出经 Wait 收割后 /proc/<pid>
// 必须消失——无僵尸残留；20 次高频建销循环佐证不累积（T-02-03，RESEARCH 行 570
// "reaped: OK"）。darwin 等效语义由 plan 01-04 的 TestKqueue 系列覆盖。
func TestReap(t *testing.T) {
	for i := 0; i < 20; i++ {
		sess, err := Start([]string{"/bin/true"}, StartOptions{Uid: -1, Gid: -1})
		if err != nil {
			t.Fatalf("iter %d: Start: %v", i, err)
		}
		pid := sess.Cmd.Process.Pid

		// /bin/true 正常退出：Wait 为 nil，或 *exec.ExitError 且退出码 0
		werr := sess.Wait()
		if werr != nil {
			var exitErr *exec.ExitError
			if !errors.As(werr, &exitErr) || exitErr.ExitCode() != 0 {
				t.Fatalf("iter %d: Wait = %v，want nil 或退出码 0", i, werr)
			}
		}

		if _, serr := os.Stat(fmt.Sprintf("/proc/%d", pid)); !errors.Is(serr, os.ErrNotExist) {
			t.Fatalf("iter %d: /proc/%d 在 Wait 后仍存在（stat err = %v）——僵尸残留", i, pid, serr)
		}
		if err := sess.Close(); err != nil {
			t.Fatalf("iter %d: Close: %v", i, err)
		}
	}
}
