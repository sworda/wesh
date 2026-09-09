# Phase 7: 部署与配置 - Context

**Gathered:** 2026-08-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 7 让 wesh 在真实运维场景可部署：监听形态齐全（端口 0=随机已有/绑定地址已有/UNIX socket 含属主权限位）；TOML 配置文件落地（仅 --config 显式指定，CLI 覆盖，严格模式）；反代友好（--base-path 子路径挂载 + 尾斜杠规范化；--auth-header 反代透传用户身份进服务端日志，X-Forwarded-For 同闸恢复真实客户端 IP）；子进程管理可配（--cwd/--term/--stop-signal/--stop-timeout）；降权运行（--uid/--gid + 身份环境改写）；--open 自动开浏览器；1001 优雅下线发送路径（P6 deferred 兑现：SIGTERM/INT → 1001 广播 → stop-signal 序列）。

**In scope (from ROADMAP):** OPS-01（监听：端口/绑定/UNIX socket 含属主）、OPS-02（base-path 反代子路径）、OPS-04（子进程 cwd/TERM/关闭信号可配，信号发进程组）、OPS-05（uid/gid 降权）、OPS-09（TOML 配置文件，CLI 覆盖）、OPS-11（--open 自动开浏览器）、SEC-07（auth-header 透传——语义收窄为服务端审计归因，见 D-14）；P5/P6 deferred 兑现（新 flag 配置文件收口、1001 优雅下线发送路径、X-Forwarded-For 解析——P3 deferred 并列项同批）。

**Out of scope (本阶段不做):** /healthz、/metrics、slog 结构化日志（Phase 8 OPS-06/07/08——logEvent 仍是 stderr 单行形态，remote_user/XFF 先进既有 logEvent 字段）；自定义首页（Phase 9 OPS-03）；单二进制四平台发布与参数标定回填（Phase 9 OPS-10）；配置文件热重载/多配置文件（无需求）；前端身份显示（SEC-07 收窄后的可选增强，见 deferred）；ACME 自动证书（v2 V2-ACME）；Windows 平台（含 --open 的 rundll32 形态，PROJECT Out of Scope）。

**已锁定不重复决策：** GoTTY 共享进程模型（PTY 随服务端启动、多客户端共享——SEC-07 env 注入不可行的根因，D-14）；CLI flag 全名无短选项 + 启动校验矩阵 fail-fast + 敏感值记录式错误上报（P2 D-15/P3/SEC-01 启动面红线）；配置文件解析用 pelletier/go-toml/v2 不用 viper（STACK.md 定案）；P3 D-12 Origin 显式可信源（反代 × Origin 张力已闭合，base-path 不得推翻）；SEC-06 子进程 env 替换式白名单注入（降权身份改写走白名单通道）；端口 0=随机打印实际端口（现状行为，OPS-01 该半条已落地）；exitf + sync.Once 单一终结收口（P1 硬约束——stop-signal 序列与 1001 广播只加触发源不加 exitf 分支）；关闭码全集 {1000,1002,1008,1009,1011,1013}（P2 D-05）+ 1001 占位（P2 D-08 同批，本 phase 启用）。

</domain>

<decisions>
## Implementation Decisions

