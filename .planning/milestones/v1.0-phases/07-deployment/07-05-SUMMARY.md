---
phase: 07-deployment
plan: 05
subsystem: infra
tags: [graceful-shutdown, close-code-1001, going-away, sigterm, sigint, notifycontext, stop-signal, open-browser, xdg-open, systemd, tdd]

requires:
  - phase: 07-deployment/07-04
    provides: server.stopSignal/stopTimeout 字段（Options 单一通道）+ SignalGroup 进程组信号泛化 + trap 夹具形态（落盘标记同步）
  - phase: 07-deployment/07-01
    provides: shareURLRO/shareURLRW 拼串单一事实源局部变量（--open 消费点）+ 前端相对 URL 形态
  - phase: 06-session-lifecycle/06-01
    provides: lifecycle EXIT 帧广播先例形态（exiting 门/hubMu 快照/每客户端 goroutine 同步写）+ accept-255 断言常量
  - phase: 06-session-lifecycle/06-05
    provides: phase06-dom.mjs synthClose/_savedClose CloseEvent 驱动夹具（D11 二次驱动形态来源）
  - phase: 02-protocol
    provides: proto.go 关闭码纪律块 1001 占位（P2 D-08，本 plan 翻正启用）
provides:
  - "Server.Shutdown()（internal/server/server.go）：hubMu 内置 exiting 门 → 注册表快照 → 每客户端 goroutine 同步 Close(1001 Going Away, server_shutting_down)（无 EXIT 帧前置，Close 内建 5s+5s 上界不再盒，OQ3）→ wg.Wait → stop-signal 序列（07-04 字段复用，stopTimeout 后 sleep 补 SIGKILL，ESRCH 幂等）——不调 exitf，lifecycle 单一收口（P1 硬约束零新 exit 分支）"
  - "proto.go 关闭码纪律块 1001 翻正为启用态（D-08 占位兑现——发送路径 server/server.go Shutdown、库常量 websocket.StatusGoingAway、close reason 机器串 server_shutting_down，1013 先例逐字同构）"
  - "cmd/wesh/main.go：signal.NotifyContext 捕获 SIGTERM/SIGINT → goroutine srv.Shutdown()（hs 装配后 Serve 前挂点，defer stopSignals 恢复默认）"
  - "--open CLI 公开契约（D-26/D-27 one-way）：openBrowser(url) desktop 经 xdg-open（Linux）/open（macOS）拉起分享链接（--writable 开 rw 否则 ro，含 token 免交互；shareURL 拼串单一事实源消费）；headless（无 DISPLAY 且无 WAYLAND_DISPLAY）stderr 提示后跳过不阻断；失败仅警告"
  - "validateStartup --socket×--open 组合矛盾 exit 2 双 flag 名（RESEARCH OQ1 建议行落地，D-11 socket 早退之前判定——unix socket 无 http URL 可开）"
  - "前端 onclose case 1001 → 'Server shutting down' 终态面板（逐字文案），不进 CORE-05 重连循环（仅 1006 触发，D-23 prohibition 回归锁）；main.ts 帧常量注释区 1001 启用行与 proto.go 互指"
  - "phase06-dom.mjs D11 场景：synthClose(1001) 面板+守候窗零构造；重连上下文 1001 → stopReconnect 循环终止+面板分派"
affects: [phase-07 后续 plans（07-06 配置文件 open 键——RESEARCH Pattern 4 fileConfig.Open 已预留 / 07-07 phase07.mjs 1001 关停序列与 --open 场景 / 07-08 README 优雅下线节与 flagged_assumptions macOS open 人工复核）, phase-08, verify-work]

actuals:
  tokens: 25011
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "1001 优雅下线触发源形态：SIGTERM/INT 是触发源不是 exitf 分支——NotifyContext goroutine → Shutdown（1001 广播 + stop-signal 序列）→ 子进程死亡经既有 lifecycle 收口 exitf 恰好一次（P1 硬约束零新 exit 分支；与 06-02 exit-when-empty 同族第三触发源）"
    - "无 EXIT 帧前置的关闭码终结语义：进程未退出时终结语义由关闭码承载（conn.Close(1001) 自带关闭帧），Close 内建 5s+5s 上界不再盒（OQ3；stall 端最坏 10s 不阻塞进程退出——exitf 由 lifecycle 子进程路径并发收口）"
    - "WS Close 握手测试形态：客户端读循环必须先于服务端 Close 启动（库 close 回显走读路径——无在读 Read 则服务端 Close 等满内建 5s 上界；真实浏览器协议栈透明回显无此窗口）"
    - "--open 三形态纪律：desktop 拉起（exec.Command argv 分离不经 shell，T-07-05b）/ headless stderr 提示跳过不阻断（常态部署形态）/ --socket×--open 配置矛盾 exit 2（给了无法兑现的 flag 组合 = 配置错误，「显式」哲学一贯性）"

