---
phase: 12-per-client
verified: 2026-09-04T16:48:52Z
status: human_needed
score: 25/25 must-haves verified
behavior_unverified: 0 # 全部行为依赖型 truth 均有本次独立复跑的通过测试实证（Go 定向组 -race / phase12.mjs 两轮 / phase12-dom / phase06-dom / TestPerClientReconnectNewPid）
overrides_applied: 0
deferred: # 已登记的后续 phase 裁决项（非本 phase 失败）
  - truth: "默认 --ping-interval=5s 配置下 TCP 级停读客户端在 (停读+5s, 停读+10s] 被 1006 pong_timeout 先杀，dwell 1013 结构性后到（writeControl 5s 写超时 × pinger 单一 DeadlineExceeded 判读）"
    addressed_in: "Phase 13"
    evidence: "STATE.md:137 Blockers 登记「[Phase 12-04 发现 → Phase 13 裁决] pinger/dwell 竞态」；12-04-SUMMARY key-decisions 实测材料（detach 恰于 attach+10.0007s，reason=pong_timeout）；phase12.mjs S6 以公开 CLI flag --ping-interval=0 隔离取证，dwell 机制本身生产 10s 零覆写实证（本次复跑实测 10.3s）"
human_verification: # status: human_needed 的唯一来源——CR-01 修复的人工残余面（fixer 显式标注）
  - test: "在真实浏览器（Windows 工作站 Playwright 层）per-client 模式 attach 后拖窗放大/缩小往复，观察渲染尺寸是否即时跟随窗口（无折行错位、无 attach 时旧尺寸钳制）"
    expected: "渲染尺寸随窗口即时跟随 fit（jsdom D2e/D2f/D2g 三断言在真实浏览器成立：行数 24→30、RESIZE 载荷 {cols:98,rows:30}、90 字符单行不折）；重连后旧屏残影被 reset 清除、画面干净（SC3 浏览器半侧观感）"
    why_human: "Linux 开发机 headless 禁 Playwright（CODEBUDDY.md 双机拓扑约束）；浏览器观感属视觉面 grep/jsdom 无法覆盖；12-REVIEW-FIX 将 CR-01 标注为 *requires human verification*（残余人工面=真实浏览器 resize 观感），已登记归 Phase 14 pw 层——本项为登记确认，非新发现缺口"
---

# Phase 12: per-client 交互与背压语义 Verification Report

**Phase Goal:** per-client 模式下尺寸、输入、重连、慢客户端四类交互语义各归各会话——无仲裁、无串扰、重连即全新
**Verified:** 2026-09-04T16:48:52Z
**Status:** human_needed
**Re-verification:** No — initial verification

**验证对象**: 修复后最终代码库状态（HEAD=4fe1f75，含 a3365a4 CR-01 三层恒等式修复 + 56e0923 WR-01 S5 ping 隔离）。验证方式 = must_haves 对照实际源码锚点 + git diff 证据 + 本次独立复跑关键测试（非全量重跑——12-05 收口闸六段式与回归门已跑过全量均绿，本验证按指示抽查复跑）。

## Goal Achievement

### Observable Truths

