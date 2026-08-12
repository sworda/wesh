# Architecture Research — Web 终端共享系统（wesh / ttyd-class）

**Domain:** Web 终端共享工具（PTY over WebSocket，单静态二进制）
**Researched:** 2026-08-13
**Confidence:** HIGH（核心结论均经一手来源验证：ttyd 本地源码、coder Go 源码、Linux man page、GNU screen 手册、tokio/xterm.js 官方文档；个别系统内部细节为 MEDIUM，已逐条标注）

---

## 0. 先行结论（TL;DR）

成熟系统解决"会话解耦 + 多客户端"存在一条清晰的能力光谱，wesh 的甜点位已经存在但被各占一半：

| 系统 | 会话存活于断连 | 多客户端 attach | 重连恢复手段 | 写权限模型 | 验证来源 |
|---|---|---|---|---|---|
| **tmux** | ✓（server 进程持有 pane/PTY） | ✓ | **服务端完整终端模拟**，按 grid 重渲染（精确） | 无细粒度（同 socket 属主） | HIGH（deepwiki 带源码行号） |
| **GNU screen** | ✓ | ✓（`-x`） | 同上（服务端屏幕状态） | **ACL r/w/x 每用户每窗口 + writelock 单写者锁** | HIGH（官方手册镜像） |
| **abduco / dtach** | ✓（每会话一个 detached 进程） | ✓ | **无**：不重放不模拟，靠应用自重绘（^L / dvtm） | `-r` 只读仅为客户端建议性，非安全特性 | HIGH（作者官网） |
| **coder reconnectingpty** | ✓（无连接超时后 kill） | ✓（activeConns map） | **64KiB 内存 ring 原始字节重放**（官方自称 "buggy"，优先委托给 screen 后端） | 无 | HIGH（v2.36.0 源码精读） |
| **Eternal Terminal** | ✓（etterminal 进程存活） | ✗（1:1） | **序号化可靠字节流**（BackedReader/Writer 重传丢失段） | — | MEDIUM-HIGH（deepwiki） |
| **mosh** | ✓ | ✗ | 服务端终端模拟 + **状态 diff 同步**（UDP） | — | MEDIUM（训练知识+检索摘要） |
| **shellhub** | ✗（活动会话不恢复，仅录像可回放） | ✗ | — | — | MEDIUM-HIGH（仓库分析） |
| **k8s remotecommand** | ✗（断连即容器内进程结束） | ✗ | — | — | HIGH（client-go 官方 API 文档） |
| **ttyd / gotty** | ✗（WS 关闭即 kill，源码已核实） | ✗ | 无 | 全局 `-W` 开关 | HIGH（ttyd 本地源码） |

**wesh 的定位 = abduco 的简洁进程模型 + coder 的 ring 重放 + screen 的写权限语义，并修复 coder 已验证的 fan-out 缺陷。** 不需要 tmux/mosh 的服务端全量终端模拟（实现一个 VT 解析器是巨大表面，超出 v1 收益）；ring 重放 + resize 触发的 SIGWINCH 重绘是 coder 已趟平的"够用"方案，其已知近似性要写进文档。

**ttyd 必须抛弃的三个结构性耦合（本地源码核实）：**
1. **连接即进程**：`pss_tty`（每 WS 连接的 lws 状态）直接持有 `pss->process`，`LWS_CALLBACK_CLOSED` 里 `pty_kill`（protocol.c:366-384）。解耦的第一步就是把进程所有权从连接状态中拿出来。
2. **持锁阻塞式 fan-out 的反面教材已在 coder 复现**：coder buffered.go 单读 goroutine 在**持有一把大锁**的情况下对每个 conn 做阻塞 `conn.Write`，代码内 TODO 自述应改为 channel-per-conn——一个慢客户端拖住所有人。ttyd 靠"一读一停"（read_cb 里 `uv_read_stop`，写完再 resume，pty.c:65-77）把问题变成了吞吐受限。
3. **每进程一个 waitpid 线程**（pty.c:398-417, 483）：N 个会话 = N 个阻塞线程。pidfd/kqueue 可以零线程并入事件循环。

---

## 1. Standard Architecture

### 1.1 System Overview

单进程、异步 IO、actor 模型。所有会话共享状态只被其 Session Actor 触碰，跨组件一律消息传递——从根上消灭 ttyd 的 pss 跨 lws/libuv 双域 UAF 风险类。

