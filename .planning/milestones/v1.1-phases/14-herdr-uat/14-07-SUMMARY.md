---
phase: 14-herdr-uat
plan: 07
subsystem: testing
tags: [go-test, load, per-client, calibration, resource-accounting, sc4]

requires:
  - phase: 13-resource-defense
    provides: churn 格双采样差值断言母本（TestChurnPerClientSpawnThrottle）+ spawn 节流桶测试覆写通道（13-02 Rule 3 先例）+ per-client 装配族
  - phase: 14-herdr-uat
    provides: 14-01 defaultPCSpawnFn 生产闭包镜像 + startPerClientServerTrackedWithSpawn 装配族扩展
provides:
  - TestLoadPerClientFloodMatrix 洪水格（N∈{1,4,16,32} 每会话独立 seq 洪水 + spawn_total 程序序对照 + LOADDATA cell=pc_flood 行）
  - TestLoadPerClientResident 驻留格（N 会话 idle 双采样差值三面断言 + /proc VmRSS 子进程观测 + LOADDATA cell=pc_resident 行）
  - loadDrain.note() 跨帧滚动流尾窗（Rule 1 修正——帧级重置在 tty ONLCR 行尾拆分下误判流截断）
  - readProcVmRSS /proc/<pid>/status 观测夹具（readProcState 姊妹面）
  - D-11 两段式判定结论：32 会话资源实测在账面内——证真，maxClients=32 默认值不动（零契约变更）
affects: [14-11, 14-12]

actuals:
  tokens: 4863   # 19453 chars / 4 over ab95f1c~1..0062c94（internal/server/load_test.go，+352/-4）
  tasks: 2
  commits: 2

tech-stack:
  added: []   # 零新依赖（T-14-SC 红线——纯 Go stdlib + 既有夹具）
  patterns:
    - "每客户端各发一次触发 INPUT（per-client 每会话独立 stdin——shared 单端触发先例必须调整的差异点）"
    - "spawn 节流桶放宽隔离两测试面（13-02 TestPerClientTeardownRaceOnce 先例同形态——节流行为由 churn 格生产默认桶专测）"
    - "连续 3 拍持续达标驻留稳定门（禁固定 sleep 精确时点——瞬时达标不构成驻留证据）"
    - "跨帧滚动流尾采样窗（恒 ≤128B 有界——wire 帧拆分免疫 + 流截断判别力保持）"

key-files:
  created: []
  modified:
    - internal/server/load_test.go

key-decisions:
  - "D-11 判定：证真——32 会话实测（wesh 侧 mem_delta 2.2MB ≈ 24MiB 账面的 9% / 子进程 VmRSS 合计 ~120MB < ~160MB 账面）在可接受界线内，maxClients=32 默认值不动，数据供 14-11 README 标定表回填；one-way 确认门不触发"
  - "fd 账面推导逐项证实：实测差值精确 4N = master(1)+pidfd(1)+accepted(1) 服务端账面 + dial(1) in-process harness 成本——harness 侧 dial socket 单独注释归类非账面放宽"
  - "gor 实测精确 5N+1（harness PingInterval 零值使 pinger 退场）——断言仍按生产账面 6N 收口（pinger 武装的生产形态上界）"
  - "驻留格 mem 断言口径：outbox/inputQ 为容量记账非预分配（newOutbox/newInputQ 空 q 起步）——idle 驻留实测 68-103KiB/会话（账面 9-13%），ε=128KiB 为实测上界保守余量非账面放宽"

patterns-established:
  - "Pattern: 双剖面负载格（洪水+驻留）共用差值断言骨架——基线首 scrape 先行建立 keep-alive 连接使 scrape 通道对差值零贡献（churn 格先例一般化）"
  - "Pattern: 子进程内存观测走 /proc/<pid>/status VmRSS（readProcVmRSS 夹具，零新依赖红线）"

requirements-completed: []   # PC-12 文档承载归 14-11（本 plan 是数据源）；shared-ID 门先例延续

