---
phase: 11-per-client
plan: "03"
subsystem: server
tags: [per-client, capacity-gate, spawn-failure, race-recheck, reap, go, websocket]

requires:
  - phase: 11-per-client
    plan: "01"
    provides: "per-client 生命周期主干（upgradePerClient 位序锚点/pcSessions 注册表/teardownPCLocked 信号纪律/spawn_failed 审计通道/perclient_test.go harness）"
provides:
  - "D-02 pre-spawn 容量再闸：hubMu 短临界区读 len(pcSessions) >= maxClients，满员即 Error{server_error,\"server is at capacity\"} 直写 + max_clients 事件 + Close(1011)——与 spawn 失败同码同串，分辨率由事件名承担（wire 聚合、日志细分）"
  - "D-03 注册点复检回收：spawn 成功后同一 hubMu 持有内复检，超编者 reapOrphanSession 异步回收 + 同容量拒绝序列——「并发子进程数 ≤ maxClients」硬不变量 Phase 11 即成立，Phase 13 裁决项④提前消解"
  - "reapOrphanSession（internal/server/perclient.go）：SignalGroup(HUP 字面——容量回收非断开语义不走 --stop-signal 通道) → AfterFunc 补 KILL（局部 reaped 原子闸，Pitfall 2 同构）→ Drain(200ms) → Close → Wait 完整收割，孤儿会话唯一收割者，绝不占 hubMu"
  - "capacityMessage 常量单点 + rejectCapacity 序列单点（两容量拒绝路径 wire 形态不可区分是有意为之，D-02）"
  - "PCSessionsLenForTest 测试出口（export_test.go append-only）+ startPerClientServerWithSpawn harness（spawnFn 可注入 + spawnedSessions 访问器）"
  - "三测锁定：spawn 失败 Pitfall 5 清理清单逐条 / D-02 linger 形态容量闸 / D-03 barrier 竞态「恰一胜一负 + 终态==1 + 败者 ESRCH」"
affects: [11-04, 11-05, 11-06, 13-termination, 14-matrix]

actuals:
  tokens: 8408
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "容量拒绝单点化：capacityMessage 常量 + rejectCapacity helper——pre-spawn 闸与复检回收两路径共用同一字面量/同一序列（D-02「wire 聚合」的结构性保证）"
    - "linger 形态确定性注入：trap 免疫会话断开待收割滞留 pcSessions + registry 清空——WS 面容量闸的确定性触发窗口（phase11.mjs S6 将用同形态）"
    - "barrier 竞态注入：SpawnFunc 进入即阻塞共享 barrier（计数器==2 放行）——并发升档同过 pre-spawn 闸的超编窗口确定性打开"
    - "回收路径 reaped 闸局部 atomic.Bool（孤儿会话无 watcher 无注册，无第二置位/读取方——Pitfall 2 同构语义且免 hubMu 耦合）"

key-files:
  created: []
  modified:
    - internal/server/perclient.go
    - internal/server/perclient_test.go
    - internal/server/export_test.go

key-decisions:
  - "容量文案走常量形态（capacityMessage 单点定义）——满足验收「文案字面量仅一处定义」且两路径结构性和文案漂移绝缘"
  - "Task 2 TDD 以 plan 显式单 test 提交收口（11-01 先例延续）：RED=PCSessionsLenForTest undefined 编译失败观察在任务内完成，避免不可编译中间提交"
  - "harness 薄包装保旧签名——11-01 五测调用点零改动（断言逐字保持），新三测走 startPerClientServerWithSpawn 全量返回（srv/spawnedSessions）"
  - "竞态测不映射「连接↔会话」身份：以「两会话 pgid 恰一个 ESRCH + 另一个存活」承载「败者被收割」语义（wire 面两容量拒绝形态有意不可区分）"
  - "PC-02 勾选延续 11-01/11-02 既定：ID 跨 6 plan 共享，phase 末 plan 11-06 统一勾选（requirements-completed 留空防可追溯表污染）"

requirements-completed: []

