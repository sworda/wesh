---
phase: 11-per-client
plan: "04"
subsystem: server
tags: [per-client, lifecycle, exit-frame, sighup, esrch, teardown, race-injection, kill-fallback, go, testing]

requires:
  - phase: 11-per-client
    plan: "01"
    provides: "per-client 生命周期主干（sessionWatcher EXIT 私有化直写 / teardownPCLocked D-01 固定序列 + reaped 栅栏 / 唯一收割者纪律 / perclient_test.go harness 与 readOutputUntil）"
  - phase: 11-per-client
    plan: "03"
    provides: "PCSessionsLenForTest 测试出口 + startPerClientServerWithSpawn harness（spawnFn 注入 + spawnedSessions）+ linger 形态与 trap 免疫夹具先例"
provides:
  - "EXIT 私有化两形态行为锁（PC-04）：exit 42 → 本端末帧 EXIT exit_code==42 + 1000；信号死亡（kill -HUP $$）→ EXIT exit_code==-1 + SIGHUP 文案 + 1000；他端 1.5s 静默窗零帧（任何类型字节到达即 FAIL）+ 窗后 echo 存活"
  - "断开 SIGHUP 无宽限无僵尸行为锁（PC-03）：Close(1000) → 2s 护栏内 pgid ESRCH（含收割完成）+ pcSessions 收敛 0 + /healthz clients==0"
  - "断线重连 = 全新进程锁：pid2 != pid1（ttyd parity；前端 reset 归 Phase 12）"
  - "D-01 KILL 兜底时序双断言（Pitfall 8 的 Go 侧证据）：trap 免疫 + StopTimeout=1s → 300ms 时点存活 + 到期后 ~1.006s ESRCH（5s 护栏内）+ pcSessions 收敛 0"
  - "teardown 恰好一次竞态注入（Pitfall 3 的 Phase 11 侧锁）：10 轮 exit 0×Close 并发 → 终态四件套（pcSessions 空 / healthz 0 / pgid ESRCH / exitf 零调用）+ wire 面 EXIT ≤1 且到达即 exit_code==0 + 零 panic"
  - "新 helper：frameRes/readPump/drainQuiet/accumFramesUntil/readSessionPid/waitPgroupESRCH（append-only）"
affects: [11-05, 11-06, 13-termination, 14-matrix]

actuals:
  tokens: 5900
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "严格静默窗强形态：并发泵（readPump）+ 前置收干（drainQuiet 首帧门 + quiet 窗口）——「他端零帧」断言须先排空本端自身提示符，否则假阳性"
    - "pgid ESRCH 轮询探针（waitPgroupESRCH）：kill(-pid,0) 25ms 步进 + 总护栏；「kill 0 无错 = 存活探针」红线注释化（ESRCH ⊇ 死亡且收割完成——无僵尸强证据）"
    - "时序双断言的 per-client 同构：stopseq_test.go「到期前静默窗 / 到期后护栏内收码」形态镜像，断言对象 exitf(-1) → pgid ESRCH + pcSessions 收敛"
    - "竞态注入锁不变量而非时序胜者：终态四件套 + exitf 零调用 + wire EXIT ≤1；「竞态固有二形态 ≠ 断言放宽」注释论证（Pitfall 11 误读防护）"
    - "客户端主动 Close 后的关闭证据双通道：泵侧 CloseError{1000} 直读 / Close nil 返回（库 closeHandshake 内 CloseStatus 校验）——库 prepareRead 先查 c.closed 使 Close 后 Read 恒 net.ErrClosed 的语义注释化"

key-files:
  created: []
  modified:
    - internal/server/perclient_test.go

