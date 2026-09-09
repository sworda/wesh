# Feature Research: wesh v1.1 — per-client 会话模式

**Domain:** Web 终端工具的 per-connection spawn 会话模型（ttyd-class 生命周期语义）
**Researched:** 2026-09-02
**Confidence:** HIGH（生命周期语义均为一手源码行号证据：ttyd 本地 clone 直读 + GoTTY/wetty 仓库代码检索；shellinabox 细节为二手生成文档，标 MEDIUM）

> **置信度标注说明**：本环境 `classify-confidence` seam 对全部 provider（含 `curated --verified`）退化返回 LOW（无 provider 注册表数据），故本文件按证据等级人工标注：一手源码行号直读 = HIGH；代码片段检索（含行号、可复核）= HIGH/MEDIUM；生成文档转述 = MEDIUM。digest 已按 seam 值入 research-store，正文标注以证据为准。

## 核心结论（先读这个）

1. **per-connection spawn 是品类默认，shared 才是 wesh 的差异化。** ttyd、GoTTY、wetty、shellinabox 四款同类工具全部为"每连接/每会话一进程"。wesh v1.0 的"启动时 spawn 一次、N 客户端共享"在品类内是独一份的架构。因此 per-client 模式的定位是**补齐品类表赌注**（ttyd 语义对齐），而非发明新东西；shared 模式继续作为差异化能力保留为默认。
2. **纠偏 v1.0 调研与 ARCHITECTURE.md 的"GoTTY 式共享进程模型"措辞**：经 GoTTY master 源码 + README 双重核实，GoTTY 实为 per-connection spawn（每 WS 连接 `factory.New(params)` → `pty.Start(cmd)`，handlers.go:114 → local_command.go:32-37；README「Sharing with Multiple Clients」原文："GoTTY starts a new process with the given command when a new client connects to the server. This means users cannot share a single terminal with others by default."）。v1.0 FEATURES.md 中 GoTTY 行"一个进程被所有客户端共享"为训练知识误记。wesh 的共享模型无品类先例可挂靠，docs 措辞应修正为自有设计（不影响任何代码）。
3. **调研问题的七项语义全部有 ttyd 行号证据兜底**，且与 PROJECT.md v1.1 目标特性完全一致（attach spawn / 断开 SIGHUP / EXIT 私有化 / 重连=全新进程 / resize 直通）。本调研的作用是确认这些目标即品类标准答案，并补充 wesh 可以做得更好的点（见 Differentiators）。
4. **wesh 协议面已严格优于 ttyd 等价物**：ttyd 子进程退出只发 1000/1006 关闭码、无退出码载荷（1006 还违反 RFC6455，issue #1395）；spawn 失败仅静默断连。wesh 的类型化 EXIT 帧（含 exit_code）与错误帧在 per-client 分支零改动直接继承——per-client 不需要任何协议变更。
5. **SEC-07（--auth-header）的 D-15 收窄理由在 per-client 分支结构性消失**："spawn 时无 HTTP 请求在手"对 attach 后 spawn 不再成立。ttyd 有 `TTYD_USER`（protocol.c:141-146）、gotty 有 `--pass-headers` 先例。是否借机支持子进程 env 注入属用户裁决项（见 Differentiators D5）。

## 同类工具行为矩阵（证据底座）

