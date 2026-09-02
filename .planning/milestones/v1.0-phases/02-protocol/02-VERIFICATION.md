---
phase: 02-protocol
verified: 2026-08-15T12:49:47Z
status: passed
score: 10/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_view_findings_assessment:

  - id: CR-01
    severity_in_review: critical
    verifier_classification: warning
    is_phase_goal_gap: false
    decision_owner: human

  - id: WR-01
    severity_in_review: warning
    verifier_classification: warning
    is_phase_goal_gap: false
    addressed_in: "Phase 5"

  - id: WR-02
    severity_in_review: warning
    verifier_classification: warning
    is_phase_goal_gap: false
human_verification:

  - test: "只读默认 + 首帧 Hello 顺序：wesh -- bash 后浏览器打开页面，标题带 [ro] 前缀，键盘敲击终端无反应；DevTools → Network → WS 面板确认首帧为 Hello('H') 与 Welcome(mode=ro)，后续每 5s 一对 ping/pong 帧"
    expected: "标题 [ro] 前缀、键盘无响应、首帧 Hello（非 RESIZE）、Welcome mode=ro、周期 ping/pong"
    why_human: "前端 helloSent 门的真实浏览器时序（term.onResize 常驻接线在首次 fit 触发）Go e2e 覆盖不到；disableStdin/[ro] 标题是渲染层行为"

  - test: "可写模式：wesh --writable -- bash 刷新页面，键入命令"
    expected: "输入正常回显，Welcome(mode=rw)"
    why_human: "真实浏览器端到端输入回显需人工确认"

  - test: "resize：ro 模式下拖动浏览器窗口（可先 --writable 起 vim 观察全屏程序）"
    expected: "无法输入但尺寸跟随重绘；DevTools 可见 RESIZE 帧照常发出"
    why_human: "全屏程序 resize 重绘是视觉行为"

  - test: "关闭码文案分派：子进程 exit 显示 Session ended；伪造 wesh.v9 Hello 显示 Connection refused（version_mismatch）；不发 Hello 等 5s 被 1008 关闭"
    expected: "onclose 按码分派人话文案生效"
    why_human: "onclose 文案面板是视觉/交互行为"

  - test: "单客户端：另开第二个标签访问同地址"
    expected: "显示 Unable to connect（409 语义不变）"
    why_human: "多标签浏览器交互需人工确认"

  - test: "【人工决策·非测试】CR-01 修复时机：Attach 读循环同步写 PTY master 在特定条件下永久阻塞"
    expected: "决定立即做最小缓解（master fd 置 O_NONBLOCK + ErrWouldBlock 走既有收口），或将完整的『有界输入队列 + 独立写 goroutine』背压方案并入 Phase 5"
    why_human: "这是修复范围/时机的工程权衡决策，非自动化可判定；详见『Code Review 发现评估』节"
---

# Phase 2: 协议基线 Verification Report

**Phase Goal:** WS 协议层一次性到位——版本化、类型化错误帧、三层资源上限、合规关闭码，预认证攻击面在结构上消除
**Verified:** 2026-08-15T12:49:47Z
**Status:** human_needed
**Re-verification:** No — initial verification

