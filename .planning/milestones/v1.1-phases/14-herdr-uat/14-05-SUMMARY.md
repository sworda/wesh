---
phase: 14-herdr-uat
plan: 05
subsystem: testing
tags: [go-test, dual-mode, mode-agnostic, handshake, keepalive, auth, origin, throttle, tickets, race-detector, t-run]

requires:
  - phase: 14-herdr-uat
    provides: 14-01 newTestServer 小族四形态装配点（普通/tracked/handle/sess）+ startPerClientServerTrackedWithSpawn 姊妹变体——本 plan 七文件改造的统一 mode 参数化入口
  - phase: 12-per-client
    provides: D-06 per-client RESIZE 直通语义（ro/rw 同形，server.go RESIZE case 两分支）+ TestPerClientResizePassthroughRO 白盒承载
  - phase: 03-auth
    provides: 认证传输面测试组（auth_e2e 九测 + auth/origin/throttle/tickets 纯函数白盒四文件）
provides:
  - mode-agnostic 前半批收口：协议守卫 10 测（handshake 7 + keepalive 3）+ 认证面 9 测（auth_e2e）双模式同断言双跑——38 个 mode= 子测试 -race 逐名绿
  - 纯函数面形态判定登记：auth/origin/throttle/tickets 四文件 6 测保持单跑（无装配可参数化，注释载明 + SUMMARY 清单）
  - auth_e2e_test.go:403 在册特殊调用点收编 newTrackedTestServer（tracked 形态第二个消费方，TestOversize1009 后第二证）
  - startTestServer 兼容包装收编删除（14-02/14-04 收编闭环第三例——改造后全仓零引用）
affects: [14-06, 14-12]

actuals:
  tokens: 25128   # 100510 chars / 4 over 8019727..HEAD（8 文件：七 plan 文件 + e2e_test.go -6 行）
  tasks: 2
  commits: 2

tech-stack:
  added: []   # 零新依赖（T-14-SC 红线；零装包任务 legitimacy 门不触发）
  patterns:
    - "mode-agnostic 同断言双跑——两列执行同一断言体、期望值逐字一致（grep -oE 断言字面量全集 diff 为空的机器自审形态）"
    - "零值/Writable:false Options 的 mutate 显式回写（小族基线 {Writable:true} 下，TestNoAuthMode/TestReadOnly* 经 mutate 覆写回 false——期望值零改写）"
    - "语义分叉面按断言分叉表处理（PITFALLS P11 红线辨析：非「两模式都接受」的削弱，per-client 列断言本模式真值全强度）"

key-files:
  created: []
  modified:
    - internal/server/handshake_test.go
    - internal/server/keepalive_test.go
    - internal/server/auth_e2e_test.go
    - internal/server/auth_test.go
    - internal/server/origin_test.go
    - internal/server/throttle_test.go
    - internal/server/tickets_test.go
    - internal/server/e2e_test.go   # Rule 3 枚举缺口：startTestServer 零引用删除（-6 行）

key-decisions:
  - "TestReadOnlyAllowsResize 是 handshake 行的唯一分叉面：ro 运行期 RESIZE 处置两模式真值相反（shared = D-09 第二闸忽略 / per-client = D-06 直通，server.go:1264-1287 文档化分歧）——按 14-01 断言分叉表落地：shared 列 stty#2 \"44 111\" 原断言逐字，per-client 列断言直通真值 \"50 120\"（比忽略更强的可证伪面）；exitf 面同 TestExitFrameSignal 先例分叉（waitExit(0) / assertNoExit）"
  - "TestNoAuthMode 原零值 server.Options（Writable false）经 mutate 显式回写 o.Writable = false——小族基线 {Writable: true} 不改期望值（Welcome mode==\"ro\" 双列同值）；TestReadOnlyDropsInput/TestReadOnlyAllowsResize 同款"
  - "auth/origin/throttle/tickets 四文件 6 测形态判定为纯函数白盒单跑（ParseCredential/matchCredential/NormalizeOrigin/originAllowed/throttleStore/ticketStore 无 server 装配面）——非覆盖缺口，传输面由 auth_e2e 九测双跑承载"
  - "TestLogRedaction :403 在册调用点收编 newTrackedTestServer——stderr 捕获同步边 waitHandlers 两列同构，SEC-01 红线四禁出串断言双跑两列同值零缩水"
  - "TestPingDisabled mutate 显式 PingInterval=0（小族基线即零值，显式写出锁定「0 = 禁用」语义防基线漂移时静默变质）"

