# Phase 7: 部署与配置 - Research

**Researched:** 2026-08-25
**Domain:** 部署形态（监听/配置文件/反代/子进程管理/降权/优雅下线）——Go 1.26.3 服务端 + TypeScript/xterm.js 6 前端
**Confidence:** HIGH（核心机制全部本 session 源码级核实：GOROOT go1.26.3、GOMODCACHE 在库依赖、现状代码逐行、本机 Node 探针实证；外部 API 语义 CITED pkg.go.dev 官方文档）

## Summary

本 phase 七个需求几乎全部落在**既有架构的预留挂点**上，唯一新依赖是 CONTEXT/STACK 已定案的 pelletier/go-toml/v2。全部 13 个新 flag 的宿主（config struct + fs.Visit 显式设置位 + validateStartup 校验矩阵 + credErr/clientOptErr 记录式上报）已在 main.go 成型；OPS-04 的三个挂点（cmd.Dir / TERM= 行 / SysProcAttr.Credential）在 spawn.go:50,65 注释逐字预留；1001 在 proto.go:13 占位待启用；clientIP/logEvent 的 XFF 消费点（server.go:519-529,585-587）注释已指名 Phase 7。研究发现的**六个计划期必须处理的机制细节**（全部源码级实证）：

① **Go 不会在 bind 前 unlink 既有 unix socket 文件**——GOROOT sock_posix.go listenStream 直接 `syscall.Bind`（本 session 逐行核实，无 unlink 调用），残留 socket 必收 EADDRINUSE；D-10 的 listen 前 os.Remove 是**必需**而非保险。② **socket 文件 mode = 0777 & ~umask（内核行为，Go 不做任何 chmod）**——D-09 的 0660 确定性必须 listen 后 os.Chmod/os.Chown 达成；UnixListener 默认 `unlink: true`（Close 自动删文件，D-10 语义在关闭半侧免费获得）。③ **前端相对路径改造若只做「去掉前导斜杠」会打断分享链接**——页面挂载点有两类（`/` 与 `/s/{token}/`），`/s/{token}/` 下 `fetch('api/attach')` 解析为 `/s/{token}/api/attach`（服务端无此路由，凭据模式落 basicAuth 401 → 误显 Invalid share link）；正解 = share 正则不锚 ^ 兼作挂载点检测 + `../../` 升级前缀（D-14 三改是同一机制的两半，本 research Pattern 3 给出完整形态）。④ **Go 1.22+ mux 的 307 尾斜杠规范化免费可用但有前提**：必须注册 `/wesh/` 子树模式，matchOrRedirect 才会对 `/wesh` 追加斜杠重匹配并发 307（GOROOT server.go:2687,2721-2745 逐行核实）；share 路由 `/wesh/s/{token}/` 同理。⑤ **降权 Credential 与 creack/pty 完全兼容**：StartWithSize 只补 Setsid/Setctty 两字段不覆盖调用方 SysProcAttr（GOMODCACHE start.go:18-25 逐行核实）；forkExec 子进程顺序 setsid → setgid/setuid → TIOCSCTTY（GOROOT exec_linux.go:385,490-510,632），降权后设 ctty 走继承 fd 无权限问题。⑥ **零依赖 UAT 无法让 Node 原生 WebSocket 直连 unix socket**（本机探针：无全局 Agent、无 node:undici builtin、无裸 undici 包）；正解 = 15 行 TCP relay 管道转发（本机探针实证），复用全部既有 WS 断言机制。

**Primary recommendation:** 按七条主线拆 plan——(1) 配置文件（go-toml 严格模式 + 两阶段合并 + fs.Visit 消费）；(2) 监听形态（--socket 三 flag + Remove/Chmod/Chown 序列 + validateStartup 矩阵扩展）；(3) base-path（mux 模式串前缀装配 + StripPrefix 仅静态伺服 + 前端相对 URL 含 share 升级前缀）；(4) auth-header/XFF（HTTP 层提取 + logEvent 扩字段 + clientIP 信任闸）；(5) 子进程管理（--cwd/--term + pty.Start 选项化 + stop-signal 序列）；(6) 降权（--uid/--gid + Credential + 白名单身份改写）；(7) 1001 优雅下线 + --open（signal.NotifyContext + EXIT 帧广播先例复用 + onclose 1001 分派）——每条均有前序 phase 逐字先例或本 research 源码级配方。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**配置文件（OPS-09）**
- **D-01:** 路径发现 = **仅 `--config` 显式指定**，不做任何隐式默认路径搜索——裸 `wesh -- bash` 行为与今天完全一致，零意外；systemd 部署显式路径最可控 — **Reversibility:** one-way — CLI flag 公开契约
- **D-02:** 可重复 flag（--credential/--origin/--client-option）与配置文件同名列表的合并 = **CLI 给出则替换整个列表**——与 P3 D-01 WESH_CREDENTIAL env 兜底「flag 非空则整体忽略」先例一致；CLI 能完整表达最终状态；标量 flag 自然是 CLI 显式设置则覆盖（fs.Visit 显式设置位模式承载） — **Reversibility:** one-way — 合并语义是部署行为契约
- **D-03:** TOML 形状 = **平铺 key = value，键名 = flag 名**——心智成本零，help 文案与配置文档单一事实源；go-toml v2 直解到与 config struct 同构类型；拒绝分组 sections
- **D-04:** 覆盖面 = **全部长期运行 flag 可入配置 + `command = ["bash", "-l"]` exec 数组形式可入**（CLI `--` 后 argv 非空则覆盖）；边界：--no-auth/--insecure-http 逃生门、--version/--help/--config 本身不入配置文件
- **D-05:** 优先级链 = **flag > env > 配置文件 > 内置默认**——与 P3 D-01 现状完全兼容；env 作为 systemd EnvironmentFile= 600 通道仍优先于配置文件明文
- **D-06:** 加载失败与未知键 = **exit 2 fail-fast 严格模式**——文件不存在/TOML 解析失败/未知键均拒绝（未知键拒绝防拼写错误静默失效）
- **D-07:** 文件权限检查 = **含 credential 键且权限非 600/400 时 stderr 警告放行**（不阻断——挂载盘/容器 secret 权限语义不可靠）+ README 明示建议 chmod 600

**监听形态（OPS-01）**
- **D-08:** UNIX socket = **独立 `--socket /run/wesh.sock` flag，与 --port/--bind 互斥**（组合冲突进 validateStartup fail-fast） — **Reversibility:** one-way — CLI flag 公开契约
- **D-09:** socket 属主/权限 = **`--socket-mode 0660`（八进制，默认 0660）+ `--socket-owner user[:group]`**（user.Lookup 解析）；两 flag 仅随 --socket 有意义，单独给出 = 配置矛盾 fail-fast — **Reversibility:** one-way — CLI flag 公开契约
- **D-10:** 既有 socket 文件 = **listen 前 os.Remove**（IPC 端点残留即垃圾；不 unlink 则 bind EADDRINUSE；systemd Restart= 场景零人工干预）
- **D-11:** unix socket 形态下 validateStartup 的 bind 安全校验矩阵 = **本机信任跳过**（loopback 早退同款逻辑）——访问控制由 socket 文件权限位承担，文件系统权限即认证边界
- **D-12:** unix socket 形态下启动打印 = `listening on unix:///path` 实际地址；分享链接两行退化为 **unix:// 提示行**（明示反代后链接由反代 URL 决定）——无 host:port 可拼时绝不拼误导性 TCP 链接

**base-path 反代挂载（OPS-02）**
- **D-13:** `--base-path /wesh` 值校验 = **严格模式：必须以 / 开头、不得以 / 结尾（根 / 视为未配置）、拒绝 .. 与重复斜杠、仅 URL path 安全字符**——parse 期规范化+校验（NormalizeOrigin 先例），非法值 exit 2；拒绝宽容自动修正 — **Reversibility:** one-way — CLI flag 公开契约
- **D-14:** 前端 URL 构造 = **改相对路径**（`fetch('api/attach')`、`new URL('ws', location.href)`、share 正则不锚 ^）——go:embed 静态伺服零改动、无模板注入面；**配套硬要求：`/wesh` → `/wesh/` 尾斜杠 307 规范化**；拒绝服务端注入 base；Origin 校验不受影响；分享链接打印含 base-path

**auth-header 透传与 X-Forwarded-For（SEC-07）**
- **D-15:** SEC-07 语义收窄 = **只要审计归因：attach 时 logEvent 记录 remote_user**——共享进程模型下 per-client env 注入结构性不成立；SEC-07 需求文本修订为服务端侧身份记录，README 明示与 ttyd -H 的模型差异 — **Reversibility:** one-way — 需求文本修订 + README 公开承诺的模型差异说明
- **D-16:** 信任模型 = **裸信任 + 暴露面启动警告**：`--auth-header X-Remote-User` 配置即信任该头（ttyd 同款）；validateStartup 检测 bind 非 loopback 且无凭据时 stderr 警告「auth-header 可被直连伪造，确保 wesh 不直接暴露」
- **D-17:** 与认证体系关系 = **正交提取**：只做用户名提取进 logEvent，不做任何认证决定；Basic/--no-auth/share token 语义全不变
- **D-18:** 头名 = **`--auth-header` 可配头名（单个）**——反代生态头名不统一（authelia 发 Remote-User、oauth2-proxy 发 X-Forwarded-User），可配零猜测 — **Reversibility:** one-way — CLI flag 公开契约
- **D-19:** remote_user 值清洗 = **剥离 C0/C1 控制字符 + 截断 128 字符**（P4 D-03 标题 sanitize 同款纪律；logEvent 是 stderr 单行文本，控制字符注入伪造日志行的风险当期就存在）
- **D-20:** X-Forwarded-For **同批做**：与 auth-header **共用信任闸**（--auth-header 给定 = 「信任反代」总开关，零双轨）；XFF 取链中首个 IP；消费范围 = logEvent remote 字段与 throttle per-IP 键同换 — **Reversibility:** costly — throttle 计数键变更是安全语义变更

