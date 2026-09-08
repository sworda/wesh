---
phase: 13-resource-defense
plan: 04
subsystem: server-perclient-termination
tags: [pc-shutdown, n-pgroup-snapshot, bounded-join, broadcast-wakeup, d-state-fallback, PC-09, OPS-12]
requires:
  - "13-03 pcSupervisor 第二终结源 + exiting 消费面（hubCond 等 (pcExitReq || exiting) && len(pcSessions)==0 → terminate(last-reaped-code)）"
  - "13-03 teardownPCLocked 补 KILL 回调形态母本（hubMu 复检 reaped + WR-02 waitDone 非阻塞 select 栅栏双防线）"
  - "13-03 sessionWatcher 收割链（emit → close(waitDone) → delete+Broadcast happens-before 链）"
  - "11-04 trap '' HUP 免疫夹具 + 11-07 waitPgroupESRCH EPERM 容忍探针（本 plan 测试组直接复用）"
  - "13-01 D-01 per-client stopTimeout 默认 5s（KILL 兜底默认开启——本 plan join 上界推导的配置基座）"
provides:
  - "Shutdown per-client 分支填充（server.go :1726-1814）：hubMu 内 pcSessions 快照 → 放锁 → 逐组 SignalGroup + AfterFunc 补 KILL → 有界 join（stopTimeout + shutdownJoinMargin 2s）→ 尾部 hubCond.Broadcast() 唤醒 pcSupervisor"
  - "shutdownJoinMargin 内部常量（2s 余量——join 上界 = stopTimeout + 余量，研究 A1 保守形态定值）"
  - "Shutdown 侧 D-state 兜底 terminate（join 到期未清零时经 termOnce 直接收口——T-13-13 mitigation 完整形态；terminate 调用方三源化）"
  - "shutdown_test.go per-client 四形态测试组（TestPerClientShutdown*：在线双端 / 断开未收割残留 / 有界 join / join 到期无条件退出）+ trapHupLoopArgv 免疫夹具构造件"
affects:
  - "13-07（phase13.mjs S4 进程级断言消费本机制：两 pgid ESRCH + session_end 数==2 + 服务端进程退出——Go 级四形态已齐）"
  - "13-08（收口闸：PC-09 flagged-unverified 终验 + diff 白名单审查——shared else 体逐字零改动证据链就位）"
tech-stack:
  added: []
  patterns:
    - "sync.Cond 超时等待形态（AfterFunc 到期兜底 Broadcast——deadline 截断无超时 cond 等待，Wait 期间锁已释放故兜底回调可取锁）"
    - "快照-放锁-行动三段式（hubMu 内拷贝快照 → 放锁 → 快照上行动——注册表快照先例的 pcSessions 版）"
    - "时长双锚断言（下界证行为真实发生 + 上界证有界——空 if 体旧形态翻车于下界、无界等待形态翻车于上界，两向判别力）"
key-files:
  created: []
  modified:
    - internal/server/server.go
    - internal/server/shutdown_test.go
decisions:
  - "Shutdown 侧 D-state 兜底 terminate 落地（Rule 2——T-13-13 mitigation「有界 join 后无条件经 termOnce 退出（Task 1）」为 threat register 硬要求但 plan behavior/action 四锚点未列 terminate 调用）：drained 形态终结仍归 pcSupervisor（13-03 设计零漂移），仅 join 到期未清零分支 Shutdown 直调 terminate——termOnce 与 pcSupervisor 交汇恰好一次；退出码同 last-reaped-code 规则（无收割缺省 0）"
  - "快照信号循环加 WR-02 waitDone 非阻塞 select 栅栏（研究明示「精确栅栏形态属 Claude's Discretion（WR-02 修法同构）」——按 13-03 planner 裁定「Pitfall 2 语义对一切 kill(-pgid) 同构适用」选栅栏形态，kill-after-reap 面在 Shutdown 路径同构闭合）"
  - "join 实现形态选 hubCond.Wait + AfterFunc 兜底 Broadcast（零新同步件——hubCond 既有消费面扩展；teardown 慢半段 delete 点 Broadcast 即时唤醒，deadline 兜底截断 D-state 无限等待）"
  - "第四测试形态 TestPerClientShutdownDeadlineExits（Rule 2 补测——T-13-13 后半「到期无条件 termOnce 退出」唯一可测证据：stopTimeout=0 + 免疫子进程 = 不可杀 D-state 代理，exitf(0) + kill 0 存活探针双观测）"
  - "夹具落点：复用 perclient_test.go startPerClientServerWithSpawn（11-03 已参数化 spawnFn+mutate）而非参数化 startShutdownServerWith（plan 文本落点）——同包零新装配零重复，plan 意图（per-client 服务端 + srv 句柄 + spawnFn 直通）全保持"
  - "PC-09/OPS-12 勾选留 phase 末收口（11-01/12-01/13-01/13-02/13-03 先例：ID 跨 plan 共享——本 plan Go 级证据齐，13-07 phase13.mjs S4 进程级断言与 13-08 终验未落）"
