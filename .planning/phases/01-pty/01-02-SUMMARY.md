---
phase: 01-pty
plan: 02
subsystem: testing
tags: [go, pty, creack-pty, tiocswinsz, env-whitelist, race-detector]

requires:
  - phase: 01-pty plan 01-01
    provides: internal/pty 的 Start/whitelistEnv/ReadLoop/Resize/Wait 被测实现（spawn.go/io.go/reap_linux.go）
provides:
  - spawn 三红线自动化回归网（exec 数组不经 shell / env 零泄露 / spawn 失败不伤 fd 0/1/2）
  - resize TIOCSWINSZ 集成证明（24 80 → 50 132）
  - 收割无僵尸证明（/proc/<pid> 消失，20 次循环）
  - 测试共享夹具 startCollect/awaitSession（readDone happens-before 同步 + 10s 护栏）
affects: [01-pty 后续 plan（01-03/01-04/01-05 复用 awaitSession 夹具与测试纪律）]

actuals:
  tokens: 1945
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "内测 package pty + startCollect/awaitSession：ReadLoop 输出经 readDone channel close 建立 happens-before，免互斥锁即 -race 安全"
    - "fd 活性探测用 syscall.Fsync 而非 os.NewFile（后者带 finalizer，GC 会误关真实 fd 0/1/2）"
    - "PTY 输出断言按 strings.Fields 空白切分，免疫 ONLCR 的 \\n→\\r\\n 翻译"

key-files:
  created:
    - internal/pty/spawn_test.go
    - internal/pty/io_test.go
    - internal/pty/reap_test.go
  modified: []

key-decisions:
  - "fd 0/1/2 探测采用 syscall.Fsync（等价 syscall 探测，plan 明示允许）而非 os.NewFile——规避 finalizer 误关真实 fd 的测试自伤"
  - "TestResize 断言按空白切分比对 [24 80 50 132]，兼容 PTY ONLCR 换行翻译"

patterns-established:
  - "测试夹具 10s 统一超时护栏（Wait/drain 各一道 select+time.After），防挂死拖死测试进程"
  - "子进程 env 泄露断言必须配阳性对照（断言 TERM 存在）防空输出假绿"

requirements-completed: [SEC-06, CORE-01, CORE-02]

coverage:
  - id: D1
    description: "argv 以 exec 数组原样传递：$(id) 字面量不被 shell 展开（CORE-01）"
    requirement: CORE-01
    verification:
      - kind: unit
        ref: "internal/pty/spawn_test.go#TestExecArrayNoShell"
        status: pass
    human_judgment: false
  - id: D2
    description: "env 白名单：宿主注入 AWS_SECRET_ACCESS_KEY 后白名单函数与子进程 env 输出双层不含（SEC-06）"
    requirement: SEC-06
    verification:
      - kind: unit
        ref: "internal/pty/spawn_test.go#TestEnvWhitelist"
        status: pass
    human_judgment: false
  - id: D3
    description: "spawn 不存在二进制返回错误且服务端 fd 0/1/2 保持有效（ttyd close(0) 缺陷回归）"
    requirement: SEC-06
    verification:
      - kind: unit
        ref: "internal/pty/spawn_test.go#TestSpawnFailKeepsStdio"
        status: pass
    human_judgment: false
  - id: D4
    description: "resize 经 TIOCSWINSZ 同步：stty size 序列 24 80 → 50 132（CORE-02）"
    requirement: CORE-02
    verification:
      - kind: integration
        ref: "internal/pty/io_test.go#TestResize"
        status: pass
    human_judgment: false
  - id: D5
    description: "收割无僵尸：短命令退出后 /proc/<pid> 消失，20 次高频建销循环（成功准则 3）"
    verification:
      - kind: integration
        ref: "internal/pty/reap_test.go#TestReap"
        status: pass
    human_judgment: false

duration: 4min
completed: 2026-08-14
status: complete
---

