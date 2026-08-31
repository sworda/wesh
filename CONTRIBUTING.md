<!-- generated-by: gsd-doc-writer -->
# 贡献指南

感谢关注 wesh！本文说明开发约定、CI 门禁与协作流程。上手请先读：

- [docs/GETTING-STARTED.md](docs/GETTING-STARTED.md) —— 前置要求、安装与首次运行
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) —— 本地开发环境与构建流程
- [docs/TESTING.md](docs/TESTING.md) —— 测试分层与运行方式

仓库地址：[github.com/sworda/wesh](https://github.com/sworda/wesh)。

## 平台边界

wesh 只支持 **linux/darwin（amd64/arm64）**，Windows 不在支持范围——`internal/pty` 以 `//go:build linux` / `//go:build darwin` 构建标签限定，服务端无法在 Windows 上构建运行。后端改动请在 Linux 或 macOS 上开发验证；纯前端（`web/`）与文档改动不受此限。

## 开发环境

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | >= 1.26.3 | 以 `go.mod` 为准，CI 按其钉版 |
| Node.js | 24 | CI 钉版 |
| pnpm | 11.21.0 | CI 钉版（`web/package.json` 无 `packageManager` 字段，须显式对齐） |

关键构建纪律——**前端构建必须先于 `go build`**：

```sh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

`web/embed.go` 的 `//go:embed all:dist` 在编译期要求 `web/dist/` 存在。仓库提交了 `web/dist/index.html` 构建产物（真实终端页），裸 clone 可直接编译与跑测试；但修改 `web/` 前端源码后必须重新 `pnpm -C web build` 再 `go build`，否则二进制内嵌的仍是旧产物。详见 [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)。

## 代码规范

- **Go**：标准 `gofmt` 格式；CI 以 `go vet ./...` + `-race` 全量测试为门禁，无额外 lint 工具链。
- **前端**：TypeScript 类型检查内嵌在构建里（`pnpm -C web build` 为 `tsc && vite build && gzip` 一体脚本），CI web job 强制执行；无独立 ESLint/Prettier 配置。
- **提交信息**：Conventional Commits 形态 `type(scope): subject`。实际使用的 type 包括 `feat` / `fix` / `docs` / `test` / `chore` / `style`。注意：发布 changelog 自动剔除 `docs:` / `test:` / `chore:` / `ci:` / `style:` 前缀的提交（见 `.goreleaser.yml`）——只有 `feat` / `fix` 等实质变更会进入发布说明。

## PR 指南

- **目标分支 `main`**；分支命名无强制约定，惯例为短生命周期的主题分支（如 `fix/xxx`）。
- **CI 三 job 全绿**是合入前提（`push` 与 `pull_request` 均触发）：

| Job | 运行器 | 内容 |
|-----|--------|------|
| `go` | ubuntu + macos 矩阵 | `go vet ./...` + `go test -race -count=1 -v ./...` |
| `web` | ubuntu | `pnpm -C web install --frozen-lockfile` + `pnpm -C web build`（类型检查 + 构建 + 预压缩） |
| `fuzz` | ubuntu | `FuzzDecodeHello`（`./internal/proto/`）与 `FuzzDecodeFileConfig`（`./cmd/wesh/`）各 60s 短跑回归 |

- **提交前本地自查**建议与 CI 同口径：`go vet ./...`、`go test -race -count=1 ./...`、`pnpm -C web build`。
- **前端改动的产物纪律**：改 `web/` 后重建，若重建改写了被跟踪的 `web/dist/index.html`，新产物须随源码一并提交——发布闸按「已提交 dist 与构建产物一致」校验，不一致会拒绝发布（`scripts/release.sh` 的 dist 漂移闸）。
- **PTY/信号/进程收割相关改动**需在 Linux 与 macOS 双平台验证（CI 矩阵已覆盖，macOS 腿同时承担 kqueue 运行时行为的验证）。
- **测试分层**：改哪层跑哪层——Go 逻辑用包内单测；协议层用 `web/uat/phaseNN.mjs` 零依赖脚本（spawn 真实二进制断言）；浏览器观感面用 `web/uat/pw/` Playwright 套件（双机模型，见其 README）。分层策略详见 [docs/TESTING.md](docs/TESTING.md)。fuzz 崩溃语料会自动落入对应包 `testdata/fuzz/`，修复后随常规测试回归。

## Issue 报告

仓库未提供 issue 模板，报告时请包含：

1. **复现步骤**：完整启动命令（含 flag）与操作序列
2. **期望行为 vs 实际行为**
3. **环境**：OS 与架构（仅 linux/darwin 受支持）、`wesh --version` 输出、浏览器类型与版本（前端问题）

安全提醒：不要在 issue 中粘贴凭据或 ro/rw 分享链接中的 token——token 每次启动重新生成，报告时请先脱敏。

## 发布流程（维护者）

发布由维护者执行，贡献者无需操作；了解触发链有助于理解上面的提交信息约定：

1. `scripts/release.sh vX.Y.Z [--dry-run]`：前置四闸校验（tag 形态 / 不重复 / 工作树干净 / 与远端同步）→ 全量测试（与 CI 同口径）→ 前端构建 + dist 漂移闸 → 两目标各 10 分钟长 fuzz → 负载矩阵（`-tags=load`，30 分钟上限）→ 人工确认 → 创建并推送 tag。
2. `v*` tag push 触发 `.github/workflows/release.yml`：CI 侧重建前端后由 goreleaser 产出 linux/darwin × amd64/arm64 四平台 `tar.gz` 与 `checksums.txt`。

## 文档

权威文档为 [README.md](README.md) 与 `docs/` 目录（GETTING-STARTED / ARCHITECTURE / CONFIGURATION / DEPLOYMENT / DEVELOPMENT / TESTING）。这些文档由工具链自动生成并会整体再生成——**直接手改的内容在下次生成时可能被覆盖**，发现问题请开 issue 或在 PR 描述中指出。代码注释与 `web/uat/` 脚本内的说明不属于自动生成范围。

## 许可证

[MIT](LICENSE) © 2026 sworda。提交 PR 即表示同意贡献内容以 MIT 许可证发布。
