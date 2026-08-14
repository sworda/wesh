# wesh

`wesh` — 通过 Web 分享终端的命令行工具：`wesh [flags] -- <cmd> [args...]` 启动后在指定端口提供 HTTP/WebSocket 服务，浏览器打开页面即获得一个运行 `<cmd>` 的完整交互终端。

> ⚠️ **Phase 1 无认证：任何能访问监听地址的人都获得一个以你身份运行的 shell。仅在可信网络使用。**
>
> 当前默认 `--bind 0.0.0.0`（LAN 可达）。认证、TLS、Origin 白名单将在 Phase 3 到位；在此之前不要把 wesh 暴露到公网或不可信网络。

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
| `--version` | — | 打印版本并退出 |
| `--help` | — | 打印用法 |

启动后仅打印单行：`listening on http://host:port`（无 banner、无 emoji）。浏览器打开该地址即进入终端；Phase 1 同时只允许一个客户端连接，第二个连接会收到 409。

## 构建

构建顺序是硬依赖：**前端构建必须先于 go build**（`go:embed all:dist` 编译期要求 `web/dist/` 存在）。

```sh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

裸 clone 仓库中只有 `web/dist/index.html` 占位文件——可直接 `go test ./...`（编译与测试不依赖真实前端产物），但**运行前必须先构建前端**，否则浏览器只能看到占位页。

## 安全说明

**env 白名单（SEC-06）**：子进程只能看到以下环境变量，服务端其余环境变量一律不透传：

- 固定注入：`TERM=xterm-256color`、`COLORTERM=truecolor`
- 按名继承：`PATH`、`HOME`、`USER`、`LOGNAME`、`SHELL`
- 按前缀继承：`LANG`、`LC_*`

在 web shell 里执行 `env` 不应看到任何服务端机密变量。

**协议帧形状**（WebSocket 二进制帧）：1 字节类型 + 载荷。

| 类型字节 | 含义 | 载荷 |
|----------|------|------|
| `'0'` | INPUT（C→S）/ OUTPUT（S→C） | 原始字节 |
| `'1'` | RESIZE（C→S） | JSON `{"cols":N,"rows":N}`，钳制 [1,1000] |

未知类型一律以 1002 关闭连接；线上关闭码只出现 1000/1002/1009（1006 永不写入）。

## 测试

```sh
go test -race -count=1 ./...
```

CI 为双平台矩阵（ubuntu + macos，含 macOS kqueue 收割验证）加独立 web 构建 job。注意 `-race` 需要 CGO，测试环境不要设 `CGO_ENABLED=0`。
