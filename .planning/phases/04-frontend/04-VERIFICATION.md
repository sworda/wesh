---
phase: 04-frontend
verified: 2026-08-19T00:04:53Z
status: human_needed
score: 45/47 must-haves verified
behavior_unverified: 2
overrides_applied: 0
behavior_unverified_items:
  - truth: "中文拼音/日文 IME 组合输入不丢字、CJK/emoji 宽度正确占两格（04-02 backstop truth）"
    test: "04-UAT.md T1/T2：终端 echo 中文/emoji、vim/htop 显示中文；真实拼音/日文 IME 组合输入"
    expected: "CJK 占两格不错位、emoji 后光标位置正确；IME 组合与上屏不丢字不乱码"
    why_human: "字形渲染与宽度测量是视觉效果；真实 IME 栈（OS 输入法 → 浏览器 composition 事件 → xterm textarea）自动化结构性不可达。代码面已验：Unicode11Addon 加载+activeVersion='11' 硬顺序在 main.ts:113-114，输入链路 onData 未改"
  - truth: "真实浏览器权限模型下：选中即复制生效、Ctrl+Shift+V 粘贴生效、权限拒绝静默、浮层时序与淡出、beforeunload 拦截与会话终结后放行（04-03 backstop truth）"
    test: "04-UAT.md T5-T9：真实浏览器拖动选中/粘贴/resize 拖窗/关页，含 ro 形态与明文降级形态"
    expected: "选中即复制入系统剪贴板（150ms 防抖只写最终选区）；rw 粘贴到 shell 且 bracketed paste 语义保留、ro 无权限弹窗；浮层 resize 期间显示静止 600ms 淡出；会话中关页拦截、Session ended 后放行"
    why_human: "系统剪贴板真实状态、浏览器原生确认框、浮层淡出时序均需真实浏览器人工核对。代码面已验：clipboardOK 门/防抖/双门/条件注册/onclose 移除全部就位（main.ts:274-302, 242-256, 476-478, 532）"