coverage:
  - id: D1
    description: "洪水格 TestLoadPerClientFloodMatrix 四档（每会话独立洪水 + spawn_total==N + kicks==0 + 收流完整）"
    requirement: PC-12
    verification:
      - kind: unit
        ref: "command: time go test -tags=load -count=1 -timeout=30m -run 'TestLoadPerClientFloodMatrix' ./internal/server/ -v → 四格 sessions_1/4/16/32 PASS（四连跑 16 格全绿）"
        status: pass
      - kind: unit
        ref: "command: go vet -tags=load ./internal/server/ && $(go env GOROOT)/bin/gofmt -l internal/server/load_test.go → 零输出"
        status: pass
    human_judgment: false
  - id: D2
    description: "驻留格 TestLoadPerClientResident 四档（差值断言三面 + VmRSS ≤15MB/进程 + ε 注释论证）"
    requirement: PC-12
    verification:
      - kind: unit
        ref: "command: time go test -tags=load -count=1 -timeout=30m -run 'TestLoadPerClientResident' ./internal/server/ -v → 四格 sessions_1/4/16/32 PASS（三连跑 12 格全绿）"
        status: pass
      - kind: unit
        ref: "LOADDATA cell=pc_resident 四行（含 rss_per_proc_max/rss_sum 字段）产出；32 格 wesh 侧 mem_delta=1991528 ≤ 32×(768KiB+128KiB)=28835840；rss_sum=120221696 ≤ ~160MB 账面"
        status: pass
    human_judgment: false
  - id: D3
    description: "既有负载格零回归（note() 夹具修正后全套件绿）"
    requirement: PC-12
    verification:
      - kind: unit
        ref: "command: time go test -tags=load -count=1 -timeout=30m ./internal/server/ -v → 全套件 PASS（3m22s，含 fanout/legit_slow/memory_bound/gate_transitions/defunct/churn 六既有格 + 双剖面新格）"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-09-06
status: complete
---

# Phase 14 Plan 07: per-client 负载矩阵双剖面（洪水+驻留）Summary

**洪水格四档每会话独立 seq 洪水（spawn_total 程序序对照 + 零误踢 + 收流完整逐端一致）与驻留格四档 idle 双采样差值三面断言（mem/gor/fd 账面全部证真）落地，D-11 两段式裁决：32 会话资源实测远低于账面——maxClients=32 默认值不动，LOADDATA 八行数据产出供 14-11 README 标定表回填。**

## Performance

- **Duration:** 40 min
- **Started:** 2026-09-06T12:40:07Z
- **Completed:** 2026-09-06T13:20:14Z
- **Tasks:** 2
- **Files modified:** 1（internal/server/load_test.go，+352/-4）

## Accomplishments

- **洪水格 TestLoadPerClientFloodMatrix（D-10）**：N∈{1,4,16,32} 每会话独立 bash `read x` 门控洪水——每客户端各发一次触发 INPUT（per-client 每会话独立 stdin，与 shared 扇出格 conns[0] 单端触发的关键差异点）；断言字节数跨端相等 + 流尾末位字段==loadFloodLast() + wesh_pty_spawn_total==N 程序序精确对照 + kicks==0；awaitDrain 240s/端护栏沿用（实测 N=32 全端 8.8s 完成，护栏余量 27×）
- **驻留格 TestLoadPerClientResident（D-12）**：N 会话 sh 零输入驻留，连续 3 拍持续达标稳定门（spawn_total==N && session_active==N && outbox_sum==0——sh prompt 排空即静默驻留态）后双采样；差值断言三面全部按账面收口：mem ≤ N×(768KiB+ε)（实测 68-103KiB/会话）/ gor ≤ 基线+6N+ε（实测精确 5N+1）/ fd ≤ 基线+4N+ε（实测精确 4N）；子进程 /proc VmRSS 逐会话采样 rss_per_proc_max≤15MB（实测 3.85MB/进程，本机 sh→bash）
- **D-11 两段式判定：证真（数据在可接受界线内）**——32 会话 wesh 侧 Alloc 增量 2.0-2.2MB（24MiB 账面的 9%）+ 子进程 VmRSS 合计 119.5-120.2MB（~160MB 账面的 75%，实测 3.85MB/进程 vs 账面假设 5MB/进程）+ gor 182（生产账面 193 上界内）+ fd 差值精确账面——**maxClients=32 默认值不动（零公开契约变更），one-way 确认门不触发**，实测数据供 14-11 README 资源义务段标定表回填
- **LOADDATA 数据行产出**（plan verification 命令原跑摘录，下一节）——cell=pc_flood / cell=pc_resident 各四行
- **验证矩阵**：双剖面八格 `-run 'TestLoadPerClient'` 全绿；洪水格四连跑 16 格全绿；驻留格三连跑 12 格全绿；全套件（-tags=load 全量含六既有负载格 + 全部常规测试）3m22s 全绿；go vet -tags=load 与 GOROOT gofmt（go1.26.3）零输出

