# Phase 4: 前端体验 - Pattern Map

**Mapped:** 2026-08-18
**Files analyzed:** 8 个待创建/修改文件
**Analogs found:** 7 / 8（全部 analog 均已逐行 Read 核实）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/src/main.ts`（修改） | 前端入口/component | event-driven + request-response | 同文件既有段落（自扩展） | exact（就地扩展） |
| `web/package.json`（修改） | config | — | 同文件（现状 3 依赖段） | exact |
| `internal/proto/proto.go`（修改） | model（协议单一事实源） | request-response | 同文件 HelloPayload/WelcomeFrame | exact |
| `internal/server/server.go`（修改） | service（WS 服务端） | event-driven | 同文件 Options/New/升档序列 | exact |
| `cmd/wesh/main.go`（修改） | CLI 入口/config | request-response | 同文件 fs.Func/validateStartup | exact |
| `web/uat/phase04.mjs`（新建） | test（e2e 协议断言） | request-response | `web/uat/phase03.mjs` | exact |
| `internal/proto/proto_test.go`（修改） | test（unit） | — | 同文件 TestWelcomeFrameErrorFrame | exact |
| `cmd/wesh/main_test.go`（修改） | test（unit） | — | 同文件 TestParseArgs/TestTLSKeyPairError | exact |
| `web/src/lib/*.ts`（可选新建，A3） | utility（纯函数） | transform | 无前端工具模块先例 | **no analog** |

## Pattern Assignments

### `web/src/main.ts`（前端入口，event-driven）— 就地扩展

本 phase 主战场。六个能力（标题/unicode11/web-links/clipboard/浮层+unload/prefs）全部在此接线。**逐字契约细节以 04-UI-SPEC.md 与 04-RESEARCH.md §Pattern 1-6 为准**，本节只钉"接在既有哪段、照哪段的写法"。

**① Imports + 帧常量段**（main.ts:1-18，新 addon import 加在第 4 行后）：
```typescript
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
// 新增：Unicode11Addon / WebLinksAddon 同形态命名导入；ClipboardAddon 同上（运行期条件加载）

// 帧常量与 internal/proto/proto.go 手工对齐（D-16，两侧注释互相指路）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31,
  HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45, SUBPROTOCOL = 'wesh.v1';
```

**② Terminal 构造选项段**（main.ts:21-56）——query 覆盖的装配点（RESEARCH §Pattern 6①：构造前解析 query，白名单键 JSON.parse 成功项作构造初值并入此对象字面量）：
```typescript
const term = new Terminal({
  fontSize: 14,
  fontFamily: 'Menlo, Monaco, "Cascadia Mono", …, monospace',
  cursorBlink: true, // 有意覆盖 xterm 默认 false
  scrollback: 10000,
  theme: { foreground: '#ffffff', background: '#000000', /* tango 全表 */ },
});
// 注：theme 常量化是 Pitfall 3 修正前提——`{...defaultTheme, ...prefs.theme}` 合并需要
// 构造时的 tango 对象可引用，planner 决定是否把 theme 提为命名常量。
```

**③ addon 加载段模式**（main.ts:58-70）——unicode11/web-links 紧随 webgl 段后照此形态续写：
```typescript
const fit = new FitAddon();
term.loadAddon(fit);
term.open(document.getElementById('terminal')!);

