---
phase: 13-resource-defense
plan: 03
subsystem: server-perclient-termination
tags: [pc-supervisor, second-terminator, exit-when-empty, last-reaped-code, session-end-audit, wr02-reaped-fence, PC-09, OPS-12]
requires:
  - "Phase 11 D-01/D-04 终结拓扑（sessionWatcher/teardownPCLocked/pcSessions 注册表 + pc.startedAt/exitCode 预留字段）"
  - "06-02 maybeExitWhenEmptyLocked 四守卫/宽限计时器机械 + 08-review WR-01 空纪元门闩（本 plan per-client 侧激活）"
  - "server.go terminate/termOnce 唯一收口件（:1586-1590——逐字复用）与 hubCond 既有装配（:525 区域）"
  - "11-04 KILL 兜底时序双断言形态 + trap '' HUP 免疫夹具（WR-02 测试与 session_end 信号死亡形态复用）"
provides:
  - "pcSupervisor 第二终结源单例 goroutine（hubCond 等 (pcExitReq || exiting) && len(pcSessions)==0 → terminate(last-reaped-code)——Pitfall 1 窗口期闭合）"
  - "maybeExitWhenEmptyLocked per-client 触发端（立即/宽限到期两置位点 pcExitReq=true + Broadcast；四守卫与计时器机械两模式共用）"
  - "upgradePerClient 宽限取消点 + 空纪元门闩清零（Rule 1 补齐——shared Attach 路径同位同款）"
  - "sessionWatcher 收割链三补（pcLastExitCode/pcHasExitCode 记录 + session_end 事件（shared schema + client_id）+ 慢半段 delete 后 Broadcast）"
  - "teardownPCLocked WR-02 waitDone 非阻塞 select 结构性栅栏（快半段信号分支门卫 + 补 KILL 回调同构覆盖）"
  - "export_test 白盒出口 TeardownReapedFenceForTest（waitDone 预关闭 + reaped 未置位构造 + teardown 直调）"
affects:
  - "13-04（Shutdown N 进程组快照 + exiting 侧 Broadcast 填充——pcSupervisor 的 exiting 消费面已先行落定）"
  - "13-05（session_start emit 挂点 upgradePerClient :267-269 区间 per D-09 后半）"
  - "13-07（phase13.mjs S3 进程级 255 断言消费 last-reaped-code 语义）"
  - "13-08（收口闸：PC-09 flagged-unverified 终验 + gofmt 存量 deferred 命中知悉）"
tech-stack:
  added: []
  patterns:
    - "第二终结源单例（supervisor goroutine + cond 等待复合条件——「装配期一次分岔」分支点形态扩展）"
    - "结构性同步边栅栏（channel 关闭态非阻塞 select 替代标志位读——Wait-return→lock-acquire 微窗口闭合零新同步件）"
    - "happens-before 链式 emit 定位（emit → close(waitDone) → delete → Broadcast → exitf——事件先于进程级收口落流的结构保证）"
    - "白盒微窗口确定性注入 + 源码 region 断言双通道（行为面不可观测的形态锁定）"
key-files:
  created: []
  modified:
    - internal/server/server.go
    - internal/server/clients.go
    - internal/server/perclient.go
    - internal/server/emptyexit_test.go
    - internal/server/perclient_test.go
    - internal/server/events_test.go
    - internal/server/export_test.go
