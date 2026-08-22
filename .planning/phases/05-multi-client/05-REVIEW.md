---
phase: 05-multi-client
reviewed: 2026-08-22T05:16:37Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - internal/proto/proto.go
  - internal/proto/proto_test.go
  - internal/pty/spawn.go
  - internal/server/resize.go
  - internal/server/server.go
  - internal/server/clients.go
  - internal/server/clients_test.go
  - internal/server/multi_test.go
  - internal/server/resize_arb_test.go
  - internal/server/auth.go
  - internal/server/sharetoken_test.go
  - web/src/main.ts
  - web/uat/phase05.mjs
  - web/uat/phase05-dom.mjs
  - web/uat/phase05-dims.mjs
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 5: Code Review Report（增量复审：G-05-1 会话尺寸下发 + G-05-7 错 token→401）

**Reviewed:** 2026-08-22T05:16:37Z
**Depth:** standard
**Files Reviewed:** 15（diff_base 9e01d75 起的增量 + 全量上下文）
**Status:** issues_found

## Summary

本次增量覆盖两组 gap-closure 改动：**G-05-1**（Welcome 三通道恒携会话 cols/rows + 前端 refit() 视口约束渲染）与 **G-05-7**（无认证模式携错 token → 401，统一 authRequiredBody）。本报告取代 2026-08-21 全量评审同路径文件（05-REVIEW.md），聚焦增量 delta。

已验证的健全面：

- **编译与测试**：`go vet ./...` 干净；`go test ./internal/...` 全绿（33s）；关键多客户端/仲裁用例（TestWelcomeSessionDims/TestResizeArbitration/TestAllPolicy/TestSuccession 等）`-race` 通过；`tsc --noEmit` 通过。
- **协议一致性**：WelcomeFrame 4 参签名全部 5 个调用点（resize.go:163 / server.go:730 / clients.go:540 + 两测试文件）同步更新无遗漏；前后端 Welcome schema 注释互指一致；cols/rows 恒序列化有 map 级键存在性回归锁（proto_test.go:120-135）。
- **锁序纪律**：升档重排后 addMember/recalcNow 前移至 Welcome 组帧前，hubMu > sess.fdMu（recalcNow→Resize）与 hubMu > outbox.mu（pushSessionDimsLocked→trySend）均未反序；Welcome 恒首帧不变量（入队先于 registerLocked）保持；attach 者自身不被自己的 recalc 推送触达（尚未注册）论证成立。
- **G-05-7**：shareResult 三态拆分正确；无认证模式 shareInvalid→401 无挑战头、shareAbsent→404 探测信号不变；凭据模式错/无 token 401 同文同码无 oracle（sharetoken_test.go:340 逐字节断言锁定）；主体恢复供委托链重读逻辑未受签名变更影响。
- **前端 refit() 收编**：term.onResize 订阅已拆除且无残留旁路（全文件无第二处 term.resize 调用方）；Hello 首尺寸/lastReported 同步、ro 期 isRO 门不记账、升格后纠正链、prefs 重放幂等（queryKeys 跳过 + osc52Loaded 门闩）逐路径推演成立。

发现 2 个 WARNING（均在 G-05-1 尺寸推送与既有背压/踢出机制的交界面上）与 4 个 INFO，无 Critical。

## Warnings

### WR-01: pushSessionDimsLocked 循环内踢出触发嵌套重算后，继续投递已过时的旧尺寸帧

**File:** `internal/server/resize.go:156-168`（联动 `internal/server/clients.go:479-480`）
**Issue:** `pushSessionDimsLocked(target)` 的 `target` 是外层 `recalcNow` 捕获的形参。循环内若某客户端 outbox 满被踢（`kickOrCreditLocked` → `kickSlowConsumerLocked` → `removeMember(c)` → 嵌套 `recalcNow`），且被踢者是仲裁参与集成员、其移除改变仲裁结果（纯 ro 会话全部 ro 端均为成员；all 模式被踢 rw 端亦为成员——min-rect 的某轴最小值恰由被踢者持有时），则嵌套 recalcNow 算出新目标 T2 ≠ T1：PTY 已 Resize 到 T2，嵌套推送把 W(T2) 送达**全部**留存客户端（含外层循环尚未访问者）。外层循环恢复后仍用捕获的 T1 组帧，继续向尚未访问的客户端投递 W(T1)。

线上时序后果（map 遍历序随机，约半数留存客户端命中）：先收 W(T2) 后收 W(T1)——前端 last-write-wins，`sessionDims` 终值 = 过期 T1，而 PTY 实际尺寸 = T2。受影响客户端按错误约束渲染，叠写/错渲即 G-05-1 要消除的缺陷类，直到下一次尺寸事件才自愈。

可达路径两条：all 模式 rw 端运行期 RESIZE 防抖到期重算的推送循环内踢出慢成员；纯 ro/all 模式 attach/detach 即时重算的推送循环内踢出慢成员。

触发条件较窄（尺寸变化推送恰好撞上慢消费者踢出、且踢出改变仲裁结果），但 resize.go:153-155 的安全性注释只论证了「踢出触发 promoteNextLocked → 嵌套推送」这条**实际不可达**的路径（owner 模式唯一可写端恒走信用不被踢、all 模式 owner 恒 nil——promoteNextLocked 从推送循环内不可达），漏掉了真实可达的 removeMember→嵌套 recalcNow 路径，注释的正确性论证不覆盖本缺陷。

**Fix:** 踢出/信用判定返回后复检权威性——嵌套推送已对全部留存客户端送达最新尺寸，stale 外层扇出可直接中止：

