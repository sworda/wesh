# Roadmap: wesh

## Overview

从 PTY 核心管道出发（行走骨架），先把 WS 协议层一次性设计到位（类型化帧、三层上限、合规关闭码——事后补洞要动协议），再建立认证与 TLS 安全基线（多客户端权限需要身份概念先行）；随后补齐前端体验至 ttyd 基线对等，交付核心差异化能力——多客户端共享（fan-out、ro/rw 权限、背压、resize 仲裁），完善会话生命周期与断线重连，最后铺面部署配置与可观测性，以单静态二进制四平台发布收尾。v1 不做会话保持（用户以 tmux/herdr 覆盖），采用 GoTTY 共享进程模型：PTY 进程随服务端启动创建、多客户端共享，进程退出以类型化终结帧通知全部客户端；outbox/fan-out 结构为多客户端保留。

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: 行走骨架（核心 PTY 管道）** - PTY 双向转发 + resize + xterm.js 前端接通 + pidfd/kqueue 零线程收割
- [ ] **Phase 2: 协议基线** - wesh.v1 类型化帧、WS 三层上限、合规关闭码、默认只读、ping/pong 保活
- [ ] **Phase 3: 认证与传输安全** - 一次性 ticket、时序安全比较、失败节流、Origin 白名单、TLS 加固
- [ ] **Phase 4: 前端体验** - CJK/IME、超链接、现代剪贴板、标题同步、服务端偏好下发
- [ ] **Phase 5: 多客户端共享** - fan-out、ro/rw 权限、慢客户端背压踢出、resize 仲裁、ro/rw 分享链接
- [ ] **Phase 6: 会话生命周期与重连** - --once/无人退出/类型化终结帧、断线重连接回同一进程
- [ ] **Phase 7: 部署与配置** - 监听/base-path/配置文件/降权/子进程管理/auth-header 透传
- [ ] **Phase 8: 可观测性** - /healthz、/metrics、JSON 结构化审计日志
- [ ] **Phase 9: 发布与打磨** - 单静态二进制四平台发布、自定义首页、负载/模糊测试回填默认参数

## Phase Details

### Phase 1: 行走骨架（核心 PTY 管道）
**Goal**: 用户运行 `wesh -- <command>` 后在浏览器获得一个可用的完整交互终端
**Depends on**: Nothing (first phase)
**Requirements**: CORE-01, CORE-02, FE-01, FE-03, SEC-06
**Success Criteria** (what must be TRUE):
  1. 用户启动 `wesh -- bash`（或任意命令及参数）后，浏览器打开页面即获得完整交互终端，键盘输入与终端输出经 WS 双向实时转发
  2. 拖动浏览器窗口时前端 fit 自适应且服务端同步 TIOCSWINSZ，远端 vim/htop 等全屏应用随 resize 正确重绘
  3. 子进程退出后被正确收割（Linux pidfd / macOS kqueue，零额外线程、无僵尸残留）；Web shell 内 `env` 看不到服务端环境变量（白名单最小集）
**Plans**: TBD
**UI hint**: yes
**Research flag**: macOS kqueue EVFILT_PROC/NOTE_EXIT 退出监视需早期原型验证（MEDIUM-HIGH 置信，平台差异风险；失败兜底为 SIGCHLD + WNOHANG 循环 reap）——建议 `/gsd-plan-phase --research-phase 1`

含：Go module + pnpm/Vite 前端工程脚手架、go:embed 单 HTML 伺服、forkpty/setsid/exec 数组（绝不经 shell）、env 白名单在 spawn 路径一次到位、最小协议帧（OUTPUT/INPUT/RESIZE）、CI 强制 `go test -race`。spawn 失败不得关闭服务端自身 fd 0/1/2（ttyd pty.c:87,112 已核实缺陷）。

### Phase 2: 协议基线
**Goal**: WS 协议层一次性到位——版本化、类型化错误帧、三层资源上限、合规关闭码，预认证攻击面在结构上消除
**Depends on**: Phase 1
**Requirements**: CORE-04, CORE-06, SEC-08, RES-01
**Success Criteria** (what must be TRUE):
  1. 空帧、百万个 1 字节 continuation 帧、超限帧打过来时，服务端以 1009 合规关闭连接，不崩溃、内存平坦（三层上限：单帧/分片数/累积字节；认证通过前零缓冲分配）
  2. 默认只读模式下浏览器键盘输入被丢弃，显式开启可写后输入才生效；线上关闭码只出现在 1000/1008/1009/1011/1013 集合内（1006 永不发送）
  3. WS ping/pong 按可配间隔保活，反代空闲超时下连接不被切断
