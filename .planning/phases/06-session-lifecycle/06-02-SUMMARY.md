---
phase: 06-session-lifecycle
plan: 02
subsystem: api
tags: [websocket, go, lifecycle, signal, sighup, timer, concurrency]

requires:
  - phase: 06-session-lifecycle/06-01
    provides: EXIT 帧契约（'X'/ExitPayload/ExitFrame）+ lifecycle 写序安全广播形态（本 plan 的 exiting 门挂在该快照段）
  - phase: 05-multi-client
    provides: 注册表 removeLocked 两移除点（detach/kickSlowConsumerLocked）、hubMu 单锁纪律、arbiter AfterFunc 计时器先例、stall 夹具戒律（05-07）
provides:
  - pty.Session.SignalHangup()（linux/darwin 双平台同签名，syscall.Kill(-pid, SIGHUP) 进程组，P1 git 历史 cc03c79~1 逐字形态复活）
  - Options.ExitWhenEmpty bool + ExitWhenEmptyGrace time.Duration（D-14 set/grace 分离，grace=0 合法显式值，默认不开启）
  - maybeExitWhenEmptyLocked 注册表非空→空迁移触发（detach/kick 两调用点）+ 宽限计时器（AfterFunc + hubMu 复查）+ cancelExitEmptyTimerLocked 恰好一次取消点
  - lifecycle exiting 门（快照前置位，抑制广播期 detach 致空误触发）
  - emptyexit_test.go 六测 + io_test.go TestSignalHangup（accept-255 断言常量 -1）
  - OQ1 门裁决 accept-255：断开退出收口路径 wesh 进程退出状态 255（os.Exit(-1) 截断），下游消费点（06-06 S3/S4/S5 进程级断言、06-07 README 文案）按此落地
affects: [06-03, 06-04, 06-06, 06-07, phase-07]

actuals:
  tokens: 7362
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "注册表空触发 = 事件非状态：检测只挂 removeLocked 成功后的两移除点（detach/kick），启动期恒空天然免疫，严禁轮询（RESEARCH Pitfall 2）"
    - "宽限计时器 hubMu 纪律：启停全在 hubMu 内（两移除点启动 / registerLocked 成功后取消置 nil），AfterFunc 回调入内取 hubMu 复查『仍空且未 exiting』才动作（Pitfall 4 恰好一次兜底，resize.go initArbiter 同款）"
    - "断开退出只发 SIGHUP 不调 exitf——exitf+sync.Once 单点收口零新分支（D-13 硬约束），终结由既有 lifecycle 单一路径到达"
    - "stall 夹具断言序：踢出触发前绝不 Read——先等结构性后果（exitf）再读关闭帧取证（05-07 戒律的 regression-proof 形态）"

key-files:
  created:
    - internal/pty/signal_linux.go
    - internal/pty/signal_darwin.go
    - internal/server/emptyexit_test.go
  modified:
    - internal/server/server.go
    - internal/server/clients.go
    - internal/pty/io_test.go

key-decisions:
  - "OQ1 确认门用户裁决 accept-255（2026-08-23）：--once/--exit-when-empty 收口路径 = 子进程被 SIGHUP 终结，exitf 以 -1 收口、wesh 进程退出状态 255——lifecycle 零分支改动（D-13 字面形态），与 D-09「信号死亡 exit_code=-1」语义同源；三处下游消费点（本 plan 测试断言常量 -1 / 06-06 phase06.mjs S3-S5 进程级 255 断言 / 06-07 README 明示文案）按裁决值单点落地"
  - "宽限取消点落位调和：plan 字面「registerLocked 尾部」不可达（registerLocked 是 registry 方法无 Server 视角，TestClientCountInvariant 直构造 registry 的白盒形态不可破）——实现为 clients.go cancelExitEmptyTimerLocked，由 Attach 升档序列在同一 hubMu 持有内、registerLocked 成功后调用（恰好一次语义不变）"
  - "TestExitWhenEmptyGraceCancel 静默窗修正：plan 字面 1.5s 时点早于旧 timer 到期 2s 不构成取消证据——延长越过到期点 +500ms 余量（按 parenthetical「旧 timer 若未取消此时已到期」语义）"
  - "TestExitWhenEmptyKickTrigger 断言序翻转：先 waitExit(-1)（夹具下子进程唯一致死路径 = kick→空迁移→SIGHUP 的结构性证据）再读 1013——反序会让 readUntilError 提前排空管道使踢出永不成立（05-07 登记戒律，本 plan 实测竞态 ~50% 翻 1000）"

