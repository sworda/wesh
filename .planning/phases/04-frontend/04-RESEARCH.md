# Phase 4: 前端体验 - Research

**Researched:** 2026-08-18
**Domain:** xterm.js 6.0.0 addon 生态（unicode11/web-links/clipboard）+ 浏览器 Clipboard/beforeunload API + Go 协议/CLI 扩展
**Confidence:** HIGH（全部关键 API 事实均从 npm 发布产物源码与已装 @xterm/xterm 6.0.0 产物逐行核实；版本/合法性经 npm registry + legitimacy seam 核实；浏览器 API 行为引 MDN 原文）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**标题同步（CORE-03）**
- **D-01:** 标题走**纯前端解析**：xterm.js 默认解析 OSC 0/1/2 并触发 `onTitleChange`，前端订阅直写 `document.title`。服务端 OUTPUT 流保持零拷贝直发，不跑 OSC 状态机。`'T'` TITLE 帧占位保留不实现——Phase 5 多客户端场景再评估（新客户端 ring 重放含 OSC 序列可自然恢复标题）；ARCHITECTURE.md §2.8 的 S→C TITLE 帧设计语义让位于本决策
- **D-02:** ro 前缀融合（兑现 P2 D-14 挂账）：`[ro] ` 恒在标题最前——`[ro] {动态标题}`；rw 无前缀纯同步。防远程内容把只读标识顶掉（PITFALLS OSC 标题注入伪装主机名/路径对策）
- **D-03:** 标题注入防护在前端 sanitize：`onTitleChange` 钩子里剥离 C0/C1 控制字符 + 截断 128 字符后写 `document.title`
- **D-04:** 无品牌后缀：`document.title = {动态标题}`（ro 时 `[ro] {动态标题}`），终端程序设什么是什么

**超链接（FE-04）**
- **D-05:** `@xterm/addon-web-links` 保持**默认正则**（http/https/ftp/mailto 等，官方维护的成熟平衡），不自定义收紧或放宽
- **D-06:** OSC 8 显式超链接**放行**（xterm 内建解析）+ hover 展示真实 URL（ROADMAP 准则 1 要求）——用户可辨别"显示 github.com 点开 evil.com"的钓鱼面（PITFALLS C5）
- **D-07:** 点击打开行为以 xterm 官方默认为准（research 核实修饰键要求），新窗口 `target=_blank` + `rel=noopener` 为锁定项
- **D-08:** 服务端**不做** OSC 序列剥离/过滤——数据泵零拷贝理念；OSC 8 由前端 hover 兜底、OSC52 由 addon 默认关闭兜底。过滤开关记 deferred

**剪贴板（FE-05）**
- **D-09:** 选中即复制：`term.onSelectionChange` → `navigator.clipboard.writeText`（ttyd 行为对等，替代已废弃 execCommand）
- **D-10:** 粘贴形态 `Ctrl+Shift+V` → `navigator.clipboard.readText` → 写 INPUT 帧；浏览器权限门控；ro 模式不触发读剪贴板（避免无谓权限弹窗——ro 下 INPUT 本就被服务端丢弃）
- **D-11:** 明文 HTTP 非 localhost 下降级：`navigator.clipboard` 仅安全上下文可用——检测存在性，不可用时选中复制**静默不生效**（不弹错、不落旧 API），README 明示剪贴板需 HTTPS/localhost
- **D-12:** OSC52 开关 = CLI flag `--osc52`（布尔，默认关），经 Welcome prefs 下发前端启用 addon-clipboard；开启时**只写不读**（ROADMAP 锁定）。URL query 不可开启 OSC52（安全敏感项不允许用户侧绕过服务端意图） — **Reversibility:** one-way — CLI flag 是公开契约（P2 D-15 纪律）

**偏好下发与辅助交互（FE-07 + FE-06）**
- **D-13:** PREFS 走 **Welcome 帧内嵌**可选字段 `prefs`（omitempty）——握手期原子到达、加字段不破坏协议（P2 D-02 纪律）；前端 Welcome 处理时经 `term.options.*` 运行时应用并 `fit.fit()` 重算。`'P'` PREFS 帧字节占位保留（未来运行期变更推送语义，v2 再议）
- **D-14:** 可下发偏好**白名单** = `fontSize/fontFamily/cursorBlink/cursorStyle/scrollback/lineHeight/letterSpacing/theme`（8 个 xterm 视觉选项）+ `resizeOverlay/confirmBeforeUnload`（FE-06 两开关，非 xterm 选项，前端自定义键）。白名单制防任意 option 注入（allowProposedApi 等危险面）
- **D-15:** CLI flag = `--client-option key=value` 可重复（ttyd `-t` client-option 等价）；值 JSON parse。key 不在白名单或值非法 JSON 均**启动报错**（P3 启动校验矩阵同风格，显式优于静默） — **Reversibility:** one-way — flag 契约同上
- **D-16:** URL query 覆盖：`?fontSize=16&cursorBlink=false`——同白名单 + 值 JSON parse；非法 query **静默忽略 + console.warn**（用户侧输入不该让终端不可用）；优先级 **URL query > --client-option > 内置默认**。`osc52` 不在 query 白名单（D-12）
- **D-17:** resize 浮层默认**开**：resize 期间右上角显示 COLSxROWS，停止后淡出（ttyd 先例、PITFALLS 推荐）；经 PREFS/query 可关
- **D-18:** 离开页面前确认（beforeunload）默认**开**：当前单次语义下误关页面 = 会话终结；ro 模式同启（查看中误关也终结）；Phase 6 重连落地后仍保留开关。现代浏览器 beforeunload 自定义文案被忽略，用标准确认框
- **D-19:** theme 下发表达 = **完整 JSON 对象**（`--client-option 'theme={"background":"#000"}'`，对标 ttyd -t JSON 值）；无预设主题名库；非法 JSON 启动报错（D-15 同口径）

### Claude's Discretion
- addon 精确版本号（与 @xterm/xterm 6.0.0 同批次兼容版，research 定稿钉入 package.json）→ **已定稿：0.9.0 / 0.12.0 / 0.2.0（UI-SPEC §Addon Contract，本研究核实一致）**
- `onSelectionChange` 写剪贴板的防抖细节（拖动选择期间频次控制）→ UI-SPEC 定稿 150ms trailing debounce
- web-links 修饰键要求（Ctrl/Cmd+Click vs 单击）→ **research 核实：单击无修饰键（见 §Link Contract 核实）**
- resize 浮层具体样式 → UI-SPEC §Resize Overlay Spec 定稿
- 标题 sanitize 的具体截断策略 → UI-SPEC 定稿（剥离 C0/DEL/C1 + Array.from 128 code point）
- Welcome prefs 的 JSON schema 细节 → 本研究 §Prefs Schema 给出建议形状
- 前端 query 解析与 `term.options` 运行时应用的装配顺序 → UI-SPEC §Prefs Contract 定稿
- FE-06 两开关的前端键名映射 → UI-SPEC 定稿 `resizeOverlay`/`confirmBeforeUnload`

