---
phase: 13-resource-defense
verified: 2026-09-05T20:25:33Z
status: passed
score: 9/9 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 13: 资源防线与终结语义 Verification Report

**Phase Goal:** per-client 模式下并发进程有硬顶、已认证 churn 打不垮服务端、HUP 免疫进程必被收割、关停覆盖全部存活进程组；--once/--exit-when-empty 触发语义与退出码规则成立（含注册表空迁移第二终结源），metrics/审计达 per-client 粒度，反代身份注入子进程环境
**Verified:** 2026-09-05T20:25:33Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

（来源：ROADMAP.md Phase 13 Success Criteria 1-9 —— 全部为行为依赖型真值，行为证据经本次验证会话独立复跑取得，不采信 SUMMARY 声明）

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 满员 503 闸保留 + hubMu 内复检计数硬不变量（无 ttyd 式 == 闸 + 异步 spawn 窗口超编） | ✓ VERIFIED | server.go:823-826（issueTicketJSON 503）+ :968-972（Attach 守卫 503）双闸在场；perclient.go:199-205 pre-spawn 闸与 :261-266 注册点复检均为 hubMu 持有内 `len(pcSessions) >= s.maxClients` 整数比较（`>=` 非 `==`）；spawnFunc 调用点 :214 在 hubMu 外；超编者经 reapOrphanSession 回收。既有测试 TestPerClientCapacityGate（:534）/TestPerClientTeardownRaceOnce（:1221）锁定不变量（本次定向组全绿） |
| 2 | churn 被 spawn 双令牌桶限速，拒绝在 spawn 前、关闭码避开 1006；churn 负载下 RSS/goroutine/fd 有界 | ✓ VERIFIED | spawnthrottle.go 四常量 8/16/1/4 + 双桶 allow(ip,now) + 15min 惰性过期；perclient.go:180-186 桶判定在 pre-spawn 容量闸之前，拒绝序列 = Error{server_error, capacityMessage} + spawn_throttled 事件 + Close(1011) + ptySpawnThrottled 递增；前端 1011 不触发重连实证（main.ts:1006 case 1011 走终态面板、:1023 仅 1006 → startReconnect；reconnect.test.ts 断言 1011 false）。行为证据：五测组（Throttle/Global/XFF/Expiry/WireForm）-race 绿（本次复跑 1.079s）；`-tags=load` TestChurn 10rps×30s 绿（本次复跑 30.161s，基线差值断言 gor/mem/fd + throttled>0 在场 :605-:644/:712-:718）；phase13.mjs S1（14 连 attach 成功 4/拒绝 10 全 1011 + 三方计数 10==10==10 + XFF 双态）PASS（本次复跑） |
| 3 | HUP 免疫子进程 stop-timeout 到期被 SIGKILL 兜底收割；per-client 默认 5s（option-a 用户裁决） | ✓ VERIFIED | main.go:1304-1309 resolveStopTimeout（per-client && !stopTimeoutSet → 5s）；shared 字面 0 逐字未动（:245）；双源显式位（fs.Visit :572 + TOML :606）；显式 0 warn 分支 :1030。行为证据：TestStopTimeoutResolution 三态组绿（本次复跑）；teardownPCLocked AfterFunc SIGKILL 兜底（perclient.go:596-618，reaped 复检 + waitDone 栅栏双防线）；phase13.mjs S2 零配置实测断开至收割 ≈5.0s + 显式 0 对照泄漏存活 + warn 行 PASS（本次复跑） |
| 4 | 优雅关停覆盖全部存活 per-client 进程组，有界 join 后退出（不等 D-state，不丢 session_end） | ✓ VERIFIED | server.go:1746-1826 per-client 分支：pcSessions 快照（hubMu 内，含残留会话——Pitfall 6）→ 放锁逐组 SignalGroup + AfterFunc 补 KILL（reaped 复检 + WR-02 waitDone 栅栏）→ 有界 join（stopTimeout + shutdownJoinMargin 2s，AfterFunc 兜底 Broadcast + deadline 截断）→ 尾部 Broadcast 唤醒 pcSupervisor → join 到期未清零 D-state 兜底 terminate；shared else 体（:1827-1832）逐字未动。行为证据：四形态测试组（TwoGroups/ResidualGroup/JoinBounded/DeadlineExits）-race 绿（本次复跑 6.357s，含残留会话形态 + session_end==N + 时长双锚）；phase13.mjs S4（SIGTERM 双端 1001 + 双 pgid ESRCH + session_end==2 + 退出 255）PASS（本次复跑）。⚠️ 已知 advisory：WR-01（关停 join 窗口内新 attach 逃逸快照——13-REVIEW.md 评 WARNING 非阻塞，见反模式节） |
| 5 | --once/--exit-when-empty 三形态可退出 + 第二终结源 + 退出 255 与 shared 对齐 + Key Decisions 登记 | ✓ VERIFIED | server.go:1646-1657 pcSupervisor 单例（hubCond 等 `(pcExitReq\|\|exiting) && len(pcSessions)==0` → terminate(last-reaped-code)，termOnce 单点）；New per-client 分支钉死 `go s.pcSupervisor()`（:595）；clients.go 两置位点（立即 :976 + 宽限到期回调 :1009）pcExitReq=true + Broadcast；perclient.go:296-305 宽限取消点 + 门闩清零补齐。行为证据：三形态 + Once 测试组 -race 绿（本次复跑）；phase13.mjs S3 三形态进程级退出码 255 逐值 + 宽限取消跨期存活窗 PASS（本次复跑）；PROJECT.md Key Decisions 两行登记在场（:154 D-01 双默认值 / :155 pcSupervisor 第二终结源 255 对齐——ROADMAP 准则 5 登记要求兑现） |
| 6 | 两时序退出码规则与 shared 逐位对齐（255 / 子进程退出码透传，last-reaped-code） | ✓ VERIFIED | sessionWatcher hubMu 内 pcLastExitCode/pcHasExitCode 置位（perclient.go:539-540）→ pcSupervisor 出循环消费。行为证据：TestEmptyExitPerClientChildFirst（子先死 exit 42 → EXIT{42}+1000 wire 观测 → exitf(42) 恰好一次）+ TestEmptyExitPerClientClientFirst（客户端先断 → exitf(-1) 恰好一次 + exit_when_empty 事件恰一）双绿（本次复跑 8.369s）；进程级 255 由 phase13.mjs S3 承载（PASS） |
| 7 | /metrics per-client 粒度（活跃会话 gauge + spawn/kill 计数器）、零身份 label 红线；/healthz D-06 恒 true | ✓ VERIFIED | metrics.go：21 series（四计数器尾部追加 :190-193）+ session_active 模式分支（per-client = snap.pcSessions 计数语义 / shared 探活逐字不动 :160-167）+ HELP 双模式文案 + 零身份 label（唯一 label = build_info version 既有）；health.go:44-45 per-client SessionActive 恒 true（D-06，四字段键集不变）。行为证据：TestMetricsPerClient 双模式绿 + TestPerClientSessionStart/SpawnEventsSchema 绿（本次复跑）；phase13.mjs S5（四 series 全在 + session_active==1 计数语义 + HELP 双模式 + 零 label 花括号运行时自证 + shared 对照恒 0 series 保留）PASS（本次复跑）。⚠️ 已知 advisory：WR-02（Shutdown 快照路径 KILL 不递增 ptyKills——13-REVIEW.md 评 WARNING，见反模式节） |
| 8 | 审计事件（session_start/session_end/spawn_failed）pid 归因 + client_id 关联 + 零敏感值 | ✓ VERIFIED | session_start：perclient.go:335（pid + client_id，attach 事件后/startSessionGoroutines 前）；session_end：:520-531（exit_code/duration_seconds/client_id/signal 归因——KILL 兜底经 signal=SIGKILL）；spawn_failed：:223-224（定值文案 "failed to start process"，零敏感值）；spawn_throttled 同款定值文案。行为证据：TestPerClientSessionEnd 两形态（exit 42 无 signal 键 / SIGKILL 归因 + client_id 与 attach 同值）+ TestPerClientSessionStart（pid==实际 PID + 全生命周期串联）+ TestSpawnEventsSchema（canary/错误文本/路径/errno 四形态负断言）全绿（本次复跑）；phase13.mjs SEC 自净 28 details 零 token/pid 命中（本次复跑） |
| 9 | WESH_REMOTE_USER 注入（sanitize 后入子进程 env，键名白名单固定）；shared 不注入 | ✓ VERIFIED | 全链：spawn.go whitelistEnv 第三参（非空出键 "WESH_REMOTE_USER="+值 / 空串不出键 :178-179）← StartOptions.RemoteUser（:64/:87）← main.go:1382-1385 生产闭包（startOpts 局部复制 + RemoteUser 赋值——防串台）← server.go SpawnFunc 三参类型（:188/:347）← perclient.go:214 调用点（remoteUser 为 Attach 提取 sanitize 产物直传）。键名代码常量、零 CLI/TOML 面。行为证据：TestEnvWhitelist 三分支 + e2e 双形态绿、TestPerClientRemoteUserEnv 四形态绿（本次复跑）；phase13.mjs S6（per-client 携头 env 回读 alice + NEL 控制字符剥离 carl + shared 对照无键——echo 标记程序序锚定）PASS（本次复跑） |

