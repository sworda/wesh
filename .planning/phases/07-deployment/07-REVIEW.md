---
phase: 07-deployment
reviewed: 2026-08-26T07:31:41Z
reverified: 2026-08-26T08:39:28Z
depth: standard
files_reviewed: 25
files_reviewed_list:
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - cmd/wesh/config.go
  - cmd/wesh/config_test.go
  - go.mod
  - go.sum
  - internal/proto/proto.go
  - internal/pty/spawn.go
  - internal/pty/spawn_test.go
  - internal/pty/signal_linux.go
  - internal/pty/signal_darwin.go
  - internal/pty/signal_test.go
  - internal/server/server.go
  - internal/server/sharetoken.go
  - internal/server/auth.go
  - internal/server/clients.go
  - internal/server/proxy.go
  - internal/server/proxy_test.go
  - internal/server/basepath_test.go
  - internal/server/shutdown_test.go
  - internal/server/stopseq_test.go
  - internal/server/proxy_e2e_test.go
  - web/src/main.ts
  - web/uat/phase07.mjs
  - web/uat/phase06-dom.mjs
findings:
  critical: 3
  warning: 2
  info: 1
  total: 6
status: clean
---

# Phase 7: Code Review Report

**Reviewed:** 2026-08-26T07:31:41Z
**Re-verified:** 2026-08-26T08:39:28Z（fix 区间 167e572..e022a8b 复核通过）
**Depth:** standard（逐文件 + 语言特定检查；跨文件追踪关键调用链）
**Files Reviewed:** 25
**Status:** clean（六项发现全部 verified，无 rejected，无新发现）

## Summary

对照 07-CONTEXT 锁定决策与六条安全红线逐文件审查。红线核验结果：D-03（token/ticket 不入 logEvent）主路径守住且有运行时自净断言，但 --auth-header 头名未做凭据头拒绝，存在配置即破线的结构性缺口（CR-03）；D-17（头值只记录不影响认证）成立（TestAuthHeaderNoAuthBypass 回归锁）；D-20（未配置时 XFF 完全忽略）成立；停止信号负 pid 进程组 + uid/gid 成对强制成立；1001 不进前端重连成立；config 值剥离红线在 credential/client-option 路径守住，origin 路径含值（IN-01）。另发现 listenSocket 无条件删除任意文件（CR-01）与 XFF 值未清洗进日志（CR-02）两处数据/安全问题。

**Re-verify 结论（2026-08-26T08:39:28Z）：** 六项修复全部真实落地且测试实证覆盖，未发现回归与新问题；CR-01 拒绝 tier 归属裁决为**接受 fixer 的 exit 1 运行时通道**（理由见文末裁决节）。验证活动：`go test ./...` 五包全绿（46.7s）；修复断言定向 -v 运行全 PASS 无 SKIP（TestListenSocket 5 子测 / TestProxyClientIP 22 行 / TestTLSKeyPairError 4 新行 / TestStartupRefusalNoResource auth-header 行 / TestConfigRedLines 2 新行 / TestConfigMerge 3 新子测）；当前 HEAD 重构建二进制（时间戳核验）重跑 phase07.mjs **34/34** + 1 skipped（S8c 平台豁免）+ SEC 自净 34 detail 零命中；WR-01 独立冒烟 exit 143 / 第二次 SIGTERM 后 3ms 终结；修复区间 diff --stat 核对仅 7 预期文件无夹带；GOROOT NotifyContext stop() 幂等性核验（cancel 可重入 + signal.Stop 对摘除后 channel no-op，双调无害声明成立）。

## Critical Issues

### CR-01: listenSocket 无条件 os.Remove，非 socket 类型文件被静默删除

