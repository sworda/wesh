# Roadmap: wesh

## Overview

从 PTY 核心管道出发（行走骨架），先把 WS 协议层一次性设计到位（类型化帧、三层上限、合规关闭码——事后补洞要动协议），再建立认证与 TLS 安全基线（多客户端权限需要身份概念先行）；随后补齐前端体验至 ttyd 基线对等，交付核心差异化能力——多客户端共享（fan-out、ro/rw 权限、背压、resize 仲裁），完善会话生命周期与断线重连，最后铺面部署配置与可观测性，以单静态二进制四平台发布收尾。v1 不做会话保持（用户以 tmux/herdr 覆盖），采用 GoTTY 共享进程模型：PTY 进程随服务端启动创建、多客户端共享，进程退出以类型化终结帧通知全部客户端；outbox/fan-out 结构为多客户端保留。

**v1.1（per-client 会话模式）**：在 shared 模型默认零回归前提下新增 ttyd 式 per-connection spawn 第二路径——每 WS 客户端独立 PTY 子进程，使 herdr 等自带多客户端仲裁（is_foreground + per-client area 渲染）的应用恢复正确行为。骨架接缝先行（模式阀门一次装配、全部 inert），随后打通 attach spawn / 断开即杀进程组 / EXIT 私有化的生命周期主链，补齐 resize 直通、ro 门控、重连 reset、慢客户端停读续读与 1013 踢出的交互语义；再筑资源防线（maxClients 兼任进程硬顶、spawn 双令牌桶、KILL 兜底、关停覆盖 N 进程组）与终结语义（--once/exit-when-empty 第二终结源、metrics/审计 per-client 粒度、WESH_REMOTE_USER 注入）；最后以双模式 -race 门、协议层 UAT 与 Windows Playwright herdr 全链收口，负载矩阵实测回填标定与双模式文档。架构形态：装配期一次分岔、运行期零分岔，不抽象 session 接口，两模式共享面 ≥90%。

## Milestones

- ✅ **v1.0** — Phases 1-9（shipped 2026-08-31，v1.0.0 四平台发布上架，44/44 需求收口）
- 🚧 **v1.1 per-client 会话模式** — Phases 10-14（roadmap created 2026-09-02；2026-09-03 原 13/14 合并、原 15 重编号 14）

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: 行走骨架（核心 PTY 管道）** - PTY 双向转发 + resize + xterm.js 前端接通 + pidfd/kqueue 零线程收割 (completed 2026-08-14)
- [x] **Phase 2: 协议基线** - wesh.v1 类型化帧、WS 三层上限、合规关闭码、默认只读、ping/pong 保活 (completed 2026-08-15)
- [x] **Phase 3: 认证与传输安全** - 一次性 ticket、时序安全比较、失败节流、Origin 白名单、TLS 加固 (completed 2026-08-18)
- [x] **Phase 4: 前端体验** - CJK/IME、超链接、现代剪贴板、标题同步、服务端偏好下发 (completed 2026-08-19)
- [x] **Phase 5: 多客户端共享** - fan-out、ro/rw 权限、慢客户端背压踢出、resize 仲裁、ro/rw 分享链接 (completed 2026-08-22)
- [x] **Phase 6: 会话生命周期与重连** - --once/无人退出/类型化终结帧、断线重连接回同一进程 (completed 2026-08-24)
- [x] **Phase 7: 部署与配置** - 监听/base-path/配置文件/降权/子进程管理/auth-header 透传 (completed 2026-08-27)
- [x] **Phase 8: 可观测性** - /healthz、/metrics、JSON 结构化审计日志 (completed 2026-08-28)
- [x] **Phase 9: 发布与打磨** - 单静态二进制四平台发布、自定义首页、负载/模糊测试回填默认参数 (completed 2026-08-31)
- [x] **Phase 10: 模式装配与接缝** - --session-mode flag + TOML 键 + Options/StartWithSize 接缝，全部 inert 零回归 (completed 2026-09-03)
- [ ] **Phase 11: per-client 生命周期主干** - attach spawn / 断开即杀进程组 / EXIT 私有化 / teardown 恰好一次
- [ ] **Phase 12: per-client 交互与背压语义** - resize 直通 / ro 门控 / 重连 reset / 停读续读 / 1013 踢出
- [ ] **Phase 13: 资源防线与终结语义** - maxClients 进程硬顶 / spawn 双令牌桶 / KILL 兜底 / 关停 N 进程组 / 第二终结源 / 退出码对齐 / metrics 审计 per-client 粒度 / WESH_REMOTE_USER
- [ ] **Phase 14: 双模式验证矩阵、标定与 herdr UAT** - 双模式 -race 门 / 协议层 + Playwright UAT / 负载矩阵回填 / 模式文档

## Phase Details

<details>
<summary>✅ v1.0（Phases 1-9）— SHIPPED 2026-08-31（44/44 需求收口，v1.0.0 已发布）</summary>

### Phase 1: 行走骨架（核心 PTY 管道）

**Goal**: 用户运行 `wesh -- <command>` 后在浏览器获得一个可用的完整交互终端
**Depends on**: Nothing (first phase)
**Requirements**: CORE-01, CORE-02, FE-01, FE-03, SEC-06
**Success Criteria** (what must be TRUE):

  1. 用户启动 `wesh -- bash`（或任意命令及参数）后，浏览器打开页面即获得完整交互终端，键盘输入与终端输出经 WS 双向实时转发
  2. 拖动浏览器窗口时前端 fit 自适应且服务端同步 TIOCSWINSZ，远端 vim/htop 等全屏应用随 resize 正确重绘
  3. 子进程退出后被正确收割（Linux pidfd / macOS kqueue，零额外线程、无僵尸残留）；Web shell 内 `env` 看不到服务端环境变量（白名单最小集）

**Plans**: 5/5 plans executed
**Wave 1**

