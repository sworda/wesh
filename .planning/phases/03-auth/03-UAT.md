---
status: complete
phase: 03-auth
source: [03-VERIFICATION.md]
started: 2026-08-17T09:49:00Z
updated: 2026-08-18T08:35:00Z
---

## Current Test

[testing complete]

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
test: `wesh --bind 127.0.0.1 --credential user:pass --writable -- bash` 后浏览器打开 `http://127.0.0.1:7681`（⚠️ 必须带 --writable，否则 ro 模式按 D-14 设计禁输入，标题栏显示 [ro]）
expected: 浏览器弹**原生** Basic 登录框（非页面内自建表单）；输入一次凭据即进入终端且可正常打字；DevTools → Network 可见 `POST /api/attach` 返回 200（同源 fetch 自动携带缓存凭据，A2 假设验证点——若 401 则 D-02 流程断裂需复盘）；**新开标签页**访问不再弹窗（凭据缓存生效。⚠️ 不要用刷新验证：D-11 单次语义 WS 断开即服务端退出，刷新=杀服务，断线重连属 Phase 6）
why_human: 浏览器原生弹窗与 HTTP auth 凭据缓存是浏览器行为，Go/Node 自动化无法驱动
auto_evidence: |
  web/uat/phase03.mjs 场景 1 全绿（2026-08-17 复跑再确认）：401 challenge 头形态、无/错凭据同文、
  200+ticket+no-store、Hello→Welcome(mode=rw)、非法 ticket auth_failed+1008 全链路自动化通过
manual_evidence: |
  2026-08-17 首测报告"无法输入"→ 诊断非缺陷：实例未带 --writable → Welcome mode=ro →
  前端按 D-14 设计 disableStdin + 标题 [ro]（main.ts:202-205）；带 --writable 后 agent 实测
  WS Hello→Welcome(rw)→INPUT 回显全通（echo UATOK 回显 + bash 提示符）。待用户用正确实例复测
diagnosis: 非代码缺陷——ro 是默认安全姿态；复现命令漏 --writable（03-VERIFICATION.md 同源问题，已纳入 G-03-5 文档清扫）
result: pass
manual_confirm: "2026-08-18 用户复测（自带实例 WESH_CREDENTIAL user:1234 + TLS + --writable）：登录进终端可正常操作，确认没问题"
source: automated+manual

### 2. ticket 过期行为（准则 1 服务端半侧，D-10）
test: 已登录页面放置 >60s（ticket TTL 过期）后直接操作终端
expected: 连接**保持存活**、终端正常可用——ticket 仅在 WS Hello 时一次性核销，已建立连接不做中途重验（设计语义，非缺陷）；过期语义由服务端自动化锁定（见 auto_evidence）
why_human: 原期望（静置后见 auth_failed + 静默重试）经诊断不可在浏览器复现——前端每次 connect() 都新取 ticket，永不复用过期 ticket（main.ts:147-153）；auth_failed 静默重试仅覆盖 attach→Hello 间隔超 TTL 的竞态窗口（main.ts:260-265）。已建立连接静置后不断开才是正确行为
auto_evidence: |
  auth_failed 静默重试一次逻辑由 03-05 前端实现（main.ts connect() retriedAuth 守卫）；
  Error{auth_failed}+1008 wire 形态 phase03.mjs S1f 自动化锁定；
  2026-08-17 补充：TTL=60s 真实等待 65s 服务端半侧实测——过期 ticket Hello 收
  Error{auth_failed} + close 1008 reason=auth_failed（X2a-c 全绿，与非法 ticket 同口径无 oracle）；
  Go TestTicketExpiry（SEC-02，TTL=100ms 注入）同口径机器断言
manual_evidence: |
  2026-08-17 用户实测：页面静置 >60s 后连接未断开、终端可操作——与设计语义一致（判定 pass）。
  首测时按原期望误报"连接没断开"，诊断为期望误写而非产品缺陷
diagnosis: 期望误写已修正——静置不断连为正确行为；过期语义由三层自动化（Go 单测/phase03.mjs/65s 实测）锁定
result: pass
source: automated+manual

### 3. 爆破节流感知（准则 2 用户感知面）
test: 凭据弹窗连续输错 3+ 次（或脚本快速连发错凭据），观察页面与 stderr
expected: 页面落 "Too many attempts" 面板（429 语义）；服务端 stderr 事件行只有 remote/code/reason 三要素，**无凭据/base64/ticket 任何形态**（日志红线人工复核）
why_human: 弹窗交互与面板视觉呈现需人工；红线输出复核需人眼
auto_evidence: |
  phase03.mjs 场景 2 自动化：首 401 后续 429+Retry-After；日志红线由 Go 测试
  TestLogRedaction（03-02）机器断言；UAT 输出本身 grep 零凭据/ticket 命中（03-06 已验）；
  2026-08-17 补充：真实实例触发错凭据×3/无凭据/成功/非法 ticket 后 stderr 实测 grep——
  密码明文/错误密码/凭据 base64/Authorization/Basic/ticket 七模式零命中（X3a），
  事件行 remote/code/reason 三要素形态确认（X3b）；
  testssl.sh 扫描期间未认证连发请求实测撞 429（节流真实生效的外部侧证）
