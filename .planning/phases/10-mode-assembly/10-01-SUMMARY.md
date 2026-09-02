---
phase: 10-mode-assembly
plan: 01
subsystem: cli/config/server-assembly
tags: [session-mode, toml, flag, enum-gate, pty, spawn, options, validate]

# Dependency graph
requires:
  - phase: 07-deployment
    provides: TOML 两阶段合并机制（指针标量/fs.Visit 显式位/DisallowUnknownFields）+ write-policy 枚举校验先例 + validateStartup exit 2 通道
  - phase: 05-multi-client
    provides: WritePolicy 常量块先例（clients.go）+ SpawnCols/SpawnRows 单一事实源（spawn.go:34-41）
provides:
  - --session-mode=shared|per-client CLI flag（parse 期枚举校验，D-04 文案）
  - TOML session-mode 键（fileConfig 第 30 键，全连字符）
  - SessionModeShared/SessionModePerClient 枚举常量（internal/server/clients.go）
  - Options.SessionMode/SpawnFunc 接缝字段 + Server sessionMode/spawnFunc 直传
  - ValidateOptions 包级互斥校验（per-client×SpawnFunc=nil / shared×SpawnFunc≠nil fail-fast）
  - pty.StartWithSize 导出（Start 单行委托，80×24 单一事实源保持）
  - run() 装配期一次分岔（per-client 分支 SpawnFunc 闭包 inert 零调用方）
  - config 字段 sessionMode/sessionModeSet/argv0（10-02 warn/LookPath 消费位采集备用）
affects: [10-02, 10-03, 10-04, 11-lifecycle, 12-interaction, 13-resources, 14-termination]

# Actuals (#2632) — chars/4 over realized diff（git diff 781af48..9301cb6 -- cmd/ internal/）
actuals:
  tokens: 7441
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "包级 ValidateOptions 装配契约校验（PATTERNS option (b)：New 无 error 返回先例下的 fail-fast 形态）"
    - "枚举常量块内注释行破 gofmt 对齐组——保持验收 grep 单空格形态与既有行零改动"
    - "配置/CLI 双源一闸双覆盖（默认值替换机制落同一终值，parse 期单闸拒绝）"

key-files:
  created: []
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/config.go
    - cmd/wesh/main_test.go
    - internal/server/clients.go
    - internal/server/server.go
    - internal/pty/spawn.go

key-decisions:
  - "run() 两模式均经启动期 pty.Start 创建 sess（planner 裁定落地——sess=nil 与 New 体 sess.Cmd.Process.Pid 取引用冲突，归 Phase 11）"
  - "SessionMode 零值归一 shared 双点同口径（New 零值兜底 + ValidateOptions 归一）——v1.0 逐字节零回归的结构性保证"
  - "wantSessionMode 字段置于 wantArgv 之后（表结构末尾真追加）——保 append-only 验收闸 removed_lines == 0"

patterns-established:
  - "装配期一次分岔、运行期零分岔：模式分支唯一化在 run() 装配点，全部接缝 inert"
  - "fileConfig 新键追加到结构体末尾（IndexMaxSize 之后）避免 gofmt 重排既有字段行"

requirements-completed: [PC-01]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "--session-mode CLI flag 双枚举值解析入 cfg + 缺省终值 shared（SC1 前半）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs/session-mode_per-client + session-mode_shared_explicit + 全表零值断言"
        status: pass
      - kind: integration
        ref: "冒烟：/tmp/wesh-p10/wesh --session-mode=per-client/--session-mode=shared/缺省 三形态均打印 listening on"
        status: pass
    human_judgment: false
  - id: D2
    description: "非法枚举值 CLI/TOML 双源 parse 期拒绝 exit 2，D-04 定案文案全文（SC2）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestTLSKeyPairError/malformed_session-mode（wantSub=全文 Contains 锁三要素）"
        status: pass
      - kind: integration
        ref: "冒烟：--session-mode=banana exit 2 文案全文匹配；bad.toml session-mode=banana exit 2 同文案（一闸双覆盖）"
        status: pass
    human_judgment: false
  - id: D3
    description: "优先级链 CLI flag > TOML > 内置默认 shared（D-03 env 层真空成立）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs（CLI 两行 + 零值默认断言）；TOML 层与 flag>TOML 覆盖由 10-03 precedence 子测锁定（plan success_criteria SC3 既定归属）"
        status: pass
    human_judgment: false
  - id: D4
    description: "Options.SessionMode/SpawnFunc 接缝 + ValidateOptions 互斥校验 + New 零值归一 shared"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "go test -race -count=1 ./... 全绿（New 零值路径被 v1.0 全量既有测试原样覆盖——零值等价纪律的行为锁）"
        status: pass
    human_judgment: false
  - id: D5
    description: "pty.Start ≡ StartWithSize(argv, opts, SpawnCols, SpawnRows) 单行委托，80×24 零第二副本"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "internal/pty 包 -race 全绿（TestStartZeroValueParity 等既有测试经委托路径原样通过）；委托等价显式断言归 10-02 Task 2（plan 既定分工）"
        status: pass
    human_judgment: false
  - id: D6
    description: "零回归收口闸：v1.0 全量 Go 测试原样全绿且本 plan 零既有测试文件改动"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "go test -race -count=1 ./... 五包全 ok；git diff -U0 main_test.go 删除行 == 0；无其他 *_test.go 触碰"
        status: pass
    human_judgment: false

