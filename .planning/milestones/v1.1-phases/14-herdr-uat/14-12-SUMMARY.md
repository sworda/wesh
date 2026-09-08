---
phase: 14-herdr-uat
plan: "12"
subsystem: testing
tags: [closing-gate, regression-gate, dual-mode, race-detector, darwin-cross-compile, uat-matrix, load-matrix, diff-audit, requirements-checkoff, flagged-assumptions, zero-regression, milestone-closeout]
requires:
  - phase: 14-herdr-uat
    plan: "01"
    provides: "newTestServer 小族装配点（四形态）+ exit/stopseq/health/slowclient/limits mode-mapped 批"
  - phase: 14-herdr-uat
    plan: "02"
    provides: "e2e/multi 双模式断言分叉表（50 mode= 子测，owner 四测 D-03 未装配列）"
  - phase: 14-herdr-uat
    plan: "03"
    provides: "emptyexit/shutdown/metrics 三文件归一（第二终结源列 + session_active 语义 + series 镜像列）"
  - phase: 14-herdr-uat
    plan: "04"
    provides: "resize_arb D-03 可证伪列 + 纯白盒单跑判定 + 蓝本三则归属偏差 CONTEXT 回写"
  - phase: 14-herdr-uat
    plan: "05"
    provides: "handshake/keepalive + auth*/origin/throttle/tickets 八文件同断言双跑（TestReadOnlyAllowsResize 唯一分叉面）"
  - phase: 14-herdr-uat
    plan: "06"
    provides: "sharetoken/tls/customindex/proxy*/basepath 部署面批 + 三维归类 34 文件收口核对清单（216 mode= 子测）"
  - phase: 14-herdr-uat
    plan: "07"
    provides: "TestLoadPerClientFloodMatrix 洪水格 + TestLoadPerClientResident 驻留格（LOADDATA 八行 + D-11 判定证真）"
  - phase: 14-herdr-uat
    plan: "08"
    provides: "phase14.mjs S1 driving 翻转链 + S2 ro 汇聚两轮 18/18（PC-13 协议面）+ flagged_assumptions unclassified 承接块"
  - phase: 14-herdr-uat
    plan: "09"
    provides: "phase14-pw.mjs Windows 双 tab 观感两轮 4/4 + 截图六帧 + minrepro 双件套（PC-13 浏览器面）"
  - phase: 14-herdr-uat
    plan: "10"
    provides: "run-all.mjs 17 项矩阵 runner（D-04 形式化，首跑 17/17 基线一致）"
  - phase: 14-herdr-uat
    plan: "11"
    provides: "PC-12 文档三件套（README 会话模式节+标定表 / ARCHITECTURE 双模式段+GoTTY 误记修正 / CONFIGURATION max-clients 语义行）"
provides:
  - "Phase 14 收口闸六段式全绿证据：静态面（GOROOT gofmt + go vet + go vet -tags=load 三命令零输出）+ 全量 -race 五包 2m37.986s（mode= 子测试 216 RUN/216 PASS 与 14-06 收口审计精确一致——双模式 t.Run 覆盖的执行证据）+ GOOS=darwin amd64/arm64 双编译闸四命令零错 + web 构建 dist byte-identical（md5 d5c25e27 与 13-08 一致）+ UAT 矩阵 run-all.mjs 17/17 零修改重跑（210.9s 逐脚本计数与 13-08 基线/14-08/14-10 记录一致，6 skipped 全平台豁免带 reason）+ load 双剖面八格 + 期望值逐字未动 diff 白名单终审（35 文件零外改动/565 真删除行全归类/红线四查全零）"
  - "PC-12/PC-13 勾选承载兑现：REQUIREMENTS.md 两条 [x] + Traceability 两行 Complete + Last updated 行——v1.1 里程碑 15/15 需求全闭合"
  - "PROJECT.md Key Decisions 三行登记：三维归类执行机械（newTestServer 小族 + t.Run 双跑，D-01/D-02/D-03）+ maxClients=32 实测成立不动（D-11 两段式结论，one-way 门不触发）+ herdr 观测通道钉定（api snapshot 翻转链 + 版本钉定 0.8.100，D-06/D-09）"
  - "flagged_assumptions 终验登记（13-08 先例同构）：14-08 PC-13 unclassified 三边界形态检索结论——herdr 版本漂移韧性 / is_foreground >2 客户端与高频竞争 / ro 汇聚 herdr 侧广播语义，均保持 flagged-unverified（主面覆盖 + 同构论证口径）"
  - "Task 2 人工闸（D-04 blocking）user approved 记录：run-all 17/17 + load 八格 + pw 引用 14-09 已确认记录 + 残留核验四证据链"