**Fixed:** 167e572（UAT 场景重对齐 8abe69c）
**Verified:** 修复真实解决问题且测试覆盖充分。① 当前源码 main.go:1015-1027 Lstat 类型闸落地：存在且非 ModeSocket → 返回错误拒绝启动，文件零触碰；存在且为 socket → Remove 后 Listen；Lstat 非 ENOENT 类错误（如父目录 EACCES）跳过删除直接 Listen 报错——fail-safe 方向不删文件。② 单测 TestListenSocket 五子测全绿：残留清理子测改用真实残留 socket（listen + SetUnlinkOnClose(false) + Close 制造，systemd Restart= 同款现场），新增 non-socket refused and preserved 子测锁拒绝 + 内容逐字节保留 + 类别文案。③ UAT S2a 用 SIGKILL 制造真实残留（Node net.Server close 会 unlink 造不出，夹具选择正确）、S2f 锁 exit 1 + "not a socket" + 内容保留，34/34 实证。④ 回归面考察：symlink 按非 socket 拒绝（Lstat 不跟随，保守防符号链接诱导，注释明示）；Lstat→Remove 间 TOCTOU 理论存在，但威胁模型为 operator 手误（目录写权限攻击者本可直接删文件），接受；rollback 路径（Chmod/Chown 失败回滚）五子测之一持续锁定未受影响。拒绝 tier 归属裁决见文末。
**File:** `cmd/wesh/main.go:963`
**Issue:** `_ = os.Remove(path)` 对 `--socket` 指向的任何现存文件类型一律删除——D-10 意图仅为清理残留 socket 端点，operator 手误指向普通文件（root/systemd 部署下有权限删除）即静默丢数据，超出决策面。
**Fix:** Remove 前 Lstat 判定类型，非 socket 拒绝启动：
```go
if fi, err := os.Lstat(path); err == nil {
    if fi.Mode()&os.ModeSocket == 0 {
        return nil, fmt.Errorf("%s exists and is not a socket", path)
    }
    _ = os.Remove(path)
}
```

### CR-02: XFF 链首值未清洗直接进入 logEvent remote 字段

**Fixed:** 85d161e
**Verified:** 修复真实解决问题。① 当前源码 proxy.go:93 落地：`ip != "" && net.ParseIP(ip) != nil` 才采信链首，否则与缺席同档回退 TCP 对端 host——正是审查建议的首选方案（同时收敛节流键卫生）。② ParseIP 通过值字符集恒为 [0-9a-fA-F:.]，C0/C1/CSI/空白结构性无从进入，与 sanitizeRemoteUser 同威胁类等价闸的论证成立。③ 测试 TestProxyClientIP 新增六行全绿且语义精确：合法 IPv6 保留（不误伤）、garbage/"unknown"/尾垃圾（"203.0.113.7 evil"）/CSI 注入（"1.2.3.4\u009b"，日志注入主证据）/IPv6 zone（"fe80::1%eth0"——带 zone 字面拒收属有意保守，XFF 不应携带 zone）均回退；trust_off 三行与非 trust remote() host:port 形态零漂移。④ UAT S4 四断言（链首换键 / sanitize / 对照组忽略 XFF）回归通过。⑤ 回归面考察：垃圾值回退共享 TCP 对端键后不再独占节流配额，D-20 已裁决风险面随之收敛，与注释声明一致；无新增行为分叉。
**File:** `internal/server/proxy.go:79-103`（写出点 `internal/server/server.go:1071-1077`）
**Issue:** trust 开启后 `remote()` 把 XFF 首段原样写入 stderr 单行日志——Go/nginx 均放行头值中的 obs-text（0x80-0xFF），攻击者经标准追加式反代（`$proxy_add_x_forwarded_for` 首段恒为客户端可控）可注入 C1/CSI（0x9B）伪造日志行甚至终端转义序列；D-19 为同一威胁类构建了 sanitizeRemoteUser 却只覆盖 remote_user，remote 路径漏防，且 CONTEXT 明示的「非法值回退 TCP 对端 IP」也只对空值生效（"unknown"/垃圾值直接当键）。
**Fix:** clientIP 首段先经 `net.ParseIP` 校验（非法即回退 TCP 对端，同时收敛节流键卫生），或对 remote() 的日志值施加与 sanitizeRemoteUser 相同的 C0/C1/DEL 剥离。

### CR-03: --auth-header 未拒绝 Authorization/Cookie 等凭据头名

