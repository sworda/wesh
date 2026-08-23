---
status: testing
phase: 06-session-lifecycle
created: 2026-08-23
source: [06-VERIFICATION.md, 06-07-PLAN.md, 06-05-SUMMARY.md, 06-06-SUMMARY.md]
started: 2026-08-23T10:05:00Z
updated: 2026-08-23T10:05:00Z
---

## Current Test

number: 1
name: 断网 30s 恢复自动重连（CORE-05 主场景观感）
expected: |
  断网后数秒内出现「Reconnecting」面板（attempt 计数 + 下次重试倒计时，退避 1s×2 封顶 30s）；断网约 30s 期间计数递增、倒计时周期变长；恢复网络后 5s 内自动接回原会话（同一 shell 进程，断网前现场仍在上游）
awaiting: user response

[awaiting manual execution — 开发机为永久 headless 环境（无 GUI/浏览器，禁装 playwright——见根 CODEBUDDY.md），本清单供外部有浏览器的机器人工执行]

## 自动化执行说明（2026-08-23）

Phase 6 行为在 headless 硬约束下已按分层策略自动化覆盖：**协议层**由 `web/uat/phase06.mjs` 覆盖（23/23 pass + 1 skipped——EXIT 双端逐字节一致广播、信号死亡 exit_code=-1+大写 SIGHUP、--once 双点位 503 + 进程退出状态 255、--exit-when-empty 立即/宽限取消/宽限到期三形态、断连重接同一 PTY 进程 ID 相等主证据）；**DOM 逻辑层**由 `web/uat/phase06-dom.mjs` 覆盖（30/30 pass + 1 skipped——1006 重连全链、1002/1013/1008 不触发边界、双触发幂等、Reconnect now 手动入口、代际守卫、EXIT 帧端到端逐字文案、online 快路径）。

下列六项为**残余人工验证项**——真实 OS 断网栈与浏览器原生 online/offline 事件序列、断网/恢复观感、像素级重绘观感、真实浏览器点击入口，任何自动化（含 playwright）结构性不可测，按 CODEBUDDY.md 平台原生行为豁免条款风险接受。每项注明已落盘的自动化等价面。

### 前置

- 构建：`pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh`
- 自动化复跑：`go build -o /tmp/wesh-uat/wesh ./cmd/wesh && node web/uat/phase06.mjs && node web/uat/phase06-dom.mjs`
- 人工执行启动命令（默认 bind 0.0.0.0:7681 有两道安全闸——无凭据拒听、凭据走明文 HTTP 拒听，报错均为设计行为不是故障）：
  - 局域网直接分发：`./wesh --writable --credential user:pass --insecure-http -- bash`
    （stderr 有明文警告；有证书则 `--tls-cert/--tls-key` 取代 --insecure-http）
  - 或 loopback + SSH 转发（免凭据免警告）：`./wesh --writable --bind 127.0.0.1 -- bash`，
    浏览器侧 `ssh -L 7681:127.0.0.1:7681 <本机>` 后访问 `http://127.0.0.1:7681`
  - 可信网络裸跑：`./wesh --writable --no-auth -- bash`（stderr 有警告）

## Tests

### 1. 断网 30s 恢复自动重连（CORE-05 主场景观感）

expected: 断网后数秒内出现「Reconnecting」面板（attempt 计数 + 下次重试倒计时，退避 1s×2 封顶 30s）；断网约 30s 期间计数递增、倒计时周期变长；恢复网络后 5s 内自动接回**原会话**（同一 shell 进程，断网前现场仍在上游）
result: pending
source: manual
steps: "1. 启动 `./wesh --writable --credential user:pass --insecure-http -- bash`（或 loopback + SSH 转发形态，见前置节），浏览器打开会话，确认可输入；2. 在终端执行 `echo $$` 记录 shell 进程号，再执行 `echo before-disconnect` 留下可辨识现场；3. 断网（飞行模式/拔网线/禁用网卡）；4. 观察页面：数秒内应出现「Reconnecting」面板，正文显示 attempt N 与下次重试倒计时；5. 保持断网约 30s——attempt 计数应递增、倒计时周期按 1s×2 封顶 30s 变长；6. 恢复网络；7. 预期：5s 内自动接回原会话——面板消失、终端清屏后重绘，`echo $$` 与断网前相同（同一进程）"
note: "自动化等价面：phase06-dom.mjs D1（合成 1006 → Reconnecting 面板三件套逐字要点 → 退避自动重连 → 面板隐藏 → 清屏可观测）+ D8（online 事件快路径）；phase06.mjs S6（真实 TCP 断连 → 重接同一 PTY，pidPre==pidPost 进程 ID 主证据）。真实断网栈与浏览器原生事件时序属平台豁免（两脚本各以 skipped+reason 登记）"

### 2. 重连成功清屏与程序重绘观感（CORE-05 首屏恢复）

expected: 全屏程序（vim/htop）运行中断网再恢复后：自动接回先清屏（断网前画面不残留错位/叠影），随后程序经 SIGWINCH 秒级重绘出干净完整画面；断网窗口期错过的输出行不滚动回放（设计使然，README 生命周期节明示）
result: pending
source: manual
steps: "1. 启动会话（同 Test 1 前置），浏览器进入后运行 `vim <可辨识文件>` 或 `htop`；2. 断网，等「Reconnecting」面板出现；3. 恢复网络；4. 预期：接回瞬间终端清屏，随后 1-2s 内 vim/htop 重绘出完整干净画面（无新旧画面叠加错位）；5. 回到 shell 后观察：断网窗口期产生的输出不出现在屏幕上——行内历史恢复属 tmux/herdr 既定分工，wesh 不做滚动回放"
note: "自动化等价面：phase06-dom.mjs D1h（重连成功 term.clear() 可观测——断连前 echo 标记从终端 DOM 消失）；phase05.mjs S8（attach 路径 SIGWINCH 强制重绘，vim 实证——重连 attach 复用同一挂点）。像素级观感属平台豁免"