key-files:
  created:
    - internal/server/shutdown_test.go
  modified:
    - internal/server/server.go
    - internal/proto/proto.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - web/src/main.ts
    - web/uat/phase06-dom.mjs
    - web/dist/index.html

key-decisions:
  - "Shutdown 的 stop-signal 序列取同步 sleep 形态（plan behavior 逐字）：stopChildLocked（07-04）的 AfterFunc 异步补 KILL 服务 hubMu 持有方（exit-when-empty），Shutdown 不持 hubMu 同步 sleep 更直白且 KILL 测序确定——字段复用（Options 单一通道）是硬约束，函数复用非约束（plan read_first 仅指名字段）"
  - "测试装配取 startShutdownServerWith 本地变体（plan 字面 startTrackedServerWith 按意图修正）：Shutdown 直接调用面结构性需要 *server.Server 句柄，startTrackedServerWith 三元组不暴露；装配序列逐字复制并额外返回 srv（waitHandlers 同步边不需要——本组不断言 stderr 事件行）"
  - "parseArgs 头注释 flag 计数 28→29 并补 --open 行（Rule 1 文档漂移修复，07-02 先例第二次沿用）"

patterns-established:
  - "服务端主动 Close 的 Go 测试夹具：readCloseAsync（Shutdown 前启动客户端读循环 goroutine + 缓冲 channel 上报）——Close 握手应答面，避免等满 5s 内建上界（c.Read 不可并发纪律不受影响：dialHello 已返回）"
  - "fake xdg-open PATH 前置断言 argv 的 --open 测试形态（t.TempDir 可执行脚本记录 argv 落盘 + 轮询同步——exec.Command(...).Start() 不等待）；darwin 分支注释登记不做运行时断言（CI macOS 同款测试形态即整体 Skip）"

requirements-completed: [OPS-11]

coverage:
  - id: D1
    description: "1001 优雅下线服务端全链：Shutdown 向全部已注册客户端发 Close 1001（reason server_shutting_down）→ 子进程进程组收 stop-signal 序列（默认 HUP 信号死亡 = -1 桩码）→ lifecycle 收口 exitf 恰好一次；stop-timeout 时 TERM 被 trap 忽略后补 SIGKILL 终结"
    requirement: OPS-11
    verification:
      - kind: unit
        ref: "internal/server/shutdown_test.go#TestShutdown1001/#TestShutdownStopTimeout（0.21s/0.51s）+ go test ./internal/server -count=1 全绿（45.8s）+ go test -race -count=1 ./... 五包全绿（54.3s）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（wesh -- bash + attach 后 kill -TERM：客户端 close 1001 reason server_shutting_down，wesh 进程退出码 255——accept-255 同源）"
        status: pass
    human_judgment: false
  - id: D2
    description: "proto.go 纪律块 1001 翻正启用 + 前端 onclose case 1001 'Server shutting down' 终态面板（逐字文案）不进重连循环；重连上下文收 1001 → stopReconnect 循环终止+面板分派；1000/1006/1013 既有分派零回归；main.ts 注释与 proto.go 互指"
    requirement: OPS-11
    verification:
      - kind: unit
        ref: "node web/uat/phase06-dom.mjs（37/37 含 D11a-d 四断言）+ phase04-dom（37/37）+ phase05-dom（19/19）+ grep -c '1001 优雅下线已于 Phase 7 启用' proto.go == 1 + grep -c 'Server shutting down' web/dist/index.html == 1"
        status: pass
    human_judgment: false
  - id: D3
    description: "--open 三形态：desktop 经 fake xdg-open 收到 rw 分享链接（--writable；与 stdout 打印同一 URL 单一事实源）；headless stderr 提示 'no display detected (headless), skipping browser launch' 跳过不阻断；--socket×--open exit 2 文案含 --open 与 --socket"
    requirement: OPS-11
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestOpenBrowser（headless 跳过 + fake xdg-open argv 断言两场景）/#TestStartupMatrix（socket×open 冲突行 + open 放行行）/#TestParseArgs（wantOpen 行）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（DISPLAY=:99 --open --writable → fake xdg-open argv == share read-write 行逐字一致；headless → 提示行+正常服务+xdg-open 未调用；--socket×--open → exit 2 双 flag 名）"
        status: pass
    human_judgment: false
  - id: D4
    description: "SIGTERM/SIGINT 捕获接线：run() hs 装配后 Serve 前 signal.NotifyContext + goroutine srv.Shutdown()——不调 exitf（触发源非分支，lifecycle 单一收口 P1 硬约束）；defer stopSignals 恢复默认"
    requirement: OPS-11
    verification:
      - kind: unit
        ref: "grep -c 'signal.NotifyContext' cmd/wesh/main.go == 1 + go vet ./... 零告警 + go test -race -count=1 ./cmd/wesh（1.0s）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（同 D1——kill -TERM 全链：NotifyContext → Shutdown → 1001 → lifecycle exitf → 进程 255）"
        status: pass
    human_judgment: false