### Deferred Ideas (OUT OF SCOPE)
- **'T' TITLE 帧实现** — Phase 5 多客户端场景再评估
- **'P' PREFS 帧运行期推送** — v2 语义；本 phase 仅 Welcome 内嵌一次性下发
- **服务端 OSC 序列过滤开关**（--filter-osc 剥离 OSC52/OSC8）— 高敏部署场景增强
- **多客户端旁观模式 OSC52 强制关** — Phase 5
- **移动端浏览器基础可用**（可读+滚动）— 未映射 phase
- **偏好持久化**（localStorage 记忆用户 query 选择）— 无需求支撑
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CORE-03 | 终端标题变化同步到浏览器标签页标题 | §Pattern 1（onTitleChange 纯前端解析 + sanitize + 单一写口）；核实 onTitleChange 实际触发范围为 OSC 0/2（§Pitfall 6） |
| FE-02 | Unicode 11 宽字符支持，CJK/IME 正常输入显示 | §Pattern 2（Unicode11Addon 加载 + activeVersion='11' 硬顺序）；IME 走 xterm 内建 composition 不改输入链路 |
| FE-04 | 终端输出中的 URL 自动识别为可点击超链接 | §Pattern 3（WebLinksAddon 默认正则/handler 逐字核实 + OSC8 linkHandler 必须显式设置否则 confirm() 弹窗） |
| FE-05 | 选中即复制，navigator.clipboard 现代 API | §Pattern 4（onSelectionChange 防抖 + 安全上下文检测 + Ctrl+Shift+V paste 链路 + OSC52 write-only provider） |
| FE-06 | resize COLSxROWS 浮层、离开页面前确认（均可开关） | §Pattern 5（onResize 浮层时序 + beforeunload preventDefault/sticky activation/移除时机） |
| FE-07 | 客户端偏好服务端下发，URL query 可覆盖 | §Pattern 6（Welcome prefs 通道 + 白名单 11 键 + 装配顺序 + **theme 运行时替换语义陷阱**，§Pitfall 3） |
</phase_requirements>

## Summary

本 phase 的全部外部依赖（三个 `@xterm` addon）已逐包核实：版本与 @xterm/xterm 6.0.0 同批次（2025-12-22 发布、同 commit f447274）、legitimacy 全 OK、零 postinstall，且 npm 产物内含完整 TypeScript 源码——UI-SPEC §Addon Contract 的全部 API 声明（activeVersion 激活、默认正则、默认 handler、provider 构造位）均与产物源码逐字一致。CONTEXT 委托项中"web-links 修饰键"已核实为**单击无修饰键**（核心 mousedown 记录 + mouseup 同链接判定后 activate，路径上无 ctrlKey/metaKey 检查），UI-SPEC 结论正确。

研究新增三个 UI-SPEC/CONTEXT 未覆盖或需修正的实现级发现：(1) **OSC8 必须设置 `linkHandler`**——核心缺省回退是 `confirm()` 原生警告框（产物源码与 d.ts 双重核实），UI-SPEC 已正确要求；(2) **`term.options.theme` 运行时替换是"逐键回退 xterm 内建默认"语义**——部分 theme 对象会把 wesh tango 调色板未指定键重置为 xterm 默认而非保留现值，D-19"完整 JSON 对象整体替换"在字面上成立但与 ttyd 同样有 surprising 行为，建议前端合并覆盖当前调色板后再赋值（§Pitfall 3，planner 决策点）；(3) **write-only clipboard provider 的 readText 应 resolve 空串而非 reject**——核心异步 OSC handler 链对 rejected promise 是 rethrow 形态（unhandled rejection 噪音），resolve '' 同等安全且协议完整（§Pitfall 4，与 UI-SPEC "恒 reject" 字面不同、安全姿态相同）。

**Primary recommendation:** 按 §Standard Stack 钉入三个 addon 精确版本；前端六个能力按 §Architecture Patterns 的六个 Pattern 接线；Go 侧 WelcomePayload 加 `Prefs json.RawMessage` omitempty 字段 + `--client-option`/`--osc52` 两个 flag 走既有 fs.Func/启动校验矩阵形态；两处研究修正项（theme 合并、readText resolve ''）在 plan 阶段定稿。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 标题同步（CORE-03） | Browser / Client | — | D-01：xterm 内建 OSC 解析 → document.title；服务端 OUTPUT 零拷贝不跑 OSC 状态机 |
| Unicode 宽度测量（FE-02） | Browser / Client | — | 渲染层测量问题，addon-unicode11 注册 provider；字形渲染走 OS 字体回退 |
| URL 识别与打开（FE-04） | Browser / Client | — | 终端内容数据面；服务端 D-08 不过滤 |
| 剪贴板读写（FE-05） | Browser / Client | API / Backend（仅 `--osc52` 开关下发） | navigator.clipboard 安全上下文门控在浏览器；OSC52 启用意图属服务端（D-12） |
| 偏好下发通道（FE-07） | API / Backend | Browser / Client | Welcome 帧内嵌 prefs 握手期原子到达（D-13）；前端负责应用与 query 覆盖 |
| 偏好校验与白名单（FE-07） | API / Backend | — | `--client-option` key/value 合法性启动期 fail-fast（D-15），P3 校验矩阵同风格 |
| resize 浮层 / 离开确认（FE-06） | Browser / Client | — | 纯前端辅助交互，开关状态由 prefs/query 驱动 |

## Standard Stack

### Core（本 phase 新增）

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@xterm/addon-unicode11` | `0.9.0` [VERIFIED: npm registry] | Unicode 11 宽字符测量（CJK/emoji 正确占宽） | 官方 addon，FE-02 对等项；2025-12-22 与 xterm 6.0.0 同批发布（同 commit f447274），680k 下载/周 |
| `@xterm/addon-web-links` | `0.12.0` [VERIFIED: npm registry] | 终端文本 URL 自动识别为可点击链接 | 官方 addon，FE-04 对等项；同批次发布，1.0M 下载/周；默认正则/handler 经产物源码逐字核实（§Pattern 3） |
| `@xterm/addon-clipboard` | `0.2.0` [VERIFIED: npm registry] | OSC52 剪贴板序列支持（默认不加载） | 官方 addon，D-12 开关项；同批次发布，222k 下载/周；provider 注入点经产物源码核实（§Pattern 4） |

### Supporting（既有，不变动）

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@xterm/xterm` | `6.0.0`（既有） | 终端模拟器核心 | onTitleChange/onSelectionChange/paste/options/unicode/linkHandler 均由此出 |
| `@xterm/addon-fit` | `0.11.0`（既有） | 容器自适应 | prefs 应用后 `fit.fit()` 重算（D-13） |
| `@xterm/addon-webgl` | `0.19.0`（既有） | WebGL 渲染 | 不变 |
| `js-base64` | `^3.7.5`（addon-clipboard 传递依赖） | OSC52 base64 编解码 | 仅随 addon-clipboard 安装；见 §Package Legitimacy Audit 的 SUS 说明与钉版建议 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| addon-web-links 默认正则 | 自定义 urlRegex 放宽 ftp/mailto | D-05 锁定不自定义；且 0.12.0 默认正则实为 **仅 http(s)**（§Pitfall 1 注），放宽需自维护正则 |
| addon-clipboard | 自写 OSC52 parser handler | 重复造轮子；官方 addon 的 base64/选择缓冲语义已处理（含非法 base64 清空语义），只需换 provider |
| navigator.clipboard | execCommand('copy') | 已废弃（MDN 明示 deprecated），ttyd 技术债，不采用 |

