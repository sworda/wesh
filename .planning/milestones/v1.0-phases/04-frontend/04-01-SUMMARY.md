---
phase: 04-frontend
plan: 01
subsystem: api
tags: [go, websocket, protocol, cli, prefs, osc52, whitelist]

# Dependency graph
requires:
  - phase: 02-protocol
    provides: WelcomeFrame 组帧模式、HelloPayload.Ticket omitempty 先例、未知字段忽略纪律（P2 D-02）
  - phase: 03-auth
    provides: fs.Func parse 期校验先例（--credential）、命名字段表驱动转换先例、启动面红线测试形态
provides:
  - WelcomePayload.Prefs 字段（json.RawMessage，prefs,omitempty）与 WelcomeFrame(mode, prefs) 两参签名
  - proto.ValidClientOptionKey 恰 10 键白名单（8 xterm 视觉键 + resizeOverlay/confirmBeforeUnload）
  - server.Options.ClientPrefs / Server.clientPrefs 注入与 Welcome 组帧挂点
  - --client-option（可重复，parse 期白名单+JSON 校验）与 --osc52（默认关）两 CLI flag
  - aggregateClientPrefs 聚合（last-wins；osc52=true 并入；零配置 nil）
  - 握手 e2e TestWelcomePrefs 两半侧（注入携 prefs / 未注入无键）
affects: [04-04 UAT 协议断言, 04-05 前端 prefs 消费, 04-02]

# Actuals (#2632) — chars/4 over realized diff，与 plan estimate（25000, confidence: low）配对校准
actuals:
  tokens: 7446
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []  # 零新依赖（encoding/json 标准库）
  patterns:
    - "prefs 通道形态：--client-option parse 期校验（白名单+JSON fail-fast）→ 聚合 last-wins → Welcome 内嵌一次性下发；服务端对 blob 不透明透传不解析（无双写漂移面）"
    - "启动面红线测试形态：错误子串表 + forbiddenSub 值内容禁入断言（SEC-01 延伸）"
    - "flag 回调校验错误记录式上报（避开 flag 包 invalid value %q 包装回显值内容）"

key-files:
  created: []
  modified:
    - internal/proto/proto.go
    - internal/proto/proto_test.go
    - internal/server/server.go
    - internal/server/e2e_test.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go

key-decisions:
  - "--client-option 校验错误经 clientOptErr 记录、Parse 后于 showVersion 早退之下统一上报——flag 包会把回调返回的错误包装为 `invalid value %q …`（%q 为原始 key=value 串）并同时打印 stderr，直接 return 必然泄露值内容，违反本 plan 红线"
  - "WelcomeFrame 签名演进为 (mode, prefs json.RawMessage) 两参——nil prefs 经 omitempty 使 JSON 不出 prefs 键，旧前端零漂移（P2 D-02 加字段纪律）"
  - "白名单用直白 switch（恰 10 键逐一列出），不引入新类型/注册表抽象（反过度设计纪律）"

patterns-established:
  - "prefs 三件套纪律：parse 期校验在 CLI 层一次性完成（白名单+JSON），server/proto 层零二次解析"
  - "omitempty 缺席回归锁：组帧后 bytes.Contains 反向断言 + e2e 无键断言双保险"

requirements-completed: [FE-07]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "FE-07 Go 通道全链：WelcomePayload.Prefs + WelcomeFrame 两参 + ValidClientOptionKey 白名单 + server ClientPrefs 注入 + --client-option/--osc52 两 flag + 聚合"
    requirement: FE-07
    verification:
      - kind: unit
        ref: "internal/proto/proto_test.go#TestValidClientOptionKey（15 行表：10 键通过 + osc52/allowProposedApi/空串/大小写变体/未知键拒绝）"
        status: pass
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs（client options / osc52 flag 两行）+ #TestClientOptionError（4 行错误表 + 红线断言）+ #TestAggregateClientPrefs（3 subtest）"
        status: pass
    human_judgment: false
  - id: D2
    description: "握手端到端：Options.ClientPrefs 注入时 Welcome JSON 携 prefs 逐键值相等；未注入时无 prefs 键（omitempty 缺席回归）"
    requirement: FE-07
    verification:
      - kind: e2e
        ref: "internal/server/e2e_test.go#TestWelcomePrefs（两半侧）"
        status: pass
      - kind: unit
        ref: "internal/proto/proto_test.go#TestWelcomeFrameErrorFrame/prefs_round-trip + prefs_omitted_when_nil_(omitempty)"
        status: pass
    human_judgment: false

