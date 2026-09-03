---
phase: 10-mode-assembly
plan: 04
subsystem: docs
tags: [documentation, configuration, regression-gate, uat, smoke]

requires:
  - phase: 10-mode-assembly plan 01/02/03
    provides: session-mode 阀门全装配（flag/TOML/枚举闸/Options 接缝/run() 分岔/StartWithSize）、validateStartup 双行、TOML 三面证据
provides:
  - CONFIGURATION.md session-mode 五处落地（键表行/显式位清单/校验矩阵枚举行/warn 清单行/默认值行）+ 键计数 29→30
  - README.md session-mode 一句最小明示
  - Phase 10 收口验证证据（六段全 PASS——零回归双证据成立）
affects: [phase-11 生命周期主干, phase-15 标定/UAT（PC-12 完整语义段）]

actuals:
  tokens: 1400
  tasks: 2
  commits: 1

tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - docs/CONFIGURATION.md
    - README.md

key-decisions:
  - "D-05 最小明示口径三处同文：「per-client 行为装配中，当前版本与 shared 等价」（键表/默认值表/README；完整语义段归 Phase 14 PC-12）"

patterns-established: []

requirements-completed: [PC-01]

coverage:
  - id: D1
    description: "CONFIGURATION.md 五处 + 29→30 计数 + README 一句 + --help 口径一致（D-05）"
    requirement: PC-01
    verification:
      - kind: other
        ref: "grep 计数：CONFIGURATION session-mode×5 / 共 30 键×1 / 全部 30 个配置键×1 / 装配中×2；README×1；--help session-mode 行"
        status: pass
    human_judgment: false
  - id: D2
    description: "零回归双证据：全量 -race 五包全绿 + 既有协议 UAT 八脚本原样重跑全过（脚本零修改）"
    requirement: PC-01
    verification:
      - kind: unit
        ref: "go test -race -count=1 ./...（5 包 ok，59.9s）"
        status: pass
      - kind: e2e
        ref: "web/uat/phase02-09.mjs 八脚本 exit 全 0（18/18、21/21 等 PASS 计数与 v1.0 基线一致）"
        status: pass
    human_judgment: false
  - id: D3
    description: "冒烟三命令：CLI/TOML banana 双源 exit 2 同文案；per-client 与缺省两形态 listening on"
    requirement: PC-01
    verification:
      - kind: e2e
        ref: "/tmp/wesh-uat/wesh 三命令直证（见 Verification 节摘录）"
        status: pass
    human_judgment: false

duration: 25 min
completed: 2026-09-02
status: complete
---

# Phase 10 Plan 04: 文档收口与全量验证闸 Summary

**session-mode 文档五处 + README 一句落地（D-05 最小明示，--help 口径一致）；收口闸六段全 PASS——Go 全量 -race 绿 + 既有八 UAT 脚本原样全绿的零回归双证据成立，SpawnFunc 零调用方 inert 证明闭合。**

## Performance

- **Duration:** 25 min（2026-09-02T13:16Z → 13:41Z）
- **Tasks:** 2/2
- **Commits:** 1（Task 2 为验证任务零提交）
- **Files:** 2 modified（纯文档）

## Accomplishments

- CONFIGURATION.md：键表 `session-mode` 行（write-policy 行后）、显式位清单追加、校验矩阵值域/枚举行追加 `--session-mode`、「放行但警告」清单追加 write-policy×per-client 行、默认值表行；键计数 29→30 两处；三处「装配中，当前版本与 shared 等价」强制口径
- README.md 可写协作示例块后一句最小明示（flag 名/两枚举/默认 shared/装配中注记四要素齐）
- --help 核对：`session mode: shared|per-client (default shared; per-client is being assembled and currently behaves as shared)`——枚举名单/默认值/装配中注记与文档同一事实叙述，无需回改
- 收口全量验证六段证据（下方 Verification 节）+ SpawnFunc inert 证明：生产代码仅 ValidateOptions 校验读取与 server.go:446 结构体透传，零调用点

## Task Commits

1. `d8b25b2` — docs(10-04): session-mode minimal documentation — CONFIGURATION.md rows + README note (PC-01, D-05)

## Verification

| 段 | 命令 | 结果 |
|----|------|------|
| ① 静态面 | `gofmt -l .` = 0；`go vet ./...` 干净；`time go build ./...` OK（0.588s） | PASS |
| ② Go 全量 | `go test -race -count=1 ./...` 五包全 ok（59.9s；FuzzDecodeFileConfig 十种子随套件过） | PASS |
| ③ UAT 八脚本 | `node web/uat/phase0{2..9}.mjs` 退出码全 0；PASS 计数与 v1.0 基线一致（如 phase03 18/18、phase08 21/21）；无 FAIL 断言行（grep -i 命中均为 "fail-fast" 文案词） | PASS |
| ④ 冒烟 CLI | `wesh --session-mode=banana -- bash` → exit 2 + `invalid --session-mode "banana": must be shared or per-client` 全文 | PASS |
| ④ 冒烟 TOML | `session-mode = "banana"` → exit 2 同文案（一闸双覆盖进程级证据） | PASS |
| ④ 冒烟接受面 | `--session-mode=per-client` 与缺省两形态均 `listening on`（SC1） | PASS |

**既有套件零修改声明**：`git status --short web/uat/` 零输出；本 phase 全部测试文件改动均为 append-only（10-01/10-02/10-03 各 plan 的 `git diff -U0` 删除行 == 0 证据链）；工作区除预存在的 web/dist+pnpm 构建产物外干净。

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Authentication Gates

None.

## User Setup

None.

## Next Step

Phase complete, ready for next step（phase 级 verify-work / Phase 11 生命周期主干规划）。

## Self-Check: PASSED
