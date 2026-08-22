---
phase: 05-multi-client
reviewed: 2026-08-22T11:33:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - internal/server/resize.go
  - internal/server/clients.go
  - internal/server/resize_test.go
  - internal/server/clients_test.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 5: Code Review Report（增量复审：05-13 WR-01/WR-02 gap-closure 修复）

**Reviewed:** 2026-08-22T11:33:00Z
**Depth:** standard
**Files Reviewed:** 4（diff 027ec63..4936f1c，+301/-6）
**Status:** issues_found

> 本轮取代 2026-08-22T05:16Z 增量评审（同路径 05-REVIEW.md，已覆盖）。上一轮的 WR-01/WR-02 是本轮的审查对象本身——本轮验证其逐字补丁是否如实落地并闭合；上一轮的 IN-01..IN-04 仍为挂账打磨项，按既定裁决不在本轮范围内，未重复报告。

## Summary

本轮增量覆盖 05-13 gap-closure plan 的两个修复提交（74d1bff WR-01、4936f1c WR-02）。审查方式：逐字比对 05-REVIEW 补丁文本与落地 diff、通读四文件全貌、交叉核对注释中的每个机制性断言与行号引用、独立运行两个新回归测试（`-race -count=1` 全 PASS，TestPushSessionDimsKickRecalc 第 9 轮迭代命中 B-first 危险序），并核查修复与既有测试面（TestGlobalCredit 字节精确断言、mergeBatch 控制帧纪律）的交互。

**补丁保真度判定（逐 WR）：**

- **WR-01（复检中止 stale 扇出）——逐字一致。** resize.go:173-179 的 `if s.arbiter.last != target { return }` 复检精确位于 trySend 失败分支内、`kickOrCreditLocked` 调用行之后（未移出循环、未挂成功路径），两行补丁注释逐字相同。安全性注释改写覆盖 plan 要求的四要素：(a) range 期间 map delete 的 Go spec 安全性保留；(b) 主论证落在真实可达的 removeMember→嵌套 recalcNow 路径（被踢参与集成员持某轴最小值时仲裁改变，嵌套推送已送达 W(T2)，外层 T1 stale）；(c) 踢出不改仲裁或走信用路径时 last==target、外层正确继续——经推演成立（含 0 人零值哨兵边界：嵌套 recalcNow 提前返回、last 保持 T1、PTY 物理尺寸即 T1，继续投递 W(T1) 正确）；(d) 每次踢出永久移除一端保证嵌套有界终止。promoteNextLocked 不可达性压缩为从句且论断准确（owner 模式唯一可写端恒走信用分支不被踢、all 模式 owner 恒 nil）。
- **WR-02（afterDrain 开门补发，option (a)）——逐字一致。** 补发段精确插入 `c.creditBlocked = false` 与 `gateTransitions++`（原位未动）之后、`hubCond.Broadcast()` 之前（clients.go:459-467）；`_ =` trySend 形态保持；prefs 按 c.mode 选档与 pushSessionDimsLocked 的写法逐字同构（D-13 双档不漂移）。补发有序性注释的互斥归因已按 plan-check 修订落在「afterDrain 全程持有 hubMu + outbox FIFO」，未使用「门仍闭合」旧归因。kickOrCreditLocked 的「触发帧不丢」承诺已收窄为首帧暂存语义（clients.go:390-393），resize.go:148-151 镜像收窄并互相指路，两处注释与实现一致。

**新缺陷扫描结论：** 复检不会跳过任何必需推送（仲裁未变/信用路径下 last==target，循环正确继续）；arbiter.last 的唯一写点是 recalcNow（resize.go:138），写后必接推送，故 `last != target` ⟹ 嵌套推送已完成新值扇出，复检逻辑充分；afterDrain 补发在 sessionDimsLocked 回落 spawn 尺寸的场景下语义正确且无害（生产上 rw 端 attach 即参与仲裁，last 非零，该路径实际不可达）；未引入新锁获取，hubMu > outbox.mu 锁序保持；两修复可组合（嵌套推送期间被守卫跳过的 blocked 端由 afterDrain 补发收敛）。测试夹具无 goroutine/fd 泄漏（httptest handler 读至出错即返、`Session.Close` 对 nil Cmd 安全——io.go:65-76 仅触及 Master、踢出 goroutine 在真实 conn 上自行终结、每轮迭代 pty/conn 双闭）。TestGlobalCredit 的字节精确断言结构性免疫补发帧（readUntilError 仅累积 proto.Output 帧，slowclient_test.go:48）。

