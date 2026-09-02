# Phase 6: 会话生命周期与重连 - Context

**Gathered:** 2026-08-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 6 补全会话生命周期的三种形态并落地断线重连闭环：`--once` 只接受一个客户端、其断开后服务端退出（SESS-01）；可配"所有客户端断开后退出"模式（SESS-02）；子进程退出后所有在线客户端收到含退出码的类型化终结帧、随后 1000 正常关闭（SESS-03）；WS 异常断开后前端自动重连并接回同一 PTY 进程（CORE-05——共享进程模型下重连即接回原进程，无滚动回放，屏幕靠程序重绘或 tmux/herdr 恢复）。

**In scope (from ROADMAP):** SESS-01（--once）、SESS-02（所有客户端断开退出）、SESS-03（类型化终结帧含退出码 + 1000）、CORE-05（指数退避 + 上限 + 手动入口的自动重连）。

**Out of scope (本阶段不做):** 滚动回放/会话保持（v1 不做，PROJECT 锁定）、1001 优雅下线发送路径（Phase 7）、配置文件收口新 flag（Phase 7 OPS-09）、auth-header 透传（Phase 7 SEC-07）、审计事件 slog 化与 metrics（Phase 8）、宽限/退避参数负载标定回填（Phase 9）、会话代际标识（本 phase D-07 裁决不做）、顶部状态条重连 UI（本 phase deferred）。

**已锁定不重复决策：** GoTTY 共享进程模型（PTY 随服务端启动，重连天然接回同一进程，无会话 ID 协商）；无滚动回放（屏幕靠程序重绘或 tmux/herdr，文档明示）；exitf + sync.Once 单一终结收口（P1 硬约束，新终结路径只加触发源不加分支）；关闭码全集 {1000,1002,1008,1009,1011,1013}（P2 D-05）；P2 D-01/D-02 帧类型与加字段纪律；P3 D-10 auth_failed 静默重试一次（connect() 已是可重入入口）；P4 D-18 beforeunload 重连落地后保留；P5 D-02 share token 重启即废、D-06/D-07 owner 递补语义、D-11 SIGWINCH 强制重绘挂点、D-12 标题随重绘/下次 OSC 2 自然恢复。

</domain>

<decisions>
## Implementation Decisions

### 自动重连策略（CORE-05）
- **D-01:** 触发范围 = **仅 1006 类无码异常断开**（断网/TCP 断开/pong 超时服务端 CloseNow 后前端所见的无码关闭）自动重连；1000/1008/1009/1011 不自动重连（明确终结/策略/错误语义各自有面板）；**1013 维持 P5 D-10 手动刷新**——被踢说明消费跟不上，自动重连只会再被踢，后台标签页会循环放大流量（P5 边界纪律本 phase 确认保持）
- **D-02:** 退避参数 = **1s×2 封顶 30s 无限重试**（throttleStore 同族参数族，P3 D-08 形态延伸）+ 面板「立即重试」按钮跳过当前等待（ROADMAP「手动入口」落此）；无尝试次数上限——个人运维「标签页放着，回来已接回」是主场景，30s 一次重试流量可忽略；重连成功（WELCOME 到达）退避清零
- **D-03:** 重连 UI = **复用 showStatus 全屏三态面板**：标题 Reconnecting，正文显示 attempt N / 下次重试倒计时，hint 处放可点「Reconnect now」——零新 UI 组件（P5 D-07 哲学）；顶部状态条记 deferred

