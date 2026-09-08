# Phase 14: 双模式验证矩阵、标定与 herdr UAT - Context

**Gathered:** 2026-09-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 14 交付 v1.1 的验证与文档收口：① **-race 双模式全量 Go 测试 CI 门**——三维归类（mode-agnostic 同断言 / mode-mapped 断言分叉表 / mode-exclusive 显式断言未装配）经 `newTestServer(t, mode)` + `t.Run` 子测试双跑落地；② **零回归双证据**——phase02-09 既有协议 UAT 默认 shared 零修改重跑全过（经统一 runner 脚本一键矩阵 + 人工收口闸），shared 列期望值逐字未动 diff 审查；③ **herdr driving scenario E2E**——per-client 模式下移动端小屏 attach 后桌面端面板不被压缩（herdr is_foreground + per-client area 仲裁恢复），Linux 协议层双层断言 + Windows Playwright 观感断言；④ **per-client 负载矩阵**（1/4/16/32 会话洪水+驻留双剖面）实测回填 maxClients 建议值与 README 资源义务段；⑤ **PC-12 模式语义文档**——README/CONFIGURATION/ARCHITECTURE 三文档补段 + GoTTY 误记修正。

**In scope (from ROADMAP):** PC-12（模式语义文档三文档 + GoTTY 误记修正）、PC-13（herdr/tmux driving scenario 多客户端互不干扰 + 协议层 UAT + Windows Playwright 全链观感）；SC1 三维归类与 CI 门、SC4 负载矩阵标定回填。

**Out of scope (本阶段不做):** 任何产品代码行为变更（本 phase 零功能改动——验证与文档 only；例外面仅测试 harness 重构与 load_test.go 扩展）；v1.1.0 发布动作（milestone 收口走 /gsd:complete-milestone + release.sh 独立执行）；pinger 区分写阻塞/pong 超时（13-CONTEXT D-10 已接受 1006 语义，保持 deferred）；令牌桶/dwell 调优入口（12/13-CONTEXT 已锁内部常量，负载数据如需调整走常量改值非契约变更）；tmux 对照 UAT（仅文档一句对照说明）。

**已锁定不重复决策（继承，下游直接执行）：** 零回归双证据口径（shared 原样绿 + 期望值逐字未动，禁止断言放宽成「两模式都接受」）；双机测试拓扑（Linux 协议层 headless 禁浏览器 / Windows Playwright 经 TCP 转发器——CODEBUDDY.md 硬约束）；load_test.go 夹具纪律（LOADDATA 行 / 差值断言 / //go:build load 隔离 / churn 格 13-07 已建）；dwell 10s 与 1006 先杀语义（13-CONTEXT D-10，README/CONFIGURATION 段已落）；per-client 全部功能语义（PC-01~PC-11、SEC-09、OPS-12 已收口，本 phase 零改动）。

</domain>

<decisions>
## Implementation Decisions

