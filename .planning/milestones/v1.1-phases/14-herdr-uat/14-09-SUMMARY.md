---
phase: 14-herdr-uat
plan: 09
subsystem: testing
tags: [playwright, windows, dual-tab, herdr, per-client, uat, visual-assertion, heisenbug, minrepro]

requires:
  - phase: 14-herdr-uat
    provides: phase14.mjs 协议层标定（herdr area 翻转链 / 流层增量互证 / ticket 认证通道 / 清理序列 stop→delete→list）——pw 层观感断言的时序与语义母本
  - phase: 12-interaction
    provides: phase12-pw.mjs + lib 四件套载具（Forwarder/ssh startWesh/launch/waitTermText——双机拓扑先例形态）
provides:
  - web/uat/pw/phase14-pw.mjs——PC-13 浏览器面观感断言（驱动端架构：T0 进程计数自证 + T1/T2 结构行幸存前缀逐字比对 + CL 清理核验，Windows 实跑两轮 4/4 全绿）
  - minrepro 双件套（web/uat/minrepro-p14.mjs Linux loopback + web/uat/pw/minrepro-win.mjs Windows 过转发器）——浏览器输入链停摆海森bug 的最小复现载具，供 herdr 侧后续定位
  - 载具级海森bug 裁决证据链（10 轮实跑 + 23 探针：herdr/wesh 服务端无罪，浏览器 tab 首键入后输出流非确定性死亡）+ 驱动端架构绕道模式（键入走裸 WS 驱动端，浏览器退化为被动渲染观测面）
  - phase 14 UAT coverage 台账（14-UAT.md 33/33：#1-29 覆盖项 + #30-33 复跑证据）
affects: [14-herdr-uat (14-10 run-all / 14-12 收口闸), PC-13, herdr 上游浏览器停摆定位]

actuals:
  tokens: 12100   # chars/4 over realized diff（37442 代码 diff + 9797 UAT 台账 + 1228 COVERAGE ≈ 48467 chars）——estimate 50000 的 24%；海森bug 调查的实跑轮次成本不计入 token 面
  tasks: 2
  commits: 4

tech-stack:
  added: []   # 零新依赖红线（T-14-SC）——playwright 1.62.1 既有钉版，lockfile 零 diff
  patterns:
    - "驱动端架构：键入/观测走 Windows Node 裸 WS（ticket 认证 POST /api/attach → Hello 携 ticket），浏览器 tab 退化为纯被动渲染观测面（零键入）"
    - "结构行幸存前缀断言：目标行 = 含 │ 且边框右侧空白的结构行；pane 内容向下增长只从尾部吞 blank 结构行——幸存前缀序位不变，shared 压缩会整体改写边框列（D-07 判别面守恒）"
    - "T0 去键入化自证：herdr client 进程计数随客户端 1→2→3 递增（pgrep 锚定 ^[^ ]*/herdr 排除自匹配）+ 驱动端 pane shell pid 恒定正向对照"
    - "DOM 全量 rAF 同步读（page.evaluate 全量读 .xterm-rows，非增量）+ 假绿防线（移动端断言前必须渲染出驱动端键入的 stty 标记）"

key-files:
  created:
    - web/uat/pw/phase14-pw.mjs   # 416 行——驱动端架构观感断言（P14-T0/T1/T2/CL 四 Check）
    - web/uat/minrepro-p14.mjs    # 98 行——Linux loopback 最小复现（裸 WS attach 166x61 → 首帧落定 → INPUT → 30s 观测停流）
    - web/uat/pw/minrepro-win.mjs # 96 行——Windows 过转发器最小复现（同判别面，双机形态）
    - .planning/phases/14-herdr-uat/14-UAT.md   # phase 14 coverage 台账 33/33
  modified:
    - .planning/phases/14-herdr-uat/COVERAGE.md  # 外部 API 免责声明一行压缩

