---
phase: 11-per-client
plan: "06"
subsystem: testing
tags: [regression-gate, gofmt, go-vet, race-detector, darwin-cross-compile, uat, diff-audit, per-client, zero-regression]

requires:
  - phase: 11-per-client
    plan: "01"
    provides: "per-client 生命周期主干 + perclient_test.go 五测（TwoClientsTwoPids/WelcomeDims/InputEcho/NewModeSessContract/NoSessionStartEvent）"
  - phase: 11-per-client
    plan: "02"
    provides: "darwin dup-watch fail-closed（errDupWatch + watch() 防御 + TestWatchDupPidFailClosed CI 锁）"
  - phase: 11-per-client
    plan: "03"
    provides: "D-02 容量再闸 + D-03 复检回收 + spawn 失败清理清单三测（SpawnFailure/CapacityGate/CapacityRecheckRace）"
  - phase: 11-per-client
    plan: "04"
    provides: "per-client 生命周期六行为测（ExitPrivate42/ExitSignalMinus1/DisconnectSIGHUP/ReconnectNewPid/StopTimeoutKillFallback/TeardownRaceOnce——StopTimeout=1s 覆写形态 14143fe）"
  - phase: 11-per-client
    plan: "05"
    provides: "web/uat/phase11.mjs 八场景协议层 UAT（21/21 + S4b skipped 豁免登记）"
provides:
  - "Phase 11 零回归收口闸六段式全绿证据：静态面（GOROOT gofmt 零输出/vet 干净/build 0.570s）+ 全量 -race 5 包 1m5.656s + 十四测名单对账 + GOOS=darwin build/vet 双闸 + phase02-09 八脚本两轮 exit 全 0（基线 12/18/10/28/23/34/21/18 逐脚本一致）+ phase11.mjs 21/21+1 skipped"
  - "期望值逐字未动 diff 审查四件套：既有 *_test.go 零修改（reap_darwin_test.go/export_test.go 两白名单 append-only 删除行==0）+ web/uat/ 仅新增 phase11.mjs + proto/web/src/metrics.go/health.go/resize.go 红线零出现 + 依赖清单零漂移（T-11-SC）"
  - "prohibitions 19 条人工确认零违反记录（11-01 七条 + 11-02 两条 + 11-03 三条 + 11-04 四条 + 11-05 三条）+ 已知中间态注释锚定与测试锁存在性确认"
  - "PC-02/PC-03/PC-04 需求勾选（双证据链闭合：Go 十四测 + 协议八场景 + diff 审查）"
affects: [12-interaction, 13-termination, 14-matrix]

actuals:
  tokens: 2600
  tasks: 2
  commits: 1

tech-stack:
  added: []
  patterns:
    - "收口闸六段式记录形态（10-04 先例延伸）：命令 + 退出码 + PASS 计数 + diff 清单全量落档，纯验证 plan 零代码 commit"
    - "phase 基点口径（branching_strategy=none 平直 main）：git merge-base HEAD main 退化为 HEAD 自身，phase 基点 = phase 首提交父提交（954da7c）"
    - "fail 命中行人工复核口径：grep -i fail 命中逐条确认为协议机器串文案（auth_failed）而非断言失败；^  FAIL 断言行零命中为机器闸"
    - "pgrep -f 自匹配陷阱纪律：滞留进程检查须排除查询命令自身（命令行含模式字面即自命中）"

key-files:
  created: []
  modified: []

key-decisions:
  - "phase 基点口径裁决：branching_strategy=none（无 phase 分支）下 plan 文本 git merge-base HEAD main 退化为 HEAD 自身——以 11-01 首提交父提交 954da7c（Phase 10 收口点，commit message 自证『Phase 11 起点』）为等价基点承载 diff 审查"
  - "PC-02/PC-03/PC-04 勾选承载兑现：三 ID 跨 6 plan 共享的 phase 末统一勾选既定决策（11-01/11-03/11-04/11-05 SUMMARY 四度登记）在本 plan 执行——十四测 + 八场景 + diff 审查三重证据链闭合"
  - "TestPerClientTeardownRaceOnce 重跑绿属预期内（14143fe StopTimeout=1s 覆写为已登记偏差，bash 4.4 pselect 竞态窗吸收 SIGHUP 系 shell 侧行为）——本 plan 不回退该覆写，全量 -race 与 -v 对账均按其现形态验收"