```
                        浏览器客户端 A(读写)  B(只读)  C(读写)
                              │        │        │
                              └────────┴────────┘
                                 HTTPS + WSS
                    (TLS 配置 / Origin 白名单 / per-IP 限流)
┌────────────────────────────────── wesh 单进程 ──────────────────────────────────┐
│                                                                                 │
│  ┌──────────────────────────── Gateway ────────────────────────────┐            │
│  │  HTTP: 内嵌前端 / /healthz / /metrics / POST /api/attach          │            │
│  │  WS:   upgrade、子协议协商 wesh.v1、Hello 首帧校验、协议编解码       │            │
│  └───────┬───────────────────────────────────────────────┬─────────┘            │
│          │ 验票/签发(常量时间比较,失败节流)                  │ attach→(session,mode)│
│  ┌───────▼────────┐                              ┌────────▼──────────┐           │
│  │  Auth Service  │                              │ Session Registry  │           │
│  │ 一次性 ticket  │                              │ id→Actor 句柄表    │           │
│  │ (单次/短 TTL)  │                              │ 空闲回收/全局上限  │           │
│  └────────────────┘                              └────────┬──────────┘           │
│                                                           │ spawn/attach/detach  │
│        ┌──────────────────────────────────────────────────┤                      │
│        ▼                                                  ▼                      │
│  ┌─────────── Session Actor #1（每 PTY 会话一个）──────────┐                       │
│  │ 状态: master fd / pidfd / ring buffer / clients / size  │                       │
│  │       / 写模式(all|owner|ro) / 退出墓碑(exit code)       │                       │
│  │ 输入路由(权限检查) · 输出 fan-out · 重放 · resize 仲裁    │                       │
│  └──┬───────────────┬───────────────┬─────────────────────┘                       │
│     │ outbox mpsc   │               │                       （每个 Client Conn:     │
│     ▼               ▼               ▼                        有界发件箱+专属 writer）│
│  Conn A writer   Conn B writer   Conn C writer                                    │
│                                                                                 │
│  ┌──────────── PTY Engine ────────────┐   ┌────────── Observability ─────────┐  │
│  │ forkpty/setsid · env 白名单 · cwd   │   │ 结构化日志 · metrics · 审计事件    │  │
│  │ uid/gid 降权 · rlimits · TIOCSWINSZ │   │ (每客户端队列深度/lag 踢出/字节数) │  │
│  │ pidfd(Linux)/kqueue(macOS) 退出监视 │   └───────────────────────────────────┘  │
│  └───────────────┬────────────────────┘                                           │
└──────────────────┼────────────────────────────────────────────────────────────────┘
                   ▼ fork/exec（进程组）
              子进程组: bash → vim/htop/...
```

### 1.2 Component Responsibilities（组件边界）

| 组件 | 职责（拥有什么） | 不知道什么（边界） | 典型实现 |
|---|---|---|---|
| **Gateway** | TLS、HTTP 路由、WS upgrade、帧编解码、消息长度上限、认证超时(5s)、per-IP 连接上限 | PTY、会话内部状态 | axum/tokio-tungstenite 或 Go net/http + coder/websocket |
| **Auth Service** | 凭据校验（时序安全比较）、一次性 ticket 签发/核销（单次使用、60s TTL、绑定会话与模式）、认证失败指数退避 | 会话内容 | 内存票表 + HMAC/随机 128bit |
| **Session Registry** | session_id→Actor 句柄；create-or-attach 决策；空闲超时回收（无客户端计时器）；全局会话/连接上限 | 协议帧、PTY 系统调用 | 一张并发安全 map + 定时器 |
| **Session Actor** | **每会话唯一权威**：PTY master fd、子进程 pid/pidfd、ring buffer、客户端集合及各自模式、窗口尺寸、写模式策略、退出墓碑 | HTTP/WS 细节（只见解码后的 typed message） | 单 task/goroutine + 邮箱（mpsc），内部零锁 |
| **Client Conn（每连接）** | 有界 outbox + **唯一**碰 WS 写端的 writer task、输入权限门、心跳、detach 清理 | 其他客户端、ring 内部 | 每连接 2 个 task（reader/writer）+ bounded mpsc |
| **Ring Buffer** | 每会话有界原始字节环（默认 256KiB–1MiB 可配；coder 用 64KiB 偏小），attach 时快照 | 谁在消费 | Vec+u64 写游标，O(1) append |
| **PTY Engine** | forkpty/setsid/控制终端、env 白名单、cwd、uid/gid、rlimits、O_NONBLOCK/CLOEXEC、TIOCSWINSZ、进程组信号、**pidfd(Linux 5.3+)/kqueue(macOS) 退出监视** | 上层会话语义 | 平台抽象层，fd 注册进事件循环，零额外线程 |
| **Observability** | 结构化日志、/healthz、metrics（会话数、每客户端 outbox 深度、lag 踢出数、字节吞吐） | 业务逻辑 | slog/zap/tracing + prometheus 或手写 /metrics |
| **Frontend（内嵌）** | xterm.js + fit/unicode11/web-links/clipboard，重连状态机（带 session_id 重 attach），PAUSE/RESUME 客户端流控，重放批量写入 | 服务端实现 | ttyd 前端已验证，改造即可 |

**边界规则：** Gateway↔Registry↔Actor 之间只传 typed message（Attach/Detach/Input/Resize/Output...）；PTY fd 不出 Actor；WS 写端只属于 Conn writer。这三条遵守后，并发 bug 的主战场（共享可变状态）在结构上不存在。

---

## 2. 设计决策专题（问题 → 决策 → 证据）

### 2.1 会话/进程解耦模型：控制面与数据面分离

**决策：单进程内"Registry + 每会话 Actor"，而非 abduco 的"每会话一个 detached 进程"。**

