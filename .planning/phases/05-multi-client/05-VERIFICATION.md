---
phase: 05-multi-client
verified: 2026-08-22T05:35:00Z
status: gaps_found
score: 3/3 roadmap success criteria verified (8/8 requirements satisfied)；gap-closure truths 14/15 verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 3/3 roadmap criteria（8/8 requirements）
  gaps_closed:
    - "G-05-1 主场景（异尺寸双端相对寻址流叠写）：S→C 会话尺寸下发三通道 + 前端 refit() 视口约束落地，三层自动化回归锁（S10/D6/D6H）本次独立复跑全过——用户实测的确定性叠写路径已消除并门禁化"
    - "前次 follow-up WR-01（--credential 回显泄露，main.go:92）已修复：credErr 记录式上报，flag 回显通道不含值（main.go:89-100 核读确认）"
    - "前次 follow-up WR-02（writer 合并控制帧竞态，clients.go:569）已修复：mergeBatch 合并条件已加 proto.Output 守卫（clients.go:575 核读确认）"
  gaps_remaining:
    - "G-05-1 闭合缝合面残留：尺寸推送挂点 pushSessionDimsLocked 存在代码核读确认的可达乱序/丢失路径（05-REVIEW WR-01/WR-02，修复未落地）——同缺陷类（端按错误会话尺寸约束渲染→相对寻址流叠写）在窄窗口内可瞬态复发，详见 gaps"
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
behavior_unverified_items: []
gaps:
  - truth: "05-10 must_have：『owner 窗口 resize 经 50ms 防抖重算后，全部在线客户端（含 ro 旁观者与上报者自身）收到携新会话尺寸的推送』+ 05-10 success criterion『任意时刻任意在线客户端持有的最近一帧 Welcome 尺寸 == 服务端会话尺寸』——在代码核读确认的可达交织下不成立"
    status: partial
    reason: "G-05-1 主场景已闭合并三层锁定，但闭合缝合面（recalcNow 推送挂点）残留两条代码核读确认的可达缺陷路径（05-REVIEW WR-01/WR-02，修复均未落地），同属 G-05-1 缺陷类（端 sessionDims ≠ PTY 实际尺寸 → 相对寻址流叠写/错渲），瞬态自愈但确为可达：WR-01 = pushSessionDimsLocked 循环内踢出触发 removeMember→嵌套 recalcNow（clients.go:479-480 核读确认该调用链），嵌套推送把 W(T2) 送达全部留存端后，外层循环仍用捕获的 T1 继续向未访问端投递（resize.go:156-168 无权威性复检）——map 遍历序下约半数留存端终值 = 过期 T1 而 PTY = T2；WR-02 = 已 creditBlocked 端 trySend 失败时被 `if !c.creditBlocked` 守卫跳过暂存（clients.go:411-415），尺寸推送帧静默丢弃，afterDrain 恢复只重投 creditPending 不补发尺寸（clients.go:429-447），该端 sessionDims 过期至下次尺寸事件。显式裁定：WR-01/WR-02 不完全重开 G-05-1（用户实测的确定性路径已死且门禁化），但 WR-01 使 G-05-1 症状类在窄窗口内残留可达——gap 闭合不能判为完整，须修后收口"
    artifacts:
      - path: "internal/server/resize.go"
        issue: "pushSessionDimsLocked（:156-168）循环内 kickOrCreditLocked 返回后无 `s.arbiter.last != target` 权威性复检；:152-155 安全性注释只论证了实际不可达的 promoteNextLocked 嵌套路径，漏掉真实可达的 removeMember→嵌套 recalcNow 路径"
      - path: "internal/server/clients.go"
        issue: "kickOrCreditLocked（:411-415）已 blocked 端静默丢帧 + afterDrain（:429-447）恢复路径不补发会话尺寸"
    missing:
      - "WR-01 修复（05-REVIEW 已给逐字补丁）：pushSessionDimsLocked 在 kickOrCreditLocked 返回后复检 `if s.arbiter.last != target { return }`——嵌套推送已把更新值送达全部留存端，stale 外层扇出直接中止；同时把 resize.go:152-155 注释的嵌套路径论证从 promoteNextLocked 换成 removeMember→recalcNow；补白盒回归测试（推送循环内踢出改变仲裁结果的交织）"
      - "WR-02 修复（两选一）：(a) afterDrain 清位开门时向该端补发一帧当前 sessionDimsLocked() 的 Welcome（收敛性正解，05-REVIEW 已给逐字补丁）；(b) 收窄 resize.go:148-150『触发帧不丢』注释承诺 + deferred-items 挂账"
