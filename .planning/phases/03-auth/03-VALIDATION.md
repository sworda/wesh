---
phase: 3
slug: auth
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-17
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.26.3 stdlib `testing`) |
| **Config file** | none — stdlib |
| **Quick run command** | `go test ./... -run 'Ticket|Auth|Throttle|Origin|Redact|TLS' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'Ticket|Auth|Throttle|Origin|Redact|TLS' -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD-by-planner | — | — | SEC-01..05 | — | TBD | unit/integration | `go test ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Planner fills this map per task when PLAN.md files are created.*

---

## Wave 0 Requirements

- [ ] `auth_ticket_test.go` — stubs for SEC-01 (ticket 单次使用/TTL/重放拒绝)
- [ ] `auth_throttle_test.go` — stubs for SEC-02 (指数退避/常数时间比较)
- [ ] `log_redact_test.go` — stubs for SEC-03 (凭据/ticket/Authorization 日志脱敏)
- [ ] `origin_tls_test.go` — stubs for SEC-04/SEC-05 (Origin 拒绝/TLS 配置/安全头)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| testssl.sh 无弱项 | SEC-05 | 外部扫描工具，需运行中的 TLS 端点 | 启动 wesh --tls 后运行 `testssl.sh https://localhost:7681`，确认无 LOW/MEDIUM/HIGH  findings |
| 浏览器 attach→WS 全流程 | SEC-01 | 需真实浏览器验证 fetch 带凭据 + Hello 首帧核销 | 登录后打开终端页面，DevTools 观察 POST /api/attach 200 且 WS 建立；刷新页面重放旧 ticket 被拒绝 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
