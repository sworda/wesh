# Phase 5: 多客户端共享 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-19
**Phase:** 5-multi-client
**Areas discussed:** 分享链接认证形态, 写权限模式与 owner 归属, resize 仲裁参与集, 踢出与新客首屏体验

---

## 分享链接认证形态

### Q1: token 与凭据/整站 Basic 体系的关系

| Option | Description | Selected |
|--------|-------------|----------|
| 独立第三认证通道 | 持有效 token 可 GET 页面 + POST /api/attach 换 ticket（绕过 Basic）；无/错 token 时 Basic 矩阵不变；operator 用凭据、旁观者用链接，两体系共存 | ✓ |
| 链接即唯一认证 | 启用分享链接时不开 Basic，与 --credential 互斥；模型最简但 operator 失去稳定凭据入口 | |
| 仅无认证模式可用 | 凭据模式不打印分享链接；纵深零削弱，但推荐部署形态下 MULTI-05 完全不可用 | |

**User's choice:** 独立第三认证通道
**Notes:** 公网部署（推荐带凭据）下分享必须可用；Basic 纵深多一个 token 闸门是接受的代价。

### Q2: token 时效语义（"一次性"落在哪里）

| Option | Description | Selected |
|--------|-------------|----------|
| 可复用至重启 | 链接 token 是能力凭证，attach 时换 SEC-02 一次性 ticket——一次性落在 ticket 上；ro/rw token 每轮启动重新随机生成 | ✓ |
| 严格一次性 | 每条链接仅核销一次 attach；多人分享需新增生成通道，复杂度高一档 | |
| 可复用 + 短 TTL | 可复用但短 TTL，过期需重新获取；仍需生成新链接通道 | |

**User's choice:** 可复用至重启
**Notes:** ro 链接必须可同时给多个旁观者，否则演示场景不成立。

### Q3: token 在 URL 的位置

| Option | Description | Selected |
|--------|-------------|----------|
| #key= fragment | fragment 不上 wire、不进日志（推荐项） | |
| ?key= query | 上 wire 进日志，前端可 replaceState 抹地址栏 | |
| /s/{token}/ 路径段 | 服务端页面 GET 时即可校验 token 完成门禁 | ✓ |

**User's choice:** /s/{token}/ 路径段（逆推荐）
**Notes:** 用户选择门禁闭环能力优先于日志暴露面；暴露面由"token 永不作 logEvent 参数 + README 明示反代日志脱敏"兜底。

### Q4: 启动打印链接的 host 部分

| Option | Description | Selected |
|--------|-------------|----------|
| 原样用 bind 值 | 零猜测零魔法，但 0.0.0.0 违背"即打即用" | |
| 回填接口 IP | 通配 bind 时取首个非 loopback IPv4，具体地址原样用 | ✓ |
| --public-url flag | 显式指定，最准确但多一个 one-way flag | |

**User's choice:** 回填接口 IP

---

## 写权限模式与 owner 归属

### Q1: CLI 面与默认模式

| Option | Description | Selected |
|--------|-------------|----------|
| 拆两个 flag，默认 owner | --writable 总闸 + --write-policy=owner\|all 默认 owner（安全默认哲学） | ✓ |
| 拆两个 flag，默认 all | ttyd -W 行为对等；演示场景误开 rw 链接会被抢输 | |
| --writable 枚举化 | 单 flag 表达全部；破坏已发布布尔契约，违背 P2 D-15 | |

**User's choice:** 拆两个 flag，默认 owner

### Q2: owner 断线后写所有权

| Option | Description | Selected |
|--------|-------------|----------|
| 顺序递补 | 在场 rw 客户端按 attach 顺序递补；会话不锁死 | ✓ |
| 不让渡，全场 ro | owner 断线 = 演示结束；意外断线会话报废 | |
| 退化为 all | owner 掉线瞬间旁观者都能输入，安全不讨喜 | |

**User's choice:** 顺序递补
**Notes:** 前提由讨论明确：owner 无法显式指定（无本地终端 UI），首个 rw attach 者即 owner。

### Q3: owner 在位时后续 rw attach 待遇

