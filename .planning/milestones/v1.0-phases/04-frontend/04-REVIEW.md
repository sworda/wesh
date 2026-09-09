---
phase: 04-frontend
reviewed: 2026-08-18T23:36:00Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - internal/proto/proto.go
  - internal/proto/proto_test.go
  - internal/server/e2e_test.go
  - internal/server/server.go
  - README.md
  - web/index.html
  - web/package.json
  - web/pnpm-workspace.yaml
  - web/src/lib/prefs.test.ts
  - web/src/lib/prefs.ts
  - web/src/lib/title.test.ts
  - web/src/lib/title.ts
  - web/src/main.ts
  - web/tsconfig.json
  - web/uat/phase04.mjs
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-08-18T23:36:00Z
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

评审覆盖 Phase 4 全链：CLI（`--client-option`/`--osc52` parse 期校验与聚合）、proto（Welcome prefs 载荷 + 白名单）、server（Welcome 内嵌下发）、前端（query/prefs 双通道消费、theme 合并、标题、剪贴板、OSC52、浮层、beforeunload）、UAT 与文档。`web/dist/index.html`（生成产物）与 `web/pnpm-lock.yaml`（锁文件）按范围规则排除逐行评审，但做了抽查：dist 内含 `confirmBeforeUnload`/`resize-overlay`/`osc52` 等新代码（与 src 同步），lockfile 中 `js-base64` override 生效为 3.9.2。

验证基线：`go vet` 三个包干净；`go test -count=1 ./cmd/wesh ./internal/proto ./internal/server` 全绿；`pnpm exec tsc --noEmit` 通过；`node --test src/lib/*.test.ts` 15/15 通过。

**已核实无问题的安全面**（负面结论有据）：
- OSC8 非 http(s) 协议在 xterm 核心 `OscLinkProvider.ts:72-77` 建链前即被过滤（`allowNonHttpProtocols` 未设 → false），`javascript:` URI 结构性到不了自定义 `linkHandler.activate`，`window.open` + `opener=null` 形态正确。
- `osc52` 双通道结构性排除核实：Go 白名单（proto.go:128-141）与 TS 白名单（prefs.ts:6-18）均不含 osc52，query 通道测试锁死（prefs.test.ts:30-34），E3 UAT 锁死 CLI 通道。Go/TS 白名单逐键比对一致（恰 10 键）。
- SEC-01 启动面红线：main.go:100-118 记录式错误上报正确绕开 flag 包的 `%q` 回显泄露面；TestClientOptionError 的 forbiddenSub 断言与 TestStartupMatrix 凭据值禁入断言均在。
- prefs 合并 last-wins 三端一致（aggregateClientPrefs map 覆盖 / parseQueryPrefs 后者覆盖 / main.ts queryKeys 跳过），e2e + UAT S6 双锁。
- `onChunk` 组帧缓冲 32KiB 与 pty ReadLoop 读块 32KiB 恰好一致，无截断面。

**关键关切**：4 个 WARNING 全部位于「值域校验缺口」与「文档/防御一致性」主题——服务端只验 JSON 形态不验值域的设计在前端两个消费点（xterm 运行时选项赋值、OSC52 write 回链）留下了未接住的硬边界，另有 README 示例在真实 shell 下必然失败、标题 sanitize 未覆盖 bidi 控制字符。

## Critical Issues

无。

## Warnings

### WR-01: WELCOME prefs 应用循环内 xterm 选项赋值可抛异常——FE-06 开关门（welcomeDone/beforeunload）静默永久失效

