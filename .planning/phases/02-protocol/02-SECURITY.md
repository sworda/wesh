---
phase: 02
slug: protocol
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-15
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| client → server（未认证 WS 握手） | Accept 前 HTTP 预检 + Accept 后 4KiB/5s 窗口内全部输入不可信 | 握手字节流 / 子协议头 |
| client → server（稳态数据帧流） | 16KiB 稳态档内全部字节流不可信 | INPUT/RESIZE 帧载荷 |
| server → PTY 会话 | INPUT 帧跨越信任边界写入 PTY；ro 门是唯一写守门 | 用户键盘输入 |
| server → client（保活控制帧） | ping/pong 唯一服务端周期流量；误装配自伤可用性 | WS 控制帧 |
| 协议契约 → 前后端消费者 | proto.go 常量/schema 是全体实现的信任锚点 | 帧类型/版本常量 |
| 文档 → 部署者 | README 是部署安全认知唯一来源（补偿控制） | 部署警示/限制说明 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Tampering | proto.go 帧类型/版本常量 | high | mitigate | TestProtocolConstants 逐字锁定帧字节与 Subprotocol（VERIFICATION 实跑 PASS） | closed |
| T-02-02 | Information Disclosure | ErrorPayload schema | medium | mitigate | D-06 受众分治钉死常量注释；攻击面路径零 Error 帧（TestPrematureFrame 锁定） | closed |
| T-02-03 | DoS | 上限常量取值 | high | mitigate | ReadLimitPreAuth/PostAuth 注释标定依据；分片层等效防线注释位（D-09 修订） | closed |
| T-02-04 | Elevation of Privilege | server.go ro 门 | high | mitigate | 服务端按 writable 丢弃 INPUT（server.go:349）；TestHelloWelcome + TestReadOnlyDropsInput 双层锁定；UAT 自动化 T1c 标记串零回显 | closed |
| T-02-05 | DoS | Attach 预认证窗口 | high | mitigate | SetReadLimit(4KiB) + 5s hello_timeout(1008)；读 ctx 恒无 deadline；UAT 自动化 T4c 实测 5002ms 1008 | closed |
| T-02-06 | Tampering | 首帧校验（抢跑/空消息/畸形） | medium | mitigate | 三类违规 1002 直关零反馈 + stderr logEvent（D-06/D-12②） | closed |
| T-02-07 | Information Disclosure | version_mismatch 反馈 | low | accept | AR-02-01 | closed |
| T-02-08 | Tampering | 子协议头解析 | medium | mitigate | headerHasToken 按 token 精确比较 + Accept 后 Subprotocol() assert 双闸；TestSubprotocolRequired 锁定 | closed |
| T-02-09 | DoS | 半开连接占用（慢 loris 变种） | high | mitigate | per-IP 半开上限 8（429）+ 5s hello_timeout + ReadHeaderTimeout 5s 三层；TestHalfOpenPerIP429/TestHelloTimeout PASS | closed |
| T-02-10 | DoS | per-IP 计数器泄漏误 429 | medium | mitigate | release 恰好一次不变量 + 到 0 删 key；TestHalfOpenPerIP429 "c1 随后握手成功"断言 | closed |
| T-02-11 | Information Disclosure | 守卫拒绝响应（400/429/409） | low | accept | AR-02-02 | closed |
| T-02-12 | Elevation of Privilege | ro 模式 INPUT 绕过 | high | mitigate | TestReadOnlyDropsInput 裸 WS 客户端视角断言服务端丢弃（前端 disableStdin 不可信） | closed |
| T-02-13 | Tampering | 子协议多值头误判 | low | mitigate | TestSubprotocolRequired(c) 多值头放行组锁定 headerHasToken 拆分语义 | closed |
| T-02-14 | DoS | pinger 误装配 | high | mitigate | 独立 goroutine + 单 reader 并发（库硬性要求）；TestPingKeepalive 空闲存活回归 | closed |
| T-02-15 | DoS | 健康长空闲会话被误杀 | high | mitigate | 读路径恒无 deadline；仅 pong 超时断开；TestPingKeepalive 锁定；UAT 自动化 T1d 11s+ 存活 | closed |
| T-02-16 | DoS | 攻击者占用连接不应答 ping | low | accept | AR-02-03 | closed |
| T-02-17 | DoS | 单帧/累积字节超限 | critical | mitigate | SetReadLimit 两档库流式执行超限自动 1009；TestOversize1009/TestReadLimitBoundary/TestPreHelloReadLimit PASS | closed |
| T-02-18 | DoS | 1 字节 × N 分片洪水 | high | mitigate | 累积字节硬顶 16385 处切断；库流式重组 O(1)/帧；TestFragmentedFlood1009 PASS | closed |
| T-02-19 | DoS | 0 字节空帧洪水 | medium | accept | AR-02-04 | closed |
| T-02-20 | Information Disclosure | 库自动 1009 的 close reason | low | accept | AR-02-05 | closed |
| T-02-21 | Information Disclosure | README 误部署到不可信网络 | high | mitigate | 无认证警示首屏醒目 + "协议基线 ≠ 公网安全" + per-IP 反代聚合限制文档化（VERIFICATION 对码确认） | closed |
| T-02-22 | Tampering | 文档与 wire 漂移 | medium | mitigate | 协议节直译 D-03/D-05/D-13~D-16 与 main.go flag 真源；grep 断言关键串存在 | closed |
| T-02-SC ×6 | Tampering | npm/pip/cargo installs | high | accept | AR-02-06 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-07 | 正常客户端需要可诊断版本错误（D-06 受众分治）；Error 帧只含 code+固定文案，无内部细节 | 用户（D-06 裁决） | 2026-08-15 |
| AR-02-02 | T-02-11 | 标准 HTTP 状态语义不含内部细节；D-06 攻击面零反馈裁决；TestPrematureFrame 锁定无 'E' 帧 | 用户（D-06 裁决） | 2026-08-15 |
| AR-02-03 | T-02-16 | pong 超时 CloseNow 即处置路径；单次语义下 exitf 收口释放全部资源 | plan-time 裁决 | 2026-08-15 |
| AR-02-04 | T-02-19 | 无应用层钩子（库内部吞空帧 read.go:457-479）——D-09 修订用户已裁决接受；预认证窗口由 5s 超时+per-IP 8+409 门盒住；TestEmptyFragmentFloodResilience 断言存活+内存平坦 | 用户（D-09 修订裁决） | 2026-08-15 |
| AR-02-05 | T-02-20 | reason 不含敏感信息；机器串可见性落 stderr（D-12②）；前端按 code 分派不读 reason | plan-time 裁决 | 2026-08-15 |
| AR-02-06 | T-02-SC ×6 | 本 phase 零新增依赖（RESEARCH §Package Legitimacy Audit：Legitimacy Gate 不适用）；coder/websocket v1.8.15 与 @xterm/xterm 6.0.0 均 Phase 1 已钉版 | plan-time 裁决 | 2026-08-15 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-15 | 28 | 28 | 0 | gsd-secure-phase (L1, asvs_level=1) |

验证依据：02-VERIFICATION.md goal-backward 10/10 truths 全绿（`go test -race -count=1 ./...` 4 包 ok）；limits 五测/守卫 ro 八测/保活三测全 PASS；02-UAT.md 协议层自动化 11/11 断言通过（web/uat/phase02.mjs）。

已知非阻塞项（非本 phase 协议层威胁，移交跟踪）：CR-01（Attach 读循环同步写 PTY master 阻塞）用户已决策立即最小缓解；WR-01（S→C 写无超时背压）defer Phase 5；详见 02-VERIFICATION.md「Code Review 发现评估」节。

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-15
