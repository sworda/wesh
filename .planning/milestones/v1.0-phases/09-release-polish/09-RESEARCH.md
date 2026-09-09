# Phase 9: 发布与打磨 - Research

**Researched:** 2026-08-29
**Domain:** Go 发布工程（goreleaser/GitHub Actions）+ Go fuzz/负载测试 + 部署配方（Caddy/Cloudflare/Docker/systemd）+ 前端清零修复
**Confidence:** MEDIUM-HIGH（goreleaser/tini/Action 全部经 gh api 直读官方仓库一手核实 + Go fuzz 本机 1.26.3 实证；Caddy/CF 行为面为 MEDIUM——Context7 配额耗尽、WebFetch 被网络策略阻断，D-15 本机实证为既定兜底）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**发布流程与版本策略（OPS-10）**
- **D-01:** 发布触发 = **tag push 自动触发**：推 `v*` tag 触发独立 release.yml，版本号起点 v1.0.0——版本史与 git tag 同源，个人运维工具零额外流程负担
- **D-02:** 供应链文件 = **仅 checksums.txt**（goreleaser 默认产出）——无 cosign 签名/SBOM；个人运维工具用户自编译成本低、供应链威胁模型弱，零密钥管理负担
- **D-03:** 前端产物固化 = **release.yml workflow 步骤显式编排**：pnpm install+build（钉 pnpm 11.21.0/node 24，与 ci.yml web leg 同版）→ setup-go → goreleaser——构建顺序肉眼可审，不用 goreleaser before hooks（环境隐式），不用入库 dist（发布产物可能滞后 web/src 未构建提交）
- **D-04:** 打包形态 = **tar.gz 含 wesh + LICENSE + README.md**（`wesh_v1.0.0_linux_amd64.tar.gz` 命名族）——解压即见文档，scp 单文件也行 — **Reversibility:** costly

**自定义首页（OPS-03）**
- **D-05:** 语义 = **整页替换（ttyd -i 同款）**：用户 HTML 完全替代首页，终端功能由用户页面自行实现（WS 端点照旧暴露），wesh 只负责伺服——零模板注入面，拒绝片段注入 — **Reversibility:** one-way
- **D-06:** 给页通道 = **全通道统一替换**：根路径与 `/s/{token}/` 都伺服自定义页（单一给页源零双轨）；README 明示「自定义页需自行实现终端逻辑，否则分享链接失去终端功能」 — **Reversibility:** one-way
- **D-07:** flag 形态 = **`--index /path/to.html` 启动一次读入内存**（不存在/不可读/非常规文件 exit 2 fail-fast）；运行期零磁盘依赖，改文件需重启生效；入 TOML 配置同名键 — **Reversibility:** one-way
- **D-08:** 大小上限 = **默认 16MiB 硬顶**（io.LimitReader 启动读入，超限 exit 2 报错行只含路径不含内容——启动面红线）+ **TOML 配置键 `index-max-size` 可调**；**不开 `--index-max-size` CLI flag**——纯配置键无对应 flag 是 P7 D-03 纪律的明示例外，README 写明

**负载/模糊测试**
- **D-09:** fuzz 目标面 = **proto 帧解码（DecodeHello/帧拆分）+ TOML 配置解析**两处——WS 远程输入面与本地配置文件解析面，Go native fuzz 零新依赖
- **D-10:** fuzz 运行形态 = **CI 短跑 1-2min 回归门 + 发布前长跑 10min+**；崩溃语料 `testdata/fuzz/` 入库永久回归；长跑纳入发布脚本（D-14）
- **D-11:** 负载测试载体 = **Go 黑盒负载测试**（internal/server，build tag 隔离不进常规 CI，手动 `-tags=load` 跑）：coder/websocket 客户端性能远超 Node 单线程；runtime.NumGoroutine/ReadMemStats 上界断言直接；与既有夹具同语言同库
- **D-12:** 标定回填纪律 = **验证为主、证伪才改**——默认验证现值成立（README 既定验收：合法慢端零误踢 + 内存上界成立 + 信用门开闭频率可接受）；数据证伪才改**常量默认值**（改值需负载数据支撑），**不动可配性/flag 面**
- **D-13:** 标定结果落点 = **README「默认参数与 Phase 9 标定」节更新**（初值→标定值/验证结论 + 实测数据摘要），表头改名去「Phase 9 标定」挂账语

**发布脚本**
- **D-14:** 发布前操作整合为**发布脚本**（长 fuzz 10min+ → 负载矩阵 → 打 tag push 触发 release.yml），发布前跑一次——用户裁决原话：「把所有发布时需要做的操作都整合在这个脚本内，发布之前跑一次即可」；脚本即发布文档的可执行形态，README/文档引用

**部署文档（ROADMAP SC3）**
- **D-15:** 反代配方验证深度 = **Caddy 本机实证**（二进制部署 + 双机全链套路同 nginx G-07-2 先例）+ **Cloudflare 按官方文档写并标注「未实测」**（SaaS 无法本机复现，风险接受）；两配方均含空闲超时与 ping 间隔关系（CORE-06 默认 5s ping × 反代 idle timeout 匹配表）
- **D-16:** Docker = **Dockerfile 入库**（scratch + 静态二进制 + tini 作 PID 1 收割）+ **本机 docker 24.0.6 构建实测**（PID 1 收割/僵尸残留行为验证）；**不发布镜像**（scp 哲学一致，镜像用户自建）
- **D-17:** systemd unit = **`deploy/wesh.service` 入库**（Restart=/LimitNOFILE=/EnvironmentFile=600 全配）+ README 引用 + 复用 P8 实机 systemctl 通道最小实测（08-05 draining 观测同通道）

**ship 清零（07 deferred-items.md 全结）**
- **D-18:** 三项 UI WARNING 全清——① 1001 关停面板 hint 按场景条件化（main.ts:903 hintPrefix 条件化）；② `#status` 面板族加 `role="alert"`（index.html:63 单属性零视觉影响）；③ pre-onopen 到达的 1001 先按码分派（main.ts:881-884 `!opened` 分支前分派 ev.code===1001）

**已锁定不重复决策：** embed 链 pnpm build 先于 go build、dist 真实产物入库、裸 clone 可编译（P1 D-18）；`version = "dev"` ldflags 注入点已就位（main.go:32）；CI 注释明示 CGO_ENABLED=0 只属于发布构建（-race 需 cgo，ci.yml 两通道并存）；CLI flag 全名无短选项 + parse/validate 分层 + fail-fast + 敏感值记录式上报（P2 D-15/P3）；配置文件 TOML 平铺键 = flag 同名、未知键拒绝（P7 D-03/D-06）；凭据/token 永不入日志（SEC-01/P5 D-03）；文档即被测物（G-07-2 nginx 配方双机实证先例）；测试分层策略与双机拓扑（CODEBUDDY.md：Linux 侧协议/DOM UAT，Windows 侧 Playwright，禁装顺序不变）；退出码 255 语义（P6 OQ1）；标定挂账清单（README §默认参数与 Phase 9 标定：outbox 512KiB / 水位 50% / input 32KiB/s÷64KiB / max-clients 32 / resize 50ms / attachGrace 500ms / pong 10s / hello 5s / EXIT 2s / stop-timeout 0 / exit-when-empty 宽限）。

### Claude's Discretion

- goreleaser 配置细节（-trimpath、ldflags `-s -w -X main.version={{.Version}}`、mod_timestamp、CGO_ENABLED=0 env、archive 命名模板精确形状）
- release.yml 与 ci.yml 的文件关系（独立文件；tag 触发条件写法；web leg 步骤复用形态）
- 发布脚本的落点与形态（scripts/release.sh 候选；含干跑/确认闸；打 tag 前工作树干净校验）
- fuzz 种子语料选型（合法 Hello/畸形帧/边界长度/TOML 合法与畸形样本）与 CI 短跑精确时长
- 负载矩阵格子数与每格时长（README 方法论 1/4/16/32 × 输出速率 × 慢链路注入的实例化；defunct 检测口径 = goroutine 数/fd 数/僵尸进程三面）
- 自定义页的 gzip 处理（启动读入后可顺手预压一次缓存——实现细节，与 embed .gz 旁路正交）【09-UI-SPEC §Custom Index Contract 4 已定稿：采纳预压】
- --index 与 --base-path 组合下的伺服装配（mux 前缀内自然成立）与安全头中间件适用性（现状同源）
- Caddy 实证的部署形态（官方二进制直装，禁 apt 桌面包纪律不涉服务端软件）与双机全链断言面
- Dockerfile 细节（scratch vs distroless、CA 证书束 TLS 出站需要性评估——wesh 无出站调用倾 scratch 零证书、EXPOSE/ENTRYPOINT 形态、--socket 容器内语义说明）
- D-18 三项修复的精确文案与断言面（09-UI-SPEC C-10/R1-R3 已逐字定稿）

### Deferred Ideas (OUT OF SCOPE)

- **cosign 签名 / SBOM 供应链增强** — D-02 裁决仅 checksums；若发布后被企业用户要求合规再评估
- **Docker 镜像发布（ghcr.io）** — D-16 裁决 Dockerfile 入库不发镜像；真实需求出现再评估
- **nightly/snapshot 预发布通道** — D-01 裁决 tag 触发正式 release；个人项目 v1 阶段无需求
- **负载测试进 CI 常规回归** — D-11 裁决 build tag 手动跑；若未来参数回归事故再评估 CI 化
- **Cloudflare 配方实证** — D-15 风险接受标注未实测；有 CF 账号环境后补实测
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OPS-03 | 自定义首页 HTML | D-05..D-08 行为契约 + 09-UI-SPEC §Custom Index Contract 逐字边界；扩展点实证：server.go:449 唯一 `web.Handler()` 调用点（装饰落点）、sharetoken.go:87-96 sharePage 委托（全通道统一替换交汇点）、config.go fileConfig 两新键宿主；启动校验矩阵/错误红线先例（P3/P7）直接复用 |
| OPS-10 | 单静态二进制发布（linux/darwin × amd64/arm64），前端 embed 单 HTML | goreleaser v2.18.0 + goreleaser-action v7.2.3 一手核实配置（CGO_ENABLED=0 四平台、`-X main.version` 挂 main.go:32 既有注入点、tar.gz+LICENSE/README、checksums.txt 显式钉名）；release.yml D-03 显式编排形态（pnpm build → setup-go → goreleaser）；embed 链 pnpm→go build 顺序 P1 D-18 已锁定 |
</phase_requirements>