requirements-completed: [PC-02, PC-03, PC-04]

coverage:
  - id: D1
    description: "静态面三段全绿：GOROOT gofmt（go1.26.3 现代 doc-comment 规则，10-05 收口闸工具纪律）-l 零输出；go vet ./... 无输出 exit 0；time go build ./... 0.570s exit 0"
    verification:
      - kind: other
        ref: "$(go env GOROOT)/bin/gofmt -l .（零输出）&& go vet ./...（exit 0）&& time go build ./...（0.570s exit 0）"
        status: pass
    human_judgment: false
  - id: D2
    description: "全量 -race 双证据：go test -race -count=1 ./... 5 包全 ok 1m5.656s（cmd/wesh 1.317s/proto 1.014s/pty 2.651s/server 65.133s/web 1.011s）——shared 既有全部测试原样绿（零回归第一证据）+ per-client 十四测逐名 PASS（-v 名单对账 5+3+6=14：TwoClientsTwoPids/WelcomeDims/InputEcho/NewModeSessContract/NoSessionStartEvent + SpawnFailure/CapacityGate/CapacityRecheckRace + ExitPrivate42/ExitSignalMinus1/DisconnectSIGHUP/ReconnectNewPid/StopTimeoutKillFallback/TeardownRaceOnce）"
    requirement: PC-02
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./...（exit 0，5 包全 ok）；go test -race -count=1 ./internal/server/ -run 'TestPerClient|TestNewModeSessContract' -v（14/14 PASS，exit 0）"
        status: pass
    human_judgment: false
  - id: D3
    description: "darwin 编译闸：GOOS=darwin go build ./... 与 GOOS=darwin go vet ./... 双 exit 0（11-02 dup-watch 防御编译面实证；运行态由 CI macOS leg 承担）"
    requirement: PC-03
    verification:
      - kind: other
        ref: "GOOS=darwin go build ./...（exit 0）&& GOOS=darwin go vet ./...（exit 0）"
        status: pass
    human_judgment: false
  - id: D4
    description: "既有协议 UAT 零回归脚本级证据：phase02-09.mjs 八主脚本默认 shared 模式零修改重跑两轮 exit 全 0，PASS 计数 12/18/10/28/23/34/21/18 与 v1.0 基线逐脚本一致（10-04/11-01 对账同形态）；grep -i fail 命中 4 行逐条人工复核为 auth_failed 机器串文案非断言失败；^  FAIL 断言行八脚本零命中；跑后滞留进程零（pgrep 自匹配陷阱已排除）"
    verification:
      - kind: e2e
        ref: "for s in phase02..phase09: node web/uat/$s.mjs /tmp/wesh-p11/wesh（exit 全 0 ×2 轮；PASS 计数 12/18/10/28/23/34/21/18；/tmp/wesh-p11/uat-logs/ 八份日志存证）"
        status: pass
    human_judgment: false
  - id: D5
    description: "phase11.mjs 八场景全绿（D-06 兑现）：S1-S8 全 PASS（21/21 断言）+ S4b skipped 带 reason（CODEBUDDY.md §5 平台豁免，WINDOWS.md #31 在案）+ SEC 输出自净 details=21 零命中；exit 0"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（21/21 PASS + 1 skipped，exit 0；输出全文存 /tmp/wesh-p11/phase11-out.txt）"
        status: pass
    human_judgment: false
  - id: D6
    description: "期望值逐字未动 diff 审查（Pitfall 11 正面证据）：phase 基点 954da7c 以来——新增(A) 仅 perclient.go/perclient_test.go/phase11.mjs 三文件；修改(M) 六文件（main.go/reap_darwin.go/reap_darwin_test.go/clients.go/export_test.go/server.go）；删除(D) 零；既有 *_test.go 零修改（M 清单中仅 reap_darwin_test.go/export_test.go 两 append-only 白名单，'^-[^-]' 删除行计数双双==0）；web/uat/ 仅 phase11.mjs 新增；internal/proto/、web/src/、internal/server/metrics.go、health.go、resize.go 零出现；go.mod/go.sum/package.json/pnpm-lock.yaml 依赖清单零漂移（T-11-SC 闭合）"
    verification:
      - kind: other
        ref: "git diff 954da7c..HEAD --stat/--diff-filter=A,M,D + '^-[^-]' 删除行计数 + 红线路径 name-only（全部机器闸输出见 Verification 节）"
        status: pass
    human_judgment: false
  - id: D7
    description: "每阶段收口闸纪律复核 + prohibitions 19 条人工确认零违反 + 已知中间态（11-01 登记①②③④ + 11-03 linger 形态）注释锚定与测试锁存在性确认（本 plan 不裁决，翻转归 Phase 12/13）"
    verification:
      - kind: other
        ref: "五份前序 SUMMARY 逐份复核（命令/退出码齐全、零放宽语句、零遗留 FAIL、Self-Check 全 PASSED）+ 19 条 prohibitions 逐条对照 diff 与测试锁记录（见 Verification 节对账表）+ 中间态锚定 grep（clients.go:84/server.go:507,513,1569/perclient_test.go:361-391,482-522/server.go:510）"
        status: pass
    human_judgment: true
    rationale: "prohibitions 为 spec-less fallback 的 descriptor-less flagged-unverified 项——人工确认是 plan 明示的唯一闭合出口（Task 2 ⑤）；本 SUMMARY 承载逐条对照记录即确认凭证，VERIFICATION 阶段可据表复核"

