---
phase: 11-per-client
plan: "01"
subsystem: server
tags: [per-client, pty, websocket, lifecycle, teardown, sighup, exit-frame, go]

requires:
  - phase: 10-mode-assembly
    provides: "SessionMode/SpawnFunc 接缝 + ValidateOptions 互斥校验 + pty.StartWithSize + --session-mode flag（全部 inert 装配，本 plan 首次消费）"
provides:
  - "pcSession 类型与 pcSessions 会话活性注册表（internal/server/perclient.go / server.go）"
  - "upgradePerClient 升档分支：ticket 核销后 / hubMu 外 spawn（SEC-08 + Anti-Pattern 1/3），失败 Error{server_error,'failed to start process'}+1011+spawn_failed 单行审计"
  - "startSessionGoroutines 五件装配（writer/pinger/ReadLoop 闭包/inputWriter/sessionWatcher），Welcome 恒首帧（本端 Hello 钳制尺寸回显）"
  - "sessionWatcher：每会话唯一收割者 + EXIT 私有化直写（组帧一次→同步 Write 2s ctx→Close(1000)）"
  - "teardownPCLocked：D-01 固定序列恰好一次（SIGHUP 经 reaped 栅栏→AfterFunc 补 KILL→Drain(200ms)→Close→waitDone→pcSessions 单点移除）"
  - "New 入口 sess×mode 契约 panic + New 尾部模式分岔 + INPUT case cl.inQ 一行切换 + inputWriter 包级参数化 + detach/kick teardown 挂点 + maybeExitWhenEmptyLocked/Shutdown 两处守卫"
  - "main.go run() per-client sess=nil（启动期零子进程）+ startOpts 收编 + 回滚 nil 守护"
  - "perclient_test.go harness（startPerClientServer + readOutputUntil）与五测（D-05 独立文件）"
affects: [11-02, 11-03, 11-04, 11-05, 11-06, 12-interaction, 13-termination, 14-matrix]

actuals:
  tokens: 15930
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Pattern 2 间接字段（cl.inQ）：升档解析一次，读循环逐行不分支"
    - "Pattern 3 参数化泵函数（inputWriter(sess,q,done)）：1 份代码 N 实例"
    - "reaped 栅栏（hubMu 内标志位）：信号与 reap 锁内序列化，kill-after-reap 结构性不可能"
    - "teardown sync.Once 双路触发恰好一次 + 快半段（hubMu 内信号）/慢半段（goroutine 阻塞调用）分裂"

key-files:
  created:
    - internal/server/perclient.go
    - internal/server/perclient_test.go
  modified:
    - internal/server/server.go
    - internal/server/clients.go
    - cmd/wesh/main.go

key-decisions:
  - "perclient_test.go 落 package server_test：plan 文本「package server」与「复用 e2e_test.go 同包既有形态」自相矛盾，按后者裁决（dialHello/captureStderr/eventsNamed 复用是硬约束，同包=server_test）"
  - "Task 1 以单 feat 提交收口（plan 显式提交指令优先于通用 TDD RED/GREEN 双提交流；RED=编译期/运行期 panic 观察在任务内完成，避免不可编译中间提交）"
  - "PC-02/03/04 需求勾选留给 phase 末 plan 11-06：三 ID 跨 6 plan 共享，11-01 仅落地机制本体+冒烟（行为全断言归 11-03/04/05，success_criteria 明示），提前勾选会污染可追溯表"
  - "reaped 栅栏覆盖 AfterFunc 补 KILL 发信号点（planner 裁定执行落地）：快半段 !reaped 才发 SIGHUP+武装 timer，闭包同锁复检 !reaped 才补 SIGKILL"

requirements-completed: []

