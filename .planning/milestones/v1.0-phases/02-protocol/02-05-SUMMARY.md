---
phase: 02-protocol
plan: 05
subsystem: api
tags: [websocket, read-limit, dos-defense, 1009, stderr-events, go, coder-websocket]

requires:
  - phase: 02-protocol
    provides: 02-01 proto 读上限常量（ReadLimitPreAuth/PostAuth）、02-02 握手段两处读错误路径与 logEvent 三要素、02-04 pinger 埋点形态
provides:
  - server.go reader 错误路径 ErrMessageTooBig 分类钩子（预认证首读 + 稳态读循环两处，同一 logIfMessageTooBig 调用形态）——D-12② 超限可见性三腿之二落地
  - limits_test.go RES-01 攻击面五测：TestOversize1009（含 stderr 事件断言）/TestReadLimitBoundary（16384 通过 + 16385→1009）/TestFragmentedFlood1009（1 字节分片洪水）/TestEmptyFragmentFloodResilience（空帧洪水存活+内存平坦）/TestPreHelloReadLimit（预认证 4KiB 档）
  - captureStderr/readCloseErr 测试 helper（同包复用）
affects: [02-06 期末 UAT, Phase 8 slog 结构化日志迁移]

actuals:
  tokens: 3959
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "logIfMessageTooBig 分类钩子：errors.Is(ErrMessageTooBig) → logEvent(remote, 1009, message_too_big)，两处读路径同一调用形态；机器串落 stderr 不落线上 reason（库内字符串不可定制）"
    - "边界测试载荷纪律：PTY 规范模式 ECHOCTL 将控制字符回显为 ^X 两字节（实测 NUL 2× 失真），边界计数断言必须用可打印字符（1:1 回显）"
    - "stderr 断言的竞态安全形态：logEvent 与 exitf 同 goroutine 程序序，waitExit 返回即事件已落盘（channel happens-before 后再恢复 os.Stderr）"
    - "SetReadLimit 边界语义实证：库内部 +1 余量供 fin 收尾读（read.go:97-105），边界恰为 limit 通过 / limit+1 拒绝"

key-files:
  created:
    - internal/server/limits_test.go
  modified:
    - internal/server/server.go

key-decisions:
  - "limits_test.go 随 Task 1 RED 先行创建（TDD 纪律：TestOversize1009 即 Task 1 行为的失败测试），Task 2 扩展其余四测——同 02-01 先例的执行顺序说明，非计划变更"
  - "TestReadLimitBoundary 载荷用 'A' 不用 plan 字面 zeros：NUL 经 PTY 规范模式 ECHOCTL 回显为 \"^@\" 两字节（实测 16383→32766），16383 字节回显断言失真；'A' 回显 1:1（实测恰 16383 字节 ~22ms 收齐）"

patterns-established:
  - "五测共用 readCloseErr（CloseError 收口循环）+ 单测独占 captureStderr（os.Pipe 替换，幂等恢复，仅 TestOversize1009 断言 stderr 避免 fd 竞争）"
  - "空帧洪水断言形态：存活（echo 全链路）+ exitf 未触发 + HeapAlloc 增量 <8MiB 宽松防线——D-09 修订残余风险的显式断言模板"

requirements-completed: [RES-01, SEC-08]

coverage:
  - id: D1
    description: "ErrMessageTooBig stderr 钩子：库自动 1009 后服务端 stderr 恰一行 message_too_big 事件（remote/code/reason 三要素），预认证首读与稳态读循环两处埋点"
    requirement: RES-01
    verification:
      - kind: e2e
        ref: "internal/server/limits_test.go#TestOversize1009"
        status: pass
      - kind: other
        ref: "grep 静态核查：两处 logIfMessageTooBig 埋点 + 八处 logEvent 三要素一致"
        status: pass
    human_judgment: false
  - id: D2
    description: "RES-01 两层硬顶边界精确：16384 字节消息正常 echo、16385 → 1009；1 字节分片洪水在累积 16385 字节处 1009；空消息洪水存活+内存平坦"
    requirement: RES-01
    verification:
      - kind: e2e
        ref: "internal/server/limits_test.go#TestReadLimitBoundary"
        status: pass
      - kind: e2e
        ref: "internal/server/limits_test.go#TestFragmentedFlood1009"
        status: pass
      - kind: e2e
        ref: "internal/server/limits_test.go#TestEmptyFragmentFloodResilience"
        status: pass
      - kind: other
        ref: "go test -race -count=3 五测连跑稳定"
        status: pass
    human_judgment: false
  - id: D3
    description: "SEC-08 预认证 4KiB 档攻击面证据：Hello 前 >4KiB 消息被库自动 1009"
    requirement: SEC-08
    verification:
      - kind: e2e
        ref: "internal/server/limits_test.go#TestPreHelloReadLimit"
        status: pass
    human_judgment: false

