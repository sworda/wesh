# Project Research Summary

**Project:** wesh（Web 终端分享工具，ttyd 1.7.7 现代化重写）
**Domain:** 单静态二进制 Web 终端服务器（PTY ↔ WebSocket ↔ xterm.js），个人运维场景
**Researched:** 2026-08-13
**Confidence:** MEDIUM-HIGH（架构与 ttyd 缺陷为一手源码核实 HIGH；选型与竞品格局为多源交叉 MEDIUM）

## Executive Summary

wesh 是 ttyd 品类的 Web 终端分享工具：浏览器经 WebSocket 获得远程 PTY，分发形态为单静态二进制。品类能力光谱已被成熟系统（tmux/screen/abduco/coder/sshx 等）充分验证——**会话存活于断连、多客户端 attach 是品类主流能力，ttyd 是唯一"每连接一进程、断开即杀"的主流孤例**。wesh 的甜点位已经存在但被各占一半：**abduco 的简洁进程模型 + coder 的 ring 重放 + screen 的写权限语义**，并修复 coder 已验证的持锁 fan-out 缺陷。不需要 tmux/mosh 的服务端全量 VT 模拟（v1 收益不抵成本）。

推荐技术路线：**Go 1.26 + stdlib net/http + creack/pty + coder/websocket + embed 嵌入前端 + xterm.js 6（Vite 8 单文件构建）**。Go 在"单静态二进制、Linux+macOS、个人工具快速迭代"约束组合上全面贴合：CGO_ENABLED=0 全静态、单机交叉编译四平台、秒级编译、goroutine 模型与"PTY↔WS 数据泵 + 会话 hub 广播"结构一一对应。架构采用**单进程异步 IO + actor 模型**：Session Actor 是每会话唯一权威（PTY fd、ring、客户端集合），Client Conn 有界 outbox + 专属 writer，pidfd/kqueue 零线程收割——从结构上消除 ttyd 的 pss 跨域 UAF、每进程 waitpid 线程、连接即进程三大耦合。

关键风险集中在三处，均有明确对策：(1) **预认证攻击面**——ttyd 两个最严重漏洞（空帧空指针、分片重组内存放大）都发生在认证前的手写重组代码里，对策是 coder/websocket 内建 SetReadLimit + 三层上限（单帧/累积字节/分片帧数）+ 握手期认证前零资源分配；(2) **慢客户端背压**——一个 stalled 客户端可楔死整个 PTY 扇出，对策是每客户端有界 outbox 满则以 1013 踢出 + ring 重放重同步（ring 一鱼两吃）；(3) **字节流回放 ≠ 屏幕状态**——全屏 TUI 重连花屏是品类实测陷阱，v1 采用"ring 重放 + WINCH 触发应用重绘"的近似方案并诚实写入文档，精确 VT 重渲染留作 v2 选项。

## Key Findings

### Recommended Stack

后端选 **Go**（对 Rust 的裁决基于 8 个维度对比，Go 在 PTY 生态、并发模型匹配度、静态发布、编译速度上全胜；Rust 栈 tokio+axum+rustls 可行但 portable-pty 需 spawn_blocking 胶水、macOS 必须本机构建）。前端 xterm.js 6 + Vite 8 单文件产物，go:embed 嵌一个 HTML。

**Core technologies:**
- Go 1.26.x: 后端语言 — CGO_ENABLED=0 全静态 + 单机交叉编译 4 平台，goroutine 匹配 PTY↔WS 泵
- creack/pty v1.1.24: forkpty/TIOCSWINSZ — Go 生态 PTY 事实标准（1,263 个导入模块），纯 Go 无 cgo
- coder/websocket v1.8.15: WebSocket 服务端 — SetReadLimit 直接根治 ttyd 预认证分片重组漏洞；RFC 合规 Close；OriginPatterns 默认拒跨域
- stdlib net/http + crypto/tls: HTTP/TLS — 端点少，框架是负资产；HTTP/2 内建；TLS13 一行收敛
- embed + http.FS: 前端嵌入 — 零依赖复刻 ttyd 嵌入分发形态
- @xterm/xterm 6.0.0 + addon-webgl 0.19.0 + addon-fit 0.11.0: 前端 — v6 已删 canvas 渲染器，@xterm scope 官方包
- Vite 8.2.1 + vite-plugin-singlefile 2.3.3: 前端构建 — 产出单 HTML，embed 只需嵌一个文件
- 备选/支撑：autocert 或 certmagic（进程内 ACME，仅公网域名场景）、pflag、go-toml/v2、x/time rate、slog、crypto/subtle、goreleaser、golangci-lint、go test -race（CI 强制）