| Option | Description | Selected |
|--------|-------------|----------|
| 降级 ro + 递补队列 | Welcome mode=ro，owner 断线自动升格；复用现有 ro 前端形态零新 UI | ✓ |
| 拒绝 attach | 1008 permission_denied；体验差且需区分人群 | |
| 降级 ro 不递补 | 实现最简但与递补语义冲突 | |

**User's choice:** 降级 ro + 递补队列

### Q4: 最大并发客户端数（RES-03）

| Option | Description | Selected |
|--------|-------------|----------|
| --max-clients flag | 容量策略是部署关切；满员 Accept 前 HTTP 503；初值负载测试标定 | ✓ |
| 常量，Phase 7 收口 | 沿用 P2 D-10 纪律；部署侧无法调容量 | |
| flag + 背压参数也开 | 一次新增多个 one-way 契约，与 D-10 冲突 | |

**User's choice:** --max-clients flag

---

## resize 仲裁参与集

### Q1: 最小公共矩形的参与集

| Option | Description | Selected |
|--------|-------------|----------|
| 按写权限分层 | owner 模式仅 owner；all 模式全部 rw 取 min；纯 ro 会话全体 ro 取 min | ✓ |
| 全体一律参与 | 含 ro 旁观者；手机旁观者缩全场，演示场景致命 | |
| 永远只跟 owner | all 模式下第二人拖窗不生效，与 MULTI-04 冲突 | |

**User's choice:** 按写权限分层
**Notes:** 连带修订 P2 D-13（ro 放行 RESIZE 是单客户端语境）：ro 端 RESIZE 不参与仲裁、服务端忽略、前端 ro 不发。推论：min-rect 保证任何客户端窗口 ≥ PTY 尺寸，无需 S→C 尺寸下发帧。

---

## 踢出与新客首屏体验

### Q1: 1013 踢出后前端处理

| Option | Description | Selected |
|--------|-------------|----------|
| 提示 + 手动刷新 | onclose 1013 分派 showStatus 提示；重连归 Phase 6；避免后台标签重连循环 | ✓ |
| 自动重连一次 | 过渡版；Phase 6 落地时拆掉重写 | |
| 完整退避提前 | 模糊 phase 边界 | |

**User's choice:** 提示 + 手动刷新（组件连续空响应后文本作答 "1"）

### Q2: 新 attach 客户端首屏

| Option | Description | Selected |
|--------|-------------|----------|
| SIGWINCH 强制重绘 | TIOCGPGRP 取 pgid + kill；全屏程序立即重绘，新客秒见画面 | ✓ |
| 空白等输出 + 文档 | 零成本；观众面对黑屏第一印象差 | |

**User's choice:** SIGWINCH 强制重绘（文本作答 "1"）

### Q3: 新 attach 客户端标题初始状态

| Option | Description | Selected |
|--------|-------------|----------|
| 保持纯前端解析 | P4 D-01 终局：服务端零拷贝不跑 OSC 状态机；标题随重绘自然恢复；'T' 帧终局不实现 | ✓ |
| 服务端缓存标题补发 | 违背 D-01 零拷贝理念，收益仅消除短暂默认标题窗口 | |

**User's choice:** 保持纯前端解析（文本作答 "1"）

---

## Claude's Discretion

- 递补升格的 S→C 通知通道（复用 Welcome 帧 vs 新类型字节）
- 分享 token 存储/比较形态（ticketStore 同款；失败是否计入 throttle）
- outbox 容量/水位/strikes、输入限速 rate、--max-clients 默认初值（负载测试标定，Phase 9 回填）
- 接口 IP 选取策略、/s/{token}/ 路由装配、token 无效响应文案
- max-clients 计数口径、permission_denied 使用场景评估、1013 reason 命名
- 注册表数据结构与 fan-out goroutine 拓扑、输入限速超限行为

## Deferred Ideas

- 完整断线自动重连 — Phase 6 CORE-05
- --once / 所有客户端断开退出 — Phase 6 SESS-01/02
- EXIT 终结帧广播形态 — Phase 6 SESS-03
- 参数标定回填 — Phase 9；outbox 深度/踢出计数 metrics — Phase 8
- 1001 优雅下线 — Phase 7；新 flag 配置文件收口 — Phase 7 OPS-09
- 'T' TITLE 帧 — 终局不实现（D-12 关闭，非延期）
