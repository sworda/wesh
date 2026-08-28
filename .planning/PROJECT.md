# wesh

> Share terminal over web —— 用现代技术路线重写的 Web 终端分享工具

## What This Is

wesh 是一个"通过 Web 分享终端"的命令行工具：`wesh [options] <command> [args...]` 启动后在指定端口提供 HTTPS/WebSocket 服务，浏览器打开页面即获得一个运行 `<command>` 的完整终端。它是对 ttyd 1.7.7 的现代化重写，面向个人运维场景，在保持 ttyd"单静态二进制、scp 上去就能跑"核心优势的同时，原生解决 ttyd 的无多路复用、安全缺陷、资源控制缺失等已核实问题（会话保持由 tmux/herdr 覆盖，v1 不做）。

## Core Value

**浏览器里获得一个可靠、安全、可多人共享的远程终端。** 其他一切（文件传输、会话保持、Sixel）都可以后续迭代，但"打开页面就有可用的安全终端、能方便地分享给别人看/操作"必须成立。

## Requirements

### Validated

- ✓ 启动时指定任意命令及参数，浏览器获得完整交互终端（PTY 双向转发）— Phase 1（CORE-01；TestEchoPTY + 生命周期五测 + UAT 浏览器实测）
- ✓ 终端尺寸同步（前端 resize → 服务端 TIOCSWINSZ）— Phase 1（CORE-02；TestResize 24 80→50 132 + UAT vim resize 跟随实测）
- ✓ 子进程环境变量白名单（不继承父进程全部 env）— Phase 1（SEC-06；TestEnvWhitelist 双层断言宿主注入 AWS_SECRET_ACCESS_KEY 不可见）
- ✓ 只读/可写模式（默认只读，显式 --writable 才接受输入；服务端真边界丢 INPUT）— Phase 2（CORE-04；TestReadOnlyDropsInput/TestHelloWelcome + UAT 自动化标记串零回显 + 浏览器实测）
- ✓ WS 消息长度上限与分片重组缓冲上限（两档字节硬顶 4KiB/16KiB 库流式执行，超限 1009；预认证内存放大消除）— Phase 2（RES-01/SEC-08；limits 五测 -race PASS）
- ✓ WS ping/pong 保活（可配间隔，默认 5s，0 禁用；仅 pong 超时断开，读路径恒无 deadline）— Phase 2（CORE-06；保活三测 PASS + UAT 自动化 11s+ 存活）
- ✓ 版本化 WS 协议 wesh.v1（类型化握手/错误帧、子协议双闸、合规关闭码 {1000,1002,1008,1009}、1006 永不发送）— Phase 2（SEC-08；守卫链七测 + UAT 关闭路径自动化实测）
- ✓ 认证：时序安全比较、凭据不明文进日志、一次性短时令牌（单次使用/60s TTL/mode 绑定）— Phase 3（SEC-01/02/03；ticketStore+subtle 比较 + phase03.mjs 六场景 UAT）
- ✓ 认证失败节流防爆破（1s×2 封顶 30s 指数退避/成功清零）— Phase 3（SEC-03；throttleStore + UAT 退避窗口实测）
- ✓ Origin 允许列表校验（规范化比对）— Phase 3（SEC-04；03-02/03-03 守卫链 ⓪ 位）
- ✓ TLS（MinVersion 1.2 + 6 AEAD cipher + 安全响应头 + 证书启动预检）— Phase 3（SEC-05；03-02/03-04/03-07）
- ✓ 窗口标题同步（OSC 2 单一写口，ro 形态恒 `[ro] ` 前缀最前）— Phase 4（CORE-03；UAT T3 5/5）
- ✓ 前端 xterm.js 生态：Unicode 11/CJK/IME、超链接（裸 URL + OSC 8 双通道）、现代剪贴板（选中即复制 150ms 防抖/Ctrl+Shift+V/安全上下文静默降级/OSC52 write-only）— Phase 4（FE-02/FE-04/FE-05；UAT T1-T2/T4-T7 全过）
- ✓ 客户端偏好下发（--client-option 白名单 + Welcome prefs + query 覆盖 + theme 合并不丢内置调色板）— Phase 4（FE-07；UAT T10 6/6）
- ✓ 原生多客户端 attach 同一会话，写入权限可配置（全员可写 / 主写旁观 + 递补升格）— Phase 5（MULTI-01/02；TestMultiClientFanout/TestAllPolicy/TestOwnerPolicy -race 绿 + S1b 双端 338958 字节逐字节一致；异尺寸按 min-rect 约束渲染经 S10/D6/D6H 三层锁定）
- ✓ 慢客户端不拖累他人：有界 outbox 写满 1013 踢出、重连从最新输出看起、PTY 读循环永不阻塞 — Phase 5（MULTI-03/RES-04；TestSlowConsumerKick/TestGlobalCredit -race 绿 + S6 三断言）
- ✓ 背压控制与每客户端限速（全局信用门 + 每客户端输入速率限制）— Phase 5（RES-02/RES-04；TestInputRateLimit/TestGlobalCredit -race 绿）
- ✓ 最大并发客户端数限制（满员 503 + 客户端计数不变量）— Phase 5（RES-03；TestMaxClients503/TestClientCountInvariant + S5）
- ✓ 多客户端 resize 仲裁（单端 last-wins / ≥2 端 min-rect / 2→1 恢复）与 ro/rw 一次性分享链接即打即用 — Phase 5（MULTI-04/05；TestArbitrate/TestResizeArbitration + TestShareToken + S2-S4 全链）
- ✓ WS 异常断开后前端自动重连并接回同一 PTY 进程（共享进程模型；无滚动回放，屏幕内容靠程序重绘或 tmux/herdr 恢复）— Phase 6（CORE-05；仅 1006 触发 + 退避 1s×2 封顶 30s；phase06-dom D1/D8 + Playwright 实测断网 30s 恢复同 pid 接回）
- ✓ --once 模式：只接受一个客户端，其断开后服务端退出 — Phase 6（SESS-01；--once ≡ --max-clients=1 --exit-when-empty=0；S3 双点位 503 + 进程退出 255 + Playwright T5 全链）
- ✓ 可配置"所有客户端断开后退出"模式 — Phase 6（SESS-02；--exit-when-empty[=duration] 立即/宽限取消/宽限到期三形态，S4/S5 锁定）
- ✓ 子进程退出后客户端收到明确提示（类型化错误帧，含退出码），而非静默断开 — Phase 6（SESS-03；EXIT 帧三形态文案 + EXIT→1000 广播序列；S1/S2 + Playwright T4 双形态逐字 + 双端一致广播）
- ✓ 监听配置：端口（0=随机并打印实际端口）/绑定地址/UNIX socket（--socket/--socket-mode/--socket-owner；活性探测拒存活实例防静默赢者，残留自动清理不回归）— Phase 7（OPS-01；TestListenSocket 六子测 + phase07.mjs S2 + b1b5.sh 7/7 二进制直证）
- ✓ 反代子路径挂载（--base-path parse 期严格校验；README nginx 配方经双机全链实证——proxy_set_header Host $http_host 为 WS 同源校验放行前提，精确块 308 保方法）— Phase 7（OPS-02；TestBasePathRoutes/WS + phase07.mjs S3a-h + phase07-a2-pw.mjs 5/5 真 nginx 全链）
- ✓ 子进程 cwd/TERM/关闭信号可配置（信号发进程组，setsid pgid==pid 不变量防误杀）— Phase 7（OPS-04；TestStartOptionsDir/TestSignalGroup + phase07.mjs S5）
- ✓ 降权运行（--uid/--gid 数字直通成对强制 exit 2 零窗口；HOME/USER/LOGNAME 按 LookupId 改写、查不到剔除三键）— Phase 7（OPS-05；TestDropPrivilegesSelf/IdentityEnv + phase07.mjs S6 降权 self 全链）
- ✓ 配置文件支持（--config TOML 显式加载、CLI>env>file 优先级、DisallowUnknownFields 严格拒绝、错误三要素零敏感值回显、权限非 0600/0400 警告）— Phase 7（OPS-09；TestLoadFileConfig/TestConfigMerge/TestConfigPrecedence/TestConfigRedLines + S1 + B5 4/4）
- ✓ 反代身份透传审计归因（--auth-header 头值 sanitize C0/C1/DEL+128 rune 截断入日志 remote_user；XFF 换键同步节流计数；零认证效力）— Phase 7（SEC-07；TestRemoteUserLogging/TestSanitizeRemoteUser/TestXFFThrottleKey + S4 + b2.mjs 4/4）
- ✓ 优雅关停（SIGTERM/SIGINT → 1001 广播 → 退出码 255；stall 客户端内建 5s+5s 上界不拖延退出）与 --open 自动打开浏览器（headless 跳过提示、opener 非零退出 stderr 警告不阻断、goroutine Wait 收割零僵尸、警告行结构性不含 URL）— Phase 7（D-23/OPS-11；phase07.mjs S7/S8 + b6.sh 7/7）
- ✓ /healthz 健康检查端点（免认证唯一窄例外 D-07，四字段键集白名单、draining 两态、503 摘流观测）— Phase 8（OPS-06；TestHealthz/TestHealthzDraining + phase08.mjs S1/S4 + 实机 systemctl restart 轮询 200→503×15→000）
- ✓ /metrics 监控端点（17 条 wesh_* series 零身份 label、build_info 转义、basic_auth 闸跟随、快照锁序防死锁）— Phase 8（OPS-07；TestMetricsExposition/TestMetricsAuth/TestMetricsSnapshotRace + phase08.mjs S2/S3 + Prometheus 2.55.1 实机 scrape 17 series 全入库）
- ✓ 结构化审计日志（slog JSON 单行事件 stderr 输出、auth_failed 无用户名红线、C0/C1 注入剥离、journald+jq 检索示例可用）— Phase 8（OPS-08/SEC-01；TestAuthFailedNoUsername/TestRemoteSanitize + phase08.mjs S5/S6 + 实机 sg systemd-journal 双示例 jq 零 parse error）

