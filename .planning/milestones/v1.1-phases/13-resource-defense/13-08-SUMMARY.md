---
phase: 13-resource-defense
plan: "08"
subsystem: testing
tags: [regression-gate, closing-gate, gofmt, go-vet, race-detector, darwin-cross-compile, uat-matrix, diff-audit, requirements-checkoff, wr-02-closure, d10-doc, zero-regression]
requires:
  - phase: 13-resource-defense
    plan: "01"
    provides: "stop-timeout 双默认值 + 显式位 + resolveStopTimeout 纯函数（TestStopTimeoutResolution 三态断言组）"
  - phase: 13-resource-defense
    plan: "02"
    provides: "spawn 双令牌桶端到端 + 四组扩展测试（global/XFF/expiry/wire-form）+ 四计数器字段"
  - phase: 13-resource-defense
    plan: "03"
    provides: "pcSupervisor 第二终结源 + last-reaped-code + session_end 审计 + WR-02 waitDone 栅栏（八测）"
  - phase: 13-resource-defense
    plan: "04"
    provides: "Shutdown N 进程组快照逐组信号 + 有界 join + D-state 兜底 terminate（四形态测试组）"
  - phase: 13-resource-defense
    plan: "05"
    provides: "metrics 21 series + healthz D-06 + session_start 审计 + 镜像 17→21（三断言组）"
  - phase: 13-resource-defense
    plan: "06"
    provides: "WESH_REMOTE_USER 注入链（whitelistEnv 第三参 + SpawnFunc 三参全链 + 四形态断言）"
  - phase: 13-resource-defense
    plan: "07"
    provides: "phase13.mjs 六场景两轮 29/29 基线 + TestChurn churn 负载格基线（四需求证据链最后一环）"
provides:
  - "Phase 13 收口闸六段式全绿证据：静态面（GOROOT gofmt 零输出含三处 deferred 存量归一 + go vet 零输出）+ 全量 -race 5 包 1m37.2s（Phase 13 新测 26 测函数逐名对账）+ GOOS=darwin amd64/arm64 双编译闸（附 darwin vet 加验）+ web 构建 dist byte-identical + UAT 矩阵 17 轮全绿（既有 15 脚本默认 shared 零修改重跑基线逐脚本一致 + phase13.mjs 两轮 29/29）+ churn 负载格单跑绿 + 期望值逐字未动 diff 白名单审查（23 文件全白名单内/三明示登记项逐字核对/go.mod go.sum 零 diff/放宽形态零命中）"
  - "四需求勾选承载兑现：PC-08/PC-09/SEC-09/OPS-12 [x] + Traceability 四行 Complete（证据链 = Go 新测组 26 测 + phase13.mjs 六场景两轮 + churn 负载格 + diff 审查）"
  - "PROJECT.md Key Decisions 两行登记：D-01 per-client stop-timeout 默认值 0→5s（one-way 公开契约变更）+ 第二终结源 pcSupervisor/pcExitReq 退出状态 255 对齐（ROADMAP 准则 5 登记要求）"
  - "D-10 文档明示段落地：README 保活先杀时序段 + CONFIGURATION「ping-interval 与断开时序」小节（1006 先杀语义五要点）"
  - "WR-02 闭合回指登记：STATE.md「[Phase 11 REVIEW WR-02 → Phase 13]」行按 waitDone 非阻塞 select 形态显式闭合（三处 kill(-pgid) 面同构覆盖）"
affects: [14 (Phase 14 双模式验证矩阵与 herdr UAT——本 phase 零回归基线 17 轮为其对照面；churn 防线参数经负载矩阵实测后如需调整为常量改值非公开契约变更)]

# Actuals (#2632) — 与 plan estimate (40000 tokens) 同标尺（diff chars/4）。
# 口径注记：纯验证收口 plan（11-06/12-05 同形态）——代码面仅 gofmt 存量三行归一
# （style 5310723）与文档段；diff 主体在 .planning 文档面。
# 诚实注记（12-05 同款）：diff 标尺不覆盖执行期成本——收口闸六段式实跑
# （全量 -race 97s + UAT 矩阵 17 轮 ~3.5min + churn 30s + diff 逐行审查）
# 不在 tokens 口径内。
actuals:
  tokens: 5200
  tasks: 2
  commits: 2

