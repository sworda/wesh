---
phase: 13-resource-defense
reviewed: 2026-09-05T20:12:32Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - cmd/wesh/config_test.go
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - docs/CONFIGURATION.md
  - internal/pty/reap_darwin_test.go
  - internal/pty/spawn.go
  - internal/pty/spawn_test.go
  - internal/server/clients.go
  - internal/server/emptyexit_test.go
  - internal/server/events_test.go
  - internal/server/export_test.go
  - internal/server/health.go
  - internal/server/load_test.go
  - internal/server/metrics.go
  - internal/server/metrics_test.go
  - internal/server/options_test.go
  - internal/server/perclient.go
  - internal/server/perclient_test.go
  - internal/server/server.go
  - internal/server/shutdown_test.go
  - internal/server/spawnthrottle.go
  - README.md
  - web/uat/phase13.mjs
findings:
  critical: 0
  warning: 2
  info: 3
  total: 5
status: issues_found
---

# Phase 13: Code Review Report

**Reviewed:** 2026-09-05T20:12:32Z
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

对 Phase 13（resource-defense）全部 23 个变更文件做了标准深度审查：通读全部生产源码（perclient.go / server.go / clients.go / spawnthrottle.go / metrics.go / health.go / spawn.go / main.go），以 `git diff e0ae66b^..HEAD` 锚定各文件实际变更范围，交叉验证了前端 `shouldReconnect` 声明（main.ts:1023 确认仅 1006 触发重连，1011 走终态面板——perclient.go 注释声明属实）、coder/websocket v1.8.15 库源码（`writeControl` 内建 5s 写超时——README 时序文档依据属实）、proxy.go sanitize/XFF 提取链与 pty.go whitelistEnv 注入链。构建 `go build ./...`、`go vet` 及 Phase 13 相关测试子集（throttle/metrics/remote-user/shutdown/emptyexit/reaped-fence，共 20+ 用例）全部通过。

核心结论：**shared 模式零回归红线成立**——shared 路径代码逐字未动，`resolveStopTimeout` 只在 `!stopTimeoutSet && per-client` 时覆写，`TestStopTimeoutResolution`/`TestConfigMerge` 双源（CLI/TOML）显式位锁定完备；spawn 双令牌桶判序（per-IP 先、全局后）与 wire 聚合（1011+定值文案三拒绝不可区分）实现与 D-04/D-05 声明一致；WR-02 waitDone 栅栏、teardownOnce 恰好一次、锁序（hubMu > outbox.mu，spawn 在 hubMu 外）均无破口。

发现两个 WARNING 级缺陷，均集中在 **Shutdown × per-client 的关停竞态面**：①关停窗口内新 attach 的会话逃逸快照信号面与 KILL 兜底（HUP 免疫子进程泄漏——恰是本 phase 默认 5s stopTimeout 防线要堵的洞）；②Shutdown 快照路径的 SIGKILL 兜底不递增 `ptyKills`，与 perclient.go「两路径覆盖全部 SIGKILL 兜底发送面」注释失实。无 Critical。

## Structural Findings (fallow)

（本次评审未提供 `<structural_findings>` 预载荷——无结构化 fallow 发现可登记。）

## Narrative Findings (AI reviewer)

### Warnings

### WR-01: Shutdown 窗口内新 attach 的 per-client 会话逃逸信号面与 KILL 兜底——HUP 免疫子进程泄漏

**File:** `internal/server/server.go:1746-1833` + `internal/server/perclient.go:158-339`
**Issue:** `Shutdown()` 的 per-client 分支先做注册表快照（1001 广播）再做 `pcSessions` 快照（逐组 stop-signal + AfterFunc KILL 兜底），最后有界 join（上界 = stopTimeout + 2s，默认 7s）。但整个关停期间 HTTP listener 与 `/ws` 持续服务（`Shutdown` 只置 `draining`——仅 `/healthz` 消费，`Attach`/`upgradePerClient` 无任何 `draining`/`exiting` 检查，守卫区③位 `registry.n` 在广播后为 0 恒放行）。在 join 窗口内完成 Hello→spawn→注册的会话：

1. 不在 `pcs` 快照内——Shutdown 既不发 stop-signal 也不武装 KILL 兜底；
2. 客户端在线期间 `teardownPCLocked` 不会触发（teardown 挂 detach/kick/watcher 三点）——detach-armed 的 5s KILL 兜底同样不存在；
3. join 到期 `!drained` → `terminate` → `os.Exit`——进程退出仅靠 master fd 被 OS 关闭产生的 SIGHUP 收口子进程。

