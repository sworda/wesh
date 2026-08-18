# Phase 4: 前端体验 - Context

**Gathered:** 2026-08-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 4 把前端达到并超越 ttyd 功能基线：Unicode 11/CJK/IME（FE-02）、超链接识别（FE-04）、现代剪贴板（FE-05）、终端标题同步浏览器标签页（CORE-03）、resize COLSxROWS 浮层 + 离开页面前确认（FE-06，均可开关）、服务端偏好下发 + URL query 覆盖（FE-07）。修掉 ttyd 的废弃 API（execCommand）与停更依赖；OSC52 剪贴板 addon 默认关闭、开启只写不读（ROADMAP 注，Warp CVE-2025-48725 教训）。

**In scope (from ROADMAP):** CORE-03（标题同步）、FE-02（Unicode 11 宽字符/CJK/IME）、FE-04（URL 自动识别可点击超链接）、FE-05（选中即复制 navigator.clipboard）、FE-06（resize 浮层 + 离开确认，均可开关）、FE-07（服务端下发 fontSize/theme 等偏好，URL query 覆盖）；`@xterm/addon-unicode11`/`addon-web-links`/`addon-clipboard` 接入。

**Out of scope (本阶段不做):** 多客户端 fan-out 与 ro/rw 分享链接（Phase 5）、断线自动重连（Phase 6 CORE-05）、EXIT 终结帧（Phase 6）、服务端 OSC 序列过滤开关（deferred）、移动端专项 UI、trzsz/ZMODEM/Sixel（v2）。

**已锁定不重复决策：** addon 组合与版本随 `@xterm/xterm` 6.0.0 同批次（STACK.md 定案）；`'T'`/`'P'` 类型字节协议层已占住（P2 D-01）；未知字段忽略纪律（P2 D-02）；前端文案全英文（P2 惯例）；FE-02 = 装 addon-unicode11 + IME 由 xterm composition 内部处理（main.ts:90 注释既有路线），字体走系统等宽栈 OS 回退（UI-SPEC），无灰色。

</domain>

<decisions>
## Implementation Decisions

### 标题同步（CORE-03）
- **D-01:** 标题走**纯前端解析**：xterm.js 默认解析 OSC 0/1/2 并触发 `onTitleChange`，前端订阅直写 `document.title`。服务端 OUTPUT 流保持零拷贝直发，不跑 OSC 状态机。`'T'` TITLE 帧占位保留不实现——Phase 5 多客户端场景再评估（新客户端 ring 重放含 OSC 序列可自然恢复标题）；ARCHITECTURE.md §2.8 的 S→C TITLE 帧设计语义让位于本决策
- **D-02:** ro 前缀融合（兑现 P2 D-14 挂账）：`[ro] ` 恒在标题最前——`[ro] {动态标题}`；rw 无前缀纯同步。防远程内容把只读标识顶掉（PITFALLS OSC 标题注入伪装主机名/路径对策）
- **D-03:** 标题注入防护在前端 sanitize：`onTitleChange` 钩子里剥离 C0/C1 控制字符 + 截断 128 字符后写 `document.title`
- **D-04:** 无品牌后缀：`document.title = {动态标题}`（ro 时 `[ro] {动态标题}`），终端程序设什么是什么

### 超链接（FE-04）
- **D-05:** `@xterm/addon-web-links` 保持**默认正则**（http/https/ftp/mailto 等，官方维护的成熟平衡），不自定义收紧或放宽
- **D-06:** OSC 8 显式超链接**放行**（xterm 内建解析）+ hover 展示真实 URL（ROADMAP 准则 1 要求）——用户可辨别"显示 github.com 点开 evil.com"的钓鱼面（PITFALLS C5）
- **D-07:** 点击打开行为以 xterm 官方默认为准（research 核实修饰键要求），新窗口 `target=_blank` + `rel=noopener` 为锁定项
- **D-08:** 服务端**不做** OSC 序列剥离/过滤——数据泵零拷贝理念；OSC 8 由前端 hover 兜底、OSC52 由 addon 默认关闭兜底。过滤开关记 deferred

