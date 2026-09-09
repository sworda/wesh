---
phase: 06
slug: session-lifecycle
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-23
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Node 原生 WebSocket/fetch UAT 脚本 + @xterm/headless + jsdom + `go test` + `node --test`（前端 lib 单测） |
| **Config file** | 无新增 — 复用 `web/uat/phaseNN.mjs` 既有模式；前端 lib 单测为 `node --test` 直跑 `.ts`（Node 24 内建 type stripping，零框架零配置——项目内不存在前端测试框架依赖，`web/package.json` 与 `web/uat/package.json` 均零命中） |
| **Quick run command** | `node --test web/src/lib/*.test.ts` 与 `go test ./...`（增量） |
| **Full suite command** | `go build -o /tmp/wesh-uat/wesh ./cmd/wesh` 后九脚本对同一构建产物：`node web/uat/phase02.mjs && node web/uat/phase03.mjs && node web/uat/phase04.mjs && node web/uat/phase04-t1-width.mjs && node web/uat/phase05.mjs && node web/uat/phase05-dom.mjs && node web/uat/phase05-dims.mjs && node web/uat/phase06.mjs && node web/uat/phase06-dom.mjs`；加 `go test -race -count=1 ./...` 与 `node --test web/src/lib/*.test.ts` 全量 |
| **Estimated runtime** | ~30–60 秒（单 UAT 脚本/定向 go 包级）；九脚本全量约 2–3 分钟 |

---

## Sampling Rate

- **After every task commit:** 对应模块的定向测试（`go test ./internal/...` 或 `node --test web/src/lib/<file>.test.ts`）
- **After every plan wave:** 该 wave 归属的 UAT 脚本（`node web/uat/phase06.mjs` 或 `node web/uat/phase06-dom.mjs`）+ `go test -race -count=1 ./...`
- **Before `/gsd:verify-work`:** 九脚本 UAT 全绿（phase02/03/04/04-t1-width/05/05-dom/05-dims/06/06-dom 对同一构建产物）+ `go test -race -count=1 ./...` 全绿 + `node --test web/src/lib/*.test.ts` 全绿 + `pnpm -C web build` 退出 0
- **Max feedback latency:** 60 秒

---

## Per-Task Verification Map

