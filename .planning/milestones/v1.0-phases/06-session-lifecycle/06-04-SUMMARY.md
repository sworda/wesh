---
phase: 06-session-lifecycle
plan: 04
subsystem: cli
tags: [go, flag, cli, lifecycle, tdd, one-way-contract]

requires:
  - phase: 06-session-lifecycle/06-02
    provides: Options.ExitWhenEmpty/ExitWhenEmptyGrace 两字段 + New 装配（本 plan 的接线消费点）+ OQ1 accept-255 裁决（下游 06-06/06-07 按此落地）
  - phase: 05-multi-client
    provides: --max-clients flag 与 503 计数拒绝路径（D-08——--once 第二客户端拒绝复用，409 不复活）+ fs.Visit writePolicySet 显式判定先例（05-03）
provides:
  - "--once BoolVar（D-12 语法糖 ≡ --max-clients=1 --exit-when-empty=0，help 单行标明等价关系）"
  - exitEmptyValue 类型（flag.Value + IsBoolFlag 可选值惯例，GOROOT flag.go:350-356）——--exit-when-empty[=duration] 三形态：不写=不开启 / 裸写=立即退出 / =duration=重连宽限
  - fs.Visit maxClientsSet/exitEmptySet 显式设置判定 → parse 期语法糖展开（只填未显式位）→ validateStartup 两组合冲突行（锚定显式设置位，双 flag 名进文案，exit 2）
  - "run() Options 接线 ExitWhenEmpty: cfg.exitEmpty.set / ExitWhenEmptyGrace: cfg.exitEmpty.grace（06-02 空触发机制消费；--once 展开后同通道）"
  - TestParseArgs 三新行 + 错误表两行 + TestStartupMatrix 四新行（两拒绝 + 两一致冗余放行）
affects: [06-06, 06-07, phase-07]

actuals:
  tokens: 4877
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "可选值 flag 形态：flag.Value + IsBoolFlag（GOROOT flag.go:350-356 官方惯例）——裸写 ≡ =true 不消费下一参数，空格分隔形态结构性排除；Set 内 \"true\" 分支特判裸写"
    - "语法糖分层纪律：fs.Visit 显式设置位判定 → parse 期展开只填未显式位（用户显式值保持）→ validateStartup 锚定显式设置位判组合矛盾（不依赖展开不变量，自证性更强，review #3）"
    - "one-way CLI 契约确认门 → TDD 落地形态：checkpoint:decision 锁定公开契约后，RED（表行+矩阵行先行）→ GREEN（注册+展开+校验+接线）两提交"

key-files:
  created: []
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go

key-decisions:
  - "D-12/D-14 确认门 as-locked（用户 2026-08-23 裁决）：--once BoolVar（≡ --max-clients=1 --exit-when-empty=0，README 标明等价关系，第二客户端拒绝走既有 503 计数路径）+ --exit-when-empty[=duration]（自定义 flag.Value + IsBoolFlag 惯例：裸写=立即退出、=duration=重连宽限、不写=不开启；空格分隔形态不传值）——与 06-CONTEXT.md D-12/D-14 逐字一致"
  - "IsBoolFlag 落码形态调和：plan 要求『注释逐字引 GOROOT flag.go:350-356』与验收 grep IsBoolFlag==1 并立——逐字引文作 func 行尾注释（grep -c 按行计数，同行单行使两约束同时满足）；类型 doc 注释转述不含字面量"
  - "RED 已知伪影登记：错误表两行（=abc/=-5s）在 RED 阶段因 flag 包『flag provided but not defined: -exit-when-empty』报错串含 flag 名而意外通过——GREEN 后错误改由 Set 产生本质锁定，无语义混淆（fail-fast 规则核査：特性实质缺失由 TestParseArgs 三行 flag-not-defined 失败与 TestStartupMatrix 两拒绝行 err=nil 失败确证）"

patterns-established:
  - "可选值 CLI flag 三形态落地：exitEmptyValue{set, grace} 两字段分离（set=开启位、grace=0 合法显式值——06-02 Options 同构），String() 未开启返回 \"\" 使 PrintDefaults 不显示 default 标注"
  - "显式设置位冲突矩阵行形态：config{once:true, maxClientsSet:true, ...} 直构造 + wantErrSub2 双 flag 名断言；一致冗余放行（=1 / grace 0 显式同给）以行为锁防误判"