### 断线检测与首屏恢复（CORE-05）
- **D-04:** 断线检测 = **浏览器 online/offline 事件 + onclose 双触发**：offline 立即启动重连循环、online 立即试一次——OS 级网络断开/恢复秒级感知，零协议改动。不引应用层心跳帧：浏览器 WS API 不暴露 ping/pong（CORE-06 的 5s ping 前端不可见）、空闲终端无 OUTPUT 流量，「多久没收到消息」判据在浏览器侧结构性不成立。黑洞场景（无 RST 无事件）退化为 TCP 超时后重连——风险接受
- **D-05:** 重连成功首屏 = **term.clear() 清屏 + 服务端复用 SIGWINCH 强制重绘**（P5 D-11 挂点延伸到重连 attach 路径）——全屏程序秒级重绘干净画面；行内 shell 历史交 tmux/herdr（ROADMAP 既定分工）。不保留旧 buffer：重连窗口期错过的输出形成断层，全屏程序增量重绘花屏（G-05-1 同类风险）
- **D-06:** owner 重连**不加豁免**——按新 attach 走 P5 D-06/D-07 既定递补语义（原 owner 降级 ro 入队），文档明示「重连不恢复写权限」。CORE-05 承诺边界刻意收窄 = 接回同一进程、输入输出一致；不含身份恢复（恢复窗口需身份暂存/倒计时/双 owner 交接新状态机，与 P5 递补确定性冲突）
- **D-07:** 服务端重启场景 = **自然行为 + 文档明示**：share token 重启即废（P5 D-02）→ attach 失败落手动面板等用户拿新链接；凭据模式重连成功接回的是全新 shell。README 明示「重连目标 = 同一 URL 的当前进程，服务端重启后是全新会话」。不引入会话代际标识（generation id）

### 子进程退出终结帧（SESS-03）
- **D-08:** EXIT 帧 = **新 S→C 类型字节**（不复用 'E' Error 帧——用户裁决：终结语义独立于错误语义，类型字节承载）—— **Reversibility:** one-way — 类型字节是前后端公开协议契约（P2 D-01 纪律），发布后改值/改义破坏全部已部署客户端
- **D-09:** 载荷 = **`{"exit_code": N, "message": 人话}`**——exit_code 结构化供测试断言，message 前端直显；信号死亡 exit_code=-1、message 含信号名（服务端组文案唯一写口，前端不自维护信号文案表）—— **Reversibility:** one-way — 载荷形状同上公开契约（P2 D-02 加字段纪律约束后续演进）
- **D-10:** 广播序列 = **EXIT 帧 → 1000 正常关闭**（ROADMAP 锁定）；前端 EXIT 帧暂存（lastError 同款通道）→ onclose 1000 → 「Session ended」正文显示 message——面板结构不变，退出码/信号名人话进正文
- **D-11:** 重连循环遇服务端已退出的收口 = **Reconnecting 面板 hint 文案明示「若服务端已退出请从 shell 重启」**——零新逻辑；前端无法区分断网 vs 服务端退出（浏览器 connect 失败不暴露 refused/timeout 差异），两场景同一面板通吃

### --once 与无人退出模式（SESS-01/02）
- **D-12:** `--once` ≡ `--max-clients=1 --exit-when-empty=0` **语法糖**：CLI 保留独立 --once flag（ttyd 肌肉记忆），README 标明等价关系；第二客户端拒绝走既有 503 计数路径（P5 D-08 守卫链零新分支，409 不复活）—— **Reversibility:** one-way — CLI flag 公开契约（P2 D-15 纪律）
- **D-13:** 断开退出统一收口路径 = 注册表空（--once 唯一客户端断开 / --exit-when-empty 宽限到期仍空）→ **SIGHUP 进程组（P1 D-11 语义复活）** → Drain → exitf 以子进程退出码收口——exitf + sync.Once 单一收口纪律保持，两模式零分支差异；Phase 7 OPS-04 的信号可配化在此之前不提前设计
- **D-14:** SESS-02 flag = **`--exit-when-empty[=duration]` 单 flag 可选值**：裸写 = 最后一个客户端断开立即退出；`=duration` 给重连宽限（计时内任一端 attach 成功则取消退出）——Go flag 自定义 Value + IsBoolFlag 惯例；默认不开启（现状保持：无客户端时子进程继续运行，P5 推论）—— **Reversibility:** one-way — CLI flag 公开契约

