---
phase: 13-resource-defense
plan: 05
subsystem: server-observability
tags: [metrics-series-21, pty-counters, session-active-gauge, help-dual-mode, healthz-d06, session-start-audit, zero-sensitive-values, mirror-17-to-21, OPS-12]
requires:
  - "13-02 metricsCounters 四枚预埋 atomic 字段（ptySpawn/ptySpawnFailures/ptyKills/ptySpawnThrottled——D-08 一次性全加）+ ptySpawnThrottled 递增点已接线"
  - "13-03 session_end 审计 emit（shared schema + client_id 关联键，D-09 前半）+ pcSessions 注册表（session_active per-client 分支数据源）+ WR-02 waitDone 栅栏形态（ptyKills 递增点落位锚）"
  - "08-04 metrics 骨架（17 series exposition / snapshotMetrics 单趟快照 R-07 / metricsSeries17 测试侧镜像）与 08-03 healthz 四字段键集红线"
provides:
  - "/metrics 21 series：四计数器尾部追加（wesh_pty_spawn_total/wesh_pty_spawn_failures_total/wesh_pty_kills_total/wesh_pty_spawn_throttled_total——per-client 出数、shared 恒 0 series 保留不摘）"
  - "wesh_session_active 同名 series 模式分支（D-07：shared 探活 0/1 现状逐字 / per-client = len(pcSessions) 活跃会话计数）+ HELP 文案按模式生成"
  - "三计数器递增点接线（ptySpawn 登记成功点 / ptySpawnFailures spawn 失败分支 / ptyKills teardown 与孤儿回收两 AfterFunc 补 KILL 实际发送点）"
  - "healthz session_active per-client 恒 true（D-06 形态① handler 内模式分支——「会话服务可用」语义诚实；四字段键集红线不变）"
  - "session_start 审计 emit（每次 spawn 成功恰一条：pid + client_id——紧随 attach 之后、startSessionGoroutines 之前；与 session_end 同键闭合单个会话全生命周期串联）"
  - "metricsSeries21 测试侧镜像（17→21 一次性扩展，Phase 11 D-04 既定窗口兑现）+ TestMetricsPerClient/TestPerClientSessionStart/TestSpawnEventsSchema 三断言组"
affects:
  - "13-07（phase13.mjs S5 进程级 scrape 断言：四计数器出数 + session_active 恒 true + 零身份 label——Go 级机制与断言面已就位）"
  - "13-08（收口闸：OPS-12 勾选终验 + diff 白名单审查——perclient_test.go:421 翻转段归 13-CONTEXT D-06 明示翻转点）"
tech-stack:
  added: []
  patterns:
    - "同名 series 按模式分支取值 + HELP 文案按模式生成（D-07：否决另起 series 名——清单膨胀且 shared 侧恒 0 徒增噪音；shared 分支语义逐字不动红线）"
    - "per-client-only 计数器 shared 恒 0 不摘（credit_gate 恒 0 先例——series 在场性是采集方契约，摘除即断流）"
    - "gauge vs counter 取值形态判别（attach N 瞬态等值无判别力——断开一端后 gauge 收敛 1 而 counter 恒 N 就此分叉）"
    - "零成功 spawn 的事件测试构造（burst=1 + 恒败注入：两拒绝事件零会话零 watcher——迟到 emit 面结构性不存在，后继捕获窗不受染）"
key-files:
  created: []
  modified:
    - internal/server/metrics.go
    - internal/server/health.go
    - internal/server/perclient.go
    - internal/server/metrics_test.go
    - internal/server/perclient_test.go
    - internal/server/events_test.go
decisions:
  - "TestSpawnEventsSchema 构造取 burst=1 + spawn 恒败注入：dial ① 耗尽令牌后 spawn 失败（spawn_failed）、dial ② 令牌未补给即节流（spawn_throttled）——两拒绝事件一窗捕获且零成功 spawn（零会话零 watcher = 零迟到 emit 面，13-03 TestAuthFailedNoUsername 跨测试迟写教训的前置规避）"
  - "TestPerClientSessionStart 的 pid 断言取与 spawnedSessions() 实际 PID 相等（shared 母本 TestSessionEnd 同款）——比 plan 文本「正整数」更强（缺省 0/负值与未 emit 同判别档收紧）"
  - "metricsSeries17→metricsSeries21 变量重命名（plan「变量名同步改语义注释」——名字反映条目数，assertExpositionShape 两处引用与行数闸 3*len 机械跟随零逻辑改动）"
  - "ptyKills 恰按 plan 枚举两路径（teardownPCLocked + reapOrphanSession AfterFunc）；13-04 Shutdown 路径的补 KILL AfterFunc（server.go）不在计数面——plan files_modified 白名单不含 server.go 且 must_haves 明示「两路径」，Shutdown 路径 KILL 计数缺口登记为观测面注意项（非缺陷：series 语义仍为「teardown/孤儿回收兜底」，13-07/13-08 复核知悉）"
  - "OPS-12 勾选留 phase 末收口 plan（11-01/12-01/13-01/13-02/13-03/13-04 先例：ID 跨 plan 共享——本 plan Go 级全量证据齐，13-07 phase13.mjs S5 进程级 scrape 断言未落）"