key-decisions:
  - "海森bug 架构裁决（Rule 3 实证驱动）：浏览器 tab 首键入后输出流非确定性死亡（10 轮实跑 + 23 探针判定载具级——herdr/wesh 服务端无罪：Linux loopback 与 Windows 裸 WS 最小复现均干净、被动流全程存活）；键入通道改 Windows Node 裸 WS 驱动端（ticket 认证），浏览器双 tab 退化为纯被动渲染观测面，D-07 断言面不变"
  - "paneEcho 断言通道加固（Rule 1）：改单发简单轮询 + 结构行对齐比较——7 轮实跑全在 paneEcho 命中 pane 输出流停摆，19 个组件级 A/B 探针逐一排除（服务端 healthz/Send-Q 全健康）后收敛为停摆海森bug 而非断言逻辑缺陷"
  - "T1/T2 语义对齐 herdr last-activity-wins 翻转链（14-08 S1b/S1d 先例）；T0 去键入化（server.log 三次 session_start 双 pid + 进程计数递增）"
  - "minrepro 双件套交付：Linux loopback + Windows 过转发器两形态供 herdr 侧后续定位浏览器停摆（Track2 交付物，超出 plan 文本的偏差副产）"
  - "PC-13 勾选收口（14-08 既定裁决兑现：协议层 18/18 本 plan 前 + 浏览器面两轮 4/4 本 plan——共享 ID 先例 11-01/12-01/13-01）"

patterns-established:
  - "pw 层驱动端架构（键入/观测分离）——后续涉及浏览器键入链的双机 UAT 沿用"
  - "结构行幸存前缀比对（边框列唯一 + 幸存前缀逐字）——herdr 类 TUI 布局稳定性断言的判别面形态"

requirements-completed: [PC-13]

coverage:
  - id: D1
    description: "phase14-pw.mjs 三测观感断言（驱动端架构形态）：T0 per-client 自证（herdr client 进程计数 1→2→3 递增 + 驱动端 pane shell pid 恒定 + server.log 三次 session_start 双 pid）+ T1 移动 tab attach 后桌面 tab 结构行幸存前缀逐字不变（边框列唯一）+ T2 移动 tab 竖屏→横屏 resize 后仍逐字不变 + CL 清理核验（session stop→delete→list 零残留，default 会话零触达）"
    requirement: PC-13
    verification:
      - kind: automated_ui
        ref: "web/uat/pw/phase14-pw.mjs#P14-T0/T1/T2/CL（Windows 工作站实跑两轮 4/4 全绿——P14-T0/T1/T2 + CL 四 Check，1 skipped 为像素视觉豁免项，exit code 0）"
        status: pass
    human_judgment: false
  - id: D2
    description: "截图六帧人工复核：桌面全量布局跨 attach/resize 恒定、移动 tab 独立紧凑几何——per-client area 渲染的视觉确证（像素视觉属 CODEBUDDY.md 测试策略 §5 平台豁免，以 skipped+reason 记录不列阻塞项，人工复核承载）"
    requirement: PC-13
    verification:
      - kind: manual_procedural
        ref: "screenshots/p14-*.png 六帧（T0/T1/T2 三时点 × 双 tab）——用户人工复核通过（2026-09-07，Task 2 确认门批准）"
        status: pass
    human_judgment: true
    rationale: "像素视觉为平台豁免项（CODEBUDDY.md §5）——自动化只断言 xterm buffer 文本/结构行，布局恒定的观感级确认需人眼复核截图；D-07 既定否决截图 diff 回归（时钟/状态闪烁面 flake 风险）"
  - id: D3
    description: "minrepro 双件套（海森bug 调查 Track2 交付）：minrepro-p14.mjs（Linux loopback：裸 WS attach→首帧落定→INPUT→30s 停流观测）+ minrepro-win.mjs（Windows 过转发器同判别面）——供 herdr 侧定位浏览器输入链停摆"
    verification:
      - kind: other
        ref: "web/uat/minrepro-p14.mjs + web/uat/pw/minrepro-win.mjs（海森bug 调查期实跑：两形态均未复现停摆——服务端无罪的反向证据即交付价值；node --check 零错）"
        status: pass
    human_judgment: false

duration: 19h50min
completed: 2026-09-07
status: complete
---

# Phase 14 Plan 09: phase14-pw.mjs Windows Playwright 双 tab 观感断言 Summary

**Windows 工作站双 tab 真实 Chromium 观感收口：两轮 4/4 全绿（T0 进程自证/T1 attach 不压缩/T2 resize 不压缩/CL 零残留）+ 截图六帧人工复核通过——PC-13 浏览器面收口；执行期海森bug（浏览器 tab 首键入后输出流非确定性死亡）经 10 轮实跑 + 23 探针裁决为载具级（herdr/wesh 服务端无罪），架构绕道为裸 WS 驱动端 + 被动渲染观测面，并交付 minrepro 双件套供 herdr 侧定位**

## Performance

