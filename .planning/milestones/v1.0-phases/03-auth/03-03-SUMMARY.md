---
phase: 03-auth
plan: 03
subsystem: auth
tags: [basic-auth, one-time-ticket, throttling, origin-whitelist, security-headers, websocket, integration-test]

requires:
  - phase: 03-01
    provides: ticketStore（issue/redeem 原子查删）、throttleStore（allow/recordFail/recordSuccess）、proto Ticket 字段与 ErrAuthFailed 常量
  - phase: 03-02
    provides: Credential/ParseCredential/matchCredential（常数时间比较）、originAllowed/NormalizeOrigin、securityHeaders 中间件
provides:
  - server.go 集成完成：Options 六新字段（Credentials/Origins/TLS/TicketTTL/ThrottleBase/ThrottleCap）、Handler() 重装配（securityHeaders 最外层 + 整站 Basic + /api/attach 守卫链 + 显式 405 fallback）、attachHandler 签发端点、Attach 守卫区 ⓪ Origin、握手段 checkTicket 核销闸与 auth_failed 统一口径分支
  - auth.go basicAuth 中间件（retryAfter 429 闸 + 401 同文 challenge + D-08 统一计数器钩子）
  - origin.go originMiddleware（/api/attach Origin 闸，与 /ws ⓪ 同形态）
  - throttle.go retryAfter 只读访问器（429 Retry-After 数据源，不延长窗口）
  - auth_e2e_test.go 九个集成测试（黑盒 server_test）全部 -race 绿色
affects: [03-04 main 装配, 03-05 前端消费, 03-06 UAT]

actuals:
  tokens: 15414
  tasks: 3
  commits: 3

tech-stack:
  added: []（零新增依赖，RESEARCH §Package Legitimacy Audit 纪律）
  patterns:
    - "HTTP 中间件链组合：securityHeaders(basicAuth/originMiddleware(...))——仓内首批 func(http.Handler) http.Handler 先例的装配形态"
    - "D-08 统一节流计数器：/api/attach 凭据失败与 Hello 核销失败计入同一 per-IP throttleStore，成功清零，429 短路不 recordFail 不延长窗口"
    - "D-10 无 oracle 统一口径：过期/非法/重放/节流中同 Error{auth_failed}+1008 同名 reason；401 无/错凭据 body 逐字节同文"
    - "ServeMux 405 显式 fallback：方法模式内建 405 被 / 子树吞掉时补同文 fallback（Allow: POST）"

key-files:
  created:
    - internal/server/auth_e2e_test.go
  modified:
    - internal/server/server.go
    - internal/server/auth.go
    - internal/server/origin.go
    - internal/server/throttle.go
    - internal/server/throttle_test.go
    - internal/server/e2e_test.go

key-decisions:
  - "logEvent 从 Server 方法提为包级函数：plan 指定的 basicAuth 三参自由函数签名需调用日志出口，logEvent 无状态依赖；HTTP 层事件 code 复用 HTTP 状态码值（websocket.StatusCode 底层 int，PATTERNS 裁决），三要素唯一出口红线不变"
  - "originMiddleware 落位 origin.go（与 originAllowed 内聚同文件；plan step 5 显式命名该中间件，files_modified 未列 origin.go 属清单小缺口）"
  - "TestLogRedaction 复用 limits_test.go 既有 captureStderr（同包 server_test 已有 os.Pipe 同形态 helper，plan 字面「复制 main_test.go captureFd」调整为复用，避免重声明编译冲突）"
  - "TestOriginEndpoints 全部场景共用单实例收口：八场景均为 HTTP 层拒绝（400/403/401 零 attach 零终结路径），plan 字面「每 WS 场景独立实例 waitExit(0)」结构性不可达（无 attach 则 exitf 永不触发），注释钉死理由；≥7 子断言验收满足"

patterns-established:
  - "中间件守卫链：405(显式 fallback) → Origin 403 → 429+Retry-After → Basic 401 同文 → 签发 200，顺序敏感注释钉死"
  - "checkTicket 核销闸收口：节流命中不核销不 recordFail → redeem 失败 recordFail → 成功返回 ticket 绑定 mode；无认证模式整体跳过恒放行"
  - "集成测试 pacing 模式：连续失败编排必须 ThrottleBase ms 级覆写 + sleep 过窗，生产默认 base=1s 下相邻断言确定性撞窗"