**子进程管理（OPS-04）**
- **D-21:** `--cwd /path`（默认继承服务端 cwd 现状）+ `--term`（默认 xterm-256color 现状）两 flag——落 cmd.Dir 与 whitelistEnv 的 TERM= 行（spawn.go:50,65 注释预留位）；--cwd 目录不存在 = stat 预检启动报错 fail-fast — **Reversibility:** one-way — CLI flag 公开契约
- **D-22:** 停止信号 = **`--stop-signal HUP|TERM|INT|KILL`（默认 HUP 保持现状）+ `--stop-timeout`（默认 0 = 不补 KILL 纯单信号）**——exitf 收口时显式 kill(pgid, stop-signal) → 等 timeout → 仍存活补 SIGKILL；「Close master 内核发 SIGHUP」免费通道保留为兼容底层，显式信号在上层；exitf + sync.Once 单一收口纪律保持，只加触发源不加分支 — **Reversibility:** one-way — CLI flag 公开契约
- **D-23:** 1001 优雅下线（P6 deferred 兑现）= **wesh 捕获 SIGTERM/SIGINT → 向全部客户端发 1001 Going Away → 子进程 stop-signal 序列 → exit**——1001 不在 CORE-05 重连触发集（P6 D-01 仅 1006），前端显示「Server shutting down」面板而非重连循环 — **Reversibility:** one-way — 关闭码是前后端公开协议契约

**降权运行（OPS-05）**
- **D-24:** `--uid`/`--gid` **数字直通，成对给出**（只给一个 = 启动报错）——避免静态二进制在极简容器（无 /etc/passwd）里的 NSS 解析差异；降权挂点 = spawn 时 SysProcAttr.Credential（fork 后 exec 前） — **Reversibility:** one-way — CLI flag 公开契约
- **D-25:** 降权后身份环境 = **按目标 uid 查 passwd 条目自动改写白名单里 HOME/USER/LOGNAME**（查不到则剔除三键让 shell 自默认）——降权直觉语义 = 连身份环境一起降；走 SEC-06 白名单通道（替换式注入纪律不变）

**自动打开浏览器（OPS-11）**
- **D-26:** `--open` 布尔 flag，打开 **operator 视角入口：--writable 时开 rw 分享链接，否则开 ro 链接**（含 token 免交互即打即用；token 通道绕过 Basic 是 P5 D-01 既定语义） — **Reversibility:** one-way — CLI flag 公开契约
- **D-27:** 平台机制 = **xdg-open（Linux）/ open（macOS）**；headless 检测（无 DISPLAY 且无 WAYLAND_DISPLAY）时 **stderr 提示后跳过不阻断启动**；Windows 不做（PROJECT Out of Scope）

### Claude's Discretion
- 配置文件加载在 parseArgs 中的装配顺序（--config 先解析出路径 → 加载 TOML → 按 D-02/D-05 合并 → fs.Visit 显式设置位判定；二次解析 vs 单 pass 的实现形态由 planner 定）
- `--stop-timeout` 的 flag 形态（DurationVar 直收 vs exitEmptyValue 同款自定义 Value）与 KILL 补发后 exitf 退出码语义（子进程被 KILL 收 -1/255 与 P6 OQ1 accept-255 同源）
- 1001 广播与慢客户端 outbox 的写序（P6 EXIT 帧先例：lifecycle 组帧一次共享只读 + 每客户端 goroutine 同步 Write 带超时；是否复用 2s 定值）
- SIGTERM/SIGINT 捕获的挂点（signal.NotifyContext vs 显式 goroutine）与 run() 返回路径的收口形态（保持 exitf 单一收口）
- base-path 前缀剥离的服务端装配形态（mux 模式串拼接 vs http.StripPrefix 中间件；headers.go 中间件先例参照）
- XFF 解析细节（多值取首个、非法值回退 TCP 对端 IP、空格清洗）与 clientIP 函数的改造形态
- --open 的实现位置（启动打印后 goroutine 调用 vs 阻塞调用；xdg-open 失败只警告）
- remote_user 在 logEvent 三要素中的字段位置与 share token 通道是否同样提取（倾向一致提取）
- UAT 场景矩阵（phase07.mjs：配置文件合并/unix socket 全链/base-path 页面+WS 升级/auth-header 记录/XFF/stop-signal 宽限/降权/1001 关停序列）

### Deferred Ideas (OUT OF SCOPE)
- **remote_user 进 slog 结构化审计事件**（attach/detach 事件携 remote_user 字段检索）— Phase 8 OPS-08；本 phase 先进 logEvent stderr 单行（D-15）
- **前端身份显示**（Welcome 帧携 remote_user，标题/面板显示「as alice」）— SEC-07 收窄时被用户裁掉的第二层价值
- **auth-header 可信来源 IP 校验**（--trusted-proxy CIDR 限定采信来源）— D-16 裸信任+警告的升级路径
- **/healthz、/metrics、per-IP 节流在 XFF 下的指标口径** — Phase 8 OPS-06/07
- **配置文件热重载（SIGHUP reload）与多配置文件** — 无需求支撑，roadmap 未列
- **自定义首页 HTML（--index）** — Phase 9 OPS-03
- **负载测试标定回填**（stop-timeout 合理默认等）— Phase 9
- **Windows 平台 --open（rundll32 url.dll）与整体 Windows 支持** — PROJECT Out of Scope（非延期，终局不做）
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OPS-01 | 监听配置：端口（0=随机并打印实际端口）/绑定地址/UNIX socket（含属主设置） | 端口 0/绑定现状已有（main.go:119-120,459,483）；UNIX socket = net.Listen("unix") 分岔 + listen 前 os.Remove（GOROOT 核实 Go 不自动 unlink——Pattern 1）+ listen 后 os.Chmod/os.Chown + validateStartup loopback 早退同款跳过（main.go:396-398 先例） |
| OPS-02 | 反代子路径挂载（base-path） | mux 模式串前缀装配 + 注册子树得免费 307 规范化（GOROOT server.go:2687,2721-2745 核实——Pattern 2）+ StripPrefix 仅包静态伺服 + 前端相对 URL 三改含 share 页 `../../` 升级前缀（Pattern 3 关键形态） |
| OPS-04 | 子进程 cwd/TERM/关闭信号可配置（信号发给进程组） | spawn.go:50,65 注释预留挂点；SignalHangup 负 pid 进程组先例（signal_linux.go:15-17）；stop-signal 序列 = kill(-pgid, sig) → sleep(timeout) → kill(-pgid, SIGKILL)（ESRCH 幂等纪律 signal_linux.go:10-11）；--cwd stat 预检 fail-fast |
| OPS-05 | 降权运行（setuid/setgid） | syscall.Credential{Uid,Gid uint32}（GOROOT exec_unix.go:124-129 逐字）+ creack/pty StartWithSize 合并式 SysProcAttr（GOMODCACHE start.go:18-25 核实兼容）+ forkExec 顺序 setsid→降权→TIOCSCTTY（exec_linux.go:385,490-510,632）+ user.LookupId 查 passwd 改 HOME/USER/LOGNAME（whitelistEnv 替换式通道 spawn.go:63-85） |
| OPS-09 | 配置文件支持，CLI 参数覆盖配置文件 | go-toml/v2 v2.4.3 严格模式 `NewDecoder(r).DisallowUnknownFields().Decode(&v)` → StrictMissingError（pkg.go.dev CITED——Pattern 4）；合并宿主 = fs.Visit 显式设置位（main.go:216-228 三先例）+ 优先级 flag > env > config > default（WESH_CREDENTIAL 先例 main.go:268-276） |
| OPS-11 | 可选启动后自动打开浏览器 | xdg-open/open + headless 检测（无 DISPLAY/WAYLAND_DISPLAY 跳过——本机实测两变量均空，skip 路径是常态部署形态）+ 分享链接 URL 含 token（main.go:502-504 启动打印形态）+ goroutine 调用失败只警告 |
| SEC-07 | 反代 auth-header 透传（D-15 收窄：attach 时 logEvent 记录 remote_user） | HTTP 层提取零 WS 资源（守卫区顺序敏感纪律）+ logEvent 三要素唯一出口（server.go:975-977）+ D-19 sanitize（web/src/lib/title.ts C0/C1+128 同款纪律的 Go 移植）+ XFF 消费点 clientIP（server.go:523-529）与 throttle per-IP 键（throttle.go:57,70）；ttyd -H 模型差异 = per-connection spawn（ttyd protocol.c:188-189 核实） |
</phase_requirements>

## Project Constraints (from CODEBUDDY.md)

| 约束 | 对本 phase 的影响 |
|------|------------------|
| 双机拓扑：Linux 开发机构建/运行 + Windows 工作站跑 Playwright（2026-08-24 修订）；Linux 侧禁装 GUI/浏览器/playwright | 协议层 UAT（phase07.mjs）与 Go 单测在 Linux 侧；base-path 页面观感可列 pw 可选场景（phase06-pw.mjs 先例），非阻塞 |
| 测试分层：① `web/uat/phaseNN.mjs` 零依赖协议脚本；② `@xterm/headless`（需 allowProposedApi）；③ jsdom + mock；④ Windows pw 实测；⑤ 平台原生行为显式豁免 | phase07.mjs 同款零依赖纪律；unix socket 场景 WS 断言经 TCP relay（本机探针实证，Pattern 7）；xdg-open 真实弹浏览器列 skipped+reason |
| 不要在本机启动 wesh 实例等待人工浏览器访问 | --open 场景 UAT 以「headless 检测正确跳过」+「xdg-open 调用参数断言（fake xdg-open 前置 PATH）」自动化，不留人工环节 |
| pnpm 而非 npm；构建命令带 `time` 前缀 | 前端 dist 重建：`time pnpm -C web build` |
| Go 单测全量 + `-race`（CI ci.yml 既定） | 新组合校验进 TestStartupMatrix/TestParseArgs 表驱动扩展；`go test -race -count=1 ./...` |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 配置文件加载/合并/严格校验 | API / Backend (cmd/wesh parseArgs) | — | 进程启动面纯服务端职责；前端零感知 |
| UNIX socket 监听/属主/权限位 | API / Backend (run() net.Listen 分岔) | OS 文件系统（权限位即认证边界） | listen/chmod/chown 是 OS 调用；访问控制委托文件系统（D-11） |
| base-path 路由前缀装配 | API / Backend (Handler mux) | — | 路由是服务端结构；go:embed 静态伺服经 StripPrefix 零改动 |
| base-path 前端 URL 构造 | Browser / Client (main.ts) | — | 相对路径解析发生在浏览器；D-14 明确拒绝服务端注入 base |
| auth-header/XFF 提取与记录 | API / Backend (HTTP 层 + logEvent) | — | 头是 HTTP 层属性；D-17 正交提取不做认证决定 |
| 子进程 cwd/TERM/降权 | API / Backend (pty spawn) | — | exec 属性仅服务端可达；spawn.go 预留挂点 |
| stop-signal 序列/进程组信号 | API / Backend (pty.Session + 收口路径) | — | OS 级信号；SignalHangup 平台构建标签先例（signal_linux/darwin） |
| SIGTERM/INT 捕获 + 1001 广播 | API / Backend (run() + lifecycle) | — | 进程信号服务端专属；广播复用 lifecycle EXIT 帧挂点 |
| 1001 前端分派面板 | Browser / Client (onclose switch) | — | showStatus 三态复用（main.ts:873-929 既有 switch 加 case） |
| --open 浏览器拉起 | API / Backend (run() 启动后) | OS 桌面环境（xdg-open/open） | 进程拉起是服务端行为；是否有效取决于 OS 会话（headless 跳过） |

