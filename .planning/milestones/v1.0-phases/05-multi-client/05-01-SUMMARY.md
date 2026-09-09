---
phase: 05-multi-client
plan: 01
subsystem: api
tags: [go, websocket, fan-out, hub, outbox, registry, backpressure, pty, multi-client]

# Dependency graph
requires:
  - phase: 04-frontend
    provides: 单客户端服务端（409 原子门/attached+conn 字段/onChunk 直写/单次语义生命周期）
provides:
  - clients.go：client/outbox/registry/hub onChunk/writer/kickSlowConsumerLocked/detach 全套多客户端拓扑
  - server.go 多客户端化：注册表登记升档、Welcome 经 outbox 首条入队、lifecycle 广播 1000 + exitf、per-client INPUT 门
  - Options 五测试可覆写字段（OutboxBytes/MaxClients/InputRate/InputBurst/ResizeDebounce）+ 五默认常量（Phase 9 标定回填）
  - proto.go 1013 关闭码启用（websocket.StatusTryAgainLater，reason=slow_consumer）
  - e2e 套件多客户端语义全绿 + multi_test.go 三测试（VALIDATION 05-W0-01/05-01-01）
affects: [05-02 信用门+SIGWINCH, 05-03 权限体系, 05-04 resize 仲裁, 05-05 限速+CR-01, 05-06 分享链接, 05-07 max-clients, Phase 8 OPS-07]

# Actuals (#2632)
actuals:
  tokens: 19902
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "fan-out hub：ReadLoop 单读 → hubMu 内组一次共享只读帧 → 逐客户端 outbox 非阻塞 trySend（P5-1 别名红线）"
    - "outbox 字节有界队列 + cap 1 信号 channel：trySend 满即 1013 踢出，绝不丢帧保连接"
    - "踢出绝不内联：注册表同步移除 + cancel + go c.Close(1013)（P5-2，close.go 幂等自界）"
    - "writer 同类型连续段合并写出：合并成单帧 = 类型字节一次 + 载荷拼接（ARCHITECTURE §2.5），1 WS 消息 = 1 帧不变"
    - "断开收口 detach：注册表移除 + close(done) + cancel，不进 exitf（P1 D-11 单次语义终结）"
    - "startTrackedServerWith：WaitGroup happens-before 边替代消亡的 waitExit channel 同步（stderr 捕获类测试专用）"

key-files:
  created:
    - internal/server/clients.go
    - internal/server/multi_test.go
  modified:
    - internal/server/server.go
    - internal/proto/proto.go
    - internal/server/e2e_test.go
    - internal/server/handshake_test.go
    - internal/server/limits_test.go
    - internal/server/keepalive_test.go
    - internal/server/auth_e2e_test.go

key-decisions:
  - "[Phase 05-01]: writer 合并形态取 ARCHITECTURE §2.5『合并成单帧』本意——同类型连续段合并（类型字节一次 + 载荷拼接），1 WS 消息 = 1 帧线上纪律不变、前端零改动；plan 字面 bytes.Join(batch) 会把内嵌类型字节写进终端流（前端 buf[0] 单帧分派被破坏，TestReadLimitBoundary 实测溢出 3 字节）"
  - "[Phase 05-01]: 五默认常量声明落 clients.go（hub/outbox/限速均属多客户端关切面），server.go New 零值兜底逐字段引用——同时满足 plan 验收 grep server.go == 5 与 HelloTimeout 先例形态"
  - "[Phase 05-01]: stderr 捕获类测试（TestOversize1009/TestLogRedaction）改用 startTrackedServerWith——waitExit 消亡后 captureStderr restore() 与 handler 内 logEvent 读 os.Stderr 无同步边（-race 实测命中），WaitGroup happens-before 是 race detector 认可的替代"

patterns-established:
  - "多客户端生命周期断言形态：断开 → assertNoExit(200ms 静默反证) + 同实例再 attach 存活断言；子进程退出 → 广播 1000 + waitExit(code) 保留"
  - "读循环违规路径 cl==nil 判空收口（INPUT 门 cl == nil || cl.mode == proto.ModeRO 静默丢）——握手违规落入读循环时空指针防线"
  - "registry 成员判定互斥：removeLocked 返回 bool，kick 与 detach 的 close(done)/cancel 恰好一次"

