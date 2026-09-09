# Stack Research — wesh v1.1 per-client 会话模式

**Domain:** Go 单静态二进制 Web 终端（PTY ↔ WebSocket）——新增 ttyd 式 per-connection spawn：每 WS 客户端独立 PTY 子进程
**Researched:** 2026-09-02
**Confidence:** HIGH（核心结论均经本机钉版工具链 `go doc`、GOROOT/GOMODCACHE 源码、仓库一手代码直接核实，非二手资料）

---

## 核心结论：零新增依赖

per-client 会话模式所需的全部能力——N 路并发 spawn、N 子进程收割、per-client TIOCSWINSZ、N 进程资源护栏——**现有六个依赖（creack/pty、coder/websocket、x/sys、x/time、go-toml/v2、stdlib）完整覆盖，不需要任何新库**。

本里程碑的工作量是 **server 包控制面重构**（spawn 时机从启动期移到 attach 期、`pty.Session` 从单例变 N 实例、生命周期 goroutine 从全局收窄为 per-client），**不是栈扩展**。pty 包本体近乎零改动——`Session` 类型已是 per-spawn 设计（`Cmd`/`Master`/`readDone`/`fdMu` 全为实例字段），N 实例化无需结构变更。

---

## Recommended Stack（现有依赖在 per-client 模式的充足性论证）

### Core Technologies

| Technology | Version | per-client 模式中的角色 | Why 充足（核实依据） |
|---|---|---|---|
| Go（os/exec + runtime） | 1.26.3（go.mod 钉版） | **N 子进程收割**：每子进程一个 goroutine 阻塞 `cmd.Wait()` | GOROOT 实证：`syscall/exec_linux.go:312` fork 时自动置 CLONE_PIDFD；`os/pidfd_linux.go:30` `pidfdWorks()` 运行期探测、`:83` pidfdWait 以 waitid(P_PIDFD) 等待。每个 `exec.Cmd` 独立 pidfd——无全局状态、无 PID 复用竞态、无僵尸、退出码经 `*exec.ExitError` 完整。**N 子进程 = N 个阻塞 goroutine，天然并发安全；现有 reap_linux.go 模型逐字泛化，一行不改** |
| github.com/creack/pty | v1.1.24 | **每 attach 一次 `StartWithSize`**：openpty + fork + setsid/setctty | GOMODCACHE 实证（start.go:18-25）：无包级状态，每次调用独立开 pty 对（经 StartWithAttrs → 平台 open()），**并发 spawn 安全**。唯一纪律：它原地改写 `cmd.SysProcAttr.Setsid/Setctty`——spawn.go `Start()` 每次调用内部新建 `exec.Cmd`，已结构性满足「每次 attach 全新 Cmd」要求 |
| golang.org/x/sys | v0.47.0 | per-client TIOCSWINSZ（`Session.Resize`）、TIOCGPGRP+SIGWINCH（`SignalForegroundGroup`）、进程组信号（`SignalGroup`） | 三方法全部 **Session 实例作用域**——Session N 实例化后零改动直接复用。`unix.Getrlimit/Setrlimit` 同包备便（server 自身 NOFILE 启动自检的可选挂点，见护栏节） |
| github.com/coder/websocket | v1.8.15 | 不变。连接生命周期本就 per-client 实例化 | 协议层零改动——wesh.v1 握手/类型化错误帧/合规关闭码全复用；EXIT 帧私有化只是发送范围从广播收窄为单客户端 |
| golang.org/x/time | v0.15.0 | 不变。输入限速本就 per-client（Attach 升档 limiter 构造） | 里程碑既定 ro 门控与限速在 per-client 分支保留——现有 per-client limiter 直接复用 |
| github.com/pelletier/go-toml/v2 | v2.4.3 | `session-mode` 配置键 | 零新解析需求；CLI>env>file 优先级与 DisallowUnknownFields 严格性既有机制覆盖新键 |

### Reaping N children —— 显式故事