**明确不用：** 裸 libwebsockets 手写重组（ttyd 漏洞源头）、gorilla/websocket（近乎停滞 + 并发写 panic）、Node 路线（违反单二进制）、viper、zmodem.js/decko/execCommand('copy')（停更/废弃）。

### Expected Features

v1 scoping 经竞品验证成立，无需扩张核心范围；但发现 3 个低成本补漏项（`--once` 生命周期模式、启动打印 ro/rw 分享链接、auth-header 反代透传），前两个建议并入 v1，第三个列 v1.x。

**Must have (table stakes):**
- 任意命令 PTY 双向转发 + resize/title 同步 — 品类定义本身
- xterm.js 现代前端（WebGL/Unicode11/CJK/IME/剪贴板，用 navigator.clipboard） — ttyd 已验证标配
- **会话保持 + 滚动回放 + 断线重连接回原会话** — 品类分水岭，ttyd 唯一缺失，v1 最大工程项
- **原生多客户端 attach + ro/rw 权限** — 拉回品类及格线
- 认证 + TLS（时序安全比较、一次性短时令牌、失败节流、Origin 白名单）— 公网暴露底线
- WS keepalive + 前端自动重连、监听配置（端口/绑定/UNIX socket）
- 补漏：`--once` 无人退出、启动打印带一次性 token 的 ro/rw 链接、/healthz、结构化日志、配置文件

**Should have (competitive):**
- 安全默认全集（token 不进日志、env 白名单）— 主打"敢暴露到公网"，修复 ttyd 已核实 8 项缺陷
- 资源控制（消息/重组上限、per-client 背压、最大连接数）— 品类普遍缺失
- 单静态二进制无运行时依赖 — 继承 ttyd 核心优势

**Defer (v1.x/v2+):**
- 会话录制导出 asciicast v2/v3（v1.x，session 抽象落地后是低成本 tee）、auth-header 透传（v1.x）、metrics、移动端基础体验
- trzsz（v2）、Sixel（v2）、ZMODEM（大概率永不做）、Windows ConPTY（已排除）

**Anti-features（明确不做）：** E2E 加密（与服务端会话保持本质冲突，FAQ 说明权衡）、多会话管理 UI、SaaS relay、依赖外部 tmux、多租户 RBAC、`?arg=` URL 传参（已核实注入面，v1 直接砍掉）、录制上云。

### Architecture Approach

单进程异步 IO + actor 模型，控制面与数据面分离。所有会话共享状态只被其 Session Actor 触碰，跨组件一律 typed message 传递；PTY fd 不出 Actor；WS 写端只属于 Conn writer。三条边界规则遵守后，并发 bug 主战场（共享可变状态）在结构上不存在。

**Major components:**
1. **Gateway** — TLS/HTTP 路由/WS upgrade/帧编解码/预认证资源上限（长度上限、5s 认证超时、per-IP 连接上限）
2. **Auth Service** — 时序安全凭据校验、一次性 ticket（单次使用/60s TTL/绑定会话与模式）、失败指数退避
3. **Session Registry** — id→Actor 句柄表、create-or-attach 决策、空闲回收、全局上限
4. **Session Actor**（核心）— 每会话唯一权威：PTY master fd、pidfd、ring buffer、客户端集合、写模式、退出墓碑；单 goroutine + 邮箱，内部零锁
5. **Client Conn** — 每连接有界 outbox + 唯一 WS writer + reader
6. **Ring Buffer** — 每会话有界原始字节环（默认 256KiB–1MiB 可配），attach 快照，兼作慢客户端重同步机制
7. **PTY Engine** — forkpty/setsid、env 白名单、降权/rlimits、TIOCSWINSZ、pidfd(Linux)/kqueue(macOS) 零线程收割
8. **Observability** — slog 结构化日志、/healthz、metrics（每客户端 outbox 深度/踢出数）

