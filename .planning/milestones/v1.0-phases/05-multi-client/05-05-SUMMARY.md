---
phase: 05-multi-client
plan: 05
subsystem: api
tags: [go, rate-limit, x-time, token-bucket, backpressure, input-queue, goroutine, multi-client, race]

# Dependency graph
requires:
  - phase: 05-multi-client plan 01/03
    provides: per-client mode 门（INPUT case ro 静默丢，atomic.Value 承载）/Options 五测试可覆写字段与五默认常量（InputRate/InputBurst 已立）；writer/pinger goroutine 装配先例
provides:
  - client.limiter 每客户端 x/time/rate 令牌桶（Attach 升档构造 32KiB/s + 64KiB burst，R-01 参数表；Options.InputRate/InputBurst 覆写）
  - inputQ 会话级字节有界输入队列（256KiB ≥16 个 16KiB 满帧；tryEnqueue 满则丢 + droppedInputs 计数；dequeue swap 出队）
  - inputWriter 单 goroutine 独占 sess.Master.Write（New 启动、lifecycle close(inputDone) 收口；Drain→Close 解除在途写阻塞 D-12 同款）
  - INPUT 门链：mode 门 → limiter.AllowN（超限丢弃 + inputDrops 计数，R-02 不断开不逐次日志不踢出）→ tryEnqueue——读循环同步 Master.Write 删除（CR-01 完整背压修复，PROJECT.md 锁定项兑现）
  - go.mod 新 direct 依赖 golang.org/x/time v0.15.0（proxy 引入，go.sum 校验链入库；钉版 rationale 注释落 clients.go 引入点）
  - TestInputRateLimit 双子测（VALIDATION 05-01-05：超限丢弃 + 连接存活 + 未超限送达 + 对照全量送达）
affects: [05-09 README（限速丢弃语义与分段粘贴建议、all 模式多写者交错不排序明示）, Phase 8 OPS-07（inputDrops/droppedInputs 计数器进 metrics）, Phase 9（inputRate/inputBurst/inputQueueBytes 标定回填）]

# Actuals (#2632)
actuals:
  tokens: 6147
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added:
    - "golang.org/x/time v0.15.0（rate.Limiter——官方命名空间 + sum.golang.org 校验链，RESEARCH §Package Legitimacy Audit Approved；rate API 自 2015 年签名稳定，钉版防漂移，升级经 go.sum 审计链显式进行）"
  patterns:
    - "INPUT 门链（读循环内逐帧）：mode 门（ro 静默丢）→ limiter.AllowN(now, len-1)（超限静默丢 + inputDrops.Add(1)）→ inputQ.tryEnqueue(data[1:])（满则丢 + droppedInputs 计数）——全链无阻塞无同步写"
    - "会话级输入队列 + 单 input-writer goroutine：outbox 同款形态（mu + [][]byte + bytes/cap + cap 1 信号 channel），tryEnqueue 非阻塞投递、dequeue swap 整队、writer 顺序写 master——CR-01：阻塞被关进专属 goroutine，master fd 保持默认阻塞模式"
    - "goroutine 生命周期挂服务端形态：New 内与 ReadLoop/lifecycle 同批启动；lifecycle 子进程退出路径 Drain→Close（runtime poller 解除在途写阻塞）→ close(inputDone)（解除 select 等待）——双通道收口零泄漏"
    - "热路径计数器 atomic 形态：inputDrops/droppedInputs 均 atomic.Int64（读循环每击键无锁递增；Phase 8 metrics 读取端免锁），区别于 registry.kicks/gateTransitions 的 hubMu 内 plain int（信用门判定本就在锁内）"

key-files:
  created:
    - .planning/phases/05-multi-client/deferred-items.md
  modified:
    - internal/server/clients.go
    - internal/server/server.go
    - internal/server/multi_test.go
    - go.mod
    - go.sum

key-decisions:
  - "[Phase 05-05]: droppedInputs 计数递增收进 inputQ.tryEnqueue 内部（自含记账，outbox bytes 同款）——调用方形态 `if !tryEnqueue { continue }`；inputDrops 为 Server 字段 atomic.Int64（INPUT 门热路径），两计数器均挂 Phase 8 OPS-07 注释（review #10）"
  - "[Phase 05-05]: input-writer 终结双通道——lifecycle 内 Drain→Close 先关 master fd（在途 Write 经 runtime poller 解除阻塞返回错误即 return），close(inputDone) 解除 select 等待；写失败路径直接 return（子进程退出由 lifecycle 收口），队列残余随会话消亡"
  - "[Phase 05-05]: TestInputRateLimit 回显计数模型——/bin/cat 默认 canonical+ECHO 使每送达帧产双份 'x'（行规即时回显 + cat 读后拷贝，1022/帧），ONLCR 只展开 '\\n' 不影响 'x' 计数；帧长 512B ≤ burst（AllowN 对 n>burst 恒 false 的硬约束）且 ≪ MAX_CANON 4096"
  - "[Phase 05-05]: 对照子测取显式 1MiB/1MiB 覆写（plan 授权『调大或不覆写默认』二选一）——整批 64KiB 一次性容纳零时序依赖，'x' 计数精确 == 发送量构成『丢弃确由限速器』的排他性证据"

