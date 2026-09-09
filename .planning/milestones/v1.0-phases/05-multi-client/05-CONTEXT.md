# Phase 5: 多客户端共享 - Context

**Gathered:** 2026-08-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 5 把单客户端服务端升级为多客户端共享：多个 WS 客户端同时 attach 同一 PTY 会话、输出实时扇出（MULTI-01）；写权限可配 all/owner（MULTI-02）；慢客户端有界 outbox 写满 1013 踢出不拖累他人（MULTI-03/RES-04）；resize 最小公共矩形仲裁（MULTI-04）；启动打印 ro/rw 两条分享链接即打即用（MULTI-05）；每客户端输入限速（RES-02）与最大并发数满员拒绝（RES-03）。CR-01 完整背压修复（有界输入队列 + 写 goroutine）落本 phase（PROJECT.md Key Decisions）。

**In scope (from ROADMAP):** MULTI-01, MULTI-02, MULTI-03, MULTI-04, MULTI-05, RES-02, RES-03, RES-04；每客户端有界 outbox + 专属 writer（Actor 只做 try_send）；全体可写客户端阻塞时停读 PTY 的全局信用；resize 防抖 50ms 与 1000×1000 钳制；x/time/rate 输入限速；ticket 按模式分签发 ro/rw。

**Out of scope (本阶段不做):** 断线自动重连（Phase 6 CORE-05）、--once/所有客户端断开退出（Phase 6 SESS-01/02）、EXIT 终结帧（Phase 6 SESS-03）、1001 优雅下线（Phase 7）、配置文件收口新 flag（Phase 7 OPS-09）、outbox 深度/踢出计数进 metrics（Phase 8 OPS-07）、参数标定回填（Phase 9）。

**已锁定不重复决策：** GoTTY 共享进程模型（PTY 随服务端启动、ring 回放/会话保活取消出 v1）；outbox 有界 + 1013 踢出 + 重连从最新输出看起；resize 仲裁算法（≥2 一律 min(cols)×min(rows)、2→1 恢复 last-wins、50ms 防抖、1000×1000 钳制——ARCHITECTURE §2.9 "owner 跟随"作废）；全局信用停读 PTY；x/time/rate 输入限速与 outbox 参数走常量纪律（P2 D-10，负载测试标定 Phase 9 回填，Phase 7 配置文件收口）；1013 常量占位启用（P2 D-08）；`permission_denied` code 入表挂账（P3 deferred）；P2 D-02 加字段不动协议纪律；`--client-option`/`--osc52` 等 P4 契约不变。**多客户端必然推论（roadmap 锁定项的直接结果，非新决策）：** P1 D-11 单次语义终结——任何客户端断开不再触发 exitf/SIGHUP，服务端生命周期只随子进程（D-10 唯一终结路径），无客户端时子进程继续运行、分享链接仍可 attach；"所有客户端断开退出"是 Phase 6 SESS-02 的可配模式。

</domain>

<decisions>
## Implementation Decisions

### 分享链接认证形态（MULTI-05）
- **D-01:** 链接 token = **独立第三认证通道**：启动时生成 ro/rw 两个 128bit 随机 token；持有效 token 可 GET 页面 + POST /api/attach 换 ticket（绕过 Basic）；无/错 token 时 P3 Basic 矩阵不变；与凭据共存（operator 走凭据、旁观者走链接）。P3 D-11 的"/api/attach 可选 mode 参数"预期**细化作废**——mode 由 token 绑定（ro token → ro ticket、rw token → rw ticket），/api/attach 不收 mode 参数 — **Reversibility:** one-way — 分享链接的认证语义是公开行为契约，改动破坏已分享出去的链接语义
- **D-02:** token **可复用至进程重启**；"一次性"落在 SEC-02 ticket 上（token → /api/attach → 一次性 ticket → Hello 链路不变）；ro/rw token 每轮启动重新随机生成——重启即废全部旧链接，吊销语义 = 重启
- **D-03:** URL 形态 **`/s/{token}/`** 路径段：服务端页面 GET 时即可校验 token 完成门禁（fragment 做不到这一点）；token 永不作 logEvent 参数（SEC-01 红线延伸）；README 明示反代访问日志会记录路径、建议脱敏 — **Reversibility:** one-way — URL 形态是用户书签/分享契约
- **D-04:** 启动打印链接的 host：bind 为 0.0.0.0/:: 时回填**首个非 loopback IPv4 接口地址**；具体 bind 地址原样使用；scheme 感知同 D-04 TLS 分支（https:// 当 TLS 启用）

