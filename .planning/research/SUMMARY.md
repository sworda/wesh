# Project Research Summary

**Project:** wesh v1.1 — per-client 会话模式
**Domain:** Go 单静态二进制 Web 终端（PTY ↔ WebSocket）——新增 ttyd 式 per-connection spawn：每 WS 客户端独立 PTY 子进程
**Researched:** 2026-09-02
**Confidence:** HIGH

> 本文件为 v1.1 里程碑级调研综合（覆盖 2026-09-01/02 四份研究）；v1.0 项目级调研史见 git 历史 2026-08-13 版。

## Executive Summary

wesh v1.1 是在已有 shared 会话模型（单 PTY 子进程 × N WS 客户端）的成熟系统上新增第二条会话路径：每 WS 客户端独立 spawn 一个 PTY 子进程（ttyd 语义）。品类调研确认 per-connection spawn 是该品类的标准答案——ttyd/GoTTY/wetty/shellinabox 四款同类工具全部如此（GoTTY 经 master 源码核实证伪了 v1.0 调研的"共享进程"误记），wesh 的 shared 模型才是品类内独一份的差异化本体。因此 per-client 的定位是**补齐品类表赌注**，shared 保持默认零回归。七项生命周期语义（断开即杀进程组无宽限 / 子死只关本端 / 重连即全新进程 / 尺寸随 Hello / 连接闸即进程闸 / ro=自有进程输入门控 / 分享链接=独立进程入场券）全部有 ttyd 行号证据兜底，且 wesh 协议面（类型化 EXIT 帧含退出码、spawn 失败类型化错误帧）严格优于 ttyd 等价物——零协议改动即可继承。

技术栈结论极为干净：**零新增依赖**。N 路并发 spawn、N 子进程收割、per-client TIOCSWINSZ、进程计数护栏全部由现有六依赖覆盖——Linux 上 Go ≥1.23 的 pidfd 收割模型逐字泛化（每子进程一个阻塞 goroutine，天然并发安全），darwin 的共享 kqueue watcher 设计上就是 N 会话形态。本里程碑的工作量是 server 包控制面重构（spawn 时机从启动期移到 attach 期、Session 从单例变 N 实例、生命周期 goroutine 从全局收窄为 per-client），不是栈扩展。架构形态锁定为「**装配期一次分岔，运行期零分岔**」：不抽象 session 接口（6-7 个显式分支点，每处 ≤10 行），每会话 watcher + 单 supervisor 复用 termOnce 保持「exitf 恰好一次」硬约束，两模式共享面 ≥90%。

最大风险不在 spawn 本身，而在**破坏既有不变量而不自知**：D-10 唯一终结路径、D-13 零新 exitf 分支、唯一收割者纪律、Welcome 恒首帧、零身份 label 红线。11 条陷阱中 5 条 Critical 直接指向：退出路径重建（exit-when-empty 无子可等永不退出）、kill-after-reap 误杀（pgid 复用窗口打中无关进程组）、双路 teardown 竞态、已认证 churn fork bomb（maxClients 限并发不限速率）、spawn 失败四连坑（fd 泄漏 / 失败 UX / EMFILE 级联）。每条均有显式预防策略与测试锁定方案。

## Key Findings

### Recommended Stack

零新增依赖：per-client 全部能力由现有六依赖（creack/pty v1.1.24、coder/websocket v1.8.15、x/sys v0.47.0、x/time v0.15.0、go-toml/v2 v2.4.3、stdlib）完整覆盖。唯一表面扩展是 `pty.StartWithSize`（Start 委托，80x24 单一事实源纪律保持）。详见 [STACK.md](STACK.md)。

**Core technologies:**
- Go 1.26.3（os/exec + runtime）：N 子进程收割——每子进程一个 goroutine 阻塞 `cmd.Wait()`；Linux 5.3+ 自动 pidfd 路径（运行期探测，老内核回退 wait4），无全局状态、无 PID 复用竞态；现有 reap_linux.go 模型逐字泛化，一行不改
- creack/pty v1.1.24：每 attach 一次 StartWithSize——无包级状态，并发 spawn 安全；唯一纪律是每次 attach 必须新建 exec.Cmd（StartWithSize 原地改写 SysProcAttr），spawn.go 现状已结构性满足
- x/sys v0.47.0：per-client TIOCSWINSZ / TIOCGPGRP+SIGWINCH / 进程组信号——三方法全为 Session 实例作用域，N 实例化零改动直接复用
- darwin reap（reap_darwin.go）：进程级共享 kqueue exitWatcher 按 pid 多路订阅、EV_ADD|EV_ONESHOT 自动注销——设计上即 N 会话形态，per-client 直接消费，平台分叉为零
- 资源护栏：进程计数 ≡ 连接计数（1:1），复用既有 maxClients 闸（Accept 前、spawn 前触发）——不加 OS 级进程限制

