<!-- generated-by: gsd-doc-writer -->

# 开发指南

面向 wesh 贡献者的本地开发环境、构建流程与协作规范。首次上手（安装运行）见 [GETTING-STARTED.md](GETTING-STARTED.md)，系统结构见 [ARCHITECTURE.md](ARCHITECTURE.md)，测试细节见 [TESTING.md](TESTING.md)。

## 本地环境搭建

**前置要求**

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | >= 1.26.3 | 以 `go.mod` 为准（CI 用 `go-version-file: go.mod` 钉版） |
| Node.js | 24 | CI 钉 node 24；前端单测依赖 Node 24 内建 type stripping 直跑 `.ts` |
| pnpm | 11.21.0 | CI 钉版（lockfileVersion 9.0，与本地保持一致） |

**克隆与安装**

```sh
git clone https://github.com/sworda/wesh.git
cd wesh
pnpm -C web install
```

无需任何环境变量或配置文件——`wesh --bind 127.0.0.1 -- bash` 本机裸跑不受启动校验限制，是开发期最短的验证路径（loopback 流量不出机）。

**构建顺序（硬依赖）**

前端构建必须先于 `go build`——`web/embed.go` 的 `//go:embed all:dist` 在编译期要求 `web/dist/` 存在：

```sh
pnpm -C web build && go build -o wesh ./cmd/wesh
```

两个关键事实：

- 仓库提交了 `web/dist/index.html` 构建产物（真实终端页，完整 vite 单文件构建）——裸 clone 后不跑前端构建也能直接 `go build` / `go test ./...` 并运行；
- **修改 `web/src/` 前端源码后必须先重新 `pnpm -C web build` 再 `go build`**，否则二进制内嵌的仍是旧产物（`.gz` 预压产物由构建生成、不入库，`.gitignore` 忽略 `web/dist/*.gz`）。

包结构说明：`web/`、`web/uat/`、`web/uat/pw/`、`scripts/` 是四个**互相独立**的 pnpm 包（各自的 `package.json` 与 lockfile）——应用依赖树、jsdom/headless UAT 依赖树、Playwright 浏览器实测层、文档校验工具（`check-mermaid.mjs` 所需的 jsdom + mermaid，`pnpm -C scripts install`）刻意隔离；`web/pnpm-workspace.yaml` 只承载 overrides（`js-base64` 钉 3.9.2），不是 workspace 定义。

## 构建与常用命令

| 命令 | 说明 |
|------|------|
| `pnpm -C web install` | 安装前端依赖（改动 `web/package.json` 后执行） |
| `pnpm -C web build` | 前端构建：`tsc` 类型检查 + `vite build` 单文件打包 + `gzip -k -9` 预压 `dist/index.html` |
| `pnpm -C web dev` | Vite 开发服务器（前端独立调试） |
| `go build -o wesh ./cmd/wesh` | 构建服务端二进制（须在前端构建之后） |
| `go vet ./...` | Go 静态检查（CI 门禁） |
| `go test -race -count=1 ./...` | 全量测试（CI 同口径；**`-race` 需要 CGO，测试环境不要设 `CGO_ENABLED=0`**） |
| `node --test web/src/lib/*.test.ts` | 前端单元测试（Node 24 内建 type stripping 直跑 `.ts`，零测试框架依赖） |
| `node web/uat/phaseNN.mjs [二进制路径]` | 协议层 UAT（Node >= 22 原生 WebSocket/fetch 零依赖脚本，spawn 真实二进制断言；默认路径 `/tmp/wesh-uat/wesh`） |
| `node web/uat/run-all.mjs [二进制路径] [过滤子集...]` | UAT 一键矩阵 runner（17 项 = 协议 12 + jsdom 4 + herdr 场景 1）：串行 spawn 聚合汇总表 + 逐脚本 10min 超时护栏（超时 SIGTERM 进程组转 FAIL 不阻断后续）；过滤子集空格分隔、用默认二进制时首位传 `''` 占位，未知名 exit 2 防拼写静默跑空；退出码全绿 0 / 任一失败 1 / 用法错误 2；二进制缺席自动 `go build`；pw 层不纳入矩阵（双机拓扑硬约束） |
| `node scripts/check-mermaid.mjs [file.md ...]` | mermaid 词法校验载具（jsdom 注入 + Node 端 `mermaid.parse`，禁 mmdc/puppeteer——Linux headless 约束下的渲染合法性等价物）：无参默认扫 `docs/` 全部 `.md`，任一块 FAIL 退出 1；首次需 `pnpm -C scripts install` |
| `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wesh ./cmd/wesh` | 纯静态构建（Docker scratch 镜像等场景；`CGO_ENABLED=0` 仅属发布构建，日常测试禁用该变量） |
| `./scripts/release.sh --dry-run v1.0.0` | 发布干跑：只执行前置校验四闸（tag 形态/不存在/工作树干净/与远端同步），打印步骤清单不执行 |
| `./scripts/release.sh vX.Y.Z` | 真实发布：前置校验 → 全量测试 → 前端构建 → 长 fuzz ×2（每目标 10 分钟）→ 负载矩阵 → 确认闸 → tag push |

