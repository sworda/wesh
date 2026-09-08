---
phase: 11-per-client
plan: "07"
subsystem: testing
tags: [gap-closure, esrch, eperm, macos-ci, test-helper, probe-injection, ps1-interleave]

requires:
  - phase: 11-per-client
    plan: "04"
    provides: "waitPgroupESRCH 原始形态（kill(-pgid, 0) 非 ESRCH 立即 Fatal 的 11-04 期断言）——G-11-2 的缺陷载体与四调用点格局"
provides:
  - "waitPgroupESRCHWithProbe 探针参数化核心（error 返回形态）：EPERM 归类「进程组仍存在」形态落入护栏轮询，护栏到期未 ESRCH 仍 Fatal，其余非 ESRCH 非 EPERM 错误维持立即 Fatal"
  - "TestWaitPgroupESRCHProbeSemantics 四子测（瞬态容忍收敛/持续到期翻车/他错立即失败/无错存活）——EPERM 分支行为在 Linux 的唯一确定性锁定通道"
  - "PS1 交错容忍正则（(?:\$ )?）三处：TestPerClientInputEcho / echoMarker / TestPerClientExitPrivate42 B 端——CI 慢 runner shell 冷启动时序鲁棒性"
affects: [12-stop-read, 13-termination, 14-matrix]

actuals:
  tokens: 2300
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "探针参数化（probe injection）：平台不可达分支（Linux 同 uid 下 EPERM 结构性不可达）经脚本化探针闭包注入锁定语义——真实夹具（异 uid 子进程）破坏被测语义时的唯一出路"
    - "PS1 交错容忍：终端测试断言对 shell 启动提示符与回显/结果行时序交错的鲁棒性形态（(?:\$ )? 前缀容忍——结果行锚定与回显行排除语义不变）"

key-files:
  created: []
  modified:
    - internal/server/perclient_test.go

key-decisions:
  - "单一文件门基点按 plan 规则以实际起始 HEAD 975af23 替换假设基点 f55c1ea（两者间仅 7358b82/975af23 两个 .planning-only 提交，f55c1ea 交叉核对同结果：恰 internal/server/perclient_test.go 一文件）"
  - "EPERM 修复的证据主载体 = 确定性单测（四子测），CI 复跑承载「FAIL 形态不再可达」的运行面确认——复跑 PASS 不单独证明修复生效（µs 窗瞬态本轮可能未命中），证据边界按 plan flagged_assumptions 口径记录"
  - "Task 3 复验中发现的 ubuntu flaky（PS1 交错）以第三提交 9936f2b 修正：属 CI 环境时序鲁棒性缺陷非断言语义弱化（六案例正则自检：交错形态命中 + 回显行/PS1+回显行排除），记录为 Deviation 而非 plan 范围内工作"

requirements-completed: []