**显式不做**：cgroups 库（需 root/systemd 耦合，破坏单静态二进制定位）、per-child rlimit（Go 1.26.3 实证不支持，且 RLIMIT_NPROC 按 UID 计数会连服务端自身一起限死）、手写 SIGCHLD/pidfd 收割器（与 `cmd.Wait()` 抢收割权丢退出码，D-14 红线）、预热进程池（违反 kill-on-detach ttyd 语义且扩大 env 泄露面）、metrics 按 pid/客户端加 label（OPS-07 零身份 label 红线）、任何前端改动依赖（xterm.js 6 嵌入产物原样复用）。

### Expected Features

per-connection spawn 是品类默认，七项语义逐项有 ttyd 源码行号裁决。Feature 依赖链：`T1 gates T2 → T2 enables T3/T4/T6, requires T8, feeds T9`；`T11 requires T1-T7 语义冻结`。详见 [FEATURES.md](FEATURES.md)。

**Must have (table stakes，T1-T11 全部 P1):**
- T1 `--session-mode=shared|per-client` flag + TOML 键，默认 shared——模式入口，零回归前提
- T2 attach 后 spawn：Hello 认证通过 → Session.Start，Hello cols/rows 作初始 winsize（消除 80x24 首帧闪烁）；spawn 失败回类型化 Error 帧 + 1011
- T3 断开 → SIGHUP 进程组立即杀，无宽限（ttyd parity；SignalGroup + setsid pgid==pid 不变量既有）
- T4 子进程退出 → 私有 EXIT 帧（含 exit_code）→ 1000 关该客户端；服务端不退、他端零感知
- T5 重连 = 全新进程 + 前端 `terminal.reset()` 清屏（ttyd parity）
- T6 resize 直通无仲裁（但保留每会话防抖——见 Pitfall 7）
- T7 ro 输入门控 + 每客户端限速保留（零改动继承）；T8 maxClients 兼任进程上限 + spawn 前 hubMu 内复检；T9 --once/exit-when-empty 终结 N 进程组；T10 慢客户端有界 outbox + 1013 踢保留；T11 模式语义文档（分享链接/ro 语义变化不文档化 = 功能缺陷）
- herdr 场景 E2E UAT——driving scenario 验收

**Should have (differentiators):**
- D1 私有 EXIT 帧带退出码 + spawn 失败原因可达客户端（ttyd 只给无载荷 1000/1006，1006 还违反 RFC6455）——随 P1 免费落地，UAT 断言锁定
- D2 ro/rw 分享链接在 per-client 下成为「按权限独立 spawn 凭证」——品类无等价物，机制零改动，价值在语义设计与文档
- D3 安全默认全套继承（env 白名单/TLS/Origin/节流/ticket/读上限）——ttyd 已核实缺陷清单的对立面
- D4 metrics/审计 per-client 粒度（活跃会话计数 gauge、spawn_failed 事件、client_id 关联键）——P2，零身份 label 红线不破

**Defer (v1.1.x / v2+):**
- D5 per-connection HTTP 上下文注入子进程 env（WESH_REMOTE_USER）——D-15 收窄理由在 per-client 下结构性消失（ttyd TTYD_USER、gotty --pass-headers 先例），但属 SEC-07 裁决重开，**需用户显式裁决**，v1.1 可不做
- D6 per-PTY 停读/续读背压（ttyd pty_pause/resume parity）——T10 自然反压已够用，体验优化
- A1/A4 重连 reattach 与服务端滚动回放——V2-SESSION 领土，显式不在本里程碑开口子

**Anti-features（永不清单）**：A1 重连接回同一进程、A2 断开 linger 宽限、A3 运行期/按 URL 切模式或选命令（?arg= 注入面前车之鉴）、A5 per-client 设默认（违背零回归承诺）、A6 per-client 仍装配仲裁/hub「以防万一」（1:1 下竞态面是纯负债）、A7 ro 访客共享进程（直接重新引入 driving bug）。