key-decisions:
  - "plan 文本 kill -TERM $$ 勘误为 kill -HUP $$：交互式 shell 无 trap 时忽略 SIGTERM（bash/dash 手册语义），TERM 自杀不致死（实测 10s 统护到期）；HUP 对交互 shell 致死且与 exit_test.go 信号夹具同款，断言面不变（Rule 3 偏差，测试注释锚定）"
  - "竞态注入测「读连接至 CloseError」经并发泵实现：客户端主动 Close 完成后 Read 恒返 net.ErrClosed（库 prepareRead 语义），1000 关闭证据由泵（CloseError）与 Close 返回值（nil = 库内校验已过）双通道承载；两通道均未取得即 FAIL 的结构性守卫"
  - "wire 面 EXIT 到达即 exit_code==0 的确定性断言（非放宽）：EXIT 上 wire 蕴含 watcher 在 conn 关闭前完成终结写 → 子死路径先收口 → 死因 = exit 0（reaped 栅栏内 SIGHUP 不发）——论证链注释锚定"
  - "PC-03/PC-04 需求勾选延续 11-01/11-02/11-03 既定：ID 跨 6 plan 共享，phase 末 plan 11-06 统一勾选（requirements-completed 留空防可追溯表污染）"
  - "TDD 形态延续 11-01/11-03 先例：plan 显式单 test 提交指令优先；六测为 11-01 已落地机制的行为锁，假绿防线由自证形态承担（SIGHUP 缺席则 ESRCH 轮询超时翻车、KILL 缺席则 trap 免疫进程存活到护栏翻车、EXIT 串台则静默窗即时 FAIL）"

requirements-completed: []

coverage:
  - id: D1
    description: "EXIT 私有化 exit 42 形态：A 端末帧 EXIT exit_code==42 + close 1000；B 端 1.5s 静默窗零帧（任何类型字节到达即 FAIL——T-11-04a 他端零扰动强形态）；窗后 B echo 唯一标记回读（服务端续跑 + B 会话无扰动）"
    requirement: PC-04
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientExitPrivate42（-race -count=3 全 PASS）"
        status: pass
    human_judgment: false
  - id: D2
    description: "EXIT 私有化信号死亡形态：kill -HUP $$ → A 端末帧 EXIT exit_code==-1 + message 含大写 SIGHUP + close 1000（信号死亡 -1 语义经 watcher 内联退出码提取与 exitMessage/exitSignalNum 复用面）"
    requirement: PC-04
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientExitSignalMinus1（-race -count=3 全 PASS）"
        status: pass
    human_judgment: false
  - id: D3
    description: "断开 SIGHUP 无宽限无僵尸（ROADMAP SC3）：Close(1000) 后 2s 护栏内 pgid ESRCH（setsid pgid==pid；ESRCH ⊇ 收割完成——无僵尸）+ pcSessions 收敛 0 + /healthz clients==0"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientDisconnectSIGHUP（-race -count=3 全 PASS）"
        status: pass
    human_judgment: false
  - id: D4
    description: "断线重连 = 全新进程（ttyd parity）：pid1 ESRCH 后同 wsURL 再 attach 回读 pid2 != pid1（前端 terminal.reset() 归 Phase 12 不在此断言）"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientReconnectNewPid（-race -count=3 全 PASS）"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-01 KILL 兜底时序双断言（Pitfall 8）：trap 免疫 + StopTimeout=1s → 断开后 300ms 时点存活（HUP 免疫实证）→ 到期后 5s 护栏内 ESRCH（实测三次 1.0055-1.0059s）→ pcSessions 收敛 0"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientStopTimeoutKillFallback（-race -count=3 全 PASS，t.Logf 时点记录）"
        status: pass
    human_judgment: false
  - id: D6
    description: "teardown 恰好一次竞态注入（Pitfall 3）：10 轮 exit 0×Close 并发 → 每轮终态 quiescent（pcSessions 空 / healthz 0 / pgid ESRCH）+ exitf 桩全程零调用（D-10/D-13 硬约束 + 11-01 中间态①②反向锁）+ wire EXIT ≤1 且到达即 exit_code==0 + 零 panic"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientTeardownRaceOnce（-race -count=3 全 PASS）"
        status: pass
    human_judgment: false
  - id: D7
    description: "零回归收口闸：shared 全量 Go 测试原样绿 + 期望值逐字未动；plan 全范围 diff 仅 perclient_test.go（+467/-0 净增，append-only——5 行删除为同提交内新增注释的 gofmt 规整）"
    verification:
      - kind: other
        ref: "go test -race -count=1 ./...（5 包全绿 ×2 轮）；git diff 689cfa3..HEAD --stat 仅 perclient_test.go；GOROOT gofmt -l 零输出"
        status: pass
    human_judgment: false
  - id: D8
    description: "phase11.mjs S4/S5/S8 协议层对照（断开 SIGHUP / EXIT 私有化 / KILL 兜底的 UAT 面双锁）"
    requirement: PC-03
    verification: []
    human_judgment: true
    rationale: "Go 测试侧已全部锁定；协议层 UAT 对照按 plan 切片归 11-05（D-06 八场景）——本 plan 不宣称闭合；1006 真实异常断开形态按 CODEBUDDY.md 分层测试策略 §5 平台豁免（协议层覆盖 = 正常关闭 + reader 错误路径，11-04 flagged_assumptions 锚定）"

