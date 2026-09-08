---
phase: 11-per-client
reviewed: 2026-09-04T02:18:54Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - cmd/wesh/main.go
  - internal/pty/reap_darwin.go
  - internal/pty/reap_darwin_test.go
  - internal/server/clients.go
  - internal/server/export_test.go
  - internal/server/perclient.go
  - internal/server/perclient_test.go
  - internal/server/server.go
  - web/uat/phase11.mjs
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 11: Code Review Report

**Reviewed:** 2026-09-04T02:18:54Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

审查范围 = diff 基点 954da7c 以来全部 9 个变更源文件（与 config files 清单逐一核对一致，无漏审）。审查方法：逐文件精读 + 跨文件追踪（pty.Session 的 Wait/Drain/Close/SignalGroup/ReadLoop 语义、resize 零值仲裁器安全性、proto 字节常量方向复用、diff 词级比对确认 shared 路径逐字零回归、规划文档「已知中间态」登记对账）。

核心不变量核验结果（均成立，无 BLOCKER）：

- **shared 零回归**：词级 diff 确认 server.go/clients.go/main.go 的 shared 路径变化仅为 else 块缩进、per-client 增量字段与挂点；升档序列逐字保持。
- **Welcome 恒首帧**：per-client 入队先于 registerLocked/pcSessions 登记且全程持 hubMu，闭包只投本端 outbox，注册前无帧夹入窗口。
- **EXIT 私有化直写**：sessionWatcher 组帧一次 → 同步 Write(2s ctx) → Close(1000)，未经 outbox 异步入队（Anti-Pattern 6 防线成立）。
- **teardown 恰好一次**：sync.Once 双触发单执行；waitDone/teardownDone 各单点 close，失守即 panic 的结构性锁成立。
- **唯一收割者**：正常路径 sessionWatcher 唯一 Wait 调用方；orphan 路径 reapOrphanSession 唯一 Wait 调用方；两路径互斥不可同时装配。
- **spawn 失败通用文案**："failed to start process" 定值常量，err.Error() 零拼接；审计面四段 schema 零敏感值（测试含注入文本/argv 路径反断言）。
- **容量硬顶**：pre-spawn 闸 + 注册点复检双闸，「登记会话数 ≤ maxClients」硬不变量成立（瞬时 spawn 数可超编但 orphan 即杀，D-03 设计）。
- **锁序**：hubMu > outbox.mu 未反序；pcSessions 全部访问点持 hubMu；SignalGroup 不取 fdMu（signal_linux.go:16-18 实证），hubMu 内发信号安全。
- **已登记中间态不重复计为缺陷**：SIGTERM 后进程不自退（11-01-PLAN:281 明示接受）、--once/--exit-when-empty 永不退出、RESIZE 静默无效、session_start/end 空白、linger 滞留占容量、慢客户端 1013 默认行为——均已登记，本报告不计为 finding。

发现 2 项 WARNING（per-client 输出路径绕过 kickOrCreditLocked 两层判定重引入已实证修复的误踢竞态；reaped 栅栏存在 Wait-return→hubMu-acquire 窗口，「结构性不可能」注释声明过强且零成本严格修法存在）与 4 项 INFO。

## Warnings

### WR-01: per-client ReadLoop 闭包 trySend 失败直踢，绕过 kickOrCreditLocked 的 attach 宽限与「全体均满」信用两层判定

**File:** `internal/server/perclient.go:263-267`（对照 `internal/server/clients.go:498-520`、`:62-67`）
**Issue:**
per-client 每会话输出闭包在 `cl.outbox.trySend(frame)` 失败时无条件 `kickSlowConsumerLocked(cl)`（1013 立即踢出 → teardown SIGHUP → 会话销毁）。这绕过了 shared 路径 trySend 失败的既定判定机械 kickOrCreditLocked 的两层防线：

1. **attach 宽限层**（clients.go:505 `time.Since(c.attachedAt) >= defaultAttachGrace`）：500ms 宽限是 2026-08-29 三轮实证（kick_fail2_3：健康端 attach+6ms 因 writer 首次调度前满箱瞬态被误踢）后落地的误判防线——「新端瞬态满箱不是慢端证据」。per-client 闭包无此判定，同一竞态类重新敞开：慢链路/调度受限环境（GOMAXPROCS=1 CI 实证场景）下，客户端 attach 后遇子进程爆发输出（如 `wesh --session-mode=per-client --writable -- cat largefile`），outbox 512KiB 在 writer 首次调度前写满 → 新端在毫秒内被踢、shell 被杀。
2. **「全体可写端均满 → 置信用不踢」层**（clients.go:506-511）：shared 模式下唯一 rw 客户端满箱时，`oc != c && oc.mode == rw && !oc.creditBlocked` 遍历找不到他端 → 恒置信用（停读背压），**会话拥有者事实上永不被 OUTPUT 满踢出**。per-client 下同是该会话唯一 rw 消费者，出宽限后满箱必被踢——行为方向与 shared 相反，慢链路用户「attach → 输出一屏 → 被踢 → 重连新进程（PC-03）→ 再被踢」循环丢会话。

