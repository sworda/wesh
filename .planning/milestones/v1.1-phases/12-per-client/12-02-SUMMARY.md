---
phase: 12-per-client
plan: 02
subsystem: server-frontend
tags: [resize-passthrough, debouncer, per-client, ro-gate, rate-limit, tiocswinsz, jsdom, tdd]

# Dependency graph
requires:
  - phase: 12-per-client plan 01
    provides: WelcomePayload.Session 模式位（proto→wire）+ 前端 sessionMode per-connection 变量（WELCOME 分支解析/connect() 重置块）+ phase12-dom.mjs 夹具（loadTerminal/SpyWebSocket/D1/D3 形态）
  - phase: 11-per-client
    provides: upgradePerClient 升档分岔 + pcSession 结构 + teardownPCLocked 序列 + perclient_test.go harness（startPerClientServerWithSpawn/readPump/frameRes）
  - phase: 05-multi-client
    provides: resize.go arbiter 防抖机械（抽取母本）+ resize_arb_test.go 观测面（ptySize/pollSize/sendResize）+ 每客户端限速（RES-02）
provides:
  - debouncer 共用件（newDebouncer/Reset/Stop，resize.go）——arbiter 与每会话直通共用同一组件同一时长源
  - server.go RESIZE case per-client 直通分支（cl.pc != nil 门，ro/rw 同形直通——D-06 服务端第二闸 per-client 不生效）
  - pcSession 每会话防抖三字段（resizeMu/pendingResize/resizeDeb）+ teardown Stop 挂点
  - main.ts sendResize 第一闸模式位条件化（isRO && sessionMode !== 'per-client'——D-07 与 D-06 同 plan 配对生效）
  - phase12-dom.mjs D2 场景（per-client ro RESIZE 上行 + shared 对照零 RESIZE）
  - 五 Go 行为测（直通 RW/RO/隔离零 'W'/ro INPUT 丢弃/限速保留）
affects: [12-03 (debouncer 共用件为 dwell 计时器 AfterFunc 纪律邻件——不直接消费), 12-04 (phase12.mjs S2/S3/S4 协议层对照直接消费本 plan 服务端语义), 12-05 (PC-05/PC-07 勾选证据链——Go 五测 + jsdom D2), 14 (herdr ro 移动端转屏场景前半已通)]

# Actuals (#2632) — 与 plan estimate (65000 tokens) 同标尺。
# 口径注记：源码 diff chars/4（internal/server/ + web/src/ + web/uat/phase12-dom.mjs），
# 排除 web/dist/index.html（构建再生产物，机械重建非作者工作，12-01 口径沿用）。
# 大幅低于 estimate 的原因：五测大量复用同包既有夹具（ptySize/pollSize/sendResize/
# readPump），前端仅改一闸一行，无新文件级建设。
actuals:
  tokens: 7173
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: [] # 零新依赖红线保持（T-12-SC）：pnpm-lock.yaml 零 diff；creack/pty、jsdom 均既有
  patterns:
    - debouncer 共用件（单组件双消费——arbiter 与每会话实例防双写漂移，ROADMAP「含」兑现）
    - 读循环 case 级模式分岔（cl.pc != nil 门先于 mode 闸——RESIZE 因锁序差异必须分支，INPUT 经 cl.inQ 间接字段零分支的对照）
    - D2 判别面唯一性设计（Hello 即首报同步 lastReported——握手后基线恒零 RESIZE，fit 真变才发帧）
key-files:
  created: [] # 全部为既有文件扩展，无新建
  modified:
    - internal/server/resize.go
    - internal/server/server.go
    - internal/server/perclient.go
    - internal/server/perclient_test.go
    - web/src/main.ts
    - web/dist/index.html
    - web/uat/phase12-dom.mjs

