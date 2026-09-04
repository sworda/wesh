---
phase: 12-per-client
plan: "05"
subsystem: testing
tags: [regression-gate, closing-gate, gofmt, go-vet, race-detector, darwin-cross-compile, uat-matrix, diff-audit, wr-01-closure, requirements-checkoff, zero-regression]

requires:
  - phase: 12-per-client
    plan: "01"
    provides: "WelcomePayload.Session 协议面 + 前端模式位 reset + phase12-dom.mjs D1/D3（TestWelcomeFrameSession / TestPerClientWelcomeSession）"
  - phase: 12-per-client
    plan: "02"
    provides: "RESIZE 直通分支 + ro 双闸配对 + debouncer 共用件 + Go 五测 + phase12-dom D2"
  - phase: 12-per-client
    plan: "03"
    provides: "阻塞持帧 + dwell 看门狗 + outbox notFull 恢复信号 + GateTransitionsForTest 观测出口 + Go 四测（WR-01 闭合代码与注释双侧落码）"
  - phase: 12-per-client
    plan: "04"
    provides: "phase12.mjs 六场景协议层 UAT（两轮 20/20 基线）+ RawStallClient 夹具 + pinger/dwell 竞态发现（Phase 13 裁决材料）"
provides:
  - "Phase 12 收口闸六段式全绿证据：静态面（GOROOT gofmt 零输出/vet 零输出）+ 全量 -race 5 包 1m21.441s（Phase 12 新测 12/12 逐名对账）+ GOOS=darwin amd64/arm64 双编译闸 + web 构建 dist byte-identical + UAT 矩阵 16 轮全绿（既有 10 协议脚本默认 shared 零修改重跑 PASS 计数与基线逐脚本一致 + 3 jsdom + phase12.mjs 两轮 20/20 + phase12-dom 14/14）"
  - "期望值逐字未动 diff 审查：白名单三处逐条吻合 + 白名单补充项①（export_test.go 观测出口，12-03 plan 明示落地项，append-only 零断言——12-05 plan 白名单枚举缺口如实登记）+ 断言放宽形态扫描测试文件零命中 + 红线文件（metrics.go/main.go）零 diff + 零新依赖终审（T-12-SC）"
  - "WR-01 闭合回指登记：STATE.md 登记项（规划期 :99，现位 :103——12-01..04 决策追加后行移，内容逐字核对）按 D-04「dwell 涵盖不复刻」形态显式闭合声明"
  - "PC-05/PC-06/PC-07/PC-10/PC-11 五需求勾选（三证据链：Go 新测组 12 测 + phase12.mjs 六场景两轮 + phase12-dom 三场景）+ Traceability 五行 Complete + ROADMAP Phase 12 完成标记"
affects: [13 (Phase 13 规划输入：pinger/dwell 竞态裁决材料就绪 + stop-timeout 默认值重议 + spawn 双令牌桶), 14 (双模式验证矩阵 + herdr UAT——本 phase 零回归基线为对照面)]

# Actuals (#2632) — 与 plan estimate (40000 tokens) 同标尺（diff chars/4）。
# 口径注记：纯验证收口 plan（11-06 同形态）——代码面零改动，diff 全部在
# .planning 文档面（REQUIREMENTS/ROADMAP/SUMMARY/STATE/WINDOWS）。
# 诚实注记（11-06/12-03/12-04 同款）：diff 标尺不覆盖执行期成本——收口闸
# 六段式实跑（全量 -race 81s + UAT 矩阵 16 轮含 10s 级 dwell 真实等待
# ~6.5min + diff 逐行审查）不在 tokens 口径内。
actuals:
  tokens: 7400
  tasks: 2
  commits: 1

