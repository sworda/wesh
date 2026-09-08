# Phase 14: 双模式验证矩阵、标定与 herdr UAT - Pattern Map

**Mapped:** 2026-09-06
**Files analyzed:** 9（新建 4 + 修改 5 组）
**Analogs found:** 8 / 9（run-all.mjs 无精确类比，见末节）

> 本 phase 零产品代码变更——全部模式为测试 harness / UAT 脚本 / 文档层。三维归类改造（~13 个 _test.go 文件）是同构批量的单条模式分配。

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/server/harness_test.go`（newTestServer 收编点） | test harness | request-response | `internal/server/e2e_test.go` + `perclient_test.go`（两族母本） | exact（即两族直传） |
| `internal/server/*_test.go` 三维归类改造（e2e/multi/exit/emptyexit/shutdown/metrics/slowclient/health/resize/resize_arb/clients/stopseq 等 ~13 文件） | test（改造） | request-response | `internal/server/perclient_test.go` + `e2e_test.go` | exact |
| `internal/server/load_test.go`（新增 pc_flood / pc_resident 格） | test（load，//go:build load） | streaming / batch | 同文件 `TestLoadFanoutMatrix`(:298) + `TestChurnPerClientSpawnThrottle`(:725) | exact（自文件） |
| `web/uat/phase14.mjs`（herdr 协议层 UAT） | test（协议 UAT 脚本） | streaming（WS 帧）+ request-response | `web/uat/phase13.mjs` | exact（同构第三代母本） |
| `web/uat/pw/phase14-pw.mjs`（Windows 双 tab 观感） | test（Playwright） | event-driven（浏览器） | `web/uat/pw/phase12-pw.mjs` | exact |
| `web/uat/run-all.mjs`（一键矩阵 runner） | utility script | batch（串行 spawn + exit code 聚合） | 无精确类比（部分：phase13.mjs 收口段门禁形态） | none（见末节） |
| `README.md`（新增「会话模式」节 + 标定表回填） | docs | — | README.md:96-98 段形态 | exact（同位邻居） |
| `docs/ARCHITECTURE.md`（双模式段 + :7 GoTTY 误记修正） | docs | — | docs/ARCHITECTURE.md:9-21 mermaid 图形态 | exact（文件自身） |
| `docs/CONFIGURATION.md`（max-clients per-client 语义行） | docs | — | docs/CONFIGURATION.md:170-178 段形态 | exact（同位邻居） |

## Pattern Assignments

### `internal/server/harness_test.go`（test harness，request-response）

**Analog:** `internal/server/e2e_test.go`（shared 族）+ `internal/server/perclient_test.go`（per-client 族）

**Imports pattern**（e2e_test.go:1-23——全仓 server 测试统一外部包形态）:
```go
package server_test

import (
	"context"
	// ...
	"github.com/coder/websocket"

	"github.com/sworda/wesh/internal/proto"
	"github.com/sworda/wesh/internal/pty"
	"github.com/sworda/wesh/internal/server"
)
```

**核心装配 pattern——shared 族母本**（e2e_test.go:171-194，启动期 spawn + handler 追踪 + t.Cleanup 收口）:
```go
func startTrackedServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, waitHandlers func()) {
	t.Helper()
	sess, err := pty.Start(argv, pty.StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	exitCh = make(chan int, 1)
	srv := server.New(sess, func(code int) { exitCh <- code }, opts)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	t.Cleanup(func() { killServer(ln, sess) })
	var wg sync.WaitGroup
	h := srv.Handler()
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()
		h.ServeHTTP(w, r)
	}))
	return exitCh, "ws://" + ln.Addr().String() + "/ws", wg.Wait
}
```

**核心装配 pattern——per-client 族母本**（perclient_test.go:60-106，attach 期 spawn + SpawnFunc 追踪 + Cleanup 逐一 Kill+Close）:
```go
func startPerClientServerWithSpawn(t *testing.T, spawnFn func(cols, rows int, remoteUser string) (*pty.Session, error), mutate func(*server.Options)) (exitCh chan int, wsURL string, srv *server.Server, spawnedSessions func() []*pty.Session) {
	t.Helper()
	var mu sync.Mutex
	var spawned []*pty.Session
	exitCh = make(chan int, 1)
	opts := server.Options{
		SessionMode: server.SessionModePerClient,
		SpawnFunc: func(cols, rows int, remoteUser string) (*pty.Session, error) {
			sess, err := spawnFn(cols, rows, remoteUser)
			if err != nil {
				return nil, err
			}
			mu.Lock()
			spawned = append(spawned, sess)
			mu.Unlock()
			return sess, nil
		},
		Writable: true,
	}
	if mutate != nil {
		mutate(&opts)
	}
	srv = server.New(nil, func(code int) { exitCh <- code }, opts)
	// ...net.Listen + t.Cleanup{ln.Close(); 逐 sess Kill+Close} + go http.Serve
}
```

**薄包装形态**（perclient_test.go:114-122，newTestServer per-client 分支的直传目标）:
```go
func startPerClientServer(t *testing.T, argv []string, mutate func(*server.Options)) (exitCh chan int, wsURL string) {
	t.Helper()
	exitCh, wsURL, _, _ = startPerClientServerWithSpawn(t, func(cols, rows int, remoteUser string) (*pty.Session, error) {
		opts := pty.StartOptions{Uid: -1, Gid: -1}
		opts.RemoteUser = remoteUser
		return pty.StartWithSize(argv, opts, cols, rows)
	}, mutate)
	return exitCh, wsURL
}
```

**模式常量**（clients.go:110-114，t.Run 子测试名逐字值——`mode=shared` / `mode=per-client`）:
```go
const (
	SessionModeShared = "shared" // 默认（REQUIREMENTS 反特性 A5）
	SessionModePerClient = "per-client"
)
```

**关键不对称点（newTestServer 收编设计必须处理）**：shared 族在 `server.New(sess, ...)` 前**启动期 spawn**；per-client 族 `server.New(nil, ...)` + SpawnFunc 闭包**attach 期 spawn**。RESEARCH §Pattern 1 已给出收编骨架（修订后为**小族四形态**：newTestServer 普通二值 / newTrackedTestServer 带 waitHandlers / newHandleTestServer 带 waitHandlers+srv / newSessTestServer 带 sessions 访问器——二值签名无法承载修订期实证的特殊调用点（limits:132/auth_e2e:403/proxy_e2e:46,218,281/multi:756/emptyexit:250,322/health:251/shutdown:107,137/resize_arb:111,144,190,248）；per-client 族补 startPerClientServerTrackedWithSpawn 姊妹变体（perclient_test.go 新增，wg 包裹 handler 镜像 startTrackedServerWith :186-193——per-client 族原无 handler 追踪形态）。注意**包墙**：clients/resize/sharetoken 等 12 个 `package server` 白盒文件结构性不可达 server_test 包的 harness 小族——sharetoken 走本地 startShareServer 加 mode 参数的白盒镜像（14-06），clients/resize 实测为纯函数零装配保持单跑（14-04 登记）。

---

### `internal/server/*_test.go` 三维归类改造（test，request-response）

**Analog:** 同上两族母本 + `perclient_test.go` 头注释纪律

**断言分叉表 pattern**（D-01/D-02 形态——RESEARCH Code Examples 已给出骨架；纪律本体在 perclient_test.go:3-14 头注释）:
```
// perclient_test.go:3-14 头注释红线（逐字沿用为改造期纪律）：
// 「Pitfall 11 红线——shared 零回归证据不动一行，既有测试期望值
// 逐字未动，禁止断言放宽成『两模式都接受』形态」
// 「夹具纪律红线：客户端 Read 永不带 per-read deadline——静默窗口一律
// select + time.After 竞速形态，统护 ctx 只做护栏」
```

改造形态 = 每个 mode-agnostic / mode-mapped 测试体包一层：
```go
for _, mode := range []string{server.SessionModeShared, server.SessionModePerClient} {
	t.Run("mode="+mode, func(t *testing.T) {
		_, wsURL := newTestServer(t, mode, argv, nil)
		// 断言分叉点显式成表（mode → expected），shared 列期望值与 v1.0 逐字一致
	})
}
```

**mode-exclusive（D-03 显式断言未装配）**：resize_arb_test.go / clients_test.go 等 shared-only 测试的 per-client 分支断言组件不装配（resize 直通无 min-rect 约束、线上零 'W' 帧）——做成可证伪断言，否决 t.Skip。

**收口纪律**（e2e_test.go:120-134 killServer 注释——泄漏级联减速教训，改造后必须保留）:
```go
// killServer 收口测试服务端资源：关 listener + 杀子进程 + 关 PTY master。
// ...必须在每个装配函数的 t.Cleanup 调用——泄漏的子进程（尤其 seq 大洪水）在测试
// 返回后继续输出...（ubuntu-latest 实测级联减速：吞吐 9.4MB/s → 140KB/s）
func killServer(ln net.Listener, sess *pty.Session) {
	ln.Close()
	if sess.Cmd != nil && sess.Cmd.Process != nil {
		_ = sess.Cmd.Process.Kill()
	}
	_ = sess.Close()
}
```

---

### `internal/server/load_test.go` 新增格（test，streaming/batch）

**Analog:** 同文件 `TestLoadFanoutMatrix`(:298-374) + `TestChurnPerClientSpawnThrottle`(:725-835)——夹具层全部既有，新格只换装配与断言面

**文件头纪律**（load_test.go:1-12，新格同文件内自动继承）:
```go
//go:build load

// 运行（手动，build tag 隔离不进常规 CI）：
//	go test -tags=load -count=1 -timeout=30m ./internal/server/ -v
// 首行硬纪律：//go:build load 必须是文件首行
```

**夹具复用清单**（零新写）：drainClient(:80) / drainRateLimited(:103) / awaitDrain(:126) / assertClosed1000(:139) / dialLoadClient(:149) / scrapePeakSampler(:174) / allocPeakSampler(:211) / readAlloc(:232) / loadFloodLast(:242) / gatedFloodArgv(:256) / startLoadSamplers(:268) / countFds(:555) / readProcState(:567)。

**洪水格 pattern——shared 母本**（load_test.go:298-372，含 LOADDATA 行格式）:
```go
func TestLoadFanoutMatrix(t *testing.T) {
	last := loadFloodLast()
	for _, n := range []int{1, 4, 16, 32} {
		n := n
		t.Run(fmt.Sprintf("clients_%d", n), func(t *testing.T) {
			_, wsURL, _ := startTrackedServerWith(t, gatedFloodArgv(last), server.Options{
				Writable: true, WritePolicy: "all",
			})
			// ...N 端 dialLoadClient + drainClient；runtime.GC() 基线；startLoadSamplers
			if err := conns[0].Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'}); err != nil {
				t.Fatalf("write 触发 INPUT: %v", err)
			}
			// awaitDrain 240s/端 → assertClosed1000 → 字节逐端相等 + 流尾末位字段==last
			// → kicks 精确==0 + 放大比断言 → LOADDATA 行：
			t.Logf("LOADDATA cell=fanout clients=%d profile=seq_flood(last=%d) slowlink=none kicks=%d ...", ...)
		})
	}
}
```

**per-client 格的关键差异点**（RESEARCH §Pattern 4 实证结论——每会话独立 stdin，单端触发先例必须调整）:
```go
// 装配换 churn 格同款：startPerClientServer(t, gatedFloodArgv(last), nil)
// 【与 shared 格的关键差异】每客户端各自触发（每会话独立 stdin）：
for _, c := range conns {
	c.Write(ctx, websocket.MessageBinary, []byte{proto.Input, 'x', '\n'})
}
// 断言面：每端收流完整 + wesh_pty_spawn_total == N（churn 格 :813 程序序精确对照先例）+ kicks==0
```

**驻留格 pattern——双采样差值断言母本**（load_test.go:725-835 churn 格）:
```go
// 断言三面纪律（:809-831——Pitfall 7：禁无基线对照的绝对上限）：
if spawnTotal != int64(attached) { t.Fatalf(...) }          // 程序序精确对照
if endGor > baseGor+8 { t.Fatalf(...) }                     // 差值上界（容差注释论证）
const memDeltaCeil = int64(16 * 1024 * 1024)                // 标定来源必须注释论证
if endMem-baseMem > memDeltaCeil { t.Fatalf(...) }
// 回收窗口轮询禁固定 sleep（:776-789）：200ms 间隔轮询 + 10s 护栏到期转 FAIL
```

**驻留格断言口径（D-12）**：Alloc 增量 ≤ N×(768KiB outbox+inputQ 账面 + ε)——账面锚点 clients.go:36 `defaultOutboxBytes = 512*1024` / :50 inputQ 256KiB；子进程观测经 readProcState 同文件 /proc 先例读 `/proc/<pid>/status` VmRSS 入 LOADDATA + ≤15MB/进程宽松上界。

---

### `web/uat/phase14.mjs`（test，streaming + request-response）

**Analog:** `web/uat/phase13.mjs`（同构第三代母本，逐字可复用面最大）

**文件头 pattern**（phase13.mjs:1-44——场景清单 + 红线声明 + 时序纪律 + 运行方式四段式头注释）:
```js
// Phase 13 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch/child_process）。
// 覆盖 ...：
//   S1 ...
// 红线（phase12.mjs:18-21 纪律逐字沿用）：token/凭据/pid 数值只作断言材料，永不
// 进入 check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/文案常量
// 时序纪律：真实等待，超时上限只做护栏，禁精确时点断言；时钟不 mock
// 运行：node web/uat/phaseNN.mjs [wesh 二进制路径]（默认 /tmp/wesh-uat/wesh；
// 先构建：go build -o /tmp/wesh-uat/wesh ./cmd/wesh）
import { spawn, execFileSync } from 'node:child_process';
const WESH = process.argv[2] ?? '/tmp/wesh-uat/wesh';
```

**check/skip 门禁 pattern**（phase13.mjs:67-80）:
```js
const results = [];
const emittedDetails = []; // assertOutputClean 遍历材料——红线运行时自净断言
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
const skip = (id, name, reason) => { /* ok:null 形态——CODEBUDDY.md §5 显式豁免 */ };
```

**startWesh 真实二进制 spawn pattern**（phase13.mjs:106-134——`--bind 127.0.0.1 --port 0` + stdout 两行解析 + stderr 持续捕获 + kill 恒 SIGKILL）:
```js
function startWesh(args) {
  return new Promise((resolve, reject) => {
    const child = spawn(WESH, ['--bind', '127.0.0.1', '--port', '0', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    // 解析 listening on / share read-only 行 → resolve({port, shareRO, shareRW, stderrText, kill, child})
    // kill 恒 SIGKILL（CPU 受限 CI 泄漏级联减速实证纪律）
  });
}
```

**WS 握手/帧收集 pattern**（phase13.mjs:143-163 dialHello；:171-203 dialAttach——Pitfall 4 binaryType 纪律已在位）:
```js
const ws = new WebSocket(url, [SUBPROTOCOL]);
ws.binaryType = 'arraybuffer';   // 必须——默认 'blob' 使 new Uint8Array(ev.data) 抛异常
const frames = [];
ws.onmessage = (ev) => frames.push(new Uint8Array(ev.data));
ws.onopen = () => ws.send(helloFrame({ cols, rows }));
// 10s watchdog 防挂死 + poll 至 frames.some(f => f[0] === WELCOME)
```

**pid 观测 pattern**（phase13.mjs:253-297——pgrep 计数陷阱的规避通道，Pitfall 3）:
```js
const waitScanPid = async (frames, tag) => { /* 轮询既有帧扫 <TAG>=<pid>——pid 数值只作断言材料 */ };
const pgroupAlive = (pid) => { try { process.kill(-pid, 0); return true; } catch (e) { if (e.code === 'ESRCH') return false; throw e; } };
const pollESRCH = async (pid, guardMs) => { /* 50ms 轮询进程组至 ESRCH；超时返回 false 由断言转 FAIL */ };
```

**ro ticket 全链 pattern**（phase05.mjs:262-274——D-08 ro 汇聚场景的唯一通道，Pitfall 8）:
```js
const respRO = await fetch(`http://127.0.0.1:${inst.port}/api/attach`, {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ token: tokenFromUrl(inst.shareRO) }),
});
const bodyRO = await respRO.json();
const cRO = await dialHello(inst.port, { ticket: bodyRO.ticket }); // Hello JSON 携 ticket 键——WS query 参数无效
// 自检断言 Welcome.mode==='ro' 落到 check 行（防假绿）
```

**收口门禁 pattern**（phase13.mjs:748-769——串行场景 + 异常纳入 emittedDetails + exit code）:
```js
const scenarios = [s1..., s2..., ...];
let failed = 0;
for (const s of scenarios) {
  try { await s(); } catch (e) { failed++; emittedDetails.push(String(e.message)); console.log(`  FAIL  场景异常: ${e.message}`); }
  await sleep(300);
}
assertOutputClean(); // phase13.mjs:740-746 输出自净（token/pid 零泄漏运行时自证）——必须沿用
// 汇总行 + process.exit(failedN === 0 && failed === 0 ? 0 : 1)
```

**herdr 增量面（phase14.mjs 独有，RESEARCH §Pattern 3 spike 标定产物直接落地）**:
```js
const SESSION = `wesh-uat-p14-${process.pid}`;
const SOCK = `${os.homedir()}/.config/herdr/sessions/${SESSION}/herdr.sock`;
// wesh argv: --session-mode=per-client --writable -- <HERDR> --session <SESSION>
// API 观测（关系断言，禁硬编码 chrome 常量——Pitfall 2）：
const snap = JSON.parse(execFileSync(HERDR, ['api', 'snapshot'],
  { env: { ...process.env, HERDR_SOCKET_PATH: SOCK }, encoding: 'utf8' }));
