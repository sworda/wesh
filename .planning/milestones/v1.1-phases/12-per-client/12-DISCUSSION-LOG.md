# Phase 12: per-client 交互与背压语义 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-04
**Phase:** 12-per-client
**Areas discussed:** 背压踢出判据与停读机制, ro 端 RESIZE 语义, Welcome 模式位与前端 reset, 验证面切片

**Session note:** 会话开始时发现 2026-09-04T08:30 的中断检查点（1/4 区域完成），用户选择 Start fresh——旧检查点删除，全部区域重新讨论；新一轮决策与旧轮同向收敛（阻塞持帧/dwell 10s/不复刻），另新增 ro RESIZE、模式位、验证面决策。

---

## 背压踢出判据与停读机制

**Q1 停读的实现形态**

| Option | Description | Selected |
|--------|-------------|----------|
| ReadLoop 闭包阻塞持帧 | trySend 失败 → 闭包 select 等待 outbox 恢复信号/cl.done 逃逸，恢复后重试同帧；ttyd pty_pause 等效 | ✓ |
| 显式 pause/resume 状态机 | pcSession paused 位 + cond，唯一消费方 metrics 归 Phase 13，本 phase 无消费方 | |
| outbox 上层加中间缓冲 | 与「内核缓冲即缓冲」哲学冲突 | |

**Q2 「持续过载」的踢出判据**

| Option | Description | Selected |
|--------|-------------|----------|
| dwell 计时器 | 停读态连续无恢复 > T → 1013；续读重置；慢但前进者永不踢 | ✓ |
| 无看门狗 | 纯 ttyd parity，但浏览器自动回 pong，SC5 永不达成，需修 ROADMAP | |
| 写超时 | 改 writer 语义，慢网络正常消费端误伤面 | |

**Q3 dwell 阈值 T 的定值与可配性**

| Option | Description | Selected |
|--------|-------------|----------|
| 10s 常量 | defaultSlowDwell 类内部常量 + 测试可覆写；不暴露 flag | ✓ |
| 30s 常量 | 更宽容，但真死慢端子进程多阻塞 20s | |
| 可配 flag + TOML 键 | 公开契约面 +1，个人运维无调优动机 | |

**Q4 WR-01（宽限门 + creditPending 暂存层）闭合形态**

| Option | Description | Selected |
|--------|-------------|----------|
| dwell 涵盖，不复刻 | dwell 10s 从停读起点武装涵盖一切瞬态；阻塞持帧即暂存（帧在闭包栈上） | ✓ |
| 逐字复刻 WR-01 两层 | 与 kickOrCreditLocked 形态对齐，但 per-client 下是死代码面 | |
| 只复刻宽限门 | 多一个定时器一个分支，dwell 已涵盖 | |

**Q5 停读/续读周期的观测口径**

| Option | Description | Selected |
|--------|-------------|----------|
| 复用 gateTransitions | 停读/续读两点递增既有计数器；metrics.go 零改动，非新增 series | ✓ |
| 不计数，归 Phase 13 | 窗口期线上无可见性 | |

---

## ro 端 RESIZE 语义

**Q1 服务端第二闸：per-client 下 ro RESIZE 放行还是丢弃**

| Option | Description | Selected |
|--------|-------------|----------|
| 放行直通 | ttyd parity 只门 INPUT；per-client 无旁观对象；herdr ro 移动端转屏后 area 尺寸正确 | ✓ |
| 维持丢弃 | 两模式闸一致无分支，但语义错位——只是 ro 端自己体验受损 | |

**Q2 前端第一闸（ro 不发 RESIZE，05-08）是否同步放开**

| Option | Description | Selected |
|--------|-------------|----------|
| 同步放开 | per-client 按 Welcome 模式位恢复上报；shared 保持不发；与第二闸放行配套生效 | ✓ |
| 仅服务端放行 | 自家前端行为零变化，herdr ro 移动端问题依旧 | |

---

## Welcome 模式位与前端 reset

**Q1 模式位字段形态**

| Option | Description | Selected |
|--------|-------------|----------|
| session 字符串枚举 | `session: "shared"|"per-client"` 与 CLI flag 同词；恒序列化；旧服务端缺键=shared | ✓ |
| per_client 布尔 | 表达力弱，与 CLI flag 不同词需注释互指 | |

**Q2 reset 调用时机**

| Option | Description | Selected |
|--------|-------------|----------|
| Welcome 统一判断 | 模式位=per-client → terminal.reset()；首连 no-op 等价，代码零分支 | ✓ |
| 仅重连分支 reset | 多一个 reconnecting 状态读取分支 | |

**Q3 重连 reset 后是否需要「新会话」提示**

| Option | Description | Selected |
|--------|-------------|----------|
| 静默无提示 | ttyd parity；画面恢复交给子程序重绘/herdr attach | ✓ |
| 加新会话提示 | 普通 shell 友好，但 herdr/tmux 场景是噪音 | |

---

## 验证面切片

**Q1 Go 测试文件归属**

| Option | Description | Selected |
|--------|-------------|----------|
| 扩展 perclient_test.go | 同模式单一家，Phase 14 harness 收编只动一个文件 | ✓ |
| 新开 perclient_io_test.go | 语义分组，但 Phase 14 多一个迁移单位 | |

**Q2 dwell 10s 不可配下协议层 UAT 的 1013 验证**

| Option | Description | Selected |
|--------|-------------|----------|
| 真实 10s 等待 | Go 测覆写短 dwell 确定性断言；phase12.mjs 真实等待做端到端证据；零测试钩子 | ✓ |
| 测试 env 钩子注入 | UAT 快 10s，但隐藏调优面与 D-03 精神冲突 | |

**Q3 前端改动验证层切片**

| Option | Description | Selected |
|--------|-------------|----------|
| jsdom+协议层，PW 归 14 | jsdom 断言 reset/ro RESIZE/缺键；浏览器观感归 Phase 14 herdr 全链 | ✓ |
| 本 phase 补最小 Playwright | 双机链条提前一轮，与 Phase 14 重复建设 | |

---

## Claude's Discretion

用户未显式指定 "you decide"，以下经讨论后归入实现细节自由裁量：阻塞持帧恢复信号形态、dwell 武装/重置挂点、RESIZE 直通 case 分支形态、debouncer 组件复用形态、前端模式位存储与 RESIZE 发送门分支点、phase12.mjs 场景编号与断言颗粒度、jsdom 测试文件归属与 mock 形态、dist 重建机械步骤。

## Deferred Ideas

- 显式 pause/resume 状态位观测（Phase 13 metrics 粒度落地时再评）
- dwell 阈值调优入口（仅当后台标签页节流实证成为痛点；常量改值非契约变更）
- WR-01 宽限门/creditPending 复刻（若瞬态满箱误踢实证出现，回写 WR-01 重开）
- 重连「新会话」提示文案（D-10 静默裁决，UX 迭代候选）
- 后台标签页 1013 后自动重连体验（PC-06 语义自然延伸，非缺陷）
- Phase 13 既定范围（令牌桶/Shutdown N 组/第二终结源/stop-timeout 默认值/metrics 审计粒度/WESH_REMOTE_USER）与 Phase 14 既定范围（参数化 harness/PC-12 文档/PC-13 herdr UAT/Playwright 层）原样登记