### Architecture Approach

「装配期一次分岔，运行期零分岔」：模式在 New 内一次定型（sessionMode 不可变字段 + goroutine 拓扑分岔 + shared/per-client 互斥校验 fail-fast），运行期热路径零模式判定。Server 保留 `sess` 字段（shared 专用零漂移），新增 spawnFn 会话工厂 + pcSessions 注册表；**不抽象 session 接口**——分支点仅 6-7 处且全部显式（每处 ≤10 行），shared 零回归可由代码评审逐行核对。详见 [ARCHITECTURE.md](ARCHITECTURE.md)。

**Major components:**
1. **spawnFn 会话工厂**（main 闭包捕获 argv+StartOptions，经 Options 注入）——attach 升档序列内调用：ticket 核销之后（SEC-08 预认证零资源）、Welcome 组帧之前、hubMu 之外（阻塞 syscall 绝不持锁）
2. **pcSession + pcSessions 注册表**（hubMu 保护）——每会话状态（sess/inQ/inputDone/startedAt/exitCode）；与 registry 双注册表分工：registry=WS 连接活性，pcSessions=会话活性（客户端断开后会话仍可存活至收割——这是 Shutdown 能覆盖残留者的核心理由）
3. **每会话 goroutine 群 + pcSupervisor 单例**——ReadLoop 闭包 / inputWriter 参数化 / sessionWatcher（Wait → EXIT 私有化 → 关本端 WS）；supervisor 经 hubCond 等待 `(pcExitReq||exiting) && active==0`，复用 termOnce/terminate 单点收口——「exitf 唯一收口」的 per-client 同构映射
4. **client.inQ 间接字段**——升档时把模式相关的输入队列解析为 client 直接字段，读循环 INPUT case 逐行不分支的关键（CR-01 纪律保持）
5. **pty.StartWithSize**（pty 包唯一表面扩展）——Start 委托本函数（SpawnCols×SpawnRows 回落），shared 路径逐字节零漂移

**并发纪律**：hubMu 绝不横跨 spawn；per-client RESIZE 只取 sess.fdMu 不持 hubMu；锁序全序保持无新锁类型。退出码规则 = last-reaped-code，--once 两时序（先断后死 / 先死后断）与 shared 逐位对齐（255 / 子进程码），SESS-01/02 既定 UAT 断言原样成立。goroutine 拓扑：shared 3+3N，per-client 1+6N（maxClients=32 → 193，wesh_goroutines 既有 series 可观测）。**零改动红线**：proto.go（复用 ErrServerError）、web/ 全部前端、resize.go 及认证传输面 11 个文件。

### Critical Pitfalls

11 条全清单见 [PITFALLS.md](PITFALLS.md)（含阶段映射与「看似完成实则不然」检查单）。Top 5：

1. **「空即不死」——exit-when-empty/--once 在 per-client 下无子进程可等，服务端永不退出**——注册表空迁移且活动会话==0 时直接调度 terminate(255)（termOnce 收口保证恰好一次）；这是 D-13 在新模式下的显式重议，**必须登记 Key Decisions**，禁止偷偷加 exitf 调用或在 stopChildLocked 里写 `if s.sess == nil { return }` 静默兜底
2. **kill-after-reap 误杀——pgid 复用窗口内 SIGHUP 打中无关进程组**——每会话状态锁 + reaped 标志，信号与 reap 锁内序列化，误杀结构性不可能；Linux pidfd 收割不覆盖应用层 kill(-pgid)，别指望它兜底；断开路径绝不自己调 Wait（唯一收割者纪律）
3. **断开↔子死双路收口竞态——teardown 必须恰好一次且顺序固定**——每会话 sync.Once + 固定序列（SIGHUP 经 reaped 栅栏 → stop-timeout 补 SIGKILL → Drain(200ms) → Close → Wait 返回 → removeLocked 单点记账）；SIGHUP 可在 hubMu 内发（SignalGroup 不取 fdMu），但 Drain/Close/Wait 绝不能在 hubMu 内（200ms 级阻塞 = 全服务端行头阻塞）
4. **重连 fork bomb——maxClients 限并发不限速率，既有三道闸全部不覆盖已认证 churn**——spawn 双令牌桶（全局 8/s burst 16 防惊群 + per-IP 1/s burst 4 防单点 churn），取不到令牌在 spawn 前拒绝且**关闭码必须避开 1006**（前端只对 1006 自动重连，用 1008/1011 让前端停手、闸自调节）；容量记账改 spawn-intent 口径（预占/回滚）或显式裁决登记超编接受，不许无意识继承 R-06
5. **spawn 失败路径四连坑（fd 泄漏 / 启动预检假设失效 / 失败 UX / EMFILE 级联）**——失败 UX 定型：Error 帧通用文案（**绝不拼 err.Error()** 回显路径/errno）+ Close(1011)（不在前端重连码集）；清理清单化并测试锁定（release half-open → 名额回滚 → 不进注册表 → 仅 spawn_failed 单事件零敏感值）；`wesh_pty_spawn_failures_total` counter 进 metrics 让 EMFILE 级联立即可见

