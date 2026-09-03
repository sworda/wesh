---
phase: 03-auth
plan: 02
subsystem: auth
tags: [sha256, crypto-subtle, constant-time, origin-allowlist, cswsh, tls1.2, aead-ciphers, csp, hsts, security-headers]

requires:
  - phase: 02-protocol
    provides: headerHasToken 精确比较纪律、守卫区拒绝形态、logEvent 三要素出口（server.go 既有模式）
provides:
  - Credential 导出类型（字段不导出，SHA-256 预哈希摘要对不变量）+ ParseCredential（首个 ':' 切分）
  - matchCredential 常数时间多组轮询（|= 不短路，erratum 修正形态）
  - NormalizeOrigin 导出（小写 host + 剥默认端口 + 拒 path/query/fragment/userinfo/glob）
  - originAllowed 四段语义（空放行/同源放行/集合查找/否则拒，与 coder/websocket accept.go:228-264 对齐）
  - securityHeaders 中间件（恒在六项 + tlsOn 分支 HSTS）
  - TLSConfig（MinVersion 1.2 + 显式 6 AEAD 清单）
affects: [03-03 server 集成, 03-04 main 装配, 03-06 UAT]

actuals:
  tokens: 5700
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "SHA-256 等长化 + subtle.ConstantTimeCompare + 逐组 |= 不短路（常数时间凭据比较三件套）"
    - "func(http.Handler) http.Handler 最小中间件形态（本仓首例，无框架引入）"
    - "测试内 crypto/x509+ecdsa 自签证书 helper（无 *.pem fixture、无 GODEBUG 依赖）"

key-files:
  created:
    - internal/server/auth.go
    - internal/server/auth_test.go
    - internal/server/origin.go
    - internal/server/origin_test.go
    - internal/server/headers.go
    - internal/server/tls.go
    - internal/server/tls_test.go
  modified: []

key-decisions:
  - "matchCredential 按 planner erratum 修正形态实现（|= 位或累积；RESEARCH Pattern 2 的 &= 初值 0 恒 false 不可照抄），TestCredentialMatch 多组各自命中锁死该回归"
  - "空 pass 合法（ParseCredential(\"user:\") 不额外禁止，passHash 为空串摘要）——文档化决策"

patterns-established:
  - "常数时间比较构造保证（subtle+定长摘要+不短路循环），时序测量断言不可移植不做（backstop truth 对应物）"
  - "安全契约值（CSP/HSTS）测试逐字符精确断言，防手滑漂移"

requirements-completed: [SEC-01, SEC-04, SEC-05]

coverage:
  - id: D1
    description: "凭据预哈希与常数时间比较组件（Credential/ParseCredential/matchCredential）"
    requirement: SEC-01
    verification:
      - kind: unit
        ref: "internal/server/auth_test.go#TestParseCredential"
        status: pass
      - kind: unit
        ref: "internal/server/auth_test.go#TestCredentialMatch"
        status: pass
    human_judgment: false
  - id: D2
    description: "Origin 规范化与白名单检查（NormalizeOrigin/originAllowed，含 null Origin 拒绝）"
    requirement: SEC-04
    verification:
      - kind: unit
        ref: "internal/server/origin_test.go#TestNormalizeOrigin"
        status: pass
      - kind: unit
        ref: "internal/server/origin_test.go#TestOriginAllowed"
        status: pass
    human_judgment: false
  - id: D3
    description: "TLS 声明式配置与版本/cipher 下限矩阵（1.1 败/1.2 成/1.3 默认/CBC 败）"
    requirement: SEC-05
    verification:
      - kind: integration
        ref: "internal/server/tls_test.go#TestTLSVersionAndCipherFloor"
        status: pass
    human_judgment: false
  - id: D4
    description: "安全响应头中间件双分支（恒在六项精确值 + HSTS 仅 TLS 分支）"
    requirement: SEC-05
    verification:
      - kind: unit
        ref: "internal/server/tls_test.go#TestSecurityHeaders"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-08-17
status: complete
---

# Phase 03 Plan 02: 认证与传输安全组件 Summary

**四个零依赖纯组件落地：凭据 SHA-256 预哈希 + subtle 逐组不短路比较（erratum 修正形态）、Origin 规范化/四段白名单检查、OWASP 六项安全头中间件（HSTS 仅 TLS 分支）、TLS 1.2 下限 + 显式 6 AEAD 清单——全部 TDD（RED→GREEN 六提交），D-07 自动化矩阵四例锁定。**

