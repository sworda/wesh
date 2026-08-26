---
phase: 07-deployment
plan: 03
subsystem: auth
tags: [auth-header, remote-user, x-forwarded-for, reverse-proxy, sso, audit-logging, log-injection-defense, throttle-key, tdd]

requires:
  - phase: 07-deployment/07-01
    provides: Options 生产直传字段先例（BasePath 注释形态）+ 07-01 六段式全绿基线
  - phase: 07-deployment/07-02
    provides: validateStartup 扩展落点（D-11 socket 早退——D-16 警告 socket 跳过同位逻辑）+ TestStartupMatrix 表行扩展形态
  - phase: 03-auth/03-03
    provides: basicAuth/logEvent/throttle 三消费点现状形态 + authRequiredBody 统一 401 面
  - phase: 04-terminal-ui
    provides: web/src/lib/title.ts C0/C1/DEL+128 sanitize 纪律（D-19 Go 移植对照源）
  - phase: 05-multi-client/05-01
    provides: startTrackedServerWith/captureStderr -race 同步纪律（stderr 捕获类测试唯一合法形态）
provides:
  - --auth-header CLI 公开契约（D-18 one-way）：可配反代用户头名，生产直传 Options.AuthHeader → Server.proxy 装配（信任闸 = AuthHeader 非空，D-20 零双轨）
  - proxy.go 提取层：sanitizeRemoteUser（D-19：C0/C1/DEL 剥离 + 128 rune 截断、多字节不碎、空结果即不出键）+ proxyInfo.clientIP/remote/remoteUser 三方法（XFF 链首 Cut+TrimSpace，非法/缺席回退 TCP 对端现状取值）
  - logEvent variadic 第四参 remote_user（非空追加 ` remote_user=<u>`；未配置/头缺席与现状逐字节一致）+ D-03 红线随新字段延伸注释（提取源只能是配置头名 HTTP 头，token/ticket 结构性不可能入参）
  - XFF 换键全链：basicAuth ip（throttle 键）与 401/429 remote、Attach ip（halfOpen/checkTicket 键）与 remote、issueTicketJSON 503——日志归因与节流计数同键不分叉
  - validateStartup D-16 暴露面警告行（--auth-header 非空 + bind 非 loopback + 无凭据，wesh: warning: 前缀，文案含 flag 名不含头值；socket 形态同 D-11 逻辑跳过）
  - 五新测试 + TestShareChannelRemoteUser 双通道断言 + D-03 运行时自净断言（share token/ticket 值永不出 stderr）
affects: [phase-07 后续 plans（07-06 配置文件 auth-header 键——RESEARCH Pattern 4 fileConfig.AuthHeader 已预留 / 07-07 UAT assertOutputClean 消费 remote_user 行 / 07-08 README 反代节与 SEC-07 需求文本修订）, phase-08 OPS-08 slog 结构化（remote_user 字段先行形态）, verify-work]

actuals:
  tokens: 17067
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "反代信任单一闸形态：proxyInfo{trust,userHeader} 零值 = 不信任（行为与现状逐字节一致），--auth-header 给定 = XFF 换键与 remote_user 提取共用总开关（D-20 零双轨）；提取全部在 HTTP 层 Accept 前完成（守卫区零 WS 资源纪律）"
    - "logEvent 可选第四字段 variadic 形态：既有调用点零改动编译通过 + 新消费点逐点显式传值；清洗在提取点单一写口完成，logEvent 不做二次清洗"
    - "attach 入口一次提取贯通形态：remote/remoteUser 在 Attach 入口各取一次，握手事件行、client 构造（注册表事件/pinger/exit_when_empty 家族）全部共享同值——share token 渠道与 Basic 渠道同经该提取点（双通道同口径的结构保证）"

key-files:
  created:
    - internal/server/proxy.go
    - internal/server/proxy_test.go
    - internal/server/proxy_e2e_test.go
  modified:
    - internal/server/server.go
    - internal/server/auth.go
    - internal/server/clients.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go