decisions:
  - "session_end emit 位置取「Wait 返回 + 退出码提取后、close(waitDone) 前」（plan 文本「hubMu 解锁后」的偏差，Rule 1）：与 shared lifecycle 母本同位（emit 先于一切收口活动），且建立 emit → close(waitDone) → 慢半段 <-waitDone → delete+Broadcast → pcSupervisor exitf 的 happens-before 链——plan 位置（unlock 后）与慢半段 Broadcast 无同步边，调度停摆下 exitf(os.Exit) 可先于事件落流（生产事件丢失面 + 测试 pcSessions 收敛观测同步边失效）"
  - "WR-02 栅栏同构覆盖补 KILL AfterFunc 回调（plan 文本只点名快半段）：既有代码注释载 planner 裁定「Pitfall 2 信号/reap 序列化语义对一切 kill(-pgid) 同构适用」——同一微窗口、同一修法，回调内 reaped 复检后追加 waitDone 非阻塞 select"
  - "子先死断言值取 42 非 plan 文本例值 0：缺省 0 与「pcLastExitCode 未记录」缺陷同值（supervisor 缺省 0 路径 vacuous pass），42（SESS-01/02 UAT 既有断言值）才有判别力——收紧非放宽"
  - "KILL 兜底归因断言 signal==\"SIGKILL\"（plan 文本 \"KILL\" 为路径简称）：signalName 映射链的实际产出为显式大写全名 SIGKILL——断言锁实际链路值"
  - "Rule 1 关键补齐：upgradePerClient registerLocked 后补 cancelExitEmptyTimerLocked + exitEmptySignaled=false（11-01 早退守卫期两挂点 per-client 不可达故从未装配；触发端激活后缺失 = 宽限取消仅靠回调 registry 非空复查兜底 + 门闩永不清零致 exit-when-empty 单发）——Task 2 GraceCancel 测试首跑 FAIL 实测暴露，单独 fix 提交收口"
  - "Rule 1 跨测试迟写修复：TestAuthFailedNoUsername 装配改 startEventsServerWith + 体内 Kill→waitExit 收口同步边（原装配弃置 exitCh，cleanup SIGKILL 的迟到 lifecycle session_end emit 落入后继捕获窗——全量 -race 下 TestPerClientSessionEnd/exit_code_42 严格计数实测受害；断言行零改动）"
  - "PC-09/OPS-12 勾选留 phase 末收口（11-01/12-01/13-01/13-02 先例：ID 跨 plan 共享——本 plan 机制与 Go 级证据齐，13-07 phase13.mjs S3 进程级 255 断言未落）"
metrics:
  duration: 50min
  completed: 2026-09-05
  tasks: 2
  commits: 3
status: complete
actuals:
  tokens: 14427
  tasks: 2
  commits: 3
---

# Phase 13 Plan 03: pcSupervisor 第二终结源 + last-reaped-code + session_end 审计（PC-09）Summary

per-client 终结语义重建：pcSupervisor 单例 goroutine（hubCond 复合条件等待 → terminate(last-reaped-code)，termOnce 逐字复用）+ maybeExitWhenEmptyLocked 三形态触发端闭合 Pitfall 1「永不退出」窗口期 + session_end per-client 粒度审计事件（shared schema + client_id）+ WR-02 waitDone 结构性栅栏；退出码两时序（子先死透传/客户端先断 -1→255）与 shared 逐位对齐。

## What Was Built

**Task 1（机制，feat 2c5fd6e）**：

- `server.go`：Server 三新字段 `pcExitReq/pcLastExitCode/pcHasExitCode`（hubMu 保护，注释载 happens-before 论证——goroutine 启动 + hubMu 读写建立同步边）；`pcSupervisor()` 方法（研究 §4.1 参考实现逐字段对应：hubMu 内 `for !((pcExitReq || exiting) && len(pcSessions) == 0) { hubCond.Wait() }` → 出循环读 last-reaped-code → 放锁后 `s.terminate(code)`——termOnce 恰好一次与 shared 同一收口件，函数体零 `s.exitf(` 直调）；New per-client 分支在 pcSessions/spawnThrottle 装配后钉死 `go s.pcSupervisor()`（shared 分支零装配）；termOnce/terminate 注释更新为双触发源交汇口径（T-13-11）
- `clients.go`：maybeExitWhenEmptyLocked 早退守卫（11-01「永不退出」窗口期）替换为第二终结源触发端——四守卫与宽限计时器机械两模式共用同代码，仅两触发点动作面按模式分岔：grace==0 立即形态与宽限到期回调（身份比对 + 复查通过后）各置位 `pcExitReq = true + hubCond.Broadcast()`（不发信号——末端断开的 teardown 已 SIGHUP 其会话）；per-client 分支绝不调 stopChildLocked（s.sess 恒 nil，nil-deref 防线语义迁移至注释）；宽限取消点 cancelExitEmptyTimerLocked 零改动
- `perclient.go`：sessionWatcher 收割链三补——hubMu 置位区段追加 `pcLastExitCode/pcHasExitCode`（last-reaped-code 记录）；session_end emit（shared schema 母本逐键：event/exit_code/duration_seconds=Since(pc.startedAt)/signal 经 exitSignalNum+signalName 链 + `client_id`=cl.attachSeq 关联键；位置 = Wait 返回+退出码提取后、close(waitDone) 前——见 Decisions）；teardown 慢半段 delete(pcSessions) 后同锁内 `hubCond.Broadcast()` 补行唤醒 pcSupervisor 重估；teardownPCLocked 快半段 WR-02 栅栏（`if !pc.reaped` 内非阻塞 select waitDone——已关闭即跳过信号分支；补 KILL AfterFunc 回调同构栅栏）；:28-32 窗口期头注释移除改写为 13-03 落地登记；pcSession startedAt/exitCode 字段注释激活/更新
- verify：build+vet 零输出；全量 -race 81.6s 首跑绿（既有组零回归 = 机制插入零破坏）；acceptance grep 闸逐条过（pcSupervisor 两命中/函数体零 exitf/两置位点/三锚点/select 守卫先于 SignalGroup/永不退出表述零命中/diff 白名单恰三文件）

