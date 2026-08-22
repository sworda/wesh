# Phase 6: 会话生命周期与重连 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-23
**Phase:** 6-session-lifecycle
**Areas discussed:** 自动重连策略, 断线检测与首屏恢复, 子进程退出终结帧, --once 与无人退出模式

---

## 自动重连策略

### Q1: 触发码集

| Option | Description | Selected |
|--------|-------------|----------|
| 1006 类才自动（推荐） | 仅无码异常自动重连；1013 维持 P5 D-10 手动刷新 | ✓ |
| 1013 也纳入 | 1013 自动重连但加倍退避/限次 | |
| 除终结外全自动 | 1008/1009/1011 之外全自动 | |

**User's choice:** 1006 类才自动
**Notes:** 1013 自动重连会把"慢"的惩罚消解掉，后台标签页重连→再被踢循环放大流量。

### Q2: 退避参数与尝试上限

| Option | Description | Selected |
|--------|-------------|----------|
| 无限重试（推荐） | 1s×2 封顶 30s 无限重试 + 「立即重试」按钮 | ✓ |
| 限 10 次 | 约 17 分钟窗口后落手动面板 | |
| 交 research 标定 | 调研同类工具后定 | |

**User's choice:** 用户回复「继续」默认推荐项（无限重试）
**Notes:** 30s 一次重试流量对个人运维可忽略；「标签页放着，回来已接回」是主场景。

### Q3: 重连期间 UI 形态

| Option | Description | Selected |
|--------|-------------|----------|
| 复用全屏面板（推荐） | showStatus 三态：Reconnecting / attempt N / Reconnect now | ✓ |
| 新做顶部状态条 | 不遮冻结现场，新 UI 组件 | |
| 先面板，状态条 defer | | |

**User's choice:** 复用全屏面板

---

## 断线检测与首屏恢复

### Q1: 断线检测

| Option | Description | Selected |
|--------|-------------|----------|
| online/offline 事件（推荐） | 浏览器事件 + onclose 双触发，零协议改动 | ✓ |
| 应用层心跳帧 | 检测最准但动协议 | |
| 纯 onclose 依赖 | 黑洞场景分钟级延迟 | |

**User's choice:** online/offline 事件
**Notes:** 浏览器 WS API 不暴露 ping/pong 给 JS，空闲终端无 OUTPUT 流量——「多久没收到消息」判据在浏览器侧结构性不成立。黑洞场景（无 RST 无事件）退化为 TCP 超时，风险接受。

### Q2: 重连成功后缓冲区处置

| Option | Description | Selected |
|--------|-------------|----------|
| 清屏+重绘（推荐） | term.clear() + SIGWINCH（P5 D-11 挂点延伸） | ✓ |
| 保留旧现场 | shell 历史连续但断层+花屏风险 | |
| 全重置 term.reset() | 超出必要 | |

**User's choice:** 清屏+重绘

### Q3: owner 重连与递补

| Option | Description | Selected |
|--------|-------------|----------|
| 不加豁免（推荐） | 按新 attach 走 P5 递补语义，文档明示 | ✓ |
| owner 恢复窗口 | 身份暂存/倒计时/双 owner 交接新状态机 | |
| 交 research | | |

**User's choice:** 不加豁免

### Q4: 服务端重启后的重连

| Option | Description | Selected |
|--------|-------------|----------|
| 文档明示（推荐） | token 失效落手动面板；凭据模式接回新 shell，README 明示 | ✓ |
| 代际标识 | Welcome 加 generation id | |

**User's choice:** 文档明示

---

## 子进程退出终结帧

### Q1: 协议形态

| Option | Description | Selected |
|--------|-------------|----------|
| Error 帧扩展（推荐） | 'E' + 新 code + exit_code 可选字段 | |
| 新类型字节 | EXIT 专用类型字节 | ✓ |
| close reason 携带 | 违背「只认 code 不认 reason」纪律 | |