- abduco/dtach 的每会话一进程模型对 CLI 工具优雅（会话天然跨终端存活、socket 即句柄），但 wesh 的会话由本进程内嵌 HTTP/WS 服务暴露，没有必要再跨进程；进程内 Actor 获得同等解耦且省去 unix socket 权限/重建（abduco 需 SIGUSR1 重建 socket）的运维坑。
- tmux 证明的关键点不是"另一个进程"，而是**会话状态有一个唯一属主、客户端是可替换的渲染端**。wesh 的 Actor=属主，WS 连接=渲染端。
- **连接存活期间的行为全解耦**：WS 关闭只触发 `Actor.detach(conn)`；进程死活由会话策略决定（空闲超时、显式销毁、`--once` 类一次性会话）。
- **退出墓碑**：借鉴 abduco——子进程退出后会话不立即消失，保留 exit code + ring 终态，迟到的重连者能看到最后输出与退出码（abduco 的 `+` 会话语义），墓碑 TTL 可配。

### 2.2 PTY 生命周期管理

**决策：forkpty + setsid 语义 + 进程组信号 + pidfd/kqueue 收割，全部并入事件循环。**

- 创建：`forkpty()` 一把梭（内部完成 openpty+fork+子进程 setsid+控制终端），子进程 exec 前做 env 白名单注入/cwd/uid-gid/rlimits。ttyd 直接用 forkpty 是对的，但其 env 全继承是漏洞（pty.c:441-444 已核实）——wesh 用**白名单**（TERM、LANG/LC_*、PATH、HOME、USER、SHELL、COLORTERM 等可配）。
- 尺寸：`ioctl(master, TIOCSWINSZ)`；**内核自动向前台进程组发 SIGWINCH**，无需手动 kill（这使"attach 时先设为接入者尺寸"同时成为全屏应用的重绘触发器——coder 正是这么做的）。
- EOF 语义：子进程退出且 slave 侧无打开者后，master read 在 Linux 返回 **EIO**、在 BSD/macOS 返回 0/EOF——平台抽象层统一归一为 "Drain-到-EOF 后触发退出流程"，先读尽残余输出再宣告退出。
- 销毁：向**进程组**发信号（`kill(-pgid, sig)`，ttyd 同样这么做，pty.c:165），可配 TERM→KILL 升级宽限。
- 失败路径：spawn 失败必须能把**原因**经协议错误帧带回客户端（ttyd 做不到——协议无错误类型，且 pty_spawn 失败路径误 close(0) 已核实）。

### 2.3 子进程收割：pidfd > SIGCHLD；macOS 用 kqueue

**决策：Linux 用 pidfd_open+事件循环可读通知+waitid(P_PIDFD) 收割；macOS 用 kqueue EVFILT_PROC/NOTE_EXIT；消灭每进程一线程。**

| 方案 | 结论 | 证据 |
|---|---|---|
| **pidfd（Linux 5.3+）** | **采用**。fd 可 poll（子进程变 zombie→EPOLLIN），钉住 task 无 PID 复用竞态；`waitid(P_PIDFD)` 取退出码；`pidfd_send_signal` 无竞态 kill | HIGH：man7 官方手册（2026-02 版） |
| kqueue EVFILT_PROC | **macOS/BSD 采用**（NOTE_EXIT），同源并入事件循环 | MEDIUM-HIGH：BSD 通用知识，macOS 目标平台需原型验证 |
| SIGCHLD + self-pipe/signalfd | 兜底。信号不排队，必须 WNOHANG 循环 reap 全部；与运行时信号语义纠缠，能不用就不用 | HIGH（教科书级） |
| ttyd 的每进程 waitpid 线程 | **禁止**。N 会话=N 线程，线程与事件循环间还要 uv_async 编组 | HIGH（ttyd pty.c 本地源码） |

注意 man 页警告：pidfd 语义成立的前提是父进程未把 SIGCHLD 设为 SIG_IGN/SA_NOCLDWAIT 且未在别处 reap——PTY Engine 必须是子进程收割的**唯一**执行者。

### 2.4 滚动缓冲（重放）：内存原始字节 ring，非磁盘、非全量 VT 模拟

**决策：v1 采用每会话内存 ring（原始 PTY 字节流），attach 时 `reset + ring 快照 + 按需 WINCH 重绘`。**

三条路线对比（全部有成熟系统背书）：

| 路线 | 代表 | 优点 | 代价 | 结论 |
|---|---|---|---|---|
| 原始字节 ring | coder（64KiB circbuf） | 实现 ~100 行；滚动历史自然保留；同时是**慢客户端重同步**机制 | 全屏应用(vim/htop)重放是近似的；**断连期间尺寸变化后重放会按旧宽度排版**（coder 文档自称 "buggy" 的主因） | **v1 采用**（默认 256KiB–1MiB 可配） |
| 服务端 VT 模拟 | tmux(grid/screen)、mosh(Terminal::Display) | 任意尺寸重渲染，精确 | 自写 VT 解析器是巨大攻击面/工作量 | v2+ 候选 |
| 折中：headless xterm | @xterm/headless + addon-serialize（官方 typings 已核实：serialize() 可还原行列/光标/modes/scrollback） | 精确且不用自写解析器 | 需要内嵌 JS 运行时，与单静态二进制约束冲突 | 排除（除非未来有原生 VT 库；Rust `vt100` crate 是 v2 选项） |
| 磁盘 ring | — | 服务重启后仍可重放 | 项目明确不做重启恢复（CRIU 类 out of scope）；纯磁盘无收益 | 排除 |

