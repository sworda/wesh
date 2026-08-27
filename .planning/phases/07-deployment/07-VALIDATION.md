---
phase: 7
slug: deployment
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: false
wave_0_complete: true
created: 2026-08-25
validated: 2026-08-27
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib + `go vet`；UAT: Node 原生脚本（零依赖）+ jsdom 25.0.1 + @xterm/headless；浏览器实测层：Playwright Chromium（Windows 工作站侧，web/uat/pw/） |
| **Config file** | 无单独配置（go test 内建）；CI 见 .github/workflows/ci.yml |
| **Quick run command** | `go test ./cmd/wesh ./internal/pty ./internal/server`（<5s 级；按任务 -run 收窄） |
| **Full suite command** | `go test -race -count=1 ./...`（~50s）+ `time pnpm -C web build` + `node web/uat/phase07.mjs` |
| **Estimated runtime** | quick <5 秒；full ~50 秒 + UAT（pw 全链回归 ~2min 含 65s 空闲窗） |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/wesh ./internal/pty ./internal/server -run '<相关 TestX>' -count=1`
- **After every plan wave:** Run `go test -race -count=1 ./...` + `time pnpm -C web build`
- **Before `/gsd:verify-work`:** Full suite must be green + phase07.mjs 全场景 PASS + 既有 phase02-06 UAT 回归
- **Max feedback latency:** 50 秒

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-06 T1/T2 | 07-06 | W1 | OPS-09 | T-07-06b/c | 配置错误文案零敏感值回显；未知键严格拒绝 | unit + UAT | `go test -run 'TestLoadFileConfig\|TestConfigMerge\|TestConfigPrecedence\|TestConfigRedLines\|TestParseArgsWithConfig' ./cmd/wesh`；`node web/uat/phase07.mjs`（S1）；`bash web/uat/phase07-b1b5.sh`（B5 4/4） | ✅ | ✅ green |
| 07-02 T1-T3 + 07-10 T1 | 07-02/07-10 | W1 | OPS-01 | T-07-02a/b、T-07-10a/b | listen 前 Remove + 活性探测拒存活 socket + listen 后显式 Chmod/Chown | unit + UAT | `go test ./cmd/wesh -run TestListenSocket`（六子测含 live instance refused）；`node web/uat/phase07.mjs`（S2）；`bash web/uat/phase07-b1b5.sh`（7/7） | ✅ | ✅ green |
| 07-01 T1-T3 + 07-08 + 07-09 T1/T2 | 07-01/07-08/07-09 | W1/W2 | OPS-02 | T-07-01a/b、T-07-09a/b | base-path 页面/WS 升级/308/share 交叉；nginx 配方 Host $http_host 全链通 | unit + UAT + pw | `go test ./internal/server -run 'TestBasePathRoutes\|TestBasePathWS\|TestBasePathEmptyUnchanged\|TestNormalizeOrigin\|TestOriginAllowed'`；`node web/uat/phase07.mjs`（S3a-h）；`node web/uat/pw/phase07-a2-pw.mjs`（5/5，2026-08-27） | ✅ | ✅ green |
| 07-03 T1-T3 | 07-03 | W1 | SEC-07 | T-07-03a/b/c/d | remote_user sanitize C0/C1/DEL+128；XFF 换键节流；多值头取首值 | unit + UAT | `go test ./internal/server -run 'TestRemoteUserLogging\|TestSanitizeRemoteUser\|TestXFFThrottleKey\|TestProxyClientIP\|TestShareChannelRemoteUser'`；`node web/uat/phase07.mjs`（S4）；`node web/uat/phase07-b2.mjs`（4/4） | ✅ | ✅ green |
| 07-04 T1-T3 | 07-04 | W1 | OPS-04 | T-07-04a/d | pgid==子进程 pid（setsid 不变量）；--cwd stat 预检 | unit + UAT | `go test ./internal/pty -run 'TestStartOptionsDir\|TestSignalGroup\|TestSignalGroupHangup'`；`node web/uat/phase07.mjs`（S5） | ✅ | ✅ green |
| 07-04 T3 | 07-04 | W1 | OPS-05 | T-07-04b/c | uid/gid 成对强制 exit 2；HOME/USER/LOGNAME 按 LookupId 改写或剔除 | unit + UAT | `go test ./internal/pty -run 'TestDropPrivilegesSelf\|TestDropPrivilegesIdentityEnv'`；`node web/uat/phase07.mjs`（S6a/S6b 降权 self 全链）；nobody root 面 → Manual-Only | ✅ | ✅ green |
| 07-05 T1-T3 + 07-10 T2 | 07-05/07-10 | W1 | OPS-11 | T-07-05b/c、T-07-10c/d | headless 跳过 --open；fake xdg-open argv 断言；opener 非零退出警告行不含 URL；Wait 收割零僵尸 | unit + UAT | `go test ./cmd/wesh -run TestOpenBrowser`（三子测含 non-zero opener warns）；`node web/uat/phase07.mjs`（S8a/S8b，S8c 平台豁免 skip）；`bash web/uat/phase07-b6.sh`（7/7） | ✅ | ✅ green |
| 07-05 T2 | 07-05 | W1 | D-23 | T-07-05a/d | 1001 关停序列（SIGTERM → 1001 广播 → 退出码 255） | UAT + DOM | `node web/uat/phase07.mjs`（S7 关停场景）；phase07-dom 面板断言 | ✅ | ✅ green |
| 07-07 T1-T3 | 07-07 | W1 | 全量横切 | T-07-07a | UAT 输出自净（凭据/token 零命中） | UAT 基建 | `node web/uat/phase07.mjs`（34/34，assertOutputClean 运行时自证）；十脚本回归清单 | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*2026-08-27 刷新：执行后全量回填（plan-phase 播种的 TBD 占位全部替换为实测测试面）；07-09/07-10 gap-closure 覆盖行并入。*

---

## Wave 0 Requirements

- [x] `web/uat/phase07.mjs` — 覆盖 OPS-01/02/04/09/11、SEC-07、D-23 的协议层场景（含 TCP relay 夹具）——34/34 PASS（S8c 平台豁免 skip）
- [x] `cmd/wesh/main_test.go` 扩展 — 配置文件/新 flag 组合校验表驱动用例（TestParseArgs/TestStartupMatrix/TestListenSocket/TestOpenBrowser 等全绿，-race 干净）

*既有 Go 测试基建完备，无框架安装需求；jsdom/xterm-headless 已在 web/uat 就位；Playwright Chromium 在 Windows 工作站侧就位（web/uat/pw/）。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 浏览器自动打开真实效果（真实 OS 弹窗观感） | OPS-11 | wesh 只能跑在 Linux headless 开发机（真实弹窗不可达）；Windows 侧 wesh 不可构建（既定 Out of Scope）——平台原生行为豁免（CODEBUDDY.md 测试策略第 5 条），等价面 S8a/S8b + b6.sh 已自动化闭合 | 有 GUI 环境运行 `wesh --open --writable -- bash` 人工确认——**2026-08-27 用户明示风险接受（07-UAT.md A1 skipped）** |
| root 降权 nobody（附加组清空 / 无 shell 场景） | OPS-05 | 需 root 执行降权；Linux 开发机 sudo 损坏无提权通道——环境限制；降权 self 面已由 phase07.mjs S6a/S6b 自动化覆盖 | 有 root 通道机器上 `sudo ./wesh --uid 65534 --gid 65534 -- bash` 后 attach 复核 `id` / `echo $HOME $USER $LOGNAME`——**2026-08-27 用户明示风险接受（07-UAT.md B4 skipped）** |
| macOS open 真实弹窗 | OPS-11 | 无 Mac 环境；darwin 分支为构建标签差异——平台豁免 | macOS 上 `wesh --open -- bash` 人工确认——**平台豁免登记（phase07.mjs S8c skip+reason）** |

*注：原 Manual-Only「反代真实 nginx 挂载观感」（OPS-02）已于 2026-08-27 移出——phase07-a2-pw.mjs 双机全链（Windows Chromium → LAN nginx 1.14.1 → wesh）5/5 自动化闭合，截图留档 web/uat/pw/screenshots/。*

---

## Validation Audit 2026-08-27

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0（无新 gap——全部需求已有自动化覆盖在案） |
| Escalated | 0 |
| Manual-Only 残余 | 3（A1/B4/macOS open——全部用户风险接受或平台豁免登记） |

*刷新动因：plan-phase 播种的 draft 全 TBD 占位 + 07-09/07-10 gap-closure 改动落地后的覆盖面回填；验证者复验 55/55 must-haves passed（07-VERIFICATION.md，2026-08-27）。*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 50s
- [ ] `nyquist_compliant: true` set in frontmatter（3 项 Manual-Only 风险接受/平台豁免残余——PARTIAL 形态既定）

**Approval:** validated 2026-08-27（codebuddy，validate-phase State A 刷新；零 gap 未 spawn auditor）
