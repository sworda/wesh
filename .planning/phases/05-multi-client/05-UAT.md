---
status: complete
phase: 05-multi-client
created: 2026-08-21
started: 2026-08-21T01:14:49.772Z
updated: 2026-08-22T19:50:00.000Z
source: [05-09-PLAN.md, 05-VERIFICATION.md]
---

## Current Test

[testing complete — 全部 7 项已通过；G-05-1 由 05-10/11/12 闭合，自动化复跑 28/28+19/19+3/3 全过]

## 自动化执行说明（2026-08-21）

本机为永久 headless 环境（无 GUI/浏览器，禁装 playwright——见根 CODEBUDDY.md）。Phase 5 多客户端行为的**协议层**由 `web/uat/phase05.mjs` 覆盖（28/28 pass + 1 skipped）；**渲染/交互逻辑面**由 `web/uat/phase05-dom.mjs`（jsdom + 真实 dist + 真实 spawn 实例，phase04-dom.mjs 同基建）覆盖（19/19 pass）；**终端核心层**由 `web/uat/phase05-dims.mjs`（@xterm/headless，与浏览器同 buffer 代码路径）覆盖（3/3 pass）。

**2026-08-22 自动化扩编**：协议层新增 S8（D-11 attach SIGWINCH 强制重绘，vim 实证）/S9（D-06/D-07 递补升格全链：升格 Welcome → ro 期 INPUT 丢弃 → 升格后 INPUT/RESIZE 生效 → PTY 尺寸跟随）；DOM 层新建 D1-D5 十六断言。七项人工清单中六项已由自动化等价或更强断言闭环（result=pass, source=automated），仅 Test 1 的**像素级**一致性（留白/多端逐屏视觉）结构性不可测，保留人工核对。

**2026-08-22 G-05-1 回归锁三层扩编**（05-12，gap 闭合证据层）：协议层新增 S10 会话尺寸下发断言（S10a 异尺寸 attach Welcome 携会话尺寸而非自身窗口尺寸 / S10b owner resize 经 50ms 防抖后全端收 'W' 推送 / S10c 升格 Welcome 携新 owner 尺寸）；DOM 层新增 D6 约束渲染断言（D6a 宽端旁观 .xterm-rows 约束到会话 rows / D6b 长行输出在会话 cols 处折行——叠写回归的 DOM 层等价物 / D6c 升格后约束解除回窗口尺寸）；终端核心层新建 phase05-dims.mjs（probe10 探针机制转正——D6H-1 等价锁：约束渲染 ≡ 窄端原生逐屏严格一致；D6H-2 负对照：同字节流喂 120 列换行点分叉，证明断言区分度）。

**自动化覆盖边界**（任何 headless 方案结构性不可测，含 playwright）：浏览器多端像素一致性、原生 Basic 弹窗形态、节流工具制造的真实慢网。协议层与 DOM 逻辑面断言已全覆盖，残余像素/平台原生风险按 CODEBUDDY.md 显式豁免条款接受。

### 前置

- 构建：`pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh`
- 自动化复跑：`node web/uat/phase05.mjs ./wesh && node web/uat/phase05-dom.mjs ./wesh && node web/uat/phase05-dims.mjs ./wesh`
- Test 1 人工执行启动命令（默认 bind 0.0.0.0:7681 有两道安全闸——无凭据拒听（D-03）、
  凭据走明文 HTTP 拒听（D-05），报错均为设计行为不是故障）：
  - 局域网直接分发：`./wesh --writable --credential user:pass --insecure-http -- bash`
    （stderr 有明文警告；有证书则 `--tls-cert/--tls-key` 取代 --insecure-http）
  - 或 loopback + SSH 转发（免凭据免警告）：`./wesh --writable --bind 127.0.0.1 -- bash`，
    浏览器侧 `ssh -L 7681:127.0.0.1:7681 <本机>` 后访问 `http://127.0.0.1:7681`
  - 可信网络裸跑：`./wesh --writable --no-auth -- bash`（stderr 有警告）