**Plans**: TBD
**Research flag**: WS 三层上限默认值需实测标定（C→S 单帧 16KiB 起步；累积字节与分片帧数硬顶——Bandit CVE 教训：只限字节不限帧数无效）

含：`proto/` 单一事实源（帧类型、版本、错误码、close code 常量）、版本化子协议 `wesh.v1`、Hello/Welcome/Error 握手帧、coder/websocket SetReadLimit、5s 未认证超时、per-IP 未认证连接上限、permessage-deflate 默认关。

### Phase 3: 认证与传输安全
**Goal**: 认证与 TLS 达到"敢暴露到公网"标准，修复 ttyd 已核实的认证连环错全套
**Depends on**: Phase 2
**Requirements**: SEC-01, SEC-02, SEC-03, SEC-04, SEC-05
**Success Criteria** (what must be TRUE):
  1. 已认证 HTTP `POST /api/attach` 换取一次性 ticket（单次使用、60s TTL、绑定权限级别），WS Hello 首帧核销；重放同一 ticket 被拒绝
  2. 脚本爆破 100 次错误凭据触发指数退避节流；凭据比较走 `crypto/subtle` 常数时间（先哈希等长）；凭据/ticket/Authorization 头任何形态不出现在任何日志（有日志脱敏测试）
  3. 不在 Origin 允许列表内的 WS 握手被拒绝；TLS 仅协商 1.2+（默认 1.3），响应含 HSTS/X-Content-Type-Options 等安全头，testssl.sh 无弱项
**Plans**: TBD

### Phase 4: 前端体验
**Goal**: 前端达到并超越 ttyd 功能基线（修掉其废弃 API 与停更依赖）
**Depends on**: Phase 2（TITLE/PREFS 协议帧已在协议基线定义）
**Requirements**: CORE-03, FE-02, FE-04, FE-05, FE-06, FE-07
**Success Criteria** (what must be TRUE):
  1. 中文/emoji 等宽字符正常输入显示（Unicode 11 + IME 组合输入不丢字），终端输出中的 URL 自动识别为可点击超链接（hover 显示真实地址）
  2. 选中即复制走 `navigator.clipboard` 现代 API（替代已废弃的 execCommand）；终端标题变化同步到浏览器标签页标题
  3. resize 时显示 COLSxROWS 浮层、离开页面前确认（均可开关）；服务端下发的 fontSize/theme 等偏好生效，URL query 可覆盖
**Plans**: TBD
**UI hint**: yes

注：OSC52 剪贴板 addon 默认关闭，开启时只写不读（PITFALLS C5，Warp CVE-2025-48725 教训）。

### Phase 5: 多客户端共享
**Goal**: 多个客户端可同时 attach 同一 PTY 会话，权限可配、慢客户端不拖累他人——核心差异化能力
**Depends on**: Phase 3（ro/rw 权限是对"同一 session 多个连接"的属性，身份概念先行）
**Requirements**: MULTI-01, MULTI-02, MULTI-03, MULTI-04, MULTI-05, RES-02, RES-03, RES-04
**Success Criteria** (what must be TRUE):
  1. 两个浏览器 attach 同一会话输出实时一致；`all` 模式全员可写（协作排障），`owner` 模式仅 owner 可写、ro 链接旁观者输入被丢弃（演示旁观）
  2. 一个客户端停止读取 TCP 流时其他客户端无卡顿：慢客户端 outbox 写满被 1013 踢出，重连后从最新输出看起；PTY 读循环永不因任何客户端阻塞
  3. 异尺寸两客户端按最小公共矩形 `min(cols)×min(rows)` 渲染，2→1 时恢复 last-wins；启动时打印含一次性 token 的 ro/rw 两条分享链接，即打即用
**Plans**: TBD
**UI hint**: yes
**Research flag**: outbox 容量/水位/strikes 默认参数需负载测试标定（可在执行中以测试任务消化，Phase 9 回填）。**resize 仲裁分歧已闭合**：以需求 MULTI-04 为准——所有模式下 ≥2 客户端一律最小公共矩形；ARCHITECTURE.md §2.9 "owner 模式跟随 owner 尺寸"表述作废。

