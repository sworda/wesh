package pty

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestResize（VALIDATION 1-01-04，CORE-02）：Resize 经 pty.Setsize 触发 TIOCSWINSZ，
// 子进程 stty size（按 "rows cols" 序输出）须从初始 24 80 变为 resize 后 50 132
// （RESEARCH 行 571 本机原型实测 [24 80 50 132]）。
func TestResize(t *testing.T) {
	// /bin/sh -c 仅作测试编排夹具（产品 spawn 路径绝不经 shell，D-02/D-15 纪律不变）
	sess, err := Start([]string{"/bin/sh", "-c", "stty size; sleep 1; stty size"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	buf := startCollect(sess)

	// 首个 stty 输出后、sleep 1 窗口内触发 TIOCSWINSZ。
	// Resize(cols, rows)：cols=132、rows=50——切勿按 (rows, cols) 序误传，
	// 否则子进程报 "132 50" 必红。
	time.Sleep(150 * time.Millisecond)
	if err := sess.Resize(132, 50); err != nil {
		t.Fatalf("Resize: %v", err)
	}

	out, werr := awaitSession(t, sess, buf)
	if werr != nil {
		t.Fatalf("sh 退出异常: %v", werr)
	}
	// PTY ONLCR 将 stty 的 \n 译为 \r\n，按空白切分最稳
	got := strings.Fields(out)
	want := []string{"24", "80", "50", "132"}
	if !slices.Equal(got, want) {
		t.Fatalf("stty 序列 = %v（raw %q），want %v（24 80 → Resize(132,50) → 50 132）", got, out, want)
	}
}

// TestResizeAfterClose（02-02 fd 竞态修复的语义回归锁）：Close 后 Resize 返回
// os.ErrClosed，重复 Close 幂等返回 nil。并发面（Resize↔Close）由 -race 下的
// e2e 套件覆盖（02-02 握手 Resize 调用点暴露的原缺陷形态）。
func TestResizeAfterClose(t *testing.T) {
	sess, err := Start([]string{"/bin/cat"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := sess.Resize(80, 24); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Resize after Close = %v, want os.ErrClosed", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close = %v, want nil (idempotent)", err)
	}
	// 收割子进程防僵尸（master 已关，cat 收 SIGHUP 消亡；Kill 兜底，错误忽略）
	_ = sess.Cmd.Process.Kill()
	_ = sess.Wait()
}
