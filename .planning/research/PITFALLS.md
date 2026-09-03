# Pitfalls Research — v1.1 per-client 会话模式

**Domain:** 给既有共享会话（GoTTY 模型：单 PTY 子进程 × N WS 客户端）系统新增 per-connection PTY spawn
**Researched:** 2026-09-02
**Confidence:** HIGH（wesh 代码库一手逐文件分析：server.go/clients.go/resize.go/spawn.go/io.go/reap_*.go/metrics.go + v1 已 CI 裁决机制）；MEDIUM（Go os/exec 文档佐证）；LOW（平台通用机制的 web 佐证——均已与 v1 CI 实测结论交叉核对后才采信）

> **下游使用说明**：每条陷阱含预警信号、预防策略、建议处理阶段。阶段编号引用建议的 v1.1 路线图骨架（占位命名，路线图定稿为准）：
> - **P10 模式装配与接缝**：`--session-mode` flag/TOML 键 + session 供给 seam（Server 与"会话集合"解耦）+ validateStartup 组合矩阵 + 配置 fuzz 语料
> - **P11 per-client 生命周期主干**：attach spawn / 断开 SIGHUP / EXIT 私有化 / 重连=新进程 / teardown 恰好一次 / spawn 失败 UX
> - **P12 资源与容量防线**：spawn 限速、容量超编栅栏、KILL 兜底、N 进程组 Shutdown、资源标定
> - **P13 语义适配与观测面**：--once/--exit-when-empty 退出路径重建、healthz/metrics/audit per-client 语义、resize 直通防抖
> - **P14 双模式测试矩阵与 UAT**：mode 参数化测试 harness、-race 双模式门、phaseNN.mjs per-client 全链、herdr Playwright 端到端
>
> **本里程碑特有的方法论警告**：这不是在新系统里设计 spawn——是在一个 44/44 需求已收口、每条生命周期不变量都有测试锁定的系统里加第二条进程路径。最大的陷阱不是"spawn 写错"，而是**破坏既有不变量却以为自己没碰它**（D-10 唯一终结路径、D-13 零新 exitf 分支、唯一收割者、Welcome 恒首帧、零身份 label 红线）。每条陷阱都标注了它威胁的既有不变量。

## Critical Pitfalls

### Pitfall 1: 「空即不死」——exit-when-empty/--once 在 per-client 下无子进程可等，服务端永不退出

**What goes wrong:**
v1 的全部退出路径（SESS-01 --once、SESS-02 --exit-when-empty 立即/宽限到期）都收敛到同一个机制：`stopChildLocked()` 向**那个唯一的** `s.sess` 进程组发 stop-signal → 子进程死亡 → `sess.Wait()` 返回 → `lifecycle()` → `terminate(code)` → exitf（D-10 唯一终结路径、D-13「零新 exitf 分支」硬约束）。per-client 模式下客户端断开即 SIGHUP 各自子进程，注册表变空的时刻**系统里已经没有任何子进程**——`stopChildLocked` 无进程可杀，没有 `sess.Wait` 会返回，lifecycle 永不触发，**`--once` 和 `--exit-when-empty` 的服务端挂在那里永不退出**。更隐蔽的是：这条路在 shared 模式测试里全绿，只有 per-client × exit-when-empty 的组合用例会抓到——而组合用例恰恰最容易漏写。

**Why it happens:**
D-13「终结只经由子进程死亡」是 v1 的正确设计，但它内嵌了一个隐性前提：**"子进程存在"与"服务端该活"等价**。per-client 把这两条解耦了（无客户端=无进程，但服务端该继续活；--once 要求无进程时服务端必须死）。把 `stopChildLocked` 照搬到 per-client 分支，就是对这条隐性前提的无意识继承。

**How to avoid:**
- 显式承认 per-client 需要第二条终结触发源，并用既有 `termOnce` 收口保证恰好一次：注册表非空→空迁移 且 活动会话数==0 时，`--exit-when-empty` 路径直接调度 `terminate(255)`（accept-255 裁决的 per-client 映射），不再绕道信号。这不是违反 D-13，是 D-13 在新模式下的显式重议——**必须在 Key Decisions 里登记**，而不是偷偷加一个 exitf 调用。
- 退出码语义对齐：shared 模式 --once 退出 255（SIGHUP 终结子进程，-1 截断）；per-client 子进程同样被 SIGHUP 终结，exitf 参数应沿用同一语义（255），三消费点（Go 测试断言/进程级/README 文案）同步更新。
- 实现上把「空判定」做成 mode 无关的单一挂点（`maybeExitWhenEmptyLocked` 同位），差异只在该函数体内的终结动作：shared=信号唯一会话，per-client=直接 terminate。禁止在 detach/kick 里另开 mode 分支。

**Warning signs:**
- `stopChildLocked` 里出现 `if s.sess == nil { return }` 之类的静默兜底——这就是 bug 被掩埋的现场。
- 测试矩阵里 emptyexit_test 只以默认（shared）模式跑。
- UAT 没有「--once + per-client：断开 → 进程退出 255」用例。

**Phase to address:** P13（退出路径重建）；P10 的 validateStartup 矩阵先行明确组合语义

---

### Pitfall 2: kill-after-reap 误杀——pgid 复用窗口内 SIGHUP 打中无关进程组

**What goes wrong:**
per-client 断开路径要向"自己那个"子进程组发 SIGHUP（`kill(-pgid, SIGHUP)`）。如果子进程在断开检测到达之前已经退出 **且已被 Wait 收割**，其 pid 可以被内核回收复用——setsid 不变量使 pgid==pid，于是 `kill(-pgid)` 命中的可能是一个**刚复用该 pgid 的无关进程组**。v1 里同样的竞态存在但无害：`stopChildLocked` 的调用点（exit-when-empty/Shutdown）之后进程随即 exitf，误杀窗口随进程消亡。per-client 下**服务端长期存活**，每次断开都滚一次这个骰子，误杀的是宿主机上别的用户进程。ESRCH 静默纪律（signal_linux.go 注释）在这里不保护你——复用后的 pgid 存在，kill 成功返回。

**Why it happens:**
v1 的「信号幂等、ESRCH 静默」纪律是在"进程即将退出"的语境里证明的，证明前提（exitf 紧随）在新模式下不成立。僵尸窗口内（子死但未 reap）发信号是安全的——pid 仍被僵尸持有不会复用；**危险区是 reap 完成之后**。断开路径（detach）与收割路径（session 的 Wait goroutine）在两个 goroutine 里，无同步就是裸竞态。

