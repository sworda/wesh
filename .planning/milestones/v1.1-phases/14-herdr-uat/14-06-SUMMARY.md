---
phase: 14-herdr-uat
plan: 06
subsystem: server-test-matrix
tags: [dual-mode, test-harness, deployment-surface, audit, sc1]
requires:
  - "14-01 harness 小族四形态（newTestServer/newTracked/newHandle/newSess）"
provides:
  - "部署面 9 测双模式同断言双跑（sharetoken 2 子测 + customindex 8 子测 + proxy_e2e 4 测 + basepath 3 测）"
  - "server 包 34 文件三维归类收口核对清单（SC1 完成判定面）"
  - "D-02 偏差登记第 4-6 则（events/log 单跑判定 + TestReadLimitBoundary 补登）"
affects: []
tech-stack:
  added: []
  patterns:
    - "白盒镜像双分支（package server 文件经本地 mode 参数化镜像小族两分支——包墙结构性不可达的 D-01 同构形态）"
    - "装配调用点按用途分流（stderr 同步边消费 → newTrackedTestServer；丢弃 → newTestServer）"
key-files:
  created:
    - .planning/phases/14-herdr-uat/deferred-items.md
  modified:
    - internal/server/sharetoken_test.go
    - internal/server/tls_test.go
    - internal/server/customindex_test.go
    - internal/server/basepath_test.go
    - internal/server/proxy_e2e_test.go
    - internal/server/proxy_test.go
    - .planning/phases/14-herdr-uat/14-CONTEXT.md
key-decisions:
  - "startShareServer 白盒镜像双分支：per-client 侧 New(nil)+SessionModePerClient+SpawnFunc 生产闭包镜像（含 RemoteUser 局部构造）+spawned Kill+Close Cleanup——harness 小族（server_test 包）对 package server 白盒文件包墙结构性不可达，D-01 同构形态本地镜像"
  - "customindex 五处原零值 Writable 子测试不显式回写 false：各子测试断言集对 Writable 无观测面（HTTP 伺服/404/gzip/头同源），基线 true 与原零值行为等价——14-05 TestLogRedaction 未回写同款口径（Pattern 9 仅在 Writable 语义被断言处触发）"
  - "TestXFFThrottleKey mode 循环提升至测试顶层（newServer/postBadCreds 闭包捕获 mode，两 trust 子测嵌套其内）——避免逐子测重复包裹"
  - "PC-13 勾选留 14-09（共享 ID 先例：pw 观感层承载——14-08 已登记同口径，本 plan 仅双跑矩阵面贡献）"
metrics:
  duration: 28min
  completed: 2026-09-06
  tests-added: 0
  tests-passing: "全量 -race 五包绿；-v 全包 mode= 子测试 216 PASS"
status: complete
actuals:
  tokens: 19202
  tasks: 2
  commits: 2
---

# Phase 14 Plan 06: mode-agnostic 部署面批 + 三维归类收口核对 Summary

部署面四文件 9 测（sharetoken/customindex/proxy_e2e/basepath）双模式同断言双跑落地——sharetoken 为 package server 白包镜像双分支（包墙不可达小族的 D-01 同构形态），tls/proxy 两文件 5 测实测纯函数形态保持单跑并注释登记；server 包 34 文件三维归类收口核对清单完成（18 文件 81 测双跑 / 216 mode= 子测试 -race PASS），events/log 单跑与 14-01 TestReadLimitBoundary 例外经 14-CONTEXT.md D-02 第 4-6 则续登。

## What Was Built

### Task 1: 部署面前批（commit 843832b）

- **sharetoken_test.go**：`startShareServer(t, mode, opts)` 白盒镜像双分支——shared = 原 `pty.Start + New(sess,...)` 形态逐字保留；per-client = `New(nil, noop-exitf, opts{SessionMode, SpawnFunc})` + 生产闭包镜像（`pty.StartOptions{Uid:-1,Gid:-1}` + `RemoteUser` 赋值——SEC-09 注入链）+ `spawned` mu 保护追踪 + Cleanup 逐一 Kill+Close（perclient_test.go:60-106 母本同构）。"/s/ 门禁" 与 "/api/attach token 分支" 两子测试 for-mode + t.Run 双跑（4 装配调用点传 mode）；"lookup 矩阵" 纯白盒子测试保持单跑。per-client 列实测零 spawn（/s/ 门禁 HTTP-only）与 2 spawn/实例（token 分支双 dial），spawn per-IP 桶 burst 4 内零节流。
- **customindex_test.go**：8 处 `startTestServerWith` 收编 `newTestServer` + :214 `startBasePathServer(t, mode, mutate)`，八子测试双跑；断言体原样搬入两列。
- **basepath_test.go**：`startBasePathServer(t, mode, mutate)` 参数化直传 newTestServer（mutate 先于 `o.BasePath="/wesh"` 强制覆写——原 helper 覆写语义等价保留）；:61/:190 过渡态传字面 `server.SessionModeShared`（与改造前行为逐字等价，Task 2 替换）。
- **tls_test.go**：纯函数判定登记（httptest.NewUnstartedServer + securityHeaders 中间件直测，零 wesh server 装配可参数化——蓝本 tls 行实测偏差），保持单跑，仅文件头注释行。