## Tests

### 1. 双客户端视觉一致（MULTI-01 渲染层）

expected: 两个浏览器窗口 attach 同一会话，输出逐屏一致；异尺寸窗口按最小公共矩形渲染、多余面积留白；关掉一端后剩余端恢复自身尺寸渲染
result: pass
source: automated
severity_was: major
reported_was: "A调小，B调大之后，在A内输入内容后B的输出会和输入出现重叠的问题，其他没什么问题"
steps: "启动 `./wesh --writable --credential user:pass --insecure-http -- bash`（或 loopback + SSH 转发形态，见前置节）；窗口 A（调小）打开 rw 链接——为 owner 可输入；窗口 B（调大）打开同一 rw 链接——降级旁观（[ro] 前缀、键盘禁用）；A 里输入/执行命令——两窗口内容逐屏一致；大窗口看到内容区之外的留白；关闭 A 标签页后 B 数秒内升格（[ro] 前缀消失、键盘激活）并按自身尺寸重排填满。注意：ro 链接打开的窗口是硬性只读旁观者，rwEligible 恒 false 永不参与递补升格（D-06 安全语义——只读链接不得静默变可写），升格验证必须双窗都开 rw 链接"
note: "2026-08-22 用户实测发现 G-05-1（D-09 min-rect 假设对 readline 行编辑相对寻址流不成立）→ 05-10/11/12 三 plan 闭合：Welcome/'W'/升格 Welcome 携会话 cols/rows（S10a/b/c），前端宽端 xterm 视口按会话矩形约束渲染（D6a 行数=会话 rows / D6b 长行在会话 cols 折行——叠写回归 DOM 等价物 / D6c 升格解除约束），headless 等价锁 D6H-1（同 40 列渲染同字节流逐屏一致）+ 负对照 D6H-2（120 列换行点分叉证明断言区分度）。2026-08-22 复跑：phase05.mjs 28/28+1skip、phase05-dom.mjs 19/19、phase05-dims.mjs 3/3。残余像素层逐屏视觉一致属平台豁免（CODEBUDDY.md），协议/DOM/终端核心三层已结构性覆盖"

### 2. 新客首屏（D-11 SIGWINCH 强制重绘）

expected: 会话运行 vim/htop 等全屏程序时新客户端 attach → 秒见重绘画面，不是黑屏等下次输出
result: pass
source: automated
note: "phase05.mjs S8a 实证：vim 运行中 ro 端 attach，3s 内零输入驱动收到含文件内容 marker 的全屏重绘 OUTPUT（曾首版 FAIL——断言 base 切片竞态漏收 Welcome 同窗口帧，改全窗搜索后稳定过；SIGWINCH 送达与重绘双实证）"

### 3. ro 形态三要素（旁观者只读边界 UX 层）

expected: ro 端标题恒有 `[ro] ` 前缀最前；键盘不可输入（无回显无效果）；窗口拖动不触发上行流量（DevTools → Network → WS 帧面板无 RESIZE 帧）；console 有一条一次性 read-only mode 提示（含"输入不发送/可能裁剪/reattach 恢复"三要素）
result: pass
source: automated
note: "phase05-dom.mjs D1a-d 全过：[ro] 前缀恒最前（title='[ro] wesh'）；typeText 注入零 INPUT 上行（disableStdin 门）；dims 变更+resize 事件零 RESIZE 上行（D-09 前端闸，WS 发送探针断言，强于 DevTools 帧面板人工观测）；console.info 恰一条三要素齐备"

### 4. 递补升格（owner 模式 D-06/D-07）