requirements-completed: [SESS-01, SESS-02]

coverage:
  - id: D1
    description: "--once 语法糖展开（maxClients=1 + exitEmpty set/grace 0）与 --exit-when-empty 三形态解析（不写/裸写/=30s）；defaults 行零值断言扩展（once=false、exitEmpty.set=false）"
    requirement: SESS-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs（once_sugar_expands / exit-when-empty_bare / exit-when-empty_grace + defaults，-race）"
        status: pass
    human_judgment: false
  - id: D2
    description: "非法/负值 duration parse 期拒绝（=abc / =-5s，文案含 exit-when-empty；d<0 检查是负值唯一闸——ParseDuration(\"-5s\") 解析成功）"
    requirement: SESS-02
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestTLSKeyPairError（exit-when-empty_bad_duration / negative_duration，-race）"
        status: pass
    human_judgment: false
  - id: D3
    description: "--once × 显式矛盾值组合冲突 fail-fast（显式 --max-clients≠1 / 显式 grace≠0 拒绝，双 flag 名进文案）+ 一致冗余放行（显式 =1 / 裸写 grace 0）"
    requirement: SESS-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix（四新行，-race）"
        status: pass
      - kind: other
        ref: "二进制冒烟：wesh --once --max-clients=2 / --once --exit-when-empty=5s 均 exit 2 且文案含双 flag 名（validateStartup 先于 pty.Start 零资源占用）"
        status: pass
    human_judgment: false
  - id: D4
    description: "cfg → server.Options 接线（ExitWhenEmpty/ExitWhenEmptyGrace 直传解析产物，06-02 空触发机制消费；--once 展开后同通道——服务端无 --once 概念）"
    requirement: SESS-01
    verification:
      - kind: other
        ref: "验收 grep ExitWhenEmpty: cfg.exitEmpty.set == 1 + go build/vet 退出 0 + 五包 -race 全绿（06-02 服务端机制既有测试零回归）"
        status: pass
    human_judgment: false
  - id: D5
    description: "确认门 as-locked 落地：两 one-way CLI 公开契约与 06-CONTEXT.md D-12/D-14 逐字一致（flag 名/语义/等价关系零调整）"
    requirement: SESS-01
    verification:
      - kind: other
        ref: "Task 1 用户裁决（2026-08-23 as-locked，execute 续跑 prompt 确认）+ 本文件记录 + STATE decisions"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-08-23
status: complete
---

# Phase 06 Plan 04: --once / --exit-when-empty CLI 契约（SESS-01/02 CLI 半侧）Summary

**SESS-01/02 CLI 投影落码：--once BoolVar 语法糖（≡ --max-clients=1 --exit-when-empty=0，fs.Visit 显式位判定后展开只填未显式位）+ --exit-when-empty[=duration] 可选值 flag（exitEmptyValue 实现 flag.Value + IsBoolFlag，GOROOT flag.go:350-356 惯例使空格分隔形态结构性不传值）+ validateStartup 两组合冲突行（锚定显式设置位，双 flag 名进文案）+ Options 接线 06-02 消费点——TDD 两提交，TestParseArgs 18 行 / TestTLSKeyPairError 7 行 / TestStartupMatrix 17 行与五包 -race 全绿**

## 确认门结果（Task 1，checkpoint:decision）

用户裁决 **as-locked**（2026-08-23，execute 续跑 prompt 确认）——按锁定值落地，零调整：

- `--once`（BoolVar 全名无短选项，P2 D-15）：≡ `--max-clients=1 --exit-when-empty=0` 语法糖，help 文案单行标明等价关系；第二客户端拒绝走既有 503 计数路径（409 单客户端门不复活，D-12）
- `--exit-when-empty[=duration]`：自定义 flag.Value + IsBoolFlag 惯例（GOROOT flag.go:350-356 源码核实）——裸写 = 最后一个客户端断开立即退出、=duration = 重连宽限、不写 = 不开启（默认现状保持）；空格分隔形态（`--exit-when-empty 30s`）不传值（IsBoolFlag 惯例副产品，README 用法行由 06-07 明示 = 号形态）
- 与 06-CONTEXT.md D-12/D-14 逐字一致；one-way 公开契约纪律保持（发布后改名/改语义破坏部署脚本与文档）

