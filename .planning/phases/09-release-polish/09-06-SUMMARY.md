---
phase: 09-release-polish
plan: 06
subsystem: testing
tags: [load-test, build-tag, backpressure, credit-gate, metrics, defunct, calibration, go]

# Dependency graph
requires:
  - phase: 05-multi-client
    provides: outbox/信用门/kickOrCreditLocked/afterDrain（被验证对象）；stall 夹具纪律（slowclient_test.go）
  - phase: 08-observability
    provides: /metrics 17 series 黑盒 scrape 数据源（八条负载 series 预埋挂点）
provides:
  - internal/server/load_test.go（//go:build load 黑盒负载矩阵：TestLoadFanoutMatrix/TestLoadLegitSlowReaderZeroKick/TestLoadMemoryBound/TestLoadGateTransitions/TestLoadDefunct）
  - 标定实测数据源（LOADDATA 每格数据行 + 三断言验证结论）——09-09 D-13 README 标定表回填的直接输入
affects: [09-09（README 标定表回填），release.sh（负载矩阵为发布脚本步骤 5）]

# Actuals (#2632) — chars/4 over the realized diff, same scale as estimateTokens.
actuals:
  tokens: 6400    # 25529 chars / 4，load_test.go 单文件
  tasks: 2
  commits: 3      # 2 task commits + 1 docs commit

tech-stack:
  added: []       # 零新依赖（coder/websocket 为 go.mod 既有，D-11）
  patterns:
    - "build tag 隔离重型测试（//go:build load 首行硬纪律 + go list 双清单机械证据）"
    - "触发式洪水（bash read 闸 + INPUT 触发）消除 pre-attach drain 不确定量，使字节级断言精确"
    - "尾闸 sleep（子进程退出前停 1s）消除 EXIT+1000 广播对 outbox 残余的截断竞态"
    - "限速读者令牌节流 drainLoop（Read 无 deadline + 按消息字节折算停顿）"
    - "defunct 三面口径（NumGoroutine + /proc/self/fd + /proc/<pid>/stat Z 态，轮内显式收口）"

key-files:
  created:
    - internal/server/load_test.go
  modified: []    # 零生产代码改动（验收硬条件，git diff --exit-code 通过）

key-decisions:
  - "尾闸 sleep 1 纳入洪水/滴漏/突发全部生成器 argv：子进程退出即触发 EXIT+1000 广播（绕过 outbox 直写 wire，server.go:1114 先例），无尾闸时 32 端格首跑实测 client 4 缺 22428/34888899 字节——停 1s 腾空 outbox 使严格字节相等断言结构性成立（非放宽断言）"
  - "FanoutMatrix 取 WritePolicy=all 形态：全员 rw 武装离群误踢路径（rw+出宽限+存在未 blocked 可写端→踢），kicks==0 在该分工表形态下方具判别力"
  - "defunct 测试轮内显式 killServer 收口（非 t.Cleanup）：fd/goroutine 回基线口径要求轮间释放，t.Cleanup 会把 200 个 listener 积压到测试尾污染测量"
  - "D-12 三断言全部现值成立（零证伪）——常量默认值零改动，README 回填走 09-09 验证结论形态"

patterns-established:
  - "负载测试标定通道：LOADDATA 行（clients/profile/slowlink/kicks/gate_transitions/outbox_max/alloc_peak/alloc_base/dur_ms）为 README 标定表回填的统一数据格式"
  - "go list 排除证据取 TestGoFiles+XTestGoFiles 双清单（外部包测试文件在 XTestGoFiles——单查 TestGoFiles 是弱证据）"

requirements-completed: [OPS-10]

coverage:
  - id: D1
    description: "负载矩阵四族 + defunct 三面（//go:build load 隔离，手动 -tags=load 运行，常规 CI 零捡起）"
    requirement: OPS-10
    verification:
      - kind: unit
        ref: "go test -tags=load -count=1 -timeout=30m ./internal/server/（106 测试全 PASS，含五族负载格）"
        status: pass
      - kind: unit
        ref: "go test -race -count=1 ./...（全仓五包全绿，常规面零回归）"
        status: pass
      - kind: unit
        ref: "go vet -tags=load ./internal/server/ + go list 双清单排除证据"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-08-29
status: complete
---

# Phase 09 Plan 06: 负载矩阵与标定验证（D-11/D-12） Summary

**`//go:build load` 黑盒负载矩阵落地：三断言（合法慢端零误踢/内存上界/信用门不震颤）全量实测成立 + defunct 三面（goroutine/fd/Z 态）零泄漏——P2/P5/P6 挂账默认参数获负载实证，零证伪零生产代码改动**

## Performance

- **Duration:** 20 min
- **Started:** 2026-08-29T14:56:03Z
- **Completed:** 2026-08-29T15:16:22Z
- **Tasks:** 2
- **Files modified:** 1（新建 internal/server/load_test.go，546+110 行两提交）

## Accomplishments