**How to avoid:**
- 每会话一把状态锁 + `reaped` 标志：session 专属 lifecycle goroutine 在 `Wait()` 返回后置位；断开路径发信号前必须在同一把锁内检查。**信号与 reap 的先后由此序列化，误杀结构性不可能**。
- 保持「唯一收割者」纪律（reap_linux.go D-14）：断开路径绝不自己调 `Wait()`，只经 session 的 done channel 等待收割完成。
- Linux 上 Go≥1.23 的 pidfd 收割已保证 Wait 本身无 PID 复用问题（waitid P_PIDFD）；本条防的是**应用层 kill(-pgid)**，pidfd 不覆盖进程组信号，别指望它兜底。

**Warning signs:**
- 代码里出现不持锁的 `sess.SignalGroup(...)` 调用点在断开路径上。
- 压测「connect→spawn→立即 disconnect」十万次循环 + 宿主机同 pgid 复用注入（测试可用占位进程组模拟）缺位。

**Phase to address:** P11（teardown 恰好一次的组成部分）

---

### Pitfall 3: 断开↔子死双路收口竞态——teardown 必须恰好一次且顺序固定

**What goes wrong:**
per-client 会话有两条终结触发路径：(a) 客户端断开 → detach/kick → SIGHUP 进程组；(b) 子进程自己退出 → EXIT 帧私有化 → 关闭该客户端。两条路径在两个 goroutine 里并发到达，各自都要做一摊子清理（信号、Drain master、Close、Wait 收割、WS 关闭、注册表移除、容量释放）。无协调的典型后果：双重 Close(master)（已有幂等保护，OK）、**双重 SIGHUP**（第二次打中 reap 后复用的 pgid——Pitfall 2）、EXIT 帧与 1013 踢出竞序（客户端先收 1013 再收 EXIT？）、以及最毒的——**容量/注册表记账漂移**（两条路径各自减一次计数，`TestClientCountInvariant` 类不变量崩坏）。
PTY master fd 的回收时机也有讲究：孙进程继承 slave fd 会使 master 读不到 EOF/EIO（v1 `Drain(200ms)` 已为此存在），per-client 每条会话都必须保留这个带时限 drain——断开即 `Close(master)` 而不 drain 会把"子进程最后输出"丢掉，而且内核 hangup SIGHUP 时序变得不可控；反过来只 drain 不显式 SIGHUP，则依赖子进程对 hangup 的默认行为，nohup/trap 程序直接漏网（见 Pitfall 8）。

**Why it happens:**
v1 的收口是单线程式的：`lifecycle()` 是唯一终结者，detach 明确"不进 exitf、不发任何信号"。per-client 把终结权下放给每条会话，v1 的"唯一触发源"论证整体失效，需要每会话重建一次恰好一次语义。

**How to avoid:**
- 每会话 `sync.Once` + 固定顺序的 teardown 序列：**SIGHUP 进程组（经 Pitfall 2 的 reaped 栅栏）→ stop-timeout 到期补 SIGKILL → Drain(d) → Close(master) → Wait 返回 → 注册表/容量记账单点移除**。两路径都只"触发"，执行序列只有一个。
- SIGHUP 可以在 hubMu 内发（SignalGroup 不取 fdMu，锁序 hubMu > sess.fdMu 不受影响——signal_linux.go 既有论证）；但 **Drain/Close/Wait 绝不能在 hubMu 内做**（200ms 级阻塞 = 全服务端行头阻塞）——teardown 的慢半段放会话自己的 goroutine。
- EXIT 帧私有化写序复用 v1 形态：组帧一次 → 同步 Write(EXIT, 2s ctx) → Close(1000)，禁经 outbox 异步入队（关闭帧超车问题，Phase 6 已裁决）。
- 记账纪律沿用 review #7：一切移除收口 `removeLocked` 单点，detach 与 kick 互斥由成员判定保证；per-client 新增的"会话终结移除"是**第三个移除触发源**，必须走同一单点，不能自开计数路径。

**Warning signs:**
- teardown 逻辑出现在两个以上函数的函数体里（而非一个 Once 序列）。
- hubMu 持锁栈里出现 `Drain`/`Wait`/`time.Sleep`。
- -race 双模式全量出现 `removeLocked` 计数断言失败或 outbox use-after-kick。

**Phase to address:** P11

---

### Pitfall 4: 重连 fork bomb——maxClients 限并发不限速率，既有三道闸全部不覆盖已认证 churn

**What goes wrong:**
per-client 模式下每次 attach = 一次 fork+exec。一个持合法 ticket 的客户端（或被盗的分享 token）可以 connect→Hello→spawn→disconnect 死循环：maxClients=32 只限制**同时存活**的进程数，对**单位时间 spawn 次数**零约束；per-IP half-open 帽（8）在 Hello 完成即释放，管不到升档后的 churn；SEC-03 节流只对**认证失败**计数，合法票据的成功 attach 不在其计数器内。后果不只是 CPU：每次循环还伴随 SIGHUP/收割/fd 开闭，若 teardown 有任何滞后（Pitfall 3/8），进程与 fd 泄漏速率 ×N。另一个形态是**无差别惊群**：机房网络抖动后 30 个浏览器同时自动重连（前端 1006 退避 1s×2 封顶 30s），per-client 模式瞬间 30 个 fork+exec——shared 模式的"接回同一进程"在这里变成"同时 spawn 30 个进程"。
还有第二层容量语义陷阱：③位 503 闸与 /api/attach 早闸的判定源都是 `registry.n`——**注册成功才计数**（R-06）。per-client 下 spawn 发生在 Hello 之后、register 之前的窗口里：8 个并发握手（half-open 帽内）各自通过容量检查后同时 spawn，瞬时进程数 = 32 + 8。shared 模式这个超编只是 8 条连接（已裁决接受，RESEARCH A5）；per-client 模式是 8 个完整进程 + 随后可能又要把超员的 SIGHUP 掉——超编成本与回收语义都变了。

**Why it happens:**
v1 的资源模型是"一个进程 + N 条连接"，所有容量闸都为连接设计。per-client 把最贵资源（进程）的创建挂到了连接生命周期上，而连接层的闸从来不是为了保护 fork 预算设计的。

**How to avoid:**
- **spawn 路径自带速率闸**（per-client 模式专属装配）：全局 spawn 令牌桶（防惊群，如 8/s burst 16）+ per-IP spawn 桶（防单点 churn，如 1/s burst 4），取不到令牌在 spawn 前拒绝：Error 帧（`spawn_throttled` 类机器串）+ Close(1008)。**关闭码必须避开 1006**——前端只对 1006 自动重连（Phase 6 shouldReconnect 裁决），用 1008/1011 让正常前端立即停手，闸自调节。
- 容量记账改为 **spawn-intent 口径**：容量检查通过→hubMu 内预占名额→spawn 失败回滚→register 转正。或者接受超编但显式裁决（像 A5 那样登记"超编 ≤8 个进程可接受"并写进 README）——二选一，不许无意识继承 R-06。
- 拒绝路径的一切既有纪律不变：half-open release 恰好一次（sync.Once + defer 已有）、不进注册表、不打含敏感值的日志。
- 负载测试新增「合法票据 churn」用例：单 IP 10rps attach 30s，断言服务端 RSS/goroutine/fd 有界（`wesh_goroutines`/`wesh_mem_alloc_bytes` 已有观测钩子）。

