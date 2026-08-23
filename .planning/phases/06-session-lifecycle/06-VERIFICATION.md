---
phase: 06-session-lifecycle
verified: 2026-08-23T10:58:00Z
status: human_needed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "断网 30s 恢复自动重连观感（06-UAT.md Test 1）：浏览器打开会话 → 断网（飞行模式/拔线）→ 观察 Reconnecting 面板倒计时 → 恢复网络 → 预期 5s 内自动接回原会话（echo $$ 同进程判据）"
    expected: "Reconnecting 面板出现并倒计时；恢复后自动接回同一 PTY 进程（pid 相等）"
    why_human: "真实 OS 断网栈与浏览器原生 online/offline 事件时序——headless 硬约束（CODEBUDDY.md 平台原生行为豁免条款），任何自动化结构性不可测；自动化等价面已覆盖：phase06.mjs S6（真实 TCP 断连重接同一 PTY，pidPre==pidPost）+ phase06-dom.mjs D1/D4/D8（合成事件驱动同一状态机）"
  - test: "重连成功清屏与程序重绘观感（06-UAT.md Test 2）：重连后观察终端画面"
    expected: "重连成功清屏后全屏程序秒级重绘干净画面；行内 shell 历史由 tmux/herdr 覆盖"
    why_human: "像素视觉观感——headless 豁免；自动化等价面：phase06-dom.mjs D1h（term.clear() 可观测——断开前写入文本重连后从 DOM 消失）"
  - test: "Reconnect now 手动跳过（06-UAT.md Test 3）：Reconnecting 面板等待期点击 hint 链接"
    expected: "倒计时未完即发起新连接，循环不终止直至接回"
    why_human: "真实浏览器点击 UX——headless 豁免；自动化等价面：phase06-dom.mjs D5（800ms 容差窗内构造 +1）"
  - test: "Session ended 面板退出码与信号人话（06-UAT.md Test 4）：双端（ro/rw）观察子进程 exit N 与 kill -HUP 两形态"
    expected: "面板正文逐字 'The process exited with code {N}.' / 'The process was killed by signal SIGHUP.'"
    why_human: "真实浏览器渲染观感——headless 豁免；自动化等价面：phase06-dom.mjs D7（正文逐字 'The process exited with code 7.'）+ phase06.mjs S1/S2（协议层帧内容与帧序）"
  - test: "--once 第二客户端 503 页（06-UAT.md Test 5）：--once 实例下第二浏览器访问"
    expected: "第二客户端见 503 专版页；唯一客户端断开后服务端退出（退出状态 255）"
    why_human: "真实浏览器页面——headless 豁免；自动化等价面：phase06.mjs S3（双点位 503 + 进程退出 255 协议层全证）"
  - test: "owner 断线重连 [ro] 前缀（06-UAT.md Test 6）：owner 模式会话中断线重连"
    expected: "重连后标题出现 [ro] 前缀，写权限不恢复（按新 attach 走递补语义）"
    why_human: "真实浏览器 UI 状态——headless 豁免；自动化等价面：phase06-dom.mjs D10c（标题 [ro] 前缀断言的判别力已由负面对照实证）+ P5 递补语义既有套件（phase05 系列）"
---

# Phase 6: 会话生命周期与重连 Verification Report

**Phase Goal:** 会话生命周期模式完整，断线重连闭环——共享进程模型下重连即接回原 PTY 进程
**Verified:** 2026-08-23T10:58:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths（ROADMAP 成功准则逐条）

