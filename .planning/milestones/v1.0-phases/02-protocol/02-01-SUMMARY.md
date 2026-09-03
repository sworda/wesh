---
phase: 02-protocol
plan: 01
subsystem: api
tags: [websocket, protocol, json, go, coder-websocket]

requires:
  - phase: 01-pty
    provides: proto.go 帧常量现状（'0'/'1'）、ClampDim、Decode+ok 惯例、D-16 前后端手工对齐纪律
provides:
  - wesh.v1 协议单一事实源：Hello/Welcome/Error 帧类型字节 + Subprotocol 常量 + Error code 表 + 两档读上限常量
  - Hello/Welcome/Error JSON 编解码（DecodeHello/WelcomeFrame/ErrorFrame）与 D-02 未知字段忽略纪律
  - 02-02 握手段全部 proto 符号依赖（DecodeHello/WelcomeFrame/ErrorFrame/ReadLimit*/Subprotocol/ModeRO/ModeRW/ErrXxx）
affects: [02-02 握手, 02-03 攻击面, 02-05 上限 e2e, web/src/main.ts 前端对齐]

actuals:
  tokens: 2614
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "包 doc 三段式：帧格式 / 前端对齐互相指路(D-16) / 关闭码纪律(D-05 全集)"
    - "协议常量逐字锁定测试（TestProtocolConstants）——手滑改码即红的协议破坏防线"
    - "分片数上限注释位模式：库不暴露 API 的能力缺口以注释显式记录等效防线与残余风险（D-09 修订）"

key-files:
  created:
    - internal/proto/proto_test.go
  modified:
    - internal/proto/proto.go

key-decisions:
  - "Task 1 tdd=true 与 Task 2 拥有 proto_test.go 的张力按 TDD 纪律裁决：测试文件随 RED 先行创建，Task 2 扩展（执行顺序说明，非计划变更）"

patterns-established:
  - "Decode+ok + ClampDim 复用惯例扩展到 Hello（DecodeHello 不做 version 校验，校验语义在 server 握手段）"
  - "S→C 控制帧组帧函数 WelcomeFrame/ErrorFrame 与 onChunk 1+payload 模式同构"

requirements-completed: [CORE-04, SEC-08, RES-01]

coverage:
  - id: D1
    description: "wesh.v1 全量契约入 proto 包：帧类型/Subprotocol/Error code/Mode/读上限常量 + 关闭码注释表（1001/1013 占位标注启用 phase）"
    requirement: CORE-04
    verification:
      - kind: unit
        ref: "internal/proto/proto_test.go#TestProtocolConstants"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Hello/Welcome/Error 编解码往返正确 + D-02 未知字段忽略自动化回归 + ClampDim 边界钳制"
    requirement: SEC-08
    verification:
      - kind: unit
        ref: "internal/proto/proto_test.go#TestDecodeHello"
        status: pass
      - kind: unit
        ref: "internal/proto/proto_test.go#TestWelcomeFrameErrorFrame"
        status: pass
      - kind: unit
        ref: "go test -race -count=1 ./internal/proto/"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-08-15
status: complete
---

# Phase 2 Plan 01: 协议基线——wesh.v1 契约单一事实源 Summary

**wesh.v1 协议契约一次定稿入 `internal/proto`：'H'/'W'/'E' 帧字节 + Subprotocol/Error code/Mode/两档读上限常量 + Hello/Welcome/Error JSON 编解码，D-02 未知字段忽略纪律获自动化回归锁定**

## Performance

- **Duration:** 5 min
- **Started:** 2026-08-15T05:39:27Z
- **Completed:** 2026-08-15T05:44:07Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- proto 包成为 wesh.v1 协议完整单一事实源：帧常量块新增 Hello='H'/Welcome='W'/Error='E' 并占住 'X'/'T'/'P'（D-01）；Subprotocol="wesh.v1" 三处同源（D-03）；ModeRO/ModeRW 入 proto（D-14）
- Error code 表落地 D-06/D-07 受众分治纪律：仅 version_mismatch(+1008)/server_error(+1011) 两个正常客户端可见码；攻击面路径不发 Error 帧的纪律随常量注释传递
- 两档读上限 ReadLimitPreAuth=4KiB/ReadLimitPostAuth=16KiB 进常量并注释标定依据（D-09 修订/D-11）；分片数上限 32 仅注释位，显式记录等效防线与残余风险（D-09 2026-08-15 修订）
- DecodeHello 以 json.Unmarshal 默认行为实现 D-02 未知字段忽略（零成本、禁止 DisallowUnknownFields）；TestDecodeHello 未知字段混合组成为演化纪律的自动化回归
- TestProtocolConstants 逐字锁定六个帧字节/Subprotocol/mode 字符串/读上限数值——手滑改码即协议破坏面被测试封死（T-02-01 缓解）