metrics:
  duration: 19min
  completed: 2026-09-05
  tasks: 2
  commits: 2
status: complete
actuals:
  tokens: 9321
  tasks: 2
  commits: 2
---

# Phase 13 Plan 05: OPS-12 观测面 per-client 粒度（metrics/healthz/audit 三面）Summary

/metrics 17→21 series（四 spawn 计数器 + session_active 同名 series 按模式分支 + HELP 双模式文案）+ healthz session_active per-client 恒 true（D-06）+ session_start 审计 emit（pid+client_id，与 session_end 同键闭合全生命周期串联）+ 三计数器递增点接线 + 拒绝事件零敏感值红线扩展。

## What Was Built

**Task 1（feat 1685485，TDD——RED 观察于任务内完成即转 GREEN，11-01/13-02/13-03 先例单 feat 提交）**：

- `metrics.go`：
  - metricsSnap 扩五字段（pcSessions + 四计数器锁内 Load）——snapshotMetrics 单趟 hubMu 持有内 `sn.pcSessions = int64(len(s.pcSessions))`（shared 下 pcSessions 为 nil，len==0 自然成立，同一行两模式零分支；Open Question 1 推荐形态，绝不第二趟取锁——函数体仅一处 hubMu.Lock acceptance 闸验证）
  - metricsHandler 尾部追加四条 writeCounter（17→21，契约序 = 既有 17 条输出逐字不动 + 四条尾部追加）：wesh_pty_spawn_total / wesh_pty_spawn_failures_total / wesh_pty_kills_total / wesh_pty_spawn_throttled_total——per-client-only 出数，shared 恒 0 series 保留不摘（credit_gate 恒 0 先例）
  - session_active 分支（D-07）：per-client → `sessionActive = snap.pcSessions`，HELP = "Number of active per-client PTY sessions."；shared → `sessionAlive.Load()` 探活语义与 HELP 文案逐字不动（既有 0/1 语义红线）
  - label 红线注释（:15-19）扩一句「13-05 红线扩到新四 series」（T-13-16）；metricsCounters doc 注释从「13-02 先立字段」更新为「13-05 接线完成」
- `perclient.go` 三递增点：ptySpawn=upgradePerClient 登记成功点（hubMu 持有内 `s.pcSessions[pc] = struct{}{}` 后——孤儿淘汰不计）；ptySpawnFailures=spawn 失败分支 logEvent 同点；ptyKills=teardownPCLocked 与 reapOrphanSession 两 AfterFunc 补 KILL「实际发送」点（复检/WR-02 栅栏通过后、SignalGroup(SIGKILL) 之前——被栅栏拦截的空转回调不计）
- `metrics_test.go`：metricsSeries17→metricsSeries21 镜像一次性扩展（新四条 counter 追加尾部；行数闸 3*len 自动跟随）+ `TestMetricsPerClient` 双模式断言组——shared_zeros（四计数器恒 0 且 series 在场 + HELP 探活文案逐字）/ per_client_branch（attach 2 → session_active==2 + spawn_total==2 + HELP 会话计数文案；断开一端 → gauge 轮询收敛 1 而 spawn_total 恒 2——活跃计数 vs spawn 累计取值形态判别，等值瞬态无判别力的补强）

**Task 2（feat 5f6324b，TDD 同上）**：

