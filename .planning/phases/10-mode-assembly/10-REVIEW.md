---
phase: 10-mode-assembly
reviewed: 2026-09-03T07:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - README.md
  - cmd/wesh/config.go
  - cmd/wesh/config_test.go
  - cmd/wesh/fuzz_test.go
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - docs/CONFIGURATION.md
  - internal/pty/spawn.go
  - internal/pty/spawn_test.go
  - internal/server/clients.go
  - internal/server/options_test.go
  - internal/server/server.go
findings:
  critical: 1
  warning: 2
  info: 7
  total: 10
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-09-03T07:00:00Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

本轮为对当前树（HEAD 含 10-05 缺口修复提交 189d081/0ec37cb/23f2df2）的独立对抗性复审：12 个列入文件全量通读 + phase diff（f57c701^..HEAD，+566/−33）聚焦 + 进程级实证复现。第一轮 review 的 WR-01（per-client 预检 cwd 错位）/WR-02（ValidateOptions 资源获取后调用）/WR-03（exit-when-empty 陈旧回调）三修复经逐项推演确认正确（WR-03 的 happens-before 链成立：武装方持 hubMu 完成 `t` 赋值，回调取锁后必可见；四条陈旧逃逸路径——新纪元重武装/取消置 nil/注册表非空/exiting——全部收口）。

实证基线：`go build ./...` + `go vet`（三包）干净；`go test ./cmd/wesh ./internal/pty ./internal/server -count=1` 全绿（server 包 53.7s）；`GOOS=darwin GOARCH=arm64` 交叉编译通过。反向实证两条：`GOOS=linux GOARCH=386 go build ./...` **编译失败**（WR-03）；`wesh true --config probe.toml` 进程级复现配置文件从子命令 argv 位被静默加载（CR-01）。

主要问题：prescanConfigPath 与 flag.Parse 的扫描边界分叉（CR-01，前轮复审已报、当前树仍未修，本轮独立复现并升级定级）；D-07 空数组误报与 32-bit 编译破口两条 Warning；另有 7 条 Info（死状态字段、注释/文档失真、未测修复分支等）。

## Critical Issues

### CR-01: prescanConfigPath 不停于首个非 flag 参数——子命令 argv 位的 `--config` 被静默当作 wesh 配置加载

**File:** `cmd/wesh/config.go:177-199`（消费点 `cmd/wesh/main.go:201-216`）
**Issue:** 预扫器只在遇到字面量 `--` 时停止，而 Go flag 包在**首个非 flag 参数**处即停止解析（其后全部落入 `fs.Args()` 子命令 argv）。`--` 是可选的（`wesh bash` 合法），因此两条扫描边界结构性分叉：

- `wesh true --config /path/x.toml`：`--config` 按 CLI 契约属子命令 argv，正式 Parse 永不消费它；但预扫把它当 wesh flag 加载 x.toml 并以其值铺底。**进程级复现**：probe.toml 内容 `port=0, bind="127.0.0.1"`，运行后启动打印 `listening on http://127.0.0.1:38123`——bind/port 均来自 TOML 而非内置默认 `0.0.0.0:7681`，证明文件被静默加载应用。若该 TOML 含 `writable = true`，operator 以为的默认只读会话实际可写——安全相关配置被未经明示地施加。
- `wesh -- mytool --config prod.toml` 形态（子命令自带 `--config`）：若 prod.toml 恰好是合法 TOML 且含任何非 wesh 键 → 严格模式按未知键 exit 2，**子命令被拒绝启动**；不合法 TOML 同拒。主流用法（sshd/各类守护进程的 `--config`）被结构性误伤。
- 值位变体 `wesh --credential --config x.toml`：预扫照样加载 x.toml（正式 Parse 把 `--config` 吞作 `--credential` 的值）——虽最终必落错误退出，但「绝不为子命令/他 flag 参数加载配置文件」的不变量已破。

代码自身注释（config.go:174-176）明示不变量「`--` 之后是子命令 argv，其 --config 属于子命令**不扫描**（绝不为子命令参数加载配置文件）」——实现只覆盖了 `--` 形态，未覆盖 flag 包的 first-positional 停止规则，违反 D-01「仅显式指定、裸启动零漂移」的公开契约。前轮复审（04:31 报告 WR-01）已报此项，当前树仍未修复。
**Fix:** 以正式 Parse 为权威做交叉校验（廉价且收敛整类问题，优于在预扫器内维护带值 flag 表）：
```go
// fs.Parse 之后：
if configPath != configFileFlag {
    // 预扫路径未被正式 Parse 确认为 --config flag 值（落在子命令 argv/他 flag 值位）
    return cfg, nil, errors.New("invalid --config: must be given as a wesh flag before the command")
}
```
（last-wins 双给等一致形态不受影响：两通道同值即放行。）

