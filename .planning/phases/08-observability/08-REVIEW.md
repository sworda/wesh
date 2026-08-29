---
phase: 08-observability
reviewed: 2026-08-28T12:27:12Z
depth: standard
files_reviewed: 27
files_reviewed_list:
  - cmd/wesh/main.go
  - internal/server/auth_e2e_test.go
  - internal/server/auth.go
  - internal/server/clients.go
  - internal/server/emptyexit_test.go
  - internal/server/events_test.go
  - internal/server/exitmsg_test.go
  - internal/server/health.go
  - internal/server/health_test.go
  - internal/server/limits_test.go
  - internal/server/log.go
  - internal/server/log_test.go
  - internal/server/metrics.go
  - internal/server/metrics_test.go
  - internal/server/multi_test.go
  - internal/server/proxy_e2e_test.go
  - internal/server/proxy.go
  - internal/server/proxy_test.go
  - internal/server/server.go
  - internal/server/slowclient_test.go
  - README.md
  - web/uat/phase05-dom.mjs
  - web/uat/phase05.mjs
  - web/uat/phase07-b2.mjs
  - web/uat/phase07.mjs
  - web/uat/phase08-journal.mjs
  - web/uat/phase08.mjs
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-08-28T12:27:12Z
**Depth:** standard
**Files Reviewed:** 27
**Status:** issues_found

## Summary

本轮为同一 phase 的新一轮对抗性审查，范围含 gap closure 08-06 新增的 README.md journald 防护段与 web/uat/phase08-journal.mjs 合流模拟回归脚本。上一轮（2026-08-28T04:26:40Z）4 项发现已逐条复核：**全部修复落地**——log.go D-15 注释已改为「RFC3339Nano 携进程本地时区偏移」正确陈述、metrics.go:146 已改 `_, _ = fmt.Fprint` 显式丢弃、proxy.go 已补 IN-02 分叉边界注释、README 事件表 auth_failed/throttled 行已补 `(, remote_user)` 可选键。

本轮审查覆盖：08-01 slog 迁移基座（log.go）、08-02 审计事件目录（server.go/clients.go/auth.go 挂点）、08-03 /healthz（health.go + 路由注册）、08-04 /metrics（metrics.go + 计数器挂点）、08-06 README journald 示例 + phase08-journal.mjs，以及全部受影响的 Go 测试与 UAT 脚本。交叉读取了 config.go/throttle.go/tickets.go/sharetoken.go/origin.go/headers.go/export_test.go 以验证跨文件契约（UAT 断言的字符串契约、路由注册语义、节流窗口算术）逐条相符。

**安全红线专项复核（本轮重走数据流）**：

- 凭据/ticket/share token/Authorization 头无任何形态进入事件流或 metrics label；`--auth-header` 凭据载体头名拒绝闸（main.go:602-607）封闭了「配置即破线」缺口；UAT 三脚本（phase07/07-b2/08/08-journal）的 sensitiveTokens + assertOutputClean 运行时自净断言完整，phase08-journal.mjs 的 jq 缺失豁免路径（exit 0 前未跑 assertOutputClean）经核实彼时 emittedDetails 仅含 skip reason，无敏感面。
- phase08-journal.mjs 合流夹具断言组完备：负对照（J1 无防护管道必 parse error）证明夹具不空转，全流纯度（J2）+ 两则 README 逐字管道（J3/J4）锁定 G-08-2 修复面；`PIPE_EX2` 的 `==1` 与 README 示例 `==7` 的数字差异有显式豁免注释（夹具确定性参数 vs 示例 N），不构成漂移。
- 锁序与并发：snapshotMetrics 的 hubMu > outbox.mu 单趟快照无 ABBA 面（TestMetricsSnapshotRace 压力锁）；stderrMu 叶锁形态保持；pinger pongTimedOut 置位/读取均经 hubMu（Pattern 4 形态 b）。
- 节流窗口算术逐个测试核过（TestThrottleHTTP/TestMetricsAuth/TestThrottledRetryAfter 的 sleep 序列与 `base << min(fails-1, 5)` 级数逐值相符）；README「爆破 100 次累计等待 ≥47 分钟」与 1s 翻倍 30s 封顶级数一致。

未发现 Blocker 级缺陷。2 项 Warning：exit-when-empty 在「递补升格踢出致空」边角路径下事件恰好一次纪律失守（重复触发）；`--ping-interval` 负值缺失同级 duration flag 均有的负值闸，静默退化为禁用保活。4 项 Info 为文档准确性/可移植性/校验完备性层面的收口项。

## Warnings

