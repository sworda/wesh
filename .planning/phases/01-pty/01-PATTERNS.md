# Phase 1: 行走骨架（核心 PTY 管道）- Pattern Map

**Mapped:** 2026-08-13
**Files analyzed:** 18（含测试与配置）
**Analogs found:** 0 / 18（本地代码库无任何既有代码——已核实的 greenfield）

## 0. 本地 Analog 核查结论（已实证）

| 候选来源 | 状态 | 结论 |
|----------|------|------|
| 仓库内源代码 | 仅 `.git/` + `.planning/`，`Glob **/*.go` 零命中 | **无任何本地 analog** |
| 项目级 CODEBUDDY.md / `.codebuddy/skills/` / `.agents/skills/` | 均不存在 | 无项目级模式约束（用户全局规则除外） |
| Go 模块缓存 `creack/pty@v1.1.24` / `coder/websocket@v1.8.15` / `x/sys@v0.47.0` | **不在缓存**（研究时于他处验证） | `go get` 落地后可在线精读；当前以 RESEARCH.md 的 VERIFIED 摘录为准 |
| GOROOT（go1.26.3，`~/softwares/go`） | `src/os/pidfd_linux.go`、`src/syscall/exec_linux.go` 本机可读 | reap_linux.go 注释引证的一手来源 |
| ttyd 1.7.7 源码（`~/open_src/ttyd/src/`） | 存在（pty.c / protocol.c / server.h） | **仅缺陷对照面，禁止参考其实现方式**（CONTEXT.md canonical_refs 明示） |

**因此本 PATTERNS.md 的"Analog"列全部指向阶段文档内已验证的模式摘录**（01-RESEARCH.md Pattern 1-5 均经模块源码精读或本机运行验证；01-UI-SPEC.md 为前端视觉/交互契约；01-VALIDATION.md 为测试映射）。planner 写 PLAN.md 时按下列行号直接摘抄，executor 照抄后微调。

## 0.1 模式来源行号索引