**验证方法说明（goal-backward）：** 本报告不信 SUMMARY 自述，全部结论以代码实读 + `go test -race -count=1 ./...` 实跑（4 包全绿）+ 逐测试组实跑为据。三条 ROADMAP 成功准则逐条对到可观测代码证据。

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 版本化子协议 + 类型化握手/错误帧（wesh.v1 契约） | ✓ VERIFIED | `proto.go` Subprotocol/Hello/Welcome/Error/ErrorFrame/WelcomeFrame；`TestProtocolConstants`/`TestDecodeHello`/`TestWelcomeFrameErrorFrame` PASS；前端 `SUBPROTOCOL='wesh.v1'` 对齐 |
| 2 | 准则1a：超限帧（>16KiB 稳态）以 1009 合规关闭 | ✓ VERIFIED | `TestOversize1009`/`TestReadLimitBoundary`（16384 通过 + 16385→1009）PASS；`SetReadLimit(ReadLimitPostAuth)` server.go:321 |
| 3 | 准则1b：1 字节 × N 分片洪水在累积 16385 字节处 1009 | ✓ VERIFIED | `TestFragmentedFlood1009` PASS（库 limitReader 流式截断，无应用层计数器） |
| 4 | 准则1c：0 字节空帧洪水下服务存活、内存平坦 | ✓ VERIFIED | `TestEmptyFragmentFloodResilience` PASS（存活 + echo 正常 + HeapAlloc 增量 <8MiB 宽松防线） |
| 5 | 准则1d：预认证窗口 >4KiB 消息以 1009 关闭 | ✓ VERIFIED | `TestPreHelloReadLimit` PASS；`SetReadLimit(ReadLimitPreAuth)` server.go:259 |
| 6 | 准则2a：默认只读服务端丢 INPUT、RESIZE 放行（服务端真边界） | ✓ VERIFIED | server.go:349-357 `if !s.writable { continue }` + RESIZE 放行；`TestReadOnlyDropsInput`/`TestReadOnlyAllowsResize` PASS |
| 7 | 准则2b：`--writable` 开启后输入生效 | ✓ VERIFIED | main.go:41 flag + server.go Options.Writable；`TestHelloWelcome`(rw 半侧)/`TestEchoPTY` PASS |
| 8 | 准则2c：线上关闭码合规、1006 永不发送、无自定义 4000 段 | ✓ VERIFIED | wire 关闭码扫描 = {1000,1002,1008,1009}；CloseNow 不发关闭帧（1006 仅客户端本地合成）；无 4000 段私码 |
| 9 | 准则3：WS ping/pong 按可配间隔保活、反代空闲超时下连接不被切断 | ✓ VERIFIED | `pinger`（server.go:398）+ `--ping-interval`（默认 5s，0 禁用）；`TestPingKeepalive`/`TestPongTimeout`/`TestPingDisabled` PASS |
| 10 | 预认证攻击面结构消除（SEC-08：子协议400/per-IP429/409/5s超时/零缓冲） | ✓ VERIFIED | 守卫区顺序 server.go:202-247（400→429→409→Accept→assert）；`TestSubprotocolRequired`/`TestHalfOpenPerIP429`/`TestHelloTimeout` PASS；预认证路径无消息级缓冲预分配 |

**Score:** 10/10 truths verified

### 关于准则 2 关闭码集合的措辞校准（非缺口）