判定基准 = ROADMAP Success Criteria 5 条（非协商契约）+ 5 个 PLAN frontmatter must_haves 合并（plan 只增不减）。合并后 25 条 truth：

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | SC1: per-client RESIZE 直通本会话 TIOCSWINSZ（[1,1000] 钳制 Decode 层既有 + 50ms 防抖共用 debouncer 保留） | ✓ VERIFIED | server.go:1216 `cl != nil && cl.pc != nil` 分支 → DecodeResize → resizeMu 内 pendingResize + resizeDeb.Reset(s.resizeDebounce)；resize.go:60-72 debouncer 共用件（arbiter 同件同源，无第二常量）；perclient.go:103-105 三字段 + :202-206 装配。本次复跑 TestPerClientResizePassthroughRW/RO PASS（-race） |
| 2 | SC1: 双客户端尺寸互不影响（A resize 后 B 的 PTY winsize 不变） | ✓ VERIFIED | TestPerClientResizeIsolation PASS（本次复跑，-race）；phase12.mjs S2b PASS（B 端 stty 仍 "28 90"，本次实跑） |
| 3 | SC1: 运行期线上零 'W' 约束帧；resize 仲裁/owner 递补/fan-out/信用门在 per-client 分支不装配 | ✓ VERIFIED | 源码：直通分支不调 recalcNow/pushSessionDimsLocked（server.go:1216-1231 分支体核实），arbiter 零值天然 no-op（kickSlowConsumerLocked 内 removeMember/recalcNow/promoteNextLocked 对零值 no-op，REVIEW 对抗性追踪核实）；TestPerClientResizeIsolation 静默窗零 'W' 断言 + phase12.mjs S2c PASS（A端额外W=0 B端额外W=0，本次实跑） |
| 4 | SC1/D-06/D-07: ro RESIZE 双闸配对放行（服务端第二闸 per-client 不生效 + 前端第一闸按模式位）；shared ro 丢弃闸逐字保留 | ✓ VERIFIED | server.go:1216 分支先于 D-09 第二闸（else 半侧 ro continue + reportResize 逐字保留，源码核实）；main.ts:348 `if (isRO && sessionMode !== 'per-client') return`；TestPerClientResizePassthroughRO PASS + phase12.mjs S3b（ro 直通 "55 133"）/S3d（shared 对照零新输出）PASS + jsdom D2b/D2d PASS（本次实跑） |
| 5 | SC2: ro 客户端 attach 后照常获得自己的独立进程 | ✓ VERIFIED | TestPerClientResizePassthroughRO（Writable:false 覆写 harness spawn 后 RESIZE 直通自己 PTY）PASS；phase12.mjs S3a/S4a（mode=ro 会话建立）PASS（本次实跑） |
| 6 | SC2: ro 键盘输入被服务端丢弃（对自身进程同样无效） | ✓ VERIFIED | server.go:1178 INPUT case `cl == nil \|\| cl.mode.Load() == proto.ModeRO → continue`（丢弃先于 inQ 入队——永不到达任何 PTY）；TestPerClientROInputDropped PASS（ro 半场零回显 + rw 对照半场 echo 正常，0602e0b labeled break 修复后半场可达）；phase12.mjs S4b PASS（本次实跑） |
| 7 | SC2: 每客户端输入限速保留（RES-02 drop 语义不踢不断） | ✓ VERIFIED | server.go:1181-1183 限速门既有保留；TestPerClientInputRateLimitKept PASS（-race）；phase12.mjs S4c PASS（120×15KiB 超速率 flood 后探针回显，本次实跑） |
| 8 | SC3: Welcome 恒携 session 键（"shared"\|"per-client" 恒序列化无省略，五生产调用点统一传 s.sessionMode） | ✓ VERIFIED | proto.go:123 `Session string json:"session"`（无 omitempty）+ proto.go:170 WelcomeFrame 第 5 参；五调用点逐点核实：perclient.go:251 / server.go:1099 / clients.go:612（afterDrain）/ clients.go:724（promoteNextLocked）/ resize.go:205（pushSessionDimsLocked）；TestWelcomeFrameSession + TestPerClientWelcomeSession PASS；phase12.mjs S1a/S1b PASS（双模式对照，本次实跑） |
| 9 | SC3: 客户端异常断线（1006）重连后获得全新进程（新 pid） | ✓ VERIFIED | TestPerClientReconnectNewPid PASS（本次单测复跑 -race 0.04s；服务端语义 Phase 11 已锁、本 phase 保持零改动——diff 核实该区域无触碰） |
| 10 | SC3: 浏览器旧屏残留经 terminal.reset() 清除（用户看到干净的新会话） | ✓ VERIFIED（jsdom 面） | main.ts:729-731 WELCOME 分支统一判断 `if (sessionMode === 'per-client') term.reset()`；phase12-dom.mjs D1a-f 6 check PASS（alt-screen 残影判别通道：1049l 弹回后旧 normal buffer 残影不复活 + 新内容完整），本次实跑 17/17。浏览器观感半侧按 D-13 归 Phase 14（见 Human Verification） |
| 11 | SC3: shared 模式与缺 session 键（旧服务端）永不 reset（互操作零漂移） | ✓ VERIFIED | main.ts:696-700 白名单解析（缺键静默保持 shared、非法值 warn）；reset 仅在 sessionMode==="per-client" 时执行（:729）；phase12-dom.mjs D3a-c PASS（剥键注入不 reset）+ phase06-dom.mjs 零修改重跑 40/40+2skip PASS（shared 零漂移，本次实跑） |
| 12 | SC3: 重连 reset 静默无提示——零新面板/文案 | ✓ VERIFIED | 源码：reset 判断无任何 overlay/文案分支；D1f PASS（重连 reset 静默零新面板，面板保持隐藏，本次实跑） |
| 13 | SC3: sessionMode 为 per-connection 状态，connect() 重置块同批清零（IN-01 口径） | ✓ VERIFIED | main.ts:549 `sessionMode = 'shared'`（connect() 重置块内，:552 sessionDims=null 同批） |
| 14 | CR-01 修复: per-client 渲染尺寸即时跟随 fit（sessionDims 恒等式维护——去重闸前同步 + refit 上报先行 + 镜像服务端 [1,1000] 钳制） | ✓ VERIFIED（jsdom 面；真实浏览器观感 → 人工验证项 1） | main.ts:363-365（`if (sessionMode === 'per-client') sessionDims = {cols: Math.min(cols,1000), rows: Math.min(rows,1000)}`，位于去重闸 :369 之前）+ :394 refit 内 sendResize 上报先行（:398-399 渲染 min 后算）；dist/index.html（a3365a4 同 commit 重建）压缩产物含完整修复形态（`Kp==="per-client"&&(dm={cols:Math.min(e,1e3),...})` 先于去重闸、refit 先 hm() 后 min）；phase12-dom.mjs D2e/D2f/D2g 三渲染断言 PASS（红态 16/17 → 绿态 17/17，REVIEW-FIX 记录；本次实跑 17/17） |
| 15 | SC4: trySend 失败后读循环闭包阻塞持帧——输出积压于内核缓冲、子进程写阻塞而非丢数据（ttyd pty_pause parity） | ✓ VERIFIED | perclient.go:312-372 ReadLoop 闭包 D-01 形态（trySend 失败 → hubMu 内停读点 → 放锁 select notFull/cl.done → 持帧期间零锁）；每帧 make+copy 保持（:319-321）；TestPerClientStallBlocksAndResumes PASS（seq 首值恒 1/严格 +1/末位连续零缺口）；phase12.mjs S5a PASS（34,888,896 字节算术步进严格 +1 连续，本次实跑） |
| 16 | SC4: outbox drain 至非满即恢复信号——自动续读（cap-1 信号量） | ✓ VERIFIED | clients.go:214 notFull 字段 + :219 newOutbox 初始化 + :255-258 drain() 尾部非阻塞发送；perclient.go:355-359 notFull 唤醒重试同一帧；测试实证同 #15 |
| 17 | SC4: 每次续读重置 dwell 计时——慢但在前进的客户端永不被踢（D-02 判据核心） | ✓ VERIFIED | perclient.go:365-371 续读点 dwellTimer Stop 置 nil（再停重新武装完整 dwell）；TestPerClientDwellNoKickWhileProgressing PASS（事件驱动 duty-cycle：单轮 0.5×dwell < dwell、3 轮累计 > dwell，非重置实现必翻车判别力内建，本次复跑 -race） |
| 18 | SC4/D-05: 停读/续读两点递增 registry.gateTransitions；metrics.go 零改动 | ✓ VERIFIED | perclient.go:328（停读点）/ :366（续读点）hubMu 内递增；export_test.go:62 GateTransitionsForTest；`git diff e8b39c0 -- internal/server/metrics.go` = 0 行（本次核实）；TestPerClientStallGateTransitions PASS |
| 19 | SC5: 停读态连续无恢复超过 dwell 阈值 → 1013 slow_consumer 踢出（kick 序列零改动复用） | ✓ VERIFIED | perclient.go:394-412 armSlowDwellLocked（AfterFunc 三件套 + 身份比对 + cl.done 早退 → kickSlowConsumerLocked）；clients.go:636-661 kick 序列（1013 wire/kicks++/detach 事件/teardown 挂点）既有零改动；TestPerClientDwellKick PASS（500ms 覆写）；phase12.mjs S6a-d PASS（生产 10s 零覆写真实到期，本次实跑实测停读至踢出 10.3s，1013 slow_consumer 机器串逐字 + ESRCH 收割） |
| 20 | SC5/D-03: dwell 阈值 = 10s 内部常量 + Options 测试可覆写 + 零 CLI flag/TOML 键 | ✓ VERIFIED | clients.go:89 `defaultSlowDwell = 10 * time.Second`；server.go:329 Options.SlowDwell + :410-413 New 零值兜底 + :490 Server.slowDwell 装配；`git diff e8b39c0 -- cmd/wesh/main.go` = 0 行（本次核实） |
| 21 | SC5: 踢出时服务端与其他客户端不受影响 | ✓ VERIFIED | phase12.mjs S5c PASS（B 端停读窗内+窗后 echo 照常）+ S6d PASS（踢出后 3s 护栏内进程组 ESRCH——会话终结挂点联动）；kickSlowConsumerLocked arbiter 空集 no-op（REVIEW 核实） |
| 22 | 12-05: 收口闸六段式全绿（静态面/全量 -race/darwin 双编译/web 构建/UAT 矩阵 16 轮/diff 审查） | ✓ VERIFIED | 12-05-SUMMARY 六段式全量落档（全部命令当日实跑 exit 0）；本次独立复跑关键段全过：gofmt 零输出 + go vet 零输出 + go build 成功 + Go 定向组 -race 全绿 + phase12.mjs 两轮 20/20 + phase12-dom 17/17 + phase06-dom 40/40+2skip（基线一致） |
| 23 | 12-05: 期望值逐字未动（diff 白名单审查，零断言放宽） | ✓ VERIFIED | 本次独立核对 `git diff e8b39c0`：删除行恰 8 行 = proto_test.go 4 处 WelcomeFrame 调用点 + clients_test.go 3 调用点 + 1 邻接注释行（断言行 t.Fatalf/t.Errorf 零触碰）；perclient_test.go +540/-0 纯新增；export_test.go +14/-0 append-only（WINDOWS.md #33 已登记补充项①）；既有 web/uat 脚本零修改（仅两新增文件） |
| 24 | 12-05: PC-05/06/07/10/11 五需求勾选 + Traceability 五行 Complete + ROADMAP Phase 12 完成标记 | ✓ VERIFIED | REQUIREMENTS.md:84-90 五条 [x] + :195-201 五行 Complete（本次核实）；ROADMAP Phase 12 条目 5/5 plans [x] + completed（本次核实） |
| 25 | 12-05: WR-01 闭合回指登记（D-04「dwell 涵盖不复刻」形态，含重开条件） | ✓ VERIFIED | 12-05-SUMMARY「WR-01 闭合回指」专段（登记原文逐字引用 + 闭合论证 + 验证证据 + 重开条件四件）；STATE.md:119/:124 双登记；代码侧注释双侧回指（perclient.go:297-305 闭包注释段 + clients.go defaultSlowDwell 注释段，源码核实） |