key-decisions:
  - "proxy 测试分文件（白盒 proxy_test.go / 黑盒 proxy_e2e_test.go）——Go 单文件单 package 约束使 plan『proxy_test.go 单文件四测试』字面不可达（白盒表驱动需 package server，stderr 捕获全链测需 server_test 的 startTrackedServerWith/captureStderr 同步纪律）；05-04 resize 两测试分文件先例第二次沿用，验证命令与测试命名逐字保持"
  - "remote_user 载体范围判定：attach 链路事件行全覆盖（握手拒绝/max_clients/message_too_big 两档）+ 注册客户端会话事件（slow_consumer/pong_timeout/exit_when_empty 家族）——must_have truth『attach 链路的 logEvent 行携 remote_user』字面要求与 behavior『clients.go 的会话事件（kick/pong_timeout 等）』的自然覆盖；未配置时全部与现状逐字节一致（variadic 空值不出键）"
  - "D-16 警告取合并形态：--auth-header 暴露面警告为主（可被直连伪造 + 确保不直接暴露 + 反代建议），--no-auth 裸奔语义随同行保持不丢（矩阵其余行语义不变的字面落地）；socket 形态经 D-11 早退自然跳过（unix socket 信任边界同 D-11）"
  - "Task 2 字面验收 grep（'clientIP(' 排除 proxy.go/test 后 ==0）与 Task 1 字面验收 grep（'s.proxy.clientIP\\|p.clientIP' ≥2）结构性自相矛盾——按意图修正执行：旧自由函数定义已删、调用零残留（修正版 grep==0，逐行核读 3 处匹配均为注释或 Task 1 要求的方法调用）"

patterns-established:
  - "proxyInfo 三方法取值语义分层：clientIP（per-IP 计数键——trust 时 XFF 链首，否则 SplitHostPort host）/ remote（logEvent remote 字段——trust 时同 clientIP，否则 RemoteAddr 原样 host:port）/ remoteUser（日志字段——trust 且头存在时 sanitize 值，否则空串不出键）；三方法各自独立表驱动锁定"
  - "D-03 红线运行时自净断言形态：测试收尾遍历 stderr 断言 share token 与一次性 ticket 值零出现（T-07-03c——与 UAT assertOutputClean 同族，Go 单测侧先行落地）"

requirements-completed: [SEC-07]

