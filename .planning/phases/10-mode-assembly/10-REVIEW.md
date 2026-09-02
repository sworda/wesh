---
phase: 10-mode-assembly
reviewed: 2026-09-02T15:49:06Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - README.md
  - cmd/wesh/config.go
  - cmd/wesh/config_test.go
  - cmd/wesh/fuzz_test.go
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - docs/CONFIGURATION.md
  - internal/pty/spawn.go
  - internal/pty/spawn_test.go
  - internal/server/clients.go
  - internal/server/options_test.go
  - internal/server/server.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-09-02T15:49:06Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

本轮为 phase 10（mode-assembly：--session-mode flag/TOML 键、Options.SessionMode/SpawnFunc 接缝、ValidateOptions 装配契约、run() 装配分岔、pty.StartWithSize 导出、validateStartup 的 write-policy×per-client warn 与 per-client LookPath 预检）的对抗性复审。评审基线 = 全部 12 个列入文件全量通读 + phase diff（f57c701^..HEAD，+503/-23）聚焦 + 交叉核实。

**复审再验证（2026-09-02T15:49Z 轮）**：评审范围 12 个文件相对 HEAD 逐文件 diff 为空（字节级不变），全部 7 条发现的行号锚点逐一重新核对（WR-01 → main.go:1043-1047、WR-02 → main.go:1342-1348、WR-03 → clients.go:883-894、IN-01 → spawn.go:100、IN-02 → config.go:174-176/main.go:509-511、IN-03 → clients.go:692、IN-04 → config.go:160-167）——结论原样成立，无新增/消亡发现。

已做实证（非仅纸面阅读）：

- `go build ./... && go vet` 干净；`go test ./cmd/wesh ./internal/pty` 全绿；`internal/server` 首轮三包并行跑出现一次 FAIL，随后 5 轮复跑（含同形态三包并行）全绿——一次性 flake 未定位到用例名，登记观察项，建议后续以 `-json` 落盘长跑盯防。
- creack/pty@v1.1.24 `start.go` 源码核实：`StartWithSize` 仅在 `cmd.SysProcAttr == nil` 时分配并只补 Setsid/Setctty 两字段——spawn.go 降权 Credential 不被覆盖的注释声明属实（该声明是 uid/gid 降权安全性的承重墙，必须核实而非信注释）。
- go-toml v2.4.3（go.mod 钉版）实证：`credential = []` 解码为非 nil 空切片（IN-04 的前提）。
- WR-01 进程级独立复现：per-client × `--cwd` × 相对 argv0 → exit 2 误拒；同参数 shared 正常 spawn（session_start 事件 + listening 打印实证）。
- 值剥离红线抽查：ParseCredential 错误文案不含值（auth.go 源码核实），config.go 错误三要素（类别+键名+行号）实现与 fuzz 探针断言面一致；docs/CONFIGURATION.md 的 30 键表、退出码表与 main.go/config.go 逐键对账一致（28 flag 同名 + command + index-max-size）。

前置评审（HEAD 提交的 10-REVIEW.md，0C/2W/1I）结论经独立验证成立并吸收为 WR-01/WR-02/IN-01（WR-01 本轮重新复现）。本轮新增：WR-03（exit-when-empty 宽限计时器陈旧回调竞态，可提前杀死子进程）、IN-02（prescan 与 flag.Parse 的解析一致性注释 overstated）、IN-03（mergeBatch 空帧守卫不对称的潜在 panic）、IN-04（D-07 权限警告对空 credential 数组误报）。WR-03/IN-02/IN-03/IN-04 均位于列入文件内的**前序 phase 代码**（非 phase 10 diff 引入），因文件在审且缺陷可证而登记，已逐条标注出处。

## Warnings

### WR-01: per-client LookPath 预检不感知 --cwd，相对路径 argv0 被误拒（本轮独立复现）

**File:** `cmd/wesh/main.go:1043-1047`
**Issue:** SC4 预检 `exec.LookPath(cfg.argv0)` 在**服务端进程 cwd** 下解析，而实际 spawn（`pty.StartWithSize` → `exec.Command` + `cmd.Dir = opts.Dir`）中，含路径分隔符的 argv0 在子进程 chdir(cfg.cwd) 之后 execve——相对 `--cwd` 解析。两者在 `--cwd` × 相对路径 argv0 组合下双向发散：

- 误拒（本轮复现）：`/tmp/wesh-review-verify/appdir/run.sh` 存在且可执行，从不含 run.sh 的 `/tmp` 启动——`wesh --session-mode=per-client --cwd=<appdir> --bind 127.0.0.1 -- ./run.sh` → **exit 2**，`invalid command "./run.sh": not found in PATH (per-client startup preflight)`；同参数 `--session-mode=shared` 正常 spawn（listening 打印 + session_start 事件实证）。
- 误放（同机制反向）：服务端 cwd 恰有 `./run.sh` 而 --cwd 目录没有时，预检放行、Phase 11 attach 期 spawn 才失败——预检存在意义（命令缺失不推迟为 attach 期故障）落空。
- 次要：PATH 含 `.`/空元素等相对项时 LookPath 返回相对路径，同一 cwd 错位同样发生；文案 `not found in PATH` 对带斜杠路径不准确（该形态不经 PATH 解析）。