patterns-established:
  - "事件型空触发检测形态：removeLocked 返回 true 之后、hubCond.Broadcast 之前，detach/kick 两调用点同款注释同位插入"
  - "生命周期门位：lifecycle hubMu 快照段取锁后立即置 exiting=true 再快照——终结广播期的一切 detach 致空被结构性抑制"

requirements-completed: [SESS-01, SESS-02]

coverage:
  - id: D1
    description: "SignalHangup 双平台送达语义：SIGHUP 进程组（负 pid）→ WaitStatus Signaled + Signal==SIGHUP（T-06-02a 缓解的可执行锁定）"
    requirement: SESS-01
    verification:
      - kind: unit
        ref: "internal/pty/io_test.go#TestSignalHangup（-race）"
        status: pass
    human_judgment: false
  - id: D2
    description: "立即形态（grace=0）：唯一客户端断开 → 迁移点直接 SIGHUP（无计时器）→ exitf 收 -1 恰好一次（accept-255 断言常量）"
    requirement: SESS-01
    verification:
      - kind: integration
        ref: "internal/server/emptyexit_test.go#TestExitWhenEmptyImmediate（-race）"
        status: pass
    human_judgment: false
  - id: D3
    description: "宽限三态：计时内 attach 取消（echo 存活 + 越过旧到期点静默 + 再断开重新计时收码）/ 到期仍空退出（到期前 100ms 不过早退出 + 到期收码）"
    requirement: SESS-02
    verification:
      - kind: integration
        ref: "internal/server/emptyexit_test.go#TestExitWhenEmptyGraceCancel + TestExitWhenEmptyGraceExpire（-race）"
        status: pass
    human_judgment: false
  - id: D4
    description: "kick 第二移除路径空触发：stall 端 1013 slow_consumer 踢出 → 注册表空 → SIGHUP → exitf -1"
    requirement: SESS-02
    verification:
      - kind: integration
        ref: "internal/server/emptyexit_test.go#TestExitWhenEmptyKickTrigger（-race ×6 稳定）"
        status: pass
    human_judgment: false
  - id: D5
    description: "exiting 门与计时器晚于 lifecycle：自然退出 exit 42 路径 EXIT{42}+1000+exitf(42) 恰好一次；宽限计时器 lifecycle 后到期不补发 SIGHUP、stderr 无触发事件行（review #5 吸收）"
    requirement: SESS-01
    verification:
      - kind: integration
        ref: "internal/server/emptyexit_test.go#TestExitWhenEmptyLifecycleGate + TestExitWhenEmptyTimerAfterLifecycle（-race）"
        status: pass
    human_judgment: false
  - id: D6
    description: "OQ1 门裁决落地：accept-255——测试断言常量 -1（exitf 捕获桩层），进程级退出状态 255 断言与 README 明示文案由 06-06/06-07 承接（参数化消费点，无需二次裁决）"
    requirement: SESS-01
    verification:
      - kind: other
        ref: "Task 1 用户裁决（2026-08-23 accept-255）+ 本文件记录 + STATE decisions"
        status: pass
    human_judgment: false

duration: 41min
completed: 2026-08-23
status: complete
---

# Phase 06 Plan 02: 断开退出服务端机制（SESS-01/02）Summary

**SESS-01/02 服务端语义核心落码：pty.SignalHangup（SIGHUP 进程组，P1 逐字形态复活）+ Options.ExitWhenEmpty（set/grace 分离）+ detach/kick 两移除点空迁移触发 + 宽限计时器恰好一次启停 + lifecycle exiting 门——exitf 单点收口零新分支，七测 -race 全绿；OQ1 门裁决 accept-255 落地断言常量 -1**

## 确认门结果（Task 1，checkpoint:decision）

用户裁决 **accept-255**（2026-08-23，execute 续跑 prompt 确认）——照单收 255，不再重议：

- --once/--exit-when-empty 收口路径：SIGHUP 杀子进程 → ExitCode()=-1（GOROOT exec_posix.go:155-157 语义）→ exitf(-1) → os.Exit(-1) 被 Unix 截断为退出状态 **255**
- lifecycle 零分支改动（D-13 字面形态）；与 D-09「信号死亡 exit_code=-1」语义同源
- 三处下游消费点按裁决值单点落地（review OQ1 阻塞链关切闭合）：本 plan Task 3 断言常量 **-1**（exitf 捕获桩收 ExitError ExitCode 原值，os.Exit 截断只在真实二进制出现）；06-06 phase06.mjs S3/S4/S5 断言进程级退出状态 **255**；06-07 README 明示「--once/--exit-when-empty 收口 = 子进程被 SIGHUP 终结，wesh 退出状态 255」

