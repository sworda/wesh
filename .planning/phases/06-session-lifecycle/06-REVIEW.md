---
phase: 06-session-lifecycle
reviewed: 2026-08-23T09:45:00Z
depth: standard
files_reviewed: 18
files_reviewed_list:
  - internal/proto/proto.go
  - internal/proto/proto_test.go
  - internal/server/server.go
  - internal/server/clients.go
  - internal/server/exit_test.go
  - internal/server/exitmsg_test.go
  - internal/server/emptyexit_test.go
  - internal/pty/signal_linux.go
  - internal/pty/signal_darwin.go
  - internal/pty/io_test.go
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - web/src/main.ts
  - web/src/lib/reconnect.ts
  - web/src/lib/reconnect.test.ts
  - web/uat/phase06.mjs
  - web/uat/phase06-dom.mjs
  - internal/server/e2e_test.go
findings:
  critical: 1
  warning: 2
  info: 4
  total: 7
status: fixes_applied
fixed: [CR-01, WR-01, WR-02]
fixed_at: 2026-08-23T10:33:09Z
fix_commits:
  CR-01: 010a3df
  WR-01: 95ab12a
  WR-02: 9d5e067
fix_report: .planning/phases/06-session-lifecycle/06-REVIEW-FIX.md
---

# Phase 6: Code Review Report

**Reviewed:** 2026-08-23T09:45:00Z
**Depth:** standard（逐文件核读 + 语言专项检查 + 关键调用链跨文件追踪）
**Files Reviewed:** 18（另交叉核读 internal/pty/spawn.go、internal/pty/io.go 作为 SignalHangup 前提验证的依赖；README.md 为文档、web/dist/index.html 为构建产物，排除）
**Status:** issues_found

## Summary

Phase 6 改动覆盖四个面：EXIT 帧契约与 lifecycle 写序安全广播（06-01）、SIGHUP 空触发断开退出机制（06-02）、前端重连状态机（06-03/05）、CLI 投影与协议层 UAT（06-04/06/07）。

服务端 Go 侧整体质量高：hubMu 单锁纪律保持（maybeExitWhenEmptyLocked 三守卫、exiting 门置位先于注册表快照 server.go:1096<1098、宽限计时器启停全在 hubMu 内且回调复查）、SignalHangup 双平台同签名且 setsid 前提经 spawn.go 调用链核实成立、EXIT→1000 写序靠同 goroutine 同步 Write→Close + 库帧级串行化成立（writer 写不持 hubMu，无交叉死锁）、CLI 侧 IsBoolFlag 三形态与显式设置位冲突矩阵正确（d<0 负值闸、--once 展开只填未显式位、validateStartup 纯函数先于资源占用）。协议字节契约两侧对齐（'X'/0x58，proto.go:35 ↔ main.ts:29 ↔ phase06.mjs:28）。实测验证：`go build/vet` 退出 0，`go test -race -count=1` proto/pty/cmd/wesh 与 server 包 phase-06 相关测试（TestExit*/TestSignal*/TestClientCount）全绿，`node --test web/src/lib/*.test.ts` 19/19。

主要问题在前端重连状态机：**06-03 deviation #1 只堵住了双在飞 attempt 的「迟到失败」通道，「迟到成功」通道无守卫**——`connect()` 在 fetch 解析与 `new WebSocket` 之间没有任何代际/陈旧性检查，旧 attempt 迟到成功会直接踩掉健康连接句柄，在 owner 模式下把用户降级成只读并遗留幽灵连接（CR-01）。此外两项测试层 Warning（D7 硬编码 2s 期限与子进程 sleep 2 耦合的 flake 面；UAT 异常通道绕过 assertOutputClean 自净断言）。

## Critical Issues

### CR-01: connect() 迟到成功的旧 attempt 无守卫踩掉健康连接——owner 模式下用户被降级只读 + 幽灵连接驻留槽位

**File:** `web/src/main.ts:578`（另涉 502-513 fetch 段、561-575 catch 段、229-235 runAttempt、922-925 online 监听）

**Issue:** 06-03 deviation #1 认识到「online 事件/手动点击可在前一 attempt 的 fetch 飞行中再启 attempt」（D-04 既定双在飞形态），并给 fetch **失败**通道补了 `if (welcomeDone) return;` 代际守卫（main.ts:572）。但 fetch **成功**通道没有任何等价守卫：`resp.ok` 取到 ticket 后径直执行 `ws = new WebSocket(...)`（main.ts:578），无条件覆盖模块级 `ws` 句柄。四个 socket handler 的 `sock !== ws` 代际守卫（586/785/804/822）只能让被踩连接的事件静默空转，**不能阻止踩占本身**，且全文件没有任何 `.close()` 调用（grep 实证）——被踩的旧 socket 永不关闭，浏览器协议栈自动应答服务端 ping，服务端 pinger/pongTimeout 永不触发，幽灵连接在页面存活期内一直占用注册表槽位与 owner 身份。

