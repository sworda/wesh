# Phase 6: 会话生命周期与重连 - Research

**Researched:** 2026-08-23
**Domain:** WS 会话生命周期（退出模式/类型化终结帧）+ 浏览器端断线重连（Go 1.26.3 服务端 + TypeScript/xterm.js 6 前端）
**Confidence:** HIGH（核心机制全部本 session 源码级核实：GOROOT、在库依赖源码、git 历史、现状代码逐行；外部浏览器语义 CITED MDN/ttyd 官方文档）

## Summary

本 phase 四个需求全部落在**既有架构的既有挂点**上，零新依赖、零新 goroutine 拓扑、零新锁。CONTEXT.md 的 14 条决策已与现状代码逐条对齐核实：EXIT 帧落 `proto.go` 已预留的 `'X'` 类型字节（proto.go:32 注释本 session 逐字核实）；广播序列挂 `lifecycle()` 的 Drain 后、并行 Close 前（server.go:955-995 逐行核实）；`--once` 语法糖展开复用既有 503 双闸（server.go:422-426 / 566-571）；`--exit-when-empty` 宽限计时器挂 `detach`/`kickSlowConsumerLocked` 两移除点（clients.go:677-696 / 491-513）；SIGHUP 进程组收口复用 Phase 1 被拆除的旧实现形态（git 历史 cc03c79~1 逐字恢复：`syscall.Kill(-sess.Cmd.Process.Pid, syscall.SIGHUP)`）；前端重连复用已验证可重入的 `connect()` 入口（main.ts:390-757，IN-01 per-connection 重置块 391-404 注释明示"Phase 6 自动重连落地前提"）。

研究发现的**三个计划期必须处理的机制细节**（全部源码级实证）：① EXIT 帧与 1000 关闭帧的**写序竞态**——若 EXIT 走 outbox 异步入队，并行 Close 的关闭帧可能超车，客户端收 1000 却丢了退出码；正解 = 每客户端 goroutine 内同步 `Write(EXIT)` 后接 `Close(1000)`（库帧级串行化保序，close.go 5s+5s 上界已核实）。② `syscall.Signal.String()` 返回的是 `"hangup"` 式小写描述词而非 `"SIGHUP"`（GOROOT zerrors 表逐字核实）——D-09 的 "killed by signal SIGHUP" 文案需要显式映射或大写名表，不能裸用 String()。③ SIGHUP 致死的子进程 `ExitCode() = -1`（GOROOT exec_posix.go:155-157 逐字核实），`--once` 断开退出路径的进程退出状态将经 `os.Exit(-1)` 截断为 255（Phase 1 旧语义为 exit 0）——进 Open Questions 待裁决。

**Primary recommendation:** 按计划拆四条主线——(1) proto+ lifecycle EXIT 帧；(2) pty SIGHUP 方法 + registry 空触发 + 宽限计时器；(3) CLI 两 flag + 校验矩阵；(4) 前端重连状态机（`ev.code === 1006` 显式触发、代际守卫防陈旧 socket、WELCOME 到达即退避清零 + `term.clear()`）——每条都有前序 phase 逐字先例可照抄。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**自动重连策略（CORE-05）**
- **D-01:** 触发范围 = **仅 1006 类无码异常断开**（断网/TCP 断开/pong 超时服务端 CloseNow 后前端所见的无码关闭）自动重连；1000/1008/1009/1011 不自动重连（明确终结/策略/错误语义各自有面板）；**1013 维持 P5 D-10 手动刷新**——被踢说明消费跟不上，自动重连只会再被踢，后台标签页会循环放大流量（P5 边界纪律本 phase 确认保持）
- **D-02:** 退避参数 = **1s×2 封顶 30s 无限重试**（throttleStore 同族参数族，P3 D-08 形态延伸）+ 面板「立即重试」按钮跳过当前等待（ROADMAP「手动入口」落此）；无尝试次数上限——个人运维「标签页放着，回来已接回」是主场景，30s 一次重试流量可忽略；重连成功（WELCOME 到达）退避清零
- **D-03:** 重连 UI = **复用 showStatus 全屏三态面板**：标题 Reconnecting，正文显示 attempt N / 下次重试倒计时，hint 处放可点「Reconnect now」——零新 UI 组件（P5 D-07 哲学）；顶部状态条记 deferred

**断线检测与首屏恢复（CORE-05）**
- **D-04:** 断线检测 = **浏览器 online/offline 事件 + onclose 双触发**：offline 立即启动重连循环、online 立即试一次——OS 级网络断开/恢复秒级感知，零协议改动。不引应用层心跳帧：浏览器 WS API 不暴露 ping/pong（CORE-06 的 5s ping 前端不可见）、空闲终端无 OUTPUT 流量，「多久没收到消息」判据在浏览器侧结构性不成立。黑洞场景（无 RST 无事件）退化为 TCP 超时后重连——风险接受
- **D-05:** 重连成功首屏 = **term.clear() 清屏 + 服务端复用 SIGWINCH 强制重绘**（P5 D-11 挂点延伸到重连 attach 路径）——全屏程序秒级重绘干净画面；行内 shell 历史交 tmux/herdr（ROADMAP 既定分工）。不保留旧 buffer：重连窗口期错过的输出形成断层，全屏程序增量重绘花屏（G-05-1 同类风险）
- **D-06:** owner 重连**不加豁免**——按新 attach 走 P5 D-06/D-07 既定递补语义（原 owner 降级 ro 入队），文档明示「重连不恢复写权限」。CORE-05 承诺边界刻意收窄 = 接回同一进程、输入输出一致；不含身份恢复（恢复窗口需身份暂存/倒计时/双 owner 交接新状态机，与 P5 递补确定性冲突）
- **D-07:** 服务端重启场景 = **自然行为 + 文档明示**：share token 重启即废（P5 D-02）→ attach 失败落手动面板等用户拿新链接；凭据模式重连成功接回的是全新 shell。README 明示「重连目标 = 同一 URL 的当前进程，服务端重启后是全新会话」。不引入会话代际标识（generation id）

**子进程退出终结帧（SESS-03）**
- **D-08:** EXIT 帧 = **新 S→C 类型字节**（不复用 'E' Error 帧——用户裁决：终结语义独立于错误语义，类型字节承载）—— **Reversibility:** one-way — 类型字节是前后端公开协议契约（P2 D-01 纪律），发布后改值/改义破坏全部已部署客户端
- **D-09:** 载荷 = **`{"exit_code": N, "message": 人话}`**——exit_code 结构化供测试断言，message 前端直显；信号死亡 exit_code=-1、message 含信号名（服务端组文案唯一写口，前端不自维护信号文案表）—— **Reversibility:** one-way — 载荷形状同上公开契约（P2 D-02 加字段纪律约束后续演进）
- **D-10:** 广播序列 = **EXIT 帧 → 1000 正常关闭**（ROADMAP 锁定）；前端 EXIT 帧暂存（lastError 同款通道）→ onclose 1000 → 「Session ended」正文显示 message——面板结构不变，退出码/信号名人话进正文
- **D-11:** 重连循环遇服务端已退出的收口 = **Reconnecting 面板 hint 文案明示「若服务端已退出请从 shell 重启」**——零新逻辑；前端无法区分断网 vs 服务端退出（浏览器 connect 失败不暴露 refused/timeout 差异），两场景同一面板通吃

**--once 与无人退出模式（SESS-01/02）**
- **D-12:** `--once` ≡ `--max-clients=1 --exit-when-empty=0` **语法糖**：CLI 保留独立 --once flag（ttyd 肌肉记忆），README 标明等价关系；第二客户端拒绝走既有 503 计数路径（P5 D-08 守卫链零新分支，409 不复活）—— **Reversibility:** one-way — CLI flag 公开契约（P2 D-15 纪律）
- **D-13:** 断开退出统一收口路径 = 注册表空（--once 唯一客户端断开 / --exit-when-empty 宽限到期仍空）→ **SIGHUP 进程组（P1 D-11 语义复活）** → Drain → exitf 以子进程退出码收口——exitf + sync.Once 单一收口纪律保持，两模式零分支差异；Phase 7 OPS-04 的信号可配化在此之前不提前设计
- **D-14:** SESS-02 flag = **`--exit-when-empty[=duration]` 单 flag 可选值**：裸写 = 最后一个客户端断开立即退出；`=duration` 给重连宽限（计时内任一端 attach 成功则取消退出）——Go flag 自定义 Value + IsBoolFlag 惯例；默认不开启（现状保持：无客户端时子进程继续运行，P5 推论）—— **Reversibility:** one-way — CLI flag 公开契约