- [x] 01-01-PLAN.md — 行走骨架 tracer：仓库重命名 + Go module + CLI/proto/embed 契约 + 端到端 PTY 管道（spawn/io/reap/server/生命周期）+ TestEchoPTY + 前端 UI-SPEC 全量接入

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-02-PLAN.md — pty 引擎测试加固：exec 数组/env 白名单/spawn 失败不伤 fd（spawn_test）+ resize TIOCSWINSZ/收割无僵尸（io_test/reap_test）
- [x] 01-03-PLAN.md — server/main 生命周期测试：第二连接 409、退出码传递、未知帧 1002、断开 SIGHUP 进程组 + CLI parseArgs/无命令报错
- [x] 01-04-PLAN.md — darwin 共享 kqueue watcher + Q1 竞态裁决双测试（CI-only）+ 双平台 CI（go 矩阵 ubuntu/macos + web 构建）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-05-PLAN.md — README（无认证警示 + 单次语义）+ 全量收口验证（-race 全量/前端构建/裸 clone embed 链/启动冒烟）+ 浏览器手动 checklist

**UI hint**: yes
**Research flag**: macOS kqueue EVFILT_PROC/NOTE_EXIT 退出监视需早期原型验证（MEDIUM-HIGH 置信，平台差异风险；失败兜底为 SIGCHLD + WNOHANG 循环 reap）——建议 `/gsd-plan-phase --research-phase 1`

含：Go module + pnpm/Vite 前端工程脚手架、go:embed 单 HTML 伺服、forkpty/setsid/exec 数组（绝不经 shell）、env 白名单在 spawn 路径一次到位、最小协议帧（OUTPUT/INPUT/RESIZE）、CI 强制 `go test -race`。spawn 失败不得关闭服务端自身 fd 0/1/2（ttyd pty.c:87,112 已核实缺陷）。

### Phase 2: 协议基线

**Goal**: WS 协议层一次性到位——版本化、类型化错误帧、三层资源上限、合规关闭码，预认证攻击面在结构上消除
**Depends on**: Phase 1
**Requirements**: CORE-04, CORE-06, SEC-08, RES-01
**Success Criteria** (what must be TRUE):

  1. 百万个 1 字节 continuation 帧、超限帧打过来时，服务端以 1009 合规关闭连接；0 字节空帧洪水下服务存活、不崩溃、内存平坦（两层硬顶：单帧 16KiB / 累积字节 16KiB，SetReadLimit 库执行；分片数层库不暴露、经等效防线覆盖——见 02-CONTEXT.md D-09 修订；认证通过前零缓冲分配）
  2. 默认只读模式下浏览器键盘输入被丢弃，显式开启可写后输入才生效；线上关闭码只出现在 1000/1008/1009/1011/1013 集合内（1006 永不发送）
  3. WS ping/pong 按可配间隔保活，反代空闲超时下连接不被切断

**Plans**: 6/6 plans executed
**Wave 1**

- [x] 02-01-PLAN.md — proto 契约：'H'/'W'/'E' 类型字节 + Subprotocol 常量 + Error code 表 + 关闭码注释表（1001/1013 占位）+ 两档读上限常量（D-09 修订分片层注释位）+ Hello/Welcome/Error 编解码 + proto 单测（D-01/D-02/D-05/D-06/D-07/D-08/D-10）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-02-PLAN.md — 握手与只读基线 tracer：服务端握手段（子协议双闸/4KiB/5s 超时/抢跑与空消息 1002/version_mismatch Error+1008/Welcome/升档 16KiB）+ ro INPUT 门 + --writable + 前端握手/onclose 按码分派 + e2e 全量握手改造（dialHello）+ TestHelloWelcome（CORE-04/SEC-08/RES-01）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-03-PLAN.md — SEC-08 预认证守卫链：per-IP 半开帽 8（HTTP 429，位于 409 门之前——D-04 可触达性裁决）+ http.Server ReadHeaderTimeout 5s + handshake_test.go 七测（子协议 400×2+多值/429/hello_timeout/抢跑/version_mismatch/ro 丢 INPUT/ro 放行 RESIZE）（SEC-08/CORE-04）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 02-04-PLAN.md — CORE-06 保活：pinger goroutine（Welcome 升档后启动，pong 超时 10s CloseNow）+ --ping-interval（默认 5s，0 禁用）+ keepalive_test.go 三测（存活/pong 超时/禁用反证）（CORE-06）

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 02-05-PLAN.md — RES-01 攻击面与超限可见性：ErrMessageTooBig → stderr 单行事件钩子（D-12②）+ limits_test.go 五测（17KiB 1009/16384·16385 边界/1 字节分片洪水 1009/0 字节空消息洪水存活/预认证 4KiB 档 1009）（RES-01/SEC-08）

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 02-06-PLAN.md — 收口：README 同步 wesh.v1 协议语义与新 flag（无认证警示保持）+ 全量验证六段式（GOROOT gofmt/vet/-race/web 构建/裸 clone/冒烟）+ 浏览器人工 UAT 清单

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

**Plans**: 7 plans（6 executed + 03-07 gap closure）
**Wave 1**

- [x] 03-01-PLAN.md — 协议契约增量（ErrAuthFailed/HelloPayload.Ticket）+ ticketStore（单次使用/60s TTL/mode 绑定）+ throttleStore（1s×2 封顶 30s/成功清零/惰性过期）纯组件（SEC-02/SEC-03）
- [x] 03-02-PLAN.md — 凭据预哈希 subtle 比较（含 RESEARCH `&=` erratum 修正为 `|=`）+ Origin 规范化/检查 + 安全头中间件 + TLSConfig（MinVersion 1.2 + 6 AEAD）纯组件（SEC-01/SEC-04/SEC-05）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03-03-PLAN.md — server.go 集成 tracer：/api/attach 守卫链（405/403/429/401/签发）+ 整站 Basic + 守卫区 ⓪ Origin + Hello 核销 auth_failed 统一口径 + 集成测试组（含日志红线运行时捕获）（SEC-01..SEC-04）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 03-04-PLAN.md — main.go：6 新 flag（--credential/--tls-cert/--tls-key/--no-auth/--insecure-http/--origin）+ WESH_CREDENTIAL 兜底 + 启动校验矩阵（D-03/D-05 拒绝路径）+ ServeTLS 分岔（SEC-01/SEC-04/SEC-05）
- [x] 03-05-PLAN.md — 前端 connect() 改造：fetch ticket → Hello{ticket} → auth_failed 静默重试一次 + wss scheme + dist 重建提交（SEC-02）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 03-06-PLAN.md — 收口：phase03.mjs 六场景 UAT + phase02.mjs 的 D-03 适配 + README 认证/TLS 与行为变更明示 + 03-UAT.md 人工清单 + 全量验证六段式（SEC-01..SEC-05）