**Score:** 9/9 truths verified（0 present, behavior-unverified）

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| internal/server/spawnthrottle.go | spawn 双令牌桶存储（四常量 + 双桶 + 惰性过期） | ✓ VERIFIED | 115 行实质实现，四常量 8/16/1/4、allow(ip,now) 时间注入、per-IP 先短路判序；被 server.go New 装配 + upgradePerClient 消费（wired） |
| internal/server/perclient.go | 桶判定挂点 + 三道闸 + session_start/end emit + KILL 兜底 + 三计数器递增点 | ✓ VERIFIED | :180-186 节流判定、:199-205/:261-266 容量闸、:335 session_start、:520-531 session_end、:596-618 KILL 兜底、:311/:227/:616/:667 计数器递增点全部在场 |
| internal/server/server.go | pcSupervisor + Shutdown per-client 分支 + SpawnFunc 三参 + Options 四覆写 | ✓ VERIFIED | :1646-1657 pcSupervisor、:1746-1826 Shutdown 分支、:347/:188 三参、:362-364 四覆写字段 + :453-460 零值兜底 + :591 装配 |
| internal/server/clients.go | maybeExitWhenEmptyLocked per-client 触发端 | ✓ VERIFIED | :976/:1009 两置位点（pcExitReq + Broadcast），shared stopChildLocked 分支并存 |
| internal/server/metrics.go | 21 series + session_active 分支 + HELP 双模式 | ✓ VERIFIED | 四计数器 series :190-193、分支 :160-167、快照单趟 :118 |
| internal/server/health.go | D-06 per-client 恒 true 分支 | ✓ VERIFIED | :44-45，四字段键集不变 |
| internal/pty/spawn.go | StartOptions.RemoteUser + whitelistEnv 第三参 | ✓ VERIFIED | :64/:87/:128/:178-179，空串不出键结构性保证 |
| cmd/wesh/main.go | stopTimeoutSet 显式位 + resolveStopTimeout + warn + 闭包 | ✓ VERIFIED | :86/:245/:572/:606/:1030/:1304-1309/:1382-1386 全部在场 |
| web/uat/phase13.mjs | 六场景协议层 UAT（≥400 行） | ✓ VERIFIED | 769 行，六场景函数 s1-s6 在场（:335/:416/:492/:560/:617/:674），本次复跑 29/29 PASS |
| internal/server/load_test.go | TestChurn churn 负载格 | ✓ VERIFIED | :725，//go:build load 首行、基线差值断言（无绝对上限形态），本次复跑绿 |
| cmd/wesh/main_test.go 等测试面 13 件 | 26 测函数组 | ✓ VERIFIED | 逐名 grep 确认全部存在（见行为抽查表），定向组本次复跑全绿 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| server.go Attach | perclient.go upgradePerClient | :942 `ip := s.proxy.clientIP(r)` → :1110 调用传参（throttleIP） | ✓ WIRED | XFF 换键单点（D-05）：键提取复用 clientIP，事件行 remote==XFF 链首（UAT S1f 断言） |
| clients.go maybeExitWhenEmptyLocked | server.go pcSupervisor | pcExitReq 置位 + hubCond.Broadcast → 谓词重估 → terminate | ✓ WIRED | 两置位点 + supervisor 等待条件 + termOnce 单点闭环 |
| perclient.go sessionWatcher | server.go pcSupervisor | pcLastExitCode/pcHasExitCode + 慢半段 delete 后 Broadcast | ✓ WIRED | :539-540 置位 + :628-630 delete+Broadcast，last-reaped-code 消费链成立 |
| perclient.go 计数器递增点 | metrics.go series 输出 | mc atomic Add → snapshotMetrics 单趟读 → metricsHandler | ✓ WIRED | 四计数器全链接线（throttled :184 / spawn :311 / failures :227 / kills :616+:667） |
| upgradePerClient | internal/pty whitelistEnv | spawnFunc 三参 → StartWithSize → StartOptions.RemoteUser → 出键 | ✓ WIRED | SEC-09 全链五环（含 main.go 闭包局部复制防串台） |
| server.go Shutdown | perclient.go 收割链 | 快照逐组 SignalGroup → watcher 收割 → session_end + delete + Broadcast → pcSupervisor | ✓ WIRED | 有界 join 观测 len(pcSessions)，程序序链保证事件先于退出落流 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| metrics.go wesh_session_active | sessionActive | per-client: len(pcSessions) 快照 / shared: sessionAlive.Load() | ✓ | ✓ FLOWING |
| metrics.go 四计数器 series | snap.ptySpawn* | perclient.go 四递增点 atomic | ✓ | ✓ FLOWING |
| session_end duration_seconds | time.Since(pc.startedAt) | pc.startedAt spawn 时刻写一次 | ✓ | ✓ FLOWING |
| session_start pid | pc.sess.Cmd.Process.Pid | 真实子进程 PID | ✓ | ✓ FLOWING |
| WESH_REMOTE_USER 值 | remoteUser | Attach 提取 sanitizeRemoteUser 清洗产物 | ✓ | ✓ FLOWING（sanitize 剥离经 UAT S6b/Go 测双重实证） |
| healthz session_active | sessionActive | per-client 恒 true（New 期固化 sessionMode） | ✓ | ✓ FLOWING |