| 来源 | 关键区段（行号） |
|------|------------------|
| `.planning/phases/01-pty/01-RESEARCH.md` | Pattern 1 spawn+env（179-233）；Pattern 2 收割（235-326）；Pattern 3 WS 数据泵（328-392）；Pattern 4 embed+gzip（394-426）；Pattern 5 前端（428-483）；协议帧规格（589-603）；package.json/vite.config.ts（605-629）；CI yaml（631-661）；Anti-Patterns（485-494）；Pitfalls 1-7（510-557）；测试映射（730-748）；项目结构（152-177） |
| `.planning/phases/01-pty/01-UI-SPEC.md` | Terminal Options 契约（94-128）；页面 Shell CSS + #status 面板（131-149）；Renderer 契约 onContextLoss（153-157）；三态文案（171-183） |
| `.planning/phases/01-pty/01-VALIDATION.md` | Per-Task 测试映射（41-53）；Wave 0 清单（57-64） |
| `.planning/phases/01-pty/01-CONTEXT.md` | D-01~D-18 决策（17-46）；specifics 文档要求（99-105） |

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `go.mod` / `go.sum` | config | — | RESEARCH.md 安装段（82-88）+ **D-01 覆盖模块路径** | external-doc |
| `cmd/wesh/main.go` | controller（入口/装配/生命周期） | event-driven | **无 analog**——由 D-02~D-11 决策组装（见 §No Analog Found） | none |
| `internal/proto/proto.go` | utility（协议单一事实源） | transform | RESEARCH.md 协议帧规格（589-603） | external-doc |
| `internal/pty/spawn.go` | service | file-I/O（fork/exec/fd） | RESEARCH.md Pattern 1（179-233） | external-doc（本机运行验证） |
| `internal/pty/io.go` | service | streaming | RESEARCH.md Pattern 3 写泵段（350-366）+ Pitfall 3/4 | external-doc（部分拼装） |
| `internal/pty/reap_linux.go` | service | event-driven | RESEARCH.md Pattern 2 Linux 段（237-250）+ GOROOT 源码 | external-doc |
| `internal/pty/reap_darwin.go` | service | event-driven | RESEARCH.md Pattern 2 darwin 骨架（252-324） | external-doc（编译验证） |
| `internal/server/server.go`（含 session） | controller（HTTP/WS 网关） | streaming + request-response | RESEARCH.md Pattern 3（328-392） | external-doc（核心泵验证；生命周期无代码） |
| `web/embed.go` | utility（静态伺服） | request-response | RESEARCH.md Pattern 4（394-426） | external-doc（本机运行验证） |
| `internal/pty/spawn_test.go` | test | — | VALIDATION.md 1-01-01/02/03（43-45）+ Pattern 1 断言点 | external-doc |
| `internal/pty/io_test.go` | test | — | VALIDATION.md 1-01-04（46）+ 原型实测输出（RESEARCH 563-575） | external-doc |
| `internal/pty/reap_test.go` | test | — | VALIDATION.md 1-01-05/07（47,49）+ Open Questions Q1 方案 | external-doc |
| `internal/server/e2e_test.go` | test | — | VALIDATION.md 1-01-06（48） | external-doc |
| `web/src/main.ts` | component | streaming（WS 客户端） | RESEARCH.md Pattern 5（428-483）+ **UI-SPEC 扩展**（94-183） | external-doc |
| `web/index.html` | component（入口页） | — | UI-SPEC 页面契约（131-149）+ create-vite 脚手架（A2） | external-doc + scaffold |
| `web/package.json` / `pnpm-lock.yaml` / `vite.config.ts` / `tsconfig.json` | config | — | RESEARCH.md（605-629）+ `pnpm create vite` 实际生成物 | external-doc + scaffold |
| `web/dist/index.html`（占位） | config（占位） | — | Pitfall 6（RESEARCH 547-551） | external-doc |
| `.github/workflows/ci.yml` | config（CI） | batch | RESEARCH.md CI 提案（631-661） | external-doc |
| `README.md` | docs | — | **无 analog**——内容要求见 CONTEXT.md specifics（99-105） | none |

> 仓库重命名 `stow/` → `wesh/`（D-01）是目录级操作，不占文件行；planner 需将其编排进 Wave 0 最前置任务。

## Pattern Assignments

### `internal/pty/spawn.go`（service, file-I/O）

**Analog:** 本地无；模式源 = `01-RESEARCH.md` Pattern 1（lines 179-233，本机运行验证：env 零泄露、失败不伤 fd 0/1/2）

**Imports pattern**（按 RESEARCH 原型推导）：
```go
import (
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)
```

**核心 spawn 模式**（RESEARCH.md lines 197-208，照抄）：
```go
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
```

**env 白名单模式**（RESEARCH.md lines 210-231，照抄）：
```go
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

**关键纪律（RESEARCH.md line 233）：** 调用方**不得**设置 `cmd.Stdin/Stdout/Stderr`（`StartWithAttrs` 仅在三者全 nil 时接管 tty）；`cmd.Dir` Phase 1 不设（继承服务端 cwd）；服务端只持有并最后关闭 master。

---

### `internal/pty/reap_linux.go`（service, event-driven）

**Analog:** 本地无；模式源 = `01-RESEARCH.md` Pattern 2 Linux 段（lines 237-250）+ 本机 GOROOT 可在线引证（`~/softwares/go/src/os/pidfd_linux.go`、`src/syscall/exec_linux.go:310-312`）

**核心模式（整文件即此——`cmd.Wait()` 直达 + 文档化注释）：**
```go
//go:build linux

