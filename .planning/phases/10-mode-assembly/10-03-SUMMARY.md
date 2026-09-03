---
phase: 10-mode-assembly
plan: 03
subsystem: testing
tags: [go, toml, config, fuzz, table-driven-tests]

requires:
  - phase: 10-mode-assembly plan 01
    provides: fileConfig.SessionMode 注册（DisallowUnknownFields 白名单）、fc 换算块 + sessionModeSet 显式位置位、parseArgs 枚举闸（一闸双覆盖单写口）
provides:
  - TestConfigMerge 追加两子测（TOML 铺底 + sessionModeSet 置位 / CLI 覆盖）
  - TestConfigPrecedence 追加 session-mode 三层链子测（flag > TOML > 内置默认，env 真空）
  - TestConfigRedLines 追加三子测（下划线未知键 / banana 与 CLI 同文案 / 类型不符 invalid toml 分支）
  - FuzzDecodeFileConfig 五新种子（合法×2/非法枚举/下划线/类型不符）
affects: [10-04 文档收口, phase-13 资源防线配置面]

actuals:
  tokens: 2600
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "一闸双覆盖行为锁：TOML 非法枚举与 CLI 同文案全文断言（零第二校验行的结构证据）"

key-files:
  created: []
  modified:
    - cmd/wesh/config_test.go
    - cmd/wesh/fuzz_test.go

key-decisions:
  - "session-mode 显式位对配置来源同档置位经子测锁定（fc.SessionMode 非 nil → sessionModeSet=true——D-02 warn 锚定机制的配置侧证据）"

patterns-established:
  - "类型不符断言限键名与类别（invalid toml + key \"session-mode\"），值非敏感不禁言——exit-when-empty bool 形态子测同纪律"

requirements-completed: [PC-01]

coverage:
  - id: D1
    description: "TOML session-mode 铺底生效 + sessionModeSet 置位 + CLI 覆盖"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/config_test.go#TestConfigMerge 两子测"
        status: pass
    human_judgment: false
  - id: D2
    description: "优先级链 flag > TOML > 内置默认 shared（env 真空）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/config_test.go#TestConfigPrecedence/session-mode precedence chain（三腿）"
        status: pass
    human_judgment: false
  - id: D3
    description: "redlines：下划线键未知键拒绝 / banana 与 CLI 同文案 / 类型不符 invalid toml 分支"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "cmd/wesh/config_test.go#TestConfigRedLines 三子测"
        status: pass
      - kind: e2e
        ref: "冒烟：TOML session_mode 与 banana 各 exit 2（未知键 / 同文案）"
        status: pass
    human_judgment: false
  - id: D4
    description: "FuzzDecodeFileConfig 五新种子（零时长回归门 + 30s 短跑零崩溃零红线破口）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "go test ./cmd/wesh -run FuzzDecodeFileConfig$（十种子全过）"
        status: pass
      - kind: other
        ref: "go test -fuzz FuzzDecodeFileConfig -fuzztime 30s（3.04M execs PASS）"
        status: pass
    human_judgment: false

duration: 18 min
completed: 2026-09-02
status: complete
---

# Phase 10 Plan 03: session-mode TOML 三面证据与 fuzz 语料 Summary

**TOML 侧 merge 铺底/CLI 覆盖/三层优先级链/下划线拒绝/非法枚举同文案/类型不符六面全部经表驱动锁定（append-only），fuzz 五种子落地且 30s 短跑 3.04M execs 零崩溃零红线破口。**

## Performance

- **Duration:** 18 min（2026-09-02T12:58Z → 13:16Z）
- **Tasks:** 2/2
- **Commits:** 2
- **Files:** 2 modified（append-only，零删除行）

## Accomplishments

- TestConfigMerge 追加 `session-mode scalar from config only`（铺底 + **sessionModeSet 置位**——D-02 显式位机制配置来源同档证据）与 `session-mode CLI overrides config`（flag > 配置）
- TestConfigPrecedence 追加 `session-mode precedence chain` 三腿合一（flag > TOML / TOML > 内置默认 / 键缺席 → shared 且 sessionModeSet=false；D-03 env 层真空成立不断言）
- TestConfigRedLines 追加三子测：`session_mode` 下划线未知键拒绝（unknown keys + 键名）；`session-mode = "banana"` 与 CLI 同文案全文断言（`invalid --session-mode "banana": must be shared or per-client`——一闸双覆盖锁）；`session-mode = 1` 经 invalid toml 分支拒绝（键名入文案、不经枚举闸）
- FuzzDecodeFileConfig 追加五种子（合法 shared/per-client、banana 非法枚举、下划线键、类型不符），逐种子注释登记断言面归属；文件头两不变量与 stripKeyNameEcho 豁免面零改动

## Task Commits

1. `b1aaa42` — test(10-03): session-mode TOML coverage — merge/precedence/redlines (PC-01, D-03/D-04)
2. `eddcc35` — test(10-03): extend FuzzDecodeFileConfig seeds for session-mode key (PC-01, Pitfall 11)

## Verification

- `go test ./cmd/wesh -run 'TestConfigMerge$|TestConfigPrecedence$|TestConfigRedLines$' -count=1` PASS（六新子测）
- `go test ./cmd/wesh -run 'FuzzDecodeFileConfig$' -count=1` PASS（十种子零时长回归门）
- `go test ./cmd/wesh -fuzz FuzzDecodeFileConfig -fuzztime 30s -count=1`：3,038,140 execs，PASS 零崩溃零红线破口
- `go test ./cmd/wesh -count=1` 全包 PASS；`gofmt -l` 零输出
- acceptance grep：config_test.go `session-mode` ×15（≥6）、`session_mode` ×5（≥1）、`invalid --session-mode` ×1（≥1）；fuzz_test.go `session-mode` ×5（≥4）、`session_mode` ×1（≥1）；两文件 `git diff -U0` 删除行 == 0（append-only）
- TOML 冒烟（success criteria SC2 双源证据）：`session_mode = "shared"` → exit 2 `unknown keys (session_mode)`；`session-mode = "banana"` → exit 2 与 CLI 同文案

## Deviations from Plan

None - plan executed exactly as written.（一处机械适配：config_test.go 新增 server import——plan 注记「server 包已 import」与实际不符，常量引用所需，零行为变更。）

## Issues Encountered

None.

## Authentication Gates

None.

## User Setup

None.

## Next Step

Ready for 10-04（Phase 10 收口：D-05 文档最小明示 + 每阶段收口闸全量验证）——wave 3，depends 10-02/10-03 均已收口。

## Self-Check: PASSED