| #   | Truth   | Status     | Evidence       |
| --- | ------- | ---------- | -------------- |
| 1 | `wesh --once` 只接受一个客户端，其断开后服务端退出；配置"所有客户端断开后退出"时，最后一个客户端断开即触发退出 | ✓ VERIFIED | **行为证据（本 verifier 实跑）**：phase06.mjs S3 全绿——首客户端 attach 成功、第二客户端双点位 503（/api/attach 早闸 + WS ③位，409 不复活）、断开后进程退出状态 255（OQ1 accept-255 裁决兑现）、stderr 无 panic；S4 立即形态（裸 flag IsBoolFlag 实证 + 启动 400ms 守候不触发 + close 后退出 255）；S5 宽限取消/到期两子场景全绿。Go 层：emptyexit_test.go 六测（Immediate/GraceCancel/GraceExpire/KickTrigger/LifecycleGate/TimerAfterLifecycle）-race 全绿。CLI：--once 展开（main.go:233-239 只填未显式位）、冲突矩阵行为实证（`--once --max-clients=2` / `--once --exit-when-empty=5s` 均 exit 2 含双 flag 名；`=abc`/`=-5s` 均 exit 2） |
| 2 | 子进程退出后所有在线客户端收到含退出码的类型化终结帧提示（非静默断开），随后以 1000 正常关闭 | ✓ VERIFIED | **行为证据（本 verifier 实跑）**：phase06.mjs S1——双端（rw+ro）同收 EXIT{exit_code:42, message 逐字 'The process exited with code 42.'}、帧体逐字节一致（62 字节）、帧序 EXIT 先于 close 1000、进程 exit 42；S2——信号死亡 exit_code=-1 + message 含大写 SIGHUP + 负向断言无 'hangup' 小写回归 + close 1000。Go 层：TestExitFrameBroadcast/TestExitFrameSignal（帧序/双端一致/退出码）+ TestSignalName/TestExitMessage 白盒 -race 全绿。前端全链：phase06-dom.mjs D7——真实服务端 exit 7 → 'Session ended' 正文逐字 'The process exited with code 7.' + 1000 不触发重连。写序安全机制核读：server.go:1079 组帧一次共享只读 → 每客户端 goroutine 同步 Write(EXIT,2s)→Close(1000)（:1111-1114），不经 outbox 异步入队 |
| 3 | 断网 30s 恢复后前端自动重连（指数退避 + 上限 + 手动入口）并接回同一 PTY 进程，输入输出一致（无滚动回放——文档明示） | ✓ VERIFIED（真实断网栈观感 → human_verification） | **行为证据（本 verifier 实跑）**：退避序列 backoffMs=min(1000·2^attempt,30000) 由 reconnect.test.ts 锁定（19/19 node --test 全绿）；phase06-dom.mjs 33/33——D1 全链（1006→Reconnecting 面板 C-9 逐字→退避自动重连→面板隐藏→term.clear() 可观测→beforeunload 重注册）、D5 手动入口（Reconnect now 点击 800ms 窗内发起）、D8 online 快路径、D4 双触发幂等、D2/D3 不触发边界（1002/1013/1008 零新连接）；接回同一 PTY：phase06.mjs S6——pidPost==pidPre（echo S6PID=$$ 进程 ID 相等强证据）+ weshmark42 变量跨断连存活 + 首连接全程无 EXIT 帧。文档明示：README:25-33 重连六要点段（触发范围/退避上限/手动入口/重连目标/无滚动回放 tmux-herdr 分工/写权限不恢复）逐条在场。**豁免部分**：真实 OS 断网栈 30s 场景以 skipped+reason 记录于两脚本（S7/D9），指向 06-UAT.md 人工清单——按 CODEBUDDY.md 硬约束不列为 gap |

**Score:** 3/3 truths verified（0 present-but-behavior-unverified——全部行为相关断言均有本 verifier 实跑的行为测试证据）

### Required Artifacts