# Metrics
duration: 25min
completed: 2026-08-18
status: complete
---

# Phase 4 Plan 01: FE-07 Go 通道端到端 Summary

**Welcome 帧内嵌 prefs 一次性下发通道全链落地：proto 契约（Prefs omitempty + 恰 10 键白名单）→ server ClientPrefs 注入 → CLI --client-option/--osc52 两 flag 与 last-wins 聚合，握手 e2e 双半侧锁定，值内容禁入错误文案红线经记录式上报守住**

## Performance

- **Duration:** 25 min
- **Started:** 2026-08-18T15:21:47Z
- **Completed:** 2026-08-18T15:47:06Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- D-13/D-14/D-15/D-12 服务端半侧落地：Welcome 内嵌 prefs（omitempty 缺席即旧前端零漂移）+ 白名单 parse 期 fail-fast + osc52 服务端专有开关（结构性排除出用户侧通道）
- wire 兼容锁定：nil prefs 时 Welcome JSON 与 Phase 3 逐字节同形（proto 组帧 bytes.Contains 反向断言 + e2e 无键断言双保险）
- 启动面红线延伸：client-option 错误文案只含 key 名与错误类别，TestClientOptionError 全行 forbiddenSub 断言绿
- 04-04 UAT 协议断言与 04-05 前端消费获得稳定通道（Options.ClientPrefs → Welcome prefs）

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): FE-07 Go 通道端到端** - `2c03378` (feat)
2. **Task 2: 表驱动测试扩展** - `b030198` (test)

**Plan metadata:** 见尾部 docs 提交

_Tracer 反馈门：提交后重跑 `go build ./... && go test ./internal/proto ./internal/server ./cmd/wesh -count=1` 全绿，⚡ Tracer verified end-to-end — expanding（自主形态：verify 为纯自动化命令且项目 human_verify_mode=end-of-phase，不做中途人工停留）。_

## Files Created/Modified
- `internal/proto/proto.go` - WelcomePayload.Prefs 字段、WelcomeFrame 两参签名、ValidClientOptionKey 白名单、头部注释同步（'P' 占位注记）
- `internal/server/server.go` - Options.ClientPrefs / Server.clientPrefs 字段、New() 装配拷贝、Welcome 挂点传 s.clientPrefs（顺序敏感注释补记）
- `internal/server/e2e_test.go` - dialHelloPayload helper（不改既有 dialHello 签名）、TestWelcomePrefs 两半侧
- `cmd/wesh/main.go` - clientOption 类型、config 两字段、--client-option/--osc52 两 flag、aggregateClientPrefs、parseArgs 头注释共 13 个
- `internal/proto/proto_test.go` - prefs 往返 + omitempty 缺席两 subtest、TestValidClientOptionKey 15 行表
- `cmd/wesh/main_test.go` - TestParseArgs 加 wantClientOptions/wantOSC52 字段与两行、TestClientOptionError 错误表 + 红线断言、TestAggregateClientPrefs 三 subtest

