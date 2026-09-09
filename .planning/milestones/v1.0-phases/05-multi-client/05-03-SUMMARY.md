---
phase: 05-multi-client
plan: 03
subsystem: api
tags: [go, websocket, write-policy, owner, fifo-succession, promotion, prefs-dual-tier, osc52, multi-client, race]

# Dependency graph
requires:
  - phase: 05-multi-client plan 01/02
    provides: 注册表/hub 共享帧扇出/outbox/writer/detach/kick 拓扑；全局信用门与 P5-7 统一 Broadcast 挂点
provides:
  - 模式判定矩阵（decideModeLocked）：ticket mode × --writable × write-policy × owner 在位 → 生效 mode + rwEligible + 立 owner（RESEARCH Pattern 5 逐字）
  - owner FIFO 递补升格（promoteNextLocked）：detach 与 kickSlowConsumerLocked 双调用点（review #3 时序闭合）；升格 Welcome 复用 'W' 帧携 rw 档 prefs（R-09/P5-6）+ hubCond.Broadcast 升格挂点
  - --write-policy=owner|all CLI 公开契约（D-05，默认 owner；Task 1 确认门 as-locked 通过）：parse 期枚举校验 + fs.Visit 显式设置判定 + validateStartup 组合校验（显式 write-policy 无 --writable 拒绝 exit 2）
  - prefs 双档装配（D-13）：Options.ClientPrefsRO/ClientPrefsRW 替代 ClientPrefs；aggregateClientPrefs 产双 blob（ro 档永不含 osc52 键）；attach Welcome 按生效 mode 选档、升格 Welcome 携 rw 档
  - client.mode atomic.Value 承载（升格写 × INPUT 门无锁读并发安全）
  - TestOwnerPolicy/TestAllPolicy/TestSuccession/TestSuccessionKickRace 四测试（VALIDATION 05-01-02/05-01-07）
affects: [05-04 resize 仲裁（仲裁参与集切换挂点已登记 promoteNextLocked 注释）, 05-05 限速, 05-06 分享链接（token 绑定 mode 走同一矩阵）, 05-08 前端升格翻转, Phase 8 OPS-07]

# Actuals (#2632)
actuals:
  tokens: 17552
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "模式判定矩阵：升档时一次性判定（ticket 绑定 mode × writable × writePolicy × owner 在位）→ 生效 mode + rwEligible + becomeOwner，hubMu 内完成"
    - "FIFO 递补升格：registry.order 遍历首个 rwEligible 在线者 → Welcome{rw, rw档prefs} 入队 + owner 指针切换 + Broadcast；晋升恒在 hubMu 内、先于任何重连登记（review #3 时序闭合）"
    - "prefs 双档：聚合期产 ro/rw 两 blob（ro 档结构性不含 osc52 键），服务端不透明透传零运行期 JSON 手术（P5-6）"
    - "per-client mode atomic.Value 承载：INPUT 门每击键无锁 Load，升格写在 hubMu 内 Store——热路径与晋升并发安全"
    - "Welcome 入队先于注册表登记且全程持 hubMu：hub 扇出遍历注册表，防 onChunk 在 Welcome 前夹入 OUTPUT（首帧时序不变量）"
    - "fs.Visit 显式设置判定：writePolicySet 标记供 validateStartup 区分『默认 owner 未显式设置（纯 ro 会话放行）』与『显式设置无 --writable（配置矛盾拒绝）』"

key-files:
  created: []
  modified:
    - internal/server/clients.go
    - internal/server/server.go
    - internal/proto/proto.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - internal/server/multi_test.go
    - internal/server/slowclient_test.go
    - internal/server/e2e_test.go

