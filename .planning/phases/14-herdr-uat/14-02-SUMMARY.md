---
phase: 14-herdr-uat
plan: 02
subsystem: testing
tags: [go-test, dual-mode, e2e, multi-client, fanout, owner-policy, max-clients, spawn-intent, race-detector, t-run]

requires:
  - phase: 14-herdr-uat
    plan: 01
    provides: newTestServer 小族四形态装配点（harness_test.go）+ startPerClientServerTrackedWithSpawn 姊妹变体 + D-01 执行机械 tracer 实证
  - phase: 13-resource-defense
    provides: per-client 机制全收口（teardown stop-signal 序列 / pcSupervisor / StopTimeout KILL 兜底）
  - phase: 11-per-client
    provides: perclient_test.go 探针族（waitPgroupESRCH/readPump/drainQuiet/accumFramesUntil/readSessionPid）
provides:
  - e2e_test.go 生命周期测双模式断言分叉表（断开=进程存活可重连 vs SIGHUP 杀进程组、重连=接回同进程（CORE-05）vs 全新进程新 pid——反转面显式成表）
  - multi_test.go 十一测双模式分叉表（fanout 逐字节一致扇出 vs 双标记串交叉断言、EXIT 广播 vs 私有化+他端零感知、MaxClients spawn-intent 口径程序序精确对照、owner 四测 D-03 未装配可证伪列）
  - TestSigwinchOnAttach D-03 列（attach 期显式 SIGWINCH shared-only 装配的可证伪锁——server.go:1098-1104 注释锚定）
  - 蓝本 owner 递补行归属修正证据（四测实在 multi_test.go 非 clients_test——D-02 偏差登记）
affects: [14-03, 14-04, 14-05, 14-06, 14-12]

actuals:
  tokens: 38310   # 153238 chars / 4 over 0da23bf..HEAD（两测试文件，估 65000 的 59%——模式化改造的重排开销低于 planner 低置信预估）
  tasks: 2
  commits: 2

tech-stack:
  added: []   # 零新依赖（T-14-SC 红线，go.mod/go.sum 零 diff）
  patterns:
    - "断开/重连语义分叉表（e2e 主干）：shared 列 v1.0 逐字 + per-client 列 waitPgroupESRCH/新 pid——pid 锚点经 newSessTestServer spawned 访问器（argv 零漂移）"
    - "双标记串交叉断言（fanout per-client 列）：先发标记落前缀使 accumPayload 精确比对翻红 + 双泵静默窗帧级累积 Contains（免疫标记跨帧切分）"
    - "D-03 未装配可证伪列（owner 四测/Sigwinch/Drain）：静默窗零 Welcome/零标记 + 行为零变化正向断言，否决 t.Skip"
    - "spawn-intent 口径（MaxClients per-client 列）：wesh_pty_spawn_total 与成功 attach 数程序序精确对照（metrics 通道）"

key-files:
  created: []
  modified:
    - internal/server/e2e_test.go
    - internal/server/multi_test.go

key-decisions:
  - "TestEchoPTY per-client pid 观测经 newSessTestServer sess 形态（spawned 访问器直取 Cmd.Process.Pid 作进程组锚点）——argv 两列同持 /bin/cat 零漂移；readSessionPid 需 sh 会改 shared argv 破坏逐字红线，故弃"
  - "TestSigwinchOnAttach D-03 可证伪面选型：shared 尾段整段复制进 per-client 升档会 s.sess nil-deref 被任意测捕获（粗粒度面免费覆盖）；本列专锁「误向既有会话补发信号」细粒度漂移面（c1 已武装处理器标记文件 2s 窗口缺席）+ c2 READY 回读自证新会话功能性"
  - "TestExitBroadcast per-client 列 B 自发触发退 3：B 的 sh 阻塞在 read x 无法 echo 存活标记——以「B 发一行触发自身 sh 退出收自己的 1000」作自有会话独立存活的行为化证明（shared 列无此面）"
  - "MaxClients spawn-intent 经 metrics 通道（wesh_pty_spawn_total == 2 拒绝点 / == 3 cE 后）而非 spawnedSessions 计数——tracked 形态丢弃 spawned 访问器，metrics 为 plan action 3 明示择一选项（churn 格 load_test.go 先例）"
  - "kick 子测 per-client 列同步边走 /healthz clients 2→1 轮询 + SlowDwell=500ms 覆写（14-01 TestSlowConsumerKick 勘误既定通道）；darwin skip 保持子测级覆盖两列（平台闸非模式闸，per-client darwin 面由 TestSlowConsumerKick per-client 列承载）"
  - "TestInputRateLimit 局部变量 mode→effMode 重命名（mode 循环变量遮蔽）——断言消息恢复逐字，per-client 说明移入注释，comm 自审归零"

