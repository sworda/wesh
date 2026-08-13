# Phase 1: 行走骨架（核心 PTY 管道）- Research

**Researched:** 2026-08-13
**Domain:** Go PTY 服务器 + WebSocket 双向管道 + xterm.js 前端（单会话 GoTTY 共享进程模型）
**Confidence:** HIGH（后端关键路径——spawn/env 白名单/resize/收割/EIO/spawn 失败 fd 安全——已在本机 Linux 5.4 + Go 1.26.3 上原型运行验证；全部第三方库 API 经模块源码精读核实；唯一 MEDIUM 缺口是 darwin kqueue 运行时行为，已给出 CI 验证方案与兜底）

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-01 | 命令行指定任意命令及参数，浏览器获得完整交互终端（PTY 双向转发） | `pty.StartWithSize` + exec 数组 spawn（原型验证）；coder/websocket 数据泵模式（源码验证）；xterm.js `write(Uint8Array)` 二进制安全写入（官方 typings） |
| CORE-02 | 前端 resize → 服务端 TIOCSWINSZ 同步 | `pty.Setsize` = ioctl(TIOCSWINSZ) 封装（源码+本机实测：24x80→50x132 子进程 stty 确认）；addon-fit `fit()` + `onResize` 事件（官方文档） |
| FE-01 | xterm.js 6 渲染，WebGL 失败回落 DOM | 官方 demo 回落模式（try loadAddon WebglAddon → catch → dispose → DOM `open()`） |
| FE-03 | 浏览器窗口变化自动 fit | addon-fit + window resize 监听 + debounce（Pitfall 10 防抖/NaN 防护） |
| SEC-06 | 子进程环境变量白名单 | `cmd.Env = []string{...}` 在 spawn 路径一次到位（本机实测：`env` 零泄露）；白名单最小集清单见 §安全域 |
</phase_requirements>

## Summary

Phase 1 的全部技术风险点都已闭环验证。**最重要的发现：Linux 侧不需要手写任何 pidfd 代码**——Go ≥1.23 的 `os/exec` 在 Linux 5.3+ 内核上自动以 `CLONE_PIDFD` fork、以 `waitid(P_PIDFD)` 等待（本机 Go 1.26.3 源码核实：`os/pidfd_linux.go`、`syscall/exec_linux.go:311`）。因此 `pty.StartWithSize(cmd, ws)` + 每会话一个 goroutine 阻塞在 `cmd.Wait()` 就是"pidfd 收割"的正确实现：零额外线程、无僵尸、退出码完整。手动 `pidfd_open(cmd.Process.Pid)` 反而引入"谁负责 reap"的所有权冲突，明确不采用。

macOS 侧没有 pidfd，研究旗帜要求的 kqueue EVFILT_PROC/NOTE_EXIT 路径已给出**编译验证过**（darwin/amd64+arm64 均过 `go build`+`go vet`）的最小骨架：`x/sys/unix` 提供 `Kqueue/Kevent/SetKevent/Kevent_t` 及 `EVFILT_PROC/NOTE_EXIT/NOTE_EXITSTATUS/EV_ADD/EV_ONESHOT` 全部常量。遗留的平台运行时问题只有一个：**子进程在 kevent 注册之前就已退出（僵尸态）时，注册是否立即补发事件**——这决定"共享 watcher"设计是否需要竞态兜底，必须在 macOS CI runner 上用两个针对性测试早期验证（方案见 §Open Questions）。兜底方案已明确：darwin 退化为每会话 `cmd.Wait()` goroutine（阻塞 wait4，每会话占一个 M；v1 单会话规模完全可接受），**而不是** STATE.md 里写的 SIGCHLD+WNOHANG 手动 reap——手动 reap 会与 `os/exec.Wait` 争夺收割权导致退出码丢失，本研究对此做出修正。

其余路径全部有实证：env 白名单（`cmd.Env` 替换式注入，实测服务端 env 零泄露）、resize（`Setsize` 实测生效，ws_xpixel/ws_ypixel 置 0 是 ttyd 与 creack/pty 共同实践，不需要设置）、spawn 失败不误关 fd 0/1/2（实测回归 ttyd pty.c 缺陷，creack/pty 无此问题）、Linux 子进程退出后 master 读返回 EIO（实测）、coder/websocket 默认读上限 32768 字节+自动 pong+同源 Origin 默认放行（源码核实）、go:embed+预 gzip 伺服（原型运行验证）、vite-plugin-singlefile 2.3.3 显式支持 Vite 8（插件源码核实 viteMajor>=8 分支）。