**Warning signs:**
- spawn 调用点在代码里不受任何 limiter 保护（`grep` 不到一个桶）。
- 压测只测并发驻留，没测 attach 速率。
- `wesh_goroutines` 在 churn 下单调上涨不回落。

**Phase to address:** P12

---

### Pitfall 5: spawn 失败路径四连坑——fd 纪律 / 启动预检假设失效 / Error 帧 UX / 级联 EMFILE

**What goes wrong:**
(a) **fd 泄漏**：ttyd 的 pty_spawn 失败误 close(0) 是 PROJECT.md 已核实的缺陷，wesh v1 在单次 spawn 路径修掉了（creack/pty 失败只关自己的 fd + "只关成功打开且登记在册的 fd"纪律，spawn_test 实测 fd 0/1/2 完好）。per-client 把 spawn 挪到 attach 热路径，失败概率从"启动一次"变成"每连接一次"——任何新写的清理代码（关 master、回滚容量名额、移除半成品注册项）都是新的 fd/记账泄漏面。
(b) **启动预检假设失效**：shared 模式 spawn 在启动期，二进制不存在/cwd 不存在 = 启动 fail-fast，运维当场看到。per-client 的 spawn 发生在服务端跑了几天之后——这期间二进制可能被删、`--cwd` 目录可能被 rm、EMFILE/EAGAIN/`/dev/ptmx` 耗尽（kernel pty 上限、RLIMIT_NOFILE）都会出现。**这些不是启动错误而是运行期常态**，绝不能走 fatal/exit 路径，也不能让单个 spawn 失败拖垮 accept 循环或其他客户端。
(c) **失败 UX**：现有握手不变量是「Welcome 恒首帧」（P2 D-02：注册前 OUTPUT 一律丢弃，Error 帧只在握手违规直写场景出现）。spawn 失败发生在 ticket 核销之后、Welcome 之前——若什么都不发直接 Close，前端就是黑屏+莫名断开。
(d) **级联**：EMFILE 时若失败清理不到位（master fd 半开），fd 进一步泄漏 → 后续全部 spawn 失败 → 等价于服务端死亡，但 /healthz 还是 200。

**Why it happens:**
v1 把 spawn 当启动事件设计（预检→fail-fast→零资源占用拒绝纪律），per-client 把它变成请求路径事件——错误处理哲学完全不同（per-request degrade vs fail-fast），最容易把启动期假设无意识带过来。

**How to avoid:**
- 失败 UX 定型：**先 Error 帧（类型化机器串如 `spawn_failed`，message 用通用文案——不带 errno 字符串/路径细节回显，防服务器文件系统布局泄露）→ Close(1011)**。1011 不在前端自动重连码集内，用户看到明确提示而非无限转圈。协议面零新帧类型（Error 帧类型空间既有，D-01 纪律不破）；前端把新机器串映射到「服务端无法启动会话」文案。
- 失败清理清单化并测试锁定：release half-open（既有 sync.Once）→ 回滚 spawn-intent 名额（Pitfall 4）→ 不进注册表 → 不产生任何 session_start/attach 审计事件（或只产生 spawn_failed 单事件，无 pid/client_id）→ 审计行零敏感值。
- spawn 失败计数进 metrics：`wesh_pty_spawn_failures_total` counter（零身份 label 红线不变）——EMFILE 级联在 Prometheus 里立即可见，而不是等用户报障。
- 测试：pty 包 spawn 失败 fd 完好断言扩展到"N 次连续失败后进程 fd 计数不变"；server 包注入 spawn 失败桩断言上述清理清单逐条 + 其他已 attach 客户端零感知。

**Warning signs:**
- attach 路径出现 `log.Fatal`/`os.Exit`/panic。
- Error 帧 message 里拼 `err.Error()`（含路径/errno）。
- fd 计数测试只在 pty 包单 spawn 场景。

**Phase to address:** P11（失败 UX 与清理）、P12（EMFILE/限额标定）

---

### Pitfall 6: 模式分支漂移——`s.sess` 单例字段遍布 Server，散点 if/else 半年后必然腐化

**What goes wrong:**
`s.sess` 在 v1 Server 里是单例：`New` 的三个 goroutine（ReadLoop/inputWriter/lifecycle）、`recalcNow` 的 Resize、`SignalForegroundGroup`、`stopChildLocked`、`inputWriter` 的 Master.Write、healthz 的 sessionAlive——**每一个都是 per-client 下语义不同的触点**。最直觉的实现是在每个触点写 `if s.perClient { ... } else { ... }`：第一个月没问题，之后 shared 侧修 bug 只改一个分支（另一个人不知道有第二分支）、per-client 侧的新测试在 shared 下意外通过（分支没走到），两个模式缓慢长成两个产品，而 CI 只证明"各自当下能跑"。
最危险的三个具体漂移点：
1. **`s.exiting` / `s.closeBroadcastCode` 层级错位**：v1 里它们是服务端全局门（lifecycle/Shutdown 置位，detach 读它来打 reason=shutdown）。per-client 下单条会话终结**绝不允许置全局 exiting**——否则其他客户端后续的正常断开全部被误记 reason=shutdown（审计语义污染），更糟的是 `maybeExitWhenEmptyLocked` 检查 exiting 后不再触发退出逻辑（Pitfall 1 的退出路径被自己毒死）。per-session 的终结门必须是新字段，层级分明。
2. **fan-out hub 形态**：`onChunk` 的信用门 Wait 循环 + 注册表遍历扇出是为 1:N 设计的；per-client 是 1:1（一条会话的输出只去它的属主客户端）。让 per-client 复用全局 hub，等于每条会话的 ReadLoop 都要过全局 hubMu + 遍历注册表找属主——N 条会话的输出在**一把全局锁上互相串行**，一个客户端的信用门闭合波及无关会话。
3. **inputQ/inputWriter 单例**：v1 会话级输入队列独占 `sess.Master.Write`；per-client 每条会话一个 master，必须每会话各一套，但 CR-01 的教训（读循环零同步写）必须原样继承——per-client 分支里"简单起见"让读循环直写 master，就是 CR-01 回潮。

**Why it happens:**
模式判定是横切关注点，Go 没有特性开关系统，散点分支是零设计的默认结果。且 v1 代码的注释密度极高，分支改造时注释（论证不变量的）最容易留下指向旧事实的谎言。