## LOADDATA 数据行摘录

plan verification 命令（`go test -tags=load -run 'TestLoadPerClient' -v`）单跑摘录——洪水四行 + 驻留四行：

```text
LOADDATA cell=pc_flood sessions=1 spawn_total=1 profile=seq_flood(last=4000000) slowlink=none kicks=0 outbox_max=217 alloc_peak=2804512 alloc_base=934736 mem_alloc_end=1961640 bytes_per_session=34888899 dur_ms=3323
LOADDATA cell=pc_flood sessions=4 spawn_total=4 profile=seq_flood(last=4000000) slowlink=none kicks=0 outbox_max=3699 alloc_peak=2256008 alloc_base=1192120 mem_alloc_end=2280856 bytes_per_session=34888899 dur_ms=4615
LOADDATA cell=pc_flood sessions=16 spawn_total=16 profile=seq_flood(last=4000000) slowlink=none kicks=0 outbox_max=205111 alloc_peak=15965712 alloc_base=2354688 mem_alloc_end=2973024 bytes_per_session=34888899 dur_ms=6683
LOADDATA cell=pc_flood sessions=32 spawn_total=32 profile=seq_flood(last=4000000) slowlink=none kicks=0 outbox_max=524266 alloc_peak=57687144 alloc_base=3729304 mem_alloc_end=5608048 bytes_per_session=34888899 dur_ms=8792
LOADDATA cell=pc_resident sessions=1 spawn_total=1 profile=sh_idle_resident kicks=0 mem_base=1963928 mem_end=2047064 mem_delta=83136 mem_peak=2096032 gor_base=25 gor_end=31 gor_peak=31 fd_base=19 fd_end=23 rss_per_proc_max=3747840 rss_sum=3747840 dur_ms=556
LOADDATA cell=pc_resident sessions=4 spawn_total=4 profile=sh_idle_resident kicks=0 mem_base=2032280 mem_end=2272992 mem_delta=240712 mem_peak=2310968 gor_base=29 gor_end=50 gor_peak=50 fd_base=21 fd_end=37 rss_per_proc_max=3796992 rss_sum=15073280 dur_ms=559
LOADDATA cell=pc_resident sessions=16 spawn_total=16 profile=sh_idle_resident kicks=0 mem_base=2104392 mem_end=3087112 mem_delta=982720 mem_peak=3127896 gor_base=33 gor_end=114 gor_peak=114 fd_base=23 fd_end=87 rss_per_proc_max=3792896 rss_sum=59957248 dur_ms=572
LOADDATA cell=pc_resident sessions=32 spawn_total=32 profile=sh_idle_resident kicks=0 mem_base=2210696 mem_end=4202224 mem_delta=1991528 mem_peak=4255784 gor_base=37 gor_end=198 gor_peak=198 fd_base=25 fd_end=153 rss_per_proc_max=3842048 rss_sum=120221696 dur_ms=589
```

注：驻留格基线含同进程先行跑的洪水格残留（gor_base 25-37 vs 单跑 9-21）——差值形态不受影响（实测 mem/gor/fd 差值与单跑逐项一致：5N+1 / 4N / 68-103KiB 每会话）。

### 32 格与 ~160MB 账面对照（A4 裁决）