**项目规则符合性：** 无新导出符号、proto.go/前端/UAT 脚本零改动、无新帧类型；中文注释风格与文件惯例一致；无过度设计；executor 自报的 1 项 deviation（registerLocked 接收者修正为 s.registry.registerLocked）经核实为散文简写→真实 API 的机械调和，准确无隐瞒。

发现 1 个 WARNING（新写安全注释内的跨文件行号引用已被同轮后一个提交挪位）与 1 个 INFO（补发数学保证注释的容量下界表述精度），无 Critical。

## 上轮发现闭合判定

- **WR-01（pushSessionDimsLocked 循环内踢出后投递过时尺寸帧）：完全闭合。** 逐字补丁 + 注释改写 + TestPushSessionDimsKickRecalc 白盒回归（修复前 B-first 态末帧 60x24 ≠ arbiter.last 必败的测试牙齿、32 轮未命中即 FAIL 不放行空转绿）三要素全部落地。
- **WR-02（creditBlocked 端尺寸推送帧被守卫静默丢弃且无补发）：完全闭合。** option (a) 逐字补丁 + 双注释收窄 + TestAfterDrainResendsDims 两子测（守卫不覆写暂存语义锁 + 补发收敛的帧序/当前尺寸/rw-ro 选档区分度断言）全部落地。

**判定：上一轮的 WR-01/WR-02 由本 diff 完全解决（fully resolved）；phase 评审环可闭合。** 下述 WR-03/IN-05 为本轮新引入的注释精度项，均不触及行为，可作为低成本打磨跟进，不构成闭合阻塞。

## Warnings

### WR-03: resize.go 新安全注释的跨文件行号引用「clients.go:479-480」已被同轮 WR-02 提交挪位失效

**File:** `internal/server/resize.go:155`（指向 `internal/server/clients.go:501-502`）
**Issue:** 复检安全性注释写着「循环内踢出经 clients.go:479-480 removeMember → 嵌套 recalcNow 真实可达」。该行号引自 05-13-PLAN（针对 diff 基线 027ec63 正确），在 WR-01 提交（74d1bff）写入时仍正确；但同轮 WR-02 提交（4936f1c）在 clients.go 该点之前净增 22 行（kickOrCreditLocked 注释 +3、afterDrain 文档 +1/+9、补发代码 +9），removeMember/recalcNow 实际已下移至 **clients.go:501-502**。当前 clients.go:479-480 落在 kickSlowConsumerLocked 文档注释内（`//` 空行与「1013 关闭帧可达性不变量」段），按引用导航的读者会定位到无关注释而非代码。本项目注释惯例依赖精确行号互指（proto.go:165-182、close.go:87-89、io_test.go:24-25 等），且本轮主题即注释真实性（两条上轮 WR 均涉注释过度承诺），新写的安全论证携带一个已失效的坐标属于同类缺陷。同注释区另一引用「onChunk → kickOrCreditLocked（clients.go:354-358）」经核对仍然准确（编辑点均在其后）。
**Fix:** 把 resize.go:155 的 `clients.go:479-480` 改为 `clients.go:501-502`。后续涉及跨文件行号引用的多提交 plan，建议收尾时重核一遍引用坐标（行号漂移是同轮多提交的天然副产物）。

## Info

### IN-05: afterDrain 补发「入队必成」数学保证注释的容量下界未计入补发帧自身

**File:** `internal/server/clients.go:439-440`（联动既有注释 :429-432）
**Issue:** 新注释称「入队必成沿用重投的数学保证（余量 ≥ cap/2+1 ≫ ~100B Welcome）」。严格推导：重投后 bytes ≤ (cap/2 − 1) + (32KiB + 1)，补发 ~100B Welcome 必成的充要下界约为 cap ≥ 64KiB + 200（比注释记载的「重投保证 cap ≥ 64KiB」略高）。在 cap 恰好取测试覆写下界 64KiB、且暂存帧恰为最大帧、cur 恰贴近半水位的叠加角情形下，`_ =` 补发会静默失败。实际影响可忽略：生产默认 cap 512KiB 余量 ~224KiB，失败形态仅是少补一帧尺寸 Welcome、下次尺寸事件自愈（与 WR-02 修复前同价），且 plan 已显式裁决失败属配置错误不兜底、禁止加错误处理分支。仅注释精度与下界记载从严登记。
**Fix:** 如需打磨，把该句修为「余量 ≥ cap/2+1 − 最大暂存帧 ≈ cap/2 − 32KiB（默认 cap 512KiB 下 ~224KiB ≫ ~100B Welcome；cap 覆写下界对补发帧须再留 ~100B 余量）」，或在 64KiB 下界句后补一句「补发帧另需 ~100B，合并保证下界 64KiB+200」。

---

_Reviewed: 2026-08-22T11:33:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
