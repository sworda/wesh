---
phase: 1
slug: pty
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-13
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `-race`（后端）；`vite build` 内 tsc 类型检查（前端）；e2e 用 coder/websocket `Dial` 客户端 |
| **Config file** | none（go test 零配置） |
| **Quick run command** | `go test ./... -count=1` |
| **Full suite command** | `go test -race -count=1 ./... && pnpm -C web build` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -count=1`
- **After every plan wave:** Run `go test -race -count=1 ./... && pnpm -C web build`
- **Before `/gsd:verify-work`:** Full suite must be green + CI 双平台绿 + 手动 FE checklist 通过
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | CORE-01 | — | exec 数组不经 shell（`$(id)` 不被展开） | unit | `go test ./internal/pty -run TestExecArrayNoShell` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | SEC-06 | T-1-env | env 白名单：注入 `AWS_SECRET_ACCESS_KEY` 到宿主 env，断言子进程 env 不含 | unit | `go test ./internal/pty -run TestEnvWhitelist` | ❌ W0 | ⬜ pending |
| 1-01-03 | 01 | 1 | SEC-06/Pitfall 1 | T-1-fd | spawn 失败不伤 fd：spawn 不存在二进制后 fd 0/1/2 `Fsync` 非 EBADF | unit | `go test ./internal/pty -run TestSpawnFailKeepsStdio` | ❌ W0 | ⬜ pending |
| 1-01-04 | 01 | 1 | CORE-02 | — | resize 同步 TIOCSWINSZ：spawn `stty size; sleep 1; stty size`，中途 `Setsize(50,132)`，输出含 `50 132` | 集成 | `go test ./internal/pty -run TestResize -count=1` | ❌ W0 | ⬜ pending |
| 1-01-05 | 01 | 1 | 成功准则3 | T-1-zombie | 收割无僵尸：短命令退出后 `/proc/<pid>` 消失（linux） | 集成 | `go test ./internal/pty -run TestReap` | ❌ W0 | ⬜ pending |
| 1-01-06 | 01 | 1 | CORE-01 | — | WS echo e2e：`wesh -- /bin/cat`，`Dial` 发 INPUT 帧，断言收到同字节 OUTPUT 帧 | e2e | `go test ./internal/server -run TestEchoPTY -count=1` | ❌ W0 | ⬜ pending |
| 1-01-07 | 01 | 1 | 研究旗帜 | — | darwin kqueue 竞态：正常路径 + 僵尸注册路径双测试（CI-only，macos runner） | CI 集成 | `go test ./internal/pty -run TestKqueue -count=1`（macos runner） | ❌ W0 | ⬜ pending |
| 1-01-08 | 01 | 1 | FE-01 | — | WebGL→DOM 回落：DevTools 禁 WebGL 后页面仍渲染 | 手动 | — | 手动 checklist | ⬜ pending |
| 1-01-09 | 01 | 1 | FE-03 | — | 窗口变化自适应：拖动窗口远端 `stty size` 跟随，vim 重绘正常 | 手动 | — | 手动 checklist | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` / `cmd/wesh/main.go` 骨架——greenfield 从零创建
- [ ] `internal/pty/{spawn_test,io_test,reap_test}.go` —— 覆盖 CORE-01/CORE-02/SEC-06/收割
- [ ] `internal/server/e2e_test.go` —— WS echo 端到端（httptest.Server + websocket.Dial）
- [ ] `web/` 前端工程（package.json/vite.config.ts/index.html/src/main.ts）+ `web/dist/index.html` 占位
- [ ] `.github/workflows/ci.yml`
- [ ] 框架安装：无需安装——Go stdlib testing 零依赖；前端依赖由 `pnpm install --frozen-lockfile` 落地

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| WebGL 渲染失败回落 DOM | FE-01 | 需真实浏览器 GPU 上下文 | DevTools → Rendering → 禁 WebGL，刷新页面，终端仍渲染且可交互 |
| 窗口拖动自适应 + vim 重绘 | FE-03 | 需人工观察 TUI 重绘质量 | 打开 `wesh -- vim`，拖动浏览器窗口，远端 `stty size` 跟随变化且 vim 无闪屏/错乱 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
