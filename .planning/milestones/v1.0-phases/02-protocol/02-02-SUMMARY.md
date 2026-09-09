---
phase: 02-protocol
plan: 02
subsystem: api
tags: [websocket, handshake, wesh.v1, readonly-mode, subprotocol, go, typescript, coder-websocket, xterm]

requires:
  - phase: 02-protocol
    provides: 02-01 proto 单一事实源（Hello/Welcome/Error 帧字节、Subprotocol、ModeRO/ModeRW、ErrVersionMismatch、ReadLimitPreAuth/PostAuth、DecodeHello/WelcomeFrame/ErrorFrame）
provides:
  - 服务端 wesh.v1 握手段：子协议双闸（400 预检 + Accept 回显 + assert 兜底）、4KiB 预认证读档、5s hello_timeout(1008)、抢跑/空帧/畸形 1002 直关、version_mismatch Error+1008、升档序列（Resize→Welcome→16KiB）
  - 服务端 ro INPUT 门（!writable 静默丢弃，RESIZE 放行）与 Options{Writable/HelloTimeout} 注入结构（02-03/02-04 字段预留）
  - --writable CLI flag（D-15 逐字 help）与 TestParseArgs writable 组
  - e2e 基建 dialHello（cols/rows 参数化签名）+ startTestServerWith；六测握手改造全绿 + TestHelloWelcome ro/rw 两半侧
  - 前端握手全链路：子协议建连、helloSent 门 + Hello 首帧携首尺寸、WELCOME(mode→disableStdin/[ro] 标题)、ERROR→lastError、onclose 按码分派（1000/1008/1009/1011/1013/default）
  - pty.Session fdMu：Resize↔Close master fd 竞态修复（closed 幂等语义 + TestResizeAfterClose 回归锁）
affects: [02-03 守卫链与握手违规七测, 02-04 保活, 02-05 上限 e2e, 02-06 期末 UAT]

actuals:
  tokens: 9353
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "握手状态机：首帧必须 Hello，违规只关 conn 落入读循环——终结经既有 wsDisconnected→terminate 单一路径收口（CONTEXT L92），零新增 exitf 调用点"
    - "conn 延迟上线：s.conn.Store 在 Welcome 发出后才生效——Welcome 恒为 S→C 首帧无时序竞态，预认证窗口零 PTY 输出"
    - "helloSent 门：term.onResize 常驻接线的首次 fit 抢跑由门吞掉，线上首帧恒为 Hello"
    - "logEvent stderr 单行事件三要素（remote/code/reason 机器串），Phase 8 slog 前的过渡形态（D-12②）"
    - "pty fdMu 护 Resize↔Close：os.File 读写经 fdmu 自同步，裸 Fd() ioctl（Setsize）必须应用层互斥"

key-files:
  created: []
  modified:
    - internal/server/server.go
    - internal/pty/spawn.go
    - internal/pty/io.go
    - internal/pty/io_test.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - internal/server/e2e_test.go
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "握手违规路径不落 plan 字面『return』：关 conn 后落入读循环，经既有 reader 终结→wsDisconnected→terminate 单一路径收口——CONTEXT L92 硬约束 + PATTERNS 收口纪律 + Phase 1 unknown-frame 先例 + 02-03 TestVersionMismatch waitExit(0) 可达性四处共同锁定"
  - "conn 在握手完成后才上线（plan 未规定时机）：结构性保证 Welcome 恒为 S→C 首帧（dialHello 首帧断言无时序竞态），并消除预认证窗口 PTY 输出流向未认证客户端的信息泄露面"
  - "pty.Session 增 fdMu+closed 修 Resize↔Close fd 竞态（-race 实测命中）：修复归位 fd 所有者，Phase 5 多客户端 resize 仲裁（MULTI-04）同受益"

