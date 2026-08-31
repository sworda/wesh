<!-- generated-by: gsd-doc-writer -->

# wesh 架构

## 系统概览

wesh 是一个通过 Web 分享终端的命令行工具：`wesh [flags] -- <cmd> [args...]` 启动后在指定端口提供 HTTP/WebSocket 服务，浏览器打开页面即获得一个运行 `<cmd>` 的完整交互终端。整体为**单二进制分层架构**——Go 服务端（CLI 解析/装配层 → HTTP+WS 网关层 → PTY 数据面层）内嵌经 Vite 单文件构建的 xterm.js 前端（`go:embed`），采用 **GoTTY 式共享进程模型**：PTY 子进程随服务端启动时 spawn 一次，多个浏览器客户端共享同一会话（输出实时扇出、写权限经 owner 递补仲裁），这与 ttyd 的 per-connection spawn 模型是核心架构分叉。主要输入是浏览器键盘/粘贴字节与 RESIZE 事件，主要输出是 PTY 子进程的字节流（扇出 ×N 客户端）；wire 层为自定义二进制 WebSocket 协议 `wesh.v1`（1 字节帧类型 + 载荷）。支持平台仅 linux/darwin（amd64/arm64），Windows 不在支持范围（PTY 层构建标签限定）。

## 组件图

```mermaid
graph TD
    FE[浏览器前端<br/>web/src/main.ts · xterm.js]

    subgraph cmd/wesh（CLI 装配层）
        CLI[flag + TOML 解析<br/>启动校验矩阵 · TLS 预检]
    end

    subgraph internal/server（网关层）
        HTTP[mux 路由 + 认证链<br/>basicAuth · throttle · origin · 安全头]
        ATTACH[WS 握手状态机<br/>server.Attach · Hello/ticket 核销]
        HUB[注册表 + fan-out hub<br/>信用门 · 模式判定矩阵]
        CLIENTS[每客户端 outbox + writer<br/>512KiB 有界队列]
        INPUTQ[输入限速 + 会话输入队列<br/>inputQ + 单 input-writer]
        ARB[resize 仲裁器<br/>min-rect · 50ms 防抖]
        OBS[可观测性<br/>/healthz · /metrics · slog JSON]
    end

    subgraph internal/pty（数据面）
        PTY[pty.Session<br/>master 读写 · 信号 · 平台收割]
    end

    subgraph web（前端装配）
        EMBED[go:embed 静态伺服<br/>gzip 预压 · 自定义首页装饰]
    end

    CHILD[子进程 &lt;cmd&gt;]

    CLI -->|spawn| PTY
    CLI -->|Options 装配| HTTP
    FE -->|GET / · /s/{token}/| EMBED
    FE -->|POST /api/attach 换 ticket| HTTP
    FE -->|WS /ws（wesh.v1）| ATTACH
    HTTP --> ATTACH
    ATTACH -->|注册| HUB
    PTY -->|ReadLoop 32KiB chunk| HUB
    HUB -->|'0' OUTPUT 扇出| CLIENTS
    CLIENTS -->|WS 下行| FE
    FE -->|'0' INPUT / '1' RESIZE| ATTACH
    ATTACH --> INPUTQ
    ATTACH --> ARB
    INPUTQ -->|独占 Master.Write| PTY
    ARB -->|TIOCSWINSZ| PTY
    PTY --- CHILD
    HTTP --- OBS
```

HTTP 路由面（`server.Handler()` 装配，`--base-path` 时统一加前缀）：`GET /` 与 `/s/{token}/`（内嵌页/分享链接）、`POST /api/attach`（换一次性 ticket）、`/ws`（WS 升级）、`GET /healthz`（免认证探活，根路径固定不受 base-path 影响）、`GET /metrics`（Prometheus 文本，跟随认证闸）。

## 数据流

**连接建立（浏览器 → 服务端）**