coverage:
  - id: D1
    description: "sanitizeRemoteUser 清洗纪律（C0/C1/DEL 逐 rune 剥离、128 rune 截断、多字节不碎、控制字符不占截断预算、空结果即空串）与 proxyInfo 三提取方法（trust 开/关 × XFF 链首/单值/空格/空首段/缺席 × 回退形态 × remoteUser 提取/多值头首值）"
    requirement: SEC-07
    verification:
      - kind: unit
        ref: "internal/server/proxy_test.go#TestSanitizeRemoteUser（15 子测 + UTF-8 有效性断言）+ #TestProxyClientIP（16 子测）"
        status: pass
    human_judgment: false
  - id: D2
    description: "attach 链路 logEvent 行携 sanitize 后 remote_user（alice/C1 剥离 carol）；不携头行与现状逐字节一致（不出 remote_user 键）；真实二进制冒烟：凭据模式 401 行含 remote=203.0.113.7 与 remote_user=alice"
    requirement: SEC-07
    verification:
      - kind: integration
        ref: "internal/server/proxy_e2e_test.go#TestRemoteUserLogging"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（/tmp 临时二进制，curl 携 X-Remote-User/XFF 观测 stderr 行；产物已清理）"
        status: pass
    human_judgment: false
  - id: D3
    description: "XFF 换键全链：trust 开启时 throttle per-IP 键换 XFF 链首（同 XFF 429、异 XFF 独立 401）；未配置时 XFF 完全忽略（异 XFF 共享 TCP 对端回退键 429）；logEvent remote 同步换键（冒烟 remote=203.0.113.7 vs 未配置 remote=127.0.0.1:port）"
    requirement: SEC-07
    verification:
      - kind: integration
        ref: "internal/server/proxy_e2e_test.go#TestXFFThrottleKey（双子测）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（配置/未配置两实例对照观测）"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-17 认证正交回归锁：伪造 X-Remote-User: root 头无凭据请求 /api/attach 收 401 照旧（WWW-Authenticate 挑战形态不变），伪造值只进日志不进认证判定；Basic/ticket/share token 三通道语义零改动（既有套件 -race 全绿）"
    requirement: SEC-07
    verification:
      - kind: integration
        ref: "internal/server/proxy_e2e_test.go#TestAuthHeaderNoAuthBypass"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（forged root 头 401 观测）+ go test -race -count=1 ./internal/server ./cmd/wesh 全绿"
        status: pass
    human_judgment: false
  - id: D5
    description: "D-16 暴露面警告矩阵：触发（非 loopback + 无凭据 + --no-auth + auth-header → wesh: warning: 前缀含 flag 名不含头值）、D-03 拒绝不削弱、两不触发（loopback/凭据+TLS）、socket 跳过五行；--auth-header flag parse 契约（TestParseArgs 命名字段扩展）"
    requirement: SEC-07
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix（D-16 五行）+ #TestParseArgs（auth-header 行）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（D-16 警告行与无 auth-header 对照行观测）"
        status: pass
    human_judgment: false
  - id: D6
    description: "share token 通道同口径：issueTicketJSON max_clients 503 在 Basic/token 双通道均携 remote_user；token 渠道 WS attach 经同一 Attach 入口提取点（稳态 message_too_big 携 dave）；D-03 红线运行时自净（share token/ticket 值零出现 stderr）；-race 全量回归绿"
    requirement: SEC-07
    verification:
      - kind: integration
        ref: "internal/server/proxy_e2e_test.go#TestShareChannelRemoteUser + go test -race -count=1 ./internal/server ./cmd/wesh（48.8s+1.0s 全绿）"
        status: pass
    human_judgment: false

duration: 30min
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 03: auth-header 透传与 X-Forwarded-For 信任闸 Summary

**--auth-header 全链落地：proxy.go 提取层（D-19 sanitize + proxyInfo 三方法）→ logEvent 可选第四字段 remote_user → XFF 与 auth-header 共用信任闸换 per-IP 键（throttle/halfOpen/logEvent remote 同键）→ D-16 暴露面启动警告；伪造头只记录不生效（D-17 正交回归锁），未配置时日志与计数语义与现状逐字节一致。**

## Performance

- **Duration:** 30 min
- **Started:** 2026-08-26T01:50:43Z
- **Completed:** 2026-08-26T02:21:08Z
- **Tasks:** 2/2
- **Files modified:** 8（新建 3：proxy.go + proxy_test.go + proxy_e2e_test.go；修改 5：server.go/auth.go/clients.go/main.go/main_test.go；合计 749 insertions / 60 deletions）

## Accomplishments

