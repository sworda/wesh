---
phase: 12-per-client
reviewed: 2026-09-04T15:47:16Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - internal/proto/proto.go
  - internal/proto/proto_test.go
  - internal/server/clients.go
  - internal/server/clients_test.go
  - internal/server/export_test.go
  - internal/server/perclient.go
  - internal/server/perclient_test.go
  - internal/server/resize.go
  - internal/server/server.go
  - web/dist/index.html
  - web/src/main.ts
  - web/uat/phase12-dom.mjs
  - web/uat/phase12.mjs
findings:
  critical: 1
  warning: 1
  info: 2
  total: 4
status: issues_found
---

# Phase 12: Code Review Report

**Reviewed:** 2026-09-04T15:47:16Z
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

审查范围 = e8b39c0..HEAD 的 Phase 12 全量变更：Welcome.session 模式位（协议 additive）、per-client RESIZE 直通（服务端 D-06 + 前端 D-07 配对）、ro 放行配对、背压停读/续读 + dwell 看门狗（替代 Phase 11 trySend 失败直踢）、协议层 UAT phase12.mjs 与 jsdom phase12-dom.mjs。

总体评估：**并发工程质量高，但存在一个 per-client resize 的功能性缺陷（CR-01）**。

核实为正确的关键面（对抗性追踪后未发现问题）：

- **dwell 看门狗三防线**（perclient.go）：ReadLoop 闭包串行（pty.ReadLoop 同 goroutine 同步回调）使「武装/续读/逃逸」三路径不可能并发重入；全部返回路径（trySend 成功不武装 / cl.done 逃逸 defer Stop+nil / 续读点 Stop+nil）均先停摆再返回，`pc.dwellTimer` 不存在「未停旧计时器即覆写」的路径；到期回调 hubMu 内身份比对 + cl.done 早退覆盖了「timer 已 fire 但回调等锁」的全部交错序（fire 与续读点争锁的两种顺序均安全）；`var t *time.Timer` 自引用闭包形态正确（赋值先于回调取锁）。
- **notFull 恢复信号量**（clients.go）：cap 1 + drain 内非阻塞发送，滞留 token 至多一次伪唤醒（消费后重试失败继续 select，drain 每次发新 token 无死锁面）；shared 路径零消费者零行为变化。
- **每会话 RESIZE 防抖**（resize.go debouncer + perclient.go）：单一时长源 s.resizeDebounce 防双写；回调锁序（resizeMu 叶锁 → 放锁 → sess.Resize 仅 fdMu）不触 hubMu，锁序三规则成立；teardown 后在途 RESIZE 再武装 → Resize 返 os.ErrClosed 静默，无害有界。
- **Welcome.session additive**：五调用点（server.go 升档 / perclient.go upgradePerClient / clients.go afterDrain + promoteNextLocked / resize.go pushSessionDimsLocked）统一传 s.sessionMode，无双写面；恒序列化契约（无 omitempty）与「缺键 = 旧服务端」前端缺省两侧对齐，proto_test.go 以 map 键存在性锁定。
- **kickSlowConsumerLocked 在 per-client 的复用**：arbiter 零值下 removeMember（nil map delete no-op）/ recalcNow（零值哨兵提前返回）/ promoteNextLocked（owner 恒 nil 不触发）全部天然 no-op，逐行核实成立。
- **ro 放行配对**：服务端 `cl.pc != nil` 直通分支（server.go:1216）与前端 `isRO && sessionMode !== 'per-client'` 闸（main.ts:344）两侧均落地且注释互指。
- **安全/卫生**：无硬编码凭据、无危险函数、无调试残留；UAT 红线（token/pid 不入 detail）经 assertOutputClean 运行时自净断言兜底。
- **dist 一致性**：web/dist/index.html 与 web/src/main.ts 同 commit（4aadd93）提交，bundle 含 session 解析 / `Kp==="per-client"&&$.reset()` / per-client ro 闸三处 Phase 12 标记，构建产物与源一致。

已知中间态（计划内显式登记，非本报告发现项）：per-client 下 --once/--exit-when-empty 永不退出、session_start/session_end 审计空白（均归 Phase 13）；默认 ping 配置下停读客户端 1006 pong_timeout 与 dwell 1013 的竞态（S6 注释实证登记，STATE Blockers，Phase 13 裁决）。

## Critical Issues

### CR-01: per-client 窗口放大后渲染钳制在过时 sessionDims——渲染尺寸与 PTY 实际尺寸分叉（本阶段核心功能缺陷）

**File:** `web/src/main.ts:369-374, 684`（根因）；`internal/server/server.go:1216-1224`、`internal/server/perclient.go:251`（配伍面）
**Issue:** G-05-1 渲染约束 `min(fit, sessionDims)`（main.ts:369-370）依赖「会话尺寸变化 → 服务端推送新 Welcome → sessionDims 刷新」闭环，该闭环只在 shared 模式成立（recalcNow → pushSessionDimsLocked）。Phase 12 的 per-client RESIZE 直通（server.go:1216 分支，刻意零 Welcome 再推送，S2c 断言锁定「零 W 帧」）打破了闭环：`sessionDims` 唯一赋值点是 WELCOME 分支（main.ts:684），值恒为 attach 时 Hello 尺寸（= attach 时 fit）。用户放大窗口后：

