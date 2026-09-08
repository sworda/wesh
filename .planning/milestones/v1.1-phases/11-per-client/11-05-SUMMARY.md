---
phase: 11-per-client
plan: "05"
subsystem: testing
tags: [per-client, uat, protocol, websocket, pty, esrch, kill-fallback, capacity-gate, node, zero-dependency]

requires:
  - phase: 11-per-client
    plan: "01"
    provides: "per-client 生命周期主干真实二进制行为面（attach spawn / Welcome 首帧 / teardown SIGHUP / EXIT 私有化直写）"
  - phase: 11-per-client
    plan: "03"
    provides: "D-02 容量再闸 wire 形态（Error{server_error, server is at capacity}+1011）与 linger 注入机理（Go 侧 TestPerClientCapacityGate 对照基线）"
  - phase: 11-per-client
    plan: "04"
    provides: "Go 侧行为对照基线（断开 ESRCH / EXIT 两形态 / 重连新 pid / KILL 兜底时序双断言 / kill -HUP 夹具勘误先例）"
provides:
  - "web/uat/phase11.mjs（新建 595 行）——per-client 协议层 UAT 八场景一次建齐（D-06）：S1 双 pid 独立 / S2 首帧 winsize / S3 运行期删命令 spawn 失败 / S4 断开 ESRCH（S4b 1006 形态 skipped+reason）/ S5 EXIT 私有化两形态 / S6 容量再闸 linger 注入 / S7 重连新 pid / S8 KILL 兜底时序双断言"
  - "夹具层：startWesh / dialHello / dialExpectReject / collectUntilClose / sendInput / readPid / waitScanPid / echoMark / pgroupAlive / pollESRCH——pgid ESRCH 断言通道为 PATTERNS No Analog ② 新形态首落地（setsid pgid==pid 锚点，kill(-pid,0) ESRCH ⊇ 收割完成强证据）"
  - "pid 数值红线的运行时自净形态：sensitivePids 闭包收集 + assertOutputClean 扫描（phase06 token 自净断言向 pid 维度的延伸）"
  - "S6/S8 滞留进程组显式 SIGKILL 清场纪律（CI 夹具——trap 免疫进程组绝不随脚本泄漏，三连跑后 pgrep 实证零泄漏）"
affects: [11-06, 12-interaction, 13-termination, 14-matrix]

actuals:
  tokens: 8504
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "pgid ESRCH 轮询探针（pollESRCH）：process.kill(-pid,0) 50ms 步进 + 总护栏；「无错返回 = 存活探针，严禁当死亡证据」红线注释化"
    - "非交互夹具 pid 回读（waitScanPid）：启动即印 pid 的死循环命令不读 stdin——pid 经初始 OUTPUT 扫描而非 INPUT echo（S6/S8 夹具形态）"
    - "升档拒绝面断言通道（dialExpectReject）：无 Welcome 的 Error 帧 + close 收集（phase02.mjs T4b 形态——S3/S6 B 端共用）"
    - "「零帧」断言先排空：echoMark 回读 + 300ms 落定窗后记录基线（11-04 drainQuiet 的 UAT 同构，防假阳性）"
    - "时序双断言的协议层镜像：stopseq_test.go「到期前静默窗 / 到期后护栏内收码」→ S8「300ms 存活 + 5s 护栏 ESRCH」"

key-files:
  created:
    - web/uat/phase11.mjs
  modified: []

key-decisions:
  - "S5d 自杀信号沿用 11-04 勘误（kill -TERM → kill -HUP）：交互 shell 无 trap 忽略 SIGTERM 为 bash/dash 手册语义，STATE.md 已裁决「后续 plan 信号夹具直接用 HUP/trap 形态」——断言面（exit_code==-1 + 大写 SIGHUP + close 1000）与 plan 逐字等价"
  - "pid 数值纳入红线运行时自净：sensitivePids 收集一切回读 pid，assertOutputClean 遍历 detail 扫描 token + pid + /s/ 三通道——phase06.mjs review #7 形态向本 phase pid 断言面的必然延伸"
  - "S1b/S7b pid 判定方向为 phase06.mjs S6c 的反转（per-client pid **不等** = 独立进程强证据）——注释锚定语义差异防误抄（PATTERNS §9C 纪律）"
  - "PC-02/PC-03/PC-04 需求勾选延续 11-01/11-03/11-04 既定：ID 跨 6 plan 共享，phase 末 plan 11-06 统一勾选（plan flagged_assumptions 明示保持 flagged-unverified，11-06 收口闸承载终验）"