| Artifact | Expected    | Status | Details |
| -------- | ----------- | ------ | ------- |
| `internal/proto/proto.go` | Exit='X' 常量 + ExitPayload + ExitFrame | ✓ VERIFIED | :35 `Exit = 'X'`（含与 'E' 语义边界注释）；:126-129 ExitPayload{exit_code,message} snake_case tag；:187-190 ExitFrame 组帧（ErrorFrame 同构） |
| `internal/proto/proto_test.go` | 'X' 行 + ExitFrame round-trip | ✓ VERIFIED | proto 包 -race 全绿（含 TestExitFrame 含 Exit!=Error 区分断言） |
| `internal/server/server.go` | lifecycle EXIT 广播段 + signalName/exitMessage + Options 两字段 + exiting 门 | ✓ VERIFIED | :997 signalName（13 信号映射表）；:1039 exitMessage 三形态；:1079 组帧一次；:1096 exiting=true 先于 :1097 快照（行号序断言成立）；:1111-1114 Write(2s)→Close(1000)；:194-195 Options 字段 + :278-279 New 装配 + :252-253 负值钳 0；termOnce.Do 非注释行 ==1（exitf 单点收口 D-13 保持） |
| `internal/server/clients.go` | maybeExitWhenEmptyLocked + 两调用点 + 宽限计时器 + 取消点 | ✓ VERIFIED | :732 定义（三守卫 :733）；:511（kick）/:702（detach）两调用点均在 removeLocked 成功后；:746 AfterFunc 回调复查『仍空且未 exiting』才 SIGHUP；:766 cancelExitEmptyTimerLocked（Stop+置 nil 恰好一次），server.go:771 Attach 升档序列调用 |
| `internal/pty/signal_linux.go` / `signal_darwin.go` | SignalHangup 双平台同签名 | ✓ VERIFIED | 两文件 :15-17 `syscall.Kill(-s.Cmd.Process.Pid, syscall.SIGHUP)`（负 pid 进程组，错误静默）；构建标签各在位；TestSignalHangup（io_test.go:71）-race 绿 |
| `internal/server/exit_test.go` / `exitmsg_test.go` / `emptyexit_test.go` | 两测 + 两白盒 + 六测 | ✓ VERIFIED | 函数齐（TestExitFrameBroadcast/TestExitFrameSignal；TestSignalName/TestExitMessage；TestExitWhenEmpty×6）；server 包 -race 全绿（49.2s 本 verifier 实跑） |
| `cmd/wesh/main.go` | --once + exitEmptyValue + IsBoolFlag + 展开 + 冲突矩阵 + Options 接线 | ✓ VERIFIED | :199 BoolVar（help 含等价关系逐字）；:204 fs.Var；:82 IsBoolFlag（GOROOT flag.go:350-356 行尾引文）；:90-98 Set 三形态（d<0 负值闸）；:223-226 fs.Visit 显式位；:233-239 展开；:383-388 双冲突行（显式设置位形态）；:476 Options 接线。行为实证：--help 两 flag 行形态正确；冲突/非法 duration 均 exit 2 |
| `web/src/lib/reconnect.ts` + `reconnect.test.ts` | backoffMs + shouldReconnect 纯函数 | ✓ VERIFIED | :7-9 backoffMs=min(1000·2^attempt,30000)；:13-15 shouldReconnect=code===1006；node --test 19/19 全绿（本 verifier 实跑） |
| `web/src/main.ts` | 重连状态机 + 代际守卫 + case 1006/EXIT + lastExit + CR-01 connectGen 双查 | ✓ VERIFIED | :206/:216/:235/:244 状态机四函数；:434 RECONNECTING_TITLE；:451 showStatus 第四可选参 action?:；:609/:808/:827/:845 四 handler sock!==ws 守卫；:914-918 case 1006→startReconnect；:767-768 WELCOME 成功点 stopReconnect+term.clear()；:791-793 case EXIT 暂存 lastExit；:879 onclose 1000 正文 lastExit?.message ?? 回退；:938-947 online/offline 监听。**CR-01 修复在位**：:185 connectGen；:481 gen 捕获；:524 复查①（fetch resolve 后分派前）；:598 复查②（resp.json 后提交句柄前）；:599 ws?.close() 关闭被取代 socket；:601 才 new WebSocket |
| `web/uat/phase06.mjs` | 协议层七场景 + 豁免 + SEC 自净 | ✓ VERIFIED | 本 verifier 实跑 **23/23 PASS + 1 skipped（S7 豁免）+ SEC PASS**；WR-02 修复在位：:83 redactArgs、:95 超时 reject 脱敏、:499 异常通道纳入 emittedDetails |
| `web/uat/phase06-dom.mjs` | jsdom 状态机八场景 + D10 + 豁免 | ✓ VERIFIED | 本 verifier 实跑 **33/33 PASS + 1 skipped（D9 豁免）+ SEC PASS**——含 CR-01 D10 三断言（D10a 零新构造/D10b 面板隐藏/D10c 无 [ro] 降级）；WR-01 修复在位：:512 D7 waitFor 期限 5000ms |
| `web/dist/index.html` | 重建产物入库且与源码同步 | ✓ VERIFIED | `time pnpm -C web build` 退出 0（2.3s）；重建后 git status 零 diff（产物与提交态一致）；产物含 Reconnecting 状态机代码 |
| `README.md` | 生命周期节 + 重连段 + flag 表两行 + 协议节 EXIT 行 | ✓ VERIFIED | :17 EXIT 终结帧行为；:19 --once 等价关系逐字；:21 --exit-when-empty 三形态与迁移语义；:23 accept-255 收口文案「wesh 退出状态 255」；:25-33 重连六要点；:50-51 flag 表两行；:133 协议节 'X' 行；token 占位符 `<ro-token>`/`<rw-token>` 形态保持 |
| `.planning/phases/06-session-lifecycle/06-UAT.md` | 六项人工清单 | ✓ VERIFIED | 六项齐备（7 个 `###` 节含头部）；Reconnect×9；headless 豁免前提与自动化等价面对照登记 |
| `.planning/phases/06-session-lifecycle/06-VALIDATION.md` | 实测行 + 置位 + 失效框架清零 | ✓ VERIFIED | nyquist_compliant: true、wave_0_complete: true（:7-8）；vitest 字面量 0 命中 |

