---
phase: 05-multi-client
plan: 07
subsystem: api
tags: [go, websocket, max-clients, capacity, http-503, atomic-counter, registry, multi-client, race]

# Dependency graph
requires:
  - phase: 05-multi-client plan 01/06
    provides: 注册表/hub/outbox/detach/kick 拓扑（removeLocked 唯一收口点）；Options.MaxClients 字段与 defaultMaxClients=32 常量；Attach 守卫区③位占位注释（05-01 拆 409 后）；shareAttach token 分支与 issueTicketJSON 共享签发点（05-06）
provides:
  - registry 内嵌 n atomic.Int64 注册计数器（R-06 口径：registerLocked +1 / removeLocked -1 唯一收口点，review #7 对称不变量逐字注释）
  - Attach 守卫区③位 503 闸：满员 Accept 前 HTTP 503 + release() 恰好一次 + logEvent(remote, 503, "max_clients")（P5-5 顺序纪律）
  - /api/attach 503 早闸（OQ2）：issueTicketJSON 唯一共享签发点一处检查，Basic 链与 token 分支同查，双点位同 reason 串
  - --max-clients flag（IntVar 默认 32，D-08 one-way 确认门 as-locked）→ Options.MaxClients 接线；≤0 经 validateStartup 拒绝 exit 2
  - TestMaxClients503（VALIDATION 05-02-02：WS 503 + halfOpen 无泄漏 + 早闸双通道 + detach/kick 双路径槽位释放 + stderr 事件）+ TestClientCountInvariant（VALIDATION 05-01-08 白盒逐步不变量 + 幂等防御）
affects: [05-08 前端（Server is full 专版文案 C-2 的服务端半侧已就绪——/api/attach fetch 阶段 503 + WS 握手 503）, 05-09 README（容量段：注册后计数、瞬时超编 ≤8、--max-clients flag 说明）+ UAT phase05.mjs（满员 503 场景锚点）, Phase 8 OPS-07（连接数指标观测通道）, Phase 9（负载标定回填默认 32）]

# Actuals (#2632)
actuals:
  tokens: 9946
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "容量计数内嵌 registry：n atomic.Int64——③位闸在 hubMu 外 atomic load 故须 atomic（与 kicks/gateTransitions 的 hubMu 内 plain int 成场景化选型）；registerLocked/removeLocked 唯一加减点，运行期全部移除路径收口于 removeLocked 单点"
    - "守卫区③位 503 闸形态：⓪Origin 403 → ①子协议 400 → ②halfOpen 429 → ③max-clients 503（P5-5：503 必须在 halfOpen acquire 之后——满员时攻击者不得占半开名额；拒绝路径 release() 恰好一次，02-03 sync.Once + defer 兜底先例）"
    - "/api/attach 容量早闸落唯一共享签发点 issueTicketJSON：Basic 链与 token 分支一处检查两通道同查；WS ③位兜底竞态窗口（ticket 已签但握手时满员 → WS 侧 503，语义无害）"
    - "halfOpen 泄漏的行为化断言形态：MaxHalfOpenPerIP=1 装配下 ③位 503 后再来一人仍收 503 而非 429——泄漏一个名额即触 ②位 429，503 复现即 release() 恰好一次的证据"
    - "stall 夹具纪律（05-02 既有，本 plan 违例实测后钉死）：stall 端在踢出触发前绝不 Read——先等正常端累积超 12MiB（stall 端管道必然已满、踢出已触发），才经 assertKicked1013 首次 Read 取证；洪水须以数量级余量压过踢出点（389MB ≫ ~10MiB 吸收极限），防子进程先耗尽触发 lifecycle 1000 与异步 Close(1013) 竞态"

key-files:
  created:
    - internal/server/clients_test.go
  modified:
    - internal/server/clients.go
    - internal/server/server.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - internal/server/multi_test.go

key-decisions:
  - "[Phase 05-07]: D-08 确认门 as-locked 通过（Task 1）——--max-clients 默认 32 + ③位 Accept 前 503 + R-06 注册后计数，与 CONTEXT.md D-08 逐字一致；瞬时超编 ≤8 已裁决接受（容量策略非安全边界）"
  - "[Phase 05-07]: /api/attach 早闸落 issueTicketJSON 而非 attachHandler 字面——plan must_have 要求 Basic 链与 token 分支同查，issueTicketJSON 是两通道唯一共享签发点（一处检查两通道同查 + WS ③位兜底竞态窗口）；signature 加 *http.Request 供 logEvent 取 RemoteAddr"
  - "[Phase 05-07]: registerLocked 惰性建 map 使 registry 零值可用——plan Task 3 锁定 TestClientCountInvariant『直构造 registry（零值可用——无需 Server/pty 装配）』，零值 registry 的 nil map 直写必 panic；New 显式初始化路径不受影响"
  - "[Phase 05-07]: kick 子场景 stall 夹具两处修正（-count=3 实测驱动）——stall 端踢出触发前绝不 Read（assertKicked1013 的 readUntilError 即读者，提前调用排空管道使踢出永不成立）；洪水 38.9MB→389MB（子进程先耗尽则 lifecycle 1000 与 Close(1013) 竞态，stall 端观测 1000，实测命中）"

