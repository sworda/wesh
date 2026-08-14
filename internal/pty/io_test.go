package pty

import (
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