human_verification:
  - test: "T1 CJK/emoji 宽度（FE-02）：形态 A 执行 echo '中文测试🙂🎉' 与 printf 对齐测试；vim/htop 显示中文"
    expected: "CJK 字符占两格不错位；emoji 后光标位置正确"
    why_human: "字形渲染与宽度测量的视觉效果自动化不可测"
  - test: "T2 IME 组合输入（FE-02）：形态 A 用中文拼音（如有日文 IME 一并）输入完整句子"
    expected: "组合过程与上屏不丢字、不乱码、组合框位置正常"
    why_human: "真实 IME 栈自动化不覆盖（UI-SPEC backstop 人工出口）"
  - test: "T3 标题同步（CORE-03）：形态 A printf '\\e]2;custom-title\\a' 并开 vim/tmux；形态 B 重复"
    expected: "标签页标题同步并随程序变化；形态 B 恒为 '[ro] custom-title' 且前缀最前不丢"
    why_human: "标签页标题是浏览器可视面"
  - test: "T4 超链接（FE-04）：echo 裸 URL 与 printf OSC8 序列，hover + 单击"
    expected: "hover 显示完整真实地址 tooltip，单击新标签页打开；OSC8 链接可辨别显示文本与目标不一致，点击无 confirm 原生框"
    why_human: "hover tooltip 视觉与点击行为需真实浏览器"
  - test: "T5 选中即复制（FE-05）：形态 A 拖动选中文本后到他处粘贴"
    expected: "系统剪贴板即为所选内容；拖动过程不频繁写剪贴板（150ms 防抖最终选区一次写入）"
    why_human: "系统剪贴板真实状态需人工核对"
  - test: "T6 粘贴（FE-05/D-10）：形态 A 与形态 B 各按 Ctrl+Shift+V"
    expected: "形态 A 内容到达 shell（bracketed paste 语义保留）；形态 B 无权限弹窗、无效果"
    why_human: "浏览器剪贴板权限门控与 bracketed paste 行为需真实浏览器"
  - test: "T7 明文降级（D-11）：形态 C（明文 HTTP + 非 localhost）拖动选中与 Ctrl+Shift+V"
    expected: "选中复制与粘贴静默不生效（不弹错无提示）；终端其余功能正常"
    why_human: "非安全上下文的浏览器行为差异需真实环境"
  - test: "T8 resize 浮层（FE-06/D-17）：形态 A 拖动窗口；再以 resizeOverlay=false 重启重复"
    expected: "默认 resize 期间右上角 COLSxROWS 浮层，静止约 600ms 淡出；开关关闭后不显示"
    why_human: "浮层视觉与淡出时序需人眼"
  - test: "T9 离开确认（FE-06/D-18）：形态 A 会话中关页、Session ended 后关页、confirmBeforeUnload=false 关页"
    expected: "会话中关页被浏览器标准确认框拦截；会话终结后不再拦截；开关关闭后不拦截（零交互直接关页不弹框为浏览器预期 sticky activation 语义，已裁决非缺陷）"
    why_human: "浏览器原生确认框与 sticky activation 语义自动化不可驱动"
  - test: "T10 偏好下发与 query 覆盖（FE-07）：--client-option fontSize=18 + theme 启动，再 ?fontSize=11 与非法 ?fontFamily=Menlo 打开"
    expected: "字号变大背景生效且 ANSI 色相保持内置调色板；query 覆盖优先；非法 query 静默忽略且 console 有 warn"
    why_human: "视觉效果（字号/色相）与 console warn 需真实浏览器"
  - test: "T11 OSC52 写入（可选，D-12）：形态 A 加 --osc52，printf OSC52 写/读查询序列"
    expected: "系统剪贴板写入 hello；读查询无 unhandled rejection（console 干净）"
    why_human: "系统剪贴板真实状态与 console 噪音需人工核对"
---

# Phase 4: 前端体验 验证报告

**Phase Goal:** 前端达到并超越 ttyd 功能基线（修掉其废弃 API 与停更依赖）
**Verified:** 2026-08-19T00:04:53Z
**Status:** human_needed
**Re-verification:** No — initial verification（验证基线为 04-REVIEW-FIX.md 修复后代码：WR-01..WR-04 四项修复全部核实落地）

## Goal Achievement

### Observable Truths

#### ROADMAP 成功标准映射（3 条 → plan truths 承载，不重复计分）

| # | Success Criterion | 承载 truths | 状态 |
|---|-------------------|------------|------|
| SC1 | 中文/emoji 宽字符正常输入显示 + URL 自动识别可点击超链接（hover 真实地址） | 04-02 T1-T4 + backstop | ✓ 代码面全验；IME/渲染面人工（T1/T2/T4） |
| SC2 | 选中即复制走 navigator.clipboard（替代 execCommand）+ 标题同步标签页 | 04-03 T1-T4 + 04-02 T5-T7 | ✓ 代码面全验（execCommand 全仓零引用）；真实剪贴板/标签页人工（T3/T5-T7） |
| SC3 | resize 浮层 + 离开确认（均可开关）+ 服务端偏好生效 query 可覆盖 | 04-01/04-03/04-04/04-05 全部 | ✓ 协议层 UAT 10/10 实测；渲染面人工（T8-T10） |

