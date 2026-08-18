---
phase: 03-auth
plan: 04
subsystem: auth
tags: [cli-flags, startup-validation, tls, basic-auth, origin-whitelist, env-fallback, tdd]

requires:
  - phase: 03-02
    provides: server.ParseCredential（user:pass parse 期校验）、server.NormalizeOrigin（小写 host+剥默认端口）、server.TLSConfig（MinVersion 1.2 + 显式 6 AEAD 清单）
  - phase: 03-03
    provides: server.Options{Credentials, Origins, TLS} 装配字段与整站 Basic/ticket/Origin 守卫链消费端
provides:
  - cmd/wesh/main.go：config 六新字段、6 个新 CLI flag（--credential/--tls-cert/--tls-key/--no-auth/--insecure-http/--origin，全名无短选项）、WESH_CREDENTIAL env 兜底（flag 优先）、cert/key 成对 parse 期校验、isLoopbackBind、validateStartup 纯函数（D-03/D-05 八行矩阵逐字文案）、run() ServeTLS 分岔与 scheme 感知启动行、逃生门 stderr 醒目警告
  - cmd/wesh/main_test.go：TestParseArgs 表结构扩展+4 新行、TestCredentialFlagEnv、TestTLSKeyPairError、TestStartupMatrix（八行全覆盖）、TestStartupRefusalNoResource
  - internal/pty/spawn_test.go：TestEnvWhitelist 追加 WESH_CREDENTIAL 双层剥离断言（SEC-06 回归锁）
affects: [03-05 前端, 03-06 文档/UAT（wss 场景前提与 --help 冒烟）]

actuals:
  tokens: 6201
  tasks: 2
  commits: 4

tech-stack:
  added: []（零新增依赖，RESEARCH §Package Legitimacy Audit 纪律）
  patterns:
    - "fs.Func 回调 parse 期校验：--credential/--origin 在 flag 收集回调内经 server.ParseCredential/NormalizeOrigin 即时报错——systemd 配置错误零窗口暴露"
    - "启动校验纯函数分层：parseArgs 管配置形态（cert/key 成对），validateStartup 管部署安全矩阵（D-03/D-05），均无副作用、先于 pty.Start/net.Listen"
    - "run 级 captureFd 拒绝测试：正常快速返回即证明零资源占用（误 listen 挂死变红，TestNoCommandError 同构）"

key-files:
  created: []
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - internal/pty/spawn_test.go

key-decisions:
  - "TestParseArgs 既有表行转命名字段初始化：Go 位置初始化不可扩展字段，命名转换是表结构扩展（+6 want 字段）与既有行值/断言零改动的唯一调和形态"
  - "parse 期校验插入点在 showVersion 早退之后：--version 是纯信息路径，不应被 TLS 成对/env 凭据校验阻断；测试行与 help 冒烟均不受影响"
  - "warn 串自含 `wesh: warning:` 前缀由 validateStartup 返回完整行（plan 字面「中文前缀」按全仓 CLI 输出统一英文的既定形态裁决）；run 仅 Fprintln 透传"
  - "spawn WESH_CREDENTIAL 断言挂既有 TestEnvWhitelist 双层结构（逐键断言形态 → 加单条，plan 授权的最小改法）"

patterns-established:
  - "fs.Func 可重复 flag + 回调内 parse 期校验（配置错误即时报错，收集与校验同点）"
  - "启动矩阵表驱动直调纯函数 + run 级 captureFd 集成断言的双层测试形态"
  - "启动面红线断言形态：矩阵全部行断言 warn/err 不含凭据值样例串（SEC-01 延伸到启动输出）"

requirements-completed: [SEC-01, SEC-04, SEC-05]

