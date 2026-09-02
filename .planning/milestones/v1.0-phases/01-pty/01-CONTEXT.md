# Phase 1: 行走骨架（核心 PTY 管道）- Context

**Gathered:** 2026-08-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 1 交付行走骨架：用户运行 `wesh [flags] -- <cmd>` 后，浏览器打开页面获得一个可用的完整交互终端——PTY 双向转发 + resize 同步 + xterm.js 前端接通 + 零线程收割 + env 白名单。**Phase 1 是单次语义**：任意终结路径（子进程退出 / WS 断开）都使服务端整体退出；多客户端 attach、断线重连、会话保持不在本阶段。

**In scope (from ROADMAP):** CORE-01, CORE-02, FE-01, FE-03, SEC-06；Go module + pnpm/Vite 前端工程脚手架、go:embed 单 HTML 伺服、forkpty/setsid/exec 数组、env 白名单、最小协议帧（OUTPUT/INPUT/RESIZE）、CI 强制 `go test -race`。

**Out of scope (本阶段不做):** 认证/TLS（Phase 3）、子协议 `wesh.v1` 协商（Phase 2）、多客户端 fan-out（Phase 5）、断线重连与生命周期完整语义（Phase 6）、配置文件/base-path/降权（Phase 7）、metrics/healthz/结构化日志（Phase 8）、自动开浏览器（Phase 7 OPS-11）。

</domain>

<decisions>
## Implementation Decisions

### CLI 形态与项目落盘
- **D-01:** 项目名 wesh；仓库目录从 `stow/` 重命名为 `wesh/`；Go module path = `github.com/sworda/wesh`（对应远端 `git@github.com:sworda/wesh.git`） — **Reversibility:** one-way — module path 是 Go 生态的发布契约，改名即变更所有 import 路径与远端仓库地址；首期落地后不再改。
- **D-02:** CLI 形态 = `wesh [flags] -- <cmd> [args...]`；`--` 后原样传递给 `exec.Command(name, args...)`，绝不经 shell — **Reversibility:** one-way — CLI 是公开契约，发布后再改会破坏脚本与文档。
- **D-03:** 无命令时直接报错退出（不起登录 shell、不打 help 后跑命令），要求显式提供 `-- <cmd>`。
- **D-04:** Phase 1 仅接受 4 个 flag：`--port` / `--bind` / `--help` / `--version`。`--once` / `--read-only` / `--config` 等延后到对应 phase 再加。

### 监听默认值与启动行为
- **D-05:** 默认 bind `0.0.0.0`（用户明确选择；Phase 1 无认证，LAN 可达是已知接受的取舍，Phase 3 认证+TLS 到位后才是公网/不可信网络安全） — **Reversibility:** reversible — 默认值随时可调。
- **D-06:** 默认端口 `7681`（与 ttyd 相同，运维记忆友好）；`--port 0` 仍表示随机并打印实际端口。
- **D-07:** 启动后仅打印单行 `listening on http://host:port`；不自动开浏览器（OPS-11 延后到 Phase 7）。
- **D-08:** Phase 1 不打 ro/rw 双链接（无认证亦无 token）；Phase 5 MULTI-05 才引入含一次性 token 的分享链接。

### Phase 1 临时多客户端与生命周期（单次语义）
- **D-09:** 第二个 WS 连接 attach 时返回 HTTP 409 拒绝；本阶段不存在"两个浏览器同时看"。Phase 5 才改为多客户端 fan-out — **Reversibility:** reversible — 临时语义，Phase 5 整体重写。
- **D-10:** 子进程退出后服务端跟随退出：先给当前 WS 客户端发 1000 正常关闭帧，随后进程退出（退出码 = 子进程退出码）。Phase 6 才引入"通知所有客户端 + 按 --once/无人退出策略走"。
- **D-11:** WS 客户端主动断开或网络断开时，服务端 SIGHUP 子进程进程组后自身退出。Phase 6 才引入"断线保持进程 + 重连接回同一 PTY"。

### attach 前输出处理
- **D-12:** 首客户端 attach 前的 PTY 输出直接丢弃：服务端启动时即起一个 drain goroutine 持续读 master 并丢弃，防 64KiB PTY 内核缓冲填满导致子进程写阻塞。不做环形缓冲重放（留 Phase 6 滚动回放时再统一设计）。