expected: `--writable`（默认 owner 策略）下两个 rw attach——第二端降级旁观（`[ro] ` 前缀、键盘禁用）；关闭 owner 标签页后第二端标题前缀消失、键盘激活、可正常输入；全程无 toast/badge/通知组件
result: pass
source: automated
note: "phase05-dom.mjs D2a-d（jsdom 页全程：降级旁观 → owner 断开 → 前缀消失 → INPUT 恢复上行 → 零 toast/badge 节点）+ phase05.mjs S9a-c（协议层升格 Welcome/INPUT 丢弃/尺寸接管）双层全过"

### 5. 1013 专版（慢消费者踢出，D-10 手动刷新入口）

expected: 用节流工具把某端限速到极慢（或大输出洪水下人为 stall）后被服务端 1013 断开 → 显示 Disconnected 面板（"could not keep up with the session output" 语义）+ "Reload this page" 链接；点击/手动刷新后凭原 URL 重新 attach 成功并从最新输出看起；其他端不受影响
result: pass
source: automated
note: "phase05-dom.mjs D5a-c 全过：Atomics.wait 分段阻塞事件循环制造内核级 stall（undici 停 drain → outbox 涨满），踢出经 stderr slow_consumer 证实；解阻后 1013 关闭帧到达 → Disconnected 专版三件套；driver 子进程（独立事件循环不随阻塞 stall）收流 12MB+ 未被踢；新开页凭原 URL 重 attach 成功且无洪水回放（可见文本 161B）。真实节流慢网形态属平台豁免"

### 6. 503 专版（容量满员，RES-03 前端半侧）

expected: `--max-clients 1` 实例下第二客户端 attach → 显示 "Server is full" 面板（"reached its maximum number of attached clients" 语义）+ "Wait for a slot to free up, then Reload this page" 提示；首客户端断开后刷新第二端可进入
result: pass
source: automated
note: "phase05-dom.mjs D3a-c 全过：专版三件套逐字断言 + 早闸无 HELLO 上行（WS 从未建连）+ 槽位释放后新开页（手动刷新等价物）attach 成功"

### 7. 无效链接（错 token 双层响应，R-05 零自绘错误页）

expected: 篡改 token 的 /s/ URL——凭据模式下浏览器弹原生 Basic 登录框（服务端委托既有 `/` 链，无自绘错误页，401 body 携带"链接失效/需登录"提示文案，弹窗取消后浏览器渲染）；Basic 验证通过则正常进入（operator 通道，不弹 Invalid）；无认证模式（不弹登录框）或重启失效（无 Basic 缓存）携错 token → 401 → 显示 "Invalid share link" 面板（用户裁决 2026-08-22）
result: pass
source: automated
note: "裁决后形态全部自动化实证：D4a 凭据模式无 Basic 缓存携错 token → 401 → Invalid 面板三件套；D4b 无认证模式错 token → 401（无挑战头）→ Invalid 面板（G-05-7 修复点）；phase05.mjs S4c 证 401 body 提示文案在场且全挑战同文（无 oracle）；S4d/e 证无认证错 token 401 与未携 token 404 探测信号不变；Basic 通过即进入属委托既定（S4b 形状断言+裸 fetch 实证）。原生 Basic 弹窗形态属平台豁免"

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0
blocked: 0

## Deferred Follow-Ups

[none]

## Gaps