普通 shell 子进程会随 fd 关闭死亡，但 **HUP 免疫子进程（nohup / `trap '' HUP`）在该窗口泄漏存活**——这正是 13-01 D-01 默认 5s stopTimeout 要结构性堵住的洞（server.go:1736 注释自述「快照漏掉残留者即服务端退出后进程泄漏」），late-attach 会话构成快照之外的第四类漏网形态。附带影响：该客户端既不在 1001 广播集（若晚于注册表快照注册）也大概率收不到 EXIT 帧（`os.Exit` 与 sessionWatcher 的 EXIT 直写竞速），表现为关停期秒级可用后突然 1006 死亡。

复现路径（默认配置）：per-client 实例 + `nohup` 型子进程 + SIGTERM 触发 Shutdown → 7s join 窗口内浏览器自动重连/新访客 attach → 进程退出后该子进程残留。窗口窄（≤7s）、需 HUP 免疫子进程叠加，故评 WARNING 而非 BLOCKER，但按本 phase 自身「服务端退出后进程泄漏」红线应修复。

**Fix:** 在 `upgradePerClient` 注册点复检临界区（perclient.go:261 的容量复检同款位置，hubMu 持有内）增加 `s.exiting` 复查——exiting 即按容量拒绝同款序列收口（`reapOrphanSession(sess)` + `rejectCapacity`），使「关停期 spawn」结构性不可能注册：

```go
	// D-03 注册点复检（既有）之后、client 构造之前：
	if s.exiting {
		s.hubMu.Unlock()
		s.reapOrphanSession(sess) // 孤儿回收含 HUP + stopTimeout KILL 兜底——WR-01 泄漏面闭合
		rejectCapacity(ctx, c, remote, remoteUser)
		return nil
	}
```

（如嫌 spawn 白费，可另在 throttle 判定前加 `s.draining.Load()` 早拒——但注册点复查是竞态无窗的唯一闸，两道并用更稳。）

### WR-02: Shutdown 快照路径的 SIGKILL 兜底不递增 ptyKills——metrics 观测面少计 + 注释声明失实

**File:** `internal/server/server.go:1768-1782`
**Issue:** SIGKILL 兜底「实际发送」点共三处，`s.mc.ptyKills.Add(1)` 只在两处：
- `perclient.go:616`（teardownPCLocked AfterFunc——递增 ✓）
- `perclient.go:667`（reapOrphanSession AfterFunc——递增 ✓）
- `server.go:1780`（Shutdown 快照 AfterFunc——**不递增** ✗）

后果：关停场景下在线会话的 KILL 由 Shutdown 侧计时器先发（与 detach-armed 计时器双武装竞速，Shutdown 侧先武装则先到期）时，`wesh_pty_kills_total` 少计。perclient.go:663-666 注释明文声称「两路径覆盖**全部** SIGKILL 兜底发送面」——与三发送点的现实不符（注释失实）；HELP 文案（metrics.go:192）"…(teardown and orphan reaping)" 则把 Shutdown 路径排除在外。两处自我描述互相矛盾且至少一处与代码行为不符。该 series 是 13-05 D-08 点名的 KILL 兜底 ops 信号，关停期恰是 KILL 兜底最密集的窗口（Trap 免疫会话集中于此收割），少计使「防线生效过」的观测面在最需要它的场景失真。`TestPerClientShutdownResidualGroup` 未断言 ptyKills，故该缺口无测试暴露。

**Fix:** 在 server.go:1780 发送点前与另两处同款递增（栅栏通过后、SignalGroup 之前）：

```go
					select {
					case <-pc.waitDone:
						return // Wait 已返回——reaped 置位在途，KILL 跳过
					default:
					}
					s.mc.ptyKills.Add(1) // WR-02：与 teardown/orphan 两发送点同计——三路径全覆盖
					pc.sess.SignalGroup(syscall.SIGKILL)
```

同步修正 perclient.go:663-666 注释（「两路径」→「三路径」）与 metrics.go:192 HELP 文案（补 "and shutdown"）。若裁决刻意不计关停路径，则反向修两处注释并登记理由——当前「注释说全覆盖、代码少一处」的漂移状态不可保留。

### Info

### IN-01: teardownPCLocked KILL 兜底回调内重复的 `if pc.reaped` 块——编辑残留死代码

**File:** `internal/server/perclient.go:600-605`
**Issue:** AfterFunc 回调体内连续两个完全相同的检查：

```go
					if pc.reaped {
						return
					}
					if pc.reaped {   // ← 不可达（同条件已在上行返回）
						return
					}
```

第二块恒为死代码。行为无害（幂等），但在本代码库「注释-代码一一锚定」的纪律下，这形态高度疑似补丁重复应用/手动编辑残留——若原意第二个检查应是别的条件（如 `s.exiting`），则真正意图已被吞掉。Shutdown 侧同构回调（server.go:1772-1774）只有单个 `pc.reaped` 检查，佐证本处为重复而非设计。

**Fix:** 删除 perclient.go:603-605 的重复块，保留单个 `pc.reaped` 检查 + 后续 waitDone 非阻塞 select 栅栏（与 server.go:1772-1779 形态对齐）。