deferred_note: "WR-01/WR-02 不属于任何后续 phase 的 goal/success criteria（Phase 6 生命周期 / Phase 7 配置 / Phase 8 观测 / Phase 9 标定均不覆盖尺寸推送正确性），不可 defer"
human_verification:
  - test: "双客户端视觉一致（MULTI-01 渲染层，backstop truth）"
    expected: "两浏览器窗口 attach 同一会话输出逐屏一致；异尺寸窗口约束到会话矩形渲染、多余面积留白（G-05-1 修复形态）；关掉一端后剩余端恢复自身尺寸渲染"
    why_human: "headless 硬约束——本机永不具备浏览器（CODEBUDDY.md 平台原生行为显式豁免条款），像素层一致性任何自动化结构性不可测；协议层等价断言本次独立复跑全过（S1b 双端 338958 字节逐字节一致 + D6H-1 约束渲染≡窄端原生逐屏严格一致 + D6H-2 负对照分叉）。清单：05-UAT.md 第 1 项"
  - test: "新客首屏 SIGWINCH 重绘（D-11）"
    expected: "会话运行 vim/htop 时新客户端 attach 秒见重绘画面，非黑屏等下次输出"
    why_human: "浏览器渲染行为不可测；协议层证据 = TestSigwinchOnAttach（落盘标记，全量 -race 绿）+ server.go 调用点。清单：05-UAT.md 第 2 项"
  - test: "ro 形态三要素 + console 一次性提示"
    expected: "[ro] 标题前缀、键盘不可输入、窗口拖动无上行 RESIZE 帧、console 一条 read-only mode 提示（尺寸推送形态下仍恰一次）"
    why_human: "DevTools 帧面板与标题栏属浏览器平台行为；jsdom 层 D1b/D1d 本次复跑通过（infos=1 条 / 上行帧=[]）。清单：05-UAT.md 第 3 项"
  - test: "递补升格 UX（owner 模式 D-06/D-07）"
    expected: "第二 rw 端降级旁观 → 关闭 owner 标签页 → 前缀消失、键盘激活、可输入、约束解除恢复窗口渲染；全程无 toast/badge"
    why_human: "浏览器 UI 行为；服务端机制 TestSuccession/TestSuccessionKickRace + 协议层 S9a-c/S10c + jsdom D2c/D6c 本次复跑全过。清单：05-UAT.md 第 4 项"
  - test: "1013 专版 + 手动刷新链路（D-10）"
    expected: "stall 被踢后 Disconnected 面板 + Reload this page；刷新凭原 URL 重新 attach 成功并从最新输出看起；其他端不受影响"
    why_human: "真实慢网与浏览器面板行为；协议层 phase05.mjs S6 三断言本次复跑通过。清单：05-UAT.md 第 5 项"
  - test: "503 专版与无效链接专版（含 G-05-7 无认证错 token 401 → Invalid 面板）"
    expected: "--max-clients 1 实例第二客户端 attach → Server is full 面板；错误 token /s/ URL → 凭据模式 Basic 框 / 无认证模式直接 Invalid share link 面板"
    why_human: "浏览器面板与原生 Basic 弹窗行为；协议层 S4c-e（401 同文无 oracle / 无认证错 token 401 / 未携 404 探测信号）与 jsdom D4a/D4b 本次复跑全过。清单：05-UAT.md 第 6/7 项"