// Linux 收割 = 每会话一个 goroutine 阻塞在 cmd.Wait()。
// Go ≥1.23 的 os/exec 在 Linux 5.3+ 自动以 CLONE_PIDFD fork、以 waitid(P_PIDFD) 等待
// （GOROOT syscall/exec_linux.go:310-312、os/pidfd_linux.go），即"pidfd 收割"的正确实现：
// 零额外线程、无 PID 复用竞态、无僵尸、*exec.ExitError 带退出码。
// 禁止手写 pidfd_open / SIGCHLD+waitpid——会引入第二个收割者与 Wait 竞争，丢退出码
// （RESEARCH.md Anti-Patterns，lines 487-488）。
func awaitExit(cmd *exec.Cmd) error {
	return cmd.Wait()
}
```

**签名注意：** RESEARCH darwin 骨架的会话侧是 `awaitExit(cmd, exited <-chan struct{})`（lines 320-323）。planner 需统一两平台的 `awaitExit` 签名（linux 版忽略第二参或拆不同函数名），保证 `spawn.go`/`io.go` 调用点平台无关。

---

### `internal/pty/reap_darwin.go`（service, event-driven）

**Analog:** 本地无；模式源 = `01-RESEARCH.md` Pattern 2 darwin 骨架（lines 252-324，`GOOS=darwin` amd64+arm64 交叉编译+vet 通过；运行时行为由 CI macos leg 裁决，Open Questions Q1）

**完整骨架照抄 RESEARCH.md lines 254-324**（`exitWatcher` 结构 / `newExitWatcher` / `watch` / `loop` / 会话侧 `awaitExit`）。要点摘录：

```go
//go:build darwin

import "golang.org/x/sys/unix"

// 进程级共享 watcher：一个 kqueue fd、一个 goroutine，N 会话共用（零每会话线程）
type exitWatcher struct {
	kq   int
	mu   sync.Mutex
	subs map[int]chan<- struct{} // pid -> notify
}

// EV_ADD 注册；EV_ONESHOT 触发一次即自动注销
ev := []unix.Kevent_t{{
	Ident:  uint64(pid),
	Filter: unix.EVFILT_PROC,
	Flags:  unix.EV_ADD | unix.EV_ONESHOT,
	Fflags: unix.NOTE_EXIT | unix.NOTE_EXITSTATUS,
}}

// 会话侧：通知到达后 Wait 收割（唯一收割者，退出码完整）
func awaitExit(cmd *exec.Cmd, exited <-chan struct{}) error {
	<-exited          // kqueue 早知退出
	return cmd.Wait() // 立即返回：wait4 收割 + *exec.ExitError
}
```

**竞态兜底（planner 必须编进任务）：** 僵尸注册竞态（A1）由 CI 两个针对性测试裁决（RESEARCH.md lines 691-694 / VALIDATION.md task 1-01-07）；若超时未补发事件，darwin `awaitExit` 退化为直接 `cmd.Wait()` goroutine，watcher 代码以 build tag 保留。**禁止** SIGCHLD+WNOHANG 手动 reap（RESEARCH 修正决议，line 488）。

---

### `internal/pty/io.go`（service, streaming）

**Analog:** 本地无；模式源分散三处，planner 拼装：

1. **master 读循环**（RESEARCH.md Pattern 3 写泵段，lines 350-366）——该循环逻辑归属 pty/io 还是 server 由 planner 定（RESEARCH 架构图 line 136 把它画在数据泵内；`pty/` 数据面隔离原则建议读循环放 io.go、WS 写留 server）：
```go
buf := make([]byte, 32*1024)
for {
	n, err := master.Read(buf)
	if n > 0 { /* 投递 OUTPUT */ }
	if err != nil { // Linux EIO / darwin EOF——统一视为输出终结（Pitfall 3）
		return
	}
}
```
2. **EIO/EOF 归一纪律**（Pitfall 3，RESEARCH lines 526-531）：任何 read 错误（含 `io.EOF` 与 EIO）统一归一为"输出终结"，**禁止** `err == io.EOF` 单判。
3. **resize 钳制 + Setsize**（D-16 + Pitfall 2，RESEARCH lines 519-524）：`Setsize` 前钳制 cols/rows 到 `[1,1000]`；`ws_xpixel/ws_ypixel` 置 0（ttyd 与 creack/pty 共同实践，RESEARCH line 25）。
4. **drain 语义**（D-12 + Pitfall 4，RESEARCH lines 533-538）：attach 前 drain goroutine 持续读 master 丢弃（防 64KiB 内核缓冲写阻塞）；`Wait()` 返回后启动**带时限 drain**（~200ms 或 EOF 先到为准），随后无条件 `close(master)`。

---

### `internal/proto/proto.go`（utility, transform）

**Analog:** 本地无；模式源 = `01-RESEARCH.md` 协议帧规格（lines 589-603，ttyd 同构形状、wesh 自定义取值）

**核心内容（整文件照此定义——单一事实源）：**
```go
package proto