affects: [v1.1 里程碑收口（/gsd:complete-milestone + release.sh 独立执行——14-CONTEXT deferred 既定，非本 phase 范围）]

# Actuals (#2632) — 与 plan estimate (55000 tokens) 同标尺（diff chars/4）。
# 口径注记：纯验证收口 plan（11-06/12-05/13-08 第四代同形态）——代码面零改动，
# diff 主体为 .planning 登记面（REQUIREMENTS/ROADMAP/PROJECT/STATE/WINDOWS + 本 SUMMARY）。
# 诚实注记（13-08 同款）：diff 标尺不覆盖执行期成本——六段式闸实跑（全量 -race
# 158s + UAT 矩阵 210.9s + load 双剖面 26s + diff 逐行审查 35 文件）与 14-06..14-11
# 各 plan 已各自登记的落地成本均不在 tokens 口径内。
actuals:
  tokens: 7700
  tasks: 3
  commits: 1

tech-stack:
  added: [] # 零新依赖红线终审（T-14-SC）：go.mod/go.sum + web 依赖清单五文件 + .github/（ci.yml）基点以来 0 行 diff
  patterns:
    - "收口闸六段式记录形态（11-06/12-05/13-08 先例第四代沿用）：命令 + 退出码 + PASS 计数 + diff 清单全量落档，日志归档 /tmp/14-12-closeout/ 十份"
    - "phase 基点口径第四代：03268f0^（Phase 14 首提交 docs(14): capture phase context 的父提交 = 96a188a Phase 13 PR #17 合并点）——branching_strategy=none 平直 main 下 merge-base 退化，三代先例同构"
    - "真删除行归类审查形态（本代新增机械化）：删除行剥离首尾空白后在新增行集合中配对（缩进归一化），未重现者即需人工逐行核对的语义改动面——565 行全归类零期望值漂移，配合抽查断言文案字面量（grep 新形态）双重锁定"
    - "Task 1-2 纯验证零提交形态（reversibility 声明兑现）：六段闸前四段 + 人工两段全部一次跑齐零修复，全部产出（SUMMARY/三档登记/STATE/WINDOWS）单 docs commit 收口"

key-files:
  created:
    - .planning/phases/14-herdr-uat/14-12-SUMMARY.md
  modified:
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md
    - .planning/PROJECT.md
    - .planning/STATE.md
    - .planning/WINDOWS.md

key-decisions:
  - "phase 基点 = 03268f0^：Phase 14 首提交的父提交（96a188a，Phase 13 PR #17 合并点）——branching_strategy=none 平直 main 下 merge-base 退化为 HEAD 自身，11-06/12-05/13-08 三代先例同构"
  - "Task 1-2 零提交收口形态：纯验证段一次跑齐全绿零修复（预期正常态），全部登记面归 Task 3 单 docs commit——与 13-08 的「段① 存量归一 style commit + docs commit」双提交形态差异源于本 phase 零代码面改动（无 gofmt 存量、无修复项）"
  - "diff 白名单 minrepro 补充项三轴裁决（WINDOWS #46，#33 先例同构）：web/uat/minrepro-p14.mjs + web/uat/pw/minrepro-win.mjs 不在任何 plan files_modified 但为 14-09 Track2 偏差副产（SUMMARY key-files + 偏差 #2 完整登记在案）——plan 相邻授权 / SUMMARY 声明在案 / 零产品代码与零断言影响，纳入白名单非回归"
  - "Task 2 人工闸 blocking 形态兑现：checkpoint 返回后用户 approved（2026-09-07），pw 段按 plan 授权引用 14-09 Task 2 已确认记录（两轮 4/4 + 截图六帧——PC-13 浏览器面以此收口）而非重跑"

requirements-completed: [PC-12, PC-13]

duration: 9h20min
completed: 2026-09-08
status: complete
---

# Phase 14 Plan 12: 收口闸（六段式验证总账 + UAT 矩阵人工闸 + diff 白名单终审 + PC-12/PC-13 勾选）Summary