patterns-established:
  - "Pattern 4: 同断言双跑的显式判定注释（TestDetach/TestInputRateLimit/早闸双通道/TestUnknownFrame1002）——「该行为与进程模型无关」判定 + 蓝本行号锚定 + 分叉本体归他测的防双写注记（T-14-04）"
  - "Pattern 5: owner 四测 D-03 列四段式（B 恒 rw + prefs 档照发 + 零升格 Welcome 静默窗 + INPUT 生效）——四测共享同一未装配语义的四个观测面，递补/归队/再递补各测锁各自分叉点"

requirements-completed: [PC-13]   # 共享 ID 门：herdr E2E 证据未齐，勾选归 phase 末收口 plan（14-01 先例延续）

coverage:
  - id: D1
    description: "e2e_test.go 生命周期测双模式断言分叉表（断开/重连语义两列显式成表，CORE-05 反转面可证伪）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/e2e_test.go#TestEchoPTY/mode=shared+per-client（断开=assertNoExit+重连 echo 逐字 / pgid ESRCH+新 pid）、TestDrainBeforeAttach、TestSecondClientAttach、TestExitCodePropagation、TestUnknownFrame1002、TestHelloWelcome、TestWelcomePrefs/mode=*"
        status: pass
    human_judgment: false
  - id: D2
    description: "multi_test.go fanout/EXIT 广播双模式分叉（shared=逐字节一致扇出 / per-client=双标记串交叉断言 + 私有化他端零感知）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/multi_test.go#TestMultiClientFanout/mode=shared+per-client、TestExitBroadcast/mode=shared+per-client"
        status: pass
    human_judgment: false
  - id: D3
    description: "owner 四测 per-client 列 D-03 显式断言未装配（B 恒 rw + 零升格 Welcome 静默窗可证伪 + 重连直接 rw + 零扇出给旁观端，零 t.Skip）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/multi_test.go#TestOwnerPolicy|TestAllPolicy|TestSuccession|TestSuccessionKickRace/mode=shared+per-client"
        status: pass
    human_judgment: false
  - id: D4
    description: "MaxClients per-client 列 spawn-intent 口径（503 闸先于 spawn——wesh_pty_spawn_total 程序序精确对照）+ :756 收编 newTrackedTestServer"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/multi_test.go#TestMaxClients503/mode=per-client/WS_满员_503_与_halfOpen_无泄漏与槽位释放（spawn_total==2/==3）+ kick_路径槽位释放（dwell 500ms + /healthz 轮询）"
        status: pass
    human_judgment: false
  - id: D5
    description: "TestSigwinchOnAttach D-03 列 + TestWelcomeSessionDims 尺寸语义分叉（恒等式 vs min-rect/会话尺寸 + 零升格推送）"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "internal/server/multi_test.go#TestSigwinchOnAttach/mode=shared+per-client、TestWelcomeSessionDims/mode=shared+per-client（B 120x40 vs 40x10、60x50 vs 60x43）"
        status: pass
    human_judgment: false
  - id: D6
    description: "shared 列零回归证据（D-02 红线）：期望值文案逐字存活 + 母本函数零 diff + 依赖面零 diff"
    requirement: PC-13
    verification:
      - kind: unit
        ref: "comm 自审：e2e 29/29 + multi 58/58 旧期望值文案逐字存活；git diff hunks 证明 e2e 母本区（原 118-443 行）零改动；go.mod/go.sum/ci.yml 零 diff"
        status: pass
    human_judgment: false

duration: 28min
completed: 2026-09-06
status: complete
---

# Phase 14 Plan 02: e2e/multi 双模式断言分叉表改造 Summary