### Key Link Verification

| From | To  | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| server.go lifecycle | proto.ExitFrame | sess.Wait → code/message 提取 → 组帧一次 → 快照 → 每客户端 Write(EXIT)→Close(1000) | ✓ WIRED | server.go:1079 ExitFrame 调用点；行为由 S1/S2 帧序断言实证（末帧 EXIT 且 close 1000） |
| server.go lifecycle | os/exec + syscall | errors.As ExitError → ExitCode()；WaitStatus.Signaled()/Signal() → signalName 大写映射 | ✓ WIRED | exitMessage :1039-1060；行为由 S2（SIGHUP 大写）与 TestExitFrameSignal 实证 |
| clients.go detach/kick | pty.SignalHangup | removeLocked 成功且 len==0 且 !exiting → grace==0 直接 SIGHUP / grace>0 AfterFunc | ✓ WIRED | :511/:702 两调用点；行为由 S4/S5 与六测实证 |
| clients.go registerLocked（经 Attach） | exitEmptyTimer 取消 | 登记成功后同 hubMu 内 cancelExitEmptyTimerLocked（Stop+置 nil+logEvent） | ✓ WIRED | server.go:771 调用点；行为由 S5a（宽限内再 attach 取消）与 TestExitWhenEmptyGraceCancel 实证 |
| lifecycle exiting 门 | clients.go 空触发 | 快照前置 s.exiting=true 抑制广播期 detach 致空 | ✓ WIRED | 行号序 :1096<:1097；行为由 TestExitWhenEmptyLifecycleGate/TimerAfterLifecycle 实证 |
| main.ts onclose case 1006 | connect() 重入 | startReconnect→scheduleAttempt（backoffMs+面板倒计时）→runAttempt→connect()→WELCOME→stopReconnect+term.clear() | ✓ WIRED | 行为由 D1 全链实证（9 断言全绿） |
| main.ts case EXIT | onclose case 1000 | JSON.parse 暂存 lastExit → 1000 正文 lastExit?.message ?? 硬编码回退 | ✓ WIRED | :791-793→:879；行为由 D7 逐字文案实证 |
| window online/offline | 重连循环 | offline→非 OPEN 即 startReconnect；online→reconnecting 则 runAttempt | ✓ WIRED | :938-947；行为由 D4（双触发幂等）/D8（online 快路径）实证 |
| cmd/wesh parseArgs | server.Options.ExitWhenEmpty | fs.Visit 显式判定 → --once 展开 → run() Options 字面量 | ✓ WIRED | main.go:476 接线；行为由 S3（--once 语法糖服务端全链）实证 |
| 前端 bundle | dist/index.html | pnpm build 产物与提交态零 diff | ✓ WIRED | 本 verifier 重建实证字节稳定 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| EXIT 帧载荷 | code/message | sess.Wait() 真实进程退出状态（errors.As ExitError / WaitStatus 信号提取） | ✓ 真实进程路径（S1 exit 42 / S2 SIGHUP 实证） | ✓ FLOWING |
| 前端 Session ended 正文 | lastExit.message | 真实 EXIT 帧 JSON.parse（服务端组文案唯一写口） | ✓（D7 真实服务端 exit 7 逐字文案端到端） | ✓ FLOWING |
| Reconnecting 面板计数 | attempt/retryAt | 真实 setTimeout/setInterval 退避定时器 + backoffMs 纯函数 | ✓（D1 body 'attempt 1' 逐字实证） | ✓ FLOWING |
| --once/--exit-when-empty 服务端行为 | cfg.exitEmpty.set/grace | 真实 CLI 解析（IsBoolFlag 三形态）→ Options 直传 | ✓（S3/S4/S5 真实二进制行为实证） | ✓ FLOWING |
| S6 同进程证据 | pidPre/pidPost | 真实 bash `echo S6PID=$$` 输出正则解析 | ✓（pid 相等实证，无 mock） | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| 全量构建 | `go build ./... && go vet ./...` | 退出 0（0.4s） | ✓ PASS |
| Go 全测试（竞态） | `go test -race -count=1 ./...` | 5/5 包 ok（cmd/wesh 1.0s / proto 1.0s / pty 2.0s / server 49.2s / web 1.0s） | ✓ PASS |
| 前端 lib 单测 | `node --test web/src/lib/*.test.ts` | 19/19 pass（prefs 8 + title 8 + reconnect 3） | ✓ PASS |
| 前端构建 | `time pnpm -C web build` | 退出 0（2.3s）；dist 与提交态零 diff | ✓ PASS |
| CLI 冲突拒绝 | `wesh --once --max-clients=2` / `--once --exit-when-empty=5s` | 均 exit 2，文案含双 flag 名 | ✓ PASS |
| CLI 非法 duration | `wesh --exit-when-empty=abc` / `=-5s` | 均 exit 2（d<0 负值闸实证——ParseDuration("-5s") 解析成功但被拒） | ✓ PASS |
| --help 形态 | `wesh --help` | `-exit-when-empty` 无值占位符（IsBoolFlag 形态）；`-once` help 含等价关系逐字 | ✓ PASS |
| 协议层 UAT | `node web/uat/phase06.mjs` | **23/23 PASS + 1 skipped（S7 豁免）+ SEC PASS** | ✓ PASS |
| jsdom UAT | `node web/uat/phase06-dom.mjs` | **33/33 PASS + 1 skipped（D9 豁免）+ SEC PASS**（含 CR-01 D10 三断言） | ✓ PASS |
| 前序回归 8 套件 | phase02/03/04/04-t1-width/05/05-dims/04-dom/05-dom | 12/12、18/18、10/10、PASS、28/28+1skip、PASS、37/37、19/19——全绿 | ✓ PASS |

