---
phase: 11-per-client
plan: "02"
subsystem: pty
tags: [darwin, kqueue, exit-watcher, fail-closed, pitfall-9, go]

requires:
  - phase: 11-per-client
    plan: "01"
    provides: "per-client 生命周期主干（N 会话每会话一 goroutine 消费共享 darwin exit watcher——dup-watch 挂账到期的触发面）"
provides:
  - "errDupWatch 包级错误值（internal/pty/reap_darwin.go，errors.New 单点定义，文案零敏感值）"
  - "watch() dup-watch fail-closed 防御：w.mu 内重复 pid 注册拒绝返回 errDupWatch，先注册订阅零影子化"
  - "awaitExit 既有 watch-error 分支注释锚定 errDupWatch 唯一预期错误源（退化 cmd.Wait() 兜底零新面）"
  - "TestWatchDupPidFailClosed（internal/pty/reap_darwin_test.go，append-only）——CI macOS leg 运行锁 + 首订阅不影子化反向锁"
affects: [11-03, 11-04, 11-05, 11-06, 14-matrix]

actuals:
  tokens: 1119
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "dup-watch fail-closed：kqueue (ident,filter) 唯一键语义下重复注册从「静默影子化→收割挂死」变「可观测错误→调用方退化 cmd.Wait()」"

key-files:
  created: []
  modified:
    - internal/pty/reap_darwin.go
    - internal/pty/reap_darwin_test.go

key-decisions:
  - "errDupWatch 置于 import 后包级（exitWatcher 注释块前），注释自带 Pitfall 9 机理与调用方兜底闭环登记"
  - "awaitExit ④注释落 watch-error 分支行内（plan「在该分支注释补一句」字面落点），分支逻辑零改动"
  - "PC-03 勾选留给 phase 末 plan 11-06 延续（11-01 既定：ID 跨 6 plan 共享，本 plan 仅平台收割防御面，flagged_assumptions 保持 unverified）"

requirements-completed: []

coverage:
  - id: D1
    description: "watch() 对重复 pid 注册 fail-closed：errDupWatch 返回 + nil channel，先注册订阅零影子化（Pitfall 9 防御，T-11-02a mitigate）"
    requirement: PC-03
    verification:
      - kind: other
        ref: "GOOS=darwin go build ./internal/pty/ && GOOS=darwin go vet ./internal/pty/（darwin 编译闸，本机通过）"
        status: pass
      - kind: unit
        ref: "internal/pty/reap_darwin_test.go#TestWatchDupPidFailClosed"
        status: unknown
    human_judgment: true
    rationale: "darwin-only 测试在本机（Linux）由 build tag 排除，仅经 GOOS=darwin 编译/vet 闸验证；实际运行由 CI macos-latest leg 承担（Phase 1 双平台 CI 矩阵既有），推送后核验——plan flagged_assumptions 既定口径"
  - id: D2
    description: "awaitExit 对 errDupWatch 退化 cmd.Wait() 阻塞直等（既有 watch-error 分支零改动消费，唯一收割者纪律保持）"
    requirement: PC-03
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./...（全仓 5 包全绿）+ grep 锁（return cmd.Wait() ≥1 / dup 检查 ==1 / errDupWatch ≥2）"
        status: pass
    human_judgment: false
  - id: D3
    description: "零回归收口闸：shared 全量 Go 测试原样绿 + 期望值逐字未动；Linux 侧零改动（reap_linux.go/signal_linux.go 无 diff）；reap_darwin_test.go append-only（diff 零删除行）"
    verification:
      - kind: other
        ref: "git diff --stat（仅两 darwin 文件，+71/-0）；git diff -U0 reap_darwin_test.go 删除行 ==0；gofmt -l 零输出"
        status: pass
    human_judgment: false

duration: 9min
completed: 2026-09-03
status: complete
---

# Phase 11 Plan 02: darwin exit watcher dup-watch fail-closed Summary

**darwin 共享 kqueue exitWatcher 的 Pitfall 9 挂账兑现：watch() 对重复 pid 注册 fail-closed（errDupWatch），影子注册从「会话收割挂死」变「可观测错误 → awaitExit 既有分支退化 cmd.Wait() 兜底」；TestWatchDupPidFailClosed append-only 锁进 CI macOS leg；Linux 侧零漂移、全仓 -race 全绿。**

## Performance

- **Duration:** 9 min
- **Started:** 2026-09-03T17:08:24Z
- **Completed:** 2026-09-03T17:16:55Z
- **Tasks:** 2
- **Files modified:** 2（均为 darwin build tag 文件，+71/-0）