patterns-established:
  - "限速测试宽界断言形态：小 rate/burst 覆写加速 + 发送量 25% 上界（达界需 ~15s 连续 refill，远超 1.5s 收集窗口）+ '>0' 下界（burst 首批必达）——令牌桶窗口滑动与调度迟滞的时序不确定性由宽界免疫，免 sleep 精算"
  - "存活双断言形态：洪水后 c.Ping(ctx) 收 pong（未被踢未 1011——accum goroutine 兼任库硬性要求的并发 reader）+ refill 后小量 marker INPUT 回显（输入路径未降权），连接级与路径级分离取证"
  - "依赖引入顺序纪律：先落码（import 存在）再 go get + go mod tidy——tidy 会移除无引用依赖（本 plan 首次 go get 后被 tidy 回收的实测教训）"

requirements-completed: [RES-02]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "INPUT 洪水（128 帧 × 512B = 64KiB，InputRate=1KiB/s + InputBurst=1KiB 覆写）：超限部分被丢弃（送达 'x' < 发送量 25%）+ 未超限部分送达（> 0，burst 容纳首批 2 帧）+ 连接存活（Ping 收 pong + refill 后 marker 回显）+ 服务端零退出"
    requirement: RES-02
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestInputRateLimit/超限丢弃且连接存活"
        status: pass
    human_judgment: false
  - id: D2
    description: "对照子测：InputRate/InputBurst 调大（1MiB/1MiB）下同量 INPUT 全量送达（'x' 计数精确 == 发送量 130816）——证明丢弃确由限速器而非 inputQ/其他路径"
    requirement: RES-02
    verification:
      - kind: integration
        ref: "internal/server/multi_test.go#TestInputRateLimit/对照大限额全量送达"
        status: pass
    human_judgment: false
  - id: D3
    description: "CR-01 完整背压修复：读循环 Master.Write 零残留（grep 证据，仅注释提及）+ echo 全链路（INPUT → inputQ → inputWriter → master → cat → OUTPUT 扇出）经 server 包既有测试 -race 全绿证明零回归"
    requirement: RES-02
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./internal/server/（全量绿，含 TestEchoPTY/TestMultiClientFanout/TestOwnerPolicy 等 INPUT 路径测试）"
        status: pass
    human_judgment: false

# Metrics
duration: 45min
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 05: 每客户端输入限速 + CR-01 完整背压修复 Summary

**RES-02 + CR-01 落地：x/time/rate v0.15.0 每客户端令牌桶（32KiB/s + 64KiB burst，超限丢弃不断开——R-02）+ 会话级 256KiB 字节有界 inputQ + 单 input-writer goroutine 独占 Master.Write——Attach 读循环同步写 PTY 的已知缺陷（CR-01，ttyd 同款）终结；TestInputRateLimit 双子测（超限丢弃/存活/送达 + 对照全量送达）与 server 包全量 -race 绿锁定行为。**

## Performance

- **Duration:** 45 min（含 ~12min 单次负载 flake 复现排查——14 次全量 + 3 次定向压力未复现，登记 deferred-items.md）
- **Started:** 2026-08-20T13:39:41Z
- **Completed:** 2026-08-20T14:25:04Z
- **Tasks:** 2
- **Files modified:** 5（4 修改 + 1 新建 deferred-items.md）+ multi_test.go（Task 2）

## Accomplishments