// FE-01：首选 WebGL 渲染器，加载失败停留 DOM 渲染器
try {
  const webglAddon = new WebglAddon();
  webglAddon.onContextLoss(() => webglAddon.dispose());
  term.loadAddon(webglAddon);
} catch (e) {
  console.warn('webgl addon load failed, stay on DOM renderer', e);
}
// 新增（RESEARCH §Pattern 2 硬顺序，不可颠倒）：
// term.loadAddon(new Unicode11Addon());
// term.unicode.activeVersion = '11';
// term.loadAddon(new WebLinksAddon(undefined, { hover, leave }));
// ClipboardAddon 不在此——仅 WELCOME prefs.osc52===true 时加载
```

**④ 常驻事件接线模式**（main.ts:92-115）——onSelectionChange/onTitleChange/keydown 照此模块级常驻接线形态：
```typescript
const enc = new TextEncoder();
term.onData((s) => {
  if (ws !== null && ws.readyState === WebSocket.OPEN) { // null 闸 + OPEN 闸双门
    ws.send(concat(new Uint8Array([INPUT]), enc.encode(s)));
  }
});
term.onResize(({ cols, rows }) => sendResize(cols, rows)); // 浮层挂此 handler 内扩展
let timer: number | undefined;                             // debounce timer 形态
window.addEventListener('resize', () => {
  clearTimeout(timer);
  timer = window.setTimeout(() => fit.fit(), 100);
});
```
注意：**onResize 现接线于 main.ts:110 只有一行 sendResize**——浮层逻辑（RESEARCH §Pattern 5）并入此 handler 或新挂一个 onResize 均可，planner 定；`welcomeDone` 门需新增模块级布尔。

**⑤ WELCOME 分支扩展点**（main.ts:197-210）——prefs 应用与 ro 模块级化的落点：
```typescript
case WELCOME: {
  try {
    const w = JSON.parse(new TextDecoder().decode(buf.subarray(1)));
    if (w.mode === 'ro') {
      term.options.disableStdin = true;
      document.title = '[ro] ' + document.title; // ← Phase 3 一次性前缀写法，本 phase
      // 由 setTitle() 单一写口替代（RESEARCH §Pattern 1：标题变化时前缀会丢）
    }
    // 新增：w.prefs 逐项应用（跳过 queryKeys）→ term.options.* → fit.fit()
    //       → behavior 两开关 → osc52===true 时 loadAddon(ClipboardAddon)
  } catch {
    console.warn('discard malformed WELCOME frame'); // 畸形帧丢弃纪律不变
  }
  break;
}
```
**模块级状态提升要求**（RESEARCH §Pattern 6 核实注）：`mode` 当前是 WELCOME 分支局部变量，需提为模块级 `isRO`——粘贴门（D-10）、标题写口 `[ro] ` 前缀（D-02）、osc52 分支三处共用。

**⑥ onclose 分派段**（main.ts:256-318）——beforeunload listener 移除挂点：WS close 任意路径（含 showStatus 各分支）都要移除 beforeunload，"Session ended" 后关页不再被拦截（RESEARCH §Pattern 5）。

---

### `web/package.json`（config）— 加三依赖

**Analog:** 同文件现状（精确版本钉死、无 `^` 前缀纪律）：
```json
"dependencies": {
  "@xterm/xterm": "6.0.0",
  "@xterm/addon-fit": "0.11.0",
  "@xterm/addon-webgl": "0.19.0"
  // 新增同形态钉死：
  // "@xterm/addon-unicode11": "0.9.0",
  // "@xterm/addon-web-links": "0.12.0",
  // "@xterm/addon-clipboard": "0.2.0"
}
```
**建议项**（RESEARCH §Package Legitimacy Audit，planner 定稿）：`pnpm.overrides: { "js-base64": "3.9.2" }` 钉住 addon-clipboard 的 1 天新传递依赖；不采纳则 install 前加 `checkpoint:human-verify`。

---

### `internal/proto/proto.go`（协议 model）— WelcomePayload 加字段

**Analog:** 同文件 HelloPayload.Ticket omitempty 先例（proto.go:77-87）+ WelcomeFrame 组帧（proto.go:109-114）：

**加字段模式**（proto.go:72-82 注释纪律 + 84-87 现状）：
```go
// HelloPayload 显式 json tag，防字段名漂移。
// 未知字段由 json.Unmarshal 默认忽略——D-02 演化纪律的零成本实现
type HelloPayload struct {
	Version string `json:"version"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	Ticket  string `json:"ticket,omitempty"` // omitempty：缺席即 JSON 不出键
}

// WelcomePayload 现状（84-87）——照 Ticket 同形态加 Prefs：
type WelcomePayload struct {
	Mode string `json:"mode"`
	// 新增：Prefs json.RawMessage `json:"prefs,omitempty"`
	// （RESEARCH §Pattern 6 Go 契约：服务端原样透传不解析，缺席即旧前端零漂移）
}
```

**组帧函数签名演进**（proto.go:109-114 现状）：
```go
// WelcomeFrame 组 Welcome 帧：1 字节类型 + JSON {"mode":M}，调用方直接 c.Write
func WelcomeFrame(mode string) []byte {
	b, _ := json.Marshal(WelcomePayload{Mode: mode})
	return append([]byte{Welcome}, b...)
}
// 演进为 WelcomeFrame(mode string, prefs json.RawMessage)——空 prefs 时 JSON 不出 prefs 键
```

**白名单键常量落点**：照 ModeRO/ModeRW（proto.go:33-37）与 Error codes（44-50）的常量块 + 注释形态，加 client-option 白名单键表（8 xterm 键 + resizeOverlay/confirmBeforeUnload；osc52 不在内）。遵守 CODEBUDDY「简洁常量/switch，不引入新抽象层」。

---

### `internal/server/server.go`（WS 服务端）— Options 加字段 + Welcome 挂点注入

**① Options 结构体扩展模式**（server.go:84-96，逐字段注释 + 零值语义明确）：
```go
type Options struct {
	Writable         bool
	PingInterval     time.Duration
	// …（HelloTimeout/MaxHalfOpenPerIP/PongTimeout/Credentials/Origins/TLS/…）
	// 新增：ClientPrefs json.RawMessage（--client-option 聚合 + osc52 并入；空 = 不下发）
}
```

**② New() 装配拷贝模式**（server.go:127-139）：
```go
s := &Server{
	sess:         sess,
	writable:     opts.Writable,
	pingInterval: opts.PingInterval,
	// …逐字段拷贝；新增 clientPrefs: opts.ClientPrefs 同形态
}
```

**③ Welcome 组帧挂点**（server.go:433，升档序列尾段——顺序敏感注释 420-428 保留）：
```go
// Welcome 写失败不补救——连接已死，读循环下一拍收口。
_ = c.Write(ctx, websocket.MessageBinary, proto.WelcomeFrame(mode))
// 演进：proto.WelcomeFrame(mode, s.clientPrefs)
```
纪律：服务端对 prefs 是**不透明 blob 透传**（RESEARCH §Anti-Patterns：白名单/JSON 校验在 `--client-option` parse 期已完成，服务端二次解析引入双写漂移面）。

---

### `cmd/wesh/main.go`（CLI 入口）— 两个新 flag

**① 可重复 flag + parse 期校验模式**（main.go:54-63，--credential 先例；--client-option 照抄形态）：
```go
// D-01：可重复凭据 flag，fs.Func 回调内 parse 期校验（畸形值即时报错——
// systemd 配置错误零窗口暴露）。
fs.Func("credential", "basic auth credential user:pass (repeatable; …)", func(s string) error {
	c, err := server.ParseCredential(s)
	if err != nil {
		return err
	}
	cfg.credentials = append(cfg.credentials, c)
	return nil
})
// --client-option 同形态：strings.Cut(s, "=") → proto.ValidClientOptionKey(key)
// → json.Unmarshal 值校验 → cfg.clientOptions = append(…)（RESEARCH §Code Examples）
```

**② 布尔 flag 模式**（main.go:66-67，--osc52 照抄）：
```go
fs.BoolVar(&cfg.noAuth, "no-auth", false, "allow listening on non-loopback address without credentials (explicit escape hatch)")
// fs.BoolVar(&cfg.osc52, "osc52", false, "enable OSC52 clipboard write (write-only; default off)")
```

**③ config 结构体扩展**（main.go:25-38，逐字段注释带决策号纪律）：
```go
type config struct {
	port     int
	bind     string
	// …（每字段尾注释引用 D-xx）
	// 新增：clientOptions []clientOption / osc52 bool——尾注释引 P4 D-12/D-15
}
```

**④ parse 期成对/形态校验模式**（main.go:88-92，cert/key 成对先例）：
```go
if (cfg.tlsCert == "") != (cfg.tlsKey == "") {
	return cfg, nil, errors.New("must give both --tls-cert and --tls-key")
}
```
纪律：配置形态错误在 parseArgs 报（exit 2 路径），validateStartup 不重复——client-option 白名单/JSON 校验属 fs.Func 内 parse 期，osc52 无校验需求（纯布尔）。

**⑤ Options 传递**（main.go:201）：
```go
srv := server.New(sess, os.Exit, server.Options{Writable: cfg.writable, PingInterval: cfg.pingInterval, Credentials: cfg.credentials, Origins: cfg.origins, TLS: cfg.tlsCert != ""})
// 新增 ClientPrefs: 聚合结果（clientOptions + osc52 并入为单个 json.RawMessage）
```

---

### `web/uat/phase04.mjs`（新建 e2e 协议断言）— 照 phase03.mjs 全形态

**Analog:** `web/uat/phase03.mjs`（零依赖 Node >= 22，逐字复用以下骨架）：

**① 文件头 + 帧常量对齐注释**（phase03.mjs:1-26）：
```javascript
// Phase 3 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch）。
// 红线（SEC-01）：凭据值与 ticket 值只作协议构造材料，永不进入 check detail
// 运行：node web/uat/phase03.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
import { spawn, execFile } from 'node:child_process';
// …
// 帧类型（与 internal/proto/proto.go 对齐）
const OUTPUT = 0x30, INPUT = 0x30, RESIZE = 0x31, HELLO = 0x48, WELCOME = 0x57, ERROR = 0x45;
const SUBPROTOCOL = 'wesh.v1';
```

**② 实例 spawn + 端口解析**（phase03.mjs:57-73）：
```javascript
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    // scheme 感知正则解析 listening 行；8s 超时 SIGKILL
    const m = /listening on (https?):\/\/[^\s]+:(\d+)/.exec(d.toString());
    if (m) { resolve({ port: Number(m[2]), scheme: m[1], kill: () => child.kill('SIGKILL'), child }); }
  });
}
```

**③ 拒绝路径 spawn-exit 形态**（phase03.mjs:77-89）——`--client-option` 非法值启动报错断言照此：
```javascript
function spawnExpectExit(args) { /* 3s 内自行退出 → resolve({ code, stderr })，超时 reject */ }
```

**④ WS 握手 + Welcome 断言**（phase03.mjs:94-108, 180-182）——Welcome prefs 形状断言照此：
```javascript
const { ws, frames } = await dialHello(inst.port, { ticket: body.ticket });
const welcome = JSON.parse(dec.decode(frames.find((f) => f[0] === WELCOME).subarray(1)));
check('S1e', 'Hello 携 ticket → Welcome(mode=rw)', welcome.mode === 'rw', `mode=${welcome.mode}`);
// phase04 同形态：断言 welcome.prefs 逐键值 / 缺省无 prefs 键（omitempty 回归）
```

**⑤ 结果收集与出口码**（phase03.mjs:48-52, 347-360）：
```javascript
const results = [];
const check = (id, name, ok, detail = '') => { results.push({ id, name, ok, detail });
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`); };
// 尾部：const passed = results.filter((r) => r.ok).length;
// process.exit(passed === results.length && failed === 0 ? 0 : 1);
```
注意 phase03.mjs 的**单次语义适配**（S1f 注释）：同进程第二次 WS 建连不可行，每个需 WS 的场景独立 spawn 实例——phase04 的 osc52/prefs 两场景同此纪律。

---

### `internal/proto/proto_test.go`（修改）— 表驱动扩展

**Analog:** 同文件 TestWelcomeFrameErrorFrame（proto_test.go:60-90）+ TestDecodeHello 表（13-58）：
```go
// 组帧往返逐字锁定形态（62-73）：
wf := WelcomeFrame(ModeRO)
if len(wf) == 0 || wf[0] != Welcome {
	t.Fatalf("WelcomeFrame[0] = %#x, want 'W'(%#x)", wf[0], Welcome)
}
var wp WelcomePayload
if err := json.Unmarshal(wf[1:], &wp); err != nil {
	t.Fatalf("WelcomeFrame payload unmarshal: %v", err)
}
if wp.Mode != ModeRO { t.Errorf("WelcomeFrame mode = %q, want %q", wp.Mode, ModeRO) }
```
扩展方向（RESEARCH §Wave 0）：prefs 往返行 + **omitempty 缺席行**（空 prefs 组帧后 JSON 无 `prefs` 键——可用 `bytes.Contains(wf, []byte("prefs"))` 反向断言或 unmarshal map 查键）。白名单键函数（ValidClientOptionKey）照 TestProtocolConstants（94-150）的逐字钉死形态加用例。

---

### `cmd/wesh/main_test.go`（修改）— 表驱动加行 + 错误子串表

**① 正常行表驱动**（main_test.go:27-98）——表加 `wantClientOptions`/`wantOSC52` 字段（P3 命名字段转换先例：wantCredentials 计数断言因 Credential 字段不导出；clientOptions 的 value 是 json.RawMessage 可 DeepEqual）：
```go
{name: "two credentials", args: []string{"--credential", "alice:pw1", "--credential", "bob:pw2", "--", "bash"},
	wantBind: "0.0.0.0", wantPort: 7681, wantPingInterval: 5 * time.Second, wantCredentials: 2, wantArgv: []string{"bash"}},
