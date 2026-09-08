---
phase: 14-herdr-uat
plan: 11
subsystem: documentation
tags: [docs, per-client, session-mode, calibration, gotty-errata, pc-12]

requires:
  - phase: 14-herdr-uat
    provides: 14-07 LOADDATA pc_flood/pc_resident 八行实测数据 + D-11 证真结论（maxClients=32 默认不动）——标定表数据源
  - phase: 14-herdr-uat
    provides: 14-08/14-09 herdr 配方实证 argv 形态（phase14.mjs/phase14-pw.mjs——文档即被测物的逐字来源）
provides:
  - README「会话模式」节（模式语义表 + 分享链接/ro/herdr·tmux 三语义行 + herdr 配方 sh 块 + tmux 对照句）+ 资源义务段（实测标定表双剖面 + max-clients 建议值表）
  - docs/ARCHITECTURE.md「双模式架构」节（装配期一次分岔说明 + goroutine 拓扑 mermaid 对比图 + keep/degrade/vanish 表）+ :7 GoTTY 误记修正
  - docs/CONFIGURATION.md max-clients per-client 兼任并发进程上限语义行
affects: [14-12]

actuals:
  tokens: 3500    # 13931 chars / 4 over 5f8d711..HEAD（README.md + docs/ARCHITECTURE.md + docs/CONFIGURATION.md realized diff；plan 估算 45000 高估——估算含研究上下文阅读面）
  tasks: 2
  commits: 3

tech-stack:
  added: []   # 零新依赖（T-14-SC 红线——纯文档面零装包）
  patterns:
    - "同位扩展零侵蚀：:96 session-mode 单段与 :98 保活段字节级不动，新节内容纯插入（git diff 纯新增零删除——「既有 shared 表述零削弱」红线以 diff 形态自证）"
    - "标定表数据保真链：LOADDATA 原始字节值 → 30 项程序化换算断言核对（禁手抄——N=16 取整偏差即被此链捕获修正）"

key-files:
  created: []
  modified:
    - README.md
    - docs/ARCHITECTURE.md
    - docs/CONFIGURATION.md

key-decisions:
  - "herdr 配方 flag 顺序取 phase14.mjs 实证 argv 逐字形态（--writable 在 --session-mode=per-client 之前）——T-14-23「配方与 argv 逐字核对」按被测物优先；plan truth 文本的 flag 顺序与 mjs 相反，语义等价（Go flag 解析序无关）但逐字一致性以实证形态为准"
  - "D-11 结论按 14-07 证真口径书写：默认 32「实测可承载，保持不变」明示 + 建议值表三档（常规服务器 32 不动 / 低配 VPS·内存受限 8 / 个人多端 4）——建议值标注资源画像分档非硬性门槛，依据列全部锚定实测值"
  - "ARCHITECTURE 七处显式分支点以代码 grep 核实枚举（New:586/Attach:1109/RESIZE:1278/detach:663/kick:890/exit-when-empty:973+1007/Shutdown:1746——clients.go 两处为同一逻辑分支的立即/宽限两形态）；README 节取 ## 顶层标题（D-14 可达性否决二级文档的同理面）"

patterns-established:
  - "Pattern: 文档即被测物的机器验证三件套——grep 验收闸（正断言）+ 措辞红线负断言 + 程序化数据核对（30 项换算断言）——后续文档类 plan 的验收面形态"

requirements-completed: []   # PC-12 共享 ID 门（#2388）：14-12-PLAN 同声明 PC-12 且无 SUMMARY——勾选留 14-12 收口（11-01/12-01/13-01 先例延续）