1. 浏览器 `GET /` 取内嵌单页（`web/embed.go`：`go:embed all:dist`，gzip 预压旁路 + `Vary: Accept-Encoding`）；`--index` 自定义首页时整页替换伺服字节。
2. 认证模式下前端 `POST /api/attach`（Basic 凭据经 401 challenge 获取）换一次性 ticket（128bit `crypto/rand`、单次使用、60s TTL、绑定 ro/rw 模式）；无认证模式该端点 404，前端据此跳过取票直连 WS。
3. WS 连接 `/ws` 必须协商子协议 `wesh.v1`，握手守卫链（`server.Attach`）：Origin 白名单检查（403）→ 子协议预检（400）→ per-IP 半开上限 8（429）→ `--max-clients` 满员 503 闸 → Accept。
4. 首帧必须为 Hello `{"version","cols","rows","ticket"?}`——5s 超时、预认证读上限 4KiB；ticket 核销失败统一 `auth_failed` + 1008（无区分 oracle）。
5. 核销通过后进入模式判定矩阵（ticket 绑定 mode × `--writable` × `--write-policy` × owner 在位 → 生效 ro/rw），注册进客户端表，回 Welcome `{"mode","cols","rows","prefs"?}`（cols/rows 为会话尺寸恒在）；稳态读上限切 16KiB，pinger 按 `--ping-interval`（默认 5s）保活、pong 超时 10s 断开。

**输出路径（PTY → 浏览器）**

1. `pty.Session.ReadLoop`（`internal/pty/io.go`）以 32KiB 缓冲循环读 master，每 chunk 回调 `onChunk`。
2. `onChunk`（`clients.go`）持 hubMu：先过全局信用门（全体可写端 outbox 均满 → 持块停读 PTY，形成反压），再组帧 `'0' + chunk` 逐客户端 `outbox.trySend`。
3. 每客户端 outbox 为 512KiB 有界队列，独立 writer goroutine drain 批量写 WS——慢客户端不拖累他人。
4. 写满处理：ro 端立即 1013 踢出；rw 端在 attach 宽限（500ms）外且存在健康可写端时踢出，否则置信用保护位（owner 演示者不被误踢），恢复至 50% 水位后重投暂存帧并开门。

**输入路径（浏览器 → PTY）**

1. INPUT 帧经各客户端读循环：先过 per-client mode 门（ro 端输入服务端静默丢弃——只读是服务端边界），再过限速器（持续 32KiB/s、burst 64KiB，超限静默丢弃）。
2. 载荷入会话级有界 `inputQ`（256KiB），由**单 input-writer goroutine** 独占 `Master.Write`——读循环零同步写。

**RESIZE 路径**

RESIZE 帧 `{"cols","rows"}` 经钳制 [1,1000] 后进仲裁器（`resize.go`）：≥2 端取参与集最小公共矩形（参与集按写权限分层：owner 模式仅 owner、all 模式全部 rw 端、纯 ro 会话全部 ro 端 Hello 首尺寸），50ms 防抖后 TIOCSWINSZ 落盘，会话尺寸变化经 `'W'` 帧再推送全端约束视口。

**终结路径**

- 子进程退出（唯一终结路径）：lifecycle goroutine 广播 EXIT `'X'` 帧（`{"exit_code","message"}`，信号死亡 exit_code=-1）→ 以 1000 关闭全部客户端 → `exitf` 按子进程退出码退出。
- SIGTERM/SIGINT 优雅下线：向全部客户端发 1001（close reason `server_shutting_down`，前端终态面板不自动重连）→ 对子进程进程组执行 stop-signal 序列（`--stop-signal` → `--stop-timeout` 宽限 → SIGKILL）。
- `--once` / `--exit-when-empty` 空触发：向子进程进程组发 SIGHUP，wesh 退出状态 255。

## 关键抽象