coverage:
  - id: D1
    description: "waitPgroupESRCH EPERM 容忍语义：EPERM 与探针无错同归「进程组仍存在」形态落入护栏轮询（护栏内 ESRCH 收敛 PASS / 到期未 ESRCH 仍 Fatal / 其余错误立即 Fatal）——G-11-2 missing item 1"
    requirement: PC-03
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestWaitPgroupESRCHProbeSemantics（四子测，本机 -race 4/4 PASS + CI 33844831146 macOS leg 4/4 PASS）"
        status: pass
      - kind: other
        ref: "T1 grep 断言链（旧论断计数 0 / CI run 33832096581 引证 ≥1 / 核心函数单点定义 ==1）"
        status: pass
    human_judgment: false
  - id: D2
    description: "函数头注释实证修正：macOS XNU 退出过渡态 EPERM 可达性（CI run 33832096581 实证）+ macOS 组信号语义差异注记 + 被证伪旧论断整句移除——G-11-2 missing item 2"
    requirement: PC-03
    verification:
      - kind: other
        ref: "grep 'EPERM 不可达' internal/server/perclient_test.go ==0（旧论断归零）；grep '33832096581' ≥1（实证引证在案）；GOROOT gofmt -l 零输出（CJK doc-comment 规则）"
        status: pass
    human_judgment: false
  - id: D3
    description: "零回归收口闸：全量 -race 五包 ok + darwin build/vet 双闸 + gofmt + 单一文件 diff 门（生产代码零改动）"
    requirement: PC-04
    verification:
      - kind: integration
        ref: "go vet ./... && go test -race -count=1 ./...（5 包 ok，本机两轮 1m6.6s/1m7.2s）&& GOOS=darwin go build/vet ./... && gofmt 零输出 && git diff --name-only 975af23..HEAD -- internal/ cmd/ web/ == internal/server/perclient_test.go"
        status: pass
    human_judgment: false
  - id: D4
    description: "CI macOS leg 运行面确认：TestPerClientTeardownRaceOnce PASS（33832096581 的 FAIL 现场测试转绿）+ 同 helper 三调用点保持 PASS + internal/server darwin 运行面全绿（G-11-2 truth 原文）"
    requirement: PC-03
    verification:
      - kind: other
        ref: "CI run 33844831146 macOS leg：TestPerClientTeardownRaceOnce PASS (1.40s) / DisconnectSIGHUP (0.04s) / ReconnectNewPid (0.05s) / StopTimeoutKillFallback (1.02s) / TestWatchDupPidFailClosed (0.11s) / TestKqueueExitZombieRace (1.10s) / internal/server ok (66.4s)；ubuntu leg 同绿"
        status: pass
    human_judgment: false

duration: 1h 50min
completed: 2026-09-04
status: complete
---

# Phase 11 Plan 07: G-11-2 waitPgroupESRCH EPERM 容忍（gap closure）Summary

**G-11-2 闭合：waitPgroupESRCH 重构为探针参数化双层形态（waitPgroupESRCHWithProbe 核心 + 薄包装），EPERM 按 POSIX「目标存在」语义归类存活形态落入护栏轮询——护栏保留与他错立即 Fatal 两半边由四子测确定性锁定；CI 复验链闭环（33844831146 双平台全绿，FAIL 现场测试转绿）；副产 PS1 交错容忍修正三处。**

## Performance

- **Duration:** 1h 50min（含两轮 CI 复验等待）
- **Started:** 2026-09-04T04:00:00Z
- **Completed:** 2026-09-04T06:40:00Z
- **Tasks:** 3（Task 1 TDD 修复 / Task 2 零回归收口闸 / Task 3 CI macOS leg 确认门）
- **Files modified:** 1（internal/server/perclient_test.go，+120/-12）

## Accomplishments

- **TDD 对完成**：`afb77a8`（test——四子测 RED）+ `5aad25a`（fix——核心实现 GREEN）。waitPgroupESRCHWithProbe 探针参数化核心（error 返回形态，不接 testing.T）：EPERM 归类「进程组仍存在」形态落入 deadline 续轮询；护栏到期未 ESRCH 仍 Fatal（僵尸残留检测保留）；其余非 ESRCH 非 EPERM 错误立即 Fatal（文案逐字不变）；护栏到期文案更新为存活探针两形态明示（含「护栏」与「EPERM」字样）
- **头注释五要点重写**（missing item 2）：macOS XNU 退出过渡态 EPERM 可达（P_LIST_EXITED/proc_exit 进行中，实证 = CI run 33832096581——TeardownRaceOnce 首探针 µs 级紧贴 closeCh 确认唯一命中，同断言三测试未命中排除环境级拦截）；POSIX 语义 EPERM = 目标存在但权限拒绝 ≠ 消失；「ESRCH ⊇ 死亡且收割完成」的 macOS 组信号语义差异补注（形态不同、结论同向）；EPERM 护栏轮询两半边；其余错误立即 Fatal。被证伪旧论断「EPERM 不可达」整句移除（grep 计数归零）
- **零回归收口闸全绿**（两轮：执行期 + ship 复核期）：go vet 零输出；全量 -race 五包 ok（1m6.6s / 1m7.2s）；GOOS=darwin build/vet 双闸 exit 0；GOROOT gofmt 零输出；单一文件门 `git diff --name-only 975af23..HEAD -- internal/ cmd/ web/` == internal/server/perclient_test.go（生产代码零改动实证，基点按 plan flagged_assumptions 以实际起始 HEAD 975af23 替换）
- **CI 运行面确认门闭环**（Task 3）：复验两轮——33843785651（ubuntu flaky 连带取消 macOS）→ **33844831146 双平台全绿**：macOS leg TestPerClientTeardownRaceOnce PASS (1.40s，33832096581 的 FAIL 0.01s 现场测试转绿)、同 helper 三调用点保持 PASS、TestWatchDupPidFailClosed/TestKqueueExitZombieRace PASS（darwin 面再证）、internal/server ok (66.4s)。护栏翻车形态（EPERM 持续到期）未出现——修法充分性经运行面确认