requirements-completed: [SEC-01, SEC-02, SEC-03, SEC-04]

coverage:
  - id: D1
    description: "/api/attach 端点守卫链与一次性 ticket 签发（405+Allow / 413 / 403 不回显 / 429 / 401 同文 / 200 三头 + no-store + 22 字符 ticket）"
    requirement: SEC-02
    verification:
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestAttachFlow"
        status: pass
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestAttachEndpoint"
        status: pass
    human_judgment: false
  - id: D2
    description: "Hello 首帧 ticket 核销与 D-10 统一口径（非法/过期同 Error{auth_failed}+1008 同名 reason，无 oracle）"
    requirement: SEC-02
    verification:
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestTicketInvalid"
        status: pass
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestTicketExpiry"
        status: pass
    human_judgment: false
  - id: D3
    description: "无认证模式零漂移（/api/attach 404 探测 + Hello 无 ticket 直连收 Welcome；既有全部测试零改动绿色）"
    requirement: SEC-01
    verification:
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestNoAuthMode"
        status: pass
      - kind: e2e
        ref: "go test -race -count=1 ./...（全量零漂移）"
        status: pass
    human_judgment: false
  - id: D4
    description: "日志红线（SEC-01）：完整失败轮后 stderr 不含 base64(凭据)/明文密码/ticket 值/authorization，正向对照防假绿"
    requirement: SEC-01
    verification:
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestLogRedaction"
        status: pass
    human_judgment: false
  - id: D5
    description: "HTTP 层指数退避（SEC-03）：429+Retry-After≥1、窗口后成功清零、级数从 base 重启（ROADMAP 准则 2 锚点经 HTTP 层集成测试证明）"
    requirement: SEC-03
    verification:
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestThrottleHTTP"
        status: pass
    human_judgment: false
  - id: D6
    description: "D-08 统一计数器行为级反证：HTTP 凭据失败编排后合法 ticket 被 WS 侧 auth_failed 拒绝（无共享计数器必收 Welcome）"
    requirement: SEC-03
    verification:
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestThrottleHelloSharedCounter"
        status: pass
    human_judgment: false
  - id: D7
    description: "SEC-04 双端点 Origin 执行：/ws ⓪+库 OriginPatterns 与 /api/attach originMiddleware（邪恶源/null 403、同源/白名单/无 Origin 放行、403 不回显不计数）"
    requirement: SEC-04
    verification:
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestOriginEndpoints"
        status: pass
    human_judgment: false

duration: 52min
completed: 2026-08-17
status: complete
---

# Phase 03 Plan 03: 认证主链路集成（/api/attach + Hello 核销 + auth_failed 统一口径）Summary

**POST /api/attach 整站 Basic 换一次性 ticket、WS Hello 首帧核销升档（D-11 绑定 mode）、失败全形态统一 Error{auth_failed}+1008 无 oracle、per-IP 指数退避 429+Retry-After、双端点 Origin 白名单——九个黑盒集成测试 -race 全绿，既有套件零漂移**

## Performance

- **Duration:** 52 min
- **Started:** 2026-08-17T07:55:48Z
- **Completed:** 2026-08-17T08:48:39Z
- **Tasks:** 3
- **Files modified:** 7（1 新建 + 6 修改）

## Accomplishments

