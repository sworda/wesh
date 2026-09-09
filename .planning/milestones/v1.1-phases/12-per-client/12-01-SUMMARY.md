---
phase: 12-per-client
plan: 01
subsystem: protocol
tags: [websocket, welcome-frame, session-mode, xterm, terminal-reset, jsdom, per-client]

# Dependency graph
requires:
  - phase: 11-per-client
    provides: per-client 生命周期主干（升档分岔 upgradePerClient/Welcome 组帧点/重连=新进程语义 phase11.mjs S7）
  - phase: 10-mode-assembly
    provides: SessionMode/Options 接缝与 s.sessionMode 装配期归一值（五调用点统一传参的取值源）
provides:
  - WelcomePayload.Session 字段（json:"session"，恒序列化无 omitempty——v1.1.0 起公开 wire 契约，D-08 one-way 落定）
  - WelcomeFrame 五参签名（第 5 参 session string，五生产调用点恒传 s.sessionMode）
  - 前端 sessionMode per-connection 模式位（WELCOME 分支解析 + connect() 重置块清零，IN-01 登记口径）
  - WELCOME 分支统一 reset 判断（per-client → terminal.reset()；shared/缺键永不 reset）
  - web/uat/phase12-dom.mjs（D1 reset 全链 / D3 缺键不 reset 两场景 + SpyWebSocket 投递拦截面）
  - TestWelcomeFrameSession（proto）与 TestPerClientWelcomeSession（server e2e）
affects: [12-02 (D-07 ro RESIZE 第一闸按模式位放开——同源消费 sessionMode), 12-04 (phase12.mjs S1 Welcome.session 两模式对照), 13 (v1.1.0 公开协议面冻结基线)]

# Actuals (#2632) — 与 plan estimate (60000 tokens) 同标尺。
# 口径注记：源码 diff chars/4（internal/ + web/src/ + web/uat/phase12-dom.mjs），
# 排除 web/dist/index.html（构建再生产物，机械重建非作者工作，计入将使标尺失义）。
actuals:
  tokens: 9959
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: [] # 零新依赖红线保持（T-12-SC）：pnpm-lock.yaml 零 diff；jsdom/@xterm/headless 均既有
  patterns:
    - 恒序列化模式位键（G-05-1 Cols/Rows 同形态先例第三次沿用——「缺席 = 旧服务端」识别契约）
    - reset ⊇ clear 的 DOM 判别通道（alt-screen 1049l 弹回残影复活证据，@xterm/headless 探针实证后入 UAT）
    - SpyWebSocket 投递拦截注入（子类 onmessage setter 包装 + super 转发——旧服务端 wire 形态注入面）
key-files:
  created:
    - web/uat/phase12-dom.mjs
  modified:
    - internal/proto/proto.go
    - internal/proto/proto_test.go
    - internal/server/server.go
    - internal/server/perclient.go
    - internal/server/clients.go
    - internal/server/resize.go
    - internal/server/clients_test.go
    - internal/server/perclient_test.go
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "D-08 one-way 决策门按用户派发时确认 option-a 落定：session 字符串枚举（\"shared\"|\"per-client\"）恒序列化，与 --session-mode 同词同值域"
  - "模式位解析缺键语义取 sessionDims :655-664 容错同构（plan 自引先例）：键缺席（旧服务端）静默保持 shared——行为零漂移；键在场值非法 console.warn——T-12-01 缓解不变"
  - "D1 断言通道经 @xterm/headless 探针实证升级为 alt-screen 残影判别（1049l 弹回复活 vs reset 清 buffer+退 alt）——plan 原始 DOM 空白断言（reset ⊇ clear）两态皆过无判别力，探针证明 clear() 不退 alt screen 且不清其背后 normal buffer"
  - "PC-06 需求勾选留 phase 末 plan 12-05（ID 跨 12-01/04/05 共享，12-04 S1 shared 对照证据未落——Phase 11-01 先例延续）"

patterns-established:
  - "恒在键协议演化纪律前端半侧：解析段白名单赋值 + 缺键静默缺省 + 非法 warn 降级（sessionDims 段同构，后续加键复用）"
  - "jsdom 旧服务端形态注入：SpyWebSocket onmessage 子类 setter 投递拦截（Node 24 WebSocket.prototype.onmessage 为原型访问器）——后续缺键/畸形 wire 兼容场景可复用"

requirements-completed: []  # PC-06 跨 plan 共享（12-01/12-04/12-05），按 11-01 先例留 phase 末勾选