**Linux：现有模型直接泛化，零代码改动。**
reap_linux.go 的「每会话一个 goroutine 阻塞 `cmd.Wait()`」在 per-client 模式下 = 每子进程一个 goroutine。Go ≥1.23 的 os/exec 在 Linux 5.3+ 自动走 pidfd 路径（老内核经 `pidfdWorks()` 探测自动回退 wait4，行为不变）：每子进程一个 pidfd，waitid 阻塞收割。D-14 纪律原样有效：**禁手写 pidfd_open / SIGCHLD+waitpid**——第二个收割者与 Wait 竞争会丢退出码。

成本标定（defaultMaxClients=32）：N goroutine × ~8KiB 栈 + N 个阻塞在 waitid 的 OS 线程 + N 个 pidfd——总量可忽略；即便 `--max-clients` 调至数百，runtime 线程上限（默认 10000）仍远在上方。对照：ttyd「每客户端独占一条 waitpid 线程」（pty.c:483）被 v1.0 源码核实记为缺陷；wesh 的 goroutine + pidfd 形态是同语义的正确实现。

**darwin：N 子进程支持已提前落地，平台分叉为零。**
reap_darwin.go 的 `exitWatcher` 是进程级共享 kqueue 单例：`subs map[int]chan<- struct{}` 按 pid 多路订阅、EV_ADD|EV_ONESHOT 触发即自动注销——文件注释原文「N 会话共用（零每会话线程，D-14）」「待 Phase 5 多会话时重估」。**设计上就是 N 会话形态**，per-client 模式直接消费。

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---|---|---|---|
| （无新增） | — | — | — |

spawn / resize / signal / reap / drain 五个能力面全部由 internal/pty 现有 Session 方法承载，无 supporting library 需求。

### Development Tools

| Tool | Purpose | Notes |
|---|---|---|
| `go test -race` | N 实例化后的并发回归 | per-client 分支的 spawn/detach/resize/kill 并发面须 -race 全绿准入（v1.0 既定纪律沿用） |
| web/uat/phaseNN.mjs 协议层 UAT | herdr 场景端到端验证 | 双机拓扑既定：Linux 侧 spawn 真实二进制断言，Windows 侧 Playwright 浏览器实测 |

## 与现有 internal/pty / server 的集成点

1. **spawn 时机迁移（控制面唯一结构性变化）**：main.go:1210 的单一 `pty.Start(argv, StartOptions{...})` 在 per-client 分支改为 spawn 工厂闭包经 Options 注入 server；server 在 Attach 流程 **Hello 校验通过后、registerLocked 升档前** 调用。Hello 载荷已携带 `cols`/`rows` 且经 ClampDim 钳制 [1,1000]（proto.go:95-96, 141-142）——**协议零改动即支持按客户端真实首尺寸 spawn**，消除 80x24 首帧闪烁（herdr 的 per-client area 渲染对初始面积敏感）。
2. **pty 包唯一表面扩展**：`StartOptions` 增加 `Cols/Rows` 两字段，零值 = SpawnCols/SpawnRows（80x24）回落——沿用 spawn.go 既定「零值等价纪律」+ TestStartZeroValueParity 锁定先例。不加这两字段则首帧必闪 80x24。
3. **per-client 生命周期 goroutine**：现有 lifecycle 形态缩小到单客户端作用域——`sess.Wait()` → 退出码/信号提取（exitSignalNum/exitMessage 现有单侧定义复用）→ EXIT 帧发本客户端（非广播）→ `Drain(200ms)` → `Close`。
4. **kill-on-detach**：复用现有 `SignalGroup(stopSignal)` + stopTimeout 补 SIGKILL 机制，按 Session 实例调用；setsid 使 pgid==pid 不变量对每个子进程独立成立，无误杀面。
5. **输入路径**：CR-01 纪律在 per-client 分支仍成立（客户端读循环不得同步写 master——阻塞写破坏断开收口与 D-11 退出保证）。现有 inputQ + input-writer 是会话作用域结构，per-client 模式按客户端实例化一份即可。
6. **spawn 失败投递**：`pty.Start` 错误 → 类型化 Error 帧 + 合规关闭码。wesh.v1 已有错误帧类型；PROJECT.md Context 节明示 ttyd「spawn 失败原因无法传给客户端」是要修复的缺陷——per-client 分支必须兑现（shared 模式 spawn 在启动期，失败即 exit 1，无此面）。