// 数据面 binary frame：1 字节 ASCII 类型 + 载荷。
// 前端 TS 常量手工对齐本文件（web/src/main.ts）。
const (
	Input  = '0' // 0x30, C→S, raw bytes → 写 master
	Resize = '1' // 0x31, C→S, JSON {"cols":C,"rows":R} → 钳制 1..1000 后 Setsize
	Output = '0' // 0x30, S→C, master 读块直发
)

// Phase 2 预留（现在不实现，收到即 1002 关闭）：
// '2' PAUSE / '3' RESUME / 'E' ERROR+JSON / 'X' EXIT+退出码
```

**关闭码纪律（lines 600-603）：** 主动发送仅限 1000（子进程退出正常关）与 1002（未知帧）；1009 由 `SetReadLimit` 默认自动产生；**1006 永不写入**（RFC6455 §7.4，PITFALLS C9）。

**RESIZE JSON 编解码 + 钳制 helper** 亦落本文件（D-16 钳制 1..1000；解码失败即丢弃，不关闭连接——RESEARCH V5 行 line 768）。前端对齐点：`web/src/main.ts` 的 `OUTPUT/INPUT/RESIZE` 常量（Pattern 5 line 451）。

---

### `internal/server/server.go`（controller, streaming + request-response）

**Analog:** 本地无；模式源 = `01-RESEARCH.md` Pattern 3（lines 328-392，API 签名逐行核自 coder/websocket v1.8.15 源码）+ CONTEXT.md 生命周期决策（D-09/D-10/D-11，无现成代码，见 §No Analog Found）

**Imports pattern：**
```go
import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto" // 模块路径以 D-01 为准
)
```

**核心数据泵模式（RESEARCH.md lines 334-391，照抄为骨架）：**
```go
func serveWS(w http.ResponseWriter, r *http.Request, sess *session) {
	// 不設 InsecureSkipVerify：默认同源（请求 Host）自动放行，跨源拒绝。
	// Subprotocols 留空：wesh.v1 协商是 Phase 2；CompressionMode 默认 Disabled。
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		return // Accept 失败已自动写 HTTP 错误响应
	}
	defer c.CloseNow()
	// SetReadLimit 默认 32768（read.go:107 VERIFIED），超限自动 1009——Phase 1 免设
	ctx, cancel := context.WithCancel(context.Background()) // 禁止用 r.Context()（hijack 后行为意外）
	defer cancel()

	// S→C：单 writer goroutine 独占写端（Phase 1 直写，无 outbox）
	go func() { /* master.Read → frame[0]=proto.Output → c.Write(Binary) 循环；err 即 return */ }()

	// C→S：单 reader（Read 不可并发——Pitfall 7）
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
		case proto.Input:
			sess.master.Write(data[1:])
		case proto.Resize:
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

