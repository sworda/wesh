---
phase: 05-multi-client
verified: 2026-08-22T11:50:40Z
status: passed
score: 3/3 roadmap success criteria verified（8/8 requirements satisfied）；gap-closure truths 15/15 verified（含前次 partial 的 truth #5 升级 VERIFIED）
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/3 roadmap criteria（8/8 requirements）；gap-closure truths 14/15
  gaps_closed:
    - "WR-01（pushSessionDimsLocked 循环内踢出后 stale 扇出）：74d1bff 落复检 `if s.arbiter.last != target { return }`（resize.go:177-179，kickOrCreditLocked 返回后唯一挂点）+ 安全性注释改写为真实可达的 removeMember→嵌套 recalcNow 路径四要素论证（resize.go:153-164）+ TestPushSessionDimsKickRecalc 白盒回归（本轮独立 -race 运行 PASS，第 12 轮迭代命中 B-first 危险序）——复验核读确认闭合"
    - "WR-02（creditBlocked 端尺寸推送静默丢弃无补发）：4936f1c 落 option (a)——afterDrain 清位后/Broadcast 前补发当前 sessionDimsLocked() 的 Welcome（clients.go:459-467，语句序：重投→清位→gateTransitions++→补发→Broadcast 核读确认）+ kickOrCreditLocked/afterDrain 双注释收窄互指 + TestAfterDrainResendsDims 两子测（本轮独立 -race 运行全 PASS）——复验核读确认闭合"
    - "WR-03（新注释跨文件行号漂移）：3a81dfb 把 resize.go:155 引用改为 clients.go:501-502——核读确认该行号恰为 removeMember/recalcNow 两调用行；同注释区另一引用 clients.go:354-358（onChunk 推送循环）仍准确；IN-05 注释精度项登记 deferred-items.md（生产默认 512KiB 无影响，失败形态自愈）"
  gaps_remaining: []
  regressions: []
deferred:
  - truth: "1013 被踢后的自动重连（当前为手动刷新面板）"
    addressed_in: "Phase 6"
    evidence: "ROADMAP Phase 6 成功准则 3：断网 30s 恢复后前端自动重连（指数退避 + 上限 + 手动入口）并接回同一 PTY 进程（CORE-05）；05-CONTEXT D-10 锁定本 phase 不做任何自动重连"
  - truth: "五处观测计数器（kicks/gateTransitions/inputDrops/droppedInputs/registry.n）进 metrics"
    addressed_in: "Phase 8"
    evidence: "ROADMAP Phase 8 成功准则 2：/metrics 暴露连接数、会话数、收发字节数、每客户端 outbox 深度与踢出计数（OPS-07）；05-09-SUMMARY 交接清单已列五处 stub 位置与消费方式"
  - truth: "outbox 512KiB / 输入 32KiB/s+64KiB / max-clients 32 / 防抖 50ms / 信用半水位 50% 五项初值经负载标定回填"
    addressed_in: "Phase 9"
    evidence: "ROADMAP Phase 9 成功准则 2：负载/模糊测试通过，测试数据回填 P2/P5 默认参数；README『默认参数与 Phase 9 标定』节已落地方法论"
  - truth: "--write-policy/--max-clients 新 flag 配置文件收口"
    addressed_in: "Phase 7"
    evidence: "ROADMAP Phase 7 成功准则 1：TOML 配置文件支持，CLI 参数覆盖配置文件（OPS-09）；05-CONTEXT deferred 锁定"
  - truth: "afterDrain 补发「入队必成」注释容量下界精度（IN-05）"
    addressed_in: "下次触碰 clients.go 时顺带修正"
    evidence: "deferred-items.md 2026-08-22 05-13 评审期条目：严格下界 ≈ 64KiB+200B 而非注释所述 64KiB；生产默认 512KiB 余量 ~224KiB 无影响，失败形态为补发帧丢弃、下次尺寸事件自愈（已裁决不兜底）；纯注释精度项不触及行为"
