# Phase 8: 可观测性 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-27
**Phase:** 8-observability
**Areas discussed:** Metrics 格式与依赖、端点认证与挂载、日志迁移形态、审计事件目录、追加灰区四项

---

## Metrics 格式与依赖

### Q1: /metrics 端点用什么格式？

| Option | Description | Selected |
|--------|-------------|----------|
| 手写 Prometheus 文本格式 | 指标集小（~10 gauge/counter），文本 exposition 手写几十行；Grafana/Prometheus 直采零转换；与 STACK.md 哲学一致 | ✓ |
| 引入 client_golang | 生态标准库但依赖树大，与单静态二进制+stdlib-first 相悖 | |
| 自渲 JSON 端点 | 最简单但非标准，需转换层 | |

**User's choice:** 手写 Prometheus 文本格式

### Q2: 每客户端 outbox 深度怎么进 metrics？

| Option | Description | Selected |
|--------|-------------|----------|
| 聚合 gauge，不带客户端 label | max/sum 聚合；客户端身份进 label 是隐私红线延伸，基数不可控 | ✓ |
| 每客户端独立 series | 可定位具体慢客户端但基数/隐私双问题 | |
| 不进 metrics | 只做日志事件 | |

**User's choice:** 聚合 gauge，不带客户端 label

### Q3: 要不要暴露基础 runtime 指标？

| Option | Description | Selected |
|--------|-------------|----------|
| 业务指标 + 基础 runtime | goroutine 数/内存；P5/P6 goroutine 生命周期纪律回归可观测化 | ✓ |
| 仅业务指标 | 最小暴露面 | |

**User's choice:** 业务指标 + 基础 runtime

### Q4: 收发字节数口径？

| Option | Description | Selected |
|--------|-------------|----------|
| 网络流量视角 | WS 帧字节计，fan-out ×N 反映带宽 | |
| PTY 数据源视角 | PTY 读取字节单计 | |
| 双指标分开 | pty_output 与 ws_sent/ws_recv 分开，放大比可除出 | ✓ |

**User's choice:** 双指标分开

### Q5: 「会话数」指标口径？

| Option | Description | Selected |
|--------|-------------|----------|
| session_active gauge + 连接三件套 | gauge(0/1) 探活语义 + connected/total/kicked 三件套 | ✓ |
| 仅 gauge 最简集 | ROADMAP 字面最简执行 | |
| 跳过会话数 | 共享模型下无意义 | |

**User's choice:** session_active gauge + 连接三件套
**Notes:** 共享进程模型下会话数恒 1 是退化指标的模型纠偏

### Q6: per-IP 节流指标口径 + build_info？

| Option | Description | Selected |
|--------|-------------|----------|
| 认证计数器 + build_info | auth_failed/auth_throttled 总量（无 IP label）+ build_info{version} | ✓ |
| 仅认证计数器 | 不加 build_info | |
| 都不加 | 认证事件只查日志 | |

**User's choice:** 认证计数器 + build_info
**Notes:** P7 deferred 的 XFF 节流口径落为总量计数

---

## 端点认证与挂载

### Q1: /healthz 过不过 Basic 认证闸？

| Option | Description | Selected |
|--------|-------------|----------|
| /healthz 免认证 | 探活器结构性带不了凭据；只暴露进程存活；整站 Basic 唯一窄例外 | ✓ |
| /healthz 过 Basic 闸 | 一致性最强但编排探活实际不可用 | |
| 可配 flag | 多一个 one-way CLI 契约 | |

**User's choice:** /healthz 免认证

### Q2: /metrics 过不过 Basic 认证闸？

| Option | Description | Selected |
|--------|-------------|----------|
| /metrics 跟随认证闸 | 认证开启时过 Basic（Prometheus basic_auth）；--no-auth 自然免 | ✓ |
| /metrics 免认证 | 采集配置最简但裸奔部署泄漏行为轮廓 | |
| 可配 flag | --metrics-auth 独立 flag | |

**User's choice:** /metrics 跟随认证闸

### Q3: /healthz、/metrics 挂哪？

| Option | Description | Selected |
|--------|-------------|----------|
| 根路径固定 | 不受 base-path 影响；探活/采集直连后端路径恒定 | ✓ |
| 挂 base-path 下 | 全部端点单一前缀但探活配置跟 bp 变 | |
| 双挂 | 双写口漂移面 | |

**User's choice:** 根路径固定

### Q4: /healthz 返回什么？

| Option | Description | Selected |
|--------|-------------|----------|
| 200 + 状态 JSON | status/clients/max_clients/session_active | ✓ |
| 200 极简 body | 状态查询全走 /metrics | |
| liveness/readiness 双端点 | k8s 双探针完整但误导编排摘流 | |

**User's choice:** 200 + 状态 JSON

### Q5: 优雅关停进行中 /healthz 返 503？

| Option | Description | Selected |
|--------|-------------|----------|
| 关停中 503 draining | 反代/编排不向将死实例导新流；atomic bool 与 1001 同源挂点 | ✓ |
| 恒 200 不管关停 | 最简；关停窗口短 | |

**User's choice:** 关停中 503 draining

### Q6: 运维端点独立端口？

| Option | Description | Selected |
|--------|-------------|----------|
| 同端口 | 个人运维零额外配置；暴露面由认证闸决策承担 | ✓ |
| 独立运维端口 | 公网部署物理隔离但复杂度高 | |
| 同端口 + 独立端口 deferred | v1 同端口留升级路径 | |