## Decisions Made
- **client-option 校验错误记录式上报**（clientOptErr + Parse 后统一返回，插入点在 showVersion 早退之后，03-04 先例）：flag 包 failf 会把回调错误包装为 `invalid value %q for flag -client-option: …` 且 %q 正是原始 key=value 串，并同时打印到 stderr——plan 字面「回调内 return fmt.Errorf」会让值内容同时进入返回错误串与启动 stderr，违反本 plan 自身红线（值内容禁入错误文案）并使 TestClientOptionError 红线断言必红。记录式上报两通道均干净，exit 2 fail-fast 语义不变。
- **WelcomeFrame 两参签名全仓库同步**：proto.go/server.go:442/proto_test.go 三处调用点一次更新，无旧单参残留（plan must_haves 硬要求）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] client-option 校验错误改记录式上报（plan 字面形态违反自身红线）**
- **Found during:** Task 1（main.go fs.Func 实现）
- **Issue:** plan 字面「回调内 return fmt.Errorf(...)」——flag 包（go1.26.3 flag.go:1139）会把回调返回的错误包装为 `invalid value %q for flag -%s: %v`（%q 为原始 key=value 串，含值内容），failf 同时打印到 stderr 并返回该包装串；run() 再将其打印一次。与 plan prohibitions「启动报错/日志文案禁止含 prefs 值内容」及 Task 2 红线断言直接冲突——照字面实现则 Task 2 必红。
- **Fix:** 回调校验失败时把 fmt.Errorf 原文记入 clientOptErr 闭包变量并返回 nil（flag 不触发 failf 包装/打印），parseArgs 在 fs.Parse 返回后、showVersion 早退之下统一上报（03-04「纯信息路径不被配置校验阻断」先例）；parse 期 exit 2 fail-fast 语义不变。
- **Files modified:** cmd/wesh/main.go
- **Verification:** TestClientOptionError 4 行全绿（含 forbiddenSub 值内容禁入断言）；TestParseArgs 新行绿
- **Committed in:** 2c03378（Task 1 提交）

**2. [Rule 3 - Blocking] proto_test.go WelcomeFrame 调用点随 Task 1 同步两参化**
- **Found during:** Task 1（签名演进后编译闭环）
- **Issue:** plan must_haves 称「proto.go 与 server.go 两处调用点同步更新」，实际第三处调用点 proto_test.go:63（单参）会使 `go test ./internal/proto` 编译失败，Task 1 verify 无法绿；plan Task 2 action 1.1 本就将该更新列为两参形态同步（既有 ro 断言行零改动传 nil）。
- **Fix:** Task 1 内做最小签名适配（仅 `WelcomeFrame(ModeRO, nil)` 一处），Task 2 专注新增用例（prefs 往返/omitempty/白名单表）。
- **Files modified:** internal/proto/proto_test.go
- **Verification:** Task 1 verify（go build + 三包测试）全绿；Task 2 新增 subtest 全绿
- **Committed in:** 2c03378（Task 1 提交）

---

**Total deviations:** 2 auto-fixed (1 bug/plan 自相矛盾, 1 blocking)
**Impact on plan:** 两修正均为达成 plan 自身验收标准的必要最小改动，无范围蔓延；D-12/D-13/D-14/D-15 语义逐字保持。

## Issues Encountered
None — 两任务一次通过，无调试往返。

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- 04-02（前端三包接入）与 04-05（前端 prefs 消费/query 覆盖）的 Go 侧通道已就绪：Welcome prefs 形状、白名单键集合、osc52 并入语义均有回归锁
- 全量 `go test -race ./...` 按 plan 约定在 04-06 收口统一执行
- 无阻塞项

## Self-Check: PASSED

- 提交存在性：`2c03378`（Task 1 feat）、`b030198`（Task 2 test）均在 git log
- 契约符号：`Prefs json.RawMessage` / `func ValidClientOptionKey` / 两参 `WelcomeFrame`（proto.go）、`clientPrefs`（server.go）、`aggregateClientPrefs`（main.go）、`TestWelcomePrefs`（e2e_test.go）、`TestValidClientOptionKey`（proto_test.go）、`invalid --client-option key`（main_test.go）全部 grep 命中
- 全仓库无旧单参 WelcomeFrame 调用残留（仅两参形态两处命中）
- Plan 级 verify `go build ./... && go test ./internal/proto ./internal/server ./cmd/wesh -count=1` 退出 0；GOROOT gofmt 与 go vet 零告警

---
*Phase: 04-frontend*
*Completed: 2026-08-18*