## Performance

- **Duration:** 41 min（含 KickTrigger 竞态排查；不含确认门等待）
- **Started:** 2026-08-23T05:11:49Z
- **Completed:** 2026-08-23T05:52:47Z
- **Tasks:** 3/3（Task 1 确认门 → accept-255；Task 2 机制实现；Task 3 七测）
- **Files modified:** 6（3 新建 + 3 修改）

## Accomplishments

- `internal/pty/signal_linux.go` / `signal_darwin.go`（新建，reap_* 同款构建标签纪律，同签名零平台分支）：`SignalHangup()` = `syscall.Kill(-s.Cmd.Process.Pid, syscall.SIGHUP)`——负 pid 进程组、setsid 使 pgid==子进程 pid（P1 git 历史 cc03c79~1 逐字形态复活，D-13）；错误全静默（ESRCH 幂等）；不触 Master fd 不取 fdMu，调用方可在 hubMu 内调用
- `server.go`：Options 加 ExitWhenEmpty/ExitWhenEmptyGrace（D-14 set/grace 分离——grace=0 合法显式值，New 仅负值钳 0，禁止 <=0 兜底吞显式 0；默认不开启 = 现状保持）；Server 加四字段（exitEmptyTimer/exiting 均 hubMu 保护）；lifecycle hubMu 快照段取锁后**先置 exiting=true 再快照**（review #5 行号序断言锁定）——广播 Close 引发的 detach 致空属正常终结序列，被门抑制
- `clients.go`：`maybeExitWhenEmptyLocked`（detach/kick 两移除点 removeLocked 成功后同位插入——事件=非空→空迁移，启动期恒空天然免疫）三守卫（!exitWhenEmpty || exiting || 非空）；grace==0 立即 logEvent+SIGHUP；grace>0 幂等重启 AfterFunc 计时器（回调捕获 remote，取 hubMu 复查『仍空且未 exiting』才发 SIGHUP——只发信号不调 exitf，D-13 零新 exitf 分支）；`cancelExitEmptyTimerLocked` 宽限取消点（Stop+置 nil 恰好一次），Attach 升档序列 registerLocked 成功后调用
- logEvent 三新事件 exit_when_empty / exit_when_empty_wait / exit_when_empty_cancel（code 恒 1000 收口桶，reason 区分语义；SEC-01 红线保持——token/ticket/凭据值永不入参）
- `emptyexit_test.go` 六测（黑盒，helper 零改动复用）+ `io_test.go` TestSignalHangup——七测全绿，四包 -race 零回归

## Task Commits

1. **Task 1: 确认门（OQ1 退出状态裁决）** — 无提交（checkpoint:decision，用户裁决 accept-255，结果记录于本文件与 STATE decisions）
2. **Task 2: SignalHangup + Options + 空触发与宽限计时器 + exiting 门** — `d4f6927` (feat)——四文件（signal_linux/signal_darwin 新建 + server.go/clients.go 修改）
3. **Task 3: emptyexit_test.go 六测 + TestSignalHangup** — `692075f` (test)

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS）

## Files Created/Modified

- `internal/pty/signal_linux.go`（新建）— SignalHangup（linux 构建标签）
- `internal/pty/signal_darwin.go`（新建）— SignalHangup（darwin 构建标签，同签名）
- `internal/server/server.go` — Options 两字段 + Server 四字段 + New 装配（负值钳 0）+ lifecycle exiting 门 + Attach 宽限取消调用点
- `internal/server/clients.go` — maybeExitWhenEmptyLocked + cancelExitEmptyTimerLocked 两新函数 + detach/kick 两调用点
- `internal/server/emptyexit_test.go`（新建）— TestExitWhenEmptyImmediate/GraceCancel/GraceExpire/KickTrigger/LifecycleGate/TimerAfterLifecycle 六测
- `internal/pty/io_test.go` — TestSignalHangup 追加

## Verification Evidence