| 抽象 | 位置 | 说明 |
|------|------|------|
| `pty.Session` | `internal/pty/spawn.go` | 子进程 + PTY master 封装：`Start`（exec 数组不经 shell、env 白名单替换式注入、降权 Credential）、`ReadLoop`、`Resize`、`SignalGroup`（负 pid 进程组信号） |
| `proto` 包 | `internal/proto/proto.go` | wire 协议单一事实源：帧类型常量（`'H'`/`'W'`/`'E'`/`'0'`/`'1'`/`'X'`）、payload 结构、`Subprotocol = "wesh.v1"`、两档读上限、尺寸钳制；前端 `main.ts` 帧常量与之手工对齐 |
| `server.Server` | `internal/server/server.go` | 网关与会话生命周期收口：`New` 装配（钉死 ReadLoop/inputWriter/lifecycle 三个 goroutine）、`Handler()` 路由树、`Attach` WS 握手状态机、`Shutdown` 优雅下线 |
| `server.Options` | `internal/server/server.go` | 装配期固化配置（写权限/保活/认证/容量/背压/部署形态），零值兜底集中在 `New` |
| `client` / `outbox` / `registry` | `internal/server/clients.go` | 多客户端 hub：每客户端有界 outbox + 独立 writer；注册表 + 全局信用门（`hubCond`）构成 fan-out 背压；owner FIFO 递补升格 |
| `inputQ` + input-writer | `internal/server/clients.go` | 会话级有界输入队列 + 单写者 goroutine 独占 master 写（读循环零同步写） |
| `arbiter` | `internal/server/resize.go` | resize 仲裁：纯函数 min-rect/last-wins + 参与集分层 + 防抖/即时重算双通道，全部字段 hubMu 保护 |
| `ticketStore` / `throttleStore` / `shareTokens` | `internal/server/tickets.go` / `throttle.go` / `sharetoken.go` | 认证三原语：一次性 ticket（60s TTL）、per-IP 指数退避（1s 起翻倍封顶 30s）、ro/rw 分享 token（128bit，SHA-256 预哈希存储，重启即废） |
| `web.Handler()` / `WithCustomIndex` | `web/embed.go` | go:embed 静态伺服：gzip 预压旁路、`Vary` 头纪律、`--index` 自定义首页 byte-identity 整页替换装饰 |
| 前端 `main.ts` + `lib/` | `web/src/` | xterm.js 终端装配（fit/webgl/unicode11/web-links/clipboard 五 addon）、wesh.v1 协议帧收发、偏好三级覆盖（URL query > `--client-option` > 内置默认）、异常断开自动重连（1s 起指数退避封顶 30s）、标题同步/超链接/剪贴板 |

**并发纪律**：单锁 `hubMu` 护注册表/信用门/仲裁器（锁序 `hubMu > outbox.mu`，绝不反序同持）；`exitf` 由 main 注入 `os.Exit`、测试注入捕获桩；运行期事件恒经 slog JSONHandler 单出口写 stderr，凭据/ticket/share token 任何形态永不入日志。

## 目录结构

```
wesh/
├── cmd/wesh/          # CLI 入口：flag + TOML 解析（config.go）、启动校验矩阵、
│                      #   TLS 证书预检、自定义首页读入、分享链接生成、信号处理
├── internal/          # 服务端实现（Go internal 包，不可外部导入）
│   ├── pty/           # PTY 数据面：spawn（exec 数组 + env 白名单 + 降权）、
│   │                  #   master 读写、平台收割（reap_linux pidfd / reap_darwin kqueue）、
│   │                  #   进程组信号（signal_linux / signal_darwin 构建标签分支）
│   ├── proto/         # wire 协议单一事实源：帧类型、payload、子协议、读上限
│   └── server/        # HTTP+WS 网关：握手状态机、多客户端注册表与扇出/背压、
│                      #   认证/节流/分享 token、resize 仲裁、健康检查与指标、
│                      #   slog JSON 审计日志、TLS/安全头/Origin/反代信任
├── web/               # 前端：src/main.ts + src/lib/（TypeScript + xterm.js 6），
│   │                  #   vite-plugin-singlefile 构建为单 HTML 进 dist/，
│   │                  #   embed.go 以 go:embed 嵌入二进制；uat/ 为协议层 UAT 脚本
├── deploy/            # systemd unit 模板（wesh.service）
├── scripts/           # release.sh 发布脚本（前置校验 → 测试 → 构建 → fuzz → tag）
├── docs/              # 项目文档（ARCHITECTURE.md、CONFIGURATION.md 等）
├── Dockerfile         # 参考镜像（FROM scratch + 静态二进制 + tini，用户自建）
└── README.md          # 项目概览：安装、快速开始、使用示例、文档索引
```

**组织原理**：`internal/` 三包按数据面职责切分——`pty` 只管子进程与 master 字节（与会话控制面解耦）、`proto` 是前后端共享的协议常量唯一来源（防双写漂移）、`server` 承载全部控制面与多客户端逻辑；`web` 是独立 pnpm 包，构建产物 `dist/` 经 `go:embed` 进入二进制（前端构建必须先于 `go build`），使部署形态收敛为"scp 单文件即用"。外部依赖刻意极简：`coder/websocket`（WS 实现）、`creack/pty`（PTY 封装）、`golang.org/x/sys`（平台 syscall 封装——TIOCGPGRP/SIGWINCH/kqueue）、`golang.org/x/time`（限速）、`pelletier/go-toml/v2`（配置解析）+ stdlib（Prometheus 文本指标、slog、net/http mux 均手写/内建，零重框架）。
