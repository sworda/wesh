---
phase: 14-herdr-uat
plan: 04
subsystem: testing
tags: [go-test, dual-mode, resize-arbitration, d-03-not-assembled, mode-exclusive, pure-whitebox, context-registration, race-detector]

requires:
  - phase: 14-herdr-uat
    plan: 01
    provides: newTestServer 小族四形态装配点（newSessTestServer sess 形态——本 plan 原定首个消费方）+ D-03 未装配先例形态（TestGlobalCredit 可证伪列）
  - phase: 14-herdr-uat
    plan: 02
    provides: owner 四测 D-03 未装配列（multi_test.go——本 plan Task 2 偏差登记的对照确认面）+ sess 形态运行期首证（TestEchoPTY pid 锚点，提前于原预期）
provides:
  - resize_arb_test.go D-03 双模式形态（shared 四子测试仲裁语义逐字 + per-client 列仲裁器未装配三重可证伪断言——各自直通 / 无 min-rect 收敛 / 运行期零 'W' 帧；蓝本 resize 行 wire 面两半侧收口）
  - startResizeServer 散点消除（装配体逐字内联至 newSessTestServer shared 分支——全仓代码面零引用，D-01 终态）
  - clients_test/resize_test 纯白盒单跑判定（六测零改动）+ 蓝本三则归属偏差登记（14-CONTEXT.md D-02 偏差登记小节——14-06 续登 events/log 偏差的既定通道）
affects: [14-06, 14-12]

actuals:
  tokens: 8541   # 34163 chars / 4 over a8f32b1..HEAD（internal/server 三文件，估 40000 的 21%——单文件 D-03 改造 + 登记批的实际开销远低于 planner 低置信预估）
  tasks: 2
  commits: 2

tech-stack:
  added: []   # 零新依赖（T-14-SC 红线，go.mod/go.sum 零 diff）
  patterns:
    - "D-03 三重可证伪断言（mode-exclusive 未装配列）：异尺寸双端各自 RESIZE 直通（各自 winsize == 自报值）+ 无 min-rect 收敛负向余量断言（> 防抖窗后保持）+ 双泵静默窗零 'W' 帧——比 TestGlobalCredit 单面形态更完整的三观测面"
    - "专用装配 helper 收编内联终态：唯一调用面收编后 helper 删除、装配体逐字内联至小族对应分支（D-01 散点消除的完整闭环——14-02 startShutdownServerWith 同款第二例）"

key-files:
  created: []
  modified:
    - internal/server/resize_arb_test.go
    - internal/server/harness_test.go
    - internal/server/e2e_test.go
    - .planning/phases/14-herdr-uat/14-CONTEXT.md

key-decisions:
  - "per-client 列三重观测面选型：双端各自发 RESIZE（A→110x40 / B→70x25，均异于 Hello 基线与对端）——min-rect 反向收敛（若误装配则双端收敛 70x25）在「B 落定后 A 保持 110x40」负向断言中最具判别力；与 perclient_test.go TestPerClientResizeIsolation（仅 A 发 RESIZE、B 保持 Hello 值）的分界 = 本列独有「双端各自上报后的互不压缩」观测面"
  - "零 'W' 帧断言通道复用 readPump/frameRes 双泵 + 500ms select 静默窗（TestPerClientResizeIsolation 既有夹具零新写）；泵在 RESIZE 序列前武装使序列期间入缓冲的推送帧同样落入断言面"
  - "startResizeServer 删除的收编落位：装配体逐字内联至 newSessTestServer shared 分支（harness_test.go）——plan 验收闸「全仓零引用」要求代码与注释双面清零，e2e_test.go:36 陈旧注释行同步改写（pte.Start+server.New 表述）"
  - "纯白盒判定依据登记：clients_test（registry/writer/&Server{} 直构造零装配）与 resize_test（arbitrate() 纯函数直测）在 package server 白盒域，harness 小族在 server_test 包——包墙结构上不可达且无装配可参数化，保持单跑为形态判定而非覆盖缺口"

patterns-established:
  - "Pattern 8: D-03 未装配列的三重观测面（直通等值 + 互不压缩负向余量 + 零控制帧静默窗）——mode-exclusive 测试的最完整可证伪形态，后续同类改造（若出现）以此为上限参照"
  - "Pattern 9: CONTEXT 偏差登记小节（D-02 条目下「执行期偏差亦续登此处」）——规划期实证与执行期偏差的单一登记点，后续 plan（14-06 events/log）追加条目不改写既有三则"

requirements-completed: [PC-13]   # 共享 ID 门：herdr E2E 证据未齐（14-08 已过协议层，pw 观感层归 14-09），勾选归 phase 末收口 plan（14-01/14-02/14-03 先例延续）

