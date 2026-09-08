---
phase: 14
slug: herdr-uat
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-08
---

# Phase 14 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
>
> Phase 14 为纯验证收口 phase（零生产代码变更）：双模式测试矩阵改造 + herdr UAT + 负载标定 + 文档 + mermaid 修复。威胁面以「证据链完整性 Tampering」为主导类别。

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 测试改造面 → 零回归证据链 | shared 列期望值是 v1.0 行为契约的证据本体——改造期字面漂移即证据篡改面（14-01..14-06 共用） | 测试断言字面量（高完整性要求） |
| 收口闸 → 里程碑证据链 | 14-12 六段式与 diff 终审是 v1.1「零回归 + 恢复正确行为」声明的最终担保面——放水即证据链断裂 | 全 phase diff / CI 产物 |
| 负载格 → 本机资源 | 32 会话 × 33.8MB 洪水子进程群的 CPU/内存冲击面（14-07） | 进程/内存/fd 账面 |
| UAT 脚本 → 用户日常 herdr 环境 | 测试驱动真实 herdr server，与用户 default 会话同机——命名会话隔离是唯一防线（14-08/09） | herdr 会话状态（隔离纪律） |
| UAT 脚本 → 控制台/日志 | 分享链接 token/ticket/pid 经 check detail 或异常输出的泄漏面（14-08） | 认证凭据 / pid |
| wesh 子进程群 → 本机资源 | herdr client/server、wesh 实例的进程残留面（14-08/10） | 进程生命周期 |
| Windows 工作站 → Linux 开发机 | pw 脚本经 ssh 管理 Linux 侧 wesh/herdr——凭据与命令通道面（14-09） | ssh 凭据 / 命令通道 |
| 浏览器 → TCP 转发器 → wesh | 真实 Chromium 全链——测试凭据（CRED）在页面/截图中的可见面（14-09） | 一次性测试凭据 |
| runner → 16 个真实二进制 spawn 子脚本 | 矩阵长跑（~5min 量级）的进程残留与挂死面（14-10） | 子进程生命周期 |
| 文档面 → 用户行为 | 模式语义/配方/标定表是用户部署决策的依据面——失实文档即误导面（14-11） | 配方/标定数据（真实性） |
| 开发机 pnpm ↔ npm registry | scripts/ 局部安装 mermaid/jsdom（dev 校验工具面，不进生产二进制/CI/web 构建，14-13） | devDependencies（钉版 ^11.17.2/^30.0.1） |
| GitHub 渲染器 ↔ ARCHITECTURE.md 内容 | 公开架构文档渲染（无敏感数据——文档本为发布面，14-13） | 公开文档内容 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-14-01 | Tampering | mode-mapped 断言改造被「和稀泥」，shared 零回归证据毁灭 | high | mitigate | 断言分叉表强制（mode → expected 显式表）+ shared 列逐字未动 git diff 自审 + 14-12 段⑥ diff 白名单逐行终审 | closed |
| T-14-02 | Tampering | newTestServer 小族收编点薄包装引入装配行为偏差 | medium | mitigate | 四形态两分支直传零改写 + tracked 形态运行期实证（limits:132 stderr）+ 双模式 -race 绿先证后推 | closed |
| T-14-03 | Tampering | fanout/重连两模式真值相反面被「和稀泥」改写 | high | mitigate | 断言分叉表显式成表 + shared 列逐字未动 + 14-12 段⑥ 终审（35 文件零外改动/565 真删除行全归类/红线四查全零） | closed |
| T-14-04 | Tampering | per-client 列与 perclient_test.go 既有测双写漂移 | medium | mitigate | read_first 强制对照既有测 + 各列只锁蓝本指定分叉点 | closed |
| T-14-05 | Tampering | 归一搬运中改写既有 per-client/shared 断言字面 | high | mitigate | 两列逐字搬入纪律 + git diff 自审 + 14-12 段⑥ 终审；14-03 SUMMARY Threat Flags 字面清单自审闭环 | closed |
| T-14-06 | Tampering | per-client-only 测被误删或强行加 shared 分支 | medium | mitigate | 归属逐测判定 + 保持不动清单登记 SUMMARY（RESEARCH OQ2 裁决：默认不动） | closed |
| T-14-07 | Tampering | mode-exclusive 的 per-client 分支写成 t.Skip/空断言 | high | mitigate | 可证伪断言强制（零 'W' 帧/各自直通观测面）+ 验收闸 grep 零 t.Skip + 14-12 终审 | closed |
| T-14-08 | Tampering | resize 分叉表「和稀泥」（直通与仲裁两语义都被接受） | high | mitigate | 断言分叉表显式成表 + shared 列逐字未动 + 14-12 段⑥ 终审 | closed |
| T-14-08b | Tampering | 蓝图归属误记被静默跟改或重复登记 | low | mitigate | Task 2 单点登记（SUMMARY + CONTEXT 回写双通道一致）+ 14-06 收口核对引用而非重登记 | closed |
| T-14-09 | Tampering | 同断言双跑退化为期望值削弱（「两模式都能过」形态） | high | mitigate | 两列同值且与改造前逐字一致 git diff 自审 + 14-12 段⑥ 终审；auth_e2e 断言字面量全集 diff 零差异（14-05 SUMMARY） | closed |
| T-14-10 | DoS | 洪水类测双跑致 CI 时长超阈值 | low | accept | 时长实测登记 SUMMARY（AR-1）；全量 -race 五包 2m37.986s 实测未超 CI 时限 | closed |
| T-14-11 | Tampering | 部署面红线断言（XFF 信任闸/sanitize/base-path）双跑期望值削弱 | high | mitigate | 两列同值逐字未动 + 14-12 段⑥ 终审；9 测双模式 PASS（UAT #31） | closed |
| T-14-12 | Tampering | 三维归类静默漏网（某 server 装配测未双跑且无登记） | medium | mitigate | 33 文件逐一归类收口核对 + 蓝图逐行对照 + 偏差登记（mode= 子测试计数 216 精确一致，UAT #32） | closed |
| T-14-13 | DoS | 洪水/驻留格子进程群泄漏抢资源 | medium | mitigate | startPerClientServerWithSpawn Cleanup 逐一 Kill+Close 追踪纪律 + 30m timeout 护栏 + load tag Linux 手动跑隔离；14-12 残留核验 pgrep wesh 零命中 | closed |
| T-14-14 | Tampering | 标定诚信面：为让账面成立调松断言常量/删格 | medium | mitigate | ε 容差注释论证强制 + D-11 两段式（证伪升级 one-way 门而非自行调整）+ LOADDATA 原始行入 SUMMARY 留档；load 双剖面八格与基线逐项一致（14-12 段⑤） | closed |
| T-14-15 | Tampering | herdr 命名会话与用户 default 会话互污染（误停/误连 default） | high | mitigate | D-09 独立会话 + HERDR_SOCKET_PATH 定向 + 跑后 session list 核验；14-12 残留核验——herdr session list 零 wesh-uat-* 残留、default 会话全程零触达 | closed |
| T-14-16 | Information Disclosure | token/pid 泄入 check detail/控制台 | medium | mitigate | 红线件三通道（sensitiveTokens/sensitivePids/emittedDetails）+ assertOutputClean() 运行时自净——五面零泄漏（14-08 D3，UAT #29 pass） | closed |
| T-14-17 | DoS | herdr server（wesh 死后存活是特性）/wesh/子进程残留 | medium | mitigate | finally 清理序列（WS close → SIGTERM → session stop → list 核验）+ 场景间 300ms 间隔 + 异常纳入计数不跳清理；S1j/S2g 会话零残留（UAT #33） | closed |
| T-14-18 | Tampering | 双机拓扑越界（Linux 侧装浏览器/Windows 侧动真实网卡） | high | mitigate | CODEBUDDY.md 硬约束 + 脚本 grep 自审（无安装/网卡动作）+ 转发器 kill/restore 先例形态；pw 永不进 Linux 矩阵（14-12 段⑤） | closed |
| T-14-19 | Information Disclosure | 测试凭据 CRED 泄入截图/日志 | low | accept | CRED 为一次性测试凭据（AR-2）；截图人工复核顺带覆盖（14-09 两轮 4/4 + 截图六帧） | closed |
| T-14-20 | DoS | pw 场景 herdr 命名会话/Chromium 进程残留 | medium | mitigate | finally 清理序列（browser.close → fwd.stop → stopWesh → session stop → list 核验）；14-09 Task 2 用户确认 + 14-12 残留核验 | closed |
| T-14-21 | DoS | 子脚本挂死致矩阵永挂 / 中途失败致进程残留 | medium | mitigate | 逐脚本 10min 超时护栏 kill 转 FAIL + 串行异常不跳后续 + 跑后残留核验；run-all.mjs 17/17 零修改重跑 210.9s（14-12 段⑤） | closed |
| T-14-22 | Information Disclosure | runner 透传面新增 token/pid 打印 | low | accept | runner 零断言零敏感值打印红线（头注释载明，AR-3）；敏感值自净归各子脚本既有 assertOutputClean 面 | closed |
| T-14-23 | Information Disclosure | 文档失实（未实证配方/编造标定数据/失实 tmux 对照）误导用户部署 | medium | mitigate | 文档即被测物：配方与 phase14.mjs argv 逐字核对 + 标定表与 14-07 LOADDATA 逐项核对 + tmux 措辞红线 grep 负断言（14-11 验收闸） | closed |
| T-14-24 | Tampering | 误记修正引入新失真（改写 :7 时上下文语义断裂） | low | mitigate | 原文逐字核读后改写 + 上下文连贯性自审（14-11 Task 2） | closed |
| T-14-25 | Tampering | 收口以放宽断言/跳过失败项换绿（证据链完整性破坏） | high | mitigate | diff 白名单逐行终审 + 零跳过红线（prohibitions 两条）+ Task 2 人工闸 blocking——用户 approved 2026-09-07；6 skipped 全平台豁免带 reason 零跳过测试类项 | closed |
| T-14-26 | Tampering | 白名单外漂移混入（go.mod/ci.yml/dist/非声明文件） | high | mitigate | Task 3 命令化零 diff 断言——go.mod/go.sum/lockfile/ci.yml/dist byte-identical 实证（dist md5 d5c25e27 与 13-08 一致） | closed |
| T-14-SC (×13 PLAN) | Tampering | 依赖面（go.mod/go.sum/lockfile/package.json）——13 个 PLAN 各自登记的同一红线 | high (12) / medium (14-13) | mitigate | 零新依赖红线终验：14-12 SUMMARY dependencies.added: []——go.mod/go.sum + web 依赖清单五文件 + .github/（ci.yml）基点以来 0 行 diff；14-13 的 scripts/ 局部安装（mermaid ^11.17.2/jsdom ^30.0.1 钉版 + lockfile 冻结 + 不进 CI/生产/web 构建）为诊断阶段钦定路线 | closed |
| T-14-35 | Tampering | scripts/check-mermaid.mjs 恒绿假阳 | low | mitigate | Task 1 负对照自证：脚本对当前含词法错误的块必须 FAIL exit 1（先例同 14-08 D3 exit-code 门禁负对照）——try/catch 吞错实现过不了负对照门 | closed |
| T-14-36 | Information Disclosure | docs/ARCHITECTURE.md 修复面 | low | accept | 公开架构文档（v1.0 起随仓库发布，AR-4），修复仅动 mermaid 块内部语法形态，零新增敏感信息 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-1 | T-14-10 | 洪水类测双跑 CI 时长风险：全量 -race 五包实测 2m37.986s 未超限；若未来超 CI 时限按 D-02 偏差登记流程再议分批（形态不变） | PLAN 14-05 (plan-time) | 2026-09-06 |
| AR-2 | T-14-19 | 测试凭据 CRED 为一次性凭据（lib/browser.mjs 头注释纪律继承），泄漏影响限于测试面；截图人工复核顺带覆盖 | PLAN 14-09 (plan-time) | 2026-09-06 |
| AR-3 | T-14-22 | runner 零断言零敏感值打印红线（头注释载明）；敏感值自净归各子脚本既有 assertOutputClean 面 | PLAN 14-10 (plan-time) | 2026-09-06 |
| AR-4 | T-14-36 | 公开架构文档修复仅动 mermaid 块内部语法形态，零新增敏感信息 | PLAN 14-13 (plan-time) | 2026-09-08 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-08 | 30 (T-14-SC ×13 PLAN 实例) | 30 | 0 | verify-work 14 → secure-phase（L1 短路：threats_open 0 + register_authored_at_plan_time 13/13 + asvs_level 1） |