### 配置文件（OPS-09）
- **D-01:** 路径发现 = **仅 `--config` 显式指定**，不做任何隐式默认路径搜索——裸 `wesh -- bash` 行为与今天完全一致，零意外；systemd 部署显式路径最可控（与项目「显式逃生门」哲学一致） — **Reversibility:** one-way — CLI flag 公开契约（P2 D-15 纪律）
- **D-02:** 可重复 flag（--credential/--origin/--client-option）与配置文件同名列表的合并 = **CLI 给出则替换整个列表**——与 P3 D-01 WESH_CREDENTIAL env 兜底「flag 非空则整体忽略」先例一致；CLI 能完整表达最终状态（含「移除」配置文件条目）；标量 flag 自然是 CLI 显式设置则覆盖（fs.Visit 显式设置位模式承载） — **Reversibility:** one-way — 合并语义是部署行为契约，改动会让既有配置文件+CLI 组合部署的生效集漂移
- **D-03:** TOML 形状 = **平铺 key = value，键名 = flag 名**——心智成本零，help 文案与配置文档单一事实源；go-toml v2 直解到与 config struct 同构类型；拒绝分组 sections（双写漂移风险）
- **D-04:** 覆盖面 = **全部长期运行 flag 可入配置 + `command = ["bash", "-l"]` exec 数组形式可入**（CLI `--` 后 argv 非空则覆盖）；边界：--no-auth/--insecure-http 逃生门、--version/--help/--config 本身不入配置文件（逃生门必须显式说出口，配置文件里写出来等于没说）
- **D-05:** 优先级链 = **flag > env > 配置文件 > 内置默认**——与 P3 D-01 现状完全兼容（无配置文件时 flag > env 不变）；env 作为 systemd EnvironmentFile= 600 通道仍优先于配置文件明文
- **D-06:** 加载失败与未知键 = **exit 2 fail-fast 严格模式**——文件不存在/TOML 解析失败/未知键均拒绝（与启动校验矩阵同档；未知键拒绝防拼写错误静默失效，--client-option 白名单「显式优于静默」同哲学）
- **D-07:** 文件权限检查 = **含 credential 键且权限非 600/400 时 stderr 警告放行**（不阻断——挂载盘/容器 secret 权限语义不可靠，ssh 式拒绝误伤多）+ README 明示建议 chmod 600

### 监听形态（OPS-01）
- **D-08:** UNIX socket = **独立 `--socket /run/wesh.sock` flag，与 --port/--bind 互斥**（组合冲突进 validateStartup fail-fast）——显式零字符串解析歧义，拒绝 --bind 值特判 — **Reversibility:** one-way — CLI flag 公开契约
- **D-09:** socket 属主/权限 = **`--socket-mode 0660`（八进制，默认 0660）+ `--socket-owner user[:group]`**（user.Lookup 解析）；两 flag 仅随 --socket 有意义，单独给出 = 配置矛盾 fail-fast（write-policy 组合校验同位先例） — **Reversibility:** one-way — CLI flag 公开契约
- **D-10:** 既有 socket 文件 = **listen 前 os.Remove**（IPC 端点残留即垃圾；不 unlink 则 bind EADDRINUSE；systemd Restart= 场景零人工干预）
- **D-11:** unix socket 形态下 validateStartup 的 bind 安全校验矩阵 = **本机信任跳过**（loopback 早退同款逻辑）——访问控制由 socket 文件权限位承担，文件系统权限即认证边界
- **D-12:** unix socket 形态下启动打印 = `listening on unix:///path` 实际地址；分享链接两行退化为 **unix:// 提示行**（明示反代后链接由反代 URL 决定）——无 host:port 可拼时绝不拼误导性 TCP 链接

### base-path 反代挂载（OPS-02）
- **D-13:** `--base-path /wesh` 值校验 = **严格模式：必须以 / 开头、不得以 / 结尾（根 / 视为未配置）、拒绝 .. 与重复斜杠、仅 URL path 安全字符**——parse 期规范化+校验（NormalizeOrigin 先例），非法值 exit 2；拒绝宽容自动修正（输入与生效值不一致是配置漂移隐蔽源） — **Reversibility:** one-way — CLI flag 公开契约
- **D-14:** 前端 URL 构造 = **改相对路径**（`fetch('api/attach')`、`new URL('ws', location.href)`、share 正则不锚 ^）——go:embed 静态伺服零改动、无模板注入面；**配套硬要求：`/wesh` → `/wesh/` 尾斜杠 307 规范化**（PITFALLS「单侧定义」锁定；尾斜杠缺失时相对解析丢前缀）；拒绝服务端注入 base（破坏单文件零处理现状、引入 CSP 复杂度）；Origin 校验不受影响（P3 D-12 已闭合张力）；分享链接打印含 base-path