**方法论警告**：这不是在新系统里设计 spawn——是在一个 44/44 需求已收口、每条生命周期不变量都有测试锁定的系统里加第二条进程路径。最大的陷阱不是「spawn 写错」，而是**破坏既有不变量却以为自己没碰它**。零回归证据 = shared 路径与 v1.0 逐字节等价（编译期 + 测试断言双证）；测试断言**禁止放宽**成「两模式都接受」的和稀泥形态（Pitfall 11）。

## Implications for Roadmap

两份研究的阶段骨架（ARCHITECTURE 的 PC-1..PC-4 与 PITFALLS 的 P10..P14）高度互洽，合并为五阶段建议：

### Phase 1: 模式装配与接缝（PC-1 ≈ P10）
**Rationale:** 骨架工作必须第一个做——接缝先行才能防止散点 if/else 半年后腐化（Pitfall 6）；全部 inert（默认 shared 零回归）；先锁定公开契约面（one-way flag 纪律）
**Delivers:** pty.StartWithSize；--session-mode flag + TOML 键 + parse 期枚举校验；Options.SessionMode/SpawnFunc + New 互斥校验；validateStartup per-client 行（exec.LookPath 预检保住「spawn 失败启动期暴露」近似 UX）；配置 fuzz 语料扩展（session_mode 键入白名单 + 非法值 parse 拒绝同 PR）
**Addresses:** T1
**Avoids:** Pitfall 6（模式分支漂移——接缝先行是骨架工作）；为 P1（退出路径）先行明确组合语义矩阵

### Phase 2: per-client 生命周期主干（PC-2 ≈ P11）
**Rationale:** 核心 E2E 成立的最长链路，依赖 Phase 1 的工厂与模式字段；本阶段结束 = 双端双 pid、输出逐字节一致、ro 门、resize 隔离、断开杀进程全部成立
**Delivers:** client.inQ/pc 两字段；升档 per-client 分支（容量再闸 → hubMu 外 spawn → 失败 Error+1011 → Welcome 回显 → 注册+登记）；五 goroutine 装配（ReadLoop 闭包 / inputWriter 参数化 / writer / pinger / sessionWatcher）；EXIT 私有化直写；INPUT 零分支 / RESIZE 直通两 case；detach/kick SIGHUP 挂点（注册表移除点，覆盖一切断开形态）；每会话 teardown Once 序列 + reaped 栅栏；darwin watcher dup-watch fail-closed 防御
**Addresses:** T2/T3/T4/T6/T7/T10
**Avoids:** Pitfall 2（kill-after-reap）、3（teardown 竞态）、5（spawn 失败四连坑）、9 之 dup watch 防御面；Anti-Pattern 1/3/5/6/7

### Phase 3: 资源与容量防线（P12）
**Rationale:** 防线挂点依赖 Phase 2 的 spawn 路径与会话表存在；与 Phase 4 互不依赖可并行，但建议先行——churn 防护缺失会使 Phase 5 压测失真
**Delivers:** spawn 双令牌桶（全局 + per-IP）+ spawn 前拒绝路径（关闭码避开 1006）；spawn-intent 容量预占/回滚（或显式裁决登记超编 ≤8 接受并写进 README）；per-client stop-timeout KILL 兜底默认值重议（公开契约变更，用户裁决项）；Shutdown N 进程组快照逐组信号 + 有界 join（不等 D-state，不丢 session_end）；churn 负载测试（合法票据 10rps × 30s，断言 RSS/goroutine/fd 有界）
**Addresses:** T8 加固（进程帽语义）
**Avoids:** Pitfall 4（fork bomb / 超编窗口）、8（HUP 免疫泄漏 ×N）、10 之 Shutdown 面

