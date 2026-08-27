---
phase: 07-deployment
plan: 06
subsystem: infra
tags: [config-file, toml, go-toml, strict-mode, disallow-unknown-fields, two-phase-merge, precedence-chain, d-01..d-07, value-stripping, permission-warning, tdd]

requires:
  - phase: 07-deployment/07-01
    provides: --base-path flag 与 parse 期规范化段（配置键终值消费对象）
  - phase: 07-deployment/07-02
    provides: --socket 三 flag + portSet/bindSet/socketModeSet/socketOwnerSet 显式位 + validateStartup 互斥/单给矩阵（配置来源值同档生效的锚）
  - phase: 07-deployment/07-03
    provides: --auth-header flag（配置键终值消费对象）
  - phase: 07-deployment/07-04
    provides: --cwd/--term/--stop-signal/--stop-timeout/--uid/--gid flag 与 parse 期校验段（配置键终值消费对象）
  - phase: 07-deployment/07-05
    provides: --open flag（配置键终值消费对象——全部 26 个长期运行 flag 至此就位）
  - phase: 06-session-lifecycle/06-04
    provides: exitEmptyValue.Set 三形态单一解析路径（OQ4 配置字符串单形态复用）+ fs.Visit 显式设置位先例
  - phase: 03-auth
    provides: credErr/clientOptErr 记录式错误上报先例（配置敏感值校验同纪律）+ WESH_CREDENTIAL env 夹层块
provides:
  - "cmd/wesh/config.go【新】：fileConfig 27 键（26 flag 同名 + command，指针标量 + []string 列表）+ loadFileConfig（严格模式未知键拒绝 + DecodeError Key/Position 提取）+ configErr 值剥离包装（类别+键名+行号三要素）+ D-07 权限警告 + prescanConfigPath（--config 两形态预扫 last-wins）"
  - "cmd/wesh/main.go：--config flag + parseArgs 两阶段合并（预扫路径 → TOML 铺底 → flag 注册解析 → fs.Visit 合并）——标量默认值替换机制承载 flag > 配置 > 默认；配置键存在即「已给定」置 portSet/bindSet/socketModeSet/socketOwnerSet/writePolicySet；配置 exit-when-empty 在 --once 展开之前应用；列表 CLI 给出则替换整个列表（D-02），env 夹层先行（D-05）；argv/command 覆盖（D-04）"
  - "go.mod：github.com/pelletier/go-toml/v2 v2.4.3（本 phase 唯一新依赖，RESEARCH 三通道核实 Approved）"
  - "TestLoadFileConfig/TestPrescanConfigPath/TestConfigMerge/TestConfigPrecedence/TestConfigRedLines/TestParseArgsWithConfig（cmd/wesh/config_test.go，912 行——合并矩阵表驱动锁定 + 值剥离红线探针运行时自证）"
affects: [phase-07 后续 plans（07-07 phase07.mjs 配置文件合并场景 / 07-08 README 配置节与 chmod 600 建议、flagged_assumptions 人工复核）, phase-08, verify-work]

actuals:
  tokens: 18673
  tasks: 2
  commits: 4

tech-stack:
  added:
    - "github.com/pelletier/go-toml/v2 v2.4.3（TOML 配置文件解析——STACK.md 定案；Decoder 严格模式官方 API 兑现 D-06 未知键拒绝）"
  patterns:
    - "指针标量区分「键缺席」与「显式零值」：fileConfig 全部标量为指针——nil = 键缺席（内置默认档生效），非 nil = 已给定（port = 0 等显式零值不被吞）；两阶段合并正确性依赖 nil 判定"
    - "默认值替换机制承载优先级链：配置标量键换算为 flag 注册默认值——CLI 未给自然落配置值、CLI 给则覆盖、内置默认仅在配置键缺席时出现（flag > 配置 > 默认两档零新判定代码）"
    - "配置键存在即「已给定」：fc.Port/Bind/SocketMode/SocketOwner/WritePolicy 非 nil 即置对应显式位——07-02 互斥/单给矩阵与 write-policy×writable 组合校验对配置驱动与 CLI 驱动同档生效（校验矩阵语义不漂移）"
    - "值剥离错误包装：go-toml DecodeError.String()/Error() 带源行上下文会回显配置值（Pitfall 5）——绝不透传，只提取 Key()/Position() 组 detail 经 configErr 统一为「类别 + 键名 + 行号」；凭据值探针串测试运行时自证零出现"
    - "配置 exit-when-empty 先于 --once 展开应用：配置不算显式（exitEmptySet 不置位），--once 展开覆盖配置值（flag > 配置直接推论）；exitEmptyValue.Set 单一解析路径零双写（OQ4）"
    - "被遮蔽配置列表不解析不校验：CLI 给出（D-02 替换）或 env 非空（D-05 夹层）时配置 credential 列表完全不应用——「不应用」语义字面落地，惰性畸形值不误伤"

