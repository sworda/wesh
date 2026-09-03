---
phase: 10-mode-assembly
verified: 2026-09-03T04:50:00Z
status: passed
score: 19/19 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 18/19
  gaps_closed:
    - "WR-01：SC4 预检 --cwd 感知对齐——per-client × --cwd × 相对路径 argv0 可执行 → 放行（truth #5 FAILED → VERIFIED，进程级冒烟腿 A 由 exit 2 误拒转 listening on + session_start）"
    - "WR-02：ValidateOptions 前移至分岔块尾部、pty.Start/listen 之前——单调用点 + V(1328)<P(1334)<L(1342) 位序断言成立，守卫触发零资源占用"
  gaps_remaining: []
  regressions: []
---

# Phase 10: 模式装配与接缝 Verification Report

**Phase Goal:** 会话模式阀门与全部接缝一次装配到位（全部 inert）——`--session-mode` 公开契约锁定，默认 shared 逐字节零回归，per-client 分支挂点唯一化防散点 if/else 腐化
**Verified:** 2026-09-03T04:50:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure（10-05 plan，fix commits 189d081/0ec37cb，验证基点 main HEAD 0424100 = aaaaa3e + docs-only）

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1：`--session-mode=per-client`/显式 shared/缺省三形态启动被接受（listening on）；缺省 shared 下 v1.0 全量 Go 测试与既有协议 UAT 原样全绿、行为逐字节不变 | ✓ VERIFIED | 复验冒烟 START_default/START_explicit_shared/START_perclient 三形态均 listening on（rc=124）；`go test -race -count=1 ./...` 五包全 ok（58s）；web/uat/phase02-09 八脚本以 HEAD 新鲜构建二进制显式传参重跑 exit 全 0，PASS 计数 12/18/10/28/23/34/21/18 逐脚本对齐 10-04 基线，FAIL=0 |
| 2 | SC2：非法模式值 CLI/TOML 双源 parse 期拒绝 exit 2，D-04 文案；凭据/token/文件内容红线保持 | ✓ VERIFIED | 进程级复跑：CLI banana → rc=2 + `invalid --session-mode "banana": must be shared or per-client` 逐字；TOML `session-mode = "banana"` → rc=2 同文案；`session_mode` 下划线 → rc=2 unknown keys；新文案面（not executable/not found in PATH）仅含 %q argv0 回显（D-04 豁免），无凭据/token/文件内容 |
| 3 | SC3：优先级链 CLI flag > TOML > 内置默认 shared（env 层真空，D-03） | ✓ VERIFIED | TestConfigPrecedence 等四具名测试 PASS；全仓 grep `WESH_SESSION_MODE` 仅命中 config_test.go:887 注释（env 真空保持） |
| 4 | SC4（拒绝半）：per-client × argv0 缺失 → 启动期 fail-fast exit 2；shared × 缺失 → 不预检零漂移 | ✓ VERIFIED | 冒烟 F 裸名缺失 → rc=2 + `not found in PATH (per-client startup preflight)` 逐字（SC4 既有面零漂移）；冒烟 SHARED_no_preflight：shared × 同裸名 → rc=1（pty.Start 现状通道）且 stderr 零 "startup preflight" 字样 |
| 5 | 10-02（放行半）：per-client × 可执行命令 → 放行（**前次 FAILED，WR-01 本体**） | ✓ VERIFIED | **WR-01 闭合**：冒烟腿 A per-client × --cwd=appdir × ./run.sh（0755 存在）→ `session_start` + `listening on`，rc=124（前次 exit 2 误拒转放行）；代码 main.go:1051-1063 `--cwd` 感知 `filepath.Join(cfg.cwd, probe)`（:1054）+ os.Stat 可执行探测，与 spawn.go:83 `cmd.Dir = opts.Dir`（空串=继承服务端 cwd）解析基准对齐；TestStartupMatrix 三 WR-01 子测逐 PASS（计数==3） |
| 6 | run() 装配期一次分岔：shared 分支 SpawnFunc=nil；per-client 装配捕获 argv+StartOptions 闭包（StartWithSize 直通）并透传 Options.SessionMode；两模式均经启动期 pty.Start（全部 inert） | ✓ VERIFIED | main.go:1314-1320 分岔块（修复后行号迁移，形态不变）；:1334 pty.Start 无条件执行；:1370 Options 字面量尾部仅 SessionMode/SpawnFunc 两键；SpawnFunc 全仓零调用方（仅 server.go:341/344 nil 性校验 + :446 结构体存储）——inert 纪律保持 |
| 7 | ValidateOptions 互斥校验 fail-fast：per-client×nil SpawnFunc 拒绝、shared×非nil 拒绝、零值归一 shared | ✓ VERIFIED | server.go:336-348 两规则 + 零值归一（:337-339）；TestValidateOptions PASS；New :392-393 零值兜底同口径 |
| 8 | pty.Start ≡ StartWithSize(argv, opts, SpawnCols, SpawnRows) 单行委托；80×24 单一事实源 | ✓ VERIFIED | spawn.go:67-69 单行委托、:76 StartWithSize 承载全逻辑；TestStartWithSizeDelegation + TestStartZeroValueParity PASS |
| 9 | 零回归收口闸：v1.0 全量测试原样全绿、既有测试行零改动 | ✓ VERIFIED | -race 五包全 ok；`git diff -U0 781af48..HEAD` 五测试文件删除行 == 0（append-only）；web/uat/ 零修改（git status 零输出） |
| 10 | writePolicySet × per-client → warn 放行，文案含 --write-policy 与 --session-mode 双 flag 名 | ✓ VERIFIED | 冒烟 WARN_dual_flags：`--writable --write-policy=all --session-mode=per-client` → warn 行（main.go:992 逐字含双 flag 名）+ listening on |
| 11 | warn 合并不遮蔽：新 warn 与非 loopback 安全警告同现；既有 warn 文案逐字未动 | ✓ VERIFIED | 冒烟 WARN_merge：bind 0.0.0.0 + --no-auth + 组合 → `wesh: warning:` 行计数 == 2，--no-auth/--write-policy/--session-mode 三子串齐全；mergeWarn 五点透出形态（:997, :1126/:1129/:1142/:1144/:1150/:1152）在码 |
| 12 | TOML session-mode 铺底生效 + sessionModeSet 置位 + CLI 覆盖 | ✓ VERIFIED | TestConfigMerge/TestConfigPrecedence PASS；config.go:86 `SessionMode *string toml:"session-mode"` 在树 |
| 13 | TOML redlines：下划线未知键拒绝 / banana 与 CLI 同文案 / 类型不符 invalid toml 分支 | ✓ VERIFIED | TestConfigRedLines PASS + 进程级双源冒烟（见 #2） |
| 14 | FuzzDecodeFileConfig 五新种子随零时长回归门全过 | ✓ VERIFIED | fuzz_test.go:83-89 五种子逐注释登记在树；种子语料随 `go test -race ./cmd/wesh` 正常模式执行全过（包 ok） |
| 15 | 精确匹配禁令：大小写变体/前后空白近形值一律 parse 期拒绝 | ✓ VERIFIED | 冒烟 NEAR_upper（`SHARED`）/NEAR_space（`" shared"`）均 rc=2 + invalid 文案 |
| 16 | CONFIGURATION.md 五处落地 + 29→30 计数 + 「装配中」注记强制口径 | ✓ VERIFIED | :57 键表、:101 显式位、:125 校验矩阵、:132 warn 清单、:154 默认值表五处齐；`共 30 键`/`全部 30 个配置键` 计数 == 2；`装配中` 注记 == 2 |
| 17 | README.md 一句最小明示；--help 与文档口径一致 | ✓ VERIFIED | README:96 四要素齐；`wesh --help` 含 `session mode: shared\|per-client (default shared; per-client is being assembled and currently behaves as shared)`（main.go:354 源字符串逐字一致） |
| 18 | 静态面干净：gofmt 零输出、go vet 干净、go build 成功 | ✓ VERIFIED | `go vet ./...` 零输出；GOROOT gofmt（go1.26.3）与 PATH gofmt 双版 `-l cmd internal web` 均零输出（a412a87 归一持效）；fresh `go build ./cmd/wesh` 成功 |
| 19 | 冒烟三命令进程级证据（CLI banana / TOML banana / per-client 接受） | ✓ VERIFIED | 本复验独立重跑 20 项进程级冒烟全 PASS（含 WR-01 六形态矩阵 + 契约回归面），见 Behavioral Spot-Checks |

