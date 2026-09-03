---
phase: 04-frontend
fixed_at: 2026-08-18T23:52:15Z
review_path: /data1/home/zexueli/open_src/wesh/.planning/phases/04-frontend/04-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 4: Code Review Fix Report

**Fixed at:** 2026-08-18T23:52:15Z
**Source review:** /data1/home/zexueli/open_src/wesh/.planning/phases/04-frontend/04-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4（fix_scope=critical_warning；0 Critical + 4 Warning；3 Info 不在范围）
- Fixed: 4
- Skipped: 0

**验证环境说明**：所有验证（`pnpm exec tsc --noEmit`、`node --test src/lib/*.test.ts`、`pnpm -C web build`、`go build ./...`、`go test ./internal/proto ./internal/server ./cmd/wesh -count=1`）均在隔离 worktree（`.codebuddy/worktrees/rf-04-*`，worktree 内 `pnpm install --frozen-lockfile` 装依赖）中运行；清理后主检出不含该 node_modules，数值需经 fast-forward 后在主检出重装依赖复现。

## Fixed Issues

### WR-01: WELCOME prefs 应用循环内 xterm 选项赋值可抛异常——FE-06 开关门静默永久失效

**Files modified:** `web/src/main.ts`, `web/dist/index.html`
**Commit:** 1d9f04f
**Applied fix:** xterm 选项应用循环改为逐键 try/catch（非法值域值如 `cursorStyle="beam"`/`lineHeight=0`/`scrollback=-1` 只丢该键并 `console.warn`，值内容不入日志，同 SEC-01 纪律），与 query 通道构造路径容错对称；同时将整个 prefs 应用段（xterm 键 → fit.fit() → behavior 键 → osc52 加载）包入独立 try/catch，使 `welcomeDone = true` 与 `beforeunload` 注册结构性不受单个非法偏好拖累；外层 catch 文案 "discard malformed WELCOME frame" 回归只覆盖 JSON 畸形帧，不再误导排障。

### WR-02: OSC52 writeText 直透 `navigator.clipboard.writeText`——拒绝时经 xterm 核心微任务硬抛

**Files modified:** `web/src/main.ts`, `web/dist/index.html`
**Commit:** bff65bb
**Applied fix:** `writeText` 改为 `navigator.clipboard.writeText(text).catch((e) => console.warn('osc52 clipboard write failed', e))`——catch 告警后 resolve，与 readText 恒 resolve `''` 同形（RESEARCH §Pitfall 4），页面失焦 NotAllowedError 等可预期拒绝不再被核心微任务硬抛成页面级未捕获异常；注释同步补写半侧纪律说明。

### WR-03: README `--client-option` theme 示例在真实 shell 下必然启动失败

**Files modified:** `README.md`
**Commit:** 32c3b26
**Applied fix:** 表格示例改为 `'theme={"background":"#000"}'`（整体单引号包裹），并补注「含引号的 JSON 值需整体单引号包裹，防 shell 剥引号」，与 UAT 脚本（phase04.mjs:149）实际可用的形态一致。

### WR-04: 标题 sanitize 未剥离 Unicode 格式控制字符（Cf）——bidi 覆盖/零宽字符钓鱼面残留

**Files modified:** `web/src/lib/title.ts`, `web/src/lib/title.test.ts`, `web/dist/index.html`
**Commit:** 3a238e7
**Applied fix:** `sanitizeTitle` 过滤条件扩展剥离 Cf：U+200B-200F（零宽/LRM/RLM）、U+202A-202E（bidi 嵌入/覆盖）、U+2066-2069（bidi isolate）、U+FEFF；文件头注释如实标注此为 UI-SPEC 逐字契约之外的加固及钓鱼面理据。`title.test.ts` 新增 Cf 剥离回归用例 4 断言（`'\u202emoc.live'` 等，转义序列形态与既有用例一致）；`node --test src/lib/*.test.ts` 16/16 通过。

## Skipped Issues

无——范围内 4 个 finding 全部修复。

---

_Fixed: 2026-08-18T23:52:15Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