tech-stack:
  added: [] # 零新依赖红线终审（T-12-SC）：go.mod/go.sum/web/pnpm-lock.yaml/web/uat/package.json/web/package.json/web/pnpm-workspace.yaml 基点以来 0 行 diff
  patterns:
    - "收口闸六段式记录形态（11-06 先例直接沿用）：命令 + 退出码 + PASS 计数 + diff 清单全量落档，纯验证 plan 零代码 commit"
    - "phase 基点口径第二代：86433a6^ = e8b39c0（Phase 12 首提交父提交 = Phase 11 PR #15 merge 点）——branching_strategy=none 平直 main，merge-base 退化，11-06 先例同构"
    - "plan 白名单枚举缺口的审查处置形态：白名单外文件按「改动授权来源 + append-only + 零断言」三轴裁决并如实登记为补充项（不静默放行、不机械判死）"

key-files:
  created:
    - .planning/phases/12-per-client/12-05-SUMMARY.md
  modified:
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md
    - .planning/STATE.md
    - .planning/WINDOWS.md

key-decisions:
  - "phase 基点 = e8b39c0（86433a6^，Phase 12 首提交「docs(12): capture phase context」的父提交 = Phase 11 PR #15 merge 点）——branching_strategy=none 下 merge-base 退化，11-06 先例（phase 首提交父提交）同构"
  - "diff 审查白名单补充项①：export_test.go M（+17/-0，GateTransitionsForTest 观测出口）为 12-03 plan 明示落地项（12-03-PLAN files_modified + action 4 逐字记载），export_test.go 系零断言纯观测出口文件（t.Fatal/t.Error grep = 0）——12-05 plan 白名单枚举未列此文件属 plan 文本枚举缺口而非回归；「期望值逐字未动」审查目的完全保持（无任何期望值触碰），按偏差如实登记不判收口失败"
  - "WR-01 闭合回指按 D-04「dwell 涵盖不复刻」形态登记（本 SUMMARY 专段）：dwell 10s 从停读起点武装结构性涵盖 500ms attach 宽限（×20 余量）；阻塞持帧即暂存（帧在闭包栈上 ≡ creditPending 字段语义等价）；若瞬态满箱误踢案例实证出现则回写重开（CONTEXT deferred 既定口径）"
  - "11-06 收口提交形态沿用：Task 1 纯验证零文件改动 + Task 2 文档面，两任务零代码 task commit，全部产出（SUMMARY/REQUIREMENTS/ROADMAP/STATE/WINDOWS）单 docs commit 收口"

requirements-completed: [PC-05, PC-06, PC-07, PC-10, PC-11]

coverage:
  - id: G1
    description: "收口闸六段式全绿（承 11-06 口径）：静态面 + 全量 -race + darwin 双编译闸 + web 构建 + UAT 矩阵 + 期望值逐字未动 diff 审查"
    verification:
      - kind: other
        ref: "本 SUMMARY Verification 节六段式全量落档（全部命令本轮实跑 exit 0，无转述前序 SUMMARY 的二手证据）"
        status: pass
    human_judgment: false
  - id: G2
    description: "零回归双证据：shared 全量 Go 测试原样绿（五包 -race exit 0）+ 既有协议 UAT 默认 shared 模式零修改重跑全过（PASS 计数与 v1.0/11-06 基线逐脚本一致）"
    verification:
      - kind: integration
        ref: "段② 全量 -race + 段⑤ 基线对照表（12/18/10/28/3/23/34/21/18/21 + jsdom 37/19/40）+ 零修改实证（git diff 基点 web/uat 仅两新增文件）"
        status: pass
    human_judgment: false
  - id: G3
    description: "期望值逐字未动（PC-05 prohibition：收口不得以放宽断言换绿）：白名单三处逐条吻合 + 放宽形态扫描测试文件零命中 + 既有断言行零触碰（删除行 = 恰 7 个 WelcomeFrame 调用点 + 1 邻接注释行）"
    verification:
      - kind: static
        ref: "段⑥ diff 审查逐行对账（proto_test.go 4 删除行/clients_test.go 4 删除行/perclient_test.go 0 删除行 + 全 diff 放宽形态 grep）"
        status: pass
    human_judgment: false
  - id: G4
    description: "PC-05/06/07/10/11 五需求勾选承载兑现：三证据链（Go 新测 12 测 / phase12.mjs 六场景两轮 / phase12-dom 三场景）+ Traceability 五行 Complete"
    verification:
      - kind: other
        ref: "REQUIREMENTS.md 五条 [x]（plan verify grep '^5$' PASS）+ 需求-证据映射表（本 SUMMARY）"
        status: pass
    human_judgment: false
  - id: G5
    description: "WR-01 闭合回指登记：STATE.md 登记项按 D-04「dwell 涵盖不复刻」形态显式闭合声明（含重开条件）"
    verification:
      - kind: other
        ref: "本 SUMMARY「WR-01 闭合回指」专段（登记原文逐字引用 + 闭合论证 + 验证证据 + 重开条件四件）"
        status: pass
    human_judgment: false

