---
phase: 05-multi-client
plan: 04
subsystem: api
tags: [go, websocket, resize, arbitration, min-rect, debounce, write-policy, multi-client, race]

# Dependency graph
requires:
  - phase: 05-multi-client plan 03
    provides: 模式判定矩阵（decideModeLocked）/owner FIFO 递补升格（promoteNextLocked）/client.mode atomic 承载/prefs 双档；递补挂点已登记
provides:
  - resize.go 仲裁器：dims 类型 + arbitrate 纯函数（0 人零值哨兵/1 人 last-wins/≥2 min-rect，MULTI-04）+ arbiter（sizes/timer/last，hubMu 保护 R-07 延伸）
  - D-09 参与集矩阵逐字落码：owner 模式仅 owner / all 模式全部 rw 端 / 纯 ro 会话全部 ro 端 Hello 首尺寸 / 含可写端会话的 ro 旁观者不参与
  - 防抖/即时双通道：RESIZE 上报 50ms 单 time.Timer reset（Options.ResizeDebounce 覆写）；attach/detach/kick/递补升格即时重算不防抖
  - ro RESIZE 服务端忽略双闸：RESIZE case mode 闸（第一兜底层）+ reportResize 成员判定（第二兜底层），注释逐字登记 P2 D-13 修订来源
  - TestArbitrate 五行表测 + TestResizeArbitration 三子测 Getsize 集成（VALIDATION 05-01-04）
  - TestReadOnlyAllowsResize 经 D-09 适配（ro RESIZE 由放行改忽略，纯 ro 会话矩阵行锁定）
affects: [05-08 前端（ro 不发 RESIZE 第一闸 + 升格 fit() 尺寸纠正通道）, 05-09 README（纯 ro 会话运行期缩放裁剪明示）, Phase 9（ResizeDebounce 标定回填）]

# Actuals (#2632)
actuals:
  tokens: 9251
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "仲裁器形态：arbitrate 纯函数（0/1/N 分支）+ sizes map[*client]dims（仅参与集）+ 单 time.Timer reset 防抖 + last 变化检测——全部字段 hubMu 保护（注册表同锁 R-07），timer 回调自有 goroutine 取 hubMu（Go 1.23+ timer 语义 Reset 并发安全）"
    - "参与集供给矩阵（D-09）：participates(effMode) = rw 或 纯 ro 会话——owner 模式仅 owner 恒 rw、all 模式全部 rw 端、纯 ro 会话全部 ro 端 Hello 首尺寸；旁观者结构性排除"
    - "双通道重算：运行期 RESIZE → reportResize 防抖（timer.Reset 50ms）；attach/detach/kick/递补升格 → recalcNow 即时（无风暴风险）；目标尺寸变化才 sess.Resize（P5-3 同尺寸不发 SIGWINCH 且省 ioctl）"
    - "Getsize 集成断言通道：startResizeServer 加挂 sess（照抄 startTestServerWith 装配、不改其签名）+ ptySize 归一 (rows,cols)→(cols,rows) + pollSize 5s 轮询 100ms 步进"

key-files:
  created:
    - internal/server/resize.go
    - internal/server/resize_test.go
    - internal/server/resize_arb_test.go
  modified:
    - internal/server/server.go
    - internal/server/clients.go
    - internal/server/handshake_test.go

key-decisions:
  - "[Phase 05-04]: client.dims = Hello 首尺寸登记后运行期不更新——参与集成员最新尺寸由 arbiter.sizes 承载，本字段只服务递补升格时新 owner 参与集切换（D-09 尺寸接管源）；旁观者运行期 RESIZE 按 D-09 直接忽略不入账，缩窗后递补的瞬态偏差由 05-08 升格 fit() 纠正通道收口"
  - "[Phase 05-04]: kick 路径补 removeMember+recalcNow（plan 仅列 detach 挂点）——all 模式被踢 rw 端滞留 sizes 则其陈旧尺寸永久拖累 min-rect（幽灵成员把 PTY 压在离群者小窗口），成员移除与注册表移除必须同点恰好一次"
  - "[Phase 05-04]: 两测试分文件落位（resize_test.go 白盒 / resize_arb_test.go 黑盒）——Go 单文件单 package 约束使 plan『两测试同文件』字面不可达；VALIDATION 命名与运行命令逐字保持"
  - "[Phase 05-04]: 升档首尺寸经 addMember+recalcNow 在 hubMu 临界区内完成（Welcome 入队/登记之后、Broadcast 之前）——参与集 attach 即时重算不防抖，旁观者 attach 不重算；锁序 hubMu > sess.fdMu 全局一致"

