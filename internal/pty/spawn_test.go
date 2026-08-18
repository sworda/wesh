package pty

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// testGuard 是全部用例的统一超时护栏：任何 Wait/drain 挂死必须在 10s 内翻车，
// 而不是拖死整个测试进程（T-02-03）。
const testGuard = 10 * time.Second

// startCollect 启动 ReadLoop 收集输出。返回的 buffer 只允许在 awaitSession 返回后
// 读取——readDone 的 close 与 <-readDone 建立 happens-before（-race 安全），
// 收集期间读 buffer 是数据竞争。
func startCollect(sess *Session) *bytes.Buffer {
	var buf bytes.Buffer
	go sess.ReadLoop(func(b []byte) { buf.Write(b) })
	return &buf
}

// awaitSession 收割子进程、等 ReadLoop 收尾（Linux EIO / darwin EOF 统一归一）、
// 关闭 master；带 10s 护栏。返回收集到的全部输出与 Wait 错误（断言交给调用方）。
func awaitSession(t *testing.T, sess *Session, buf *bytes.Buffer) (string, error) {
	t.Helper()
	waitCh := make(chan error, 1)
	go func() { waitCh <- sess.Wait() }()
	var werr error
	select {
	case werr = <-waitCh:
	case <-time.After(testGuard):
		t.Fatal("Wait 超时（10s 护栏）——子进程未退出")
	}
	select {
	case <-sess.readDone:
	case <-time.After(testGuard):
		t.Fatal("ReadLoop 未在子进程退出后收尾（10s 护栏）")
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("Close master: %v", err)
	}
	return buf.String(), werr
}

// TestExecArrayNoShell（VALIDATION 1-01-01，CORE-01，D-02/D-15）：argv 以 exec 数组
// 原样传递——`$(id)` 若经 shell 展开会变成 uid/gid 串（测试即红），字面量输出才合格。
func TestExecArrayNoShell(t *testing.T) {
	sess, err := Start([]string{"/bin/echo", "-n", "$(id)"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	out, werr := awaitSession(t, sess, startCollect(sess))
	if werr != nil {
		t.Fatalf("echo 退出异常: %v", werr)
	}
	// echo -n 无换行，PTY ONLCR 不介入，输出须恰为字面量
	if out != "$(id)" {
		t.Fatalf("输出 = %q，want 字面量 %q——argv 疑似经 shell 展开", out, "$(id)")
	}
}

// TestEnvWhitelist（VALIDATION 1-01-02，SEC-06）：宿主注入 AWS_SECRET_ACCESS_KEY 后
// 双层断言——(a) 白名单构造函数输出不含该键；(b) 子进程实际 env 输出不含该键
// （ttyd pty.c:441-444 全继承缺陷的对照回归）。
// 03-04 追加 WESH_CREDENTIAL 针对断言：替换式注入天然剥离该键，此断言防未来
// 有人改累加式注入把凭据透传进 Web shell 子进程（SEC-06 回归锁，T-03-22）。
func TestEnvWhitelist(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-leak-value")
	t.Setenv("WESH_CREDENTIAL", "test-cred-leak-value")

	// (a) 单元层：白名单构造函数
	env := whitelistEnv()
	for _, kv := range env {
		if strings.Contains(kv, "AWS_SECRET_ACCESS_KEY") {
			t.Fatalf("whitelistEnv 泄露宿主注入键: %q", kv)
		}
		if strings.Contains(kv, "WESH_CREDENTIAL") {
			t.Fatalf("whitelistEnv 泄露宿主注入键: %q", kv)
		}
	}
	if !slices.Contains(env, "TERM=xterm-256color") {
		t.Fatalf("白名单缺固定 TERM=xterm-256color: %v", env)
	}
	if !slices.Contains(env, "COLORTERM=truecolor") {
		t.Fatalf("白名单缺固定 COLORTERM=truecolor: %v", env)
	}

	// (b) e2e 层：子进程真实 env 输出
	sess, err := Start([]string{"/usr/bin/env"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	out, werr := awaitSession(t, sess, startCollect(sess))
	if werr != nil {
		t.Fatalf("env 退出异常: %v", werr)
	}
	if strings.Contains(out, "AWS_SECRET_ACCESS_KEY") {
		t.Fatal("子进程 env 输出泄露宿主注入键 AWS_SECRET_ACCESS_KEY")
	}
	if strings.Contains(out, "WESH_CREDENTIAL") {
		t.Fatal("子进程 env 输出泄露宿主注入键 WESH_CREDENTIAL")
	}
	// 阳性对照：证明确实捕获到了子进程 env 输出，而非空串假绿
	if !strings.Contains(out, "TERM=xterm-256color") {
		t.Fatalf("子进程 env 输出缺 TERM=xterm-256color（输出捕获无效？）: %q", out)
	}
}

// TestEnvWhitelistEmptyPathFallback（SEC-06 边界）：PATH 存在但为空串时须回退默认
// PATH，且 env 中不得出现重复 PATH 项——getenv 取首个匹配，"PATH="（空）在前会让
// 回退项失效，子进程 shell 将按空 PATH 找不到命令。
func TestEnvWhitelistEmptyPathFallback(t *testing.T) {
	t.Setenv("PATH", "")
	var paths []string
	for _, kv := range whitelistEnv() {
		if strings.HasPrefix(kv, "PATH=") {
			paths = append(paths, kv)
		}
	}
	if len(paths) != 1 || paths[0] != "PATH=/usr/local/bin:/usr/bin:/bin" {
		t.Fatalf("PATH 空值回退异常（须恰好一项默认 PATH）: %v", paths)
	}
}

// TestSpawnFailKeepsStdio（VALIDATION 1-01-03，Pitfall 1）：spawn 不存在的二进制必须
// 返回错误，且服务端自身 fd 0/1/2 保持有效——ttyd pty.c:87,112 close(0) 缺陷回归。
func TestSpawnFailKeepsStdio(t *testing.T) {
	sess, err := Start([]string{"/nonexistent/wesh-definitely-missing-binary"})
	if err == nil {
		if sess != nil {
			sess.Close()
		}
		t.Fatal("spawn 不存在的二进制应返回错误")
	}
	for fd := 0; fd <= 2; fd++ {
		// syscall 探测而非 os.NewFile——后者带 finalizer，GC 可能误关真实 fd。
		// EINVAL（/dev/null、终端）等任何非 EBADF 错误都证明 fd 仍有效。
		if ferr := syscall.Fsync(fd); errors.Is(ferr, syscall.EBADF) {
			t.Fatalf("fd %d 在 spawn 失败后被关闭（EBADF）——ttyd close(0) 缺陷重现", fd)
		}
	}
	// 标记日志：向真实 stdout 写一行，实证测试进程 stdio 完好
	fmt.Fprintln(os.Stdout, "spawn-fail probe: fd 0/1/2 alive, stdio intact")
	t.Log("spawn 失败后 fd 0/1/2 探测均非 EBADF，stdio 完好")
}