**Phase 14 零回归收口闸六段式一次跑齐全绿：静态面三命令零输出 + 全量 -race 五包 2m37.986s（mode= 子测 216/216 与 14-06 审计一致）+ darwin 双编译闸四命令零错 + dist byte-identical + UAT 矩阵 17/17 零修改重跑基线逐脚本一致（人工闸 user approved）+ diff 白名单终审 35 文件零外改动（565 真删除行全归类、红线四查全零）；PC-12/PC-13 勾选收口——v1.1 里程碑 15/15 需求全闭合，Key Decisions 三行登记 + flagged_assumptions 终验落档。**

## Performance

- **Duration:** 9h20min（2026-09-07 22:54 起——Task 1-2 与人工闸当日完成，Task 3 次晨续跑；纯闸执行 ~1h，其余为 checkpoint 等待）
- **Tasks:** 3（Task 1 四段自动化闸 + Task 2 人工闸 checkpoint:human-verify + Task 3 diff 终审与登记——11-06/12-05/13-08 同形态）
- **代码面改动：** 零（纯验证收口 + 登记性编辑；无 gofmt 存量、无修复项——与 13-08 的 style commit 差异源于本 phase 零代码面改动）

## Task Commits

1. **Task 1: 收口闸静态面 + 全量 -race 双模式 + darwin 双编译闸 + dist byte-identical**
   - 纯验证零文件改动零提交（reversibility 声明「正常态零文件改动」兑现——四段全绿无修复）
   - 执行记录与日志落档 /tmp/14-12-closeout/（gofmt/vet/vet-load/race-full/darwin-build-amd64/darwin-build-arm64/darwin-vet-amd64/darwin-vet-arm64/web-build 十份）
2. **Task 2: UAT 矩阵人工收口闸（checkpoint:human-verify，gate=blocking）**
   - 纯验证零提交；checkpoint 返回后用户 approved（2026-09-07），结果记录见本文「Task 2 人工闸结果记录」节
3. **Task 3: diff 白名单终审 + PC-12/PC-13 勾选 + Key Decisions 登记 + SUMMARY 收口**
   - 并入文末最终 docs commit（REQUIREMENTS/ROADMAP/PROJECT/STATE/WINDOWS + 本 SUMMARY）

## Verification（六段式全量证据落档——全部命令本轮实跑，日志 /tmp/14-12-closeout/ 在场）

### 段① 静态面

| 命令 | 结果 |
|------|------|
| `$(go env GOROOT)/bin/gofmt -l .`（go1.26.3 linux/amd64） | 零输出 PASS（gofmt.log 0 字节——13-08 段① 归一后无新增存量） |
| `go vet ./...` | 零输出 exit 0 PASS（vet.log 0 字节） |
| `go vet -tags=load ./...` | 零输出 exit 0 PASS（vet-load.log 0 字节——14-07 load 面扩展的 vet 编译面） |

### 段② 全量 -race 双模式 + mode= 子测试计数对账

`time go test -race -count=1 ./...` exit 0，real **2m37.986s**：

```
ok  github.com/sworda/wesh/cmd/wesh         1.333s
ok  github.com/sworda/wesh/internal/proto   1.016s
ok  github.com/sworda/wesh/internal/pty     2.656s
ok  github.com/sworda/wesh/internal/server  157.028s
ok  github.com/sworda/wesh/web              1.011s
```

**双模式覆盖执行证据（D-01 验收形态）**：race-full.log 中 `=== RUN.*mode=` 计 **216**、`--- PASS:.*mode=` 计 **216**——与 14-06 收口核对审计（18 文件 81 测双跑、`-race -v` 全包 mode= 子测 216 PASS）**精确一致**；时长对照 A1 估算 3-4min/leg 量级登记实测 158s（server 包独占，双跑增量在预算内——14-01 CI 时长知悉项兑现）。

### 段③ darwin 双编译闸

| 命令 | 结果 |
|------|------|
| `GOOS=darwin GOARCH=amd64 go build ./...` | exit 0 PASS（darwin-build-amd64.log 0 字节） |
| `GOOS=darwin GOARCH=arm64 go build ./...` | exit 0 PASS（darwin-build-arm64.log 0 字节） |
| `GOOS=darwin GOARCH=amd64 go vet ./...` | 零输出 exit 0 PASS（darwin-vet-amd64.log 0 字节） |
| `GOOS=darwin GOARCH=arm64 go vet ./...` | 零输出 exit 0 PASS（darwin-vet-arm64.log 0 字节——13-06 抓漏先例加验形态） |

