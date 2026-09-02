---
phase: 10-mode-assembly
reviewed: 2026-09-02T13:42:19Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - cmd/wesh/main.go
  - cmd/wesh/config.go
  - cmd/wesh/main_test.go
  - cmd/wesh/config_test.go
  - cmd/wesh/fuzz_test.go
  - internal/server/server.go
  - internal/server/clients.go
  - internal/server/options_test.go
  - internal/pty/spawn.go
  - internal/pty/spawn_test.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-09-02T13:42:19Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

审查范围锚定 `git diff 781af48..HEAD`（10 个文件，+494/-19）：--session-mode flag/TOML 键/parse 枚举闸、Options.SessionMode/SpawnFunc 接缝与 ValidateOptions 互斥校验、run() 装配期分岔、pty.StartWithSize 导出、validateStartup 的 write-policy×per-client warn（mergeWarn 累积形态）与 per-client LookPath 预检，以及 TOML 三面证据与 fuzz 种子。

总体评估：装配质量高——枚举闸一闸双覆盖正确（CLI/TOML 经默认值替换机制落同一终值），mergeWarn 全部透出点逐一核对无遗漏（socket/loopback 早退与三既有 warn 点均正确拼接，`strings.Join(nil)` 零漂移成立），既有 warn 文案逐字未动（diff 实证），Start 委托与 StartWithSize 逐字节等价（80×24 单一事实源保持）。key invariants 全部尊重：inert 接缝（SpawnFunc 零调用方、sessionModeSet 只采集）与 append-only 纪律均不列为发现。

发现 2 个 Warning（1 个经进程级实证的误拒行为缺陷、1 个违反自身纪律的资源占用放置缺陷）与 1 个 Info，无 Critical。文案红线核查通过：新增错误/warn 文案只含枚举值、命令名、flag 名——均在既定豁免面内。

## Warnings

### WR-01: per-client LookPath 预检不感知 --cwd，相对路径 argv0 被误拒（进程级实证）

**File:** `cmd/wesh/main.go:1043-1047`
**Issue:** SC4 预检 `exec.LookPath(cfg.argv0)` 在**服务端进程 cwd** 下解析，而实际 spawn（`pty.StartWithSize` → `exec.Command` + `cmd.Dir = opts.Dir`）在**子进程 chdir 之后的 --cwd** 下解析相对路径。两者在 `--cwd` × 相对路径 argv0 组合下发散：

- 带斜杠形态（`-- ./run.sh`）：LookPath 直查 `findExecutable("./run.sh")`（相对服务端 cwd）；spawn 侧 execve 在 child chdir(cfg.cwd) 后执行，相对 --cwd 解析。
- PATH 含 `.`/空元素形态：LookPath 查服务端 cwd 的 `.`；spawn 侧解析的是 cfg.cwd 的 `.`。

实证（本机，2026-09-02）：`/tmp/wesh-review/appdir/run.sh`（可执行）存在，从**不含** run.sh 的目录启动——

- `wesh --session-mode=per-client --cwd=/tmp/wesh-review/appdir -- ./run.sh` → **exit 2**，`invalid command "./run.sh": not found in PATH (per-client startup preflight)`（误拒）；
- 同参数 `--session-mode=shared` → 正常 spawn（`session_start` 事件携 pid 落流、`listening on` 打印，实证运行成功）。

即：shared 模式下完全合法的部署形态（`--cwd` + 相对命令，Phase 7 既有能力的合理用法）在 per-client 下被启动期 fail-fast 结构性误拒。这正是预检意图防的反面——预检本应只拦「命令缺失」，却把「能跑的配置」拦在门外；且 Phase 11 spawn 移至 attach 期后该预检是唯一启动闸，误拒将永久固化。次要：文案 `not found in PATH` 对带斜杠路径不准确（该形态不经 PATH 解析）。

**Fix:** 预检与 spawn 语义对齐——argv0 含路径分隔符时改为对 `--cwd` 感知的可执行探测，不经 LookPath：