## Project Constraints (from CODEBUDDY.md)

| 约束 | 对 planner 的含义 |
|------|------------------|
| 双机拓扑：Linux 开发机 headless（**禁装 playwright/X11/浏览器**，不在该侧启动 wesh 等人工浏览器访问） | Caddy/Docker/systemd 实证全部在 Linux 侧以协议层/二进制方式断言；浏览器观感断言只走 Windows 侧 Playwright（web/uat/pw/ 先例） |
| Windows 工作站（GUI）：Playwright 宿主；**禁在 Windows 构建/运行 wesh 服务端**；断网模拟一律用 TCP 转发器 kill/restore，禁操作真实网卡 | D-15 Caddy 双机全链复用 G-07-2 套路（phase07-a2-pw.mjs / phase07-a2-ctl.sh 先例） |
| 分层测试策略：协议层（Node 零依赖 .mjs spawn 真实二进制）/ @xterm/headless / jsdom / Playwright / 平台原生行为显式豁免 | OPS-03 归协议层+jsdom；D-18② role="alert" 归 jsdom 断言；真实 AT 播报属豁免面（skipped+reason） |
| pnpm 而非 npm；构建命令带 `time` 前缀 | release.yml 与文档一律 pnpm；本机验证命令带 time |
| 分析结果输出到文档文件；文档放项目目录 | RESEARCH.md 本文件即落点 |
| 技术文档用 mermaid 画 UML 图 | 本文件架构图用 mermaid |
| Bash 工具用 fish shell；修改前备份当前状态 | 提交进仓的脚本（release.sh 等）用 POSIX bash 保可移植；执行期破坏性操作先备份 |
| 禁 apt-get install 任何 GUI/X11/浏览器库（Linux 侧）；服务端软件不受此限 | Caddy/goreleaser 以官方二进制直装（GitHub release），不经 apt |

## Summary

本 phase 是把 v1.0.0 送出门的收尾：四条工作线——**(1) goreleaser 发布链**（OPS-10）、**(2) 自定义首页**（OPS-03）、**(3) fuzz + 负载测试回填标定**、**(4) 部署文档 + ship 清零**。调研结论：发布链的全部关键事实已一手核实——goreleaser 当前稳定版 v2.18.0、goreleaser-action v7.2.3，配置面有三个**与直觉/旧版本不符的实证发现**：① `.Version` 模板变量会剥离 tag 的 `v` 前缀（源码 `strings.TrimPrefix(ctx.Git.CurrentTag, "v")`），D-04 命名族 `wesh_v1.0.0_*` 需改用 `{{ .Tag }}`；② v2.18.0 的 checksum 默认文件名是 `{{ .ProjectName }}_{{ .Version }}_checksums.txt`，**不是** D-02 假定的裸 `checksums.txt`，必须显式 `name_template: checksums.txt` 钉死；③ archive 配置在 v2 是 `formats`（列表）+ `name_template`（单数），JSON schema 已核实键名。main 包在 `./cmd/wesh`（goreleaser 默认 `.` 会构建失败），`version = "dev"` ldflags 注入点已就位零代码改动。

fuzz 侧全部机制经本机 go 1.26.3 实证：`-fuzztime` 可控时长、不带 `-fuzz` 时种子语料作为普通单测跑（CI 零时长回归门）、CGO_ENABLED=0 下 fuzzing 正常、**每次 `-fuzz` 调用只能匹配单包单目标**（两目标 = 两次调用，直接影响 CI fuzz leg 与发布脚本形态）。TOML fuzz 有一个真实的接缝重构点：`loadFileConfig(path)` 是 path-in，fuzz 需要 bytes-in——提取 reader 委托助手（错误分类/configErr 包装保持单写口）是 D-09 落地的必要前置。

部署侧：tini v0.19.0 静态资产与 sha256 钉值已取到（Dockerfile 可直接钉死）；Caddy v2.11.4 与 Cloudflare 的超时/Host 语义为 MEDIUM 置信（官方文档无法本会话直取），D-15 既定的 Caddy 本机实证是兜底；systemd 的关键交互（wesh 优雅关停退出码 255 × Restart= 语义）已理清。Docker 有一个必须写进文档的设计张力：**scratch 镜像里没有 shell**，wesh 的核心用途（spawn 用户命令）在纯 scratch 里无命令可跑——Dockerfile 是 PID-1 收割模式的参考形态，僵尸收割实测需 bind-mount 宿主二进制做夹具（镜像本体保持纯净）。

**Primary recommendation:** 按 §Code Examples 的 goreleaser.yml / release.yml / Dockerfile / wesh.service 定稿形态直接落地——全部版本与关键键名已核实；fuzz/负载测试按 §Architecture Patterns 的接缝设计（proto 两目标直挂 DecodeHello/DecodeResize，TOML 先提取 reader 委托）；--index 按 09-UI-SPEC §Custom Index Contract 在 server.go:449 唯一调用点装饰。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 发布触发与编排（tag→构建→GitHub Release） | CI/GitHub Actions（release.yml） | goreleaser（构建执行器） | D-01/D-03：tag 是版本史唯一源头；workflow 显式编排保可审性 |
| 四平台交叉编译与打包 | Build tier（goreleaser 配置） | go build（embed 读取 dist） | CGO_ENABLED=0 纯 Go 交叉编译无 C 工具链负担 |
| 前端产物固化（dist → embed） | Build tier（pnpm build，release.yml 显式步骤） | go:embed（web/embed.go） | P1 D-18：pnpm 先于 go build；D-03：不用 goreleaser before hooks |
| 自定义首页伺服 | API/Backend（Go server 给页通道） | web 包（装饰实现） | 启动一次读入内存，运行期纯伺服；`/s/{token}/` 经 sharePage 委托统一 |
| --index 启动校验与配置 | API/Backend（cmd/wesh parse/validate 分层） | config.go fileConfig | P2 D-15/P3 fail-fast 矩阵与 P7 TOML 机制的既有宿主 |
| fuzz（proto/TOML） | Test tier（Go stdlib testing.F） | CI fuzz leg + 发布脚本长跑 | D-09/D-10：零新依赖，种子语料即单测 |
| 负载矩阵与标定回填 | Test tier（internal/server `-tags=load` 黑盒） | /metrics 计数器（数据源） | D-11/D-12：验证为主、证伪才改 |
| 部署配方（Caddy/CF/Docker/systemd） | Docs tier（README + deploy/ 入库件） | 双机全链实证（G-07-2 套路） | 文档即被测物；CF 唯一例外标注未实测 |
| D-18 三项 UI 清零 | Browser/Client（main.ts/index.html） | jsdom 断言层 | 09-UI-SPEC R1-R3 逐字定稿 |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| goreleaser | **v2.18.0**（2026-08-24）[VERIFIED: gh api repos/goreleaser/goreleaser/releases/latest] | 四平台交叉编译/打包/校验和/GitHub Release | Go 项目发布事实标准；D-01..D-04 全部能力内建 |
| goreleaser-action | **v7.2.3**（2026-06-29）[VERIFIED: gh api repos/goreleaser/goreleaser-action/releases/latest + action.yml] | CI 中安装并运行 goreleaser | 官方 Action；自动校验 goreleaser 下载包 checksums（README「Verification」节） |
| Go stdlib `testing.F` | go **1.26.3**（本机实证）[VERIFIED: 本机 `go version` + fuzz 冒烟] | proto/TOML 两面 fuzz | D-09 零新依赖裁决；种子语料自动变单测 |
| tini | **v0.19.0**（2020-04-19，项目成熟低变更）[VERIFIED: gh api repos/krallin/tini/releases/latest] | 容器 PID 1 僵尸收割/信号转发 | PITFALLS C8/Pitfall 8 既定方案；静态单文件 1MB 内 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| actions/checkout | v7.0.1 [VERIFIED: gh api releases/latest] | 检出（须 `fetch-depth: 0`） | release.yml（与 ci.yml 钉版一致） |
| actions/setup-go | v7.0.0 [VERIFIED: gh api releases/latest] | Go 安装（`go-version-file: go.mod`） | release.yml（与 ci.yml 钉版一致） |
| actions/setup-node | ci.yml 现钉 @v4（最新 v7.0.0 存在） | node 24 安装 | release.yml web leg——建议逐行复用 ci.yml 的 @v4 保单一形态（升级是另一笔 chore） |
| pnpm/action-setup | v6.0.10（钉 pnpm 11.21.0）[VERIFIED: gh api releases/latest] | pnpm 安装 | release.yml web leg（逐行复用 ci.yml） |
| Caddy | **v2.11.4**（2026-06-03）[VERIFIED: gh api repos/caddyserver/caddy/releases/latest] | D-15 反代配方本机实证 | 仅实证环境安装（GitHub release 静态二进制直装），非 wesh 依赖 |
| coder/websocket | v1.8.15（go.mod 现状）[VERIFIED: go.mod] | 负载测试 WS 客户端库 | D-11：与服务端同库，零新依赖 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| goreleaser | 手写 Makefile + GOOS/GOARCH 循环 + gh release upload | 校验和/命名/changelog/Release API 全要手搓；D-04 命名族 costly 级契约下无收益 |
| goreleaser-action 安装 | `go install github.com/goreleaser/goreleaser/v2@v2.18.0` | Action 自带下载校验 + 缓存；go install 全量编译慢数倍。**本机验证**（`goreleaser check`/`--snapshot`）可用 go install 或官方二进制 |
| tini | dumb-init / `docker run --init` / 自实现收割 | tini 静态单文件最适合 scratch ADD；`--init` 依赖 docker CLI 旗标不入镜像（文档提及作替代）；自实现 = 重新造 C8 的错 |
| scratch | distroless/static | wesh 无 glibc/证书依赖（CGO_ENABLED=0 + 无 TLS 出站），distroless 多出的 tzdata/证书是死重；D-16 已裁决 scratch |
| Go 黑盒负载测试 | Node 洪水驱动（phase05-flood-driver.mjs 形态） | D-11 已裁决：Node 单线程性能不足以压满服务端；且 goroutine/内存断言需在进程内 |