**Primary recommendation:** 后端 = `pty.StartWithSize(cmd, &pty.Winsize{Rows:24, Cols:80})` + `cmd.Env` 白名单 + `cmd.Wait()` 收割 goroutine（Linux 即 pidfd）+ darwin 共享 kqueue watcher（CI 先行验证）+ coder/websocket 单 reader/单 writer 数据泵 + go:embed 预 gzip 单 HTML；前端 = Vite 8 + vite-plugin-singlefile + @xterm/xterm 6 + addon-fit + addon-webgl（失败回落 DOM）；协议 = ttyd 同构的 1 字节 ASCII 类型前缀二进制帧（'0' INPUT/OUTPUT、'1' RESIZE+JSON），子协议协商与类型化错误帧留给 Phase 2 但预留类型字节空间。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 命令 spawn（exec 数组/setsid/ctty） | 服务端 `internal/pty` | — | fork/exec 只能发生在服务端；绝不经过 shell（PITFALLS C7） |
| env 白名单 | 服务端 `internal/pty`（spawn 路径） | — | SEC-06 要求一次到位；`cmd.Env` 替换式注入即完成 |
| 子进程收割/退出码 | 服务端 `internal/pty`（reaper） | — | Linux=`cmd.Wait()`（stdlib pidfd）；darwin=kqueue watcher+Wait |
| TIOCSWINSZ | 服务端 `internal/pty` | 前端 fit（触发源） | 内核持有 winsize，前端只上报 cols/rows |
| WS 帧编解码 | 服务端 `internal/proto` | 前端同构常量 | 单一事实源；Phase 2 扩展的锚点 |
| 终端渲染/键盘采集 | 浏览器 xterm.js | — | FE-01/FE-03；`write(Uint8Array)` 二进制安全 |
| HTML 资产伺服 | 服务端 `internal/web`（go:embed） | 构建期 vite+gzip | 单 HTML + 预压缩，运行时零磁盘依赖 |
| 窗口尺寸计算 | 前端 addon-fit | 服务端钳制（1..1000） | fit 计算像素→行列；服务端防恶意/异常值（PITFALLS C10） |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.x（本机 1.26.3） | 后端语言/工具链 | 既定（STACK.md）；`os/exec` 内建 pidfd 收割需 ≥1.23，1.26 满足 [VERIFIED: 本机 `go version` 输出 `go version go1.26.3 linux/amd64`] |
| github.com/creack/pty | v1.1.24 | forkpty/setsid/ctty/TIOCSWINSZ | 既定；源码精读确认 `StartWithSize` 自动 `Setsid+Setctty`、spawn 前设初始尺寸、失败只关自己打开的 fd；**零外部依赖**（go.mod 仅 `go 1.18`）[VERIFIED: 模块缓存源码 run.go/start.go/winsize_unix.go] |
| github.com/coder/websocket | v1.8.15 | WS 服务端+测试客户端 | 既定；源码核实 `Accept(w,r,opts)`、`SetReadLimit` 默认 32768（read.go:107）、自动 pong（read.go:317-323）、并发写安全；**零外部依赖**（go.mod 仅 `go 1.23`）[VERIFIED: 模块缓存源码] |
| golang.org/x/sys | v0.47.0 | darwin kqueue 封装 | `unix.Kqueue/Kevent/SetKevent` + 全常量；darwin 双架构交叉编译验证 [VERIFIED: 本机 `GOOS=darwin go build` 通过] |
| embed + io/fs + net/http（stdlib） | 随工具链 | 单 HTML 嵌入与伺服 | 预 gzip 伺服原型本机运行验证 [VERIFIED: 本机原型] |
| @xterm/xterm | 6.0.0 | 浏览器终端模拟器 | 既定；`write(string\|Uint8Array)` 二进制安全、`onData/onResize` 事件 [CITED: Context7 xterm.js 官方 typings/demo] |
| @xterm/addon-fit | 0.11.0 | 容器自适应 | 既定；与 6.0.0 同批发布 [VERIFIED: npm registry] |
| @xterm/addon-webgl | 0.19.0 | WebGL2 渲染器 | 既定；官方回落模式见代码示例 [CITED: Context7 xterm.js demo/client.ts] |
| Vite | 8.2.1 | 前端构建 | 既定；engines 要求 Node ^20.19 \|\| ≥22.12，本机 v24.13.0 满足 [VERIFIED: npm registry + 本机 `node --version`] |
| vite-plugin-singlefile | 2.3.3 | 全部 JS/CSS inline 成单 HTML | peer 范围 `vite: ^5.4.21 \|\| ^6 \|\| ^7 \|\| ^8` 显式覆盖 Vite 8；插件源码对 viteMajor>=8 走 `codeSplitting:false` 分支 [VERIFIED: npm registry peerDependencies + CITED: Context7 插件源码] |
| TypeScript | ^5.5（或上游 7.x） | 前端语言 | 既定（STACK.md）[ASSUMED: 具体小版本由 pnpm 解析] |
| pnpm | 11.21.0（本机） | 前端包管理 | 用户既定偏好 [VERIFIED: 本机 `pnpm --version`] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| testing + `go test -race`（stdlib） | 随工具链 | 单元/集成/e2e 测试 + 竞态检测 | 全程；e2e 用 coder/websocket `Dial` 做 WS 客户端，零新增测试依赖 |
| actions/checkout | v7.0.1 | CI 检出 | CI [CITED: github.com/actions/checkout/releases/latest] |
| actions/setup-go | v7.0.0 | CI Go 工具链 | CI，`go-version-file: go.mod` [CITED: github.com/actions/setup-go/releases/latest] |
| pnpm/action-setup | v6.0.10 | CI pnpm | CI 前端 job（官方仓库已指向后继 `pnpm/setup`，v6 仍为当前最新稳定线）[CITED: github.com/pnpm/action-setup/releases/latest] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `cmd.Wait()`（Linux pidfd 收割） | 手动 `unix.PidfdOpen`+poll+`waitid` | **明确不采用**。stdlib 已内建（Go≥1.23）；手动路径与 `Wait` 争收割权，丢退出码 |
| x/sys/unix kqueue 封装 | stdlib `syscall.Kqueue/Kevent` | stdlib 在 darwin 也有（syscall_bsd.go:423，build tag 含 darwin），但无 `SetKevent` 辅助函数且包已冻结；x/sys 是官方推荐路径 [VERIFIED: GOROOT 源码核实两者均存在] |
| coder/websocket | gorilla/websocket | STACK.md 已定案：gorilla 停滞+并发写 panic 高发+默认无读上限 |
| vite-plugin-singlefile | 自写 inline 插件/多资源 embed | v1 单 HTML 最简；v2 资源膨胀时再退多资源（STACK.md 已记） |
| 预 gzip 旁路文件 | 运行时 `compress/gzip` | 终端 HTML 构建期压缩一次即可；运行时压缩白耗 CPU 且 handler 更复杂 |

**Installation:**

```bash
# 后端（仓库根目录）
go mod init github.com/sworda/wesh   # 模块名以实际远端为准
go get github.com/creack/pty@v1.1.24
go get github.com/coder/websocket@v1.8.15
go get golang.org/x/sys@v0.47.0

# 前端（web/ 目录）
pnpm add @xterm/xterm@6.0.0 @xterm/addon-fit@0.11.0 @xterm/addon-webgl@0.19.0
pnpm add -D vite@8.2.1 vite-plugin-singlefile@2.3.3 typescript
```

**Version verification:** 全部版本已于 2026-08-13 经 registry 当日核实：npm（`npm view`：@xterm/xterm 6.0.0、addon-fit 0.11.0、addon-webgl 0.19.0、vite 8.2.1）；Go 模块经 `go mod download` 拉取并精读源码（creack/pty v1.1.24、coder/websocket v1.8.15、x/sys v0.47.0）；GitHub Actions 经官方 releases 页核实（checkout v7.0.1 / setup-go v7.0.0 / pnpm action-setup v6.0.10）。

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| @xterm/xterm | npm | 8 月（2025-12-22） | 3.70M/wk | github.com/xtermjs/xterm.js | OK | Approved |
| @xterm/addon-fit | npm | 8 月（2025-12-22） | 2.72M/wk | github.com/xtermjs/xterm.js | OK | Approved |
| @xterm/addon-webgl | npm | 8 月（2025-12-22） | 1.12M/wk | github.com/xtermjs/xterm.js | OK | Approved |
| vite | npm | 最新发布 7 天（2026-08-06） | 164.3M/wk | github.com/vitejs/vite | SUS（too-new） | Approved——seam 仅因"最新版发布 <30 天"触发启发式；164M 周下载+官方仓库，误判。锁 `8.2.1`（或 `^8`，planner 决定） |
| vite-plugin-singlefile | npm | 4 月（2026-04-17） | 1.87M/wk | github.com/richardtallent/vite-plugin-singlefile | OK | Approved |
| typescript | npm | 1 月（2026-07-08） | 260M/wk | （官方） | OK | Approved |
| creack/pty | Go proxy | 2024-10-31 | 1,263 个导入模块（pkg.go.dev，STACK.md 已核） | github.com/creack/pty | OK | Approved——seam 不支持 Go ecosystem，以 `go mod download`+源码精读替代验证（更强证据） |
| coder/websocket | Go proxy | 2026-06-15 | Coder 生产使用（STACK.md 已核） | github.com/coder/websocket | OK | Approved——同上 |
| golang.org/x/sys | Go proxy | v0.47.0 | Go 官方扩展库 | go.googlesource.com/sys | OK | Approved——同上 |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** vite（已裁定为启发式误报，理由见表；planner 无需插入 checkpoint，pin 版本即可）

*全部 npm 包 postinstall 均为 null（seam signals 核实），无安装期脚本风险。*

## Architecture Patterns

### System Architecture Diagram

```
┌─ 浏览器 ─────────────────────────────────────────────┐
│ index.html（go:embed 内嵌，预 gzip）                    │
│   xterm.js 6 ──addon-fit──> onResize{cols,rows}       │
│      ▲ write(Uint8Array)        │ onData(string)      │
│      │                          ▼                     │
│   WebSocket（binary frame: [type][payload]）           │
└──────┼──────────────────────────┼─────────────────────┘
       │ OUTPUT '0'+raw bytes     │ INPUT '0'+raw bytes / RESIZE '1'+JSON
┌──────▼──────────────────────────▼─── wesh 单进程 ─────┐
│ Gateway（net/http + coder/websocket）                 │
│   Accept(Subprotocols: nil /* Phase 2 再加 wesh.v1 */)│
│   SetReadLimit 默认 32768 即基线防护                    │
│      │                       ▲                        │
│      ▼ 单 reader goroutine   │ 单 writer（Phase 1 直写）│
│   proto 解帧 ──> INPUT ──────┼──> pty master Write    │
│                └─ RESIZE ─钳制1..1000─> Setsize(TIOCSWINSZ)
│   proto 组帧 <── OUTPUT <────┼── pty master Read(32KiB 循环)
│                              │                        │
│ PTY Engine（internal/pty）    │                        │
│   StartWithSize(24x80) → Setsid+Setctty → exec 数组    │
│   cmd.Env = 白名单（SEC-06）                            │
│   Reaper：                                                │
│     linux:  goroutine cmd.Wait() ＝ stdlib pidfd waitid  │
│     darwin: 共享 kqueue watcher(NOTE_EXIT) → cmd.Wait()  │
│   退出 → drain master 至 EOF/EIO → 关 master → 关 WS(1000)│
└──────────────────────────────┼────────────────────────┘
                               ▼ fork/exec（新会话、进程组组长）
                          子进程: bash → vim/htop/...
```