patterns-established:
  - "确认门恢复执行形态第三次沿用：Task 1 checkpoint:decision 经用户 as-locked 裁决通过，纯确认门不产生独立代码提交，在 Task 2 提交消息与本 SUMMARY 登记（05-03/05-06 先例）"
  - "计数泄漏的行为化断言：槽位释放用『A CloseNow → 轮询 dial 直至成功（5s  deadline）』形态——detach 经服务端 reader 终结异步完成，轮询消除到达序竞态；kick 路径用『1013 观测即计数已减』（removeLocked 先于关闭帧发出，无需轮询）"

requirements-completed: [RES-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "WS 满员 503：MaxClients=2 下 A/B attach 成功后第三人在 Accept 前收 HTTP 503；halfOpen 无泄漏（MaxHalfOpenPerIP=1 下第四人仍收 503 而非 429——release() 恰好一次行为化）；stderr 事件含 max_clients + code=503（R-10）"
    requirement: RES-03
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestMaxClients503/WS_满员_503_与_halfOpen_无泄漏与槽位释放"
        status: pass
    human_judgment: false
  - id: D2
    description: "/api/attach 503 早闸（OQ2）：凭据模式 MaxClients=1 占满后 Basic 正确 POST → 503；token 通道实例 body 携 rw token → 503（两签发路径同查）"
    requirement: RES-03
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestMaxClients503/api_attach_早闸双通道"
        status: pass
    human_judgment: false
  - id: D3
    description: "槽位释放双路径（Pitfall 4 / T-05-04b）：A CloseNow → detach -1 → 第三人 attach 成功；stall B 被 1013 踢出（removeLocked 第二移除路径）→ 第三人 attach 成功"
    requirement: RES-03
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestMaxClients503（WS 子测槽位释放段 + kick_路径槽位释放子测）"
        status: pass
    human_judgment: false
  - id: D4
    description: "计数对称不变量白盒（review #7）：register/remove 交错序列逐步 n == len(set)；重复移除与幽灵移除幂等 no-op 且计数不变"
    requirement: RES-03
    verification:
      - kind: unit
        ref: "internal/server/clients_test.go#TestClientCountInvariant"
        status: pass
    human_judgment: false
  - id: D5
    description: "--max-clients CLI 契约：IntVar 默认 32 接线 Options.MaxClients；≤0 经 validateStartup 拒绝 exit 2（文案含 max-clients，loopback 不豁免）；既有套件零回归"
    requirement: RES-03
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix（max-clients zero/negative 拒绝两行 + 既有 11 行基值注入后零回归）"
        status: pass
      - kind: integration
        ref: "go test -race -count=1 ./internal/server/ ./cmd/wesh/ ./internal/proto/ ./internal/pty/（全绿）；定向 -count=3 三连跑零失败"
        status: pass
    human_judgment: false

# Metrics
duration: 1h 15m
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 07: max-clients 容量闸（③位 503 + 注册计数器 + /api/attach 早闸）Summary

**RES-03 落地：--max-clients flag（默认 32，D-08 确认门 as-locked）+ registry 内嵌 atomic 注册计数器（R-06 注册后计数，removeLocked 唯一收口）+ Attach ③位满员 Accept 前 HTTP 503（release 恰好一次 + max_clients 事件）+ /api/attach 503 早闸双通道（OQ2，issueTicketJSON 共享签发点）+ detach/kick 双路径槽位释放与计数不变量三层测试锁定，四包 -race 全绿。**

## Performance

- **Duration:** 1h 15m
- **Started:** 2026-08-20T15:51:59Z
- **Completed:** 2026-08-20T17:06:58Z
- **Tasks:** 3（Task 1 确认门 as-locked 通过 + Task 2 主干 + Task 3 测试组）
- **Files modified:** 6（1 新建：clients_test.go；5 修改：clients.go/server.go/main.go/main_test.go/multi_test.go）

## Accomplishments

- **确认门（Task 1）**：D-08 --max-clients CLI 公开契约经用户裁决 **as-locked** 通过——flag 默认 32（ARCHITECTURE §6『10–100 连接=团队围观/教学』区间下沿）+ 满员 Accept 前 HTTP 503 + R-06 注册后计数，与 CONTEXT.md D-08 逐字一致；纯确认门不产生独立代码提交（05-03/05-06 先例沿用）
- **注册计数器**（clients.go registry `n atomic.Int64`）：R-06 口径——registerLocked 成功后 +1、removeLocked -1，半开连接不计入（与 halfOpenCounter 正交两闸：SEC-08 半开面 vs RES-03 稳态容量面）；③位闸在 hubMu 外 atomic load 故须 atomic；review #7 对称不变量逐字注释登记（运行期全部移除路径——reader-error detach / 1013 踢出 / pinger CloseNow→detach——收口于 removeLocked 单点，漂移结构上不可能；lifecycle 广播后进程即退出、panic 进程崩溃——两路径无漂移窗口）
- **③位 503 闸**（server.go Attach，05-01 占位注释处重建）：⓪Origin 403 → ①子协议 400 → ②halfOpen 429 → ③max-clients 503——P5-5 顺序纪律（503 必须在 halfOpen acquire 之后：满员时攻击者不得占半开名额）；拒绝路径 release() 恰好一次（02-03 sync.Once + defer 兜底先例）；logEvent(remote, 503, "max_clients")（R-10：HTTP 层事件 code 复用状态码值，auth.go 401/429 强转先例）；瞬时超编 ≤8 注释明示（A5 裁断，容量策略非安全边界）
- **/api/attach 503 早闸**（OQ2）：落 issueTicketJSON——Basic 链与 token 分支的唯一共享签发点，一处检查两通道同查；WS ③位兜底竞态窗口（ticket 已签但握手时满员 → WS 侧 503，语义无害）；双点位同 reason 串 max_clients 保持可观测一致性（P3 纪律）
- **--max-clients flag 全链**：IntVar 默认 32（全名无短选项 P2 D-15；容量策略部署关切 D-08 与 P2 D-10 攻击面常量不同类，Phase 9 负载标定回填注释）；≤0 经 validateStartup 拒绝 exit 2（纯配置有效性与 bind 无关，loopback 不豁免，文案含 max-clients）；cfg.maxClients → Options.MaxClients 接线
- **三层测试锁定**（review #7 证据链）：TestClientCountInvariant 白盒逐步不变量（n == len(set) 逐步断言 + 重复移除/幽灵移除幂等零计数变化）+ TestMaxClients503 槽位释放（detach 路径：A CloseNow → 轮询 attach 成功）+ kick 路径槽位释放子场景（stall B 被 1013 踢出 → 第三人 attach 成功）；WS 满员 503 + halfOpen 无泄漏（MaxHalfOpenPerIP=1 行为化）+ stderr max_clients 事件；定向 -race -count=3 三连跑零失败，四包全量 -race 绿

## Task Commits

Each task was committed atomically:

1. **Task 1 (checkpoint:decision): --max-clients CLI 公开契约确认门（D-08，one-way）** - 无代码提交（用户裁决 as-locked 通过）
2. **Task 2: ③位 503 闸 + 注册计数器 + --max-clients flag + /api/attach 早闸** - `51e32e1` (feat)
3. **Task 3: TestMaxClients503 + TestClientCountInvariant（含 kick 子场景夹具修正）** - `43fa63e` (test)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `internal/server/clients.go` - registry 内嵌 `n atomic.Int64`（R-06 口径 + review #7 对称不变量逐字注释）；registerLocked +1（含惰性建 map 零值可用）/ removeLocked -1（非成员幂等且计数不变——减计数不得越过实际移除）；defaultMaxClients 注释消费点兑现
- `internal/server/server.go` - Attach 守卫区③位 503 闸（占位注释重建为实闸：registry.n.Load >= maxClients → release() + logEvent(remote, 503, "max_clients") + http.Error 503）；issueTicketJSON 加 OQ2 早闸（signature 加 *http.Request 供 logEvent remote）——shareAttach/attachHandler 两调用点同步；Server/Options 注释消费点兑现
- `cmd/wesh/main.go` - config.maxClients 字段；--max-clients IntVar 默认 32（D-08 注释：容量策略部署关切，Phase 9 标定回填）；validateStartup ≤0 拒绝（loopback 早退之前——纯配置有效性）；run() Options.MaxClients 接线；parseArgs 文档注释 14→15 flag
- `cmd/wesh/main_test.go` - TestStartupMatrix：既有 11 行注入 maxClients:32 基值（零值误拒基线同步，断言语义不变）+ maxClients=0/-1 拒绝两行（bind 127.0.0.1 隔离其他校验维度）
- `internal/server/multi_test.go` - TestMaxClients503 三子测（WS 满员 503+halfOpen 无泄漏+detach 槽位释放+stderr 事件 / api attach 早闸双通道 / kick 路径槽位释放）；net/http、strings、sync/atomic import
- `internal/server/clients_test.go`（新）- TestClientCountInvariant 同包白盒（tickets_test.go 先例）：register/remove 交错序列逐步 n == len(set) + 幂等防御（重复移除/幽灵移除零计数变化）+ 全量移除归零断言

## Decisions Made

- **/api/attach 早闸落点（issueTicketJSON 而非 attachHandler 字面）**：plan action 字面「attachHandler 内」与 must_have「Basic 链与 token 分支同查」在机械层面冲突——attachHandler 只覆盖 Basic 链，token 分支（shareAttach 直签）不在其内。issueTicketJSON 是两通道唯一共享签发点，plan 自身要求的注释「一处检查两处用」正指此形态；signature 加 *http.Request 供 logEvent 取 RemoteAddr（两调用方均有 r，零成本）。
- **registerLocked 惰性建 map**：plan Task 3 锁定「直构造 registry（零值可用——无需 Server/pty 装配）」，但零值 registry 的 set 为 nil map，registerLocked 直写必 panic——惰性初始化使「零值可用」字面成立（New 的显式初始化路径不受影响，两行防御）。
- **registry.n 与既有计数器的形态选型**：kicks/gateTransitions 是 hubMu 内 plain int（R-07 单锁纪律），n 必须 atomic——③位闸在守卫区 Accept 前执行，取 hubMu 会把握手路径与 fan-out 临界区串行化且违反守卫区零阻塞形态；字段注释登记场景化选型理由。
- **TestStartupMatrix 基值注入取逐行显式形态**：11 行逐一注入 maxClients: 32 而非循环内零值兜底——拒绝行之一恰是 maxClients=0（零值即测试值），循环兜底会使该行不可表达；逐行显式注入同时满足 plan「保持 wantErr/wantWarn 断言语义不变」。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] kick 子场景 stall 夹具违例：踢出触发前提前 Read stall 端**
- **Found during:** Task 3（定向 -race -count=3 稳定性验证首跑即红：`stall B not kicked within 15s` / `stall B close code = 1000, want 1013` 两种失败签名交替）
- **Issue:** 初版在 dialHello 后立即调 assertKicked1013——其 readUntilError 即启动读者 goroutine 持续排空 B 的管道，stall 前提被破坏（B 不再 stall，outbox 永不写满或仅靠 reader GC 迟滞偶发触发）；38.9MB 洪水形态下还有第二竞态：子进程先于踢出耗尽退出 → lifecycle 广播 1000 与 kick 的异步 Close(1013) 经 casClosing 竞态，stall 端观测到 1000 而非 1013
- **Fix:** 按 TestSlowConsumerKick 逐字纪律重排——A 持续读取计数，先等 A 累积超 12MiB（此时 B 管道 ~10MiB 最坏吸收必然已满、outbox 已写满、踢出已触发），才经 assertKicked1013 首次 Read B 取证（关闭帧写出 5s 超时窗口内开始消化管道）；洪水加大至 seq 1 50000000 ≈389MB（踢出点 ~10MiB 吸收极限的数量级余量，lifecycle 1000 结构性不可能先于踢出）
- **Files modified:** internal/server/multi_test.go
- **Verification:** 修正后定向 -race -count=3 三连跑（=9 次执行）零失败；四包全量 -race 绿
- **Committed in:** 43fa63e（Task 3 提交）

