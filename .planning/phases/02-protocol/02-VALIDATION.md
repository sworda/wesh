---
phase: 02
slug: protocol
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-15
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `-race`（CI 固化：ubuntu + macOS 双平台）；攻击面用例经库客户端 `Writer()`/`DialOptions` 构造（D-09 修订后无裸帧 helper），正常流程用 coder/websocket 客户端 `Dial` |
| **Config file** | none |
| **Quick run command** | `go test -race -count=1 ./internal/server/ ./internal/proto/` |
| **Full suite command** | `go vet ./... && go test -race -count=1 -v ./... && pnpm -C web build`（与 CI 两腿一致） |
| **Estimated runtime** | ~30 seconds |

前端无单测 runner（web/package.json 无 test script）——前端改动由 `tsc` 类型检查（build 内含）+ Go e2e（裸 WS 客户端驱动）+ 期末人工 UAT（config human_verify_mode: end-of-phase）三层收口，与 Phase 1 相同。不引入 vitest。

---

## Sampling Rate

- **After every task commit:** Run `go test -race -count=1 ./internal/server/ ./internal/proto/`
- **After every plan wave:** Run `go vet ./... && go test -race -count=1 -v ./... && pnpm -C web build`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> 2026-08-15 planner 回填（plan 重生成后）：逐测试函数一行；Threat Ref 指向各 plan `<threat_model>` 的 T-02-XX 编号。

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 02-01 | 1 | D-02（CORE-04 契约面） | T-02-01 | TestDecodeHello：Hello 编解码往返 + 未知字段忽略 + 畸形拒绝 + ClampDim | unit | `go test -race -count=1 -run TestDecodeHello ./internal/proto/` | ❌ 02-01 新建 | ⬜ pending |
| 2-01-02 | 02-01 | 1 | D-07 | T-02-02 | TestWelcomeFrameErrorFrame：'W'/'E' 帧形状与 {mode}/{code,message} 往返 | unit | `go test -race -count=1 -run TestWelcomeFrameErrorFrame ./internal/proto/` | ❌ 02-01 新建 | ⬜ pending |
| 2-01-03 | 02-01 | 1 | D-05/D-09修订 | T-02-01/03 | TestProtocolConstants：Subprotocol/帧字节/上限常量逐字锁定 | unit | `go test -race -count=1 -run TestProtocolConstants ./internal/proto/` | ❌ 02-01 新建 | ⬜ pending |
| 2-02-01 | 02-02 | 2 | CORE-04 | T-02-04 | TestHelloWelcome：握手全帧校验 + mode ro/rw 与 INPUT echo | e2e | `go test -race -count=1 -run TestHelloWelcome ./internal/server/` | ❌ 02-02 新建（e2e_test.go 内） | ⬜ pending |
| 2-02-02 | 02-02 | 2 | D-10/D-11 回归 | — | 既有六测（TestEchoPTY/TestSecondClient409/TestExitCodePropagation/TestUnknownFrame1002/TestClientDisconnectSIGHUP 经 dialHello 握手改造 + TestDrainBeforeAttach 适配 New 签名——无 Dial 不需握手）保持绿 | e2e | `go test -race -count=1 ./internal/server/` | ✅ 改造（e2e_test.go） | ⬜ pending |
| 2-02-03 | 02-02 | 2 | CORE-04/D-12① | T-02-04 | 前端子协议建连 + Hello 首帧 + Welcome(disableStdin/[ro] 标题) + onclose 按码分派 | 构建 + 人工 | `pnpm -C web build`；浏览器行为并入 02-06 UAT | ✅ 修改（main.ts） | ⬜ pending |
| 2-03-01 | 02-03 | 3 | SEC-08 | T-02-13 | TestSubprotocolRequired：无子协议 400 / 错子协议 400 / 多值头放行 | e2e | `go test -race -count=1 -run TestSubprotocolRequired ./internal/server/` | ❌ 02-03 新建 | ⬜ pending |
| 2-03-02 | 02-03 | 3 | SEC-08 | T-02-09/10 | TestHalfOpenPerIP429：半开占帽后第 2 条 429；在先连接不误伤（acquire/release 恰好一次） | e2e | `go test -race -count=1 -run TestHalfOpenPerIP429 ./internal/server/` | ❌ 02-03 新建 | ⬜ pending |
| 2-03-03 | 02-03 | 3 | SEC-08 | T-02-09 | TestHelloTimeout：静默连接 1008 + reason hello_timeout（HelloTimeout=200ms 注入） | e2e | `go test -race -count=1 -run TestHelloTimeout ./internal/server/` | ❌ 02-03 新建 | ⬜ pending |
| 2-03-04 | 02-03 | 3 | D-06 | T-02-06 | TestPrematureFrame：抢跑 INPUT → 1002 且全程无 'E' 帧（攻击面零反馈） | e2e | `go test -race -count=1 -run TestPrematureFrame ./internal/server/` | ❌ 02-03 新建 | ⬜ pending |
| 2-03-05 | 02-03 | 3 | D-06/D-07 | T-02-07 | TestVersionMismatch：先收 Error{version_mismatch} 帧再收 1008，机器串同名 | e2e | `go test -race -count=1 -run TestVersionMismatch ./internal/server/` | ❌ 02-03 新建 | ⬜ pending |
| 2-03-06 | 02-03 | 3 | CORE-04 | T-02-12 | TestReadOnlyDropsInput：ro 下裸 WS 客户端 INPUT 无回显且连接存活（服务端边界） | e2e | `go test -race -count=1 -run TestReadOnlyDropsInput ./internal/server/` | ❌ 02-03 新建 | ⬜ pending |
| 2-03-07 | 02-03 | 3 | CORE-04 | T-02-12 | TestReadOnlyAllowsResize：ro 下 Hello 尺寸生效 + RESIZE 后 stty 跟随（D-13 放行） | e2e | `go test -race -count=1 -run TestReadOnlyAllowsResize ./internal/server/` | ❌ 02-03 新建 | ⬜ pending |
| 2-04-01 | 02-04 | 4 | CORE-06 | T-02-14/15 | TestPingKeepalive：PingInterval=100ms 注入，空闲 >3 间隔连接存活且 echo 正常 | e2e | `go test -race -count=1 -run TestPingKeepalive ./internal/server/` | ❌ 02-04 新建 | ⬜ pending |
| 2-04-02 | 02-04 | 4 | CORE-06 | T-02-16 | TestPongTimeout：客户端停 Read 不回 pong → interval+PongTimeout 内服务端断开，exitf(0) 收口 | e2e | `go test -race -count=1 -run TestPongTimeout ./internal/server/` | ❌ 02-04 新建 | ⬜ pending |
| 2-04-03 | 02-04 | 4 | CORE-06 | T-02-15 | TestPingDisabled：PingInterval=0 时不回 pong 仍存活（反证未发 ping） | e2e | `go test -race -count=1 -run TestPingDisabled ./internal/server/` | ❌ 02-04 新建 | ⬜ pending |
| 2-05-01 | 02-05 | 5 | RES-01 | T-02-17 | TestOversize1009：16KiB+1 消息 → 1009 + stderr message_too_big 单行事件（D-12②） | e2e | `go test -race -count=1 -run TestOversize1009 ./internal/server/` | ❌ 02-05 新建 | ⬜ pending |
| 2-05-02 | 02-05 | 5 | RES-01 | T-02-17 | TestReadLimitBoundary：16384 正常 echo / 16385 → 1009（边界精确） | e2e | `go test -race -count=1 -run TestReadLimitBoundary ./internal/server/` | ❌ 02-05 新建 | ⬜ pending |
| 2-05-03 | 02-05 | 5 | RES-01 | T-02-18 | TestFragmentedFlood1009：库客户端 Writer 逐帧写 1 字节分片流 → 累积 16385 处 1009 | e2e | `go test -race -count=1 -run TestFragmentedFlood1009 ./internal/server/` | ❌ 02-05 新建 | ⬜ pending |
| 2-05-04 | 02-05 | 5 | RES-01 | T-02-19 | TestEmptyFragmentFloodResilience：5000 空消息洪水下服务存活、echo 正常、HeapAlloc 增量 <8MiB（宽松参考防线，不断言 1009——D-09 修订） | e2e | `go test -race -count=1 -run TestEmptyFragmentFloodResilience ./internal/server/` | ❌ 02-05 新建 | ⬜ pending |
| 2-05-05 | 02-05 | 5 | SEC-08 | T-02-17 | TestPreHelloReadLimit：Hello 前 >4KiB 消息 → 1009（预认证档生效） | e2e | `go test -race -count=1 -run TestPreHelloReadLimit ./internal/server/` | ❌ 02-05 新建 | ⬜ pending |
| 2-06-01 | 02-06 | 6 | 全部 | — | 六段式收口：GOROOT gofmt / vet / -race 全量 / web 构建 / 裸 clone / 冒烟（--help 新 flag、400 预检） | 全量 + 人工 | `go vet ./... && go test -race -count=1 ./... && pnpm -C web build` | — | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