coverage:
  - id: D1
    description: "Welcome 帧 session 模式位协议面：两模式恒携 session 键且值正确（恒序列化无省略，additive 不挤压既有键）"
    requirement: PC-06
    verification:
      - kind: unit
        ref: "internal/proto/proto_test.go#TestWelcomeFrameSession"
        status: pass
      - kind: integration
        ref: "internal/server/perclient_test.go#TestPerClientWelcomeSession"
        status: pass
    human_judgment: false
  - id: D2
    description: "前端 reset 全链：per-client 1006 重连 → 新 WELCOME 模式位 → terminal.reset() 静默清屏（旧屏残影不复活、新内容完整、零新面板文案）"
    requirement: PC-06
    verification:
      - kind: automated_ui
        ref: "web/uat/phase12-dom.mjs#D1（D1a-D1f 6 check 全过）"
        status: pass
      - kind: automated_ui
        ref: "web/uat/phase06-dom.mjs 零修改重跑（shared 零漂移，40/40+2skip）"
        status: pass
    human_judgment: false
  - id: D3
    description: "缺 session 键 Welcome（旧服务端）→ 不 reset 防御性缺省（D-08 识别契约前端半侧，误 reset 即清掉同进程有效旧屏）"
    requirement: PC-06
    verification:
      - kind: automated_ui
        ref: "web/uat/phase12-dom.mjs#D3（D3a-D3c 3 check 全过）"
        status: pass
    human_judgment: false

# Metrics
duration: 38min
completed: 2026-09-04
status: complete
---

# Phase 12 Plan 01: PC-06 全链 tracer Summary

**Welcome 帧加 session 模式位恒序列化键（proto→五组帧调用点→wire→前端解析→per-client 重连静默 terminal.reset() 清旧屏残影），jsdom alt-screen 判别通道端到端锁定**

## Performance

- **Duration:** 38 min
- **Started:** 2026-09-04T11:14:33Z
- **Completed:** 2026-09-04T11:52:00Z
- **Tasks:** 3（Task 1 决策门 + Task 2 协议面 tracer + Task 3 前端 reset 全链）
- **Files modified:** 11

## Accomplishments

- **D-08 one-way 协议面落定**：WelcomePayload.Session（`json:"session"` 恒序列化无 omitempty，G-05-1 Cols/Rows 同形态先例）+ WelcomeFrame 第 5 参；五生产调用点（per-client attach / shared attach / promoteNextLocked 升格 / afterDrain 补发 / pushSessionDimsLocked 运行期推送）统一恒传 `s.sessionMode`——验收 grep 闸 perclient.go:1 / server.go:1 / clients.go:2 / resize.go:1 精确命中
- **既有测试七调用点机械加参零断言漂移**：proto_test.go 四处 "shared" 字面量（proto 包不引 server 常量，注释互指 clients.go:88-92）+ clients_test.go 三处 SessionModeShared 常量；diff 白名单审查确认断言行（期望值）逐字未动
- **前端 reset 全链（D-09/D-10）**：sessionMode per-connection 变量（connect() 重置块同批清零，IN-01 口径）+ WELCOME 分支模式位解析（白名单赋值/缺键静默/非法 warn）+ 统一 reset 判断（per-client → term.reset()，首连 no-op 等价零分支，静默零新文案）；既有 reconnecting 分支 term.clear() 逐字不动
- **phase12-dom.mjs（D-13）**：D1/D3 两场景 10/10 全过——D1 以 alt-screen 残影判别通道（旧会话 1049h 藏残影 → 重连 → 新会话 1049l 弹回）实证 reset 效应：无 reset 时旧 normal buffer 残影复活且新会话内容落入过期 alt buffer 被丢弃（探针 Case A 两症状，RED 期 D1d/D1e FAIL 双证）；D3 经 SpyWebSocket 投递拦截剥 session 键注入旧服务端形态，锁定不 reset
- **零回归三证据**：phase06-dom.mjs 零修改重跑 40/40+2skip 全绿（shared 零漂移）；`go test -race ./internal/... -count=1` 三包全绿（server 65.5s）；pnpm-lock.yaml 零 diff（T-12-SC 零新依赖红线）

## Task Commits

Each task was committed atomically:

1. **Task 1: D-08 one-way 确认门** — 用户派发时确认 option-a（session 字符串枚举恒序列化），无代码产出，门记录为通过
2. **Task 2: 协议面 tracer——WelcomePayload.session + WelcomeFrame 五参全调用点同步** - `6a4f292` (feat)
3. **Task 3: 前端 reset 全链 + dist 重建 + phase12-dom.mjs** - `c7924c3` (feat)

**Plan metadata:** docs commit（本 SUMMARY + STATE/ROADMAP 更新）

## Files Created/Modified