### Task 2: 部署面后批 + 收口核对（commit 67f11c6）

- **proxy_e2e_test.go**：四测双跑；5 处调用点按用途分流——TestRemoteUserLogging(:46) 与 TestShareChannelRemoteUser(:218/:281) 消费 waitHandlers stderr 同步边 → `newTrackedTestServer`；TestXFFThrottleKey(:127) 与 TestAuthHeaderNoAuthBypass(:179) 丢弃 → `newTestServer`。TestXFFThrottleKey 的 mode 循环提升至测试顶层（newServer 闭包捕获 mode）。XFF 信任闸/remote_user sanitize 红线断言两列同值逐字未动。
- **basepath_test.go**：三测包 mode 循环替换过渡态字面量；TestBasePathEmptyUnchanged 装配收编 `newTestServer(t, mode, argv, nil)`（原 `{Writable:true}` = 基线逐字等价）。
- **proxy_test.go**：纯函数判定登记（sanitizeRemoteUser/proxyInfo 三测零装配——:16 的 startTrackedServerWith 串为注释提及非调用），保持单跑，仅注释行。
- **14-CONTEXT.md D-02 续登三则**（追加第 4-6 则，不改写 14-04 已登三则）：④ events_test 9 测保持单跑（6 shared 形态装配测 + 3 per-client 形态 Phase 13 承载 schema 面）；⑤ log_test 1 测同判定；⑥ 14-01 TestReadLimitBoundary 结构性例外补登（pre-listen stty 装配与 attach 期 spawn 不等价）。
- **deferred-items.md** 新建：TestMaxClients503/mode=per-client 隔离复跑 flake 登记（见 Deferred Issues）。

## 三维归类收口核对清单（SC1 完成判定面）

**实测口径**：`ls internal/server/*_test.go` = **34 文件**（plan 文本「33 文件」= 研究期计数，harness_test.go 系 14-01 执行期落地的小族本体文件、零 Test 函数、非三维归类对象——34 = 33 + 1 的计数差，蓝本表（PITFALLS :378-399）成表时该文件不存在，非归属偏差）；Test 函数合计 152。双跑存量 = **18 文件 81 测**，`-race -count=1 -v` 全包 mode= 子测试 **216 PASS**（含 mode= 路径下嵌套子测）。

