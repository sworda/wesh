---
phase: 10-mode-assembly
reviewed: 2026-09-03T04:31:15Z
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
  critical: 0
  warning: 2
  info: 5
  total: 7
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-09-03T04:31:15Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

本轮为 phase 10 修复后（HEAD = aaaaa3e，含 189d081/0ec37cb/23f2df2 三修复提交与 10-05 收口文档）当前树态的全新复审。评审基线 = 12 个列入文件全量通读 + phase diff（f57c701^..HEAD，+566/-33）聚焦 + 交叉核实（grep 消费面追踪 + 进程级实证复现）。

**前轮发现复核（当前树态逐一验证）：**

- WR-01（per-client 预检 --cwd 错位）已修复且修复正确：main.go:1051-1063 的 stat 探测与 child chdir 后 execve 语义逐案对齐（相对 slash 路径 Join(cwd)、绝对路径直 stat、无 slash 走 LookPath——PATH 解析与父子 cwd 无关，无错位面）；三条 TestStartupMatrix 行为锁（放行/缺失/无执行位）就位。
- WR-02（ValidateOptions 调用点在资源获取之后）已修复且修复正确：main.go:1328 前移至 pty.Start/listen 之前，守卫触发时零资源占用，与其注释自引纪律恢复一致。
- WR-03（exit-when-empty 陈旧回调竞态）已修复且修复正确：clients.go:889-906 的计时器身份比对（`s.exitEmptyTimer != t`）——武装方全程持 hubMu，回调取锁后赋值必然可见（锁同步边），陈旧回调四条逃逸路径（新纪元重武装 / 取消置 nil / 注册表非空 / exiting）逐一推演全部收口。但见 IN-01：该修复无回归测试。
- 前轮 IN-01/IN-03/IN-04（uint16 截断契约、mergeBatch 守卫不对称、D-07 空数组误报）在当前树原样存续，本轮结转登记为 IN-04/IN-02/IN-03。

**本轮实证（非纸面阅读）：**

- `go build ./... && go vet`（三包）干净；`go test ./cmd/wesh ./internal/pty ./internal/server` 全绿（server 包 54.6s 无 flake）。
- **WR-01 进程级复现**：构建真实二进制，`wesh true --config /tmp/wesh-probe.toml`（无 `--`，配置文件内容为 `port=0, bind=127.0.0.1`）——启动打印 `listening on http://127.0.0.1:37487`（随机端口 + loopback 均来自 TOML 而非内置默认 0.0.0.0:7681），证明文件被当作 wesh 配置静默加载应用；同形态指向不存在文件 → exit 2 拒绝启动。两形态均与 flag.Parse 语义（`--config` 属子命令 argv）矛盾。
- `sessionModeSet` 消费面 grep 全仓核实：生产代码零读取点（写入点 main.go:531/581，读取点仅 config_test.go 两处断言），WR-02 前提成立。
- 配置键计数对账：fileConfig 30 键 = 28 flag 同名（32 注册 flag − no-auth/insecure-http/version/config 四排除项；help 非注册 flag 但 TOML 侧按未知键拒绝）+ command + index-max-size——config.go 头注释、CONFIGURATION.md「30 键」表逐键核对一致。

**本轮新增发现**：WR-01（prescanConfigPath 不在首个非 flag 参数处停止——前轮 IN-02 同族的更强向量，静默误载配置文件，已复现）、WR-02（sessionModeSet 只写字段 + 注释声称的 10-02 消费未发生）、IN-01（WR-03 竞态修复无回归测试）、IN-05（文档/测试锚点过期族）。WR-01 不涉及 phase 10 diff 新引入逻辑（07-06 区段前序代码），WR-02 是 10-01/10-02 的接缝残留，按前例标注出处。

## Warnings

### WR-01: prescanConfigPath 不在首个非 flag 参数处停止——子命令 argv 中的 --config 被当作 wesh 配置静默加载（07-06 区段前序代码；本轮进程级复现）

**File:** `cmd/wesh/config.go:177-199`（扫描循环）；注释声明处 `cmd/wesh/config.go:174-176` 与 `cmd/wesh/main.go:509-511`（「预扫与正式 Parse 的 --config 值一致」「双通道同值」）

**Issue:** Go flag 包的解析语义是「在首个非 flag 参数（不以 `-` 开头或为 `-`）处停止」（flag 包文档逐字："Flag parsing stops just before the first non-flag argument ("-" is a non-flag argument) or after the terminator \"--\""），而 prescanConfigPath 的循环只在 `"--"` 处 break——**不在首个非 flag 参数处停止**。于是无 `--` 形态的调用两通道判定分叉：

- `wesh true --config probe.toml`：flag.Parse 在 `true` 处停止，`--config probe.toml` 落入子命令 argv；prescan 却继续扫描并命中 `--config`，真实读盘并以其铺底。
- **复现证据（本轮实证）**：probe.toml 内容为 `port = 0` + `bind = "127.0.0.1"`，上述调用启动打印 `listening on http://127.0.0.1:37487`——bind 与随机端口均来自 TOML（内置默认是 0.0.0.0:7681），静默误载成立；同调用换不存在路径 → exit 2 `invalid config file ... cannot read`，即「子命令自己的参数」使 wesh 拒绝启动。

