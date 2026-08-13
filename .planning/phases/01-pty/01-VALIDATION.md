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
> Seeded from 01-RESEARCH.md ## Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `-race`（后端）；`vite build` 内 tsc 类型检查（前端）；e2e 用 coder/websocket `Dial` 客户端（零新增测试依赖） |
| **Config file** | none（go test 零配置） |
| **Quick run command** | `go test ./... -count=1` |
| **Full suite command** | `go test -race -count=1 ./... && pnpm -C web build` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -count=1`（秒级）
- **After every plan wave:** Run `go test -race -count=1 ./... && pnpm -C web build`
- **Before `/gsd:verify-work`:** Full suite must be green + CI 双平台绿 + 手动 FE checklist 通过
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | 01 | 1 | CORE-01 | — | spawn 任意命令+双向转发 | e2e：server `wesh -- /bin/cat`，WS Dial 发 INPUT 断言同字节 OUTPUT | `go test ./internal/server -run TestEchoPTY -count=1` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | CORE-01 | 命令注入 | exec 数组不经 shell：`$(id)` 不被展开 | unit | `go test ./internal/pty -run TestExecArrayNoShell` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | CORE-02 | — | resize 同步 TIOCSWINSZ：`Setsize(50,132)` 后 `stty size` 输出 `50 132` | 集成 | `go test ./internal/pty -run TestResize -count=1` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | SEC-06 | env 泄露 | 白名单：宿主注入 `AWS_SECRET_ACCESS_KEY`，spawn `/usr/bin/env` 输出不含 | unit+e2e | `go test ./internal/pty -run TestEnvWhitelist` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | SEC-06/Pitfall | fd 破坏 | spawn 不存在二进制，err 非 nil 且 fd 0/1/2 `Fsync` 非 EBADF | unit | `go test ./internal/pty -run TestSpawnFailKeepsStdio` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | （成功准则3） | 僵尸残留 | 短命令退出后 `/proc/<pid>` 消失 | 集成 | `go test ./internal/pty -run TestReap` | ❌ W0 | ⬜ pending |
| TBD | 01 | 1 | （研究旗帜） | darwin 收割 | kqueue NOTE_EXIT 正常路径 + 先死后注册竞态路径 | CI-only 双测试 | `go test ./internal/pty -run TestKqueue -count=1`（macos runner） | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` / `cmd/wesh/main.go` 骨架——greenfield 从零创建
- [ ] `internal/pty/{spawn_test,io_test,reap_test}.go`——覆盖 CORE-01/CORE-02/SEC-06/收割
- [ ] `internal/server/e2e_test.go`——WS echo 端到端（httptest.Server + websocket.Dial）
- [ ] `web/` 前端工程（package.json/vite.config.ts/index.html/src/main.ts）+ `web/dist/index.html` 占位
- [ ] `.github/workflows/ci.yml`——linux/darwin matrix，`go test -race`
- [ ] 框架安装：无需安装——Go stdlib testing 零依赖；前端依赖由 `pnpm install --frozen-lockfile` 落地

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| WebGL→DOM 回落 | FE-01 | 需真实浏览器 GPU 控制 | DevTools 禁用 WebGL 后页面仍渲染终端 |
| 窗口变化自适应 | FE-03 | 需真实窗口拖拽 | 拖动窗口，远端 `stty size` 跟随变化；vim 重绘正常 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