metrics:
  duration: 13min
  completed: 2026-09-05
  tasks: 2
  commits: 2
status: complete
actuals:
  tokens: 5722
  tasks: 2
  commits: 2
---

# Phase 13 Plan 04: Shutdown N 进程组快照逐组信号 + 有界 join + Broadcast 补行（PC-09 关停覆盖）Summary

Shutdown stop-signal 段 per-client 分支（11-01 空操作占位的既定填充点）：pcSessions 快照逐组 stop-signal + KILL 兜底 + 有界 join（stopTimeout+2s）+ 尾部 Broadcast 唤醒 pcSupervisor——优雅关停覆盖全部存活进程组（含断开未收割残留者，Pitfall 6），join 有界不等 D-state（PITFALLS P10），session_end 零丢失；shutdown_test.go 四形态测试组收口。

## What Was Built

**Task 1（机制，feat fe3ca4a）**：

- `server.go` Shutdown per-client 分支填充（116+/6-，shared else 体逐字零改动）：
  - **快照**：hubMu 内 `pcs := make([]*pcSession, 0, len(s.pcSessions))` 拷贝 → 放锁——快照源 = pcSessions 非 registry（Pitfall 6：断开未收割残留会话只在 pcSessions 在册，1001 广播驱动的 detach→teardown 链路覆盖不了无客户端会话，registry 快照必漏即进程泄漏）
  - **逐组信号**：放锁后循环——WR-02 waitDone 非阻塞 select 栅栏（快照→发信号间隔内 watcher 可能已收割，已关闭即跳过——kill-after-reap 误杀复用 pgid 面结构性闭合，teardownPCLocked 快半段同形态）→ `pc.sess.SignalGroup(s.stopSignal)`（ESRCH 幂等）
  - **补 KILL**：`stopTimeout>0` 每组 AfterFunc 回调（teardownPCLocked :569-585 同形态母本每会话化）：hubMu 内复检 `!pc.reaped` + waitDone 非阻塞 select 双防线后才发 SIGKILL——与 detach 路径武装的兜底计时器双发幂等（后到者必被栅栏拦截）
  - **有界 join**：`bound := s.stopTimeout + shutdownJoinMargin`（新内部常量 2s，研究 A1 保守形态）；AfterFunc(bound) 到期兜底 Broadcast（sync.Cond 无超时等待形态——Wait 期间 hubMu 已释放故兜底回调可取锁无死锁面）+ hubMu 内 `for len(pcSessions)>0 && time.Now().Before(deadline) { hubCond.Wait() }`；teardown 慢半段 delete 点 Broadcast 即时唤醒（13-03 落地）；到期无论收割完毕与否无条件继续——不等 D-state
  - **尾部 Broadcast 补行**：同临界区 `hubCond.Broadcast()` 唤醒 pcSupervisor 重估 `(pcExitReq||exiting)&&len==0`——pcSessions 全空形态下本补行是其唯一唤醒源（入口置位不发信号，watcher 链静止无 Broadcast）；exiting 置位入口原位不动（早于 1001 广播注册表快照，D-13 防线序保持）
  - **D-state 兜底 terminate**（Rule 2，见 Deviations①）：join 到期未清零（`!drained`）时读 last-reaped-code 后 `s.terminate(code)`——T-13-13 mitigation「有界 join 后无条件经 termOnce 退出」完整形态；drained 形态终结仍归 pcSupervisor（13-03 设计零漂移）
  - 注释四要素全载：快照源论证（Pitfall 6）/ 锁序红线（快照 hubMu 内、信号 hubMu 外）/ teardownOnce 双触发幂等论证（1001 detach 链 × Shutdown 快照链）/ 不等 D-state 纪律（PITFALLS P10）；`terminate` doc 注释调用方清单三源化
