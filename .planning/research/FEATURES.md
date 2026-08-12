# Feature Research: wesh（Web 终端分享工具）

**Domain:** Web 终端分享（ttyd-class：浏览器经 WebSocket 获得远程 PTY）
**Researched:** 2026-08-13
**Confidence:** MEDIUM（品类格局为常识级且经官方站点/仓库交叉验证；个别产品细节为单源，已标注）

## 竞品格局速览

| 产品 | 形态/接入 | 会话保持 | 多客户端 | 权限模型 | 关键差异化 |
|------|-----------|----------|----------|----------|------------|
| ttyd 1.7.7 | 单二进制 C，浏览器/WS | ❌ 每连接一进程，断开即杀 | ❌（官方推荐配 tmux） | 默认只读，`-W` 全员可写 | ZMODEM/trzsz/Sixel（均默认关闭） |
| GoTTY | 单二进制 Go，浏览器/WS（~2020 起停更） | ✅ 一个进程被所有客户端共享 | ✅ 原生（共享同一 PTY） | `--permit-write` 全员可写 | 证明"共享进程"架构需求真实存在，但已无人维护 |
| Wetty | Node.js 服务，浏览器→SSH 到本机/远端 | ❌（SSH 会话随连接） | ❌ | 真实 SSH 登录认证 | 复用 SSH 认证体系；Node 运行时依赖重 |
| tmate | tmux fork + SSH 中继（tmate.io） | ✅（本质是 tmux） | ✅ | **ro / rw 两条连接串**，另有 Web 只读链接 | 零配置 NAT 穿透、双权限令牌、可自建中继 |
| sshx | Rust 单二进制客户端 + gRPC mesh 服务端 | ✅（服务端 Redis 快照） | ✅ 多人协作（实时多光标、无限画布） | 链接即权限 | E2E 加密（AES+Argon2）、预测回显；自托管困难 |
| upterm | Go 单二进制，SSH 接入（非浏览器优先） | ✅（tmux-backed） | ✅ | `--read-only` | CI 调试集成（GitHub Actions）、可自建中继 |
| termpair | Rust，浏览器 | 部分（广播模型） | ✅ 观看 | 默认只读 | 浏览器端 E2E 加密，服务端只见密文；小众 |
| asciinema 3.0（2025-09，Rust 重写） | CLI + Web 播放器 | ✅（`stream` 会话独立于观看者） | ✅ 多观看者 | 只读广播 | 录制 + `asciinema stream` 实时直播（内建 HTTP 服务/远程中继） |
| shellhub | SSH 网关 + Web UI（车队管理） | ✅ | 经网关 | 公钥/MFA/防火墙规则 | 会话录制回放、审计日志、设备管理（企业向） |
| code-server | 浏览器内 VS Code | ✅（终端跨刷新重连） | ❌（单用户） | 密码/none | 编辑器+终端 tabs/分屏；非分享工具 |
| AWS CloudShell / GCP Cloud Shell / EC2 Instance Connect / SSM Session Manager | 云控制台内浏览器终端 | ✅（持久 home 1GB/5GB；SSM 会话日志落 S3） | 部分 | IAM 预认证 | 文件上传下载、审计、免开端口；树立"浏览器终端开箱即用"的用户预期 |

**品类分水岭结论（MEDIUM 置信）：** 会话生命周期独立于连接、多路复用 attach，是品类主流能力——ttyd 是唯一"每连接一进程、断开即杀"的主流工具（其官方手册用 tmux 变通即为需求证据）。wesh v1 的两项核心改进（detach/reattach、原生多客户端 attach）是把 ttyd 拉回品类及格线，安全与资源控制才是相对同类的真正差异化。

## v1 范围验证（对 PROJECT.md 的 scoping 决策逐项核对）

| 已承诺的 v1 项 | 验证结论 | 依据 |
|----------------|----------|------|
| 会话保持（detach/reattach + 滚动回放） | ✅ 正确，且应定位为"补齐表赌注"而非差异化 | tmate/sshx/GoTTY/upterm/code-server/CloudShell 全部具备；ttyd 系孤例 |
| 原生多客户端 attach，写入权限可配置（全员可写 / 主写旁观） | ✅ 正确，双模式精确覆盖品类两大主流场景 | 协作排障（tmate rw、sshx 多人）+ 广播演示（asciinema stream、termpair ro、tmate ro 链接） |
| 安全加固 | ✅ 正确；对公网暴露的个人工具，安全本身是表赌注 | ttyd 已核实预认证 DoS/内存放大；云厂商产品全部以 IAM/审计为先 |
| 资源控制（消息上限/背压/限速/最大连接数） | ✅ 正确；多客户端引入后 per-client 背压从"增强"升级为"必需" | ttyd 单 PAUSE/RESUME 模型在 N 客户端共享一会话时会让慢客户端阻塞所有人 |
| ZMODEM / trzsz / Sixel 推迟到 v2 | ✅ 正确 | 品类内仅 ttyd 提供带内文件传输且默认关闭（非表赌注）；ZMODEM 依赖停更的 zmodem.js；Sixel 小众；trzsz 是 v2 正确候选 |