duration: 32min
completed: 2026-09-04
status: complete
---

# Phase 12 Plan 05: Phase 12 收口闸（六段式验证总账 + 五需求勾选）Summary

**Phase 12 零回归收口闸六段式一次跑齐全绿：静态面（GOROOT gofmt 零输出/vet 零输出）+ 全量 -race 5 包 1m21.441s（Phase 12 新测 12/12 逐名 PASS）+ GOOS=darwin amd64/arm64 双编译闸 + web 构建 dist byte-identical + UAT 矩阵 16 轮全绿（既有 10 协议脚本默认 shared 零修改重跑与基线逐脚本一致 + 3 jsdom + phase12.mjs 两轮 20/20 + phase12-dom 14/14）+ 期望值逐字未动 diff 审查（白名单三处吻合 + 补充项①如实登记 + 放宽形态零命中 + 红线文件零 diff + 零新依赖终审）；PC-05/06/07/10/11 五需求勾选收口，WR-01 按 D-04「dwell 涵盖不复刻」形态闭合回指登记。**

## Performance

- **Duration:** 32 min
- **Started:** 2026-09-04T15:04:02Z
- **Completed:** 2026-09-04T15:36Z
- **Tasks:** 2（Task 1 六段式收口闸 + Task 2 需求勾选/ROADMAP/SUMMARY——均为验证与文档任务，零代码改动，11-06 同形态）
- **Files modified:** 0（代码面）；文档面 = REQUIREMENTS.md + ROADMAP.md + 本 SUMMARY + STATE.md + WINDOWS.md

## Task Commits

本 plan 为收口闸（Task 1 纯验证零文件改动 + Task 2 文档面），两任务零代码 task commit——11-06 同形态：

1. **Task 1: 收口闸六段式执行 + 期望值逐字未动 diff 审查** — 无 commit（证据落本 SUMMARY Verification 节）
2. **Task 2: 需求勾选 + ROADMAP 同步 + SUMMARY（WR-01 闭合回指登记）** — 并入文末最终 docs commit

**Plan metadata:** docs(12-05) commit（SUMMARY/REQUIREMENTS/ROADMAP/STATE/WINDOWS 五件）

## Verification（六段式全量证据落档——全部命令本轮实跑）

### 段① 静态面（Task 1 步 1）

| 命令 | 结果 |
|------|------|
| `$(go env GOROOT)/bin/gofmt -l .`（GOROOT=/data1/home/zexueli/softwares/go，go1.26.3 linux/amd64） | 零输出 = 零未格式化文件 PASS |
| `go vet ./...` | 无输出 exit 0 PASS |

### 段② 全量 -race + Phase 12 新测名单对账（Task 1 步 2）

`time go test -race ./... -count=1` exit 0，real 1m21.441s：

```
ok  github.com/sworda/wesh/cmd/wesh         1.334s
ok  github.com/sworda/wesh/internal/proto   1.015s
ok  github.com/sworda/wesh/internal/pty     2.654s
ok  github.com/sworda/wesh/internal/server  79.954s
ok  github.com/sworda/wesh/web              1.011s
```

