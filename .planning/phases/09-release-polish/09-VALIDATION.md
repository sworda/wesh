---
phase: 09
slug: release-polish
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-29
---

# Phase 09 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test（Go 1.26.3，含 fuzz `-fuzz`）+ Node 原生 WebSocket/fetch UAT 脚本（web/uat/phaseNN.mjs） |
| **Config file** | none — 现有 go.mod 与 web/uat/ 先例（phase02/03/04.mjs） |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test ./... && go vet ./...` + 相关 `web/uat/phaseNN.mjs` 脚本 |
| **Estimated runtime** | ~60 秒（不含负载/模糊长测） |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run full suite + 该波次相关 UAT 脚本
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 秒

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | OPS-03 / OPS-10 | — | N/A | unit / uat | `go test ./...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*（planner 产出 PLAN.md 后回填每任务行）*

---

## Wave 0 Requirements

- [ ] `goreleaser` 本机安装（go install 或官方二进制）— 发布链任务前置
- [ ] `loadFileConfig` reader 委托接缝重构 — TOML fuzz 目标前置

*若不需要：删除对应行。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| darwin 产物 macOS 冒烟 | OPS-03 | 本机无 macOS；RESEARCH 已登记取舍 | scp 到 Mac 运行 `wesh --version` |
| Caddy/Cloudflare 空闲超时行为面 | OPS-10 | 官方文档被网络策略阻断，D-15 既定实证兜底 | 按部署文档配方在真实反代后观察空闲连接 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