**Task 2（测试组，test 81f4c91 + fix c44a8d9）**：

- `emptyexit_test.go`：`TestEmptyExitPerClientChildFirst`（子先死：EXIT{42}+1000 wire 观测 → exitf(42) 恰好一次——last-reaped-code 判别值 42；TestExitWhenEmptyLifecycleGate per-client 同构）+ `TestEmptyExitPerClientClientFirst`（客户端先断：exitf(-1) 恰好一次 + exit_when_empty 事件恰一——emit 先于 pcExitReq 置位的程序序使 waitExit 收码即事件落流）；头注释载 §4.3 两时序逐位对齐证明表；shared 既有断言零改动（diff 纯新增 101 行零删除）
- `perclient_test.go`：`TestPerClientExitWhenEmptyImmediate/GraceExpire/GraceCancel`（三形态全锁——到期前 100ms 静默/到期 3s 护栏收码/宽限内重连 echo 验证 + 越旧到期点静默 + 再断开重新计时收码）+ `TestPerClientOnce`（--once ≡ maxClients=1+exit-when-empty+grace=0 语法糖展开同路径）+ `TestPerClientReapedFence`（WR-02 双通道：源码 region 断言 select 守卫先于 SignalGroup + 白盒构造行为面——teardownDone 3s 护栏收口、不阻塞不 panic；测试侧唯一 Wait 调用方防僵尸）
- `events_test.go`：`TestPerClientSessionEnd` 两形态 schema——exit_code_42（恰一 + exit_code==42 + duration>0 + client_id 与 attach 同值 + 无 signal 键；同步边 = EXIT 帧 wire 观测）/ signal_sigkill（trap "" HUP 免疫 + StopTimeout=500ms → 补 KILL → exit_code==-1 + signal=="SIGKILL" 归因；同步边 = pcSessions 收敛观测——emit→close(waitDone)→delete 程序序链）；`TestAuthFailedNoUsername` 体内 lifecycle 收口同步边补齐（Rule 1，见 Deviations）
- `export_test.go`：`TeardownReapedFenceForTest` 白盒构造出口（pcSession 全字段真实构造 + waitDone 预关闭 + hubMu 内 teardownPCLocked 直调，ForTest 四件套纪律）

**Tracer 反馈门形态**（autonomous: true，11-01/13-02 先例）：Task 1 提交后全量 -race 端到端重跑绿即放行 Task 2；Task 2 内两组 Rule 1 修复后定向组 + 全量 -race 共三轮绿收口。

## Commits

| Task | Commit | Type | Summary |
|------|--------|------|---------|
| 1 | 2c5fd6e | feat | 三字段 + pcSupervisor 钉死 + 触发端三形态 + watcher 收割链三补 + WR-02 栅栏（3 文件 207+/67-） |
| — | c44a8d9 | fix | Rule 1：upgradePerClient 宽限取消点/门闩清零补齐 + pcSupervisor 注释 gofmt 归一（2 文件 11+/1-） |
| 2 | 81f4c91 | test | 两时序分叉 + 三形态 + once + ReapedFence + session_end schema + 白盒出口（4 文件 521+/2-） |

## Verification Results

