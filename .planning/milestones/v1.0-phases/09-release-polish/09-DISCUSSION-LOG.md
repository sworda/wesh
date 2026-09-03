# Phase 9: 发布与打磨 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-29
**Phase:** 9-发布与打磨
**Areas discussed:** 发布流程与版本策略, 自定义首页语义（OPS-03）, 负载/模糊测试范围与载体, 部署文档形态与 ship 清零

---

## 发布流程与版本策略

### Q1 发布触发方式与版本号起点？

| Option | Description | Selected |
|--------|-------------|----------|
| tag push 自动触发 | 推 v1.0.0 等 tag 自动触发 release.yml——版本史与 git tag 同源，个人运维工具零额外流程负担 | ✓ |
| 手动 workflow_dispatch | GitHub Actions 页面手动触发，输入版本号——发布节奏完全手动控制，但多一步人工操作 | |
| 每次合并 snapshot | main 分支每次合并都产出 snapshot 预发布——个人项目 v1 阶段过度工程 | |

**User's choice:** tag push 自动触发

### Q2 发布产物附带哪些供应链文件？

| Option | Description | Selected |
|--------|-------------|----------|
| 仅 checksums.txt | goreleaser 默认自带 checksums.txt；个人运维工具用户自编译成本低，供应链威胁模型弱——零密钥管理负担 | ✓ |
| checksums + SBOM | 加 SBOM（goreleaser syft 集成一行配置）——依赖清单可审计，企业合规场景友好 | |
| 签名 + SBOM 全套 | cosign 密钥/keyless 签名 + SBOM——供应链强保证，但引入密钥管理/Fulcio 依赖，个人工具偏重 | |

**User's choice:** 仅 checksums.txt

### Q3 前端产物在发布链中如何固化？

| Option | Description | Selected |
|--------|-------------|----------|
| workflow 步骤显式编排 | release.yml 内显式步骤：pnpm install+build → setup-go → goreleaser——与 ci.yml web leg 同钉版（pnpm 11.21.0/node 24），构建顺序肉眼可审 | ✓ |
| goreleaser before hooks | goreleaser before.hooks 里调 pnpm——发布链单文件自含，但 hook 环境隐式（CI 镜像需预装 node/pnpm），调试链路长 | |
| 直接用入库 dist | dist 已入库（裸 clone 可编译既定策略），发布直接打当前 HEAD 的 dist——零 node 依赖，但发布产物可能滞后于 web/src 未构建提交 | |

**User's choice:** workflow 步骤显式编排

### Q4 发布产物的打包形态？

| Option | Description | Selected |
|--------|-------------|----------|
| tar.gz 带 LICENSE/README | wesh_v1.0.0_linux_amd64.tar.gz 内含 wesh + LICENSE + README.md——用户解压即见文档，scp 单文件也行 | ✓ |
| tar.gz 纯二进制 | tar.gz 内仅 wesh 裸二进制——最小体积，文档走 GitHub 仓库 | |
| 裸二进制直传 | 四平台裸二进制直接传 Release（不打包）——下载即 chmod +x，但丢失打包校验与附带文档 | |

**User's choice:** tar.gz 带 LICENSE/README

---

## 自定义首页语义（OPS-03）

### Q1 自定义首页的语义定位？替换后终端功能谁来承担？

| Option | Description | Selected |
|--------|-------------|----------|
| 整页替换（ttyd -i 同款） | --index 后用户 HTML 完全替代首页，终端功能由用户页面自行实现（ttyd -i 同款）——面向自托管前端的高级用户，wesh 只负责伺服，零模板注入面 | ✓ |
| 模板注入片段 | wesh 终端页为主，用户 HTML 片段注入说明区/banner——保留终端能力，但引入模板拼接与 CSP/注入复杂度 | |
| 根路径替换+分享通道保留 | 自定义页只在无 token 根路径展示，/s/{token}/ 始终给内嵌终端页——分享链接永可用，但首页语义双轨 | |

**User's choice:** 整页替换（ttyd -i 同款）

### Q2 整页替换后，/s/{token}/ 分享链接给哪个页面？

| Option | Description | Selected |
|--------|-------------|----------|
| 全通道统一替换 | 根路径与 /s/{token}/ 都伺服自定义页——单一给页源零双轨，README 明示「自定义页需自行实现终端逻辑，否则分享链接失去终端功能」 | ✓ |
| 分享通道例外保留 | /s/{token}/ 仍给内嵌终端页——分享链接永远有终端，但同实例两种首页是双轨语义，与整页替换裁决有张力 | |

**User's choice:** 全通道统一替换