patterns-established:
  - "Getsize 参数序双保险：creack/pty Getsize 原生返回 (rows, cols) 与 sess.Resize(cols, rows) 入参序相反——断言侧经 ptySize helper 单点归一，注释显式化反向关系（io_test.go:24-25 纪律的仲裁映射，review MEDIUM 处置）"
  - "忽略闸断言形态：动作（ro RESIZE）送达后 sleep(>防抖窗) 断言 Getsize 不变——若误受理早已应用，静默窗口即行为证据"
  - "ro 语义修订的测试迁移形态：保留测试名与夹具（文档锚点不漂），断言随 D-09 修订翻转（stty 不跟随），注释逐字登记修订来源（P2 D-13 → D-09）"

requirements-completed: [MULTI-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "all 模式异尺寸双客户端（132x43/80x24）PTY = min-rect 80x24（Getsize 实证）；B 断开后 2→1 恢复 last-wins 132x43"
    requirement: MULTI-04
    verification:
      - kind: integration
        ref: "internal/server/resize_arb_test.go#TestResizeArbitration/all模式min-rect与2to1恢复"
        status: pass
    human_judgment: false
  - id: D2
    description: "50ms 防抖合并：ResizeDebounce 覆写 200ms，100ms 内 3 次异尺寸 RESIZE → 窗口内 Getsize 未变、窗口后应用为最后上报值 120x50（单 time.Timer reset 合并）"
    requirement: MULTI-04
    verification:
      - kind: integration
        ref: "internal/server/resize_arb_test.go#TestResizeArbitration/防抖合并"
        status: pass
    human_judgment: false
  - id: D3
    description: "owner 模式参与集分层（D-09）：ro 旁观者 attach 不影响 PTY（132x43 而非 min）；ro 端 RESIZE 服务端忽略（Getsize 不变）；A 断开后 B 递补升格，PTY 切到新 owner Hello 登记尺寸 80x24"
    requirement: MULTI-04
    verification:
      - kind: integration
        ref: "internal/server/resize_arb_test.go#TestResizeArbitration/owner模式参与集与ro忽略闸"
        status: pass
    human_judgment: false
  - id: D4
    description: "arbitrate 纯函数：0 人零值哨兵 / 1 人 last-wins / 2 人 min-rect（含逐维独立取 min 行）/ 3 人极小矩形（含交换序无关行）/ 2→1 恢复 last-wins"
    requirement: MULTI-04
    verification:
      - kind: unit
        ref: "internal/server/resize_test.go#TestArbitrate"
        status: pass
    human_judgment: false
  - id: D5
    description: "纯 ro 会话矩阵行：Hello 首尺寸参与仲裁（111x44 生效）；运行期 ro RESIZE 被 D-09 第二闸忽略（stty 不跟随，TestReadOnlyAllowsResize 经修订适配）"
    requirement: MULTI-04
    verification:
      - kind: integration
        ref: "internal/server/handshake_test.go#TestReadOnlyAllowsResize"
        status: pass
    human_judgment: false

# Metrics
duration: 41min
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 04: resize 仲裁器（MULTI-04 + D-09 参与集分层）Summary

**MULTI-04 落地：arbitrate 纯函数（0 人零值哨兵/1 人 last-wins/≥2 min-rect）+ D-09 参与集矩阵逐字落码（owner 仅 owner / all 全部 rw / 纯 ro 全体 ro Hello 尺寸 / 旁观者不参与）+ 50ms 防抖与即时重算双通道 + ro RESIZE 服务端忽略双闸；TestArbitrate 表测与 TestResizeArbitration Getsize 集成三子测全量 -race 绿，ROADMAP 成功准则 3 前半（异尺寸 min-rect + 2→1 恢复）实证闭合。**

## Performance

- **Duration:** 41 min
- **Started:** 2026-08-20T12:41:53Z
- **Completed:** 2026-08-20T13:22:58Z
- **Tasks:** 2
- **Files modified:** 6（3 新建 + 3 修改）

## Accomplishments

- **arbitrate 纯函数落码**（resize.go，RESEARCH Code Examples 逐字形态）：0 人 → 零值 dims 哨兵（调用方 recalcNow 对零值不发起 Resize，不动 PTY 保持现状）；1 人 → last-wins；≥2 → 逐维独立 min(cols)×min(rows)（min-rect 不变量：任何参与端窗口 ≥ PTY 尺寸，各端多余面积留白，无 S→C 尺寸下发帧，D-09 推论）
- **arbiter 装配**：sizes map[*client]dims（仅参与集成员）+ 单 time.Timer（AfterFunc 创建即 Stop 为 stopped 态，首次 reportResize 的 Reset 才武装——server.go helloTimeout AfterFunc 先例，Don't Hand-Roll 纪律零 goroutine 计时循环）+ last 变化检测（目标尺寸变化才 sess.Resize，P5-3：同尺寸 TIOCSWINSZ 内核不发 SIGWINCH 且避免无谓 ioctl）；全部字段 hubMu 保护（R-07 单锁延伸），timer 回调取 hubMu（锁序 hubMu > sess.fdMu 全局一致）
- **D-09 参与集矩阵逐字落地**：participates(effMode) 判定——rw 端（owner 模式仅 owner / all 模式全部 rw 端）与无 --writable 纯 ro 会话全部 ro 端（Hello 首尺寸，否则会话冻结 80x24）参与；含可写端会话的 ro 旁观者结构性排除。升档序列 Hello 首尺寸入仲裁（addMember + recalcNow，attach 即时重算不防抖），client.dims 字段登记供递补切换
- **防抖/即时双通道**：RESIZE 上报 → reportResize（成员判定 → sizes 更新 → timer.Reset(50ms)，Options.ResizeDebounce 测试可覆写）；detach/kick/递补升格 → removeMember/addMember + recalcNow 即时重算（无风暴风险，RESEARCH Pattern 4）
- **ro RESIZE 忽略双闸**（D-09 第二闸 + 兜底层）：RESIZE case `cl == nil || cl.mode.Load() == proto.ModeRO → continue`（注释逐字登记『P2 D-13 ro 放行 RESIZE 为单客户端语境，已被 D-09 修订』）+ reportResize 成员判定兜底；第一闸（前端 ro 不发）由 05-08 落地
- **递补升格参与集切换**：promoteNextLocked 内 addMember(cand, cand.dims) + recalcNow——旧 owner 已在 detach/kick 移除，addMember 后 sizes 恒为 {新 owner} 单员，新 owner Hello 登记尺寸接管（TestResizeArbitration 实证 132x43 → 80x24 切换）
- **测试组**：TestArbitrate 五行表测（含逐维独立取 min 行与交换序无关行，同包白盒 tickets_test.go 先例）；TestResizeArbitration 三子测 Getsize 实证（all 模式 min-rect + 2→1 恢复 / 200ms 防抖合并窗口内外双断言 / owner 参与集 + ro 忽略 + 递补接管）；startResizeServer 局部 helper 加挂 sess（startTestServerWith 签名零改动，调用点零波及——checker W3 方案 b）；ptySize 归一 (rows,cols)→(cols,rows) 注释显式化（review MEDIUM 参数序陷阱双保险）
- **D-09 修订的测试迁移**：TestReadOnlyAllowsResize 第二 stty 断言由跟随（"50 120"）翻转为不跟随（"44 111"）——纯 ro 会话运行期 RESIZE 忽略行锁定；TestReadOnlyDropsInput 存活断言注释同步修订

## Task Commits

Each task was committed atomically:

1. **Task 1: resize.go 仲裁器 + server/clients 挂点 + TestReadOnlyAllowsResize D-09 适配** - `916e09c` (feat)
2. **Task 2: TestArbitrate 表测 + TestResizeArbitration Getsize 集成** - `7139efa` (test)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `internal/server/resize.go`（新）- dims 类型（ClampDim 已钳注释）；arbitrate 纯函数（MULTI-04/D-09 注释引）；arbiter 结构（sizes/timer/last）；initArbiter（timer stopped 态装配）；reportResize（D-09 第二闸兜底层 + 防抖 reset）；addMember/removeMember（成员供给注释矩阵）；recalcNow（即时重算 + P5-3 变化才 Resize + 参数序注释锁定）；participates（D-09 矩阵判定）
- `internal/server/server.go` - Server 加 arbiter 字段（hubMu 保护注释）；New 调 initArbiter（goroutine 启动前）；升档序列移除直调 sess.Resize、client 构造加 dims 字段、登记后按参与集规则 addMember+recalcNow（注释矩阵逐字）；RESIZE case ro 忽略闸 + hubMu 内 reportResize；Options/参数字段注释更新 ResizeDebounce 消费点已落地
- `internal/server/clients.go` - client.dims 字段（Hello 首尺寸登记注释）；detach/kickSlowConsumerLocked 各补 removeMember+recalcNow（kick 补挂点理由注释：幽灵成员拖累 min-rect）；promoteNextLocked 参与集切换（addMember 新 owner + recalcNow，sizes 恒单员论证注释）+ 05-04 挂点注释转落地表述；defaultResizeDebounce 注释消费点更新
- `internal/server/handshake_test.go` - TestReadOnlyAllowsResize 经 D-09 适配（文档注释/RESIZE 段注释/第二 stty 断言翻转 + 忽略证据注释）；TestReadOnlyDropsInput 存活断言注释修订
- `internal/server/resize_test.go`（新）- TestArbitrate 五行表测（package server 白盒）
- `internal/server/resize_arb_test.go`（新）- TestResizeArbitration 三子测（package server_test 黑盒）+ startResizeServer/ptySize/pollSize/sendResize 四 helper

## Decisions Made

- **client.dims = Hello 首尺寸登记、运行期不更新**：参与集成员的最新尺寸由 arbiter.sizes 承载（reportResize 更新）；ro 旁观者的运行期 RESIZE 按 D-09 直接忽略、不入任何账——plan must_haves 字面（『Hello 首尺寸登记』+『直接忽略』）。推论：旁观者缩窗后被递补时以 Hello 登记尺寸接管，若实际窗口已小于该尺寸则瞬态违反 min-rect（新 owner 窗口 < PTY）——纠正通道 = 05-08 升格处理 fit()（触发 sendResize 立即上报实际尺寸）；此瞬态与 RESEARCH A3 已接受的纯 ro 运行期裁剪风险同类，已登记遗留。
- **kick 路径补 removeMember+recalcNow（Deviation 1）**：plan 仅列 detach 挂点，但 all 模式下被 1013 踢出的 rw 端是参与集成员——滞留 sizes 则幽灵成员陈旧尺寸永久拖累 min-rect。成员移除与注册表移除同点恰好一次，detach/kick 两路径对称落码。
- **两测试分文件（Deviation 2）**：plan 要求 TestArbitrate（package server 白盒）与 TestResizeArbitration（package server_test 黑盒）同落 resize_arb_test.go——Go 单文件单 package 约束下字面不可达（白盒访问不导出类型、黑盒复用 e2e helper 不可兼得）。拆分 resize_test.go（白盒）/ resize_arb_test.go（黑盒），VALIDATION 05-01-04 命名与运行命令逐字保持。
- **验收 grep 驱动的注释压缩**：RESIZE case 的 D-09 修订注释初版 3 行使 `proto.ModeRO` 判定落在 `grep -A3` 窗口外——压缩为 2 行注释使闸在验收窗口内，修订来源逐字登记语义不变。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] kick 路径补 removeMember + recalcNow（防幽灵成员拖累 min-rect）**
- **Found during:** Task 1 挂点落码（kickOrCreditLocked 分工推演：all 模式下 rw 端可被 1013 踢出——『剔除 c 后仍存在未 blocked 的可写端』分支）
- **Issue:** plan Task 1 action 3 仅列 detach 的 removeMember 挂点；all 模式被踢 rw 端是参与集成员，若滞留 sizes，其陈旧尺寸将永久参与 min-rect——PTY 被压在已离场的离群者小窗口（渲染正确性静默损坏，正是 prohibitions 禁止的『仲裁把 PTY 压出参与端窗口』形态）
- **Fix:** kickSlowConsumerLocked 在 removeLocked 后对称补 removeMember + recalcNow（owner 模式被踢者为 ro 旁观者非成员，removeMember 幂等 no-op；注释登记论证）
- **Files modified:** internal/server/clients.go
- **Verification:** server 包全量 -race 绿；TestResizeArbitration owner 子测递补路径（detach 移除）与既有 kick 测试零回归
- **Committed in:** 916e09c