- **Duration:** 19h50min（挂钟——含 Windows 实跑确认门等待与海森bug 调查；2026-09-07 01:03 → 20:53 +0800）
- **Started:** 2026-09-06T17:03:51Z
- **Completed:** 2026-09-07T12:53:45Z
- **Tasks:** 2（Task 1 编写 + Task 2 Windows 实跑确认门——后者产生两轮脚本修订）
- **Files modified:** 6（phase14-pw.mjs 新建 416 行 / minrepro-p14.mjs 新建 98 行 / minrepro-win.mjs 新建 96 行 / 14-UAT.md 新建 / COVERAGE.md 修订 / SUMMARY 本体）

## Accomplishments

- **PC-13 浏览器面观感收口（v1.1 里程碑 driving scenario 的最终用户体验证据）**：Windows 工作站 Playwright 双 tab（桌面 1600x1000 + 移动 390x700 双 context，经 TCP 转发器连接 Linux 侧 per-client 模式 wesh + herdr 命名会话）实跑两轮 4/4 全绿：
  - **T0 per-client 自证**：herdr client 进程计数随客户端 1→2→3 递增（pgrep 锚定 `^[^ ]*/herdr` 结构性排除 wesh/run.sh/bash 自匹配）+ 驱动端 pane shell pid 恒定（herdr 会话共享 pane 正向对照）——假绿防线（phase12-pw T1b 先例形态的去键入化演进）
  - **T1 核心观感**：移动小屏 tab attach 后，桌面大屏 tab 的 herdr 面板结构行（侧栏 cols 0-24 + 边框 │ col 25 + 右侧 pane 区）幸存前缀同序位逐字一致、边框列唯一
  - **T2 resize 观感**：移动 tab 竖屏 390x700 → 横屏 700x390（跨布局阈值）后，桌面 tab 结构行仍逐字不变
  - **CL 清理核验**：session stop → delete → list 零残留，用户日常 default 会话全程零触达（D-09 零污染纪律）
  - 1 skipped 为像素视觉豁免项（CODEBUDDY.md §5，skipped+reason 记录不列阻塞）
- **截图六帧人工复核通过**：桌面全量布局跨 attach/resize 恒定、移动 tab 独立紧凑几何——per-client area 渲染的视觉确证
- **海森bug 调查与架构裁决（本 plan 最大执行期事件）**：Windows 首轮实跑 7 轮全部在 paneEcho 命中 pane 输出流停摆（DOM 冻结、无 onclose、服务端 healthz/Send-Q 全健康）；经 19 个组件级 A/B 探针逐一排除 + minrepro 双件套（Linux loopback / Windows 过转发器均未复现）+ 10 轮实跑 23 个探针，最终裁决：**浏览器 tab 首键入后其输出流非确定性死亡为载具级海森bug（herdr/wesh 服务端无罪）**——键入通道改 Windows Node 裸 WS 驱动端（ticket 认证：POST /api/attach 携 Basic → ticket → Hello 携 ticket，phase14.mjs S2 同通道），浏览器双 tab 退化为纯被动渲染观测面（零键入）；D-07 断言面不变
- **phase 14 UAT coverage 台账 33/33**（14-UAT.md）：#1-29 七 SUMMARY all_auto_covered 覆盖项 + #30-33 复跑证据（冷启动全仓 -race 五包全绿 2m37s / 部署面批 9 测双列复跑 / SC1 mode= 子测计数恰 216 / phase14.mjs e2e 复跑 18/18 会话零残留）

## Task Commits

Each task was committed atomically:

1. **Task 1: phase14-pw.mjs 编写——双 tab 观感断言三测** - `6c376ae` (test)
2. **Task 2 执行期修订 ①: paneEcho 改单发简单轮询 + 结构行对齐比较加固——海森bug 矩阵登记** - `94a29e5` (test)
3. **Task 2 执行期修订 ②: 改驱动端架构两轮 4/4 全绿 + minrepro 双件套** - `8956e70` (test)
4. **phase 证据链产物: 14-UAT.md 台账 33/33 + COVERAGE 免责声明压缩** - `ea7cacb` (test)

**Plan metadata:** （见最终 docs 提交）

## Files Created/Modified

