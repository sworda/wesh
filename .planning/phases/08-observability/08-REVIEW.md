---
phase: 08-observability
reviewed: 2026-08-28T04:26:40Z
depth: standard
files_reviewed: 27
files_reviewed_list:
  - cmd/wesh/main.go
  - internal/server/auth.go
  - internal/server/auth_e2e_test.go
  - internal/server/clients.go
  - internal/server/emptyexit_test.go
  - internal/server/events_test.go
  - internal/server/exitmsg_test.go
  - internal/server/export_test.go
  - internal/server/health.go
  - internal/server/health_test.go
  - internal/server/limits_test.go
  - internal/server/log.go
  - internal/server/log_test.go
  - internal/server/metrics.go
  - internal/server/metrics_test.go
  - internal/server/multi_test.go
  - internal/server/proxy.go
  - internal/server/proxy_e2e_test.go
  - internal/server/proxy_test.go
  - internal/server/server.go
  - internal/server/slowclient_test.go
  - README.md
  - web/uat/phase05-dom.mjs
  - web/uat/phase05.mjs
  - web/uat/phase07-b2.mjs
  - web/uat/phase07.mjs
  - web/uat/phase08.mjs
findings:
  critical: 0
  warning: 1
  info: 3
  total: 4
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-08-28T04:26:40Z
**Depth:** standard
**Files Reviewed:** 27
**Status:** issues_found

## Summary

以对抗姿态评审了 Phase 08 可观测性交付（15d07f6..HEAD，+3341/-185）：slog JSONHandler 迁移（log.go 动态 stderrW + stderrMu）、审计事件目录（attach/detach/session_*/shutdown）、/healthz 与 /metrics 端点、认证计数器挂点、phase08.mjs UAT 与 README 运维文档，以及全部受迁移影响的既有测试/UAT 断言（子串 → parseEvents 字段断言）。

**安全红线专项核验（逐条追踪过数据流，非仅抽样）**：

- 凭据/ticket/share token 无任何形态进入事件或 metrics label——`logEvent` 四参签名结构性无用户名通道，`auth_failed` 站点保持零改动；`TestAuthFailedNoUsername` 行为锁在场。
- 全部用户可控字段（`remote` 的 XFF 链首、`remote_user` 头值）在提取点完成 C0/C1/DEL 剥离 + 128 rune 截断；encoding/json 不转义 C1（已对照 GOROOT `encode.go` 语义核实），清洗确为唯一防线且双闸（ParseIP + sanitize）并存，无遗漏提取路径。
- metrics 17 series 零身份 label（唯一 label = build_info 的 version，经 `escLabel` 转义且转义顺序正确：反斜杠先行）；`/healthz` body 恒四字段粗粒度容量面，测试侧 `DisallowUnknownFields` 白名单锁完整。
- `basicAuth` 三调用点（root / attach 链 / /metrics）同传 `&s.mc`，401/429 计数与事件同址递增；WS 侧节流命中按 D-10 统一口径计入 `auth_failed` 而非 `auth_throttled`——与 HELP 文案（"HTTP 429"）及 README 一致，无双轨。
- 锁序核验：`snapshotMetrics` hubMu > outbox.mu 单趟快照，与 onChunk→trySend、afterDrain 同序，无 ABBA 面；`stderrMu` 为叶锁（持锁内仅 os.Stderr.Write，不取其他锁），无锁序环。
- 路由注册核验：`GET /metrics`+`/metrics`、`GET /healthz`+`/healthz` 方法模式与 path-only fallback 成对，无 mux 冲突；bp 模式下两运维端点根路径固定、拒绝双挂，测试逐码锁定。
- UAT 红线：phase08.mjs 的 sensitiveTokens 闭包 + assertOutputClean 运行时自净断言覆盖了凭据/探针串/token 三通道，含场景异常通道（emittedDetails 延伸）——未发现泄露路径。

**验证执行**：`go build ./...` 与 `go vet` 干净；phase-08 相关测试集（log/health/metrics/events/remote-sanitize/exit-signal 共 13 个测试函数）以 `-race -count=1` 全绿（8.6s）。

未发现 Blocker 级缺陷。1 项 Warning 为决策记录中的格式契约事实性错误（slog JSON time 并非「RFC3339 毫秒 UTC」），3 项 Info 为文档/约定层面的收口建议。

## Critical Issues

无。

## Warnings

### WR-01: D-15 决策记录对 slog JSON time 格式的描述与 stdlib 实际行为不符（两处均错：精度与时区）