**Score:** 19/19 truths verified（含前次 FAILED 的 truth #5 转 VERIFIED）

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/wesh/main.go` | flag 注册 + 枚举闸 + SC4 预检（--cwd 感知）+ run() 分岔 + ValidateOptions 前移调用 | ✓ VERIFIED | :354 注册、:1051-1063 预检块（WR-01 目标态）、:1314-1320 分岔、:1328 ValidateOptions 单调用点（WR-02 目标态）、:1370 opts 两键 |
| `cmd/wesh/config.go` | fileConfig.SessionMode 第 30 键 | ✓ VERIFIED | :86 `toml:"session-mode"` |
| `internal/server/clients.go` | SessionMode 枚举常量对 | ✓ VERIFIED | :88/:90 |
| `internal/server/server.go` | Options 两字段 + ValidateOptions + New 兜底 | ✓ VERIFIED | :301/:309 字段、:336-348 ValidateOptions、:392-393 兜底、:445-446 直传 |
| `internal/pty/spawn.go` | StartWithSize 导出 + Start 单行委托 + cmd.Dir 语义锚 | ✓ VERIFIED | :67-69/:76；:83 `cmd.Dir = opts.Dir`（预检对齐对照事实源） |
| `cmd/wesh/main_test.go` | 启动矩阵含 WR-01 三行 + cwdCmdDir fixture，append-only | ✓ VERIFIED | :699-708 fixture（run.sh 0755/noexec.sh 0644/t.TempDir）、:832-834 三命名行；781af48..HEAD 删除行 == 0 |
| `internal/server/options_test.go` | TestValidateOptions 表驱动 | ✓ VERIFIED | PASS |
| `internal/pty/spawn_test.go` | TestStartWithSizeDelegation | ✓ VERIFIED | PASS |
| `cmd/wesh/config_test.go` | merge/precedence/redlines | ✓ VERIFIED | PASS，append-only |
| `cmd/wesh/fuzz_test.go` | 五新种子 | ✓ VERIFIED | :83-89，append-only |
| `docs/CONFIGURATION.md` | 五处 + 计数 | ✓ VERIFIED | 见 truth #16 |
| `README.md` | 一句明示 | ✓ VERIFIED | :96 |
| `.planning/phases/10-mode-assembly/10-05-SUMMARY.md` | 闭合证据（对账表 + 收口闸数字） | ✓ VERIFIED | 存在且含 WR-01/WR-02 missing 逐项→证据对账表；本复验全部数字独立复跑一致 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| parseArgs 枚举闸 | clients.go 常量 | 共用同一对常量 | ✓ WIRED | 双写漂移结构性消除 |
| run() per-client 闭包 | pty.StartWithSize | 闭包体直通（main.go:1317-1319） | ✓ WIRED | 有意 inert 零调用方（T-10-01c） |
| config.go SessionMode | Parse 返回处枚举闸 | 默认值替换落同一终值 | ✓ WIRED | TOML banana 进程级同文案 |
| run() | ValidateOptions → pty.Start → listen | **位序 V(1328) < P(1334) < L(1342)** | ✓ WIRED | **WR-02 闭合**：单调用点计数门 == 1（`grep -c 'verr := server.ValidateOptions('`）；失败通道 `wesh: %v` + return 2 与 validateStartup 同构；守卫触发时 spawn/listen 均未发生，零资源占用纪律恢复；opts 注释 :1366-1369 登记前移事实、旧调用点不存在 |
| validateStartup warn | writePolicySet × sessionMode | D-02 显式位锚定（:990-992） | ✓ WIRED | 冒烟双证 |
| validateStartup 预检 probe | cfg.argv0 × cfg.cwd | **filepath.Join(cfg.cwd, probe)（:1054）≡ spawn 侧 child chdir 后 execve 基准（spawn.go:83）** | ✓ WIRED | **WR-01 闭合**（前次 ⚠️ PARTIAL）：cwd 非空相对斜杠 → join 后 os.Stat；cwd 空相对斜杠 → 服务端 cwd stat（child cmd.Dir 零值继承服务端 cwd，基准一致）；裸名 → LookPath（PATH 解析与 cwd 无关） |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| run() → server.New | SessionMode | parseArgs 双源合并终值 | ✓ CLI/TOML/默认三层进程级实证 | ✓ FLOWING |
| run() → server.New | SpawnFunc | 分岔闭包（StartWithSize 直通） | ✓ inert 装配，ValidateOptions 消费 nil 性 | ✓ FLOWING |
| validateStartup | argv0 × cwd | parseArgs argv[0] + --cwd 落定 | ✓ 六形态冒烟实证（含 join 后 stat 真实放行/拒绝） | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| **WR-01 腿 A（前次 FAIL 面）**：per-client × --cwd × ./run.sh 可执行 → 放行 | 新鲜构建二进制 + timeout 3 | `session_start` + `listening on`，rc=124 | ✓ PASS |
| WR-01 腿 B：./no-such.sh 缺失 | 同上 | rc=2 + `invalid command "./no-such.sh": not executable (per-client startup preflight)` 逐字 | ✓ PASS |
| WR-01 腿 C：./noexec.sh 0644 无执行位 | 同上 | rc=2 + `not executable (per-client startup preflight)` | ✓ PASS |
| WR-01 腿 E：无 --cwd 且服务端 cwd 无此文件 | 同上 | rc=2 + `not executable` | ✓ PASS |
| WR-01 腿 F：裸名缺失（SC4 既有面） | 同上 | rc=2 + `not found in PATH (per-client startup preflight)` 逐字零漂移 | ✓ PASS |
| WR-01 腿 D2：shared × --cwd × ./run.sh 对照 | 同上 | `session_start` + `listening on`，rc=124（shared 零漂移） | ✓ PASS |
| WR-02 单调用点 + 位序 | grep -c == 1；V=1328 < P=1334 < L=1342 | 数值断言成立 | ✓ PASS |
| WR-02 契约测试 | `go test ./internal/server -run TestValidateOptions` | PASS | ✓ PASS |
| WR-01 行为锁 | `go test -run 'TestStartupMatrix/per-client_cwd-relative' -v` | `--- PASS` 计数 == 3 | ✓ PASS |
| CLI/TOML banana / 下划线 / 近形值 ×2 | 进程级 | 均 rc=2，文案逐字命中 | ✓ PASS |
| 三形态启动接受 | 进程级 timeout 3 | 均 listening on | ✓ PASS |
| shared 不预检 | shared × 裸名缺失 | rc=1（非 rc=2），stderr 零 preflight 字样 | ✓ PASS |
| warn 双 flag 名 / warn 合并 2 行三子串 | 进程级 | 全命中 | ✓ PASS |
| --help 文案 | `wesh --help` | 逐字命中 | ✓ PASS |
| 全量 -race | `go test -race -count=1 ./...` | 五包全 ok（58.4s） | ✓ PASS |
| UAT 八脚本（HEAD 新鲜二进制显式传参） | `node web/uat/phase0{2..9}.mjs $BIN` | exit 全 0；PASS 12/18/10/28/23/34/21/18 对齐基线，FAIL=0 | ✓ PASS |

冒烟合计：20/20 PASS（独立脚本 /tmp/wesh-v10-reverify/smoke.sh，运行时材料不入库——D-06 纪律）。

### Probe Execution

SKIPPED（本阶段无 probe 脚本约定——验证面为 Go 测试 + UAT + 进程级冒烟，已在上节全覆盖）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PC-01 | 10-01/10-02/10-03/10-04 + 10-05（五 plan 全数申领） | `--session-mode=shared\|per-client` flag（或 TOML 键）选择会话模式；缺省 shared，v1.0 全部行为逐字节不变 | ✓ SATISFIED | 主文面（flag/TOML/默认/枚举闸/优先级链）+ 零回归双证据（-race 五包 + 八 UAT 原样计数对齐）全部成立；前次派生行为缺口（SC4 预检 --cwd 误拒）经 WR-01 闭合转绿（truth #5 进程级实证）——REQUIREMENTS.md traceability `PC-01 \| Phase 10 \| Complete` 与代码事实一致 |

孤儿需求检查：REQUIREMENTS.md 仅 PC-01 映射 Phase 10（:191），五 plan `requirements:` 字段全数申领 PC-01——无孤儿、无遗漏。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | 债务标记扫描（TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER）覆盖 12 个 phase 文件 | — | 零命中 |
| — | — | 前次两条（WR-01 Blocker / WR-02 Warning） | 已闭合 | 见 truth #5 与 Key Links 第四/六行 |

**Info（非 gap）**：工作树 web/dist/index.html、web/package.json、web/pnpm-lock.yaml 自 2026-09-01 14:29 起处于未提交修改态（xterm beta 升级实验），早于 Phase 10 全部执行与证据采集——10-04/10-05 收口闸与本复验均基于同一树状态，证据内部自洽；属既有 WIP，非本 phase 交付面（Phase 10 零前端改动纪律不破——10-05 期间 web/ 三文件零新增改动）。

### Human Verification Required

无。本 phase 交付面全部 inert（零 per-client 运行期行为）、零前端改动；全部 truths 均有进程级/测试级行为证据（含前次 FAILED 的 truth #5 经六形态矩阵行为化复验），无「存在但行为未实证」项。浏览器观感面归 Phase 12/15（前端改动与 herdr UAT 所在 phase），非本 phase 契约。

### Gaps Summary

无缺口。前次 gaps_found 18/19 的唯二缺口均经代码级 + 行为级独立复验确认闭合：

1. **WR-01（Blocker）→ 闭合**：预检 main.go:1051-1063 满足全部三条 missing——(a) `filepath.Join(cfg.cwd, probe)` 与 spawn 侧 `cmd.Dir` 解析基准单点对齐（无双写）；(b) TestStartupMatrix 三命名行 + cwdCmdDir fixture 在树且逐 PASS，append-only 零删除行；(c) 带斜杠形态文案 `not executable` 与裸名形态 `not found in PATH` 分形态准确。进程级六形态矩阵本复验独立重跑全中，truth #5 由 FAILED 转 VERIFIED。
2. **WR-02（Warning）→ 闭合**：ValidateOptions 前移至分岔块尾部（:1328），pty.Start（:1334）与 listen 分岔（:1342）之前——V<P<L 数值断言成立；全文件单调用点（计数门 == 1）；最小字面量实参与 server.go:336-348 只读两字段语义等价；失败经 exit 2 通道且触发时零资源可回滚，「与 validateStartup 拒绝路径零资源占用同构」纪律恢复。

10-05 flagged_assumption 挂账项（PATH 含相对项 × 裸命令名的残余错位面，Go LookPath ErrDot 走拒绝通道——安全方向无误放）已显式登记归 Phase 11 attach 期 spawn 语义收口复核，非本 phase 缺口。Phase 11 前提（SC4 预检为唯一启动闸，语义已固化并经行为锁 + 进程级双向实证）就位。

---

_Verified: 2026-09-03T04:50:00Z_
_Verifier: Claude (gsd-verifier)_
