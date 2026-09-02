---
phase: 02-protocol
plan: 04
subsystem: api
tags: [websocket, keepalive, ping-pong, core-06, go, coder-websocket, reverse-proxy-idle-timeout]

requires:
  - phase: 02-protocol
    provides: 02-02 握手升档序列与 conn 延迟上线、02-03 Options 同构注入结构与 logEvent 埋点模式
provides:
  - pinger goroutine（ticker + Ping + pong 超时 logEvent/CloseNow）挂 Welcome 升档序列尾段，与单 reader 并发装配
  - Options.PingInterval（生产直传，0=禁用）/ Options.PongTimeout（测试可覆写，零值默认 defaultPongTimeout=10s）
  - --ping-interval CLI flag（DurationVar，默认 5s，0 禁用，D-16）与 config.pingInterval 全链路透传
  - Attach ctx 改 WithCancel + defer cancel——pinger 终结进既有单一生收口，零新 exitf 分支
  - keepalive_test.go 保活三测：TestPingKeepalive / TestPongTimeout / TestPingDisabled
affects: [02-05 上限 e2e（logEvent 埋点清单复核）, 02-06 期末 UAT, Phase 9 反代配方文档]

actuals:
  tokens: 3407
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "pinger 与单 reader 并发装配：Ping 必须与 Reader 并发（conn.go:218-220 库硬性要求），pong 由读循环 handleControl 处理（read.go:317-337），不得为 ping 再开 reader"
    - "Ping 错误路径精确分类：仅 errors.Is(err, context.DeadlineExceeded) 才 pong_timeout + CloseNow——父 ctx 为 WithCancel 无 deadline，DeadlineExceeded 唯一来源即 pctx 到期；其余错误（对端关闭/写失败/cancel 级联）静默返回"
    - "goroutine 终结挂点形态：Attach 内 context.WithCancel 派生 + defer cancel——pinger 随 handler 返回终结，进既有 wsDisconnected→terminate 单一收口，零新 exitf 分支"

key-files:
  created:
    - internal/server/keepalive_test.go
  modified:
    - internal/server/server.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go

key-decisions:
  - "pinger 签名增 remote 参数（plan 字面三参 ctx/c/interval 与 action 文本 logEvent 三要素的调和）：pong_timeout 事件需要 Attach 入口保存的 r.RemoteAddr，签名扩为 (ctx, c, remote, interval)"
  - "Task 1 tdd=true 与 keepalive_test.go 归属 Task 2 的张力按 02-03 先例裁决：测试文件随 RED 先行创建（含 TestPingKeepalive + TestParseArgs ping 组，编译期失败为正确失败原因），Task 2 扩展其余两测"
  - "pinger 错误路径收窄为仅 DeadlineExceeded 才打事件+CloseNow：修复正常终结竞态下的误报 pong_timeout（短 interval 下几乎必现），详见偏差 1"

patterns-established:
  - "负例测试的防自证形态：停止 Read 窗口用 sleep 等待（窗内任何 Read 都会回 pong 破坏负例），断开观察用独立 2s 护栏 ctx + errors.Is(DeadlineExceeded) 排除护栏到期误通过"
  - "禁用语义强断言形态：存活证据反证未发 ping（PongTimeout 陷阱参数——误启用必触发断开，echo 必失败）"

requirements-completed: [CORE-06]

