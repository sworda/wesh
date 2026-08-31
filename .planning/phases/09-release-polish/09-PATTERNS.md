# Phase 9: 发布与打磨 - Pattern Map

**Mapped:** 2026-08-29
**Files analyzed:** 22（新建 12 + 修改/扩展 10）
**Analogs found:** 20 / 22（2 个纯新建件以 RESEARCH.md 定稿配方为准）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `.github/workflows/release.yml`（新建） | workflow-config | event-driven（tag push） | `.github/workflows/ci.yml` | role-match |
| `.github/workflows/ci.yml`（改，加 fuzz job） | workflow-config | event-driven | 自身（go/web 两 leg 结构） | exact |
| `.goreleaser.yml`（新建） | build-config | batch（交叉编译打包） | 无（仓内首件）→ RESEARCH §Code Examples 定稿 | none |
| `scripts/release.sh`（新建） | utility-script（bash） | batch（顺序编排+确认闸） | `web/uat/pw/phase07-a2-ctl.sh` | role-match |
| `Dockerfile`（新建） | container-config | batch（镜像构建） | 无（仓内首件）→ RESEARCH §Code Examples 定稿 | none |
| `deploy/wesh.service`（新建） | config（systemd unit） | — | `README.md:287-295` systemd 配方先例 | role-match |
| `cmd/wesh/main.go`（改：--index flag + 启动读入 + 校验矩阵扩展） | CLI-entry（parse/validate 分层） | request-response（启动一次性 file-I/O） | 自身 parseArgs/validateStartup | exact |
| `cmd/wesh/config.go`（改：两新键 + decodeFileConfig reader 委托接缝） | config（TOML 加载） | file-I/O（path-in → bytes-in 接缝） | 自身 loadFileConfig | exact |
| `cmd/wesh/fuzz_test.go`（新建） | test（fuzz） | transform（bytes-in 纯函数） | `cmd/wesh/config_test.go` 探针断言形态 + RESEARCH 定稿 | role-match |
| `cmd/wesh/config_test.go`（扩：index/index-max-size 键） | test（unit） | file-I/O | 自身 writeToml + 探针表驱动 | exact |
| `cmd/wesh/main_test.go`（扩：--index 校验矩阵行） | test（unit） | — | 自身 TestStartupMatrix/TestParseArgs | exact |
| `internal/proto/fuzz_test.go`（新建） | test（fuzz） | transform（bytes-in 纯函数） | `internal/proto/proto_test.go` Decode 表驱动 + RESEARCH 定稿 | role-match |
| `internal/server/server.go`（改：Options.CustomIndex + :449 装饰） | server-assembly | request-response | 自身 Handler() 装配（:447-530） | exact |
| `internal/server/sharetoken.go`（大概率零改动） | route-guard | request-response | 自身 sharePage 委托（:87-96） | exact |
| `web/embed.go`（可能改：自定义 index 装饰函数宿主） | static-handler | request-response（gzip/Vary） | 自身 Handler()（:24-51） | exact |
| `internal/server/load_test.go`（新建，`//go:build load`） | test（load 黑盒） | streaming（洪水 fan-out/慢链路） | `internal/server/e2e_test.go` + `slowclient_test.go` + `metrics_test.go` | exact |
| `internal/server/customindex_test.go`（新建，TestCustomIndex） | test（Go e2e） | request-response | `internal/server/e2e_test.go` fixture + `health_test.go` http 形态 | exact |
| `web/uat/phase09.mjs`（新建） | test（协议 UAT） | request-response（spawn 真实二进制） | `web/uat/phase07.mjs` | exact |
| `web/uat/pw/phase09-caddy-pw.mjs`（新建，Windows 侧） | test（Playwright 双机） | request-response | `web/uat/pw/phase07-a2-pw.mjs` | exact |
| `web/src/main.ts:881-905`（改：D-18 ①③） | browser-client | event-driven（onclose 分派） | 自身 HINT_RESTART 常量 + case 1001 | exact |
| `web/index.html:63`（改：D-18 ② role="alert"） | page-shell | — | 自身 #status 面板结构 | exact |
| `web/uat/phase06-dom.mjs`（扩：D-18 断言） | test（jsdom） | event-driven | 自身 SpyWebSocket/synthClose 夹具 | exact |
| `README.md`（改：发布节/--index 节/部署节扩充/标定表） | docs | — | 自身 nginx 配方节（:301-329）+ systemd 配方（:287-295）+ 标定表（:231-243） | exact |

---

## Pattern Assignments

### 1. `.github/workflows/release.yml`（新建，workflow-config）

**Analog:** `.github/workflows/ci.yml`（全文 30 行）