**缓解近似性的工程手段**（coder 模式 + 增强）：
1. attach 流程：**先**把 PTY 尺寸设为接入者尺寸（内核自动 SIGWINCH，全屏应用自重绘），**再**重放 ring。尺寸变化场景下 ring 内容供滚动参考，当前屏由应用重绘修正。
2. 协议提供 REPLAY_BEGIN/END 帧，前端 `term.reset()` 后批量写入、避免逐帧闪烁，并可提示"已恢复 N KiB 历史"。
3. 文档明示近似性；重度全屏用户可自行 `wesh -- tmux`（不依赖、但兼容）。

### 2.5 IO fan-out：每客户端有界 outbox + 专属 writer（actor 不碰 socket）

**决策：Session Actor 读 PTY → append ring → 对每个客户端 `try_send` 到其有界 outbox；每条连接一个专属 writer task 做唯一的 WS 写端。**

- **反面教材（源码实证）**：coder buffered.go 在持有全局锁的读循环里逐 conn 阻塞写，TODO 自述应改 channel-per-conn——这就是行头阻塞，一个慢客户端冻结所有订阅者并卡住 ring 写入。ttyd 的"一读一停"则是另一个极端：每块数据 3-4 次拷贝、64KB 定缓冲读后即停，吞吐受限。
- tokio 官方文档佐证原语选择：`broadcast` 的 Lagged(n) 会**静默丢数据**（对有序字节流即客户端画面损坏），只适合"Lagged 即触发全量重同步"的用法；`mpsc`-per-client 才是有界、可观测、按客户端隔离的正解。
- 内存上界可预计算：`会话数 × (ring + 客户端数 × outbox容量)`，对齐 PROJECT.md 的资源控制要求。
- 写合并：PTY 读按 chunk（16–64KiB，可配），writer 从 outbox 批量 drain 合并成单帧，减少帧数与小包。

### 2.6 背压：有序流语义 → 慢客户端"踢出+重同步"，绝不丢字节

**决策：终端输出是有序 delta 流，丢帧=客户端画面静默损坏。慢消费者策略 = 有界 outbox 满 → 以 close code 1013 踢出该客户端 → 前端自动重连 → ring 重放完成重同步。ring 因此一鱼两吃（重连恢复 + 慢客户端重同步）。**

证据与细则（异步背压专文 + coder 缺陷 + tokio 文档三方互证）：
- 绝不在广播循环里 `await send`（最慢者拖住全部）；也绝不 `spawn(send)` 无界派发（内存放大至 OOM——ttyd 的预认证分片重组无上限同属此类，protocol.c:288-296 已核实）。
- 80% 高水位预警 + 成功即清零的 strikes 宽限，避免网络抖动误杀。
- **全局信用**：当**所有**可写客户端都阻塞时，Actor 停止读 PTY（master 不读 → 子进程写满 64KiB 管道缓冲自然阻塞）——这是唯一合法的"反压到生产者"。
- 客户端→服务端方向保留 ttyd 的 PAUSE/RESUME 语义（前端写缓冲水位触发），服务端对应停读 PTY/恢复。
- metrics 暴露每客户端 outbox 深度与踢出计数（运维可观测，PROJECT.md 明确要求 metrics）。

### 2.7 事件循环模型：单进程异步，单线程即够；actor 模型与运行时无关

- 参照系：tmux 单线程 libevent + 命令队列串行化全部变更——**串行化所有者**思想在十年前就被验证；个人运维场景并发量级是"几十连接 × 突发高吞吐"，瓶颈在拷贝与帧数，不在核数。
- Rust/tokio：current_thread 或 multi_thread(2) 均可；actor=task，邮箱=mpsc，fd 用 AsyncFd。Go：goroutine-per-session/conn + channel，等价自然。**组件结构与运行时选择正交**——语言决策留给 STACK.md，两种语言下本文组件映射都成立。
- 真正要避免的是 ttyd 的"裸 C 回调跨 lws/libuv 双域、靠标志位防 UAF"模型——高级框架（tokio/Go runtime）+ actor 所有权从结构上消除该类 bug。

### 2.8 WS 协议设计：版本化子协议 + 类型化帧 + 握手期认证 + 合规关闭码

**帧格式：数据面二进制 1 字节类型前缀（ttyd 已验证的零解析开销方案），控制面 JSON 文本帧。**

| 方向 | 帧 | 说明 |
|---|---|---|
| C→S | Hello（首帧，JSON） | `{version, ticket, attach: {session_id?|new, mode}, caps, cols, rows}`；**握手期认证**：ticket 核销成功前不分配任何会话资源，5s 超时，per-IP 未认证连接上限 |
| S→C | Welcome / Error(JSON) | Welcome `{version, session_id, mode, cols, rows}`；Error 带**类型化 code**（auth_failed/session_gone/permission_denied/spawn_failed/oversized...）后按码关闭 |
| S→C | OUTPUT(0x30+) / REPLAY_BEGIN / REPLAY_END | 重放段显式包裹，前端批量写入 |
| C→S | INPUT / RESIZE{cols,rows} / PAUSE / RESUME | 沿用 ttyd 语义；INPUT 经写权限门 |
| S→C | EXIT{code} / SIZE_NOTICE / TITLE / PREFS | EXIT 后按 1000 正常关闭；SIZE_NOTICE 广播他端尺寸变更 |
| 双向 | WS 原生 ping/pong | 应用层不再造心跳 |