## Performance

- **Duration:** 12 min（不含确认门等待）
- **Started:** 2026-08-23T06:52:39Z
- **Completed:** 2026-08-23T07:04:25Z
- **Tasks:** 2/2（Task 1 确认门 → as-locked；Task 2 TDD 实现）
- **Files modified:** 2（cmd/wesh/main.go + cmd/wesh/main_test.go）

## Accomplishments

- `exitEmptyValue` 类型（main.go 未导出）：`set bool; grace time.Duration` 两字段分离（grace=0 合法显式值，与 06-02 Options set/grace 分离同构）；`String()` 未开启返回 ""（PrintDefaults 不显示 default 标注——不写即不开启的零配置形态）；`IsBoolFlag() bool { return true }`（行尾注释逐字引 GOROOT flag.go:350-356——裸写 ≡ =true 不消费下一参数，`--exit-when-empty 30s` 空格分隔形态结构性排除）；`Set(s)` 三分支——"true" → set+grace 0（裸写=立即退出）；否则 time.ParseDuration，`err!=nil || d<0` 报错（**d<0 检查是负值拒绝的唯一闸**——`ParseDuration("-5s")` 解析成功返回 -5s，review 驳回项的行为锁）；错误直接 return error 含 flag 名与类别（duration 值非敏感，T-06-04a——flag 包包装串回显值可接受，不走 credErr/clientOptErr 记录式）
- 两 flag 注册：`--once` BoolVar（help 单行标明等价关系 ≡ --max-clients=1 --exit-when-empty=0）+ `--exit-when-empty` fs.Var（help 明示 = 号形态与裸写语义）；parseArgs 文档注释 15→17 个 flag 同步
- fs.Visit 扩展 `maxClientsSet`/`exitEmptySet` 显式设置判定（write-policy 159-163 同款闭包形态）；语法糖展开插入 fs.Visit 之后、showVersion 早退之前——只填未显式位（显式给定不覆盖，T-06-04c），分层纪律注释登记（parse = 形状与展开、validate = 组合矛盾）
- validateStartup 两组合冲突行（write-policy 行同位，loopback 早退之前判定，纯函数零副作用、exit 2）：`--once && maxClientsSet && maxClients != 1` 与 `--once && exitEmptySet && exitEmpty.grace != 0`——**判定锚定显式设置位而非展开后终值**（review #3 吸收，注释登记与展开形态的逻辑等价性论证）；一致冗余（显式 =1 / 显式裸写 grace 0）放行
- run() 的 server.New Options 字面量加 `ExitWhenEmpty: cfg.exitEmpty.set, ExitWhenEmptyGrace: cfg.exitEmpty.grace` 两键（06-02 消费点接线；注释登记 --once 展开后同通道——服务端无 --once 概念，SESS-01 = maxClients=1 + ExitWhenEmpty grace 0 组合语义）
- 测试扩展：TestParseArgs 表结构加 `wantOnce`/`wantMaxClients`/`wantExitEmptySet`/`wantExitEmptyGrace` 四命名字段（03-04 转换先例，零值语义照 wantWritePolicy——wantMaxClients 零值 = 期望默认 32；既有行值/断言零改动）+ 三新行（--once 展开 / 裸写 / =30s）；TestTLSKeyPairError 加 =abc / =-5s 两错误行（值非敏感，forbiddenSub 置空）；TestStartupMatrix 加四行（两拒绝含双 flag 名 wantErrSub2 断言 + 两一致冗余放行行——D-12 等价关系的行为锁）

## Task Commits

1. **Task 1: 确认门（--once / --exit-when-empty CLI 公开契约，D-12/D-14 one-way）** — 无提交（checkpoint:decision，用户裁决 as-locked，结果记录于本文件与 STATE decisions）
2. **Task 2 RED: 失败测试先行** — `080795b` (test)——TestParseArgs 四字段+三新行、错误表两行、TestStartupMatrix 四新行 + 最小编译脚手架（config 四字段 + exitEmptyValue 类型声明，无行为）；RED 实证：三新行 flag-not-defined 失败 + 两拒绝行 err=nil 失败
3. **Task 2 GREEN: 两 flag + 展开 + 矩阵 + 接线** — `3e5d3e6` (feat)——exitEmptyValue 方法集 + 两注册 + fs.Visit 扩展 + 展开 + validateStartup 两行 + Options 接线；全测试转绿，无 REFACTOR 需要

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS）