key-decisions:
  - "[Phase 05-03]: client.mode 改 atomic.Value 承载——promoteNextLocked 的 ro→rw 升格写（hubMu 内）与 Attach 读循环 INPUT 门每击键无锁读并发（-race 实测命中）；每击键取 hubMu 会把输入热路径与 fan-out 临界区串行化，atomic 是唯一合理形态"
  - "[Phase 05-03]: promoteNextLocked 升格 Welcome trySend 失败按 R-08 同义踢出并重扫——该端 outbox 连 ~100B 余量都没有即事实上 stalled，绝不立无法送达升格通知的 owner（T-05-08 权限真空防线）"
  - "[Phase 05-03]: TestSuccessionKickRace 触发形态由『1013 踢出』改 pong_timeout 收口——owner 被 1013 踢出在 R-08 分工表下结构性不可达（owner 恒为唯一可写端，满即信用门而非踢出）；pong_timeout 是 stall owner 唯一可达的服务端主动移除路径，四断言（晋升/归队/单 owner/再递补）同款锁定"
  - "[Phase 05-03]: TestDetach 显式 WritePolicy=all 适配——owner 默认策略下 B 降级 ro（INPUT 静默丢）且 A 再 attach 归队 ro，生命周期断言前提失效；plan 适配清单（fanout/credit/kick 三项）的直接推论补全"
  - "[Phase 05-03]: WritePolicy 取值常量 WritePolicyOwner/WritePolicyAll 导出落 clients.go——main.go parse 校验与 server 矩阵判定共用同一对常量防双写漂移（proto.ModeRO/ModeRW 先例形态）"

patterns-established:
  - "确认门恢复执行形态：Task 1 checkpoint:decision 经用户 as-locked 裁决通过，纯确认门不产生独立代码提交，在 Task 2 提交消息与 SUMMARY 中登记"
  - "跨 wave 语义前提适配形态：owner 默认策略改变 wave 1/2 测试的双 rw 前提——显式 WritePolicy=all + 注释指向专测（fan-out/生命周期/背压语义与权限语义隔离）"
  - "INPUT 丢弃断言形态：先发 ro 端标记串 + pacing 300ms（保证服务端读循环已处理）再发 rw 端标记串，rw 端输出视角断言收齐自身标记且全程不见 ro 标记"

requirements-completed: [MULTI-02]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "owner 模式：首个 rw attach 为 owner（Welcome rw + rw 档 prefs 含 osc52），后续 rw 降级 ro 进递补队列（Welcome ro + ro 档 prefs 无 osc52 键）；降级端 INPUT 被丢弃、owner INPUT 生效"
    requirement: MULTI-02
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestOwnerPolicy"
        status: pass
    human_judgment: false
  - id: D2
    description: "all 模式：A/B 均 Welcome rw，双端 INPUT 均生效；无递补概念（A 断开后 B 保持 rw、无升格帧）"
    requirement: MULTI-02
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestAllPolicy"
        status: pass
    human_judgment: false
  - id: D3
    description: "owner FIFO 递补升格：owner 断线后降级端收第二帧 Welcome mode=rw 且携 rw 档 prefs（osc52 键），升格后 INPUT 生效；无可递补者时新 rw attach 直接立 owner"
    requirement: MULTI-02
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestSuccession"
        status: pass
    human_judgment: false
  - id: D4
    description: "owner 移除/重连时序闭合（review #3）：stall owner 被移除后晋升先于其重连登记、重连旧 owner 归队 FIFO 尾（Welcome ro）、全程单 owner、再递补链完整"
    requirement: MULTI-02
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestSuccessionKickRace"
        status: pass
    human_judgment: false
  - id: D5
    description: "--write-policy CLI 契约：默认 owner、显式 owner/all 原样解析、畸形值 parse 期 exit 2；显式 write-policy 无 --writable 启动拒绝（配置矛盾 fail-fast），默认 owner 无 --writable 纯 ro 会话放行"
    requirement: MULTI-02
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs/TestTLSKeyPairError/TestStartupMatrix/TestAggregateClientPrefs"
        status: pass
    human_judgment: false

# Metrics
duration: 53min
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 03: 写权限体系（owner/all + FIFO 递补升格 + prefs 双档）Summary

