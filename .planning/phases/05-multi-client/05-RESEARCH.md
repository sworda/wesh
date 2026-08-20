# Phase 5: 多客户端共享 - Research

**Researched:** 2026-08-19
**Domain:** Go WS 服务端多客户端 fan-out / 背压 / 权限仲裁 / 分享链接认证通道
**Confidence:** HIGH（核心机制全部经本机源码/实证验证：GOROOT 1.26.3 net/http 源码、coder/websocket v1.8.15 模块缓存源码、x/time v0.10.0 模块缓存源码、Linux SIGWINCH/TIOCGPGRP 实证探针、UDP-dial 选路实证；个别 darwin 行为与参数初值为 MEDIUM/ASSUMED，逐条标注）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**分享链接认证形态（MULTI-05）**
- **D-01:** 链接 token = 独立第三认证通道：启动时生成 ro/rw 两个 128bit 随机 token；持有效 token 可 GET 页面 + POST /api/attach 换 ticket（绕过 Basic）；无/错 token 时 P3 Basic 矩阵不变；与凭据共存（operator 走凭据、旁观者走链接）。P3 D-11 的"/api/attach 可选 mode 参数"预期细化作废——mode 由 token 绑定（ro token → ro ticket、rw token → rw ticket），/api/attach 不收 mode 参数
- **D-02:** token 可复用至进程重启；"一次性"落在 SEC-02 ticket 上（token → /api/attach → 一次性 ticket → Hello 链路不变）；ro/rw token 每轮启动重新随机生成——重启即废全部旧链接，吊销语义 = 重启
- **D-03:** URL 形态 `/s/{token}/` 路径段：服务端页面 GET 时即可校验 token 完成门禁（fragment 做不到这一点）；token 永不作 logEvent 参数（SEC-01 红线延伸）；README 明示反代访问日志会记录路径、建议脱敏
- **D-04:** 启动打印链接的 host：bind 为 0.0.0.0/:: 时回填首个非 loopback IPv4 接口地址；具体 bind 地址原样使用；scheme 感知同 D-04 TLS 分支（https:// 当 TLS 启用）

**写权限模式与 owner 归属（MULTI-02）**
- **D-05:** `--writable` 保持总闸（不给 = 只打印 ro 链接，全员只读，现状语义零漂移）；新增 `--write-policy=owner|all` 默认 **owner**（安全默认哲学：旁观是被动场景、协作主动开启）
- **D-06:** owner = 首个以 rw 身份完成 attach 的客户端（无显式指定通道——服务端无本地终端 UI，operator 也是浏览器客户端）；owner 断线后按 attach 顺序递补
- **D-07:** owner 在位时后续 rw attach 降级 ro + 进递补队列（Welcome mode=ro；owner 断线自动升格 rw 并推送通知）；复用现有 ro 前端形态（disableStdin + `[ro] ` 标题前缀），零新 UI 组件
- **D-08:** RES-03 开 `--max-clients` flag（容量策略是部署关切，与 P2 D-10 攻击面上限常量不同类）；满员在 Accept 前以 HTTP 503 拒绝（Attach 守卫区既有形态延伸）；默认初值负载测试标定、Phase 9 回填

**resize 仲裁参与集（MULTI-04 细化）**
- **D-09:** 参与集按写权限分层：owner 模式仅 owner 尺寸参与（递补后新 owner 尺寸接管）；all 模式全部 rw 端取 min；无 --writable 纯 ro 会话全体 ro 端取 min（否则会话冻结 80x24）。ro 旁观者永不影响可写端尺寸。P2 D-13 修订："ro 放行 RESIZE"是单客户端语境，多客户端下 ro 端 RESIZE 不参与仲裁——服务端直接忽略，前端 ro 形态不发 RESIZE 帧。推论（算法直接结果，无需 S→C 尺寸下发帧）：min-rect 保证任何客户端窗口 ≥ PTY 尺寸，各端按自己窗口渲染、多余面积留白

**踢出与新客首屏（MULTI-03 UX）**
- **D-10:** 1013 踢出后前端 onclose 按码分派 → showStatus 提示 + 手动刷新（英文文案，"因接收过慢被断开"语义）；自动重连全部归 Phase 6 CORE-05，本 phase 不做任何自动重连（避免后台标签页重连→再被踢循环）
- **D-11:** 新客户端 attach 完成时，服务端向 PTY 前台进程组显式发一次 SIGWINCH（TIOCGPGRP 取 pgid → kill 进程组）强制全屏程序重绘——新客秒见画面；行内 shell 下次输出自然追上；TIOCGPGRP 失败/无前台进程组时静默降级
- **D-12:** 标题保持纯前端解析（P4 D-01 终局裁决）：服务端 OUTPUT 零拷贝不跑 OSC 状态机；新客标题随 SIGWINCH 重绘/下次 OSC 2 自然恢复；'T' TITLE 帧终局不实现（类型字节保留注释）

**旁观端 OSC52 强制关（兑现 P4 deferred）**
- **D-13:** 多客户端下 ro 端（旁观者）强制不下发 `osc52:true`，即使全局 --osc52 开启——rw 端按全局 --osc52 下发；prefs 按客户端 mode 分化

**领域推论（已锁定，非新决策）：** P1 D-11 单次语义终结——任何客户端断开不再触发 exitf/SIGHUP，服务端生命周期只随子进程（D-10 唯一终结路径），无客户端时子进程继续运行、分享链接仍可 attach；"所有客户端断开退出"是 Phase 6 SESS-02 的可配模式。CR-01 完整背压修复（有界输入队列 + 写 goroutine）落本 phase。1013 常量占位启用（P2 D-08）。outbox/限速/max-clients 参数走常量纪律（P2 D-10，负载测试标定 Phase 9 回填，Phase 7 配置文件收口）。

### Claude's Discretion（本 research 逐项裁决，见「裁断建议」节）

- 递补升格的 S→C 通知通道（复用 'W' Welcome 帧运行期推送 vs 新类型字节；遵守 P2 D-01/D-02 纪律）
- 分享 token 存储与比较形态；token 校验失败是否计入 throttleStore 同一 per-IP 计数器
- outbox 容量/水位/strikes 与输入限速 rate 参数初值、--max-clients 默认初值
- 首个非 loopback IPv4 接口选取策略（多接口排序）、/s/{token}/ 路由装配、token 无效时页面响应形态与文案
- max-clients 计数口径（半开是否计入——倾向 attach 成功后计入，与 halfOpen 正交）
- permission_denied code 入表的具体使用场景评估（无真实使用场景则保持占位注释不硬用）
- 1013 close reason 机器串与 stderr logEvent reason 命名（slow_consumer 类）
- 客户端注册表数据结构与 fan-out hub goroutine 拓扑（ReadLoop 单读 → hub try_send → 每客户端 writer）
- 输入限速器（x/time/rate）超限时的行为（丢弃 vs 断开）按 RES-02 语义定

### Deferred Ideas (OUT OF SCOPE)

- 完整断线自动重连（指数退避 + 上限 + 手动入口）— Phase 6 CORE-05；本 phase 1013 仅提示 + 手动刷新
- --once / 所有客户端断开退出模式 — Phase 6 SESS-01/02；本 phase 服务端生命周期只随子进程
- EXIT 终结帧多客户端广播形态 — Phase 6 SESS-03（类型字节已占住）
- outbox/限速/max-clients 参数标定回填 — Phase 9（负载测试标定）
- 每客户端 outbox 深度与 1013 踢出计数进 metrics — Phase 8 OPS-07
- 1001 优雅下线发送路径 — Phase 7
- 新 flag 配置文件收口（--write-policy/--max-clients）— Phase 7 OPS-09
- 'T' TITLE 帧 — 终局不实现（D-12）
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MULTI-01 | 多个 WS 客户端同时 attach 同一会话，输出实时扇出 | Pattern 1（fan-out hub 拓扑）+ Pattern 2（有界 outbox/共享帧）；coder 持锁写反例已证伪（ARCHITECTURE §2.5 HIGH） |
| MULTI-02 | 写入权限可配置 all/owner | 注册表模式判定矩阵（D-05..D-07 落地形态，见裁断 R-07）；递补升格复用 'W' Welcome（R-09） |
| MULTI-03 | 慢客户端不阻塞他人：有界 outbox 写满 1013 踢出，重连从最新输出看起 | Pattern 2 + 踢出机制安全性验证（`go c.Close(1013)` 不阻塞 hub，close.go 源码核实）；1013 = `websocket.StatusTryAgainLater`（VERIFIED 逐字引用） |
| MULTI-04 | resize 仲裁：单客户端 last-wins；≥2 最小公共矩形；2→1 恢复 | Pattern 4（仲裁器 + 50ms 防抖 + 参与集分层）；D-09 参与集矩阵；`pty.Getsize` 断言通道核实存在 |
| MULTI-05 | 启动打印 ro/rw 两条分享链接即打即用 | Pattern 6（/s/{token}/ mux 装配，GOROOT 源码核实通配语义与 301 行为）+ Pattern 7（UDP-dial 选路实证） |
| RES-02 | 每客户端输入速率限制 | x/time/rate 令牌桶（模块缓存源码核实 Allow/AllowN 语义）+ 超限丢弃裁决（R-02） |
| RES-03 | 最大并发客户端数限制，满员拒绝 | 守卫区 503 闸（替换 409 位）+ 计数口径裁决（R-06） |
| RES-04 | PTY 输出背压：暂停读 PTY 或断开慢客户端 | Pattern 3（全局信用门：cond 变量门 + resume 水位 + pinger 死锁免除证明，mu.lock ctx 语义源码核实） |
</phase_requirements>

## Summary

本 phase 把单客户端服务端升级为多客户端共享，核心工程面有五块：(1) **fan-out 拓扑**——ReadLoop 单读不变，`onChunk` 改造为 hub：对每客户端有界 outbox 做 `trySend`，写满即 1013 踢出（ro/普通 rw）或计入全局信用（见下），每客户端专属 writer goroutine 独占 WS 写端；(2) **全局信用**——全体可写客户端 outbox 均满时 hub 持块等待（停读 PTY → 内核缓冲填满 → 子进程自然阻塞），任一可写端 drain 至半水位恢复；(3) **生命周期改造**——单次语义终结，客户端断开只触发注册表移除 + 递补/仲裁重算，子进程退出成唯一终结路径（广播 1000 + exitf）；(4) **分享链接第三认证通道**——`/s/{token}/` 路径段门禁 + /api/attach body 携 token 换 mode 绑定 ticket；(5) **CR-01 完整背压修复**——有界输入队列 + 单写 goroutine + 每客户端 x/time/rate 输入限速。