数据流主路径：键盘 → onData → INPUT 帧 → master write；程序输出 → master read → OUTPUT 帧 → term.write。resize：window resize → debounce → fit() → onResize → RESIZE 帧 → 钳制 → TIOCSWINSZ → 内核自动 SIGWINCH → 远端 TUI 重绘。

### Recommended Project Structure

```
stow/
├── go.mod / go.sum
├── cmd/wesh/main.go          # CLI 解析（wesh -- <cmd> [args...]）+ 组件装配
├── internal/
│   ├── proto/                # 帧类型字节常量 + RESIZE JSON 编解码（单一事实源）
│   │   └── proto.go
│   ├── pty/                  # PTY Engine
│   │   ├── spawn.go          # StartWithSize + env 白名单 + exec 数组
│   │   ├── io.go             # master 读写循环、Setsize、EIO/EOF 归一
│   │   ├── reap_linux.go     # cmd.Wait() 直达（文档化 stdlib pidfd 事实）
│   │   └── reap_darwin.go    # 共享 kqueue watcher（EVFILT_PROC/NOTE_EXIT）
│   ├── server/               # HTTP + WS gateway、数据泵 goroutines
│   └── web/                  # （见下）——go:embed 需在资产同级或下级
├── web/
│   ├── embed.go              # package web; //go:embed all:dist
│   ├── package.json / pnpm-lock.yaml / vite.config.ts / tsconfig.json
│   ├── index.html            # vite 入口（含 #terminal 容器与 main.ts script）
│   ├── src/main.ts           # xterm 初始化 + WS 客户端 + fit/resize 回路
│   └── dist/                 # 构建产物（index.html + index.html.gz）；仓库内放占位
└── .github/workflows/ci.yml
```

**结构理由：** `proto/` 独立成单一事实源，Phase 2 的类型化错误帧/子协议常量落同一文件；`pty/` 与会话/WS 解耦（ARCHITECTURE 控制面/数据面分离）；`web/embed.go` 放 `web/` 内是因为 `//go:embed` 不能引用包目录之外的文件（`../web/dist` 非法）——这是 go:embed 的硬约束。

### Pattern 1: spawn 路径（exec 数组 + env 白名单 + 初始尺寸一次到位）

**What:** `pty.StartWithSize` 完成 openpty + fork + 子进程 setsid + 控制终端 + 初始 winsize；`cmd.Env` 替换式注入白名单；`exec.Command(name, args...)` 数组形式绝不拼 shell。
**When:** 每个会话 spawn。
**Why:** creack/pty 源码核实的行为（run.go / start.go）：

```go
// Source: 模块缓存 creack/pty v1.1.24 start.go（VERIFIED 原文）
func StartWithSize(cmd *exec.Cmd, ws *Winsize) (*os.File, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true
	return StartWithAttrs(cmd, ws, cmd.SysProcAttr)
}
```

```go
// Source: 本研究原型（本机运行验证：env 零泄露、收割干净、失败不伤 fd 0/1/2）
func spawn(argv []string) (*exec.Cmd, *os.File, error) {
	cmd := exec.Command(argv[0], argv[1:]...) // exec 数组，绝不经 shell
	cmd.Env = whitelistEnv()                  // SEC-06：替换式注入，非追加
	// 初始尺寸 80x24；首个客户端 RESIZE 到达后即刻纠正（PITFALLS C10 首帧窗口可接受）
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, nil, err // creack/pty 失败路径只关闭自己打开的 fd（实测 fd 0/1/2 完好）
	}
	return cmd, ptmx, nil
}

func whitelistEnv() []string {
	env := []string{
		"TERM=xterm-256color", // wesh 前端真实能力；OPS-04 可配置留到 Phase 7
		"COLORTERM=truecolor",
	}
	// 仅继承非机密必需项；LANG/LC_* 前缀匹配继承；其余一律丢弃
	for _, k := range []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "LANG=") || strings.HasPrefix(kv, "LC_") {
			env = append(env, kv)
		}
	}
	if _, ok := os.LookupEnv("PATH"); !ok {
		env = append(env, "PATH=/usr/local/bin:/usr/bin:/bin")
	}
	return env
}
```

**关键细节（源码核实）：** `StartWithAttrs` 只在 `c.Stdin/Stdout/Stderr == nil` 时才赋 tty——调用方不要自己设置这三个字段；它以 `defer tty.Close()` 关闭父进程的 slave 副本（子进程持有自己的 fd 0/1/2），服务端只需持有并最后关闭 master。`cmd.Dir` Phase 1 不设（继承服务端 cwd；OPS-04 在 Phase 7 可配）。

### Pattern 2: 收割（Linux 零手写 pidfd；darwin kqueue watcher + Wait）

**Linux（VERIFIED：GOROOT 源码 + 本机原型）：** `os/exec` 在 Linux 5.3+ 自动走 pidfd——

```go
// GOROOT go1.26.3 src/syscall/exec_linux.go:310-312（VERIFIED 原文）
//	if sys.PidFD != nil {
//		flags |= CLONE_PIDFD
//	}
// GOROOT go1.26.3 src/os/pidfd_linux.go: pidfdWait() 内（VERIFIED 摘要）
//	err := ignoringEINTR(func() error {
//		return unix.Waitid(unix.P_PIDFD, int(handle), &info, syscall.WEXITED, &rusage)
//	})
```

因此 Linux reaper 就是一个 goroutine 调 `cmd.Wait()`：pidfd 钉住进程（无 PID 复用竞态）、Wait 返回即已收割（无僵尸）、`*exec.ExitError` 带退出码。本机内核 5.4.241 > 5.3 ✓；更老内核 stdlib 自动回退 waitid 路径，行为不变。**不要**手动 `pidfd_open`——会产生第二个收割者与 Wait 竞争。

**darwin（编译验证，运行时待 CI 验证）：** 共享单 kqueue watcher，每会话注册 `EVFILT_PROC/NOTE_EXIT`，事件到达后仍由 `cmd.Wait()` 收割（保持 os/exec 状态机一致）：

