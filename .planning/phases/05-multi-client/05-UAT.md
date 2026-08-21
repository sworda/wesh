---
status: testing
phase: 05-multi-client
created: 2026-08-21
started: 2026-08-21T01:14:49.772Z
updated: 2026-08-21T01:14:49.772Z
source: [05-09-PLAN.md, 05-VERIFICATION.md]
---

## Current Test

[awaiting manual execution — 需外部浏览器环境]

## 自动化执行说明（2026-08-21）

本机为永久 headless 环境（无 GUI/浏览器，禁装 playwright——见根 CODEBUDDY.md）。Phase 5 多客户端行为的**协议层**全部由 `web/uat/phase05.mjs` 自动化覆盖（18/18 pass + 1 skipped）；本文件是多客户端**渲染层 / 浏览器原生行为**人工核对清单（phase05.mjs S7 skipped 豁免的缓解闭环，review #8 裁决形态——与 01/02/03/04-UAT.md 同位置同形态）。

**自动化覆盖边界**（任何 headless 方案结构性不可测，含 playwright）：浏览器多端像素一致性、原生 Basic 弹窗、DevTools 帧面板观测、节流工具制造的真实慢网。这些属浏览器平台行为，协议层断言已由 phase05.mjs 全部覆盖，残余渲染层风险按 CODEBUDDY.md 显式豁免条款接受——执行本清单即完成闭环。

### 前置

- 构建：`pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh`
- 启动命令均为示例；`--bind 0.0.0.0` 时按启动打印的分享链接两行直接分发（`--credential user:pass` 与链接通道正交，可同时配置）。
- 每项给出 expected（预期）与操作步骤；result 初始 pending，人工执行后回填 pass/fail 与 note。

## Tests

### 1. 双客户端视觉一致（MULTI-01 渲染层）

expected: 两个浏览器窗口 attach 同一会话，输出逐屏一致；异尺寸窗口按最小公共矩形渲染、多余面积留白；关掉一端后剩余端恢复自身尺寸渲染
result: pending
source: manual
steps: "启动 `./wesh --writable -- bash`；两个浏览器窗口（调不同尺寸）分别打开 ro 链接；任一可写端执行 `seq 1 500` 或运行 `yes test` 数秒——两窗口内容逐屏一致；大窗口看到内容区之外的留白；关闭一个窗口后剩余端立即按自身尺寸重排"
note: "协议层等价断言已由 phase05.mjs S1b 覆盖（双客户端 OUTPUT payload 逐字节一致）；本项核对渲染层像素一致"

### 2. 新客首屏（D-11 SIGWINCH 强制重绘）

expected: 会话运行 vim/htop 等全屏程序时新客户端 attach → 秒见重绘画面，不是黑屏等下次输出
result: pending
source: manual
steps: "启动 `./wesh --writable -- bash` 后 attach 一端并运行 `vim` 或 `htop`；第二个浏览器窗口打开 ro 链接——打开瞬间即见 vim/htop 当前画面（服务端 attach 完成时向 PTY 前台进程组显式发 SIGWINCH 强制重绘）"
note: "行内 shell 无重绘需求（下次输出自然追上），本项只核对全屏程序场景"

### 3. ro 形态三要素（旁观者只读边界 UX 层）

expected: ro 端标题恒有 `[ro] ` 前缀最前；键盘不可输入（无回显无效果）；窗口拖动不触发上行流量（DevTools → Network → WS 帧面板无 RESIZE 帧）；console 有一条一次性 read-only mode 提示（含"输入不发送/可能裁剪/reattach 恢复"三要素）
result: pending
source: manual
steps: "打开 ro 分享链接；查看标签页标题前缀；敲击键盘确认无输入效果；拖动窗口边框缩放，同时观察 DevTools Network 的 WS 连接帧面板——只看到下行帧，无上行 RESIZE 帧；打开 console 确认有一条 read-only mode 提示（只出现一次）"
note: "ro 不发 RESIZE 是 D-09 第一闸（省上行流量），服务端忽略为第二闸；协议面无断言等价物（浏览器行为），故列人工核对"