process_notes:
  - "前次两个 follow-up 已核实修复：credential 回显（main.go:89-100 credErr 记录式）与 writer 合并控制帧（clients.go:575 mergeBatch 加 proto.Output 守卫）——re_verification.gaps_closed 登记"
  - "05-REVIEW 四个 INFO（S8a/S9a 检查点 ID 复用 / stall 夹具注释 off-by-one / pushSessionDimsLocked 无守卫类型断言 / 无认证 401 body 文案提及 operator credentials）均为打磨级，不使任何 truth 为假，随 REVIEW 挂账不阻塞"
  - "G-05-7（无认证错 token → 401）顺带核读：shareResult 三态拆分与 shareInvalid→401 落位（server.go:351-352/368-369），sharetoken 测试与 S4c-e/D4b 本次复跑全过——05-UAT.md 已标 resolved，无争议"
  - "负载 flake 前次登记 deferred-items.md 维持原判；本次验证 go test -race -count=1 ./... 独立全量绿（server 38.3s）未重现"
---

# Phase 5: 多客户端共享 — Verification Report（复验：G-05-1 gap 闭合后）

**Phase Goal:** 多个客户端可同时 attach 同一 PTY 会话，权限可配、慢客户端不拖累他人——核心差异化能力
**Verified:** 2026-08-22T05:35:00Z
**Status:** gaps_found
**Re-verification:** Yes — after G-05-1 gap closure（05-10/05-11/05-12）

## Goal Achievement

### Observable Truths（ROADMAP 成功准则 — 回归层）

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 两个浏览器 attach 同一会话输出实时一致；all 模式全员可写，owner 模式仅 owner 可写、ro 链接旁观者输入被丢弃 | ✓ VERIFIED | 回归：TestMultiClientFanout/TestAllPolicy/TestOwnerPolicy 全量 -race 绿（server 38.3s，本次独立运行）；phase05.mjs S1b 双端 338958 字节逐字节一致 + S1a D-07 形态（本次复跑 28/28） |
| 2 | 一个客户端停止读取 TCP 流时其他客户端无卡顿：慢客户端 outbox 写满被 1013 踢出，重连后从最新输出看起；PTY 读循环永不因任何客户端阻塞 | ✓ VERIFIED | 回归：TestSlowConsumerKick/TestGlobalCredit -race 绿；S6 三断言（1013 slow_consumer 命中 / 字节单调增长 / resume 终结）本次复跑通过；hub 非阻塞 trySend + 信用门路径未受 G-05-1 改动触碰（onChunk clients.go:345-359 形态不变） |
| 3 | 异尺寸两客户端按 min(cols)×min(rows) 渲染，2→1 恢复 last-wins；启动打印含一次性 token 的 ro/rw 两条分享链接，即打即用 | ✓ VERIFIED（渲染不变量见 gaps 残留窗口） | 仲裁：TestArbitrate/TestResizeArbitration 四子测 -race 绿；渲染约束新形态由 S10/D6/D6H 三层锁定（见下表）；分享链接 S2/S3/S4 全链本次复跑通过。**注意**：渲染不变量在 WR-01/WR-02 窄窗口内瞬态失真（自愈）——主路径成立且门禁化，残留计入 gaps 不重复扣分 |

**Score:** 3/3 roadmap truths verified

### Observable Truths（G-05-1 gap 闭合 — 05-10/05-11/05-12 must_haves）