| 行为 | ttyd 1.7.7（本地源码直读） | GoTTY（yudai master / sorenisanerd fork） | wetty（butlerx main） | shellinabox |
|------|---------------------------|------------------------------------------|----------------------|-------------|
| spawn 时机 | 每 WS 连接，收到首条 JSON init 后（protocol.c:332-353 → spawn_process:154-165） | 每 WS 连接（handlers.go:114 → factory.New → pty.Start） | 每 socket.io 连接 `pty.spawn`（spawn.ts:14） | 每 HTTP session fork，session 结构持独立 pid+PTY（session.h:53-69） |
| 初始尺寸 | 首条 JSON 消息带 `columns/rows`，spawn 前写入 process（protocol.c:39-49,154-158） | 客户端 init 动态给定；`--width/--height` 可静态覆盖（options.go:30-31） | spawn opts 携带客户端 cols/rows | LaunchRequest 携带 width/height（launcher.c:525-540） |
| 客户端断开 → 子进程 | 立即 `pty_kill(process, sig_code)`，`uv_kill(-pid, sig)` = **进程组信号**（protocol.c:377-384；pty.c:160-165；子进程 setsid，pty.c:439）。默认 **SIGHUP**（server.c:172），`-s/--signal` 可配。**无宽限** | `slave.Close()` 杀该连接进程 | `socket.on('disconnect') → term.kill()`（spawn.ts / login.ts；node-pty kill 默认 SIGHUP） | session 超时（~45s）后进 graveyard 收割（session.c:68-108）——有宽限，因 AJAX 轮询无法即时感知断开 |
| 子进程退出 → 客户端 | 仅关**该客户端** WS：exit 0 → 1000，非 0 → 1006（违反 RFC6455，无退出码载荷）（protocol.c:89-90,96-113） | `ErrSlaveClosed` → 关该连接 | `term.onExit` → 通知并断开该 socket | session done，客户端轮询获知 |
| 重连语义 | 前端非 1000 关闭码自动重连（onSocketClose，xterm/index.ts:280-302）；重连发新 JSON init → **服务端 spawn 全新进程**；前端 `terminal.reset()` 清屏（index.ts:262-274）。**无 reattach** | sorenisanerd：`--reconnect/--reconnect-time` 前端自动重连 = 新进程 | 刷新页面 = 新 ssh/login 进程 | **唯一例外**：128-bit sessionKey + 服务端输出缓冲 → 刷新可 reattach 同一会话（session.c） |
| ro 模式含义 | `-R`：INPUT 服务端丢弃（protocol.c:312-313），**进程照常每人各 spawn 一个** | 默认 ro，`-w` 放行写 | 恒 rw（真实 SSH 登录） | n/a（login shell） |
| 并发/进程上限 | `--max-clients` 默认 0=不限；握手 FILTER 阶段预 Accept 拒绝（protocol.c:212-215），被拒者零 spawn 开销（注意用 `==` 比较） | sorenisanerd `--max-connection` 默认 0=不限，handler 入口闸 | 无（OS 进程表即上限） | 无 |
| `--once` / 空退 | `-o` 仅一客户端断开即退；`-q/--exit-no-conn` 全断开即退（server.c:113-114；protocol.c:208-211,386-398） | sorenisanerd `--once`：后到者 503 | n/a | n/a |
| 服务端是否随子进程退出而退出 | 否（per-connection 模型下子进程死亡只关本连接；除非 --once/-q 走 force_exit） | 否 | 否 | 否 |

**矩阵结论**：per-client 模式下的全部生命周期问题在品类内已有高度一致的答案——断开即杀（SIGHUP 进程组、无宽限）、子死仅关己、重连即新生、尺寸随首消息、连接闸即进程闸。shellinabox 的 reattach 是孤例且绑定 AJAX 轮询传输，不构成跟随理由（wesh V2-SESSION 领土）。

## 七项语义逐项裁决（对应调研问题）

1. **client disconnect → kill child? SIGHUP? grace?**
   裁决：**立即向该客户端子进程的进程组发 SIGHUP，无宽限**（ttyd parity：protocol.c:377-384 + pty.c:160-165 + server.c:172；wetty 同语义）。wesh 既有原语完整复用：`SignalGroup`（负 pid 进程组信号）+ setsid pgid==pid 不变量 + `--stop-signal` 可配。理由：WS 长连接能即时感知断开（不同于 shellinabox 轮询），宽限只会制造收割窗口与僵尸驻留；"断线不死"需求由子进程侧 herdr/tmux 承接（per-client 模式的存在意义恰是让 herdr 的多客户端仲裁正确工作）。
2. **child exit → 只关该客户端 WS?**
   裁决：**是**。向该客户端发私有 EXIT 帧（含 exit_code，信号死亡 -1）→ 1000 关连；服务端不退出、其他客户端无感知（ttyd parity：protocol.c:96-113 只关本 wsi）。wesh 的 EXIT 帧比 ttyd 的 1000/1006 无载荷方案严格更强，零改动继承。前端收到 EXIT 显示终态面板不自动重连（1000 不在 1006-only 重连触发面内，与 shared 模式一致）。