tech-stack:
  added: [] # 零新依赖红线终审（T-13-SC）：go.mod/go.sum + web/pnpm-lock.yaml/web/package.json/web/pnpm-workspace.yaml/web/uat/package.json 基点以来 0 行 diff
  patterns:
    - "收口闸六段式记录形态（11-06/12-05 先例第三代沿用）：命令 + 退出码 + PASS 计数 + diff 清单全量落档"
    - "phase 基点口径第三代：e0ae66b^（Phase 13 首提交「docs(13): capture phase context」的父提交 = Phase 12 CR-01 后置 fix 之后的平直 main 点）——branching_strategy=none 下 merge-base 退化，12-05 先例同构"
    - "deferred gofmt 存量的收口闸处置形态：must_haves「零输出」与 deferred 登记「范围外不修」冲突时按 Rule 3 归一（白名单内文件、纯注释/空行、断言零触碰），deferred-items.md 处置列回写——两先例纪律（登记不散失 + 闸门不妥协）同时保持"
    - "基线演变登记形态：phase12-dom 14→17 系 Phase 12 纪元 CR-01 后置 fix（a3365a4，+3 断言）所致——12-05 登记的 14 为闸前口径；Phase 13 零触碰实证（web/uat diff 仅 A phase13.mjs），如实登记非回归"

key-files:
  created:
    - .planning/phases/13-resource-defense/13-08-SUMMARY.md
  modified:
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md
    - .planning/PROJECT.md
    - .planning/STATE.md
    - .planning/WINDOWS.md
    - .planning/phases/13-resource-defense/deferred-items.md
    - README.md
    - docs/CONFIGURATION.md
    - cmd/wesh/main_test.go
    - internal/server/perclient_test.go

key-decisions:
  - "phase 基点 = e0ae66b^：Phase 13 首提交（docs(13): capture phase context）的父提交——branching_strategy=none 平直 main 下 merge-base 退化，11-06/12-05 先例同构；与 1a8659a^ 基点对代码面 diff 逐字等价（两者间仅 .planning 提交，实测 IDENTICAL）"
  - "段① gofmt 三处 deferred 存量按 Rule 3 归一（style 5310723）：must_haves truth「GOROOT gofmt 零输出」为收口闸硬要求，13-02/13-03 登记「范围外不修」的辖域是当值 plan 而非收口闸——两文件均在 phase 白名单内、修正纯注释/空行零断言触碰，deferred-items.md 处置列回写闭环"
  - "README:96 / CONFIGURATION:57/:154「per-client 行为装配中，当前版本与 shared 等价」失实残留最小修正（Rule 1）：Phase 10 D-05 时代文案，Phase 11-13 行为全部落地后与同段 stop-timeout per-client 语义自相矛盾；仅删失实括注/改行为描述，PC-12 完整模型文档段仍归 Phase 14"
  - "phase12-dom 基线按 CR-01 后置口径对齐（17/17）：a3365a4（2026-09-05 00:27）在 12-05 收口闸（62f4b7b/804b285，2026-09-04 23:25-23:27）之后 1 小时提交——+3 断言（D2e/f/g）属 Phase 12 纪元 code-review 修复，Phase 13 基点已含（merge-base --is-ancestor 实证），非 Phase 13 回归"
  - "11-06/12-05 收口提交形态沿用：Task 1 纯验证 + 段① 存量归一单 style commit + Task 2 文档面，全部产出（SUMMARY/REQUIREMENTS/ROADMAP/PROJECT/STATE/WINDOWS/deferred-items + README/CONFIGURATION D-10 段）单 docs commit 收口"

requirements-completed: [PC-08, PC-09, SEC-09, OPS-12]

duration: 37min
completed: 2026-09-05
status: complete
---

# Phase 13 Plan 08: 收口闸（六段式验证总账 + 四需求勾选 + Key Decisions/D-10 登记）Summary

**Phase 13 零回归收口闸六段式一次跑齐全绿：静态面（GOROOT gofmt 零输出——三处 deferred 存量归一 + go vet 零输出）+ 全量 -race 5 包 1m37.2s（新测 26 测函数逐名 PASS）+ darwin amd64/arm64 双编译闸 + dist byte-identical + UAT 矩阵 17 轮全绿（既有 15 脚本零修改基线一致 + phase13.mjs 两轮 29/29）+ churn 负载格 + diff 白名单审查（23 文件全白名单内/放宽形态零命中/零新依赖 0 行 diff）；PC-08/PC-09/SEC-09/OPS-12 四需求勾选收口，Key Decisions 两行 + D-10 文档段落地，WR-02 闭合回指登记。**

## Performance

- **Duration:** 37 min（2026-09-05T19:26Z 起）
- **Tasks:** 2（Task 1 六段式收口闸 + diff 审查；Task 2 勾选/登记/D-10/SUMMARY——11-06/12-05 同形态）
- **代码面改动：** 段① 存量归一 2 文件 2+/3-（style 5310723）+ D-10/残留修正文档两件；其余零代码改动

## Task Commits

1. **Task 1: 收口闸六段式执行 + 期望值逐字未动 diff 白名单审查**
   - `style(13-08)` **5310723** — GOROOT gofmt 三处 deferred 存量归一（段① 全绿前置，Rule 3，见 Deviations #1）
   - 六段式执行记录与 diff 审查结论落本 SUMMARY Verification 节（纯验证零文件改动部分）
