---
phase: 06-session-lifecycle
plan: 01
subsystem: api
tags: [websocket, go, typescript, protocol, lifecycle, xterm]

requires:
  - phase: 05-multi-client
    provides: lifecycle hubMu 快照 + 并行 Close 广播形态、注册表/outbox 结构、write-policy 分档
  - phase: 02-protocol
    provides: 帧类型字节空间纪律（P2 D-01/D-02）、ErrorFrame 组帧先例、关闭码全集
provides:
  - proto.Exit='X'(0x58) S→C 终结帧类型字节 + ExitPayload{exit_code,message} + ExitFrame 组帧
  - lifecycle EXIT→1000 写序安全广播（组帧一次共享只读 + 每客户端 goroutine 同步 Write(2s)→Close）
  - signalName 大写信号名映射 + exitMessage 三形态组文案（服务端唯一写口）
  - 前端 EXIT=0x58 常量 + lastExit 暂存通道 + onclose 1000 正文 message 承接
affects: [06-02, 06-03, 06-05, 06-06, phase-07]

actuals:
  tokens: 21000
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "写序安全广播：lifecycle 组帧一次（共享只读引用）+ 每客户端 goroutine 内同步 Write(EXIT,2s ctx)→Close(1000)，禁止 outbox 异步入队（Pitfall 1 关闭帧超车）"
    - "信号名显式大写映射 helper（signalName）——Signal.String() 小写描述词陷阱（Pitfall 3）的唯一合法规避形态"
    - "EXIT message 服务端组文案唯一写口（exitMessage 三形态），前端 textContent 直显零改写"

key-files:
  created:
    - internal/server/exit_test.go
    - internal/server/exitmsg_test.go
  modified:
    - internal/proto/proto.go
    - internal/proto/proto_test.go
    - internal/server/server.go
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "确认门 Task 1 用户裁决 as-locked（2026-08-23）：EXIT='X'(0x58) + {\"exit_code\":N,\"message\":M} 三形态文案 + EXIT→1000 广播序列，与 06-CONTEXT D-08/D-09/D-10 逐字一致"
  - "EXIT 广播 Write 超时取 2s 常量（RESEARCH OQ3 定值，拒绝可配化——P2 D-10 常量纪律，Phase 9 标定挂账）；stall 端退化为 1000+前端回退文案（非致命）"
  - "exitMessage 信号死亡分支的 WaitStatus 断言失败兜底：sig 占位 code（恒 -1）产数字形态——断言失败在 unix 真实进程不可达（ExitError.Sys() 恒 WaitStatus）"
  - "TDD 提交纪律按 plan 字面收口：RED（编译失败实证）→GREEN 顺序保持，proto+server+三测试文件同任务同提交（plan action 6 明示，不拆 test/feat 双提交）"

patterns-established:
  - "写序安全终结广播：同步 Write(EXIT) 后接 Close(1000)，同 goroutine 帧级串行化保序"
  - "白盒 helper 测试分文件：package server（exitmsg_test.go）与黑盒 package server_test（exit_test.go）并存的 Go 单文件单 package 约束解法"

requirements-completed: [SESS-03]