即：shared 下完全合法的部署形态（`--cwd` + 相对命令，Phase 7 既有能力的合理用法）在 per-client 下被结构性误拒；Phase 11 spawn 移至 attach 期后该预检是唯一启动闸，误拒将永久固化。

**Fix:** 预检与 spawn 语义对齐——argv0 含 `/` 时改为对 `--cwd` 感知的可执行探测，不经 LookPath：

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

**File:** `cmd/wesh/main.go:1342-1348`（调用点）；对照 `internal/server/server.go:324-328`（ValidateOptions 注释自引「与 validateStartup『拒绝路径零资源占用』纪律同构」）
**Issue:** run() 中 ValidateOptions 在 `pty.Start`（已 spawn 子进程，main.go:1307）与 `net.Listen`/`listenSocket`（已占用监听，main.go:1316-1322）**之后**、New 之前调用，失败分支直接 `return 2`——既无 `sess.Close()` 也无 `ln.Close()`。与紧邻的失败路径不对称（listen 失败 1326 行与 serve 失败 1452 行均 `_ = sess.Close()` 回滚），并直接违反其注释自引的零资源占用纪律。守卫触发时的后果：子进程依赖进程退出时 master 关闭的 SIGHUP 被动收口（非主动回滚）；`--socket` 形态下 socket 文件残留磁盘（unlink 依赖 ln.Close()，os.Exit 不触发——下次启动经活性探测自愈，但残留事实成立）。

当前不可达（parse 枚举闸 + run() 分岔逻辑保证 SessionMode/SpawnFunc 恒一致），但该守卫的全部存在意义是拦截未来漂移——一旦触发，清理路径即错。

**Fix:** 将校验前移至 `pty.Start` 之前——两个输入（`cfg.sessionMode`、分岔产物 `spawnFunc`）在装配分岔块尾部（main.go:1304）即已完全确定：

```go
// 10-01 分岔块之后、pty.Start 之前：
if verr := server.ValidateOptions(server.Options{SessionMode: cfg.sessionMode, SpawnFunc: spawnFunc}); verr != nil {
    fmt.Fprintf(os.Stderr, "wesh: %v\n", verr)
    return 2
}
sess, err := pty.Start(...)
```

（opts 字面量其余字段与本校验无关，New 前的完整 opts 构造保持原位；或保留原位调用但补上 `sess.Close()`/`ln.Close()` 回滚——前移方案更简单且恢复零占用语义，推荐前者。）

### WR-03: exit-when-empty 宽限计时器陈旧回调竞态——attach/detach 翻转窗口内子进程被提前杀死（前序 phase 代码，06-02/08-review 区段）

**File:** `internal/server/clients.go:883-894`（回调复查缺纪元判定）；关联 `clients.go:877-879`（武装点）、`internal/server/server.go:1033`+`clients.go:920-927`（取消点）
**Issue:** grace>0 形态下，计时器回调只复查 `s.exiting || len(s.registry.set) != 0`，不校验自身是否已被新纪元取代。竞态序列（attach+detach 全周期落入回调触发到取锁之间的窗口）：

1. 纪元 1 致空 → 武装 T1（30s 宽限）；
2. T1 到期触发，回调 goroutine 已创建但被调度延迟/等 hubMu；
3. 客户端 B attach：`cancelExitEmptyTimerLocked` 对已触发计时器 Stop 返回 false（无效），置 nil，`exitEmptySignaled=false`；
4. B 随即 detach：`maybeExitWhenEmptyLocked` 正常走完全程——置位 signaled、武装新计时器 T2（重新计 30s）；
5. 陈旧 T1 回调取得 hubMu：复查成立（registry 空、未 exiting）→ `stopChildLocked()` **立即**发 stop-signal——纪元 2 的宽限被架空，子进程提前至多一个 grace 周期死亡。

窗口成立条件苛刻（回调触发后恰好完成一轮 attach+detach——macOS CI/GOMAXPROCS=1 单核调度延迟场景本项目已有同类先例，kickOrCreditLocked 注释演化记录即单核实证产物），但后果真实（grace 语义被静默破坏），-race 抓不到（无数据竞争，纯逻辑竞态），T2 稍后重复发信号幂等收口使现场更难排查。注释（clients.go:860-862）声称复查是「恰好一次的兜底」，但该复查不区分纪元，恰恰漏掉这个窗口。

**Fix:** 回调携带计时器身份，复查时比对（武装与回调均在 hubMu 内/需 hubMu，赋值与比较无窗口）：