coverage:
  - id: D1
    description: "per-client attach 升档即独立 PTY spawn（Hello 钳制尺寸直通 StartWithSize），两客户端各得不同 pid 且输出互不串台"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientTwoClientsTwoPids"
        status: pass
    human_judgment: false
  - id: D2
    description: "Welcome 恒 S→C 首帧且 cols/rows 回显本端 Hello 钳制尺寸（无 80x24 中间态）"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientWelcomeDims"
        status: pass
    human_judgment: false
  - id: D3
    description: "输入链全链：INPUT→cl.inQ→每会话 inputWriter→master→回显"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientInputEcho"
        status: pass
    human_judgment: false
  - id: D4
    description: "sess×mode 装配契约 New 入口程序错误 panic（两形态文案锁定 + exitf 零调用）"
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestNewModeSessContract"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-04 窗口期空白：零 session_start 事件 + /healthz session_active==false 中间态显式锁"
    verification:
      - kind: integration
        ref: "internal/server/perclient_test.go#TestPerClientNoSessionStartEvent"
        status: pass
    human_judgment: false
  - id: D6
    description: "shared 模式逐字节零回归：全量 -race 绿 + phase02-09 八主脚本默认模式零修改重跑 exit 全 0 + 既有测试期望值逐字未动"
    verification:
      - kind: e2e
        ref: "go test -race -count=1 ./...（全绿）；node web/uat/phase02-09.mjs /tmp/wesh-p11/wesh（12/18/10/28/23/34/21/18 对齐基线）"
        status: pass
    human_judgment: false
  - id: D7
    description: "断开全形态 SIGHUP teardown 行为断言（pgid ESRCH 无僵尸 / trap '' HUP + stop-timeout KILL 兜底）"
    requirement: PC-03
    verification: []
    human_judgment: true
    rationale: "机制本体已就位（teardownPCLocked 固定序列 + detach/kick 挂点），行为断言按 plan 切片归 11-04 竞态注入测试与 phase11.mjs S4/S8——本 plan 不宣称闭合"
  - id: D8
    description: "EXIT 私有化行为断言（exit 42/信号死亡仅本端收 EXIT+1000，他端零感知）"
    requirement: PC-04
    verification: []
    human_judgment: true
    rationale: "sessionWatcher 直写序列已就位，行为断言按 plan 切片归 11-04/11-05 与 phase11.mjs S5——本 plan 不宣称闭合"
  - id: D9
    description: "spawn 失败 wire 面行为断言（Error 通用文案 + 1011 + spawn_failed 事件 + 他端零感知）"
    requirement: PC-02
    verification: []
    human_judgment: true
    rationale: "失败路径代码已就位（定值常量文案 + logEvent 单行 + 注册前零残留），行为全断言按 plan 切片归 11-03 失败桩测试与 phase11.mjs S3——本 plan 不宣称闭合"

duration: 40min
completed: 2026-09-04
status: complete
---

# Phase 11 Plan 01: per-client 生命周期主干 Summary

**per-client 模式端到端打通：attach（ticket 核销）→ hubMu 外 spawn 独立 PTY（Hello 钳制尺寸直通）→ Welcome 回显 → 输入/输出双向管道 → 断开即 SIGHUP（teardown Once 全序列含 KILL 兜底）→ 子死仅本端私有 EXIT+1000；shared 模式逐字节零回归（全量 -race 绿 + 八 UAT 脚本基线对齐）。**

## Performance

- **Duration:** 40 min
- **Started:** 2026-09-03T16:13:49Z
- **Completed:** 2026-09-03T16:54:37Z
- **Tasks:** 2
- **Files modified:** 5（3 改 + 2 新建）

## Accomplishments

- per-client 主干 tracer 全链成立：TestPerClientTwoClientsTwoPids 双端双 pid 不等 + 2s 静默窗零串台；TestPerClientWelcomeDims 首帧 Welcome cols==111/rows==44/mode=="rw"；TestPerClientInputEcho 输入回显全链（五测全 PASS，-race）
- 装配面切换落地：New 入口 sess×mode 契约 panic 两行（planner 裁定承载位，options_test.go 一字节未动）+ New 尾部模式分岔（per-client 零全局 goroutine）+ INPUT case cl.inQ 一行切换 + inputWriter 包级参数化 + detach/kick 两处 teardown 挂点 + maybeExitWhenEmptyLocked/Shutdown 两处 nil-deref 守卫
- main.go run() per-client 启动期零子进程（sess=nil 切换 + startOpts 单一声明收编兑现 10-01 注释预言 + 两回滚块 nil 守护）；ValidateOptions/opts 字面量/New 调用点逐字不动
- 零回归双证据：`go test -race -count=1 ./...` 全仓全绿；phase02-09.mjs 八主脚本默认 shared 模式零修改重跑 exit 全 0（12/18/10/28/23/34/21/18 逐一对齐 Phase 10 基线）；`git diff -w` 审查 shared 行为行逐字未动（仅再缩进）

## Task Commits

1. **Task 1: per-client 主干端到端装配（tracer, TDD）** - `8850986` (feat)
2. **Task 2: 装配契约与输入链测试 + 零回归收口闸首跑** - `484e3f3` (test)

**Plan metadata:** 见文末最终 docs commit（SUMMARY/STATE/ROADMAP）

## Files Created/Modified

- `internal/server/perclient.go`（新建）— pcSession 结构 + upgradePerClient + startSessionGoroutines + sessionWatcher + teardownPCLocked（文件头登记 D-01..D-04/PC-02..04 与研究/Pitfall 锚点）
- `internal/server/perclient_test.go`（新建）— startPerClientServer harness（spawned 追踪收口夹具）+ readOutputUntil + 五测
- `internal/server/server.go` — New 契约 panic/pcSessions 字段/尾部分岔；Attach 升档分岔（else 内 shared 序列逐字 + client 字面量追加 inQ: s.inputQ）；INPUT case :1120 一行切换；Shutdown stop-signal 段守卫；三处注释更新（SpawnFunc 两注释 + ValidateOptions 预言句指向 New 契约）
- `internal/server/clients.go` — client +inQ/pc 两字段；inputWriter 包级参数化（+pty import）；detach/kick 两处 teardown 挂点；maybeExitWhenEmptyLocked 早退守卫
- `cmd/wesh/main.go` — run() 分岔区：startOpts 收编 + sess=nil 分支 + 两回滚块 nil 守护 + 10-01 裁定注释块更新为落地事实陈述