**Gap closure** *(UAT G-03-5，wave 1 独立可执行)*

- [x] 03-07-PLAN.md — G-03-5 闭合：TLS 证书启动预检（print-then-die 修复）+ serve 失败 sess.Close() 回滚（pty 孤儿修复）+ 文档复现命令 --writable 清扫（SEC-05/SEC-01）

### Phase 4: 前端体验

**Goal**: 前端达到并超越 ttyd 功能基线（修掉其废弃 API 与停更依赖）
**Depends on**: Phase 2（TITLE/PREFS 协议帧已在协议基线定义）
**Requirements**: CORE-03, FE-02, FE-04, FE-05, FE-06, FE-07
**Success Criteria** (what must be TRUE):

  1. 中文/emoji 等宽字符正常输入显示（Unicode 11 + IME 组合输入不丢字），终端输出中的 URL 自动识别为可点击超链接（hover 显示真实地址）
  2. 选中即复制走 `navigator.clipboard` 现代 API（替代已废弃的 execCommand）；终端标题变化同步到浏览器标签页标题
  3. resize 时显示 COLSxROWS 浮层、离开页面前确认（均可开关）；服务端下发的 fontSize/theme 等偏好生效，URL query 可覆盖

**Plans**: 6/6 plans executed
**Wave 1**

- [x] 04-01-PLAN.md — FE-07 偏好下发 Go 通道 tracer：WelcomePayload.Prefs（omitempty）+ ValidClientOptionKey 白名单 + server 注入 + --client-option/--osc52 + 聚合 + 握手 e2e（FE-07）
- [x] 04-02-PLAN.md — 前端 addon 接入：三 addon 钉版 + unicode11 激活 + web-links/OSC8 双通道 + hover tooltip + 标题同步单一写口（CORE-03/FE-02/FE-04）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 04-03-PLAN.md — 剪贴板（选中即复制防抖 + Ctrl+Shift+V 粘贴 + 安全上下文门）+ resize 浮层 + beforeunload 三开关量埋点（FE-05/FE-06）
- [x] 04-04-PLAN.md — phase04.mjs 协议 UAT：Welcome prefs 六正场景 + client-option 启动拒绝四负场景 + 前序 UAT 回归（FE-07）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 04-05-PLAN.md — 前端 prefs 通道：query 覆盖 + WELCOME 应用 + theme 合并 + behavior 开关接线 + OSC52 write-only 加载（FE-06/FE-07）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 04-06-PLAN.md — 收口：README 前端体验节 + 04-UAT.md 人工清单 + 全量六段式验证 + 三套 UAT 回归（CORE-03/FE-02/FE-04/FE-05/FE-06/FE-07）

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

**Plans**: 13/13 plans executed（12 executed + 1 gap closure — REVIEW WR-01/WR-02）
**Wave 1**

- [x] 05-01-PLAN.md — tracer：多客户端 fan-out 主干（clients.go 注册表/hub/outbox/writer + 409 门拆除 + 生命周期改造断开不退出/子进程退出广播 1000）+ e2e 单次语义迁移 + TestMultiClientFanout/TestDetach/TestExitBroadcast（MULTI-01）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 05-02-PLAN.md — 全局信用门（全体可写端满停读 PTY/半水位恢复/统一 Broadcast）+ D-11 SIGWINCH 新客重绘 + TestSlowConsumerKick/TestGlobalCredit（MULTI-03/RES-04）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 05-03-PLAN.md — 写权限体系：--write-policy=owner|all + owner FIFO 递补 + 降级/升格 Welcome + prefs 双档 osc52（含 D-05 one-way 确认门）+ 权限测试组 + TestSuccessionKickRace 继承竞态时序闭合（MULTI-02）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 05-04-PLAN.md — resize 仲裁器：arbitrate 纯函数 + D-09 参与集分层 + 50ms 防抖 + ro 忽略闸 + TestArbitrate/TestResizeArbitration（MULTI-04）

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 05-05-PLAN.md — RES-02 输入限速（x/time/rate 超限丢弃）+ CR-01 完整背压（256KiB 输入队列 + input-writer 独占 Master.Write）+ TestInputRateLimit（RES-02）

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 05-06-PLAN.md — 分享链接：shareTokens 两条目 store + /s/{token}/ 门禁 + attach token 分支 + 启动打印两行 + outboundIPv4（含 D-01/D-03 one-way 确认门）+ TestShareToken（MULTI-05）

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 05-07-PLAN.md — --max-clients（默认 32）+ ③位 503 闸 + /api/attach 早闸（含 D-08 one-way 确认门）+ TestMaxClients503 + TestClientCountInvariant 计数对称不变量（RES-03）

**Wave 8** *(blocked on Wave 7 completion)*

- [x] 05-08-PLAN.md — 前端：/s/ token 进入 + 响应分派矩阵 + 1013/503/无效链接三专版 + 文案清扫 R1-R3 + 升格 rw 分支 + ro 不发 RESIZE + OSC52 门闩 + dist 重建（MULTI-02/03/04/05）

**Wave 9** *(blocked on Wave 8 completion)*

- [x] 05-09-PLAN.md — 收口：phase05.mjs 协议 UAT（链接全链/双客户端一致/满员 503/S6 1013 踢出活跃场景）+ 05-UAT.md 人工清单 + phase02/03.mjs 生命周期适配 + README 多客户端节（含反代脱敏示例/暴露面清单/标定方法论）+ 全量六段式（MULTI-01/03/05/RES-03）

