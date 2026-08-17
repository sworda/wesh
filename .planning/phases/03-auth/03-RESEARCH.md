# Phase 3: 认证与传输安全 - Research

**Researched:** 2026-08-17
**Domain:** Go HTTP/WS 服务的认证、节流与传输安全（wesh 单进程自托管工具）
**Confidence:** HIGH（TLS/cipher/Origin/405 等关键结论均经本机 GOROOT 1.26.3 与 coder/websocket v1.8.15 一手源码核实；OWASP 基线经官方 Cheat Sheet 原文抓取）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**凭据形态与认证入口**
- **D-01:** 凭据双通道：`--credential user:pass` 可重复 flag（多组凭据支持按人撤销，常数时间比较逐组轮询）+ `WESH_CREDENTIAL` env 兜底，flag 优先（systemd `EnvironmentFile=` 600 路径，PITFALLS 部署矩阵） — **Reversibility:** one-way — CLI flag/env 名是公开契约（P2 D-15 同纪律），发布后改名破坏脚本与 systemd unit
- **D-02:** 整站 Basic：`/` 与 `/api/attach` 均挂 401 challenge（WWW-Authenticate），浏览器原生弹窗输一次凭据后同源 fetch 自动带缓存凭据；前端零新 UI 组件，仅加"fetch ticket → Hello 携带"逻辑；静态页本身受保护 = 纵深防御（ttyd 同形态）
- **D-03:** 裸跑收口：bind loopback 且无凭据 → 放行免警告；bind 非 loopback（含默认 0.0.0.0）且无凭据 → 拒绝启动，需显式 `--no-auth` 放行。Phase 1 D-05 的临时取舍在此收口 — **Reversibility:** one-way — flag 契约同上；行为变更影响 Phase 1/2 既有用法（`wesh -- bash` 裸跑需 `--no-auth` 或凭据），README 必须明示

**TLS 形态与证书来源**
- **D-04:** 显式证书才开 TLS：`--tls-cert`/`--tls-key` 成对给出才 ServeTLS（只给一个 = 启动报错）；否则明文 HTTP + stderr 醒目警告，bind 非 loopback 时警告升级。反代终止 TLS 部署（wesh 自身明文）是常态场景，零意外行为 — **Reversibility:** one-way — flag 契约同上
- **D-05:** 凭据×明文耦合：loopback + 明文 + 凭据 → 放行免警告（流量不出机）；非 loopback + 明文 + 凭据 → 拒绝启动，需显式 `--insecure-http` 放行（与 D-03 `--no-auth` 同构的显式逃生门） — **Reversibility:** one-way — flag 契约同上
- **D-06:** TLS 强度实现目标：MinVersion 1.2、默认协商 1.3、1.2 弱 cipher 剔除；安全响应头集合交 research 按 OWASP 基线定稿——HSTS 仅 TLS 时发送；单文件全内联现实下 CSP 实用化（script/style 'unsafe-inline' 不可避免，default-src/connect-src 收同源、ws/wss 限同源）；frame-ancestors 'none' 兼发 X-Frame-Options: DENY 兼容老浏览器
- **D-07:** TLS 验证形态：Go 测试自动化断言（crypto/tls client MaxVersion 1.1 必败、1.2 必成、弱 cipher 拒绝）+ testssl.sh 进手动 UAT 清单并文档化命令；不进 CI（外部依赖重、与现有轻量 CI 风格不搭）

**失败节流与核销语义**
- **D-08:** 两处失败统一 per-IP 指数退避计数器：`/api/attach` 凭据失败与 Hello ticket 核销失败计入同一 per-IP 计数器；认证成功清零
- **D-09:** 退避参数（初始值/倍率/封顶/是否临时锁定）交 research 标定（参照 fail2ban 生态与同类工具）；验收锚点 = ROADMAP 准则 2"爆破 100 次触发可观测退避"
- **D-10:** 核销失败统一 `Error{code: auth_failed}` + 1008 关闭（close reason 同名机器串，P2 D-07 纪律）——过期/非法/重放同口径，不给攻击者区分 oracle；前端拿 `auth_failed` 机器码自动重取 ticket 静默重试一次（ticket 过期是正常场景：页面放置超 60s）。`auth_failed` 进 proto Error code 表，兑现 P2 deferred 挂账 — **Reversibility:** costly — Error code 入表即前后端公开契约（P2 D-07 同评级），更名/改义牵动两侧
- **D-11:** ticket 绑权限：`/api/attach` 请求体为空，ticket 内部携带 mode 字段（值 = 全局 `--writable` 模式）；Phase 5 分签发时给请求加可选 mode 参数即向后兼容，本 phase 不提前收参

**Origin 白名单语义**
- **D-12:** `--origin` 可重复 flag，值为完整 origin（scheme://host[:port]），解析规范化后精确比较；不配 flag 时维持库默认同源校验（零配置场景行为不变）；反代场景显式加公网 origin 解 FEATURES 张力 — **Reversibility:** one-way — flag 契约同上
- **D-13:** 无 Origin 头放行（非浏览器客户端：curl/Node e2e/UAT 零摩擦；CSWSH 威胁模型只约束浏览器，浏览器必发 Origin）；有 Origin 必查（同源或在 `--origin` 列表内）；`/ws` 与 `/api/attach` 均查，静态页 GET 不查（无副作用）

### Claude's Discretion
- ticket 存储数据结构与核销原子性（进程内 map + O(1) 查表删除，ARCHITECTURE 已定方向；128bit crypto/rand、60s TTL、单次核销、独立 secret 为 ROADMAP/PITFALLS 锁定项）
- 无认证模式（--no-auth/loopback 裸跑）下 `/api/attach` 的行为与前端探测顺序（如 404 → 前端跳过 fetch 直连 WS）
- `/api/attach` 实现细节：POST-only（其他方法 405）、body 上限、响应 JSON 字段名
- 退避计数器内存界与清理时机（map + 计数 + 过期清理，防单调增长——halfOpenCounter 同款 Pitfall 4 纪律）
- Origin 规范化比较细节（默认端口省略/大小写/尾斜杠）、Basic realm 字符串、401/403 body 文案
- 日志脱敏测试形态（CI grep base64/凭据样例扫描）、节流/认证失败事件 stderr 单行形态（沿用 logEvent 三要素，Phase 8 升级 slog）
- TLS 1.2 cipher 具体清单（Go stdlib 安全集基础上去弱，以 research 为准）

### Deferred Ideas (OUT OF SCOPE)
- **ro/rw 分别签发 ticket 与分享链接** — Phase 5 MULTI-05；本 phase ticket 内部 mode 字段已占位（D-11）
- **permission_denied Error code** — Phase 5 权限场景加入 code 表（P2 deferred 同批挂账）
- **X-Forwarded-For/可信代理解析与 auth-header 透传（SEC-07）** — Phase 7；server.go 现行注释误写"Phase 3 SEC-07"，本 phase 顺手校正注释
- **节流失败计数进 metrics / 审计事件 slog 结构化** — Phase 8（PITFALLS"失败计数进 metrics"挂账）
- **ACME/Let's Encrypt 自动证书** — v2（V2-ACME）
- **flag 可配性收口进配置文件** — Phase 7（OPS-09）
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SEC-01 | 凭据时序安全比较（crypto/subtle），凭据不明文出现在任何日志 | Pattern 2（SHA-256 等长化 + 逐组 & 累积比较，pkg.go.dev 官方长度泄露说明）+ Pattern 8 日志红线测试形态；Go 1.26.3 GOROOT 源码核实 ConstantTimeCompare 长度不等立即返回 0 |
| SEC-02 | WS 认证一次性短时令牌（单次使用、短 TTL、绑权限级别） | Pattern 1（crypto/rand 128bit + base64url + map 原子查删核销 + 60s TTL + mode 绑定）；proto.go HelloPayload 加字段与 Error code 表扩展点已逐字核实 |
| SEC-03 | 认证失败节流（指数退避/速率限制） | Pattern 3（per-IP 指数退避计数器：1s ×2 封顶 30s，OWASP Authentication Cheat Sheet 原文锚点 + fail2ban 生态佐证；halfOpenCounter 同构内存纪律） |
| SEC-04 | WS 握手 Origin 允许列表校验 | Pattern 4（coder/websocket v1.8.15 accept.go:228-264 authenticateOrigin 一手源码核实语义 + flag 解析规范化使 path.Match 退化为精确比较；/api/attach 共享同一 normalize helper） |
| SEC-05 | TLS 最低 1.2（默认 1.3）、合理 cipher、安全响应头 | Pattern 5（GOROOT 1.26.3 cipher_suites.go:283-312 一手核实默认列表含 4 个 CBC-SHA1 套件 → 必须显式 6 AEAD 清单）+ Pattern 6（OWASP HTTP Headers Cheat Sheet 官方推荐值定稿） |
</phase_requirements>

## Summary

本 phase 的全部能力都可用 **Go 标准库 + 既有依赖（coder/websocket v1.8.15）** 落地，**零新增外部包**。这是本次研究最重要的结构性结论：ticket（crypto/rand）、常数时间比较（crypto/subtle）、TLS 加固（crypto/tls）、405 POST-only（net/http ServeMux 方法模式，Go 1.22+）、Basic 解析（r.BasicAuth()）、Origin 同源校验（库内建 AcceptOptions.OriginPatterns）全部是 stdlib/现依赖能力，前端只需 `fetch` + Hello 加字段，无新 npm 包。