（darwin 运行行为测试由 CI macOS leg 承担，本机编译闸——三代先例口径。）

### 段④ web 构建

| 项 | 结果 |
|----|------|
| `pnpm -C web build`（time 前缀） | exit 0（web-build.log） |
| dist byte-identity | md5 `d5c25e27…` 复建前后一致（与 13-08 记录同值——本 phase 零前端改动，dist 差异即异常）；`git diff --stat web/dist` 零输出 → **零差异 PASS** |

### 段⑤ UAT 矩阵人工闸（Task 2——D-04 人工闸，user approved 2026-09-07）

`time go build -o /tmp/wesh-uat/wesh ./cmd/wesh` 后 `time node web/uat/run-all.mjs`——exit 0，**17/17 全绿**，runner 210.9s / wall 3m30.9s（2026-09-07 22:59-23:03，run-all.log 在场）：

| 脚本 | exit | PASS/SKIP | 基线对照（13-08 基线 = 14-08/14-10 记录） |
|------|------|-----------|------------------------------------------|
| phase02 | 0 | 12/12 | 12 ✓ |
| phase03 | 0 | 18/18 | 18 ✓ |
| phase04 | 0 | 10/10 | 10 ✓ |
| phase05 | 0 | 28/28 + 1 skipped 豁免 | 28 ✓ |
| phase05-dims | 0 | DIMS PASS（D6H-1 等价锁 + D6H-2 负对照） | ✓ |
| phase06 | 0 | 23/23 + 1 skipped 豁免 | 23 ✓ |
| phase07 | 0 | 34/34 + 1 skipped 豁免 | 34 ✓ |
| phase08 | 0 | 21/21 | 21 ✓ |
| phase09 | 0 | 18/18 | 18 ✓ |
| phase04-dom | 0 | 37/37 | 37 ✓ |
| phase05-dom | 0 | 19/19 | 19 ✓ |
| phase06-dom | 0 | 40/40 + 2 skipped 豁免 | 40+2skip ✓ |
| phase12-dom | 0 | 17/17 | 17 ✓（CR-01 后置口径） |
| phase11 | 0 | 21/21 + 1 skipped 豁免 | 21 ✓ |
| phase12 | 0 | 20/20 | 20 ✓ |
| phase13 | 0 | 29/29 | 29 ✓ |
| phase14 | 0 | 18/18 | 18 ✓（14-08 两轮基线；SEC 输出自净 details=17 命中=false） |

- **6 行 skipped 全部带 reason 且属平台豁免类**（CODEBUDDY.md §5）：phase05 S7（像素层）/ phase06 S7（真实断网栈）/ phase07 S8c（真实弹浏览器）/ phase06-dom D9+D12b（OS 断网栈/真实 AT 栈）/ phase11 S4b（OS 网卡栈时序）——与 13-08/14-10 同六行，零跳过测试类项（prohibitions PC-13 零跳过红线兑现）
- **load 双剖面八格**：`go test -tags=load -count=1 -timeout=30m -run 'TestLoadPerClient' ./internal/server/ -v` PASS **25.967s**——LOADDATA 八行与 14-07 基线一致：spawn_total==N（1/4/16/32）、kicks=0 全格、gor 差值精确 5N+1（+6/+21/+81/+161）、fd 差值精确 4N（+4/+16/+64/+128）、mem_delta 79,368B（N=1）→ 1,971,608B（N=32）≪ N×768KiB 账面、rss_per_proc ≤ 4.4MB、rss_sum 137.8MB ≤ 160MB 账面（32 shell 驻留推算）
- **pw 段（Windows 侧）**：按 plan Task 2 授权引用 14-09 Task 2 用户确认记录——两轮 4/4（T0/T1/T2/CL）+ 截图六帧人工复核，PC-13 浏览器面以此收口（14-09-SUMMARY 落档；双机拓扑 pw 永不进 Linux 矩阵）
- **残留核验**：`pgrep wesh` 零命中；`herdr session list` 零 wesh-uat-* 残留；用户 default 会话全程零触达——14-10 Rule 3 清障后清洁基线起跑兑现

### 段⑥ 期望值逐字未动 diff 白名单终审（D-02 验收闸）

**phase 基点**：`branching_strategy: none`（平直 main）下 merge-base 退化为 HEAD 自身——以 Phase 14 首提交 **03268f0**（`docs(14): capture phase context`）的父提交 **03268f0^**（= 96a188a，Phase 13 PR #17 合并点）为等价基点（11-06/12-05/13-08 三代先例同构）。plan verify 命令锚点实测解析至同一点。

