---
status: complete
phase: 12-per-client
source: [12-VERIFICATION.md]
started: 2026-09-04T16:51:32Z
updated: 2026-09-05T06:47:00Z
---

## Current Test

[testing complete]

## Tests

### 1. CR-01 真实浏览器 resize 观感验证
expected: 在真实浏览器（Windows 工作站 Playwright 层）per-client 模式 attach 后拖窗放大/缩小往复，观察渲染尺寸是否即时跟随窗口（无折行错位、无 attach 时旧尺寸钳制）；重连后旧屏残影被 reset 清除、画面干净。
result: pass
note: 2026-09-05 由新建载具 web/uat/pw/phase12-pw.mjs 执行真实浏览器实测（Windows Playwright Chromium → 本机 TCP 转发器 → Linux 侧 per-client wesh 实例），6/6 场景、20 条断言全绿。关键证据：attach 基线 35r/95c（渲染==PTY）→ 放大 1600x1000 后渲染 62r/193c 且渲染行数==PTY 行数（sessionDims 恒等式成立，修复前恒钳在 attach 35r）→ 173 字符长行单行在场（cols 轴跟随，修复前折为两行）→ 缩小 700x420 回落到 26r（往复非单向）→ 回到 attach 视口回到 35r（无漂移）→ killNet 断网重连后旧 normal buffer 残影不复活 + 新 shell PID 变化 + 面板保持隐藏。模式自证：两浏览器页 shell PID 不同（per-client 每客户端独立会话进程，排除 shared 下假绿）。

### 2. 伺服两通道产物一致性（gzip 预压旁路 vs 明文）
expected: 浏览器带 Accept-Encoding: gzip（全部真实浏览器的默认行为）访问时，服务端下发的前端产物与明文通道（dist/index.html）逐字节一致；前端修复一旦进入产物，必须同时到达 gzip 与明文两条通道。
result: pass
note: 当前两通道逐字节一致（phase12-pw.mjs P12-T0 实测：gzip=500355B 明文=500355B，差 0B）。首轮实测曾失败并定位到陈旧预压体——详见下方 Deferred Follow-Ups（非 Phase 12 源码/受管产物缺陷，已按用户 2026-09-05 裁决登记延后并风险接受）。

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Deferred Follow-Ups

- test: 2
  idea: "构建产物一致性隐患——web/embed.go 的 name+'.gz' 预压旁路直发构建期预压体 dist/index.html.gz，与 dist/index.html 是两个相互独立的实体；.gitignore:2 'web/dist/*.gz' 使预压产物未纳管。任何只更新受管产物 dist/index.html 而不跑全量 pnpm build 的动作（git pull/checkout/revert 等）都会留下陈旧 .gz，而真实浏览器恒带 Accept-Encoding: gzip → 恒取陈旧旁路 → 前端修复被静默回退，且 git status/CI/人工核验均不可见。建议方向：运行期压缩缓存替代旁路 / 构建期或伺服期校验和门禁 / 取消忽略并纳管，使分叉时构建失败。"
  deferred_at: 2026-09-05
  deferred_reason: "非 Phase 12 源码或受管产物缺陷——HEAD 的 web/dist/index.html 含 CR-01 修复（已核验），干净 clone + 全量 pnpm build 不会分叉；故障态仅出现在构建机遗留陈旧 .gz 的情形。已手工重新生成预压产物并重建二进制消除当前分叉，残余为流程隐患。"
  evidence: "Linux 开发机（9.134.229.124）取证据：web/dist/index.html 500355B（9月5日 00:34，含修复，grep 'Math.min(e,1e3)' 计数 1）/ index.html.gz 132393B（9月4日 23:07，不含修复，计数 0）；curl --compressed 与 identity 两响应 sha256 不同（500285B vs 500355B）。执行 'gzip -k -9 -f web/dist/index.html' 并重建二进制后两通道一致，phase12-pw.mjs 6/6 全绿。"
  guard_added: "web/uat/pw/phase12-pw.mjs P12-T0——比对 gzip 与明文两通道解压后字节，把「预压产物陈旧」整类问题挡在观感断言之前（否则本载具会在旧包上跑出全绿）。"

## Gaps

[none]