## Performance

- **Duration:** 15 min
- **Started:** 2026-08-17T07:28:29Z
- **Completed:** 2026-08-17T07:43:26Z
- **Tasks:** 3
- **Files modified:** 0（7 个全新创建）

## Accomplishments

- SEC-01 比较核心锁定：Credential 导出类型字段不导出（只能经 ParseCredential 构造，不变量 = 永远是 32B 预哈希摘要），matchCredential 位或累积不短路（planner erratum 修正形态，多组命中测试直接锁死 `&=` 恒 false 回归）
- SEC-04 规范化核心锁定：NormalizeOrigin 小写 host + 剥默认端口（RFC 6454，Pitfall 3）+ 拒 glob 字符（D-12 精确比较）；originAllowed 与 coder/websocket accept.go:228-264 逐项对齐，null Origin（CSWSH 载体）拒绝专断言
- SEC-05 双组件锁定：TLSConfig 显式 6 AEAD 清单（GOROOT 1.26.3 默认含 4 个 CBC-SHA1，Pitfall 2）；D-07 自动化矩阵（1.1 必败/1.2 必成/1.3 默认/CBC-only 必败）四例全过；securityHeaders 恒在六项 + HSTS 仅 tlsOn 分支（Pitfall 7），CSP 值与 RESEARCH Pattern 6 逐字符一致（diff 实测）

## Task Commits

每个任务按 TDD 双提交（test → feat）原子落地：

1. **Task 1: auth.go 凭据预哈希与常数时间比较** - `1eab62f` (test) + `00bbdc0` (feat)
2. **Task 2: origin.go Origin 规范化与白名单检查** - `3b471eb` (test) + `7beb3a1` (feat)
3. **Task 3: headers.go + tls.go 安全头与 TLS 配置** - `4e9cd42` (test) + `b4782bc` (feat)

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS 更新）

## Files Created/Modified

- `internal/server/auth.go` - Credential（导出，字段不导出）+ ParseCredential + matchCredential（|= 不短路）
- `internal/server/auth_test.go` - TestParseCredential 5 行表驱动 + TestCredentialMatch（单组/空列表/三组各自命中/跨组错配/空 user/pass）
- `internal/server/origin.go` - NormalizeOrigin（导出）+ originAllowed（四段语义）
- `internal/server/origin_test.go` - TestNormalizeOrigin 13 行 + TestOriginAllowed 9 行表驱动
- `internal/server/headers.go` - securityHeaders 中间件（本仓中间件首例，最小形态）
- `internal/server/tls.go` - TLSConfig（MinVersion 1.2 + 6 AEAD，无 MaxVersion/PreferServerCipherSuites/1.3 cipher 死代码）
- `internal/server/tls_test.go` - 自签证书 helper + TestTLSVersionAndCipherFloor 四例 + TestSecurityHeaders 双分支

## Decisions Made

- **matchCredential erratum 修正形态落地**：RESEARCH Pattern 2 行 288-297 的 `matched &= ...`（初值 0）恒为 false，按 plan objective 修正为 `matched |= user比较 & pass比较`；TestCredentialMatch「多组各自命中」三例直接锁死该回归（任一退化为 `&=` 即 RED）
- **空 pass 合法**：`ParseCredential("user:")` 不额外禁止（passHash 为空串摘要），按 plan 行为规格文档化；空 user 仍拒绝（RFC 7617 user-id 约束）

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- PATH 上的 `/usr/bin/gofmt` 陈旧（解析 server.go 泛型报错），按 Phase 01-03 既有决策改用 GOROOT gofmt 格式化 tls_test.go——已知环境问题，非本 plan 产物缺陷。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 03-03 集成的全部组件输入面就绪：`Credential`/`ParseCredential`（basicAuth 中间件装配）、`NormalizeOrigin`/`originAllowed`（守卫区 ⓪ + AcceptOptions.OriginPatterns + /api/attach 三处执行点）、`securityHeaders`（mux 外层包装）、`TLSConfig`（预留 03-04 分岔）
- 03-04 消费点就绪：`ParseCredential`/`NormalizeOrigin` 供 --credential/--origin 的 fs.Func 回调 parse 期校验；`TLSConfig()` 供 run() TLS 分岔
- 无阻塞；testssl.sh 手动复核属 03-06 UAT 清单（D-07 分工）

## Self-Check: PASSED