3. **reconnect → 新进程还是 reattach?**
   裁决：**新进程**（ttyd 语义，PROJECT.md 既定）。前端在 per-client 模式下重连成功时 `terminal.reset()` 清屏（ttyd parity：index.ts:267）——旧屏幕内容对新进程无意义，残留反而误导；画面恢复交给子程序重绘/herdr attach。reattach 显式不做（见 Anti-Features A1）。触发面维持 v1.0 的 1006-only + 1s×2 封顶 30s 退避不变（ttyd 是"非 1000 即重连"更宽，wesh 决策更保守，不因本模式改变）。
4. **initial terminal size → Hello cols/rows 作 spawn 尺寸?**
   裁决：**是**（ttyd parity：首条 JSON 消息带 columns/rows 再 spawn，protocol.c:332-353）。wesh Hello 已携带 `cols/rows` 且钳制 [1,1000] 既有；per-client 分支直接作为 `pty.Session.Start` 的 winsize，Welcome 回显自有尺寸，无 'W' 约束帧（单客户端独占尺寸，无仲裁对象）。
5. **resource limits → max-clients 即进程上限?**
   裁决：**是，且闸位置保持握手前置**（ttyd parity：protocol.c:212 预 Accept 拒绝，被拒客户端零 spawn 开销）。per-client 下客户端数==进程数 1:1，`--max-clients` 自然兼任进程闸；既有 per-IP 半开上限 8（429）、认证失败节流继续构成第一道防线。**加固点**：ttyd 闸在 ESTABLISHED、spawn 在 JSON_DATA，中间有并发窗口且用 `==` 比较；wesh 应在 spawn 前于 hubMu 内复检计数（防御两个并发 attach 同时过闸）。资源标定：每会话上界 ≈ 32KiB 读缓冲 + 512KiB outbox + 256KiB inputQ ≈ 0.8MiB + 子进程本体，max-clients=20 → 服务端侧最坏 ~16MiB，线性可预期。
6. **ro 在 per-client 下的含义?**
   裁决：**ro = 对自有进程的服务端输入门控**（ttyd parity：protocol.c:312-313 丢弃 INPUT 但照常 spawn）。ro 不再意味着"看别人操作"——那是 shared 模式语义。当子进程是多路复用器（`herdr attach` / `tmux new -A`）时，ro 访客的独立进程仍汇聚到同一底层会话，输入被 wesh 门控丢弃 → 实际获得"围观同一会话"体验，语义经子程序间接保留。此点必须进文档（见 T11）。
7. **分享链接在每人独立进程下的语义?**
   裁决：**ro/rw 分享链接从"同会话视图凭证"变为"按权限级别的独立进程入场券"**。机制零改动（token 核销、mode 绑定照旧），语义改变：子进程为普通 shell 时，每位开链接者得到互不可见的私有 shell；子进程为 herdr/tmux 时，独立进程经多路复用汇聚，分享体验保留且 herdr 的 last-attach-wins/per-client area 渲染恢复正确——这正是 driving scenario。无品类参照物（gotty 的 `--permit-arguments` 是注入面、ttyd 的 `?arg=` 已被 wesh v1 砍掉），属 wesh 自有语义设计点，文档为硬性配套。

## Feature Landscape

### Table Stakes（per-client 模式必备，缺即破）