### Claude's Discretion
- EXIT 帧类型字节具体值（建议 'X'，避开已占位 'T'/'P'；proto.go 常量 + 前后端注释手工对齐纪律沿用）
- EXIT 帧广播与慢客户端 outbox 的写序（lifecycle 现有 hubMu 快照 + 并行 Close 模式延伸；trySend 失败即走既有收口——进程已退出场景无需保帧）
- message 文案具体措辞（英文；exit code N / killed by signal SIGHUP；信号名提取经 ExitError.Sys().WaitStatus；非 ExitError 形态（Wait 返回其他错误）的 message 处理）
- online/offline 与 onclose 双触发的幂等（重连循环单例、防双循环；重连成功判定 = WELCOME 到达后退避清零）
- --exit-when-empty 宽限计时器挂点（detach 致注册表空 → 启动 timer；attach → 取消；timer 随会话消亡，零新 exitf 分支纪律）
- Reconnecting 面板 attempt 计数/倒计时格式（showStatus 三态内既有结构承载）
- UAT 场景矩阵（phase06.mjs：断线重连/清屏重绘/EXIT 帧退出码与信号死亡/--once 单客户端+断开退出/--exit-when-empty 立即与宽限取消）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 6 — 成功准则 3 条（--once 单客户端断开退出 / EXIT 帧含退出码 + 1000 / 断网 30s 恢复自动重连接回同一进程 + 无滚动回放文档明示）
- `.planning/REQUIREMENTS.md` — SESS-01、SESS-02、SESS-03、CORE-05 原文
- `.planning/PROJECT.md` — Key Decisions（GoTTY 共享进程模型、v1 不做会话保持=重连≠会话保持的边界、--once 属部署语义非会话保持）

### 前序 phase 决策
- `.planning/phases/02-protocol/02-CONTEXT.md` — D-01/D-02（帧类型空间与加字段纪律——EXIT 帧落此规范）、D-05（关闭码全集）、D-07（Error JSON 形状——EXIT 独立后不受影响）
- `.planning/phases/03-auth/03-CONTEXT.md` — D-10（auth_failed 静默重试一次先例——connect() 可重入入口，重连循环复用）
- `.planning/phases/04-frontend/04-CONTEXT.md` — D-18（beforeunload 默认开，重连落地后保留——重连成功 WELCOME 后按开关重注册先例见 main.ts:690）
- `.planning/phases/05-multi-client/05-CONTEXT.md` — D-02（share token 重启即废——D-07 依据）、D-06/D-07（owner 递补语义——D-06 依据）、D-10（1013 手动刷新边界——D-01 确认保持）、D-11（SIGWINCH 挂点——D-05 复用）、D-12（标题随重绘自然恢复先例）

### 调研结论
- `.planning/research/PITFALLS.md` — Pitfall 2（读路径禁 deadline——重连后新连接保活路径不变）、Pitfall 4（计数器/map 防单调增长——宽限计时器/重连计数纪律）

### 现状代码（扩展点）
- `internal/server/server.go` — lifecycle()（955-995：EXIT 帧广播挂点 = Drain 后、并行 Close 前）；terminate/exitf + sync.Once（1000-1004 单一收口）；registry/detach（注册表空检测 → 宽限计时器挂点）
- `internal/server/clients.go` — 注册表结构与 max-clients 503 计数路径（--once 语法糖展开点）
- `internal/proto/proto.go` — 类型字节常量区（'T'/'P' 占位注释）+ 帧编解码函数先例（EXIT 帧落此）
- `cmd/wesh/main.go` — parseArgs（--once/--exit-when-empty flag + 语法糖展开 + 启动校验矩阵扩展）
- `web/src/main.ts` — connect()（688-757：可重入入口，重连循环/退避/online-offline 监听落此）；onclose 按码分派（706-753：1006 类进入重连分支、1000 分支 EXIT message 显示、1013 维持手动）；lastError 暂存通道（643-648，EXIT 同款）；showStatus 三态面板
- `internal/pty/` — Session（SIGWINCH 挂点 P5 D-11，重连 attach 路径复用；Drain D-12 语义）
- `web/uat/phase02.mjs/phase05.mjs` — UAT harness 模式（phase06.mjs 同款；断线/重连场景驱动形态参照 phase05-flood-driver.mjs）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `connect()`（main.ts:757）— auth_failed 静默重试一次已验证的可重入入口；重连循环直接复用（fetch ticket → new WebSocket → Hello 全链不变）
- `showStatus()` 三态面板（main.ts）— Reconnecting 面板复用，零新 UI 组件
- `lastError` 暂存通道（main.ts:643-648）— EXIT 帧暂存同款模式
- `lifecycle()` 广播形态（server.go:974-988）— hubMu 快照 + 并行 Close；EXIT 帧在快照循环内先 send 后 Close 的挂点既有
- `throttleStore` 退避参数族（1s×2 封顶 30s）— 前端重连退避同族参数
- P5 D-11 SIGWINCH 挂点（pty.Session TIOCGPGRP → kill 进程组）— 重连 attach 完成回调复用
- `exitf` 注入 + `sync.Once` 收口（server.go:1000-1004）— 新终结触发源（注册表空）只进 terminate 单一路径
- UAT harness（web/uat/phaseNN.mjs）— Node 原生 WS 零依赖脚本，phase06.mjs 断线/重连场景同款