- verify：`go build ./...` + `go vet ./...` 零输出；GOROOT gofmt（go1.26.3）server.go clean（perclient_test.go 两处命中为 13-02/13-03 已登记 deferred 存量，零触碰）；全量 `-race ./internal/server/` 89.8s 首跑绿（既有 shutdown/stopseq shared 组 + 13-03 落地组零回归 = 机制插入零破坏）；acceptance 四锚点 region grep 逐条过（pcSessions 快照 :1749-1750 / 逐组 SignalGroup :1761 / 有界 join deadline+Wait :1800-1803 / Broadcast :1816）；shared else 体 diff 零触碰（diff 中 `s.sess.SignalGroup`/`time.Sleep` 零出现）

**Task 2（测试组，test e9f34ab）**：

- `shutdown_test.go` 纯新增 280 行零删除（既有 shared 用例断言行零改动——白名单「仅新增函数」形态）：
  - `TestPerClientShutdownTwoGroups`（形态一·在线双端）：两客户端 attach 各独立 sh → 各回读 pid → Shutdown → 两端各收 1001 + server_shutting_down → 两 pgid 各 ESRCH → `exitf(-1)` 恰一次（默认 HUP 信号死亡，last-reaped-code 规则）→ **session_end == 2**（signal=SIGHUP 归因逐事件断言；同步边 = waitExit 收码 ⟹ pcSupervisor 谓词 len==0 已满足 ⟹ emit→close(waitDone)→delete 程序序链全部落流）
  - `TestPerClientShutdownResidualGroup`（形态二·断开未收割残留——Pitfall 6 告警面）：trap 免疫夹具 + StopTimeout=2s 长值（防 KILL 兜底在快照前抢跑）→ 断开 → healthz clients==0（detach 临界区完成同步边——removeLocked 与 teardownPCLocked 同一 hubMu 持有）+ `PCSessionsLenForTest()==1`（**残留在册实证**）→ Shutdown → 时长双锚（≥1s 证 join 真实等待收割——11-01 空 if 体旧形态 ~0ms 返回即翻车；< stopTimeout+余量+护栏 6s）→ exitf(-1) + ESRCH + session_end 恰 1（signal=SIGKILL 归因——KILL 兜底经 watcher 收割链 emit）
  - `TestPerClientShutdownJoinBounded`（形态三·有界 join——T-13-13 前半）：免疫在线 + StopTimeout=500ms → join 等待 KILL 到期收割 → 时长双锚 [250ms, 4.5s]（无界 join 形态翻车于上界——免疫子进程可长期存活）→ exitf(-1) + ESRCH + session_end 恰 1（SIGKILL）
  - `TestPerClientShutdownDeadlineExits`（形态四·join 到期无条件退出——Rule 2 补测，见 Deviations③）：免疫 + StopTimeout=0（零值 = 无补杀通道，「不可被本服务端信号配置杀死」的 D-state 代理）→ join 到期（2s 余量）无条件 terminate → **exitf(0) 恰一次**（无收割 → last-reaped 缺省 0——pcSupervisor 出循环同规则）+ 免疫子进程仍存活（kill 0 探针——退出不拖死实证另一面）+ 时长双锚 [1.5s, 4s]；收口同步边 = 体内 Kill → 收割链排空（PCSessionsLenForTest()==0）后才 restore（13-03 TestAuthFailedNoUsername 迟写修法同款——迟收割 session_end 落本捕获窗不污染后继测试）
  - `trapHupLoopArgv` 夹具构造件（`trap '' HUP; echo TAG=$$; while :; do sleep 10; done`——trap 安装先于 echo，客户端在线期回读 pid 即 trap 就位的同步点，11-03 免落盘纪律；11-04/13-03 同款）
- verify：定向组 `-run 'TestPerClientShutdown' -race -v` 两轮绿（6.36s/6.36s，四测时长 0.22s/2.21s/0.71s/2.22s 与机制分析逐点吻合）；全量 `-race ./internal/server/` 96.5s 绿；全仓四包快速回归绿（cmd/wesh + proto + pty）

## Commits

| Task | Commit | Type | Summary |
|------|--------|------|---------|
| 1 | fe3ca4a | feat | Shutdown per-client 分支：快照/逐组信号+KILL 兜底/有界 join/Broadcast 补行/D-state 兜底 terminate（server.go 116+/6-） |
| 2 | e9f34ab | test | per-client Shutdown 四形态测试组 + trapHupLoopArgv 夹具（shutdown_test.go 280+/0-） |