## Task Commits

1. **Task 1 RED: Hello/Welcome/Error 编解码失败测试** - `f95c2a0` (test)
2. **Task 1 GREEN: wesh.v1 协议全量契约实现** - `bd80876` (feat)
3. **Task 2: TestProtocolConstants 常量形状锁定** - `8e6c7a5` (test)

**Plan metadata:** 见下方 docs 提交（docs(02-01): complete ...）

## Files Created/Modified

- `internal/proto/proto.go` - 扩展为 wesh.v1 全量契约：包 doc 三段式改写（删除 Phase 2 预留注释）、帧常量/Subprotocol/Mode/Error code/读上限常量、三个 payload 类型、DecodeHello/WelcomeFrame/ErrorFrame
- `internal/proto/proto_test.go` - 新建：TestDecodeHello（5 组表驱动）/TestWelcomeFrameErrorFrame/TestProtocolConstants

## Decisions Made

- 执行顺序说明：Task 1 标记 `tdd="true"` 而 proto_test.go 归属 Task 2——按 TDD 纪律测试文件随 RED 阶段先行创建（含 TestDecodeHello/TestWelcomeFrameErrorFrame），Task 2 在其上扩展 TestProtocolConstants。交付物与验收标准与计划一致，仅文件创建时点前移。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] TestProtocolConstants 增补 ModeRO/ModeRW 字符串锁定**
- **Found during:** Task 2（常量形状测试编写）
- **Issue:** 计划验收仅列 Subprotocol/帧字节/读上限；ModeRO="ro"/ModeRW="rw" 同为前后端公开对齐字符串（D-14），漏锁则手滑改值无测试兜底
- **Fix:** TestProtocolConstants 增补 ModeRO/ModeRW 逐字断言（与 Subprotocol 锁定同性质）
- **Files modified:** internal/proto/proto_test.go
- **Verification:** `go test -race -count=1 ./internal/proto/` 全绿
- **Committed in:** 8e6c7a5（Task 2 提交）

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** 常量锁定覆盖补齐 D-14 对齐面，无范围蔓延。

## TDD Gate Compliance

- RED gate: `f95c2a0` test(02-01) — 测试因 undefined symbols 失败（正确失败原因）✓
- GREEN gate: `bd80876` feat(02-01) — RED 之后，测试转绿 ✓
- REFACTOR gate: 省略——实现即为最小形态，无需清理

## Issues Encountered

None——`go build ./...`、`go vet ./...`、`go test -race -count=1 ./...` 一次全绿，既有测试（cmd/wesh、pty、server）不受影响。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 02-02 握手段所需的全部 proto 符号（DecodeHello/WelcomeFrame/ErrorFrame/ReadLimitPreAuth/ReadLimitPostAuth/Subprotocol/ModeRO/ModeRW/ErrVersionMismatch/ErrServerError）就绪
- 前端 web/src/main.ts 常量对齐在后续 plan 落地（D-16 手工对齐，两侧注释互相指路）
- 无阻塞项

## Self-Check: PASSED

- [x] `internal/proto/proto.go` 存在（6274 字节，含全部约定符号）
- [x] `internal/proto/proto_test.go` 存在（4182 字节，三测试函数）
- [x] 提交 `f95c2a0` / `bd80876` / `8e6c7a5` 均见于 `git log`
- [x] plan 级 verification 全绿：`go build ./... && go vet ./...` 退出 0；`go test -race -count=1 ./...` 全包 PASS
- [x] 无意外文件删除（`git diff --diff-filter=D HEAD~3..HEAD` 为空）

---
*Phase: 02-protocol*
*Completed: 2026-08-15*