**How to avoid:**
- **单一接缝**：定义 session 供给抽象（如 `sessionSet`：shared=启动期预建的单条会话；per-client=attach 期按连接创建/按断开销毁的会话表）。Server 主流程只面对这个接口；mode 分支**只允许出现在接缝实现内部与 New 的装配段**（装哪些组件：仲裁器/信用门/owner 递补在 per-client 不装配——milestone 已定）。
- shared 模式实现做成既有代码的**薄适配**，不是重写——零回归的最强证据是"shared 路径与 v1.0 逐字节等价"（编译期 + 测试断言双证）。
- 全局门与每会话门显式分字段：`s.exiting`（仅 lifecycle/Shutdown 两置位点）vs per-session `teardownOnce/reaped`；detach 的 reason 判定矩阵加一列模式来源，测试锁定。
- 每会话一套 ReadLoop/inputWriter/lifecycle goroutine + 每会话 outbox 路由（1:1 直投属主 outbox，信用门不装配=满即 1013 踢——R-08 在 1:1 下退化为单端判定，无需 cond）。
- review 纪律：每个 v1.1 PR 自检"这次改动在另一模式下的对应物是什么"，写进 PR 描述。

**Warning signs:**
- `grep -n "perClient\|sessionMode" internal/server/` 命中散点分布在 5 个以上函数体。
- per-client 分支里 `onChunk` 仍遍历全局注册表找属主。
- 注释里出现「唯一终结路径」「唯一会话」之类旧不变量但没标注模式适用范围。

**Phase to address:** P10（接缝先行——这是骨架工作，必须第一个做）

---

### Pitfall 7: per-client 分支丢失防抖 → SIGWINCH 风暴回潮（C10 老坑新踩）

**What goes wrong:**
v1 的 resize 链路：RESIZE → arbiter（50ms 防抖 timer 合并风暴）→ 目标变化才 TIOCSWINSZ（resize.go reportResize/recalcNow，`defaultResizeDebounce=50ms` 就是为 SIGWINCH 风暴设的防线，PITFALLS C10）。milestone 明确 per-client「resize 直通（无仲裁）」——**直通如果字面实现为"每帧 RESIZE 直接 TIOCSWINSZ"，浏览器拖窗口一次产生上百帧 RESIZE，每帧一次 ioctl + 对前台进程组一发 SIGWINCH**，vim/htop 被信号洪水打得全程重绘，ttyd 式直通的老坑原样回潮。同理还有 attach 时的 `SignalForegroundGroup`（D-11 强制重绘）——per-client 的进程是刚 spawn 的，首尺寸正确的话这一发完全多余（但留着也无害，属可裁量项）。

**Why it happens:**
「无仲裁」被误读为「无节流」。仲裁器在 v1 身兼两职：min-rect 仲裁（per-client 不需要）**和** 防抖/变化检测（per-client 仍然需要）。把 arbiter 整体跳过，等于把防抖也扔了。

**How to avoid:**
- per-client resize 路径保留**每会话单 timer 防抖 + last 变化检测**：RESIZE → 更新 pending 尺寸 → timer.Reset(50ms) → 到期目标≠last 才 `sess.Resize`。直接复用 arbiter 的 timer 形态但参与集恒为单员，或者抽一个 `resizeDebouncer` 小组件两模式共用（防双写漂移——这正是 PITFALLS C10 当年反对的散点魔法数）。
- spawn 尺寸直接吃 Hello 的 cols/rows（`StartWithSize` 参数化，替代 SpawnCols×SpawnRows 常量 + 首个 RESIZE 纠正的 80x24 首帧窗口）——spawn-on-attach 天然有 Hello 在手，这是 per-client 白送的 UX 改进，顺带消除一次不必要的 SIGWINCH。注意 spawn 尺寸常量的单一事实源纪律（spawn.go SpawnCols/SpawnRows 注释）在参数化后改为"Hello 已 ClampDim 钳制 [1,1000]"。

**Warning signs:**
- per-client 分支的 RESIZE case 里出现直接 `sess.Resize(...)` 无 timer。
- UAT 拖动窗口时服务端 strace 里 TIOCSWINSZ 频率 = RESIZE 帧频率。

**Phase to address:** P13

---

### Pitfall 8: HUP 免疫子进程泄漏——nohup/trap 程序在断开后永生，泄漏速率 ×N

**What goes wrong:**
per-client 断开清理 = SIGHUP 进程组。但 `nohup cmd`、`trap '' HUP`、tmux/screen 类程序**显式忽略或处置 SIGHUP**——断开后这些进程活着，master fd 还连着，ReadLoop goroutine 还在 drain，会话资源一样不释放。v1 的 stop-signal 序列有 `--stop-timeout` 补 SIGKILL 的设计（07-04），但**默认值 0 = 不补 KILL**（合法现状语义）；在 shared 模式这没问题（HUP 不死的进程本来就是用户要的会话保持场景），在 per-client 模式这就是**每次断开都可能泄漏一整棵进程树**——而且泄漏与 Pitfall 4 的 churn 叠加：攻击者每轮循环 spawn 一个 nohup 进程，maxClients 名义上 32，实际驻留进程无界增长。

**Why it happens:**
`--stop-timeout=0` 的默认是在「断开不退出、子进程继续运行」的 shared 语境里定的（P5 产品承诺）。per-client 的产品语义相反（ttyd 语义：断开=进程该死），默认值的产品前提翻转了，但配置零值兜底机制（server.go New 的 `if opts.StopSignal == 0` 形态）会把旧默认无意识带过来。

**How to avoid:**
- per-client 断开清理**强制有 KILL 兜底**：复用 `--stop-timeout` 通道（Options 单一通道纪律——双写即漂移，07-04 注释），但 per-client 模式下 0 的语义需要重议：推荐 per-client 模式给非零默认（如 5s）或文档红线明示「--stop-timeout=0 + per-client = HUP 免疫进程泄漏，风险自留」。这是需要用户裁决的公开契约变更，登记 Key Decisions。
- 兜底时序复用 v1 形态：SIGHUP → AfterFunc(stopTimeout) 补 SIGKILL（ESRCH 幂等静默），但**挂到每会话 teardown 序列**（Pitfall 3），不是 hubMu 内 sleep。
- UAT 必做：`trap '' HUP; sleep 1000` 作为命令启动 per-client，断开 WS，断言 stop-timeout 后 pgid ESRCH、master fd 关闭、goroutine 回落。

**Warning signs:**
- 断开路径代码里只有 SIGHUP 没有后续 KILL 调度。
-  soak 测试（churn + nohup 命令）后 `ps` 见到驻留子孙。

**Phase to address:** P12

---

### Pitfall 9: darwin 共享 kqueue watcher 的 N 规模重估——dup watch 影子注册 + Q1 裁决的规模外推