duration: 11min
completed: 2026-09-04
status: complete
---

# Phase 11 Plan 06: Phase 11 收口闸（六段式验证总账）Summary

**Phase 11 零回归收口闸六段式一次跑齐全绿：静态面（GOROOT gofmt 零输出/vet 干净/build 0.570s）+ 全量 -race 5 包 1m5.656s（shared 原样绿 + per-client 十四测 -v 名单对账 5+3+6=14 全 PASS）+ GOOS=darwin build/vet 双闸 + 既有八 UAT 脚本两轮零修改重跑 exit 全 0（PASS 计数 12/18/10/28/23/34/21/18 与 v1.0 基线逐脚本一致）+ phase11.mjs 八场景 21/21+1 skipped + 期望值逐字未动 diff 审查四件套全过（既有测试零修改/append-only 零删除行/新增白名单三文件/红线六面零出现/依赖零漂移）；prohibitions 19 条人工确认零违反，PC-02/PC-03/PC-04 双证据链闭合并勾选。**

## Performance

- **Duration:** 11 min
- **Started:** 2026-09-04T01:27:27Z
- **Completed:** 2026-09-04T01:38:44Z
- **Tasks:** 2（均为纯验证任务，零代码改动零 task commit——证据落本 SUMMARY，与 10-04 Task 2 同形态）
- **Files modified:** 0（代码面；.planning 文档面 = 本 SUMMARY + STATE/ROADMAP/REQUIREMENTS 状态更新）

## Accomplishments

