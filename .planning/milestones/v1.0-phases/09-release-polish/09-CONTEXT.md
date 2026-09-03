# Phase 9: 发布与打磨 - Context

**Gathered:** 2026-08-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 9 以单静态二进制四平台发布收尾：goreleaser 产出 linux/darwin × amd64/arm64 全静态二进制（CGO_ENABLED=0、前端单 HTML embed、tar.gz 带 LICENSE/README、仅 checksums.txt）；自定义首页 `--index`（ttyd -i 同款整页替换语义）；负载/模糊测试回填 P2/P5/P6/P7 挂账默认参数（验证为主、证伪才改）；部署文档补齐 Caddy/Cloudflare 反代配方 + Dockerfile + systemd unit 模板；07 三项 UI WARNING 随 ship 清零。

**In scope (from ROADMAP):** OPS-10（单静态二进制四平台发布、前端 embed 单 HTML）、OPS-03（自定义首页 HTML 可配置生效）；负载/模糊测试（高吞吐 fan-out、慢客户端矩阵、百万小帧/空帧、高频建销会话无 defunct）回填 P2/P5 默认参数；部署文档（nginx 已有实证配方 + Cloudflare/Caddy 新增 + Docker tini/PID 1 + systemd unit Restart/LimitNOFILE/EnvironmentFile 600）；07 deferred 三 UI WARNING 清零（1001 hint 条件化 / #status role="alert" / pre-onopen 1001 分派）。

**Out of scope (本阶段不做):** cosign 签名/SBOM（D-02 裁决仅 checksums）；Docker 镜像发布 ghcr.io（D-12 裁决 Dockerfile 入库不发镜像）；Windows 平台（PROJECT 终局不做）；滚动回放/会话保持（PROJECT 锁定）；ZMODEM/trzsz/Sixel（v2）；挂账参数可配化（D-16 纪律：证伪只改常量默认值，不动 flag 面）；配置文件热重载（无需求）。

**已锁定不重复决策：** embed 链 pnpm build 先于 go build、dist 真实产物入库、裸 clone 可编译（P1 D-18）；`version = "dev"` ldflags 注入点已就位（main.go:32）；CI 注释明示 CGO_ENABLED=0 只属于发布构建（-race 需 cgo，ci.yml 两通道并存）；CLI flag 全名无短选项 + parse/validate 分层 + fail-fast + 敏感值记录式上报（P2 D-15/P3）；配置文件 TOML 平铺键 = flag 同名、未知键拒绝（P7 D-03/D-06）；凭据/token 永不入日志（SEC-01/P5 D-03）；文档即被测物（G-07-2 nginx 配方双机实证先例）；测试分层策略与双机拓扑（CODEBUDDY.md：Linux 侧协议/DOM UAT，Windows 侧 Playwright，禁装顺序不变）；退出码 255 语义（P6 OQ1）；标定挂账清单（README §默认参数与 Phase 9 标定：outbox 512KiB / 水位 50% / input 32KiB/s÷64KiB / max-clients 32 / resize 50ms / attachGrace 500ms / pong 10s / hello 5s / EXIT 2s / stop-timeout 0 / exit-when-empty 宽限）。

</domain>

<decisions>
## Implementation Decisions

### 发布流程与版本策略（OPS-10）
- **D-01:** 发布触发 = **tag push 自动触发**：推 `v*` tag 触发独立 release.yml，版本号起点 v1.0.0——版本史与 git tag 同源，个人运维工具零额外流程负担
- **D-02:** 供应链文件 = **仅 checksums.txt**（goreleaser 默认产出）——无 cosign 签名/SBOM；个人运维工具用户自编译成本低、供应链威胁模型弱，零密钥管理负担
- **D-03:** 前端产物固化 = **release.yml workflow 步骤显式编排**：pnpm install+build（钉 pnpm 11.21.0/node 24，与 ci.yml web leg 同版）→ setup-go → goreleaser——构建顺序肉眼可审，不用 goreleaser before hooks（环境隐式），不用入库 dist（发布产物可能滞后 web/src 未构建提交）
- **D-04:** 打包形态 = **tar.gz 含 wesh + LICENSE + README.md**（`wesh_v1.0.0_linux_amd64.tar.gz` 命名族）——解压即见文档，scp 单文件也行 — **Reversibility:** costly — 发布后用户脚本/文档会引用产物文件名格式，改形态破坏既有下载锚点