`git diff 03268f0^..HEAD --name-status -- . ':(exclude).planning' ':(exclude)web/dist'` → **35 文件（30 M + 5 A）8284+/4524-**，与十一 plan 声明集合**逐文件吻合，白名单外零改动**：

- **internal/server 27 件恰= 各 plan files_modified 并集**：26 个既有测试文件 M（14-01 七件 + 14-02 两件 + 14-03 三件 + 14-04 一件 + 14-05 七件 + 14-06 六件（含计划内 CONTEXT 外的六个测试文件——逐一对号）+ 14-07 一件，精确 26 无超限）+ harness_test.go A（14-01 小族本体）
- **文档 3 件**：README.md / docs/ARCHITECTURE.md / docs/CONFIGURATION.md（14-11 三件套）
- **web/uat 5 件 A**：phase14.mjs（14-08）/ run-all.mjs（14-10）/ pw/phase14-pw.mjs（14-09）三件 plan 声明 + **minrepro 双件套**（minrepro-p14.mjs + pw/minrepro-win.mjs——14-09 Track2 偏差副产，14-09 SUMMARY key-files + 偏差 #2 完整登记；三轴裁决（plan 相邻授权/SUMMARY 声明在案/零产品代码零断言影响）纳入白名单，WINDOWS #46 登记——#33 export_test.go 先例同构，非回归）
- **既有 16 UAT 脚本零修改实证**：web/uat diff 仅 5 A 零 M——phase02-13 全部脚本 phase 以来零触达（零回归双证据的 UAT 侧本体）

**真删除行归类（机械化 + 逐行人工核对）**：删除行剥离缩进后在新增行集合配对，未重现者（= 语义改动面）共 **565 行 / 19 文件**，全量归类无一断言期望值漂移：

| 类别 | 典型文件 | 说明 |
|------|----------|------|
| 装配点收编改名 | 全部 19 文件 | startTestServerWith/startTestServer/startTrackedServerWith/startShutdownServerWith/startBasePathServer 调用点 → newTestServer/newSessTestServer/newTrackedTestServer 小族（D-01） |
| helper 定义删除 | e2e/resize_arb/shutdown/basepath/sharetoken | startTestServer（-6 行，#45）/ startResizeServer（#43 收编）/ startShutdownServerWith / startBasePathServer / startShareServer 旧签名——14-02/04/05/06 收编闭环 |
| Options 字面量 → mutate 闭包 | auth_e2e/basepath/customindex/multi 等 | Pattern 9（14-05 裁决：零值/Writable:false 显式回写保期望值零改写） |
| Phase 13 per-client 测试 → 双模式列重构 | emptyexit/shutdown/metrics | TestEmptyExitPerClient*/TestPerClientShutdown*/TestMetricsPerClient 由单模式重排为 mode= 列（14-03 归一裁决——per-client 列=既有断言，shared 列=原 shared 断言，注释头随之改写） |
| loadDrain 滚动窗修正 | load_test | 14-07 Rule 1（帧级重置→跨帧滚动窗，STATE 已登记） |
| 断言行变量名形态变化 | auth_e2e/e2e/handshake/multi | mode→wmode/mode2/mode3 等——**文案字面量逐字保留**（抽查实证：ticket 绑定=writable 装配 / 零值 Options=默认只读 / writable=false / re-attach welcome mode / owner 默认策略单客户端立 owner / wmB 60×43（WINDOWS #11 语义）/ fan-out ×2 放大比 / draining 断言全部在场） |
| #44 分叉表消息参数化 | handshake | TestReadOnlyAllowsResize second stty 断言：期望值 "44"/"111" 作为 shared 列数据保留 + per-client 列 "50"/"120" 直通真值 + D-09 注语文本保留在 mode map——WINDOWS #44 登记的分叉表形态，非放宽 |
| 注释改写/搬移 | 全部 | 文件头 helper 复用注释、13-03 增量段注、蓝本归属注等随结构改写 |

**红线四查**：

