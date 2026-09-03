---
phase: 10-mode-assembly
plan: 02
subsystem: infra
tags: [go, startup-validation, session-mode, pty, table-driven-tests]

requires:
  - phase: 10-mode-assembly plan 01
    provides: cfg.sessionMode/sessionModeSet/argv0 字段、SessionMode 常量、Options.SessionMode/SpawnFunc 接缝、ValidateOptions 互斥校验、pty.StartWithSize 导出
provides:
  - validateStartup write-policy×per-client 组合 warn（D-01/D-02，warn 明示放行——静默永不接受兑现）
  - validateStartup per-client LookPath(argv0) 启动预检（SC4——命令缺失暴露在启动期而非首个 attach）
  - warn 累积合并透出形态（mergeWarn——socket/loopback 早退与三既有 warn 返回点拼接，既有文案逐字未动）
  - TestStartupMatrix 八追加行 + TestValidateStartupWarnMerge 组合测试
  - internal/server/options_test.go【新】TestValidateOptions 三态契约表驱动锁
  - internal/pty/spawn_test.go 追加 TestStartWithSizeDelegation（委托等价 + 132x43 尺寸面）
affects: [phase-11 生命周期主干, phase-12 交互背压, phase-14 终结语义, 10-03, 10-04]

actuals:
  tokens: 3200
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "warn 累积合并透出：函数内 []string 累积 + mergeWarn(sec) 闭包拼接（安全警告在前显著性优先）——新 warn 在 socket/loopback 早退也可达且零遮蔽既有警告"
    - "启动预检只读探测扩展：exec.LookPath 与 os.Stat 同档（纯函数纪律内，--cwd 先例）"

key-files:
  created:
    - internal/server/options_test.go
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - internal/pty/spawn_test.go

key-decisions:
  - "D-01/D-02 落地形态：锚定 writePolicySet × 模式终值（非 sessionModeSet——终值判定即双源覆盖）；warn 文案含 --write-policy/--session-mode 双 flag 名 + ro/rw 级别仍按 ticket 生效注记"
  - "SC4 LookPath 预检仅 per-client × argv0 非空触发——shared（含零值模式）零漂移（spawn 失败仍走 pty.Start exit 1 现状通道）"

patterns-established:
  - "mergeWarn 合并透出：早退点 mergeWarn(\"\") / 既有 warn 点 mergeWarn(原文)——strings.Join(nil) == \"\" 使空累积零漂移"
  - "组合面与主矩阵断言形态不同即独立小函数（TestValidateStartupWarnMerge，TestClientOptionError 分函数先例）"

requirements-completed: [PC-01]

coverage:
  - id: D1
    description: "write-policy×per-client warn（CLI/TOML 双源同档触发、双 flag 名文案、shared/未显式两面不触发）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix（四追加行）"
        status: pass
      - kind: e2e
        ref: "冒烟：wesh --writable --write-policy=all --session-mode=per-client -- sh → warn 行 + listening on"
        status: pass
    human_judgment: false
  - id: D2
    description: "warn 合并不遮蔽（非 loopback 安全警告与新 warn 同现；socket 早退透出）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestValidateStartupWarnMerge（两形态）"
        status: pass
      - kind: e2e
        ref: "冒烟：+ --no-auth 非 loopback → stderr 两类警告同现"
        status: pass
    human_judgment: false
  - id: D3
    description: "per-client LookPath(argv0) 启动预检（SC4：缺失拒绝/可执行放行/shared 不预检/空串不触发）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix（四追加行）"
        status: pass
      - kind: e2e
        ref: "冒烟：wesh --session-mode=per-client -- wesh-no-such-cmd-7f3a → exit 2 + 文案"
        status: pass
    human_judgment: false
  - id: D4
    description: "ValidateOptions 三态互斥契约（两拒绝 + 两放行 + 零值归一）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "internal/server/options_test.go#TestValidateOptions（五态）"
        status: pass
    human_judgment: false
  - id: D5
    description: "Start ≡ StartWithSize(argv, opts, SpawnCols, SpawnRows) 委托等价 + 132x43 尺寸真实到达 TIOCSWINSZ"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "internal/pty/spawn_test.go#TestStartWithSizeDelegation（两面）"
        status: pass
    human_judgment: false

duration: 35 min
completed: 2026-09-02
status: complete
---

# Phase 10 Plan 02: validateStartup 双行与接缝契约 Summary

**write-policy×per-client warn（D-01/D-02）与 per-client LookPath 启动预检（SC4）落地，warn 累积合并形态零遮蔽既有安全警告；ValidateOptions 三态与 StartWithSize 委托等价经新测试锁定，shared 路径零漂移（全量 -race 绿）。**

## Performance

- **Duration:** 35 min（2026-09-02T12:20Z → 12:58Z；含 gsd-executor 子代理通道 400 故障后的内联接管）
- **Tasks:** 2/2
- **Commits:** 2
- **Files:** 4 modified（1 新建）