patterns-established:
  - "Pattern 9: 零值 Options 测试的 mutate 显式回写——小族统一基线后，原零值/只读装配经 mutate 覆写表达，期望值零改写（TestPreHelloReadLimit 14-01 注记的不可观测形态 vs 本批可观测形态的完整解法）"
  - "断言字面量全集 diff 自审：git show HEAD:file | grep -oE '(Fatalf|Errorf|Fatal)\\(\"[^\"]*\"' 新旧排序比对——期望值零改写的机器可验证证据（本批抓出一处消息字面漂移并修复）"

requirements-completed: [PC-13]   # 共享 ID 门：herdr E2E 证据未齐（14-08 已过协议层，pw 观感层归 14-09），勾选归 phase 末收口 plan（14-01/02/04 先例延续）

coverage:
  - id: D1
    description: "协议守卫批双模式同断言双跑（handshake 7 测 + keepalive 3 测经 newTestServer，两列同一断言体期望值逐字一致；TestReadOnlyAllowsResize 唯一分叉面按断言分叉表处理）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "command: go test -race -count=1 -v -run 'TestHalfOpenPerIP429|TestSubprotocolRequired|TestHelloTimeout|TestPrematureFrame|TestVersionMismatch|TestReadOnly|TestPingKeepalive|TestPongTimeout|TestPingDisabled' ./internal/server/ → 20 个 mode= 子测试全 PASS"
        status: pass
      - kind: unit
        ref: "git show HEAD:handshake_test.go 断言字面量全集 diff：仅 second stty 分叉面一处（shared 值字面在场）；keepalive_test.go 零差异"
        status: pass
    human_judgment: false
  - id: D2
    description: "认证面批双模式同断言双跑（auth_e2e 9 测：8 处 newTestServer + :403 经 newTrackedTestServer；auth/origin/throttle/tickets 纯函数 6 测单跑判定登记）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "command: go test -race -count=1 -v -run 'TestAttachFlow|TestTicketInvalid|TestNoAuthMode|TestAttachEndpoint|TestTicketExpiry|TestLogRedaction|TestThrottle|TestOriginEndpoints' ./internal/server/ → 18 个 mode= 子测试全 PASS + 纯函数 6 测绿"
        status: pass
      - kind: unit
        ref: "auth_e2e_test.go 断言字面量全集 diff 零差异（全字符串字面量唯一增量 = mode=shared/mode=per-client 结构标签）"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-06
status: complete
---

# Phase 14 Plan 05: mode-agnostic 协议守卫 + 认证批双模式同断言双跑 Summary

**协议守卫 10 测 + 认证面 9 测共 38 个 mode= 子测试双跑落地（两列期望值逐字一致机器自审），纯函数 6 测形态判定单跑登记，auth_e2e :403 特殊调用点收编 newTrackedTestServer，startTestServer 兼容包装零引用收编删除——mode-agnostic 前半批（D-01）收口**

## Performance

- **Duration:** 25 min（2026-09-06T15:54:33Z → 16:19:45Z）
- **Tasks:** 2
- **Files modified:** 8（七 plan 文件 + e2e_test.go -6 行）

## Accomplishments

- **协议守卫批（Task 1）**：handshake 七测（半开 429/子协议双闸/5s 超时/抢跑/version_mismatch/ro 门控两测）+ keepalive 三测（保活/pong 超时/禁用）经 newTestServer 小族 t.Run 双跑——子协议/半开/超时/抢跑全部发生在升档分岔之前，pinger 是每连接 goroutine，与进程模型零耦合；10 测 20 个 mode= 子测试 -race 逐名绿
- **认证面批（Task 2）**：auth_e2e 九测（Basic→ticket→Hello 主链路/非法 ticket/无认证 404/端点守卫链/TTL 过期/日志红线/HTTP 节流生命周期/共享计数器反证/双端点 Origin）双跑——Basic/节流/ticket 核销/Origin 闸全部位于升档分岔之前的传输面，per-client 列 spawn 语义不触达任何断言点；TestLogRedaction 的 stderr 捕获同步边（在册 :403 调用点）收编 newTrackedTestServer，四禁出串红线断言两列同值零缩水
- **期望值零改写机器自审**：git show HEAD 断言字面量全集（grep -oE 排序 diff）——keepalive 零差异；handshake 唯一差异 = TestReadOnlyAllowsResize 分叉面（shared 值 "44"/"111" 字面在场 + per-client 真值 "50"/"120"）；auth_e2e 零差异（自审抓出一处消息字面漂移并修复，见 Deviations #2）。全字符串字面量唯一增量 = mode= 结构标签
- **CI 时长增量登记（T-14-10）**：本批 19 测双模式目标批实测 18.2s（改造前单模式约 9s），增量约 +9s；internal/server 全量 -race 三轮实测 152.5s/155.8s/156.9s（-v 轮）——14-12 收口闸 CI 时长预算知悉项续登
- **验证矩阵**：全包 178 个 mode= 子测试（含 14-01..04 已改造批）-race 零 FAIL；纯函数 6 测绿；go vet 与 GOROOT gofmt（go1.26.3）零输出；两任务提交后 git 零未跟踪文件、零文件级删除

