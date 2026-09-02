---
phase: 08-observability
verified: 2026-08-28T12:41:00Z
status: passed
score: 28/28 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 24/24
  gaps_closed:

    - "G-08-2"
  gaps_remaining: []
  regressions: []
human_verification:

  - test: "A2 真实 journald 复测（G-08-2 修复后）：systemd 用户级 unit（wesh-uat.service，A2 既存事件可回溯）下以 sg systemd-journal 代跑 README 两则新示例（含 grep '^\\{' 防护段）——journalctl -u wesh -o cat | grep '^\\{' | jq -c 'select(.event==\"auth_failed\")' 与 'select(.client_id==N)'"
    expected: "jq 零 parse error（横幅行被防护段滤除）；检出既存 auth_failed 事件行（无 user/username 键）与同一 client_id 的 attach/detach 对（reason=normal）"
    why_human: "依赖真实 systemd/journald 检索面；机械化等价面已绿——phase08-journal.mjs 6/6（合流模拟上 README 逐字管道端到端 + 负对照自证夹具不空转）。处置与 08-06-PLAN verification 第 4 条 / 08-06-SUMMARY coverage D3（human_judgment: true，非阻塞 UAT resume 项）一致；A2 blocked 点（journal 读权限）已随用户加 systemd-journal 组解除，事件无需重制"
---

# Phase 8: 可观测性 Verification Report

**Phase Goal:** ttyd 缺失的可运维性补齐——健康检查（OPS-06 /healthz）、指标（OPS-07 /metrics）、JSON 结构化审计日志（OPS-08）
**Verified:** 2026-08-28T12:41:00Z
**Status:** human_needed
**Re-verification:** 是——gap closure 08-06（G-08-2）完成后的复验。前件（2026-08-28T06:27:12Z，human_needed 24/24，无 gaps 区；G-08-2 登记于 08-UAT.md）结论未被采信，本轮全部证据由本验证者独立重执行：**gap 闭合项全量三级验证（存在/实质/接线+行为）+ 已通过项快速回归（指纹 grep + 全量套件 + UAT 复跑）**。

## Goal Achievement

### Observable Truths