patterns-established:
  - "Options 注入：生产直传字段（Writable）与测试可覆写字段（HelloTimeout，零值取默认常量）同构分组注释，02-03/02-04 按此追加 MaxHalfOpenPerIP/PingInterval/PongTimeout"
  - "dialHello 握手 helper：子协议 Dial → Hello 首帧 → Welcome 断言取 mode，cols/rows 参数化（02-03 以 (111,44) 复用）"
  - "startTestServerWith(opts) 主实现 + startTestServer(argv) 兼容包装（Writable:true 保持既有 echo 语义）的测试装配双层形态"

requirements-completed: [CORE-04, SEC-08, RES-01]

coverage:
  - id: D1
    description: "服务端 wesh.v1 握手段 + ro INPUT 门 + --writable flag：Hello→Welcome(ro/rw) 端到端打通，六个既有生命周期测试在握手语义下保持绿色，TestHelloWelcome 新绿"
    requirement: CORE-04
    verification:
      - kind: e2e
        ref: "internal/server/e2e_test.go#TestHelloWelcome"
        status: pass
      - kind: e2e
        ref: "go test -race -count=1 ./internal/server/（八测全 PASS，含六既有改造测）"
        status: pass
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs（writable 组）"
        status: pass
    human_judgment: false
  - id: D2
    description: "预认证窗口三要素就位：子协议 400 预检 + assert 双闸、4KiB 预认证读档、5s hello_timeout(1008)；抢跑/空帧/畸形 1002 零反馈 + logEvent 埋点"
    requirement: SEC-08
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... && grep 静态核查（握手段全要素/六埋点/ro 门/exitf 唯一收口）"
        status: pass
    human_judgment: false
  - id: D3
    description: "前端握手全链路：子协议建连 + Hello 首帧 + helloSent 门 + WELCOME/ERROR 分派 + onclose 按码文案"
    requirement: CORE-04
    verification:
      - kind: other
        ref: "pnpm -C web build（tsc 类型检查 + vite + gzip 全绿，产物时间戳已核验）"
        status: pass
    human_judgment: true
    rationale: "真实浏览器行为（ro 键盘无响应、[ro] 标题、onclose 文案、WS 帧面板）按 plan <human-check> 声明并入 02-06 期末 UAT 清单（config human_verify_mode: end-of-phase），构建绿不能替代浏览器确认"
  - id: D4
    description: "pty.Session Resize↔Close master fd 竞态修复（fdMu+closed 幂等语义）"
    requirement: RES-01
    verification:
      - kind: unit
        ref: "internal/pty/io_test.go#TestResizeAfterClose"
        status: pass
      - kind: e2e
        ref: "go test -race -count=3 ./internal/server/ ./internal/pty/（三轮无 race 报告）"
        status: pass
    human_judgment: false

duration: 1h 27m
completed: 2026-08-15
status: complete
---

# Phase 2 Plan 02: 握手 tracer——wesh.v1 端到端脊梁 Summary

**wesh.v1 握手端到端打通：浏览器子协议建连 → onopen Hello 首帧携首尺寸 → 服务端预认证窗口（子协议双闸/4KiB/5s 超时/抢跑 1002）→ Welcome{mode} 下发升档 16KiB → ro 服务端丢 INPUT/RESIZE 放行/rw 正常回显；默认只读用户路径成立，六个既有 e2e 在握手语义下全绿**

## Performance

- **Duration:** 1h 27m
- **Started:** 2026-08-15T05:53:03Z
- **Completed:** 2026-08-15T07:20:46Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments

- 服务端握手段全要素落地：headerHasToken 子协议 400 预检（token 拆分防 `wesh.v1.evil` 前缀绕过）→ 409 原子门 → Accept 回显 + `c.Subprotocol()` assert 双闸 → 4KiB 预认证档 → AfterFunc hello_timeout(1008，读 ctx 恒无 deadline) → 首帧状态机（empty_frame/frame_before_hello/malformed_hello 1002 零 Error 帧；version_mismatch 先 Error 帧后 1008 同名机器串）→ 升档序列 close(helloDone)→Resize(Hello 首尺寸)→Welcome→16KiB
- 默认只读为真实服务端边界：`case proto.Input` 下 `if !s.writable { continue }` 静默丢弃（防按键洪水不打日志），RESIZE 放行（D-13）；`--writable` flag（D-15 逐字 help）开启后 Welcome{mode:"rw"}、INPUT 正常 echo（TestHelloWelcome ro/rw 两半侧锁定）
- 前端握手全链路：`new WebSocket(url, [SUBPROTOCOL])`、helloSent 门保证线上首帧恒为 Hello（term.onResize 常驻接线的首次 fit 抢跑被门吞掉——不门则真实浏览器主路径被 1002 直关，Go e2e 覆盖不到此时序）、WELCOME ro→disableStdin+`[ro] ` 标题、ERROR→lastError、onclose 按码分派（只认 code 不认 reason，1009 文案不提 flag）
- 全部既有 e2e 经 dialHello/startTestServerWith 握手改造保持绿色；TestSecondClient409 第二次 Dial 补带子协议（防 400 拦截造成负例语义漂移）
- logEvent stderr 单行事件六处埋点（hello_timeout/empty_frame/frame_before_hello/malformed_hello/version_mismatch/subprotocol_required，三要素 remote/code/reason）
- 顺带修复 pty.Session Resize↔Close master fd 竞态（-race 实测命中，详见偏差 3）

## Task Commits

1. **Task 1 (tracer): 服务端握手段 + ro 门 + --writable + e2e 全量握手改造** - `2f85753` (feat)
2. **Task 2: 前端握手全链路** - `8a0d9af` (feat)

**Plan metadata:** 见下方 docs 提交（docs(02-02): complete ...）

**Tracer 反馈门：** Task 1 提交后立即重跑完整 `<verify>`（go build+vet+`-race` 三包）全绿，方进入 Task 2 扩展（自主模式：verify 为全自动命令，项目 human_verify_mode=end-of-phase，无人工判断成分）。

## Files Created/Modified

- `internal/server/server.go` - Options/New 新签名、headerHasToken、Attach 握手段+ro 门、logEvent、defaultHelloTimeout(5s)
- `cmd/wesh/main.go` - config.writable + --writable flag + server.New 新装配
- `cmd/wesh/main_test.go` - TestParseArgs 表驱动加 wantWritable 字段与 writable 组
- `internal/server/e2e_test.go` - dialHello + startTestServerWith/兼容包装 + 六测握手改造 + TestHelloWelcome
- `web/src/main.ts` - HELLO/WELCOME/ERROR/SUBPROTOCOL 常量、helloSent/lastError、onopen Hello 首帧、onmessage switch、onclose 按码分派
- `web/dist/index.html` - 构建产物随源更新（Phase 1 决策：dist 提交真实产物）
- `internal/pty/spawn.go` - Session 增 fdMu/closed 字段（竞态修复，偏差 3）
- `internal/pty/io.go` - Resize/Close 经 fdMu 互斥 + closed 幂等语义
- `internal/pty/io_test.go` - TestResizeAfterClose 回归锁

## Decisions Made