### IN-02: README「pong 超时 5s~10s 窗口」对空闲连接形态低估（实际最坏 ~15s）

**File:** `README.md:97`（「保活先杀时序」段）
**Issue:** README 称完全停读连接「会在『停止读取后 5s~10s』窗口内被 pong 超时以 1006 关闭」。该区间只覆盖发送窗口已满的形态（`writeControl` 内建 5s 写超时，coder/websocket v1.8.15 write.go:276-279 核实）：**空闲连接**（无 OUTPUT 流量、TCP 窗口未满）形态下 ping 写成功、pong 等待耗满 `pongTimeout` 10s——最坏 = 下次 ping 间隔 5s + 10s ≈ **15s**。docs/CONFIGURATION.md:158 同段落措辞（「在连接满发送窗口时同样判读为 pong 超时」）是精确的，README 的无条件「5s~10s」与之自相矛盾。对按此窗口设计探活/超时预算的运维读者是误导。

**Fix:** README 该行改为「5s~15s 窗口（发送窗口满时 ~5-10s 经写超时收口；空闲连接经 pong 等待 10s 收口，最坏 ~15s）」或直接引用 CONFIGURATION.md 的条件化措辞。

### IN-03: spawnThrottleStore per-IP map 条目只重置不删除——trust 开启下键客户端可控的无界增长面（先例同构备案）

**File:** `internal/server/spawnthrottle.go:100-114`
**Issue:** `allow()` 对 15min 未活动条目做惰性**重置**（满额新桶）但从不 `delete`——map 单调增长，上界仅为历史 distinct 键数。throttle.go:34-35 对同构结构有明示裁决（「≈56B × 4096 IP ≈ 230KB 可接受；不设硬上限驱逐」），故非新缺陷；需备案的**增量**差异：throttleStore 条目只在认证**失败**时创建（recordFail），本 store 条目在**每次 attach 尝试**（成功/拒绝皆然）创建，且 trust 开启（`--auth-header` 配置）时键为 XFF 链首——标准追加式反代语义下该值客户端可控（proxy.go:82-86 自认），持有效凭据/token 的攻击者以旋转 XFF 高频连接可加速条目累积（~56B + limiter + string 键/条）。实际增速受连接速率与全局桶（8/s）约束，且需「trust 开启 + 已认证」双前提，量级远不及 DoS 面，维持 INFO。

**Fix:** 可选加固——`allow()` 内顺带清扫同键过期即 delete 再重建（语义不变），或定期（如每 N 次调用）遍历删除 lastSeen 超 15min 条目；不改亦可，建议在文件头内存界注释中补记「trust 开启下键客户端可控」一句使裁决覆盖增量面。

---

### 已核查无问题的关键面（负面结果备案）

- **shared 零回归**：`resolveStopTimeout` 判定仅依赖 `sessionMode`×`stopTimeoutSet`；shared 分支 `Shutdown` else 体、`stopChildLocked`、lifecycle 全链逐字未动；`TestStopTimeoutResolution` 八行覆盖 CLI/直构/TOML 三源。
- **锁序**：spawn 与 throttle 判定均在 hubMu 外；`snapshotMetrics` 单趟 hubMu→outbox.mu 同序；teardown 慢半段不占 hubMu；dwell 持帧段零锁——R-07/ABBA 无破口。
- **恰好一次**：`teardownOnce`/`termOnce`/`releaseOnce`/`exitEmptySignaled` 四件在双触发路径（detach×watcher、lifecycle×supervisor×Shutdown-fallback）下均有测试或结构性论证。
- **WR-02 栅栏（kill-after-reap）**：teardownPCLocked 与 Shutdown 快照两信号点的 `!reaped` + waitDone 非阻塞 select 双防线形态一致；`TeardownReapedFenceForTest` 确定性注入微窗口态验证收口链完整。
- **SEC-09 注入链**：`startOpts` 局部复制防串台（T-13-21）；sanitizeRemoteUser 先于注入（C0/C1/DEL 剥离使 env 注入面结构性不存在——cmd.Env 为字符串数组，无换行分裂面）；shared 恒空串不出键有 e2e 对照。
- **零敏感值**：spawn_failed/spawn_throttled/max_clients 三拒绝事件定值文案 + 键集白名单 + canary 负断言齐备；phase13.mjs 输出自净断言（token/pid 永不进 detail）运行时自证。
- **1011 不重连声明**：main.ts:1023 确认仅 1006 触发重连、1011 走终态面板（main.ts:1006）——churn 防线「拒绝不放大」前提成立。
- **phase13.mjs 帧常量**：`OUTPUT = INPUT = 0x30` 与 proto.go:22-24（`Input='0'`/`Output='0'` 同字节）一致，非笔误。

_Reviewed: 2026-09-05T20:12:32Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