**Gap closure** *(UAT G-05-1：异尺寸双端行编辑叠写——D-09 min-rect 不变量不覆盖相对寻址流；用户裁决方向 A = 会话尺寸下发 + 前端视口约束，2026-08-22)*

- [x] 05-10-PLAN.md — 服务端：Welcome 恒携会话 cols/rows + recalcNow 推送挂点（运行期 'W' 再推送复用升格先例）+ attach 升档时序重排 + 升格携新 owner 尺寸 + Go 行为测试组（MULTI-01/MULTI-04）
- [x] 05-11-PLAN.md — 前端：sessionDims/refit 统一入口（上报=fit / 渲染=逐轴 min 拆分）+ WELCOME 尺寸应用与升格解除 + ro 提示门闩 + dist 重建（MULTI-01/MULTI-04）
- [x] 05-12-PLAN.md — UAT 三层断言（S10 协议 / D6 DOM 约束渲染 / phase05-dims.mjs headless 等价+负对照）+ README/05-UAT 同步 + 全量六段式回归（MULTI-01/MULTI-04）

**Gap closure** *(VERIFICATION 2026-08-22 复验 gaps_found：G-05-1 缝合面残留 WR-01 推送循环内踢出致 stale 扇出 + WR-02 creditBlocked 端尺寸推送丢失——05-REVIEW 逐字补丁，不可 defer)*

- [x] 05-13-PLAN.md — pushSessionDimsLocked 嵌套重算 arbiter.last 复检（stale 扇出中止）+ 注释论证改写 + afterDrain 开门补发当前会话尺寸 Welcome（option (a)）+ TestPushSessionDimsKickRecalc/TestAfterDrainResendsDims 两白盒回归（MULTI-04/RES-04）

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

**Plans**: 7/7 plans executed
**Wave 1**

- [x] 06-01-PLAN.md — SESS-03 EXIT 帧端到端 tracer：proto 'X'/ExitPayload/ExitFrame + lifecycle 写序安全广播（同步 Write→Close 1000）+ exit_test.go 两测 + exitmsg_test.go 白盒测（signalName/exitMessage）+ 前端暂存承接与 dist（含 D-08/D-09 one-way 确认门）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 06-02-PLAN.md — SESS-01/02 服务端：pty.SignalHangup（SIGHUP 进程组复活）+ Options.ExitWhenEmpty（set/grace 分离）+ 注册表空触发与宽限计时器 + exiting 门 + 七测（含 OQ1 退出状态确认门）
- [x] 06-03-PLAN.md — CORE-05 前端重连状态机：backoffMs 纯函数 + 1006 显式触发 + online/offline 双触发 + Reconnecting 面板（showStatus 参数化）+ 代际守卫 + dist

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 06-04-PLAN.md — SESS-01/02 CLI：--once 语法糖 + --exit-when-empty[=duration]（IsBoolFlag）+ 冲突校验矩阵 + Options 接线（含 D-12/D-14 one-way 确认门）
- [x] 06-05-PLAN.md — phase06-dom.mjs：jsdom 重连状态机八场景（1006 全链/1002·1013·1008 不触发/双触发幂等/手动入口/代际守卫/EXIT 全链/online 快路径）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 06-06-PLAN.md — phase06.mjs 协议层 UAT：EXIT 双端广播（ro/rw 同帧 + 帧序 + 进程退出码）/信号死亡/--once 全链/--exit-when-empty 立即与宽限/断连重接同一 PTY

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 06-07-PLAN.md — 收口：README 生命周期与重连节 + 06-UAT.md 人工清单 + VALIDATION 同步 + 全量六段式与九 UAT 脚本回归

**UI hint**: yes

### Phase 7: 部署与配置

**Goal**: 真实运维场景可部署——监听形态齐全、配置文件落地、反代友好
**Depends on**: Phase 3（auth-header 透传依赖认证体系）
**Requirements**: OPS-01, OPS-02, OPS-04, OPS-05, OPS-09, OPS-11, SEC-07
**Success Criteria** (what must be TRUE):

  1. 端口（0=随机并打印实际端口）/绑定地址/UNIX socket（含属主）可配置；TOML 配置文件支持，CLI 参数覆盖配置文件
  2. 反代子路径挂载（`/wesh/` base-path）下页面与 WS 升级均正常（尾斜杠规范化）；反代注入的可信用户头记录进服务端审计日志（remote_user 审计归因——D-15 修订：原「作为环境变量出现在子进程中」语义在 GoTTY 共享进程模型下结构性不成立）
  3. 子进程以指定 cwd/TERM 启动，停止信号发给进程组（可配 TERM→KILL 宽限）；可以指定 uid/gid 降权运行；可选启动后自动打开浏览器

**Plans**: 9/10 plans executed（8/8 executed + 07-09/07-10 gap closure）
**Wave 1**

- [x] 07-01-PLAN.md — base-path tracer（OPS-02，D-13/D-14）：--base-path 严格校验 + mux 前缀装配（StripPrefix 仅静态伺服 + 307 免费）+ 前端相对 URL 三改含 share 升级前缀 + dist（OPS-02）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 07-02-PLAN.md — UNIX socket（OPS-01，D-08..D-12）：--socket/--socket-mode/--socket-owner + listen 前 Remove + listen 后 Chmod/Chown + validateStartup 互斥与跳过 + unix:// 打印与分享链接退化（OPS-01）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 07-03-PLAN.md — auth-header/XFF（SEC-07，D-15..D-20）：proxy.go sanitize/extract + logEvent remote_user 第四字段 + XFF 信任闸换 logEvent/throttle 键 + D-16 暴露面警告（SEC-07）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 07-04-PLAN.md — 子进程管理+降权（OPS-04/05，D-21/D-22/D-24/D-25）：StartOptions Dir/Term + SignalGroup + stop-signal/stop-timeout 序列 + uid/gid Credential + whitelistEnv 身份改写（OPS-04/OPS-05）

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 07-05-PLAN.md — 1001 优雅下线 + --open（D-23 + OPS-11，D-26/D-27）：Server.Shutdown 1001 广播 + SIGTERM/INT 捕获 + proto 1001 启用 + 前端关停面板 + --open headless 跳过（OPS-11）

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 07-06-PLAN.md — TOML 配置文件（OPS-09，D-01..D-07）：go-toml 严格模式 + 27 键（26 flag 同名 + command）两阶段合并 + flag>env>config>default 优先级 + D-07 权限警告 + 值剥离红线（OPS-09）

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 07-07-PLAN.md — phase07.mjs 协议层 UAT：配置合并/unix socket relay/base-path 交叉/auth-header/XFF/stop-signal/降权/1001/--open 八场景 + 自净红线（全需求）

