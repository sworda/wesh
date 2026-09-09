---
phase: 01-pty
plan: 04
subsystem: infra
tags: [darwin, kqueue, EVFILT_PROC, github-actions, ci, pnpm, go]

requires:
  - phase: 01-pty (plan 01-01)
    provides: reap_linux.go 的 awaitExit(cmd *exec.Cmd) error 签名基准、spawn.go 的 whitelistEnv（SEC-06）
provides:
  - darwin 进程级共享 kqueue exit watcher（exitWatcher/newExitWatcher/watch/loop）+ 跨平台统一签名 awaitExit
  - Q1 僵尸注册竞态裁决双测试（TestKqueueExitNormal / TestKqueueExitZombieRace，CI-only）
  - 双平台 CI（go 矩阵 ubuntu+macos / web 构建 job），macos leg 承担 Q1 运行时裁决
affects: [01-05, phase-05-multi-session, phase-08-logging, phase-09-release]

actuals:
  tokens: 2409
  tasks: 3
  commits: 3

tech-stack:
  added: [GitHub Actions ci.yml（checkout@v7.0.1 / setup-go@v7.0.0 / pnpm-action-setup@v6.0.10 / setup-node@v4）]
  patterns:
    - "平台收割统一签名：awaitExit(cmd *exec.Cmd) error，调用点零平台分支；watcher 只做早知，cmd.Wait() 唯一收割"
    - "TestHelperProcess argv 守卫（-- 后 wesh-helper-* 标记），不走 env（SEC-06 白名单剥离自定义变量）"
    - "CI 竞态裁决模式：超时分支 t.Skip + 裁决标记（兜底为计划内路径，非 fail）"

key-files:
  created:
    - internal/pty/reap_darwin.go
    - internal/pty/reap_darwin_test.go
    - .github/workflows/ci.yml
  modified: []

key-decisions:
  - "darwin awaitExit 经包级 sync.Once 单例 watcher；初始化/注册失败均退化为直接 cmd.Wait()（兜底不致命）"
  - "CI 显式钉 pnpm 11.21.0：web/package.json 无 packageManager 字段，pnpm/action-setup 需要版本源"

patterns-established:
  - "共享 watcher 单例 + EV_ONESHOT：进程级一个 kqueue fd 一个 goroutine，N 会话共用（D-14）"
  - "argv 守卫 helper 模式：-test.run=TestHelperProcess -- <标记>，os.Args 判定分派"

requirements-completed: [CORE-01]

coverage:
  - id: D1
    description: "darwin 共享 kqueue exit watcher 编译期完备（reap_darwin.go），awaitExit 签名与 linux 统一"
    requirement: CORE-01
    verification:
      - kind: other
        ref: "GOOS=darwin GOARCH=amd64 go build ./... && GOOS=darwin GOARCH=arm64 go build ./... && GOOS=darwin GOARCH=arm64 go vet ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Q1 竞态裁决双测试落地（reap_darwin_test.go）——运行时裁决待 CI macos-latest leg 首推执行"
    requirement: CORE-01
    verification:
      - kind: integration
        ref: "GOOS=darwin GOARCH=amd64 go test -c -o /dev/null ./internal/pty（编译通过）；go test ./internal/pty -run TestKqueue -count=1（macos runner，待首推）"
        status: unknown
    human_judgment: true
    rationale: "本机无 macOS（RESEARCH Environment Availability），kqueue 运行时行为只能由 CI macos leg 首推裁决；两条出路（成立/兜底 skip）均为计划内路径"
  - id: D3
    description: "双平台 CI workflow（.github/workflows/ci.yml）：go 矩阵 ubuntu+macos + web 构建 job"
    verification:
      - kind: other
        ref: "grep 结构断言：macos-latest / go test -race -count=1 / ubuntu-latest / pnpm -C web install --frozen-lockfile / pnpm -C web build 全部命中，无 CGO_ENABLED=0"
        status: pass
    human_judgment: false

duration: 4min
completed: 2026-08-14
status: complete
---

# Phase 01 Plan 04: darwin 收割模型 + 双平台 CI Summary

**darwin 共享 kqueue exit watcher（EVFILT_PROC/NOTE_EXIT 早知 + cmd.Wait() 唯一收割）编译期完备，Q1 僵尸注册竞态双测试就位，双平台 CI（ubuntu+macos go 矩阵 / pnpm web 构建）落地——运行时裁决待 CI 首推。**

## Performance

- **Duration:** 4 min
- **Started:** 2026-08-14T02:02:52Z
- **Completed:** 2026-08-14T02:07:16Z
- **Tasks:** 3
- **Files modified:** 3（全部新建）

## Accomplishments

- darwin 半边收割模型补全：进程级共享 kqueue watcher（一 fd 一 goroutine，N 会话共用，零每会话线程），`EV_ADD|EV_ONESHOT` 注册，`awaitExit(cmd *exec.Cmd) error` 与 linux 签名统一、调用点零平台分支；交叉编译（amd64+arm64）与 vet 全绿，linux 本机无回归
- Q1 竞态裁决双测试编入 CI 运行面：正常路径断言事件到达 + 退出码 42；竞态路径"先 sleep 200ms 成僵尸再注册"，1s 超时分支 `t.Skip` + `Q1-VERDICT` 裁决标记——兜底退化（直接 cmd.Wait()）为计划内路径，不产生 checkpoint
- 双平台 CI 落地：go job 矩阵 ubuntu-latest+macos-latest 跑 `go vet ./...` 与 `go test -race -count=1 ./...`（不设 CGO_ENABLED=0，Pitfall 5）；web job 跑 `pnpm -C web install --frozen-lockfile` 与 `pnpm -C web build`（D-18 构建顺序固化）；actions 全部钉版