*06-07 实测同步（占位契约行已替换为各 plan 实测行；Status 均为执行期全绿终态）。*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-01-T2 | 06-01 | 1 | SESS-03 | T-06-01a/b/c | EXIT 帧契约（'X'=0x58 + ExitPayload）+ lifecycle 写序安全广播（同步 Write(EXIT,2s)→Close(1000)，禁 outbox 入队）+ signalName/exitMessage 服务端唯一写口 | unit+integration | `go test -race -count=1 ./internal/proto/ ./internal/server/ -run 'TestExit\|TestSignalName\|TestProtocolConstants'` | ✅ `internal/proto/proto_test.go`、`internal/server/exit_test.go`、`internal/server/exitmsg_test.go` | ✅ green |
| 06-01-T3 | 06-01 | 1 | SESS-03 | T-06-01b | 前端 EXIT 承接（EXIT=0x58 + lastExit 暂存 + onclose 1000 正文）+ dist 重建 | build+unit | `pnpm -C web build && node --test web/src/lib/*.test.ts` | ✅ `web/src/main.ts`、`web/dist/index.html` | ✅ green |
| 06-02-T2 | 06-02 | 2 | SESS-01, SESS-02 | T-06-02a/b/c/d | pty.SignalHangup（SIGHUP 进程组）+ Options.ExitWhenEmpty（set/grace 分离）+ 注册表空迁移触发（detach/kick 两调用点）+ 宽限计时器恰好一次启停 + lifecycle exiting 门 | unit+integration | `go test -race -count=1 ./internal/pty/ ./internal/server/ -run 'TestSignalHangup\|TestExitWhenEmpty'` | ✅ `internal/pty/signal_linux.go`、`signal_darwin.go`、`internal/server/emptyexit_test.go` | ✅ green |
| 06-03-T1 | 06-03 | 2 | CORE-05 | T-06-03a | backoffMs/shouldReconnect 纯函数（1s×2 封顶 30s；仅 1006 触发） | unit | `node --test web/src/lib/reconnect.test.ts` | ✅ `web/src/lib/reconnect.ts`、`reconnect.test.ts` | ✅ green |
| 06-03-T2 | 06-03 | 2 | CORE-05 | T-06-03a/b/c/d | 重连状态机四函数 + case 1006 分派 + 代际守卫×4 + online/offline 双触发 + WELCOME 成功点 term.clear() + 既有面板零漂移 | build+integration | `pnpm -C web build && node web/uat/phase04-dom.mjs && node web/uat/phase05-dom.mjs` | ✅ `web/src/main.ts`、`web/dist/index.html` | ✅ green |
| 06-04-T2 | 06-04 | 3 | SESS-01, SESS-02 | T-06-04a/b/c | --once 语法糖展开（≡ --max-clients=1 --exit-when-empty=0）+ --exit-when-empty[=duration] IsBoolFlag 三形态 + validateStartup 两组合冲突行（exit 2 先于资源占用）+ Options 接线 | unit | `go test -race -count=1 ./cmd/wesh/` | ✅ `cmd/wesh/main.go`、`cmd/wesh/main_test.go` | ✅ green |
| 06-05-T1/T2 | 06-05 | 3 | CORE-05 | T-06-05a/b | jsdom 重连状态机八场景（1006 全链/1002·1013·1008 不触发/双触发幂等/Reconnect now/代际守卫/EXIT 全链/online 快路径）+ D9 豁免 + assertOutputClean 红线自证 | automated_ui | `node web/uat/phase06-dom.mjs` | ✅ `web/uat/phase06-dom.mjs` | ✅ green（30/30 + 1 skipped） |
| 06-06-T1/T2 | 06-06 | 4 | SESS-01, SESS-02, SESS-03, CORE-05 | T-06-06a/b | 协议层七场景（EXIT 双端广播/信号死亡/--once 全链 503+255/--exit-when-empty 三形态/断连重接同一 PTY）+ S7 豁免 + assertOutputClean 红线自证 | integration | `node web/uat/phase06.mjs` | ✅ `web/uat/phase06.mjs` | ✅ green（23/23 + 1 skipped） |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `web/uat/phase06.mjs` — 协议层 UAT：`--once` 单接/断开后退出、`--exit-when-empty` 全员断开退出、EXIT 帧（含 exit code、SIGHUP 形态、1000 关闭）、重连接回原 PTY（杀 WS → 重连 → 输入/输出一致性）——06-06 交付（23/23 + 1 skipped）
- [x] `web/uat/phase06-dom.mjs` — jsdom 重连前端逻辑：1006 触发自动重连、指数退避 + 上限、手动入口（Reconnect now）、陈旧 socket 代际守卫（`if (sock !== ws) return;`）、1002 协议错误**不**触发重连——06-05 交付（30/30 + 1 skipped）
- [x] `web/src/lib/reconnect.ts` — 纯函数抽取（computeDelay(attempt, base, max)、shouldReconnect(closeEvent)），便于 jsdom 直测——06-03 交付（backoffMs/shouldReconnect，node --test 3/3）
- [x] `internal/server/*_test.go` — 新增 lifecycle/exit-broadcast 用例（EXIT 帧字节为 `'X'`、同步直写而非 outbox 入队、Close(1000)）——06-01 交付（exit_test.go/exitmsg_test.go）+ 06-02 交付（emptyexit_test.go 六测）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 浏览器原生断网/恢复事件序列 | SESS-03 | headless 环境无浏览器；navigator.onLine/offline 真实栈不可测 | 见 `.planning/phases/06-session-lifecycle/06-UAT.md` Test 1（断网 30s 恢复自动重连，外部浏览器可执行）；开发机以 phase06.mjs S6（协议层真实 TCP 断连重接）+ phase06-dom.mjs D1/D8（合成事件状态机）覆盖等价面 |
| tmux/herdr 屏幕重绘观感 | SESS-03 | 视觉/像素级，依赖真实终端 | 见 `.planning/phases/06-session-lifecycle/06-UAT.md` Test 2（重连成功清屏与程序重绘观感）；README 生命周期节已明示无滚动回放语义，自动化等价面见 phase06-dom.mjs D1h（term.clear() 可观测）+ phase05.mjs S8（SIGWINCH 强制重绘） |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending（06-07 全量回归终证后由 validate-phase 收口）