- **断言放宽形态扫描**（prohibition PC-12/PC-13 双面）：全代码 diff grep「两模式都接受 / both modes / either mode」→ 2 命中均为「非『两模式都接受』的削弱形态」红线注释（PITFALLS P11 注记），**零实际放宽** ✓
- **零跳过红线**：26 改造文件新增 `t.Skip` 3 处全为 darwin 平台豁免且带 reason（TestLoadPerClientResident Linux-only /proc 口径 + TestGlobalCredit darwin creditBlocked 时序 + multi_test 注释行「否决 t.Skip」文本）——模式面零跳过 ✓
- **零新依赖红线**（T-14-SC）：`git diff 03268f0^..HEAD -- go.mod go.sum .github/ web/package.json web/uat/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml` = **0 行**（plan verify 命令实测 0）✓
- **web/src 与 dist 零 diff**：`git diff 03268f0^..HEAD -- web/src` = 0 行；dist 段④ byte-identical + 基点 diff 0 行 ✓（本 phase 零前端改动红线兑现）

**WINDOWS 偏差登记对账**：Phase 14 四条（#42 metrics_test series 口径 / #43 harness_test 枚举缺口 / #44 handshake 断言分叉 / #45 e2e_test startTestServer 删除）全部在白名单文件内且为对应 plan SUMMARY 声明项；本 plan 新增 #46（minrepro 补充项）——diff 审查与登记双向一致，无未登记改动。

**三维归类收口核对复核**（must_haves truth 3）：14-06 SUMMARY 34 文件清单复核成立——① 18 文件 81 测双跑（216 mode= 子测本收口闸复跑精确一致）+ ② perclient_test 36 测单模式 + ③ load 8 测双剖面 + ④ 纯白盒/纯函数 10 文件 + ⑤ events 9/log 1 单跑（D-02 第 4/5 则登记）+ ⑥ export_test/harness_test 零测试桶；蓝本 PITFALLS :378-399 十七行全命中或 D-02 第 1-6 则偏差登记，**无静默漏网**。

## 需求勾选证据链映射（v1.1 15/15 全闭合收口）

| 需求 | Go 双模式矩阵（-race 216 子测） | 协议层 UAT | Windows pw | 文档/标定 | diff 审查 |
|------|--------------------------------|------------|------------|-----------|-----------|
| **PC-12**（模式语义文档 + GoTTY 误记修正） | —（文档需求） | — | — | README「会话模式」节（:96 段纯插入 + 资源义务段 + 标定表 30 项程序化核对）/ ARCHITECTURE 七分支点 + goroutine 拓扑 mermaid + :7 GoTTY 误记修正（实为 per-connection spawn）/ CONFIGURATION max-clients 兼任进程上限行 + D-11 三档建议值表（默认 32 明示/低配 8/个人多端 4，依据锚定 14-07 LOADDATA） | 三文档段全在 14-11 files_modified 白名单内；README :96 段字节级不动纯插入（shared 表述零削弱以 diff 纯新增自证） |
| **PC-13**（herdr 多客户端互不干扰） | resize 隔离/ro 门控/owner 未装配等断言双跑（14-01..06 分叉表，shared 列期望值逐字） | phase14.mjs 两轮 18/18：S1 driving 三通道（area 翻转链四步 + 流层 254B/55B≪97KB + wesh 层双 pid/双 Welcome/流几何 120vs40）+ S2 ro 汇聚（ticket 全链 + pane read 门控实证 + rw 对照） | phase14-pw 两轮 4/4（T0 进程自证/T1 attach 不压缩/T2 resize 不压缩/CL 零残留）+ 截图六帧人工复核（14-09 Task 2 user approved 记录，本 plan 授权引用） | — | 白名单内零断言漂移；herdr 观测通道钉定 0.8.100 + 命名会话零污染 |

勾选承载：REQUIREMENTS.md 两条 `[x]` + Traceability 两行 Complete + Last updated 行（plan verify grep 计数 2 实测 PASS）；ROADMAP Phase 14 勾选 + 12/12 + Progress 表 Complete 2026-09-08。

## D-11 标定结论（两段式收口登记）

1. **判定证真**（14-07）：maxClients=32 默认值经负载矩阵实测成立——32 会话驻留 wesh 侧 Alloc 增量 1.97MB ≈ 24MiB 账面 9%、子进程 VmRSS 合计 137.8MB < 160MB 账面推算、fd 差值精确 4N、gor 5N+1；洪水格 kicks=0、每会话 34.9MB 收流完整（本收口闸段⑤ 八格复跑逐项一致）。
2. **结论**：**默认值不动（零公开契约变更，one-way 确认门不触发）**；文档承载 = README 资源义务段 + 三档建议值表（14-11 落地，依据列锚定 LOADDATA 实测值：每会话 bash ~3.7MiB + wesh 侧 ~61KiB）。