const area = snap.result.snapshot.layouts[0].area; // 恒等于前台客户端 pane 面积
// 就绪门：轮询 snapshot 至 layouts 非空（禁固定 sleep）
// pane 观测：herdr pane read w1:p1 --source visible（标记命令只用 echo MARKER_STR——Pitfall 6 fish 兼容）
// 清理 finally 兜底：wesh SIGTERM → herdr session stop <SESSION> → session list 核验 stopped
```

---

### `web/uat/pw/phase14-pw.mjs`（test，event-driven 浏览器层）

**Analog:** `web/uat/pw/phase12-pw.mjs`（resize 观感先例，lib 载具四件套全复用）

**lib 载具 imports pattern**（phase12-pw.mjs:29-34）:
```js
import { mkdirSync } from 'node:fs';
import { Check, sleep } from './lib/check.mjs';
import { Forwarder } from './lib/forwarder.mjs';
import { TARGET_HOST, TARGET_PORT, ssh, ensureRunSh, startWesh, stopWesh } from './lib/server.mjs';
import { launch, CRED, openSession, runCmd, waitTermText, panel, fireOnline } from './lib/browser.mjs';
```

**双机拓扑 pattern**（phase12-pw.mjs:127-136——TCP 转发器 + Linux 侧 startWesh）:
```js
const fwd = new Forwarder(PORT_BASE, TARGET_HOST, TARGET_PORT);
await ensureRunSh();
await startWesh(`--session-mode per-client --writable --insecure-http --credential ${CRED} -- ...`);
await fwd.start();
browser = await launch();
// finally: browser.close() → fwd.stop() → stopWesh()
```

**buffer 观测通道 pattern**（phase12-pw.mjs:50-60——D-07 断言面的直接通道）:
```js
const renderRows = (page) =>
  page.evaluate(() => document.querySelector('.xterm-rows')?.childElementCount ?? -1);