- 负载矩阵五族全绿：fanout {1,4,16,32} 端 × 触发式 seq 洪水 34.9MB/端（32 端格总 fan-out ≈1.1GB，4.1s 完成），全端收流逐字节一致 + 末位字段到洪水末尾；kicks 精确 ==0；放大比 ws_sent ≥ N×pty_output 成立
- D-12 断言一（合法慢端零误踢）：滴漏 205KB/s + 128KiB 突发格，限速读者 400KB/s 与快读者收流逐字节相等（3,362,720 字节两端一致），kicks==0
- D-12 断言二（内存上界）：32 端洪水 Alloc 峰值 19.8MB ≤ 64MiB（4× 账面最坏），GC 后 3.14MB 回基线 ±50% 内（基线 3.70MB）
- D-12 断言三（信用门频率）：突发 2.2MB/s × 限速 600KB/s 格，outbox 峰值 523,449B（≈99.8% 容量——承压实证到位），门 6 次开闭 / 16.7s = 0.36/s 不震颤（50% 半水位迟滞生效），kicks==0（单体 rw 信用保护形态）
- defunct 三面（Linux-only）：200 轮高频建销（121ms）后 goroutine 89→89、fd 68→68 精确回基线，/proc/<pid>/stat 零 Z 态

## 标定数据表（全量 run LOADDATA 转录——09-09 D-13 README 回填数据源）

| cell | clients | profile | slowlink | kicks | gate_transitions | outbox_max | alloc_peak | alloc_base | dur |
|------|---------|---------|----------|-------|------------------|------------|------------|------------|-----|
| fanout | 1 | seq_flood last=4000000（34.9MB） | none | 0 | 0 | 465B | 3.51MB | 2.33MB | 3444ms |
| fanout | 4 | 同上 | none | 0 | 0 | 10,236B | 3.93MB | 2.46MB | 2786ms |
| fanout | 16 | 同上 | none | 0 | 0 | 114,018B | 7.10MB | 2.96MB | 3256ms |
| fanout | 32 | 同上 | none | 0 | 0 | 99,895B | 16.21MB | 3.61MB | 4114ms |
| legit_slow | 2 | drip 205KB/s + burst 128KiB×10 | rate-limited 400KB/s | 0 | 0 | 0B | 5.24MB | 3.37MB | 13049ms |
| memory_bound | 32 | seq_flood last=4000000 | none | 0 | 0 | 133,498B | 19.76MB | 3.70MB（post-GC 3.14MB） | 4250ms |
| gate_transitions | 1 | burst 64KiB/30ms（2.2MB/s）×150 | rate-limited 600KB/s | 0 | 6（0.36/s） | 523,449B | 7.32MB | 3.66MB | 16724ms |
| defunct | — | 200 轮 spawn+即退（true） | — | — | — | — | goroutine 89→89 / fd 68→68 / Z 态 0 | — | 121ms |

**验证结论（D-12 验证为主）：三断言全部现值成立，零证伪**——outbox 512KiB / 水位 50% / max-clients 32 / attachGrace 500ms 等挂账默认值在负载矩阵下行为符合一阶依据，无常量改动建议（09-09 README 回填走「验证结论」形态）。

**数据判读备注：**
- 活跃读格 outbox 峰值极低（≤133KB ≪ 512KiB）——fan-out 写路径在 32 端规模下远未触及容量，512KiB 默认值对活跃读场景裕度 ≈4× 以上
- legit_slow 格 outbox_max=0 是 loopback 物理：~10MiB 内核吸收带（wmem 4MiB+rmem 6MiB）使应用层限速读者的突发全被内核吸收，outbox 不承压——该格验证的是「drain≈产出即不踢」判别路径武装化下的零误踢，outbox 承压面由 gate_transitions 格覆盖
- gate_transitions 格 outbox 峰值 523,449B ≈ cap 524,288B 的 99.8%——trySend 在容量边界精确失败转信用，无溢出无踢出

## Task Commits

Each task was committed atomically:

1. **Task 1: 负载矩阵骨架与三断言** - `cf069af` (test)
2. **Task 2: defunct 三面 + 全矩阵实跑 + 标定数据落表** - `d2871b3` (test)

**Plan metadata:** 见尾部 docs 提交（本 SUMMARY + STATE/ROADMAP）

## Files Created/Modified

- `internal/server/load_test.go`（新建，656 行）——`//go:build load` 首行硬纪律 + package server_test 外部包；五族测试 + 夹具层（drainClient/drainRateLimited/scrapePeakSampler/allocPeakSampler/gatedFloodArgv/loadSamplers/countFds/readProcState）；零生产代码改动（`git diff --exit-code internal/server/clients.go internal/server/server.go go.mod` 通过）

## Decisions Made

见 frontmatter key-decisions（尾闸 sleep / WritePolicy=all 武装 / 轮内收口 / 零证伪结论四条）。沿用 09-02 先例：plan type=execute 不适用 plan 级 RED/GREEN 门序列；task 级 tdd 对纯测试交付物（验收硬条件 = 零生产代码改动）无法映射 test→feat 拆分——两提交均按 plan 指定的 test(...) 形态。