## Verification Results

- `go build ./...` + `go vet ./...` 零输出；GOROOT gofmt 两改动文件 clean
- `time go test -race ./internal/server/ -count=1` 全绿三轮（Task 1 后 89.8s / Task 2 后 96.5s / 新测定向组复跑 6.36s）——既有 shutdown/stopseq shared 用例、13-03 落地组（TestEmptyExit*/TestPerClientExitWhenEmpty*/TestPerClientSessionEnd 等）零回归
- shared else 分支（`s.sess.SignalGroup` + sleep + KILL 原字面序列）git diff 零触碰；exiting 置位点入口原位不动；1001 广播段零改动
- Task 1 acceptance 四锚点 region grep 全过（快照/SignalGroup/deadline+Wait/Broadcast 限 Shutdown 函数体）；信号发送点在 Unlock 之后（AfterFunc 回调内 Lock 仅为 reaped 复检+栅栏——teardownPCLocked 同形态）
- Task 2 acceptance：四形态全在场且 -race 绿（含残留会话形态）；session_end 数==N 断言在场（2/1/1/1）；Shutdown 耗时上界断言在场（三处双锚，无精确时点断言）；shutdown_test.go diff 纯新增 280 行零删除
- 13-07 phase13.mjs S4 进程级断言（两 pgid ESRCH + session_end 数==2）——同 wave 兄弟 plan 范围，Go 级机制与断言面已就位

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - threat-register mitigation] Shutdown 侧 D-state 兜底 terminate（join 到期未清零分支）**
- **Found during:** Task 1 实现设计（plan must_haves truth「有界 join 后无条件经 termOnce 退出」与 prohibitions「到期无条件 termOnce 退出」× behavior/action 四锚点清单（快照/信号/join/Broadcast，无 terminate）的张力审查）
- **Issue:** plan behavior 的 Shutdown 序列在 join 到期后仅「无条件继续 + Broadcast」——正常 drained 形态 pcSupervisor 收口无碍，但 D-state 不可杀残余使 `len(pcSessions)` 永不归零，pcSupervisor 谓词 `(pcExitReq||exiting)&&len==0` 永假 → 服务端在 Shutdown 返回后挂死——threat register T-13-13（high, mitigate）明示 Task 1 mitigation =「有界 join（stopTimeout+余量）后无条件 termOnce 退出」，该 mitigation 属 correctness requirement 不可缺席
- **Fix:** join 到期未清零（`!drained`）时读 last-reaped-code（hubMu 内，与 pcSupervisor 同规则）后放锁调 `s.terminate(code)`——termOnce 与 pcSupervisor 交汇恰好一次；drained 形态终结仍归 pcSupervisor（13-03「② Shutdown 终结（既有 exiting 位）」设计零漂移）；terminate doc 注释调用方清单三源化。测试不可达路径由形态四锁定（见③）
- **Files modified:** internal/server/server.go
- **Commit:** fe3ca4a

**2. [Rule 2 - 加固] 快照信号循环加 WR-02 waitDone 非阻塞 select 栅栏**
- **Found during:** Task 1 实现（plan behavior「逐组 SignalGroup（ESRCH 幂等——已死 pgid 静默）」无栅栏 vs 13-03 planner 裁定一致性审查）
- **Issue:** 快照（hubMu 内）→ 发信号（放锁后）间隔内 watcher 可能已收割（Wait 已返回、reaped 置位在途）——对已 reap 的 pgid 发信号是 WR-02 同类微窗口（pid 回绕复用理论面；13-03 裁定「Pitfall 2 信号/reap 序列化语义对一切 kill(-pgid) 同构适用」）
- **Fix:** 每组发信号前对 `pc.waitDone` 非阻塞 select（已关闭即跳过）——零成本结构性栅栏，channel 关闭态读零锁安全不违反「信号在 hubMu 外发」红线；研究明示「精确栅栏形态属 Claude's Discretion（WR-02 修法同构）」，按裁定选栅栏形态
- **Files modified:** internal/server/server.go
- **Commit:** fe3ca4a