**2. [Rule 3 - Plan 字面不可达] 两测试分文件：Go 单文件单 package 约束**
- **Found during:** Task 2 测试设计（TestArbitrate 需 package server 白盒访问不导出 dims/arbitrate；TestResizeArbitration 需 package server_test 黑盒复用 dialHello/assertNoExit 等 e2e helper）
- **Issue:** plan 要求两测试同落 resize_arb_test.go 且验收 `grep -c 'func TestArbitrate|func TestResizeArbitration' resize_arb_test.go == 2`——单文件只能有一个 package 子句，两测试的包要求互斥，字面结构性不可达
- **Fix:** 拆分为 resize_test.go（package server，TestArbitrate）与 resize_arb_test.go（package server_test，TestResizeArbitration + 四 helper）；两文件合计 grep == 2，VALIDATION 05-01-04 的测试命名与运行命令（`go test -race -run 'TestArbitrate|TestResizeArbitration' ./internal/server/`）逐字保持
- **Files modified:** internal/server/resize_test.go, internal/server/resize_arb_test.go
- **Verification:** 两测试 -race 3 连跑绿；验收断言除单文件 grep 外全部满足（该条以拆分形态等价满足，本条即登记）
- **Committed in:** 7139efa

**3. [Rule 3 - Blocking] TestReadOnlyAllowsResize/TestReadOnlyDropsInput 经 D-09 适配**
- **Found during:** Task 1 前置勘察（handshake_test.go:354 第二 stty 断言 "50 120" 建在 P2 D-13『ro 放行 RESIZE』语义上）
- **Issue:** D-09（plan must_haves 逐字登记）将 ro RESIZE 由放行修订为忽略——不迁移则 Task 1 落地后该测试必红，plan success criteria『全量 -race 绿』结构性不可达；plan files_modified 未列测试文件（漏估适配面）
- **Fix:** 保留测试名与夹具（Phase 2 文档锚点不漂），第二 stty 断言翻转为 "44 111" 不跟随（忽略证据：RESIZE 已送达，若放行 stty 必读出 "50 120"）；文档注释逐字登记修订来源（P2 D-13 → D-09）；TestReadOnlyDropsInput 的『RESIZE 放行』存活断言注释同步修订（行为不变——忽略 ≠ 关连接）
- **Files modified:** internal/server/handshake_test.go
- **Verification:** 适配后测试 -race 绿（首个 stty "44 111" 由纯 ro 参与集仲裁路径达成——Hello 首尺寸生效语义同时锁定）；server 包全量 -race 绿
- **Committed in:** 916e09c