**MULTI-02 落地：--write-policy=owner|all（默认 owner，D-05 确认门 as-locked）+ 模式判定矩阵 + owner 断线 FIFO 递补升格（复用 'W' Welcome 帧运行期推送，携 rw 档 prefs）+ prefs 按 mode 双档分化（旁观者永不受 osc52）+ 四权限测试 -race 全绿。**

## Performance

- **Duration:** 53 min
- **Started:** 2026-08-20T11:38:25Z
- **Completed:** 2026-08-20T12:31:48Z
- **Tasks:** 3（Task 1 确认门 as-locked 通过 + Task 2 主干 + Task 3 测试组）
- **Files modified:** 8（0 新建）

## Accomplishments

- **确认门（Task 1）**：D-05 --write-policy CLI 公开契约经用户裁决 **as-locked** 通过——`--write-policy=owner|all` 默认 owner，与 CONTEXT.md D-05 逐字一致；纯确认门不产生独立代码提交（恢复指令授权）
- **模式判定矩阵落码**（clients.go decideModeLocked，RESEARCH Pattern 5 逐字）：rw×owner×无 owner → rw 立 owner；rw×owner×owner 在位 → ro 降级 + rwEligible 进 FIFO 尾；rw×all → rw；ro ticket × 任意 → ro 永不递补；无 --writable → ro 总闸（现状语义零漂移）
- **FIFO 递补升格**（promoteNextLocked）：owner 断线/被移除 → order 首个 rwEligible 在线者升格 → outbox 入队 Welcome{mode:rw, prefs:rw 档}（R-09 复用 'W' 帧零新类型字节；P5-6 升格即获 osc52）→ hubCond.Broadcast（P5-7 升格挂点：新 rw 端进信用集）；detach 与 kickSlowConsumerLocked 双调用点在同一 hubMu 持有内同步晋升（review #3 时序闭合——晋升必然先于被移除 owner 的任何重连登记，无双 owner 无晋升丢失）
- **--write-policy flag 全链**：parse 期枚举校验（畸形值 exit 2）+ fs.Visit 显式设置判定 + validateStartup 组合校验（显式 write-policy 无 --writable 拒绝，文案含双 flag 名；默认 owner 无 --writable 纯 ro 会话放行）
- **prefs 双档**（D-13 兑现 P4 deferred）：aggregateClientPrefs 产 ro/rw 两 blob（ro 档结构性不含 osc52 键，即使全局 --osc52 开启——T-05-07 旁观者剪贴板防线）；Options.ClientPrefsRO/ClientPrefsRW 替代单字段；attach Welcome 按生效 mode 选档、升格 Welcome 必携 rw 档；服务端不透明透传零运行期 JSON 手术（P5-6）
- **四测试锁定行为**：TestOwnerPolicy（升降级 + INPUT 门 + prefs 双档）/ TestAllPolicy（全员可写无递补）/ TestSuccession（升格 Welcome + osc52 档 + C 场景新 owner）/ TestSuccessionKickRace（review #3 四断言）；server 包全量 -race 绿，新测试 -race 3 连跑零失败

## Task Commits

Each task was committed atomically:

1. **Task 1 (checkpoint:decision): --write-policy CLI 公开契约确认门** - 无代码提交（用户裁决 as-locked 通过）
2. **Task 2: 模式判定矩阵 + owner/FIFO 递补 + 升格 Welcome + --write-policy flag + prefs 双档** - `602a824` (feat)
3. **Task 3: 权限测试组（含 client.mode atomic 竞态修复）** - `33961a9` (test)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `internal/server/clients.go` - WritePolicyOwner/WritePolicyAll 导出常量；client.rwEligible 字段；client.mode 改 atomic.Value；registry.owner 指针；decideModeLocked 判定矩阵（must_haves 五行逐字 + 越权面 prohibition 注释）；promoteNextLocked（FIFO 递补 + 升格 Welcome 携 rw 档 + Broadcast + 升格通知不可达同义踢出重扫 + 仲裁参与集切换挂点登记 05-04）；detach/kickSlowConsumerLocked 双调用点（review #3 时序闭合注释逐字）
- `internal/server/server.go` - Server/Options 加 writePolicy 与 clientPrefsRO/clientPrefsRW（ClientPrefs 单字段移除）；New 零值兜底 owner；升档序列 hubMu 内矩阵判定 + Welcome 按生效 mode 选档（入队先于登记保首帧时序）+ owner 指针登记；INPUT 门改 atomic Load
- `cmd/wesh/main.go` - config 加 writePolicy/writePolicySet；--write-policy flag（全名无短选项 P2 D-15）+ parse 期枚举校验 + fs.Visit 显式设置判定；validateStartup 组合校验（loopback 早退之前——纯配置矛盾与 bind 无关）；aggregateClientPrefs 双档产出；run 装配双档 + WritePolicy 透传
- `cmd/wesh/main_test.go` - TestParseArgs 加 wantWritePolicy（零值=owner 全行断言）+ owner/all 两行；TestTLSKeyPairError 畸形 write-policy 行；TestStartupMatrix 加 wantErrSub2 字段 + 组合校验拒绝/放行两行；TestAggregateClientPrefs 双档改写（ro 档 osc52 缺席断言）
- `internal/proto/proto.go` - Welcome 注释补运行期再推送（R-09——P2 D-01/D-02 不算动协议）；permission_denied 占位保持注释（CONTEXT 裁断：owner 降级走 Welcome 非 Error）
- `internal/server/multi_test.go` - sendInput/accumPayload/readUntilWelcome 三 helper；TestMultiClientFanout/TestDetach 显式 WritePolicy=all 适配；四权限测试追加
- `internal/server/slowclient_test.go` - TestGlobalCredit/TestSlowConsumerKick 显式 WritePolicy=all 适配（05-02 Known Stubs 登记项兑现）
- `internal/server/e2e_test.go` - TestWelcomePrefs 随 ClientPrefs 字段分裂迁移（双档注同一 blob 保原断言强度；双档分化行为归 TestOwnerPolicy）

## Decisions Made

- **client.mode atomic.Value 承载（Rule 1 修复定稿）**：promoteNextLocked 的升格写（hubMu 内）与 Attach 读循环 INPUT 门的无锁读（每击键热路径）并发——Task 3 首跑 -race 实测命中。备选「INPUT 门每击键取 hubMu」会把全部客户端击键与 fan-out 临界区串行化，不可接受；atomic.Value（string 动态类型）是热路径无锁读的唯一合理形态，hubMu 侧读取同经 Load 统一。
- **升格 Welcome trySend 失败的处置（Rule 2 防线）**：候选端 outbox 连 ~100B Welcome 余量都没有 = 事实上 stalled——按 R-08 分工表同义踢出（ro 满即踢）并继续扫描下一位，绝不立无法送达升格通知的 owner（T-05-08 权限真空防线；该端下一轮 onChunk 本就必被踢，仅提前一拍收口）。重扫循环防 removeLocked 在 range 中途改 order 切片的下标漂移。
- **TestSuccessionKickRace 触发形态（重大偏离，见 Deviation 2）**：plan 字面「owner stall 被 1013 踢出」在 R-08 分工表下结构性不可达——owner 模式 owner 恒为唯一可写端，其 outbox 写满走信用门（creditBlocked）而非踢出。改用短 ping/pong 参数使 stall owner 经 pong_timeout 收口（stall=不 Read=不答 pong）→ detach 路径晋升——与 kick 路径共享同一 hubMu 时序闭合论证，四断言（晋升/重连归队/全程单 owner/再递补）完整锁定。
- **TestDetach 适配补全（Rule 3）**：plan Task 2 item 6 适配清单只列 fanout/credit/kick 三测试，但 owner 默认策略同样使 TestDetach 断言前提失效（B 降级 ro 后 INPUT 静默丢、A 再 attach 归队 ro）——同款显式 WritePolicy=all + 注释指向专测。
- **WritePolicy 值常量导出**：WritePolicyOwner/WritePolicyAll 落 clients.go，main.go parse 校验与 server 矩阵判定共用（proto.ModeRO/ModeRW 先例防双写漂移）；适配测试按 plan 验收 grep 用 "all" 字面量。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] client.mode 竞态修复：升格写 × INPUT 门无锁读**
- **Found during:** Task 3（TestSuccession 首跑 -race 命中：promoteNextLocked 写 cand.mode 与 Attach 读循环 INPUT 门读 cl.mode 并发，server.go:570 × clients.go:412）
- **Issue:** 05-01/05-02 的 client.mode 注册后从不改写，读循环无锁读安全；05-03 升格引入运行期写者后构成 data race——这是 plan 新增机制（promoteNextLocked）的直接推论，不修则 CI 必现
- **Fix:** mode 改 atomic.Value 承载（string 动态类型）：升档 Store 初始值、promoteNextLocked Store 翻转、INPUT 门/信用门/分工表全部经 Load；每击键热路径保持无锁
- **Files modified:** internal/server/clients.go, internal/server/server.go
- **Verification:** 四权限测试 + server 包全量 -race 绿；新测试 -race 3 连跑零竞态
- **Committed in:** 33961a9（Task 3 提交）