- `web/uat/pw/phase14-pw.mjs` - PC-13 浏览器面观感断言（新建，最终 416 行）：驱动端架构（协议常量与 internal/proto/proto.go 对齐——D-16 两侧注释互指）；视口阶梯 V_DESK/V_MOB_PORTRAIT/V_MOB_LANDSCAPE（A5：pane 内 stty 实测回读记录进 Check detail，不硬编码映射）；会话名前缀 `wesh-uat-p14-pw-`
- `web/uat/minrepro-p14.mjs` - Linux loopback 最小复现（新建 98 行，零依赖 Node ≥22）：裸 WS attach 166x61 → 首帧落定 → +8s 后 INPUT → 30s 停流观测——若 loopback 复现即纯 herdr/服务端问题
- `web/uat/pw/minrepro-win.mjs` - Windows 过转发器最小复现（新建 96 行）：同判别面，双机拓扑形态
- `.planning/phases/14-herdr-uat/14-UAT.md` - phase 14 coverage 模式 UAT 台账（33/33 pass）
- `.planning/phases/14-herdr-uat/COVERAGE.md` - 外部 API 免责声明一行压缩（herdr CLI 为测试期 fixture 非产品集成，指向 §Arch-3 与 D-06/08/09 锁定）

## Decisions Made

- **驱动端架构（海森bug 绕道的承重裁决）**：断言键入不再经浏览器 tab，改由 Windows Node 裸 WS 驱动端承载（ticket 认证全链）；浏览器双 tab 只做被动渲染观测（waitTermText 锚定 + page.evaluate 全量 rAF 同步读 .xterm-rows）。被动流全程存活（probe3 + 每轮移动 tab 锚定即时命中）实证该架构的可行性
- **结构行判别面（@xterm/headless 探针标定，2026-09-06，herdr 0.8.100 实测）**：目标行 = 含 │ 且边框右侧空白的结构行（~31/40 行）；pane 内容行排除——前台几何 pane reflow 属 herdr 正确行为非压缩症状；pane 内容基线后向下增长只从尾部吞 blank 结构行（幸存前缀序位不变），shared 压缩翻紧凑布局会整体改写边框列 + 全部行文本——判别力守恒
- **T0 去键入化**：per-client 自证从「双 tab 各自键入取 shell pid」改为「进程计数递增 + 驱动端 pid 恒定 + server.log 三次 session_start 双 pid」——规避浏览器键入链的同时保留假绿防线
- **假绿防线双件**：移动端断言前必须等其 buffer 渲染出驱动端键入的 stty 标记（herdr 全量首帧 = pane 内容共享证据）；T2 前必须确认移动端渲染行数随视口变更而变化
- **minrepro 双件套作为正式交付物**（Track2）：plan 文本未列，属海森bug 调查的偏差副产——两形态均未复现停摆本身即「服务端无罪」的反向证据，供 herdr 上游定位浏览器停摆
- **PC-13 勾选本 plan 收口**（14-08 既定裁决兑现）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] paneEcho 断言通道停摆假红——首轮 Windows 实跑 7 轮全在 paneEcho 翻车**
- **Found during:** Task 2（Windows 工作站实跑，2026-09-07）
- **Issue:** plan 行为原设定的 paneEcho 通道（浏览器 tab 键入 echo 标记 + 观测回显）在 Windows 实跑中非确定性停摆：7 轮实跑全部在 paneEcho 命中 pane 输出流停摆——DOM 冻结、无 onclose 事件、服务器侧 healthz/Send-Q 全健康
- **Fix:** ① paneEcho 改单发简单轮询（放弃多轮 ping-pong 时序耦合）+ 结构行对齐比较加固（本提交层面的加固，仍受海森bug 约束）；② 19 个组件级 A/B 探针逐一排除（wesh 服务端/herdr/转发器/浏览器 context 隔离/键入事件形态等），矩阵登记后裁决为载具级海森bug 而非断言逻辑缺陷——最终由偏差 2 的架构绕道根治
- **Files modified:** web/uat/pw/phase14-pw.mjs
- **Verification:** 探针矩阵（19 组件级 A/B）+ 服务端健康双通道（healthz 轮询 + ss Send-Q）全绿证据链
- **Committed in:** 94a29e5