## Standard Stack

**唯一新依赖 = pelletier/go-toml/v2（STACK.md 2026-08-13 定案，不用 viper）；其余全部 stdlib + 在库依赖。**

### Core（新增）
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/pelletier/go-toml/v2` | v2.4.3 (2026-07-05) [VERIFIED: proxy.golang.org @latest → `{"Version":"v2.4.3","Time":"2026-07-05T02:25:11Z"}`，go-toml 官方仓库 refs/tags/v2.4.3] | TOML 配置文件解析（D-01..D-07） | STACK.md 定案：「配置文件解析（TOML）…v2 活跃维护。不用 viper——对单文件配置过重」；`Decoder.DisallowUnknownFields()` 官方严格模式直接兑现 D-06 未知键拒绝 [CITED: pkg.go.dev/github.com/pelletier/go-toml/v2] |

### Core（既有/stdlib，直接复用）
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net` | go1.26.3 [VERIFIED: `go version`] | `net.Listen("unix", path)` UNIX socket 监听 | bind 不自动 unlink（listenStream 直接 syscall.Bind）[VERIFIED: GOROOT/src/net/sock_posix.go listenStream]；`UnixListener` 默认 unlink-on-close [VERIFIED: GOROOT/src/net/unixsock_posix.go:210-216,230] |
| Go stdlib `net/http` | go1.26.3 | base-path mux 装配 + 307 尾斜杠规范化 | `matchOrRedirect` 恒 307 + 追加斜杠重匹配 [VERIFIED: GOROOT/src/net/http/server.go:2687,2721-2745]；`http.StripPrefix` 前缀剥离 [CITED: pkg.go.dev/net/http#StripPrefix] |
| Go stdlib `flag` | go1.26.3 | 13 个新 flag 注册 + fs.Visit 显式设置位 | main.go:216-228 三先例（writePolicySet/maxClientsSet/exitEmptySet）[VERIFIED: cmd/wesh/main.go:216-228] |
| Go stdlib `os/user` | go1.26.3 | --socket-owner 名字解析 + D-25 按 uid 查 passwd 改写身份环境 | 双实现：cgo getpwuid_r / 纯 Go 解析 /etc/passwd；静态二进制自动纯 Go，osusergo tag 强制 [CITED: pkg.go.dev/os/user] |
| Go stdlib `syscall` | go1.26.3 | `SysProcAttr.Credential` 降权 + 进程组信号 | Credential 结构 [VERIFIED: GOROOT/src/syscall/exec_unix.go:124-129]；forkExec 顺序 setsid→setgid/setuid→TIOCSCTTY [VERIFIED: GOROOT/src/syscall/exec_linux.go:385,490-510,632] |
| Go stdlib `os/signal` | go1.26.3 | SIGTERM/SIGINT 捕获（D-23） | `signal.NotifyContext(parent, signals...) (ctx, stop)`——信号到达/调 stop/父 ctx 关闭三者取先取消 [CITED: pkg.go.dev/os/signal#NotifyContext] |
| `github.com/coder/websocket` | v1.8.15 [VERIFIED: go.mod:6] | 1001 广播关闭码 | `StatusGoingAway StatusCode = 1001` [VERIFIED: GOMODCACHE coder/websocket@v1.8.15/close.go:29] |
| `github.com/creack/pty` | v1.1.24 [VERIFIED: go.mod:7] | spawn 降权挂点 | StartWithSize 只补 Setsid/Setctty 不覆盖调用方 SysProcAttr [VERIFIED: GOMODCACHE creack/pty@v1.1.24/start.go:18-25] |
| Go stdlib `os/exec` | go1.26.3 | --open 拉起 xdg-open/open + cmd.Dir（--cwd） | 失败只警告（D-27）；exec.Command(...).Start() 不等待 |

### Supporting（测试基建，既有）
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Node 原生 WebSocket/fetch | v24.13.0 [VERIFIED: `node --version`] | phase07.mjs 协议层断言 | unix socket 场景经 TCP relay 转发（Pattern 7 探针实证） |
| Node `node:net` | v24.13.0 | TCP↔unix socket relay（UAT 夹具） | `net.createServer(c => c.pipe(net.createConnection(sock)).pipe(c))` [VERIFIED: 本机探针] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-toml/v2 | spf13/viper | STACK.md 定案否决：「对单文件配置过重」；viper 的隐式默认值合并语义与 D-05 显式优先级链（flag > env > config > default）哲学冲突 |
| go-toml/v2 | BurntSushi/toml | 维护活性弱于 pelletier v2 线；STACK.md 已定案 pelletier |
| StripPrefix 中间件 | mux 模式串全量前缀拼接（每路由注册时拼 base） | 两者都要改注册点；StripPrefix 只包静态伺服（embed handler 是唯一路径敏感 handler），其余 handler 路径无关——混合形态最简（Pattern 2 推荐） |
| signal.NotifyContext | 显式 signal.Notify + goroutine | NotifyContext 零自管 channel 缓冲/停止语义（stop() 注销信号行为恢复默认）[CITED: pkg.go.dev/os/signal]；与 run() 的 select 收口自然组合 |
| --open 用 xdg-open | 直接写 D-Bus/苹果事件 | 无收益；xdg-open/open 是两平台事实标准启动器 |

**Installation:**
```bash
go get github.com/pelletier/go-toml/v2@v2.4.3
go mod tidy
# 依赖引入顺序纪律（05-05 登记）：先落码（import 存在）再 go get + go mod tidy，否则 tidy 回收无引用依赖
```

**Version verification:** v2.4.3 经 proxy.golang.org @latest 与 @v/list 当日核实（2026-08-25），与 STACK.md（2026-08-13 核实）同版未漂移。

## Package Legitimacy Audit

> gsd-tools package-legitimacy 仅支持 npm/pypi/crates 三生态（golang 不在其列——本 session 实测返回 Usage 错误）。Go 包走 Go module proxy + 官方文档双通道核实。

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| github.com/pelletier/go-toml/v2 | proxy.golang.org | v2 线自 2021（首个 v2 tag 2021 年，4+ 年）；v2.4.3 发布 2026-07-05（7 周前） | pkg.go.dev 高导入量（Kubernetes/Docker 生态标准 TOML 库） | github.com/pelletier/go-toml（proxy Origin 字段核实：`"URL":"https://github.com/pelletier/go-toml","Ref":"refs/tags/v2.4.3"`） | OK | Approved |
| github.com/coder/websocket（既有） | proxy.golang.org | 多年 | 高 | github.com/coder/websocket | OK | 既有依赖，go.sum 锁定 |
| github.com/creack/pty（既有） | proxy.golang.org | 多年 | 高（1,263 导入模块，STACK.md 核实） | github.com/creack/pty | OK | 既有依赖，go.sum 锁定 |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*go-toml/v2 由 STACK.md（2026-08-13 官方 registry 核实）+ 本 session proxy.golang.org @latest 复核 + pkg.go.dev 官方文档三通道确认，非 WebSearch/训练数据发现——不触发 [ASSUMED] 标记。Go 生态无 postinstall 脚本面（module zip 纯源码，无安装期执行）。*

## Architecture Patterns

### System Architecture Diagram

```
                          ┌──────────────── 启动装配（main.go，顺序敏感）────────────────┐
                          │ parseArgs: 预扫 --config → TOML 严格加载(未知键拒) → flag 注册 │
                          │  → fs.Parse → fs.Visit 显式设置位 → 合并(flag>env>cfg>默认)   │
                          │ validateStartup: 组合矛盾矩阵(socket×port / owner单给 / uid单给│
                          │  / auth-header 暴露面警告 / unix 跳过 bind 矩阵)               │
                          └──────────────┬──────────────────────────────────────────────┘
                                         ▼
        ┌─────────────────── run() 监听分岔 ───────────────────┐
        │ --socket: os.Remove → net.Listen("unix") → Chmod → Chown │ --port: net.Listen("tcp")
        └──────────────┬──────────────────────────────────────────┘
                       ▼
   ┌──────────── Handler() mux 装配（base-path 前缀注入点）────────────┐
   │ bp+"/" → StripPrefix(bp, basicAuth?/embed)  ← 307: /wesh → /wesh/ │
   │ "POST "+bp+"/api/attach" → shareAttach peek → origin→basic→签发   │
   │ "GET "+bp+"/s/{token}/" → sharePage(改写"/"委托，前缀无关)         │
   │ bp+"/ws" → Attach 守卫区⓪Origin→①子协议→②halfOpen→③容量           │
   │           （auth-header/XFF 在 HTTP 层提取：remote_user/remote）   │
   └──────────────┬───────────────────────────────────────────────────┘
                  ▼ attach（Accept 前零 WS 资源）
   logEvent(remote[XFF], code, reason[, remote_user])  ← D-15/D-20 记录点
                  ▼
   ┌────────── pty.Start(argv, opts) ──────────┐
   │ cmd.Dir=--cwd · TERM=--term ·             │
   │ SysProcAttr.Credential{--uid,--gid} ·     │
   │ whitelistEnv 改写 HOME/USER/LOGNAME(D-25) │
   └──────────────┬────────────────────────────┘
                  ▼
   SIGTERM/INT ──► 1001 Going Away 广播（EXIT 帧先例：快照+每客户端 2s 同步写）
              ──► stop-signal 序列: kill(-pgid, HUP|TERM|INT|KILL) → [stop-timeout] → kill(-pgid, SIGKILL)
              ──► 子进程死亡 → sess.Wait 返回 → lifecycle(EXIT+1000, 注册表已空) → exitf（单一收口）
```

### Recommended Project Structure（新增/修改面）

```
cmd/wesh/
├── main.go           # config struct 扩 13 flag + 配置文件两阶段合并 + validateStartup 扩展 + run() 分岔
├── config.go         # 【新】TOML 文件加载/严格校验/权限警告（parseArgs 调用，单职责拆分）
internal/pty/
├── spawn.go          # Start 选项化（Dir/Term/Credential），whitelistEnv 加身份改写参数
├── signal_linux.go   # SignalHangup 同款 SignalGroup(sig) 泛化（stop-signal 任意信号）
├── signal_darwin.go  # 同签名平台对件
internal/server/
├── server.go         # Options 加 AuthHeader；clientIP XFF 改造；logEvent remote_user 字段；
│                     # lifecycle 旁加 Shutdown（1001 广播 + stop-signal 触发源，非 exitf 分支）
├── proxy.go          # 【新】auth-header/XFF 提取与 sanitize（headers.go/origin.go 中间件同位文件）
├── handler 装配      # Handler() 加 base-path 前缀参数（registerShareRoutes 同改）
web/src/main.ts       # 相对 URL 三改 + onclose 1001 case
internal/proto/proto.go # 1001 注释启用（占位翻正）
```

### Pattern 1: UNIX socket 监听序列（OPS-01，D-08..D-12）

**What:** --socket 独立 flag 的 listen 分岔与属主/权限落地序列。
**Why this exact sequence:** Go 的 net.Listen("unix") **不会在 bind 前 unlink 既有文件**——GOROOT `listenStream` 直接 `syscall.Bind(fd.pfd.Sysfd, lsa)`，无 unlink 调用（本 session 逐行核实）；文件存在即 EADDRINUSE。socket 文件 mode 由内核定为 `0777 & ~umask`，**Go 不做任何 chmod**（GOROOT net 包全文 grep 无 umask/chmod）——D-09 的 0660 确定性只能在 listen 后显式达成。

**Example:**
```go
// Source: GOROOT go1.26.3 src/net/sock_posix.go（listenStream 无 unlink）+
//         src/net/unixsock_posix.go:210-216,230（unlink:true 默认）+ D-08..D-12
// 序列（顺序敏感，失败即回滚）：
_ = os.Remove(sockPath)                       // D-10：残留即垃圾；不 Remove 则 bind EADDRINUSE
ln, err := net.Listen("unix", sockPath)       // 内核建文件，mode=0777&~umask（不确定）
if err != nil { /* exit 1，同 tcp 分岔 */ }
if err := os.Chmod(sockPath, socketMode); err != nil { // D-09：0660 确定性
	_ = ln.Close() // Close 自动 unlink（UnixListener 默认 unlink:true）——回滚零残留
	/* exit 1 */
}
if owner != "" {                              // D-09：user[:group]，user.Lookup 解析
	uid, gid := resolveOwner(owner)       // user.Lookup + strings.Cut(":") + LookupGroup
	if err := os.Chown(sockPath, uid, gid); err != nil {
		_ = ln.Close()
		/* exit 1 */
	}
}
// 之后与 tcp 分岔汇合：同一个 hs.Serve(ln)/ServeTLS
```

**关键事实（全部本 session 核实）：**
- `UnixListener` 以 `unlink: true` 构造——「The default behavior is to unlink the socket file only when package net created it.」 [VERIFIED: GOROOT/src/net/unixsock_posix.go:210-216 逐字 + L230 `return &UnixListener{fd: fd, path: fd.laddr.String(), unlink: true}`] → **进程退出/Close 自动删 socket 文件**，D-10 关闭半侧免费
- chmod 与 listen 之间存在 umask 窗口（文件先以 0755 类模式出现）——wesh 先完成 Chmod/Chown 再打 listening 行，窗口期内无客户端被指引，风险接受
- validateStartup 跳过（D-11）= loopback 早退同款形态：`if isLoopbackBind(cfg.bind) { return "", nil }`（main.go:396-398 逐字先例）[VERIFIED: cmd/wesh/main.go:396-398]
- 启动打印 unix 分支（D-12）：`listening on unix:///run/wesh.sock`；分享链接两行退化为提示行——main.go:498-501 的 `ln.Addr().(*net.TCPAddr)` 断言在 unix 形态为 false（ok 分支），现有代码已天然不拼 TCP 端口，planner 注意保留该防御

### Pattern 2: base-path 服务端装配（OPS-02，D-13/D-14）

**What:** mux 注册点统一注入 base 前缀；StripPrefix 仅包路径敏感的静态伺服；307 尾斜杠规范化免费获得。
**Why:** 全部 handler 中只有 embed 静态伺服（web.Handler，按 `r.URL.Path` 取文件）是路径敏感的；Attach/attachHandler/sharePage/issueTicketJSON 全部路径无关（sharePage 自改写 `r.URL.Path = "/"` 后委托 [VERIFIED: internal/server/sharetoken.go:87-96]）。

**Example:**
```go
// Source: GOROOT server.go:2687,2721-2745（307 机制）+ pkg.go.dev/net/http#StripPrefix + D-13/D-14
// bp = ""（未配置）或 "/wesh"（D-13 校验后：/ 开头、无尾斜杠、无 ..、无重复斜杠）
func (s *Server) Handler(bp string) http.Handler {
	mux := http.NewServeMux()
	wh, _ := web.Handler()
	// 静态伺服：StripPrefix 把 /wesh/... → /...；bp="" 时 StripPrefix 零剥离子行为不变
	root := wh
	if len(s.credentials) > 0 {
		root = basicAuth(wh, s.credentials, s.throttle)
	}
	if bp != "" {
		mux.Handle(bp+"/", http.StripPrefix(bp, root))
	} else {
		mux.Handle("/", root)
	}
	// 前缀注册示例（路径无关 handler 原样）：
	mux.Handle("POST "+bp+"/api/attach", attachChain)
	mux.HandleFunc(bp+"/api/attach", methodNotAllowed405)
	mux.Handle("GET "+bp+"/s/{token}/", s.sharePage(page, root)) // sharePage 改写"/"，前缀无关
	mux.HandleFunc(bp+"/s/{token}/", shareMethodNotAllowed405)
	mux.HandleFunc(bp+"/ws", s.Attach)
	return securityHeaders(mux, s.tlsOn)
}
```

**关键事实：**
- 注册 `bp+"/"` 子树后，`GET /wesh`（无尾斜杠）经 matchOrRedirect 追加 `/` 重匹配命中 → **307 保方法重定向** [VERIFIED: GOROOT/src/net/http/server.go:2687 `RedirectHandler(u.String(), StatusTemporaryRedirect)` + 2721-2745 matchOrRedirect 逐行]——D-14「尾斜杠 307 规范化」零自写代码；share 路由 `/wesh/s/{token}/` 同款（sharetoken.go:109-113 既有注释登记的同一机制）
- `http.StripPrefix` 语义：「removing the given prefix from the request URL's Path (and RawPath if set)… a request for a path that doesn't begin with prefix by replying with an HTTP 404」 [CITED: pkg.go.dev/net/http#StripPrefix]——mux 已保证前缀匹配，404 分支结构性不可达；StripPrefix 浅拷贝 request 不原地改（sharePage 的 `r.URL.Path = "/"` 改写不受影响）
- Origin 校验零影响（D-14 明示）：Origin 头无 path 分量（origin.go:64-82 只看 scheme://host）[VERIFIED: internal/server/origin.go:64-82]
- CSP 零影响：`connect-src 'self'` 按 origin 不按 path（headers.go:33）[VERIFIED: internal/server/headers.go:33]
- **推荐文档化 nginx 配方**（PITFALLS 部署矩阵素材）：`location = /wesh { return 308 /wesh/; }` + `location /wesh/ { proxy_pass http://127.0.0.1:7681; proxy_http_version 1.1; proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection $connection_upgrade; proxy_read_timeout 3600s; }`——nginx `location /wesh/` 不匹配裸 `/wesh`，精确重定向块必需 [ASSUMED: nginx location 匹配语义为运维常识，README 交付前建议在 pw 层或人工复核]
- 对照面：ttyd `-b` 用 **302**（HTTP_STATUS_FOUND）[VERIFIED: ttyd/src/http.c:133-142 `redirects /base-path to /base-path/` + `lws_add_http_header_status(wsi, HTTP_STATUS_FOUND, …)`]——GET 下等价，wesh 的 307 更严格（PITFALLS 表「301 丢 WS 升级」教训的正式规避）

### Pattern 3: 前端相对 URL 改造（OPS-02 前端半侧，D-14）——**本 research 最关键发现**

**What:** 三处硬编码 URL 改相对构造；share 页挂载点检测 + `../../` 升级前缀。
**Why the up-level prefix:** 页面挂载点有两类——根挂载 `/`（或 `/wesh/`）与分享挂载 `/s/{token}/`（或 `/wesh/s/{token}/`）。sharePage 服务端改写路径不影响浏览器地址栏——浏览器仍在 `/s/{token}/` 下解析相对 URL：朴素 `fetch('api/attach')` 解析为 `/s/{token}/api/attach`（服务端无此路由 → 落 `/` 子树 → 凭据模式 basicAuth 401 → 前端误显 Invalid share link——**功能回归**）。

**Example:**
```typescript
// Source: D-14 + main.ts:500,509-510,601 现状改造点（本 session 逐行核实）
// share 正则不锚 ^（D-14 第三改的真实作用：兼作挂载点检测）
const shareMatch = location.pathname.match(/\/s\/([^/]+)\/$/);
const shareToken = shareMatch ? shareMatch[1] : undefined;
// 升级前缀：share 页深两段（/s/{token}/ → 站根需上两级）；根挂载为空串
const up = shareMatch ? '../../' : '';
// ① fetch（main.ts:509-510 现状 '/api/attach'）：
const resp = await fetch(up + 'api/attach', { method: 'POST', /* … */ });
// ② WS（main.ts:601 现状 (protocol)+location.host+'/ws'）：
const wsUrl = new URL(up + 'ws', location.href);
wsUrl.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
ws = new WebSocket(wsUrl, [SUBPROTOCOL]);
```

**验证矩阵（相对解析，全部按 RFC 3986 语义推演）：**

| 页面地址 | up | fetch 解析 | WS 解析 |
|---|---|---|---|
| `/` | `''` | `/api/attach` ✓ | `/ws` ✓ |
| `/wesh/` | `''` | `/wesh/api/attach` ✓ | `/wesh/ws` ✓ |
| `/s/{token}/` | `'../../'` | `/api/attach` ✓ | `/ws` ✓ |
| `/wesh/s/{token}/` | `'../../'` | `/wesh/api/attach` ✓ | `/wesh/ws` ✓ |
| `/wesh`（裸） | — | 不可达：mux 307 → `/wesh/` 先行 | 同左 |

**Anti-Patterns to Avoid:**
- **只去前导斜杠不做升级前缀：** share 链接全灭（上述 401 回归链）——UAT 必须含「share 页进入 + base-path」交叉场景
- **服务端注入 `<base href>`：** D-14 已否决（破坏 go:embed 单文件零处理现状、引入 CSP 复杂度）
- **`new URL('ws', location.href)` 后忘换 protocol：** URL 构造器继承 http(s) scheme，WebSocket 构造只收 ws/wss——必须显式 `wsUrl.protocol = 'wss:'/'ws:'`

### Pattern 4: 配置文件两阶段合并（OPS-09，D-01..D-07）

**What:** 预扫 --config → TOML 严格加载铺底 → flag 注册解析 → fs.Visit 显式位合并。
**Why strict mode:** D-06 未知键拒绝 = go-toml 官方严格模式直接兑现；`DisallowUnknownFields` 同时把 D-04 排除项（no-auth/insecure-http/version/help/config）自然变成「未知键」拒绝——结构体不含这些字段即可。

**Example:**
```go
// Source: pkg.go.dev/github.com/pelletier/go-toml/v2（严格模式 API）+ D-01..D-07
// fileConfig：指针标量区分「键缺席」与「零值」；列表 nil 同理
type fileConfig struct {
	Port         *int     `toml:"port"`
	Bind         *string  `toml:"bind"`
	Writable     *bool    `toml:"writable"`
	PingInterval *string  `toml:"ping-interval"` // duration 串，time.ParseDuration 复用
	WritePolicy  *string  `toml:"write-policy"`
	MaxClients   *int     `toml:"max-clients"`
	Once         *bool    `toml:"once"`
	ExitWhenEmpty *string `toml:"exit-when-empty"` // 复用 exitEmptyValue.Set 三形态语义
	Credential   []string `toml:"credential"`
	Origin       []string `toml:"origin"`
	ClientOption []string `toml:"client-option"`
	TLSCert      *string  `toml:"tls-cert"`
	TLSKey       *string  `toml:"tls-key"`
	Osc52        *bool    `toml:"osc52"`
	Socket       *string  `toml:"socket"`
	SocketMode   *string  `toml:"socket-mode"`   // 八进制串，strconv.ParseUint(s, 8, 32)
	SocketOwner  *string  `toml:"socket-owner"`
	BasePath     *string  `toml:"base-path"`
	AuthHeader   *string  `toml:"auth-header"`
	Cwd          *string  `toml:"cwd"`
	Term         *string  `toml:"term"`
	StopSignal   *string  `toml:"stop-signal"`
	StopTimeout  *string  `toml:"stop-timeout"`
	Uid          *int     `toml:"uid"`
	Gid          *int     `toml:"gid"`
	Open         *bool    `toml:"open"`
	Command      []string `toml:"command"` // D-04：exec 数组；CLI `--` argv 非空则覆盖
	// D-04 排除项不在结构体：no-auth/insecure-http/version/help/config
	// → DisallowUnknownFields 以「未知键」拒绝（逃生门必须显式说出口）
}

func loadFileConfig(path string) (*fileConfig, error) {
	f, err := os.Open(path) // D-06：不存在 exit 2
	if err != nil { return nil, err }
	defer f.Close()
	var fc fileConfig
	err = toml.NewDecoder(f).DisallowUnknownFields().Decode(&fc)
	// D-06 严格模式：未知键 → *toml.StrictMissingError（含行列上下文）
	if err != nil { return nil, err }
	return &fc, nil
}
```

**严格模式 API（官方文档逐字）：**「DisallowUnknownFields causes the Decoder to return an error when the destination is a struct and the input contains a key that does not match a non-ignored field. In that case, the Decoder returns a StrictMissingError」 [CITED: pkg.go.dev/github.com/pelletier/go-toml/v2]；错误类型 `type StrictMissingError struct { Errors []DecodeError }` 带 `String()` 多行可读输出。**注意：该库没有 SetStrict 方法**——严格模式唯一 API 是 DisallowUnknownFields。

**合并算法（D-02/D-05 落到 fs.Visit）：**
```
显式位 = fs.Visit 收集（writePolicySet/maxClientsSet/exitEmptySet 三先例 main.go:216-228）
对每个配置项 X：
  if flag X 被显式设置 → CLI 值（最高优先）
  else if env X 非空（仅 WESH_CREDENTIAL 现状） → env 值
  else if fileConfig.X 非 nil → 配置值（credential 等敏感值校验错误走记录式，
                                   credErr/clientOptErr 同款 main.go:143-152,174-192）
  else → 内置默认
列表项（credential/origin/client-option）：CLI 回调非空 = 显式给出 → 整个列表替换（D-02）；
  CLI 未给且配置有键 → 配置列表经各自 parse 期校验（ParseCredential/NormalizeOrigin/
  clientOption 白名单——同一校验函数复用，零双写）
argv：fs.Args() 非空 → CLI argv；空且 fileConfig.Command 非 nil → 配置 command；皆空 → missing command（D-04）
```

**D-07 权限检查：** `os.Stat(path)` → `info.Mode().Perm()`；含 credential 键（fc.Credential 非 nil）且 perm 非 0600/0400 → `wesh: warning: config file <path> contains credentials and is readable by others (mode XXXX); recommend chmod 600`（stderr 放行不阻断；警告串不含凭据值——SEC-01 启动面红线延伸）。

### Pattern 5: auth-header / XFF 提取与记录（SEC-07，D-15..D-20）

**What:** HTTP 层提取（Accept 前零 WS 资源）→ sanitize → logEvent 扩字段；XFF 共用信任闸换 clientIP。
**Example:**
```go
// Source: D-15..D-20 + server.go:519-529,585-587,975-977 现状消费点（本 session 逐行核实）
// sanitizeRemoteUser（D-19：C0/C1 剥离 + 128 截断——title.ts 同款纪律的 Go 移植）
func sanitizeRemoteUser(s string) string {
	r := make([]rune, 0, len(s))
	for _, ch := range s {
		if ch <= 0x1f || ch == 0x7f || (ch >= 0x80 && ch <= 0x9f) { continue }
		r = append(r, ch)
		if len(r) >= 128 { break }
	}
	return string(r)
}

// clientIP XFF 改造（D-20：--auth-header 给定 = 信任反代总开关；取链首 IP）
func (s *Server) clientIP(r *http.Request) string {
	if s.authHeader != "" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")          // 链首 = 最原始客户端
			if ip := strings.TrimSpace(first); ip != "" { // 空格清洗
				return ip
			}
		}
	} // 非法/缺席 → 回退 TCP 对端（现状行为）
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil { return r.RemoteAddr }
	return host
}
```

**关键事实：**
- logEvent 现状三要素单行：`fmt.Fprintf(os.Stderr, "wesh: close remote=%s code=%d reason=%s\n", remote, code, reason)` [VERIFIED: internal/server/server.go:975-977 逐字]——remote_user 作为第四字段追加（如 ` remote_user=alice`，缺席不出键）；**红线保持：remote_user 不得为 token/ticket/凭据**（D-03 延伸：share token 通道同样提取头值记录，但 token 本身永不入参）
- XFF 消费范围（D-20 锁定）：logEvent remote 字段（Attach 入口 `remote := r.RemoteAddr` → 改走统一取值函数）与 throttle per-IP 键（`s.throttle.allow(ip, now)`/`recordFail(ip, now)` throttle.go:57,70 + checkTicket 参数 server.go:876-899）[VERIFIED: 行号本 session 核实]
- ttyd 模型差异对照（D-15 README 素材）：ttyd `-H` 把头值拷进 `pss->user` 供 **per-connection spawn** 注入子进程 env [VERIFIED: ttyd/src/protocol.c:188-189 `return lws_hdr_custom_copy(wsi, pss->user, sizeof(pss->user), server->auth_header, strlen(server->auth_header)) > 0;`]——wesh GoTTY 共享进程模型（spawn 时无 HTTP 请求、多客户端共享一个 shell）下该语义结构性不成立，故只要审计归因
- D-16 启动警告挂 validateStartup：`--auth-header` 非空 && bind 非 loopback && 无凭据 → warn（warn 串自含 `wesh: warning:` 前缀先例 main.go:403,409）[VERIFIED: cmd/wesh/main.go:403,409]

### Pattern 6: 子进程选项化 + stop-signal 序列 + 降权（OPS-04/05，D-21..D-25）

**What:** pty.Start 选项化承载四新参；进程组信号泛化；Credential 降权 + 身份环境改写。
**Example:**
```go
// Source: spawn.go:40-85 现状 + GOROOT exec_unix.go:124-129 + GOMODCACHE start.go:18-25
// pty.Start 选项化（调用方 main.go:452 现状 pty.Start(argv)）
type StartOptions struct {
	Dir  string            // --cwd（空 = 继承现状 spawn.go:50 注释预留位）
	Term string            // --term（空 = "xterm-256color" 现状 spawn.go:65）
	Uid  int               // --uid（-1 = 不降权）
	Gid  int               // --gid
}
func Start(argv []string, opts StartOptions) (*Session, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = whitelistEnv(opts.Term, opts.Uid) // D-25：uid>=0 时 user.LookupId 改写 HOME/USER/LOGNAME
	cmd.Dir = opts.Dir                          // 空串 = 继承（exec.Cmd 零值语义）
	if opts.Uid >= 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{Uid: uint32(opts.Uid), Gid: uint32(opts.Gid)},
		}
	}
	// creack/pty StartWithSize 只补 Setsid/Setctty 两字段——Credential 保留
	// [VERIFIED: GOMODCACHE creack/pty@v1.1.24/start.go:18-25 逐行]
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: SpawnRows, Cols: SpawnCols})
	/* … */
}
// Credential 结构逐字 [VERIFIED: GOROOT/src/syscall/exec_unix.go:124-129]：
//   type Credential struct {
//   	Uid         uint32   // User ID.
//   	Gid         uint32   // Group ID.
//   	Groups      []uint32 // Supplementary group IDs.
//   	NoSetGroups bool     // If true, don't set supplementary groups
//   }
// forkExec 子进程顺序：setsid(L385) → setgroups/setgid/setuid(L490-510) → TIOCSCTTY(L632)
// [VERIFIED: GOROOT/src/syscall/exec_linux.go 行号本 session 核实]——降权后设 ctty 走
// 已继承的 tty fd，无文件打开权限问题（OpenSSH 同款降权 pty 形态）
```

**stop-signal 序列（D-22）：**
```go
// SignalHangup 泛化为 SignalGroup（signal_linux.go:15-17 逐字先例，平台对件纪律）：
//   _ = syscall.Kill(-s.Cmd.Process.Pid, sig)  // 负 pid = 进程组；setsid 使 pgid == pid
// D-22 序列（收口路径触发源——exit_when_empty 立即/宽限到期 + 1001 关停）：
sess.SignalGroup(stopSignal)        // 显式信号（默认 HUP = SignalHangup 现状语义）
if stopTimeout > 0 {
	time.Sleep(stopTimeout)          // 或 timer select（planner discretion）
	sess.SignalGroup(syscall.SIGKILL) // ESRCH 幂等——已死进程组重复发送无害
	                                 // [VERIFIED: signal_linux.go:10-11 注释既定纪律]
}
// 「Close master 内核发 SIGHUP」免费通道保留（Drain→Close 既有底层），显式信号在上层
```

**信号名→常量映射（flag 值 HUP|TERM|INT|KILL 枚举校验，parse 期）：** 平台文件集中（signal_linux.go/signal_darwin.go 先例）；signalName 大写名映射已有先例（server.go:997-1027 的 switch 形态——但那是 signal→name，此处需 name→signal 反向表，避免复用错方向）。

### Pattern 7: 1001 优雅下线 + SIGTERM/INT 捕获（D-23）

**What:** signal.NotifyContext 捕获 → 1001 广播（EXIT 帧先例形态）→ stop-signal 序列 → 既有 lifecycle 收口。
**Example:**
```go
// Source: pkg.go.dev/os/signal#NotifyContext + server.go:1090-1124（EXIT 帧广播先例逐行核实）
// run() 内（listen 成功、server 装配后）：
sigCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stopSignals()
go func() {
	<-sigCtx.Done()
	srv.Shutdown(stopSignal, stopTimeout) // 1001 广播 + 子进程信号序列——不调 exitf！
}()
// http.ErrServerClosed / Serve 返回后 run() 现状错误路径保持；进程终结由
// lifecycle（子进程死亡 → sess.Wait 返回 → terminate）收口——exitf + sync.Once 单一收口
```

```go
// Server.Shutdown（新增——触发源非 exitf 分支，P1 硬约束）：
func (s *Server) Shutdown(sig syscall.Signal, timeout time.Duration) {
	s.hubMu.Lock()
	s.exiting = true                     // 空触发抑制门复用（server.go:1096 先例）
	clients := snapshot(s.registry)      // hubMu 下快照（lifecycle:1097-1101 先例）
	s.hubMu.Unlock()
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 1001 直接 Close——无 EXIT 帧前置（进程未退出，终结语义由关闭码承载）；
			// Close 内建 5s+5s 上界（close.go:87-89，P6 注释核实）
			_ = c.conn.Close(websocket.StatusGoingAway, "server_shutting_down")
		}()
	}
	wg.Wait()
	s.sess.SignalGroup(sig)              // D-22 序列（Pattern 6）
	if timeout > 0 { time.Sleep(timeout); s.sess.SignalGroup(syscall.SIGKILL) }
	// 返回后：子进程死亡 → lifecycle 的 EXIT+1000 广播在空注册表上零循环 → terminate(code)
}
```

**关键事实：**
- `StatusGoingAway StatusCode = 1001` [VERIFIED: GOMODCACHE coder/websocket@v1.8.15/close.go:29 逐字]
- NotifyContext 语义逐字：「marked done … when one of the listed signals arrives, when the returned stop function is called, or when the parent context's Done channel is closed, whichever happens first」 [CITED: pkg.go.dev/os/signal#NotifyContext]
- 前端 onclose 1001 分派（main.ts:873-929 switch 加 case）：`showStatus('Server shutting down', …)`——**1001 不得进重连触发集**（D-23/P6 D-01：仅 1006，main.ts:914-918 `case 1006: startReconnect();` 逐字现状）；`reconnecting && ev.code === 1006` 分支（main.ts:862-865）对 1001 自然落到 stopReconnect + 面板分派，零冲突
- proto.go:13 注释启用：「1001 优雅下线 Phase 7 启用，本期占位不实现」 [VERIFIED: internal/proto/proto.go:13 逐字] → 翻正为启用态，前后端注释手工对齐纪律
- **1001 广播与 EXIT 广播的竞态**：Shutdown 置 exiting=true 后子进程才死亡，lifecycle 快照在 Shutdown 之后 → 注册表已空 → 零重复关闭；若子进程在 1001 广播完成前自然死亡，lifecycle 的 EXIT+1000 与 Shutdown 的 1001 可能先后到达同端——coder/websocket Close 幂等（重复 Close 返回错误静默），前端以先到的关闭码分派，两语义都正确（进程死 vs 服务关停），风险接受

### Pattern 8: --open 浏览器拉起（OPS-11，D-26/D-27）

```go
// Source: D-26/D-27 + main.go:502-504（分享链接打印形态本 session 核实）
// 启动打印完成后（ goroutine 调用，失败只警告）：
func openBrowser(cfg config, shareURL string) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" && runtime.GOOS == "linux" {
		fmt.Fprintln(os.Stderr, "wesh: --open: no display detected (headless), skipping browser launch")
		return
	}
	tool := "xdg-open"
	if runtime.GOOS == "darwin" { tool = "open" }
	if err := exec.Command(tool, shareURL).Start(); err != nil {
		fmt.Fprintf(os.Stderr, "wesh: --open: failed to launch browser: %v\n", err) // 只警告
	}
}
// URL = 分享链接（含 token 免交互）：--writable → shareRW 链接，否则 shareRO（D-26）
// base-path 配置时链接路径含前缀（D-14：分享链接打印含 base-path——main.go:502-504
// 拼串点统一注入，打印与 --open 消费同一 URL 单一事实源）
// 本机实测：DISPLAY 与 WAYLAND_DISPLAY 均空（headless 服务器常态）——skip 路径是
// 部署常态，必须不阻断启动（D-27）
```

### Anti-Patterns to Avoid
- **配置文件用 viper：** STACK.md 定案否决（过重 + 隐式合并语义与 D-05 显式链冲突）
- **listen 前不 Remove unix socket：** Go 不自动 unlink（Pattern 1 GOROOT 实证），EADDRINUSE 必现
- **socket 权限靠 umask 控制：** 进程 umask 不可控（systemd UMask= 与交互 shell 不同），0660 必须显式 Chmod
- **前端只去前导斜杠：** share 页回归（Pattern 3 401 链）
- **StripPrefix 包整个 mux：** 会把 `/s/{token}` PathValue 模式语义与 405 fallback 形态搅乱；只包静态伺服
- **auth-header 值参与认证决定：** D-17 否决——「头存在跳过 Basic」与裸信任叠加 = 伪造头绕过认证
- **XFF 取链尾或多值拼接：** 链尾是最近代理（恒为反代自己，无信息）；取链首才接近真实客户端（D-20 锁定）
- **stop-signal 发单 pid 不发进程组：** shell 的孩子（vim 等）收不到信号成孤儿——负 pid 进程组（SignalHangup 先例）
- **1001 走 outbox.trySend 异步入队：** P6 EXIT 帧同款写序竞态（关闭帧超车）——Shutdown 内同步 Close 即可（Close 自带关闭帧，无需先写数据帧）
- **降权用名字解析 uid/gid：** D-24 否决——极简容器无 /etc/passwd 时 NSS 解析差异；数字直通
- **--cwd 不预检：** spawn 后才发现 ENOENT = 资源已占用且错误面到客户端；stat 预检 fail-fast（D-21）

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TOML 解析/严格模式 | 手写 INI 式解析或 encoding/json 凑合 | pelletier/go-toml/v2 `DisallowUnknownFields` | TOML 规范边界（多行串/数组/转义）手写必漏；官方严格模式含行列错误定位 [CITED: pkg.go.dev] |
| 尾斜杠规范化 | 自写 redirect 中间件 | mux 注册子树模式，307 免费 | GOROOT matchOrRedirect 内建 [VERIFIED: server.go:2721-2745] |
| base-path 前缀剥离 | 手写 strings.TrimPrefix + 路径清理 | http.StripPrefix | RawPath 同步处理 + 转义字符精确匹配 404 [CITED: pkg.go.dev/net/http] |
| 信号捕获 | signal.Notify + 自管 channel | signal.NotifyContext | stop() 注销恢复默认行为 + ctx 组合 [CITED: pkg.go.dev/os/signal] |
| 降权 | 自 fork + setuid  syscall 序列 | SysProcAttr.Credential | forkExec 顺序正确性（setgroups/gid/uid/ctty）内核级细节 [VERIFIED: GOROOT exec_linux.go] |
| uid→身份查询 | 手解析 /etc/passwd | os/user.LookupId | cgo/纯 Go 双实现自动选择 [CITED: pkg.go.dev/os/user] |
| 关闭码 1001 | 手写 StatusCode(1001) | websocket.StatusGoingAway | 库常量 [VERIFIED: close.go:29] |
| UAT unix socket WS | 手写 WS 帧编解码 over unix | TCP relay + 原生 WebSocket | 探针实证 relay 15 行可用；手写帧编解码 ~80 行高风险 |

**Key insight:** 本 phase 的「新功能」80% 是**既有机制的新触发源/新参数面**——std/mux/pty 三层全部预留了挂点，计划期的风险不在「能不能做」而在「顺序与组合校验是否与前序纪律逐字一致」。

## Common Pitfalls

### Pitfall 1: 以为 Go 会自动清理 unix socket 残留文件
**What goes wrong:** systemd Restart= 或崩溃后重启，`net.Listen("unix", path)` 返回 `bind: address already in use`。
**Why it happens:** Go 的 listenStream 直接 `syscall.Bind`，**无 bind 前 unlink**（GOROOT 逐行核实；WebFetch 二手摘要曾错误声称「Go bind 前会 unlink」——以 GOROOT 源码为准）。
**How to avoid:** D-10 listen 前 `os.Remove`（Pattern 1 序列首位）。
**Warning signs:** 重启后 EADDRINUSE；/run 下残留 .sock 文件。

### Pitfall 2: socket 文件权限依赖 umask，systemd 与 shell 部署行为漂移
**What goes wrong:** 交互 shell（umask 022）下 socket 是 0755，systemd UMask=0077 下是 0700——D-09 的 0660 契约两种环境都不成立。
**Why it happens:** 内核定 `0777 & ~umask`，Go 不干预。
**How to avoid:** listen 后显式 `os.Chmod(path, 0660)` + `os.Chown`；chown 需 root/CAP_CHOWN——非 root 运行 + --socket-owner 时 Chown 失败即 exit 1（错误文案不含敏感值）。
**Warning signs:** `stat /run/wesh.sock` 权限与 --socket-mode 不符。

### Pitfall 3: 前端相对路径改造打断 share 链接（本 phase 最高回归风险）
**What goes wrong:** 凭据模式下 share 页 `fetch('api/attach')` → POST `/s/{token}/api/attach` → 无路由落 basicAuth 401 → 前端 Invalid share link 面板——全部分享链接失效。
**Why it happens:** 浏览器相对解析以地址栏路径为基（sharePage 服务端改写路径不影响浏览器）。
**How to avoid:** Pattern 3 的 up-level 前缀（`../../`）；UAT 交叉矩阵必含「share token × base-path」。
**Warning signs:** jsdom/协议 UAT 中 share 场景 401；浏览器 devtools 里 fetch 打到 /s/ 下。

### Pitfall 4: 307 规范化「单侧定义」——只注册 `/wesh/` 忘了 `/wesh/s/{token}/` 家族
**What goes wrong:** `/wesh/s/TOKEN`（无尾斜杠）404 而非 307 到 `/wesh/s/TOKEN/`。
**Why it happens:** matchOrRedirect 只对**已注册子树**追加斜杠重匹配；share 路由若只注册 `"GET "+bp+"/s/{token}/"` 单条，裸路径无匹配。
**How to avoid:** 四条注册（GET share + 405 fallback + api/attach POST + 405 fallback）全部带前缀；sharetoken.go:107-113 既有注释登记的 mux 通配语义三坑同纪律。
**Warning signs:** curl 裸路径返回 404 而非 307。

### Pitfall 5: 配置文件敏感值经错误通道泄露（SEC-01 启动面红线）
**What goes wrong:** TOML 解析错误/校验错误的文案回显 credential 行内容（go-toml 的 DecodeError 默认含行列上下文——StrictMissingError.String() 会印出未知键所在行）。
**Why it happens:** flag 包 `invalid value %q` 包装回显的同类问题（credErr/clientOptErr 记录式上报的存在理由）。
**How to avoid:** 配置加载错误统一为「类别 + 键名 + 行号」三要素，禁含值；credential 值校验错误走记录式（D-06 与 credErr 同纪律）；StrictMissingError 只含未知**键名**（值在文档行内可能出现——错误输出前过一道值剥离或只报键名清单）。
**Warning signs:** `wesh --config bad.toml` 的错误输出里出现密码明文。

### Pitfall 6: 降权后子进程 HOME 指向原用户家目录
**What goes wrong:** root 启动 `--uid 65534` 后 shell 的 HOME=/root——nobody 对 /root 无权限，shell 初始化报错/历史写失败。
**Why it happens:** whitelistEnv 按名继承服务端 env（spawn.go:71-75），HOME/USER/LOGNAME 原样带过去。
**How to avoid:** D-25 按目标 uid `user.LookupId` 改写三键；查不到（极简容器无 /etc/passwd）剔除三键让 shell 自默认。
**Warning signs:** 降权后 `echo $HOME` 是原用户家目录；shell 启动报 permission denied。

### Pitfall 7: XFF 信任闸与认证正交性失守
**What goes wrong:** 「头存在跳过 Basic」式捷径——直连攻击者自设 `X-Remote-User: root` 即绕过认证。
**Why it happens:** auth-header 语义被误读为认证机制；D-17 明确否决。
**How to avoid:** auth-header **只做记录**（logEvent remote_user 字段）；认证体系零改动；D-16 启动警告提示部署面（非 loopback 无凭据时 stderr 醒目）。
**Warning signs:** 代码里出现 authHeader != "" 与 basicAuth 旁路的条件组合。

### Pitfall 8: stop-timeout 的 KILL 补发打断 exitf 收口时序
**What goes wrong:** stop-signal 序列的 sleep 期间 lifecycle 已从子进程死亡路径收口（exitf 触发进程退出），sleep 后的 KILL 永不到达或到达空 pgid。
**Why it happens:** 序列与 lifecycle 并发，无协调。
**How to avoid:** KILL 补发 ESRCH 幂等静默（signal_linux.go:10-11 既定纪律）——序列无需感知 lifecycle 状态；sleep 用可中断 timer（select sigCtx.Done）让进程退出不被 sleep 拖延（planner discretion 细化）。
**Warning signs:** 关停耗时恒为 stop-timeout 全量（进程早死也等满）。

## Code Examples

Verified patterns from official sources:

### TOML 严格加载（go-toml/v2）
```go
// Source: pkg.go.dev/github.com/pelletier/go-toml/v2（官方文档示例形态）
err := toml.NewDecoder(f).DisallowUnknownFields().Decode(&fc)
if err != nil {
	var strictErr *toml.StrictMissingError
	if errors.As(err, &strictErr) {
		// strictErr.String() 带行列上下文——输出前注意值剥离（Pitfall 5）
	}
	return err
}
```

### Go 1.22+ mux 307（GOROOT 逐字证据）
```go
// Source: GOROOT go1.26.3 src/net/http/server.go:2687
return RedirectHandler(u.String(), StatusTemporaryRedirect), n.pattern.String(), nil, nil
// matchOrRedirect（2721-2745）：无精确匹配 && u != nil && 无尾斜杠 && path != "" 
//   → path += "/" 重匹配 → exactMatch 命中 → &url.URL{Path: cleanPath(u.Path)+"/", RawQuery: u.RawQuery}
```

### 降权 spawn（Credential 与 creack/pty 兼容链）
```go
// Source: GOMODCACHE creack/pty@v1.1.24/start.go:18-25（逐字）
func StartWithSize(cmd *exec.Cmd, ws *Winsize) (*os.File, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	return StartWithAttrs(cmd, ws, cmd.SysProcAttr)
}
// → 调用方预设 Credential 零冲突（Pattern 6 示例）
```

### UAT TCP↔unix relay（本机探针实证）
```javascript
// Source: 本 session 本机 Node v24.13.0 探针（探针输出 "relay probe result: ok:/ws"）
const relay = net.createServer((c) => {
  const u = net.createConnection(SOCK);
  c.pipe(u).pipe(c);
});
relay.listen(0, '127.0.0.1', () => {
  const port = relay.address().port;
  // 原生 WebSocket/fetch 连 127.0.0.1:port —— 既有 phase06.mjs 断言机制零改动复用
});
```

### signal.NotifyContext（官方文档形态）
```go
// Source: pkg.go.dev/os/signal#NotifyContext
sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stop() // stop 注销信号行为恢复默认 + 释放资源（文档明示尽快调用）
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 尾斜杠 301/302 重定向（ttyd -b 用 302） | Go 1.22+ mux 内建 307 保方法 | go1.22（2024-02） | POST/WS 升级方法保持；wesh 注册子树即免费获得 |
| 配置文件 viper 全家桶 | go-toml/v2 单库严格模式 | STACK.md 2026-08-13 定案 | 零隐式合并语义，D-05 显式优先级链 |
| setuid 降权手写 fork+syscall 序列 | SysProcAttr.Credential 声明式 | Go 1.x 起 | forkExec 内核级顺序正确性由 stdlib 保证 |
| WS 关闭码数字字面量 | 库常量 StatusGoingAway | coder/websocket 全版本 | 前后端对齐纪律（proto.go ↔ main.ts 注释互指） |

**Deprecated/outdated:**
- ttyd `-H` per-connection env 注入模型：wesh GoTTY 共享进程模型下结构性不成立（D-15 收窄的根因；README 需明示模型差异）
- gorilla/websocket：STACK.md 已否决（本 phase 无新增触点，不重复论证）

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | nginx `location /wesh/` 不匹配裸 `/wesh` 请求，README 配方需 `location = /wesh { return 308 /wesh/; }` 精确块 | Pattern 2 | README 配方少一块 → 用户裸访问 /wesh 得 nginx 404 而非重定向；文档级风险，pw/人工复核可纠 |
| A2 | remote_user 作为 logEvent 第四字段追加（缺席不出键）不破既有 logEvent 消费方 | Pattern 5 | logEvent 消费方只有测试断言与人工阅读（无机器解析）；若 UAT 有逐字行断言需同步扩展 |
| A3 | share token 通道同样提取 remote_user 记录（CONTEXT discretion「倾向一致提取」按推荐落地） | Pattern 5 | 与最终裁决不一致时只需收窄提取点，行为差异仅在审计完整性 |
| A4 | --open 与 --socket 组合时无 TCP URL 可开——按配置矛盾 fail-fast 处理（CONTEXT 未显式覆盖该组合） | Open Questions 1 | 若裁决为「跳过+警告」则校验矩阵行语义不同，一行改动 |
| A5 | socket chmod 与 listen 之间的 umask 窗口风险可接受（Chmod 在 listening 打印前完成，窗口内无客户端被指引） | Pattern 1 | 极端竞争下窗口期连接获较宽权限——本地同机攻击者模型内，风险极低 |

## Open Questions

1. **--open 与 --socket 组合的语义？**
   - What we know: unix socket 形态下无 host:port 可拼（D-12 分享链接退化为提示行）；--open 需要 http(s) URL
   - What's unclear: 配置矛盾 fail-fast（--socket×--open 进 validateStartup 拒绝）还是 headless-skip 同款「跳过+警告」
   - Recommendation: fail-fast 组合校验（与 --socket-owner 单给、--uid 单给同档——「显式哲学」一贯性：给了无法兑现的 flag 组合 = 配置错误）；planner 落 validateStartup 一行

2. **--stop-timeout 的 KILL 补发后 wesh 进程退出码？**
   - What we know: 子进程被 SIGKILL 收 → ExitCode -1 → exitf(-1) → Unix 进程退出状态 255（P6 OQ1 accept-255 同语义，已裁决形态）
   - What's unclear: systemd 视角 SIGTERM 关停的理想退出码是 0/143；255 是否可接受
   - Recommendation: 沿用 accept-255 同源语义（CONTEXT discretion 已暗示「与 P6 OQ1 accept-255 同源」）；README 运维节明示；systemd SuccessExitStatus= 由部署侧自决

3. **1001 广播是否复用 EXIT 帧的 2s 写超时定值？**
   - What we know: P6 EXIT 广播 = 每客户端 goroutine 同步 Write 带 2s ctx（server.go:1111-1113，RESEARCH OQ3 定值拒绝可配化）；1001 路径用 conn.Close 直接带关闭帧，Close 内建 5s+5s 上界（close.go:87-89）
   - What's unclear: Close 的上界是否需要在 Shutdown 内再盒一层
   - Recommendation: 不再盒——Close 内建上界足够；stall 端最坏 10s 不阻塞进程退出（exitf 由 lifecycle 子进程路径收口，与 Shutdown goroutine 并发）

4. **配置文件里 exit-when-empty 的取值形态？**
   - What we know: flag 三形态（不写/裸写/=duration）由 exitEmptyValue.Set 承载（"true"→立即）
   - What's unclear: TOML 里写 `exit-when-empty = true`（bool）还是 `exit-when-empty = "30s"`（串）
   - Recommendation: 字符串单形态，复用 exitEmptyValue.Set 解析（"true"/"0"/"30s" 全通）——单一解析路径零双写；bool 形态由 go-toml 类型不符自然拒绝

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | 全部后端改动 | ✓ | go1.26.3（go.mod 钉 1.26.3） | — |
| Node.js | UAT + 前端构建 | ✓ | v24.13.0 | — |
| pnpm | 前端构建 | ✓ | 11.21.0（CI 同钉） | — |
| pelletier/go-toml/v2 | 配置文件 | ✓（proxy 可达） | v2.4.3 | — |
| xdg-open | --open Linux 形态 | ✗（本机无——headless） | — | D-27 skip 路径即设计内行为；UAT 用 fake xdg-open（PATH 前置）断言调用 |
| DISPLAY/WAYLAND_DISPLAY | --open 桌面检测 | ✗（本机两变量均空） | — | headless skip 路径实测环境（本机即常态） |
| 多用户/NSS（nobody 等） | 降权 UAT 场景 | ✓（/etc/passwd 存在） | — | UAT 降权到 self（`id -u`/`id -g`）免 root；降权到 nobody 需 root 列人工/可选场景 |
| Windows 工作站 + Playwright | 浏览器观感层 | ✓（phase06-pw.mjs 先例 46/46） | — | base-path 观感可列可选场景，非阻塞 |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** xdg-open（D-27 skip 即设计）、DISPLAY（headless 是常态部署形态）

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib + `go vet`（CI: `go test -race -count=1 -v ./...`）；UAT: Node 原生脚本（零依赖）+ jsdom 25.0.1 |
| Config file | 无单独配置（go test 内建）；CI 见 .github/workflows/ci.yml |
| Quick run command | `go test ./cmd/wesh ./internal/proto ./internal/pty`（<5s）；单测 `go test -run TestStartupMatrix ./cmd/wesh` |
| Full suite command | `go test -race -count=1 ./...`（~50s）+ `time pnpm -C web build` + `node web/uat/phaseNN.mjs` 全脚本 |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OPS-09 | TOML 加载/未知键拒绝/权限警告/合并优先级 | unit | `go test -run 'TestConfigFile\|TestParseArgs' ./cmd/wesh` | ❌ 新表驱动用例入 main_test.go（TestParseArgs/TestStartupMatrix 扩展） |
| OPS-01 | unix socket listen/mode/owner/残留清理/打印 | unit + UAT | `go test ./cmd/wesh -run TestSocket`（新增）；`node web/uat/phase07.mjs`（relay 全链） | ❌ 新用例；phase07.mjs 新建 |
| OPS-02 | base-path 页面/WS 升级/307/share 交叉 | unit + UAT | `go test ./internal/server -run TestBasePath`（新增）；phase07.mjs S 场景 | ❌ 新增 |
| SEC-07 | auth-header 记录/sanitize/XFF 换键 | unit + UAT | `go test ./internal/server -run 'TestRemoteUser\|TestXFF'`（新增） | ❌ 新增 |
| OPS-04 | cwd/TERM/停止信号进程组/宽限补 KILL | unit + UAT | `go test ./internal/pty -run 'TestStartOptions\|TestSignalGroup'`（新增）；phase07.mjs stop-signal 场景 | ❌ 新增 |
| OPS-05 | uid/gid 降权/身份环境改写 | unit | `go test ./internal/pty -run TestDropPriv`（新增——降权到 self 免 root） | ❌ 新增 |
| OPS-11 | --open headless 跳过/调用参数 | unit | fake xdg-open PATH 前置断言 argv（main_test.go 新增） | ❌ 新增 |
| D-23 | 1001 关停序列（SIGTERM → 1001 → 进程退出） | UAT | phase07.mjs：spawn 实例 SIGTERM → 客户端收 1001 → waitExit | ❌ 新增 |

### Sampling Rate
- **Per task commit:** `go test ./cmd/wesh ./internal/pty ./internal/server -run '<相关 TestX>' -count=1`
- **Per wave merge:** `go test -race -count=1 ./...` + `time pnpm -C web build`
- **Phase gate:** 全量绿 + phase07.mjs 全场景 PASS + 既有 phase02-06 UAT 脚本回归（六段式纪律：新脚本不得破坏旧脚本）

### Wave 0 Gaps
- [ ] `web/uat/phase07.mjs` — 覆盖 OPS-01/02/04/09/11、SEC-07、D-23 的协议层场景（含 TCP relay 夹具）
- [ ] main_test.go 配置文件/新 flag 组合校验表驱动扩展（TestParseArgs/TestStartupMatrix 既有表结构沿用——03-04 先例：命名字段转换，既有行零改动）

*(既有 Go 测试基建完备，无框架安装需求；jsdom/xterm-headless 已在 web/uat 就位)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes（间接） | auth-header **不做认证决定**（D-17 正交）——Basic/ticket/share token 三通道零改动；SEC-07 收窄后无新认证语义 |
| V3 Session Management | no | — |
| V4 Access Control | yes | unix socket 文件权限位即认证边界（D-11）；--socket-mode 默认 0660 最小授权；降权 Credential 最小权限运行 |
| V5 Input Validation | yes | TOML 严格模式（未知键拒）；--base-path 严格字符集（D-13）；--stop-signal 枚举；--uid/--gid 数字直通；--socket-mode 八进制解析上限钳制 |
| V6 Cryptography | no | —（无新加密面；share token 生成既有） |
| V7 Error Handling / Logging | yes | remote_user sanitize C0/C1+128（D-19 日志注入防线）；配置错误文案禁含值（Pitfall 5）；token 永不入 logEvent 红线保持 |

### Known Threat Patterns for {stack}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| X-Forwarded-For 伪造（直连客户端自设 XFF/auth-header） | Spoofing | 裸信任 + D-16 暴露面启动警告（非 loopback 无凭据 stderr 醒目）；--trusted-proxy CIDR 列 deferred；README 明示「--auth-header 仅反代后部署」 |
| 配置文件凭据泄露（world-readable） | Information Disclosure | D-07 权限检查警告（600/400 之外 stderr 提醒）；WESH_CREDENTIAL env 优先于配置文件明文（D-05 链内） |
| 配置解析错误回显敏感值 | Information Disclosure | 记录式错误上报（credErr/clientOptErr 同款两通道纪律，Pitfall 5） |
| 日志注入（remote_user 含控制字符伪造日志行） | Tampering | D-19 C0/C1 剥离 + 128 截断（logEvent stderr 单行形态先行防护，Phase 8 slog 化同纪律） |
| 残留 unix socket 被预置（攻击者先在路径建文件） | Tampering | D-10 listen 前 os.Remove + listen 后显式 Chmod/Chown（窗口期风险 A5 已评估接受）；部署建议 /run 下 root 拥有目录 |
| 降权后身份环境错乱（HOME 指向原用户） | Elevation of Privilege（反向——权限错乱可用性/信息面） | D-25 LookupId 改写 HOME/USER/LOGNAME；查不到剔除三键 |
| stop-signal 误发无关进程组 | Denial of Service | pgid == 子进程 pid（setsid 组长，signal_linux.go:9-10 既定不变量）；PID 复用窗口 = 子进程死后 wesh 立即退出，窗口极小 |

## Sources

### Primary (HIGH confidence)
- **GOROOT go1.26.3 源码（本机 `/data1/home/zexueli/softwares/go`）** — sock_posix.go listenStream（unix bind 无 unlink）；unixsock_posix.go:210-216,230（unlink:true 默认）；net/http/server.go:2687,2721-2745（307 + matchOrRedirect）；syscall/exec_unix.go:124-129（Credential 逐字）；syscall/exec_linux.go:385,490-510,632（forkExec 顺序）
- **GOMODCACHE 在库源码** — creack/pty@v1.1.24/start.go:18-25（SysProcAttr 合并）；coder/websocket@v1.8.15/close.go:29（StatusGoingAway=1001）
- **现状代码逐行（wesh 仓）** — main.go:28-54,104-282,368-412,414-533；spawn.go:40-85；server.go:519-529,561-602,975-977,1062-1124；clients.go:706-758；sharetoken.go:76-122；origin.go:28-96；headers.go:30-44；proto.go:8-38；signal_linux.go 全文件；main.ts:500,509-510,601,842-929；web/embed.go 全文件
- **proxy.golang.org** — go-toml/v2 @latest = v2.4.3（2026-07-05，Origin refs/tags 核实）
- **ttyd 本地源码（~/open_src/ttyd）** — http.c:133-142（-b 302 重定向）；protocol.c:188-189（-H 头值入 pss->user）；server.c:59,65,102,117（-H/-b flag 定义）
- **本机探针实证** — Node v24.13.0：无全局 Agent/getGlobalDispatcher、node:undici ERR_UNKNOWN_BUILTIN_MODULE、裸 undici MODULE_NOT_FOUND；TCP↔unix relay 探针输出 `ok:/ws`；DISPLAY/WAYLAND_DISPLAY 均空
- **既有测试全绿** — `go test ./... -count=1` 5 包 ok（server 44.3s）

### Secondary (MEDIUM confidence)
- **pkg.go.dev 官方文档** — go-toml/v2 DisallowUnknownFields/StrictMissingError（无 SetStrict）；os/signal NotifyContext；net/http StripPrefix；os/user 双实现（cgo/纯 Go /etc/passwd + osusergo tag）；syscall SysProcAttr/Credential
- **undici 官方 WebSocket 文档** — dispatcher 选项存在但 Node 全局环境无可构造 Agent（与本机探针互证）

### Tertiary (LOW confidence)
- nginx location 匹配语义（A1——运维常识级，README 交付前建议复核）；无其他 LOW 项进入推荐面

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 唯一新依赖 go-toml/v2 经 proxy + 官方文档 + STACK.md 三通道；余全部 stdlib/在库源码核实
- Architecture: HIGH — 七个模式六个有 GOROOT/GOMODCACHE/现状代码逐行证据；Pattern 3（前端相对 URL）为 RFC 3986 语义推演 + 现状代码行号锚定
- Pitfalls: HIGH — 八个陷阱七个源码级实证；Pitfall 3 的 401 回归链为推演（UAT 必测项锁定）

**Research date:** 2026-08-25
**Valid until:** 2026-09-24（Go 依赖均为低频稳定库；GOROOT 语义随 go.mod 钉版 1.26.3 不变）