关键验证结论（全部本 session 一手完成）：`websocket.StatusTryAgainLater = 1013` 库常量存在；`c.Close()` 内部 5s+5s 自有界且解除全部 goroutine 阻塞，因此**踢出必须 `go c.Close(...)` 独立 goroutine 执行、hub 绝不内联调用**；Linux 同尺寸 TIOCSWINSZ **不发** SIGWINCH（实证探针：3 次 Setsize 仅 1 次命中），D-11 显式 SIGWINCH 必要且全链路（TIOCGPGRP→kill(-pgid)）实证可达；Go 1.22+ mux `GET /s/{token}/` 通配语义、`/s/abc` → 301 → `/s/abc/` 行为经 GOROOT 源码核实；x/time/rate `Allow()` 官方语义即"超限丢弃"；UDP-dial 选出站 IP 技巧本机实证（正确避开 docker0/bridge 接口）。

**Primary recommendation:** 按 Pattern 1-4 的 hub/outbox/credit/arbiter 四件套落地：outbox 字节有界 512KiB/客户端（共享帧零逐客户端拷贝）、满即 1013 踢出（唯一例外：全体可写端满 → 信用门停读）；注册表单 mutex + FIFO 递补队列；新依赖仅 `golang.org/x/time`（rate 限速器）与 `golang.org/x/sys`（transitive→direct，TIOCGPGRP）。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 输出 fan-out / outbox / 1013 踢出 | API/Backend (Go server) | — | 背压是会话资源治理，必须在服务端；前端只被动收 1013 |
| 全局信用停读 PTY | API/Backend | — | ReadLoop 与注册表的唯一交点，服务端私有 |
| resize 仲裁（min-rect/防抖/钳制） | API/Backend | Browser（fit+RESIZE 上报） | PTY winsize 唯一属主在服务端；前端 ro 不发 RESIZE 是减负非仲裁 |
| owner/rw/ro 模式判定与递补 | API/Backend | Browser（Welcome mode 应用） | 权限真边界在服务端（P2 D-13/D-14 纪律延伸）；前端只做 UX 层 disableStdin/标题 |
| 分享 token 签发/校验/throttle | API/Backend | — | 认证通道属服务端；token 永不进 logEvent（D-03 红线） |
| 输入限速（RES-02） | API/Backend | — | 每客户端 limiter 在服务端 INPUT 门内；前端无感知 |
| /s/{token}/ token 提取与 attach 流程 | Browser (main.ts) | API/Backend（页面门禁） | URL 解析天然前端职责；服务端在 GET 时已完成门禁 |
| 1013 踢出文案 + 手动刷新 | Browser | — | onclose 按码分派既有形态（D-10），英文文案 |
| 升格 Welcome 运行期处理（ro→rw 翻转 + fit/RESIZE 纠正） | Browser | API/Backend（推送 Welcome） | disableStdin/标题前缀切换在前端（D-07 复用既有 ro 形态） |
| OSC52 旁观端强制关（D-13） | API/Backend（prefs 按 mode 分化） | Browser（osc52 键缺席即不加载） | 安全不对称纪律：安全敏感项只能服务端控制（P4 D-12 延伸） |
| SIGWINCH 强制重绘（D-11） | API/Backend（pty.Session 方法） | — | TIOCGPGRP/kill 是 PTY fd 操作，归 pty 包持 fdMu 纪律 |

## Project Constraints (from CODEBUDDY.md)

从 `/data1/home/zexueli/open_src/wesh/CODEBUDDY.md` 与本机全局 CODEBUDDY.md 提取的可执行指令，planner 必须核对：

1. **纯 headless 环境，永不具备浏览器/GUI**——禁止安装 playwright 及任何 GUI/X11 库；禁止建议"装浏览器再测"；禁止启动 wesh 实例等人工浏览器访问。
2. **测试分层策略（强制）**：协议层 `web/uat/phaseNN.mjs`（Node 原生 WebSocket/fetch 零依赖，spawn 真实二进制断言）；终端核心逻辑 `@xterm/headless`（需 `allowProposedApi: true`）；前端 DOM 逻辑 jsdom + mock；平台原生行为（权限弹窗/原生 confirm/真实 IME/像素视觉）显式豁免，UAT 以 `skipped` + reason 记录。
3. **pnpm 而非 npm**；构建命令带 `time` 前缀；偏好全量构建；编译超时 20 分钟；编译后验证产物时间戳。
4. **必须始终使用中文回答**；技术术语/代码标识符/文件路径保持原文。
5. 关注空指针检查、数据一致性、边界条件；不接受过度设计或不必要的新类型/枚举。
6. 修改前备份当前状态；编译通过后才进行下一步。

对 Phase 5 的直接推论：UAT 走 `web/uat/phase05.mjs`（phase02/03/04.mjs 先例形态）；慢客户端真 stall 场景在 Go 集成测试落地（测试客户端不调用 Read 即可制造 TCP  stall），不依赖任何浏览器。

## Standard Stack

### Core（全部既有依赖，无新增核心库）

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/coder/websocket | v1.8.15 [VERIFIED: go.mod:6] | WS 连接、Close(1013) 踢出、pinger | 既有依赖；`StatusTryAgainLater=1013` 常量核实存在（见下逐字引用） |
| github.com/creack/pty | v1.1.24 [VERIFIED: go.mod:7] | PTY spawn/Setsize/Getsize | 既有依赖；`Getsize(t *os.File) (rows, cols int, err error)` [VERIFIED: creack/pty@v1.1.24/winsize.go:18] 供仲裁断言 |
| golang.org/x/sys | v0.47.0 [VERIFIED: go.mod:8，indirect→direct] | `unix.IoctlGetInt` + `unix.TIOCGPGRP` + `unix.Kill`（D-11 SIGWINCH） | 已在依赖树（indirect），本 phase 转 direct；常量双平台核实（见下） |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| golang.org/x/time | v0.15.0（proxy 最新；模块缓存已有 v0.10.0 可离线 fallback）[VERIFIED: `go list -m -versions golang.org/x/time` 输出 v0.1.0…v0.15.0] | `rate.Limiter` 每客户端输入限速（RES-02） | Attach 升档时按客户端构造；INPUT 门内 `AllowN` |

**Installation:**
```bash
go get golang.org/x/time@latest   # rate API 十年稳定；go.sum 经 sum.golang.org 校验
go mod tidy                       # x/sys 由 indirect 转 direct
```

**Version verification:** `go list -m -versions golang.org/x/time` 本 session 实测返回至 v0.15.0（网络可达 proxy）；模块缓存已含 v0.10.0（断网可 fallback）。x/time v0.10.0 的 `rate.go` 已逐行核实 API 语义（见 Code Examples 节逐字引用）——`NewLimiter`/`Allow`/`AllowN` 签名自 2015 年起稳定，v0.10.0→v0.15.0 无行为差异风险 [ASSUMED：未逐版本 diff，但 rate 包历史极稳定]。

## Package Legitimacy Audit

> 合法性门禁 seam（`gsd-tools query package-legitimacy check`）仅支持 npm|pypi|crates 三个生态，**不支持 Go modules**——本 phase 唯二新 direct 依赖走 Go module proxy + sum.golang.org 校验链验证（等效权威：`golang.org/x/*` 命名空间即 Go 官方团队扩展库，非第三方可注册）。前端零新 npm 依赖。

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| golang.org/x/time | Go module proxy（sum.golang.org 校验） | ~10 年（2015 年起） | 标准库级普及 | go.googlesource.com/time（Go 官方） | OK | Approved |
| golang.org/x/sys | Go module proxy | 既有 indirect 依赖（v0.47.0 已在 go.sum） | 标准库级 | go.googlesource.com/sys | OK | Approved（indirect→direct） |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                         浏览器客户端 A(rw/owner)  B(ro spectator)  C(rw queued)
                               │ POST /api/attach {token?} → ticket(mode-bound)
                               │ WS /ws + Hello{ticket,cols,rows}