### WR-01: exit-when-empty 在「递补升格踢出致空」路径下重复触发——事件恰好一次纪律失守

**File:** `internal/server/clients.go:583-611`（promoteNextLocked 循环）× `internal/server/clients.go:517-547`（kickSlowConsumerLocked）× `internal/server/clients.go:802-831`（maybeExitWhenEmptyLocked）

**Issue:** `maybeExitWhenEmptyLocked` 的三守卫（`!exitWhenEmpty || exiting || len(set)!=0`）不含「本轮致空已触发」门闩，而调用链存在同一次断开中两次到达的可能：

1. owner A detach（或 owner A 被 kick）→ `removeLocked(A)` 成功 → `promoteNextLocked()`（detach 路径 clients.go:747 / kick 路径 536）；
2. 递补循环找到 rwEligible 候选 B，但 B 的 outbox 连 ~100B 升格 Welcome 都写不进（事实上 stalled——该踢出重扫分支正是 05-03 为此边角所建）→ `kickSlowConsumerLocked(B)` → `removeLocked(B)` 后注册表恰空 → **kick 内部的 `maybeExitWhenEmptyLocked(B)` 首次触发**（clients.go:541）；
3. 循环重扫无可递补者 → owner=nil → 返回 → **外层 detach/kick 自身的 `maybeExitWhenEmptyLocked(A)` 在注册表仍空下第二次触发**（clients.go:752 / 541）。

后果：grace==0 时 `exit_when_empty` 事件行打两条（remote 分别归属 B 与 A）+ stop-signal 序列发两次（信号幂等无操作面危害）；grace>0 时 `exit_when_empty_wait` 打两条且计时器被 Stop 后重新武装。审计面「非空→空迁移每纪元恰一条」的语义（06-02 Pitfall 2 / 08-02 D-21 单入口纪律）在该边角失守。触发条件叠加（exit-when-empty 开启 × owner 模式 × 唯一递补者恰好 stall 到 outbox 全满）使其罕见，但路径结构上可达，且现有测试均未覆盖此交织（emptyexit_test.go 六路径的 kick 触发子测用 `Writable:false` 纯 ro 形态，无 owner 无递补，结构上走不到 promote 分支）。

**Fix:** 加「空纪元」门闩使触发每纪元恰好一次——Server 增加 hubMu 保护字段 `exitEmptySignaled bool`：

```go
func (s *Server) maybeExitWhenEmptyLocked(c *client) {
	if !s.exitWhenEmpty || s.exiting || len(s.registry.set) != 0 || s.exitEmptySignaled {
		return
	}
	s.exitEmptySignaled = true // 空纪元内幂等：promote 踢出致空与外层移除点只发一次
	// ……既有 grace==0 / grace>0 两形态不变
}
```

并在 Attach 升档序列 `registerLocked` 成功后的同一 hubMu 持有内清零（`s.exitEmptySignaled = false`，与 `cancelExitEmptyTimerLocked` 同点）——新 attach 开启新纪元，下次致空重新允许触发。补一个白盒/集成测试：owner A + rwEligible 且 outbox 预填至满的 B，detach A，断言 `exit_when_empty`（或 `_wait`）事件恰 1 条。

### WR-02: `--ping-interval` 缺负值闸——负 duration 静默退化为「禁用保活」，与同级 flag 校验纪律不一致

**File:** `cmd/wesh/main.go:310`（flag 注册）× `internal/server/server.go:1117-1120`（pinger `interval <= 0` 直接返回）

**Issue:** 项目对 duration flag 的既定纪律是负值 parse 期拒绝——`--exit-when-empty`（exitEmptyValue.Set，main.go:129 `err != nil || d < 0`）与 `--stop-timeout`（main.go:621-623）都把「负 duration 是合法语法，负值检查是唯一闸」写成显式防线。`--ping-interval` 无此闸：`--ping-interval=-5s`（或配置文件 `ping-interval = "-5s"`，合并期 `time.ParseDuration` 同样放行负值）被接受，随后 pinger 按 `interval <= 0` 静默不启动——即用户笔误把保活关了而零报错。文档语义只承诺 `0` = 禁用（D-16/README「`0` = 禁用」），负值无任何文档地位。实际危害面：反代部署下保活静默消失后，空闲连接被反代空闲超时收割（nginx 60s 级），用户表现为「终端莫名掉线」而启动面无任何提示。与「配置错误零窗口暴露」的启动校验哲学（P3 校验矩阵纪律）不一致。

**Fix:** parseArgs 的 Parse 返回处补同级负值闸（write-policy/stop-timeout 同位先例；配置来源负值经默认值替换机制落同一终值，一闸双覆盖）：