**User's choice:** 同端口
**Notes:** 用户选纯同端口，未挂 deferred

---

## 日志迁移形态

### Q1: logEvent 三要素单行怎么迁 slog？

| Option | Description | Selected |
|--------|-------------|----------|
| 全量原子迁移 | 包级函数内部换 slog，调用点零改动；无双轨漂移面 | ✓ |
| 双轨过渡 | 双倍输出噪音，无既有消费者需兼容 | |
| 只新增事件进 slog | 同一事件两种形态永久并存 | |

**User's choice:** 全量原子迁移

### Q2: 启动行/分享链接行（含 token）怎么处理？

| Option | Description | Selected |
|--------|-------------|----------|
| 启动行保持人读文本 | 冒烟/UAT 解析消费者零破坏；operator 输出与机器事件分流 | ✓ |
| 启动行 JSON 化去 token | 机器可解析但既有消费者全要改 | |
| 人读行 + slog 事件并行 | 双通道共存 | |

**User's choice:** 启动行保持人读文本

### Q3: 要不要 --log-format/--log-level flag？

| Option | Description | Selected |
|--------|-------------|----------|
| 恒 JSON 不加 flag | 零新 CLI 契约；人读走 jq | ✓ |
| --log-format flag | json\|text 可选，多一个 one-way 契约 | |
| TTY 自动检测 | DX 最好但隐式行为 | |

**User's choice:** 恒 JSON 不加 flag

### Q4: 启动期警告行进 slog？

| Option | Description | Selected |
|--------|-------------|----------|
| 警告行保持文本 | 与启动行同分类；07 结构性断言零改动 | ✓ |
| 警告行进 slog WARN | 机器事件统一但 operator 体验变差 | |

**User's choice:** 警告行保持文本

---

## 审计事件目录

### Q1: 审计事件集定哪些？

| Option | Description | Selected |
|--------|-------------|----------|
| 全量事件目录 | auth/connect/session/shutdown 四面 + exit_when_empty 族 | ✓ |
| ROADMAP 最小集 | 不扩充新事件 | |
| 现状集不扩充 | 原样 JSON 化 | |

**User's choice:** 全量事件目录

### Q2: slog JSON 里事件名放哪？

| Option | Description | Selected |
|--------|-------------|----------|
| event 字段独立 | msg 恒 "event"，event="attach" 直打字段索引 | ✓ |
| msg 携事件名 | slog 惯例少一键但检索要全文匹配 | |

**User's choice:** event 字段独立

### Q3: XFF 链首 IP 要不要过 sanitize？

| Option | Description | Selected |
|--------|-------------|----------|
| remote 也过 sanitize | 客户端可注入 XFF 首段，控制字符伪造日志行风险当期存在 | ✓ |
| remote 不动 | 信任反代语义下视为可信 | |

**User's choice:** remote 也过 sanitize

### Q4: 要不要 client_id 字段关联？

| Option | Description | Selected |
|--------|-------------|----------|
| 加 client_id 序号 | 进程内单调序号，同一连接事件流可关联；无隐私面 | ✓ |
| 不加关联字段 | 靠 remote 肉眼对 | |

**User's choice:** 加 client_id 序号

### Q5: kick/pong_timeout 独立事件还是 detach 的 reason 字段？

| Option | Description | Selected |
|--------|-------------|----------|
| detach 单事件 + reason | 连接断开检索单入口；计数走 metrics | ✓ |
| 泛事件 + 独立事件并存 | 同一断开打两行 | |

**User's choice:** detach 单事件 + reason

---

## 追加灰区（第二轮多选）

### 运维采集栈确认

**User's choice:** 按照你推荐的来就可以（free-text）
**Notes:** 手写 Prometheus 文本格式在直采/兼容栈/人工 curl 三形态均成立，前提风险闭合

### session_end 字段集

| Option | Description | Selected |
|--------|-------------|----------|
| exit_code+signal+duration | 与 EXIT 帧同源；单事件齐备 | ✓ |
| 仅 exit_code | 最小字段集 | |

**User's choice:** exit_code+signal+duration

### 认证事件字段边界

**User's choice:** 按推荐（用户 2026-08-27 批示「后续讨论内容的决策都使用你推荐的方式」）
**Notes:** throttled 携 retry_after 秒数；auth_failed 不含用户名（SEC-01 红线重申）

### 测试断言迁移形态

**User's choice:** 按推荐（同上批示）
**Notes:** captureStderr 后按行 json.Unmarshal 到 map 断言字段的 helper；不用子串正则（JSON 转义/键序下正则脆）

---

## Claude's Discretion

- series 命名细节与预埋挂点兑现形态
- metrics handler 读取 registry 状态的并发形态
- exposition 格式版本（text 0.0.4）与 Content-Type
- slog 装配点与字段命名
- 测试断言迁移 helper 具体形态
- draining bool 挂点、phase08.mjs UAT 场景矩阵、README 运维节写法

## Deferred Ideas

- liveness/readiness 双端点（/readyz）— 真实 k8s 编排反馈再评估
- OpenMetrics 格式升级 — 待真实需求
- 每客户端 label 细化指标 — 隐私/基数纪律否决，定位走日志 client_id
- 独立运维端口 — 用户明确不挂 deferred，仅记录否决事由
- --log-format/--log-level flag — DEBUG 需求出现再评
