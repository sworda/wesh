# Phase 14: 双模式验证矩阵、标定与 herdr UAT - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-06
**Phase:** 14-herdr-uat
**Areas discussed:** 三维归类与 CI 门形态, herdr UAT 载具与断言面, 负载矩阵形状与回填, 协议层 UAT 增量与文档段

---

## 三维归类与 CI 门形态

### Q1: 双模式 -race 全量门的执行机械

| Option | Description | Selected |
|--------|-------------|----------|
| t.Run 子测试双跑（推荐） | PITFALLS :264 推荐形态：测试内部 t.Run 两模式子测试 + 断言分叉表；CI 单 step 天然双模式，CI 结构零新 leg；代价 = 40+ 文件逐文件改造 | ✓ |
| env 变量全量重跑 | 测试文件零改造，CI 加第二 step；CI 时长 ×2，mode-mapped 分叉仍逃不掉 | |
| 混合 | mode-agnostic 经 env 双跑；mode-mapped 文件内分叉表 | |

**User's choice:** t.Run 子测试双跑
**Notes:** CI 结构零改动（darwin leg 自动双模式）是该形态的重要收益。

### Q2: PITFALLS :378-399 归属表与实测冲突时的处置

| Option | Description | Selected |
|--------|-------------|----------|
| 蓝本为准 + 偏差登记（推荐） | 按蓝本落地；实测误判时 plan 偏差登记 + CONTEXT 回写，不静默跟改 | ✓ |
| 先全量审计定稿再改 | 改造前 40+ 文件全量审计产出定稿表 | |
| Claude 裁量 | harness 设计使归属切换成本最低 | |

**User's choice:** 蓝本为准 + 偏差登记

### Q3: mode-exclusive 测试的 per-client 分支形态

| Option | Description | Selected |
|--------|-------------|----------|
| 显式断言未装配（推荐） | per-client 分支断言「组件未装配」（无 min-rect / 零 'W' 帧）——可证伪防线，防装配漂移腐化 | ✓ |
| t.Skip 带理由 | 最省事但「不装配」无防线 | |
| 混合按可断言性分 | 能断言的断言，纯 shared 语义的只跑 shared | |

**User's choice:** 显式断言未装配

### Q4: 零回归双证据 UAT 侧形式化

| Option | Description | Selected |
|--------|-------------|----------|
| runner 脚本 + 人工收口（推荐） | run-all.mjs 一键矩阵（10 协议脚本 + jsdom + per-client 三脚本），人工收口闸 | ✓ |
| 协议 UAT 进 CI | ubuntu leg 跑全量协议脚本；flake 面与时长不合算 | |
| 保持逐脚本人工跑 | 不建 runner | |

**User's choice:** runner 脚本 + 人工收口
**Notes:** pw 层永远 Windows 侧进不了 CI（双机拓扑硬约束）是 UAT 不进 CI 的结构性理由。

---

## herdr UAT 载具与断言面

### Q1: SC3 driving 载具选型

| Option | Description | Selected |
|--------|-------------|----------|
| 仅真 herdr（推荐） | herdr per-client area 渲染是唯一可实证面；tmux 多客户端仍 min-size 压缩（自身语义）只能文档对照 | ✓ |
| herdr + tmux 对照双跑 | 对照证据更强但复杂度翻倍 + tmux 配置漂移 flake 风险 | |
| tmux 替代 | 语义上无法实证 SC3 核心断言，等于放弃 driving scenario 验收 | |

**User's choice:** 仅真 herdr
**Notes:** 本机 herdr 0.8.100 已装（~/.local/bin/herdr），源码在 ~/open_src/herdr。

### Q2: Linux 协议层断言深度

| Option | Description | Selected |
|--------|-------------|----------|
| 双层断言 + API 观测（推荐） | wesh 层（双 pid + winsize 隔离）+ herdr 行为层经 socket API 结构化断言；观测通道先 spike 标定 | ✓ |
| 仅 wesh 层 + pw 观感 | 协议层简单但 driving bug 无协议证据 | |
| 输出流嗅探 | 不依赖 herdr API 但版本漂移即碎 | |

**User's choice:** 双层断言 + API 观测
**Notes:** ROADMAP Research flag 既定 herdr 断言设计需实测标定——spike 先行。

### Q3: Windows Playwright 观感断言形态

| Option | Description | Selected |
|--------|-------------|----------|
| buffer 断言 + 截图留档（推荐） | xterm buffer 文本/光标位置断言（phase06-pw 先例）+ 截图人工复核（像素豁免先例） | ✓ |
| 仅截图 + 全链跑通 | 只证明真实浏览器全链跑通 | |
| 截图 diff 回归 | 最严格但 herdr 渲染闪烁面 flake 风险高 | |

**User's choice:** buffer 断言 + 截图留档

### Q4: herdr UAT 场景集范围

| Option | Description | Selected |
|--------|-------------|----------|
| driving + ro 汇聚（推荐） | 核心 driving（移动端不压缩桌面端）+ ro 分享链接汇聚（裁决 7 文档叙事实证防线） | ✓ |
| 仅核心 driving | ro 汇聚靠文档与 phase12 既有断言拼接 | |
| 全量含仲裁 | + is_foreground last-attach-wins 全量实证；spike 工作量最大 | |