### 剪贴板（FE-05）
- **D-09:** 选中即复制：`term.onSelectionChange` → `navigator.clipboard.writeText`（ttyd 行为对等，替代已废弃 execCommand）
- **D-10:** 粘贴形态 `Ctrl+Shift+V` → `navigator.clipboard.readText` → 写 INPUT 帧；浏览器权限门控；ro 模式不触发读剪贴板（避免无谓权限弹窗——ro 下 INPUT 本就被服务端丢弃）
- **D-11:** 明文 HTTP 非 localhost 下降级：`navigator.clipboard` 仅安全上下文可用——检测存在性，不可用时选中复制**静默不生效**（不弹错、不落旧 API），README 明示剪贴板需 HTTPS/localhost
- **D-12:** OSC52 开关 = CLI flag `--osc52`（布尔，默认关），经 Welcome prefs 下发前端启用 addon-clipboard；开启时**只写不读**（ROADMAP 锁定）。URL query 不可开启 OSC52（安全敏感项不允许用户侧绕过服务端意图） — **Reversibility:** one-way — CLI flag 是公开契约（P2 D-15 纪律）

### 偏好下发与辅助交互（FE-07 + FE-06）
- **D-13:** PREFS 走 **Welcome 帧内嵌**可选字段 `prefs`（omitempty）——握手期原子到达、加字段不破坏协议（P2 D-02 纪律）；前端 Welcome 处理时经 `term.options.*` 运行时应用并 `fit.fit()` 重算。`'P'` PREFS 帧字节占位保留（未来运行期变更推送语义，v2 再议）
- **D-14:** 可下发偏好**白名单** = `fontSize/fontFamily/cursorBlink/cursorStyle/scrollback/lineHeight/letterSpacing/theme`（8 个 xterm 视觉选项）+ `resizeOverlay/confirmBeforeUnload`（FE-06 两开关，非 xterm 选项，前端自定义键）。白名单制防任意 option 注入（allowProposedApi 等危险面）
- **D-15:** CLI flag = `--client-option key=value` 可重复（ttyd `-t` client-option 等价）；值 JSON parse。key 不在白名单或值非法 JSON 均**启动报错**（P3 启动校验矩阵同风格，显式优于静默） — **Reversibility:** one-way — flag 契约同上
- **D-16:** URL query 覆盖：`?fontSize=16&cursorBlink=false`——同白名单 + 值 JSON parse；非法 query **静默忽略 + console.warn**（用户侧输入不该让终端不可用）；优先级 **URL query > --client-option > 内置默认**。`osc52` 不在 query 白名单（D-12）
- **D-17:** resize 浮层默认**开**：resize 期间右上角显示 COLSxROWS，停止后淡出（ttyd 先例、PITFALLS 推荐）；经 PREFS/query 可关
- **D-18:** 离开页面前确认（beforeunload）默认**开**：当前单次语义下误关页面 = 会话终结；ro 模式同启（查看中误关也终结）；Phase 6 重连落地后仍保留开关。现代浏览器 beforeunload 自定义文案被忽略，用标准确认框
- **D-19:** theme 下发表达 = **完整 JSON 对象**（`--client-option 'theme={"background":"#000"}'`，对标 ttyd -t JSON 值）；无预设主题名库；非法 JSON 启动报错（D-15 同口径）

### Claude's Discretion
- addon 精确版本号（与 @xterm/xterm 6.0.0 同批次兼容版，research 定稿钉入 package.json）
- `onSelectionChange` 写剪贴板的防抖细节（拖动选择期间频次控制）
- web-links 修饰键要求（Ctrl/Cmd+Click vs 单击）以 research 核实 xterm 官方默认/推荐为准
- resize 浮层具体样式（右上角、半透明黑底白字、淡出差时）——UI-SPEC 阶段定稿
- 标题 sanitize 的具体截断策略（128 字符按 code point 计）
- Welcome prefs 的 JSON schema 细节（键名、嵌套 theme 对象传递形态）
- 前端 query 解析与 `term.options` 运行时应用的装配顺序（prefs 应用后 fit.fit() 时机）
- FE-06 两开关的前端键名映射（`resizeOverlay`/`confirmBeforeUnload` 为非 xterm 自定义键，服务端白名单需同时认 xterm 键与自定义键）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 4 — 成功准则 3 条（CJK/IME+超链接 hover / navigator.clipboard+标题同步 / 浮层+离开确认可开关+偏好下发 query 覆盖）与 OSC52 注（默认关闭、只写不读）
- `.planning/REQUIREMENTS.md` — CORE-03、FE-02、FE-04、FE-05、FE-06、FE-07 原文
- `.planning/PROJECT.md` — 约束（xterm.js 生态、单静态二进制、单 HTML 零运行时网络依赖）