2. **Task 2: 需求勾选 + Key Decisions + D-10 文档段 + SUMMARY 收口登记**
   - 并入文末最终 docs commit（README/CONFIGURATION D-10 段与残留修正 + REQUIREMENTS/ROADMAP/PROJECT/STATE/WINDOWS/deferred-items + 本 SUMMARY）

## Verification（六段式全量证据落档——全部命令本轮实跑，无转述前序 SUMMARY 的二手证据）

### 段① 静态面

| 命令 | 结果 |
|------|------|
| `$(go env GOROOT)/bin/gofmt -l .`（go1.26.3 linux/amd64） | 初跑命中 2 文件 3 处（= deferred-items.md 登记存量逐字吻合：main_test.go `//（fs.Visit` 行 / perclient_test.go `//（洪水后` 行 / dialHelloWithXFF 前双空行）→ **style 5310723 归一后复跑零输出 PASS** |
| `go vet ./...` | 零输出 exit 0 PASS |

### 段② 全量 -race + Phase 13 新测名单对账

`time go test -race ./... -count=1` exit 0，real **1m37.229s**：

```
ok  github.com/sworda/wesh/cmd/wesh         1.327s
ok  github.com/sworda/wesh/internal/proto   1.014s
ok  github.com/sworda/wesh/internal/pty     2.653s
ok  github.com/sworda/wesh/internal/server  95.533s
ok  github.com/sworda/wesh/web              1.011s
```

Phase 13 新测/扩展测 **26 测函数**逐名对账（聚焦 -race -v 14.921s 全 PASS；churn 格段⑤ 单跑）：

| 来源 | 测试名 | 结果 |
|------|--------|------|
| 13-01 | TestStopTimeoutResolution | PASS (0.00s) |
| 13-01 | TestValidateStartupWarnMerge（扩展第四枚负例） | PASS (0.00s) |
| 13-01 | TestConfigMerge（扩展 TOML 两 t.Run） | PASS (0.02s) |
| 13-02 | TestPerClientSpawnThrottle | PASS (0.01s) |
| 13-02 | TestPerClientSpawnGlobal | PASS (0.01s) |
| 13-02 | TestPerClientSpawnXFF（两 t.Run） | PASS (0.01s) |
| 13-02 | TestSpawnThrottleExpiry | PASS (0.01s) |
| 13-02 | TestPerClientSpawnThrottleWireForm | PASS (0.00s) |
| 13-03 | TestEmptyExitPerClientChildFirst | PASS (1.21s) |
| 13-03 | TestEmptyExitPerClientClientFirst | PASS (0.20s) |
| 13-03 | TestPerClientExitWhenEmptyImmediate | PASS (0.20s) |
| 13-03 | TestPerClientExitWhenEmptyGraceExpire | PASS (0.60s) |
| 13-03 | TestPerClientExitWhenEmptyGraceCancel | PASS (4.70s) |
| 13-03 | TestPerClientOnce | PASS (0.20s) |
| 13-03 | TestPerClientReapedFence | PASS (0.20s) |
| 13-03 | TestPerClientSessionEnd（两形态） | PASS (0.52s) |
| 13-04 | TestPerClientShutdownTwoGroups | PASS (0.21s) |
| 13-04 | TestPerClientShutdownResidualGroup | PASS (2.21s) |
| 13-04 | TestPerClientShutdownJoinBounded | PASS (0.71s) |
| 13-04 | TestPerClientShutdownDeadlineExits | PASS (2.22s) |
| 13-05 | TestMetricsPerClient（双模式） | PASS (0.06s) |
| 13-05 | TestPerClientSessionStart | PASS (0.00s) |
| 13-05 | TestSpawnEventsSchema | PASS (0.00s) |
| 13-06 | TestEnvWhitelist（扩展三分支 + e2e 双形态） | PASS (0.00s) |
| 13-06 | TestPerClientRemoteUserEnv（四形态） | PASS (0.01s) |
| 13-07 | TestChurnPerClientSpawnThrottle（load tag） | PASS (30.16s，段⑤) |

### 段③ darwin 编译闸

| 命令 | 结果 |
|------|------|
| `GOOS=darwin GOARCH=amd64 go build ./...` | exit 0 PASS（0.58s） |
| `GOOS=darwin GOARCH=arm64 go build ./...` | exit 0 PASS（0.81s） |
| `GOOS=darwin GOARCH=arm64 go vet ./...`（13-06 抓漏先例加验） | 零输出 exit 0 PASS |

（darwin 运行行为测试由 CI macOS leg 承担，本机编译闸——11-06/12-05 口径。）

