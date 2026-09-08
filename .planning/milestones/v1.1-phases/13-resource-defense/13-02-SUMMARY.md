---
phase: 13-resource-defense
plan: 02
subsystem: server-spawn-throttle
tags: [spawn-throttle, dual-token-bucket, churn-defense, fork-bomb, xff-keying, lazy-expiry, wire-aggregation, PC-08]
requires:
  - "Phase 11 D-02/D-03 容量三道闸（rejectCapacity 序列母本 + upgradePerClient pre-spawn 闸挂点——本 plan 桶判定插在其之前，容量闸逐字零改动）"
  - "throttleStore mu+map+惰性过期母本（throttle.go——存储件形态平移 + 56B×4096≈230KB 内存界论证）"
  - "x/time/rate v0.15.0 既有直接依赖（perclient.go 输入限速在用——零新依赖）"
  - "proxy.clientIP XFF 换键单点（07-03——桶键同源采信，D-05）"
provides:
  - "spawnThrottleStore 双令牌桶存储（全局 8/s burst 16 + per-IP 1/s burst 4 + 15min 惰性过期 + allow(ip, now) 时间注入判定）"
  - "upgradePerClient pre-spawn 桶判定挂点 + spawn_throttled 拒绝序列（Error{server_error, capacityMessage}+Close(1011)+事件+计数器——wire 聚合日志细分第三次应用）"
  - "Options 四测试覆写字段（SpawnGlobalRate/SpawnGlobalBurst/SpawnPerIPRate/SpawnPerIPBurst——D-03 零 CLI flag/TOML 面）"
  - "metricsCounters 四枚新 atomic（ptySpawn/ptySpawnFailures/ptyKills/ptySpawnThrottled——本 plan 接线 ptySpawnThrottled 递增点，其余三件与 series 输出归 13-05）"
  - "export_test 观测出口（PTYSpawnThrottledForTest 计数器读 + SpawnThrottleProbeForTest 直调薄桥）"
  - "XFF 头注入夹具（dialHelloWithXFF/handshakeCollectUntilCloseWithXFF）——后续 plan 反代场景测试可复用件"
affects:
  - "13-03（churn 场景 UAT 消费本机制与拒绝序列）"
  - "13-05（/metrics series 输出消费本 plan 预埋的四计数器字段与递增点）"
  - "13-07（churn 压测 RSS/goroutine/fd 有界断言以本防线为本体）"
tech-stack:
  added: []
  patterns:
    - "双令牌桶串联判定 per-IP 先短路（单 IP churn 不耗全局预算——D-05 多用户不误伤的判序面）"
    - "wire 聚合日志细分第三次应用（capacityMessage 定值同串 + 事件名/计数器分辨率）"
    - "时间注入直调薄桥（SpawnThrottleProbeForTest——ForTest 四件套纪律，包内私有件的 server_test 直调面）"
    - "判别面构造：per-IP burst=1200 > 15min TTL 补给上限 900（正补给使小 burst 的 allow-结果无判别力——重置与补给同满额，必须大 burst 计数判别）"
key-files:
  created:
    - internal/server/spawnthrottle.go
  modified:
    - internal/server/server.go
    - internal/server/perclient.go
    - internal/server/metrics.go
    - internal/server/perclient_test.go
    - internal/server/export_test.go
decisions:
  - "双桶判序 per-IP 先、全局后（短路）：单 IP churn 过量尝试在 per-IP 桶即拒（AllowN 失败零消耗）不耗全局预算——反代后合法多用户共享全局配额不被单一 churning IP 占干；plan/RESEARCH 蓝本均未定序（「串联判定：任一 Allow 失败即拒」），属实现 latitude 内裁决"
  - "Rule 3：TestPerClientTeardownRaceOnce mutate 放宽 per-IP 桶（10 轮同 IP 连续 attach 超默认 burst 4，第 5 轮起被节流拒绝）——测试对象是 teardown 竞态非 churn 防线，放宽隔离两测试面；断言行零改动"
  - "惰性过期判别面：perIPRate=1/s、burst=1200 > TTL 补给上限 900——无重置实现超窗后 960 令牌 vs 重置 1200（相位三精确计数）；过早重置实现相位二 1200 vs 补给 120 翻车——双向判别"
  - "事件 schema 键集白名单含日志封套（time/level/msg）+ 四段 schema（event/remote/code[/remote_user]）——parseEvents 解析整行 JSON，纯四段白名单必翻车"
  - "PC-08 勾选留 phase 末收口 plan（11-01/12-01/13-01 先例：ID 跨 plan 共享——本 plan 是机制本体，13-03 场景面/13-07 压测面证据未齐）"
  - "GOROOT gofmt（go1.26.3）存量命中两处（cmd/wesh/main_test.go 13-01 遗留 / perclient_test.go:1535 Phase 12 遗留）登记 deferred-items.md——范围外不修"