**证据锚点**（L1 grep 深度，全部为已落档产物）：
- 14-12-SUMMARY 六段式收口闸：静态面三命令零输出 / 全量 -race 五包 2m37.986s（mode= 216/216）/ darwin 双编译四命令零错 / dist byte-identical（md5 d5c25e27）/ UAT 矩阵 17/17（210.9s，6 skipped 全平台豁免带 reason）/ diff 白名单终审 35 文件零外改动、565 真删除行全归类、红线四查全零 / 残留核验 pgrep wesh 零命中 + herdr session list 零 wesh-uat-* 残留 + default 会话全程零触达 / dependencies.added: []
- 14-03-SUMMARY Threat Flags：无新增安全面（T-14-05/06/SC 字面清单自审闭环）
- 14-08 D3：assertOutputClean 五面零泄漏 + exit code 门禁负对照自证（UAT #29 pass）
- 14-09 Task 2：Windows 侧两轮 4/4 + 截图六帧人工复核（用户确认 2026-09-07）
- 14-13-SUMMARY：check-mermaid.mjs 负对照自证（对含错块 FAIL exit 1）+ mermaid.parse 全 3 块 PASS + 用户渲染目检 approved
- 14-UAT.md：34/34 pass（含冷启动全仓 -race 五包 ok、协议层 UAT 18/18、部署面 9 测双模式、mode= 计数 216）

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-08