**范围验证总评：** v1 scoping 成立，无需扩张核心范围。但发现 4 个被低估的表赌注项（见下节"表赌注补漏"），均为低成本，建议并入 v1。

## Feature Landscape

### Table Stakes（缺了用户就走）

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| 任意命令 PTY 双向转发 | 品类定义本身 | MEDIUM | 协议重设计（版本协商/类型化错误帧/合规关闭码）在此一并完成 |
| 终端尺寸同步 + fit 自适应 | 所有竞品均有 | LOW | resize → TIOCSWINSZ；多客户端时需仲裁策略（建议最后写入者生效，或 owner 尺寸优先） |
| xterm.js 现代前端（WebGL 渲染、Unicode11/CJK/IME、超链接、剪贴板） | ttyd/code-server 已验证的标配组合 | LOW | 剪贴板须用 `navigator.clipboard` 替代 ttyd 已废弃的 `execCommand('copy')` |
| 会话保持 + 滚动缓冲回放 + 断线重连接回原会话 | 品类分水岭（ttyd 唯一缺失） | HIGH | 环形滚动缓冲 + reattach 协议 + 每客户端回放位置；v1 最难项 |
| 默认只读 + 显式可写开关 | ttyd/GoTTY/termpair 一致默认 | LOW | 安全默认 |
| 认证 + TLS | 公网暴露工具的底线；云厂商全部预认证 | MEDIUM | 时序安全比较、一次性短时令牌、失败节流；TLS 禁旧协议+cipher 控制+安全响应头 |
| WS keepalive + 前端自动重连 | ttyd/GoTTY/sshx 均有 | LOW | 重连价值依赖会话保持（否则重连=新会话，即 ttyd 现状） |
| 监听配置（端口/绑定地址/UNIX socket） | 全部竞品均有 | LOW | 含 socket 属主设置 |
| 生命周期模式：`--once` 单客户端、无人连接即退出 | ttyd/GoTTY 均有；CI/一次性分享常用 | LOW | **PROJECT.md 未列入，补漏 #1** |
| 启动即打印可分享 URL（含一次性 token；ro/rw 两条链接） | tmate/sshx/upterm 的"打印即用"是分享工具 UX 基准 | LOW-MEDIUM | **PROJECT.md 未显式列入，补漏 #2**；多客户端分享的临门一脚 |
| 结构化日志 + /healthz | 可运维性底线 | LOW | ttyd 缺失，低成本补齐 |
| 配置文件 | ttyd 缺失被诟病的点 | LOW | CLI 覆盖配置文件即可 |

### Differentiators（相对同类的竞争优势）

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| 原生多客户端 attach + ro/rw 权限（单二进制、浏览器、自托管） | 对 ttyd/GoTTY 是代差；tmate/sshx 要么非浏览器优先、要么难自托管 | HIGH | 依赖会话抽象；含输入仲裁与 per-client 背压 |
| 安全默认全集（token 不明文进日志、Origin 白名单、子进程 env 白名单、认证节流） | 同类普遍有已核实缺陷；主打"敢暴露到公网" | MEDIUM | 直接修复 ttyd 源码核实的 8 项安全问题 |
| 资源控制（消息上限/分片重组上限/per-client 限速/最大连接数） | 品类普遍缺失；防预认证内存放大 | MEDIUM | 多客户端下与 attach 联动设计 |
| 会话录制导出 asciicast v2/v3 | asciinema 生态直接可播放/嵌入；shellhub 证明录制是强需求 | LOW-MEDIUM | 会话抽象已存在时只是 tee PTY 流；**建议 v1.x 跟进** |
| 反代 auth-header 透传（`-H` 等价） | 个人运维常挂 authelia/nginx SSO 后面 | LOW | **PROJECT.md 未列入，补漏 #3**；ttyd 已有此能力，重写不该丢 |
| 单静态二进制 + 无运行时依赖 | 继承 ttyd 核心优势；vs Wetty 的 Node 依赖、sshx 的 mesh 运维 | — | 分发形态约束，非功能 |

### Anti-Features（明确不做）

