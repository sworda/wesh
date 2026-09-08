# Phase 11: per-client 生命周期主干 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-03
**Phase:** 11-per-client
**Areas discussed:** KILL 兜底装配时机, 容量再闸拒绝形态, 观测面最小钩子, 验证面切片

---

## KILL 兜底装配时机

| Option | Description | Selected |
|--------|-------------|----------|
| 机制先行（推荐） | 每会话 teardown 固定序列一次定型：SIGHUP → (stopTimeout>0 则 AfterFunc 补 SIGKILL) → Drain(200ms) → Close → Wait。默认值 0 语义不变（不补），Phase 13 只裁决默认值。teardown 序列不留二期开口，Pitfall 3 恰好一次序列完整落地 | ✓ |
| 严格切片 | Phase 11 只发 SIGHUP，KILL 机制连默认值一起归 Phase 13。diff 更小，但 teardown 序列 Phase 13 要再开一次；Phase 11→13 窗口期 HUP 免疫进程泄漏到自然死亡（pcSessions 持续登记，Shutdown 暂不能覆盖残留者） | |

**User's choice:** 机制先行
**Notes:** 只锁机制不锁默认值——stop-timeout 默认值重议（公开契约变更）仍归 Phase 13 裁决项①。

---

## 容量再闸拒绝形态

**Q1 满员拒绝 wire 形态：**

| Option | Description | Selected |
|--------|-------------|----------|
| 1011 + Error message（推荐） | Error{server_error, "server is at capacity…"} + Close(1011)。前端 1011 分派渲染 message——文案准确可达；协议零改动红线保持（复用既有机器串）；与 spawn 失败同码同串，服务端日志以事件名区分 | ✓ |
| 1013 max_clients | Close(1013, "max_clients") 裸关闭。try-again 语义最准，但前端只认 code → 显示「慢消费者被踢」错位文案（Phase 12 才有前端改动窗口），且 1013 计数与慢客户端 kicks 混同 | |
| 1008 policy | Error + Close(1008)。「Connection refused」标题贴容量拒绝，message 可达；但 1008 既有语义=认证/版本策略违反，容量策略混入 | |

**User's choice:** 1011 + Error message
**Notes:** 关键依据 = 前端 1013 分派只认 ev.code 不渲染 reason（main.ts:946，防钓鱼面设计），固定慢消费者文案在 per-client 满员场景语义双重错位。

**Q2 闸后竞态窗口复检+回收时机：**

| Option | Description | Selected |
|--------|-------------|----------|
| 复检回收本阶段装（推荐） | spawn 成功后注册点 hubMu 内复检 len(pcSessions)：超编者 SignalGroup(HUP)+Drain 回收（≤5 行，研究 §5 规则 1 建议实现）。「并发子进程数 ≤ maxClients」硬不变量 Phase 11 即成立，Phase 13 裁决项④提前消解 | ✓ |
| 留 Phase 13 裁决 | 本阶段只做闸前检查，竞态窗口超编接受；严格口径留 Phase 13 随裁决项④一并定。Phase 11 diff 更小，但窗口期硬不变量不成立 | |

**User's choice:** 复检回收本阶段装
**Notes:** 连锁效果——Phase 13 裁决项④应从 STATE.md Blockers 移除，Phase 13 本体收窄为 spawn 令牌桶 + stop-timeout 默认值 + Shutdown N 组。

---

## 观测面最小钩子

| Option | Description | Selected |
|--------|-------------|----------|
| 仅事件先行（推荐） | logEvent spawn_failed 单行事件（零敏感值）随失败路径同 PR——Pitfall 5 清理清单的测试锁定项；metrics 17 series 契约保持 Phase 13 一次性镜像扩展（本阶段不动 metrics.go） | ✓ |
| 事件+counter 都先行 | spawn_failed 事件 + wesh_pty_spawn_failures_total counter 都本阶段装——EMFILE 级联立即可见；代价：metricsSeries17 镜像 Phase 11 就动（17→18），Phase 13 变增量 | |
| 全归 Phase 13 | Phase 11 观测面零改动；窗口期 spawn 失败服务端无痕（仅客户端 1011 可见 + UAT 断言） | |

**User's choice:** 仅事件先行
**Notes:** Phase 11→13 窗口期 per-client 会话生命周期审计空白（session_start/end 的 client_id 粒度归 Phase 13 一次补齐），已明示接受。

---

## 验证面切片

**Q1 Go 测试归属形态：**

| Option | Description | Selected |
|--------|-------------|----------|
| 新增独立测试文件（推荐） | 新测试全部进 perclient_test.go（研究 §11 文件清单先例），per-client-only；既有 shared 测试零改动原样跑；参数化 harness 与三维归类表归 Phase 14——Phase 11 diff 不碰任何既有测试文件装配点 | ✓ |
| 提前建参数化 harness | 本阶段建 newTestServer(t, mode)，新测试直接走参数化——Phase 14 归类前移；代价：动 shared 测试装配点（零回归风险面），harness 双模式价值 Phase 14 才兑现 | |

**User's choice:** 新增独立测试文件

**Q2 phase11.mjs 场景范围：**

| Option | Description | Selected |
|--------|-------------|----------|
| 全链八场景（推荐） | ①双端双 pid 独立输出 ②首帧 winsize=Hello 钳制尺寸（stty size 断言）③spawn 失败 Error+1011（运行期删命令注入）④正常关闭与 1006 两形态断开→pgid ESRCH ⑤exit 42/信号死亡→仅本端 EXIT+1000、他端逐字节无扰动 ⑥max-clients=1 容量再闸 1011+容量文案 ⑦重连=新 pid ⑧trap '' HUP+stop-timeout=1s→KILL 兜底 pgid ESRCH | ✓ |
| 仅 SC 核心五场景 | 只做 SC 四条最小断言（①②③④⑤），容量闸/重连新 pid/HUP 免疫 KILL 归 Go 白盒测——脚本更薄，但少黑盒端到端证据 | |

**User's choice:** 全链八场景
**Notes:** S3 注入手法 = 运行期删命令（启动期 LookPath 预检通过后、attach 前 unlink），实证 Pitfall 5b「启动 fail-fast vs 运行期 per-request degrade」哲学分界。

---

## Claude's Discretion

pcSession 字段集与 pcSessions 临界区形态；升档分支精确落点行序；ReadLoop 闭包 detach 门与 P5-1 别名红线保持；inputWriter 参数化签名；sessionWatcher EXIT 直写序列（Drain/close(inputDone) 次序）；reaped 栅栏锁归属；detach/kick SIGHUP 挂点精确插入行；darwin dup-watch fail-closed 形态；容量文案措辞；spawn_failed 事件字段集；perclient_test.go 拆分与 export_test.go 暴露面；phase11.mjs 场景内编号与断言颗粒度。

## Deferred Ideas

- wesh_pty_spawn_failures_total counter → Phase 13（metricsSeries17 镜像一次性扩展）
- per-client session_start/session_end 审计（client_id 关联键）→ Phase 13
- per-client stop-timeout 默认值重议（裁决项①）→ Phase 13（D-01 机制已就位，届时仅改默认值）
- pcSupervisor / 第二终结源 / --once·exit-when-empty per-client 语义 → Phase 13（Pitfall 1）
- newTestServer(t, mode) 参数化 harness 与三维归类表 → Phase 14
- RESIZE 直通 / ro 断言 / 重连 reset / 停读续读 → Phase 12
- spawn 双令牌桶 / Shutdown N 进程组快照 → Phase 13