含：每客户端有界 outbox + 专属 writer（Actor 只做 try_send）、全体可写客户端阻塞时停读 PTY 的全局信用、resize 防抖（50ms 合并窗口）与尺寸上限钳制（1000×1000）、每客户端输入速率限制（x/time rate）、最大并发客户端数满员拒绝、ticket 按模式分别签发 ro/rw。

### Phase 6: 会话生命周期与重连
**Goal**: 会话生命周期模式完整，断线重连闭环——共享进程模型下重连即接回原 PTY 进程
**Depends on**: Phase 5（"所有客户端断开"语义与终结帧全员通知在多客户端语境下完整）
**Requirements**: SESS-01, SESS-02, SESS-03, CORE-05
**Success Criteria** (what must be TRUE):
  1. `wesh --once` 只接受一个客户端，其断开后服务端退出；配置"所有客户端断开后退出"时，最后一个客户端断开即触发退出
  2. 子进程退出后所有在线客户端收到含退出码的类型化终结帧提示（非静默断开），随后以 1000 正常关闭
  3. 断网 30s 恢复后前端自动重连（指数退避 + 上限 + 手动入口）并接回同一 PTY 进程，输入输出一致（无滚动回放，屏幕靠程序重绘或 tmux/herdr 恢复——文档明示）
**Plans**: TBD
**UI hint**: yes

### Phase 7: 部署与配置
**Goal**: 真实运维场景可部署——监听形态齐全、配置文件落地、反代友好
**Depends on**: Phase 3（auth-header 透传依赖认证体系）
**Requirements**: OPS-01, OPS-02, OPS-04, OPS-05, OPS-09, OPS-11, SEC-07
**Success Criteria** (what must be TRUE):
  1. 端口（0=随机并打印实际端口）/绑定地址/UNIX socket（含属主）可配置；TOML 配置文件支持，CLI 参数覆盖配置文件
  2. 反代子路径挂载（`/wesh/` base-path）下页面与 WS 升级均正常（尾斜杠规范化）；反代注入的可信用户头作为环境变量出现在子进程中
  3. 子进程以指定 cwd/TERM 启动，停止信号发给进程组（可配 TERM→KILL 宽限）；可以指定 uid/gid 降权运行；可选启动后自动打开浏览器
**Plans**: TBD

### Phase 8: 可观测性
**Goal**: ttyd 缺失的可运维性补齐——健康检查、指标、审计日志
**Depends on**: Phase 5（metrics 含多客户端指标：每客户端 outbox 深度、1013 踢出数）
**Requirements**: OPS-06, OPS-07, OPS-08
**Success Criteria** (what must be TRUE):
  1. `/healthz` 返回服务健康状态，可用于反代/编排探活
  2. `/metrics` 暴露连接数、会话数、收发字节数、每客户端 outbox 深度与踢出计数
  3. 日志为 JSON 结构化输出（slog），认证失败、连接建立/断开、会话生命周期等审计事件可检索；日志中无凭据（回归 P3 红线），用户可控字段已剥离控制字符
**Plans**: TBD

### Phase 9: 发布与打磨
**Goal**: 单静态二进制四平台发布，默认参数经负载测试标定，部署文档齐全
**Depends on**: Phase 8
**Requirements**: OPS-03, OPS-10
**Success Criteria** (what must be TRUE):
  1. goreleaser 产出 linux/darwin × amd64/arm64 四个全静态二进制（CGO_ENABLED=0），前端单 HTML 经 embed 内嵌，scp 到干净机器即可运行
  2. 自定义首页 HTML 可配置生效；负载/模糊测试通过（高吞吐 fan-out、慢客户端矩阵、百万小帧/空帧、高频建销会话无 defunct），测试数据回填 P2/P5 默认参数
  3. 部署文档覆盖 nginx/Cloudflare/Caddy 反代配方（含空闲超时与 ping 间隔关系）、Docker（tini/PID 1 收割）、systemd unit 模板（Restart/LimitNOFILE/EnvironmentFile 600）
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. 行走骨架（核心 PTY 管道） | TBD | Not started | - |
| 2. 协议基线 | TBD | Not started | - |
| 3. 认证与传输安全 | TBD | Not started | - |
| 4. 前端体验 | TBD | Not started | - |
| 5. 多客户端共享 | TBD | Not started | - |
| 6. 会话生命周期与重连 | TBD | Not started | - |
| 7. 部署与配置 | TBD | Not started | - |
| 8. 可观测性 | TBD | Not started | - |
| 9. 发布与打磨 | TBD | Not started | - |