duration: 9min
completed: 2026-08-15
status: complete
---

# Phase 2 Plan 05: RES-01 上限攻击面——三层硬顶与超限可见性自动化证据 Summary

**server.go 补 ErrMessageTooBig stderr 钩子（预认证首读 + 稳态读循环两处同一形态），limits_test.go 五测锁死 RES-01 等效防线：16384/16385 边界精确、1 字节分片洪水在累积 16385 字节处 1009、5000 空消息洪水存活且内存平坦（HeapAlloc 增量实测 -87KiB）、预认证 4KiB 档 1009——D-12 三腿齐了（前端 1009 文案 + stderr 事件 + 机器串）**

## Performance

- **Duration:** 9 min
- **Started:** 2026-08-15T09:06:02Z
- **Completed:** 2026-08-15T09:15:09Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- **D-12② 超限可见性落地**：`logIfMessageTooBig` 钩子在预认证首读（4KiB 档）与稳态读循环（16KiB 档）两处埋点，`errors.Is(err, websocket.ErrMessageTooBig)` → `logEvent(remote, 1009, message_too_big)`；机器串落 stderr 不落线上 reason（库内 "read limited at N bytes" 不可定制，PATTERNS 注意 7）；埋点后错误仍走 wsDisconnected→terminate 既有单一路径收口，零新 exitf 分支
- **五测与 RESEARCH §Validation Architecture RES-01 五行映射一一对应**：TestOversize1009（1009 + stderr 恰一行事件）/ TestReadLimitBoundary（16384 echo 收齐 + 16385→1009）/ TestFragmentedFlood1009（库客户端 Writer 逐非 fin 帧构造 1 字节分片流，无裸帧 helper）/ TestEmptyFragmentFloodResilience（存活 + echo 正常 + exitf 未触发 + HeapAlloc 增量 <8MiB，实测 -86784 字节）/ TestPreHelloReadLimit（Hello 前 4097 字节 → 1009）
- **边界语义实证钉死**：SetReadLimit 内部 +1 余量供 fin 收尾读（read.go:97-105），边界恰为 limit 通过 / limit+1 拒绝——plan 的 16384/16385 断言与库行为逐字节吻合
- **logEvent 埋点全清单复核归一**：八处调用（subprotocol_required/hello_timeout/empty_frame/frame_before_hello/malformed_hello/version_mismatch/pong_timeout/message_too_big）三要素格式一致，doc 注释同步更新
- 全量 `go build ./... && go vet ./... && go test -race -count=1 ./...` 绿；五测 `-race -count=3` 连跑稳定

## Task Commits

1. **Task 1 RED: TestOversize1009 失败测试（stderr 事件断言）** - `a6116fb` (test)
2. **Task 1 GREEN: ErrMessageTooBig stderr 钩子两处埋点** - `9dd766e` (feat)
3. **Task 2: 其余四测（边界/分片洪水/空帧存活/预认证档）** - `d9c3988` (test)

**Plan metadata:** 见下方 docs 提交（docs(02-05): complete ...）

## Files Created/Modified

- `internal/server/server.go` - 预认证首读与稳态读循环两处挂 `logIfMessageTooBig`；新增 logIfMessageTooBig 方法；logEvent doc 更新埋点全集
- `internal/server/limits_test.go` - 新建：五测 + readCloseErr/captureStderr helper

## Decisions Made

