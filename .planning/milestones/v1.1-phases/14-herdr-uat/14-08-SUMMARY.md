---
phase: 14-herdr-uat
plan: 08
subsystem: testing
tags: [herdr, per-client, uat, websocket, protocol-test, is-foreground]

requires:
  - phase: 14-herdr-uat-research
    provides: herdr 观测通道 spike 标定产物（api snapshot area 翻转链 / pane read / 会话隔离与清理序列——RESEARCH §Pattern 3）
  - phase: 13-lifecycle
    provides: phase13.mjs 同构母本（check/skip/startWesh/dialHello/assertOutputClean 红线件与门禁形态）
provides:
  - web/uat/phase14.mjs——PC-13 协议面 herdr driving + ro 汇聚两场景 UAT（18 断言两轮全绿 + exit code 门禁 + 清理零残留）
  - herdr 断言类 UAT 的就绪门形态（初始全量帧到达落定 + area 几何双条件——layouts 非空不充分）
  - herdr 清理完整序列（stop → delete → list 核验——stop 后行保留 stopped 的实测语义）
affects: [14-herdr-uat (14-09 pw 层 / 14-10 run-all / 14-12 收口闸), PC-13]

actuals:
  tokens: 8800   # chars/4 over realized diff（35009 chars / 4）——远低于 estimate 70000（spike 标定产物齐备使实现面收敛）
  tasks: 2
  commits: 2

tech-stack:
  added: []   # 零新依赖红线（T-14-SC）——go.mod/go.sum/package.json/lockfile 零 diff
  patterns:
    - "herdr 就绪门：初始全量帧到达落定（OUTPUT>0 且 150ms 双采样相等）+ area 几何双条件"
    - "maxCursorCol 流层几何特征：CUP/HVP/CHA 三形态正则扫描列坐标最大值（关系断言材料）"
    - "focused_pane_id 动态发现（snapshot 结构定位，不硬编码 w1:p1）"
    - "herdr 清理序列：WS close → wesh SIGTERM(+KILL 兜底) → session stop → delete → list 核验"

key-files:
  created:
    - web/uat/phase14.mjs
  modified: []

key-decisions:
  - "就绪门 Rule 1 实测修正：layouts 非空不充分（server 启动期 area 先 {0,0,0,0} 再默认布局瞬态，后者同样满足 isFullFor 谓词）——改为全量帧到达落定 + area 双条件；移动端必须过门再 attach"
  - "清理序列补 session delete：stop 后 session list 行保留 stopped 态（探针实证），delete 才清册——acceptance「list 不含 SESSION」的完整闭环"
  - "PC-13 勾选留 14-09（pw 观感层承载，共享 ID 先例：11-01/12-01/13-01）"

patterns-established:
  - "herdr 断言类 UAT 就绪门形态（帧到达锚点）——后续 herdr 相关 UAT 复用"
  - "几何断言关系谓词三件套：isCompactFor/isFullFor/maxCursorCol（零 chrome 常量，Pitfall 2 落地形态）"

requirements-completed: []   # PC-13 勾选留 14-09（pw 层承载——协议层证据本 plan 已落）

coverage:
  - id: D1
    description: "S1 herdr driving 场景：桌面 120x40 + 移动 40x12 双 attach——herdr area 翻转链四步（初始全量→移动紧凑→桌面 INPUT 翻回→RESIZE 新几何）+ 流层增量互证（254B/55B << 97049B 全量）+ wesh 层三断言（双 session_start 双 pid / 双端 Welcome per-client / 桌面流 maxCol=120 vs 移动 40）"
    requirement: PC-13
    verification:
      - kind: e2e
        ref: "web/uat/phase14.mjs#S1（node web/uat/phase14.mjs 两轮 18/18 全绿，detail 逐项一致）"
        status: pass
    human_judgment: false
  - id: D2
    description: "S2 ro 汇聚场景：rw 桌面端 + ro 移动端独立 herdr client 经 server 汇聚同会话——ticket 全链唯一通道（POST /api/attach → Hello ticket 键）双端 Welcome.mode 自检（Pitfall 8 防假绿）+ ro INPUT 后 pane read 逐字不变标记缺席（wesh ro 门控实证）+ rw 对照标记可见"
    requirement: PC-13
    verification:
      - kind: e2e
        ref: "web/uat/phase14.mjs#S2（node web/uat/phase14.mjs 两轮 18/18 全绿）"
        status: pass
    human_judgment: false
  - id: D3
    description: "红线自净 + 门禁收口：assertOutputClean 五面零泄漏（token/ticket/pid/标记串/'/s/' 链接形态串）运行时自证 + exit code 门禁（负对照注入必败 check → exit 1 自证）+ 双会话清理零残留（session list 核验）"
    verification:
      - kind: e2e
        ref: "web/uat/phase14.mjs#SEC + #S1j/#S2g（两轮运行 SEC 零命中；负对照副本 exit 1 后还原）"
        status: pass
    human_judgment: false