behavior_unverified_items: []
human_verification:
  - test: "双客户端视觉一致（MULTI-01 渲染层，backstop truth）"
    expected: "两浏览器窗口 attach 同一会话输出逐屏一致；异尺寸窗口约束到会话矩形渲染、多余面积留白（G-05-1 修复形态）；关掉一端后剩余端恢复自身尺寸渲染"
    why_human: "headless 硬约束——本机永不具备浏览器（CODEBUDDY.md 平台原生行为显式豁免条款），像素层一致性任何自动化结构性不可测；协议层等价断言本轮独立复跑全过（S1b 双端逐字节一致 + D6H-1 约束渲染≡窄端原生逐屏严格一致 + D6H-2 负对照分叉）。清单：05-UAT.md 第 1 项。按豁免条款风险接受，不构成 status 路由依据"
  - test: "新客首屏 SIGWINCH 重绘（D-11）"
    expected: "会话运行 vim/htop 时新客户端 attach 秒见重绘画面，非黑屏等下次输出"
    why_human: "浏览器渲染行为不可测；协议层证据 = TestSigwinchOnAttach（落盘标记，全量 -race 绿）+ server.go 调用点。清单：05-UAT.md 第 2 项。豁免条款风险接受"
  - test: "ro 形态三要素 + console 一次性提示"
    expected: "[ro] 标题前缀、键盘不可输入、窗口拖动无上行 RESIZE 帧、console 一条 read-only mode 提示（尺寸推送形态下仍恰一次）"
    why_human: "DevTools 帧面板与标题栏属浏览器平台行为；jsdom 层 D1b/D1d 本轮复跑通过（infos=1 条 / 上行帧=[]）。清单：05-UAT.md 第 3 项。豁免条款风险接受"
  - test: "递补升格 UX（owner 模式 D-06/D-07）"
    expected: "第二 rw 端降级旁观 → 关闭 owner 标签页 → 前缀消失、键盘激活、可输入、约束解除恢复窗口渲染；全程无 toast/badge"
    why_human: "浏览器 UI 行为；服务端机制 TestSuccession/TestSuccessionKickRace + 协议层 S9a-c/S10c + jsdom D2c/D6c 本轮复跑全过。清单：05-UAT.md 第 4 项。豁免条款风险接受"
  - test: "1013 专版 + 手动刷新链路（D-10）"
    expected: "stall 被踢后 Disconnected 面板 + Reload this page；刷新凭原 URL 重新 attach 成功并从最新输出看起；其他端不受影响"
    why_human: "真实慢网与浏览器面板行为；协议层 phase05.mjs S6 三断言本轮复跑通过。清单：05-UAT.md 第 5 项。豁免条款风险接受"
  - test: "503 专版与无效链接专版（含 G-05-7 无认证错 token 401 → Invalid 面板）"
    expected: "--max-clients 1 实例第二客户端 attach → Server is full 面板；错误 token /s/ URL → 凭据模式 Basic 框 / 无认证模式直接 Invalid share link 面板"
    why_human: "浏览器面板与原生 Basic 弹窗行为；协议层 S4c-e（401 同文无 oracle / 无认证错 token 401 / 未携 404 探测信号）与 jsdom D4a/D4b 本轮复跑全过。清单：05-UAT.md 第 6/7 项。豁免条款风险接受"
process_notes:
  - "本轮为 05-13 gap-closure 后的复验：三条缺陷（WR-01/WR-02/WR-03）全部由代码核读 + 独立行为证据双重确认闭合，gaps 块清空，status 由 gaps_found 转 passed"
  - "人工验证 6 项沿前次清单不变——全部命中 CODEBUDDY.md 平台原生行为显式豁免条款（headless 永不可测），协议/DOM/headless-core 三层等价断言本轮独立复跑全绿，按既定裁决风险接受、不驱动 status"
  - "REQUIREMENTS.md 状态列（MULTI-01/02/03/05、RES-02/03 = Gaps Found；MULTI-04/RES-04 = Complete）为追踪器元数据，由 orchestrator 工作流在 phase 收口时统一更新，非本报告职责；本报告判定的 8/8 SATISFIED 为代码层实现证据结论"
  - "05-REVIEW（2026-08-22T11:33Z 增量复审）逐字比对补丁保真度：WR-01/WR-02 均逐字一致、上轮发现 fully resolved；WR-03/IN-05 为注释精度项已分别修复/挂账，不触及行为"
  - "IN-01..IN-04 打磨项（前次 REVIEW 挂账）维持既定裁决不动，不阻塞"