### 自定义首页（OPS-03）
- **D-05:** 语义 = **整页替换（ttyd -i 同款）**：用户 HTML 完全替代首页，终端功能由用户页面自行实现（WS 端点照旧暴露），wesh 只负责伺服——零模板注入面，拒绝片段注入（CSP/拼接复杂度） — **Reversibility:** one-way — 行为契约：README 公开承诺后改语义破坏已部署自定义页用户的预期
- **D-06:** 给页通道 = **全通道统一替换**：根路径与 `/s/{token}/` 都伺服自定义页（单一给页源零双轨）；README 明示「自定义页需自行实现终端逻辑，否则分享链接失去终端功能」 — **Reversibility:** one-way — 同上行为契约
- **D-07:** flag 形态 = **`--index /path/to.html` 启动一次读入内存**（不存在/不可读/非常规文件 exit 2 fail-fast）；运行期零磁盘依赖，改文件需重启生效（与 embed 静态伺服同语义）；入 TOML 配置同名键（P7 D-04 覆盖面先例） — **Reversibility:** one-way — CLI flag 公开契约（P2 D-15 纪律）
- **D-08:** 大小上限 = **默认 16MiB 硬顶**（io.LimitReader 启动读入，超限 exit 2 报错行只含路径不含内容——启动面红线）+ **TOML 配置键 `index-max-size` 可调**；**不开 `--index-max-size` CLI flag**——P2 D-15「flag 不轻易新增」与「配置可调」（用户裁决原话）的调和：配置文件承载低频调参，纯配置键无对应 flag 是 P7 D-03 纪律的明示例外，README 写明

### 负载/模糊测试
- **D-09:** fuzz 目标面 = **proto 帧解码（DecodeHello/帧拆分）+ TOML 配置解析**两处——WS 远程输入面与本地配置文件解析面，Go native fuzz 零新依赖
- **D-10:** fuzz 运行形态 = **CI 短跑 1-2min 回归门 + 发布前长跑 10min+**；崩溃语料 `testdata/fuzz/` 入库永久回归；长跑纳入发布脚本（D-14）
- **D-11:** 负载测试载体 = **Go 黑盒负载测试**（internal/server，build tag 隔离不进常规 CI，手动 `-tags=load` 跑）：coder/websocket 客户端性能远超 Node 单线程；runtime.NumGoroutine/ReadMemStats 上界断言直接；与既有夹具同语言同库
- **D-12:** 标定回填纪律 = **验证为主、证伪才改**——默认验证现值成立（README 既定验收：合法慢端零误踢 + 内存上界成立 + 信用门开闭频率可接受）；数据证伪才改**常量默认值**（改值需负载数据支撑），**不动可配性/flag 面**（P2 D-10 拒绝可配化纪律延伸）
- **D-13:** 标定结果落点 = **README「默认参数与 Phase 9 标定」节更新**（初值→标定值/验证结论 + 实测数据摘要），表头改名去「Phase 9 标定」挂账语

### 发布脚本
- **D-14:** 发布前操作整合为**发布脚本**（长 fuzz 10min+ → 负载矩阵 → 打 tag push 触发 release.yml），发布前跑一次——用户裁决原话：「把所有发布时需要做的操作都整合在这个脚本内，发布之前跑一次即可」；脚本即发布文档的可执行形态，README/文档引用

### 部署文档（ROADMAP SC3）
- **D-15:** 反代配方验证深度 = **Caddy 本机实证**（二进制部署 + 双机全链套路同 nginx G-07-2 先例）+ **Cloudflare 按官方文档写并标注「未实测」**（SaaS 无法本机复现，风险接受）；两配方均含空闲超时与 ping 间隔关系（CORE-06 默认 5s ping × 反代 idle timeout 匹配表）
- **D-16:** Docker = **Dockerfile 入库**（scratch + 静态二进制 + tini 作 PID 1 收割）+ **本机 docker 24.0.6 构建实测**（PID 1 收割/僵尸残留行为验证）；**不发布镜像**（scp 哲学一致，镜像用户自建）
- **D-17:** systemd unit = **`deploy/wesh.service` 入库**（Restart=/LimitNOFILE=/EnvironmentFile=600 全配）+ README 引用 + 复用 P8 实机 systemctl 通道最小实测（08-05 draining 观测同通道）

