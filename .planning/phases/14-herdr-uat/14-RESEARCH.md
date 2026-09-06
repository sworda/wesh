# Phase 14: 双模式验证矩阵、标定与 herdr UAT - Research

**Researched:** 2026-09-06
**Domain:** Go 测试 harness 双模式改造 / Node 协议层 UAT / herdr 多路复用 E2E / 负载标定
**Confidence:** HIGH（最大风险面 herdr 断言设计已经两轮 live spike + 一轮 wesh×herdr 全链 smoke 实证钉定）

## Summary

本 phase 是 v1.1 的验证与文档收口，**零产品代码变更**（例外面仅测试 harness 重构与 load_test.go 扩展）。四个工作面中三个是既有夹具/先例的组装：三维归类改造有 PITFALLS :378-399 逐文件蓝本；负载矩阵有 load_test.go 全套夹具（drainClient/gatedFloodArgv/loadSamplers/LOADDATA）；协议层 UAT 有 phase11/12/13.mjs 三代同构母本。唯一真正的未知面——**herdr 观测通道**——已按 ROADMAP Research flag 与 14-CONTEXT D-06 的要求完成 spike 标定（本研究 §Architecture Patterns 第 3 节为标定产物）。

Spike 核心结论：herdr 0.8.100 的 per-client area 渲染经 `herdr api snapshot` 的 `layouts[].area` 结构化可观测——该区域恒等于**前台（is_foreground，last-activity-wins）客户端**的 pane 面积，移动端 attach 后翻转、桌面端输入后翻回，翻转链即「仲裁恢复生效」的可证伪证据；桌面端字节流在小屏 attach/resize 全程只收小增量帧（无压缩重渲染），流层与 API 层双通道互证。wesh×herdr 全链 smoke（per-client 模式真实二进制 + 真实 herdr 会话）一次通过：双 pid、Welcome session=per-client、ro ticket 全链门控（`herdr pane read` 观测）、herdr server 在 wesh 死后存活、`herdr session stop` 清理可靠。

**Primary recommendation:** 按 D-01..D-14 既定决策直接施工；herdr 断言行以本研究 spike 标定的翻转链（§Patterns-3 断言表）为准，几何断言用**关系断言**（area 属于两客户端几何之一 + 翻转）而非绝对像素常量（用户 config.toml 共享、版本漂移面）；负载矩阵 per-client 洪水格每客户端各自发送触发 INPUT（每会话独立 stdin 是 shared 格的单端触发先例必须调整的点）。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**三维归类与 -race 双模式 CI 门（SC1）**
- **D-01:** 执行机械 = **PITFALLS :264 推荐形态逐字落地**——`newTestServer(t, mode)` 单一装配点（收编 startTrackedServerWith / startPerClientServer 两族），每个 mode-agnostic / mode-mapped 测试内部 `t.Run("mode=shared"/"mode=per-client")` 子测试双跑，断言分叉点显式写成表（mode → expected）；CI **结构零新 leg**——`go test -race -count=1 -v ./...` 单 step 天然双模式覆盖，darwin leg 自动同形态（Q1 裁决所需 -v 保留）。否决 env 变量全量重跑（CI 时长 ×2 且 mode-mapped 分叉仍逃不掉）与混合形态 — **Reversibility:** costly — 40+ 测试文件改造落地且被 CI 双模式绿锁定后，改执行机械需逐文件重写
- **D-02:** 归属表纪律 = **蓝本为准 + 偏差登记**——PITFALLS :378-399 逐文件归类表为改造蓝本；实测发现归属误判（如某「同断言」测试实际两模式行为不同）时在 plan 偏差登记 + 本 CONTEXT 回写，不静默跟改。验收闸 = 双模式 -race 全绿 + shared 列期望值逐字未动 git diff 白名单审查（12-05/13-08 先例）
- **D-03:** mode-exclusive（shared-only）测试的 per-client 分支 = **显式断言未装配**——如仲裁器测试在 per-client 下断言 resize 直通无 min-rect 约束、线上零 'W' 帧；把「不装配」做成可证伪断言（有人误在 per-client 装配仲裁器时测试翻红），否决 t.Skip（腐化无信号）
- **D-04:** UAT 零修改重跑形式化 = **统一 runner 脚本（web/uat/run-all.mjs 类）+ 人工收口闸**——10 个既有协议脚本 + jsdom 套件 + per-client 三脚本串成一键矩阵；协议 UAT **不进 CI**（真实二进制 spawn 的环境敏感度/flake 面/时长不合算；pw 层永远 Windows 侧进不了 CI——双机拓扑硬约束）

**herdr driving scenario E2E（SC3 / PC-13）**
- **D-05:** 载具 = **仅真 herdr**（本机 `~/.local/bin/herdr` 0.8.100）——herdr 的 per-client area 渲染是「恢复正确行为」的唯一可实证面；tmux 多客户端 attach 仍取 min-size（tmux 自身语义，window-size 可配但默认压缩），**不能**作为恢复证据，只在 README 模式语义段一句对照说明。否决 herdr+tmux 对照双跑（复杂度翻倍 + tmux 压缩形态随配置漂移 flake 风险）
- **D-06:** Linux 协议层断言深度 = **双层断言 + API 观测**——wesh 层（双端双 pid + winsize 隔离）+ herdr 行为层（经 herdr socket API / `herdr api` 结构化通道断言 server 视角双 client 各自 area 尺寸、移动端 attach 后桌面端 area 不变）。否决终端输出嗅探（herdr 版本漂移改渲染格式即碎）。**herdr 观测通道需先 spike 标定**（ROADMAP Research flag 既定——规划前以 spike 实测 API 响应形态定断言行）
- **D-07:** Windows Playwright 层 = **xterm buffer 文本/光标位置断言为主 + 截图留档人工复核**——大屏 tab 的 herdr 面板边框列位置在小屏 tab attach + resize 后逐字不变（phase06-pw 六项 46/46 先例形态）；像素视觉豁免（CODEBUDDY.md 测试策略 §5）。否决截图 diff 回归（herdr 渲染含时钟/状态闪烁面，flake 风险高）
- **D-08:** 场景集 = **driving + ro 汇聚**——① 核心 driving：桌面端 120x40 attach → 移动端 40x12 attach → 桌面端 area 不变；移动端 resize（转屏/拖窗）→ 仍不变；② ro 汇聚：ro 分享链接移动端 attach 经 herdr 汇聚同会话、输入被 wesh 门控（FEATURES 裁决 7 文档叙事的实证防线——防文档说谎）；is_foreground 仲裁证据若 socket API 现成则带上，不强求
- **D-09:** herdr 环境纪律 = **独立会话/socket 隔离 + 版本钉定**——UAT 脚本起独立 herdr 会话（`herdr --session wesh-uat-*` 形态）+ 跑完显式清理（会话销毁/server stop），零污染用户日常 herdr server；断言基于 herdr 0.8.100 API 形态，版本漂移处置纪律写进 UAT 文档（失败先核 `herdr --version`）

**并发进程负载矩阵与标定回填（SC4）**
- **D-10:** 矩阵剖面 = **洪水 + 驻留双剖面**——N ∈ {1,4,16,32} × 每会话 seq 洪水（gatedFloodArgv 触发式先例复用）断言吞吐（每端收流完整）+ 峰值内存/fd/goroutine 有界；N 会话驻留空转格实证 idle 账面（32 shell 驻留 ~160MB 账面推算的实测证真/证伪）。churn 格 13-07 已建不重复建设
- **D-11:** maxClients=32 默认值 **不动**（零公开契约变更）——实测数据回填 README 资源义务段 + 按部署形态建议值表（如「低配 VPS 建议 --max-clients=8」）；**仅当数据证伪**（32 会话资源超不可接受界线）时才开 one-way 确认门动默认值（13-01 D-01 先例形态：用户派发确认）。否决「未实测先改小」（若 32 实测可接受则白付契约变更成本）与「启动 warn」（噪音面）
- **D-12:** 驻留格内存断言口径 = **wesh 侧硬断言 + 子进程观测**——Alloc 增量 ≤ N×(768KiB outbox+inputQ 账面 + ε 容差)（churn 格双采样差值先例）；子进程 /proc/<pid>/status VmRSS 采样入 LOADDATA 观测记录 + 宽松上界防线（≤15MB/进程——防 wesh 侧大 env/驻留缓冲注入子进程的泄漏面；bash 自身 RSS 环境因子不硬断）