**WS 协议：** 版本化子协议 `wesh.v1`；数据面二进制 1 字节类型前缀，控制面 JSON；认证并入握手（已认证 HTTP POST /api/attach 换一次性 ticket → WS Hello 首帧核销，通过前零会话资源）；合规关闭码（1000/1008/1011/1013，1006 永不线上发送）。

### Critical Pitfalls

1. **手写 WS 分片重组 + 认证前无资源上限（预认证 DoS/崩溃）** — 绝不手写重组；三层上限齐下（单帧+累积字节+帧数，Bandit CVE 教训：只限字节没用）；认证并入握手，通过前零缓冲分配（Phase 1）
2. **慢客户端背压楔死整个 PTY 扇出** — 每客户端有界 outbox + try_send + 满则 1013 踢出 + ring 重同步；PTY 读循环永不因客户端阻塞（Phase 2/4）
3. **原始字节流回放 ≠ 屏幕状态（全屏 TUI 花屏）** — ring 重放 + attach 先设尺寸触发 SIGWINCH 重绘；文档明示近似性；v1 不上服务端 VT 模拟（Phase 3）
4. **滚动缓冲无上限（双侧内存膨胀）** — 服务端按字节环形上限；前端分块回放 + write callback 流控（xterm.js 50MB watermark 会 throw）（Phase 3）
5. **认证子系统连环错（ttyd 五项全中）** — 常数时间比较（先哈希等长）、一次性 token 独立 secret、失败节流、凭据任何形态永不进日志（Phase 2）
6. **子进程 env 全继承泄密 + `?arg=` 注入** — env 白名单最小集；`?arg=` v1 砍掉；exec 数组绝不经 shell（Phase 1/2）
7. **每进程 waitpid 线程 / 僵尸泄漏** — pidfd/kqueue 并入事件循环统一收割；容器内 PID 1 义务（tini 或自身收割）（Phase 1）

## Implications for Roadmap

综合 ARCHITECTURE.md 的构建顺序（依赖链：`PTY Engine ≺ Session Actor ≺ {Ring 重放, 多客户端 fan-out}`；`协议/认证 ≺ 多客户端权限`；`Conn outbox 结构 ≺ 背压策略`）与 PITFALLS.md 的阶段映射，建议 6 个阶段：

### Phase 1: 行走骨架（核心管道）
**Rationale:** 无依赖，最先验证语言/运行时与端到端管道；协议层一次性设计到位（事后补洞要动协议）
**Delivers:** Gateway（裸 WS + 静态页）+ PTY Engine（forkpty/env 白名单/pidfd 收割/exec 数组）+ 单客户端 Session + 最小协议（OUTPUT/INPUT/RESIZE）+ xterm.js 前端接通
**Addresses:** 核心 PTY 终端 + 现代前端、resize/title 同步
**Avoids:** Pitfall 1（三层上限/认证前零分配，协议层预留）、7（env 白名单/spawn 失败不关自身 fd）、8（pidfd 收割）、9（合规关闭码、permessage-deflate 默认关）

### Phase 2: 协议与安全基线
**Rationale:** 必须先于共享功能——多客户端权限需要身份概念；安全是本项目核心卖点，不能事后补
**Delivers:** proto/ 类型化帧 + 版本协商 + 错误帧 + 关闭码映射；Auth（ticket + 时序安全比较 + 失败节流）；Origin 白名单；TLS 配置（禁旧协议 + 安全响应头）
**Addresses:** 认证/TLS/安全加固表赌注
**Uses:** coder/websocket（SetReadLimit/OriginPatterns）、crypto/subtle、x/time rate、crypto/tls
**Implements:** Auth Service、Gateway limits、proto 单一事实源
**Avoids:** Pitfall 5（序列过滤策略）、6（认证连环错全套）