## 资源护栏：进程计数 ≡ 连接计数（显式裁决）

**裁决：不加 OS 级进程限制；per-client 语义下进程计数恒等于连接计数，复用既有 maxClients 闸。**

- 1 注册客户端 = 1 子进程。Attach 守卫区 ③ 位 `registry.n >= maxClients` → 503（server.go:793）在 **Accept 前、spawn 前** 触发——进程数永不超过 maxClients（默认 32）。并发握手瞬时超编 ≤ per-IP 半开帽 8 的既有裁断（v1.0 RESEARCH A5 接受）同样覆盖 spawn 竞态。
- **per-child rlimits：不可行且不做（v1.1 非目标）。** 本机 Go 1.26.3 `go doc syscall.SysProcAttr` 实证无 Rlimit 字段（Go proposal #46279 至今未落地）。仅有的旁路均违反红线：shell 包装 `ulimit` 违反 D-02「exec 数组，绝不经 shell」；父进程 set-rlimit→fork→恢复在并发 spawn 下竞态。**RLIMIT_NPROC 另有语义陷阱：按 real-UID 计数该用户全部进程，设在子进程上会连 wesh 服务端自身一起限死**——不是 per-child 护栏。
- **fd 预算**：每 per-client 会话 ≈ 3 fd（WS 连接 + PTY master + Linux pidfd）。maxClients=32 → ~96 fd，对默认 1024 NOFILE 余量一个数量级。运维调大 `--max-clients` 至数百时：README 记载 `prlimit`/`ulimit -n` 指引；可选增强为 main 启动期 `unix.Getrlimit(RLIMIT_NOFILE)` 自检（x/sys 已是依赖）仅告警不拒绝——是否落地留 phase 裁决，非必需。
- **内存/CPU**：子进程自身消耗由用户命令决定，不属 wesh 栈问题；wesh 侧每客户端成本 = outbox 512KiB + inputQ 256KiB + 常数 goroutine，全部既有有界结构 × maxClients 封顶。
- **metrics/审计粒度红线**：`wesh_session_active`（0/1 gauge）在 per-client 分支改为计数型（活跃会话数）；**禁按 pid/客户端加 label**（OPS-07 零身份 label 红线 + 基数爆炸）。`wesh_pty_output_bytes_total` 由「fan-out 源计一次」变为「N 个 master 各自计入」——同一 counter，N 个递增点。session_start/session_end 审计按 spawn 发（pid 键现有形态）。

## Installation