- **握手违规路径收口方式（对 plan 字面 "return" 的裁决）：** 四个违规路径（empty_frame/frame_before_hello/malformed_hello/version_mismatch）关 conn 后**落入读循环**而非 return——下一拍 `c.Read` 错误经既有 `wsDisconnected→terminate` 单一路径收口。依据四处锁定：① CONTEXT L92/PATTERNS 硬纪律"一切新终结全部经『关 conn → reader 终结 → wsDisconnected → terminate』既有单一路径收口"；② Phase 1 unknown-frame 先例即此形态（server.go:114 close 后循环，TestUnknownFrame1002 断言 waitExit(0)）；③ 02-03-PLAN TestVersionMismatch 锁定 `waitExit(0)`——若按字面 return 无 exitf，该测试届时必红；④ 本 plan 验收"无新增 exitf 调用点"在此形态下零新调用点即满足。**02-03 执行者注意：per-IP release 挂点为"拒绝路径关 conn 前"与"Hello 完成后"，恰好一次不变量不受影响。**
- **conn 延迟到握手完成后上线：** plan 未规定 `s.conn.Store` 时机。Phase 1 位置（Accept 后即上线）在握手语义下有两个缺陷：① 子进程输出可与 Welcome 争 S→C 首帧（dialHello 首帧断言存在理论时序竞态）；② 预认证窗口内 PTY 输出流向未认证客户端（信息泄露面）。现 Welcome 发出并升档后才 `conn.Store`——Welcome 恒为首帧、未认证零输出，双重结构性保证。
- **fd 竞态修复归位 pty.Session（而非 server 层互斥）：** fd 生命周期的不变量天然属 fd 所有者；server 层互斥会把同样的坑留给 Phase 5 多客户端 resize 仲裁（MULTI-04 多 goroutine Resize）等未来调用方。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking / 跨 plan 契约冲突] 握手违规路径不落字面 "return"，落入读循环经既有单一路径收口**
- **Found during:** Task 1（握手段实现前的跨文档一致性核对）
- **Issue:** plan action 文本对四个违规路径写 "logEvent 后 return"；但 PATTERNS（同日 planner 裁决修订）规定一切新终结经"关 conn→reader 终结→wsDisconnected→terminate"收口，且 02-03-PLAN TestVersionMismatch 明确断言 `waitExit(0)`——字面 return 后无人调 exitf，该锁定测试必红；Phase 1 unknown-frame 路径（close 后继续循环）与 TestUnknownFrame1002 的 waitExit(0) 断言是库内既有先例
- **Fix:** 违规路径仅关 conn（满足验收"仅关 conn、无新增 exitf 调用点"——零新调用点），落入数据面读循环由下一拍 reader 错误走既有 D-11 收口；assert 兜底路径（理论不可达）保持 plan 字面"释放门位直返"作防御性退化
- **Files modified:** internal/server/server.go（注释记录裁决理由）
- **Verification:** TestUnknownFrame1002（1002+waitExit(0)）等六测全绿；02-03 TestVersionMismatch/TestHelloTimeout 的 waitExit(0) 路径经走查可达
- **Committed in:** `2f85753`（Task 1 提交）

**2. [Rule 2 - Missing Critical] conn 推迟到 Welcome 发出后才上线**
- **Found during:** Task 1（Attach 重写时审查 s.conn.Store 时机）
- **Issue:** plan 未规定 Store 时机；沿用 Phase 1 位置（Accept 后即上线）则 ① onChunk 可将子进程输出抢在 Welcome 前写出（dialHello"首帧必为 Welcome"断言存在理论时序竞态，-race CI 上有 flake 风险）② 预认证窗口内（≤5s）PTY 输出流向未完成握手的客户端——本 plan threat model 未覆盖的信息泄露面
- **Fix:** `s.conn.Store(c)` 移至升档序列末尾（Welcome 发出 + 16KiB 升档后）；预认证窗口 onChunk 一律 drain（D-12 语义自然延伸）；lifecycle 1000 关闭目标不受影响（握手完成后 conn 已在位）
- **Files modified:** internal/server/server.go
- **Verification:** 八测全绿；TestClientDisconnectSIGHUP（READY 输出经握手后正常送达）等输出型测试无回归
- **Committed in:** `2f85753`（Task 1 提交）