### Q3 --index flag 的读取时机与校验纪律？

| Option | Description | Selected |
|--------|-------------|----------|
| --index 启动一次读入 | --index /path/to.html，启动时读入内存（不存在/不可读 exit 2）——与 embed 静态伺服同语义：运行期零磁盘依赖，改文件需重启生效 | ✓ |
| 每请求读盘热生效 | --index 运行期每请求读盘——改文件即生效免重启，但引入磁盘 IO/TOCTOU 面，与 embed 语义不一致 | |
| 你决定 | 交给 Claude/规划阶段按 embed 一致性原则定 | |

**User's choice:** --index 启动一次读入

### Q4 自定义首页文件大小上限？

| Option | Description | Selected |
|--------|-------------|----------|
| 16MiB 上限 fail-fast | 启动读入时设 16MiB 上限（超限 exit 2）——防误指 ISO/视频等大文件把内存吃爆，16MiB 对 HTML 是天文数字不误伤 | |
| 无上限 | 不设上限，用户自己负责——ttyd 同款；但误指大文件 OOM 是静默坑 | |
| 你决定 | （用户补充：若设上限须可配置） | ✓ |

**User's choice:** 你决定——「如果要设置上限的话，需要支持配置，如果后续确实有较大文件，通过改配置的方式可以进行调整」
**Notes:** Claude 按条件裁决：默认 16MiB 硬顶（io.LimitReader 启动读入，超限 exit 2 且错误行只含路径不含内容）+ TOML 配置键 index-max-size 可调；不开 --index-max-size CLI flag（P2 D-15「flag 不轻易新增」与「配置可调」的调和：配置文件承载低频调参，纯配置键无对应 flag 是 P7 D-03 纪律的明示例外，README 明示）。用户随后确认「可以」。

---

## 负载/模糊测试范围与载体

### Q1 模糊测试的目标面？

| Option | Description | Selected |
|--------|-------------|----------|
| proto 解码 + 配置解析 | 对 proto 帧解码（DecodeHello/帧拆分）与 TOML 配置解析跑 Go native fuzz——输入面最危险两处，go test -fuzz 零新依赖 | ✓ |
| 仅 proto 解码 | 仅 proto 帧解码——WS 输入面是远程攻击面，配置是本地信任输入可缓 | |
| 三面全跑 | proto 解码 + 配置解析 + WS 握手路径（Hello JSON/Origin 头）三面全跑 | |

**User's choice:** proto 解码 + 配置解析

### Q2 fuzz 的运行形态与 CI 集成？

| Option | Description | Selected |
|--------|-------------|----------|
| CI 短跑+发布前长跑 | CI 跑 1-2min 短 fuzz（proto/配置两目标），发布前手动长跑 10min+；崩溃语料入库永久回归 | ✓ |
| CI 长跑 | 接受 CI 每次 5-10min fuzz——反馈环变慢换发现能力，公开仓库分钟数免费 | |
| 仅本地手动 | fuzz 不进 CI，仅本地手动跑 | |

**User's choice:** 先反问「CI长跑有什么问题，个人项目runner分钟数长会有什么问题么」——Claude 说明：公开仓库分钟数免费无上限，实际代价是反馈环时长（每次 push 等 10min+）与 fuzz 发现能力随时长递减（短跑抓回归、长跑挖深坑），崩溃语料入库后 CI 常规测试永久重放。用户随后裁决：「按你推荐的来吧，并把发布时的这个操作记录在发布文档里，或者后续你整理一个发布脚本，把所有发布时需要做的操作都整合在这个脚本内，发布之前跑一次即可」——由此新增 D-14 发布脚本决策（长 fuzz → 负载矩阵 → 打 tag 触发 release.yml）。

### Q3 负载测试的载体形态？

| Option | Description | Selected |
|--------|-------------|----------|
| Go 黑盒负载测试 | internal/server 黑盒负载测试（build tag 隔离不进常规 CI，手动 -tags=load 跑）：coder/websocket 客户端性能远超 Node，goroutine/内存上界断言直接（runtime.NumGoroutine/ReadMemStats），与既有夹具同语言 | ✓ |
| Node UAT 驱动扩展 | 扩展 phase05-flood-driver.mjs 模式——与 UAT harness 一致、spawn 真实二进制，但 Node 单线程客户端性能有限，读 /metrics 断言 | |
| Go 主力 + Node 巡检 | Go 黑盒做矩阵主力 + Node 脚本做真实二进制 defunct 巡检（僵尸进程/fd 残留）——双载体各司其职，工作量略增 | |

**User's choice:** Go 黑盒负载测试