coverage:
  - id: D1
    description: "proto EXIT 帧契约：Exit='X'(0x58) 常量（与 'E' Error 语义边界注释）+ ExitPayload{exit_code,message} + ExitFrame 组帧 round-trip（含 Exit!=Error 区分断言）"
    requirement: SESS-03
    verification:
      - kind: unit
        ref: "internal/proto/proto_test.go#TestExitFrame + TestProtocolConstants（'X' 行）"
        status: pass
    human_judgment: false
  - id: D2
    description: "lifecycle EXIT→1000 写序安全广播：双端（rw+ro 混合）收逐字节一致 EXIT 帧（exit_code==42、message 逐字）先于 1000 到达；信号死亡 exit_code=-1 + message 含大写 SIGHUP；exitf 退出码传递不变"
    requirement: SESS-03
    verification:
      - kind: integration
        ref: "internal/server/exit_test.go#TestExitFrameBroadcast + TestExitFrameSignal（-race -count=3 稳定）"
        status: pass
    human_judgment: false
  - id: D3
    description: "message 三形态服务端唯一写口：signalName 映射表逐行（表外 SIGWINCH 回退）+ exitMessage nil err/非 ExitError 两可构造分支"
    requirement: SESS-03
    verification:
      - kind: unit
        ref: "internal/server/exitmsg_test.go#TestSignalName + TestExitMessage"
        status: pass
    human_judgment: false
  - id: D4
    description: "前端 EXIT 承接：EXIT=0x58 常量 + lastExit 暂存（case EXIT，malformed 静默丢弃）+ onclose 1000 正文 lastExit?.message ?? 硬编码回退 + connect() 重置块清零 + dist 重建入库"
    requirement: SESS-03
    verification:
      - kind: other
        ref: "time pnpm -C web build（tsc+vite+gzip 退出 0）+ node --test web/src/lib/*.test.ts（16/16）+ 验收 grep 断言（0x58/case EXIT/lastExit×4/R2 正文逐字）全过"
        status: pass
    human_judgment: false

duration: 14min
completed: 2026-08-23
status: complete
---

# Phase 06 Plan 01: EXIT 帧契约与写序安全广播（SESS-03 tracer）Summary

**SESS-03 脊柱端到端打通：子进程退出后全部在线客户端（rw+ro 混合）先收 'X'(0x58) EXIT 帧（{"exit_code":N,"message":M}，信号死亡 -1+大写信号名）再收 1000——proto 契约、lifecycle 写序安全广播、前端 lastExit 承接三侧落码，Go -race 全绿零回归**

## 确认门结果（Task 1，checkpoint:decision）

用户裁决 **as-locked**（2026-08-23，execute 续跑 prompt 确认）——按锁定值落地，不再重议：
- EXIT 帧 = 新 S→C 类型字节 **'X'（0x58）**，载荷 **{"exit_code":N,"message":M}**（snake_case tag）
- message 三形态：正常退出 `The process exited with code {N}.`；信号死亡 `The process was killed by signal {SIGNAME}.`（显式大写映射，未知名回退数字形态）；非 ExitError 兜底 `The process terminated.`
- 广播序列 EXIT→1000；前端 lastExit 暂存 → onclose 1000 正文
- 与 06-CONTEXT.md D-08/D-09/D-10、proto.go:32 预留位、06-UI-SPEC.md 三形态表逐字一致

## Performance

- **Duration:** 14 min（不含确认门等待）
- **Started:** 2026-08-23T03:47:33Z
- **Completed:** 2026-08-23T04:01:20Z
- **Tasks:** 3/3（Task 1 确认门 → as-locked；Task 2 tracer；Task 3 前端）
- **Files modified:** 7（2 新建测试 + 5 修改）

## Accomplishments

- proto.go：`Exit = 'X'` 常量（32 行占位注释兑现改写，登记与 'E' Error 语义边界——终结 ≠ 错误）+ `ExitPayload{ExitCode, Message}`（显式 snake_case json tag）+ `ExitFrame()`（ErrorFrame 逐字同构）
- server.go lifecycle：Drain 后组帧一次（全客户端共享只读引用，P5-1 纪律），hubMu 快照后每客户端 goroutine【同步 Write(EXIT, 2s ctx) → Close(1000)】——写序靠库帧级串行化保证，禁止 outbox 异步入队（Pitfall 1 红线落地）；Write 失败不补救直接 Close
- server.go 两 helper：`signalName`（13 个常见信号显式大写名映射，表外 ok==false——规避 Signal.String() "hangup" 小写陷阱）+ `exitMessage`（三形态文案与 UI-SPEC 逐字一致）
- 测试四面锁定：TestProtocolConstants 'X' 行、TestExitFrame round-trip（含 Exit!=Error 区分断言）、TestExitFrameBroadcast（帧序+双端逐字节一致+exit 42+1000+waitExit(42)）、TestExitFrameSignal（-1+SIGHUP+1000+waitExit(-1)）、TestSignalName/TestExitMessage 白盒
- 前端：EXIT=0x58 常量 + lastExit 模块级暂存（lastError 邻位）+ case EXIT（malformed console.warn 丢弃）+ onclose 1000 正文 `lastExit?.message ?? 硬编码回退` + connect() 重置块清零；dist 重建入库

