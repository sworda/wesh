---
phase: 01-pty
plan: 03
subsystem: testing
tags: [go, e2e, websocket, pty, lifecycle, coder-websocket, race-fix]

requires:
  - phase: 01-pty
    plan: 01
    provides: server.New/Attach/生命周期实现（D-09~D-12）与 e2e 构造模式（sess + exitf 桩 + 随机端口监听）
provides:
  - D-09/D-10/D-11 与未知帧 1002 四条生命周期边界的自动化回归锁定
  - CLI 契约（D-02~D-06）表驱动测试
  - server.go childExited 修复：D-10 退出码不再被 D-11 路径竞态覆盖
  - TestHelperProcess argv 守卫模式（exit42/sighup 两分支，供后续 plan 复用）
affects: [01-04, 01-05, 06-lifecycle]

actuals:
  tokens: 5549
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "TestHelperProcess argv 守卫分派（`-test.run=TestHelperProcess -- wesh-helper-*`），env 守卫被 SEC-06 白名单剥离故禁用"
    - "helper 同步协议：exit42 读 stdin 一行再自杀 / sighup 先 READY 再收信号——消除 spawn 时序竞态"

key-files:
  created:
    - cmd/wesh/main_test.go
  modified:
    - internal/server/e2e_test.go
    - internal/server/server.go

key-decisions:
  - "[Phase 01-03]: D-10/D-11 终结竞态修复——lifecycle 先置位 childExited 再发 1000 关闭帧，wsDisconnected 见置位即放弃 exitf 竞争，退出码传递确定化"
  - "[Phase 01-03]: SIGHUP 送达证据用落盘标记文件而非 stdout——WS 断开后 server onChunk 丢弃输出，stdout 标记不可观测；落盘跨平台（/proc 仅 Linux）"

patterns-established:
  - "生命周期 e2e 构造收口：startTestServer（sess+exitf 桩+随机端口）/ waitExit（exitf 码断言）/ helperArgv（TestHelperProcess argv 生成）三件套"
  - "关闭码断言：读循环至 CloseError，断言 Code==1000/1002 且 !=1006"

requirements-completed: [CORE-01]

coverage:
  - id: D1
    description: "第二 WS 连接 attach 在 Accept 前收 HTTP 409 且首连接不受影响（D-09）"
    requirement: CORE-01
    verification:
      - kind: e2e
        ref: "internal/server/e2e_test.go#TestSecondClient409"
        status: pass
    human_judgment: false
  - id: D2
    description: "子进程 exit(42) → 已 attach 客户端先收 1000 关闭帧 → exitf(42)（D-10 退出码传递）"
    requirement: CORE-01
    verification:
      - kind: e2e
        ref: "internal/server/e2e_test.go#TestExitCodePropagation"
        status: pass
    human_judgment: false
  - id: D3
    description: "未知类型帧（'9'）→ WS 以 1002 关闭且全程无 1006（D-16 关闭码纪律）"
    requirement: CORE-01
    verification:
      - kind: e2e
        ref: "internal/server/e2e_test.go#TestUnknownFrame1002"
        status: pass
    human_judgment: false
  - id: D4
    description: "WS 客户端断开 → 子进程进程组收 SIGHUP（落盘标记佐证）且 exitf(0)（D-11）"
    requirement: CORE-01
    verification:
      - kind: e2e
        ref: "internal/server/e2e_test.go#TestClientDisconnectSIGHUP"
        status: pass
    human_judgment: false
  - id: D5
    description: "CLI 契约：默认值 0.0.0.0:7681、`--` 后参数原样透传、无命令报错含 usage 行、--version（D-02~D-06）"
    requirement: CORE-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs / TestNoCommandError / TestVersionFlag"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-08-14
status: complete
---

# Phase 01 Plan 03: 生命周期与 CLI 契约测试网 Summary

**Phase 1 单次语义四条生命周期边界（409/退出码 42/未知帧 1002/断开 SIGHUP）与 CLI 契约全部自动化锁定；顺带暴露并修复 D-10 退出码被 D-11 竞态覆盖的真实 bug（childExited 标志收口）。**

## Performance

- **Duration:** 18 min
- **Started:** 2026-08-14T01:35:19Z
- **Completed:** 2026-08-14T01:53:18Z
- **Tasks:** 2
- **Files modified:** 3（2 测试文件 + 1 被测实现修复）

## Accomplishments