### Q4 标定回填的纪律：验证还是调优？

| Option | Description | Selected |
|--------|-------------|----------|
| 验证为主证伪才改 | 默认验证现值成立（零误踢+内存上界+门频率），数据证伪才改默认值——改值需负载数据支撑且只动常量默认值，不动可配性/flag 面 | ✓ |
| 全面调优 | 标定=调优——所有挂账参数都按实测数据重调（outbox/水位/rate/grace/2s 全可能变） | |
| 你决定 | | |

**User's choice:** 验证为主证伪才改

---

## 部署文档形态与 ship 清零

### Q1 Cloudflare/Caddy 配方的验证深度？（本机无 Cloudflare 验证条件）

| Option | Description | Selected |
|--------|-------------|----------|
| Caddy 实证 + CF 标注未实测 | Caddy 本机可装可实证（同 nginx 双机套路）；Cloudflare 是 SaaS 无法本机复现——按官方文档写并标注「未实测，基于官方文档」，风险接受 | ✓ |
| 两者都标注未实测 | 两者都仅按官方文档写并标注未实测——本 phase 文档工作量最小，但 Caddy 配方未经实证与「文档即被测物」先例有张力 | |
| 你决定 | | |

**User's choice:** Caddy 实证 + CF 标注未实测

### Q2 Docker 支持形态？（本机 docker 24.0.6 可构建实测）

| Option | Description | Selected |
|--------|-------------|----------|
| Dockerfile 入库实测，不发镜像 | Dockerfile 入库（scratch + 静态二进制 + tini 作 PID 1）+ 本机构建实测收割行为；不发布镜像——scp 哲学一致，镜像由用户自建 | ✓ |
| 入库 + 发布 ghcr.io | Dockerfile 入库 + goreleaser 顺带发 ghcr.io 镜像——用户 docker run 即用，但引入镜像仓库维护面 | |
| 纯文档配方 | README 内嵌 Dockerfile 配方不入库——文档级覆盖，无本机实测锚点 | |

**User's choice:** Dockerfile 入库实测，不发镜像

### Q3 systemd unit 模板的落点？

| Option | Description | Selected |
|--------|-------------|----------|
| deploy/ 入库 + 实测 | deploy/wesh.service 入库（Restart=/LimitNOFILE=/EnvironmentFile=600 全配）+ README 引用 + 复用 P8 实机 systemctl 通道最小实测——与 Dockerfile 入库对称，可被 curl 直取 | ✓ |
| README 内嵌升级 | README 内嵌完整 unit（现状内嵌示例升级）——单文件文档哲学，复制粘贴即用，但无独立文件可直取 | |
| 你决定 | | |

**User's choice:** deploy/ 入库 + 实测

### Q4 07 三项 UI WARNING 是否随本 phase 清零（既定「ship 后清零」路由）？

| Option | Description | Selected |
|--------|-------------|----------|
| 三项全清 | 1001 hint 条件化 + #status role="alert" + pre-onopen 1001 分派三项全清——发布前扫尾，07 deferred-items.md 全结 | ✓ |
| 清高价值两项 | hint 条件化 + role="alert" 两项高价值清；pre-onopen 1001 竞态（毫秒级窗口）继续挂账 | |
| 都不做 | 三项都不进本 phase，继续挂账 | |

**User's choice:** 三项全清

---

## Claude's Discretion

- goreleaser 配置细节（-trimpath、ldflags、mod_timestamp、CGO_ENABLED=0 env、archive 命名模板）
- release.yml 与 ci.yml 文件关系与 tag 触发条件写法
- 发布脚本落点与形态（scripts/release.sh 候选；干跑/确认闸；工作树干净校验）
- fuzz 种子语料选型与 CI 短跑精确时长
- 负载矩阵格子数与每格时长、defunct 检测口径（goroutine/fd/僵尸进程三面）
- 自定义页 gzip 处理（启动预压可选）
- --index × --base-path 组合装配与安全头适用性
- Caddy 实证部署形态与双机断言面
- Dockerfile 细节（scratch vs distroless、CA 证书束评估、EXPOSE/ENTRYPOINT）
- D-18 三项修复精确文案与断言面

## Deferred Ideas

- cosign 签名 / SBOM 供应链增强 — 发布后被企业用户要求合规再评估
- Docker 镜像发布（ghcr.io）— 真实需求出现再评估
- nightly/snapshot 预发布通道 — v1 阶段无需求
- 负载测试进 CI 常规回归 — 参数回归事故发生再评估
- Cloudflare 配方实证 — 有 CF 账号环境后补