- `go build ./... && go vet ./...` 退出 0
- Task 2 门禁：`go test -race -count=1 ./internal/server/ ./internal/pty/` 全绿（既有套件零回归零适配——默认不开启零漂移）
- Task 3 门禁：`go test -race -count=1 ./internal/server/ ./internal/pty/ ./internal/proto/ ./cmd/wesh/` 四包全绿（含七新测试）
- 七新测试 `-race -count=5` 复跑全 PASS（KickTrigger 修复后 ×6 稳定无 flake）
- Task 2 验收 grep 全过：SignalHangup 双平台 ==1、构建标签各 ==1、ExitWhenEmpty(server.go)=11≥4、maybeExitWhenEmptyLocked(clients.go)=4≥3（定义+两调用点+注释）、exitEmptyTimer=7≥3、exiting server.go=4/clients.go=5 均 ≥2、exit_when_empty=7≥3、termOnce.Do 非注释行 ==1（exitf 单点收口不变量保持）；lifecycle 体内行号序 `s.exiting = true`(32) < `range s.registry.set`(34)
- review #5 核读项：removeLocked 返回值语义 = 成员删除成功才 true（kick 路径消费正确）；maybeExitWhenEmptyLocked 调用点恰好两处（clients.go:511 kick 尾部 / :702 detach 尾部）且均在 removeLocked(c) 返回 true 之后
- Task 3 验收 grep：六测函数 ==6、TestSignalHangup ==1；新/触测试文件 GOROOT gofmt 全净

## Decisions Made

- **确认门 accept-255**（见上节）——断开退出路径 wesh 退出状态 255 定稿，下游三消费点参数化闭合
- **宽限取消点落位调和**（plan 字面不可达的机械调和）：registerLocked 是 registry 方法（无 Server 视角；TestClientCountInvariant 直构造 registry 的白盒形态钉死该接收者），取消点实现为 clients.go `cancelExitEmptyTimerLocked` 并由 Attach 升档序列在同一 hubMu 持有内、registerLocked 成功后调用——「登记成功后恰好一次取消」语义逐字保持
- **GraceCancel 静默窗按语义修正**：plan 字面「1.5s 时点静默」早于旧 timer 到期点 2s（若未取消此时也未到期）不构成取消证据——按 parenthetical「旧 timer 若未取消此时已到期」延长至越过到期点 +500ms 余量
- **KickTrigger 断言序翻转**（05-07 登记戒律的回归形态）：plan 字面先 assertKicked1013 会让 readUntilError 从 0 时点排空管道，踢出永不成立（实测 ~50% 子进程跑完 38.9MB 收 1000）——翻转为先 waitExit(-1)（本夹具下子进程唯一致死路径即 kick→空迁移→SIGHUP，结构性证据）再读 1013 取证；断言面（1013 + exit -1）零损失
- **syscall.Kill 逐字沿用**（P1 历史形态/plan 指定）而非 io.go 的 unix.Kill——两 API 在 linux/darwin 同语义，plan 字面优先

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 宽限取消点从「registerLocked 尾部」调和为 Attach 调用点**
- **Found during:** Task 2（clients.go 实现）
- **Issue:** plan 字面「registerLocked 尾部加宽限取消点」使用 `s.exitEmptyTimer`——但 registerLocked 是 `*registry` 方法无 Server 视角，字面形态不编译；且 registry 接收者被 TestClientCountInvariant 直构造白盒形态钉死不可改
- **Fix:** 取消逻辑落 clients.go `cancelExitEmptyTimerLocked(remote)`（Stop+置 nil+logEvent 恰好一次），由 server.go Attach 升档序列在同一 hubMu 持有内、`s.registry.registerLocked(cl)` 成功后调用；双侧注释登记调和缘由
- **Files modified:** internal/server/clients.go, internal/server/server.go
- **Verification:** 验收 grep（exitEmptyTimer clients.go=7≥3）+ TestExitWhenEmptyGraceCancel 行为锁定（宽限内 attach 取消实证）
- **Committed in:** `d4f6927`（Task 2 提交内）

**2. [Rule 1 - Bug] GraceCancel 静默窗延长至越过旧 timer 到期点**
- **Found during:** Task 3（测试设计核实）
- **Issue:** plan 字面「1.5s 时点 exitCh 静默」早于旧 timer 到期点（grace=2s）——未取消时该时点同样静默，断言空转
- **Fix:** 静默窗延长至 closedAt+grace+500ms（越过到期点 500ms 余量），测试注释登记算术修正
- **Files modified:** internal/server/emptyexit_test.go
- **Verification:** TestExitWhenEmptyGraceCancel -race 全绿（取消实证语义成立）
- **Committed in:** `692075f`（Task 3 提交内）