三个一手源码核实的关键事实，直接决定实现形态：
1. **Go 1.26.3 默认 TLS 1.2 cipher 列表仍含 4 个 ECDHE CBC-SHA1 套件**（cipher_suites.go:283-312 `cipherSuitesPreferenceOrder` 含 "CBC w/ ECDHE" 段；defaults.go:69-77 `defaultCipherSuites` 只删 RC4/CBC_SHA256/RSA kex/3DES）——D-06"1.2 弱 cipher 剔除"与 ROADMAP 准则 3"testssl.sh 无弱项"**必须显式给出 6 个 AEAD 套件清单**，靠零值默认过不了关。
2. **coder/websocket 的 Origin 校验语义与 D-13 完全同构**（accept.go:228-260：空 Origin 放行 → `EqualFold(r.Host, u.Host)` 同源放行 → OriginPatterns 匹配）——`/ws` 直接复用库，只需 flag 解析时规范化（去默认端口/小写/禁 glob 字符）使 `path.Match` 退化为精确比较；`/api/attach` 用共享 normalize helper 自建同款检查。
3. **`subtle.ConstantTimeCompare` 长度不等立即返回 0**（pkg.go.dev 官方明示）——凭据必须先 SHA-256 等长化再比较；多组凭据逐组用位与 `&` 累积不短路，防组序号时序泄露。

退避参数标定（D-09 交 research）：**初始 1s、倍率 ×2、封顶 30s、不做临时锁定**。OWASP Authentication Cheat Sheet 原文锚定"指数退避从约 1 秒起每次失败翻倍"；fail2ban 生态默认 bantime 600s/findtime 10m/maxretry 5 佐证量级。该参数下爆破 100 次累计等待 ≥47 分钟（可观测退避锚点达成），持续爆破稳态限速 2 次/分钟/IP。OWASP 的"按账户计数"建议不适用于 wesh 共享凭据模型（会被用来 DoS 全体用户），D-08 per-IP 统一计数器是正确取舍。

**Primary recommendation:** 全部 stdlib 实现；TLS 显式 6 AEAD cipher 清单 + MinVersion 1.2；ticket = crypto/rand 128bit + map 原子查删；节流 = per-IP 1s×2 封顶 30s 计数器；Origin 复用库语义 + flag 规范化；安全头按 Pattern 6 定稿中间件；测试全部落在 Go `-race` 单测/e2e + Node UAT 脚本，testssl.sh 走 docker 手动 UAT。

## Project Constraints (from CODEBUDDY.md)

| 指令 | 对本 phase 的影响 |
|------|------------------|
| 使用 pnpm 而非 npm | 前端构建命令 `pnpm -C web build`（CI 已固化 pnpm 11.21.0） |
| Bash 工具使用 fish shell | 验证命令注意 fish 语法（无 `$(...)` 兼容问题，本研究命令已实测） |
| 构建命令使用 `time` 前缀 | 全量验证时 `time go test -race -count=1 ./...` |
| 不接受过度设计或不必要的新类型/枚举 | 本研究所有模式均为最小实现：ticket/throttle 各一个 struct + map，不引入新框架、不设新接口抽象 |
| 指针获取后判空 `if (Type* ptr = GetXxx())` 风格 | C++ 约定，Go 不适用；对应纪律：error 判空后立即处理，不逃逸作用域 |
| 沟通与文档使用中文、简洁直接 | RESEARCH.md 即中文；代码注释延续现有中文风格 |
| P4 版本控制规则 | **不适用**：wesh 是 git 项目（GitHub PR 流程，`.github/workflows/ci.yml` 已核实） |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 凭据校验（Basic、常数时间比较） | API / Backend（wesh 进程） | — | 单进程工具，HTTP 层中间件即唯一正确位置；浏览器只负责弹窗与缓存凭据 |
| ticket 签发/核销/存储 | API / Backend | — | 进程内 map，随进程生灭；Arch §2.2 Auth Service 职责表已定内存票表 |
| 失败节流计数器 | API / Backend | — | per-IP 计数必须在服务端；反代层限流（nginx limit_req）是部署可选增强非替代 |
| Origin 校验 | API / Backend | Browser（自动发 Origin 头） | CSWSH 防线在服务端（库 Accept + /api/attach 中间件双挂点）；浏览器只负责必发 Origin |
| TLS 终止与 cipher/版本下限 | API / Backend（`--tls-cert` 直跑） | CDN / 反代（D-04 明文的常态部署形态） | wesh 自身 TLS 由 crypto/tls tls.Config 声明式达成；反代终止时 wesh 明文属预期（D-04/D-05 矩阵约束） |
| 安全响应头 | API / Backend | — | 中间件统一设置；HSTS 仅 TLS 分支（D-06）；反代可能再叠加，不冲突 |
| ticket 获取与 Hello 携带、auth_failed 重试 | Browser / Client | — | 前端唯一新逻辑：fetch POST /api/attach → Hello{ticket} → onclose 按 code 重试一次（CONTEXT.md L75） |
| 启动校验矩阵（凭据×bind×TLS） | API / Backend（main 组合根） | — | parseArgs/run 层拒绝或警示，http.Server 分岔 ServeTLS/Serve（main.go:86-96 现状已核实） |

## Standard Stack