requirements-completed: [MULTI-01]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "双客户端 attach 同一服务端各自收到 Welcome，且实时收到同一 OUTPUT 字节流（两端累积 payload 逐字节一致）"
    requirement: MULTI-01
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestMultiClientFanout"
        status: pass
      - kind: integration
        ref: "internal/server/e2e_test.go#TestSecondClientAttach"
        status: pass
    human_judgment: false
  - id: D2
    description: "任何客户端断开不再触发 exitf/SIGHUP：服务端进程存活、会话仍可 echo、注册表移除该客户端（断开者可立即再 attach）"
    requirement: MULTI-01
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestDetach"
        status: pass
    human_judgment: false
  - id: D3
    description: "子进程退出是唯一终结路径：lifecycle 广播 1000 关闭全部客户端，exitf 以子进程退出码收口"
    requirement: MULTI-01
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestExitBroadcast"
        status: pass
      - kind: integration
        ref: "internal/server/e2e_test.go#TestExitCodePropagation"
        status: pass
    human_judgment: false
  - id: D4
    description: "hub 每 chunk 组一次共享只读帧 + trySend 满即 1013 踢出（go c.Close 异步，绝不内联）；e2e 套件多客户端语义迁移全绿"
    requirement: MULTI-01
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./internal/server/ ./internal/proto/ ./internal/pty/（全绿；-race 5 连跑零竞态）"
        status: pass
    human_judgment: false

# Metrics
duration: 1h 0m
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 01: 多客户端 fan-out 主干 Summary

**单客户端服务端升级为多客户端 fan-out：注册表替代 409 原子门，hub 共享帧扇出 + 每客户端有界 outbox + 专属 writer，断开=注册表移除不退出、子进程退出=广播 1000+exitf，e2e 套件单次语义全量迁移 -race 绿。**

## Performance

- **Duration:** 1h 0m
- **Started:** 2026-08-20T07:06:20Z
- **Completed:** 2026-08-20T08:06:37Z
- **Tasks:** 2
- **Files modified:** 9（2 新建 + 7 修改）

## Accomplishments

- MULTI-01 主干打通：双客户端 attach 同一服务端各自收 Welcome 并实时收到逐字节一致的 OUTPUT 流（TestMultiClientFanout 异尺寸 80x24/132x43 + TestSecondClientAttach 双证）
- 生命周期改造落地：任何客户端断开不再触发 exitf/SIGHUP（TestDetach：静默反证 + B 存活 + 再 attach）；子进程退出唯一终结路径（TestExitBroadcast：双端 1000 + exitf(3)）
- hub 关键纪律全部落位：每 chunk 组一次共享只读帧（P5-1）、outbox 字节有界 512KiB 默认、trySend 满即 1013 slow_consumer 踢出且 go c.Close 异步绝不内联（P5-2）、Welcome 经 outbox 首条入队（writer 全程唯一写端）
- e2e 套件 5 个测试文件单次语义全量迁移，server+proto+pty 三包 -race 全绿且 5 连跑零竞态——Wave 2 一切工作的地基

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): clients.go 注册表/hub/writer + server.go 多客户端化主干改造** - `cc03c79` (feat)
2. **Task 2: e2e_test.go 单次语义迁移 + multi_test.go 三测试** - `202f012` (test)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `internal/server/clients.go`（新）- client 结构（conn/remote/mode/attachSeq/outbox/done/cancel）、outbox（字节有界 + cap 1 信号 channel，trySend/drain）、registry（set map + FIFO order + seq + kicks 计数挂点——Phase 8 OPS-07）、hub onChunk（共享帧扇出）、writer（同类型连续段合并写出）、kickSlowConsumerLocked（1013 slow_consumer 异步踢出）、detach
- `internal/server/multi_test.go`（新）- TestMultiClientFanout / TestDetach / TestExitBroadcast（VALIDATION 05-W0-01/05-01-01）
- `internal/server/server.go` - attached/conn/frame 三字段与 409 原子门退役（③位留 05-07 503 闸占位注释）；Server 加 hubMu/registry/五参数字段；Options 加五测试可覆写字段；New 零值兜底；升档走注册表登记 + Welcome 经 outbox；INPUT 门 per-client c.mode 判定（含 cl==nil 违规路径判空）；lifecycle 广播 1000 并行 Close + WaitGroup；wsDisconnected/childExited/SIGHUP 路径消亡；terminate 收口单调用方
- `internal/proto/proto.go` - 1013 注释启用（发送路径 = clients.go kickSlowConsumerLocked）；分片防线注释中 409 门表述更新为已拆除（05-07 重建）
- `internal/server/e2e_test.go` - assertNoExit helper（200ms 静默反证）；startTrackedServerWith（WaitGroup 同步边）；TestEchoPTY/TestUnknownFrame1002/TestHelloWelcome/TestWelcomePrefs 断开断言改写；TestSecondClient409 → TestSecondClientAttach 整测替换；TestClientDisconnectSIGHUP + wesh-helper-sighup 分支删除；TestExitCodePropagation 注释改广播形态
- `internal/server/handshake_test.go` - 6 处 waitExit(0) → assertNoExit（TestReadOnlyAllowsResize 子进程退出走 D-10 保留不变）
- `internal/server/limits_test.go` - 5 处 waitExit(0) → assertNoExit；captureStderr 时序注释更新；TestOversize1009 改 startTrackedServerWith + waitHandlers 同步边
- `internal/server/keepalive_test.go` - 3 处 waitExit(0) → assertNoExit
- `internal/server/auth_e2e_test.go` - 6 处 waitExit(0) → assertNoExit；TestLogRedaction 改 startTrackedServerWith + waitHandlers 同步边

