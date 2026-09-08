---
phase: 13-resource-defense
plan: 01
subsystem: cli-assembly
tags: [stop-timeout, per-client, dual-defaults, explicit-set-bit, leak-defense, one-way-contract]
requires:
  - "Phase 11 D-01 teardown KILL 兜底机制（teardownPCLocked stopTimeout>0 AfterFunc SIGKILL——本 plan 仅改默认值来源，机制零改动）"
  - "Phase 10 显式位先例（fs.Visit + TOML 键存在双源置位，07-06 合并收尾形态）"
provides:
  - "per-client 模式 --stop-timeout 双默认值：未显式设置默认 5s（HUP 免疫泄漏防线默认开启）"
  - "cfg.stopTimeoutSet 显式位（CLI/TOML 双源置位）——「未设置」vs「显式 0」区分机制"
  - "validateStartup per-client × 显式 0 泄漏风险 warn（modeWarns 累积通道）"
  - "resolveStopTimeout 纯函数（run() Options 装配前终值落定单点，可直调）"
affects:
  - "internal/server/perclient.go teardownPCLocked 既有 KILL 兜底分支（消费点——cfg.stopTimeout → Options.StopTimeout，本 plan 零改动）"
tech-stack:
  added: []
  patterns:
    - "显式设置位双源置位第八位（fs.Visit + fc 键非 nil，07-06 先例第 N 次同构）"
    - "纯函数提取可测性（resolveStopTimeout——loadCustomIndex 同位纪律）"
key-files:
  created: []
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - cmd/wesh/config_test.go
    - docs/CONFIGURATION.md
    - README.md
decisions:
  - "D-01 one-way 门 option-a 用户派发确认：per-client 未显式设置默认 5s（12-01 D-08 one-way 门先例同形态）；shared 字面 0 逐字不动（v1.0 零回归红线）"
  - "终值落定提取为 resolveStopTimeout 纯函数（Rule 3 可测性——plan action「终值落定直调」的直调点，行为零变化）"
  - "TestValidateStartupWarnMerge 加第四枚负例（per-client 未设无 warn——锚定显式位而非终值的过宽实现必翻车，Rule 2 判别力补强）"
metrics:
  duration: 30min
  completed: 2026-09-05
  tasks: 3
  commits: 2
status: complete
actuals:
  tokens: 5236
  tasks: 3
  commits: 2
---

# Phase 13 Plan 01: stop-timeout 双默认值与显式位（D-01/D-02）Summary

per-client 模式下 `--stop-timeout` 双默认值落地：未显式设置默认 5s（HUP 免疫泄漏防线默认开启）+ 显式 0 经 stopTimeoutSet 显式位尊重并 warn 泄漏风险；shared 字面 0 逐字不动。

## What Was Built

**Task 1（checkpoint:decision，D-01 one-way 确认门）**：用户派发确认 option-a——per-client 双默认值 5s + 显式 0 尊重+warn（D-01/D-02 既定方案）。实现前确认门按项目先例（05-03/06-04/09-04/12-01）在 Task 2 实现前通过；决策已登记 STATE.md。

**Task 2（feat 27909f8）**：机制四点 + 文档双行。
- `cfg.stopTimeoutSet bool` 显式位字段（writePolicySet 等七先例同位同形态）
- fs.Visit 块第八位（`f.Name == "stop-timeout"` 置位）+ 配置显式位块 `fc.StopTimeout != nil` 置位——双源同档，TOML `stop-timeout = "0s"` 与 CLI `--stop-timeout=0` 等价置位
- run() Options 装配前终值落定：per-client 且未置位 → `cfg.stopTimeout = 5s`（PITFALLS P8 取值；论证注释载 D-01 双默认值边界）；shared 路径 parseArgs `stopTimeoutDefault := time.Duration(0)` 字面 0 逐字未动
- validateStartup modeWarns 累积通道加 per-client × 显式 0 泄漏 warn 分支（定值文案零插值，:985 红线保持；mergeWarn 既有透出点零改动）
- docs/CONFIGURATION.md TOML 键表（:77）+ 默认值表（:160）双行、README.md flag 语义行同步为双默认值语义

**Task 3（test bfe6291）**：三态断言组收口。
- `TestStopTimeoutResolution` 表驱动两通道：CLI 参数切片经 parseArgs 全链（显式位置位 + 解析产出 + 终值三断言）+ config 直构经 resolveStopTimeout 边界——8 行锁定三态：shared 未设=0（红线）/ per-client 未设=5s（D-01 核心）/ per-client 显式 0=0+置位（D-02 核心）/ per-client 显式 3s 保持 / shared 显式 0 保持 + 直构边界三行
- `TestValidateStartupWarnMerge` 四枚 t.Run：per-client 显式 0 → warn 含 `--stop-timeout=0` 与 `--session-mode=per-client`；per-client 显式 5s / shared 显式 0 / per-client 未设 → 均无该子串
- `TestConfigMerge` 两枚 t.Run：TOML `stop-timeout = "0s"` 键存在 → 置位且 per-client 终值 0；键缺席 → 不置位且终值 5s（TOML 源的 D-01/D-02 双证据）

## Commits