要点与证据：
- **版本化**：子协议协商 `wesh.v1`（k8s remotecommand 的 `v5.channel.k8s.io` 是同款先例，官方 client-go 文档已核实 WebSocket 为主、SPDY 回落的演进史）；Hello 内再带 semver 便于次版本能力协商。
- **认证并入握手**：浏览器 `new WebSocket()` 只能带 URL+子协议，**无法设 Authorization 头**（websocket.org 指南，HIGH）。方案：先经已认证 HTTP `POST /api/attach`（Basic/Bearer，时序安全比较）换取**一次性 ticket**（随机 128bit、单次使用、60s TTL、绑定会话与模式），WS Hello 携带核销。这直接修复 ttyd `/token` 把长期凭据明文下发且 AuthToken 与 Basic 凭据复用的设计缺陷。失败节流：per-IP 指数退避。
- ticket 不走 URL query（会进访问日志/历史）；也不塞子协议头（非标癖好）——Hello 首帧 + 严苛的未认证资源上限是规范与安全的平衡点。
- **合规关闭码**：1000 正常 / 1008 策略违反(认证、权限) / 1011 服务端错误 / 1013 慢客户端踢出(可重试)。**1006 永远不上线**（ttyd protocol.c:90,105 把 1006 写进 close frame 违反 RFC6455，已核实）。
- **长度上限**：所有帧设上限（C→S 默认 16KiB；S→C 按读块），分片重组缓冲设硬顶且**认证通过前禁用任何累积**——修复 ttyd 两个预认证漏洞（空消息空指针、分片内存放大）。

### 2.9 多客户端写权限与 resize 仲裁

**决策：会话级写模式 `owner|all|ro`（创建时定，默认 ro 对标 ttyd 默认只读）；输入路由在 Actor 内做权限门；resize 跟随写权限。**

- GNU screen 的多用户语义（手册核实）：权限位 r/w/x 按用户×窗口×命令粒度，**writelock 单写者锁**——auto 模式下先敲键盘者持锁，离开窗口释放。screen 用"任意时刻只有一个写者"回答多人同时输入的行内交错混乱。
- abduco 的 resize 仲裁（官网核实）：只接受"最近接入的非只读客户端"的尺寸请求。
- wesh v1 落地：`all` 模式全员可写（协作排障，文档注明行内交错的固有问题）；`owner` 模式仅创建者可写（演示教学，其余旁观）；`ro` 全员只读。resize：仅可写客户端可发起，last-wins，广播 SIZE_NOTICE；owner 模式下跟随 owner 尺寸。writelock-auto 记为 v2 增强（screen 已验证其价值）。
- ticket 绑定 mode：旁观链接与可写链接可以分别签发（分享场景：把 ro 链接发给围观者）。

---

## 3. Recommended Project Structure

语言中立模块图（Rust 形状示意；Go 下为同构 package。最终以 STACK.md 选型为准）：

```
src/
├── main.rs              # CLI 解析 + 配置文件加载 + 组件装配（组合根）
├── gateway/             # HTTP/WS 前门
│   ├── http.rs          # 静态资源(内嵌前端)、/healthz、/metrics、POST /api/attach
│   ├── ws.rs            # upgrade、子协议协商、Hello/Welcome、帧编解码
│   └── limits.rs        # per-IP 限流、未认证超时、消息长度上限
├── auth/                # 凭据校验(时序安全)、一次性 ticket、失败节流
├── session/
│   ├── registry.rs      # id→actor 句柄、空闲回收、全局上限
│   ├── actor.rs         # 会话状态机 + 邮箱循环（核心）
│   ├── client.rs        # 每连接 outbox + writer/reader task
│   └── ring.rs          # 有界字节环 + 快照
├── pty/
│   ├── spawn.rs         # forkpty、env 白名单、cwd、uid/gid、rlimits
│   ├── io.rs            # master fd 非阻塞读写、TIOCSWINSZ、EIO/EOF 归一
│   └── reaper.rs        # pidfd(Linux)/kqueue(macOS) 平台抽象
├── proto/               # 帧类型、版本、错误码、close code 常量（单一事实源）
├── observe/             # 结构化日志、metrics 注册表、审计事件
└── config/              # flag + 配置文件 + 校验
```

**结构理由：** `session/` 与 `pty/` 分离 = 会话语义与操作系统细节分离（2.1 的控制面/数据面）；`proto/` 独立成单一事实源，gateway 与前端共用一份类型定义；`gateway/limits.rs` 把所有"预认证资源上限"收拢到一处，安全审计只看一个文件。

---

## 4. Architectural Patterns

### Pattern 1: Session Actor（会话唯一属主）

**What:** 每会话一个串行执行体，拥有全部会话可变状态；外界只通过邮箱发消息。
**When:** 任何"多客户端共享一份有状态资源"的场景。
**Trade-offs:** 杜绝锁与数据竞争；代价是单会话吞吐受单 task 限制——终端场景远够（瓶颈在 WS 写出，已外移到各 Conn writer）。