- 认证主链路端到端打通（tracer 验收）：无凭据 401 → 错凭据 401（body 逐字节同文）→ 正确凭据 200+no-store 取 ticket → Hello 核销 → Welcome{mode:"rw"} → 正常关闭 exitf(0)（TestAttachFlow）
- /api/attach 守卫链完整落地：非 POST 405+Allow:POST → Origin 403 不回显 → 节流 429+Retry-After（ceil 秒，retryAfter 只读访问器供数）→ Basic 401 同文 challenge（RFC 7617）→ 签发 200（MaxBytesReader 1KiB/413、Cache-Control: no-store、22 字符 ticket）
- D-08 统一计数器行为级反证：HTTP 侧凭据失败编排后合法 ticket 被 WS 侧闸住（TestThrottleHelloSharedCounter）；节流命中不核销不 recordFail 不延长窗口
- SEC-01 日志红线获运行时行为证据：os.Pipe 捕获完整失败轮（401+429+auth_failed），四类禁出串断言 + auth_failed/throttled 正向对照防假绿（TestLogRedaction）
- SEC-04 双端点执行：/ws 守卫区 ⓪ + AcceptOptions.OriginPatterns 二次校验、/api/attach originMiddleware（邪恶源/null 403、同源/白名单/无 Origin 放行，8 子断言）
- 无认证模式零漂移：/api/attach 显式 404、Hello 无 ticket 直连；`go test -race -count=1 ./...` 全量绿色反证
- 顺手项完成：server.go 三处「Phase 3 SEC-07」注释校正为「Phase 7 SEC-07」

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): server.go 集成——/api/attach 端点 + Hello 核销 + auth_failed 统一口径** - `53e30d5` (feat)
2. **Task 2: 端点守卫链 / ticket 过期 / 日志红线集成测试** - `ad1160e` (test，含 Rule 1 405 fallback 修复)
3. **Task 3: 429 闸 / D-08 共享计数器 / 双端点 Origin 集成测试** - `131d710` (test)

**Plan metadata:** 见最终 docs 提交（本文件之后）

## Files Created/Modified

- `internal/server/auth_e2e_test.go`（新建，黑盒 package server_test）- 九个集成测试：TestAttachFlow/TestTicketInvalid/TestNoAuthMode/TestAttachEndpoint/TestTicketExpiry/TestLogRedaction/TestThrottleHTTP/TestThrottleHelloSharedCounter/TestOriginEndpoints + dialHelloTicketWantAuthFailed 负例 helper；文件头注释列九测与需求映射
- `internal/server/server.go` - Options 六新字段；Server 装配 credentials/origins/originList/tickets/throttle/tlsOn；Handler() 重装配（securityHeaders 最外层 + 整站 Basic + /api/attach 链 + 显式 405 fallback）；attachHandler；Attach ⓪ Origin + OriginPatterns；握手段 checkTicket 核销分支（version 后升档前）；checkTicket 方法；logEvent 提为包级；三处注释校正
- `internal/server/auth.go` - basicAuth 中间件（429 闸 + 401 同文 + D-08 钩子）
- `internal/server/origin.go` - originMiddleware（/api/attach Origin 闸）
- `internal/server/throttle.go` - retryAfter 只读访问器（429 Retry-After 数据源）
- `internal/server/throttle_test.go` - TestThrottleStore 末尾 retryAfter 断言段
- `internal/server/e2e_test.go` - dialHelloTicket/attachURL/postAttach helper（dialHello 本体零改动）

## Decisions Made