| Task | Commit | Type | Summary |
|------|--------|------|---------|
| 2 | 27909f8 | feat | 显式位第八位 + 双默认值终值落定 + warn 分支 + 双文档行 |
| 3 | bfe6291 | test | 三态断言组 + warn 四分支 + TOML 置位 + resolveStopTimeout 提取 |

## Verification Results

- `go build ./...` + `go vet ./...` + `gofmt -l cmd/wesh/` 零输出（GOROOT go1.26.3 收口闸工具，10-05 先例）
- `time go test ./cmd/wesh/ -count=1` 全绿（既有测试断言零改动——test 文件 diff 纯新增 0 删除行，为 shared 零回归证据）
- `time go test ./... -count=1` 全仓 5 包全绿（cmd/wesh + proto + pty + server 75.4s + web）
- 定向组 `TestStopTimeout|TestValidateStartupWarnMerge` 全绿（14 子测）
- acceptance grep 锚定逐条通过：字面 0 恰一命中（:245，struct 加行后自 :244 漂移一行，内容逐字不动）；三置位点、终值落定行（:1429 位于 Options 装配 :1438 之前）、warn 分支（:1030）、文档 5s 语义（两文件）各就位

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - 可测性提取] run() 内联覆写提取为 resolveStopTimeout 纯函数**
- **Found during:** Task 3
- **Issue:** Task 3 behavior 要求断言「per-client 未设 → 终值 5s」且 action 1 明示「终值落定直调」，但 Task 2 落地形态为 run() 内联 if——run() 不可直调，plan 要求的核心断言无直调点
- **Fix:** 提取为包级纯函数 `resolveStopTimeout(cfg config) config`（loadCustomIndex 同位纪律，run() 上方纯 helper 区），run() 消费点改为 `cfg = resolveStopTimeout(cfg)` 单行调用；行为零变化（同一判定逻辑，Options.StopTimeout 终值单点不变），并入 Task 3 test 提交（11-01/11-03/12-03 两提交先例保持——feat + test）
- **Files modified:** cmd/wesh/main.go
- **Commit:** bfe6291

**2. [Rule 2 - 判别力补强] TestValidateStartupWarnMerge 加第四枚负例（plan 三枚之外）**
- **Found during:** Task 3
- **Issue:** plan 指定三枚 warn 分支（显式 0 有 / 显式 5s 无 / shared 显式 0 无）测不住「warn 判定锚定显式位而非终值」的过宽实现——`sessionMode==per-client && stopTimeout==0`（不查 stopTimeoutSet）的错误实现三枚全过，但「per-client 未设」形态（stopTimeout 同为 0）会错误 warn
- **Fix:** 加第四枚 t.Run「per-client unset stop-timeout no leak warn」——未设置态无该 warn，过宽实现必翻车；与 must_haves truth「显式设置位区分『未设置』vs『显式 0』」的判别面直接对应
- **Files modified:** cmd/wesh/main_test.go
- **Commit:** bfe6291

### Plan 措辞与实证的微小出入（不构成偏差）

- acceptance criteria「run() 函数体内 5 * time.Second 恰一命中」：run() 内实际两处（终值落定行 + 既有 `ReadHeaderTimeout: 5 * time.Second`）；以 `cfg.stopTimeout = 5 * time.Second` 精确锚定恰一命中且位于 Options 装配之前——判定意图满足
- acceptance criteria「main.go:244 字面 0」：struct 加 stopTimeoutSet 字段后行号漂移至 :245，内容逐字未动

## Requirements Trace

PC-08 勾选**不随本 plan 执行**（先例 11-01/12-01：ID 跨 plan 共享——PC-08 双令牌桶本体属 13-02/13-03 范围，本 plan 是其默认值前提片；勾选归 phase 末收口 plan）。

## Known Stubs

None——全链无桩：CLI/TOML 双源显式位、双默认值终值、warn 通道、文档行均为真实实现，测试组为真实行为断言。

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-01 (DoS: per-client 默认 0 泄漏) | mitigate（本 plan 承载半面） | D-01 默认 5s 落地 + TestStopTimeoutResolution「per-client unset resolves to 5s」行锁定（进程级 trap '' HUP 实证归 13-07 S2 既定） |
| T-13-02 (Tampering: 显式位误判) | mitigate | 双源置位（fs.Visit 第八位 + fc.StopTimeout 非 nil）+ TestStopTimeoutResolution CLI/直构通道 + TestConfigMerge TOML 双断言 + 第四枚 warn 负例 |
| T-13-03 (Info Disclosure: warn 含敏感值) | accept | warn 文案定值零插值（:985 红线注释保持；TestStartupMatrix 既有凭据探针断言覆盖） |
| T-13-SC (供应链) | mitigate | 零新依赖——go.mod/go.sum 零 diff |

## Self-Check: PASSED

- 文件存在：cmd/wesh/main.go（含 stopTimeoutSet/resolveStopTimeout/warn 分支）、cmd/wesh/main_test.go（TestStopTimeoutResolution）、cmd/wesh/config_test.go（TOML 两 t.Run）、docs/CONFIGURATION.md（双行 5s 语义）、README.md（5s 语义行）——全部 FOUND
- 提交存在：27909f8（feat）、bfe6291（test）——git log 确认 FOUND