## flagged_assumptions 终验登记（13-08 先例同构——检索结论登记而非静默放行）

**PC-13（14-08 flagged）**——unclassified 三边界形态终验检索结论：

- **herdr 版本漂移对断言韧性的影响**：D-06 结构化观测通道（api snapshot layouts[0].area + pane read --source visible）+ D-09 版本钉定 0.8.100 + 启动 herdr --version 打进日志 + 失败先核版本纪律——断言不依赖渲染格式（否决终端嗅探），API 形态漂移将响亮失败非静默假绿。执行期（14-08/09/12 全链）未发现 0.8.100 内新边界。**保持 flagged-unverified**（0.8.100 后版本 API 漂移无构造性验证——外部依赖版本面，回写条件 = herdr 升级时按版本纪律先核）。
- **is_foreground 仲裁在 >2 客户端或高频活动竞争下的行为面**：S1 以双客户端（桌面 120x40 + 移动 40x12）覆盖翻转链四步 + resize 翻新几何；>2 客户端与高频竞争未构造。论证：wesh 层义务 = per-client winsize 隔离与直通（N=2 已锁定 + resize 隔离断言双跑承载，客户端数量不改变该保证）；herdr 侧 area 仲裁时序为 herdr 自身行为面（wesh 不断言其内部仲裁，正如 D-05 否决 tmux 对照）；pw 层结构行幸存前缀判别面提供渲染层稳定性证据。**保持 flagged-unverified**。
- **ro 汇聚下 herdr 侧自身的输入广播语义**：S2 断言 wesh 门控面（ro INPUT 丢弃——pane read 逐字不变 + RO_MARKER 缺席 + rw 对照可见）；herdr 侧自身多 pane 广播语义未断言——超出 wesh 义务面（wesh 只保证 ro INPUT 不达 PTY，herdr 内部行为归 herdr）。**保持 flagged-unverified**。
- **结论**：执行期未发现上述三面之外的未覆盖边界；外部依赖版本面/超义务面保持 **flagged-unverified**（主面覆盖 + 同构论证；构造性验证成本与外部依赖不可控性不匹配——13-08「µs 级时序窗口类」同款风险接受口径）。

## Task 2 人工闸结果记录（checkpoint:human-verify，gate=blocking）

- **闸点**：Task 1 四段全绿后 checkpoint 返回（UAT 矩阵 + load 双剖面 + pw 确认 + 残留核验四段人工闸，D-04）
- **用户裁决**：**approved（2026-09-07）**——四证据链全部满足：
  1. run-all.mjs 全矩阵零修改重跑 17/17 绿 exit 0（210.9s，逐脚本计数与 13-08 基线及 14-08/14-10 记录一致；6 skipped 全 reason 标注平台豁免同六行）
  2. load 双剖面八格全绿 25.967s（LOADDATA 不变量与 14-07 一致）
  3. pw 段按 plan 授权引用 14-09 Task 2 用户确认记录（两轮 4/4 + 截图六帧——PC-13 浏览器面）
  4. 残留核验：pgrep wesh 零命中 + herdr session list 零 wesh-uat-* 残留 + default 会话零触达
- **续跑**：Task 3 由 continuation agent 承接（git log 核验 8f36a2f 14-11 收口在场、Tasks 1-2 零提交为预期态）

## Deviations from Plan

None —— 收口闸六段式一次跑齐全绿（Task 1 四段零修复、Task 2 人工闸一次通过、Task 3 登记零偏差）；13-08 的段① gofmt 存量归一偏差形态在本 phase 不存在（13-08 归一后零新增存量）。

### Plan 措辞与实证的微小出入（不构成偏差）

