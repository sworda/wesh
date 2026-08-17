---
status: pending-human
phase: 03-auth
source: [03-06-PLAN.md]
started: 2026-08-17T09:49:00Z
updated: 2026-08-17T09:49:00Z
---

## Current Test

[awaiting human verification — end-of-phase 人工确认清单]

## 自动化层说明

协议层断言已全自动化（`node web/uat/phase03.mjs <wesh 二进制>`，零依赖 Node ≥22）：

| 场景 | 覆盖 | 对应 ROADMAP 准则 |
|------|------|-------------------|
| 1 scenarioAuthFlow | 401 challenge（WWW-Authenticate Basic realm）→ 无/错凭据同文 401 → 200+ticket+no-store → Hello→Welcome(mode=rw) → 非法 ticket Error{auth_failed}+1008 | 准则 1 |
| 2 scenarioThrottle | 8 次错凭据连发 → 首 401 后续 429+Retry-After | 准则 2 |
| 3 scenarioOrigin | /api/attach 邪恶 Origin 403 / 白名单 200；/ws 邪恶 Origin 403 / 无 Origin 放行 | 准则 3（SEC-04） |
| 4 scenarioNoAuth | 无认证模式 /api/attach 404 探测 + Hello 无 ticket 直连 Welcome(mode=ro) | D-03 放行面 |
| 5 scenarioTLS | wss 全链路 + HSTS(max-age=63072000)/nosniff/CSP frame-ancestors 'none' | 准则 3（SEC-05） |
| 6 scenarioStartupMatrix | 默认 bind 无凭据拒绝启动（D-03 文案）；凭据+明文+非 loopback 拒绝启动（D-05 文案） | 行为变更端到端证据 |

以下五项为自动化覆盖不到的人工确认项（浏览器原生行为、外部扫描工具、视觉/交互感知）。

## Tests

### 1. 凭据弹窗与缓存（准则 1 + A2 验证点）
test: `wesh --bind 127.0.0.1 --credential user:pass -- bash` 后浏览器打开 `http://127.0.0.1:7681`
expected: 浏览器弹**原生** Basic 登录框（非页面内自建表单）；输入一次凭据即进入终端；DevTools → Network 可见 `POST /api/attach` 返回 200（同源 fetch 自动携带缓存凭据，A2 假设验证点——若 401 则 D-02 流程断裂需复盘）；刷新页面不再弹窗（凭据缓存生效）
why_human: 浏览器原生弹窗与 HTTP auth 凭据缓存是浏览器行为，Go/Node 自动化无法驱动
auto_evidence: |
  web/uat/phase03.mjs 场景 1 全绿（2026-08-17）：401 challenge 头形态、无/错凭据同文、
  200+ticket+no-store、Hello→Welcome(mode=rw)、非法 ticket auth_failed+1008 全链路自动化通过
manual_evidence: [待用户确认]
result: pending
source: automated+manual

### 2. ticket 过期静默重试（准则 1 前端半侧，D-10）
test: 已登录页面放置 >60s（ticket TTL 过期）后直接操作终端或刷新页面
expected: 无错误面板直进终端——WS Hello 携过期 ticket 收 auth_failed 后前端静默重取 ticket 重试一次成功（DevTools 可见一次 Error{auth_failed} + 第二次 /api/attach 200）
why_human: 60s 真实等待 + 前端重试链路的端到端时序需浏览器实测
auto_evidence: auth_failed 静默重试一次逻辑由 03-05 前端实现（main.ts connect() retriedAuth 守卫）；Error{auth_failed}+1008 wire 形态 phase03.mjs S1f 自动化锁定
manual_evidence: [待用户确认]
result: pending
source: automated+manual

### 3. 爆破节流感知（准则 2 用户感知面）
test: 凭据弹窗连续输错 3+ 次（或脚本快速连发错凭据），观察页面与 stderr
expected: 页面落 "Too many attempts" 面板（429 语义）；服务端 stderr 事件行只有 remote/code/reason 三要素，**无凭据/base64/ticket 任何形态**（日志红线人工复核）
why_human: 弹窗交互与面板视觉呈现需人工；红线输出复核需人眼
auto_evidence: |
  phase03.mjs 场景 2 自动化：首 401 后续 429+Retry-After；日志红线由 Go 测试
  TestLogRedaction（03-02）机器断言；UAT 输出本身 grep 零凭据/ticket 命中（03-06 已验）
manual_evidence: [待用户确认]
result: pending
source: automated+manual

### 4. Origin 拒绝（准则 3，SEC-04）
test: `curl -i -H "Origin: https://evil.example" -X POST http://127.0.0.1:7681/api/attach`（凭据实例），或对 /ws 手构带邪恶 Origin 的升级请求
expected: 403（通用文案，不回显 Origin 值）；不带 Origin 的 curl 正常走 Basic 认证流程（非浏览器放行，D-13）
why_human: 已由自动化完整覆盖，人工抽查确认即可
auto_evidence: phase03.mjs 场景 3 全绿（2026-08-17）：/api/attach 邪恶 403/白名单 200、/ws 邪恶 403/无 Origin 400（越过 Origin 闸）
manual_evidence: [待用户确认]
result: pending
source: automated+manual

### 5. TLS 与安全头（准则 3，SEC-05）
test: mkcert/自签起 TLS 实例（`--credential user:pass --tls-cert cert.pem --tls-key key.pem`），浏览器 https 访问正常进终端；跑 testssl.sh docker 快速组；另起明文 HTTP 实例核对响应头
expected:
  - 浏览器 https 访问弹 Basic 后进终端，WS 走 wss 正常
  - testssl.sh 快速组无弱项：
    `docker run --rm -ti drwetter/testssl.sh --protocols --std --server-defaults --header <host>:<port>`
    （协议下限 TLS 1.2、cipher 清单 AEAD-only、安全头齐全；全量漏洞扫描可选 `-U`，耗时分钟级）
  - 明文 HTTP 实例响应**无 HSTS** 头（Pitfall 7 人工复核；CSP/nosniff/XFO 恒在）
  - ⚠️ HSTS 粘性风险：max-age=63072000（两年）——测试完改回 HTTP 部署时，访问过 TLS 实例的浏览器会对该 host:port 强制 HTTPS 至过期，需清浏览器 HSTS 缓存（chrome://net-internals/#hsts）或换端口
why_human: testssl.sh 外部依赖重不进 CI（D-07 裁决）；HSTS 粘性与明文分支头差异需真实浏览器/扫描器
auto_evidence: |
  Go 自动化矩阵（03-02 tls_test.go）：TLS 1.1 必败/1.2 必成/1.3 默认/CBC-only 必败/
  安全头双分支（TLS 有 HSTS、明文无）已 -race 全绿；phase03.mjs 场景 5：wss 全链路 +
  HSTS max-age=63072000/nosniff/CSP frame-ancestors 三头断言自动化通过
manual_evidence: [待用户确认]
result: pending
source: automated+manual

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps

[none]
