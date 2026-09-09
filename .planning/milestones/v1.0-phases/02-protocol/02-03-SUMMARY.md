---
phase: 02-protocol
plan: 03
subsystem: api
tags: [websocket, security, dos-protection, slowloris, per-ip-limit, readonly-mode, go, coder-websocket]

requires:
  - phase: 02-protocol
    provides: 02-02 握手骨架（守卫区 400/409 双闸、4KiB/5s 预认证窗口、握手段状态机、Options 注入结构、dialHello/startTestServerWith 测试基建）
provides:
  - per-IP 半开连接计数器 halfOpenCounter（acquire/release，到 0 删 key）+ clientIP + Options.MaxHalfOpenPerIP（零值默认 8，D-04）
  - 守卫区三闸完整顺序：子协议 400 → per-IP 429 → 409 → Accept → assert，release 恰好一次不变量（sync.Once+defer 兜底）
  - http.Server ReadHeaderTimeout=5s（预认证 HTTP 层慢 loris 防御，不伤 WS 长连接）
  - handshake_test.go 七测：守卫链与握手违规全攻击面 + ro 服务端边界双测自动化锁定
affects: [02-04 保活（Options 同构追加 PingInterval/PongTimeout）, 02-05 上限 e2e, 02-06 期末 UAT, Phase 3 SEC-07（X-Forwarded-For 信任）]

actuals:
  tokens: 7180
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "per-IP 计数恰好一次不变量的实现形态：局部 sync.Once + defer 兜底——显式释放挂 409/Accept/assert 失败与握手升档点，defer 覆盖违规落循环与连接终结的一切 return 路径"
    - "ro 静默窗口测试形态：goroutine Read(context.Background()) + 缓冲 channel + select time.After 竞速——客户端 Read 永不携带 deadline ctx（Pitfall 2 回归锁）"
    - "预认证 HTTP 层慢 loris 防御：http.Server{ReadHeaderTimeout} 显式装配，ReadTimeout/WriteTimeout 不设（误伤 WS 长连接语义）"

key-files:
  created:
    - internal/server/handshake_test.go
  modified:
    - internal/server/server.go
    - cmd/wesh/main.go

key-decisions:
  - "release 恰好一次的实现选型（plan discretion）：局部 sync.Once + defer release() 兜底——显式调用只标『早于此处释放』的关键点（409/Accept/assert/升档），一切遗漏路径由 defer 恰好一次收口"
  - "Task 1 tdd=true 与 handshake_test.go 归属 Task 2 的张力按 TDD 纪律裁决（02-01 先例）：测试文件随 RED 先行创建（含 TestHalfOpenPerIP429），Task 2 扩展其余六测"

patterns-established:
  - "守卫拒绝路径的计数器纪律：任何 acquire 后的拒绝/失败 return 前先 release()——被 409 拒的连接不残留半开计数"
  - "时序敏感 e2e 的容错窗形态：注入短超时（200ms）+ 断言窗（2s）10 倍余量；-count=3 连跑验证无 flake"

requirements-completed: [SEC-08, CORE-04]

coverage:
  - id: D1
    description: "per-IP 半开上限闸：MaxHalfOpenPerIP=1 注入下 c1 半开占帽 → c2 收 HTTP 429；c1 补发 Hello 握手成功不误伤（acquire/release 恰好一次间接证明）；Options 零值默认 8"
    requirement: SEC-08
    verification:
      - kind: e2e
        ref: "internal/server/handshake_test.go#TestHalfOpenPerIP429"
        status: pass
      - kind: e2e
        ref: "go test -race -count=3 ./internal/server/（三轮无 race 无 flake）"
        status: pass
    human_judgment: false
  - id: D2
    description: "守卫链与握手违规路径自动化锁定：无/错子协议 400、多值头放行（Pitfall 5）、hello_timeout 1008 机器串、抢跑 1002 零 'E' 帧（D-06 零反馈）、version_mismatch 先 Error 后 1008 同名机器串（D-07）"
    requirement: SEC-08
    verification:
      - kind: e2e
        ref: "internal/server/handshake_test.go#TestSubprotocolRequired"
        status: pass
      - kind: e2e
        ref: "internal/server/handshake_test.go#TestHelloTimeout"
        status: pass
      - kind: e2e
        ref: "internal/server/handshake_test.go#TestPrematureFrame"
        status: pass
      - kind: e2e
        ref: "internal/server/handshake_test.go#TestVersionMismatch"
        status: pass
    human_judgment: false
  - id: D3
    description: "http.Server ReadHeaderTimeout=5s 落地（预认证 HTTP 层慢 loris 盒住），ReadTimeout/WriteTimeout 不设"
    requirement: SEC-08
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... + 静态走查（cmd/wesh/main.go http.Server{ReadHeaderTimeout: 5 * time.Second} 装配，无 ReadTimeout/WriteTimeout）——plan 验收准则即为静态形态"
        status: pass
    human_judgment: false
  - id: D4
    description: "ro 服务端边界双测：裸 WS 客户端 INPUT 被丢弃（200ms 静默窗口，禁 deadline ctx 的 goroutine 竞速实现）且连接存活；RESIZE 放行尺寸跟随（Hello 111x44 → \"44 111\"，RESIZE 120x50 → \"50 120\"）"
    requirement: CORE-04
    verification:
      - kind: e2e
        ref: "internal/server/handshake_test.go#TestReadOnlyDropsInput"
        status: pass
      - kind: e2e
        ref: "internal/server/handshake_test.go#TestReadOnlyAllowsResize"
        status: pass
    human_judgment: false