**钉版与步骤形态**（ci.yml:10-13, 21-30 逐行复用——D-03「与 ci.yml web leg 同版」）：
```yaml
      - uses: actions/checkout@v7.0.1
      - uses: actions/setup-go@v7.0.0
        with:
          go-version-file: go.mod
      # web leg：
      - uses: pnpm/action-setup@v6.0.10
        with:
          # web/package.json 无 packageManager 字段，显式钉版（与本地一致，lockfileVersion 9.0）
          version: 11.21.0
      - uses: actions/setup-node@v4
        with:
          node-version: 24
      - run: pnpm -C web install --frozen-lockfile
      - run: pnpm -C web build # tsc 类型检查 + vite 构建一体（D-18 构建顺序的 CI 固化）
```

**CGO 注释纪律**（ci.yml:15——release.yml 同样不设 CGO_ENABLED，由 .goreleaser.yml `builds.env` 单侧持有）：
```yaml
      # 注意：不设 CGO_ENABLED（保持默认启用）——-race 需要 cgo（Pitfall 5；该变量只属于 Phase 9 goreleaser 发布构建）
```

**差异点（RESEARCH 定稿新增面）：** `on.push.tags: ["v*"]` 触发；`permissions: contents: write`；checkout 加 `fetch-depth: 0`（Pitfall 11）；尾部 `goreleaser/goreleaser-action@v7.2.3` 步骤（`args: release --clean` + `GITHUB_TOKEN` env）。

---

### 2. `.github/workflows/ci.yml`（改：新增 fuzz job）

**Analog:** 自身——go/web 两 job 并列顶层结构（ci.yml:3-30）。

新增独立 `fuzz:` job 与 go/web 并列（两目标**两次调用**——`-fuzz` 单包单目标，RESEARCH Pitfall 4）：
```yaml
  fuzz:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: actions/setup-go@v7.0.0
        with:
          go-version-file: go.mod
      - run: go test -fuzz=FuzzDecodeHello -fuzztime=60s ./internal/proto/
      - run: go test -fuzz=FuzzDecodeFileConfig -fuzztime=60s ./cmd/wesh/
```
不加 `-race`（与 go leg 并行墙钟 +60-70s）；种子/崩溃语料随常规 `go test ./...` 零时长回归（go leg 既有行覆盖）。

---

### 3. `scripts/release.sh`（新建，utility-script）

**Analog:** `web/uat/pw/phase07-a2-ctl.sh`（bash 入库脚本形态先例）

**脚本头与结构纪律**（phase07-a2-ctl.sh:1-8, 46-94）：
```bash
#!/usr/bin/env bash
# Phase 07 UAT A2 控制脚本（Linux 侧）：专用一次性 nginx + wesh 实例生命周期
# 用法: a2-ctl.sh setup | variant exact|noexact | teardown
set -u
NGINX_DIR=/tmp/wesh-uat/a2-nginx
...
case "${1:-}" in
  setup) ... ;;
  *) echo "usage: a2-ctl.sh setup|variant exact|noexact|teardown"; exit 2;;
esac
```

**release.sh 在此形态上加固**：`set -euo pipefail`（发布脚本比 UAT 控制脚本更严——RESEARCH Pattern 5）；顶部用法注释；POSIX bash（**非 fish**——入库可移植 artifact，CODEBUDDY 纪律）；末尾 usage + `exit 2`（与 wesh exit 2 fail-fast 语义同族）。

**顺序（RESEARCH Pattern 5 定稿）：** 前置校验（`git status --porcelain` 空 / `v$X.Y.Z` 形态 / tag 不存在 / 与远端同步）→ `go vet ./... && go test -race -count=1 ./...` → `pnpm -C web install --frozen-lockfile && pnpm -C web build` → 长 fuzz 2×10min（两次调用）→ `go test -tags=load -count=1 -timeout=30m ./internal/server/` → 确认闸（回显 tag+最近提交）→ `git tag && git push origin`。

---

### 4. `deploy/wesh.service`（新建，systemd unit）

**Analog:** `README.md:287-295` 既有 systemd 配方先例

**既有入库配方形态**（README:289-295）：
```ini
# /etc/systemd/system/wesh.service
[Service]
RuntimeDirectory=wesh
EnvironmentFile=/etc/wesh/credentials   # chmod 600，内容为 WESH_CREDENTIAL=user:pass
ExecStart=/usr/local/bin/wesh --socket /run/wesh/wesh.sock --socket-owner www-data:www-data -- bash
```

**Phase 9 unit 模板在此先例上扩展**（D-17 全配 + RESEARCH 定稿）：`[Unit]` 段 `After=/Wants=network-online.target`；`EnvironmentFile=-/etc/wesh/credentials`（`-` 前缀缺席不拒）；`Restart=on-failure` + 注释写明 255 交互选型理由（Pitfall 10——`Restart=always` 会在会话终结后复活服务）；`RestartSec=2`；`TimeoutStopSec=15s`（1001 广播 + stall Close 5s+5s 上界余量）；`LimitNOFILE=65536`；注释明示不做 ProtectHome/ProtectSystem 默认加固（wesh 本职是 spawn 用户 shell）；`[Install] WantedBy=multi-user.target`。

