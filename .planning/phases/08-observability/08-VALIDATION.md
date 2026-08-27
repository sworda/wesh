---
phase: 8
slug: observability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-08-27
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing`（-race 强制，CI ci.yml 既定） |
| **Config file** | 无独立配置——`go.mod` + CI `go test -race -count=1 -v ./...` |
| **Quick run command** | `go test ./internal/server/ -count=1` |
| **Full suite command** | `go vet ./... && go test -race -count=1 ./...` |
| **UAT command** | `node web/uat/phase08.mjs`（真实二进制默认 /tmp/wesh-uat/wesh，先 `time go build -o /tmp/wesh-uat/wesh ./cmd/wesh`） |
| **Estimated runtime** | quick ~15s；full（-race 全量）~2min；phase08.mjs ~1min |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/ -count=1`（涉及包级快速回归）
- **After every plan wave:** Run `go vet ./... && go test -race -count=1 ./...`
- **Before `/gsd:verify-work`:** Full suite must be green + `node web/uat/phase08.mjs` 全场景绿 + 既有 UAT 脚本（phase02..07 及 phase07-b2/dom/width/dims 变体）回归全绿（六段式纪律）
- **Max feedback latency:** ~120 seconds（-race 全量）

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | OPS-08 | T-08-01c | 事件单行 JSON 六键；captureStderr 动态 writer 捕获成立；无双轨 | 黑盒（captureStderr+parseEvents） | `go test ./internal/server/ -run TestLogEventJSON -count=1 -v` | ❌ 随任务创建（log_test.go） | ⬜ pending |
| 08-01-02 | 01 | 1 | OPS-08 | T-08-01b | 红线负断言逐字保留；事件断言 JSON 字段化 | 迁移（5 文件） | `go test ./internal/server/ -count=1` | ✅（迁移既有） | ⬜ pending |
| 08-01-03 | 01 | 1 | OPS-08 | T-08-01b | UAT 断言 JSON 行解析；token/凭据 detail 红线 | UAT 迁移 | `node web/uat/phase05.mjs && node web/uat/phase07.mjs && node web/uat/phase07-b2.mjs` | ✅（迁移既有） | ⬜ pending |
| 08-02-01 | 02 | 2 | OPS-08 | T-08-02c | attach/detach client_id 关联；reason 四值；pinger→detach hubMu 同步边 -race 干净 | 黑盒 ×5 | `go test ./internal/server/ -race -count=1 -run 'TestAttachDetachEvents|TestDetachReason' -v` | ❌ 随任务创建（events_test.go） | ⬜ pending |
| 08-02-02 | 02 | 2 | OPS-08 | — | session_start(pid)/session_end(exit_code/signal/duration)/shutdown 三事件 | 黑盒 ×3 | `go test ./internal/server/ -race -count=1 -run 'TestSessionEnd|TestShutdownEvent' -v` | ❌ 随任务创建 | ⬜ pending |
| 08-02-03 | 02 | 2 | OPS-08 | T-08-02a/02b | throttled 携 retry_after；auth_failed 无用户名；remote 字段 C0/C1/DEL 剥离 | 黑盒 ×2 + 白盒 ×1 | `go test ./internal/server/ -race -count=1 -run 'TestThrottledRetryAfter|TestAuthFailedNoUsername|TestRemoteSanitize' -v` | ❌ 随任务创建 | ⬜ pending |
| 08-03-01 | 03 | 3 | OPS-06 | T-08-03b | /healthz 免认证 200 四字段；bp 根路径固定；405；session_active 翻转 | 黑盒 ×5 子测 | `go test ./internal/server/ -race -count=1 -run TestHealthz -v` | ❌ 随任务创建（health_test.go） | ⬜ pending |
| 08-03-02 | 03 | 3 | OPS-06 | — | Shutdown 进行中 /healthz 503 draining | 集成 | `go test ./internal/server/ -race -count=1 -run TestHealthzDraining -v` | ❌ 随任务创建 | ⬜ pending |
| 08-04-01 | 04 | 4 | OPS-07 | T-08-04d/04e | exposition 格式合法（Content-Type/三行组序/末行 \n）；认证闸两态；escLabel 转义 | 白盒 + 黑盒 | `go test ./internal/server/ -race -count=1 -run 'TestMetricsExposition|TestMetricsAuth|TestBuildInfo' -v` | ❌ 随任务创建（metrics_test.go） | ⬜ pending |
| 08-04-02 | 04 | 4 | OPS-07 | T-08-04a/04e | 五类指标数值正确；快照锁序 -race 干净 | 黑盒 + -race 压力 | `go test ./internal/server/ -race -count=1 -run 'TestMetricsValues|TestMetricsSnapshotRace' -v` | ❌ 随任务创建 | ⬜ pending |
| 08-05-01 | 05 | 5 | OPS-06 | — | S1 healthz 免认证/四字段/bp 固定/405 真实二进制全链 | UAT | `PHASE08_ONLY=S1 node web/uat/phase08.mjs` | ❌ 随任务创建（phase08.mjs） | ⬜ pending |
| 08-05-02 | 05 | 5 | OPS-06/07/08 | T-08-05a | S2-S6 场景矩阵（metrics 两态/指标/draining/事件检索/NEL 剥离） | UAT | `node web/uat/phase08.mjs` | ❌ 随任务创建 | ⬜ pending |
| 08-05-03 | 05 | 5 | OPS-06/07/08 | T-08-05b/05c | README 例外明示与配方落文；六段式全绿 | 文档断言 + 全量回归 | `go vet ./... && go test -race -count=1 ./... && time pnpm -C web build && git diff --exit-code web/dist/index.html` | ✅（README 改） | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

无独立 Wave 0——本 phase 测试基建全部随任务同 plan 创建交付（先例：Phase 7 各 plan 自带测试任务）：

- [x] `parseEvents` helper — 08-01 Task 1 同任务交付（log_test.go）
- [x] 既有断言迁移清单 — 08-01 Task 2/3（limits/emptyexit/auth_e2e/proxy_e2e/multi + phase05/07/07-b2.mjs，RESEARCH Runtime State Inventory 全量盘点）
- [x] 新测试文件（log_test/events_test/health_test/metrics_test）— 各 plan 任务内创建
- [x] 无框架安装需求（stdlib + 既有 Node 零依赖纪律；go.mod/go.sum 逐字节不动——08-04 验收锁）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 真实 Prometheus scrape 兼容（target up + series 可见） | OPS-07 | 本机无 Prometheus 实例（RESEARCH Environment Availability 登记 fallback：exposition 文本断言代替） | 08-UAT.md：按 README 配方起 Prometheus → target up → `wesh_build_info` 等 series 可见 |
| journald 实机检索（jq 两示例） | OPS-08 | 需 systemd 部署环境（本机非 systemd 运行态） | 08-UAT.md：`journalctl -u wesh -o cat \| jq -c 'select(.event=="auth_failed")'` 与 `select(.client_id==N)` |
| draining 窗口编排观测率 | OPS-06 | 默认配置窗口 <1s，轮询观测属部署面时序 | 08-UAT.md：systemd restart 场景观察反代健康检查翻转 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（无独立 Wave 0——随任务交付清单全列出）
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