```go
t := time.AfterFunc(s.exitWhenEmptyGrace, func() {
    s.hubMu.Lock()
    defer s.hubMu.Unlock()
    if s.exitEmptyTimer != t || s.exiting || len(s.registry.set) != 0 {
        return // 陈旧回调（已被新纪元取代/取消）不动作
    }
    logEvent(remote, websocket.StatusNormalClosure, "exit_when_empty", remoteUser)
    s.stopChildLocked()
})
s.exitEmptyTimer = t
```

## Info

### IN-01: StartWithSize 的 uint16 截断锐利边（契约已挂账，建议接缝处防御）

**File:** `internal/pty/spawn.go:100`
**Issue:** `pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}`——int→uint16 转换对越界值静默截断（`rows = -1` → 65535；`cols = 70000` → 4464）。注释已将 ClampDim [1,1000] 钳制显式划归 Phase 12 调用侧（Hello 登记路径 DecodeHello 已钳制），当前唯一调用方 Start 传常量 80/24，无现实风险。但该函数是本 phase 新导出的接缝，Phase 11 SpawnFunc 挂接后调用面扩大，契约仅靠注释承载。
**Fix:** 接缝处加防御性钳制（复用 ClampDim 语义）或对越界输入显式报错，使契约由代码而非注释承载；若坚持现状（零开销论点），建议在 Phase 11 挂接 PR 中复查一次调用链钳制证据。

### IN-02: prescanConfigPath「与正式 Parse 同值」注释 overstated——畸形 CLI 下两通道解析分叉（前序 phase 代码，07-06 区段）

**File:** `cmd/wesh/config.go:174-176`（注释声明「预扫与正式 Parse 的 --config 值一致」）；`cmd/wesh/main.go:509-511`（「预扫与正式 Parse 双通道同值」）
**Issue:** 预扫器只特判 `--config`，不知道其他 flag 是否消费值。畸形输入 `--credential --config x.toml`：预扫在位置 1 命中 `--config` 并加载 x.toml；而 flag.Parse 把 `--config` 字符串**作为 credential 的值**消费（flag 包对取值 flag 不检查下一参数的 `-` 前缀）——两通道对「哪个 token 是 --config」判定不同。后果轻微（该 CLI 本就畸形，credErr 使运行 exit 2 fail-fast，配置文件值剥离红线不受损），但「值一致」的不变量声明在畸形输入下不成立，且预扫会为一个 flag.Parse 从不视为 --config 的调用形去真实读盘。另：`--config=`（显式空值）两通道一致地静默按「未给配置」放行——显式给空路径多为笔误，fail-fast 更贴 D-01「显式」哲学，至少应登记语义。
**Fix:** 二选一：(a) 注释降级为「well-formed CLI 下与正式 Parse 同值；畸形 CLI 由后续 fail-fast 收口」并登记 `--config=` 空值语义；(b) 预扫器跳过已知取值 flag 的参数位（维护成本高，不推荐）。

### IN-03: mergeBatch 守卫不对称——空帧会使 `batch[i][0]` panic（前序 phase 代码，05-03 区段，潜在缺陷）

**File:** `internal/server/clients.go:692`
**Issue:** 内层合并条件 `len(batch[j]) > 0 && batch[j][0] == batch[i][0] && ...` 只守卫了 `batch[j]` 的取字节，`batch[i][0]` 无长度守卫——零长度帧进入 outbox 即 slice 越界 panic（writer goroutine 崩溃 → 该客户端写端静默死亡）。当前全部生产者（onChunk 组帧 `1+len(chunk)`、WelcomeFrame/ExitFrame 协议帧）恒 ≥1 字节，结构性不可达；但守卫不对称说明作者考虑过空帧，而守卫放错了元素。一行防御可消除该锐利边。
**Fix:** 循环头归一化为非空不变量，例如 `for i := 0; i < len(batch); { if len(batch[i]) == 0 { i++; continue } ...`，或在 trySend 入口断言帧非空（单一事实源更靠入口侧）。

### IN-04: D-07 权限警告对空 credential 数组误报（前序 phase 代码，07-06 区段；go-toml v2.4.3 实证）

**File:** `cmd/wesh/config.go:160-167`
**Issue:** 判定条件 `decoded.Credential != nil`——go-toml v2.4.3（go.mod 钉版，本轮沙盒实证）把 `credential = []` 解码为**非 nil 空切片**，于是空数组 + 0644 权限也打出「config file ... contains credentials and is readable by others」警告。文件实际不含任何凭据，警告为误报（狼来了效应稀释 D-07 信号）。应用侧语义不受影响（空数组按缺席处理，main.go:786 循环零迭代）。
**Fix:** 判定改为 `len(decoded.Credential) > 0`，与「实际含凭据」语义对齐。

---

_Reviewed: 2026-09-02T15:49:06Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