```go
//go:build darwin

// Source: 本研究骨架（GOOS=darwin amd64+arm64 交叉编译+vet 通过；运行时在 macOS CI 验证）
import "golang.org/x/sys/unix"

// 进程级共享 watcher：一个 kqueue fd、一个 goroutine，N 会话共用（零每会话线程）
type exitWatcher struct {
	kq   int
	mu   sync.Mutex
	subs map[int]chan<- struct{} // pid -> notify
}

func newExitWatcher() (*exitWatcher, error) {
	kq, err := unix.Kqueue() // x/sys/unix v0.47.0 zsyscall_darwin（VERIFIED 存在）
	if err != nil {
		return nil, err
	}
	w := &exitWatcher{kq: kq, subs: map[int]chan<- struct{}{}}
	go w.loop()
	return w, nil
}

func (w *exitWatcher) watch(pid int) (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	w.mu.Lock()
	w.subs[pid] = ch
	w.mu.Unlock()
	// EV_ADD 注册；EV_ONESHOT 触发一次即自动注销
	ev := []unix.Kevent_t{{
		Ident:  uint64(pid),
		Filter: unix.EVFILT_PROC,          // = -0x5（VERIFIED: zerrors_darwin）
		Flags:  unix.EV_ADD | unix.EV_ONESHOT,
		Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS, // 0x80000000|0x4000000
	}}
	_, err := unix.Kevent(w.kq, ev, nil, nil) // 非阻塞注册
	return ch, err
}

func (w *exitWatcher) loop() {
	events := make([]unix.Kevent_t, 8)
	for {
		n, err := unix.Kevent(w.kq, nil, events, nil) // 阻塞取事件（单 goroutine 占一个 M）
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return // TODO: slog.Error；Phase 1 进程级致命即可
		}
		for i := 0; i < n; i++ {
			if events[i].Fflags&unix.NOTE_EXIT == 0 {
				continue
			}
			pid := int(events[i].Ident)
			w.mu.Lock()
			ch, ok := w.subs[pid]
			delete(w.subs, pid)
			w.mu.Unlock()
			if ok {
				ch <- struct{}{}
			}
		}
	}
}

// 会话侧：通知到达后 Wait 收割（唯一收割者，退出码完整）
func awaitExit(cmd *exec.Cmd, exited <-chan struct{}) error {
	<-exited          // kqueue 早知退出
	return cmd.Wait() // 立即返回：wait4 收割 + *exec.ExitError
}
```

**注册竞态（必须 CI 验证，见 Open Questions）：** 若子进程在 `kevent(EV_ADD)` 之前已退出成僵尸，NOTE_EXIT 是否补发未在任何官方文档确认 [ASSUMED]。测试方案与兜底见 §Open Questions Q1。

### Pattern 3: WS 数据泵（单 reader + 单 writer + 帧编解码）

**What:** coder/websocket 约束（源码核实）：`Reader/Read` 不可并发（单 reader goroutine）；其余方法均并发安全；收到 ping 自动回 pong（read.go:317-323 `case opPing: ... return c.writeControl(ctx, opPong, b)`）；`Accept` 后不得再用 `r.Context()`（官方 README：hijack 后行为意外，用 `context.Background()` 派生）。

```go
// Source: API 签名 VERIFIED（coder/websocket v1.8.15 accept.go:102 / write.go:43 / read.go:41）
func serveWS(w http.ResponseWriter, r *http.Request, sess *session) {
	// 不設 InsecureSkipVerify：默认同源（请求 Host）自动放行，跨源拒绝。
	// Phase 1 页面与 WS 同 server 同 origin，默认即安全；Origin 白名单属 Phase 3。
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Subprotocols 留空：wesh.v1 协商是 Phase 2 范围（STATE.md 已锁定）
		// CompressionMode 默认 CompressionDisabled：终端高熵数据压缩无收益（PITFALLS 性能表）
	})
	if err != nil {
		return // Accept 失败已自动写 HTTP 错误响应
	}
	defer c.CloseNow()
	// SetReadLimit 默认 32768（read.go:107 VERIFIED），超限自动 1009 关闭——Phase 1 免设
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// S→C 方向：本 goroutine 独占写端（Phase 1 单客户端，无 outbox——outbox 属 Phase 3/5 结构）
	go func() {
		buf := make([]byte, 32*1024)
		frame := make([]byte, 1+32*1024)
		frame[0] = proto.Output // '0'
		for {
			n, err := sess.master.Read(buf)
			if n > 0 {
				copy(frame[1:], buf[:n])
				if werr := c.Write(ctx, websocket.MessageBinary, frame[:1+n]); werr != nil {
					return
				}
			}
			if err != nil { // Linux EIO / darwin EOF——统一视为输出终结
				return
			}
		}
	}()

	// C→S 方向：单 reader
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			var ce websocket.CloseError
			if errors.As(err, &ce) { /* 对端关闭，正常清理 */ }
			return
		}
		if len(data) == 0 {
			continue
		}
		switch data[0] {
		case proto.Input: // '0'
			sess.master.Write(data[1:])
		case proto.Resize: // '1' + JSON {"cols":C,"rows":R}
			var rs struct{ Cols, Rows uint16 }
			if json.Unmarshal(data[1:], &rs) == nil {
				sess.resize(rs.Cols, rs.Rows) // 内部钳制 1..1000（PITFALLS C10）
			}
		default:
			c.Close(websocket.StatusProtocolError, "unknown frame type") // 1002
		}
	}
}
```

### Pattern 4: go:embed + 构建期预 gzip 伺服

**What:** `//go:embed all:dist` 嵌单 HTML 及其 .gz 旁路；按 `Accept-Encoding` 直发预压缩产物。本机原型运行验证（gzip 协商与明文回退双路径均正确）。

```go
// Source: 本研究原型（本机运行验证）
//go:embed all:dist
var dist embed.FS

func handler() (http.Handler, error) {
	sub, err := fs.Sub(dist, "dist") // 剥掉 dist/ 前缀
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			if data, err := fs.ReadFile(sub, name+".gz"); err == nil {
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(data)
				return
			}
		}
		http.FileServerFS(sub).ServeHTTP(w, r)
	}), nil
}
```

**构建顺序（硬依赖）：** `pnpm -C web build`（产出 dist/index.html + .gz）→ `go build`。`//go:embed all:dist` 在 dist 不存在时**编译失败**——仓库内需提交 `web/dist/index.html` 占位文件保证裸 clone 后 `go test` 可编译（CI 中前端构建先于 Go 步骤覆盖它）。

### Pattern 5: 前端最小接入（xterm 6 + fit + WebGL 回落 + resize 回路）

```typescript
// Source: xterm.js 官方 README/typings/demo（Context7 引用）+ 本研究协议设计
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';

const term = new Terminal({ fontSize: 14, scrollback: 10000 });
const fit = new FitAddon();
term.loadAddon(fit);
term.open(document.getElementById('terminal')!);

// FE-01：WebGL 失败回落 DOM（xterm.js 官方 demo 模式）
try {
  term.loadAddon(new WebglAddon());
} catch (e) {
  console.warn('webgl addon load failed, stay on DOM renderer', e);
}

const ws = new WebSocket(`ws://${location.host}/ws`);
ws.binaryType = 'arraybuffer';

const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31; // '0' '0' '1'（与 proto/ 单一事实源对齐）

// S→C：OUTPUT 二进制帧直写（write 接受 Uint8Array，二进制安全）
ws.onmessage = (ev) => {
  const buf = new Uint8Array(ev.data as ArrayBuffer);
  if (buf[0] === OUTPUT) term.write(buf.subarray(1));
};

// C→S：键盘输入（CJK/IME 由 xterm 内部 composition 处理，onData 交付最终字符串）
const enc = new TextEncoder();
term.onData((s) => {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(concat([INPUT], enc.encode(s)));
  }
});

// FE-03 + CORE-02：窗口 resize → debounce → fit → onResize → RESIZE 帧
function sendResize(cols: number, rows: number) {
  ws.send(concat([RESIZE], enc.encode(JSON.stringify({ cols, rows }))));
}
term.onResize(({ cols, rows }) => sendResize(cols, rows));
let timer: number;
window.addEventListener('resize', () => {
  clearTimeout(timer);
  timer = setTimeout(() => {
    fit.fit(); // display:none 时 proposeDimensions 返回无效值——PITFALLS C10，需防护
    // onResize 仅在尺寸实际变化时触发；NaN 防护：
    if (!Number.isInteger(term.cols) || term.cols <= 0) return;
  }, 100);
});