- 生命周期 e2e 四测落地：TestSecondClient409（D-09）/ TestExitCodePropagation（D-10）/ TestUnknownFrame1002（D-16）/ TestClientDisconnectSIGHUP（D-11），`go test -race -count=10 ./internal/server` 全绿
- CLI 测试落地：TestParseArgs 表驱动三例（默认值 / `-` 开头参数透传 / 子命令 flag 隔离）+ TestNoCommandError（D-03）+ TestVersionFlag
- 修复 server.go 终结竞态：lifecycle 的 1000 关闭帧会终结 Attach 读循环并误入 wsDisconnected，terminate(true, 0) 与 terminate(false, 42) 经 sync.Once 竞争——exitf(0) 可抢跑顶替子进程退出码（D-10 被违反）。childExited 标志使 D-10 确定优先
- TestHelperProcess argv 守卫模式钉死（env 守卫会被 SEC-06 白名单剥离）；两个 helper 分支各带同步协议消除时序竞态

## Task Commits

Each task was committed atomically:

1. **Task 1: server 生命周期 e2e 四测** - `d5f67ab`（fix：竞态修复）+ `de87429`（test：四测与 helper）
2. **Task 2: CLI 层测试** - `c7a05a7`（test）

## Files Created/Modified

- `internal/server/e2e_test.go` - 增量四测 + TestHelperProcess（argv 守卫分派）+ startTestServer/waitExit/helperArgv 公共装配（+265 行）
- `internal/server/server.go` - childExited 竞态修复（+15/-1 行）
- `cmd/wesh/main_test.go` - TestParseArgs/TestNoCommandError/TestVersionFlag（新文件，92 行）

## Decisions Made

- **SIGHUP 送达证据用落盘标记文件**（替代 plan 提及的 stdout 标记经 drain 可见）：WS 断开后 server 侧 onChunk 丢弃输出（conn 已清），stdout 标记对测试不可观测；/proc/<pid> 仅 Linux 可用。落盘文件跨平台且确定性可断言，仍属 plan 认可的"标记"证据范畴
- **exit42 helper 读 stdin 一行后再自杀**：保证 D-10 的 1000 关闭帧发出时仍有已 attach 接收方，消除子进程抢在 Dial 完成前退出的竞态
- **sighup helper 先报 READY 再收信号**：消除 SIGHUP 先于 signal.Notify 就位的竞态

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] D-10 退出码被 D-11 路径竞态覆盖（server.go）**
- **Found during:** Task 1（TestExitCodePropagation 首跑即红：exit code = 0, want 42）
- **Issue:** lifecycle 向客户端发 1000 关闭帧后，Attach 读循环的 c.Read 必然返回 CloseError 走入 wsDisconnected；terminate(true, 0) 与 terminate(false, 42) 经 sync.Once 竞争，exitf(0) 稳定抢跑——D-10"退出码 = 子进程退出码"被违反。plan 01-01 的实现缺陷，由本 plan 的测试暴露
- **Fix:** Server 增加 childExited atomic.Bool；lifecycle 在 Wait 返回后先置位再关 conn；wsDisconnected 见置位即返回，exitf 固定由 lifecycle 以子进程退出码收口
- **Files modified:** internal/server/server.go
- **Verification:** `go test -race -count=10 ./internal/server` 全绿；既有 TestEchoPTY/TestDrainBeforeAttach 不回归
- **Committed in:** `d5f67ab`

**2. [Rule 2 - 实现细节] SIGHUP 证据改为落盘标记文件**
- **Found during:** Task 1（TestClientDisconnectSIGHUP 设计时）
- **Issue:** plan action 的"stdout 标记经 drain 输出可见"不可实现——WS 断开后 onChunk 丢弃全部输出
- **Fix:** helper 收 SIGHUP 后向 argv 传入的临时文件写 GOT_SIGHUP 标记，测试轮询断言
- **Files modified:** internal/server/e2e_test.go
- **Verification:** 测试稳定通过（含 -race ×10）
- **Committed in:** `de87429`

---

**Total deviations:** 2 auto-fixed（1 bug 修复 / 1 实现细节修正）
**Impact on plan:** 竞态修复是 D-10 正确性的必要前提（测试即因此红）；无范围蔓延。

## Issues Encountered

- `/usr/bin/gofmt` 是系统陈旧版本（不识泛型，误报 server.go 语法错误）——改用 `$(go env GOROOT)/bin/gofmt` 与 `go vet` 验证，实际代码无格式问题。后续 plan 的 fmt 检查均须走 GOROOT 版本

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 1 单次语义的全部边界行为已进入回归网（-race 强制），01-04（darwin kqueue 专项）与 01-05（收尾/文档）可直接开工
- server.go 的 childExited 收口为 Phase 6 生命周期重写留下明确锚点（届时整体替换，但竞态教训须保留：服务端主动关闭必经读循环终结路径）
- TestHelperProcess argv 守卫模式可在后续 plan 直接复用（helperArgv + `--` 后标记分派）

---
*Phase: 01-pty*
*Completed: 2026-08-14*

## Self-Check: PASSED

- 文件全部存在：internal/server/e2e_test.go、internal/server/server.go、cmd/wesh/main_test.go、01-03-SUMMARY.md
- 提交全部存在：d5f67ab（fix）、de87429（test）、c7a05a7（test）
