# Requirements: wesh

**Defined:** 2026-08-13
**Core Value:** 浏览器里获得一个可靠、安全、可多人共享的远程终端

## v1 Requirements

### 核心终端（CORE）

- [x] **CORE-01**: 用户可通过命令行指定任意命令及参数，浏览器获得完整交互终端（PTY 双向转发）
- [x] **CORE-02**: 前端窗口 resize 时服务端同步调整 PTY 尺寸（TIOCSWINSZ）
- [x] **CORE-03**: 终端标题变化同步到浏览器标签页标题
- [x] **CORE-04**: 默认只读模式（丢弃客户端输入），显式开启可写后才接受输入
- [x] **CORE-05**: WS 异常断开后前端自动重连并接回同一 PTY 进程（共享进程模型；无滚动回放，屏幕内容靠程序重绘或 tmux/herdr 恢复）
- [x] **CORE-06**: WS ping/pong 保活，间隔可配置，防止反代空闲超时断连

### 前端（FE）

- [x] **FE-01**: 基于 xterm.js 6 渲染，WebGL 渲染器失败时自动回落 DOM 渲染器
- [x] **FE-02**: Unicode 11 宽字符支持，CJK/IME 正常输入显示
- [x] **FE-03**: 浏览器窗口变化时终端自动 fit 适配
- [x] **FE-04**: 终端输出中的 URL 自动识别为可点击超链接
- [x] **FE-05**: 选中即复制，剪贴板使用 navigator.clipboard 现代 API（替代已废弃的 execCommand）
- [x] **FE-06**: 辅助交互可开关：resize 时显示 COLSxROWS 浮层、离开页面前确认
- [x] **FE-07**: 客户端偏好（fontSize/theme 等 xterm 选项）可由服务端下发，URL query 可覆盖

### 会话生命周期（SESS）

- [x] **SESS-01**: --once 模式：只接受一个客户端，其断开后服务端退出
- [x] **SESS-02**: 可配置"所有客户端断开后退出"模式
- [x] **SESS-03**: 子进程退出后客户端收到明确提示（类型化错误帧，含退出码），而非静默断开

### 多客户端（MULTI）

- [x] **MULTI-01**: 多个 WS 客户端可同时 attach 同一会话，输出实时扇出
- [x] **MULTI-02**: 写入权限可配置：全员可写（协作排障）/ 仅 owner 可写其余只读（演示旁观）
- [x] **MULTI-03**: 慢客户端不阻塞其他客户端：每客户端有界 outbox，写满则以 1013 断开该客户端（重连后从最新输出看起）
- [x] **MULTI-04**: 多客户端 resize 仲裁（单客户端 last-wins；≥2 客户端最小公共矩形；2→1 恢复）
- [x] **MULTI-05**: 服务端启动时打印含一次性 token 的 ro/rw 两条分享链接，即打即用

### 安全（SEC）

- [x] **SEC-01**: 凭据时序安全比较（crypto/subtle），凭据不明文出现在任何日志
- [x] **SEC-02**: WS 认证采用一次性短时令牌（单次使用、短 TTL、绑定会话与权限级别），替代 ttyd 的 /token 明文下发
- [x] **SEC-03**: 认证失败节流（指数退避/速率限制），防止暴力破解
- [x] **SEC-04**: WS 握手 Origin 允许列表校验，不在列表内拒绝
- [x] **SEC-05**: TLS 最低 1.2（默认 1.3），合理 cipher 套件，安全响应头（HSTS/X-Content-Type-Options 等）
- [x] **SEC-06**: 子进程环境变量白名单，不继承服务端全部 env（防密钥泄露进 Web shell）
- [x] **SEC-07**: 反代 auth-header 透传：可信 HTTP 头注入的用户名记录进服务端审计日志（remote_user 审计归因）
  - *D-15 修订注记*：原「作为子进程环境变量」语义在 GoTTY 共享进程模型下结构性不成立（PTY 随服务端启动、spawn 时无 HTTP 请求；多客户端共享一个 shell，env 是一次性快照），收窄为服务端审计归因——头值经清洗后只进日志，不做任何认证决定（2026-08-25 Phase 7 CONTEXT D-15 裁决）
- [x] **SEC-08**: 认证完成前零缓冲分配（防 ttyd 式预认证内存放大/崩溃）