┌────────────────────────────────── wesh 单进程 ──────────────────────────────────┐
│                                                                                 │
│  HTTP mux:  / (basicAuth→embed)   GET /s/{token}/ (token 门禁→embed index)      │
│             POST /api/attach (Basic 或 token→ticket)   /ws → Attach             │
│                                                                                 │
│  Attach 守卫区(Accept 前零 WS 资源): ⓪Origin 403 → ①子协议 400 →                │
│       ②halfOpen 429 → ③max-clients 503（替换 P1 409 门）                        │
│                          │ Hello 核销(mode 解析)                                │
│                          ▼                                                      │
│  ┌──────────── Client Registry（单 mutex；map + FIFO 递补队列 + owner 指针）──┐ │
│  │  per client: conn / mode(ro|rw) / rwEligible / attachSeq /                 │ │
│  │              outbox(字节有界) / writer goroutine / pinger / rate.Limiter   │ │
│  └──────┬───────────────────▲──────────────────────────────┬─────────────────┘  │
│         │ register/detach   │ Welcome(promotion)           │ INPUT/RESIZE       │
│         ▼                   │                              ▼                    │
│  ┌──────────── Hub（= ReadLoop goroutine 内 onChunk）──────┐│  ┌─ 输入方向 ────┐ │
│  │  门: 全体可写端 outbox 满 → hubCond.Wait（停读 PTY）    ││  │ limiter.AllowN│ │
│  │  fan-out: 每客户端 outbox.trySend(共享帧)               ││  │  ↓ 超限丢弃   │ │
│  │   ├ 满 & 可踢(非"最后可写端") → 登记移除 +             ││  │ 输入队列(有界) │ │
│  │   │   go c.Close(1013,"slow_consumer") + logEvent      ││  │  ↓ 满则丢弃   │ │
│  │   └ 满 & 全体可写端均满 → creditBlocked，持块等待       ││  │ input writer  │ │
│  │       （任一 writer drain 至半水位 → Broadcast 恢复）   ││  │ goroutine     │ │
│  └──────┬────────────────────────────────────────────────┘│  └──────┬────────┘ │
│         │ outbox per client（共享帧引用，逐客户端字节计数） │         │ Master.Write
│         ▼                                                 │         ▼           │
│   writer A / writer B / writer C（各自独占 WS 写端，批量 drain 合并单帧）         │
│         ▼                                                 │   ┌──────────────┐  │
│        WS A / WS B / WS C                                 │   │ pty.Session  │  │
│                                                           │   │ master fd    │  │
│  Arbiter（hub 持有）: 参与集按 D-09 分层 → min(cols)×min(rows) │  │ TIOCSWINSZ   │  │
│   resize 上报 50ms 防抖 / detach 即时重算 / 2→1 last-wins    │   │ TIOCGPGRP→   │  │
│                                                           │   │ kill(-pgid,  │  │
│  lifecycle（唯一终结路径）: Wait → Drain(200ms) →            │   │  SIGWINCH)   │  │
│   广播 1000（并行 Close + 有界等待）→ exitf(code)            │   └──────────────┘  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── proto/proto.go        # 既有 + 本 phase：1013 注释启用、升格 Welcome 说明（不加新类型字节）
├── server/
│   ├── server.go         # Attach 改造（注册表登记/守卫生 503）、lifecycle 广播、wsDisconnected→detach
│   ├── clients.go        # 【新】client 结构 + 注册表（map+FIFO+owner）+ outbox + writer + hub fan-out + 信用门
│   ├── resize.go         # 【新】arbiter：参与集分层/min-rect/50ms 防抖/即时重算
│   ├── sharetoken.go     # 【新】分享 token store（ro/rw 两条目 + subtle 比较）+ /s/{token}/ handler
│   ├── tickets.go        # 既有（mode 绑定占位字段本 phase 兑现，结构零改动）
│   ├── throttle.go       # 既有（token 失败经既有 401 路径自然计入，见裁断 R-03）
│   └── auth.go           # 既有 + /api/attach body token 分支
├── pty/
│   ├── io.go             # 既有 + Session.SignalForegroundGroup()（TIOCGPGRP→SIGWINCH，fdMu 纪律）
│   └── （spawn/reap 不动）
cmd/wesh/main.go          # --write-policy/--max-clients flag、分享链接两行打印、出站 IP 回填
web/src/main.ts           # /s/{token}/ 提取、attach body 携 token、1013 文案、ro 不发 RESIZE、升格 Welcome 处理
web/uat/phase05.mjs       # 【新】协议层 UAT
```

新文件仅 3 个（clients.go / resize.go / sharetoken.go），符合"不接受过度设计"约束；hub 不独立成 actor goroutine——**hub 就是 ReadLoop goroutine 内的 onChunk 调用**（零额外线程，串行化天然成立）。

### Pattern 1: Fan-out Hub 拓扑（ReadLoop 单读 → hub trySend → 每客户端 writer）

**What:** ReadLoop 保持唯一 PTY 读者（D-12 drain 语义不变）；`onChunk` 从"单 conn 直写"改造为 hub：持 registry 锁遍历客户端做 outbox `trySend`；每客户端专属 writer goroutine 是唯一 WS 写端。
**When:** 一对多有序字节流扇出，消费者速度不均。
**Why:** coder buffered.go 持锁遍历阻塞写是已核实反例（ARCHITECTURE §2.5 HIGH）；trySend 永不阻塞 → hub 临界区有界 → ReadLoop 永不因客户端阻塞（MULTI-03/RES-04 硬要求）。

```go
// hub 扇出（在 ReadLoop goroutine 内运行；hubMu 同时护 registry 与信用门状态）
func (s *Server) onChunk(chunk []byte) {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	// 全局信用门（Pattern 3）：全体可写端满 → 持块等待（停读 PTY）
	for s.allWritableBlockedLocked() {
		s.hubCond.Wait() // Wait 期间释放 hubMu；chunk 持于 ReadLoop 缓冲（别名利用安全：阻塞期间无下次读）
	}
	frame := make([]byte, 1+len(chunk)) // 每 chunk 仅一次拷贝；全部客户端共享同一只读帧
	frame[0] = proto.Output
	copy(frame[1:], chunk)
	for c := range s.registry.iter() {
		if !c.outbox.trySend(frame) { // 共享帧：outbox 存引用不拷贝（见 Pattern 2 内存分析）
			s.kickOrCreditLocked(c)   // 满 → 1013 踢出 或 计入信用（Pattern 3 规则）
		}
	}
}
```

**关键纪律：**
- **chunk 别名红线**：`pty/io.go:13-14` 注释明示 [VERIFIED: internal/pty/io.go:13-14]——"onChunk 在读循环 goroutine 内同步调用、复用底层缓冲——回调方如需跨帧持有须自行拷贝"。hub 组帧一次（`frame := make(...)`）后**跨客户端共享该只读帧**，满足拷贝纪律且做到逐客户端零拷贝。
- **踢出绝不内联 Close**：`c.Close()` 文档逐字："It will write a WebSocket close frame with a timeout of 5s and then wait 5s for the peer to send a close frame." [VERIFIED: coder/websocket@v1.8.15/close.go:87-89]。hub 内联调用 = 单慢客户端阻塞 ReadLoop 最长 ~10s（正是本 phase 要消灭的行头阻塞）。正确形态：`go c.Close(websocket.StatusTryAgainLater, "slow_consumer")`——Close 幂等（"The connection can only be closed once." [VERIFIED: close.go:92]）且 "Close will unblock all goroutines interacting with the connection once complete" [VERIFIED: close.go:95-96]，被踢客户端卡死的 writer/reader 随 Close 完成解除阻塞。
- **1013 库常量**：`StatusTryAgainLater           StatusCode = 1013` [VERIFIED: coder/websocket@v1.8.15/close.go:52]。
- **reason 上限**："The maximum length of reason must be 125 bytes. Avoid sending a dynamic reason." [VERIFIED: close.go:93]——机器串 `slow_consumer` 13 字节，静态串，合规。

### Pattern 2: 字节有界 Outbox + 共享帧（内存上界 ≈ 最慢者滞后量，非求和）

**What:** 每客户端 outbox = `[][]byte` 队列 + 字节计数 + cap 1 信号 channel；存的是 hub 组好的**共享帧引用**（只读），逐客户端只记字节账。
**When:** 帧计数有界（chan cap N）在帧大小不均时内存上界失真（1B 帧与 32KiB 帧同占一席）。
**Why:** Pitfall 4 纪律要求字节级上界；共享帧使全局 WS 出站内存 ≈ max（各客户端滞后量） 而非 Σ——N 个客户端挂满同一输出流时，保留的帧区间是重叠的（并集 ≤ 最慢客户端的 outbox 容量）。

```go
type outbox struct {
	mu       sync.Mutex
	q        [][]byte // 共享帧（hub 分配、只读，引用计数靠 GC 自然回收）
	bytes    int
	cap      int
	notEmpty chan struct{} // cap 1 信号量：trySend 非阻塞投递，writer 阻塞消费
}

func (o *outbox) trySend(frame []byte) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.bytes+len(frame) > o.cap {
		return false
	}
	o.q = append(o.q, frame)
	o.bytes += len(frame)
	select {
	case o.notEmpty <- struct{}{}:
	default: // 已有信号在飞，writer 必会 drain 到本帧
	}
	return true
}

