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
| **Framework** | Go stdlib `testing` + `-race`；攻击面用例用裸 TCP 帧 helper；正常 WS 流程用 coder/websocket `Dial` |
| **Config file** | none |
| **Quick run command** | `go test ./... -count=1` |
| **Full suite command** | `go test -race -count=1 ./... && pnpm -C web build` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -count=1`
- **After every plan wave:** Run `go test -race -count=1 ./... && pnpm -C web build`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-03-T1 | 02-03 | 3 | RES-01 | T-02-03-02 | 超限消息 → 1009 关闭（17KiB 单帧 + stderr message_too_big 单行） | e2e | `go test ./internal/server -run TestMessageTooBig -count=1` | ❌ W0 | ⬜ pending |
| 02-03-T2 | 02-03 | 3 | RES-01 | T-02-03-01 | 百万 1 字节 continuation → ≤16385 帧内 1009 且随后握手成功（存活代理） | e2e(裸帧) | `go test ./internal/server -run TestFragmentByteFlood -count=1` | ❌ W0 | ⬜ pending |
| 02-03-T1 | 02-03 | 3 | RES-01 | T-02-03-01 | 分片数 32 硬顶 → 1009 `fragment_limit`（33 个 1B 分片，33B<16KiB 最小反例） | e2e(裸帧) | `go test ./internal/server -run TestFragmentCountLimit -count=1` | ❌ W0 | ⬜ pending |
| 02-03-T1 | 02-03 | 3 | RES-01 | T-02-03-01 | 空帧不崩溃不关连接 | e2e(裸帧) | `go test ./internal/server -run TestEmptyFrame -count=1` | ❌ W0 | ⬜ pending |
| 02-03-T2 | 02-03 | 3 | RES-01 | T-02-03-03 | 空分片慢滴 → 每消息完成时限内连接被关（Q1 裁决=实现，export_test.go 注入 500ms） | e2e(裸帧) | `go test ./internal/server -run TestFragmentTrickle -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T1 | 02-02 | 2 | SEC-08 | T-02-02-03 | 预认证档：Hello 前 >4KiB 消息 → 1009（3KiB 未知字段填充合法 Hello 不误杀） | e2e(裸帧) | `go test ./internal/server -run TestPreAuthReadLimit -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T1 | 02-02 | 2 | (D-17) | — | offer permessage-deflate → 101 响应无扩展协商头（压缩默认关） | e2e(裸HTTP) | `go test ./internal/server -run TestNoPerMessageDeflate -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T1 | 02-02 | 2 | SEC-08 | T-02-02-06 | 无 wesh.v1 → HTTP 400 未升级（含 wesh.v1.evil 前缀伪造负例） | e2e(裸HTTP) | `go test ./internal/server -run TestSubprotocolRequired -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T2 | 02-02 | 2 | SEC-08 | T-02-02-01 | per-IP 半开帽 → 429（SetMaxHalfOpenPerIP(1) 确定性构造）；Hello 完成即释放（409 而非 429） | e2e(裸帧) | `go test ./internal/server -run TestHalfOpenCap -count=1` | ❌ W0 | ⬜ pending |
| 02-02-T1 | 02-02 | 2 | SEC-08 | T-02-02-01 | 5s 无 Hello → 1008 `hello_timeout`（容忍窗 4-8s） | e2e(裸帧) | `go test ./internal/server -run TestHelloTimeout -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | SEC-08 | T-02-01-01 | 抢跑帧 → 1002 且线上无 Error 帧（无 'E' 帧断言） | e2e | `go test ./internal/server -run TestPrematureFrame -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | (D-06/D-07) | T-02-01-01 | version 错 → 先 Error{version_mismatch} 帧后 1008 同名 reason | e2e | `go test ./internal/server -run TestVersionMismatch -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | (D-05) | T-02-01-01 | 数据面 text 消息 → 1002 协议错误 | e2e | `go test ./internal/server -run TestTextFrame1002 -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | CORE-04 | T-02-01-06 | ro 模式 INPUT 被丢弃，Welcome.mode=="ro"，连接存活 | e2e | `go test ./internal/server -run TestReadOnlyDropsInput -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | CORE-04 | T-02-01-06 | ro 下 RESIZE 放行（D-13）：RESIZE 至 132x43 → stty size 输出 "43 132" | e2e | `go test ./internal/server -run TestReadOnlyAllowsResize -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | CORE-04 | T-02-01-06 | --writable 后 INPUT 生效（TestEchoPTY 原地改造：Writable:true 装配 + 握手） | e2e | `go test ./internal/server -run TestEchoPTY -count=1` | ✅ 需改造 | ⬜ pending |
| 02-04-T1 | 02-04 | 4 | CORE-06 | T-02-04-04 | ping 按间隔到达且连接保活（interval=200ms，1s 内 ≥2 个 op=0x9） | e2e(裸帧) | `go test ./internal/server -run TestPingKeepalive -count=1` | ❌ W0 | ⬜ pending |
| 02-04-T2 | 02-04 | 4 | CORE-06 | T-02-04-02 | pong 超时 → 连接关闭（export_test.go SetPongTimeout 注入 500ms） | e2e(裸帧) | `go test ./internal/server -run TestPongTimeout -count=1` | ❌ W0 | ⬜ pending |
| 02-04-T1 | 02-04 | 4 | CORE-06 | T-02-04-04 | --ping-interval=0 → 线上零 ping 帧（D-16 0=禁用） | e2e(裸帧) | `go test ./internal/server -run TestPingDisabled -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T1 | 02-01 | 1 | (D-05) | T-02-01-01 | 关闭码全集静态合规（无 1005/1006/1015/4000 段） | unit | `go test ./internal/proto -run TestCloseCodeSet -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T1 | 02-01 | 1 | (D-02) | T-02-01-05 | Hello 未知字段忽略 | unit | `go test ./internal/proto -run TestHelloIgnoresUnknown -count=1` | ❌ W0 | ⬜ pending |
| 02-01-T2 | 02-01 | 1 | 既有回归 | — | Phase 1 生命周期五测 + echo 全部先过 Hello 握手（dialAndHandshake 收口） | e2e | `go test ./internal/server -count=1` | ✅ 需改造 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task ID / Plan / Wave / Threat Ref 列已由 planner 回填（2026-08-15，5 plans / 5 waves：02-01 握手+ro tracer → 02-02 SEC-08 守卫链 → 02-03 RES-01 三层上限 → 02-04 CORE-06 保活 → 02-05 收口）；行集含 specless probe fallback 边缘分析补充的可辩护 edge（TestFragmentTrickle/TestPreAuthReadLimit/TestNoPerMessageDeflate/TestVersionMismatch/TestTextFrame1002/TestPingDisabled）。*

---

## Wave 0 Requirements

planner 回填（2026-08-15，二次修订对齐 5-plan 结构）：无独立 Wave 0——测试基建与实现同 plan 共生，归属如下（文件名以 02-PATTERNS.md 为准：`rawws_test.go`，本节原名 `ws_rawframe_test.go` 为同一物）：

- [ ] `internal/proto/proto_test.go` — proto 常量表与 Hello 解析单测 → **plan 02-01 Task 1**（wave 1）
- [ ] `internal/server/e2e_test.go` — `dialAndHandshake(t, wsURL, wantMode)` helper 改造既有测试 → **plan 02-01 Task 2**（wave 1；startTestServer 同步加 opts 参）
- [ ] `internal/server/handshake_test.go` — 握手负例（1008/1002/text 1002）→ **plan 02-01 Task 2 新建**；守卫链六测（400/429/释放/5s/预认证档/压缩拒绝）→ **plan 02-02 扩展**（wave 2）
- [ ] `internal/server/rawws_test.go` — 裸帧攻击面测试 helper（dialRawWS/writeRawFrame/readRawFrame）→ **plan 02-02 Task 1**（wave 2；02-03/02-04 复用）
- [ ] `internal/server/export_test.go` — package server 内测缝（SetMaxHalfOpenPerIP/SetMsgCompleteTimeout/SetStderrWriter/SetPongTimeout）→ **plan 02-02 新建，02-03/02-04 增量**
- [ ] `internal/server/limits_test.go` — RES-01 攻击面五测 → **plan 02-03**（wave 3）
- [ ] `internal/server/keepalive_test.go` — 保活三测 → **plan 02-04**（wave 4）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 预认证内存平坦 | SEC-08 | 内存采样断言脆弱，作参考不门禁 | 代码走查 Accept 前守卫区零分配 + flood 测试内存采样 |
| 浏览器端 ro 表现 | CORE-04 | 需真实浏览器观察 | DevTools：ro 标题前缀、键盘无响应、WS 帧面板可见 ping/pong、1009 文案 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