## Decisions Made

- **perclient_test.go 包归属**：plan 文本「package server」与同句「复用 e2e_test.go 同包既有形态」矛盾——按后者裁决落 `package server_test`（同包 = server_test 才成立；dialHello/dialHelloPayload/captureStderr/parseEvents/eventsNamed 复用是硬约束，options_test.go 的 package server 白盒形态不适用于本文件任何用例）
- **Task 1 提交粒度**：plan 显式单 feat 提交指令优先于通用 TDD 双提交流；RED 观察（New nil sess panic @server.go:489）在任务内完成即转 GREEN，避免不可编译中间提交
- **需求勾选时机**：PC-02/03/04 跨 6 plan 共享，本 plan 仅机制本体+冒烟（success_criteria 明示行为断言归 11-03/04/05），REQUIREMENTS.md 勾选留给 phase 末 plan（防可追溯表污染）
- **tracer 反馈门**：本 run 无 checkpoint 任务（Pattern A 全自治），按自治形态重跑 tracer verify（build+vet+per-client 测+全量 -race 全绿）后放行 Task 2

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] plan 文本包名矛盾裁决（package server → package server_test）**
- **Found during:** Task 1 ⑩（perclient_test.go 新建）
- **Issue:** plan 写「package server」，但同句要求「dialHello 复用 e2e_test.go 同包既有形态」——dialHello 在 package server_test，两者互斥；且 Task 2 ③ 的 captureStderr/eventsNamed 同属 server_test
- **Fix:** 落 package server_test，helper 全复用零重造
- **Files modified:** internal/server/perclient_test.go
- **Verification:** 五测全 PASS；验收 grep 计数（三测名 ==3 / 契约文案 ≥2）全过
- **Committed in:** 8850986 / 484e3f3

**2. [Rule 3 - Blocking] Attach 分岔 else 块再缩进经 gofmt 收口**
- **Found during:** Task 1 ⑧（server.go 升档分岔）
- **Issue:** shared 升档序列 ~90 行整体移入 else 分支需 +1 缩进，手工逐行重排高风险
- **Fix:** 结构编辑后以 GOROOT gofmt -w 机械重排（10-05 收口闸工具纪律），`git diff -w` 复核行为行逐字未动
- **Files modified:** internal/server/server.go
- **Verification:** gofmt -l 全仓零输出；全量 -race 绿；八脚本基线对齐
- **Committed in:** 8850986

---

**Total deviations:** 2 auto-fixed（均为 Rule 3 阻塞类：plan 文本自相矛盾裁决 + 机械重排工艺）
**Impact on plan:** 无范围蔓延；两条均为落地工艺/文本矛盾调和，行为面与 plan 逐字一致。plan 文本的「package server」记述偏差建议 planner 侧知悉（行号勘误表同类）。

## Issues Encountered

None——一次构建通过，两冒烟测 RED（nil sess panic @server.go:489，预期形态）→ GREEN 直接转绿；全量 -race 与八脚本首轮全绿，无返工。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 11-02/11-03（容量再闸与 spawn 失败面）：upgradePerClient 内预留位序注释已锚定（闸在 spawn 前、复检在 registerLocked 同一 hubMu 持有内）；spawn_failed 审计通道已通
- 11-04（竞态注入）：teardownOnce/reaped/waitDone/teardownDone 同步边全部就位；export_test.go 暴露面按需新增
- 11-05/11-06（phase11.mjs 八场景）：/tmp/wesh-p11/wesh 构建口径已验证；本机 Linux headless 协议层直跑
- 已知中间态（均明示接受，注释锚定）：per-client 下 --once/--exit-when-empty 永不退出（守卫在 maybeExitWhenEmptyLocked）；SIGTERM 后进程不自退（第二信号逃生形态既有）；/healthz session_active 恒 false + wesh_session_active 恒 0；RESIZE 静默无效（直通归 Phase 12）
- 窗口期记账提醒：TestPerClientNoSessionStartEvent 的 /healthz 断言在 Phase 13 语义裁决落地时按裁决翻转（测试注释已锚定）

## Self-Check: PASSED

- 文件存在性：perclient.go / perclient_test.go / server.go / clients.go / main.go / 11-01-SUMMARY.md 全部 FOUND
- 提交存在性：8850986（Task 1 feat）/ 484e3f3（Task 2 test）全部 FOUND
- 删除检查：两提交 `git diff --diff-filter=D` 均无删除行

---
*Phase: 11-per-client*
*Completed: 2026-09-04*