**2. [Rule 1 - Plan 缺陷] TestSuccessionKickRace 触发形态：1013 踢出 owner 结构性不可达 → pong_timeout 收口**
- **Found during:** Task 3 测试设计（kickOrCreditLocked 分工表推演）
- **Issue:** plan 字面前提「owner 模式 A(owner rw)/B(降级 ro)；洪水使 A stall 被 1013 踢出」在 R-08 分工表下不可达——owner 恒为唯一可写端，「剔除 c 后仍存在未 blocked 的可写端」对唯一可写端恒假，故 owner outbox 写满走信用门（creditBlocked 闭门）而非 1013 踢出；owner 被 kick 需 ≥2 个 rw 端，owner 模式结构性 ≤1 个。该前提与 plan 自身 must_haves（05-02 锁定的分工表）矛盾
- **Fix:** 保留测试名与四断言（晋升 / 重连归队 ro / 全程单 owner / 再递补），触发改为 PingInterval=100ms/PongTimeout=2s 覆写：stall owner（不 Read）不答 pong → pinger pong_timeout CloseNow → detach → promoteNextLocked 第一调用点晋升。pong_timeout 是 stall owner 唯一可达的服务端主动移除路径；detach/kick 两调用点共享「removeLocked 后同一 hubMu 持有内同步晋升」的时序闭合论证，review #3 性质（晋升先于重连登记、无双 owner、无晋升丢失）同款锁定。kick 路径第二调用点按 plan 保留为防御性挂点（未来策略形态变化的防线；grep 验收满足）
- **Files modified:** internal/server/multi_test.go
- **Verification:** TestSuccessionKickRace -race 3 连跑绿（含 pong_timeout 晋升 ~2.1s 时序证据）
- **Committed in:** 33961a9

**3. [Rule 3 - Blocking] 跨 wave 适配补全：TestDetach + TestWelcomePrefs**
- **Found during:** Task 2（适配面勘察：owner 默认策略推演全部双客户端测试）
- **Issue:** plan Task 2 item 6 适配清单只列 TestMultiClientFanout/TestGlobalCredit/TestSlowConsumerKick 三项；但 owner 默认下 TestDetach 的「B 继续 echo」（B 降级 ro 后 INPUT 静默丢）与「A 再 attach mode=rw」（owner B 在位，A 归队 ro）两断言前提失效；TestWelcomePrefs 因 ClientPrefs 字段分裂编译失败
- **Fix:** TestDetach 同款显式 WritePolicy=all（生命周期语义与权限语义隔离，注释指向专测）；TestWelcomePrefs 两档注同一 blob（单客户端 rw 半侧实际选 rw 档，原断言强度保持，双档分化归 TestOwnerPolicy）
- **Files modified:** internal/server/multi_test.go, internal/server/e2e_test.go
- **Verification:** server 包全量 -race 绿（05-01/05-02 测试零回归）
- **Committed in:** 602a824（Task 2 提交）

