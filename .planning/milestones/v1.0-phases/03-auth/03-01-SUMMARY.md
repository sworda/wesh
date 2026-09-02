---
phase: 03-auth
plan: 01
subsystem: auth
tags: [websocket, one-shot-ticket, exponential-backoff, crypto-rand, base64url, wesh.v1, go, tdd]

requires:
  - phase: 02-protocol
    provides: proto.go 契约基线（Error code 表/HelloPayload/D-02 未知字段忽略纪律）、halfOpenCounter mu+map+删 key 模式（server.go:132-162）、Options 零值兜底先例（HelloTimeout）
provides:
  - proto 契约增量：ErrAuthFailed = "auth_failed"（D-10 统一口径，发 Error 帧 + 1008，P2 deferred 挂账兑现）+ HelloPayload.Ticket（json `ticket,omitempty`，Hello 载荷为 ticket 唯一传输通道）
  - ticketStore（internal/server/tickets.go）：crypto/rand 16B → base64url 22 字符签发、原子查删核销（单次使用）、60s TTL、mode 绑定（D-11）、签发惰性清扫（无 janitor goroutine）
  - throttleStore（internal/server/throttle.go）：per-IP 指数退避 1s×2 封顶 30s（级数 1/2/4/8/16/30/30）、窗口内命中不延长、成功清零（D-08）、15min 惰性过期
  - 单元测试锁定：TestDecodeHello ticket 两行 + TestProtocolConstants ErrAuthFailed 逐字锁 + TestTicketStore 六组 + TestThrottleStore 七组（全部 now 注入零 sleep，-race 绿）
affects: [03-02（auth.go/origin.go/headers.go 同批纯组件）, 03-03（集成 tracer 直接消费两 store 与契约）, 03-05（前端按 auth_failed 分派重试）, 03-06（收口验证）, verify-work]

actuals:
  tokens: 5404
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "mu+map+惰性清理最小存储形态：核销即删/签发顺手清扫/15min 惰性过期——无常驻 janitor goroutine（零新 exitf 分支纪律），halfOpenCounter 同构"
    - "now 参数注入时序测试：全部时间语义经 now 手工推进断言，零真实 sleep（TestTicketStore/TestThrottleStore）"
    - "同目录双包测试：内部类型白盒 package server（tickets_test/throttle_test）与既有黑盒 package server_test 共存"
    - "ROADMAP 锚点可测化：爆破 100 次累计等待 ≥47min 由级数和（1+2+4+8+16+95×30s=2881s）在单元测试内直接断言"

key-files:
  created:
    - internal/server/tickets.go
    - internal/server/tickets_test.go
    - internal/server/throttle.go
    - internal/server/throttle_test.go
  modified:
    - internal/proto/proto.go
    - internal/proto/proto_test.go

key-decisions:
  - "TestDecodeHello 表加 checkTicket bool 闸：plan「表加 wantTicket 字段 + 既有行补零值 + 禁止改 unknown-fields 行」三约束与全表统一 Ticket 断言不可同时成立（unknown 行载荷 ticket:\"secret\" 加字段后解码入 Ticket，零值断言必红）——闸化后仅两个新行断言 Ticket，unknown 行零值补位、D-02 回归锁（attach 未知字段忽略）逐字不动"
  - "ErrAuthFailed 入 TestProtocolConstants 逐字锁定（snake_case 形状 + 值 == \"auth_failed\"）——该文件既定职责即锁前后端公开契约常量（T-02-01 缓解形态），D-10 costly 可逆级常量入锁属正确性要求"

patterns-established:
  - "一次性 ticket 存储：crypto/rand 直生独立 secret（不从凭据派生，C6）+ map 原子查删（查即删单次使用）+ 重放/过期/非法同归 false（D-10 无 oracle）"
  - "per-IP 节流计数器：失败即拒不 sleep 不挂 goroutine；allow 只读不延长窗口；recordSuccess delete 清零；位移起级 base<<min(fails-1,5) 超 cap 截断"