key-files:
  created:
    - cmd/wesh/config.go
    - cmd/wesh/config_test.go
  modified:
    - cmd/wesh/main.go
    - go.mod
    - go.sum

key-decisions:
  - "cfgCredErr/cfgClientOptErr 上报点落配置列表合并段末尾（env 块之后）而非字面「与 credErr 并列」——env/CLI 遮蔽的配置列表不解析不校验（D-02/D-05「不应用」语义字面落地）；仍在 showVersion 早退之后，纯信息路径不被配置校验阻断（03-04 先例保持）"
  - "配置 exit-when-empty 在 --once 展开之前应用（非之后）——之后应用会使配置值覆盖 --once 展开产物（违反 flag > 配置）；before 形态下配置不算显式，展开覆盖配置值，三源组合（CLI/配置/--once）行为全部收敛"
  - "duration 类配置键（ping-interval/stop-timeout/exit-when-empty）解析错误取不含值的更严形态（configErr 类别 + 键名）——plan 允许含值（非敏感），实现取统一值剥离简化红线审计面"
  - "DisallowUnknownFields 字面在 config.go 收敛为 1 处（调用行）——验收 grep ==1 是源码级机械检查，注释提及同样计数（05-08 登记纪律第四次沿用），注释改述为「严格模式」"
  - "parseArgs 头注释 29→30 flag 并补两阶段合并段（07-02/07-05 先例第三次沿用——同区域文档漂移随 flag 新增一次修正）"

patterns-established:
  - "TOML 配置键校验错误的 go-toml 行列提取形态：errors.As 先 StrictMissingError（逐 Errors[i].Key() 组键名清单）后 DecodeError（Key()/Position() 组 key+line），go-toml 错误文本永不进输出（Pitfall 5 防线）"
  - "配置驱动组合矛盾的测试形态：parseArgs(--config) 产 cfg → 直接喂 validateStartup 断言同文案拒绝（配置驱动与 CLI 驱动同矩阵语义的端到端证明，TestStartupMatrix 表外补充通道）"
  - "prescanConfigPath 预扫形态：手工扫描 --config=<v>/空格两形态 + 单横线变体（与 flag 包语义一致），last-wins，`--` 之后中断（子命令 argv 的 --config 属子命令绝不加载）"

requirements-completed: [OPS-09]

