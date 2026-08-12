# Stack Research — wesh（Web 终端分享工具，ttyd 现代化重写）

**Domain:** 单静态二进制 Web 终端服务器（PTY ↔ WebSocket ↔ xterm.js）
**Researched:** 2026-08-13
**Confidence:** MEDIUM（所有版本号均以官方 registry API 当日核实并交叉验证；Go-vs-Rust 决策基于多条独立 MEDIUM 证据的一致指向）

---

## 核心决策：后端选 Go

**推荐：Go 1.26.x。** Rust 栈（tokio + axum + rustls + portable-pty）完全可行且同样消除 ttyd 的 C 内存 bug 类别，但对本项目"单静态二进制、Linux+macOS、个人运维工具、快速迭代"的约束组合，Go 在每个维度上都更贴合：

| 维度 | Go | Rust | 裁决 |
|---|---|---|---|
| PTY 库 | `creack/pty` v1.1.24 —— forkpty 薄封装，1,263 个导入模块（AWS SSM agent、Docker buildx、CRI-O、containerd/nerdctl、Argo、Coder、Charm 等），纯 Go 无 cgo | `portable-pty` 0.9.0（wezterm 出品）为阻塞式 std::io Read/Write，接 tokio 需 spawn_blocking/AsyncFd 胶水；`pty-process` 0.5.3 虽 tokio-native 但生态薄（14KB 小众 crate） | **Go 胜**（生态证据差一个数量级） |
| 并发模型 | goroutine-per-connection + channel，与"PTY↔WS 数据泵 + 会话 hub 广播"结构一一对应，无需 async 框架仪式 | tokio task 图 + `select!`，功能更强但此类 I/O 泵程序用不上 | **Go 胜**（匹配问题形状） |
| HTTP/TLS | stdlib `net/http` + `crypto/tls`（HTTP/2 内建、无依赖决策） | axum 0.8 + tokio-rustls 0.26 + rustls 0.23，三者均优秀但存在版本耦合矩阵 | **Go 胜**（零依赖决策） |
| 前端嵌入 | stdlib `embed` + `http.FS`（Go 1.16+），零依赖 | `rust-embed` 8.12（derive 宏，活跃维护）或 `include_str!` | 平手（都简单） |
| 静态二进制 | `CGO_ENABLED=0` 默认全静态；单台 Linux 机器交叉编译 linux/amd64、linux/arm64、darwin/arm64 全部产物 | Linux 静态需 musl target 或 cargo-zigbuild；macOS 产物必须在 macOS 上构建（或 osxcross） | **Go 胜**（发布流水线最简） |
| 编译速度 | 秒级 | tokio+axum+rustls 依赖树分钟级 | **Go 胜**（个人项目迭代体验） |
| 内存安全 | GC + 内存安全（消除 ttyd 全部 UAF/空指针/越界 bug 类）；残余风险为数据竞争，用 `-race` CI + hub 单所有权模式规避 | 编译期内存安全 + 无 GC | Rust 小胜（但 ttyd 的教训是 C 的问题，Go 已覆盖） |
| 同类先例 | gotty、upterm（Go） | sshx（Rust+gRPC 中心化服务器，非单二进制模式） | 平手 |

**Rust 的诚实优势（何时改选 Rust）：** 团队 Rust 熟练度显著高于 Go；需要极致内存占用；或想要 axum 的一流 WS 配置 API 与 rustls 生态并愿意承担 musl/zigbuild 工具链与 macOS 本地构建。sshx 证明了 Rust+axum 做协作终端可行。若改选 Rust，备选栈见文末"Alternatives Considered"。

---

## Recommended Stack

### Core Technologies（后端）