- **x/time v0.15.0 引入**（proxy 形态——`go get golang.org/x/time@v0.15.0` 网络直达，未动用 v0.10.0 模块缓存 fallback）：go.mod 新 direct 依赖 + go.sum 校验链入库；钉版 rationale 注释落 clients.go 引入点（review MEDIUM 处置——rate API 自 2015 年签名稳定，升级经 go.sum 审计链显式进行）
- **per-client limiter**（clients.go client.limiter 字段 + server.go 升档构造 `rate.NewLimiter(rate.Limit(s.inputRate), s.inputBurst)`）：默认 32KiB/s + burst 64KiB（R-01 参数表——击键 ~10B/s、快粘 ~50KB 瞬时由 burst 容纳、持续 32KiB/s 远超合法远低于洪水）；ro 客户端同样构造（无害——INPUT 先过 mode 门）
- **inputQ 会话级字节有界队列**（clients.go，outbox 同款形态）：256KiB（≥16 个 16KiB 满帧 ReadLimitPostAuth，defaultInputQueueBytes 常量挂 Phase 9 标定注释）；tryEnqueue 满则丢 + droppedInputs 计数内增（atomic，Phase 8 OPS-07 挂点注释）；入队零拷贝——coder/websocket Read 每次返回新分配 payload，持引用无别名风险（注释与 OUTPUT 方向 chunk 别名红线 P5-1 对照登记）
- **inputWriter 单 goroutine 独占 sess.Master.Write**（clients.go）：New 内与 ReadLoop/lifecycle 同批启动（server.go `go s.inputWriter()` 唯一装配点）；lifecycle 子进程退出路径 Drain→Close 解除在途写阻塞（runtime poller，D-12 同款机制）+ close(inputDone) 解除 select 等待——双通道收口零泄漏
- **INPUT 门链落码**（server.go）：mode 门（ro 静默丢，既有）→ `limiter.AllowN(time.Now(), len(data)-1)`（超限静默丢 + `s.inputDrops.Add(1)`——R-02：不断开不打逐次日志不踢出不降权，Allow godoc 官方 drop 语义 rate.go:113-115 注释逐字）→ `s.inputQ.tryEnqueue(data[1:])`（false → continue，计数已在 tryEnqueue 内）；**读循环内 `s.sess.Master.Write(data[1:])` 直写删除**——CR-01 收口的正向证据（grep 零残留，仅注释提及；Phase 2 的 O_NONBLOCK 最小缓解从未落地且不再需要，master fd 保持默认阻塞模式）
- **TestInputRateLimit 双子测**（VALIDATION 05-01-05，multi_test.go）：洪水子测三件套（送达 'x' > 0 / < 发送量 25% / Ping+pong 与 refill 后 marker 回显双存活断言）+ assertNoExit（洪水不拖死会话）；对照子测 1MiB/1MiB 大限额 'x' 计数精确 == 发送量（丢弃归因排他性证据）；-race 4 连跑（1+3）零失败

## Task Commits

Each task was committed atomically:

1. **Task 1: x/time 引入 + per-client limiter + 有界输入队列 + input-writer goroutine** - `ac1698a` (feat)
2. **Task 2: TestInputRateLimit——超限丢弃 + 连接存活 + 未超限送达** - `0cb775c` (test)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `internal/server/clients.go` - 文件头职责注释补 RES-02/CR-01；import x/time/rate（钉版 rationale 注释）；defaultInputQueueBytes 常量（256KiB，≥16 个 16KiB 满帧注释 + Phase 9 标定挂账）+ defaultInputRate/defaultInputBurst 注释消费点更新（05-05 已落地）；client.limiter 字段（R-01 参数表 + R-02 丢弃语义 + 并发安全注释逐字）；inputQ 类型组（tryEnqueue/dequeue/newInputQ，droppedInputs atomic 计数内增）；inputWriter goroutine（独占 Master.Write 注释逐字 + 生命周期双通道收口注释）
- `internal/server/server.go` - Server 加 inputQ/inputDone/inputDrops 字段（注释矩阵）；New 装配 inputQ/inputDone + `go s.inputWriter()` 唯一装配点 + New 文档注释两 goroutine→三 goroutine；Options/字段注释 InputRate/InputBurst 消费点已落地更新；升档序列 client 构造加 limiter；INPUT case 门链（mode → AllowN + inputDrops → tryEnqueue）+ 读循环 Master.Write 直写删除（CR-01 收口注释登记）；lifecycle Drain 后 close(s.inputDone) 收口 input-writer
- `internal/server/multi_test.go` - TestInputRateLimit 双子测 + accum 快照夹具（goroutine Read(context.Background()) + 互斥锁缓冲，Pitfall 2 竞速形态）；sync import
- `go.mod`/`go.sum` - golang.org/x/time v0.15.0 新 direct 依赖
- `.planning/phases/05-multi-client/deferred-items.md`（新）- 单次负载 flake 登记（见 Issues Encountered）

## Decisions Made