### auth-header 透传与 X-Forwarded-For（SEC-07）
- **D-15:** SEC-07 语义收窄 = **只要审计归因：attach 时 logEvent 记录 remote_user**——共享进程模型下 per-client env 注入结构性不成立（PTY 随服务端启动 spawn 时无 HTTP 请求；多客户端共享一个 shell，env 是一次性快照、写谁的名字都错）；SEC-07 需求文本修订为服务端侧身份记录，README 明示与 ttyd -H 的模型差异（shell 内身份感知在共享 shell 下本质不成立，ttyd per-connection spawn 才能做到） — **Reversibility:** one-way — 需求文本修订 + README 公开承诺的模型差异说明；将来若要 env 注入需 attach 时 spawn 的架构变更
- **D-16:** 信任模型 = **裸信任 + 暴露面启动警告**：`--auth-header X-Remote-User` 配置即信任该头（ttyd 同款）；validateStartup 检测 bind 非 loopback 且无凭据时 stderr 警告「auth-header 可被直连伪造，确保 wesh 不直接暴露」；可信来源 IP 校验列 deferred（按真实部署反馈再加）
- **D-17:** 与认证体系关系 = **正交提取**：只做用户名提取进 logEvent，不做任何认证决定；Basic/--no-auth/share token 语义全不变（零新认证语义、零信任模型扩张；「头存在跳过 Basic」被否决——与裸信任叠加伪造头即绕过认证）
- **D-18:** 头名 = **`--auth-header` 可配头名（单个）**——反代生态头名不统一（authelia 发 Remote-User、oauth2-proxy 发 X-Forwarded-User），可配零猜测；多反代串联由反代统一头名 — **Reversibility:** one-way — CLI flag 公开契约
- **D-19:** remote_user 值清洗 = **剥离 C0/C1 控制字符 + 截断 128 字符**（P4 D-03 标题 sanitize 同款纪律；Phase 8 OPS-08「用户可控字段剥离控制字符」既定方向本 phase 先行——logEvent 是 stderr 单行文本，控制字符注入伪造日志行的风险当期就存在）
- **D-20:** X-Forwarded-For **同批做**（P3 deferred 并列项）：与 auth-header **共用信任闸**（--auth-header 给定 = 「信任反代」总开关，零双轨）；XFF 取链中首个 IP；消费范围 = logEvent remote 字段与 throttle per-IP 键同换（反代后 per-IP 计数全聚合为代理 IP 的现状限制解除） — **Reversibility:** costly — throttle 计数键变更是安全语义变更，反代部署的节流行为改变

### 子进程管理（OPS-04）
- **D-21:** `--cwd /path`（默认继承服务端 cwd 现状）+ `--term`（默认 xterm-256color 现状）两 flag——落 cmd.Dir 与 whitelistEnv 的 TERM= 行（spawn.go:50,65 注释预留位）；--cwd 目录不存在 = stat 预检启动报错 fail-fast（spawn 前零资源占用） — **Reversibility:** one-way — CLI flag 公开契约
- **D-22:** 停止信号 = **`--stop-signal HUP|TERM|INT|KILL`（默认 HUP 保持现状）+ `--stop-timeout`（默认 0 = 不补 KILL 纯单信号）**——exitf 收口时显式 kill(pgid, stop-signal) → 等 timeout → 仍存活补 SIGKILL；「Close master 内核发 SIGHUP」免费通道保留为兼容底层，显式信号在上层；exitf + sync.Once 单一收口纪律保持，只加触发源不加分支 — **Reversibility:** one-way — CLI flag 公开契约
- **D-23:** 1001 优雅下线（P6 deferred 兑现）= **wesh 捕获 SIGTERM/SIGINT → 向全部客户端发 1001 Going Away → 子进程 stop-signal 序列 → exit**——1001 不在 CORE-05 重连触发集（P6 D-01 仅 1006），前端显示「Server shutting down」面板而非重连循环（systemd restart 场景 UX 闭环；1001 启用 P2 D-08 占位，扩展线上关闭码集合） — **Reversibility:** one-way — 关闭码是前后端公开协议契约（P2 D-05 同评级）

### 降权运行（OPS-05）
- **D-24:** `--uid`/`--gid` **数字直通，成对给出**（只给一个 = 启动报错）——避免静态二进制在极简容器（无 /etc/passwd）里的 NSS 解析差异；名字解析场景运维先 `id -u` 查好；降权挂点 = spawn 时 SysProcAttr.Credential（fork 后 exec 前） — **Reversibility:** one-way — CLI flag 公开契约
- **D-25:** 降权后身份环境 = **按目标 uid 查 passwd 条目自动改写白名单里 HOME/USER/LOGNAME**（查不到则剔除三键让 shell 自默认）——降权直觉语义 = 连身份环境一起降，否则子进程 HOME 指向原用户家目录即权限错乱；走 SEC-06 白名单通道（替换式注入纪律不变）

