---
phase: 06
slug: session-lifecycle
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-23
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Node 原生 WebSocket/fetch UAT 脚本 + @xterm/headless + jsdom + `go test` |
| **Config file** | 无新增 — 复用 `web/uat/phase02.mjs`/`phase03.mjs`/`phase04.mjs` 既有模式；前端 vitest 已在位 |
| **Quick run command** | `cd web && pnpm vitest run src/lib/__tests__` 与 `go test ./...`（增量） |
| **Full suite command** | `node web/uat/phase06.mjs` 与 `node web/uat/phase06-dom.mjs`（Wave 0 新建），加 `go test ./...` 全量 |
| **Estimated runtime** | ~30–60 秒（UAT 全套 spawn 真实二进制 + jsdom + go test） |

---

## Sampling Rate

- **After every task commit:** 对应模块的定向测试（`go test ./internal/...` 或 `pnpm vitest run <file>`）
- **After every plan wave:** `node web/uat/phase06.mjs` 或 `node web/uat/phase06-dom.mjs`（按 wave 归属）+ `go test ./...`
- **Before `/gsd:verify-work`:** 两条 UAT 全绿 + `go test ./...` 全绿 + `pnpm vitest run` 全绿
- **Max feedback latency:** 60 秒

---

## Per-Task Verification Map

*Wave 0 完成后由 planner 在 PLAN.md 中逐 task 填具体行；本表为占位契约。*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-W0-01 | — | 0 | (基建) | — | N/A | harness | `node web/uat/phase06.mjs --help` 与 `node web/uat/phase06-dom.mjs --help` 不崩 | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/uat/phase06.mjs` — 协议层 UAT：`--once` 单接/断开后退出、`--exit-when-empty` 全员断开退出、EXIT 帧（含 exit code、SIGHUP 形态、1000 关闭）、重连接回原 PTY（杀 WS → 重连 → 输入/输出一致性）
- [ ] `web/uat/phase06-dom.mjs` — jsdom 重连前端逻辑：1006 触发自动重连、指数退避 + 上限、手动入口（Reconnect now）、陈旧 socket 代际守卫（`if (sock !== ws) return;`）、1002 协议错误**不**触发重连
- [ ] `web/src/lib/reconnect.ts` — 纯函数抽取（computeDelay(attempt, base, max)、shouldReconnect(closeEvent)），便于 jsdom 直测
- [ ] `internal/server/*_test.go` — 新增 lifecycle/exit-broadcast 用例（EXIT 帧字节为 `'X'`、同步直写而非 outbox 入队、Close(1000)）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 浏览器原生断网/恢复事件序列 | SESS-03 | headless 环境无浏览器；navigator.onLine/offline 真实栈不可测 | 用 `network throttling` 或拔网线，30s 断开后恢复，确认自动重连接回原 PTY；开发机以 UAT + jsdom 单测覆盖 |
| tmux/herdr 屏幕重绘观感 | SESS-03 | 视觉/像素级，依赖真实终端 | 在真实终端断开重连，确认无滚动回放、屏幕靠程序重绘；README 已明示此语义 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