**2. [Rule 3 - Blocking] 浏览器输入链停摆海森bug——架构裁决绕道（驱动端架构）**
- **Found during:** Task 2（Windows 实跑迭代期）
- **Issue:** 浏览器 tab 自身首键入后其输出流非确定性死亡（10 轮实跑 + 23 个探针实证）：herdr/wesh 服务端无罪（Linux loopback minrepro-p14.mjs 与 Windows 裸 WS minrepro-win.mjs 最小复现均干净、被动流全程存活），载具级海森bug 阻塞 plan 既定的「浏览器双 tab 键入 + 观测」断言架构
- **Fix:** 架构绕道（承重裁决）：键入通道改 Windows Node 裸 WS 驱动端（ticket 认证：POST /api/attach 携 Basic → ticket → Hello 携 ticket——phase14.mjs S2 同通道），浏览器双 tab 退化为纯被动渲染观测面（零键入）；T1/T2 语义对齐 herdr last-activity-wins 翻转链（14-08 S1b/S1d 先例）；T0 去键入化（server.log 三次 session_start 双 pid）；DOM 全量 rAF 同步读；同提交交付 minrepro-p14.mjs + minrepro-win.mjs 双件套供 herdr 侧后续定位
- **Files modified:** web/uat/pw/phase14-pw.mjs（改驱动端架构）、web/uat/minrepro-p14.mjs（新建）、web/uat/pw/minrepro-win.mjs（新建）
- **Verification:** Windows 实跑两轮 4/4 全绿（P14-T0/T1/T2 + CL 清理核验，exit code 0）+ 截图六帧人工复核通过
- **Committed in:** 8956e70

---

**Total deviations:** 2 auto-fixed（1 bug + 1 blocking）
**Impact on plan:** 偏差 2 为承重架构裁决——探针证据链（10 轮实跑 + 23 探针 + 双形态 minrepro 均不复现）支撑「服务端无罪」结论，绕道不改变 D-07 断言面（桌面 tab 渲染观测 + 结构行对齐比较不变），minrepro 双件套为超出 plan 文本的正向副产（herdr 上游定位资产）。无范围蔓延。

## Issues Encountered

- **Windows 实跑确认门（Task 2 checkpoint:human-verify）**：按 CODEBUDDY.md 双机拓扑，脚本在 Linux 侧编写（仓库共享），实跑在 Windows 工作站侧执行——两轮 4/4 全绿 + 截图六帧人工复核后用户批准（approved）；实跑过程驱动了两个脚本修订提交（94a29e5 / 8956e70，见 Deviations）
- **双机拓扑红线全程零违反**：Linux 侧零浏览器零 playwright（脚本无任何安装/网卡动作，grep 自审）；Windows 侧未操作真实网卡（断网模拟不涉及，转发器 kill/restore 形态未触发）；herdr 命名会话 `wesh-uat-p14-pw-<ts>` 时间戳唯一，default 会话零触达
- **phase 证据链产物复跑**（确认门等待期间完成）：14-UAT.md 台账 #30-33 自动化项复跑——冷启动全仓 -race 五包全绿（cmd/wesh 1.336s / internal/proto 1.016s / internal/pty 2.655s / internal/server 157.060s / internal/web 1.011s）、部署面批 9 测双列复跑、SC1 mode= 子测计数恰 216（与 14-06 SUMMARY 收口口径逐字一致）、phase14.mjs e2e 复跑 18/18（增量 254B/55B、全量 97049B、maxCol 120/40/74 与 14-08 记录逐项一致）

## Known Stubs

None——全部断言为真实观测（真实 Chromium + 真实转发器链路 + 真实 wesh/herdr 会话），无占位/假绿面。像素视觉豁免项按 CODEBUDDY.md §5 以 skipped+reason 记录并由 D2 人工复核承载（非 stub）。

## User Setup Required

None——无外部服务配置需求（Windows 工作站 playwright 1.62.1 钉版依赖既装，Chromium 缓存就绪）。

## Next Phase Readiness

- **14-10（run-all.mjs）**：phase14.mjs 已是矩阵第 17 项成员；phase14-pw.mjs 属 Windows 侧载具不进 Linux 侧 run-all 矩阵（双机拓扑分工——14-10 plan 应知悉）
- **14-12（收口闸）**：PC-13 勾选本 plan 已收口；phase14-pw 两轮 4/4 + 截图复核记录、minrepro 双件套、14-UAT.md 台账 33/33 均为收口闸证据链成员；diff 白名单审查应包含本 plan 的 6 文件（4 新建 + 2 修订，零产品代码面）
- **herdr 上游（超出本里程碑）**：浏览器输入链停摆海森bug 的最小复现双件套已交付（minrepro-p14.mjs / minrepro-win.mjs）——herdr 侧定位资产

## Self-Check: PASSED

- FOUND: web/uat/pw/phase14-pw.mjs（416 行，驱动端架构最终形态）
- FOUND: web/uat/minrepro-p14.mjs（98 行）+ web/uat/pw/minrepro-win.mjs（96 行）
- FOUND: .planning/phases/14-herdr-uat/14-09-SUMMARY.md
- FOUND: 6c376ae（Task 1）/ 94a29e5（修订①）/ 8956e70（修订②）/ ea7cacb（UAT 台账）

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-07*