duration: 39min
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 05: 优雅下线（1001）与自动开浏览器（--open）Summary

**D-23 优雅下线全链落地：wesh 捕获 SIGTERM/SIGINT → Server.Shutdown() 向全部客户端广播 Close 1001 Going Away（reason server_shutting_down）→ 子进程进程组收 stop-signal 序列（07-04 字段复用，stop-timeout 补 KILL）→ 既有 lifecycle 单一收口 exitf（零新 exit 分支）；前端 onclose case 1001 落 'Server shutting down' 终态面板不进重连循环（systemd restart UX 闭环）；OPS-11 --open 三形态（desktop xdg-open/open 拉起 rw/ro 分享链接、headless stderr 提示跳过不阻断、--socket×--open exit 2）；proto.go 纪律块 1001 翻正启用，前后端注释互指。**

## Performance

- **Duration:** 39min
- **Started:** 2026-08-26T04:08:14Z
- **Completed:** 2026-08-26T04:48:05Z
- **Tasks:** 3/3
- **Files modified:** 8（新建 1：shutdown_test.go；修改 7；合计 467 insertions / 9 deletions，plan 全 diff 100046 chars）

## Accomplishments

- `wesh -- bash` + attach 后 `kill -TERM <wesh_pid>` 真实二进制冒烟：客户端收到 close 1001（reason `server_shutting_down`），wesh 进程以 255 退出（accept-255 同源——默认 HUP 信号死亡 exitf(-1)）——SIGTERM → NotifyContext → Shutdown（exiting 门 + 注册表快照 + 每客户端 goroutine 同步 Close(1001)，无 EXIT 帧前置）→ stop-signal 序列 → lifecycle EXIT+1000 广播在已空注册表上零循环 → terminate 收口，全链零新 exit 分支
- `DISPLAY=:99 wesh --open --writable -- bash`（fake xdg-open PATH 前置拦截）真实二进制冒烟：xdg-open 收到的 argv[1] 与 stdout `share read-write:` 行逐字一致（拼串单一事实源兑现）；headless 同命令：stderr `wesh: --open: no display detected (headless), skipping browser launch` 后正常服务（xdg-open 未被调用）；`wesh --socket /tmp/x.sock --open -- bash` → exit 2 文案含 --open 与 --socket
- 前端 1001 分派锁定：phase06-dom D11 四断言（稳态 1001 → 'Server shutting down' 逐字面版 + 2.5s 守候窗零新连接；重连上下文 1001 → stopReconnect 循环终止 + 终态面板 + 退避定时器已清）——prohibition「1001 绝不触发重连」运行时回归锁落地；1000/1006/1013 既有分派经 D1/D2/D3/D7 零回归
- 协议层翻正：proto.go 关闭码纪律块 1001 从「Phase 7 启用，本期占位不实现」翻正为启用态（1013 先例逐字同构——发送路径/库常量/reason 机器串三要素），main.ts 帧常量注释区同步补行互指

## Task Commits

Task 1/2 各 TDD 两提交（RED/GREEN），Task 3 单提交：