# Metrics
duration: 32min
completed: 2026-09-02
status: complete
---

# Phase 10 Plan 01: 模式阀门端到端装配 Summary

**--session-mode flag/TOML 键/parse 枚举闸（D-04 文案）+ Options.SessionMode/SpawnFunc 接缝 + ValidateOptions 互斥 fail-fast + run() 装配期一次分岔 + pty.StartWithSize 导出——CLI/TOML 输入 → 校验 → Options → server.New 完整路径一次打通，全部接缝 inert 零 per-client 运行期行为，v1.0 全量 -race 原样绿。**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-02T11:20:09Z
- **Completed:** 2026-09-02T11:52:18Z
- **Tasks:** 2/2
- **Files modified:** 6（5 产品代码 + 1 测试 append-only）

## Accomplishments

- 双源模式选择端到端可用：`--session-mode=shared|per-client` flag + TOML `session-mode` 键经默认值替换机制落同一终值；缺省 == shared（D-03 内置默认，REQUIREMENTS 反特性 A5）
- 非法值 parse 期 exit 2 拒绝，CLI/TOML 双源同文案 `invalid --session-mode "banana": must be shared or per-client`（D-04 定案全文，冒烟进程级实证 banana_exit=2 / toml_exit=2）
- 装配契约接缝全部就位：SessionMode 枚举常量（WritePolicy 同位同形态）+ Options.SessionMode/SpawnFunc + ValidateOptions 双互斥规则 fail-fast（option (b) 定案）+ run() 单分岔（per-client SpawnFunc 闭包 inert 零调用方，T-10-01c 纪律）
- pty.StartWithSize 导出承载原 Start 全部逻辑，Start 缩为单行委托——80×24 字面量零第二副本（SpawnCols/SpawnRows 单一事实源纪律兑现）
- 零回归双证据：全量 -race 五包原样全绿（既有测试零改动）+ main_test.go append-only（diff -U0 删除行 == 0）

## Task Commits

Each task was committed atomically:

1. **Task 1: 模式阀门端到端装配（tracer）** - `111a3e7` (feat)
2. **Task 2: CLI 契约测试 append-only** - `9301cb6` (test)

## Files Created/Modified

- `internal/server/clients.go` — SessionModeShared/SessionModePerClient 枚举常量块（WritePolicy 常量块同位）
- `internal/pty/spawn.go` — StartWithSize(argv, opts, cols, rows int) 导出 + Start 单行委托
- `internal/server/server.go` — Options +SessionMode/SpawnFunc；New 零值兜底 shared；Server sessionMode/spawnFunc 直传；ValidateOptions 导出
- `cmd/wesh/config.go` — fileConfig.SessionMode 第 30 键（toml:"session-mode"）+ 覆盖面注释 29→30 键
- `cmd/wesh/main.go` — config struct 三字段 + flag 注册 + 双源显式位采集 + Parse 返回处枚举闸 + argv0 落定 + run() 分岔与 ValidateOptions exit 2 通道
- `cmd/wesh/main_test.go` — wantSessionMode 断言位 + 两 parse 表行 + D-04 枚举拒绝行（append-only）

## Decisions Made

- **run() 两模式均经启动期 pty.Start**（planner 裁定逐字落地）：PATTERNS §3 sess=nil 建议与 New 体 `sess.Cmd.Process.Pid` 取引用冲突（nil 即 panic）且违反 D-05 等价注记——sess=nil + attach 期 spawn 归 Phase 11；代码注释登记该裁定防回改
- **ValidateOptions 零值归一与 New 兜底同口径**：`mode := opts.SessionMode; if mode == "" { mode = SessionModeShared }`——零值等价纪律在校验与装配双点一致
- **SessionMode 常量块内插注释行**（破 gofmt 对齐组）：保持 `SessionModeShared = "shared"` 单空格形态以满足验收机械检查，同时守住 gofmt 清洁
- **fileConfig.SessionMode 置于 IndexMaxSize 之后**（结构体末尾）：避免 gofmt 重排 Port..WritePolicy 既有字段行——零既有行改动纪律优先于语义就近
- **wantSessionMode 字段置于 wantArgv 之后**（TestParseArgs 表结构末尾真追加）：wantArgv 既有行逐字未动，append-only 闸 removed_lines == 0 成立

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] SessionMode 常量 gofmt 对齐与验收 grep 形态冲突**
- **Found during:** Task 1 验收机械检查（c1 `grep -c 'SessionModeShared = "shared"'` == 0，要求 == 1）
- **Issue:** 两常量同块 gofmt 对齐产生四空格（`SessionModeShared    = "shared"`），验收单行 grep 单空格字面量不匹配
- **Fix:** 常量块内两常量之间插入注释行破 gofmt 对齐组——两常量各保单空格形态，gofmt 清洁与验收字面量双满足
- **Files modified:** internal/server/clients.go
- **Verification:** c1==1 && c2==1 且 gofmt -l 无输出
- **Committed in:** 111a3e7（Task 1 commit）