coverage:
  - id: D1
    description: "TOML 加载层：fileConfig 27 键（指针标量区分缺席/零值）+ 严格模式三族拒绝（文件不存在/解析失败/未知键——逃生门五键自然拒绝为未知键，D-04 边界）+ 值剥离红线（凭据探针串错误输出零出现）+ D-07 权限警告四态（600/400 静默，644+凭据警告且不含值，644 无凭据静默）+ prescan 两形态 last-wins"
    requirement: OPS-09
    verification:
      - kind: unit
        ref: "cmd/wesh/config_test.go#TestLoadFileConfig（12 子测）/#TestPrescanConfigPath（9 子测）全绿"
        status: pass
  - id: D2
    description: "两阶段合并矩阵：标量铺底/CLI 覆盖/显式零值/列表替换（D-02）/argv×command（含空数组缺席）/exit-when-empty 三形态（OQ4）/--once 覆盖配置/优先级链 flag > env > config > 默认/write-policy×writable 与 socket 族互斥单给矩阵配置同档（D-08/D-09 文案逐字同）"
    requirement: OPS-09
    verification:
      - kind: unit
        ref: "TestConfigMerge（23 子测）/TestConfigPrecedence（5 子测）/TestConfigRedLines（12 子测含 run() exit 2 端到端）经 TestParseArgsWithConfig 伞测试全绿；go test ./cmd/wesh -count=1 全绿（既有行零改动零回归）；go test -race -count=1 ./... 五包全绿（53.6s）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟四条全过：①port=0+bind+credential+command 配置启动（随机端口打印/401 Basic/bash 子进程/D-07 警告不含值）；②+CLI --max-clients 5 正常启动；③no-auth 写入 TOML → exit 2 且文案 unknown keys (no-auth) 不含 true 值；④裸 wesh -- bash → D-03 拒绝 exit 2 与今日逐字节一致"
        status: pass
    human_judgment: false

status: complete
---

# Phase 07 Plan 06: TOML 配置文件 Summary

**一句话：** `--config` 显式指定 TOML 配置文件全链落地——go-toml/v2 v2.4.3 严格模式（未知键拒绝）+ 27 键两阶段合并（flag > env > 配置 > 默认）+ 值剥离红线探针自证 + D-07 权限警告，裸启动零漂移。

**Requirement:** OPS-09（配置文件支持，CLI 参数覆盖配置文件——P5 deferred「新 flag 配置文件收口」兑现）

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 RED | 加载层失败测试 | ec84d25 | cmd/wesh/config_test.go（新，356 行） |
| 1 GREEN | TOML config file loader with strict mode and permission warning | 8a61b54 | cmd/wesh/config.go（新，167 行）、go.mod、go.sum |
| 2 RED | 合并矩阵失败测试 | 21d3e61 | cmd/wesh/config_test.go（+556 行） |
| 2 GREEN | two-phase config merge with flag > env > config > default precedence | ee7f516 | cmd/wesh/main.go（+266/-24） |

## What Was Built

**Task 1（加载层）：** `cmd/wesh/config.go` 新文件——`fileConfig` 27 键（26 flag 同名 + `command` exec 数组；标量全指针区分「键缺席」与「显式零值」）；`loadFileConfig` 经 go-toml `Decoder` 严格模式链式调用解码（未知键拒绝——no-auth/insecure-http/version/help/config 五逃生门键不在结构体，自然拒绝为未知键，D-04 边界）；错误统一 `configErr(path, category, detail)` 包装为「类别 + 键名 + 行号」三要素（go-toml 错误文本带源行上下文会回显值——只提取 `Key()`/`Position()`，Pitfall 5 红线）；D-07 权限检查（含 credential 键且非 600/400 → warn 串，不含凭据值）；`prescanConfigPath` 手工预扫两形态 last-wins（`--` 后中断）。依赖引入守 05-05 纪律（先落码含 import 再 `go get` + `go mod tidy`——v2.4.3 入库，`go mod verify` 全过）。

**Task 2（两阶段合并）：** parseArgs 装配链——预扫路径 → TOML 铺底 → 标量键换算 flag 注册默认值（flag > 配置 > 默认由默认值替换机制天然成立）→ `--config` 正式注册 → fs.Parse → fs.Visit → 配置键存在即「已给定」补置五个显式位（socket 族互斥/单给与 write-policy×writable 矩阵对配置来源值同档生效）→ 配置 exit-when-empty（`exitEmptyValue.Set` 单一解析路径，OQ4；在 --once 展开之前应用）→ --once 展开 → 既有全部 parse 期校验段消费终值（零新代码路径）→ env 夹层 → 配置列表合并（CLI 给出则替换整个列表 D-02；credential/client-option 错误记录式类别+键名禁含值；被遮蔽列表不解析不校验）→ argv/command 覆盖（空数组按缺席语义）。parseArgs 头注释 29→30 flag + 合并段。