1. **Task 1 RED: TestShutdown1001/TestShutdownStopTimeout 失败测试** - `adbc5be` (test)
2. **Task 1 GREEN: Server.Shutdown with 1001 Going Away broadcast** - `a6128a8` (feat)
3. **Task 2 RED: --open/openBrowser/StartupMatrix 失败测试** - `6d40125` (test)
4. **Task 2 GREEN: SIGTERM/SIGINT graceful shutdown wiring and --open browser launch** - `32662a6` (feat)
5. **Task 3: dispatch 1001 to shutdown panel, rebuild dist** - `ebc66e9` (feat)

**Plan metadata:** docs 提交在本 SUMMARY 之后（`docs(07-05): complete graceful-shutdown plan`，hash 见 git log）。

## Files Created/Modified

- `internal/server/shutdown_test.go`【新】- TestShutdown1001（1001 + reason + exitf(-1) 恰好一次）/ TestShutdownStopTimeout（TERM 忽略 → stop-timeout 补 KILL）+ startShutdownServerWith 装配变体 + readCloseAsync 夹具（文件头登记两纪律）
- `internal/server/server.go` - `func (s *Server) Shutdown()`（exiting 门复用 lifecycle 先例；无 EXIT 帧前置/OQ3 不再盒/不调 exitf/1001×EXIT 竞态论证/字段复用五段注释纪律）
- `internal/proto/proto.go` - 关闭码纪律块 1001 占位行翻正为启用态（全集表述保持）
- `cmd/wesh/main.go` - config.open 字段 + --open flag 注册 + openBrowser(url)（headless 检测只在 linux 分支判定，darwin 直接 open）+ run() NotifyContext 接线 + shareURLRO/RW hoist 两消费点共用 + validateStartup --socket×--open 冲突行 + parseArgs 头注释 28→29
- `cmd/wesh/main_test.go` - TestOpenBrowser 两场景 + TestStartupMatrix 两行 + TestParseArgs wantOpen 断言位与新行（03-04 命名字段扩展先例）
- `web/src/main.ts` - onclose switch case 1001（逐字文案 + D-23 注释登记 prohibition）+ 帧常量注释区 1001 启用行
- `web/uat/phase06-dom.mjs` - D11 场景（两实例四断言）+ 头部覆盖清单 D11 行 + 场景数组登记
- `web/dist/index.html` - 重建入库（'Server shutting down' 字面量进产物，grep == 1）

## Decisions Made

- **Shutdown 的 stop-signal 序列取 plan behavior 逐字的同步 sleep 形态**——07-04 stopChildLocked 的 AfterFunc 异步补 KILL 专为 hubMu 持有方（exit-when-empty 两触发点）设计；Shutdown 不持 hubMu，`SignalGroup(stopSignal) → sleep(stopTimeout) → SignalGroup(SIGKILL)` 同步形态更直白且 KILL 测序确定。硬约束 = 字段复用（s.stopSignal/s.stopTimeout 经 Options 单一通道，双写即漂移），plan read_first 亦仅指名字段未指名函数。
- **测试装配取 startShutdownServerWith 本地变体**（详见 Deviations #1）——plan 字面「startTrackedServerWith 起实例」与「调 s.Shutdown()」结构性矛盾（srv 句柄不暴露），按意图修正为同形态变体，装配序列逐字复制。
- **parseArgs 头注释 flag 计数 28→29**（Rule 1 文档漂移修复，07-02 先例第二次沿用）——同区域主题直接相关一次修正。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - 计划内矛盾] startShutdownServerWith 本地变体替代 plan 字面 startTrackedServerWith**
- **Found during:** Task 1 RED 编写
- **Issue:** plan behavior 字面「startTrackedServerWith 起实例 + 握手客户端 → 调 s.Shutdown()」——startTrackedServerWith 只返回 (exitCh, wsURL, waitHandlers) 三元组，不暴露 *server.Server 句柄，Shutdown 直接调用面结构性不可达
- **Fix:** shutdown_test.go 内落同形态本地变体 startShutdownServerWith——装配序列逐字复制（pty.Start → New → Listen → Cleanup → Serve）并额外返回 srv 句柄；waitHandlers 同步边不需要（本组不断言 stderr 事件行）；文件头登记修正依据
- **Files modified:** internal/server/shutdown_test.go
- **Verification:** 两测试 PASS；全量套件零回归
- **Committed in:** adbc5be