### 三维归类与 -race 双模式 CI 门（SC1）
- **D-01:** 执行机械 = **PITFALLS :264 推荐形态逐字落地**——`newTestServer(t, mode)` 单一装配点（收编 startTrackedServerWith / startPerClientServer 两族），每个 mode-agnostic / mode-mapped 测试内部 `t.Run("mode=shared"/"mode=per-client")` 子测试双跑，断言分叉点显式写成表（mode → expected）；CI **结构零新 leg**——`go test -race -count=1 -v ./...` 单 step 天然双模式覆盖，darwin leg 自动同形态（Q1 裁决所需 -v 保留）。否决 env 变量全量重跑（CI 时长 ×2 且 mode-mapped 分叉仍逃不掉）与混合形态 — **Reversibility:** costly — 40+ 测试文件改造落地且被 CI 双模式绿锁定后，改执行机械需逐文件重写
- **D-02:** 归属表纪律 = **蓝本为准 + 偏差登记**——PITFALLS :378-399 逐文件归类表为改造蓝本；实测发现归属误判（如某「同断言」测试实际两模式行为不同）时在 plan 偏差登记 + 本 CONTEXT 回写，不静默跟改。验收闸 = 双模式 -race 全绿 + shared 列期望值逐字未动 git diff 白名单审查（12-05/13-08 先例）
  - **D-02 偏差登记（14-04 规划修订期实证；执行期偏差亦续登此处）**——逐则登记（14-06 收口核对清单引用本处登记，不重复登记；每则含蓝本行原文要点、实测归属、处置面、证据命令；1-3 = 14-04 规划修订期归属误记/双面分置，4-6 = 14-06 执行期单跑判定/结构性例外）：
    1. **蓝本 :391 行「resize_arb_test / TestGlobalCredit / owner 递补（clients_test 大部）｜shared-only」的 owner 递补归属误记**——owner 四测实在 multi_test.go:541/647/746/850（TestOwnerPolicy / TestAllPolicy / TestSuccession / TestSuccessionKickRace），D-03 未装配列由 14-02 承载（四测 mode=per-client 列已落地，零跳过形态）；clients_test.go 三测（TestClientCountInvariant:29 / TestWriterMergeControlFramesOnly:96 / TestAfterDrainResendsDims:150）实测判定为纯白盒（package server，registry/writer/&Server{} 直构造，零 server 装配），保持单跑。证据：`grep -n '^func Test' internal/server/multi_test.go internal/server/clients_test.go`。
    2. **蓝本 :388 行「resize_test（单端 last-wins）｜双模式断言分叉」的 wire 面归属误记**——resize 分叉的 wire 级行为断言唯 resize_arb_test.go 承载（TestResizeArbitration，14-04 双模式落地：shared 四子测试仲裁语义逐字 + per-client 列直通/无 min-rect/零 'W' 帧可证伪）；resize_test.go 三测（TestArbitrate:26 / TestSessionDimsLocked:87 / TestPushSessionDimsKickRecalc:123）实测判定为 arbitrate() 纯函数白盒测（零 server 装配、无 mode 参数可参数化），保持单跑——仲裁器「不装配」的 wire 级防线由 resize_arb_test.go per-client 列单一承载（防双写漂移）。证据：`grep -n '^func Test' internal/server/resize_test.go internal/server/resize_arb_test.go`。
    3. **蓝本 :389 行「multi_test（fanout/MaxClients/计数不变量）｜双模式断言分叉」的计数不变量双面分置**——白盒面 = clients_test.go TestClientCountInvariant:29（registry 计数对称不变量机械锁定，纯白盒单跑）；wire 面 = multi_test.go TestMaxClients503:1222（满员 503 闸 + 槽位释放，14-02 已双跑含 spawn-intent 口径程序序对照）。clients_test.go / resize_test.go 两文件零改动（形态判定，非覆盖缺口）。证据：`grep -n 'TestClientCountInvariant' internal/server/clients_test.go && grep -n 'func TestMaxClients503' internal/server/multi_test.go`。
    4. **蓝本 :392 行「exitmsg_test / log_test / events_test｜双模式同断言（扩展后）」的 events 部分实测处置 = 保持单跑（14-06 执行期偏差登记）**——events_test.go 现存 9 测：6 测 shared 形态 server 装配测（TestAttachDetachEvents/TestDetachReason/TestSessionEnd/TestShutdownEvent/TestThrottledRetryAfter/TestAuthFailedNoUsername，startTrackedServerWith + 本地事件变体族装配）+ 3 测 per-client 形态（TestPerClientSessionEnd/TestPerClientSessionStart/TestSpawnEventsSchema——Phase 13 落地，即蓝本「扩展后锁定」的落地产物）。判定依据：per-client 事件 schema 面（client_id 关联/signal 归因/spawn 拒绝事件键集）已由同文件三测承载；shared 侧事件 emit 点在 Attach 入口/读循环/lifecycle（与进程模型正交的同码路径），双跑仅复跑既有断言无增量覆盖——9 测 ×2 装配成本无对应覆盖收益。证据：`grep -c '^func Test' internal/server/events_test.go`（=9）。
    5. **蓝本 :392 行的 log 部分实测处置 = 保持单跑（14-06 执行期偏差登记，同 ⑤ 类）**——log_test.go 1 测 TestLogEventJSON（:69 startTrackedServerWith 装配）：logEvent 迁移 slog JSONHandler 的端到端基座（D-13/D-18 schema 六键），事件面与进程模型正交（emit 同码路径），与 events 同判定——per-client 侧 schema 面由 events_test 三测 + proxy_e2e 四测（14-06 双跑，remote_user 键/四段 schema）承载。证据：`grep -n 'startTrackedServerWith' internal/server/log_test.go`（=:69 单调用点）。
    6. **蓝本 :382 行「handshake_test / limits_test / keepalive_test｜双模式同断言」的 limits 例外——TestReadLimitBoundary 保持 shared 单跑（14-01 执行期偏差，此处补登）**——startRawCatServer 的装配前提 = net.Listen 之前经 master fd 同步 stty raw（消除子进程 stty 启动窗口的双重回显竞态）；per-client 的 spawn 发生在 attach 期（Listen 之后），无 pre-listen 窗口可用——装配结构性不等价，强行双跑即装配语义漂移。边界值断言本体（16384 通过/16385 切断）与进程模型无关，per-client 侧同值面由 TestOversize1009/TestPreHelloReadLimit 双跑承载（limits_test.go:186-196 文件内注释已载明，14-01 SUMMARY/STATE 已登记——本条为 CONTEXT 双通道纪律的补登面）。证据：`grep -n 'startRawCatServer' internal/server/limits_test.go`（:45 定义 + TestReadLimitBoundary 调用）。