后果两态均真实：文件存在 → 操作者无意的配置被静默应用（bind/port/credential 等全部铺底生效，无任何提示）；文件不存在 → 与 wesh 无关的子命令参数触发 exit 2。`--` 是文档推荐形态但非强制（`wesh bash` 合法可用），该缝隙现实可达。前轮 IN-02 登记的是同函数的另一分叉向量（`--credential --config x.toml`——取值 flag 消费下一参数致判定错位），但评定「后果轻微（credErr 使 exit 2）」；本向量无 fail-fast 兜底、静默生效，故升级为 Warning。前轮 IN-02 建议的修复选项 (a)（仅降级注释）对本向量不充分。

**Fix:** 预扫循环增加非 flag 参数停止条件，与 flag.Parse 语义对齐：

```go
for i := 0; i < len(args); i++ {
    a := args[i]
    if a == "--" {
        break
    }
    if len(a) == 0 || a[0] != '-' || a == "-" {
        break // flag.Parse 在首个非 flag 参数处停止——此后一切 token 属子命令 argv
    }
    // ...既有 --config/-config 匹配逻辑不变
}
```

（说明：`-` 单独出现按 flag 语义也是非 flag 参数，一并 break；该停止条件同时使前轮 IN-02 的取值 flag 错位向量收窄但不消除——`--credential --config x.toml` 形态下 `--credential` 本身是 flag 不触发 break，该向量仍存续，维持前轮注释降级建议作为残余登记。）补 TestPrescanConfigPath 两行：`{"stop at first non-flag", []string{"bash", "--config", "/x.toml"}, ""}` 与裸 `-` 形态。

### WR-02: sessionModeSet 只写字段——注释声称的 10-02 消费未发生，测试反而把死位锁成「机制证据」（10-01/10-02 接缝残留）

**File:** `cmd/wesh/main.go:93`（字段注释「D-02 双源机制采集备用，write-policy×per-client warn 锚定归 10-02 消费」）、`cmd/wesh/main.go:529-532`（fs.Visit 写入）、`cmd/wesh/main.go:580-582`（fc 合并写入）、`cmd/wesh/main.go:179-182`（parseArgs 头注释同款声称）；对照消费点实况 `cmd/wesh/main.go:985-993`（warn 锚定 `cfg.sessionMode == server.SessionModePerClient` 终值，注释自述「锚定模式终值而非 sessionModeSet」）；测试锁定 `cmd/wesh/config_test.go:806-819`、`905-909`

**Issue:** 全仓 grep 核实：sessionModeSet 在生产代码中**只有两个写入点、零读取点**。10-02 落地时 D-01/D-02 warn 改为锚定模式终值（main.go:985-986 注释明确记录该取舍），字段注释与 parseArgs 头注释声称的「消费归 10-02」从未发生。更糟的是测试侧把死位锁成了机制证据：config_test.go:817-818 断言 sessionModeSet 置位并标注「D-02 warn 锚定机制的配置来源同档」——该机制实际不存在（warn 不读此位），断言为死代码提供了虚假合法性。当前无行为错误（终值锚定本身正确且双源覆盖成立），危害在维护面：Phase 11+ 实现者读字段注释会误认为存在既有消费者而沿用错误前提；「显式位」家族（writePolicySet/maxClientsSet/exitEmptySet 等）的全部先例都是「采集即有消费」，本字段破坏了该不变量却无任何注释承认。

**Fix:** 二选一：(a) 字段与两处写入保留（Phase 11 备用语义成立时），但全部四处注释改为「当前零消费方，D-02 warn 经终值锚定双源覆盖；本位为 Phase 11 备用采集」并修正 config_test.go:818/908 两处的机制声称；(b) 若 Phase 11 前无确定消费场景，删除字段 + fs.Visit/fc 两处写入 + 测试断言，需要时再以显式位家族先例重新引入。推荐先落 (a) 的注释修正，(b) 的删除评估随 Phase 11 挂接时定夺——现状（声称已消费）不可接受。

## Info

### IN-01: WR-03 陈旧计时器修复（23f2df2）无回归测试——身份比对分支无执行证据

**File:** `internal/server/clients.go:889-906`（修复本体）；对照 `internal/server/emptyexit_test.go`（本 phase diff 为空——七用例中无覆盖陈旧回调场景者）