### Phase 4: 终结语义与观测面适配（PC-3 ≈ P13）
**Rationale:** 终结语义依赖 Phase 2 的 watcher/pcSessions；独立可切片——Phase 2 可先行人工验证主链
**Delivers:** pcSupervisor + termOnce 收口；exit-when-empty/--once 空迁移 terminate 分支（Key Decisions 登记第二终结源 + accept-255 映射）；last-reaped-code 规则与 --once 两时序逐位对齐验证（255 / exit 42 透传）；每会话 resize 防抖（共用 debouncer 组件防双写漂移）；前端重连 reset（Welcome/prefs 下发模式）；session_start/end per-client 粒度 + client_id 关联键 + spawn_failed 事件；metrics/healthz 模式分支（四个 OQ 逐项裁决落地）；metricsSeries17 镜像扩展 + 零身份 label 红线扩到新 series
**Addresses:** T5、T9、D4
**Avoids:** Pitfall 1（空即不死）、7（SIGWINCH 风暴回潮）、10 之观测面（healthz 语义 / metrics 契约 / audit 关联键）

### Phase 5: 双模式测试矩阵、标定与 UAT（PC-4 ≈ P14）
**Rationale:** 依赖 Phase 2-4 功能完整；零回归的双证据（Go 测试 + UAT 脚本）在此收口；负载数据回填 maxClients 默认建议值与文档义务段
**Delivers:** newTestServer(t, mode) 共享 harness + 三维测试归类表落地（mode-agnostic 同断言 / mode-mapped 断言分叉表 / mode-exclusive 断言不装配）；-race 双模式全量 CI 门；协议层 UAT phaseNN.mjs per-client 全链（双端双 pid / EXIT 不串台 / resize 隔离 / --once 退 255）+ phase02-09 默认模式零修改重跑；Windows Playwright herdr 端到端（driving scenario：is_foreground + per-client area 渲染实测）；并发进程负载矩阵（1/4/16/32 会话：内存/fd/goroutine/吞吐）；README/ARCHITECTURE/CONFIGURATION 双模式文档段（T11）
**Addresses:** T11、herdr E2E UAT（MVP 验收项）、D1/D2/D3 UAT 断言锁定
**Avoids:** Pitfall 11（测试矩阵爆炸 / 零回归证伪）、9 之 N 规模复演面（darwin CI 32 路并发退出）

### Phase Ordering Rationale

- 依赖链：装配阀门 ≺ 生命周期主干 ≺ 终结/资源语义 ≺ 标定/UAT——与 ARCHITECTURE.md §12 一致；每阶段以「shared 全量测试原样绿 + 期望值逐字未动」为收口闸
- Phase 3 与 Phase 4 互不依赖（均只依赖 Phase 2），顺序可换或并行；建议资源防线先行是因为 churn 防护缺失会使 Phase 5 压测失真
- 接缝先行（Phase 1）是防 Pitfall 6 的唯一窗口——散点分支一旦落地，收编成本随时间递增
- D5（env 注入）为独立小变更，待用户裁决后可插入任何阶段；建议 Phase 1 CONTEXT 阶段裁决后挂账
- 前端改动极小（重连 reset + spawn 失败文案），集中在 Phase 4；前端零改动面（ticket / 重连退避 / 标题前缀）已在 FEATURES.md Keep 清单锁定

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 5:** 并发进程资源曲线需实测（唯一 MEDIUM 置信面——32 会话内存/fd 为账面推算，无实机数据）；herdr 端到端场景依赖外部子程序行为，UAT 断言设计需实测标定

