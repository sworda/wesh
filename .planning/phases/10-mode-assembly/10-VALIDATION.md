---
phase: 10
slug: mode-assembly
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-03
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test（Go 原生，含 -race）+ Node 零依赖协议 UAT 脚本（web/uat/phaseNN.mjs） |
| **Config file** | none — Go 原生测试框架，无独立配置 |
| **Quick run command** | `go test ./cmd/wesh -run 'TestParseArgs|TestStartupMatrix|TestConfigMerge|TestConfigPrecedence|TestConfigRedLines' -count=1` |
| **Full suite command** | `go test -race -count=1 ./...` + `node web/uat/phase0{2..9}.mjs` |
| **Estimated runtime** | ~60s（-race 全量）/ ~90s（八 UAT 脚本） |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/wesh -run 'TestParseArgs|TestStartupMatrix' -count=1`
- **After every plan wave:** Run `go test -race -count=1 ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | PC-01 | — | N/A | unit+integration | `go test ./cmd/wesh -run TestParseArgs -count=1`（session-mode 表行）+ 冒烟三形态 | ✅ | ✅ green |
| 10-01-02 | 01 | 1 | PC-01 | — | N/A | unit | `go test ./cmd/wesh -run TestParseArgs -count=1`（wantSessionMode 断言 + D-04 拒绝行，append-only） | ✅ | ✅ green |
| 10-02-01 | 02 | 2 | PC-01 | — | N/A | unit+e2e | `go test ./cmd/wesh -run 'TestStartupMatrix|TestValidateStartupWarnMerge' -count=1` + 冒烟 warn/LookPath | ✅ | ✅ green |
| 10-02-02 | 02 | 2 | PC-01 | — | N/A | unit | `go test ./internal/server -run TestValidateOptions -count=1` + `go test ./internal/pty -run TestStartWithSizeDelegation -count=1` | ✅ | ✅ green |
| 10-03-01 | 03 | 2 | PC-01 | — | N/A | unit | `go test ./cmd/wesh -run 'TestConfigMerge|TestConfigPrecedence|TestConfigRedLines' -count=1`（六新子测） | ✅ | ✅ green |
| 10-03-02 | 03 | 2 | PC-01 | — | N/A | fuzz | `go test ./cmd/wesh -run FuzzDecodeFileConfig -count=1`（十种子）+ `-fuzz FuzzDecodeFileConfig -fuzztime 30s` | ✅ | ✅ green |
| 10-04-01 | 04 | 3 | PC-01 | — | N/A | other | grep 计数闸（CONFIGURATION session-mode×5 / 30 键×2 / 装配中×2 / README×1 / --help 行） | ✅ | ✅ green |
| 10-04-02 | 04 | 3 | PC-01 | — | N/A | e2e | `go test -race -count=1 ./...` + `node web/uat/phase0{2..9}.mjs`（八脚本 exit 0）+ 冒烟三命令 | ✅ | ✅ green |
| 10-05-01 | 05 | 1 | PC-01 | — | N/A | e2e+unit | WR-01 六形态进程级冒烟矩阵 + `go test ./cmd/wesh -run 'TestStartupMatrix/per-client_cwd' -count=1`（三命名行） | ✅ | ✅ green |
| 10-05-02 | 05 | 1 | PC-01 | — | N/A | other | WR-02 断言链：`grep -c 'verr := server.ValidateOptions('` == 1 + V(1328)<P(1334)<L(1342) 位序 | ✅ | ✅ green |
| 10-05-03 | 05 | 1 | PC-01 | — | N/A | e2e | `go test -race -count=1 ./...` + 八 UAT 脚本 + append-only 零删除行 + banana 双源冒烟 | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

（Go 原生测试框架 + 既有 web/uat 协议脚本体系自 Phase 02 起沿用；本 phase 全部验证挂点在既有测试文件 append-only 或既有脚本原样重跑，无新基础设施需求。）

---

## Manual-Only Verifications

All phase behaviors have automated verification.

（21/21 coverage 交付物 classify-coverage 判定全量 auto_passed、present 为空；2026-09-03 verify-work 自动化复验五链全绿：静态三闸 / -race 五包 / 冒烟矩阵 12/12 / 八 UAT 脚本 / 文档口径。）

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（零 MISSING）
- [x] No watch-mode flags
- [x] Feedback latency < 120s（全量 -race ~60s；八 UAT 脚本 ~90s）
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-03