### 资源控制（RES）

- [x] **RES-01**: WS 消息三层上限：单帧长度、分片数量、累积字节数
- [x] **RES-02**: 每客户端输入速率限制
- [x] **RES-03**: 最大并发客户端数限制，满员拒绝新连接
- [x] **RES-04**: PTY 输出背压：客户端消费不及时时暂停读 PTY 或断开慢客户端

### 部署运维（OPS）

- [x] **OPS-01**: 监听配置：端口（0=随机并打印实际端口）/绑定地址/UNIX socket（含属主设置）
- [x] **OPS-02**: 反代子路径挂载（base-path）
- [x] **OPS-03**: 自定义首页 HTML
- [x] **OPS-04**: 子进程 cwd/TERM/关闭信号可配置（信号发给进程组）
- [x] **OPS-05**: 降权运行（setuid/setgid）
- [x] **OPS-06**: /healthz 健康检查端点
- [x] **OPS-07**: /metrics 监控端点（连接数、会话数、收发字节数）
- [x] **OPS-08**: 结构化日志（JSON），含审计事件（认证失败、连接建立/断开、会话生命周期）
- [x] **OPS-09**: 配置文件支持，CLI 参数覆盖配置文件
- [x] **OPS-10**: 单静态二进制发布（linux/darwin × amd64/arm64），前端 embed 内嵌为单 HTML
- [x] **OPS-11**: 可选启动后自动打开浏览器

## v1.1 Requirements（per-client 会话模式）

> 来源：2026-09-02 生态研究（.planning/research/FEATURES.md）——per-connection spawn 为品类标准答案（ttyd/GoTTY/wetty/shellinabox 全部如此），wesh shared 模型为差异化本体保持默认。T1–T11 table stakes 整组纳入；D4/D5/D6 经用户裁决纳入；D5 = 重开 D-15（per-client 下 attach 后 spawn，HTTP 上下文在手，收窄理由结构性消失）。

### 会话模式（PC）

- [x] **PC-01**: 用户可通过 `--session-mode=shared|per-client` flag（或 TOML `session_mode` 键）选择会话模式；缺省 shared，v1.0 全部行为逐字节不变
- [ ] **PC-02**: per-client 模式下每个 WS 客户端 attach 认证通过后独立 spawn 自己的 PTY 子进程（Hello cols/rows 经钳制后作初始 winsize）；spawn 失败时该客户端收类型化 Error 帧并以 1011 关闭，服务端与其他客户端不受影响
- [ ] **PC-03**: per-client 客户端断开（含异常）后其子进程进程组立即收 SIGHUP（随 `--stop-signal` 可配），无宽限；信号发送与收割序列化，杜绝 kill-after-reap 误杀复用 pgid
- [ ] **PC-04**: per-client 子进程退出后仅该客户端收私有 EXIT 帧（含 exit_code，信号死亡 -1）并以 1000 关闭；服务端与其他客户端继续运行
- [ ] **PC-05**: per-client 模式下 RESIZE 直通本会话 TIOCSWINSZ（钳制 [1,1000] 与 50ms 防抖保留），无仲裁器、无 'W' 约束帧
- [ ] **PC-06**: per-client 模式下断线重连成功即获得全新进程；前端按 Welcome 下发的模式位在重连分支执行 terminal.reset() 清屏（旧屏残留对新进程无意义）
- [ ] **PC-07**: ro 客户端在 per-client 模式下照常 spawn 独立进程，其 INPUT 被服务端丢弃（ro=自有进程输入门控）；每客户端输入限速保留
- [ ] **PC-08**: per-client 模式下 `--max-clients` 兼任并发进程上限：握手 503 闸保留 + spawn 前 hubMu 内复检计数（防 ttyd 式 == 闸 + 异步 spawn 窗口的并发超编）；并发子进程数 ≤ max-clients 为硬不变量
- [ ] **PC-09**: `--once` / `--exit-when-empty` / 优雅关停语义适配：触发条件（计数归零）不变，终结目标为全部存活 per-client 进程组各执行一遍 stop-signal 序列；注册表空迁移存在显式第二终结源（无子进程可等时仍能退出）
- [ ] **PC-10**: per-client 慢客户端保护：每客户端有界 outbox 写满 1013 踢出（无全局信用门；自然反压为停读该 PTY→内核缓冲满→子进程写阻塞）
- [ ] **PC-11**: per-PTY 停读/续读背压（ttyd pty_pause/resume parity）：慢客户端先停读其 PTY 而非立即踢出，恢复后自动续读；持续过载仍按 PC-10 踢出
- [ ] **PC-12**: 模式语义文档：README/CONFIGURATION/ARCHITECTURE 补 per-client 模型段（分享链接=按权限级别的独立进程入场券、ro=自有进程输入门控、配合 herdr/tmux 时经多路复用汇聚）；修正 v1.0「GoTTY 式共享进程模型」误记（GoTTY 实为 per-connection spawn，源码已核实）
- [ ] **PC-13**: herdr/tmux 等多路复用应用场景下多客户端互不干扰：移动端 attach 不再压缩其他客户端面板尺寸（herdr is_foreground + per-client area 仲裁恢复生效）；协议层 UAT 断言进程独立/尺寸互不干扰 + Windows Playwright 全链观感断言