### 自动打开浏览器（OPS-11）
- **D-26:** `--open` 布尔 flag，打开 **operator 视角入口：--writable 时开 rw 分享链接，否则开 ro 链接**（含 token 免交互即打即用；token 通道绕过 Basic 是 P5 D-01 既定语义） — **Reversibility:** one-way — CLI flag 公开契约
- **D-27:** 平台机制 = **xdg-open（Linux）/ open（macOS）**；headless 检测（无 DISPLAY 且无 WAYLAND_DISPLAY）时 **stderr 提示后跳过不阻断启动**（headless 服务器是常态部署形态，--open 本质是桌面便利功能）；Windows 不做（PROJECT Out of Scope）

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

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 7 — 成功准则 3 条（监听形态+TOML 配置 CLI 覆盖 / base-path 页面与 WS 升级正常 + auth-header 环境变量【已被 D-15 修订为服务端日志归因】/ cwd+TERM+停止信号进程组+uid/gid 降权+--open）
- `.planning/REQUIREMENTS.md` — OPS-01/02/04/05/09/11、SEC-07 原文（SEC-07 按 D-15 收窄解读）
- `.planning/PROJECT.md` — Key Decisions（GoTTY 共享进程模型——D-15 根因）、Constraints（单静态二进制——D-24 数字 uid 依据；Linux+macOS——D-27 平台边界）、Out of Scope（ttyd CLI 不兼容——本 phase 全部新 flag 全新设计）

### 调研结论
- `.planning/research/STACK.md` — pelletier/go-toml/v2 配置文件解析定案（不用 viper）；stdlib net/http ServeMux 路由（base-path 装配不引框架）
- `.planning/research/FEATURES.md` §功能张力 — base-path/auth-header × Origin 校验张力（P3 D-12 已闭合，D-14 不得推翻）；§补漏 #3 auth-header 透传原始动机（挂 SSO 的用户会第一时间提）
- `.planning/research/PITFALLS.md` — base-path 尾斜杠表（301 丢 WS 升级教训 → D-14 的 307 规范化单侧定义）；nginx 反代配方表（Upgrade/Connection 头、proxy_read_timeout——README 反代节素材）；§部署矩阵（降权/cwd 预留接口——D-21/D-24 落点）
- `.planning/research/ARCHITECTURE.md` — PTY Engine 职责表（forkpty/setsid、env 白名单、cwd、uid/gid 预留——D-21/D-24/D-25 的架构依据）

### 前序 phase 决策
- `.planning/phases/03-auth/03-CONTEXT.md` — D-01（flag 优先 env 兜底——D-02/D-05 先例）、D-03/D-05（启动校验矩阵与逃生门——D-04 边界依据、D-11 unix socket 跳过形态同源）、D-12（Origin 显式可信源）、deferred（X-Forwarded-For/auth-header——D-15/D-20 兑现）
- `.planning/phases/05-multi-client/05-CONTEXT.md` — D-01（token 通道绕过 Basic——D-26 --open 开分享链接依据）、D-03（token 永不入日志红线延伸——remote_user 记录不得含 token）、deferred（新 flag 配置文件收口——D-01..D-07 兑现）
- `.planning/phases/06-session-lifecycle/06-CONTEXT.md` — D-01（重连仅 1006 触发——D-23 的 1001 不进重连集依据）、D-13（断开退出 SIGHUP 收口——D-22 stop-signal 默认 HUP 的现状语义）、deferred（1001 优雅下线发送路径——D-23 兑现）

### 现状代码（扩展点）
- `cmd/wesh/main.go` — config struct + parseArgs（17 flag 现状 + fs.Visit 显式设置位模式——D-02 合并与全部新 flag 的宿主；credErr/clientOptErr 记录式上报先例——敏感值新 flag 同纪律）；validateStartup（校验矩阵——D-08/D-09/D-11/D-16/D-24 组合校验落点）；run()（net.Listen 分岔点——D-08 unix socket 挂点；启动打印——D-12/D-26；ServeTLS 分岔）
- `internal/pty/spawn.go` — Start（cmd.Dir/TERM= 注释预留位——D-21；SysProcAttr.Credential——D-24 降权挂点）；whitelistEnv（SEC-06 白名单——D-25 身份改写挂点）
- `internal/server/server.go` — Handler() mux 装配（Go 1.22 模式路由——D-14 base-path 前缀挂点）；logEvent 三要素（D-15/D-19/D-20 记录点；token 永不入参红线）；clientIP（D-20 XFF 消费点）；throttle per-IP 键（D-20）；shareAttach/attachHandler（D-15 attach 时提取挂点）；lifecycle（D-23 的 1001 广播挂点参照 P6 EXIT 帧先例）
- `internal/server/headers.go` — securityHeaders 中间件先例（D-14 base-path 装配若取中间件形态的参照）
- `internal/server/origin.go` — NormalizeOrigin parse 期规范化先例（D-13 --base-path 校验参照）
- `web/src/main.ts` — 硬编码 URL 三处（L510 '/api/attach'、L601 '/ws'、L500 share 正则 ^/s/——D-14 相对路径改造点）；onclose 按码分派（D-23 的 1001 分派「Server shutting down」面板——showStatus 三态复用）
- `web/uat/phaseNN.mjs` — UAT harness 模式（phase07.mjs 同款；unix socket 场景的 Node WS over unix socket 连接形态需探针）

