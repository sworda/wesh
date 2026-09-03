---
phase: 06-session-lifecycle
plan: 07
subsystem: docs
tags: [documentation, readme, uat, validation, regression, lifecycle, reconnect]

requires:
  - phase: 06-session-lifecycle/06-01
    provides: EXIT 帧契约与 accept-255 之外的终结帧文档事实（'X'/ExitPayload/三形态文案）
  - phase: 06-session-lifecycle/06-02
    provides: OQ1 accept-255 门裁决（README 收口文案逐字来源）+ 空触发迁移语义
  - phase: 06-session-lifecycle/06-03
    provides: 重连状态机行为事实（六要点文档化的实现侧依据）
  - phase: 06-session-lifecycle/06-04
    provides: CLI 契约（--once 等价关系/IsBoolFlag 三形态）+ deferred -max-clients help 重复标注项
  - phase: 06-session-lifecycle/06-05
    provides: phase06-dom.mjs 自动化等价面对照（06-UAT.md 头部登记源）
  - phase: 06-session-lifecycle/06-06
    provides: phase06.mjs 协议层证据对照 + S7 豁免指针落点
provides:
  - README 生命周期节（EXIT 终结帧/--once 等价关系逐字/--exit-when-empty 三形态与迁移语义/accept-255 收口文案）+ 断线自动重连六要点段
  - README flag 表 --once/--exit-when-empty 两行 + 协议节帧表 EXIT 'X' 行
  - 06-UAT.md 六项人工清单（编号逐步 + 预期观察 + 需求 ID + 自动化等价面对照 + headless 豁免登记）
  - 06-VALIDATION.md 实测同步（Per-Task Verification Map 实测行/nyquist_compliant+wave_0_complete 置位/失效框架引用清零）
  - -max-clients help 重复标注修复（06-04 deferred 项裁决落地）
  - Phase 6 全量回归终证（六段式 + 九脚本全绿，零适配）
affects: [phase-07, verify-work]

actuals:
  tokens: 8800
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "文档与 parse 行为同真的核对形态：README flag 表/生命周期节逐行对照 cmd/wesh/main.go parseArgs 注册与 help 文案（等价关系/三形态/空格不传值逐字一致）"
    - "人工清单自动化等价面对照形态：每项人工步骤注明已落盘的自动化断言面（phaseNN.mjs 场景号），豁免边界与覆盖事实同文登记"

key-files:
  created:
    - .planning/phases/06-session-lifecycle/06-UAT.md
  modified:
    - README.md
    - .planning/phases/06-session-lifecycle/06-VALIDATION.md
    - cmd/wesh/main.go
    - internal/server/clients.go
    - internal/server/clients_test.go
    - internal/server/resize.go

key-decisions:
  - "-max-clients help 重复标注裁决为修复（06-04 deferred 既定路由本 plan）：移除 help 文案自含 (default 32)，flag 包自动追加为单一事实源——纯展示层零语义，one-way 契约的 flag 名/语义面不动"
  - "冒烟断言按 02-06/UAT 既定惯例补 --bind 127.0.0.1：plan 字面 `--port 0 -- bash` 在 Phase 3 启动矩阵下拒绝启动（0.0.0.0 无凭据 exit 2）结构性到不了 listening 行——loopback 是全部 UAT startWesh 的统一形态"
  - "旧 UAT 脚本零适配落锤：planner 逐行核查预判成立——phase02 T4a 仅断言 close code、phase03 无子进程退出场景，EXIT 帧被『读至 close 丢弃途中帧』形态天然吸收，九脚本首跑全绿"

patterns-established:
  - "VALIDATION 文件失效引用清零纪律：断言 grep（vitest==0）自身不得含该字面量——注释转述用「前端测试框架」类上位词"

requirements-completed: [SESS-01, SESS-02, SESS-03, CORE-05]