- **D-03:** mode-exclusive（shared-only）测试的 per-client 分支 = **显式断言未装配**——如仲裁器测试在 per-client 下断言 resize 直通无 min-rect 约束、线上零 'W' 帧；把「不装配」做成可证伪断言（有人误在 per-client 装配仲裁器时测试翻红），否决 t.Skip（腐化无信号）
- **D-04:** UAT 零修改重跑形式化 = **统一 runner 脚本（web/uat/run-all.mjs 类）+ 人工收口闸**——10 个既有协议脚本 + jsdom 套件 + per-client 三脚本串成一键矩阵；协议 UAT **不进 CI**（真实二进制 spawn 的环境敏感度/flake 面/时长不合算；pw 层永远 Windows 侧进不了 CI——双机拓扑硬约束）

### herdr driving scenario E2E（SC3 / PC-13）
- **D-05:** 载具 = **仅真 herdr**（本机 `~/.local/bin/herdr` 0.8.100）——herdr 的 per-client area 渲染是「恢复正确行为」的唯一可实证面；tmux 多客户端 attach 仍取 min-size（tmux 自身语义，window-size 可配但默认压缩），**不能**作为恢复证据，只在 README 模式语义段一句对照说明。否决 herdr+tmux 对照双跑（复杂度翻倍 + tmux 压缩形态随配置漂移 flake 风险）
- **D-06:** Linux 协议层断言深度 = **双层断言 + API 观测**——wesh 层（双端双 pid + winsize 隔离）+ herdr 行为层（经 herdr socket API / `herdr api` 结构化通道断言 server 视角双 client 各自 area 尺寸、移动端 attach 后桌面端 area 不变）。否决终端输出嗅探（herdr 版本漂移改渲染格式即碎）。**herdr 观测通道需先 spike 标定**（ROADMAP Research flag 既定——规划前以 spike 实测 API 响应形态定断言行）
- **D-07:** Windows Playwright 层 = **xterm buffer 文本/光标位置断言为主 + 截图留档人工复核**——大屏 tab 的 herdr 面板边框列位置在小屏 tab attach + resize 后逐字不变（phase06-pw 六项 46/46 先例形态）；像素视觉豁免（CODEBUDDY.md 测试策略 §5）。否决截图 diff 回归（herdr 渲染含时钟/状态闪烁面，flake 风险高）
- **D-08:** 场景集 = **driving + ro 汇聚**——① 核心 driving：桌面端 120x40 attach → 移动端 40x12 attach → 桌面端 area 不变；移动端 resize（转屏/拖窗）→ 仍不变；② ro 汇聚：ro 分享链接移动端 attach 经 herdr 汇聚同会话、输入被 wesh 门控（FEATURES 裁决 7 文档叙事的实证防线——防文档说谎）；is_foreground 仲裁证据若 socket API 现成则带上，不强求
- **D-09:** herdr 环境纪律 = **独立会话/socket 隔离 + 版本钉定**——UAT 脚本起独立 herdr 会话（`herdr --session wesh-uat-*` 形态）+ 跑完显式清理（会话销毁/server stop），零污染用户日常 herdr server；断言基于 herdr 0.8.100 API 形态，版本漂移处置纪律写进 UAT 文档（失败先核 `herdr --version`）

