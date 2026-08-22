---
phase: 05-multi-client
plan: 12
subsystem: testing
tags: [websocket, protocol, uat, jsdom, xterm-headless, resize-arbitration, multi-client, regression-lock]

# Dependency graph
requires:
  - phase: 05-multi-client
    provides: 05-10 Welcome 三通道恒携会话 cols/rows（attach/升格/运行期推送）+ 05-11 refit() 视口约束渲染（逐轴 min(fit, sessionDims)）
provides:
  - G-05-1 三层自动化回归锁：S10 协议面（carriage/推送/升格尺寸）+ D6 DOM 面（约束 rows/会话 cols 折行/升格解除）+ D6H 终端核心层（等价锁/负对照）
  - web/uat/phase05-dims.mjs（@xterm/headless 等价断言新脚本，probe10 探针机制转正）
  - README 协议表 'W' 行与 resize 行为节新形态描述；05-UAT.md 自动化说明段与 05-VALIDATION.md Nyquist 回填
  - phase05-dom.mjs/phase05-flood-driver.mjs 补录入库（05-09 产物首次进 git）
affects: [phase-06 协议文档, verify-work 对 G-05-1 翻牌对账（gap_ids 链接）, /gsd:validate-phase §6]

# Actuals (#2632) — chars/4 over the realized diff（estimate 同尺）
# 口径注：仅计本 plan 撰写的新增 churn（S10 块 3856 + D6 块 3162 + phase05-dims.mjs 全新 7270
# + 三文档 3940 = 18228 chars /4 ≈ 4557）；Task 2 提交内 phase05-dom.mjs 基座与
# phase05-flood-driver.mjs（05-09 产物补录，~49KB）非本 plan 撰写不计（05-11 dist 排除先例同款口径）。
actuals:
  tokens: 4557
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "UAT 断言区分度三层纪律：数值不相等证明语义在场（S10a 旁观者收 40x10 ≠ 自身 120x40）/ 无约束形态下断言必败（D6b 80A 单行 vs 40+40 折行）/ 负对照不全等证明非恒真（D6H-2 同流喂 120 列分叉）"
    - "headless 等价锁精确形态：120x40 建起再 resize(40,10) = 前端 refit() 生产路径的 1:1 复刻（xterm 按 fit 创建、WELCOME 后 resize 到会话尺寸），与窄端原生 40x10 对拍逐屏全等"
    - "buffer 快照比对：translateToString(true) 去尾空行 join——折行点分叉可见且不受终端 rows 高度差影响"

key-files:
  created:
    - web/uat/phase05-dims.mjs
  modified:
    - web/uat/phase05.mjs
    - web/uat/phase05-dom.mjs
    - README.md
    - .planning/phases/05-multi-client/05-UAT.md
    - .planning/phases/05-multi-client/05-VALIDATION.md
    - .planning/phases/05-multi-client/deferred-items.md

key-decisions:
  - "[Phase 05-12]: D6H-1 等价锁取「120x40 建起再 resize(40,10)」形态——精确复刻前端 refit() 生产路径（WELCOME 后 term.resize 到会话尺寸），比双 40x10 原生对拍更强的约束渲染等价断言"
  - "[Phase 05-12]: D6H-2 负对照以 buffer 快照（translateToString(true) 去尾空行 join）比对——折行点分叉驱动的不全等，不受 rows 高度差影响（phase04-t1-width U6 对照组先例的区分度纪律）"
  - "[Phase 05-12]: S10c 取最后一帧 WELCOME 解码——容忍升格 Welcome 与紧随 recalcNow 推送双帧相继到达（同值幂等）；S10b 50ms 防抖经轮询吸纳无显式等待（S9b stty 同款形态）"
  - "[Phase 05-12]: probe10.mjs 探针血缘处理——探针为 G-05-1 诊断期一次性用具从未入库，按 plan 机制描述重建为 phase05-dims.mjs，文件头注释登记血缘并与 05-UAT.md G-05-1 root_cause 互链"

patterns-established:
  - "G-05-1 类修复的最强可验证形态模板：协议（phaseNN.mjs）→ DOM（phaseNN-dom.mjs）→ headless 核心（phaseNN-dims.mjs）三层断言 + 复跑链三条命令"
  - "探针转正纪律：一次性诊断探针机制按 plan 描述重建为门禁断言脚本，文件头注释登记血缘出处"

