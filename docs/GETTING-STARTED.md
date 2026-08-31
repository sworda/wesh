<!-- generated-by: gsd-doc-writer -->
# 快速上手

从零开始，在浏览器里看到自己终端的三步路径：装好依赖 → 构建 → loopback 启动。全程不需要管理员权限，也没有额外的系统库依赖（纯 Go 静态构建，前端页面已内嵌进二进制）。

## 前置要求

| 依赖 | 版本要求 | 用途 |
|------|----------|------|
| 操作系统 | Linux 或 macOS | wesh 依赖 PTY 子系统，仅实现 linux/darwin 平台（`internal/pty` 平台文件为 `*_linux.go`/`*_darwin.go`）；Windows 不在支持范围 |
| Git | 任意近期版本 | 克隆仓库 |
| Go | >= 1.26.3（见 `go.mod`） | 构建与运行服务端 |
| Node.js + pnpm | CI 钉版 Node 24 / pnpm 11.21.0（参考值） | **仅在修改 `web/` 前端时需要**；后端开发可跳过 |

## 安装步骤

```sh
git clone https://github.com/sworda/wesh.git
cd wesh
pnpm -C web install   # 前端依赖，仅改前端时需要
```

仓库提交了 `web/dist/index.html` 构建产物（真实终端页，满足 `go:embed` 的编译期要求），裸 clone 不装 pnpm 也能直接 `go build` / `go test ./...` 并得到完整功能的终端；仅在修改 `web/` 前端源码时才需要 `pnpm -C web install && pnpm -C web build`。

## 首次运行

1. **构建**：

   ```sh
   pnpm -C web build && go build -o wesh ./cmd/wesh
   ```

   修改 `web/` 前端源码后必须先 `pnpm -C web build` 再 `go build`，否则二进制内嵌的仍是旧快照（裸 clone 无需此步，dist 已含真实构建产物）。

2. **启动**（loopback 免凭据）：

   ```sh
   ./wesh --bind 127.0.0.1 -- bash
   ```

   > 默认 `--bind 0.0.0.0` 下无凭据会拒绝启动（安全默认值，见下方常见问题第 1 条）。`--bind 127.0.0.1` 流量不出机，免凭据直接可用。

3. **打开浏览器**访问 `listening on` 打印的地址（默认端口 7681），即可看到一个运行 `bash` 的完整交互终端。

启动后输出（stdout；默认只读，终端标题带 `[ro] ` 前缀，浏览器键盘输入不产生效果属预期行为）：

```
listening on http://127.0.0.1:7681
share read-only:  http://127.0.0.1:7681/s/<ro-token>/
```

`Ctrl+C` 关停：SIGTERM/SIGINT 触发优雅下线，子进程随进程组终结。运行期事件以单行 JSON 打到 stderr，启动行/分享链接行保持人读文本打到 stdout。

## 常见问题

**1. `wesh -- bash` 拒绝启动（exit 2）**

报错形如 `refusing to listen on non-loopback address without credentials; pass --no-auth to disable authentication`。这是刻意的安全默认值：默认 `--bind 0.0.0.0` 下无凭据不允许裸奔。三选一：

```sh
./wesh --bind 127.0.0.1 -- bash          # 本机使用（推荐）
./wesh --credential user:pass -- bash    # 配置凭据对外分享
./wesh --no-auth -- bash                 # 显式声明「我知道我在裸奔」
```

**2. 修改了 `web/` 前端源码，但页面没变化**

`go:embed` 在编译期把 `web/dist/` 快照进二进制。改前端后必须重新执行 `pnpm -C web build` 再 `go build`，否则二进制内嵌的仍是旧产物。

**3. `go test -race` 报 cgo 相关错误**

`-race` 竞态检测器需要 cgo。测试环境不要设置 `CGO_ENABLED=0`（该变量只属于发布构建场景，CI 的 go 测试通道显式不设它）。

**4. 端口 7681 被占用**

`--port 0` 让内核分配随机端口，启动时打印实际端口：

```sh
./wesh --bind 127.0.0.1 --port 0 -- bash
```

**5. 剪贴板（选中复制 / Ctrl+Shift+V 粘贴）不生效**

浏览器剪贴板 API 仅在 HTTPS 或 localhost 访问时可用。明文 HTTP 非 localhost 部署下，选中复制与粘贴静默失效（终端其余功能不受影响）。详见 [CONFIGURATION.md](CONFIGURATION.md)。

**6. 在 Windows 上无法构建**

报错来自 `internal/pty` 缺少平台实现——Windows 终局不在支持范围。请在 Linux、macOS 或 WSL2 环境中构建运行。

## 下一步

- [CONFIGURATION.md](CONFIGURATION.md) —— 全部 CLI flag、TOML 配置文件与环境变量（含认证/TLS/分享链接语义）
- [DEPLOYMENT.md](DEPLOYMENT.md) —— 反向代理（nginx/Caddy）、systemd、Docker、UNIX socket 生产部署
- [DEVELOPMENT.md](DEVELOPMENT.md) —— 本地开发环境、构建流程与发布流程
- [TESTING.md](TESTING.md) —— 测试框架、运行方式与 CI 集成
- [ARCHITECTURE.md](ARCHITECTURE.md) —— 系统架构、组件与 wesh.v1 协议
- [CONTRIBUTING.md](../CONTRIBUTING.md) —— 贡献指南
