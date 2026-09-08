---
phase: 14-herdr-uat
plan: 10
subsystem: testing
tags: [uat, matrix, runner, zero-regression, sc2, herdr, d-04]

requires:
  - phase: 14-herdr-uat/14-08
    provides: phase14.mjs（第 17 项矩阵成员，18/18 两轮基线 + exit code 门禁 + 清理零残留）
  - phase: 13-resource-defense/13-08
    provides: 16 项既有脚本人工逐跑基线计数表（SUMMARY :172-193——零回归核对基准）
provides:
  - web/uat/run-all.mjs——17 项 UAT 一键矩阵 runner（串行 spawn 聚合 + 10min 超时护栏 + argv 过滤子集 + 汇总表 + exit code 门禁）
  - D-04 零修改重跑形式化执行入口（phase02-09 既有协议 UAT 默认 shared 零修改重跑全过的单命令证据形态）
  - D-13 SC2 矩阵证据（phase11/12/13 三脚本经 runner 重跑全绿且计数与基线一致）
affects: [14-herdr-uat (14-12 收口闸直接复用)]

actuals:
  tokens: 2115   # chars/4 over realized diff（8461 chars / 4——run-all.mjs 唯一代码产物）——远低于 estimate 35000（16 项既有脚本自带门禁使 runner 零断言面收敛）
  tasks: 2
  commits: 2

tech-stack:
  added: []   # 零新依赖红线（T-14-SC）——Node 原生 child_process/fs/path，package.json/lockfile 零 diff
  patterns:
    - "跨脚本门禁聚合：串行 spawn('node',[script,WESH]) 只聚合 exit code，断言语义归各子脚本（phase13.mjs 收口段提升到脚本间层）"
    - "超时护栏进程组收割：spawn detached 成组 + 超时 kill(-pgid) SIGTERM→2s 宽限→SIGKILL——挂死脚本的未自清 wesh 子进程一并终结（T-14-21）"
    - "防静默跑空：过滤子集未知名 exit 2 + 脚本名误传二进制位守卫（自动构建产物错位拦截）"

key-files:
  created:
    - web/uat/run-all.mjs
  modified: []

key-decisions:
  - "runner 执行序：协议基线（phase02-09）→ jsdom（04/05/06/12-dom）→ per-client 协议（11/12/13）→ herdr（phase14 末位）——must_haves「协议 12 + jsdom 4」计数口径与 13-08 逐跑序兼容的场景序落地"
  - "输出形态择一取管道转发前缀化（[脚本名] 行前缀）——矩阵上下文可读性优先；红线保持：前缀仅脚本名，零 token/pid 打印面"
  - "二进制前置择一取「先构建后指引」：缺席自动 go build（cwd=仓库根），失败打印手动构建指引退出 1"

requirements-completed: []   # PC-13 已于 14-09 收口勾选（协议层证据本 plan 经 runner 复跑再证）

coverage:
  - id: D1
    description: "一键矩阵落地（D-04）：17 项枚举 + 串行 spawn + exit code 聚合 + 汇总表（脚本/结果/耗时 + 总计 + pw 独立入口注记）+ 全绿 0/任一失败 1 门禁"
    requirement: PC-13
    verification:
      - kind: e2e
        ref: "web/uat/run-all.mjs（node --check 零错 + phase04-dom 子集冒烟 exit 0 + 全矩阵 17/17 PASS exit 0）"
        status: pass
    human_judgment: false
  - id: D2
    description: "零回归双证据形式化：phase02-09 既有协议 UAT 默认 shared 零修改重跑全过——逐脚本计数与 13-08 基线表完全一致（12/18/10/28+1skip/DIMS/23+1skip/34+1skip/21/18）"
    requirement: PC-13
    verification:
      - kind: e2e
        ref: "全矩阵首跑（/tmp/runall-full.log 逐脚本结果行 vs 13-08-SUMMARY :172-193）"
        status: pass
    human_judgment: false
  - id: D3
    description: "SC2 三脚本重跑证据（D-13）：phase11/12/13 经本 runner 重跑全绿（21/21+1skip、20/20、29/29）且与基线逐项一致——协议层 per-client 全链六项矩阵证据成型"
    requirement: PC-13
    verification:
      - kind: e2e
        ref: "全矩阵首跑 phase11/12/13 三行（重跑形态 node web/uat/run-all.mjs '' phase11 phase12 phase13 亦可用）"
        status: pass
    human_judgment: false

---

# Phase 14 Plan 10: run-all.mjs 一键矩阵 runner Summary

**One-liner:** 17 项 UAT 一键矩阵 runner（串行 spawn 聚合 + 超时护栏 + 汇总表）——全矩阵首跑 17/17 全绿 213.4s，逐脚本计数与 13-08 基线完全一致，D-04 零修改重跑形式化与 D-13 SC2 证据成型。