## Verification

- `go test ./cmd/wesh -count=1` 全绿（新 61 子测 + 既有行零改动零回归——裸启动零漂移的结构性证明）
- `go test -race -count=1 ./...` 五包全绿（53.6s）
- `go build ./...` / `go vet ./cmd/wesh` / gofmt 零漂移（internal/server 两既有漂移文件未触碰，SCOPE BOUNDARY）
- `go mod verify` → all modules verified
- 验收锚点：`grep -c 'DisallowUnknownFields' cmd/wesh/config.go` == 1；`grep -c 'pelletier/go-toml' go.mod` == 1；`grep -c 'prescanConfigPath' cmd/wesh/main.go` == 3；config_test.go 912 行 ≥ 150
- 真实二进制冒烟（success_criteria 四条）：见 coverage D2 other 行

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] 验收 grep == 1 与注释提及的机械调和**
- **Found during:** Task 1 GREEN 自检
- **Issue:** `grep -c 'DisallowUnknownFields' cmd/wesh/config.go` 验收锚 == 1，初版注释三处提及该方法名（计数为 4）——05-08 登记纪律：验收 grep 是源码级机械检查，注释提及同样计数
- **Fix:** 注释改述为「严格模式」，方法名字面仅留调用行一处
- **Files modified:** cmd/wesh/config.go
- **Commit:** 8a61b54

**2. [按意图修正] cfgCredErr 上报点落合并段末尾（非字面「与 credErr 并列」）**
- **Found during:** Task 2 实现
- **Issue:** plan 行为文本「统一上报点与 credErr 并列插入 showVersion 早退之后」与同段「env 块之后新插入」存在位置张力——若按字面在 credErr 点上报，配置 credential 须在 env 块之前解析，则 env 遮蔽的惰性畸形配置会被误拒（违反 D-05「env 非空则配置 credential 不应用」）
- **Fix:** 上报点落配置列表合并段末尾（仍在 showVersion 早退之后，03-04 先例保持）；被 CLI/env 遮蔽的配置列表不解析不校验——「不应用」语义字面落地
- **Files modified:** cmd/wesh/main.go
- **Commit:** ee7f516

**3. [实现工具偏差] 并行 Edit 同文件丢失后串行重放**
- **Found during:** Task 2 GREEN 编译
- **Issue:** 14 个并行 Edit 调用同文件，部分注册行默认值替换丢失（编译报 declared and not used）
- **Fix:** 缺失 10 处串行逐个重放并 grep 核验——非计划语义偏差，产物与 plan 一致
- **Commit:** ee7f516

其余：plan  executed as written（TDD 门序完整——每 task test 提交先于 feat 提交）。

## Threat Flags

无新增威胁面——plan threat_model 登记的四项全部按 mitigate 落地：T-07-06a（D-07 权限警告 + env 优先于配置明文）、T-07-06b（configErr 值剥离 + 记录式扩展 + 探针断言运行时自证）、T-07-06c（严格模式未知键拒绝）、T-07-06d（仅 --config 显式指定，裸启动零路径搜索）、T-07-SC（go.sum 锁定 + go mod verify 过）。

## Known Stubs

无。

## TDD Gate Compliance

- RED 门：ec84d25（Task 1 test）/ 21d3e61（Task 2 test）——运行确认失败形态正确（undefined 符号 / `flag provided but not defined: -config`）
- GREEN 门：8a61b54（Task 1 feat）/ ee7f516（Task 2 feat）——全子测转绿
- 门序完整，无警告。

## Self-Check: PASSED

- 文件：cmd/wesh/config.go ✓ / cmd/wesh/config_test.go ✓ / 07-06-SUMMARY.md ✓
- 提交：ec84d25 ✓ / 8a61b54 ✓ / 21d3e61 ✓ / ee7f516 ✓
- 依赖：go.mod 含 pelletier/go-toml ✓；config.go 含严格模式 API 调用 ✓
