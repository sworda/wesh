---
phase: 13
slug: resource-defense
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-05
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test（服务端）+ Node 原生 WebSocket/fetch 协议层 UAT 脚本（`web/uat/phaseNN.mjs` 先例，零依赖） |
| **Config file** | none — 既有 go test + web/uat 先例覆盖 |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test ./... && node web/uat/phase13*.mjs`（本 phase 新增 UAT 脚本，随 wave 落地） |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run `go test ./...` + 本 wave 涉及的 `web/uat/phase13*.mjs`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | PC-08/PC-09/SEC-09/OPS-12 | — | TBD（planner 填充：令牌桶限速断言、SIGKILL 兜底收割断言、关停覆盖断言、退出码对齐断言、metrics/审计断言） | unit + uat | TBD | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] 无新框架安装 — 既有 go test + web/uat 协议层脚本先例覆盖
- [ ] churn 负载测试脚本骨架（合法票据 10rps × 30s，断言 RSS/goroutine/fd 有界）——随首个 churn 任务落地

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 真实 OS 网卡栈断网时序下 churn 行为 | PC-08 | 平台原生行为显式豁免（CODEBUDDY.md 测试策略第 5 条） | UAT 中以 `skipped` + reason 记录，风险接受；用 TCP 转发器 kill/restore 模拟替代 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