ws.onopen = () => { fit.fit(); sendResize(term.cols, term.rows); term.focus(); };
```

### Anti-Patterns to Avoid

- **手动 pidfd_open + poll + waitid：** 与 `os/exec.Wait` 双收割者竞争，丢退出码；Go≥1.23 的 Wait 在 Linux 已是 pidfd waitid。直接用 Wait。
- **SIGCHLD handler + 手动 waitpid 循环（STATE.md 原兜底文案）：** 同样与 Wait 争收割权（Wait 将报 `no child processes` 并丢失退出状态）；Go 里正确的兜底是"退回 `cmd.Wait()` goroutine"。本研究修正该兜底表述。
- **经 shell 启动（`sh -c strings.Join(argv, " ")`）：** 注入面+引号地狱；`exec.Command(name, args...)` 数组直达（PITFALLS C7）。
- **env 追加式注入（`append(os.Environ(), ...)`）：** ttyd 同款泄露（pty.c:441-444 全继承已核实）；必须替换式 `cmd.Env = 白名单`。
- **InsecureSkipVerify: true：** Phase 1 页面与 WS 同源，coder/websocket 默认"请求 Host 自动放行"已够用；开了等于自拆 CSWSH 防线。
- **为 ping/pong 造应用层心跳：** 库内建自动 pong；CORE-06 的保活间隔属 Phase 2（用 `c.Ping(ctx)` 周期调用即可，勿自造帧）。
- **r.Context() 贯穿 WS 生命周期：** hijack 后该 context 行为不符合直觉（官方 README 明示）；用 `context.Background()` 派生。
- **前端 `term.write` 不分块猛灌：** xterm WriteBuffer 超 ~50MB 直接 throw 丢数据（PITFALLS C4）；Phase 1 无回放，实时流自然分块即可，回放分块流控属 Phase 6。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WS 帧解析/分片重组/mask/UTF-8 校验 | 手写帧层 | coder/websocket | ttyd 两个预认证严重漏洞都在手写重组里（PITFALLS C1）；库内建 32KB 读上限+1009 合规关闭 |
| forkpty/setsid/控制终端/初始 winsize | 手动 openpty+ForkExec | `pty.StartWithSize` | setsid/ctty/winsize 顺序错一步就是疑难 bug；creack/pty 是 1,263 模块在用的薄封装 |
| Linux 子进程退出感知 | 手动 pidfd_open/poll | `cmd.Wait()`（stdlib 内建 pidfd） | Go 1.23+ 内建；手写即引入收割权竞争 |
| darwin 子进程退出感知 | 每会话 wait4 线程（ttyd 模式） | 共享 kqueue watcher（本研究骨架） | ttyd 每进程一线程是 UAF 温床（pty.c:483 已核实） |
| 终端模拟/解析转义序列 | 任何自写 VT 解析 | @xterm/xterm | VS Code 同款；自写是巨大攻击面（ARCHITECTURE §2.4） |
| 构建期压缩 | 运行时 gzip middleware | `gzip -k9` 构建产物 + Content-Encoding 协商 | HTML 静态不变，运行时压缩纯浪费（原型已验证双路径） |
| ping/pong 保活 | 应用层心跳帧 | WS 原生 ping（`c.Ping`，库自动 pong） | ARCHITECTURE §2.8：双向用 WS 原生 ping/pong，不造应用层心跳 |

**Key insight:** 本 phase 的"自主代码"面积极小——spawn/reap/pump/帧/embed 每个环节都有经过生产验证的专用组件；自写任何一块都是在库已解决的问题上重新造错（ttyd 的全部重大漏洞都源于此）。

## Common Pitfalls

### Pitfall 1: spawn 失败误关服务端自身 fd（ttyd close(0) 缺陷）

**What goes wrong:** ttyd `process_init` memset 全零（pty.c:87）后，若 spawn 在 `process->pty = master`（pty.c:476）之前失败，`process_free` 的 `close(process->pty)`（pty.c:112）会 close(0)——服务端 stdin 被关。
**Why it happens:** 用"零值"表示"未初始化"，close 路径不区分。
**How to avoid:** creack/pty 无此缺陷（失败路径只 close 自己打开的 ptmx，本机原型实测 spawn 失败后 fd 0/1/2 全部完好）；wesh 自身代码遵守同一纪律：**只关闭成功打开且登记在册的 fd**，master 用"有效指针判空"而非零值推断。回归测试已列入验证架构（spawn 失败注入后 Fsync 探测 0/1/2）。
**Warning signs:** spawn 失败路径出现裸 `close(fd)`；日志在 spawn 失败后断流。

### Pitfall 2: resize 风暴与无效尺寸

**What goes wrong:** 拖拽窗口产生高频 resize → 高频 TIOCSWINSZ → SIGWINCH 风暴，远端 TUI 疯狂重绘；`display:none` 时 fit 的 `proposeDimensions` 返回无效值（NaN），上报后终端尺寸错乱。
**Why it happens:** 前端直报每一次窗口事件；元素不可见时像素测量无意义。
**How to avoid:** 前端 debounce ~100ms + 发送前 `Number.isInteger && >0` 防护（PITFALLS C10）；服务端钳制 cols/rows 到 [1,1000]；相同尺寸不重设（onResize 本身只在变化时触发）。
**Warning signs:** 拖窗口时远端 vim 闪屏；服务端日志出现 0 或超大尺寸。

### Pitfall 3: Linux EIO vs darwin EOF 的 master 读终结分歧

**What goes wrong:** 子进程退出且 slave 无持有者后，Linux master read 返回 EIO（本机实测：`read /dev/ptmx: input/output error`），darwin/BSD 返回 0/EOF。
**Why it happens:** 平台 PTY 实现差异（ARCHITECTURE §2.2）。
**How to avoid:** 读循环把**任何 read 错误**（含 io.EOF 与 EIO）统一归一为"输出终结"——Pattern 3 的 `if err != nil { return }` 已覆盖；不要用 `err == io.EOF` 单判。
**Warning signs:** darwin 上子进程退出后 WS 不收尾（挂在读循环）。

### Pitfall 4: 会话 leader 退出后孙进程持有 slave 导致 master 永不 EOF

**What goes wrong:** `wesh -- bash` 中 bash 退出但某后台孙进程（忽略 SIGHUP）仍持有 slave → master 读不到 EOF → drain 阶段挂死。
**Why it happens:** PTY master 的 EOF 条件是"所有 slave 打开者关闭"，不是"直接子进程退出"。
**How to avoid:** Phase 1：`Wait()` 返回后启动**带时限的 drain**（如 200ms 或读到 EOF 先到为准），随后无条件 close(master)（残留孙进程下次读写 slave 得 EIO 自然消亡）。进程组信号（OPS-04 可配关闭信号）在 Phase 7 细化。
**Warning signs:** 子进程已退出但服务端不退出/WS 不关闭。

### Pitfall 5: `go test -race` 需要 CGO，与静态构建纪律冲突

**What goes wrong:** race detector 依赖 cgo；若在 CI 里沿用发布构建的 `CGO_ENABLED=0`，`go test -race` 直接报错或静默跳过。
**Why it happens:** 把发布构建的环境变量原样搬进测试 job。
**How to avoid:** CI 测试 job **不设** `CGO_ENABLED=0`（ubuntu/macos runner 自带 gcc/clang，默认 CGO_ENABLED=1 即可 -race）；`CGO_ENABLED=0` 只出现在 Phase 9 的 goreleaser 发布构建。creack/pty 与 coder/websocket 均纯 Go 无 cgo，-race 构建无障碍。
**Warning signs:** CI 日志里 `-race` 报 "requires cgo" 或测试数异常少。

### Pitfall 6: go:embed 编译期硬依赖 dist 存在

**What goes wrong:** 裸 clone 仓库直接 `go build/test`，`web/dist/` 不存在 → `//go:embed all:dist` 编译失败，报"pattern all:dist: no matching files found"。
**Why it happens:** embed 是编译期指令，不等待前端构建。
**How to avoid:** 提交占位 `web/dist/index.html`（内容注明"前端未构建，请先 pnpm -C web build"）；CI 顺序：前端构建 → Go 测试/构建。
**Warning signs:** 新贡献者第一步 `go test ./...` 就挂。