- **六段式验证总账一次跑齐**（命令 + 退出码 + PASS 计数全量落档于下方 Verification 节）：① 静态面三段 ② 全量 -race（5 包 65.1s 峰值为 server 包——十四测含 2s 级真实时序断言）③ darwin 双闸 ④ 八脚本两轮全绿 ⑤ phase11.mjs 八场景 ⑥ diff 审查四件套——全部 exit 0 零 FAIL
- **per-client 十四测名单对账闭合**：11-01 五测 + 11-03 三测 + 11-04 六测逐名出现于 -v 输出并全 PASS；TestPerClientTeardownRaceOnce 以 14143fe 覆写形态（StopTimeout=1s）1.28s 绿——预期内，不回退
- **期望值逐字未动正面证明**：diff-filter 分组（A=3 新文件/M=6 改文件/D=0）+ 两白名单测试文件 '^-[^-]' 删除行==0 + 红线六路径 name-only 零输出 + 依赖清单零漂移——「看似完成实则不然」行的机器闸证据
- **prohibitions 19/19 人工确认闭合**：11-01 七条（SEC-08 预认证零资源/hubMu 不横跨阻塞/CR-01/EXIT 禁 outbox 异步/失败文案零细节/零新 exitf/禁断言放宽）+ 11-02 两条 + 11-03 三条 + 11-04 四条 + 11-05 三条——逐条对照记录见 Verification 节
- **PC-02/PC-03/PC-04 需求勾选**：Go 十四测 + 协议八场景 + diff 审查三重证据链闭合（flagged-unverified 三行的 plan 层显式证据承载兑现，VERIFICATION 阶段对证收口材料齐备）

## Task Commits

本 plan 为纯验证收口闸（files_modified 空集），两任务零代码改动、零 task commit——与 10-04 Task 2 同形态：

1. **Task 1: 静态面 + 全量 -race + GOOS=darwin 编译闸** — 无 commit（证据落本 SUMMARY）
2. **Task 2: UAT 双证据 + 期望值逐字未动 diff 审查 + prohibitions 人工确认** — 无 commit（证据落本 SUMMARY）

**Plan metadata:** 见文末最终 docs commit（SUMMARY/STATE/ROADMAP/REQUIREMENTS）

## Verification（六段式全量证据落档）

### 段① 静态面（Task 1①）

| 命令 | 结果 |
|------|------|
| `$(go env GOROOT)/bin/gofmt -l .`（GOROOT=/data1/home/zexueli/softwares/go，go1.26.3 linux/amd64） | 零输出 = 零未格式化文件 PASS |
| `go vet ./...` | 无输出 exit 0 PASS |
| `time go build ./...` | exit 0，real 0m0.570s PASS |

### 段② 全量 -race + 十四测对账（Task 1②）

`go test -race -count=1 ./...` exit 0，real 1m5.656s：

```
ok  github.com/sworda/wesh/cmd/wesh         1.317s
ok  github.com/sworda/wesh/internal/proto   1.014s
ok  github.com/sworda/wesh/internal/pty     2.651s
ok  github.com/sworda/wesh/internal/server 65.133s
ok  github.com/sworda/wesh/web              1.011s
```

`go test -race -count=1 ./internal/server/ -run 'TestPerClient|TestNewModeSessContract' -v` exit 0——十四测逐名对账（5+3+6=14）：

| 来源 | 测试名 | 结果 |
|------|--------|------|
| 11-01 | TestPerClientTwoClientsTwoPids | PASS (2.01s) |
| 11-01 | TestPerClientWelcomeDims | PASS |
| 11-01 | TestPerClientInputEcho | PASS |
| 11-01 | TestNewModeSessContract（含两子测） | PASS |
| 11-01 | TestPerClientNoSessionStartEvent | PASS |
| 11-03 | TestPerClientSpawnFailure | PASS (0.01s) |
| 11-03 | TestPerClientCapacityGate | PASS (0.01s) |
| 11-03 | TestPerClientCapacityRecheckRace | PASS (0.21s) |
| 11-04 | TestPerClientExitPrivate42 | PASS (1.81s) |
| 11-04 | TestPerClientExitSignalMinus1 | PASS |
| 11-04 | TestPerClientDisconnectSIGHUP | PASS (0.03s) |
| 11-04 | TestPerClientReconnectNewPid | PASS (0.05s) |
| 11-04 | TestPerClientStopTimeoutKillFallback | PASS (1.01s) |
| 11-04 | TestPerClientTeardownRaceOnce（14143fe 覆写形态） | PASS (1.28s) |

### 段③ darwin 编译闸（Task 1③）

| 命令 | 结果 |
|------|------|
| `GOOS=darwin go build ./...` | exit 0 PASS |
| `GOOS=darwin go vet ./...` | exit 0 PASS |