### 段④ web 构建

| 项 | 结果 |
|----|------|
| `pnpm -C web build` | exit 0，real 3.294s（dist/index.html 500.35 kB） |
| dist byte-identity | md5 `d5c25e271c923572c626515f9d6ba644` 复建前后一致；`git status -- web/dist/` 零输出——本 phase 零前端改动，dist 差异即异常 → **零差异 PASS** |

### 段⑤ UAT 矩阵（17 轮 + churn 负载格）

`time go build -o /tmp/wesh-uat/wesh ./cmd/wesh` exit 0（0.775s，11841529 字节）后串行执行，日志存 /tmp/wesh-uat/uat-logs/：

| 脚本 | exit | PASS/SKIP | 基线对照（12-05 登记口径） |
|------|------|-----------|--------------------------|
| phase02 | 0 | 12/12 | 12 ✓ |
| phase03 | 0 | 18/18 | 18 ✓ |
| phase04 | 0 | 10/10 | 10 ✓ |
| phase05 | 0 | 28/28 + 1 skipped 豁免 | 28 ✓ |
| phase05-dims | 0 | DIMS PASS（D6H-1 等价锁 + D6H-2 负对照） | ✓ |
| phase06 | 0 | 23/23 + 1 skipped 豁免 | 23 ✓ |
| phase07 | 0 | 34/34 + 1 skipped 豁免 | 34 ✓ |
| phase08 | 0 | 21/21 | 21 ✓ |
| phase09 | 0 | 18/18 | 18 ✓ |
| phase11 | 0 | 21/21 + 1 skipped 豁免 | 21 ✓ |
| phase12 | 0 | 20/20 | 20 ✓ |
| phase04-dom | 0 | 37/37 | 37 ✓ |
| phase05-dom | 0 | 19/19 | 19 ✓ |
| phase06-dom | 0 | 40/40 + 2 skipped 豁免 | 40+2skip ✓ |
| phase12-dom | 0 | 17/17 | 14→17 基线演变（CR-01 后置 fix a3365a4 +3 断言，Phase 12 纪元，见 Decisions）✓ |
| phase13 轮一 | 0 | 29/29 | 29 ✓（13-07 两轮基线），real 23.1s |
| phase13 轮二 | 0 | 29/29 | 29 ✓，real 23.1s |
| **churn 负载格** | — | `time go test -tags=load -run TestChurn -count=1 ./internal/server/` PASS（30.16s） | LOADDATA attempts=300 rate=10rps **attached=33 rejected=267 throttled=267 gor 8→8（峰 12）mem 875480→989608（峰 2.9MB）fd 11→11** —— 与 13-07 基线逐项一致 ✓ |

- **零修改实证**：`git diff e0ae66b^ --name-status -- web/uat/` → 仅 `A web/uat/phase13.mjs`——既有 15 脚本（11 协议 + 4 jsdom）phase 以来零触达
- **`^  FAIL` 机器闸**：18 份日志（17 轮 + churn）零命中 PASS
- **6 行 skipped 全部带 reason 且属平台豁免类**（CODEBUDDY.md §5）：phase05 S7（像素层）、phase06 S7（真实断网栈 + 重绘观感）、phase06-dom D9/D12b（OS 断网栈/真实 AT 栈）、phase07 S8c（真实弹浏览器）、phase11 S4b（OS 网卡栈时序）——零跳过测试类项；phase13.mjs 两轮零 skip（协议层与负载格均在 Linux 开发机执行）
- **滞留进程检查**：`pgrep -f '^/tmp/wesh-uat/wesh'` 锚定零命中（11-06 patterns 锚定形态）PASS

### 段⑥ 期望值逐字未动 diff 白名单审查

**phase 基点**：`branching_strategy: none`（平直 main）下 merge-base 退化为 HEAD 自身——以 Phase 13 首提交 **e0ae66b**（`docs(13): capture phase context`）的父提交 **e0ae66b^** 为等价基点（12-05 先例同构；与 1a8659a^ 基点对代码面逐字等价，实测 IDENTICAL）。

`git diff e0ae66b^ --name-status -- . ':(exclude).planning' ':(exclude)web/dist'` → **23 文件（21 M + 2 A）3517+/174-**，与七 plan 声明集合（files_modified 并集 + SUMMARY key-files 修正口径）**逐文件吻合，白名单外零改动**：

- 产品面 10 件：main.go / perclient.go / server.go / clients.go / metrics.go / health.go / spawn.go / spawnthrottle.go（A）/ README.md / docs/CONFIGURATION.md
- 测试面 13 件：main_test / config_test / perclient_test / export_test / emptyexit_test / events_test / shutdown_test / metrics_test / options_test / spawn_test / reap_darwin_test / load_test / phase13.mjs（A）