### 调研结论
- `.planning/research/STACK.md` — addon 组合定案（unicode11/web-links/clipboard 与 xterm 6.0.0 同批次；execCommand 废弃用 navigator.clipboard）
- `.planning/research/PITFALLS.md` — Pitfall 5 / C5（OSC 52 剪贴板 Warp CVE-2025-48725、OSC 8 钓鱼链接、OSC 标题注入）；§反模式表（execCommand→navigator.clipboard、resize 浮层、只读界面提示、标题同步含主机名前缀）
- `.planning/research/ARCHITECTURE.md` §2.8 — TITLE/PREFS 帧的调研期设计（S→C TITLE 语义已被本文件 D-01 让位修订——纯前端解析）
- `.planning/research/FEATURES.md` — ttyd 前端功能基线对照

### 前序 phase 决策
- `.planning/phases/01-pty/01-UI-SPEC.md` — Terminal Options Contract（构造选项逐项钉死表，本 phase prefs 白名单的默认值来源）、Renderer Contract、无白闪契约、状态面板契约
- `.planning/phases/01-pty/01-CONTEXT.md` — D-13（前端栈锁定）、D-16（帧形状与类型空间预留）、D-18（go:embed 单 HTML）
- `.planning/phases/02-protocol/02-CONTEXT.md` — D-01（'T'/'P' 字节占位）、D-02（未知字段忽略=prefs 内嵌加字段依据）、D-07（Error JSON 形状）、D-14（ro 标题 [ro] 前缀挂账，本 phase D-02 兑现融合）
- `.planning/phases/03-auth/03-CONTEXT.md` — D-02（整站 Basic、前端连接流程现状）

### 现状代码（扩展点）
- `web/src/main.ts` — Terminal 构造选项（21-56 行）、onData/onResize 接线、connect() 连接流程、WELCOME 处理（ro 分支）、onclose 码值分派；本 phase 在此加 addon 加载、onTitleChange/onSelectionChange、快捷键、PREFS 应用
- `web/package.json` — 现有依赖（@xterm/xterm 6.0.0 + addon-fit + addon-webgl），本 phase 加三个 addon
- `internal/proto/proto.go` — 'T'/'P' 占位注释（25 行）；WelcomePayload 加 `prefs` 可选字段；本 phase 在此扩展
- `internal/server/server.go` — Welcome 组帧挂点（prefs 从配置注入）
- `cmd/wesh/main.go` — parseArgs 加 `--client-option`/`--osc52`；启动校验矩阵扩展（白名单/JSON 校验报错路径）

### ttyd 源码（缺陷对照面，不参考实现）
- `~/open_src/ttyd/` — execCommand('copy') 已废弃（FE-05 替代动因）；zmodem.js/decko 停更（PROJECT.md Context 节）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `term` Terminal 实例与构造选项表（main.ts:21-56）— prefs 白名单的默认值即此表现值；`term.options.*` 运行时赋值 xterm 原生支持
- `fit` FitAddon 实例（main.ts:58）— prefs 应用后 `fit.fit()` 重算尺寸复用既有接线
- `showStatus()` 三态面板（main.ts:120-136）— 本 phase 无新状态面板需求，保持不变
- `concat()`/帧常量（main.ts:12-18, 80-88）— 粘贴写 INPUT 帧复用 concat + INPUT 常量
- UI-SPEC Terminal Options Contract — prefs 白名单逐项的默认值契约来源

