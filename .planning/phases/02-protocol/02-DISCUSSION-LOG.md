# Phase 2: 协议基线 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-15
**Phase:** 2-协议基线
**Areas discussed:** 握手与帧编码形状, 关闭码与错误码集合, 三层上限标定与配置形态, 只读边界与保活参数

---

## 握手与帧编码形状

### Q1: 控制面帧（Hello/Welcome/Error）怎么编码？

| Option | Description | Selected |
|--------|-------------|----------|
| A: binary 统一分派 | 所有帧 = 1 字节类型 + 载荷，控制帧载荷为 JSON；前后端单一 switch 分派，与 D-16 预留一致 | ✓ |
| B: 控制面 text JSON | 控制帧走 WS text frame 纯 JSON，无类型字节；ARCHITECTURE §2.8 原方案 | |

**User's choice:** A（先要求详细解释分歧后决策）
**Notes:** 用户决策前要求展开解释——控制面帧解决什么问题（Phase 1 协议是纯数据管道，服务端无法对客户端说话：无错误类型、无版本协商，ttyd spawn 失败原因传不出）、分歧来源（ARCHITECTURE.md 是调研期语言中立设计；proto.go 是 Go 栈已定后的更近决策）。解释后选 A。

### Q2: Hello 帧 schema 现在定多少？

| Option | Description | Selected |
|--------|-------------|----------|
| 最小 schema + 演化纪律 | Phase 2 只定 {version, cols, rows}；未知字段必须忽略；后续 phase 加字段向后兼容 | ✓ |
| 完整 schema 一次定死 | 现在定义 {version, ticket, attach, mode, caps, cols, rows}，Phase 2 忽略未用字段 | |

**User's choice:** 最小 schema + 演化纪律

### Q3: 不带 wesh.v1 子协议的客户端在哪一层拒？

| Option | Description | Selected |
|--------|-------------|----------|
| HTTP 层拒绝 | Accept 前检查 Sec-WebSocket-Protocol 头，不含 wesh.v1 返回 400；Accept 后 assert 兜底 | ✓ |
| Accept 后发 Error 再关 | 客户端拿到人话错误，但 WS 升级已完成、资源已分配 | |
| 只靠 Hello.version 裁决 | 最宽松，失去握手期防线 | |

**User's choice:** HTTP 层拒绝

### Q4: per-IP 未认证（半开）连接上限默认值？

| Option | Description | Selected |
|--------|-------------|----------|
| 8 + HTTP 429 | 半开连接只来自扫描器/异常客户端，8 留足余量，Accept 前拒绝零 WS 资源 | ✓ |
| 16 + HTTP 429 | 更宽松，容忍同 IP 多标签异常重试风暴 | |
| 4 + HTTP 429 | 对标高暴露面服务，可能误伤同 IP 多实例调试 | |

**User's choice:** 8 + HTTP 429
**Notes:** 问题上下文明确：只管"连上但 Hello 未完成"的半开连接；NAT 多人场景 Hello 已完成不计入；5s 未认证超时 ROADMAP 已定不重问；抢跑帧按协议违规关闭（码值留区域 2 裁决）。

---

## 关闭码与错误码集合

### Q1: 1002 去留怎么裁决？

| Option | Description | Selected |
|--------|-------------|----------|
| 两集合并集 | {1000,1001,1002,1008,1009,1011,1013}；1002=协议错误、1008=策略违反；Phase 1 的 1002 保留；ROADMAP 准则视为漏写 | ✓ |
| 严格按 ROADMAP | {1000,1008,1009,1011,1013}，协议违规并入 1008；需改 Phase 1 代码（server.go:114） | |

**User's choice:** 两集合并集
**Notes:** 分歧背景：ROADMAP 成功准则 2 集合不含 1002，PITFALLS 映射表含 1002 不含 1013，Phase 1 代码已用 1002 关未知帧。

### Q2: 哪些错误发 Error 帧、哪些直接关闭码？

| Option | Description | Selected |
|--------|-------------|----------|
| 按受众分治 | 正常客户端错误（version_mismatch/server_error）发 Error+码；攻击面（unknown_frame/抢跑/超限）直接关 | ✓ |
| 一律先发 Error | 调试信息最全，但给攻击者额外反馈面 | |
| 只靠关闭码 | 协议最瘦，前端 UX 无区分 | |

**User's choice:** 按受众分治
**Notes:** 关键硬约束先交代：SetReadLimit 超限时库自动发 1009，应用层没机会补 Error 帧。

### Q3: Error 帧与 close reason 的文案形态？

| Option | Description | Selected |
|--------|-------------|----------|
| code+英文 message | Error={code,message}，message 英文人话前端直接展示；close reason 带同名机器串 | ✓ |
| 只带 code 前端映射 | 协议更瘦但前端要维护映射表，新增 code 旧前端显示不了人话 | |

**User's choice:** code+英文 message

### Q4: 1001/1013 在 Phase 2 只占位还是提前实现？

| Option | Description | Selected |
|--------|-------------|----------|
| 占位不实现 | 常量表写全 7 码注释启用 phase；Phase 2 只产生 1000/1002/1008/1009/1011 发送路径 | ✓ |
| 1001 提前实现 | SIGTERM 时发 1001 再退出——超出 phase 边界（信号处理属 Phase 7） | |

