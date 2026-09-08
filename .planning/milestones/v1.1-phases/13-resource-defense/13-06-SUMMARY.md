---
phase: 13-resource-defense
plan: 06
subsystem: pty-env-injection
tags: [SEC-09, WESH_REMOTE_USER, env-whitelist, spawnfunc-signature, remote-user-sanitize, per-client-only, shared-zero-drift]
requires:
  - "SEC-07 sanitizeRemoteUser 既有清洗（proxy.go:136-141——C0/C1/DEL 剥离 + 128 rune 截断，提取点单一写口）"
  - "11-01 upgradePerClient spawn 调用点 + SpawnFunc 装配挂点（T-10-01c 升档分岔唯一调用语义）"
  - "07-03 AuthHeader 信任闸（D-18/D-20：--auth-header 单一开关，头值只在 trust 开启时被采信）"
provides:
  - "pty.StartOptions.RemoteUser 字段 + whitelistEnv 第三参（非空尾部出键 WESH_REMOTE_USER=、空串不出键——键名白名单固定代码常量由 pty 包单侧定义）"
  - "SpawnFunc 三参签名 func(cols, rows int, remoteUser string)（Options/Server 字段 + upgradePerClient 调用点 + main.go 闭包全链一致）"
  - "main.go 生产闭包 startOpts 局部复制 + RemoteUser 赋值（T-13-21 多客户端串台防线）"
  - "TestEnvWhitelist 三分支扩展（注入可见/空串不出键/e2e 双形态）+ TestPerClientRemoteUserEnv 四形态传递链断言（干净头值/C1 剥离/头缺席空串/trust 关闭头忽略）"
affects:
  - "13-07（phase13.mjs S6 进程级 env 回读断言：Web shell 内 env 可见 WESH_REMOTE_USER + shared 对照无键——Go 级机制与传递链已就位）"
  - "13-08（收口闸：SEC-09 勾选终验 + diff 白名单审查知悉三文件机械加参 + WINDOWS #36）"
tech-stack:
  added: []
  patterns:
    - "空串不出键结构性保证（零值形态非分支判断——shared 零漂移由类型系统承载，D-15 收窄语义不变）"
    - "签名扩散单提交纪律（生产+夹具+断言同 commit——中间态编译红不独立成提交）"
    - "防串台闭包形态：共享 startOpts 局部复制后赋值（每客户端 remoteUser 不同，直改共享字段即 env 串台）"
    - "C1 控制字符头值 U+0085 为 Go http 客户端合法可发边界（C0/DEL 客户端侧 httpguts.ValidHeaderFieldValue 即拒）——sanitize 断言取该边界形态"
key-files:
  created: []
  modified:
    - internal/pty/spawn.go
    - internal/pty/spawn_test.go
    - internal/pty/reap_darwin_test.go
    - internal/server/server.go
    - internal/server/perclient.go
    - cmd/wesh/main.go
    - internal/server/options_test.go
    - internal/server/perclient_test.go
    - internal/server/shutdown_test.go
    - internal/server/metrics_test.go
    - internal/server/events_test.go
decisions:
  - "Task 1/Task 2 TDD 均按 11-01/13-05 先例单 feat 提交收口（RED=编译红观察于任务内完成即转 GREEN——签名扩展类 TDD 的诚实形态）；plan Task 2 步 6「签名扩散与断言同提交」照办"
  - "darwin 双编译闸前移到 Task 2（plan verify 未列）——抓出 reap_darwin_test.go whitelistEnv 两参遗漏（build-tag 文件 Linux 编译面不含，全仓 build/vet/test 均不暴露），独立 fix 提交收口；13-06 起签名扩散类改动 darwin 闸应随任务即跑"
  - "startPerClientServer 默认 spawnFn 升级为生产镜像完整形态（局部复制 + RemoteUser 赋值）——「main.go 生产闭包镜像」注释为契约，生产闭包获 RemoteUser 后镜像必须跟随，否则注释失真（plan diff 白名单外的 3 行裁量，注释真值优先）"
  - "SpawnFunc 签名扩散波及 shutdown_test/metrics_test/events_test 三文件注入点（plan files_modified 未列）——编译必需的机械加参，12-05 export_test.go 先例同构登记 WINDOWS #36"
metrics:
  duration: 23min
  completed: 2026-09-05
  tasks: 2
  commits: 3
status: complete
actuals:
  tokens: 11162
  tasks: 2
  commits: 3
---

# Phase 13 Plan 06: SEC-09 WESH_REMOTE_USER 反代身份注入（per-client env 面）Summary