## Files Created/Modified

- `cmd/wesh/main.go` — config 四字段（once/exitEmpty/maxClientsSet/exitEmptySet，Phase 6 分组成段注释 + one-way 纪律标注）+ exitEmptyValue 类型与方法集 + 两 flag 注册 + fs.Visit 两判定 + 语法糖展开段 + validateStartup 两冲突行 + Options 接线两键 + parseArgs 文档注释同步
- `cmd/wesh/main_test.go` — TestParseArgs 四命名字段 + 三新行 + 断言段扩展；TestTLSKeyPairError 两错误行；TestStartupMatrix 四新行；三函数文档注释同步

## Verification Evidence

- `go build ./... && go vet ./...` 退出 0（time 包装，0.6s）
- `go test -race -count=1 ./cmd/wesh/` 全绿——TestParseArgs 18 行（含三新行）/ TestTLSKeyPairError 7 行（含两新行）/ TestStartupMatrix 17 行（含四新行）逐行 PASS，既有行零回归
- 全量回归：`go test -race -count=1 ./...` 五包全绿（cmd/wesh 1.0s / proto 1.0s / pty 2.0s / server 50.3s / web 1.0s）
- 验收 grep 全过：`"once"`==1（BoolVar 注册）；`exit-when-empty`==18≥3；`IsBoolFlag`==1（func 行尾注释同行承载逐字引文）；`maxClientsSet|exitEmptySet`==8≥4；`cfg.maxClientsSet && cfg.maxClients != 1`==1 且 `cfg.exitEmptySet && cfg.exitEmpty.grace != 0`==1（review #3 显式设置位形态钉死）；`ExitWhenEmpty: cfg.exitEmpty.set`==1（Options 接线）；测试文件 `wantExitEmptyGrace|wantOnce`==9≥2
- 二进制冒烟（零资源占用拒绝路径）：`--once --max-clients=2` → exit 2 文案含双 flag 名；`--once --exit-when-empty=5s` → exit 2 文案含双 flag 名；`--exit-when-empty=abc` → flag 包包装串 `invalid boolean value "abc" for -exit-when-empty: invalid --exit-when-empty duration "abc": must be a non-negative duration (e.g. 30s)`（IsBoolFlag 使包装串用 boolean 措辞，含 flag 名；duration 值回显=T-06-04a accept 形态实证）
- `--help` 输出：`-exit-when-empty` 不带值占位符（IsBoolFlag 形态正确），`-once` help 单行含等价关系逐字
- TDD 门禁合规：test 提交 `080795b`（RED，实证失败）→ feat 提交 `3e5d3e6`（GREEN）次序正确；无 REFACTOR 需要
- 触碰文件 GOROOT gofmt 全净（/usr/bin/gofmt 陈旧问题不适用——本 plan 零漂移引入）

## Decisions Made

- **确认门 as-locked**（见上节）——两 one-way CLI 公开契约按 06-CONTEXT.md D-12/D-14 锁定值逐字落地
- **IsBoolFlag 注释落位调和**（plan 两约束机械调和）：plan 要求「注释逐字引 GOROOT flag.go:350-356」而验收 grep `IsBoolFlag==1` 禁止第二行出现该字面量——逐字引文作 func 行尾注释（grep -c 按行计数，同行单行使两约束并立），类型 doc 注释转述惯例语义不含字面量
- **RED 错误表两行已知伪影**（TDD fail-fast 规则核査记录）：=abc/=-5s 两行在 RED 阶段因 flag 包「flag provided but not defined: -exit-when-empty」报错串含 flag 名而意外通过——特性实质缺失由 TestParseArgs 三新行（flag not defined → t.Fatalf）与 TestStartupMatrix 两拒绝行（err nil）失败确证，非「特性已存在/测试测错」；GREEN 后错误改由 Set 产生，断言本质锁定

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] IsBoolFlag 逐字引文改作 func 行尾注释**
- **Found during:** Task 2（GREEN 实现，验收 grep 预检）
- **Issue:** plan action 要求「注释逐字引 GOROOT flag.go:350-356 惯例」，但 GOROOT 原文含字面量 "IsBoolFlag"，独立注释行会使验收 grep `IsBoolFlag==1` 变 2（grep -c 按行计数）——plan 自身两约束冲突
- **Fix:** 逐字引文置于 `func (v *exitEmptyValue) IsBoolFlag() bool { return true }` 行尾注释（单行两出现计 1 行）；类型 doc 注释转述惯例语义（不含字面量）——两约束同时满足，引文逐字性零损失
- **Files modified:** cmd/wesh/main.go
- **Verification:** 验收 grep `IsBoolFlag`==1 通过；行尾注释含 GOROOT 原文逐字
- **Committed in:** `3e5d3e6`（Task 2 GREEN 提交内）