// writer goroutine：每客户端唯一 WS 写端。批量 drain 合并单帧（ARCHITECTURE §2.5 写合并）。
// c.Write(ctx=Background) 允许阻塞——阻塞即"该客户端慢"，由 outbox 满 → 踢出/信用收口；
// pinger 的 Ping 经库 writeFrameMu 与数据写串行化，无帧交错（既有 02-04 纪律）。
func (s *Server) writer(c *client) {
	for {
		select {
		case <-c.done:
			return
		case <-c.outbox.notEmpty:
		}
		batch, bytes := c.outbox.drain() // swap 出整队，重置计数
		if len(batch) == 0 {
			continue
		}
		if err := c.conn.Write(context.Background(), websocket.MessageBinary, concatFrames(batch)); err != nil {
			return // 写失败：终结由该客户端 reader 路径收口（既有 D-11 纪律的多客户端映射）
		}
		c.afterDrain(bytes) // 信用门恢复判定（半水位）+ cond.Broadcast
	}
}
```

**Welcome 帧也走 outbox**（registration 时首条入队）——writer 是全程唯一写端，"Welcome 恒为 S→C 首帧"由 FIFO 自然保证，消除直写/outbox 双通道排序问题。握手期 Error 帧（version_mismatch/auth_failed）发生在注册前，维持直写不变。

### Pattern 3: 全局信用门（全体可写端阻塞 → 停读 PTY）

**What:** 信用集合 = 当前可写（rw）客户端。fan-out 前检查：≥1 个可写端且**全部** creditBlocked → hub 在 `sync.Cond` 上等待（onChunk 持块 = ReadLoop 停读 = PTY 内核 64KiB 缓冲填满 = 子进程 write 自然阻塞——ARCHITECTURE §2.6"唯一合法的反压到生产者"）。任一可写端 writer drain 至 outbox 字节 < cap/2（半水位迟滞防门震颤）→ 清 creditBlocked + `Broadcast`。
**踢出与信用的分工（裁断 R-08 核心）：**

| 场景 | 动作 |
|------|------|
| ro 客户端 outbox 满 | 立即 1013 踢出（旁观者可弃，永不持信用） |
| rw 客户端满，但**不是**全体可写端都满 | 立即 1013 踢出（慢的是离群者，MULTI-03 本义） |
| 全体可写端均满（≥1） | 不踢：hub 信用门等待（保护 owner 演示者；纯 ro 会话信用集为空 → 门永不闭合 → 满即踢） |
| 门闭合期间可写端死亡 | pinger pong_timeout → CloseNow → reader 错误 → detach → 门重估（死锁免除证明见下） |
| 门闭合期间子进程退出 | lifecycle 广播 Close → writer/reader 全部出错 → detach → 注册表空 → 门开 → ReadLoop 续 drain（D-12） |

**死锁免除证明（源码级）：** 门闭合时某可写端 TCP 全死（合盖）——其 writer 卡死在 `c.Write` 持 `writeFrameMu`；pinger 的 `Ping` → `writeControl`（自带 5s ctx）→ `writeFrameMu.lock(ctx)` 超时返回 `fmt.Errorf("failed to acquire lock: %w", ctx.Err())` [VERIFIED: coder/websocket@v1.8.15/conn.go:286-292，`mu.lock` 的 `case <-ctx.Done()` 分支逐字]——`errors.Is(err, context.DeadlineExceeded)` 为真 → 既有 02-04 pinger 精确分类（仅 DeadlineExceeded → pong_timeout + CloseNow）正确触发 → 死连接 ≤(ping 间隔 + 5s) 内 detach → 门重估。另：`writeFrame` 内 `c.setupWriteTimeout(ctx)` 把 ctx 期限翻译为 `net.Conn.SetWriteDeadline` [VERIFIED: write.go:318-320]，Close 的 5s 写超时对全 stall TCP 同样生效。

**为何不需要独立的"信用超时"定时器：** 全死连接由 pinger 有界收口（上）；慢而活着的滴漏读者每次 drain 即恢复（门开），下一 chunk 可能再闭门——振荡但各端持续前进，且只有"全体可写端同步慢"才发生，可接受（Phase 9 负载标定时再评估是否加 dwell 计时）。

### Pattern 4: Resize 仲裁器（参与集分层 + min-rect + 50ms 防抖）

**What:** hub 持有的仲裁器：`sizes map[*client]dims`（仅参与集成员）+ 单 `time.Timer` 防抖。RESIZE 上报 → 更新 → 重置 50ms timer；detach/递补 → **即时**重算（无风暴风险）。目标尺寸变化才 `sess.Resize`（TIOCSWINSZ，内核自动 SIGWINCH——异尺寸时）。
**When:** MULTI-04 + D-09。
**参与集矩阵（D-09 落地）：**

| 会话形态 | 参与集 | 算法 |
|---|---|---|
| `--writable --write-policy=owner` | 仅 owner（其 Hello/RESIZE 最新上报） | 1 人 = last-wins（即 owner 尺寸）；递补后新 owner 尺寸接管 |
| `--writable --write-policy=all` | 全部 rw 端 | ≥2 → min(cols)×min(rows)；2→1 恢复 last-wins（剩余者尺寸） |
| 无 `--writable`（纯 ro） | 全部 ro 端的 **Hello 尺寸**（D-09：ro RESIZE 忽略） | 同上 min-rect；attach/detach 时重算 |
| ro 旁观者（任何含可写端会话） | **不参与** | 服务端直接忽略其 RESIZE（第二闸；前端 ro 不发为第一闸） |

**已知行为推论（须进 README）：** 纯 ro 会话中旁观者窗口运行期缩放不上报（D-09 省流量裁决）——窗口缩到小于 PTY 尺寸的旁观者看到裁剪画面，重新 attach 恢复。min-rect 保证任何客户端窗口 ≥ PTY 尺寸，各端多余面积留白（D-09 推论，无需 S→C 尺寸下发帧）。

**钳制沿用既有：** `DecodeResize`/`DecodeHello` 已钳 [1,1000]（`ClampDim`，proto.go:168-176）；`sess.Resize(cols, rows)` 参数序陷阱已有测试注释锁定（io_test.go:24-25：切勿按 (rows, cols) 序误传）。

### Pattern 5: 客户端注册表与模式判定矩阵

**What:** `registry struct { mu(=hubMu); set map[*client]struct{}; order []*client（attach FIFO）; owner *client }`。模式判定在 Attach 升档时一次完成：

| ticket 绑定 mode | --writable | write-policy | owner 在位？ | → 生效 mode | 入递补队列？ |
|---|---|---|---|---|---|
| rw | ✓ | owner | 无 | **rw（成为 owner）** | —（本人即 owner） |
| rw | ✓ | owner | 有 | ro（D-07 降级） | ✓（FIFO 尾） |
| rw | ✓ | all | — | rw | —（all 模式无递补概念） |
| ro | ✓ | 任意 | — | ro | ✗（ro ticket 永不递补，D-06"以 rw 身份"） |
| 任意 | ✗ | — | — | ro（D-05 总闸） | ✗ |

owner 断线 → 取 `order` 中首个 rwEligible 且仍在线者 → 升格：outbox 入队 `Welcome{mode:"rw", prefs: rwVariant}`（D-13：升格即获 rw 档 prefs 含 osc52）→ 仲裁参与集切换（新 owner 尺寸接管）→ 前端升格处理（Pattern 9）。
**client 结构字段：** conn / remote / mode / rwEligible / attachSeq / outbox / done(cancel) / limiter / creditBlocked。**锁纪律：** 注册表+信用门共用 hubMu；outbox 自有 mu；锁序 hubMu > outboxMu（writer drain 完才取 hubMu 做恢复判定，绝不反序同持）。

### Pattern 6: 分享 token 门禁与 /s/{token}/ 路由装配（GOROOT 核实）

**路由装配（Go 1.22+ mux 通配语义，GOROOT 1.26.3 源码核实）：**

```go
mux.Handle("GET /s/{token}/", s.sharePage)          // 页面门禁
mux.HandleFunc("/s/{token}/", func(w, r) {           // P3 /api/attach 同款 405 fallback 纪律：
	w.Header().Set("Allow", http.MethodGet)          // 方法模式内建 405 会被 "/" 子树吞掉
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
})
```

**核实的 mux 行为（planning 必须知晓的三个坑）：**
1. 模式 `/s/{token}/` 的尾斜杠 = 匿名多段通配（`{...}`）：除 `/s/abc/`（精确匹配）外还匹配 `/s/abc/任意/深度`（非精确匹配）。pattern.go 段语义逐字："`{x}` => segment{s: \"x\", wild: true}" 单段通配；"If both wild and multi are true, it matches all remaining path segments." [VERIFIED: GOROOT go1.26.3 src/net/http/pattern.go:48-55]。
2. `GET /s/abc`（无尾斜杠）→ **301 重定向到 `/s/abc/`**：`matchOrRedirect` 对非精确匹配且不以 / 结尾的路径追加 / 重匹配，命中精确则重定向 [VERIFIED: GOROOT src/net/http/server.go:2721-2743 函数体 + exactMatch 注释块 2749-2780]。token 出现在 Location 头——D-03 已接受的反代日志暴露面同例。
3. 方法模式的内建 405 仅在"无任何其它模式匹配"时触发，会被 `/` 子树吞掉（P3 已踩，server.go:192-198 注释同款）——必须显式注册 path-only 405 fallback。
4. 取值：`r.PathValue("token")` [VERIFIED: GOROOT src/net/http/request.go:1469 `func (r *Request) PathValue(name string) string`]。

**页面伺服：** 无效/缺席 token → **委托既有 `/` 处理链**（凭据模式 `basicAuth(wh)` → 401 challenge；无认证模式 `wh` 直接给页——此时全站本就无门，D-01"无/错 token 时 P3 Basic 矩阵不变"的字面落地）。有效 token → 将 `r.URL.Path` 改写为 `/` 后调既有 embed handler（保留 gzip 旁路与 Vary 头语义，web/embed.go:33-52 形态复用）；dist 为单文件全内联 index.html（本 session 核实：`web/dist/` 仅 index.html + index.html.gz，无任何外部 src/href 引用），`/s/{token}/` 路径下无相对资源解析问题。

**token store（裁断 R-04）：** 仅两条目（ro/rw），不用 map——`struct{ ro, rw [sha256.Size]byte }` 启动时生成即预哈希；校验走 `matchCredential` 同款位或累积不短路（auth.go:56-65 的 planner-erratum 修正形态 `matched |=`），返回命中的 mode。生成复用 tickets.go:45-49 形态（crypto/rand 16B → base64.RawURLEncoding 22 字符 [VERIFIED: internal/server/tickets.go:45-49]）。**token 值永不入 logEvent 参数（D-03 红线延伸）；启动打印是产品行为不是日志（MULTI-05 明确授权）。**

### Pattern 7: 启动打印分享链接与出站 IP 回填

**D-04 落地（裁断 R-05）：**
```go
// bind 为 0.0.0.0/:: 时的 host 回填：UDP-dial 技巧优先（路由表感知），接口扫描兜底
func outboundIPv4() string {
	// UDP dial 不产生任何流量（无握手），仅让内核按路由表选出站接口的本地地址
	if conn, err := net.Dial("udp", "192.0.2.1:80"); err == nil { // RFC 5737 TEST-NET-1，永不真实路由
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP.To4() != nil {
			return addr.IP.String()
		}
	}
	// fallback：net.Interfaces() 索引序，首个 up && !loopback 接口的首个 IPv4
	...
}
```
**实证（本机，2026-08-19）：** UDP-dial 返回 `9.134.229.124`（eth1，正确主接口）；索引序扫描同机亦返回 eth1，但 docker0(172.17.0.1)/br-*(172.18.0.1/192.168.64.1) 紧随其后——docker 先于主接口创建的机器上朴素扫描必中 docker0（错误地址）。UDP-dial 由路由表驱动，结构性避开容器桥接口。**打印形态（planner 级建议）：** `listening on ...` 行不动，追加两行：
```
share read-only:  http://9.134.229.124:7681/s/<ro-token>/
share read-write: http://9.134.229.124:7681/s/<rw-token>/   # 仅 --writable 时打印（D-05）
```
具体 bind 地址原样使用（D-04）；loopback bind 打印 loopback（链接本机自用）；端口取 `ln.Addr()`（--port 0 随机端口既有 D-06 形态）；scheme 随 TLS 分岔（https://）。全失败兜底打印 0.0.0.0 原样（不阻断启动）。

### Pattern 8: CR-01 完整背压修复（输入方向）

**What:** 每客户端 `rate.Limiter` + 会话级单输入队列（字节有界）+ 单 input-writer goroutine（独占 `Master.Write`）。
**Why now:** PROJECT.md Key Decisions 锁定本 phase 落地；本 session 核实 Phase 2 的最小缓解（O_NONBLOCK）**从未执行**——`server.go:474` 仍是 Attach 读循环内同步 `s.sess.Master.Write(data[1:])`（当前全仓无 `O_NONBLOCK`/`ErrWouldBlock` 任何踪迹 [VERIFIED: 全仓 grep 零命中]）。完整修复落地后 master fd **保持默认阻塞模式**——阻塞被关进专属 goroutine，队列有界 + 丢弃即背压；Drain→Close 解除在途写阻塞（与 Read 同机制，runtime poller）。
**输入路径：** Attach 读循环 INPUT 帧 → mode 门（ro 静默丢，既有）→ `limiter.AllowN(time.Now(), len(data)-1)` → false 静默丢（裁断 R-02）→ 输入队列 tryEnqueue（满则丢 + 计数器，Phase 8 进 metrics）→ writer 顺序写 master。
**多写者交错不做排序承诺**（ARCHITECTURE §2.9 screen 同款语义，all 模式文档明示）。

### Pattern 9: 前端升格与连接流程改造

**升格 Welcome 复用 'W' 帧（裁断 R-09）：** 不新增类型字节（P2 D-01/D-02 纪律：既有帧类型的运行期再推送 + 加字段均不算动协议；旧前端对未知时序 Welcome 的处理是再应用一次 mode，无害）。前端 WELCOME 分支补 **rw 分支**（现状只有 `if (w.mode === 'ro')` 单分支 [VERIFIED: web/src/main.ts:391-397]）：`isRO=false; term.options.disableStdin=false; setTitle()`（去 `[ro] ` 前缀，经单一写口）+ `fit.fit()`（触发 onResize→sendResize，自动纠正排队期间可能过期的尺寸——owner 尺寸接管的前端半侧）。addEventListener 同参重复注册幂等（DOM 规范去重），beforeunload 重注册无泄漏。
**连接流程：** `location.pathname` 匹配 `^/s/([^/]+)/$` 提取 token → `fetch('/api/attach', {method:'POST', body: JSON.stringify({token})})` → 后续 ticket→Hello 链路不变。ro 形态 `sendResize` 加 `if (isRO) return` 门（D-09 第一闸；Hello 携首尺寸不受影响——helloSent 门先于 isRO 生效）。
**1013 文案（D-10）：** onclose 1013 分支更新为"slow consumer 被断开 + 手动刷新"英文语义（现状占位文案 "The server asked this client to retry later." [VERIFIED: web/src/main.ts:581-587]），维持 showStatus + "Reload this page" 链接形态，不做任何自动重连。
**409 相关文案清理：** 现状 `onerror`/未 opened onclose 与 fetch catch 三处文案含 "another client is already attached (wesh currently allows a single client)" [VERIFIED: web/src/main.ts:365-371, 519-525, 544-549]——多客户端后事实错误，必须改写（可并入"server unreachable or at capacity"语义；若 /api/attach 加 503 早闸可给"Server is full"专版，见 Open Question 2）。

### Anti-Patterns to Avoid

- **hub 内联 `c.Close()` 踢出：** 最长 ~10s 阻塞 ReadLoop，把要消灭的行头阻塞换个地方复活。必须 `go c.Close(1013, reason)`（Close 幂等且解除全部 goroutine 阻塞，见 Pattern 1 引用）。
- **踢出前丢帧"宽限"（strikes 误用）：** 有序字节流丢一段转义序列 = 客户端画面静默损坏（ARCHITECTURE Anti-Pattern 3）。"宽限"只能由 outbox 深度实现，绝不能丢帧保连接。
- **每客户端帧计数 channel 当字节上界：** 帧大小 1B~32KiB 不均，内存上界失真（Pitfall 4 字节级纪律）。
- **为踢出/升格新增 WS 帧类型字节：** 违反 P2 D-01 类型空间经济；升格复用 'W'，踢出用标准关闭码 1013。
- **共享 token 存 map + TTL 清理：** 仅两条目、生命周期=进程，map/janitor 全是过度设计。
- **接口扫描取首个非 loopback 当唯一手段：** docker0/bridge 接口索引序抢先（本机实证），必须 UDP-dial 路由感知优先。
- **同尺寸 TIOCSWINSZ 当 SIGWINCH 用：** Linux 同尺寸不发信号（本机实证，Pattern 见 D-11 验证），必须显式 TIOCGPGRP+kill。
- **读路径加 deadline / 为输入方向保留同步 Master.Write：** Pitfall 2 与 CR-01 的老路，禁止回潮。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 输入限速令牌桶 | 自写时间窗/计数器 | `golang.org/x/time/rate` Limiter | 令牌桶边界条件（突发/零值/并发）已全处理；"Limiter is safe for simultaneous use by multiple goroutines" [VERIFIED: x/time@v0.10.0 rate.go:56-57] |
| token 生成与比较 | math/rand、`==` 比较 | crypto/rand + crypto/subtle（ticketStore/matchCredential 既有形态） | PITFALLS C6 锁定项；128bit + 常数时间 |
| WS 关闭帧构造 | 手写 close 帧字节 | `c.Close(1013, reason)` 库调用 | RFC6455 合规（mask/长度/125B reason 上限）库保证 |
| 路径通配解析 | 正则/手拆 `strings.Split(path,"/")` | ServeMux `{token}` 通配 + `r.PathValue` | GOROOT 内建语义（301/405/优先级）零代码获得 |
| 出站 IP 探测 | 解析 /proc/net/route、netlink | `net.Dial("udp", ...)` 路由感知技巧 + `net.Interfaces()` 兜底 | 跨平台 stdlib；零流量；本机实证 |
| TIOCGPGRP | 手填 ioctl 魔数 | `unix.IoctlGetInt(fd, unix.TIOCGPGRP)` | 双平台常量经 x/sys 核实（linux 0x540f / darwin 0x40047477） |
| 尺寸读回断言 | ioctl TIOCGWINSZ 手写 | `pty.Getsize` | 既有依赖现成 API [VERIFIED: creack/pty@v1.1.24/winsize.go:18] |
| resize 防抖 | 自写 goroutine 计时循环 | 单 `time.Timer` reset（AfterFunc 先例） | 既有代码同款（helloTimeout AfterFunc，server.go:378-386） |

**Key insight:** 本 phase 的全部原语（令牌桶、通配路由、ioctl、关闭握手、防抖）都有库/标准库/既有代码形态；真正的设计量在**拓扑与规则**（hub/信用/仲裁/递补），不在任何底层机制。

## 裁断建议（Claude's Discretion 逐项）

| # | 事项 | 建议裁决 | 依据 |
|---|------|----------|------|
| R-01 | **outbox 容量/水位/strikes 初值** | 容量 **512KiB/客户端**（字节有界）；resume 水位 **50%（256KiB）**；**strikes 不设**（有序流丢帧=画面静默损坏，宽限由深度实现；80% 观测水位留 Phase 8 metrics） | PITFALLS Pitfall 2 参考区间 1-4MB 是独立服务器语境；512KiB×32 客户端最坏 16MiB 账面（共享帧实占 ≈ 最慢者滞后量，见 Pattern 2）；100KB/s 慢链路约 5s 抖动容忍；持续失配本就该踢（重连从最新看起，D-domain 锁定）。Phase 9 标定回填 |
| R-02 | **输入限速超限行为** | **丢弃**（不断开）+ 计数器（Phase 8 进 metrics），不打逐次日志 | RES-02 是资源保护非策略违例：all 模式下激进粘贴的合法用户被踢是 UX 灾难；INPUT 丢弃是可见可恢复的（丢键用户自然放慢），不同于 OUTPUT 丢帧的静默损坏；ro INPUT 静默丢不立日志是既有先例（server.go:472）；断开会放大成重连风暴。x/time `Allow` godoc 逐字："Use this method if you intend to drop / skip events that exceed the rate limit." [VERIFIED: x/time@v0.10.0 rate.go:113-115] |
| R-03 | **token 校验失败计入 throttleStore？** | **计入，但基本零新代码**：/s/{token}/ 无效 token → 委托 basicAuth 链 → 401 + recordFail 自动发生（D-08 既有统一计数器，凭据模式）；/api/attach token 分支失败走同一 401 路径同计数器。有效 token 优先于 throttle 直接放行（capability 语义，避免 NAT 出口 IP 误伤持票旁观者）；128bit 空间使无节流枚举无意义（2^128，热寂级） | D-08 精神（一切认证失败同一 per-IP 计数器）；无 oracle（失败形态与 Basic 失败同文）；throttle.go 既有 `recordFail`/`retryAfter` 直接复用 [VERIFIED: internal/server/throttle.go:70-107] |
| R-04 | **首个非 loopback IPv4 选取策略** | **UDP-dial 路由感知优先**（`net.Dial("udp", "192.0.2.1:80")`，零流量，RFC 5737 文档地址）→ 失败 fallback `net.Interfaces()` 索引序首个 up 非 loopback 首个 IPv4 → 全失败打印 bind 原样 | 本机实证（Pattern 7）：朴素扫描在多 docker 桥接口机器上必中 docker0；UDP-dial 由路由表驱动结构性正确 |
| R-05 | **/s/{token}/ 装配与无效 token 页面形态** | `GET /s/{token}/` + path-only 405 fallback（P3 同款纪律）；有效 → 改写 `r.URL.Path="/"` 委托 embed handler；**无效 → 原样委托 `/` 链**（凭据模式 401 challenge/无认证模式给页），不加 Cache-Control 新头 | GOROOT mux 语义核实（Pattern 6 三坑）；D-01"无/错 token 时 P3 Basic 矩阵不变"的字面落地；零新响应形态 |
| R-06 | **max-clients 计数口径** | **注册成功后计数**（atomic int），半开不计入（halfOpenCounter 正交）；守卫区 ③ 位 503 用 atomic load 判定 | CONTEXT 倾向明示；容量策略非安全边界，并发握手竞态最坏瞬时超编 ≤ per-IP 半开帽（8），可接受且注释明示；acquire/release 双计数器是不必要的复杂度 |
| R-07 | **注册表结构与 hub 拓扑** | 单 `hubMu` 护注册表+信用门+仲裁器；`map[*client]struct{}` + `[]*client` FIFO + `owner` 指针；hub=ReadLoop 内 onChunk（零新 goroutine）；每客户端 writer+pinger 两 goroutine（pinger 既有 per-conn 模式） | Pattern 1/3/5；fan-out 临界区只含 trySend（非阻塞）故单锁足够；pinger 装配参照 server.go:550 既有先例 |
| R-08 | **全局信用机制** | Pattern 3 完整形态：cond 门 + 半水位恢复 + "ro 满即踢/rw 满看全体"分工表；不设独立信用超时（pinger 死锁免除证明已源码级闭合） | ARCHITECTURE §2.6 全局信用定义 + CONTEXT "全体可写客户端阻塞时停读 PTY" 字面的自洽实现 |
| R-09 | **递补升格通知通道** | **复用 'W' Welcome 运行期推送**（载荷 `{"mode":"rw","prefs":rw档}`）；前端补 rw 分支 | P2 D-01/D-02：新类型字节是新协议面，Welcome 再推送零协议成本；前端 disableStdin/标题切换落既有 ro 形态翻转 |
| R-10 | **1013 reason 与 logEvent 命名** | close reason = `slow_consumer`；logEvent：`logEvent(remote, websocket.StatusTryAgainLater, "slow_consumer")`；503 满员事件：`logEvent(remote, 503, "max_clients")`（HTTP 层 code 复用状态码既有裁决） | D-07 机器串纪律；既有命名族（hello_timeout/pong_timeout/message_too_big 同构 snake_case） |

**参数初值汇总（P2 D-10 常量纪律；Options 测试可覆写字段先例——HelloTimeout/MaxHalfOpenPerIP/PongTimeout 同款，server.go:85-86 注释形态）：**

| 参数 | 初值 | 论证 |
|------|------|------|
| outbox 字节容量/客户端 | 512KiB | 16×32KiB 读块；100KB/s 链路 ~5s 抖动容忍；32 客户端账面最坏 16MiB（共享帧实占更低） |
| outbox resume 水位 | 50% | 迟滞防门震颤；半满语义直白 |
| 输入限速 rate / burst | 32KiB/s / 64KiB | 人类击键 ~10B/s、快粘 ~50KB 瞬时（burst 内容纳一次 16KiB 满帧×4）；持续 32KiB/s 远超合法、远低于洪水 |
| 输入队列容量 | 256KiB | ≥16 个 16KiB 满帧（ReadLimitPostAuth）；满则丢（限速器在前，队列满本应罕见） |
| --max-clients 默认 | 32 | ARCHITECTURE §6 "10–100 连接=团队围观/教学"区间下沿；账面内存与 goroutine 开销微小；flag 可调 |
| resize 防抖 | 50ms（已锁定） | PITFALLS Pitfall 10 SIGWINCH 风暴防线 |
| 尺寸钳制 | [1,1000]（既有 ClampDim） | C10 已落地，零改动 |

## Common Pitfalls（本 phase 特有，区别于通用 PITFALLS.md）

### Pitfall P5-1: chunk 别名跨帧持有（ReadLoop 缓冲复用）
**What goes wrong:** hub 把 `onChunk(chunk)` 的 chunk 直接存进各 outbox，下一拍 Read 覆写底层缓冲 → 全部客户端收到花屏数据。
**Why it happens:** `pty/io.go:13-14` 明示"复用底层缓冲——回调方如需跨帧持有须自行拷贝"；单客户端时代 `onChunk` 拷进 `s.frame`（server.go:525-526），多客户端化时容易只搬结构不搬拷贝纪律。
**How to avoid:** hub 组帧一次（Pattern 1 的 `frame := make(...)`），outbox 只存该只读帧引用。测试防线：两客户端高压 fan-out + `-race` + 输出校验和一致性断言。
**Warning signs:** 仅在高压/多客户端下画面偶发损坏。

### Pitfall P5-2: hub 内联 Close 踢出（行头阻塞还魂）
**What goes wrong:** trySend 满 → hub 直接 `c.Close(1013,...)` → 对 stall 客户端最长 ~10s 阻塞（5s 写超时 + 5s 等对端关闭帧）→ ReadLoop 停摆 → 全员卡死，比改前更糟。
**How to avoid:** `go c.Close(...)`（Close 幂等、自界、解除全部 goroutine 阻塞——Pattern 1 逐字引用）；注册表移除与 `cancel()` 同步内联（非阻塞），Close 异步。
**Warning signs:** 慢客户端混沌测试里"其他客户端卡顿 ~10s"。

### Pitfall P5-3: 同尺寸 TIOCSWINSZ 不发 SIGWINCH（D-11 失效）
**What goes wrong:** attach 时仲裁尺寸未变（同尺寸新客）→ 内核不发 SIGWINCH → vim/htop 不重绘 → 新客黑屏，D-11 目标落空。
**实证（本机 Linux，2026-08-19 探针）：** 子进程 trap WINCH 落盘计数；Setsize 24x80→50x132（异尺寸）命中 1 次，随后两次 50x132→50x132（同尺寸）**零命中**。
**How to avoid:** D-11 显式 `TIOCGPGRP + kill(-pgid, SIGWINCH)` 无条件执行（与仲裁 resize 是否发生无关）；重复 SIGWINCH 无害（终端应用必须容忍伪信号）。全链路探针实证：`unix.IoctlGetInt(master.Fd(), unix.TIOCGPGRP)` 取到 fg pgid，`unix.Kill(-pgid, unix.SIGWINCH)` 后 trap 命中。
**darwin 注：** 常量与 API 同款存在（x/sys v0.47.0 darwin zerrors），同尺寸是否发信号未实证 [ASSUMED：BSD 系 tty 同为变化才发；即便误判，显式 SIGWINCH 只是冗余无害]。CI 有 darwin runner 覆盖。

### Pitfall P5-4: 既有 e2e 测试套件的单次语义依赖（迁移面）
**What goes wrong:** Phase 1-3 的 e2e 测试全部建立在"客户端断开 → SIGHUP + exitf(0)"上——本 phase 必然推论将其终结，测试成片变红。
**逐文件核实（本 session grep）：** `e2e_test.go` 中 `waitExit` 收口断言 ≥8 处（TestEchoPTY 断开断言、TestSecondClient409 整测作废、SIGHUP 落盘标记两测、退出码传递等）[VERIFIED: internal/server/e2e_test.go:68-76, 326-371, 432-462 等]。
**How to avoid:** planner 必须单列 **Wave 0 测试迁移任务**：断开不再 exitf（改为注册表移除断言）；409 测替换为双 attach 成功 + 第三人 503；SIGHUP 测删除（路径消亡）；退出码传递改为"子进程退出 → 广播 1000 → exitf(code)"新形态。UAT 脚本 phase02/03.mjs 的"单次语义独立 spawn"纪律可放松但不必改；其生命周期断言必须改。

### Pitfall P5-5: 守卫区 503 闸顺序与 halfOpen 泄漏
**What goes wrong:** 503 闸放在 halfOpen acquire 之前 → 满员时攻击者仍可占半开名额；或 503 拒绝后忘 release → halfOpen 计数泄漏（Pitfall 4 单调增长）。
**How to avoid:** 沿用既有顺序 ⓪Origin→①子协议→②halfOpen 429→③503（原 409 位）；③ 拒绝路径 `release()` 恰好一次（02-03 局部 sync.Once + defer 兜底先例，server.go:326-334 形态）。

### Pitfall P5-6: 升格 Welcome 与 prefs 分化的时序
**What goes wrong:** ro 旁观者 attach 时拿到无 osc52 的 prefs；升格 rw 后若不重发 prefs，其 osc52 永远不开（D-13 的另一半落空）；反之若 attach 时给了全量 prefs 则旁观者 osc52 被开（D-13 失守）。
**How to avoid:** 装配期产两档 prefs blob（含/不含 osc52——osc52 是服务端专有键，main.go aggregateClientPrefs 产双变体，保持服务端不透明透传纪律：不做运行期 JSON 手术）；attach Welcome 按 mode 选档；升格 Welcome 携 rw 档。前端 WELCOME 既有 prefs 应用段幂等（queryKeys 跳过机制）可重入。

### Pitfall P5-7: 信用门死锁（全体等待、无人唤醒）
**What goes wrong:** 门闭合期间可写端全部 TCP 假死（合盖），无 drain 事件 → Broadcast 永不发生 → ReadLoop 永久停摆。
**How to avoid:** pinger 是唯一必需的唤醒源——`mu.lock(ctx)` 超时返回包装 `context.DeadlineExceeded` [VERIFIED: conn.go:286-292]，pinger 既有精确分类（02-04：仅 DeadlineExceeded → pong_timeout + CloseNow）≤(interval+5s) 杀死假死连接 → detach 路径必须 `Broadcast` 重估门。detach/kick/attach/子进程退出全部路径统一 Broadcast。验证：Go 测试"全可写端 stall → 门闭合 → 一客户端 CloseNow → 门开"序列。

## Code Examples

### 分享 token 校验（matchCredential 同款位或形态）
```go
// Source: internal/server/auth.go:56-65 形态复用（planner erratum 修正后的 |= 形态）
type shareTokens struct{ ro, rw [sha256.Size]byte } // 启动生成即预哈希（SHA-256 等长化，SEC-01 长度侧信道纪律）

