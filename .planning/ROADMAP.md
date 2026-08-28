# Roadmap: wesh

## Overview

从 PTY 核心管道出发（行走骨架），先把 WS 协议层一次性设计到位（类型化帧、三层上限、合规关闭码——事后补洞要动协议），再建立认证与 TLS 安全基线（多客户端权限需要身份概念先行）；随后补齐前端体验至 ttyd 基线对等，交付核心差异化能力——多客户端共享（fan-out、ro/rw 权限、背压、resize 仲裁），完善会话生命周期与断线重连，最后铺面部署配置与可观测性，以单静态二进制四平台发布收尾。v1 不做会话保持（用户以 tmux/herdr 覆盖），采用 GoTTY 共享进程模型：PTY 进程随服务端启动创建、多客户端共享，进程退出以类型化终结帧通知全部客户端；outbox/fan-out 结构为多客户端保留。

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
- [x] **Phase 6: 会话生命周期与重连** - --once/无人退出/类型化终结帧、断线重连接回同一进程 (completed 2026-08-24)
- [x] **Phase 7: 部署与配置** - 监听/base-path/配置文件/降权/子进程管理/auth-header 透传 (completed 2026-08-27)
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

**Plans**: 4/5 plans executed
**Wave 1**

- [x] 08-01-PLAN.md — slog 原子迁移：logEvent 迁入 log.go 换 slog JSONHandler + 动态 stderr writer + parseEvents helper + 5 Go 测试与 phase05/07 两 UAT 脚本断言迁移 JSON 行解析（OPS-08，D-13/D-14/D-15/D-16/D-18）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 08-02-PLAN.md — 审计事件目录：attach/detach（client_id + reason 四值）/session_start/session_end/shutdown + throttled retry_after + remote 字段 sanitize 推广（OPS-08，D-17/D-19/D-20/D-21/D-22/D-23）

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 08-03-PLAN.md — /healthz：免认证 200 JSON 四字段 + 根路径固定 + 405 成对 + sessionAlive/draining 两 atomic.Bool（OPS-06，D-07/D-09/D-10/D-11/D-12）

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 08-04-PLAN.md — /metrics：手写 text 0.0.4 exposition 17 series + hubMu 快照 + 认证闸跟随 + Options.Version + 计数器挂点兑现（OPS-07，D-01..D-06/D-08/D-09/D-12）

**Wave 5** *(blocked on Wave 4 completion)*

- [ ] 08-05-PLAN.md — 收口：phase08.mjs 六场景 UAT + README 运维节（免认证例外/Prometheus 配方/jq 检索）+ 08-UAT.md + 全量六段式回归（OPS-06/07/08）

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
| 1. 行走骨架（核心 PTY 管道） | 5/5 | Complete    | 2026-08-14 |
| 2. 协议基线 | 6/6 | Complete    | 2026-08-15 |
| 3. 认证与传输安全 | 7/7 | Complete    | 2026-08-18 |
| 4. 前端体验 | 6/6 | Complete    | 2026-08-19 |
| 5. 多客户端共享 | 13/13 | Complete    | 2026-08-22 |
| 6. 会话生命周期与重连 | 7/7 | Complete    | 2026-08-24 |
| 7. 部署与配置 | 10/10 | Complete    | 2026-08-27 |
| 8. 可观测性 | 4/5 | In Progress|  |
| 9. 发布与打磨 | TBD | Not started | - |
