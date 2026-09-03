---
phase: 08-observability
plan: 02
subsystem: infra
tags: [slog, audit-events, structured-logging, observability, detach-reason, go-stdlib]

requires:
  - phase: 08-observability
    plan: 01
    provides: emitEvent 底层出口（slog JSONHandler，msg 恒 "event"）+ parseEvents/countByEvent 断言 helper + stderrMu/LockStderr 竞态防护（c4a8eed）
provides:
  - 事件目录全量（D-17）：attach{remote,client_id,mode,remote_user?} / detach{remote,client_id,code,reason,remote_user?} / session_start{pid} / session_end{exit_code,signal?,duration_seconds} / shutdown{} / throttled 增 retry_after 键——ROADMAP SC3 三面（认证/连接/会话生命周期）可检索
  - client_id 关联检索（D-20）：attach/detach 携 attachSeq（进程内单调递增从 1 起），同一连接事件流可按 client_id 关联
  - detach 单事件 reason 四值（D-21）：normal(1000)/kick(1013)/pong_timeout(1006)/shutdown(1000 lifecycle|1001 Shutdown)——kick/pong_timeout 独立事件行折入（零残留行为锁），wire 关闭形态逐字节不变
  - client.pongTimedOut（hubMu 保护，RESEARCH Pattern 4 形态 b 同步边）+ Server.closeBroadcastCode（hubMu，与广播码同源）+ Server.startedAt（D-22 duration 数据源）
  - exitSignalNum 包级 helper（信号号提取单侧定义——exitMessage 与 session_end 共用，行为逐字节不变）
  - pinger 新签名 (ctx, cl, interval)；emitDetachLocked 唯一 detach emit 形态
  - D-19 纵深第二道：remote() trust 分支过 sanitizeRemoteUser（ParseIP 闸保持，双闸并存）
  - D-23 字段边界：throttled 携 retry_after（与 Retry-After 头同值）；auth_failed 无用户名结构性锁（TestAuthFailedNoUsername）
  - phase05.mjs S6 终态迁移（detach reason=kick JSON 断言）
affects: [08-03 healthz, 08-04 metrics, 08-05 UAT/README 收口（jq 示例引用本事件 schema）]

actuals:
  tokens: 14413
  tasks: 3
  commits: 7

tech-stack:
  added: []
  patterns:
    - "detach reason 跨 goroutine 传递的 hubMu 同步边（Pattern 4 形态 b）：pinger 置位取锁写 cl.pongTimedOut → CloseNow → reader 错误 → detach 同锁读——-race 干净由测试四子测锁定"
    - "终结广播码单写口：lifecycle/Shutdown 在 exiting 置位同点写 closeBroadcastCode（1000/1001），detach reason=shutdown 的 code 与 wire 广播码结构性同源"
    - "恰好一次事件归属：detach 事件由 removeLocked 返回 true 的路径 emit（reader detach 与 kick 互斥——既有成员判定所有权规则的事件面映射）"
    - "进程级事件同步边：session_start 在 New 内（程序序）、session_end 先于 terminate→exitf（waitExit 收码即同步）、shutdown 由调用方 goroutine 直调 emit——零客户端场景无需 waitHandlers"
    - "XFF 注入面测试取中段注入（尾段 NEL 被 TrimSpace 先吃掉——unicode.IsSpace(NEL)=true 实证），真实受力面是 ParseIP 闸"

key-files:
  created:
    - internal/server/events_test.go
  modified:
    - internal/server/server.go
    - internal/server/clients.go
    - internal/server/auth.go
    - internal/server/proxy.go
    - internal/server/exitmsg_test.go
    - internal/server/proxy_test.go
    - web/uat/phase05.mjs

key-decisions:
  - "emitDetachLocked 为 detach 事件唯一 emit 形态（detach 与 kickSlowConsumerLocked 两调用点共用，D-18 schema 单侧定义）；reason 判定序 pongTimedOut → exiting → normal 落 detach() 内"
  - "attach 事件 emit 点 = Attach 升档序列 hubMu.Unlock 之后、SetReadLimit 之前（registerLocked 的 attachSeq 写与本读同 goroutine 程序序 happens-before）"
  - "session_end 在 lifecycle 的 sess.Wait 返回与退出码提取完成后、EXIT 帧组帧之前 emit——exit_code 与 EXIT 帧同源（信号死亡 -1），signal 键仅信号死亡且 signalName 命中出键（A7 裁决落地）"
  - "exitSignalNum 抽取保持 exitMessage 行为逐字节不变（未命中时 sig 保持占位 code=-1 的回退形态原样）——TestExitMessage/TestExitFrameBroadcast/TestExitFrameSignal 全绿为回归证据"