Fuzz 目标三个：`FuzzDecodeHello`（internal/proto）、`FuzzDecodeResize`（internal/proto）、`FuzzDecodeFileConfig`（cmd/wesh）——其中 Hello 与 FileConfig 进 CI 60s 短跑与发布脚本 10 分钟长跑（下方两命令），`FuzzDecodeResize` 仅随常规 `go test` 做种子回归；崩溃语料自动落对应包 `testdata/fuzz/`：

```sh
go test -fuzz=FuzzDecodeHello -fuzztime=60s ./internal/proto/
go test -fuzz=FuzzDecodeFileConfig -fuzztime=60s ./cmd/wesh/
```

浏览器实测层（Playwright 驱动真实 Chromium，覆盖面板文案/重连倒计时/清屏重绘等观感面）在 `web/uat/pw/` 独立包内，运行模型见该目录 README.md——需要具备 GUI 的机器，不属常规开发循环。

## 代码风格

无独立 lint/format 工具链，纪律由 stdlib 与 CI 门禁承载：

- **Go**：`gofmt` 标准格式（全仓已格式化，提交前 `gofmt -l ./cmd ./internal ./web` 应输出为空）+ `go vet ./...`（CI 强制）。未配置 golangci-lint。收口闸必须用工具链自带的 `$(go env GOROOT)/bin/gofmt`（go1.26.3，识别 Go 1.19+ 现代 doc-comment 规则的注释块重排）——PATH 上的旧版系统 gofmt（如本机 `/usr/bin/gofmt`，实为 go1.15.3）不识别新规则，会漏检。
- **TypeScript**：`web/tsconfig.json` 严格模式（`strict`、`noUnusedLocals`、`noUnusedParameters`、`noFallthroughCasesInSwitch`、`verbatimModuleSyntax`），类型检查随 `pnpm -C web build` 的 `tsc` 步骤执行（CI web job 强制）。未配置 ESLint/Prettier/Biome。
- **测试文件**：`web/src/lib/*.test.ts` 被 tsconfig 排除，不参与 `tsc`，只经 `node --test` 执行——相对导入必须带 `.ts` 扩展名。
- **注释语言**：现有代码库注释为中文，含决策依据登记；新代码保持一致。
- **依赖纪律**：Go 侧零新依赖——`go.mod` 维持 5 个外部依赖（`coder/websocket`、`creack/pty`、`golang.org/x/sys`、`golang.org/x/time`、`go-toml/v2`）不变；UAT 协议层与矩阵 runner 同为 Node 原生能力零依赖形态。

## 测试机械约定（Go 侧）

internal/server 的多模式测试执行三维归类机械（测试体系分层总览见 [TESTING.md](TESTING.md)，此处是改动服务端测试时的装配纪律）：