- **droppedInputs 计数收进 tryEnqueue 内部**：plan 两处规格（clients.go『满 → 丢 + droppedInputs 计数器递增』与 server.go『false → 计数递增 continue』）的调和——计数内增使队列自含记账（outbox bytes 同款形态），调用方 `if !tryEnqueue { continue }` 注释明示计数已内增；验收 grep 不钉计数位置，语义两全。inputDrops 为 Server 字段 atomic.Int64（INPUT 门每击键热路径无锁递增）——与 registry.kicks/gateTransitions（hubMu 内 plain int，判定本就在锁内）形成场景化选型。
- **input-writer 终结双通道**：lifecycle 内 Drain→Close 先关 master fd——在途 Master.Write 经 runtime poller 解除阻塞返回错误（inputWriter 写失败即 return）；close(inputDone) 解除 select 上的等待。两通道任一先到均收口；写失败先行 return 后 close(inputDone) 无害（幂等语义由 lifecycle 单次执行保证）。队列残余随会话消亡（子进程已退出，输入无意义）。
- **对照子测取显式 1MiB/1MiB 覆写**：plan 授权『InputRate/InputBurst 调大（或不覆写默认）』二选一——默认值（burst 64KiB）恰等于洪水总量属零裕度边界（第 128 帧 AllowN 恰好耗尽桶），显式大限额消去一切时序/边界依赖，'x' 计数精确 == 发送量（130816）构成『丢弃确由限速器而非队列/其他路径』的排他性证据（inputQ 256KiB ≫ 64KiB 洪水，对照路径不触队列上限）。
- **回显计数模型（测试设计）**：/bin/cat 默认 canonical+ECHO——每送达帧产双份 'x'（行规 ECHO 即时回显 + cat 读后 stdout 拷贝，每帧 511 'x' × 2 = 1022），ONLCR 只把 '\n' 展开为 \r\n 不影响 'x' 计数；帧载荷 511 'x' + '\n'（行长 512B ≪ MAX_CANON 4096 不触 canonical 行缓冲顶；帧长 ≤ burst 是 AllowN 可通的硬约束——n > burst 恒 false）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 依赖引入顺序修正：先落码后 go get**
- **Found during:** Task 1 步骤 1（plan action 顺序为先 `go get` 后落码）
- **Issue:** 按 plan 字面顺序 `go get golang.org/x/time@v0.15.0 && go mod tidy` 后 go.mod 无 x/time 条目——tidy 移除无引用依赖（此时无任何源码 import）
- **Fix:** 调整为先落码（clients.go/server.go import 与使用就位）再 `go get + go mod tidy`——依赖条目正确留存；钉版 v0.15.0 与 plan 一致，仅顺序修正
- **Files modified:** go.mod, go.sum（最终形态不变）
- **Verification:** grep 'golang.org/x/time' go.mod（1 行）go.sum（2 行）就位；go build/vet 退出 0
- **Committed in:** ac1698a

---

**Total deviations:** 1 auto-fixed（Rule 3 - Blocking；纯执行顺序修正，plan 锁定的机制形态零改动）
**Impact on plan:** 无——x/time 版本、引入形态（proxy）、钉版注释位置全部与 plan 一致。

## Known Stubs

本 plan 新增两个**计划内观测性 stub**（review #10 授权「采纳（stub 形态，原 plan 既有）」，均已入 WINDOWS.md 破窗台账）：

| Stub | 位置 | 消费点 |
|------|------|--------|
| `inputDrops`（限速丢弃计数） | internal/server/server.go（Server 字段，atomic.Int64） | Phase 8 OPS-07 进 metrics |
| `inputQ.droppedInputs`（队列满丢弃计数） | internal/server/clients.go（tryEnqueue 内增） | Phase 8 OPS-07 进 metrics |

两计数器只增不读是 plan 规格原文（计数器注释挂 Phase 8 metrics），非阻断性 stub。既有挂账项保持：registry.kicks/gateTransitions（同 Phase 8 OPS-07）、permission_denied 占位注释。

## Threat Model 处置