### Active

**核心终端（对标 ttyd）**
（Phase 1-6 已全部闭合，见 Validated）

**安全（改进 ttyd 限制 #3 + 源码核实的新发现）**
（Phase 3 已全部闭合，见 Validated）

**资源控制（改进 ttyd 限制 #4/#5）**
（Phase 5 已全部闭合，见 Validated）

**部署与集成**
（监听配置/反代子路径/子进程环境/降权/配置文件 Phase 7 已全部闭合，见 Validated）
- [ ] 自定义首页（Phase 9）

**可观测性**
（/healthz、/metrics、结构化日志 Phase 8 已全部闭合，见 Validated）

**质量底线**
- [ ] 修复源码核实的全部 ttyd 缺陷（见 Context 节清单）

### Out of Scope

- **ZMODEM 文件传输** — v2；依赖停更的 zmodem.js，且 trzsz 已覆盖现代场景
- **trzsz 文件传输** — v2；核心优先，v1 先不做
- **Sixel 图片** — v2
- **会话保持（断线保活/滚动回放/保活回收）** — v1 不做：用户日常以 tmux/herdr 管理终端会话，该能力已被覆盖，自研性价比不足；v2 视需要重评估
- **Windows (ConPTY) 支持** — 复杂度高，个人运维场景以 Linux/macOS 为主
- **服务端重启后会话恢复** — 需 CRIU 类技术，复杂度极高；断线保活已覆盖主要痛点
- **多租户 / 嵌入产品的 API 平台化** — 定位为个人运维工具，不做 SaaS 化
- **ttyd CLI 参数兼容** — 用户明确选择全新设计，不背兼容包袱
- **ttyd 式 ?arg= URL 传参** — 已核实注入面，v1 砍掉（Key Decisions）；v2 以命令模板安全替代。Phase 4 的 query 覆盖仅限 --client-option 白名单键，非命令注入面

