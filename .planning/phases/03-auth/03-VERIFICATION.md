---
phase: 03-auth
verified: 2026-08-17T12:39:36Z
status: passed
score: 7/8 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:

  - test: "wesh --bind 127.0.0.1 --credential user:pass --writable -- bash 后浏览器打开页面"
    expected: "浏览器弹原生 Basic 登录框（非页面内表单）；输入一次即进终端；DevTools 可见 POST /api/attach 200（同源 fetch 自动携带缓存凭据，A2 假设验证点）；刷新不再弹窗"
    why_human: "浏览器原生弹窗与 HTTP auth 凭据缓存是浏览器行为，Go/Node 自动化无法驱动（03-UAT.md Test 1，status: pending-human）"

  - test: "已登录页面放置 >60s（ticket TTL 过期）后直接操作终端或刷新页面"
    expected: "无错误面板直进终端——Hello 携过期 ticket 收 auth_failed 后前端静默重取 ticket 重试一次成功"
    why_human: "60s 真实等待 + 前端重试链路端到端时序需浏览器实测（03-UAT.md Test 2）"

  - test: "凭据弹窗连续输错 3+ 次，观察页面面板与 stderr 输出"
    expected: "页面落 Too many attempts 面板（429 语义）；stderr 事件行只有 remote/code/reason 三要素，无凭据/base64/ticket 任何形态"
    why_human: "弹窗交互与面板视觉呈现需人工；红线输出人眼复核（03-UAT.md Test 3；机器断言 TestLogRedaction 已通过）"

  - test: "docker run --rm -ti drwetter/testssl.sh --protocols --std --server-defaults --header host:port 对 TLS 实例扫描（全量 -U 可选）"
    expected: "testssl.sh 无弱项（协议下限/cipher 清单/安全头）；明文 HTTP 实例响应无 HSTS"
    why_human: "外部扫描工具行为，D-07 裁决不进 CI；Go 自动化矩阵（TestTLSVersionAndCipherFloor：1.1 败/1.2 成/1.3 默认/CBC 败）已覆盖协议与 cipher 下限，testssl.sh 为声明的人工补充验证（03-UAT.md Test 5）"
---

# Phase 03: 认证与传输安全 Verification Report

**Phase Goal:** 认证与 TLS 达到"敢暴露到公网"标准，修复 ttyd 已核实的认证连环错全套
**Verified:** 2026-08-17T12:39:36Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth（ROADMAP 准则原子化） | Status | Evidence |
| --- | --- | --- | --- |
| 1 | 已认证 HTTP POST /api/attach 换取一次性 ticket（单次使用、60s TTL、绑定权限级别），WS Hello 首帧核销 | ✓ VERIFIED | server.go:206 attachHandler（MaxBytesReader 1KiB/mode 绑定/no-store）+ server.go:410 握手段 checkTicket 核销分支；tickets.go redeem 原子查删（查即删行 70）、defaultTicketTTL=60s；TestAttachFlow/TestTicketStore/TestTicketExpiry 全部 PASS（本次复跑）；UAT phase03.mjs S1a–S1e PASS |
| 2 | 重放同一 ticket 被拒绝 | ✓ VERIFIED | 三层组合证明（planner 裁决的单会话结构约束下形态）：① TestTicketStore/redeem_单次使用 重放 false（查即删同一代码路径）PASS；② wire 级 TestTicketInvalid 非法 ticket（=已删除键）Error{auth_failed}+1008 reason 同名 PASS；③ UAT S1f 端到端 PASS |
| 3 | 脚本爆破 100 次错误凭据触发指数退避节流 | ✓ VERIFIED | throttle.go recordFail 位移起级 base<<min(fails-1,5) 封顶 30s；TestThrottleStore 含「爆破_100_次累计等待至少_47min（ROADMAP 准则 2 锚点）」子测试 PASS（2881s ≥ 47min）；HTTP 层 TestThrottleHTTP（429+Retry-After/成功清零/级数重启）PASS；UAT S2a（8 连发：首 401 后续 429+Retry-After）PASS |
| 4 | 凭据比较走 crypto/subtle 常数时间（先哈希等长） | ✓ VERIFIED | auth.go matchCredential：双方 sha256.Sum256 32B 定长摘要入 subtle.ConstantTimeCompare，逐组 \|= 位或累积不短路不 break（erratum 修正形态，行 56–65 走查确认无条件 return/break）；TestCredentialMatch（多组各自命中/跨组错配）PASS。时序测量断言按 plan 声明不可移植，以代码形态走查为证（backstop 项，plan 声明的验证形态即走查——已执行） |
| 5 | 凭据/ticket/Authorization 头任何形态不出现在任何日志（有日志脱敏测试） | ✓ VERIFIED | TestLogRedaction PASS（os.Pipe 捕获完整失败轮 401+429+auth_failed，断言不含 base64(凭据)/明文密码/ticket 值/authorization 大小写不敏感，含 auth_failed/throttled 正向对照防假绿）；logEvent 全部 11 个调用点 grep 走查仅 remote/code/reason 三要素；main.go 启动面 warn/err 文案不含凭据值 |
| 6 | 不在 Origin 允许列表内的 WS 握手被拒绝 | ✓ VERIFIED | server.go Attach 守卫区 ⓪ originAllowed 403（Accept 前）+ AcceptOptions.OriginPatterns 库内二次校验；TestOriginEndpoints 双端点 ≥7 断言 PASS（邪恶源/null 403、同源/白名单/无 Origin 放行）；UAT S3a–S3d PASS |
| 7 | TLS 仅协商 1.2+（默认 1.3），响应含 HSTS/X-Content-Type-Options 等安全头 | ✓ VERIFIED | tls.go MinVersion=TLS12 + 显式 6 AEAD 清单（无 MaxVersion/PreferServerCipherSuites/1.3 cipher 死代码）；TestTLSVersionAndCipherFloor 四例 PASS（1.1 败/1.2 成/零值默认 1.3/CBC-only 败）；headers.go 恒在六项 + tlsOn 分支 HSTS；TestSecurityHeaders 双分支精确值 PASS；UAT S5a（hsts=true nosniff=true）+ S5c wss 全链路 PASS |
| 8 | testssl.sh 无弱项 | ? UNCERTAIN | 外部扫描工具，自动化不可驱动（D-07 裁决不进 CI）；Go 自动化矩阵已锁定协议/cipher 下限（truth 7），testssl.sh 为声明的人工补充验证 → Human Verification #4 |

