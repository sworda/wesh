---
phase: 10-mode-assembly
plan: 05
subsystem: testing
tags: [startup-preflight, cwd-semantics, validate-options, gap-closure, gofmt]

# Dependency graph
requires:
  - phase: 10-mode-assembly
    provides: session-mode 装配阀门（10-01..10-04）与 10-REVIEW-FIX 已提交修复（WR-01=189d081 / WR-02=0ec37cb / WR-03=23f2df2）
provides:
  - WR-01 闭合证据：SC4 预检 --cwd 感知对齐的六形态进程级冒烟矩阵 + TestStartupMatrix 三命名行全绿
  - WR-02 闭合证据：ValidateOptions 单调用点 + V(1328) < P(1334) < L(1342) 位序数值断言
  - fix 提交后 main HEAD（含 189d081/0ec37cb/23f2df2）零回归双证据首跑：-race 五包全 ok + 八 UAT 脚本原样全过
affects: [phase-11-lifecycle, phase-10-reverification]

# Actuals (#2632) — chars/4 over the realized diff（assert-first 闭合：代码面仅 a412a87 注释空格两行 1563 chars + 本 SUMMARY 14343 chars）
actuals:
  tokens: 3976
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Assert-first gap closure：修复已随评审修复轮落树时，闭合 plan 三任务均为只读断言链——全绿即零代码提交，仅证据落盘；断言失败才按 action 内目标态规格补缺"

key-files:
  created:
    - .planning/phases/10-mode-assembly/10-05-SUMMARY.md
  modified:
    - cmd/wesh/main.go（仅 :171 注释补一空格——GOROOT gofmt 闸补缺，WR 目标态零改动）
    - internal/pty/spawn.go（仅 :66 注释补一空格——同上）

key-decisions:
  - "GOROOT gofmt（go1.26.3 现代 doc-comment 规则）为本 plan 收口闸指定工具：其标记的 10-01 遗留两行 CJK 标点接续注释（//—— 与 //（）按 [Rule 3-Blocking] 补空格归一（a412a87），新旧两版 gofmt 双 clean；历史闸用 PATH gofmt（2021 旧版）故历代未报"

patterns-established:
  - "Assert-first 闭合形态：断言链全绿 → Task 1/2 零代码提交，SUMMARY 记录「无代码提交」即完成态"

requirements-completed: [PC-01]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "WR-01 闭合——SC4 预检 --cwd 感知对齐（放行面：per-client × --cwd × 相对路径 argv0 可执行 → listening on；拒绝面：缺失/无执行位 → exit 2 not executable；裸名缺失 → not found in PATH 零漂移；shared 对照零漂移）"
    requirement: PC-01
    verification:
      - kind: e2e
        ref: "进程级六形态冒烟矩阵（/tmp/wesh-v10-close 运行时材料，本 SUMMARY 对账表）"
        status: pass
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix/per-client_cwd-relative 三子测逐 PASS"
        status: pass
    human_judgment: false
  - id: D2
    description: "WR-02 闭合——ValidateOptions 前移至分岔块尾部、pty.Start 之前（守卫触发零资源占用），单调用点计数门 + V<P<L 位序断言锁定"
    requirement: PC-01
    verification:
      - kind: other
        ref: "grep -c 'verr := server.ValidateOptions(' == 1；V=1328 < P=1334 < L=1342；go test ./internal/server -run TestValidateOptions"
        status: pass
    human_judgment: false
  - id: D3
    description: "零回归收口闸：-race 五包全 ok + web/uat/phase02-09.mjs 八脚本原样 exit 全 0（12/18/10/28/23/34/21/18 对齐 10-04 基线）+ 五测试文件 append-only 零删除行 + banana 双源冒烟"
    requirement: PC-01
    verification:
      - kind: e2e
        ref: "go test -race -count=1 ./... + node web/uat/phase0{2..9}.mjs + git diff -U0 781af48..HEAD 过滤计数门"
        status: pass
    human_judgment: false

# Metrics
duration: 31min
completed: 2026-09-03
status: complete
---

# Phase 10 Plan 05: WR-01/WR-02 缺口闭合 Summary