**2. [Rule 1 - Bug] 顺序式「Shutdown() 后 Read」使服务端 Close 等满内建 5s 上界**
- **Found during:** Task 1 GREEN 首次验证（TestShutdown1001 5.21s / TestShutdownStopTimeout 5.51s——Close 握手超时形态）
- **Issue:** coder/websocket 的 close 回显走读路径——测试客户端无在读 Read 时，服务端 `conn.Close(1001)` 等不到对端回显，每客户端等满内建 5s 上界（close.go:87-89）；真实浏览器协议栈透明回显无此窗口，纯测试夹具缺陷
- **Fix:** readCloseAsync 夹具——Shutdown 前启动客户端读循环 goroutine（plan behavior「客户端读循环收到 CloseError」字面形态），缓冲 channel 上报首个错误，主 goroutine 5s 护栏断言 CloseError；c.Read 不可并发纪律不受影响（dialHello 已返回，读循环 goroutine 是唯一读者）
- **Files modified:** internal/server/shutdown_test.go
- **Verification:** 两测试 0.21s/0.51s PASS（原 5.21s/5.51s 同样 PASS 但恒等 5s 上界）；-race 全绿
- **Committed in:** a6128a8

**3. [Rule 1 - 文档漂移] parseArgs 头注释 flag 计数 28→29 并补 --open 行**
- **Found during:** Task 2 GREEN
- **Issue:** 07-04 落地后头注释计数为 28，本 plan 再加 --open 使计数陈旧
- **Fix:** 计数 28→29 + Phase 7 段补 --open 行（07-02 先例第二次沿用——同区域主题直接相关一次修正）
- **Files modified:** cmd/wesh/main.go
- **Verification:** go vet/build 零告警
- **Committed in:** 32662a6

**4. [Rule 3 - gofmt] 新增代码两处 CJK 注释/字段对齐漂移机械修正**
- **Found during:** Task 1/Task 2 GREEN 验证（GOROOT gofmt 判定）
- **Issue:** proto.go 翻正行 `//（D-08` 缺空格（GOROOT gofmt CJK 注释规则）；main_test.go 新增 wantOpen 字段改变列对齐（wantUid/wantGid 注释列漂移两行）
- **Fix:** 手工补空格 + gofmt -w 两行机械修正（均为本 plan 新增区域，两既有漂移文件 multi_test.go/slowclient_test.go 按 SCOPE BOUNDARY 未触碰）
- **Files modified:** internal/proto/proto.go, cmd/wesh/main_test.go
- **Verification:** GOROOT gofmt 漂移清单 == 两既有文件（本 plan 零新增）
- **Committed in:** a6128a8 / 32662a6

---

**Total deviations:** 4 auto-fixed（1 计划内矛盾 Rule 3，2 Rule 1，1 gofmt Rule 3）
**Impact on plan:** 全部 must_have truths 与 prohibition 逐字达成——1001 广播 + reason server_shutting_down / 子进程 stop-signal 序列 / lifecycle 单一收口零新 exit 分支 / 前端 1001 面板不重连（1000/1006/1013 逐字节不变）/ --open desktop 与 headless 跳过 / --socket×--open exit 2 / proto.go 纪律块翻正。无范围蔓延。

## TDD Gate Compliance

Task 1/2 均 tdd="true"，gate 序列逐 task 核验（git log 顺序 + RED 失败证据）：

| Task | RED（test 提交，失败确认） | GREEN（feat 提交，全绿确认） | 序列 |
|------|---------------------------|------------------------------|------|
| 1 | `adbc5be`（编译失败——srv.Shutdown undefined ×2） | `a6128a8` | ✓ |
| 2 | `6d40125`（编译失败——openBrowser undefined ×2 + cfg.open undefined ×2 + unknown field open ×2） | `32662a6` | ✓ |

RED 阶段无测试意外通过（fail-fast 规则未触发；两 RED 均为编译失败形态——新 API/字段未存在，07-04 先例同属有效 RED）。REFACTOR 无需（GREEN 即最终形态；Deviation #2 的夹具修正发生在 GREEN 提交前，非行为变更）。Task 3 为 type="auto" 非 TDD（前端分派 + UAT 场景 + dist 重建，无独立 RED 门）。

## Issues Encountered

- **coder/websocket Close 握手语义实证**（本 plan 最重要机制发现）：库 close 回显由读路径驱动——Go 测试客户端无在读 Read 时服务端主动 Close 必等满 5s 内建上界。该形态不影响真实浏览器（协议栈透明回显），但一切「服务端主动 Close + Go 测试客户端」的后续测试（07-07 phase07.mjs 的 Node 客户端有常驻事件循环不受影响；Go 侧新增 Shutdown 类测试）须沿用 readCloseAsync 夹具形态（patterns-established 已登记）。