**协议层 UAT 增量与模式语义文档（SC2 / SC5 / PC-12）**
- **D-13:** SC2 增量形态 = **phase14.mjs = herdr 协议层脚本**（driving + ro 汇聚 + wesh 层复核）——ROADMAP SC2 所列六项（双 pid / EXIT 不串台 / resize 隔离 / ro 门控 / --once 255 / spawn 失败 1011）已被 phase11.mjs（21/21）、phase12.mjs（20/20）、phase13.mjs（29/29）逐一覆盖，**不重复断言**——三脚本经 run-all 重跑即 SC2 证据，避免 UAT 断言双写漂移
- **D-14:** PC-12 文档分布 = **README 主承载 + 双 docs 补段**——README 新增「会话模式」节（模式语义表：分享链接=按权限级别的独立进程入场券 / ro=自有进程输入门控 / 配合 herdr·tmux 经多路复用汇聚 + herdr 配方示例 + 资源义务段含实测标定表）；`docs/ARCHITECTURE.md` 补双模式架构段（含 goroutine 拓扑 mermaid 图——项目文档规则既定）+ **:7 GoTTY 误记修正**（「GoTTY 式共享进程模型」→ GoTTY 实为 per-connection spawn 的修正表述 + 双模式分岔说明）；`docs/CONFIGURATION.md` 补 max-clients per-client 语义行。否决独立 SESSION-MODES.md（模式语义是 v1.1 核心卖点，二级文档降低可达性）。**REQUIREMENTS.md:50 D-15 历史注记不改写**（裁决档案保持当时语境）

**已锁定不重复决策（继承，下游直接执行）：** 零回归双证据口径（shared 原样绿 + 期望值逐字未动，禁止断言放宽成「两模式都接受」）；双机测试拓扑（Linux 协议层 headless 禁浏览器 / Windows Playwright 经 TCP 转发器——CODEBUDDY.md 硬约束）；load_test.go 夹具纪律（LOADDATA 行 / 差值断言 / //go:build load 隔离 / churn 格 13-07 已建）；dwell 10s 与 1006 先杀语义（13-CONTEXT D-10，README/CONFIGURATION 段已落）；per-client 全部功能语义（PC-01~PC-11、SEC-09、OPS-12 已收口，本 phase 零改动）。

### Claude's Discretion
- `newTestServer(t, mode)` 精确签名与两装配族（startTrackedServerWith / startPerClientServer）的收编形态；40+ 文件改造的 wave 切片（建议按归属类别分批：mode-agnostic → mode-mapped → mode-exclusive）
- LOADDATA 行格式 per-client 扩展字段与 README 标定表回填精确格式（09-09 D-13 先例表格形态沿用）
- herdr socket API 具体调用面与断言行（**spike 标定产物**——规划前先做 herdr 观测通道 spike：独立会话起 herdr、attach 双 client、`herdr api` 查 client/area 尺寸，确定稳定断言字段）
- phase14-pw.mjs 断言行精确颗粒度、TCP 转发器拓扑复用形态（phase06-pw/phase12-pw 载具先例）、双 tab viewport 尺寸选型
- run-all.mjs 脚本形态（串行执行/逐脚本 exit code 聚合/结果汇总表输出）
- ARCHITECTURE.md 双模式段 mermaid 图精确内容与 :7 误记改写措辞
- herdr 独立会话/socket 的精确隔离形态（--session 名 vs 环境变量 socket 路径）与清理序列
- 负载矩阵 darwin 口径（沿用先例：load tag Linux 手动跑，darwin 面 skip；P9 kqueue N 规模由 CI macOS leg 常规测试承担）

### Deferred Ideas (OUT OF SCOPE)
- **pinger 区分写阻塞/pong 超时**——13-CONTEXT D-10 已接受 1006 语义；herdr UAT 实证中若自管 socket 客户端受害场景出现，回写重开（herdr 实测恰是该 deferred 的证伪通道）
- **令牌桶参数调优入口 / dwell 阈值调优**——12/13-CONTEXT 既定；负载矩阵数据如需调整走常量改值（非公开契约变更）
- **v1.1.0 发布闸**——非本 phase 范围；Phase 14 收口后走 /gsd:complete-milestone + scripts/release.sh 独立执行（Phase 9 发布链先例）
- **tmux 对照 UAT 脚本**——D-05 裁决文档一句对照替代；若未来 tmux window-size 行为实证需求出现再评
- **后台标签页 1013 后自动重连体验 / 重连「新会话」提示文案**——12-CONTEXT deferred 原样保持（UX 迭代候选，非本里程碑）
- **maxClients per-client 默认值变更**——D-11 两段式的后半段：仅负载数据证伪时经 one-way 确认门执行

**Out of scope (本阶段不做):** 任何产品代码行为变更（本 phase 零功能改动——验证与文档 only；例外面仅测试 harness 重构与 load_test.go 扩展）；v1.1.0 发布动作；pinger 区分写阻塞/pong 超时；令牌桶/dwell 调优入口；tmux 对照 UAT（仅文档一句对照说明）。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PC-12 | 模式语义文档：README/CONFIGURATION/ARCHITECTURE 补 per-client 模型段（分享链接=按权限级别的独立进程入场券、ro=自有进程输入门控、配合 herdr/tmux 时经多路复用汇聚）；修正 v1.0「GoTTY 式共享进程模型」误记 | D-14 三文档落点现状核实（§Architecture Patterns-5）：README :96-98 已有 session-mode 单段与保活先杀段（新节同位邻居）；ARCHITECTURE.md :7 误记原文已逐字核读；CONFIGURATION.md :176 默认值表 stop-timeout 双默认值行已落（max-clients 语义行同表补位）。tmux 对照句的实证依据见 §State of the Art（3.6 默认 window-size=latest，非 smallest——措辞须避免「默认取最小值」字面失实） |
| PC-13 | herdr/tmux 等多路复用应用场景下多客户端互不干扰：移动端 attach 不再压缩其他客户端面板尺寸（herdr is_foreground + per-client area 仲裁恢复生效）；协议层 UAT 断言进程独立/尺寸互不干扰 + Windows Playwright 全链观感断言 | herdr 观测通道 spike 标定完成（§Architecture Patterns-3）：`herdr api snapshot` layouts[].area 翻转链 + 流层增量帧双通道断言设计全部经 wesh×herdr 真实全链 smoke 实证；ro 汇聚场景的 ticket 全链（POST /api/attach → Hello 携 ticket）与 `herdr pane read` 观测通道实证可用；环境隔离形态（--session + HERDR_SOCKET_PATH + session stop 清理序列）实证可靠 |
</phase_requirements>

## Project Constraints (from CODEBUDDY.md)