## TDD Gate Compliance

本 plan 两任务 tdd="true" 但交付物为纯测试文件（验证既有 Phase 5/8 行为，D-12「验证为主」）。按 09-02 已登记先例处理：测试首跑即 PASS 属**设计内验证门性质**（被验证行为由 TestSlowConsumerKick/TestGlobalCredit 等既有 -race 测试先行锁定），按 TDD fail-fast 规则调查后继续——调查结论：kickOrCreditLocked/afterDrain/信用门机制自 05-07/05-13 已全量落地，本矩阵是标定验证通道而非新行为驱动开发。RED/GREEN/REFACTOR 门序列不适用于验证型负载测试；两 task 提交均为 plan 逐字指定的 `test(09-06):` 形态。TDD fail-fast 调查过程的副产品：首跑确实抓到一处**测试自身**的竞态缺陷（见 Deviations Rule 1 #2），修复后全绿。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] loadSamplers.stop() 通道索引笔误**
- **Found during:** Task 1（go vet -tags=load 首次静态检查）
- **Issue:** `outboxMax = <-s.scrapeCh["wesh_outbox_depth_bytes_max"]` 对 <-chan 直接索引，编译失败
- **Fix:** 先 `peaks := <-s.scrapeCh` 收通道再索引 map
- **Files modified:** internal/server/load_test.go
- **Verification:** go vet -tags=load 通过
- **Committed in:** `cf069af`（Task 1 提交内）

**2. [Rule 1 - Bug] 32 端 fanout 格严格字节相等断言被关闭广播截断竞态打破**
- **Found during:** Task 1（首跑 clients_32 子测）
- **Issue:** 子进程退出即触发 lifecycle EXIT+1000 广播（绕过 outbox 直写 wire，server.go:1114 既定设计）；32 端 CPU 竞争下 client 4 的 outbox 残余 22,428/34,888,899 字节（0.06%）随关闭丢弃——非产品 bug（广播截断是 D-10 序列既定语义），是测试断言形态未消化该语义
- **Fix:** 尾闸 sleep 1 纳入全部三个输出生成器 argv（gatedFloodArgv / 滴漏 / 突发）——子进程退出前停 1s 让 writer 腾空 outbox 后广播，严格字节相等断言结构性成立（选择消除竞态而非放宽断言——标定测试的判别力即严格性）
- **Files modified:** internal/server/load_test.go
- **Verification:** 复跑四族全绿；全量 run（106 测试）32 端格逐字节一致
- **Committed in:** `cf069af`（Task 1 提交内）

**3. [Rule 1 - Bug] t.Cleanup(slow.CloseNow) 签名不匹配**
- **Found during:** Task 1（go vet 第二轮）
- **Issue:** CloseNow 返回 error，t.Cleanup 要求 func()
- **Fix:** 包匿名函数 `t.Cleanup(func() { slow.CloseNow() })`
- **Files modified:** internal/server/load_test.go
- **Verification:** go vet -tags=load 通过
- **Committed in:** `cf069af`（Task 1 提交内）

---

**Total deviations:** 3 auto-fixed（1 blocking 编译错误 + 2 test-side bug；全部在测试文件内，零生产代码触及）
**Impact on plan:** 全部修复为测试正确性必需；无 scope creep。plan 字面验收 `go list -f '{{.TestGoFiles}}'` 对 server_test 外部包是弱证据（外部包测试文件在 XTestGoFiles），实际执行取双清单（TestGoFiles+XTestGoFiles）的强证据形态，断言面零损失。

## Issues Encountered

- plan 验收命令 `go list -f '{{.TestGoFiles}}' ./internal/server/` 不列外部包测试文件（server_test 包全部测试文件均不在该清单）——按意图升级为双清单检查，常规（无 tag）清单不含 load_test.go、-tags=load 清单含的机械证据完整保留

## User Setup Required

None - no external service configuration required.（负载测试手动运行通道：`go test -tags=load -count=1 -timeout=30m ./internal/server/ -v`）

## Next Phase Readiness

- 09-09（README 标定表回填 D-13）数据源就绪：本 SUMMARY 的标定数据表 + 「三断言现值成立、零证伪」结论可直接转录；表头去挂账语、初值列改验证结论
- scripts/release.sh（09-08 候选）步骤 5 负载矩阵命令已经本 plan 实证（全量 104s ≪ 30m 预算）
- 无阻塞项；defunct 三面为零泄漏证据锁定（goroutine/fd/Z 态全回基线）

## Self-Check: PASSED

- 文件存在：`internal/server/load_test.go` FOUND（656 行，首行 //go:build load）
- 提交存在：`cf069af` / `d2871b3` 均见于 git log
- 全量验证：`go test -tags=load -count=1 -timeout=30m ./internal/server/` 106 PASS；`go test -race -count=1 ./...` 五包全绿；`go vet -tags=load ./internal/server/` 通过；`git diff --exit-code internal/server/clients.go internal/server/server.go go.mod` 退出 0

---
*Phase: 09-release-polish*
*Completed: 2026-08-29*