### 写权限模式与 owner 归属（MULTI-02）
- **D-05:** `--writable` 保持总闸（不给 = 只打印 ro 链接，全员只读，现状语义零漂移）；新增 **`--write-policy=owner|all`** 默认 **owner**（安全默认哲学：旁观是被动场景、协作主动开启） — **Reversibility:** one-way — CLI flag 公开契约（P2 D-15 纪律）
- **D-06:** owner = **首个以 rw 身份完成 attach 的客户端**（无显式指定通道——服务端无本地终端 UI，operator 也是浏览器客户端）；owner 断线后**按 attach 顺序递补**
- **D-07:** owner 在位时后续 rw attach **降级 ro + 进递补队列**（Welcome mode=ro；owner 断线自动升格 rw 并推送通知）；复用现有 ro 前端形态（disableStdin + `[ro] ` 标题前缀），零新 UI 组件
- **D-08:** RES-03 开 **`--max-clients`** flag（容量策略是部署关切，与 P2 D-10 攻击面上限常量不同类）；满员在 Accept 前以 **HTTP 503** 拒绝（Attach 守卫区既有形态延伸）；默认初值负载测试标定、Phase 9 回填 — **Reversibility:** one-way — CLI flag 公开契约

### resize 仲裁参与集（MULTI-04 细化）
- **D-09:** 参与集**按写权限分层**：owner 模式仅 owner 尺寸参与（递补后新 owner 尺寸接管）；all 模式全部 rw 端取 min；无 --writable 纯 ro 会话全体 ro 端取 min（否则会话冻结 80x24）。ro 旁观者永不影响可写端尺寸。**P2 D-13 修订**："ro 放行 RESIZE"是单客户端语境，多客户端下 ro 端 RESIZE 不参与仲裁——服务端直接忽略，前端 ro 形态不发 RESIZE 帧。推论（算法直接结果，无需 S→C 尺寸下发帧）：min-rect 保证任何客户端窗口 ≥ PTY 尺寸，各端按自己窗口渲染、多余面积留白

### 踢出与新客首屏（MULTI-03 UX）
- **D-10:** 1013 踢出后前端 **onclose 按码分派 → showStatus 提示 + 手动刷新**（英文文案，"因接收过慢被断开"语义）；自动重连全部归 Phase 6 CORE-05，本 phase 不做任何自动重连（避免后台标签页重连→再被踢循环）
- **D-11:** 新客户端 attach 完成时，服务端向 PTY 前台进程组**显式发一次 SIGWINCH**（TIOCGPGRP 取 pgid → kill 进程组）强制全屏程序重绘——新客秒见画面；行内 shell 下次输出自然追上；TIOCGPGRP 失败/无前台进程组时静默降级
- **D-12:** 标题**保持纯前端解析**（P4 D-01 终局裁决）：服务端 OUTPUT 零拷贝不跑 OSC 状态机；新客标题随 SIGWINCH 重绘/下次 OSC 2 自然恢复；**'T' TITLE 帧终局不实现**（类型字节保留注释）

### 旁观端 OSC52 强制关（兑现 P4 deferred）
- **D-13:** 多客户端下 **ro 端（旁观者）强制不下发 `osc52:true`**，即使全局 --osc52 开启——rw 端按全局 --osc52 下发；prefs 按客户端 mode 分化（PITFALLS C5 对策的另一半：共享终端的 OSC52 不应劫持全体旁观者剪贴板）

### Claude's Discretion
- 递补升格的 S→C 通知通道（复用 'W' Welcome 帧运行期推送 vs 新类型字节；遵守 P2 D-01/D-02 纪律，前端处理 disableStdin/标题前缀切换）
- 分享 token 存储与比较形态（ticketStore 同款 mu+map + crypto/rand 128bit + subtle 常数时间比较；token 校验失败是否计入 throttleStore 同一 per-IP 计数器由 research 定）
- outbox 容量/水位/strikes 与输入限速 rate 参数初值、--max-clients 默认初值（research 负载测试标定，Phase 9 回填，P2 D-10 常量纪律）
- 首个非 loopback IPv4 接口选取策略（多接口排序）、/s/{token}/ 路由装配（Go 1.22 mux 模式）、token 无效时页面响应形态与文案
- max-clients 计数口径（半开是否计入——倾向 attach 成功后计入，与 halfOpen 正交）
- permission_denied code 入表的具体使用场景评估（owner 模式降级走 Welcome 非 Error；若无真实使用场景则保持占位注释不硬用）
- 1013 close reason 机器串与 stderr logEvent reason 命名（slow_consumer 类）
- 客户端注册表数据结构（attach 顺序 FIFO 递补队列）与 fan-out hub 的 goroutine 拓扑（ReadLoop 单读 → hub try_send → 每客户端 writer）
- 输入限速器（x/time/rate）超限时的行为（丢弃 vs 断开）由 research 按 RES-02 语义定

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 5 — 成功准则 3 条（双客户端输出一致 + all/owner 权限 / 慢客户端 1013 踢出 PTY 读循环永不阻塞 / 最小公共矩形 + ro/rw 分享链接即打即用）与 research flag（outbox 参数标定）；§Phase 6 — SESS/CORE-05 边界确认
- `.planning/REQUIREMENTS.md` — MULTI-01..MULTI-05、RES-02/03/04 原文
- `.planning/PROJECT.md` — Key Decisions（CR-01 完整背压留本 phase、GoTTY 共享进程模型、多客户端写入权限可配置）