**本文件还需实现、但无模式代码的决策逻辑（planner 从决策直接翻译）：**
- **D-09：** 第二 WS 连接 → HTTP 409 拒绝（attach 原子标志位即可；UI-SPEC 文案已含 409 场景，line 178）。
- **D-10：** 子进程退出 → 先发 WS 1000 正常关闭帧 → 进程退出，**退出码 = 子进程退出码**（从 `*exec.ExitError.ExitCode()` 取）。
- **D-11：** WS 断开（reader 返回错误）→ SIGHUP 子进程**进程组**（`syscall.Kill(-pgid, SIGHUP)`，pgid = 子进程 pid，因 setsid）→ 自身退出。
- **D-12：** drain goroutine 从 spawn 起持续运行（见 `io.go` 条目）。

---

### `web/embed.go`（utility, request-response）

**Analog:** 本地无；模式源 = `01-RESEARCH.md` Pattern 4（lines 394-426，本机运行验证 gzip 协商与明文回退双路径）

**完整模式照抄（RESEARCH.md lines 398-424）：**
```go
package web

//go:embed all:dist
var dist embed.FS

func Handler() (http.Handler, error) {
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

**硬约束（RESEARCH line 177 + Pitfall 6）：** `//go:embed` 不能引用包目录外文件（`../web/dist` 非法）——故 `embed.go` 必须与 `dist/` 同级，即放 `web/`（包名 `web`）；仓库必须提交 `web/dist/index.html` 占位（内容注明"前端未构建，请先 pnpm -C web build"），否则裸 clone `go test` 编译失败。
**位置冲突提示：** RESEARCH 责任映射表（line 39）写 `internal/web`，但推荐项目结构（line 169）与 CONTEXT.md code_context（line 90）均为 `web/embed.go`——**以后两者为准**。

---

### `web/src/main.ts`（component, streaming）

**Analog:** 本地无；模式源 = `01-RESEARCH.md` Pattern 5（lines 428-483）**+ `01-UI-SPEC.md` 三处扩展**（planner 必须合并两者，UI-SPEC 在以下三点严于 Pattern 5）

**基础骨架照抄 RESEARCH.md lines 432-482**（Terminal 构造 → fit addon → WebGL try/catch 回落 → WS binary 帧收发 → onData/onResize/resize debounce 回路）。UI-SPEC 扩展：

1. **Terminal Options 契约（UI-SPEC lines 94-128，逐项显式钉死，不得依赖库默认值）：**
```typescript
const term = new Terminal({
  fontSize: 14,
  fontFamily: 'Menlo, Monaco, "Cascadia Mono", Consolas, "DejaVu Sans Mono", "Liberation Mono", "Courier New", monospace',
  lineHeight: 1.0,
  letterSpacing: 0,
  fontWeight: 400,
  fontWeightBold: 700,
  cursorStyle: 'block',
  cursorBlink: true,        // 有意覆盖 xterm 默认 false
  scrollback: 10000,
  allowTransparency: false,
  theme: { /* UI-SPEC §Terminal Theme 全 16 色 + foreground/background/cursor/selectionBackground 照抄 */ },
});
```
2. **Renderer 契约（UI-SPEC lines 153-157）：** WebglAddon 除 try/catch 外**必须**注册 `webglAddon.onContextLoss(() => webglAddon.dispose())`（GPU 上下文丢失自动回落 DOM，不黑屏）——Pattern 5 未含此行，必须补上。
3. **#status 状态面板（UI-SPEC lines 144-149 + 文案 171-183）：** `ws.onerror`/`ws.onclose` 显示默认隐藏的 `#status` 面板，按三态选文案：未 open 过 → "Unable to connect"（含 409 场景）；open 后 1000 → "Session ended"；open 后非 1000 → "Connection lost"。键盘输入已有 `readyState === OPEN` 门控（Pattern 5 line 462），面板期间输入静默丢弃。

**保留的 Pattern 5 纪律：** resize 发送前 `Number.isInteger && > 0` 防护（line 478）；`ws.onopen` 内 `fit.fit(); sendResize(...); term.focus()`（line 482）；帧常量与 `proto/` 手工对齐（line 451）。