| Threat ID | 处置 | 证据 |
|-----------|------|------|
| T-05-03（输入洪水 DoS） | **mitigate 已落地** | 三层叠加：ReadLimit 16KiB 帧顶（P2 既有）+ per-client AllowN 丢弃（32KiB/s+64KiB burst，R-02）+ inputQ 256KiB 满则丢；TestInputRateLimit/超限丢弃且连接存活 子测三件套 + assertNoExit 锁定 |
| T-05-03b（输入写阻塞复活 CR-01） | **mitigate 已落地** | 单 input-writer goroutine 独占 Master.Write（server.go `go s.inputWriter()` 唯一装配点 grep == 1）；读循环 Master.Write 零残留（grep 证据——仅注释提及）；队列有界满则丢（不阻塞读循环）；Drain→Close 解除在途阻塞（D-12 机制）；server 包全量 -race 绿（echo 端到端无回归） |
| T-05-SC（x/time 依赖引入） | **mitigate 已落地** | golang.org/x/* 官方命名空间 + Go module proxy + sum.golang.org 校验（go.sum 两条目入库）；钉版 v0.15.0 + 钉版 rationale 注释落 clients.go 引入点；proxy 直达未动用 fallback；无 [ASSUMED]/[SUS]/[SLOP] 项 |

无新增威胁面（INPUT 路径改造在 plan 既有威胁模型边界内；无新网络端点/认证路径/文件访问形态；协议零改动——无 S→C 反馈帧，P2 D-01 类型空间纪律保持，review #5 延期处置不变）。

## Issues Encountered

- **全量 -race 单次失败未复现（负载 flake）**：Task 1 落地后首次全量运行 1 败（44.7s vs 典型 32s，尾部为 slow_consumer 1013 常规事件——指向 OUTPUT 洪水测试簇时序窗命中机器负载）；随后 14 次全量 + 3 次时序簇定向压力（TestGlobalCredit/TestSlowConsumerKick/TestSuccessionKickRace -count=3）全绿未复现。本 plan diff 对 OUTPUT/信用门/踢出路径零改动（该簇测试全程不发 INPUT，共享资源仅 CPU 调度），按越界纪律登记 `.planning/phases/05-multi-client/deferred-items.md` 不修复；若 CI 复现需单独排查 TestGlobalCredit 15s 开门窗口。
- **go mod tidy 回收无引用依赖**：见 Deviation 1——执行顺序修正，同时固化为 patterns-established 第三条（后续 phase 引入依赖复用）。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的占位与后续挂点）

- **README 明示（05-09）**：INPUT 限速丢弃语义与分段粘贴建议（review #5 缓解链第三腿）+ all 模式多写者输入交错不做排序承诺（ARCHITECTURE §2.9 screen 同款语义）
- **Phase 8 OPS-07**：inputDrops/droppedInputs 两计数器进 metrics（Known Stubs 表 + WINDOWS.md id 8/9）
- **Phase 9 标定回填**：inputRate/inputBurst/inputQueueBytes 初值（常量注释已挂账）
- **协议级反馈帧**：超限丢弃的用户侧运行期反馈若未来需要，须重新 discuss 动协议（review #5 延期——P2 D-01 类型空间纪律）

## Next Phase Readiness

- 05-06 分享链接：INPUT 门链与 token 绑定 mode 正交（mode 门在限速门之前，ro token 持有者 INPUT 仍在第一闸静默丢）；无阻塞项
- 05-07 max-clients：inputQ/input-writer 装配在 New（与 503 闸无交互）；Server 字段注释已预留 maxClients 消费点指向
- server/proto/pty/cmd 四包 -race 全绿；TestInputRateLimit -race 4 连跑零失败

## Self-Check: PASSED

- FOUND: internal/server/clients.go（`func (s *Server) inputWriter` == 1 + tryEnqueue 合计 5 处；`rate.NewLimiter` 字段注释 + rate.Limiter 类型引用；defaultInputQueueBytes == 256\*1024）
- FOUND: internal/server/server.go（`go s.inputWriter()` == 1 唯一装配点；`inputQ.tryEnqueue` == 1；AllowN == 1 + NewLimiter == 1；读循环 Master.Write 代码调用零残留——grep 命中全部为注释）
- FOUND: internal/server/multi_test.go（`func TestInputRateLimit` == 1，双子测；-race 1+3 连跑绿）
- FOUND: go.mod `golang.org/x/time v0.15.0` + go.sum 两条目
- FOUND: commit ac1698a（Task 1）、0cb775c（Task 2）均在 git log；两提交均无意外文件删除（--diff-filter=D 检查通过）
- 验收 grep 全过：x/time go.mod == 1 / go.sum == 2；rate.NewLimiter|AllowN 两文件合计 6（≥2）；func inputWriter|tryEnqueue clients.go == 5（≥2）；inputQ.tryEnqueue server.go == 1（≥1）；go s.inputWriter() server.go == 1（==1）
- go build/vet 退出 0；go test -race -count=1 ./internal/server/（35s 全绿）./internal/proto/ ./internal/pty/ ./cmd/wesh/ 全绿

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