**3. [Rule 1 - Bug] pty.Session Resize↔Close master fd 竞态修复**
- **Found during:** Task 1（`-race` 全量测试命中：WARNING: DATA RACE——`internal/poll.FD.destroy`（lifecycle→Drain→Close）与 `creack/pty Setsize` 裸 `Fd()`（Attach→Resize，winsize_unix.go）无序并发；竞态在三次连跑中间歇复现）
- **Issue:** os.File 的 Read/Write/Close 经内部 fdmu 自同步，唯独 Setsize 裸取 `Fd()` 不过 fdmu——握手成功路径新增的 `sess.Resize(h.Cols, h.Rows)` 首次让该裸访问与 lifecycle Close 并发；fd 关闭后可被内核回收重用，裸 TIOCSWINSZ 可能打到无关 fd。本任务新调用点暴露的潜伏缺陷（Phase 1 无 e2e 触发该并发），02-03 TestReadOnlyAllowsResize 与 Phase 5 多客户端 resize 必踩
- **Fix:** Session 增 `fdMu sync.Mutex` + `closed bool`：Resize/Close 互斥，Close 幂等，Resize 见 closed 返回 `os.ErrClosed`（Attach 忽略 Resize 错误，语义不变）；Read 不入锁（读阻塞期间 Close 会被拖死，注释钉死）；新增 TestResizeAfterClose 回归锁
- **Files modified:** internal/pty/spawn.go、internal/pty/io.go、internal/pty/io_test.go
- **Verification:** `go test -race -count=3 ./internal/server/ ./internal/pty/` 三轮无 race 报告；TestResizeAfterClose 绿
- **Committed in:** `2f85753`（Task 1 提交）

---

**Total deviations:** 3 auto-fixed (1 blocking/跨 plan 契约, 1 missing critical, 1 bug)
**Impact on plan:** 三处均为正确性/安全性必要修复，无范围蔓延；deviation 1/2 改变了 plan 两处实现细节字面（return→落循环、Store 时机），全部行为语义与验收标准保持或增强。

## Issues Encountered

- `-race` 间歇报 Resize↔Close 竞态（单跑 TestHelloWelcome 不报，全量跑报）——定位后经 race trace 确认为 creack/pty Setsize 裸 `Fd()` 路径，按偏差 3 修复并三轮连跑验证。
- 跨 plan 文档冲突（02-02 字面 "return" vs 02-03 TestVersionMismatch 锁定 waitExit(0) vs PATTERNS 收口纪律）——按偏差 1 裁决，已在本 SUMMARY Decisions 与代码注释中为 02-03 执行者留指引。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 02-03 挂点全部就绪：Options 结构预留（MaxHalfOpenPerIP 零值取默认 8 的注释位已在 Options 注释声明）、守卫区注释标明 per-IP 429 插入位置（子协议 400 与 409 之间，planner 裁决顺序）、dialHello 参数化签名供 TestReadOnlyAllowsResize 以 (111,44) 复用、startTestServerWith 承载 Options 注入
- 02-04 ping/pong：Options 追加 PingInterval/PongTimeout 的同构注释模式已建立
- 02-05：logEvent 埋点清单已按 plan 建立六处，ErrMessageTooBig 钩子（1009 stderr 事件）挂点在两个读循环错误分支
- 02-06 期末 UAT 待确认项（来自 Task 2 `<human-check>`）：ro 标题前缀/键盘无响应、onclose 各码文案、devtools 观察 WS 帧
- 无阻塞项

## Self-Check: PASSED

- [x] 九个修改文件与 02-02-SUMMARY.md 均存在于磁盘
- [x] 提交 `2f85753` / `8a0d9af` 均见于 `git log`
- [x] `logEvent(remote` 六处埋点 + 一处定义（grep 计数 7）
- [x] plan 级 verification 全绿：`go build ./... && go vet ./...` 退出 0；`go test -race -count=1 ./internal/... ./cmd/...` 四包全 PASS（server 八测含 TestHelloWelcome；另 `-count=3` 连跑无 race）；`pnpm -C web build` 退出 0 且产物时间戳已核验
- [x] 无意外文件删除（两次任务提交的 `git diff --diff-filter=D` 均为空）

---
*Phase: 02-protocol*
*Completed: 2026-08-15*