ROADMAP 三条 Success Criteria（SC1/SC2/SC3）为合同主线。前件 24 条真理本轮以快速回归核验（证据列标注「回归」）；08-06 gap closure 新增 4 条真理（T25-T28）按全量三级验证（标注「本轮实测」）。

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | /healthz 返回服务健康状态，可用于反代/编排探活 | ✓ VERIFIED | 回归：server.go:538 `"GET /healthz"` 注册在场；全量套件 -race 五包全 ok（含 TestHealthz 族）；phase08.mjs 复跑 21/21（S1 四组 + S4 draining） |
| SC2 | /metrics 暴露连接数、会话数、收发字节数、每客户端 outbox 深度与踢出计数 | ✓ VERIFIED | 回归：server.go:486（basicAuth 包装）/527（无认证直通）双注册在场；phase08.mjs S2/S3（认证闸两态 + 17 series + 数值）复跑 PASS；全量套件 -race 全 ok |
| SC3 | 日志为 JSON 结构化输出（slog），审计事件可检索；无凭据；用户可控字段剥离控制字符 | ✓ VERIFIED | 回归：`func logEvent` 仅 log.go 一处（grep==1）、server.go Fprintf==0、proxy.go:126 sanitizeRemoteUser(p.clientIP) 在场；phase08.mjs S5/S6 复跑 PASS；本轮新增 phase08-journal.mjs 6/6（J3 auth_failed 零 user/username 键实测） |
| T1-T24 | 前件 24 条 plan must_have 真理（健康检查四字段/draining 503/bp 固定/405/exposition 规范/17 series/零身份 label/版本通道/事件目录/单出口迁移/红线等） | ✓ VERIFIED | 回归：Go 源码自前件验证点（0ae4c0f）至 HEAD 零改动（`git diff 0ae4c0f..HEAD -- '*.go'` 为空）——前件行为证据（独立冒烟 + 命名测试 -race）结构性保持有效；本轮独立复跑 `go test -race -count=1 ./...` 五包全 ok（server 58.0s）、phase08.mjs 21/21 确认零漂移 |
| T25 | **（G-08-2 truth）**README 两则 journald jq 示例在 systemd 默认配置（StandardOutput=journal）下复制粘贴即可运行：合流流上 jq 零 parse error、检出预期事件行 | ✓ VERIFIED | 本轮实测：phase08-journal.mjs 全新构建二进制（HEAD）复跑 6/6——J2 全流纯度（防护后 jq . 退出 0、stderr 空、`{` 起始行恰 4 防 vacuous）、J3 示例一 auth_failed 恰 1 行零 user/username 键、J4 示例二 attach+detach 恰 2 行 client_id 均 1 reason=normal；真实 journald 检索面确认为残余人工项（见 Human Verification，处置与前文一致） |
| T26 | README「结构化日志」节如实陈述 journald 合流机理（stdout 横幅 + stderr JSON 事件），grep 防护带一句理由，文案风格与既有 README 一致 | ✓ VERIFIED | 本轮通读 README:459：引言句完整陈述合流机理（systemd 默认 StandardOutput=journal 合流 → jq 遇非 JSON 行中止 → 统一 `grep '^\{'` 预滤，与 parseEvents 滤行约定同款），中文精炼带理由，风格一致 |
| T27 | wesh 源码与启动输出零改动——横幅保持 stdout 人读文本（D-14）、运行期事件恒 stderr JSON（D-15）、警告行保持文本（D-16） | ✓ VERIFIED | 本轮实测：提交边界逐核——f19811b 仅 README.md（5+/5-）、9ebbbe4 仅新增 phase08-journal.mjs（342+）、d8f0fd8/fe2afa1 仅 .planning 文档；`git diff 0ae4c0f..HEAD -- '*.go'` 为空；工作区干净（git status --porcelain 空）；夹具 J0/J1 反向实证启动横幅仍为 stdout 人读文本（负对照 parse error 恰由横幅行触发） |
| T28 | 合流模拟夹具自证不空转：未防护管道在合流流上必 parse error（负对照），防护后两则示例管道与全流纯度断言全绿 | ✓ VERIFIED | 本轮实测：J1 负对照 exit=4 且 stderr 含 parse error（jq 1.6 行为与 A2 实机记录一致——夹具确复现 G-08-2 原缺陷形态）；J2/J3/J4 同跑全绿 |