**e2e_test.go 7 测 + multi_test.go 11 测双模式断言分叉表落地——断开/重连（CORE-05 反转面）、fanout 双标记串交叉断言、EXIT 私有化他端零感知、owner 四测 D-03 未装配可证伪列、MaxClients spawn-intent 程序序对照，50 个 mode= 子测试 -race 逐名绿，shared 期望值 87 条文案逐字零漂移**

## Performance

- **Duration:** 28 min
- **Started:** 2026-09-06T14:10:15Z
- **Completed:** 2026-09-06T14:38:31Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- **e2e_test.go 断开/重连语义分叉表（Task 1）**：TestEchoPTY 断开语义分叉（shared=进程存活可重连原断言逐字 / per-client=断开即 SIGHUP 杀进程组 pgid ESRCH）+ 重连语义分叉（shared=接回同进程 CORE-05 / per-client=全新进程新 pid）——pid 锚点经 newSessTestServer spawned 访问器直取（argv 两列同持 /bin/cat 零漂移，waitPgroupESRCH 既有探针零新写）；TestDrainBeforeAttach D-03 列（零 attach 零 spawn 可证伪）；TestSecondClientAttach fanout 分叉（c2 静默窗零 c1 标记）；TestExitCodePropagation exitf 面分叉（42 收口逐字 / 静默续跑）；TestUnknownFrame1002/TestHelloWelcome/TestWelcomePrefs 同断言双跑（蓝本 :378-379 协议守卫行口径，注释载明归属）
- **multi_test.go 十一测分叉表（Task 2）**：fanout 双标记串交叉断言（先发标记落前缀精确比对翻红 + 双泵 1s 静默窗帧级累积 Contains 免疫跨帧切分）；EXIT 广播反转面（属主私有 1000 + B 静默窗零帧零错误 + B 自发触发退 3 收自己的 1000）；TestSigwinchOnAttach D-03 列（shared-only 装配可证伪锁：c1 已武装处理器零信号 + c2 READY 功能性自证）；owner 四测 D-03 四段式（B 恒 rw + rw 档 prefs 照发 + 零升格/dims-push Welcome 静默窗 + INPUT 生效 + 重连直接 rw + 零扇出给旁观端，零 t.Skip）；MaxClients spawn-intent 口径（wesh_pty_spawn_total == 2 拒绝点 / == 3 cE 后程序序精确对照）+ :756 收编 newTrackedTestServer + kick 子测 per-client 分叉（SlowDwell=500ms + /healthz 轮询同步边）；TestWelcomeSessionDims 尺寸语义分叉（恒等式 vs min-rect：B 120x40 vs 40x10、60x50 vs 60x43）
- **验证矩阵**：50 个 mode= 子测试逐名 PASS（e2e 7×2=14 + multi 22 顶层 + 14 嵌套）；包 -race 全量两轮绿（132.3s/133.4s）；全仓 5 包 -race 绿；go vet 与 GOROOT gofmt（go1.26.3）零输出；go.mod/go.sum/ci.yml 零 diff（T-14-SC）
- **CI 时长增量登记**：包 -race 全量从 14-01 基线 ~104-106s 增至 ~132-133s（两轮实测）——增量 ~26-28s（multi 双跑的静默窗 + InputRateLimit/MaxClients 三子测全双跑为主贡献）；14-12 收口闸知悉 CI 时长预算（蓝本估算 3-4 min/leg，当前累计仍在界内）

## Task Commits

Each task was committed atomically:

1. **Task 1: e2e_test.go 生命周期测双模式断言分叉表改造（D-01/D-02）** - `6fb8019` (test)
2. **Task 2: multi_test.go 十一测双模式断言分叉表改造（D-01/D-02/D-03 + 蓝本归属修正）** - `5ab2eb9` (test)

**Plan metadata:** （见下方 docs 收口提交）

## Files Created/Modified

- `internal/server/e2e_test.go` - 7 测双模式改造：断开/重连语义分叉表（CORE-05 反转面）+ TestDrainBeforeAttach D-03 列 + exitf 面分叉 + 协议守卫同断言双跑；装配收编 newSessTestServer/newTestServer 小族
- `internal/server/multi_test.go` - 11 测双模式改造：fanout 双标记串交叉断言 + EXIT 私有化 + owner 四测 D-03 未装配列 + MaxClients spawn-intent 口径（:756 收编 newTrackedTestServer）+ 尺寸语义分叉