```rust
// 伪代码：所有共享状态只在 actor 循环内被触碰
loop {
    select! {
        chunk = pty.read_chunk() => { ring.append(chunk); fanout(chunk); }
        msg = mailbox.recv() => match msg {
            Attach(conn)  => { replay(&conn, ring.snapshot()); clients.insert(conn); }
            Detach(id)    => { clients.remove(id); arm_idle_timer_if_empty(); }
            Input(id, b)  => { if writable(id) { pty.write(b); } }
            Resize(id,s)  => { if writable(id) { pty.set_winsize(s); broadcast_size(s); } }
            Kill          => { pty.signal_group(SIGTERM); }
        },
        () = &mut exit_future => { drain_pty(); broadcast_exit(reap()); tombstone(); }
    }
}
```

### Pattern 2: 每连接有界 Outbox + 专属 Writer

**What:** fan-out 时 Actor 只做 `try_send`；每条连接独立 writer 任务独占 WS 写端。
**When:** 一对多流式分发，且各消费者速度不均。
**Trade-offs:** 每连接少量常驻内存；换来严格内存上界与慢客户端隔离。coder 的持锁遍历写是反例（源码 TODO 自述）。

```rust
match outbox.try_send(frame) {
    Err(Full(_)) => { metrics.lag_kicks.inc(); ws.close(1013, "slow consumer"); }
    Ok(_) => {}
}
```

### Pattern 3: Ring 重放 + WINCH 重绘（近似重连恢复）

**What:** attach 时先设尺寸（触发内核 SIGWINCH→全屏应用重绘）→ REPLAY_BEGIN + ring 快照 + REPLAY_END → 进入实时流。
**When:** 不做服务端 VT 模拟时的最佳近似。
**Trade-offs:** 全屏应用依赖其 SIGWINCH 重绘（bash/readline/vim/htop 主流均支持）；纯滚动输出场景完全精确。coder 生产环境在用，官方注明 buffer 后端 "buggy"——我们把尺寸变更场景写进已知限制。

### Pattern 4: 一次性 Ticket 握手认证

**What:** 已认证 HTTP 端点换单次短 TTL ticket → WS Hello 核销 → 通过前零会话资源。
**When:** 浏览器 WS 无法带 Authorization 头（平台约束，非选择）。
**Trade-offs:** 比 ttyd `/token` 明文下发长期凭据暴露面小一个量级；需要前端多一次往返（毫秒级）。

### Pattern 5: pidfd/kqueue 退出监视（零线程收割）

**What:** fork 后立即 `pidfd_open`，fd 注册进事件循环；EPOLLIN→`waitid(P_PIDFD)` 取退出码。
**When:** 需要精确退出码且不想为每个子进程开线程。
**Trade-offs:** Linux 5.3+（2019 年后的内核，目标场景全覆盖）；macOS 走 kqueue 分支，平台抽象层消化差异。

---

## 5. Data Flow

### 5.1 Attach 流程（重连同路径）

```
浏览器                     Gateway              Auth        Registry        Session Actor      PTY
  │  POST /api/attach        │                   │             │                 │               │
  │─────────────────────────>│ 校验 Basic/Bearer  │             │                 │               │
  │                          │───────────────────>│ 时序安全比较  │                 │               │
  │  {ticket, session_id,    │<── ticket(单次,60s) │             │                 │               │
  │   ws_url, mode}          │                   │             │                 │               │
  │  WSS upgrade (wesh.v1)   │                   │             │                 │               │
  │─────────────────────────>│ Origin 白名单/限流  │             │                 │               │
  │  Hello{ticket,caps,size} │                   │             │                 │               │
  │─────────────────────────>│ 核销(5s 超时)       │             │                 │               │
  │                          │────────────────────────────────>│ attach→actor    │               │
  │                          │                                 │────────────────>│ 设尺寸(WINCH)  │──> 应用重绘
  │  Welcome                 │<────────────────────────────────┼─────────────────│               │
  │  REPLAY_BEGIN+ring+END   │<── Conn writer(批量)             │                 │               │
  │  OUTPUT 实时流 ...       │<────────────────────────────────┼── fan-out ──────│<── read ──────│
```

### 5.2 输出 fan-out（数据流向单一：PTY→Actor→各 Conn，无反向写）

```
PTY master ──read(16–64KiB)──> Session Actor ──append──> Ring
                                  │
                    try_send ┌────┼────┐ (Full→1013 踢出+metrics)
                             ▼    ▼    ▼
                          outboxA outboxB outboxC      （各自有界）
                             │    │    │
                          writerA writerB writerC      （独占各自 WS 写端，批量 drain 合并帧）
                             ▼    ▼    ▼
                             WS   WS   WS
```

### 5.3 输入 / resize / detach / exit

1. **输入：** WS INPUT → Gateway 解码 → Actor 权限门（ro 丢弃；owner 模式仅 owner）→ 写 master。多写者交错不做排序承诺（screen 同款语义，v2 可加 writelock）。
2. **resize：** 可写客户端 RESIZE → Actor last-wins → TIOCSWINSZ（内核自动 SIGWINCH）→ SIZE_NOTICE 广播 → 其他前端 fit。
3. **detach：** WS 关闭 → Actor 移除 Conn → 归零则启动空闲计时器 → 超时向进程组发可配信号 → 收割 → 墓碑。
4. **exit：** pidfd EPOLLIN → waitid 收割 → master 读至 EOF（EIO 归一）→ ring 收尾 → EXIT{code} 广播 → 客户端 1000 关闭 → 墓碑保留至 TTL（迟到重连可见终态与退出码，abduco 语义）。