与规划文本的对照加重了该偏差的性质：11-PATTERNS.md:218-219 给出的闭包母本是 `s.kickOrCreditLocked(c, frame)`（含两层判定的完整机械），落地形态（11-PATTERNS.md:233、:474 与实现）换成了 kickSlowConsumerLocked；11-CONTEXT.md:94 登记「慢客户端 outbox 满 → 1013 的本阶段默认行为」时援引的理由是「1013 既有机械零改动复用」——但「既有机械」在单 rw 场景的实际行为是置信用不踢，且宽限判定是 05-13 才并入该机械的新部件。登记文本所援引的「零改动复用」与落地的行为分叉不符；宽限层的丢失未在任何规划文本中登记（「停读续读层 Phase 12」登记的是背压缺位，不覆盖误判防线）。

**Fix:**
闭包内 trySend 失败改为经过 kickOrCreditLocked 的等价判定，至少补回两层：

```go
if !cl.outbox.trySend(frame) {
    s.hubMu.Lock()
    if time.Since(cl.attachedAt) < defaultAttachGrace {
        // 宽限内瞬态满箱非慢端证据——置信用暂存（creditBlocked/creditPending
        // 与 afterDrain 半水位重投机械不依赖全局信用门，per-client 天然可用）
        if !cl.creditBlocked {
            cl.creditBlocked = true
            cl.creditPending = frame
            s.registry.gateTransitions++
        }
    } else {
        s.kickSlowConsumerLocked(cl)
    }
    s.hubMu.Unlock()
}
```

（若裁决维持「宽限外慢端 → 1013」的 Phase 11 默认行为，宽限层仍应补回；并在 11-CONTEXT 补记两层判定的取舍依据，消除「零改动复用」文本与实现的偏差。）

### WR-02: teardownPCLocked 的 reaped 栅栏存在 Wait-return→hubMu-acquire 窗口，「kill-after-reap 结构性不可能」声明过强

**File:** `internal/server/perclient.go:291-303`（sessionWatcher）、`:336-346`（teardownPCLocked 快半段与 AfterFunc）、`:85-89`（pcSession.reaped 注释）
**Issue:**
reaped 的置位序列是：`pc.sess.Wait()` 返回（**reap 在此完成，pgid 已释放可被内核复用**）→ `close(pc.waitDone)` → `s.hubMu.Lock()` → `pc.reaped = true`。即发信号点依赖的栅栏位在 reap 完成之后、隔一次锁获取才置位。两个发信号点均存在窗口：

1. detach 触发的快半段（:336-337）：detach 持 hubMu 期间若子进程刚被 Wait 收割（watcher 阻塞在 hubMu 外），`!pc.reaped` 为真 → `SignalGroup(s.stopSignal)` 发往已释放的 -pgid；
2. AfterFunc(stopTimeout) 补 KILL（:339-345）：到期回调与 watcher 竞争 hubMu，回调先取到则同样 `!pc.reaped` 为真 → SIGKILL 发往已释放的 -pgid。

两形态正是 pcSession.reaped 注释（:88「kill-after-reap 误杀复用 pgid 结构性不可能」）与 teardownPCLocked 注释（:335-336 同文）声称结构性排除的 kill-after-reap。实际可利用性确实极低（Linux pid 整轮回绕 pid_max 32768~4194304 + 微秒级窗口），与 reapOrphanSession 注释如实登记的「窄窗口……实际不可达」（:374-377）同档——但主路径注释用了「结构性不可能」的更强断言，且本路径窗口隔了一次锁获取（hubMu 竞争下无界），宽于 orphan 路径「Wait 返回 → reaped.Store 两条相邻指令」的窗口。声明与机制不符会误导后续维护者放松警惕。

关键的缓解事实是**严格的结构性栅栏零成本存在**：waitDone 在 reap 完成后、hubMu 获取前关闭，故「waitDone 未闭 ⇒ Wait 未返回 ⇒ reap 未完成 ⇒ pgid 未释放」恒成立。

**Fix:**
两发信号点统一改用 waitDone 非阻塞检查（或与 reaped 联合判定），并修正注释：

```go
// 快半段（:336）与 AfterFunc 回调（:342）的发信号前置判定：
reaped := pc.reaped
select {
case <-pc.waitDone:
    reaped = true // reap 完成的严格同步边（先于 hubMu 关闭，无锁窗口）
default:
}
if !reaped {
    pc.sess.SignalGroup(s.stopSignal) // 或 syscall.SIGKILL
}
```