### 段④ 既有 UAT 八脚本零修改重跑（Task 2②）

`time go build -o /tmp/wesh-p11/wesh ./cmd/wesh`（exit 0，real 0m0.454s，11849493 字节）后按序串行重跑，两轮全绿：

| 脚本 | 轮一 exit | 轮一 PASS | 轮二 exit | 轮二 PASS | v1.0 基线 |
|------|-----------|-----------|-----------|-----------|-----------|
| phase02 | 0 | 12/12 | 0 | 12 | 12 ✓ |
| phase03 | 0 | 18/18 | 0 | 18 | 18 ✓ |
| phase04 | 0 | 10/10 | 0 | 10 | 10 ✓ |
| phase05 | 0 | 28/28（1 skipped 豁免） | 0 | 28 | 28 ✓ |
| phase06 | 0 | 23/23（1 skipped 豁免） | 0 | 23 | 23 ✓ |
| phase07 | 0 | 34/34（1 skipped 豁免） | 0 | 34 | 34 ✓ |
| phase08 | 0 | 21/21 | 0 | 21 | 21 ✓ |
| phase09 | 0 | 18/18 | 0 | 18 | 18 ✓ |

- 脚本零修改实证：八主脚本最后改动提交 = 1649639（test(09-05)，v1.0 期），本 phase 零触达
- `grep -i fail` 命中 4 行逐条人工复核：phase03.log:1（场景标题含 auth_failed）/ phase03.log:7（PASS 行断言目标机器串）/ phase08.log:17（场景标题）/ phase08.log:19（PASS 行事件名断言）——全部为协议机器串文案词，非断言失败
- `^  FAIL`（脚本断言失败形态）八份日志零命中
- 滞留进程检查：`pgrep -f 'wesh-p11/wesh'` 命中 pid 888458 经 `ps -p` 实证为查询命令自身瞬态进程（pgrep -f 自匹配——命令行含模式字面即命中），排除后零滞留 wesh 进程、零滞留 trap 免疫进程（T-11-06b 闭合）

### 段⑤ phase11.mjs 八场景（Task 2③）

`node web/uat/phase11.mjs /tmp/wesh-p11/wesh` exit 0——21/21 PASS + 1 skipped：

```
S1a/S1b/S1c PASS（双 pid 独立 + 启动期零子进程 pgrep 实证 + 1.5s 静默窗零串台）
S2a/S2b PASS（首帧 Welcome cols==111/rows==44 + stty size "44 111" 回读）
S3a/S3b/S3c PASS（运行期删命令 → Error{server_error,"failed to start process" 逐字}+1011；他端零影响）
S4a PASS（断开 2s 护栏内 pgid ESRCH）；S4b SKIP（平台豁免 reason 登记，WINDOWS.md #31）
S5a/S5b/S5c/S5d PASS（EXIT 私有化 exit 42 / 信号 -1 大写 SIGHUP；B 1.5s 零帧 + 窗后 echo 照常）
S6a/S6b/S6c PASS（容量再闸 linger 注入 → "server is at capacity" 逐字 + 1011；A pgid 存活实证）
S7a/S7b PASS（断开重连 = 全新进程 pid2 ≠ pid1）
S8a/S8b PASS（trap 免疫 + stop-timeout=1s KILL 兜底：~300ms 存活 + 5s 护栏内 ESRCH）
SEC PASS（details=21 零 token 零 pid 数值）
```

### 段⑥ 期望值逐字未动 diff 审查（Task 2④）

**phase 基点口径**：`branching_strategy: none`（平直 main 提交）下 plan 文本的 `git merge-base HEAD main` 退化为 HEAD 自身（f9e3eba）；以 11-01 首提交父提交 **954da7c**（`docs(planning): v1.1 阶段重编号回填…+ Phase 11 起点`——commit message 自证）为等价基点。

`git diff 954da7c..HEAD -- internal/ web/uat/ cmd/` diff-filter 分组：

