---
phase: 13
slug: resource-defense
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-09-06
---

# Phase 13 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 操作者→CLI/配置面 (13-01) | 启动期默认值决定 HUP 免疫泄漏防线开关——未显式设置的默认路径是不可信省略面 | stop-timeout 默认值（HUP 免疫进程泄漏防线开关） |
| client→server attach 频率 (13-02) | 已认证客户端 connect→spawn→disconnect 频率是 churn 防线的不可信输入面——fork 预算保护必须在 spawn 调用点之前 | attach 频率 / fork 预算 |
| 反代→server XFF 头 (13-02) | XFF 值只在 --auth-header 信任闸开启时被采信为桶键（07-03 既有信任边界），伪造头在 trust off 时零影响 | XFF 头值（IP 归因键） |
| 子进程→server 退出时序 (13-03) | 「先断后死」「先死后断」两时序与 Wait-return→hubMu-acquire 微窗口是终结语义的不可信时序面 | 进程退出时序 / pgid 生命周期 |
| OS 信号→server 关停时序 (13-04) | SIGTERM/SIGINT 到达时各会话所处生命周期阶段（在线/断开待收割/收割中）是不可信时序面 | 关停信号 / 会话生命周期状态 |
| server→运维通道 (13-05) | /metrics /healthz /审计日志观测面输出是跨信任边界的数据出流——身份粒度与敏感值是红线面 | metrics series / 事件键集 / 探活语义 |
| 反代→server auth-header 头值 (13-06) | 透传用户名是不可信输入——经 SEC-07 sanitize 清洗后才允许跨进子进程 env 边界 | 透传用户名（WESH_REMOTE_USER 值） |
| server→子进程 env 注入 (13-06) | env 是子进程的能力面——键名/值的合法性由 pty 包白名单单侧收口 | 子进程 env 键值对 |
| client→server churn 注入面 (13-07) | 压测自身即防线攻击面的受控重放——断言失败 = 防线破口 | churn 负载（RSS/goroutine/fd 采样） |
| 收口闸→交付物 (13-08) | 收口是本 phase 全部防线与语义承诺的终验面——放宽断言换绿即整个 phase 证据链失效 | 全部交付物 diff / 依赖清单 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-13-01 | DoS | per-client stop-timeout 默认 0（HUP 免疫进程泄漏 ×N） | high | mitigate | D-01 默认 5s KILL 兜底默认开启（main.go:1306 实证）+ TestStopTimeoutResolution 三态锁定；trap '' HUP 进程级实证由 13-07 S2 承载 | closed |
| T-13-02 | Tampering | 显式位误判（「未设置」误判为「显式 0」或反向覆盖） | medium | mitigate | fs.Visit + TOML 键存在双源置位 + CLI/直构/TOML 三态测试组 + warn 负例 | closed |
| T-13-03 | Information Disclosure | warn 文案含敏感值 | low | accept | 文案定值无插值（main.go 既有红线注释），零新风险面 | closed |
| T-13-04 | DoS | 已认证 churn fork bomb（connect→spawn→disconnect 死循环） | high | mitigate | spawn 双令牌桶（全局 8/s burst 16 + per-IP 1/s burst 4，spawnthrottle.go）+ TestPerClientSpawnThrottle 三通道；churn 压测 RSS/goroutine/fd 有界断言由 13-07 承载 | closed |
| T-13-05 | DoS | 断网惊群（N 浏览器同时自动重连 = N 个 fork+exec） | high | mitigate | 全局桶 8/s 上限 + TestPerClientSpawnGlobal 独立判定 + 前端 30s 封顶退避既有（06-03） | closed |
| T-13-06 | DoS | 节流拒绝误入 1006 → 前端重连放大 fork 循环 | medium | mitigate | 1011 锁定（StatusInternalError 常量，不在 shouldReconnect 触发集）+ tracer 逐值断言 close code | closed |
| T-13-07 | DoS | per-IP 桶键不换 XFF → 反代部署全部用户共享一桶误伤 | medium | mitigate | 桶键 = proxy.clientIP(r) 同源（D-05）+ TestPerClientSpawnXFF 两态（trust on 异键 / trust off 共享回退键） | closed |
| T-13-08 | DoS | per-IP map 无界增长（IP 基数放大内存） | low | mitigate | 15min 惰性过期 + TestSpawnThrottleExpiry now 注入精确计数 + 56B/条 × 4096 IP ≈ 230KB 内存界 | closed |
| T-13-09 | DoS | 空即不死——per-client --once/--exit-when-empty 永不退出 | high | mitigate | pcSupervisor/pcExitReq 第二终结源（server.go:206 实证）+ 三形态触发端 + 三形态测试锁定 | closed |
| T-13-10 | Tampering | kill-after-reap 误杀复用 pgid（微窗口内 reaped 误读） | medium | mitigate | WR-02 waitDone 非阻塞 select 结构性栅栏 + TestPerClientReapedFence 白盒构造；既有 reaped 栅栏保持 | closed |
| T-13-11 | DoS | exitf 多触发源竞态双收口 | medium | mitigate | termOnce/terminate 单点复用（server.go:241 实证）+ pcSupervisor 出循环后唯一调用点 + -race 全量绿 | closed |
| T-13-12 | Information Disclosure | session_end 事件敏感值 | low | accept | schema 复用 shared 母本（四键全数值/关联键），零敏感值经 emitEvent 既有通道 | closed |
| T-13-13 | DoS | Shutdown 无界 join 被 D-state 不可杀进程拖死 | high | mitigate | 有界 join（stopTimeout+余量 AfterFunc 兜底 Broadcast）+ 到期未清零 Shutdown 侧 terminate 无条件收口 + 形态三/四测试锁定（D-state 代理 = stopTimeout 0 + HUP 免疫） | closed |
| T-13-14 | DoS | 「断开未收割」残留会话漏信号 → 服务端退出后残留进程 | high | mitigate | 快照源 = pcSessions 非 registry + 残留形态测试（残留在册实证 + 快照链覆盖 + session_end 恰 1） | closed |
| T-13-15 | Tampering | 双触发竞态（detach 链与 Shutdown 快照链重复信号/重复收口） | medium | mitigate | teardownOnce 幂等承接 + 快照信号 WR-02 waitDone 栅栏 + 补 KILL 回调 reaped 复检双防线 | closed |
| T-13-16 | Information Disclosure | metrics 新 series 带身份 label → 基数爆炸 DoS + 身份泄露 | high | mitigate | 零身份 label 红线扩到新四 series（metrics.go:15-18 实证）+ assertExpositionShape 21 series 形态锁；per-client 明细一律审计日志 | closed |
| T-13-17 | Information Disclosure | session_start/end 事件携带敏感值 | high | mitigate | emitEvent 既有红线通道 + 事件键集白名单断言（数值/关联键）+ 注入错误文本零出现负断言 | closed |
| T-13-18 | Tampering | healthz session_active 恒 true 语义漂移误读 | low | accept | D-06 语义诚实论证注释入 health.go（per-client 无「会话死亡=服务终结」态；编排探活只看 200/503）；文档面归 13-08/Phase 14 | closed |
| T-13-19 | Elevation of Privilege | WESH_REMOTE_USER 值注入（控制字符/超长值/伪造头进 env） | high | mitigate | 值 = sanitizeRemoteUser 清洗产物（proxy.go:55 既有，下游零二次清洗零绕过）+ 键名白名单固定代码常量 + 清洗形态断言；伪造头值面由 --auth-header 信任闸既有语义承接 | closed |
| T-13-20 | Tampering | shared 模式误注入（D-15 收窄破坏） | medium | mitigate | 空串不出键结构性保证（whitelistEnv 形参零值形态双向断言）+ ValidateOptions shared×SpawnFunc≠nil 拒绝既有 | closed |
| T-13-21 | Tampering | 多客户端 env 串台（共享 startOpts 被闭包改写） | medium | mitigate | 闭包内局部复制后赋值（防串台论证注释 + T-13-21 字面锚定）+ 逐端捕获断言 | closed |
| T-13-22 | DoS | churn 负载下资源无界（RSS/goroutine/fd 泄漏未检出） | high | mitigate | TestChurn 双采样差值上界 + 轮询 + throttled>0 生效证据（实测回落基线）+ phase13.mjs S1 协议层节流全链 | closed |
| T-13-23 | DoS | UAT 自身泄漏（trap 免疫对照进程残留） | medium | mitigate | S2 对照形态 finally kill -9 清理 + 场景尾 pgrep 零命中断言 + 收口后全局复核 | closed |
| T-13-24 | Information Disclosure | UAT 控制台输出泄漏 token/pid | medium | mitigate | emittedDetails 收集 + assertOutputClean 运行时自证（两轮零命中；detail 只打印状态码/布尔/计数/退出码/文案常量） | closed |
| T-13-25 | Tampering | shared 回归未检出（per-client 改动溢出污染 shared 路径） | high | mitigate | 零回归双证据（全量 -race 五包 exit 0 + 既有 15 脚本默认 shared 零修改重跑基线一致）+ diff 白名单 23 文件逐行归类 | closed |
| T-13-26 | Tampering | 断言放宽换绿（「两模式都接受」形态） | high | mitigate | 放宽形态全代码 diff grep 零命中 + 三明示登记项逐字核对 + prohibitions 红线人工确认零违反 | closed |
| T-13-SC | Tampering | 依赖面供应链（8 plan 共有红线） | high | mitigate | 零新依赖红线终验：x/time/rate v0.15.0 为 go.mod 既有直接依赖（go.mod:11 实证）；go.mod/go.sum + web 依赖清单五文件 0 行 diff；全 phase 无装包任务，legitimacy 门未触发 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-13-01 | T-13-03 (low) | warn 文案为编译期定值、零插值（main.go 既有红线注释），不存在运行时敏感值流入面——零新风险面 | gsd-secure-phase (L1 audit) | 2026-09-06 |
| AR-13-02 | T-13-12 (low) | session_end 事件 schema 复用 shared 母本，exit_code/duration/signal/client_id 四键全为数值/关联键，零敏感值经 emitEvent 既有通道 | gsd-secure-phase (L1 audit) | 2026-09-06 |
| AR-13-03 | T-13-18 (low) | per-client 模式无「会话死亡=服务终结」态，session_active 恒 true 为语义诚实形态（D-06 论证注释入 health.go）；编排探活契约只依赖 200/503 status（draining 分支既有）；文档面澄清归 13-08/Phase 14 | gsd-secure-phase (L1 audit) | 2026-09-06 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-06 | 27 | 27 | 0 | gsd-secure-phase (ASVS L1, short-circuit: register_authored_at_plan_time + threats_open:0 + asvs_level 1) |

**Audit method (ASVS L1 — grep-depth verification):**

- 8/8 PLAN 含 `<threat_model>` 块（register_authored_at_plan_time: true）；8/8 SUMMARY 含 Threat Mitigations Applied 证据表
- 实现文件存在性验证：22/22 key-files 全部 FOUND（cmd/wesh、internal/pty、internal/server、web/uat）
- 代码锚点抽查全部命中：
  - T-13-01 → cmd/wesh/main.go:1306（`cfg.stopTimeout = 5 * time.Second` 默认兜底）
  - T-13-04/05 → internal/server/spawnthrottle.go（global `*rate.Limiter` + perIP map 双令牌桶）
  - T-13-09 → internal/server/server.go:206（pcExitReq 第二终结源）、:241（termOnce 单点收口）
  - T-13-16 → internal/server/metrics.go:15-18（零身份 label 红线注释）
  - T-13-19 → internal/server/proxy.go:55（sanitizeRemoteUser 清洗单侧定义）
  - T-13-SC → go.mod:11（x/time v0.15.0 既有直接依赖，零新增）
- 3 项 accepted risks（T-13-03/T-13-12/T-13-18，均 low）已登记于 Accepted Risks Log

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-06