func (st *shareTokens) lookup(token string) (mode string, ok bool) {
	h := sha256.Sum256([]byte(token))
	matched := proto.ModeRO
	matchedInt := 0
	matchedInt |= subtle.ConstantTimeCompare(h[:], st.ro[:]) // 不短路：耗时与命中哪个 token 正交
	if subtle.ConstantTimeCompare(h[:], st.rw[:]) == 1 {
		return proto.ModeRW, true
	}
	if matchedInt == 1 {
		return matched, true
	}
	return "", false
}
```
（注：两组目标无需组序号防泄露——ro/rw 是公开语义；位或累积防"第几位命中"时序泄露的形态与 auth.go 同源。planner 可微调命名。）

### 输入限速门（x/time/rate，模块缓存源码核实签名）
```go
// Source: golang.org/x/time@v0.10.0/rate/rate.go:100-117 逐字签名：
//   func NewLimiter(r Limit, b int) *Limiter
//   func (lim *Limiter) Allow() bool
//   func (lim *Limiter) AllowN(t time.Time, n int) bool
// Attach 升档：c.limiter = rate.NewLimiter(rate.Limit(inputBytesPerSec), inputBurst)
// Attach 读循环 INPUT 分支：
case proto.Input:
	if c.mode == proto.ModeRO {
		continue // ro 静默丢（既有 P2 D-13/D-14，多客户端化为 per-client 判定）
	}
	if !c.limiter.AllowN(time.Now(), len(data)-1) {
		continue // R-02：超限丢弃，不断开不打日志（计数器 Phase 8）
	}
	s.inputQ.tryEnqueue(data[1:]) // 满则丢 + 计数（CR-01 完整修复）