coverage:
  - id: D1
    description: "README 生命周期节改写：EXIT 终结帧行为（含退出码/信号人话 + 1000 非静默断开）+ --once 等价关系逐字（≡ --max-clients=1 --exit-when-empty=0，两处）+ --exit-when-empty 三形态与迁移语义明示 + accept-255 收口文案逐字（子进程被 SIGHUP 终结，wesh 退出状态 255）"
    requirement: SESS-01
    verification:
      - kind: other
        ref: "验收 grep：exit-when-empty×5≥3、等价关系逐字串×2、wesh 退出状态 255×1、迁移事件语义×1"
        status: pass
    human_judgment: false
  - id: D2
    description: "README 断线自动重连段六要点（D-07/D-06 文档义务 + ROADMAP 准则 3 文档侧）：触发范围（仅异常断开，1013/明确关闭不重连）+ 退避 1s×2 封顶 30s 无限重试 + Reconnect now 手动入口 + 重连目标=同一 URL 当前进程（重启即全新会话）+ 无滚动回放（tmux/herdr 分工）+ 写权限不恢复"
    requirement: CORE-05
    verification:
      - kind: other
        ref: "验收 grep Reconnect×1≥1 + 六要点人工核读逐条在场；1013 节「当前版本无自动重连（Phase 6 提供）」陈旧表述同步改写"
        status: pass
    human_judgment: false
  - id: D3
    description: "flag 表 --once/--exit-when-empty 两行（flag 名+默认+语义逐行格式）+ 协议节帧表 EXIT 'X' 行（类型字节 + JSON 载荷形状，proto.go 单一事实源同步）"
    requirement: SESS-03
    verification:
      - kind: other
        ref: "验收 grep 'X'/0x58×2≥1；flag 行语义与 main.go 注册/help 文案逐行核对一致（含空格分隔不传值）"
        status: pass
    human_judgment: false
  - id: D4
    description: "06-UAT.md 六项人工清单（断网 30s 重连/清屏重绘观感/Reconnect now/Session ended 人话/--once 503 页/owner 重连不恢复写权限）——编号逐步 + 预期观察 + 需求 ID + 自动化等价面对照 + headless 豁免头部登记"
    requirement: CORE-05
    verification:
      - kind: other
        ref: "验收 grep Reconnect×9≥1、### N. 项×6；token 红线核读零真实值（user:pass 示例凭据同 05-UAT 先例）"
        status: pass
    human_judgment: false
  - id: D5
    description: "06-VALIDATION.md 同步：Per-Task Verification Map 占位契约行 → 06-01..06-06 实测行（Task/Plan/Wave/Requirement/测试类型/命令/文件存在状态）+ nyquist_compliant: true + wave_0_complete: true + Manual-Only 表与 06-UAT.md 互指 + 失效框架引用清零（vitest==0）"
    requirement: SESS-03
    verification:
      - kind: other
        ref: "验收 grep vitest==0、Quick command=node --test web/src/lib/*.test.ts（×4）、Full suite/Sampling Rate=九脚本清单"
        status: pass
    human_judgment: false
  - id: D6
    description: "全量回归终证：六段式逐条退出 0（GOROOT gofmt 零差异/vet/-race 五包/pnpm build/node --test 19/19/冒烟端口解析）+ 九脚本 UAT 对同一构建产物全绿（phase02 12/12、phase03 18/18、phase04 10/10、phase05 28/28+1skip、phase05-dom 19/19、phase05-dims PASS、phase04-t1-width PASS、phase06 23/23+1skip、phase06-dom 30/30+1skip）"
    requirement: CORE-05
    verification:
      - kind: integration
        ref: "Task 2 逐条命令退出 0（证据见 Verification Evidence 节）——旧脚本零适配（预期落锤）"
        status: pass
    human_judgment: false

duration: 26min
completed: 2026-08-23
status: complete
---

# Phase 06 Plan 07: Phase 6 收口——README 生命周期/重连文档 + 06-UAT 人工清单 + VALIDATION 同步 + 全量回归 Summary

**Phase 6 文档面与回归面双收口：README 生命周期节改写（EXIT 终结帧/--once 等价关系逐字/--exit-when-empty 三形态迁移语义/accept-255 收口文案）与断线自动重连六要点段兑现 D-06/D-07 明示义务，flag 表与协议节同步单一事实源；06-UAT.md 六项人工清单落位（自动化等价面逐项对照）；06-VALIDATION.md 实测同步（占位契约行退役、nyquist/wave_0 置位、失效框架引用清零）；六段式 + 九脚本对同一构建产物全绿零适配——SESS-01/02/03/CORE-05 全部落地且对 Phase 1-5 零回归，phase 验收就绪**

## Performance

