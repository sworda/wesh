# Phase 10: 模式装配与接缝 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-02
**Phase:** 10-mode-assembly
**Areas discussed:** write-policy×per-client 处置、env 兜底键是否引入、README 文档面深度、验证面形态、枚举值回显口径（讨论中新浮出）

---

## write-policy=owner × per-client 组合处置

| Option | Description | Selected |
|--------|-------------|----------|
| warn 明示放行（推荐） | validateStartup 输出警告行（owner 仲裁/递补在 per-client 下不装配；ro/rw 权限级别仍按 ticket 生效）后正常启动；研究推荐；auth-header 暴露面警告同通道 | ✓ |
| exit 2 拒绝 | 双 flag 名进文案 fail-fast（write-policy×writable 先例同位）；代价：per-client 下想用 ro 分享链接的用户必须同时 --write-policy=all 或弃用该 flag | |

**User's choice:** warn 明示放行
**Notes:** ROADMAP 明示「warn 或拒绝，规划期裁决——静默永不接受」；用户认可推荐理据——ro/rw 权限级别仍被 ticket 消费（分享链接=按权限级别的独立进程入场券），非纯配置矛盾。

## warn 触发条件锚定

| Option | Description | Selected |
|--------|-------------|----------|
| 显式设置即 warn（推荐） | 锚定 writePolicySet 显式设置位：owner|all 任一显式给出 × per-client 即 warn；owner|all 在 per-client 下均为真空语义，只 warn owner 是口径分裂；writePolicySet 先例现成 | ✓ |
| 仅 owner 时 warn | 严格按 ROADMAP 字面；=all 的「全员可写」与 per-client 默认行为表面一致不 warn | |

**User's choice:** 显式设置即 warn
**Notes:** 配置来源同档（fc.WritePolicy 非 nil 即置位，07-06 合并收尾先例）——CLI 与 TOML 双源同档触发。

## env 兜底键是否引入

| Option | Description | Selected |
|--------|-------------|----------|
| 不引入（推荐） | env 层是敏感值专用通道（WESH_CREDENTIAL 先例，systemd EnvironmentFile=600 场景）；session-mode 非敏感，CLI+TOML 全覆盖；SC3 env 层真空成立，断言 flag>TOML>默认；不开非敏感键 env 先例 | ✓ |
| 引入 WESH_SESSION_MODE | SC3 字面成立；容器/systemd env 注入便利；代价：env 面扩大先例，27+ 非敏感键跟进压力 | |

**User's choice:** 不引入
**Notes:** env 层定位 = 「敏感值不落盘」专用通道而非通用配置层——用户认可此解读。

## README 文档面深度

| Option | Description | Selected |
|--------|-------------|----------|
| 最小明示（推荐） | CONFIGURATION.md flag 表+TOML 键表+校验矩阵各加行（注记「per-client 装配中与 shared 等价」）+ README 一句；完整语义段留 Phase 14（PC-12） | ✓ |
| 仅 --help 文案 | 文档不动避免描述不存在的行为；代价：--help 可见的 flag 文档查无此项，Phase 11-14 各阶段都得记着补 | |
| 完整语义段 | 文档先行驱动实现；代价：写的是研究设计而非已验证行为，每次裁决都得回改（漂移源） | |

**User's choice:** 最小明示
**Notes:** flag 公开即文档义务（每 phase 收口先例）；「装配中」注记防误用预期。

## 验证面形态

| Option | Description | Selected |
|--------|-------------|----------|
| Go 测试+既有 UAT 重跑（推荐） | Go 测试新增面（parse 拒绝矩阵/优先级链/New 互斥/fuzz 语料/StartWithSize 委托等价/warn 触发）+ 既有 phase02-09 UAT 默认模式原样重跑 + -race 全量 = 零回归双证据 | ✓ |
| 新增 phase10.mjs | 双模式启动冒烟 + 非法值进程级断言；代价：inert 无新协议行为可断言，与既有脚本重跑断言重叠 | |

**User's choice:** Go 测试+既有 UAT 重跑
**Notes:** SC1 锁定「既有协议 UAT 原样全绿」即此口径；新脚本只能重复既有脚本已证明的等价性。

## 枚举值回显口径（讨论中新浮出灰区）

| Option | Description | Selected |
|--------|-------------|----------|
| 回显值（与先例一致，推荐） | `invalid --session-mode "banana": must be shared or per-client`（write-policy main.go:619 %q 先例）；SC2 解读为凭据/token/内容红线保持、枚举非敏感面豁免（CONFIGURATION.md:124 既有口径；PITFALLS「值不敏感可回显」） | ✓ |
| 不回显值（SC2 字面） | `invalid --session-mode: must be shared or per-client`（键名可入文案）；代价：与 write-policy/--cwd 同类错误两形态并存，CONFIGURATION.md:124 需标注例外 | |

**User's choice:** 回显值
**Notes:** 代码勘察浮出的冲突——ROADMAP SC2 字面「错误文案不泄露用户输入值内容」vs 现行枚举先例回显值。裁决：SC2 的「启动面红线保持」= 凭据/token/文件内容不回显（SEC-01 本义），枚举非敏感面按 P5/P7 豁免先例回显；值域是两个固定单词无秘密可泄，回显助定位拼写错误。

---

## Claude's Discretion

- pty.StartWithSize 精确签名与 Start 委托形态、零值等价测试形态
- Options.SessionMode 字段类型与 SpawnFunc 签名形态；New 互斥校验 fail-fast 实现选型
- run() 分岔点精确位置与 sess nil 语义
- validateStartup per-client 行（LookPath 预检）文案与落点
- warn 行精确文案（双 flag 名进文案纪律）
- fuzz 语料精确样本集与 TestConfigMerge/Precedence/RedLines 扩展断言面
- --help 文案与 CONFIGURATION.md 三处加行精确措辞
- Go 测试文件归属与 server 包互斥校验测试落点

## Deferred Ideas

- `WESH_SESSION_MODE` env 键——D-03 裁决不引入；真实 env 注入需求出现再评估
- phase10.mjs 协议 UAT——D-06 裁决不建；per-client 真实行为 UAT 随 Phase 11+
- per-client 完整模式语义文档段——Phase 14（PC-12）
- write-policy warn→reject 收紧——仅真实配置漂移事故支撑时重议