**Issue:** WR-03 的身份比对修复经审查推演正确（武装方持 hubMu、回调取锁后 t 可见、四逃逸路径均收口），但 `s.exitEmptyTimer != t` 这一新增判定分支没有任何测试能到达：emptyexit_test.go 在本 phase 零改动，既有用例覆盖立即形态/宽限取消/到期/kick 触发/lifecycle 门/计时器越过 lifecycle/promote-kick 门闩，独缺「回调已触发但尚未取得 hubMu 期间完成一轮 attach+detach」的 WR-03 场景。竞态修复的正确性目前纯靠代码评审承载——与项目「行为锁」纪律（同文件 08-review WR-01 门闩即有 TestExitWhenEmptyPromoteKickOnce 白盒锁定）不一致。确定性构造可行：白盒持 hubMu → 等计时器到期（回调排队等锁）→ 同锁内完成 attach+detach 重武装 T2 → 放锁，断言陈旧回调不动作（无 exit_when_empty 触发行、子进程在 T1 到期点不收信号）。

**Fix:** 在 emptyexit_test.go 增补上述白盒用例（照 TestExitWhenEmptyPromoteKickOnce 的白盒先例直接驱动 hubMu/registry），或至少在该测试文件头注释登记「WR-03 分支经评审锁定、无执行证据」的风险接受。

### IN-02（结转前轮 IN-03）: mergeBatch 守卫不对称——空帧会使 `batch[i][0]` panic（05-03 区段前序代码）

**File:** `internal/server/clients.go:692`
**Issue:** 内层合并条件 `len(batch[j]) > 0 && batch[j][0] == batch[i][0] && batch[i][0] == proto.Output` 只守卫 `batch[j]` 的取字节，`batch[i][0]` 无长度守卫——零长度帧进入 outbox 即 slice 越界 panic（writer goroutine 崩溃 → 该客户端写端静默死亡）。当前全部生产者（onChunk 组帧 `1+len(chunk)`、WelcomeFrame/ExitFrame）恒 ≥1 字节，结构性不可达；前轮评定后本次复核现状不变。
**Fix:** 循环头归一化非空不变量（`if len(batch[i]) == 0 { i++; continue }`），或在 trySend 入口断言帧非空（单一事实源更靠入口侧）。

### IN-03（结转前轮 IN-04）: D-07 权限警告对空 credential 数组误报（07-06 区段前序代码）

**File:** `cmd/wesh/config.go:160-167`
**Issue:** 判定条件 `decoded.Credential != nil`——go-toml 把 `credential = []` 解码为非 nil 空切片（前轮沙盒实证，本轮复核代码未变），空数组 + 0644 权限也打出「contains credentials」警告（文件实际不含凭据，狼来了效应稀释 D-07 信号）。应用侧语义不受影响（main.go:787 循环零迭代，空数组按缺席处理）。另注（本轮附加观察，同函数）：os.Open 之后 os.Stat(path) 存在 TOCTOU 窗口——open 与 stat 之间文件被替换则警告反映的不是被读文件；advisory 性质下影响可忽略，仅登记。
**Fix:** 判定改 `len(decoded.Credential) > 0`；如需消除 TOCTOU 可同时改 `f.Stat()`（打开句柄）。

### IN-04（结转前轮 IN-01）: StartWithSize 的 uint16 截断锐利边仍仅由注释承载契约

**File:** `internal/pty/spawn.go:100`
**Issue:** `pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}`——int→uint16 对越界值静默截断（-1 → 65535、70000 → 4464）。ClampDim 钳制已显式划归 Phase 12 调用侧，当前唯一调用方 Start 传常量 80/24，无现实风险；但 Phase 11 SpawnFunc 挂接后调用面扩大，契约仍只靠注释。前轮建议（接缝处防御性钳制或显式报错）未采纳，现状复核不变。
**Fix:** 维持前轮建议：挂接 PR 中复查调用链钳制证据，或在 StartWithSize 入口加 ClampDim 同语义钳制使契约由代码承载。

### IN-05: 锚点过期族——fuzz_test.go 行号引用与 config_test.go 键数命名未随 10-01/10-03 演化同步

**File:** `cmd/wesh/fuzz_test.go:25-28`；`cmd/wesh/config_test.go:31-33,38,158-181`
**Issue:** 三处同族过期（本 phase 新增键后未回填）：

1. fuzz_test.go:25 注「configErr 单写口（config.go:86）」——configErr 实际在 config.go:93；
2. fuzz_test.go:28 注「值剥离经『只取 Key()』实现，config.go:98-102」——该逻辑实际在 config.go:116-122（SessionMode 字段及注释使行号漂移）；
3. config_test.go:38 子测名 "all 27 keys load" 与同函数头注释「27 键 = 26 flag 同名 + command」——fileConfig 现 30 键（该用例 TOML 只覆盖 27 键；index 两键与 session-mode 各有专测，但 "absent keys stay nil"（config_test.go:171-178）未把 SessionMode 纳入缺席-nil 断言清单——30 键中唯它无 decode 层缺席锚）。

均为注释/命名/覆盖精度问题，无行为影响；但本项目注释承担决策溯源职能（D-xx 锚定），行号/计数失真会误导后续考古。

**Fix:** 更新两处行号引用（或去行号化改为符号引用，免后续腐化）；子测名改 "all flag-named keys load" 类免计数命名并同步头注释键数；absent-keys 断言清单补 `fc.SessionMode != nil` 项。

---

_Reviewed: 2026-09-03T04:31:15Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
