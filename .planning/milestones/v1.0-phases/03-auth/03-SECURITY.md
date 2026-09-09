---
phase: 03
slug: auth
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-18
---

# Phase 03 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Phase 03 即「认证与传输安全」phase；全部威胁在 PLAN 期建模（7 个 PLAN 含 `<threat_model>`），mitigation 已在实现层 grep-level 验证存在，VERIFICATION=passed，REVIEW.md（23 文件）已完成代码评审。

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 攻击者可控输入 → ticket/节流状态机 | ticket 串（Hello 载荷）与失败事件驱动状态迁移 | ticket（128bit 随机）/ 失败计数 |
| 攻击者可控输入 → 凭据比较/Origin 检查 | Authorization 与 Origin 头完全不可信 | Basic 凭据 / Origin 串 |
| 客户端 ← TLS/安全头 | 传输层强度与浏览器安全策略契约 | TLS 握手 / 安全响应头 |
| 未认证 HTTP → /api/attach 守卫链 | 405/403/429/401 各闸为未认证面核心防线 | 凭据 + body（≤1KiB） |
| 未认证 WS → 握手段核销 | Hello 载荷（含 ticket）不可信 | ticket + 版本协商 |
| 凭据/ticket → stderr 日志 | 日志是泄密面 | logEvent 三要素（remote/code/reason） |
| 部署者输入 → parseArgs/validateStartup | flag/env 半可信；误配需变显式错误 | 配置值（非凭据） |
| 同机其他用户 → 凭据面 | ps 输出与环境文件属本机威胁面 | WESH_CREDENTIAL env |
| 服务端响应 → 前端分派 | /api/attach 响应与 WS Error 帧驱动前端状态机 | ticket / auth_failed |
| 文档 → 部署者 | README/03-UAT.md 是部署安全认知唯一来源 | 逃生门语义、HSTS 粘性、凭据推荐形态 |
| UAT 脚本 → 被测二进制 | UAT 输出不得成为凭据/ticket 泄漏面 | CI 日志 / 终端回滚缓冲 |
| 本地证书/私钥 → 进程（启动面） | 启动期读取 operator 密钥材料 | 证书路径 / TLS 错误文案 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-03-01 | Spoofing | ticket 枚举/预测 | high | mitigate | crypto/rand 16B → base64.RawURLEncoding（`internal/server/tickets.go:42`） | closed |
| T-03-02 | Spoofing | ticket 重放 | high | mitigate | redeem 原子查删（`tickets.go:70`）+ 60s TTL | closed |
| T-03-03 | Elevation of Privilege | 凭据在线爆破 | high | mitigate | per-IP 指数退避，cap 30s（`throttle.go:13`）；D-08 双失败点统一计数 | closed |
| T-03-04 | DoS | ticket/节流 map 单调增长 | medium | mitigate | 核销即删 + 惰性清扫 + 15min 过期 | closed |
| T-03-05 | Information Disclosure | 核销结果 oracle | medium | mitigate | D-10 同归 false + 窗口内不延长 notBefore | closed |
| T-03-06 | Information Disclosure | 凭据时序侧信道 | high | mitigate | SHA-256 等长化 + `subtle.ConstantTimeCompare` + `\|=` 不短路（`auth.go:5,26`） | closed |
| T-03-07 | Elevation of Privilege | CSWSH 跨站 WS 劫持 | high | mitigate | `originAllowed` 四段检查（`origin.go:64`）+ null Origin 拒绝 | closed |
| T-03-08 | Tampering | Origin 白名单绕过 | high | mitigate | `NormalizeOrigin` 剥默认端口 + 拒 glob + 小写（`origin.go:28`） | closed |
| T-03-09 | Tampering | TLS 降级（1.0/1.1/CBC） | high | mitigate | `MinVersion=TLS12` + 6 AEAD 套件显式清单（`tls.go:21-22`） | closed |
| T-03-10 | Tampering | Clickjacking | medium | mitigate | CSP `frame-ancestors 'none'` + `X-Frame-Options: DENY` 双发（`headers.go:33-34`） | closed |
| T-03-11 | Information Disclosure | HSTS 误发明文分支 | low | mitigate | tlsOn 分支参数；双分支断言 | closed |
| T-03-12 | Elevation of Privilege | /api/attach 凭据爆破 | high | mitigate | basicAuth 内 429 闸 + 常数时间比较 + 401 同文 | closed |
| T-03-13 | Spoofing | 非法/过期 ticket 建连 | high | mitigate | 握手段核销闸统一 auth_failed+1008 | closed |
| T-03-14 | Information Disclosure | 凭据/ticket 经日志外泄 | high | mitigate | `logEvent` 三要素唯一出口（`server.go:591`）+ TestLogRedaction | closed |
| T-03-15 | Information Disclosure | 核销/认证结果 oracle | medium | mitigate | D-10 统一口径 + 401 同文 + 节流命中不核销 | closed |
| T-03-16 | Elevation of Privilege | CSWSH 经 /ws 或 /api/attach | high | mitigate | ⓪ 守卫 + OriginPatterns 双执行 + 中间件同款 | closed |
| T-03-17 | DoS | /api/attach 大 body | medium | mitigate | `MaxBytesReader` 1KiB → 413（`server.go:207`）+ ReadHeaderTimeout 5s | closed |
| T-03-18 | Elevation of Privilege | 节流计数器误伤合法用户（shared-IP） | low | accept | per-IP 语义下 NAT 共担已知限制；封顶 30s 无锁定状态机；X-Forwarded-For 属 Phase 7 | closed |
| T-03-19 | Elevation of Privilege | 无认证裸奔监听公网 | high | mitigate | D-03 非 loopback 无凭据拒绝启动 + `--no-auth` 显式逃生门（`main.go:66`） | closed |
| T-03-20 | Information Disclosure | 凭据经明文 HTTP 跨网 | high | mitigate | D-05 非 loopback 明文+凭据拒绝启动 + `--insecure-http` 逃生门（`main.go:67`） | closed |
| T-03-21 | Information Disclosure | 凭据经 ps 泄露 | medium | mitigate | WESH_CREDENTIAL env 兜底 + help 提示 + README 推 EnvironmentFile 600 | closed |
| T-03-22 | Information Disclosure | WESH_CREDENTIAL 透传子进程 | high | mitigate | SEC-06 替换式白名单（`pty/spawn.go:51-61`）+ 针对断言（`spawn_test.go:71-107`） | closed |
| T-03-23 | Tampering | TLS 误配降级 | medium | mitigate | 成对校验 parse 期报错 + TLSConfig 声明式下限 | closed |
| T-03-24 | Information Disclosure | ticket 经前端泄漏 | high | mitigate | 仅存闭包变量与 Hello 载荷；无 localStorage/URL 写入 | closed |
| T-03-25 | DoS | auth_failed 无限重试放大节流 | medium | mitigate | `retriedAuth` 单次门闩（`web/src/main.ts:78,260`） | closed |
| T-03-26 | Spoofing | 前端把非 401 异常当成功 | low | mitigate | 仅 `resp.ok` 取 ticket；404 显式直连；其余统一失败面板 | closed |
| T-03-27 | Information Disclosure | 部署者按过时文档裸奔 | high | mitigate | D-03/D-05 行为变更首屏 + 用法节双重明示 + 冒烟场景 6 断言 | closed |
| T-03-28 | Tampering | 文档与 wire 漂移 | medium | mitigate | flag/协议节直译真源 + grep 断言关键串 | closed |
| T-03-29 | Information Disclosure | 自签教程引向关闭证书校验 | medium | mitigate | prohibition 锁定 mkcert/CA 方向；testssl.sh 含 `--header` 组验证 | closed |
| T-03-07-01 | Information Disclosure | 证书预检错误文案 | medium | mitigate | 错误仅经 `fmt %v` 透传 OS/tls 包错误（路径+系统错误），不拼接凭据材料 | closed |
| T-03-07-02 | Denial of Service | serve 失败孤儿 pty 泄漏 | low | mitigate | Serve/ServeTLS 共享错误路径 `_ = sess.Close()` 与 listen 失败对称回滚 | closed |
| T-03-SC | Tampering | npm/pip/cargo installs | high | accept | 零新增依赖（RESEARCH §Package Legitimacy Audit——全 stdlib / 浏览器内建） | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-03-01 | T-03-18 | per-IP 语义下 NAT 共担是已知限制；30s 上限无锁定状态机（D-09 裁决）；X-Forwarded-For 信任属 Phase 7 反向代理工作 | planner (D-09) | 2026-08-15 |
| AR-03-02 | T-03-SC | 全 stdlib + 浏览器内建，零新增三方依赖；供应链攻击面无引入 | RESEARCH §Package Legitimacy Audit | 2026-08-15 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-18 | 32 | 32 | 0 | orchestrator (L1 grep-level + REVIEW.md 23-file audit 已闭合) |

**Audit 方法**：`register_authored_at_plan_time=true` + `asvs_level=1` + `threats_open=0` → 按 short-circuit 规则 L1 grep-level 深度足够，未 spawn auditor 子代理。关键 mitigation 已在实现层逐条 grep 验证（见 Threat Register Mitigation 列文件引用）。REVIEW.md（standard depth，23 文件）已独立确认无新发现。

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-18