- `refit()` 上报**全值 fit 尺寸**（main.ts:374 `sendResize(d.cols, d.rows)`）→ 服务端 PTY 直通变为新尺寸（如 120x50）；
- 前端渲染 `min(120, 100) × min(50, 30)` = **100x30（attach 时的过时值）**——终端永远渲染在 attach 时尺寸，shell/TUI 按 120x50 输出的字节流在 100x30 视口上折行错位、全屏程序布局损坏。缩小方向不受影响（fit < sessionDims 时约束不激活），仅放大触发，且重连前不可恢复。

这直接违反本阶段计划自己的设计声明：12-CONTEXT D-30 以「herdr 场景 ro 移动端转屏/拖窗后**自身 area 渲染尺寸正确**」为 ro 放行的立项理由；研究 ARCHITECTURE §3.4 的「G-05-1 契约在 per-client 退化为恒等式」只在 attach 时刻成立，直通使 PTY 尺寸随 fit 变化后恒等式即破坏。jsdom D2（phase12-dom.mjs:428-453）只断言 RESIZE 帧**已发送**，不观察渲染尺寸；协议层 S2 只断言 PTY 侧 stty——两侧测试对该缺陷均盲。
**Fix:** 前端在 per-client 模式发送 RESIZE 成功后同步 sessionDims，恢复恒等式（服务端补发 Welcome 会违反本阶段「零 W 帧」的 wire 契约，不可取）：

```ts
// main.ts sendResize()，发送成功记账处：
  ws.send(concat(new Uint8Array([RESIZE]), enc.encode(JSON.stringify({ cols, rows }))));
  lastReported = { cols, rows };
  // per-client：会话即本端独占，RESIZE 直通后 PTY 尺寸 = 上报 fit——同步
  // sessionDims 维持 G-05-1 恒等式，防渲染钳在 attach 时的过时尺寸
  // （shared 不动：sessionDims 由服务端 Welcome 推送刷新，客户端不自记）
  if (sessionMode === 'per-client') {
    sessionDims = { cols, rows };
  }
```

该修复使下一次 `refit()` 的 `min(fit, sessionDims)` 回到恒等（fit ≤ fit），渲染随窗口恢复；shared 路径零改动。建议同时补一条 jsdom 断言：per-client 布局突变后 `term.cols/rows` 跟随 fit（D2 现只查帧计数）。

## Warnings

### WR-01: phase12.mjs S5 未隔离 pinger/dwell 竞态——慢 CI 上存在窄假阳窗口

**File:** `web/uat/phase12.mjs:597`
**Issue:** S6 注释（phase12.mjs:657-667）本阶段实证并登记了默认 `--ping-interval=5s`（main.go:228 `pingIntervalDefault`）下的交互：stall 端 writer 持 writeFrameMu 阻塞于满 TCP 时，ping tick 的 writeControl 内层 5s 写超时返回 DeadlineExceeded，pinger（server.go:1336 只认该单一形态）误判为 pong 超时 → 1006 先杀。S5 的 `RawStallClient` 处于同一交互面（pause 停读同时停了自动 pong、writer 同样会 mu 阻塞），但实例未加 `--ping-interval=0`（对比 S6 line 671）：停读窗标称 ~3s，慢 CI 上 B 探针（echoMark 5s 超时上界）+ 30.9MB 管线排空延迟可把「writer mu 阻塞中」窗口拉过首个 ping tick（attach+5s），在 tick+5s=attach+10s 处触发 1006 → S5a/S5b 假阳 FAIL。S5 不测任何保活语义，竞态面应与 S6 同样隔离。16 轮 UAT 全绿说明概率低，属测试可靠性风险非必然失败。
**Fix:** S5 startWesh 实参加 `--ping-interval=0`（与 S6 同款隔离；S5 全部断言——序号连续性、连接存活、B 端无扰——不依赖保活，隔离零弱化）：

```js
const inst = await startWesh(['--session-mode=per-client', '--writable', '--ping-interval=0', '--', 'sh']);
```

## Info

### IN-01: `skip` 帮助函数定义后零调用（两 UAT 脚本）

**File:** `web/uat/phase12.mjs:64-68`、`web/uat/phase12-dom.mjs:64-68`
**Issue:** 平台豁免记录通道 `skip(id, name, reason)` 在两脚本中均定义且计入汇总（`skipped` 过滤），但无任何调用点——phase12.mjs 无平台分支（S5/S6 的 400 万洪水与 dwell 为 Linux 侧设计，jsdom 侧无豁免场景），死代码会给未来读者「存在豁免路径」的误导。
**Fix:** 删除两处 `skip` 定义，或在本阶段无豁免场景处加一行注释锚定「预留、暂无调用点」。

### IN-02: phase12-dom.mjs 携带的 phase06-dom 夹具面零消费

**File:** `web/uat/phase12-dom.mjs:211-221, 283-287`
**Issue:** `holdAttachFetchN`/`releaseHeldFetch`（fetch 挂起注入）与 `staleClose`（代际守卫二次驱动）从 phase06-dom.mjs 逐字复用，但 D1/D2/D3 三场景均未使用（无 opts.holdAttachFetchN 传入、无 staleClose 调用）——死夹具面随复制漂入。
**Fix:** 若 Phase 14 参数化收编时需要可保留并注释「暂无消费方」；否则删除以收窄夹具面。

---

_Reviewed: 2026-09-04T15:47:16Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
