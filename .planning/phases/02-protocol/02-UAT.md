---
status: testing
phase: 02-protocol
source: [02-VERIFICATION.md]
started: 2026-08-15T12:50:00Z
updated: 2026-08-15T12:50:00Z
---

## Current Test

number: 1
name: 只读默认 + 首帧 Hello 顺序
expected: |
  标题 [ro] 前缀、键盘无响应、首帧 Hello（非 RESIZE）、Welcome mode=ro、周期 ping/pong
awaiting: user response

## Tests

### 1. 只读默认 + 首帧 Hello 顺序
test: wesh -- bash 后浏览器打开页面，标题带 [ro] 前缀，键盘敲击终端无反应；DevTools → Network → WS 面板确认首帧为 Hello('H') 与 Welcome(mode=ro)，后续每 5s 一对 ping/pong 帧
expected: 标题 [ro] 前缀、键盘无响应、首帧 Hello（非 RESIZE）、Welcome mode=ro、周期 ping/pong
why_human: 前端 helloSent 门的真实浏览器时序 Go e2e 覆盖不到；disableStdin/[ro] 标题是渲染层行为
result: [pending]

### 2. 可写模式
test: wesh --writable -- bash 刷新页面，键入命令
expected: 输入正常回显，Welcome(mode=rw)
why_human: 真实浏览器端到端输入回显需人工确认
result: [pending]

### 3. resize
test: ro 模式下拖动浏览器窗口（可先 --writable 起 vim 观察全屏程序）
expected: 无法输入但尺寸跟随重绘；DevTools 可见 RESIZE 帧照常发出
why_human: 全屏程序 resize 重绘是视觉行为
result: [pending]

### 4. 关闭码文案分派
test: 子进程 exit 显示 Session ended；伪造 wesh.v9 Hello 显示 Connection refused（version_mismatch）；不发 Hello 等 5s 被 1008 关闭
expected: onclose 按码分派人话文案生效
why_human: onclose 文案面板是视觉/交互行为
result: [pending]

### 5. 单客户端
test: 另开第二个标签访问同地址
expected: 显示 Unable to connect（409 语义不变）
why_human: 多标签浏览器交互需人工确认
result: [pending]

### 6. 【人工决策·非测试】CR-01 修复时机
test: Attach 读循环同步写 PTY master 在特定条件下永久阻塞（code review BLOCKER，详见 02-REVIEW.md / 02-VERIFICATION.md「Code Review 发现评估」节）
expected: 决定立即做最小缓解（master fd 置 O_NONBLOCK + ErrWouldBlock 走既有收口），或将完整的「有界输入队列 + 独立写 goroutine」背压方案并入 Phase 5
why_human: 修复范围/时机的工程权衡决策，非自动化可判定
result: [pending]

## Summary

total: 6
passed: 0
issues: 0
pending: 6
skipped: 0
blocked: 0

## Gaps