- **双机拓扑硬约束**：协议层/Go 测试/负载格在 Linux 开发机（headless，**禁**装 GUI/X11/浏览器/playwright，禁在该侧起 wesh 等人工访问）；Playwright 浏览器全链仅在 Windows 工作站侧，经 TCP 转发器（kill/restore）连接，禁止操作真实网卡
- **pnpm 而非 npm**；构建命令带 `time` 前缀
- **文档规则**：技术文档用 mermaid 画 UML/流程图（ARCHITECTURE.md 双模式段 goroutine 拓扑图适用）
- 测试策略五层分层与平台原生行为显式豁免条款（§5）——UAT 以 `skipped` + reason 记录豁免项
- 修改前备份检查点（收口闸六段式惯例即项目级checkpoint形态）

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 三维归类 harness 改造（newTestServer 收编） | Go test 层（internal/server/*_test.go） | — | 纯测试代码重构，零生产面 |
| -race 双模式 CI 门 | GitHub Actions（.github/workflows/ci.yml） | — | D-01 结构零改动：单 step t.Run 双跑天然覆盖双 leg |
| run-all.mjs 一键矩阵 | Linux 开发机（web/uat/） | — | 真实二进制 spawn，环境敏感面大故不进 CI（D-04） |
| phase14.mjs herdr 协议层 UAT | Linux 开发机 | herdr 命名会话（外部子程序） | wesh 层 wire 断言 + herdr 行为层 API 观测（D-06） |
| phase14-pw.mjs 观感断言 | Windows 工作站（web/uat/pw/） | Linux 侧 wesh+herdr 经 SSH 管理 | 双机拓扑；双 tab 真实 Chromium buffer 断言（D-07） |
| per-client 负载矩阵（洪水+驻留） | load_test.go（//go:build load，Linux 手动） | /proc 子进程观测 | D-10/D-12；darwin 面沿用 skip 先例 |
| README/ARCHITECTURE/CONFIGURATION 文档段 | 文档层 | — | D-14；文档即被测物（herdr 配方经 phase14.mjs/pw 实证防说谎） |
| herdr 会话生命周期（起/停/清理） | UAT 脚本进程管理面 | herdr server daemon（独立存活） | wesh 只杀 client 进程组；server 由 UAT 显式 stop（spike 实证） |

## Standard Stack

**本 phase 零新增依赖**（项目红线延续：13-08 tech-stack 终审 go.mod/go.sum/lockfile 零 diff 先例）。全部构件为既有基础设施：

### Core
| Library/Tool | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` + `-race` | go1.26.3 [VERIFIED: `go version` 本机实测] | 三维归类改造与双模式 CI 门 | 项目唯一测试框架（docs/TESTING.md：无第三方断言框架） |
| `t.Run` 子测试 | stdlib | 双模式双跑执行机械 | D-01 锁定形态；PITFALLS :264 推荐 |
| Node 原生 WebSocket/fetch/child_process | v24.13.0 [VERIFIED: `node --version` 本机] | phase14.mjs / run-all.mjs 零依赖协议层 | phase02-13 全部协议脚本同款（docs/TESTING.md 层 1） |
| herdr | 0.8.100（`~/.local/bin/herdr`，symlink → ~/open_src/herdr/target/release/herdr，built 2026-09-04）[VERIFIED: `herdr --version` 本机] | driving scenario 载具 | D-05 锁定；per-client area 渲染唯一可实证面 |
| Playwright | 1.62.1（web/uat/pw/package.json 钉版）[VERIFIED: web/uat/pw/package.json] | Windows 侧浏览器全链观感 | 双机拓扑层 4；phase06-pw/phase12-pw 先例 |

### Supporting
| Library/Tool | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@xterm/headless` | 6.0.0（web/uat/package.json 既有，已装）[VERIFIED: web/uat/package.json + node_modules/@xterm/headless 存在] | phase14.mjs/pw 断言行可选的 buffer 级几何解析（需 `allowProposedApi: true`——CODEBUDDY.md 既定注意项） | 若想对 herdr 渲染输出做 buffer 文本断言替代裸 CUP 正则；**可选**——spike 实证裸 CUP 解析已够用 |
| jsdom | 25.0.1（既有） | 本 phase 无新增 jsdom 面 | run-all.mjs 重跑 4 个既有 *-dom.mjs 套件时 |
| tmux | 3.6 [VERIFIED: `tmux -V` 本机] | 仅 README 一句对照说明的核实依据 | 不建对照 UAT（D-05 锁定） |
| Python3 | 3.12.12 [VERIFIED 本机] | 本研究 spike 用 PTY harness | phase14.mjs 不需要——Node 直接驱 wesh wire 层即可 |

**Installation:** 无（零新依赖）。`web/uat/` 依赖已安装（@xterm/headless、addon-unicode11、jsdom 在位 [VERIFIED: node_modules 实测]）；`web/uat/pw/` 依赖仅 Windows 侧安装（本机未装属预期 [VERIFIED: node_modules 缺席实测]）。

## Package Legitimacy Audit

**本 phase 安装零外部包**——零新依赖红线（13-08 终审先例）适用于本 phase 全部工作面：run-all.mjs/phase14.mjs 为 Node 原生零依赖脚本；load 格扩展为 Go stdlib；pw 层复用既有 playwright 1.62.1 钉版依赖。无 npm/pypi/crates 引入，legitimacy gate 无检查对象。

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*唯一外部子程序 herdr 0.8.100 非包管理器依赖：本机已装、源码在 ~/open_src/herdr 可审计、版本经 `herdr --version` 钉定（D-09）。*

## Architecture Patterns

### Pattern 1: newTestServer(t, mode) 单一装配点（D-01 收编形态）

**What:** 收编 startTrackedServerWith（shared 族，e2e_test.go:171）与 startPerClientServer/WithSpawn（per-client 族，perclient_test.go:60/114）为单一装配入口，按 mode 分岔。
**When to use:** 全部 mode-agnostic 与 mode-mapped 测试的改造目标形态。

**现状核实（逐字签名，[VERIFIED: 本会话 Read]）：**

```go
// internal/server/e2e_test.go:171 —— shared 装配族母本
func startTrackedServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, waitHandlers func())

// internal/server/perclient_test.go:114 —— per-client 装配族薄包装
func startPerClientServer(t *testing.T, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string)

// internal/server/perclient_test.go:60 —— per-client 全形态（SpawnFunc 注入 + 会话追踪）
func startPerClientServerWithSpawn(t *testing.T, spawnFn func(cols, rows int, remoteUser string) (*pty.Session, error), mutate func(*server.Options)) (exitCh chan int, wsURL string, srv *server.Server, spawnedSessions func() []*pty.Session)
```

**关键不对称点（收编设计必须处理）**：shared 族在 `server.New(sess, ...)` 前**启动期 spawn**（pty.Start 直传）；per-client 族 `server.New(nil, ...)` + SpawnFunc 闭包**attach 期 spawn**。`newTestServer` 的 mode 分支即此两形态直传，per-client 分支必须保留 `startPerClientServerWithSpawn` 的 Cleanup 追踪纪律（spawned 逐一 Kill+Close——perclient_test.go:88-98 既有形态，防 seq 洪水泄漏级联减速教训 e2e_test.go:120-134 注释）。

**模式枚举逐字值** [VERIFIED: internal/server/clients.go:110-114]：

```go
const (
	SessionModeShared = "shared" // 默认（REQUIREMENTS 反特性 A5）
	SessionModePerClient = "per-client"
)
```

t.Run 子测试名建议逐字用 D-01 形态：`t.Run("mode=shared", ...)` / `t.Run("mode=per-client", ...)`。

**改造面实测盘点** [VERIFIED: 本会话 grep+wc 逐文件盘点]：server 包 32 个 _test.go / 153 个 Test 函数；全仓 42 个测试文件 / ~202 测试函数（cmd/wesh 22 + pty 21 + proto 6 + server 153）。`perclient_test.go`（36 测）为 Phase 11-13 per-client 新测归宿主文件，天然 per-client-only；两族装配函数调用分布（谁需要收编）：

| 文件 | tracked 族调用 | perclient 族调用 | 测试数 | 蓝本归属（PITFALLS :378-399） |
|------|------|------|------|------|
| e2e_test.go | 13 | 0 | 8 | 双模式断言分叉 |
| multi_test.go | 18 | 0 | 11 | 双模式断言分叉 |
| exit_test.go | 3 | 0 | 2 | 双模式断言分叉 |
| emptyexit_test.go | 7 | 3 | 9 | 双模式断言分叉（Phase 13 已混入 per-client 测） |
| shutdown_test.go | 4 | 5 | 6 | 双模式断言分叉（同已混入） |
| metrics_test.go | 15 | 1 | 6 | 双模式断言分叉（同已混入） |
| slowclient_test.go | 2 | 0 | 2 | 双模式断言分叉 |
| health_test.go | 6 | 0 | 2 | 双模式断言分叉 |
| resize_test.go | 0 | 0 | 3 | 双模式断言分叉（无装配族调用——自含装配，收编时个别看） |
| resize_arb_test.go | 2 | 0 | 1 | **shared-only（D-03 显式断言未装配形态）** |
| clients_test.go | 0 | 0 | 3 | shared-only 大部（owner 递补） |
| stopseq_test.go | 3 | 0 | 2 | 双模式断言分叉 |
| handshake/limits/keepalive/auth*/origin/throttle/tickets/sharetoken/tls/proxy*/basepath/customindex/log/exitmsg/events/options | — | — | — | mode-agnostic 同断言（蓝本表行） |

**CI 时长预算** [VERIFIED: 13-08-SUMMARY :29 登记值]：现全量 `-race` 5 包 1m37.2s；t.Run 双跑后多数测试体 ×2，**估算 3-4 min/leg**（darwin leg 更慢）[ASSUMED: 线性外推估算]。CI 结构零改动（ci.yml:17 单 step 保留 -v [VERIFIED: 本会话 Read ci.yml]）。

### Pattern 2: 断言分叉表与「显式断言未装配」（D-02/D-03）

**What:** mode-mapped 测试的断言分叉点显式写成表（mode → expected），shared 列期望值与 v1.0 逐字一致（零回归证据本体）；mode-exclusive（shared-only）测试在 per-client 分支断言组件不装配。
**纪律红线**（PITFALLS :270-273 警告征象逐字）：断言里出现 `if mode == ... { /* 跳过核心断言 */ }` 即腐化；禁止断言放宽成「两模式都接受」。
**先例形态**：Phase 13 已在 emptyexit/shutdown/metrics 三文件混入 per-client 测试（上表），这些文件的改造是把既有 per-client 测与 shared 测经 newTestServer 归一到同一测试体内的双 t.Run 分支，而非新写。