## Decisions Made

- **TestEchoPTY per-client pid 观测通道**：readSessionPid 需 sh argv（echo $$）——会改 shared 列 argv 破坏逐字红线，故经 newSessTestServer sess 形态的 spawned 访问器直取 Cmd.Process.Pid（setsid pgid==pid 不变量下即进程组锚点）；sess 形态因此提前获首个运行期消费方（原预期 14-04）
- **TestSigwinchOnAttach D-03 可证伪面分层**：粗粒度面（shared 尾段整段复制进 per-client）会 s.sess nil-deref 被任意 per-client 测捕获——免费覆盖；本列专锁细粒度漂移面「误向既有会话补发信号」（c1 已武装处理器的标记文件 2s 窗口缺席），并补 c2 新会话 READY 回读自证「出生即正确尺寸 ≠ 会话不可用」
- **TestExitBroadcast per-client 列的 B 存活性证明形态**：B 的 sh 阻塞在 `read x` 无法 echo 存活标记（perclient_test.go TestPerClientExitPrivate42 的 echo 形态不适用）——以「B 发一行触发自身 sh 退 3、经泵源读至自己的 CloseError 1000」证明自有会话独立存活且私有收口同链
- **MaxClients spawn-intent 观测通道择一**：tracked 形态（waitHandlers 同步边所需）丢弃 spawnedSessions 访问器——spawn 计数经 /metrics 的 wesh_pty_spawn_total（plan action 3 明示选项，churn 格先例）；拒绝点 ==2（被拒 dial 零 spawn，闸后置即翻红）+ cE 后 ==3（恰一次新 spawn）
- **kick 子测 per-client 列同步边**：/healthz clients 2→1 轮询（14-01 TestSlowConsumerKick 勘误既定通道——A 的字节进度与 B 的踢出结构性解耦）+ SlowDwell=500ms 覆写（默认 10s 恒在窗外）；A 保活读取防计数落 0；darwin skip 保持子测级覆盖两列（平台闸非模式闸，per-client darwin 面由 TestSlowConsumerKick per-client 列承载——14-01 已双平台绿）
- **TestInputRateLimit 遮蔽重命名**：内层 `mode`（dialHello 返回）遮蔽外层循环变量——重命名 effMode；断言消息恢复逐字原文案（comm 自审归零），per-client 权限来源说明移入注释

## Deviations from Plan

### Plan-text 与实测归属偏差（D-02 登记，非代码缺陷）

**1. [口径偏差] plan「八测」计数把 TestHelperProcess 子进程演员计入**
- **Found during:** Task 1
- **Issue:** e2e_test.go 有 8 个 `func Test` 声明，但 TestHelperProcess 是 `-test.run=TestHelperProcess` argv 守卫驱动的子进程演员（测试基建非生命周期测），包裹进 mode 循环属噪声且干扰演员契约
- **Resolution:** 实际改造 7 测（TestEchoPTY/TestDrainBeforeAttach/TestSecondClientAttach/TestExitCodePropagation/TestUnknownFrame1002/TestHelloWelcome/TestWelcomePrefs），演员保持原样——验收 grep 命中数口径为 7 实测（≠ plan 字面 8）
- **Files:** internal/server/e2e_test.go（演员零改动）

**2. [Rule 3 - 结构收编] TestEchoPTY/TestDrainBeforeAttach 为自装配（非 plan 所记「7 处普通装配调用」覆盖的形态）**
- **Found during:** Task 1
- **Issue:** plan read_first 记「8 测 7 处普通装配调用」，暗示全部测试经 startTestServer 族装配——实测两测为 pty.Start+server.New 直连自装配（TestEchoPTY 01-01 先于 killServer 纪律、TestDrainBeforeAttach 无 listener）；且 per-client 断开/重连分叉需 pid 观测面
- **Fix:** 两测经小族统一收编——TestEchoPTY 经 newSessTestServer（sess 形态第四变体，pid 锚点 + Cleanup 补齐泄漏面）、TestDrainBeforeAttach 同（原无 listener，shared 分支装配序列同构断言面零变化）；sess 形态因此提前获首个运行期消费方（原预期 14-04）——「特殊调用点仅 :756 一处」的 plan 预告随之修正为两处（:756 tracked + EchoPTY sess）
- **Files modified:** internal/server/e2e_test.go
- **Verification:** 7×2 mode= 子测试全绿；shared 断言逐字（Writable 基线 true 与原零值 Options 在零客户端形态结构性不可观测——14-01 TestPreHelloReadLimit 注记同款论证）
- **Committed in:** 6fb8019