## Context

**功能基线**：完整功能清单见 `~/open_src/ttyd/.codebuddy/ttyd-analysis/01-功能清单.md`。ttyd 后端 C 约 2100 行 + 前端 TS 约 940 行，架构为 libwebsockets + libuv + forkpty，前端 xterm.js。

**源码核实结论**（2026-08-13，Explore agent 对 ttyd 1.7.7 全量核实，含行号证据）：

原清单 6 条限制全部属实。另发现更严重问题：

*安全（严重）*
- 预认证远程崩溃：空 WS 消息导致空指针解引用，任何客户端可 DoS 整服（utils.c:34, protocol.c:298）
- 预认证内存放大：分片累积在认证检查之前（protocol.c:288-296）
- 凭据 base64 明文打印进日志（server.c:142）；超长凭据静默截断
- Origin 校验弱：仅字符串比对，可绕过（protocol.c:51-71）
- 子进程继承全部父环境变量，env 中密钥泄露给 Web shell（pty.c:441-444）
- ?arg= 无校验无上限拼接（protocol.c:241-249）
- TLS 仅禁 1.0/1.1，无 cipher 控制/HSTS/安全响应头
- pty_spawn 失败路径误 close(0)（pty.c:87,112, protocol.c:161）

*健壮性/性能*
- 关闭码 1006 写入 close frame 违反 RFC6455（protocol.c:90,105）
- 每数据块 3-4 次拷贝；固定 64KB 读缓冲且读后即停，吞吐受限（pty.c:40-66）
- 每客户端独占一条 waitpid 线程（pty.c:483）；单 libuv 循环承载全部 IO
- 无 /healthz、metrics、结构化日志、配置文件