### Claude's Discretion
- EXIT 帧类型字节具体值（建议 'X'，避开已占位 'T'/'P'；proto.go 常量 + 前后端注释手工对齐纪律沿用）
- EXIT 帧广播与慢客户端 outbox 的写序（lifecycle 现有 hubMu 快照 + 并行 Close 模式延伸；trySend 失败即走既有收口——进程已退出场景无需保帧）
- message 文案具体措辞（英文；exit code N / killed by signal SIGHUP；信号名提取经 ExitError.Sys().WaitStatus；非 ExitError 形态（Wait 返回其他错误）的 message 处理）
- online/offline 与 onclose 双触发的幂等（重连循环单例、防双循环；重连成功判定 = WELCOME 到达后退避清零）
- --exit-when-empty 宽限计时器挂点（detach 致注册表空 → 启动 timer；attach → 取消；timer 随会话消亡，零新 exitf 分支纪律）
- Reconnecting 面板 attempt 计数/倒计时格式（showStatus 三态内既有结构承载）
- UAT 场景矩阵（phase06.mjs：断线重连/清屏重绘/EXIT 帧退出码与信号死亡/--once 单客户端+断开退出/--exit-when-empty 立即与宽限取消）

### Deferred Ideas (OUT OF SCOPE)
- **顶部状态条重连 UI**（不遮冻结现场的 Reconnecting 形态）— 后续迭代；本 phase 复用全屏面板（D-03）
- **会话代际标识（generation id）**——服务端重启后重连成功时提示「这是新会话」— D-07 裁决以文档明示替代；若用户反馈混淆再评估
- **--exit-when-empty 宽限默认值的负载标定** — Phase 9（与 outbox/限速参数同批回填）
- **1001 优雅下线发送路径** — Phase 7（P2 D-08 同批占位）
- **新 flag（--once/--exit-when-empty）配置文件收口** — Phase 7 OPS-09
- **断开退出事件进 metrics/审计日志**（once/empty 触发计数） — Phase 8 OPS-07/OPS-08
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SESS-01 | --once 模式：只接受一个客户端，其断开后服务端退出 | D-12 语法糖展开（max-clients=1 复用 server.go:422-426/566-571 双 503 闸）+ D-13 注册表空 → SIGHUP 进程组（git 历史形态逐字可恢复）→ 既有 lifecycle 收口；UAT phase06.mjs 协议层全链可断言 |
| SESS-02 | 可配置"所有客户端断开后退出"模式 | D-14 `--exit-when-empty[=duration]`（GOROOT flag.go:350-356 IsBoolFlag 惯例核实）+ 宽限计时器挂 detach/kick 两移除点 + attach 取消；冲突校验进 validateStartup 矩阵（main.go:290-322 既有形态） |
| SESS-03 | 子进程退出后客户端收到明确提示（类型化错误帧，含退出码），而非静默断开 | D-08/D-09/D-10：'X' 字节 proto.go:32 已预留 + ExitPayload 组帧（ErrorFrame 先例 proto.go:167-170）+ lifecycle 广播挂点 server.go:965-988 + 退出码/信号提取 GOROOT 配方（exec_posix.go:155-157 / syscall_linux.go:471,486-491） |
| CORE-05 | WS 异常断开后前端自动重连并接回同一 PTY 进程（共享进程模型；无滚动回放） | D-01..D-07：`connect()` 可重入入口（main.ts:390，P3 D-10 auth_failed 重试先例）+ 1006 显式触发 + online/offline 双触发（MDN CITED）+ 退避 1s×2 封顶 30s（throttle.go:12-13 同族参数）+ term.clear()（xterm.d.ts 核实）+ SIGWINCH 既有 attach 挂点（server.go:752 零服务端改动） |
</phase_requirements>

## Project Constraints (from CODEBUDDY.md)

| 约束 | 对本 phase 的影响 |
|------|------------------|
| 纯 headless 环境，**禁止安装浏览器/Playwright/GUI 库**（2026-08-19 用户裁决，永久） | 重连/清屏/面板的浏览器行为**不可**用真实浏览器验证；全部自动化走协议层 + jsdom + @xterm/headless 三层 |
| 验证分层：① `web/uat/phaseNN.mjs` Node 原生 WebSocket/fetch 零依赖脚本 spawn 真实二进制（phase02/03/04/05 先例）；② `@xterm/headless`（需 `allowProposedApi: true`）；③ jsdom + mock；④ 平台原生行为显式豁免（skipped + reason） | phase06.mjs（协议层）+ phase06-dom.mjs（jsdom 重连逻辑面）双脚本形态既定；浏览器权限弹窗/真实断网栈等列为 skipped |
| pnpm 而非 npm；构建命令带 `time` 前缀 | `time pnpm -C web build` |
| 不要在本机启动 wesh 实例等待人工浏览器访问 | UAT 全自动化断言，不留人工等待环节 |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 重连循环/退避/尝试计数 | Browser / Client (main.ts) | — | 网络事件（online/offline/onclose）只在浏览器可见；D-02/D-03/D-04 全部前端状态机 |
| 断线检测（服务端侧） | API / Backend (pinger) | — | 既有 pong_timeout → CloseNow（server.go:887-920），客户端所见即 1006，本 phase 零改动 |
| EXIT 帧组帧与广播 | API / Backend (lifecycle + proto) | — | 退出码唯一来源是服务端 `sess.Wait()`；D-09 服务端组文案唯一写口 |
| 退出码/信号提取 | API / Backend (pty reap + os/exec) | — | `exec.ExitError`/`WaitStatus` 仅服务端可达 |
| 注册表空检测 + 宽限计时 | API / Backend (server registry) | — | 注册表单锁 hubMu 内状态（clients.go），前端无此视角 |
| SIGHUP 进程组 | API / Backend (pty.Session) | — | OS 级信号，需平台构建标签纪律（reap_linux/reap_darwin 先例） |
| --once/--exit-when-empty CLI | API / Backend (cmd/wesh) | — | flag 解析 + 启动校验矩阵纯服务端 |
| 重连首屏恢复 | Browser / Client（term.clear()） | API / Backend（SIGWINCH） | D-05 双件：清屏是前端 buffer 操作；强制重绘信号挂既有 attach 路径（server.go:752，每 attach 恒触发——重连免费获得，**服务端零改动**） |
| owner 递补（重连场景） | API / Backend (registry) | — | D-06 不加豁免 = 走 P5 D-06/D-07 既有 promoteNextLocked 路径，本 phase 零新逻辑 |

## Standard Stack

**本 phase 零新依赖**——全部能力由既有依赖与标准库承载：

### Core（既有，直接复用）
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `flag` | go1.26.3 [VERIFIED: `go version`] | `--exit-when-empty[=duration]` 可选值 flag（自定义 Value + IsBoolFlag） | GOROOT flag.go:350-356 官方惯例："If a Value has an IsBoolFlag() bool method returning true, the command-line parser makes -name equivalent to -name=true rather than using the next command-line argument." [VERIFIED: GOROOT/src/flag/flag.go:350-356] |
| Go stdlib `os/exec` + `syscall` | go1.26.3 | 退出码/信号提取（ExitError.ExitCode / WaitStatus.Signaled / Signal） | lifecycle 已在用（server.go:959-964）；WaitStatus 语义 GOROOT 核实 [VERIFIED: GOROOT/src/os/exec_posix.go:155-157, syscall/syscall_linux.go:471,486-491] |
| `github.com/coder/websocket` | v1.8.15 [VERIFIED: go.mod:6] | EXIT 帧 Write + Close(1000)；Close 内建 5s+5s 上界 | close.go:185/199 两处 `context.WithTimeout(context.Background(), time.Second*5)` 本 session 核实 [VERIFIED: GOMODCACHE coder/websocket@v1.8.15/close.go:185,199] |
| `@xterm/xterm` | 6.0.0 [VERIFIED: web/package.json] | 重连清屏 `term.clear()` | d.ts 逐字："Clear the entire buffer, making the prompt line the new first line." `clear(): void;` [VERIFIED: web/node_modules/@xterm/xterm/typings/xterm.d.ts — clear(): void] |
| 浏览器 `window` online/offline 事件 | Baseline 2015-07 | D-04 断线快路径双触发 | MDN："The offline event of the Window interface is fired when the browser has lost access to the network and the value of Navigator.onLine switches to false." [CITED: developer.mozilla.org/en-US/docs/Web/API/Window/offline_event] |