### ship 清零（07 deferred-items.md 全结）
- **D-18:** 三项 UI WARNING 全清——① 1001 关停面板 hint 按场景条件化（systemd Restart=always 自重启形态下「Start wesh again from your shell」为无效指引，main.ts:903 hintPrefix 条件化）；② `#status` 面板族加 `role="alert"`（index.html:63 单属性零视觉影响）；③ pre-onopen 到达的 1001 先按码分派（main.ts:881-884 `!opened` 分支前分派 ev.code===1001，毫秒级窗口误述修复）

### Claude's Discretion
- goreleaser 配置细节（-trimpath、ldflags `-s -w -X main.version={{.Version}}`、mod_timestamp、CGO_ENABLED=0 env、archive 命名模板精确形状）
- release.yml 与 ci.yml 的文件关系（独立文件；tag 触发条件写法；web leg 步骤复用形态）
- 发布脚本的落点与形态（scripts/release.sh 候选；含干跑/确认闸；打 tag 前工作树干净校验）
- fuzz 种子语料选型（合法 Hello/畸形帧/边界长度/TOML 合法与畸形样本）与 CI 短跑精确时长
- 负载矩阵格子数与每格时长（README 方法论 1/4/16/32 × 输出速率 × 慢链路注入的实例化；defunct 检测口径 = goroutine 数/fd 数/僵尸进程三面）
- 自定义页的 gzip 处理（启动读入后可顺手预压一次缓存——实现细节，与 embed .gz 旁路正交）
- --index 与 --base-path 组合下的伺服装配（mux 前缀内自然成立）与安全头中间件适用性（现状同源）
- Caddy 实证的部署形态（官方二进制直装，禁 apt 桌面包纪律不涉服务端软件）与双机全链断言面
- Dockerfile 细节（scratch vs distroless、CA 证书束 TLS 出站需要性评估——wesh 无出站调用倾 scratch 零证书、EXPOSE/ENTRYPOINT 形态、--socket 容器内语义说明）
- D-18 三项修复的精确文案与断言面（1001 hint 条件化文案；role="alert" 的 jsdom 断言；pre-onopen 分派顺序）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 9 — 成功准则 3 条（goreleaser 四平台全静态 + embed 单 HTML + scp 即跑 / 自定义首页生效 + 负载模糊测试通过回填 P2/P5 默认参数 / 部署文档覆盖 nginx+CF+Caddy+Docker+systemd）
- `.planning/REQUIREMENTS.md` — OPS-03、OPS-10 原文
- `.planning/PROJECT.md` — Constraints（单静态二进制分发必须保持；Linux+macOS 平台边界；CGO 无关的纯 Go 依赖栈）、Key Decisions（单静态二进制分发 Pending Phase 9 goreleaser 验证——本 phase 兑现点）

### 标定挂账与方法论
- `README.md` §默认参数与 Phase 9 标定（231-243 行）— 挂账参数表（初值+一阶依据）与既定标定方法论（负载矩阵形状、验收标准、数据源=Phase 8 已埋计数器）
- `.planning/STATE.md` §Blockers/Concerns — P2/P5/P6 标定挂账条目（outbox 容量/水位、EXIT 直写 2s、宽限默认值）

### 前序 phase 决策
- `.planning/phases/07-deployment/07-CONTEXT.md` — D-03/D-04/D-06（TOML 平铺键=flag 同名、覆盖面、未知键拒绝——D-07/D-08 的 --index 入配置与 index-max-size 例外依据）、deferred（--index Phase 9、负载标定回填——本 phase 兑现）
- `.planning/phases/07-deployment/deferred-items.md` — 三项 UI WARNING 原文与修复指引（D-18 兑现清单；含 main.ts:903/index.html:63/main.ts:881-884 精确落点）
- `.planning/phases/06-session-lifecycle/06-CONTEXT.md` — EXIT 直写 2s 标定挂账（D-12 验证对象）、deferred（exit-when-empty 宽限默认值标定）
- `.planning/phases/05-multi-client/05-CONTEXT.md` — deferred（outbox 容量/水位/strikes 标定挂账——注意 strikes/gateDwell 已于 2026-08-29 实证废弃，标定面以现状 kickOrCreditLocked 为准）
- `.planning/phases/08-observability/08-CONTEXT.md` — D-03/D-05（metrics series 与预埋挂点——负载测试数据源：踢出数/门开闭/输入丢弃/outbox 深度）