metrics:
  duration: 50min
  completed: 2026-09-05
  tasks: 2
  commits: 2
status: complete
actuals:
  tokens: 9384
  tasks: 2
  commits: 2
---

# Phase 13 Plan 02: spawn 双令牌桶 churn 防线（PC-08 tracer）Summary

per-client spawn 双令牌桶端到端落地：全局桶（8/s burst 16）收断网惊群 + per-IP 桶（1/s burst 4，XFF 换键）收单点 churn，拒绝序列与容量拒绝同码同串（wire 聚合第三次应用）+ spawn_throttled 事件与计数器分辨率；同 IP 超 burst 收 1011 不触发前端重连放大。

## What Was Built

**Task 1（tracer，feat 46f28ac，TDD——RED 观察于任务内完成即转 GREEN，11-01 先例单 feat 提交）**：

- `internal/server/spawnthrottle.go`（新件 112 行）：四内部常量 defaultSpawnGlobalRate/Burst（8/16）+ defaultSpawnPerIPRate/Burst（1/4，D-03 零 CLI/TOML 面，注释载取值依据）；`spawnPerIPEntry{limiter, lastSeen}` + `spawnThrottleStore{mu, global, perIP map, perIPRate, perIPBurst}`（throttleStore mu+map+惰性过期形态平移）；`newSpawnThrottleStore` 四参数零值兜底（newThrottleStore 先例）；`allow(ip, now) bool`——now 参数化时间注入面（AllowN 双桶同刻注入），惰性过期重置（>15min 满额新桶）+ per-IP 取用 + 双桶串联判定
- `server.go` 五处：Options 四测试覆写字段（InputRate/InputBurst 注释分档同档）；New 零值兜底四行（同位先例）；Server.spawnThrottle 字段（per-client 装配/shared nil——装配期一次分岔）；New per-client 分支 `newSpawnThrottleStore(...)` 装配；Attach 调用点传 :883 既有 `ip`（clientIP 提取点复用，D-05 键单点）
- `perclient.go`：upgradePerClient 签名加 `throttleIP string`（remoteUser 之后）；桶判定插在 pre-spawn 容量闸之前（fork 预算保护在 spawn 调用点之前）——判定失败拒绝序列：Error{ErrServerError, capacityMessage}（与容量拒绝同码同串，D-04）→ logEvent(1011, "spawn_throttled") → Close(StatusInternalError) → `s.mc.ptySpawnThrottled.Add(1)`；容量闸与 spawnFunc 调用点逐字不动（spawnFunc 保持 hubMu 外——region grep 验证）
- `metrics.go`：metricsCounters 追加 ptySpawn/ptySpawnFailures/ptyKills/ptySpawnThrottled 四枚 atomic.Int64（D-08 一次性全加；本 plan 仅接线 throttled 递增点，series 输出归 13-05——本文件除结构体四行+注释外零改动）
- `export_test.go`：PTYSpawnThrottledForTest 观测出口（GateTransitionsForTest 先例——/metrics series 未接线期的计数器观测面）
- `TestPerClientSpawnThrottle`（tracer e2e）：mutate 覆写 per-IP rate=1 burst=2 + 全局放宽 100/100 隔离 → 两次 dialHello 耗尽 burst → 第三次裸握手三通道断言：wire（恰一 Error 帧 ErrServerError + "server is at capacity" 逐字 + close 1011 逐值——Pitfall 3 判别面）/ 事件（spawn_throttled 恰一，remote 127.0.0.1: 前缀 + code 1011 + 无 remote_user；max_clients 零命中——事件名分治）/ 计数器（delta==1）；零 spawn（spawnedSessions==2）零注册（/healthz==2、attach 恰 2）

**Tracer 反馈门**（autonomous: true → 11-01 先例自治形态）：tracer 提交后端到端重跑 verify（build+vet+定向 -race+全量 -race 82s）全绿后放行 Task 2。

**Task 2（扩展测试组，test 202348e）**：