// 行文本断言：page.evaluate 读 .xterm-rows > div textContent（:200-201 形态）
// waitTermText(page, /regex/, timeout)——buffer 文本轮询锚定
```

**模式自证防线 pattern**（phase12-pw.mjs:165-174 T1b——per-client 假绿防线，phase14-pw 双 tab 场景同构必带）:
```js
const pidA = await shellPid(page);
const page2 = await ctx.newPage();
await openSession(page2, `${BASE}/`);
const pidB = await shellPid(page2);
t1.ok(pidA !== null && pidB !== null && pidA !== pidB,
  'per-client 模式自证：两浏览器页 shell PID 不同', `pidA≠pidB=${pidA !== pidB}`);
```

**Check 收集 + 截图留档 + exit code 收口 pattern**（phase12-pw.mjs:120-127, 280-286）:
```js
const t0 = new Check('P12-T0', '...');
mkdirSync('screenshots', { recursive: true });
await page.screenshot({ path: 'screenshots/p14-*.png' }); // 截图留档人工复核（像素视觉豁免）
// 尾：for (const t of [...]) { const s = t.summary(); if (s.total > 0) results.push(s.pass); }
// process.exit(failed ? 1 : 0)
```

**视口选型先例**（phase12-pw.mjs:37-39 阶梯形态；phase14 双 tab = 桌面 ~1600x1000 / 移动小屏两 context——RESEARCH A5：cols/rows 以 stty 实测回读为准，不硬编码）。

---

### 文档三件套（README.md / docs/ARCHITECTURE.md / docs/CONFIGURATION.md）

**Analog:** 各文件同位邻居段——D-14 落点形态

**README 段 pattern**（README.md:96-98——单段致密中文叙事 + flag 值 inline code + 模式分岔表述）:
```markdown
`--session-mode=shared|per-client` 选择会话模式（默认 `shared`）。`--stop-timeout` 默认值按模式分岔：`shared` 默认 `0`（...）；`per-client` 未显式设置默认 `5s`（...）。