**失败场景 A（迟到成功）：** 重连循环中 attempt A 的 fetch 悬挂（网络闪断/服务端黑洞）→ 用户点「Reconnect now」或 online 事件触发 runAttempt → attempt B 快速完成：attach → WELCOME → stopReconnect()，健康会话建立在 B 上（owner 空位，B 成 owner）→ 悬挂的 fetch A 经 TCP 重传迟到 resolve → 无守卫走到 578 行踩占 `ws` → A attach 时 owner 在位，decideModeLocked 按矩阵降级 ro（clients.go:300-302 「rw ticket × owner 在位 → ModeRO」）→ A 的 WELCOME{mode:"ro"} 驱动 UI 置 [ro] + disableStdin。**用户眼睁睁看着「重连成功」却失去写权限**；幽灵 B 以 owner 身份永久驻留（浏览器透明回 pong），同时占用一个 max-clients 槽位。`--once`（maxClients=1）下更糟：A 的 WS 升级撞 ③位 503 闸 → onclose 落 `!opened` 分支 → 「Unable to connect」面板直接覆盖健康会话的 UI。

**失败场景 B（快速双击 Reconnect now，无需任何网络病理）：** 两次快速点击 → runAttempt×2 → 两条并发 connect() 链；先 resolve 的链完成 socket 创建与 attach（成 owner），后 resolve 的链踩占句柄并以 ro 身份接回——同一 ro 降级 + 幽灵结果，触发只需用户习惯性双击。

**06-05 的 D1-D8 场景均未覆盖此面**（D6 只测 stale onclose 事件方向，无双在飞 fetch 迟到成功场景）。

**Fix:** 给 connect() 加单调代际序号，fetch 解析后、建 socket 前复查，陈旧链直接丢弃返回：

```ts
let connectGen = 0;
async function connect(): Promise<void> {
  const gen = ++connectGen;
  // ... 既有重置与 fetch ...
  const resp = await fetch(/* ... */);
  // ... 既有状态码分支 ...
  // 代际复查：本链已被更新的 connect() 取代（双击/online 竞速/迟到成功），
  // 或健康会话已建立（welcomeDone）——不得再造 socket 踩占句柄
  if (gen !== connectGen || welcomeDone) return;
  ws = new WebSocket(/* ... */);
}
```

（`welcomeDone` 检查覆盖场景 A——健康会话已在时旧链必须弃票；`gen` 检查覆盖场景 B——后发起的链是最新用户意图，唯一存活。注意不可用 `ws?.close()` 反向修补：场景 A 中当前 `ws` 正是健康会话 B，关闭它会把好连接杀掉。）修补后建议在 phase06-dom.mjs 补「双在飞 attempt 迟到成功不踩占」场景。

## Warnings

### WR-01: D7 会话建立 waitFor 硬编码 2000ms 与子进程 `sleep 2` 耦合——慢机器上假性 FAIL

**File:** `web/uat/phase06-dom.mjs:491`（配合 484 行 spawn `sh -c 'sleep 2; exit 7'`）

**Issue:** D7 用 `waitFor(() => ctx.bu.on === 1, ..., 2000)` 等 WELCOME 处理完成，期限恰好等于子进程自杀时点。该等待**并不需要**抢在子进程退出前完成——WS 帧按序到达（WELCOME → EXIT → close 1000），即使 WELCOME 处理落在 exit 之后，Session ended 面板链依然正确。但 waitFor 到期即 throw → 场景异常计 FAIL。jsdom 加载 500KB bundle + eval + WS 握手在本机实测约 0.5-1s，负载高的机器上逼近 2s 即假性失败（产品行为完全正确）。06-05 SUMMARY 自报 30/30 是在空闲机器上取得的，余量真实消耗过半。

**Fix:** 会话建立等待改用默认 5000ms 上限（与 waitReady 一致）：

```js
await waitFor(() => ctx.bu.on === 1, '会话建立（WELCOME 处理完成，beforeunload 注册）', 5000);
```

帧序天然保证语义，期限放宽不削弱任何断言面（D7a 逐字文案、D7b 进程退出码、D7c 零新连接均不受影响）。

### WR-02: UAT 异常通道绕过 assertOutputClean 自净——启动超时 reject 原样回显 argv（含 --credential 值）

**File:** `web/uat/phase06.mjs:85`（配合 487 行场景异常打印与 S1:193/S3:276 的 `--credential UAT_CREDENTIAL` 传参）

**Issue:** 06-05/06-06 把 token 红线从注释纪律升级为 `assertOutputClean()` 运行时自证，但其断言面只覆盖 `emittedDetails`（check/skip 的 detail 数组）。场景异常通道 `console.log('FAIL 场景异常: ' + e.message)`（487 行）不在扫描范围内，而 startWesh 的启动超时 reject 消息为 `` `wesh 启动超时: ${args.join(' ')}; stderr=${stderr}` ``（85 行）——S1/S3 的 args 含 `--credential uat:uat-pass-x9`，一旦启动超时，凭据值明文进入控制台/CI 日志，正是红线要防的通道。当前值是仓库内测试夹具常量（真实暴露为零），但该形态是逐字复用链（phase02-06 全部同款 85 行形态），phase06.mjs 在自称更强红线 posture 的同时继承了未覆盖面；若日后凭据改从环境变量注入，泄漏即成事实。