**User's choice:** 新类型字节（推翻推荐）
**Notes:** 用户裁决：终结语义独立于错误语义——子进程正常退出（exit 0）不是"错误"，不该挤进 Error 帧 code 空间。

### Q2: 载荷形状

| Option | Description | Selected |
|--------|-------------|----------|
| exit_code+message（推荐） | 结构化断言 + 前端直显；信号死亡 -1+信号名 | ✓ |
| exit_code+signal | 前端自行组文案 | |
| 仅 exit_code | 前端维护信号文案表 | |

**User's choice:** exit_code+message

### Q3: 前端展示

| Option | Description | Selected |
|--------|-------------|----------|
| 正文含退出码（推荐） | EXIT 暂存 → 1000 → 「Session ended」正文 message | ✓ |
| 标题带码 | 标题长度不控 | |
| 非零警告样式 | 超出三态面板契约 | |

**User's choice:** 正文含退出码

### Q4: 重连循环遇服务端退出的收口

| Option | Description | Selected |
|--------|-------------|----------|
| 文案明示（推荐） | Reconnecting hint「若服务端已退出请从 shell 重启」 | ✓ |
| 重试降级文案 | 计时阈值新逻辑 | |
| 交 research | | |

**User's choice:** 文案明示
**Notes:** 浏览器 connect 失败不暴露 refused/timeout 差异，两场景同一面板通吃。

---

## --once 与无人退出模式

### Q1: --once 闸位

| Option | Description | Selected |
|--------|-------------|----------|
| 复用 503 路径（推荐） | --once ≡ --max-clients=1 + 断开退出 | ✓ |
| 409 语义复活 | 语义最准但守卫链多一条分支 | |
| WS 层 1008 | 违背 Accept 前零分配纪律 | |

**User's choice:** 复用 503 路径

### Q2: --once 断开后子进程处置

| Option | Description | Selected |
|--------|-------------|----------|
| SIGHUP 进程组（推荐） | 复活 P1 D-11：SIGHUP → Drain → exitf 退出码收口 | ✓ |
| TERM→KILL | 与 Phase 7 OPS-04 重复设计 | |
| 关 fd 不杀 | 守护型进程可能漏 | |

**User's choice:** SIGHUP 进程组

### Q3: SESS-02 flag 形态

| Option | Description | Selected |
|--------|-------------|----------|
| 可选值单 flag（推荐） | --exit-when-empty[=duration]，Go IsBoolFlag 惯例 | ✓ |
| 纯布尔 | 断网重连场景此模式等于不可用 | |
| 双 flag | CLI 面膨胀 | |

**User's choice:** 可选值单 flag

### Q4: --once 与 SESS-02 实现关系

| Option | Description | Selected |
|--------|-------------|----------|
| 语法糖统一（推荐） | --once ≡ --max-clients=1 + --exit-when-empty=0，单一收口路径 | ✓ |
| 独立机制 | 违背 exitf 单一收口纪律 | |
| 砍 --once | 违背 SESS-01 需求原文 | |

**User's choice:** 语法糖统一

---

## Claude's Discretion

- EXIT 帧类型字节具体值（建议 'X'，避开已占位 'T'/'P'）
- EXIT 帧广播与慢客户端 outbox 的写序
- message 文案措辞与信号名提取（WaitStatus）
- online/offline 与 onclose 双触发幂等（重连循环单例）
- --exit-when-empty 宽限计时器挂点（零新 exitf 分支纪律）
- Reconnecting 面板 attempt/倒计时格式
- UAT 场景矩阵（phase06.mjs）

## Deferred Ideas

- 顶部状态条重连 UI（不遮冻结现场）— 后续迭代
- 会话代际标识（generation id）— 服务端重启提示增强
- --exit-when-empty 宽限默认值负载标定 — Phase 9
- 1001 优雅下线发送路径 — Phase 7
- 新 flag 配置文件收口 — Phase 7 OPS-09
- 断开退出事件进 metrics/审计日志 — Phase 8