- `wesh --bind 127.0.0.1 --credential u:p --auth-header X-Remote-User -- bash` 真实二进制冒烟：curl 携 `X-Remote-User: alice` 与 `X-Forwarded-For: 203.0.113.7, 10.0.0.1` 的 401 事件行 = `remote=203.0.113.7 code=401 reason=auth_failed remote_user=alice`（XFF 链首换入 remote 字段 + sanitize 后头值第四键）；同形态不配置 --auth-header 时相同请求得 `remote=127.0.0.1:51002 ... reason=auth_failed`——无 remote_user 键、host:port 现状形态逐字节一致（D-20 单一信任闸）
- XFF 换键与节流计数同键不分叉：trust 开启时同一 XFF 连续失败触发 429 而另一 XFF 独立计数 401（throttle 键 = XFF 链首）；未配置时两不同 XFF 共享 TCP 对端回退键（直连客户端自设 XFF 零效果）；halfOpen/checkTicket/logEvent remote 全部走同一 proxyInfo.clientIP/remote 提取
- D-17 正交性回归锁成立：伪造 `X-Remote-User: root` 头无凭据请求 /api/attach 收 401 照旧（挑战形态不变，冒烟 + 单测双证）——auth-header 值只做记录不做任何认证决定，Basic/ticket/share token 三通道语义零改动（-race 全量 48.8s+1.0s 全绿）
- D-19 清洗纪律 Go 移植：C0/C1/DEL 逐 rune 剥离 + 128 rune 截断（多字节不碎、控制字符不占预算、空结果即不出键），15 子测表驱动锁定；C1 NEL 头值全链实证（`c\u0085arol` → `remote_user=carol`）
- D-16 暴露面警告落地：非 loopback + 无凭据 + --no-auth + --auth-header 时 stderr 醒目警告（含 --auth-header flag 名、不含任何头值；--no-auth 裸奔语义同行保持）；D-03 拒绝不被 auth-header 削弱、loopback/凭据+TLS 不触发、socket 形态同 D-11 逻辑跳过——TestStartupMatrix 五行锁定
- D-03 红线随新字段延伸并运行时自净：share token 通道与 Basic 通道同口径提取（issueTicketJSON 503 双通道断言 bob/carol、token 渠道 WS attach 稳态事件携 dave），测试收尾断言 share token 与一次性 ticket 值不出现在 stderr 任何事件行（T-07-03c）

## Task Commits

每个任务原子提交（Task 1 TDD 含 RED/GREEN 两提交）：

1. **Task 1 RED: 五新测试 + D-16 矩阵行失败测试** - `6461f0c` (test)
2. **Task 1 GREEN: proxy.go 提取层 + logEvent 第四字段 + --auth-header + D-16** - `141fd3f` (feat)
3. **Task 2: share 通道同口径覆盖 + -race 全量回归** - `938e8f6` (test)

**Plan metadata:** docs 提交在本 SUMMARY 之后（`docs(07-03): complete auth-header plan`，hash 见 git log）。

## Files Created/Modified

- `internal/server/proxy.go`【新】- D-15..D-20 决策依据注释头 + D-03 红线延伸登记；sanitizeRemoteUser（D-19 Go 移植）；proxyInfo{trust,userHeader} + clientIP/remote/remoteUser 三提取方法
- `internal/server/proxy_test.go`【新】- 白盒：TestSanitizeRemoteUser（15 子测 + UTF-8 断言）+ TestProxyClientIP（16 子测含多值头首值）
- `internal/server/proxy_e2e_test.go`【新】- 黑盒：TestRemoteUserLogging/TestXFFThrottleKey/TestAuthHeaderNoAuthBypass/TestShareChannelRemoteUser（双通道 503 + token 渠道 WS attach + D-03 自净断言）
- `internal/server/server.go` - Options.AuthHeader + Server.proxy 装配；删包级 clientIP 自由函数；logEvent variadic 第四参（红线注释扩展）；Attach 入口 remote/remoteUser 一次提取贯通握手事件行与 client 构造；issueTicketJSON 503 换键携值；pinger/logIfMessageTooBig 透传
- `internal/server/auth.go` - basicAuth 加 p proxyInfo 参数（ip 节流键换 p.clientIP、401/429 logEvent 换 p.remote + p.remoteUser；D-17：p 与认证判定零数据流注释登记）
- `internal/server/clients.go` - client.remoteUser 字段（Attach 写一次此后只读，并发论证注释）；slow_consumer/exit_when_empty 家族会话事件携第四字段
- `cmd/wesh/main.go` - config.authHeader + --auth-header flag 注册 + parseArgs 头注释 21→22 + validateStartup D-16 警告行 + run() Options 接线
- `cmd/wesh/main_test.go` - TestParseArgs wantAuthHeader 命名字段扩展（03-04 先例）+ TestStartupMatrix D-16 五行

## Decisions Made

