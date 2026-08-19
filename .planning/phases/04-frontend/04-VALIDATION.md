---
phase: 04
slug: frontend
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-18
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Filled by plan-phase (6 plans / 4 waves)；渲染面人工项汇总于 04-UAT.md（04-06 创建）。

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`（协议/CLI 契约锁定）+ Node 零依赖 UAT 脚本（web/uat/*.mjs 先例）+ `node --test`（web/src/lib/*.test.ts 纯函数，Node 24 内建 type stripping 零新依赖）+ `tsc --noEmit` 类型门（随构建） |
| **Config file** | none（Go 标准测试；UAT 脚本与 node --test 零配置；tsconfig 既有，`exclude: ["src/**/*.test.ts"]` 由 04-02 加入） |
| **Quick run command** | `go test ./internal/... ./cmd/... -count=1 && pnpm -C web exec tsc --noEmit` |
| **Full suite command** | `go test -race -count=1 ./... && time pnpm -C web build && go build -o /tmp/wesh-uat/wesh ./cmd/wesh && node web/uat/phase04.mjs /tmp/wesh-uat/wesh` |
| **Estimated runtime** | ~90 秒（-race 全量 + web 构建 + UAT 十场景） |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/... ./cmd/... -count=1 && pnpm -C web exec tsc --noEmit`（Go 任务跑前半，前端任务跑后半）
- **After every plan wave:** Run full suite command（Wave 2 起含 phase04.mjs；Wave 1 无 UAT 脚本可跳过该段）
- **Before `/gsd:verify-work`:** Full suite must be green + phase02/phase03 UAT 回归绿 + 04-UAT.md 人工项用户确认
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-01-1 | 01 | 1 | FE-07 | T-04-01/02 | 白名单 fail-fast；osc52 不入用户侧通道 | e2e+unit | `go test ./internal/server -run TestWelcomePrefs -count=1 && go test ./internal/proto ./cmd/wesh -count=1` | ✅（e2e_test.go 扩） | ✅ green |
| 04-01-2 | 01 | 1 | FE-07 | T-04-01/02/03 | 错误文案不含值；白名单边界锁定 | unit | `go test ./internal/proto ./cmd/wesh -count=1` | ✅（两测试文件扩） | ✅ green |
| 04-02-1 | 02 | 1 | FE-02/FE-04 | T-04-05/06/SC | 库默认 handler opener=null；linkHandler 显式设置 | build | `pnpm -C web install && pnpm -C web exec tsc --noEmit` | ✅ | ✅ green |
| 04-02-2 | 02 | 1 | CORE-03 | T-04-04 | sanitize 剥离+截断防标题注入 | unit | `node --test web/src/lib/title.test.ts && pnpm -C web exec tsc --noEmit` | ❌ W0（lib/title.ts+test 随任务创建） | ✅ green |
| 04-02-3 | 02 | 1 | CORE-03/FE-02/FE-04 | — | N/A | build | `time pnpm -C web build && go build ./...` | ✅ | ✅ green |
| 04-03-1 | 03 | 2 | FE-05 | T-04-08/09 | 手势外不读剪贴板；失败静默 | build | `pnpm -C web exec tsc --noEmit` | ✅ | ✅ green |
| 04-03-2 | 03 | 2 | FE-06 | T-04-10 | 会话终结移除 beforeunload listener | build | `pnpm -C web exec tsc --noEmit` | ✅ | ✅ green |
| 04-03-3 | 03 | 2 | FE-05/FE-06 | — | N/A | build | `time pnpm -C web build && go build ./...` | ✅ | ✅ green |
| 04-04-1 | 04 | 2 | FE-07 | T-04-11 | detail 不打值内容 | e2e | `node --check web/uat/phase04.mjs` | ❌ W0（随任务创建） | ✅ green |
| 04-04-2 | 04 | 2 | FE-07 | T-04-12 | 测试实例隔离（临时端口/路径） | e2e | `node web/uat/phase04.mjs /tmp/wesh-uat/wesh && node web/uat/phase02.mjs /tmp/wesh-uat/wesh && node web/uat/phase03.mjs /tmp/wesh-uat/wesh` | ❌ W0（同上） | ✅ green |
| 04-05-1 | 05 | 3 | FE-07 | T-04-13/16 | osc52 排除出 query；非法 query 静默 | unit | `node --test web/src/lib/prefs.test.ts && pnpm -C web exec tsc --noEmit` | ❌ W0（lib/prefs.ts+test 随任务创建） | ✅ green |
| 04-05-2 | 05 | 3 | FE-06/FE-07 | T-04-14/15 | OSC52 write-only；theme 合并 | build | `pnpm -C web exec tsc --noEmit` | ✅ | ✅ green |
| 04-05-3 | 05 | 3 | FE-06/FE-07 | — | N/A | e2e | `time pnpm -C web build && node web/uat/phase04.mjs /tmp/wesh-uat/wesh` | ✅ | ✅ green |
| 04-06-1 | 06 | 4 | 全部 | T-04-17/18 | 文档不含 osc52 用户侧路径暗示 | docs | `grep -q '\-\-client-option' README.md && ! grep -q '?osc52' README.md` | ✅ | ✅ green |
| 04-06-2 | 06 | 4 | 全部 | — | N/A | full | 六段式全量 + 三套 UAT（见 Full suite command） | ❌ W0（04-UAT.md 随任务创建） | ✅ green |