Phase 12 新测 12/12 逐名对账（-race -v 聚焦组 16.331s）：

| 来源 | 测试名 | 结果 |
|------|--------|------|
| 12-01 | TestWelcomeFrameSession（proto，含两子测） | PASS |
| 12-01 | TestPerClientWelcomeSession | PASS (0.01s) |
| 12-02 | TestPerClientResizePassthroughRW | PASS (0.10s) |
| 12-02 | TestPerClientResizePassthroughRO | PASS (0.10s) |
| 12-02 | TestPerClientResizeIsolation | PASS (0.91s) |
| 12-02 | TestPerClientROInputDropped | PASS (2.01s) |
| 12-02 | TestPerClientInputRateLimitKept | PASS (1.20s) |
| 12-03 | TestPerClientStallBlocksAndResumes | PASS (4.55s) |
| 12-03 | TestPerClientDwellKick | PASS (0.85s) |
| 12-03 | TestPerClientDwellNoKickWhileProgressing | PASS (5.15s) |
| 12-03 | TestPerClientStallGateTransitions | PASS (0.39s) |

### 段③ darwin 编译闸（Task 1 步 3）

| 命令 | 结果 |
|------|------|
| `GOOS=darwin GOARCH=amd64 go build ./...` | exit 0 PASS |
| `GOOS=darwin GOARCH=arm64 go build ./...` | exit 0 PASS |

（darwin 运行行为测试由 CI macOS leg 承担，本机仅编译闸——11-06 口径。）

### 段④ web 构建（Task 1 步 4）

| 项 | 结果 |
|----|------|
| `cd web && time pnpm build` | exit 0，real 2.886s（tsc 类型检查 + vite:singlefile，dist/index.html 500.28 kB） |
| dist byte-identity | md5 `6393c78357233c16ff9a1ea5f3c7990e` 复建前后一致；`git status -- dist/` 零输出——提交态与复建产物逐字节一致 PASS |

### 段⑤ UAT 矩阵（Task 1 步 5）

`time go build -o /tmp/wesh-uat/wesh ./cmd/wesh` exit 0（real 0.667s，11819287 字节）后按序串行执行，16 轮全绿（日志存 /tmp/wesh-uat/uat-logs/）：

| 脚本 | exit | PASS/SKIP | 基线对照 |
|------|------|-----------|----------|
| phase02 | 0 | 12/12 | 12 ✓（11-06 基线） |
| phase03 | 0 | 18/18 | 18 ✓ |
| phase04 | 0 | 10/10 | 10 ✓ |
| phase05 | 0 | 28/28 + 1 skipped 豁免 | 28 ✓ |
| phase05-dims | 0 | 3/3（DIMS PASS：D6H-1 等价锁 + D6H-2 负对照） | —（v1.0 期脚本） |
| phase06 | 0 | 23/23 + 1 skipped 豁免 | 23 ✓ |
| phase07 | 0 | 34/34 + 1 skipped 豁免 | 34 ✓ |
| phase08 | 0 | 21/21 | 21 ✓ |
| phase09 | 0 | 18/18 | 18 ✓ |
| phase11 | 0 | 21/21 + 1 skipped 豁免 | 21 ✓（11-06 基线） |
| phase04-dom | 0 | 37/37 | —（v1.0 期脚本） |
| phase05-dom | 0 | 19/19 | 19 ✓（12-02 记录） |
| phase06-dom | 0 | 40/40 + 2 skipped 豁免 | 40+2skip ✓（12-01/02 记录） |
| phase12 轮一 | 0 | 20/20 | 20 ✓（12-04 两轮基线），real 25.974s |
| phase12 轮二 | 0 | 20/20 | 20 ✓，real 26.152s |
| phase12-dom | 0 | 14/14（D1/D2/D3 三场景） | 14 ✓（12-01/02 记录），real 5.651s |