| Anti-Feature | Why Requested | Why Problematic | Alternative |
|--------------|---------------|-----------------|-------------|
| E2E 加密（termpair/sshx 式） | "sshx 都有，为什么 wesh 没有" | 与服务端会话保持/滚动回放**本质冲突**（服务端须持明文滚动缓冲）；多客户端密钥分发复杂；自托管场景 TLS+认证已足够 | TLS + 认证 + token；FAQ 中明确说明此权衡 |
| 多会话管理 UI（tabs/会话列表/dashboard） | code-server/CloudShell 都有 tabs | 把 wesh 拖向 shellhub/code-server 平台化；会话生命周期管理复杂度爆炸 | 需要多个终端就在 wesh 里跑 tmux |
| SaaS relay / 托管服务 / 账户体系 | sshx/tmate.io 的托管便利 | 运维成本与隐私责任（sshx mesh 即前车之鉴）；PROJECT.md 已排除 | 单二进制自托管；NAT 场景文档推荐反代/隧道 |
| 依赖外部 tmux 实现会话保持 | ttyd 生态惯用"ttyd + tmux new -A" | 破坏单二进制零依赖承诺；外部进程状态不可控 | 原生实现 session 抽象 |
| 多租户 / RBAC 用户库 | shellhub 有 | 个人运维工具边界；ro/rw 双令牌已覆盖协作与演示 | ro/rw 两级权限即止 |
| mosh 式预测回显 / 多端独立光标 | sshx 协作体验 | 复杂度极高，个人运维（1-3 人）无收益 | 不做；协作体验让位于简单可靠 |
| 浏览器内编辑器 / GUI 化 | code-server 方向 | 完全是另一个产品 | 不做 |
| URL `?arg=` 任意拼接命令参数 | ttyd 有此能力（`-a`） | 已核实的注入面（无校验无上限拼接） | **建议 v1 直接砍掉**；确需参数化用启动命令模板；若保留必须白名单+长度上限 |
| 录制自动上传云端/第三方平台 | asciinema.org 模式 | 隐私风险；个人工具无此诉求 | 本地 asciicast 文件导出即止 |

## Feature Dependencies

```
[会话抽象/保持]                                  ← v1 地基，最高优先
    ├──required by──> [多客户端 attach]
    │                     ├──required by──> [ro/rw 写入权限配置]
    │                     └──required by──> [per-client 背压/限速]
    ├──required by──> [滚动缓冲回放]
    │                     └──required by──> [断线重连接回原会话]
    ├──enhanced by──> [会话保活/超时回收策略]
    └──enables──────> [会话录制 asciicast 导出]（v1.x）

[认证子系统（token/TLS/节流）]
    └──required by──> [启动打印分享 URL（ro/rw 链接）]

[反代部署（base-path/auth-header）] ──tension──> [Origin 白名单校验]
[?arg= URL 传参]                  ──conflicts──> [安全加固]（建议砍掉）
```

### Dependency Notes

- **多客户端 attach 依赖会话抽象**：attach 的前提是进程生命周期独立于任何单条连接；没有 session 抽象就没有 attach。
- **ro/rw 权限依赖 attach**：权限是对"同一 session 的多个连接"的属性，单连接模型下无意义。
- **per-client 背压在多客户端下从增强变必需**：ttyd 单 PAUSE/RESUME 模型下，一个慢客户端会阻塞共享会话的所有客户端。
- **重连的价值依赖会话保持**：ttyd 已有前端重连，但重连即新会话，用户感知为"重连没用"；接回原会话才是完整闭环。
- **录制依赖会话抽象**：session 是天然的 PTY 流 tee 点，落成 asciicast v2/v3 文件即可被 asciinema 播放器生态消费。
- **base-path/auth-header 与 Origin 校验存在张力**：反代后 Origin 与 Host 可能不一致，Origin 白名单必须支持配置可信源，否则与反代部署冲突。
- **`?arg=` 与安全加固直接冲突**：无校验拼接是 ttyd 已核实的注入面；v1 建议砍掉，避免背着包袱做安全。

## MVP Definition

### Launch With（v1，与 PROJECT.md Active 一致 + 补漏）

- [ ] 核心终端（PTY 双向转发、resize/title 同步、xterm.js 现代前端）— 品类定义
- [ ] 会话保持 + 滚动回放 + 自动重连接回原会话 — 品类及格线，v1 最大工程项
- [ ] 多客户端 attach + ro/rw 权限 — 核心差异化
- [ ] 认证/TLS/一次性 token/节流/Origin 白名单/env 白名单 — 敢暴露公网的底线
- [ ] 资源控制（消息/重组缓冲上限、per-client 背压、最大连接数）— 修复已核实崩溃向量
- [ ] 监听/部署配置（端口/绑定/UNIX socket/base-path/降权/cwd/TERM/关闭信号/`-t` 等价机制）
- [ ] `--once` / 无人退出（补漏 #1）+ 启动打印 ro/rw 分享链接（补漏 #2）
- [ ] /healthz、结构化日志、配置文件