---

## 6. Scaling Considerations

个人运维工具定位，真实量级是个位数会话、几十连接。**先优化每会话吞吐与内存上界，而非并发数。**

| 规模 | 调整 |
|---|---|
| 1–10 连接（单人） | 默认参数即可；单线程运行时足够 |
| 10–100 连接（团队围观/教学） | 关注每客户端 outbox 深度分布与 1013 踢出率；调大 ring 与 outbox；metrics 定位慢网客户端 |
| 100+ / 多租户 | **明确不支持**（PROJECT.md out of scope）。天然上限：全局连接数/会话数硬顶 + per-IP 限流兜底 |

**Scaling priorities：**
1. **第一瓶颈：单会话高吞吐输出**（`cat` 大文件）。手段：读块合并、writer 批量 drain、零/少拷贝管道（读缓冲复用，禁止 ttyd 式每块 3–4 次拷贝）。
2. **第二瓶颈：慢客户端。** 手段见 2.6；永远不能让一个弱网客户端反压 PTY 影响其他客户端。

---

## 7. Anti-Patterns

### Anti-Pattern 1: 连接即进程（ttyd/gotty 模型）
**What people do:** WS 连接状态直接持有子进程，断开即 kill。
**Why it's wrong:** 会话保持与多客户端在结构上永远不可能；ttyd 官方只能让用户套 tmux。
**Do this instead:** 进程归 Session Actor 所有，连接只是可替换的渲染端（2.1）。

### Anti-Pattern 2: 持锁（或 await）遍历写做 fan-out
**What people do:** 读循环里逐连接阻塞写（coder buffered.go，源码 TODO 自述）。
**Why it's wrong:** 最慢客户端行头阻塞所有人，还卡住 ring 追加。
**Do this instead:** 每客户端有界 outbox + `try_send` + 专属 writer（Pattern 2）。

### Anti-Pattern 3: 对有序字节流静默丢帧
**What people do:** 用 tokio broadcast 之类"慢者丢数据"原语直接扇出终端字节。
**Why it's wrong:** 丢一段转义序列 = 客户端画面持久损坏且无任何信号。
**Do this instead:** 丢帧即破坏 → 踢出(1013)+重连重放；若用 broadcast，Lagged 必须触发全量重同步而非继续。

### Anti-Pattern 4: 认证前分配可增长资源
**What people do:** ttyd 在认证检查之前做分片 `realloc` 累积（protocol.c:288-296，预认证内存放大漏洞）。
**Why it's wrong:** 任何人匿名打爆内存。
**Do this instead:** 未认证连接只有定长小状态 + 5s 超时 + per-IP 上限；一切缓冲分配发生在票据核销之后。

### Anti-Pattern 5: 每进程一个收割线程
**What people do:** ttyd 每 PTY 一个阻塞 waitpid 线程 + uv_async 编组回事件循环。
**Why it's wrong:** 线程数随会话线性增长，编组路径是 UAF 温床。
**Do this instead:** pidfd/kqueue 并入事件循环（Pattern 5）。

### Anti-Pattern 6: v1 就上服务端全量终端模拟
**What people do:** 为了重连画面精确，自写 VT 解析器/grid。
**Why it's wrong:** tmux/mosh 的核心资产就是这块，工作量与攻击面巨大；coder 的教训说明 ring 已覆盖主要痛点。
**Do this instead:** ring + WINCH 重绘（Pattern 3），把"精确重渲染"留作 v2 选项（Rust 可考虑 vt100 crate）。

---

## 8. Integration Points

### Internal Boundaries

| 边界 | 通信方式 | 注意事项 |
|---|---|---|
| Gateway ↔ Auth | 进程内同步调用（核销为 O(1) 查表+删除） | 核销必须原子（单次使用）；失败计数写入节流派 |
| Gateway ↔ Registry | typed message（Attach/Detach） | Gateway 不持有 Actor 内部状态，只拿 Conn 句柄 |
| Registry ↔ Session Actor | actor 邮箱（mpsc） | Registry 只存句柄与元信息；空闲计时器归 Actor 自己 |
| Session Actor ↔ Client Conn | outbox mpsc（数据）+ 控制消息 | Actor 永远不直接写 WS |
| Session Actor ↔ PTY Engine | fd 事件 + exit future（同一事件循环） | PTY fd 不出 Actor；收割唯一入口在 Engine |
| 全部 ↔ Observability | metrics handle（无锁计数器） | 每客户端队列深度必须可导出 |

### External Services

| 外部依赖 | 集成方式 | 坑 |
|---|---|---|
| 前端 xterm.js 生态 | 构建期打包内嵌（gzip 直发，ttyd 同款） | addon 版本矩阵（webgl/unicode11/fit/serialize）需在 UI 阶段锁定 |
| 系统 PTY/内核 | forkpty/ioctl/pidfd/kqueue | Linux EIO vs macOS EOF 差异；内核 <5.3 无 pidfd（可放弃支持并文档化） |
| TLS 证书 | 用户提供的 cert/key 或自签 | cipher/协议下限与安全响应头集中在一处配置（修复 ttyd 缺陷） |