### Supporting（测试基建，既有）
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| jsdom | 25.0.1 [VERIFIED: web/uat/package.json] | 重连状态机 DOM 逻辑面断言 | phase06-dom.mjs（phase05-dom.mjs SpyWebSocket 注入先例） |
| `@xterm/headless` | 6.0.0 [VERIFIED: web/uat/package.json] | term.clear() 后 buffer 状态断言（如需终端核心层锁定） | phase04-t1-width.mjs 先例；需 `allowProposedApi: true`（项目硬约束） |
| Node 原生 WebSocket/fetch | v24.13.0 [VERIFIED: `node --version`] | 协议层 UAT 驱动 | phase06.mjs 零依赖脚本（phase02/03/05.mjs 先例） |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| term.clear()（D-05 锁定） | 服务端注入 `\x1b[2J\x1b[H` 转义序列进 OUTPUT 流 | 注入伪造输出污染字节流契约（服务端 OUTPUT 零拷贝纪律，P4 D-12）；xterm 原生 API 单调用等价，无注入面 |
| Go flag IsBoolFlag（D-14 锁定） | 预扫描 os.Args 手工改写后再喂 flag 包 | 双解析漂移面；IsBoolFlag 是 stdlib 官方可选值惯例（`-name` ≡ `-name=true`） |
| 前端 setTimeout 退避循环（D-02 锁定） | 引入重连库（如 reconnecting-websocket） | 零新依赖纪律 + 退避逻辑 ~20 行 + 需要与 connect()/showStatus/面板深集成，库反而隔膜 |
| SIGHUP 进程组（D-13 锁定） | 直接 exitf(0) 不杀子进程 | 违反 D-13「exitf 以子进程退出码收口」与 exitf+sync.Once 单一收口纪律；且孤儿子进程泄漏（spawn 失败回滚路径同用 SIGHUP，main.go:371-373 先例） |

**Installation:** 无（零新依赖）。

**Version verification:** 全部为在库/在工具链版本，本 session 逐一命令核实（`go version` = go1.26.3、`node --version` = v24.13.0、`pnpm --version` = 11.21.0、go.mod / package.json / web/uat/package.json 逐字读取）。无训练数据版本依赖。

## Package Legitimacy Audit

**本 phase 不安装任何外部包**——Standard Stack 全部为既有锁定依赖（go.mod / web/package.json / web/uat/package.json 本 session 逐字读取确认）或 Go/Node 标准库。无 `npm install` / `go get` 步骤，Package Legitimacy Gate 无需运行（无输入）。

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| —（无新增） | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```mermaid
flowchart TD
    subgraph 服务端终结路径（SESS-03 / SESS-01 / SESS-02 统一收口）
        A[子进程退出<br/>sess.Wait 返回] --> B[lifecycle:<br/>exit_code 提取<br/>ExitError.ExitCode]
        C[注册表空触发<br/>detach/kick 移除点] -->|--once 或宽限到期| D[SIGHUP 进程组<br/>kill -pgid]
        D -->|子进程被杀| A
        B --> E[Drain 200ms<br/>close inputDone]
        E --> F[hubMu 快照注册表]
        F --> G[每客户端并行:<br/>Write EXIT 帧<br/>→ Close 1000]
        G --> H[terminate: sync.Once<br/>→ exitf code]
    end

    subgraph 前端重连路径（CORE-05）
        J[onclose code==1006] --> K[重连单例循环<br/>attempt N+1]
        L[window offline] --> K
        M[window online] --> K
        K -->|退避 1s×2 封顶30s<br/>或 Reconnect now 跳过| N[connect 重入<br/>fetch ticket → WS → Hello]
        N -->|WELCOME 到达| O[退避清零 + term.clear<br/>+ beforeunload 重注册]
        N -->|fetch 401/404/429/503| P[既有专版面板<br/>循环终止]
        N -->|fetch throw / 再 1006| K
        O --> Q[服务端 attach 路径<br/>SIGWINCH 强制重绘<br/>server.go:752 既有]
    end
```

### Recommended Project Structure（仅列本 phase 触碰/新增）

```
internal/
├── proto/proto.go          # + EXIT = 'X' 常量 + ExitPayload + ExitFrame()（ErrorFrame 先例逐字同构）
├── proto/proto_test.go     # + TestProtocolConstants 帧字节行 + ExitFrame round-trip
├── server/server.go        # lifecycle() 965-988 插 EXIT 广播；Options + ExitWhenEmpty 字段
├── server/clients.go       # detach/kickSlowConsumerLocked 移除后空调用点 + 宽限计时器启停
├── pty/io.go（或新 signal_linux/darwin 文件）# + Session SIGHUP 进程组方法（平台构建标签纪律）
cmd/wesh/main.go            # + --once / --exit-when-empty flag + 语法糖展开 + validateStartup 矩阵行
web/src/main.ts             # 重连状态机 + EXIT 帧 case + onclose 1000 message 显示 + online/offline 监听
web/src/lib/reconnect.ts    # （建议）退避序列纯函数——node --test 可测（prefs.ts/title.ts 先例）
README.md                   # 生命周期节（L13-15）改写 + 用法 flag 表两行 + 重连语义明示（D-07）
web/uat/phase06.mjs         # 协议层 UAT（phase05.mjs 骨架逐字沿用）
web/uat/phase06-dom.mjs     # jsdom 重连逻辑面（phase05-dom.mjs SpyWebSocket 先例）
```

### Pattern 1: EXIT 帧组帧（ErrorFrame 先例逐字同构）

**What:** 新 S→C 控制帧 = 1 字节类型 + JSON 载荷，组帧函数返回 `[]byte` 供调用方直接写。
**When to use:** SESS-03 lifecycle 广播；D-08/D-09。
**关键先例事实：** proto.go:32 注释逐字预留——`'X' EXIT / 'T' TITLE / 'P' PREFS —— 类型字节本 phase 占住，语义分属 Phase 6/4（D-01）` [VERIFIED: internal/proto/proto.go:32]。'T' 已终局不实现（P5 D-12）、'P' 留 v2，**'X' 就是 EXIT 的指定位**（与 CONTEXT  discretion 建议一致）。
**Example:**
```go
// Source: internal/proto/proto.go:165-170 ErrorFrame 先例形态（本 session 逐字读取）：
// func ErrorFrame(code, message string) []byte {
// 	b, _ := json.Marshal(ErrorPayload{Code: code, Message: message})
// 	return append([]byte{Error}, b...)
// }

// EXIT 帧同构落地（D-09 载荷形状锁定——json tag snake_case，前后端公开契约）：
const Exit = 'X' // 0x58, S→C, JSON {"exit_code":N,"message":M}——子进程退出终结帧（Phase 6 D-08/D-09）

type ExitPayload struct {
	ExitCode int    `json:"exit_code"` // 信号死亡 = -1（D-09；exec ExitCode 语义同源）
	Message  string `json:"message"`   // 英文人话，前端直显（服务端组文案唯一写口）
}

func ExitFrame(exitCode int, message string) []byte {
	b, _ := json.Marshal(ExitPayload{ExitCode: exitCode, Message: message})
	return append([]byte{Exit}, b...) // 固定 schema 下 json.Marshal 不会失败（ErrorFrame 同款注释）
}
```
**测试锁定：** TestProtocolConstants 帧字节表（proto_test.go:196-212 逐字表）加 `{"Exit", Exit, 'X'}` 行；ExitFrame round-trip 断言并入 TestWelcomeFrameErrorFrame 同文件先例。

### Pattern 2: lifecycle EXIT 广播（写序安全形态）