- `TestPerClientSpawnGlobal`（全局桶独立判定，T-13-05）：per-IP 放宽 100/100 + 全局 rate=1 burst=2 + trust 开三 XFF（203.0.113.7/198.51.100.9/192.0.2.11 各自满额 per-IP 桶）→ 前两次放行耗尽全局 burst → 第三次全新 IP 仍 1011——拒绝源只能是全局桶；remote==XFF 值断言（trust 换键入事件）
- `TestPerClientSpawnXFF`（D-05 两态，两 t.Run——TestXFFThrottleKey 先例同构）：trust on——A 桶耗尽（burst=1 二连 1011）不影响 B（异 XFF attach 成功收 Welcome）；trust off——XFF 完全忽略，异 XFF 头两客户端共享 loopback 回退键（任一耗尽皆拒）；误用 XFF 键的错误实现两半边各有翻车面
- `TestSpawnThrottleExpiry`（map 无界增长防线）：SpawnThrottleProbeForTest 直调薄桥 + now 时间注入（零真实等待 15min）——perIPRate=1/s、burst=1200 > TTL 补给上限 900 的判别面构造：三相位精确计数 1200（T0 满额）/120（T0+2min 窗内仅补给——过早重置必翻车）/1200（T0+18min 超窗重置满额——无重置实现 960 必翻车）
- `TestPerClientSpawnThrottleWireForm`（文案定值收口）：Error 帧字节级锁定——逐字节等于 `'E' + {"code":"server_error","message":"server is at capacity"}`（与容量拒绝同串，json.Marshal 固定 schema 确定性产物）；事件键集恰「日志封套 time/level/msg + 四段 schema」白名单（封套外零键）；canary ticket "TICKET-CANARY-x7q9" 携带于 Hello 全程不落 stderr（零敏感值红线可观测承载）；spawnedSessions==1（拒绝零 spawn）
- 夹具：`dialHelloWithXFF`/`handshakeCollectUntilCloseWithXFF`（ws.DialOptions.HTTPHeader 注入，dialHello/handshakeCollectUntilClose 既有形态零改动——变体新增不加参）
- export_test.go：SpawnThrottleProbeForTest 直调薄桥（newSpawnThrottleStore/allow 直通，ForTest 四件套纪律）

## Commits

| Task | Commit | Type | Summary |
|------|--------|------|---------|
| 1 | 46f28ac | feat | 存储件+装配+挂点+拒绝序列+四计数器+tracer e2e（含 Rule 3 teardown 测试桶放宽） |
| 2 | 202348e | test | 全局桶/XFF 两态/惰性过期/文案定值四组扩展 + 直调薄桥 + XFF 夹具 |

## Verification Results

- `go build ./...` + `go vet ./...` 零输出；GOROOT gofmt（go1.26.3，10-05 收口闸工具）本 plan 全部文件 clean（spawnthrottle.go 一行 CJK 标点空格归一后 clean；perclient_test.go/spawnthrottle.go 两文件新增行 clean）
- 定向组 `TestPerClientSpawnThrottle|TestPerClientSpawnGlobal|TestPerClientSpawnXFF|TestSpawnThrottleExpiry` -race 全绿（5 测含两 subtest）
- `time go test -race ./internal/server/ -count=1` 全绿三轮（82s/83s/83s——tracer 后/Rule 3 修复后/Task 2 后；shared 与 per-client 既有组零回归 = 容量三道闸零改动实证）
- 全仓四包快速回归绿（cmd/wesh + proto + pty + web）；`go test ./cmd/wesh/` 绿证 13-01 三态组不受影响
- acceptance grep 闸逐条：spawnthrottle.go 四常量 8/16/1/4 ✓；`cmd/wesh/` 零 spawn flag/TOML 键命中（D-03 零公开面）✓；upgradePerClient 区域 StatusInternalError 常量命中、1006 魔数零命中（注释对前端触发集的引用除外）✓；spawnFunc 调用点在 hubMu Lock/Unlock 区间外（形态承现状）✓；`git diff --quiet HEAD -- cmd/wesh/main.go web/` 零 diff ✓；go.mod/go.sum 零 diff（T-13-SC 零新依赖）✓
- Task 2 白名单闸：perclient_test.go diff 纯新增 298 行零删除（既有断言行零改动；gofmt -w 曾误触一行 Phase 12 存量注释——checkout 重建剔除，存量形态保持）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TestPerClientTeardownRaceOnce mutate 放宽 per-IP 桶**
- **Found during:** Task 1 ⑥（全量 -race 首跑）
- **Issue:** 该测试 10 轮同 IP 连续 attach（每轮 dialHello + exit/close + teardown 收口，轮间 <100ms）超默认 per-IP 桶（1/s burst 4）——第 5 轮起收 Error 帧而非 Welcome，全量闸 FAIL。plan 锁定默认常量（must_haves truth）与「既有测试组原样绿」（verification）两条要求在默认桶下对该测试互斥（plan 侧对既有测试 attach 密度的枚举缺口）
- **Fix:** 该测试 mutate 追加 SpawnPerIPRate/Burst=100（测试对象是 teardown 竞态不变量非 churn 防线——放宽隔离两测试面；节流行为由 TestPerClientSpawnThrottle 专测）。断言行零改动，仅 Options 装配面 +2 行 + 注释
- **Files modified:** internal/server/perclient_test.go
- **Commit:** 46f28ac