- **Duration:** 26 min
- **Started:** 2026-08-23T08:32:11Z
- **Completed:** 2026-08-23T08:58:06Z
- **Tasks:** 2/2（Task 1 文档三面；Task 2 六段式 + 九脚本）
- **Files modified:** 7（1 新建 + 6 修改——含 style 清零三文件）

## Accomplishments

- `README.md` 生命周期节改写（节名「Phase 5/6 行为变更」）：默认生命周期（断开不退出、无客户端期间子进程继续运行）→ **子进程退出终结帧**（EXIT 'X' 含退出码与人话消息、信号死亡 exit_code=-1+大写信号名、随后 1000 关闭、Session ended 面板非静默断开、wesh 按子进程退出码退出）→ **--once**（等价关系逐字 ≡ `--max-clients=1 --exit-when-empty=0`、语法糖展开只填未显式位、显式矛盾值拒绝、第二客户端双点位 503、409 不复活）→ **--exit-when-empty[=duration]**（三形态逐字 + 「所有客户端断开 = 最后一个客户端断开的**迁移事件**，启动期零客户端不触发」误读澄清 + duration 只能经 = 号传入、空格形态不传值）→ **断开退出收口**（SIGHUP 子进程进程组——子进程被 SIGHUP 终结，wesh 退出状态 255，OQ1 accept-255 裁决值逐字）
- `README.md` 断线自动重连段（六要点）：触发范围（仅 1006 类异常断开；1000/1008/1009/1011 不自动重连，1013 维持手动刷新——被踢重连只会再被踢）/ 退避 1s×2 封顶 30s 无限重试（成功清零，online 事件快路径）/ Reconnect now 手动入口 / 重连目标 = 同一 URL 当前进程（**服务端重启后是全新会话**——share token 重启即废需用新链接）/ 无滚动回放（清屏 + SIGWINCH 重绘；行内历史 tmux/herdr 既定分工）/ 写权限不恢复（owner 重连按新 attach 走递补，[ro] 前缀入队）
- `README.md` flag 表 +--once/+--exit-when-empty 两行（--max-clients 行后同位）；协议节帧表 +EXIT `'X'` 行（JSON `{"exit_code":N,"message":M}`、信号死亡 -1、先于 1000、前端直显）；1013 节陈旧表述「当前版本无自动重连（Phase 6 提供）」改写为 1013 不自动重连的设计理由 + 指引
- `06-UAT.md`（新建）：头部登记 headless 豁免前提与两层自动化覆盖对照（phase06.mjs 23/23+1skip / phase06-dom.mjs 30/30+1skip）+ 前置启动命令三形态（沿用 05-UAT 先例）；六项人工清单各为编号逐步 + expected/result(pending)/source(manual)/steps/note 同构——断网 30s 重连（含 `echo $$` 同进程判据）/ 清屏重绘观感 / Reconnect now 手动跳过 / Session ended 退出码与 SIGHUP 人话（双端 + 双形态）/ --once 第二客户端 503 页 + 断开退出 255 / owner 断线重连 [ro] 前缀不恢复写权限
- `06-VALIDATION.md`：Per-Task Verification Map 占位契约行 → 06-01..06-06 九行实测行（Task ID/Plan/Wave/Requirement/Threat Ref/Secure Behavior/Test Type/Automated Command/File Exists/Status 全绿终态）；frontmatter `nyquist_compliant: true` + `wave_0_complete: true` 按实况置位；Wave 0 Requirements 四项勾选（交付 plan 标注）；Manual-Only 表两行 Test Instructions 指向 06-UAT.md Test 1/2 并注明自动化等价面；Test Infrastructure 表与 Sampling Rate 失效引用清零——Config file 行改 node --test 直跑实况（Node 24 内建 type stripping，零框架零配置）、Quick command 改 `node --test web/src/lib/*.test.ts`、Full suite/Sampling Rate 改九脚本对同一构建产物清单、Estimated runtime 按实况分档；Sign-Off 六项勾选（Approval 留 validate-phase 收口）
- `cmd/wesh/main.go`：-max-clients help 文案移除自含 `(default 32)`（flag 包自动追加为单一事实源）——06-04 deferred 项裁决落地（见 Deviations #1）
- 六段式段 1 顺带清零三文件既有 GOROOT gofmt 漂移（deferred-items 既定路由，独立 style 提交 62e6520，纯 `//（`→`// （` 排版零语义）