**What:** 子进程退出后向全部在线客户端广播 EXIT 帧再 1000 关闭。
**When to use:** SESS-03 唯一挂点 = `lifecycle()` Drain 之后、并行 Close 循环之内（server.go:955-995 逐行核实：Wait 959 → code 提取 960-964 → Drain(200ms) 965 → close(inputDone) 969 → hubMu 快照 974-979 → 并行 Close(1000) 981-988 → terminate 994）。
**Example:**
```go
// Source 基座: internal/server/server.go:974-988（本 session 逐字读取的快照+并行 Close 形态）
// 在每客户端 goroutine 内【先同步 Write(EXIT) 再 Close(1000)】——写序论证：
// ① coder/websocket 帧级写串行化（库内 write 路径互斥），同 goroutine 先写后发关闭帧，
//    wire 序恒 EXIT 在前；② Write 用带超时 ctx（如 2s）——stall 客户端不拖延 exitf，
//    写失败直接进 Close（进程已退出场景无需保帧，CONTEXT discretion 授权）；
// ③ Close 内建 5s+5s 上界 [VERIFIED: coder/websocket@v1.8.15 close.go:185,199]，
//    并行等待自然有界（server.go:971-973 既有注释同款论证）。
exitFrame := proto.ExitFrame(code, exitMessage(err, code)) // 组帧一次，全客户端共享只读引用（P5-1 纪律）
s.hubMu.Lock()
clients := make([]*client, 0, len(s.registry.set))
for c := range s.registry.set {
	clients = append(clients, c)
}
s.hubMu.Unlock()
var wg sync.WaitGroup
for _, c := range clients {
	wg.Add(1)
	go func() {
		defer wg.Done()
		wctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = c.conn.Write(wctx, websocket.MessageBinary, exitFrame) // 失败不补救——Close 仍把 1000 送上
		cancel()
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
	}()
}
wg.Wait()
```
**Anti-pattern 警示：** 不要把 EXIT 帧经 `outbox.trySend` 异步入队后再并行 Close——writer drain 与 Close 写关闭帧竞态，关闭帧可能超车（客户端收 1000 却无退出码）。EXIT 必须同步直写（见 Common Pitfalls P1）。

### Pattern 3: 退出码与信号名提取（GOROOT 核实配方）

**What:** 从 `sess.Wait()` 返回错误组成 D-09 的 `{exit_code, message}`。
**Verified semantics（本 session GOROOT 逐字核实）：**
- `ProcessState.ExitCode()`：`"ExitCode returns the exit code of the exited process, or -1 if the process hasn't exited or was terminated by a signal."` [VERIFIED: GOROOT/src/os/exec_posix.go:155-157]（`exec.ExitError` 内嵌 `*os.ProcessState`，exec.go:885-886）
- `WaitStatus.Signaled()` / `WaitStatus.Signal()` [VERIFIED: GOROOT/src/syscall/syscall_linux.go:471,486-491]
- **信号名陷阱：** `Signal.String()` 走 `signals` 表，值为小写描述词——`1: "hangup"`, `2: "interrupt"`, `9: "killed"` [VERIFIED: GOROOT/src/syscall/zerrors_linux_amd64.go:1491-1500]——**不是** "SIGHUP"。D-09 示例文案 "killed by signal SIGHUP" 若裸用 `sig.String()` 会产出 "killed by signal hangup"（语义别扭）。二选一（discretion）：`fmt.Sprintf("killed by signal %d (%s)", sig, sig)` 或自维护 SIG* 大写名小表。
```go
// Source: server.go:959-964 既有提取形态扩展（本 session 逐字读取）
err := s.sess.Wait()
code := 0
var ee *exec.ExitError
if errors.As(err, &ee) {
	code = ee.ExitCode()
}
var msg string
switch {
case err == nil:
	msg = "process exited with code 0"
	case code >= 0:
	msg = fmt.Sprintf("process exited with code %d", code)
default:
	// 信号死亡（ExitCode = -1）：WaitStatus 取信号（unix 两平台同型；
	// 非 ExitError 形态（Wait 其他错误）走兜底文案——discretion 授权项）
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		msg = fmt.Sprintf("killed by signal %d (%s)", int(ws.Signal()), ws.Signal())
	} else {
		msg = "process terminated" // 非 ExitError 兜底（discretion）
	}
}
```

### Pattern 4: 注册表空触发 → SIGHUP 进程组（P1 D-11 语义复活）

**What:** `--once` 唯一客户端断开 / `--exit-when-empty` 宽限到期仍空 → SIGHUP 子进程进程组 → 既有 lifecycle 收口。
**挂点（逐行核实）：** 注册表移除恰好两个调用点——`detach`（clients.go:677-696）与 `kickSlowConsumerLocked`（clients.go:491-513），两者都在 `removeLocked` 成功后仍持 hubMu；attach 侧登记点 = `registerLocked`（server.go:731，hubMu 内）。**timer 启停全在 hubMu 内，零新锁。**
**SIGHUP 形态（git 历史逐字恢复）：** P1 旧实现（05-01 多客户端化时拆除）——
```go
// Source: git show cc03c79~1:internal/server/server.go:649-657（本 session 逐字读取）：
// "sighup 为真时先 SIGHUP 子进程进程组：负 pid = 进程组；setsid 使子进程为组长，
//  pgid = 子进程 pid（D-11）。Start 成功后 Cmd.Process 必非 nil。"
syscall.Kill(-sess.Cmd.Process.Pid, syscall.SIGHUP)
```
setsid 由 creack/pty StartWithSize 内建（spawn.go:51 调用链），pgid == 子进程 pid 的论证沿用 P1 注释。**落点建议：** 作为 `pty.Session` 的平台方法（reap_linux.go/reap_darwin.go 构建标签纪律同款——syscall.Kill 为 unix-only，V2-WINDOWS 期的分叉面一次性收进 pty 包）。ttyd 先例：`-s, --signal  Signal to send to the command when exit it (default: 1, SIGHUP)` [CITED: github.com/tsl0922/ttyd man/ttyd.1]——SIGHUP 默认与 D-13 一致，Phase 7 OPS-04 可配化不提前设计。
**宽限计时器骨架（discretion 项的推荐形态）：**
```go
// hubMu 内调用点统一形态：移除后 len(s.registry.set)==0 且模式开启 → 启 timer；
// registerLocked 后 → 停 timer。time.Timer 回调自有 goroutine 需取 hubMu
//（arbiter timer 同款纪律，server.go:90-91 注释先例：「timer 回调自有 goroutine 取 hubMu」）。
// 回调内复查仍空 → 调 SIGHUP 方法（不调 exitf——零新 exitf 分支，D-13 硬约束）。
```

### Pattern 5: `--exit-when-empty[=duration]` 可选值 flag（IsBoolFlag 惯例）

**What:** 单 flag 三形态：不写 = 不开启；裸写 = 立即退出（grace 0）；`=30s` = 宽限。
**Verified semantics：** GOROOT flag.go:350-356 逐字——`"If a Value has an IsBoolFlag() bool method returning true, the command-line parser makes -name equivalent to -name=true rather than using the next command-line argument."` [VERIFIED: GOROOT/src/flag/flag.go:350-356]。**推论：** `--exit-when-empty 30s`（空格分隔）**不消费** `30s`（会被当 argv 首元）；只有 `--exit-when-empty=30s` 等号形态传值——恰好符合 D-14「裸写/=duration」两形态设计。
```go
// Source: GOROOT flag 包惯例 + main.go:159-163 fs.Visit 先例（writePolicySet 判定形态）
type exitEmptyValue struct{ set bool; grace time.Duration }
func (v *exitEmptyValue) String() string { /* ... */ }
func (v *exitEmptyValue) IsBoolFlag() bool { return true } // 裸写 → Set("true")
func (v *exitEmptyValue) Set(s string) error {
	v.set = true
	if s == "true" { v.grace = 0; return nil }        // 裸写 = 立即退出
	d, err := time.ParseDuration(s)                    // =duration 形态；负值/非 duration 报错
	if err != nil || d < 0 { return fmt.Errorf("invalid duration") }
	v.grace = d
	return nil
}
```
**错误上报红线：** 值非敏感（duration），可直接 return error——但注意 main.go:96-104 credErr/clientOptErr 记录式先例仅用于值含敏感内容的 flag；本 flag 走 `--write-policy` 同款直接 return（main.go:180-182 先例）。
**`--once` 语法糖展开与冲突校验：** fs.Visit 显式设置判定先例 [VERIFIED: cmd/wesh/main.go:159-163]——`--once` 展开为 maxClients=1 + exitEmpty{set, grace:0}；显式 `--max-clients` 值 ≠1 或显式 `--exit-when-empty` 值非 0 与 `--once` 同给 = 配置矛盾，进 `validateStartup` 矩阵 fail-fast（main.go:290-322 既有形态：纯函数零副作用、先于 pty.Start/net.Listen、exit 2）。