**三项明示登记项逐字核对**（must_haves truth 2）：

1. **perclient_test.go:420-422 断言翻转**（13-05 D-06）：`if body.SessionActive` → `if !body.SessionActive` + 文案载 D-06 落地回指——13-CONTEXT D-06 明示翻转点，非放宽（翻向前「窗口期 false」语义已被 D-06 裁决取代）
2. **metricsSeries17→21 镜像扩展**（13-05）：变量重命名 + 行数闸 `3*len()` 机械跟随 + 消息参数化——公式未动，契约清单尾部追加四 counter
3. **SpawnFunc 签名机械加参**（13-06，WINDOWS #39）：perclient_test.go 26 删除行 + shutdown/metrics/events/options 四文件注入点 `_ string` 占位——每删除行均与加参后新增行配对

**既有断言行零改动核验**（删除行 174 全量归类）：

| 文件 | 删除行 | 归类 |
|------|--------|------|
| perclient_test.go | 32 | 26 签名机械（harness + 19 注入点 + spawnFn 透传）+ 5 D-06 翻转段/窗口期注释改写（13-05 声明）+ 1 gofmt 存量（段① 归一） |
| spawn_test.go | 11 | whitelistEnv 机械加参（调用与消息串配对更新，slices.Contains/Equal 断言语义逐字保持）+ 注释重排 |
| metrics_test.go | 8 | 镜像重命名（metricsSeries17→21 三引用 + 消息串） |
| reap_darwin_test.go | 3 | 13-06 fix 41d5504（whitelistEnv 第三参 + 注释） |
| options_test.go | 1 | 13-06 机械占位（`_ string`） |
| events_test.go | 1 | 13-03 声明（TestAuthFailedNoUsername 装配行，WINDOWS #37） |
| shutdown_test / load_test / export_test / emptyexit_test / main_test / config_test / health_test | 0 | 纯新增零删除（health_test.go 零 diff——shared 既有 healthz 测试零改动实证） |

**红线四查**：

- **断言放宽形态扫描**（prohibition PC-08/PC-09 双面）：全代码 diff grep「两模式都接受 / both modes / either mode / shared||per-client / per-client||shared」→ **零命中** ✓
- **零新依赖红线**（T-13-SC）：`git diff e0ae66b^ -- go.mod go.sum` = **0 行**；web 依赖清单（pnpm-lock.yaml/package.json/pnpm-workspace.yaml/uat package.json）= **0 行** ✓
- **main.ts/web src 零 diff**：`git diff e0ae66b^ -- web/src` = 0 行 ✓
- **dist 零 diff**：段④ byte-identical + 基点 diff 0 行 ✓（本 phase 零前端改动红线兑现）

**WINDOWS 偏差登记对账**：Phase 13 六条（#34 13-02 桶放宽 / #35 #36 13-03 perclient.go 两 Rule 1 / #37 events_test 收口同步边 / #38 13-04 D-state terminate / #39 13-06 签名扩散）全部在白名单文件内且为对应 plan SUMMARY 声明项——diff 审查与登记双向一致，无未登记改动。

## 需求勾选证据链映射（四证据链承载兑现）

| 需求 | Go 新测组 | phase13.mjs（两轮 29/29×2） | churn 负载格 | diff 审查 |
|------|-----------|------------------------------|--------------|-----------|
| **PC-08**（进程硬顶 + churn 防线） | SpawnThrottle/Global/XFF/Expiry/WireForm 五测（13-02）+ TestChurn（13-07） | S1（14 连 attach 成功 4/拒绝 10 全 1011 + 三方计数 10==10==10 + XFF 换键双态） | 300 次/10rps：rejected=267=throttled、gor/fd 精确回落、mem +114KB≪16MiB | 白名单内零超编改动 + 断言全基线差值形态 |
| **PC-09**（第二终结源 + Shutdown N 组 + 退出码对齐） | 13-03 八测（两时序/三形态/once/ReapedFence/SessionEnd）+ 13-04 四形态 | S3（--once/裸 flag/宽限三形态进程级 255 + 取消形态跨期存活窗）+ S4（SIGTERM 双端 1001 + 双 pgid ESRCH + session_end==2 + 退出 255） | —（churn 语境不触及终结语义） | 第二终结源 Key Decisions 登记 + 三 kill(-pgid) 面栅栏在场 |
| **SEC-09**（WESH_REMOTE_USER） | TestEnvWhitelist 三分支 + e2e 双形态（13-06）+ RemoteUserEnv 四形态（13-06） | S6（携头 attach → printenv 回读 alice/NEL 剥离 carl + shared 对照 printenv 零输出——echo 标记程序序锚定） | — | 空串不出键结构性保证 + 键名白名单代码常量单侧定义 |
| **OPS-12**（观测面 per-client 粒度） | TestMetricsPerClient 双模式 + SessionStart + SpawnEventsSchema（13-05） | S5（四计数器 series 全在 + spawn_total==1 + session_active==1 会话计数 + HELP 双模式 + 零 label；shared 对照恒 0 series 保留 + 探活文案逐字） | spawn_total==attached==33 程序序精确对照 | 零身份 label 红线注释扩到新四 series + 镜像 17→21 |