---

# Phase 5: 多客户端共享 — Verification Report（复验：05-13 WR-01/WR-02 gap 闭合后）

**Phase Goal:** 多个客户端可同时 attach 同一 PTY 会话，权限可配、慢客户端不拖累他人——核心差异化能力
**Verified:** 2026-08-22T11:50:40Z
**Status:** passed
**Re-verification:** Yes — after WR-01/WR-02/WR-03 gap closure（05-13，commits 74d1bff/4936f1c/3a81dfb）

## Goal Achievement

### Observable Truths（ROADMAP 成功准则 — 回归层）

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 两个浏览器 attach 同一会话输出实时一致；all 模式全员可写，owner 模式仅 owner 可写、ro 链接旁观者输入被丢弃 | ✓ VERIFIED | 回归：全量 go test -race -count=1 5/5 包 ok（本轮独立运行，server 38.6s）；phase05.mjs 本轮复跑 28/28+1skip（S1b 双端逐字节一致 + S1a D-07 形态） |
| 2 | 一个客户端停止读取 TCP 流时其他客户端无卡顿：慢客户端 outbox 写满被 1013 踢出，重连后从最新输出看起；PTY 读循环永不因任何客户端阻塞 | ✓ VERIFIED | 回归：TestSlowConsumerKick/TestGlobalCredit 随全量 -race 绿；S6 三断言本轮复跑通过；onChunk 非阻塞 trySend + 信用门路径（clients.go:345-359）本轮未被 05-13 触碰（diff 仅 afterDrain 补发段 + 注释） |
| 3 | 异尺寸两客户端按 min(cols)×min(rows) 渲染，2→1 恢复 last-wins；启动打印含一次性 token 的 ro/rw 两条分享链接，即打即用 | ✓ VERIFIED | 仲裁：TestArbitrate/TestResizeArbitration 随全量 -race 绿；渲染约束 S10/D6/D6H 三层本轮复跑全过；分享链接 S2/S3/S4 本轮复跑通过。**前次保留条款解除**：WR-01/WR-02 窄窗口已闭合（见下表 truth #5），渲染不变量在全部已核读交织下成立 |

**Score:** 3/3 roadmap truths verified

### Observable Truths（G-05-1 gap 闭合 + 缝合面 — 05-10/11/12/13 must_haves）