- **单一装配点**：服务端装配一律走 `internal/server/harness_test.go` 的 `newTestServer` 四形态小族——二值普通形 `newTestServer(t, mode, argv, mutate)`（绝大多数调用点）、三值 `newTrackedTestServer`（+`waitHandlers`，stderr 捕获类同步边）、四值 `newHandleTestServer`（+`waitHandlers`+`srv`，Shutdown 直调/白盒面）、三值 `newSessTestServer`（+`sessions` 访问器，PTY 读回面）。各形态对 `mode=shared`/`mode=per-client` 两分支直传既有母本 helper（零装配行为改写，Cleanup 纪律保留），未知 mode 一律 `t.Fatalf` fail-fast。
- **t.Run 双跑**：测试经 `t.Run("mode=shared")` / `t.Run("mode=per-client")` 子测试天然双模式覆盖（CI `go test` 单 step 零改动承载），当前规模 216 个 `mode=` 子测试。
- **三维归类**：① mode-agnostic——同一断言双模式直跑；② mode-mapped——断言分叉点显式成表（先例 `exit_test.go` 双模式分叉表：shared 列期望值与 v1.0 语义逐字一致作零回归证据本体，per-client 列单列）；③ mode-exclusive——显式断言某能力在另一模式未装配，而非默认不测。

新增覆盖双模式行为的测试时：装配复用小族入口、不新建散点 helper；断言按三维显式归类，不写隐式的 `if mode` 分支。

## 分支约定

- 主分支为 `main`；版本史与 git tag 同源（`v*` tag 触发发布，起点 v1.0.0）。
- 无成文分支命名规范。仓库现行实践：功能开发按主题开分支（如 `phase08-observability`、`phase-07`），修复走 `fix/*` 前缀（如 `fix/scan-and-macos-ci`）。

## PR 流程

仓库未提供 PR 模板，贡献以 CI 全绿为准入门槛（`.github/workflows/ci.yml`，push 与 PR 均触发）：

1. **提交信息**遵循 Conventional Commits 形态 `type(scope): subject`——现行 type：`feat` / `fix` / `docs` / `test` / `chore` / `ci` / `style`（`docs:`、`test:`、`chore:`、`ci:`、`style:` 前缀的提交不进发布 changelog）。
2. **CI 三 job 必须全绿**：
   - `go`（ubuntu + macos 双平台矩阵）：`go vet ./...` + `go test -race -count=1 -v ./...`——macos leg 同时承担 kqueue 子进程收割的运行时验证，勿以「本机 Linux 过了」替代；
   - `web`（ubuntu）：`pnpm -C web install --frozen-lockfile` + `pnpm -C web build`；
   - `fuzz`（ubuntu）：两目标各 60s。
3. **提交前自查**：`$(go env GOROOT)/bin/gofmt -l ./cmd ./internal ./web` 零输出（用 GOROOT 版，见「代码风格」）、`go vet` 零告警、`tsc` 严格模式零错误、改过 `web/src/` 后已重新执行 `pnpm -C web build`。
4. 提交应尽量原子化（一提交一主题），scope 用包名或主题域（如 `fix(09): ...`、`test(server): ...` 的现行粒度）。

发布流程（`scripts/release.sh` 单脚本整合：校验 → 测试 → 构建 → fuzz → 负载矩阵 → tag push，tag push 后 goreleaser 接管四平台产物）属维护者操作，贡献者无需介入；细节见 [README.md](../README.md) 与脚本头部注释。版本三元组注入点为 `cmd/wesh/main.go` 的 `var version/commit/builtAt`——goreleaser 发布构建经 `-X` 注入 tag 版本、完整 commit hash 与 commit 时间戳（Unix 秒，`--version` 按运行机时区显示；时间戳与 goreleaser `mod_timestamp` 同源，保可复现构建）；本地 `go build` 版本号恒显示 `dev`，commit/时间由 Go 工具链内嵌的 VCS 构建信息自动回填（仓库外构建显示 `none`/`unknown`）。

协作规范总览见 [CONTRIBUTING.md](../CONTRIBUTING.md)。