*协议/前端*
- 无版本协商、无错误消息类型、spawn 失败原因无法传给客户端
- AuthToken 与 Basic 凭据同一 secret 复用
- zmodem.js 0.1.10（2017 年停更需本地 patch）、decko 停更、execCommand('copy') 已废弃

*libwebsockets 编程模型是 bug 高发区*：手工 LWS_PRE 预留、手写分片重组（两大预认证漏洞均在此）、pss 生命周期跨 lws/libuv 双域仅靠标志位防 UAF。重写应弃用裸 lws 回调状态机，改用高级框架。

**对重写的启示**：协议重设计（版本协商、类型化错误帧、认证并入握手、长度上限、合规关闭码）；会话与进程解耦（session 抽象支持挂接/共享/保活）；安全默认；零拷贝管道；SIGCHLD/pidfd 统一收割替代每进程一线程。

## Constraints

- **分发形态**: 单静态二进制 — ttyd 的核心优势，必须保持（scp 上去就能跑，无运行时依赖）
- **平台**: Linux + macOS — 个人运维主场景；Windows 不做
- **技术选型**: 后端语言/框架由调研决定（Rust vs Go 为主要候选） — 用户明确授权"选择最合适的"
- **前端**: xterm.js 生态（渲染器/CJK/fit 等 addon）— ttyd 已验证的正确选择，前端无重写必要
- **兼容性**: 不兼容 ttyd CLI 参数，全新设计 — 用户明确决策

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 项目名 wesh | web + shell；原名 stow 与 GNU Stow 严重撞名 | ✓ 落地 github.com/sworda/wesh（Phase 1） |
| 单静态二进制分发 | 保持 ttyd 核心优势 | — Pending（Phase 9 goreleaser 验证） |
| 全新 CLI 设计，不兼容 ttyd 参数 | 不背兼容包袱，怎么合理怎么设计 | ✓ Phase 1 CLI 契约落地（`--` 透传/默认 0.0.0.0:7681/无命令 usage 退 2/--version） |
| v1 不做会话保持 | 用户以 tmux/herdr 覆盖断线保活需求，自研性价比不足；架构上仍需为 v2 留出演进空间 | — Pending |
| 多客户端共享写入权限可配置 | 同时覆盖协作排障（全员可写）与演示教学（主写旁观） | — Pending（Phase 5） |
| v1 核心优先，ZMODEM/trzsz/sixel 放 v2 | 先把核心终端+安全做到位 | — Pending |
| ?arg= URL 传参 v1 砍掉 | 已核实注入面；v2 以命令模板安全替代 | — Pending |
| E2E 加密明确不做 | 自托管场景威胁模型不成立，TLS+认证足够 | — Pending |
| 后端语言由调研决定 → Go | 调研结论：creack/pty 纯 Go 生态、coder/websocket 根治 ttyd 两类漏洞、静态编译发布故事最顺 | ✓ Phase 1 行走骨架落地，-race 全绿 + ubuntu/macos 双平台 CI 通过 |
| darwin 收割用共享 kqueue exit watcher（非 SIGCHLD+WNOHANG 手动 reap） | EVFILT_PROC/NOTE_EXIT 早知 + cmd.Wait() 唯一收割；Q1 僵尸注册竞态由 CI 裁决 | ✓ Q1 裁决=watcher 成立（kqueue 对僵尸进程补发 NOTE_EXIT，TestKqueueExitNormal/ZombieRace CI 双 PASS），兜底路径休眠 |
| WS 上限三层改两层（D-09 修订） | coder/websocket SetReadLimit 流式截断已覆盖单帧+累积字节两层；分片数层库不暴露，以 1 字节分片洪水测试构成等效防线 | ✓ Phase 2 limits 五测 -race PASS；空帧洪水残余风险用户裁决接受 |
| CR-01（Attach 读循环同步写 PTY master 可永久阻塞）立即最小缓解 | 非协议层缺口（协议透明）但破坏 D-11 退出保证+可误杀健康连接；O_NONBLOCK+ErrWouldBlock 走既有收口，完整背压（有界输入队列+写 goroutine+1013）留 Phase 5 | — Pending（最小缓解待执行） |
| --client-option 校验错误记录式上报（clientOptErr + Parse 后统一返回） | flag 包 failf 会将回调错误包装为 `invalid value %q` 并把原始 key=value 串打到 stderr，必然违反启动面"值内容不外泄"红线；记录式两通道干净且 exit 2 语义不变 | ✓ Phase 4 落地，client-option 负场景 UAT 全过 |
| js-base64 等 overrides 落 web/pnpm-workspace.yaml 而非 package.json | pnpm 11 不再读 package.json 的 pnpm 字段（CI 钉 11.21.0 同版），overrides 官方新家即 pnpm-workspace.yaml | ✓ Phase 4 落地，lockfile 三处解析均 3.9.2 |
| 前端 Terminal 构造必须 `allowProposedApi: true` | xterm 6.0 的 unicode API 仍标 EXPERIMENTAL，缺省 false 时 loadAddon(Unicode11Addon) 模块顶层同步抛错 → connect() 永不执行、终端黑屏（UAT 自动化抓到的 P0） | ✓ Phase 4 修复并重建 dist，jsdom 套件即回归测试 |
| EXIT 广播写序安全形态：lifecycle 组帧一次共享只读 + 每客户端 goroutine 同步 Write(EXIT,2s ctx)→Close(1000) | stall 客户端不得拖延全局终结；禁 outbox 异步入队；2s 为 RESEARCH OQ3 定值拒绝可配化 | ✓ Phase 6 落地，TestExitFrameBroadcast + S1/S2 + Playwright T4 锁定；2s 标定挂账 Phase 9 |
| OQ1 裁决 accept-255：--once/--exit-when-empty 收口路径退出状态 255 | 子进程被 SIGHUP 终结，exitf 以 -1 收口、Unix 进程退出状态截断为 255；lifecycle 零分支改动，与 EXIT exit_code=-1 同源 | ✓ 用户 2026-08-23 裁决；Go 测试断言 -1 / 进程级 255 / README 文案三消费点单点落地 |
| 重连触发面收窄为仅 1006 + 无限重试（退避 1s×2 封顶 30s） | 1002/1013/1008 带码关闭语义确定不自动重连（防再踢循环/协议错误放大）；「标签页放着回来已接回」主场景 30s 一次流量可忽略 | ✓ Phase 6 落地（shouldReconnect/backoffMs）；Playwright T1 30s 退避观测实证 attempts 1→5 |
| --once ≡ --max-clients=1 --exit-when-empty=0 语法糖分层 | fs.Visit 显式设置位先行，展开只填未显式位，矛盾组合留 validateStartup fail-fast 拒绝（不静默改写用户输入） | ✓ Phase 6 落地，TestStartupMatrix/TestParseArgs 锁定 |
| 反代后 WS 同源校验放行必须 nginx `proxy_set_header Host $http_host;` | nginx 默认转发 Host=$proxy_host（127.0.0.1:后端口）与浏览器 Origin 不同源被 originAllowed 403；$host 剥端口在非默认端口仍不匹配（全链实证） | ✓ Phase 7 G-07-2 闭合：README 配方修正 + pw 双机回归 5/5 锁定（文档即被测物） |
| unix socket 存活/残留以活性探测再分（类型闸不可区分） | Lstat 类型闸后无条件 Remove 会让第二实例 unlink 存活 socket（静默赢者孤儿化前者）；net.Dial 连通即拒（EADDRINUSE 同形态文案 exit 1），TOCTOU 两向安全降级 | ✓ Phase 7 G-07-3 闭合：main.go:1038 + 存活竞争子测 + b1b5 7/7 |
| --auth-header 收窄为服务端审计归因（不进子进程环境） | ttyd -H per-connection spawn 模型在 GoTTY 共享进程模型下结构性不成立（PTY 启动时无 HTTP 请求在手，env 一次性快照写谁都错） | ✓ Phase 7 D-17/D-18 落地；README 模型差异段防误用预期 |
| opener 子进程 goroutine Wait + 非零退出 stderr 警告行 | fire-and-forget 使桌面异常不可观测且每次 --open 驻留一个僵尸；Wait err 仅 exit status N 结构性保证警告行不含 URL（token 红线） | ✓ Phase 7 G-07-8 闭合（选项 A）：main.go:1282 + b6 7/7 |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-08-28 after Phase 8（可观测性闭合：/healthz + /metrics + slog JSON 审计日志；UAT 3/3 闭合（A1 Prometheus 实机 scrape / A2 journald+jq 检索含 G-08-2 修复复测 / A3 draining 窗口观测）+ VERIFICATION 28/28 passed + SECURITY 26/26 closed + phase08.mjs 21 断言全绿）*
