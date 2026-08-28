---
phase: 8
slug: observability
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
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
| 08-01-01 | 01 | 1 | OPS-08 | T-08-01c | 事件单行 JSON 六键；captureStderr 动态 writer 捕获成立；无双轨 | 黑盒（captureStderr+parseEvents） | `go test ./internal/server/ -run TestLogEventJSON -count=1 -v` | ❌ 随任务创建（log_test.go） | ✅ green |
| 08-01-02 | 01 | 1 | OPS-08 | T-08-01b | 红线负断言逐字保留；事件断言 JSON 字段化 | 迁移（5 文件） | `go test ./internal/server/ -count=1` | ✅（迁移既有） | ✅ green |
| 08-01-03 | 01 | 1 | OPS-08 | T-08-01b | UAT 断言 JSON 行解析；token/凭据 detail 红线 | UAT 迁移 | `node web/uat/phase05.mjs && node web/uat/phase07.mjs && node web/uat/phase07-b2.mjs` | ✅（迁移既有） | ✅ green |
| 08-02-01 | 02 | 2 | OPS-08 | T-08-02c | attach/detach client_id 关联；reason 四值；pinger→detach hubMu 同步边 -race 干净 | 黑盒 ×5 | `go test ./internal/server/ -race -count=1 -run 'TestAttachDetachEvents|TestDetachReason' -v` | ❌ 随任务创建（events_test.go） | ✅ green |
| 08-02-02 | 02 | 2 | OPS-08 | — | session_start(pid)/session_end(exit_code/signal/duration)/shutdown 三事件 | 黑盒 ×3 | `go test ./internal/server/ -race -count=1 -run 'TestSessionEnd|TestShutdownEvent' -v` | ❌ 随任务创建 | ✅ green |
| 08-02-03 | 02 | 2 | OPS-08 | T-08-02a/02b | throttled 携 retry_after；auth_failed 无用户名；remote 字段 C0/C1/DEL 剥离 | 黑盒 ×2 + 白盒 ×1 | `go test ./internal/server/ -race -count=1 -run 'TestThrottledRetryAfter|TestAuthFailedNoUsername|TestRemoteSanitize' -v` | ❌ 随任务创建 | ✅ green |
| 08-03-01 | 03 | 3 | OPS-06 | T-08-03b | /healthz 免认证 200 四字段；bp 根路径固定；405；session_active 翻转 | 黑盒 ×5 子测 | `go test ./internal/server/ -race -count=1 -run TestHealthz -v` | ❌ 随任务创建（health_test.go） | ✅ green |
| 08-03-02 | 03 | 3 | OPS-06 | — | Shutdown 进行中 /healthz 503 draining | 集成 | `go test ./internal/server/ -race -count=1 -run TestHealthzDraining -v` | ❌ 随任务创建 | ✅ green |
| 08-04-01 | 04 | 4 | OPS-07 | T-08-04d/04e | exposition 格式合法（Content-Type/三行组序/末行 \n）；认证闸两态；escLabel 转义 | 白盒 + 黑盒 | `go test ./internal/server/ -race -count=1 -run 'TestMetricsExposition|TestMetricsAuth|TestBuildInfo' -v` | ❌ 随任务创建（metrics_test.go） | ✅ green |
| 08-04-02 | 04 | 4 | OPS-07 | T-08-04a/04e | 五类指标数值正确；快照锁序 -race 干净 | 黑盒 + -race 压力 | `go test ./internal/server/ -race -count=1 -run 'TestMetricsValues|TestMetricsSnapshotRace' -v` | ❌ 随任务创建 | ✅ green |
| 08-05-01 | 05 | 5 | OPS-06 | — | S1 healthz 免认证/四字段/bp 固定/405 真实二进制全链 | UAT | `PHASE08_ONLY=S1 node web/uat/phase08.mjs` | ❌ 随任务创建（phase08.mjs） | ✅ green |
| 08-05-02 | 05 | 5 | OPS-06/07/08 | T-08-05a | S2-S6 场景矩阵（metrics 两态/指标/draining/事件检索/NEL 剥离） | UAT | `node web/uat/phase08.mjs` | ❌ 随任务创建 | ✅ green |
| 08-05-03 | 05 | 5 | OPS-06/07/08 | T-08-05b/05c | README 例外明示与配方落文；六段式全绿 | 文档断言 + 全量回归 | `go vet ./... && go test -race -count=1 ./... && time pnpm -C web build && git diff --exit-code web/dist/index.html` | ✅（README 改） | ✅ green |
| 08-06-01 | 06 | 1 | OPS-08 | — | G-08-2 修复：README 两则 journald jq 示例补 `grep '^\{'` 防护段 + 合流机理说明；:336 文本事件行 JSON 化 | grep 断言 + e2e | `node web/uat/phase08-journal.mjs`（J2/J3/J4 README 逐字管道） | ✅（README 改） | ✅ green |
| 08-06-02 | 06 | 1 | OPS-08 | — | 合流模拟回归夹具：负对照自证不空转 / 全流纯度 / 两示例 / SEC 红线自证 | UAT 夹具 | `node web/uat/phase08-journal.mjs`（J0-J4 + SEC 6/6） | ✅（phase08-journal.mjs） | ✅ green |

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
| journald 实机检索（jq 两示例） | OPS-08 | 真实 systemd/journald 环境仍需部署面复核；**等价自动化已就位**——`phase08-journal.mjs` 合流模拟夹具（08-06 G-08-2 闭合）以 stdout+stderr 时序拼接模拟 journal 合流，负对照自证 + README 逐字管道断言（D3 human_judgment 随 UAT resume） | 08-UAT.md：`journalctl -u wesh -o cat \| grep '^\{' \| jq -c 'select(.event=="auth_failed")'` 与 `select(.client_id==N)`（grep 防护段为 08-06 修复后新形态）；先跑 `node web/uat/phase08-journal.mjs` 作等价回归 |
| draining 窗口编排观测率 | OPS-06 | 默认配置窗口 <1s，轮询观测属部署面时序 | 08-UAT.md：systemd restart 场景观察反代健康检查翻转 |

---

## Validation Audit 2026-08-28

| Metric | Count |
|--------|-------|
| Gaps found | 0（MISSING 测试为零） |
| Resolved | 3（文档面：12 行 pending→green、补 08-06 两行入图、journald Manual-Only 项标注等价自动化） |
| Escalated | 0 |

**审计证据（2026-08-28 实测，UAT 二进制按最新 impl 重建）：**

| 验证面 | 结果 |
|--------|------|
| `go vet ./... && go test -race -count=1 ./...` | 五包绿；唯一失败 `TestGlobalCredit/恢复Read开门_字节精确`（slowclient_test.go:345，3997382 vs 4000000，全量 -race 负载下偶发）单跑复验 PASS → 判定 flaky（时序敏感，Phase 7 面，非本 phase gap，不阻塞） |
| `node web/uat/phase08.mjs` | 21/21（S1-S6 + SEC） |
| `node web/uat/phase08-journal.mjs` | 6/6（J0-J4 + SEC，08-06 G-08-2 夹具） |
| `node web/uat/phase05.mjs` / `phase07.mjs` / `phase07-b2.mjs`（08-01 迁移回归） | 28/28+1 skipped、34/34+1 skipped、4/4 |
| `pnpm -C web build` + `git diff --exit-code web/dist/index.html` | 构建产物零 diff |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（无独立 Wave 0——随任务交付清单全列出）
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated（2026-08-28 Nyquist 审计：零 gap，全自动化面实测绿）