### Core（全部 Go stdlib + 现依赖，零新增）

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| crypto/rand | go1.26.3 [VERIFIED: go.mod:3 + 本机 toolchain] | ticket 128bit 随机源 | 密码学安全随机的唯一 stdlib 入口；ARCHITECTURE §2.2 锁定 128bit |
| crypto/sha256 + crypto/subtle | 同上 | 凭据等长化 + 常数时间比较 | PITFALLS C6 对策官方原语；subtle 长度泄露语义经 pkg.go.dev 核实 [VERIFIED: pkg.go.dev/crypto/subtle] |
| crypto/tls | 同上 | TLS 1.2 下限 + 显式 cipher 清单 | MinVersion/CipherSuites 声明式配置；默认行为经 GOROOT 源码核实（见 Pattern 5）[VERIFIED: GOROOT/src/crypto/tls] |
| net/http（ServeMux 方法模式） | Go 1.22+ | `POST /api/attach` 405 自动拒绝 | 方法不匹配自动 405 + `Allow` 头（server.go:2699-2710 已核实）[VERIFIED: GOROOT/src/net/http/server.go:2699-2710] |
| net/http `r.BasicAuth()` | stdlib | Authorization: Basic 解析 | 官方解析器，不手拆 base64 [VERIFIED: pkg.go.dev/net/http#Request.BasicAuth] |
| encoding/base64 | stdlib | ticket base64url 编码 | RawURLEncoding，16 字节 → 22 字符 |
| github.com/coder/websocket | v1.8.15（现依赖） | WS Accept 的 Origin 校验 | `AcceptOptions.OriginPatterns` 语义经 accept.go:228-264 一手核实 [VERIFIED: GOMODCACHE accept.go] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| crypto/x509 + crypto/ecdsa | go1.26.3 | 测试内生成自签证书 | TLS 测试夹具（stdlib 模式，无外部 fixture 文件） |
| testssl.sh（docker `drwetter/testssl.sh`） | 3.2.x [CITED: testssl.sh 官网] | TLS 审计（协议/cipher/漏洞/安全头） | 手动 UAT 专用，不进 CI（D-07）；本机无二进制，docker 24.0.6 可用 |
| web/uat/phase03.mjs（Node 原生 WS/fetch） | Node ≥22（本机 v24.13.0） | 协议层自动化 UAT | 沿用 phase02.mjs 零依赖模式（web/uat/phase02.mjs:1-8 已核实） |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 自建 per-IP 指数退避计数器 | `golang.org/x/time/rate` 令牌桶 | rate 是平滑限速语义（速率+突发），D-08 要的是"失败次数 → 指数延迟 + 成功清零"的计数器语义；为不匹配的原语新增外部依赖不值（x/time 引入即 go.mod 新行） |
| ServeMux 方法模式 405 | 手写 method 检查 | 手写 3 行也行，但 mux 自带 `Allow` 头与 405 文案，零代码且语义标准 |
| 库 OriginPatterns | 全自建 Origin 检查 + InsecureSkipVerify | 自建失去库同源校验的纵深防御双保险；且 D-12 要求"不配 flag 维持库默认"——复用库即零行为漂移 |
| Go http.Server ServeTLS | Caddy/反代全权终止 TLS | D-04 已裁决：显式证书才 ServeTLS，反代场景 wesh 明文是常态；两者并存由启动矩阵控制 |

**Installation:** 无（零新增 Go/npm 依赖；go.mod 与 web/package.json 不动）。

**Version verification:** go.mod:3 钉 `go 1.26.3`，本机 toolchain `go1.26.3 linux/amd64` 一致 [VERIFIED: `go version` 实测]；coder/websocket v1.8.15 见 go.mod:6 [VERIFIED: go.mod 本次会话 Read]。所有 stdlib API（ServeMux 方法模式、tls.Config、subtle、BasicAuth）均在本机 GOROOT 1.26.3 源码内逐处核实存在与语义。

## Package Legitimacy Audit

**本 phase 不安装任何新外部包**（Go 侧全 stdlib；前端零新 npm 依赖——`fetch` 为浏览器内建）。无 package-legitimacy 检查对象。

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| —（无新增） | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

既有依赖不变：coder/websocket v1.8.15、creack/pty v1.1.24、golang.org/x/sys v0.47.0（go.mod:5-9 本次会话 Read 核实）；前端 @xterm/* 三件套不动（web/package.json 本次会话 Read 核实）。

## Architecture Patterns

### System Architecture Diagram

```
浏览器                                    wesh 单进程
───────                                   ──────────────────────────────────────────────
GET / ────────────────────────────────► 安全头中间件（恒在：nosniff/CSP/frame-ancestors/...）
                                            │
                                        Basic 中间件（仅 / 与 /api/attach，D-02）
                                            │ 无/错凭据 → 401 + WWW-Authenticate（浏览器弹窗）
                                            │ 节流中 → 429 + Retry-After
                                            ▼
                                        静态页（embed）        POST /api/attach 守卫链：
fetch POST /api/attach ─────────────────►   ① ServeMux 方法模式 405 → ② Origin 检查 403
（同源自动带 Basic 缓存凭据）                 → ③ 节流闸 429 → ④ Basic 常数时间比较 401
                                            → ⑤ 签发 ticket（crypto/rand 128bit，60s TTL，
                                              mode=全局 writable，Cache-Control: no-store）
                                            │ {"ticket":"..."}
new WebSocket(wss://host/ws) ───────────► /ws Attach 守卫区（Accept 前零 WS 资源）：
  │                                         ⓪ Origin 403 → ① 子协议 400 → ② per-IP 429 → ③ 409
  │                                         → Accept（库内 OriginPatterns 二次校验，纵深防御）
  ├─ Hello{version,ticket,cols,rows} ───► 握手段：malformed/version_mismatch 既有桶
  │                                         → 【新】节流闸（notBefore 内 → auth_failed）
  │                                         → 【新】ticket 核销（map 原子查删；过期/非法/重放
  │                                            统一 Error{auth_failed}+1008，D-10 无 oracle）
  │                                         → 既有升档序列（Resize→Welcome→16KiB→pinger）
  ◄── Welcome{mode} ────────────────────────┤
  │  ... OUTPUT/INPUT 数据面（Phase 2 现状不动）
  │
  └─ auth_failed onclose → 自动重 fetch ticket 静默重试一次（仅一次，失败才展示人话）

TLS 分岔（main run）：
  --tls-cert + --tls-key 成对 → http.Server{TLSConfig: MinVersion 1.2 + 6 AEAD 清单}.ServeTLS
                              （HSTS 仅此分支发送）
  否则 → Serve（明文）+ stderr 醒目警告；启动校验矩阵（D-03/D-05）先行拒绝非法组合

失败节流计数器（per-IP，D-08 统一）：
  /api/attach 凭据失败 ─┐
                        ├─→ throttle.recordFail(ip)：fails++，notBefore=now+min(30s, 1s<<fails)
  Hello 核销失败 ───────┘    认证成功 → delete(ip)；15min 未活动惰性过期
```

### Recommended Project Structure（在现状上增量，不新建包）

```
cmd/wesh/main.go          # +parseArgs 6 个新 flag、启动校验矩阵、ServeTLS 分岔、警示打印
internal/proto/proto.go   # +HelloPayload.Ticket、+ErrAuthFailed 常量（Error code 表）
internal/server/
├── server.go             # +Attach 守卫区 ⓪ Origin、握手段 ticket 核销、Options 新字段
├── auth.go               # 【新】凭据类型（启动时预哈希）+ Basic 中间件 + 常数时间比较
├── tickets.go            # 【新】ticketStore（签发/核销/TTL/惰性清理）
├── throttle.go           # 【新】throttleStore（per-IP 指数退避计数器）
├── origin.go             # 【新】--origin 解析规范化 + originAllowed(r) 共享 helper
└── headers.go            # 【新】安全响应头中间件（TLS 分支感知 HSTS）
web/src/main.ts           # 连接流程改造：fetch ticket → Hello{ticket} → auth_failed 重试一次
```

结构理由：auth/tickets/throttle/headers 各自独立小文件对应一条 ROADMAP 准则的测试文件（auth_test/throttle_test/origin_test/tls_test），安全审计单文件可读——延续 Arch §3"预认证资源上限收拢一处"的审计友好传统。不新建 `internal/auth` 子包：四个文件总量 ~400 行，单包内分文件已足够（CODEBUDDY 不过度设计纪律）。

### Pattern 1: 一次性 ticket 签发与核销

**What:** `/api/attach` 认证成功后签发 128bit 随机 ticket（60s TTL、绑定 mode、单次使用）；Hello 首帧核销。
**When to use:** 浏览器 WS 无法设 Authorization 头（平台约束），认证必须并入握手（ARCHITECTURE §2.8 Pattern 4）。
**Why this shape:** map 原子"查+删"即单次使用语义；128bit 随机使在线枚举无意义（2^128 空间），map 查表无凭据比较的前缀时序问题——subtle 纪律只用于静态凭据，ticket 查表是行业标准做法 [ASSUMED：基于通行实践，无官方文档逐字背书；风险极低]。

```go
// ticketStore 一次性 ticket 表：签发=随机+登记，核销=原子查删（单次使用）。
type ticketStore struct {
	mu sync.Mutex
	m  map[string]ticketEntry // key = base64url(16B random)
	ttl time.Duration          // 默认 60s（ROADMAP 锁定）；Options 可覆写供测试（HelloTimeout 先例）
}
type ticketEntry struct {
	mode string    // proto.ModeRO/ModeRW，签发时 = 全局 --writable（D-11）
	exp  time.Time
}

func (ts *ticketStore) issue(mode string, now time.Time) string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand 失败即进程级问题，沿用 Go 惯例可读性处理
	t := base64.RawURLEncoding.EncodeToString(b[:]) // 16B → 22 字符
	ts.mu.Lock()
	defer ts.mu.Unlock()
	// 惰性清理：签发顺手机会性清扫过期项（Pitfall 4 纪律——无界增长防线）
	for k, e := range ts.m {
		if now.After(e.exp) {
			delete(ts.m, k)
		}
	}
	ts.m[t] = ticketEntry{mode: mode, exp: now.Add(ts.ttl)}
	return t
}

// redeem 原子查删：查即删保证单次使用；过期按不存在处理（D-10 同口径无 oracle）。
func (ts *ticketStore) redeem(t string, now time.Time) (mode string, ok bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	e, ok := ts.m[t]
	if !ok {
		return "", false
	}
	delete(ts.m, t) // 无论成败先删——重放/过期/非法同归 false
	if now.After(e.exp) {
		return "", false
	}
	return e.mode, true
}
```

关键决策：
- ticket **不走 URL query、不塞子协议头**（ARCHITECTURE §2.8 锁定：query 进访问日志/浏览器历史）——Hello JSON 载荷携带。
- ticket 与静态凭据是**独立 secret**：crypto/rand 直接生成，不从凭据派生（PITFALLS C6"令牌独立 secret"）。
- TTL 60s 到点**惰性**清理（签发时清扫 + redeem 时判断），不开常驻 janitor goroutine——延续"零新 exitf 分支/goroutine 随 handler 生灭"纪律。
- 无认证模式下 `/api/attach` **不注册路由 → 404**；前端探测到 404 跳过 fetch 直连 WS，Hello 不带 ticket 字段（服务端无认证模式跳过核销）。探测顺序：fetch → 200 拿 ticket / 404 无认证 / 401 展示认证失败 / 429 展示节流。

### Pattern 2: 常数时间凭据比较（多组轮询不短路）

**What:** 启动时把每组 `user:pass` 预哈希为 SHA-256 摘要对；请求到来时同样哈希后逐组比较，位与累积。
**Why SHA-256 先哈希:** `subtle.ConstantTimeCompare` 官方文档明示"If the lengths of x and y do not match it returns 0 immediately"——直接比较明文会泄露长度侧信道（C6 对策核心）[VERIFIED: pkg.go.dev/crypto/subtle]。
**Why 不短路:** `&&` 短路会经 CPU 分支与耗时泄露"第几组凭据匹配"；逐组 `&` 累积使耗时与组数恒定正交。

```go
// credential 启动时预哈希（SHA-256 等长化 32B，消除长度侧信道）。
type credential struct {
	userHash [sha256.Size]byte
	passHash [sha256.Size]byte
}

// parseCredential 切分首个 ':'（密码可含 ':'；user 不可含 ':'，RFC 7617 user-id 约束）。
func parseCredential(s string) (credential, error) {
	u, p, ok := strings.Cut(s, ":")
	if !ok || u == "" {
		return credential{}, fmt.Errorf("credential must be user:pass")
	}
	return credential{userHash: sha256.Sum256([]byte(u)), passHash: sha256.Sum256([]byte(p))}, nil
}

// match 逐组轮询、位与累积——不短路（防组序号时序泄露）；无凭据模式不进此函数。
func matchCredential(creds []credential, user, pass string) bool {
	uh := sha256.Sum256([]byte(user))
	ph := sha256.Sum256([]byte(pass))
	matched := 0
	for _, c := range creds {
		matched &= subtle.ConstantTimeCompare(uh[:], c.userHash[:]) &
			subtle.ConstantTimeCompare(ph[:], c.passHash[:])
	}
	return matched == 1
}
```

Basic 中间件（挂 `/` 与 `/api/attach`，D-02；`/ws` 不挂——ticket 即其认证）：

```go
func basicAuth(next http.Handler, creds []credential, th *throttleStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || !matchCredential(creds, u, p) {
			th.recordFail(clientIP(r)) // D-08 统一计数器
			w.Header().Set("WWW-Authenticate", `Basic realm="wesh", charset="UTF-8"`) // RFC 7617
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		th.recordSuccess(clientIP(r)) // 认证成功清零（D-08）
		next.ServeHTTP(w, r)
	})
}
```

注意：401 body 对"无凭据/错用户/错密码"**完全同文**（OWASP Authentication Cheat Sheet 通用错误消息纪律；状态码也恒 401）[CITED: cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html]。

### Pattern 3: per-IP 指数退避节流计数器（D-09 标定）

**What:** 两处失败（/api/attach 凭据失败、Hello ticket 核销失败）计入同一 per-IP 计数器；失败 → 下次允许时刻指数后移；成功清零。
**参数标定（研究结论）:**

| 参数 | 取值 | 依据 |
|------|------|------|
| 初始延迟 | 1s | OWASP Authentication Cheat Sheet 原文："exponential lockout, where the lockout duration starts as a very short period (e.g., one second), but doubles after each failed login attempt" [CITED: OWASP Authentication Cheat Sheet] |
| 倍率 | ×2 | 同上 OWASP 原文（doubles） |
| 封顶 | 30s | fail2ban 生态默认 bantime 600s/findtime 10m/maxretry 5 佐证十分钟级合理 [CITED: fail2ban 文档惯例]；30s 使 100 次爆破累计 ≥47min（验收锚点"可观测退避"充分达成），且合法用户连续手滑 6 次后最坏等 30s 可恢复 |
| 临时锁定 | 不做 | 个人工具单/少用户场景，锁定状态机增加攻击面（锁定期 vs 正常期的可区分 oracle）与实现复杂度；封顶退避已达稳态 2 次/分钟/IP 的限速效果 |
| 窗口期条目过期 | 15min 无活动惰性过期 | 防 map 单调增长（halfOpenCounter 同款 Pitfall 4 纪律）；15min > 30s 封顶一个数量级以上，不误清活跃退避 |

```go
// throttleStore per-IP 失败计数与退避闸（D-08 两处失败统一计数）。
type throttleStore struct {
	mu   sync.Mutex
	m    map[string]throttleEntry
	base time.Duration // 默认 1s；Options 可覆写供测试（毫秒级提速，HelloTimeout 先例）
	cap  time.Duration // 默认 30s
}
type throttleEntry struct {
	fails     int
	notBefore time.Time // 该时刻前一律拒绝
	lastSeen  time.Time // 惰性过期依据（>15min 未活动删除）
}

// allow 返回 false 表示该 IP 处于退避窗口（调用方 429/auth_failed 同口径拒绝）；
// 窗口内命中不延长 notBefore（恢复期可预期；稳态限速已达）。
func (t *throttleStore) allow(ip string, now time.Time) bool { ... }

func (t *throttleStore) recordFail(ip string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.m[ip]
	if now.Sub(e.lastSeen) > 15*time.Minute {
		e = throttleEntry{} // 惰性过期重置（map 上界纪律）
	}
	e.fails++
	d := t.base << min(e.fails-1, 5) // 1s,2s,4s,8s,16s,30s(cap)…——位移即 ×2 幂
	if d > t.cap {
		d = t.cap
	}
	e.notBefore = now.Add(d)
	e.lastSeen = now
	t.m[ip] = e
}

func (t *throttleStore) recordSuccess(ip string) { // D-08 成功清零
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, ip)
}
```

map 上界补充：条目仅 ~56B，4096 个恶意 IP ≈ 230KB，个人工具场景可接受；`recordFail` 内惰性过期已保证非活跃条目被回收。不设硬上限删除策略（4096 不同源 IP 的协同爆破已超出威胁模型；若 planner 认为需要，可在 recordFail 内 len>4096 时全表扫过期——**不建议**加随机驱逐，复杂度换不到安全收益）。

Hello 侧接入点（Attach 握手段，version 检查之后、升档之前）：

```go
} else if !s.throttle.allow(ip, time.Now()) || !s.redeemTicket(h.Ticket) {
	// D-10 统一口径：节流中/过期/非法/重放 → 同 Error{auth_failed}+1008，无 oracle。
	// 节流命中不再 recordFail（不延长窗口）；核销失败 recordFail（D-08 统一计数器）。
	_ = c.Write(ctx, websocket.MessageBinary, proto.ErrorFrame(proto.ErrAuthFailed, "authentication failed"))
	s.logEvent(remote, websocket.StatusPolicyViolation, proto.ErrAuthFailed)
	_ = c.Close(websocket.StatusPolicyViolation, proto.ErrAuthFailed)
}
```

### Pattern 4: Origin 规范化 + 双端点执行（D-12/D-13）

**库语义一手核实**（coder/websocket v1.8.15 accept.go:228-264，本次会话 Read）：

```go
func authenticateOrigin(r *http.Request, originHosts []string) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil                    // ① 无 Origin 放行 —— 与 D-13 完全一致
	}
	u, err := url.Parse(origin)
	...
	if strings.EqualFold(r.Host, u.Host) {
		return nil                    // ② 同源放行（Host 与 Origin host 大小写不敏感比较）
	}
	for _, hostPattern := range originHosts {
		target := u.Host
		if strings.Contains(hostPattern, "://") {
			target = u.Scheme + "://" + u.Host  // ③ pattern 含 :// 时匹配 scheme://host
		}
		matched, err := match(hostPattern, target) // path.Match（大小写不敏感，glob 语义）
		...
	}
	...
}
```

**结论：库默认行为 = D-13 要求的行为**（无 Origin 放行/同源放行/列表放行）。两个缺口由 flag 解析规范化补齐：

1. **默认端口不对称**：浏览器对默认端口的 Origin 头省略端口（`https://foo.com` 而非 `https://foo.com:443`，RFC 6454 序列化），用户若配置 `--origin https://foo.com:443` 会永不匹配 → 解析时剥离默认端口。
2. **glob 语义**：`path.Match` 把 `*?[\` 当模式字符 → 解析时拒绝含这些字符的 origin（D-12"精确比较"）。

```go
// normalizeOrigin 把 --origin 值规范化为小写 scheme://host[:port]（去默认端口），
// 供 AcceptOptions.OriginPatterns（/ws）与 /api/attach 中间件共用。
func normalizeOrigin(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("origin must be scheme://host[:port]: %q", s)
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("origin must not contain path/query/fragment/userinfo: %q", s)
	}
	if strings.ContainsAny(s, "*?[\\") {
		return "", fmt.Errorf("origin must not contain glob characters: %q", s)
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = "" // 默认端口剥离——浏览器 Origin 序列化省略默认端口（RFC 6454）
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	return u.Scheme + "://" + host, nil
}
```

双端点执行：
- `/ws`：`AcceptOptions{Subprotocols: ..., OriginPatterns: normalized}`——库在 Accept 内拒绝（403），零自代码；另在守卫区加显式 ⓪ Origin 检查（`originAllowed(r)`）使拒绝发生在 Accept 前、错误形态与 /api/attach 一致、且 HTTP 层可测（不用 WS 客户端即可测 403）。双重执行同语义，纵深防御。
- `/api/attach`：中间件用同一 normalize 结果做精确集合查找（`map[string]struct{}`），语义与库对齐：空 Origin 放行 → 同源（`EqualFold(r.Host, u.Host)`）放行 → 集合内放行 → 否则 403。
- `Origin: null`（沙箱 iframe 等）：url.Parse("null") 后 u.Host 为空、不匹配任何项 → 拒绝。**这是正确行为**（null Origin 是 CSWSH 常见载体）[CITED: portswigger.net/web-security/websockets/cross-site-websocket-hijacking]。

### Pattern 5: TLS 配置与 ServeTLS 分岔（D-04/D-06）

**一手核实的关键事实**（GOROOT go1.26.3 源码，本次会话 Read）：

- `cipherSuitesPreferenceOrder`（cipher_suites.go:283-312）含 CBC 段：

```go
	// CBC w/ ECDHE
	TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
```

- `defaultCipherSuites`（defaults.go:69-77）只删除 `disabledCipherSuites`（RC4/CBC_SHA256）、RSA kex（GODEBUG tlsrsakex）、3DES（GODEBUG tls3des）——**4 个 ECDHE CBC-SHA1 套件留在默认列表**。testssl.sh 会把 CBC 归入 "Obsoleted CBC ciphers" 报告 → ROADMAP 准则 3"无弱项"过不了 [VERIFIED: GOROOT cipher_suites.go:283-312 + defaults.go:69-77；testssl.sh 行为 CITED: testssl.sh 官网]。
- `Config.CipherSuites` 仅约束 TLS 1.0–1.2；TLS 1.3 cipher 不可配（三个全是 AEAD）[VERIFIED: pkg.go.dev/crypto/tls]。
- `MinVersion` 默认 TLS 1.2、`MaxVersion` 默认 TLS 1.3 [VERIFIED: pkg.go.dev/crypto/tls]。
- `ServeTLS` 克隆 TLSConfig、加载证书对、包 tls.NewListener 后走 Serve（server.go:3482-3504 已核实）；TLS 模式下默认启用 HTTP/2——WS Upgrade 恒走 HTTP/1.1（浏览器行为），与 h2 并存无冲突。

**定稿配置：**

```go
func tlsConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		// 显式 AEAD-only 清单（D-06"1.2 弱 cipher 剔除"）——Go 默认列表含 CBC-SHA1，
		// 不设此清单 testssl.sh 必报弱项。1.3 套件不可配（全是 AEAD）。
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
	}
}

// main.go run() 分岔（D-04：成对才 TLS；单给 = parseArgs 报错）：
hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
if cfg.tlsCert != "" { // parseArgs 已保证 cert/key 成对
	hs.TLSConfig = tlsConfig()
	err = hs.ServeTLS(ln, cfg.tlsCert, cfg.tlsKey)
} else {
	err = hs.Serve(ln) // 明文：D-04/D-05 矩阵已先行约束 + stderr 醒目警告
}
```

附带收益：Go 1.26 默认曲线已含 X25519MLKEM768 等混合后量子密钥交换（defaults.go:19-37 已核实）——零配置获得。

### Pattern 6: 安全响应头中间件（D-06 按 OWASP 基线定稿）

OWASP HTTP Headers Cheat Sheet 官方推荐值 [CITED: cheatsheetseries.owasp.org/cheatsheets/HTTP_Headers_Cheat_Sheet.html]，结合单文件全内联现实（D-06）裁剪：

| Header | Value | When |
|--------|-------|------|
| Content-Security-Policy | `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'` | 恒在 |
| X-Frame-Options | `DENY` | 恒在（frame-ancestors 的老浏览器兼容，D-06） |
| X-Content-Type-Options | `nosniff` | 恒在 |
| Referrer-Policy | `strict-origin-when-cross-origin` | 恒在（OWASP 基线原值） |
| Cross-Origin-Opener-Policy | `same-origin` | 恒在 |
| Cross-Origin-Resource-Policy | `same-origin` | 恒在（比 OWASP same-site 更严；本站资源全部同源内联，零功能代价）[ASSUMED：无外部资源引用，UAT 视觉确认] |
| Strict-Transport-Security | `max-age=63072000; includeSubDomains` | **仅 TLS 分支**（D-06）；去 preload（自托管工具不进入浏览器预载表，误配代价大） |
| Cache-Control | `no-store` | 仅 `/api/attach` 响应（ticket 不可落缓存） |

```go
// securityHeaders 统一安全头中间件；tlsOn 区分 HSTS（D-06：明文发 HSTS 无意义且误导）。
func securityHeaders(next http.Handler, tlsOn bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if tlsOn {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
```

CSP 说明：
- `script-src/style-src 'unsafe-inline'`：vite-plugin-singlefile 全内联现实不可避免（D-06 已裁决接受）。
- `connect-src 'self'`：CSP3 中 'self' 覆盖同源 ws:/wss:；前端 WS 地址由 location.host 构造恒同源 [ASSUMED：CSP3 规范行为；老 Safari（<15.x）曾有不认 'self' 含 wss 的 bug——若 UAT 命中，回退 `connect-src 'self' ws: wss:`（放宽为任意 ws 端点，仅影响老浏览器）]。
- Go 默认不发 Server/X-Powered-By 头，无需移除动作。

### Pattern 7: 启动校验矩阵（D-03/D-05）

`parseArgs` 之后、`pty.Start` 之前的纯函数校验（错误文案进 main_test.go 断言）：

| bind | 凭据 | TLS | 逃生门 | 行为 |
|------|------|-----|--------|------|
| loopback | 无 | 无 | — | 放行免警告（D-03 现状保持） |
| loopback | 有 | 无 | — | 放行免警告（D-05：流量不出机） |
| loopback | 任意 | 有 | — | 放行（TLS 凭据最强形态） |
| 非 loopback（含默认 0.0.0.0） | 无 | 任意 | 无 `--no-auth` | **拒绝启动**："refusing to listen on non-loopback address without credentials; pass --no-auth to disable authentication"（D-03） |
| 非 loopback | 无 | 任意 | `--no-auth` | 放行 + stderr 醒目警告（裸奔显式确认） |
| 非 loopback | 有 | 无 | 无 `--insecure-http` | **拒绝启动**："refusing to serve credentials over plaintext HTTP on non-loopback address; pass --insecure-http or provide --tls-cert/--tls-key"（D-05） |
| 非 loopback | 有 | 无 | `--insecure-http` | 放行 + stderr 醒目警告（反代终止 TLS 的常态场景，D-04） |
| 任意 | — | 只给 cert/key 之一 | — | 启动报错（D-04：成对才 ServeTLS） |

loopback 判定：`bind == ""` 视为全网卡（非 loopback）；`net.ParseIP(bind).IsLoopback()`；`localhost` 特判 loopback；`0.0.0.0`/`::` 非 loopback；其他主机名保守按非 loopback（文档化）[ASSUMED：判定函数为平凡实现，无外部依赖]。

新 flag 一览（全名无短选项，P2 D-15 纪律；`--credential`/`--origin` 可重复用 `flag.Func` 收集）：
`--credential user:pass`（可重复）、`--tls-cert PATH`、`--tls-key PATH`、`--no-auth`、`--insecure-http`、`--origin ORIGIN`（可重复）。CONTEXT domain 节称"5 个新 CLI flag"——按决策点计数（cert/key 为一对），实际注册 6 个名字，见 Open Questions Q1。

### Pattern 8: 日志红线与脱敏测试（SEC-01 验收锚点）

红线：凭据、ticket、Authorization 头**任何形态（含 base64）**永不进任何日志。现状 `logEvent`（server.go:441-443）三要素（remote/code/reason）天然满足——新埋点（认证失败/节流事件）复用同函数，**禁止把凭据/ticket/Authorization 值作为任何参数传入**。

脱敏测试形态（Claude's Discretion 定稿）：os.Pipe 置换 os.Stderr 捕获，跑一轮完整失败认证（HTTP 401 + WS auth_failed），断言输出不含：(a) base64(user:pass) 串；(b) 明文 password；(c) 已签发的 ticket 值；(d) "authorization"（大小写不敏感）。另加 CI 静态红线：`grep -rn "credential\|ticket" internal/server/*.go | grep -i "log\|Fprintf"` 类扫描可选——**推荐以运行时捕获断言为主**（静态 grep 误报率高，运行时断言是行为证据）。

### Anti-Patterns to Avoid

- **明文比较凭据（`==`/`strings.Equal`）：** 逐字节短路时序泄露（ttyd strcmp 反例，C6）→ SHA-256 等长化 + subtle + 位与累积（Pattern 2）。
- **ticket 放 URL query 或子协议头：** query 进访问日志/浏览器历史；子协议头是非标癖好（ARCHITECTURE §2.8 已裁决）→ Hello JSON 载荷。
- **手写 WS 帧/握手解析：** 库已解决（coder/websocket）；Origin 检查也是库内建——自建 = ttyd protocol.c:51-71 弱比对反例的重演。
- **手写 TLS 版本/cipher 过滤逻辑（如握手后检查 ConnectionState 再断）：** tls.Config 声明式下限在握手层拒绝，事后补救窗口不存在才是正确形态。
- **给 ticket/凭据打日志"调试方便"：** ttyd server.c:142 凭据 base64 进日志的反例（本次会话已核实行号：`lwsl_notice("  credential: %s\n", server->credential)`，server.c:142 区域 print_config）→ 脱敏测试守住。
- **节流用 sleep 延迟响应：** 挂住连接资源（goroutine+fd），慢速攻击放大 → 429/auth_failed 失败即拒，Retry-After 告知。
- **为节流引入 x/time/rate：** 令牌桶语义与"失败计数→指数退避→成功清零"不匹配（见 Alternatives）。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TLS 握手/版本/cipher 下限 | 自过滤或后置检查 | `tls.Config{MinVersion, CipherSuites}` | 声明式在握手层拒绝；Go 1.26 默认已 1.2/1.3，显式清单只为剔 CBC [VERIFIED: GOROOT] |
| Basic 头解析 | 手拆 `Authorization: Basic base64(...)` | `r.BasicAuth()` | 官方解析器处理 base64/冒号边界 [VERIFIED: pkg.go.dev/net/http] |
| `/api/attach` POST-only 405 | handler 内 if method != POST | `mux.HandleFunc("POST /api/attach", h)` | ServeMux 方法模式自动 405 + `Allow` 头（server.go:2699-2710 已核实） |
| WS Origin 同源校验 | 自写字符串比较 | `AcceptOptions.OriginPatterns`（库内建） | ttyd 弱比对反例；库语义 = D-13（accept.go:228-264 已核实） |
| 密码学随机数 | math/rand、时间戳哈希 | crypto/rand | 可预测 ticket = 认证绕过 |
| 常数时间比较 | 自写循环 XOR | crypto/subtle | 编译器优化/分支预测会毁掉手写版 |
| 速率限制器 | x/time/rate 新依赖 | 进程内计数器（Pattern 3，~60 行） | 语义不匹配 + 零新依赖纪律 |

**Key insight:** 本 phase 的全部密码学正确性都来自"用对 stdlib 原语 + 声明式配置"，唯一需要手写的是**语义层**（ticket 生命周期、节流计数器、启动矩阵）——这些恰恰是库里没有、也借不来的领域逻辑。

## Common Pitfalls

### Pitfall 1: ConstantTimeCompare 的长度泄露
**What goes wrong:** 直接 `subtle.ConstantTimeCompare([]byte(pass), stored)`——长度不等立即返回 0，攻击者逐长度探测恢复凭据长度。
**Why it happens:** 官方文档一句话说明容易被跳过："If the lengths of x and y do not match it returns 0 immediately" [VERIFIED: pkg.go.dev/crypto/subtle]。
**How to avoid:** 双方先 `sha256.Sum256` 成 32B 定长摘要再比较（Pattern 2）；多组凭据位与 `&` 累积不短路。
**Warning signs:** 代码里 ConstantTimeCompare 的操作数是原始字符串。

### Pitfall 2: Go 默认 TLS 1.2 cipher 含 CBC——零值过不了 testssl.sh
**What goes wrong:** 只设 `MinVersion: tls.VersionTLS12`，以为 Go 默认足够安全；testssl.sh 报 "Obsoleted CBC ciphers" 弱项。
**Why it happens:** Go 为老客户端兼容在默认列表保留 4 个 ECDHE CBC-SHA1（cipher_suites.go:289-291；defaults.go 的 DeleteFunc 不删它们）[VERIFIED: GOROOT 1.26.3 源码]。
**How to avoid:** 显式 6 AEAD 清单（Pattern 5）；自动化测试用 CBC-only client 断言握手必败（D-07）。
**Warning signs:** tls.Config 里没有 CipherSuites 字段。

### Pitfall 3: Origin 默认端口不对称导致白名单永不命中
**What goes wrong:** 用户配 `--origin https://wesh.example.com:443`，浏览器发 `Origin: https://wesh.example.com`（RFC 6454 省略默认端口）——比较永远失败，正常用户被 403。
**Why it happens:** coder/websocket 库不做端口规范化（accept.go 逐字核实：pattern 直接与 u.Host 比）[VERIFIED: accept.go:243-254]。
**How to avoid:** flag 解析时剥离默认端口 + 小写 host（Pattern 4 normalizeOrigin）；e2e 覆盖"配置带默认端口 → 浏览器形态 Origin 放行"用例。
**Warning signs:** 反代部署下只有配了默认端口的用户报 403。

### Pitfall 4: ticket/凭据经调试日志外泄
**What goes wrong:** 排查认证问题时顺手 `log.Printf("ticket=%s", t)`；或 401 响应把 WWW-Authenticate 以外的请求详情打进 stderr；ttyd server.c:142 凭据 base64 进日志是已核实反例（本次会话 Read：`if (server->credential != NULL) lwsl_notice("  credential: %s\n", server->credential);`）。
**Why it happens:** 日志埋点没有红线审查；"base64 不是明文"的错觉（C6）。
**How to avoid:** 新埋点只走 `logEvent(remote, code, reason)` 三要素（server.go:441-443）；脱敏测试（Pattern 8）作为 SEC-01 硬验收；401/403 body 恒为通用文案。
**Warning signs:** 任何 log 调用参数里出现 ticket/credential/Authorization 变量名。

### Pitfall 5: 节流计数器 map 单调增长
**What goes wrong:** 每个失败 IP 永久驻留 map；扫描器打一周 = 内存稳步上涨（halfOpenCounter 同款 Pitfall 4 纪律）。
**Why it happens:** 只写不删；成功清零（recordSuccess）覆盖不了"从未成功"的攻击者条目。
**How to avoid:** 15min 无活动惰性过期（recordFail 内检查 lastSeen 重置）+ 认证成功 delete；条目 ~56B，4096 并发恶意 IP ≈ 230KB 可接受。
**Warning signs:** 长跑后 map len 只增不减。

### Pitfall 6: fetch 401 不弹浏览器原生对话框
**What goes wrong:** 以为 fetch 收到 401 会像页面导航一样弹 Basic 登录框——**不会**（fetch/XHR 的 401 直接返回给 JS）。
**Why it happens:** 浏览器原生弹窗只在顶级导航/子资源请求的 401+WWW-Authenticate 触发；D-02 成立的前提是**先导航 GET / 弹窗缓存凭据，后续同源 fetch 自动携带**（credentials 默认 same-origin 含 HTTP auth 条目）[ASSUMED：浏览器通行行为，UAT 必验]。
**How to avoid:** 前端 fetch 401 → 展示"认证失败，重新加载页面"（重新导航触发弹窗），不自建登录表单（D-02 零新 UI 纪律）。
**Warning signs:** 直接访问 `/api/attach` 测试时"没弹窗"——预期行为，不是 bug。

### Pitfall 7: HSTS 在明文 HTTP 分支发送
**What goes wrong:** 安全头中间件不分支，明文 HTTP 也发 HSTS——浏览器忽略之（规范要求仅 TLS），但暴露配置混乱；更糟的是反代终止 TLS 场景 wesh 明文发 HSTS 会与反代策略打架。
**How to avoid:** D-06 已裁决 HSTS 仅 TLS 分支；中间件带 `tlsOn bool` 参数（Pattern 6）；测试双分支断言。

### Pitfall 8: 凭据经 CLI flag 进 `ps` 输出
**What goes wrong:** `--credential admin:secret` 在命令行对同机所有用户可见（`ps aux`）。
**Why it happens:** CLI 参数天然非机密通道。
**How to avoid:** D-01 已提供 `WESH_CREDENTIAL` env 兜底 + flag 优先；README 部署章节必须把 EnvironmentFile 600 作为推荐形态（PITFALLS systemd 矩阵已有此条）；help 文案注明"flag 值对同机用户可见，生产建议 env"。
**Warning signs:** README 只示例 `--credential` 不提 env 路径。

### Pitfall 9: TLS 测试卡在 GODEBUG/证书夹具
**What goes wrong:** 想构造"TLS 1.1 客户端"发现 Go 1.26 默认不再提供 1.1（需 GODEBUG tls10client=1，进程启动期读取，t.Setenv 无效）；或引入 testdata 证书文件腐烂。
**How to avoid:** D-07 的"1.1 必败"断言=客户端 `MaxVersion: tls.VersionTLS11` 握手失败即可（客户端本地拒绝或服务端拒绝都算失败——断言连接错误，不区分哪侧）；证书用 crypto/x509 + ecdsa 测试内生成（stdlib 模式，无 fixture 文件）[ASSUMED：测试内生成自签 ECDSA 证书为 stdlib 通行模式，~40 行]。
**Warning signs:** 测试依赖 GODEBUG 环境变量；仓库出现 *.pem fixture。

## Code Examples

### 前端连接流程改造（web/src/main.ts）

现状（main.ts:73 + 150-161，本次会话 Read）：

```ts
const ws = new WebSocket('ws://' + location.host + '/ws', [SUBPROTOCOL]); // D-03：wesh.v1 子协议建连
...
ws.onopen = () => {
  opened = true;
  fit.fit();
  ws.send(
    concat(
      new Uint8Array([HELLO]),
      enc.encode(JSON.stringify({ version: SUBPROTOCOL, cols: term.cols, rows: term.rows })),
    ),
  );
  helloSent = true;
  term.focus();
};
```

改造为（auth 感知 + auth_failed 静默重试一次）：

```ts
// 连接流程：fetch ticket → new WebSocket → Hello{version,ticket,cols,rows}。
// 404 = 无认证模式（--no-auth/loopback 裸跑）跳过 ticket 直连；401/429 展示状态面板。
let retriedAuth = false; // auth_failed 静默重试仅一次（CONTEXT specifics：非无限循环）

async function connect(): Promise<void> {
  let ticket: string | undefined;
  try {
    const resp = await fetch('/api/attach', { method: 'POST' }); // 同源自动带 Basic 缓存凭据（D-02）
    if (resp.ok) {
      ticket = (await resp.json()).ticket;
    } else if (resp.status === 404) {
      ticket = undefined; // 无认证模式：直连
    } else {
      showStatus(
        resp.status === 429 ? 'Too many attempts' : 'Authentication failed',
        resp.status === 429
          ? 'Too many failed authentication attempts. Wait a moment, then'
          : 'The server rejected your credentials. Check with the operator, then',
        '',
      );
      return;
    }
  } catch {
    showStatus('Unable to connect', 'The wesh server is unreachable. It may have exited.', 'Check the shell where wesh is running, then');
    return;
  }

  const ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws', [SUBPROTOCOL]);
  ws.binaryType = 'arraybuffer';
  ws.onopen = () => {
    opened = true;
    fit.fit();
    ws.send(concat(new Uint8Array([HELLO]), enc.encode(JSON.stringify({
      version: SUBPROTOCOL, cols: term.cols, rows: term.rows, ticket, // ticket undefined 时 JSON 省略字段
    }))));
    helloSent = true;
    term.focus();
  };
  ws.onclose = (ev) => {
    // auth_failed（D-10）：ticket 过期是正常场景（页面放置 >60s）——静默重取重试一次
    if (lastError?.code === 'auth_failed' && !retriedAuth) {
      retriedAuth = true;
      lastError = null;
      void connect();
      return;
    }
    /* ……既有按码分派不变（1000/1008/1009/1011/1013/default，main.ts:206-249）…… */
  };
  /* ……onmessage/onerror 既有逻辑不变…… */
}
void connect();
```

### proto.go 扩展（verbatim 现状 + 增量）

现状 Error code 表（proto.go:43-46，本次会话 Read）：

```go
const (
	ErrVersionMismatch = "version_mismatch" // 正常客户端可见，发 Error 帧 + 1008
	ErrServerError     = "server_error"     // 发 Error 帧 + 1011
)
```

增量（D-10 兑现 P2 deferred 挂账）：

```go
	ErrAuthFailed = "auth_failed" // ticket 核销失败统一口径（过期/非法/重放/节流），发 Error 帧 + 1008