## Task Commits

1. **前置裁决：-max-clients help 重复标注修复** — `314d3f2` (fix)——06-04 deferred-items 登记项按既定路由（「留待 06-07 一并裁决」）由本 plan 裁决为修复
2. **Task 1: README + 06-UAT.md + 06-VALIDATION.md** — `6108d59` (docs)
3. **Task 2 段 1 清零：GOROOT gofmt 三文件** — `62e6520` (style)——纯排版零语义；Task 2 主体（六段式 + 九脚本）零文件改动零适配，无任务提交

**Plan metadata:** 见最终 docs 提交（本文件 + STATE/ROADMAP/REQUIREMENTS）

## Files Created/Modified

- `README.md` — 生命周期节改写 + 重连段 + flag 表两行 + 协议节 EXIT 行 + 1013 节同步（+27/-2 净）
- `.planning/phases/06-session-lifecycle/06-UAT.md`（新建）— 六项人工清单 + 豁免/自动化对照头部
- `.planning/phases/06-session-lifecycle/06-VALIDATION.md` — 实测行替换 + 置位 + 失效引用清零
- `cmd/wesh/main.go` — help 文案去重（1 行）
- `internal/server/clients.go` / `clients_test.go` / `resize.go` — gofmt 清零（纯注释排版 7 处）

## Verification Evidence

**Task 1 验收（全过）：**
- `grep -c 'exit-when-empty' README.md` = 5 ≥ 3；`grep -c 'Reconnect' README.md` = 1 ≥ 1；`grep -c 'Reconnect' 06-UAT.md` = 9 ≥ 1（verify 命令逐字执行 PASS）
- `grep -c "'X'\|0x58" README.md` = 2 ≥ 1（协议节 EXIT 行）
- `--max-clients=1 --exit-when-empty=0` 逐字串 ×2（生命周期节 + flag 表）；`wesh 退出状态 255` ×1（accept-255 文案逐字）
- `grep -c vitest 06-VALIDATION.md` = 0（失效框架引用清零）；Quick command `node --test web/src/lib/*.test.ts` ×4；Full suite/Sampling Rate = 九脚本清单
- 06-UAT.md `### N.` 六项齐备；README/06-UAT.md 全文无真实 token（人工核读——占位符 `<ro-token>`/`<rw-token>` 形态与 05-09 红线同款保持）

**Task 2 六段式（逐条退出 0）：**
1. `$(go env GOROOT)/bin/gofmt -l .` 输出为空（零差异——三文件漂移经 62e6520 清零后）
2. `go vet ./...` PASS
3. `go test -race -count=1 ./...` 五包全 ok（cmd/wesh 1.0s / proto 1.0s / pty 2.0s / server 49.5s / web 1.0s）
4. `time pnpm -C web build` 退出 0（2.3s；dist/index.html 499.22 kB 字节不变零漂移——本 plan 零前端源码改动）
5. `node --test web/src/lib/*.test.ts` 19/19 pass
6. `go build -o /tmp/wesh-uat/wesh ./cmd/wesh` + 冒烟显式断言 PASS：spawn `--bind 127.0.0.1 --port 0 -- bash --norc --noprofile` → 8s 内 stdout 出现 `listening on http://127.0.0.1:37467` 行且正则解析端口 37467（\d+ 且 0<port<65536）→ SIGKILL 收口

**Task 2 九脚本 UAT（对同一构建产物 /tmp/wesh-uat/wesh，总耗时 1m36s）：**

| 脚本 | 结果 |
|------|------|
| phase02.mjs | 12/12 协议断言通过 |
| phase03.mjs | 18/18 协议断言通过 |
| phase04.mjs | 10/10 协议断言通过 |
| phase05.mjs | 28/28 + 1 skipped（豁免） |
| phase05-dom.mjs | 19/19 DOM 断言通过 |
| phase05-dims.mjs | PASS（D6H-1 等价锁 + D6H-2 负对照） |
| phase04-t1-width.mjs | PASS（U11 4/4 + U6 对照 1/1） |
| phase06.mjs | 23/23 + 1 skipped（S7 豁免） |
| phase06-dom.mjs | 30/30 + 1 skipped（D9 豁免） |