#### 04-01（FE-07 Go 通道，7 truths）— 7/7 VERIFIED

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | WelcomePayload.Prefs omitempty：非空携带、nil 缺席 | ✓ VERIFIED | proto.go:89-92；UAT S1 实测（无 flag 时 prefs 键缺席=true） |
| 2 | WelcomeFrame(mode, prefs) 签名演进无遗留旧调用 | ✓ VERIFIED | proto.go:118；server.go:442 唯一调用点 `proto.WelcomeFrame(mode, s.clientPrefs)` |
| 3 | ValidClientOptionKey 白名单恰 10 键，osc52/allowProposedApi/未知键拒绝 | ✓ VERIFIED | proto.go:128-141 switch 恰 10 键；TestValidClientOptionKey 表驱动锁定；UAT E1/E3 实测 exit 2 |
| 4 | --client-option parse 期校验（缺'='/白名单外/非法 JSON 即时报错 exit 2）+ --osc52 默认 false | ✓ VERIFIED | main.go:101-121 fs.Func 回调三段校验；UAT E1-E4 实测 exit 2+文案 |
| 5 | 聚合 last-wins + osc52 并入 + 零配置 nil | ✓ VERIFIED | main.go:166-176 aggregateClientPrefs；UAT S4/S5/S6 实测 |
| 6 | 服务端不透明透传不二次解析 | ✓ VERIFIED | proto.go:87 注释明示 + json.RawMessage 端到端无 Unmarshal；server.go:146 直传 |
| 7 | e2e 断言 Welcome prefs 注入/未注入两形态 | ✓ VERIFIED | e2e_test.go:527 TestWelcomePrefs；`go test -race` 实测 ok |

#### 04-02（addon 接入 + 标题 + 超链接，9 truths + 1 backstop）— 9/9 VERIFIED + 1 ⚠️

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | unicode11 加载-激活硬顺序 | ✓ VERIFIED | main.ts:113-114 `loadAddon(new Unicode11Addon())` 紧随 `term.unicode.activeVersion = '11'` |
| 2 | web-links 默认正则 + 不传自定义 handler | ✓ VERIFIED | main.ts:119 第一参 `undefined`（库默认 window.open→opener=null） |
| 3 | OSC8 显式 linkHandler（window.open + opener=null；allowNonHttpProtocols 默认 false） | ✓ VERIFIED | main.ts:125-135；未设 allowNonHttpProtocols |
| 4 | link tooltip 双通道统一（xterm-hover class/完整 URL/+8px 偏移/视口翻转/pointer-events none） | ✓ VERIFIED | main.ts:141-163 + index.html:45；hover/leave 双通道挂接（119, 133-134） |
| 5 | 标题同步单一写口（onTitleChange→sanitizeTitle→setTitle；128 code point；空串回退 wesh） | ✓ VERIFIED | main.ts:266-269 + 207-209；title.ts:10-18；document.title 全文件唯一赋值点（main.ts:208） |
| 6 | setTitle 取代一次性前缀拼接；isRO 模块级共用 | ✓ VERIFIED | main.ts:173 isRO 模块级；389-394 WELCOME ro 分支经 setTitle() 补前缀（重收 WELCOME 不双前缀）；isRO 被粘贴门（297）/osc52 门（460 间接）共用 |
| 7 | node --test 锁定 sanitizeTitle | ✓ VERIFIED | 实跑 16/16 通过（含 WR-04 新增 Cf 剥离 4 断言：`\u202emoc.live`→`moc.live` 等） |
| 8 | UI-SPEC 契约面不变（无 spinner/五态面板/z-index 不动） | ✓ VERIFIED | main.ts 无新状态面板路径；showStatus 三态结构未改 |
| 9 | dist 重建入库且含检索串 | ✓ VERIFIED | git ls-files 确认入库；**重建哈希字节级一致**（构建前后 SHA256 前缀均 468ef8c298413d0d） |
| 10 | ⚠️ backstop：IME 组合不丢字、CJK/emoji 占两格 | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Unicode11 激活与输入链路代码面在；渲染/IME 效果无测试可验 → 人工 T1/T2 |