patterns-established:
  - "事件恰一次计数锁形态：singleDetachReason（event+reason 精确计数==1）+ 独立事件名零残留断言（countByEvent(slow_consumer/pong_timeout)==0 锁 D-21 折入）"
  - "进程级事件测试形态：零客户端实例（事件面恰 N 条强锁）+ sess 句柄暴露夹具（pid 相等性断言）"

requirements-completed: [OPS-08]

coverage:
  - id: D1
    description: "attach/detach 事件链：client_id 关联（attachSeq 从 1 起）+ reason 四值行为锁（normal/kick/pong_timeout/shutdown 两形态）+ wire 关闭形态零漂移 + kick/pong_timeout 独立行零残留"
    requirement: OPS-08
    verification:
      - kind: unit
        ref: "internal/server/events_test.go#TestAttachDetachEvents + #TestDetachReason（五断言体 -race 全绿）"
        status: pass
      - kind: e2e
        ref: "node web/uat/phase05.mjs 28/28（S6a 终态：event=detach reason=kick code=1013）"
        status: pass
    human_judgment: false
  - id: D2
    description: "会话生命周期事件：session_start{pid} / session_end{exit_code,signal?,duration_seconds} / shutdown{}；exitSignalNum 单侧定义抽取（exitMessage 逐字节回归）"
    requirement: OPS-08
    verification:
      - kind: unit
        ref: "internal/server/events_test.go#TestSessionEnd（两子测）+#TestShutdownEvent + exitmsg_test.go#TestExitSignalNum（-race 全绿）；exitmsg 既有全绿"
        status: pass
      - kind: integration
        ref: "真实二进制冒烟：sh -c 'exit 7' → session_start{pid:900165} + session_end{exit_code:7,duration_seconds} JSON 行可检索"
        status: pass
    human_judgment: false
  - id: D3
    description: "认证事件边界：throttled 携 retry_after（与 Retry-After 头同值）；auth_failed 无 user/username 键且全文不含用户名串；remote() trust 分支 sanitize 推广（白盒属性断言无 C0/C1/DEL rune）"
    requirement: OPS-08
    verification:
      - kind: unit
        ref: "internal/server/events_test.go#TestThrottledRetryAfter+#TestAuthFailedNoUsername + proxy_test.go#TestRemoteSanitize（-race 全绿）"
        status: pass
      - kind: integration
        ref: "go test -race -count=1 ./... 五包全绿（54.2s）；phase07.mjs 34/34（S4 XFF/NEL 断言零回归）"
        status: pass
    human_judgment: false

duration: 71min
completed: 2026-08-28
status: complete
---

# Phase 8 Plan 02: 审计事件目录全量 Summary

**D-17 审计事件目录在 08-01 slog 基座上全量落地：attach/detach（client_id 关联 + reason 四值，kick/pong_timeout 折入 detach 零残留）+ session_start/session_end/shutdown 进程级三事件 + throttled 携 retry_after + remote 字段 sanitize 纵深第二道；exitSignalNum 单侧定义抽取零行为漂移；全仓 -race 绿 + phase05/07 UAT 62 断言全过**

## Performance

- **Duration:** 71 min
- **Started:** 2026-08-27T23:58:47Z
- **Completed:** 2026-08-28T01:10:13Z
- **Tasks:** 3/3
- **Files modified:** 8（1 新建 + 7 修改，与 plan files_modified 清单逐一对应）

## Accomplishments