### 技术决策（研究已锁定，planner 直接采用）
- **D-13:** 后端 Go 1.26 + `creack/pty` v1.1.24 + `coder/websocket` v1.8.15 + `golang.org/x/sys` v0.47.0；前端 Vite 8 + `vite-plugin-singlefile` + `@xterm/xterm` 6 + `addon-fit` + `addon-webgl`（WebGL 失败回落 DOM）。版本号以 `01-RESEARCH.md` 为准。
- **D-14:** 收割模型：Linux `cmd.Wait()`（stdlib 内建 pidfd waitid，Go ≥1.23）；darwin 共享 kqueue watcher（EVFILT_PROC/NOTE_EXIT）+ `cmd.Wait()`。**不手写** `pidfd_open`、**不**用 SIGCHLD+WNOHANG 手动 reap（两者都会与 `Wait` 争收割权）。
- **D-15:** spawn 路径：`pty.StartWithSize(24x80)` + `cmd.Env` 替换式注入白名单（TERM/COLORTERM 固定 + PATH/HOME/USER/LOGNAME/SHELL/LANG/LC_* 继承）+ `exec.Command(name, args...)` 数组形式，**绝不经 shell**。
- **D-16:** 协议形状：binary frame = 1 字节 ASCII 类型 + 载荷；C→S `'0'` INPUT（raw bytes）/ `'1'` RESIZE（JSON `{"cols","rows"}`，服务端钳制 1..1000）；S→C `'0'` OUTPUT。子协议 `wesh.v1` 协商、类型化错误帧、Hello/Welcome 握手留给 Phase 2，类型字节空间已预留。
- **D-17:** WS 数据泵：单 reader goroutine + 单 writer goroutine；不设 `InsecureSkipVerify`（默认 Origin 同源校验已够 Phase 1 用）；`SetReadLimit` 用 coder/websocket 默认 32768 字节作预认证基线；不使用 permessage-deflate。
- **D-18:** 静态资产：`//go:embed all:dist` 嵌单 HTML 及 `.gz` 旁路；构建顺序硬依赖 `pnpm -C web build` 先于 `go build`；仓库提交 `web/dist/index.html` 占位保证裸 clone 可 `go test`。

