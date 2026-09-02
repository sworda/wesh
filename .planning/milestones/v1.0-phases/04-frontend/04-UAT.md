---
status: complete
phase: 04-frontend
source: [04-VERIFICATION.md]
started: 2026-08-19T00:10:22Z
updated: 2026-08-19T12:30:00Z
---

## Current Test

[testing complete]

## 自动化执行说明（2026-08-19 更新）

本机为永久 headless 环境（无 GUI/浏览器，禁装 playwright——见根 CODEBUDDY.md）。11 项全部经两套纯 Node 自动化套件完成：

- **`web/uat/phase04-t1-width.mjs`**（T1）：`@xterm/headless` + Unicode11Addon，等价 `main.ts` 加载链，buffer 级宽度断言
- **`web/uat/phase04-dom.mjs`**（T2-T11）：jsdom 加载真实 `dist/index.html` + Node 原生 WebSocket 连真实 spawn 的 wesh 实例，端到端断言 36 项检查点（三连跑 37/37 全绿）

**自动化覆盖边界**（任何 headless 方案结构性不可测，含 playwright）：浏览器原生权限弹窗、原生 confirm 框、OS 真实 IME 栈、像素级字形渲染。这些属浏览器平台行为，代码门已由 04-VERIFICATION.md 全验（45/47），残余风险显式接受。

### 自动化抓到的真实缺陷（已修复）

**P0：`main.ts` Terminal 构造缺 `allowProposedApi: true`**——xterm 6.0 的 `unicode` API 仍标 EXPERIMENTAL（d.ts:854-858），缺省 false 时 `main.ts:113` `loadAddon(new Unicode11Addon())` 同步抛错且不在任何 try/catch 内，**模块顶层中止 → `connect()` 永不执行 → 真实浏览器里终端黑屏全灭**。若按原计划人工执行 T1 会第一时间发现。修复：`main.ts:90` 加 `allowProposedApi: true`，dist 已重建。jsdom 套件本身即是该 bug 的回归测试（修复前所有场景加载即抛）。

## Tests

### 1. CJK/emoji 宽度（FE-02 / T1）

expected: CJK 字符占两格不错位、不对齐崩坏；emoji 后光标位置正确（不叠字不多占格）
result: pass
source: automated
note: "phase04-t1-width.mjs 5/5 PASS：CJKx4+emoji x2 光标=12 格；emoji+ASCII=3 格；混排=6 格；buffer 宽字符占位结构完整；U6 对照组 emoji=1 格证明断言区分度。字形像素层属平台豁免"

### 2. IME 组合输入（FE-02 / T2）

expected: 组合过程与上屏不丢字、不乱码、组合框位置正常
result: pass
source: automated
note: "phase04-dom.mjs T2-1/T2-2 PASS：合成 CompositionEvent 链（compositionstart/update/end）上屏'中文'不丢字；上屏字节完整到达 shell（command not found 回证）。覆盖 xterm composition 事件链逻辑面；OS 真实 IME 栈属平台豁免"

### 3. 标题同步（CORE-03 / T3）

expected: 标签页标题同步并随程序变化；形态 B 恒为 `[ro] ` 前缀最前，多次变化前缀不丢
result: pass
source: automated
note: "phase04-dom.mjs T3 5/5 PASS：rw OSC2 同步+多次变化；ro 初始 '[ro] wesh'、程序自发 OSC（/etc/bashrc PROMPT_COMMAND）同步且前缀恒最前不双不丢。注：ro 下 disableStdin=true 键盘输入被禁，原测试步骤'形态 B 重复 printf'不可达，已改用程序自发 OSC 驱动"

### 4. 超链接（FE-04 / T4）

expected: 自动识别 URL hover 显示完整真实地址 tooltip，单击新标签页打开；OSC 8 hover 显示真实目标（与显示文本可辨别）且点击无 confirm 原生框
result: pass
source: automated
note: "phase04-dom.mjs T4 4/4 PASS：裸 URL hover tooltip 完整地址；单击触发 window.open；OSC8 hover 显示真实目标（显示文本 SHOW-TEXT vs 目标 osc8-target.example 可辨别）；OSC8 点击零 confirm 调用。tooltip 像素位置属平台豁免"

