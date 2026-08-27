---
phase: 07-deployment
plan: 10
subsystem: infra
tags: [unix-socket, liveness-probe, eaddrinuse, xdg-open, zombie-reap, tdd, uat, gap-closure]

# Dependency graph
requires:
  - phase: 07-deployment
    provides: 07-02 listenSocket 序列（Lstat 类型闸/Remove/Chmod/Chown，CR-01 收窄）；07-05 openBrowser（D-26/D-27，RESEARCH Pattern 8）；07-07 phase07.mjs 协议套件；07-08 十脚本回归清单
provides:
  - G-07-3 闭合——listenSocket 活性探测（net.Dial unix：连通=存活→拒绝 EADDRINUSE 同形态文案 exit 1；失败=残留→照旧 Remove），静默赢者结构性消除
  - G-07-8 闭合（选项 A）——openBrowser Start 成功后 goroutine Wait() 收割 opener（零僵尸）+ 非零退出 stderr 警告行（D-27「只警告不阻断」覆盖运行期非零退出；警告行不含 URL，token 红线保持）
  - TestListenSocket 第六子测 live instance refused EADDRINUSE and unharmed（存活方零损伤双断言：文件仍在 + 仍连通）
  - TestOpenBrowser 第三子测 non-zero opener warns without blocking（异步警告捕获 + URL 占位串零命中反断言）
  - b6.sh B6f 轮询化（50×0.1s，消除 listening 行与异步警告行落盘到达竞态）
  - 二进制直证：b1b5.sh 7/7（B1a 转 exit1-eaddrinuse 分支）+ b6.sh 7/7（B6f warn 行）；协议层十脚本零漂移
affects: [verify-work, ship]

# Actuals (#2632) — chars/4 over the realized diff（14854 chars / 4 ≈ 3713）
actuals:
  tokens: 3713
  tasks: 3
  commits: 5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - 活性探测区分同型文件：文件类型不可区分（存活 vs 残留 socket）时以 net.Dial 建连能力再分活性；TOCTOU 窗口两向安全降级注释登记（探后死亡→下次清理；清后抢绑→Listen 真 EADDRINUSE 兜底）
    - 异步子进程警告捕获测试形态：os.Pipe 换管 os.Stderr + mutex 保护 buffer + 2s 轮询观测 + close(w)→restore→<-done 序列（05-01 happens-before 纪律的 goroutine Wait 变体，-race 干净）

key-files:
  created:
    - .planning/phases/07-deployment/07-10-SUMMARY.md
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - web/uat/phase07-b6.sh

key-decisions:
  - "G-07-8 选项 A 裁决落地：goroutine Wait + 非零退出警告行——D-27「只警告不阻断」从实现侧闭合（不改 07-CONTEXT/UAT/plan 锁定文档链）；附带闭合 opener 僵尸驻留（fire-and-forget 每次 --open 驻留一个僵尸至服务终结）"
  - "Dial 失败全形态按残留处理（「不可服务即残留」）：EACCES 等跨用户活体误删由目录写权限/sticky 位结构性抑制，D-10 systemd Restart= 零人工干预优先——07-10 flagged_assumptions 登记"
  - "存活 socket 拒绝文案与 net.Listen EADDRINUSE 逐字全等（listen unix <path>: bind: address already in use）——第二实例经 run() 既有 listen 失败通道 exit 1，错误 tier 与文案形态零新增"

patterns-established:
  - "类型闸→活性探测两级收窄链：CR-01 按类型拒非 socket，G-07-3 按活性拒存活 socket——两级各自拒绝、残留照旧清理，三态（非 socket/存活/残留）语义完备"
  - "警告行纪律延伸：运行期异步事件警告自含 wesh: warning: 前缀（03-04 启动警告纪律）、不含 URL/凭据值（P5 D-03 token 红线以 Wait err 仅 exit status N 结构性达成 + 测试运行时反断言双重锁定）"

