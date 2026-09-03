---
phase: 10-mode-assembly
fixed_at: 2026-09-03T16:49:32+08:00
review_path: /data1/home/zexueli/open_src/wesh/.planning/phases/10-mode-assembly/10-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 10: Code Review Fix Report

**Fixed at:** 2026-09-03T16:49:32+08:00
**Source review:** .planning/phases/10-mode-assembly/10-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3（fix_scope=critical_warning：CR-01 + WR-01 + WR-02；7 条 Info 出范围未处理）
- Fixed: 3
- Skipped: 0

## Fixed Issues

### CR-01: prescanConfigPath 不停于首个非 flag 参数——子命令 argv 位的 `--config` 被静默当作 wesh 配置加载

**Files modified:** `cmd/wesh/main.go`, `cmd/wesh/config_test.go`
**Commit:** b007c3a
**Applied fix:** 按 review 建议在 `fs.Parse` 之后插入交叉校验——预扫路径 `configPath` 必须被正式 Parse 确认为 `--config` flag 值（`configFileFlag`），两通道值不一致即 `invalid --config: must be given as a wesh flag before the command` 拒绝启动（exit 2）。Go flag 包在首个非 flag 参数处即停止解析而预扫只停于字面量 `--`，交叉校验以正式 Parse 为权威收敛整类边界分叉（`wesh true --config x.toml`、`wesh -- mytool --config prod.toml`、`wesh --credential --config x.toml` 三形态同闸）；last-wins 双给等一致形态两通道同值自然放行。补两个回归子测：`config flag after positional arg rejected`（argv 位 --config 拒绝）与 `config last-wins double-given passes cross-check`（一致形态放行 + 后者铺底）。进程级实证复现 review 场景：`true --config probe.toml` → exit 2 拒绝；`--config probe.toml -- true` → 配置正常加载；`-- true --config probe.toml` → 子命令 --config 不加载（行为不变）。

### WR-01: D-07 权限警告对 `credential = []` 空数组误报「含凭据」

**Files modified:** `cmd/wesh/config.go`, `cmd/wesh/config_test.go`
**Commit:** 92e2111
**Applied fix:** 按 review 建议把 D-07 判定从 `decoded.Credential != nil` 改为 `len(decoded.Credential) > 0`——go-toml 把显式空数组解码为非 nil 零长切片，与键缺席同档按「无凭据」处理（合并层零迭代同语义），0644 权限不再误报。同步更新 loadFileConfig 文档注释（`fc.Credential != nil` → `len(fc.Credential) > 0`）。按 review 要求补回归子测 `D-07 silent on empty credential array`（`credential = []` + 0644 → warn 为空）；既有 `D-07 warns on group/world readable with credential`（非空数组仍警告）保持绿。

### WR-02: linux/386（含 32-bit arm）编译破口——`4294967295` 在 int32 上溢出

**Files modified:** `cmd/wesh/main.go`
**Commit:** 34279d1
**Applied fix:** 按 review 建议把 uid/gid 值域比较改为 `int64(cfg.uid) > math.MaxUint32` / `int64(cfg.gid) > math.MaxUint32`（int64 上转后比较，32-bit 平台 int=int32 不再撞无类型常量溢出），新增 `math` import，注释登记破口原因。错误文案保持 `0..4294967295` 字面量（字符串无编译问题）。实证：修复前 `GOOS=linux GOARCH=386 go build ./...` 复现 review 报告的两处 overflow 编译错误；修复后 linux/386、linux/arm(GOARM=7)、本机 native、darwin/arm64 四平台编译全部通过，`--uid 4294967296` 等值域拒绝用例行为不变（cmd/wesh 包全量测试绿）。

## Verification

**验证运行环境：** 隔离 worktree（`.codebuddy/worktrees/rf-10-*`，gsd-reviewfix/10-* 分支）——三提交经 `git merge --ff-only` 落回 main 后 worktree 已移除，main 树与验证时树逐字节一致（ff 无合并提交），结果可从 main 直接复现。

- `go build ./cmd/wesh/`（每个修复后）：通过
- `GOOS=linux GOARCH=386 go build ./...` / `GOOS=linux GOARCH=arm GOARM=7 go build ./...` / `GOOS=darwin GOARCH=arm64 go build ./...`（WR-02 后）：全部通过
- `go vet ./cmd/wesh/`：干净
- `go test ./cmd/wesh/ -count=1`：全绿（含 4 个新增回归子测；internal/pty、internal/server 未被触碰，未重跑其测试套件）
- 进程级实证（CR-01）：review 复现形态三向验证通过（拒绝/加载/不加载各归其位）

---

_Fixed: 2026-09-03T16:49:32+08:00_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