### Behavioral Spot-Checks

（全部于本次验证会话独立执行——命令实跑，非转述 SUMMARY）

| # | Behavior | Command | Result | Status |
|---|----------|---------|--------|--------|
| 1 | 构建/静态面 | `go build ./... && go vet ./...` | 零输出 exit 0 | ✓ PASS |
| 2 | stop-timeout 三态（SC3） | `go test ./cmd/wesh/ -run TestStopTimeoutResolution -count=1` | ok 0.004s | ✓ PASS |
| 3 | spawn 双令牌桶五测组（SC2） | `go test -race ./internal/server/ -run 'TestPerClientSpawnThrottle\|Global\|XFF\|Expiry\|WireForm' -count=1` | ok 1.079s | ✓ PASS |
| 4 | 白名单注入三分支 + e2e（SC9） | `go test ./internal/pty/ -run TestEnvWhitelist -count=1 -v` | 4 子测全 PASS | ✓ PASS |
| 5 | 终结语义组：两时序/三形态/once/ReapedFence（SC5/SC6） | `go test -race ./internal/server/ -run 'TestEmptyExitPerClient\|TestPerClientExitWhenEmpty\|TestPerClientOnce\|TestPerClientReapedFence' -count=1` | ok 8.369s | ✓ PASS |
| 6 | Shutdown 四形态（SC4） | `go test -race ./internal/server/ -run 'TestPerClientShutdown' -count=1` | ok 6.357s | ✓ PASS |
| 7 | 观测面 + 注入链组（SC7/SC8/SC9） | `go test -race ./internal/server/ -run 'TestPerClientSessionEnd\|SessionStart\|TestSpawnEventsSchema\|TestMetricsPerClient\|TestPerClientRemoteUserEnv' -count=1` | ok 1.590s | ✓ PASS |
| 8 | churn 负载格 10rps×30s（SC2 资源有界） | `go test -tags=load -run TestChurn -count=1 ./internal/server/` | ok 30.161s（基线差值断言全过） | ✓ PASS |
| 9 | phase13.mjs 六场景进程级 UAT（全 SC） | `node web/uat/phase13.mjs` | **29/29 PASS**（23.1s；S1-S6 全绿 + SEC 自净 28 details 零命中） | ✓ PASS |