| 观测面 | 32 格实测 | 账面 | 结论 |
|---|---|---|---|
| wesh 侧 Alloc 增量 | 2.0-2.2MB | 32×768KiB = 24MiB | **证真**（9%） |
| goroutine 总量 | 198（基线 37 + 161） | 1+6×32 = 193 上界（harness pinger 退场实测 5N+1） | **证真** |
| fd 增量 | +128（精确 4N） | 4N 推导（master+pidfd+accepted+dial） | **证真**（逐项证实） |
| 子进程 VmRSS/进程 | 3.74-3.85MB（bash） | ≤15MB 宽松上界 | **证真** |
| 子进程 VmRSS 合计 | 119.5-120.2MB | ~160MB（5MB/进程假设 ×32） | **证真**（75%） |

**D-11 判定登记：数据全部在可接受界线内——「32 并发 shell 是重负载」的账面推算成立但实测余量充足，maxClients=32 默认值不动（零公开契约变更），one-way 确认门不触发**；按部署形态建议值表（如低配 VPS 建议 --max-clients=8）由 14-11 README 回填承载。

## Task Commits

Each task was committed atomically:

1. **Task 1: 洪水格 TestLoadPerClientFloodMatrix——N∈{1,4,16,32} 每会话独立洪水（D-10）** - `ab95f1c` (test)
2. **Task 2: 驻留格 TestLoadPerClientResident——N 会话 idle 空转账面实证（D-12 + D-11）** - `0062c94` (test)

## Files Created/Modified

- `internal/server/load_test.go` - note() 跨帧滚动流尾窗修正（Rule 1）+ TestLoadPerClientFloodMatrix + readProcVmRSS + TestLoadPerClientResident（+352/-4）

## Decisions Made

- **fd 账面 4N 推导的归类诚实性**：in-process 装配使测试侧 dial socket（每客户端 1）同落 /proc/self/fd——注释单独归类为 harness 成本（生产服务端账面 = master+pidfd+accepted = 3N，ARCHITECTURE §10 :480「~2×N + pidfd」+ accepted socket 的展开）；LOADDATA 原始 base/end 差值如实登记（标定诚信——非为让账面成立调松断言）
- **gor 断言按生产账面 6N 收口而非实测 5N**：harness PingInterval 零值使 pinger 启动即退场（emptyexit_test.go:305 先例），实测 5N+1；断言上界用生产形态 6N（pinger 武装时）——账面口径与 ARCHITECTURE §5 表一致，实测值在注释中如实登记
- **驻留格 mem 账面 768KiB 本体不动**：outbox/inputQ 为帧队列容量记账非预分配（newOutbox/newInputQ 空 q 起步——首跑实测 idle 驻留 68-103KiB/会话即账面的 9-13%）；ε=128KiB 只吸收控制结构（pcSession/WS-http 连接缓冲/令牌桶条目），实测上界注释论证
- **洪水格 gate_transitions 字段不采**：per-client 不装配信用门（D-03——12-05 TestGlobalCredit per-client 列显式断言未装配），恒 0 series 入行徒增噪音（注释锚定）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] spawn 节流桶放宽（洪水格 + 驻留格同款）**
- **Found during:** Task 1 首跑（sessions_16/32 档）
- **Issue:** plan 装配蓝图 `startPerClientServer(t, gatedFloodArgv(last), nil)`（mutate=nil 生产默认参数）下，同 IP（loopback 测试拓扑结构性单 IP）快速连发 N attach 超生产默认桶——per-IP 1/s burst 4 使第 5 个起、全局 8/s burst 16 使第 17 个起 spawn_throttled 1011 拒绝（首跑实测 dialHello 得 Error 帧即 Fatal）
- **Fix:** mutate 放宽四桶（SpawnPerIPRate/Burst=100 + SpawnGlobalRate/Burst=100）——13-02 TestPerClientTeardownRaceOnce perclient_test.go:1296-1306 先例同形态（「测试对象非 churn 防线，放宽隔离两测试面」）；节流行为由 TestChurnPerClientSpawnThrottle 生产默认桶参数专测，不受影响
- **Files modified:** internal/server/load_test.go
- **Verification:** 放宽后洪水格四连跑 16 格全绿 + 驻留格三连跑 12 格全绿 + churn 格全套件回归绿（节流断言面零弱化）
- **Committed in:** ab95f1c（Task 1）/ 0062c94（Task 2 同款）