**Wave 8** *(blocked on Wave 7 completion)*

- [x] 07-08-PLAN.md — 收口：README 部署与配置节（含 ttyd -H 模型差异）+ SEC-07 需求文本 D-15 修订 + 07-UAT.md 人工清单 + 全量六段式与十脚本回归（全需求）

**Gap closure** *(UAT 2026-08-26 三 issue：G-07-2 反代配方缺 Host 转发跨机 WS 403 / G-07-3 存活 socket 被静默接管 / G-07-8 opener 非零退出静默——A1/B4 为 blocked 环境前置（平台拓扑/无 root 通道），非代码问题不出 plan)*

- [x] 07-09-PLAN.md — G-07-2 闭合：README nginx 配方补 Host $http_host + 精确块理据按 proxy_pass 301 实证改写 + pw 回归载具同步双机全链 5/5（OPS-02）
- [x] 07-10-PLAN.md — G-07-3/G-07-8 闭合：listenSocket 活性探测（存活拒绝 EADDRINUSE exit 1）+ openBrowser goroutine Wait 非零警告（选项 A）+ b1b5/b6 二进制直证与协议套件回归（OPS-01/OPS-11）

### Phase 8: 可观测性

**Goal**: ttyd 缺失的可运维性补齐——健康检查、指标、审计日志
**Depends on**: Phase 5（metrics 含多客户端指标：每客户端 outbox 深度、1013 踢出数）
**Requirements**: OPS-06, OPS-07, OPS-08
**Success Criteria** (what must be TRUE):

  1. `/healthz` 返回服务健康状态，可用于反代/编排探活
  2. `/metrics` 暴露连接数、会话数、收发字节数、每客户端 outbox 深度与踢出计数
  3. 日志为 JSON 结构化输出（slog），认证失败、连接建立/断开、会话生命周期等审计事件可检索；日志中无凭据（回归 P3 红线），用户可控字段已剥离控制字符

**Plans**: 6/6 plans executed（5/5 executed + 1 gap-closure pending）
**Wave 1**

- [x] 08-01-PLAN.md — slog 原子迁移：logEvent 迁入 log.go 换 slog JSONHandler + 动态 stderr writer + parseEvents helper + 5 Go 测试与 phase05/07 两 UAT 脚本断言迁移 JSON 行解析（OPS-08，D-13/D-14/D-15/D-16/D-18）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 08-02-PLAN.md — 审计事件目录：attach/detach（client_id + reason 四值）/session_start/session_end/shutdown + throttled retry_after + remote 字段 sanitize 推广（OPS-08，D-17/D-19/D-20/D-21/D-22/D-23）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 08-03-PLAN.md — /healthz：免认证 200 JSON 四字段 + 根路径固定 + 405 成对 + sessionAlive/draining 两 atomic.Bool（OPS-06，D-07/D-09/D-10/D-11/D-12）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 08-04-PLAN.md — /metrics：手写 text 0.0.4 exposition 17 series + hubMu 快照 + 认证闸跟随 + Options.Version + 计数器挂点兑现（OPS-07，D-01..D-06/D-08/D-09/D-12）

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 08-05-PLAN.md — 收口：phase08.mjs 六场景 UAT + README 运维节（免认证例外/Prometheus 配方/jq 检索）+ 08-UAT.md + 全量六段式回归（OPS-06/07/08）

**Wave 6** *(gap closure — G-08-2，08-UAT test 2)*

- [x] 08-06-PLAN.md — G-08-2 闭合：README journald 示例补 grep 防护 + 合流机理说明（wesh 代码零改动，D-14/D-15/D-16 保持）+ phase08-journal.mjs 合流模拟回归（负对照自证）（OPS-08）

### Phase 9: 发布与打磨

**Goal**: 单静态二进制四平台发布，默认参数经负载测试标定，部署文档齐全
**Depends on**: Phase 8
**Requirements**: OPS-03, OPS-10
**Success Criteria** (what must be TRUE):

  1. goreleaser 产出 linux/darwin × amd64/arm64 四个全静态二进制（CGO_ENABLED=0），前端单 HTML 经 embed 内嵌，scp 到干净机器即可运行
  2. 自定义首页 HTML 可配置生效；负载/模糊测试通过（高吞吐 fan-out、慢客户端矩阵、百万小帧/空帧、高频建销会话无 defunct），测试数据回填 P2/P5 默认参数
  3. 部署文档覆盖 nginx/Cloudflare/Caddy 反代配方（含空闲超时与 ping 间隔关系）、Docker（tini/PID 1 收割）、systemd unit 模板（Restart/LimitNOFILE/EnvironmentFile 600）

**Plans**: 10/10 plans executed
**Wave 1**