duration: 16min
completed: 2026-08-15
status: complete
---

# Phase 2 Plan 03: SEC-08 守卫链收口——per-IP 429 闸 + 握手违规七测 Summary

**SEC-08 预认证守卫链最后两道闸落地：per-IP 半开连接上限（默认 8，HTTP 429）以『子协议 400 → per-IP 429 → 409』裁决顺序插入守卫区，release 恰好一次不变量由 sync.Once+defer 钉死；http.Server ReadHeaderTimeout=5s 盒住 HTTP 层慢 loris；handshake_test.go 七测把 D-03/D-04/D-06/D-13 全部违规路径与 ro 服务端边界锁成自动化回归（-race 三连跑无 flake）**

## Performance

- **Duration:** 16 min
- **Started:** 2026-08-15T07:45:49Z
- **Completed:** 2026-08-15T08:01:58Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- per-IP 半开计数器 `halfOpenCounter`（map+Mutex）：acquire 上限拦截 / release 到 0 删 key 防 map 单调增长（Pitfall 4 泄漏面）；`clientIP` 以 SplitHostPort 取主机部分作键（带端口当键上限形同虚设，Pitfall 6），反代聚合限制注释明示、X-Forwarded-For 信任留 Phase 3 SEC-07
- 守卫区三闸顺序按 planner 裁决落地并注释钉死理由：per-IP 429 必须在 409 之前——409 在前则 429 在单客户端模型下结构性不可达（D-04 可触达性）；release 恰好一次不变量以局部 sync.Once + defer 兜底实现，显式释放挂 409/Accept/assert 失败与握手升档点（Hello 校验通过后、Welcome 发出前）
- `cmd/wesh/main.go` 显式 `http.Server{ReadHeaderTimeout: 5 * time.Second}` 替换 `http.Serve`——预认证 HTTP 层慢 loris 与 helloTimeout 同 5s 量级盒住；ReadTimeout/WriteTimeout 不设（误伤 WS 长连接语义）
- handshake_test.go 七测全绿：子协议三组（无 400/错 400/多值头放行+c.Subprotocol() 断言）、per-IP 429（占帽→429→在先连接不误伤→echo 全链路）、hello_timeout（200ms 注入，1008+机器串，2s 容忍窗）、抢跑帧（1002 且全程零数据帧，D-06 零反馈最强形态断言）、version_mismatch（先 'E' 帧后 1008，两处机器串同名）、ro 丢 INPUT（goroutine Read(Background)+channel+select 竞速静默窗，禁 deadline ctx 的 Pitfall 2 回归锁；RESIZE 写通+正常关闭握手证存活）、ro 放行 RESIZE（夹具前导 sleep 0.5 防 drain 吞输出，Hello 携尺寸与 RESIZE 跟随双断言）
- 既有七测（e2e 六测 + TestHelloWelcome）在新守卫链下保持全绿——429 闸对正常握手零干扰获得既有回归间接证明

## Task Commits

1. **Task 1 RED: TestHalfOpenPerIP429 失败测试** - `f327448` (test) — unknown field MaxHalfOpenPerIP（正确失败原因）
2. **Task 1 GREEN: per-IP 半开计数器 + 429 闸 + ReadHeaderTimeout** - `c06cae6` (feat)
3. **Task 2: 守卫链与握手违规六测** - `658874c` (test)