**3. [Rule 2 - 补测] 第四测试形态 TestPerClientShutdownDeadlineExits**
- **Found during:** Task 2 测试设计（plan 三形态的 D-state 判别力审查——三形态的 join 均经 KILL 兜底在界限内完成，「到期无条件 termOnce 退出」半边无测试面）
- **Issue:** T-13-13 mitigation 后半（到期无条件退出）在 plan 三形态下无证据——KILL 兜底（SIGKILL 不可被 trap 免疫）使一切 stopTimeout>0 场景必在界限内收割，唯一可构造的「不可杀残余」= stopTimeout=0 + HUP 免疫（无补杀通道的 D-state 代理）
- **Fix:** 新增形态四：exitf(0) 恰一次 + 免疫子进程仍存活（kill 0 探针）+ 时长双锚 [1.5s, 4s] + 体内 Kill 收口同步边（迟收割 emit 不污染后继捕获窗——13-03 TestAuthFailedNoUsername 修法同款）；无本 plan 机制（无界 join 或无到期收口）时本测双向翻车（前者 Shutdown 永不返回 / 后者 waitExit 5s 护栏超时）
- **Files modified:** internal/server/shutdown_test.go
- **Commit:** e9f34ab

### Plan 措辞与实证的微小出入（不构成偏差）

- 夹具落点：plan 文本「startShutdownServerWith 参数化 SessionMode/SpawnFunc」→ 实际复用 perclient_test.go `startPerClientServerWithSpawn`（11-03 已参数化 spawnFn+mutate 且返回 srv 句柄，同包 server_test）——零新装配零重复，plan 意图（per-client 服务端 + Shutdown 直调面 + spawnFn 直通）全保持；plan 引用行号（:1614-1621/:1592 等）为 13-03 提交前基点，实际填充点 :1726-1814（13-03 落地 pcSupervisor 后行号整体后移）
- GOROOT gofmt（go1.26.3）doc 注释归一四处提交前处理（`//（` 行首补空格 ×2、`trap ''` → `trap ""` ×2——ASCII 撇号对在 doc 注释被归一为弯引号，ASCII 双引号对不在归一面，perclient_test.go:1126 注释先例同款）

## Requirements Trace

PC-09/OPS-12 勾选**不随本 plan 执行**（11-01/12-01/13-01/13-02/13-03 先例：ID 跨 plan 共享）——本 plan 落地 Shutdown 关停覆盖机制本体 + Go 级四形态证据（在线双端/残留/有界 join/到期无条件退出 + session_end 数==N），13-07 phase13.mjs S4 进程级断言（两 pgid ESRCH + session_end 数==2 + 服务端进程退出）与 13-08 收口闸终验未齐；勾选归 phase 末收口 plan。

## Known Stubs

None——全链无桩：快照/逐组信号/KILL 兜底/有界 join/Broadcast 补行/D-state 兜底 terminate 均为真实实现；四形态测试全绿无 skip。

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-13 (DoS: 无界 join 被 D-state 拖死) | mitigate | 有界 join（stopTimeout+2s AfterFunc 兜底 Broadcast 截断）+ 到期未清零 Shutdown 侧 terminate 无条件收口（Task 1 双件）+ 形态三/四测试锁定（前者证上界、后者证到期收口——D-state 代理 stopTimeout=0 + HUP 免疫） |
| T-13-14 (DoS: 残留会话漏信号 → 进程泄漏) | mitigate | 快照源 = pcSessions 非 registry（Task 1）+ 形态二测试（PCSessionsLenForTest()==1 残留在册实证 + 快照链覆盖 + join 等待收割时长双锚 + session_end 恰 1） |
| T-13-15 (Tampering: 双触发竞态重复信号/重复收口) | mitigate | teardownOnce 幂等承接（既有）+ 快照信号 WR-02 waitDone 栅栏 + 补 KILL 回调 reaped 复检 + waitDone 栅栏双防线（与 detach 路径计时器双发幂等）；termOnce 承接 pcSupervisor × Shutdown 兜底交汇 |
| T-13-SC (供应链) | mitigate | 零新依赖——sync.Cond/hubMu/hubCond/time.AfterFunc 全既有件；go.mod/go.sum 零 diff |

## Self-Check: PASSED

- 文件存在：server.go（per-client 分支填充 + shutdownJoinMargin 常量 + terminate 注释三源化）/ shutdown_test.go（四形态 + trapHupLoopArgv）——全部 FOUND
- 提交存在：fe3ca4a（feat）、e9f34ab（test）——git log 确认 FOUND
- 全量 -race 绿（96.5s 最终轮）；shared else 体逐字未动（diff 证据）；既有测试零改动（shutdown_test.go 纯新增 280 行）