requirements-completed: [OPS-01, OPS-11]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "存活实例同 --socket 路径第二实例被拒：EADDRINUSE 同形态文案 exit 1，存活端点零损伤（socket 文件仍在、原 listener 仍 accept）"
    requirement: OPS-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestListenSocket/live_instance_refused_EADDRINUSE_and_unharmed"
        status: pass
      - kind: e2e
        ref: "bash web/uat/phase07-b1b5.sh — B1a exit1-eaddrinuse 分支 PASS（7/7）"
        status: pass
    human_judgment: false
  - id: D2
    description: "残留 socket（SIGKILL 后）照旧自动清理、启动成功——D-10 语义零漂移；非 socket 文件拒绝（CR-01）不回归"
    requirement: OPS-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestListenSocket（stale/non-socket 等五既有子测零改动全 PASS）"
        status: pass
      - kind: e2e
        ref: "bash web/uat/phase07-b1b5.sh — B1c/B1d PASS（7/7）"
        status: pass
    human_judgment: false
  - id: D3
    description: "opener 非零退出 stderr 警告行（wesh: warning: --open: browser launcher exited with error: exit status N）+ 服务不阻断 + 子进程经 Wait 收割零僵尸；exit 0 零新增输出；警告行不含 URL"
    requirement: OPS-11
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestOpenBrowser/non-zero_opener_warns_without_blocking（-race 专项 PASS，含 URL 占位串零命中反断言）"
        status: pass
      - kind: e2e
        ref: "bash web/uat/phase07-b6.sh — B6e/B6f/B6g PASS（7/7）"
        status: pass
    human_judgment: false
  - id: D4
    description: "协议层零漂移：phase07.mjs S2 unix socket / S8 --open 直触改动面全绿；07-08 十脚本回归同款清单全绿"
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs — 34/34（S8c 平台豁免 skip）；phase02/03/04/05/05-dims/06 + 04-dom/05-dom/06-dom 全绿"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 10: Gap Closure（G-07-3/G-07-8）Summary

**listenSocket 活性探测使第二实例收 EADDRINUSE exit 1（静默赢者消除，G-07-3）+ openBrowser goroutine Wait 收割僵尸并非零退出补 stderr 警告行（D-27 字面达成，G-07-8 选项 A），b1b5/b6 二进制直证 7/7 双闭合、协议层十脚本零漂移**

## Performance

- **Duration:** 20 min
- **Started:** 2026-08-26T14:05:12Z
- **Completed:** 2026-08-26T14:25:19Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- G-07-3（major，OPS-01）闭合：listenSocket 在 Lstat 类型闸与 Remove 之间插入 net.Dial("unix") 活性探测——连通=存活实例占用→拒绝（错误文案与 net.Listen EADDRINUSE 逐字全等，经 run() 既有 listen 失败通道落地 `wesh: listen unix <path>: bind: address already in use` exit 1）；Dial 失败全形态=残留→照旧 Remove（D-10 零人工干预不回归）；TOCTOU 两向安全降级注释登记
- G-07-8（minor，OPS-11）闭合（选项 A）：openBrowser Start 成功后 goroutine cmd.Wait()——opener 子进程必被收割（每次 --open 一个僵尸的驻留消除）；非零退出补 stderr 警告行 `wesh: warning: --open: browser launcher exited with error: exit status N`（自含 warning 前缀、不含 URL——Wait err 结构性无 argv，token 红线保持）；headless 跳过行与 Start 失败警告行逐字未动
- 单测锁定：TestListenSocket 六子测（新增 live instance refused——拒绝 + 文件仍在 + 仍连通三断言）、TestOpenBrowser 三子测（新增 non-zero opener warns——os.Pipe 异步捕获 + URL 占位串零命中反断言，-race 干净）
- 二进制直证：b1b5.sh 7/7（B1a 由 silent-winner 转 exit1-eaddrinuse 分支 PASS，b.log 首行证据 `wesh: listen unix ...: bind: address already in use`）；b6.sh 7/7（B6f warn 行 PASS——B6f 轮询化消除 listening 行与异步警告行到达竞态）
- 协议层零漂移：phase07.mjs 34/34（S2/S8 直触面全绿，S8c 平台豁免 skip）；phase02/03/04/05/05-dims/06 + 04-dom/05-dom/06-dom 九脚本全绿

## Task Commits

每个任务原子提交（TDD 任务含 RED→GREEN 两提交）：

1. **Task 1 RED: TestListenSocket 存活竞争失败子测** - `6dd552e` (test)
2. **Task 1 GREEN: listenSocket 活性探测（G-07-3）** - `0fda7a0` (fix)
3. **Task 2 RED: TestOpenBrowser 非零警告失败子测** - `f5659a7` (test)
4. **Task 2 GREEN: openBrowser goroutine Wait + 非零警告行（G-07-8 选项 A）** - `f19be02` (fix)
5. **Task 3: b6.sh B6f 轮询化（deflake）** - `47ea9e0` (test)

**Plan metadata:** 见文末 final docs commit（docs: complete plan）

## Files Created/Modified

- `cmd/wesh/main.go` - listenSocket 活性探测块 + D-10 收窄链第二环注释；openBrowser goroutine Wait + 非零警告行 + Pattern 8 偏差登记注释
- `cmd/wesh/main_test.go` - TestListenSocket 第六子测（live refused）、TestOpenBrowser 第三子测（non-zero warns）+ sync import + 两函数头注释更新
- `web/uat/phase07-b6.sh` - B6f 即时 grep 改 50×0.1s 轮询（断言面不变）
- `.planning/phases/07-deployment/07-10-SUMMARY.md` - 本文件