```go
func (s *Server) pushSessionDimsLocked(target dims) {
	for c := range s.registry.set {
		mode := c.mode.Load().(string)
		prefs := s.clientPrefsRO
		if mode == proto.ModeRW {
			prefs = s.clientPrefsRW
		}
		frame := proto.WelcomeFrame(mode, prefs, target.cols, target.rows)
		if !c.outbox.trySend(frame) {
			s.kickOrCreditLocked(c, frame)
			// 踢出可能经 removeMember→嵌套 recalcNow 把 arbiter.last 推进到更新值，
			// 嵌套推送已向全部留存客户端送达新值——本循环的 target 已过时，中止防旧值反超。
			if s.arbiter.last != target {
				return
			}
		}
	}
}
```

（被踢但仲裁结果不变时嵌套 recalcNow 提前返回、last 不变，外层循环正确继续；信用路径无成员变动同样继续。）同时修正 resize.go:153-155 注释，把嵌套路径论证从 promoteNextLocked 换成 removeMember→recalcNow。

### WR-02: 已处于 creditBlocked 的客户端收不到尺寸推送帧，且恢复后无补发

**File:** `internal/server/resize.go:164-166` + `internal/server/clients.go:409-415, 438-443`
**Issue:** 全局信用门闭合期间（全体可写端均 blocked，hub 停读 PTY），C→S 方向的 RESIZE 不受门控仍可到达 → 防抖到期 `recalcNow` → `pushSessionDimsLocked` 向 blocked 客户端 trySend ~100B Welcome——若其 outbox 余量不足（被 block 后、writer 下次 drain 前 bytes 仍贴近 cap）则失败 → `kickOrCreditLocked` 的 `if !c.creditBlocked` 守卫跳过暂存（creditPending 已被首个触发帧占），**该尺寸推送帧被静默丢弃**。`afterDrain` 半水位恢复只重投 creditPending 的旧 OUTPUT 帧，不补发错过的会话尺寸——该端 `sessionDims` 一直过期到下一次成功推送或重连。

resize.go:148-150 注释声称「trySend 失败走 kickOrCreditLocked……触发帧不丢——信用路径暂存 creditPending 既有形态」，对已 blocked 情形不成立，注释过度承诺。窗口较窄（仅 blocked 至下次 drain 之间）、受影响端本已处于严重滞后态，影响为约束渲染短暂失真（非崩溃非安全面），故定 WARNING。

**Fix:** 两选一——(a) `afterDrain` 清位开门时向该端补发一帧当前 `sessionDimsLocked()` 的 Welcome（~100B，恢复路径低频，收敛性正解）：

```go
// afterDrain：c.creditBlocked = false 之后、Broadcast 之前
	sd := s.sessionDimsLocked()
	mode := c.mode.Load().(string)
	prefs := s.clientPrefsRO
	if mode == proto.ModeRW {
		prefs = s.clientPrefsRW
	}
	_ = c.outbox.trySend(proto.WelcomeFrame(mode, prefs, sd.cols, sd.rows)) // 补发阻塞期错过的尺寸推送
```

(b) 退而求其次：修正注释把「触发帧不丢」收窄为「首帧暂存、阻塞期后续尺寸推送可丢、下次尺寸事件自愈」，并在 deferred-items 挂账。

## Info

### IN-01: UAT 检查点 ID 复用（S8a/S9a 各出现两次）

**File:** `web/uat/phase05.mjs:489,502`（S8a）；`:524,544`（S9a）
**Issue:** 前置条件门（vim 首绘就绪 / B 初始为 ro）与真正的行为断言共用同一检查点 ID。通过路径下 results 出现两条同名记录，汇总计数与归因含糊；失败路径下前置 FAIL 与断言 FAIL 无法区分。
**Fix:** 前置门改用独立 ID（如 S8p/S9p，或顺延 S8b/S9b），或前置失败直接抛场景异常（`throw`，计入「场景异常」栏）而非 check。

### IN-02: stall 夹具帧长注释 off-by-one

**File:** `web/uat/phase05.mjs:162`
**Issue:** 注释称 "'H'+JSON 载荷 = 41 字节"——helloFrame() 实为 1 字节类型 + 41 字节 JSON = **42** 字节。`<126 单帧短形` 结论不受影响，纯注释数值漂移。
**Fix:** 改为 "42 字节（1 类型字节 + 41 字节 JSON）"。

### IN-03: pushSessionDimsLocked 的 `c.mode.Load().(string)` 无守卫类型断言

**File:** `internal/server/resize.go:158`
**Issue:** 当前不可 panic（`mode.Store` 先于 `registerLocked`，注册表内客户端恒存 string），但同生态位 `allWritableBlockedLocked`（clients.go:369）用的是 `!= proto.ModeRW` 接口比较、零断言。若未来出现零值 client 入注册表的测试/重构路径，此处 panic 发生在 hubMu 持有内，会炸垮整个服务端。
**Fix:** 与既有先例统一为比较形态，或 `mode, _ := c.mode.Load().(string)`（零值按 ro 档处理）。

### IN-04: 无认证模式 401 body 提及不存在的 "operator credentials"

**File:** `internal/server/server.go:352` + `internal/server/auth.go:72-74`
**Issue:** G-05-7 无认证模式携错 token 返回的 authRequiredBody 写着 "Enter the operator credentials to continue"，但该模式本无凭据可输。前端流程不受影响（携 token 401 被 C-3「Invalid share link」面板截获，body 永不展示），仅影响 curl 等原始 HTTP 客户端的文案准确性。同文无 oracle 是有意裁决，登记备查。
**Fix:** 如需打磨，无认证分支可用独立文案（如 "share link invalid or expired"）——两模式 body 不同不构成本模式内枚举 oracle；或维持现状并在注释明示该取舍。

---

_Reviewed: 2026-08-22T05:16:37Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