| # | 文件 | 测数 | 归类桶 | 蓝本行对照（PITFALLS :378-399） | 落地/登记 |
|---|------|-----|--------|--------------------------------|-----------|
| 1 | e2e_test.go | 8 | ① 双跑（7 测；TestHelperProcess 为 exec 子进程桩，非归类测试） | :384 双模式断言分叉 | 14-02 |
| 2 | multi_test.go | 11 | ① 双跑（owner 四测 D-03 四段式未装配列） | :389 + D-02 第 1 则（owner 归属修正：实在本文件非 clients_test） | 14-02 |
| 3 | exit_test.go | 2 | ① 双跑 | :385 EXIT 广播分叉 | 14-01 tracer |
| 4 | stopseq_test.go | 2 | ① 双跑 | :387 | 14-01 |
| 5 | health_test.go | 2 | ① 双跑（srv 面 newHandleTestServer 首证 TestHealthzDraining） | :393 | 14-01 |
| 6 | slowclient_test.go | 2 | ① 双跑（TestGlobalCredit per-client 列 D-03 满即踢退化锁） | :390 | 14-01 |
| 7 | limits_test.go | 5 | ① 双跑 4 测 + TestReadLimitBoundary shared 单跑（D-02 第 6 则结构性例外） | :382 + 例外 | 14-01 |
| 8 | emptyexit_test.go | 7 | ① 双跑 | :386 | 14-03 |
| 9 | shutdown_test.go | 4 | ① 双跑 2 测（TestShutdown1001/StopTimeout 归一映射）+ ② per-client-only 2 测（ResidualGroup/DeadlineExits，14-03 裁决无 shared 对照面） | :387 | 14-03 |
| 10 | metrics_test.go | 5 | ① 双跑（TestMetricsValues 放大比分叉） | :393 | 14-03 |
| 11 | handshake_test.go | 7 | ① 双跑（TestReadOnlyAllowsResize 为 handshake 行唯一分叉面） | :382 | 14-05 |
| 12 | keepalive_test.go | 3 | ① 双跑 | :382 | 14-05 |
| 13 | auth_e2e_test.go | 9 | ① 双跑（:403 收编 newTrackedTestServer 第二消费方） | :383 auth* | 14-05 |
| 14 | resize_arb_test.go | 1 | ① 双跑（D-03 显式断言未装配可证伪列） | :388/:391 + D-02 第 2 则（resize wire 面实测归属即本文件） | 14-04 |
| 15 | sharetoken_test.go | 1 | ① 双跑（白盒镜像双分支，4 调用点） | :383 sharetoken | **14-06 本 plan** |
| 16 | customindex_test.go | 1 | ① 双跑（8 普通 + 1 bp 调用点） | :383 customindex | **14-06 本 plan** |
| 17 | proxy_e2e_test.go | 4 | ① 双跑（5 调用点按用途分流 tracked/普通） | :383 proxy*（e2e 半侧） | **14-06 本 plan** |
| 18 | basepath_test.go | 3 | ① 双跑（helper mode 参数化直传小族） | :383 basepath | **14-06 本 plan** |
| 19 | perclient_test.go | 36 | ② per-client-only 单模式（per-client 断言本体宿主） | :398 新增 per-client-only 行 | 11-13 |
| 20 | load_test.go | 8 | ③ load 面（//go:build load 隔离；双模式各至少一轮 = shared 格 + churn/pc 格，非同测双跑） | :397 | 14-07 |
| 21 | clients_test.go | 3 | ④ 纯白盒单跑（registry/writer 直构造零装配） | :391 + D-02 第 1/3 则 | 14-04 判定 |
| 22 | resize_test.go | 3 | ④ 纯函数单跑（arbitrate() 直测零装配） | :388 + D-02 第 2 则 | 14-04 判定 |
| 23 | auth_test.go | 2 | ④ 纯白盒单跑 | :383 auth*（白盒半侧） | 14-05 判定 |
| 24 | origin_test.go | 2 | ④ 纯函数单跑 | :383 origin | 14-05 判定 |
| 25 | throttle_test.go | 1 | ④ 纯白盒单跑 | :383 throttle | 14-05 判定 |
| 26 | tickets_test.go | 1 | ④ 纯白盒单跑 | :383 tickets | 14-05 判定 |
| 27 | exitmsg_test.go | 3 | ④ 纯白盒单跑 | :392（exitmsg 部分） | 本 plan 收口登记 |
| 28 | options_test.go | 1 | ④ 纯白盒单跑 | :392（隐含） | 本 plan 收口登记 |
| 29 | proxy_test.go | 3 | ④ 纯函数单跑（本 plan 注释登记） | :383 proxy*（白盒半侧） | **14-06 本 plan** |
| 30 | tls_test.go | 2 | ④ 纯函数形态单跑（本 plan 注释登记——蓝本 tls 行实测偏差：零 wesh 装配可参数化） | :383 tls | **14-06 本 plan** |
| 31 | events_test.go | 9 | ⑤ 事件面单跑（D-02 第 4 则：6 shared 形态 + 3 per-client 形态 Phase 13 承载 schema 面） | :392（events 部分） | 本 plan 登记 |
| 32 | log_test.go | 1 | ⑤ 事件面单跑（D-02 第 5 则，:69 同形态） | :392（log 部分） | 本 plan 登记 |
| 33 | export_test.go | 0 | ⑥ 纯 helper 零测试桶（ForTest 导出桩） | 研究期关键结构事实① | 本 plan 收口登记 |
| 34 | harness_test.go | 0 | 装配基础设施（小族本体，非归类对象——plan 33 计数 = 34 − 本文件） | —（蓝本成表时不存在） | 14-01 落地 |

**包外（⑦ 桶）**：cmd/wesh 3 文件 22 测（config/fuzz/main）+ internal/pty 5 文件 21 测（io/reap×2/signal/spawn）+ internal/proto 2 文件 6 测——无 server 装配者天然 mode-agnostic 单跑；config/fuzz 已 Phase 10 扩展 session_mode 键语料与红线；reap_darwin 双平台面由 CI macOS leg 承担（蓝本 :394/:395 行落地形态）。蓝本 :399 行（UAT phaseNN.mjs）非 Go 测试面——phase11/12/13.mjs 既有 + phase14.mjs（14-08）+ run-all 矩阵（D-04）承载。

**蓝本逐行对照结论**：:378-399 全部 17 行命中或经 D-02 登记偏差处置（第 1-6 则覆盖 owner 归属/resize 分置/计数不变量分置/events·log·TestReadLimitBoundary 单跑判定与 tls 纯函数偏差），零静默漏网。

## Verification Results