**Score:** 7/8 truths verified（truth 8 为外部工具人工验证项）

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/proto/proto.go` | ErrAuthFailed + HelloPayload.Ticket | ✓ VERIFIED | 行 49 `ErrAuthFailed = "auth_failed"`（D-10 注释）；行 81 `Ticket string json:"ticket,omitempty"` |
| `internal/server/tickets.go` | ticketStore（issue/redeem） | ✓ VERIFIED | 76 行完整实现；crypto/rand 16B→base64url 22 字符；核销即删；惰性清扫；零 janitor goroutine |
| `internal/server/throttle.go` | throttleStore + retryAfter | ✓ VERIFIED | 108 行；指数退避/封顶/成功清零/15min 惰性过期/retryAfter 只读访问器（03-03 追加） |
| `internal/server/auth.go` | Credential/ParseCredential/matchCredential/basicAuth | ✓ VERIFIED | 常数时间比较三件套 + 429 闸 + 401 同文 challenge + D-08 钩子；r.BasicAuth() stdlib 解析 |
| `internal/server/origin.go` | NormalizeOrigin/originAllowed/originMiddleware | ✓ VERIFIED | 小写 host+剥默认端口+拒 glob；四段语义与库对齐；null Origin 拒绝 |
| `internal/server/headers.go` | securityHeaders 中间件 | ✓ VERIFIED | 恒在六项精确值 + tlsOn 分支 HSTS（max-age=63072000） |
| `internal/server/tls.go` | TLSConfig | ✓ VERIFIED | MinVersion 1.2 + 恰 6 AEAD 套件 |
| `internal/server/server.go` | 集成装配 | ✓ VERIFIED | Options 六新字段、Handler() 重装配（securityHeaders 最外层 + 整站 Basic + /api/attach 守卫链 + 显式 405 fallback）、守卫区 ⓪、checkTicket 核销闸（行 488–506） |
| `cmd/wesh/main.go` | 6 flag + env 兜底 + validateStartup + ServeTLS 分岔 | ✓ VERIFIED | 11 flag 全名无短选项；parse 期校验（cert 成对/ParseCredential/NormalizeOrigin）；八行矩阵纯函数；冒烟实测拒绝 exit=2 逐字文案 |
| `web/src/main.ts` | connect() 认证感知流程 | ✓ VERIFIED | fetch /api/attach 五路分派、Hello{ticket}、auth_failed retriedAuth 单次门闩、ws/wss 分支；无 localStorage/console 泄漏（grep 零命中） |
| `web/dist/index.html` | 重建产物入库 | ✓ VERIFIED | commit 8eb2fd0 入库；含 /api/attach 与 wss:// 检索串（.gz 按 .gitignore 既定策略不入库，03-05 已裁决登记） |
| `web/uat/phase03.mjs` | 六场景协议 UAT | ✓ VERIFIED | 本次复跑 18/18 PASS；输出红线（凭据/ticket 值只作构造材料不入 check detail） |
| `README.md` | 认证/TLS/行为变更文档 | ✓ VERIFIED | 六 flag 表、首屏「默认拒绝裸奔」+ 行为变更醒目段、/api/attach 契约、testssl.sh 命令、HSTS 粘性提示；无 InsecureSkipVerify 类教程（grep 零命中） |
| `.planning/phases/03-auth/03-UAT.md` | 人工验证清单 | ✓ VERIFIED | 五项人工清单文档化，status: pending-human（本报告 Human Verification 节即其汇入点） |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| server.go attachHandler | tickets.go | `s.tickets.issue(mode, now)`（行 216） | ✓ WIRED | UAT S1d 200+22 字符 ticket 实证 |
| server.go 握手段 | tickets.go/throttle.go | `s.checkTicket(ip, h.Ticket)`（行 410）→ allow/redeem/recordFail（行 497–503） | ✓ WIRED | TestTicketInvalid/TestThrottleHelloSharedCounter 实证 |
| server.go Handler() | headers.go/auth.go/origin.go | `securityHeaders(mux, s.tlsOn)` + `basicAuth(wh/attachHandler)` + `originMiddleware`（行 181–198） | ✓ WIRED | TestAttachEndpoint 五组断言 + UAT 实证 |
| main.go | server 组件 | `server.ParseCredential`/`server.NormalizeOrigin`（fs.Func 回调）/`server.TLSConfig()`+`hs.ServeTLS`（行 197–199） | ✓ WIRED | TestStartupMatrix/TestTLSKeyPairError + 冒烟 exit=2 实证 |
| main.ts | /api/attach + proto | `fetch('/api/attach', {method:'POST'})`（行 151）+ Hello ticket 字段（行 235）+ auth_failed 守卫（行 260） | ✓ WIRED | dist 产物含检索串；UAT S1 全链路（浏览器半侧 pending-human） |
| auth.go basicAuth | throttle.go | `th.retryAfter/recordFail/recordSuccess`（行 88–104） | ✓ WIRED | TestThrottleHTTP 429+Retry-After 实证 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| attachHandler | ticket | `s.tickets.issue()` → crypto/rand 实时生成 | ✓ | FLOWING |
| basicAuth 429 | Retry-After | `th.retryAfter(ip)` → 真实 notBefore 余量 | ✓ | FLOWING |
| Welcome mode | mode | `checkTicket` → ticket 绑定值（D-11） | ✓ | FLOWING |
| main.ts ticket | ticket | `fetch /api/attach` → 响应 JSON | ✓ | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| 全量测试 | `go test -count=1 ./...` | 五包全 ok（cmd/proto/pty/server/web） | ✓ PASS |
| ticket 单次使用/TTL/惰性清扫 | `go test -v -run TestTicketStore` | 6 子测试全 PASS | ✓ PASS |
| 退避级数/100 次 ≥47min | `go test -v -run TestThrottleStore` | 8 子测试全 PASS（含 ROADMAP 锚点断言） | ✓ PASS |
| 认证集成 9 测 | `go test -v -run 'TestAttachFlow\|...'` | 9/9 PASS | ✓ PASS |
| TLS 下限矩阵 | `go test -v -run 'TestTLSVersionAndCipherFloor\|TestSecurityHeaders'` | 6 子测试全 PASS | ✓ PASS |
| 启动矩阵 | `go test -v -run 'TestStartupMatrix\|...' ./cmd/wesh/` | 5/5 PASS | ✓ PASS |
| 协议 UAT | `node web/uat/phase03.mjs /tmp/wesh-uat/wesh` | 18/18 断言 PASS | ✓ PASS |
| Phase 2 回归 UAT | `node web/uat/phase02.mjs /tmp/wesh-uat/wesh` | 11/11 PASS | ✓ PASS |
| D-03 拒绝冒烟 | `/tmp/wesh-uat/wesh -- /bin/cat` | exit=2 + 逐字拒绝文案 | ✓ PASS |
| help 六 flag | `/tmp/wesh-uat/wesh --help` | credential/tls-cert/tls-key/no-auth/insecure-http/origin 全在 | ✓ PASS |
| gofmt/vet | GOROOT gofmt -l . / go vet ./... | 输出为空 / exit 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
| ----------- | ------------ | ----------- | ------ | -------- |
| SEC-01 | 03-02/03-03/03-04/03-06 | 凭据时序安全比较（crypto/subtle），凭据不明文出现在任何日志 | ✓ SATISFIED | matchCredential（subtle+32B 等长+不短路）+ TestLogRedaction 运行时捕获断言 + 启动面红线断言（TestStartupMatrix 不含凭据值） |
| SEC-02 | 03-01/03-03/03-05/03-06 | WS 认证采用一次性短时令牌（单次使用、短 TTL、绑定会话与权限级别），替代 ttyd /token 明文下发 | ✓ SATISFIED | ticketStore 原子查删 + 60s TTL + mode 绑定 + /api/attach 签发 + Hello 核销 + 前端 connect() 全流程；UAT S1 全链路 |
| SEC-03 | 03-01/03-03/03-06 | 认证失败节流（指数退避/速率限制），防止暴力破解 | ✓ SATISFIED | throttleStore 1s×2 封顶 30s + 429+Retry-After + D-08 统一计数器（TestThrottleHelloSharedCounter 反证）+ UAT S2 |
| SEC-04 | 03-02/03-03/03-04/03-06 | WS 握手 Origin 允许列表校验，不在列表内拒绝 | ✓ SATISFIED | originAllowed 四段语义 + 守卫区 ⓪ + OriginPatterns 二次校验 + /api/attach originMiddleware；UAT S3 双端点 403 |
| SEC-05 | 03-02/03-04/03-06 | TLS 最低 1.2（默认 1.3），合理 cipher 套件，安全响应头（HSTS/nosniff 等） | ✓ SATISFIED（外部扫描 pending-human） | TLSConfig 声明式下限 + ServeTLS 分岔 + securityHeaders 双分支；Go 矩阵四例 + UAT S5 wss 全链路；testssl.sh 为声明的人工补充（Human Verification #4） |

**Orphaned requirements:** 无。REQUIREMENTS.md 映射 Phase 3 恰为 SEC-01..SEC-05，全部被 plan 认领且全部核实；REQUIREMENTS.md 五行均已勾 [x] 与代码证据一致。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | 无 | — | TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER grep 零命中（9 个 phase 改动文件全扫）；无空实现；无硬编码空数据流 |

### Prohibitions 核查（must_haves.prohibitions 全量走查）

| Prohibition | Status | Evidence |
| ----------- | ------ | -------- |
| ticket 不经 URL query/子协议头 | ✓ 守住 | 唯一通道为 Hello JSON 载荷（main.ts:235；ARCHITECTURE §2.8） |
| ticket 随机源禁 math/rand/凭据派生 | ✓ 守住 | tickets.go:47 crypto/rand.Read 直生 |
| 节流禁 sleep/延迟响应 | ✓ 守住 | basicAuth 失败即拒 429+Retry-After，无 sleep |
| 凭据比较禁明文/原始字符串入 ConstantTimeCompare | ✓ 守住 | 操作数全为 sha256.Sum256 [32]B |
| 禁配 TLS 1.3 cipher/PreferServerCipherSuites | ✓ 守住 | tls.go 无此二项 |
| HSTS 不在明文分支发送 | ✓ 守住 | headers.go tlsOn 分支；TestSecurityHeaders 双分支断言 |
| TLS 测试禁 *.pem fixture/GODEBUG | ✓ 守住 | tls_test.go 测试内自签 |
| 不发废弃头（X-XSS-Protection 等） | ✓ 守住 | headers.go 六项+HSTS 之外零发送 |
| 凭据/ticket/Authorization 禁入日志参数 | ✓ 守住 | logEvent 11 调用点三要素走查 + TestLogRedaction 行为锁 |
| 禁手拆 Authorization base64 | ✓ 守住 | auth.go:96 r.BasicAuth() stdlib |
| 核销失败禁可区分响应 | ✓ 守住 | D-10 统一 Error{auth_failed}+1008；401 无/错凭据 body 逐字节同文（UAT S1c 同文=true） |
| 401/403/429 body 禁回显请求细节 | ✓ 守住 | 通用文案；TestAttachEndpoint c) 断言不含 evil.example |
| validateStartup 纯函数禁副作用 | ✓ 守住 | main.go:127–144 无 listen/spawn/写文件 |
| 新 flag 禁短选项/别名 | ✓ 守住 | 11 flag 全名 |
| 前端禁自建登录表单 | ✓ 守住 | showStatus 三态面板复用，401 引导 reload 触发原生弹窗 |
| auth_failed 重试禁无限循环 | ✓ 守住 | retriedAuth 单次门闩（main.ts:260–264） |
| ticket 禁写 URL/localStorage/console | ✓ 守住 | grep 零命中（仅闭包变量与 Hello 载荷） |
| UAT 输出禁打印凭据/ticket 值 | ✓ 守住 | 本次复跑输出 18 断言 detail 仅状态码/布尔/枚举名 |
| README/UAT 禁 InsecureSkipVerify 类教程 | ✓ 守住 | grep 零命中；自签指引 mkcert/CA 方向 |
| 冒烟/UAT 禁以 --no-auth 掩盖 D-03 | ✓ 守住 | S6a 默认 bind 拒绝路径为显式断言项 |
| 禁止以 sleep 挂起 goroutine 式节流 | ✓ 守住 | 见上「节流禁 sleep」 |

### Human Verification Required

### 1. 浏览器 Basic 弹窗与凭据缓存（A2 假设验证点）

**Test:** `wesh --bind 127.0.0.1 --credential user:pass --writable -- bash` 后浏览器打开页面
**Expected:** 浏览器弹原生 Basic 登录框（非页面内表单）；输入一次即进终端；DevTools 可见 `POST /api/attach` 200（同源 fetch 自动携带缓存凭据）；刷新不再弹窗
**Why human:** 浏览器原生弹窗与 HTTP auth 凭据缓存是浏览器行为，Go/Node 自动化无法驱动。若 401 则 D-02 流程断裂需复盘（03-UAT.md Test 1，auto_evidence：phase03.mjs S1 全绿）

### 2. ticket 过期静默重试（D-10 前端半侧）

**Test:** 已登录页面放置 >60s 后直接操作终端或刷新页面
**Expected:** 无错误面板直进终端（auth_failed → 静默重取 ticket 重试一次成功）
**Why human:** 60s 真实等待 + 前端重试链路端到端时序需浏览器实测（03-UAT.md Test 2）

### 3. 爆破节流用户感知面与红线人眼复核

**Test:** 凭据弹窗连续输错 3+ 次，观察页面与 stderr
**Expected:** 页面落 "Too many attempts" 面板；stderr 事件行只有 remote/code/reason 三要素
**Why human:** 弹窗交互与面板视觉呈现需人工（机器断言 TestLogRedaction/UAT S2a 已通过，03-UAT.md Test 3）

### 4. testssl.sh 外部扫描（ROADMAP 准则 3 尾项）

**Test:** `docker run --rm -ti drwetter/testssl.sh --protocols --std --server-defaults --header host:port` 对 TLS 实例扫描（全量 -U 可选）
**Expected:** 无弱项；明文 HTTP 实例响应无 HSTS
**Why human:** 外部扫描工具行为，D-07 裁决不进 CI。Go 自动化矩阵已锁定协议/cipher 下限（1.1 败/1.2 成/1.3 默认/CBC 败，本次复跑全 PASS）+ UAT S5a 安全头断言，testssl.sh 为声明的人工补充验证（03-UAT.md Test 5）

### Gaps Summary

无 gaps。全部 7 项可自动化 truth 经单元/集成/UAT/冒烟四层证据 VERIFIED；五项需求 SEC-01..SEC-05 全部 SATISFIED；21 条 prohibition 全部守住；反模式零命中；phase02 UAT 回归 11/11 零漂移。唯余 4 项浏览器/外部工具行为需人工确认（03-UAT.md pending-human 清单即其载体）。

---

_Verified: 2026-08-17T12:39:36Z_
_Verifier: Claude (gsd-verifier)_