- **零修改实证**：`git diff --name-status e8b39c0 -- 'web/uat/*.mjs' 'web/uat/*.sh' 'web/uat/package.json'` → 仅 `A web/uat/phase12-dom.mjs` / `A web/uat/phase12.mjs`——既有 13 个脚本（10 协议 + 3 jsdom）phase 以来零触达
- **`^  FAIL` 机器闸**：16 份日志零命中 PASS
- **skipped 六行全部带 reason 且属平台豁免类**（CODEBUDDY.md 分层测试策略 §5）：phase05 S7（像素层渲染，浏览器）、phase06-dom D9/D12b（真实 OS 断网栈/真实 AT 栈）、phase06 S7（真实断网栈 + tmux/herdr 重绘观感）、phase07 S8c（真实弹浏览器）、phase11 S4b（OS 网卡栈断网时序）——零跳过测试类项
- **`grep -i fail` 命中人工复核**：`webgl addon load failed, stay on DOM renderer`（phase04-dom ×16——jsdom 无 WebGL2 的 FE-01 DOM 渲染器回落设计内 console.warn，非断言失败）+ `auth_failed` / `"failed to start process"` 协议机器串断言目标文案——全部为机器串，非断言失败
- **滞留进程检查**：`pgrep -f '^/tmp/wesh-uat/wesh'` 锚定零命中（首轮未锚定命中两 pid 经 `ps -p` 实证为查询命令自身 bash 包装——pgrep -f 自匹配陷阱，11-06 patterns 登记形态）；零滞留 wesh、零滞留子进程 PASS

### 段⑥ 期望值逐字未动 diff 审查（Task 1 步 6）

**phase 基点口径**：`branching_strategy: none`（平直 main）下 merge-base 退化为 HEAD 自身——以 Phase 12 首提交 **86433a6**（`docs(12): capture phase context`）的父提交 **e8b39c0**（Phase 11 PR #15 merge 点）为等价基点，11-06 先例同构。

`git diff e8b39c0 -- 'internal/**/*_test.go' 'web/uat/*.mjs'` name-status：

| 状态 | 文件 | 白名单归属 |
|------|------|-----------|
| M | internal/proto/proto_test.go | ① 四调用点加参 + 新增函数 |
| M | internal/server/clients_test.go | ① 三调用点加参 + 邻接注释补写 |
| M | internal/server/export_test.go | 补充项①（偏差登记，见下） |
| M | internal/server/perclient_test.go | ② 仅新增函数与 import |
| A | web/uat/phase12.mjs | ③ 纯新增（754 行） |
| A | web/uat/phase12-dom.mjs | ③ 纯新增 |

**白名单逐条对账**：