### Pattern 3: herdr 观测通道（spike 标定产物——本 phase 最高价值研究发现）

**全部结论经两轮 python PTY spike + 一轮 wesh×herdr 真实二进制全链 smoke 实证**（2026-09-06，herdr 0.8.100，protocol 19，schema_version 1）。

#### 3.1 会话隔离与清理序列（D-09 落地形态）[VERIFIED: live spike]

```
wesh argv:  wesh --session-mode=per-client --writable -- <HERDR 绝对路径> --session wesh-uat-<tag>
API 观测:   HERDR_SOCKET_PATH=~/.config/herdr/sessions/wesh-uat-<tag>/herdr.sock herdr api snapshot
清理:       herdr session stop wesh-uat-<tag>   # wesh 死后 herdr server 仍存活（特性），必须显式 stop
```

- socket 路径推导：`~/.config/herdr/sessions/<name>/herdr.sock`（源码 session.rs:169-171 `api_socket_path_for` = `data_dir_for(name)/herdr.sock`）[VERIFIED: ~/open_src/herdr/src/session.rs:161-171 Read + spike 实证]
- `herdr api snapshot` **不接受任何参数**（cli/api.rs:56-61），目标会话只能经 `HERDR_SOCKET_PATH` 环境变量指定（session.rs:173-181 active_api_socket_path 解析序）[VERIFIED: 源码 Read + spike 实证]
- `herdr session list` 表格输出含 name/status/directory/socket 四列，可作清理核验 [VERIFIED: spike 实证]

#### 3.2 is_foreground 仲裁与 per-client area 的 API 断言面（D-06 核心）[VERIFIED: live spike]

**`herdr api snapshot` → `result.snapshot.layouts[].area`（{width,height,x,y}）恒等于前台客户端的 pane 面积**。前台 = last-activity-wins（attach 或输入均为活动；源码 clients.rs:53-54 last_activity + headless.rs 渲染循环 resize_panes=is_foreground [VERIFIED: ~/open_src/herdr/src/server/headless.rs:4065-4090 Read]——前台渲染才 resize pane，背景客户端 `compute_view_without_resizing_panes` 保 scroll 状态）。

spike 实测翻转链（桌面 120x40 / 移动 40x12）：

| 步骤 | snapshot layouts[0].area | 含义 |
|------|--------------------------|------|
| S1 桌面 attach | `{width:94, height:39, x:26, y:1}` | 桌面前台：全量布局（26 列侧栏 + 1 行顶栏 chrome） |
| S2 移动 attach | `{width:40, height:10, x:0, y:2}` | 移动变前台：紧凑布局（无侧栏，2 行 chrome） |
| S3 API 注入 pane 活动 | 不变 | 前台不随 pane 输出翻转 |
| S4 桌面端按键（经 wesh INPUT） | 翻回 `{94,39,26,1}` | **仲裁恢复的可证伪证据** |
| S5 移动 resize→50x20 | `{width:50, height:18, x:0, y:2}` | 移动新尺寸（前台翻转后观测） |

**断言行设计（phase14.mjs 采用形态）**：断言翻转链而非绝对值——`area ∈ {桌面几何派生值, 移动几何派生值}` 关系断言 + 翻转方向断言（移动 attach 后=移动几何、桌面活动后=桌面几何）。**禁止硬编码 94/26 等 chrome 常量**：用户 config.toml 共享（见 Pitfall 5），`DEFAULT_MOBILE_WIDTH_THRESHOLD=64`（herdr config.rs:42，<64 列走紧凑布局）是几何分岔的版本钉定点 [VERIFIED: ~/open_src/herdr/src/config.rs:42 Read]。

#### 3.3 流层互证（桌面端不被压缩的直接证据）[VERIFIED: live spike + wesh smoke]

- 移动端 attach 后桌面端只收 **55 字节**增量（无压缩重渲染）；移动 resize 后桌面端收 **450 字节**增量（CUP max_col=76，仍为桌面几何增量更新）
- 移动端 attach 自身收全量紧凑布局帧（13047 字节）；桌面端 attach 收 96625 字节全量 120 宽帧
- wesh 层全链 smoke：双 session_start 事件双 pid（stderr JSON，`client_id` 归因 [VERIFIED: OPS-12 既有面 + smoke 实证]）、两端 Welcome `session:"per-client"`、桌面 OUTPUT 流含 120 宽边框特征、移动端 CUP max_col=40
- **pw 层对应断言（D-07）**：大屏 tab xterm buffer 的 herdr 面板边框列位置在小屏 tab attach+resize 后逐字不变——phase06-pw/phase12-pw 的 buffer 文本断言通道（`page.evaluate` 读 `.xterm-rows` / waitTermText 先例）直接复用

#### 3.4 ro 汇聚场景观测通道（D-08 ②）[VERIFIED: live smoke 全链]

- **ticket 全链**：`POST /api/attach` 携 share token → 返回一次性 ticket → Hello 帧 JSON 携 `ticket` 键 → Welcome `mode:"ro"`/`"rw"`（phase05.mjs :43-49/:251-268 既有形态 [VERIFIED: 本会话 Read]；**注意**：Hello 携 ticket 是唯一通道，WS query 参数无效——本研究 smoke4 误用教训）
- **pane 内容结构化观测**：`herdr pane read w1:p1 --source visible`（text 格式默认）——ro 端打字后 pane 内容逐字不变 + 标记串缺席断言；rw 端打字后标记串可见。比流嗅探稳定（D-06 否决输出嗅探的同款理由）
- `herdr pane send-text <pane_id> <text>`：API 侧输入注入（spike S3 用例实证），可作确定性 pane 活动触发器

#### 3.5 herdr 进程形态（pid 断言纪律）[VERIFIED: 源码 Read + smoke 实证]

- client：`herdr --session <name>`（wesh 子进程，PTY 内 TUI）；server：首 client attach 时惰性 spawn 为 `herdr server`（cmdline **不含**会话名，经 HERDR_SESSION env 传承——autodetect.rs:189-236）
- **pgrep 陷阱**：`pgrep -f "herdr --session X"` 会同时命中 wesh 自身（其 argv 含 `-- herdr --session X`）——pid 断言应走 wesh stderr 的 session_start 事件（OPS-12 既有结构化归因）+ ESRCH 探针（phase11/13.mjs waitScanPid 先例），不用 pgrep 计数
- wesh 断连杀的是 client 进程组（SIGHUP + 默认 5s KILL 兜底），herdr server 存活是特性（A2 裁决：「herdr 场景下 wesh 杀掉的是 herdr client，herdr-server 会话本就活着」）；UAT 清理序列 = 关 WS → SIGTERM wesh → `herdr session stop <name>` → `session list` 核验 stopped

#### 3.6 就绪等待与时序标定 [VERIFIED: spike 实测]

- 首个 client attach 触发 herdr server 惰性 spawn：attach→全量首帧 ≤6s（本机实测；含 server 启动）
- **就绪门推荐形态**：轮询 `herdr api snapshot` 至 `layouts` 非空（结构化、无 flake），而非固定 sleep——phase06.mjs :354-356 时序纪律（真实等待 + 护栏上限，禁精确时点断言）的同构应用
- 第二 client attach（server 已在）首帧同样 ≤6s 量级

### Pattern 4: per-client 负载矩阵格设计（D-10/D-12）

**夹具全部既有** [VERIFIED: load_test.go 本会话 Read]：drainClient(:80)/drainRateLimited(:103)/awaitDrain(:126)/assertClosed1000(:139)/dialLoadClient(:149)/scrapePeakSampler(:174)/allocPeakSampler(:211)/readAlloc(:232)/loadFloodLast(:242)/gatedFloodArgv(:256)/startLoadSamplers(:268)/countFds(:555)/readProcState(:567)；churn 格 TestChurnPerClientSpawnThrottle(:725) 为 per-client 装配 + 双采样差值断言母本。

**洪水格（N∈{1,4,16,32}）与 shared 格（TestLoadFanoutMatrix :298）的关键差异点**：

```go
// shared 格：单进程洪水，仅 conns[0] 发触发 INPUT（load_test.go:331）
// per-client 格：每会话独立进程各自 read stdin —— 必须【每客户端各发一次】触发：
for _, c := range conns {
    c.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'})
}
```