**验证通道：** `systemd-analyze verify deploy/wesh.service` + P8 实机 systemctl 通道最小实测（08-05 draining 观测同通道）。

---

### 5. `cmd/wesh/main.go`（改：--index flag + 启动读入 + 校验矩阵扩展）

**Analog:** 自身——parseArgs 两阶段合并 + validateStartup 校验矩阵

**新 flag 注册形态**（main.go:306-321——StringVar 全名无短选项 + help 文案 + 注释引决策编号）：
```go
	fs.StringVar(&cfg.bind, "bind", bindDefault, "listen address")
	...
	// D-08：最大并发客户端数（one-way 公开契约——容量策略是部署关切开 flag，
	// 与 P2 D-10 攻击面上限常量不同类）。默认 32（ARCHITECTURE §6...），Phase 9 负载
	// 标定回填。满员行为：/ws Accept 前 HTTP 503（守卫区③位）+ /api/attach
	// 503 早闸（OQ2）；≤0 经 validateStartup 拒绝（exit 2 配置校验矩阵形态）。
	fs.IntVar(&cfg.maxClients, "max-clients", maxClientsDefault, "maximum simultaneous attached clients")
```

**TOML 配置键 → flag 默认值铺底形态**（main.go:229-304——`index`/`index-max-size` 两键同法接入）：
```go
	if fc != nil {
		if fc.MaxClients != nil {
			maxClientsDefault = *fc.MaxClients
		}
		if fc.StopTimeout != nil {
			d, perr := time.ParseDuration(*fc.StopTimeout)
			if perr != nil {
				return cfg, nil, configErr(configPath, "invalid duration", `key "stop-timeout"`)
			}
			stopTimeoutDefault = d
		}
		...
	}
```
注意：`index-max-size` 是**纯配置键**（D-08：不开 `--index-max-size` CLI flag——P7 D-03 纪律的明示例外），故只进 fileConfig 与合并段，**不注册 flag**；`index` 键正常进 fileConfig + 注册 `--index` flag。

**validateStartup 校验矩阵行形态**（main.go:927-937——--cwd 预检先例：纯配置有效性、loopback 早退之前判定、错误文案回显路径）：
```go
	// D-21 预检（配置错误 fail-fast，write-policy 行同位——纯配置有效性与
	// bind 安全形态无关，loopback 早退之前判定）：--cwd 非空时 os.Stat 只读
	// 探测...不存在或非目录即拒（spawn 前零资源占用...）。值非敏感，错误文案可回显路径
	if cfg.cwd != "" {
		if fi, serr := os.Stat(cfg.cwd); serr != nil || !fi.IsDir() {
			return "", fmt.Errorf("invalid --cwd %q: not an existing directory", cfg.cwd)
		}
	}
```
--index 四拒绝（不存在/不可读/非常规/超限）同位落矩阵；**差异纪律**：D-08 红线——错误行只含路径+类别，**绝不含文件内容字节**（比 --cwd 更严：HTML 内容是探针面，Pitfall 9）。

**启动读入形态**（RESEARCH Pattern 2 定稿）：`os.Stat` → `Mode().IsRegular()` 闸（目录/设备/socket 拒）→ `io.LimitReader(f, max+1)` 读入 → `len(data) > max` 拒（max 默认 16MiB）；读入产物 []byte + gzip 预压缓存经 `server.Options.CustomIndex` 传给 server 装配层（Options 生产直传字段，BasePath/AuthHeader/Version 先例形态）。

---

### 6. `cmd/wesh/config.go`（改：两新键 + decodeFileConfig reader 委托接缝）

**Analog:** 自身——fileConfig 结构 + loadFileConfig

**fileConfig 指针键形态**（config.go:42-72——两新键追加位；注释引 D 编号）：
```go
type fileConfig struct {
	Port          *int     `toml:"port"`
	...
	MaxClients    *int     `toml:"max-clients"`
	...
	// D-04 排除项不在结构体：no-auth/insecure-http/version/help/config
	// → 严格模式以「未知键」拒绝（逃生门必须显式说出口）。
}
```
追加 `Index *string \`toml:"index"\`` 与 `IndexMaxSize *int \`toml:"index-max-size"\``（整数字节形态，RESEARCH OQ1 推荐——与 max-clients 整数先例同型，零新解析代码）；文件头「覆盖面 = 27 键」注释同步改写（27→29）。

**fuzz 接缝重构（RESEARCH Pattern 3 必要前置）**——loadFileConfig（config.go:97-137）提取 reader 委托：
```go
// 现状（path-in）：
func loadFileConfig(path string) (fc *fileConfig, warn string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", configErr(path, "cannot read", err.Error())
	}
	defer func() { _ = f.Close() }()
	var decoded fileConfig
	derr := toml.NewDecoder(f).DisallowUnknownFields().Decode(&decoded)
	// ... 错误分类三分支（StrictMissingError/DecodeError/兜底）+ D-07 权限警告
}
```
提取后：`decodeFileConfig(path string, r io.Reader) (*fileConfig, error)` 承载解码 + 错误分类 + configErr 包装（**单写口不动**）；`loadFileConfig` 缩为 open-file + 委托 + D-07 权限警告。错误分类三分支（config.go:105-126）逐字迁入委托函数——值剥离红线逻辑不许复制第二份。