## Task Commits

Each task was committed atomically:

1. **Task 1: 协议守卫批双模式同断言双跑——handshake 7 测 + keepalive 3 测** - `ae387eb` (test)
2. **Task 2: 认证面批双模式同断言双跑——auth_e2e 9 测 + 纯函数面单跑判定登记** - `84e627b` (test)

## Files Created/Modified

- `internal/server/handshake_test.go` - 七测双跑 + TestReadOnlyAllowsResize 断言分叉表（ro RESIZE：shared D-09 忽略逐字 / per-client D-06 直通真值 + exitf 分叉）
- `internal/server/keepalive_test.go` - 三测双跑（pinger 每连接同件同参数，mode-agnostic）
- `internal/server/auth_e2e_test.go` - 九测双跑 + :403 收编 newTrackedTestServer + TestNoAuthMode mutate 显式 Writable:false
- `internal/server/auth_test.go` - 纯函数单跑判定注释（TestParseCredential/TestCredentialMatch）
- `internal/server/origin_test.go` - 纯函数单跑判定注释（TestNormalizeOrigin/TestOriginAllowed）
- `internal/server/throttle_test.go` - 纯函数单跑判定注释（TestThrottleStore）
- `internal/server/tickets_test.go` - 纯函数单跑判定注释（TestTicketStore）
- `internal/server/e2e_test.go` - startTestServer 兼容包装删除（-6 行，Rule 3 枚举缺口 WINDOWS #45）

## 纯函数保持单跑清单（形态判定，非覆盖缺口）

| 文件 | 测试 | 判定依据 | 传输面承载 |
|------|------|----------|------------|
| auth_test.go | TestParseCredential、TestCredentialMatch | ParseCredential/matchCredential 纯函数白盒（package server），无 server 装配面 | auth_e2e 九测双跑（凭据经 Options.Credentials 注入全链） |
| origin_test.go | TestNormalizeOrigin、TestOriginAllowed | NormalizeOrigin/originAllowed 纯函数（httptest 构造请求），无装配面 | TestOriginEndpoints 双跑（双端点 Origin 白名单执行） |
| throttle_test.go | TestThrottleStore | throttleStore 经 now 手工注入推进，无装配面 | TestThrottleHTTP / TestThrottleHelloSharedCounter 双跑 |
| tickets_test.go | TestTicketStore | ticketStore 经 now 手工注入推进，无装配面 | TestAttachFlow / TestTicketInvalid / TestTicketExpiry 双跑 |

## Decisions Made

- **TestReadOnlyAllowsResize 分叉面判定**：plan truth 断言 handshake 行 10 测全部「两列同值零改写」，但 ro 运行期 RESIZE 在 server.go RESIZE case 两分支真值相反（shared 忽略 / per-client 直通——12-02 D-06 文档化产品语义，phase12.mjs S3 与 TestPerClientResizePassthroughRO 已锁定 per-client 侧）。按 14-01 断言分叉表落地：两列各断言本模式真值全强度（per-client 列断言 resize 确实到达 PTY——比「忽略」更强的可证伪面），Hello 首尺寸/stty#1/close 1000 等其余断言面两列同值逐字
- **TestNoAuthMode 的 Writable 回写**：原装配零值 Options（Writable false → Welcome mode "ro" 是可观测断言），小族基线 {Writable: true} 下必须经 mutate 覆写回 false——与 TestPreHelloReadLimit（14-01，Writable 不可观测故无需回写）形成对照的「可观测零值」完整解法（Pattern 9）
- **TestLogRedaction per-client 列零 spawn 论证**：(0) 对照 ticket 不使用 + (a)(b) HTTP 失败 + (c) auth_failed 关闭——全部发生在 upgradePerClient 之前，两列事件流同构（throttled/auth_failed 各自 ≥1），红线断言零缩水
- **TestAttachFlow per-client effMode 同值论证**：ticket 绑定 rw（D-11 全局 writable）→ per-client 单行门 `s.writable && ticketMode==rw` 与 shared decideModeLocked 在本测单客户端形态下同值 "rw"——Welcome mode 断言两列同值

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 4→分叉表先例裁决] TestReadOnlyAllowsResize 期望值无法两列同值**
- **Found during:** Task 1 规划期代码核对（执行前置 read_first 阶段）
- **Issue:** plan must_haves 断言 handshake 行 10 测「期望值两列同值且与改造前逐字一致（零改写）」，但 ro 运行期 RESIZE 处置是 12-02 D-06 文档化的两模式语义分歧（server.go RESIZE case：per-client 分支 ro/rw 同形直通，D-09 第二闸不生效——ttyd parity），per-client 列强行沿用「44 111」期望即断言错误语义（直通已生效却断言忽略）
- **Fix:** 按 14-01 断言分叉表形态落地（PITFALLS :264 认可形态 + TestExitFrameSignal 先例）：shared 列原断言逐字（stty#2 "44 111" + waitExit(0)），per-client 列断言直通真值（stty#2 "50 120" + assertNoExit）；其余断言面两列同值。非「两模式都接受」的削弱形态（P11 红线辨析）——per-client 列是更强的可证伪断言
- **Files modified:** internal/server/handshake_test.go
- **Verification:** 双模式子测试逐名 PASS；断言字面量全集 diff 确认唯一差异即本分叉面；WINDOWS #44 登记
- **Committed in:** ae387eb（Task 1 提交内）