```

现状 HelloPayload（proto.go:71-75，本次会话 Read）：

```go
type HelloPayload struct {
	Version string `json:"version"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
}
```

增量（P2 D-02 演化纪律：纯加字段不破坏协议，未知字段忽略不受影响）：

```go
	Ticket  string `json:"ticket,omitempty"`
```

### TLS 自动化断言（D-07 形态，测试内自签证书）

```go
// 客户端 MaxVersion 1.1 必败 / 1.2 必成 / 1.3 默认 / CBC-only 必败
func TestTLSVersionAndCipherFloor(t *testing.T) {
	// 测试内 crypto/x509 + ecdsa.P256 自签 localhost 证书（无 fixture 文件）
	// server: tlsConfig()（Pattern 5）包 tls.NewListener + httptest 级装配
	cases := []struct {
		name    string
		cfg     *tls.Config
		wantErr bool
	}{
		{"tls11-rejected", &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}, true},
		{"tls12-ok", &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}, false},
		{"tls13-default", &tls.Config{InsecureSkipVerify: true}, false},
		{"cbc-only-rejected", &tls.Config{MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA}, InsecureSkipVerify: true}, true},
	}
	// 逐例 tls.Dial，断言握手成败；tls13-default 追加断言 ConnectionState().Version == tls.VersionTLS13
}
```

### testssl.sh 手动 UAT 命令（文档化进 03-UAT.md，不进 CI——D-07）

```bash
# 本机无 testssl.sh 二进制，docker 24.0.6 可用（已实测）：
docker run --rm -ti drwetter/testssl.sh --protocols --std --server-defaults --header <host>:<port>
# 全量（含漏洞扫描，耗时分钟级）：
docker run --rm -ti drwetter/testssl.sh -U <host>:<port>
```

[CITED: testssl.sh 官网选项表：-p/--protocols、-s/--std、-S/--server-defaults、-h/--header、-U/--vulnerable]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| TLS 1.0/1.1 支持 | Go 默认 MinVersion=TLS 1.2（GODEBUG tls10server=1 才回退） | Go 1.22 | wesh 显式 MinVersion 1.2 是文档化契约而非依赖默认 [VERIFIED: pkg.go.dev/crypto/tls] |
| RSA key exchange 套件在默认列表 | 从默认移除（tlsrsakex=1 回退） | Go 1.22 | 无前向保密套件默认出局 |
| 3DES 在默认列表 | 从默认移除（tls3des=1 回退） | Go 1.23 | SWEET32 面关闭 |
| —（经典曲线） | X25519MLKEM768/SecP256r1MLKEM768 混合后量子密钥交换进默认曲线 | Go 1.24/1.26 | 零配置获得 PQ 混合握手 [VERIFIED: GOROOT defaults.go:19-37] |
| ServeMux 仅路径路由 | 方法模式 `"POST /path"` + 自动 405/Allow | Go 1.22 | /api/attach POST-only 零自代码 [VERIFIED: GOROOT server.go:2699-2710] |
| X-XSS-Protection 头 | 废弃（设 0 或不设，以 CSP 替代） | 业界共识（OWASP） | 不发送 |
| Expect-CT / Public-Key-Pins | 废弃 | 业界共识（OWASP 标记 ❌） | 不发送 |
| ttyd /token 明文下发长期凭据 | 一次性 60s ticket + Hello 核销 | 本 phase | C6 修复核心 |

**Deprecated/outdated:**
- `PreferServerCipherSuites`：Go 已废弃（忽略，无效果）[VERIFIED: pkg.go.dev/crypto/tls]——不要使用。
- TLS 1.3 cipher 配置尝试：不可配（设计如此），任何"配 1.3 cipher"的代码都是死代码。
- 自签证书 + `InsecureSkipVerify` 教程：PITFALLS 已列为反模式（文档推荐 mkcert/CA 方案，不给 skip-verify 教程）——UAT 文档遵守。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | CSP `connect-src 'self'` 覆盖同源 ws:/wss:（CSP3）；老 Safari（<15.x）有例外 bug | Pattern 6 | 低-中：老 Safari 上 WS 连接被 CSP 拦截 → 回退 `connect-src 'self' ws: wss:`；UAT 浏览器实测覆盖 |
| A2 | 浏览器对同源 fetch 自动附带已缓存的 HTTP Basic 凭据（D-02 成立前提） | Pitfall 6 / 前端流程 | 高（若错则 D-02 流程断裂）但属浏览器通行行为；UAT 必验"弹窗一次后 fetch 200" |
| A3 | 退避参数 1s/×2/30s/无锁定为正确标定 | Pattern 3 | 低：OWASP 原文锚定 1s 翻倍；100 次 ≥47min 满足验收锚点；planner/用户可在 plan 阶段调整 |
| A4 | CORP `same-origin`（比 OWASP same-site 更严）对全内联单页零功能影响 | Pattern 6 | 极低：本站无跨源资源；UAT 视觉确认 |
| A5 | `/api/attach` 响应 JSON 字段名 `ticket`、无认证模式 404 探测 | Pattern 1 | 低：Claude's Discretion 内事项，前后端同仓同步落地 |
| A6 | map 查表核销 ticket 无实际时序攻击面（2^128 随机空间 + map 实现细节） | Pattern 1 | 低：通行实践；若有疑虑可对 ticket 也走 SHA-256+subtle，成本极小但无必要 |
| A7 | 测试内生成自签 ECDSA 证书（crypto/x509）为 stdlib 通行测试模式 | Pitfall 9 | 低：stdlib 自身测试用 internal/testcert，外部项目等效自生成 ~40 行 |
| A8 | loopback 判定规则（空=非 loopback、localhost=loopback、其他主机名保守非 loopback） | Pattern 7 | 低：保守方向出错=多要一个显式逃生门 flag，不削弱安全 |

## Open Questions

1. **"5 个新 CLI flag"与实际 6 个名字的计数口径**
   - What we know: CONTEXT domain 节写"5 个新 CLI flag"；D-01/D-03/D-04/D-05/D-12 共 5 个决策点，但注册名为 6 个（--credential/--tls-cert/--tls-key/--no-auth/--insecure-http/--origin）
   - What's unclear: 纯计数口径，无实质分歧
   - Recommendation: 按 6 个注册名实现；planner 不必回头确认
2. **Hello 核销与 version 检查的先后顺序**
   - What we know: D-10 要求核销失败统一 auth_failed；version_mismatch 是既有独立码（非秘密信息）
   - What's unclear: version 先查（不匹配报 version_mismatch）还是先核销
   - Recommendation: version 先查（公开协议信息，无 oracle），核销紧随其后；planner 最终定夺（CONTEXT 已授权）
3. **HSTS max-age 取值**
   - What we know: OWASP 推荐 63072000（2 年）；自托管工具主机名/证书生命周期短
   - What's unclear: 2 年承诺对临时实例过重（换 HTTP 后用户浏览器仍强制 HTTPS 至过期）
   - Recommendation: 按 OWASP 基线 63072000 落地（D-06 授权"按 OWASP 基线定稿"）；UAT 文档注明 HSTS 粘性风险

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | 全部实现与测试 | ✓ | go1.26.3 linux/amd64（与 go.mod 一致） | — |
| Node | web/uat/phase03.mjs 协议 UAT | ✓ | v24.13.0（≥22 满足） | — |
| pnpm | 前端构建 | ✓ | 11.21.0（CI 同钉） | — |
| docker | testssl.sh UAT | ✓ | 24.0.6 | — |
| testssl.sh 二进制 | TLS 审计 | ✗ | — | `docker run --rm -ti drwetter/testssl.sh`（官方镜像，官网核实） |
| curl | 手动探测（401/403/安全头） | ✓ | 7.61.1 | — |
| openssl | （可选）手动造证书 | ✓ | 1.1.1k | 测试内 Go 自签（推荐，无外部依赖） |

**Missing dependencies with no fallback:** 无
**Missing dependencies with fallback:** testssl.sh 二进制 → docker 官方镜像（D-07 本就定为手动 UAT，不阻塞执行）

Step 2.5 Runtime State Inventory: **SKIPPED**——本 phase 为纯增量功能（新增端点/flag/中间件），无 rename/refactor/migration。唯一顺手项：server.go:167 与 224-225 注释误写"Phase 3 SEC-07"，本 phase 校正为 Phase 7（CONTEXT deferred 节明示）。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib + `-race`（无 testify 等断言库，现状延续） |
| Config file | none（`go test` 零配置；CI = `go vet` + `go test -race -count=1 -v ./...`，.github/workflows/ci.yml 已核实） |
| Quick run command | `go test -count=1 ./internal/server/ ./internal/proto/ ./cmd/wesh/` |
| Full suite command | `go test -race -count=1 -v ./... && pnpm -C web build && node web/uat/phase03.mjs /tmp/wesh-uat/wesh` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEC-01 | 常数时间比较（正/错 user/pass、多组轮询） | unit | `go test ./internal/server/ -run TestCredentialMatch -count=1` | ❌ 新建 auth_test.go |
| SEC-01 | 日志红线：stderr 不含 base64/凭据/ticket/Authorization | integration（os.Pipe 捕获） | `go test ./internal/server/ -run TestLogRedaction -count=1` | ❌ 新建（Pattern 8） |
| SEC-02 | /api/attach：401/405/429/200+JSON+no-store | integration（httptest 级） | `go test ./internal/server/ -run TestAttachEndpoint -count=1` | ❌ 新建 auth_test.go |
| SEC-02 | ticket 单次使用：重放同一 ticket → auth_failed+1008 | e2e（WS 握手级） | `go test ./internal/server/ -run TestTicketReplay -count=1` | ❌ 新建 |
| SEC-02 | ticket 60s TTL 过期拒绝（Options 注入短 TTL） | e2e | `go test ./internal/server/ -run TestTicketExpiry -count=1` | ❌ 新建 |
| SEC-03 | 爆破 N 次 → 429+Retry-After；成功清零；Hello 失败同计数器 | integration（Options 注入 ms 级 base） | `go test ./internal/server/ -run TestThrottle -count=1` | ❌ 新建 throttle_test.go |
| SEC-04 | /ws 与 /api/attach：无 Origin 放行/同源放行/白名单放行/邪恶源 403/null 拒绝 | integration | `go test ./internal/server/ -run TestOrigin -count=1` | ❌ 新建 origin_test.go |
| SEC-05 | TLS 1.1 必败/1.2 必成/1.3 默认/CBC-only 必败 | integration（tls.Dial 矩阵） | `go test ./internal/server/ -run TestTLS -count=1` | ❌ 新建 tls_test.go |
| SEC-05 | 安全头：TLS→HSTS 在、明文→无 HSTS、nosniff/CSP/XFO 恒在 | integration | `go test ./internal/server/ -run TestSecurityHeaders -count=1` | ❌ 新建 tls_test.go |
| D-03/D-05 | 启动校验矩阵 8 行（拒绝路径文案断言） | unit（parseArgs/run 级） | `go test ./cmd/wesh/ -run TestStartupMatrix -count=1` | ❌ 扩展 main_test.go |
| 准则 1 | 完整链路：Basic → ticket → Hello → Welcome → 重放拒绝 | e2e + UAT | `node web/uat/phase03.mjs <bin>` | ❌ 新建（沿用 phase02.mjs 模式） |
| 准则 3 | testssl.sh 无弱项 | manual-only（D-07 裁决不进 CI） | `docker run --rm -ti drwetter/testssl.sh --protocols --std --header host:port` | UAT 清单文档化 |

**Manual-only 说明：** testssl.sh 外部依赖重（docker 镜像 ~百 MB、分钟级扫描）与轻量 CI 风格不搭（D-07 用户已裁决）；浏览器原生 Basic 弹窗与"凭据缓存后 fetch 自动携带"（A2）必须人工浏览器确认。

### Sampling Rate
- **Per task commit:** `go test -count=1 ./internal/server/ ./internal/proto/ ./cmd/wesh/`（秒级）
- **Per wave merge:** `go test -race -count=1 -v ./... && pnpm -C web build`
- **Phase gate:** 全量绿 + `node web/uat/phase03.mjs` 全 PASS + testssl.sh 手动 UAT 无弱项，再 `/gsd-verify-work`

### Wave 0 Gaps
- 无独立 Wave 0 基建需求：测试框架/e2e 装配（startTestServerWith/dialHello，e2e_test.go:105-121 已核实）/CI 全部就位
- 随测试任务落地的辅助件（非独立 Wave 0）：TLS 自签证书 helper（~40 行 stdlib）、stderr os.Pipe 捕获 helper、Options 注入字段（TicketTTL/ThrottleBase 等，延续 HelloTimeout 测试覆写先例 server.go:63-69）

## Security Domain

### Applicable ASVS Categories（security_asvs_level: 1）

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Basic over TLS（或 loopback）+ SHA-256 等长化 subtle 比较 + per-IP 指数退避（Pattern 2/3）；多组凭据按人撤销（D-01） |
| V3 Session Management | yes（ticket 即会话凭据） | 128bit crypto/rand、单次使用原子核销、60s TTL、独立 secret、不进 URL/日志（Pattern 1） |
| V4 Access Control | yes（ticket 绑 mode） | ticket.mode = 全局 --writable（D-11）；ro 服务端丢 INPUT 既有边界（P2 D-13）不动 |
| V5 Input Validation | yes | ServeMux 方法模式 405、MaxBytesReader body 上限、Hello JSON 未知字段忽略纪律（P2 D-02）、Origin 规范化白名单 |
| V6 Cryptography | partial | 不做数据落盘加密；密码学用途仅限 TLS（Pattern 5 声明式）与 ticket 随机源——均 stdlib，无手卷 |
| V7 Error Handling & Logging | yes | 日志红线 + 脱敏测试（Pattern 8）；通用 401/403 文案（无用户枚举 oracle）；logEvent 三要素唯一出口 |
| V9 Communications | yes | MinVersion TLS 1.2、显式 6 AEAD 清单、HSTS 仅 TLS、testssl.sh UAT（Pattern 5/6） |
| V12 Files & Resources | no | 本 phase 无文件上传/静态资源变更（embed 伺服现状不动） |
| V13 API | yes | /api/attach 是 JSON API：POST-only、no-store、通用错误文案、节流 |

### Known Threat Patterns for {stack}（Go net/http + coder/websocket + 浏览器 Basic/ticket）

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 凭据爆破（在线字典攻击） | Elevation of Privilege | per-IP 指数退避（Pattern 3）+ 常数时间比较防时序辅助 + 通用 401 无枚举 oracle |
| 时序侧信道恢复凭据 | Information Disclosure | SHA-256 等长化 + subtle + 逐组位与（Pattern 2） |
| CSWSH（跨站 WS 劫持） | Elevation of Privilege / Tampering | Origin 白名单双端点执行（Pattern 4）；ticket 二次屏障（攻击者跨源读不到 /api/attach 响应）[CITED: PortSwigger CSWSH] |
| 凭据/ticket 日志外泄 | Information Disclosure | 日志红线 + 运行时脱敏测试（Pattern 8） |
| 降级攻击（TLS 1.0/1.1/弱 cipher） | Tampering | MinVersion 1.2 + 显式 AEAD 清单 + HSTS（Pattern 5/6） |
| ticket 重放/枚举 | Spoofing | 单次使用原子核销 + 60s TTL + 128bit 空间（Pattern 1） |
| 慢速 body/连接资源占用（Slowloris 变体） | Denial of Service | ReadHeaderTimeout 5s（现状 main.go:92）+ MaxBytesReader + 429 即拒不挂连接 |
| Clickjacking（终端页被 iframe 嵌套钓鱼） | Tampering | frame-ancestors 'none' + X-Frame-Options: DENY（Pattern 6） |
| MIME 嗅探/XSS 辅助面 | Tampering | nosniff + CSP default-src 'self'（Pattern 6） |
| 命令行凭据 `ps` 可见 | Information Disclosure | env 兜底 + EnvironmentFile 600 文档化（Pitfall 8；D-01 已定） |

## Sources

### Primary (HIGH confidence)
- **GOROOT go1.26.3 源码（本机 Read）**：`src/crypto/tls/cipher_suites.go:56-73,283-312`（默认/安全 cipher 列表、CBC 在默认集）、`src/crypto/tls/defaults.go:19-37,69-77`（PQ 曲线、defaultCipherSuites 删除逻辑）、`src/net/http/server.go:2699-2710`（ServeMux 方法模式 405+Allow）、`src/net/http/server.go:3482-3504`（ServeTLS 语义）
- **coder/websocket v1.8.15（GOMODCACHE Read）**：`accept.go:228-264`（authenticateOrigin 全函数 + match path.Match glob 语义）
- **wesh 现状代码（本次会话 Read）**：server.go（守卫区 200-223、logEvent 441-443、halfOpenCounter 136-162、clientIP 168-174）、proto.go（Error 表 43-46、HelloPayload 71-75）、main.go（flags 38-42、http.Server 86-96）、web/src/main.ts（73、150-161、206-249）、web/embed.go、go.mod、ci.yml
- **ttyd 1.7.7 本地源码（本次会话 Read）**：server.c:142 区域（print_config 凭据进日志反例）、protocol.c:51-71（check_host_origin 弱比对反例）

### Secondary (MEDIUM confidence)
- pkg.go.dev/crypto/tls（WebFetch 官方文档）：MinVersion/MaxVersion 默认、CipherSuites 仅 1.0-1.2、1.3 不可配、PreferServerCipherSuites 废弃
- pkg.go.dev/crypto/subtle（WebFetch 官方文档）：ConstantTimeCompare 长度立即返回 0
- OWASP HTTP Headers Cheat Sheet（WebFetch 官方）：各安全头推荐值逐条
- OWASP Authentication Cheat Sheet（WebFetch 官方）：指数退避 1s 翻倍原文、通用错误消息、按账户计数建议（及不适用本模型的论证）
- PortSwigger Web Security Academy CSWSH（WebFetch）：CSWSH 机制与 Origin 防线
- testssl.sh 官网（WebFetch）：选项表与 docker 镜像用法

### Tertiary (LOW confidence)
- fail2ban 默认值（bantime 600s/findtime 10m/maxretry 5）：多个二手教程交叉一致，官方文档未本次直读——仅作量级佐证，不作为参数来源
- 浏览器 fetch 同源自动带 Basic 缓存凭据（A2）：通行行为，UAT 实测兜底
- CSP 'self' 对 ws/wss 的覆盖与老 Safari bug（A1）：CSP3 规范认知，UAT 实测兜底

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 全部 stdlib/现依赖，API 均经本机 GOROOT/GOMODCACHE 一手核实
- Architecture: HIGH — ticket/节流/Origin/TLS 模式均有官方文档或一手源码锚点；与 CONTEXT 决策逐条对齐
- Pitfalls: HIGH — C6 全套为 ttyd 源码实证 + stdlib 行为一手核实；A1/A2 两个浏览器行为假设已标注并安排 UAT 兜底
- 参数标定（退避/HSTS）: MEDIUM — OWASP 原文锚定方向，具体数值为研究建议（D-09 授权），planner/用户可调整

**Research date:** 2026-08-17
**Valid until:** 2026-09-16（stdlib 行为钉 go1.26.3；OWASP 基线稳定；Go 版本升级时需复核 cipher 默认集——本项目 go.mod 钉版，风险低）