coverage:
  - id: D1
    description: "pinger 保活默认工作：PingInterval=100ms 注入下空闲 350ms（>3 间隔）连接存活 + INPUT echo 功能完好（Pitfall 2/3 回归锁）；--ping-interval 默认 5s 全链路（flag → config → Options → goroutine）"
    requirement: CORE-06
    verification:
      - kind: e2e
        ref: "internal/server/keepalive_test.go#TestPingKeepalive"
        status: pass
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs（ping interval 30s 组 + ping disabled 0 组 + 既有四组默认 5s）"
        status: pass
      - kind: e2e
        ref: "go test -race -count=3 -run 'TestPingKeepalive|TestPongTimeout|TestPingDisabled' ./internal/server/（三连跑无 race 无 flake）"
        status: pass
    human_judgment: false
  - id: D2
    description: "pong 超时主动清理：客户端停 Read 不回 pong，服务端在 interval+timeout 内打 stderr 单行事件（pong_timeout，三要素）并 CloseNow；reader 终结走 D-11 既有收口 exitf(0)，不泄漏半死连接"
    requirement: CORE-06
    verification:
      - kind: e2e
        ref: "internal/server/keepalive_test.go#TestPongTimeout（运行日志可见 pong_timeout 事件仅在此测出现）"
        status: pass
    human_judgment: false
  - id: D3
    description: "--ping-interval 0 禁用语义：PingInterval=0 + PongTimeout=100ms 陷阱参数下客户端停 Read 500ms 后 echo 存活，反证未发任何 ping"
    requirement: CORE-06
    verification:
      - kind: e2e
        ref: "internal/server/keepalive_test.go#TestPingDisabled"
        status: pass
    human_judgment: false
  - id: D4
    description: "pinger 装配纪律：启动点在 Welcome 升档序列尾段、与单 reader 并发、写并发安全（writeFrameMu）、终结挂 WithCancel defer cancel 零新 exitf 分支"
    requirement: CORE-06
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... + 静态走查（go s.pinger 位于 SetReadLimit(16KiB) 之后；s.exitf( 调用点计数仍为 1；既有十四测全绿）"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-08-15
status: complete
---

# Phase 2 Plan 04: CORE-06 保活——pinger + --ping-interval + 保活三测 Summary

**WS ping/pong 保活全链路落地：--ping-interval flag（默认 5s，0 禁用）经 config→Options 透传至 Welcome 升档后启动的 pinger goroutine；pong 超时 10s（可覆写）打 stderr 单行事件并 CloseNow；保活三测（空闲存活/pong 超时断开/0 禁用反证）-race 三连跑全绿，读路径恒无 deadline，健康长空闲会话零误杀**

## Performance

- **Duration:** 13 min
- **Started:** 2026-08-15T08:16:58Z
- **Completed:** 2026-08-15T08:29:31Z
- **Tasks:** 2
- **Files modified:** 4（1 新建 + 3 修改）

## Accomplishments

- `pinger` goroutine 按 RESEARCH Pattern 3 语义落地：`interval<=0` 禁用直返（D-16）；ticker 循环 `select ctx.Done()/t.C`；每触发 `pctx = WithTimeout(ctx, s.pongTimeout)` 后 `c.Ping(pctx)`——真正的 pong 超时（`errors.Is(err, context.DeadlineExceeded)`）才打 `logEvent(remote, 1006, "pong_timeout")` + `CloseNow()` 后返回；启动点挂 Welcome 升档序列尾段（`SetReadLimit(16KiB)` 之后，PATTERNS 注意 5），注释钉死三条源码纪律（Ping 与 Reader 并发 conn.go:218-220 / pong 由 handleControl read.go:317-337 / 写串行化 write.go:288-293）
- 终结装配零新 exitf 分支：Attach 的 `ctx` 由 `context.Background()` 改为 `context.WithCancel` 派生 + `defer cancel()`——pinger 随 Attach 返回终结，进既有 wsDisconnected→terminate 单一路径收口（CONTEXT L92 硬约束）；`s.exitf(` 调用点计数保持 1
- Options 同构追加（02-02 建立的模式）：`PingInterval` 生产直传（0=禁用原样透传，New 不设默认）与 `PongTimeout` 测试可覆写（零值取 `defaultPongTimeout=10s`，D-16）分组注释；Server 增两 plain 字段（装配期固化）
- `cmd/wesh/main.go`：`fs.DurationVar(&cfg.pingInterval, "ping-interval", 5*time.Second, "WS ping interval (0 = disable)")`（DurationVar 原生支持 "5s" 语法，RESEARCH Don't Hand-Roll 表）；run 装配 `Options{Writable, PingInterval}` 透传
- `keepalive_test.go` 三测：TestPingKeepalive（100ms 注入，空闲 350ms >3 间隔存活 + echo 证据——Pitfall 2/3 回归锁）、TestPongTimeout（停 Read 600ms 不回 pong → 服务端 CloseNow；断首开观察用独立 2s 护栏 ctx + `errors.Is(DeadlineExceeded)` 排除护栏到期自证；waitExit(0) 锁 D-11 收口）、TestPingDisabled（PingInterval=0 + PongTimeout=100ms 陷阱参数，停 Read 500ms 后 echo 存活反证未发 ping）
- TestParseArgs 表驱动加 `wantPingInterval` 列：30s 解析组 + 0 禁用组 + 既有四组补默认 5s 断言
- 既有全部测试（server 十四测 + proto/pty/cmd 包）在保活装配件下保持全绿——pinger 对既有生命周期语义零干扰

## Task Commits

1. **Task 1 RED: TestParseArgs ping 组 + TestPingKeepalive 失败测试** - `2acccf6` (test) — cfg.pingInterval / Options.PingInterval 未定义（正确失败原因）
2. **Task 1 GREEN: pinger goroutine + --ping-interval flag + Options 扩展** - `1c32762` (feat)
3. **Task 2: keepalive 保活三测（含 pinger 错误路径收窄修复）** - `cf6bdee` (test)

**Plan metadata:** 见下方 docs 提交（docs(02-04): complete ...）

## TDD Gate Compliance

- RED gate: `2acccf6` test(02-04) — 编译期失败（unknown field PingInterval / cfg.pingInterval undefined，正确失败原因）✓
- GREEN gate: `1c32762` feat(02-04) — RED 之后，新测转绿且既有全量保持绿 ✓
- REFACTOR gate: 省略——实现即为最小形态；偏差 1 的错误路径收窄属行为修复（Rule 1），随 Task 2 提交

## Files Created/Modified

- `internal/server/server.go` - pinger 方法、Options.PingInterval/PongTimeout、defaultPongTimeout(10s)、Server 两 plain 字段、Attach ctx 改 WithCancel+defer cancel、升档序列尾段 pinger 启动点、logEvent 注释更新七处埋点清单
- `cmd/wesh/main.go` - config.pingInterval 字段、--ping-interval flag（DurationVar 默认 5s）、run 装配 Options.PingInterval
- `cmd/wesh/main_test.go` - TestParseArgs 加 wantPingInterval 列与 ping interval/ping disabled 两组
- `internal/server/keepalive_test.go` - 新建：保活三测（TestPingKeepalive/TestPongTimeout/TestPingDisabled）

## Decisions Made

- **pinger 签名增 remote 参数：** plan 文本给出 `go s.pinger(ctx, c, s.pingInterval)` 三参调用，但 action 同时要求 Ping 错误路径打 logEvent 三要素（remote/code/reason）——remote 是 Attach 入口保存的 `r.RemoteAddr`（logEvent 既有契约，反代聚合限制注释同源）。两处 plan 要求调和后签名扩为 `pinger(ctx, c, remote, interval)`。
- **Task 1 tdd=true 与测试文件归属的张力（02-03 先例照搬）：** keepalive_test.go 名义归属 Task 2，但 Task 1 的 TDD RED 需要行为失败测试先行——测试文件随 RED 创建（含 TestPingKeepalive + TestParseArgs ping 组，双双编译期失败），Task 2 在其上扩展 TestPongTimeout/TestPingDisabled。交付物与验收标准与计划一致，仅文件创建时点前移。
- **logEvent 的 code 参数记 1006（StatusAbnormalClosure）：** CloseNow 无关闭帧，客户端将观测到本地合成 1006——记 1006 即记录客户端真实可观测码；D-05"1006 永不发送"约束的是服务端 wire 发送，CloseNow 路径无 wire 码，不违反。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] pinger 错误路径收窄：仅 DeadlineExceeded 才打 pong_timeout + CloseNow**
- **Found during:** Task 2（TestPingKeepalive 首跑观测到 `code=1006 reason=pong_timeout` stderr 事件——该测试是正常终结路径，不应产生 pong 超时事件）
- **Issue:** 初版实现按 plan 字面"err 非 nil → logEvent → CloseNow"；但正常终结竞态下（客户端 Close(1000) 后、Attach defer cancel 执行前的微秒窗口），在途 Ping 因连接关闭而失败且 ctx.Err() 尚为 nil——误报 pong_timeout。短 interval（测试 100ms）下几乎必现；生产默认 5s 间隔窗口极小但语义同样失真——误报会污染 D-12② stderr 事件流的可信度
- **Fix:** 错误分类精确化——Ping 对 pong 等待的 ctx 到期返回包装后的 `context.DeadlineExceeded`（conn.go:251-258 `"failed to wait for pong: %w"`），父 ctx 是 WithCancel 无 deadline，故 DeadlineExceeded 唯一来源即 pctx 到期（真 pong 超时）；其余错误（net.ErrClosed、写失败、cancel 级联取消）一律静默返回，连接终结由既有 reader 路径收口
- **Files modified:** internal/server/server.go
- **Verification:** 修复后 TestPingKeepalive 运行零事件、TestPongTimeout 仍正确产出 pong_timeout 事件；三测 `-race -count=3` 三连跑无 flake
- **Committed in:** `cf6bdee`（Task 2 提交）

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** 修复保活事件流的误报面，属正确性必要修复；plan 的行为语义（真 pong 超时 → 事件 + CloseNow）完整保留且更精确。无范围蔓延。