---

**Total deviations:** 3 auto-fixed（1 Rule 1 - Bug，2 Rule 3 - Blocking/Plan 字面不可达）
**Impact on plan:** 三处修正全部服务于 plan 自身 truths/prohibitions/验收（min-rect 不变量防幽灵成员、全量绿、VALIDATION 命名保持），未改动 plan 锁定的机制形态（arbitrate 逐字/参与集矩阵/双通道/双闸全部保持）；测试拆分与适配均在本 plan 授权语义内，无 scope creep。

## Known Stubs

None — 本 plan 无新增占位 stub。既有挂账项保持：registry.kicks/gateTransitions 计数器（Phase 8 OPS-07 消费）、permission_denied 占位注释（CONTEXT 裁断不硬用）。

## Threat Model 处置

| Threat ID | 处置 | 证据 |
|-----------|------|------|
| T-05-11（RESIZE 风暴 DoS） | **mitigate 已落地** | 50ms 单 time.Timer reset 合并（resize.go reportResize）+ ClampDim [1,1000] 既有钳制 + 目标尺寸变化才 Resize（P5-3）；TestResizeArbitration/防抖合并 子测双断言（窗口内未应用 + 窗口后应用为最后值）锁定 |
| T-05-11b（仲裁撑出客户端窗口） | **mitigate 已落地** | min-rect 不变量（≥2 一律 min(cols)×min(rows)，TestArbitrate 含逐维独立取 min 行）+ D-09 参与集分层（旁观者不参与不拖累，owner 子测锁定）；kick 路径幽灵成员防线（Deviation 1）；prohibitions 已在 resize.go/clients.go 注释登记 |