---

**Total deviations:** 1 auto-fixed（Rule 1 - Bug，测试夹具形态——生产代码零偏离）
**Impact on plan:** 修正全部落在测试编排（stall 纪律 + 洪水量余量），plan 锁定的机制形态（③位闸/计数口径/早闸点位/flag 契约）逐字保持；05-02 既有的 stall 夹具纪律注释在本 plan 以实测代价再次验证其必要性。

## Threat Model 处置

| Threat ID | 处置 | 证据 |
|-----------|------|------|
| T-05-04（连接数耗尽，high） | **mitigate 已落地** | ③位 503（Accept 前零 WS 资源分配）+ per-IP halfOpen 429 两闸正交（守卫区注释逐字登记顺序）；/api/attach 早闸减少 ticket 空转；TestMaxClients503 满员拒绝 + 双通道早闸锁定 |
| T-05-04b（计数器泄漏，high） | **mitigate 已落地** | R-06 对称记账：计数器内嵌 registry（n atomic.Int64），registerLocked/removeLocked 唯一加减点——运行期全部移除路径收口 removeLocked 单点（review #7 注释逐字登记）；503 拒绝 release() 恰好一次（P5-5，MaxHalfOpenPerIP=1 行为化断言）；三层证据：TestClientCountInvariant 白盒逐步不变量 + detach 槽位释放 + kick 路径槽位释放 |
| T-05-04c（瞬时超编竞态，low） | **accept（登记维持）** | R-06/A5 裁断：容量策略非安全边界，超编幅度 ≤ per-IP 半开帽 8；③位闸与 clients.go 字段注释均明示；Phase 9 负载标定复核；README 容量段说明由 05-09 落地 |