key-decisions:
  - "winsize 观测面复用同包 resize_arb_test.go 的 ptySize/pollSize（creack/pty Getsize 即 TIOCGWINSZ 直读）——plan 原文 unix.IoctlGetWinsize 直接写的同语义既有件，零新代码零新导入，与既有仲裁测试同观测通道"
  - "D2 判别面经 main.ts onopen lastReported 同步语义（Hello 载荷即首次尺寸上报）收敛：WELCOME refit 的等值上报被去重吞掉，握手后基线恒零 RESIZE——布局桩突变（720x408→900x510）+ resize 事件才产生新帧，D2b 判别力唯一（无「WELCOME refit 已发帧」假阳面）"
  - "TestPerClientROInputDropped 双实例形态（Writable:false ro 半场 + Writable:true rw 对照半场）：同断言通道证明丢弃是 mode 闸语义而非链路故障"
  - "TestPerClientInputRateLimitKept 洪水后 250ms 令牌回充等待（32KiB/s × 250ms = 8KiB ≫ 探针 14B，令牌桶数学确定性）+ 探针前 \\r 收口 canonical 行缓冲——两步消除 drop 语义下的探针帧误丢 flaky 面"
  - "PC-05/PC-07 需求勾选留 phase 末 12-05（ID 跨 12-02/04/05 共享，12-04 协议层证据未落——11-01/12-01 先例延续）"

patterns-established:
  - "读循环 case 的 per-client 分岔形态：cl.pc != nil 门 + continue 收口 + shared 半侧逐字保留（diff 纯新增零删除可验收）——12-03 停读续读分支可同构"
  - "jsdom 布局桩突变驱动 fit 尺寸变化（ctx.dims 闭包对象突变 + resize 事件 → proposeDimensions 新值）：后续 resize 相关 DOM 断言复用"

requirements-completed: []  # PC-05/PC-07 跨 plan 共享（12-02/12-04/12-05），按 11-01/12-01 先例留 phase 末 12-05 勾选

coverage:
  - id: T1
    description: "per-client RESIZE 直通本会话 TIOCSWINSZ——钳制 [1,1000]（Decode 层既有）与 50ms 防抖保留（共用 debouncer）；ro 直通放行（D-06）；无 'W' 约束帧"
    requirement: PC-05
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestPerClientResizePassthroughRW / #TestPerClientResizePassthroughRO / #TestPerClientResizeIsolation（-race 全绿）"
        status: pass
      - kind: static
        ref: "server.go RESIZE case diff 纯新增 23 行零删除（shared 半侧逐字保留）+ perclient.go debouncer 回调函数体 hubMu 零命中"
        status: pass
    human_judgment: false
  - id: T2
    description: "前端 ro 第一闸按模式位放开：per-client ro 端 fit 变化上报 RESIZE，shared ro 保持不发（D-07/D-06 同 plan 配对落地）"
    requirement: PC-05
    verification:
      - kind: automated_ui
        ref: "web/uat/phase12-dom.mjs#D2（D2a-D2d 4 check 全过：per-client ro 事件驱动 RESIZE 0x31 上行 + shared 对照零 RESIZE）"
        status: pass
      - kind: automated_ui
        ref: "phase06-dom.mjs 40/40+2skip + phase05-dom.mjs 19/19 零修改重跑（shared 零漂移）"
        status: pass
    human_judgment: false
  - id: T3
    description: "ro 客户端 INPUT 被服务端丢弃（对自身进程同样无效）；每客户端输入限速保留（RES-02 drop 语义）"
    requirement: PC-07
    verification:
      - kind: unit
        ref: "internal/server/perclient_test.go#TestPerClientROInputDropped / #TestPerClientInputRateLimitKept（-race 全绿）"
        status: pass
      - kind: static
        ref: "server.go INPUT case ro 丢弃闸（:1157-1159 现状）零改动（T-12-06 缓解——diff 审查确认）"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-09-04
status: complete
---

# Phase 12 Plan 02: PC-05/PC-07 RESIZE 直通与 ro 门控 Summary