- `go build ./...` + `go vet ./...` 零输出；GOROOT gofmt（go1.26.3，10-05 收口闸工具）本 plan 全部新增行 clean（六处 CJK 标点接续行与 trap 引号形态在提交前归一；存量 deferred 命中三处见下）
- 定向组 `TestPerClientExitWhenEmpty|TestPerClientOnce|TestPerClientReapedFence|TestPerClientSessionEnd|TestEmptyExit` -race 全绿（8 PASS 含两 subtest）
- `time go test -race ./internal/server/ -count=1` 全绿四轮（81.6s/90.6s/89.9s/91.8s——机制后/leak 修复后/复跑/最终；shared 与 per-client 既有组零回归）
- 全仓四包快速回归绿（cmd/wesh + proto + pty）
- WR-02 双通道：白盒构造（teardownDone 护栏内收口）+ 源码 region 断言（select 守卫 @region 内先于 SignalGroup）均在场
- 白名单闸：Task 1 diff 恰三产品文件；emptyexit_test.go diff 零删除行；events_test.go 唯一删除行 = TestAuthFailedNoUsername 装配调用行（断言行零改动）；cancelExitEmptyTimerLocked 函数体零 diff

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] session_end emit 位置：close(waitDone) 前而非「hubMu 解锁后」**
- **Found during:** Task 1 实现（plan behavior 文本「hubMu 解锁后、EXIT 组帧直写前」的位置论证审查）
- **Issue:** plan 位置（unlock 后 emit）与慢半段 delete+Broadcast 无同步边——调度停摆下 pcSupervisor 的 terminate→exitf（生产 = os.Exit）可先于事件落流（审计事件丢失面）；且 Task 2 signal_sigkill 形态的 pcSessions 收敛观测同步边失效（观测到 len==0 不保证 emit 完成，测试 flake 面）
- **Fix:** emit 前移至 Wait 返回+退出码提取后、close(waitDone) 前——与 shared lifecycle 母本同位（emit 先于 Drain/Close 等一切收口活动），建立 emit → close(waitDone) → 慢半段 <-waitDone → delete+Broadcast → exitf 的完整 happens-before 链；plan 的两条 rationale（事件先于 EXIT 帧 wire 落流 ✓ / 避开 hubMu 内 I/O ✓）全保持
- **Files modified:** internal/server/perclient.go
- **Commit:** 2c5fd6e

**2. [Rule 1/Rule 3 - Blocking] upgradePerClient 缺宽限取消点与空纪元门闩清零**
- **Found during:** Task 2 首跑（TestPerClientExitWhenEmptyGraceCancel FAIL：第二次断开后 exit_when_empty_wait 未重武装、exitf 不触发）
- **Issue:** shared Attach 路径（server.go :1147-1151）在 registerLocked 后调 cancelExitEmptyTimerLocked + exitEmptySignaled=false；per-client 升档路径（upgradePerClient）在 11-01 落地时因早退守卫使两挂点不可达而从未装配。触发端激活后缺失 = ①宽限取消只靠回调 registry 非空复查兜底（客户端在宽限内断开时计时锚点漂移至旧纪元）；②门闩永不清零 → 后续空迁移被 exitEmptySignaled 抑制永不重触发（exit-when-empty 退化为进程生命周期单发）
- **Fix:** upgradePerClient registerLocked 后同 hubMu 持有内补两行（shared 同位同款）；宽限取消点函数本体零改动（plan「取消点零改动」按字面保持）
- **Files modified:** internal/server/perclient.go
- **Commit:** c44a8d9

**3. [Rule 1 - Bug] TestAuthFailedNoUsername 跨测试迟写 lifecycle emit**
- **Found during:** Task 2 全量 -race 首跑（TestPerClientSessionEnd/exit_code_42 FAIL：session_end count=2——混入无 client_id 的 shared SIGKILL 事件）
- **Issue:** 该测试装配弃置 exitCh，t.Cleanup killServer 的 SIGKILL 触发 lifecycle 的 session_end emit 与测试收口无同步边——迟到的 emit 落入后继捕获测试窗口（两测复现确认：迟写行实测截获于 TestPerClientSessionEnd 运行期输出）；属既有潜在缺陷，本 plan 严格计数测试首次使其可见
- **Fix:** 装配改 startEventsServerWith（补 sess/exitCh 句柄）+ 体内显式 Kill → waitExit(-1) 收口同步边（waitExit 收码即 emit 已落本测试窗内）；断言行零改动
- **Files modified:** internal/server/events_test.go
- **Commit:** 81f4c91

**4. [Rule 1 - 加固] WR-02 栅栏同构覆盖补 KILL AfterFunc 回调**
- **Found during:** Task 1 实现（plan 文本只点名快半段 SignalGroup 判定）
- **Issue:** 补 KILL 回调的 `if !pc.reaped` 复检有同一 Wait-return→hubMu-acquire 微窗口（回调取锁先于 watcher 置位时对已收割 pgid 发 KILL）；既有代码注释已载 planner 裁定「Pitfall 2 信号/reap 序列化语义对一切 kill(-pgid) 同构适用，不只 SIGHUP」
- **Fix:** 回调内 reaped 复检后追加同一 waitDone 非阻塞 select 栅栏（同构修法、零新同步件）
- **Files modified:** internal/server/perclient.go
- **Commit:** 2c5fd6e

### Plan 措辞与实证的微小出入（不构成偏差）

