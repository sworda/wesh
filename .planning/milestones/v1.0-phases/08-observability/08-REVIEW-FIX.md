---
phase: 08-observability
fixed_at: 2026-08-28T21:40:00+08:00
review_path: .planning/phases/08-observability/08-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
out_of_scope: 4
status: all_fixed
---

# Phase 08: Code Review Fix Report

**Fixed at:** 2026-08-28T21:40:00+08:00
**Source review:** .planning/phases/08-observability/08-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope (critical_warning): 2（WR-01、WR-02；本 phase 无 Critical/Blocker 级）
- Fixed: 2
- Skipped (in-scope): 0
- Out of scope (Info 级，fix_scope=critical_warning 不含): 4（IN-01..IN-04，见末节登记）

**验证环境声明：** 全部构建与测试在隔离 worktree（`.codebuddy/worktrees/rf-08-*`，分支 `gsd-reviewfix/08-*`）内执行；Go 测试不依赖 worktree 外资源，清理后主 checkout 的 `phase08-observability` 分支经 fast-forward 含同等提交，可直接复跑复现。

## Fixed Issues

### WR-01: exit-when-empty 在「递补升格踢出致空」路径下重复触发——事件恰好一次纪律失守

**Files modified:** `internal/server/server.go`、`internal/server/clients.go`、`internal/server/export_test.go`、`internal/server/e2e_test.go`、`internal/server/emptyexit_test.go`
**Commit:** df92a3f
**Applied fix:**

生产代码（按审查建议逐字落地，三处）：

1. `server.go` Server 结构体新增 `exitEmptySignaled bool` 字段（hubMu 保护，与 `exitEmptyTimer` 同锁同生命周期纪律，附空纪元语义注释）。
2. `clients.go` `maybeExitWhenEmptyLocked` 三守卫扩为四守卫（`|| s.exitEmptySignaled`），通过守卫后立即置位——「空纪元内幂等：promote 踢出致空与外层移除点只发一次」；doc 注释同步更新为四守卫语义。
3. `server.go` Attach 升档序列 `registerLocked` 成功后的同一 hubMu 持有内清零（`s.exitEmptySignaled = false`，与 `cancelExitEmptyTimerLocked` 同点）——新 attach 开启新纪元，下次致空重新允许触发。

回归测试（新增 `TestExitWhenEmptyPromoteKickOnce`，emptyexit_test.go）：

- 夹具：owner A + rwEligible 递补者 B（owner 默认策略，D-07 降级 ro），`/bin/cat` 静默（onChunk 不触发，B 不会被 ro-满即踢路径先收口），pinger 零值禁用。
- 「B 的 outbox 连升格 Welcome 都写不进」由新白盒出口 `ShrinkOutboxForTest`（export_test.go）注入——把 B 的 outbox cap 改写为 1，promote 的 `trySend` 结构性必败。**未采用真实字节预填**：实测发现 writer 的 drain 是整批 swap 语义，预填与 drain 竞态下「填满」状态会在填充返回后、promote 前被一次 drain 清空（45s 超时实证），cap 改写无此窗口且与 TCP 吸收带/平台缓冲无关。
- 断言链：B 端 1013 slow_consumer 客户端侧证据（assertKicked1013 复用）→ waitHandlers 同步边 → stderr 事件 `exit_when_empty_wait` 恰 1 条（修复前 2 条，remote 分属 B/A）、`exit_when_empty` 0 条（grace=1min 未到期）、`detach reason=kick` 恰 1 条（promote 同义踢出分支正面证据，排除假绿形态）。
- 配套 helper：`startTrackedServerHandle`（e2e_test.go）——startTrackedServerWith 的 `*server.Server` 暴露变体，语义逐字相同。

**Test results:**
- 红验证：stash 生产修复后新测试 FAIL（`exit_when_empty_wait count = 2, want 1`，事件流逐条呈现双重触发路径）→ 恢复后 PASS（0.01s）——甄别力实证。
- `go build ./...` + `go vet ./...` 通过。
- `go test ./internal/server/... -count=1` 全绿（53.4s）。
- `-race` 关键组（TestExitWhenEmpty*/TestSuccession*/TestOwnerPolicy/TestSlowConsumerKick/TestClientCountInvariant）全绿（16.5s）。