| # | Feature | Why Expected | Complexity | Notes |
|---|---------|--------------|------------|-------|
| T1 | `--session-mode=shared|per-client` flag + TOML 键，默认 shared | 模式入口；零回归承诺要求默认不变 | LOW | 复用 config.go 既有 Visit/合并/DisallowUnknownFields 机制 + 启动校验矩阵新枚举值 |
| T2 | attach 后 spawn：Hello 认证通过 → `pty.Session.Start`，Hello cols/rows 作初始 winsize | ttyd/wetty/gotty 全部如此；herdr 场景前提 | MEDIUM | `pty.Session` 原样复用（env 白名单/cwd/TERM/降权/收割全继承）；spawn 从 server.New 移至 Attach 后；**spawn 失败回类型化错误帧**（ttyd 只能静默断连，wesh 协议免费变强） |
| T3 | 断开 → SIGHUP 进程组立即杀，无宽限 | 裁决 1；ttyd/wetty 一致答案 | LOW | SignalGroup + setsid 不变量既有；默认 SIGHUP，随 `--stop-signal` 可配 |
| T4 | 子进程退出 → 私有 EXIT 帧（含 exit_code）→ 1000 关该客户端；服务端不退 | 裁决 2；EXIT 帧机制既有 | LOW-MEDIUM | 广播改单播；lifecycle goroutine 从全局唯一变每会话一条 |
| T5 | 重连 = 全新进程 + 前端 `terminal.reset()` | 裁决 3；ttyd parity | MEDIUM | 服务端零新增（新 attach 即新 spawn）；前端需感知模式——Welcome/prefs 下发 session mode，重连分支调 reset() |
| T6 | resize 直通：RESIZE → 本会话 TIOCSWINSZ，无仲裁 | per-client 无仲裁对象 | LOW | 钳制 [1,1000] 既有；仲裁器在 per-client 分支不装配（PROJECT.md 既定） |
| T7 | ro 输入门控 + 每客户端输入限速保留 | 裁决 6；两闸均在读循环路径，模式无关 | LOW | 零改动继承；ro 语义文档化（T11） |
| T8 | max-clients 兼任进程上限：握手 503 闸保留 + spawn 前 hubMu 内复检 | 裁决 5；防 ttyd 式 `==` 闸 + 异步 spawn 窗口的并发超编 | LOW-MEDIUM | 闸机制既有，复检为新增防御纵深 |
| T9 | `--once` / `--exit-when-empty` 语义适配：触发条件不变（计数归零），终结时杀**全部** per-client 进程组；子进程退出不再联动服务端退出 | 生命周期正确性；SIGHUP 提前到断开时刻后，净行为与 v1.0 等价 | MEDIUM | 优雅关停同理乘 N：1001 广播 → 对每进程组执行 stop-signal 序列；收割器每会话一条（pidfd/kqueue 平台路径既有） |
| T10 | 慢客户端保护保留：每客户端有界 outbox + 持续过载 1013 踢 | 每客户端 outbox 是既有架构，per-client 天然适用 | LOW | 全局信用门不装配（PROJECT.md 既定）；per-client 背压简化为"outbox 满→停读该 PTY→内核缓冲满→子进程写阻塞"自然反压，停读优化见 D6 |
| T11 | 模式语义文档段：分享链接=独立进程入场券、ro=自有进程输入门控、配合 herdr/tmux 时经多路复用汇聚 | 裁决 6/7；无语义文档 = 用户必然误判（v1.0 README 的分享叙事全部建立在 shared 模型上） | LOW | README + CONFIGURATION.md 各一段；herdr 配方示例 |

### Differentiators（超出品类标准的加分项）

| # | Feature | Value Proposition | Complexity | Notes |
|---|---------|-------------------|------------|-------|
| D1 | 私有 EXIT 帧带退出码 + spawn 失败原因可达客户端 | ttyd 只给 1000/1006 无载荷（1006 还违反 RFC6455）、spawn 失败静默断连；wesh 用户能区分"命令跑完"与"命令崩了" | LOW | 协议既有，per-client 分支继承即得；UAT 断言点 |
| D2 | ro/rw 分享链接在 per-client 下成为"按权限独立 spawn 凭证" | 品类无等价物；gotty/ttyd 的 URL 传参方案均为已核实注入面 | LOW | 机制零改动，价值在语义设计与文档（T11 的差异化面） |
| D3 | 安全默认全套继承：env 白名单/TLS/Origin/节流/ticket/读上限 | ttyd 已核实缺陷清单（env 泄露、预认证放大等）的对立面；per-client 经 `pty.Session.Start` 复用零成本获得 | LOW | 已有能力，仅需在 per-client 路径复测（测试矩阵加一维） |
| D4 | /metrics + 审计日志 per-client 粒度：活跃进程数 gauge、spawn/kill 计数、日志事件带 pid 归因 | 品类无一有 metrics/结构化审计；per-client 下"有几个进程在跑"是一等运维问题 | MEDIUM | 遵守零身份 label 红线（只用进程计数，不带任何用户标识）；事件加 pid/session 序号字段 |
| D5 | 候选：per-connection HTTP 上下文注入子进程 env（如 `WESH_REMOTE_USER`） | D-15 收窄理由（shared 模型 spawn 时无 HTTP 上下文）在 per-client 分支结构性消失；ttyd `TTYD_USER`、gotty `--pass-headers` 为先例 | MEDIUM | **需用户显式裁决重开 D-15**；白名单键名、值清洗沿用 SEC-07 sanitize；v1.1 可不做，但应在 phase CONTEXT 阶段裁决而非遗留 |
| D6 | per-PTY 停读/续读背压（ttyd `pty_pause/pty_resume` parity） | 比"踢掉重连"更平滑：慢一点不断线 | MEDIUM | T10 的自然反压已可用，此为体验优化；`ReadLoop` 需加可暂停控制点 |