- `internal/proto/proto.go` - WelcomePayload.Session 字段（恒序列化）+ WelcomeFrame 五参 + 帧常量区/结构体/组帧函数三处注释（D-08 契约 + D-16 与 main.ts 互指）
- `internal/proto/proto_test.go` - TestWelcomeFrameSession（两值 round-trip + JSON map 恒在键断言）；既有四调用点加参
- `internal/server/perclient.go` - upgradePerClient Welcome 组帧点传 s.sessionMode（per-client attach 通道）
- `internal/server/server.go` - shared attach Welcome 组帧点传 s.sessionMode
- `internal/server/clients.go` - afterDrain 补发 + promoteNextLocked 升格两 Welcome 组帧点传 s.sessionMode
- `internal/server/resize.go` - pushSessionDimsLocked 运行期推送组帧点传 s.sessionMode
- `internal/server/clients_test.go` - 既有三调用点加 SessionModeShared（断言零改动）
- `internal/server/perclient_test.go` - TestPerClientWelcomeSession（per-client e2e：session=="per-client" 且 mode/cols/rows 共存不挤压）
- `web/src/main.ts` - sessionMode 变量 + connect() 清零 + WELCOME 分支解析/reset 判断 + 帧常量注释 session 键互指
- `web/dist/index.html` - dist 重建纳管（embed 链经 go build ./... 验证）
- `web/uat/phase12-dom.mjs` - 新建：D1/D3 两场景 + SpyWebSocket 投递拦截面 + assertOutputClean 红线

## Decisions Made

- **模式位解析缺键语义**：plan behavior 字面为「键缺席或值非法 → console.warn」，按 plan 自引的 sessionDims :655-664 容错同构落地——键缺席（旧服务端）静默保持 shared（零漂移、无兼容性噪音），键在场值非法才 warn。T-12-01 缓解语义（非法值不得触发 reset）完全保持
- **D1 断言通道升级（CONTEXT Claude's Discretion 范围内）**：plan 原始 D1 断言（新 WELCOME 后 terminalText 为空）经分析对 clear()/reset() 两态皆过（无判别力，非有效 RED）。以 @xterm/headless 探针实证 clear() ⊄ reset 的两个 DOM 可观测差异（alt screen 不退出；其背后 normal buffer 不清），将 D1 落地为 alt-screen 残影判别链路——RED 期 D1d（残影复活）/D1e（新内容落过期 alt buffer 被丢弃）双 FAIL 证明判别力，GREEN 期双 PASS
- **TDD 提交形态**：Task 2 按 plan 显式「一次性同提交（签名变更编译原子性）」单 feat 提交（11-01 先例延续）——RED 证据为新测试 5 参形态的编译失败运行；Task 3 RED（D1d/D1e FAIL，当前 dist 无 reset）→ GREEN（10/10）→ 单 feat 提交
- **Tracer 反馈门**：Task 2 提交后 tracer `<verify>`（go build + Welcome 测试组）已端到端重跑通过；interactive 模式默认应停 human-verify 门，但用户派发指令显式预授权 Task 2→Task 3 直进（决策门与续行授权均在 checkpoint_resolution 载明），按指令续行

## TDD Gate Compliance

- Task 2（tdd=true）：RED = TestWelcomeFrameSession/TestPerClientWelcomeSession 以 5 参形态写入后 `go test` 编译失败（`too many arguments in call to WelcomeFrame` / `wp.Session undefined`，11 项编译错误实证）；GREEN = 协议面落地后 Welcome 测试组全绿。单 feat 提交形态为 plan 显式指令（签名变更编译原子性），gate 序列 RED→GREEN 完整
- Task 3（tdd=true）：RED = phase12-dom.mjs 对当前 dist 运行，D1d/D1e FAIL（reset 缺位的两个残影症状）；GREEN = main.ts 实现后 10/10 全过。单 feat 提交

## Deviations from Plan

None - plan executed as written（上述 Decisions 段两条为 plan 文本歧义的裁决与 Claude's Discretion 范围内的断言通道选型，均已记录依据；无 Rule 1-4 自动修复发生）。

## Issues Encountered

- gofmt 对齐：Session 字段加入后 WelcomePayload struct 列对齐变化，GOROOT gofmt -w 一次修正（10-05 收口闸工具既定）——已含在 Task 2 提交内
- xterm.js write() 异步性：@xterm/headless 探针首版同步读 buffer 得空结果，改回调等待形态后实证成立（探针为临时工具已删，结论固化于 phase12-dom.mjs 头注释）

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **12-02（D-07）**：sessionMode 已就位，ro RESIZE 第一闸（main.ts sendResize `if (isRO) return`）可按模式位放开——服务端第二闸（D-06）同 plan 配对落地
- **12-04（phase12.mjs S1）**：Welcome.session 两模式对照断言可直接消费本 plan 协议面（shared 模式取值 "shared" 的协议层证据归其承担，D-11 单一归属）
- **v1.1.0 公开协议面**：session 键 one-way 契约已冻结——删键/改词即破坏已发布契约，Phase 13/14 不得触碰

---
*Phase: 12-per-client*
*Completed: 2026-09-04*

## Self-Check: PASSED

- 7/7 关键文件在场（proto.go/main.ts/phase12-dom.mjs/dist/index.html/perclient_test.go/proto_test.go/SUMMARY.md）
- 2/2 任务提交在场（6a4f292 / c7924c3）
- 锚点 grep：main.ts sessionMode ×5（声明/重置块/解析/reset 判断）；proto.go `Session string` ×1；phase12-dom.mjs 440 行（≥200 门槛）