---

## 9. 构建顺序建议（供 roadmap 分阶段）

依赖链：`PTY Engine ≺ Session Actor ≺ {Ring 重放, 多客户端 fan-out}`；`协议/认证 ≺ 多客户端权限`；`Conn outbox 结构 ≺ 背压策略`。

| 建议阶段 | 内容 | 依赖与理由 |
|---|---|---|
| **P1 行走骨架** | Gateway(裸 WS+静态页) + PTY Engine + 单客户端 Session + 最小协议(OUTPUT/INPUT/RESIZE) + xterm.js 前端接通 | 无依赖。最先验证语言/运行时与 e2e 管道；达到"ttyd 核心 parity（无认证）" |
| **P2 协议与安全基线** | proto/ 类型化帧与版本协商、错误帧、合规关闭码、长度上限；Auth(ticket+时序安全+节流)、Origin 白名单、TLS 配置 | 依赖 P1 的管道。**必须先于共享功能**：多客户端权限需要身份概念 |
| **P3 会话解耦（跨越 #1）** | Registry + Session Actor 化 + detach/reattach + Ring 重放 + 空闲回收 + 退出墓碑 + 前端重连状态机 | 依赖 P1/P2。Actor 化时**就把 Conn 建成 outbox+writer 结构**（单客户端也如此），P4 零返工 |
| **P4 多客户端（跨越 #2）** | fan-out、慢客户端 1013+重同步、写模式 owner/all/ro、resize 仲裁、SIZE_NOTICE、按模式分别签发票据 | 依赖 P3 的 Actor/outbox 与 P2 的身份。coder/screen/abduco 语义在此落地 |
| **P5 资源控制与可运维** | 全局连接/会话上限、每客户端限速、/healthz、metrics、结构化日志、配置文件、env 白名单、降权、rlimits | 依赖 P3/P4 结构稳定后铺面；部分(env 白名单/降权)可提前并入 P1 的 PTY Engine |
| **P6 打磨与发布** | base-path、自定义首页、客户端偏好下发、打包单二进制、模糊/负载测试（重点：高吞吐 fan-out 与慢客户端矩阵） | 收尾；负载测试数据回填 P4/P5 的默认参数 |

**Research flags：**
- P1：macOS kqueue 退出监视需要早期原型验证（MEDIUM-HIGH 置信，有平台差异风险）。
- P3：ring 默认容量与"尺寸变更后重放"体验需要实测调参（coder 64KiB 仅作下界参考）。
- P4：默认 outbox 容量/水位/strikes 需负载测试标定。
- 其余为标准模式，无需追加研究。

---

## 10. Sources

| 来源 | 用途 | 置信度 |
|---|---|---|
| ttyd 1.7.7 本地源码（protocol.c / pty.c，行号见文内） | 重写基线与缺陷实证 | HIGH（一手源码） |
| coder/coder v2.36.0 `agent/reconnectingpty/`（pkg.go.dev 文档 + buffered.go 源码精读，2026-08 发布） | 重连 PTY、ring 重放、fan-out 反例、超时回收 | HIGH（一手源码） |
| tmux deepwiki（带 tmux.h 行号引用） | client-server、服务端屏幕状态、命令队列串行化 | HIGH |
| abduco 作者官网（brain-dump.org/projects/abduco） | 极简 detach 模型、只读 attach、resize 仲裁、退出状态保留 | HIGH |
| GNU screen 手册 multiuser 章节（aperiodic.net 镜像） | ACL rwx、writelock 单写者锁 | HIGH |
| man7 pidfd_open(2)（man-pages 6.18，2026-02） | pidfd 收割语义与前置条件 | HIGH |
| websocket.org 认证指南（2026-03） | 浏览器 WS 认证约束、ticket/首帧/续期模式 | HIGH |
| async-concurrency.com 慢消费者背压专文（2026-06） | 有界队列/踢出策略/水位/metrics | HIGH（与 coder 源码、tokio 文档三方互证） |
| tokio 官方文档（Context7） | broadcast Lagged 语义、mpsc 背压 | HIGH（官方文档） |
| xterm.js 官方 typings/文档（Context7） | addon-serialize、@xterm/headless、scrollback 默认 | HIGH（官方文档） |
| k8s client-go remotecommand pkg.go.dev | 版本化子协议、WebSocket 主 SPDY 回落、TerminalSizeQueue | HIGH（渠道编号细节 MEDIUM，来自训练知识） |
| EternalTerminal deepwiki | etserver/etterminal 进程模型、ID/passkey 重连、序号化重传 | MEDIUM-HIGH |
| shellhub 仓库 README 分析 | 网关型会话不持久（仅录像持久） | MEDIUM-HIGH |
| mosh 官网/GitHub 摘要 + 训练知识 | 状态 diff 同步路线对照 | MEDIUM |
| gotty（yudai）模型描述 | Go 版 ttyd 同模型佐证 | MEDIUM（repo 抓取失败，取自检索摘要+训练知识） |

---
*Architecture research for: web 终端共享工具（wesh，ttyd 重写）*
*Researched: 2026-08-13*