### Anti-Features（看似美好、实则挖坑，显式不做）

| # | Feature | Why Requested | Why Problematic | Alternative |
|---|---------|---------------|-----------------|-------------|
| A1 | 重连接回同一 per-client 进程（shellinabox 式 sessionKey + 服务端输出缓冲 reattach） | "断网回来接着干"直觉诉求 | 等于把 V2-SESSION（会话保持/滚动回放）偷渡进 v1.1：服务端需为每进程持滚动缓冲，内存×N、收割复杂度、与 v1"会话保持由 tmux/herdr 覆盖"既定决策正面冲突 | 子进程跑 herdr/tmux——per-client 模式的存在意义正是让多路复用器正确工作 |
| A2 | 断开后 linger 宽限再杀子进程 | "误刷新不至于丢进程" | 半吊子会话保持：宽限窗内进程无客户端可交互却占资源，收割竞态面增大；ttyd/wetty 均立即杀 | 同 A1：herdr/tmux 承接持久性；herdr 场景下 wesh 杀掉的是 herdr client，herdr-server 会话本就活着 |
| A3 | 运行期/按 URL 切换模式或按 URL 选命令 | "一个实例两种模式都想要" | `?arg=` 注入面前车之鉴（ttyd protocol.c:241-249 已核实，v1 砍掉）；运行期切模式使全部生命周期不变量双份化 | 起两个 wesh 实例不同端口（单二进制零成本） |
| A4 | per-client 下服务端滚动回放缓冲 | 重连后想看到之前输出 | 同 A1，V2-SESSION 领土；且 per-client 每会话一份缓冲，内存放大更直接 | 子程序重绘（herdr/tmux attach 即全量重绘） |
| A5 | per-client 设为默认模式 | "ttyd 用户迁移更习惯" | 违背 v1.0 零回归承诺与 README 既定分享叙事；shared 模式（真·多人同屏协作）是 wesh 差异化本体 | shared 保持默认，per-client 显式 opt-in（T1 已定） |
| A6 | per-client 分支仍装配 fan-out hub/resize 仲裁器/owner 递补"以防以后用得上" | 代码复用惰性 | 1:1 关系下这些组件的竞态面（hubMu/信用门/参与集）是纯负债；PROJECT.md 已明确不装配 | 分支内 1 客户端 ↔ 1 PTY 直通；shared 组件零改动保留在原分支 |
| A7 | 为省进程让 ro 访客共享一个进程 | "ro 又不输入，合一个得了" | 直接重新引入 driving bug（移动端 attach 缩小所有人面板）；混合模型使两种模式语义都不再成立 | ro 访客各 spawn 各的（T7）；进程开销由 T8 上限管控 |

## Feature Dependencies

```
T1 session-mode flag
   └──gates──> T2 attach spawn（Hello cols/rows 初始尺寸）
                    ├──enables──> T3 断开杀进程组
                    ├──enables──> T4 私有 EXIT
                    ├──enables──> T6 resize 直通
                    ├──requires──> T8 spawn 前计数复检
                    └──feeds────> T9 --once/exit-when-empty 终结 N 进程组

T5 前端重连 reset ──requires──> Welcome/prefs 下发模式（T2 副产品）
T7 ro 门控 + 限速 ──独立保留──（读循环路径，模式无关）
T10 慢客户端保护 ──独立保留──（每客户端 outbox 既有）
T11 语义文档 ──requires──> T1-T7 语义冻结
D4 metrics/审计粒度 ──enhances──> T2/T3/T4（事件源）
D5 env 注入 ──requires──> 用户裁决重开 D-15 ──且──> T2（attach 时 HTTP 上下文在手）
herdr E2E UAT ──requires──> T1-T6 全部
```