requirements-completed: [MULTI-01, MULTI-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "S10 会话尺寸下发协议断言：S10a 异尺寸双端 attach Welcome 携会话尺寸（旁观者 40x10 ≠ 自身 120x40）；S10b owner RESIZE 经防抖后全端收 'W' 推送（旁观者 ro/60x15、上报者自身 rw/60x15 同收）；S10c 升格 Welcome(mode=rw) 携 cand.dims 120x40"
    requirement: MULTI-01
    verification:
      - kind: e2e
        ref: "node web/uat/phase05.mjs ./wesh（28/28 pass + 1 skipped，S10a/b/c 全 PASS，S1-S9 零回归）"
        status: pass
    human_judgment: false
  - id: D2
    description: "D6 视口约束渲染 DOM 断言：D6a 宽端 ro 旁观 .xterm-rows 行数约束到会话 rows=10（无约束为 24）；D6b 80 个 A 在会话 cols=40 处折行为相邻两 div 各 40 字符（叠写回归 DOM 层等价物）；D6c 升格后约束解除回窗口 24 行"
    requirement: MULTI-01
    verification:
      - kind: e2e
        ref: "node web/uat/phase05-dom.mjs ./wesh（19/19 pass，D6a/b/c 全 PASS，D1-D5 零回归——D1b ro 提示恰一次在推送形态下保持、D1d 零 RESIZE 上行顺带覆盖）"
        status: pass
    human_judgment: false
  - id: D3
    description: "phase05-dims.mjs 终端核心层：D6H-1 等价锁（120x40→resize(40,10) 约束渲染 ≡ 40x10 窄端原生逐屏严格一致）+ D6H-2 负对照（同字节流喂 120 列换行点分叉不全等，G-05-1 叠写机理复现，证明断言区分度）"
    requirement: MULTI-01
    verification:
      - kind: e2e
        ref: "node web/uat/phase05-dims.mjs ./wesh（3/3 pass 含 D6H-0 前置齐读，退出码 0）"
        status: pass
    human_judgment: false
  - id: D4
    description: "既有断言零回归：phase02/03/04 全套 + phase05.mjs S1b（双端逐字节一致）/S9b（升格 PTY 跟随）+ go test ./... -race 全量"
    requirement: MULTI-04
    verification:
      - kind: e2e
        ref: "七脚本连跑全过（phase02 12/12、phase03 18/18、phase04 10/10、phase05 28/28+1skip、phase05-dom 19/19、phase05-dims 3/3、phase04-t1-width 5/5）+ go test -race 全包 ok（39.8s）"
        status: pass
    human_judgment: false
  - id: D5
    description: "文档同步：README 协议表 'W' 行恒在 cols/rows + 正文同步 + resize 节新形态（约束视口渲染）；05-UAT.md 自动化说明段（复跑命令/覆盖计数/S10-D6-D6H 三句）；05-VALIDATION.md 三行映射 + Full suite command + Estimated runtime 实测回填"
    requirement: MULTI-01
    verification:
      - kind: other
        ref: "核读落盘 + 全量回归实测值回填（计数 28/28、19/19、3/3 均为本 plan 复跑实际输出）"
        status: pass
    human_judgment: false

# Metrics
duration: 32min
completed: 2026-08-22
status: complete
---

# Phase 05 Plan 12: G-05-1 验证与文档收口（gap 闭合证据层）Summary

**G-05-1 三层自动化回归锁落地——协议面 S10（Welcome carriage / 运行期 'W' 推送 / 升格尺寸）、DOM 面 D6（约束 rows / 会话 cols 折行 / 升格解除）、终端核心层 D6H（约束渲染 ≡ 窄端原生等价锁 + 120 列分叉负对照）把用户实测发现的异尺寸双端叠写场景固化为永久门禁断言，README 协议表/resize 节与 05-UAT/05-VALIDATION 同步至新形态，全量六段式回归零回归。**

## Performance

- **Duration:** 32 min
- **Started:** 2026-08-22T04:23:54Z
- **Completed:** 2026-08-22T04:56:00Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- phase05.mjs S10 三断言一次通过（28/28 = 改造前 25 + 3）：S10a 旁观者 Welcome 携会话尺寸 40x10 而非自身窗口 120x40（数值不相等 = carriage 语义在场的区分度断言）；S10b owner RESIZE(60x15) 后旁观者（ro/60x15）与上报者自身（rw/60x15）同收 'W' 推送；S10c 升格 Welcome(mode=rw) 携 cand.dims 120x40（取最后一帧容忍升格+recalcNow 推送同值双帧）
- phase05-dom.mjs D6 三断言一次通过（19/19 = 16 + 3）：D6a 宽端旁观 .xterm-rows 约束到 10 行；D6b 80 个 A 折行为相邻两 div 各恰 40 字符（折行点 = 会话 cols 而非窗口 80——无约束时单行断言必败，区分度锁定）；D6c owner 断开升格后 [ro] 前缀消失且渲染回 24 行（设计约束 4 端到端）
- 新建 phase05-dims.mjs（3/3 pass）：D6H-1 等价锁以「120x40 建起再 resize(40,10)」精确复刻前端 refit() 生产路径，与窄端原生 40x10 对拍逐屏严格一致；D6H-2 负对照同字节流喂 120 列换行点分叉（99 字符 echo 在 40 列 40+40+19 三行 vs 120 列单行），证明等价断言非恒真
- 全量六段式回归：go vet 干净；go test ./... -race 全包 ok（39.8s）；pnpm build 干净（2.3s，dist 重建产物与入库版字节一致——构建确定性实证）；七 UAT 脚本连跑全过（63s）
- 文档同步：README 'W' 行载荷恒在 cols/rows + 运行期推送说明、resize 节改写为约束视口渲染新形态（纯 ro 会话段逐字保持）；05-UAT.md 复跑命令补 dims 脚本、覆盖计数按实测更新、新增 G-05-1 三层扩编段；05-VALIDATION.md 回填 05-10/11/12 三行映射（均实测 green）+ Full suite command 两脚本 + Estimated runtime 实测 ~110s

## Task Commits

Each task was committed atomically:

1. **Task 1: phase05.mjs S10 会话尺寸下发协议断言** - `fa15114` (test)
2. **Task 2: phase05-dom.mjs D6 视口约束断言 + 新建 phase05-dims.mjs（含 05-09 两文件补录）** - `ce91dc5` (test)
3. **Task 3: README/05-UAT/05-VALIDATION 文档同步 + 全量六段式回归** - `fd26ebe` (docs)

**Plan metadata:** 见最终 docs 提交（docs(05-12): complete ... plan）

## Files Created/Modified

- `web/uat/phase05.mjs` - S10 场景函数 s10SessionDims() 挂入 scenarios 尾 + 文件头覆盖注记补 G-05-1 行（+72/-2）
- `web/uat/phase05-dom.mjs` - D6 场景函数 d6ViewportConstraint() 挂入执行序列 + 文件头 D6 三层锁定注记（05-09 产物首次入库，本 plan 增量 +65 行）
- `web/uat/phase05-dims.mjs` - 新建：probe10 探针机制转正（spawn/dialHello 夹具照 phase05.mjs 形态 + @xterm/headless 快照比对 + phase04-t1-width.mjs 结果形态），文件头注释登记血缘
- `web/uat/phase05-flood-driver.mjs` - 05-09 产物首次入库（D5 既有依赖，零内容改动）
- `README.md` - 协议表 'W' 行 + L104 正文 Welcome JSON 同步恒在 cols/rows；resize 行为节改写（会话尺寸下发/约束渲染留白/小窗轴裁剪不变/减员恢复）
- `.planning/phases/05-multi-client/05-UAT.md` - 自动化执行说明段：三层覆盖计数实测更新 + G-05-1 扩编段 + 复跑命令补 dims 脚本（Test 1 result/note 与 Gaps G-05-1 状态字段按 plan 不动，留 verify-work 对账）
- `.planning/phases/05-multi-client/05-VALIDATION.md` - Per-Task Verification Map 追加 05-10-01/05-11-01/05-12-01 三行（实测 green）+ Full suite command 补两脚本 + Estimated runtime 实测回填（frontmatter status/nyquist_compliant 按 plan 不越权）
- `.planning/phases/05-multi-client/deferred-items.md` - 登记 05-10 三文件 gofmt CJK 注释排版漂移 + 仓库根 wesh 二进制未 gitignore（均越界）

## Decisions Made

- **D6H-1 等价锁的精确形态**：取「120x40 建起再 resize(40,10)」而非双 40x10 原生对拍——前端生产路径是 xterm 按 fit 尺寸创建、WELCOME 到达后 refit() 内 term.resize 到会话尺寸，等价锁 1:1 复刻该路径（含 resize 调用本身），断言强度更高
- **D6H-2 负对照的快照比对口径**：buffer.active 全行 translateToString(true)（trimRight）去尾空行后 join 比对——折行点分叉（40+40+19 vs 99 单列）在快照层确定可见，且排除终端 rows 高度差（10 vs 40）造成的尾部空行数差异干扰
- **S10 时序形态**：S10b 断言前无显式等待（50ms 防抖经 5s 轮询窗吸纳，S9b stty 同款）；S10c 取最后一帧 WELCOME 解码（升格 Welcome 与紧随 recalcNow 推送同值幂等，plan 明示意图）
- **probe10.mjs 血缘处理**：plan context 引用的 web/uat/probe10.mjs 从未入库（G-05-1 诊断期一次性探针，git 全历史零记录）——按 plan Task 2 的完整机制描述重建为 phase05-dims.mjs，文件头注释登记血缘并与 05-UAT.md Gaps G-05-1 root_cause 的探针实证记载互链

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] plan context 引用的 probe10.mjs 不存在（从未入库）**
- **Found during:** Task 2（执行前 context 读取——文件系统与 git 全历史均无 probe10.mjs）
- **Issue:** plan 要求「流程照 probe10.mjs」「端口解析复用其正则」「probe10.mjs 保留原位（探针档案）」，但该文件是 G-05-1 诊断期的一次性探针，从未提交（05-UAT.md Gaps G-05-1 root_cause 仅登记其结论「40 列 PTY 字节流喂 120 列 xterm 换行点分叉」）
- **Fix:** 按 plan Task 2 对该探针机制的完整描述（spawn 形态 / A(40,10) owner + B(120,40) 旁观 / 超 40 列长命令注入 / base 切片取流 / 等价+负对照两断言）重建为 phase05-dims.mjs；端口解析正则与夹具复用 phase05.mjs 同款（plan 本意）；新文件头注释登记血缘出处
- **Files modified:** web/uat/phase05-dims.mjs（新建）
- **Verification:** phase05-dims.mjs 3/3 pass 退出码 0（D6H-1 等价 + D6H-2 负对照均成立——探针结论在门禁形态下复现）
- **Committed in:** `ce91dc5`（Task 2 commit）

