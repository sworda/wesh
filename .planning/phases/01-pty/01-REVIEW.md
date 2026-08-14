---
phase: 01-pty
reviewed: 2026-08-14T02:51:57Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - .github/workflows/ci.yml
  - README.md
  - cmd/wesh/main_test.go
  - internal/pty/io_test.go
  - internal/pty/reap_darwin.go
  - internal/pty/reap_darwin_test.go
  - internal/pty/reap_test.go
  - internal/pty/spawn_test.go
  - internal/server/e2e_test.go
  - internal/server/server.go
findings:
  critical: 0
  warning: 3
  info: 5
  total: 8
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-08-14T02:51:57Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

审查范围覆盖 Phase 1 提交的全部 10 个变更文件（1 个 CI 工作流、1 个 README、2 个生产源文件、6 个测试文件）。为验证被审代码的行为断言，同时交叉阅读了调用链上的依赖文件（`cmd/wesh/main.go`、`internal/pty/spawn.go`、`internal/pty/io.go`、`internal/pty/reap_linux.go`、`internal/proto/proto.go`、`web/embed.go`）。

佐证：`go vet ./...` 干净；`go test -race -count=1 ./...` 在本机 Linux 全绿（2.5s）。darwin 专属路径（`reap_darwin.go` 及其测试）无法在本机执行，依赖 CI macos leg 裁决。

整体评估：代码纪律性好（env 白名单、关闭码纪律、exitf-once 收口、竞态注释准确），无 Critical 级问题。三项 Warning 集中在：darwin 收割路径的潜在死锁（Q1 裁决仍未落地，且裁决测试以 `t.Skip` 表达否定结论，CI 绿色 ≠ 代码正确）、WS 写路径无超时导致的整服挂死场景、以及一个计时敏感的 PTY 尺寸测试。

## Warnings

### WR-01: darwin `awaitExit` 串行依赖 kqueue 事件，watcher 失效即整服死锁（Q1 裁决未落地）

**File:** `internal/pty/reap_darwin.go:109-120`（关联 `:64-88`）
**Issue:** `awaitExit` 的执行序为 `<-exited` → `return cmd.Wait()`——Wait 的调用被串行排在 kqueue 事件到达之后。两条失效路径都会让 `<-exited` 永久阻塞，进而子进程永不收割、lifecycle 永不推进、wesh 永不退出：

1. **Q1 否定裁决路径**：`watch(pid)` 在 spawn 之后注册。若 kqueue 不为"注册时已是僵尸"的进程补发 NOTE_EXIT（这正是 TestKqueueExitZombieRace 要裁决的问题，且据 01-04-SUMMARY 运行时裁决**尚未发生**），任何快退出命令（`wesh -- /bin/true`）都会命中该窗口。关键在于：否定裁决以 `t.Skip` + 日志标记表达（`reap_darwin_test.go:112,119`），**CI 依然全绿**——兜底退化（直接 `cmd.Wait()`）依赖人工看到 skip 后改代码，在此之前缺陷随绿 CI 悄然存在。
2. **watcher 死亡路径**：`loop()` 在收到任何非 EINTR 的 Kevent 错误时静默 `return`（`:71-72`，注释自称"进程级致命即可"，但实际效果是挂死而非致命退出）。watcher 死后 `watch()` 的 EV_ADD 注册依然成功（kq fd 仍有效），但再无事件送达——同样的永久阻塞。

当前实现对 Phase 1 而言，`<-exited` 先行没有带来任何收益（"早知"结果未被使用），却引入了 Wait 永不执行的额外失效面。
**Fix:** 让 Wait 与 watcher 竞速而非串行，Wait 仍是唯一收割者，watcher 失效自动退化为普通 Wait：

```go
func awaitExit(cmd *exec.Cmd) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	w, err := sharedExitWatcher()
	if err != nil {
		return <-done
	}
	exited, err := w.watch(cmd.Process.Pid)
	if err != nil {
		return <-done
	}
	select {
	case <-exited:
		return <-done
	case err := <-done: // watcher 死亡/不补发时兜底，永不挂死
		return err
	}
}
```

同时建议将否定裁决的表达从 `t.Skip` 升级为显式失败（或在 CI 中对 `Q1-VERDICT` 标记做 grep 闸口），避免"skip 即绿"掩盖兜底未执行的状态。

### WR-02: `onChunk` 写 WS 无超时，静默停滞的客户端可挂死整个服务