**User's choice:** 占位不实现
**Notes:** 此问由用户反问"这个区域还有没有其他问题"触发。同时定下纪律：应用层超限检测复用 1009，不得发明新码或自定义 4000 段（直接作为纪律，无需用户决策）。

---

## 三层上限标定与配置形态

### Q1: 三层上限默认值选哪档？

| Option | Description | Selected |
|--------|-------------|----------|
| 16KiB/32/16KiB | 合法流量两个数量级余量；比库默认 32KiB 更紧；经研究+负载测试标定 | ✓ |
| 32KiB/32/32KiB | 与库默认对齐，预认证窗口放宽一倍 | |
| 64KiB/128/64KiB | 为大粘贴留空间，预认证单连接内存×4 | |

**User's choice:** 16KiB/32/16KiB（先要求查 ttyd 参考值后决策）
**Notes:** 用户要求列出 ttyd 数据参考——核实结论：ttyd C→S 三层全零（protocol.c:288-296 无限 xrealloc 累积 = 预认证内存放大漏洞本体；298 空帧空指针），无参考价值，反面教材。补充对照：tungstenite 64MiB/16MiB（通用库偏宽）、coder/websocket 默认 32KiB、gorilla 默认无限制。

### Q2: 三层上限的可配形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 单 flag --max-message | 一个 flag 钉单帧+累积，分片数 32 常量；覆盖粘贴调大场景 | |
| 三 flag 全开 | 粒度最全，认知负担与 flag 面膨胀 | |
| 常量，等配置文件 | Phase 7 配置文件（OPS-09）统一收口；本 phase 超限只能重编 | ✓ |

**User's choice:** 常量，等配置文件
**Notes:** 用户先反问两个问题：① 选推荐参数后续有没有方便调整的方式？② 超限必须给明确清晰提示，不能吞错误。回答：可配形态正是本问（后用户选了常量路线）；提示走三腿（前端 1009 人话——因选常量路线文案不提 flag、服务端 stderr 单行事件、close reason 机器串）。库自动 1009 无法补 Error 帧的时序约束已向用户说明。

### Q3: Hello 前后是否切换 SetReadLimit？

| Option | Description | Selected |
|--------|-------------|----------|
| 两档切换 4K→16K | Accept 后 4KiB，Hello/Welcome 完成后 16KiB；预认证窗口内存最小化 | ✓ |
| 统一 16KiB | 少一处状态，预认证窗口允许 16KiB 消息 | |

**User's choice:** 两档切换 4K→16K

---

## 只读边界与保活参数

### Q1: ro 模式下 RESIZE 丢不丢？

| Option | Description | Selected |
|--------|-------------|----------|
| 只禁 INPUT | RESIZE 放行，单客户端窗口拖动仍同步（ttyd -R 行为一致）；Phase 5 才收写权限门 | ✓ |
| INPUT+RESIZE 都禁 | 严格零影响；PTY 尺寸=Hello 初始 cols/rows 后冻结 | |

**User's choice:** 只禁 INPUT
**Notes:** 取舍先交代：CORE-04 只说丢弃输入；ARCHITECTURE §2.9 "resize 跟随写权限"是 Phase 5 多客户端仲裁语境。

### Q2: ro 模式的前端提示形态？

| Option | Description | Selected |
|--------|-------------|----------|
| disableStdin+title 前缀 | Welcome.mode=ro → disableStdin=true + 标题 "[ro] " 前缀；零新 UI 组件 | ✓ |
| 仅 disableStdin | 最简但用户不知键盘为何无效 | |
| 角标 badge + toast | 最醒目但引入新 UI 组件 | |

**User's choice:** disableStdin+title 前缀

### Q3: 开启可写的 flag 叫什么？

| Option | Description | Selected |
|--------|-------------|----------|
| --writable | 语义直白，与现有全名风格一致 | ✓ |
| --rw | 与项目 ro/rw 术语对齐，但两字母不像正经 flag 名 | |
| --write | 动词感但表意泛 | |

**User's choice:** --writable

### Q4: ping 间隔默认值？

| Option | Description | Selected |
|--------|-------------|----------|
| 5s 默认 | ttyd 生产验证，对一切已知反代空闲超时显著小（含 30s 型 ingress） | ✓ |
| 30s 默认 | nginx 默认场景安全，但 30s 型代理处于时序边缘 | |
| 60s 默认 | nginx 默认场景即失效 | |

**User's choice:** 5s 默认
**Notes:** --ping-interval flag（0=禁用）+ pong 超时 10s 常量随包锁定。

---

## Claude's Discretion

- proto 包内部组织（常量分组、Error code 表、编解码函数签名）
- per-IP 半开计数器数据结构与 5s 计时器挂点
- 前端 onclose 各码值具体英文文案（沿用 showStatus 三态面板风格）
- stderr 单行事件具体格式（Phase 8 升级为 slog）
- ping goroutine 与单 reader/单 writer 结构的装配（coder/websocket Ping API 用法以 research 为准）

## Deferred Ideas

- 1001 发送路径 → Phase 7（优雅下线/信号处理）
- 1013 发送路径 → Phase 5（慢客户端背压踢出）
- 三层上限可配性 → Phase 7 配置文件（OPS-09）
- Error code 扩展（auth_failed/permission_denied）→ Phase 3/5
- EXIT/TITLE/PREFS 帧实现 → Phase 6/4（类型字节本 phase 占住）
- ROADMAP.md 准则 2 关闭码集合校正（补 1002）→ 下次 roadmap 维护