duration: 23min
completed: 2026-09-06
status: complete
---

# Phase 14 Plan 08: herdr 协议层 UAT（driving + ro 汇聚）Summary

**phase14.mjs 两场景 18 断言两轮全绿：herdr api snapshot area 翻转链（is_foreground 仲裁 + per-client area 渲染）+ 流层增量互证 + wesh 层三断言三通道锁定「移动端 attach 桌面端不被压缩」，ro 汇聚经 pane read 实证输入门控——PC-13 协议面收口（v1.1 存在意义的可证伪证据）**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-06T13:24:10Z
- **Completed:** 2026-09-06T13:47:00Z
- **Tasks:** 2
- **Files modified:** 1（web/uat/phase14.mjs 新建，572 行）

## Accomplishments

- **S1 driving 三通道互证**（「移动端 attach 桌面端不被压缩」——v1.1 里程碑存在意义）：
  - herdr 行为层：`HERDR_SOCKET_PATH` 定向 `herdr api snapshot` 的 `layouts[0].area` 翻转链四步全绿——初始桌面全量（x>0 且 40<width≤120）→ 移动 attach 翻紧凑（width==40 且 x==0）→ 桌面 INPUT 翻回全量（is_foreground 仲裁恢复）→ 移动 RESIZE 50x20 翻新几何（width==50 且 x==0）
  - 流层：移动 attach 后桌面增量 254B << 初始全量 97049B（<5% 关系阈值，无压缩重渲染）；resize 后 55B 增量 + CUP maxCol=74>50（桌面几何特征增量更新）
  - wesh 层：stderr 恰双 session_start 事件（client_id 各异 + pid 不等）、双端 Welcome `session=="per-client"`、桌面流光标列 maxCol=120 vs 移动端 40（双端各自几何独立渲染）
- **S2 ro 汇聚**（FEATURES 裁决 7 文档叙事防说谎防线）：rw 桌面端 + ro 移动端各持独立 herdr client 经 server 汇聚同会话；ticket 全链唯一通道（POST /api/attach 携 share token → Hello JSON ticket 键，零 WS query 形态）双端 Welcome.mode 自检落 check（Pitfall 8 防假绿）；ro 端 INPUT 后 pane read `--source visible` 内容逐字不变 + RO 标记缺席（wesh 服务端丢弃 ro INPUT 实证）；rw 对照标记可见（观测通道自证）+ 终态复合断言
- **红线与门禁**：assertOutputClean 五面零泄漏（token/ticket/pid/标记串/'/s/' 链接形态串）运行时自证；exit code 门禁负对照自证（注入必败 check → exit 1）；几何断言全关系谓词零 chrome 常量；双会话清理零残留（default 会话全程零触达）
- **确定性**：两轮运行 18/18 逐项 detail 完全一致（254B/55B/97049B/maxCol 全同值）——flake 面排除

## Task Commits

Each task was committed atomically:

1. **Task 1: phase14.mjs 骨架 + S1 driving 场景（wesh 层 + herdr 行为层翻转链）** - `7e890d7` (test)
2. **Task 2: S2 ro 汇聚场景 + assertOutputClean 自净 + exit code 收口** - `c5c4608` (test)

**Plan metadata:** （见最终 docs 提交）

## Files Created/Modified

- `web/uat/phase14.mjs` - herdr 协议层 UAT（新文件，572 行）：S1 driving / S2 ro 汇聚两场景；helper 符号 startWesh/dialHello/herdrSnap/areaOf/focusedPaneOf/paneRead/pollUntil/isCompactFor/isFullFor/maxCursorCol/assertOutputClean；常量 WESH/HERDR/SESSION1/SESSION2/SOCK1/SOCK2；会话名前缀 `wesh-uat-p14-`

## Decisions Made