- **proxy 测试分文件**（详见 Deviations #1）——白盒/黑盒两文件承载 plan 的四测试组，05-04 先例第二次沿用。
- **remote_user 载体范围判定**——attach 链路事件行全覆盖（含 message_too_big 两档 variadic 透传）+ 注册客户端会话事件（slow_consumer/pong_timeout/exit_when_empty 家族同携）：must_have truth「attach 链路的 logEvent 行携 remote_user」的字面要求与 behavior「clients.go 的会话事件（kick/pong_timeout 等）」的自然覆盖；未配置时全部与现状逐字节一致（variadic 空值不出键），零回归由 -race 全绿与冒烟对照双重锁定。
- **D-16 警告合并形态**——validateStartup 单 warn 通道下，--auth-header 暴露面警告为主句（可被直连伪造/确保不直接暴露/反代建议），--no-auth 裸奔语义随同行保持不丢；「凭据/TLS 矩阵其余行语义不变」由既有行零改动 + 新增五行锁定。
- **basicAuth 内 Basic 凭据变量改名 u/p→u/pass**——p 形参（proxyInfo）遮蔽原 p（密码）的机械调和，零语义改动。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - 字面不可达] proxy 测试分文件（白盒 proxy_test.go / 黑盒 proxy_e2e_test.go）**
- **Found during:** Task 1 RED（按 plan action ⑥ 写 proxy_test.go 时）
- **Issue:** plan must_have 要求 proxy_test.go 单文件提供 TestSanitizeRemoteUser/TestProxyClientIP/TestRemoteUserLogging/TestXFFThrottleKey 四组——前两组需 package server 白盒（sanitizeRemoteUser/proxyInfo 不导出），后两组按 plan 字面需 startTrackedServerWith/captureStderr（server_test 包既有 helper，05-01 -race 同步纪律禁止另造同步形态）；Go 单文件单 package 约束使单文件承载字面不可达
- **Fix:** 分文件——proxy_test.go（package server，白盒两表驱动组）+ proxy_e2e_test.go（package server_test，黑盒全链三组 + Task 2 的 TestShareChannelRemoteUser）；测试命名与 plan verify 命令逐字保持（-run 按名匹配不受文件分布影响）；min_lines 100 由白盒文件独立满足（127 行）
- **Files modified:** internal/server/proxy_test.go, internal/server/proxy_e2e_test.go
- **Verification:** 五测试全 PASS；`go test ./internal/server -run 'TestSanitizeRemoteUser|TestProxyClientIP|TestRemoteUserLogging|TestXFFThrottleKey|TestAuthHeaderNoAuthBypass' -count=1` 绿
- **Committed in:** 6461f0c（RED）/ 141fd3f（GREEN，gofmt）

**2. [Rule 1 - 验收命令自相矛盾] Task 2 字面验收 grep 按意图修正执行**
- **Found during:** Task 2（acceptance 复查）
- **Issue:** Task 2 acceptance 要求 `grep -rn 'clientIP(' internal/server --include='*.go' | grep -v '_test.go' | grep -v 'proxy.go' | wc -l` == 0；但 Task 1 acceptance 要求 `grep -c 's.proxy.clientIP\|p.clientIP' server.go auth.go` ≥ 2——方法调用行同样含 `clientIP(` 子串，两验收结构性自相矛盾（实测字面 grep = 3：server.go 注释 1 行 + Task 1 要求的方法调用 2 行）
- **Fix:** 按括号内意图（「旧自由函数零残留调用」）修正执行——旧自由函数定义已删除（server.go:576 注释登记），修正版 grep（排除 `.clientIP(` 方法调用与注释行）== 0；全部 per-IP 键与 remote 取值消费点逐行核读无遗漏（r.RemoteAddr 残留仅 proxy.go 回退路径与注释，语义正确）
- **Files modified:** 无（验收解释偏差，代码即 plan 意图形态）
- **Verification:** 修正版 grep == 0；`go test -race -count=1 ./internal/server ./cmd/wesh` 全绿（variadic logEvent 与 proxyInfo 零值零回归）
- **Committed in:** 938e8f6（Task 2 提交内登记）

---

