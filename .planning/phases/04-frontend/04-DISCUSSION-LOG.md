# Phase 4: 前端体验 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-18
**Phase:** 4-frontend
**Areas discussed:** 标题同步路径 (CORE-03)、超链接与 OSC 8 (FE-04)、剪贴板方案 (FE-05)、偏好下发与辅助交互 (FE-07+FE-06)

---

## 标题同步路径 (CORE-03)

| Option | Description | Selected |
|--------|-------------|----------|
| 纯前端解析 | xterm.js 默认解析 OSC 0/1/2 触发 onTitleChange，前端订阅直写 document.title。零服务端工作、数据泵零拷贝；'T' 帧占位保留，Phase 5 再评估 | ✓ |
| 服务端解析发 TITLE 帧 | 服务端在 OUTPUT 流上跑 OSC 状态机，截获标题后发 'T' 帧。可集中过滤/审计，但与零拷贝数据泵有张力 | |
| 混合双通道 | 前端解析 + 服务端 'T' 帧双通道冗余，两套逻辑需一致，复杂度最高 | |

**User's choice:** 纯前端解析

| Option (ro 前缀) | Description | Selected |
|--------|-------------|----------|
| 前缀恒前 | [ro] 恒在标题最前：[ro] {动态标题}；防远程内容顶掉只读标识 | ✓ |
| ro 时禁同步 | ro 模式下不同步动态标题，标题静态保持 [ro] wesh | |
| ro 后缀 | 动态标题在前，[ro] 后缀放最后 | |

**User's choice:** 前缀恒前（兑现 P2 D-14 挂账）

| Option (注入防护) | Description | Selected |
|--------|-------------|----------|
| 前端 sanitize | onTitleChange 钩子里剥 C0/C1 控制字符 + 截断 128 字符（PITFALLS 标题注入对策） | ✓ |
| 直通不防护 | xterm 给的标题直接写入 document.title，不额外处理 | |

**User's choice:** 前端 sanitize

| Option (品牌后缀) | Description | Selected |
|--------|-------------|----------|
| 纯同步 | document.title = {动态标题}（ro 时 [ro] {动态标题}），终端程序设什么就是什么 | ✓ |
| 带 - wesh 后缀 | document.title = {动态标题} - wesh，保留产品标识 | |

**User's choice:** 纯同步

---

## 超链接与 OSC 8 (FE-04)

| Option | Description | Selected |
|--------|-------------|----------|
| addon 默认正则 | http/https/ftp/mailto 等，xterm 官方维护的成熟平衡，不自定义收紧 | ✓ |
| 收紧 http/https | 仅 http/https，减少误识别面（终端输出里 ftp/mailto 链接极少） | |
| 宽松自定义 | 自定义宽松正则，覆盖更多形态（如 www. 开头无 scheme） | |

**User's choice:** addon 默认正则

| Option (OSC 8) | Description | Selected |
|--------|-------------|----------|
| 放行+hover 显真 | xterm 内建解析 OSC 8 显式链接，hover 展示真实 URL（ROADMAP 要求）——用户可辨别显示文本与目标不符的钓鱼链接 | ✓ |
| 禁用 OSC 8 | 前端不解析 OSC 8（或服务端剥离该序列）——最严安全姿态，但 git log/GitHub CLI 等正当 OSC 8 输出失效 | |
| 点击前确认 | 放行但点击 OSC 8 链接时弹确认框展示真实 URL | |

**User's choice:** 放行+hover 显真

| Option (点击行为) | Description | Selected |
|--------|-------------|----------|
| xterm 默认 | hover 高亮+tooltip，点击新窗口打开（target=_blank rel=noopener 锁定）；修饰键要求以 research 核实 xterm 官方默认为准 | ✓ |
| 强制修饰键+点击 | 必须按住 Ctrl/Cmd 点击才打开——防误触（终端单击常用于选择/定位光标） | |
| 单击直开 | 单击直接打开，与 ttyd 行为对齐 | |

**User's choice:** xterm 默认

| Option (序列过滤) | Description | Selected |
|--------|-------------|----------|
| 不做过滤 | 服务端不过滤任何 OSC 序列——数据泵零拷贝理念；OSC8 前端 hover 兜底、OSC52 addon 默认关闭兜底（过滤开关记 deferred） | ✓ |
| 提供过滤 flag | 新增 --filter-osc flag：服务端剥离 OSC52/OSC8 序列，面向高敏部署场景 | |
| 仅默认过滤 OSC52 | 默认剥离 OSC52（写剪贴板最危险），OSC8 保留 | |

**User's choice:** 不做过滤

---

## 剪贴板方案 (FE-05)

| Option | Description | Selected |
|--------|-------------|----------|
| 选中即复制 | onSelectionChange → navigator.clipboard.writeText，ttyd 行为对等，无需按键 | ✓ |
| 手动快捷键 | 不自动复制，仅 Ctrl+Shift+C 手动复制当前选中 | |
| 两者均启 | 选中即复制 + Ctrl+Shift+C 双通道 | |

**User's choice:** 选中即复制

| Option (粘贴形态) | Description | Selected |
|--------|-------------|----------|
| Ctrl+Shift+V | Ctrl+Shift+V → navigator.clipboard.readText → INPUT 帧；浏览器权限门控；ro 模式不触发读剪贴板（避免无谓权限弹窗） | ✓ |
| 右键粘贴 | 浏览器原生右键菜单粘贴（需自建 contextmenu 处理，xterm 默认无右键菜单） | |
| 双通道 | Ctrl+Shift+V + 右键双通道 | |

**User's choice:** Ctrl+Shift+V