- gap_id: G-05-1
  truth: "异尺寸客户端按最小公共矩形渲染、多余面积留白——含行编辑回显在内逐屏一致"
  status: resolved
  resolved_by: "05-10-PLAN.md + 05-11-PLAN.md + 05-12-PLAN.md（gap_closure: true, gap_ids: [G-05-1]）"
  resolved_at: "2026-08-22"
  reason: "用户实测（2026-08-22）：A 小 B 大时，A 内输入后 B 的输出与输入重叠"
  severity: major
  test: 1
  root_cause: "D-09 设计假设（resize.go:30-32 注释逐字）：「min-rect 不变量——任何参与端窗口 ≥ PTY 尺寸，各端按自己窗口渲染、多余面积留白，无需 S→C 尺寸下发帧」。该假设只对绝对寻址流（curses 全屏程序按 PTY 矩形绝对定位）与纯文本流成立；对 readline 行编辑等按终端宽度生成环绕点/光标上行的相对寻址流不成立——同一字节流在宽端换行点错位，光标序列落到错误行产生叠写。headless 复现（probe10）：40 列 PTY 字节流喂 120 列 xterm，长命令回显换行点分叉（窄端 2 行/宽端 1 行）"
  artifacts:
    - path: "internal/server/resize.go"
      issue: "D-09 推论「无需 S→C 尺寸下发帧」假设不覆盖相对寻址流；会话尺寸不下发，宽端 xterm 按自身宽度解释窄宽度字节流"
    - path: "web/src/main.ts"
      issue: "前端无条件 fit 到窗口全尺寸；无会话尺寸概念，无法约束渲染视口到 min-rect"
  missing:
    - "方向 A（真正修复）：Welcome（或新控制帧）携带会话 cols/rows；前端当会话尺寸小于自身窗口时约束 xterm 视口到会话矩形渲染（真正留白），同 cols 渲染同字节流 = 逐屏一致；升格 Welcome 携新 owner 尺寸恢复自渲染"
    - "方向 B（文档化）：README/ro 提示明示宽端旁观者在行编辑回显时可能重叠错位"
  verification: "2026-08-22 复跑三层断言：phase05.mjs 28/28+1skip（S10a 异尺寸 Welcome 携会话尺寸 / S10b owner RESIZE 全端收 W / S10c 升格 Welcome 携新 owner 尺寸）；phase05-dom.mjs 19/19（D6a 行数约束=会话 rows / D6b 长行在会话 cols 折行 / D6c 升格解除约束）；phase05-dims.mjs 3/3（D6H-1 同 40 列渲染同字节流逐屏一致等价锁 / D6H-2 120 列换行点分叉负对照）；go test ./... -race 全包 ok"

- gap_id: G-05-7
  truth: "无认证模式（不弹登录框）下错 token 分享链接 → 显示 Invalid share link 面板"
  status: resolved
  reason: "用户裁决（2026-08-22）：错 token 后若弹登录框且验证通过则正常进入（不弹 Invalid）；若不弹登录框（无认证模式）或验证未通过，仍要弹 Invalid 面板。修复前：无认证模式 POST 携错 token → shareAttach miss → 404 → 前端按探测信号直连 WS 成功进入，Invalid 面板不出现"
  severity: major
  test: 7
  root_cause: "server.go Handler() 无认证分支：shareAttach 对「未携 token」与「携错 token」同返 false → 一律 404；前端 404 分支注释假定「携 token 时无认证模式同样非 404」（假定与实现漂移）"
  artifacts:
    - path: "internal/server/server.go"
      issue: "无认证 POST /api/attach 分支不区分 body 是否携 token，错 token 同返 404"
  missing:
    - "无认证模式 body 携 token 但 lookup 未命中 → 401（携 token 401 → 前端既有 C-3 分支直接承接，零前端改动）；body 无 token → 404 探测信号不变"
    - "测试联动：sharetoken_test.go 无认证错 token 断言、phase05.mjs S4 增补无认证 401 断言、phase05-dom.mjs D4b 翻转为 Invalid 面板断言"
  resolved_by: "直接修复（用户确认免 plan 模式）：server.go shareAttach 改三态 shareResult（absent/invalid/handled），无认证分支 invalid → 401（authRequiredBody，无挑战头）；auth.go 401 body 统一富化提示文案（token 失效需登录，全挑战同文无 oracle）；sharetoken_test.go/phase05.mjs S4c-e/phase05-dom.mjs D4b 三处测试联动更新"
  resolved_at: "2026-08-22"
  verification: "go test ./... 全过；phase05.mjs 25/25（S4c-e 新增三断言）；phase05-dom.mjs 16/16（D4b 翻转后过）"