coverage:
  - id: D1
    description: "resize_arb_test.go D-03 双模式改造（shared 四子测试仲裁断言逐字 + per-client 列未装配三重可证伪断言 + startResizeServer 收编删除）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "command: go test -race -count=1 -v -run 'TestArbitrate|TestResizeArbitration' ./internal/server/ → mode=shared 四子测试 + mode=per-client 全 PASS（3.13s）"
        status: pass
      - kind: unit
        ref: "grep -rn startResizeServer internal/ cmd/ web/src → 零命中；grep t.Skip internal/server/resize_arb_test.go → 零命中（字面闸——注释措辞规避 t.Skip 字样）"
        status: pass
      - kind: unit
        ref: "command: go vet ./... && $(go env GOROOT)/bin/gofmt -l internal/server/ → 零输出；go.mod/go.sum/ci.yml 零 diff"
        status: pass
    human_judgment: false
  - id: D2
    description: "shared 列期望值字面逐字一致（D-02 红线）——剥离缩进 diff 自审：四子测试断言行零改动，diff 仅为结构包裹 + 装配行换新形态 + per-client 列纯新增"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "git show HEAD~2:internal/server/resize_arb_test.go 与工作版 sed 剥离缩进后 diff——命中行仅：头注释新增 14-04 块 / import 删 net,net/http / startResizeServer helper 删除 / 四处装配行（Options 字面 → mutate 等价增量）/ for-mode+t.Run 包裹 / per-client 列新增"
        status: pass
    human_judgment: false
  - id: D3
    description: "纯白盒面判定登记（clients_test/resize_test 六测零改动单跑）+ 蓝本三则归属偏差 CONTEXT 回写（owner 四测实在 multi_test.go / resize wire 面实在 resize_arb_test.go / 计数不变量白盒与 wire 双面分置）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "grep -n 'owner 四测实在 multi_test.go' .planning/phases/14-herdr-uat/14-CONTEXT.md → :26 命中；test -z git status clients_test.go resize_test.go → 零输出"
        status: pass
    human_judgment: false
  - id: D4
    description: "全量回归零回归证据（包 -race 全量 + 全仓 5 包）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "command: go test -race -count=1 ./internal/server/ → ok 147.067s；go test -race -count=1 ./... → 5 包全 ok（148.9s）"
        status: pass
    human_judgment: false

duration: 27min
completed: 2026-09-06
status: complete
---

# Phase 14 Plan 04: mode-exclusive 未装配断言批（resize_arb D-03 + 纯白盒登记）Summary

**resize_arb_test.go 落 D-03 双模式形态——shared 列四子测试仲裁语义逐字（min-rect/2to1/防抖/参与集与 ro 闸/尺寸推送），per-client 列仲裁器未装配三重可证伪断言（异尺寸双端各自直通 + 无 min-rect 收敛 + 零 'W' 帧）；startResizeServer 收编删除（全仓代码面零引用）；clients_test/resize_test 六测纯白盒单跑判定 + 蓝本三则归属偏差登记落地（SUMMARY + CONTEXT 双通道）**

## Performance

- **Duration:** 27 min
- **Started:** 2026-09-06T15:30:50Z
- **Completed:** 2026-09-06T15:57:00Z
- **Tasks:** 2
- **Files modified:** 4（代码 3 + CONTEXT 1）

## Accomplishments