### WR-02: `--ping-interval` 缺负值闸——负 duration 静默退化为「禁用保活」

**Files modified:** `cmd/wesh/main.go`、`cmd/wesh/main_test.go`、`cmd/wesh/config_test.go`
**Commit:** 36df4c9
**Applied fix:**

1. `main.go` parseArgs 的 Parse 返回校验区、`--stop-timeout` 负值闸同位新增：
   `if cfg.pingInterval < 0 { return ..., fmt.Errorf("invalid --ping-interval %v: must be a non-negative duration (0 = disable keepalive)", cfg.pingInterval) }`——与审查建议逐字一致，值非敏感回显（exitEmptyValue.Set/--stop-timeout 同纪律）；配置来源负值经默认值替换机制落 `cfg.pingInterval` 同一终值，一闸双覆盖。
2. `main_test.go` TestTLSKeyPairError 负例表新增 CLI 行：`--ping-interval=-5s` → 断言 `invalid --ping-interval -5s`。
3. `config_test.go` TestConfigRedLines「config values flow through existing parse validation」表新增配置来源行：`ping-interval = "-5s"` → 同闸拒绝（一闸双覆盖回归锁）。

**Test results:**
- 红验证：stash `main.go` 后 CLI 行 FAIL（`parseArgs = nil error`——负值静默放行的 bug 形态复现）→ 恢复后 PASS。
- 配置来源子测试 `ping-interval_negative` PASS。
- `go test ./cmd/... -count=1` 全绿（0.056s）。
- 终态全仓 `go test ./... -count=1` 全绿（cmd/wesh、internal/proto、internal/pty、internal/server、internal/web 五包）。

## Skipped Issues (out of scope)

以下 Info 级发现属 `fix_scope=critical_warning` 之外，本轮不修，登记待后续处置：

### IN-01: README detach 行「计数走 metrics」对 pong_timeout 不成立

**File:** `README.md:451`
**Reason:** out of scope（Info 级，fix_scope=critical_warning）
**Original issue:** 事件目录表 detach 行括注后半句只对 kick 成立——17 series 契约中无 pong_timeout 计数器，operator 按表找会扑空。修法应改文档而非加 series（17 series 锁定契约）。

### IN-02: uid/gid 值域上限常量 `4294967295` 在 32 位架构下编译失败

**File:** `cmd/wesh/main.go:628-633`
**Reason:** out of scope（Info 级，可移植性隐患而非现行缺陷——目标平台 amd64/arm64 不受影响）
**Original issue:** `int` 与无类型常量 `4294967295` 比较在 GOARCH=386 等 32 位目标上编译期错误。修法二选一：显式 int64 转换比较，或登记 32 位 Out of Scope 假设。

### IN-03: `normalizeBasePath` 放行含 `/.` 段的值，该值在 ServeMux 下结构性不可路由

**File:** `cmd/wesh/main.go:804-827`
**Reason:** out of scope（Info 级；非安全问题，失败形态是响亮 404）
**Original issue:** Go 1.22+ ServeMux 先 CleanPath 重定向，注册模式串未净化，含单点段 base-path 下所有端点 404。修法：校验追加单点段拒绝。

### IN-04: detach reason 判定序下 pong_timeout 与并发正常关闭/关停广播的归属竞态

**File:** `internal/server/clients.go:729-735`
**Reason:** out of scope（Info 级；审查注明「处置可 wontfix」——两窄竞态窗口内连接本身确已恶化，误标仅影响审计归因精度，无功能后果）
**Original issue:** reason 判定 switch 先查 `c.pongTimedOut` 再查 `s.exiting` 的两条窄竞态；如决定修正可将 `s.exiting` 判定提前消 (b) 窗口，(a) 窗口本质性存在。

---

_Fixed: 2026-08-28T21:40:00+08:00_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