**User's choice:** driving + ro 汇聚
**Notes:** is_foreground 仲裁证据若 socket API 现成则带上，不强求。

---

## 负载矩阵形状与回填

### Q1: per-client 负载矩阵剖面形状

| Option | Description | Selected |
|--------|-------------|----------|
| 洪水 + 驻留双剖面（推荐） | N∈{1,4,16,32} × seq 洪水 + N 会话驻留空转格（32 shell ~160MB 账面实证）；churn 格已有不重复 | ✓ |
| 仅洪水剖面 | 驻留账面由 churn 终点采样顺带覆盖 | |
| 三剖面含混合 | +部分洪水部分 idle 混合负载 | |

**User's choice:** 洪水 + 驻留双剖面

### Q2: maxClients=32 默认值处置

| Option | Description | Selected |
|--------|-------------|----------|
| 不动默认 + 数据回填（推荐） | 零契约变更；README 资源义务段 + 建议值表；数据证伪再开 one-way 门（13-01 D-01 先例） | ✓ |
| per-client 默认改小 | 一次性契约变更；未实测先改的风险 | |
| 默认不动 + 启动 warn | validateStartup warn 通道先例 | |

**User's choice:** 不动默认 + 数据回填
**Notes:** 负载矩阵因此是「默认 32 是否成立」的实证裁决工具。

### Q3: 驻留格内存「有界」断言口径

| Option | Description | Selected |
|--------|-------------|----------|
| wesh 硬断 + 子进程观测（推荐） | Alloc 增量 ≤ N×(768KiB+ε) 硬断言；子进程 VmRSS 观测入 LOADDATA + ≤15MB/进程宽松防线 | ✓ |
| 仅 wesh 侧断言 | 子进程内存完全不断言 | |
| 双侧硬断言 | 判别力最强但 bash RSS 环境因子 flake | |

**User's choice:** wesh 硬断 + 子进程观测

---

## 协议层 UAT 增量与文档段

### Q1: SC2 协议层 UAT 增量形态

| Option | Description | Selected |
|--------|-------------|----------|
| phase14.mjs = herdr 场景（推荐） | 六项已被 phase11/12/13 覆盖不重复断言；run-all 重跑即 SC2 证据，避免双写漂移 | ✓ |
| 汇总脚本 + herdr 独立 | 验收叙事清晰但断言重复 | |
| 不建新协议脚本 | herdr 只建 pw 层，协议层 API 观测面放弃（与区域 2 决策冲突） | |

**User's choice:** phase14.mjs = herdr 场景

### Q2: PC-12 模式语义文档分布与深度

| Option | Description | Selected |
|--------|-------------|----------|
| README 主承载 + 双 docs 补段（推荐） | README 会话模式节（语义表 + herdr 配方 + 资源义务段）；ARCHITECTURE.md 双模式段（mermaid）+ :7 误记修正；CONFIGURATION.md max-clients 语义行 | ✓ |
| 独立 SESSION-MODES.md | README 精简但核心卖点可达性降低 | |
| 最小字面满足 | 无配方示例无 mermaid 图 | |

**User's choice:** README 主承载 + 双 docs 补段
**Notes:** REQUIREMENTS.md:50 D-15 历史注记不改写（裁决档案保持当时语境）。

### Q3: herdr UAT 环境隔离与版本漂移纪律

| Option | Description | Selected |
|--------|-------------|----------|
| 独立会话 + 版本钉定（推荐） | herdr --session wesh-uat-* 隔离 + 跑完清理；断言基于 0.8.100 API 形态，失败先核版本 | ✓ |
| 复用日常 server | 互相污染 | |
| Claude 裁量 | 随 spike 标定结果定 | |

**User's choice:** 独立会话 + 版本钉定

---

## Claude's Discretion

（以下由用户明示或在讨论中归为 Claude 裁量）

- newTestServer(t, mode) 精确签名与两装配族收编形态；40+ 文件改造 wave 切片（建议按归属类别分批）
- LOADDATA 行 per-client 扩展字段与 README 标定表回填精确格式
- herdr socket API 具体调用面与断言行（spike 标定产物）
- phase14-pw.mjs 断言行颗粒度、TCP 转发器拓扑复用、双 tab viewport 尺寸选型
- run-all.mjs 脚本形态（串行/exit code 聚合/汇总表）
- ARCHITECTURE.md mermaid 图精确内容与 :7 误记改写措辞
- herdr 独立会话/socket 精确隔离形态与清理序列
- 负载矩阵 darwin 口径（沿用先例：load tag Linux 手动跑）

## Deferred Ideas

- pinger 区分写阻塞/pong 超时（13-CONTEXT D-10 已接受 1006 语义；herdr UAT 实证是证伪通道）
- 令牌桶/dwell 调优入口（12/13-CONTEXT 既定，常量改值非契约变更）
- v1.1.0 发布闸（非本 phase；走 /gsd:complete-milestone + release.sh）
- tmux 对照 UAT 脚本（文档一句对照替代）
- maxClients per-client 默认值变更（D-11 两段式后半段：仅数据证伪时经 one-way 确认门）