**File:** `internal/server/log.go:10-12`
**Issue:** 决策头注释 D-15 称「time 为 RFC3339 毫秒 UTC（stdlib 固定不可配）」。对照本机 GOROOT 实证：`log/slog/json_handler.go:93-104` 的 `appendJSONTime` 使用 `t.AppendFormat(*s.buf, time.RFC3339Nano)`——**纳秒精度**，毫秒截断只存在于 TextHandler 路径（`handler.go:622 appendRFC3339Millis`）；且 Record 时间取自 `time.Now()` 本地时区，输出携 `+08:00` 类本地偏移而**非 UTC**。README.md:438 的示例行（`"2026-08-28T10:40:01.013456789+08:00"`）恰为正确形态——即 README 与 log.go 决策记录互相矛盾，错误的一方是 log.go。
这是审计日志格式契约（D-15/D-18 schema 是外部消费面——jq/Loki/解析脚本按此书写）：若下游解析器按「固定毫秒 + Z 时区」假设书写（如定宽切分或省略时区归一化），多机部署下会出现解析失败或跨时区时间不可比。行为本身无需改动（README 示例已正确），但决策记录的错误陈述应修正以防按错契约写消费端。
**Fix:**
```go
//   - D-15：运行期事件恒 JSON 恒 INFO，无 --log-format/--log-level（零新 CLI
//     契约）；time/level/msg 用 slog 默认键，time 为 RFC3339Nano 携进程本地
//     时区偏移（stdlib 固定不可配；毫秒截断仅 TextHandler 路径，JSONHandler
//     为纳秒）。人读检索走 jq。
```

## Info

### IN-01: metricsHandler 响应写出未按本仓库约定显式丢弃错误

**File:** `internal/server/metrics.go:146`
**Issue:** `fmt.Fprint(w, b.String())` 静默吞掉返回值。同 phase 的 health.go:53 用 `_, _ = w.Write(body)` 显式形态，全仓库 ignore-error 均为 `_, _ =` 显式标注（本文件 metrics.go 自身在 server.go 等处的先例同形态）。裸调用会被 errcheck 类 lint 命中，也与「显式丢弃即审阅过」的代码库惯例不一致。
**Fix:** 改为 `_, _ = fmt.Fprint(w, b.String())`（写出失败 = 采集端断开，静默即正确处置，仅需显式标注）。

### IN-02: `remote()` 加 sanitize 后与 `clientIP()` 的「同键不分叉」注册不变量出现理论缺口

**File:** `internal/server/proxy.go:105-123`（对照 `clientIP` L89-103）
**Issue:** 08-02 为 `remote()` trust 分支返回值追加 `sanitizeRemoteUser`（纵深第二道，正确），但注释登记的既有不变量「XFF 链首换入，日志归因与节流计数同键——两消费不分叉」不再严格成立：`clientIP()`（throttle/halfOpen 计数键）不清洗，`remote()`（日志字段）清洗。两值仅在「RemoteAddr 不可 SplitHostPort 的整串回退」路径上可能分叉（该回退值含控制字符时，节流键 = 原始串、日志 remote = 清洗串）。实际上此路径的输入为内核 netstack 提供的对端地址，字符集天然安全，缺口不可达；但按本仓库「注释登记的不变量即契约」的纪律，契约陈述已超前于实现。
**Fix:** 二选一——(a) 在 `remote()` 注释补一句分叉边界（「sanitize 仅日志面；计数键与日志值在内核源 RemoteAddr 回退路径上理论可分叉，实际不可达」）；或 (b) `clientIP()` 的回退末端同样过 `sanitizeRemoteUser`，使不变量字面成立。倾向 (a)：计数键不经日志面，无注入威胁，(b) 是为纯度付的额外开销。

### IN-03: README 事件目录表漏标 auth_failed/throttled 两事件的 `remote_user` 可选键

**File:** `README.md:446-455`
**Issue:** 事件目录表对 attach/detach/exit_when_empty 族标注了「(, remote_user)」可选键，但 auth_failed 与 throttled 行未标——而代码上两事件在配置 `--auth-header` 时同样携 remote_user（auth.go:127-129 throttled 分支、auth.go:141 经 logEvent 第四参的 auth_failed 分支，提取点 sanitize 同口径）。注意这与 SEC-01 红线无冲突（remote_user 是反代可信头，非凭据用户名），纯文档完整性缺口：operator 按表写 `select(.event=="auth_failed" and has("remote_user"))` 类检索时会误以为该键不存在。
**Fix:** 表中 auth_failed 行字段列改为 `remote, code=401/1008(, remote_user)`，throttled 行改为 `remote, code=429, retry_after(, remote_user)`。

---

_Reviewed: 2026-08-28T04:26:40Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