### Established Patterns
- **帧常量前后端手工对齐**（main.ts:6-11 ↔ proto.go，注释互相指路）— Welcome prefs 字段两侧同步加注释
- **未知字段忽略**（P2 D-02）— Welcome 加 prefs 是加字段不是动协议；前端旧版本无 prefs 处理也兼容
- **启动校验矩阵 fail-fast**（P3 main.go）— `--client-option` 非法 key/值进同矩阵报错路径
- **CLI flag 全名无短选项**（P2 D-15）— `--client-option`/`--osc52` 同纪律
- **零运行时网络依赖**（UI-SPEC 字体策略）— 禁 webfont；CJK 字形走 OS 字体回退，fontFamily 栈不变
- **ro 边界双层**（P2 D-13/D-14：服务端丢 INPUT 为真边界，前端 disableStdin 为 UX 层）— 粘贴 handler 的 ro 检查是第三层 UX 优化，不改变真边界

### Integration Points
- `web/src/main.ts` — addon 加载段（unicode11/web-links 紧随现有 webgl 加载段后）；`onTitleChange`/`onSelectionChange`/快捷键 handler；WELCOME 分支扩展 prefs 应用（ro 分支既有）；query 解析在 Terminal 构造前读取（构造选项初值 = 内置默认 ← query 覆盖先行，prefs 到达后再覆盖）
- `internal/proto/proto.go` — WelcomePayload 加 `Prefs json.RawMessage` 或具体 map 类型（omitempty）；白名单键常量表
- `internal/server/server.go` — Welcome 组帧处注入 prefs（配置 → Welcome）
- `cmd/wesh/main.go` — `--client-option`（可重复 flag，key=value 解析+白名单+JSON 校验）与 `--osc52`（布尔）；配置结构体扩展；OSC52 开启时并入 prefs 下发
- `web/dist/index.html` 重建入库（构建链既有：`pnpm -C web build` → `go build`）

</code_context>

<specifics>
## Specific Ideas

- **OSC52 与 query 的安全不对称**：D-12/D-16 明确 `osc52` 只能经 CLI flag 由服务端开启，URL query 不可触碰——安全敏感项不允许用户侧输入绕过服务端意图；其余外观类偏好 query 自由覆盖
- **选中即复制的安全上下文依赖**：D-11 静默降级 + README 明示是刻意取舍——明文 HTTP 是反代终止 TLS 的常态部署（P3 D-04），不该为剪贴板弹错打扰终端主流程
- **ro 模式粘贴不触发读剪贴板**（D-10）：ro 下 INPUT 必被服务端丢弃，读剪贴板只会换来一次无谓的浏览器权限弹窗
- **离开确认的 Phase 语境**（D-18）：当前服务端仍是单次语义（P1 D-11：WS 断开即退出），误关页面 = 会话终结——默认开的依据；Phase 6 重连落地后误关代价降低，但开关保留
- **theme 无预设库**（D-19）：v1 单深色调色板现状下预设名单薄，完整 JSON 对象对标 ttyd -t 且零维护负担

</specifics>

<deferred>
## Deferred Ideas

- **'T' TITLE 帧实现** — Phase 5 多客户端场景再评估（D-01：新客户端 ring 重放含 OSC 序列可自然恢复标题，可能永不需要）
- **'P' PREFS 帧运行期推送** — v2 语义（连接存续期间服务端主动变更偏好）；本 phase 仅 Welcome 内嵌一次性下发
- **服务端 OSC 序列过滤开关**（--filter-osc 剥离 OSC52/OSC8）— 高敏部署场景增强；本 phase D-08 由前端 hover + addon 默认关兜底
- **多客户端旁观模式 OSC52 强制关** — Phase 5（PITFALLS C5 对策的另一半）
- **移动端浏览器基础可用**（可读+滚动）— FEATURES.md 愿景项，未映射 phase；非本 phase 范围
- **偏好持久化**（localStorage 记忆用户 query 选择）— 无需求支撑，roadmap 未列

</deferred>

---

*Phase: 4-frontend*
*Context gathered: 2026-08-18*