### 4. 递补升格（owner 模式 D-06/D-07）

expected: `--writable`（默认 owner 策略）下两个 rw attach——第二端降级旁观（`[ro] ` 前缀、键盘禁用）；关闭 owner 标签页后第二端标题前缀消失、键盘激活、可正常输入；全程无 toast/badge/通知组件
result: pending
source: manual
steps: "启动 `./wesh --writable -- bash`；窗口 A 打开 rw 链接（成为 owner 可输入）；窗口 B 打开同一 rw 链接——B 呈 `[ro] ` 前缀且键盘无效；关闭 A 标签页——B 在数秒内前缀消失、键盘激活，输入 `echo promoted` 可见回显"
note: "升格信号 = 标题前缀消失 + 键盘激活（D-07 零新 UI 纪律）；升格 Welcome 携 rw 档 prefs（含 osc52 按全局开关下发）"

### 5. 1013 专版（慢消费者踢出，D-10 手动刷新入口）

expected: 用节流工具把某端限速到极慢（或大输出洪水下人为 stall）后被服务端 1013 断开 → 显示 Disconnected 面板（"could not keep up with the session output" 语义）+ "Reload this page" 链接；点击/手动刷新后凭原 URL 重新 attach 成功并从最新输出看起；其他端不受影响
result: pending
source: manual
steps: "启动 `./wesh --writable -- bash` 并 attach 两端；对被测端施加网络节流（如 Chrome DevTools Network throttling 自定义极低速率），另一端执行 `seq 1 3000000` 制造输出洪水；被测端随后出现 Disconnected 面板；刷新页面（URL 中 token 保留）——重新 attach 成功、画面从最新输出开始"
note: "协议层等价断言已由 phase05.mjs S6 覆盖（stderr code=1013 reason=slow_consumer + 他人持续推进 + resume 终结）；本项核对文案面板与手动刷新链路。当前版本无自动重连（Phase 6 CORE-05）"

### 6. 503 专版（容量满员，RES-03 前端半侧）

expected: `--max-clients 1` 实例下第二客户端 attach → 显示 "Server is full" 面板（"reached its maximum number of attached clients" 语义）+ "Wait for a slot to free up, then Reload this page" 提示；首客户端断开后刷新第二端可进入
result: pending
source: manual
steps: "启动 `./wesh --writable --max-clients 1 -- bash`；窗口 A 打开 rw 链接正常进入；窗口 B 打开 ro 链接——直接显示 Server is full 面板（/api/attach 早闸 503，无终端画面）；关闭 A 后在 B 刷新——成功进入"
note: "协议层等价断言已由 phase05.mjs S5 覆盖（/api/attach 503 早闸 + WS ③位 503）；本项核对专版文案与槽位释放后的手动进入"

### 7. 无效链接（错 token 双层响应，R-05 零自绘错误页）

expected: 篡改 token 的 /s/ URL——凭据模式下浏览器弹原生 Basic 登录框（服务端委托既有 `/` 链，无自绘错误页）；绕过/通过 Basic 后 POST 携错 token 失败 → 显示 "Invalid share link" 面板（"invalid or has expired…regenerated each time wesh restarts" 语义）
result: pending
source: manual
steps: "启动 `./wesh --credential user:pass --writable -- bash`；把 ro 链接末段 token 改两个字符后访问——浏览器弹原生 Basic 框；输入凭据通过后页面显示 Invalid share link 面板；无认证实例（loopback 裸跑）下错 token URL 直接给页后显示 Invalid share link 面板"
note: "协议层等价断言已由 phase05.mjs S4 覆盖（/s/ 401 challenge + /api/attach 错 token 与无 token 401 同文同码无 oracle）；本项核对原生 Basic 框形态与专版文案"

## Summary

total: 7
passed: 0
issues: 0
pending: 7
skipped: 0
blocked: 0

## Gaps

无未决 gap 待自动化侧处理——七组清单项全部有协议层自动化等价断言（phase05.mjs 18/18），本文件仅为渲染层/浏览器原生行为的人工闭环。执行环境要求：任意现代浏览器 + 可触达 wesh 实例的网络。