| # | Truth（来源 plan） | Status | Evidence |
|---|-------|--------|----------|
| 4 | attach Welcome 恒携会话 cols/rows，数值 = attach 完成后生效的会话尺寸（05-10） | ✓ VERIFIED | 前轮已验（proto.go:105-108 / server.go:716-731 / TestWelcomeSessionDims / S10a）；05-13 零触碰该路径，本轮 S10a 复跑通过（A=rw/40x10 B=ro/40x10） |
| 5 | owner resize 防抖后全部在线客户端收到携新会话尺寸的推送；任意时刻任意在线客户端持有的最近一帧 Welcome 尺寸 == 服务端会话尺寸（05-10 must_have + success criterion） | ✓ VERIFIED（前轮 partial → 本轮升级） | 主路径：recalcNow last 变化分支唯一挂点 + S10b 本轮复跑通过（B=ro/60x15 A=rw/60x15 同收）。**两条违例路径本轮确认闭合**：(a) WR-01 踢出嵌套重算交织——resize.go:177-179 复检落位，充分性论证核读成立（arbiter.last 唯一写点 = recalcNow:138 且写后必接嵌套推送，last != target ⟹ 新值已送达全部留存端；踢出不改仲裁/信用路径/零成员哨兵三边界 last==target 正确继续），TestPushSessionDimsKickRecalc 本轮 -race PASS（第 12 轮命中 B-first；测试牙齿核读确认——修复前 B-first 态 A.outbox=[W(60x50),W(60x24)]，末帧 ≠ arbiter.last 必败且永不产出 len==1，32 轮内必爆）；(b) WR-02 blocked 端丢帧——clients.go:459-467 afterDrain 补发落位（语句序核读：重投→清位→gateTransitions++→补发→Broadcast），TestAfterDrainResendsDims 本轮 -race 两子测全 PASS。S→C 尺寸五通道（attach S10a / 防抖推送 S10b / 升格 S10c / 踢出嵌套重算白盒 / 信用恢复白盒）全部有行为测试锁定——量化断言在全部已核读交织下成立 |
| 6 | 递补升格 Welcome 携新会话尺寸（= cand.dims）（05-10） | ✓ VERIFIED | 前轮已验（clients.go:540 + 单员恒等论证）；05-13 零触碰，本轮 S10c 复跑通过（升格=rw/120x40） |
| 7 | 尺寸变化检测唯一挂点 = recalcNow last 变化分支，目标不变零推送（05-10） | ✓ VERIFIED | resize.go:135-140 核读确认形态不变；复检只在 trySend 失败分支内（:177-179），成功路径零干涉；五调用点（attach/detach/kick/升格/防抖）全覆盖 |
| 8 | 既有行为零回归（05-10/11/12/13 合并） | ✓ VERIFIED | 本轮独立复跑：全量 go test -race 5/5 包 ok；go vet 干净；phase05.mjs 28/28+1skip、phase05-dom 19/19、phase05-dims DIMS PASS、phase02 12/12、phase03 18/18、phase04 10/10、t1-width 5/5——exit 全 0 |
| 9 | 宽端约束渲染到会话矩形留白；同 cols 渲染同字节流逐屏一致（05-11） | ✓ VERIFIED | 前轮已验（main.ts:284-285 逐轴 min）；本轮 D6a/D6b/D6H-1/D6H-2 复跑全过 |
| 10 | rw 上行 RESIZE 恒为窗口 fit 尺寸；升格纠正链不断裂（05-11） | ✓ VERIFIED | 前轮已验（main.ts:289/:265-267）；本轮 S9b/D6c 复跑通过 |
| 11 | 升格 Welcome 到达后约束解除（05-11） | ✓ VERIFIED | 前轮已验（main.ts:507→:548 顺序）；本轮 D6c 复跑通过（行数回 24） |
| 12 | 重复 Welcome 全链幂等；ro 提示每连接恰一次（05-11） | ✓ VERIFIED | 前轮已验（roNotified 门闩 + 变化守卫链）；本轮 D1b/D2a 复跑通过。WR-02 补发帧复用 'W' 帧——前端幂等链对补发通道结构性免疫 |
| 13 | 旧服务端（Welcome 无尺寸键）前端行为零漂移（05-11） | ✓ VERIFIED | 前轮已验（main.ts:498-508 成对校验降级）；05-13 零前端改动 |
| 14 | headless 等价锁 + 负对照（05-12） | ✓ VERIFIED | 本轮 phase05-dims.mjs 复跑 DIMS PASS：D6H-1 逐屏全等 / D6H-2 负对照分叉 |
| 15 | 文档同步（05-12）+ 缝合面注释真实性（05-13） | ✓ VERIFIED | 前轮已验 README/05-UAT/05-VALIDATION 落盘；本轮核读：resize.go:153-164 安全性注释改写四要素齐备（removeMember 真实可达路径主论证 + promoteNextLocked 不可达性压缩为从句）、:149-151 触发帧承诺收窄与 clients.go:390-393 互指一致、:155 行号引用 clients.go:501-502 精确命中 removeMember/recalcNow 调用行（WR-03 闭合） |

**Score:** 15/15 gap-closure truths verified（#5 由 partial 升级 VERIFIED）