- [x] 09-01-PLAN.md — 发布链 tracer：.goreleaser.yml（D-01..D-04 定稿）+ release.yml 显式编排 + 本机 snapshot 预演与四平台产物分层断言（OPS-10）
- [x] 09-02-PLAN.md — fuzz 两目标：decodeFileConfig reader 接缝 + FuzzDecodeFileConfig/FuzzDecodeHello/FuzzDecodeResize + ci.yml fuzz leg（D-09/D-10）
- [x] 09-06-PLAN.md — 负载矩阵：load_test.go（//go:build load）三断言（零误踢/内存上界/门频率）+ defunct 三面 + 标定数据落表（D-11/D-12）
- [x] 09-07-PLAN.md — Dockerfile（scratch+tini sha256 钉死）+ deploy/wesh.service + 本机 docker/实机 systemctl 双实测（D-16/D-17）
- [x] 09-08-PLAN.md — Caddy 配方实证：Linux 协议层（Host 透传/WS 全链/idle 存活）+ 双机载具与 Windows 确认门（D-15）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 09-03-PLAN.md — D-18 ship 清零：HINT_SHUTDOWN 条件化 + #status role="alert" + pre-onopen 1001 分派 + jsdom 扩展 + dist 重建（OPS-10）
- [x] 09-04-PLAN.md — OPS-03 --index：one-way 确认门（D-05/D-06/D-07）+ CLI/config 两键/启动读入/四拒绝 + WithCustomIndex 装饰 + TestCustomIndex（OPS-03）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 09-05-PLAN.md — OPS-03 协议层 UAT：phase09.mjs（校验矩阵/双通道/gzip/Vary/认证面/base-path）+ 既有脚本回归（OPS-03）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 09-09-PLAN.md — scripts/release.sh（D-14）+ README 全量（发布节/--index 节/Caddy+CF+Docker+systemd/标定表 D-13 回填）

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 09-10-PLAN.md — 收口：全量六段式 + 全量 UAT + fuzz/load/snapshot 复演 + 发布闸（v1.0.0 裁决）（OPS-03/OPS-10）

**UI hint**: yes

</details>

**Milestone v1.1 Goal:** wesh 支持 ttyd 式 per-connection spawn——每个 WebSocket 客户端独立 PTY 子进程，使 herdr 等自带多客户端仲裁（is_foreground + per-client area 渲染）的应用在 wesh 下恢复正确行为；shared 共享模式保持默认、零回归。架构形态锁定「装配期一次分岔，运行期零分岔」：不抽象 session 接口（6-7 个显式分支点），两模式共享面 ≥90%。

### Phase 10: 模式装配与接缝

**Goal**: 会话模式阀门与全部接缝一次装配到位（全部 inert）——`--session-mode` 公开契约锁定，默认 shared 逐字节零回归，per-client 分支挂点唯一化防散点 if/else 腐化
**Depends on**: Nothing（v1.1 首阶段；基线 = v1.0 已发布全绿）
**Requirements**: PC-01
**Success Criteria** (what must be TRUE):

  1. 用户以 `--session-mode=per-client`（或 TOML `session_mode = "per-client"`）启动被接受；缺省/显式 shared 启动后 v1.0 全量 Go 测试与既有协议 UAT 原样全绿、行为逐字节不变
  2. 非法模式值（CLI 或 TOML）在 parse 期被拒绝（exit 2），错误文案不泄露用户输入值内容（启动面红线保持）
  3. CLI flag > env > TOML > 默认的既定优先级链对 session_mode 成立（CLI 显式覆盖配置文件值）
  4. per-client 模式下启动预检（exec.LookPath 等 validateStartup 行）把命令缺失等配置错误暴露在启动期，而非推迟到首个客户端 attach 才失败

**Plans**: 5/5 plans executed（4/4 executed + 10-05 gap closure）
**Wave 1**

- [x] 10-01-PLAN.md — 模式阀门 tracer：--session-mode 全链装配（flag/TOML 键/枚举闸/Options.SessionMode+SpawnFunc/ValidateOptions/run() 分岔/StartWithSize）+ CLI 契约测试（PC-01，D-03/D-04）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 10-02-PLAN.md — validateStartup 双行（write-policy×per-client warn D-01/D-02 + per-client LookPath 预检 SC4）+ 启动矩阵/warn 合并/ValidateOptions/StartWithSize 委托等价测试（PC-01）
- [x] 10-03-PLAN.md — TOML 三面测试（merge/precedence/redlines）+ fuzz 语料五种子扩展（PC-01，D-03/D-04，Pitfall 11 同 PR）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 10-04-PLAN.md — D-05 文档最小明示（CONFIGURATION.md 五处 + README 一句 + --help 核对）+ 收口全量验证（-race 全量 + 既有协议 UAT 原样重跑 + 冒烟三命令）（PC-01，D-06）

**Gap closure** *(VERIFICATION 2026-09-02 gaps_found 18/19：WR-01 SC4 预检 --cwd 盲误拒相对路径 argv0 + WR-02 ValidateOptions 调用点在资源获取后零回滚——修复经 10-REVIEW-FIX 提交 189d081/0ec37cb 落 main，本 plan 出闭合断言与 fix 后全量双证据首跑)*

- [x] 10-05-PLAN.md — WR-01/WR-02 闭合：预检 --cwd 感知对齐六形态进程级冒烟 + ValidateOptions 前移位序断言 + 零回归收口闸（PC-01）

含：pty.StartWithSize（Start 委托、80x24 单一事实源纪律保持）、Options.SessionMode/SpawnFunc + New 互斥校验 fail-fast、配置 fuzz 语料扩展（session_mode 键入白名单 + 非法值 parse 拒绝同 PR）、write-policy=owner × per-client 组合的 validateStartup 处置（warn 或拒绝，规划期裁决——静默永不接受）。本阶段结束时不存在任何 per-client 运行期行为，接缝全部 inert；先锁定公开契约面（one-way flag 纪律）。

### Phase 11: per-client 生命周期主干

**Goal**: per-client 模式下每个浏览器客户端 attach 即获得独立 PTY 子进程，其生死只影响自己——核心 E2E 最长链成立
**Depends on**: Phase 10
**Requirements**: PC-02, PC-03, PC-04
**Success Criteria** (what must be TRUE):

  1. 两个客户端 attach 后各自获得独立 shell（协议层可观测两个不同 pid，各端输出即自身进程输出、互不串台）；首帧 winsize = Hello 上报尺寸经钳制（无 80x24 中间态闪烁）
  2. spawn 失败（命令不可执行/资源耗尽）时该客户端收到类型化 Error 帧（通用文案，绝不拼 err.Error() 回显路径/errno）并以 1011 关闭；服务端与其他在线客户端不受影响
  3. 客户端断开（正常关闭或异常 1006）后其子进程进程组立即收到 SIGHUP（随 --stop-signal 可配），无宽限、无僵尸残留；信号与收割锁内序列化，pgid 复用窗口内不误杀无关进程组
  4. 子进程退出（exit 42 或信号死亡）后仅该客户端收到私有 EXIT 帧（含 exit_code，信号死亡 -1）并以 1000 关闭；服务端与其他客户端继续运行