### Phase 3: 会话解耦（品类跨越 #1）
**Rationale:** 依赖 P1/P2 管道与身份；**Actor 化时就把 Conn 建成 outbox+writer 结构**（单客户端也如此），Phase 4 零返工
**Delivers:** Registry + Session Actor 化 + detach/reattach + Ring 重放（REPLAY_BEGIN/END + WINCH 重绘）+ 空闲回收 + 退出墓碑 + 前端重连状态机
**Addresses:** 会话保持 + 滚动回放 + 重连接回原会话（v1 最大工程项）；`--once`/无人退出
**Implements:** Session Actor、Ring Buffer、Client Conn outbox 结构、Registry
**Avoids:** Pitfall 3（TUI 花屏，明示降级语义）、4（字节级环上限 + 前端分块流控）

### Phase 4: 多客户端（品类跨越 #2）
**Rationale:** 依赖 P3 的 Actor/outbox 与 P2 的身份；coder/screen/abduco 语义在此落地
**Delivers:** fan-out、慢客户端 1013 踢出 + 重同步、写模式 owner/all/ro、resize 仲裁（last-wins；≥2 客户端最小公共矩形 + 防抖）、SIZE_NOTICE、按模式分别签发 ro/rw 票据、启动打印分享链接
**Addresses:** 原生多客户端 attach + ro/rw 权限（核心差异化）；分享链接打印（补漏 #2）
**Avoids:** Pitfall 2（背压）、10（多客户端 resize 最小矩形）、3 的旁观模式 OSC52 强制关

### Phase 5: 资源控制与可运维
**Rationale:** 依赖 P3/P4 结构稳定后铺面；ttyd 缺失的可观测性不拖到 v2
**Delivers:** 全局连接/会话上限、每客户端限速、/healthz、metrics（outbox 深度/踢出率/字节数）、结构化日志、配置文件（TOML）、降权/rlimits、监听配置全集（UNIX socket/base-path）
**Addresses:** 资源控制差异化；healthz/日志/配置文件表赌注
**Avoids:** 结构化日志缺失陷阱；systemd/Docker 部署 gotchas 部分前置

### Phase 6: 打磨与发布
**Rationale:** 收尾；负载测试数据回填 P4/P5 默认参数
**Delivers:** 自定义首页、客户端偏好下发、goreleaser 单二进制四平台产物、模糊/负载测试（重点：高吞吐 fan-out、慢客户端矩阵、百万小帧/空帧）、部署文档（nginx/Cloudflare/Docker tini/systemd unit）
**Addresses:** 发布形态；集成 gotchas 全表
**Avoids:** 反代超时、容器 PID 1 僵尸、base-path 尾斜杠

### Phase Ordering Rationale

- **协议与安全前置（P2 在 P3 前）**：多客户端 ro/rw 权限是对"同一 session 多个连接"的属性，身份概念必须先存在；ttyd 教训证明认证事后补要动协议与前端。
- **Actor 化一步到位（P3 建 outbox 结构）**：单客户端阶段就按多客户端结构建模，避免 P4 返工——这是 ARCHITECTURE 给出的关键构建技巧。
- **会话保持先于多客户端**：attach 的前提是进程生命周期独立于连接；没有 session 抽象就没有多客户端。
- **可运维不拖 v2**：ttyd 无 healthz/metrics 是被诟病的点，P5 在发布前补齐。
- 每阶段出口对应 PITFALLS 的 "Looks Done But Isn't" 清单验收项。

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** macOS kqueue EVFILT_PROC 退出监视需早期原型验证（MEDIUM-HIGH 置信，有平台差异风险）——建议 `/gsd-plan-phase --research-phase 1`
- **Phase 3:** ring 默认容量与"尺寸变更后重放"体验需实测调参（coder 64KiB 仅作下界参考，建议 256KiB–1MiB 起步）——建议 `--research-phase 3`
- **Phase 4:** outbox 容量/水位/strikes 默认参数需负载测试标定——可在执行中以测试任务消化，不必专门研究

Phases with standard patterns (skip research-phase):
- **Phase 2:** 认证/TLS 模式均有官方文档与成熟库支撑（HIGH 置信源）
- **Phase 5/6:** 资源控制、可观测性、goreleaser 发布均为标准模式

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM | 所有版本号经官方 registry API 当日核实交叉验证；Go-vs-Rust 决策基于多条独立 MEDIUM 证据一致指向，非单一权威源 |
| Features | MEDIUM | 品类格局为常识级 + 官方源交叉验证；ttyd 细节为本地源码审计 HIGH；个别产品细节单源已标注 |
| Architecture | HIGH | 核心结论均经一手来源验证：ttyd 本地源码（带行号）、coder v2.36.0 源码精读、man7 pidfd、GNU screen 手册、tokio/xterm.js 官方文档 |
| Pitfalls | HIGH/MEDIUM | ttyd 缺陷为源码审计一手核实（HIGH）；生态通用陷阱多源交叉（MEDIUM） |