per-client 模式下 --auth-header 透传用户名经 SEC-07 sanitize 后注入子进程环境变量 WESH_REMOTE_USER——whitelistEnv 白名单第三参扩展（空串不出键）+ SpawnFunc 三参签名全链扩散（server/main/闭包局部复制防串台）+ 双模式测试断言；shared 保持 D-15 收窄语义零漂移。

## What Was Built

**Task 1（feat a09f756，TDD——RED 编译红观察于任务内完成即转 GREEN，11-01/13-05 先例单 feat 提交）**：

- `spawn.go`：
  - StartOptions 加 RemoteUser string 字段（注释载 SEC-09：仅 per-client 分支赋值；值为提取点 sanitizeRemoteUser 清洗产物——pty 包零二次清洗零绕过，本字段即唯一注入通道；零值等价纪律扩展至五字段）
  - whitelistEnv 加 remoteUser string 第三参：非空时产物尾部追加 `"WESH_REMOTE_USER=" + remoteUser`，空串不出键——shared 路径与未携头场景零漂移的结构性保证（零值形态非分支判断）；键名白名单固定 = 代码常量、由本函数单侧定义（SEC-06 防线内聚：StartOptions.Env 通道那类把键名合法性推给调用方的误用面不引入）
  - StartWithSize :81 调用点传 opts.RemoteUser（既有替换式注入注释逐字保持，扩 SEC-09 短注）
- `spawn_test.go`：
  - TestEnvWhitelist 扩展三分支：注入可见（whitelistEnv("", -1, "alice") 含 "WESH_REMOTE_USER=alice" 精确行）/ 空串不出键（零值形态键前缀零命中）/ e2e 双形态（/usr/bin/env 子进程真实输出：RemoteUser="bob" 含精确行 + 阳性对照 / 零值键整行缺席 + 阳性对照——防空串假绿）
  - 既有 (a)/(b) 断言逐字保持（whitelistEnv("", -1) 机械加空串第三参）；其余 7 处 whitelistEnv 调用点机械加第三参（TestStartOptionsTerm/ZeroValueParity/DropPrivilegesIdentityEnv/WhitelistEnvDropUnknownUid 等）

**Rule 3 修复（fix 41d5504，darwin 闸抓漏）**：

- `reap_darwin_test.go:57`：spawnHelper 内 whitelistEnv("", -1) 补第三参空串——darwin-only build-tag 文件不在 Linux 编译面，Task 1 全仓 build/vet/test 均未暴露；GOOS=darwin go vet 闸（Task 2 补跑）抓出后独立 fix 提交

**Task 2（feat 553712e，TDD 同上——RED 编译红 = Options.SpawnFunc 仍两参 vs 夹具三参类型失配，GREEN = 生产+夹具+断言同提交）**：

- `server.go`：SpawnFunc 类型签名第三参 remoteUser string（Options 字段 :347 + Server 字段 :188）+ 两处注释同步（T-10-01c 注记语义逐字保持：SpawnFunc 只许在 upgradePerClient 升档分岔内被调用；新增第三参 = Attach 提取 sanitize 产物 → 生产闭包经 StartOptions.RemoteUser 落子进程 env）
- `perclient.go`：:208 调用点 `s.spawnFunc(h.Cols, h.Rows, remoteUser)`——remoteUser 为 upgradePerClient 既有形参（Attach :953 提取点 s.proxy.remoteUser(r) 产物直传，零新管道）；shared 模式零漂移论证入注释（spawnFunc 恒 nil 不经本调用点）；文件头补 13-06 落地登记
- `main.go`：spawnFunc 闭包签名扩展 + 闭包内 startOpts 局部复制后赋 RemoteUser——T-13-21 防串台论证注释（startOpts 为 run() 共享变量、shared 分支 pty.Start 亦消费，每客户端 remoteUser 不同，直接改共享字段即 A 的用户名落进 B 的子进程 env）
- 测试夹具机械加参（既有断言行零改动，diff 删除行全部为签名机械行 + 三个有意修改块）：
  - options_test.go:18 空闭包占位 `_ string`（inert 纪律保持）
  - perclient_test.go harness：spawnFn 三参签名 + SpawnFunc 追踪包装透传第三参（断言面可捕获）+ 19 处 WithSpawn 注入点 `_ string` + TestNewModeSessContract 内联 SpawnFunc
  - shutdown_test.go（4 处）/ metrics_test.go（1 处）/ events_test.go（3 处）WithSpawn 注入点机械加参（plan files_modified 未列——编译必需，WINDOWS #36 登记）
  - startPerClientServer 默认 spawnFn 升级生产镜像完整形态（局部复制 + RemoteUser 赋值同构——镜像注释契约跟随）