---

### 7. `cmd/wesh/fuzz_test.go`（新建，fuzz test）

**Analog:** `cmd/wesh/config_test.go` 探针断言形态 + RESEARCH §Code Examples 定稿

**红线探针断言先例**（config_test.go:208-226——fuzz 不变量断言的直接祖先）：
```go
	t.Run("unknown key rejected, value stripped", func(t *testing.T) {
		// D-06 严格模式：未知键拒绝；红线运行时自证——凭据值探针串
		// "s3cr3t-probe" 写入 TOML 后断言错误输出零出现（RESEARCH Pitfall 5：...
		path := writeToml(t, "credential = [\"alice:s3cr3t-probe\"]\nno-auth = true\n", 0o600)
		_, _, err := loadFileConfig(path)
		...
		if strings.Contains(err.Error(), "s3cr3t-probe") {
			t.Errorf("error = %q, must not contain credential value probe", err)
		}
	})
```

**FuzzDecodeFileConfig 定稿形态**（RESEARCH §Code Examples）：`package main`（main 包内测试，调未导出 decodeFileConfig）；种子 = 合法键 / 探针键 / 未知键 / 类型不符 / 非 UTF-8 五类；不变量两枚——不 panic + err.Error() 不含 "FUZZ_PROBE_SECRET"（值剥离红线的 fuzz 断言形态；键名回显合法不在断言面）。

---

### 8. `internal/proto/fuzz_test.go`（新建，fuzz test）

**Analog:** `internal/proto/proto.go` DecodeHello/DecodeResize/ClampDim 契约 + RESEARCH 定稿

**被测签名与不变量依据**（proto.go:136-144, 203-209, 212-220）：
```go
// DecodeHello 解码 Hello 帧载荷 {"version":V,"cols":C,"rows":R}。
// 解码失败返回 ok=false；成功时 Cols/Rows 经 ClampDim 钳制到 [1,1000] 后返回。
func DecodeHello(payload []byte) (HelloPayload, bool) {
	var hp HelloPayload
	if err := json.Unmarshal(payload, &hp); err != nil {
		return HelloPayload{}, false
	}
	hp.Cols = ClampDim(hp.Cols)
	hp.Rows = ClampDim(hp.Rows)
	return hp, true
}

func DecodeResize(payload []byte) (cols, rows int, ok bool) { ... }

func ClampDim(v int) int {
	if v < 1 { return 1 }
	if v > 1000 { return 1000 }
	return v
}
```

**fuzz 形态**（RESEARCH §Code Examples 定稿）：`package proto_test`（外部包，与 proto_test.go 同）；种子 = 合法 / 负值超大 / 截断 / 空 / 类型混乱五类；FuzzDecodeHello 不变量 = 成功 ⇒ 尺寸恒在 [1,1000]（ClampDim 契约）+ 任意输入不 panic；FuzzDecodeResize 同法（cols/rows 返回值同样经 ClampDim）。bytes-in 纯函数零改造直挂——无接缝重构需求（与 TOML 侧的关键差异）。

---

### 9. `internal/server/server.go` + `web/embed.go`（改：CustomIndex 装饰）

**Analog:** 自身——Handler() 装配（server.go:447-530）+ web.Handler（embed.go:24-51）

**唯一装饰点**（server.go:447-455——全仓唯一 `web.Handler()` 调用点）：
```go
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	wh, err := web.Handler()
	if err != nil {
		// fs.Sub 仅在内嵌 FS 缺 dist 时失败（编译期 go:embed 已保证存在）；防御性 500。
		wh = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded assets unavailable", http.StatusInternalServerError)
		})
	}
	bp := s.basePath
	if len(s.credentials) > 0 {
		root := basicAuth(wh, s.credentials, s.throttle, s.proxy, &s.mc)
		...
		s.registerShareRoutes(mux, bp, wh, root)
	} else {
		...
		s.registerShareRoutes(mux, bp, wh, wh) // 无认证模式 page/root 同为 wh——给页无门
	}
```

**装饰语义（RESEARCH Pattern 2 定稿）：** `s.customIndex != nil` 时在此把 `wh` 包一层——`index.html` 路径（含空路径回落）返回启动读入字节，其余路径照旧委托原 wh（相对资源 404 是契约语义）。`/` 与 `/s/{token}/` 两通道**经 wh 单点装饰自然统一**——sharePage 委托（sharetoken.go:87-96）的 `page` 参数就是被装饰的 wh，sharetoken.go **零改动**：
```go
func (s *Server) sharePage(page, root http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/"
		if _, ok := s.shares.lookup(r.PathValue("token")); ok {
			page.ServeHTTP(w, r) // 有效 token：embed 页（token 即凭证，绕过 Basic）
			return
		}
		root.ServeHTTP(w, r)
	}
}
```