ROADMAP 准则 2 字面写「线上关闭码只出现在 1000/1008/1009/1011/1013 集合内」，但**实际 wire 关闭码为 {1000,1002,1008,1009}**——含 1002（协议错误）。这是 ROADMAP 措辞遗漏，非代码缺陷：权威契约 D-05 与 README 关闭码全集表均含 1002（README L69「1002 协议错误（未知帧/抢跑/畸形）」），且 1002 是 RFC 合规的协议错误码，仅攻击/畸形流量触发，正常浏览器客户端不可达。**判定：服务端关闭码行为正确且合规；ROADMAP 该处枚举不全属文档措辞不精确，不构成缺口。**

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/proto/proto.go` | wesh.v1 契约单一事实源 | ✓ VERIFIED | 143 行；Subprotocol/Hello/Welcome/Error/ModeRO/ModeRW/ErrVersionMismatch/ErrServerError/ReadLimitPreAuth=4096/ReadLimitPostAuth=16384 + DecodeHello/WelcomeFrame/ErrorFrame 全在 |
| `internal/proto/proto_test.go` | 编解码往返 + 常量锁定 | ✓ VERIFIED | 134 行；3 测试函数 PASS |
| `internal/server/server.go` | 握手段 + ro 门 + 守卫链 + pinger + 超限钩子 | ✓ VERIFIED | 499 行；全部要素实读在码 |
| `cmd/wesh/main.go` | --writable/--ping-interval + ReadHeaderTimeout | ✓ VERIFIED | flag L41-42；`http.Server{ReadHeaderTimeout:5s}` L89 |
| `web/src/main.ts` | 子协议建连 + Hello 首帧 + onclose 分派 | ✓ VERIFIED | SUBPROTOCOL/HELLO/WELCOME/ERROR/helloSent 门/onclose per-code 全在；tsc+vite 构建绿 |
| `internal/server/handshake_test.go` | 守卫链七测 | ✓ VERIFIED | 424 行；7 测试函数 PASS |
| `internal/server/keepalive_test.go` | 保活三测 | ✓ VERIFIED | 135 行；3 测试函数 PASS |
| `internal/server/limits_test.go` | RES-01 五测 | ✓ VERIFIED | 283 行；5 测试函数 PASS |
| `internal/pty/io.go`/`spawn.go` | fd 竞态修复（fdMu） | ✓ VERIFIED | fdMu+closed 幂等；`TestResizeAfterClose` 回归锁 PASS |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `server.go` | `proto.go` | DecodeHello/WelcomeFrame/ErrorFrame/Subprotocol/ReadLimit* | ✓ WIRED | grep 11 处 proto 符号引用 |
| `main.go` | `server.go` | `server.New(sess, os.Exit, Options{Writable, PingInterval})` | ✓ WIRED | main.go:83 |
| `main.ts` | `proto.go` | SUBPROTOCOL/帧常量手工对齐（D-16） | ✓ WIRED | 常量值逐字一致 |
| `limits_test.go` | `proto.go` | 边界断言引用 proto.ReadLimit 常量 | ✓ WIRED | 5 处引用，非硬编码数字 |
| `server.go` | `coder/websocket` | SetReadLimit/Ping/CloseNow/Accept | ✓ WIRED | 库调用实读在码 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| 握手段 | Hello cols/rows | `c.Read` 实时客户端帧 | 是 | ✓ FLOWING |
| onChunk 数据泵 | OUTPUT 帧载荷 | `sess.Master.Read`（ReadLoop） | 是 | ✓ FLOWING |
| Welcome mode | `s.writable` | main.go `--writable` flag | 是 | ✓ FLOWING |
| pinger 间隔 | `s.pingInterval` | main.go `--ping-interval` flag | 是 | ✓ FLOWING |

无静态返回、无硬编码空数据、无 mock 充数。

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 全量测试（-race） | `go test -race -count=1 ./...` | 4 包全 ok（5.5s） | ✓ PASS |
| go vet | `go vet ./...` | exit 0 | ✓ PASS |
| gofmt（GOROOT） | `gofmt -l .` | 输出为空 | ✓ PASS |
| 前端构建 | `pnpm -C web build` | tsc+vite+gzip 绿（273ms） | ✓ PASS |
| RES-01 五测 | `-run TestOversize1009\|...` | 5/5 PASS | ✓ PASS |
| 守卫/ro 八测 | `-run TestHelloWelcome\|...` | 8/8 PASS | ✓ PASS |
| 保活三测 | `-run TestPingKeepalive\|...` | 3/3 PASS | ✓ PASS |

### Probe Execution

无 `scripts/**/probe-*.sh` 声明或约定探针；本 phase 的量化断言面由 limits_test.go 五测承载（已实跑全绿）。SKIPPED（无探针入口）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CORE-04 | 02-02/02-03/02-06 | 默认只读，显式可写才接受输入 | ✓ SATISFIED | ro 门 server.go:349；`--writable`；TestReadOnlyDropsInput/TestHelloWelcome PASS |
| CORE-06 | 02-04/02-06 | WS ping/pong 保活，间隔可配 | ✓ SATISFIED | pinger + `--ping-interval`；保活三测 PASS |
| SEC-08 | 02-01/02-02/02-03/02-05 | 认证前零缓冲分配/预认证攻击面 | ✓ SATISFIED | 守卫区四闸 + 4KiB 档 + 5s 超时；TestSubprotocolRequired/TestHalfOpenPerIP429/TestPreHelloReadLimit PASS；预认证路径无消息级缓冲预分配 |
| RES-01 | 02-01/02-02/02-05/02-06 | WS 消息三层上限 | ✓ SATISFIED | 两层字节硬顶（SetReadLimit 库执行）+ 分片层等效防线；limits 五测 PASS |

REQUIREMENTS.md traceability 表（L116-142）将四条全部映射到 Phase 2 且标 Complete——与代码证据一致。四条需求 ID 全部有落点，无 ORPHANED。

---

## Code Review 发现评估（goal-backward 判定）

用户要求评估 02-REVIEW.md 的 1 BLOCKER + 2 WARNING 是否构成对 phase goal / success criteria 的**实质性缺口**。三条发现均已对码实证（非复述评审结论）。**核心判定：三条都是真实代码缺陷，但都不是 Phase 2 协议层目标的结构性缺口——协议契约（线格式/关闭码/上限/握手）全部验证通过，且三条的修复都是『协议透明』的纯服务端实现改动，不需要返工协议层。** 这正对应 phase goal 的「协议层一次性到位」——协议本身到位了，缺陷在传输实现层。

### CR-01（评审 BLOCKER）→ 判定：WARNING，非 phase-goal 缺口，但需人工决策修复时机

**对码实证（确认缺陷真实存在）：** server.go:352 `s.sess.Master.Write(data[1:])` 是单线程读循环内的**同步阻塞写**；`Master` 为阻塞态 `*os.File`（`pty.StartWithSize` 返回，全仓 grep 无 `O_NONBLOCK`/`SetWriteDeadline`/输入队列/独立写 goroutine）。子进程置 raw 模式且停读 stdin 时，n_tty 缓冲填满后该 Write 永久阻塞。

**两个后果（评审所述，机制成立）：**

1. 读循环卡 Write → `c.Read` 停摆 → 库只在读路径处理 pong（read.go:317-337）→ pinger 在 pongTimeout(10s) 后 `CloseNow` 误杀一条健康连接。
2. 读循环卡 Write → 客户端断开无法唤醒 → `wsDisconnected`/`terminate`/`exitf` 不触发 → D-11 单次语义退出保证失效，服务端挂死。

**为何判定为『非 phase-goal 缺口』（goal-backward 推理）：**

- **不触三条成功准则任一条：** 准则1（READ 上限）与写路径无关；准则2（ro 默认）在 CR-01 触发前提（`--writable`）之外——默认 ro 模式下 INPUT 在 server.go:349 即被 `continue` 丢弃，**永远到不了 Write**；准则3（保活）的字面场景是『反代空闲超时』——空闲连接无输入、读循环不卡 Write、pong 正常处理，CR-01 不破坏该场景。
- **协议透明：** CR-01 的修复（有界输入队列 + 写 goroutine，或最小 O_NONBLOCK）是纯服务端数据面实现改动，**不改任何线格式/关闭码/上限/握手**——协议层无需返工，「一次性到位」成立。
- **非 Phase 2 引入的回归：** 同步 Write 模式自 Phase 1 即存在；Phase 2 的 pinger 只是给这个潜伏缺陷新增了『误杀健康连接』这一后果，并未引入阻塞本身。
- **触发条件窄：** 需同时满足 `--writable`（非默认）+ raw 模式子进程 + 子进程停读 stdin + 客户端灌入 ≥16KiB 输入。主流路径（ro 默认；writable + 正常读 stdin 的 TUI 如 vim/htop）不触发。

**但它是真实的 blocker 级可靠性缺陷**（破坏 D-11 退出保证 + 可误杀健康连接），必须修复。评审给的完整修法（有界输入队列 + 独立写 goroutine，满时背压）与 Phase 5 的 1013 踢出对齐；最小缓解（O_NONBLOCK + ErrWouldBlock 走既有收口）可现在做。**这是修复范围/时机的工程权衡，归入 human_verification 由人决策**——不宜由验证器单方面裁定阻塞整个 phase（协议目标已达成），也不应静默放过。

### WR-01（评审 WARNING）→ 判定：WARNING，非 phase-goal 缺口，defer Phase 5

**对码实证：** server.go:374 `onChunk` 的 `c.Write(context.Background(), ...)` 无超时。对一个『TCP 存活、回 pong、但应用层永不读数据帧』的客户端，服务端/客户端内核缓冲填满后 ReadLoop 永久停滞于 onChunk，PTY drain 与 Drain(200ms) 时限承诺被击穿。

**判定：** 这是 S→C 背压问题，ROADMAP Phase 5 明确承载（「每客户端有界 outbox」「全体可写客户端阻塞时停读 PTY 的全局信用」、1013 背压踢出 D-08 占位）。评审自述「1013 背压踢出虽属 Phase 5」。与准则1/2/3 无涉。**defer Phase 5，非本 phase 缺口。**

### WR-02（评审 WARNING）→ 判定：WARNING（轻微一致性缺陷），非 phase-goal 缺口

**对码实证：** server.go:360 `default:` 分支关闭 reason 为 `"unknown frame type"`（带空格英文句），违反 D-07 机器串纪律（其余路径全为 snake_case：`empty_frame`/`frame_before_hello`/`malformed_hello`/`hello_timeout`/`version_mismatch`/`subprotocol_required`）；且该路径是所有违规路径中**唯一不打 logEvent stderr 事件**的。

**判定：** 关闭**码**（1002）合规，仅 reason **字符串格式**与一行 stderr 日志缺失。不触任何成功准则（准则2 约束的是码值集合与 1006，reason 格式不在其列）。属一致性/可观测性小修，可随时做（一行 logEvent + reason 改 `unknown_frame`）。**非缺口。**

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/pty/reap_darwin.go` | 72 | `TODO(Phase 8): 接 slog` | ℹ️ Info | Phase 1 文件，引用正式后续工作（Phase 8 OPS-08），非本 phase 产物，非阻塞 |