coverage:
  - id: D1
    description: "spawn 失败 Pitfall 5 清理清单逐条锁：B 收恰一帧 Error{server_error,failed to start process 逐字}+1011；spawn_failed 恰一条（code=1011、零敏感值——不含注入文本与 argv 路径）；无 B attach 事件；/healthz clients==1；A echo 照常（他端零感知）"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientSpawnFailure"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-02 pre-spawn 容量再闸：linger 形态（trap 免疫会话滞留 pcSessions + registry 清空）下 B 过 ③位 503 闸后命中 WS 面闸——Error 容量文案逐字 + close 1011 + max_clients 事件恰一条；注册表计数不变（闸在 spawn 前零 spawn）；A pgid 存活实证 trap 免疫"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientCapacityGate"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-03 注册点复检回收（barrier 竞态注入）：恰一胜一负（Welcome / Error 容量文案+1011）+ 注册表终态==1 + 败者 pgid 2s 内 ESRCH（回收含 Wait 收割无僵尸）——「并发子进程数 ≤ maxClients」硬不变量竞态实测"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "internal/server/perclient_test.go#TestPerClientCapacityRecheckRace（-count=3 复跑无 flake）"
        status: pass
    human_judgment: false
  - id: D4
    description: "零回归收口闸：shared 全量 Go 测试原样绿 + 期望值逐字未动；既有测试文件零改动（本 plan diff 仅 perclient.go/perclient_test.go/export_test.go，export_test.go append-only 无删除行）"
    verification:
      - kind: other
        ref: "go test -race -count=1 ./...（5 包全绿）；git diff --stat 范围核对；export_test.go diff 零删除行；GOROOT gofmt -l 零输出"
        status: pass
    human_judgment: false
  - id: D5
    description: "phase11.mjs S6 协议层对照（D-02 wire 形态的 UAT 面双锁）"
    requirement: PC-02
    verification: []
    human_judgment: true
    rationale: "Go 测试侧已锁定 1011+容量文案 wire 形态；协议层 UAT 对照按 plan 切片归 11-05（phase11.mjs S6 同 linger 注入形态已锚定）——本 plan 不宣称闭合"

duration: 19min
completed: 2026-09-04
status: complete
---

# Phase 11 Plan 03: per-client 容量防线与 spawn 失败面闭合 Summary

**per-client 昂贵资源（每连接一进程）硬帽机制落地：D-02 pre-spawn 容量再闸（1011+「server is at capacity」wire 形态，与 spawn 失败同码同串、事件名细分）+ D-03 注册点复检回收（reapOrphanSession 完整 SignalGroup→Drain→Close→Wait 序列，「并发子进程数 ≤ maxClients」硬不变量竞态注入实测成立，Phase 13 裁决项④提前消解）+ spawn 失败 Pitfall 5 清理清单逐条测试锁定；shared 模式全量 -race 原样绿、既有测试零改动。**

## Performance

- **Duration:** 19 min
- **Started:** 2026-09-03T17:30:05Z
- **Completed:** 2026-09-03T17:49:22Z
- **Tasks:** 2
- **Files modified:** 3（perclient.go +103/-8；perclient_test.go +425/-10 演进；export_test.go +13/-0 append-only）

## Accomplishments

- D-02 pre-spawn 容量再闸落地（upgradePerClient 内 effMode 单行门之后、spawnFunc 之前）：hubMu 短临界区只读计数、绝不持锁 spawn（Anti-Pattern 1）；满员拒绝序列单点化（capacityMessage 常量 + rejectCapacity helper）；注释登记 1013/1008 否决理由与 ③位 503 闸分工（注册表满→503 既有零改动 / linger 或竞态窗口→WS 面 1011 本 plan 新增面）
- D-03 注册点复检回收落地：注册段同一 hubMu 持有内复检，超编者放锁后 reapOrphanSession 异步回收（HUP 字面——容量回收非断开语义不走 --stop-signal 通道；局部 reaped 原子闸防 kill-after-reap，Pitfall 2 同构；孤儿会话唯一收割者，Drain/Close/Wait 阻塞面绝不占 hubMu）——Phase 13 裁决项④（spawn-intent 口径）提前消解，注释锚定 STATE.md Blockers 移除指引
- 三测行为锁定（-race 全绿 + 竞态测 -count=3 无 flake）：失败面（恰一帧 Error 逐字文案 + 1011 + spawn_failed 恰一条零敏感值 + 无 B attach 事件 + clients==1 + A echo 照常）；linger 形态容量闸（确定性触发 D-02 闸，phase11.mjs S6 同形态锚定）；barrier 竞态（恰一胜一负 + 终态==1 + 败者 pgid ESRCH 收割实证）
- harness 演进零回归：startPerClientServer 薄包装保 11-01 五测调用点签名（断言逐字未动），export_test.go append-only

## Task Commits

Each task was committed atomically:

1. **Task 1: 容量再闸 + D-03 注册点复检回收 + reapOrphanSession（D-02/D-03，PC-02）** - `e7c6252` (feat)
2. **Task 2: spawn 失败清理清单 + 容量闸三形态测试（TDD）** - `812d801` (test)

**Plan metadata:** 见文末最终 docs commit（SUMMARY/STATE/ROADMAP）

## Files Created/Modified

- `internal/server/perclient.go` — capacityMessage 常量 + rejectCapacity 单点；upgradePerClient pre-spawn 闸（D-02）+ 注册段复检分支（D-03）；reapOrphanSession 异步回收（teardownPCLocked 之后）；文件头 11-03 落地锚定更新
- `internal/server/perclient_test.go` — harness 拆分（startPerClientServerWithSpawn 通用形态 + 薄包装）+ handshakeCollectUntilClose/decodeSingleErrorFrame/healthzClients 三 helper + 三新测（测名仅现于 func 声明行——grep 行计数闸纪律）
- `internal/server/export_test.go` — 11-03 观测出口（hubMu 内 len 读，调用方不得持 hubMu；注释不含出口名——验收 grep ==1 闸）；append-only 零删除行