- **resize_arb_test.go D-03 双模式改造（Task 1）**：for-mode + t.Run("mode=") 包裹，per-client 列先return 分叉——三重可证伪面：① 异尺寸双端 A(100x30)/B(60x20) attach 各自 spawn（Hello 钳制尺寸出生基线）后各自 RESIZE（A→110x40 / B→70x25），经 sessions() 逐一会话 pollSize 断言各自 winsize == 各自自报值（若误装配仲裁器，双端参与集按 min-rect 收敛 70x25，A 的 110x40 断言翻红）；② 无 min-rect 收敛负向断言（B 落定后 300ms 余量 > 50ms 防抖窗，A 保持 110x40 不被压缩）；③ 运行期帧流零 'W' 类型字节（readPump 双泵 + 500ms select 静默窗，泵在 RESIZE 序列前武装使序列期间入缓冲的推送帧同落断言面）——缺席以断言承载而非跳过（D-03 既定裁决）
- **shared 列四子测试断言逐字（D-02 红线）**：all 模式 min-rect 与 2to1 恢复 / 防抖合并（200ms 覆写）/ owner 模式参与集与 ro 忽略闸 / 运行期尺寸变化推送（G-05-1）——期望值字面与注释零漂移（剥离缩进 diff 自审：断言行零改动）；装配换 newSessTestServer（mutate 传递原 Options 增量，Writable:true 基线同构），sess 读回统一经 sessions() 访问器取唯一会话
- **startResizeServer 散点消除（D-01 终态）**：本地 helper 删除，装配体逐字内联至 newSessTestServer shared 分支（harness_test.go——原本 5 调用面：本文件 4 + 小族 1）；e2e_test.go:36 陈旧注释行同步改写——`grep -rn startResizeServer internal/ cmd/ web/src` 零命中（.planning 历史档案保留时点准确提及，非引用面）
- **纯白盒面判定与偏差登记（Task 2）**：clients_test.go 三测（TestClientCountInvariant:29 / TestWriterMergeControlFramesOnly:96 / TestAfterDrainResendsDims:150——registry/writer/&Server{} 直构造零 server 装配）与 resize_test.go 三测（TestArbitrate:26 / TestSessionDimsLocked:87 / TestPushSessionDimsKickRecalc:123——arbitrate() 纯函数直测）保持单跑，两文件零改动；三则蓝本归属偏差（:391 owner 递补实在 multi_test.go:541/647/746/850 / :388 resize wire 面实在 resize_arb_test.go / :389 计数不变量白盒面 clients_test + wire 面 TestMaxClients503:1222 双面分置）登记 SUMMARY + 14-CONTEXT.md D-02 偏差登记小节（「执行期偏差亦续登此处」明示——14-06 续登 events/log 的既定通道）
- **验证矩阵**：TestResizeArbitration 双模式逐名 PASS（shared 四子测试 1.93s + per-client 列 1.21s，-race）；TestArbitrate 纯函数五子测零改动 PASS；包 -race 全量 147.1s/148.9s（两轮）+ 全仓 5 包 -race 绿；go vet 与 GOROOT gofmt（go1.26.3）零输出；go.mod/go.sum/ci.yml 零 diff（T-14-SC）
- **CI 时长增量**：本批增量 ≈ +1-2s（per-client 列 1.21s 实测；14-03 基线 ~141-147s → 本批后 ~147-149s）——三维归类改造的增量递减趋势确认（单测双跑 vs 洪水/静默窗类双跑），14-12 收口闸 CI 时长预算仍在蓝本 3-4 min/leg 界内

## Task Commits

Each task was committed atomically:

1. **Task 1: resize_arb_test.go D-03 双模式改造——仲裁器未装配显式断言 + startResizeServer 收编（D-01/D-03）** - `5202e23` (test)
2. **Task 2: 纯白盒面判定登记批——clients_test/resize_test 纯函数单跑判定 + 蓝本归属偏差登记与 CONTEXT 回写（D-02）** - `fffb894` (test)

**Plan metadata:** （见下方 docs 收口提交）

## Files Created/Modified

- `internal/server/resize_arb_test.go` - D-03 双模式改造：for-mode 包裹 + per-client 列三重可证伪断言 + 四处装配收编 newSessTestServer（sessions() 访问器）；startResizeServer 删除
- `internal/server/harness_test.go` - newSessTestServer shared 分支收编内联原 startResizeServer 装配体（+net/net/http 导入）；小族容量注释更新（sess 形态消费方现状）
- `internal/server/e2e_test.go` - :36 陈旧注释行改写（原引用已删 helper 名——全仓零引用字面闸的注释面清零）
- `.planning/phases/14-herdr-uat/14-CONTEXT.md` - D-02 条目下「偏差登记」小节新增（三则蓝本归属偏差 + 执行期续登通道明示）

## Decisions Made