```bash
# 零新增依赖——go.mod 六行 require 原样保留
# 唯一动作：per-client 分支代码落地後
go mod tidy && CGO_ENABLED=0 go build ./cmd/wesh && go test ./... -race
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|---|---|---|
| 每会话 goroutine + `cmd.Wait()`（现有模型泛化） | 集中式 SIGCHLD 派发器 / 手写 pidfd poll 循环 | 永不——与 os/exec 抢收割权丢退出码（D-14 既定纪律） |
| Hello 后按客户端尺寸 spawn | 80x24 spawn + 首帧 RESIZE 纠正 | 若协议不携带首尺寸才需要——wesh.v1 Hello 已携带 cols/rows，无理由接受首帧闪烁 |
| maxClients 应用层进程计数 | cgroups v2（containerd/cgroups 等库） | 永不适用本定位——需 root/委派、耦合 systemd、破坏单静态二进制零依赖承诺 |
| spawn-on-attach | 预热进程池（pre-spawn 空闲子进程） | 永不——违反 kill-on-detach ttyd 语义；空闲子进程持有 env 快照扩大泄露面；省下的毫秒级 spawn 延迟无感 |

## What NOT to Add

| Avoid | Why | Use Instead |
|---|---|---|
| cgroups 库（containerd/cgroups、libcontainer 等） | root/委派前提 + systemd 耦合 + 二进制体积，与个人运维单二进制定位根本冲突 | maxClients 应用层闸（已有）+ README prlimit 指引 |
| per-child rlimit 的任何实现（shell 包装 / 父进程 set-restore / 新依赖） | os/exec 不表达（Go 1.26.3 实证）；shell 包装违反 D-02 红线；父进程法并发竞态；RLIMIT_NPROC 按 UID 计数会限死服务端自身 | 接受为 v1.1 非目标；资源标定靠 maxClients |
| 手写 SIGCHLD / pidfd 收割器 | 第二个收割者与 `cmd.Wait()` 竞争，丢退出码（D-14） | 现有 per-Session goroutine Wait——N 实例化即泛化 |
| 任何新 WS / PTY / 进程管理库 | 现有六依赖全覆盖 | — |
| metrics 按 pid / 客户端加 label | OPS-07 零身份 label 红线 + 基数爆炸 | 计数型 gauge（活跃会话数），同红线同形态 |
| 前端任何改动依赖 | per-client 是服务端会话拓扑变化；前端重连语义（1006 触发接回）在 per-client 下由「全新进程」语义自然承接，无新前端能力 | 现有 xterm.js 6 嵌入产物原样复用 |

## Stack Patterns by Variant

**shared 模式（默认，零回归）：**
- 装配形态逐字保持现状：启动期一次 spawn、arbiter/fan-out/信用门/resize 仲裁全装配
- per-client 分支不装配的组件在 shared 分支不得有任何行为漂移——准入 = 现有测试套件 -race 全绿

**per-client 模式：**
- Session 按客户端实例化；Resize 直通无仲裁；EXIT 帧私有化；spawn 失败走类型化 Error 帧
- `--once` ≡ `--max-clients=1` 既有语法糖在 per-client 下自然成立（1 客户端 = 1 进程，断开即 exit-when-empty 收口）
- 重连 = 全新进程（ttyd 语义）；前端 1006 重连机制零改动，差异仅在服务端接回的是新 spawn

## Version Compatibility

| Package A | Compatible With | Notes |
|---|---|---|
| Go 1.26.3 构建产物 | Linux ≥5.3 运行时 | pidfd 收割生效前提；`pidfdWorks()` 运行期探测，老内核自动回退 wait4——零代码分支，部署面零感知 |
| Go 1.26.3 构建产物 | darwin（全支持版本） | kqueue EVFILT_PROC/NOTE_EXIT 为 Darwin 恒有 API；watcher 单例 N 订阅已落地 |
| creack/pty v1.1.24 | 并发 StartWithSize | 无包级状态；**纪律：每次 attach 必须新建 exec.Cmd**（StartWithSize 原地改写 SysProcAttr）——spawn.go `Start()` 现状已满足 |
| x/sys v0.47.0 | TIOCSWINSZ / TIOCGPGRP / Getrlimit | 前两者已在用，Getrlimit 同包备便（可选 NOFILE 自检） |

## Sources

- 本机 Go 1.26.3 工具链 `go doc syscall.SysProcAttr`——无 Rlimit 字段实证 — HIGH（钉版工具链地面真值）
- GOROOT 源码 `syscall/exec_linux.go:312`（CLONE_PIDFD）、`os/pidfd_linux.go:30,83`（pidfdWorks/pidfdWait）— HIGH（stdlib 源码直读）
- GOMODCACHE `creack/pty@v1.1.24/start.go:18-25`——StartWithSize 无状态 + SysProcAttr 原地改写 — HIGH（钉版依赖源码直读）
- 仓库一手代码：reap_linux.go / reap_darwin.go（收割模型）、spawn.go（零值等价纪律）、io.go（Resize/Drain/SignalForegroundGroup 实例作用域）、server.go:793（maxClients 预 spawn 闸）、proto.go:95-96,141-142（Hello 携 cols/rows + ClampDim）、main.go:1210（现状单一 spawn 点）— HIGH
- ttyd per-connection spawn 语义参照 — MEDIUM（PROJECT.md 内 2026-08-13 源码核实分析转述 + web 检索交叉；仅作语义参照，不构成实现依据）
- Go proposal #46279（per-child rlimit）状态：截至 Go 1.26.3 未落地 — HIGH（工具链实证缺失即未落地；issue 页抓取被网络策略拦截，不影响结论）

---
*Stack research for: wesh v1.1 per-client 会话模式（milestone 级调研；v1.0 项目级 Go-vs-Rust 选型史见 git 历史 2026-08-13 版）*
*Researched: 2026-09-02*
