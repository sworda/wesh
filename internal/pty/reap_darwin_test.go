//go:build darwin

package pty

// Q1 竞态裁决双测试（RESEARCH Open Questions Q1，CI-only，macos-latest leg 运行；
// 本机 Linux 由 build tag 排除）。
//
// 守卫判定走 argv 标记，不走 env 变量（钉死，理由）：spawn 路径
// cmd.Env = whitelistEnv() 为替换式注入，GO_WANT_HELPER_PROCESS 类自定义守卫
// 变量不在 SEC-06 白名单（仅 TERM/COLORTERM 固定 + PATH/HOME/USER/LOGNAME/SHELL
// 按名继承 + LANG=/LC_ 前缀继承）内会被剥离——helper 收不到 env 守卫将直接
// return、以退出码 0 收场；argv 经 spawn 原样透传（D-02）必然到达。
//
// helper argv 构造：[os.Executable(), "-test.run=TestHelperProcess", "--", <标记>]

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess 是子进程 helper 的唯一入口：os.Args 不含 "wesh-helper-"
// 前缀标记即直接 return；含则按 "--" 后标记分派。
func TestHelperProcess(t *testing.T) {
	idx := -1
	for i, a := range os.Args {
		if a == "--" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+1 >= len(os.Args) || !strings.HasPrefix(os.Args[idx+1], "wesh-helper-") {
		return // 非 helper 调用，直接 return（退出码 0）
	}
	switch os.Args[idx+1] {
	case "wesh-helper-exit42-delay":
		time.Sleep(100 * time.Millisecond)
		os.Exit(42)
	case "wesh-helper-exit0":
		os.Exit(0)
	}
}

// spawnHelper 以 argv 标记 spawn 一个 helper 子进程（不经 shell）。env 走与生产
// 一致的 whitelistEnv 替换式注入（07-04 签名选项化：零值等价形态 Term 空/Uid -1），
// 顺带证明 argv 守卫在白名单注入下必然到达。
func spawnHelper(t *testing.T, marker string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	cmd := exec.Command(exe, "-test.run=TestHelperProcess", "--", marker)
	cmd.Env = whitelistEnv("", -1)
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn helper %s: %v", marker, err)
	}
	return cmd
}

// TestKqueueExitNormal（正常路径）：spawn helper（sleep 0.1s 后 exit 42）→ spawn
// 后立即 watch(pid) → 等事件（5s 超时）→ 断言事件到达；随后 cmd.Wait() 断言
// *exec.ExitError.ExitCode() == 42。
func TestKqueueExitNormal(t *testing.T) {
	w, err := newExitWatcher()
	if err != nil {
		t.Fatalf("newExitWatcher: %v", err)
	}
	cmd := spawnHelper(t, "wesh-helper-exit42-delay")
	exited, err := w.watch(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Wait()
		t.Fatalf("watch(%d): %v", cmd.Process.Pid, err)
	}
	select {
	case <-exited:
		// 事件到达，正常路径成立
	case <-time.After(5 * time.Second):
		_ = cmd.Wait()
		t.Fatal("kqueue NOTE_EXIT 事件 5s 超时未到达（正常路径）")
	}
	err = cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Wait 返回非 *exec.ExitError: %v", err)
	}
	if got := exitErr.ExitCode(); got != 42 {
		t.Fatalf("退出码 = %d，期望 42", got)
	}
}

// TestKqueueExitZombieRace（竞态路径，Q1 核心裁决）：spawn 瞬时退出的 helper →
// 先 sleep 200ms 确保已退出成僵尸，再注册 watch → kqueue 带 1s 超时等事件：
// 事件到达 → 共享 watcher 无竞态成立，测试通过；超时未到达 → 不 fail，t.Skip
// 并打印裁决标记——executor 见到 skip 即执行兜底：reap_darwin.go 的 awaitExit
// 退化为直接 cmd.Wait()，watcher 代码以 build tag 保留待 Phase 5 重估。
// 两条出路都是计划内路径。
func TestKqueueExitZombieRace(t *testing.T) {
	w, err := newExitWatcher()
	if err != nil {
		t.Fatalf("newExitWatcher: %v", err)
	}
	cmd := spawnHelper(t, "wesh-helper-exit0")
	// 竞态核心：先 sleep 200ms 确保子进程已退出成僵尸，再注册 watch
	time.Sleep(200 * time.Millisecond)
	exited, err := w.watch(cmd.Process.Pid)
	if err != nil {
		// 僵尸注册失败（如 ESRCH）等同"不补发"裁决
		_ = cmd.Wait()
		t.Skipf("Q1-VERDICT: 僵尸进程 kqueue 注册失败（%v）——裁决为不补发，awaitExit 退化为直接 cmd.Wait()", err)
	}
	select {
	case <-exited:
		// 事件到达——共享 watcher 无竞态成立
	case <-time.After(1 * time.Second):
		_ = cmd.Wait()
		t.Skip("Q1-VERDICT: kqueue 未补发僵尸进程 NOTE_EXIT（1s 超时）——兜底：awaitExit 退化为直接 cmd.Wait()，watcher 代码以 build tag 保留待 Phase 5 重估")
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Wait（exit0 helper 应成功）: %v", err)
	}
}