## Warnings

### WR-01: D-07 权限警告对 `credential = []` 空数组误报「含凭据」

**File:** `cmd/wesh/config.go:160`
**Issue:** D-07 判定为 `if decoded.Credential != nil`。go-toml 把显式空数组 `credential = []` 解码为**非 nil 零长切片**（本轮以 go-toml 探针实测：`Credential==nil:false len=0`），于是空数组 + 文件权限 0644 会打出 `... contains credentials and is readable by others ...`——文件中实际没有任何凭据。与 config.go:46-47 既定语义自相矛盾（「非 nil 空数组按缺席语义处理」）：合并层把空数组当缺席（零次迭代），警告层当含凭据。误导 operator 对无密文件 chmod；现有测试无 `credential = []` 用例。
**Fix:**
```go
if len(decoded.Credential) > 0 {
    if info, serr := os.Stat(path); serr == nil { /* ... */ }
}
```
并补 `credential = []` + 0644 → warn 为空的回归子测。

### WR-02: linux/386（含 32-bit arm）编译破口——`4294967295` 在 int32 上溢出

**File:** `cmd/wesh/main.go:706-711`
**Issue:** D-24 值域校验 `cfg.uid > 4294967295` / `cfg.gid > 4294967295`：`cfg.uid/gid` 为 `int`，无类型常量 4294967295 在 32-bit 平台（int=int32）不可表示 → 编译错误。实测：
```
$ GOOS=linux GOARCH=386 go build ./...
cmd/wesh/main.go:706:31: 4294967295 (untyped int constant) overflows int
cmd/wesh/main.go:709:31: 4294967295 (untyped int constant) overflows int
```
07-04 遗留（非本阶段引入），但 main.go 在本阶段审查面内。release 矩阵（.goreleaser.yml）仅 amd64/arm64，故为潜伏缺陷；internal/pty 构建标签只按 GOOS 不限 arch，源码自构建的 32-bit Linux 用户（32 位用户态 ARM 板等）直接撞编译错误。
**Fix:**
```go
if cfg.uid < -1 || int64(cfg.uid) > math.MaxUint32 { /* ... */ }
if cfg.gid < -1 || int64(cfg.gid) > math.MaxUint32 { /* ... */ }
```

## Info

### IN-01: `sessionModeSet` 只写不读的死状态字段；CONFIGURATION.md 过度承诺其组合校验面

**File:** `cmd/wesh/main.go:93, 531, 581`；`docs/CONFIGURATION.md:101`
**Issue:** `sessionModeSet` 在 parseArgs 两处置位（fs.Visit + fc.SessionMode 非 nil），注释称「消费归 10-02」；但 10-02 落地的 write-policy×per-client 警告锚定 `writePolicySet` + 模式终值（main.go:991），grep 全仓核实该字段生产代码**零读取点**（仅 config_test.go:817/908 断言置位）。CONFIGURATION.md:101 同时把 `session-mode` 列入「参与互斥/组合校验」的显式位键清单——该键不存在任何组合校验消费点，文档失真。
**Fix:** Phase 11+ 无消费计划则删字段与置位并修正文档；有既定消费点则改记真实挂点。

### IN-02: 测试命名陈旧——「all 27 keys load」

**File:** `cmd/wesh/config_test.go:32, 38`
**Issue:** fileConfig 覆盖面已 27→30 键（config.go 头注已同步 30），TestLoadFileConfig 子测名与头注仍写「all 27 keys / 27 键 = 26 flag 同名 + command」。子测实际加载 27 键（index 两键与 session-mode 由其余子测覆盖），「all」名实不符，误导后续维护者以为存在全覆盖断言。
**Fix:** 改名（如「core 27 keys load」）并注明 index/index-max-size/session-mode 的覆盖归属。

### IN-03: afterDrain 补发 Welcome「入队必成」的数学保证在文档自述下限容量处不成立（05-13 既有）