| # | Truth（来源 plan） | Status | Evidence |
|---|-------|--------|----------|
| 4 | attach Welcome 恒携会话 cols/rows，数值 = attach 完成后生效的会话尺寸（05-10） | ✓ VERIFIED | proto.go:105-108 Cols/Rows 恒序列化无 omitempty；server.go:716-731 升档重排（addMember/recalcNow → sessionDimsLocked → Welcome 组帧 → registerLocked）核读确认；TestWelcomeSessionDims（owner attach 40x10 / 旁观者携 40x10 ≠ 自身 120x40 / all min-rect 60x43 ≠ B 自身 60x50）+ S10a（A=rw/40x10 B=ro/40x10）本次复跑通过；proto_test.go dims round-trip + 恒在键 map 断言（:77-80/:120-135） |
| 5 | owner resize 防抖后全部在线客户端收到携新会话尺寸的推送（05-10） | ✗ FAILED（partial） | 主路径 VERIFIED：recalcNow last 变化分支唯一挂点（resize.go:135-140）+ resize_arb 推送子测 + S10b（B=ro/60x15 A=rw/60x15 同收）本次复跑通过。**但** WR-01/WR-02 代码核读确认两条可达违例路径（修复未落地），详见 gaps——「全部在线客户端」「任意时刻 ==」在窄窗口内不成立 |
| 6 | 递补升格 Welcome 携新会话尺寸（= cand.dims）（05-10） | ✓ VERIFIED | clients.go:540 WelcomeFrame(rw, prefsRW, cand.dims.cols, cand.dims.rows) + 单员恒等论证注释（:512-519）核读确认；TestWelcomeSessionDims 升格子测 + S10c（升格=rw/120x40）本次复跑通过 |
| 7 | 尺寸变化检测唯一挂点 = recalcNow last 变化分支，目标不变零推送（05-10） | ✓ VERIFIED | resize.go:135-140：`target == (dims{}) || target == s.arbiter.last` 提前返回，变化才 Resize+推送；五调用点（attach/detach/kick/升格/防抖）核读全覆盖 |
| 8 | 既有行为零回归（05-10：S1b/ro 双闸/升格 PTY 跟随）（05-11：ro 零上行/升格纠正链）（05-12：全套既有断言） | ✓ VERIFIED | 全量 go test -race 5/5 包 ok；phase02 12/12、phase03 18/18、phase04 10/10、t1-width 5/5、phase05.mjs 28/28+1skip、phase05-dom 19/19（D1b 恰一次 / D1d 零上行 / D2c 升格恢复）、phase05-dims 3/3——全部本次独立复跑 exit 0 |
| 9 | 宽端约束渲染到会话矩形留白；同 cols 渲染同字节流逐屏一致（05-11） | ✓ VERIFIED | main.ts:284-285 逐轴 min(fit, sessionDims)；D6a（.xterm-rows=10 而非 24）+ D6b（80 个 A 折行为相邻两 div 各 40 字符——无约束形态下单行必败，区分度在场）+ D6H-1（120x40→resize(40,10) ≡ 窄端原生逐行全等）本次复跑通过 |
| 10 | rw 上行 RESIZE 恒为窗口 fit 尺寸；升格纠正链不断裂（05-11） | ✓ VERIFIED | main.ts:289 sendResize(d.cols, d.rows) 恒报 fit + :265-267 lastReported 去重（ro 期 isRO 门拦截不记账→升格后首次 refit 必真实上报）；S9b stty 尺寸跟随 + D6c 升格解除本次复跑通过 |
| 11 | 升格 Welcome 到达后约束解除：sessionDims 先更新再 refit，min(fit, session) 自然回窗口（05-11） | ✓ VERIFIED | main.ts:507（赋值）→ mode 分支 → :548（统一 refit）顺序核读确认；D6c（[ro] 前缀消失 + 行数回 24）本次复跑通过 |
| 12 | 重复 Welcome 全链幂等；ro 提示每连接恰一次（05-11） | ✓ VERIFIED | roNotified 门闩（main.ts:526-527）+ term.resize 变化守卫（:286）+ sendResize 去重 + overlay fitChanged 门（:294-295）核读确认；D1b/D2a 本次复跑通过（infos=1 不随推送增长） |
| 13 | 旧服务端（Welcome 无尺寸键）前端行为零漂移（05-11） | ✓ VERIFIED | main.ts:498-508 两键均缺席不动 sessionDims（恒 null → 渲染=fit）；非法键 console.warn 降级；成对校验 typeof/Number.isInteger/[1,1000] 核读确认 |
| 14 | headless 等价锁 + 负对照（05-12） | ✓ VERIFIED | phase05-dims.mjs 本次复跑 3/3：D6H-1 同 40 列渲染同一字节流逐屏严格一致；D6H-2 同流喂 120 列换行点分叉不全等（负对照证明断言区分度） |
| 15 | 文档同步：README 协议表/resize 节 + 05-UAT.md + 05-VALIDATION.md（05-12） | ✓ VERIFIED | README:109 'W' 行恒在 cols/rows + :183 resize 节新形态（约束视口渲染/裁剪不变/减员恢复）；05-UAT.md:20 三层扩编段；05-VALIDATION.md:54-56 三行映射 + Full suite command 两脚本——核读落盘 |

**Score:** 14/15 gap-closure truths verified（#5 partial → gaps）