requirements-completed: []

coverage:
  - id: D1
    description: "S1 双端独立 pid 互不串台：S1a 启动期零子进程 pgrep -P 空输出（spawn 点后置 = PC-02 直接证据）；S1b 两端 echo $$ pid 不等；S1c A 唯一标记 B 1.5s 静默窗零命中零帧"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S1a/S1b/S1c PASS ×3 连跑）"
        status: pass
    human_judgment: false
  - id: D2
    description: "S2 首帧 winsize = Hello 钳制尺寸：首帧 Welcome cols==111/rows==44（无 80x24 中间态）+ stty size 回读 \"44 111\"（rows cols 序，StartWithSize 直通实证）"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S2a/S2b PASS ×3 连跑）"
        status: pass
    human_judgment: false
  - id: D3
    description: "S3 运行期删命令 spawn 失败注入：unlink argv0 副本后 B 收恰一帧 Error{server_error, failed to start process 逐字} + close 1011；A echo 照常 + /healthz 200（Pitfall 5b 启动期 fail-fast vs 运行期 per-request degrade 哲学分界实证）"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S3a/S3b/S3c PASS ×3 连跑）"
        status: pass
    human_judgment: false
  - id: D4
    description: "S4 断开 → pgid ESRCH 无僵尸：正常关闭 ws.close(1000) 后 2s 护栏内 process.kill(-pid,0) 抛 ESRCH（setsid pgid==pid；ESRCH ⊇ 收割完成强证据）"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S4a PASS ×3 连跑）"
        status: pass
    human_judgment: false
  - id: D5
    description: "S4b 1006 真实异常断开形态（OS 网卡栈断网时序）→ pgid ESRCH"
    requirement: PC-03
    verification: []
    human_judgment: true
    rationale: "CODEBUDDY.md 分层测试策略 §5 平台豁免（真实 OS 断网时序不列阻塞项）+ Node 原生 WebSocket 无 TCP 层强杀面；脚本内 skipped+reason 登记，WINDOWS.md #31 账本在案；协议层等价物 = S4a + 11-01 detach/kick 挂点覆盖论证 + 11-04 竞态注入测"
  - id: D6
    description: "S5 EXIT 私有化两形态：A exit 42 → 末帧 EXIT exit_code==42 + close 1000；B 1.5s 窗零帧扰动 + 窗后 echo 照常；信号死亡 kill -HUP → exit_code==-1 + 大写 SIGHUP 文案 + 1000"
    requirement: PC-04
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S5a/S5b/S5c/S5d PASS ×3 连跑）"
        status: pass
    human_judgment: false
  - id: D7
    description: "S6 --max-clients=1 容量再闸（linger 注入，D-02 wire 形态协议层实证）：trap 免疫滞留 pcSessions + 注册表清空窗口 → B 过 ③位 503 闸后命中 WS 面再闸 Error{server_error, server is at capacity 逐字} + close 1011；A pgid 存活实证 trap 免疫；尾部 SIGKILL 清场"
    requirement: PC-02
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S6a/S6b/S6c PASS ×3 连跑；三连跑后 pgrep 实证零滞留进程泄漏）"
        status: pass
    human_judgment: false
  - id: D8
    description: "S7 断开重连 = 全新进程：pid1 → close → ESRCH → 重连 pid2 ≠ pid1（服务端语义；前端 terminal.reset() 归 Phase 12 不在此断言）"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S7a/S7b PASS ×3 连跑）"
        status: pass
    human_judgment: false
  - id: D9
    description: "S8 trap '' HUP + --stop-timeout=1s KILL 兜底（D-01 机制先行端到端证据）：断开后 ~300ms 时点进程组存活（HUP 免疫静默窗）→ 1s 到期后 5s 护栏内 ESRCH（SIGKILL 补发收割）"
    requirement: PC-03
    verification:
      - kind: e2e
        ref: "node web/uat/phase11.mjs /tmp/wesh-p11/wesh（S8a/S8b PASS ×3 连跑）"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-09-04