---

**Total deviations:** 3 auto-fixed（2 Rule 1 - Bug/Plan 缺陷，1 Rule 3 - Blocking）
**Impact on plan:** 三处修正全部服务于 plan 自身 truths/prohibitions/验收（-race 绿、时序闭合、零回归），未改动 plan 锁定的机制形态（判定矩阵/递补升格/双档/分工表全部逐字保持）；Deviation 2 仅改测试触发形态，review #3 的四断言性质完整锁定。

## Known Stubs

None — 本 plan 无新增占位 stub。既有挂账项保持：registry.kicks/gateTransitions 计数器（Phase 8 OPS-07 消费）、permission_denied 占位注释（CONTEXT 裁断不硬用）、promoteNextLocked 注释登记的仲裁参与集切换挂点（05-04 resize.go 落地）。

## Issues Encountered

- **05-02 Known Stubs 登记项兑现**：TestGlobalCredit/TestSlowConsumerKick 的 WritePolicy=all 适配为 05-02 预先登记的适配点，本 plan 落地时顺手补齐（含 TestSlowConsumerKick 的 Writable=true 前提注释钉死）。
- **promoteNextLocked 的 range-改切片陷阱**：升格 Welcome trySend 失败的同义踢出会经 removeLocked 改 registry.order——range 循环持有旧切片头会继续遍历陈旧下标；改为「扫描取首个候选 → 踢出后重扫」循环形态消除。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的占位与后续挂点）

- **仲裁参与集切换挂点**：promoteNextLocked 注释已登记「05-04 resize.go 落地时新 owner 尺寸接管」（D-09：owner 模式仅 owner 尺寸参与仲裁）
- **前端升格翻转**：升格信号 = 标题前缀消失 + 键盘激活，无 toast/面板/badge（D-07 零新 UI 组件）——前端 WELCOME 分支补 rw 分支由 05-08 落地
- **kick 路径 promoteNextLocked 第二调用点**：防御性挂点（R-08 分工表下 owner 模式不可达；all 模式无 owner）——未来策略形态变化的防线
- **permission_denied 占位**：保持注释不硬用（CONTEXT 裁断；P3 deferred 入表挂账纪律延续）

## Next Phase Readiness

- 05-04 resize 仲裁：参与集判定可直接读 c.mode（atomic Load）与 registry.owner；升格挂点已登记
- 05-06 分享链接：token 绑定 mode（ro/rw ticket）直接喂 decideModeLocked 矩阵——ro token 持有者永不递补已由「ro ticket × 任意 → ro（rwEligible=false）」行覆盖
- 无阻塞项；server/cmd/proto/pty 全量 -race 绿，新测试 3 连跑稳定

## Self-Check: PASSED

- FOUND: internal/server/clients.go rwEligible/promoteNextLocked/decideModeLocked（grep 16/6/4 处）
- FOUND: internal/server/multi_test.go func TestOwnerPolicy/TestAllPolicy/TestSuccession（grep == 3）+ TestSuccessionKickRace（== 1）
- FOUND: commit 602a824（Task 2）、33961a9（Task 3）均在 git log
- 验收 grep 全过：ClientPrefsRO/RW server.go+main.go == 8（≥4）、write-policy main.go == 11（≥2）、WelcomeFrame(proto.ModeRW clients.go == 1（≥1）、WritePolicy "all" 适配 == 4（≥2）
- go build/vet 退出 0；go test -race -count=1 ./internal/server/（31s 全绿）./cmd/wesh/ ./internal/proto/ ./internal/pty/ 全绿；权限测试组 -race 3 连跑零失败

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
