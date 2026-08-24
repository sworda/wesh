---
phase: 06
slug: session-lifecycle
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-24
---

# Phase 06 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 子进程退出状态→全部在线客户端 | 退出码/信号信息经服务端组帧广播给不可信客户端——文案内容与写序都不得被利用 | 退出码/信号名（非敏感，服务端硬编码模板） |
| lifecycle↔每客户端连接 | EXIT 直写与 Close 关闭帧共享同一 WS 写路径——stall 客户端不得拖延全局终结 | WS 控制帧 |
| 注册表状态→子进程进程组 | 客户端断开事件驱动 OS 级信号——信号目标必须恒为本会话进程组，误发即杀无关进程 | OS 信号（SIGHUP） |
| 计时器 goroutine↔hubMu 状态 | 宽限回调并发访问注册表——泄漏/双触发/竞态会把活会话静默杀死 | 内部状态 |
| 网络事件→重连状态机 | online/offline/onclose 事件驱动自动重连——触发面不得被放大为流量放大器 | WS 关闭码 |
| 重连 attempt→认证链 | 每次重连是新的完整认证——不得出现绕过 ticket 核销的静默通道 | ticket（一次性） |
| 服务端文案→面板 DOM | 面板正文/提示行渲染——钓鱼文案注入面 | UI 文案（textContent 渲染） |
| 命令行→进程配置 | flag 值驱动进程生命周期语义——畸形/矛盾输入必须 fail-fast（exit 2）而非带病启动 | CLI flag 值 |
| flag 值→stderr 错误串 | 解析错误经 flag 包包装打印——值内容回显面需显式论证 | flag 值（非敏感论证） |
| 测试驱动→真实服务端/二进制 | jsdom 页面与 Node 脚本连真实 spawn 实例——测试流量覆盖 06-01..06-04 全部新面 | 协议帧 |
| 敏感值→测试输出 | UAT 凭据/token 只作断言材料——测试输出可能进 CI 日志 | token/凭据（红线：永不入输出） |
| 文档→部署者 | README 示例与语义说明驱动真实部署——误导性文档是运维面 | 示例链接（占位符） |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-06-01a | Spoofing | EXIT 帧伪造（客户端注入终结提示） | low | accept | 结构性不可能：EXIT 为 S→C 类型字节，未知 C→S 字节 1002 直关（server.go 读循环 default 闸） | closed |
| T-06-01b | Tampering | EXIT message 渲染注入 | medium | mitigate | message 三形态服务端硬编码模板（server.go:1032-1035）；前端 textContent 直显（main.ts 全程 0 innerHTML）；JSON.parse 失败静默丢弃 | closed |
| T-06-01c | Denial of Service | stall 客户端拖延 exitf 广播 | high | mitigate | 同步 Write 带 2s 超时 ctx（server.go:1111）+ Close 内建上界 + 每客户端并行 goroutine + WaitGroup 有界等待；TestExitFrameBroadcast 与 06-06 S1/S2 锁定 | closed |
| T-06-02a | Tampering | SIGHUP 误发无关进程组 | high | mitigate | kill 目标恒为 -sess.Cmd.Process.Pid（setsid 组长，signal_linux.go:16 / signal_darwin.go:17）；不读外部 pid；TestSignalHangup 锁定 | closed |
| T-06-02b | Denial of Service | 计时器泄漏/双触发杀死活会话 | high | mitigate | 启停全在 hubMu 内（置 nil 防重复）；回调取 hubMu 复查；SIGHUP 幂等（ESRCH 静默）；exitf sync.Once 兜底；TestExitWhenEmptyGraceCancel/Expire 锁定 | closed |
| T-06-02c | Denial of Service | 启动即退出（注册表启动期恒空被当触发） | medium | mitigate | 事件=非空→空迁移，检测只挂 detach/kick 两移除点（removeLocked 成功后才判定），启动期零调用结构性免疫；严禁轮询 | closed |
| T-06-02d | Information Disclosure | logEvent 新事件泄露敏感值 | low | mitigate | 三要素 remote/code/reason 单行（server.go:975 签名锁定）；token/ticket/凭据值永不入参（SEC-01 红线）；reason 为静态机器串 | closed |
| T-06-03a | Denial of Service | 重连循环流量放大 | high | mitigate | 仅 1006 触发（reconnect.ts:14 shouldReconnect）+ 退避 1s×2 封顶 30s（reconnect.ts:8，最坏 2 次/分钟）+ 1013 不自动重连 + 终态码终止循环 | closed |
| T-06-03b | Tampering | Reconnecting 面板钓鱼文案 | medium | mitigate | title/body/hint 全前端硬编码常量（main.ts:425-443）；textContent 渲染无 innerHTML；EXIT message 同 T-06-01b 防线 | closed |
| T-06-03c | Spoofing | 重连绕过认证（免 ticket 静默通道） | high | mitigate | 结构性不存在：每次 attempt 走完整 connect() 重入链（fetch /api/attach → Hello 核销，main.ts:480+）；服务端 checkTicket 零特判 | closed |
| T-06-03d | Tampering | 陈旧 socket 代际事件污染新会话 | medium | mitigate | 全部 handler 入口 if (sock !== ws) return（main.ts 6 处守卫）；beforeunload 移除同受守卫；06-05 D6 场景断言锁定 | closed |
| T-06-04a | Information Disclosure | flag 值进错误串 | low | accept | duration 值非敏感；记录式 credErr/clientOptErr 仅用于值含敏感内容的 flag；校验文案仅含 flag 名与类别 | closed |
| T-06-04b | Denial of Service | 配置矛盾带病启动（--once × 显式矛盾值） | medium | mitigate | validateStartup 组合校验 fail-fast（exit 2，先于 pty.Start/net.Listen 零资源占用，cmd/wesh/main.go）；TestStartupMatrix 锁定 | closed |
| T-06-04c | Tampering | 语法糖展开覆盖用户显式值 | medium | mitigate | fs.Visit 显式设置判定先行——展开只填未显式位，显式矛盾值留给 validateStartup 拒绝（不静默改写用户输入）；TestParseArgs 锁定 | closed |
| T-06-05a | Information Disclosure | token/凭据值进测试输出 | high | mitigate | 红线注释沿用（phase06-dom.mjs 头部）：detail 只打状态码/布尔/形状/文案常量；Basic 凭据不经本脚本 | closed |
| T-06-05b | Denial of Service | 测试挂死拖死 CI | medium | mitigate | waitFor 轮询带上限（phase06-dom.mjs:67-71）+ 守候窗显式值 + 进程级 watchdog；spawn 实例全部 SIGKILL 收口（3 处） | closed |
| T-06-06a | Information Disclosure | token/凭据值进测试输出 | high | mitigate | 红线注释沿用（phase06.mjs:11-13）：detail 只打状态码/布尔/形状/退出码/文案常量；链接与 token 只存闭包变量 | closed |
| T-06-06b | Denial of Service | 宽限/退出场景时序假绿或挂死 | medium | mitigate | 余量关系注释明示（标称 1500ms vs 400ms 取消窗 + 5s 护栏）；waitExit 全带超时；spawn 实例全部 SIGKILL 收口（3 处） | closed |
| T-06-07a | Information Disclosure | 文档示例含真实 token | medium | mitigate | 示例链接占位符（README.md:68 `<ro-token>` 形态，05-09 红线同款）；人工核读入 acceptance_criteria | closed |
| T-06-07b | Tampering | 旧 UAT 适配削弱安全断言面 | high | mitigate | 适配白名单两类变更；06-07-SUMMARY 实证零适配（git 核读 web/uat 零 diff）；六段式全绿为证 | closed |
| T-06-SC | Tampering | 依赖安装供应链（×7 plan） | low | accept | 全 phase 零新依赖零安装（06-RESEARCH Package Legitimacy Audit：无输入，Gate 无需运行） | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-06-1 | T-06-01a | EXIT 帧伪造结构性不可能：S→C 类型字节在 C→S 类型空间（'0'/'1'/'H'）无映射，未知 C→S 字节 1002 直关（server.go 读循环 default 闸既有） | planner (06-01-PLAN) | 2026-08-24 |
| AR-06-2 | T-06-04a | duration 类 flag 值非敏感；记录式 credErr/clientOptErr 纪律仅用于值含敏感内容的 flag，本 flag 族校验文案仅含名称与类别 | planner (06-04-PLAN) | 2026-08-24 |
| AR-06-3 | T-06-SC（全 7 plan） | 全 phase 零新依赖零安装——供应链面无输入，Package Legitimacy Gate 无需运行（06-RESEARCH 审计） | planner (06-0{1..7}-PLAN) | 2026-08-24 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-24 | 24 | 24 | 0 | gsd-secure-phase (ASVS L1, grep-depth；register 全部 plan-time 著述，SUMMARY Threat Flags 全 None 无未登记面) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-24