### 并发进程负载矩阵与标定回填（SC4）
- **D-10:** 矩阵剖面 = **洪水 + 驻留双剖面**——N ∈ {1,4,16,32} × 每会话 seq 洪水（gatedFloodArgv 触发式先例复用）断言吞吐（每端收流完整）+ 峰值内存/fd/goroutine 有界；N 会话驻留空转格实证 idle 账面（32 shell 驻留 ~160MB 账面推算的实测证真/证伪）。churn 格 13-07 已建不重复建设
- **D-11:** maxClients=32 默认值 **不动**（零公开契约变更）——实测数据回填 README 资源义务段 + 按部署形态建议值表（如「低配 VPS 建议 --max-clients=8」）；**仅当数据证伪**（32 会话资源超不可接受界线）时才开 one-way 确认门动默认值（13-01 D-01 先例形态：用户派发确认）。否决「未实测先改小」（若 32 实测可接受则白付契约变更成本）与「启动 warn」（噪音面）
- **D-12:** 驻留格内存断言口径 = **wesh 侧硬断言 + 子进程观测**——Alloc 增量 ≤ N×(768KiB outbox+inputQ 账面 + ε 容差)（churn 格双采样差值先例）；子进程 /proc/<pid>/status VmRSS 采样入 LOADDATA 观测记录 + 宽松上界防线（≤15MB/进程——防 wesh 侧大 env/驻留缓冲注入子进程的泄漏面；bash 自身 RSS 环境因子不硬断）

### 协议层 UAT 增量与模式语义文档（SC2 / SC5 / PC-12）
- **D-13:** SC2 增量形态 = **phase14.mjs = herdr 协议层脚本**（driving + ro 汇聚 + wesh 层复核）——ROADMAP SC2 所列六项（双 pid / EXIT 不串台 / resize 隔离 / ro 门控 / --once 255 / spawn 失败 1011）已被 phase11.mjs（21/21）、phase12.mjs（20/20）、phase13.mjs（29/29）逐一覆盖，**不重复断言**——三脚本经 run-all 重跑即 SC2 证据，避免 UAT 断言双写漂移
- **D-14:** PC-12 文档分布 = **README 主承载 + 双 docs 补段**——README 新增「会话模式」节（模式语义表：分享链接=按权限级别的独立进程入场券 / ro=自有进程输入门控 / 配合 herdr·tmux 经多路复用汇聚 + herdr 配方示例 + 资源义务段含实测标定表）；`docs/ARCHITECTURE.md` 补双模式架构段（含 goroutine 拓扑 mermaid 图——项目文档规则既定）+ **:7 GoTTY 误记修正**（「GoTTY 式共享进程模型」→ GoTTY 实为 per-connection spawn 的修正表述 + 双模式分岔说明）；`docs/CONFIGURATION.md` 补 max-clients per-client 语义行。否决独立 SESSION-MODES.md（模式语义是 v1.1 核心卖点，二级文档降低可达性）。**REQUIREMENTS.md:50 D-15 历史注记不改写**（裁决档案保持当时语境）

