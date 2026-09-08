# Phase 13: 资源防线与终结语义 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-05
**Phase:** 13-resource-defense
**Areas discussed:** stop-timeout 默认值、spawn 双令牌桶、观测面四 OQ、pinger/dwell 竞态

---

## stop-timeout 默认值（公开契约变更）

### Q1: per-client 模式下 --stop-timeout 默认值如何裁决？

| Option | Description | Selected |
|--------|-------------|----------|
| per-client 默认 5s | PITFALLS P8 推荐值；HUP 免疫进程 5s 后被 SIGKILL 强收；ttyd 语义对齐 | ✓ |
| per-client 默认 1s | 更激进收仰；shell 写 history 等清理窗口可能不够 | |
| 两模式统一默认 5s | 公开契约变更面更大，shared v1.0 行为变化 | |
| 保持 0 + 文档红线 | 零配置面变更；泄漏窗已实证存在，接受即默许 | |

**User's choice:** per-client 默认 5s
**Notes:** 机制 Phase 11 D-01 已就位，仅改默认值；shared 保持 0 零回归。

### Q2: 用户显式 --stop-timeout=0 + per-client 时如何处置？

| Option | Description | Selected |
|--------|-------------|----------|
| 显式 0 尊重 + warn | fs.Visit 显式位区分；validateStartup warn 一行明示泄漏风险 | ✓ |
| 显式 0 静默尊重 | 配置面更安静；用户可能不知 per-client 下 0 的泄漏语义 | |
| per-client 强制 5s | 违背「不静默改写用户输入」纪律 | |

**User's choice:** 显式 0 尊重 + warn

---

## spawn 双令牌桶（churn 防线）

### Q1: 参数与调优入口？

| Option | Description | Selected |
|--------|-------------|----------|
| 内部常量 8/16+1/4 | 全局 8/s burst 16 + per-IP 1/s burst 4；Options 测试覆写；不暴露公开面 | ✓ |
| 暴露 flag/TOML | 运维可调；标定数据未到手先锁公开面 | |
| 调整参数量级 | 用户有其他量级直觉 | |

**User's choice:** 内部常量 8/16+1/4

### Q2: 取不到令牌的拒绝 wire 形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 1011 + 复用 server_error | 与容量再闸同码同串（wire 聚合、日志细分）；协议零改动 | ✓ |
| 1008 + spawn_throttled 新串 | PITFALLS 原推荐；1008 受众混入容量策略（D-02 曾否决此码） | |
| 1011 + spawn_throttled 新串 | wire 可区分但需动 proto 错误码表 | |

**User's choice:** 1011 + 复用 server_error（日志事件名 spawn_throttled 细分）

### Q3: per-IP 桶的键如何取？

| Option | Description | Selected |
|--------|-------------|----------|
| XFF 换键 | --auth-header 信任闸同闸采信；反代后多用户不共享一桶 | ✓ |
| 仅 remote IP | 实现最简；反代场景 per-IP 桶形同虚设 | |

**User's choice:** XFF 换键

---

## 观测面四 OQ

### Q1 (OQ①): healthz session_active per-client 语义？

| Option | Description | Selected |
|--------|-------------|----------|
| 恒 true | 「会话服务可用」语义诚实；编排探活只看 200/503 不受影响 | ✓ |
| 有活会话=true | 保留 shared 字面；per-client 空闲期恒 false，探活信息量为零 | |

**User's choice:** 恒 true

### Q2 (OQ②): 活跃会话数 gauge 的 series 形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 同名按模式出计数 | wesh_session_active per-client = len(pcSessions)；HELP 按模式生成 | ✓ |
| 另起 wesh_sessions_active | 两 series 语义各自单一；series 清单膨胀 | |

**User's choice:** 同名按模式出计数

### Q3: spawn/kill 计数器范围？

| Option | Description | Selected |
|--------|-------------|----------|
| 四件全入 | spawn 成功/失败/KILL 兜底/节流拒绝；throttled 是防线「生效过」唯一 metrics 信号 | ✓ |
| 三件（throttled 仅日志） | metrics 面更精简，churn 观测依赖日志检索 | |
| 最小化（仅 gauge） | 只在日志面出事件，metrics 只加 session 计数 | |

**User's choice:** 四件全入（per-client-only 出数，shared 恒 0 不摘 series）

### Q4: 审计事件粒度确认？

| Option | Description | Selected |
|--------|-------------|----------|
| 按研究 schema 执行 | session_start/end 带 pid+client_id；KILL 兜底经 signal 字段归因 | ✓ |
| kill 独立事件 | 泄漏防线触发历史独立可检；事件目录膨胀 | |

**User's choice:** 按研究 §7.1 schema 执行

---

## pinger/dwell 竞态（Phase 12-04 发现）

### Q1: 裁决方向？

| Option | Description | Selected |
|--------|-------------|----------|
| 接受 1006 语义 | 真死连接更早收口语义正确；dwell 面=回 pong 的活慢端（真实浏览器形态）；零库内部依赖 | ✓ |
| pinger 区分两类超时 | 一切停读端到达 dwell 1013；匹配库内部错误文案脆弱（库升级风险） | |

**User's choice:** 接受 1006 语义 + 文档明示 + S6 --ping-interval=0 隔离测试纪律固化
**Notes:** 关键论证——浏览器网络栈自动回 pong 不受 JS 节流影响，竞态对真实浏览器结构性不可达；触发面只剩「连 pong 都不回」的真死连接，1006 语义本就正确。

---

## Claude's Discretion

- pcSupervisor / maybeExitWhenEmptyLocked per-client 分支 / Shutdown N 组有界 join 上界值 / 退出码 last-reaped-code 实现（研究 §4.1/§4.3/§4.4 参考实现已备）
- WESH_REMOTE_USER 注入接入点（whitelistEnv vs StartOptions.Env 通道）
- per-IP 桶存储形态（惰性过期）、WR-02 修复精确形态、churn 压测断言挂接
- phase13.mjs 场景集编号与断言颗粒度、metricsSeries17 镜像扩展形态、文档精确措辞

## Deferred Ideas

- 令牌桶参数调优入口（Phase 14 负载矩阵后常量改值即可，公开入口需真实需求）
- pinger 区分写阻塞/pong 超时（herdr 类客户端实证受害时后补）
- session_killed 独立审计事件（运维检索需求实证后再评）
- 参数化 harness / 模式文档 PC-12 / herdr E2E UAT / Playwright 层 / 负载矩阵标定 — Phase 14 既定
