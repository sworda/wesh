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
- [ ] **OPS-03**: 自定义首页 HTML
- [x] **OPS-04**: 子进程 cwd/TERM/关闭信号可配置（信号发给进程组）
- [x] **OPS-05**: 降权运行（setuid/setgid）
- [ ] **OPS-06**: /healthz 健康检查端点
- [ ] **OPS-07**: /metrics 监控端点（连接数、会话数、收发字节数）
- [ ] **OPS-08**: 结构化日志（JSON），含审计事件（认证失败、连接建立/断开、会话生命周期）
- [x] **OPS-09**: 配置文件支持，CLI 参数覆盖配置文件
- [ ] **OPS-10**: 单静态二进制发布（linux/darwin × amd64/arm64），前端 embed 内嵌为单 HTML
- [x] **OPS-11**: 可选启动后自动打开浏览器

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
| OPS-03 | Phase 9 | Pending |
| OPS-04 | Phase 7 | Complete |
| OPS-05 | Phase 7 | Complete |
| OPS-06 | Phase 8 | Pending |
| OPS-07 | Phase 8 | Pending |
| OPS-08 | Phase 8 | Pending |
| OPS-09 | Phase 7 | Complete |
| OPS-10 | Phase 9 | Pending |
| OPS-11 | Phase 7 | Complete |

**Coverage:**

- v1 requirements: 44 total（原写 42，按实际条目数修正：CORE 6 + FE 7 + SESS 3 + MULTI 5 + SEC 8 + RES 4 + OPS 11）
- Mapped to phases: 44
- Unmapped: 0 ✓

---
*Requirements defined: 2026-08-13*
*Last updated: 2026-08-13 after roadmap creation（traceability 填充；Core Value 与 PROJECT.md 对齐——v1 不做会话保持，"断线不丢"改为"可多人共享"）*