- **attach/detach 事件链端到端（D-17/D-20/D-21）**：attach 携 remote/client_id/mode（RESEARCH A6 增强字段落地）；detach 单事件 reason 四值——normal(1000)/kick(1013)/pong_timeout(1006)/shutdown(1001 Shutdown / 1000 lifecycle 广播窗口，code 与广播码同源 closeBroadcastCode)；kick 与 pong_timeout 独立事件行删除折入（行为锁断言零残留），wire Close(1013,"slow_consumer") 与 kicks 计数逐字不动
- **跨 goroutine reason 传递 -race 干净**：pinger 签名收窄 (ctx, cl, interval)，DeadlineExceeded 分支取 hubMu 置 cl.pongTimedOut 后 CloseNow（RESEARCH Pattern 4 形态 b）；detach 同锁读——四子测 -race 强制
- **会话生命周期三面（D-17/D-22）**：session_start{pid=sess.Cmd.Process.Pid}（New 尾部 goroutine 启动前）；session_end{exit_code 与 EXIT 帧同源、signal 仅信号死亡且映射命中出键、duration_seconds=startedAt 起点}；shutdown{}（Shutdown 入口）；exitSignalNum 抽取为 exitMessage 与 session_end 共用单侧定义（白盒四形态 + 既有三测逐字节回归）
- **认证事件边界（D-23/D-19）**：throttled 事件携 retry_after（与 Retry-After 响应头同一变量同值）；auth_failed 无用户名由 logEvent 四参签名结构性保持 + 行为锁（无 user/username 键 + 全文不含独特用户名串）；remote() trust 分支过 sanitizeRemoteUser——ParseIP 第一道闸保持，C1 穿透清洗第二道（Pitfall 5），两闸并存
- **phase05.mjs S6 终态迁移**：S6a 断言 event=detach reason=kick code=1013（08-01 预告的二次迁移兑现），全脚本 28/28 绿

## Task Commits

Each task was committed atomically（TDD RED→GREEN 每任务两提交）:

1. **Task 1 RED: attach/detach 失败测试先行** - `3683e7f` (test)
2. **Task 1 GREEN (tracer): attach/detach 事件链 + reason 四值 + S6 迁移** - `43215d5` (feat)
3. **Task 2 RED: session 生命周期失败测试先行** - `536eedd` (test)
4. **Task 2 GREEN: session_start/session_end/shutdown + exitSignalNum 抽取** - `03cc874` (feat)
5. **Task 3 RED: 认证边界测试先行** - `32b96c7` (test)
6. **Task 3 GREEN: throttled retry_after + auth_failed 红线锁 + remote sanitize** - `5a5f7a0` (feat)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP）

_Tracer feedback gate（autonomous，08-01 同款）：Task 1 提交后 verify 四腿端到端重跑通过（-race 专项 9.0s / 全包 48.5s / 构建 / phase05.mjs 28/28），方进入 Task 2/3 扩展面。_

## Files Created/Modified

- `internal/server/events_test.go`（新建）— D-17/D-20/D-21/D-23 行为锁：TestAttachDetachEvents / TestDetachReason（normal/kick/pong_timeout/shutdown 四子测，shutdown 含 1001/1000 两形态）/ TestSessionEnd（exit 42 + kill -HUP 两子测）/ TestShutdownEvent / TestThrottledRetryAfter / TestAuthFailedNoUsername；eventsNamed/singleDetachReason 断言 helper + startEventsServerWith 全量返回夹具（srv+sess 句柄 + waitHandlers 同步边）
- `internal/server/server.go` — Server 加 closeBroadcastCode/startedAt 字段；Attach 升档 attach emit；pinger 签名收窄 + pongTimedOut 置位（独立事件行删除）；lifecycle closeBroadcastCode=1000 同点登记 + session_end emit；Shutdown closeBroadcastCode=1001 + 入口 shutdown emit；exitSignalNum 抽取 + exitMessage 改调
- `internal/server/clients.go` — client 加 pongTimedOut 字段（hubMu 保护注释登记）；detach() reason 判定序 + emit；kickSlowConsumerLocked 的 slow_consumer 行替换为 detach reason=kick；emitDetachLocked 唯一 emit 形态
- `internal/server/auth.go` — throttled 站点换 emitEvent 携 retry_after（与 Retry-After 头同值）；auth_failed 保持 logEvent 零改动（红线注释点名 D-23）
- `internal/server/proxy.go` — remote() trust 分支过 sanitizeRemoteUser（D-19 推广，注释登记双闸论证）
- `internal/server/exitmsg_test.go` — TestExitSignalNum 白盒四形态表（真实子进程产出 ExitError 两形态）
- `internal/server/proxy_test.go` — TestRemoteSanitize 表驱动 + 逐 rune 无 C0/C1/DEL 属性断言
- `web/uat/phase05.mjs` — S6a 二次迁移（detach reason=kick）+ 注释更新