无 TBD/FIXME/XXX；无 placeholder/空实现；无 stub（全部测试为真实 e2e：起真实 PTY + WS 断言行为）。

## Human Verification Required

见 frontmatter `human_verification`（6 项：5 项浏览器 UAT + 1 项 CR-01 修复时机人工决策）。

---

## 总结

**Phase 2 协议基线目标达成。** 三条 ROADMAP 成功准则全部有 `-race` 自动化证据：超限/分片/空帧洪水的 1009 与存活（准则1）、默认只读服务端边界与合规关闭码（准则2）、ping/pong 保活三态（准则3）。四条需求（CORE-04/CORE-06/SEC-08/RES-01）全部 SATISFIED。预认证攻击面经守卫区四闸 + 两档读上限 + 5s 超时 + 零缓冲在结构上收敛。

**三条 code review 发现均为真实缺陷，但均不构成协议层目标的结构性缺口**（协议契约验证通过、修复协议透明）。其中 CR-01 是 blocker 级可靠性缺陷（破坏 D-11 + 可误杀健康连接），需人工决策修复时机（立即最小缓解 vs 完整方案并入 Phase 5）；WR-01 defer Phase 5；WR-02 为可随时做的一致性小修。

状态为 **human_needed**：自动化验证全绿、目标达成，但有 5 项浏览器 UAT 与 1 项 CR-01 修复决策需人工确认，phase 不宜直接标 complete。

---

_Verified: 2026-08-15T12:49:47Z_
_Verifier: Claude (gsd-verifier)_
