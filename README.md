# wesh

`wesh` — 通过 Web 分享终端的命令行工具：`wesh [flags] -- <cmd> [args...]` 启动后在指定端口提供 HTTP/WebSocket 服务，浏览器打开页面即获得一个运行 `<cmd>` 的完整交互终端。

> ⚠️ **无认证：任何能访问监听地址的人都获得一个以你身份运行的 shell。仅在可信网络使用。**
>
> 当前默认 `--bind 0.0.0.0`（LAN 可达）。Phase 2 协议基线（子协议握手/消息上限/合规关闭码）已就位**不等于**可公网暴露——认证、TLS、Origin 白名单在 Phase 3 到位；在此之前不要把 wesh 暴露到公网或不可信网络。

## 单次语义（Phase 1）

Phase 1 为单次语义：**子进程退出或 WS 断开，服务端即整体退出**（退出码 = 子进程退出码）。断线重连与 `--once` 等完整生命周期语义在后续阶段（Phase 6）提供——**WS 断开即退出是当前阶段的预期行为，不是 bug**。

## 用法

```
wesh [flags] -- <cmd> [args...]
```

`--` 之后的命令及参数原样传递（exec 数组形式，不经 shell）。

| Flag | 默认值 | 说明 |
|------|--------|------|
| `--port` | `7681` | 监听端口；`0` = 随机端口，启动时打印实际端口 |
| `--bind` | `0.0.0.0` | 监听地址 |
| `--writable` | `false` | 只读模式：客户端输入被丢弃；开启后客户端输入写入终端 |
| `--ping-interval` | `5s` | WS ping 保活间隔（防反代空闲超时断连）；`0` = 禁用 |
| `--version` | — | 打印版本并退出 |
| `--help` | — | 打印用法 |

启动后仅打印单行：`listening on http://host:port`（无 banner、无 emoji）。浏览器打开该地址即进入终端；Phase 1 同时只允许一个客户端连接，第二个连接会收到 409。

## 构建

构建顺序是硬依赖：**前端构建必须先于 go build**（`go:embed all:dist` 编译期要求 `web/dist/` 存在）。

```sh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

仓库提交了前端构建产物（`web/dist/index.html` 及其 `.gz`，由 `go:embed` 嵌入二进制）——裸 clone 即可直接 `go build` / `go test ./...` 并运行。**修改 `web/` 前端源码后必须先重新 `pnpm -C web build` 再 `go build`**，否则二进制内嵌的仍是旧产物。

## 安全说明

**env 白名单（SEC-06）**：子进程只能看到以下环境变量，服务端其余环境变量一律不透传：

- 固定注入：`TERM=xterm-256color`、`COLORTERM=truecolor`
- 按名继承：`PATH`、`HOME`、`USER`、`LOGNAME`、`SHELL`
- 按前缀继承：`LANG`、`LC_*`

在 web shell 里执行 `env` 不应看到任何服务端机密变量。

**协议（wesh.v1）**：WebSocket 连接必须协商子协议 `wesh.v1`（缺失或不含该值的请求在升级前以 HTTP 400 拒绝）。建连后客户端首帧必须是 Hello `{"version":"wesh.v1","cols":N,"rows":N}`——5s 内未收到合法 Hello 以 1008 关闭，抢跑（Hello 前的数据帧）或畸形帧以 1002 关闭。服务端握手成功回 Welcome `{"mode":"ro"|"rw"}`。所有帧为 WebSocket 二进制帧：1 字节类型 + 载荷。

| 类型字节 | 含义 | 载荷 |
|----------|------|------|
| `'H'` | Hello（C→S，必须为首帧） | JSON `{"version":"wesh.v1","cols":N,"rows":N}` |
| `'W'` | Welcome（S→C，握手成功） | JSON `{"mode":"ro"\|"rw"}` |
| `'E'` | Error（S→C） | JSON `{"code":"...","message":"..."}` |
| `'0'` | INPUT（C→S）/ OUTPUT（S→C） | 原始字节 |
| `'1'` | RESIZE（C→S） | JSON `{"cols":N,"rows":N}`，钳制 [1,1000] |

Error 帧只含两个正常客户端可见码：`version_mismatch`（随后以 1008 关闭）与 `server_error`（随后以 1011 关闭）；攻击面路径（未知/抢跑/畸形帧、超限）直接关闭连接、不发 Error 帧——不给攻击者反馈面。

关闭码全集：

| 关闭码 | 含义 |
|--------|------|
| 1000 | 正常关闭 |
| 1002 | 协议错误（未知帧/抢跑/畸形） |
| 1008 | 策略违反（Hello 超时/版本不符） |
| 1009 | 超出消息上限 |
| 1011 | 内部错误 |
| 1001 / 1013 | 已在协议占住（服务端下线 / 踢出可重试），由后续阶段启用发送路径 |

1005/1006/1015 永不发送。

消息上限（C→S）：握手完成前（预认证窗口）4KiB，握手完成后稳态 16KiB——单帧与单消息累积字节同顶，超限由 WS 库自动以 1009 关闭并在服务端 stderr 打单行事件。保活：握手完成后服务端按 `--ping-interval`（默认 5s）发 WS ping，pong 超时 10s 主动断开连接；`0` = 禁用。

**默认只读**：不带 `--writable` 时浏览器键盘不产生输入（终端标题带 `[ro] ` 前缀），裸 WS 客户端发来的 INPUT 帧同样被服务端静默丢弃——只读是服务端边界，不只是前端行为；RESIZE 在只读下照常生效。`--writable` 开启后 Welcome 带 `mode=rw`，客户端输入写入终端。

**部署注意**：per-IP 半开连接上限（默认 8，超限 HTTP 429）在**直连部署**下有效；置于反向代理之后时所有客户端聚合为代理 IP，该限制可能误伤正常用户——可信头（X-Forwarded-For）透传属后续阶段（SEC-07）。

## 测试

```sh
go test -race -count=1 ./...
```

CI 为双平台矩阵（ubuntu + macos，含 macOS kqueue 收割验证）加独立 web 构建 job。注意 `-race` 需要 CGO，测试环境不要设 `CGO_ENABLED=0`。