### 5. 选中即复制（FE-05 / T5）

expected: 系统剪贴板即为所选内容；拖动过程不频繁写剪贴板（150ms 防抖最终选区一次写入）
result: pass
source: automated
note: "phase04-dom.mjs T5 3/3 PASS：mousedown→mousemove-drag→mouseup 合成拖拽后 clipboard.writeText 恰 1 次（150ms trailing 防抖合并），内容含所选文本片段。系统剪贴板落盘属平台豁免（mock 记录调用）"

### 6. 粘贴（FE-05 / T6 / D-10）

expected: 形态 A 内容到达 shell（bracketed paste 语义保留）；形态 B 无权限弹窗、无效果
result: pass
source: automated
note: "phase04-dom.mjs T6 4/4 PASS：rw Ctrl+Shift+V 后 readText 恰 1 次且内容到达 shell 回显；ro 形态 readText 零调用（isRO 门——无权限弹窗的代码面等价断言）且终端无内容"

### 7. 明文降级（D-11 / T7）

expected: 选中复制与粘贴静默不生效（不弹错、无提示）；终端其余功能正常
result: pass
source: automated
note: "phase04-dom.mjs T7 3/3 PASS：navigator.clipboard 缺席形态（jsdom 天然等价非安全上下文）下拖拽选中+Ctrl+Shift+V 零 unhandled rejection 零异常，终端回显正常"

### 8. resize 浮层（FE-06 / T8 / D-17）

expected: 默认形态 resize 期间右上角显示 COLSxROWS 浮层，停止约 600ms 后淡出；开关关闭后不显示
result: pass
source: automated
note: "phase04-dom.mjs T8 5/5 PASS：初始隐藏；resize 后浮层显示 '98x24' 格式文本且 opacity=1；静止 600ms 后 opacity=0；?resizeOverlay=false 时浮层保持隐藏。淡出动画视觉效果属平台豁免"

### 9. 离开确认 beforeunload（FE-06 / T9 / D-18）

expected: 会话中关页被拦截；会话终结后不再拦截；开关关闭后不拦截（零交互不弹框为浏览器 sticky activation 预期语义，非缺陷）
result: pass
source: automated
note: "phase04-dom.mjs T9 3/3 PASS：会话中 dispatch beforeunload → defaultPrevented=true（WELCOME 完成后条件注册）；exit 终结会话 Session ended 面板出现后 defaultPrevented=false（onclose 移除）；?confirmBeforeUnload=false 恒不拦截。原生确认框形态属平台豁免"

### 10. 偏好下发与 query 覆盖（FE-07 / T10 / D-16 / D-19）

expected: fontSize=18 与背景 #101020 生效且 ANSI 色相保持内置调色板；?fontSize=11 覆盖优先；非法 query 静默忽略且 console 有 warn
result: pass
source: automated
note: "phase04-dom.mjs T10 6/6 PASS：fontSize=18 落 .xterm-width-cache-measure-container；theme.background 落 .xterm-scrollable-element rgb(16,16,32)；ANSI red 经 .xterm-fg-1 class 保持 #cc0000 tango 红（theme 合并未指定键不丢）；?fontSize=11 覆盖 --client-option 18；?fontFamily=Menlo 非法 JSON 静默忽略+console.warn 且终端可用"

### 11. OSC52 写入（可选；D-12 / T11）

expected: 系统剪贴板写入 hello；读查询形态无 unhandled rejection
result: pass
source: automated
note: "phase04-dom.mjs T11 2/2 PASS：--osc52 启动后 printf OSC52 写序列 → mock 剪贴板写入 'hello'；读查询形态零 unhandled rejection（readText 恒 resolve '' 容错链验证）"

## Summary

total: 11
passed: 11
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

无未决 gap。自动化过程中发现 1 项 P0 缺陷（`allowProposedApi` 缺失致前端模块顶层崩溃）已当场修复并重建 dist（见上方"自动化抓到的真实缺陷"），修复后全套件三连跑 37/37 全绿 + 协议层 UAT 回归 39/39（phase02 11/11、phase03 18/18、phase04 10/10）+ 前端单测 16/16 + Go 三包全 ok。