**gzip/Vary 纪律**（embed.go:34-48——自定义页伺服必须复刻的同款行为）：
```go
		// 同一 URL 对 gzip/非 gzip 客户端返回不同实体编码，两种表示都要带
		// Vary——否则中间缓存键不完整，可能把压缩体发给不接受 gzip 的客户端。
		w.Header().Set("Vary", "Accept-Encoding")
		if acceptsGzip(r.Header.Get("Accept-Encoding")) {
			if data, err := fs.ReadFile(sub, name+".gz"); err == nil {
				w.Header().Set("Content-Encoding", "gzip")
				...
			}
		}
```
自定义页：启动预压一次缓存（09-UI-SPEC §4 定稿采纳）；`Accept-Encoding` 显式含 gzip → 发预压体 + `Content-Encoding: gzip`；`Vary: Accept-Encoding` **恒发**；解析复用 web 包 `acceptsGzip`（embed.go:53-72——零第二份 Accept-Encoding 解析器）。

**Options 字段先例**（server.go:400-405 区域——CustomIndex []byte 生产直传，零值 nil = 与现状逐字节一致的兜底纪律）。

---

### 10. `internal/server/load_test.go`（新建，`//go:build load`）

**Analog 三件套:** `e2e_test.go`（fixture）+ `slowclient_test.go`（stall 纪律）+ `metrics_test.go`（scrape）

**文件首行硬纪律**（RESEARCH Anti-Pattern：build tag 必须是文件首行，否则常规 CI 捡起 30min 测试）：
```go
//go:build load

package server_test // 全仓测试统一外部包形态（e2e_test.go:1/limits_test.go:1/metrics_test.go:1）
```

**服务器装配夹具**（e2e_test.go:171-194——startTrackedServerWith 直接复用）：
```go
func startTrackedServerWith(t *testing.T, argv []string, opts server.Options) (exitCh chan int, wsURL string, waitHandlers func()) {
	t.Helper()
	sess, err := pty.Start(argv, pty.StartOptions{Uid: -1, Gid: -1})
	...
	t.Cleanup(func() { killServer(ln, sess) }) // 收口纪律：泄漏 seq 洪水子进程会拖垮后续测试（e2e_test.go:120-134 注释实测教训）
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Add(1)
		defer wg.Done()
		h.ServeHTTP(w, r)
	}))
	return exitCh, "ws://" + ln.Addr().String() + "/ws", wg.Wait
}
```

**stall 夹具纪律**（slowclient_test.go:7-13 头注释 + readUntilError:58-75）：
```go
// stall 夹具纪律：dialHello 成功后不再调用 Read——TCP 接收缓冲填满 → 服务端 writer 阻塞
// → outbox 涨满。loopback 单连接最坏吸收量 ≈ wmem 4MiB + rmem 6MiB（本机实测上限），
// 故输出洪水必须远超该量级才能让 outbox 写满...
// 客户端 Read 永不带 deadline ctx（Pitfall 2）——一律 goroutine + 缓冲 channel +
// select time.After 竞速形态。
func readUntilError(c *websocket.Conn) <-chan readResult {
	ch := make(chan readResult, 1)
	go func() {
		var r readResult
		for {
			_, data, err := c.Read(context.Background())
			if err != nil { r.err = err; ch <- r; return }
			if len(data) > 0 && data[0] == proto.Output { r.acc = append(r.acc, data[1:]...) }
		}
	}()
	return ch
}
```
洪水生成器平台分支先例：`seqFlood()`（slowclient_test.go:44-49——darwin 999999 / Linux 4000000）；load 测试为 Linux 手动跑（darwin 分支 skip）。

**/metrics 黑盒 scrape 形态**（metrics_test.go:62-80）：
```go
func getMetrics(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	...
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; version=0.0.4; charset=utf-8" { ... }
	return string(b)
}
```
负载断言数据源 8 条已就位（metrics_test.go:40-58 契约清单）：`wesh_clients_kicked_total` / `wesh_outbox_depth_bytes_max|sum` / `wesh_input_rate_dropped_total` / `wesh_input_queue_dropped_total` / `wesh_credit_gate_transitions_total` / `wesh_goroutines` / `wesh_mem_alloc_bytes`。进程内断言补 `runtime.NumGoroutine`/`runtime.ReadMemStats` + `/proc/self/fd` 计数 + `/proc/<child>/stat` Z 态（defunct 三面，Linux-only）。

**标定验证对象锚点**（clients.go:32-67 六常量，注释自带一阶依据 + Phase 9 挂账语——D-12 证伪改值时注释同步改写为实测依据；server.go:278 hello 5s / :287 pong 10s / :1345 EXIT 直写 2s / :585 MaxBytesReader 4096；proto.go:76 ReadLimitPostAuth 16KiB）。

---

### 11. `web/uat/phase09.mjs`（新建，协议层 UAT）