## Decisions Made

- **容量文案常量形态**：acceptance 允许字面量两处或常量单点——选常量（capacityMessage），「文案字面量仅一处定义」人工核对项结构性满足，两路径文案漂移绝缘
- **Task 2 TDD 单 test 提交**（11-01 先例延续）：plan 显式提交指令优先于通用 TDD RED/GREEN 双提交流；RED（PCSessionsLenForTest undefined ×5 处编译失败）任务内观察后转 GREEN，避免不可编译中间提交
- **harness 薄包装保旧签名**：startPerClientServer 签名不变 → 11-01 五测调用点零改动（「调用点机械更新」实际为零更新，断言逐字保持——零回归红线的最保守形态）
- **竞态测不断言胜者身份**（注释明示非断言放宽）：以「恰一胜一负 + 注册表终态==1 + 两会话 pgid 恰一个 ESRCH」锁定不变量；「连接↔会话」身份在 wire 面不可映射是有意设计（D-02 两拒绝形态不可区分）
- **PC-02 勾选延续既定**：跨 6 plan 共享 ID，phase 末 11-06 统一勾选；本 plan flagged_assumptions 保持 flagged-unverified

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] gofmt doc-comment 规则把注释内 `trap '' HUP` 规整为排版引号**
- **Found during:** Task 2 ⑥（收口自检 gofmt -l）
- **Issue:** GOROOT gofmt（go1.26.3 现代 doc-comment 规则，10-05 收口闸工具纪律）将 doc 注释中成对 ASCII 单引号 `''` 规整为 `”`，使注释文本不再忠实描述 argv 实际内容
- **Fix:** 注释改写为 `trap "" HUP`（双引号形态——stopseq_test.go 文件头同款先例；shell 语义相同，argv 代码行 `trap '' HUP` 不变）
- **Files modified:** internal/server/perclient_test.go
- **Verification:** GOROOT gofmt -l 全仓零输出；三测 -race 全绿
- **Committed in:** 812d801

---

**Total deviations:** 1 auto-fixed（Rule 3 阻塞类：收口闸工具纪律调和，纯注释措辞，行为面零影响）
**Impact on plan:** 无范围蔓延；与 plan 逐字一致外的唯一调整为注释引号形态。

## Issues Encountered

None——一次构建通过，Task 1 验收 grep 六项首轮全过（len 计数 ==2 / 容量文案 ==1 / max_clients ≥1 / reapOrphanSession ==1 / pcSess.Wait() ==1 / 1011 同码同串 ==2）；Task 2 RED→GREEN 直接转绿，-count=3 复跑无 flake。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 11-04（竞态注入/断开行为断言面）：teardown 与 watcher 同步边既有；harness 的 spawnFn 注入位与 spawnedSessions 访问器可直接复用
- 11-05（phase11.mjs 八场景）：S6 容量再闸用本 plan Go 测试同款 linger 注入形态（trap 免疫滞留 + registry 清空窗口）；S3 运行期删命令注入的 wire 断言面（Error 文案逐字 + 1011 + spawn_failed）已被 Go 测锁定提供协议层对照基准
- 11-06（phase 收口闸）：PC-02 勾选承载点；本 plan coverage D1-D4 全 pass、D5 留 UAT 对照
- Phase 13 规划提醒：STATE.md Blockers 裁决项④（spawn-intent 口径）已经 D-03 提前消解——规划时直接移除该开放项（perclient.go 复检分支注释已锚定）；spawn 令牌桶与 Shutdown N 组仍为 Phase 13 本体
- 威胁登记闭合：T-11-03a（竞态超编 DoS）→ mitigate（复检回收 + 竞态实测）；T-11-03b（敏感值泄露）→ mitigate（定值常量 + 事件零敏感值断言）；T-11-03c（kill-after-reap）→ mitigate（局部 reaped 原子闸 + 唯一收割者纪律）；T-11-SC 零新依赖保持（go.mod/go.sum 零漂移）

## Self-Check: PASSED

- 文件存在性：internal/server/perclient.go / internal/server/perclient_test.go / internal/server/export_test.go / .planning/phases/11-per-client/11-03-SUMMARY.md 全部 FOUND
- 提交存在性：e7c6252（Task 1 feat）/ 812d801（Task 2 test）全部 FOUND
- 删除检查：两提交 `git diff --diff-filter=D` 均无文件删除；export_test.go diff 零删除行（append-only）
- 验收 grep：Task 1 六项 + Task 2 六项全部达标（见 Issues Encountered 与任务内记录）

---
*Phase: 11-per-client*
*Completed: 2026-09-04*