**2. [Rule 1 - Bug] 自审抓出 TestAttachEndpoint 消息字面漂移并修复**
- **Found during:** Task 2 期望值零改写机器自审（断言字面量全集 diff）
- **Issue:** 改写时误将 TestAttachFlow 的 Cache-Control 断言消息（带 CJK 后缀「（ticket 不可落缓存）」）复制到 TestAttachEndpoint——HEAD:322 原文为无后缀形态 `want no-store`，消息字面漂移违反零改写红线（虽非期望值本体，T-14-09 证据链要求逐字）
- **Fix:** 恢复 HEAD 逐字原文；修复后断言字面量全集 diff 零差异
- **Files modified:** internal/server/auth_e2e_test.go
- **Verification:** 修复后 auth_e2e 字面量 diff 零差异 + 认证面批 -race 重跑绿 + 全量回归绿
- **Committed in:** 84e627b（Task 2 提交内）

**3. [Rule 3 - 枚举缺口] startTestServer 兼容包装删除**
- **Found during:** Task 1（handshake 三处调用点改造后全仓零引用）
- **Issue:** plan files_modified 未列 e2e_test.go；不删则留死代码 helper（「保持既有五个 Dial 测试 echo 语义」的消费方已全部改造）
- **Fix:** 按 14-02 startShutdownServerWith / 14-04 startResizeServer 收编闭环先例删除（第三例）；WINDOWS #45 登记
- **Files modified:** internal/server/e2e_test.go（-6 行）
- **Verification:** go vet 零输出（无 unused 报错面）+ 全量 -race 绿
- **Committed in:** ae387eb（Task 1 提交内）

---

**Total deviations:** 3（1 分叉表先例裁决 + 1 bug 自审修复 + 1 Rule 3 枚举缺口）
**Impact on plan:** 全部为测试形态/字面保真问题，无范围蔓延；分叉面是 plan 文本对代码现实的误设（D-06 语义先于本 plan 存在），以既有分叉表 pattern 承载。

## Issues Encountered

None——CI 时长增量已实测登记（本批约 +9s，见 Accomplishments）。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- mode-agnostic 前半批（D-01）收口：14-01 limits 五测 + 本批 19 测 = 24 测双跑落地；14-06（proxy/basepath/customindex/events/log）为后半批——proxy_e2e 的 :46/:218/:281 三处 tracked 在册调用点是 newTrackedTestServer 的下一批消费方
- startTrackedServerWith/startTestServerWith 仍被 customindex/basepath/log/events/load/proxy_e2e 持有（14-06 改造范围），收编闭环由该 plan 延续
- PC-13 勾选按 shared-ID 门归 phase 末收口 plan（14-01/02/04 先例延续；14-08 已过协议层，pw 观感层归 14-09）
- 14-12 收口闸 diff 白名单应含：八文件（七 plan 文件 + e2e_test.go -6 行）+ WINDOWS #44/#45 两条台账

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-06*

## Self-Check: PASSED

- 8 个 key-files 全部在盘（七 plan 文件修改 + e2e_test.go）
- 2 个任务提交在库（ae387eb / 84e627b）
- frontmatter `status: complete` 在场
- 38 个本批 mode= 子测试 -race 逐名 PASS（全包 178 个零 FAIL）；全量 -race 三轮绿；go vet + GOROOT gofmt 零输出
- 断言字面量机器自审：keepalive/auth_e2e 零差异，handshake 唯一差异为已登记分叉面
- WINDOWS 台账 #44/#45 已登记（.planning/WINDOWS.md）