无新增威胁面——③位闸与早闸即 plan 威胁模型的本体；无新端点/协议改动（P2 D-01 纪律保持）。

## Known Stubs

None — 本 plan 无新增占位 stub（无硬编码空值/占位文案/TODO；全部 verify 均已运行）。既有挂账项保持：registry.kicks/gateTransitions/inputDrops/droppedInputs（Phase 8 OPS-07 消费）、permission_denied 占位注释。

## Issues Encountered

- **验收 grep 与 gofmt 对齐组冲突**：`grep -c 'n atomic.Int64'` 因 gofmt 将 n 与相邻 kicks 字段对齐（`n     atomic.Int64`）而得 0——字段间加空行断开对齐组解决（单空格字面满足验收）。
- **gofmt 版本漂移（既有项，非本 plan 引入）**：/usr/bin/gofmt 与 GOROOT gofmt 对 CJK 注释（//（ → // （）与尾注释对齐规则不同，HEAD 多文件本即在 GOROOT gofmt 下 dirty（CI 无 gofmt 门禁）——本 plan 改动保持文件局部风格一致，未新增非注释类格式漂移；属 01-03 已登记事项，不重复挂账。
- **负载时序簇两次实测失败**（见 Deviation 1）：均以夹具纪律修正收口，非生产代码缺陷；修正后 9 连跑零失败。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的占位与后续挂点）