### ttyd 源码（缺陷对照面，不参考实现）
- `~/open_src/ttyd/` — `-H` auth-header（D-15 模型差异对照）；`-i` 接口监听含 unix socket（D-08 独立 flag 的对照面）；`-b` base-path（行为对照）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `config struct + fs.Visit 显式设置位`（main.go:28-54, 216-228）— 配置文件合并（D-02/D-05）的天然宿主：标量覆盖与列表替换都靠「显式设置位」判定，writePolicySet/maxClientsSet/exitEmptySet 三先例已立
- `credErr/clientOptErr 记录式错误上报`（main.go:143-152, 174-192）— 值含敏感内容的新 flag（如配置文件里的 credential）同纪律：校验错误先记录、Parse 返回处统一上报，两通道不含值
- `validateStartup 纯函数校验矩阵`（main.go:368-412）— 全部新组合矛盾（--socket×--port、--socket-owner 单给、--uid 单给、--auth-header 暴露面警告）的落点；loopback 早退形态是 D-11 unix socket 跳过的直接参照
- `NormalizeOrigin`（origin.go）— parse 期规范化+校验先例，--base-path 严格校验（D-13）同款形态
- `exitEmptyValue`（main.go:68-102）— 自定义 flag.Value + IsBoolFlag 先例，--stop-timeout 若取自定义值形态的参照
- `whitelistEnv`（spawn.go:63-85）— SEC-06 替换式注入白名单，D-25 身份环境改写在此通道内做（查 passwd 改 HOME/USER/LOGNAME 三键，绝不动 os.Environ() 追加纪律）
- `logEvent 三要素单行`（server.go）— remote_user/XFF 唯一记录出口；token 永不入参红线保持
- `P6 EXIT 帧广播形态`（lifecycle 组帧一次共享只读 + 每客户端 goroutine 同步 Write 带超时）— D-23 的 1001 广播直接参照
- `showStatus 三态面板 + onclose 按码分派`（main.ts）— 1001「Server shutting down」面板落此，零新 UI 组件
- UAT harness（web/uat/phaseNN.mjs）— phase07.mjs 同款；Node 原生 WS 支持 unix socket 路径连接（需探针验证）

### Established Patterns
- **CLI flag 全名无短选项 + 启动校验矩阵 fail-fast + 分层纪律**（parse = 形状与展开，validate = 组合矛盾）— 全部新 flag 同纪律；--socket 与 --port/--bind 互斥校验落 validate 层
- **exitf + sync.Once 单一终结收口**（P1 硬约束）— stop-signal 序列与 1001 广播只加触发源（SIGTERM/INT 捕获、注册表空既有路径），不加 exitf 分支
- **守卫区顺序敏感 + Accept 前零 WS 资源分配** — auth-header/XFF 提取在 HTTP 层完成（零 WS 资源），attach 时记录
- **帧常量与关闭码前后端手工对齐**（proto.go ↔ main.ts 注释互相指路）— 1001 启用两侧同步注释
- **logEvent 三要素 stderr 单行**（Phase 8 才 slog 化）— 本 phase remote_user/XFF 进既有三要素形态，不提前结构化
- **前端相对 URL 零服务端注入**（go:embed 单 HTML 静态伺服）— D-14 相对路径改造保持单文件零处理现状
- **307 保方法重定向**（Go 1.22+ mux matchOrRedirect 恒 307，05-06 实证）— /wesh → /wesh/ 规范化落同一机制