### Required Artifacts（05-10/11/12 新增面，三级+数据流）

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/proto/proto.go` | WelcomePayload Cols/Rows 恒在 + WelcomeFrame 4 参签名 | ✓ VERIFIED | :105-108 无 omitempty；:140 签名携 cols/rows；5 个调用点全同步 |
| `internal/pty/spawn.go` | SpawnCols/SpawnRows 导出常量 | ✓ VERIFIED | :36-37 常量 + :51 StartWithSize 引用（单一事实源） |
| `internal/server/resize.go` | sessionDimsLocked + pushSessionDimsLocked 挂 recalcNow | ⚠️ VERIFIED-with-gap | 两函数落位且挂点唯一（:135-140/:156-168/:180-185）；**WR-01 权威性复检缺失 + 注释论证覆盖错误路径**（见 gaps） |
| `internal/server/server.go` | attach 升档序列重排 | ✓ VERIFIED | :716-731 新时序核读；Welcome 恒首帧与锁序不变量注释更新 |
| `internal/server/clients.go` | 升格 Welcome 携 cand.dims | ⚠️ VERIFIED-with-gap | :540 升格组帧核读；**WR-02 静默丢帧 + afterDrain 不补发**（:411-415/:429-447，见 gaps） |
| `web/src/main.ts` | 四状态 + refit() + sendResize 去重 + WELCOME 尺寸分支 + roNotified 门闩 | ✓ VERIFIED | :242-247 四状态 / :278-309 refit / :265-267 去重 / :498-548 WELCOME 链 / :526 门闩 / :401-404 connect 重置 / :661-673 onopen 同步；term.onResize 订阅零残留（全文件 grep 无调用） |
| `web/dist/index.html` | 重建产物 | ✓ VERIFIED | mtime 2026-08-22 12:45（晚于 main.ts 12:04）；Math.min 指纹 ×10；'ignoring invalid session dims' 检索串 src/dist 各 1 命中一致 |
| `internal/server/{multi,resize,resize_arb}_test.go` + `internal/proto/proto_test.go` | 新测试组 | ✓ VERIFIED | TestWelcomeSessionDims/TestSessionDimsLocked/运行期尺寸变化推送子测/dims round-trip/恒在键断言全部存在且本次 -race 通过 |
| `web/uat/phase05.mjs` S10 / `phase05-dom.mjs` D6 / `phase05-dims.mjs` | 三层回归锁 | ✓ VERIFIED | 本次独立复跑：28/28+1skip / 19/19 / 3/3，exit 全 0 |

### Key Link Verification（新增挂点逐条核读）

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| recalcNow last 变化分支 | pushSessionDimsLocked | resize.go:138-140 | ✓ WIRED | 唯一挂点；目标不变零推送 |
| attach 升档 | Welcome 携会话尺寸 | addMember/recalcNow(:716-717) → sessionDimsLocked(:719) → WelcomeFrame(:730) → registerLocked(:731) | ✓ WIRED | 恒首帧不变量保持；推送循环不触达未登记的 attach 者自身 |
| promoteNextLocked | 升格 Welcome 携 cand.dims | clients.go:540 | ✓ WIRED | 单员恒等论证注释在场 |
| pushSessionDimsLocked | kickOrCreditLocked → removeMember → 嵌套 recalcNow | resize.go:164-166 → clients.go:479-480 | ⚠️ WIRED- defective | 嵌套推送 T2 后外层继续 T1——WR-01 乱序路径（见 gaps） |
| WELCOME 分支 | sessionDims → refit | main.ts:507 → mode 分支 → :548 | ✓ WIRED | 成对校验 [1,1000]，非法降级保持旧值 |
| onopen Hello | lastReported 同步 | main.ts:661-673 | ✓ WIRED | 握手后等值 RESIZE 零重发 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| sessionDimsLocked | arbiter.last / spawn 回落 | 仲裁器重算 / pty.SpawnCols×SpawnRows | ✓（S10a 旁观者 40x10 ≠ 自身 120x40 实测） | ✓ FLOWING |
| pushSessionDimsLocked | target | arbitrate(members) | ✓（S10b 60x15 双端实测；WR-01/02 窄窗口失真见 gaps） | ✓ FLOWING（残留窗口挂账） |
| 前端 sessionDims | Welcome cols/rows | S→C 三通道 | ✓（D6a/D6b 约束渲染实测） | ✓ FLOWING |
| refit 上报 | fit.proposeDimensions | 窗口物理尺寸 | ✓（S9b stty 跟随实测） | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 构建+静态检查 | `go build -o wesh ./cmd/wesh && go vet ./...` | exit 0 | ✓ PASS |
| G-05-1 新测试组 | `go test ./internal/server/ -run 'TestSessionDimsLocked\|TestWelcomeSessionDims\|TestResizeArbitration' -race` | 3 测试 8 子测全 PASS（3.4s） | ✓ PASS |
| 全量测试 | `go test ./... -race -count=1` | 5/5 包 ok（server 38.3s，单次运行） | ✓ PASS |
| dist 一致性 | `ls -l` + `grep -c 'Math\.min'` + 检索串 | mtime 晚于 src；指纹 ×10；warn 串 src/dist 一致 | ✓ PASS |

### Probe Execution（独立进程复跑，非 SUMMARY 转述）

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| phase05.mjs（S10 G-05-1 协议面） | `node web/uat/phase05.mjs ./wesh` | 28/28 + 1 skipped，exit 0；S10a/b/c 全 PASS（区分度实测值：B 旁观者 40x10≠120x40 / 推送双端同收 60x15 / 升格 120x40） | ✓ PASS |
| phase05-dom.mjs（D6 约束渲染） | `node web/uat/phase05-dom.mjs ./wesh` | 19/19，exit 0；D6a 行数=10 / D6b 40+40 折行 / D6c 升格回 24 全 PASS；D1b/D1d/D2c 零回归 | ✓ PASS |
| phase05-dims.mjs（headless 等价+负对照） | `node web/uat/phase05-dims.mjs ./wesh` | 3/3，exit 0；D6H-1 逐屏全等 / D6H-2 负对照不全等 | ✓ PASS |
| phase02/03/04 回归 | 三脚本连跑 | 12/12、18/18、10/10，exit 全 0 | ✓ PASS |
| phase04-t1-width 回归 | `node web/uat/phase04-t1-width.mjs` | 5/5（U11 4/4 + U6 1/1），exit 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| MULTI-01 | 05-01, 05-09, 05-10/11/12 | 多 WS 客户端同时 attach 同一会话，输出实时扇出 | ✓ SATISFIED | TestMultiClientFanout + S1b 逐字节一致（复跑）+ S10/D6/D6H 一致性扩编 |
| MULTI-02 | 05-03, 05-08 | 写权限可配 all/owner | ✓ SATISFIED | TestOwnerPolicy/TestAllPolicy/TestSuccession -race 绿；S1a/S9 复跑 |
| MULTI-03 | 05-02, 05-08, 05-09 | 慢客户端有界 outbox 写满 1013 踢出 | ✓ SATISFIED | TestSlowConsumerKick + S6 三断言复跑 |
| MULTI-04 | 05-04, 05-08, 05-10/11/12 | resize 仲裁 min-rect/last-wins/2→1 恢复 | ✓ SATISFIED | TestArbitrate/TestResizeArbitration 四子测 + S9b/S10b PTY 跟随复跑（推送缝合面残留见 gaps，不否定仲裁本身） |
| MULTI-05 | 05-06, 05-08, 05-09 | 启动打印 ro/rw 两条分享链接即打即用 | ✓ SATISFIED | TestShareToken + S2/S3/S4 全链复跑 |
| RES-02 | 05-05 | 每客户端输入速率限制 | ✓ SATISFIED | TestInputRateLimit -race 绿 |
| RES-03 | 05-07, 05-09 | 最大并发客户端数满员拒绝 | ✓ SATISFIED | TestMaxClients503/TestClientCountInvariant + S5 复跑 |
| RES-04 | 05-02 | PTY 输出背压 | ✓ SATISFIED | TestGlobalCredit -race 绿（WR-02 为该机制与新推送的交界面缺陷，不否定信用门本身） |

计划 requirements 并集（含 05-10/11/12 各 [MULTI-01, MULTI-04]）= {MULTI-01..05, RES-02/03/04} 与 REQUIREMENTS.md Phase 5 映射（:35-39/:55-57/:129+ 全标 Complete）完全一致——**无孤儿需求**。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | 无 TBD/FIXME/XXX/TODO/HACK/placeholder/空实现 | — | 05-10/11/12 九文件扫描零命中 |
| internal/server/resize.go | 156-168 | 外层循环用捕获 target 继续投递（WR-01） | 🛑 Blocker（gap） | 窄窗口内端 sessionDims 过期 → G-05-1 缺陷类瞬态复发，见 gaps |
| internal/server/clients.go | 411-415, 429-447 | 已 blocked 端尺寸推送静默丢弃无补发（WR-02） | ⚠️ Warning（同 gap 折叠） | 同上，影响面更窄（已严重滞后端） |

### Human Verification Required

浏览器渲染层与平台原生行为 6 组（沿前次清单，全部协议层等价断言本次独立复跑通过；WR-01/WR-02 为代码核读确认的确定性缺陷，不属人工项）：

1. **双客户端视觉一致** — 逐屏一致 + 约束留白 + 2→1 恢复（05-UAT.md #1，backstop；D6H-1/D6H-2 已把等价/分叉锁进终端核心层）
2. **新客首屏** — vim/htop attach 秒见重绘（05-UAT.md #2）
3. **ro 形态三要素** — 前缀/键盘禁用/零上行/console 恰一次（05-UAT.md #3；D1b/D1d 复跑绿）
4. **递补升格 UX** — 前缀消失 + 键盘激活 + 约束解除（05-UAT.md #4；D6c 复跑绿）
5. **1013 专版 + 手动刷新链路**（05-UAT.md #5；S6 复跑绿）
6. **503 专版与无效链接专版**（含 G-05-7 无认证 401 → Invalid；05-UAT.md #6/#7；S4c-e/D4b 复跑绿）

全部项按 CODEBUDDY.md 显式豁免条款风险接受，人工清单执行即闭环；不构成 status 路由依据（本状态由 gaps 决定）。

### Gaps Summary

**G-05-1 主场景已闭合，闭合缝合面残留一条可达乱序路径（WR-01）+ 一条可达丢失路径（WR-02）——判定 gaps_found。**

证据侧（闭合面）：05-10/11/12 的 15 条 must_have truths 中 14 条在代码库中验证为真且全部有行为测试/探针实测锁定（本次独立复跑：go -race 全量 5/5 包、S10 28/28、D6 19/19、D6H 3/3、phase02/03/04/t1-width 回归全绿）。用户实测的「A 小 B 大 A 内输入叠写」确定性路径已消除并被三层断言永久门禁化。前次两个 follow-up（credential 回显 / writer 合并控制帧）核读确认已修复。

缺口侧（缝合面）：05-REVIEW（2026-08-22）WR-01/WR-02 经本次逐行核读确认成立且修复未落地——

- **WR-01（resize.go:156-168）**：pushSessionDimsLocked 循环内踢出经 clients.go:479-480 removeMember→嵌套 recalcNow 可达（踢出 ro 成员或 all 模式 rw 离群成员且其持有某轴最小值时仲裁结果改变），嵌套推送 W(T2) 送达全部留存端后外层循环仍以捕获的 T1 组帧投给未访问端——终值倒挂（端 sessionDims=T1 ≠ PTY=T2），**G-05-1 缺陷类在窄窗口内瞬态复发**，下次尺寸事件自愈。
- **WR-02（clients.go:411-415 + 429-447）**：已 creditBlocked 端的尺寸推送帧被 `if !c.creditBlocked` 守卫静默丢弃，afterDrain 恢复不补发——同缺陷类，影响面限于已严重滞后端，同样自愈。

**显式裁定：WR-01/WR-02 不完全重开 G-05-1**（gap 原文 truth 的主场景「含行编辑回显在内逐屏一致」在全部已行使路径上成立且门禁化），**但 WR-01 使同一症状类残留可达**——05-10 自身 success criterion「任意时刻任意在线客户端持有的最近一帧 Welcome 尺寸 == 服务端会话尺寸」在该交织下为假，gap 闭合不能判完整。两条修复均为小补丁（05-REVIEW 已给逐字方案），建议以 `/gsd-plan-phase --gaps` 收口；WR-01/WR-02 不匹配任何后续 phase 的 goal/success criteria，不可 defer。

---

_Verified: 2026-08-22T05:35:00Z_
_Verifier: Claude (gsd-verifier)_