### 3. Reconnect now 手动跳过（CORE-05 手动入口）

expected: Reconnecting 面板等待期点击「Reconnect now」链接后立即发起重连（不等当前退避倒计时到期），接回成功后面板消失
result: pending
source: manual
steps: "1. 启动会话并进入（同 Test 1 前置）；2. 断网使「Reconnecting」面板出现，确认倒计时正在等待（如 countdown 显示 2s/4s 等 >1s 值）；3. 恢复网络，立即点击面板 hint 处的「Reconnect now」链接；4. 预期：点击后立刻发起本次重连（不等待倒计时走完），数秒内面板消失、接回会话"
note: "自动化等价面：phase06-dom.mjs D5（等待期点击 #status-hint a → 800ms 容差窗内新连接构造，≪ 标称退避 1s——倒计时未完即 attempt → 循环以成功终止）。真实浏览器点击手感属平台豁免"

### 4. 子进程退出 Session ended 面板（SESS-03 终结帧人话）

expected: 子进程退出后所有在线客户端显示「Session ended」面板：正常退出正文为 `The process exited with code N.`（退出码人话）；被信号杀死正文为 `The process was killed by signal SIGNAME.`（大写信号名）——非静默断开/黑屏
result: pending
source: manual
steps: "1. 启动 `./wesh --writable --credential user:pass --insecure-http -- bash`，窗口 A、窗口 B 各打开会话（双端在线）；2. 在窗口 A 输入 `exit 42`；3. 预期：两个窗口都显示「Session ended」面板，正文逐字为 `The process exited with code 42.`；连接以 1000 正常关闭（DevTools → Network → WS 帧面板可见 1000），wesh 进程退出码 42；4. 信号形态：重新启动 `-- sh -c 'sleep 300'` 进入会话，在终端执行 `kill -HUP $$`；5. 预期：面板正文逐字为 `The process was killed by signal SIGHUP.`（大写信号名，非小写 hangup），wesh 进程退出状态 255"
note: "自动化等价面：phase06-dom.mjs D7（真实服务端 EXIT 帧 → Session ended 正文逐字 'The process exited with code 7.' + wesh 进程 exit 码 7）；phase06.mjs S1（双端帧体逐字节一致 + 帧序 EXIT 先于 1000 + 退出码 42）/S2（exit_code=-1 + 大写 SIGHUP + 小写 hangup 负向锁定）。面板像素观感属平台豁免"

### 5. --once 第二客户端 503（SESS-01 容量页与断开退出）

expected: `--once` 实例下第二个浏览器客户端打开同一 URL → 显示「Server is full」面板（reached its maximum number of attached clients 语义 + 等槽位释放提示）；唯一客户端断开后 wesh 进程退出，退出状态 255（子进程被 SIGHUP 终结）
result: pending
source: manual
steps: "1. 启动 `./wesh --once --writable --credential user:pass --insecure-http -- bash`；2. 窗口 A 打开会话——正常进入；3. 窗口 B 打开同一 URL；4. 预期：B 显示「Server is full」面板（不进入终端），A 不受影响；5. 关闭窗口 A 标签页（唯一客户端断开）；6. 预期：wesh 进程退出，shell 侧 `echo $?` 显示 255；7. 补充观察：B 此时刷新不会进入（服务端已退出，页面停在连接失败面板）"
note: "自动化等价面：phase06.mjs S3（第二客户端 /api/attach 早闸 + WS 直连双点位 503、唯一客户端断开后进程级退出状态 255、stderr 无 panic）；phase05-dom.mjs D3（Server is full 面板三件套逐字断言，--max-clients 1 同路径）。README 生命周期节已明示 --once ≡ --max-clients=1 --exit-when-empty=0 与退出状态 255"

### 6. owner 断线重连不恢复写权限（D-06 递补语义）

expected: owner 客户端断线重连后按新 attach 走递补语义——不恢复写权限：重连回来的端显示 `[ro] ` 标题前缀、键盘禁用（写权限已由递补队列前位的端接管）
result: pending
source: manual
steps: "1. 启动 `./wesh --writable --credential user:pass --insecure-http -- bash`（默认 owner 写策略）；2. 窗口 A 打开 rw 分享链接——成为 owner，可输入；窗口 B 打开同一 rw 链接——降级旁观（`[ro] ` 前缀、键盘禁用）；3. 窗口 A 断网（或 DevTools → Network 断开其 WS），等「Reconnecting」面板出现后恢复网络；4. A 自动重连成功后观察两端；5. 预期：A 重连后标题带 `[ro] ` 前缀、键盘禁用（不恢复写权限）；B 已升格为 owner（前缀消失、键盘激活）——写权限按 attach 顺序递补，重连不构成豁免"
note: "自动化等价面：phase05-dom.mjs D2（rw 第二端降级旁观 → owner 断开 → 升格全链逐字断言）+ phase05.mjs S9（递补升格协议层全链）——重连 attach 与新 attach 走同一升档序列（服务端无重连概念），D-06 语义由该同构性覆盖；重连触发面由 phase06-dom.mjs D1 覆盖"

## Summary

total: 6
passed: 0
issues: 0
pending: 6
skipped: 0
blocked: 0

## Deferred Follow-Ups

[none]

## Gaps

[none]