**Analog:** `web/uat/phase07.mjs`（S1 配置场景 = --index 启动校验矩阵的直接先例）

**脚本头纪律**（phase07.mjs:1-25——覆盖清单注释 + 红线声明 + 运行方式）：
```js
// Phase 7 协议层自动化 UAT（零依赖，Node >= 22 原生 WebSocket/fetch）。
// 覆盖 07-01..07-06 六 plan 服务端机制对真实二进制的全链断言...
// 红线（phase06.mjs:11-13 纪律逐字沿用）：share token/凭据值只作断言材料，永不进入
// check detail 或任何控制台输出——detail 只打印状态码/布尔/形状/退出码/文案常量...
// 运行：node web/uat/phase07.mjs [wesh 二进制路径]   （默认 /tmp/wesh-uat/wesh）
```

**check/skip/红线三件套**（phase07.mjs:62-106 逐字复用形态）：
```js
const results = [];
const emittedDetails = []; // assertOutputClean 遍历材料（运行时自净断言）
const check = (id, name, ok, detail = '') => {
  results.push({ id, name, ok });
  emittedDetails.push(String(detail));
  console.log(`  ${ok ? 'PASS' : 'FAIL'}  ${id} ${name}${detail ? ` — ${detail}` : ''}`);
};
const skip = (id, name, reason) => { ... }; // 平台豁免：不计失败
const sensitiveTokens = []; // token/探针值闭包数组——只作 assertOutputClean 材料
```

**startWesh spawn 夹具**（phase07.mjs:108-111 注释起的形态）：`--bind 127.0.0.1 --port 0` 前置（loopback 随机端口零干扰）；`opts.defaultListen:false` 时 argv 原样（--index 校验矩阵场景 = 期望 exit 2 不启动，S1 不存在文件 exit 2 断言同通道）；`redactArgs` 脱敏超时 reject 消息。phase09.mjs 断言面（RESEARCH Test Map）：--index 四拒绝 exit 2 + 错误行零内容探针、双通道（/ 与 /s/{token}/）统一给页、gzip/Vary、内建相对资源 404、/api/attach 与 /ws 照旧。

---

### 12. `web/uat/pw/phase09-caddy-pw.mjs`（新建，Windows 侧 Playwright）

**Analog:** `web/uat/pw/phase07-a2-pw.mjs`（D-15 Caddy 双机全链 = G-07-2 nginx 套路复用——把 nginx 换成 Caddy）

**双机控制拓扑**（phase07-a2-pw.mjs:24-35 + phase07-a2-ctl.sh 全文）：
```js
const SSH = '9.134.229.124';
const BASE = 'http://9.134.229.124:10013'; // LAN 直连（安全组已放通，连通性实证）
const ssh = (cmd) => execSync(`ssh -o BatchMode=yes ${SSH} ${JSON.stringify(cmd)}`, { encoding: 'utf8' }).trim();
const ctl = (args) => ssh(`bash /tmp/wesh-uat/a2-ctl.sh ${args}`);
async function setup() {
  execSync(`scp -o BatchMode=yes "${CTL_LOCAL}" ${SSH}:/tmp/wesh-uat/a2-ctl.sh`, { stdio: 'pipe' });
  const out = ctl('setup');
  if (!out.includes('NGINX_UP')) throw new Error(`Linux 侧 setup 失败: ${out}`);
}
```
配套 `phase09-caddy-ctl.sh`（同目录新建）：scp 上传 → setup/variant/teardown case 分派 → `fuser -k` 预清理 → nohup 起 wesh → 探活（ss -ltn grep）→ 就绪标志串回显（`CADDY_UP` 形态）；Caddy 官方二进制直装（禁 apt 纪律不涉服务端软件）。

**断言形态**（phase07-a2-pw.mjs:50-77——request 层状态码/Location 断言 + 浏览器层 echo 全链 + idle 存活 + panel hidden）：
```js
  const respBare = await ctx.request.get(`${BASE}/wesh`, { maxRedirects: 0, headers: { Authorization: AUTH_HEADER } });
  t1.ok(respBare.status() === 308, 'status 308', `got=${respBare.status()}`);
  ...
  await runCmd(page, `echo ${MK1}:$((6*7))`);
  await waitTermText(page, new RegExp(`${MK1}:42`), 10000);
  ...
  await sleep(IDLE_MS); // >60s 空闲窗（ping 5s < 反代 idle timeout 验证）
  const p = await panel(page);
  t4.ok(p.hidden, '空闲期间无断连状态面板', `hidden=${p.hidden}`);
```
红线同口径：凭据值只作构造材料，永不进 detail/控制台（phase07-a2-pw.mjs:8 头注释）。Caddy 断言面差异（Pitfall 6）：Host 默认透传（Origin 校验天然通过——不配 nginx 的 Host 行）；WS upgrade reverse_proxy 内建自动处理。

---

### 13. `web/src/main.ts:881-905`（改：D-18 ①③）