```

### 1013 踢出（hub 内）
```go
// Source: Pattern 1/2；close.go:87-96 语义核实
func (s *Server) kickSlowConsumerLocked(c *client) {
	s.registry.removeLocked(c)      // 同步移除（map+slice 删除，非阻塞）
	c.cancel()                      // writer/pinger 终结（非阻塞）
	logEvent(c.remote, websocket.StatusTryAgainLater, "slow_consumer") // R-10 命名
	go func() {
		_ = c.conn.Close(websocket.StatusTryAgainLater, "slow_consumer") // 幂等自界；永不内联（P5-2）
	}()
}
```

### D-11 SIGWINCH 挂点（pty.Session 新方法，fdMu 纪律）
```go
// Source: 本机探针实证 + x/sys@v0.47.0 常量核实：
//   zerrors_linux_amd64.go:449  "TIOCGPGRP = 0x540f"
//   zerrors_darwin_amd64.go:1509 "TIOCGPGRP = 0x40047477"
//   ioctl_unsigned.go:58 "func IoctlGetInt(fd int, req uint) (int, error)"
// SignalForegroundGroup 向 PTY 前台进程组发 SIGWINCH（D-11 新客强制重绘）；
// TIOCGPGRP 失败/无前台进程组静默降级（D-11 授权）。fdMu 与 Resize/Close 互斥（既有纪律）。
func (s *Session) SignalForegroundGroup() {
	s.fdMu.Lock()
	defer s.fdMu.Unlock()
	if s.closed {
		return
	}
	pgid, err := unix.IoctlGetInt(int(s.Master.Fd()), unix.TIOCGPGRP)
	if err != nil || pgid <= 0 {
		return // 静默降级
	}
	_ = unix.Kill(-pgid, unix.SIGWINCH) // 负 pid = 进程组；失败静默
}
```

### 仲裁器核心（纯函数可单测）
```go
// Source: D-09 参与集矩阵 + MULTI-04 算法（≥2 min、2→1 last-wins）
func arbitrate(members []dims) dims { // members = 参与集最新上报尺寸（已 ClampDim 钳制）
	switch len(members) {
	case 0:
		return dims{} // 无参与者：不动 PTY（保持现状）
	case 1:
		return members[0] // last-wins
	default:
		out := members[0]
		for _, m := range members[1:] {
			out.cols = min(out.cols, m.cols)
			out.rows = min(out.rows, m.rows)
		}
		return out
	}
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 409 单客户端原子门（`s.attached` CAS） | 客户端注册表 + max-clients 503 闸 | 本 phase | server.go:61-62 的 `attached atomic.Bool`/`conn atomic.Pointer` 两字段整体退役，由注册表取代 |
| 客户端断开 → SIGHUP + exitf(0)（P1 D-11 单次语义） | 断开 = 注册表移除 + 递补/仲裁重算；子进程退出唯一终结（D-10） | 本 phase（CONTEXT 必然推论锁定） | `terminate(true, 0)` 路径消亡；测试迁移见 P5-4 |
| `onChunk` 单 conn 直写（`s.conn.Load()` + `c.Write(Background)`） | hub trySend + 每客户端 writer | 本 phase | onChunk 永不阻塞成为结构属性 |
| Attach 读循环同步 `Master.Write`（CR-01 已知缺陷） | 输入队列 + 专属 input writer goroutine | 本 phase | PROJECT.md 锁定项兑现；Phase 2 的 O_NONBLOCK 最小缓解从未落地且不再需要 |
| ARCHITECTURE §2.9 "owner 模式跟随 owner 尺寸" | D-09 参与集分层矩阵（owner 模式仅 owner 参与 ≠ "跟随"——全员 min 语义在 all 模式成立） | CONTEXT 已锁定 | 仲裁器按 Pattern 4 矩阵实现 |
| REQUIREMENTS MULTI-05 "含一次性 token 的分享链接" | D-02：token 可复用至重启，"一次性"在 ticket 上 | CONTEXT D-02（REQUIREMENTS 原文前期表述，以 CONTEXT 为准） | planner 勿按字面实现一次性链接 |

**Deprecated/outdated:**
- 无新增废弃；x/time/rate API 十年稳定；Go mux 通配自 1.22 成熟（项目 go 1.26.3）。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | darwin 同尺寸 TIOCSWINSZ 同样不发 SIGWINCH（Linux 已实证，darwin 未实证） | P5-3 | 低：误判后果仅是显式 SIGWINCH 冗余（无害）；CI darwin runner 可观测 |
| A2 | x/time v0.10.0→v0.15.0 的 rate API 无行为变化（未逐版本 diff） | Standard Stack | 极低：包历史极稳定；即便有变，go test 套件会立即暴露 |
| A3 | 纯 ro 会话运行期旁观者窗口缩放不上报（D-09 字面结果），画面裁剪风险由用户接受 | Pattern 4 | 低：D-09 已锁定，此处仅显式化后果；README 明示即可 |
| A4 | 无认证模式下分享链接仍应产生 ro/rw mode 绑定（/api/attach 接受 token body；Open Question 1） | Open Questions | 中：产品语义选择，CONTEXT 未覆盖该交叉面——若用户选"无认证不打印链接"，则 /s/ 路由在凭据模式才注册 |
| A5 | 并发握手竞态下 max-clients 瞬时超编可接受（R-06） | 裁断 R-06 | 低：容量策略非安全边界；超编幅度 ≤ per-IP 半开帽 8 |

## Open Questions (RESOLVED)

1. **无认证模式（--no-auth/loopback 裸跑）下分享链接的形态**
   - What we know: D-02 token 每轮启动无条件生成；MULTI-05 打印链接未限定凭据模式；D-01 以"与凭据共存"框架行文，未覆盖无认证交叉面；现状无认证模式 /api/attach 显式 404（前端探测信号，server.go:201-203）。
   - What's unclear: 无认证模式下 /api/attach 是否接受 token body 以兑现 ro/rw mode 绑定（ro 链接在无密码演示场景正是卖点）；若接受，前端探测逻辑变为"URL 携 /s/ token 时必走 fetch"。
   - Recommendation: 采纳"token 通道与认证模式正交"（无认证也打印链接、/s/ 门禁语义在无认证模式天然形同虚设但 mode 绑定有效）；/api/attach 在无认证模式仅当 body 携 token 时才非 404。请用户确认（影响 D-01 的适用范围解释）。
   - **Resolution（用户 2026-08-19 裁决）**：采纳 Recommendation——token 通道与认证模式正交；逐字记录于 05-UI-SPEC.md 锁定项对照表「Open Question 1」行（→ §Share Link Entry Contract）；服务端 /s/ 路由与 token 签发由 05-06 truths 兑现，前端携 token fetch 分派由 05-08 落地。

2. **/api/attach 是否加 503 容量早闸（UX 层）**
   - What we know: D-08 只锁定 /ws Accept 前 503；前端 fetch 阶段拿到 503 可给"Server is full"专版面，比 WS onerror 的通用"Unable to connect"更可操作。
   - What's unclear: 双层 503 的竞态（ticket 已签发但 WS 满员）语义无害但 UX 略绕。
   - Recommendation: 加（同一 atomic 计数读取，一处检查两处用）；若不加，前端改写通用文案覆盖容量场景即可。低风险，planner 可自决。
   - **Resolution（用户 2026-08-19 裁决）**：加——/api/attach 503 容量早闸；逐字记录于 05-UI-SPEC.md 锁定项对照表「Open Question 2」行（→ §Copywriting Contract C-2 "Server is full" 专版）；服务端早闸由 05-07 truths（attachHandler 双点位 503）兑现，前端 C-2 专版由 05-08 落地。

3. **信用门内"滴漏型"可写端的长时振荡**
   - What we know: Pattern 3 设计下滴漏读者每次 drain 开门、下一 chunk 可能再闭门，全体客户端随之抖动；无独立超时机制。
   - What's unclear: 多久算"该踢的滴漏"——需要真实负载数据。
   - Recommendation: 本 phase 接受（pinger 保底全死连接）；Phase 9 负载标定时评估加 dwell 计时器（连续信用阻塞 >30s 也踢）。
   - **Resolution**：本 phase 接受振荡——05-02 威胁登记 T-05-01c 已引用本条处置落地；Phase 9 负载标定时评估 dwell 计时器（连续信用阻塞 >30s 也踢）。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 工具链 | 全部后端改动 | ✓ | go1.26.3 linux/amd64 [VERIFIED: `go version` 本 session] | — |
| Node | UAT harness | ✓ | v24.13.0（≥22 原生 WebSocket/fetch，phase04.mjs 注释要求） | — |
| pnpm | 前端构建 | ✓ | 11.21.0（CI 同钉） | — |
| 网络（Go proxy） | `go get golang.org/x/time` | ✓（`go list -m -versions` 实测通） | proxy 最新 v0.15.0 | 模块缓存已有 x/time v0.10.0，断网可 `GOFLAGS=-mod=mod go get golang.org/x/time@v0.10.0` |
| golang.org/x/sys v0.47.0 | TIOCGPGRP/SIGWINCH | ✓ | 已在 go.sum（indirect） | — |
| coder/websocket v1.8.15 | 1013 常量/Close 语义 | ✓ | 模块缓存源码已核实 | — |
| darwin 验证 | 双平台 | ✓ | CI macos runner（Phase 1 起双平台） | 本机 Linux-only，darwin 行为经 CI |
| 浏览器/显示 | — | ✗（永久约束） | — | CODEBUDDY.md 分层测试策略（协议层 + headless + jsdom），无需浏览器 |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** 无浏览器 → 既定分层测试策略覆盖（非本 phase 新增缺口）。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `-race`（CI 强制）；UAT = Node 原生 WS 脚本（web/uat/phaseNN.mjs 零依赖传统）+ @xterm/headless 6.0.0 + jsdom 25.0.1（web/uat/package.json 既有依赖） |
| Config file | none（无 jest/vitest/pytest 配置；CI `.github/workflows/ci.yml`：`go test -race -count=1 -v ./...` + `pnpm -C web install --frozen-lockfile && pnpm -C web build`）[VERIFIED: ci.yml 本 session 读取] |
| Quick run command | `go test -race -count=1 ./internal/server/` |
| Full suite command | `go test -race -count=1 ./... && time pnpm -C web build && node web/uat/phase05.mjs && node web/uat/phase02.mjs && node web/uat/phase03.mjs && node web/uat/phase04.mjs`（UAT 需先 `go build -o /tmp/wesh-uat/wesh ./cmd/wesh`，harness 既定约定） |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MULTI-01 | 双客户端 attach 收到同一 OUTPUT 流 | integration | `go test -race -run TestMultiClientFanout ./internal/server/` | ❌ Wave 0（`multi_test.go` 新文件；dialHello 双客户端变体复用 e2e_test.go:133 形态） |
| MULTI-02 | owner/all 矩阵：Welcome mode、per-client INPUT 门、递补升格 | integration | `go test -race -run 'TestOwnerPolicy|TestAllPolicy|TestSuccession' ./internal/server/` | ❌ Wave 0 |
| MULTI-03 | stall 客户端 1013 被踢（reason=slow_consumer），其他客户端无卡顿，ReadLoop 不阻塞 | integration（测试客户端建连后不 Read 制造真 stall；输出洪水经子进程 `yes`/`seq` 产生） | `go test -race -run TestSlowConsumerKick ./internal/server/` | ❌ Wave 0（Options.OutboxBytes 调小加速触发，测试覆写先例） |
| MULTI-04 | 异尺寸双客户端 min-rect；2→1 恢复 last-wins；50ms 防抖合并 | unit（arbitrate 纯函数）+ integration（`pty.Getsize` 读回断言 [VERIFIED: winsize.go:18]） | `go test -race -run 'TestArbitrate|TestResizeArbitration' ./internal/server/` | ❌ Wave 0 |
| MULTI-05 | 启动打印两条 /s/{token}/ 链接；token GET 200（无 Basic）；错 token → Basic 矩阵；token→/api/attach→ticket→attach 全链 mode 正确 | UAT（phase05.mjs stdout 解析 + fetch + WS）+ Go handler 单测 | `node web/uat/phase05.mjs` + `go test -race -run TestShareToken ./internal/server/` | ❌ Wave 0 |
| RES-02 | INPUT 洪水超限被丢弃且连接存活、未超限部分送达 | integration | `go test -race -run TestInputRateLimit ./internal/server/` | ❌ Wave 0（Options 覆写 rate/burst 加速） |
| RES-03 | max-clients 满员 → /ws Accept 前 HTTP 503；halfOpen 计数不泄漏 | integration | `go test -race -run TestMaxClients503 ./internal/server/` | ❌ Wave 0 |
| RES-04 | 全体可写端 stall → 信用门闭合（子进程输出暂停可观测）；一端恢复/死亡 → 门开 | integration | `go test -race -run TestGlobalCredit ./internal/server/` | ❌ Wave 0 |
| 生命周期改造 | 客户端断开服务不退出（exitf 不被调用）；子进程退出 → 全员 1000 → exitf(code) | integration（P5-4 迁移） | `go test -race -run 'TestDetach|TestExitBroadcast' ./internal/server/` | ❌ Wave 0（e2e_test.go 改造） |

### Sampling Rate
- **Per task commit:** `go test -race -count=1 ./internal/server/ ./internal/proto/ ./internal/pty/`（秒级）
- **Per wave merge:** full suite（上表命令全链）
- **Phase gate:** 全量绿 + phase02/03/04.mjs 回归适配后全过 + phase05.mjs 全过，再进 `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/server/e2e_test.go` 单次语义迁移（P5-4：≥8 处 waitExit 断言改写、TestSecondClient409 替换、SIGHUP 两测删除）——**最高优先，阻塞一切新测试**
- [ ] `internal/server/multi_test.go`——双客户端 dialHello 变体（复用 e2e_test.go:133/177 形态）+ fan-out/权限/递补测试组
- [ ] `internal/server/slowclient_test.go`——stall 客户端夹具（建连后不 Read）+ 输出洪水子进程夹具
- [ ] `internal/server/resize_arb_test.go`——arbitrate 纯函数表测 + Getsize 集成断言
- [ ] `internal/server/sharetoken_test.go`——token store subtle 比较 + /s/ 路由门禁 + /api/attach token 分支
- [ ] `web/uat/phase05.mjs`——phase04.mjs 骨架复用（startWesh/spawnExpectExit/dialHello/check 形态）
- [ ] `server.Options` 测试覆写字段扩展（OutboxBytes/MaxClients/InputRate/InputBurst/ResizeDebounce——HelloTimeout 先例 [VERIFIED: server.go:90-103 Options 结构]）
- [ ] 前端 dist 重建（`time pnpm -C web build`，构建后验证产物时间戳——项目约束）

## Security Domain

### Applicable ASVS Categories（Level 1 基线，config security_asvs_level=1）

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | 分享 token = crypto/rand 128bit + SHA-256 等长 + subtle 位或比较（ticketStore/matchCredential 既有形态）；失败计入 D-08 统一 per-IP 退避计数器 |
| V3 Session Management | partial | token 吊销语义 = 进程重启（D-02 明示）；ticket 单次使用 60s TTL（既有 SEC-02 不变） |
| V4 Access Control | yes | per-client mode 门（ro 丢 INPUT/忽略 RESIZE）；owner 递补 FIFO；write-policy 默认 owner（安全默认）；D-13 旁观端 osc52 强制不下发 |
| V5 Input Validation | yes | 既有 ClampDim [1,1000]、ReadLimit 两档、Hello/RESIZE JSON 解码失败静默丢——零改动沿用；/s/{token}/ 单段通配天然限长（22 字符 base64url） |
| V6 Cryptography | yes | 永不手写：crypto/rand（token）、crypto/subtle（比较）、SHA-256（等长化）——全部既有纪律复用 |

### Known Threat Patterns for Go WS 多客户端 fan-out

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 慢/死客户端拖死全员（行头阻塞） | Denial of Service | Pattern 1/2/3：trySend + 1013 踢出 + 信用门；hub 永不内联 Close（P5-2） |
| 输入洪水（all 模式恶意协作者/脚本） | Denial of Service | per-client x/time/rate 丢弃（R-02）+ 有界输入队列 + ReadLimit 16KiB 帧顶（既有） |
| 连接数耗尽 | Denial of Service | --max-clients 503（RES-03）+ per-IP halfOpen 429（既有 SEC-08）——两闸正交（R-06） |
| 分享 token 在线爆破 | Information Disclosure / Elevation | 128bit 空间（2^128 枚举无意义）+ 失败入统一退避（R-03）+ token 永不入日志（D-03） |
| token 经反代访问日志泄露 | Information Disclosure | D-03 已接受的取舍；README 必须明示反代日志脱敏建议（CONTEXT specifics 锁定） |
| OSC52 劫持旁观者剪贴板 | Tampering / Info Disclosure | D-13：ro 端 prefs 不含 osc52（服务端双档 blob，Pattern 9/P5-6） |
| owner 断线后的权限真空/抢夺 | Elevation of Privilege | D-06 FIFO 递补（attach 顺序），无"先抢先得"竞态；升格经服务端 Welcome 推送非客户端请求 |
| 1013 踢出伪造引导钓鱼 | Spoofing | close reason 静态机器串（库 125B 上限），前端文案硬编码不渲染 reason 内容（onclose 只认 code 既有纪律） |

## Sources

### Primary (HIGH confidence)
- GOROOT go1.26.3 源码直读（本 session）：`net/http/pattern.go:48-55`（段通配语义）、`net/http/server.go:2721-2780`（matchOrRedirect/exactMatch，301 行为）、`net/http/request.go:1469`（PathValue）——/s/{token}/ 装配三坑
- coder/websocket v1.8.15 模块缓存源码直读：`close.go:52`（StatusTryAgainLater=1013）、`close.go:86-96`（Close 5s+5s/幂等/unblock 语义）、`conn.go:286-292`（mu.lock ctx 超时包装 DeadlineExceeded）、`write.go:318-320`（setupWriteTimeout→SetWriteDeadline）
- golang.org/x/time v0.10.0 模块缓存源码直读：`rate.go:100-117`（NewLimiter/Allow/AllowN 签名与 drop 语义 godoc）、`rate.go:56-57`（goroutine-safe）
- golang.org/x/sys v0.47.0：`zerrors_linux_amd64.go:449` / `zerrors_darwin_amd64.go:1509`（TIOCGPGRP）、`ioctl_unsigned.go:58`（IoctlGetInt）
- 本机实证探针（2026-08-19）：同尺寸 TIOCSWINSZ 不发 SIGWINCH（3 次 Setsize 1 次命中）；TIOCGPGRP+kill(-pgid,SIGWINCH) 全链路命中；UDP-dial 选出站 IP 避开 docker0/bridge
- 仓内源码直读（本 session，行号随文标注）：server.go / proto.go / tickets.go / throttle.go / auth.go / main.go / pty io.go+spawn.go / web main.ts / embed.go / e2e_test.go
- `.planning/research/ARCHITECTURE.md` §2.5/2.6（fan-out/背压，前序 HIGH 调研）与 `.planning/research/PITFALLS.md`（Pitfall 2/4/10、C5、C10）

### Secondary (MEDIUM confidence)
- `go list -m -versions golang.org/x/time`（proxy 实测至 v0.15.0）——版本新鲜度
- ARCHITECTURE §6 规模表（10–100 连接区间）——max-clients 默认 32 的定位依据

### Tertiary (LOW confidence)
- darwin 同尺寸 TIOCSWINSZ 行为（A1，Linux 实证外推）
- x/time v0.10.0→v0.15.0 API 稳定性（A2，未逐版本 diff）
- WebSearch（outbox  sizing 通用文章检索返回均为低质内容，未采用；本研究 sizing 论证全部基于仓内调研 + 一阶推算）

**Context7 注：** 本月配额耗尽（"Monthly quota exceeded"），docs 类问题全部改由 GOROOT/模块缓存源码直读完成——对本 phase 的问题面（Go 标准库与已钉版依赖的精确语义）而言实为更高权威来源。

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH——零新核心库；x/time 官方命名空间 + proxy 实测；全部 API 签名经模块缓存源码核实
- Architecture: HIGH——fan-out/outbox/信用/仲裁四件套有 ARCHITECTURE.md 前序 HIGH 调研 + 本 session 库源码级机制验证（Close 语义/pinger 死锁免除/chunk 别名）双保险
- Pitfalls: HIGH——P5-1..P5-7 全部经仓内源码或本机实证；darwin 细节 LOW 已隔离进 Assumptions
- 参数初值： MEDIUM——论证充分但本质需负载标定（CONTEXT 已挂 Phase 9 回填纪律，初值仅求"合理不离谱"）

**Research date:** 2026-08-19
**Valid until:** 2026-09-18（依赖全为稳定面：Go stdlib/钉版库/官方 x 包；30 天保守有效）