```go
if cfg.sessionMode == server.SessionModePerClient && cfg.argv0 != "" {
    probe := cfg.argv0
    if strings.ContainsRune(probe, '/') && cfg.cwd != "" && !filepath.IsAbs(probe) {
        probe = filepath.Join(cfg.cwd, probe) // 与 child chdir 后 execve 的解析对齐
    }
    if strings.ContainsRune(probe, '/') {
        if fi, serr := os.Stat(probe); serr != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
            return "", fmt.Errorf("invalid command %q: not executable (per-client startup preflight)", cfg.argv0)
        }
    } else if _, lerr := exec.LookPath(probe); lerr != nil {
        return "", fmt.Errorf("invalid command %q: not found in PATH (per-client startup preflight)", cfg.argv0)
    }
}
```

或最小修复：`--cwd` 非空且 argv0 带斜杠时跳过预检（退让给 spawn 期错误通道），并在注释登记该已知缝隙。两案均需补 `--cwd`×相对路径的 TestStartupMatrix 行（放行面）。

### WR-02: ValidateOptions 调用点在资源获取之后，失败路径零回滚（与自引纪律矛盾）

**File:** `cmd/wesh/main.go:1342-1348`（调用点）；对照 `server.go:324-328`（ValidateOptions 注释自引「与 validateStartup『拒绝路径零资源占用』纪律同构」）
**Issue:** run() 中 ValidateOptions 在 `pty.Start`（已 spawn 子进程）与 `net.Listen`/`listenSocket`（已占用监听）**之后**、New 之前调用，失败分支直接 `return 2`——既无 `sess.Close()` 也无 `ln.Close()`。与紧邻的两个失败路径不对称（listen 失败与 serve 失败均 `_ = sess.Close()` 回滚），并直接违反 ValidateOptions 注释自引的零资源占用纪律。具体后果（守卫触发时）：子进程依赖进程退出时 master 关闭的 SIGHUP 被动收口（非主动回滚）；`--socket` 形态下 socket 文件残留磁盘（listenSocket 的 unlink 依赖 ln.Close()，进程退出不触发——下次启动经活性探测自愈，但残留事实成立）。

当前不可达（parse 枚举闸 + 分岔逻辑保证 SessionMode/SpawnFunc 恒一致），但该守卫的全部存在意义是拦截未来漂移——一旦触发，清理路径即错。exit 2 通道形态本身也属配置校验语义，此处是程序不变量违反，零资源占用前提被打破后该形态不再自洽。

**Fix:** 将校验前移至 `pty.Start` 之前——两个输入（`cfg.sessionMode`、分岔产物 `spawnFunc`）在装配分岔块尾部即已完全确定：

```go
// 10-01 分岔块之后、pty.Start 之前：
if verr := server.ValidateOptions(server.Options{SessionMode: cfg.sessionMode, SpawnFunc: spawnFunc}); verr != nil {
    fmt.Fprintf(os.Stderr, "wesh: %v\n", verr)
    return 2
}
sess, err := pty.Start(...)
```

（opts 字面量其余字段与本校验无关，New 前的完整 opts 构造保持原位；或保留原位调用但补上 `sess.Close()`/socket unlink 回滚——前移方案更简单且恢复零占用语义，推荐前者。）

## Info

### IN-01: StartWithSize 的 uint16 截断锐利边（契约已挂账，建议接缝处防御）

**File:** `internal/pty/spawn.go:100`
**Issue:** `pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}`——int→uint16 转换对越界值静默截断（`rows = -1` → 65535；`cols = 70000` → 4464）。注释已将 ClampDim [1,1000] 钳制显式划归 Phase 12 调用侧（Hello 尺寸登记路径 DecodeHello 已钳制），当前唯一调用方 Start 传常量 80/24，无现实风险。但该函数是本 phase 新导出的接缝，Phase 11 SpawnFunc 挂接后调用面扩大，契约仅靠注释承载。

**Fix:** 接缝处加防御性钳制（复用 ClampDim 语义）或对越界输入显式报错，使契约由代码而非注释承载；若坚持现状（性能零开销论点），建议在 Phase 11 挂接 PR 中复查一次调用链钳制证据。

---

_Reviewed: 2026-09-02T13:42:19Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