**2. [Rule 3 - Blocking] phase05-dom.mjs / phase05-flood-driver.mjs 为 05-09 产物但从未入库**
- **Found during:** Task 1 提交前 git status 核查——两文件 untracked，git log --all 零记录
- **Issue:** plan Task 2 修改 phase05-dom.mjs（新增 D6），但该文件不在 git 索引中，「修改」无法以增量形态提交；phase05-flood-driver.mjs 是 D5 场景的既有依赖（spawn 引用），不入库则 phase05-dom.mjs 不可复跑
- **Fix:** 两文件随 Task 2 提交整体补录（flood-driver 零内容改动；dom 文件含本 plan 的 D6 增量），提交信息内显式登记补录事实
- **Files modified:** web/uat/phase05-dom.mjs、web/uat/phase05-flood-driver.mjs（入库）
- **Verification:** 入库后 phase05-dom.mjs 19/19 pass；git ls-files 确认两文件在索引
- **Committed in:** `ce91dc5`（Task 2 commit）

**3. [Rule 1 - Bug] plan 字面 05-10-01 行 Automated Command 的 `\|` 选择器零匹配假绿**
- **Found during:** Task 3（05-VALIDATION.md 回填行的命令实测取证——`go test -race -run 'TestSessionDimsLocked\|TestWelcomeSessionDims\|TestResizeArbitration'` 输出 "no tests to run"）
- **Issue:** Go -run 为正则（RE2），`\|` 是字面管道符而非交替——plan 字面命令匹配零个测试，按此回填 ✅ green 即成假绿行（既有表 05-01-02/05-01-04 等行同形 `\|` 系同一陷阱，越界不动）
- **Fix:** 回填行的命令改裸 `|` 交替（RE2 正确形态），以修正后命令实测 green（三测试组全 PASS，3.4s）后落表
- **Files modified:** .planning/phases/05-multi-client/05-VALIDATION.md
- **Verification:** 修正后命令实测 TestSessionDimsLocked/TestWelcomeSessionDims（两子测）/TestResizeArbitration（四子测）全 PASS
- **Committed in:** `fd26ebe`（Task 3 commit）