**File:** `internal/server/server.go:128`
**Issue:** `c.Write(context.Background(), ...)` 无任何时限。若已 attach 客户端停止读取 TCP（连接存活但接收窗口归零），Write 永久阻塞 → ReadLoop 随之阻塞 → 64KiB PTY 内核缓冲写满 → 子进程写阻塞永不退出 → lifecycle 停在 `sess.Wait()`。D-11 收口路径不会触发，因为连接层面并未断开（`Attach` 的 `c.Read` 也正常阻塞）。结果是整服挂死且无任何外部可观测错误。Phase 1 可信网络定位降低了触发概率，但这是可用性正确性问题而非性能问题。
**Fix:** 写路径派生带时限的 context，超时按断连处理：

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := c.Write(ctx, websocket.MessageBinary, s.frame[:1+n]); err != nil {
	return
}
```

### WR-03: TestResize 依赖固定 150ms 时序假设，CI 高负载下存在片状风险

**File:** `internal/pty/io_test.go:24`
**Issue:** 测试正确性要求 `/bin/sh` 完成启动并执行第一个 `stty size` 的时间落在固定 `time.Sleep(150ms)` 之内；若 macOS leg 高负载导致 sh 启动+首次 stty 超过 150ms，首个采样将变成 resize 后的 `50 132`，断言序列 `[24 80 50 132]` 变红——典型的时序片状（flaky）测试，影响测试可靠性。
**Fix:** 用观测替代固定睡眠：让收集回调在收到首个 `"24\r\n80"`（或 `strings.Fields` 已满两项）时关闭一个 channel，测试阻塞在该 channel 上（带 5s 上限）再触发 `Resize`。注意 `startCollect` 的 buffer 不可并发读（`spawn_test.go:19-21` 已注明），需要换一个内部加锁的收集器来实现观测。

## Info

### IN-01: `s.frame` 缓冲尺寸与 ReadLoop 读缓冲隐式耦合，`copy` 静默截断

**File:** `internal/server/server.go:48`（耦合点 `internal/pty/io.go:16`）
**Issue:** `frame = make([]byte, 1+32*1024)` 的容量假设 ReadLoop 的 `buf` 恒为 `32*1024`；`n := copy(s.frame[1:], chunk)` 在 chunk 超限时会静默截断数据。当前两侧均为 32KiB 故无实际损失，属潜在耦合。
**Fix:** 在 pty 包导出 `ReadBufSize` 常量供 server 侧引用，或在 onChunk 内对 `len(chunk) > len(s.frame)-1` 的情况分帧发送。

### IN-02: README 对退出码语义的概括与 D-11 行为不符

**File:** `README.md:11`
**Issue:** "子进程退出或 WS 断开，服务端即整体退出（退出码 = 子进程退出码）"——括号语义覆盖了两条路径，但 WS 断开路径按 D-11 以 0 退出（`server.go:159` `terminate(true, 0)`），并非子进程退出码。
**Fix:** 改为分别陈述："子进程退出 → 退出码 = 子进程退出码；WS 断开 → 子进程收 SIGHUP，服务端以 0 退出"。

### IN-03: `captureFd` 夹具的两处潜在陷阱

**File:** `cmd/wesh/main_test.go:48-65`
**Issue:** (a) pipe 读端在 `f()` 返回后才 drain——若被测函数向捕获 fd 写入超过 pipe 容量（Linux 64KiB）将死锁（当前 usage/version 输出极小，属潜在风险）；(b) `f()` panic 时 `*target` 不恢复、`w` 不关闭，会污染同进程后续测试。
**Fix:** drain 放入独立 goroutine；用 `defer` 恢复 `*target` 并关闭 fd。

### IN-04: CI actions 钉版不一致且未声明最小权限

**File:** `.github/workflows/ci.yml:25`
**Issue:** `actions/setup-node@v4` 仅钉大版本，而同文件 checkout@v7.0.1 / setup-go@v7.0.0 / pnpm-action-setup@v6.0.10 均钉到 patch（01-04-SUMMARY 亦自述"actions 全钉版"，与此处不符）；另工作流未声明 `permissions:`，GITHUB_TOKEN 取仓库默认（可能过宽）。
**Fix:** setup-node 钉至具体 patch 版本；文件头加 `permissions: { contents: read }`。

### IN-05: echo 断言未覆盖"数据真正到达子进程"路径（缺失断言）

**File:** `internal/server/e2e_test.go:48-67, 226-243`
**Issue:** TestEchoPTY 与 TestSecondClient409 的 echo 校验发送的载荷均不含 `'\n'`。PTY 规范模式下行律（line discipline）会立即回显输入字节——测试收到的 OUTPUT 帧可完全来自行律回显，即使 master→slave→子进程这条链路断裂（cat 从未收到数据），测试依然全绿。属 missing-assertion 类测试可靠性问题。
**Fix:** 载荷追加 `'\n'`（如 `"hello wesh\n"`），使 cat 真实收到并回吐一行，断言收到的累积输出包含该行（注意此时行律回显 + cat 输出会出现两份，断言宜用 Contains 而非逐字节相等）。

---

_Reviewed: 2026-08-14T02:51:57Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