#### 04-03（剪贴板 + 浮层 + beforeunload，7 truths + 1 backstop）— 7/7 VERIFIED + 1 ⚠️

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | clipboardOK 存在性检测 | ✓ VERIFIED | main.ts:274 `typeof navigator.clipboard !== 'undefined'` |
| 2 | 选中即复制：onSelectionChange→150ms trailing 防抖→非空非重复→writeText；失败 catch 静默 | ✓ VERIFIED | main.ts:279-290（selTimer/lastCopied/`.catch(console.warn)`） |
| 3 | Ctrl+Shift+V 粘贴：preventDefault→readText→term.paste；clipboardOK+isRO 双门；读拒绝 catch 静默 | ✓ VERIFIED | main.ts:296-302 |
| 4 | 非安全上下文整体静默降级，不落 execCommand | ✓ VERIFIED | clipboardOK 门 + **全仓 execCommand 零引用**（grep src/ 与 index.html） |
| 5 | resize 浮层元素/样式/600ms 静止淡出/COLSxROWS | ✓ VERIFIED | index.html:32-42,67（hidden 默认、样式逐规格）+ main.ts:241-256（overlayTimer 600ms→opacity 0→transition 200ms 淡出） |
| 6 | 浮层双门（welcomeDone && resizeOverlayOn）；ro 同显 | ✓ VERIFIED | main.ts:247 双门早退 |
| 7 | beforeunload 仅 preventDefault；WELCOME 完成时条件注册；onclose 任意路径移除 | ✓ VERIFIED | main.ts:201-203（无自定义文案）、476-478（confirmBeforeUnloadOn 条件注册）、532（onclose 首行 removeEventListener，含重试路径） |
| 8 | ⚠️ backstop：真实浏览器权限模型下剪贴板/浮层/beforeunload 行为 | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | 代码面全在；真实权限/焦点/原生框行为自动化不可达 → 人工 T5-T9 |

#### 04-04（协议 UAT，7 truths）— 7/7 VERIFIED（全部实测）

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | phase04.mjs 零依赖 Node 原生 WS；每场景独立 spawn | ✓ VERIFIED | web/uat/phase04.mjs 存在（12KB）；实测 10/10 通过 |
| 2 | 默认形态无 prefs 键 | ✓ VERIFIED | UAT S1 PASS（prefs键缺席=true） |
| 3 | --client-option 注入（fontSize/cursorBlink/theme 对象逐键相等） | ✓ VERIFIED | UAT S2/S3 PASS |
| 4 | --osc52 注入与组合 | ✓ VERIFIED | UAT S4/S5 PASS |
| 5 | 重复 key last-wins | ✓ VERIFIED | UAT S6 PASS（fontSize=22 取后者） |
| 6 | 启动拒绝矩阵四负场景 exit 2 + 文案 | ✓ VERIFIED | UAT E1-E4 PASS（allowProposedApi/非法 JSON/osc52 key/缺'='） |
| 7 | 输出红线只打形状/布尔/状态码 | ✓ VERIFIED | 实测输出 detail 仅 keys 名与等式布尔，无值内容 |

#### 04-05（前端 prefs 通道，9 truths）— 9/9 VERIFIED

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | query 构造前解析（白名单 10 键/非法静默+warn/osc52 排除/theme 构造特判合并） | ✓ VERIFIED | main.ts:57-75；prefs.ts:24-44；prefs.test.ts 实跑通过（含 osc52 排除专项） |
| 2 | defaultTheme 常量化三处同源 | ✓ VERIFIED | main.ts:29-51 模块级常量；构造初值（90）、query 特判（68）、WELCOME 合并（418）三处同源 |
| 3 | WELCOME prefs 应用装配顺序（queryKeys 跳过/theme 合并/非对象忽略+warn） | ✓ VERIFIED | main.ts:399-430（WR-01 修复后逐键 try/catch + 整段独立 try，值内容不入日志） |
| 4 | 应用完 fit.fit() 重算 | ✓ VERIFIED | main.ts:434 |
| 5 | behavior 两键 typeof boolean 校验写开关量；位置在注册点前 | ✓ VERIFIED | main.ts:438-451 在 475-478 之前 |
| 6 | osc52===true && clipboardOK 时 ClipboardAddon write-only 加载 | ✓ VERIFIED | main.ts:460-467（readText 恒 resolve ''；writeText 含 WR-02 catch-resolve 修复） |
| 7 | 优先级 URL query > --client-option > 内置默认 | ✓ VERIFIED | 构造初值扩散（93）+ queryKeys 跳过（407, 439）双机制 |
| 8 | node --test 锁定 parseQueryPrefs/splitPrefs/mergeTheme | ✓ VERIFIED | 实跑通过（9 用例） |
| 9 | dist 重建入库 | ✓ VERIFIED | 重建哈希一致（见 04-02 T9） |