- **就绪门形态（Rule 1 实测修正的产物）**：「桌面初始全量帧到达落定 + area 桌面全量几何」双条件——帧到达晚于 size 上报，是最强就绪锚点；后续 herdr 断言类 UAT 沿用
- **maxCursorCol 三形态扫描**：CUP/HVP（`\x1b[<r>;<c>H|f`）+ CHA（`\x1b[<n>G`）正则取列坐标最大值——spike 只标定 CUP，实现扩展为三形态更稳（关系断言材料，移动端几何只能寻址 ≤cols 列）
- **focused_pane_id 动态发现**：经 snapshot 结构定位 pane id，不硬编码 w1:p1（plan 的「蓝本外形态不静默跟改」预防性落地）
- **PC-13 勾选留 14-09**：协议层证据（本 plan）已落，浏览器观感面归 14-09 phase14-pw.mjs——共享 ID 先例（11-01/12-01/13-01）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] 就绪门「layouts 非空」不充分——首跑 S1a/b/c/i 四项假红/假绿**
- **Found during:** Task 1（首轮运行 6/10）
- **Issue:** plan behavior 原文「轮询 snapshot 至 layouts 非空」实测不充分：herdr server 启动期 `layouts[].area` 先为 `{0,0,0,0}`，再入默认布局瞬态 `{x>0, width≈默认宽-侧栏}`（**后者同样满足 isFullFor 谓词**——80 列默认布局的 54 宽 pane 落在 (40,120] 区间）；桌面 herdr client 的 size 上报之后才翻桌面几何并推全量帧。三个连锁后果：(a) 就绪门过早通过（S1a 假红——area 还是瞬态值）；(b) D0 基线 settle 在 0 字节上假稳定（S1c 假红——全量帧未到，5% 阈值退化为 0）；(c) 移动端过早 attach 使桌面 size 上报成为 last-activity，compact-40 翻转永不出现且 S1d 假绿（area 本就在全量态，「翻回」平凡满足）
- **Fix:** 就绪门改为「桌面初始全量帧到达落定（OUTPUT 字节 >0 且 150ms 双采样相等）+ area 桌面全量几何派生」双条件，D0 基线并入就绪门产出；移动端必须过此门后再 attach
- **Files modified:** web/uat/phase14.mjs（S1 步骤①重写，步骤注释重编号）
- **Verification:** 两轮隔离探针（单桌面客户端时序打点 + 全 S1 流程逐步打点）钉定根因；修复后两轮 10/10 全绿且 detail 逐项一致
- **Committed in:** 7e890d7（Task 1 提交）

**2. [Rule 3 - Blocking] 清理序列补 session delete 步**
- **Found during:** Task 1 前置探针标定
- **Issue:** plan 清理序列为「stop → list 核验缺席或 stopped」，但实测 `herdr session stop` 后 session list 行**保留 stopped 态**（不缺席）——acceptance「session list 输出不含 SESSION」无法直接满足
- **Fix:** 清理序列补 `herdr session delete <name>`（探针实证：delete 清册 + `~/.config/herdr/sessions/<name>/` 状态目录清除；running 态拒删由核验落 FAIL——fail-closed）；顺带实证 `session stop` 以会话名为准（ambient HERDR_SOCKET_PATH 指向 default 时不干扰——执行 shell 位于 herdr pane 内的运行期再实证）
- **Files modified:** web/uat/phase14.mjs（herdrSessionCleanup 三步序列）
- **Verification:** 四轮运行（含负对照）后 session list 仅 default 行；sessions 状态目录为空
- **Committed in:** 7e890d7（Task 1 提交）

---

**Total deviations:** 2 auto-fixed（1 bug + 1 blocking）
**Impact on plan:** 两者均为断言正确性/验收闭环必需，无范围蔓延。就绪门修正是 plan 断言形态与 herdr 运行时现实的校准（spike 标定未覆盖 server 启动期瞬态），已固化为可复用模式。

## Issues Encountered

- **执行环境 Pitfall 5 实证**：本执行 shell 位于 herdr pane 内（HERDR_ENV=1 + ambient HERDR_SOCKET_PATH 指向 default）——手工探针（python PTY 起 herdr client）首次被嵌套阻断拒绝启动，`env -u` 清除后通过；UAT 脚本经 wesh 路径零影响（env 白名单结构性剥离——研究既定结论的运行期再实证，脚本无需任何特判）
- **调试期 pgrep/pkill 自匹配**（Pitfall 3 执行面）：`pkill -f "p14-probe.py"` 匹配到外层 bash 包装命令行杀死自身 shell——括号模式（`p14-probe[.]py`）规避；仅调试期现象，脚本内无 pgrep 形态（pid 断言走 stderr session_start 事件，纪律既定）
- **pankill 探针废弃**：`script -qec` 后台形态起 herdr client 不存活（0x0 winsize + 环境双重问题），改用 python PTY 探针（研究 spike 同款载具）

## Known Stubs

None——全部断言为真实观测（真实二进制 + 真实 herdr 会话 + 真实 wire），无占位/假绿面。

## User Setup Required

None——无外部服务配置需求（herdr 0.8.100 本机在位，版本启动打日志自证）。

## Next Phase Readiness

- **14-09（phase14-pw.mjs Windows 侧）**：协议层翻转链/流层/wesh 层证据已齐，pw 层专注观感断言（双 tab xterm buffer 文本/光标位置——D-07）；PC-13 勾选归其承载
- **14-10（run-all.mjs）**：phase14.mjs 为第 17 项矩阵成员（exit code 0/1 门禁语义就位，两轮基线 18/18）
- **14-12（收口闸）**：herdr 版本漂移处置纪律已落（启动打 --version 进日志）；PC-13 协议面证据链完备

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-06*

## Self-Check: PASSED

- FOUND: web/uat/phase14.mjs（572 行，两任务提交载体）
- FOUND: .planning/phases/14-herdr-uat/14-08-SUMMARY.md
- FOUND: 7e890d7（Task 1 提交）
- FOUND: c5c4608（Task 2 提交）
- 残留会话数: 0（herdr session list 无 wesh-uat-* 行——零残留）