### v1.0 既有特性的 keep / degrade / vanish 映射（下游关键消费点）

**Vanish（per-client 分支不装配，shared 分支零改动保留）：**

| v1.0 特性 | per-client 下的去向 |
|-----------|---------------------|
| resize 仲裁（min-rect / last-wins / 防抖 / 参与集分层） | 不装配——单客户端独占尺寸，直通 TIOCSWINSZ（T6） |
| owner 递补升格 / 写权限仲裁矩阵 | 不装配——每进程一客户端，ticket 绑定 mode 直接生效 |
| fan-out hub（输出扇出 ×N） | 不装配——1:1 直通管道 |
| 全局信用门（hubCond） | 不装配——自然反压 + 1013 踢（T10） |
| 'W' 约束帧（异尺寸 min-rect 视口约束） | 不发——Welcome cols/rows 即自有尺寸 |
| 重连接回同一进程（CORE-05 行为本体） | 不存在——重连即新进程（T5） |

**Degrade（机制保留，语义改变）：**

| v1.0 特性 | 语义变化 |
|-----------|----------|
| EXIT 帧广播 | 广播 → 私有单播（T4）；服务端退出与子进程死亡解耦 |
| --once / --exit-when-empty | 触发（计数归零）不变；终结目标 1 进程组 → N 进程组（T9） |
| 优雅关停 stop-signal 序列 | 单进程组 → 每进程组各执行一遍（T9） |
| ro/rw 分享链接 | "同会话视图凭证" → "独立进程入场券"（裁决 7） |
| max-clients | 连接闸 → 兼任进程闸（语义增强，机制不变 + spawn 复检） |
| Welcome cols/rows | "会话尺寸（可能被仲裁约束）" → "自有尺寸"，前端处理不变 |

**Keep（模式无关，零改动）：** ticket 核销（单次/60s/mode 绑定）、认证失败节流、Origin 白名单、per-IP 半开上限 8、TLS + 安全头、读上限两档 4KiB/16KiB、ping/pong 保活、关闭码纪律 {1000,1002,1008,1009,1013}、1006-only 重连触发 + 退避、env 白名单、cwd/TERM/stop-signal 配置、降权、base-path、UNIX socket、/healthz、/metrics 骨架、审计日志骨架、--client-option/prefs/query 覆盖、标题同步（ro 恒 `[ro] ` 前缀——per-client 下仍按 ticket mode 生效）、剪贴板/超链接/Unicode addons、自定义首页、--open。

## MVP Definition

### Launch With（v1.1 最小闭合）

- [ ] T1 模式 flag + TOML 键（默认 shared）——模式入口，一切前提
- [ ] T2 attach spawn（初始尺寸 + spawn 失败错误帧）——模式本体
- [ ] T3 断开 SIGHUP 进程组——生命周期闭环（否则僵尸进程泄漏）
- [ ] T4 私有 EXIT + 1000——ttyd parity 底线（且 wesh 协议免费更强）
- [ ] T6 resize 直通——herdr 场景"移动端不再缩面板"的另一半（per-client area 渲染需正确尺寸）
- [ ] T5 前端重连 reset——不做则重连后旧屏残留误导，ttyd parity
- [ ] T7/T8/T10 保留项——零改动继承 + spawn 复检一点新增
- [ ] T9 --once/exit-when-empty 适配——生命周期正确性，不做则终结路径杀不干净
- [ ] T11 语义文档——分享语义变化不文档化 = 功能缺陷
- [ ] herdr 场景 E2E UAT——driving scenario 实证（协议层 + 协议层断言 is_foreground/resize 行为）

### Add After Validation（v1.1.x）

- [ ] D4 metrics/审计 per-client 粒度——metrics 系列可后置；日志事件带 pid 成本极低可提前混入 MVP
- [ ] D6 per-PTY 停读背压——T10 自然反压已够用，体验优化
- [ ] D5 WESH_REMOTE_USER env 注入——待用户裁决，独立小变更