requirements-completed: [SEC-02, SEC-03]

coverage:
  - id: D1
    description: "proto 契约增量：ErrAuthFailed = \"auth_failed\" 入 Error code 表（注释写明 D-10 统一口径语义、P2 deferred 挂账兑现表述更新）+ HelloPayload.Ticket 可选字段（`ticket,omitempty`，未知字段忽略纪律不受影响，既有五行测试行为不变）"
    requirement: SEC-02
    verification:
      - kind: unit
        ref: "internal/proto/proto_test.go#TestDecodeHello（ticket round-trip / ticket omitted 两行）+ #TestProtocolConstants（ErrAuthFailed 逐字与 snake_case 形状）"
        status: pass
    human_judgment: false
  - id: D2
    description: "ticketStore 一次性 ticket 签发/核销：22 字符 base64url（16B crypto/rand）、两次签发互异、mode/exp 精确登记、单次使用查即删、重放/过期/未签发/畸形/空串同归 false、签发惰性清扫（map 长度回落）、零值 ttl 兜底 60s"
    requirement: SEC-02
    verification:
      - kind: unit
        ref: "internal/server/tickets_test.go#TestTicketStore（六组子测试，now 注入零 sleep，-race PASS）"
        status: pass
    human_judgment: false
  - id: D3
    description: "throttleStore per-IP 指数退避：未知 IP 放行、首败 notBefore 恰好 now+base、窗口内命中不延长、级数 1/2/4/8/16/30/30 逐项、recordSuccess delete 清零后从 base 重启、lastSeen 超 15min 惰性重置、零值兜底 1s/30s、爆破 100 次累计 ≥47min（ROADMAP 准则 2 锚点）"
    requirement: SEC-03
    verification:
      - kind: unit
        ref: "internal/server/throttle_test.go#TestThrottleStore（七组子测试，now 注入零 sleep，-race PASS）"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-08-17
status: complete
---

# Phase 3 Plan 01: 协议契约增量 + ticket/节流两纯内存存储 Summary

**auth_failed 机器串与 ticket 字段入 wesh.v1 契约（P2 deferred 挂账兑现），ticketStore（crypto/rand 22 字符一次性票、原子查删、60s TTL、mode 绑定）与 throttleStore（per-IP 指数退避 1s×2 封顶 30s、成功清零、15min 惰性过期）以纯组件形态经 13 组单元测试钉死语义，03-03 集成 tracer 的全部前置件就绪**

## Performance

- **Duration:** 18min
- **Started:** 2026-08-17T06:59:02Z
- **Completed:** 2026-08-17T07:16:52Z
- **Tasks:** 3
- **Files modified:** 6（2 修改 + 4 新建）

## Accomplishments

- proto.go 契约增量落地：`ErrAuthFailed = "auth_failed"`（D-10 核销失败统一口径——过期/非法/重放/节流同口径无 oracle，发 Error 帧 + 1008，D-06 正常客户端可见码）+ `HelloPayload.Ticket`（`json:"ticket,omitempty"`，Hello JSON 载荷为 ticket 唯一传输通道——ARCHITECTURE §2.8 锁定不走 query/子协议头）；两处注释预告位（Error 表 deferred 表述、HelloPayload「Phase 3 加 ticket」）同步改为已兑现表述
- ticketStore 签发/核销语义钉死：crypto/rand 16 字节 → base64.RawURLEncoding 22 字符（与静态凭据独立 secret，C6）；核销原子查删（单次使用）；签发顺手机会性清扫过期项（Pitfall 4 惰性清理，无常驻 goroutine）；零值 ttl 兜底 60s（ROADMAP 锁定）
- throttleStore 退避语义钉死：D-09 标定参数 base=1s/cap=30s（OWASP 1s 翻倍锚点），位移级数 base<<min(fails-1,5) 超 cap 截断（实测级数 1/2/4/8/16/30/30）；窗口内 allow 只读不延长 notBefore；recordSuccess delete 清零（D-08）；lastSeen 超 15min 惰性重置；爆破 100 次累计等待 2881s ≥ 47min 锚点直接在测试中断言
- 零新增依赖（全 stdlib，T-03-SC accept 项不触发）；无 goroutine、无 sleep、无 time.Ticker——失败即拒不挂连接（prohibition 三条全守）