status: complete
---

# Phase 11 Plan 05: per-client 协议层 UAT 八场景 Summary

**D-06 八场景一次建齐全绿：web/uat/phase11.mjs（595 行零依赖协议层 UAT）对真实二进制实证 PC-02/PC-03/PC-04 全部用户可观测面——双端独立 pid + 启动期零子进程（pgrep 实证 spawn 点后置）、首帧 winsize 111x44 直通（无 80x24 中间态）、运行期删命令 spawn 失败 1011 他端零影响、断开 pgid ESRCH 无僵尸、EXIT 私有化两形态他端 1.5s 零帧、容量再闸 linger 注入 1011 逐字文案、重连新 pid、trap 免疫 KILL 兜底时序双断言；21/21 PASS ×3 连跑无 flake，与 Go 测试（11-03/11-04）形成双证据对照。**

## Performance

- **Duration:** 18 min
- **Started:** 2026-09-03T18:38:22Z
- **Completed:** 2026-09-03T18:56:20Z
- **Tasks:** 2
- **Files modified:** 1（web/uat/phase11.mjs 新建，595 行）

## Accomplishments

- 八场景全绿（21/21 断言 PASS + S4b skipped 豁免登记）×3 连跑无 flake，退出码全 0：`node web/uat/phase11.mjs /tmp/wesh-p11/wesh`
- 进程级断言面首落地（PATTERNS No Analog ②）：S1a 启动期零子进程经 `pgrep -P <weshPid>` 空输出实证（PC-02 spawn 点后置的直接证据）；S4/S7/S8 的 pgid ESRCH 轮询经 `process.kill(-pid, 0)` 50ms 步进 + 总护栏承载（「无错返回 = 存活探针，严禁当死亡证据」红线注释化）
- 三注入形态协议层实证（与 Go 测试同构对照成立）：S3 运行期删命令（启动期 LookPath 预检通过后 unlink argv0 副本 → exec ENOENT 路径，Pitfall 5b 哲学分界）/ S6 linger 容量窗（trap 免疫滞留 pcSessions + registry 清空 → WS 面再闸确定性触发，11-03 同机理）/ S8 trap 免疫 KILL 兜底（时序双断言：~300ms 存活 + 5s 护栏内 ESRCH，11-04 stopseq 形态镜像）
- pid 红线运行时自净：sensitivePids 闭包收集 + assertOutputClean 三通道扫描（token / pid 数值 / '/s/' 链接形态串），21 条 detail 零命中——phase06.mjs review #7 形态向 pid 断言面的延伸
- CI 夹具纪律实证：S6/S8 场景尾部显式 SIGKILL 清场滞留进程组，三连跑后 `pgrep -f` 零滞留（prohibitions 第三条兑现）

## 逐场景实测输出（验收复核要求——输出行全量记录）

```
S1a PASS 零子进程=true（attach 前 pgrep -P 空输出）
S1b PASS 解析成功=true pid不等=true（布尔表达，pid 数值零打印）
S1c PASS A回读=true B零命中=true B窗内帧数=0
S2a PASS 首帧Welcome=true cols=111 rows=44
S2b PASS 回读命中=true（stty size "44 111"，rows cols 序）
S3a PASS 回读=true（删除前 A 会话正常）
S3b PASS Error帧数=1 code=server_error 文案逐字=true close=1011
S3c PASS A回读=true healthz=200（他端与服务端零影响）
S4a PASS pid解析=true ESRCH到达=true（2s 护栏内）
S4b SKIP 1006 真实异常形态（CODEBUDDY.md §5 平台豁免 + 无 TCP 层强杀面）
S5a PASS exit_code=42 close=1000（末帧 EXIT，帧序先于 1000）
S5b PASS B窗内帧数=0（A 终结余波逐字节不可见）
S5c PASS 回读=true（窗后 B echo 照常）
S5d PASS exit_code=-1 SIGHUP大写=true close=1000（kill -HUP 形态）
S6a PASS 解析=true（初始 OUTPUT 回读 S6PID，非交互夹具）
S6b PASS Error帧数=1 code=server_error 文案逐字=true close=1011（容量文案锁定）
S6c PASS 存活=true（linger 实证——trap 免疫）
S7a PASS 解析=true ESRCH=true（pid1 旧进程终结收割）
S7b PASS 解析=true pid不等=true（重连 = 全新进程）
S8a PASS 解析=true（trap '' HUP 夹具启动即印 pid）
S8b PASS 静默窗存活=true 护栏内ESRCH=true（KILL 兜底时序双断言）
SEC PASS details=21 命中=false（输出自净：零 token 零 pid 数值）
```