### Future Consideration（v2+）

- [ ] A1/A4 的重连 reattach 与滚动回放——V2-SESSION 统一评估，不在本里程碑开口子
- [ ] per-client + shared 混合部署配方（反代按路径分流到两实例）——文档级，v2 视需求

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| T1 模式 flag | HIGH | LOW | P1 |
| T2 attach spawn + 初始尺寸 + 失败错误帧 | HIGH | MEDIUM | P1 |
| T3 断开 SIGHUP 进程组 | HIGH | LOW | P1 |
| T4 私有 EXIT | HIGH | LOW-MEDIUM | P1 |
| T6 resize 直通 | HIGH | LOW | P1 |
| T5 重连 reset | MEDIUM-HIGH | MEDIUM | P1 |
| T7 ro 门控 + 限速保留 | HIGH | LOW | P1 |
| T8 进程闸 + spawn 复检 | HIGH | LOW-MEDIUM | P1 |
| T9 once/exit-when-empty 适配 | MEDIUM-HIGH | MEDIUM | P1 |
| T10 慢客户端保护（自然反压版） | HIGH | LOW | P1 |
| T11 语义文档 | HIGH（防误判） | LOW | P1 |
| herdr E2E UAT | HIGH（driving scenario 验收） | MEDIUM | P1 |
| D4 metrics/审计粒度 | MEDIUM | MEDIUM | P2 |
| D6 per-PTY 停读背压 | MEDIUM | MEDIUM | P2 |
| D5 env 注入（裁决后） | MEDIUM | MEDIUM | P2（待裁决） |
| D1/D2/D3 继承型差异化 | HIGH | LOW | 随 P1 免费落地，UAT 断言锁定 |

**Priority key:** P1 = v1.1 发布必需；P2 = 应尽快跟进；P3 = 未来考虑（本表无独立 P3 项，A 系列反特性即"永不"清单）。

## Sources

| 来源 | 内容 | 置信度 |
|------|------|--------|
| 本地 ttyd 1.7.7 源码（`~/open_src/ttyd`）直读 | protocol.c（spawn/断开/退出/--once/max-clients/ro/初始尺寸）、pty.c（pty_kill 进程组、setsid）、server.c（默认 SIGHUP、-s/--signal）、html/src xterm/index.ts（重连+reset） | HIGH（一手行号证据；与项目 2026-08-13 既有审计互洽） |
| yudai/gotty master（grep_app 代码检索） | README「Sharing with Multiple Clients」、server/handlers.go:114、backend/localcommand/local_command.go:32-37、factory.go:46 | HIGH（多文件互证 + README 原文） |
| sorenisanerd/gotty（grep_app 代码检索） | server/options.go:21-31（--once/--max-connection/--reconnect/--width/--height）、handlers.go（once CAS 闸 + 503） | HIGH |
| butlerx/wetty main（grep_app 代码检索） | src/server/spawn.ts（pty.spawn、disconnect→term.kill）、src/server/login.ts（同模式） | MEDIUM-HIGH（片段直读非全文件；node-pty kill 默认 SIGHUP 为 API 文档常识级，未本次复核） |
| shellinabox deepwiki 生成文档（引用 session.c/launcher.c 行号） | session 结构/sessionKey 128-bit/输出缓冲 reattach/45s 超时 graveyard | MEDIUM（二手生成文档，结构事实与公认设计一致） |
| GitHub issue 检索 | tsl0922/ttyd#1395（非零退出发 1006 违反 RFC6455）、#1528（断开杀进程语义讨论）、#628（重连行为） | MEDIUM（issue 标题/摘要级，未逐楼精读） |
| wesh 项目内 | .planning/PROJECT.md（v1.1 目标、D-15/SEC-07 裁决记录）、docs/ARCHITECTURE.md、.planning/REQUIREMENTS.md | HIGH（项目权威文档） |

---
*Feature research for: wesh v1.1 per-client 会话模式*
*Researched: 2026-09-02（前版为 2026-08-13 v1.0 调研，已被本文件按里程碑覆盖；其中 GoTTY 行结论经源码核实证伪，见核心结论 2）*