**保活先杀时序**（默认 `--ping-interval=5s`）：...自管 WebSocket socket 的客户端（如 herdr 类）若停读则适用——...
```
新「会话模式」节即此形态扩展：模式语义表 + herdr 配方示例（```sh 代码块）+ 资源义务段含标定表（09-09 D-13 表格形态）。tmux 对照句措辞红线：tmux 3.6 默认 `window-size latest`，**不得写「默认取最小值」**（RESEARCH §State of the Art 实证修正）。

**ARCHITECTURE mermaid pattern**（ARCHITECTURE.md:9-21——`graph TD` + subgraph 分层 + `<br/>` 换行节点文案）:
```markdown
```mermaid
graph TD
    FE[浏览器前端<br/>web/src/main.ts · xterm.js]
    subgraph internal/server（网关层）
        HTTP[mux 路由 + 认证链<br/>basicAuth · throttle · origin · 安全头]
```
双模式段 goroutine 拓扑图沿用此形态（CODEBUDDY.md 文档规则：技术文档用 mermaid）。:7 误记修正点在段内长句：「采用 **GoTTY 式共享进程模型**」→ 修正为 GoTTY 实为 per-connection spawn + 双模式分岔说明（D-14 措辞见 CONTEXT）。

**CONFIGURATION 段 pattern**（CONFIGURATION.md:170-178——`### flag 名` 小节 +  bullet 列表分模式叙事）:
```markdown
### `ping-interval` 与断开时序
...**pong 超时（1006）先于慢客户端踢出（1013）**——
- 默认 `--ping-interval=5s` 下，...
- **真实浏览器结构性不会触发**：...
```
max-clients per-client 语义行在默认值表（:165-166 表格）补行 + 必要时分岔说明 bullet。