## Accomplishments

- validateStartup 新增 write-policy×per-client 组合 warn：锚定 `writePolicySet && sessionMode == SessionModePerClient`（D-02 显式位 × 模式终值），文案含双 flag 名与「ro/rw 权限级别仍按 ticket 生效」注记（D-01 否决 exit 2 的理据入文案）
- warn 累积合并透出形态：`modeWarns []string` + `mergeWarn(sec)` 闭包——socket/loopback 早退 `mergeWarn("")` 透出、三既有 warn 返回点 `mergeWarn(原文)` 拼接（安全警告在前显著性优先）；既有 warn 文案逐字未动（既有矩阵行零改动通过即证据）
- per-client LookPath(argv0) 启动预检（--cwd stat 同位，纯函数只读探测纪律内）：`invalid command %q: not found in PATH (per-client startup preflight)`——命令名回显 + not found 语义 + per-client 注记三要素
- TestStartupMatrix 八追加行（warn 双形态/两不触发面/LookPath 拒绝×2/放行/shared 不预检）+ TestValidateStartupWarnMerge 两形态（非 loopback 合并 + socket 透出）
- internal/server/options_test.go【新文件】TestValidateOptions 五态表驱动（per-client×nil 拒 / shared×非nil 拒 / 合法两态 / 零值归一）
- internal/pty/spawn_test.go 追加 TestStartWithSizeDelegation：委托等价面（Env slices.Equal + Dir + SysProcAttr nil 性 + 两会话正常退出）与 132x43 尺寸面（GetsizeFull 读回 TIOCSWINSZ）

## Task Commits

1. `1b2419b` — feat(10-02): validateStartup session-mode rows — write-policy x per-client warn (D-01/D-02) + per-client LookPath preflight (SC4) (PC-01)
2. `5aaa065` — test(10-02): ValidateOptions mutex contract + StartWithSize delegation parity (PC-01)

## Verification

- `go test ./cmd/wesh -run 'TestStartupMatrix$|TestValidateStartupWarnMerge$' -count=1` PASS（八追加行 + 合并两形态）
- `go test ./internal/server -run 'TestValidateOptions$' -count=1 -v` 五态全 PASS
- `go test ./internal/pty -run 'TestStartWithSizeDelegation$|TestStartZeroValueParity$' -count=1 -v` 全 PASS（既有 parity 原样绿）
- `go build ./...` / `go vet ./...` 干净；`gofmt -l` 零输出
- `go test -race -count=1 ./...` 五包全绿（1m0.9s）
- acceptance grep：`sessionMode == server.SessionModePerClient` ×3（≥2）、`exec.LookPath` ×2（≥1）、`func TestValidateOptions`/`func TestStartWithSizeDelegation` 各 ×1；`git diff -U0 main_test.go / spawn_test.go` 删除行 == 0（append-only）
- 冒烟三命令（success criteria 进程级实证）：
  - `--writable --write-policy=all --session-mode=per-client -- sh` → stderr warn 含双 flag 名 + listening on 正常启动（D-01 放行语义）✓
  - `--session-mode=per-client -- wesh-no-such-cmd-7f3a` → exit 2 + `invalid command "wesh-no-such-cmd-7f3a": not found in PATH (per-client startup preflight)`（SC4）✓
  - `--bind 0.0.0.0 --no-auth --writable --write-policy=all --session-mode=per-client -- sh` → stderr 同时出现 --no-auth 安全警告与 --write-policy/--session-mode warn（合并不遮蔽）✓

## Deviations from Plan

**[Rule 1 - 文案断言适配] TestValidateOptions shared×非nil 行断言子串** — Found during: Task 2 | Plan 写「err 含 `SessionMode`」，10-01 落地文案为 `server: session-mode shared must not set SpawnFunc`（camelCase `SessionMode` 不存在于文案）；采用分支互斥子串 `must not set SpawnFunc`（与 per-client 行的 `requires SpawnFunc` 分锁），10-01 落地文案零改动 | Files: internal/server/options_test.go | Verification: 五态全 PASS | Commit: 5aaa065

**执行方式偏差：** gsd-executor 子代理连续三次被模型服务商 400 拒绝（10-01 同通道正常），按 execute-phase runtime_compatibility fallback 规则改为编排器内联执行（Pattern C）；执行内容/验收/提交纪律与子代理形态完全一致。

**Total deviations:** 1 auto-fixed（断言子串适配，零行为变更）+ 1 执行方式偏差登记。**Impact:** 无——全部 must_haves truths 与 acceptance criteria 达成。

## Issues Encountered

None（子代理通道故障已在偏差节登记，不影响产物）。

## Authentication Gates

None.

## User Setup

None.

## Next Step

Ready for 10-03（session-mode TOML 三面证据 + fuzz 语料扩展）——同 wave 2，无 files 重叠，串行执行。

## Self-Check: PASSED