### Probe Execution

SKIPPED——本项目无 `scripts/*/tests/probe-*.sh` 形态探针；协议层/DOM 层 UAT 脚本（phaseNN.mjs）即本项目的可执行验证通道，已在上表逐一实跑（非 SUMMARY 转述）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| SESS-01 | 06-02（服务端）/ 06-04（CLI）/ 06-06（协议 UAT）/ 06-07（文档） | --once 模式：只接受一个客户端，其断开后服务端退出 | ✓ SATISFIED | S3 全链（双点位 503 + 断开退出 255）；TestExitWhenEmptyImmediate 等 Go 测；--once 展开与冲突矩阵行为实证 |
| SESS-02 | 06-02 / 06-04 / 06-06 / 06-07 | 可配置"所有客户端断开后退出"模式 | ✓ SATISFIED | S4 立即形态（启动守候不触发）+ S5 宽限取消/到期；GraceCancel/GraceExpire/KickTrigger Go 测；三形态 CLI 解析行为实证 |
| SESS-03 | 06-01（proto/server/前端）/ 06-06 / 06-07 | 子进程退出后客户端收到明确提示（类型化错误帧，含退出码），而非静默断开 | ✓ SATISFIED | S1 双端同帧逐字节 + 帧序 + exit 42；S2 信号死亡 -1+SIGHUP；D7 前端逐字文案；Go 四测 -race 绿 |
| CORE-05 | 06-03（前端状态机）/ 06-05（jsdom）/ 06-06（协议）/ 06-07（文档） | WS 异常断开后前端自动重连并接回同一 PTY 进程 | ✓ SATISFIED | D1-D8/D10 33/33（退避/上限/手动入口/幂等/代际守卫）；S6 pidPre==pidPost 同进程强证据；README 六要点文档明示 |