---

### `web/index.html`（component）

**Analog:** 本地无；模式源 = `01-UI-SPEC.md` 页面契约（lines 131-149）+ create-vite vanilla-ts 模板（A2，executor 以 `pnpm create vite` 实际生成物为底微调）

**契约级要点（逐字实现）：**
```html
<head>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>wesh</title>
  <style>
    /* singlefile 保证 CSS 先于 JS 内联——无白闪契约 */
    html, body { margin: 0; padding: 0; height: 100%; background: #000000; overflow: hidden; }
    #terminal { width: 100%; height: 100%; }
  </style>
</head>
<!-- 页面仅两个顶层元素：#terminal（始终存在）与 #status（默认 hidden） -->
```
- **禁用一切 webfont/外部字体请求**（singlefile 离线硬约束，UI-SPEC line 30）。
- #status 面板样式按 UI-SPEC line 147 的 CSS 值（`position:fixed; inset:0; rgba(0,0,0,0.6)` 遮罩 + 480px/24px/8px/#161616 面板）。

---

### 测试文件（role: test）

**Analog:** 本地无；模式源 = `01-VALIDATION.md` Per-Task 映射（lines 41-53，测试名/命令逐条已定）+ `01-RESEARCH.md` 原型实测断言点（lines 563-575）

| 文件 | 覆盖 | 断言要点（来自研究原型实测） |
|------|------|------------------------------|
| `internal/pty/spawn_test.go` | 1-01-01/02/03 | argv 含 `$(id)` 不被展开；宿主注入 `AWS_SECRET_ACCESS_KEY` 后白名单结果不含；spawn 不存在二进制后 fd 0/1/2 `Fsync` 非 EBADF |
| `internal/pty/io_test.go` | 1-01-04 | spawn `stty size; sleep 1; stty size`，中途 `Setsize(50,132)`，输出含 `50 132`（原型已验证该逻辑成立） |
| `internal/pty/reap_test.go` | 1-01-05/07 | linux：短命令退出后 `/proc/<pid>` 消失；darwin（CI-only）：Q1 双测试——正常路径（`sleep 0.1 && exit 42` 断言退出码 42）+ 僵尸注册竞态路径（先 sleep 200ms 再注册，1s 超时等事件） |
| `internal/server/e2e_test.go` | 1-01-06 | `httptest.Server` + `wesh -- /bin/cat` + `websocket.Dial`，发 INPUT 帧断言收到同字节 OUTPUT 帧 |

**测试基建纪律：**
- 框架 = Go stdlib `testing` + `-race`，e2e 用 coder/websocket `Dial` 客户端，**零新增测试依赖**。
- Quick：`go test ./... -count=1`；Full：`go test -race -count=1 ./... && pnpm -C web build`。
- **Pitfall 5：** CI 测试 job **不设** `CGO_ENABLED=0`（-race 需要 cgo；该变量只出现在 Phase 9 发布构建）。
- FE-01/FE-03 为手动 checklist（VALIDATION.md lines 68-74），无自动化文件。

---

### 配置与 CI（role: config）

**`go.mod`**：`go mod init github.com/sworda/wesh`——**D-01 覆盖 RESEARCH.md line 84 的 `github.com/sworda/wesh`**（CONTEXT 决策优先）；随后 `go get github.com/creack/pty@v1.1.24 github.com/coder/websocket@v1.8.15 golang.org/x/sys@v0.47.0`。