---

**Total deviations:** 3 auto-fixed (1 Rule 1 - Bug, 2 Rule 3 - Blocking)
**Impact on plan:** 三处均为 plan 字面与仓库实况的机械矛盾修正（探针未入库 / 夹具未入库 / 选择器语法），全部 must_have truths 与三脚本断言面逐字落地，无范围蔓延；越界项（05-10 gofmt 漂移、wesh 二进制 gitignore、既有表行 `\|` 同形陷阱）登记 deferred-items 不修复。

## Issues Encountered

- GOROOT gofmt 段 1 取证发现 05-10 提交引入的三文件 CJK 注释排版漂移（`//（` → `// （` 半角空格，纯排版零语义）——本 plan 零 Go 文件改动，按 plan 授权跳过清零，登记 deferred-items 留给下一次 Go 触碰 plan 按先例独立 style 提交
- fish 环境下 `tail -2 多文件` 报 "option used in invalid context"（GNU tail 多文件须 `tail -n 2`）——改用 `tail -n 2` 取证，无实质影响

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **G-05-1 翻牌对账就绪**：三层断言（S10/D6/D6H）全部自动化锁定且复跑链三条命令可重现——`/gsd:verify-work` 重跑后可对 05-UAT.md Test 1 result 与 Gaps G-05-1 状态字段翻牌（本 plan 按既定分工未动）；用户实测场景「A 小 B 大，A 内输入」在自动化中等价覆盖（D6b 折行点一致 = 不叠写的可断言等价物；像素层一致性仍属既有豁免）
- **05-11 SUMMARY coverage D6 改判材料就绪**：05-11 coverage D6（human_judgment: true，「行为级端到端断言由 05-12 D6/S10 承载」）——本 plan D2/D1 条目即其回填证据
- **VALIDATION 收口**：05-VALIDATION.md Per-Task Verification Map 已含 05-10/11/12 三行实测 green；frontmatter status/nyquist_compliant 翻牌为 validate-phase §6 职责（本 plan 未越权）
- 无阻塞项；威胁模型三条目（T-05G-07/08/SC）形态与 plan 登记一致——token 红线在新脚本天然不触（dims 不走分享链接无 token 面）、SIGKILL 收口与 jsdom cleanup 纪律沿用、零新依赖（@xterm/headless 6.0.0 锁文件既有），无新增安全面

---
*Phase: 05-multi-client*
*Completed: 2026-08-22*

## Self-Check: PASSED

- 全部 9 个创建/修改文件落盘确认（FOUND）：phase05.mjs / phase05-dom.mjs / phase05-dims.mjs / phase05-flood-driver.mjs / README.md / 05-UAT.md / 05-VALIDATION.md / deferred-items.md / 05-12-SUMMARY.md
- Task 提交 fa15114 / ce91dc5 / fd26ebe 均在 git log 确认（FOUND）；三提交 post-commit deletion check 均为零删除
- Stub 扫描：三脚本无 TODO/FIXME/placeholder/coming soon/not available 命中；无空值直渲 UI 面（纯测试断言脚本）
- 验证证据：S10 28/28+1skip、D6 19/19、dims 3/3（退出码均 0）；go test ./... -race 全包 ok（39.8s）；七 UAT 脚本连跑全过（63s）；dist 重建字节不变
- WINDOWS.md 台账登记三条偏差（id 13/14/15）