**2. [Rule 1 - Bug] WireForm 测试事件键集白名单初版漏日志封套**
- **Found during:** Task 2 首跑（Task 2 内自愈，随 test 提交收口）
- **Issue:** parseEvents 解析整行 JSON 日志——每事件 map 恒含封套键 time/level/msg，纯四段白名单 {event, remote, code} 必翻车（首跑 Fatal「出白名单外键 "level"」）
- **Fix:** 白名单改为「封套 time/level/msg + 四段 schema event/remote/code[/remote_user]」——零敏感值断言语义不变（canary ticket 不落 stderr 仍独立承载）
- **Files modified:** internal/server/perclient_test.go
- **Commit:** 202348e

### Plan 措辞与实证的微小出入（不构成偏差）

- plan action 5「覆写四桶参数小值」：行为行明示「覆写 per-IP 桶参数（rate=1 burst=2 级小值）」——按后者落地为 per-IP 小值 + 全局显式放宽 100/100（隔离判别：全局桶由 TestPerClientSpawnGlobal 专测）
- RESEARCH Code Example 1 蓝本示意 `!global.Allow() || !perIPAllow(ip)` 全局先判——plan 正文（must_haves/behavior）未定序（「串联判定：任一 Allow() 失败即拒」），落地取 per-IP 先短路（判序裁决见 Decisions，蓝本为骨架示意非约束）
- tracer action 5「既有容量闸路径不受影响（桶参数放宽后 max_clients 拒绝仍 1011 同码——两事件名分治断言）」：由三证据承载——tracer 事件流 max_clients 零命中 + 文案字面与 TestPerClientCapacityGate 断言的同一字面量（逐字相同传递闭包）+ 全量 -race 下该测试（默认桶=2 次尝试的宽松面）原样绿；未在本 plan 内复刻 linger 注入（11-03 已锁，避免重复 55 行夹具）

## Requirements Trace

PC-08 勾选**不随本 plan 执行**（11-01/12-01/13-01 先例：ID 跨 plan 共享）——本 plan 落地机制本体（双令牌桶 + 拒绝序列 + 计数器），13-03 场景面 UAT 与 13-07 churn 压测证据未齐；勾选归 phase 末收口 plan。

## Known Stubs

None——全链无桩：双桶判定/惰性过期/拒绝序列/计数器递增均为真实实现；四组扩展测试为真实行为断言。metrics 四计数器中 ptySpawn/ptySpawnFailures/ptyKills 三件为**有意预埋字段**（D-08 一次性全加——递增点与 series 输出归 13-05 接线，plan 明示非桩）。

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-04 (DoS: 已认证 churn fork bomb) | mitigate | per-IP 桶 1/s burst 4 + TestPerClientSpawnThrottle 三通道（churn 压测 RSS/goroutine/fd 有界断言归 13-07 既定） |
| T-13-05 (DoS: 断网惊群) | mitigate | 全局桶 8/s burst 16 + TestPerClientSpawnGlobal 独立判定（全新 IP 满额 per-IP 桶仍拒）；前端 30s 封顶退避既有零改动 |
| T-13-06 (DoS: 1006 重连放大) | mitigate | 1011 锁定（StatusInternalError 常量，region grep 1006 魔数零命中）+ tracer 逐值断言 close code；main.ts:1023 仅 1006 触发实证 |
| T-13-07 (DoS: XFF 桶键不换键误伤) | mitigate | 桶键 = Attach :883 clientIP 产物透传（D-05 同源）+ TestPerClientSpawnXFF 两态（trust on 异键互不影响 / trust off 共享回退键） |
| T-13-08 (DoS: per-IP map 无界增长) | mitigate | 15min 惰性过期 + TestSpawnThrottleExpiry now 注入判别（1200 vs 960 精确计数）；56B/条 × 4096 IP ≈ 230KB 内存界论证平移 |
| T-13-SC (供应链) | mitigate | 零新依赖——x/time/rate v0.15.0 go.mod 既有直接依赖在产线在用；go.mod/go.sum 零 diff |

## Self-Check: PASSED

- 文件存在：internal/server/spawnthrottle.go（四常量 + allow(ip, now)）/ perclient.go（spawn_throttled 拒绝序列 + throttleIP 形参）/ server.go（SpawnGlobalRate 四字段 + 装配）/ metrics.go（ptySpawnThrottled 四枚）/ perclient_test.go（TestPerClientSpawnThrottle + 四扩展组）/ export_test.go（两出口）——全部 FOUND
- 提交存在：46f28ac（feat）、202348e（test）——git log 确认 FOUND