**10-VERIFICATION 唯二缺口（WR-01 Blocker / WR-02 Warning）断言优先闭合——六形态进程级冒烟矩阵实证 truth #5 由 FAILED 转 PASS（per-client × --cwd × ./run.sh 由 exit 2 误拒转 listening on），ValidateOptions 位序 V<P<L 数值锁定，fix 提交后 main HEAD 零回归双证据（-race 五包 + 八 UAT 原样）首跑全绿。**

## Performance

- **Duration:** 31 min
- **Started:** 2026-09-03T03:03:35Z
- **Completed:** 2026-09-03T03:34:35Z
- **Tasks:** 3（全断言优先路径；补缺分支未触发于 WR 目标态）
- **Files modified:** 2 代码文件（各 1 行注释空格，gofmt 闸补缺）+ 1 SUMMARY

## Accomplishments

- **WR-01 闭合**：SC4 预检 --cwd 感知对齐（189d081 目标态逐项核实）——六形态进程级冒烟全中，TestStartupMatrix 三 WR-01 子测逐 PASS，VERIFICATION truth #5 由 FAILED 转可复验 PASS
- **WR-02 闭合**：ValidateOptions 前移位序（0ec37cb 目标态逐项核实）——单调用点计数门 == 1，V(1328) < P(1334) < L(1342) 位序断言成立，守卫触发零资源占用纪律恢复
- **零回归收口闸首跑**：fix 提交后 main HEAD 上 -race 五包全 ok + 八 UAT 脚本原样 exit 全 0 且 PASS 计数逐脚本对齐 10-04 基线 + 五测试文件 append-only 零删除行 + banana 双源冒烟逐字命中

## 缺口闭合对账表（missing 逐项 → 证据链）

### WR-01（Blocker）——SC4 预检不感知 --cwd，相对路径 argv0 误拒

| VERIFICATION missing 项 | 目标态位置 | 闭合证据 |
|---|---|---|
| (a) 预检与 spawn 语义对齐（--cwd join + os.Stat 可执行探测） | cmd/wesh/main.go:1051-1063（仅 per-client × argv0 非空触发；`filepath.Join(cfg.cwd, probe)` :1054 与 spawn.go:83 `cmd.Dir = opts.Dir` 解析基准对齐；裸名保持 LookPath 通道） | 冒烟 A：per-client × --cwd=appdir × `./run.sh`（0755 存在）→ `session_start(pid)` + `listening on`，rc=124（修复前 exit 2 误拒） |
| (b) 补 --cwd × 相对路径 argv0 的 TestStartupMatrix 放行面测试行 | cmd/wesh/main_test.go:832-834（三命名行）+ :699-708 cwdCmdDir fixture（run.sh 0755 / noexec.sh 0644，t.TempDir 运行时材料） | `--- PASS` 计数 == 3：`per-client_cwd-relative_executable_allowed_(WR-01)` / `_missing_refused_(WR-01)` / `_non-executable_refused_(WR-01)`；既有行逐字未动（append-only 闸 0 删除行） |
| (c) 带斜杠形态文案修正（not found in PATH → not executable） | main.go:1058 `invalid command %q: not executable (per-client startup preflight)`；裸名 :1061 `not found in PATH` 逐字保持 | 冒烟 B（./no-such.sh）/ C（./noexec.sh 0644）/ E（无 --cwd 且服务端 cwd 无该文件）各 rc=2 + `not executable (per-client startup preflight)`；冒烟 F（裸名缺失）rc=2 + `not found in PATH (per-client startup preflight)`（SC4 既有面零漂移）；冒烟 D2 shared 对照 → `listening on` rc=124（零漂移） |

### WR-02（Warning）——ValidateOptions 调用点在资源获取之后、失败零回滚

| VERIFICATION missing 项 | 目标态位置 | 闭合证据 |
|---|---|---|
| 前移至 10-01 分岔块尾部、pty.Start 之前（推荐前移方案） | main.go:1328 单调用点（`verr := server.ValidateOptions(server.Options{SessionMode: cfg.sessionMode, SpawnFunc: spawnFunc})` 最小字面量——server.go:336-348 核实只读该两字段）；旧调用点不存在；opts 注释 :1366-1369 登记前移事实 | 计数门：`grep -c 'verr := server.ValidateOptions('` == 1；位序数值断言 V(1328) < P(1334, pty.Start) < L(1342, `var ln net.Listener`)——守卫触发时 spawn/listen 均未发生，无 sess/ln 可回滚；TestValidateOptions 三态契约 PASS；cmd/wesh 包全量绿 |

