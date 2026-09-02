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

`--session-mode=shared|per-client` 选择会话模式（默认 `shared`；`per-client` 行为装配中，当前版本与 `shared` 等价）。

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