## What Was Built

**web/uat/run-all.mjs**（150 行，零依赖 Node>=22）——D-04 落地：

- **SCRIPTS 17 项枚举**（与 web/uat/ ls 逐字核对）：协议 12（phase02/03/04/05/05-dims/06/07/08/09/11/12/13）+ jsdom 4（phase04-dom/05-dom/06-dom/12-dom）+ herdr 1（phase14 末位，含真实等待）；执行序 = 协议基线 → jsdom → per-client 协议 → herdr
- **串行 spawn 聚合**：`spawn('node', [script, WESH])` 逐脚本 await，stdout/stderr 管道转发 + `[脚本名]` 行前缀（半行缓冲 + close 冲刷）；runner 零断言零解析零 skip 开关——门禁语义归各子脚本既有 exit code 0/1
- **超时护栏（T-14-21）**：逐脚本 10min——detached 进程组 `kill(-pgid)` SIGTERM → 2s 宽限 → SIGKILL 兜底，转 FAIL 不阻断后续脚本
- **argv 过滤子集**：`node web/uat/run-all.mjs [wesh] phase11 phase12 ...`（含/不含 .mjs 均可，按矩阵序执行）；未知名 exit 2 防拼写错误静默跑空；脚本名误传二进制位守卫拦截；默认二进制占位 `''` 形态支持
- **二进制前置**：缺席自动 `go build -o <path> ./cmd/wesh`（cwd=仓库根），失败打印手动构建指引退出 1
- **汇总表**：逐脚本 PASS/FAIL/耗时行 + 总计行 + pw 独立入口注记（Windows 工作站 `node web/uat/pw/phase14-pw.mjs`——双机拓扑硬约束，OQ3 裁决不纳入矩阵）+ `process.exit(全绿 ? 0 : 1)` 门禁
- **UAT 红线继承（T-14-22）**：runner 自身不新增任何 token/pid 打印面（前缀仅为脚本名）；assertOutputClean 职责归各子脚本；.github/workflows/ 零 diff（D-04 不进 CI）

## Test/Evidence

### Task 1 静态核验 + 子集冒烟

- `node --check web/uat/run-all.mjs` 零错；SCRIPTS 程序化核数 count=17
- 五面 grep 可证：串行 spawn（:93）/ 超时护栏（TIMEOUT_MS :50）/ 汇总表 / exit code 门禁（:150）/ argv 过滤（FILTERS :64）；pw 注记 grep phase14-pw 命中（:149）
- 子集冒烟 `node web/uat/run-all.mjs /tmp/wesh-uat/wesh phase04-dom`：37/37 PASS，runner exit 0，汇总表含该脚本行

### Task 2 全矩阵首跑（herdr 0.8.100 built 2026-09-07T19:19:30+08:00；go build 0.716s / 11841577 字节）

`time node web/uat/run-all.mjs` → **17/17 PASS，exit 0，全程 213.4s（real 3m33.5s）**——对照 A6 量级预估（~3.5min + phase14 增量 1-2min）：总量吻合预估下沿，phase14 实测 5.6s 远快于 1-2min 外推（herdr 场景真实等待比预估短）。

逐脚本结果与 13-08 基线计数核对（全部一致）：

| # | 脚本 | 基线（13-08 :172-193 / 14-08） | 实测 | 耗时 | 一致 |
|---|------|------|------|------|------|
| 1 | phase02 | 12/12 | 12/12 | 24.4s | ✓ |
| 2 | phase03 | 18/18 | 18/18 | 10.8s | ✓ |
| 3 | phase04 | 10/10 | 10/10 | 2.3s | ✓ |
| 4 | phase05 | 28/28+1skip | 28/28+1skip（S7 像素层） | 17.5s | ✓ |
| 5 | phase05-dims | DIMS PASS | DIMS PASS（D6H-1 等价锁+D6H-2 负对照） | 0.8s | ✓ |
| 6 | phase06 | 23/23+1skip | 23/23+1skip（S7 断网栈） | 8.3s | ✓ |
| 7 | phase07 | 34/34+1skip | 34/34+1skip（S8c 弹浏览器） | 4.7s | ✓ |
| 8 | phase08 | 21/21 | 21/21 | 17.0s | ✓ |
| 9 | phase09 | 18/18 | 18/18 | 2.5s | ✓ |
| 10 | phase04-dom | 37/37 | 37/37 | 10.1s | ✓ |
| 11 | phase05-dom | 19/19 | 19/19 | 7.6s | ✓ |
| 12 | phase06-dom | 40/40+2skip | 40/40+2skip（D9/D12b） | 33.8s | ✓ |
| 13 | phase12-dom | 17/17 | 17/17 | 6.6s | ✓ |
| 14 | phase11 | 21/21+1skip | 21/21+1skip（S4b） | 12.0s | ✓ |
| 15 | phase12 | 20/20 | 20/20 | 26.5s | ✓ |
| 16 | phase13 | 29/29 | 29/29 | 23.1s | ✓ |
| 17 | phase14 | 18/18（14-08 两轮基线） | 18/18 | 5.6s | ✓ |