```go
// D-16：--ping-interval 负值拒绝（0 = 禁用为唯一合法非正形态；
// exitEmptyValue.Set/--stop-timeout 负值闸同纪律；值非敏感可回显）。
if cfg.pingInterval < 0 {
	return cfg, nil, fmt.Errorf("invalid --ping-interval %v: must be a non-negative duration (0 = disable keepalive)", cfg.pingInterval)
}
```

## Info

### IN-01: README detach 行「计数走 metrics」对 pong_timeout 不成立

**File:** `README.md:451`

**Issue:** 事件目录表 detach 行括注「kick/pong_timeout 不再单独打行，计数走 metrics」。前半句正确（D-21 折入 detach reason）；后半句只对 kick 成立（`wesh_clients_kicked_total`，clients.go:522 挂点）——17 series 契约中没有任何 pong_timeout 计数器，该 reason 的唯一计数面就是日志事件本身。operator 按表找 pong_timeout 的 metrics 计数会扑空。注意 17 series 是锁定契约（metricsSeries17 形状断言），修法应改文档而非加 series。

**Fix:** 该行括注改为「kick/pong_timeout 不再单独打行（折入本事件 reason）；kick 计数走 metrics（`wesh_clients_kicked_total`）」。

### IN-02: uid/gid 值域上限常量 `4294967295` 在 32 位架构下编译失败

**File:** `cmd/wesh/main.go:628-633`

**Issue:** `cfg.uid`/`cfg.gid` 为 `int`（`fs.IntVar` 承载），与无类型常量 `4294967295`（> math.MaxInt32）比较时，常量须可表示为 `int`——在 GOARCH=386 等 32 位目标上该比较是**编译期错误**（constant overflows int）。internal/pty 的构建标签只约束 linux/darwin 不约束架构，故 `GOOS=linux GOARCH=386 go build ./...` 会失败。项目实际目标平台（amd64/arm64）不受影响，属可移植性隐患而非现行缺陷。

**Fix:** 二选一——(a) 比较前显式转 int64（`if cfg.uid < -1 || int64(cfg.uid) > 4294967295`）；(b) 若 32 位明确 Out of Scope，在代码注释或 PROJECT.md 登记该假设，防未来交叉编译踩中。

### IN-03: `normalizeBasePath` 放行含 `/.` 段的值，该值在 ServeMux 下结构性不可路由

**File:** `cmd/wesh/main.go:804-827`

**Issue:** 严格校验拒绝 `..`/重复斜杠/尾斜杠/非安全字符，但接受含单点段的路径（如 `/wesh/./x`——`.` 在允许字符集内，单点不含 `..` 子串）。Go 1.22+ ServeMux 对一切请求先 CleanPath 并重定向到净化路径（`/wesh/./x/` → 301 至 `/wesh/x/`），而注册的模式串是未净化原值——净化后的路径永不匹配，该 base-path 下所有端点 404。即：输入被 parse 期接受、生效值结构性不可达，正是 D-13 注释声明要防的「输入与生效值分叉」形态（此处分叉来自 mux 净化语义而非自动修正）。非安全问题（失败形态是响亮 404 而非静默漂移），但属严格模式校验的完备性缺口，operator 要到运行期才发现配置不可用。

**Fix:** 校验段追加单点段拒绝（任一路径段恰为 `.` 即拒，如 `strings.Split(s, "/")` 逐段检查），错误文案与既有四形态同款回显原输入。

### IN-04: detach reason 判定序下 pong_timeout 与并发正常关闭/关停广播的归属竞态

**File:** `internal/server/clients.go:729-735`

**Issue:** reason 判定 switch 先查 `c.pongTimedOut` 再查 `s.exiting`。两个窄竞态窗口：(a) 客户端已发出正常关闭帧、pong 恰好同时在 pinger 侧超时——pinger 先取 hubMu 置位则本次正常关闭被记为 `pong_timeout`；(b) Shutdown/lifecycle 广播窗口（`s.exiting=true`）内，一条早已不应答 pong 的死连接由 pinger 超时置位先行，其 detach 记为 `pong_timeout` 而非 `shutdown`。两窗口内连接本身确已恶化（pong 超时在先），误标语义上可辩，仅影响审计归因精度，无功能后果。按「观察到即登记」纪律记此一条，处置可 wontfix。

**Fix:**（如决定修正）判定序调整为 `s.exiting` 优先可消 (b)；(a) 窗口本质性存在，可不动。

---

_Reviewed: 2026-08-28T12:27:12Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