| 状态 | 文件 |
|------|------|
| 新增(A) | internal/server/perclient.go（+397）/ internal/server/perclient_test.go（+1183）/ web/uat/phase11.mjs（+595） |
| 修改(M) | cmd/wesh/main.go / internal/pty/reap_darwin.go / internal/pty/reap_darwin_test.go / internal/server/clients.go / internal/server/export_test.go / internal/server/server.go |
| 删除(D) | 零 |

四件套对账：

1. **既有 *_test.go 零修改**：M 清单中测试文件仅 reap_darwin_test.go 与 export_test.go（两个 plan 明示 append-only 白名单）；e2e_test.go/exit_test.go/stopseq_test.go/options_test.go 等全部既有测试零出现 ✓
2. **append-only 零删除行**：`git diff 954da7c..HEAD -- internal/pty/reap_darwin_test.go | grep -c '^-[^-]'` == 0；export_test.go 同款 == 0 ✓
3. **web/uat/ 仅新增 phase11.mjs**：diff-filter=A 清单实证 ✓
4. **红线零出现**：`git diff 954da7c..HEAD --name-only -- internal/proto/ web/src/ internal/server/metrics.go internal/server/health.go internal/server/resize.go` 零输出（协议零改动 / 前端零改动 / D-04 metrics 零改动三红线保持）✓
5. **依赖清单零漂移（T-11-SC）**：go.mod/go.sum/web/uat/package.json/web/uat/pnpm-lock.yaml/web/package.json/web/pnpm-lock.yaml 全零出现 ✓
6. **全仓面完整清单**：除上述 9 代码文件外仅 .planning/ 文档面 8 文件（ROADMAP/STATE/WINDOWS + 五份 SUMMARY）——3594 insertions(+)/160 deletions(-) 全量对账无清单外改动

### prohibitions 19 条人工确认记录（Task 2⑤——spec-less flagged-unverified 的人工确认出口）

| 来源 | prohibition | 确认证据 |
|------|-------------|----------|
| 11-01 #1 | 预认证零资源（spawn 在 ticket 核销后） | PATTERNS §1A 位序（checkTicket :1159 → 升档分支）+ phase11 S1a 启动期零子进程 pgrep 实证 ✓ |
| 11-01 #2 | hubMu 绝不横跨 spawn/Drain/Close/Wait | upgradePerClient 闸内读计数→放锁 spawn→再取锁注册；teardown 快/慢半段分裂；reapOrphanSession 注释「阻塞面绝不占 hubMu」；全量 -race 无 deadlock ✓ |
| 11-01 #3 | 读循环零同步直写 master（CR-01） | INPUT case cl.inQ 一行切换 + inputWriter 参数化；TestPerClientInputEcho 全链 PASS ✓ |
| 11-01 #4 | EXIT 禁 outbox 异步入队 | sessionWatcher 直写序列（组帧一次→Write 2s ctx→Close 1000）；S5a/S5d 帧序 EXIT 先于 1000 实证 ✓ |
| 11-01 #5 | 失败/容量文案零底层细节 | "failed to start process"/capacityMessage 双定值常量；11-03 三测 spawn_failed 零敏感值断言；S3b/S6b 文案逐字 PASS ✓ |
| 11-01 #6 | 零新 exitf 触发源 | TestPerClientTeardownRaceOnce exitf 桩全程零调用断言（len(exitCh)!=0 FAIL 结构性守卫）PASS ✓ |
| 11-01 #7 | 禁「两模式都接受」断言放宽 | 段⑥ 既有测试零修改 + 五份 SUMMARY/代码/phase11.mjs grep 放宽形态零命中 ✓ |
| 11-02 #1 | dup-watch fail-closed 唯一形态 | errDupWatch + watch() w.mu 内 dup 检查；TestWatchDupPidFailClosed 锁 CI macOS leg；darwin 编译闸段③ 双绿 ✓ |
| 11-02 #2 | 无新锁类型/不改 w.mu 保护面 | 11-02 diff +18/-0（仅 dup 检查分支与注释）；锁序全序保持 ✓ |
| 11-03 #1 | 容量拒绝 = 1011 唯一形态（否 1013/1008） | D-02 裁决落地；S6b close==1011 精确值断言 + Go 双测 PASS ✓ |
| 11-03 #2 | 事件零敏感值 | TestPerClientSpawnFailure/CapacityGate 事件断言 PASS（恰一条 + 零注入文本/argv 路径）✓ |
| 11-03 #3 | 回收必完整 Wait 收割 | reapOrphanSession SignalGroup→Drain→Close→Wait 全序列；竞态测败者 pgid 2s 内 ESRCH PASS ✓ |
| 11-04 #1 | EXIT 零广播零串台 | B 端 1.5s 静默窗零帧强形态（泵源零交付，任何类型字节到达即 FAIL）+ S5b PASS ✓ |
| 11-04 #2 | 断开路径无 linger 宽限 | TestPerClientDisconnectSIGHUP 2s 护栏内 ESRCH；S4a PASS ✓ |
| 11-04 #3 | 断开路径不自行 cmd.Wait | 唯一收割者 = sessionWatcher；teardown 只触发 + waitDone 同步边；竞态测零 panic ✓ |
| 11-04 #4 | 竞态测锁不变量非时序胜者 | 终态四件套 + exitf 零调用 + EXIT ≤1 且到达即 exit_code==0 论证链注释锚定（「竞态固有二形态≠断言放宽」）✓ |
| 11-05 #1 | S6 两关闭面各自逐字锁定 | S6b Error 帧数==1 + 文案逐字 + close==1011 精确值（无「503 或 1011 都接受」形态）；grep 放宽形态零命中 ✓ |
| 11-05 #2 | 控制台零 token/凭据/pid 数值 | SEC 输出自净 details=21 命中=false（本轮重跑实证）✓ |
| 11-05 #3 | S6/S8 trap 免疫滞留进程组清场 | 场景尾部 SIGKILL 清场 + startWesh 追踪收口；本轮跑后滞留进程零（自匹配陷阱排除后实证）✓ |