**per-client RESIZE 直通本会话 TIOCSWINSZ（debouncer 单组件双消费 + 每会话 50ms 防抖）+ ro 双闸配对放行（服务端 D-06 直通分支 × 前端 D-07 模式位闸）+ ro INPUT 门控与限速保留，Go 五测 + jsdom D2 双向断言锁定，shared 路径 diff 纯新增零回归**

## Performance

- **Duration:** 20 min
- **Started:** 2026-09-04T12:02:29Z
- **Completed:** 2026-09-04T12:24:46Z
- **Tasks:** 3（Task 1 服务端直通 + Task 2 前端配对 + Task 3 Go 断言组）
- **Files modified:** 7（源码 6 + dist 1）

## Accomplishments

- **debouncer 共用件抽取（ROADMAP「含」兑现）**：resize.go 抽出 `debouncer` 类型（newDebouncer 构造即 Stop / Reset / Stop——AfterFunc 形态注释论证随件迁移），arbiter.timer 改持 `*debouncer`，initArbiter/reportResize 调用形态不变；既有 resize_arb_test.go/resize_test.go **零改动全绿**为行为等价证据；时长源恒 s.resizeDebounce 单点（defaultResizeDebounce/Options.ResizeDebounce），不新增第二份常量（双写漂移防线，Pitfall 7）
- **server.go RESIZE per-client 直通分支（D-06）**：`cl != nil && cl.pc != nil` 门插在 D-09 第二闸之前——per-client 会话 RESIZE 直通自己 PTY（resizeMu 内写 pendingResize + resizeDeb.Reset，到期回调 sess.Resize 仅 fdMu 不持 hubMu，锁序三规则 §5）；**ro/rw 同形直通**（D-06：第二闸 per-client 不生效——ttyd parity，protocol.c 只门 INPUT 不门 RESIZE）；shared 半侧（ro 丢弃 continue + reportResize 入 arbiter）**diff 纯新增 23 行零删除**；直通路径零 Welcome 再推送（不调 recalcNow/pushSessionDimsLocked，arbiter 零值天然 no-op）
- **pcSession 每会话防抖三字段**：resizeMu（叶锁不嵌套）/pendingResize/resizeDeb；upgradePerClient 装配（回调函数体 hubMu 零命中——源码断言验收）；teardownPCLocked 快半段 resizeDeb.Stop()（计时器随会话消亡 + closed 会话 Resize 返 os.ErrClosed 静默双防线）
- **前端第一闸模式位条件化（D-07，与 D-06 同 plan 配对生效）**：`if (isRO && sessionMode !== 'per-client') return`——per-client ro 恢复 fit 变化上报（herdr ro 移动端转屏/拖窗场景前半打通），shared ro 不发逐字保留（05-08 + 服务端 D-09 第二闸纵深）；闸注释重写载 D-07 裁决与 server.go D-06 分支互指（proto.go:6 两侧互指纪律同款）
- **phase12-dom.mjs D2 场景**：per-client ro 实例（无 --writable）布局桩突变 720x408→900x510 + window resize 事件 → sentFrames 含 RESIZE 0x31（D2b）；shared ro 对照同操作全程零 RESIZE（D2d，prohibition 回归锁）；RED→GREEN 完整（旧 dist 下 D2b waitFor 超时实证）
- **Go 断言组五测（D-11 单一家）**：直通 RW（Hello 111x44 出生→RESIZE 120x50 防抖窗后应用）/ 直通 RO（Writable:false 覆写→RESIZE 133x55 直通自己 PTY）/ 尺寸隔离（双端 Hello 尺寸各异→A 变 B 恒 + 双端读泵 500ms 静默窗零 'W' 帧）/ ro INPUT 丢弃（SHOULD_NOT_ECHO 零命中 + rw 对照 echo 正常）/ 限速保留（150KiB 洪水超限丢弃不断开 + 回充后探针回显）
- **零回归三证据**：全量 `go test -race ./internal/...` 三包绿（server 70s）；phase06-dom 40/40+2skip、phase05-dom 19/19 零修改重跑绿；pnpm-lock.yaml 零 diff（T-12-SC 零新依赖红线）