## 收口闸数字（Task 3 全量，main HEAD 含 189d081/0ec37cb/23f2df2 + a412a87）

| 闸 | 结果 | 数字 |
|---|---|---|
| `time go build ./...` | PASS | 0.579s |
| `go vet ./...` | PASS | 零输出 |
| GOROOT gofmt -l cmd internal web | PASS | 零输出（a412a87 后；go1.26.3） |
| `go test -race -count=1 ./...` | PASS | 五包全 ok：cmd/wesh 1.328s / internal/proto 1.015s / internal/pty 2.653s / internal/server 59.103s / web 1.011s |
| UAT phase02-09 八脚本 | PASS | exit 全 0；PASS 计数 12/12、18/18、10/10、28/28、23/23、34/34、21/21、18/18——与 10-04 基线逐脚本一致，FAIL=0 |
| web/uat/ 零修改（D-06） | PASS | `git status --short -- web/uat/` 零输出 |
| append-only 闸 | PASS | `git diff -U0 781af48..HEAD` 五测试文件删除行计数 == 0 |
| 冒烟 CLI banana | PASS | rc=2 + `invalid --session-mode "banana": must be shared or per-client` 逐字 |
| 冒烟 TOML banana | PASS | rc=2 同文案（一闸双覆盖） |
| 冒烟 per-client 接受面 | PASS | `session_start(pid)` + `listening on`，rc=124 |

进程级六形态冒烟矩阵（Task 1，/tmp/wesh-v10-close 运行时材料，日志未入库——D-06 纪律）：

| 腿 | 形态 | 结果 |
|---|---|---|
| A | per-client × --cwd=appdir × ./run.sh（可执行存在） | `session_start(pid=1697453)` + `listening on http://127.0.0.1:37445`，rc=124 ✅ 放行（VERIFICATION 复现场景由误拒转放行） |
| B | per-client × --cwd=appdir × ./no-such.sh（缺失） | rc=2 + `invalid command "./no-such.sh": not executable (per-client startup preflight)` |
| C | per-client × --cwd=appdir × ./noexec.sh（0644 无执行位） | rc=2 + `invalid command "./noexec.sh": not executable (per-client startup preflight)` |
| E | per-client 无 --cwd × ./run.sh（服务端 cwd 无此文件） | rc=2 + `invalid command "./run.sh": not executable (per-client startup preflight)` |
| F | per-client × 裸名 wesh-no-such-cmd-7f3a | rc=2 + `invalid command "wesh-no-such-cmd-7f3a": not found in PATH (per-client startup preflight)`（SC4 既有面零漂移） |
| D2 | shared × --cwd=appdir × ./run.sh（对照面） | `session_start(pid=1697870)` + `listening on http://127.0.0.1:44347`，rc=124（shared 零漂移——同参数不预检、照常 spawn） |

## Task Commits

断言优先路径全绿，Task 1/2 的 WR 目标态零代码改动——按计划承诺**无任务代码提交**：

1. **Task 1: WR-01 闭合冒烟矩阵（tracer）** — 无代码提交（目标态 189d081 已在树，断言链 WR01_CLOSURE_PASS）；tracer 反馈闸：verify 端到端重跑通过，⚡ 放行扩展
2. **Task 2: WR-02 前移位序断言** — 无代码提交（目标态 0ec37cb 已在树，断言链 WR02_CLOSURE_PASS）
3. **Task 3: 零回归收口闸 + 证据归集** — 无代码提交（CLOSURE_GATE_CORE_PASS + 本 SUMMARY 落盘）

**Deviation style commit:** `a412a87`（style: GOROOT gofmt 注释空格归一，见下节）
**Plan metadata:** 伴随 docs 提交（SUMMARY + STATE/ROADMAP 跟踪文件，hash 见 git log 最新 docs(10-05)）

## Files Created/Modified

- `.planning/phases/10-mode-assembly/10-05-SUMMARY.md` — 本闭合证据文件（对账表 + 收口闸数字）
- `cmd/wesh/main.go` — 仅 :171 注释补一空格（gofmt 闸补缺，WR 目标态零改动）
- `internal/pty/spawn.go` — 仅 :66 注释补一空格（同上）