**What goes wrong:**
reap_darwin.go 的 exitWatcher 是进程级单例（一个 kqueue fd + 一个 loop goroutine + `subs map[int]chan<- struct{}`），注释里明写「兜底路径休眠……watcher 代码以 build tag 保留，**待 Phase 5 多会话时重估**」——Phase 5 没有引入多会话，**这个重估挂账现在到期**。三个具体风险：
1. **dup watch 影子注册**：kqueue 的 kevent 以 (ident, filter) 为唯一键——同一 pid 重复 EV_ADD 是**替换**而非叠加，而 `subs[pid] = ch` 也是覆盖。两个会话绝无共享 pid（僵尸期内 pid 不复用，安全），但若未来代码路径出现重复 watch 同 pid（比如 teardown 重入），先注册者的 channel 被影子化，awaitExit 的 `<-exited` 永远等不到 → **该会话收割挂死**（goroutine 泄漏 + EXIT 永不送达）。当前 `watch()` 对重复 pid 无任何防御。
2. **Q1 裁决的规模外推**：TestKqueueExitNormal/ZombieRace 证明的是"单/少进程 + 注册晚于退出"场景下 kqueue 补发 NOTE_EXIT。N 并发退出（Shutdown 向 32 个进程组同时发信号）+ 注册竞态的组合没测过——loop 每轮 `kevent` 只取 8 个事件的批量在 burst 下没问题（循环续取），但裁决证据需要 N 规模复演。
3. **subs map 生命周期**：watch 注册失败已摘除订阅（Rule 2 修复）；但正常路径里 NOTE_EXIT 到达前会话被 teardown，订阅残留到子进程真正退出才被 loop 清——per-client churn 下 subs map 会随在飞会话数波动，需要确认无单调增长路径（loop 的 delete 是唯一收口的另一触发源——kill 后子进程必死，必到，OK；但要防「watch 后永不退出」的泄漏与 Pitfall 8 叠加）。

**Why it happens:**
watcher 是 v1 为「零每会话线程」设计的提前优化，其正确性论证全部在单会话规模做出。N 会话不是"自然成立"——每条不变量都要在新规模下重新过一遍。

**How to avoid:**
- `watch()` 加防御：重复 pid 注册直接返回错误（fail-closed），调用方退化为直接 `cmd.Wait()`（既有兜底形态）——影子注册从「挂死」变「可观测的错误」。
- reap 测试扩展：**N 路并发 spawn + 并发退出 + 混合僵尸竞态**（darwin CI 跑），Q1 裁决结论按 N 规模重登记。
- Linux 侧无需动作（pidfd 每进程独立，N 规模天然成立——reap_linux.go 注释已论证），但双平台收割语义等价性测试（退出码/信号死亡两形态）应以 N 并发跑。

**Warning signs:**
- `subs[pid] = ch` 赋值前无 `_, dup := w.subs[pid]` 检查。
- darwin CI 的 reap 测试数与 v1 相同（没有 N 并发新用例）。

**Phase to address:** P11（watcher 重估随生命周期主干）、P14（N 规模测试）

---

### Pitfall 10: Shutdown/healthz/metrics/audit 观测面与收口面的模式适配——N 进程组与零身份 label 红线

**What goes wrong:**
四个既有面的隐性单会话假设：
1. **Shutdown（1001 优雅下线）**：v1 序列 = 广播 1001 → `stopChildLocked`（打 `s.sess` 一个组）→ 返回，exitf 由 lifecycle 的子进程死亡收口。per-client 下有 N 个进程组要打；而且 exitf 的收口前提再次断裂（同 Pitfall 1）——N 条会话的 Wait 全部返回才安全退出，否则 SIGHUP 已发但服务端抢在子进程死透前 exit，容器无 init 场景留僵尸（虽然很快会被收，但退出码/审计 session_end 事件全丢）。反过来无限等又会被 D-state 不可杀进程拖死。
2. **healthz `session_active`**：四字段键集白名单（D-07），shared 语义 = 唯一 PTY 会话存活。per-client 下"零会话"是健康常态，直接映射会让探活器在空闲时看到 session_active=0 误判半死。
3. **metrics 17 series 契约**：metricsSeries17 测试侧镜像锁定清单与序。per-client 需要新 series（sessions_active/sessions_total/spawn_failures_total……），**红线是零身份 label**（remote/remote_user/client_id/pid 永不进 label——隐私 + 基数纪律，metrics.go:15-18）。per-client 粒度观测的诱惑 = 按 client_id/pid 打 label → 基数爆炸 + 隐私破窗。
4. **audit 事件**：session_start/session_end 现在是进程级（pid 键、无 remote——New 尾部 goroutine 启动前 emit 一次）。per-client 下每 attach 一对事件，必须挂关联键让「哪条会话属于哪个连接」可检索（D-20 的 client_id=attachSeq 关联检索先例）；不加键就是审计盲区，加错键（remote 进 label/事件值带 ticket）就是红线事故。

**Why it happens:**
观测面与收口面在 v1 是按「进程即会话」建模的，每个断言背后都有测试锁定（TestHealthz*/TestMetricsExposition/events_test）——模式适配时这些测试会"帮你"保持旧语义，改起来处处红灯，诱惑是绕过而不是重议语义。

**How to avoid:**
- Shutdown per-client 序列：广播 1001 → 对**活动会话表快照**逐组 stop-signal（复用同一 stopSignal/stopTimeout Options 通道）→ **有界 join**（stopTimeout + 余量上限）后无条件经 termOnce 退出——不等 D-state，不丢 session_end（界限内正常死亡的会话事件已 emit）。
- healthz：per-client 模式 session_active 重定义为「服务端可接受新会话」（= !draining 且进程未终结），或改报 sessions_active 计数——键集白名单不动，语义变化写进 README（文档即被测物先例）。
- metrics：新增 series 只加 counter/gauge 无身份 label（`wesh_sessions_active` gauge / `wesh_sessions_total` counter / `wesh_pty_spawn_failures_total` counter），metricsSeries17 镜像同步扩为 N 条并锁序；**禁止任何 per-client label**，per-client 明细一律查审计日志（SEC-07 的"日志归因 vs metrics 聚合"分工先例）。
- audit：session_start/session_end 在 per-client 模式增加 `client_id` 键（attachSeq，与 attach/detach 事件同键关联）；值域与红线（无 ticket/凭据/token）逐项过 SEC-01 检查单。

**Warning signs:**
- Shutdown 代码里 `SignalGroup` 调用点仍只有一处且参数是单例 sess。
- 新 metrics 带任何非 version 的 label。
- session_start 事件在 per-client 模式下没有 client_id 键。

**Phase to address:** P12（Shutdown N 组）、P13（观测面）

---