- 装配换 `startPerClientServer(t, gatedFloodArgv(last), nil)`（churn 格 :726 同款）
- 断言面：每端收流完整（字节数跨端相等 + 末位字段==洪水末位——同 :345-356 形态）+ `wesh_pty_spawn_total == N`（churn 格 :813 程序序精确对照先例）+ kicks==0（活跃读面）+ 峰值采样入 LOADDATA
- 总量估算：N=32 时 32×33.8MB PTY 输出，awaitDrain 240s/端护栏先例沿用 [ASSUMED: 与 shared 扇出格同量级外推]

**驻留格（N 会话 idle 空转）**：
- 装配 `startPerClientServer(t, []string{"sh"}, nil)` + N 端 attach 后零输入驻留
- wesh 侧硬断言（D-12）：Alloc 增量 ≤ N×(768KiB+ε)——768KiB = outbox 512KiB + inputQ 256KiB 账面 [VERIFIED: internal/server/clients.go:36 `defaultOutboxBytes = 512 * 1024` 与 :50 `defaultInputQueueBytes` 256KiB 注释，本会话 Read]；ε 容纳 pcSession 控制结构/goroutine 栈/令牌桶 map 条目，首跑实测定值并注释论证（churn 格 +8 goroutine 容差推导形态先例）
- 子进程观测（D-12）：/proc/<pid>/status VmRSS 经 readProcState 同文件先例（:567）采样入 LOADDATA + ≤15MB/进程宽松上界
- fd 账面 ~2×N+pidfd、goroutine 账面 1+6N（ARCHITECTURE §10 :480-481 表 [VERIFIED: 本会话 Read]）作差值断言参照
- `//go:build load` 首行隔离 + Linux 手动跑纪律不变（load_test.go:1-8 头注释逐字纪律）

**LOADDATA per-client 扩展字段建议**（Discretion 面）：沿用 `cell=<name>` 前缀，新增格名 `pc_flood` / `pc_resident`，字段加 `sessions=N spawn_total=N rss_per_proc_max=…`；README 标定表回填沿 09-09 D-13 先例表格形态。

### Pattern 5: run-all.mjs 一键矩阵（D-04）

**枚举源（13-08 收口闸实测基线 [VERIFIED: 13-08-SUMMARY :172-193 表，本会话 Read]）**——协议 11 + jsdom 4 + per-client 增量：

| 脚本 | 基线 | 脚本 | 基线 |
|------|------|------|------|
| phase02 | 12/12 | phase09 | 18/18 |
| phase03 | 18/18 | phase11 | 21/21+1skip |
| phase04 | 10/10 | phase12 | 20/20 |
| phase05 | 28/28+1skip | phase04-dom | 37/37 |
| phase05-dims | DIMS PASS | phase05-dom | 19/19 |
| phase06 | 23/23+1skip | phase06-dom | 40/40+2skip |
| phase07 | 34/34+1skip | phase12-dom | 17/17 |
| phase08 | 21/21 | phase13 | 29/29 |

- 每脚本 exit code 0/1 门禁语义逐脚本既有（phase13.mjs 尾：`process.exit(failedN === 0 && failed === 0 ? 0 : 1)` [VERIFIED: 本会话 Read]）——runner 只需串行 spawn + 聚合 exit code + 汇总表
- 前置：`go build -o /tmp/wesh-uat/wesh ./cmd/wesh`（脚本默认二进制路径约定，phase13.mjs 头注释形态）
- 辅助脚本（phase04-t1-width/phase07-b*/phase08-journal 等）不在收口矩阵——13-08 十五脚本口径即「零遗漏」的定义基准，runner 枚举以它为准
- 全矩阵时长量级 ~3.5min（13-08 十七轮实测 [VERIFIED: 13-08-SUMMARY :40 注记]）+ phase14.mjs 增量（herdr 场景含真实等待，预估 1-2min）[ASSUMED: 外推]

### System Architecture Diagram（phase14 验证拓扑）

