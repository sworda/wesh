---
status: testing
phase: 04-frontend
source: [04-VERIFICATION.md]
started: 2026-08-19T00:10:22Z
updated: 2026-08-19T00:10:22Z
---

## Current Test

number: 1
name: CJK/emoji 宽度（FE-02 / T1）
expected: |
  CJK 字符占两格不错位、不对齐崩坏；emoji 后光标位置正确（不叠字不多占格）
awaiting: user response

## 说明与前置条件

协议层断言已全自动化（`node web/uat/phase04.mjs <wesh 二进制>`，十场景：Welcome prefs 形状/注入/theme 对象/osc52 下发/last-wins/启动拒绝矩阵），本清单只列**渲染/浏览器平台行为面**——自动化结构性不可达的部分（沿用 03-UAT.md 分工先例）。条目编号保留 T1-T11 以便与 04-VERIFICATION.md 交叉引用。

1. 按 README 构建新二进制：`pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh`
2. 三组启动形态（`--port 0` 随机端口，启动行打印实际地址）：

| 形态 | 命令 | 用途 |
|------|------|------|
| A（rw 全功能） | `wesh --bind 127.0.0.1 --port 0 --writable -- bash` | 粘贴/偏好/OSC52 等需可写或全功能的项 |
| B（ro 形态） | `wesh --bind 127.0.0.1 --port 0 -- bash`（不带 `--writable`） | ro 标题前缀、ro 不读剪贴板 |
| C（明文降级） | `wesh --bind <局域网 IP> --port 0 --no-auth -- bash`，另一台设备或同机非 localhost 地址访问 | 明文 HTTP 非 localhost 剪贴板静默降级（D-11） |

每项记录 **PASS/FAIL** 与备注；全部 PASS 后 phase 渲染面收口。

## Tests

### 1. CJK/emoji 宽度（FE-02 / T1）

test: 形态 A。终端内执行 `echo '中文测试🙂🎉'` 与 `printf '中文字符对齐测试\n'`；再打开 vim/htop 显示中文内容
expected: CJK 字符占两格不错位、不对齐崩坏；emoji 后光标位置正确（不叠字不多占格）
why_human: 字形渲染与宽度测量的视觉效果，自动化不可测
result: pending

### 2. IME 组合输入（FE-02 / T2，UI-SPEC backstop 人工出口）

test: 形态 A。用中文拼音输入法在终端输入完整句子（观察组合过程中间态与上屏结果）；如有日文 IME 一并试
expected: 组合过程与上屏不丢字、不乱码、组合框位置正常
why_human: 真实 IME 栈（OS 输入法 → 浏览器 composition 事件 → xterm textarea）自动化不覆盖
result: pending

### 3. 标题同步（CORE-03 / T3）

test: 形态 A 执行 `printf '\e]2;custom-title\a'`，再开 vim/tmux 观察标题随程序变化；形态 B 重复
expected: 浏览器标签页标题同步为 `custom-title` 并随程序变化；形态 B 下恒为 `[ro] custom-title`（`[ro] ` 前缀最前），标题多次变化前缀不丢
why_human: 标签页标题是浏览器可视面
result: pending

### 4. 超链接（FE-04 / T4）

test: 形态 A。`echo 'see https://example.com and https://github.com/sworda/wesh'`——hover 与单击；再 `printf '\e]8;;https://example.com\a显示文本\e]8;;\a'`（OSC 8，显示文本与目标不同）
expected: 自动识别的 URL hover 显示完整真实地址 tooltip，单击新标签页打开；OSC 8 链接 hover 显示 `https://example.com`（显示文本与目标不一致可辨别）且点击**无** confirm 原生框
why_human: hover tooltip 视觉与点击行为需真实浏览器
result: pending

### 5. 选中即复制（FE-05 / T5）

test: 形态 A。鼠标拖动选中一段终端文本，粘贴到他处（编辑器/另一终端）验证；拖动过程中观察
expected: 系统剪贴板即为所选内容；拖动过程不频繁写剪贴板（最终选区一次写入，150ms 防抖）
why_human: 系统剪贴板真实状态需人工核对
result: pending

### 6. 粘贴（FE-05 / T6 / D-10）

test: 形态 A `Ctrl+Shift+V` 粘贴进终端（如粘贴一行命令回显/执行）；形态 B 同样按 `Ctrl+Shift+V`
expected: 形态 A 内容到达 shell（bracketed paste 语义保留，多行不自动执行）；形态 B **无权限弹窗、无效果**（ro 不读剪贴板）
why_human: 浏览器剪贴板权限门控与 bracketed paste 行为需真实浏览器
result: pending

### 7. 明文降级（D-11 / T7）

test: 形态 C 访问（明文 HTTP + 非 localhost 地址）。拖动选中文本、按 `Ctrl+Shift+V`
expected: 选中复制与粘贴**静默不生效**（不弹错、无提示）；终端显示与输入等其余功能正常
why_human: 非安全上下文的浏览器行为差异需真实环境
result: pending

### 8. resize 浮层（FE-06 / T8 / D-17）

test: 形态 A。拖动浏览器窗口改变大小；再以 `--client-option resizeOverlay=false` 重启（或同地址加 `?resizeOverlay=false`）重复拖动
expected: 默认形态 resize 期间右上角显示 `COLSxROWS` 浮层，停止约 600ms 后淡出；关闭开关后不显示
why_human: 浮层视觉与淡出时序需人眼
result: pending

### 9. 离开确认 beforeunload（FE-06 / T9 / D-18；含 sticky activation 裁决记录）

test: 形态 A。会话中（已有终端交互）直接关闭标签页；在终端 `exit` 终结会话（Session ended 面板出现后）再关页；再以 `?confirmBeforeUnload=false` 打开新会话重复关页
expected: 会话中关页被浏览器标准确认框拦截；会话终结后关页不再拦截；开关关闭后不拦截。**打开页面后零交互直接关页不弹框为浏览器预期行为**（sticky activation——04-RESEARCH §Open Questions 3 裁决：接受浏览器语义，零交互即无会话投入，不做代码补偿；记录非缺陷）
why_human: 浏览器原生确认框与 sticky activation 语义自动化不可驱动
result: pending

### 10. 偏好下发与 query 覆盖（FE-07 / T10 / D-16 / D-19）

test: 形态 A 加 `--client-option fontSize=18 --client-option 'theme={"background":"#101020"}'` 启动；同地址再加 `?fontSize=11` 打开；再加 `?fontFamily=Menlo`（裸词非法 JSON）打开
expected: 首发字号变大且背景变 `#101020`，ANSI 彩色输出保持内置调色板色相（theme 合并——未指定色键不丢）；`?fontSize=11` 时字号为 11（query 覆盖 --client-option，优先级）；`?fontFamily=Menlo` 时终端正常可用且 DevTools console 有 warn（非法 query 静默忽略）
why_human: 视觉效果（字号/色相）与 console warn 需真实浏览器
result: pending

### 11. OSC52 写入（可选；D-12 / T11）

test: 形态 A 加 `--osc52` 启动。`printf '\e]52;c;aGVsbG8=\a'` 后到他处粘贴；再 `printf '\e]52;c;?\a'`
expected: 系统剪贴板写入 `hello`；读查询形态无 unhandled rejection（DevTools console 干净）
why_human: 系统剪贴板真实状态与 console 噪音需人工核对
result: pending

## Summary

total: 11
passed: 0
issues: 0
pending: 11
skipped: 0
blocked: 0

## Gaps