无新增威胁面（RESIZE 路径与 PTY 尺寸属 plan 既有威胁模型边界内；无新网络端点/认证路径/文件访问形态）。

## Issues Encountered

- **plan 双包同文件字面缺陷**：见 Deviation 2——planner 未察觉 Go 单文件单 package 约束；拆分后两测试各自落位反而更贴合职责（白盒表测随 resize.go、黑盒集成随断言通道）。
- **验收 grep 与注释篇幅的调和**：见 Decisions 第四条——`-A3` 窗口硬约束下压缩注释行数，修订来源逐字登记语义完整保持。
- **旁观者缩窗后递补的瞬态 min-rect 偏差**：见 Decisions 第一条与遗留事项——plan 字面（Hello 首尺寸登记）的直接推论，纠正通道归 05-08。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的占位与后续挂点）

- **前端第一闸与升格纠正通道（05-08）**：前端 ro 形态不发 RESIZE（D-09 第一闸，省上行流量）；升格 Welcome 的 rw 分支处理含 fit()（触发 sendResize 上报实际尺寸）——旁观者缩窗后递补的瞬态尺寸偏差由此收口
- **README 明示（05-09）**：纯 ro 会话旁观者运行期窗口缩放不上报（D-09 省流量裁决），缩到小于 PTY 尺寸者看到裁剪画面、重新 attach 恢复（RESEARCH Pattern 4 行为推论/A3）
- **ResizeDebounce 标定（Phase 9）**：50ms 初值经 PITFALLS Pitfall 10 论证（中间尺寸合并正是防抖设计意图）；review MEDIUM『vim/htop 场景或过激、建议可配置』延期处置——flag/配置收口归 Phase 7 OPS-09（CONTEXT deferred 锁定），Options.ResizeDebounce 测试可覆写已在
- **review #4 延期部分**：ro 端窗口 < PTY 尺寸的精确视觉裁剪指示需协议新增尺寸下发帧——与 D-09 推论直接冲突，采纳部分（05-08 console.info 一次性 ro 模式提示 + README 明示 + [ro] 标题前缀）不变