**Score:** 28/28 truths verified（0 present-but-behavior-unverified——gap 闭合行为面有夹具行为证据，残余真实 journald 检索面按既定处置路由人工复核）

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `README.md` | 两则 journald 示例均含 `grep '^\{'` 防护段 + 合流机理说明句；:336 文本事件行 JSON 化 | ✓ VERIFIED | plan automated 断言组本轮全过：防护段 ≥2（:463/:465 实测）、无未防护 journalctl 行、`^wesh: ` 文本事件行清零、`select(.client_id==7)` 字面量保持；:336 现为 JSON detach 行（字段序同 emitDetachLocked：time/level/msg→event→remote→client_id→code→reason→remote_user，围栏 json） |
| `web/uat/phase08-journal.mjs` | G-08-2 回归夹具：合流模拟 + 四组断言（负对照/全流纯度/示例一/示例二） | ✓ VERIFIED | 342 行实质实现（通读）：spawn 真实二进制 stdio 分离捕获、'close' 事件收口、stdout+stderr 按 journal 时序拼接经 stdin 喂入 /bin/sh -c 管道；骨架件（startWesh/waitEvent/dialHello/check/assertOutputClean）与 phase08.mjs 同形；jq 缺失 skipped 兜底在场（:320-325）；本轮复跑 6/6 |
| 前件 15 项 artifacts（log.go/health.go/metrics.go/server.go/clients.go/auth.go/proxy.go/main.go/测试族/phase08.mjs/README 运维节/08-UAT.md） | — | ✓ VERIFIED（回归） | Go 面零改动 + 全量套件 -race 五包 ok + phase08.mjs 21/21 复跑——前件三级验证结论保持有效 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| README 示例 grep+jq 段 | phase08-journal.mjs 断言管道 | 逐字一致（范围=管道形态+防护段+引号形态；select 字面量豁免） | ✓ WIRED | 本轮比对：README `grep '^\{'` 段 ≡ 脚本 GREP_GUARD（:213）；PIPE_EX1 与示例一 jq 段逐字同形；PIPE_EX2 `==1` vs README `==7` 为 plan 明示豁免（夹具确定性 vs 示例 N），防漂移注释在场（:207-212） |
| 合流流构造 | 真实二进制 stdout/stderr | spawn stdio 分离捕获 + 'close' 收口 + 时序拼接 | ✓ WIRED | 本轮复跑 J0（事件制造 401/200/detach 落流 client_id==1）+ J1（负对照 parse error 证明横幅确在合流流中）行为实证 |
| 前件 10 条 key links（logEvent 出口/pinger→detach reason/Shutdown→draining/字节计数器/version 通道等） | — | — | ✓ WIRED（回归） | Go 源码零改动 + 全量套件 -race ok——接线结构性保持 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| phase08-journal.mjs 合流流 | stdout 横幅 + stderr JSON 事件 | 真实 spawn 的 wesh 二进制两路 fd 捕获 | 是（J0 实测 401→auth_failed、真实 WS 握手→attach/detach client_id==1 落流） | ✓ FLOWING |
| 前件 6 条数据流（healthz 四字段/metrics 17 series/事件流各字段） | — | — | 是（回归：Go 零改动 + phase08.mjs S1-S6 复跑 21/21） | ✓ FLOWING |

无 STATIC/DISCONNECTED/HOLLOW_PROP 项。

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| G-08-2 合流模拟四组断言 + SEC 自净 | `node web/uat/phase08-journal.mjs /tmp/wesh-uat/wesh-verify`（HEAD 全新构建） | 6/6 PASS：J0 事件制造 / J1 负对照 exit=4 parse error / J2 纯度 `{`行恰4 / J3 示例一恰1行零用户名键 / J4 示例二恰2行 reason=normal / SEC 零命中 | ✓ PASS |
| phase08.mjs 六场景回归 | `node web/uat/phase08.mjs /tmp/wesh-uat/wesh-verify` | 21/21 PASS（含 SEC 自净） | ✓ PASS |
| 全量套件 -race | `go test -race -count=1 ./...` | 五包全 ok（server 58.0s） | ✓ PASS |
| README 断言组（plan Task 1 automated） | grep 四连（防护段≥2 / 无未防护 journalctl / `^wesh: ` 清零 / `==7` 保持） | 全 PASS | ✓ PASS |
| wesh 源码零改动 | `git diff 0ae4c0f..HEAD -- '*.go'` + 逐提交 --stat | diff 为空；f19811b=README only、9ebbbe4=新脚本 only、d8f0fd8/fe2afa1=.planning only | ✓ PASS |

### Probe Execution