- **limits_test.go 随 Task 1 RED 先行创建**：Task 1 标记 `tdd="true"` 而测试文件归属 Task 2——按 TDD 纪律测试文件随 RED 阶段先行创建（TestOversize1009 即 Task 1 行为的失败测试，含 stderr 断言），Task 2 扩展其余四测。与 02-01 同一形态的执行顺序说明，交付物与验收标准不变。
- **边界测试载荷选型（'A' 而非 zeros）**：plan 字面 `make([]byte, ...)` zeros 载荷会撞 PTY 规范模式 ECHOCTL——NUL 回显为 "^@" 两字节（实验实测：16383 零字节 → 32766 字节回显），"累积收齐 16383 字节"断言语义失真；可打印字符回显 1:1（实测恰 16383 字节、~22ms 收齐）。另实测排除两个替代方案：stty raw 在本机 echo/echoctl 仍置位（`stty -a` 实证，回显 3× 字节），纯 zeros 方案 2× 计数。详见偏差 1。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TestReadLimitBoundary 载荷 zeros → 'A'（PTY ECHOCTL 实测证据）**
- **Found during:** Task 2（边界测试实现前的 PTY 行为实证——对 plan 字面 `make([]byte, proto.ReadLimitPostAuth-1)` 的回显可行性验证）
- **Issue:** plan 指定零字节载荷经 /bin/cat 回显断言 16383 字节收齐；但 PTY 规范模式默认 ECHOCTL 置位，NUL（控制字符）回显为 "^@" 两字节——实测 16383 零字节产生 32766 字节回显，累积计数断言必然失真（且依赖"回显在缓冲满后继续"的未验证内核行为，有挂死风险）
- **Fix:** 载荷改 `bytes.Repeat([]byte("A"), proto.ReadLimitPostAuth-1)`——可打印字符不受 ECHOCTL 影响，实测回显恰 16383 字节、~22ms 收齐、写不阻塞；测试注释内记录实证依据。断言目标（16384 总长正常处理 / 16385 切断）与 plan 完全一致
- **Files modified:** internal/server/limits_test.go
- **Verification:** TestReadLimitBoundary 绿（0.02s），五测 `-race -count=3` 连跑稳定
- **Committed in:** `d9c3988`（Task 2 提交）

---

**Total deviations:** 1 auto-fixed (1 blocking/实证修正)
**Impact on plan:** 仅测试载荷选型修正，断言语义与验收标准保持；plan 的"累积收齐回显"意图原样实现。

## TDD Gate Compliance

- RED gate: `a6116fb` test(02-05) — 测试因 stderr 无 message_too_big 事件失败（正确失败原因：CloseError 1009 已由库行为到达，仅事件缺失）✓
- GREEN gate: `9dd766e` feat(02-05) — RED 之后，测试转绿 ✓
- REFACTOR gate: 省略——实现即为最小形态（两处一行埋点 + 一个分类方法），无需清理

## Issues Encountered

- 实验环境一次自伤：首个 PTY 探针程序误用 `SetReadDeadline`（pty master 阻塞模式 fd 不支持，Read 永久阻塞）导致探针挂起——与产品代码无关，重写探针后完成实证。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 02-06 期末 UAT：本 plan 全部为自动化证据，无新增人工确认项；前端 1009 文案（02-02 已落地）浏览器确认仍属期末清单
- ROADMAP 准则 1 自动化成立：超限帧 1009 合规关闭、空帧洪水下服务存活内存平坦、两层硬顶（4KiB/16KiB）经 SetReadLimit 库执行——STATE.md 中"WS 三层上限默认值需实测标定"的 Phase 2 blocker 可视为闭合（数值经攻击面测试标定：合法流量远小于 16KiB，洪水在 16385 处切断）
- D-12 三腿齐了：前端 1009 文案（02-02）+ stderr 事件（本 plan）+ 机器串（stderr reason 字段）
- 无阻塞项

## Self-Check: PASSED

- [x] `internal/server/server.go`（钩子两处）、`internal/server/limits_test.go`（五测 + 两 helper）、`02-05-SUMMARY.md` 均存在于磁盘
- [x] 提交 `a6116fb` / `9dd766e` / `d9c3988` 均见于 `git log`
- [x] 五测函数名与 plan artifacts 逐一对应（grep 计数 5）
- [x] plan 级 verification 全绿：`go build ./... && go vet ./...` 退出 0；`go test -race -count=1 ./...` 四包全 PASS；五测 `-race -count=3` 连跑无 race、无 flake
- [x] 无意外文件删除（`git diff --diff-filter=D HEAD~3..HEAD` 为空）
- [x] TDD 门序列：RED `a6116fb`（test）→ GREEN `9dd766e`（feat）顺序正确

---
*Phase: 02-protocol*
*Completed: 2026-08-15*