## Accomplishments

- `errDupWatch` 包级错误值落地（errors.New 单点定义，文案 "pty: duplicate exit watch pid" 零敏感值）；注释登记 Pitfall 9 完整机理链（kqueue (ident,filter) 唯一键 → 重复 EV_ADD 是替换非叠加 → subs[pid] 覆盖影子化先注册者 channel → awaitExit `<-exited` 永等 → 收割挂死）与 fail-closed 处置闭环
- `watch()` dup 检查插入 w.mu 内 subs 赋值之前：重复 pid 拒绝（unlock + return nil, errDupWatch），已登记订阅零改动；注册失败摘除分支（:54-60 区）与正常路径逐字不动；w.mu 既有保护面零变更（无新锁类型，prohibition 两条均闭合）
- `awaitExit` 既有 watch-error 分支注释补 errDupWatch 唯一预期错误源——退化 cmd.Wait() 兜底形态保持，调用方零新面
- `TestWatchDupPidFailClosed` 追加（append-only）：同 pid 二次注册 errors.Is(err, errDupWatch) + nil channel 断言；反向锁——首订阅不被影子化（helper 退出后首 channel 收 NOTE_EXIT + Wait 退出码 42）；注释登记 darwin-only 运行位（CI macOS leg）与 Phase 14 挂账（N 路并发退出 + 混合僵尸竞态复演）

## Task Commits

Each task was committed atomically:

1. **Task 1: reap_darwin.go watch() dup-watch fail-closed 防御 + errDupWatch** - `430fd1b` (fix)
2. **Task 2: reap_darwin_test.go 新增 TestWatchDupPidFailClosed** - `aa8d966` (test)

**Plan metadata:** 见文末最终 docs commit（SUMMARY/STATE/ROADMAP）

## Files Created/Modified

- `internal/pty/reap_darwin.go` — errors 导入补 + errDupWatch 包级错误值（Pitfall 9 注释块）+ watch() dup 检查分支 + exitWatcher 文件头 11-02 落地锚定行 + awaitExit watch-error 分支行内注释（+18/-0）
- `internal/pty/reap_darwin_test.go` — TestWatchDupPidFailClosed 追加（+53/-0，既有测试函数逐字未动）

## Decisions Made

- **errDupWatch 放置位**：import 块之后、exitWatcher 注释块之前的包级单点——错误值先于其唯一消费类型出现，注释块自带机理与兜底闭环登记（对齐文件既有注释密度纪律）
- **awaitExit 注释落点**：plan ④「在该分支注释补一句」按字面落 watch-error 分支行内两行至 :132-133，分支逻辑（return cmd.Wait()）零改动
- **PC-03 需求勾选**：延续 11-01 既定决策（ID 跨 6 plan 共享，phase 末 11-06 统一勾选），本 plan flagged_assumptions 明示 PC-03 保持 flagged-unverified；requirements-completed 留空防可追溯表污染

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None——darwin 编译闸（GOOS=darwin build/vet）与 Linux 全量 -race 首轮全绿，无返工；全部 acceptance grep 一次通过。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 11-03/11-04/11-05（容量再闸/spawn 失败/竞态注入/EXIT 断言面）：本 plan 与 11-01 主干文件面零重叠（internal/pty 平台文件 vs internal/server+cmd/wesh），Wave 1 并行无冲突
- CI macOS leg：推送后 TestWatchDupPidFailClosed 随 Phase 1 双平台矩阵自动运行；Q1 双测（TestKqueueExitNormal/ZombieRace）同 leg 既有运行
- Phase 14 挂账（注释已锚定）：N 路并发退出 + 混合僵尸竞态复演（Pitfall 9 剩余面——subs map 生命周期单调增长路径排查与 Q1 裁决 N 规模外推）
- 威胁登记 T-11-02a（重复 pid 注册影子化 DoS，medium）→ mitigate 闭合：fail-closed + 退化兜底 + CI 测试锁三件套就位

## Self-Check: PASSED

- 文件存在性：internal/pty/reap_darwin.go / internal/pty/reap_darwin_test.go / 11-02-SUMMARY.md 全部 FOUND
- 提交存在性：430fd1b（Task 1 fix）/ aa8d966（Task 2 test）全部 FOUND
- 删除检查：两提交 `git diff --diff-filter=D` 均无删除；plan 全 diff +71/-0

---
*Phase: 11-per-client*
*Completed: 2026-09-03*