- 子先死断言值 42 vs plan 文本例值 0：缺省 0 与 pcLastExitCode 未记录缺陷同值（vacuous pass），42 才有判别力（SESS-01/02 UAT 既有断言值「exit 42 透传」）——收紧非放宽，头注释登记
- KILL 兜底归因断言 signal=="SIGKILL" vs plan 文本 "KILL"：signalName 映射链实际产出显式大写全名 SIGKILL（exitmsg.go 映射表），断言锁实际链路值
- GOROOT gofmt（go1.26.3）对 `//（`/`//「` CJK 标点接续行与 doc 注释内 ASCII `''` 的归一要求：本 plan 六处新增行提交前补空格/改全角引号归一；另发现 13-02 遗留 perclient_test.go:2007-2009 双空行命中（13-02 SUMMARY gofmt 核查遗漏）登记 deferred-items.md——范围外不修

## Flagged Assumptions 处置（plan flagged_assumptions 兑现）

PC-09 unclassified 边界形态执行中发现一项，显式登记（非静默放行）：

- **grace=0 立即形态 + 重连竞态边界**：立即形态置位 pcExitReq 后该位不重置（蓝本 §4.1 参考实现同形态——无 reset 路径）。若置位后会话收割前新客户端 attach：supervisor 因 len(pcSessions)>0 继续 服务新会话，新会话终结后服务端退出（「已武装的退出」语义）。与 shared 对照：shared 立即形态以 SIGHUP 不可撤回性承担同语义（重连 attach 到将死会话随其死亡）。两模式「退出不可撤销」一致、重连存活窗口形态不同（per-client 新会话被服务至其终结）。判定：蓝本忠实形态 + --once（maxClients=1）下容量闸使重连在收割完成前即被拒（结构性收敛），保持 flagged-unverified 由 13-08 收口闸终验
- 其余未分类边界（--once 与 --exit-when-empty 叠加、grace 与 Shutdown 竞态交错）未在执行中发现新面——叠加形态经 TestPerClientOnce（同路径展开）覆盖 Go 级，Shutdown 交错归 13-04 填充后专测

## Requirements Trace

PC-09/OPS-12 勾选**不随本 plan 执行**（11-01/12-01/13-01/13-02 先例：ID 跨 plan 共享）——本 plan 落地机制本体 + Go 级全量证据（两时序/三形态/WR-02/session_end schema），13-07 phase13.mjs S3 进程级 255 断言与 13-08 收口闸终验未齐；勾选归 phase 末收口 plan。

## Known Stubs

None——全链无桩：pcSupervisor/触发端/收割链三补/WR-02 栅栏均为真实实现；session_start emit 归 13-05（D-09 后半，plan 明示分片非桩）；Shutdown N 进程组快照归 13-04（objective 明示分片）。

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-09 (DoS: 空即不死) | mitigate | pcSupervisor/pcExitReq 第二终结源 + 三形态触发端（Task 1）+ 三形态测试锁定（Task 2——GraceCancel 取消形态静默实证绝不置位） |
| T-13-10 (Tampering: kill-after-reap 误杀复用 pgid) | mitigate | WR-02 waitDone 非阻塞 select 结构性栅栏（快半段 + 补 KILL 回调双覆盖）+ TestPerClientReapedFence 白盒构造 + 源码 region 断言双通道；既有 reaped 栅栏保持 |
| T-13-11 (DoS: exitf 多触发源竞态双收口) | mitigate | termOnce/terminate 逐字复用单点；pcSupervisor 出循环后唯一调用点（函数体零 s.exitf 直调，acceptance grep 过）；全量 -race 下 11-04 竞态注入组原样绿 |
| T-13-12 (Information Disclosure: session_end 敏感值) | accept | schema 复用 shared 母本（exit_code/duration/signal/client_id 全数值/关联键，零敏感值）；emitEvent 既有通道保持 |
| T-13-SC (供应链) | mitigate | 零新依赖——sync.Cond/hubCond 既有件复用；go.mod/go.sum 零 diff |

## Self-Check: PASSED

- 文件存在：server.go（pcSupervisor + 三字段 + go 钉死）/ clients.go（两置位点触发端）/ perclient.go（session_end emit + last-reaped 记录 + 慢半段 Broadcast + WR-02 栅栏 + 头注释改写）/ emptyexit_test.go（两分叉测）/ perclient_test.go（三形态 + once + ReapedFence）/ events_test.go（session_end 两形态 + quiesce 修复）/ export_test.go（白盒出口）——全部 FOUND
- 提交存在：2c5fd6e（feat）、c44a8d9（fix）、81f4c91（test）——git log 确认 FOUND