SKIPPED——本 phase 无 `scripts/*/tests/probe-*.sh` 形态探针（项目无 scripts/ 目录；验证面为 Go 测试 + web/uat/*.mjs 协议脚本，已全部独立执行）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| OPS-06 | 08-03, 08-05 | /healthz 健康检查端点 | ✓ SATISFIED | 回归：server.go:538 注册在场 + 全量套件 -race ok + phase08.mjs S1/S4 复跑 PASS；REQUIREMENTS.md L67 已勾选、Traceability L152 Complete |
| OPS-07 | 08-04, 08-05 | /metrics 监控端点（连接数、会话数、收发字节数） | ✓ SATISFIED | 回归：server.go:486/527 双注册在场 + phase08.mjs S2/S3 复跑 PASS；REQUIREMENTS.md L68 已勾选、Traceability L153 Complete |
| OPS-08 | 08-01, 08-02, 08-05, **08-06** | 结构化日志（JSON），含审计事件（认证失败、连接建立/断开、会话生命周期） | ✓ SATISFIED | 回归 + 本轮实测：log.go 单出口/proxy.go sanitize 在场；phase08.mjs S5/S6 PASS；08-06 补全 journald 检索面文档防护与回归夹具（G-08-2 闭合）；REQUIREMENTS.md L69 已勾选、Traceability L154 Complete |

**Orphaned requirements:** 无——REQUIREMENTS.md Traceability 表 Phase 8 行恰为 OPS-06/07/08 三条，全部被 plan `requirements:` 字段认领（08-01: OPS-08 / 08-02: OPS-08 / 08-03: OPS-06 / 08-04: OPS-07 / 08-05: OPS-06+07+08 / 08-06: OPS-08）。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| README.md / phase08-journal.mjs | - | TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER | 无 | 本轮修改/新增两文件 grep 全空 |
| README.md | ~~336~~ | ~~陈旧文本事件行示例~~ | **已闭合** | 前件登记的唯一 ⚠️ Warning（D-13 迁移前文本事件行示例）经 08-06 Task 1 顺手收口——:336 现为 JSON detach 行（与 :438 schema 同形），本验证者通读确认 |

### Human Verification Required

以下 1 项为 G-08-2 修复面的真实环境确认，处置与 08-06-PLAN verification 第 4 条 / 08-06-SUMMARY coverage D3（human_judgment: true，明示非阻塞 UAT resume 项）/ 08-UAT.md A2 前文一致——机械化等价面已全绿，属残余实机复核：

### 1. A2 真实 journald 复测（G-08-2 修复后）

**Test:** systemd 用户级 unit（wesh-uat.service，A2 既存事件在 journal 可回溯、无需重制）下以 sg systemd-journal 代跑 README 两则新示例：`journalctl -u wesh -o cat | grep '^\{' | jq -c 'select(.event=="auth_failed")'` 与 `select(.client_id==N)`
**Expected:** jq 零 parse error（stdout 横幅行被防护段滤除）；检出既存 auth_failed 事件行（无 user/username 键）与同一 client_id 的 attach/detach 对（reason=normal）
**Why human:** 依赖真实 systemd/journald 检索面；自动化等价面已绿（phase08-journal.mjs 6/6——合流模拟上 README 逐字管道端到端 + 负对照自证）；A2 原 blocked 点（journal 读权限）已随用户加 systemd-journal 组解除

### Gaps Summary

无差距。上一轮唯一 major gap **G-08-2 对账为 resolved/已闭合**：README 两则 journald 示例补 `grep '^\{'` 防护 + 合流机理说明（f19811b），phase08-journal.mjs 合流模拟回归夹具（9ebbbe4）本轮经 HEAD 全新构建二进制独立复跑 6/6——负对照（J1 exit=4 parse error）证明夹具确复现原缺陷形态、非空转通过；wesh 源码与启动输出零改动（D-14/D-15/D-16 保持，git diff 实证）。前件唯一 Warning（README:336 陈旧文本事件行）同轮顺手收口。OPS-06/07/08 三需求维持 SATISFIED；全量回归（go test -race 五包 58.0s + phase08.mjs 21/21）零破坏、零回归。状态为 human_needed 仅因 A2 真实 journald 复测一项残余人工复核（处置与前文一致，非阻塞）。

---

_Verified: 2026-08-28T12:41:00Z_
_Verifier: Claude (gsd-verifier)_