**Plans**: 6/6 plans executed
**Wave 1**

- [x] 11-01-PLAN.md — tracer 主干：per-client 端到端（attach spawn→Welcome 回显→五 goroutine 装配→断开 SIGHUP teardown Once 含 KILL 兜底→EXIT 私有化直写）+ New 分岔 + main sess=nil + harness 冒烟五测（PC-02/PC-03/PC-04，D-01/D-04）
- [x] 11-02-PLAN.md — darwin watcher dup-watch fail-closed 防御 + errDupWatch（PC-03，Pitfall 9）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 11-03-PLAN.md — 容量再闸（D-02 1011+容量文案 wire 形态）+ D-03 注册点复检回收 + spawn 失败 Pitfall 5 清理清单测试（PC-02）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 11-04-PLAN.md — 生命周期 Go 测试六测：EXIT 私有化两形态/断开 SIGHUP pgid ESRCH/重连新 pid/KILL 兜底时序双断言/teardown 恰好一次竞态注入（PC-03/PC-04）
- [x] 11-05-PLAN.md — web/uat/phase11.mjs 协议层八场景（D-06：双 pid 互不串台/首帧 winsize 钳制/运行期删命令 spawn 失败 1011/断开 ESRCH 无僵尸/EXIT 不串台/容量 1011 文案/重连新 pid/trap 免疫 KILL 兜底）（PC-02/PC-03/PC-04）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 11-06-PLAN.md — 收口闸：静态面 + 全量 -race + GOOS=darwin 编译闸 + phase02-09 默认模式零修改重跑 + phase11.mjs 全绿 + 期望值逐字未动 diff 审查（PC-02/PC-03/PC-04）

含：client.inQ/pc 字段、升档 per-client 分支（容量再闸 → hubMu 外 spawn → 失败 Error+1011 → Welcome 回显 → 注册+登记）、五 goroutine 装配（ReadLoop 闭包 / inputWriter 参数化 / writer / pinger / sessionWatcher）、EXIT 私有化直写、detach/kick SIGHUP 挂点（注册表移除点覆盖一切断开形态）、每会话 teardown sync.Once 固定序列 + reaped 栅栏、darwin watcher dup-watch fail-closed 防御。Welcome 恒首帧、exitf 恰好一次（termOnce 复用）、唯一收割者纪律三大不变量保持。

### Phase 12: per-client 交互与背压语义

**Goal**: per-client 模式下尺寸、输入、重连、慢客户端四类交互语义各归各会话——无仲裁、无串扰、重连即全新
**Depends on**: Phase 11
**Requirements**: PC-05, PC-06, PC-07, PC-10, PC-11
**Success Criteria** (what must be TRUE):

  1. 客户端 resize 直通自身 PTY 的 TIOCSWINSZ（[1,1000] 钳制与 50ms 防抖保留）；其他客户端尺寸不受影响，线上无 'W' 约束帧，resize 仲裁/owner 递补/fan-out/信用门在 per-client 分支不装配
  2. ro 客户端 attach 后照常获得自己的独立进程，其键盘输入被服务端丢弃（对自身进程同样无效，ro=自有进程输入门控）；每客户端输入限速保留
  3. 客户端异常断线（1006）重连后获得全新进程（新 pid），浏览器旧屏残留经 terminal.reset() 清除——用户看到干净的新会话而非旧屏残影
  4. 慢客户端停止消费时其 PTY 先被停读（输出积压于内核缓冲、子进程写阻塞而非丢数据），恢复消费后自动续读（ttyd pty_pause/resume parity）
  5. 持续过载的慢客户端 outbox 写满后以 1013 被踢出，服务端与其他客户端不受影响

**Plans**: TBD

含：INPUT 零分支 / RESIZE 直通两 case、每会话 resize 防抖（共用 debouncer 组件防双写漂移）、前端重连分支按 Welcome 模式位执行 terminal.reset() + dist 重建（本里程碑唯一前端改动）、per-PTY 停读/续读状态机。

### Phase 13: 资源防线与终结语义

**Goal**: per-client 模式下并发进程有硬顶、已认证 churn 打不垮服务端、HUP 免疫进程必被收割、关停覆盖全部存活进程组；--once/--exit-when-empty 触发语义与退出码规则成立（含注册表空迁移第二终结源），metrics/审计达 per-client 粒度，反代身份注入子进程环境
**Depends on**: Phase 11（spawn 路径/pcSessions 注册表/watcher/teardown）
**Requirements**: PC-08, PC-09, SEC-09, OPS-12
**Success Criteria** (what must be TRUE):

  1. 满员时第 max-clients+1 个客户端握手即收 503（既有闸保留）；并发 attach 竞态下「并发子进程数 ≤ max-clients」硬不变量始终成立（spawn 前 hubMu 内复检计数，无 ttyd 式 == 闸 + 异步 spawn 窗口超编）
  2. 已认证客户端高频断开重连（churn）被 spawn 双令牌桶（全局防惊群 + per-IP 防单点 churn）限速，取不到令牌在 spawn 前拒绝且关闭码避开 1006（前端不进入自动重连放大循环）；churn 负载下 RSS/goroutine/fd 有界
  3. SIGHUP 免疫的子进程在 stop-timeout 到期后被 SIGKILL 兜底收割、不泄漏（per-client 下 stop-timeout 默认值重议经用户裁决落地——公开契约变更）
  4. 优雅关停（SIGTERM/SIGINT）时全部存活 per-client 进程组各执行一遍 stop-signal 序列，有界 join 后退出（不等 D-state，不丢 session_end 事件）
  5. --once 下唯一客户端断开后服务端退出；--exit-when-empty 在全部客户端断开后触发退出——含「注册表已空且无子进程可等」形态下第二终结源生效（不会永不退出），退出状态 255 与 shared 语义对齐（第二终结源登记 Key Decisions）
  6. 「先断后死」「先死后断」两种时序下 wesh 退出码规则与 shared 模式逐位对齐（255 / 子进程退出码透传，last-reaped-code 规则）
  7. 运维者从 /metrics 读到 per-client 粒度指标（活跃会话 gauge、spawn 成功/失败与 kill 计数器），全部 series 保持零身份 label 红线；/healthz 的 session_alive 语义在 per-client 下按裁决落地
  8. 审计日志会话生命周期事件（session_start/session_end/spawn_failed）携带 pid 归因与 client_id 关联键，可串联单个 per-client 会话全生命周期；spawn_failed 事件零敏感值
  9. per-client 模式下 --auth-header 透传的用户名经 SEC-07 sanitize 后作为 WESH_REMOTE_USER 出现在该客户端子进程环境中（Web shell 内 env 可见，键名白名单固定）；shared 模式不注入（D-15 收窄语义不变）