**3. [Rule 1 - Bug] KickTrigger 断言序翻转（stall 夹具戒律回归）**
- **Found during:** Task 3（-race 实测 ~50% flake：stall client close code = 1000, want 1013）
- **Issue:** plan 字面先 assertKicked1013——readUntilError 从 0 时点开始排空管道，outbox 永不写满则踢出永不成立（05-07 已登记的 stall 夹具戒律「踢出触发前绝不 Read」的回归形态）；竞态落败时子进程跑完 38.9MB 自然退出，客户端收 EXIT{0}+1000
- **Fix:** 翻断言序——先 waitExit(-1)（本夹具无人读取时子进程唯一致死路径 = outbox 满→1013 踢出→注册表空→SIGHUP，exitf(-1) 到达即踢出与空触发的结构性证据）再 assertKicked1013 取证关闭帧；测试注释登记戒律与实测
- **Files modified:** internal/server/emptyexit_test.go
- **Verification:** 修复后 -race ×6 连跑全 PASS（修复前 ~50% 失败率）
- **Committed in:** `692075f`（Task 3 提交内）

**4. [Rule 3 - Blocking] GOROOT gofmt 清零本 plan 触碰区自身 hunk**
- **Found during:** Task 2 提交前检查
- **Issue:** 我新增的 clients.go 三行注释 `//（` 排版 + server.go Options 结构体字段列需随 ExitWhenEmptyGrace 长字段重排
- **Fix:** GOROOT gofmt 修正自身 hunk（server.go -w 整文件仅我一处 hunk；clients.go 手工 Edit 三行注释——避免 -w 连带清零既有漂移段）；既有 clients.go 388/438/584 行漂移（06-01 已登记 deferred-items）未触碰
- **Files modified:** internal/server/server.go, internal/server/clients.go
- **Verification:** 触碰文件我所属 hunk gofmt -d 全净；build/vet/test 全绿
- **Committed in:** `d4f6927`（Task 2 提交内）

---

**Total deviations:** 4 auto-fixed（1 Rule 3 调和 + 2 Rule 1 测试修正 + 1 Rule 3 排版）
**Impact on plan:** 全部为语义保持型机械修正；断言面零损失（KickTrigger 翻序后断言集不变）。无范围蔓延。

## Threat Flags

None——威胁登记全部 plan threat_model 内闭环：T-06-02a（SIGHUP 目标恒为本会话进程组——SignalHangup 无外部 pid 输入，TestSignalHangup 锁定送达）、T-06-02b（计时器泄漏/双触发——hubMu 启停纪律 + 回调复查 + SIGHUP 幂等 + termOnce 兜底，GraceCancel/GraceExpire/TimerAfterLifecycle 三测锁定）、T-06-02c（启动期恒空免疫——检测只挂两移除点，结构性）、T-06-02d（logEvent 三要素红线保持）。零新依赖零安装（T-06-SC accept 不变）。无新增未登记面。

## Known Stubs

None——全链 wired：SignalHangup 真实送达（TestSignalHangup 实证）、空触发/计时器/取消点/门全部行为测试锁定，零占位零 mock。

## Issues Encountered

- **KickTrigger ~50% flake 排查**：经临时探针（lifecycle 快照 n=1 code=0 且无 kick 事件）定位为 stall 夹具戒律回归——readUntilError 提前排空管道使踢出条件不成立；原始 TCP 实测本机 loopback 不读取吸收量 ~3.9MiB ≪ 38.9MB 洪水，确认「无人读取时子进程结构性不可能跑完」，支撑断言序翻转为先 exitf 后读帧（deviation #3）。探针已全数还原（git checkout 单文件还原到 d4f6927 提交态），生产代码零残留改动。

## Next Phase Readiness

- **06-03（重连）**：无依赖交叉——本 plan 全部服务端面
- **06-04（CLI 投影）**：Options.ExitWhenEmpty/ExitWhenEmptyGrace 两字段 + New 装配已就位——--once 语法糖展开（--max-clients=1 --exit-when-empty=0）与 --exit-when-empty[=duration] 解析直传即可；冲突校验进 validateStartup 矩阵
- **06-06（UAT）**：phase06.mjs S3/S4/S5 进程级退出状态断言按门裁决 **255** 落地；logEvent 事件串 exit_when_empty/_wait/_cancel 可作 stderr 断言材料
- **06-07（README）**：明示文案按门裁决落地——「--once/--exit-when-empty 收口 = 子进程被 SIGHUP 终结，wesh 退出状态 255」+ --once 等价关系（--max-clients=1 --exit-when-empty=0）
- 关注点：无阻塞；stall 类测试的「踢出触发前绝不 Read」戒律本 plan 再次实测命中，后续 plan 涉及同类夹具时直接沿用翻序形态

## Self-Check: PASSED

- 全部 6 个产物文件 + 本 SUMMARY 落盘核实（FOUND ×7）
- 任务提交 d4f6927 / 692075f 在 git log 核实（FOUND ×2）；两提交 post-commit 删除检查均无文件删除、无遗留 untracked

---
*Phase: 06-session-lifecycle*
*Completed: 2026-08-23*