### Add After Validation（v1.x）

- [ ] 会话录制导出 asciicast v2/v3 — 会话抽象落地后成本低，shellhub/asciinema 证明需求强
- [ ] auth-header 反代认证透传（补漏 #3）— 挂 SSO 的用户会第一时间提
- [ ] metrics 端点 — 运维增强
- [ ] 移动端浏览器基础可用（可读+滚动）— 个人运维"用手机看一眼"场景真实存在；完整 IME 不必

### Future Consideration（v2+）

- [ ] trzsz 文件传输 — 现代带内传输方案，等核心稳定后做
- [ ] Sixel 图片 — 小众增值
- [ ] ZMODEM — 大概率永不做（依赖停更组件，trzsz 已覆盖）；届时仅文档说明决策
- [ ] Windows (ConPTY) — PROJECT.md 已排除，除非用户结构变化

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| 核心 PTY 终端 + 现代前端 | HIGH | MEDIUM | P1 |
| 会话保持 + 回放 + 重连闭环 | HIGH | HIGH | P1 |
| 多客户端 attach + ro/rw | HIGH | HIGH | P1 |
| 认证/TLS/安全加固 | HIGH | MEDIUM | P1 |
| 资源控制/背压 | HIGH | MEDIUM | P1 |
| 监听/部署配置全集 | MEDIUM | LOW | P1 |
| --once/无人退出 + 分享链接打印 | MEDIUM | LOW | P1 |
| healthz/结构化日志/配置文件 | MEDIUM | LOW | P1 |
| 会话录制 asciicast | MEDIUM | LOW-MEDIUM | P2 |
| auth-header 透传 | MEDIUM | LOW | P2 |
| metrics | LOW-MEDIUM | LOW | P2 |
| 移动端基础体验 | LOW-MEDIUM | MEDIUM | P2 |
| trzsz | MEDIUM | MEDIUM | P3 |
| Sixel | LOW | LOW | P3 |

## Competitor Feature Analysis

| Feature | ttyd | tmate / sshx | wesh 策略 |
|---------|------|--------------|-----------|
| 会话保持 | ❌（配 tmux 变通） | ✅（tmux 本质 / 服务端快照） | 原生 session 抽象，滚动回放 |
| 多客户端共享 | ❌ | ✅ | ✅ 原生 attach，ro/rw 可配置 |
| 分享 UX | 手拼 URL + 明文 /token | 打印 ro/rw 连接串/链接 | 启动打印带一次性 token 的 ro/rw 链接 |
| 安全 | 8 项已核实缺陷 | tmate：SSH 双令牌；sshx：E2E | TLS+token+节流+白名单，FAQ 明示为何不做 E2E |
| 文件传输 | ZMODEM/trzsz（默认关） | ❌（scp 走带外） | v2 trzsz；v1 文档指带外 scp |
| 录制回放 | ❌ | shellhub/asciinema  headline 能力 | v1.x asciicast 导出 |
| 自托管成本 | 单二进制 | tmate 可自建中继；sshx 难 | 单二进制（核心卖点） |

## Sources

- ttyd 1.7.7 全量源码核实（2026-08-13，Explore agent，含行号证据）→ `.codebuddy/ttyd-analysis/01-功能清单.md` — **HIGH**（本地一手核实）
- sshx 官网与 GitHub 仓库（sshx.io / github.com/ekzhang/sshx）：E2E 加密（AES+Argon2）、多人画布、Redis 快照恢复、自托管困难 — MEDIUM（官方源交叉验证）
- asciinema CHANGELOG / GitHub（3.0.0，2025-09-15）：Rust 重写、`stream` 实时直播（本地内建 HTTP/远程中继）— MEDIUM（官方源）
- tmate 官网及使用文档：SSH 中继、ro/rw 双连接串、Web 链接、可自建 — MEDIUM（官方源+多教程交叉）
- shellhub 官方文档（docs.shellhub.io）：会话录制回放、审计、MFA、防火墙规则 — MEDIUM（官方源）
- upterm 官网/GitHub：SSH 接入、`--read-only`、CI 调试集成 — MEDIUM（官方源）
- termpair GitHub：浏览器端 E2E 加密、默认只读 — MEDIUM（官方源）
- GoTTY GitHub（yudai/gotty）：单进程多客户端共享架构、停更 — MEDIUM（官方源）
- AWS CloudShell / GCP Cloud Shell / SSM Session Manager 官方文档：预认证、持久 home、文件上传下载、会话日志 — MEDIUM（官方源）
- 各产品横向对比文章（pistack.xyz 2026-04 等）— LOW（仅作线索，结论均回溯官方源验证）

---
*Feature research for: wesh（personal-ops Web 终端分享，ttyd 现代化重写）*
*Researched: 2026-08-13*