---

**Total deviations:** 1 auto-fixed（Rule 3 plan 约束自洽调和）
**Impact on plan:** 纯注释落位调和，语义与验收面零损失。无范围蔓延。

## Threat Flags

None——威胁登记全部 plan threat_model 内闭环：T-06-04a（duration 值回显 accept——冒烟实证包装串形态，值非敏感论证注释登记在 Set）、T-06-04b（组合矛盾 fail-fast exit 2 先于 pty.Start/net.Listen 零资源占用——两拒绝行 + 二进制冒烟双锁定）、T-06-04c（展开只填未显式位不静默改写用户输入——TestParseArgs 展开行与矩阵放行行为锁）、T-06-SC（零新依赖零安装）。prohibitions 两项保持：--once 展开零 409 复活（第二客户端拒绝只走既有 503 计数路径——本 plan 零服务端改动）；CLI 默认零漂移（defaults 行 once=false/exitEmpty.set=false/maxClients=32 零值断言扩展锁定）。

## Known Stubs

None——全链 wired：两 flag 真实注册解析（TestParseArgs 三形态 + 错误两行行为锁）、展开/冲突校验/Options 接线全部行为测试与二进制冒烟锁定，零占位零 mock。协议层端到端证据（进程级退出状态 255、第二客户端 503）按 plan 路由至 06-06 phase06.mjs S3-S5。

## Issues Encountered

- **预存在发现（未修，scope 边界）：** `--help` 输出 `-max-clients` 行默认标注重复（`(default 32) (default 32)`——05-07 注册时 help 文案自含 `(default 32)`，flag 包对非零 int 默认值自动追加一份；纯展示层零语义）。非本 plan 改动引入，已登记 `.planning/phases/06-session-lifecycle/deferred-items.md`（修需改 one-way 契约文案面，留待 06-07 一并裁决）。

## Next Phase Readiness

- **06-06（UAT）**：CLI 面全部就绪——phase06.mjs S3（--once 第二客户端 503 + 断开后进程级退出状态 **255**，accept-255 裁决值）/ S4（--exit-when-empty=grace 启动期不触发 400ms 守候 + attach 取消）/ S5（grace 到期退出 255）可直接驱动；组合冲突 exit 2 文案可作 stderr 断言材料
- **06-07（README）**：--once 行等价关系逐字标明（≡ --max-clients=1 --exit-when-empty=0，D-12）+ --exit-when-empty 行 = 号形态用法明示与「启动时为空即退」误读澄清（迁移语义——仅非空→空迁移触发）+ accept-255 收口文案；顺带裁决 deferred 的 -max-clients help 重复标注
- **06-05（其余 CLI/部署面）**：无依赖交叉
- 关注点：无阻塞；端到端协议证据在 06-06 落地前，SESS-01/02 的 CLI 半侧为单测/冒烟级证据（plan 既定分工）

## Self-Check: PASSED

- 全部 2 个产物文件 + 本 SUMMARY 落盘核实（FOUND ×3）
- 任务提交 080795b / 3e5d3e6 在 git log 核实（FOUND ×2）；488b145..HEAD 区间 post-commit 删除检查无文件删除、无遗留 untracked（除本 SUMMARY 与 deferred-items.md 待 docs 提交）

---
*Phase: 06-session-lifecycle*
*Completed: 2026-08-23*