## User Setup Required

None - no external service configuration required.

## Threat Flags

None——全部新表面均在 plan `<threat_model>` T-07-05a/b/c/d 四条登记内：Close 内建 5s+5s 上界不再盒 + exitf 由 lifecycle 并发收口（T-07-05a mitigate，两测试 + 冒烟锁定）；--open URL wesh 自构 + exec.Command argv 分离 + --socket×--open 拒绝（T-07-05b mitigate，TestOpenBrowser/StartupMatrix + 冒烟锁定）；token URL 桌面暴露（T-07-05c accept，D-26 既定）；1001×EXIT 竞态（T-07-05d accept，Shutdown 注释登记风险接受论证）。无未建模的信任边界扩张。

## Known Stubs

None——无占位实现；六条 must_have truths 全部经 Go 单测 + -race 全量 + 真实二进制冒烟 + jsdom 场景达成（1001 广播与 reason / stop-signal 序列与 KILL 补发 / lifecycle 单一收口 / 前端面板不重连 / --open 三形态 / proto.go 翻正互指）。macOS open 真实弹窗与 xdg-open 非零返回属 flagged_assumptions 既定人工复核面（07-08 清单），非 stub。

## Next Phase Readiness

- **07-06 配置文件直接可用面：** config.open 字段就位（RESEARCH Pattern 4 fileConfig.Open 键预留）；validateStartup --socket×--open 行与 parse 期零新校验面直接复用，零双写
- **07-07 UAT 素材：** 1001 关停序列场景（spawn 实例 SIGTERM → 客户端收 1001 → waitExit 255）冒烟脚本形态本 plan 已验证（Node 原生 WebSocket 协议层断言，/tmp/wesh-uat/wesh 已含全部新行为）；--open 场景 fake xdg-open PATH 前置形态同 Go 侧
- **07-08 人工 UAT 复核联动：** flagged_assumptions OPS-11 条目维持原登记——macOS open 行为未在本机实测（CI macOS 跑 TestOpenBrowser 同款形态即整体 Skip，真实弹窗列人工清单）；xdg-open 返回非零只警告不阻断（D-27）；--open × TLS 打开 https:// 链接（自签证书浏览器警告属用户预期面）；README 优雅下线节需落「1001 序列 + 退出码 255 运维注记（systemd SuccessExitStatus= 部署侧自决，RESEARCH OQ2 去向）」
- **OPS-11 闭合：** --open 三形态全链可用；D-23 1001 优雅下线（P6 deferred 兑现）闭合——wesh 现具备 systemd 部署的完整关停语义（SIGTERM → 1001 广播 → stop-signal 序列 → 退出 255）

## Self-Check: PASSED

- 文件存在性：8/8 FOUND（shutdown_test.go 含 TestShutdown1001/TestShutdownStopTimeout；server.go 含 func (s \*Server) Shutdown() ×1；proto.go 含 '1001 优雅下线已于 Phase 7 启用' ×1；main.go 含 signal.NotifyContext ×1/func openBrowser ×1/'--open conflicts with --socket' ×1；main_test.go 含 TestOpenBrowser；main.ts 含 'Server shutting down'；phase06-dom.mjs 含 d11Shutdown1001NoReconnect；dist/index.html 含 'Server shutting down' ×1）+ 本 SUMMARY FOUND
- 提交存在性：5/5 FOUND（adbc5be / a6128a8 / 6d40125 / 32662a6 / ebc66e9）
- 全量验证：`go test -race -count=1 ./...` 五包全绿（54.3s）；`go test ./internal/server -count=1`（45.8s）与 `go test ./cmd/wesh -count=1` 全绿；`go vet ./...` 零告警；GOROOT gofmt 漂移清单 == 两既有文件（本 plan 零新增）；`time pnpm -C web build` 退出 0（2.4s，产物时间戳已验证）
- UAT 套件：phase06-dom 37/37（含 D11a-d）+ phase04-dom 37/37 + phase05-dom 19/19 + phase02 12/12 + phase06 23/23 各自退出 0
- 冒烟三 success_criteria 逐条观测达成（SIGTERM → 1001+reason+255 / DISPLAY=:99 --open --writable fake xdg-open 收 rw 链接 + headless 跳过提示 / --socket×--open exit 2 双 flag 名）

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