- **logEvent 提为包级函数**：plan 指定的 `basicAuth(next, creds, th)` 三参自由函数签名需要调用日志出口；logEvent 本无 Server 状态依赖，提为包级使 HTTP/WS 两层共用唯一出口（红线纪律不变）。HTTP 层事件 code 复用 HTTP 状态码值（`websocket.StatusCode(http.StatusTooManyRequests)` 等，PATTERNS Shared Patterns 裁决形态）。既有 8 处调用点机械适配
- **originMiddleware 落位 origin.go**：与 originAllowed 内聚同文件；plan step 5 显式命名该中间件，files_modified 清单未列 origin.go 属小缺口（功能为 plan 内命名组件，非新增范围）
- **captureStderr 复用既有 helper**：limits_test.go 已有 os.Pipe 同形态 captureStderr（返回恢复函数形态），plan 字面「复制 main_test.go captureFd 形态」调整为复用——同包重声明编译冲突，且恢复函数形态自带幂等 defer 兜底更优
- **TestOriginEndpoints 单实例收口**：plan 字面「每 WS 场景独立服务器实例收口 waitExit(0)」与场景设计矛盾——八场景全为 Accept 前 HTTP 层拒绝（零 attach），无终结路径触发 exitf，waitExit 结构性不可达；改为全场景共用单实例并注释钉死理由（纯拒绝场景无单会话约束），≥7 子断言验收满足

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ServeMux 方法模式内建 405 被 "/" 子树吞掉，补显式同文 fallback**
- **Found during:** Task 2（TestAttachEndpoint a) 首跑：GET /api/attach 返回 401 而非 405）
- **Issue:** plan/RESEARCH 假设 `mux.Handle("POST /api/attach", ...)` 白拿 405+Allow 头；但 GOROOT server.go:2699-2710 的内建 405 回退仅在 `n==nil`（无任何模式匹配）时触发——"/" 子树模式匹配一切路径，GET /api/attach 被路由进 basicAuth(wh) 返回 401，方法模式 405 结构性被吞
- **Fix:** 显式注册 `mux.HandleFunc("/api/attach", ...)` path-only 模式：非 POST 一律 405 + `Allow: POST` + `http.StatusText(405)` body——与 mux 内建回退逐字同文；"POST /api/attach" 方法模式更具体，POST 仍走完整守卫链。Go mux 允许两模式共存（方法限定更具体不冲突）
- **Files modified:** internal/server/server.go（Handler() 装配 + 注释钉死 GOROOT 行号依据）
- **Verification:** TestAttachEndpoint a) 断言 405 + Allow 含 POST 通过；全量 -race 绿色
- **Committed in:** `ad1160e`（Task 2 提交的一部分）

---

**Total deviations:** 1 auto-fixed（Rule 1 bug）
**Impact on plan:** 修复为 plan 验收标准（405+Allow）成立的必要前提；拒绝形态与 mux 内建回退同文，无 wire 形态漂移，无范围蔓延。另有三项实现形态调整（logEvent 包级化 / originMiddleware 落位 / captureStderr 复用与 Origin 测试收口形态）已在 Decisions Made 记录，均属 plan 字面与实现约束的调和，非行为偏差。

## TDD Gate 说明

Task 2/3 标记 `tdd="true"`，但两任务均为纯集成测试追加（files 仅 auth_e2e_test.go），被测实现已由 Task 1（tracer）落地——RED 阶段结构性不适用（测试首跑即绿为预期，非「意外通过的 RED」）。按 test 类型原子提交。Plan 级 type=execute 且 config tdd_mode=false，无 plan 级 TDD 门禁适用。

## Issues Encountered

- TestAttachEndpoint a) 首跑 401≠405 → 即上述 Rule 1 偏差，定位 GOROOT 源码后修复
- captureStderr 初版与 limits_test.go 重声明编译冲突 → 复用既有 helper（见 Decisions）

## Authentication Gates

None - 无外部服务认证需求。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 03-04（main 装配）全部服务端接口就绪：`server.Options{Credentials, Origins, TLS, TicketTTL, ThrottleBase, ThrottleCap}` 六个注入点已定（Credentials 经 ParseCredential、Origins 经 NormalizeOrigin 构造；TLS 仅驱动 HSTS）；Handler() 已按认证模式分岔
- 03-05（前端消费）接口就绪：`POST /api/attach` 200 `{"ticket":...}`/401/404/429+Retry-After 探测语义、Hello ticket 字段、auth_failed 机器码（自动重取重试一次的分派锚点）
- 03-06（UAT）锚点就绪：完整链路/重放拒绝（同 ticket 二次 Hello）/401/429 场景的服务端行为已全部自动化锁定
- 关注点：无

---
*Phase: 03-auth*
*Completed: 2026-08-17*

## Self-Check: PASSED

- 文件：7 改动文件 + SUMMARY.md 全部 FOUND（auth_e2e_test.go 含 9 个 Test 函数）
- 提交：`53e30d5` / `ad1160e` / `131d710` 全部 FOUND；三次提交 post-deletion 检查均无意外删除
- 关键符号：retryAfter（throttle.go:99）、basicAuth（auth.go:85）、originMiddleware（origin.go:85）、attachHandler（server.go:206）、checkTicket（server.go:488）全部就位
- 验证：`go test -race -count=1 ./...` 全绿（9 新集成测试 + 既有全部零改动）；`go build ./... && go vet ./...` 退出 0