勾选承载：REQUIREMENTS.md 四条 `[x]` + Traceability 四行 Complete + Last updated 行（grep 四命中实证）；ROADMAP Phase 13 勾选 + 8/8 + Progress 表 Complete 2026-09-05。

## WR-02 闭合回指（must_haves key_links 承载）

**登记原文**（STATE.md `[Phase 11 REVIEW WR-02 → Phase 13]` 行，规划期 :112）：

> [Phase 11 REVIEW WR-02 → Phase 13]: reaped 栅栏 Wait-return→hubMu-acquire 微窗口（kill-after-reap 理论面，实际不可达=pid 回绕+µs 窗）——零成本严格修法：waitDone 在 reap 完成点关闭 + 快半段非阻塞 select 即结构性栅栏。随 Phase 13 终结语义一并处置

**闭合声明——waitDone 非阻塞 select 结构性栅栏已落地**（13-03 落码，13-04 同构扩展，本收口闸全量重跑验证）：

1. **栅栏形态**：channel 关闭态读零锁安全——`if !pc.reaped` 复检后对 `pc.waitDone` 非阻塞 select，已关闭（reap 已完成）即跳过信号分支，未关闭（waitDone 未关 = Wait 未返回）才发 kill(-pgid)。Wait-return→hubMu-acquire 微窗口内「对已收割 pgid 发信号」结构性不可达。
2. **同构覆盖三处 kill(-pgid) 面**（planner 裁定「Pitfall 2 信号/reap 序列化语义对一切 kill(-pgid) 同构适用」）：teardownPCLocked 快半段 SIGHUP 信号分支（13-03）/ teardownPCLocked 补 KILL AfterFunc 回调（13-03，Rule 1 加固）/ Shutdown 快照信号循环每组（13-04，Rule 2 加固）。
3. **验证证据**：TestPerClientReapedFence 双通道（白盒构造 waitDone 预关闭 + reaped 未置位直调 teardown——3s 护栏内收口不阻塞不 panic；源码 region 断言 select 守卫先于 SignalGroup）+ Shutdown 四形态 -race 绿 + 本收口闸全量 -race 五包与 UAT 矩阵全绿。
4. **重开条件**：无（零成本严格修法一次闭合——不同于 WR-01 的「涵盖不复刻」形态，栅栏为结构性消除而非风险接受；若未来重构 waitDone 生命周期须保持「reap 完成点关闭」不变量）。

## Flagged Assumptions 终验登记（11-05 先例同构——检索结论登记而非静默放行）

**PC-09（13-03 flagged）**——unclassified 边界形态（--once 与 --exit-when-empty 叠加、grace 宽限与 Shutdown 竞态交错）终验检索结论：

- 叠加形态：TestPerClientOnce（--once ≡ maxClients=1 + exit-when-empty + grace=0 同路径展开，13-03）Go 级覆盖 + phase13.mjs S3 三形态进程级 255 逐值
- Shutdown 交错：13-04 四形态（在线双端/残留/有界 join/到期无条件退出）覆盖 exiting 消费面——pcSupervisor 谓词 `(pcExitReq||exiting) && len==0` 两触发源经 hubCond.Broadcast 三唤醒点（入口快照前置位/teardown delete 点/Shutdown 尾部补行）收敛
- 13-03 SUMMARY 另登记边界（grace=0 立即形态置位后收割前新 attach → 新会话被服务至其终结的「已武装退出」语义）：蓝本忠实形态（§4.1 参考实现无 reset 路径），--once 下容量闸使收割前重连即被拒（结构性收敛）；phase13.mjs S3 未构造该 µs 级窗口
- **结论**：执行期（13-03..13-08 全链）未发现上述面之外的新未覆盖边界；µs 级时序窗口类（置位-收割间隙 attach）保持 **flagged-unverified**（蓝本同构语义 + 结构性收敛论证，构造性验证成本与时序窗口不可达性不匹配——12-05 WR-01「dwell 涵盖」同款风险接受口径）

**SEC-09（13-06 flagged）**——unclassified 边界形态（空用户名/缺头/多值头/头值恰为 sanitize 截断边界）终验检索结论：