同时将 :88 与 :335-336 注释从「结构性不可能」修正为与实际保障机制一致的表述（经 waitDone 同步边排除 / 残余窗口登记同 orphan 路径口径）。

## Info

### IN-01: darwin kqueue Q1 裁决结果未在仓库锚定，watcher 版 awaitExit 的正确性依赖 CI 人工观测

**File:** `internal/pty/reap_darwin.go:12-15`（兜底预案）、`:125-138`（awaitExit）、`internal/pty/reap_darwin_test.go:101-125`（ZombieRace）
**Issue:**
文件头兜底预案登记：若 TestKqueueExitZombieRace 裁决 kqueue 不补发僵尸进程 NOTE_EXIT，awaitExit 须退化为直接 `cmd.Wait()`。当前代码为 watcher 版本（隐含「补发」成立），但该竞态测试的否定出路是 `t.Skip` 而非 FAIL——CI 绿不证明裁决为「补发」。若 macOS leg 实际为 SKIP 且无人执行兜底，darwin 上「spawn → watch 注册窗口内瞬时退出的子进程」使 `awaitExit` 在 `<-exited` 永等 → 收割挂死（goroutine 泄漏 + EXIT 永不送达 + pcSessions 滞留占容量）。Phase 11 N 会话化放大了触发频率（任意命令 attach 期 spawn，瞬时退出命令常态化）。11-02-SUMMARY:129 仅记录测试随 CI 运行，未记录裁决结果；Linux 开发机无法验证。
**Fix:** 查明 CI macOS leg 的 ZombieRace 实际结果并在 reap_darwin.go 注释锚定裁决（PASS = 补发成立，watcher 安全）；若为 SKIP 立即执行兜底退化。

### IN-02: per-client 启动预检对 argv0="" 病态输入跳过，启动成功但每 attach 恒 1011

**File:** `cmd/wesh/main.go:1065`
**Issue:**
`cfg.argv0 != ""` 守卫使 `wesh --session-mode=per-client -- ""` 跳过 SC4 预检正常启动；此后每 attach 的 spawnFunc → `pty.StartWithSize([""], ...)` 失败 → Error+1011。shared 模式同输入在启动期即 pty.Start 失败 exit 1（启动期暴露）。病态输入，但 10-02 SC4「per-client 启动期 fail-fast 是 spawn 后置的结构性补偿」承诺存在覆盖缺口。
**Fix:** 预检守卫改为无条件检查（argv0 空串拒绝启动），或在 parseArgs 拒绝空 argv0 元素。

### IN-03: reapOrphanSession 的 Drain(200ms) 对从未装配 ReadLoop 的会话恒走超时分支

**File:** `internal/server/perclient.go:392`（对照 `internal/pty/io.go:85-91`）
**Issue:**
`Session.Drain` 等待的 `readDone` 仅由 ReadLoop 退出关闭（io.go:17）；orphan 会话从未装配 ReadLoop（本函数注释 :362-364 自述「从未装配 goroutine 群」），故 readDone 永不关闭，Drain 恒为空转 200ms 延迟后才 Close。行为正确（无正确性影响），但「给读循环留 EOF 窗口」的语义对孤儿恒不成立，读者易误判该等待有意义。
**Fix:** 孤儿路径直接 `_ = pcSess.Close()` 并以注释说明无 ReadLoop 故无 drain 对象；或保留现状但补注释说明恒超时的无害性。

### IN-04: 已登记中间态的衍生面——SIGTERM 后 per-client 进程在「关停中」稳态无限期接受新 attach 并 spawn

**File:** `internal/server/server.go:1570-1577`（Shutdown per-client 空分支）、`:888`（③位闸无 draining/exiting 检查）
**Issue:**
「SIGTERM 后进程不自退」已在 11-01-PLAN:281 明示接受（第二信号/systemd SIGKILL 逃生），本报告不重复计为缺陷。但其衍生面未登记：Shutdown 置 draining/exiting 后，Attach 升档链（③位闸 → upgradePerClient）不检查这两位——进程在 1001 广播后无限期接受新客户端 attach 并 spawn 新会话。shared 模式该窗口仅存于 1001 广播到子进程死亡的毫秒级区间（既有形态），per-client 下因进程不自退成为稳态：operator 视角「已发关停」的实例仍在产生新会话。
**Fix:** Phase 13 pcSupervisor/第二终结源落地时一并裁决（Attach 升档前检查 s.exiting 拒绝新 attach）；本阶段建议在 11-CONTEXT 补一行登记该衍生面。

---

_Reviewed: 2026-09-04T02:18:54Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