## Next Phase Readiness

- 05-05 限速 + CR-01 输入队列：RESIZE/INPUT 双门均为 per-client mode 判定形态，输入队列可挂同一读循环分派点；仲裁器与限速正交（互不改动对方路径）
- 05-08 前端：ro 不发 RESIZE 第一闸、升格 rw 分支 fit() 纠正——服务端侧双闸与递补接管已全部就绪并锁定
- 无阻塞项；server/proto/pty/cmd 四包 -race 全绿，新测试 -race 3 连跑零失败

## Self-Check: PASSED

- FOUND: internal/server/resize.go（func arbitrate == 1；reportResize/addMember/removeMember/recalcNow == 4 方法行；.Reset( == 1；time.AfterFunc == 1 唯一装配点）
- FOUND: internal/server/resize_test.go（func TestArbitrate == 1，5 个 t.Run 行）、internal/server/resize_arb_test.go（func TestResizeArbitration == 1，3 个子测；'rows, cols' 注释 == 4 处）
- FOUND: commit 916e09c（Task 1）、7139efa（Task 2）均在 git log
- 验收 grep 全过：arbitrate == 1、四方法 >= 4、resizeDebounce 两文件合计 11（>= 2）、RESIZE case -A3 含 ModeRO == 1、.Reset( == 1；两测试命名合计 == 2（拆分形态，Deviation 2 登记）
- go build/vet 退出 0；go test -race -count=1 ./internal/server/（32s 全绿）./internal/pty/ ./internal/proto/ ./cmd/wesh/ 全绿；TestArbitrate|TestResizeArbitration -race 3 连跑零失败

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