## Task Commits

Each task was committed atomically:

1. **Task 1: 服务端直通——debouncer 共用件 + RESIZE per-client 分支 + pcSession 防抖字段（D-06）** - `9cedbc8` (feat)
2. **Task 2: 前端第一闸放开（D-07）+ dist 重建 + phase12-dom D2** - `4aadd93` (feat)
3. **Task 3: Go 断言组五测（PC-05/PC-07 服务端收口，D-11）** - `bcaee49` (test)

**Plan metadata:** docs commit（本 SUMMARY + STATE/ROADMAP 更新）

## Files Created/Modified

- `internal/server/resize.go` - debouncer 共用件（type + newDebouncer/Reset/Stop）+ arbiter.timer 改持 *debouncer（initArbiter 经共用件构造，reportResize 调用形态零变化）
- `internal/server/server.go` - RESIZE case per-client 直通分支（cl.pc != nil 门，+23 行纯新增；D-06 裁决注释 + main.ts D-07 互指 + 零 Welcome 再推送论证）
- `internal/server/perclient.go` - pcSession 三字段（锁序注释载 resizeMu 叶锁/仅 fdMu）+ upgradePerClient 防抖装配（回调零 hubMu）+ teardownPCLocked 快半段 Stop 挂点
- `internal/server/perclient_test.go` - 五新测追加（+232 行纯新增，既有测试断言行零改动）
- `web/src/main.ts` - sendResize isRO 闸模式位条件化（+D-07 裁决注释重写；sessionMode 消费点 12-01 落地直接复用）
- `web/dist/index.html` - pnpm build 重建纳管
- `web/uat/phase12-dom.mjs` - D2 场景四 check + 场景清单头注释更新

## Decisions Made

- **winsize 观测面选型**（plan action 2 字面为 unix.IoctlGetWinsize 直读）：落地复用同包 resize_arb_test.go 既有 `ptySize`/`pollSize`——creack/pty Getsize 内部即 TIOCGWINSZ ioctl，同一观测语义零新代码，且与既有仲裁测试同通道（数值可比性）；plan 的「x/sys/unix 既有依赖零新增」约束实质满足（甚至更省——连导入都未加）
- **D2 判别面设计**：原计划「dispatchEvent + 等待 100ms → sentFrames 含 0x31」在 WELCOME refit 语义下有假阳面风险（gate 放开后 WELCOME 分支统一 refit 可能先发帧）——经 main.ts onopen `lastReported = {cols, rows}`（Hello 即首报）同步语义分析，等值上报被去重吞掉，握手后基线恒零 RESIZE；D2 以布局桩突变（fit 80x24→100x30）保证事件驱动帧是新帧，判别力唯一。D2a 显式断言基线零 RESIZE 锁定该前提
- **TestPerClientROInputDropped 双实例形态**：Writable:false 单实例无法同时获得 ro 端与 rw 对照端（无认证模式 mode 由 --writable 全局派生，ticket 通道过重）——两 harness 先后装配（各自 t.Cleanup 收口），ro 半场静默窗断言 + rw 半场 echo 对照
- **TestPerClientInputRateLimitKept 确定性设计**：洪水后 250ms 令牌回充等待（数学确定性：8KiB 令牌 ≫ 探针 14B）+ 探针前 `\r` 收口 canonical 行缓冲——两步消除「探针帧被限速误丢」与「垃圾行吞并探针命令」两类 flaky 面；洪水 10×15KiB 帧（< 16KiB ReadLimitPostAuth 单帧硬顶）

## TDD Gate Compliance