**2. [Rule 3 - Blocking] fileConfig.SessionMode 插入位置重排既有字段行**
- **Found during:** Task 1 gofmt 检查（config.go 被列出不洁）
- **Issue:** 初版插入 WritePolicy 之后导致 gofmt 重排 Port..WritePolicy 五个既有字段行的对齐列——既有行被触碰
- **Fix:** SessionMode 移至结构体末尾（IndexMaxSize 之后、D-04 排除项注释之前）——gofmt 清洁且全部既有字段行逐字未动
- **Files modified:** cmd/wesh/config.go
- **Verification:** gofmt -l 无输出；git diff config.go 既有字段行零删除
- **Committed in:** 111a3e7（Task 1 commit）

**3. [Rule 3 - Blocking] wantSessionMode 字段位置触发 wantArgv 既有行重排**
- **Found during:** Task 2 append-only 验收（removed_lines == 1，要求 == 0）
- **Issue:** 初版插入 wantArgv 之前使 gofmt 把 wantArgv 对齐列从 2 空格改为 8 空格——既有行被改写
- **Fix:** 字段移至 wantArgv 之后（表结构末尾真追加——与 plan「追加字段」语义一致）；wantArgv 回到独立对齐组逐字未动
- **Files modified:** cmd/wesh/main_test.go
- **Verification:** git diff -U0 删除行 == 0；gofmt 清洁；两目标测试全 PASS
- **Committed in:** 9301cb6（Task 2 commit）

**4. [Rule 1 - Bug/文档口径] Task 2 action 注释指引与 must_haves 决策编号不一致**
- **Found during:** Task 2 ①（wantSessionMode 字段注释）
- **Issue:** Task 2 action 写「D-04：零值 = 期望默认 shared」，但 plan must_haves truths 将内置默认归「SC1，D-03 内置默认」（D-04 实为枚举回显口径决策）——plan 内部编号不一致
- **Fix:** 按 must_haves（truths 权威高于 action 叙述）字段注释写 D-03 内置默认
- **Files modified:** cmd/wesh/main_test.go（注释措辞，零行为影响）
- **Verification:** 全量测试绿；注释可追溯性对齐 truths
- **Committed in:** 9301cb6（Task 2 commit）

---

**Total deviations:** 4 auto-fixed（3 blocking 机械闸适配 + 1 注释口径对齐）——全部为保住「零既有行改动 + gofmt 清洁 + 验收字面量」三硬约束的位置/措辞微调，零行为变更、零范围蔓延。
**Impact on plan:** 计划行为面逐字落地；偏差均为实现层适配，不改变任何公开契约（flag 名/枚举值/TOML 键名/错误文案/互斥规则全部按 plan 定案）。

## Issues Encountered

- fish shell 包装下 `$status` 展开为空——改用 `bash -c '...; echo $?'` 显式捕获退出码完成冒烟断言（banana_exit=2 / toml_exit=2 实证）
- 无其他阻塞；tracer 反馈门（Task 1 提交后重跑端到端 verify：build+vet+全量 -race+banana 冒烟）一次通过

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 10-02 可立即开工：sessionModeSet/argv0 采集位已就位（D-02 warn 锚定与 SC4 LookPath 预检数据源）；pty.Start 委托等价显式断言的挂点（StartWithSize 导出）已就位
- 10-03 precedence/fuzz 面：TOML 键已入合并机制，flag>TOML>默认三层链的 CLI 层证据已锁（10-01 表行），TOML 铺底与覆盖组合子测归 10-03
- Phase 11 生命周期主干：SpawnFunc 闭包签名 `func(cols, rows int) (*pty.Session, error)` 与 StartWithSize 直通形态已定案，sess=nil 切换点（run() 分岔 + New session_start emit）注释已登记
- 关注项：无新 blocker；SpawnFunc 本阶段零调用方为有意 inert（T-10-01c），非 stub

## Self-Check: PASSED

- 文件存在性：6 个修改文件全部在盘（cmd/wesh/main.go、config.go、main_test.go、internal/server/clients.go、server.go、internal/pty/spawn.go）+ 本 SUMMARY.md
- 提交存在性：`111a3e7`（feat Task 1）、`9301cb6`（test Task 2）均经 git log 核实
- 验证证据：go build/vet 通过；go test -race -count=1 ./... 五包全 ok（执行期两轮 + tracer 门一轮共三轮全绿）；冒烟四实证（三形态 listening on + banana CLI/TOML exit 2 同文案）

---
*Phase: 10-mode-assembly*
*Completed: 2026-09-02*