# Phase 01 Plan 02: PTY 专项测试网 Summary

**PTY 引擎五红线自动化锁定：`$(id)` 不经 shell、env 白名单双层零泄露、spawn 失败 fd 0/1/2 存活、Resize(132,50) 后 stty 报 50 132、/bin/true 收割后 /proc 消失——VALIDATION 1-01-01~05 全绿，-race 干净。**

## Performance

- **Duration:** 4 min
- **Started:** 2026-08-14T01:18:59Z
- **Completed:** 2026-08-14T01:23:00Z
- **Tasks:** 2
- **Files modified:** 3（全部新建测试文件）

## Accomplishments

- VALIDATION.md 1-01-01/02/03/04/05 五项从 pending 变 green：spawn 三红线 + resize 同步 + 收割无僵尸全部自动化证明成立
- SEC-06 验收锚点成立：单元层（whitelistEnv 输出）+ e2e 层（子进程 `/usr/bin/env` 实际输出）双层断言宿主注入的 AWS_SECRET_ACCESS_KEY 不可见，并配 TERM 阳性对照防空输出假绿
- ROADMAP 成功准则 2（resize 同步）与准则 3（env 零泄露 + 收割无僵尸）获得自动化证明
- `go test -race -count=1 ./internal/pty` 与整仓 `go test ./... -count=1` 全绿

## Task Commits

Each task was committed atomically:

1. **Task 1: spawn 专项三测——exec 数组 / env 白名单 / 失败不伤 fd** - `90fda90` (test)
2. **Task 2: 集成两测——resize TIOCSWINSZ + 收割无僵尸** - `234c48d` (test)

**Plan metadata:** 见最终 docs 提交（complete plan）

## Files Created/Modified

- `internal/pty/spawn_test.go` - TestExecArrayNoShell / TestEnvWhitelist / TestSpawnFailKeepsStdio + 共享夹具 startCollect/awaitSession
- `internal/pty/io_test.go` - TestResize（stty 序列 24 80 → 50 132 断言）
- `internal/pty/reap_test.go` - TestReap（`//go:build linux`，/proc/<pid> 消失 + 20 次循环）

## Decisions Made

- fd 0/1/2 活性探测采用 `syscall.Fsync`（plan 明示允许的"等价 syscall 探测"）而非 `os.NewFile().Fsync()`——os.NewFile 返回的 *File 带 finalizer，GC 可能误关真实 fd 0/1/2，测试自身制造正被回归的 ttyd close(0) 同款缺陷
- TestResize 断言用 `strings.Fields` 空白切分比对 `[24 80 50 132]`，而非原始串精确匹配——PTY ONLCR 会把 stty 的 `\n` 译为 `\r\n`，逐字段比对语义等价且免疫行尾差异
- 共享夹具读取同步走 readDone channel close 的 happens-before（-race 证明干净），不引入互斥锁

## Deviations from Plan

None - plan executed exactly as written.

（说明：syscall.Fsync 与 strings.Fields 两处均为 plan action 文本明示允许的等价实现选择，不构成计划外工作。）

## Issues Encountered

None。三个 spawn 测试与两个集成测试首跑即绿；本机环境与 RESEARCH 原型实测输出（行 563-575）完全一致。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- internal/pty 包测试网成型，01-03/01-04/01-05 可直接复用 startCollect/awaitSession 夹具与 10s 护栏纪律
- darwin 侧等效覆盖（TestKqueue 系列，VALIDATION 1-01-07）按计划在 01-04 落地，CI macos runner 验证

## Self-Check: PASSED

- FOUND: internal/pty/spawn_test.go, internal/pty/io_test.go, internal/pty/reap_test.go
- FOUND: .planning/phases/01-pty/01-02-SUMMARY.md
- FOUND commits: 90fda90 (Task 1), 234c48d (Task 2)

---
*Phase: 01-pty*
*Completed: 2026-08-14*