### Pattern 6: 前端重连状态机（connect() 重入 + 代际守卫）

**What:** 单例重连循环：1006 触发 → 退避等待（面板倒计时）→ connect() 重入 → WELCOME 成功清零。
**When to use:** CORE-05 全部前端逻辑。
**挂点（main.ts 逐行核实）：**
- `connect()` 已是可重入入口（P3 D-10 auth_failed 静默重试先例，main.ts:696-701）；IN-01 per-connection 重置块 391-404（注释逐字："auth_failed 重试/未来 Phase 6 重连不携带上连接的残留……"）——**重连状态清零基建已就位**
- onclose 按码分派 switch（main.ts:706-753）：1000/1008/1009/1011/1013 各分支保持面板语义不动；**触发谓词 = `ev.code === 1006` 显式判定**，不是 default 桶——1002 协议错误也落 default（main.ts:745 注释逐字："含 1002 协议错误与无码异常断开"），default 整体转重连会把确定性协议失败卷入重试循环
- 重连成功判定 = WELCOME 到达（main.ts:634 `welcomeDone = true` 点）；beforeunload 重注册既有（635-637，P4 D-18 先例）
- `showStatus(title, body, hintPrefix)`（main.ts:365-381）：**hint 链接当前硬编码 "Reload this page"**（370-377）——D-03 的「Reconnect now」需要将 hint 动作链接参数化（小重构，幂等纪律保持：textContent 先清后建，365-381 注释既有）

```typescript
// Source 基座: web/src/main.ts:20-26 帧常量（本 session 逐字读取）+ connect() 390-757 形态
// 帧常量表加 EXIT = 0x58（proto.go 'X' 手工对齐，D-16 两侧注释互相指路纪律）

// 代际守卫（本 phase 新必要件）：重连使页面首次存在「旧 socket 未死透 + 新 socket 已建立」
// 双连接窗口——旧 socket 迟到的 onclose/onmessage 必须不触碰新会话状态。
// connect() 内既有 `const sock = ws`（main.ts:476，TS 闭包收窄先例）天然承载：
//   sock.onclose = (ev) => { if (sock !== ws) return; /* stale 代际丢弃 */ ... }
// 今日单连接生命周期下该判定恒真，重连落地后成为必需闸。

// 退避纯函数（建议抽 web/src/lib/reconnect.ts——node --test 单测先例 prefs.test.ts/title.test.ts）：
export function backoffMs(attempt: number): number {
  // D-02 参数族：1s×2 封顶 30s 无限重试（throttle.go:12-13 同族：
  // defaultThrottleBase = 1s / defaultThrottleCap = 30s [VERIFIED: internal/server/throttle.go:12-13]）
  return Math.min(1000 * 2 ** attempt, 30000);
}
```

**重连循环策略规则（D-01/D-02/D-11 合并推导）：**
- fetch throw（网络不可达/服务器已退出无法区分，D-11）→ 留在循环，Reconnecting 面板持续（hint 明示「若服务端已退出请从 shell 重启」）
- fetch 收到 401/404/429/503 → 既有专版面板（main.ts:434-468 四分支）= 终态，**循环终止**（D-01：仅 1006 类自动重连）
- WS onclose code===1006 → 留在循环；1000/1008/1009/1011/1013/其他 → 既有面板分支，循环终止
- WELCOME 到达 → attempt 清零、面板隐藏、`term.clear()`、状态面板复位

**EXIT 帧前端承接（D-10）：**
```typescript
// onmessage switch 加 case EXIT（0x58）：
case EXIT: // D-09/D-10：暂存 {exit_code,message}，onclose 1000 分支正文显示 message
  try {
    lastExit = JSON.parse(new TextDecoder().decode(buf.subarray(1))); // lastError 同款暂存通道（main.ts:171 先例）
  } catch {
    console.warn('discard malformed EXIT frame'); // ERROR 帧同款容错（main.ts:646-648）
  }
  break;
// onclose case 1000（main.ts:707-713）：body = lastExit?.message ?? 既有硬编码文案
```
**前向兼容：** 旧前端 default 分支静默跳过未知 S→C 帧（main.ts:650-651 逐字核实）——旧前端收 EXIT 落 default 丢弃，1000 面板显示既有硬编码文案，优雅降级成立（P2 D-02 纪律）。

### Anti-Patterns to Avoid
- **EXIT 帧走 outbox 异步入队：** 与并行 Close 竞态致关闭帧超车（见 Pitfall P1）——同步直写唯一正解
- **重连触发用 onclose default 桶：** 1002 落 default（main.ts:745）——谓词必须 `ev.code === 1006` 显式
- **以「收到 1006」为线上协议预期：** 1006 永不出现线上（RFC6455 §7.4 保留值，proto.go:14 注释；浏览器本地合成）——PITFALLS Pitfall 9 老纪律，本 phase 触发谓词是浏览器 CloseEvent 语义而非线上码
- **给前端加应用层心跳帧判活：** D-04 已裁决——浏览器 WS API 不暴露 ping/pong，空闲终端无流量，「多久没收到消息」判据结构性不成立
- **重连保留旧 buffer 等增量重绘：** D-05 已裁决——相对寻址流花屏风险（G-05-1 同源），term.clear() + SIGWINCH 重绘唯一正解
- **owner 重连恢复写权限：** D-06 已裁决——身份恢复窗口与 P5 递补确定性冲突
- **自写 duration 解析/预扫描 os.Args：** IsBoolFlag 惯例（Pattern 5）+ time.ParseDuration 全覆盖
- **新 exitf 分支/绕过 terminate：** D-13 硬约束——断开退出只发 SIGHUP，终结由既有 lifecycle 单一路径收口

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 可选值 CLI flag | os.Args 预扫描/双 flag 组合 | flag.Value + IsBoolFlag（Pattern 5） | GOROOT 官方惯例；空格分隔误用结构性排除 [VERIFIED: flag.go:350-356] |
| duration 解析 | 自写 "30s" 解析器 | `time.ParseDuration` | 单位矩阵（ms/s/m/h）与边界（负值/溢出）已处理 |
| 退出码/信号提取 | 手解 wait status 位 | `ExitError.ExitCode()` + `WaitStatus.Signaled()/Signal()` | GOROOT 语义核实（Pattern 3）；位操作跨平台脆弱 |
| 清屏 | 注入 `\x1b[2J` 转义进输出流 | `term.clear()` [VERIFIED: xterm.d.ts] | 伪造输出污染零拷贝纪律；API 单调用等价 |
| 关闭码异常检测 | 解析 ev.reason 文本/自定义码 | `ev.code === 1006` | reason 不可控（库内字符串，main.ts:685-687 注释）；4000 段私码禁止（proto.go:14） |
| 重连退避 | 第三方重连库 | ~20 行自管循环 + `backoffMs` 纯函数 | 零新依赖纪律；与 connect()/面板深集成 |
| SIGHUP 进程组 | cmd.Process.Signal（只打主进程） | `syscall.Kill(-pid, SIGHUP)`（Pattern 4） | 负 pid = 进程组，子进程孙子全覆盖；P1 逐字先例 |

**Key insight:** 本 phase 的全部「难」都在**时序与收口纪律**（EXIT↔Close 写序、timer 启停恰好一次、代际守卫、exitf 单点），而非新算法/新库——既有代码库已为每个挂点备好先例，照抄形态是唯一正确姿势。

## Common Pitfalls

### Pitfall 1: EXIT 帧与 1000 关闭帧写序竞态
**What goes wrong:** EXIT 帧经 outbox.trySend 入队后并行 `Close(1000)`——writer goroutine 尚未 drain EXIT，Close 的关闭帧先行上线，客户端收 1000 却无退出码（SESS-03 失败但测试偶现通过——快客户端 drain 够快时序成立）。
**Why it happens:** lifecycle 现有广播形态（server.go:981-988）只有 Close 没有数据帧；加 EXIT 时顺手复用 outbox 扇出直觉。
**How to avoid:** 每客户端 goroutine 内**同步** `conn.Write(EXIT)`（带 ~2s 超时 ctx 防 stall 拖延 exitf）后接 `conn.Close(1000)`——库帧级写串行化保证同 goroutine 先写先发；失败不补救（进程已退出无需保帧，CONTEXT discretion 授权）。
**Warning signs:** UAT 断言「EXIT 先于 onclose 到达」间歇性失败；慢链路下退出码丢失。