manual_evidence: "2026-08-18 用户复测确认无问题（429 节流面板链路含于复测通过范围）"
result: pass
source: automated+manual

### 4. Origin 拒绝（准则 3，SEC-04）
test: `curl -i -H "Origin: https://evil.example" -X POST http://127.0.0.1:7681/api/attach`（凭据实例），或对 /ws 手构带邪恶 Origin 的升级请求
expected: 403（通用文案，不回显 Origin 值）；不带 Origin 的 curl 正常走 Basic 认证流程（非浏览器放行，D-13）
why_human: 已由自动化完整覆盖，人工抽查确认即可
auto_evidence: |
  phase03.mjs 场景 3 全绿（2026-08-17 复跑再确认）：/api/attach 邪恶 403/白名单 200、
  /ws 邪恶 403/无 Origin 400（越过 Origin 闸）；
  2026-08-17 补充：按 UAT 原文 curl 形式对真实实例抽查——邪恶 Origin 403 且响应体
  不回显 Origin 值（X4a）、不带 Origin 的 curl 携凭据 200 正常走 Basic 流程（X4b，D-13）
manual_evidence: 2026-08-17 由 agent 按 UAT 原文形式完成抽查（why_human 注明"人工抽查确认即可"）
result: pass
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
  HSTS max-age=63072000/nosniff/CSP frame-ancestors 三头断言自动化通过；
  2026-08-17 补充：testssl.sh docker 快速组对真实 TLS 实例实测——协议 SSLv2/SSLv3/
  TLS1.0/1.1 全 not offered，下限 TLS 1.2 + 1.3 final；cipher 仅 FS AEAD offered（CBC/
  NULL/EXPORT/LOW/3DES 全 not offered）；安全头 HSTS 63072000 includeSubDomains +
  XFO DENY + nosniff + CSP + COOP/CORP + Referrer-Policy 齐全（429 响应亦带头）；
  明文实例实测无 HSTS 且 CSP/nosniff 恒在（X5a-c，Pitfall 7 机器复核）
manual_evidence: |
  2026-08-17 首测报告：`--tls-cert cert.pem` 相对路径报错 "open cert.pem: no such file or directory"
  ——直接原因是 CWD 无此文件（环境问题）；但暴露两个真实产品缺陷（已立案 G-03-5）：
  ① "listening on https://" 打印先于证书加载（main.go:192 print vs :202 ServeTLS），坏证书时先打印后死；
  ② ServeTLS 失败路径未回滚已 spawn 的 pty 子进程（main.go:206-209 缺 sess.Close()，对照 :178-181 listen 失败有回滚）。
  待用户用绝对路径证书复测浏览器 https 环节
  2026-08-18 复测通过：WESH_CREDENTIAL + --tls-cert/--tls-key 绝对路径起服，
  浏览器 https 登录进终端操作正常（wss 全链路）。期间 stderr 出现
  "TLS handshake error from 21.36.101.x: unknown certificate / i/o timeout"——
  诊断为外部探测流量（0.0.0.0 暴露面，陌生扫描器/网关探活拒证书/静默超时），
  与用户会话无关，日志仅含 IP 无凭据，不触红线，判定正常背景噪音
result: pass
source: automated+manual

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

附注：5/5 测试通过；G-03-5（minor）为 Test 5 复测期间顺带发现的产品缺陷，不影响测试判定，已诊断待修复。

## Gaps

- gap_id: G-03-5
  truth: "坏证书路径应在打印 listening 之前报错退出，且不泄漏已 spawn 的 pty 子进程"
  status: failed
  reason: "用户报告: --tls-cert cert.pem 相对路径不存在时，先打印 listening on https://[::]:10112 再报 open cert.pem: no such file or directory"
  severity: minor
  test: 5
  root_cause: |
    ① main.go:192 先打印 listening，:202 ServeTLS 才加载证书文件——坏证书时 print-then-die 时序误导；
    ② main.go:206-209 ServeTLS 失败路径直接 return 1，未 sess.Close() 回滚已 spawn 的 pty 子进程
    （对照 :178-181 net.Listen 失败路径有回滚），孤儿 bash 残留
  artifacts:
    - path: "cmd/wesh/main.go"
      issue: "证书加载晚于 listening 打印；ServeTLS 失败缺 pty 回滚"
  missing:
    - "启动早期（pty.Start/print 之前）预检证书可读性（如 tls.LoadX509KeyPair 或 os.Open 探测），失败即在零资源占用阶段报错"
    - "或 ServeTLS 失败路径补 sess.Close() 回滚（与 listen 失败路径对称）"
    - "文档清扫：03-VERIFICATION.md / 03-UAT.md / README 复现命令补 --writable（ro 是默认安全姿态，交互式复测必须显式可写）"
  debug_session: "in-orchestrator 直接诊断（main.go 源码对照 + 用户终端输出实证），未起 debug agent"