*Status: ✅ green · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/uat/phase04.mjs` — Welcome prefs 形状/osc52 注入/client-option 启动拒绝十场景（04-04 创建，phase03.mjs 零依赖形态）
- [ ] `web/src/lib/title.ts` + `web/src/lib/title.test.ts` — sanitizeTitle 纯函数与 node --test 用例（04-02 创建）
- [ ] `web/src/lib/prefs.ts` + `web/src/lib/prefs.test.ts` — parseQueryPrefs/splitPrefs 纯函数与 node --test 用例（04-05 创建）
- [ ] `internal/server/e2e_test.go` 扩展 — dialHelloPayload helper + TestWelcomePrefs（04-01 创建）
- [ ] `internal/proto/proto_test.go` / `cmd/wesh/main_test.go` 表扩展 — prefs 往返/omitempty 缺席/白名单/错误表/聚合（04-01 创建）

*所有 Wave 0 项均由对应 plan 内任务创建，无前置独立 Wave 0 plan。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CJK/emoji 宽度与 IME 组合输入不丢字 | FE-02 | 真实 IME 栈与字体渲染自动化不可达（UI-SPEC 🧪 backstop） | 04-UAT.md T1/T2（04-06 创建） |
| 链接 hover 真实 URL/单击新标签页/OSC8 无 confirm 框 | FE-04 | 浏览器 hover/弹窗平台行为 | 04-UAT.md T4 |
| 选中即复制/Ctrl+Shift+V 粘贴/ro 不弹权限 | FE-05 | 剪贴板权限模型需真实浏览器手势 | 04-UAT.md T5/T6 |
| 明文 HTTP 非 localhost 剪贴板静默降级 | FE-05 | 非安全上下文需真实部署形态 | 04-UAT.md T7 |
| 浮层时序/淡出/开关关闭 | FE-06 | 视觉瞬态元素 | 04-UAT.md T8 |
| beforeunload 拦截与会话终结后放行 | FE-06 | 浏览器原生确认框（含 sticky activation 预期记录） | 04-UAT.md T9 |
| 标题同步与 [ro] 前缀保持 | CORE-03 | 标签页标题渲染面 | 04-UAT.md T3 |
| prefs 视觉效果/theme 合并/query 覆盖/OSC52 写入 | FE-07 | 渲染与剪贴板平台行为 | 04-UAT.md T10/T11 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（均随 plan 内任务创建，无悬空 MISSING）
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-19（validate-phase §6 复核：16/16 自动断言绿，无 gap）

---

## Validation Audit 2026-08-19

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

### 执行结果快照

| 测试 | 结果 |
|------|------|
| `go test ./internal/... ./cmd/...` | 4/4 包 ok |
| `node --test web/src/lib/{title,prefs}.test.ts` | 16/16 pass |
| `pnpm -C web exec tsc --noEmit` | clean |
| `web/uat/phase04.mjs`（S1-S6 + E1-E4） | 10/10 pass |
| `web/uat/phase02.mjs`（回归） | 11/11 pass |
| `web/uat/phase03.mjs`（回归） | 18/18 pass |
| `web/uat/phase04-t1-width.mjs`（@xterm/headless 宽度断言） | 5/5 pass |
| `web/uat/phase04-dom.mjs`（jsdom DOM 面） | 37/37 pass |

Wave 0 五项全部产出并就位；Manual-Only 8 项已在 04-UAT.md 记录并按项目约束风险接受（headless 环境豁免）。