### Pitfall 2: 「注册表空」在启动期恒真 → --exit-when-empty 启动即退出
**What goes wrong:** 服务刚启动尚无客户端 attach，注册表本就为空——若空调检测挂在「轮询注册表是否为空」而非「非空→空的迁移」，`--exit-when-empty` 启动即触发退出；`--once` 同形（首个客户端还没来就退了）。
**Why it happens:** 把状态条件（is-empty）当事件（became-empty）。
**How to avoid:** 检测只挂两个移除点（detach/kickSlowConsumerLocked 的 removeLocked 成功后）——只有曾非空才可能走到移除；启动期零调用天然免疫。timer 回调内复查仍空再发 SIGHUP（宽限窗口内 attach 已取消 timer，复查是恰好一次的兜底）。
**Warning signs:** `wesh --exit-when-empty -- bash` 启动后立刻退出；UAT 首场景即挂。

### Pitfall 3: Signal.String() 产出 "hangup" 而非 "SIGHUP"
**What goes wrong:** D-09 示例文案 "killed by signal SIGHUP"，裸用 `ws.Signal().String()` 产出 "killed by signal hangup"——GOROOT signals 表是小写描述词（`1: "hangup"`，`9: "killed"`）[VERIFIED: zerrors_linux_amd64.go:1491-1500]。
**How to avoid:** 文案组写口显式选形：`"killed by signal %d (%s)"`（数字 + 描述词）或自维护大写名小表（discretion 项；文案唯一写口在服务端，前端不自维护信号表——D-09 纪律）。
**Warning signs:** 面板/断言里出现 "signal hangup" 式文案。

### Pitfall 4: 宽限计时器泄漏/双触发
**What goes wrong:** timer 启动后会话经 lifecycle 正常退出，timer 仍在跑——到期向已死进程组发 SIGHUP（kill 失败静默，无害但脏）；或多路径重复启 timer。
**Why it happens:** timer 生命周期没有跟会话绑定。
**How to avoid:** timer 启停全部在 hubMu 内（启动=移除点，取消=registerLocked 后）；回调先取 hubMu 复查（arbiter timer 同款纪律，server.go:90-91 注释先例）；SIGHUP 幂等（kill 已死 pgid 返回 ESRCH 静默忽略）；exitf 侧 sync.Once 天然兜底双触发。
**Warning signs:** -race 下 timer 回调与 detach 数据竞争；进程退出后 stderr 出现莫名信号错误日志。

### Pitfall 5: 重连双循环（online + onclose + offline 同时触发）
**What goes wrong:** 断网瞬间 offline 事件与 WS onclose(1006) 相继到达，各启一个重连循环——双循环竞争 connect()，attempt 计数与退避状态互相覆盖。
**How to avoid:** 重连循环单例门闩（`reconnecting` 布尔/state 机），三触发源全部经「已在循环则幂等返回」入口（CONTEXT discretion 明示项）；`online` 立即试一次 = 清当前 setTimeout 立即 attempt，不是新循环。
**Warning signs:** 面板 attempt 计数跳变（1→3→2）；同时两个 fetch /api/attach 在飞。

### Pitfall 6: 陈旧 socket 代际事件污染新会话
**What goes wrong:** 重连已建立新连接后，旧 socket 迟到的 onclose（TCP RST 终于到达）执行旧闭包——`showStatus` 把健康会话盖成「Connection lost」面板，或重置新连接的 per-connection 状态。
**Why it happens:** 今日代码单连接生命周期（一页面一连接自始至终），handler 闭包无代际概念；重连首次引入双连接窗口。
**How to avoid:** 每个 handler 入口 `if (sock !== ws) return;`——`const sock = ws` 闭包先例（main.ts:476）天然承载；onclose 的 beforeunload 移除（main.ts:691）同样需代际判定（否则会拆了新连接刚注册的监听）。
**Warning signs:** 重连成功后面板偶发闪回「Connection lost」；beforeunload 在健康会话上神秘失效。

### Pitfall 7: 重连中 fetch 失败面板覆盖 Reconnecting 面板
**What goes wrong:** connect() 既有 catch 分支（main.ts:469-472）与 onerror-!opened 分支（677-683）会 `showStatus('Unable to connect', …)`——重连循环内每次失败尝试都把「Reconnecting attempt N」面板打回「Unable to connect」，倒计时/立即重试入口消失。
**How to avoid:** connect() 增加重连上下文参数（或面板抑制回调）——循环驱动的尝试失败只更新 Reconnecting 面板计数，专版面板仅终态分支（401/404/429/503/明确关闭码）才允许覆盖。
**Warning signs:** 断网测试下面板标题在 Reconnecting 与 Unable to connect 之间抖动。

### Pitfall 8: `os.Exit(-1)` 截断为 255（SIGHUP 收口路径退出状态漂移）
**What goes wrong:** D-13 路径 SIGHUP 杀子进程 → `ExitCode() = -1`（GOROOT 语义，Pattern 3）→ `exitf(-1)` → `os.Exit(-1)` Unix 截断为退出状态 **255**——与 Phase 1 旧单次语义的 exit 0（git 历史 `s.terminate(true, 0)`）不一致。
**How to avoid:** 见 Open Questions OQ1——三选项（照单收 255 / 映射 0 / 映射 128+SIGHUP=129）需裁决；测试断言按裁决值锁定。
**Warning signs:** UAT 断言进程退出码时拿到 255 而非预期 0。

## Code Examples

全部关键示例已并入上方 Architecture Patterns（Pattern 1-6 含逐字核实来源标注）。此处仅补 UAT 断言形态：

### UAT：EXIT 帧断言（phase06.mjs 形态，phase05.mjs 骨架沿用）
```javascript
// Source 基座: web/uat/phase05.mjs:98-122 dialHello/waitClose 先例（本 session 逐字读取）
// 场景：两在线客户端 + 子进程退出（如 spawn 'sh -c "exit 42"'）→ 双端同收 EXIT{exit_code:42} 后 1000
const EXIT = 0x58; // 与 proto.go 对齐（harness 帧常量区先例 phase05.mjs:28）
const exitOf = (frames) => JSON.parse(dec.decode(frames.find((f) => f[0] === EXIT).subarray(1)));
// 断言：exitOf(a.frames).exit_code === 42（spawn 'sh','-c','exit 42'）；
//        waitClose(ws) 得 code === 1000；
//        信号死亡形态（spawn 'sleep 600' 后外部 kill）→ exit_code === -1 且 message 含信号信息；
//        双端帧体逐字节一致（ro/rw 全员同帧——终结无权限语义，CONTEXT Established Patterns）
```

