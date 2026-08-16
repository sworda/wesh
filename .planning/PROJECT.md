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

### Active

**核心终端（对标 ttyd）**
- [ ] 窗口标题同步
- [ ] 前端基于 xterm.js 生态：WebGL 渲染、Unicode 11/CJK/IME、fit 自适应、超链接、剪贴板（WebGL 渲染回落 + fit 自适应已于 Phase 1 验证；CJK/IME/超链接/剪贴板 Phase 4）
- [ ] 断线自动重连接回同一进程（共享进程模型；历史现场恢复依赖 tmux/herdr）

**多客户端共享（改进 ttyd 限制 #2）**
- [ ] 原生多客户端 attach 同一会话，写入权限可配置（全员可写 / 主写旁观）

**安全（改进 ttyd 限制 #3 + 源码核实的新发现）**
- [ ] 认证：时序安全比较、凭据不明文进日志、一次性短时令牌替代 ttyd 的 /token 明文下发
- [ ] 认证失败节流防爆破
- [ ] Origin 允许列表校验
- [ ] TLS（禁旧协议、合理 cipher、安全响应头）
- [ ] URL 传参严格校验与上限（若保留该能力）

**资源控制（改进 ttyd 限制 #4/#5）**
- [ ] 背压控制与每客户端限速
- [ ] 最大连接数限制

**部署与集成**
- [ ] 端口/绑定地址/UNIX socket 监听配置
- [ ] 反代子路径挂载（base-path）
- [ ] 自定义首页
- [ ] 子进程 cwd/TERM/关闭信号配置
- [ ] 降权运行（uid/gid）
- [ ] 客户端偏好下发（-t key=value 等价机制）
- [ ] /healthz、metrics、结构化日志（ttyd 缺失的可运维性）
- [ ] 配置文件支持

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
*Last updated: 2026-08-15 after Phase 2*