## Task Commits

Each task was committed atomically:

1. **Task 1: G-11-2 语义修复（探针参数化核心 + 四案例单测 + 头注释实证修正，TDD）** - `afb77a8` (test) + `5aad25a` (fix)
2. **Task 2: 零回归收口闸（只读断言 + 证据归集）** - 无独立提交（证据归档于本 SUMMARY，plan Task 2 定义为只读收口闸）
3. **Task 3: CI macOS leg 复跑确认（人工确认门）** - 证据 = CI run 33844831146（复验中间产物 9936f2b 见 Deviations）

**Plan metadata:** `7358b82`（docs: plan 创建）+ `975af23`（docs: checker 反馈修订）

## Files Created/Modified

- `internal/server/perclient_test.go` — waitPgroupESRCH 段重构（薄包装 + waitPgroupESRCHWithProbe 核心 + EPERM 容忍语义 + 重写头注释）+ TestWaitPgroupESRCHProbeSemantics 四子测 + scriptedProbe 闭包（+108/-7）+ PS1 交错容忍三处（+12/-5）

## Decisions Made

- **单一文件门基点替换**：plan flagged_assumptions 假设基点 f55c1ea，实际执行起始 HEAD 为 975af23（两者间仅 7358b82/975af23 两个 .planning-only 提交）——以 975af23 为门基点，f55c1ea 交叉核对同结果
- **EPERM 修复证据结构**：修复行为的主证据 = 四子测确定性单测（Linux 同 uid 下 EPERM 分支结构性不可达、macOS 瞬态非确定——脚本化探针注入是唯一确定性锁定通道）；CI 复跑承载运行面「FAIL 形态不再可达」确认，复跑 PASS 不单独证明修复生效（µs 窗瞬态可能未命中）——防证据错位
- **护栏到期文案更新**：含「护栏」与「EPERM」字样 + 存活探针两形态明示（无错与 EPERM 同为「进程组仍存在」）——「严禁把探针无错当死亡证据」红线保留

## Deviations from Plan

### Auto-fixed Issues

**1. [CI 环境时序 - 计划外] PS1 交错容忍修正（ubuntu flaky）**
- **Found during:** Task 3（CI 复验第一轮 33843785651）
- **Issue:** ubuntu leg TestPerClientInputEcho / TestPerClientSpawnFailure FAIL (10s 统护)——CI 慢 runner 上 shell 冷启动慢，PS1 打印晚于 tty 回显，落在回显行与结果行之间（累积串 "echo X\r\n$ X\r\n$ " 实证），行首锚定 ^X 失配。macOS leg 被连带取消。与 G-11-2 修复无关（两测试不经 waitPgroupESRCH）；上轮 CI ubuntu PASS 同代码——纯时序 flaky
- **Fix:** `(?:\$ )?` 可选前缀容忍三处（TestPerClientInputEcho :317 / echoMarker :431 / TestPerClientExitPrivate42 B 端 :986 同源风险一并修正）；结果行锚定与回显行排除语义不变（"$ echo X" 行首 "$ echo" 仍不命中）
- **Files modified:** internal/server/perclient_test.go
- **Verification:** 六案例正则自检（交错形态命中 + 回显行/PS1+回显行/仅 PS1 排除）全过；本机全量 -race 五包 ok；CI run 33844831146 ubuntu+macOS 双绿
- **Committed in:** `9936f2b`