coverage:
  - id: D1
    description: "--credential 可重复 flag（fs.Func 回调 parse 期校验）+ WESH_CREDENTIAL env 兜底（flag 优先、env 畸形报错注明来源）+ help 文案 ps 可见性提示（Pitfall 8）"
    requirement: SEC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs(two credentials 行)"
        status: pass
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestCredentialFlagEnv"
        status: pass
    human_judgment: false
  - id: D2
    description: "--tls-cert/--tls-key 成对 parse 期校验 + run() ServeTLS 分岔（TLSConfig 声明式下限复用 03-02 组件）+ 启动行 scheme 感知"
    requirement: SEC-05
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestTLSKeyPairError"
        status: pass
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix(TLS 行)"
        status: pass
    human_judgment: true
    rationale: "wss 真实握手（ServeTLS 路径端到端）无自动化测试——plan verification 节明示 --help/wss 冒烟收进 03-06 UAT；自动化仅覆盖 flag 解析、矩阵 TLS 行与 scheme 分支"
  - id: D3
    description: "--origin 可重复 flag 经 NormalizeOrigin parse 期规范化（大写+默认端口剥离断言、glob 拒绝）入 Options.Origins"
    requirement: SEC-04
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs(origin normalized 行)"
        status: pass
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestTLSKeyPairError(origin glob rejected 行)"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-03/D-05 启动校验矩阵八行全覆盖（两拒绝文案与 RESEARCH Pattern 7 逐字）+ 拒绝路径零资源占用 + 逃生门 stderr 醒目警告 + 启动面红线（输出不含凭据值）"
    requirement: SEC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix"
        status: pass
      - kind: integration
        ref: "cmd/wesh/main_test.go#TestStartupRefusalNoResource"
        status: pass
      - kind: other
        ref: "运行时冒烟：裸跑拒绝 exit=2 逐字文案 / --no-auth 警告含 flag 名 / loopback 免警告（三路实测）"
        status: pass
    human_judgment: false
  - id: D5
    description: "WESH_CREDENTIAL 不进子进程环境（SEC-06 白名单针对回归：防未来改累加式注入，T-03-22）"
    requirement: SEC-01
    verification:
      - kind: integration
        ref: "internal/pty/spawn_test.go#TestEnvWhitelist(WESH_CREDENTIAL 双层断言)"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-08-17
status: complete
---

# Phase 03 Plan 04: CLI 面与启动安全矩阵（6 flag + env 兜底 + D-03/D-05 收口 + ServeTLS 分岔）Summary

**6 个新 CLI flag（--credential/--tls-cert/--tls-key/--no-auth/--insecure-http/--origin）+ WESH_CREDENTIAL env 兜底全部 parse 期校验落地；D-03/D-05 八行启动矩阵经 validateStartup 纯函数把两类危险误配变为显式拒绝（逐字文案、零资源占用）；TLS 经 TLSConfig+ServeTLS 分岔真实可伺服；Phase 1/2 裸跑行为变更完成代码侧收口——全量 -race 绿，TDD RED/GREEN 双循环四提交**

## Performance

- **Duration:** 18 min
- **Started:** 2026-08-17T09:01:27Z
- **Completed:** 2026-08-17T09:19:20Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- CLI 解析面契约锁定：6 个新 flag（全名无短选项，P2 D-15）+ WESH_CREDENTIAL 兜底（flag 非空时 env 整体忽略）+ cert/key 成对与凭据/origin 畸形全部 parse 期报错（TestParseArgs 10 行 + TestCredentialFlagEnv 3 组 + TestTLSKeyPairError 4 行）
- 启动校验矩阵落地：validateStartup 纯函数八行全覆盖，两条拒绝文案与 RESEARCH Pattern 7 逐字一致；拒绝路径先于 pty.Start/net.Listen（TestStartupRefusalNoResource 证明零资源占用）；逃生门放行 stderr 醒目警告且含 flag 名；全部行断言输出不含凭据值（启动面红线）
- run() 装配完成：Options 透传 Credentials/Origins/TLS（TicketTTL/ThrottleBase/ThrottleCap 零值走默认，D-09 标定值即契约）；tlsCert 非空走 hs.TLSConfig=server.TLSConfig()+ServeTLS，启动行 scheme 分支感知
- 行为变更收口（D-03）：默认 bind 0.0.0.0 下 `wesh -- bash` 现拒绝启动（冒烟实测 exit=2 + 逐字文案），迁移路径文档待 03-06
- SEC-06 针对回归锁：TestEnvWhitelist 追加 WESH_CREDENTIAL 双层剥离断言（白名单构造层 + 子进程真实 env 层）

## Task Commits

Each task was committed atomically:

1. **Task 1: parseArgs——6 个新 flag + WESH_CREDENTIAL env 兜底** - `5bb0b9a` (test, RED) → `3d40ac7` (feat, GREEN)
2. **Task 2: run()——启动校验矩阵 + ServeTLS 分岔 + 警示打印** - `e7f330b` (test, RED) → `d64e6cb` (feat, GREEN)

**Plan metadata:** 见最终 docs 提交（本文件之后）

## Files Created/Modified