duration: 30min
completed: 2026-09-04
status: complete
---

# Phase 11 Plan 04: per-client 生命周期行为锁定 Summary

**per-client 生命周期全部可观测面六测锁定：EXIT 私有化两形态（exit 42 / 信号死亡 -1，本端末帧断言 + 他端 1.5s 零帧强形态）、断开 SIGHUP 无宽限无僵尸（pgid ESRCH + pcSessions 收敛）、重连新 pid、D-01 KILL 兜底时序双断言（trap 免疫下实测断开→ESRCH 1.0055-1.0059s，stop-timeout=1s 精度）、teardown 恰好一次 10 轮竞态注入（quiescent 四件套 + exitf 零调用 + 零 panic）；Pitfall 2/3/8 的 Phase 11 侧防线全部收口，shared 全量 -race 原样绿、既有测试逐字未动。**

## Performance

- **Duration:** 30 min
- **Started:** 2026-09-03T17:56:20Z
- **Completed:** 2026-09-03T18:26:14Z
- **Tasks:** 2
- **Files modified:** 1（internal/server/perclient_test.go，净 +467 行 append-only）

## Accomplishments

- 六行为测全绿且 -race -count=3 连跑无 flake：四测（Task 1）+ 两测（Task 2）合并复跑 18/18 PASS；全量 `go test -race -count=1 ./...` 两轮全绿（5 包）
- 行为断言实测输出逐条（验收复核要求）：
  - **B 端静默窗零帧**：TestPerClientExitPrivate42 ×3 PASS——A 终结后 1.5s 窗口内泵源零交付（OUTPUT/EXIT/任何类型字节均未到达），窗后 B echo BSURVIVE_4k8 回读成功（服务端续跑 + B 会话无扰动）
  - **close 1000 两形态**：exit 42 与信号死亡两测的 CloseError.Code==1000 断言 ×3 PASS
  - **pid 不等**：TestPerClientReconnectNewPid ×3 PASS（pid1 ESRCH 后重连 pid2 != pid1）
  - **ESRCH 到达**：断开 SIGHUP 测 ×3 PASS（2s 护栏内 kill(-pid,0) 至 ESRCH，pcSessions 收敛 0，healthz clients==0）
  - **时序双断言实测时点**：断开后 300ms 时点 pgid 存活（HUP 免疫实证）+ ESRCH 历时 1.005763s / 1.005736s / 1.005637s（stop-timeout=1s 到期后 KILL 兜底，远 < 5s 护栏）——三次连跑 t.Logf 记录
- 竞态注入测每轮四件套终态 + exitf 零调用 + wire 面 EXIT ≤1（到达即 exit_code==0，论证链注释锚定）10 轮 ×3 连跑稳定
- 零回归：plan 全范围 diff（689cfa3..HEAD）仅 perclient_test.go 且净零删除；既有八测（11-01 五测 + 11-03 三测）逐字未动

## Task Commits

Each task was committed atomically:

1. **Task 1: EXIT 私有化两形态 + 断开 SIGHUP + 重连新 pid 四测（PC-03/PC-04）** - `8edab65` (test)
2. **Task 2: D-01 KILL 兜底时序双断言 + teardown 恰好一次竞态注入（Pitfall 3/8，PC-03）** - `1ac7d24` (test)

**Plan metadata:** 见文末最终 docs commit（SUMMARY/STATE/ROADMAP）

## Files Created/Modified

- `internal/server/perclient_test.go` — 11-04 增量段（六测 + 六 helper：frameRes/readPump/drainQuiet/accumFramesUntil/readSessionPid/waitPgroupESRCH）；append-only 纪律，文件头与 11-01/11-03 段逐字未动

## Decisions Made

- **kill -TERM → kill -HUP 勘误**（Rule 3，见 Deviations #1）：交互式 shell 无 trap 忽略 SIGTERM——plan 文本的 TERM 自杀夹具实测不致死；HUP 致死且与 exit_test.go 信号夹具同款，断言面（-1 + 大写信号名 + 1000）不变
- **竞态注入测的关闭观测形态**：客户端主动 Close 后 Read 恒返 net.ErrClosed（coder/websocket prepareRead 先查 c.closed，库源码实证），故「读至 CloseError」经并发泵实现；1000 证据双通道（泵 CloseError / Close nil 返回 = 库内 CloseStatus 校验已过），两通道皆空即 FAIL 的结构性守卫
- **wire 面 EXIT 到达 ⇒ exit_code==0 确定性断言**：EXIT 上 wire 蕴含 watcher 在 conn 关闭前完成终结写 ⇒ 子死路径先收口（断开先收口则库自动回显已 full-close conn，watcher 的 EXIT 写在 +200ms Drain 后必然失败静默）⇒ 死因 = exit 0（reaped 栅栏内 SIGHUP 不发）——注释锚定论证链，非断言放宽
- **PC-03/PC-04 勾选归 11-06**：跨 6 plan 共享 ID，phase 末 plan 统一勾选（11-01/11-02/11-03 既定延续，防可追溯表污染）
- **TDD 形态**：两任务 plan 显式单 test 提交指令优先于通用 RED/GREEN 双提交流（11-01/11-03 先例延续）；六测为已落地机制的行为锁——假绿防线由断言的自证形态承担（SIGHUP 缺席 → ESRCH 轮询超时翻车；KILL 缺席 → trap 免疫进程存活到 5s 护栏翻车；EXIT 串台 → 静默窗即时 FAIL；exitf 失守 → len(exitCh)!=0 FAIL）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] plan 文本自杀信号选型勘误（kill -TERM $$ → kill -HUP $$）**
- **Found during:** Task 1 ②（TestPerClientExitSignalMinus1 首跑）
- **Issue:** plan behavior/action 均写 `kill -TERM $$`（sh 自杀）——但**交互式 shell 无 trap 时忽略 SIGTERM**（bash 手册 Signals 节「When bash is interactive, in the absence of any traps, it ignores SIGTERM」，dash 同语义）；实测首跑 10s 统护到期（收 3-4 帧提示符/回显后无 EXIT），shell 存活
- **Fix:** 自杀信号改 SIGHUP（交互 shell 对 HUP 致死；exit_test.go TestExitFrameSignal 既有信号夹具同款），message 断言同步为大写 SIGHUP；测试 doc 注释锚定勘误理由；断言面（exit_code==-1 + 大写信号名文案 + close 1000）与 plan 逐字等价
- **Files modified:** internal/server/perclient_test.go
- **Verification:** 修正后 -race -count=3 全 PASS（0.00-0.01s/轮——致死即时，非护栏触发）；WINDOWS.md 账本登记（kind=deviation，与 11-01 偏差条目同款）
- **Committed in:** 8edab65