- Task 1（tdd=true，重构纪律形态）：既有 resize_arb_test.go/resize_test.go 为等价证据网（组件抽取前后零改动全绿）——debouncer 抽取是 refactor 语义，RED/GREEN 由既有测试组承载（resize_arb_test 的「防抖合并」子测即 debouncer 行为的先行断言）；全量 ./internal/server -race 绿收口
- Task 2（tdd=true，标准 RED→GREEN）：RED = D2 以 5 处新 check 写入后对当前 dist 运行，D2b waitFor 超时 FAIL（旧 isRO 无条件闸下 per-client ro 不发 RESIZE——判别力实证）；GREEN = main.ts 闸改写 + dist 重建后 14/14 全过（D1/D3 既有 10 check 零回归）。单 feat 提交
- Task 3（tdd=true，断言收口形态——plan action 3 显式「此时直通分支已落地，测试先行编写并与实现同 PR 提交」）：五测对已落地实现编写，聚焦组 -race -v 全绿（5.3s）+ 全量 ./internal/... 三包 -race 绿；单 test 提交。gate 序列按 plan 显式指令执行（11-01/12-01 先例延续）

## Deviations from Plan

None - plan executed as written（上述 Decisions 段四条均为 plan 文本工具选型的落地裁决与 Claude's Discretion 范围内的断言通道设计，无 Rule 1-4 自动修复发生；D-06/D-07 配对落地、shared 逐字保留、锁序三规则等硬约束全部按 plan 执行）。

## Issues Encountered

- **Bash 工具 cwd 调用间重置**：`cd web && node ...` 后下一调用回到 repo root 导致 MODULE_NOT_FOUND——以绝对路径单命令规避（环境行为，非项目问题）
- 五测首次运行即全绿（无迭代修复）；gofmt（GOROOT go1.26.3）一次通过

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **12-03（PC-10/PC-11 停读续读）**：输出闭包现状（trySend 失败直踢）是唯一未触达的改动面；本 plan 的 debouncer 共用件与锁序注释（resizeMu 叶锁形态）为其 dwell 计时器 AfterFunc 纪律提供邻件参照；cl.pc != nil 门 + shared 半侧逐字保留的分岔形态可同构复用
- **12-04（phase12.mjs）**：S2 resize 隔离 / S3 ro 直通 stty 证据 / S4 门控限速——服务端语义已全部就位，协议层对照可直接消费；S1 shared Welcome.session 对照承担协议面 shared 半侧证据（D-11 归属）
- **12-05（收口）**：PC-05/PC-07 勾选证据链 = Go 五测（本 plan）+ phase12.mjs 六场景（12-04）+ phase12-dom 三场景（本 plan D2 + 12-01 D1/D3）
- **herdr 场景（Phase 14）**：ro 移动端转屏/拖窗 → 自身 area 渲染尺寸正确的前半链路（ro RESIZE 上行→服务端直通→SIGWINCH 重绘）本 plan 已通

## Threat Flags

无新增威胁面——T-12-04（钳制+防抖两防线经直通路径结构性保持）、T-12-05（锁序：回调函数体 hubMu 零命中源码断言 + 全量 -race）、T-12-06（ro INPUT 丢弃闸零改动 + 行为断言锁定）、T-12-SC（pnpm-lock 零 diff）四项 mitigate 处置全部落地验收。

---

*Phase: 12-per-client*
*Completed: 2026-09-04*

## Self-Check: PASSED

- 7/7 关键文件在场（resize.go/server.go/perclient.go/perclient_test.go/main.ts/dist/index.html/phase12-dom.mjs）
- 3/3 任务提交在场（9cedbc8 / 4aadd93 / bcaee49）
- 锚点 grep：resize.go newDebouncer ×4；server.go `cl.pc != nil` RESIZE 分支 ×1（+INPUT 间接门既有）；perclient.go resizeDeb ×5；main.ts `sessionMode !== 'per-client'` ×1；五测函数名各 ×1（perclient_test.go）