### 调研结论
- `.planning/research/PITFALLS.md` — Pitfall 5（-race 需 cgo——CI 常规测试 CGO 默认启用、发布构建 CGO_ENABLED=0 两通道分立的依据）；计数器/map 防单调增长（负载测试断言面纪律）

### 现状代码（扩展点）
- `cmd/wesh/main.go` — version = "dev" ldflags 注入点（:32，goreleaser -X 挂点）；parseArgs/config struct/validateStartup（--index flag 与 index-max-size 配置键宿主；fail-fast 校验矩阵先例）；whitelistEnv 之外的启动面红线（错误行不含值内容——D-08 依据）
- `web/embed.go` — Handler（空路径回落 index.html、.gz 旁路、Vary 头——D-05/D-06 自定义页替换点；启动读入形态参照）
- `internal/server/sharetoken.go:83` — /s/{token}/ 给页委托 embed handler（D-06 全通道统一替换的交汇点）
- `internal/server/server.go` — defaultHelloTimeout 5s（:278）/ defaultPongTimeout 10s（:287）/ EXIT 直写 2s（:1345）/ MaxBytesReader 4096（:585）——标定验证对象
- `internal/server/clients.go:33-66` — defaultOutboxBytes 512KiB / defaultMaxClients 32 / defaultInputRate 32KiB/s / defaultInputBurst 64KiB / defaultInputQueueBytes 256KiB / defaultResizeDebounce 50ms / defaultAttachGrace 500ms——标定验证对象（注释自带一阶依据）
- `internal/proto/proto.go:76` — ReadLimitPostAuth 16KiB（P2 挂账验证对象）
- `.github/workflows/ci.yml` — go 矩阵 + web 钉版 leg（release.yml D-03 编排的参照；CGO 注释两通道依据）
- `web/uat/phase05-flood-driver.mjs` — Node 洪水驱动先例（负载测试场景设计参照，非载体）
- `web/src/main.ts:881-903` + `web/index.html:63` — D-18 三项修复落点

### ttyd 源码（缺陷对照面，不参考实现）
- `~/open_src/ttyd/` — `-i` 自定义 index（D-05 语义同款对照）；无 goreleaser/发布链（ttyd 用自有 Makefile 发布，本 phase 从零设计）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `version = "dev" + ldflags 注入点`（main.go:32）— goreleaser `-X main.version={{.Version}}` 直接挂，零代码改动
- `embed.Handler`（web/embed.go）— 自定义 index 的替换点：启动读入 []byte 后包一层「index.html 路径返回自定义字节」的 Handler 装饰即兑现 D-05/D-06，gzip 旁路对自定义页自然不适用（明文伺服或启动预压——Discretion）
- `ci.yml web leg 钉版组合`（pnpm 11.21.0 + node 24 + --frozen-lockfile）— release.yml D-03 显式编排逐行复用
- `config struct + 两阶段合并 + DisallowUnknownFields`（main.go，P7 D-01..D-07）— index-max-size 纯配置键的宿主机制现成；「配置键存在即显式位」先例保持
- `Phase 8 metrics 预埋挂点`（clients.go droppedInputs/inputDrops atomic、registry.kicks/gateTransitions、/metrics 17 series）— 负载测试断言数据源（README 标定方法论既定）
- `phase05-flood-driver.mjs 洪水场景设计` — 高吞吐 fan-out/慢客户端矩阵的场景模板（Go 黑盒载体重实现同场景）
- `P8 实机 systemctl 通道`（08-05 draining 观测：systemctl restart 轮询 200→503）— D-17 systemd unit 最小实测同通道
- `双机全链套路`（G-07-2：Windows Playwright + Linux 服务端 + TCP 转发器 + 真 nginx）— D-15 Caddy 实证复用