## Task Commits

TDD 三任务各一组 test→feat 提交：

1. **Task 1: proto.go 契约增量（ErrAuthFailed + HelloPayload.Ticket）** - `b826982` (test, RED) → `0173a7f` (feat, GREEN)
2. **Task 2: tickets.go 一次性 ticket 签发/核销存储** - `f6f5c9e` (test, RED) → `d1b3780` (feat, GREEN)
3. **Task 3: throttle.go per-IP 指数退避节流计数器** - `31fd8b2` (test, RED) → `babe38a` (feat, GREEN)

**Plan metadata:** 见尾部 docs 提交（docs(03-01): complete ...）

## Files Created/Modified

- `internal/proto/proto.go` - Error code 表 +ErrAuthFailed（D-10 语义注释）；HelloPayload +Ticket 字段；两处 deferred 注释改已兑现表述
- `internal/proto/proto_test.go` - TestDecodeHello 表 +wantTicket/checkTicket 字段与 ticket 两行；TestProtocolConstants +ErrAuthFailed 逐字与形状锁定
- `internal/server/tickets.go` - ticketStore/ticketEntry/newTicketStore/issue/redeem + defaultTicketTTL=60s
- `internal/server/tickets_test.go` - TestTicketStore 六组（package server 白盒）
- `internal/server/throttle.go` - throttleStore/throttleEntry/newThrottleStore/allow/recordFail/recordSuccess + defaultThrottleBase=1s/defaultThrottleCap=30s
- `internal/server/throttle_test.go` - TestThrottleStore 七组（package server 白盒）

## Decisions Made

- **checkTicket 闸化 ticket 断言**（偏差 1，Rule 3）：plan 字面三约束（表加 wantTicket + 既有行补零值 + 禁止改 unknown-fields 行）与全表统一 Ticket 断言冲突——unknown-fields 行载荷 `ticket:"secret"` 在 Ticket 字段落地后解码入 hp.Ticket="secret"，零值 wantTicket 统一断言必红。调和：加 `checkTicket bool` 闸，仅 "ticket round-trip"/"ticket omitted" 两新行断言 Ticket；既有五行两新字段均零值补位不参与断言——三约束逐字满足，unknown 行的 D-02 回归锁（attach 未知字段忽略 + Version/Cols/Rows 断言）逐字不动，与 read_first 预言「测试断言仍成立」一致
- **ErrAuthFailed 入 TestProtocolConstants 锁定**（偏差 2，Rule 2）：plan 只列 TestDecodeHello 改动，但该文件既定职责是「逐字锁定协议常量——手滑改码即协议破坏本测试即红」（T-02-01 缓解形态）；auth_failed 是 D-10 costly 可逆级前后端公开契约，值（"auth_failed"）与 snake_case 形状入锁属正确性要求

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TestDecodeHello 表 ticket 断言闸化（checkTicket bool）**
- **Found during:** Task 1（proto.go 契约增量）
- **Issue:** plan 要求「表结构加 wantTicket 字段，既有行补零值」且「禁止改动既有 unknown-fields 行」；但该行载荷含 `"ticket":"secret"`，Ticket 字段落地后解码入 hp.Ticket——若全表统一断言 `hp.Ticket == wantTicket`，该行（补零值）必红。三约束与统一断言不可同时成立
- **Fix:** 表结构加 `wantTicket string` + `checkTicket bool` 两字段；仅 checkTicket=true 的两个新行断言 Ticket；既有五行零值补位（wantTicket="" + checkTicket=false），unknown-fields 行载荷与 Version/Cols/Rows 断言逐字不动
- **Files modified:** internal/proto/proto_test.go
- **Verification:** `go test -count=1 -v -run TestDecodeHello ./internal/proto/` 七行全 PASS（unknown-fields 行行为不变）
- **Committed in:** `b826982`（Task 1 RED 提交）