**确认结论：19/19 条零违反。**

### 五份前序 SUMMARY 复核（Task 2⑤）

| SUMMARY | 自报命令/退出码 | 放宽语句 | 遗留 FAIL | Self-Check |
|---------|----------------|----------|-----------|------------|
| 11-01 | 全量 -race 绿 + 八脚本 12/18/10/28/23/34/21/18 + diff -w 审查 | 零（grep 实证） | 零 | PASSED |
| 11-02 | darwin build/vet + 全量 -race 5 包 + diff +71/-0 | 零 | 零 | PASSED |
| 11-03 | 全量 -race 5 包 + export_test.go 零删除行 + gofmt 零输出 | 零 | 零 | PASSED |
| 11-04 | 六测 -race -count=3 18/18 + 全量两轮 + diff 仅 perclient_test.go | 零 | 零 | PASSED |
| 11-05 | phase11.mjs 21/21 ×3 连跑 exit 全 0 + pgrep 零滞留 | 零 | 零 | PASSED |

### 已知中间态锚定复核（不裁决，仅确认注释锚定与测试锁存在）

- **①② --once/--exit-when-empty 永不退出 + 第二终结源**：clients.go:84「永不翻转为 per-client」+ server.go:507/513（pcSupervisor/pcExitReq 归 Phase 13，Pitfall 1 窗口期）+ server.go:1569；测试锁 = TestPerClientTeardownRaceOnce exitf 零调用反向锁（PASS）✓
- **③ /healthz session_active 恒 false**：perclient_test.go:361-391（断言 + Phase 13 OQ①② 翻转锚定注释）；TestPerClientNoSessionStartEvent PASS ✓
- **④ RESIZE 静默无效**：server.go:510（已知中间态注释，直通归 Phase 12）✓
- **11-03 linger 形态**：perclient_test.go:482-522（注释锚定 + 5s 护栏就位轮询测试锁）✓

## Files Created/Modified

零代码产物（files_modified 空集兑现）。文档面：`.planning/phases/11-per-client/11-06-SUMMARY.md`（本文件）。

## Decisions Made