### Claude's Discretion
- `newTestServer(t, mode)` 精确签名与两装配族（startTrackedServerWith / startPerClientServer）的收编形态；40+ 文件改造的 wave 切片（建议按归属类别分批：mode-agnostic → mode-mapped → mode-exclusive）
- LOADDATA 行格式 per-client 扩展字段与 README 标定表回填精确格式（09-09 D-13 先例表格形态沿用）
- herdr socket API 具体调用面与断言行（**spike 标定产物**——规划前先做 herdr 观测通道 spike：独立会话起 herdr、attach 双 client、`herdr api` 查 client/area 尺寸，确定稳定断言字段）
- phase14-pw.mjs 断言行精确颗粒度、TCP 转发器拓扑复用形态（phase06-pw/phase12-pw 载具先例）、双 tab viewport 尺寸选型
- run-all.mjs 脚本形态（串行执行/逐脚本 exit code 聚合/结果汇总表输出）
- ARCHITECTURE.md 双模式段 mermaid 图精确内容与 :7 误记改写措辞
- herdr 独立会话/socket 的精确隔离形态（--session 名 vs 环境变量 socket 路径）与清理序列
- 负载矩阵 darwin 口径（沿用先例：load tag Linux 手动跑，darwin 面 skip；P9 kqueue N 规模由 CI macOS leg 常规测试承担）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 14 — 成功准则 5 条（-race 双模式 CI 门三维归类 / 协议层 per-client 全链 UAT / herdr driving scenario / 负载矩阵 1/4/16/32 回填 / 模式语义文档 + GoTTY 误记修正）与 Research flag（32 会话资源曲线账面推算 MEDIUM 置信 + herdr 断言设计需实测标定）
- `.planning/REQUIREMENTS.md` §PC-12/PC-13（:91-92）— 需求原文
- `.planning/PROJECT.md` §Current Milestone v1.1 — 里程碑目标；Key Decisions 表（accept-255 / 双默认值先例形态）

### v1.1 调研结论（设计蓝本）
- `.planning/research/PITFALLS.md` — **P11 三维归类（:255-275，D-01/D-02/D-03 的蓝本本体）+ Looks-Done 验证清单（:340-349）+ 既有测试双模式归属表（:378-399，逐文件改造蓝本）**；Performance Traps 表（:309-316，驻留账面推算源）
- `.planning/research/ARCHITECTURE.md` §10（:475-486，资源标定账面表——「32 并发 shell 是重负载」文档义务源）§12 PC-4（:529，负载矩阵/文档义务枚举）
- `.planning/research/FEATURES.md` — §裁决 6/7（ro=自有进程输入门控 / 分享链接=独立进程入场券——D-08 ro 汇聚场景与 D-14 语义表的叙事源）+ T11 文档要求 + herdr E2E 依赖图（:108）+ keep/degrade/vanish 映射（:111-135，ARCHITECTURE.md 双模式段的素材）
- `.planning/research/SUMMARY.md` — 方法论警告（最大风险 = 破坏既有不变量而不自知）

### 前序 phase 决策（机制先例与边界）
- `.planning/phases/13-resource-defense/13-CONTEXT.md` — D-01 one-way 门先例（D-11 的处置形态母本）/ D-03 内部常量纪律 / churn 格断言三面先例 / Deferred（本 phase 全部范围来源清单）
- `.planning/phases/12-per-client/12-CONTEXT.md` — D-03 内部常量+测试覆写先例 / D-13 验证面切片先例（Playwright 归 Phase 14 的原始裁决）/ pinger/dwell 竞态闭合记录
- `.planning/phases/11-per-client/11-CONTEXT.md` — D-05/D-06 验证面切片先例（perclient_test.go 归属 + phaseNN.mjs 母本）
- `.planning/milestones/v1.0-phases/` 06/07/09 CONTEXT — EXIT 直写纪律 / warn 通道先例 / 标定方法论（README 231-243 矩阵形状与 LOADDATA 回填链）