- **05-08 前端**：/api/attach fetch 阶段 503 → Server is full 专版文案（UI-SPEC C-2）——服务端半侧已就绪（早闸 503 + WS ③位 503 兜底）
- **05-09 README/UAT**：容量段补『注册后计数、并发握手瞬时超编 ≤8』说明（review MEDIUM 处置）；phase05.mjs 满员 503 场景（--max-clients 小值 spawn，VALIDATION 05-02-02 UAT 侧）
- **Phase 9 负载标定**：defaultMaxClients=32 初值回填（与 outbox/限速参数同批，P2 D-10 纪律）；ro/rw 分别限额（10 rw + 100 ro 类）属 v2 候选——REQUIREMENTS 无 per-mode 容量条目，本 phase 不做（review 处置：延期）

## Next Phase Readiness

- 05-08 前端：满员 503 双点位（fetch 阶段 + WS 握手）全部就绪，可直接落 Server is full 文案分支
- 05-09 UAT：--max-clients flag 可经 spawn 参数驱动满员场景；stdout 链接解析与 503 场景正交
- 无阻塞项；server/cmd/proto/pty 四包 -race 全绿，新测试定向 9 连跑稳定

## Self-Check: PASSED

- FOUND: internal/server/clients.go `n atomic.Int64`（grep == 1）+ `.n.Add(1)`/`.n.Add(-1)`（grep == 2，registerLocked/removeLocked 对称记账唯一收口点）
- FOUND: internal/server/server.go `max_clients`（grep == 3 ≥ 2）+ `registry.n.Load`（grep == 2：③位 + 早闸双读取点）；③位拒绝路径 grep -B3 -A3 可见 release() 与 logEvent
- FOUND: cmd/wesh/main.go `max-clients`（grep == 5 ≥ 1）；internal/server/multi_test.go `func TestMaxClients503`（grep == 1）；internal/server/clients_test.go `func TestClientCountInvariant`（grep == 1）
- FOUND: commit 51e32e1（Task 2）、43fa63e（Task 3）均在 git log；两提交均无意外文件删除（--diff-filter=D 检查通过）
- go build/vet 退出 0；go test -race -count=1 ./internal/server/（37.8s 全绿）./cmd/wesh/（1.0s）./internal/proto/ ./internal/pty/ 全绿；定向 -race -count=3 三连跑零失败

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