**2. [Rule 1 - Bug] loadDrain.note() 流尾采样帧级重置在 tty ONLCR 行尾拆分下误判流截断**
- **Found during:** Task 1 验证期（洪水格 sessions_16 三连败复现）
- **Issue:** 末帧载荷可能仅是 tty ONLCR 行尾拆分片段——临时探针实证 `prevTail="…3999999\r\n4000000" lastTail="\r\n"`（末行 "4000000\n" 的 \r\n 膨胀单独成帧；字节跨端相等 + 1000 关闭均正常，纯 wire 帧拆分）；note() 帧级 tail 重置把帧边界误当流边界，末位字段断言 `流尾末位字段 = []` 误判流截断。共享扇出格同款潜在 flake（单 ReadLoop 时序下更罕见，历史上未显形）
- **Fix:** note() 改跨帧滚动 128 字节流尾窗（append 前先裁 keep——backing 稳定在 2×loadTailKeep 量级，TestLoadMemoryBound 的 GC 回基线断言不受累）；滚动窗保持对流截断的判别力（截断使窗内末位字段早于洪水末位）且对帧拆分免疫
- **Files modified:** internal/server/load_test.go（note() 既有夹具函数体——违反 Task 1 验收「既有 load 格零改动」的字面白名单，但为测试稳定性的必要正确性补齐且不弱化任何断言语义）
- **Verification:** 修正后洪水格四连跑 16 格全绿（修正前复现率 4/6 全量跑）+ 共享扇出格回归绿 + legit_slow/memory_bound/gate_transitions/defunct 四既有格回归绿
- **Committed in:** ab95f1c（Task 1 提交内）

**3. [Rule 1 - Bug] 驻留格首跑 gor_peak=0（500ms 峰值采样器未及触发）**
- **Found during:** Task 2 首跑
- **Issue:** 驻留稳定门首拍达标即 break，窗口仅 ~150-250ms——格内峰值采样器（500ms 间隔）零拍触发，LOADDATA gor_peak=0 无信息量
- **Fix:** 稳定门改连续 3 拍持续达标（≥400ms 持续稳态 + 峰值采样器必然捕获）——瞬时达标不构成「驻留」证据，且仍为轮询形态（禁固定 sleep 精确时点纪律不变）
- **Files modified:** internal/server/load_test.go
- **Verification:** 修正后三连跑 gor_peak==gor_end（稳定驻留的直接证据）全档产出
- **Committed in:** 0062c94（Task 2 提交内）

---

**Total deviations:** 3 auto-fixed（1 blocking + 2 bug）
**Impact on plan:** 两处 Rule 1 修正均为测试夹具语义补齐（流尾窗/驻留稳定门），断言面零弱化（流截断判别力与账面口径均保持）；Rule 3 节流桶放宽为 13-02 既定先例形态（隔离两测试面），churn 格节流断言不受影响。

## Issues Encountered

None——plan 预告的 A2 超时上调未触发（240s 护栏实测余量 27×）；A3/A4 ε 定值与账面裁决均按首跑实测定成。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- LOADDATA pc_flood/pc_resident 八行数据 + D-11 证真结论就绪——14-11 README 资源义务段标定表回填的直接数据源（09-09 D-13 先例表格形态）
- maxClients=32 默认值维持不动（零契约变更）——14-12 收口闸 diff 白名单应含本 plan 两个新测试函数 + note() 修正（Rule 1 偏差已登记）
- load tag Linux 手动跑纪律不变（darwin 面 skip 先例沿用——TestLoadPerClientResident 头部 GOOS 门控）；全套件 -tags=load 全量 3m22s 基线登记
- 既有负载格六格（fanout/legit_slow/memory_bound/gate_transitions/defunct/churn）全套件回归绿——note() 修正的跨格影响已验证

## Known Stubs

None——零 stub/TODO/skipped-test 残留（darwin skip 为平台既定豁免形态非本 plan 引入）。

## Self-Check: PASSED

- internal/server/load_test.go 在盘（+352/-4 两提交合计）
- 2 个任务提交在库（ab95f1c / 0062c94）
- frontmatter `status: complete` 在场
- 双剖面八格 -run 'TestLoadPerClient' 全绿；全套件 -tags=load 3m22s 全绿；go vet -tags=load + GOROOT gofmt 零输出