**Fix:** reject 消息脱敏（argv 中 `--credential` 后随值替换为 `<redacted>`，或只打 flag 名不打值），或把异常消息也纳入 emittedDetails 让 assertOutputClean 能扫到。

## Info

### IN-01: `shouldReconnect` 导出纯函数生产侧死代码——测试锁定的是分派不消费的谓词

**File:** `web/src/lib/reconnect.ts:13-15`（消费缺位：web/src/main.ts:839/891-895）

**Issue:** `shouldReconnect(code) = code === 1006` 有 node --test 正反断言锁定（reconnect.test.ts:16-22），但主分派按 plan 字面取 `case 1006:` 与 `ev.code === 1006` 字面形态，`shouldReconnect` 全仓库唯一引用是其自身测试（grep 实证）。测试给「1006 唯一触发谓词已钉死」的假象，实际生产分派漂移（如有人误改 case 值）时测试照绿。06-03 SUMMARY 已登记此为 plan 字面调和，属知情决策，但死导出面应收口。

**Fix:** 二选一——main.ts 分派改用 `shouldReconnect(ev.code)`（单一事实源，测试真正锁到生产路径），或删除导出只留 backoffMs。

### IN-02: `--exit-when-empty=false` 被拒为「invalid duration」——可选值 flag 无显式否定形态

**File:** `cmd/wesh/main.go:90-102`

**Issue:** IsBoolFlag 惯例下 `Set("false")` 落入 `time.ParseDuration("false")` 报错，用户得到 `invalid --exit-when-empty duration "false": must be a non-negative duration`——对习惯性写 `--flag=false` 显式否定的用户是误导性文案（fail-closed 方向正确，无安全面）。三形态契约（不写/裸写/=duration）本身是锁定设计，此为文案层粗糙点。

**Fix:** Set 内加 `if s == "false" { v.set = false; v.grace = 0; return nil }` 显式否定分支，或在 help 文案注明「false 非法，不写即不开启」。

### IN-03: exiting 门置位晚于 Drain(200ms)——窗口内 detach 致空产生伪 exit_when_empty 事件行与对已收割 pgid 的 SIGHUP

**File:** `internal/server/server.go:1072-1096`（lifecycle 内 Drain→close(inputDone)→组帧→hubMu 段才置 exiting）配合 `internal/pty/signal_linux.go:16`

**Issue:** `s.exiting = true` 在 hubMu 快照段（1096 行），距 `sess.Wait()` 返回（1066 行）隔着 Drain 的 200ms 上限与组帧。此窗口内最后一个客户端 detach 致空时 maybeExitWhenEmptyLocked 的三守卫全部通过（exiting 尚 false）→ grace=0 路径打伪 `exit_when_empty` 事件行并向已收割子进程的 pgid 发 SIGHUP。正常后果为零（ESRCH 静默、exitf 由 termOnce 收口、真实退出码不被翻转——子进程已死），残留面有二：(a) stderr 出现一行语义误导日志（会话实为自然终结）；(b) 理论性 pid 复用危害——内核在该 ~200ms 窗口把同值 pid 分给新进程组长的概率微乎其微（需 pid 回绕恰好命中），命中则误伤无关进程组。06-02 的 TestExitWhenEmptyTimerAfterLifecycle/LifecycleGate 覆盖的是计时器晚于 lifecycle 与广播期 detach 两形态，未覆盖 Drain 窗口形态。

**Fix:** 把 `s.exiting = true` 前移到 `s.sess.Wait()` 返回后立即置位（需取 hubMu——exitEmptyTimer 回调同锁复查语义不变），窗口缩至微秒级；或接受现状并在 maybeExitWhenEmptyLocked 注释登记该窗口。

### IN-04: 断开退出仅发 SIGHUP 无升级路径——trap/忽略 SIGHUP 的子进程使 --once/--exit-when-empty 永久不生效

**File:** `internal/pty/signal_linux.go:15-17`、`internal/pty/signal_darwin.go:16-18`（消费点 internal/server/clients.go:736-740/755-757）

**Issue:** 收口机制 = 单次 SIGHUP 进程组，无 SIGTERM/SIGKILL 升级、无重发。子进程 `trap '' HUP`（或按惯例忽略 SIGHUP 的守护型进程）时：信号被吞 → lifecycle 永不触发 → wesh 进程在零客户端状态下永久存活，SESS-01/02 的核心承诺静默落空，且无日志可辨（logEvent 已打 exit_when_empty，表象是「已触发但未退出」）。此为 D-13 锁定形态（tmux 同为 SIGHUP-only 先例），非实现错误，但属部署面真实限制——用 `nohup`/trap 风格命令作 argv 的用户会踩到。

**Fix:** 现状可接受则 README --exit-when-empty 节补一句「子进程须未忽略 SIGHUP」；或 Phase 7+ 评估宽限后 SIGKILL 升级（动 D-13 需重新裁决）。

---

_Reviewed: 2026-08-23T09:45:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