coverage:
  - id: D1
    description: "README「会话模式」节：模式语义表（shared/per-client + 选择方式列）+ 三语义行（分享链接=按权限级别独立进程入场券 / ro=自有进程输入门控 / herdr·tmux 经多路复用汇聚）+ herdr 配方 sh 块 + tmux 对照句"
    requirement: PC-12
    verification:
      - kind: command
        ref: "plan verify 命令三项全过：grep -n '会话模式' README.md（:96 命中）/ grep -c 'window-size' ≥1 / grep -q 'max-clients'"
        status: pass
      - kind: command
        ref: "负断言 '默认取最小值' README.md 零命中（tmux 措辞红线）；配方 argv 含 --session-mode=per-client 与 herdr --session（phase14.mjs 形态）；git diff 纯新增 57 行零删除"
        status: pass
    human_judgment: false
  - id: D2
    description: "资源义务段实测标定表（驻留/洪水双剖面各四行）+ max-clients 建议值表（默认 32 不动明示）"
    requirement: PC-12
    verification:
      - kind: command
        ref: "30 项程序化数据核对全过：LOADDATA 原始字节值（mem_delta/gor·fd 差值/rss_sum/alloc_peak/dur_ms/bytes_per_session）→ README 表值逐项换算断言（含 N=16 取整修正 959→960KiB）；吞吐列标注推算属性"
        status: pass
    human_judgment: false
  - id: D3
    description: "ARCHITECTURE「双模式架构」节：装配期一次分岔/运行期零分岔说明 + goroutine 拓扑 mermaid 对比图 + keep/degrade/vanish 表"
    requirement: PC-12
    verification:
      - kind: command
        ref: "grep -c 'mermaid' docs/ARCHITECTURE.md 1→2 增加；结构化校验通过（14 节点声明/全部边目标存在/2 subgraph-2 end 配对/围栏 6 行成对）"
        status: pass
    human_judgment: true
    rationale: "mermaid 渲染效果（GitHub/mermaid 渲染器）无法本机目检——Linux 开发机禁浏览器（CODEBUDDY 双机拓扑硬约束）；语法经结构化校验，渲染观感归 14-12 收口闸/verify-work 人工复核"
  - id: D4
    description: ":7 GoTTY 误记修正（per-connection spawn 修正表述 + v1.1 双模式分岔说明 + 上下文句连贯性）"
    requirement: PC-12
    verification:
      - kind: command
        ref: "grep -n 'per-connection' docs/ARCHITECTURE.md 命中（:7 两处）；负断言 'GoTTY 式共享进程模型' 全文零命中（grep 实证，勘误语境改写为修正语义）"
        status: pass
    human_judgment: false
  - id: D5
    description: "CONFIGURATION 默认值表 max-clients per-client 语义行（兼任并发进程上限 + spawn 前复检）"
    requirement: PC-12
    verification:
      - kind: command
        ref: "grep -n '兼任并发进程上限' docs/CONFIGURATION.md（:155 默认值表行命中）；stop-timeout 双默认值行邻居零改动；REQUIREMENTS.md 零 diff（D-15 历史注记不改写红线）"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-09-07
status: complete
---

# Phase 14 Plan 11: PC-12 模式语义文档三件套 Summary

**README「会话模式」节（模式语义表/三语义行/herdr 配方/tmux 对照句 + 14-07 LOADDATA 八行实测标定表双剖面 + max-clients 建议值表）+ ARCHITECTURE「双模式架构」段（七分支点分岔说明 + goroutine 拓扑 mermaid 对比图 + keep/degrade/vanish 表）与 :7 GoTTY 误记修正 + CONFIGURATION max-clients per-client 语义行——PC-12 三件套收口，全部数据可溯实测、配方与 phase14.mjs argv 逐字一致。**

## Performance

- **Duration:** 15 min
- **Started:** 2026-09-07T14:27:02Z
- **Completed:** 2026-09-07T14:41:32Z
- **Tasks:** 2
- **Files modified:** 3（README.md +58/-1、docs/ARCHITECTURE.md +54/-1、docs/CONFIGURATION.md +1/-1）

## Accomplishments