- **phase 基点口径裁决**：plan 文本 `git merge-base HEAD main` 预设了 phase 分支存在；本项目 `branching_strategy: none`（config.json 实证）平直提交于 main，merge-base 退化为 HEAD 自身——以 11-01 首提交父提交 954da7c 为 phase 基点等价物（其 commit message 自证「Phase 11 起点」；11-04 SUMMARY 的 689cfa3 系 11-04 plan 基点而非 phase 基点，本 plan 不沿用）
- **八脚本两轮重跑**：轮一对齐 PASS 基线（tail 摘要），轮二存全量日志（/tmp/wesh-p11/uat-logs/）承载 grep -i fail 人工复核与 ^FAIL 机器闸——10-04 复核口径的可复查形态升级
- **pgrep -f 自匹配陷阱登记**：滞留进程检查首轮命中 pid 888458，经 ps 实证为查询命令自身（命令行含模式字面即自命中）——排除后零滞留；该陷阱写入 patterns 供后续收口闸复用
- **TeardownRaceOnce 覆写保持**：14143fe（StopTimeout=1s）为已登记偏差（bash 4.4 pselect 竞态窗吸收 SIGHUP 系 shell 侧行为，kill 成功发出、cat 对照组 50/50 全收）——本 plan 按其现形态验收，不回退不「修复」

## Deviations from Plan

None - plan executed exactly as written（两任务均纯验证零代码改动；phase 基点口径与 shell 语法适配属执行环境事实陈述，非行为面偏差——bash 环境下以 bash 语法执行 plan 的 fish 形态 verify 命令串，语义逐字等价）。

## Issues Encountered

None——六段式全部首轮全绿，零返工；唯一调查项为 pgrep 自匹配虚警（当即经 ps 排除，实证零滞留）。

## Authentication Gates

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Phase 11 收口完成**：PC-02/PC-03/PC-04 勾选 + 证据链闭合；VERIFICATION 阶段对证材料齐备（本 SUMMARY 六段式落档 + 19 条 prohibitions 确认表 + 中间态锚定清单）
- **Phase 12（交互背压）**：RESIZE 直通/ro 断言/前端重连 reset/停读续读的全部服务端接缝就位（server.go:510 中间态注释为翻转锚点）；phase11.mjs 夹具层（dialHello/readPid/echoMark/pollESRCH）可直接复用
- **Phase 13（资源防线与终结语义）**：裁决项④（spawn-intent 口径）已经 11-03 D-03 提前消解——规划时从 STATE Blockers 移除；本体收窄为 spawn 令牌桶 + stop-timeout 默认值重议（①——SIGHUP 吸收实证已入账，14143fe 注释链）+ Shutdown N 组 + healthz/metrics OQ③ + SEC-09 WESH_REMOTE_USER + session_start/end per-client 粒度（D-04 窗口期空白翻转点 = TestPerClientNoSessionStartEvent 断言按裁决翻转，perclient_test.go:391 注释已锚定）
- **威胁登记闭合**：T-11-06a（收口阶段改动既有期望值）→ mitigate（零代码改动 + diff 四件套机器闸 + '^-[^-]' 双零 + 人工复核记录）；T-11-06b（UAT 连跑资源滞留）→ mitigate（串行执行 + 跑后零滞留实证）；T-11-SC → accept 保持（依赖清单零漂移 diff 实证）

## Self-Check: PASSED

- 文件存在性：.planning/phases/11-per-client/11-06-SUMMARY.md FOUND
- 提交存在性：本 plan 两任务纯验证零代码 commit（files_modified 空集兑现）——无 task commit 可查验；仅文末最终 docs commit
- 证据锚点抽查：/tmp/wesh-p11/phase11-out.txt 含 21 条 PASS 行（十四测对账表与八场景输出一致）；phase 基点 954da7c 身份确认（commit message 自证「Phase 11 起点」）
- 六段式验证输出与本文记录一致（全部命令本轮实跑 exit 0，无转述前序 SUMMARY 的二手证据）

---
*Phase: 11-per-client*
*Completed: 2026-09-04*