**Analog:** 自身——文案常量单写口 + onclose 分派 switch

**R1 hint 条件化（①）**：新常量落既有常量族旁（main.ts:426-433——单写口纪律注释形态先例）：
```typescript
// 05-UI-SPEC §Copywriting 同源文案常量（R1/R3 修订落点）：
const UNREACHABLE_BODY = '...';
// C-6 共用提示行前缀（1008/1009/1011 三条单写口防漂移，R3）...
const HINT_RESTART = 'If the problem persists, restart wesh from your shell, then';
```
09-UI-SPEC C-10 定稿：`const HINT_SHUTDOWN = 'If wesh is not restarted for you, start it again from your shell, then';`（同位追加）；case 1001（main.ts:900-905）的 hintPrefix 由 `'Start wesh again from your shell, then'` 改为 `HINT_SHUTDOWN`（systemd Restart=always 自重启形态下旧文案为无效指引）。

**R3 pre-onopen 1001 分派（③）**：现状 `if (!opened)` 分支（main.ts:881-884）在 switch 之前截流——pre-onopen 到达的 1001 被误述为 'Unable to connect'：
```typescript
    if (!opened) {
      showStatus('Unable to connect', UNREACHABLE_BODY, 'Check the shell where wesh is running, then');
      return;
    }
    switch (ev.code) {
      ...
      case 1001: // D-23 优雅下线...
        showStatus('Server shutting down',
          'The wesh server is shutting down. The session has ended.', ...);
        break;
```
修复 = `!opened` 分支**之前**先分派 `ev.code === 1001` → 与稳态 case 1001 **完全同一 showStatus 调用形态**（单写口纪律——文案零复制第二份）。

---

### 14. `web/index.html:63`（改：D-18 ②）

**Analog:** 自身——单属性追加，零视觉影响

```html
    <!-- 页面共三个顶层元素：#terminal（始终存在）、#status（默认 hidden）、#resize-overlay（默认 hidden，FE-06） -->
    <div id="terminal"></div>
    <div id="status" hidden>
```
改为 `<div id="status" role="alert" hidden>`——面板族 aria-live 语义（真实 AT 播报属平台豁免面，skipped+reason 记录）。dist 重建随 pnpm build 链（P1 D-18 既定）。

---

### 15. `web/uat/phase06-dom.mjs`（扩：D-18 断言）

**Analog:** 自身——jsdom 夹具与 check/skip 形态

**jsdom + SpyWebSocket 夹具**（phase06-dom.mjs:19-32 头注释 + synthClose 形态）：
```js
//   - SpyWebSocket.synthClose(code)：合成 CloseEvent 驱动 onclose 分派...
//     先取存 this.onclose 处理器并置 null（抑制随后真实 close 的 1000
//     事件混入断言面），再 try{ this.close() }catch{}...最后以该处理器调用合成事件
```
R3 断言场景 = loadTerminal 后 `synthClose(1001)` pre-onopen 驱动，断言面板 title='Server shutting down'（逐字）；R2 断言 = `#status` 元素 `role` 属性 jsdom 读取断言；R1 断言 = 面板 hint 文案逐字含 HINT_SHUTDOWN 前缀。每场景独立 spawn + 独立 jsdom 隔离纪律（phase06-dom.mjs:34）保持。

---

### 16. `README.md`（改：四节）

**Analog:** 自身——配方文档 + 标定表形态

**反代配方节形态**（README:301-329 nginx 先例——Caddy/CF 新节同构：代码块 + 超时关系段落）：
```markdown
```nginx
    location /wesh/ {
        proxy_pass http://127.0.0.1:7681;
        proxy_http_version 1.1;
        # Host 必须原样转发：nginx 默认转发 $proxy_host（127.0.0.1:后端口），与浏览器 Origin 不同源会被 wesh WS 同源校验 403；$http_host（已全链实证）
        proxy_set_header Host $http_host;
        ...
    }
```

**`proxy_read_timeout` 必须大于 `--ping-interval`（默认 `5s`）**：反代空闲超时看应用层流量——...
```
Caddy 节按实证写（D-15 本机实证兜底，标注实证日期/版本）；CF 节标注「未实测」（D-15 诚实分级——文档即被测物的唯一例外）；两节均含 idle timeout × ping 5s 关系行。

**标定表更新**（README:231-243 现状——D-13 落点）：表头「默认参数与 Phase 9 标定」去挂账语改名；初值列 → 标定值/验证结论；方法论段（:243）补实测数据摘要；「下列初值为一阶推算的合理值（非负载实测），Phase 9 负载标定后回填」句改写为已验证表述。

