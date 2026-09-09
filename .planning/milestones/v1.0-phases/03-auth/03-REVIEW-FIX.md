---
phase: 03-auth
fixed_at: 2026-08-17T14:18:00Z
review_path: .planning/phases/03-auth/03-REVIEW.md
fix_scope: critical_warning
findings_in_scope: 3
fixed: 3
skipped: 0
iteration: 1
status: all_fixed
---

# Phase 3: Code Review Fix Report

**Fixed at:** 2026-08-17T14:18:00Z
**Source review:** .planning/phases/03-auth/03-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3（WR-01、WR-02、WR-03；IN-01/IN-02 为 Info，超出 critical_warning 范围未处理）
- Fixed: 3
- Skipped: 0

**验证环境说明**：所有测试（`go test -count=1 ./internal/server/ ./cmd/wesh/`）在隔离 worktree（`.codebuddy/worktrees/rf-03-*`，临时分支 `gsd-reviewfix/03-*`）内运行通过；提交经 fast-forward 合回 main 后，worktree 已拆除——该验证结果对应修复提交时的代码状态，主仓 checkout 上可直接重跑复现。

## Fixed Issues

### WR-01: `--bind` 无法使用 IPv6 地址（listen 地址拼接缺方括号）

**Files modified:** `cmd/wesh/main.go`, `cmd/wesh/main_test.go`
**Commit:** c0b998c
**Applied fix:**
- `cmd/wesh/main.go:177`：`net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.bind, cfg.port))` 改为 `net.Listen("tcp", net.JoinHostPort(cfg.bind, strconv.Itoa(cfg.port)))`，补 `strconv` import。`net.JoinHostPort` 对 IPv4/主机名输出与现状逐字相同，仅对 IPv6 字面量加方括号，行为零漂移。
- `cmd/wesh/main_test.go`：按审查建议补两条表驱动用例——`TestParseArgs` 增 `ipv6 bind` 行（`--bind ::1` parse 期原样接收）；`TestStartupMatrix` 增 `loopback ipv6 no creds plaintext` 行（锁定 `isLoopbackBind("::1")` = true，与 IPv4 loopback 同等待遇）。
- 验证：`go test -count=1 ./cmd/wesh/` 通过。

### WR-02: `--origin` 的 glob 字符拒绝误伤 IPv6 字面量 Origin

**Files modified:** `internal/server/origin.go`, `internal/server/origin_test.go`, `README.md`
**Commit:** 3d44667
**Applied fix:** 采用审查给出的**选项 2（显式拒绝并文档化）**——行为变更最小、方向保守，与审查者的保守姿态一致；未接受选项 1（接受 IPv6 白名单配置）以避免放行面扩大。
- `internal/server/origin.go`：`NormalizeOrigin` 注释的 glob 拒绝条目补充裁决说明——IPv6 字面量 Origin（如 `https://[::1]:8443`）因含 `[` 被拒，显式不支持配置进白名单；同源 IPv6 访问经 `originAllowed` ② 的 `EqualFold(r.Host, u.Host)` 命中，不受影响。
- `internal/server/origin_test.go`：`TestNormalizeOrigin` 补 `https://[::1]:8443` 拒绝用例，把行为钉死。
- `README.md`：`--origin` flag 行注明 IPv6 字面量 Origin 不支持白名单、同源访问不受影响。
- 验证：`go test -count=1 ./internal/server/` 通过。

### WR-03: 同源 Origin 检查存在 DNS rebinding CSWSH 残余（文档未覆盖）

**Files modified:** `README.md`
**Commit:** a519599
**Applied fix:** 仅落地短期修复——README「安全说明」的「认证与传输安全」条目后新增「已知残余风险（DNS rebinding / CSWSH）」段落：说明 loopback 裸跑（无凭据）模式下 DNS rebinding 可借受害者浏览器绕过同源检查（只读下可观看终端输出，`--writable` 下为完整交互 shell），认证模式下 ticket 闸使该路径实际不可利用，建议不可信网页浏览环境配置凭据。中期 Host 白名单校验按裁决**显式推迟到 Phase 7 SEC-07，本次未实现**。
- 验证：文档改动（Tier 1 重读确认）；随后全量重跑 `go test -count=1 ./internal/server/ ./cmd/wesh/` 通过。

---

_Fixed: 2026-08-17T14:18:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