## Issues Encountered

- TestPongTimeout 的测试设计陷阱：客户端"停止 Read"与"观察断开"天然矛盾（任何 Read 都会回 pong）——按 plan 形态以 sleep 等待 + 事后单次 Read 解决；Read 护栏 ctx 若到期会整连接关闭（conn.go AfterFunc）使 `err != nil` 自证通过，故用 `errors.Is(rerr, context.DeadlineExceeded)` 显式区分（read.go:255 核实：ctx 到期 Read 直接返回 ctx.Err()）。已固化为 patterns-established。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 02-05 上限 e2e：logEvent 埋点清单已更新为七处（含 pong_timeout）；ErrMessageTooBig 钩子挂点（两个读循环错误分支）保持可挂
- 02-06 期末 UAT 增补项：devtools 可观察 WS ping/pong 帧（5s 周期）与反代空闲超时下的长连接存活（RESEARCH §Sampling Rate phase gate 已含此项）
- Phase 9：反代配方文档（nginx 60s / Cloudflare 100s / 30s 型 ingress 对照 5s 默认值的论证材料已备——plan objective 与本 SUMMARY one-liner）
- 无阻塞项

## Self-Check: PASSED

- [x] `internal/server/keepalive_test.go`（三测试函数）/ `internal/server/server.go`（pinger L393、启动点 L325）/ `cmd/wesh/main.go`（flag L42、装配 L83）/ `cmd/wesh/main_test.go`（两组 L32-33）均存在于磁盘
- [x] 提交 `2acccf6` / `1c32762` / `cf6bdee` 均见于 `git log`
- [x] plan 级 verification 全绿：`go build ./... && go vet ./...` 退出 0；`go test -race -count=1 ./...` 四包全 PASS（server 十七测：既有十四 + 新三）；保活三测 `-race -count=3` 三连跑无 race 无 flake
- [x] 验收走查：pinger 启动点在 Welcome 升档之后（SetReadLimit(16KiB) 次行）；Ping 错误路径含 logEvent+CloseNow（L421-422）；Options 含 PingInterval/PongTimeout 且 PongTimeout 零值默认 10s；`s.exitf(` 调用点计数仍为 1（零新 exitf 分支）
- [x] 无意外文件删除（三次提交的 `git diff --diff-filter=D` 均为空）
- [x] gofmt（GOROOT 版）对本 plan 触及文件零输出；`internal/proto/proto.go` 的 gofmt 标出为 02-01 既存格式偏好差异（02-03 已记录），按 scope boundary 不顺手改

---
*Phase: 02-protocol*
*Completed: 2026-08-15*