## Task Commits

1. **Task 1: 确认门（D-08/D-09 one-way）** — 无提交（checkpoint:decision，用户裁决 as-locked，结果记录于本文件与 STATE decisions）
2. **Task 2 (tracer): proto EXIT 契约 + lifecycle 广播 + 两测试文件** — `2526294` (feat)——五文件同任务同提交（plan 提交纪律）
3. **Task 3: 前端 EXIT 承接 + dist 重建** — `60ab960` (feat)

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS）

_Note: TDD 顺序实证——RED（三测试文件先行，`go test` 编译失败：undefined Exit/ExitFrame/ExitPayload/signalName/exitMessage）→ GREEN（实现后全绿）；提交按 plan action 6「同任务同提交」字面收口为单提交，未拆 test/feat 双提交（plan 字面优先于默认 TDD 分段提交）。_

## Files Created/Modified

- `internal/proto/proto.go` — Exit 常量 + ExitPayload + ExitFrame（'X' 占位兑现）
- `internal/proto/proto_test.go` — 帧字节表 'X' 行 + TestExitFrame round-trip
- `internal/server/server.go` — signalName/exitMessage 两 helper + lifecycle EXIT→1000 广播段（含 2s 权衡注释）
- `internal/server/exit_test.go`（新建）— TestExitFrameBroadcast + TestExitFrameSignal（黑盒，e2e helper 零改动复用）
- `internal/server/exitmsg_test.go`（新建）— TestSignalName + TestExitMessage（白盒，resize_test.go 先例）
- `web/src/main.ts` — EXIT 常量 + lastExit 暂存 + case EXIT + onclose 1000 正文 + 重置块
- `web/dist/index.html` — 重建产物（.gz 不入库既定纪律）

## Verification Evidence

- `go build ./... && go vet ./...` 退出 0
- `go test -race -count=1 ./internal/proto/ ./internal/server/ ./internal/pty/` 三包全绿（含两新测试；既有套件零回归零适配——TestExitBroadcast/TestExitCodePropagation 的『读到 CloseError 丢弃途中帧』形态天然吸收新增 EXIT 帧）
- 新测试 `-race -count=3` 复跑全 PASS（写序/帧序断言无 flake）
- `time pnpm -C web build` 退出 0（tsc 严格模式 + vite + gzip，3.1s）；`node --test web/src/lib/*.test.ts` 16/16
- 验收 grep 全过：`Exit = 'X'`==1、`func ExitFrame`==1、`exit_code`>=1、`ExitFrame`(server.go)>=1、`func signalName`==1、两测试函数各==2、'SIGHUP'(exit_test)>=1、三形态文案 1/2/1 计数精确命中、`0x58`>=1、`case EXIT`==1、`lastExit`==4、R2 正文逐字==1

## Decisions Made