---

## Shared Patterns

### UAT 红线：敏感值零泄漏（token/pid）
**Source:** `web/uat/phase13.mjs:67-98, 740-746`
**Apply to:** phase14.mjs、run-all.mjs（pw 层由 lib/browser.mjs 头注释纪律继承）
```js
const sensitiveTokens = []; // 分享链接 token 留入闭包数组（只作断言材料）
const sensitivePids = [];   // pid 数值同红线（断言一律以布尔「不等/ESRCH 到达」表达）
// assertOutputClean()：遍历 emittedDetails，命中 /s/ 链接形态串/token 值/pid 数值即 FAIL
```

### 时序纪律：真实等待 + 护栏上限，禁固定 sleep 精确时点
**Source:** `web/uat/phase13.mjs:36-41`（头注释）+ `load_test.go:776-789`（回收窗口轮询）
**Apply to:** phase14.mjs、phase14-pw.mjs、load_test.go 新格、全部改造测试
- UAT 侧：宽限/退避类场景真实等待，超时上限只做护栏；服务端进程退出经 `waitExit(child, timeoutMs)` 通道（phase13.mjs:213-218）
- Go 侧：回收窗口 200ms 间隔轮询 + 护栏到期转 FAIL（禁精确时点断言）

### 进程/会话清理兜底
**Source:** `internal/server/e2e_test.go:120-134`（killServer + t.Cleanup）+ `web/uat/phase13.mjs:440-444`（finally 免疫进程 kill -9）
**Apply to:** 全部新/改测试与 UAT 脚本；phase14.mjs 额外加 herdr 序列：`wesh SIGTERM → herdr session stop <name> → session list 核验 stopped`（wesh 死后 herdr server 存活是特性，必须显式 stop——RESEARCH §3.5）