- `health.go`：D-06 形态① handler 内模式分支——per-client → SessionActive 恒 true（「会话服务可用」语义诚实：per-client 无「会话死亡=服务终结」态，会话生灭是每客户端粒度事件，服务可用性变化只经 draining 503 承载；编排探活只看 200/503 status 不受影响，T-13-18 低危 accept）；shared `sessionAlive.Load()` 逐字不动；四字段键集红线（status/clients/max_clients/session_active）不变
- `perclient.go`：session_start emit——`emitEvent(slog.String("event","session_start"), slog.Int("pid", pc.sess.Cmd.Process.Pid), slog.Int64("client_id", cl.attachSeq))`，插点 = 紧随 attach 事件之后、startSessionGoroutines 之前（shared 母本 server.go :597-601「审计事件先于任何连接/会话流量落流」同序，程序序保证事件先于会话流量落流；startedAt 为 pc 既有写一次字段——13-03 session_end duration 数据源同源）；文件头 13-03 落地登记段追加 13-05 闭合登记
- `perclient_test.go:420-422`：断言翻转为恒 true 形态（`if !body.SessionActive` + D-06 落地回指注释；New 本体零 emit 语义保持——session_start 归属 upgradePerClient 每次 spawn 成功）
- `events_test.go` 两断言组：
  - `TestPerClientSessionStart`：exit 42 子进程自死 + readExitClose wire 观测同步边（emit→EXIT 帧直写程序序链）——session_start 恰一、pid==spawnedSessions() 实际 PID（比「正整数」更强的相等断言）、client_id 与 attach 事件同值、session_start 与 session_end 同 client_id（OPS-12 成功准则 3 全生命周期串联）、键集白名单（封套 time/level/msg + event/pid/client_id——零 remote/code 键，shared 母本进程级事件同形）
  - `TestSpawnEventsSchema`：零成功 spawn 构造（per-IP burst=1 + spawn 恒败注入）——dial ① spawn_failed（wire Error "failed to start process" 定值文案逐字）+ dial ② spawn_throttled（wire Error "server is at capacity"——D-04 wire 聚合同串）各恰一、零 attach 事件；两事件键集恰「封套 + 四段 schema（event/remote/code[/remote_user]）」白名单 + code==1011 + remote 127.0.0.1: 前缀；注入敏感值三形态样张（err.Error() 文本 "spawn stub failure" / 路径 "/nonexistent/binary" / errno 形态 "ENOENT=2"/"errno-42"）零出现于捕获全文（SEC-01 红线扩到新事件，T-13-17）

## Commits

| Task | Commit | Type | Summary |
|------|--------|------|---------|
| 1 | 1685485 | feat | metrics 四 series + session_active 模式分支 + HELP 双模式 + 三递增点 + 镜像 17→21（3 文件 175+/21-） |
| 2 | 5f6324b | feat | healthz D-06 恒 true + session_start emit + 零敏感值红线 + :421 翻转（4 文件 198+/14-） |

## Verification Results

- `go build ./...` + `go vet ./...` 零输出（两任务后各跑一次）；GOROOT gofmt（go1.26.3，10-05 收口闸工具）本 plan 全部改动文件 clean（health.go 两处 `//（` CJK 标点接续行提交前补空格归一；perclient_test.go 两处命中为 13-02/13-03 已登记 deferred 存量，零触碰）
- TDD RED 实证：Task 1 TestMetricsPerClient 首跑 FAIL（exposition 51 行 vs want 63 行）；Task 2 TestPerClientSessionStart FAIL（session_start=0）+ TestPerClientNoSessionStartEvent FAIL（session_active=false）——TestSpawnEventsSchema 为既有行为 schema 锁（13-02/11-03 事件面），RED 期即绿属预期
- 定向组：Task 1 `TestMetrics` 全组 -race 绿（TestMetricsExposition/Auth/BuildInfo/Values/PerClient/SnapshotRace）；Task 2 `TestPerClientSessionStart|TestSpawnEventsSchema|TestPerClientNoSessionStartEvent|TestSessionStart|TestHealthz|TestPerClient` -race 全绿（41 测含 13-03/13-04 既有组零回归）
- `time go test -race ./internal/server/ -count=1` 全绿两轮（Task 1 后 96.8s / Task 2 后 94.9s——shared 17 series 既有断言组、13-02 节流组、13-03 终结组、13-04 Shutdown 组零回归）
- 全仓三包快速回归绿（cmd/wesh + proto + pty）
- acceptance grep 闸逐条：snapshotMetrics 函数体恰一处 hubMu.Lock ✓；四 series 输出在场 ✓；递增点 ptySpawn@登记成功点/ptySpawnFailures@失败分支同点/ptyKills@两 AfterFunc 复检后（+13-02 既有 throttled）✓；session_start emit 先于 startSessionGoroutines（region grep 程序序）✓；Task 2 白名单恰四文件（health_test.go 零 diff——shared 既有 healthz 测试零改动）✓；events_test.go diff 纯新增 157 行零删除 ✓；perclient_test.go diff = :420-422 翻转段 + 注释改写 ✓