### Established Patterns
- **文档即被测物**（G-07-2 先例）— README 配方必须经实证锚点：Caddy 双机全链、Dockerfile 本机构建、systemd 实机 systemctl；CF 唯一例外（SaaS 无本机条件）且必须标注「未实测」
- **CLI flag 不轻易新增 + 配置文件承载低频调参**（P2 D-15 + P7 D-03）— D-08 的 index-max-size 纯配置键例外是该纪律的显式调和形态，README 明示防「例外蔓延」
- **启动面红线：错误行不含值内容**（P3/P4 记录式上报）— D-08 超限报错只含路径；--index 不存在/不可读同纪律
- **exit 2 fail-fast 启动校验矩阵**（P3/P7）— --index 文件校验落 parse/validate 分层既有位
- **默认常量注释自带一阶依据 + Phase 9 挂账语**（clients.go:33-66 先例）— D-12 证伪改值时注释同步改写为实测依据
- **build tag 隔离重型测试**（Go 生态惯例，本仓首例）— 负载测试 `-tags=load` 手动跑，常规 CI 零负担
- **崩溃语料入库永久回归**（Go fuzz testdata/fuzz/ 惯例）— fuzz 发现固化进常规测试面

### Integration Points
- `.github/workflows/release.yml`（新建）— tag push 触发；pnpm build → setup-go → goreleaser 显式编排
- `.goreleaser.yml`（新建）— 四平台 builds（CGO_ENABLED=0）、archives（tar.gz + LICENSE/README）、checksum、release 说明
- `cmd/wesh/main.go` — --index flag 注册 + 启动读入 + 校验矩阵扩展 + index-max-size 配置键
- `web/embed.go` 或 server 装配层 — 自定义 index 字节注入给页通道（/ 与 /s/{token}/ 统一）
- `internal/proto/` + `cmd/wesh/config`（TOML 加载点）— fuzz 两目标
- `internal/server/load_test.go`（新建，build tag load）— 负载矩阵黑盒测试
- `testdata/fuzz/`（新入库）— 崩溃语料回归
- `Dockerfile`、`deploy/wesh.service`（新建入库）— D-16/D-17
- `README.md` — 发布节（goreleaser 产物/校验和验证/发布脚本引用）+ --index 节 + 部署节扩充（Caddy/CF/Docker/systemd 引用）+ 标定表更新（D-13）
- `web/src/main.ts:881-903` / `web/index.html:63` — D-18 三项修复 + dist 重建
- `scripts/release.sh`（新建候选）— D-14 发布脚本

</code_context>

<specifics>
## Specific Ideas

- **「发布脚本即发布文档的可执行形态」**：用户明确要求发布前操作（长 fuzz、负载矩阵、打 tag）整合进单一脚本，发布前跑一次——文档（README 发布节）描述流程，脚本承载流程，两者同源不漂移
- **「如果设上限就必须可配置」**：D-08 的用户裁决原话——上限默认值防误指大文件 OOM，可配置性保后续真实大文件场景不调源码；调和形态是纯 TOML 配置键（不加 CLI flag），这是本项目「flag 面紧缩、配置面宽」哲学的首次纯配置键例外
- **标定是「验证」不是「调优」**：挂账参数的一阶依据已写在常量注释里，负载测试的首要职责是证伪（零误踢/内存上界/门频率），只在数据证伪时才改默认值——防止「为了标定而标定」把已验证的行为面搅乱
- **CF 是「文档即被测物」的唯一例外**：SaaS 反代无本机复现条件，标注「未实测」是诚实义务而非缺陷——与 Caddy/nginx 的实证配方在文档中明确分级
- **07 三项 WARNING 在 ship 清零是「发布质量门」语义**：1001 hint 误导、ARIA 缺失、pre-onopen 误述都是用户可见的发布面瑕疵，随本 phase 清零使 v1.0.0 发布物无已知 WARNING 挂账

</specifics>

<deferred>
## Deferred Ideas

- **cosign 签名 / SBOM 供应链增强** — D-02 裁决仅 checksums；若发布后被企业用户要求合规再评估
- **Docker 镜像发布（ghcr.io）** — D-16 裁决 Dockerfile 入库不发镜像；真实需求（docker run 即用呼声）出现再评估
- **nightly/snapshot 预发布通道** — D-01 裁决 tag 触发正式 release；个人项目 v1 阶段无需求
- **负载测试进 CI 常规回归** — D-11 裁决 build tag 手动跑；若未来参数回归事故再评估 CI 化
- **Cloudflare 配方实证** — D-15 风险接受标注未实测；有 CF 账号环境后补实测

</deferred>

---

*Phase: 9-release-polish*
*Context gathered: 2026-08-29*
