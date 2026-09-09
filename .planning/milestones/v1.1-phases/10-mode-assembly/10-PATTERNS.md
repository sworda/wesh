# Phase 10: 模式装配与接缝 - Pattern Map

**Mapped:** 2026-09-02
**Files analyzed:** 12（10 改 + 2 文档；无全新文件——本阶段全部是在既有文件上扩点）
**Analogs found:** 12 / 12（1 处形态缺口见「No Analog Found」：New 互斥校验 fail-fast 形态）

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/wesh/main.go`（parseArgs 段） | CLI entry / config 装配 | request-response（启动配置管线） | 同文件 write-policy 先例（:334/:504-507/:618-620） | exact（同文件） |
| `cmd/wesh/main.go`（validateStartup 段） | 启动校验纯函数 | request-response（fail-fast/warn 双通道） | 同文件 write-policy 组合校验行（:936-938）+ --cwd stat 预检行（:964-968）+ D-16 warn 行（:1046-1049） | exact（同文件） |
| `cmd/wesh/main.go`（run() 段） | 装配分岔点 | event-driven（进程/监听生命周期） | 同文件 pty.Start 直传现状行（:1210）+ Options 装配行（:1242） | exact（同文件） |
| `cmd/wesh/config.go` | config loader | file-I/O（TOML 严格解码） | 同文件 WritePolicy 键（:51）+ configErr 单写口（:86-88） | exact（同文件） |
| `internal/pty/spawn.go` | spawn service | 进程 spawn（exec 数组） | 同文件 Start（:64-95）+ SpawnCols/SpawnRows（:38-41） | exact（同文件） |
| `internal/pty/spawn_test.go` | test | 进程 spawn 断言 | 同文件 TestStartZeroValueParity（:224-242） | exact（同文件） |
| `internal/server/server.go`（Options/New） | server core | event-driven（WS 生命周期） | 同文件 Options.WritePolicy（:236）+ New 零值兜底（:338-340） | exact（同文件） |
| `internal/server/clients.go`（常量块） | 枚举常量宿主 | —（纯常量） | 同文件 WritePolicyOwner/WritePolicyAll（:74-77） | exact（同文件） |
| `internal/server/export_test.go` | test helper | —（测试期出口） | 同文件 LockStderr/ShrinkOutboxForTest（:10-39） | role-match |
| `cmd/wesh/fuzz_test.go` + `testdata/fuzz/` | fuzz test | file-I/O transform（bytes-in） | 同文件 FuzzDecodeFileConfig 五种子（:75-92） | exact（同文件） |
| `cmd/wesh/main_test.go` | test | 启动配置断言 | 同文件 TestParseArgs/TestTLSKeyPairError/TestStartupMatrix 行先例 | exact（同文件） |
| `cmd/wesh/config_test.go` | test | 配置合并断言 | 同文件 TestConfigMerge/Precedence/RedLines + parseConfigArgs 夹具（:404-411） | exact（同文件） |
| `docs/CONFIGURATION.md` + `README.md` | docs | — | CONFIGURATION.md write-policy 三处表行（:56/:100/:118-124） | exact（同文件） |

---

## Pattern Assignments

### 1. `cmd/wesh/main.go` parseArgs 段（controller 位：flag 注册 → 合并 → 校验）

**Analog:** 同文件 write-policy 全链先例（D-04 直接同形态——session-mode 是第二个枚举 string flag）

**(a) config struct 字段 + 显式设置位**（main.go:41-43，照此形态加 sessionMode/sessionModeSet 两字段）:
```go
// Phase 5 写权限体系（D-05，one-way 公开契约，P2 D-15 同纪律）：
writePolicy    string // --write-policy=owner|all（默认 owner；仅在 --writable 总闸开启时有意义）
writePolicySet bool   // --write-policy 是否被显式设置（parseArgs 经 fs.Visit 填充，validateStartup 组合校验消费）
```

**(b) 注册默认值铺底 + fc 标量换算**（main.go:218, 257-259——flag > 配置 > 内置默认的承载机制）:
```go
writePolicyDefault := server.WritePolicyOwner
// ...
if fc.WritePolicy != nil {
    writePolicyDefault = *fc.WritePolicy
}
```
→ session-mode 同款：`sessionModeDefault := server.SessionModeShared`（常量见 §7），`fc.SessionMode != nil` 时换算。

**(c) flag 注册（全名无短选项 + 注释纪律）**（main.go:330-334）:
```go
// D-05：写权限策略（one-way 公开契约）。--writable 保持总闸（不给 = 全员只读，
// 现状语义零漂移）；write-policy 仅在总闸开启时有意义（组合校验见
// validateStartup）。parse 期枚举校验在 Parse 返回处（值非敏感，直接 return
// error 即可——client-option 的记录式上报仅用于值含敏感内容的场景）。
fs.StringVar(&cfg.writePolicy, "write-policy", writePolicyDefault, "write policy when --writable is on: owner|all (default owner)")
```
→ `--session-mode` help 文案随注册同 PR（D-05 文档面）；parseArgs 头部大注释（main.go:140-180 的 flag 清单段）补 Phase 10 行。

**(d) fs.Visit 显式位**（main.go:504-507）:
```go
fs.Visit(func(f *flag.Flag) {
    if f.Name == "write-policy" {
        cfg.writePolicySet = true
    }
    // ...
})
```

**(e) 配置键存在即置位（07-06 合并收尾第一档，D-02 机制依据）**（main.go:537-553）:
```go
if fc != nil {
    // ...
    if fc.WritePolicy != nil {
        cfg.writePolicySet = true
    }
}
```
→ `fc.SessionMode != nil → cfg.sessionModeSet = true`——CLI 与 TOML 双源同档触发 warn（D-02）。

**(f) Parse 返回处枚举校验（D-04 文案形态：枚举值回显豁免）**（main.go:615-620）:
```go
// D-05：--write-policy parse 期枚举校验（插入点同 03-04 先例——showVersion
// 早退之后）。枚举值非敏感，直接 return error（不经 clientOptErr 记录式——
// 该形态仅用于值含敏感内容的 --client-option）。
if cfg.writePolicy != server.WritePolicyOwner && cfg.writePolicy != server.WritePolicyAll {
    return cfg, nil, fmt.Errorf("invalid --write-policy %q: must be owner or all", cfg.writePolicy)
}
```
→ D-04 定案文案：`invalid --session-mode "banana": must be shared or per-client`。插入点 = showVersion 早退之后、write-policy 校验同位。TOML 源非法值经默认值替换机制落 cfg.sessionMode 同一终值，**一闸双覆盖**（pingInterval 负值闸注释 :655-658 明示此机制）——TOML 双源拒绝无需第二条校验行，测试面在 config_test.go。

---

### 2. `cmd/wesh/main.go` validateStartup 段（warn 行 + LookPath 预检行）

**Analog A:** write-policy 组合校验行（main.go:930-938——warn/fail-fast 落点纪律：纯配置矛盾在 loopback 早退**之前**判定）:
```go
// validateStartup 是 D-03/D-05 启动校验矩阵……的纯函数落地——无副作用：禁止
// listen/spawn/写文件；os.Stat 只读探测允许（07-04 D-21 --cwd 预检），必须先于
// pty.Start/net.Listen 执行：拒绝路径零资源占用……
// 返回 warn（放行但需 stderr 醒目警告的逃生门/明文场景）或 err（拒绝启动）。
// 红线：warn/err 文案不得含凭据值（SEC-01 日志红线延伸到启动面）。
func validateStartup(cfg config) (warn string, err error) {
	if cfg.writePolicySet && !cfg.writable {
		return "", errors.New("--write-policy is set but --writable is not; write policy only applies when client input is enabled")
	}
```

**Analog B:** D-16 warn 行形态（main.go:1046-1049——`wesh: warning:` 前缀 + flag 名进文案 + 不含值；D-01 warn 行直接同形态）:
```go
	if cfg.authHeader != "" {
		return "wesh: warning: listening on non-loopback address with NO authentication (--no-auth) and --auth-header enabled; ...", nil
	}
```
→ D-01/D-02 warn 行：锚定 `cfg.writePolicySet && cfg.sessionMode == per-client`（owner|all 任一显式均触发，warn 放行不拒绝；双 flag 名进文案纪律）。**落点注意**：write-policy 组合语义与 bind 安全矩阵无关，应在 loopback 早退（:1033）之前判定——warn 返回值通道现成，但当前实现单 warn 返回（多处 `return "...", nil` 互斥分支）；新增 warn 行需选型：与既有 warn 并存时的合并/优先级（Discretion——既有 warn 均挂在非 loopback 分支之后，本行在 loopback 也可达，结构上位于早退前独立 return 点）。

**Analog C:** --cwd stat 只读预检行（main.go:958-968——SC4 LookPath 预检的落点与纪律先例）:
```go
// D-21 预检（……纯配置有效性与 bind 安全形态无关，loopback 早退之前判定）：
// --cwd 非空时 os.Stat 只读探测（纯函数纪律允许只读探测，见函数头注释）；
// 不存在或非目录即拒（spawn 前零资源占用——spawn 后才发现 ENOENT 则资源
// 已占用且错误面到客户端，RESEARCH Anti-Patterns）。值非敏感，错误文案可回显路径……
if cfg.cwd != "" {
	if fi, serr := os.Stat(cfg.cwd); serr != nil || !fi.IsDir() {
		return "", fmt.Errorf("invalid --cwd %q: not an existing directory", cfg.cwd)
	}
}
```
→ per-client LookPath(argv[0]) 预检行同位（loopback 早退前、只读探测纪律内）；validateStartup 当前签名只收 cfg——argv[0] 的传入需扩签名或入 cfg（Discretion；`os/exec` 已在 main.go import 列表 :18，openBrowser 同款使用）。

---

### 3. `cmd/wesh/main.go` run() 段（装配期一次分岔点）

**Analog:** pty.Start 直传现状行（main.go:1208-1214）+ Options 装配行（main.go:1242）:
```go
// D-21/D-24 接线（07-04）：--cwd/--term 落 StartOptions Dir/Term；--uid/--gid
// 落 Uid/Gid（-1 哨兵 = 不降权，Task 3 完成 flag 注册与成对校验）。
sess, err := pty.Start(argv, pty.StartOptions{Dir: cfg.cwd, Term: cfg.term, Uid: cfg.uid, Gid: cfg.gid})
if err != nil {
	fmt.Fprintf(os.Stderr, "wesh: %v\n", err)
	return 1
}
```
```go
srv := server.New(sess, os.Exit, server.Options{Writable: cfg.writable, WritePolicy: cfg.writePolicy, ..., CustomIndex: customIndex})
```
→ 分岔形态（研究 §11，CONTEXT Discretion）：shared = 现状行逐字保持（sess 非 nil 直传 New）；per-client = `sess = nil` + `Options.SpawnFunc` 装配 inert 闭包（捕获 argv + StartOptions）+ `Options.SessionMode` 透传。**零回归纪律**：shared 分支不得改动任何既有字节序（Options 字面量追加两键即可）。New 互斥校验（SessionMode=per-client × SpawnFunc=nil 拒绝；SpawnFunc≠nil × SessionMode=shared 拒绝）在 server.go 落地，run() 只负责两态正确装配。

---

### 4. `cmd/wesh/config.go`（+session-mode 第 30 键）

**Analog:** WritePolicy 键（config.go:51）+ 文件头覆盖面注释（:9-14）+ configErr 单写口（:84-88）:
```go
type fileConfig struct {
	// ...
	WritePolicy   *string  `toml:"write-policy"`
	// ...
}
```
```go
// configErr 统一包装配置文件错误：`invalid config file <path>: <category>
// (<detail>)`——detail 只含键名/行号/安全类别文案，禁含配置值（文件头红线）。
func configErr(path, category, detail string) error {
	return fmt.Errorf("invalid config file %s: %s (%s)", path, category, detail)
}
```
→ 加 `SessionMode *string `toml:"session-mode"``（全连字符命名，P7 D-03——ROADMAP/REQUIREMENTS 的 `session_mode` 下划线写法按 CONTEXT 修正记录统一为 `session-mode`，DisallowUnknownFields 下划线形态会被当未知键拒绝）；文件头「29 键」注释改 30；键枚举值非法时**不经 configErr**——非法枚举由 parseArgs 统一枚举校验闸拒绝（§1f），config.go 只负责类型解码（string 类型 go-toml 自然接受，非 string 类型由 decodeFileConfig 类型不符分支拒绝）。

---

### 5. `internal/pty/spawn.go`（+StartWithSize 导出，Start 委托）

**Analog:** Start 本体（spawn.go:61-95）+ SpawnCols/SpawnRows 单一事实源（:34-41）:
```go
// SpawnCols/SpawnRows 为 PTY spawn 初始尺寸的单一事实源（G-05-1 导出，05-10）：
// StartWithSize 的 Winsize 字面量与服务端零参与者会话尺寸回落值（server 包
// sessionDimsLocked）必须同源——两处各写魔法数会在调整时双写漂移……
const (
	SpawnCols = 80
	SpawnRows = 24
)
```
```go
func Start(argv []string, opts StartOptions) (*Session, error) {
	if len(argv) == 0 {
		return nil, errors.New("pty: empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...)   // exec 数组，绝不经 shell
	// ...（env 白名单 / Dir / Credential 分支保持唯一副本）
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: SpawnRows, Cols: SpawnCols})
	// ...
	return &Session{Cmd: cmd, Master: master, readDone: make(chan struct{})}, nil
}
```
→ 导出形态（Discretion：尺寸参数位置/类型）：`StartWithSize(argv []string, opts StartOptions, cols, rows uint16)` 承载全体现状逻辑，`Start` 缩为一行委托 `return StartWithSize(argv, opts, SpawnCols, SpawnRows)`——**80×24 字面量不得复制第二份**（注释 :35-37 已预言本纪律）；server 侧消费先例 = `resize.go:196` `return dims{cols: pty.SpawnCols, rows: pty.SpawnRows}`。

---

### 6. `internal/pty/spawn_test.go`（StartWithSize 委托等价测试）

**Analog:** TestStartZeroValueParity（spawn_test.go:220-242）:
```go
// 全部四 flag 时 pty.Start 行为与现状逐字节一致——Dir 空 = 继承（cmd.Dir 零值
// 不设）、Term 空 = xterm-256color（cmd.Env 与 whitelistEnv("", -1) 逐项相等）……
func TestStartZeroValueParity(t *testing.T) {
	sess, err := Start([]string{"/usr/bin/env"}, StartOptions{Uid: -1, Gid: -1})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if sess.Cmd.Dir != "" {
		t.Fatalf("零值 opts cmd.Dir = %q, want 空串（继承服务端 cwd 的零值语义）", sess.Cmd.Dir)
	}
	// ...
	_, werr := awaitSession(t, sess, startCollect(sess))
```
→ 委托等价断言面：Start ≡ StartWithSize(argv, opts, SpawnCols, SpawnRows)（cmd.Env/Dir/SysProcAttr 逐字段相等 + 首 winsize 同源）；复用 `awaitSession`/`startCollect` 既有夹具（同文件）。

---

### 7. `internal/server/server.go`（Options +SessionMode/SpawnFunc 两键 + New 校验）

**Analog A:** Options 生产直传字段先例（server.go:234-287 注释分档 + WritePolicy 字段 :236）:
```go
// 05-03 新增：WritePolicy 为生产直传字段（main --write-policy flag 原样透传，
// D-05；零值兜底 WritePolicyOwner——安全默认；取值常量见 clients.go）。
```
```go
type Options struct {
	Writable           bool
	WritePolicy        string
	// ...
}
```
→ SessionMode 同款注释分档（生产直传 + 零值 = shared 兜底——零值等价纪律：v1.0 逐字节不变的结构性保证）；SpawnFunc 注释登记签名形态与「inert 装配期挂点」语义（Discretion：建议 `func() (*pty.Session, error)` 闭包捕获 argv+StartOptions 形态）。

**Analog B:** New 零值兜底块（server.go:313-355）:
```go
func New(sess *pty.Session, exitf func(int), opts Options) *Server {
	// ...
	if opts.WritePolicy == "" {
		opts.WritePolicy = WritePolicyOwner // D-05 安全默认
	}
	// ...
	if opts.StopSignal == 0 {
		opts.StopSignal = syscall.SIGHUP
	}
```
→ `opts.SessionMode == "" → SessionModeShared` 兜底同位。互斥校验 fail-fast 形态见「No Analog Found」节——**New 现无 error 返回**（签名 `:313` 返回 *Server），这是本 phase 唯一无先例的形态选型点。

---

### 8. `internal/server/clients.go`（session-mode 枚举常量）

**Analog:** WritePolicy 常量块（clients.go:69-77）:
```go
// WritePolicy 取值（D-05 公开 CLI 契约——--write-policy=owner|all，全名无短选项
// P2 D-15；main.go parse 期枚举校验用同一对常量，防双写漂移）：
// owner = 首个以 rw 身份 attach 的客户端为 owner、后续 rw 降级 ro 进 FIFO 递补
// 队列（D-06/D-07，安全默认哲学：旁观是被动场景、协作主动开启）；
// all = 全员可写（服务协作排障形态，无递补概念）。
const (
	WritePolicyOwner = "owner" // 默认（D-05 安全默认）
	WritePolicyAll   = "all"
)
```
→ `SessionModeShared = "shared"` / `SessionModePerClient = "per-client"` 同形态（main.go parse 期枚举校验与 server Options 消费共用同一对常量，防双写漂移）；string 类型 vs 自定义枚举类型为 Discretion（WritePolicy 的裸 string 先例可直接对齐）。

---

### 9. `internal/server/export_test.go`（New 互斥校验测试暴露）

**Analog:** 文件头纪律 + ShrinkOutboxForTest（export_test.go:3, 15-39）:
```go
// export_test.go —— 测试期专属出口（仅 -test 编译，零生产 API 面）。
```
```go
func (s *Server) ShrinkOutboxForTest(seq int64, newCap int) bool {
	s.hubMu.Lock()
	defer s.hubMu.Unlock()
	// ...调用方不得持 hubMu；返回 false = 该 seq 不在注册表。
}
```
→ 若互斥校验结果需要白盒断言（如 New 内部拒绝标记/校验函数），经本文件暴露测试期出口；若 fail-fast 选型为可导出的包级校验函数（如 `validateOptions(opts) error`），测试直接调该函数则无需新出口。

---

### 10. `cmd/wesh/fuzz_test.go` + `testdata/fuzz/`（session-mode 语料扩展）

**Analog:** FuzzDecodeFileConfig 五种子（fuzz_test.go:75-92）+ 崩溃语料入库文件（testdata/fuzz/FuzzDecodeFileConfig/1556732f37b0e706）:
```go
func FuzzDecodeFileConfig(f *testing.F) {
	f.Add([]byte("port = 7681\nbind = \"127.0.0.1\"\n"))      // 合法键
	f.Add([]byte("credential = [\"FUZZ_PROBE_SECRET:x\"]\n")) // 值剥离红线探针
	f.Add([]byte("unknown-key = 1\n"))                        // 未知键拒绝面（严格模式）
	f.Add([]byte("port = \"not-a-number\"\n"))                // 类型不符面
	f.Add([]byte{0xff, 0xfe, 0x00})                           // 非 UTF-8/二进制
```
```
go test fuzz v1
[]byte("[\"FUZZ_PROBE_SECRET\"]")
```
→ 新种子直接追加 f.Add 行（Pitfall 11 同 PR 纪律）：`session-mode = "shared"`（合法）/ `session-mode = "per-client"`（合法）/ `session-mode = "banana"`（非法枚举——注意：非法枚举值在 decodeFileConfig 层**不报错**（类型是合法 string），拒绝在 parseArgs 枚举闸；fuzz 目标只断言 decodeFileConfig 两不变量，枚举矩阵断言归 config_test.go）/ `session_mode = "shared"`（下划线形态 → 未知键拒绝面，D-03 键名修正的行为锁）/ `session-mode = 1`（类型不符面）。语料文件形态 = `go test fuzz v1` 头 + `[]byte("...")` 行。

---

### 11. `cmd/wesh/main_test.go`（parse/枚举拒绝/启动矩阵三面）

**Analog A:** TestParseArgs 正例行（main_test.go:132-134——session-mode 合法值解析行照此加）:
```go
// D-05：--write-policy 显式传值原样解析（默认值由零值语义统一断言 = owner）。
{name: "write policy all", args: []string{"--writable", "--write-policy", "all", "--", "bash"}, wantBind: "0.0.0.0", wantPort: 7681, wantWritable: true, wantPingInterval: 5 * time.Second, wantWritePolicy: "all", wantArgv: []string{"bash"}},
```

**Analog B:** TestTLSKeyPairError 枚举拒绝行（main_test.go:415 + stop-signal 行 :442-443——D-04 文案断言落点）:
```go
{"malformed write-policy", []string{"--write-policy", "sometimes", "--", "bash"}, "must be owner or all", ""},
// ...
{"stop-signal lowercase rejected", []string{"--stop-signal", "term", "--", "bash"}, "invalid --stop-signal", ""},
```
→ 新增行断言 `invalid --session-mode` + `"banana"` 回显子串（D-04 回显口径的行为锁——双 flag 名/枚举名单进文案）。

**Analog C:** TestStartupMatrix 表驱动行（main_test.go:674-680 表结构 + :698 组合校验行 + :729 warn 行）:
```go
tests := []struct {
	name        string
	cfg         config
	wantErrSub  string // 非空 = 拒绝启动，文案须含此子串
	wantErrSub2 string // 非空 = 拒绝文案须同时含此第二子串（组合校验双 flag 名断言）
	wantWarnSub string // 非空 = 放行但 stderr 醒目警告须含此子串（逃生门 flag 名）
}{
```
```go
{"explicit write-policy without writable refused", config{bind: "127.0.0.1", maxClients: 32, indexMaxSize: 16 << 20, writePolicy: "all", writePolicySet: true}, "--write-policy", "--writable", ""},
// ...
{"auth-header non-loopback no creds warns (D-16)", config{bind: "0.0.0.0", ..., noAuth: true, authHeader: "X-Remote-User"}, "", "", "--auth-header"},
```
→ warn 触发双源两形态行（D-02：CLI 显式位 × per-client / 配置来源置位 × per-client——配置源置位行归 config_test.go）；LookPath 预检行（per-client × argv[0] 不存在 → fail-fast 拒绝文案子串；per-client × 合法命令 → 放行）。**注意 config 字面量新字段零值语义**：既有行不显式给 sessionMode 即 shared 现状——零值等价由表结构天然锁定，禁止给既有行补 sessionMode 字段（期望值逐字未动纪律）。

---

### 12. `cmd/wesh/config_test.go`（合并/优先级链/红线三面）

**Analog A:** parseConfigArgs 夹具（config_test.go:404-411——全部配置源测试的统一入口）:
```go
func parseConfigArgs(t *testing.T, tomlContent string, extra []string, trailing ...string) (config, []string, error) {
	t.Helper()
	path := writeToml(t, tomlContent, 0o600)
	args := []string{"--config", path}
	args = append(args, extra...)
	args = append(args, trailing...)
	return parseArgs(args)
}
```

**Analog B:** TestConfigMerge 标量子测（config_test.go:431-442）:
```go
t.Run("scalar from config only", func(t *testing.T) {
	cfg, _, err := parseConfigArgs(t, "port = 9999\nbind = \"127.0.0.1\"\n", nil, "--", "bash")
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if cfg.port != 9999 {
		t.Errorf("port = %d, want 9999 (config 铺底)", cfg.port)
	}
})
```

**Analog C:** TestConfigPrecedence 优先级链子测（config_test.go:811-819——D-03 裁决下 session-mode 链只断言三层：flag > TOML > 默认，无 env 键）:
```go
t.Run("flag over config", func(t *testing.T) {
	cfg, _, err := parseConfigArgs(t, "port = 9999\nwrite-policy = \"all\"\nwritable = true\n", []string{"--port", "1234", "--write-policy", "owner"}, "--", "bash")
	// ...
	if cfg.port != 1234 || cfg.writePolicy != "owner" {
		t.Errorf("port/writePolicy = %d/%q, want 1234/owner (flag 最高优先)", cfg.port, cfg.writePolicy)
	}
})
```

**Analog D:** TestConfigRedLines 值剥离探针口径（config_test.go:865-869 + fuzz_test.go:25-42 stripKeyNameEcho）——TOML 源 `session-mode = "banana"` 拒绝文案经枚举闸（main.go 单写口），**不经 configErr**；config_test 侧断言面 = 非法枚举值经 parseConfigArgs 上抛的错误串形态与 CLI 同文案（一闸双覆盖锁）+ `session_mode` 下划线键按未知键拒绝（D-03 修正的行为锁，文案 `unknown keys (session_mode)` 合法含键名）。

---

### 13. `docs/CONFIGURATION.md` + `README.md`（D-05 最小明示）

**Analog:** CONFIGURATION.md 三处表行（TOML 键表 :56 / 显式位说明 :100 / 校验矩阵表 :118-124）:
```markdown
| `write-policy` | 字符串 | `"owner"` | `owner`（首写者独占，断线递补）或 `all`（全员可写）；仅 `writable` 开启时有意义 |
```
```markdown
- **配置键显式位**：`port`/`bind`/`socket-mode`/`socket-owner`/`write-policy` 在配置文件中出现即视为「显式设置」，与 CLI 同档参与互斥/组合校验……
```
```markdown
| 写策略组合 | 显式 `--write-policy` 却未开 `--writable` | --write-policy is set but --writable is not |
| 值域/枚举 | `--write-policy`/`--stop-signal` 枚举、`--socket-mode` 八进制、…… | invalid …（值可回显，非敏感） |
```
```markdown
**放行但警告**（stderr 醒目提示，不阻断启动）：
- `--no-auth` 非 loopback：任何人可达该端口即得终端。
```
→ 三处加行（flag 表行 / TOML 键表行 / 校验矩阵枚举行各一），注记「per-client 行为装配中，当前版本与 shared 等价」；「放行但警告」清单加 write-policy × per-client warn 行（D-01）；显式位说明段的键清单追加 `session-mode`（D-02 机制文档化）；README.md 加一句同旨明示（落点随既有 flag 叙述段，README:93 一带的用法示例区）；`--help` 文案随 flag 注册同 PR（§1c）。措辞精确值 = Discretion。

---

## Shared Patterns

### S1. parse/validate 分层纪律（P2 D-15/P3，全部新校验行必须归类）
**Source:** `cmd/wesh/main.go:923-930`（validateStartup 头注释）+ `:615-618`（枚举校验落点注释）
**Apply to:** --session-mode 全部校验面
- **parse 期**（Parse 返回处，showVersion 早退之后）：形状/枚举/值域校验——session-mode 非法枚举值归此（exit 2，D-04 文案回显 `%q`）。
- **validateStartup**（组合矛盾 + 只读预检，loopback 早退之前）：write-policy 显式位 × per-client warn（D-01/D-02）、per-client LookPath 预检（SC4）。
- warn/fail-fast 双通道：`return warn, nil`（放行+stderr 醒目）vs `return "", errors.New(...)`（exit 2 拒绝）；静默永不接受（ROADMAP 锁定）。

### S2. 显式设置位机制（fs.Visit + fc.X 非 nil 置位，07-06 合并收尾第一档）
**Source:** `cmd/wesh/main.go:504-530` + `:537-553`
**Apply to:** sessionModeSet 字段（D-02 warn 锚定位）
```go
fs.Visit(func(f *flag.Flag) {
	if f.Name == "write-policy" { cfg.writePolicySet = true }
	// → 追加 "session-mode" 分支
})
// ...
if fc != nil {
	if fc.WritePolicy != nil { cfg.writePolicySet = true }
	// → 追加 fc.SessionMode != nil → sessionModeSet = true
}
```
配置来源与 CLI 同档触发——writePolicySet 已存在（main.go:43），D-02 只消费不新建。

### S3. 错误文案红线分档（SEC-01 值剥离 vs D-04 枚举回显豁免）
**Source:** `cmd/wesh/main.go:615-620`（枚举直接 return error）+ `cmd/wesh/config.go:25-29`（值剥离红线头注释）+ `docs/CONFIGURATION.md:124`
**Apply to:** session-mode 全部错误面
- CLI 枚举非法：`fmt.Errorf("invalid --session-mode %q: must be shared or per-client", v)`——值域两固定单词无秘密可泄，回显助定位拼写错误。
- TOML 源：非法枚举值经默认值替换机制落 cfg.sessionMode 同一终值，**同一闸拒绝**（零双写）；go-toml 错误文本绝不透传（configErr 三要素：类别+键名+行号）。
- 凭据/token/文件内容红线保持（credErr/clientOptErr 记录式形态本 phase 不涉及）。

### S4. 零值等价纪律（one-way flag + 默认 shared 逐字节零回归）
**Source:** `internal/server/server.go:338-340`（WritePolicy 零值兜底）+ `internal/pty/spawn_test.go:224`（TestStartZeroValueParity）
**Apply to:** Options.SessionMode / run() 分岔
- SessionMode 零值 = shared（New 零值兜底注释分档：生产直传 + 零值语义说明）；SpawnFunc 零值 = nil。
- run() shared 分支 = 现状行逐字保持（main.go:1210-1242 不改动既有字节序，Options 字面量只追加）。
- 每阶段收口闸：shared 全量测试原样绿 + 期望值逐字未动；**禁止断言放宽成「两模式都接受」**（SUMMARY.md 方法论警告）。

### S5. 单一事实源（80×24 与枚举常量防双写漂移）
**Source:** `internal/pty/spawn.go:34-41` + `internal/server/clients.go:69-77` + `internal/server/resize.go:196`
**Apply to:** StartWithSize / SessionMode 常量
- StartWithSize 的默认尺寸字面量不得复制——Start 委托传 SpawnCols/SpawnRows；服务端回落值继续消费 `pty.SpawnCols/SpawnRows`。
- SessionMode 枚举常量单点声明（server 包，WritePolicy 常量同位），main.go parse 校验与 Options 消费共用。

### S6. 测试宿主归属先例（扩既有表 > 建新文件）
**Source:** `cmd/wesh/main_test.go:44/388/659` + `cmd/wesh/config_test.go:404-421` + `internal/pty/spawn_test.go:224`
**Apply to:** 全部新 Go 测试
- parse 矩阵 → TestParseArgs 表加行 + wantSessionMode 断言位（零值 = 期望 shared 默认，wantStopSignal 零值语义先例 main_test.go:98-104）。
- 枚举拒绝矩阵（CLI）→ TestTLSKeyPairError 错误表加行。
- warn/预检矩阵 → TestStartupMatrix 表加行（wantWarnSub 通道现成）。
- TOML 三面 → TestConfigMerge/TestConfigPrecedence/TestConfigRedLines 各加子测（parseConfigArgs 夹具直用）。
- 委托等价 → spawn_test.go 新 TestXxx（awaitSession/startCollect 夹具直用）。
- server 包 New 校验 → export_test.go 先例决定暴露面（§9）。

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/server/server.go` New 互斥校验 fail-fast 形态 | server core | — | **New 现无 error 返回**（server.go:313 `func New(...) *Server`），包内无构造函数拒绝先例（grep 确认 server 包零 panic）。CONTEXT 已列为 Claude's Discretion：可选形态 = (a) New 签名加 error 返回（动全部调用点：main.go:1242 + server 包全部测试 New 调用——零回归纪律下成本高）；(b) 包级 `ValidateOptions(opts) error` 导出函数由 main 在 New 前调用（run() 启动序 exit 2 通道现成，main.go:1171-1175）；(c) New 内 panic（无先例，违项目纪律）。建议 (b)——与 validateStartup「拒绝路径零资源占用」纪律同构，run() 在 pty.Start 前调用。 |

## Metadata

**Analog search scope:** `cmd/wesh/`（main.go/config.go/fuzz_test.go/main_test.go/config_test.go/testdata/fuzz/）、`internal/pty/`（spawn.go/spawn_test.go）、`internal/server/`（server.go/clients.go/resize.go/export_test.go）、`docs/CONFIGURATION.md`、`README.md`
**Files scanned:** 14（全部精确匹配——本阶段是既有机制扩点，每个改动面的先例均在同一文件内）
**Pattern extraction date:** 2026-09-02