**Plans**: TBD

含：容量硬帽机制（闸前检查 + 注册点复检回收）经 Phase 11 D-03 提前落地，本 phase 防线本体为 spawn 双令牌桶 + stop-timeout 默认值重议 + Shutdown N 进程组快照逐组信号（原裁决项④已消解）；churn 负载测试（合法票据 10rps × 30s，断言 RSS/goroutine/fd 有界）；pcSupervisor 单例（hubCond 等 `(pcExitReq||exiting) && active==0`，termOnce/terminate 单点收口——「exitf 唯一收口」per-client 同构映射）、healthz/metrics 四个 OQ 逐项裁决落地（规划期确认门）、metricsSeries17 镜像扩展 + 零身份 label 红线扩到新 series。准则 1-4 为研究锁定防线（PITFALLS #4 fork bomb / #8 HUP 免疫泄漏 ×N / #10 Shutdown 面），是 PC-08 硬不变量在 churn 与关停语境下的操作化。churn 防护先行于 Phase 14 压测（防护缺失会使压测失真）。

### Phase 14: 双模式验证矩阵、标定与 herdr UAT

**Goal**: 双模式零回归双证据收口；herdr/tmux driving scenario 端到端恢复正确行为；并发进程资源标定与双模式文档义务落地
**Depends on**: Phase 13（churn 防护缺失会使本 phase 压测失真）
**Requirements**: PC-12, PC-13
**Success Criteria** (what must be TRUE):

  1. -race 双模式全量 Go 测试 CI 全绿（mode-agnostic 同断言 / mode-mapped 断言分叉表 / mode-exclusive 不装配三维归类落地）；phase02-09 既有协议 UAT 以默认 shared 模式零修改重跑全过（零回归双证据）
  2. 协议层 UAT（Linux 开发机，web/uat/phaseNN.mjs 模式）断言 per-client 全链：双端双 pid、EXIT 不串台、resize 隔离、ro 门控、--once 退 255、spawn 失败 1011
  3. herdr（或 tmux）下 per-client 模式多客户端 attach：移动端小屏 attach 后桌面端面板尺寸不再被压缩（herdr is_foreground + per-client area 仲裁恢复生效）；Windows 工作站 Playwright 全链观感断言通过
  4. 并发进程负载矩阵（1/4/16/32 会话）实测内存/fd/goroutine/吞吐有界，数据回填 maxClients 默认建议值与 README 资源义务段
  5. README/CONFIGURATION/ARCHITECTURE 补 per-client 模型段（分享链接=按权限级别的独立进程入场券、ro=自有进程输入门控、配合 herdr/tmux 经多路复用汇聚）；v1.0「GoTTY 式共享进程模型」误记已修正（GoTTY 实为 per-connection spawn，源码已核实）

**Plans**: TBD

**Research flag**: 32 会话资源曲线为账面推算（唯一 MEDIUM 置信面），负载矩阵实测回填；herdr 端到端断言设计依赖外部子程序行为，需实测标定——建议 `/gsd-plan-phase --research-phase 14`。测试拓扑遵循双机分工（CODEBUDDY.md）：协议层 UAT 在 Linux 开发机（headless，禁浏览器），Playwright 浏览器全链在 Windows 工作站（经 TCP 转发器 kill/restore 模拟断网）。

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → … → 9（v1.0 shipped）→ 10 → 11 → 12 → 13 → 14（v1.1；2026-09-03 原 13/14 合并——原 13 经 Phase 11 D-01/D-03 机制先行收窄后独立 phase 开销过重，合并同时提前闭合 --once/--exit-when-empty 窗口期缺口）

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. 行走骨架（核心 PTY 管道） | v1.0 | 5/5 | Complete | 2026-08-14 |
| 2. 协议基线 | v1.0 | 6/6 | Complete | 2026-08-15 |
| 3. 认证与传输安全 | v1.0 | 7/7 | Complete | 2026-08-18 |
| 4. 前端体验 | v1.0 | 6/6 | Complete | 2026-08-19 |
| 5. 多客户端共享 | v1.0 | 13/13 | Complete | 2026-08-22 |
| 6. 会话生命周期与重连 | v1.0 | 7/7 | Complete | 2026-08-24 |
| 7. 部署与配置 | v1.0 | 10/10 | Complete | 2026-08-27 |
| 8. 可观测性 | v1.0 | 6/6 | Complete | 2026-08-28 |
| 9. 发布与打磨 | v1.0 | 10/10 | Complete | 2026-08-31 |
| 10. 模式装配与接缝 | v1.1 | 5/5 | Complete    | 2026-09-03 |
| 11. per-client 生命周期主干 | v1.1 | 6/6 | In Progress|  |
| 12. per-client 交互与背压语义 | v1.1 | 0/? | Not started | - |
| 13. 资源防线与终结语义 | v1.1 | 0/? | Not started | - |
| 14. 双模式验证矩阵、标定与 herdr UAT | v1.1 | 0/? | Not started | - |