### 安全（SEC 续）

- [ ] **SEC-09**: per-client 模式下 `--auth-header` 透传的用户名注入该客户端子进程环境变量（`WESH_REMOTE_USER`；键名白名单固定、值沿用 SEC-07 sanitize 清洗）；shared 模式保持 D-15 收窄语义（仅审计归因）不变

### 部署运维（OPS 续）

- [ ] **OPS-12**: /metrics 与审计日志 per-client 粒度：活跃会话数 gauge、spawn/kill 计数器、会话生命周期事件带 pid 归因；零身份 label 红线保持

## v2 Requirements

### 文件传输与渲染增强

- **V2-TRZSZ**: trzsz 文件传输（拖拽上传）
- **V2-ZMODEM**: ZMODEM 文件传输
- **V2-SIXEL**: Sixel 终端图片显示

### 会话增强

- **V2-SESSION**: 会话保持：连接解耦（断线进程不死）、环形缓冲滚动回放、空闲保活回收（v1 由 tmux/herdr 覆盖该需求，性价比不足延期；需要时重评估）
- **V2-RECORD**: 会话录制导出 asciicast v2/v3（依赖会话抽象，跟随 V2-SESSION 之后）
- **V2-ACME**: ACME/Let's Encrypt 自动证书（autocert，公网域名场景）
- **V2-CMDTPL**: 命令模板：服务端预定义命令集，客户端按编号选择（?arg= 的安全替代）

### 平台

- **V2-WINDOWS**: Windows ConPTY 支持

## Out of Scope