**Installation:**
```bash
pnpm -C web add @xterm/addon-unicode11@0.9.0 @xterm/addon-web-links@0.12.0 @xterm/addon-clipboard@0.2.0
```

**Version verification:** 三包均经 `npm view <pkg> version` + legitimacy seam 核实（2026-08-18）；`pnpm-lock.yaml` 当前无此三包（新增）。

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `@xterm/addon-unicode11@0.9.0` | npm | 2025-12-22 发布（~8 月） | 680,666/wk | github.com/xtermjs/xterm.js | OK | Approved |
| `@xterm/addon-web-links@0.12.0` | npm | 2025-12-22 发布（~8 月） | 1,016,407/wk | github.com/xtermjs/xterm.js | OK | Approved |
| `@xterm/addon-clipboard@0.2.0` | npm | 2025-12-22 发布（~8 月） | 222,648/wk | github.com/xtermjs/xterm.js | OK | Approved |
| `js-base64`（传递依赖，`^3.7.5`） | npm | 最新 3.9.3 发布于 2026-08-17（**1 天前**） | 11,948,669/wk | github.com/dankogai/js-base64 | SUS（仅 "too-new" 单信号） | Flagged — 见下 |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** `js-base64`——十年老包、周下载 1200 万、无 postinstall、verdict 的唯一信号是"latest 版本太新"。它是 addon-clipboard 的传递依赖而非直接安装项，`^3.7.5`  semver 会解析到最新的 3.9.3（发布仅 1 天）。**建议**（非强制）：在 `web/package.json` 加 `pnpm.overrides: { "js-base64": "3.9.2" }` 钉到 2026-07-27 发布的 3.9.2，避开 1 天新包；planner 若不采纳 override，则按协议在 install 前加 `checkpoint:human-verify`。postinstall 脚本审查：三包直连依赖 `scripts.postinstall` 均空（npm view 核实）。

## Architecture Patterns

### System Architecture Diagram

```
浏览器标签页                                    wesh 服务端
┌─────────────────────────────────────────────┐  ┌──────────────────────────┐
│ location.search ──► query 解析(白名单+JSON) │  │                          │
│                     │构造初值(+queryKeys集)  │  │ --client-option k=v(重复) │
│                     ▼                       │  │ --osc52 (bool)            │
│              new Terminal(options)          │  │   │parse期校验(白名单/JSON)│
│                     │                       │  │   ▼启动报错 exit 2        │
│  loadAddon: fit → webgl → unicode11         │  │ cfg.clientPrefs + osc52   │
│    (+activeVersion='11') → web-links        │  │   │                         │
│                     │                       │  │   ▼ server.Options        │
│  term.open(#terminal)（空黑=加载态）        │  │ Attach 升档序列尾段:       │
│                     │                       │  │ WelcomeFrame(mode, prefs) │
│  connect(): fetch /api/attach → WS          │  └───────────┬──────────────┘
│                     │ WELCOME {mode,prefs?} │              │ S→C
│                     ▼                       │◄─────────────┘
│  WELCOME 分支: mode→isRO/disableStdin       │
│    prefs 逐项: 跳过 queryKeys →             │
│    term.options.* 应用 → fit.fit()          │
│    resizeOverlay/confirmBeforeUnload→开关   │
│    osc52===true → loadAddon(ClipboardAddon) │
│                     │                       │
│  运行期事件流:                              │
│  OSC0/2 ─► onTitleChange ─► sanitize ─► document.title(([ro] )+title)
│  OUTPUT ─► term.write ─► (web-links 链接化 / OSC8 linkHandler)
│  选区变化 ─► onSelectionChange ─►150ms防抖─► clipboard.writeText
│  Ctrl+Shift+V ─► clipboard.readText ─► term.paste ─► onData ─► INPUT帧
│  onResize ─► RESIZE帧 + resize浮层(600ms静止后淡出)
│  页面关闭 ─► beforeunload(默认开, WS close后移除)
└─────────────────────────────────────────────┘
```

### Recommended Project Structure（增量）

```
web/src/
├── main.ts            # 既有，本 phase 在此扩展：addon 加载段、事件接线、WELCOME prefs 分支、query 解析
└── （可选）lib/        # planner 选项：抽纯函数（sanitizeTitle/parseQuery/applyPrefs）便于 node --test（§Validation Architecture Wave 0）
internal/proto/proto.go    # WelcomePayload + Prefs 字段、白名单键常量
internal/server/server.go  # Options + ClientPrefs；Welcome 组帧注入
cmd/wesh/main.go           # --client-option / --osc52 flag + 校验
```

### Pattern 1: 标题同步单一写口（CORE-03）
**What:** `term.onTitleChange` → sanitize → 唯一函数写 `document.title`，ro 前缀恒在最前。
**When to use:** 替代 Phase 3 在 WELCOME 分支的一次性 `document.title = '[ro] ' + document.title` 拼前缀写法（main.ts:204，标题变化时前缀会丢）。
**Example:**
```typescript
// Source: 04-UI-SPEC §Title Sync Contract（逐字契约）+ xterm.d.ts:1003 onTitleChange
const sanitizeTitle = (raw: string): string => {
  // 剥离 C0(U+0000-001F)/DEL(U+007F)/C1(U+0080-009F)，按 code point 截断 128（Array.from 不拆 surrogate pair）
  const stripped = Array.from(raw).filter((ch) => {
    const cp = ch.codePointAt(0)!;
    return !(cp <= 0x1f || cp === 0x7f || (cp >= 0x80 && cp <= 0x9f));
  });
  return stripped.slice(0, 128).join('') || 'wesh'; // 空串回退静态默认，不清空标签页标题
};
const setTitle = (raw: string): void => {
  document.title = (isRO ? '[ro] ' : '') + sanitizeTitle(raw);
};
term.onTitleChange(setTitle); // ro 初始态 WELCOME 到达时 setTitle('wesh')
```
**核实注：** `onTitleChange` 实际由 OSC 0 与 OSC 2 触发（d.ts: "when an OSC 0 or OSC 2 title change occurs"；产物 lib 中 OSC 0 handler `setTitle+setIconName`、OSC 2 handler `setTitle`，`setTitle` 内 `onTitleChange.fire`）；**OSC 1 只设 icon name 不触发**——CONTEXT D-01 "OSC 0/1/2" 的表述按此修正理解，真实世界标题程序（bash PROMPT_COMMAND、tmux、vim）均用 OSC 0/2，行为等价 [VERIFIED: node_modules/@xterm/xterm/lib/xterm.js 产物 + typings/xterm.d.ts:999-1003]。