- **README「会话模式」节（D-14 主承载）**：:96 session-mode 单段同位扩展为 `##` 顶层节（原段与 :98 保活先杀段字节级零改动，新内容纯插入——「既有 shared 语义表述零削弱」prohibition 以 diff 纯新增 57 行零删除自证）——模式语义表（`shared` 多人同屏共享进程差异化本体默认 / `per-client` 每客户端独立 PTY 进程 ttyd 式 + `--session-mode`/TOML `session-mode` 选择方式列）+ 三语义行（分享链接=按权限级别的独立进程入场券 / ro=对自有进程的输入门控 / 配合 herdr/tmux 经多路复用汇聚——FEATURES 裁决 6/7 叙事源）+ herdr 配方 sh 块（`wesh --writable --session-mode=per-client -- herdr --session <name>`，与 phase14.mjs 实证 argv 逐字一致）+ is_foreground 仲裁恢复效果说明（移动端 attach 翻紧凑、桌面键入恢复全量）
- **tmux 对照句按措辞红线落地**（RESEARCH §State of the Art 本机实测修正）：默认 window-size=latest 下小屏客户端活动使窗口收缩、桌面端观感被压缩；window-size 可配但均为单窗口尺寸语义，不存在 per-client 独立渲染——负断言「默认取最小值」全文零命中（grep 实证）
- **资源义务段（D-11 回填 + 09-09 D-13 表格形态沿用）**：驻留剖面四行（wesh 侧内存 81KiB→1.9MiB / gor 5N+1 / fd 精确 4N / 子进程 VmRSS 合计 3.6→114.7MiB）+ 洪水剖面四行（Alloc 峰值 2.2→55.0MiB / 全端收流 3.3→8.8s / 每端吞吐推算列标注推算属性）+「32 个并发 shell 是重负载」义务句 + 标定口径注记（gor 5N+1 为 UAT 关保活形态、生产账面 6N；吞吐=实测字节÷实测时长）+ 建议值表三档（**默认 32 实测可承载不动明示** / 低配 VPS·内存受限 8 / 个人多端 4，依据列全部锚定 14-07 实测值）
- **ARCHITECTURE「双模式架构」节**（插点：数据流与关键抽象之间）：装配期一次分岔/运行期零分岔说明（七处显式分支点代码核实枚举 + INPUT 零分支 `client.inQ` 间接字段说明）+ goroutine 拓扑 mermaid 对比图（shared 3+3N 单会话拓扑 vs per-client 1+6N 五件装配[ReadLoop 闭包/inputWriter/writer/pinger/sessionWatcher] + pcSupervisor 终结源——事实源 perclient.go `startSessionGoroutines`）+ keep/degrade/vanish 三行组件差异表（FEATURES :111-135 素材：vanish=fan-out hub/仲裁器/owner 递补/'W' 约束帧；degrade=EXIT 单播/N 进程组终结/分享链接入场券/max-clients 兼任进程闸等；keep=ticket/节流/Origin/env 白名单等模式无关件）+ README 交叉引用句
- **:7 GoTTY 误记修正（D-14）**：「GoTTY 式共享进程模型」现状表述改写为修正语义——v1.0 共享进程模型为 wesh 自有差异化设计（勘误注记：经 GoTTY 源码核实 GoTTY 实为 per-connection spawn，每 WS 连接各 `factory.New` → `pty.Start` 一个进程，此前表述有误，同类工具中无先例）+ v1.1 起双模式分岔说明（shared 默认 / per-client opt-in）+ 上下文句（「主要输出」扇出表述按模式分岔）连贯性同步；全文负断言「GoTTY 式共享进程模型」零命中
- **CONFIGURATION max-clients per-client 语义行**：默认值表 :155 行扩展——兼任并发进程上限（握手 503 闸之外 spawn 前再复检计数，并发子进程数恒 ≤ max-clients，含断开待收割的 linger 会话）；stop-timeout 双默认值行（:160 邻居）零改动
- **验证矩阵**：两任务 grep 验收闸全过 + 三重负断言（「默认取最小值」/「GoTTY 式共享进程模型」/REQUIREMENTS.md 零 diff）全过 + LOADDATA 30 项程序化数据核对全过 + mermaid 结构化校验（14 节点/边目标全存在/subgraph-end 配对/围栏成对）通过

## Task Commits

Each task was committed atomically:

1. **Task 1: README「会话模式」节 + 资源义务段实测标定回填** - `d364349` (docs)
2. **Task 2: ARCHITECTURE 双模式段 + :7 误记修正 + CONFIGURATION max-clients 行** - `b0bc487` (docs)
3. **自审修正: 标定表 N=16 驻留内存取整 959→960KiB** - `604f5b3` (docs)

## Files Created/Modified

- `README.md` - 「会话模式」节（+57 纯新增）+ 标定表 N=16 取整修正（+1/-1）
- `docs/ARCHITECTURE.md` - 「双模式架构」节（+53 新增）+ :7 GoTTY 误记改写行（+1/-1）
- `docs/CONFIGURATION.md` - 默认值表 max-clients 行 per-client 语义扩展（+1/-1）