| Option (HTTP 降级) | Description | Selected |
|--------|-------------|----------|
| 静默降级 | navigator.clipboard 仅安全上下文（HTTPS/localhost）可用——检测存在性，不可用时选中复制静默不生效，README 明示 | ✓ |
| 提示用户 | 不可用时在状态区/console 给出明显提示 | |
| 旧 API 兜底 | 不可用时尝试 execCommand('copy') 旧 API 兜底（已废弃，不推荐） | |

**User's choice:** 静默降级

| Option (OSC52 开关) | Description | Selected |
|--------|-------------|----------|
| CLI flag 控制 | 服务端 --osc52 flag（默认关）控制，经 Welcome prefs 下发启用；开启时 addon 只写不读（ROADMAP 锁定） | ✓ |
| URL query 启用 | 前端 URL query ?osc52=true 自主启用——但 query 可绕过服务端安全意图，不推荐 | |
| 双通道 | CLI flag + URL query 双通道（query 可覆盖 flag） | |

**User's choice:** CLI flag 控制

---

## 偏好下发与辅助交互 (FE-07+FE-06)

| Option | Description | Selected |
|--------|-------------|----------|
| Welcome 内嵌 | Welcome 帧加可选 prefs 字段（omitempty）——握手期原子到达、加字段不破坏协议（P2 D-02 纪律）；'P' 字节占位保留 | ✓ |
| 独立 'P' 帧 | 独立 'P' PREFS 帧（P2 占位字节兑现），Welcome 不变 | |
| 两者并存 | Welcome 内嵌初始值 + 'P' 帧用于运行期变更推送（v2 语义提前引入） | |

**User's choice:** Welcome 内嵌

| Option (白名单) | Description | Selected |
|--------|-------------|----------|
| xterm 选项+FE-06 | fontSize/fontFamily/cursorBlink/cursorStyle/scrollback/lineHeight/letterSpacing/theme + FE-06 两开关（resizeOverlay/confirmBeforeUnload）；防任意 option 注入（allowProposedApi 等） | ✓ |
| 仅 xterm 选项 | 仅 xterm 视觉选项；FE-06 开关独立 CLI flag / query，不走 PREFS | |
| 全量透传 | 无白名单全量透传 xterm options（对标 ttyd -t 全开放）——有注入危险 option 的风险 | |

**User's choice:** xterm 选项+FE-06

| Option (CLI flag) | Description | Selected |
|--------|-------------|----------|
| 报错 fail-fast | --client-option key=value 可重复（ttyd -t 等价）；key 不在白名单或值非法 JSON 均拒绝启动（P3 启动校验矩阵同风格） | ✓ |
| warn 跳过 | 同名 flag；非法项 stderr 警告跳过、继续启动 | |
| 换个 flag 名 | flag 名另选（--pref 或 --term-option），语义同上 | |

**User's choice:** 报错 fail-fast

| Option (query 覆盖) | Description | Selected |
|--------|-------------|----------|
| 白名单+优先级 | ?fontSize=16&cursorBlink=false；同白名单+JSON parse；非法静默忽略+console.warn；优先级 URL query > --client-option > 内置默认 | ✓ |
| 限外观类 | query 仅可覆盖外观类（fontSize/theme/cursorBlink），不可关 FE-06 安全交互（离开确认防误关兜底） | |
| 默认忽略 | query 覆盖需服务端显式开启（--allow-query-prefs），默认忽略全部 query | |

**User's choice:** 白名单+优先级

| Option (浮层默认) | Description | Selected |
|--------|-------------|----------|
| 默认开 | resize 期间右上角显示 COLSxROWS，停止后淡出（ttyd 先例，PITFALLS 推荐）；可经 PREFS/query 关 | ✓ |
| 默认关 | 默认不显示，经 PREFS/query 显式开启 | |

**User's choice:** 默认开

| Option (离开确认) | Description | Selected |
|--------|-------------|----------|
| 默认开 | beforeunload 拦截——当前单次语义下误关页面=会话终结（Phase 6 重连落地后仍保留开关）；ro 模式同启（查看中误关也终结） | ✓ |
| 仅 rw 启用 | 仅可写模式启用——ro 旁观会话误关损失小 | |
| 默认关 | 默认不拦截，经 PREFS/query 显式开启 | |

**User's choice:** 默认开

| Option (theme 表达) | Description | Selected |
|--------|-------------|----------|
| 完整 JSON 对象 | --client-option 'theme={"background":"#000"}'；对标 ttyd -t 全 JSON 值，灵活无预设维护负担；非法 JSON 启动报错 | ✓ |
| 预设主题名 | --client-option theme=dark|light 预设名（服务端映射到完整 theme 对象）——简单但 v1 仅单主题，预设库单薄 | |
| theme 不下发 | theme 不可下发（v1 锁定 tango 深色调色板），白名单剔除 theme | |

**User's choice:** 完整 JSON 对象

---

## Claude's Discretion

- addon 精确版本号（与 @xterm/xterm 6.0.0 同批次兼容版，research 定稿）
- onSelectionChange 写剪贴板的防抖细节
- web-links 修饰键要求（research 核实 xterm 官方默认）
- resize 浮层具体样式（UI-SPEC 阶段定稿）
- 标题 sanitize 截断策略细节（128 字符按 code point 计）
- Welcome prefs JSON schema 细节与键名映射
- 前端 query 解析与 term.options 运行时应用的装配顺序

## Deferred Ideas

- 'T' TITLE 帧实现 — Phase 5 多客户端场景再评估
- 'P' PREFS 帧运行期推送 — v2 语义
- 服务端 OSC 序列过滤开关（--filter-osc）— 高敏部署增强
- 多客户端旁观模式 OSC52 强制关 — Phase 5
- 移动端浏览器基础可用 — FEATURES.md 愿景项，未映射 phase
- 偏好持久化（localStorage）— 无需求支撑
