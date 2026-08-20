---
phase: 05
slug: multi-client
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-20
---

# Phase 05 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `-race`（CI 强制）；UAT = Node 原生 WS 脚本（web/uat/phaseNN.mjs 零依赖传统）+ @xterm/headless 6.0.0 + jsdom 25.0.1 |
| **Config file** | none（CI `.github/workflows/ci.yml`：`go test -race -count=1 -v ./...` + `pnpm -C web install --frozen-lockfile && pnpm -C web build`） |
| **Quick run command** | `go test -race -count=1 ./internal/server/` |
| **Full suite command** | `go test -race -count=1 ./... && time pnpm -C web build && go build -o /tmp/wesh-uat/wesh ./cmd/wesh && node web/uat/phase05.mjs && node web/uat/phase02.mjs && node web/uat/phase03.mjs && node web/uat/phase04.mjs` |
| **Estimated runtime** | ~60 秒（Go 全量+race+构建+四个 UAT 脚本） |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -count=1 ./internal/server/ ./internal/proto/ ./internal/pty/`（秒级）
- **After every plan wave:** Run full suite command（上表）
- **Before `/gsd:verify-work`:** Full suite must be green，phase02/03/04.mjs 回归适配后全过 + phase05.mjs 全过
- **Max feedback latency:** 60 秒

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-W0-01 | 05-01 | 1 | 生命周期改造（多客户端必然推论） | — | 客户端断开不再触发 exitf/SIGHUP；子进程退出唯一终结路径 | integration | `go test -race -run 'TestDetach\|TestExitBroadcast' ./internal/server/` | ❌ W0（e2e_test.go 改造） | ⬜ pending |
| 05-01-01 | 05-01 | 1 | MULTI-01 | 慢客户端 DoS | 双客户端 attach 收到同一 OUTPUT 流 | integration | `go test -race -run TestMultiClientFanout ./internal/server/` | ❌ W0（multi_test.go） | ⬜ pending |
| 05-01-02 | 05-03 | 3 | MULTI-02 | owner 权限抢夺 | owner/all 矩阵：Welcome mode、per-client INPUT 门、递补升格 | integration | `go test -race -run 'TestOwnerPolicy\|TestAllPolicy\|TestSuccession' ./internal/server/` | ❌ W0 | ⬜ pending |
| 05-01-03 | 05-02 | 2 | MULTI-03 | 慢客户端 DoS | stall 客户端 1013 被踢 reason=slow_consumer；其他客户端无卡顿；ReadLoop 不阻塞 | integration | `go test -race -run TestSlowConsumerKick ./internal/server/` | ❌ W0（slowclient_test.go） | ⬜ pending |
| 05-01-04 | 05-04 | 4 | MULTI-04 | — | 异尺寸双客户端 min-rect；2→1 恢复 last-wins；50ms 防抖合并 | unit + integration | `go test -race -run 'TestArbitrate\|TestResizeArbitration' ./internal/server/` | ❌ W0（resize_arb_test.go） | ⬜ pending |
| 05-02-01 | 05-06 | 6 | MULTI-05 | token 爆破/泄露 | 启动打印两条 /s/{token}/ 链接；token GET 200（无 Basic）；错 token → Basic 矩阵；token→/api/attach→ticket→attach 全链 mode 正确 | UAT + unit | `node web/uat/phase05.mjs` + `go test -race -run TestShareToken ./internal/server/` | ❌ W0（phase05.mjs + sharetoken_test.go） | ⬜ pending |
| 05-01-05 | 05-05 | 5 | RES-02 | 输入洪水 DoS | INPUT 洪水超限被丢弃且连接存活、未超限部分送达 | integration | `go test -race -run TestInputRateLimit ./internal/server/` | ❌ W0 | ⬜ pending |
| 05-02-02 | 05-07 | 7 | RES-03 | 连接数耗尽 | max-clients 满员 → /ws Accept 前 HTTP 503；halfOpen 计数不泄漏 | integration | `go test -race -run TestMaxClients503 ./internal/server/` | ❌ W0 | ⬜ pending |
| 05-01-06 | 05-02 | 2 | RES-04 | 全局背压 DoS | 全体可写端 stall → 信用门闭合（子进程输出暂停可观测）；一端恢复/死亡 → 门开 | integration | `go test -race -run TestGlobalCredit ./internal/server/` | ❌ W0 | ⬜ pending |
| 05-01-07 | 05-03 | 3 | MULTI-02 | owner 权限抢夺 | owner 被 1013 踢出 → 晋升先于其重连登记（同一 hubMu 时序闭合）；重连旧 owner 归队 FIFO 尾；全程单 owner（review #3） | integration | `go test -race -run TestSuccessionKickRace ./internal/server/` | ❌ W0 | ⬜ pending |
| 05-01-08 | 05-07 | 7 | RES-03 | 计数器泄漏 | 客户端计数对称不变量：register/remove 交错序列逐步 n == len(set) + 非成员移除幂等（review #7） | unit（同包白盒） | `go test -race -run TestClientCountInvariant ./internal/server/` | ❌ W0（clients_test.go） | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Plan/Wave 列已按 9-wave 结构回填（plan-phase §13 收口时，2026-08-20）：01=tracer+生命周期，02=信用门+SIGWINCH，03=权限体系，04=resize 仲裁，05=限速+CR-01 背压，06=分享链接，07=max-clients，08=前端，09=UAT 收口。*

---

## Wave 0 Requirements

- [ ] `internal/server/e2e_test.go` 单次语义迁移（P5-4：≥8 处 waitExit 断言改写、TestSecondClient409 替换、SIGHUP 两测删除）——**最高优先，阻塞一切新测试**
- [ ] `internal/server/multi_test.go` — 双客户端 dialHello 变体（复用 e2e_test.go:133/177 形态）+ fan-out/权限/递补测试组
- [ ] `internal/server/slowclient_test.go` — stall 客户端夹具（建连后不 Read）+ 输出洪水子进程夹具
- [ ] `internal/server/resize_arb_test.go` — arbitrate 纯函数表测 + Getsize 集成断言
- [ ] `internal/server/sharetoken_test.go` — token store subtle 比较 + /s/ 路由门禁 + /api/attach token 分支
- [ ] `internal/server/clients_test.go` — 同包白盒 registry 计数不变量（review #7，tickets_test.go 先例）
- [ ] `web/uat/phase05.mjs` — phase04.mjs 骨架复用（startWesh/spawnExpectExit/dialHello/check 形态）+ raw-socket stall 夹具（S6，review #8）
- [ ] `server.Options` 测试覆写字段扩展（OutboxBytes/MaxClients/InputRate/InputBurst/ResizeDebounce — HelloTimeout 先例）
- [ ] 前端 dist 重建（`time pnpm -C web build`，构建后验证产物时间戳）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 浏览器原生权限弹窗/OS IME 栈/像素视觉 | — | 项目硬约束（headless 环境永不具备浏览器，CODEBUDDY.md 显式豁免） | UAT 中以 `skipped` + reason 记录并风险接受 |

*其余所有 phase 行为均有自动化验证（Go test / Node UAT / jsdom）。*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