---

**Total deviations:** 1 auto-fixed（CI 环境时序鲁棒性）
**Impact on plan:** 必要的收口路径修复——不改断言语义强度（六案例自检锚定），编辑面超出 plan「waitPgroupESRCH 段」限定但属 Task 3 复验环节发现的新独立缺陷，如实记录。无范围蔓延。

## Issues Encountered

- CI 复验第一轮（33843785651）ubuntu leg flaky 致 macOS leg 连带取消（strategy fail-fast）——诊断确认与 G-11-2 修复无关后以 9936f2b 修正，第二轮（33844831146）双平台全绿收口

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **G-11-2 完全闭合**：missing 两条全落地 + 四子测锁定 + CI 双平台运行面确认——UAT 测试 2 result: pass、Gaps G-11-2 status: closed（终局对账由 /gsd-verify-work 裁定）
- **Phase 11 收口链就绪**：11-07 闭合 + SECURITY.md（threats_open: 0，secure-phase 产出 `5b6b178`）+ UAT 全测试 pass → verify-work 重跑后 Phase 11 可 ship
- Phase 12/13 既定分流不受影响（WR-01 → Phase 12、WR-02 → Phase 13，STATE.md 已登记）
- Phase 14 挂账保持：N 路并发退出 + 混合僵尸竞态复演（11-02 注释锚定）

## G-11-2 Missing 两条对账表

| Missing item | 落地 | 证据 |
|---|---|---|
| 1. waitPgroupESRCH 将 EPERM 视为存活探针的一种形态：不 return 不 Fatal，fall through 到护栏到期检查（护栏内 ESRCH 收敛 PASS；到期未 ESRCH 仍 Fatal，僵尸残留检测保留）；其余非 ESRCH 错误维持立即 Fatal | Task 1 ①②（`5aad25a`） | waitPgroupESRCHWithProbe 分类逻辑 + TestWaitPgroupESRCHProbeSemantics 四子测（本机 + CI macOS 双 4/4 PASS）；护栏翻车形态 CI 运行面未出现 |
| 2. 函数头注释修正：EPERM 在 macOS 退出过渡态可达（引 CI run 33832096581 实证），「ESRCH ⊇ 死亡且收割完成」论证补充 macOS 组信号语义差异 | Task 1 ④（`5aad25a`） | 头注释五要点（XNU 瞬态机理 + 对照证据 + POSIX 语义 + 语义差异补注 + 两半边策略）；旧论断 grep 计数 0；gofmt 零输出 |

运行面确认（G-11-2 truth 原文「Phase 11 测试套件在 CI macOS leg 全绿」）→ Task 3：CI run 33844831146 macOS leg 全绿（含 FAIL 现场测试转绿）。

## Self-Check: PASSED

- 文件存在性：internal/server/perclient_test.go（修改）/ 11-07-SUMMARY.md（本文件）FOUND
- 提交存在性：afb77a8（test）/ 5aad25a（fix）/ 9936f2b（flaky 修正）全部 FOUND
- 删除检查：perclient_test.go 的 -12 行全部为 waitPgroupESRCH 段重构的替换删除（7 行）+ 正则行替换（5 行），无测试函数删除
- 不跑 phase11.mjs 的省略理由（plan Task 2 action 既定）：零生产改动、二进制行为面不变，UAT 对象（产品行为）不触及

---
*Phase: 11-per-client*
*Completed: 2026-09-04*