**Plan metadata:** 见下方 docs 提交（docs(02-03): complete ...）

## TDD Gate Compliance

- RED gate: `f327448` test(02-03) — 测试因 unknown field MaxHalfOpenPerIP 编译失败（正确失败原因）✓
- GREEN gate: `c06cae6` feat(02-03) — RED 之后，新测转绿且既有全量保持绿 ✓
- REFACTOR gate: 省略——实现即为最小形态，无需清理

## Files Created/Modified

- `internal/server/server.go` - halfOpenCounter 类型（acquire/release）、clientIP、Options.MaxHalfOpenPerIP + defaultMaxHalfOpenPerIP(8)、Server 结构体两新字段、守卫区 429 闸与 release 不变量、升档序列 release 挂点
- `cmd/wesh/main.go` - http.Server{ReadHeaderTimeout: 5s} 装配（imports +time）
- `internal/server/handshake_test.go` - 新建：七测（TestSubprotocolRequired/TestHalfOpenPerIP429/TestHelloTimeout/TestPrematureFrame/TestVersionMismatch/TestReadOnlyDropsInput/TestReadOnlyAllowsResize）

## Decisions Made

- **release 恰好一次的实现选型（plan 给 discretion：sync.Once 或 released 布尔）：** 采用局部 `sync.Once` + `defer release()` 兜底——显式 release 只标四个关键时点（409 拒绝/Accept 失败/assert 失败/握手升档），违规落读循环、首读失败、正常会话终结等一切其余 return 路径由 defer 恰好一次收口。比 released 布尔更能防未来新增路径时漏挂释放点。
- **Task 1 tdd=true 与测试文件归属的张力（同 02-01 先例）：** handshake_test.go 名义归属 Task 2，但 Task 1 的 TDD RED 需要失败测试先行——测试文件随 RED 创建（仅含 TestHalfOpenPerIP429），Task 2 在其上扩展其余六测。交付物与验收标准与计划一致，仅文件创建时点前移。

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- PATH 中 `/usr/bin/gofmt` 为陈旧版本（不支持泛型语法，`atomic.Pointer[websocket.Conn]` 即报语法错误）——按 Phase 01-03 累积决策改用 GOROOT gofmt（`/data1/home/zexueli/softwares/go/bin/gofmt`）完成格式化校验，仅命中一处字段对齐修正；`internal/proto/proto.go` 被新版 gofmt 标出属 02-01 既存格式偏好差异，按 scope boundary 不顺手改。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 02-04 保活：Options 追加 PingInterval/PongTimeout 的同构注释模式已就绪（Options 注释预留位已更新为 02-04）；pinger 挂点在升档序列末尾（release/Welcome/SetReadLimit 之后）
- 02-05 上限 e2e：logEvent 六处埋点未动，ErrMessageTooBig 钩子挂点（两个读循环错误分支）保持可挂；守卫区顺序注释为复核清单提供锚点
- 预认证零缓冲 backstop 说明：本 plan 守卫区与 4KiB 窗口无任何消息级缓冲预分配（拒绝路径全是 http.Error 静态响应 + map 计数；握手段单条 Read 经库 limitReader 流式截断）——代码走查 + 七测 flood/违规路径行为证据支持 SEC-08 结构目标；内存采样级断言按 plan 标记为 backstop 未做
- 无阻塞项

## Self-Check: PASSED

- [x] `internal/server/handshake_test.go`（424 行，七测试函数）/ `internal/server/server.go` / `cmd/wesh/main.go` 均存在于磁盘
- [x] 提交 `f327448` / `c06cae6` / `658874c` 均见于 `git log`
- [x] plan 级 verification 全绿：`go build ./... && go vet ./...` 退出 0；`go test -race -count=1 ./...` 四包全 PASS（server 十四测：既有七 + 新七）；`go test -race -count=3 ./internal/server/` 三连跑无 race 无 flake
- [x] 守卫区顺序走查：子协议 400（L186）→ per-IP 429（L196）→ 409（L204）→ Accept（L214）→ assert（L226）→ 4KiB（L239）→ 5s 计时器（L244）；release 挂点五处（L202 defer 兜底 / L205 / L218 / L229 / L292 升档）
- [x] 无意外文件删除（三次任务提交的 `git diff --diff-filter=D` 均为空）

---
*Phase: 02-protocol*
*Completed: 2026-08-15*