**File:** `internal/server/clients.go:516-519, 554`
**Issue:** 注释称补发 Welcome「入队必成沿用重投的数学保证（余量 ≥ cap/2+1 ≫ ~100B Welcome）」。精确推导：重投恰好可耗尽余量（cur ≤ cap/2−1 时 `cur + (32KiB+1)` 可达 cap），在注释自述的测试覆写下限 cap=64KiB 处，重投最大帧后余量可为 0，随后补发 Welcome 的 trySend 失败被 `_ =` 静默吞掉——该端会话尺寸滞留至下次 resize 事件（会自我纠正，无永久损坏）。生产默认 512KiB 余量 ~224KiB 不受影响。问题是注释把「恰好够重投」误述为「足够两者」，且失败分支零兜底零计数。
**Fix:** New 对 `opts.OutboxBytes` 设下限（低于 `2×(最大帧+Welcome 上界)` 时兜底默认或显式拒绝），或修注释为「重投必成、补发尽力而为」。

### IN-04: StartWithSize 导出函数不钳制尺寸，uint16 转换对越界输入静默回绕（本阶段新增接缝）

**File:** `internal/pty/spawn.go:71-100`
**Issue:** `StartWithSize` 契约注释把 ClampDim [1,1000] 钳制推给 Phase 12 调用侧，本函数零防御：`uint16(0)=0`（0×0 PTY）、`uint16(-1)=65535`、`uint16(70000)=4464` 均静默成形。本阶段唯一调用方是 inert SpawnFunc 闭包（零调用），风险属 latent；但导出函数 + 「调用方契约」组合在 Phase 11/12 接线时无任何编译期/运行期提醒，错误尺寸会沉默地变成另一个合法尺寸而非报错。
**Fix:** 函数入口加 `if cols < 1 || cols > 1000 || rows < 1 || rows > 1000 { return nil, errors.New("pty: dims out of range") }` 防御闸（一行成本，消灭整类静默回绕），或在 Phase 11 接线 PR 中强制补测越界用例。

### IN-05: WR-03 陈旧计时器身份闸无回归测试

**File:** `internal/server/clients.go:889-906`（测试缺口：`internal/server/emptyexit_test.go`）
**Issue:** `s.exitEmptyTimer != t` 身份比对是「已触发未取锁回调 vs 新纪元重武装」竞态的唯一闸，emptyexit_test.go 六路径覆盖立即/宽限取消/宽限到期/kick/exiting/启动免疫，但无一触发「Stop 对已触发计时器返回 false 后回调取得 hubMu」的陈旧分支——该分支只能靠白盒（预武装→替换计时器→直接驱动回调逻辑）或时序注入触及。修复正确性本轮经锁同步推演确认，但未来重构（如取消点顺序调整）无测试兜底。
**Fix:** 白盒单测：hubMu 内武装 T1 → 直接置 `s.exitEmptyTimer = T2`（模拟新纪元重武装）→ 手工执行 T1 回调体 → 断言 `stopChildLocked` 未触发（信号计数探针）。

### IN-06: mergeBatch 守卫不对称——查 `len(batch[j]) > 0` 却直接取 `batch[i][0]`（05 既有）

**File:** `internal/server/clients.go:692`
**Issue:** 合并条件 `len(batch[j]) > 0 && batch[j][0] == batch[i][0] && batch[i][0] == proto.Output` 对 j 位查空却对 i 位裸取 `[0]`。当前全部产帧方（OUTPUT 组帧/WelcomeFrame/ExitFrame）恒产 ≥2 字节帧，空帧结构性不可达，属潜伏 panic 面：任一未来产帧方入队空帧即 index-out-of-range 崩溃整个 hub writer 调用链。
**Fix:** 条件首位补 `len(batch[i]) > 0 &&`（零成本对称化），或在 trySend 入口断言帧非空。

### IN-07: run() 的 ValidateOptions 最小字面量调用存在静默欠校验漂移面；错误双前缀

**File:** `cmd/wesh/main.go:1328-1331`；`internal/server/server.go:336-348`
**Issue:** 前移后的契约校验以 `server.Options{SessionMode: ..., SpawnFunc: ...}` 最小字面量调用，「与完整 opts 语义等价」依赖 ValidateOptions 永远只读这两字段——Phase 11 已预告 sess 维度规则，届时该调用点静默欠校验且无任何测试能捕获（最小字面量恒满足新增字段零值）。另：错误经 `wesh: %v` 打印得 `wesh: server: session-mode ...` 双前缀（当前生产不可达——枚举闸 + 分岔保证两输入恒一致——仅观感）。
**Fix:** 把完整 opts 字面量（含 share token 生成）前移至 pty.Start 之前，对真实 opts 调 ValidateOptions；最低限度在 ValidateOptions 注释登记「字段增维必须同步 run() 调用点字面量」的维护义务。

---

_Reviewed: 2026-09-03T07:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