## Decisions Made

- **writer 合并形态（Rule 1 修正后的定稿）**：同类型连续段合并——合并成单帧 = 类型字节一次 + 载荷顺序拼接（ARCHITECTURE §2.5「批量 drain 合并成单帧，减少帧数与小包」的本意）；1 WS 消息 = 1 帧线上纪律不变（前端 buf[0] 单帧分派零改动，must_have「扇出对前端透明」成立），合并后 OUTPUT 字节流与逐帧接收完全相同。单帧零拷贝直写；合并时先拷贝首帧防共享帧原地 append（P5-1）。
- **五默认常量落点**：声明在 clients.go（defaultOutboxBytes=512KiB/defaultMaxClients=32/defaultInputRate=32KiB/defaultInputBurst=64KiB/defaultResizeDebounce=50ms，全部挂 Phase 9 标定回填注释），server.go New 零值兜底逐字段引用——plan 验收 `grep -c ... server.go == 5` 与 HelloTimeout 先例形态同时满足。
- **Task 1 中间态红测处置**：plan action note 3 授权「本任务提交后 e2e 旧生命周期断言预期变红，Task 2 同 plan 迁移收口」——Task 1 提交时 TestHelloWelcome/TestVersionMismatch 红为预期中间态（验证确认仅生命周期断言失败、握手主链路无恙），tracer 切片本身由 Task 2 三测试端到端证明后绿。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] writer 合并修正：bytes.Join(batch) → 同类型连续段合并**
- **Found during:** Task 2（TestReadLimitBoundary 实测 echo overflow: got 16406, want <= 16403——3 个内嵌 0x30 类型字节混入回显流）
- **Issue:** plan 字面「drain 批量合并单帧写出（bytes.Join 或自拼）」把多个完整帧（各含类型字节）拼接为一个 WS 消息——接收端按 buf[0] 单帧分派时内嵌类型字节被当作终端数据写出（前端 main.ts:380-384 会被破坏：'0' 字符写入 xterm 画面静默损坏；Welcome+OUTPUT 合帧则 JSON 解析失败 Welcome 丢失）；违反 must_have「扇出对前端透明，前端零改动」
- **Fix:** 按 ARCHITECTURE §2.5「合并成单帧」本意实现同类型连续段合并：类型字节一次 + 载荷顺序拼接；单帧零拷贝、合并先拷首帧防共享帧原地 append
- **Files modified:** internal/server/clients.go
- **Verification:** TestReadLimitBoundary 通过；三包 -race 全绿；5 连跑零失败
- **Committed in:** 202f012（Task 2 提交；clients.go 随该提交修正）

**2. [Rule 3 - Blocking] P5-4 迁移面扩展：handshake/limits/keepalive/auth_e2e 四文件 20 处断开断言同步迁移**
- **Found during:** Task 2（迁移面勘察：grep waitExit 发现 e2e_test.go 之外四个文件同样建在「断开→exitf(0)」单次语义上）
- **Issue:** plan Task 2 <files> 仅列 e2e_test.go + multi_test.go，但 handshake_test.go(6)/limits_test.go(5)/keepalive_test.go(3)/auth_e2e_test.go(6) 的断开断言 Task 1 后全红——不迁移则 plan success criteria「三包 -race 全绿」结构性不可达（P5-4 逐文件核实只覆盖 e2e_test.go 的漏估）
- **Fix:** 同语义迁移——断开→exitf(0) 断言一律改 assertNoExit(200ms 静默反证)，紧邻注释同步更新；子进程退出路径（TestReadOnlyAllowsResize/TestExitCodePropagation）保留 waitExit 不变；两处 409/SIGHUP 陈旧注释随段清理
- **Files modified:** internal/server/handshake_test.go, internal/server/limits_test.go, internal/server/keepalive_test.go, internal/server/auth_e2e_test.go
- **Verification:** 全量 -race 绿；grep 确认断开 waitExit(0) 零残留（仅余子进程退出两处合法形态）
- **Committed in:** 202f012

