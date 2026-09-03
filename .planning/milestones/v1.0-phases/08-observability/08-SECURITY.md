---
phase: 08
slug: observability
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-28
---

# Phase 08 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 客户端 → WS/HTTP（remote/remote_user 值来源） | 用户可控输入经 XFF/auth-header 进事件字段 → stderr → journald 持久化——日志注入信任边界 | 用户可控字符串（XFF 首段/auth-header 值），敏感性：审计完整性 |
| wesh → stderr/journald | 事件流是审计唯一事实源——格式完整性即审计完整性 | JSON 单行事件 + stdout 启动横幅（合流面） |
| Prometheus/curl → /metrics | 行为轮廓观测面（连接数/失败计数）——D-08 认证闸是唯一访问控制 | 聚合指标（零身份 label），敏感性：行为轮廓 |
| 探活器/任意客户端 → /healthz | 免认证公开端点（D-07 唯一窄例外）——响应面必须零敏感信息 | 粗粒度容量四字段，敏感性：无 |
| 运维文档读者 → 部署面 | README 配方是部署行为入口——配方错误直接造成暴露面 | 部署配方文本，敏感性：凭据处理纪律 |
| 测试夹具 → UAT 输出 | UAT 脚本持有 UAT_CREDENTIAL 与一次性 ticket（敏感值） | 凭据值，敏感性：CI 日志泄漏面 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-08-01a | Tampering | 日志注入伪造事件行（C1/NEL 经 XFF/auth-header 注入） | high | mitigate | encoding/json 转义 C0（log.go logEvent 单行原子写）+ C1/DEL 剥离由 sanitizeRemoteUser（proxy.go:55，trust 分支 proxy.go:126/140 换入）闭合 | closed |
| T-08-01b | Information Disclosure | 日志成为凭据库（base64 凭据/ticket/token 进 stderr→journald） | critical | mitigate | SEC-01 红线注释随 logEvent 迁至 log.go 逐字保留；TestAuthFailedNoUsername（events_test.go:603）红线负断言 + UAT assertOutputClean 全文扫描敏感串 | closed |
| T-08-01c | Tampering | 双轨输出致审计检索分裂 | medium | mitigate | D-13 原子迁移单点切换：logEvent 唯一出口（log.go:93）；G-08-2 修复补 README grep '^\{' 防护段对 stdout 横幅合流 | closed |
| T-08-02a | Tampering | 日志注入经 XFF 首段注入 remote 字段 | high | mitigate | D-19：remote() trust 分支过 sanitizeRemoteUser（C0/C1/DEL 剥离 + 截断，proxy.go:126）；TestRemoteSanitize 属性断言锁定 | closed |
| T-08-02b | Information Disclosure | auth_failed 事件含用户名 → journald 持久化凭据库 | critical | mitigate | logEvent 四参签名结构性无用户名通道（D-23，log.go:93）；TestAuthFailedNoUsername 行为锁；UAT 实机 has() 断言 user/username 双 false（2026-08-28） | closed |
| T-08-02c | Tampering | pinger→detach 跨 goroutine 数据竞争 | high | mitigate | hubMu 同锁写读（RESEARCH Pattern 4 形态 b）；TestMetricsSnapshotRace -race 压力回归 + Go -race 五包全绿 | closed |
| T-08-02d | Information Disclosure | client_id 序号泄漏连接计数侧信道 | low | accept | D-20 裁决：进程内序号无隐私面（非 IP），重启归零；关联检索收益大于侧信道 | closed |
| T-08-03a | Information Disclosure | /healthz 成为枚举 oracle | low | accept | D-10 裁决：body 恒为粗粒度容量四字段，无版本无身份；TestHealthz body 键集白名单断言（health_test.go:123） | closed |
| T-08-03b | Tampering | 免认证例外蔓延 | medium | mitigate | /healthz 双注册在认证分支之外唯一一处 + TestHealthz 对照子测（GET / 仍 401）；README 明示义务 | closed |
| T-08-03c | Denial of Service | slow loris 打 /healthz | low | mitigate | 既有 http.Server ReadHeaderTimeout=5s 盒住头读取；响应体 <200B 无写放大面 | closed |
| T-08-03d | Denial of Service | 关停窗口探活翻转被滥用（伪造 draining） | low | accept | draining 只能经 SIGTERM/INT → Shutdown 触发（OS 信号面），无网络可达置位路径 | closed |
| T-08-04a | Information Disclosure | metrics label 携身份 → 隐私泄漏 + 基数爆炸 | high | mitigate | D-02/D-06：全 series 零身份 label；TestMetricsValues prohibition 断言（exposition 全文无 remote 值串） | closed |
| T-08-04b | Information Disclosure | /metrics 行为轮廓被未授权观测 | medium | mitigate | D-08 认证闸跟随（TestMetricsAuth 两态，metrics_test.go:190）；--no-auth 暴露面 README 明示 | closed |
| T-08-04c | Denial of Service | 采集器凭据错误触发自锁（429 打 Prometheus 目标） | medium | mitigate | README 配方粗体明示后果与排查通道（throttled 事件 retry_after）；实机印证（A1：错密码 → target down + 429 自锁 + throttled×7 retry_after 翻倍） | closed |
| T-08-04d | Tampering | exposition 注入（version label 伪造 series） | low | mitigate | escLabel 三字符转义（metrics.go:172）；TestBuildInfo 表驱动锁定 | closed |
| T-08-04e | Denial of Service | 采集锁序错误致 fan-out 冻结（ABBA 死锁） | high | mitigate | snapshotMetrics 单快照 hubMu > outbox.mu（metrics.go:87）；TestMetricsSnapshotRace -race 压力回归 | closed |
| T-08-05a | Information Disclosure | UAT 脚本 detail 泄漏 token/凭据进 CI 日志 | high | mitigate | sensitiveTokens 闭包 + redactArgs + assertOutputClean 运行时自净（phase08.mjs:20-92，phase07 先例沿用） | closed |
| T-08-05b | Information Disclosure | README 配方误导致 /metrics 公开暴露 | medium | mitigate | basic_auth 配方与 --no-auth 暴露面同节明示；D-07 例外范围限定 /healthz 单端点 | closed |
| T-08-05c | Denial of Service | 采集器凭据错误自锁（README 义务） | medium | mitigate | README 配方节粗体明示后果与排查通道（RESEARCH Pitfall 6 兑现） | closed |
| T-08-06-01 | Information Disclosure | phase08-journal.mjs 测试输出泄漏凭据/ticket | low | mitigate | assertOutputClean 红线复用（phase08.mjs 同款）——grep 断言在管道 stdout 面，敏感值不回显 | closed |
| T-08-SC(01) | Tampering | npm/pip/cargo 包安装供应链 | high | mitigate | N/A——零新外部依赖（log/slog stdlib，go.mod 不动），无安装任务面 | closed |
| T-08-SC(02) | Tampering | 包安装供应链 | high | mitigate | N/A——零新依赖（stdlib only，go.mod 不动） | closed |
| T-08-SC(03) | Tampering | 包安装供应链 | high | mitigate | N/A——零新依赖（stdlib only） | closed |
| T-08-SC(04) | Tampering | 包安装供应链 | high | mitigate | N/A——D-01 手写 exposition 排除官方库；go.mod/go.sum 验收逐字节不动 | closed |
| T-08-SC(05) | Tampering | 包安装供应链 | high | mitigate | N/A——UAT 脚本 Node 原生 fetch/WebSocket 零依赖纪律 | closed |
| T-08-06-SC | Tampering | 依赖引入（npm 等） | high | accept | 零新增依赖：README 编辑 + Node stdlib + 系统既有 jq/grep——无供应链面（消除性接受） | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-08-1 | T-08-02d | client_id 进程内序号：无隐私面（非 IP），重启归零；attach/detach 关联检索收益大于连接计数侧信道 | D-20 裁决（planner） | 2026-08-28 |
| AR-08-2 | T-08-03a | /healthz body 恒为粗粒度容量四字段（status/clients/max_clients/session_active），无版本/身份/错误细节；TestHealthz 白名单行为锁兜底 | D-10 裁决（planner） | 2026-08-28 |
| AR-08-3 | T-08-03d | draining 置位仅 OS 信号面（SIGTERM/INT → Shutdown），无网络可达路径，滥用面不存在 | D-11 设计推演（planner） | 2026-08-28 |
| AR-08-4 | T-08-06-SC | 零新增依赖（README 编辑 + Node stdlib + 系统 jq/grep）——供应链面以消除方式接受 | 08-06-PLAN 登记 | 2026-08-28 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-28 | 26 | 26 | 0 | CodeBuddy agent（secure-phase short-circuit：register 计划期已立 × 6 PLAN + ASVS L1 grep 级锚点全实证） |
| 2026-08-28 | 26 | 26 | 0 | CodeBuddy agent（secure-phase 复审：short-circuit 规则命中——register 计划期已立 × 6 PLAN × ASVS L1 × threats_open: 0；L1 grep 级 9/9 锚点复核全命中） |

审计依据：6 份 PLAN 全部含计划期 `<threat_model>` 块（register_authored_at_plan_time: true）；L1 grep 级锚点抽查全部命中——sanitizeRemoteUser（proxy.go:55/126/140）、logEvent 四参签名（log.go:93）、TestAuthFailedNoUsername（events_test.go:603）、TestHealthz（health_test.go:123）、TestMetricsAuth（metrics_test.go:190）、escLabel（metrics.go:172）、snapshotMetrics（metrics.go:87）、sensitiveTokens/assertOutputClean（web/uat/phase08.mjs:20-92）；自动化面已绿（phase08.mjs 21/21 + phase08-journal.mjs 6/6 + Go -race 五包）。

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-28