### Required Artifacts（05-13 增量面，三级核读）

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/server/resize.go` | pushSessionDimsLocked 增 arbiter.last 复检 + 注释改写 | ✓ VERIFIED | :177-179 复检精确位于 trySend 失败分支内 kickOrCreditLocked 返回后（唯一挂点，未移出循环/未挂成功路径），:175-176 补丁注释落位；:153-164 注释四要素齐备；plan 验收断言实测：`grep -c 'if s.arbiter.last != target {'` = 1、文档注释区 removeMember 命中 = 1 |
| `internal/server/clients.go` | afterDrain 补发当前会话尺寸 Welcome + 注释同步 | ✓ VERIFIED | :459-467 补发段落位（sessionDimsLocked :461 + mode/prefs 选档 :462-466 与 pushSessionDimsLocked 逐字同构 + `_ =` 形态 :467）；语句序 :451-468 核读确认；:390-393/:434-441 双注释收窄互指；plan 验收断言实测：`grep -c 'sessionDimsLocked()'` = 3 |
| `internal/server/resize_test.go` | TestPushSessionDimsKickRecalc | ✓ VERIFIED | :118-230 核读：真实 conn 夹具（httptest+Dial）、creackpty 活 master、B outbox cap=1 恒败、32 轮迭代不静默 skip、普适不变量末帧 == arbiter.last；本轮 -race 独立运行 PASS（第 12 轮命中 B-first，12 条 1013 kick 日志实测） |
| `internal/server/clients_test.go` | TestAfterDrainResendsDims 两子测 | ✓ VERIFIED | :149-246 核读：子测 1 守卫语义锁（creditPending 逐字节不覆写 + kicks==0）、子测 2 表驱动 rw/ro 帧序/尺寸/选档区分度断言；本轮 -race 独立运行全 PASS |

### Key Link Verification（缝合面逐条核读）

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| recalcNow last 变化分支 | pushSessionDimsLocked | resize.go:138-140 | ✓ WIRED | 唯一挂点不变；复检不改变挂点拓扑 |
| pushSessionDimsLocked | kickOrCreditLocked → kickSlowConsumerLocked → removeMember → 嵌套 recalcNow → 外层复检中止 | resize.go:173-179 → clients.go:400-419 → :491-513 → :501-502 | ✓ WIRED（前次 defective → 本轮闭合） | 嵌套链全程 hubMu 单锁内；last 推进后嵌套推送先行送达，外层复检中止 stale 扇出——TestPushSessionDimsKickRecalc 逐帧锁定 |
| afterDrain | sessionDimsLocked → WelcomeFrame → trySend → hubCond.Broadcast | clients.go:461-468 | ✓ WIRED（WR-02 收敛链） | 补发帧 FIFO 排在重投 creditPending 之后；hubMu 全程持有 + 锁序 hubMu > outbox.mu 保持（:443-447 与 onChunk 同款） |
| 嵌套 recalcNow | 有界终止 | 每次踢出永久移除一端 | ✓ WIRED | removeLocked 恰好一次（:492-494 幂等防御），嵌套深度 ≤ max-clients 32 |
| attach 升档 / 升格 / WELCOME 前端链 | （05-10/11 既定） | server.go:716-731 / clients.go:540 / main.ts:498-548 | ✓ WIRED | 05-13 零触碰，本轮三层 UAT 复跑全过 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| sessionDimsLocked | arbiter.last / spawn 回落 | 仲裁器重算 / pty.SpawnCols×SpawnRows | ✓（S10a 本轮复跑：旁观者 40x10 ≠ 自身 120x40） | ✓ FLOWING |
| pushSessionDimsLocked | target | arbitrate(members) | ✓（S10b 本轮复跑 60x15 双端同收；WR-01 残留窗口已闭合） | ✓ FLOWING |
| afterDrain 补发帧 | sd = sessionDimsLocked() | arbiter.last 当前值 | ✓（TestAfterDrainResendsDims 子测 2：100x30 + rw/ro 选档实测） | ✓ FLOWING |
| 前端 sessionDims / refit 上报 | Welcome cols/rows / fit.proposeDimensions | S→C 通道 / 窗口物理尺寸 | ✓（D6a/D6b/S9b 本轮复跑） | ✓ FLOWING |

### Behavioral Spot-Checks（本轮独立运行，非 SUMMARY 转述）

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 构建+静态检查 | `go build -o wesh ./cmd/wesh && go vet ./...` | exit 0（build 0.6s，二进制 19:44 新鲜产物 11009872B） | ✓ PASS |
| WR-01/WR-02 新白盒测试 | `go test ./internal/server/ -run 'TestPushSessionDimsKickRecalc\|TestAfterDrainResendsDims' -race -count=1 -v` | 全 PASS（1.0s）；B-first 第 12 轮命中（12 条 kick 日志），不放行空转绿 | ✓ PASS |
| 全量测试 | `go test ./... -race -count=1` | 5/5 包 ok（server 38.6s，单次运行） | ✓ PASS |
| 测试牙齿（修复前必败） | 代码推演（不改动源码） | 修复前 B-first 态 A.outbox=[W(60x50),W(60x24)]：末帧 ≠ arbiter.last 触发 :210 断言失败，且 len 恒 ≠ 1 永不置 hitBFirst——32 轮内必爆（2^-32 残余概率等同确定性） | ✓ PASS（核读证明） |

### Probe Execution（独立进程复跑，全新二进制）

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| phase05.mjs（S10 G-05-1 协议面） | `node web/uat/phase05.mjs ./wesh` | 28/28 + 1 skipped，exit 0；S10a/b/c 全 PASS（40x10≠120x40 / 推送双端 60x15 同收 / 升格 120x40） | ✓ PASS |
| phase05-dom.mjs（D6 约束渲染） | `node web/uat/phase05-dom.mjs ./wesh` | 19/19，exit 0；D6a/D6b/D6c + D1b/D1d/D2c 全 PASS | ✓ PASS |
| phase05-dims.mjs（headless 等价+负对照） | `node web/uat/phase05-dims.mjs ./wesh` | DIMS PASS，exit 0；D6H-1 逐屏全等 / D6H-2 负对照分叉 | ✓ PASS |
| phase02/03/04 回归 | 三脚本连跑 | 12/12、18/18、10/10，exit 全 0 | ✓ PASS |
| phase04-t1-width 回归 | `node web/uat/phase04-t1-width.mjs` | T1 PASS（U11 4/4 + U6 1/1），exit 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| MULTI-01 | 05-01, 05-09, 05-10/11/12 | 多 WS 客户端同时 attach 同一会话，输出实时扇出 | ✓ SATISFIED | TestMultiClientFanout（全量 -race 绿）+ S1b 逐字节一致（本轮复跑） |
| MULTI-02 | 05-03, 05-08 | 写权限可配 all/owner | ✓ SATISFIED | TestOwnerPolicy/TestAllPolicy/TestSuccession 随全量绿；S1a/S9 本轮复跑 |
| MULTI-03 | 05-02, 05-08, 05-09 | 慢客户端有界 outbox 写满 1013 踢出 | ✓ SATISFIED | TestSlowConsumerKick 随全量绿 + S6 三断言本轮复跑 |
| MULTI-04 | 05-04, 05-08, 05-10/11/12, **05-13** | resize 仲裁 min-rect/last-wins/2→1 恢复 | ✓ SATISFIED | TestArbitrate/TestResizeArbitration 随全量绿 + S9b/S10b 本轮复跑 + **TestPushSessionDimsKickRecalc 缝合面加固（05-13）** |
| MULTI-05 | 05-06, 05-08, 05-09 | 启动打印 ro/rw 两条分享链接即打即用 | ✓ SATISFIED | TestShareToken 随全量绿 + S2/S3/S4 本轮复跑 |
| RES-02 | 05-05 | 每客户端输入速率限制 | ✓ SATISFIED | TestInputRateLimit 随全量绿 |
| RES-03 | 05-07, 05-09 | 最大并发客户端数满员拒绝 | ✓ SATISFIED | TestMaxClients503/TestClientCountInvariant 随全量绿 + S5 本轮复跑 |
| RES-04 | 05-02, **05-13** | PTY 输出背压 | ✓ SATISFIED | TestGlobalCredit 随全量绿（readUntilError 仅累积 Output 帧，结构性免疫补发帧——05-REVIEW 已证）+ **TestAfterDrainResendsDims 交界面加固（05-13）** |

13 个 plan 的 requirements 并集 = {MULTI-01..05, RES-02/03/04}，与 REQUIREMENTS.md Phase 5 映射完全一致——**无孤儿需求**（05-13 申报 [MULTI-04, RES-04] 为修复触及面，准确）。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | 无 TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER | — | 05-13 四文件本轮扫描零命中 |
| ~~internal/server/resize.go~~ | ~~156-168~~ | ~~外层循环用捕获 target 继续投递（WR-01）~~ | 已闭合 | :177-179 复检落位，白盒测试锁定 |
| ~~internal/server/clients.go~~ | ~~411-415, 429-447~~ | ~~已 blocked 端尺寸推送静默丢弃无补发（WR-02）~~ | 已闭合 | :459-467 补发落位，两子测锁定 |

### Human Verification Required

浏览器渲染层与平台原生行为 6 组（沿前次清单不变，全部协议层等价断言本轮独立复跑通过；按 CODEBUDDY.md 平台原生行为显式豁免条款风险接受，**不构成 status 路由依据**）：

1. **双客户端视觉一致** — 逐屏一致 + 约束留白 + 2→1 恢复（05-UAT.md #1，backstop；D6H-1/D6H-2 终端核心层等价/分叉锁本轮复跑通过）
2. **新客首屏** — vim/htop attach 秒见重绘（05-UAT.md #2）
3. **ro 形态三要素** — 前缀/键盘禁用/零上行/console 恰一次（05-UAT.md #3；D1b/D1d 本轮复跑绿）
4. **递补升格 UX** — 前缀消失 + 键盘激活 + 约束解除（05-UAT.md #4；D6c 本轮复跑绿）
5. **1013 专版 + 手动刷新链路**（05-UAT.md #5；S6 本轮复跑绿）
6. **503 专版与无效链接专版**（含 G-05-7 无认证 401 → Invalid；05-UAT.md #6/#7；S4c-e/D4b 本轮复跑绿）

### Gaps Summary

**无缺口——前次 gaps_found 的两条缝合面缺陷（WR-01/WR-02）与一条注释漂移（WR-03）全部闭合，status 转 passed。**

闭合证据（本轮全部独立复跑，非 SUMMARY 转述）：

- **WR-01**：resize.go:177-179 复检 + 注释改写 + TestPushSessionDimsKickRecalc 三要素落盘并 -race 绿（B-first 第 12 轮实测命中）。充分性核读：arbiter.last 唯一写点 = recalcNow:138 且写后必接嵌套推送，`last != target` ⟹ 新值已送达全部留存端，中止 stale 扇出是精确防线；三边界（踢出不改仲裁 / 信用路径 / 零成员哨兵）last==target 外层正确继续。测试牙齿核读证明：修复前 B-first 态末帧 60x24 ≠ arbiter.last 必败，且永不产出 len==1——32 轮不放行空转绿。
- **WR-02**：clients.go:459-467 afterDrain 补发（语句序核读确认）+ 双注释收窄互指 + TestAfterDrainResendsDims 两子测 -race 绿（帧序/当前尺寸/rw-ro 选档区分度全锁）。有序性归因成立：afterDrain 全程持有 hubMu + outbox FIFO。
- **WR-03**：resize.go:155 行号引用已修正为 clients.go:501-502（核读命中 removeMember/recalcNow 调用行）；IN-05 注释精度项登记 deferred-items.md（生产无影响、失败自愈、已裁决不兜底）。
- **零回归**：全量 go test -race 5/5 包、go vet 干净、phase05 三层 UAT（28/28+1skip、19/19、DIMS PASS）、phase02/03/04/t1-width 回归全绿——05-13 两修复不引入可观测协议行为变化（复用既有 'W' 帧，前端幂等链结构性免疫）。

**05-10 success criterion「任意时刻任意在线客户端持有的最近一帧 Welcome 尺寸 == 服务端会话尺寸」现成立**：S→C 尺寸五通道全部有行为测试锁定，已核读交织下无残留违例路径。Phase 5 goal 达成。

---

_Verified: 2026-08-22T11:50:40Z_
_Verifier: Claude (gsd-verifier)_