- 新测 `TestPerClientRemoteUserEnv`（perclient_test.go 尾部 13-06 增量段）四形态：
  - 服务端 A（AuthHeader="X-Remote-User"，trust 开）：干净头值 "alice" → spawnFn 捕获 "alice" 原样到达（sanitize 幂等）
  - C1 控制字符头值 "ad\u0085min"（U+0085 NEL——Go http 客户端合法可发、C0/DEL 客户端侧即拒，07-03 前例同款）→ 捕获 "admin"（提取点 sanitizeRemoteUser 剥离证据）
  - 头缺席 → 捕获空串（空串不出键链路由 Task 1 TestEnvWhitelist 承载）
  - 服务端 B（AuthHeader 未配置，trust 关）：携 X-Remote-User: alice 头也被忽略 → 捕获恒空串（D-20 单一信任闸：伪造头不得进 env 面）
  - 断言锚点 = spawnFn 第三参（userCh 缓冲通道捕获，每 dial 恰一次 spawn）；子进程 env 进程级回读归 13-07 phase13.mjs S6

## Commits

| Task | Commit | Type | Summary |
|------|--------|------|---------|
| 1 | a09f756 | feat | StartOptions.RemoteUser + whitelistEnv 第三参扩展——WESH_REMOTE_USER 空串不出键（2 文件 91+/21-） |
| — | 41d5504 | fix | reap_darwin_test.go whitelistEnv 调用点补第三参——darwin 编译闸抓漏（1 文件 3+/3-） |
| 2 | 553712e | feat | SpawnFunc 签名扩散第三参 remoteUser + TestPerClientRemoteUserEnv（8 文件 175+/46-） |

## Verification Results

- TDD RED 实证：Task 1 `go test ./internal/pty/ -run TestEnvWhitelist` 编译红（too many arguments in call to whitelistEnv + unknown field RemoteUser——签名缺口即红）；Task 2 `go vet ./internal/server/` 编译红（options_test 3 参闭包 vs Options.SpawnFunc 2 参类型失配——生产签名缺口即红）
- GREEN 定向：Task 1 TestEnvWhitelist -v 四子测全绿（三分支 + EmptyPathFallback）+ pty 全包 1.8s 绿；Task 2 TestPerClientRemoteUserEnv -race 绿（事件流同步印证 detach remote_user=alice/admin sanitize 产物）
- `go build ./...` + `go vet ./...` 零输出（签名扩散全链一致——两任务后各跑 + Task 2 提交前终跑）
- `time go test -race ./... -count=1` 全仓五包全绿（1m38s：cmd/wesh 1.3s / proto 1.0s / pty 2.7s / server 96.7s / web 1.0s——13-02 节流组/13-03 终结组/13-04 Shutdown 组/13-05 观测组零回归）
- darwin 双编译闸（Task 2 补跑，plan verify 未列）：GOOS=darwin go build ./... + go vet ./... 绿（vet 抓出 reap_darwin_test.go 遗漏 → fix 41d5504 后复跑绿）
- GOROOT gofmt（go1.26.3）：本 plan 全部改动文件/改动行 clean（perclient_test.go 两处命中 = 13-02/13-03 已登记 deferred 存量 :1546/:2018，不在本 plan 改动区零触碰）
- acceptance grep 闸：spawn.go WESH_REMOTE_USER 字面量 3 处（键名代码常量，cmd/wesh 零对应 flag/键 ✓）；server.go SpawnFunc 两字段 remoteUser string ✓；main.go RemoteUser 赋值 ✓；os.Environ() 全量追加形态零命中（:168 LANG/LC_ 白名单过滤循环为既有合法形态——plan 文本「grep os.Environ 零命中」按追加形态语义核验）；options_test 既有断言逐字未动（diff = 1 行机械加参）✓

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] reap_darwin_test.go whitelistEnv 两参遗漏**
- **Found during:** Task 2 darwin 双编译闸（plan verify 未列 darwin 闸，保险补跑）
- **Issue:** Task 1 签名扩散漏掉 darwin-only build-tag 文件（Linux 编译面不含 → 全仓 build/vet/test 均不暴露）
- **Fix:** spawnHelper whitelistEnv("", -1) 补第三参空串 + 注释同步；独立 fix 提交（41d5504）
- **Files modified:** internal/pty/reap_darwin_test.go
- **Commit:** 41d5504