#### 04-06（文档 + 全量验证，6 truths）— 6/6 VERIFIED

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | README flag 表新增 --client-option/--osc52 两行 | ✓ VERIFIED | README.md:37-38（含 WR-03 修复：单引号包裹示例 + 防 shell 剥引号注释） |
| 2 | README 前端体验节（标题/超链接/剪贴板 HTTPS 明示/浮层/偏好优先级） | ✓ VERIFIED | README.md:86-92（D-11 加粗明示"需 HTTPS 或 localhost"） |
| 3 | README 协议节 Welcome payload 含可选 prefs | ✓ VERIFIED | README.md:99 |
| 4 | 04-UAT.md 人工清单覆盖全部渲染面 | ✓ VERIFIED | 04-UAT.md 11 项（T1-T11）含 sticky activation 裁决记录 |
| 5 | 全量验证六段式 | ✓ VERIFIED | 本验证独立复跑：gofmt 0 脏文件 / go vet 净 / `go test -race -count=1` 三包全绿 / `tsc && vite build` 0.7s+193ms 且产物哈希与入库一致 / go build + UAT spawn 即启动冒烟（--port 0 解析实测） |
| 6 | 三套 UAT 回归绿 | ✓ VERIFIED | 实测：phase02 11/11、phase03 18/18、phase04 10/10 |