### 现状代码（扩展点，file:line 实证）
- `internal/server/load_test.go` — 负载矩阵夹具层全套先例（drainClient/drainRateLimited/awaitDrain/scrapePeakSampler/allocPeakSampler/gatedFloodArgv 触发式洪水/LOADDATA 行格式）+ TestChurnPerClientSpawnThrottle（:725，churn 格双采样差值断言母本——D-12 的直接先例）；per-client 洪水/驻留格在此文件扩展
- `internal/server/e2e_test.go:120-134` — startTrackedServerWith + killServer（shared 装配族母本，newTestServer 收编对象）
- `internal/server/perclient_test.go` — startPerClientServer / startPerClientServerWithSpawn（per-client 装配族母本）；Phase 11/12 全部新测归属文件（三维归类改造的主战场）
- `.github/workflows/ci.yml:17` — go leg 现状（-race -count=1 -v；D-01 下结构零改动，双模式经 t.Run 自动覆盖）；fuzz leg 零改动
- `web/uat/phase11.mjs / phase12.mjs / phase13.mjs` — per-client 协议层 UAT 三脚本（SC2 六项证据的承载体，run-all 重跑对象；phase14.mjs 同构母本）
- `web/uat/phase02-09.mjs` + `web/uat/phase04-dom/05-dom/06-dom/12-dom.mjs` — 零修改重跑矩阵全清单（runner 脚本枚举源）
- `web/uat/pw/` — phase06-pw.mjs（断网重连 46/46 先例）/ phase12-pw.mjs（resize 观感先例）/ phase07-a2 / phase09-caddy 双机载具（phase14-pw.mjs 母本；TCP 转发器 kill/restore 先例）
- `README.md:98` / `docs/CONFIGURATION.md:176` — 1006 先杀时序段已落（D-14 文档段的同位邻居）
- `docs/ARCHITECTURE.md:7` — GoTTY 误记修正点（D-14）
- `~/open_src/herdr`（herdr 源码）+ `~/.local/bin/herdr` 0.8.100 — driving 载具；socket API 形态以 spike 标定为准

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `load_test.go 夹具层`（drainClient / drainRateLimited / scrapePeakSampler / allocPeakSampler / gatedFloodArgv / LOADDATA 格式）— D-10 双剖面格的全部构件既有；per-client 格只需换 startPerClientServer 装配与断言面
- `startPerClientServer / startPerClientServerWithSpawn`（perclient_test.go，11-03 已参数化）— newTestServer(t, mode) 的 per-client 分支现成；shared 分支 = startTrackedServerWith 直传
- `herdr socket API / herdr api 子命令`（herdr 0.8.100 既有）— D-06 herdr 行为层断言的结构化观测通道（spike 标定后定断言行）
- `RawStallClient`（phase12.mjs 一般化停读夹具）与 pw 双机载具（转发器 ctl 脚本先例）— phase14.mjs / phase14-pw.mjs 的现成件
- `getMetrics/metricSample + wesh_goroutines/wesh_mem_alloc_bytes/wesh_session_active/wesh_pty_spawn_*`（Phase 8/13 观测面）— 负载矩阵与 churn 式差值断言的全部数据源既有
- `/proc/<pid>/status VmRSS` 读取（readProcState 同文件 /proc 先例）— D-12 子进程观测通道

### Established Patterns
- **三维归类逐文件蓝本**（PITFALLS :378-399）— 改造期按表施工；mode-mapped 断言分叉表形态 = t.Run 内 mode → expected 显式表，shared 列期望值与 v1.0 逐字一致（零回归证据本体）
- **「显式断言未装配」防线**（D-03）— mode-exclusive 测试 per-client 分支断言组件不装配（零 'W' 帧 / resize 无仲裁），防装配漂移腐化
- **LOADDATA + 标定表回填链**（09-06 D-11/D-12 → 09-09 D-13）— 每格产出 LOADDATA 行 → README 标定表数据源；「验证为主、证伪才改」纪律（默认验证现值成立，数据证伪才动常量/契约）
- **双采样差值 + 容差断言**（TestChurnPerClientSpawnThrottle / TestLoadMemoryBound）— 禁无基线对照的单个硬编码绝对上限（Pitfall 7）；回收窗口轮询禁固定 sleep 精确时点
- **one-way 确认门**（13-01 D-01 用户派发确认 option-a 先例）— D-11 数据证伪需动默认值时的唯一通道
- **零回归双证据口径**— shared 全量原样绿 + phase02-09 UAT 默认模式零修改重跑 + 期望值逐字未动 diff 白名单审查；本 phase 由 t.Run 双跑 + run-all.mjs 形式化