```mermaid
flowchart LR
    subgraph Linux 开发机（headless）
        GO[go test -race ./...<br/>三维归类 t.Run 双跑<br/>CI 门（ubuntu+macOS leg）]
        LOAD[load_test.go //go:build load<br/>pc_flood N×洪水格 + pc_resident 驻留格<br/>手动跑]
        RUNNER[run-all.mjs<br/>15 既有脚本 + phase14.mjs 一键矩阵]
        P14[phase14.mjs<br/>Node 原生 WS 双客户端]
        WESH[wesh --session-mode=per-client<br/>真实二进制 spawn]
        HAPI[herdr api snapshot / pane read<br/>HERDR_SOCKET_PATH 定向]
    end

    subgraph herdr 命名会话（wesh-uat-*，隔离）
        HSRV[herdr server daemon<br/>惰性 spawn · 独立于 wesh 存活]
        HC1[herdr client A 120x40<br/>wesh 子进程 pid1]
        HC2[herdr client B 40x12<br/>wesh 子进程 pid2]
    end

    subgraph Windows 工作站（GUI）
        PW[phase14-pw.mjs<br/>Playwright 双 tab]
        CH1[Chromium tab 桌面视口]
        CH2[Chromium tab 移动视口]
    end

    P14 -->|WS desktop Hello 120x40| WESH
    P14 -->|WS mobile Hello 40x12| WESH
    WESH -->|spawn| HC1
    WESH -->|spawn| HC2
    HC1 <-->|socket| HSRV
    HC2 <-->|socket| HSRV
    HAPI -->|读 layouts[].area 翻转链| HSRV
    PW -->|SSH 管理| WESH
    CH1 -->|TCP 转发器| WESH
    CH2 -->|TCP 转发器| WESH
    RUNNER -->|串行 spawn 聚合 exit code| P14
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| herdr 断言的输出嗅探 | 自写 ANSI 渲染格式解析断言 herdr 面板状态 | `herdr api snapshot` layouts + `herdr pane read` | herdr 版本漂移改渲染格式即碎（D-06 否决输出嗅探的既定裁决；spike 实证 API 通道够用） |
| UAT 结果聚合框架 | 引入 jest/mocha/tap 等跑 UAT | 逐脚本 exit code + runner 串行聚合 | 15 既有脚本全部自带 check()/exit code 门禁（docs/TESTING.md「无测试框架」既定形态） |
| 双模式执行的 env 重跑机械 | CI 加第二 step 或进程级 env 开关 | t.Run 子测试双跑（D-01） | env 全量重跑 CI 时长 ×2 且 mode-mapped 分叉仍逃不掉（D-01 否决记录） |
| 子进程内存观测 | 引入 gopsutil 类依赖读 VmRSS | readProcState 读 /proc/<pid>/status（load_test.go:567 既有先例） | 零新依赖红线；churn 格 countFds 同文件先例 |
| herdr 会话清理 | kill 进程树/pkill 模式匹配 | `herdr session stop <name>` | pgrep 误伤面（wesh argv 自匹配陷阱，§Pitfall 3）；session stop 是官方语义通道且 spike 实证可靠 |
| 就绪等待 | 固定 sleep 时长 | 轮询 snapshot 至 layouts 非空 | phase06 时序纪律（禁精确时点断言）；server 惰性 spawn 时长机器相关 |

**Key insight:** 本 phase 的全部构件（夹具/观测通道/门禁形态）在仓库内均有三代以内先例；唯一新面 herdr 观测通道已 spike 标定。手造任何东西都是绕开已实证路径引入新风险。

## Runtime State Inventory

> 本 phase 非字符串重命名，但含测试 harness 收编重构 + herdr 外部子程序运行时态，按类显式回答：

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — wesh 无持久存储；herdr 命名会话的状态目录 `~/.config/herdr/sessions/wesh-uat-*/` 为 UAT 一次性产物（清理序列含 session stop） [VERIFIED: spike 实证] | UAT 清理序列执行 + 跑后 `session list` 核验 |
| Live service config | **herdr server daemon 独立于 wesh 存活**（wesh SIGTERM 后 session 仍 running——smoke 实证）；用户日常 `default` 会话运行中（**严禁触碰**——D-09 零污染纪律） [VERIFIED: live spike + `herdr session list` 实证] | 命名会话隔离（--session wesh-uat-*）+ 显式 stop；断言失败时清理兜底（defer/finally 形态） |
| OS-registered state | None — 无 systemd/launchd/cron 注册面 | — |
| Secrets/env vars | UAT 分享链接 token/ticket 为一次性测试凭据——既有红线纪律（token/pid 只作断言材料，永不进 check detail/控制台输出，phase13.mjs :73-85 assertOutputClean 自净形态） [VERIFIED: 本会话 Read] | phase14.mjs 沿用同红线；**HERDR_ENV 嵌套阻断**：UAT 若从 herdr 内 shell 启动，wesh env 白名单结构性剥离（PATH/HOME/USER/LOGNAME/SHELL/LANG/LC_*/TERM/COLORTERM/WESH_REMOTE_USER 之外零透传 [VERIFIED: internal/pty/spawn.go:128-181 Read]），herdr 子进程侧无感；但 spike/调试期手工起 herdr client 需 `env -u HERDR_ENV` |
| Build artifacts | None — load 格与 harness 改造均为源码态；wesh 测试二进制 /tmp/wesh-uat/wesh 每次构建覆写 | — |

## Common Pitfalls

### Pitfall 1: 断言放宽毁灭零回归证据（PITFALLS P11 本体）
**What goes wrong:** mode-mapped 改造时把两模式断言「和稀泥」成都能过的形态，shared 零回归证据就此毁灭。
**Why it happens:** naive 参数化乘二，分叉断言图省事。
**How to avoid:** 断言分叉表（mode → expected），shared 列与 v1.0 逐字一致；验收闸 = git diff 白名单审查期望值逐字未动（D-02）。
**Warning signs:** `if mode == ... { 跳过核心断言 }`；shared 老测试期望值 diff。

### Pitfall 2: herdr 几何断言硬编码 chrome 常量
**What goes wrong:** 把 spike 观测值（94x39/x:26 侧栏/2 行 chrome）写死进断言行。
**Why it happens:** spike 值看起来稳定。
**How to avoid:** **关系断言**——area ∈ 两客户端几何派生集合 + 翻转方向断言；chrome 尺寸随用户 config.toml（共享，无法经 wesh 隔离——env 白名单剥离 HERDR_CONFIG_PATH/XDG_CONFIG_HOME）与 herdr 版本漂移。[VERIFIED: 白名单 spawn.go:128-181 + herdr config.rs:42 源码 + spike]
**Warning signs:** 断言里出现字面 94/26/39。

### Pitfall 3: pgrep 进程计数自匹配
**What goes wrong:** `pgrep -f "herdr --session X"` 计数 = 客户端数 + 1（wesh 自身 argv 含完整子进程命令行）。
**Why it happens:** pgrep -f 匹配全 cmdline，wesh argv 尾部就是 `-- herdr --session X`。
**How to avoid:** pid 断言走 wesh stderr session_start 事件（OPS-12 结构化 JSON 归因）+ ESRCH 探针（phase11/13 waitScanPid 先例）。[VERIFIED: smoke 实证计数偏差 + autodetect.rs:189-236 server 形态]
**Warning signs:** 「双 pid」断言数出 3 个进程。

### Pitfall 4: Node 原生 WebSocket binaryType 默认值
**What goes wrong:** `new Uint8Array(ev.data)` 在默认 `binaryType='blob'` 下抛异常，onmessage 静默断链（本研究 smoke 首跑即踩）。
**How to avoid:** attach 后立即 `ws.binaryType = 'arraybuffer'`。**既有 phaseNN.mjs 是怎么处理的——经查 phase13.mjs 用 `for await` 迭代形态规避（[ASSUMED: 未逐行核读既有脚本 WS 读取形态]），phase14.mjs 新写代码若以 onmessage 形态读帧必须显式设 binaryType**。
**Warning signs:** Welcome 帧永远收不到、脚本挂起到超时。

### Pitfall 5: herdr 嵌套阻断（HERDR_ENV）
**What goes wrong:** 从 herdr 内 shell 手工起 `herdr --session X` 调试/spike → 「nested herdr is disabled by default」拒绝启动。
**Why it happens:** HERDR_ENV=1 环境标记（main.rs:443-447 should_block_nested_for_env）。
**How to avoid:** 手工调试 `env -u HERDR_ENV herdr ...`；wesh spawn 路径无此问题（env 白名单结构性剥离 [VERIFIED: spawn.go:128-181 + smoke 实证——本研究 smoke 全部经 wesh 路径零嵌套阻断]）。
**Warning signs:** client 起不来且 stderr 有「recursive descent denied」类彩蛋文案。

### Pitfall 6: herdr pane 内 shell 非 POSIX（fish）
**What goes wrong:** UAT 标记命令用 `$((算术))` 等 bashism → pane 宿主 shell 是用户默认 shell（本机 fish）语法报错，标记串缺席假红（本研究 smoke4 实测踩坑）。
**How to avoid:** 标记命令只用最简形态 `echo MARKER_STR`（fish/bash/sh 三通）。
**Warning signs:** pane read 里出现 `^~~~^` 语法高亮错误行。

### Pitfall 7: 无基线对照的绝对上限 / 固定 sleep 精确时点（历史教训合集）
**What goes wrong:** 驻留格拍脑袋写绝对内存上限；回收等待用固定 sleep。
**How to avoid:** 双采样差值 + 容差断言（churn 格 :809-831 母本）；回收窗口轮询 + 护栏上限（:776-789）；首跑实测定容差并注释论证（:822 memDeltaCeil 16MiB 形态）。
**Warning signs:** 断言常量无注释论证来源；CI/慢机 flake。

### Pitfall 8: ro 场景 ticket 通道误用
**What goes wrong:** ticket 塞 WS query 参数（本研究 smoke4 实测——不报错但 attach 成 rw，ro 场景假绿）。
**How to avoid:** 唯一通道 = `POST /api/attach` 携 share token 换 ticket → Hello JSON `ticket` 键（phase05.mjs 既有形态）；**自检断言 Welcome.mode=="ro" 落到 check 行**——mode 不符即假绿防线（D-08「防文档说谎」同构：防 UAT 自欺）。
**Warning signs:** Welcome.mode 不是预期值但脚本继续跑。

## Code Examples

### newTestServer 收编骨架（建议形态，归 harness_test.go 或 export_test.go——Discretion 面）

```go
// 依据：两族母本签名 [VERIFIED: e2e_test.go:171 / perclient_test.go:60,114 本会话 Read]
// 模式常量 [VERIFIED: clients.go:110-114]: server.SessionModeShared="shared" / SessionModePerClient="per-client"
func newTestServer(t *testing.T, mode string, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string) {
    t.Helper()
    switch mode {
    case server.SessionModeShared:
        opts := server.Options{Writable: true}
        if mutate != nil { mutate(&opts) }
        exitCh, wsURL, _ = startTrackedServerWith(t, argv, opts) // waitHandlers 丢弃=薄包装零改动
        return exitCh, wsURL
    case server.SessionModePerClient:
        return startPerClientServer(t, argv, mutate) // 直传——Cleanup 追踪纪律保留
    default:
        t.Fatalf("unknown mode %q", mode)
        return nil, ""
    }
}
```

### mode-mapped 断言分叉表形态（D-01/D-02）

```go
// Source: PITFALLS :264 推荐形态 + 12-01 D-08 Welcome 组帧恒传模式先例
func TestExitBroadcast(t *testing.T) {
    for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
        t.Run("mode="+mode, func(t *testing.T) {
            _, wsURL := newTestServer(t, mode, []string{"sh", "-c", "exit 42"}, nil)
            // 断言分叉点显式成表——shared 列期望值与 v1.0 逐字一致（零回归证据）：
            want := map[string]struct{ bSeesExit bool }{
                server.SessionModeShared:    {bSeesExit: true},  // 广播（v1.0 EXIT 广播语义逐字）
                server.SessionModePerClient: {bSeesExit: false}, // 私有化 + 他端零感知（PC-04）
            }[mode]
            // ...双端 attach，A 进程退出，断言 B 端 seesExit == want.bSeesExit
        })
    }
}
```

### per-client 洪水格骨架（D-10）

```go
//go:build load

// Source: load_test.go:298-380 TestLoadFanoutMatrix 母本改造面 [VERIFIED 本会话 Read]
func TestLoadPerClientFloodMatrix(t *testing.T) {
    last := loadFloodLast()
    for _, n := range []int{1, 4, 16, 32} {
        t.Run(fmt.Sprintf("sessions_%d", n), func(t *testing.T) {
            _, wsURL := startPerClientServer(t, gatedFloodArgv(last), nil) // churn 格 :726 同款装配
            // ...N 端 dialLoadClient + drainClient...
            // 【与 shared 格的关键差异】每客户端各自触发（每会话独立 stdin）：
            for _, c := range conns {
                if err := c.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
                    t.Fatalf("write 触发 INPUT: %v", err)
                }
            }
            // 断言：每端收流完整（字节跨端相等+末位字段）+ spawn_total==N + kicks==0 + LOADDATA 行
        })
    }
}
```

### phase14.mjs herdr 会话夹具（spike 实证形态）

```js
// Source: 本研究 wesh×herdr 全链 smoke（/tmp 三轮迭代实证，2026-09-06）
const SESSION = `wesh-uat-p14-${process.pid}`;
const SOCK = `${os.homedir()}/.config/herdr/sessions/${SESSION}/herdr.sock`;