```

**② 错误行子串断言**（main_test.go:139-162，TestTLSKeyPairError 形态——--client-option 非法 key/非法 JSON 照此加表或新表）：
```go
tests := []struct {
	name    string
	args    []string
	wantSub string
}{
	{"tls-cert without key", []string{"--tls-cert", "/tmp/c.pem", "--", "bash"}, "both --tls-cert and --tls-key"},
	// 新增同形态：{"client-option bad key", …, "invalid --client-option key"}
	//             {"client-option bad JSON", …, "not valid JSON"}
	//             {"client-option osc52 rejected", …, …}（D-12 安全不对称）
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		_, _, err := parseArgs(tt.args)
		if err == nil {
			t.Fatalf("parseArgs(%v) = nil error, want containing %q", tt.args, tt.wantSub)
		}
		if !strings.Contains(err.Error(), tt.wantSub) {
			t.Errorf("parseArgs(%v) error = %q, want containing %q", tt.args, err, tt.wantSub)
		}
	})
}
```

---

## Shared Patterns

### 帧常量前后端手工对齐（加字段两侧同步注释）
**Source:** `web/src/main.ts:6-11` ↔ `internal/proto/proto.go:17-26`（注释互相指路）
**Apply to:** proto.go WelcomePayload 加 Prefs 时，main.ts 帧常量注释块同步提 prefs；proto.go 头部包注释（proto.go:1-13）同步。
```typescript
// main.ts:6-11 形态：帧常量与 internal/proto/proto.go 手工对齐（D-16，两侧注释互相指路）：
// …Hello 载荷 {version, cols, rows, ticket?}——ticket 为 Phase 3 认证核销一次性票…
// → Welcome 载荷注释同步加 prefs?（omitempty，P4 D-13）
```

### 未知字段忽略 = 演化纪律（禁 DisallowUnknownFields）
**Source:** `internal/proto/proto.go:72-82`（HelloPayload 注释）+ `main.ts:218-219`（default 分支静默跳过）
**Apply to:** WelcomePayload.Prefs 是加字段不是动协议；前端旧版本无 prefs 处理兼容；服务端对 prefs 不透明透传。

### 启动校验 fail-fast 分层
**Source:** `cmd/wesh/main.go:47-109`（parseArgs 报配置形态错误）+ `129-146`（validateStartup 报矩阵拒绝，exit 2）
**Apply to:** `--client-option` 白名单/JSON 校验在 fs.Func 回调内 parse 期（exit 2）；不进 validateStartup（无运行时矩阵语义）。错误文案纪律：`fmt.Errorf("invalid --client-option key %q", key)` 显式优于静默（P4 D-15）。

### CLI flag 全名无短选项 + 逐字段注释带决策号
**Source:** `cmd/wesh/main.go:40-53`（parseArgs 头注释枚举全部 flag 及其决策号）
**Apply to:** `--client-option`/`--osc52` 登记进 parseArgs 头注释清单（"共 11 个" → 共 13 个），config 字段尾注释引 P4 D-12/D-15。

### 前端安全门控三形态（本 phase 贯穿）
**Source:** `web/src/main.ts:93-97`（null 闸 + OPEN 闸双门）+ RESEARCH §Pitfall 5
**Apply to:** 所有新事件 handler：
- `navigator.clipboard` 存在性门控（`typeof navigator.clipboard !== 'undefined'`，非安全上下文整体静默，D-11）
- `isRO` 门控粘贴（ro 不读剪贴板，D-10）
- 写/读失败一律 `.catch((e) => console.warn(...))` 静默（main.ts:69 `console.warn` 先例）
- `welcomeDone` 门控浮层（onopen 初次 fit 不触发）

### 测试红线（SEC-01 延伸）
**Source:** `web/uat/phase03.mjs:6-8` + `cmd/wesh/main_test.go:200-203`
**Apply to:** phase04.mjs detail 永不打印凭据/prefs 敏感值；Go 测试错误文案不含凭据值。

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/src/lib/*.ts`（A3 可选项：sanitizeTitle/parseQuery/applyPrefs 纯函数抽取 + `node --test` 用例） | utility | transform | `web/src/` 仅 main.ts 单文件，无前端工具模块与前端单测先例。若 planner 采纳 A3：纯函数从 main.ts 内联逻辑抽取，测试用 Node 24 内建 `node --test` type stripping 零新依赖（RESEARCH §A3）；不采纳则这些逻辑走人工 UAT + 代码评审 |

**备注：** xterm addon 的 API 用法（Unicode11Addon/WebLinksAddon/ClipboardAddon 构造与 provider 形状）**代码库内无 analog**——逐字契约以 `04-RESEARCH.md §Pattern 1-6`（已逐包核实产物源码）与 `04-UI-SPEC.md` 为准，PATTERNS 不重复转录。

## Metadata

**Analog search scope:** `web/src/`、`web/uat/`、`web/package.json`、`internal/proto/`、`internal/server/`、`cmd/wesh/`
**Files scanned:** 8（main.ts 322 行、proto.go 150 行、server.go 目标两段 140 行、main.go 241 行、main_test.go 关键三段、proto_test.go 151 行、phase03.mjs 361 行、package.json——全部 Read 逐行核实）
**Pattern extraction date:** 2026-08-18