**2. [Rule 3 - Blocking] GOROOT gofmt 规整 5 行新增注释的 CJK 接续行**
- **Found during:** Task 2 ③（收口自检 gofmt -l）
- **Issue:** go1.26.3 现代 doc-comment 规则（10-05 收口闸工具纪律）要求 `//` 后 CJK 标点接续行补空格——11-04 新增注释 5 行命中
- **Fix:** GOROOT gofmt -w 机械规整；diff 复核确认 5 行删除行全部为同提交内的新增注释行（append-only 纪律不触及 11-01/11-03 既有段）
- **Files modified:** internal/server/perclient_test.go
- **Verification:** GOROOT gofmt -l 全仓零输出；六测 -race -count=3 全绿
- **Committed in:** 1ac7d24

---

**Total deviations:** 2 auto-fixed（均为 Rule 3 阻塞类：plan 文本夹具选型实测勘误 + 收口闸工具纪律调和；后者纯注释排版零行为面）
**Impact on plan:** 无范围蔓延；TERM→HUP 勘误保持 plan 的语义断言面逐字等价（-1 语义 + 信号名文案 + 1000），建议 planner 侧知悉（交互 shell 信号面：TERM 忽略 / HUP 致死——后续 plan 的信号夹具选型应直接采用 HUP 或 trap 形态）。

## Issues Encountered

None（除 Deviation #1 的夹具勘误外一次通过）——竞态注入测的并发泵 + 双通道关闭证据形态在设计期经 coder/websocket v1.8.15 库源码实证（close.go:86-99「Close 期间数据帧丢弃」、read.go:221-240 prepareRead 先查 c.closed、read.go:340-367 handleControl 自动回显关闭帧），一次写对无返工。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 11-05（phase11.mjs 八场景 UAT）：Go 侧行为基准全部就位——S4 断开 SIGHUP（pgid ESRCH 轮询形态 Go 母本 waitPgroupESRCH → Node process.kill(-pgid,0) catch ESRCH 同构）、S5 EXIT 私有化（他端静默窗零帧强形态 + exit 42 / 信号 -1 两形态基线）、S7 重连新 pid、S8 KILL 兜底（时序双断言：300ms 存活 + 到期后护栏内 ESRCH 的脚本镜像）
- 11-06（phase 收口闸）：PC-03/PC-04 勾选承载点（本 plan coverage D1-D7 全 pass、D8 留 UAT 对照）；flagged_assumptions 三项保持 flagged-unverified 归 11-06 显式证据
- Phase 13 提醒：竞态测 exitf 零调用断言是中间态①②的反向锁——pcSupervisor/第二终结源落地时按裁决翻转该断言（测试注释已锚定）；stop-timeout per-client 默认值重议（裁决项①）机制已锁（D-01），仅改默认值
- 威胁登记闭合：T-11-04a（EXIT 串台信息泄露）→ mitigate（他端 1.5s 零帧强形态实测）；T-11-04b（teardown 双路并发完整性）→ mitigate（10 轮竞态注入 quiescent 四件套 + exitf 零调用 + 零 panic + 全量 -race）；T-11-04c（HUP 免疫泄漏 DoS）→ mitigate（KILL 兜底时序双断言实测）；T-11-SC 零新依赖保持（go.mod/go.sum 零漂移）

## Self-Check: PASSED

- 文件存在性：internal/server/perclient_test.go / .planning/phases/11-per-client/11-04-SUMMARY.md 全部 FOUND
- 提交存在性：8edab65（Task 1 test）/ 1ac7d24（Task 2 test）全部 FOUND
- 删除检查：两提交 `git diff --diff-filter=D` 均无文件删除；plan 全范围 diff（689cfa3..HEAD）仅 perclient_test.go 净 +467/-0
- 验收 grep：Task 1 四项（测名 ==4 / exit_code ==4≥2 / ESRCH ==1 / Kill(-pid,0) ==2）+ Task 2 四项（测名 ==2 / StopTimeout ==3 / exitCh ==10 / 竞态注释论证块在案）全部达标
- 行为复核：B 端静默窗零帧 / close 1000 两形态 / pid 不等 / ESRCH 到达 / 300ms 存活 + ESRCH 1.0055-1.0059s（<5s 护栏）——实测输出逐条见 Accomplishments

---
*Phase: 11-per-client*
*Completed: 2026-09-04*