// 启动：per-client + writable，子进程 = herdr client 挂命名会话
const wesh = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', '--writable',
  '--session-mode=per-client', '--', HERDR, '--session', SESSION], { stdio: ['ignore', 'pipe', 'pipe'] });

// API 观测（结构化断言通道）：
const snap = JSON.parse(execFileSync(HERDR, ['api', 'snapshot'],
  { env: { ...process.env, HERDR_SOCKET_PATH: SOCK }, encoding: 'utf8' }));
const area = snap.result.snapshot.layouts[0].area; // {width,height,x,y} = 前台客户端 pane 面积

// 就绪门：轮询 snapshot 至 layouts 非空（禁固定 sleep——Pitfall 7）
// ro 汇聚：POST /api/attach {token} → ticket → Hello 携 ticket → Welcome.mode==='ro' 自检
// pane 观测：herdr pane read w1:p1 --source visible（ro 打字前后逐字比对）
// 清理（finally 兜底）：wesh SIGTERM → execFileSync(HERDR, ['session', 'stop', SESSION])
//   → herdr session list 核验 stopped（wesh 死后 herdr server 存活是特性，必须显式 stop）
```

### WS 读帧纪律（Pitfall 4）

```js
const ws = new WebSocket(`ws://127.0.0.1:${port}/ws`, 'wesh.v1');
ws.binaryType = 'arraybuffer'; // 必须——默认 'blob' 使 new Uint8Array(ev.data) 抛异常
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| tmux window-size 默认 smallest（取所有客户端最小值） | **tmux 3.6 默认 latest**（最近活动客户端尺寸）[VERIFIED: 本机 `tmux -f /dev/null -L <sock> show-options -g window-size` → `window-size latest` 实测] | tmux 3.x 演化 | D-05 结论不变（latest 下小屏 attach/活动仍压缩其他客户端观感——「tmux 不能作恢复证据」成立），但 README 对照句措辞不得写「默认取最小值」字面——应表述为「默认 window-size=latest 下小屏客户端活动时窗口收缩，桌面端观感被压缩；window-size 可配但均为单窗口尺寸语义，不存在 per-client 独立渲染」 |
| herdr 断言靠终端输出嗅探 | herdr 0.8.100 socket API 结构化观测（snapshot layouts / pane read / pane send-text） | 本 phase spike 标定 | D-06 否决输出嗅探的通道落地；protocol 19 形态钉定 |

**Deprecated/outdated:**
- 输出嗅探断言 herdr 行为：版本漂移即碎（D-06 既定否决）
- 截图 diff 回归：herdr 渲染含时钟/状态闪烁面（D-07 既定否决）

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 双模式 t.Run 改造后 CI 时长 ~3-4min/leg | Pattern 1 | 低——估算偏差只影响预期不影响形态；若超 CI 时限需分批优化（届时偏差登记） |
| A2 | per-client 洪水格 240s/端 drain 护栏沿用够用（N=32 时 32×33.8MB） | Pattern 4 | 低——首跑实测若超时按先例上调护栏并注释论证 |
| A3 | 驻留格 ε 容差首跑定值（churn 格 +8 goroutine 推导形态参照） | Pattern 4 | 低——D-12 既定「首跑实测定值并注释论证」流程即此 |
| A4 | 32 shell 驻留 ~160MB 账面（5MB/进程 ×32） | ROADMAP Research flag 本体 | 中——驻留格实测即证真/证伪通道（D-10 既定）；证伪则开 D-11 one-way 门 |
| A5 | pw 层双 tab viewport 像素尺寸（桌面~1600x1000/移动小屏）需运行时 fit 标定 | phase14-pw Discretion 面 | 低——phase12-pw V0/V_UP/V_DOWN 阶梯先例已实证 fit 链路；cols/rows 以 stty 实测回读为准 |
| A6 | phase14.mjs 增量时长 1-2min（herder server 惰性启动 + 真实等待） | Pattern 5 | 低——只影响矩阵总时长预期 |
| A7 | 既有 phaseNN.mjs 的 WS 读帧形态（for await vs onmessage）未逐行核读 | Pitfall 4 | 低——phase14.mjs 新写代码按 binaryType 纪律写即可；改造既有脚本不在本 phase 范围 |

## Open Questions

1. **herdr 版本漂移的断言行韧性边界**
   - What we know: 断言基于 0.8.100 protocol 19 实测形态；D-09 已定「失败先核 `herdr --version`」处置纪律；本机 herdr 是源码构建 symlink（用户 rebuild 即漂移）
   - What's unclear: layouts[].area 字段名/紧凑布局阈值在哪个版本区间稳定（无多版本对照实测面）
   - Recommendation: 关系断言（Pitfall 2）天然吸收 chrome 尺寸漂移；UAT 文档写明版本钉定纪律（D-09 既定）；phase14.mjs 启动时先打 `herdr --version` 进日志（非断言，诊断材料）

2. **perclient_test.go 36 测在三维归类中的归属处置**
   - What we know: 该文件全部测试天然 per-client-only（蓝本表「新增 per-client-only」行的落地产物）；D-03 只定义了 shared-only 测试的 per-client 分支形态（显式断言未装配）
   - What's unclear: 对称方向（per-client-only 测试是否要加 shared 分支断言「该面不存在」，如 spawn 失败 1011 在 shared 结构性不存在）CONTEXT 未显式定义
   - Recommendation: 默认不动（per-client-only 测试保持单模式——它们本身就是 per-client 的断言本体，蓝本「新增 per-client-only」行未要求双跑）；若 planner 判断需对称防线，按 D-03 同构形态在 shared 分支断言「不装配/不存在」（如 shared 下 spawn 失败=启动期暴露），**属 Discretion 面，落成时登记偏差**