### Pitfall 11: 测试矩阵爆炸与「零回归」证伪——×2 不是全部测试跑两遍，而是分层归类

**What goes wrong:**
直觉方案「全部测试 × 2 模式」会直接爆炸：v1 服务端测试约 40+ 文件，乘 2 后 CI 时长翻番，更糟的是**很多断言在两模式下真值不同**（EXIT 广播 vs 私有化；fanout 逐字节一致 vs 各自进程输出；重连同 pid vs 新 pid）—— naive 参数化会把断言改写成"两模式都接受"的和稀泥形态，**shared 模式的零回归证据就此毁灭**（测试不再证明 shared 行为逐字节不变）。反向陷阱同样致命：只给 per-client 写新测试，shared 老测试原样跑——shared 路径被接缝重构（Pitfall 6）碰过的每个函数都没有被证明等价。

**Why it happens:**
测试的真实需求是三维分类而非一维翻倍：mode-agnostic（协议/认证/限流——与进程模型无关）、mode-mapped（同一需求两模式断言分叉）、mode-exclusive（仲裁器只在 shared 装配；spawn 失败只在 per-client 存在）。没有这个分类，参数化就是盲目乘二。

**How to avoid:**
- **共享 harness + 模式参数**：`newTestServer(t, mode)` 单一装配点；Go 1.24 的 `t.Run("mode=shared"/"mode=per-client")` 子测试结构；断言分叉点显式写成表（mode → expected），**禁止断言放宽**（shared 列的期望值必须与 v1.0 逐字一致——这就是零回归证据）。
- 分类清单（下文「既有测试的双模式归属」表已给出逐文件归类）——**shared-only 测试（仲裁/信用门/owner 递补）在 per-client 下以"组件未装配"形态存在**（断言不装配，而不是强行跑）。
- -race 双模式全量入 CI 门（per-client 引入的新 goroutine 交互——per-session teardown × detach × lifecycle——是 -race 的主猎场）。
- UAT 脚本同样分层：协议层 phaseNN.mjs 新增 per-client 全链脚本（两客户端不同 pid、断开 SIGHUP、EXIT 私有化、重连新 pid、--once 退 255）；既有 phase02-09 脚本**默认模式原样重跑零修改**（零回归的脚本级证据）；Windows Playwright 层跑 herdr 端到端（milestone 目标场景）。
- fuzz/config：session_mode 键进 FuzzDecodeFileConfig 语料 + TestConfigMerge/Precedence/RedLines 更新（新键入白名单、非法值 parse 期拒绝、值不敏感可回显）。

**Warning signs:**
- 断言里出现 `if mode == ... { /* 跳过核心断言 */ }`。
- CI 只有一个模式的 -race 跑法。
- shared 模式老测试的期望值被改动过（git diff 审查点）。

**Phase to address:** P14

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| 散点 `if s.perClient` 分支替代 session 接缝 | 第一版快 2-3 天 | 双模式腐化（Pitfall 6），后续每个 bug 修两遍或漏一遍 | never |
| per-client 复用全局 hubMu + onChunk 扇出找属主 | 零新组件 | N 会话输出在全局锁串行，信用门语义错乱 | never（1:1 路由是必须） |
| 复用 `s.exiting` 当 per-session 终结门 | 少一个字段 | 审计 reason 污染 + exit-when-empty 被自己毒死（Pitfall 1/6） | never |
| 断开即 `Close(master)` 不做 SIGHUP/Drain | 代码最短 | 孙进程泄漏、hangup 时序不可控、最后输出丢失 | never |
| 断言放宽使同一测试两模式都过 | 测试矩阵「覆盖」账面达标 | shared 零回归证据毁灭（Pitfall 11） | never；用断言分叉表 |
| per-client 跳过 resize 防抖 | 「直通」字面实现省事 | SIGWINCH 风暴回潮（Pitfall 7） | never |
| per-client 下 `--stop-timeout` 沿用 0 默认 | 零配置面变更 | HUP 免疫进程泄漏（Pitfall 8） | 仅文档红线明示 + 用户裁决后可接受 |
| write-policy=owner 在 per-client 静默忽略 | 不加校验 | 用户以为有仲裁实际全员 rw，配置语义谎言 | 仅 validateStartup warn 明示可接受；静默永不 |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| exit-when-empty × per-client | 照搬 stopChildLocked→无子等→永不退出（P1） | 空迁移直接 termOnce(255)，Key Decisions 登记第二终结源 |
| --once × per-client | 同上（--once ≡ max-clients=1 + grace 0） | 同一空迁移路径覆盖；UAT 断言退 255 |
| Shutdown × N 进程组 | 只打 s.sess 单例 / 无界等全部 Wait | 活动会话表快照逐组信号 + 有界 join 后 termOnce |
| maxClients 容量闸 × spawn | 沿用 registry.n 注册后计数，spawn 窗口超编 8 进程 | spawn-intent 预占/回滚，或显式裁决登记超编接受 |
| 前端自动重连 × spawn 失败 | 失败关闭码误用 1006 → 前端无限重连 fork 循环 | Error 帧 + 1008/1011（不在 shouldReconnect 码集） |
| arbiter × per-client | 整体跳过 → 防抖丢失（P7）；或强行装配 → 单员 min-rect 空转 | 抽共用 debouncer；参与集概念不装配 |
| SignalForegroundGroup × per-client attach | 照搬 D-11 强制重绘 | 新进程首尺寸正确即多余；spawn 直接吃 Hello 尺寸 |
| --uid/--gid × per-client spawn | 认为降权只在启动 spawn 有效 | Credential fork 期生效天然 per-spawn；NoSetGroups euid 判定不变；TestDropPrivilegesSelf 双模式跑 |
| --auth-header/SEC-07 × per-client | 借 per-connection spawn 回潮「请求头进子进程 env」 | 维持 v1 收窄裁决（审计归因 only）；per-connection env 是 ?arg= 同级注入面，v1.1 不做 |
| 配置面 × session_mode | TOML 键加了但 fuzz 语料/红线测试没更 | DisallowUnknownFields 白名单 + 非法值 parse 拒绝 + 语料扩展同 PR |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| 断网惊群：N 浏览器同时重连 = N 个 fork+exec | 网络恢复瞬间 CPU 尖峰、spawn 延迟毛刺 | 全局 spawn 令牌桶（P4）；前端退避已有 30s 封顶 | ~20+ 客户端同会话断线重连 |
| 每连接 goroutine 账面膨胀 | wesh_goroutines 随驻留客户端线性涨（每客户端 ~6：reader/writer/pinger/ReadLoop/inputWriter/Wait+teardown） | 账面可接受（32×6≈200）；但 teardown 泄漏会被它直接暴露——把该指标当泄漏哨兵 | churn 下不回落即泄漏（P3/P8） |
| 每会话队列内存 ×N | RSS = 32×(512KiB outbox + 256KiB inputQ) 最坏 ~24MiB + 每进程 RSS（bash ~5MB → 160MB） | maxClients 的 per-client 标定写 README；考虑 per-client 默认 cap 重议 | 默认 32 进程在低配 VPS |
| 孙进程持有 slave → Drain 串行化 | Shutdown 逐会话 Drain 200ms × N 串行 = 数秒关停 | 会话 teardown 全部并发（各自 goroutine），join 只等有界上限 | N=32 全员 nohup 场景关停 |
| RESIZE 直通无防抖 | 拖窗口时 ioctl/SIGWINCH 频率爆炸 | 每会话 debouncer（P7） | 任何拖窗操作 |
| darwin watcher 8 事件批量 | Shutdown 瞬间 32 个 NOTE_EXIT | loop 循环续取天然消化；无需改 | 无需处理（验证即可，P9） |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| kill-after-reap 打中复用 pgid | 误杀宿主机无关进程组（完整性/可用性事故） | 每会话 reaped 栅栏（P2）；信号与 reap 锁内序列化 |
| 已认证 churn 无 spawn 限速 | 合法票据 fork bomb + nohup 泄漏叠加绕开 maxClients | spawn 双桶（P4）+ KILL 兜底（P8） |
| spawn 失败 Error 帧拼 err.Error() | 泄露服务器路径/errno/文件系统布局 | 通用文案 + 机器串；细节只进服务端 stderr |
| 借 per-connection spawn 回潮请求头/env 注入 | SEC-07 收窄裁决被推翻，?arg= 同级注入面重开 | v1.1 明确不做；审计归因 only（P10 validateStartup 注释锚定） |
| EXIT 帧广播习惯带过来 | 客户端 B 看到客户端 A 进程的退出码/信号（会话状态越权可见） | EXIT 私有化写序（P3）；测试断言 B 零字节 |
| metrics per-client label | client_id/pid/remote 进 label = 隐私破窗 + 基数爆炸 | 零身份 label 红线扩到新 series；明细走审计日志 |
| spawn-intent 超编被当安全边界利用 | 8 进程超编窗口跑未授权命令（容量策略非安全边界，A5 同族） | 显式裁决登记；不宣称 maxClients 是进程数硬安全界 |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| spawn 失败静默断开 | 黑屏 + 莫名断开，无法区分网络问题与服务端问题 | Error 帧（spawn_failed）+ 1011 + 前端专属文案（P5） |
| 重连=新进程但屏幕残留旧会话输出 | 用户以为接回了原会话，在错误的上下文里继续敲 | Welcome 携会话/epoch 标识（可选键，协议未知字段忽略纪律兼容）；前端检测 epoch 变化 → reset 清屏 + 提示「新会话」；shared 重连同 epoch 不动（v1 观感保持） |
| spawn 延迟无反馈 | attach 到 Welcome 之间白屏（正常 fork 毫秒级，但 EMFILE/限速排队时可感知） | 前端 attach 中已有连接中状态；spawn_throttled 专属文案 |
| ro 分享链接在 per-client 的语义 | 旁观者拿到自己的 shell 但只读——与 shared「看同一个会话」语义完全不同，用户困惑 | README/分享页文案按模式分化（ro per-client = 独立沙箱只读视图） |
| write-policy=owner + per-client 静默无效 | 用户以为有主写仲裁，实际全员各自 rw | validateStartup warn 或拒绝组合（裁决项），README 模式语义表 |