1. **① proto_test.go（4 删除行）**：删除行恰 = 四处 WelcomeFrame 调用点改写（`WelcomeFrame(ModeRO, nil, 80, 24)` → `..., "shared")` 等四形，12-01 D-08 加参）+ 3 行注释新增（"shared" 字面量值域同源声明）+ 新增 TestWelcomeFrameSession 函数（12-01 新测）——**断言行（t.Fatalf/t.Errorf）零触碰** ✓
2. **① clients_test.go（4 删除行）**：删除行 = 三处 WelcomeFrame 调用点改写（TestWriterMergeControlFramesOnly 的 wRO/wRW + TestAfterDrainResendsDims 的 kickOrCreditLocked 入参，均 + `SessionModeShared` 常量）+ 1 行调用点邻接注释补写（「第 5 参 session（D-08）传 SessionModeShared 常量」）——**断言行零触碰** ✓
3. **② perclient_test.go（0 删除行）**：纯新增 12 函数（10 测试 + waitGateTransitions/assertSeqContinuity 两 helper）+ import 增 `"runtime"` 一行——与白名单②「仅新增函数与 import」逐字吻合 ✓（12-03 对 TestPerClientROInputDropped 的 labeled break 修复发生在本 phase 内新增函数上，phase 级 diff 呈现为纯新增）
4. **③ web/uat/**：仅 phase12.mjs 与 phase12-dom.mjs 两纯新增文件，既有脚本零修改（段⑤ 实证）✓
5. **补充项①（白名单外，偏差如实登记）——export_test.go M（+17/-0）**：GateTransitionsForTest 观测出口（ForTest 四件套：hubMu 内读/调用方不得持 hubMu/仅服务测试注释）。裁决：**12-03 plan 明示落地项**（12-03-PLAN files_modified 列有 export_test.go + action 4「export_test.go：GateTransitionsForTest（PCSationsLenForTest :48-52 同款）」逐字记载）；export_test.go 系零断言纯观测出口文件（`grep -cE 't\.(Fatal|Error)'` = 0 实证）；append-only 零删除行。12-05 plan 白名单枚举未列此文件属 **plan 文本枚举缺口而非回归**——「期望值逐字未动」审查目的完全保持（无任何期望值触碰；回退反而破坏 12-03 已提交测试的编译）。按 deviation 登记（WINDOWS.md #33）而非判收口失败。

**断言放宽形态扫描**（prohibition PC-05/PC-10 双面）：全 diff grep「两模式都接受 / both modes / either mode / shared||per-client / per-client||shared」→ 命中仅 .planning 文档中的禁令原文引用 + web/dist/index.html bundle 编译产物（前端 D-07 闸 `isRO && sessionMode !== 'per-client'` 的正确编译形态）——**测试文件零命中** ✓

**红线文件零 diff**：`git diff e8b39c0 --name-only -- internal/server/metrics.go cmd/wesh/main.go` = 0 行——D-05（metrics.go 零改动红线）与 D-03（dwell 不暴露 CLI/TOML，main.go 零改动）保持 ✓

**零新依赖红线终审（T-12-SC）**：`git diff e8b39c0 -- go.mod go.sum web/pnpm-lock.yaml web/uat/package.json web/package.json web/pnpm-workspace.yaml` = **0 行**——依赖清单零漂移，legitimacy 门不触发 ✓

**全仓面对账**：代码面 13 文件（10 M + 2 A + dist/index.html M，2238 insertions / 62 deletions）+ .planning 文档面；删除(D) 零文件——清单外改动零。

## WR-01 闭合回指（D-04 规划期要求——显式闭合声明）

**登记原文**（STATE.md 规划期 :99，现位 :103——12-01..04 决策追加后行移，内容逐字核对）：

> [Phase 11 REVIEW WR-01 → Phase 12]: per-client 输出闭包 trySend 失败直踢 1013（kickSlowConsumerLocked），丢失 05-13 attach 宽限与信用门暂存层——慢链路新端瞬态满箱即循环丢会话；PATTERNS:218 母本为 kickOrCreditLocked。Phase 12（1013/背压语义主场）规划时收口：补宽限门 + creditPending/afterDrain 重投

**闭合声明——按 D-04「dwell 涵盖不复刻」形态闭合**（12-03 落码，本收口闸全量重跑验证）：

1. **dwell 结构性涵盖宽限**：`defaultSlowDwell`（10s 内部常量，D-03 零 CLI/TOML 暴露）从停读起点武装——结构性涵盖 05-13 的 500ms attach 宽限场景（×20 余量）与一切瞬态满箱。WR-01 担忧的「慢链路新端瞬态满箱即循环丢会话」不再成立：瞬态满箱下帧被持在闭包栈上（不丢），dwell 计时启动，10s 内客户端跟上（正常 attach 场景毫秒级）即续读零丢失。
2. **阻塞持帧即暂存**：帧在闭包栈上 ≡ shared creditPending 字段的暂存语义等价——shared 复刻 creditPending/afterDrain 重投的唯一理由是 fan-out 不能为单端阻塞读循环（其他端要等帧），per-client 单消费者下复刻即死代码面。宽限门与 creditPending/afterDrain 重投**均不复刻**（代码与注释双侧回指：perclient.go 闭包注释段 + clients.go defaultSlowDwell 注释段）。
3. **验证证据**（本收口闸全量重跑全绿）：12-03 四测（TestPerClientStallBlocksAndResumes 停读续读字节连续 / TestPerClientDwellKick 到期 1013 / TestPerClientDwellNoKickWhileProgressing 慢但在前进永不踢 / TestPerClientStallGateTransitions 停读续读计数配对）+ 12-04 phase12.mjs S5（停读 3s 恢复后 34,888,896 字节算术步进严格 +1 连续零缺口）与 S6（生产 10s dwell 零覆写真实到期 → 1013 slow_consumer 逐字 + ESRCH 收割）。
4. **重开条件**（CONTEXT deferred 区既定口径）：若未来实证瞬态满箱误踢案例（10s dwell 内合法恢复的消费形态仍被 1013），回写 WR-01 登记项重开 deferred 项——补宽限门再评估。当前证据面（含真实浏览器后台标签页节流风险接受段，CONTEXT specifics 量化论证）支持闭合。

## 需求勾选证据映射（T-12-13 mitigate——勾选承载兑现）

| 需求 | Go 测 | phase12.mjs | phase12-dom.mjs |
|------|-------|-------------|-----------------|
| PC-05（RESIZE 直通） | ResizePassthroughRW / RO / Isolation 三测 | S2（双端隔离 + 零 'W' 帧）/ S3（ro 直通 + shared 对照） | D2（ro 上行 + shared 零 RESIZE） |
| PC-06（重连 reset） | TestWelcomeFrameSession / TestPerClientWelcomeSession | S1（Welcome.session 双模式对照） | D1（reset 全链 alt-screen 残影判别）/ D3（缺键不 reset） |
| PC-07（ro 门控+限速） | ROInputDropped / InputRateLimitKept 两测 | S4（ro INPUT 丢弃 + rw 限速保留） | —（ro RESIZE 归 PC-05 D2） |
| PC-10（1013 踢出） | DwellKick / StallGateTransitions 两测 | S6（真实 10s+ dwell → 1013 + ESRCH） | — |
| PC-11（停读续读） | StallBlocksAndResumes / DwellNoKickWhileProgressing 两测 | S5（停读 3s → 34.9MB 字节级连续） | — |

三证据链在本收口闸全部重跑验证（段② 12/12 逐名 + 段⑤ phase12.mjs 两轮 20/20 + phase12-dom 14/14）；diff 白名单审查证实零断言放宽（PC-05 prohibition）与零跳过项（PC-10 prohibition——skipped 仅平台豁免类带 reason，段⑤）。

## Files Created/Modified

零代码产物（Task 1 纯验证 + Task 2 文档面）。文档面：

- `.planning/REQUIREMENTS.md` — PC-05/06/07/10/11 五条勾选 + Traceability 五行 Complete + Last updated 行
- `.planning/ROADMAP.md` — Phase 12 顶部清单 [x] + completed 日期 + Plans 5/5 + Wave 5 勾选 + Progress 表 5/5 Complete
- `.planning/phases/12-per-client/12-05-SUMMARY.md` — 本文件（六段式证据 + WR-01 闭合回指）
- `.planning/STATE.md` — GSD 状态簿记（advance-plan/update-progress/record-metric/add-decision/record-session）
- `.planning/WINDOWS.md` — 偏差登记 #33（export_test.go 白名单枚举缺口裁决）

## Decisions Made

（全部登记于 frontmatter key-decisions，此处列执行要点）

- **phase 基点 e8b39c0**：86433a6^——Phase 12 首提交父提交（Phase 11 PR #15 merge 点），branching_strategy=none 下 11-06 先例同构
- **白名单补充项①裁决**：export_test.go 出现在 M 清单（白名单三处之外）——按「改动授权来源（12-03 plan 逐字记载）+ append-only（+17/-0）+ 零断言（纯观测出口文件）」三轴裁决为 plan 文本枚举缺口而非回归，如实登记不判收口失败；「期望值逐字未动」审查目的完全保持
- **11-06 收口提交形态沿用**：两任务零代码 task commit，五件产出单 docs commit
- **STATE.md 簿记与内容区分工**：Task 2 按 plan 不改写 STATE.md 内容区（WR-01 回指落在 SUMMARY 专段）；GSD 标准簿记（计数器/指标/决策/会话）经 gsd-tools 处理器照常更新——12-01..04 同款

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - 计划文本缺口] diff 审查白名单外一项：export_test.go M**
- **Found during:** Task 1 段⑥（name-status 对账）
- **Issue:** 12-05 plan 白名单枚举三处未含 export_test.go，但该文件在本 phase 被 12-03 修改（GateTransitionsForTest 观测出口，+17/-0）——plan 字面「白名单外任何改动即收口失败」与 phase 实际改动面冲突
- **Fix:** 三轴裁决（12-03 plan 明示授权 + append-only + 零断言文件）登记为白名单补充项①；证据完整保留（本 SUMMARY 段⑥ 第 5 条 + WINDOWS.md #33）；不回退（回退破坏 12-03 已提交测试编译）、不判收口失败（审查目的「期望值逐字未动」完全达成）
- **Files modified:** 无代码改动（纯登记）
- **Commit:** docs(12-05)（本 SUMMARY + WINDOWS.md）

## Issues Encountered

- pgrep -f 自匹配陷阱再现（滞留进程检查首轮命中两 pid = 查询命令自身 bash 包装）——锚定形态 `^/tmp/wesh-uat/wesh` 复核零滞留，11-06 patterns 复用，零成本排除
- 六段式全部首轮全绿，零返工零修复

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Phase 12 收口完成**：PC-05/06/07/10/11 勾选 + 零回归双证据 + WR-01 闭合回指落档；v1.1 进度 6/15 需求（PC-01→10，PC-02/03/04→11，本 phase 五条）
- **Phase 13（资源防线与终结语义）**：裁决材料就绪——pinger/dwell 竞态（STATE Blockers 登记：writeControl 5s 写超时 × pinger 单一 DeadlineExceeded 判读，默认配置下 TCP 级停读客户端 1006 先杀；裁决面 = pinger 区分写阻塞超时与 pong 等待超时，或接受 1006 语义）+ stop-timeout 默认值重议（SIGHUP 吸收实证已入账）+ spawn 双令牌桶 + Shutdown N 进程组 + SEC-09 WESH_REMOTE_USER + OPS-12 metrics/审计 per-client 粒度（gateTransitions 双模式聚合口径已就绪）
- **Phase 14（双模式验证矩阵 + herdr UAT）**：本 phase 零回归基线（10 协议脚本 PASS 计数 + 3 jsdom + diff 审查形态）为其对照面；RawStallClient 夹具与 phase12.mjs 六场景可直接复用；Playwright herdr 全链在 Windows 工作站（双机拓扑）
- **威胁登记闭合**：T-12-13（无证据勾选面）→ mitigate 兑现（六段式全绿 + 三证据链映射表 + diff 白名单审查防放宽）；T-12-SC（依赖面）→ mitigate 兑现（六依赖文件 0 行 diff 终审）

## Self-Check: PASSED

- 文件存在性：SUMMARY / REQUIREMENTS / ROADMAP / WINDOWS 全在场
- Task 1/Task 2 零代码 commit 兑现（纯验证 + 文档面，11-06 形态）；仅文末 docs(12-05) commit
- 证据锚点抽查：REQUIREMENTS 五 [x]（plan verify grep '^5$' 实测 PASS）；ROADMAP Progress 5/5 Complete + Phase 12 [x] 实测 PASS；16 份 UAT 日志在场（/tmp/wesh-uat/uat-logs/）与本文表格一致
- 六段式验证输出与本文记录一致（全部命令本轮实跑 exit 0，无转述前序 SUMMARY 的二手证据）

---
*Phase: 12-per-client*
*Completed: 2026-09-04*