**Fixed:** 03e0bf1
**Verified:** 修复真实解决问题。① 当前源码 main.go:602-608 落地：parse 返回处校验段（write-policy 枚举校验同位）对 cfg.authHeader 非空时经 http.CanonicalHeaderKey 归一后拒绝 Authorization/Proxy-Authorization/Cookie/Set-Cookie 四名——覆盖审查要求的最小清单，混合大小写同拒（aUtHoRiZaTiOn 行实证）。② 配置来源同闸实证：fc.AuthHeader 经默认值替换机制（main.go:276-277 → 421 注册）落 cfg.authHeader 同一终值，校验点在 fs.Parse 与合并之后，config_test.go「auth-header credential header」行锁配置通道拒绝。③ 值剥离：拒绝文案只含 flag 名与类别枚举（公开协议常量），TestTLSKeyPairError 混合大小写行的 forbiddenSub 锁用户原文零出现。④ run() 通道 TestStartupRefusalNoResource 锁 exit 2 零资源占用（parse 期先于一切 listen/spawn）。⑤ 回归面考察：空串默认值跳过闸（`cfg.authHeader != ""` 守卫）；showVersion 早退在闸前，与其他 parse 校验同位先例一致；proxy.go 红线注释头已登记缺口封闭。考察后不予立项的残余：带空白的头名（"Authorization "）过不了 CanonicalHeaderKey 归一匹配而放行，但非法头名在 HTTP 层永不可达（Header.Get 同款归一查找必落空），属 fail-safe 死配置而非泄露面。
**File:** `cmd/wesh/main.go:418`（提取点 `internal/server/proxy.go:110-115`）
**Issue:** 头名可配但无安全校验——`--auth-header Authorization` 是合理手误（authelia/oauth2-proxy 生态头名不统一正是 D-18 动机），配置后 Basic 凭据（base64）随每个认证事件写入 logEvent remote_user，直接击穿 D-03「凭据绝不出现在 logEvent」红线并落 journald 持久化；proxy.go 注释的「结构性保证」只论证了 token/ticket 进不来，未覆盖凭据头名配置。
**Fix:** parse 期/validateStartup 拒绝凭据载体头名（至少 Authorization、Proxy-Authorization、Cookie、Set-Cookie），与项目「危险半配置 fail-fast」哲学一致。

## Warnings

### WR-01: 关停期间第二次 SIGTERM/SIGINT 被 NotifyContext 吞掉

**Fixed:** b6e4b1e
**Verified:** 修复真实解决问题且经独立冒烟实证。① 当前源码 main.go:1176-1180 落地：goroutine 内 `<-sigCtx.Done()` 之后先 `stopSignals()` 恢复默认处置再 `srv.Shutdown()`，NotifyContext 官方推荐形态。② 幂等声明经 GOROOT 核验：signalCtx.stop() = cancel(nil) + signal.Stop(c.ch)，context cancel 可重入、Stop 对已摘除 channel 为 no-op，defer 与 goroutine 双调无害成立。③ 独立冒烟（当前 HEAD 构建二进制，--stop-signal TERM --stop-timeout 30s + 子进程 trap "" TERM）：首次 SIGTERM 进优雅关停，0.5s 后第二次 SIGTERM——3ms 内以 exit 143（128+15 默认动作）终结，与 commit 声明一致；未修复形态需等满 30s 宽限。④ 回归面考察：`<-sigCtx.Done()` 解除阻塞到 stopSignals() 执行之间存在理论竞态窗（此间到达的信号进缓冲 channel 丢弃），但窗口为纳秒级而 operator 双击间隔 >>100ms，且修复前该场景 100% 被吞——严格改善无劣化；正常终结路径（lifecycle os.Exit）defer 永不执行的行为不变。
**File:** `cmd/wesh/main.go:1114-1119`
**Issue:** 首次信号后 `stopSignals()` 仅在 run() 返回的 defer 执行，而正常终结走 lifecycle os.Exit 永不返回——Shutdown 全程（Close 内建最长 10s + stopTimeout）后续 SIGTERM/SIGINT 被转发进无人读取的 channel 丢弃，operator 习惯的双击 Ctrl+C 强杀失效，只能 kill -9。
**Fix:** goroutine 内 `<-sigCtx.Done()` 之后先调 `stopSignals()` 恢复默认动作再 `srv.Shutdown()`（NotifyContext 官方推荐形态），第二次信号即按默认终结进程。

