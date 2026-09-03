---
phase: 10-mode-assembly
verified: 2026-09-02T14:00:00Z
status: gaps_found
score: 18/19 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "per-client × argv0 可执行命令 → validateStartup 放行（10-02 must_have 第二分句；SC4 预检不误拦合法配置）"
    status: failed
    reason: "WR-01 进程级复现确认：LookPath 预检在服务端进程 cwd 下解析，而实际 spawn 在子进程 chdir(--cwd) 之后解析相对路径——`--session-mode=per-client --cwd=/tmp/wesh-v10/appdir -- ./run.sh`（run.sh 存在且可执行）被 exit 2 误拒（`invalid command \"./run.sh\": not found in PATH`），同参数 shared 正常 spawn（session_start 携 pid + listening on 实证）。带斜杠形态不经 PATH 解析，文案亦不准确。Phase 11 spawn 移至 attach 期后此预检是唯一启动闸，误拒将永久固化。"
    artifacts:
      - path: "cmd/wesh/main.go"
        issue: ":1043-1047 exec.LookPath(cfg.argv0) 不感知 --cwd；相对路径 argv0（含斜杠形态与 PATH 含 . 形态）与 spawn 侧 execve 解析基准发散"
    missing:
      - "预检与 spawn 语义对齐：argv0 含路径分隔符且非绝对路径时，改为对 --cwd join 后的 os.Stat 可执行探测（或最小修复：--cwd 非空且 argv0 带斜杠时跳过预检并注释登记缝隙）"
      - "补 --cwd×相对路径 argv0 的 TestStartupMatrix 放行面测试行"
      - "修正带斜杠形态的文案（not found in PATH → not executable 类准确语义）"
  - truth: "ValidateOptions 装配契约 fail-fast 零资源占用纪律（其注释自引「与 validateStartup 拒绝路径零资源占用同构」）"
    status: partial
    reason: "WR-02 代码实证：run() 中 ValidateOptions（main.go:1345）在 pty.Start（:1307 已 spawn 子进程）与 net.Listen/listenSocket（:1315-1322 已占用监听）之后调用，失败分支直接 return 2——无 sess.Close() 亦无 ln.Close()，与紧邻两个失败路径（listen/serve 失败均 _ = sess.Close()）不对称；--socket 形态下 socket 文件残留。当前不可达（parse 枚举闸+分岔逻辑保证恒一致），但守卫存在意义即拦截未来漂移，一旦触发清理路径即错。不 falsify 任何 phase 级 must_have（『fail-fast 在 New 之前经 exit 2 通道』已验证成立），列为后续修复缺口。"
    artifacts:
      - path: "cmd/wesh/main.go"
        issue: ":1342-1348 ValidateOptions 调用点在资源获取之后，失败路径零回滚"
    missing:
      - "将 ValidateOptions 前移至 10-01 分岔块尾部、pty.Start 之前（两输入 cfg.sessionMode/spawnFunc 在该点已完全确定）；或原位调用补 sess.Close()/socket unlink 回滚（推荐前移）"
---

# Phase 10: 模式装配与接缝 Verification Report