**旧脚本适配明细：零适配**（planner 逐行核查预判落锤）——phase02.mjs T4a scenarioExit 仅断言 close.code===1000 无帧序断言、phase03.mjs 无子进程退出场景；新增 EXIT 帧被各脚本『读至 close 丢弃途中帧』形态天然吸收。安全断言面零改动（git 核读：本 plan 对 web/uat 全部脚本零 diff）。

## Decisions Made

- **-max-clients help 重复标注裁决 = 修复**（06-04 deferred 项，orchestrator 明示待裁）：help 输出 `(default 32) (default 32)` 重复系 05-07 注册时文案自含标注 + flag 包对非零 int 默认值自动追加所致；修复 = 移除文案自含份（单一事实源 = flag 包追加），默认 32 仍恰好显示一次。判定依据：纯展示层零语义、one-way 契约的 flag 名/语义面不动（P2 D-15 约束对象是契约行为而非文案排版）、零测试断言该串、v1.0 发布前最后文档 plan 是既定裁决点
- **冒烟断言补 --bind 127.0.0.1**（Rule 3 机械调和）：plan 字面 `--port 0 -- bash` 在 Phase 3 启动校验矩阵下（默认 0.0.0.0 无凭据）拒绝启动 exit 2，结构性到不了 listening 行——02-06「随机端口 + 启动行解析」先例与全部 UAT startWesh 的统一形态即 loopback；断言面（listening 行 + 端口数字解析 + SIGKILL 收口）逐字保持
- **VALIDATION 失效引用清零的自指规避**：验收 grep `vitest==0` 是机械检查——Config file 行的转述用「项目内不存在前端测试框架依赖」上位词表述，注释自身不含被清零字面量（05-08 C-4/C-6 常量化登记的同类纪律）
- **frontmatter 置位时机**：nyquist_compliant/wave_0_complete 在 Task 1 提交时按实况置位（06-01..06-06 全部 plan 已绿终态），Task 2 全量回归为终证——若回归见红则修复后复验，置位语义不变

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] -max-clients help 默认标注重复修复（deferred 项裁决落地）**
- **Found during:** Task 1 前置（06-04 deferred-items.md 登记，orchestrator prompt 明示 awaiting adjudication）
- **Issue:** `--help` 输出 `-max-clients` 行 `(default 32) (default 32)` 重复——05-07 help 文案自含标注，flag 包对非零 int 默认值再自动追加一份
- **Fix:** 移除文案自含份（`maximum simultaneous attached clients (default 32)` → `maximum simultaneous attached clients`），flag 包追加为单一事实源
- **Files modified:** cmd/wesh/main.go
- **Verification:** `go run ./cmd/wesh --help` 输出恰好一份 `(default 32)`；`go test -race -count=1 ./cmd/wesh/` 全绿（零测试断言该串）；六段式段 2/3 复验
- **Committed in:** `314d3f2`

**2. [Rule 3 - Blocking] 冒烟命令补 --bind 127.0.0.1**
- **Found during:** Task 2 段 6（冒烟断言设计核对）
- **Issue:** plan 字面 `--port 0 -- bash --norc --noprofile` 缺 bind——默认 0.0.0.0 无凭据触发 D-03 启动矩阵拒绝（exit 2），listening 行结构性不可达
- **Fix:** 按 02-06 先例与全部 UAT startWesh 统一形态补 `--bind 127.0.0.1`（loopback 免凭据矩阵放行）
- **Files modified:** 无（冒烟为一次性内联脚本，不入库）
- **Verification:** 冒烟 PASS（listening 行 + 端口 37467 解析 + SIGKILL 收口）
- **Committed in:** 无文件改动

**3. [Rule 3 - Blocking] GOROOT gofmt 三文件既有漂移清零（deferred-items 既定路由）**
- **Found during:** Task 2 段 1（gofmt -l 输出非空：clients.go/clients_test.go/resize.go）
- **Issue:** 01-03/05-09 登记的 /usr/bin/gofmt 陈旧同源 CJK 注释排版漂移（`//（`→`// （`），deferred-items 明示「留待后续 plan 的六段式段 1 或独立 style 提交处理」——本 plan 即该路由终点
- **Fix:** GOROOT gofmt -w 三文件，gofmt -d 逐行核读纯注释排版零语义（7 处 hunk 全为空格插入），独立 style 提交（02-06/03-06/05-09 先例第四次沿用）
- **Files modified:** internal/server/clients.go, internal/server/clients_test.go, internal/server/resize.go
- **Verification:** 清零后 gofmt -l 输出为空；build/vet/-race 全绿
- **Committed in:** `62e6520`