**Orphaned requirements:** 无——REQUIREMENTS.md Traceability 表 Phase 6 恰好映射此四条，全部出现在 plan requirements 字段且均已验证；REQUIREMENTS.md 四条均已勾选 [x] 且状态 Complete。

### Review Fix Verification（06-REVIEW 三修复逐项核证）

| Finding | Fix Commit | Status | Evidence |
| ------- | ---------- | ------ | -------- |
| CR-01（重连迟到成功踩占健康连接） | 010a3df | ✓ 修复成立 | main.ts:185 connectGen + :524 复查①（fetch resolve 后、状态码分派前）+ :598 复查②（resp.json 二次挂起后、提交句柄前）+ :599 `ws?.close()` 取代关闭 + :601 才建 socket；phase06-dom.mjs D10 场景三断言本 verifier 实跑全绿（holdAttachFetchN 闸控 fetch 复现双在飞，stale 迟到 resolve 后零新构造/面板隐藏/无 [ro] 降级）；fix 报告负面对照（抠掉修复则 D10a/D10c FAIL）与本轮正跑互证 |
| WR-01（D7 waitFor 2000ms 耦合 sleep 2） | 95ab12a | ✓ 修复成立 | phase06-dom.mjs:512 期限 5000ms 在位，注释登记解耦理由；D7a/D7b/D7c 本轮全绿 |
| WR-02（启动超时 reject 回显凭据） | 9d5e067 | ✓ 修复成立 | phase06.mjs:83 redactArgs（空格/等号两形态脱敏）+ :95 reject 消息改脱敏版 + :499 异常通道纳入 emittedDetails；SEC 输出自净本轮 PASS（details=23 命中=false） |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| 无 | - | TBD/FIXME/XXX 扫描 18 个本 phase 触碰文件零命中；placeholder/coming soon/not yet implemented 零命中 | - | - |

### Human Verification Required

六项均为 headless 硬约束豁免项（CODEBUDDY.md 平台原生行为豁免条款）——真实浏览器/OS 网络栈观感，自动化等价面已逐项覆盖并在 frontmatter human_verification 清单登记；人工执行入口 = `.planning/phases/06-session-lifecycle/06-UAT.md` 六项编号清单（每项含逐步操作 + 预期观察 + 需求 ID + 自动化等价面对照）：

1. **断网 30s 恢复自动重连观感**（Test 1，CORE-05）——自动化等价：phase06.mjs S6 + phase06-dom.mjs D1/D4/D8
2. **重连成功清屏与程序重绘观感**（Test 2，CORE-05）——自动化等价：D1h（term.clear() 可观测）
3. **Reconnect now 手动跳过**（Test 3，CORE-05）——自动化等价：D5
4. **Session ended 面板退出码与信号人话（双端双形态）**（Test 4，SESS-03）——自动化等价：D7 + S1/S2
5. **--once 第二客户端 503 页 + 断开退出 255**（Test 5，SESS-01）——自动化等价：S3
6. **owner 断线重连 [ro] 前缀不恢复写权限**（Test 6，CORE-05/D-06）——自动化等价：D10c + P5 递补套件

### Gaps Summary

无 gaps。三条 ROADMAP 成功准则全部有分层行为证据（Go -race 测试 + 协议层 UAT + jsdom UAT + CLI 行为冒烟 + 文档同真核对），全部为本 verifier 在本机实跑复现（非 SUMMARY 转述）；code review 的 1 Critical + 2 Warning 修复逐字核实在位且行为绿；8 个前序 phase UAT 套件回归全绿；四项需求全部 SATISFIED 且无 orphan。状态为 human_needed 唯一原因：headless 环境结构性不可测的真实浏览器/OS 网络栈观感六项（CODEBUDDY.md 显式豁免，人工清单已就位）。

---

_Verified: 2026-08-23T10:58:00Z_
_Verifier: Claude (gsd-verifier)_