- `cmd/wesh/main.go` - config 六新字段（逐字段 D-xx 锚点）；parseArgs 注册 6 flag（fs.Func 回调内 ParseCredential/NormalizeOrigin parse 期校验；--credential help 含 ps 可见性提示）+ cert/key 成对校验 + WESH_CREDENTIAL 兜底；isLoopbackBind（空串非 loopback/ParseIP IsLoopback/localhost 特判/其余保守非 loopback）；validateStartup（八行矩阵纯函数）；run() 校验前置 + Options 透传 + scheme 感知启动行 + ServeTLS 分岔
- `cmd/wesh/main_test.go` - TestParseArgs 表结构扩展（+6 want 字段，既有行转命名字段值零改动）+4 新行；TestCredentialFlagEnv（env only/flag 优先/env 畸形三组）；TestTLSKeyPairError（cert/key 单给/credential 畸形/origin glob 四行）；TestStartupMatrix（八行+红线断言）；TestStartupRefusalNoResource（captureFd run 级）；表头 t.Setenv 隔离宿主 WESH_CREDENTIAL
- `internal/pty/spawn_test.go` - TestEnvWhitelist 追加 WESH_CREDENTIAL 双层断言（单元层 whitelistEnv + e2e 层子进程 env 输出）

## Decisions Made

- **TestParseArgs 既有表行转命名字段初始化**：Go 位置初始化不允许缺省字段，表结构扩展（+6 want 字段）与「既有行值/断言零改动」的唯一调和形态是命名转换；值与断言逐字保留，diff 可机械核对
- **parse 期校验插入点在 showVersion 早退之后**：plan 字面「fs.Parse 成功后、argv 校验前」区间内 showVersion 早退居间；--version 是纯信息路径，TLS 成对/env 凭据校验不应阻断它（既有一致性：--version 无命令也放行）
- **warn 串形态**：plan 字面「中文前缀 `wesh: warning:`」自相矛盾，按全仓 CLI 输出统一英文的既定形态裁决——validateStartup 返回自含 `wesh: warning:` 前缀的完整英文警告行（含对应逃生门 flag 名），run 仅 Fprintln 透传
- **spawn 断言最小改法**：现状为逐键断言形态（非表驱动名单），按 plan 授权「逐键断言则加单条」挂入既有 TestEnvWhitelist 双层结构

## Deviations from Plan

None - plan executed exactly as written（上述四项为 plan 字面与实现约束的形态调和，已在 Decisions Made 记录，无 Rule 1-4 自动修复项）。

## TDD Gate Compliance

两任务均 `tdd="true"`，完整 RED→GREEN 双循环：

- Task 1：RED `5bb0b9a`（编译失败：config 无 credentials/tlsCert 等字段）→ GREEN `3d40ac7`（目标测试全过 + build/vet 绿）
- Task 2：RED `e7f330b`（编译失败：validateStartup 未定义）→ GREEN `d64e6cb`（矩阵八行 + run 级拒绝测试全过，`go test -race -count=1 ./...` 全量绿）
- RED 形态为 Go 编译失败（测试引用未存在符号），属 Go TDD 标准 RED；无「意外通过的 RED」
- 例外说明：spawn_test.go 的 WESH_CREDENTIAL 断言对现行替换式注入实现首跑即绿——这是 plan 明示的回归锁（「防未来有人改累加式注入」），非行为驱动测试，随 Task 2 RED 提交
- 无 REFACTOR 提交（实现即终态，无清理需求）

## Issues Encountered

None。运行时冒烟三路（裸跑拒绝 / --no-auth 警告 / loopback 免警告）一次通过。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 03-05（前端）：Options.Credentials/Origins/TLS 已真实供数，整站 Basic 下浏览器弹窗 + fetch ticket 链路可对接；无认证模式 404 探测信号不变
- 03-06（文档/UAT）：`wesh -- bash` 裸跑拒绝的行为变更需 README 迁移路径明示；--help 六 flag 行冒烟与 wss/testssl.sh 场景前提已备（ServeTLS 分岔 + TLSConfig 下限就绪）；systemd EnvironmentFile 600 推荐形态（Pitfall 8）待文档化
- 遗留：本 plan 未自动化 wss 握手（coverage D2 human_judgment=true，归 03-06 UAT）

## Self-Check: PASSED

- 文件存在：cmd/wesh/main.go ✓ cmd/wesh/main_test.go ✓ internal/pty/spawn_test.go ✓
- 提交存在：5bb0b9a ✓ 3d40ac7 ✓ e7f330b ✓ d64e6cb ✓
- 符号锚点：main.go 含 validateStartup ✓；main_test.go 含 TestStartupMatrix ✓；spawn_test.go 含 WESH_CREDENTIAL 断言 ✓
- 近四提交零文件删除 ✓

---
*Phase: 03-auth*
*Completed: 2026-08-17*