| Technology | Version | Purpose | Why Recommended |
|---|---|---|---|
| Go | 1.26.5（toolchain `go1.26.x`；次稳定线 1.25.12） | 后端语言 | 单静态二进制最佳发布故事：CGO_ENABLED=0 全静态 + 单机交叉编译四平台；秒级编译；goroutine 模型匹配 PTY↔WS 泵。版本经 go.dev/dl 官方 JSON 核实（2026-08-13） |
| net/http（stdlib，Go 1.22+ ServeMux 模式路由） | 随工具链 | HTTP 服务/路由/静态资源 | wesh 只有少量端点（index、WS 握手、/token、/healthz、metrics），引入 gin/echo/chi 是负资产；stdlib 内置 HTTP/2、超时控制、`http.Server.Shutdown` 优雅退出 |
| crypto/tls（stdlib） | 随工具链 | TLS 终止 | 生产级、默认禁旧协议、无需链接 OpenSSL（保静态二进制）；`tls.Config{MinVersion: tls.VersionTLS13}` 一行收敛协议版本 |
| github.com/creack/pty | v1.1.24 (2024-10-31) | forkpty/TIOCSWINSZ/窗口尺寸 | Go 生态 PTY 事实标准（pkg.go.dev 登记 1,263 个导入模块，含 AWS/Docker/CRI-O/containerd/Coder）；纯 Go 无 cgo，darwin+linux 均可用；低频发布是"稳定 syscall 薄封装"的正常状态，非弃维 |
| github.com/coder/websocket | v1.8.15 (2026-06-15) | WebSocket 服务端 | 零依赖、autobahn-testsuite 全通过；**`SetReadLimit`（默认 32KB，超限即 1009 关闭）直接根治 ttyd 的预认证分片重组内存放大漏洞**；RFC 合规 Close（根治 ttyd 写 1006 close frame 的违规）；context 一等支持；并发写安全（免 gorilla 的并发写 panic 类）；`OriginPatterns` 默认拒绝跨域（支撑 Origin 允许列表需求）；Coder 公司生产使用（coder.com 远程开发流量） |
| embed（stdlib）+ io/fs + http.FS | 随工具链 | 前端产物嵌入二进制 | 零依赖；`//go:embed all:dist` + `fs.Sub` + `http.FileServerFS`，配合构建期预 gzip 直发（Accept-Encoding 协商），复刻 ttyd 的嵌入分发形态 |
| golang.org/x/crypto/acme/autocert | x/crypto v0.55.0 (2026-08-11) | 进程内 ACME（Let's Encrypt）自动证书 | 最简路径：`autocert.Manager.GetCertificate` 挂进 `tls.Config` 即得 HTTP-01/TLS-ALPN-01 自动签发续期；3,292 个导入模块；x/crypto 持续活跃维护 |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---|---|---|---|
| github.com/caddyserver/certmagic | v0.25.4 (2026-06-09) | 全生命周期 ACME（多 CA、存储后端、OCSP stapling、自动续期） | 当 autocert 不够用时（需要证书持久化到指定目录、多域名管理、OCSP）；Caddy 的生产心脏，比 autocert 重但省心 |
| github.com/spf13/pflag | v1.0.10 (2025-09-02) | GNU 风格 CLI 参数（长选项、短选项合并） | wesh CLI 选项数量对标 ttyd（30+），stdlib `flag` 的 `-flag` 单横线风格与 Unix 惯例不符；pflag 是 kubectl/hugo 同款 |
| github.com/pelletier/go-toml/v2 | v2.4.3 (2026-07-05) | 配置文件解析（TOML） | PROJECT.md 要求配置文件支持；TOML 适合手写运维配置；v2 活跃维护。不用 viper——对单文件配置过重 |
| golang.org/x/time | v0.15.0 (2026-02-11) | `rate.Limiter` 令牌桶 | 认证失败节流防爆破 + 每客户端写入限速（PROJECT.md 安全与资源控制需求） |
| log/slog（stdlib） | 随工具链 | 结构化日志 | ttyd 缺失的可运维性之一；JSON handler 直出，无需 zap/logrus |
| crypto/subtle（stdlib） | 随工具链 | 时序安全比较 | 修复 ttyd `strcmp` 比较凭据的非时序安全问题：`subtle.ConstantTimeCompare` |
| testing + `go test -race`（stdlib） | 随工具链 | 竞态检测 | Go 相对 Rust 的残余风险是数据竞争；CI 全量 `-race` 是强制纪律 |

### Development Tools

| Tool | Purpose | Notes |
|---|---|---|
| goreleaser | 发布流水线 | 一条命令产出 linux/amd64、linux/arm64、darwin/amd64、darwin/arm64 四个静态二进制 + checksums + GitHub Release；`CGO_ENABLED=0` 环境即全静态 |
| golangci-lint | lint 聚合 | 启用 govet/staticcheck/errcheck/gosec；gosec 覆盖安全规则 |
| pnpm | 前端包管理 | 用户既定偏好；lockfile 保证前端构建可复现 |
| Node.js | ≥22.12（LTS） | Vite 8 engines 要求 `^20.19.0 \|\| >=22.12`；仅构建期需要，不进产物 |

