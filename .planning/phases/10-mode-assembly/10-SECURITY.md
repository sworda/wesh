---
phase: 10
slug: mode-assembly
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-03
---

# Phase 10 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CLI flag 值 / TOML 文件 → parseArgs/decodeFileConfig | operator 输入经启动面进入进程；错误输出落 stderr（systemd journald 持久化面） | 枚举值（非敏感豁免面）/ 配置值（值剥离纪律） |
| config（CLI/TOML 合并终值）→ validateStartup | 启动校验纯函数；只读探测（os.Stat/exec.LookPath）零资源占用 | flag 名/命令名（非敏感回显） |
| main run() → server.New | 装配契约边界——SessionMode×SpawnFunc 失配属程序错误 fail-fast | Options 结构体（进程内） |
| TOML 文件 / fuzz 输入 → decodeFileConfig | 严格模式 + 值剥离既有防线；新键扩展不得弱化（fuzz 运行时自证） | 任意字节（崩溃面/红线破口面） |
| 文档（CONFIGURATION/README）→ 用户 | 文档是配置行为依据——超前描述诱导误配置 | 行为语义（无敏感数据） |
| 进程 stderr → 操作员/日志收集 | 启动错误文案跨出进程边界，SEC-01 红线适用 | 错误文案（键名-only/豁免面枚举值） |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-10-01a | Information Disclosure | 非法枚举值拒绝文案（CLI/TOML 双源） | medium | mitigate | D-04 定案文案（值域两固定单词无秘密可泄）；configErr 值剥离保持；TestTLSKeyPairError/config 红线子测锁定 | closed |
| T-10-01b | Tampering | 近形模式值宽容归一 | medium | mitigate | 精确匹配枚举闸（== 两常量）；banana 拒绝矩阵锁定；fuzz 近形种子 | closed |
| T-10-01c | Elevation of Privilege | per-client 接缝误激活（SpawnFunc 被调用） | high | mitigate | inert 纪律：SpawnFunc 调用形态 grep 零匹配（2026-09-03 复验）；run() 单分岔静态结构；ValidateOptions 互斥 fail-fast | closed |
| T-10-02a | Information Disclosure | warn/err 文案含敏感值 | low | mitigate | warn 文案只含双 flag 名不含值；%q 回显命令名（非敏感豁免面）；wantWarnSub/wantErrSub 形态锁 | closed |
| T-10-02b | Tampering | write-policy×per-client 组合静默放行 | high | mitigate | D-01/D-02 warn 通道强制；冒烟实证 warn 行 + listening on（S6，2026-09-03 复验）；CONFIGURATION.md warn 清单文档化 | closed |
| T-10-02c | Denial of Service | LookPath 预检误伤 shared 路径 | medium | mitigate | 预检仅 per-client × argv0 非空触发；shared 对照冒烟零漂移（S11，2026-09-03 复验） | closed |
| T-10-02d | Information Disclosure | 新 warn 遮蔽既有安全警告 | medium | mitigate | mergeWarn 合并拼接形态；冒烟实证两类警告同现（S12，2026-09-03 复验）；既有文案逐字未动 | closed |
| T-10-03a | Information Disclosure | 新键断言面值泄露 | low | mitigate | 键名-only 断言纪律；stripKeyNameEcho 机制零改动 | closed |
| T-10-03b | Tampering | 下划线 session_mode 键宽容接受 | low | mitigate | DisallowUnknownFields 结构性拒绝 + redline 子测 + fuzz 种子；冒烟 rc=2 `unknown keys (session_mode)`（2026-09-03 复验） | closed |
| T-10-03c | Tampering | TOML 非法枚举走第二校验路径（双写漂移） | medium | mitigate | 一闸双覆盖：config.go 零枚举逻辑；redline 子测断言与 CLI 同文案；冒烟双源同文案（S1/S2，2026-09-03 复验） | closed |
| T-10-04a | Information Disclosure | 文档把未实现行为写成事实 | low | mitigate | D-05「装配中，当前版本与 shared 等价」三处同文（键表/默认值表/README，2026-09-03 复验×3）；--help 口径一致 | closed |
| T-10-04b | Tampering | 收口验证断言放宽 | high | mitigate | 八 UAT 脚本原样重跑 exit 全 0（2026-09-03 复验，PASS 计数对齐基线）；git status/diff web/uat 零输出（2026-09-03 复验）；测试文件 append-only 零删除行 | closed |
| T-10-05-01 | Tampering | SC4 预检解析基准发散（WR-01） | high | mitigate | 预检与 spawn 同基准（--cwd join + os.Stat）；六形态进程级冒烟双向实证（2026-09-03 复验 S3/S8/S9/S10/S11 + S7）；三行 TestStartupMatrix 行为锁 | closed |
| T-10-05-02 | Information Disclosure | 预检错误文案（stderr） | low | mitigate | 文案仅 %q argv0 + 固定类别语；冒烟四腿文案逐字命中（2026-09-03 复验）；零凭据/token | closed |
| T-10-05-03 | Denial of Service | 守卫触发零回滚（WR-02） | medium | mitigate | ValidateOptions 前移：V(1342) < P(1348, pty.Start) < L(1356, net.Listener) 相对位序保持（2026-09-03 复验；行号偏移源自 10-05 后 fix 提交 34279d1/b007c3a，相对顺序不变）；单调用点计数门 == 1 | closed |
| T-10-SC-01/02/04 | Tampering | 依赖供应链（go get/包安装） | high | accept | 零新依赖：go.mod/go.sum diff 781af48..HEAD == 0 行（2026-09-03 复验）；无包安装任务 | closed |
| T-10-05-SC | Tampering | npm/pip/cargo 安装 | — | not applicable | 本 phase 零包安装——Package Legitimacy Gate 不触发 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-10-01 | T-10-SC-01/02/04 | 依赖供应链风险接受：Phase 10 零新依赖（go-toml v2 既有机制覆盖新键）、go.mod/go.sum 零漂移，无新增供应链面 | GSD secure-phase（ASVS L1 短路） | 2026-09-03 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-03 | 17 | 17 | 0 | verify-work 编排器（ASVS L1 短路路径：threats_open 0 + plan 期登记 + asvs_level 1，grep 级证据五组复验：SpawnFunc 零调用形态 / V<P<L 相对位序 / go.mod 零漂移 / web-uat 零修改 / 下划线拒绝冒烟） |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-03