### Probe Execution

无 `scripts/*/tests/probe-*.sh` 探针声明（本项目验证形态 = web/uat/phaseNN.mjs 协议层 UAT，已按行为抽查第 9 项执行）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PC-08 | 13-01/13-02/13-07/13-08 | 进程硬顶 + churn 防线 | ✓ SATISFIED | REQUIREMENTS.md [x] + Traceability Complete；三道闸 + 双令牌桶 + churn 负载格 + UAT S1（本报告 Truth 1/2） |
| PC-09 | 13-03/13-04/13-07/13-08 | 第二终结源 + 关停覆盖 + 退出码对齐 | ✓ SATISFIED | [x] + Complete；pcSupervisor + Shutdown 四形态 + UAT S3/S4（Truth 4/5/6） |
| SEC-09 | 13-06/13-07/13-08 | WESH_REMOTE_USER 注入 | ✓ SATISFIED | [x] + Complete；注入链全通 + UAT S6 + shared 对照无键（Truth 9） |
| OPS-12 | 13-03/13-05/13-07/13-08 | metrics/审计 per-client 粒度 | ✓ SATISFIED | [x] + Complete；21 series + session_start/end 串联 + 零 label（Truth 7/8） |

**孤儿需求检查**：REQUIREMENTS.md Traceability 中映射 Phase 13 的恰为上述四条，全部被 plan frontmatter 认领（勾选归 13-08 收口 plan 承载，先例合规）——零 ORPHANED。