**2. [Rule 2 - Missing Critical] ErrAuthFailed 入 TestProtocolConstants 逐字锁定**
- **Found during:** Task 1（proto.go 契约增量）
- **Issue:** plan 的测试改动只列 TestDecodeHello 两行；新公开契约常量 ErrAuthFailed（D-10，costly 可逆级）无任何逐字锁定——proto_test.go 既定纪律（T-02-01 缓解）要求协议常量手滑改码即红
- **Fix:** TestProtocolConstants 的 snake_case map 加 ErrAuthFailed 形状断言 + 显式值断言 `ErrAuthFailed == "auth_failed"`
- **Files modified:** internal/proto/proto_test.go
- **Verification:** `go test ./internal/proto/` 全 PASS
- **Committed in:** `b826982`（Task 1 RED 提交）

---

**Total deviations:** 2 auto-fixed（1 blocking 约束调和、1 missing critical 测试锁定）
**Impact on plan:** 两处均为测试层最小调和/补强，零实现偏离；plan 全部 must_haves truths 与三条 prohibition 逐字落地，无 scope creep。

## TDD Gate Compliance

三任务均为 tdd="true"，RED/GREEN 门齐整：

| Task | RED (test) | GREEN (feat) | REFACTOR |
|------|-----------|--------------|----------|
| 1 | `b826982` | `0173a7f` | 无需（增量最小） |
| 2 | `f6f5c9e` | `d1b3780` | 无需 |
| 3 | `31fd8b2` | `babe38a` | 无需 |

RED 均为构建失败形态（Go TDD 常态：引用未定义符号即红灯），GREEN 后全量 -race 复跑确认。

## Issues Encountered

- `/usr/bin/gofmt` 陈旧（解析 server.go:43 报语法错误）——按 STATE.md 既有决策（Phase 01-03）使用 GOROOT gofmt；tickets.go 一处行内注释对齐经 GOROOT gofmt -w 修正后 -l 全净（含在 `d1b3780` 提交）。非偏差，环境已知项沿用既定纪律

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 03-02（auth.go/origin.go/headers.go 纯组件批）可直接开工：throttleStore 的 `recordFail(clientIP(r))`/`recordSuccess` 消费面已就绪（RESEARCH Pattern 2 basicAuth 定稿代码直接装配）
- 03-03 集成 tracer 前置件全部就绪：`proto.ErrAuthFailed` + `HelloPayload.Ticket` 契约、`s.tickets.issue/redeem`、`s.throttle.allow/recordFail/recordSuccess` 语义已单元级锁定，集成 plan 只做接线不测语义
- 03-05 前端消费面就绪：auth_failed 机器串入契约（D-16 手工对齐纪律——proto.go 行 6 注释已指路 main.ts，03-05 按计划同步）
- 无阻塞；无新增威胁面（纯内存组件，plan threat_model 五条 mitigate 项全部由实现+测试覆盖）

## Self-Check: PASSED

- 文件存在性：internal/server/tickets.go ✓ / tickets_test.go ✓ / throttle.go ✓ / throttle_test.go ✓ / proto.go 与 proto_test.go 修改在树 ✓
- 提交存在性：`b826982` `0173a7f` `f6f5c9e` `d1b3780` `31fd8b2` `babe38a` 均在 git log ✓
- 全量验证：`go test -race -count=1 ./...` 五包全 ok ✓；`go build ./... && go vet ./...` exit 0 ✓；GOROOT gofmt -l internal/ 输出为空 ✓
- Known Stubs：无（stub 模式 grep 零命中；两 store 为完整实现）
- Threat Flags：无（未引入 plan threat_model 之外的安全相关面）

---
*Phase: 03-auth*
*Completed: 2026-08-17*