### Pitfall 7: coder/websocket 读路径并发调用

**What goes wrong:** 两个 goroutine 同时 `c.Read` → 库明确不允许（"All methods may be called concurrently except for Reader and Read"，conn.go 注释核实），帧流错乱。
**How to avoid:** 单 reader goroutine 模式（Pattern 3）；`Ping` 必须在另一 goroutine 调用、由读路径交付 pong（官方 conn.go 注释）。Phase 1 无主动 ping（Phase 2 保活），记住届时 Ping 与 Read 必须并发存在。

## Code Examples

### spawn + 白名单 + 收割 + resize（Linux 全链路，本机运行验证）

见 Pattern 1/2。原型实测输出（2026-08-13，本机 Linux 5.4.241 + Go 1.26.3 + creack/pty v1.1.24）：

```
child pid: 3977743
wait err: <nil>
env leaked keys: []                 # SEC-06：服务端 env 零泄露
child saw TERM: true
reaped: OK (no /proc entry)         # Wait 收割干净，无僵尸
stty before/after resize: [24 80 50 132]  # Setsize → TIOCSWINSZ → 子进程可见
post-exit read err: read /dev/ptmx: input/output error is EIO: true
spawn-fail err: true                # 不存在二进制：Start 返回错误
fd 0 alive: true / fd 1 alive: true / fd 2 alive: true   # ttyd close(0) 缺陷回归通过
```

### WS 帧处理与数据泵

见 Pattern 3（API 签名逐行核自 coder/websocket v1.8.15 源码）。

### darwin kqueue watcher

见 Pattern 2（`GOOS=darwin GOARCH=amd64/arm64 go build` + `go vet` 通过；运行时行为见 Open Questions Q1 的 CI 验证方案）。

### go:embed 预 gzip

见 Pattern 4（本机运行验证：gzip 协商返回 54 字节压缩体+`Content-Encoding: gzip`；无 gzip 客户端回退明文）。

### 最小协议帧（本研究提案，Phase 1 范围）

数据面统一为 **binary frame：1 字节 ASCII 类型 + 载荷**（ttyd 同构、零解析开销；ttyd 常量在 server.h:8-16 核实：`#define INPUT '0'` / `#define RESIZE_TERMINAL '1'` / `#define OUTPUT '0'`，wesh 自定义取值不背兼容包袱）：

| 方向 | 类型字节 | 名称 | 载荷 | 说明 |
|---|---|---|---|---|
| C→S | `'0'` (0x30) | INPUT | raw bytes | 键盘输入，写 master |
| C→S | `'1'` (0x31) | RESIZE | JSON `{"cols":C,"rows":R}` | 服务端钳制 1..1000 后 Setsize |
| S→C | `'0'` (0x30) | OUTPUT | raw bytes | master 读块直发 |

**为 Phase 2 预留的扩展点（现在不实现）：**
- 类型字节空间开放：`'2'` PAUSE / `'3'` RESUME（ttyd 语义沿用）、`'E'` ERROR+JSON 类型化错误帧（spawn_failed 等）、`'X'` EXIT+退出码（SESS-03，Phase 6）——Phase 1 收到未知类型即 1002 关闭，协议演化无歧义。
- 子协议 `wesh.v1` 协商：`AcceptOptions.Subprotocols` 一行开启，Phase 2 一次到位（STATE.md 锁定）。
- JSON 文本控制帧（Hello/Welcome）：Phase 2 握手层；Phase 1 无握手，Accept 即接入。
- 关闭码：Phase 1 只主动发 1000（子进程退出后正常关）与 1002（未知帧）；1009 由 SetReadLimit 默认自动产生；**1006 永不写入**（RFC6455 §7.4，PITFALLS C9）。

### 前端脚手架文件（ASSUMED：按 create-vite vanilla-ts 惯例，字段以 pnpm 实际生成/插件文档为准）

```jsonc
// web/package.json
{
  "name": "wesh-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build && gzip -k -9 -f dist/index.html" // 产出 .gz 旁路供 embed 直发
  }
}
```

```typescript
// web/vite.config.ts —— Context7 引用插件官方 README/源码
import { defineConfig } from 'vite';
import { viteSingleFile } from 'vite-plugin-singlefile';

export default defineConfig({
  plugins: [viteSingleFile()], // 默认即全量 inline（assetsInlineLimit=()=>true, cssCodeSplit=false）
                               // 插件源码对 viteMajor>=8 自动走 codeSplitting:false 分支
});
```

### CI（GitHub Actions，本研究提案）