**`web/package.json` / `web/vite.config.ts`**（RESEARCH.md lines 605-629，照抄）：
```jsonc
// package.json 关键行
{ "name": "wesh-web", "private": true, "type": "module",
  "scripts": { "dev": "vite",
    "build": "vite build && gzip -k -9 -f dist/index.html" } } // 产出 .gz 旁路供 embed 直发
```
```typescript
// vite.config.ts
import { defineConfig } from 'vite';
import { viteSingleFile } from 'vite-plugin-singlefile';
export default defineConfig({
  plugins: [viteSingleFile()], // 全量 inline；插件对 viteMajor>=8 自动走 codeSplitting:false
});
```
依赖锁定（D-13）：`@xterm/xterm@6.0.0 @xterm/addon-fit@0.11.0 @xterm/addon-webgl@0.19.0`；dev：`vite@8.2.1 vite-plugin-singlefile@2.3.3 typescript`。`tsconfig.json` 与 `index.html` 底稿以 `pnpm create vite` 实际生成物为准（A2）。

**`.github/workflows/ci.yml`**（RESEARCH.md lines 631-661，照抄为底，executor 按 GitHub Actions 当前版本微调）：
```yaml
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
        with: { go-version-file: go.mod }
      - run: go vet ./...
      # 注意：不设 CGO_ENABLED=0——-race 需要 cgo（Pitfall 5）
      - run: go test -race -count=1 ./...
  web:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: pnpm/action-setup@v6.0.10
      - uses: actions/setup-node@v4
        with: { node-version: 24 }
      - run: pnpm -C web install --frozen-lockfile
      - run: pnpm -C web build
```

## Shared Patterns

以下为跨文件横切纪律，planner 应写进每个相关 PLAN.md 的 action 段：

### 协议与关闭码纪律
**Source:** `01-RESEARCH.md` lines 589-603 · **Apply to:** `internal/proto/proto.go`、`internal/server/server.go`、`web/src/main.ts`
帧常量以 `proto/` 为单一事实源，前端手工对齐；主动关闭码只发 1000/1002，1009 靠库默认，1006 永不写入；未知帧类型 → 1002。

### 错误处理（Go 惯用法，无集中错误包）
**Apply to:** 全部 Go 文件
- spawn/IO 错误直接向上返回 `error`，调用方（main）决定退出路径——Phase 1 单次语义下不引入自定义错误类型（过度设计红线）。
- WS `Accept` 失败已自动写 HTTP 响应，handler 直接 return；`c.Read` 错误用 `errors.As(err, &websocket.CloseError)` 区分对端关闭（Pattern 3 lines 370-375）。
- RESIZE JSON 解析失败**静默丢弃**（不关连接）；未知帧才 1002。

### "只关自己打开的 fd"
**Source:** Pitfall 1（RESEARCH lines 512-517）· **Apply to:** `internal/pty/*`
master 用有效指针判空而非零值推断；失败路径禁止裸 `close(fd)`；回归测试 = task 1-01-03（fd 0/1/2 Fsync 探测）。

### EIO/EOF 归一
**Source:** Pitfall 3（RESEARCH lines 526-531）· **Apply to:** `internal/pty/io.go`、`internal/server/server.go` 写泵
master 读的任何错误（含 EOF 与 EIO）统一视为"输出终结"——`if err != nil { return }`，禁止 `err == io.EOF` 单判。

### resize 双端防护
**Source:** Pitfall 2（RESEARCH lines 519-524）+ D-16 · **Apply to:** `web/src/main.ts`（前端 debounce 100ms + `Number.isInteger && >0` 防护）与 `proto/`+`pty`（服务端钳制 `[1,1000]`）

### 并发纪律
**Source:** Pitfall 7 + Pattern 3（RESEARCH lines 328-331, 554-557）· **Apply to:** `internal/server/*`
`c.Read`/`c.Reader` 单 goroutine；写端 Phase 1 单 writer 独占；Phase 2 引入 `c.Ping` 时必须在另一 goroutine（届时记住 Ping 与 Read 必须并发存在）。

### 构建顺序与 embed 硬依赖
**Source:** D-18 + Pitfall 6（RESEARCH lines 547-551）· **Apply to:** Wave 0 编排
本地构建 `pnpm -C web build` 先于 `go build`；仓库提交 `web/dist/index.html` 占位保证裸 clone `go test` 可编译。

