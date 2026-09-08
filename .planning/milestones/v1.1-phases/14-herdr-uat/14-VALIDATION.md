---
phase: 14
slug: herdr-uat
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-06
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> 种子自 14-RESEARCH.md「## Validation Architecture」（plan-phase §5.5）；Per-Task Verification Map 随 PLAN.md 落地后填充。

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `-race`（go1.26.3）；UAT 侧无框架（自带 check() 收集器 + exit code 门禁，web/uat/phaseNN.mjs 先例） |
| **Config file** | 无独立测试配置（go.mod 钉版；`//go:build load` 隔离负载格） |
| **Quick run command** | `go test -race -count=1 -run TestPerClient ./internal/server/`（单族抽样；改造哪族跑哪族） |
| **Full suite command** | `time go test -race -count=1 -v ./...`（CI 同款）；负载格 `go test -tags=load -count=1 -timeout=30m ./internal/server/ -v`；UAT `node web/uat/run-all.mjs`（本 phase 新建） |
| **Estimated runtime** | ~120 seconds（全量 -race；负载格单独 30m 护栏手动跑） |

---

## Sampling Rate

- **After every task commit:** `go test -race -count=1 -run '<本任务触及测试族>' ./internal/server/`（改造哪族跑哪族）
- **After every plan wave:** `time go test -race -count=1 ./...` 全量 + 触及文件双模式子测试逐名核对
- **Before `/gsd:verify-work`:** Full suite must be green + 收口闸六段式先例（11-06/12-05/13-08 三代同构）：静态面 + 全量 -race + darwin 双编译闸 + dist byte-identical + UAT 矩阵（run-all 全绿 + phase14.mjs + phase14-pw.mjs Windows 侧）+ diff 白名单审查
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD     | TBD  | TBD  | PC-12/PC-13 | T-14-XX    | 随 PLAN.md 落地填充 | TBD  | TBD               | TBD         | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky · ✅ gate-passed（manual-gate 人工裁决门，Nyquist 约定豁免 automated verify）*

---

## Wave 0 Requirements

- [ ] `web/uat/phase14.mjs` — herdr 协议层 UAT（PC-13 协议面 + ro 汇聚）
- [ ] `web/uat/pw/phase14-pw.mjs` — Windows Playwright 双 tab 观感（PC-13 浏览器面）
- [ ] `web/uat/run-all.mjs` — 一键矩阵 runner（D-04）
- [ ] load_test.go per-client 双剖面格（TestLoadPerClientFloodMatrix / 驻留格——名称 Discretion）
- [ ] `newTestServer` harness（harness_test.go 或 export_test.go——Discretion 面）+ 三维归类逐文件改造（~40 文件，按归属类别分 wave：mode-agnostic → mode-mapped → mode-exclusive）
- [ ] 框架安装：无（零新依赖——全部既有件：Go stdlib testing/-race、Node 24 原生 WS、@xterm/headless 6.0.0、playwright 1.62.1 钉版、herdr 0.8.100）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 真实 OS 网卡栈断网时序 | PC-12/PC-13 边界 | 平台原生行为显式豁免（CODEBUDDY.md 测试策略第 5 条） | UAT 中以 `skipped` + reason 记录，风险接受；用 TCP 转发器 kill/restore 模拟替代 |
| herdr/tmux 真实终端观感（像素级） | PC-13 | 浏览器权限弹窗/像素视觉豁免（同上） | phase14-pw.mjs 截图留档人工复核 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