- **per-client 列三重观测面选型（vs TestPerClientResizeIsolation 边界）**：双端各自发 RESIZE（均异于 Hello 基线与对端值）——「B 落定后 A 保持 110x40」的互不压缩负向断言为本列独有面（Isolation 测仅 A 发 RESIZE、B 保持 Hello 值）；min-rect 反向收敛（误装配则双端收敛 min(110,70)×min(40,25)=70x25）在该断言中最具判别力；resize 风暴/防抖细节与 ro 直通归 perclient_test.go Phase 12 三测单一承载（plan action 4 边界条款）
- **零 'W' 帧断言复用既有夹具**：readPump/frameRes 双泵 + 500ms select 静默窗（perclient_test.go 同款零新写）；cat 无自身输出使帧流恒静，任何 'W' 即运行期约束帧违规；泵在 RESIZE 序列前武装，序列期间入缓冲（chan cap 16）的推送帧在静默窗收干时同落断言面
- **startResizeServer 收编落位**：装配体逐字内联至 newSessTestServer shared 分支而非保留改名——plan 验收「全仓零引用」要求名字消失（grep 字面闸）；14-02 startShutdownServerWith 删除为同款第二例（专用 helper 唯一调用面收编后内联）
- **plan 文本「newSessTestServer(t, mode, argv, nil)」精度注记**：nil 为 per-client 列形态——shared 四子测试必须经 mutate 传递原 Options 增量（WritePolicy=all/owner、ResizeDebounce=200ms），否则逐字断言结构性失败（owner 默认策略下 B 降级 ro 使 min-rect 断言翻红）；装配语义等价（base Writable:true + mutate == 原完整 Options）
- **t.Skip 字面闸的注释措辞规避**：验收「grep 无 t.Skip 命中」为字面检查——文件头 D-03 裁决说明措辞用「缺席以断言承载而非跳过」而非引用跳过 API 名，保持机器闸干净

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - files_modified 枚举缺口] harness_test.go 与 e2e_test.go 必改但不在 plan files_modified 清单**
- **Found during:** Task 1
- **Issue:** plan 验收闸「startResizeServer 已删除且全仓零引用」与 files_modified 仅列 resize_arb_test.go + CONTEXT.md 矛盾——14-01 在 harness_test.go:135 为 newSessTestServer shared 分支引入了第 5 调用面（直传 startResizeServer），删除 helper 必须同步该分支；e2e_test.go:36 注释提及 helper 名（grep 字面命中）
- **Fix:** 装配体逐字内联至 newSessTestServer shared 分支（+net/net/http 导入）；e2e_test.go:36 注释行改写为 pty.Start+server.New 表述——`grep -rn startResizeServer internal/ cmd/ web/src` 零命中
- **Files modified:** internal/server/harness_test.go, internal/server/e2e_test.go
- **Verification:** 包 -race 全量绿（147.1s）+ 全仓 5 包绿；TestEchoPTY（sess 形态消费方）随全量回归绿
- **Committed in:** 5202e23

**2. [口径登记] plan 所记 owner 四测行号 :301/373/428/497 为规划期陈旧值**
- **Found during:** Task 2
- **Issue:** plan read_first/behavior 引用的 multi_test.go:301/373/428/497 在 14-02 改造后的现状文件中不对应任何 owner 测（14-02 的改造使行号整体后移）
- **Fix:** CONTEXT 回写按实测现位登记：TestOwnerPolicy:541 / TestAllPolicy:647 / TestSuccession:746 / TestSuccessionKickRace:850（证据命令 grep 实测核对）；蓝本归属修正本体（owner 四测实在 multi_test.go）与 plan 一致零漂移
- **Committed in:** fffb894

---

**Total deviations:** 1 auto-fixed（Rule 3 枚举缺口）+ 1 口径登记
**Impact on plan:** 零范围蔓延——两文件改动均为验收闸「全仓零引用」的必要补齐（13-06 签名扩散波及同形态先例，WINDOWS #36）；行号登记按实测现状，归属判定本体零漂移。

## Issues Encountered

None——首跑即双模式逐名绿（plan 低置信预估的风险面未现形）。

## User Setup Required

None - no external service configuration required.

## Known Stubs

None——per-client 列三重断言均为可执行真断言（直通等值/负向余量/静默窗零帧），零跳过形态、零 TODO/FIXME、零放宽（P11 和稀泥禁令：直通与仲裁两语义真值相反面显式分叉）。

## Next Phase Readiness

- 三维归类第三维（mode-exclusive + 纯白盒保持面）收口：resize_arb 为 D-03 形态的完整承载（三重观测面），TestGlobalCredit/owner 四测/Sigwinch/DrainBeforeAttach/PromoteKickOnce 各自既有列不变
- 14-06 收口核对可直接引用 CONTEXT D-02 偏差登记小节三则（「14-04 已登三则引用之，不重复登记」既定）；events/log 单跑偏差续登同小节（追加不改写）
- 14-12 收口闸 diff 白名单应含：本 plan 三代码文件 + CONTEXT.md 偏差登记小节；harness_test.go/e2e_test.go 的改动为 Rule 3 枚举缺口补齐（本 SUMMARY 登记）
- PC-13 需求勾选按 shared-ID 门延续（14-08 协议层已过、pw 观感层归 14-09）——归 phase 末收口 plan
- CI 时长：包 -race 全量 ~147-149s（本批 +1-2s）——改造增量递减趋势确认，蓝本 3-4 min/leg 预算界内

## Self-Check: PASSED

- 4 个 key-files 全部在盘且为零新文件（纯改造 + 登记）
- 2 个任务提交在库（5202e23 / fffb894）
- frontmatter `status: complete` 在场
- TestResizeArbitration 双模式逐名 PASS（-race）；包全量 147.1s + 全仓 5 包绿；go vet + GOROOT gofmt 零输出
- grep startResizeServer（internal/cmd/web/src）零命中；grep t.Skip（resize_arb_test.go）零命中；shared 列期望值逐字（剥离缩进 diff 自审）
- CONTEXT.md 偏差登记三则在册（:26 起）；clients_test.go / resize_test.go git status 零输出

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-06*