## Task Commits

Each task was committed atomically:

1. **Task 1: phase11.mjs 骨架 + 夹具层 + S1-S4（双 pid/首帧 winsize/spawn 失败注入/断开 ESRCH）** - `25445f0` (test)
2. **Task 2: S5-S8（EXIT 私有化/容量再闸 linger 注入/重连新 pid/KILL 兜底）+ 八场景收口** - `99f4dec` (test)

**Plan metadata:** 见文末最终 docs commit（SUMMARY/STATE/ROADMAP/WINDOWS）

## Files Created/Modified

- `web/uat/phase11.mjs`（新建 595 行）——头注释红线登记（token/凭据/pid 数值永不进 detail）+ D-06 八场景清单 + 夹具层（startWesh/dialHello/dialExpectReject/collectUntilClose/sendInput/readPid/waitScanPid/echoMark/pgroupAlive/pollESRCH）+ 场景函数 S1-S8（S4b skipped 登记）+ assertOutputClean 自净断言 + 场景数组串行 runner 汇总退出码；仅 node: 内置模块（child_process/fs/os/path + 原生 WebSocket/fetch），零三方依赖

## Decisions Made

- **S5d 自杀信号沿用 11-04 勘误（kill -TERM → kill -HUP）**：plan 文本 S5b 写「kill -TERM 自身」，但 11-04 已实测交互 shell 无 trap 忽略 SIGTERM（bash/dash 手册语义）且 STATE.md 裁决「后续 plan 信号夹具直接用 HUP/trap 形态」——按裁决执行，断言面（exit_code==-1 + 大写 SIGHUP 文案 + close 1000）与 plan 逐字等价；脚本注释锚定勘误理由（详见 Deviations #1）
- **pid 数值纳入 SEC 自净扫描**：plan must_haves 红线要求 pid 数值不进 detail（布尔表达）——除逐断言布尔化外，将 phase06 token 自净机制延伸到 pid 维度（sensitivePids 收集 + 汇总前扫描），红线从注释纪律升级为运行时自证
- **S6/S8 pid 回读通道选型 waitScanPid**：两场景夹具为非交互死循环（不读 stdin），plan 明示「pid 经初始 OUTPUT 回读」——与 readPid（INPUT echo 通道）分设，注释锚定差异
- **「零帧」断言先排空**：S1c/S5b 的静默窗断言前先经 echoMark 回读 + 300ms 落定窗排空被测端自身在途帧（提示符/回显），防假阳性——11-04 drainQuiet 形态的 UAT 同构
- **需求勾选归 11-06**：PC-02/03/04 跨 6 plan 共享 ID，phase 末 plan 统一勾选（11-01/11-03/11-04 既定延续 + 本 plan flagged_assumptions 明示保持 flagged-unverified）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] S5b 自杀信号 plan 文本勘误（kill -TERM $$ → kill -HUP $$）**
- **Found during:** Task 2 ①（S5 信号死亡形态实现）
- **Issue:** plan 写「A2 信号死亡（kill -TERM 自身）」——与 11-04 plan 同款笔误；交互式 shell 无 trap 时忽略 SIGTERM（bash 手册 Signals 节「When bash is interactive, in the absence of any traps, it ignores SIGTERM」，dash 同语义），11-04 已实测 TERM 自杀 10s 统护到期不致死
- **Fix:** 沿用 11-04 勘误与 STATE.md 裁决（「后续 plan 信号夹具直接用 HUP/trap 形态」）改 `kill -HUP $$`；message 断言同步为大写 SIGHUP 文案（exitMessage signalName 显式映射面）；脚本注释锚定勘误链
- **Files modified:** web/uat/phase11.mjs（S5d 断言块）
- **Verification:** S5d PASS ×3 连跑（exit_code=-1 + SIGHUP大写=true + close=1000，致死即时非护栏触发）；WINDOWS.md #30 账本登记
- **Committed in:** 99f4dec（Task 2 commit）