---

**Total deviations:** 3 auto-fixed（1 Rule 1 展示层裁决 + 2 Rule 3 机械调和/清零）
**Impact on plan:** 全部为既定路由项与字面不可达调和；断言面零损失。旧 UAT 脚本**零适配**（prohibition 白名单两类变更未触发——无变红即无适配）。无范围蔓延。

## Threat Flags

None——T-06-07a（文档示例 token 红线：占位符形态保持 + 人工核读入验收，已落地）、T-06-07b（旧 UAT 适配削弱断言面：零适配，git 核读 web/uat 零 diff，已闭环）、T-06-SC（零新依赖零安装）均在 plan threat_model 登记内，无新增未登记面。

## Known Stubs

None——本 plan 全为文档与回归验证，文档内容与实现行为同真（flag 表/生命周期节逐行对照 parseArgs 注册与 help 文案核对）；六项人工清单为显式 pending-manual 状态（非 stub——headless 硬约束下的既定人工验证通道，phase06.mjs S7 与 phase06-dom.mjs D9 豁免指针已逐字指向本文件）。

## Issues Encountered

None——plan 的 must_haves 逐字规格 + read_first 先例（05-UAT 形态/05-09 六段式纪律/06-01..06-06 SUMMARY 事实源）完备；三处 deviation 均为首跑一次性修正，无反复。

## Phase 6 遗留事项（Deferred Ideas 对照——output 指定记录）

| Deferred 项 | 既定路由 | 本 plan 状态 |
|-------------|----------|--------------|
| 顶部状态条重连 UI | 后续迭代 | 未提前实现（D-03 全屏面板保持） |
| 会话代际标识（generation id） | 用户反馈再评估 | 未提前实现（D-07 文档明示替代——README 重连段「重启后全新会话」已落） |
| --exit-when-empty 宽限默认值负载标定 | Phase 9 | 未提前实现 |
| 1001 优雅下线发送路径 | Phase 7 | 未提前实现（README 关闭码表占位行保持） |
| 新 flag 配置文件收口 | Phase 7 OPS-09 | 未提前实现 |
| 断开退出事件 metrics/审计日志 | Phase 8 OPS-07/08 | 未提前实现 |
| ~~-max-clients help 重复标注~~ | ~~06-07 裁决~~ | **本 plan 闭合（314d3f2）** |
| ~~clients.go 等三文件 gofmt 漂移~~ | ~~六段式段 1~~ | **本 plan 闭合（62e6520）** |

## Next Phase Readiness

- **phase 验收（/gsd:verify-work）前置完成**：ROADMAP Phase 6 成功准则三条分层证据齐备——准则 1（--once/--exit-when-empty：cmd/wesh 表测试 + emptyexit_test 六测 + phase06.mjs S3/S4/S5 进程级 255）、准则 2（EXIT 帧 + 1000：proto/exit 测试 + phase06.mjs S1/S2 + phase06-dom D7 逐字文案）、准则 3（断网 30s 重连接回同一进程 + 无滚动回放文档明示：phase06-dom D1/D8 + phase06.mjs S6 pidPre==pidPost + README 重连段 + 06-UAT.md Test 1/2 人工清单）
- **Phase 7**：README 关闭码表 1001 占位行、配置文件收口（OPS-09）文档侧基线已在位；SESS-01/02/03/CORE-05 四条需求待 requirements.mark-complete 勾选
- 关注点：无阻塞；06-UAT.md 六项 pending-manual 待有浏览器环境执行（平台豁免项，不阻塞验收——自动化等价面已逐项目注）

## Self-Check: PASSED

- 全部 4 个产物文件（README.md / 06-UAT.md / 06-VALIDATION.md / cmd/wesh/main.go）+ style 三文件落盘核实（FOUND ×7）
- 任务提交 314d3f2 / 6108d59 / 62e6520 在 git log 核实（FOUND ×3）；三提交 post-commit 删除检查均无文件删除、无遗留 untracked
- Task 2 全量证据（六段式 + 九脚本）退出码逐条核实（PASS ×15）

---
*Phase: 06-session-lifecycle*
*Completed: 2026-08-23*