**Score:** 25/25 truths verified（0 present-behavior-unverified——全部行为依赖型 truth 均有本次复跑的通过测试）

### Deferred Items

已登记的后续 milestone phase 裁决项（非本 phase 失败、非 truth FAIL——SC5 dwell 机制本身经生产 10s 零覆写实证成立）：

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | 默认 --ping-interval=5s 下 TCP 级停读客户端被 1006 pong_timeout 先杀（writeControl 5s 写超时 × pinger 单一 DeadlineExceeded 判读交互），dwell 1013 结构性后到；真实浏览器网络栈自动回 pong 不触发、herdr 类自管 socket 客户端可触发 | Phase 13 | STATE.md:137 Blockers 登记「[Phase 12-04 发现 → Phase 13 裁决]」；12-04-SUMMARY 实测材料（detach 恰于 attach+10.0007s，reason=pong_timeout）；phase12.mjs S6/S5 以公开 CLI flag --ping-interval=0 隔离取证（WR-01 修复 56e0923 落地，本次实跑验证隔离有效） |

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/proto/proto.go` | WelcomePayload.Session（json:"session" 恒序列化）+ WelcomeFrame 第 5 参 | ✓ VERIFIED | :123 / :170；注释载 D-08 契约与 D-16 互指 |
| `web/src/main.ts` | sessionMode per-connection 变量 + WELCOME 分支 reset 判断 + sendResize 模式位闸 + CR-01 恒等式 | ✓ VERIFIED | :200 / :549 / :696-700 / :729-731 / :348 / :363-365 / :394；四层全锚点核实 |
| `web/uat/phase12-dom.mjs` | D1/D2/D3 jsdom 断言（min_lines 200） | ✓ VERIFIED | 587 行；D1a-f/D2a-g/D3a-c 全 check 本次实跑 PASS |
| `internal/server/perclient_test.go` | 12-01/02/03 新测 10 函数（contains TestPerClientWelcomeSession 等） | ✓ VERIFIED | 10 函数全部在场（:305/:1329-1806 区）；本次定向组 -race 全绿 |
| `internal/server/resize.go` | debouncer 共用件（contains newDebouncer） | ✓ VERIFIED | :60-72；arbiter 同件持用（:108） |
| `internal/server/server.go` | RESIZE per-client 直通分支（contains cl.pc != nil）+ Options.SlowDwell 三锚点 | ✓ VERIFIED | :1216 / :329 / :410-413 / :490 |
| `internal/server/perclient.go` | pcSession 防抖三字段（contains resizeDeb）+ dwellTimer + 阻塞持帧（contains dwellTimer） | ✓ VERIFIED | :103-105 / :115 / :312-372 / :394-412 |
| `internal/server/clients.go` | notFull 恢复信号 + drain 挂点 + defaultSlowDwell（contains notFull） | ✓ VERIFIED | :214 / :219 / :249-260 / :89 |
| `internal/server/export_test.go` | GateTransitionsForTest 观测出口 | ✓ VERIFIED | :62（ForTest 四件套纪律） |
| `web/uat/phase12.mjs` | 协议层 UAT 六场景（min_lines 400，contains S6） | ✓ VERIFIED | 762 行；S1-S6 六场景函数全在场；本次两轮实跑 20/20×2 |
| `web/dist/index.html` | dist 重建纳管（embed 链，含 CR-01 修复） | ✓ VERIFIED | a3365a4 同 commit 重建；压缩产物含三处 Phase 12 标记 + CR-01 同步逻辑完整形态 |
| `.planning/REQUIREMENTS.md` | PC-05/06/07/10/11 勾选（contains [x] **PC-05**） | ✓ VERIFIED | 五条 [x] + Traceability 五行 Complete |
| `.planning/ROADMAP.md` | Phase 12 计划清单勾选（contains 12-01-PLAN.md） | ✓ VERIFIED | 5/5 plans [x] + completed 日期 + Progress 表 |

全部 artifacts 存在（Level 1）、实质（Level 2，无 stub——每件均承载真实行为并经测试实证）、接线（Level 3，见下）。

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| internal/server/perclient.go | internal/proto/proto.go | upgradePerClient Welcome 组帧传 s.sessionMode（pattern `WelcomeFrame\(.*s\.sessionMode\)`） | ✓ WIRED | perclient.go:251 五实参形态命中 |
| web/src/main.ts | internal/proto/proto.go | WELCOME 分支解析 w.session → sessionMode 存储 → reset 判断（pattern `w\.session`） | ✓ WIRED | main.ts:696-700 解析 → :729 reset；phase12-dom D1/D3 行为实证 |
| internal/server/server.go | internal/pty（sess.Resize） | RESIZE per-client 分支 → 每会话 debouncer 回调 → sess.Resize 仅 fdMu（pattern `cl\.pc != nil`） | ✓ WIRED | server.go:1216 → perclient.go:202-206 回调（函数体 hubMu 零命中，源码断言过）→ sess.Resize；S2a stty 回读 "50 120" wire 实证 |
| web/src/main.ts | internal/server/server.go | sendResize 第一闸按 sessionMode 放行（D-07）↔ 服务端第二闸 per-client 不生效（D-06）注释互指（pattern `sessionMode !== 'per-client'`） | ✓ WIRED | main.ts:348 / server.go:1216；两侧注释互指核实；D2b/D2d + S3b/S3d 双向实证 |
| internal/server/perclient.go | internal/server/clients.go | 闭包 select 等 outbox.notFull / cl.done；dwell 到期 hubMu 内 kickSlowConsumerLocked（pattern `notFull`） | ✓ WIRED | perclient.go:355 / :408；四测实证 |
| internal/server/clients.go | internal/server/metrics.go | gateTransitions 停读/续读递增 → snapshotMetrics → wesh_credit_gate_transitions_total 零改动流经（pattern `gateTransitions\+\+`） | ✓ WIRED | perclient.go:328/:366 递增点 + metrics.go 零 diff；TestPerClientStallGateTransitions 实证 |
| web/uat/phase12.mjs | wesh 二进制（/tmp/wesh-uat/wesh） | spawn 真实二进制 + 原生 WebSocket 握手 + 帧级断言（pattern `startWesh`） | ✓ WIRED | 本次两轮实跑 20/20×2，真实 dwell 10.3s 实测 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| Welcome.session | s.sessionMode → WelcomeFrame → wire → w.session → sessionMode | Options.SessionMode（CLI flag --session-mode）装配期归一 | ✓ | S1a/S1b 双模式实测（per-client/shared 各自正确值） |
| RESIZE 直通 | pendingResize → debouncer 回调 → sess.Resize（TIOCSWINSZ） | 客户端 RESIZE 帧（DecodeResize 钳制后） | ✓ | S2a stty 回读 "50 120" / S3b "55 133"（经 SIGWINCH trap 回读，全链真实数据） |
| OUTPUT 背压链 | PTY → ReadLoop 闭包 make+copy → trySend/notFull → outbox → writer → wire | 子进程 stdout（seq 洪水） | ✓ | S5a 34,888,896 字节算术步进严格 +1 连续（零缺口 = 真实数据完整流经） |
| 渲染恒等式（CR-01） | fit → sendResize（同步 sessionDims）→ min(fit, sessionDims) → term.resize | 窗口物理尺寸（fit.proposeDimensions） | ✓ | D2e（行数 24→30）/ D2g（90 字符单行不折）DOM 实测 |

无 STATIC/DISCONNECTED/HOLLOW_PROP——四条关键数据流全部实证流动。

### Behavioral Spot-Checks

本次验证独立复跑（非转述 SUMMARY——全部命令本机实跑，Linux 开发机 headless，node 直跑禁 Playwright）：

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Phase 12 Go 新测定向组 12 测（-race） | `go test -race -run 'TestWelcomeFrameSession\|TestPerClientWelcomeSession\|TestPerClientResize\|TestPerClientROInput\|TestPerClientInputRate\|TestPerClientStall\|TestPerClientDwell' ./internal/proto/ ./internal/server/ -count=1` | ok proto 1.016s / ok server 16.856s，exit 0 | ✓ PASS |
| 重连新 pid 服务端语义（SC3 前提） | `go test -race -run 'TestPerClientReconnectNewPid' ./internal/server/ -count=1 -v` | PASS (0.04s) | ✓ PASS |
| jsdom 前端三场景（D1/D2/D3 含 CR-01 三断言） | `node web/uat/phase12-dom.mjs` | 17/17 DOM 断言通过，6.567s，exit 0 | ✓ PASS |
| 协议层 UAT 六场景 轮一 | `node web/uat/phase12.mjs` | 20/20 协议断言通过，26.203s | ✓ PASS |
| 协议层 UAT 六场景 轮二（基线一致性） | `node web/uat/phase12.mjs` | EXIT=0，20 PASS 行，零 FAIL 零 skip；S6b 实测停读至踢出 10.3s（≥10s 生产值）；S5a 校验字节=34888896 | ✓ PASS |
| shared 前端零回归（零修改重跑） | `node web/uat/phase06-dom.mjs` | 40/40 + 2 skipped（平台豁免，带 reason），与 12-05 基线逐字一致 | ✓ PASS |
| 静态面 | `$(go env GOROOT)/bin/gofmt -l .` + `go vet ./...` + `go build ./...` | gofmt 零输出 / vet 零输出 / build 成功 | ✓ PASS |

### Probe Execution

本项目无 `scripts/*/tests/probe-*.sh` 形态探针——项目验证面为 `web/uat/phaseNN.mjs` 协议层脚本与 Go 测试（CODEBUDDY.md 分层测试策略），均已在上表 Behavioral Spot-Checks 中实跑。等效探针记录：

| Probe | Command | Result | Status |
| ----- | ------- | ------ | ------ |
| `web/uat/phase12.mjs` | `node web/uat/phase12.mjs` | 两轮 20/20 exit 0（含真实 10.3s dwell 到期 1013 + 34.9MB 字节连续 + ESRCH 收割） | PASS |
| `web/uat/phase12-dom.mjs` | `node web/uat/phase12-dom.mjs` | 17/17 exit 0 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| PC-05 | 12-02, 12-04 | RESIZE 直通本会话 TIOCSWINSZ（钳制/防抖保留），无仲裁器、无 'W' 约束帧 | ✓ SATISFIED | Go 3 测（RW/RO/Isolation）+ S2/S3 + D2 三证据链（本次全部复跑绿）；REQUIREMENTS.md:84 [x] |
| PC-06 | 12-01, 12-04, 12-05 | 重连全新进程 + 前端按模式位 terminal.reset() 清屏 | ✓ SATISFIED | proto 2 测 + TestPerClientReconnectNewPid + S1 + D1/D3（本次全部复跑绿）；REQUIREMENTS.md:85 [x]；浏览器观感半侧 → 人工验证项 1 |
| PC-07 | 12-02, 12-04 | ro 独立进程 INPUT 门控 + 每客户端输入限速保留 | ✓ SATISFIED | Go 2 测（ROInputDropped/InputRateLimitKept）+ S4（本次复跑绿）；REQUIREMENTS.md:86 [x] |
| PC-10 | 12-03, 12-04 | 慢客户端 outbox 写满 1013 踢出（自然反压停读） | ✓ SATISFIED | Go 2 测（DwellKick/StallGateTransitions）+ S6 真实 10s+（本次复跑绿）；REQUIREMENTS.md:89 [x]；默认 ping 配置竞态 → Deferred #1（Phase 13） |
| PC-11 | 12-03, 12-04 | 停读/续读背压（ttyd pty_pause/resume parity），持续过载按 PC-10 踢出 | ✓ SATISFIED | Go 2 测（StallBlocksAndResumes/DwellNoKickWhileProgressing）+ S5 字节级连续（本次复跑绿）；REQUIREMENTS.md:90 [x] |

无孤儿需求——REQUIREMENTS.md 映射到 Phase 12 的 5 个 ID 与 5 个 PLAN frontmatter requirements 字段完全一致（12-01: PC-06；12-02: PC-05/07；12-03: PC-10/11；12-04/05: 全 5）。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| web/uat/phase12.mjs | 64 | `skip` 帮助函数定义后零调用（死代码） | ℹ️ Info | IN-01（12-REVIEW 登记）；fix_scope=critical_warning 不含，留待 Phase 13/14 裁决——不阻塞 |
| web/uat/phase12-dom.mjs | 65 | 同上 | ℹ️ Info | 同 IN-01 |
| web/uat/phase12-dom.mjs | 211-221, 283-287 | phase06-dom 复制夹具面零消费（holdAttachFetchN/releaseHeldFetch/staleClose） | ℹ️ Info | IN-02（12-REVIEW 登记）；Phase 14 参数化收编时处置——不阻塞 |
| internal/server/export_test.go | — | 12-05 diff 白名单枚举未列（+17/-0 append-only 观测出口） | ℹ️ Info | WINDOWS.md #33 已登记（白名单补充项①，三轴裁决：12-03 plan 明示授权 + append-only + 零断言）——不阻塞 |

**零债务标记**：TBD/FIXME/XXX 全部 phase 触达文件零命中（12 文件逐个扫描）。零 placeholder/not-yet-implemented 形态。零 stub。零 BLOCKER/WARNING 级反模式。

### Human Verification Required

### 1. CR-01 真实浏览器 resize 观感（fixer 显式标注 *requires human verification*）

**Test:** 在 Windows 工作站 Playwright 层（双机拓扑）per-client 模式 attach 后拖窗放大/缩小往复，观察渲染尺寸是否即时跟随窗口；随后 1006 断线重连，观察旧屏残影清除与新会话画面。
**Expected:** 渲染尺寸随窗口即时跟随 fit——无折行错位、无 attach 时旧尺寸钳制（jsdom D2e/D2f/D2g 三断言的真实浏览器成立：行数 24→30、RESIZE 载荷 {cols:98,rows:30}、90 字符单行不折）；重连后画面干净无残影（SC3 浏览器半侧观感）。
**Why human:** Linux 开发机 headless 禁 Playwright（CODEBUDDY.md 双机拓扑硬约束），浏览器观感属视觉面 grep/jsdom 无法覆盖；12-REVIEW-FIX 将 CR-01 修复标注 *requires human verification*（逻辑面已由 jsdom 红→绿 17/17 端到端证据覆盖，残余人工面仅真实浏览器观感），已登记归 Phase 14 pw 层——**本项为登记确认（accept-or-verify-now 决策），非新发现缺口**。

### Gaps Summary

**零缺口。** 25/25 合并 truth 全部 VERIFIED——四类交互语义（尺寸直通/ro 输入门控/重连 reset/停读续读+dwell 踢出）在 per-client 分支各归各会话、无仲裁、无串扰，重连即全新。全部行为依赖型 truth 均有本次独立复跑的通过测试实证（Go 定向组 -race 12 测 + TestPerClientReconnectNewPid + phase12.mjs 两轮 20/20 + phase12-dom 17/17 + phase06-dom 40/40 基线一致 + gofmt/vet/build 静态面）。

**如实记录的登记项**（按验证指示不判为本 phase 失败）：
1. **pinger/dwell 竞态**（Deferred #1）——默认 --ping-interval 下 TCP 级停读客户端 1006 先杀；STATE.md:137 Blockers 已登记 → Phase 13 裁决。dwell 机制本身经生产 10s 零覆写实证成立（本次实测 10.3s），S5/S6 以公开 CLI flag 隔离后全绿。
2. **CR-01 人工残余面**（Human Verification #1）——真实浏览器 resize 观感，fixer 标注 *requires human verification*，归 Phase 14 pw 层；逻辑面 jsdom 红→绿证据完整。
3. **export_test.go 白名单枚举缺口**——WINDOWS.md #33 在案（append-only 观测出口，「期望值逐字未动」审查目的未受影响，本次独立 diff 复核确认）。
4. **Info 两项**（UAT skip 死代码 IN-01 / phase12-dom 零消费夹具 IN-02）——留待 Phase 13/14。

**结论**：自动化检查全部通过，phase goal 在代码库中成立。唯一未决项是 CR-01 的浏览器观感人工面（已登记 Phase 14）——接受登记延后即可转 passed，或即时在 Windows 工作站执行 pw 验证。

---

_Verified: 2026-09-04T16:48:52Z_
_Verifier: Claude (gsd-verifier)_