Phases with standard patterns (skip research-phase):
- **Phase 1-4:** 全部机制为既有件复用/实例化，研究已给出 file:line 级集成点、锁序、goroutine 拓扑与退出码对齐证明——无生态系统/docs 类遗留问题

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | 钉版工具链 go doc、GOROOT/GOMODCACHE 源码、仓库一手代码直接核实，非二手资料 |
| Features | HIGH | ttyd 本地 clone 行号直读 + GoTTY/wetty 代码检索互证；shellinabox 为 MEDIUM（二手生成文档）但仅作 anti-feature 参照，不构成实现依据 |
| Architecture | HIGH | 全部结论锚定 wesh 源码 file:line 实证 + milestone 上下文既定裁决；本课题为内部集成设计，外部检索无决策增量 |
| Pitfalls | HIGH | wesh 代码库一手逐文件分析 + v1 CI 实测裁决交叉核对；web 佐证均标 LOW 且仅在交叉核对后采信 |

**Overall confidence:** HIGH

### Gaps to Address

- **32 会话资源标定**（内存/fd/goroutine 账面推算，无实机数据）：Phase 5 负载矩阵实测回填 maxClients 默认建议值与 README 资源义务段
- **D5 per-connection env 注入（WESH_REMOTE_USER）**：D-15 收窄理由结构性消失但属 SEC-07 裁决重开——Phase 1 CONTEXT 阶段向用户显式裁决，不在代码里遗留暧昧；v1.1 不做为既定基线
- **per-client 模式 stop-timeout 默认值重议**（0 = 不补 KILL 在 per-client 下 = HUP 免疫进程泄漏，Pitfall 8）：公开契约变更需用户裁决——推荐非零默认（如 5s）或文档红线明示风险自留
- **四个 Open Questions 待 phase 规划逐项落地**（均有推荐答案）：① healthz session_active per-client 语义（推荐恒 true = 服务可接受新会话）；② wesh_session_active series 双语义（推荐同 series 名按模式出计数）；③ 慢客户端 outbox 满 1013 踢（推荐，零新机制）vs 阻塞反压；④ spawn 失败 wire 面复用 server_error/1011（推荐，协议零改动）vs 新增机器串
- **write-policy=owner × per-client 组合**：仲裁不装配使该配置静默失效——validateStartup warn 或拒绝需裁决（静默永不接受）

## Sources

### Primary (HIGH confidence)
- wesh 当前源码逐行分析（server.go / clients.go / resize.go / pty/spawn.go / io.go / reap_*.go / proto.go / metrics.go / health.go / main.go / config.go）——全部集成点、锁序、goroutine 拓扑、退出码论证、既有不变量清单与测试归属表
- 本地 ttyd 1.7.7 源码直读（~/open_src/ttyd）——protocol.c / pty.c / server.c / xterm/index.ts 行号级生命周期语义证据
- 本机 Go 1.26.3 工具链实证——`go doc syscall.SysProcAttr`（无 Rlimit 字段）、GOROOT pidfd 收割路径（exec_linux.go:312 / pidfd_linux.go:30,83）、GOMODCACHE creack/pty StartWithSize 无状态实证
- yudai/gotty master + sorenisanerd/gotty 代码检索——per-connection spawn 实证（handlers.go:114 → local_command.go:32-37；证伪 v1.0 调研"GoTTY 共享进程"误记）
- wesh 项目权威文档——PROJECT.md milestone 上下文与既定裁决、v1.0 Key Decisions（D-02/D-10/D-13/D-14/SEC-07/SEC-08/OPS-07/CR-01）、v1.0 RESEARCH 已锁定结论

### Secondary (MEDIUM confidence)
- butlerx/wetty main 代码片段检索——pty.spawn / disconnect→term.kill 模式互证
- shellinabox deepwiki 生成文档——sessionKey + 输出缓冲 reattach 结构（仅作 anti-feature A1 参照）
- GitHub issue 检索——ttyd#1395（非零退出发 1006 违反 RFC6455）、#1528、#628
- Context7 Go os/exec 文档——pidfd 收割 / Process.Kill 不打进程组 / WaitStatus 信号语义佐证

### Tertiary (LOW confidence，均经交叉核对后采信)
- Web 佐证：kill-after-reap PID 复用竞态与 pidfd 边界、PTY master close hangup 语义与孙进程持 slave、kqueue (ident,filter) 唯一键语义与 NOTE_EXIT 僵尸补发、per-connection 服务 fork bomb 防线模式——全部与 wesh v1 CI 实测裁决（TestKqueueExitNormal/ZombieRace、kick_fail 系列、P5-3 本机实证）交叉核对

---
*Research completed: 2026-09-02*
*Ready for roadmap: yes*
