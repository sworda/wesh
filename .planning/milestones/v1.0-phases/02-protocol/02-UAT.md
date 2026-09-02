---
status: complete
phase: 02-protocol
source: [02-VERIFICATION.md]
started: 2026-08-15T12:50:00Z
updated: 2026-08-15T23:25:00Z
---

## Current Test

[testing complete]

## Tests

### 1. 只读默认 + 首帧 Hello 顺序
test: wesh -- bash 后浏览器打开页面，标题带 [ro] 前缀，键盘敲击终端无反应；DevTools → Network → WS 面板确认首帧为 Hello('H') 与 Welcome(mode=ro)，后续每 5s 一对 ping/pong 帧
expected: 标题 [ro] 前缀、键盘无响应、首帧 Hello（非 RESIZE）、Welcome mode=ro、周期 ping/pong
why_human: 前端 helloSent 门的真实浏览器时序 Go e2e 覆盖不到；disableStdin/[ro] 标题是渲染层行为
auto_evidence: |
  web/uat/phase02.mjs 全绿（2026-08-15）：服务端首帧=Welcome（无 OUTPUT/RESIZE 抢跑）；
  Welcome(mode=ro)；ro 下 INPUT 标记串零回显（服务端丢弃）；11s+ 连接存活（5s ping/pong 两轮未被 pongTimeout 误杀）
manual_evidence: 用户浏览器确认——[ro] 标题前缀、键盘敲击无反应（2026-08-15）
result: pass
source: automated+manual

### 2. 可写模式
test: wesh --writable -- bash 刷新页面，键入命令
expected: 输入正常回显，Welcome(mode=rw)
why_human: 真实浏览器端到端输入回显需人工确认
auto_evidence: Welcome(mode=rw)；rw 下 INPUT 标记串经 PTY 正常回显
manual_evidence: 用户浏览器确认——键入命令正常回显（2026-08-15）
result: pass
source: automated+manual

### 3. resize
test: ro 模式下拖动浏览器窗口（可先 --writable 起 vim 观察全屏程序）
expected: 无法输入但尺寸跟随重绘；DevTools 可见 RESIZE 帧照常发出
why_human: 全屏程序 resize 重绘是视觉行为
auto_evidence: ro 下 RESIZE 帧被服务端放行（连接不因此关闭）；TIOCSWINSZ 同步由 Go e2e 覆盖
manual_evidence: 用户浏览器确认——拖窗口尺寸跟随重绘正常（2026-08-15）
result: pass
source: automated+manual

### 4. 关闭码文案分派
test: 子进程 exit 显示 Session ended；伪造 wesh.v9 Hello 显示 Connection refused（version_mismatch）；不发 Hello 等 5s 被 1008 关闭
expected: onclose 按码分派人话文案生效
why_human: onclose 文案面板是视觉/交互行为
auto_evidence: |
  子进程退出 → close 1000；Hello(wesh.v9) → Error{version_mismatch}+close 1008；
  无 Hello → 5002ms close 1008(hello_timeout)。三路径码值/错误帧全自动化通过
manual_evidence: |
  用户浏览器确认 "Session ended" 面板；1008 两面板（无法纯浏览器复现）按协议证据 +
  onclose 分派逻辑实读（main.ts:188-241，VERIFICATION 已确认）认可（2026-08-15）
result: pass
source: automated+manual

### 5. 单客户端
test: 另开第二个标签访问同地址
expected: 显示 Unable to connect（409 语义不变）
why_human: 多标签浏览器交互需人工确认
auto_evidence: 主连接占用后第二 WS Upgrade → HTTP 409（自动化通过）
manual_evidence: 用户浏览器确认第二标签 "Unable to connect" 面板（2026-08-15）
result: pass
source: automated+manual

### 6. 【人工决策·非测试】CR-01 修复时机
test: Attach 读循环同步写 PTY master 在特定条件下永久阻塞（code review BLOCKER，详见 02-REVIEW.md / 02-VERIFICATION.md「Code Review 发现评估」节）
expected: 决定立即做最小缓解（master fd 置 O_NONBLOCK + ErrWouldBlock 走既有收口），或将完整的「有界输入队列 + 独立写 goroutine」背压方案并入 Phase 5
why_human: 修复范围/时机的工程权衡决策，非自动化可判定
result: pass
decision: |
  用户决策（2026-08-15）：立即最小缓解——master fd 置 O_NONBLOCK + ErrWouldBlock 走既有收口，
  消除读循环挂死与 pinger 误杀；完整背压方案（有界输入队列 + 独立写 goroutine + 1013 踢出）仍留 Phase 5

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none]