### UAT：--once 全链（phase06.mjs）
```javascript
// --once 实例：首客户端 attach → 第二客户端 /api/attach 503 + rawUpgrade 503（phase05.mjs S5 双点位先例逐字可抄）
// → 首客户端 ws.close() → 轮询 child 进程 exit（spawn 句柄 'exit' 事件）→ 断言进程已退出
// 且 stderr 无异常；退出码按 OQ1 裁决值断言。
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| P1 D-11：任何 WS 断开 → SIGHUP + exitf(0)（单客户端单次语义） | P5：断开只做注册表移除，生命周期只随子进程 | Phase 5 (05-01) | Phase 6 的 SESS-01/02 以**显式 opt-in flag** 请回退出语义——默认行为（断开不退出）不变 |
| P1 terminate(sighup, code) 双参收口 | terminate(code) + sync.Once 单参数单触发源（lifecycle 唯一） | Phase 5 | D-13 复活 SIGHUP 时**不复活 terminate 的 sighup 分支**——SIGHUP 只是触发源，收口仍走 lifecycle 单一路径（git 历史形态对照本 session 核实） |
| 前端一切异常断开 → 「Connection lost」手动面板 | 1006 类 → 自动重连循环（其余码面板语义不变） | Phase 6 本 phase | main.ts:745-752 default 桶拆分：1006 显式抽出，其余留守 |
| ttyd `-o, --once` / `-q, --exit-no-conn` / `-s, --signal`（默认 SIGHUP） | wesh `--once` / `--exit-when-empty[=duration]` / （Phase 7 OPS-04 信号可配） | Phase 6 | 语义对齐 ttyd 肌肉记忆，flag 名全新设计（REQUIREMENTS Out of Scope：不做 ttyd CLI 兼容）[CITED: ttyd man/ttyd.1 逐字："-o, --once  Accept only one client and exit on disconnection"；"-q, --exit-no-conn  Exit on all clients disconnection"] |
| wesh --once 等价展开 | `--max-clients=1 --exit-when-empty=0` | Phase 6 D-12 | 独立 flag 保留（ttyd 肌肉记忆），README 标明等价关系 |

**Deprecated/outdated:**
- 「重连 = 会话保持/滚动回放」认知：v1 明确不做（PROJECT 锁定、REQUIREMENTS V2-SESSION 延期）；重连承诺边界 = 接回同一进程 + 输入输出一致（D-05/06/07 三收窄逐条对应被否决的状态机）
- 前端「1006 永不作为分派依据」注释（main.ts:686-687 现状）：Phase 4/5 语境下成立（彼时无自动重连）；本 phase 起 1006 成为重连唯一触发码——注释需随实现改写，避免下任读者误读

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 浏览器 online/offline 事件在目标浏览器按 MDN 语义触发（offline = Navigator.onLine 转 false 时 fire；onLine=true 不证明服务端可达） | Pattern 6 / D-04 支撑 | LOW——D-04 已把事件定位为快路径提示层，onclose 为权威信号；事件失效仅退化为 TCP 超时后重连（CONTEXT 明示风险接受）[CITED: MDN Window offline_event / Navigator.onLine] |
| A2 | jsdom 层可用注入的 SpyWebSocket 合成 `CloseEvent{code:1006}` 驱动重连循环断言 | Validation Architecture | LOW——phase05-dom.mjs 已验证 WebSocket 类整体注入形态；合成 close 事件是同型延伸，若受阻可用真实断连（destroy socket）替代 |
| A3 | 凭据模式重连的 fetch /api/attach 不经人工交互自动携浏览器 Basic 缓存凭据 | Pattern 6 循环规则 | LOW——P3 已 UAT 验证（main.ts:413-416 注释「A2 假设，UAT 必验」的兑现先例）；分享链接模式 URL token 保留可重 POST（P5 D-03） |
| A4 | `--once` 断开退出路径的进程退出状态 255（os.Exit(-1) 截断）可被接受 | Pitfall 8 / OQ1 | MEDIUM——机制 GOROOT 级确定，可接受性是产品裁决；若用户要 exit 0 或 129，lifecycle 需加一处映射分支（小改但触公开行为） |

## Open Questions

1. **--once/--exit-when-empty 收口路径的进程退出状态**
   - What we know: D-13 锁定「SIGHUP 进程组 → Drain → exitf 以子进程退出码收口」；SIGHUP 致死 → `ExitCode() = -1`（GOROOT 逐字核实）→ `os.Exit(-1)` → Unix 退出状态 255。Phase 1 旧语义为 exit 0（git 历史 `terminate(true, 0)`）。D-09 对 EXIT 帧载荷已确立「信号死亡 exit_code=-1」先例——同语义延伸到进程退出状态即 255。
   - What's unclear: 部署脚本/文档对 wesh 自身退出状态的预期（0 = 正常收口 vs 255 = 信号收口 vs 129 = 128+SIGHUP shell 惯例）。
   - Recommendation: 照单收（lifecycle 零分支改动，D-13 字面形态；README 明示「--once/--exit-when-empty 收口 = 子进程被 SIGHUP 终结，wesh 退出状态 255」）——plan-check 或计划评审时向用户确认一行裁决；备选 = terminate 前映射 -1→0（保 P1 语义）或 128+sig。
2. **Reconnecting 面板 hint 的「Reconnect now」链接形态**
   - What we know: showStatus 当前硬编码 "Reload this page" 链接（main.ts:370-377 逐字核实）；D-03 要 hint 处放可点「Reconnect now」；D-11 要 hint 文案含「若服务端已退出请从 shell 重启」。
   - What's unclear: showStatus 参数化（第四参传动作链接 label+callback）vs 为 Reconnecting 单独建变体函数。
   - Recommendation: 参数化 showStatus（动作链接 label/onClick 可选参数，默认保持 Reload 现状零漂移）——三态面板单组件纪律（P5 D-07 哲学）下唯一不复制 DOM 结构的形态。
3. **EXIT 帧广播 Write 超时时长**
   - What we know: Close 内建 5s+5s 上界（库源码核实）；EXIT 同步直写需要自带超时 ctx 防 stall 端拖延 exitf。
   - What's unclear: 具体值（2s 为建议值——足够快客户端收下 ~100B 帧，远小于 Close 上界）。
   - Recommendation: 2s 常量入 proto 或 server 常量区（P2 D-10 常量纪律，Phase 9 标定注释同款挂账）。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 工具链 | 全部服务端改动 + 单测 | ✓ | go1.26.3 [VERIFIED: `go version`] | — |
| Node.js | UAT 脚本（原生 WebSocket ≥22）+ node --test | ✓ | v24.13.0 [VERIFIED] | — |
| pnpm | 前端构建 | ✓ | 11.21.0（与 CI 钉版一致）[VERIFIED] | — |
| jsdom / @xterm/headless | DOM 层/终端核心层 UAT | ✓ | 25.0.1 / 6.0.0（web/uat 已装） | — |
| vim | UAT 全屏重绘夹具（S8 先例） | ✓ | /usr/bin/vim [VERIFIED: `which vim`] | — |
| bash | UAT 会话夹具 | ✓ | /usr/bin/bash [VERIFIED] | sh -c 等价 |
| 浏览器 | —（永不具备，项目硬约束） | ✗ | — | 四层测试策略豁免条款（phase06-dom.mjs + skipped 记录） |

**Missing dependencies with no fallback:** 无（浏览器缺失为永久既定约束，已有成熟豁免分工，不阻塞）。
**Missing dependencies with fallback:** 无。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework（Go） | testing + `-race`（go1.26.3）；CI：`go vet ./...` + `go test -race -count=1 -v ./...`（ubuntu+macos 矩阵）[VERIFIED: .github/workflows/ci.yml:14-17] |
| Framework（前端纯函数） | `node --test` 直跑 .ts（Node 24 内建 type stripping；web/src/lib/*.test.ts 先例，tsconfig exclude `src/**/*.test.ts`）[VERIFIED: web/tsconfig.json / prefs.test.ts:1-5] |
| Framework（协议层 UAT） | Node 零依赖脚本 spawn 真实二进制（web/uat/phaseNN.mjs 先例） |
| Framework（DOM 层 UAT） | jsdom 加载真实 dist/index.html + 注入 WebSocket/fetch 桩连真实实例（phase05-dom.mjs 先例） |
| Config file | 无独立配置——Go 测试随包、UAT 脚本直跑 |
| Quick run command | `go test -race -count=1 ./... && time pnpm -C web build && node --test web/src/lib/*.test.ts` |
| Full suite command | 上述 + `node web/uat/phase02.mjs … phase05.mjs phase05-dom.mjs phase05-dims.mjs phase06.mjs phase06-dom.mjs`（对同一构建产物；六段式先例：既有四脚本全绿为回归基线） |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SESS-01 | --once 第二客户端 503 双闸（/api/attach + WS ③位） | integration (UAT) | `node web/uat/phase06.mjs`（S5 双点位先例逐字可抄） | ❌ Wave 0 |
| SESS-01 | --once 唯一客户端断开 → 服务端进程退出 | integration (UAT) | `node web/uat/phase06.mjs`（spawn 'exit' 事件断言） | ❌ Wave 0 |
| SESS-01/02 | flag 解析三形态（不写/裸写/=duration）+ 语法糖展开 + 冲突矩阵 | unit | `go test ./cmd/wesh -run 'TestParseArgs|TestStartupMatrix'`（表结构扩展先例：03-04 命名字段转换） | ✅ 扩既有 |
| SESS-02 | 宽限计时内 attach 取消退出；到期仍空触发退出 | integration (UAT + Go 白盒) | `node web/uat/phase06.mjs` + `go test ./internal/server`（exitf 捕获桩先例——Server 注释「生命周期必须可测，这是硬约束」server.go:32） | ❌ Wave 0（UAT）/ ✅ 扩既有（Go） |
| SESS-03 | EXIT 帧组帧 round-trip + 'X' 字节逐字 | unit | `go test ./internal/proto -run 'TestProtocolConstants|TestExitFrame'` | ✅ 扩既有 |
| SESS-03 | 双端在线收 EXIT{exit_code:42} → 1000；信号死亡 exit_code=-1 + message 含信号信息 | integration (UAT) | `node web/uat/phase06.mjs`（sh -c 'exit 42' / 外部 kill 夹具） | ❌ Wave 0 |
| SESS-03 | EXIT→Close 写序（EXIT 必先于 onclose 到达） | integration (UAT) | `node web/uat/phase06.mjs`（帧序断言） | ❌ Wave 0 |
| CORE-05 | backoffMs 纯函数序列（1s,2s,4s…封顶 30s） | unit | `node --test web/src/lib/reconnect.test.ts` | ❌ Wave 0（建议新建 lib/reconnect.ts + 测试） |
| CORE-05 | 1006 → 自动重连接回同一 PTY（echo marker 前后一致）；1000/1008/1009/1011/1013 不重连 | logic (jsdom UAT) | `node web/uat/phase06-dom.mjs`（SpyWebSocket 合成 CloseEvent 先例延伸） | ❌ Wave 0 |
| CORE-05 | 重连成功 term.clear() + WELCOME 退避清零 + beforeunload 重注册 | logic (jsdom UAT) | `node web/uat/phase06-dom.mjs` | ❌ Wave 0 |
| CORE-05 | 断网 30s 恢复接回同一进程（协议层等价物：WS destroy → 新 attach 同 PTY marker 连续） | integration (UAT) | `node web/uat/phase06.mjs` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -count=1 ./... && time pnpm -C web build`（改动前端纯函数时加 `node --test web/src/lib/*.test.ts`）
- **Per wave merge:** quick + 既有 UAT 四脚本（phase02/03/04/05 系列）回归
- **Phase gate:** 全量含 phase06.mjs / phase06-dom.mjs 双新脚本绿，再 `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/uat/phase06.mjs` — SESS-01/02/03 协议层 + CORE-05 断连重接（phase05.mjs 骨架逐字沿用：startWesh/dialHello/waitClose/rawUpgrade/rawStallClient 五件）
- [ ] `web/uat/phase06-dom.mjs` — CORE-05 重连状态机逻辑面（phase05-dom.mjs SpyWebSocket 注入先例）
- [ ] `web/src/lib/reconnect.ts` + `reconnect.test.ts`（建议）— backoffMs 纯函数，node --test 直跑
- [ ] Go 侧扩既有文件：`internal/proto/proto_test.go`（EXIT 常量 + round-trip）、`internal/server/` lifecycle/registry-empty 测试、`cmd/wesh/main_test.go`（TestParseArgs 表行 + TestStartupMatrix 矩阵行）
- [ ] 无框架安装需求——全部基建在位

## Security Domain

### Applicable ASVS Categories（ASVS Level 1，config security_asvs_level=1）

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | 间接相关 | 重连**不绕过**认证：connect() 重入走完整 fetch ticket → Hello 核销链（无静默豁免通道）；share token URL 保留使手动/自动重 attach 均可行（P5 D-03 既定） |
| V3 Session Management | 否 | 无服务端会话状态新增（D-07 明示不引入 generation id；共享进程模型下重连 = 新 attach） |
| V4 Access Control | 是 | D-06 重连**不恢复写权限**——owner 重连按新 attach 走 P5 递补矩阵（原 owner 降级 ro 入队），零特权恢复窗口；权限不得由客户端请求获得（decideModeLocked 纪律，clients.go:308-311 注释逐字） |
| V5 Input Validation | 是 | EXIT 载荷 S→C 服务端组文案（前端**消费**不校验来源之外的注入面——message 经 textContent 渲染，showStatus 全程无 innerHTML [VERIFIED: main.ts:365-381 textContent 赋值]）；`--exit-when-empty` duration 值 time.ParseDuration fail-fast；EXIT 帧 JSON.parse 失败静默丢弃（ERROR 帧同款容错） |
| V6 Cryptography | 否 | 无新密码学面 |

### Known Threat Patterns for WS 生命周期 + 重连栈

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| EXIT 帧伪造（客户端注入终结提示误导其他用户） | Spoofing | 结构性不可能：EXIT 为 S→C 类型字节，C→S 类型空间不变（'0'/'1'/'H'），未知 C→S 字节 1002 直关（server.go:817-819 既有闸） |
| 重连循环放大流量（恶意/僵尸标签页） | DoS | D-01/D-02 边界：仅 1006 类触发、退避 1s×2 封顶 30s 无限——单标签页最坏 2 次/分钟 fetch+WS 握手，量级可忽略（CONTEXT 明示裁断）；1013 被踢端不自动重连（防再踢循环） |
| Reconnecting 面板钓鱼文案 | Tampering | hint/body 全部前端硬编码常量（UNREACHABLE_BODY/HINT_RESTART 常量化先例，05-08）；EXIT message 虽来自服务端但经 textContent 渲染无 HTML 注入面 |
| logEvent 红线 | Information Disclosure | 新事件（empty 触发/宽限启停）沿用三要素单行（remote/code/reason），token/ticket/凭据值永不入参（SEC-01 既定红线）；退出码/信号名非敏感 |
| SIGHUP 误发无关进程组 | Tampering/DoS | kill 目标 = `-sess.Cmd.Process.Pid`（setsid 组长，pgid 恒等自身 pid，P1 注释论证）；不读外部 pid；无 ttyd `-s` 式的用户可配信号面（Phase 7 才引入，届时 OPS-04 评审） |

## Sources

### Primary (HIGH confidence)
- 本 session 逐行 Read：`internal/server/server.go`（全文 1004 行）、`internal/server/clients.go`（696 行）、`internal/proto/proto.go`（198 行）、`cmd/wesh/main.go`（444 行）、`web/src/main.ts`（757 行）、`internal/pty/io.go` / `spawn.go` / `reap_linux.go`、`web/uat/phase05.mjs`（655 行）/ `phase05-dom.mjs`（头部 70 行）、`internal/proto/proto_test.go`（229 行）、README.md、ci.yml、web/package.json / tsconfig.json / uat/package.json
- GOROOT go1.26.3 源码核实：`src/flag/flag.go:146-153,350-356`（IsBoolFlag）、`src/os/exec_posix.go:155-157`（ExitCode -1 语义）、`src/os/exec.go:393-395`（Sys()）、`src/syscall/syscall_linux.go:471,486-491`（WaitStatus）、`src/syscall/zerrors_linux_amd64.go:1491-1500`（信号名表）、`src/syscall/syscall_unix.go:172`（Signal.String）
- 依赖源码核实：`coder/websocket@v1.8.15/close.go:185,199`（Close 双 5s 上界）、`@xterm/xterm` typings `clear(): void`
- git 历史：`git show cc03c79~1:internal/server/server.go:649-657`（P1 SIGHUP 逐字形态）

### Secondary (MEDIUM confidence)
- MDN Window offline_event / Navigator.onLine [CITED: developer.mozilla.org/en-US/docs/Web/API/Window/offline_event]——online/offline 事件语义与 onLine 假阳性告诫
- ttyd 官方 man 页逐字 [CITED: github.com/tsl0922/ttyd/blob/main/man/ttyd.1]——`-o, --once` / `-q, --exit-no-conn` / `-s, --signal (default: 1, SIGHUP)`
- RFC 6455 §7.4（1006 保留值）——经 in-repo proto.go:8-14 注释与 PITFALLS Pitfall 9 双重固化

### Tertiary (LOW confidence)
- 无（本 phase 无仅 WebSearch 单源支撑的实现性声明）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH——零新依赖；在库版本全部命令/文件级核实
- Architecture: HIGH——每个挂点均有本 session 逐行核实的现状代码行号 + 前序 phase 逐字先例；EXIT 写序/信号名/退出状态三处机制细节为 GOROOT/库源码级实证
- Pitfalls: HIGH——P1/P3/P8 源码级实证；P2/P4/P5/P6/P7 为既有代码结构直接推论（挂点/闭包/分派表现状逐行核实）

**Research date:** 2026-08-23
**Valid until:** 2026-09-22（30 天——全部依据为在库代码与本机工具链，无快速演进外部依赖）
