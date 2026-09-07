<!-- generated-by: gsd-doc-writer -->
# wesh

[![CI](https://github.com/sworda/wesh/actions/workflows/ci.yml/badge.svg)](https://github.com/sworda/wesh/actions/workflows/ci.yml)
[![Release](https://github.com/sworda/wesh/actions/workflows/release.yml/badge.svg)](https://github.com/sworda/wesh/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/github/license/sworda/wesh)](LICENSE)

`wesh` 是通过 Web 分享终端的单二进制命令行工具：`wesh [flags] -- <cmd> [args...]` 启动后在指定端口提供 HTTP/WebSocket 服务，浏览器打开页面即获得一个运行 `<cmd>` 的完整交互终端。`--` 之后的命令及参数原样以 exec 数组传递，不经 shell。

```
wesh [flags] -- <cmd> [args...]
```

核心特性：

- **单二进制部署**：前端页面经 `go:embed` 内嵌进二进制，scp 一个文件即用
- **默认只读**：不带 `--writable` 时浏览器输入被服务端丢弃，旁观零风险
- **多客户端共享同一会话**：ro/rw 两条分享链接复制即用，慢客户端保护性踢出，异常断线自动重连
- **安全默认值**：Basic 认证 + 一次性 ticket、TLS、Origin 白名单、认证失败节流、子进程环境变量白名单
- **生产可运维**：`/healthz` 探活、`/metrics` Prometheus 指标、JSON 结构化审计日志、优雅下线
- **部署形态齐全**：TOML 配置文件、UNIX socket、反代子路径挂载、systemd/Docker 参考配方

完整 flag 列表与配置说明见 [docs/CONFIGURATION.md](docs/CONFIGURATION.md)。

## 安装

### 预编译二进制（推荐）

发布物为 linux/darwin × amd64/arm64 四平台 tar.gz（Windows 不在支持范围），每个压缩包内含 `wesh` + `LICENSE` + `README.md` 三件套，解压即用：

```sh
curl -LO https://github.com/sworda/wesh/releases/download/v1.0.0/wesh_v1.0.0_linux_amd64.tar.gz
tar xzf wesh_v1.0.0_linux_amd64.tar.gz
./wesh --version
```

全部版本与产物清单见 [Releases](https://github.com/sworda/wesh/releases)；下载后建议以 `checksums.txt` 核对完整性：

```sh
sha256sum -c checksums.txt --ignore-missing   # 只验本机已下载的产物
```

### 源码构建

前置：Go >= 1.26.3、Node.js 与 pnpm（CI 钉版 Node 24 / pnpm 11.21.0）。

```sh
git clone https://github.com/sworda/wesh.git
cd wesh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

修改 `web/` 前端源码后，**前端构建必须先于 `go build`**（`go:embed all:dist` 编译期要求 `web/dist/` 存在，内嵌的是构建时快照）。仓库提交了 `web/dist/index.html` 构建产物（真实终端页），裸 clone 可直接 `go build` / `go test ./...`；但修改 `web/` 前端源码后必须重新 `pnpm -C web build` 再 `go build`，否则二进制内嵌的仍是旧产物。

## 快速开始

```sh
./wesh --bind 127.0.0.1 -- bash
```

> 为什么加 `--bind 127.0.0.1`：默认 `--bind 0.0.0.0` 下无凭据会**拒绝启动**（安全默认值，见下文）。loopback 监听流量不出机，免凭据直接可用。

启动后输出（默认端口 7681；`--writable` 时多一行 read-write 分享链接）：

```
listening on http://127.0.0.1:7681
share read-only:  http://127.0.0.1:7681/s/<ro-token>/
```

1. 浏览器打开 `listening on` 地址 → 进入终端（默认只读旁观，终端标题带 `[ro] ` 前缀）
2. 把 `share read-only` 链接发给同事 → 对方打开即实时旁观同一会话
3. `Ctrl+C` 关停：SIGTERM/SIGINT 触发优雅下线，子进程随进程组终结

## 使用示例

**本机只读旁观**（最短路径，见快速开始）：

```sh
./wesh --bind 127.0.0.1 -- bash
```

**公网分享**（凭据 + TLS；浏览器打开后弹 Basic 登录框）：

```sh
./wesh --credential alice:密码 --tls-cert cert.pem --tls-key key.pem -- bash
```

生产环境凭据建议用 `WESH_CREDENTIAL` 环境变量而非 flag（flag 值对同机用户可见于 `ps`）。

**可写协作**（全员可写；默认 `owner` 策略为首个可写客户端独占写权限、断开后按 attach 顺序递补）：

```sh
./wesh --writable --write-policy all --credential alice:密码 --tls-cert cert.pem --tls-key key.pem -- bash
```

## 会话模式

`--session-mode=shared|per-client` 选择会话模式（默认 `shared`）。`--stop-timeout` 默认值按模式分岔：`shared` 默认 `0`（不补发 SIGKILL，子进程继续运行）；`per-client` 未显式设置默认 `5s`（客户端断开后 SIGKILL 兜底回收 SIGHUP 免疫进程），显式 `0` 尊重用户意图但启动时警告泄漏风险。

| 模式 | 语义 | 选择方式 |
|------|------|----------|
| `shared`（默认） | 多人同屏共享同一 PTY 进程：输出实时扇出 ×N 客户端、写权限经 owner 仲裁与递补——wesh 的差异化本体 | `--session-mode=shared` 或 TOML `session-mode = "shared"`（缺省即此） |
| `per-client` | 每客户端独立 PTY 进程（ttyd 式 per-connection 生命周期）：断开即终结、重连即全新进程、尺寸直通无仲裁 | `--session-mode=per-client` 或 TOML `session-mode = "per-client"` |

`per-client` 下三个与 `shared` 直觉不同的语义：

- **分享链接 = 按权限级别的独立进程入场券**：ro/rw 链接不再指向同一会话视图——子进程为普通 shell 时，每位开链接者得到互不可见的私有 shell；ro/rw 权限级别仍由 ticket 绑定（机制零改动）。
- **ro = 对自有进程的输入门控**：只读访客同样获得独立进程，wesh 在服务端丢弃其键盘输入（只读是服务端边界，两种模式一致）；「围观同一会话」的体验经子程序间接保留（见下条）。
- **配合 herdr/tmux 时经多路复用汇聚**：每位客户端的独立 herdr/tmux 客户端进程连接同一个多路复用器会话——分享体验保留，且各客户端按自身终端几何独立渲染，移动端 attach 不再压缩桌面端。

**herdr 配方**（每客户端独立进程 + 经 herdr server 汇聚同一会话；argv 形态与仓库 UAT `web/uat/phase14.mjs` 的被测物逐字一致——文档即被测物）：

```sh
wesh --writable --session-mode=per-client -- herdr --session <name>
```

效果：herdr 以最后活动客户端（is_foreground）仲裁前台几何并按 per-client area 渲染——移动端 attach 后布局翻紧凑，桌面端一有键入即恢复全量布局，桌面端面板不再被压缩。

tmux 对照：多个客户端 attach 同一 tmux 会话时，默认 window-size=latest 下小屏客户端活动会使窗口收缩，桌面端观感被压缩；window-size 可配但均为单窗口尺寸语义，不存在 per-client 独立渲染——需要多人异尺寸互不压缩时，用上面的 per-client + herdr 配方。

### per-client 资源义务与实测标定

per-client 下客户端数 == 子进程数：`--max-clients`（默认 `32`）兼任并发进程上限（握手 503 闸 + spawn 前计数复检，并发子进程数恒 ≤ max-clients）。**32 个并发 shell 是重负载**——默认值 32 经负载矩阵实测可承载（下表数据），保持不变。

实测标定（2026-09-06，`go test -tags=load` 负载矩阵，linux/amd64；「验证为主、证伪才改」——全部实测值在账面界线内，零默认值改动）：

**驻留剖面**（N 会话 bash idle 空转，双采样差值）：

| 会话数 N | wesh 侧内存增量 | goroutine 增量 | fd 增量 | 子进程 VmRSS 合计 |
|---|---|---|---|---|
| 1 | 81KiB | 6 | 4 | 3.6MiB |
| 4 | 235KiB | 21 | 16 | 14.4MiB |
| 16 | 959KiB | 81 | 64 | 57.2MiB |
| 32 | 1.9MiB | 161 | 128 | 114.7MiB |

**洪水剖面**（每会话独立 seq 洪水 33.3MiB/端，全端收流完整、零误踢）：

| 会话数 N | wesh 侧 Alloc 峰值 | 全端收流完成 | 每端吞吐（推算） |
|---|---|---|---|
| 1 | 2.7MiB | 3.3s | ≈10.0MiB/s |
| 4 | 2.2MiB | 4.6s | ≈7.2MiB/s |
| 16 | 15.2MiB | 6.7s | ≈5.0MiB/s |
| 32 | 55.0MiB | 8.8s | ≈3.8MiB/s |

标定口径：内存/goroutine/fd 为 wesh 服务端侧双采样差值，VmRSS 为子进程 `/proc/<pid>/status` 实测合计；goroutine 实测增量 5N+1（UAT 关闭保活形态，生产 `--ping-interval=5s` 下账面 6N）；吞吐列 = 实测每端字节 ÷ 实测完成时长（推算值）。

按部署形态的 `--max-clients` 建议值（默认 32 不动；建议值是资源画像分档，非硬性门槛）：

| 部署形态 | 建议 `--max-clients` | 依据 |
|---|---|---|
| 常规服务器（默认） | 32——实测可承载，不改动 | 32 会话实测：wesh 侧驻留增量 ~1.9MiB、洪水 Alloc 峰值 55MiB、子进程合计 ~115MiB（bash） |
| 低配 VPS / 内存受限容器 | 8 | 按实测每会话 bash 子进程 ~3.7MiB + wesh 侧 ~61KiB 折算；真实成本取决于 `<cmd>` 本体（编辑器/构建工具远高于 bash），受限环境保守分档 |
| 个人多端（herdr/tmux 桌面 + 手机汇聚） | 4 | 个人多端典型 1-4 个客户端；小上限收紧误分享链接时的进程放大面 |

**保活先杀时序**（默认 `--ping-interval=5s`）：TCP 级完全停止读取的连接（不回 pong）会在「停止读取后 5s~10s」窗口内被 pong 超时以 1006 关闭——比慢客户端看门狗（1013）更早收口。真实浏览器结构性不会触发（网络栈自动回 pong，不受页面节流影响）；自管 WebSocket socket 的客户端（如 herdr 类）若停读则适用——不回 pong 的连接本就是死连接，1006 更早回收是正确行为。1006 会触发前端自动重连；`per-client` 模式下重连即获得全新进程，这正是真死连接场景的合理恢复路径。

## 安全默认值

`wesh` 提供的是一个以你身份运行的 shell，默认配置拒绝裸奔：

- **启动校验**：默认 `--bind 0.0.0.0` 下无凭据拒绝启动（需显式 `--no-auth` 逃生门）；非 loopback + 凭据 + 明文 HTTP 拒绝启动（需 `--insecure-http` 或配置 TLS）。`--bind 127.0.0.1` 本机裸跑不受限。
- **默认只读**：只读是服务端边界——不带 `--writable` 时浏览器键盘输入与裸 WS 客户端的 INPUT 帧均被丢弃。
- **分享链接重启即废**：ro/rw token 每次启动重新随机生成，怀疑泄露时重启即吊销全部旧链接。
- **子进程环境白名单**：子进程只能看到 `TERM`/`COLORTERM`（固定注入）与 `PATH`/`HOME`/`USER`/`LOGNAME`/`SHELL`/`LANG`/`LC_*`（按名/前缀继承），服务端其余环境变量一律不透传。

## 文档

| 文档 | 内容 |
|------|------|
| [GETTING-STARTED.md](docs/GETTING-STARTED.md) | 前置要求、安装步骤与首次运行 |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | 系统架构、组件与数据流 |
| [CONFIGURATION.md](docs/CONFIGURATION.md) | 全部 CLI flag、TOML 配置文件与环境变量 |
| [DEPLOYMENT.md](docs/DEPLOYMENT.md) | 反向代理（nginx/Caddy）、systemd、Docker、UNIX socket 部署 |
| [DEVELOPMENT.md](docs/DEVELOPMENT.md) | 本地开发环境与构建流程 |
| [TESTING.md](docs/TESTING.md) | 测试框架与运行方式 |

## 贡献

见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

[MIT](LICENSE) © 2026 sworda
