---
phase: 05
slug: multi-client
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-21
---

# Phase 05 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| PTY→WS fan-out | 子进程输出（可信）经 hub 扇出到多个不可信客户端——慢/死客户端不得反向拖死生产者 | 终端输出字节流 |
| ReadLoop↔outbox 缓冲边界 | chunk 底层缓冲复用——跨帧持有未拷贝即数据损坏 | 帧缓冲引用 |
| 慢客户端→全局背压 | 不可信客户端消费速度决定信用门开合 | 信用门状态 |
| PTY 前台进程组信号面 | SIGWINCH 发往 TIOCGPGRP 取到的进程组——失败须静默降级 | 控制信号 |
| 客户端→写权限 | ro→rw 唯一通道是服务端 FIFO 递补推送 | 权限状态 |
| rw 输出→ro 旁观者 | 共享终端内容（含 OSC52）扇出到旁观者 | 终端序列 |
| 客户端 RESIZE→PTY 全局尺寸 | 不可信尺寸请求影响全体渲染 | 尺寸元数据 |
| RESIZE 频率→SIGWINCH 风暴 | 高频 resize 驱动 TIOCSWINSZ/SIGWINCH | 控制信号频率 |
| 客户端 INPUT→PTY master | 不可信输入字节流直抵子进程 stdin | 输入字节流 |
| 未认证 HTTP→/s/{token}/ 与 /api/attach | token 是唯一凭证——爆破/枚举/泄露即越权 | 认证 token |
| token→日志/错误响应面 | token 出现在 URL 路径——其他一切输出面必须零泄露 | 凭证材料 |
| 未认证/已认证客户端→连接槽位 | 连接数是有限容量——耗尽即合法用户不可用 | 槽位计数 |
| URL→token→POST body | token 经 URL 进入前端——任何额外持久化/日志化都是泄露面 | 凭证材料 |
| 服务端关闭帧→状态面板 | 关闭码/reason 来自服务端可被伪造——前端不得渲染远端内容 | 关闭帧文本 |
| UAT 输出/文档→token | 测试控制台与 README 是持久面 | 凭证样本 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | DoS | hub onChunk 扇出 | high | mitigate | trySend 非阻塞 + Close(1013) 踢出（internal/server/clients.go:140,488）；TestMultiClientFanout + TestSlowConsumerKick | closed |
| T-05-10 | Tampering | chunk 别名跨帧持有 | high | mitigate | 单帧 make+copy（clients.go:351-353）；writer 共享只读帧（:572-576） | closed |
| T-05-01b | DoS | writer 写阻塞 | medium | mitigate | per-client writer goroutine（clients.go:556）；pinger 写超时→CloseNow（server.go:860-877） | closed |
| T-05-01d | Info Disclosure | 1013 reason 机器串 | low | accept | 静态机器串不携内部状态；前端 onclose 只认 code（main.ts:649-650） | closed |
| T-05-02 | DoS | 信用门死锁 | high | mitigate | pinger 独立于 hubMu；hubCond.Wait 原子释放（clients.go:349）；Broadcast 四挂点齐（:643,:486; server.go:698,953）；TestGlobalCredit | closed |
| T-05-01c | DoS | 踢出误伤慢合法读者 | medium | mitigate | 全体可写端同步满才持块不踢（clients.go:397-416）；100%/50% 2:1 迟滞带（:435） | closed |
| T-05-11a | DoS | SIGWINCH 阻塞 attach | low | mitigate | 单次 ioctl+kill 无阻塞面（pty/io.go:56-60）；TIOCGPGRP 失败静默 | closed |
| T-05-08 | EoP | owner 递补权限竞态 | high | mitigate | FIFO 递补（promoteNextLocked clients.go:518）；removeLocked 后同一 hubMu 内晋升（:470,482-484）；TestSuccession + TestSuccessionKickRace | closed |
| T-05-07 | Tampering | OSC52 劫持旁观者剪贴板 | high | mitigate | 双档 blob：ro 档永不含 osc52 键（server.go:52-53,135-139,645,675）；TestOwnerPolicy | closed |
| T-05-08b | EoP | --write-policy=all 误开 | medium | mitigate | 默认 owner（main.go:80）；parse 枚举校验（:167-168）；无 --writable fail-fast（:283-285） | closed |
| T-05-11 | DoS | RESIZE 风暴 | medium | mitigate | 50ms 单 timer 防抖（clients.go:55; resize.go:71-93）；ClampDim [1,1000]（resize.go:18）；变化才 Resize（:127-131）；TestResizeArbitration | closed |
| T-05-11b | Tampering | 仲裁撑出客户端窗口 | medium | mitigate | min-rect 不变量（resize.go:44-45）；旁观者不参与（:139,:88-90）；TestArbitrate | closed |
| T-05-03 | DoS | 输入洪水 | high | mitigate | 32KiB/s+64KiB burst（clients.go:42,46）；256KiB 有界 inputQ（:51）；ReadLimit 16KiB（server.go:700）；TestInputRateLimit | closed |
| T-05-03b | DoS | 输入写阻塞读循环 | high | mitigate | 单 input-writer goroutine 独占 Master.Write（server.go:283; clients.go:601-614）；Drain→Close 解除在途写（server.go:926-929） | closed |
| T-05-SC | Tampering | x/time 供应链 | high | mitigate | 钉版 v0.15.0（go.mod:11）；go.sum 入库；rationale 注释（clients.go:22） | closed |
| T-05-05 | Info Disclosure | 分享 token 爆破 | high | mitigate | crypto/rand 16B（sharetoken.go:50）；subtle.ConstantTimeCompare 位或不短路（:65-66）；失败经 401 统一 per-IP 退避（:81-82）；TestShareToken 无 oracle | closed |
| T-05-06 | Info Disclosure | token 经日志/错误泄露 | high | mitigate | token 永不入 logEvent；MaxBytesReader 4KiB（server.go:364）；解析失败不回显（:371-376） | closed |
| T-05-05b | EoP | token 通道削弱 Basic 矩阵 | medium | mitigate | 无/错 token → Basic 矩阵逐字节一致（sharetoken.go:81-84）；auth_e2e 回归套件在；NAT 误伤论证注释（server.go:294,314） | closed |
| T-05-06e | Info Disclosure | token 经端点环境面泄露 | medium | accept | D-03 capability-URL 取舍；README 暴露面清单+TLS 建议（README.md:173） | closed |
| T-05-04 | DoS | 连接数耗尽 | high | mitigate | ③位 503 pre-WS 闸（server.go:401,546）；per-IP halfOpen 429（auth.go:92-93）；/api/attach 早闸；TestMaxClients503 | closed |
| T-05-04b | DoS | 计数器泄漏 | high | mitigate | registry.n atomic.Int64（clients.go:253）；registerLocked/removeLocked 唯一加减（:262-290）；TestClientCountInvariant | closed |
| T-05-04c | DoS | 瞬时超编竞态 | low | accept | R-06 容量非安全边界；超编 ≤ halfOpen 帽 8（README.md:195） | closed |
| T-05-09 | Spoofing | 1013/关闭帧钓鱼 | medium | mitigate | onclose 只认 ev.code（main.ts:602,620）；文案硬编码英文（:389-390,:404,:649-660）；Reload 仅 location.reload()（:337） | closed |
| T-05-06b | Info Disclosure | token 经前端面泄露 | high | mitigate | shareToken 仅 connect 闭包+POST body（main.ts:360,375）；replaceState 全仓 grep == 0 | closed |
| T-05-09b | Tampering | 升格 Welcome 伪造 | low | accept | TLS+Origin+ticket 既有；服务端 INPUT 门按注册表 mode 独立判定（server.go:94） | closed |
| T-05-06c | Info Disclosure | token 经测试输出/README 泄露 | high | mitigate | UAT 红线注释（phase05.mjs:7-9,60-61,179）；README 占位符（:48-49,153） | closed |
| T-05-06d | Info Disclosure | 反代访问日志泄露 | medium | mitigate | README 反代脱敏节：nginx map+log_format / access_log off 双形态（README.md:148-161） | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-05-A | T-05-01d | 1013 reason 静态机器串不携内部状态；前端不渲染 reason，无钓鱼面 | D-10 (PLAN) | 2026-08-21 |
| R-05-B | T-05-06e | capability-URL 模型端点环境面取舍；端点失陷则凭据/会话同样不保；明文 http:// 由 README TLS 部署建议兜底 | D-03 (PLAN) | 2026-08-21 |
| R-05-C | T-05-04c | 容量策略非安全边界；瞬时超编 ≤ per-IP 半开帽 8，内存/goroutine 开销微小；Phase 9 负载标定时复核 | R-06 (PLAN) | 2026-08-21 |
| R-05-D | T-05-09b | 伪造 Welcome 帧无法让服务端接受 INPUT——服务端按注册表 mode 独立判定写权限 | PLAN 05-08 | 2026-08-21 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-21 | 27 | 27 | 0 | gsd-security-auditor |

### Audit 2026-08-21 注记（非阻塞）

- **T-05-01b 文档漂移**：pinger 写超时实际默认 10s（D-16），PLAN 文本写 5s——结构性缓解（有界超时收口死连接）在位，常量级漂移不阻塞。
- **路径漂移**：注册表 `files_to_check` 列登记 `internal/hub/*.go`，实际 hub 职能落位于 `internal/server/`（clients.go/resize.go/server.go）。所有模式已在实际路径核实，非缺失。

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-21