**3. [D-02 登记] 蓝本 shared-only 行「owner 递补（clients_test 大部）」归属误记（plan 明示要求登记项）**
- **Found during:** Task 2
- **Issue:** 蓝本 PITFALLS :392 行把 owner 递补测记在 clients_test——实测 TestOwnerPolicy/TestAllPolicy/TestSuccession/TestSuccessionKickRace 四测全在 multi_test.go（clients_test.go 的 TestClientCountInvariant 为纯白盒 registry 计数测，归 14-06 收口核对）
- **Resolution:** 四测按 plan 要求在本文件完成 per-client D-03 列改造；误记登记于此（14-12 收口闸 diff 白名单审查应知悉蓝本行的归属修正）

**4. [Rule 1 - 文案归一] TestInputRateLimit 局部变量遮蔽重命名**
- **Found during:** Task 2
- **Issue:** 子测内 `c, mode := dialHello(...)` 的 mode 遮蔽外层模式循环变量（Go 合法但混淆且 vet 面临隐患）
- **Fix:** 重命名 effMode；断言消息先漂移后恢复逐字（comm 自审一度报 1 条，归零收口）；per-client 权限来源说明移入注释
- **Committed in:** 5ab2eb9

---

**Total deviations:** 2 auto-fixed（Rule 1/Rule 3）+ 2 口径/归属登记（plan 文本与实测偏差）
**Impact on plan:** 零范围蔓延——所有修复均在 plan 分叉表语义框架内（装配收编/变量重命名不触期望值）；口径偏差已如实登记供 14-12 收口闸核对。

## Issues Encountered

None——plan 预告的 CI 时长增量已实测登记（~26-28s，见 Accomplishments）。

## User Setup Required

None - no external service configuration required.

## Known Stubs

None——全部断言为可执行真断言，零 t.Skip（owner 四测区）、零 TODO/FIXME、零放宽形态（P11 和稀泥禁令逐分叉点核验：fanout/断开/重连/升格四族两列真值相反面均显式成表）。

## Next Phase Readiness

- e2e/multi 两文件完成 mode-mapped 改造——后续 14-03（shutdown/metrics 归一）、14-04（resize_arb，sess 形态已获运行期首证可直接消费）、14-05（handshake/keepalive/auth_e2e mode-agnostic 批）、14-06（proxy/basepath/customindex/events/log + clients_test 归属核对）可复用本批模式化形态（同断言双跑判定注释 / D-03 四段式 / 静默窗零 Welcome 可证伪列）
- PC-13 需求勾选按 shared-ID 门阻断延续（herdr E2E 证据未齐）——归 phase 末收口 plan
- TestReadLimitBoundary D-02 偏差（14-01）与本批 4 项登记均为 14-12 收口闸 diff 白名单的应知悉项
- CI 时长：包 -race 全量 ~132s（+26-28s @ 本批）——后续 14-03..06 批量推开将持续累积，14-12 应复核蓝本 3-4 min/leg 预算

## Self-Check: PASSED

- 2 个 key-files 在盘且为零新文件（纯改造）
- 2 个任务提交在库（6fb8019 / 5ab2eb9）
- frontmatter `status: complete` 在场
- 50 个 mode= 子测试 -race 逐名 PASS；包全量两轮绿（132.3s/133.4s）+ 全仓 5 包绿；go vet + GOROOT gofmt 零输出
- go.mod/go.sum/ci.yml 零 diff（T-14-SC）；shared 期望值 87 条文案逐字存活（e2e 29 + multi 58，comm 自审归零）

---
*Phase: 14-herdr-uat*
*Completed: 2026-09-06*