### 装配收口：t.Cleanup 追踪全部已 spawn 会话
**Source:** `internal/server/perclient_test.go:88-98`
**Apply to:** newTestServer per-client 分支、load_test.go per-client 格
```go
t.Cleanup(func() {
	ln.Close()
	mu.Lock()
	defer mu.Unlock()
	for _, sess := range spawned {
		if sess.Cmd != nil && sess.Cmd.Process != nil {
			_ = sess.Cmd.Process.Kill()
		}
		_ = sess.Close()
	}
})
```

### LOADDATA 标定回填链
**Source:** `internal/server/load_test.go:370-371, 833-834`
**Apply to:** load_test.go 新格（pc_flood / pc_resident）
```go
t.Logf("LOADDATA cell=<name> clients=%d profile=... kicks=%d outbox_max=%d alloc_peak=%d alloc_base=%d dur_ms=%d", ...)
// 每格产出 LOADDATA 行 → README 标定表数据源（09-09 D-13 先例）；per-client 格字段加 sessions=N spawn_total=N rss_per_proc_max=…
```

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/uat/run-all.mjs` | utility script | batch（串行 spawn + exit code 聚合） | 仓库无既有 runner——16 项既有脚本此前靠人工逐跑（13-08 收口闸登记的「15」为 phase13 加入前快照，+phase13 = 16；+phase14.mjs = 17 项矩阵）。部分先例可用：phase13.mjs:748-769 收口段（串行执行 + 异常计数 + 汇总行 + `process.exit(0/1)` 门禁语义）即 runner 的单脚本内形态；runner 只需把它提升到跨脚本层——`spawn('node', [script])` 串行 + exit code 聚合 + 汇总表。脚本枚举清单以 RESEARCH §Pattern 5 的 16 项既有脚本基线表为准；pw 层不纳入（Windows 侧独立执行入口，RESEARCH Open Question 3 裁决） |

## Metadata

**Analog search scope:** `internal/server/`（e2e_test.go / perclient_test.go / load_test.go / clients.go）、`web/uat/`（phase05/11/13.mjs）、`web/uat/pw/`（phase12-pw.mjs + lib/）、`README.md` / `docs/ARCHITECTURE.md` / `docs/CONFIGURATION.md`
**Files scanned:** 10 个类比文件（其中 perclient_test.go 为 >2000 行大文件，按需读取 :1-141 装配区段）
**Pattern extraction date:** 2026-09-06