- **确认门 as-locked**（见上节）——one-way 协议契约定稿
- **2s Write 超时定值**（RESEARCH OQ3 落定）：stall/慢链路 2s 未写完 ~100B EXIT 帧即放弃直写，该端退化为 1000 + 前端硬编码回退文案（R2 回退路径既有，非致命）；2s ≪ Close 内建 5s+5s 上界；拒绝可配化（P2 D-10 常量纪律 + Phase 9 标定挂账），注释逐字登记权衡
- **WaitStatus 断言失败兜底的 sig 占位**：plan 字面 `int(sig)` 在断言失败路径无 sig 可取——取 `sig := code`（恒 -1）占位，断言成功后覆盖为真实信号号；断言失败产出 "signal -1"（语义 = 信号号未知），unix 真实进程不可达（防御性分支），验收 grep（%s+%d 双形态 ≥2）保持
- **signalName switch 不加 case 大括号**：server.go 文件局部惯例（774/803 行等既有 switch 均无括号），文件内一致性优先
- **tracer 反馈门以自治形态执行**：本 run 非 auto mode，严格字面应 STOP 等 human-verify；但 (a) orchestrator 续跑指令显式授权「Task 2 与 Task 3 一并实现」，(b) tracer `<verify>` 为纯自动化 go 套件（headless 硬约束下不存在人工验证通道），(c) 提交后已端到端复跑 `<verify>` 全绿——故按自治形态门处理（失败即 HALT 的语义保持），未中断执行

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] GOROOT gofmt 清零两触碰文件**
- **Found during:** Task 2 GREEN（提交前检查）
- **Issue:** 触碰的 server.go/exit_test.go 被 GOROOT gofmt 标记（import 序 syscall 位置 + CJK 注释 `//（` 排版）
- **Fix:** `$(go env GOROOT)/bin/gofmt -w` 仅作用于本 plan 触碰的两文件（纯排版零语义；/usr/bin/gofmt 陈旧不用的 01-03 纪律保持）
- **Files modified:** internal/server/server.go, internal/server/exit_test.go
- **Verification:** gofmt -l 对五触碰文件全净；build/vet/test 全绿
- **Committed in:** `2526294`（Task 2 提交内）

**范围外发现（不修，已登记 deferred-items.md）：** `internal/server/clients.go`/`clients_test.go`/`resize.go` 存在既有 GOROOT gofmt 漂移（CJK 注释排版，01-03/05-09 登记的同源问题）——非本任务改动引入，本 plan 未授权六段式 style 清零段，留待后续 plan 处理。

---

**Total deviations:** 1 auto-fixed（Rule 3）+ 1 项范围外登记
**Impact on plan:** 排版修正为零语义必要项；无范围蔓延。既有套件适配明细：**零适配**（预期内——途中帧丢弃形态吸收）。

## Threat Flags

None——EXIT 伪造面（T-06-01a accept）/message 注入面（T-06-01b：textContent 直显 + 服务端唯一写口，已落地）/stall 拖延面（T-06-01c：2s 超时 + Close 5s+5s 上界 + 并行 goroutine，已落地）均在 plan threat_model 登记内，无新增未登记面。

## Known Stubs

None——全链 wired：EXIT 帧真实进程路径产生（lifecycle），前端真实帧消费（case EXIT → lastExit → 1000 正文），零占位零 mock。

## Issues Encountered

None——plan 的 read_first 与 RESEARCH/PATTERNS 逐字先例完备，实现零探索弯路。

## Next Phase Readiness

- 06-02（--once/--exit-when-empty）：SIGHUP 收口路径产生的客户端可见形态已由本 plan 的 EXIT 帧承载（UI-SPEC「同帧」条款——SIGHUP 致死 → "killed by signal SIGHUP" + 1000，零特判）
- 06-03（重连）：onclose 1000 分支已消费 lastExit；1006 注释改写与重连分派在 06-03 落地（本 plan 按计划未动 685-687 注释）
- 06-06（UAT）：phase06.mjs 的 S1/S2 断言面（EXIT 帧序/退出码/os.Exit(-1)→255 进程级断言）所依赖的协议行为已全部锁定
- 关注点：无阻塞；stall 端 2s 直写退化为回退文案的行为在 06-06 可有专项断言（非必须）

## Self-Check: PASSED

- 全部 7 个产物文件 + 本 SUMMARY 落盘核实（FOUND ×8）
- 任务提交 2526294 / 60ab960 在 git log 核实（FOUND ×2）；两提交 post-commit 删除检查均无文件删除

---
*Phase: 06-session-lifecycle*
*Completed: 2026-08-23*