### 调研结论
- `.planning/research/ARCHITECTURE.md` §2.8 — ticket 握手认证设计（token 换 ticket 链路）；§2.9 — 写权限与 resize 仲裁（"owner 跟随"表述已作废，算法以 MULTI-04 为准；outbox/fan-out 结构有效）
- `.planning/research/PITFALLS.md` — Pitfall 2（读路径禁 deadline）、Pitfall 4（计数器/map 防单调增长）、Pitfall 7（c.Read 不可并发）、C5（OSC52 旁观者劫持——D-13 兑现）、C10（尺寸钳制）

### 前序 phase 决策
- `.planning/phases/02-protocol/02-CONTEXT.md` — D-01/D-02（帧编码与加字段纪律）、D-05（关闭码全集，1013 本 phase 启用）、D-08（1013 占位启用）、D-10（上限常量纪律）、D-13（ro RESIZE——本 phase D-09 修订）
- `.planning/phases/03-auth/03-CONTEXT.md` — D-02（整站 Basic——本 phase token 闸门绕过形态）、D-08（统一 per-IP 节流计数器）、D-10（auth_failed 同口径无 oracle 纪律）、D-11（ticket mode 占位——本 phase D-01 细化）、deferred（permission_denied 入表挂账）
- `.planning/phases/04-frontend/04-CONTEXT.md` — D-01（TITLE 纯前端解析——本 phase D-12 终局）、D-12（--osc52 安全不对称——本 phase D-13 延伸）、deferred（旁观 OSC52 强制关——本 phase 兑现）

### 现状代码（扩展点）
- `internal/server/server.go` — Attach 守卫区（③ 409 单客户端门 → 客户端注册表 + max-clients 503 闸改造点）；onChunk 单写端 → fan-out hub 改造点；halfOpenCounter（max-clients 计数参照）；logEvent 三要素；pinger per-conn goroutine 模式；wsDisconnected/terminate（单次语义 → 多客户端生命周期改造点）
- `internal/server/tickets.go` — ticketStore（mu+map + 惰性清理 + subtle 纪律）——分享 token store 同款参照
- `internal/server/throttle.go` — throttleStore（D-08 统一计数器，token 失败计入评估）
- `internal/proto/proto.go` — Hello/Welcome 载荷扩展点（未知字段忽略纪律）；1013/permission_denied 常量启用点；前后端帧常量手工对齐纪律
- `cmd/wesh/main.go` — parseArgs（--write-policy/--max-clients flag + 启动校验矩阵扩展）；启动行打印（分享链接两行 + 接口 IP 回填）
- `internal/pty/` — Session（TIOCGPGRP/SIGWINCH 挂点、Resize、Drain D-12 语义）
- `web/src/main.ts` — 连接流程（/s/{token}/ 探测 → token → /api/attach body → ticket → Hello）；onclose 按码分派（1013 加入）；ro 形态（disableStdin/标题前缀/不发 RESIZE）；递补升格 Welcome 处理
- `web/uat/phase02.mjs/phase03.mjs/phase04.mjs` — UAT harness 模式（本 phase phase05.mjs 同款）