**Phase Goal:** 会话模式阀门与全部接缝一次装配到位（全部 inert）——`--session-mode` 公开契约锁定，默认 shared 逐字节零回归，per-client 分支挂点唯一化防散点 if/else 腐化
**Verified:** 2026-09-02T14:00:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1：`--session-mode=per-client`/显式 shared/缺省三形态启动被接受（listening on）；缺省 shared 下 v1.0 全量 Go 测试与既有协议 UAT 原样全绿、行为逐字节不变 | ✓ VERIFIED | 冒烟三形态均打印 `listening on`；`go test -race -count=1 ./...` 五包全 ok（59.4s）；web/uat/phase02-09.mjs 八脚本零修改（git status 零输出）原样重跑 exit 全 0（12/12、18/18、10/10、28/28、23/23、34/34、21/21、18/18） |
| 2 | SC2：非法模式值 CLI/TOML 双源 parse 期拒绝 exit 2，D-04 文案；凭据/token/文件内容红线保持 | ✓ VERIFIED | 进程级实证：CLI banana → exit 2 + `invalid --session-mode "banana": must be shared or per-client` 全文；TOML `session-mode = "banana"` → exit 2 同文案（一闸双覆盖）；`session_mode` 下划线 → exit 2 `unknown keys (session_mode)`；枚举回显属 D-04 既定豁免面 |
| 3 | SC3：优先级链 CLI flag > TOML > 内置默认 shared（env 层真空，D-03 不引入 WESH_SESSION_MODE） | ✓ VERIFIED | TestConfigPrecedence/session-mode precedence chain 三腿 PASS；全仓 grep `WESH_SESSION_MODE` 仅命中测试注释（env 真空实证）；缺省终值 shared 由 TestParseArgs 零值断言 + 冒烟锁定 |
| 4 | SC4（拒绝半）：per-client × argv0 缺失 → 启动期 fail-fast exit 2；shared × 缺失 → 不预检零漂移 | ✓ VERIFIED | 冒烟：`-- wesh-no-such-cmd-7f3a` per-client → exit 2 + `invalid command ... not found in PATH (per-client startup preflight)`；shared 同命令 → exit 1 经 pty.Start 现状通道（`executable file not found in $PATH`），非预检 exit 2 |
| 5 | 10-02（放行半）：per-client × 可执行命令 → 放行 | ✗ FAILED | **WR-01 复现**：`--cwd=/tmp/wesh-v10/appdir -- ./run.sh`（可执行存在）per-client → exit 2 误拒；shared 同参数 → session_start(pid) + listening on 正常。相对路径 argv0 类配置被结构性误拦（见 gaps） |
| 6 | run() 装配期一次分岔：shared 分支 SpawnFunc=nil；per-client 装配捕获 argv+StartOptions 的闭包（StartWithSize 直通）并透传 Options.SessionMode；两模式均经启动期 pty.Start（全部 inert） | ✓ VERIFIED | main.go:1298-1304 分岔块；:1307 pty.Start 现状行无条件执行逐字保持；:1342 Options 字面量尾部只追加两键；SpawnFunc 全仓零调用方（仅 ValidateOptions nil 检查 + server.go:446 结构体透传）；server 内 sessionMode 只存储零运行期分支 |
| 7 | ValidateOptions 互斥校验 fail-fast：per-client×nil SpawnFunc 拒绝、shared×非nil 拒绝、零值归一 shared | ✓ VERIFIED | server.go:336-348 两规则 + 零值归一；TestValidateOptions 五态全 PASS（含 zero_mode_normalizes_to_shared）；New :393 零值兜底同口径 |
| 8 | pty.Start ≡ StartWithSize(argv, opts, SpawnCols, SpawnRows) 单行委托；80×24 单一事实源；自定义尺寸真实到达 TIOCSWINSZ | ✓ VERIFIED | spawn.go:67-69 单行委托 + :76 StartWithSize 承载全逻辑；SpawnCols/SpawnRows 常量唯一（:38-41）；TestStartWithSizeDelegation（等价面 slices.Equal/Dir/SysProcAttr + 132x43 GetsizeFull 读回）PASS；TestStartZeroValueParity 原样 PASS |
| 9 | 零回归收口闸：v1.0 全量测试原样全绿、既有测试行零改动 | ✓ VERIFIED | -race 五包全绿；`git diff -U0 781af48..HEAD -- 四个测试文件` 删除行 == 0（append-only）；UAT 脚本零修改 |
| 10 | writePolicySet × per-client → warn 放行，文案含 --write-policy 与 --session-mode 双 flag 名（D-01/D-02 双源同档；静默永不接受） | ✓ VERIFIED | 冒烟：`--writable --write-policy=all --session-mode=per-client` → warn 行含双 flag 名 + listening on；TestStartupMatrix 四行（owner/all 双形态 + 两不触发面）PASS |
| 11 | warn 合并不遮蔽：新 warn 与非 loopback 安全警告同现；socket/loopback 早退透出；既有 warn 文案逐字未动 | ✓ VERIFIED | 冒烟：bind 0.0.0.0 + --no-auth + 组合 → stderr 两条 warning、三子串（--no-auth/--write-policy/--session-mode）齐全；mergeWarn 累积形态（main.go:989-1003）+ 五透出点逐一在码；TestValidateStartupWarnMerge 两形态 PASS；既有矩阵行零改动 |
| 12 | TOML session-mode 铺底生效 + sessionModeSet 置位 + CLI 覆盖 | ✓ VERIFIED | TestConfigMerge 两子测 PASS；config.go:86 `SessionMode *string toml:"session-mode"`；main.go:270-271 默认值换算 + :579-580 显式位置位 |
| 13 | TOML redlines：下划线未知键拒绝 / banana 与 CLI 同文案 / 类型不符 invalid toml 分支 | ✓ VERIFIED | TestConfigRedLines 三子测 PASS + 进程级冒烟双源 exit 2（见 #2） |
| 14 | FuzzDecodeFileConfig 五新种子随零时长回归门全过；值剥离红线不变量保持 | ✓ VERIFIED | fuzz_test.go:83-89 五种子逐注释登记；套件内 FuzzDecodeFileConfig PASS；30s 短跑复跑 3,011,062 execs PASS 零崩溃 |
| 15 | 精确匹配禁令：大小写变体/前后空白近形值一律 parse 期拒绝 | ✓ VERIFIED | 冒烟：`SHARED` → exit 2；`" shared"` → exit 2（枚举闸 :656 == 两常量精确匹配） |
| 16 | CONFIGURATION.md 五处落地 + 29→30 计数 + 「装配中」注记强制口径 | ✓ VERIFIED | :57 键表行、:101 显式位清单、:125 校验矩阵、:132 warn 清单、:154 默认值表；`共 30 键`/`全部 30 个配置键` 各 ×1；装配中注记 ×2 |
| 17 | README.md 一句最小明示；--help 与文档口径一致 | ✓ VERIFIED | README:96 四要素齐（flag 名/两枚举/默认 shared/装配中）；`wesh --help` 含 `session mode: shared|per-client (default shared; per-client is being assembled and currently behaves as shared)` |
| 18 | 静态面干净：gofmt 零输出、go vet 干净、go build 成功 | ✓ VERIFIED | gofmt -l 零输出；go vet ./... 无输出；time go build 0.57s |
| 19 | 冒烟三命令进程级证据（CLI banana / TOML banana / per-client 接受） | ✓ VERIFIED | 本验证独立复跑全部命中（见 #2/#4） |