### Claude's Discretion
- 项目目录结构（`cmd/wesh/`、`internal/{proto,pty,server,web}`、`web/`）以 `01-RESEARCH.md` §Recommended Project Structure 为准，planner 可微调但保持 `proto/` 单一事实源与 `pty/` 数据面/控制面分离原则。
- CI yaml 细节（runner 镜像、node 版本等）以 `01-RESEARCH.md` §CI 提案为底，executor 按 GitHub Actions 当前实际版本微调。
- 前端脚手架文件具体内容（index.html/tsconfig.json 字段）以 `pnpm create vite` 实际生成物为准。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/PROJECT.md` — 项目定位、核心约束（单静态二进制、Linux+macOS、Go、xterm.js）、Key Decisions 表
- `.planning/REQUIREMENTS.md` — v1 44 条需求清单；Phase 1 覆盖 CORE-01/CORE-02/FE-01/FE-03/SEC-06
- `.planning/ROADMAP.md` §Phase 1 — 成功准则 3 条（交互终端可用 / resize 同步 / 收割无僵尸+env 零泄露）与研究旗帜

### 调研结论（关键技术决策已锁定）
- `.planning/research/STACK.md` — 后端 Go/前端 xterm.js 技术栈定案
- `.planning/research/ARCHITECTURE.md` — 整体架构（数据面/控制面分离、GoTTY 共享进程模型、outbox 结构为 Phase 5 预留）
- `.planning/research/PITFALLS.md` — ttyd 源码核实缺陷清单（C1 手写分片重组、C7 env 泄露、C9 1006 关闭码、C10 resize 钳制等），**本 phase 不得重蹈**
- `.planning/research/FEATURES.md` — ttyd 功能清单对照
- `.planning/research/SUMMARY.md` — 调研综合

### Phase 1 专项研究（一手实证）
- `.planning/phases/01-pty/01-RESEARCH.md` — **本 phase 最权威技术依据**。含：spawn/env 白名单/收割/数据泵/embed 全部 Pattern 代码、macOS kqueue watcher 骨架、CI yaml 提案、波次 0 缺口清单、测试映射表。**planner 的 Pattern 1-5、Anti-Patterns、Don't Hand-Roll、Common Pitfalls 章节必读**。

### ttyd 源码（缺陷对照面）
- `~/open_src/ttyd/` — ttyd 1.7.7 源码；pty.c:87/112（close(0) 缺陷）、pty.c:441-444（env 全继承）、protocol.c:288-298（预认证漏洞）、server.h:8-16（帧常量）。仅作缺陷对照，**不参考其实现方式**。

</canonical_refs>

<code_context>
## Existing Code Insights

**greenfield**：仓库当前仅有 `.planning/` 与 `.git/`，无既有代码、无可复用资产、无既定模式。所有代码本 phase 从零创建。

### Established Patterns (to-be-created this phase)

- **`internal/proto/` 单一事实源** — 帧类型字节常量、RESIZE JSON 编解码集中在 `proto.go`；前端 TS 常量手工对齐（Phase 2 子协议与错误码落同一文件）。
- **`internal/pty/` 数据面隔离** — spawn/io/reap 与会话/WS 解耦；`reap_linux.go` 仅含 `cmd.Wait()` 直达 + 注释说明 stdlib pidfd 事实；`reap_darwin.go` 是共享 kqueue watcher。
- **`internal/server/` 数据泵** — 单 reader + 单 writer goroutine，writer 独占 WS 写端（Phase 1 直写；outbox/actor 结构 Phase 5 再加）。
- **`web/` go:embed 同级目录** — `web/embed.go` 与 `web/dist/` 同目录（go:embed 硬约束不能 `../` 引用）。

### Integration Points

- 构建顺序：`pnpm -C web build` → `go build`；CI 双 job（web/go）顺序执行。
- `go:embed all:dist` 编译期硬依赖 dist 存在——仓库必须提交 `web/dist/index.html` 占位。

</code_context>

<specifics>
## Specific Ideas

- 启动单行打印格式：`listening on http://host:port`（无 banner、无 emoji、无 ASCII art）。
- 默认 bind `0.0.0.0` 是用户明确选择——虽然与"无认证应默认 loopback"的常规安全基线不同，但用户接受 Phase 1 内 LAN 可达的取舍；**CONTEXT.md 与 README 必须显式标注"Phase 1 无认证，仅在可信网络使用"**。
- 单次语义是 Phase 1 的简化路径，**不是产品形态**——README 与 help 文本要明确说"Phase 1：WS 断开即退出；断线重连在 Phase 6"。避免用户误以为是 bug。

</specifics>

<deferred>
## Deferred Ideas

- **OPS-11 启动后自动开浏览器** — 用户明确延后到 Phase 7（服务器场景通常无本地浏览器，默认打开反而错）。
- **Phase 6 完整生命周期语义** — `--once` / "所有客户端断开后退出" / 类型化终结帧（含退出码）/ 断线重连接回同一 PTY 进程 / 滚动回放。Phase 1 单次语义只是过渡。
- **Phase 2 协议扩展** — `wesh.v1` 子协议协商、Hello/Welcome 握手、类型化错误帧（spawn_failed 等）、ping/pong 保活间隔配置、三层上限完整版、5s 未认证超时、permessage-deflate 决策。类型字节空间（'2'/'3'/'E'/'X'）已在 Phase 1 预留。
- **Phase 5 多客户端** — fan-out、ro/rw 权限、慢客户端 outbox+1013 踢出、resize 仲裁、ro/rw 分享链接。Phase 1 的"第二连接 409"将被推翻。
- **Phase 3 认证与 TLS** — 一次性 ticket、`crypto/subtle`、Origin 白名单、TLS 1.2+/安全响应头。Phase 1 默认 0.0.0.0 的暴露面由 Phase 3 收口。
- **darwin kqueue 僵尸注册竞态的运行时验证** — 由 CI macos-latest leg 承担（方案见 `01-RESEARCH.md` §Open Questions Q1），兜底是 darwin 退化为 `cmd.Wait()` goroutine。

</deferred>

---

*Phase: 1-pty*
*Context gathered: 2026-08-13*