### Established Patterns
- **帧常量前后端手工对齐**（proto.go ↔ main.ts 注释互相指路）— EXIT 类型字节两侧同步
- **守卫区顺序敏感 + Accept 前零 WS 资源分配** — --once 第二客户端 503 走既有计数路径，不开新闸
- **CLI flag 全名无短选项 + 启动校验矩阵 fail-fast**（P2 D-15 / P3）— --once/--exit-when-empty 同纪律；--exit-when-empty 与 --max-clients=1 组合冲突校验（语法糖展开后等价判定）
- **exitf + sync.Once 单一收口** — 宽限计时器到期只调 terminate，不加新 exitf 分支
- **ro/rw 全员广播无分档** — EXIT 帧全员同帧（终结无权限语义）
- **logEvent 三要素单行事件** — 断开退出触发（once/empty）、宽限计时启动/取消事件落此

### Integration Points
- `server.go lifecycle()` — Drain 后、并行 Close 前插入 EXIT 帧广播（exit_code 从 sess.Wait 返回提取；信号死亡 ExitCode()=-1 → message 组信号名）
- `server.go detach/registry` — 注册表变空检测 → --exit-when-empty 宽限计时器启动/取消；--once 展开后等价判定走同一挂点
- `cmd/wesh/main.go` — --once（语法糖展开 max-clients=1 + exit-when-empty=0）与 --exit-when-empty[=duration]（自定义 Value+IsBoolFlag）；显式组合与语法糖冲突的校验进启动矩阵
- `web/src/main.ts` — 重连状态机（idle/reconnecting 单例循环）、online/offline 监听、重连成功 term.clear() + 状态面板清除、EXIT 帧暂存与 1000 分支 message 显示、1013 分支保持手动不变
- `proto.go` — EXIT 类型字节常量 + ExitPayload{ExitCode, Message} 编解码 + 前后端注释对齐

</code_context>

<specifics>
## Specific Ideas

- **新类型字节而非 Error 帧扩展的用户裁决**：终结语义独立于错误语义——子进程正常退出（exit 0）不是"错误"，不该挤进 Error 帧的 code 空间；类型字节承载语义独立性（D-08）
- **--once 是 ttyd 肌肉记忆的显式回归**：Phase 1 的单次语义在 P5 被多客户端推论终结，本 phase 以显式 flag 形式请回——默认行为（断开不退出）不变，--once 是部署语义的显式选择
- **CORE-05 承诺边界刻意收窄**：重连承诺 = 接回同一进程、输入输出一致；不承诺恢复写权限（D-06）、不承诺恢复旧画面（D-05 清屏）、不承诺服务端重启后的会话连续性（D-07）——收窄的每一项都对应一个被否决的状态机
- **无限重试的底气来自流量量级**：30s 一次重试对个人运维可忽略；「标签页放着，回来已接回」优先于「防僵尸标签页」
- **清屏+重绘与 G-05-1 同源认知**：陈旧 buffer + 增量重绘 = 花屏风险（相对寻址流），与 05-10 会话尺寸下发裁决是同一类问题的同一解法哲学

</specifics>

<deferred>
## Deferred Ideas

- **顶部状态条重连 UI**（不遮冻结现场的 Reconnecting 形态）— 后续迭代；本 phase 复用全屏面板（D-03）
- **会话代际标识（generation id）**——服务端重启后重连成功时提示「这是新会话」— D-07 裁决以文档明示替代；若用户反馈混淆再评估
- **--exit-when-empty 宽限默认值的负载标定** — Phase 9（与 outbox/限速参数同批回填）
- **1001 优雅下线发送路径** — Phase 7（P2 D-08 同批占位）
- **新 flag（--once/--exit-when-empty）配置文件收口** — Phase 7 OPS-09
- **断开退出事件进 metrics/审计日志**（once/empty 触发计数） — Phase 8 OPS-07/OPS-08

</deferred>

---

*Phase: 6-session-lifecycle*
*Context gathered: 2026-08-23*