3. **run-all.mjs 是否纳入 phase14-pw.mjs**
   - What we know: pw 层永远 Windows 侧（D-04 双机拓扑硬约束），run-all 跑在 Linux
   - Recommendation: 不纳入——runner 汇总表注明 pw 层独立执行入口（`node web/uat/pw/phase14-pw.mjs`），收口闸人工两段式（Linux runner 全绿 + Windows pw 全绿）

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | 全部 Go 测试/构建 | ✓ | go1.26.3 linux/amd64 [VERIFIED 本机] | — |
| Node.js | UAT 脚本（协议层/runner） | ✓ | v24.13.0 [VERIFIED 本机]（≥22 原生 WebSocket/fetch 满足） | — |
| herdr | D-05 driving 载具 | ✓ | 0.8.100（~/.local/bin/herdr → ~/open_src/herdr target/release）[VERIFIED 本机] | 无——D-05 锁定仅真 herdr；缺席则 SC3 整体阻塞 |
| tmux | README 对照句核实 | ✓ | 3.6 [VERIFIED 本机] | 不建 UAT（D-05），文档句已实测核实 |
| pnpm | web 构建（dist byte-identical 闸） | ✓（项目既定，CI 钉版 11.21.0） | 本机项目惯例在位 | — |
| web/uat node_modules | jsdom/*-dom 套件重跑 | ✓（@xterm/headless、addon-unicode11、jsdom 在位）[VERIFIED 本机] | — | `pnpm -C web/uat install` |
| Playwright + Chromium | phase14-pw.mjs | ✓（Windows 工作站侧，CODEBUDDY.md 既定「Chromium 缓存就绪」；本 Linux 机未装属预期）[VERIFIED: 本机 node_modules 缺席 + CODEBUDDY.md 拓扑声明] | playwright 1.62.1 钉版 | 无——双机拓扑硬约束，Linux 侧禁装 |
| macOS 运行环境 | darwin 行为测试 | ✗（本机无） | — | CI macOS leg 承担（11-06/12-05/13-08 既定口径）；本机仅 GOOS=darwin 双编译闸 |
| Python3 | （仅本研究 spike 用） | ✓ | 3.12.12 | phase14 交付物不需要 |

**Missing dependencies with no fallback:** 无（herdr 在位；若用户环境变化致 herdr 缺席，SC3 阻塞——D-05 无替代载具，属既定取舍）

**Missing dependencies with fallback:** macOS 运行环境 → CI macOS leg（既定先例）

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `-race`（go1.26.3）；UAT 侧无框架（自带 check() 收集器 + exit code 门禁） |
| Config file | 无独立测试配置（go.mod 钉版；`//go:build load` 隔离负载格） |
| Quick run command | `go test -race -count=1 -run TestPerClient ./internal/server/`（单族抽样） |
| Full suite command | `time go test -race -count=1 -v ./...`（CI 同款）；负载格 `go test -tags=load -count=1 -timeout=30m ./internal/server/ -v`；UAT `node web/uat/run-all.mjs`（本 phase 新建） |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PC-13 | herdr driving：移动端 attach 桌面端不压缩（协议层） | e2e（协议层 UAT） | `node web/uat/phase14.mjs` | ❌ 本 phase 新建 |
| PC-13 | herdr driving 观感（浏览器层） | e2e（Playwright，Windows 侧） | `node web/uat/pw/phase14-pw.mjs`（Windows 工作站） | ❌ 本 phase 新建 |
| PC-13 | ro 汇聚（ro 链接 attach 同 herdr 会话 + 输入门控） | e2e（协议层 UAT） | `node web/uat/phase14.mjs`（场景②） | ❌ 本 phase 新建 |
| PC-12 | 模式语义文档三件套 + GoTTY 误记修正 | 文档 + 人工 review | 收口闸 diff 审查 + 「文档即被测物」先例（herdr 配方经 phase14.mjs/pw 实证） | ❌ 文档段新建 |
| SC1 | -race 双模式 CI 门 | unit/e2e（Go） | `go test -race -count=1 -v ./...`（CI 单 step 零改动） | ✅ 改造既有 |
| SC2 | 协议层 per-client 全链六项 | e2e（协议层 UAT） | `node web/uat/run-all.mjs` 重跑 phase11/12/13（D-13 不重复断言） | ✅ 既有三脚本 |
| SC4 | 负载矩阵 1/4/16/32 回填 | load（//go:build load） | `go test -tags=load -run 'TestLoadPerClient' -count=1 -timeout=30m ./internal/server/ -v` | ❌ load_test.go 内新建格 |

### Sampling Rate
- **Per task commit:** `go test -race -count=1 -run '<本任务触及测试族>' ./internal/server/`（改造哪族跑哪族）
- **Per wave merge:** `time go test -race -count=1 ./...` 全量 + 触及文件双模式子测试逐名核对
- **Phase gate:** 收口闸六段式先例（11-06/12-05/13-08 三代同构）：静态面 + 全量 -race + darwin 双编译闸 + dist byte-identical + UAT 矩阵（run-all 全绿 + phase14.mjs + phase14-pw.mjs Windows 侧）+ diff 白名单审查——全绿后 `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/uat/phase14.mjs` — herdr 协议层 UAT（PC-13 协议面 + ro 汇聚）
- [ ] `web/uat/pw/phase14-pw.mjs` — Windows Playwright 双 tab 观感（PC-13 浏览器面）
- [ ] `web/uat/run-all.mjs` — 一键矩阵 runner（D-04）
- [ ] load_test.go per-client 双剖面格（TestLoadPerClientFloodMatrix / 驻留格——名称 Discretion）
- [ ] `newTestServer` harness（harness_test.go 或 export_test.go——Discretion 面）+ 三维归类逐文件改造（最大工作量面，~40 文件，建议按归属类别分 wave：mode-agnostic → mode-mapped → mode-exclusive）
- [ ] 框架安装：无（零新依赖）

## Security Domain

> security_enforcement 启用（config 默认）。本 phase 零产品代码变更——威胁面零新增；以下为测试/UAT 面的安全纪律核实。

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | 否（零产品变更）；UAT 涉及既有 ticket 机制的**消费** | 既有 ticket 通道（POST /api/attach → Hello 携 ticket）——phase05.mjs 先例形态，零新认证面 |
| V3 Session Management | 否（同上） | — |
| V4 Access Control | 否（ro 门控为既有 PC-07 机制的验证对象而非新实现） | ro 门控断言（phase14.mjs 场景②=既有防线的证据面） |
| V5 Input Validation | 部分（UAT 脚本自身的输入面） | UAT 脚本零外部输入面（参数仅 wesh 二进制路径）；herdr 会话名 `wesh-uat-*` 固定前缀 |
| V6 Cryptography | 否 | —（永不手造红线不变） |

### Known Threat Patterns for {测试/UAT 面}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 测试凭据/token/pid 泄入日志或控制台 | Information Disclosure | 既有红线：token/pid 只作断言材料，detail 只打状态码/布尔/形状/退出码；`assertOutputClean()` 运行时自净扫描（phase13.mjs :73-85 三代先例——phase14.mjs/run-all.mjs 必须沿用同形态） |
| UAT 测试进程/会话残留（herdr server、wesh 实例、洪水子进程） | Denial of Service（资源泄漏） | 清理序列 finally 兜底（wesh SIGTERM + `herdr session stop` + killServer 形态 t.Cleanup——e2e_test.go:120-134 泄漏级联教训先例）；跑后 `herdr session list` 核验 |
| herdr 命名会话与用户日常 default 会话互污染 | Tampering | D-09 独立会话隔离 + HERDR_SOCKET_PATH 定向；spike 全程实证 default 会话零触达 |
| 负载格洪水子进程（seq/bash）泄漏抢 CPU | Denial of Service | startPerClientServerWithSpawn Cleanup 逐一 Kill+Close 追踪纪律（perclient_test.go:88-98）；load 格手动跑 + 30m timeout 护栏 |

## Sources

### Primary (HIGH confidence)
- **Live spike 实证（2026-09-06 本会话，herdr 0.8.100 / protocol 19）**：两轮 python PTY spike（双客户端几何/翻转链/resize）+ 三轮 wesh×herdr 真实二进制全链 smoke（双 pid/Welcome/ro ticket 全链/进程形态/清理序列）——§Pattern 3 全部断言行设计的数据源
- **源码一手核读（本会话 Read/Grep，附行号）**：~/open_src/herdr（src/server/headless.rs:4065-4090 渲染循环 per-client area + resize_panes=is_foreground；src/server/clients.rs:30-62,296-321 ClientConnection.terminal_size/render_targets；src/session.rs:96-185 socket 路径推导；src/server/autodetect.rs:189-236 server daemon 形态；src/config.rs:39-42 + src/config/io.rs:29-34,168-173 配置路径；src/cli/api.rs api 子命令面）；wesh（internal/server/clients.go:34-50,110-114 常量；internal/pty/spawn.go:128-181 env 白名单；e2e_test.go:120-194 / perclient_test.go:55-135 装配族母本；load_test.go 全文夹具层；ci.yml:17；web/uat/phase13.mjs/phase12-dom.mjs 尾部门禁形态）
- **本机实测**：tmux 3.6 `window-size latest` 默认值（隔离 socket `-L` + `-f /dev/null` 实测）；环境版本（go1.26.3/node v24.13.0/herdr 0.8.100/python 3.12.12）；web/uat node_modules 在位
- **项目档案**：14-CONTEXT.md（D-01..D-14 锁定决策）；PITFALLS.md :255-275/:340-349/:378-399（三维归类蓝本本体）；ARCHITECTURE.md §10 :475-486（资源账面表）；FEATURES.md 裁决 6/7（:57-66/:131）；13-08-SUMMARY（UAT 矩阵 15 脚本枚举与基线计数 :172-193）

### Secondary (MEDIUM confidence)
- WebSearch + WebFetch 交叉核对的 tmux window-size 语义（smallest/largest/manual/latest 四值语义——默认值以本机实测为准修正了网络二手资料的不确定性）

### Tertiary (LOW confidence)
- 无（全部关键论断均经实证或源码核读）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 零新依赖，全部既有件版本本机实测
- Architecture patterns: HIGH — harness 收编两族母本逐字核读；herdr 观测通道三轮 live 实证；负载格夹具逐函数核读
- Pitfalls: HIGH — 8 条中 6 条为本研究 spike/smoke 实踩或既有档案登记，2 条（P1/P7）为 PITFALLS 蓝本本体
- 负载矩阵绝对数值（ε 容差/160MB 账面）: MEDIUM — 账面推算，D-10/D-12 既定实测回填通道即此

**Research date:** 2026-09-06
**Valid until:** 2026-10-06（herdr 版本漂移为主要失效通道——0.8.100 钉定面若升级需重核 §Pattern 3 断言行；wesh 侧先例稳定）