## Decisions Made

- **G-07-8 选项 A 落地**（plan behavior 四点理据裁决）：D-27 意图从实现侧闭合——改实现 ~6 行 vs 改 07-CONTEXT/UAT/plan 整条锁定文档链；「不阻断」是不变量、「须可见」是补齐（headless 跳过尚有提示行，桌面异常非零退出不应反而静默）；Wait() 附带闭合僵尸驻留；operator 可观测性（浏览器未弹出时零诊断线索 → 警告行使桌面异常可见）
- **Dial 失败全形态按残留**（「不可服务即残留」，flagged_assumptions 登记）：EACCES 跨用户活体误删由目录写权限/sticky 位结构性抑制（/run/wesh root 拥有、/tmp sticky）；一刀切拒绝会重新打破 D-10 systemd Restart= 零人工干预（残留 0000 权限 socket 场景）
- **拒绝文案与 net.Listen EADDRINUSE 逐字全等**：`listen unix <path>: bind: address already in use`——第二实例行为与「socket 路径被真实占用时 net.Listen 兜底报错」不可区分，错误 tier（exit 1 运行时错误）与文案形态零新增

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 存活竞争子测夹具路径超 sun_path 上限**
- **Found during:** Task 1 RED（listenSocket 存活竞争子测）
- **Issue:** plan 指定的 t.TempDir() 夹具在本机拼出的路径达 ~114 字节（子测名 46 字节 + 长 TMPDIR 根），超 Linux unix socket sun_path 108B 上限——预建存活 listener 即 bind EINVAL（既存五子测名较短未触限）；测试因夹具而非实现失败，非真 RED
- **Fix:** 该子测改用 os.MkdirTemp 短根路径（~61 字节）+ t.Cleanup 清理，并在注释登记路径长度纪律；断言面零改动
- **Files modified:** cmd/wesh/main_test.go
- **Verification:** 修正后 RED 因正确原因失败（listenSocket 未拒绝），GREEN 后六子测全 PASS
- **Committed in:** `6dd552e`（Task 1 RED 提交内）

---

**Total deviations:** 1 auto-fixed（Rule 3 blocking——测试夹具环境约束，非实现变更）
**Impact on plan:** 夹具修正为 RED 有效性前提（否则测试永因 bind EINVAL 失败，与实现无关）；对生产代码零影响，无 scope creep。

## Issues Encountered

None——两修复均按 plan action 一次性落地；六段式各段首跑全绿，无回上游修正。

## Verification Evidence（六段式核心段）

- **段 1 gofmt**：GOROOT gofmt -l cmd/wesh/main.go cmd/wesh/main_test.go 输出为空（零漂移）
- **段 2 vet**：`go vet ./...` 退出 0
- **段 3 -race 全量**：`go test -race -count=1 ./...` 五包全 ok（51.4s）
- **段 4 web build**：跳过并声明——本 plan 零前端改动，dist 不动
- **段 5 二进制直证**：`time go build -o /tmp/wesh-uat/wesh ./cmd/wesh`（0.59s，产物时间戳 2026-08-26 22:18:33 +0800 已验证）→ b1b5.sh **7 PASS, 0 FAIL**（B1a = exit1-eaddrinuse；b.log 首行 `wesh: listen unix /tmp/wesh-uat/b1b5.954vUt/wesh.sock: bind: address already in use`）→ b6.sh **7 PASS, 0 FAIL**（B6f warn 行存在 + B6g GET / 200）
- **段 6 协议套件**：phase07.mjs 34/34（1 平台豁免 skip）；phase02 12/12、phase03 18/18、phase04 10/10、phase05 28/28（1 skip）、phase05-dims PASS、phase06 23/23（1 skip）；phase04-dom 37/37、phase05-dom 19/19、phase06-dom 37/37（1 skip）
- **工作区**：`git status --short` 除 STATE.md（orchestrator 预改 + 本 plan 状态更新）外干净

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 07 gap closure 两 plan 中的 07-10 已闭合（G-07-3 major / G-07-8 minor）；余 07-09（G-07-2 README nginx 配方）一项
- 07-UAT.md 残余人工项不变：A1（physical-device blocked）、B4（无 root 通道 blocked）、A2/B1/B6 的 gap 条目状态更新随 07-09/verify 流程收口
- 协议层与二进制直证链全绿，phase-07 分支可随时进入整 phase verify

## Self-Check: PASSED

- FOUND: cmd/wesh/main.go / cmd/wesh/main_test.go / web/uat/phase07-b6.sh（三改动文件均在）
- FOUND: 6dd552e / 0fda7a0 / f5659a7 / f19be02 / 47ea9e0（五提交均在 git log）

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