**发布节（新增）**：goreleaser 产物命名族（`wesh_v1.0.0_linux_amd64.tar.gz` 含 wesh+LICENSE+README.md）+ checksums.txt 验证指引（`sha256sum -c`）+ 发布脚本引用（scripts/release.sh「发布之前跑一次」）；**--index 节（新增）**：整页替换语义 + 「自定义页需自行实现终端逻辑，否则分享链接失去终端功能」（D-06 承诺语）+ 16MiB 默认上限 + `index-max-size` 纯配置键例外写明（D-08 防例外蔓延）；**Dockerfile 节（新增）**：「本镜像不含任何可执行命令——`--` 后命令须来自 bind-mount 或 FROM 派生自建」（Pitfall 7 承诺语）+ `--socket` 容器内需配合 volume。

---

## Shared Patterns

### 启动面红线：错误行零值内容
**Source:** `cmd/wesh/config.go:23-27`（文件头红线声明）+ `config.go:74-78`（configErr 单写口）+ `main.go:361-374`（clientOptErr 记录式先例）
**Apply to:** --index 校验、index-max-size 解析、FuzzDecodeFileConfig、全部新测试
```go
// config.go:23-27——go-toml 的 DecodeError.String() 带源行上下文会回显配置值...
// 本文件绝不透传 go-toml 错误文本，只提取 DecodeError.Key()/Position()（键名 + 行号）
// 组 detail，全部错误经 configErr 统一包装为「类别 + 键名/行号」三要素。
// 测试以凭据值探针串运行时自证零出现。
func configErr(path, category, detail string) error {
	return fmt.Errorf("invalid config file %s: %s (%s)", path, category, detail)
}
```
--index 延伸形态：错误行只含路径 + 类别（不存在/不可读/非常规/超限），HTML 内容字节永不入错（Pitfall 9）；测试以内容探针串反断言。

### UAT 红线三件套（check/skip/assertOutputClean）
**Source:** `web/uat/phase07.mjs:62-106`（+ phase06-dom.mjs:54-67 同形态）
**Apply to:** phase09.mjs、phase09-caddy-pw.mjs、phase06-dom.mjs 扩展
token/凭据/探针值只作断言材料，永不进 detail/控制台/汇总行；`sensitiveTokens` 闭包数组 + 尾部 `assertOutputClean` 运行时自净；平台豁免 `skip(id, name, reason)` 不计失败。

### 零值兜底纪律（nil/零值 = 与现状逐字节一致）
**Source:** `internal/server/sharetoken.go:34-41`（newShareTokens 空串=nil 通道关闭）+ `server.go:446`（bp=="" 零漂移兜底）+ `web/embed.go` 装配
**Apply to:** `Options.CustomIndex []byte`——nil 时伺服行为与现状逐字节一致；装饰层只在非 nil 时包裹。

### 两阶段合并 + 指针标量（配置键接入机制）
**Source:** `cmd/wesh/config.go:19-21`（合并前提注释）+ `main.go:200-304`（fc → flag 默认值铺底）
**Apply to:** index / index-max-size 两键——指针类型区分「键缺席」与「显式零值」；`fc.X != nil` 即采用 `*fc.X`。

### 文档即被测物（实证分级）
**Source:** README nginx 配方（G-07-2 双机实证先例）+ CONTEXT D-15/D-16/D-17
**Apply to:** Caddy 配方（本机+双机全链实证）、Dockerfile（本机 docker build 实测）、systemd unit（实机 systemctl 通道）；CF 唯一例外标注「未实测」。文档承诺语（「本镜像不含任何可执行命令」等）必须与实测行为同源。

### 测试夹具收口纪律
**Source:** `internal/server/e2e_test.go:120-134`（killServer + t.Cleanup——泄漏 seq 洪水子进程拖垮后续测试的实测教训注释）
**Apply to:** load_test.go、customindex_test.go——每个装配函数必须 `t.Cleanup(killServer)`；load 测试矩阵每格独立收口防跨格污染。

### 常量注释一阶依据 + 标定挂账语
**Source:** `internal/server/clients.go:32-67`
**Apply to:** D-12 证伪改值时——默认值注释从「一阶依据 + Phase 9 标定回填」同步改写为实测依据（数据出处 + 日期），保持「注释自带依据」传统。

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `.goreleaser.yml` | build-config | batch | 仓内首件发布配置（ttyd 用 Makefile 发布不可参照）——RESEARCH §Code Examples 已给 v2.18.0 schema 核实定稿（Pitfall 1/2/3 三钉：`main: ./cmd/wesh` / `.Tag` 保 v 前缀 / `checksum.name_template: checksums.txt`），planner 直接采用 |
| `Dockerfile` | container-config | batch | 仓内首件容器构建件——RESEARCH §Code Examples 定稿（scratch + tini-static sha256 钉死 + `ADD --checksum` + `ENTRYPOINT ["/tini", "--", "/wesh"]`），planner 直接采用 |

---

## Metadata

**Analog search scope:** `cmd/wesh/`、`internal/{server,proto}/`、`web/`（embed.go、src/main.ts、index.html、uat/、uat/pw/）、`.github/workflows/`、`README.md`
**Files scanned:** 22（全文读 9 + 定向段读 13，全部单遍无重读）
**Pattern extraction date:** 2026-08-29