## Deviations from Plan

None——plan executed exactly as written（无 Rule 1-4 偏差；两 TDD 任务均按 11-01 先例单 feat 提交收口）。

### Plan 措辞与实证的微小出入（不构成偏差）

- TestPerClientSessionStart 的 pid 断言取与 spawnedSessions() 实际 PID 相等（plan 文本「pid 键为正整数」）——相等断言蕴含正整数且比其更强（shared 母本 TestSessionEnd 同款形态），收紧非放宽
- TestSpawnEventsSchema 以单服务端双 dial 构造两拒绝事件（plan 文本未指定构造形态）——零成功 spawn = 零会话零 watcher，迟到 emit 面结构性不存在（13-03 TestAuthFailedNoUsername 跨测试迟写教训的前置规避；TestPerClientSpawnFailure 既有形态是「A 在线 + B 失败」，本测构造更收口）
- server.go:578-580 New 分支注释「/healthz session_active 恒 false 与 wesh_session_active 恒 0 为 11→13 已知中间态，语义裁决归 Phase 13 OQ①②」的窗口期表述已历史化（D-06/D-07 经 handler/快照分支落地，New 本体「不 emit session_start、不置 sessionAlive」两核心主张仍真）——server.go 不在本 plan files_modified 白名单，按白名单纪律零触碰，13-08 收口闸 diff 审查知悉
- ptyKills 计数面 = teardown + 孤儿回收两路径（plan 明示）；13-04 Shutdown 路径补 KILL AfterFunc（server.go）不在计数面——plan 白名单明示两路径，series HELP 文案已如实限定（"teardown and orphan reaping"），运维面若需 Shutdown KILL 计数属 series 语义扩展（13-07/13-08 复核知悉项，非缺陷）

## Requirements Trace

OPS-12 勾选**不随本 plan 执行**（11-01/12-01/13-01/13-02/13-03/13-04 先例：ID 跨 plan 共享）——本 plan 落地观测面机制本体 + Go 级全量证据（/metrics 21 series 双模式 + healthz 恒 true + session_start/end 全生命周期串联 + 零敏感值红线），13-07 phase13.mjs S5 进程级 scrape 断言（四计数器出数 + session_active 恒 true + 零身份 label）与 13-08 收口闸终验未齐；勾选归 phase 末收口 plan。

## Known Stubs

None——全链无桩：四 series 输出/session_active 分支/HELP 双模式/healthz 分支/session_start emit/三递增点均为真实实现；三断言组全绿无 skip。

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-16 (Information Disclosure: metrics 新 series 带身份 label → 基数爆炸 + 泄露) | mitigate | 四新 series 经 writeCounter 零 label（label 红线注释扩到新四 series）+ assertExpositionShape 21 series 形态锁（样本行 name+空格起行——label 集合恒空）+ per-client 明细一律审计日志（client_id 关联检索） |
| T-13-17 (Information Disclosure: session_start/拒绝事件携带敏感值) | mitigate | session_start 键集白名单（event/pid/client_id 数值与关联键——emitEvent 既有通道 log.go:85-89 红线）+ TestSpawnEventsSchema 四段 schema 键集 + 注入错误文本/路径/errno 三形态零出现负断言 |
| T-13-18 (Tampering: healthz session_active 恒 true 语义漂移误读) | accept | D-06 语义诚实论证注释入 health.go（per-client 无「会话死亡=服务终结」态；编排探活只看 200/503 status——draining 分支既有）；文档面归 13-08/Phase 14 |
| T-13-SC (供应链) | mitigate | 零新依赖——atomic/strings.Builder/slog 既有件复用；go.mod/go.sum 零 diff |

## Self-Check: PASSED

- 文件存在：metrics.go（四 series + 分支 + 快照五字段）/ health.go（D-06 分支）/ perclient.go（三递增点 + session_start emit + 头注释更新）/ metrics_test.go（metricsSeries21 + TestMetricsPerClient）/ perclient_test.go（:420-422 翻转）/ events_test.go（两断言组）——全部 FOUND
- 提交存在：1685485（feat）、5f6324b（feat）——git log 确认 FOUND
- 全量 -race 绿（94.9s 最终轮）；shared 既有组零回归（health_test.go 零 diff + TestMetricsExposition/Auth/Values 原样绿）