**File:** `web/src/main.ts:402-419`（抛出点 417；受害门 423/458-461）
**Issue:** WELCOME 分支用 `(term.options as unknown as Record<string, unknown>)[k] = v` 运行时赋值 xterm 选项。xterm 6 的 `OptionsService` setter 走 `_sanitizeAndValidateOption`（node_modules/@xterm/xterm/src/common/services/OptionsService.ts:127-207），对值域非法值**抛异常**而非静默：`cursorStyle` 非 block/underline/bar 即 throw（156 行）、`lineHeight < 1` throw（177-178）、`scrollback < 0` throw（186-187）。服务端 `--client-option` 只验「白名单键 + 合法 JSON」不验值域（设计如此，main.go:111-115），故 `--client-option 'cursorStyle="beam"'`、`'lineHeight=0'`、`'scrollback=-1'` 均为合法启动配置，下发后在 417 行抛异常。整个 WELCOME 处理被 462 行的大 try/catch 吞掉，导致：(1) `welcomeDone = true`（458 行）与 `beforeunload` 注册（459-461）被跳过——resize 浮层与离开确认两个默认开的功能**静默永久死亡**，无任何用户可见信号（catch 文案 "discard malformed WELCOME frame" 还误导排障）；(2) `fit.fit()`（423 行）跳过——若抛出前已应用 fontSize 等改单元格尺寸的键（Go map marshal 按字母序，fontSize 先于 lineHeight/scrollback/theme），cols/rows 与视口不符，远端 TUI 画错直至下次窗口 resize；(3) 抛出点之后的 behavior 键与 osc52 加载不再执行。对比：query 通道走构造路径，OptionsService 构造器对逐键 try/catch（OptionsService.ts:79-84）天然免疫——同一份非法值，query 通道只丢该键，WELCOME 通道死全部开关门，两通道容错不对称（违反 D-16「非法输入不该让终端受损」意图）。

**Fix:**
```ts
for (const [k, v] of Object.entries(parts.xterm)) {
  if (queryKeys.has(k)) { continue; }
  try {
    if (k === 'theme') { /* …原有分支… */ }
    else { (term.options as unknown as Record<string, unknown>)[k] = v; }
  } catch {
    console.warn(`ignoring invalid pref value: ${k}`); // 值内容不入日志，同 SEC-01 纪律
  }
}
```
并将 `welcomeDone = true` 与 beforeunload 注册移出 prefs try 块（或置于 finally 等价位置），保证单个非法偏好永不拖累会话建立门。

### WR-02: OSC52 writeText 直透 `navigator.clipboard.writeText`——拒绝时经 xterm 核心微任务硬抛（Pitfall 4 只防了 read 半侧）

**File:** `web/src/main.ts:447-453`（450 行 writeText）
**Issue:** 注释明确援引 RESEARCH §Pitfall 4（「核心异步 OSC 链对 rejected promise rethrow 成 unhandled rejection」）为 readText 恒 resolve `''` 辩护，但同链路的 writeText 原样返回 `navigator.clipboard.writeText(text)`（450 行）。核实链路：ClipboardAddon._setOrReportClipboard 对 Promise 结果 `result.then(() => true)`（addon-clipboard/src/ClipboardAddon.ts:63-66）不挂 catch → 拒绝透传到 xterm 核心 WriteBuffer.ts:214-217：`result.catch(err => { queueMicrotask(() => {throw err;}); … })`——**微任务里硬 throw**，落 window.onerror。触发场景现实：远程程序在页面失焦（用户切到别的窗口/标签页）时输出 OSC52 写序列，Chrome 以 "Document is not focused" NotAllowedError 拒绝 clipboard.writeText——每次远端写剪贴板就在控制台炸一个未捕获错误。功能上剪贴板写失败本应静默（与 288 行选中复制失败 .catch 静默同纪律），当前实现把可预期的权限/焦点拒绝放大成页面级未捕获异常。

**Fix:**
```ts
writeText: (_sel, text): Promise<void> =>
  navigator.clipboard.writeText(text).catch((e) => console.warn('osc52 clipboard write failed', e)),
```
与 readText 的「resolve 空串协议完整」同形：catch 后 resolve，OSC 链完整且零硬抛。

### WR-03: README `--client-option` theme 示例在真实 shell 下必然启动失败

**File:** `README.md:37`
**Issue:** 表格内示例写作 `theme={"background":"#000"}`（未加外层引号）。实测 fish 与 bash：该词经引号去除后变为 `theme={background:#000}`（`{"background":"#000"}` 无未引用逗号不触发 brace expansion，但双引号被 shell 剥掉）——非法 JSON，wesh 以 exit 2 `not valid JSON` 拒绝启动。用户照抄文档必踩坑，且错误文案（"not valid JSON"）不指向引号问题，排障体验差。同文件 92 行「字符串值需 JSON 引号并 URL 编码」说的是 query 通道，未覆盖 CLI 通道的 shell 引号剥离。UAT 脚本（phase04.mjs:149）用的是正确的单引号形态 `'theme={"background":"#000"}'`。