### Integration Points
- `main.go parseArgs` — 全部新 flag（--config/--socket/--socket-mode/--socket-owner/--base-path/--auth-header/--cwd/--term/--stop-signal/--stop-timeout/--uid/--gid/--open）注册 + 配置文件两阶段合并（先解 --config 路径加载 TOML 铺底，再 CLI 覆盖）
- `main.go run()` — net.Listen 分岔（tcp vs unix；socket unlink/mode/chown 序列）；SIGTERM/INT 捕获挂点；--open 启动后调用；启动打印 unix:// 分支
- `server.go Handler()` — base-path 前缀装配（模式串拼接或 StripPrefix）；auth-header/XFF 提取中间件（headers.go 先例）
- `server.go Attach/attachHandler` — remote_user 提取记录挂点（guard 区 HTTP 层，Accept 前）
- `pty/spawn.go Start` — cmd.Dir、TERM= 行、SysProcAttr.Credential（uid/gid）、whitelistEnv 身份改写
- `pty.Session / server lifecycle` — stop-signal 显式 kill(pgid) 挂点（Close master 之前）、timeout 后 KILL 补发、1001 广播（EXIT 帧先例挂点）
- `main.ts` — 相对路径三处改造（fetch 'api/attach'、new URL('ws', location.href)、share 正则不锚 ^）；onclose 1001 分派面板；dist 重建入库
- `proto.go` — 1001 关闭码注释启用（前后端对齐）

</code_context>

<specifics>
## Specific Ideas

- **SEC-07 收窄的用户裁决过程**：先发现 ttyd -H 的 per-connection spawn 与 wesh GoTTY 共享模型的结构冲突（spawn 时无 HTTP 请求、共享 shell 无「当前用户」），用户要求说明注入用途后明确「只要审计归因」——shell 内身份感知被判定为共享模型下本质不成立的能力，不追。需求文本修订 + README 明示模型差异是正式交付物
- **「显式」哲学的一贯性**：--config 仅显式指定（不搜索默认路径）、逃生门不入配置文件（写出来等于没说）、--socket 独立 flag（不字符串特判）、--base-path 严格校验（不宽容修正）、未知配置键拒绝（不静默忽略）——本 phase 每个灰区的推荐项都落在同一哲学上
- **文件系统权限即认证边界**：unix socket 跳过 bind 校验矩阵的理据——socket-mode/owner 就是访问控制，与 loopback「流量不出机」同档信任
- **1001 的 UX 闭环价值**：不在 CORE-05 重连触发集（仅 1006）是刻意设计——systemd restart 时客户端看到「Server shutting down」而非重连循环打一个正在重启的服务
- **降权 = 连身份环境一起降**：自动改写 HOME/USER/LOGNAME 的理据是「降权运行」的直觉语义——否则 root 启动降权到 nobody 后 shell HOME 指向 /root 开箱即坑
- **--open 开分享链接而非根路径**：token 通道免交互即打即用（P5 D-01 既定），operator 本机操作无泄露面；rw 优先因为 operator 要操作

</specifics>

<deferred>
## Deferred Ideas

- **remote_user 进 slog 结构化审计事件**（attach/detach 事件携 remote_user 字段检索）— Phase 8 OPS-08；本 phase 先进 logEvent stderr 单行（D-15）
- **前端身份显示**（Welcome 帧携 remote_user，标题/面板显示「as alice」）— SEC-07 收窄时被用户裁掉的第二层价值；若真实 SSO 部署反馈需要 operator 可见性再评估
- **auth-header 可信来源 IP 校验**（--trusted-proxy CIDR 限定采信来源）— D-16 裸信任+警告的升级路径；按真实部署反馈再加
- **/healthz、/metrics、per-IP 节流在 XFF 下的指标口径** — Phase 8 OPS-06/07
- **配置文件热重载（SIGHUP reload）与多配置文件** — 无需求支撑，roadmap 未列
- **自定义首页 HTML（--index）** — Phase 9 OPS-03
- **负载测试标定回填**（stop-timeout 合理默认等）— Phase 9
- **Windows 平台 --open（rundll32 url.dll）与整体 Windows 支持** — PROJECT Out of Scope（非延期，终局不做）

</deferred>

---

*Phase: 7-deployment*
*Context gathered: 2026-08-25*