**2. [Rule 3 - Blocking] SpawnFunc 签名扩散波及 plan files_modified 未列的三测试文件**
- **Found during:** Task 2 夹具加参
- **Issue:** shutdown_test.go（4 处）/metrics_test.go（1 处）/events_test.go（3 处）的 WithSpawn 注入点持两参闭包——harness spawnFn 签名扩散后编译必需同步（plan files_modified 仅列 options_test/perclient_test）
- **Fix:** 机械加参 `_ string`（零断言改动，diff 审查逐行核验纯签名行）；12-05 export_test.go 先例同构登记 WINDOWS #36 供 13-08 diff 白名单审查知悉
- **Files modified:** internal/server/shutdown_test.go, internal/server/metrics_test.go, internal/server/events_test.go
- **Commit:** 553712e（并入 Task 2——签名扩散单提交纪律）

### 裁量内微调（不构成偏差级）

- startPerClientServer 默认 spawnFn 升级为生产镜像完整形态（局部复制 + RemoteUser 赋值）而非纯 `_ string` 忽略——「main.go run() 生产闭包的镜像形态」注释是契约性表述，生产闭包获 RemoteUser 后镜像必须跟随否则注释失真；plan「测试夹具机械加参」精神内的注释真值优先裁量（+3 行）
- Task 1 acceptance「grep 'os.Environ' internal/pty/ 零命中」按「全量追加形态」语义核验：spawn.go:168 LANG/LC_ 前缀白名单过滤循环为既有合法形态（非追加），替换式注入纪律 :111 注释逐字未动

## Requirements Trace

SEC-09 勾选**不随本 plan 执行**（11-01/12-01/13-01..13-05 先例：ID 跨 plan 共享）——本 plan 落地注入面机制本体 + Go 级全量证据（whitelistEnv 出键/空串不出键双向断言 + e2e env 双形态 + 传递链四形态断言），13-07 phase13.mjs S6 进程级 env 回读断言（Web shell 内 env 可见 + shared 对照无键）与 13-08 收口闸终验未齐；勾选归 phase 末收口 plan。

## Known Stubs

None——全链无桩：StartOptions.RemoteUser/whitelistEnv 出键/SpawnFunc 三参传递/闭包局部复制均为真实实现；新测两枚全绿无 skip。

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-19 (EoP: WESH_REMOTE_USER 值注入——控制字符/超长值/伪造头值进子进程 env) | mitigate | 值 = 提取点 sanitizeRemoteUser 清洗产物（C0/C1/DEL 剥离 + 128 rune 截断，下游零二次清洗零绕过——本 plan 传递链全程未触碰值本身）+ TestPerClientRemoteUserEnv C1 头值捕获已剥离断言；键名白名单固定代码常量（whitelistEnv 单侧定义）；伪造头值面由 --auth-header 信任闸既有语义承接（07-03） |
| T-13-20 (Tampering: shared 模式误注入——D-15 收窄破坏) | mitigate | 空串不出键结构性保证（whitelistEnv 形参零值形态，TestEnvWhitelist 双向断言 + e2e 双形态）+ ValidateOptions shared×SpawnFunc≠nil 拒绝既有（options_test 断言逐字未动）+ trust 关闭头忽略断言（TestPerClientRemoteUserEnv 服务端 B） |
| T-13-21 (Tampering: 多客户端 env 串台——共享 startOpts 被闭包改写) | mitigate | 闭包内局部复制后赋值（main.go 防串台论证注释 + T-13-21 字面锚定）+ harness 默认 spawnFn 镜像同构；逐端捕获断言由 TestPerClientRemoteUserEnv userCh 通道承载（每 dial 恰一次 spawn 恰一捕获） |
| T-13-SC (Tampering: 依赖面) | mitigate | 零新依赖——全部既有件（slices/http.Header/websocket.DialOptions.HTTPHeader）；go.mod/go.sum 零 diff |

## Self-Check: PASSED

- 文件存在：spawn.go（RemoteUser 字段 + whitelistEnv 三参）/ spawn_test.go（TestEnvWhitelist 三分支）/ reap_darwin_test.go（三参修复）/ server.go（SpawnFunc 三参两字段）/ perclient.go（:208 加参 + 头登记）/ main.go（闭包局部复制）/ options_test.go + perclient_test.go + shutdown_test.go + metrics_test.go + events_test.go（机械加参 + 新测）——全部 FOUND
- 提交存在：a09f756 / 41d5504 / 553712e——git log 确认 FOUND
- 全仓 -race 绿（1m38s）+ darwin 双闸绿 + 既有组零回归（13-02/03/04/05 全部定向组在 -race 全量内）
