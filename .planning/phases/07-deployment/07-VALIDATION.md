---
phase: 7
slug: deployment
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-25
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib + `go vet`；UAT: Node 原生脚本（零依赖）+ jsdom 25.0.1 |
| **Config file** | 无单独配置（go test 内建）；CI 见 .github/workflows/ci.yml |
| **Quick run command** | `go test ./cmd/wesh ./internal/pty ./internal/server`（<5s 级；按任务 -run 收窄） |
| **Full suite command** | `go test -race -count=1 ./...`（~50s）+ `time pnpm -C web build` + `node web/uat/phase07.mjs` |
| **Estimated runtime** | quick <5 秒；full ~50 秒 + UAT |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/wesh ./internal/pty ./internal/server -run '<相关 TestX>' -count=1`
- **After every plan wave:** Run `go test -race -count=1 ./...` + `time pnpm -C web build`
- **Before `/gsd:verify-work`:** Full suite must be green + phase07.mjs 全场景 PASS + 既有 phase02-06 UAT 回归
- **Max feedback latency:** 50 秒

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | OPS-09 | T-7-配置文件错误回显 | 配置错误文案禁含敏感值 | unit | `go test -run 'TestConfigFile\|TestParseArgs' ./cmd/wesh` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | OPS-01 | T-7-残留socket预置 | listen 前 Remove + listen 后 Chmod/Chown | unit + UAT | `go test ./cmd/wesh -run TestSocket`；`node web/uat/phase07.mjs` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | OPS-02 | — | base-path 页面/WS 升级/307/share 交叉 | unit + UAT | `go test ./internal/server -run TestBasePath`；phase07.mjs | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SEC-07 | T-7-日志注入 / T-7-XFF伪造 | remote_user sanitize C0/C1+128；XFF 换键 | unit + UAT | `go test ./internal/server -run 'TestRemoteUser\|TestXFF'` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | OPS-04 | T-7-信号误发 | pgid == 子进程 pid（setsid 不变量） | unit + UAT | `go test ./internal/pty -run 'TestStartOptions\|TestSignalGroup'`；phase07.mjs | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | OPS-05 | T-7-降权身份错乱 | HOME/USER/LOGNAME 按 LookupId 改写或剔除 | unit | `go test ./internal/pty -run TestDropPriv` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | OPS-11 | — | headless 跳过 --open；fake xdg-open 断言 argv | unit | `go test ./cmd/wesh`（main_test.go 新增用例） | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | D-23 | — | 1001 关停序列（SIGTERM → 1001 → 退出） | UAT | `node web/uat/phase07.mjs`（stop-signal 场景） | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task ID / Plan / Wave 由 planner 分配后在执行期回填。*

---

## Wave 0 Requirements

- [ ] `web/uat/phase07.mjs` — 覆盖 OPS-01/02/04/09/11、SEC-07、D-23 的协议层场景（含 TCP relay 夹具）
- [ ] `cmd/wesh/main_test.go` 扩展 — 配置文件/新 flag 组合校验表驱动用例（沿用 TestParseArgs/TestStartupMatrix 既有表结构）

*既有 Go 测试基建完备，无框架安装需求；jsdom/xterm-headless 已在 web/uat 就位。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 浏览器自动打开真实效果 | OPS-11 | headless Linux 开发机无浏览器；Windows 侧实测属浏览器层豁免 | 部署后在有 GUI 环境运行 `wesh --open` 人工确认 |
| 反代真实 nginx 挂载观感 | OPS-02 | 像素/真实反代时序属平台原生豁免（UAT 以 relay+HTTP 断言替代） | README 部署手册复核 + 截图留档 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 50s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