- **全量 -race 五包绿**：`time go test -race -count=1 ./...` → cmd/wesh 1.3s / proto 1.0s / pty 2.7s / server 157.5s / web 1.0s 全 ok（Task 1 后一次 + Task 2 全部改动后一次，两轮全绿）。
- **双模式子测试**：`go test -race -count=1 -v ./internal/server/ | grep -cE '--- PASS: .*mode='` = **216**（非零，含 mode= 路径嵌套子测）；本 plan 新增 17 个 mode= 包裹（sharetoken 2 + customindex 8 + basepath 3 + proxy_e2e 4）。
- **期望值零改写自审**：`git diff -w` 逐文件核对——改动面仅装配收编（startTestServerWith/startTrackedServerWith → 小族）、mode 循环包裹与调用点传参；全部断言行仅缩进变化（`git diff -w` 下消失），零期望值字面改动。
- **gofmt/vet**：GOROOT gofmt -l internal/ cmd/ 零输出；go vet ./... 零输出。
- **CI 结构零改动**：`.github/workflows/ci.yml` 零 diff 确认（D-01 终态——单 step `go test -race -count=1 -v ./...` 经 t.Run 双跑天然双模式覆盖）。

## Deviations from Plan

### Plan-Sanctioned Resequencing（非偏差，落地顺序说明）

**Task 1 携带 basepath helper 参数化**：plan Task 1 action 3 明示 ":214 startBasePathServer 调用点随 Task 2 的 helper mode 参数化传 mode（两任务同 plan 串行，本任务落地时一并改）"——customindex :214 调用点编译依赖 helper 签名，故 Task 1 提交内一并落地 `startBasePathServer(t, mode, mutate)`，:61/:190 两调用点过渡态传字面 `server.SessionModeShared`（与改造前行为逐字等价，测试体断言零改动），Task 2 提交替换为 mode 循环变量。两次提交各自编译自洽、全绿。

### Auto-fixed Issues

**1. [Rule 1 - 编译错] newTestServer 二值形态误写三值赋值**
- **Found during:** Task 2 proxy_e2e 转换（TestXFFThrottleKey/TestAuthHeaderNoAuthBypass）
- **Issue:** 沿用 startTrackedServerWith 三值接收形态写 `_, wsURL, _ := newTestServer(...)`，vet 报 assignment mismatch（:139/:179 两处按 plan 分流至二值普通形）
- **Fix:** 改为 `_, wsURL := newTestServer(...)`；提交前修复，无独立提交

### 判定口径说明（Pattern 9 未触发）

customindex 五处原零值 Writable 子测试（relative assets/gzip/安全头×2/empty page）未按 14-05 Pattern 9 显式回写 `o.Writable = false`：各子测试断言集（HTTP 伺服/404/gzip 双态/头同源/空页）对 Writable 无观测面，基线 true 与原零值在本文件断言集下行为等价——与 14-05 TestLogRedaction（原零值 Writable、断言集无观测面、未回写）同款口径；Pattern 9 仅在 Writable 语义被断言处（TestNoAuthMode/TestReadOnly* 先例）触发。git diff 自审确认该口径下零断言期望值改动。

## Deferred Issues

**TestMaxClients503/mode=per-client 隔离复跑时序 flake（14-02 遗留，与本 plan 无关）**：`go test -count=N -run 'TestMaxClients503'`（非 -race、-run 过滤形态）高概率失败——cE 槽位释放重 attach 得 1011 capacity Error 帧而非 Welcome（multi_test.go:1309）。根因疑为 pcSessions linger 窗口竞态（registry ③位槽位随 detach 即释放，但 pre-spawn 容量再闸在 A 会话收割完成前仍见满员；轮询重试只覆盖 HTTP 503 形态）。**基线提交 9af7ce1 临时 worktree 隔离复跑 3/3 失败实证与本 plan 改动无关**（multi_test.go 零改动、-run 过滤下本 plan 测试体不执行）；全量 -race 套件（CI 同款命令）两轮全绿不受影响。按 executor scope boundary 超范围不修，登记 `.planning/phases/14-herdr-uat/deferred-items.md`（含三条修复方向），14-12 收口闸知悉。

## Requirement Status

PC-13 保持 flagged-unverified——共享 ID 先例（11-01 起）：herdr driving scenario 的 pw 观感层（14-09）为最终承载（14-08 已登记同口径勾选归属）；本 plan 贡献其双模式验证矩阵基础面（部署面双跑 + SC1 收口核对），不单独勾选。

## Self-Check: PASSED

- 文件存在性：sharetoken/tls/customindex/basepath/proxy_e2e/proxy_test.go 六文件 + 14-CONTEXT.md + deferred-items.md 全部 FOUND（git ls-files 核对）
- 提交存在性：843832b（Task 1）/ 67f11c6（Task 2）均在 main 历史中 FOUND
- 验证命令复跑：全量 -race ./... 绿 / mode= 计数 216 / vet+gofmt 零输出 / ci.yml 零 diff