**Installation:**

```bash
# 本机验证用（Linux 开发机；release.yml 中由 goreleaser-action 自动安装，无需本机）
go install github.com/goreleaser/goreleaser/v2@v2.18.0   # Go proxy 实证 v2.18.0 最新稳定 [VERIFIED: go list -m -versions]
# 或官方二进制：github.com/goreleaser/goreleaser/releases/download/v2.18.0/goreleaser_Linux_x86_64.tar.gz

# Caddy 实证用（官方二进制直装，CODEBUDDY 禁 apt 纪律不涉服务端软件；Claude's Discretion 既定形态）
# github.com/caddyserver/caddy/releases/download/v2.11.4/caddy_2.11.4_linux_amd64.tar.gz

# tini 不进宿主——Docker 构建期由 Dockerfile ADD（sha256 钉死，见 §Code Examples）
```

**Version verification:** 全部版本于 2026-08-29 经 `gh api repos/<org>/<repo>/releases/latest`（goreleaser/goreleaser-action/tini/Caddy/四个 Action）与 `go list -m -versions`（goreleaser/v2 module）实证；训练数据版本一律未采用。

## Package Legitimacy Audit

> 本 phase **零 npm/pypi/crates 新包**（前端零新运行时依赖——09-UI-SPEC §Registry Safety；Go 零新依赖——fuzz 为 stdlib、负载测试用既有 coder/websocket）。gsd package-legitimacy seam 仅支持 npm/pypi/crates（本会话实测 `--ecosystem go` 拒绝），故外部制品全部以 **gh api 直读官方组织仓库**核验：