- 白名单「26 个既有测试文件改造/扩展上限」：实测恰 26 M（上限兑现恰等，含 load_test.go 扩展与 7 个纯判定注释行文件 auth/origin/proxy/throttle/tickets/tls +5~8 行）
- 「3 UAT 脚本新增」：实测 5 A——3 plan 声明 + minrepro 双件套（14-09 Track2 偏差副产，SUMMARY 在案 + WINDOWS #46 补充项登记，见 key-decisions）
- 「perclient_test.go 姊妹变体新增」：per-client tracked 姊妹变体（newTracked 等）落于 harness_test.go 小族本体（14-01 D-01 四形态），perclient_test.go 本体 +64/-0 纯新增零删除

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-14-25 (Tampering: 收口以放宽断言/跳过失败项换绿) | mitigate | diff 白名单逐行终审（565 真删除行全归类 + 断言文案抽查全保留）+ 放宽形态 grep 零实际命中（2 命中均为红线注释）+ 零跳过红线（skipped 仅 6 平台豁免带 reason 与 13-08 同六行；新增 t.Skip 3 处全 darwin 豁免带 reason）+ Task 2 人工闸 blocking 用户 approved |
| T-14-26 (Tampering: 白名单外漂移混入) | mitigate | Task 3 命令化零 diff 断言（go.mod/go.sum/.github/web 依赖清单五文件 = 0 行，plan verify 实测）+ 35 文件逐文件白名单核对（27 测试件恰= plan 并集 + 3 文档 + 5 UAT 含 2 已登记副产）+ dist byte-identical + web/src 零 diff |
| T-14-SC (Tampering: 依赖面供应链) | mitigate | 零新依赖红线终验：基点以来 go.mod/go.sum + web 依赖清单 0 行 diff（段⑥）；全 phase 无装包任务，legitimacy 门未触发 |

## Known Stubs

None —— 本 plan 为纯验证收口（11-06/12-05/13-08 第四代同形态）零代码改动；Phase 14 各 plan 落地件无桩（14-08 phase14.mjs / 14-09 phase14-pw.mjs / 14-10 run-all.mjs 均为真断言载具，14-11 三文档段数据全部锚定 14-07 LOADDATA 实测值——逐 plan SUMMARY Known Stubs 复核零遗留）。

## Issues Encountered

- 无阻塞问题。Task 1-2 一次跑齐全绿零返工（对照 13-08 的 gofmt 存量归一——本 phase 无存量）
- `roadmap update-plan-progress` 在 SUMMARY 落盘前按 PLAN/SUMMARY 计数回写 11/12（工具口径正确）——SUMMARY 写入后复跑闭合 12/12 Complete
- state.advance-plan 正确处理 last-plan 边缘情况（37/37 里程碑末 plan——status → verifying，Phase 14 转入验证态）

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Phase 14 收口完成**：PC-12/PC-13 勾选 + 零回归双证据闭合 + diff 白名单终审零外改动 + flagged_assumptions 终验落档——**v1.1 里程碑 15/15 需求全闭合**（PC-01→10；PC-02/03/04→11；PC-05/06/07/10/11→12；PC-08/09/SEC-09/OPS-12→13；PC-12/13→14）
- **v1.1 里程碑收口就绪**：/gsd:complete-milestone + release.sh 独立执行（14-CONTEXT deferred 既定——v1.1.0 发布闸非本 phase 范围）；零新依赖/ci.yml 零改动/dist byte-identical 三红线终态确认，发布链前置条件就绪
- **威胁登记闭合**：T-14-25/T-14-26/T-14-SC（收口闸三威胁）→ mitigate 全兑现；Phase 14 全部威胁条目经各 plan mitigation + 本收口闸终验闭合
- **herdr 上游（超出本里程碑）**：浏览器输入链停摆海森bug minrepro 双件套已交付（14-09）——载具级裁决 + 服务端无罪反向证据链在案

## Self-Check: PASSED

- 文件存在性：REQUIREMENTS.md（PC-12/PC-13 [x] grep 计数 2 + Traceability 两行 Complete + Last updated 2026-09-08 行）/ ROADMAP.md（14-12 [x] + Plans 12/12 + Progress 表 Complete 2026-09-08）/ PROJECT.md（Key Decisions 三行 + 尾注 2026-09-08）/ WINDOWS.md（#46 补充项）/ STATE.md（status verifying + 决策五行 + Phase 14 P12 指标行）——全部在场
- 提交存在：docs(14-12) 收口提交（本 SUMMARY 同批）——git log 确认
- 证据锚点抽查：plan Task 3 verify 命令实测 PASS（红线 0 行 + PC-12/13 勾选计数 2）；/tmp/14-12-closeout/ 十份日志在场（gofmt/vet/vet-load/darwin×4/web-build 零字节或构建输出、race-full 216/216、run-all 17/17 汇总、load-dual 八格 LOADDATA）与本文表格逐项一致；六段式验证输出全部本轮实跑 exit 0

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-08*