## Decisions Made

- **emitDetachLocked 单侧定义**（plan action ①「同款 detach emit」的落地形态）：detach 与 kick 两调用点共用包级方法，schema 单写口
- **startEventsServerWith 夹具全量化**（Task 2 action ③ 的 pid 相等性断言需要 sess 句柄）：startEventsShutdownServerWith 改为其包装（Task 1 引入的本地夹具演进，零外部消费面）
- **XFF 注入表行取中段**（详见 Deviations #1）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TestRemoteSanitize 表行尾段 NEL 注入不构成受力面**
- **Found during:** Task 3（RED 表设计——运行前探针实证）
- **Issue:** plan action ④ 要求「构造含 C1/NEL 的 XFF 首段与含 C1 的边界输入」；初版表行取尾段注入 `"1.2.3.4\u0085"`，但 `unicode.IsSpace(NEL)=true`（本机 go run 探针实证）使 clientIP 的 TrimSpace 先吃掉尾段 NEL → ParseIP 通过 → remote() 返回干净 IP 而非回退——该形态测不到 ParseIP 闸拒面
- **Fix:** 注入位置改中段 `"1.2.3\u00854"`（TrimSpace 不触中段 → ParseIP 拒 → 回退 TCP 对端键），表行注释登记实证依据；NEL-only 首段与 CSI 尾段行保持
- **Files modified:** internal/server/proxy_test.go
- **Verification:** TestRemoteSanitize 七行全绿（GREEN 后），属性断言逐 rune 覆盖全表输出
- **Committed in:** 32b96c7（RED）/ 5a5f7a0（GREEN）

---

**Total deviations:** 1 auto-fixed（测试设计修正——实证驱动，零行为面漂移）
**Impact on plan:** 全部锁定语义与 plan must_haves 逐字一致；无 scope creep；无 Rule 4 架构变更、无认证门、无包安装

## Issues Encountered

- **Edit 工具对 `\uXXXX` 字面量的转义歧义**——proxy_test.go 注入行的 C1 字符经 perl 落为显式 Go 转义形态（与 TestProxyClientIP 先例一致）；纯工具层周折，零语义影响
- **attach emit 块缩进**——初版插入块缩进浅一级被 GOROOT gofmt 即时标出并 -w 修正（仅本块重排，git diff 逐行核读零意外面）；multi_test.go/slowclient_test.go 的既有 gofmt 漂移按 SCOPE BOUNDARY 纪律不动（07-deployment/deferred-items.md 已登记同族）

## Known Stubs

无——全部事件字段真实接线（无占位值/空数据流于 UI 面；本 plan 纯服务端事件面）。

## Next Phase Readiness

- **08-03/08-04 挂点不动面确认**：shutdown 事件与 08-03 draining 置位「同函数不同点」约定已在注释登记；metrics 预埋挂点（kicks/gateTransitions/droppedInputs/inputDrops）本 plan 零触碰
- **08-05 README jq 示例的数据源即本 plan schema**：attach/detach 经 client_id 关联（`select(.client_id==7)` 形态已被 Go 侧行为锁覆盖）；session_end 单事件回答「活多久、怎么死的」经真实二进制冒烟实证（exit_code:7 + duration_seconds）
- **无阻塞项**

## Self-Check: PASSED

- 文件：events_test.go ✓（新建）、server.go/clients.go/auth.go/proxy.go/exitmsg_test.go/proxy_test.go/phase05.mjs ✓（修改）、08-02-SUMMARY.md ✓
- 提交：3683e7f ✓、43215d5 ✓、536eedd ✓、03cc874 ✓、32b96c7 ✓、5a5f7a0 ✓
- 关键指纹：server.go `func exitSignalNum` ×1 ✓、clients.go `pongTimedOut` ×4 ✓ / `emitDetachLocked` ×4 ✓、events_test.go `TestDetachReason` ×2 ✓
- 既有回归：TestPingKeepalive/TestSlowConsumerKick/TestSuccessionKickRace/TestShutdown1001 及全仓五包 `-race` 全绿（54.2s）；os.Stderr 竞态（c4a8eed 修复面）零重现