### ttyd 源码（缺陷对照面，不参考实现）
- `~/open_src/ttyd/` — 无多路复用（本 phase 改进点 #2）；CR-01 同步写阻塞对照

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ticketStore`（tickets.go）— mu+map、签发顺手惰性清理、crypto/rand 128bit + base64url 22 字符；分享 token store 直接参照（含 Pitfall 4 防单调增长纪律）
- `throttleStore`（throttle.go）— D-08 统一 per-IP 计数器，分享 token 校验失败计入的既有挂点
- `halfOpenCounter`（server.go:238-264）— acquire/release 恰好一次不变量；max-clients 计数器参照模式
- `logEvent`（server.go:600）— stderr 单行事件唯一出口；1013 踢出/满员拒绝事件落此；**token/ticket 永不入参红线**
- `pinger` per-conn goroutine 模式（server.go:550）— 每客户端 writer goroutine 的装配参照（ctx 挂点、终结收口纪律）
- `WelcomeFrame`/`ErrorFrame` 组帧（proto.go）— 递补升格通知与错误帧复用
- UAT harness（web/uat/phaseNN.mjs）— Node 原生 WS 零依赖脚本模式，phase05.mjs 双客户端场景同款

### Established Patterns
- **守卫区顺序敏感**（Attach Accept 前 HTTP 拒绝，⓪ Origin → ① 子协议 → ② halfOpen 429 → ③ 409）— max-clients 503 闸入此区；Zero WS 资源分配纪律
- **exitf 注入 + sync.Once 收口**（P1 硬约束）— 多客户端下 client 断开不再进 exitf 路径；子进程退出（D-10）成唯一终结触发源，广播关闭全部客户端后 exitf
- **原子门 + atomic.Pointer 连接持有** → 客户端注册表（set + FIFO 递补队列）演化；锁粒度由 planner 定
- **D-12 drain 语义**（无客户端时 ReadLoop 持续 drain 丢弃，防 PTY 内核缓冲写阻塞）— 多客户端下无客户端期间保持 drain（无 ring）
- **ro 边界服务端为真边界**（P2 D-13/D-14）— 多客户端下每客户端 mode 独立（owner rw、递补者 ro、旁观者 ro）：INPUT/RESIZE 门 per-client 判定
- **帧常量前后端手工对齐**（proto.go ↔ main.ts 注释互相指路）
- **CLI flag 全名无短选项**（P2 D-15）+ 启动校验矩阵 fail-fast（P3）— 新 flag 同纪律
- **前端 onclose 按码分派 + showStatus 三态面板**（P2/P4）— 1013 文案落此

### Integration Points
- `server.go Attach` — 409 单客户端门拆除 → 客户端注册表登记（mode 判定：ticket/token mode × write-policy → owner/rw/ro-queued）；max-clients 503 闸；per-client 状态（conn、mode、outbox、writer、pinger）装配
- `server.go onChunk` → fan-out hub：ReadLoop 单读不变，hub 对每客户端 outbox try_send，写满 → 1013 踢出；全体可写端阻塞 → 全局信用停读 PTY（RES-04）
- `server.go lifecycle/wsDisconnected` — 单次语义 → 多客户端：client 断开 → 注册表移除 + 递补/仲裁重算（不进 exitf）；子进程退出 → 广播关闭 + exitf
- `proto.go` — 1013 发送路径启用；permission_denied 入表；Welcome/升格帧扩展
- `tickets.go` — 分享 token store（ro/rw 两 token，启动生成）；/api/attach 接受 token 换 ticket（body 携 token，mode 绑定）
- `main.go` — --write-policy/--max-clients flag；启动打印分享链接两行（接口 IP 回填）；启动校验矩阵扩展
- `main.ts` — /s/{token}/ 路径探测与 token 提取；/api/attach body 携 token；1013 onclose 分派文案；ro 形态不发 RESIZE；升格 Welcome 运行期处理
- `pty.Session` — TIOCGPGRP + SIGWINCH 挂点（attach 完成回调）

</code_context>

<specifics>
## Specific Ideas

- **分享链接即打即用**：启动打印两行（ro/rw 各一），token 每轮启动重新随机——旧链接重启即废，用户预期"复制链接发给同事就能看"
- **token 路径进反代日志的显式取舍**：用户选择 /s/{token}/ 路径段（服务端可门禁页面）并接受日志暴露面——README 必须明示反代访问日志脱敏建议
- **1013 手动刷新的 phase 边界纪律**：用户明确选择不做任何过渡版自动重连——后台标签页重连→再被踢的循环比"手动刷新一次"更差；Phase 6 一次性把重连做对
- **SIGWINCH 的 UX 动机**：演示场景观众打开链接面对黑屏是第一印象杀手；一次 TIOCGPGRP+kill 换来 vim/htop 秒见画面
- **ro 不发 RESIZE 的流量考量**：旁观者窗口尺寸既然不参与仲裁，前端干脆不发 RESIZE 帧（省无谓上行流量），服务端忽略为兜底第二闸

</specifics>

<deferred>
## Deferred Ideas

- **完整断线自动重连**（指数退避 + 上限 + 手动入口）— Phase 6 CORE-05；本 phase 1013 仅提示 + 手动刷新（D-10）
- **--once / 所有客户端断开退出模式** — Phase 6 SESS-01/02；本 phase 服务端生命周期只随子进程（domain 节推论）
- **EXIT 终结帧多客户端广播形态** — Phase 6 SESS-03（类型字节已占住）
- **outbox/限速/max-clients 参数标定回填** — Phase 9（research flag：负载测试标定）
- **每客户端 outbox 深度与 1013 踢出计数进 metrics** — Phase 8 OPS-07（ROADMAP 已挂）
- **1001 优雅下线发送路径** — Phase 7（P2 D-08 同批占位）
- **新 flag 配置文件收口**（--write-policy/--max-clients）— Phase 7 OPS-09（D-10 纪律延续）
- **'T' TITLE 帧** — 终局不实现（D-12，P4 deferred 关闭，非延期）

</deferred>

---

*Phase: 5-multi-client*
*Context gathered: 2026-08-19*