---

**Total deviations:** 1 auto-fixed（Rule 3 阻塞类：plan 文本夹具选型沿用 phase 既定勘误裁决，断言面逐字等价）
**Impact on plan:** 无范围蔓延；TERM→HUP 为 11-04 已实测裁决的机械沿用，语义断言面（-1 + 大写信号名 + 1000）与 plan 逐字一致。建议 planner 侧知悉：后续涉及 shell 自杀夹具的 plan 文本应直接写 HUP/trap 形态（STATE.md 已登记同款指引）。

## Issues Encountered

None——一次写对无返工：Task 1 自检首轮 10/10 PASS + S4b skipped；Task 2 八场景首轮 21/21 PASS，三连跑无 flake；全部验收 grep（Task 1 六项 + Task 2 六项）首轮达标。夹具设计期已消化 11-04 实测先例（drainQuiet 同构排空 / HUP 信号选型 / 时序护栏形态）与 phase06.mjs 母本面，零调试轮次。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **11-06（phase 收口闸）**：PC-02/PC-03/PC-04 勾选承载点——本 plan coverage D1-D9 中 D1/D2/D3/D4/D6/D7/D8/D9 全 pass（协议层证据主体成形），D5（S4b 1006 形态）保持豁免登记 + WINDOWS.md #31 在案；plan flagged_assumptions 三项（PC-02/03/04 flagged-unverified / S4b 豁免 / S6 linger 形态）的终验材料齐备
- **Phase 12（交互背压）**：S7 注释已锚定「前端 terminal.reset() 归 Phase 12」边界；dialHello/readPid/echoMark/pollESRCH 夹具可直接复用于 12 阶段 UAT
- **Phase 14（双模式收口）**：phase11.mjs 为 10-CONTEXT D-06 deferred 项兑现资产——八场景形态即 herdr 场景协议层基线（ROADMAP Phase 14 SC3 的 Linux 侧半身；Windows Playwright 层分工不变）
- **零回归红线保持**：既有 phase02-09 UAT 脚本零修改（git status 实证）；本 plan diff 仅 web/uat/phase11.mjs 新建
- **威胁登记闭合**：T-11-05a（敏感值泄露）→ mitigate（红线逐字沿用 + pid 维度运行时自净，21 条 detail 零命中实测）；T-11-05b（滞留进程组泄漏）→ mitigate（S6/S8 尾部 SIGKILL 清场 + startWesh 子进程追踪收口，三连跑后 pgrep 零滞留实证）；T-11-05c（断言放宽）→ mitigate（S3/S6 两关闭面各自逐字锁定——Error 帧数==1 + 文案逐字 + close 码精确值，无「两码都接受」形态）；T-11-SC 零新依赖保持（仅 node: 内置模块）

## Self-Check: PASSED

- 文件存在性：web/uat/phase11.mjs / .planning/phases/11-per-client/11-05-SUMMARY.md 全部 FOUND
- 提交存在性：25445f0（Task 1 test）/ 99f4dec（Task 2 test）全部 FOUND
- 删除检查：两提交 `git diff --diff-filter=D` 均无文件删除（Task 2 diff 的 1 deletion 为场景数组行内替换——S1-S4 注册行 → S1-S8 注册行）
- 验收 grep：Task 1 六项（session-mode=per-client 4≥1 / pgrep 6≥1 / ESRCH 19≥1 / failed to start process 3≥1 / 退出码 0 / 仅内置模块）+ Task 2 六项（server is at capacity 4≥1 / exit_code 10≥2 / SIGKILL 8≥1 / stop-timeout=1s 4≥1 / 八场景 21/21 全绿 / 无和稀泥形态）全部达标
- 运行复核：`node --check` 语法闸 PASS；`node web/uat/phase11.mjs /tmp/wesh-p11/wesh` 三连跑 21/21 PASS + 1 skipped，退出码全 0；跑后 pgrep 零滞留进程

---
*Phase: 11-per-client*
*Completed: 2026-09-04*
