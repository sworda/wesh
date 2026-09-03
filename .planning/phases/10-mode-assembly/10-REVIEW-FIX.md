---
phase: 10-mode-assembly
fixed_at: 2026-09-02T16:16:39Z
review_path: /data1/home/zexueli/open_src/wesh/.planning/phases/10-mode-assembly/10-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 10: Code Review Fix Report

**Fixed at:** 2026-09-02T16:16:39Z
**Source review:** .planning/phases/10-mode-assembly/10-REVIEW.md
**Iteration:** 1
**Fix scope:** critical_warning（IN-01..IN-04 四条 Info 不在范围内，未处理）

**Summary:**
- Findings in scope: 3（WR-01 / WR-02 / WR-03）
- Fixed: 3
- Skipped: 0

**Verification ran in:** 隔离 worktree（`.codebuddy/worktrees/rf-10-14586-*`，分支 `gsd-reviewfix/10-14586`，由 main HEAD f57c701 之后的 764f07c 切出），清理尾已 ff 回 main——worktree 内的测试数字对应与本报告一致的树状态；main checkout 复跑需依赖各自环境（worktree 无 node_modules，Go 门不受影响）。

## Fixed Issues

### WR-01: per-client LookPath 预检不感知 --cwd，相对路径 argv0 被误拒

**Files modified:** `cmd/wesh/main.go`, `cmd/wesh/main_test.go`
**Commit:** 189d081
**Status:** fixed
**Applied fix:** 按评审推荐方案（全量对齐版）实施——argv0 含 `/` 时不经 PATH 解析：非绝对路径且 `--cwd` 非空时先 `filepath.Join(cfg.cwd, argv0)`（与 child chdir 后 execve 的解析对齐，`internal/pty/spawn.go` `cmd.Dir = opts.Dir` 语义已核实），随后 `os.Stat` 可执行探测（不存在/目录/无执行位同拒，文案改为 `not executable`——带斜杠路径本就不经 PATH）；不含 `/` 保持 `exec.LookPath` 原通道。补 `path/filepath` import。
**测试锁定（评审要求的放行面 TestStartupMatrix 行 + 拒绝面两反向锁）：**
- `per-client cwd-relative executable allowed (WR-01)`（cwd 下 `./run.sh` 可执行 → 放行）
- `per-client cwd-relative missing refused (WR-01)`（cwd 下缺失 → `not executable`）
- `per-client cwd-relative non-executable refused (WR-01)`（0644 无执行位 → `not executable`）

**验证：** `go build ./...` + `go vet ./cmd/wesh` 干净；`go test ./cmd/wesh -count=1` 全绿（含既有 SC4 四行零回归）；进程级实证复刻评审场景——从不含 run.sh 的 `/tmp` 启动 `wesh --session-mode=per-client --cwd=<appdir> -- ./run.sh`：修复前 exit 2 误拒，修复后正常 listening + session_start；反向 `-- ./no-such.sh` 与无 --cwd 的 `./run.sh` 均正确 exit 2 `not executable`。

### WR-02: ValidateOptions 调用点在资源获取之后，失败路径零回滚

**Files modified:** `cmd/wesh/main.go`
**Commit:** 0ec37cb
**Status:** fixed
**Applied fix:** 按评审推荐方案（前移版）实施——`ValidateOptions(server.Options{SessionMode: cfg.sessionMode, SpawnFunc: spawnFunc})` 前移至 10-01 装配分岔块尾部与 `pty.Start` 之间（两输入在该点已完全确定；已核实 `internal/server/server.go:336-348` 校验只读这两字段，最小字面量与完整 opts 语义等价）。守卫触发时恢复零资源占用语义（spawn/listen 均未发生，无 sess/ln 需回滚），失败仍走 exit 2 通道。原调用点（完整 opts 构造之后、`New` 之前）移除，opts 注释同步更新。
**验证：** `go build ./...` + `go vet ./cmd/wesh` 干净；`go test ./cmd/wesh ./internal/server -count=1` 全绿；`internal/server/options_test.go` 三态契约测试不受影响（ValidateOptions 函数本体未动）。

### WR-03: exit-when-empty 宽限计时器陈旧回调竞态——attach/detach 翻转窗口内子进程被提前杀死

**Files modified:** `internal/server/clients.go`
**Commit:** 23f2df2
**Status:** fixed: requires human verification（纯逻辑竞态修复——`-race` 抓不到该窗口，e2e 测试无法确定性触发；修复正确性依赖代码推理，建议人工复核回调身份比对逻辑）
**Applied fix:** 回调携带计时器身份，复查时比对——武装改为 `var t *time.Timer; t = time.AfterFunc(...); s.exitEmptyTimer = t`，回调复查条件升级为 `s.exitEmptyTimer != t || s.exiting || len(s.registry.set) != 0`：已触发但被调度延迟的陈旧回调在取消点置 nil/新纪元重武装后身份不等，直接返回不动作，新纪元宽限不被旧回调架空。
**对评审片段的适配（非盲贴）：** 评审代码块 `t := time.AfterFunc(...)` 存在潜在编译缺陷——`:=` 短声明作用域起始于声明结束之后，闭包内自引用 `t` 报 `undefined: t`（首轮 `go vet` 实证）；改为 `var` 预声明 + 赋值（自引用计时器标准形态）。无窗口性论证成立：回调首句取 hubMu（武装方持锁中），取锁成功时赋值必然已完成且可见（锁同步边），已在注释中写明。函数头 grace>0 段注释同步更新纪元比对语义。
**验证：** `go build ./...` + `go vet ./internal/server` 干净；`go test ./internal/server -count=1` 全绿（53.3s）；`go test -race -run TestExitWhenEmpty -count=1 ./internal/server` 全绿——既有 9 个 exit-when-empty 测试（立即/宽限取消/宽限到期/kick 触发/lifecycle 门/计时器晚于 lifecycle/promote-kick 恰好一次/自定义 stop-signal/stop-timeout SIGKILL）零回归，确认正常到期路径（身份相等 → 动作）与取消路径（身份不等 → 静默）行为不变。

## Out of Scope（Info，fix_scope=critical_warning 未处理）

- IN-01: StartWithSize uint16 截断锐利边（契约挂账，建议 Phase 11 挂接 PR 复查调用链钳制证据）
- IN-02: prescanConfigPath「与正式 Parse 同值」注释 overstated（建议注释降级方案 a）
- IN-03: mergeBatch 空帧守卫不对称（结构性不可达，一行防御可消除锐利边）
- IN-04: D-07 权限警告对空 credential 数组误报（`len(decoded.Credential) > 0` 一行修复）

---

_Fixed: 2026-09-02T16:16:39Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