**Score:** 45/47 truths verified（2 条 backstop truth 代码面在、行为未验，转人工）

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/proto/proto.go` | Prefs 字段 + WelcomeFrame + 白名单 | ✓ VERIFIED | 86-141 行全部就位，单测锁定 |
| `internal/server/server.go` | ClientPrefs 注入与 Welcome 挂点 | ✓ VERIFIED | 102/146/442 行 |
| `internal/server/e2e_test.go` | TestWelcomePrefs | ✓ VERIFIED | 527 行，race 实测 ok |
| `cmd/wesh/main.go` | 两 flag + 聚合 + Options 传递 | ✓ VERIFIED | 101-121/166-176/271 行 |
| `cmd/wesh/main_test.go` | 错误表 + 聚合单测 | ✓ VERIFIED | 192-194 行含 osc52 key 拒绝行 |
| `internal/proto/proto_test.go` | 白名单表 + prefs 往返 | ✓ VERIFIED | 113-116 行 |
| `web/package.json` | 三 addon 钉版 | ✓ VERIFIED | unicode11 0.9.0/web-links 0.12.0/clipboard 0.2.0 精确钉版 |
| `web/pnpm-workspace.yaml` | js-base64 override 3.9.2 | ✓ VERIFIED | overrides 段在（pnpm 11 新址，注释记理据） |
| `web/src/lib/title.ts` | sanitizeTitle | ✓ VERIFIED | 含 WR-04 Cf 剥离（0x200b-0x200f/0x202a-0x202e/0x2066-0x2069/0xfeff） |
| `web/src/lib/title.test.ts` | sanitize 回归锁 | ✓ VERIFIED | 16/16 实跑通过 |
| `web/src/lib/prefs.ts` | parseQueryPrefs/splitPrefs/mergeTheme | ✓ VERIFIED | 三纯函数 + 白名单常量 |
| `web/src/lib/prefs.test.ts` | query 解析回归锁 | ✓ VERIFIED | 实跑通过（osc52 排除专项在） |
| `web/src/main.ts` | 全部前端接线 | ✓ VERIFIED | 598 行，逐段核实（见 truths 表） |
| `web/index.html` | resize-overlay + xterm-hover 样式 | ✓ VERIFIED | 32-45/67 行 |
| `web/dist/index.html` | 重建产物入库 | ✓ VERIFIED | 入库且**重建哈希字节级一致**；WR-01/02/04 修复检索串均在（minify 后十进制码点 8203/8234/8238/8294-8297/65279） |
| `web/uat/phase04.mjs` | 协议层自动化 UAT | ✓ VERIFIED | 实测 10/10 |
| `README.md` | flag 文档 + 前端体验节 + 协议更新 | ✓ VERIFIED | 37-38/86-92/99 行 |
| `.planning/phases/04-frontend/04-UAT.md` | 人工确认清单 | ✓ VERIFIED | 11 项 pending 待人工执行 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| cmd/wesh/main.go | internal/server/server.go | Options.ClientPrefs 装配 | ✓ WIRED | main.go:271 → server.go:102/146 |
| internal/server/server.go | internal/proto/proto.go | WelcomeFrame(mode, s.clientPrefs) | ✓ WIRED | server.go:442 |
| cmd/wesh/main.go | internal/proto/proto.go | ValidClientOptionKey 校验 | ✓ WIRED | main.go:107 |
| web/src/main.ts | web/package.json | addon 命名导入 | ✓ WIRED | main.ts:5-7 三 addon import |
| web/src/main.ts | web/src/lib/title.ts | onTitleChange→sanitizeTitle | ✓ WIRED | main.ts:8, 267 |
| web/src/main.ts | web/index.html | xterm-hover/resize-overlay | ✓ WIRED | main.ts:142/248 → index.html:45/67 |
| web/src/main.ts | web/src/lib/prefs.ts | parseQueryPrefs/splitPrefs/mergeTheme | ✓ WIRED | main.ts:9, 57, 63, 68, 404 |
| web/src/main.ts | internal/server/server.go | w.prefs 消费 | ✓ WIRED | main.ts:399 消费 Welcome prefs；UAT S2-S6 实测端到端通 |
| web/src/main.ts | main.ts 构造段 | defaultTheme 三处同源 | ✓ WIRED | 29/68/90/418 行 |
| web/src/main.ts | main.ts WELCOME 分支 | welcomeDone 门 | ✓ WIRED | 247（浮层门）/475（置位）/476-478（beforeunload 注册） |
| web/uat/phase04.mjs | cmd/wesh/main.go + internal/proto/proto.go | spawn CLI 驱动 + Welcome JSON 断言 | ✓ WIRED | 实测 10/10 |
| README.md | cmd/wesh/main.go | flag 表与 CLI 契约一致 | ✓ WIRED | README:37-38 与 main.go:101/121 help 文案同源 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| 标题同步 | remoteTitle → document.title | xterm onTitleChange（OSC 0/2 真实序列） | ✓ | FLOWING |
| 超链接 tooltip | linkTooltip.textContent | web-links/OSC8 hover 回调真实 URI | ✓ | FLOWING |
| 选中复制 | term.getSelection() → clipboard.writeText | 真实选区 | ✓ | FLOWING |
| prefs 应用 | w.prefs → term.options | 服务端 aggregateClientPrefs 真实聚合（UAT S2-S6 实测值相等） | ✓ | FLOWING |
| resize 浮层 | onResize {cols,rows} → overlay.textContent | xterm 真实尺寸事件 | ✓ | FLOWING |
| query prefs | location.search → parseQueryPrefs | 真实 URL | ✓ | FLOWING |

无静态兜底/硬编码/mock 数据流。

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 前端纯函数测试（title/prefs） | `node --test src/lib/title.test.ts src/lib/prefs.test.ts` | 16/16 pass（含 Cf 剥离 4 断言） | ✓ PASS |
| Go 全量测试 | `go test -count=1 ./cmd/wesh ./internal/proto ./internal/server` | 三包全 ok | ✓ PASS |
| Go race 测试 | `go test -race -count=1`（同三包） | 全 ok（server 6.6s） | ✓ PASS |
| gofmt 清洁度 | `gofmt -l ./cmd ./internal` | 0 脏文件 | ✓ PASS |
| go vet | `go vet`（同三包） | 干净 | ✓ PASS |
| TS 类型检查 | `tsc --noEmit`（web/） | 零错误 | ✓ PASS |
| 构建链 + dist 同步 | `tsc && vite build`，前后 SHA256 对比 | 468ef8c298413d0d → 468ef8c298413d0d 字节级一致，git diff 空 | ✓ PASS |
| 协议 UAT（本 phase） | `node web/uat/phase04.mjs <新构建二进制>` | 10/10 PASS（S1-S6 + E1-E4） | ✓ PASS |
| 前序 UAT 回归 | `node web/uat/phase02.mjs / phase03.mjs` | 11/11 + 18/18 PASS | ✓ PASS |
| 废弃 API 清查 | `grep -r execCommand web/src web/index.html` | 0 引用 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CORE-03 | 04-02 | 终端标题变化同步到浏览器标签页标题 | ✓ SATISFIED（渲染面待 T3 人工） | onTitleChange→sanitizeTitle→setTitle 单一写口 + [ro] 前缀恒最前 + 16 单测 |
| FE-02 | 04-02 | Unicode 11 宽字符，CJK/IME 正常输入显示 | ✓ SATISFIED（IME/渲染待 T1/T2 人工） | Unicode11Addon 加载+激活硬顺序；输入链路不改 |
| FE-04 | 04-02 | URL 自动识别为可点击超链接 | ✓ SATISFIED（hover 视觉待 T4 人工） | web-links 默认 + OSC8 linkHandler + 双通道 tooltip |
| FE-05 | 04-03 | 选中即复制，navigator.clipboard 现代 API | ✓ SATISFIED（真实剪贴板待 T5-T7 人工） | 防抖复制 + 粘贴双门 + execCommand 零引用 |
| FE-06 | 04-03/04-05 | resize 浮层 + 离开确认可开关 | ✓ SATISFIED（视觉时序待 T8/T9 人工） | 双门浮层 + 条件注册/任意 onclose 移除 + prefs/query 开关接线 |
| FE-07 | 04-01/04-04/04-05 | 服务端偏好下发，URL query 可覆盖 | ✓ SATISFIED（视觉效果待 T10 人工） | Go 全链 + 前端双通道 + 优先级机制 + UAT 10/10 实测 |

REQUIREMENTS.md Traceability 表 Phase 4 六条（CORE-03/FE-02/FE-04/FE-05/FE-06/FE-07）与 PLAN frontmatter requirements 字段完全对齐，**无 ORPHANED 需求**。

### Prohibitions 核验（must-NOT 全部落实，零违反）

| # | Prohibition | Status | Evidence |
|---|-------------|--------|----------|
| 1 | osc52 不入任何用户侧通道 | ✓ 落实 | Go 白名单（proto.go:128-141）与 TS 白名单（prefs.ts:6-18）均不含 osc52；prefs.test.ts:30 与 UAT E3 双锁；README:38 明示 |
| 2 | server/proto 不二次解析 prefs | ✓ 落实 | json.RawMessage 端到端透传，无 Unmarshal |
| 3 | 服务端不做 OSC 序列过滤 | ✓ 落实 | OUTPUT 零拷贝直写（main.ts:382） |
| 4 | 报错/日志不含 prefs 值内容 | ✓ 落实 | main.go:104/108/113 仅含 key 名与错误类别；console.warn 只打键名（main.ts:428 等） |
| 5 | 无自定义 web-links 正则/handler | ✓ 落实 | main.ts:119 第一参 undefined |
| 6 | 标题写口不旁路 sanitizeTitle | ✓ 落实 | document.title 唯一赋值点 main.ts:208 |
| 7 | 无 webfont/运行时网络资源 | ✓ 落实 | 单 HTML 构建，fontFamily 为 OS 等宽栈 |
| 8 | 无 OSC 1 兼容代码 | ✓ 落实 | main.ts:264-265 注释明示接受现状 |
| 9 | 剪贴板读仅显式手势、ro 永不读 | ✓ 落实 | readText 唯一调用点 main.ts:300（Ctrl+Shift+V + isRO 门） |
| 10 | 无 execCommand 回落 | ✓ 落实 | 全仓零引用 |
| 11 | beforeunload 无自定义文案、终结后移除 | ✓ 落实 | main.ts:201-203/532 |
| 12 | UAT 不打值内容 | ✓ 落实 | 实测输出仅形状/布尔 |
| 13 | UAT 每场景独立 spawn | ✓ 落实 | 实测通过（单次语义下二次建连不可行，10 场景各自 spawn） |
| 14 | 文档不暗示 osc52 用户侧开启 | ✓ 落实 | README:38 "只能经本 flag 开启" |
| 15 | README 不弱化 HTTPS/localhost 明示 | ✓ 落实 | README:90 加粗明示 |
| 16 | 白名单外 xterm option 三通道不应用 | ✓ 落实 | prefs.ts:33 结构性过滤 + UAT E1 |
| 17 | OSC52 readText 恒 resolve '' | ✓ 落实 | main.ts:462 |
| 18 | behavior 键不写 term.options | ✓ 落实 | main.ts:438-451 仅写开关量 |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | TODO/FIXME/XXX/PLACEHOLDER 扫描（web/src、proto.go、main.go） | 无命中 | — |
| — | — | 空实现/硬编码空数据扫描 | 无命中 | — |
| web/src/main.ts | 58-59 | `export const queryKeys` 残留 export + 过时注释（REVIEW IN-01，Info 级不在修复范围） | ℹ️ Info | 入口模块死 API 面，无功能影响 |
| web/src/lib/prefs.ts | 64-66 vs main.ts 406-430 | theme 合并"同源经此函数"注释与 WELCOME 分支内联复刻漂移（REVIEW IN-02，Info 级） | ℹ️ Info | 今日行为等价，未来修 mergeTheme 可能静默漂移 |
| web/uat/phase04.mjs | 82-96 | dialHello 无超时（REVIEW IN-03，Info 级） | ℹ️ Info | 服务端回归时 UAT 挂死而非干净 FAIL |

三条 Info 均为 04-REVIEW.md 记录且 fix_scope=critical_warning 明确排除的项，不阻塞收口。

### Human Verification Required

自动化结构性不可达的渲染/浏览器平台行为面共 11 项（详见 frontmatter `human_verification`，与 04-UAT.md T1-T11 一一对应，当前全部 pending）。其中 2 条对应显式 backstop truths（T1/T2 与 T5-T9），其余为渲染面效果确认。执行方式见 04-UAT.md 前置条件（三种启动形态）。

### Gaps Summary

无 gaps。全部 47 条 truths 中 45 条自动化验证通过（含 10/10 协议 UAT 实测、16/16 前端单测实测、三包 go test -race 实测、dist 重建哈希字节级一致）；2 条 backstop truths 代码面已验、行为面按设计转人工。REVIEW 4 项 Warning 修复（WR-01 逐键容错 + 整段独立 try / WR-02 OSC52 writeText catch / WR-03 README 引号 / WR-04 Cf 剥离）全部核实落地且同步进 dist 产物。**Phase 收口条件 = 04-UAT.md 11 项人工确认全部 PASS。**

---

_Verified: 2026-08-19T00:04:53Z_
_Verifier: Claude (gsd-verifier)_