- **6 项 skipped 全部带 reason 且属平台豁免类**（CODEBUDDY.md §5 口径）：phase05 S7（像素层）/ phase06 S7（真实断网栈）/ phase07 S8c（真实弹浏览器）/ phase06-dom D9+D12b（OS 断网栈/真实 AT 栈）/ phase11 S4b（OS 网卡栈时序）——与 13-08 登记的 6 行逐字同源，零跳过测试类项
- **SC2 证据（D-13）**：phase11/12/13 三行全绿且计数与基线一致——「协议层 per-client 全链六项」矩阵证据成型（单脚本复跑形态 `node web/uat/run-all.mjs '' phase11 phase12 phase13` 可用）
- **零修改实证**：phase02-09 既有 9 协议脚本（v1.0 纪元产物）经 runner 默认 shared 零修改重跑全过——D-04 零回归双证据形式化（git 侧证据归 14-12 收口闸 diff 白名单）

### 残留核验（T-14-21 mitigation）

- 矩阵自身残留：**零**——phase14 S1j/S2g 双清理收口 PASS（herdr session list 无本会话残留行，default 会话全程零触达）；锚定 `pgrep -f '^/tmp/wesh-uat/wesh'` 对矩阵产物零命中
- 终态：herdr session list 仅存用户自有 default 会话；wesh 进程零残留（含清障后复验，见偏差①）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - 清障] 14-09 陈旧诊断残留清理（非本次矩阵泄漏）**
- **Found during:** Task 2 步 5 残留核验
- **Issue:** 残留核验命中 2 个 wesh 进程（session 名 wesh-uat-minrepro-825865/828062，19:31:04/19:31:50 启动）+ 2 个 herdr 会话（wesh-uat-minrepro-830799、wesh-uat-p14-pw-1788782983961）——经 ps 起始时间实证均早于本次矩阵运行（22:07）2.5h+，系 14-09 minrepro/pw 诊断运行的陈旧残留（minrepro 为诊断件非 UAT 矩阵项，清理语义弱于 UAT 脚本），阻塞本任务「零残留」验收面
- **Fix:** SIGTERM 两陈旧 wesh 进程（2s 后确认退场，无需 KILL 兜底）+ herdr session stop→delete 两陈旧会话（14-08 清理完整序列形态；default 用户会话零触达）；终态复验 herdr list 仅存 default、锚定 pgrep 零命中
- **Files modified:** 无（纯环境清障）
- **Commit:** 无独立提交（执行性清障，本节登记）

**2. [事实勘误-无影响] plan 文本「phase04-dom——不 spawn wesh 的最快面」与实测不符**
- **Found during:** Task 1 read_first 核对
- **Issue:** phase04-dom.mjs:41 实际 `spawn(WESH, args, ...)`（DOM 逻辑面之外也起真实实例）；子集冒烟选它依然合理（37 项最快面之一）
- **Fix:** 无需修复——Task 1 verify 链前置 `go build`，二进制在场，冒烟照常通过
- **Files modified:** 无

**3. [量级偏差-有利] phase14.mjs 实测 5.6s vs A6 预估 1-2min**
- **Issue:** RESEARCH §Pattern 5 的 phase14 增量 1-2min 为 ASSUMED 外推；实测 5.6s（herdr 场景真实等待比预估短）
- **Fix:** 无需动作——总量 213.4s 落在 ~3.5min+ 增量的预估区间内；14-12 收口闸时长预算更宽松

## Notes for 14-12 收口闸

- 执行入口：`time go build -o /tmp/wesh-uat/wesh ./cmd/wesh && time node web/uat/run-all.mjs`（零遗漏形式化）；SC2 单独复跑 `node web/uat/run-all.mjs '' phase11 phase12 phase13`
- 两段式第二段：Windows 工作站 `node web/uat/pw/phase14-pw.mjs`（汇总表注记在场）
- 10min/脚本超时护栏已覆盖挂死面；14-09 遗留的 TestMaxClients503 flake（deferred-items.md）不影响本矩阵（该 flake 属 Go 测 -run 过滤形态，UAT 脚本不触发）

## Self-Check: PASSED

- web/uat/run-all.mjs 存在（8461 字节，git HEAD 42a2041 创建）✓
- Task 1 提交 42a2041 在 git log ✓
- SUMMARY 本文件在 .planning/phases/14-herdr-uat/14-10-SUMMARY.md ✓