| Feature | Reason |
|---------|--------|
| E2E 加密（sshx/termpair 式） | 与会话保持/滚动回放本质冲突（服务端须持明文缓冲）；自托管场景威胁模型不成立，TLS+认证已足够 |
| URL ?arg= 任意传参 | ttyd 已核实注入面（无校验无上限拼接）；v2 以命令模板形式安全替代 |
| 多会话管理 UI（tabs/dashboard） | 拖向平台化，复杂度爆炸；需要多终端就在 wesh 里跑 tmux |
| SaaS relay / 托管服务 / 账户体系 | 运维成本与隐私责任；单二进制自托管定位 |
| 多租户 / RBAC 用户库 | 个人工具边界；ro/rw 双令牌已覆盖协作与演示 |
| mosh 式预测回显 / 多光标协作 | 复杂度极高，1-3 人场景无收益 |
| 浏览器内编辑器 / GUI 化 | 另一个产品（code-server 方向） |
| ttyd CLI 参数兼容 | 用户明确决策：全新设计，不背兼容包袱 |
| 服务端重启后会话恢复 | 需 CRIU 类技术，复杂度与收益不匹配；断线保活已覆盖主要痛点 |
| 依赖外部 tmux 实现会话保持 | 破坏单二进制零依赖承诺；原生实现 |
| per-client 重连 reattach（sessionKey + 服务端输出缓冲） | v1.1 反特性 A1/A4：等于把 V2-SESSION 偷渡进 per-client；持久性由子进程侧 herdr/tmux 承接（per-client 模式的存在意义） |
| per-client 断开后 linger 宽限再杀进程 | v1.1 反特性 A2：半吊子会话保持，宽限窗内进程占资源且收割竞态面增大；ttyd/wetty 均立即杀 |
| 运行期/按 URL 切换会话模式 | v1.1 反特性 A3：?arg= 注入面前车之鉴；运行期切模式使生命周期不变量双份化；替代=起两个实例不同端口 |
| per-client 设为默认模式 | v1.1 反特性 A5：违背 v1.0 零回归承诺；shared（真·多人同屏）是差异化本体，per-client 显式 opt-in |
| ro 访客共享单个进程省资源 | v1.1 反特性 A7：直接重新引入 driving bug（移动端 attach 缩小所有人面板）；进程开销由 PC-08 上限管控 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CORE-01 | Phase 1 | Complete |
| CORE-02 | Phase 1 | Complete |
| CORE-03 | Phase 4 | Complete |
| CORE-04 | Phase 2 | Complete |
| CORE-05 | Phase 6 | Complete |
| CORE-06 | Phase 2 | Complete |
| FE-01 | Phase 1 | Complete |
| FE-02 | Phase 4 | Complete |
| FE-03 | Phase 1 | Complete |
| FE-04 | Phase 4 | Complete |
| FE-05 | Phase 4 | Complete |
| FE-06 | Phase 4 | Complete |
| FE-07 | Phase 4 | Complete |
| SESS-01 | Phase 6 | Complete |
| SESS-02 | Phase 6 | Complete |
| SESS-03 | Phase 6 | Complete |
| MULTI-01 | Phase 5 | Complete |
| MULTI-02 | Phase 5 | Complete |
| MULTI-03 | Phase 5 | Complete |
| MULTI-04 | Phase 5 | Complete |
| MULTI-05 | Phase 5 | Complete |
| SEC-01 | Phase 3 | Complete |
| SEC-02 | Phase 3 | Complete |
| SEC-03 | Phase 3 | Complete |
| SEC-04 | Phase 3 | Complete |
| SEC-05 | Phase 3 | Complete |
| SEC-06 | Phase 1 | Complete |
| SEC-07 | Phase 7 | Complete |
| SEC-08 | Phase 2 | Complete |
| RES-01 | Phase 2 | Complete |
| RES-02 | Phase 5 | Complete |
| RES-03 | Phase 5 | Complete |
| RES-04 | Phase 5 | Complete |
| OPS-01 | Phase 7 | Complete |
| OPS-02 | Phase 7 | Complete |
| OPS-03 | Phase 9 | Complete |
| OPS-04 | Phase 7 | Complete |
| OPS-05 | Phase 7 | Complete |
| OPS-06 | Phase 8 | Complete |
| OPS-07 | Phase 8 | Complete |
| OPS-08 | Phase 8 | Complete |
| OPS-09 | Phase 7 | Complete |
| OPS-10 | Phase 9 | Complete |
| OPS-11 | Phase 7 | Complete |
| PC-01 | Phase 10 | Complete |
| PC-02 | Phase 11 | Pending |
| PC-03 | Phase 11 | Pending |
| PC-04 | Phase 11 | Pending |
| PC-05 | Phase 12 | Pending |
| PC-06 | Phase 12 | Pending |
| PC-07 | Phase 12 | Pending |
| PC-08 | Phase 13 | Pending |
| PC-09 | Phase 13 | Pending |
| PC-10 | Phase 12 | Pending |
| PC-11 | Phase 12 | Pending |
| PC-12 | Phase 14 | Pending |
| PC-13 | Phase 14 | Pending |
| SEC-09 | Phase 13 | Pending |
| OPS-12 | Phase 13 | Pending |

**Coverage:**

- v1 requirements: 44 total（原写 42，按实际条目数修正：CORE 6 + FE 7 + SESS 3 + MULTI 5 + SEC 8 + RES 4 + OPS 11）— 全部 Complete
- v1.1 requirements: 15 total（PC 13 + SEC 1 + OPS 1）
- Mapped to phases: 44 (v1) + 15 (v1.1)
- Unmapped: 0

---
*Requirements defined: 2026-08-13*
*Last updated: 2026-09-03 — v1.1 原 Phase 13/14 合并、原 15 重编号 14：15 条需求全量映射 Phase 10-14（PC-01→10；PC-02/03/04→11；PC-05/06/07/10/11→12；PC-08/09+SEC-09+OPS-12→13；PC-12/13→14），覆盖 15/15 无孤儿*