## No Analog Found

以下文件/逻辑在 RESEARCH.md 中也**无现成代码模式**，planner 需直接从 CONTEXT.md 决策翻译为实现（不得臆造"既有模式"）：

| File / 逻辑 | Role | 缺口说明 | 组装来源 |
|-------------|------|----------|----------|
| `cmd/wesh/main.go` | controller | Pattern 1-5 均不覆盖 CLI 解析与组件装配 | D-02（`--` 后原样数组传递，绝不经 shell）、D-03（无命令报错退出——已关闭 RESEARCH A4 假设）、D-04（仅 4 个 flag：stdlib `flag` 包即可，勿引第三方 CLI 库）、D-05/D-06（bind 0.0.0.0 / port 7681，`--port 0` 打印实际端口）、D-07（启动仅打印单行 `listening on http://host:port`）、D-10/D-11（退出码传递与 SIGHUP 进程组后退出） |
| server 生命周期逻辑（409/SIGHUP/退出码） | controller 内逻辑 | Pattern 3 只含数据泵 | D-09（第二连接 HTTP 409）、D-10（1000 帧 + 退出码 = 子进程退出码）、D-11（WS 断 → SIGHUP 进程组 → 退出） |
| `README.md` | docs | 无 | CONTEXT.md specifics（99-105）：必须显式标注"Phase 1 无认证，仅在可信网络使用"+ 单次语义说明（"WS 断开即退出；断线重连在 Phase 6"），避免用户误以为 bug |
| `web/index.html` / `tsconfig.json` 底稿 | component/config | 研究标注 ASSUMED（A2） | `pnpm create vite` 实际生成物 + UI-SPEC 页面契约覆盖 |

## Planner 注意事项（跨文档冲突与覆盖）

1. **模块路径：** D-01 = `github.com/sworda/wesh`，覆盖 RESEARCH.md line 84 的 `github.com/sworda/wesh` 示例。所有 import 路径以 D-01 为准。
2. **embed 包位置：** `web/embed.go`（推荐结构 + CONTEXT 为准），不是 RESEARCH 责任映射表的 `internal/web`。
3. **前端三处 UI-SPEC 扩展**严于 Pattern 5：Terminal Options 显式钉死、`onContextLoss` 注册、#status 三态面板——PLAN.md 必须合并两者，不得只抄 Pattern 5。
4. **`awaitExit` 跨平台签名统一**（linux 无 `exited` 参数、darwin 有）——planner 需定义统一调用点接口。
5. **ttyd 源码仅作缺陷对照**：可在注释/测试中引证行号（如 `pty.c:441-444` env 泄露对照），**禁止**参考其实现方式（C1 手写分片重组、每进程 waitpid 线程等均在本 phase Anti-Patterns 清单）。
6. **darwin watcher 与 CI 同日编排**：Q1 双测试必须与 `reap_darwin.go` 实现同 wave（RESEARCH line 694），兜底方案预先写入任务描述。

## Metadata

**Analog search scope:** 仓库全量（`Glob **/*.go` 零命中）、Go 模块缓存（三依赖缺席）、GOROOT（pidfd 源码可用）、ttyd 源码（对照面）
**Files scanned:** 仓库 2 个顶层目录 + 5 份阶段文档（CONTEXT/RESEARCH/VALIDATION/UI-SPEC 全量读取）
**Pattern extraction date:** 2026-08-13
**模式置信度:** 后端 spawn/reap/pump/embed 四 Pattern 为本机运行验证或源码精读核实（HIGH）；darwin watcher 编译验证、运行时 CI 裁决（MEDIUM 已备兜底）；前端 Pattern 5 + UI-SPEC 为官方文档+契约级（HIGH）；main.go/生命周期/README 为决策直译（无模式代码，缺口已显式标注）