### WR-02: 配置文件内 once=true 与 max-clients/exit-when-empty 冲突被静默覆盖

**Fixed:** 66c0f59
**Verified:** 修复真实解决问题且覆盖语义边界精确。① 当前源码 main.go:533-546 落地：fc.Once 非 nil 且为真时，fc.MaxClients 指向值 ≠1 或 fc.ExitWhenEmpty 经 exitEmptyValue.Set 单一解析路径换算 grace ≠0 即 configErr 拒绝——锚定 fc 字段（文件内矛盾），正是审查建议形态。② CLI 覆盖语义不受影响实证：TestConfigMerge 既有「once_overrides_config_max-clients / exit-when-empty」两行持续绿（fc.Once 未给时块不触发，CLI --once 展开覆盖配置值）。③ 一致冗余放行同档：once=true + max-clients=1 / exit-when-empty="true"（grace 0）放行（CLI --once + 显式 --max-clients=1 / 裸 --exit-when-empty 放行同档），第三新子测锁定。④ 值剥离：detail 只含类别 conflicting keys + 双键名，测试以精确子串断言锁值零出现；once 真时配置 exit-when-empty 非法 duration 也经同块 configErr 拒绝（与 553 应用点文案一致，无双轨）。⑤ 回归面考察：once=true + exit-when-empty="true" 时 Set 被解析两次（533 块内换算 + 553 应用点），幂等同值无行为影响；CLI --max-clients 显式给定不能「拯救」配置文件内自相矛盾（仍拒），与 fail-fast 严格模式哲学一致，属有意严格而非误伤。
**File:** `cmd/wesh/main.go:503-543`（矩阵落点 `:859-864`）
**Issue:** fc 只补置五个显式位（Port/Bind/SocketMode/SocketOwner/WritePolicy），MaxClients/ExitWhenEmpty 不置位——TOML 同文件内 `once=true` + `max-clients=5`（或 `exit-when-empty="30s"`）被 --once 展开静默改写为 1/0，而 CLI 同组合经 validateStartup exit 2 拒绝；同一配置文件内的自相矛盾逃过 fail-fast，与 D-06 严格模式哲学不一致。
**Fix:** 合并期显式检测配置内部矛盾（fc.Once 为真且 fc.MaxClients 指向值 ≠1、或 fc.ExitWhenEmpty 解析 grace ≠0 即 configErr 拒绝），CLI --once × 配置值的既定覆盖语义（flag > 配置）不受影响。

## Info

### IN-01: 配置 origin 校验错误文案含 origin 值

**Fixed:** b2180ab
**Verified:** 修复真实解决问题。① 当前源码 main.go:723 落地：detail 由 oerr.Error()（含 %q 原输入）改传 `key "origin"`，与 credential/client-option 记录式形态对齐。② 测试 TestConfigRedLines 子测改断言锁定双面：错误串含 invalid origin entry + `key "origin"`，且不含探针值 *.example.com（值剥离正断 + 反断齐备）。③ 回归面考察：CLI --origin 回调通道不经 configErr、值非敏感回显纪律不变（commit 声明与源码一致）；定位信息由行号降为键名属审查建议允许项（「必要时加行号」为可选——加载层类型不符已有 line N 通道，合并期语义错误以键名定位可接受）。
**File:** `cmd/wesh/main.go:676`
**Issue:** `configErr(configPath, "invalid origin entry", oerr.Error())` 把 NormalizeOrigin 错误（含 `%q` 原输入）作为 detail——origin 值经 configErr 通道进入错误串，与「类别+键名+行号」的值剥离形态不一致（credential/client-option 路径均只含类别+键名）；origin 虽按先例声明非敏感，但文案形态应统一。
**Fix:** detail 改传 `key "origin"`（必要时加行号），与 credential 记录式形态对齐。