## Task Commits

Each task was committed atomically:

1. **Task 1: darwin 共享 kqueue exit watcher（reap_darwin.go）** - `b66f29b` (feat)
2. **Task 2: Q1 竞态裁决双测试（reap_darwin_test.go，CI-only）** - `0caa01d` (test)
3. **Task 3: 双平台 CI（.github/workflows/ci.yml）** - `ff74dda` (chore)

**Plan metadata:** 见最终 docs 提交（docs(01-04): complete ...）

## Files Created/Modified

- `internal/pty/reap_darwin.go` - darwin 共享 kqueue exit watcher + 统一签名 awaitExit；文件头登记 Q1 兜底预案与禁手写 reap 纪律
- `internal/pty/reap_darwin_test.go` - TestKqueueExitNormal / TestKqueueExitZombieRace + TestHelperProcess（argv 守卫）
- `.github/workflows/ci.yml` - go 矩阵（ubuntu+macos）+ web 构建 job，actions 全钉版

## Decisions Made

- **darwin awaitExit 经包级 sync.Once 单例 watcher**：初始化失败或 watch 注册失败均退化为直接 `cmd.Wait()`——兜底不致命，与 RESEARCH"正确兜底 = 退回 Wait goroutine"的修正决议一致（禁 SIGCHLD+WNOHANG 手动 reap）
- **CI 显式钉 pnpm 11.21.0**：web/package.json 无 `packageManager` 字段，pnpm/action-setup 将无版本源而失败；钉与本地一致的版本（lockfileVersion 9.0 兼容），属 CONTEXT.md "Claude's Discretion: CI yaml 细节微调"范围
- **竞态测试注册失败（如 ESRCH）等同"不补发"裁决**：走 t.Skipf + Q1-VERDICT 标记，与超时分支同一兜底语义

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] watch() 注册失败路径摘除 subs 订阅，防 map 泄漏**
- **Found during:** Task 1（darwin 共享 kqueue exit watcher）
- **Issue:** RESEARCH 骨架在 `unix.Kevent` 注册失败时仍把 chan 留在 `subs` map 中返回——每次失败的 watch 永久泄漏一条 map 项与一个 channel
- **Fix:** 注册失败时 `delete(w.subs, pid)` 后返回 `nil, err`
- **Files modified:** internal/pty/reap_darwin.go
- **Verification:** darwin amd64/arm64 交叉编译 + vet 通过
- **Committed in:** `b66f29b`（Task 1 提交）

**2. [Rule 1 - Bug] os.Executable() 双返回值用于单值上下文，测试编译失败**
- **Found during:** Task 2（Q1 竞态裁决双测试）首次 `GOOS=darwin go vet`
- **Issue:** `exec.Command(os.Executable(), ...)` 编译错误：`multiple-value os.Executable() in single-value context`
- **Fix:** 先绑定 `exe, err := os.Executable()` 并判错，再传入 exec.Command
- **Files modified:** internal/pty/reap_darwin_test.go
- **Verification:** `GOOS=darwin GOARCH=arm64 go vet ./internal/pty` 与 `GOOS=darwin GOARCH=amd64 go test -c` 通过
- **Committed in:** `0caa01d`（Task 2 提交）

**3. [Rule 3 - Blocking] web/package.json 无 packageManager 字段，pnpm/action-setup 无版本源**
- **Found during:** Task 3（双平台 CI）
- **Issue:** 计划描述 pnpm/action-setup@v6.0.10 "读 package.json packageManager 字段"，但该字段不存在且 package.json 在 web/ 子目录——不处理则 CI web job 首步即失败
- **Fix:** 在 action `with:` 显式钉 `version: 11.21.0`（与本地 pnpm 一致，lockfileVersion 9.0 兼容）；同时将注释中的字面量 `CGO_ENABLED=0` 改写为"不设 CGO_ENABLED（保持默认启用）"，避免与验收断言"全文无 CGO_ENABLED=0"的字面 grep 冲突
- **Files modified:** .github/workflows/ci.yml
- **Verification:** 全部 grep 结构断言通过；`grep -c 'CGO_ENABLED=0'` 返回 0
- **Committed in:** `ff74dda`（Task 3 提交）

---

**Total deviations:** 3 auto-fixed（1 missing critical、1 bug、1 blocking）
**Impact on plan:** 三处均为正确性/可运行性必需修复，无范围蔓延；CI yaml 微调在 CONTEXT.md 授权范围内。

## Issues Encountered

- 本机无 macOS 是既定事实（RESEARCH Environment Availability）——darwin 运行时验证完全由 CI macos-latest leg 承担，本机侧仅做交叉编译 + vet + 测试编译（`go test -c`），与计划一致。

## User Setup Required

None - no external service configuration required.（CI 首次推送后 GitHub Actions 自动运行，无需手动配置。）

## Next Phase Readiness

- 01-05（本 phase 最后一个 plan）可直接继续；darwin 收割路径编译期完备，运行时行为不阻塞 linux 侧任何工作
- **待办（推送后）：** CI 首推观察 macos leg 的 TestKqueue 结果——若 `TestKqueueExitZombieRace` 出现 `Q1-VERDICT` skip，执行兜底：reap_darwin.go 的 awaitExit 退化为直接 `cmd.Wait()`，watcher 代码以 build tag 保留待 Phase 5 重估（两条出路均为计划内路径）

## Self-Check: PASSED

- 文件全部存在：internal/pty/reap_darwin.go、internal/pty/reap_darwin_test.go、.github/workflows/ci.yml、01-04-SUMMARY.md
- 提交全部存在：b66f29b（Task 1）、0caa01d（Task 2）、ff74dda（Task 3）

---
*Phase: 01-pty*
*Completed: 2026-08-14*