## Decisions Made

- **herdr 配方 flag 顺序取实证形态**：phase14.mjs argv 为 `--writable --session-mode=per-client`（startWesh args 顺序），plan truth 文本写作 `--session-mode=per-client --writable`——两者语义等价（Go flag 解析序无关），但 T-14-23 mitigation 要求「配方与 phase14.mjs argv 逐字核对」，逐字一致性以被测物（mjs）为准
- **README 节取 `##` 顶层标题 + 同位扩展零侵蚀**：`## 会话模式` 插入可写协作示例与 :96 段之间，:96 段成为节首段（字节级不动）；D-14 否决 SESSION-MODES.md 的理由（二级文档降低可达性）同理适用于 `###` 埋层级——主承载取顶层可见性
- **建议值表口径**：三档分档（32 不动/8/4）为 D-11 既定授权（CONTEXT「如低配 VPS 建议 --max-clients=8」），依据列全部锚定实测值并标注「资源画像分档非硬性门槛」；子进程真实成本取决于 `<cmd>` 本体（bash 实测 ~3.7MiB/进程）如实写明
- **七分支点枚举核实**：clients.go :973/:1007 经核读为 maybeExitWhenEmptyLocked 同一逻辑分支的立即/宽限到期两形态——按逻辑分支计 7 处（New/Attach/RESIZE/detach/kick/exit-when-empty/Shutdown），与 RESEARCH「6-7 显式分支点」口径一致

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - 数据保真] 标定表 N=16 驻留内存取整偏差**
- **Found during:** plan 级数据逐项核对（自审步骤）
- **Issue:** LOADDATA mem_delta=982720B = 959.69KiB，README 表初稿写 959KiB（截断取整）——与其余行标准四舍五入口径不一致（83136B→81KiB 等三行截断/舍入同值无差异，仅此行分叉）
- **Fix:** 修正为 960KiB（标准四舍五入），独立提交；30 项程序化核对全过建立防手抄回归面
- **Files modified:** README.md
- **Verification:** `Math.round(982720/1024) === 960` 程序化断言 PASS；全部 30 项数据核对 PASS
- **Committed in:** 604f5b3

---

**Total deviations:** 1 auto-fixed（数据保真）
**Impact on plan:** 取整口径统一，断言面零弱化；无结构性偏差——plan 行号引用（如 CONFIGURATION :165-166/:176 区域）按现行文件位置定位（实际 :155/:160），意图零歧义。

## Issues Encountered

None——14-07 LOADDATA 数据源就绪（plan 阻塞前提未触发）；herdr 配方/FEATURES 裁决/RESEARCH 措辞红线三源齐备。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- PC-12 三件套落地（README 主承载 / ARCHITECTURE 双模式段+误记修正 / CONFIGURATION 语义行）；**勾选留 14-12**（共享 ID 门 #2388——14-12-PLAN 同声明 PC-12/PC-13 且无 SUMMARY）
- mermaid 渲染目检归 14-12 收口闸/verify-work 人工复核（本机禁浏览器——结构化校验已过：节点/边/subgraph 配对）
- 14-12 diff 白名单应含本 plan 三提交的三文件改动（README 会话模式节纯新增+取整修正 / ARCHITECTURE 双模式段与 :7 改写行 / CONFIGURATION max-clients 行改写）
- REQUIREMENTS.md D-15 历史注记零触碰（红线保持）

## Known Stubs

None——零 stub/TODO/占位表述；全部数据可溯 14-07 LOADDATA、配方可溯 phase14.mjs 实证（文档即被测物）。

## Self-Check: PASSED

- 三文件在盘且 diff 白名单精确（README +58/-1 / ARCHITECTURE +54/-1 / CONFIGURATION +1/-1）
- 3 个提交在库（d364349 / b0bc487 / 604f5b3，`git log --oneline` 核实）
- 两任务 `<verify>` grep 命令复跑全过；全部 acceptance criteria 复验通过（含三重负断言与 30 项数据核对）
- REQUIREMENTS.md 零 diff（D-15 红线）；工作树零未跟踪文件残留