**Score:** 18/19 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/wesh/main.go` | flag 注册 + 三字段 + 枚举闸 + run() 分岔 + ValidateOptions 调用 | ✓ VERIFIED | :91-93 字段、:353 注册、:530/:579 显式位、:656-657 枚举闸、:844 argv0、:1298-1304 分岔、:1345 ValidateOptions |
| `cmd/wesh/config.go` | fileConfig.SessionMode 第 30 键 | ✓ VERIFIED | :86 `toml:"session-mode"`；零枚举逻辑（一闸双覆盖结构） |
| `internal/server/clients.go` | SessionMode 枚举常量对 | ✓ VERIFIED | :88/:90 WritePolicy 常量块同位同形态 |
| `internal/server/server.go` | Options 两字段 + ValidateOptions + New 兜底 | ✓ VERIFIED | :297-309 字段+注释分档、:336-348 ValidateOptions、:393 兜底、:445-446 直传 |
| `internal/pty/spawn.go` | StartWithSize 导出 + Start 单行委托 | ✓ VERIFIED | :67-69/:76；80×24 零第二副本 |
| `cmd/wesh/main_test.go` | wantSessionMode 断言位 + 两 parse 行 + D-04 拒绝行 + 启动矩阵八行 + warn 合并测试 | ✓ VERIFIED | :197-198/:487/:803-815/:863；append-only（删除行 == 0） |
| `internal/server/options_test.go` | TestValidateOptions 表驱动（新文件） | ✓ VERIFIED | 五态全 PASS |
| `internal/pty/spawn_test.go` | TestStartWithSizeDelegation | ✓ VERIFIED | 两面 PASS |
| `cmd/wesh/config_test.go` | merge/precedence/redlines 六子测 | ✓ VERIFIED | 全 PASS，append-only |
| `cmd/wesh/fuzz_test.go` | 五新种子 | ✓ VERIFIED | :83-89，append-only |
| `docs/CONFIGURATION.md` | 五处 + 计数 | ✓ VERIFIED | 见 truth #16 |
| `README.md` | 一句明示 | ✓ VERIFIED | :96 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| parseArgs 枚举闸 | clients.go 常量 | 共用同一对常量（:656） | ✓ WIRED | 双写漂移结构性消除 |
| run() per-client 闭包 | pty.StartWithSize | 闭包体直通（:1302） | ✓ WIRED | 本阶段零调用方（有意 inert，非 stub——T-10-01c） |
| config.go SessionMode | Parse 返回处枚举闸 | 默认值替换机制落同一终值 | ✓ WIRED | TOML banana 进程级实证同文案（一闸双覆盖） |
| run() | ValidateOptions → New | New 前 fail-fast exit 2 | ✓ WIRED | :1345/:1349；但调用点在资源获取之后——WR-02 缺口（见 gaps #2） |
| validateStartup warn | writePolicySet × sessionMode | D-02 显式位锚定（:990） | ✓ WIRED | 冒烟+矩阵双证 |
| validateStartup LookPath | cfg.argv0 | SC4 预检（:1043-1047） | ⚠️ PARTIAL | 接线在但 --cwd 相对路径误拒——WR-01 缺口（见 gaps #1） |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| run() → server.New | SessionMode | parseArgs 双源合并终值 | ✓ CLI/TOML/默认三层实证 | ✓ FLOWING |
| run() → server.New | SpawnFunc | 分岔闭包（StartWithSize 直通） | ✓ inert 装配，ValidateOptions 消费其 nil 性 | ✓ FLOWING |
| validateStartup | argv0 | parseArgs argv[0] 落定（:844） | ✓ 冒烟三形态实证 | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CLI banana 拒绝 | `wesh --session-mode=banana -- bash` | exit 2 + D-04 全文 | ✓ PASS |
| TOML banana 拒绝 | `wesh --config bad.toml -- bash` | exit 2 同文案 | ✓ PASS |
| 下划线键拒绝 | `session_mode = "shared"` | exit 2 unknown keys | ✓ PASS |
| 近形值拒绝 | `SHARED` / `" shared"` | 均 exit 2 | ✓ PASS |
| per-client/shared/缺省启动 | 三形态 timeout 3 | 均 listening on | ✓ PASS |
| LookPath 拒绝/放行 | missing cmd / `sh` | exit 2 / 放行 | ✓ PASS |
| shared 不预检 | shared × missing cmd | exit 1 经 pty.Start（非 exit 2） | ✓ PASS |
| warn 双 flag 名 + 放行 | write-policy=all × per-client | warn + listening on | ✓ PASS |
| warn 合并不遮蔽 | + --no-auth 非 loopback | 2 条 warning、三子串齐 | ✓ PASS |
| **WR-01 复现** | per-client --cwd=appdir -- ./run.sh | **exit 2 误拒（shared 对照正常 spawn）** | ✗ FAIL |
| 命名测试矩阵 | go test 三包八具名测试 | 全 ok（ValidateOptions 五态逐 PASS） | ✓ PASS |
| 全量 -race | go test -race -count=1 ./... | 五包全 ok（59.4s） | ✓ PASS |
| UAT 八脚本 | node web/uat/phase0{2..9}.mjs | exit 全 0，PASS 计数与基线一致 | ✓ PASS |
| 30s fuzz | -fuzz FuzzDecodeFileConfig -fuzztime 30s | 3.01M execs PASS | ✓ PASS |

### Probe Execution

SKIPPED（本阶段无 probe 脚本约定——验证面为 Go 测试 + UAT + 冒烟，已在上节覆盖）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PC-01 | 10-01/10-02/10-03/10-04（四 plan 全数申领） | `--session-mode=shared\|per-client` flag（或 TOML 键）选择会话模式；缺省 shared，v1.0 全部行为逐字节不变 | ⚠️ PARTIAL | flag/TOML/默认三面与零回归双证据全部成立（truth #1-3, #9, #18-19）；唯 SC4 预检对 --cwd 相对路径 argv0 存在误拒缺陷（truth #5，WR-01）——requirement 主文面达成，派生行为缺口待闭合 |

孤儿需求检查：REQUIREMENTS.md 仅 PC-01 映射 Phase 10（:191），四 plan `requirements:` 字段全数申领 PC-01——无孤儿、无遗漏。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| cmd/wesh/main.go | :1043-1047 | LookPath 预检与 spawn 解析基准发散（--cwd 盲） | 🛑 Blocker | WR-01——per-client 下合法相对路径配置被启动期误拒，Phase 11 后永久固化 |
| cmd/wesh/main.go | :1345-1348 | ValidateOptions 调用点在 pty.Start/listen 之后且零回滚 | ⚠️ Warning | WR-02——当前不可达；守卫触发时子进程/socket 清理路径错误 |

（债务标记扫描：12 个 phase 文件 TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER 零命中。）

### Gaps Summary

Phase 10 的公开契约面（flag/TOML 键/枚举闸/优先级链/文档/help）与零回归双证据（全量 -race 五包绿 + 八 UAT 脚本原样全过）全部独立复跑实证成立；全部接缝 inert 纪律（SpawnFunc 零调用方、零运行期模式分支、sess 两模式非 nil）与四条 prohibitions（精确匹配/无 env 键/既有 warn 零改动/无第二校验行）逐项核实无破口。**唯二缺口来自 10-REVIEW 的两条 Warning，本次验证均已独立复现/确认**：WR-01 是本 phase 交付的 SC4 预检对 `--cwd` × 相对路径 argv0 组合的行为性误拒（进程级实证，10-02 must_have「可执行命令 → 放行」分句被证伪）——属 Phase 10 自身交付缺陷，且 Phase 11 spawn 移至 attach 期后该预检将成为唯一启动闸而永久固化，不可 deferred；WR-02 不 falsify 任何 must_have（「fail-fast 在 New 之前」已验证），但违反 ValidateOptions 自引的零资源占用纪律，列为同批闭合的后续修复缺口。两缺口均未在 Phase 11-15 的 roadmap 目标/成功准则中被覆盖，不适用 deferred。

---

_Verified: 2026-09-02T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