**3. [Rule 1 - Bug] stderr 捕获竞态修复：startTrackedServerWith 的 WaitGroup 同步边**
- **Found during:** Task 2（全量 -race 实测命中 1 次：captureStderr.func1 写 os.Stderr 与既往 Attach handler goroutine 内 logEvent 读 os.Stderr 并发）
- **Issue:** 旧代码靠「logEvent 与 exitf 同 goroutine 程序序 + waitExit channel happens-before」保证 restore() 时 stderr 行已落盘；多客户端推论后 exitf 不再随断开触发，该同步边消亡——restore() 与 handler 内 logEvent 对 os.Stderr 变量的访问无 happens-before（时序敏感，-race 约每 4-8 全量跑命中 1 次，CI 必现）
- **Fix:** 新增 startTrackedServerWith（handler 追踪变体：http.Handler 包装 wg.Add/Done，返回 wg.Wait）；TestOversize1009 与 TestLogRedaction 改用之，restore() 前 waitHandlers() 建立 WaitGroup happens-before 边（logEvent 读必然先于 handler 返回）
- **Files modified:** internal/server/e2e_test.go, internal/server/limits_test.go, internal/server/auth_e2e_test.go
- **Verification:** 修复后 -race 5 连跑零竞态零失败
- **Committed in:** 202f012

---

**Total deviations:** 3 auto-fixed（2 Rule 1 - Bug，1 Rule 3 - Blocking）
**Impact on plan:** 三处修正全部服务于 plan 自身验收标准与协议正确性（前端透明/三包全绿/CI 稳定），无 scope creep；writer 合并语义经 ARCHITECTURE §2.5 原文复核为 plan 引用文献的本意。

## Issues Encountered

- **握手违规路径 cl==nil 空指针防线（实现期自查发现）**：读循环 INPUT case 改 per-client `cl.mode` 判定后，握手违规落入读循环的路径（cl 未赋值）若客户端管道化连发 INPUT 帧会解引用空指针——改为 `cl == nil || cl.mode == proto.ModeRO` 静默丢（旧代码 s.writable 恒可读故无此面；多客户端化的必然防線，go vet/测试无法静态覆盖的运行期路径）。
- **TestHelperProcess args 变量随 sighup 分支删除未用**：go vet 报 declared and not used——清理 marker 解析逻辑（分支唯一使用点已删）。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的占位与后续挂点）

- **③位 503 闸占位注释**（server.go 守卫区）：max-clients 闸由 05-07 落地（含 attachHandler 双点位与 TestMaxClients503）
- **四参数仅立字段与常量**：maxClients/inputRate/inputBurst/resizeDebounce 消费点分别在 05-07/05-05/05-04；outboxBytes 本 plan 已消费
- **registry.kicks 计数器**：观测性 stub（review #10），Phase 8 OPS-07 进 metrics
- **detach 无递补/仲裁重算**：owner 递补 05-03、resize 仲裁 05-04 落地；本 plan RESIZE 为 last-wins 延续
- **05-02 信用门不变量已在代码注释锁定**：门 Wait 循环必须插入 hub 组帧语句之前（clients.go onChunk 注释，review #1 别名安全）

## Next Phase Readiness

- 注册表/hub/writer 拓扑与 detach/广播生命周期是多客户端 phase 全部后续 plan 的地基——已 proofed by TestMultiClientFanout/TestDetach/TestExitBroadcast
- 05-02 可直接在 hub 组帧语句前插入信用门 Wait（锁序 hubMu > outboxMu 已立）；slowclient_test.go 夹具形态（Options.OutboxBytes 覆写加速触发）已就绪
- 无阻塞项；全量 -race 绿，CI 形态与 Phase 4 等价

## Self-Check: PASSED

- FOUND: internal/server/clients.go（func (s *Server) onChunk 落位，grep == 1）
- FOUND: internal/server/multi_test.go（TestMultiClientFanout/TestDetach/TestExitBroadcast，grep == 3）
- FOUND: commit cc03c79（Task 1）、202f012（Task 2）均在 git log
- 验收 grep 全过：atomic 零残留（server.go 非注释行 == 0）、slow_consumer >= 2、kicks >= 2、go func() >= 1、五默认常量 server.go == 5、SIGHUP e2e 零残留、TestSecondClient409 零残留
- 全量 go test -race -count=1 ./internal/server/ ./internal/proto/ ./internal/pty/ 绿；-race 5 连跑零竞态

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