## "Looks Done But Isn't" Checklist

- [ ] **exit-when-empty per-client：** 看起来「--exit-when-empty 双模式都跑通」——常漏验证**宽限到期**形态在 per-client 真的退出（无子等陷阱 P1）；验证 grace>0 + 无重连 → 进程退 255
- [ ] **断开 SIGHUP：** 看起来「断开进程死」——常漏 reaped 栅栏（P2）与 HUP 免疫 + stop-timeout KILL（P8）；验证 `trap '' HUP` 命令 + churn 压测后无驻留进程、无 goroutine 爬升
- [ ] **EXIT 私有化：** 看起来「子死客户端收到 EXIT」——常漏断言**另一客户端零感知**（连 detach 事件都不该有）；验证 A 的进程退出时 B 的会话逐字节无扰动
- [ ] **容量闸：** 看起来「maxClients 满员 503」——常漏 spawn 窗口超编（P4）与 spawn 限速桶；验证 8 并发握手 + 满员边界 + churn 30s 资源有界
- [ ] **kqueue watcher：** 看起来「darwin CI 绿」——常漏 N 并发退出 + dup watch 防御（P9）；验证 32 路并发 spawn/exit + 重复 watch 报错退化
- [ ] **Shutdown：** 看起来「1001 广播后退出」——常漏 N 组信号 + 有界 join（P10）；验证 3 客户端 per-client Shutdown 后全部 pgid ESRCH 且 session_end 事件数=3
- [ ] **shared 零回归：** 看起来「老测试全绿」——常漏老测试**期望值被改写**过（和稀泥断言 P11）；验证 git diff 审查 shared 列期望值逐字未动 + phase02-09 UAT 默认模式零修改重跑
- [ ] **观测面：** 看起来「metrics 多了几条 series」——常漏 metricsSeries17 镜像同步、零身份 label 红线、session_start 的 client_id 关联键（P10）

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| 孤儿进程泄漏（P2/P8 失守） | LOW | wesh 子进程全部 setsid 独立会话：`pkill -HUP -g <pgid>` 或按 PPID=1 + 启动命令特征清理；根修后重启 wesh |
| EMFILE 级联（P5 失守） | MEDIUM | 拒绝新 spawn（fail-closed）保既有会话；ulimit -n 与 kernel pty 上限标定进 README；泄漏点热修 |
| 模式分支已腐化（P6 失守） | HIGH | 回到 seam 重构：以「shared 与 v1.0 逐字节等价」为锚，把散点分支逐个收编进接缝；每收编一处跑双模式矩阵 |
| exit-when-empty 永不退出已上线（P1） | LOW | 进程级兜底：外部 systemd watchdog/手动 kill；根修为 P13 退出路径重建 |
| 审计语义污染（exiting 误置） | MEDIUM | 日志侧可用 code/reason 组合过滤修正检索；根修字段分层后补 events_test 锁定 |
| spawn 限速误伤合法用户 | LOW | 桶参数 CLI 可调（若裁决开放）；1008 关闭码语义保证前端停手，用户手动刷新即恢复 |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| P1 空即不死（exit-when-empty/--once） | P13（P10 矩阵先行） | emptyexit_test 双模式 + UAT：grace 0/宽限取消/宽限到期 × per-client 退 255 |
| P2 kill-after-reap 误杀 | P11 | reaped 栅栏白盒测 + churn 压测无跨组信号（占位进程组注入） |
| P3 teardown 恰好一次 | P11 | -race 双模式全量 + 断开/子死竞态注入测试 + removeLocked 单点记账不变量 |
| P4 fork bomb / 超编 | P12 | 限速桶单测 + 8 并发 spawn 超编测试 + churn 30s 资源有界断言 |
| P5 spawn 失败四连坑 | P11/P12 | spawn 失败注入桩：清理清单逐条 + fd 计数不变 + Error 帧/1011 断言 + 他端零感知 |
| P6 模式分支漂移 | P10 | 接缝代码评审（分支仅在新装配段）+ shared 路径与 v1.0 等价断言 |
| P7 SIGWINCH 风暴 | P13 | per-client resize 风暴测试（百帧 RESIZE → TIOCSWINSZ 次数有界） |
| P8 HUP 免疫泄漏 | P12 | `trap '' HUP` UAT：KILL 兜底后 pgid ESRCH |
| P9 kqueue N 规模 | P11/P14 | darwin CI：N 并发退出 + dup watch fail-closed 测试 |
| P10 Shutdown/观测面 | P12/P13 | shutdown_test 双模式（N 组信号+join）+ metrics 镜像扩展 + events_test client_id 键 |
| P11 测试矩阵 | P14 | 归类表落地 + CI 双模式 -race 门 + shared 期望值 diff 审查 |