**Overall confidence:** MEDIUM-HIGH — 可直接支撑 roadmap 创建；残余不确定性集中在平台差异与参数标定，均已在 Research Flags 标注消化路径。

### Gaps to Address

- **macOS kqueue 退出监视**：语义为 MEDIUM-HIGH 置信，Phase 1 需原型验证（fork + kqueue NOTE_EXIT 取退出码）；失败兜底为 SIGCHLD + WNOHANG 循环 reap。
- **ring/outbox 默认参数**：coder 64KiB 偏小仅作下界；Phase 3 实测调 ring（256KiB–1MiB），Phase 4 负载测试标定 outbox 水位。
- **ACME 适用边界**：进程内 autocert 仅在"有公网域名 + 80/443 可达"可用；NAT/内网（个人运维最常见）文档必须诚实推荐静态证书/Tailscale，不静默失败。
- **多客户端 resize 细节**：relay 项目 ND-39 的"≥2 客户端最小公共矩形"结论与 ARCHITECTURE 的"owner 模式跟随 owner 尺寸"需在 Phase 4 规划中统一裁决（建议：owner 模式跟 owner，all 模式最小公共矩形）。
- **重放近似性的用户沟通**：TUI 花屏降级语义需写进 Phase 3 的文档与前端提示（"已恢复 N KiB 历史"），避免口碑事故。

## Sources

### Primary (HIGH confidence)
- ttyd 1.7.7 全量本地源码审计（protocol.c / pty.c / server.c / utils.c，带行号）— 重写基线与 8 项缺陷实证
- coder/coder v2.36.0 `agent/reconnectingpty/` 源码精读 — ring 重放、fan-out 反例（持锁遍历写 TODO 自述）
- man7 pidfd_open(2)（man-pages 6.18）— pidfd 收割语义与前置条件
- GNU screen 手册 multiuser 章节 — ACL rwx、writelock 单写者锁
- abduco 作者官网 — 极简 detach 模型、resize 仲裁、退出墓碑语义
- RFC 6455 §7.4 — 1005/1006/1015 保留关闭码
- tokio 官方文档（Context7）— broadcast Lagged 语义、mpsc 背压
- xterm.js 官方 typings/文档 — WriteBuffer 50MB watermark、write callback 流控、addon-serialize
- websocket.org 认证指南 — 浏览器 WS 无法带 Authorization 头、ticket 模式
- registry API 一手核实（proxy.golang.org / registry.npmjs.org / go.dev/dl / crates.io / api.github.com）— 全部版本号与发布日期

### Secondary (MEDIUM confidence)
- sshx/tmate/upterm/termpair/shellhub/asciinema 官方站点与仓库 — 竞品能力矩阵
- GHSA-vg8x-66vg-5pxh（Bandit WS 分片 O(n²)，CVSS 8.7）、CVE-2026-42786、CVE-2026-12151（undici）— WS 重组系统性事故区
- GHSA-mc23-976p-j42x（xterm.js RCE）、CVE-2025-48725（Warp OSC52）— 转义序列注入面
- relay 项目 ND-23/ND-39 决策记录 — 多客户端 resize 最小公共矩形实测结论
- pkg.go.dev importedby — creack/pty 1,263 个导入模块、coder/websocket 生产佐证
- websocket.org Go 指南（2026-03 更新）— "新项目用 coder/websocket" 推荐
- Context7（tungstenite/gorilla 默认值、instant-acme、axum WebSocketConfig）— Rust 备选栈参数

### Tertiary (LOW confidence，仅作格局判断)
- lib.rs/ossatlas sshx-server 分析、多源横向对比文章、中文技术博客（cargo-zigbuild 交叉编译主流方案）— 结论均回溯官方源验证后采用

---
*Research completed: 2026-08-13*
*Ready for roadmap: yes*