| 制品 | 来源 | 核验证据 | Verdict | Disposition |
|------|------|----------|---------|-------------|
| goreleaser v2.18.0 | github.com/goreleaser/goreleaser + Go module proxy | releases/latest API + proxy.golang.org 版本列表一致；源码/schema 直读吻合 | OK | Approved |
| goreleaser-action v7.2.3 | github.com/goreleaser/goreleaser-action | releases/latest API；action.yml/README 直读 | OK | Approved（`@v7.2.3` 钉死） |
| tini v0.19.0 | github.com/krallin/tini | releases/latest API；资产清单含 tini-static-amd64/arm64 + .sha256sum；sha256 钉值已取回（§Code Examples） | OK | Approved（sha256 钉死进 Dockerfile） |
| caddy v2.11.4 | github.com/caddyserver/caddy | releases/latest API | OK | Approved（仅实证环境，非交付物） |
| actions/{checkout,setup-go,setup-node}、pnpm/action-setup | github.com/actions/*、pnpm/* | releases/latest API；与 ci.yml 既有钉版一致 | OK | Approved |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
**`[ASSUMED]` 包：** none（本 phase 无 WebSearch/训练来源的包名进入推荐）

## Architecture Patterns

### System Architecture Diagram

```mermaid
flowchart LR
    subgraph 发布链["发布链（D-01..D-04, OPS-10）"]
        A[scripts/release.sh<br/>长 fuzz 10min+ × 2 目标<br/>→ 负载矩阵 -tags=load<br/>→ 确认闸 → git tag v* push] -->|tag push| B[release.yml<br/>checkout fetch-depth:0]
        B --> C[pnpm install --frozen-lockfile<br/>pnpm build 钉 11.21.0/node24]
        C -->|dist 真实产物| D[setup-go go-version-file]
        D --> E[goreleaser-action v7<br/>release --clean]
        E -->|go:embed 读 dist| F[4× 静态二进制<br/>linux/darwin × amd64/arm64<br/>CGO_ENABLED=0 -trimpath<br/>-X main.version]
        F --> G[tar.gz × 4<br/>wesh+LICENSE+README.md<br/>+ checksums.txt]
        G --> H[GitHub Release v1.0.0]
    end

    subgraph 自定义首页["自定义首页（D-05..D-08, OPS-03）"]
        I[--index /path/to.html<br/>启动校验 exit 2 矩阵<br/>不存在/不可读/非常规/超16MiB] --> J[启动一次读入 []byte<br/>+ gzip 预压一次缓存]
        J --> K[server.go:449 装饰点<br/>index.html 路径返回自定义字节<br/>Vary: Accept-Encoding 恒发]
        K --> L[/ 根路径给页]
        K --> M[/s/{token}/ 给页<br/>sharePage 委托统一]
    end

    subgraph 验证链["验证链（D-09..D-13）"]
        N[FuzzDecodeHello/FuzzDecodeResize<br/>internal/proto] --> O[CI fuzz leg 短跑<br/>2×60s]
        N --> P[发布脚本长跑 10min+<br/>崩溃语料 testdata/fuzz 入库]
        Q[FuzzDecodeFileConfig<br/>cmd/wesh reader 委托接缝] --> O
        Q --> P
        R[load_test.go -tags=load<br/>矩阵 1/4/16/32 × 速率 × 慢链路] --> S[/metrics 计数器断言<br/>kicks/gateTransitions/outbox depth<br/>+ NumGoroutine/ReadMemStats/defunct 三面]
        S --> T[README 标定表回填<br/>验证为主 证伪才改]
    end
```

### Recommended Project Structure（增量）

```
repo-root/
├── .goreleaser.yml              # 新建：D-02/D-04 定稿形态（§Code Examples）
├── .github/workflows/
│   ├── ci.yml                   # 既有；新增 fuzz leg（D-10 短跑门，独立 job）
│   └── release.yml              # 新建：D-01/D-03 显式编排
├── cmd/wesh/
│   ├── config.go                # fileConfig + Index/IndexMaxSize 两键；提取 decodeFileConfig reader 委托（fuzz 接缝）
│   ├── fuzz_test.go             # 新建：FuzzDecodeFileConfig
│   └── testdata/fuzz/           # 新建入库：TOML 崩溃语料（D-10）
├── internal/proto/
│   ├── fuzz_test.go             # 新建：FuzzDecodeHello + FuzzDecodeResize
│   └── testdata/fuzz/           # 新建入库：proto 崩溃语料（D-10）
├── internal/server/
│   └── load_test.go             # 新建：//go:build load 黑盒负载矩阵（D-11）
├── scripts/
│   └── release.sh               # 新建：D-14 发布脚本（bash，确认闸+干净校验）
├── deploy/
│   └── wesh.service             # 新建：D-17 systemd unit 模板
├── Dockerfile                   # 新建：D-16 scratch+tini（sha256 钉死）
├── web/uat/phase09.mjs          # 新建：OPS-03 协议层 UAT（启动校验矩阵+双通道给页+gzip/Vary）
└── web/uat/pw/phase09-caddy-pw.mjs  # 新建（Windows 侧）：D-15 Caddy 双机全链（复用 phase07-a2 套路）
```

### Pattern 1: release.yml 显式编排（D-03 定稿形态）

**What:** tag push 触发，pnpm build → setup-go → goreleaser 逐行显式步骤，与 ci.yml web leg 同钉版。
**When to use:** 唯一发布通道；不用 goreleaser `before.hooks`（环境隐式，D-03 裁决排除）。
**关键实证点：**
- `fetch-depth: 0` 必配——goreleaser changelog/版本推断需要全量 tag 历史 [VERIFIED: goreleaser-action README「IMPORTANT」节]
- `permissions: contents: write` 是 GITHUB_TOKEN 创建 Release 的最小授权 [VERIFIED: goreleaser-action README workflow 示例]
- tag 触发写法：`on.push.tags: ['v*']` [VERIFIED: goreleaser-action README「Run on new tag」节]
- **dist .gz 不入库但发布会带上**：`.gitignore` 有 `web/dist/*.gz`（本地构建产物漂移），release.yml 中 pnpm build 生成 .gz 后 goreleaser 嵌入——release 二进制走 gzip 旁路、本地 dev 二进制明文降级，与 03-05 既定语义一致 [VERIFIED: .gitignore:2 + web/embed.go:37-48]

### Pattern 2: --index 装饰（09-UI-SPEC §Custom Index Contract 定稿）

**What:** `web.Handler()` 的唯一调用点（server.go:449，`wh, err := web.Handler()`）[VERIFIED: internal/server/server.go:449] 处按 `Options.CustomIndex []byte`（生产直传字段，BasePath/AuthHeader/Version 先例形态）装饰——`index.html` 路径（含空路径回落）返回启动读入字节，其余路径照旧走 embed FileServerFS（相对资源 404 是契约语义）。`/` 与 `/s/{token}/` 两通道经 sharePage 既有委托自然统一（装饰层在 sharetoken.go:91 委托上游）[VERIFIED: internal/server/sharetoken.go:87-96]。
**When to use:** `CustomIndex == nil` 时与现状逐字节一致（零值兜底纪律）。
**实现要点：**
- 读入：`os.Stat` → `Mode().IsRegular()` 闸（目录/设备/socket 拒）→ `io.LimitReader(f, max+1)` 读入 → `len > max` 拒（max 默认 16MiB，`index-max-size` TOML 键调）
- gzip：启动预压一次缓存（09-UI-SPEC §4 定稿采纳）；`Accept-Encoding` 显式含 gzip → 发预压体 + `Content-Encoding: gzip`；`Vary: Accept-Encoding` 恒发（embed.go:36 同款纪律）；解析复用 web 包 `acceptsGzip` [VERIFIED: web/embed.go:53-72]
- 错误行：只含路径 + 原因类别，**绝不含文件内容字节**（P3/P4 启动面红线延伸，D-08）
- base-path 组合：给页路径 `{bp}/` 与 `{bp}/s/{token}/`，mux 前缀内自然成立（07-01 StripPrefix 装配现状）

### Pattern 3: fuzz 两目标 + 单接缝重构

**What:** 
- `internal/proto/fuzz_test.go`：`FuzzDecodeHello`/`FuzzDecodeResize` 直挂既有导出函数 [VERIFIED: internal/proto/proto.go:136 `func DecodeHello(payload []byte) (HelloPayload, bool)`；:203 `func DecodeResize(payload []byte) (cols, rows int, ok bool)`]——bytes-in 纯函数零改造。不变量：DecodeHello 成功时尺寸恒在 [1,1000]（ClampDim 契约，proto.go:212）；任意输入不 panic。注：「帧拆分」面（`data[0]` 类型字节 + `data[1:]` 载荷）在 server.go 有 `len(data) == 0` 前置守卫（server.go:838/1006）[VERIFIED]，无独立 fuzz 目标必要，DecodeResize 即稳态帧解码面。
- `cmd/wesh/fuzz_test.go`：`FuzzDecodeFileConfig` 需要 **bytes-in 接缝**——现状 `loadFileConfig(path string)` 是 path-in（config.go:97-137）[VERIFIED]。提取 `decodeFileConfig(path string, r io.Reader) (*fileConfig, error)` 委托：解码 + 错误分类（StrictMissingError/DecodeError 提取）+ configErr 包装保持单写口；`loadFileConfig` 缩为 open-file + 委托 + D-07 权限警告。不变量：① 不 panic；② err 非 nil 时错误文本不含探针值（值剥离红线的 fuzz 断言形态——种子语料埋 `credential = ["FUZZ_PROBE_SECRET"]` 类探针，断言 err.Error() 不含 "FUZZ_PROBE_SECRET"；键名回显是合法行为不在断言面）。

**运行形态（D-10）：**
- CI 短跑：ci.yml 加独立 `fuzz` job——`go test -fuzz=FuzzDecodeHello -fuzztime=60s ./internal/proto/` + `go test -fuzz=FuzzDecodeFileConfig -fuzztime=60s ./cmd/wesh/`（**两次调用**——`-fuzz` 每次只能匹配单包单目标 [VERIFIED: 本机 `go help testflag`：「the command line argument must match exactly one package within the main module, and regexp must match exactly one fuzz test within that package」]；不加 -race，与 go leg 并行墙钟 +60-70s）
- 常规 `go test ./...`（含 go leg -race）：种子语料 + testdata/fuzz 崩溃语料自动作为普通单测运行 [VERIFIED: 本机实证 `--- PASS: FuzzDecodeHello/seed#0`]——零时长回归门
- 长跑：发布脚本 2×10min+（D-14）；CGO_ENABLED=0 下 fuzzing 正常 [VERIFIED: 本机实证]；JSON 解码目标实测 ~50 万 execs/s（32 workers）[VERIFIED: 本机冒烟]

### Pattern 4: 负载矩阵（D-11/D-12 定稿形态）

**What:** `internal/server/load_test.go`，`//go:build load`，package server_test（全仓测试统一外部包形态 [VERIFIED: e2e_test.go:1/limits_test.go:1/metrics_test.go:1]），复用 `startTrackedServerWith`（e2e_test.go:171）[VERIFIED] 与 stall 夹具纪律（slowclient_test.go：dialHello 后不 Read；loopback 单连接最坏吸收 ≈ wmem 4MiB + rmem 6MiB，洪水须 ≫10MiB）[VERIFIED: slowclient_test.go 头注释]。
**矩阵形状（README 231-243 既定方法论实例化）：** 客户端数 1/4/16/32 × 输出速率（seq 洪水/匀速滴漏/突发）× 慢链路注入（stall 端 + 限速合法读者）。
**断言三面（D-12 验收）：**
1. **合法慢端零误踢**：限速读者（匀速 drain 接近产出速率、突发抖动 < outbox 容量）全程 `wesh_clients_kicked_total` 不增
2. **内存上界成立**：32 端洪水下 `runtime.ReadMemStats` Alloc 稳定（账面最坏 32×512KiB outbox ≈ 16MiB + 共享帧；`wesh_mem_alloc_bytes`/`wesh_outbox_depth_bytes_max` 同步观测）
3. **信用门开闭频率可接受**：`wesh_credit_gate_transitions_total` 速率不震颤（afterDrain 半水位迟滞——`cur >= c.outbox.cap/2` 即 50% 水位 [VERIFIED: clients.go afterDrain 实现]）
**defunct 三面（Claude's Discretion 定口径）：** 高频建销循环（spawn Server + 立即退出的子进程 × N 轮）后：`runtime.NumGoroutine` 回基线 + `/proc/self/fd` 计数回基线 + `/proc/<child>/stat` 无 Z 态（Linux-only，darwin 分支 skip——load 测试本机手动跑，CI 不进）。
**数据源：** /metrics 黑盒 scrape（metrics_test.go 先例）——17 series 中负载相关 8 条已就位 [VERIFIED: metrics.go:122-145：wesh_clients_kicked_total / wesh_outbox_depth_bytes_max|sum / wesh_input_rate_dropped_total / wesh_input_queue_dropped_total / wesh_credit_gate_transitions_total / wesh_goroutines / wesh_mem_alloc_bytes]。

### Pattern 5: 发布脚本（D-14 定稿形态）

**What:** `scripts/release.sh`（bash——入库可移植 artifact，非 fish）：`set -euo pipefail` + 干跑/确认闸 + 打 tag 前工作树干净校验。
**顺序（用户裁决原话「发布之前跑一次即可」）：**
1. 前置校验：`git status --porcelain` 为空（脏树拒绝）；参数 `v$X.Y.Z` 形态校验；tag 不存在校验；当前分支与远端同步校验
2. `go vet ./... && go test -race -count=1 ./...`（六段式同口径）
3. `pnpm -C web install --frozen-lockfile && pnpm -C web build`（dist 新鲜——本地验证 embed 链）
4. 长 fuzz：`go test -fuzz=FuzzDecodeHello -fuzztime=10m ./internal/proto/` + `go test -fuzz=FuzzDecodeFileConfig -fuzztime=10m ./cmd/wesh/`（崩溃即中止——语料已落 testdata/fuzz，修复后重跑）
5. 负载矩阵：`go test -tags=load -count=1 -timeout=30m ./internal/server/`
6. 确认闸（回显将创建的 tag 与最近提交）→ `git tag v$V && git push origin v$V` → release.yml 接管

### Anti-Patterns to Avoid

- **goreleaser before.hooks 跑 pnpm build：** 环境隐式（D-03 裁决排除）——workflow 步骤显式编排肉眼可审
- **依赖 goreleaser 默认命名/默认 checksum 名：** v2.18.0 实证两者都不符 D-04/D-02 预期（§Common Pitfalls 1/2）——全部显式钉死
- **fuzz 一把梭（一个 -fuzz 跑全部目标）：** go 工具链结构性拒绝（单包单目标）——脚本逐目标调用
- **把 load 测试跑进常规 CI：** D-11 裁决手动 `-tags=load`；但也禁止忘写 build tag（`//go:build load` 行必须是文件首行，否则常规 CI 会捡起 30min 测试）
- **Caddy 配方抄 nginx 的 Host 行：** 两者默认语义相反（§Common Pitfalls 6）——配方按各自实证写
- **scratch 里 RUN 任何指令：** 无 shell——校验/下载全部放 builder stage 或 ADD --checksum
- **systemd unit 里写凭据：** unit 文件 world-readable——EnvironmentFile= 600 通道（D-17 既定，README 287-295 先例）

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 交叉编译+打包+校验和+Release 发布 | Makefile 循环 GOOS/GOARCH + sha256sum + gh release upload | goreleaser v2.18.0 | 命名模板/校验和/changelog/Release API 的边角（fetch-depth、token 权限、产物上传幂等）全是已解决的坑 |
| WS 帧畸形输入鲁棒性验证 | 手写变异器/随机字节脚本 | Go stdlib testing.F | 覆盖率引导变异 + 自动最小化崩溃用例 + 语料即单测；手写等价物是数千行 |
| 容器 PID 1 收割 | 自写 wait 循环 / bash `wait` | tini v0.19.0 静态二进制 | PITFALLS C8：PID 1 信号默认处置特殊性 + 孤儿收养收割，ttyd 已示范错误答案 |
| 反代 WS 配置 | 手研 nginx/Caddy 每条指令 | 既有 README nginx 配方（G-07-2 实证）+ 本调研 Caddy/CF 配方骨架 | D-15 验证深度已定级（Caddy 实证/CF 标注）；Host/Upgrade/超时三要素各有平台特异 |
| TOML/字节单位解析（index-max-size） | 自写 "16MiB" 单位解析器 | TOML 整数字节（推荐，§Open Questions OQ1） | 为零散键引入单位解析依赖或手写解析都违反项目「零新依赖倾向 + flag 面紧缩」哲学 |
| 发布版本号管理 | 自写 bump 脚本/CHANGELOG 生成 | git tag（D-01）+ goreleaser changelog | 版本史与 tag 同源是 D-01 裁决；个人工具零额外流程 |

**Key insight:** 本 phase 的每个「造轮子」冲动背后都有已裁决的纪律（D-01..D-18）或已实证的库能力；发布工程的最大风险不是缺工具，而是**默认值漂移**（goreleaser v2 改了 checksum 默认名——不核实就会产出不符合 D-02 契约的发布物）。

## Common Pitfalls

### Pitfall 1: goreleaser v2 checksum 默认名不是 `checksums.txt`
**What goes wrong:** 不写 `checksum:` 配置 → v2.18.0 产出 `wesh_1.0.0_checksums.txt`，D-02 契约（仅 checksums.txt）与 README 校验和验证指引不符。
**Why it happens:** D-02「goreleaser 默认产出」的表述停留在 v1 时代认知；v2 改了默认模板。
**How to avoid:** 显式 `checksum: { name_template: checksums.txt }`（实证来源：`internal/pipe/checksums/checksums.go:44-49`——`cs.NameTemplate == ""` 时默认 `"{{ .ProjectName }}_{{ .Version }}_checksums.txt"`）[VERIFIED: gh api 直读源码]。
**Warning signs:** release 产物清单里出现项目前缀的 checksums 文件。

### Pitfall 2: `.Version` 模板变量剥离 `v` 前缀
**What goes wrong:** 用默认 archive 名 → 产物 `wesh_1.0.0_linux_amd64.tar.gz`，与 D-04 命名族 `wesh_v1.0.0_linux_amd64.tar.gz` 差一个 `v`——发布后用户脚本按 D-04 文档写死带 v 的 URL 会 404（D-04 costly 级不可逆点）。
**Why it happens:** goreleaser 源码 `ctx.Version = strings.TrimPrefix(ctx.Git.CurrentTag, "v")`（internal/pipe/git/git.go:58）[VERIFIED: gh api 直读源码]。
**How to avoid:** `name_template: "{{ .ProjectName }}_{{ .Tag }}_{{ .Os }}_{{ .Arch }}"`（`.Tag` 保留原始 tag）；`ldflags -X main.version={{.Version}}` 注入 `"1.0.0"`（`--version` 输出不带 v 是 Go 惯例，可接受；若期望带 v 用 `.Tag`，planner 定稿时二选一写进 plan）。
**Warning signs:** snapshot 构建产物名与 D-04 文档族不一致。

### Pitfall 3: main 包路径默认 `.`
**What goes wrong:** wesh 的 main 包在 `cmd/wesh/`，goreleaser builds 默认 `main: .` → `go build .` 在仓库根找不到 main 包，release 直接失败（v2 schema 与 builds 文档均载明 main 字段默认 `.`）[VERIFIED: gh api schema.json + builds.md 搜索内联内容]。
**How to avoid:** `builds: [{ main: ./cmd/wesh, binary: wesh, ... }]`；发布脚本落地前本地 `goreleaser release --snapshot --clean` 预演（环境审计已有安装通道）。
**Warning signs:** `goreleaser check` 通过但 snapshot 构建报 "no Go files" / "main is not a main package"。

### Pitfall 4: fuzz 每次调用只能匹配单包单目标
**What goes wrong:** CI/发布脚本写 `go test -fuzz='Fuzz.*' -fuzztime=10m ./...` → 工具链直接报错拒绝（不是跑不起来那么简单——regexp 跨包或多目标即拒绝）。
**Why it happens:** `-fuzz` 设计如此（单目标 worker 池）[VERIFIED: 本机 `go help testflag` 逐字]。
**How to avoid:** 两目标两次调用；fuzz leg 独立 job 与 go/web leg 并行。
**Warning signs:** `go test` 输出 "must match exactly one fuzz test"。

### Pitfall 5: CGO 两通道混淆
**What goes wrong:** 发布构建忘了 CGO_ENABLED=0（→ 动态链接 glibc，scp 到干净机器跑不了，OPS-10 SC1 直接失败）；或常规 CI 误设 CGO_ENABLED=0（→ -race 编译失败）。
**Why it happens:** -race 需 cgo（PITFALLS Pitfall 5）；ci.yml:15 注释已固化「CGO_ENABLED=0 只属于发布构建」[VERIFIED: .github/workflows/ci.yml:15]。
**How to avoid:** CGO_ENABLED=0 只出现在 .goreleaser.yml `builds.env`；release.yml 不设该环境变量（goreleaser 自己注入构建环境）；本机验证静态性：`ldd dist/.../wesh` → "not a dynamic executable" + 干净容器运行。
**Warning signs:** `file` 输出含 "dynamically linked"。

### Pitfall 6: nginx/Caddy 的 Host 与超时默认语义相反，配方互抄必错
**What goes wrong:** nginx 默认转发 `Host=$proxy_host`（127.0.0.1:后端口）→ wesh Origin 校验 403（G-07-2 已实证，配方须 `proxy_set_header Host $http_host;`）；Caddy 默认**原样透传 Host**——照抄 nginx 的 Host 行到 Caddy 是多余，照抄 Caddy 的「什么都不配」到 nginx 必 403。超时侧：nginx `proxy_read_timeout` 默认 60s 切断空闲 WS（README 配方已调大）；Caddy 对 hijack 后的 WS 连接**无默认 idle 超时**（http server 超时不再约束被劫持连接）[MEDIUM 置信：websearch+训练，官方文档未能直取——D-15 本机实证兜底]。
**How to avoid:** 两配方按各自实证写；配方内含「反代 idle timeout × wesh ping 5s」关系表（D-15 既定）。
**Warning signs:** 双机全链时 WS 握手 403（Host 错）或连接恰在某整数秒被切（超时错）。

### Pitfall 7: scratch 镜像里没有 shell——僵尸收割实测需要夹具通道
**What goes wrong:** Dockerfile FROM scratch 只 COPY tini+wesh 后，本机实测「PID 1 收割/僵尸残留」（D-16 既定验证项）时发现容器内无任何可 spawn 的命令（无 /bin/bash、无 /bin/sh）——wesh 起 `wesh -- bash` 直接 spawn 失败。
**Why it happens:** wesh 的核心用途是 spawn 用户命令，scratch 零内容与其天然张力。
**How to avoid:** 镜像本体保持纯净（D-16 裁决形态）；**实测夹具走 bind-mount**：`docker run --rm -v /bin:/bin:ro -v /lib:/lib:ro -v /lib64:/lib64:ro wesh-img -- bash -c '<fork 孤儿脚本>'`（宿主动态 bash 三件套只读挂入，镜像不变）；README Dockerfile 节明示「本镜像不含任何可执行命令，-- 后命令须来自挂载或自建 FROM 派生镜像」。PID 1=tini 验证：`docker run --rm img -- ps`（同类挂载）或直接 `docker inspect` + 容器内 /proc/1/comm。
**Warning signs:** 实测第一步就报 spawn 失败——这不是 bug，是 scratch 语义。

### Pitfall 8: tini 动态链接版误入 scratch
**What goes wrong:** ADD `tini-amd64`（glibc 动态版）进 scratch → 容器启动 `no such file or directory`（ld-linux 缺失）。
**How to avoid:** 必须用 `tini-static-amd64`/`tini-static-arm64`（资产清单实证两变体并存）[VERIFIED: gh api repos/krallin/tini/releases/latest 资产列表]；sha256 钉死（§Code Examples）。
**Warning signs:** `docker run` 立即退出，inspect 显示 exit 127 类错误。

### Pitfall 9: --index 错误路径回显文件内容 / 读入无顶
**What goes wrong:** 超 16MiB 报错时把文件头部字节带进错误行（违反启动面红线 D-08）；或 `io.ReadAll` 无顶读入把误指的巨大文件吃光内存。
**How to avoid:** `io.LimitReader(f, max+1)` 读入后按 `len(data) > max` 判定；错误行只含路径 + 类别（不存在/不可读/非常规/超限），走 P3 记录式上报通道（clientOptErr 先例，04-01）；测试以内容探针串反断言（config_test.go 红线测试先例，config.go:247 注释族）。
**Warning signs:** 错误输出里出现 HTML 片段。

### Pitfall 10: systemd 语义与 wesh 退出码的交互误判
**What goes wrong:** wesh 优雅关停（SIGTERM）与会话终结（--once/--exit-when-empty）均以 **255** 退出（OQ1 accept-255 + 07-05）[VERIFIED: STATE.md 决策行「SIGTERM/SIGINT → 1001 广播 → 退出码 255」]；unit 写 `Restart=always` 会在会话终结后立刻复活服务（operator 若期望「会话完即停」则行为意外）；写 `Restart=no` 则崩溃不自愈。
**How to avoid:** 模板取 `Restart=on-failure`（非零退出重启；`systemctl stop` 永不触发重启——systemd 知道自己发起的停止）[MEDIUM：训练+搜索互证，D-17 实机 systemctl 实测兜底]；unit 注释写明 255 交互与 always/on-failure 选型理由（D-18① 的同族语境——README 1001 面板文案已按条件句式通吃两形态）。`KillSignal=` 默认 SIGTERM 即正确（07-05 Shutdown 挂钩）；`TimeoutStopSec=15s` 覆盖 1001 广播 + stall 客户端 Close 内建 5s+5s 上界。
**Warning signs:** 实机 `systemctl stop` 后服务又起来了（Restart 选型错）或 stop 卡 90s（TimeoutStopSec 默认撞上 stall 上界）。

### Pitfall 11: release.yml 漏 `fetch-depth: 0`
**What goes wrong:** 默认浅克隆（depth 1）无 tag 历史 → goreleaser 版本推断/changelog 生成失败或错乱。
**How to avoid:** checkout 步骤显式 `fetch-depth: 0`（官方 README 以 IMPORTANT 标注）[VERIFIED: goreleaser-action README]。
**Warning signs:** goreleaser 日志 "no tags found" / 版本变 snapshot 形态。

### Pitfall 12: darwin 二进制本机不可运行验证
**What goes wrong:** Linux 开发机无法执行 darwin/arm64+amd64 产物——若发布物 darwin 构建有平台特异性缺陷（如 embed 路径），发布后才被发现。
**How to avoid:** 分层验证：linux/amd64 做完整运行验证（干净容器/机器 scp 运行 OPS-10 SC1）；darwin 两产物做 `file` 类型检查 + 解压内容检查（wesh+LICENSE+README）+ CI 编译成功即接受；kqueue 运行时行为有 Phase 1 CI macos leg 常规回归背书（ci.yml darwin leg 既有）[VERIFIED: ci.yml:7]。macOS runner 冒烟（`--version`）可加进 release.yml 但超出 D-03 最小面——登记 §Open Questions OQ3。
**Warning signs:** 无——这是验证覆盖面的明示取舍，非缺陷。

## Code Examples

### .goreleaser.yml（定稿推荐——全部键名经 v2.18.0 schema 核实）

```yaml
# Source: gh api repos/goreleaser/goreleaser/contents/www/static/schema.json（键名核实）
#         + internal/pipe/{archive,checksums,git} 源码默认行为核实
version: 2
project_name: wesh

builds:
  - main: ./cmd/wesh          # Pitfall 3：默认 "." 必败
    binary: wesh
    env:
      - CGO_ENABLED=0         # Pitfall 5：只出现在这里（ci.yml -race 通道不设）
    goos: [linux, darwin]     # OPS-10：PROJECT 平台边界（Windows 终局不做）
    goarch: [amd64, arm64]
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{.Version}}   # 挂 cmd/wesh/main.go:32 `var version = "dev"`
    mod_timestamp: "{{ .CommitTimestamp }}"  # 可复现构建

archives:
  - formats: [tar.gz]         # v2 schema：formats 为列表键；name_template 单数
    # D-04 命名族 wesh_v1.0.0_linux_amd64.tar.gz：.Tag 保留 v（Pitfall 2）
    name_template: "{{ .ProjectName }}_{{ .Tag }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE               # 已实证存在于仓库根
      - README.md

checksum:
  name_template: checksums.txt  # Pitfall 1：v2.18.0 默认是项目前缀形态，D-02 须显式钉

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - "^ci:"
      - "^style:"
```

### .github/workflows/release.yml（D-01/D-03 定稿推荐）

```yaml
# Source: gh api repos/goreleaser/goreleaser-action/contents/README.md（官方 workflow 示例）
#         + 本仓 ci.yml 钉版逐行复用（D-03）
name: release
on:
  push:
    tags: ["v*"]              # D-01：v* tag 触发，起点 v1.0.0

permissions:
  contents: write             # GITHUB_TOKEN 创建 Release 最小授权（官方 README）

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7.0.1     # 与 ci.yml 同钉版
        with:
          fetch-depth: 0                  # Pitfall 11：changelog/tag 推断前提
      # D-03 显式编排：pnpm 先于 goreleaser（逐行复用 ci.yml web leg）
      - uses: pnpm/action-setup@v6.0.10
        with:
          version: 11.21.0
      - uses: actions/setup-node@v4
        with:
          node-version: 24
      - run: pnpm -C web install --frozen-lockfile
      - run: pnpm -C web build            # tsc && vite build && gzip -k9（web/package.json 实证）
      - uses: actions/setup-go@v7.0.0
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v7.2.3
        with:
          distribution: goreleaser
          version: "~> v2.18.0"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### fuzz 目标（D-09/D-10 定稿形态）

```go
// internal/proto/fuzz_test.go —— package proto_test
// Source: 本机 go 1.26.3 fuzz 冒烟实证 + proto.go:136/203 签名核实
func FuzzDecodeHello(f *testing.F) {
	// 种子选型（Claude's Discretion 定稿建议）：合法 / 畸形 / 边界尺寸 / 空 / 未知字段
	f.Add([]byte(`{"version":"wesh.v1","cols":80,"rows":24}`))
	f.Add([]byte(`{"version":"wesh.v1","cols":-1,"rows":999999,"ticket":"x"}`))
	f.Add([]byte(`{"version":`))  // 截断
	f.Add([]byte{})               // 空载荷（server 侧空帧另有 len 守卫）
	f.Add([]byte(`{"version":"wesh.v1","cols":1e999,"rows":true}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		hp, ok := proto.DecodeHello(data)
		if !ok {
			return
		}
		// 不变量：成功解码 ⇒ 尺寸恒在 [1,1000]（ClampDim 契约 proto.go:212）
		if hp.Cols < 1 || hp.Cols > 1000 || hp.Rows < 1 || hp.Rows > 1000 {
			t.Fatalf("ClampDim broken: %+v from %q", hp, data)
		}
	})
}
```

```go
// cmd/wesh/fuzz_test.go —— package main
// 前置接缝（config.go）：loadFileConfig 提取 reader 委托，错误分类/configErr 单写口不变
// func decodeFileConfig(path string, r io.Reader) (*fileConfig, error)
func FuzzDecodeFileConfig(f *testing.F) {
	f.Add([]byte("port = 7681\nbind = \"127.0.0.1\"\n"))
	f.Add([]byte("credential = [\"FUZZ_PROBE_SECRET:x\"]\n"))  // 值剥离红线探针
	f.Add([]byte("unknown-key = 1\n"))                        // 未知键拒绝面
	f.Add([]byte("port = \"not-a-number\"\n"))                // 类型不符面
	f.Add([]byte{0xff, 0xfe, 0x00})                           // 非 UTF-8/二进制
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := decodeFileConfig("fuzz.toml", bytes.NewReader(data))
		if err == nil {
			return
		}
		// 不变量：错误文本绝不含探针值（config.go 头注释值剥离红线的 fuzz 断言形态；
		// 键名回显合法不在断言面——只断言值探针）
		if strings.Contains(err.Error(), "FUZZ_PROBE_SECRET") {
			t.Fatalf("value red line broken: %v", err)
		}
	})
}
```

### 负载测试骨架（D-11/D-12）

```go
//go:build load

// internal/server/load_test.go —— package server_test（全仓统一外部包形态）
// 运行：go test -tags=load -count=1 -timeout=30m ./internal/server/
// 形态：复用 startTrackedServerWith（e2e_test.go:171）+ stall 夹具纪律（slowclient_test.go）
// 数据源：/metrics 黑盒 scrape（metrics_test.go 先例）+ runtime.NumGoroutine/ReadMemStats
//
// 矩阵（README §默认参数与 Phase 9 标定 既定方法论实例化）：
//   客户端数 {1,4,16,32} × 输出速率 {seq 洪水, 匀速滴漏, 突发} × 慢链路 {stall 端, 限速合法读者}
// 断言（D-12 验收）：
//   1. 合法慢端零误踢——限速读者全程 wesh_clients_kicked_total 不增
//   2. 内存上界——32 端洪水 ReadMemStats Alloc 平稳（账面 32×512KiB + 共享帧）
//   3. 门频率——wesh_credit_gate_transitions_total 速率不震颤（50% 半水位迟滞）
// 高频建销（defunct 三面，Linux-only）：
//   N 轮 spawn+即退子进程后 NumGoroutine/fd 计数回基线 + /proc/<child>/stat 无 Z 态
```

### Dockerfile（D-16 定稿推荐）

```dockerfile
# syntax=docker/dockerfile:1
# D-16：scratch + 静态二进制 + tini 作 PID 1；不发布镜像（用户自建）
# 构建前置：CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wesh ./cmd/wesh
FROM scratch
ARG TARGETARCH=amd64
# tini v0.19.0 sha256 钉值（升级 tini 即同步改；[VERIFIED: krallin/tini release .sha256sum]）
#   amd64: c5b0666b4cb676901f90dfcb37106783c5fe2077b04590973b885950611b30ee
#   arm64: eae1d3aa50c48fb23b8cbdf4e369d0910dfc538566bfd09df89a774aa84a48b9
ARG TINI_SHA256=c5b0666b4cb676901f90dfcb37106783c5fe2077b04590973b885950611b30ee
ADD --checksum=sha256:${TINI_SHA256} \
    https://github.com/krallin/tini/releases/download/v0.19.0/tini-static-${TARGETARCH} /tini
COPY wesh /wesh
EXPOSE 7681
# tini 默认只向直接子进程（wesh）转发信号——正确形态：wesh 自管 stop-signal 进程组序列
#（D-22），-g 会双重信号。孤儿孙进程由 PID 1=tini 收割（PITFALLS C8）。
ENTRYPOINT ["/tini", "--", "/wesh"]
# 注意（README 承诺语素材）：本镜像不含任何可执行命令——`--` 后命令须来自 bind-mount
# 或 FROM 派生自建；--socket 在容器内需配合 volume 暴露给宿主反代。
# arm64 构建：docker build --build-arg TARGETARCH=arm64 --build-arg TINI_SHA256=eae1d3aa... .
```

### deploy/wesh.service（D-17 定稿推荐）

```ini
# D-17：Restart=/LimitNOFILE=/EnvironmentFile=600 全配；实机 systemctl 通道最小实测（P8 同通道）
# Source: 训练+搜索互证（MEDIUM）+ README 287-295 既有 systemd 配方先例 + STATE 255 语义实证
[Unit]
Description=wesh — share terminal over web
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
# 凭据通道：chmod 600，内容 WESH_CREDENTIAL=user:pass（- 前缀 = 缺席不拒；README 既定先例）
EnvironmentFile=-/etc/wesh/credentials
ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml
# 255 交互（Pitfall 10）：wesh 优雅关停/会话终结恒退出 255——on-failure 下自主终结会重启
#（服务形态期望）；systemctl stop 永不触发重启。期望「会话完即停」则改 Restart=no。
Restart=on-failure
RestartSec=2
# stall 客户端 Close 内建 5s+5s 上界 + 1001 广播余量
TimeoutStopSec=15s
LimitNOFILE=65536
# 加固张力（README 说明义务）：wesh 的本职是 spawn 用户 shell——ProtectHome=/ProtectSystem=strict
# 会破坏 shell 工作流，不做默认加固；operator 可按需自行叠加（NoNewPrivileges= 安全，可启）。
# KillSignal 默认 SIGTERM 即正确（07-05 Shutdown 1001 广播挂钩）。

[Install]
WantedBy=multi-user.target
```

### Caddy / Cloudflare 配方骨架（D-15；README 部署节落点）

```caddyfile
# Caddy v2.11.4 实证对象（D-15 本机实证 + 双机全链，断言面=页面+WS 全链同 G-07-2）
wesh.example.com {
    # WS upgrade 自动处理（reverse_proxy 内建）；Host 默认原样透传（与 nginx 相反——
    # Pitfall 6，Origin 校验天然通过）；X-Forwarded-For 默认添加（--auth-header 可选）
    reverse_proxy 127.0.0.1:7681
    # hijack 后无 server 超时约束，无默认 WS idle 超时（MEDIUM——实证兜底）；
    # 空闲关系表：wesh --ping-interval 默认 5s ≪ 任何中间盒 idle 阈值
}
```

```text
# Cloudflare（D-15：按官方文档写 + 标注「未实测」）
- DNS 记录开橙云代理；WebSockets 支持默认开启（Network 面板）
- 空闲超时：社区共识 ~100s 无流量关连（官方文档本会话未能直取——MEDIUM/ASSUMED）；
  wesh 默认 ping 5s ≪ 100s，关系表结论「默认即安全」
- TLS：边缘终止；源站推荐 Full (strict)（wesh --tls-cert/--tls-key）或 loopback-only
- Host 默认保持；/s/{token}/ URL 对 CF 是明文可见第三方面（README 脱敏建议的 CF 语境延伸）
```

### D-18 三项修复（09-UI-SPEC R1-R3 逐字定稿）

```typescript
// web/src/main.ts —— R1：新常量单写口（HINT_RESTART 同族先例，main.ts:433 旁）
const HINT_SHUTDOWN =
  'If wesh is not restarted for you, start it again from your shell, then';
// R3：onclose 分派顺序——if (!opened) 分支（main.ts:881-884）之前先分派 1001：
//   ev.code === 1001 → showStatus('Server shutting down',
//     'The wesh server is shutting down. The session has ended.', HINT_SHUTDOWN)
//   （与稳态 case 1001（main.ts:895-905）完全同一 showStatus 调用形态——单写口纪律）
// R1：case 1001 的 hintPrefix 由 'Start wesh again from your shell, then' 改为 HINT_SHUTDOWN
```

```html
<!-- web/index.html:63 —— R2：单属性，零视觉影响 -->
<div id="status" role="alert" hidden>
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| goreleaser v1（`archives.format` 单值、裸 checksums.txt 默认） | goreleaser v2（`version: 2` 配置、`formats` 列表、`{{.ProjectName}}_{{.Version}}_checksums.txt` 默认） | v2 线 2024 起；v2.18.0 实证现状 | 本仓从零起步直接 v2 形态；D-02/D-04 两契约点必须显式钉名（Pitfall 1/2） |
| goreleaser-action v6 | v7（当前 v7.2.3） | 2026-06 | `@v7.2.3` 钉死；自动校验下载包 checksums |
| 手写 fuzz 变异脚本 | Go native testing.F（覆盖率引导+自动最小化+语料即单测） | Go 1.18+ 成熟，1.26.3 本机实证 | D-09 零新依赖兑现；CI 短跑+发布长跑双层是社区标准形态 |
| docker 镜像内置完整 init 系统 | tini 静态单文件（或 `docker run --init` 外置） | tini v0.19.0（2020，稳定） | scratch ADD 一步解决 C8；wesh 无 `-g` 需求（自管进程组信号） |
| nginx 唯一反代配方 | nginx（已实证）+ Caddy（本 phase 实证）+ Cloudflare（标注未实测）三配方分级 | 本 phase | D-15 验证深度分级先例：实证/未实测诚实分级 |

**Deprecated/outdated:**
- goreleaser v1 配置形态：本仓不适用（从零 v2），但 WebSearch 命中的教程多为 v1 形态（`format:` 单值）——planner 参照教程时以本调研 schema 实证为准
- `tini-amd64`（动态链接变体）：scratch 场景已废弃用法——必须 `tini-static-*`（Pitfall 8）

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Caddy hijack 后 WS 连接不受 http server 超时约束、默认无 WS idle 超时 | Pattern/Pitfall 6/配方骨架 | 配方超时表数值偏差——D-15 本机实证（双机全链 + 空闲观测）兜底；wesh ping 5s 提供大余量 |
| A2 | Cloudflare WS 空闲 ~100s 关连（社区共识；官方文档被网络策略阻断未能直取） | 配方骨架 | 数值不准——D-15 既定「未实测」标注；ping 5s ≪ 100s 余量两个数量级，结论「默认即安全」对该数值不敏感 |
| A3 | tini 默认只向直接子进程转发信号（`-g` 才扩到进程组） | Dockerfile 注释 | 信号语义偏差——D-16 本机 docker 实测（既定验证项）兜底；wesh 自管 stop-signal 进程组序列，默认形态即使偏差也可经 `-g` 修正 |
| A4 | index-max-size TOML 值形态推荐整数字节（非 "16MiB" 串） | Don't Hand-Roll/OQ1 | 形态选择偏差——实现细节非行为契约；planner 定稿时若选串形态需自写解析（已在 OQ1 登记权衡） |
| A5 | systemd Restart=on-failure 对 exit 255 分类为 failure、systemctl stop 永不触发重启 | Pitfall 10/unit 模板 | Restart 选型偏差——D-17 实机 systemctl 通道最小实测兜底；训练+搜索互证 MEDIUM |
| A6 | ADD --checksum 在本机 docker 24.0.6（buildx 0.11.2）可用（需 `# syntax=docker/dockerfile:1` 拉取 1.6+ frontend） | Dockerfile | 构建失败——D-16 本机构建实测兜底；退路 = alpine builder stage 下载+sha256sum -c（§Open Questions 未列，实现期两分钟即可切换） |
| A7 | goreleaser v1 的裸 checksums.txt 默认（D-02 表述的历史来源） | Pitfall 1 | 无行动影响——v2.18.0 实证默认已变，显式钉名与默认漂移解耦 |

## Open Questions (RESOLVED)

1. **index-max-size TOML 值形态** — **RESOLVED**: 采纳 Recommendation「整数字节」，落点 09-04 Task 2（fileConfig.IndexMaxSize *int 纯配置键、validateStartup ≤0 拒绝）与 09-09 Task 2 ③（README 配置节写明单位字节）。
   - What we know: D-08 只锁定「TOML 配置键可调、不开 CLI flag」；现有 duration 键为字符串形态（ping-interval="5s"）、计数/尺寸键为整数（max-clients=16）
   - What's unclear: 字节尺寸类键取整数（`index-max-size = 33554432`，零新解析、go-toml 原生 int）还是单位串（`"32MiB"`，需自写解析器——违零新依赖倾向）
   - Recommendation: **整数字节**（与 TOML 类型系统一致、零新代码、P2 D-15 flag 面紧缩哲学延伸）；README 写明单位字节；校验 `> 0`（负/零 exit 2 落 validateStartup 矩阵）

2. **Cloudflare idle timeout 官方数值直取** — **RESOLVED**: 按 Recommendation 既定执行——官方文档写 + 显著「未实测」标注（ping 5s 余量使结论不敏感），落点 09-09 Task 2 ④ Cloudflare 配方节。
   - What we know: 社区共识 ~100s；WebFetch 被安全中心网络策略阻断、Context7 配额耗尽，本会话无法读 developers.cloudflare.com 原文
   - What's unclear: 官方当前表述与 Enterprise 可调性
   - Recommendation: 按 D-15 既定执行（官方文档写 + 标注未实测）；执行期若网络条件允许补一次原文核实，不阻塞（ping 5s 余量使结论不敏感）

3. **darwin 产物运行冒烟是否进 release.yml** — **RESOLVED**: 采纳 Recommendation「不加」（D-03 显式编排面最小 + ci.yml darwin leg 常规回归背书），落点 09-01 Task 2（release.yml 单 ubuntu leg，无 macOS smoke leg）；macOS 冒烟留作 09-VALIDATION Manual-Only 登记项。
   - What we know: D-03 最小面只有 ubuntu leg；darwin 二进制 Linux 本机不可运行；macos-latest runner 跑 `wesh --version` 冒烟成本 ~1min
   - What's unclear: 是否值得为 darwin 产物加 macOS smoke leg（超出 D-03 字面范围）
   - Recommendation: 不加（D-03 显式编排面最小 + Phase 1 起 ci.yml darwin leg 常规回归背书）；产物检查（file/解压内容/校验和）覆盖发布物形状——登记为可接受的验证取舍，若用户要求再加

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | 构建/fuzz/负载测试 | ✓ | 1.26.3 | — |
| node | 前端构建 | ✓ | 24.13.0 | — |
| pnpm | 前端构建 | ✓ | 11.21.0 | — |
| docker | D-16 Dockerfile 本机构建实测 | ✓ | 24.0.6（buildx 0.11.2） | — |
| gh | 版本核实/发布辅助 | ✓ | 2.97.0 | 非必需（tag push 即触发） |
| systemctl | D-17 实机最小实测（P8 同通道） | ✓ | systemd 239 | — |
| nginx | 既有配方（README 已实证） | ✓ | 1.14.1 | — |
| goreleaser | 本机预演（`check`/`--snapshot`） | ✗ | — | `go install github.com/goreleaser/goreleaser/v2@v2.18.0` 或官方 release 二进制（CI 中由 goreleaser-action 自动装，本机仅预演需要） |
| caddy | D-15 本机实证 | ✗ | — | GitHub release v2.11.4 静态二进制直装（Claude's Discretion 既定形态；CODEBUDDY 禁 apt 纪律不涉服务端软件） |
| tini | Dockerfile 构建期 | host ✗（不需要） | 镜像内 v0.19.0 | 构建期 ADD（sha256 钉死），宿主无需安装 |
| Windows 工作站 + Playwright | D-15 Caddy 双机全链浏览器半侧 | 假设可用（双机拓扑既定，web/uat/pw/ 先例） | — | 用户侧执行；不可达则 Caddy 配方降级为「本机协议层实证 + 标注浏览器半侧未测」（风险升级需用户裁决） |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:**
- goreleaser（本机预演用途）→ go install / 官方二进制两通道
- caddy（D-15 实证）→ 官方二进制直装；完全不可装则配方降级标注（需用户裁决，不建议）

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib testing + `-race`（单测/黑盒 e2e）；testing.F（fuzz）；Node 原生 WS/fetch 零依赖 .mjs（协议 UAT）；jsdom 25（前端 DOM）；Playwright（Windows 浏览器实测层） |
| Config file | 无独立配置（go test 直驱；jsdom 经 web/uat/pnpm 既有装配） |
| Quick run command | `go test -count=1 ./internal/proto/ ./cmd/wesh/`（fuzz 种子语料随之单测化运行） |
| Full suite command | `go vet ./... && go test -race -count=1 ./... && pnpm -C web build` + 全量 UAT 脚本回归（六段式先例） |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OPS-03 | --index 启动校验四拒绝（不存在/不可读/非常规/超限）exit 2 + 错误行零内容 | unit | `go test -run 'TestParseArgs\|TestStartupMatrix\|TestLoadFileConfig' ./cmd/wesh/` | ❌ Wave 0（扩既有表） |
| OPS-03 | index / index-max-size TOML 键（合并/优先级/未知键拒绝面） | unit | `go test -run 'TestLoadFileConfig\|TestConfigMerge' ./cmd/wesh/` | ❌ Wave 0（扩 config_test.go） |
| OPS-03 | 自定义字节经 / 与 /s/{token}/ 统一伺服 + gzip/Vary | Go e2e | `go test -run 'TestCustomIndex' ./internal/server/` | ❌ Wave 0 |
| OPS-03 | 协议层全链：spawn --index 实例断言双通道给页 + 内建资源 404 + /api/attach /ws 照旧 | 协议 UAT | `node web/uat/phase09.mjs` | ❌ Wave 0 |
| OPS-10 | goreleaser 配置合法 + snapshot 产出四二进制+tar.gz+checksums.txt | 本机预演 | `goreleaser check && goreleaser release --snapshot --clean` | ❌ Wave 0（.goreleaser.yml） |
| OPS-10 | linux/amd64 产物静态性 + 干净环境可运行 | 二进制直证 | `ldd` 非动态 + `docker run --rm -v $PWD/wesh:/wesh debian:stable-slim /wesh --version`（docker 本机可用） | ❌ Wave 0（b 系直证脚本） |
| OPS-10 | release.yml tag 触发全链 | CI | 发布脚本最后一步实跑一次（v1.0.0 即首证） | ❌ Wave 0（release.yml） |
| D-09/10 | fuzz 两目标不 panic + 红线不变量 | fuzz | `go test -fuzz=FuzzDecodeHello -fuzztime=60s ./internal/proto/` 等（两次调用） | ❌ Wave 0 |
| D-11/12 | 负载矩阵三断言 + defunct 三面 | load | `go test -tags=load -count=1 -timeout=30m ./internal/server/` | ❌ Wave 0 |
| D-18② | #status role="alert" | jsdom | `node web/uat/phase06-dom.mjs`（扩断言） | 既有文件扩展 |
| D-18①③ | C-10 hint 逐字 + pre-onopen 1001 分派 | jsdom | `node web/uat/phase06-dom.mjs`（扩场景） | 既有文件扩展 |
| D-16 | Dockerfile 构建 + PID 1=tini + 僵尸收割 | 本机实测 | `docker build` + bind-mount 夹具（Pitfall 7 形态） | ❌ Wave 0（Dockerfile） |
| D-17 | unit 模板语法 + 实机 systemctl 最小实测 | 实机 | `systemd-analyze verify deploy/wesh.service` + systemctl 通道 | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -count=1 ./<touched-pkg>/` + 涉及前端时 `pnpm -C web build`
- **Per wave merge:** `go test -race -count=1 ./...` + 相关 UAT 脚本（phase09.mjs / phase06-dom.mjs 扩展面）
- **Phase gate:** 全量六段式 + 全量 UAT 回归 + fuzz 短跑 × 2 + load 矩阵 + goreleaser snapshot 预演 全绿，方进 `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/proto/fuzz_test.go`（FuzzDecodeHello/FuzzDecodeResize + 种子语料）
- [ ] `cmd/wesh/fuzz_test.go`（FuzzDecodeFileConfig——前置 config.go reader 委托接缝重构）
- [ ] `internal/server/load_test.go`（`//go:build load` + 矩阵夹具）
- [ ] `internal/proto/testdata/fuzz/`、`cmd/wesh/testdata/fuzz/`（崩溃语料入库点——首跑后视发现入库）
- [ ] `web/uat/phase09.mjs`（OPS-03 协议层 UAT）
- [ ] `.goreleaser.yml`、`.github/workflows/release.yml`、`Dockerfile`、`deploy/wesh.service`、`scripts/release.sh`
- [ ] ci.yml `fuzz` leg 新增（D-10 短跑门）
- [ ] 无框架安装需求——全部既有栈

## Security Domain

> security_enforcement: true（config.json），ASVS level 1

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no（认证面无改动） | —（--index 不改变认证姿态：/api/attach、/ws、share token 门禁照旧——09-UI-SPEC §7 契约行） |
| V3 Session Management | no | — |
| V4 Access Control | no（自定义页是 operator 启动期配置，非用户输入面） | — |
| V5 Input Validation | **yes** | --index 四拒绝校验矩阵（exit 2 fail-fast，parse/validate 分层既定位）；index-max-size 正整数校验；fuzz 覆盖 proto 帧解码（WS 远程输入面）与 TOML 解析（本地配置面）；错误行零值内容（P3/P4 红线延伸，fuzz 探针断言形态） |
| V6 Cryptography | no（checksums.txt 是完整性制品非密码学设计；sha256 由 goreleaser 内建） | — |
| V10 Configuration/供应链（L1 引申） | **yes** | Action 全钉版（@v7.0.1/@v7.0.0/@v6.0.10/@v7.2.3）；goreleaser-action 自校验下载包 checksums；tini sha256 钉死；permissions: contents: write 最小授权限 release job；D-02 裁决仅 checksums.txt（无签名/SBOM——威胁模型已裁决接受） |

### Known Threat Patterns for {发布链 + 自定义页伺服}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 供应链投毒（Action/tool 下载） | Tampering | 官方 org 钉版 + goreleaser-action 内建 checksum 校验 + tini sha256 钉值 + pnpm --frozen-lockfile（既有） |
| 发布物与仓库漂移（脏树打 tag） | Tampering | 发布脚本 worktree 干净校验 + tag 只在脚本确认闸后创建（D-14） |
| 自定义页 XSS 面扩大 | Tampering/XSS |  operator 自有内容 + CSP 现状同源（script-src 'self' 'unsafe-inline' 允许内联、connect-src 'self' 允许同源 WS——09-UI-SPEC §5 实证）；wesh 零注入零模板（D-05 结构防线）；README 承诺「自包含单 HTML」义务 |
| 启动错误回显文件内容（信息泄露） | Information Disclosure | D-08 红线：错误行只含路径+类别；fuzz 探针断言 + 单测反断言双锁 |
| GITHUB_TOKEN 权限过大 | Elevation of Privilege | workflow 级 permissions 只开 contents: write；ci.yml 不开（现状） |
| release.yml 被 push 触发面滥用 | Tampering | 仅 `tags: ["v*"]` 触发；tag 创建权 = 仓库写权限（个人仓库威胁模型 D-02 已裁决） |

## Sources

### Primary（一手直读，本会话实证）

- **gh api repos/goreleaser/goreleaser**：releases/latest（v2.18.0, 2026-08-24）；`internal/pipe/git/git.go:58`（.Version TrimPrefix "v"）；`internal/pipe/archive/archive.go:30-32`（默认名模板）；`internal/pipe/checksums/checksums.go:44-49`（checksum 默认名）；`www/static/schema.json`（Archive/Build/Checksum 键名核实）
- **gh api repos/goreleaser/goreleaser-action**：releases/latest（v7.2.3, 2026-06-29）；action.yml（默认 version '~> v2'）；README.md（官方 workflow/tag 触发/fetch-depth/permissions/自动校验节）
- **gh api repos/krallin/tini**：releases/latest（v0.19.0）；资产清单（tini-static-amd64/arm64 + .sha256sum）；release 下载取回 sha256 钉值两枚
- **gh api repos/caddyserver/caddy**：releases/latest（v2.11.4, 2026-06-03）
- **gh api repos/actions/{checkout,setup-go,setup-node}、pnpm/action-setup**：releases/latest（v7.0.1/v7.0.0/v7.0.0/v6.0.10）
- **本机 go 1.26.3 实证**：fuzz 冒烟（-fuzztime/种子语料单测化/CGO_ENABLED=0 fuzz/~50 万 execs/s）；`go help testflag`（-fuzz 单包单目标约束逐字）；`go list -m -versions`（goreleaser/v2 proxy 版本）
- **本仓 Read 实证**：cmd/wesh/main.go:32（version 注入点）/config.go:97-137（loadFileConfig+DisallowUnknownFields:104）；web/embed.go:24-72（Handler/Vary/acceptsGzip）；internal/server/server.go:278/287/449/464/500/585/1345；internal/server/clients.go:33-66（六常量）/afterDrain（cap/2 水位）；internal/server/sharetoken.go:87-96；internal/proto/proto.go:74-76/136/203/212；internal/server/metrics.go:122-145；internal/server/headers.go（CSP）；internal/server/{e2e,slowclient,limits,metrics}_test.go（夹具形态）；web/src/main.ts:429-433/881-925；web/index.html:63；.github/workflows/ci.yml；.gitignore；go.mod；web/package.json；README.md:180-300（脱敏/nginx/标定表/systemd 先例）

### Secondary (MEDIUM confidence)

- WebSearch（跨多源互证）：Caddy reverse_proxy WS 自动 upgrade/Host 透传/超时行为（caddyserver 文档镜像站 + raybyte.cn 两生产案例）；Cloudflare WS ~100s 空闲关连（websocket.org 指南 + 多社区源）；systemd Restart=on-failure vs always 语义（多教程源 + 训练互证）；goreleaser v2 配置示例（官方 builds.md GitHub 内联搜索内容 + 多项目 .goreleaser.yml 实例）

### Tertiary (LOW confidence)

- tini 信号转发默认语义（-g 进程组）：训练知识，krallin/tini README 未能直取——D-16 本机实测兜底（A3）
- goreleaser v1 裸 checksums.txt 默认：训练知识（A7，无行动影响）
- Dockerfile ADD --checksum 在本机 buildx 的可用性：训练知识（A6，实测兜底）

**工具受限登记：** Context7 月配额耗尽（resolve-library-id 两次调用均拒）；WebFetch 被安全中心网络策略全域阻断——全部官方文档核实改经 `gh api` 直读官方仓库源码/README/schema 完成（证据强度等同一手）。

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH（全部版本 gh api/proxy 一手核实；配置键名 schema+源码双核实）
- Architecture: HIGH（发布链/fuzz/负载形态实证）；MEDIUM（Caddy/CF 行为面——实证兜底既定）
- Pitfalls: HIGH（Pitfall 1-5/8/9/11/12 一手实证）；MEDIUM（Pitfall 6/7/10 行为语义——实证兜底既定）

**Research date:** 2026-08-29
**Valid until:** 2026-09-28（goreleaser/Action 版本面 30 天有效；行为语义面长期稳定）