**Total deviations:** 2 auto-fixed（1 字面不可达 Rule 3，1 验收命令 Rule 1）
**Impact on plan:** 两修正均为 plan 自身验收/结构字面与 Go 约束的机械调和，意图逐字保持（测试命名/验证命令/零残留语义不变）；prohibition（伪造头不得绕过认证、token/ticket 永不入 remote_user 或任何 logEvent 字段）严格保持并有运行时自净断言锁定。

## Issues Encountered

- **gofmt 对齐漂移（本 task 引入，随 task 修正）：** proxy.go/auth.go 等五文件注释列对齐，GOROOT gofmt -w 修正后零漂移（漂移清单仅剩 multi_test.go/slowclient_test.go 两既有文件，按 SCOPE BOUNDARY 未触碰，deferred-items.md 既定路由）。
- **TestShareChannelRemoteUser 设计约束：** startTrackedServerWith 的 waitHandlers 会等待长生命周期 WS attach handler——占位客户端需先 Close 再 waitHandlers（注释登记），否则 WaitGroup 永不返回；两轮独立实例（MaxClients=1 满员测 / 默认容量 token 渠道测）隔离容量维度。

## User Setup Required

None - no external service configuration required.

## Threat Flags

None——全部新表面均在 plan `<threat_model>` T-07-03a/b/c/d 四条登记内：裸信任 + D-16 启动警告（T-07-03a mitigate）、D-19 sanitize 日志注入防线（T-07-03b mitigate，换行注入子测 + C1 全链实证）、提取源限定配置头名 + D-03 红线延伸 + 运行时自净断言（T-07-03c mitigate）、XFF 轮换获独立节流配额（T-07-03d accept——D-20 已裁决，测试形态对照锁定）。无未建模的信任边界扩张。

## Known Stubs

None——无占位实现；全部六条 must_have truths 经 Go 单测 + -race 全量 + 真实二进制冒烟达成（remote_user sanitize 记录 / XFF 换键 logEvent+throttle 同键 / 未配置零漂移 / 认证正交 / D-16 警告行 / D-03 红线延伸自净）。

## Next Phase Readiness

- 07-04..07-08 可直接开工无阻塞：07-06 配置文件 auth-header 键（RESEARCH Pattern 4 fileConfig.AuthHeader 已预留）经同一 StringVar 落 cfg.authHeader 零新代码路径；07-07 UAT assertOutputClean 可消费 remote_user 行（D-03 自净断言 Go 侧已先行）；07-08 README 反代节（「--auth-header 仅反代后部署」明示 + ttyd -H 模型差异说明）与 SEC-07 需求文本修订素材齐备（D-15 收窄语义 + 本 plan 全链行为）
- SEC-07 按 D-15 收窄形态闭合（服务端审计归因）；前端身份显示与 --trusted-proxy CIDR 校验维持 CONTEXT deferred 登记不动

## Self-Check: PASSED

- 文件存在性：8/8 FOUND（proxy.go 含 func sanitizeRemoteUser ×1、type proxyInfo ×1；proxy_test.go 含 TestSanitizeRemoteUser/TestProxyClientIP；proxy_e2e_test.go 含 TestRemoteUserLogging/TestXFFThrottleKey/TestAuthHeaderNoAuthBypass/TestShareChannelRemoteUser；server.go 含 AuthHeader/s.proxy.clientIP/logEvent variadic；auth.go 含 p.clientIP ×1/p.remote(r) ×2；clients.go 含 remoteUser 字段；main.go 含 auth-header flag/D-16 警告行/AuthHeader 接线；main_test.go 含 wantAuthHeader/D-16 五行）+ 本 SUMMARY FOUND
- 提交存在性：3/3 FOUND（6461f0c / 141fd3f / 938e8f6）
- must_have 内容断言：grep 三闸 ==1 / server.go:2+auth.go:2 / auth.go:2（见 Task 1 验收输出）；`go test -race -count=1 ./internal/server ./cmd/wesh` 全绿；`go vet ./...` 零告警；GOROOT gofmt 漂移清单 == 两既有文件（本 plan 零新增）；真实二进制冒烟三项 success_criteria 逐条观测达成

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