## 裁决：CR-01 拒绝 tier 归属（fixer 遗留裁决点）

**裁决：接受 fixer 的 exit 1 运行时 listen 失败通道，不前移 validateStartup（exit 2）。** 五条理由：

1. **EADDRINUSE 等价论证（决定性）**：类型闸是删除行为的保守替代——若不加闸仅停止删除，残留普通文件下 net.Listen 必收 EADDRINUSE 走 exit 1。闸改变的只是错误文案的清晰度，不是失败档位；与 run():1098-1103「listen 失败 = sess.Close 回滚 + exit 1」既有形态逐字对称。
2. **项目 tier 分类先例一致**：main.go:1065-1069 注释明示 TLS LoadX509KeyPair 预检失败 =「运行时 I/O 错误，非 validateStartup 的 exit 2 配置矩阵错误」——文件系统内容/状态类失败（同为启动期可确定性检出）已既定归 exit 1。socket 路径上的文件类型是部署环境状态（同一路径本次不存在、下次出现普通文件），与 TLS 证书文件同档；exit 2 留给 cfg 纯函数可判定的配置形状/矛盾。
3. **TOCTOU 结构约束**：文件状态可在 validateStartup 与 listen 之间改变，权威检查点必须留在 listenSocket 内（Remove 前最后一刻）；validateStartup 前置副本无法消除运行时检查，只会制造双事实源漂移风险（两处判定逻辑一旦分叉，前置放行 + 运行时才拒 = 比现状更差的体验）。
4. **--cwd stat 预检不构成本案反例**：validateStartup 函数头允许只读探测，但 --cwd 验证的是配置值的属性（指向的目录须存在），而类型闸守护的是破坏性动作（os.Remove）的安全替代—— refusal 是被叫停的删除的替身，天然属于动作执行点。
5. **代价已覆盖**：exit 1 通道的拒绝非零资源占用（pty 已 spawn，经 sess.Close 回滚）——但回滚纪律有 T-07-02a/b 与 failure_rollback_leaves_no_residue 测试持续锁定；validateStartup 前移最多省一次 spawn-rollback，收益边际，不足以抵双事实源代价。

## Self-Check

- [x] 全部 25 个范围内源文件逐文件审查（go.mod/go.sum 核对依赖锁定；dist 产物未审，源码以 main.ts 为准）
- [x] 六条安全红线逐条核验：D-17/D-20/停止信号进程组/uid-gid 成对/1001 不重连 = 守住；D-03 主路径守住但 CR-03 结构性缺口；config 值剥离主路径守住但 IN-01 形态分叉
- [x] 锁定决策不重复报告：D-07 权限字面、supplementary groups 环境感知策略、SIG_IGN 夹具语义、S8c 豁免、gofmt 两既有漂移文件均未列入
- [x] 测试文件仅核可靠性，未报告断言外问题
- [x] 每个发现含 file:line + 严重级 + 一句话问题 + 一句话修复建议
- [x] 未修改任何源文件（只新增本 REVIEW.md）
- [x] Re-verify：六项修复逐项读 diff + 当前源码 + 测试断言三方核对；go test 全绿、定向 -v 全 PASS、UAT 34/34（HEAD 重构建二进制）、WR-01 独立冒烟、GOROOT 幂等性核验、修复区间文件清单核对无夹带
- [x] Re-verify：CR-01 tier 归属裁决出具（接受 exit 1 运行时通道，五条理由）；新发现 = 0

**Overall verdict: clean**——原 3 Critical + 2 Warning + 1 Info 全部 verified 修复到位，测试与 UAT 实证覆盖，无回归无新发现；CR-01 tier 裁决接受 fixer 方案。Phase 7 可密封。

---
_Reviewed: 2026-08-26T07:31:41Z_
_Re-verified: 2026-08-26T08:39:28Z (fix range 167e572..e022a8b)_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