## 既有测试的双模式归属（P14 输入）

| 测试 | 归属 | 说明 |
|------|------|------|
| handshake_test / limits_test / keepalive_test | 双模式同断言 | 协议守卫/上限/保活与进程模型无关 |
| auth* / origin / throttle / tickets / sharetoken / tls / proxy* / basepath / customindex | 双模式同断言 | 认证传输面 mode-agnostic（跑两遍成本低，防接缝误碰） |
| e2e_test（生命周期五测） | 双模式断言分叉 | per-client：断开即 SIGHUP、重连=新 pid（与 CORE-05 shared 断言相反） |
| exit_test（EXIT 广播） | 双模式断言分叉 | shared=全员广播；per-client=属主私有化 + 他端零感知 |
| emptyexit_test（--once/--exit-when-empty） | 双模式断言分叉 | per-client 退出路径=空迁移 terminate（P1） |
| shutdown_test / stopseq_test | 双模式断言分叉 | per-client=N 组信号 + 有界 join（P10） |
| resize_test（单端 last-wins） | 双模式断言分叉 | per-client=直通 + 防抖（P7），无 min-rect |
| multi_test（fanout/MaxClients/计数不变量） | 双模式断言分叉 | fanout→各自进程输出；容量→spawn-intent 口径（P4） |
| slowclient_test（1013/信用门） | 双模式断言分叉 | per-client 无信用门：满即踢（R-08 1:1 退化） |
| resize_arb_test / TestGlobalCredit / owner 递补（clients_test 大部） | shared-only | 组件在 per-client 不装配——断言「未装配」而非强跑 |
| exitmsg_test / log_test / events_test | 双模式同断言（扩展后） | 纯函数/事件 schema；client_id 键扩展后锁定 |
| health_test / metrics_test | 双模式断言分叉 | session_active 语义 + series 清单扩展（P10） |
| reap_test / reap_darwin_test | 双平台 N 规模扩展 | N 并发退出 + dup watch（P9） |
| spawn_test（pty 包）/ TestEnvWhitelist / TestStartOptionsDir / TestDropPrivilegesSelf | 双模式同断言 | per-client spawn 必须复用同一 pty.Start 路径（env 白名单/降权/零值等价） |
| config_test / fuzz_test | 扩展 | session_mode 键语料 + 红线 |
| load_test | 双模式各至少一轮 | per-client 重点：fork churn 内存/goroutine 有界 |
| **新增 per-client-only** | — | spawn 失败清理清单、spawn_throttled、EXIT 私有化、断开 SIGHUP pgid、epoch 重连、HUP 免疫 KILL、超编栅栏 |
| UAT phaseNN.mjs | 新增 per-client 全链 + 旧脚本默认模式零修改重跑 | 零回归双证据；Windows Playwright 跑 herdr 场景 |

## Sources

- wesh 代码库一手分析（本研究主证据，HIGH）：`internal/server/server.go`（Attach 守卫区/升档序列/lifecycle/Shutdown/exitf 装配）、`clients.go`（registry 记账/kick/detach/maybeExitWhenEmptyLocked/stopChildLocked/信用门）、`resize.go`（arbiter 防抖与变化检测）、`internal/pty/spawn.go`（Start/whitelistEnv/StartOptions）、`io.go`（ReadLoop/Resize/Drain/SignalForegroundGroup/fdMu 纪律）、`reap_linux.go`（pidfd 收割 D-14）、`reap_darwin.go`（共享 kqueue watcher + Phase 5 重估挂账）、`signal_linux.go`（SignalGroup ESRCH 纪律）、`internal/server/metrics.go`（17 series 契约 + 零身份 label 红线）、`internal/proto/proto.go`（Welcome/Exit 帧格式）
- wesh PROJECT.md Context 节：ttyd 1.7.7 缺陷源码核实清单（pty_spawn 失败 close(0)、每客户端 waitpid 线程、env 泄露——per-client 路径不得再引入的三条）
- wesh v1 Key Decisions：D-10 唯一终结路径、D-13 零新 exitf 分支、D-14 set/grace 分离、accept-255 裁决、1006-only 重连裁决、CR-01 读循环零同步写、R-06/R-07/R-08 记账与锁序纪律
- Context7（MEDIUM）：Go os/exec 并发安全与 pidfd 收割、Process.Kill 不打进程组、WaitStatus 信号语义（研究缓存 key 75ad2d03…）
- Web 佐证（LOW，均与 v1 CI 实测交叉核对）：kill-after-reap PID 复用竞态与 pidfd_send_signal 边界（e23527a3…）；PTY master close hangup SIGHUP 与孙进程持 slave 无 EOF（8a255e2b…）；kqueue (ident,filter) 唯一键语义（FreeBSD kqueue(2)）与 NOTE_EXIT 僵尸补发（08123782…）；per-connection spawn 服务的 fork bomb 防线模式（566333a7…）
- wesh v1 CI 实测裁决（HIGH，一手）：TestKqueueExitNormal/ZombieRace（kqueue 僵尸补发成立）、kick_fail 系列多轮实证（信用门演化记录）、P5-3 本机实证（同尺寸 TIOCSWINSZ 不发 SIGWINCH）

---
*Pitfalls research for: wesh v1.1 — 给共享会话系统新增 per-client PTY spawn*
*Researched: 2026-09-02*