## Decisions Made

- **GOROOT gofmt 为闸工具、两行 10-01 遗留注释补空格归一**（a412a87）：本 plan 收口闸首次指定 `"$(go env GOROOT)"/bin/gofmt`（go1.26.3，现代 doc-comment 规则），其标记 main.go:171 `//——` 与 spawn.go:66 `//（` 两行——历代闸（10-01..10-04、VERIFICATION truth #18）用 PATH gofmt（/usr/bin/gofmt，2021 旧版）故未报。修复后新旧两版 gofmt 均 clean，CI 无 gofmt 闸（ci.yml/release.yml 无此检查），零语义改动、零行为面、append-only 纪律不涉（非测试文件）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] GOROOT gofmt 闸对 10-01 遗留注释报格式缺欠，计划强制闸不可通过**
- **Found during:** Task 2（verify 链 `test -z "$(GOROOT gofmt -l cmd internal web)"` 失败）
- **Issue:** plan 断言优先前提「当前 HEAD 全断言通过」在 gofmt 项不成立——GOROOT gofmt（go1.26.3）标记 cmd/wesh/main.go:171 与 internal/pty/spawn.go:66（均 111a3e7/10-01 引入，非本 plan 改动面，非 189d081/0ec37cb 修复引入）；历史闸用 PATH 旧版 gofmt 故从未触发
- **Fix:** 两行注释各补一空格（`//——` → `// ——`、`//（` → `// （`），现代 gofmt doc-comment 归一形态；新旧两版 gofmt 双 clean 复核
- **Files modified:** cmd/wesh/main.go、internal/pty/spawn.go（各 1 行注释；spawn.go 超出 plan files_modified 声明范围，在此登记）
- **Verification:** GOROOT gofmt -l 零输出 + PATH gofmt -l 零输出 + Task 2/3 全断言链重跑通过
- **Committed in:** `a412a87`（独立 style 提交）

---

**Total deviations:** 1 auto-fixed（Rule 3 - Blocking）
**Impact on plan:** 注释白空格级修复，零语义/零行为面；不触碰任何测试行（append-only 闸复核 0 删除行）、不触碰 shared 行为面、零新增依赖。无 scope creep。

## Issues Encountered

- **gofmt 双版本岔口**（已解决，见 deviation #1）：PATH gofmt（/usr/bin/gofmt → alternatives，2021-02）与 GOROOT gofmt（go1.26.3，2026-05 安装）对 doc-comment 规则判定不同；项目历史闸均走 PATH 版。后续 phase 若沿用 GOROOT gofmt 闸口径，本 plan 归一后的树状态可持续通过。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Phase 10 可进入复验**：WR-01/WR-02 两缺口 missing 清单逐项有进程级/测试级证据映射（上对账表），10-VERIFICATION 18/19 → 19/19 对账关闭材料齐备；truth #5（per-client × 可执行命令 → 放行）已可复验通过
- **PC-01 派生行为缺口闭合**：requirement 主文面 + SC4 预检派生面均绿（REQUIREMENTS.md 复验裁决归 phase 复验所有，本 plan 不自动 resolve EDGE_ABSENT 边界行）
- **显式假设挂账（非静默丢项）**：PATH 含相对项（如 `.`）× 裸命令名的残余错位面不在本批闭合范围（裸名形态下 Go LookPath 对相对解析返回 ErrDot 走拒绝通道——安全方向无误放），Phase 11 attach 期 spawn 语义收口时复核
- **Phase 11 前提就位**：spawn 移至 attach 期后 SC4 预检为唯一启动闸——其语义（--cwd 感知对齐 + 分形态文案）已在本 plan 固化并经行为锁（三命名行）+ 进程级六形态双向实证

## Self-Check: PASSED

- FOUND: `.planning/phases/10-mode-assembly/10-05-SUMMARY.md`（本文件，含 WR-01/WR-02 对账节）
- FOUND: `a412a87`（style deviation 提交）、`189d081`（WR-01 修复）、`0ec37cb`（WR-02 修复）——全部在 git log
- 断言产物复核：WR01_CLOSURE_PASS / WR02_CLOSURE_PASS / CLOSURE_GATE_CORE_PASS 三链退出码均 0

---
*Phase: 10-mode-assembly*
*Completed: 2026-09-03*