### Anti-Patterns Found

本 phase 20 个变更文件扫描：**零 TBD/FIXME/XXX debt-marker、零 placeholder、零空实现、零 console.log 桩**。

已知 advisory 项（13-REVIEW.md 已登记，用户裁定 advisory 非阻塞——本验证在代码中逐项确认属实，如实转录）：

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| server.go + perclient.go | :1746-:1833 / :158-:339 | **WR-01**：Shutdown join 窗口（≤7s）内新 attach 会话逃逸 pcSessions 快照与 KILL 兜底——HUP 免疫子进程理论泄漏面 | ⚠️ Warning（advisory） | 窗口窄 + 需 HUP 免疫子进程叠加；快照时刻全部存活组覆盖已由四形态测试 + UAT S4 证明；建议 Phase 14 注册点补 exiting 复查（review 已给出修法） |
| server.go | :1780 | **WR-02**：Shutdown 快照路径 SIGKILL 不递增 ptyKills（三发送点只计两处）——关停场景 wesh_pty_kills_total 少计 + perclient.go:663 注释「两路径覆盖全部」与三发送点现实漂移 | ⚠️ Warning（advisory） | 观测面少计非防线破口（KILL 本身正常发出）；HELP 文案与两计数路径自洽，注释漂移待修 |
| perclient.go | :600-605 | **IN-01**：teardownPCLocked KILL 回调内重复 `if pc.reaped` 块——恒不可达死代码（编辑残留） | ℹ️ Info | 行为无害（幂等）；建议下轮清理 |
| README.md | :97-98 | **IN-02**：README「5s~10s」对空闲连接形态低估（最坏 ~15s）；CONFIGURATION:174 条件化措辞精确 | ℹ️ Info | 文档精度项，非行为缺陷 |
| spawnthrottle.go | :100-114 | **IN-03**：per-IP map 条目惰性重置但不 delete——trust 开启下键客户端可控的缓慢增长面 | ℹ️ Info | throttle.go 先例同构裁决（≈56B×4096≈230KB 可接受）；增速受全局桶 8/s 约束 |

### Human Verification Required

无——全部 9 条真值均取得行为级证据（Go 命名单测组 + churn 负载格 + phase13.mjs 进程级 UAT 均为本次验证会话独立复跑通过）。本 phase 为协议层/服务端 phase，无视觉/浏览器观感断言面（Playwright 面归 Phase 14）。

### Gaps Summary

无阻塞缺口。9/9 Success Criteria 全部以代码实证 + 行为复跑双重证据成立：

- 三道容量闸 + 双令牌桶：机制、常量、拒绝序列、XFF 换键、惰性过期全部在场并被测试/UAT 锁定；
- stop-timeout 双默认值（per-client 5s / shared 字面 0）+ 显式位 + warn：三态锁定，UAT 实测 ≈5.0s 收割；
- 第二终结源 pcSupervisor + last-reaped-code + Shutdown N 组快照 + 有界 join + D-state 兜底：两时序退出码逐位对齐（42 透传 / -1→255），三形态 + Shutdown 四形态进程级验证；
- metrics 21 series 零身份 label + healthz D-06 + session_start/end 全生命周期串联 + 零敏感值：运行时自证通过；
- WESH_REMOTE_USER 注入链五环全通，shared D-15 收窄零漂移（进程级对照）。

两条 REVIEW warning（WR-01 关停窗口 attach 逃逸 / WR-02 ptyKills 少计）为已登记 advisory 项，不在 SC 字面范围内、不构成目标未达——已如实转录于反模式节，建议随 Phase 14 或后续 code-review 修复轮处置（13-REVIEW.md 已附具体修法）。REQUIREMENTS 四条勾选（PC-08/PC-09/SEC-09/OPS-12）证据链完整，v1.1 进度 10/15 与登记一致。

---

_Verified: 2026-09-05T20:25:33Z_
_Verifier: Claude (gsd-verifier)_