- 空用户名/缺头：TestPerClientRemoteUserEnv 头缺席→捕获空串（13-06）+ TestEnvWhitelist 空串不出键双形态（单元 + e2e）——下游「不出键」结构性保证
- 多值头：提取点 `http.Header.Get` 首值语义（Go 标准库）+ sanitizeRemoteUser 清洗链承接——值面与单值头同构
- 截断边界（恰 128 rune）：sanitize 幂等（128 rune 内零截断零剥离），由 SEC-07 TestSanitizeRemoteUser 既有断言面承载（07-03）；TestPerClientRemoteUserEnv C1 剥离断言覆盖清洗通路活性
- **结论**：执行期未发现新未覆盖形态；多值头与截断边界保持 **flagged-unverified**（主面覆盖 + 同构论证；显式构造多值头 e2e 属 Phase 14 验证矩阵可选项）

## 13-01 确认门派发结果记录（Task 2 action 5）

13-01 Task 1 为 `checkpoint:decision` D-01 one-way 确认门（per-client stop-timeout 默认值重议——公开契约变更）：**用户派发 option-a**（per-client 未显式设置默认 5s + 显式 0 经显式位尊重 + validateStartup warn 泄漏风险；shared 字面 0 逐字不动）。执行期（2026-09-05）确认通过并登记 STATE.md 决策行「[Phase 13-01] D-01 one-way 门 option-a 用户派发确认落定」；本收口闸全链复验：TestStopTimeoutResolution 三态断言组 -race PASS + phase13.mjs S2 零配置实测断开至收割 ≈5.0s（两轮一致，标称 5s 零覆写实证）+ 显式 `--stop-timeout=0` 对照半场（warn 行 + 6s 泄漏存活判别面）。

## D-10 文档明示段落地（Task 2 action 4）

- **README.md**（:96 会话模式段后）：「保活先杀时序」段——默认 `--ping-interval=5s` 下 TCP 级全停读连接在停读后 5s~10s 窗口被 1006 先杀、真实浏览器结构性不可达（网络栈自动回 pong）、herdr 类自管 socket 客户端注意面、1006 触发自动重连 → per-client 下重连=全新进程的合理恢复路径
- **docs/CONFIGURATION.md**（「ping-interval 与断开时序」小节，默认值表后）：同语义五要点展开 + 写超时判读机理（写 ping 控制帧内建 5s 写超时在满发送窗口时同被判读为 pong 超时）+ `--ping-interval=0` 测试纪律注记（phase12.mjs S6 既定形态）
- grep 验收：README 1006 命中 1 / CONFIGURATION 1006 命中 4 ✓

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] GOROOT gofmt 三处 deferred 存量归一**
- **Found during:** Task 1 段①（gofmt -l 初跑命中 2 文件）
- **Issue:** 收口闸 must_haves「GOROOT gofmt 零输出」与 deferred-items.md 三处登记存量（13-02/13-03「范围外不修」）冲突——闸门无法全绿
- **Fix:** style 5310723 归一三处（main_test.go:950 `//（fs.Visit` 补空格 / perclient_test.go:1547 `//（洪水后` 补空格 / :2018 双空行归一）；两文件均在 phase 白名单内、纯注释/空行修正零断言触碰；deferred-items.md 处置列回写闭环；段① 复跑零输出
- **Files modified:** cmd/wesh/main_test.go, internal/server/perclient_test.go
- **Commit:** 5310723

**2. [Rule 1 - Bug] README/CONFIGURATION「per-client 行为装配中，当前版本与 shared 等价」失实残留修正**
- **Found during:** Task 2 D-10 段落定位（README :96 / CONFIGURATION :57/:154）
- **Issue:** Phase 10 D-05 时代文案（当时 per-client inert 属实），Phase 11-13 行为全部落地后失实——且与同段 13-01 写入的 stop-timeout per-client 双默认值语义自相矛盾（同段既说「与 shared 等价」又说「per-client 默认 5s SIGKILL 兜底」）；文档即被测物纪律下 README 当前态表述必须为真
- **Fix:** 最小修正三处——README 删「行为装配中，当前版本与 shared 等价」括注；CONFIGURATION :57 改「每 WS 客户端独立 PTY 进程，断开即终结」、:154 默认值表改行为描述；PC-12 完整模型文档段（分享链接语义/herdr 配合等）仍归 Phase 14 既定范围
- **Files modified:** README.md, docs/CONFIGURATION.md
- **Commit:** docs(13-08)（本 SUMMARY 同批）

### Plan 措辞与实证的微小出入（不构成偏差）

