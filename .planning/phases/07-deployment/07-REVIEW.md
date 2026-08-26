---
phase: 07-deployment
reviewed: 2026-08-26T07:31:41Z
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
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-08-26T07:31:41Z
**Depth:** standard（逐文件 + 语言特定检查；跨文件追踪关键调用链）
**Files Reviewed:** 25
**Status:** issues_found

## Summary

对照 07-CONTEXT 锁定决策与六条安全红线逐文件审查。红线核验结果：D-03（token/ticket 不入 logEvent）主路径守住且有运行时自净断言，但 --auth-header 头名未做凭据头拒绝，存在配置即破线的结构性缺口（CR-03）；D-17（头值只记录不影响认证）成立（TestAuthHeaderNoAuthBypass 回归锁）；D-20（未配置时 XFF 完全忽略）成立；停止信号负 pid 进程组 + uid/gid 成对强制成立；1001 不进前端重连成立；config 值剥离红线在 credential/client-option 路径守住，origin 路径含值（IN-01）。另发现 listenSocket 无条件删除任意文件（CR-01）与 XFF 值未清洗进日志（CR-02）两处数据/安全问题。

## Critical Issues

### CR-01: listenSocket 无条件 os.Remove，非 socket 类型文件被静默删除

**Fixed:** 167e572（UAT 场景重对齐 8abe69c）
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

**File:** `internal/server/proxy.go:79-103`（写出点 `internal/server/server.go:1071-1077`）
**Issue:** trust 开启后 `remote()` 把 XFF 首段原样写入 stderr 单行日志——Go/nginx 均放行头值中的 obs-text（0x80-0xFF），攻击者经标准追加式反代（`$proxy_add_x_forwarded_for` 首段恒为客户端可控）可注入 C1/CSI（0x9B）伪造日志行甚至终端转义序列；D-19 为同一威胁类构建了 sanitizeRemoteUser 却只覆盖 remote_user，remote 路径漏防，且 CONTEXT 明示的「非法值回退 TCP 对端 IP」也只对空值生效（"unknown"/垃圾值直接当键）。
**Fix:** clientIP 首段先经 `net.ParseIP` 校验（非法即回退 TCP 对端，同时收敛节流键卫生），或对 remote() 的日志值施加与 sanitizeRemoteUser 相同的 C0/C1/DEL 剥离。

### CR-03: --auth-header 未拒绝 Authorization/Cookie 等凭据头名

**Fixed:** 03e0bf1
**File:** `cmd/wesh/main.go:418`（提取点 `internal/server/proxy.go:110-115`）
**Issue:** 头名可配但无安全校验——`--auth-header Authorization` 是合理手误（authelia/oauth2-proxy 生态头名不统一正是 D-18 动机），配置后 Basic 凭据（base64）随每个认证事件写入 logEvent remote_user，直接击穿 D-03「凭据绝不出现在 logEvent」红线并落 journald 持久化；proxy.go 注释的「结构性保证」只论证了 token/ticket 进不来，未覆盖凭据头名配置。
**Fix:** parse 期/validateStartup 拒绝凭据载体头名（至少 Authorization、Proxy-Authorization、Cookie、Set-Cookie），与项目「危险半配置 fail-fast」哲学一致。

## Warnings

### WR-01: 关停期间第二次 SIGTERM/SIGINT 被 NotifyContext 吞掉

**File:** `cmd/wesh/main.go:1114-1119`
**Issue:** 首次信号后 `stopSignals()` 仅在 run() 返回的 defer 执行，而正常终结走 lifecycle os.Exit 永不返回——Shutdown 全程（Close 内建最长 10s + stopTimeout）后续 SIGTERM/SIGINT 被转发进无人读取的 channel 丢弃，operator 习惯的双击 Ctrl+C 强杀失效，只能 kill -9。
**Fix:** goroutine 内 `<-sigCtx.Done()` 之后先调 `stopSignals()` 恢复默认动作再 `srv.Shutdown()`（NotifyContext 官方推荐形态），第二次信号即按默认终结进程。

### WR-02: 配置文件内 once=true 与 max-clients/exit-when-empty 冲突被静默覆盖

**Fixed:** 66c0f59
**File:** `cmd/wesh/main.go:503-543`（矩阵落点 `:859-864`）
**Issue:** fc 只补置五个显式位（Port/Bind/SocketMode/SocketOwner/WritePolicy），MaxClients/ExitWhenEmpty 不置位——TOML 同文件内 `once=true` + `max-clients=5`（或 `exit-when-empty="30s"`）被 --once 展开静默改写为 1/0，而 CLI 同组合经 validateStartup exit 2 拒绝；同一配置文件内的自相矛盾逃过 fail-fast，与 D-06 严格模式哲学不一致。
**Fix:** 合并期显式检测配置内部矛盾（fc.Once 为真且 fc.MaxClients 指向值 ≠1、或 fc.ExitWhenEmpty 解析 grace ≠0 即 configErr 拒绝），CLI --once × 配置值的既定覆盖语义（flag > 配置）不受影响。

## Info

### IN-01: 配置 origin 校验错误文案含 origin 值

**Fixed:** b2180ab
**File:** `cmd/wesh/main.go:676`
**Issue:** `configErr(configPath, "invalid origin entry", oerr.Error())` 把 NormalizeOrigin 错误（含 `%q` 原输入）作为 detail——origin 值经 configErr 通道进入错误串，与「类别+键名+行号」的值剥离形态不一致（credential/client-option 路径均只含类别+键名）；origin 虽按先例声明非敏感，但文案形态应统一。
**Fix:** detail 改传 `key "origin"`（必要时加行号），与 credential 记录式形态对齐。

## Self-Check

- [x] 全部 25 个范围内源文件逐文件审查（go.mod/go.sum 核对依赖锁定；dist 产物未审，源码以 main.ts 为准）
- [x] 六条安全红线逐条核验：D-17/D-20/停止信号进程组/uid-gid 成对/1001 不重连 = 守住；D-03 主路径守住但 CR-03 结构性缺口；config 值剥离主路径守住但 IN-01 形态分叉
- [x] 锁定决策不重复报告：D-07 权限字面、supplementary groups 环境感知策略、SIG_IGN 夹具语义、S8c 豁免、gofmt 两既有漂移文件均未列入
- [x] 测试文件仅核可靠性，未报告断言外问题
- [x] 每个发现含 file:line + 严重级 + 一句话问题 + 一句话修复建议
- [x] 未修改任何源文件（只新增本 REVIEW.md）

**Overall verdict: issues_found**——3 Critical（任意文件删除 / 日志注入 / 凭据头名配置即泄露）+ 2 Warning + 1 Info。CR-01/CR-03 修复成本低（各一处校验），CR-02 建议与 XFF 非法值回退一并收敛；建议修复后再密封 Phase 7。

---
_Reviewed: 2026-08-26T07:31:41Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