### 前端 Core Technologies

| Technology | Version | Purpose | Why Recommended |
|---|---|---|---|
| @xterm/xterm | 6.0.0 (2025-12-22) | 浏览器终端模拟器 | 最新主版本；**v6 已删除 canvas 渲染器**（WebGL/DOM 二选一，ttyd 的 `canvas` 兼容回落选项彻底过时）；新增同步输出（DEC mode 2026）、OSC52 剪贴板、原生 ESM。5.4 起官方包全部迁到 `@xterm` scope（旧 `xterm`/`xterm-*` 已废弃，防 typosquatting） |
| @xterm/addon-webgl | 0.19.0 (2025-12-22，与 6.0.0 同 commit 发布） | WebGL2 渲染器 | ttyd 已验证的主渲染路径；失败回落 DOM 渲染器 |
| @xterm/addon-fit | 0.11.0 | 容器自适应尺寸 | 前端 resize → 服务端 TIOCSWINSZ 链路的前端侧 |
| @xterm/addon-unicode11 / addon-web-links / addon-clipboard | 与 6.0.0 同批次的 @xterm scope 版本 | Unicode 11 宽字符（CJK/IME）、超链接识别、剪贴板 | ttyd 功能基线对等项；剪贴板用 `navigator.clipboard` API 替代 ttyd 的 `execCommand('copy')`（已废弃） |
| @xterm/addon-serialize | 与 6.0.0 同批次 | 终端内容序列化 | 可选但推荐：断线重连后前端本地状态恢复的服务端滚动回放之外的补充手段 |
| Vite | 8.2.1 | 前端构建（rolldown 内核） | 当前主版本已切换到 rolldown（Rust 实现），构建快；ttyd 的 webpack 链是 2019 年遗产，无沿用理由 |
| vite-plugin-singlefile | 2.3.3 | 全部 JS/CSS inline 成单个 HTML | 显式支持 vite ^5.4.21–^8；产出 ttyd 同款"单 HTML 产物"，go:embed 只需嵌一个文件 |
| TypeScript | ^5.5（xterm.js 6 仓库自身基线；上游最新 7.0.2 原生编译器，亦可） | 前端语言 | ttyd 前端已是 TS；xterm.js 类型定义完善 |

## Installation

```bash
# === 后端（在 go.mod 根目录）===
go mod init github.com/<you>/wesh
go get github.com/creack/pty@v1.1.24
go get github.com/coder/websocket@v1.8.15
go get golang.org/x/crypto@latest          # acme/autocert + 自签证书生成
go get golang.org/x/time@v0.15.0           # rate limiter
go get github.com/spf13/pflag@v1.0.10
go get github.com/pelletier/go-toml/v2@v2.4.3
# 可选（ACME 全生命周期）：
go get github.com/caddyserver/certmagic@v0.25.4

# === 前端（web/ 目录，pnpm）===
pnpm add @xterm/xterm@^6.0.0 @xterm/addon-webgl @xterm/addon-fit \
  @xterm/addon-unicode11 @xterm/addon-web-links @xterm/addon-clipboard \
  @xterm/addon-serialize
pnpm add -D vite@^8 typescript vite-plugin-singlefile@^2.3.3

# === 构建与发布 ===
# 前端：vite build（singlefile 插件产出单个 index.html，构建期 gzip 预压缩）
# 后端：CGO_ENABLED=0 go build -ldflags="-s -w" ./cmd/wesh
# 发布：goreleaser release（4 平台静态产物）
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|---|---|---|
| Go 后端（上文整套） | **Rust 备选栈**：tokio 1.53 + axum 0.8.9（WS 内建，WebSocketConfig 可调 max_message_size/max_frame_size）+ rustls 0.23.43/tokio-rustls 0.26.4（TLS1.2/1.3 only，ring 或 aws-lc-rs provider）+ instant-acme 0.8.5（RFC 8555 全功能 ACME，rustls 团队 djc 维护）+ portable-pty 0.9.0 或 pty-process 0.5.3（tokio-native）+ rust-embed 8.12.0 | 团队 Rust 更熟练；想要编译期内存安全与无 GC；愿意接受 musl/cargo-zigbuild 工具链、macOS 必须本机构建、分钟级编译。注意 portable-pty 是阻塞 I/O，接 tokio 需 spawn_blocking 胶水 |
| stdlib net/http | chi / echo / gin | 端点数量膨胀到需要中间件生态时；wesh v1 不需要 |
| autocert | certmagic | 需要证书落盘管理、多域名、OCSP 时换 certmagic（同为 Go，换成本低） |
| 进程内 ACME | 外部 Caddy/nginx 反代终止 TLS 或 Tailscale | **个人运维常见场景：机器在 NAT/内网后、无域名——ACME 根本不可用**（HTTP-01 需公网可达 80 端口）。此时文档应推荐 Tailscale（自带证书与加密）或 mkcert/自签 + 静态证书文件，进程内 ACME 是"有公网域名"场景的可选增强，不是默认路径 |
| Vite 8 | esbuild 裸用 / Rsbuild | esbuild 裸用省插件但要手写 inline 逻辑；Rsbuild 是另一套生态。Vite + singlefile 插件是最短路径 |
| xterm.js 6 | 自研终端 / hterm | 无替代必要——xterm.js 是 VS Code/Hyper 同款，ttyd 已验证 |

## What NOT to Use

| Avoid | Why | Use Instead |
|---|---|---|
| **裸 libwebsockets 回调状态机** | ttyd 的全部两个预认证严重漏洞（空消息空指针 DoS、分片重组无上限内存放大）都发生在手写 LWS_PRE 预留与手工分片重组代码里；pss 生命周期跨 lws/libuv 双域靠标志位防 UAF | coder/websocket：框架内建分片重组 + SetReadLimit 上限 + RFC 合规关闭，从模型上删除这类 bug |
| gorilla/websocket（新项目） | 原仓库 2022 年底归档，后虽被新团队接管但近乎停滞：v1.5.3（2024-06）后无发布，GitHub 最后 push 2025-03，78 个未结 issue；并发写 panic 是该库最高发事故 | coder/websocket（官方指南与社区共识） |
| Node.js / wetty 技术线 | node runtime 依赖直接违反单静态二进制约束；pkg/nexe 打包又大又脆；node-pty 原生模块还要处理 prebuild | Go 或 Rust |
| Rust 的 openssl crate | 链接系统 OpenSSL 破坏静态构建与交叉编译，版本地狱 | rustls（纯 Rust） |
| Rust 裸 tungstenite / `ws` crate | 手写帧处理又退回 libwebsockets 同构风险；`ws` crate 生态已边缘化 | axum 内建 WS（tokio-tungstenite 内核，框架管状态机） |
| ttyd 的 webpack + html-inline 构建链 | webpack 4 时代遗产，配置复杂 | Vite 8 + vite-plugin-singlefile |
| zmodem.js (0.1.10) / decko / execCommand('copy') | zmodem.js 2017 年停更需本地 patch；decko 停更；execCommand 已被浏览器废弃 | PROJECT.md 已将 ZMODEM 划到 v2 且建议 trzsz 覆盖；剪贴板用 navigator.clipboard |
| viper（配置） | 对"读一个 TOML 文件"的需求是航母级依赖 | pelletier/go-toml/v2 直读 |
| libuv/手写事件循环（C 路线的任何残留） | ttyd 单循环承载全部 IO + 每客户端一条 waitpid 线程的教训 | Go runtime 调度器即事件循环；子进程收割用 signal.Notify(SIGCHLD) 统一 reaper |

## Stack Patterns by Variant

**若用户有公网域名且 80/443 可达：**
- 启用 autocert（或 certmagic）进程内 ACME，默认 TLS1.3
- 因为此时浏览器直接访问，可信证书体验最好

**若用户在 NAT/内网/Tailscale 场景（个人运维最常见）：**
- 静态证书文件（自签或 mkcert）或纯 HTTP + Tailscale 加密层
- 文档必须诚实说明：进程内 ACME 在此场景不可用，不要静默失败（ttyd 无此功能，无历史包袱）

**若改选 Rust（决策被推翻时）：**
- tokio + axum + rustls(tokio-rustls) + instant-acme + portable-pty + rust-embed
- 静态构建：`cargo zigbuild --target x86_64-unknown-linux-musl --release`；macOS 在 mac runner 上构建
- 绝不引入 openssl crate

**前端单文件 vs 多资源：**
- v1 用 vite-plugin-singlefile 产出单 HTML（ttyd 同款形态），go:embed 嵌一个文件
- 若 v2 加 Sixel/重资源导致 HTML 膨胀，退回多资源 + embed.FS 目录嵌入 + 预压缩 .gz 旁路文件

## Version Compatibility

| Package A | Compatible With | Notes |
|---|---|---|
| @xterm/xterm 6.0.0 | addon-fit 0.11.0 / addon-webgl 0.19.0 | 三者同 commit（f447274）同批发布，官方配套；v6 删除了 addon-canvas，不要引用 |
| vite-plugin-singlefile 2.3.3 | vite ^5.4.21 \|\| ^6 \|\| ^7 \|\| ^8 | peer 范围显式覆盖 vite 8 ✓ |
| vite 8.2.1 | Node ^20.19.0 \|\| >=22.12.0 | 构建机用 Node 22 LTS 或 24 |
| coder/websocket v1.8.x | Go 1.26 工具链 | 零外部依赖，无版本矩阵问题；生产佐证：Coder 自身产品线 |
| creack/pty v1.1.24 | CGO_ENABLED=0 交叉编译全平台 | 纯 Go + x/sys，无 cgo——这是选它的硬性理由之一 |
| （备选）axum 0.8.9 | tokio 1.x + tokio-rustls 0.26 + rustls 0.23 | 走 Rust 路线时此矩阵必须锁定，tokio-rustls 0.26 对应 rustls 0.23 系列；rustls 0.24 尚在 dev 预览（0.24.0-dev.1），勿追 |

## Sources

> 置信度标注遵循 classify-confidence seam：context7=MEDIUM；websearch 单源=LOW、交叉核实=MEDIUM；webfetch=LOW。下列 registry API（crates.io / proxy.golang.org / registry.npmjs.org / go.dev/dl / api.github.com）seam 无对应 tier——它们是版本数据的一手权威源，且多处经第二来源交叉验证，故相关版本声明按 MEDIUM 呈现。

- crates.io API（portable-pty / axum / tokio / rustls / tokio-rustls / instant-acme / rust-embed / include_dir / pty-process / tokio-tungstenite）— 版本与发布日期核实，2026-08-13
- proxy.golang.org（creack/pty v1.1.24、coder/websocket v1.8.15、gorilla/websocket v1.5.3、certmagic v0.25.4、x/crypto v0.55.0、x/time v0.15.0、go-toml/v2 v2.4.3、pflag v1.0.10）— 版本与发布日期核实
- go.dev/dl?mode=json — 当前稳定 Go 1.26.5 / 1.25.12
- registry.npmjs.org — @xterm/xterm 6.0.0、addon-webgl 0.19.0、addon-fit 0.11.0、vite 8.2.1、vite-plugin-singlefile 2.3.3、typescript 7.0.2
- api.github.com/repos/xtermjs/xterm.js/releases — 6.0.0（2025-12-22）完整变更清单
- api.github.com/repos/gorilla/websocket — archived=false 但最后 push 2025-03-19（"近乎停滞"判定的依据）
- Context7 /tokio-rs/axum — WebSocketUpgrade/WebSocketConfig（max_message_size 默认 64MB、max_frame_size 16MB）文档
- Context7 /rustls/rustls — TLS1.3-only 配置（limitedclient 示例）
- Context7 /djc/instant-acme — RFC 8555 账户/订单/挑战全流程 API
- Context7 /websites/rs_portable-pty — MasterPty/CommandBuilder/阻塞式 reader-writer API
- pkg.go.dev/creack/pty?tab=importedby — 1,263 个导入模块清单
- pkg.go.dev/coder/websocket — SetReadLimit 默认 32KB、Accept/OriginPatterns、Close 语义（MEDIUM）
- websocket.org/guides/languages/go/（2024-09 发布，2026-03 更新）— "新项目用 coder/websocket"推荐与 goroutine-per-connection 模式论述（MEDIUM）
- lib.rs/crates/sshx-server 与 ossatlas.com — sshx 为 Rust+gRPC 中心化架构（LOW，仅用于格局判断）
- 多源中文技术博客（2025-2026）— cargo-zigbuild 为当前 Rust 交叉编译 musl 静态构建主流方案（LOW）

---
*Stack research for: 单静态二进制 Web 终端服务器（wesh / ttyd 重写）*
*Researched: 2026-08-13*