### Pattern 2: Unicode 11 加载-激活硬顺序（FE-02）
**What:** `loadAddon(new Unicode11Addon())` 只注册 provider，**必须随后** `term.unicode.activeVersion = '11'`。
**Example:**
```typescript
// Source: addon-unicode11@0.9.0 src/Unicode11Addon.ts:12-17（activate 仅 terminal.unicode.register(new UnicodeV11())）
// + Context7 /xtermjs/xterm.js（addon-unicode11 README 同形态）
term.loadAddon(new Unicode11Addon());
term.unicode.activeVersion = '11'; // setter 对未注册版本抛 `unknown Unicode version "11"`——顺序不可颠倒
```
**核实注：** activeVersion setter 抛错行为核实自产物 lib（`set activeVersion(e){if(!this._providers[e])throw new Error(...)`）[VERIFIED: node_modules/@xterm/xterm/lib/xterm.js]。IME 组合输入不引入 addon：xterm 内部 textarea composition 既有处理，`onData` 交付最终字符串（main.ts:90 既有路线，本 phase 不改输入链路）。

### Pattern 3: 超链接双通道（FE-04）
**What:** web-links addon（文本 URL 正则识别）+ 核心 OSC 8 linkHandler（显式超链接），统一 hover tooltip 展示真实 URL。
**Example:**
```typescript
// Source: addon-web-links@0.12.0 src/WebLinksAddon.ts:21,24-36,42-46 + 04-UI-SPEC §Link Contract
// 不传自定义 handler——库默认 handleLink 即 window.open()+opener=null+location.href（等价 target=_blank rel=noopener）
term.loadAddon(new WebLinksAddon(undefined, {
  hover: (event, text, _location) => showLinkTooltip(event, text), // text 即 URL 本身
  leave: () => hideLinkTooltip(),
}));
// OSC 8 必须显式设 linkHandler——不设则核心回退 confirm("Do you want to navigate to …? WARNING: …") 原生框
term.options.linkHandler = {
  activate: (_event, uri) => {
    const w = window.open();
    if (w) { w.opener = null; w.location.href = uri; } // 与库默认 handleLink 同形态
  },
  hover: (event, text, _range) => showLinkTooltip(event, text), // text 为真实 uri（可与显示文本不同——钓鱼辨别点）
  leave: () => hideLinkTooltip(),
  // allowNonHttpProtocols 保持默认 false——javascript:/file: 等 OSC8 被结构性忽略
};
```
**核实注（逐字）：**
- 默认正则（WebLinksAddon.ts:21）：`/(https?|HTTPS?):[/]{2}[^\s"'!*(){}|\\\^<>`]*[^\s"':,.!?{}|\\\^~\[\]`()<>]/` —— **仅 http/https**，不匹配 ftp/mailto；UI-SPEC §Link Contract 已按此修正 D-05 描述预期 [VERIFIED: /tmp/wesh-addon-verify/xterm-addon-web-links-0.12.0/package/src/WebLinksAddon.ts:21]
- 激活 = 单击无修饰键：核心 `_handleMouseUp` 判定 mousedown 与 mouseup 同链接同位置后 `activate(e, text)`，路径无 ctrlKey/metaKey 检查 [VERIFIED: node_modules/@xterm/xterm/lib/xterm.js 产物]
- confirm() 回退（d.ts:152-163 原文）："The handler for OSC 8 hyperlinks. Links will use the `confirm` browser API with a strongly worded warning if no link handler is set." [VERIFIED: node_modules/@xterm/xterm/typings/xterm.d.ts:152-163]
- tooltip div 加 `xterm-hover` class——核心 hover 路径对该 class 的元素提前 return，防 tooltip 自身触发链接 enter/leave 抖动 [VERIFIED: node_modules/@xterm/xterm/lib/xterm.js 产物]

### Pattern 4: 剪贴板三形态（FE-05）
**What:** 选中即复制（防抖写）+ Ctrl+Shift+V 粘贴（term.paste 走既有 onData→INPUT）+ OSC52（write-only provider，默认不加载 addon）。
**Example:**
```typescript
// Source: 04-UI-SPEC §Clipboard Contract + MDN Clipboard API + addon-clipboard@0.2.0 src/ClipboardAddon.ts:14-17
const clipboardOK = typeof navigator.clipboard !== 'undefined'; // 安全上下文检测（D-11），缺失则整体静默
// ① 选中即复制：150ms trailing debounce + 空选区/重复内容不写
let selTimer: number | undefined;
let lastCopied = '';
term.onSelectionChange(() => {
  if (!clipboardOK) return;
  clearTimeout(selTimer);
  selTimer = window.setTimeout(() => {
    const text = term.getSelection();
    if (text === '' || text === lastCopied) return;
    lastCopied = text;
    navigator.clipboard.writeText(text).catch((e) => console.warn('clipboard write failed', e)); // 静默（D-11）
  }, 150);
});
// ② 粘贴：ro 不读剪贴板（D-10 无谓权限弹窗）；term.paste 保留 bracketed paste 语义走既有 onData→INPUT 链路
window.addEventListener('keydown', (e) => {
  if (!clipboardOK || isRO) return;
  if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'v') {
    e.preventDefault();
    navigator.clipboard.readText().then((t) => term.paste(t)).catch((err) => console.warn('clipboard read failed', err));
  }
});
// ③ OSC52：仅 Welcome prefs osc52===true 时加载；provider 是构造第二参
if (prefs.osc52 === true) {
  const writeOnly = {
    readText: (_sel: unknown): Promise<string> => Promise.resolve(''), // 见 §Pitfall 4：resolve '' 而非 reject
    writeText: (_sel: unknown, text: string): Promise<void> => navigator.clipboard.writeText(text),
  };
  term.loadAddon(new ClipboardAddon(undefined, writeOnly));
}
```
**核实注：** ClipboardAddon 构造签名 `(base64?: IBase64, provider?: IClipboardProvider)`——provider 为**第二参**（ClipboardAddon.ts:14-17）；activate 注册 `parser.registerOscHandler(52, …)`；核心 xterm 6.0.0 **无内建 OSC52 handler**（产物 lib 无 `]52;` 处理路径）——addon 不加载则 OSC52 惰性无害，"默认关即兜底"成立 [VERIFIED: /tmp/wesh-addon-verify/xterm-addon-clipboard-0.2.0/package/src/ClipboardAddon.ts + node_modules/@xterm/xterm/lib/xterm.js]。`term.paste(data)` 文档语义 "performing the necessary transformations for pasted text"，产物中 paste 路径检查 `bracketedPasteMode` 后 `triggerDataEvent`——即走既有 onData 链路且 bracketed paste 保留 [VERIFIED: node_modules/@xterm/xterm/typings/xterm.d.ts:1270-1275 + lib/xterm.js]。

### Pattern 5: 辅助交互两开关（FE-06）
**What:** resize 浮层（onResize 驱动，600ms 静止后 200ms 淡出）+ beforeunload（preventDefault 形态，WS close 任意路径移除）。
**Example:**
```typescript
// Source: 04-UI-SPEC §Resize Overlay Spec / §Auxiliary Interactions + MDN beforeunload_event
// resize 浮层：仅 WELCOME 处理完成后响应（onopen 初次 fit 不触发——浮层是会话辅助不是启动尺寸指示器）
term.onResize(({ cols, rows }) => {
  sendResize(cols, rows); // 既有链路
  if (!welcomeDone || !resizeOverlayOn) return;
  overlay.textContent = `${cols}x${rows}`; // 服务端钳制 1000×1000 → 最长 9 字符无溢出
  overlay.style.opacity = '1';
  clearTimeout(overlayTimer);
  overlayTimer = window.setTimeout(() => { overlay.style.opacity = '0'; }, 600); // transition 200ms ease
});
// beforeunload：preventDefault 触发浏览器标准框（自定义文案被现代浏览器忽略，不写）
const onBeforeUnload = (e: BeforeUnloadEvent): void => { e.preventDefault(); };
if (confirmBeforeUnloadOn) window.addEventListener('beforeunload', onBeforeUnload);
// WS onclose 任意路径（含状态面板展示后）移除——"Session ended" 后关页不再被拦截
```
**核实注（MDN 原文要点）：** "best practice is to trigger the dialog by invoking `preventDefault()`"；"Only show a generic browser-specified string … cannot be controlled by the webpage code"；**sticky activation 要求**——"the browser will only show the dialog box if the frame … receives a user gesture or user interaction"（用户零交互直接关页**不弹框**——对终端场景可接受：零交互即无会话投入，见 §Open Questions 3）；移动端不可靠触发（不属本 phase 范围）[CITED: developer.mozilla.org/en-US/docs/Web/API/Window/beforeunload_event]。

### Pattern 6: 偏好下发与装配顺序（FE-07）
**What:** query 先于构造解析（记 queryKeys）→ WELCOME prefs 逐项应用（跳过 queryKeys）→ `fit.fit()` 重算。
**Example:**
```typescript
// Source: 04-UI-SPEC §Prefs Contract（逐字契约）——装配顺序为定稿项
// ① 构造前：白名单键 JSON.parse，成功记 queryKeys 并作构造初值；非法静默忽略 + console.warn（D-16）
const XTERM_PREFS = ['fontSize','fontFamily','cursorBlink','cursorStyle','scrollback','lineHeight','letterSpacing','theme'] as const;
const BEHAVIOR_PREFS = ['resizeOverlay','confirmBeforeUnload'] as const; // 非 xterm 键，写前端开关不写 term.options
// ② WELCOME 到达：for key of XTERM_PREFS: if key in queryKeys skip; else term.options[key] = prefs[key]
//    theme 见 §Pitfall 3——建议合并当前调色板后整体赋值
// ③ 全部应用完 fit.fit() → 既有 onResize→RESIZE 帧自动同步服务端（D-13）
// ④ BEHAVIOR_PREFS 写开关；osc52===true 触发 Pattern 4③
```
**Go 侧通道（逐字契约建议形状）：**
```go
// internal/proto/proto.go — WelcomePayload 加字段（P2 D-02 加字段纪律，omitempty 缺席即无 prefs）
type WelcomePayload struct {
	Mode  string          `json:"mode"`
	Prefs json.RawMessage `json:"prefs,omitempty"` // 服务端 --client-option 聚合 + osc52 并入；原样透传不解析
}
// WelcomeFrame(mode string, prefs json.RawMessage) — prefs 为空时 JSON 不出 prefs 键（旧前端零漂移）
```
**核实注：** `term.options` 为可变属性（xterm.d.ts:899 `options: ITerminalOptions;`），d.ts 文档含多键赋值示例 `terminal.options = { fontSize: 12, … }`；运行时逐键赋值经 OptionsService 通知各订阅方（产物中 `onSpecificOptionChange("theme",…)` 等）[VERIFIED: node_modules/@xterm/xterm/typings/xterm.d.ts:890-899 + lib/xterm.js]。ro 判定需要模块级 `isRO` 状态（main.ts:202 的 mode 当前是 WELCOME 分支局部变量——planner 需提为模块级，供粘贴门/标题写口/③ osc52 分支共用）。

### Anti-Patterns to Avoid
- **只 loadAddon 不设 activeVersion：** unicode11 等于没装（activate 仅 register）——宽度测量仍走 Unicode 6，CJK/emoji 宽度依旧错。
- **OSC8 不设 linkHandler：** 用户点链接吃 `confirm()` 原生警告框，打断终端流（d.ts 明示的缺省回退）。
- **传自定义 web-links handler：** 库默认 handleLink 已满足 D-07（noopener 形态 + 拦截静默 warn），自定义是重复造轮子且易丢 opener=null。
- **readText reject 实现 write-only：** 核心异步 OSC 链 rethrow 拒绝（§Pitfall 4）——resolve '' 才是干净形态。
- **检测到 navigator.clipboard 缺失仍调用：** 非安全上下文属性本身 undefined，调用即 TypeError——必须存在性门控（D-11）。
- **beforeunload 写自定义文案/不清除 listener：** 文案被浏览器忽略（MDN）；会话终结后不清除则 "Session ended" 后关页仍被拦。
- **服务端解析 prefs 内容：** Welcome 内嵌 prefs 对服务端是不透明 blob（白名单/JSON 校验在 `--client-option` parse 期已完成）——服务端二次解析引入双写漂移面。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| URL 识别正则 | 自维护 URL regex | addon-web-links 默认 strictUrlRegex | 尾部标点/括号包裹/IPv6/折行拼接等边缘 case 官方已平衡（WebLinksAddon.ts:10-21 注释即教训史）；D-05 锁定 |
| 链接安全打开 | 自写 window.open 包装 | 库默认 handleLink | opener=null 防 reverse tabnapping、弹窗拦截静默——已逐字核实 |
| wcwidth 宽字符测量 | 自实现 East Asian Width 表 | addon-unicode11 | Unicode 11 全表 + 组合符/emoji 规则，维护成本极高 |
| IME 组合输入 | 自接 compositionstart/end | xterm 内建 textarea composition | 浏览器 IME 栈差异巨大；xterm 已被 VS Code 生产验证；main.ts:90 既定路线 |
| OSC52 base64 | 自写 base64 | js-base64（addon 依赖） | UTF-8 多字节安全；btoa 对非 Latin1 抛错是经典坑 |
| 剪贴板读写 | execCommand | navigator.clipboard | execCommand 已废弃（MDN 明示）；权限模型现代 |
| 帧 JSON 组帧 | 字符串拼接 | encoding/json + json.RawMessage 透传 | 转义/编码正确性；Go 侧惯例 |

**Key insight:** 本 phase 的核心工作是**接线而非实现**——xterm 官方 addon 生态已覆盖全部能力面，自定义代码集中在：sanitize（安全契约）、防抖（UX 契约）、装配顺序（优先级契约）、浮层/tooltip 两瞬态元素（UI-SPEC 契约）。凡是"解析终端序列/匹配 URL/测量字符宽度"的需求，先查 addon 是否已解决。

## Common Pitfalls

### Pitfall 1: web-links 默认正则"仅 http(s)"与 D-05 表述落差
**What goes wrong:** CONTEXT D-05 描述默认正则含 "http/https/ftp/mailto 等"，实际 0.12.0 产物 `strictUrlRegex` 只匹配 `https?`——ftp/mailto 不被链接化。
**Why it happens:** xterm 上游曾收紧默认正则（注释注明排除 unsafe chars 与尾部标点），描述性记忆滞后。
**How to avoid:** 以产物源码为准（UI-SPEC §Link Contract 已修正记录）；保持默认不动正符合 D-05 本意（不自定义收紧或放宽）。
**Warning signs:** 测试用 ftp:// 断言链接化失败——那不是 bug。

### Pitfall 2: unicode11 加载-激活顺序颠倒或漏激活
**What goes wrong:** `activeVersion = '11'` 在 loadAddon 之前 → 抛 `unknown Unicode version "11"`；只 loadAddon 不激活 → 静默无效果（宽度仍按 Unicode 6）。
**Why it happens:** activate() 只 register 不设 active（addon 源码逐字如此）； setter 校验注册表。
**How to avoid:** 两行紧随写入 addon 加载段（webgl 段后）；FE-02 验收以 CJK/emoji 宽度实测为准。
**Warning signs:** emoji 后光标位置错位、中文与 ASCII 混排重叠。

### Pitfall 3: theme 部分对象导致 wesh 调色板丢失（研究新发现）
**What goes wrong:** `--client-option 'theme={"background":"#000"}'` 运行期应用后，**未指定的 theme 键（foreground/16 色 ANSI 等）回退到 xterm 内建默认调色板**，而非保留 01-UI-SPEC 钉死的 tango 调色板。
**Why it happens:** 核心 ThemeService `_setTheme(e={})` 逐键执行 `v(e.key, defaultColor)`——`function v(e,t){if(void 0!==e)try{return css.toColor(e)}catch{}return t}`，缺键与**非法色值**都静默回退 xterm 默认（无报错面）[VERIFIED: node_modules/@xterm/xterm/lib/xterm.js 产物]。
**How to avoid:** 前端应用 theme 前先合并：`term.options.theme = { ...currentDefaultTheme, ...prefs.theme }`（currentDefaultTheme 即构造时钉死的 tango 对象常量）——"整体替换"语义保留（D-19），但未指定键保留 wesh 调色板。此为对 UI-SPEC "整体替换"字面的**实现层修正建议**，planner 定稿（与 ttyd  parity 的取舍见 §Open Questions 1）。
**Warning signs:** 下发部分 theme 后 ANSI 彩色输出色相突变（tango→xterm 默认）。

### Pitfall 4: write-only provider 的 readText reject 触发 unhandled rejection（研究新发现）
**What goes wrong:** UI-SPEC 字面 "readText 恒 reject"——ClipboardAddon `_setOrReportClipboard` 对 promise 只挂 `.then` 不挂 `.catch`（ClipboardAddon.ts:45-50），拒绝沿返回 promise 传入核心异步 OSC handler 链；核心对该链的处理是 `Promise.race([handler, 5s超时]).catch(e => { if (e!==慢超时) throw e; … })`——**rethrow 形成 unhandled rejection** 控制台噪音。
**Why it happens:** addon 假设 provider readText 成功或同步返回；核心异步链只容忍"慢"不容忍"错"。
**How to avoid:** readText 实现为 `Promise.resolve('')`——应用查询剪贴板得到空回复（OSC52 协议完整应答），剪贴板内容同样零泄露（只写不读安全姿态不变），无 unhandled rejection。
**Warning signs:** 远端程序发 OSC52 读查询（`printf '\e]52;c;?\a'`）后 DevTools console 出现 unhandled rejection。

### Pitfall 5: navigator.clipboard 安全上下文三态
**What goes wrong:** 明文 HTTP 非 localhost 下 `navigator.clipboard` **属性本身不存在**（MDN: Navigator.clipboard 标注 Secure context）——不检测即调用抛 TypeError；Chromium 写需 clipboard-write 权限**或** transient activation（选区拖拽是 activation 手势，150ms 防抖远在窗口期内）；Firefox/Safari 写需 transient activation、读弹一次性"Paste"菜单。
**Why it happens:** Clipboard API 是 [SecureContext] 接口；各浏览器权限模型分叉（MDN §Security considerations）。
**How to avoid:** 启动时 `typeof navigator.clipboard !== 'undefined'` 检测并记忆（D-11）；写失败/读拒绝一律 `.catch → console.warn` 静默；ro 不读（D-10）；README 明示 HTTPS/localhost 要求 [CITED: developer.mozilla.org/en-US/docs/Web/API/Clipboard_API]。
**Warning signs:** 反代明文 HTTP 部署下选中复制"没反应"——按设计静默，README 是出口。

### Pitfall 6: onTitleChange 不含 OSC 1
**What goes wrong:** 若程序只发 OSC 1（设 icon name），浏览器标签页标题不更新。
**Why it happens:** 核心 OSC 1 handler 仅 `setIconName`，不 fire onTitleChange（d.ts 写明 "OSC 0 or OSC 2"）。
**How to avoid:** 接受现状——真实世界标题设置（bash/tmux/vim/SSH）均用 OSC 0 或 2；不为 OSC 1 加补丁（CONTEXT D-01 的 "0/1/2" 按产物实际修正理解）。
**Warning signs:** 仅出现在手写 OSC 1 序列的对照测试里。

### Pitfall 7: prefs 应用后忘记 fit.fit() 或顺序错误
**What goes wrong:** fontSize/lineHeight/letterSpacing 改变单元格尺寸但不 fit → cols/rows 与实际视口不符，远端 TUI 画错尺寸。
**Why it happens:** term.options 赋值不触发 onResize；只有 fit.fit() 重算后尺寸变化才触发既有 onResize→RESIZE 帧链路。
**How to avoid:** 严格按 UI-SPEC 装配顺序：prefs 全部应用完 → fit.fit()（D-13 既定决策）。
**Warning signs:** 下发 fontSize 后远端 vim 底部留空/溢出。

### Pitfall 8: query 解析让终端不可用
**What goes wrong:** 非法 query（裸 `fontFamily=Menlo` 非合法 JSON、未知键）若报错/阻断，用户侧输入错误把终端打挂——违反 D-16。
**How to avoid:** 逐键 try JSON.parse，失败静默忽略 + console.warn；白名单外键同样忽略；`osc52` 不在 query 白名单（安全不对称，D-12/D-16）。
**Warning signs:** console.warn 刷屏但终端可用——按设计。

## Code Examples

### Go：--client-option parse 期校验（fs.Func 既有形态，main.go:56-63 同款）
```go
// Source: cmd/wesh/main.go 既有 --credential/--origin fs.Func 模式 + 04-CONTEXT D-15
fs.Func("client-option", "client preference key=value (repeatable; whitelisted keys, value is JSON)", func(s string) error {
	key, value, found := strings.Cut(s, "=")
	if !found || !proto.ValidClientOptionKey(key) { // 白名单含 8 xterm 键 + 2 前端键；osc52 不在内（D-12）
		return fmt.Errorf("invalid --client-option key %q", key)
	}
	var v json.RawMessage
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		return fmt.Errorf("invalid --client-option value for %q: not valid JSON", key)
	}
	cfg.clientOptions = append(cfg.clientOptions, clientOption{key: key, value: v})
	return nil
})
fs.BoolVar(&cfg.osc52, "osc52", false, "enable OSC52 clipboard write (write-only; default off)")
```

### Go：Welcome 组帧注入 prefs（server.go:433 挂点扩展）
```go
// Source: internal/server/server.go:433 既有 WelcomeFrame 调用 + proto.go:111-114 组帧模式
// server.New 装配期把 clientOptions 聚合为单个 json.RawMessage（{k1:v1,…, "osc52":true}），
// Attach 升档序列尾段：
_ = c.Write(ctx, websocket.MessageBinary, proto.WelcomeFrame(mode, s.clientPrefs))
```

### 前端：addon 加载段（main.ts webgl 段后续写）
```typescript
// Source: 04-UI-SPEC §Addon Contract 加载点 + §Pattern 2/3/4
term.loadAddon(new Unicode11Addon());
term.unicode.activeVersion = '11'; // 硬顺序（§Pitfall 2）
term.loadAddon(new WebLinksAddon(undefined, { hover: showLinkTooltip, leave: hideLinkTooltip }));
// ClipboardAddon 不在此加载——仅 WELCOME prefs osc52===true 时（§Pattern 4③）
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `execCommand('copy')` 选中复制（ttyd） | `navigator.clipboard.writeText` | API 废弃（MDN 明示 deprecated） | FE-05 现代形态；需安全上下文 + 失败静默 |
| OSC52 默认开放（含读） | addon 默认不加载；开启只写不读 | Warp CVE-2025-48725（2025）教训 | wesh D-12：--osc52 默认关 + write-only provider |
| VS Code 式 Ctrl+Click 链接激活 | xterm 默认单击无修饰键 | xterm 历来如此（VS Code 是其自身实现） | 本 phase 以库默认为准（D-07 核实结论） |
| beforeunload returnValue 文案 | preventDefault() + 无自定义文案 | 现代浏览器忽略自定义文案（MDN） | FE-06 用标准确认框（D-18） |

**Deprecated/outdated:**
- `document.execCommand`：MDN 明示 deprecated，禁止任何形式回落（D-11 静默不落旧 API）。
- OSC 1 标题同步期望：xterm 不对 OSC 1 fire onTitleChange（§Pitfall 6），不要为它写兼容代码。
- 自定义 web-links urlRegex/handler：D-05/D-07 锁定库默认。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | theme 应用建议合并当前调色板后赋值（对 D-19 "整体替换" 的实现层修正） | §Pitfall 3 | 若 planner 选 ttyd parity（部分 theme 丢 wesh 调色板），行为符合 D-19 字面但 UX surprising——非错误，是取舍 |
| A2 | write-only provider readText 用 resolve '' 替代 UI-SPEC "恒 reject" | §Pitfall 4 | 若坚持 reject：功能可用但 OSC52 读查询触发 unhandled rejection 噪音；安全姿态两者相同 |
| A3 | 纯函数单测可用 `node --test` + Node 24 内建 type stripping 跑 .ts（零新依赖） | §Validation Architecture | 若 type stripping 对所选模块布局不适用，回退手动 UAT + Go 侧覆盖（vitest 引入是更重备选） |
| A4 | Chromium transient activation 窗口（秒级）远大于 150ms 防抖 | §Pattern 4① | 几乎无风险：防抖 150ms 相对任何实现的 activation 窗口都有两个数量级余量；写失败路径本就静默 |

## Open Questions

1. **theme 部分对象：合并保留 wesh 调色板 vs ttyd parity 整体替换**
   - What we know: xterm 运行时 theme 赋值对未指定键回退 xterm 内建默认（产物源码核实）；ttyd 经构造选项传入 theme 时行为相同（同一 _setTheme 路径）——"对标 ttyd" 的字面结果就是部分 theme 丢自定义调色板。
   - What's unclear: D-19 "完整 JSON 对象整体替换" 的意图是"用户应给完整对象"还是"机制上整体替换即可"。
   - Recommendation: 前端合并覆盖（`{...defaultTheme, ...incoming}`）——保留 D-19 语义且消除 surprising 面；plan 定稿，若用户在意 ttyd 严格 parity 则记文档。

2. **OSC52 readText：resolve '' vs reject**
   - What we know: reject 在核心异步 OSC 链是 unhandled rejection（机制核实）；resolve '' 协议完整且安全等价。
   - What's unclear: UI-SPEC "恒 reject" 是安全意图表述还是机制指定。
   - Recommendation: resolve ''，在 UI-SPEC 修订注记；plan 定稿。

3. **beforeunload sticky activation 的零交互不弹框**
   - What we know: MDN 明示无用户手势不弹确认框（"no user data to save" 理据）。
   - What's unclear: 是否需要任何补偿（如页面加载即标记"会话已开始"）。
   - Recommendation: 接受浏览器语义——零交互即无会话投入，不弹合理；写入 UAT 人工确认项，不做代码补偿。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | web 构建（Vite 8 要求 ^20.19\|\|>=22.12） | ✓ | v24.13.0 | — |
| pnpm | 依赖安装/构建 | ✓ | 11.21.0 | — |
| Go | 后端构建/测试 | ✓ | go1.26.3 linux/amd64 | — |
| npm registry 可达性 | pnpm add 三个 addon | ✓（本次核实经 registry 查询） | — | — |
| 浏览器（人工 UAT） | IME/剪贴板/hover/浮层/beforeunload 验证 | 用户侧（无显示机器，Phase 2/3 UAT 同分工先例） | — | 协议层 Node UAT 自动化 + 渲染层人工确认 |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** 浏览器渲染层验证——按 Phase 2/3 既定分工（协议断言自动化 + 用户外部浏览器人工确认）。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing`（既有，逐字锁定协议/CLI 契约）+ Node 零依赖 UAT 脚本（web/uat/*.mjs 先例）+ `tsc` 类型检查（随构建） |
| Config file | none（Go 标准测试；UAT 脚本零配置） |
| Quick run command | `go test ./internal/... ./cmd/... && pnpm -C web build` |
| Full suite command | `go test -race ./... && node web/uat/phase04.mjs /tmp/wesh-uat/wesh` + 人工浏览器 checklist |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FE-07 | `--client-option` 白名单/JSON 校验启动报错 exit 2 | unit | `go test ./cmd/wesh -run TestParseArgs` | ❌ Wave 0（TestParseArgs 表加 wantClientOptions/wantOSC52 字段——P3 命名字段转换先例） |
| FE-07 | Welcome 帧携 prefs（omitempty 缺席兼容） | unit | `go test ./internal/proto -run TestWelcomeFrame` | ❌ Wave 0（TestWelcomeFrameErrorFrame 扩展 prefs 行） |
| FE-07/D-12 | `--osc52` → prefs 含 `osc52:true`；默认不含；`--client-option osc52=…` 拒绝 | unit + e2e | `go test ./cmd/wesh` + `node web/uat/phase04.mjs`（WS 握手断言 Welcome JSON） | ❌ Wave 0（uat/phase04.mjs 新建，phase03.mjs 形态） |
| CORE-03 | OSC 0/2 → document.title（含 [ro] 前缀、sanitize 截断） | unit(纯函数可选) + manual | `node --test web/src/lib/*.test.ts`（A3 备选）+ 人工浏览器 | ❌ Wave 0（若采纳纯函数抽取） |
| FE-02 | CJK/emoji 宽度、IME 组合输入不丢字 | manual-only | 人工 UAT checklist（UI-SPEC 🧪 backstop 既定） | n/a |
| FE-04 | URL 链接化、hover 真实 URL、单击新标签页、OSC8 无 confirm 框 | manual-only | 人工 UAT checklist | n/a |
| FE-05 | 选中即复制、Ctrl+Shift+V 粘贴、ro 不读、非安全上下文静默 | manual-only | 人工 UAT checklist（剪贴板权限需真实浏览器） | n/a |
| FE-06 | 浮层时序/淡出、beforeunload 拦截与会话终结后放行、开关关闭路径 | manual-only | 人工 UAT checklist | n/a |

### Sampling Rate
- **Per task commit:** `go test ./internal/... ./cmd/... && pnpm -C web build`（tsc 类型门 + Go 契约测试）
- **Per wave merge:** `go test -race ./... && node web/uat/phase04.mjs`
- **Phase gate:** 全量绿 + 人工浏览器 checklist（FE-02/04/05/06 与 CORE-03 渲染面）→ `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/uat/phase04.mjs` — WS 协议断言：Welcome prefs 形状/osc52 注入/非法 client-option 启动报错（覆盖 FE-07 + D-12 自动化面；形态照 phase03.mjs 零依赖先例）
- [ ] `internal/proto/proto_test.go` — TestWelcomeFrameErrorFrame 扩展 prefs 往返 + omitempty 缺席行（逐字锁定纪律）
- [ ] `cmd/wesh/main_test.go` — TestParseArgs 表加 `--client-option`/`--osc52` 行与非法值报错行（P3 启动校验矩阵同风格；命名字段转换先例）
- [ ] （planner 选项，A3）`web/src/lib/` 纯函数抽取 + `node --test` 用例：sanitizeTitle 截断/剥离、query 解析白名单与 JSON 容错、prefs 优先级合并——零新依赖；不采纳则这些走人工 + 代码评审

## Security Domain

### Applicable ASVS Categories（security_asvs_level: 1）

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | 本 phase 不动认证面（ticket/Basic 为 Phase 3 既有） |
| V3 Session Management | no | WS 单次语义不变 |
| V4 Access Control | no | ro/rw 边界既有（服务端丢 INPUT 为真边界，D-10 粘贴门是 UX 层） |
| V5 Input Validation | yes | query/prefs 白名单 + JSON parse（前端容错/服务端 fail-fast）；标题 sanitize 剥离 C0/DEL/C1；theme 色值非法静默回退（库行为，已知） |
| V6 Cryptography | no | — |

### Known Threat Patterns for xterm.js 终端栈

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| OSC 52 剪贴板劫持/窃取（Warp CVE-2025-48725） | Tampering / Info Disclosure | addon 默认不加载（核心无内建 handler，OSC52 惰性）；开启经 `--osc52` 服务端意图 + write-only provider（只写不读） |
| OSC 8 钓鱼链接（显示 github.com 点开 evil.com） | Spoofing | hover 展示真实 URL（双通道统一 tooltip）+ `allowNonHttpProtocols=false`（javascript:/file: 结构性忽略）+ 服务端不过滤由前端兜底（D-06/D-08） |
| OSC 0/2 标题注入伪装主机名/路径 | Spoofing | sanitize 剥离 C0/DEL/C1 + 128 code point 截断 + `[ro] ` 前缀恒在最前（D-02/D-03） |
| Reverse tabnapping（window.opener 劫持） | Tampering | 库默认 handleLink 与 linkHandler 均 `opener=null`（等价 rel=noopener），逐字核实 |
| 剪贴板权限滥用（ro 无谓弹窗/静默读） | Info Disclosure | ro 不读剪贴板（D-10）；非安全上下文整体静默（D-11） |
| 任意 xterm option 注入（allowProposedApi 等危险面） | Elevation of Privilege | prefs/query/`--client-option` 三通道同一白名单（D-14/D-16）；osc52 仅服务端可开启（D-12 安全不对称） |

## Project Constraints (from CODEBUDDY.md)

全局 CODEBUDDY.md 中适用于本 phase 的指令（planner 须遵守）：

- **使用 pnpm 而非 npm**——所有安装/构建命令走 `pnpm -C web …`。
- **始终使用中文**（代码注释/文档）；前端 UI 文案英文为项目 P2 惯例，优先于通用语言偏好。
- **构建用 `time` 前缀跟踪耗时；偏好全量构建；构建后验证产物时间戳**——`web/dist/index.html` 重建入库后核对。
- **不接受过度设计或不必要的新类型/枚举**——Go 侧白名单用简洁常量/switch 即可，不引入新抽象层。
- **switch case 内容加大括号；指针/可空值获取后判空**——Go/TS 两侧同守（TS 侧模块级 `ws: WebSocket | null` 既有 null 闸先例）。
- **修改前备份/编译通过后再下一步**——任务编排保持"前端接线 → 构建绿 → Go 扩展 → 测试绿"的可回退顺序。
- **文档输出到项目目录**（本文件即 `.planning/phases/04-frontend/` 下）。
- **未经确认禁止操作重启服务**——验证环节不重启任何用户服务；UAT 用临时端口 spawn 测试实例（phase03.mjs 先例）。

## Sources

### Primary (HIGH confidence)
- `/tmp/wesh-addon-verify/` 三包 npm 发布产物（含完整 TS 源码）— Unicode11Addon.ts:12-17、WebLinksAddon.ts:21,24-36,42-46、WebLinkProvider.ts:8-12、ClipboardAddon.ts:14-17,33-69,72-86（本 session 逐一 Read 核实）
- `node_modules/@xterm/xterm@6.0.0` 产物与 typings — xterm.d.ts:152-163（linkHandler confirm 回退）、:899（options 可变）、:999-1003（onTitleChange OSC 0/2）、:1891/:1901（unicode register/activeVersion）、:1270-1275（paste）；lib/xterm.js（_handleMouseUp 无修饰键、OSC 0/1/2 handler 分派、无内建 OSC52、_setTheme 默认回退、v() 色值回退、xterm-hover class、async OSC 链 rethrow、activeVersion setter 抛错）
- npm registry（npm view）— 三包版本/发布时间/postinstall 空；js-base64 版本时间线
- `gsd-tools query package-legitimacy check` — 三包 OK、js-base64 SUS(too-new)
- Context7 `/xtermjs/xterm.js` — unicode11 README 激活形态、OscLinkProvider defaultActivate confirm+opener=null、allowNonHttpProtocols 语义（与本地产物核实一致）

### Secondary (MEDIUM confidence)
- MDN Clipboard API（developer.mozilla.org/en-US/docs/Web/API/Clipboard_API）— Secure context、execCommand deprecated、各浏览器读写权限模型
- MDN beforeunload_event — preventDefault 形态、自定义文案忽略、sticky activation、listener 移除建议

### Tertiary (LOW confidence)
- 无（本 phase 无仅 WebSearch 单源的关键声明）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — registry + legitimacy seam + 产物源码三重核实
- Architecture: HIGH — 全部 Pattern 有产物源码或契约文件逐字依据
- Pitfalls: HIGH — Pitfall 2/3/4/6 为产物源码一手发现；1/5/7/8 有文档/契约依据

**Research date:** 2026-08-18
**Valid until:** 2026-09-17（30 天；xterm 生态稳定，beta 线日更不影响已钉稳定版）