- UAT 矩阵「phase02-12 既有脚本」按 12-05 十六轮口径执行为 15 脚本（11 协议 + 4 jsdom）+ phase13 两轮：phase12.mjs 以单轮计入（其「两轮」基线属 12-04/12-05 当值证据面；13-08 的两轮要求由 phase13.mjs 承载）——12-05 矩阵中 phase12 两轮为其「新证据面」，本 phase 新证据面为 phase13 两轮，同构对应
- phase12-dom 17/17 vs 12-05 登记 14：基线演变经 git 时序实证（CR-01 后置 fix a3365a4 在 12-05 收口后 1 小时提交，+3 断言 D2e/f/g；merge-base --is-ancestor a3365a4 ⊂ e0ae66b^）——Phase 13 零触碰（web/uat diff 仅 A phase13.mjs），非回归，如实登记
- 段③ 附加 darwin vet 加验（plan 只列 build）：13-06 抓漏先例（reap_darwin_test.go build-tag 文件 Linux 编译面不含）的零成本扩展——本 phase 签名扩散已收口，vet 零输出确认

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-25 (Tampering: shared 回归未检出——per-client 改动溢出污染 shared 路径) | mitigate | 零回归双证据：全量 -race 五包 exit 0 + 既有 15 脚本默认 shared 零修改重跑基线逐脚本一致（段② 段⑤）+ diff 白名单 23 文件逐行归类（段⑥——shared 消费面 health_test.go 零 diff / metrics 既有 17 series 断言原样绿） |
| T-13-26 (Tampering: 断言放宽换绿——「两模式都接受」形态) | mitigate | 放宽形态全代码 diff grep 零命中 + 三明示登记项逐字核对（翻转/镜像/加参均机械或声明）+ 删除行 174 全量归类无一断言语义改动 + prohibitions 两条人工确认零违反 |
| T-13-SC (Tampering: 依赖面供应链) | mitigate | 零新依赖红线终审：go.mod/go.sum + web 依赖清单五文件基点以来 0 行 diff（段⑥）；全 phase 无装包任务，legitimacy 门未触发 |

## Known Stubs

None——本 plan 为纯验证收口（11-06/12-05 同形态）：代码面仅 gofmt 存量归一（style 5310723）；Phase 13 各 plan 落地件无桩（13-02 预埋计数器已由 13-05 接线收口，13-03 session_start emit 分片已由 13-05 落地——先序 SUMMARY Known Stubs 逐 plan 复核零遗留）。

## Issues Encountered

- gofmt 段① 初跑命中 deferred 存量（预期内——13-02/13-03 已登记「13-08 收口闸应知悉」），Rule 3 归一后复跑零输出，零返工
- UAT 脚本执行 cwd 需从仓库根（`node web/uat/phaseNN.mjs`）——web/uat 相对路径一次试错后全程标准形态，零影响
- phase12-dom 基线 14→17 差异经 git 时序三步实证（文件末次改动 a3365a4 → 该提交晚于 12-05 收口 1 小时 → 是 phase base 祖先）澄清为 Phase 12 纪元基线演变，非本 phase 回归

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **Phase 13 收口完成**：PC-08/PC-09/SEC-09/OPS-12 勾选 + 零回归双证据 + WR-02 闭合回指 + flagged 终验登记落档；v1.1 进度 10/15 需求（PC-01→10；PC-02/03/04→11；PC-05/06/07/10/11→12；本 phase 四条）
- **Phase 14（双模式验证矩阵、标定与 herdr UAT）**：本 phase 零回归基线（15 脚本 PASS 计数 + diff 审查形态）为其对照面；churn 防线参数（8/16/1/4 内部常量）经其负载矩阵实测后如需调整为常量改值非公开契约变更；PC-12 模式语义文档段承载 README/CONFIGURATION 完整模型描述（本 plan 仅最小修正失实残留）；Playwright herdr 全链在 Windows 工作站（双机拓扑，CODEBUDDY.md）
- **威胁登记闭合**：T-13-25/T-13-26/T-13-SC（收口闸三威胁）→ mitigate 全兑现；Phase 13 全部 26 威胁条目经各 plan mitigation + 本收口闸终验闭合

## Self-Check: PASSED

- 文件存在性：REQUIREMENTS.md（四 [x] + Traceability 四行 Complete + Last updated）/ ROADMAP.md（Phase 13 [x] + 8/8 + Progress Complete 2026-09-05）/ PROJECT.md（Key Decisions 两行 + 尾注）/ README.md（保活先杀时序段）/ docs/CONFIGURATION.md（ping-interval 小节）/ deferred-items.md（处置列）——全部在场
- 提交存在：5310723（style）+ docs(13-08)——git log 确认
- 证据锚点抽查：plan Task 2 verify grep 四命中实测 PASS（4 / stop-timeout:154 / README 1006×1 / CONFIGURATION 1006×4）；UAT 18 份日志在场（/tmp/wesh-uat/uat-logs/）与本文表格一致；六段式验证输出全部本轮实跑 exit 0

---
*Phase: 13-resource-defense*
*Completed: 2026-09-05*