```yaml
# .github/workflows/ci.yml
name: ci
on: [push, pull_request]
jobs:
  go:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]   # darwin leg 同时承担 kqueue 运行时验证
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: actions/setup-go@v7.0.0
        with:
          go-version-file: go.mod
      - run: go vet ./...
      # 注意：不设 CGO_ENABLED=0——-race 需要 cgo（Pitfall 5）
      - run: go test -race -count=1 ./...
  web:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: pnpm/action-setup@v6.0.10     # 读取 package.json packageManager 字段
      - uses: actions/setup-node@v4         # [ASSUMED: 主版本号，随 runner 默认 node 亦可]
        with:
          node-version: 24
      - run: pnpm -C web install --frozen-lockfile
      - run: pnpm -C web build              # tsc 类型检查 + vite 构建一体
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 手动 pidfd_open+poll+waitid 收割 | `os/exec` Wait 内建 pidfd（CLONE_PIDFD + waitid P_PIDFD） | Go 1.23（2024-08） | Linux 收割零手写代码；本 phase 直接受益 |
| xterm.js canvas 渲染器 + 三选回落 | v6 删除 canvas，WebGL/DOM 二选 | 6.0.0（2025-12-22） | ttyd 的 `canvas` 兼容选项过时；回落逻辑只需 WebGL→DOM |
| Vite webpack 内核 | Vite 8 rolldown（Rust）内核；`inlineDynamicImports` → `codeSplitting:false` | Vite 8 | vite-plugin-singlefile 2.3.3 已适配（源码核实 viteMajor>=8 分支） |
| gorilla/websocket | coder/websocket | gorilla 2022 归档、后近乎停滞 | STACK.md 已定案 |
| 每子进程 waitpid 线程（ttyd） | pidfd/kqueue 事件化收割 | Linux 5.3+/本 phase | N 会话 N 线程 → 零额外线程 |

**Deprecated/outdated:**
- `execCommand('copy')`（浏览器废弃）→ Phase 4 用 `navigator.clipboard`（PITFALLS 已记）
- xterm.js 旧包名 `xterm`/`xterm-addon-*`（5.4 起迁 `@xterm` scope，防 typosquatting）→ 只装 `@xterm/*`
- SIGCHLD+WNOHANG 手动 reap 兜底（STATE.md 文案）→ 与 os/exec Wait 争收割权，修正为"退回 cmd.Wait() goroutine"

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | darwin kqueue 对"注册前已退出的僵尸子进程"是否补发 NOTE_EXIT——无官方文档确认，行为未定 | Pattern 2 / Open Questions Q1 | 若补发：共享 watcher 直接成立；若不补发：需加注册后即时探测或退回 cmd.Wait() goroutine。CI 两个测试先行裁决，不阻塞 Linux 开发 |
| A2 | 前端脚手架具体文件内容（index.html/tsconfig.json 字段）按 create-vite vanilla-ts 惯例 | Code Examples | 低：executor 以 `pnpm create vite` 实际生成物为准微调 |
| A3 | actions/setup-node@v4 主版本号 | CI 示例 | 低：CI 微调即可；或用 runner 预装 node |
| A4 | `wesh`（无 `--` 与命令）的默认行为（启动登录 shell？报错？）未在需求中定义 | — | 低：planner 决策点，建议 Phase 1 要求显式命令或默认 `$SHELL`（白名单内继承值） |

**除 A1 外无高风险假设。** A1 已通过"CI 先行测试 + 明确兜底"消化，无需用户额外确认。

## Open Questions (RESOLVED)

1. **darwin kqueue 僵尸注册竞态（A1）——早期原型验证方案**
   - What we know: API 面完整且交叉编译通过；`NOTE_EXIT` 语义（子进程退出即触发）与 `NOTE_EXITSTATUS`（附带退出码）常量齐全；`cmd.Wait()` 作为唯一收割者与 watcher 通知不冲突。
   - What's unclear: 子进程在 `kevent(EV_ADD)` 注册**之前**已退出（僵尸态）时，注册调用是否立即交付事件。短命令（`wesh -- true`、测试用例）必踩此窗口。
   - Recommendation: CI macos-latest leg 加两个针对性测试，**安排在 darwin watcher 实现任务的同一天**：(a) 正常路径——spawn `sleep 0.1 && exit 42`，注册后等事件，断言事件到达 + `Wait` 返回退出码 42；(b) 竞态路径——spawn `/usr/bin/true`，**先 sleep 200ms 确保已退出成僵尸，再注册**，kqueue 带 1s 超时等事件。若 (b) 事件到达 → 共享 watcher 无竞态成立；若超时 → 兜底启用：darwin `awaitExit` 退化为直接 `cmd.Wait()`（每会话一个阻塞 goroutine，v1 单会话可接受），watcher 代码以 build tag 保留待 Phase 5 多会话时再评估。
   - **→ RESOLVED：** 由 plan 01-04 Task 2 落地——CI macos-latest leg 双测试（`TestKqueueExitNormal` / `TestKqueueExitZombieRace`）裁决，竞态测试超时分支为 `t.Skip` + 裁决标记打印（兜底退化预先写入任务，两条出路均为计划内路径）。

2. **无客户端期间 PTY 输出的丢弃语义**
   - What we know: GoTTY 共享进程模型下 PTY 随服务端启动（STATE.md 锁定）；Phase 1 无 ring 缓冲（回放属 Phase 6）。
   - What's unclear: 首个客户端 attach 之前 master 读到的输出（如启动横幅）无处投递。
   - Recommendation: Phase 1 明确"attach 前输出直接丢弃"并写进 README；读循环从 spawn 起持续 drain（防子进程写满 64KiB PTY 缓冲阻塞）。
   - **→ RESOLVED：** 由 CONTEXT.md D-12 锁定——attach 前输出直接丢弃，服务端装配（`server.New`）时即启动 drain 读循环持续读 master；启动点接线与行为断言见 plan 01-01 Task 2（acceptance_criteria 源码断言 + `TestDrainBeforeAttach`）。

3. **默认 PTY 初始尺寸**
   - 首个 RESIZE 到达前用 80x24 硬编码（PITFALLS 技术债表认可"首帧窗口可接受"）；可配置属 Phase 7（OPS-04）。前端 attach 后立刻 fit+sendResize（Pattern 5 的 `ws.onopen`），窗口极短。
   - **→ RESOLVED：** 由 CONTEXT.md D-15 锁定——初始尺寸 24x80（`pty.StartWithSize` Rows:24, Cols:80），首个 RESIZE 到达即纠正；落地见 plan 01-01 Task 2 spawn 路径。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 工具链 | 全部后端 | ✓ | 1.26.3 linux/amd64 | — |
| Linux 内核 ≥5.3（pidfd_open） | stdlib pidfd 收割 | ✓ | 5.4.241 | stdlib 自动回退 waitid（行为等价，无需代码分支） |
| Node.js | 前端构建 | ✓ | v24.13.0（Vite 8 要求 ≥22.12 ✓） | — |
| pnpm | 前端包管理 | ✓ | 11.21.0 | — |
| git / gh | 版本控制/CI | ✓ | 2.39.3 / 2.8.9 | — |
| macOS 机器 | darwin kqueue **运行时**验证 | ✗ | — | GitHub Actions macos-latest runner（CI 方案已备，Open Questions Q1） |
| 外部服务（DB/Redis/Docker 等） | — | 无依赖 | — | — |

**Missing dependencies with no fallback:** 无阻塞项——darwin 运行时验证由 CI 承担，属于计划内路径。
**Missing dependencies with fallback:** macOS 本机（fallback=CI runner）。

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `-race`（后端）；`vite build` 内 tsc 类型检查（前端）；e2e 用 coder/websocket `Dial` 客户端（零新增测试依赖） |
| Config file | none（go test 零配置） |
| Quick run command | `go test ./... -count=1` |
| Full suite command | `go test -race -count=1 ./... && pnpm -C web build` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CORE-01 | spawn 任意命令+双向转发 | e2e（Go）：启动 server `wesh -- /bin/cat`，`Dial` 连 WS，发 INPUT 帧，断言收到同字节 OUTPUT 帧 | `go test ./internal/server -run TestEchoPTY -count=1` | ❌ Wave 0 |
| CORE-01 | exec 数组不经 shell | unit：断言 argv 原样传递（如命令含 `$(id)` 不被展开） | `go test ./internal/pty -run TestExecArrayNoShell` | ❌ Wave 0 |
| CORE-02 | resize 同步 TIOCSWINSZ | 集成：spawn `stty size; sleep 1; stty size`，中途 `Setsize(50,132)`，断言输出含 `50 132`（本研究原型已验证该逻辑成立） | `go test ./internal/pty -run TestResize -count=1` | ❌ Wave 0 |
| SEC-06 | env 白名单 | unit（白名单构造函数：注入 `AWS_SECRET_ACCESS_KEY` 到宿主 env，断言结果不含）+ e2e（spawn `/usr/bin/env`，断言输出无宿主变量） | `go test ./internal/pty -run TestEnvWhitelist` | ❌ Wave 0 |
| SEC-06/Pitfall 1 | spawn 失败不伤 fd | unit：`pty.Start` 不存在二进制，断言 err 非 nil 且 fd 0/1/2 `Fsync` 非 EBADF | `go test ./internal/pty -run TestSpawnFailKeepsStdio` | ❌ Wave 0 |
| （成功准则3） | 收割无僵尸 | 集成：spawn 短命令退出后断言 `/proc/<pid>` 消失（linux）；darwin leg 由 kqueue 测试覆盖等效语义 | `go test ./internal/pty -run TestReap` | ❌ Wave 0 |
| （研究旗帜） | darwin kqueue 竞态 | CI-only 双测试（Open Questions Q1 方案 a/b） | `go test ./internal/pty -run TestKqueue -count=1`（macos runner） | ❌ Wave 0 |
| FE-01 | WebGL→DOM 回落 | 手动（end-of-phase human verify）：浏览器 DevTools 禁 WebGL 后页面仍渲染 | — | 手动 checklist |
| FE-03 | 窗口变化自适应 | 手动：拖动窗口，远端 `stty size` 跟随变化；vim 重绘正常 | — | 手动 checklist |

### Sampling Rate

- **Per task commit:** `go test ./... -count=1`（秒级）
- **Per wave merge:** `go test -race -count=1 ./...` + `pnpm -C web build`
- **Phase gate:** 全量绿 + CI 双平台绿 + 手动 FE checklist 通过后才进 `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `go.mod` / `cmd/wesh/main.go` 骨架——greenfield 从零创建
- [ ] `internal/pty/{spawn_test,io_test,reap_test}.go`——覆盖 CORE-01/CORE-02/SEC-06/收割
- [ ] `internal/server/e2e_test.go`——WS echo 端到端（httptest.Server + websocket.Dial）
- [ ] `web/` 前端工程（package.json/vite.config.ts/index.html/src/main.ts）+ `web/dist/index.html` 占位
- [ ] `.github/workflows/ci.yml`
- [ ] 框架安装：无需安装——Go stdlib testing 零依赖；前端依赖由 `pnpm install --frozen-lockfile` 落地

## Security Domain

### Applicable ASVS Categories（ASVS L1 基线）

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no（Phase 1 无认证，Phase 3 一次性 ticket） | **补偿控制：默认绑定 127.0.0.1**（loopback 仅本机可达）；监听非 loopback 时启动打醒目警告（PITFALLS 安全表"明文 HTTP 无警告"同款纪律） |
| V3 Session Management | no | Phase 3 |
| V4 Access Control | no（Phase 2 默认只读 CORE-04） | — |
| V5 Input Validation | **yes** | RESIZE 帧 JSON 解析失败即丢弃 + cols/rows 钳制 [1,1000]；未知帧类型 1002 关闭；`SetReadLimit` 默认 32768 字节上限（超限自动 1009）构成预认证基线防护——三层上限的完整版在 Phase 2 |
| V6 Cryptography | no（Phase 1 无 TLS，loopback 明文） | Phase 3 TLS1.2+/安全响应头；本 phase 文档明示"非 loopback 部署请套 TLS 反代" |
| V10 Malicious Code / 命令注入 | **yes** | `exec.Command(name, args...)` 数组形式，**绝不经 shell**；无 `?arg=` 类 URL 传参（Out of Scope 已锁定） |
| V14 Configuration | **yes** | SEC-06 env 白名单：`cmd.Env` 替换式注入（TERM/COLORTERM 固定 + PATH/HOME/USER/LOGNAME/SHELL/LANG/LC_* 继承），web shell `env` 不得出现服务端其余变量（e2e 断言） |

### Known Threat Patterns for Go PTY+WS stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 命令注入（shell 拼接 argv） | Tampering | exec 数组；代码评审红线：出现 `sh -c`/`bash -c` 即打回 |
| 服务端密钥经 env 泄露进 web shell | Information Disclosure | SEC-06 白名单（本 phase 一次到位）+ e2e `env` 审计测试 |
| CSWSH（跨站 WS 劫持） | Elevation of Privilege | coder/websocket 默认 Origin 校验（同 Host 放行、跨源拒绝）；**禁用** `InsecureSkipVerify`；loopback 默认绑定 |
| 预认证消息内存放大 | DoS | `SetReadLimit` 默认 32768（read.go:107 核实）+ 认证前无缓冲分配（Phase 1 无认证亦无缓冲；Phase 2 三层上限补齐） |
| spawn 失败破坏服务端 stdio | DoS | 只关自己打开的 fd（creack/pty 已合规）+ fd 0/1/2 回归测试 |
| 僵尸进程耗尽进程表 | DoS | `cmd.Wait()` 唯一收割者（Linux 内建 pidfd waitid）；darwin kqueue watcher+Wait；高频建销压测留 Phase 9 |
| 非法关闭码引发代理/客户端分歧 | Tampering | 只发 1000/1002；1009 由库自动；1006 永不写入（RFC6455 §7.4） |

## Sources

### Primary（HIGH 置信——本机一手验证）

- 本机原型运行（2026-08-13，Linux 5.4.241 / Go 1.26.3 / creack/pty v1.1.24）：spawn+env 白名单零泄露、`Wait` 收割无僵尸、`Setsize` resize 生效（24x80→50x132）、master 退出后读 EIO、spawn 失败 fd 0/1/2 完好
- 本机原型运行：go:embed `all:dist` + `fs.Sub` + 预 gzip 双路径伺服
- 本机交叉编译：`GOOS=darwin GOARCH=amd64/arm64 go build`+`go vet` 通过（kqueue watcher 骨架与 creack/pty 全路径）
- Go 1.26.3 GOROOT 源码：`os/pidfd_linux.go`（pidfdWait/waitid P_PIDFD）、`syscall/exec_linux.go:310-312`（CLONE_PIDFD 条件）、`syscall/syscall_bsd.go:423`（darwin 也有 stdlib Kevent）
- 模块缓存源码精读：creack/pty v1.1.24（start.go/run.go/winsize_unix.go/pty_darwin.go）；coder/websocket v1.8.15（accept.go:102 Accept 签名、read.go:107 `defaultReadLimit = 32768`、read.go:317-323 自动 pong、conn.go 并发约束注释、close.go 状态码常量）
- ttyd 1.7.7 本地源码（~/open_src/ttyd）：pty.c:87/112（close(0) 缺陷链）、pty.c:155,433（winsize xpixel/ypixel 置 0）、pty.c:441-444（env 全继承缺陷）、pty.c:483（每进程 waitpid 线程）、server.h:8-16（协议帧常量 INPUT '0'/RESIZE_TERMINAL '1'/OUTPUT '0'）、protocol.c:44（RESIZE JSON `{columns, rows}`）
- gsd-tools package-legitimacy seam（npm 六包信号全量核实，postinstall 全 null）

### Secondary（MEDIUM 置信——官方文档引用）

- Context7 /xtermjs/xterm.js：Terminal 构造/open/loadAddon、`write(string|Uint8Array)` 二进制安全、`onResize: IEvent<{cols,rows}>`、WebGL→DOM 官方回落模式（demo/client.ts）、scrollback 默认 1000
- Context7 /richardtallent/vite-plugin-singlefile：`viteSingleFile()` 默认配置项、插件源码 viteMajor>=8 走 `codeSplitting:false`
- Context7 /coder/websocket：README Accept 模式、CloseError/ErrMessageTooBig 处理、`r.Context()` hijack 警告
- GitHub releases 官方页：actions/checkout v7.0.1、actions/setup-go v7.0.0、pnpm/action-setup v6.0.10（2026-07 当日线）
- npm registry `npm view`：@xterm/* 6.0.0/0.11.0/0.19.0、vite 8.2.1、vite-plugin-singlefile peer `vite:^5.4.21||^6||^7||^8`

### Tertiary（LOW 置信——需用户/执行期确认）

- darwin kqueue 僵尸注册补发行为（A1，CI 测试裁决）
- create-vite vanilla-ts 模板具体文件内容（A2）
- actions/setup-node 主版本（A3）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH——全部版本当日 registry 核实+源码精读+本机运行
- Architecture: HIGH——数据泵/spawn/收割/embed 均实证；darwin watcher 编译实证+运行时 CI 方案
- Pitfalls: HIGH——7 条全部带源码行号或本机实测依据
- 协议帧设计: MEDIUM——设计提案（ttyd 同构验证过形状），具体字节取值是本研究提案、待 planner 锁定

**Research date:** 2026-08-13
**Valid until:** 2026-09-12（30 天；xterm.js/Vite 快速演进，超期复核前端版本）