**Fix:** README 示例改为 `'theme={"background":"#000"}'`（单引号包裹），或在 `--client-option` 行补一句「含引号的 JSON 值需整体单引号包裹防 shell 剥引号」。

### WR-04: 标题 sanitize 未剥离 Unicode 格式控制字符（Cf）——bidi 覆盖/零宽字符钓鱼面残留

**File:** `web/src/lib/title.ts:8-11`
**Issue:** `sanitizeTitle` 只剥离 C0/DEL/C1 控制码点。远程 OSC 0/2 是不可信输入（文件注释自承 T-04-04 标题注入伪装主机名/路径钓鱼面），但 U+202A-202E（bidi 嵌入/覆盖，浏览器标签页标题走 Unicode bidi 算法，U+202E 可把 `evil.com` 视觉反转为 `moc.live`）、U+200B-200F（零宽/LRM/RLM）、U+2066-2069（bidi isolate）、U+FEFF 等 Cf 字符全部原样放行。经典标题钓鱼手法（伪装主机名/路径）恰好依赖这类字符而非 C0/C1。若为 UI-SPEC §Title Sync Contract 逐字契约的刻意取舍，建议在契约注释中明示「Cf 不过滤为已接受残余风险」；否则应补剥离。

**Fix:**
```ts
return !(cp <= 0x1f || cp === 0x7f || (cp >= 0x80 && cp <= 0x9f) ||
  (cp >= 0x200b && cp <= 0x200f) || (cp >= 0x202a && cp <= 0x202e) ||
  (cp >= 0x2066 && cp <= 0x2069) || cp === 0xfeff);
```
并补对应 node --test 用例（如 `'\u202emoc.live'` 剥离后不含 U+202E）。

## Info

### IN-01: `queryKeys` 的 export 防误报注释已过时

**File:** `web/src/main.ts:58-59`
**Issue:** 注释称「export 防 noUnusedLocals 在接线前误报」，但 queryKeys 已在 403/428 行被 WELCOME 分支实际使用——`noUnusedLocals` 不会对已使用变量报错，export 成了入口模块上的死 API 面，注释与现实脱节（接线完成的残留脚手架）。
**Fix:** 去掉 `export` 与过时注释，改为普通 `const queryKeys = query.keys;`。

### IN-02: prefs.ts 注释宣称 theme 合并「两通道同源经此函数」，实际 WELCOME 分支内联复制了一份

**File:** `web/src/lib/prefs.ts:64-66` 与 `web/src/main.ts:406-413`
**Issue:** mergeTheme 的注释称「构造段（query 通道）与 WELCOME 分支（prefs 通道）theme 合并同源经此函数」，但 main.ts WELCOME 分支并未调用 mergeTheme，而是手写复刻了其判空/判数组 + 展开合并逻辑（406-413 行）。今日行为逐字等价，但「单一事实源」的注释声明为假——未来修 mergeTheme（如键过滤）不会波及 WELCOME 分支，两通道静默漂移。另注意两通道对非对象 theme 的处置本就不同（query 回退默认调色板 / WELCOME 保留现值），注释也未覆盖这层差异。
**Fix:** WELCOME 分支改为调用 `mergeTheme(defaultTheme, v)`，`null` 时走现有 warn 分支；或修正注释如实描述两份实现。

### IN-03: UAT `dialHello` 无超时——服务端回归时挂死而非干净报错

**File:** `web/uat/phase04.mjs:82-96`
**Issue:** `dialHello` 只在见到 WELCOME 帧（resolve）或 onclose（reject）时落定；若服务端回归为「接受 WS 但既不发 Welcome 也不关闭」，promise 永不落定，整个 UAT 无限挂起（startWesh 有 8s 超时、dialHello 没有）。影响测试可靠性：CI/人工执行表现为卡死而非 FAIL。
**Fix:** 加 5s `setTimeout` 兜底 reject（`握手超时（未收到 Welcome）`），落定后 clearTimeout。

---

_Reviewed: 2026-08-18T23:36:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