### Integration Points
- `newTestServer(t, mode)`（新建，建议归 export_test.go 或新 harness_test.go）— 两装配族收编点；mode-mapped 文件的断言分叉表经此参数化
- `load_test.go` — 新增 per-client 洪水矩阵格（N×seq 每会话独立洪水）与驻留格（N 会话 idle 驻留账面）；//go:build load 隔离与 Linux 手动跑纪律不变
- `web/uat/phase14.mjs`（新建）— herdr 协议层双层断言脚本（D-06/D-08）；`web/uat/pw/phase14-pw.mjs`（新建）— 双 tab 观感断言（D-07）
- `web/uat/run-all.mjs`（新建）— 全量 UAT 矩阵 runner（D-04）
- `README.md`（新增「会话模式」节 + 资源义务段/标定表回填）/ `docs/ARCHITECTURE.md`（双模式段 + :7 修正）/ `docs/CONFIGURATION.md`（max-clients per-client 语义行）— D-14 三文档落点
- `ci.yml` — **零改动**（t.Run 双跑天然覆盖，D-01）；收口闸六段式惯例沿用

</code_context>

<specifics>
## Specific Ideas

- **tmux 不能作为恢复证据的语义分析**——tmux 多客户端 attach 同会话时窗口尺寸默认取所有客户端最小值（window-size smallest 语义）；per-client 模式下每个 WS 客户端 spawn 独立 tmux client，但 tmux server 侧仍按 min-size 压缩——「移动端压缩桌面端」在 tmux 下是 tmux 自身行为而非 wesh 缺陷。herdr 的 per-client area 渲染（每 client 独立渲染尺寸）才是 driving bug 的修复实证面。README 模式语义段用一句对照说明此差异（防用户拿 tmux 复测误判 wesh 失效）
- **「不重复断言」是 D-13 的核心论证**——SC2 六项若在新脚本重断一遍，同一行为出现两处断言源，未来行为调整需双改（phase05 期望漂移教训的同类）；run-all 重跑三脚本即「全链断言全过」的矩阵证据，phase14.mjs 只承载 herdr 独有增量
- **D-11 的「数据证伪才开 one-way 门」两段式**——本 phase 默认动作是零契约变更（文档回填建议值）；只有当 32 会话实测（驻留格 RSS / 洪水格内存）超不可接受界线时才走 13-01 先例的用户派发确认动默认值。负载矩阵因此是「默认 32 是否成立」的实证裁决工具，而非纯粹观测
- **spike 先行是 herdr 断言面的前提**——D-06 的 socket API 观测通道在规划前必须经 spike 实证（独立会话起 herdr → 双 client attach → api 查 area 尺寸的形态与字段名），否则 plan 的断言行是账面推算。spike 产物（观测通道形态 + 稳定字段清单）进 14-RESEARCH 或 plan 前置任务
- **驻留格的 ε 容差来源**——768KiB = outbox 512KiB + inputQ 256KiB 账面（ARCHITECTURE §10）；ε 容纳 per-client 控制结构（pcSession/goroutine 栈/令牌桶 map 条目），标定参照 churn 格 +8 goroutine 容差的推导形态（实测首跑定值并注释论证）

</specifics>

<deferred>
## Deferred Ideas

- **pinger 区分写阻塞/pong 超时**——13-CONTEXT D-10 已接受 1006 语义；herdr UAT 实证中若自管 socket 客户端受害场景出现，回写重开（herdr 实测恰是该 deferred 的证伪通道）
- **令牌桶参数调优入口 / dwell 阈值调优**——12/13-CONTEXT 既定；负载矩阵数据如需调整走常量改值（非公开契约变更）
- **v1.1.0 发布闸**——非本 phase 范围；Phase 14 收口后走 /gsd:complete-milestone + scripts/release.sh 独立执行（Phase 9 发布链先例）
- **tmux 对照 UAT 脚本**——D-05 裁决文档一句对照替代；若未来 tmux window-size 行为实证需求出现再评
- **后台标签页 1013 后自动重连体验 / 重连「新会话」提示文案**——12-CONTEXT deferred 原样保持（UX 迭代候选，非本里程碑）
- **maxClients per-client 默认值变更**——D-11 两段式的后半段：仅负载数据证伪时经 one-way 确认门执行

</deferred>

---

*Phase: 14-herdr-uat*
*Context gathered: 2026-09-06*