无独立 Wave 0——测试基建随首个实现 plan（02-02）共生，planner 回填归属如下：

- [x] `internal/server/e2e_test.go`：`dialHello(t, ctx, wsURL, cols, rows)`（带子协议 Dial + 发 Hello + 收 Welcome 校验 mode；尺寸参数化——02-03 TestReadOnlyAllowsResize 复用同一签名传 111x44，既有测试统一 80x24）与 `startTestServerWith(t, argv, opts)` 变体——**归属 02-02 Task 1**（与 New 签名变更同任务，编译原子性）
- [x] 超时/上限注入点：采用 `server.Options` struct 字段注入（`HelloTimeout` 02-02 落地、`MaxHalfOpenPerIP` 02-03 追加、`PingInterval`/`PongTimeout` 02-04 追加；零值取生产默认常量）——沿用 exitf 注入模式（server.go:44 先例），替代 export_test.go 包级变量测缝（e2e 为 `package server_test` 外包，且包级变量改写有 -race 并行风险）
- [x] 裸帧 helper（rawws_test.go）：**不需要**——D-09 修订后测试矩阵全部库客户端可构造（分片流用 `c.Writer()` 逐 Write 产生非 fin 帧；子协议负例用 DialOptions；空帧洪水用空消息近似），PATTERNS.md 的裸帧段随之作废
- [x] 框架安装：无——stdlib testing 覆盖全部需求

*详细映射以 02-RESEARCH.md "Validation Architecture" 一节为准（测试函数名经 planner 按 D-09 修订调整：TestFragmentCountLimit 等分片数测试已删除）。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 预认证内存平坦 | SEC-08 | 内存采样断言脆弱，作参考不门禁 | 代码走查 Accept 前守卫区零分配 + flood 测试内存采样 |
| 浏览器端 ro 表现 | CORE-04 | 需真实浏览器观察 | DevTools：ro 标题前缀、键盘无响应、WS 帧面板可见 ping/pong、1009 文案 |
| 空帧洪水残余风险 | RES-01 | 库吞空帧，无应用层钩子（Open Question 1 已 RESOLVED：D-09 修订等效防线） | flood 下服务存活、内存平坦、其他连接功能不受影响 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
