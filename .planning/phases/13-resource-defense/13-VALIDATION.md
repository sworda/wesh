---
phase: 13
slug: resource-defense
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-05
validated: 2026-09-06
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test（服务端）+ Node 原生 WebSocket/fetch 协议层 UAT 脚本（`web/uat/phaseNN.mjs` 先例，零依赖） |
| **Config file** | none — 既有 go test + web/uat 先例覆盖 |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test ./... && node web/uat/phase13*.mjs`（本 phase 新增 UAT 脚本，随 wave 落地） |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run `go test ./...` + 本 wave 涉及的 `web/uat/phase13*.mjs`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 13-01-T1 | 13-01 | 1 | PC-08（D-01 契约门） | T-13-01 | per-client stop-timeout 默认值公开契约变更 one-way 人工裁决 | manual-gate | —（checkpoint:decision，blocking 人工门） | — | ✅ gate-passed |
| 13-01-T2 | 13-01 | 1 | PC-08 | T-13-01/T-13-02 | stopTimeoutSet 显式位双源置位 + per-client 默认 5s KILL 兜底 + shared 默认 0 不变 + validateStartup warn | unit | `go build ./... && go vet ./... && time go test ./cmd/wesh/ -count=1` | ✅（cmd/wesh 测试组扩展） | ✅ green |
| 13-01-T3 | 13-01 | 1 | PC-08 | T-13-02 | 三态断言（显式 0 尊重+warn / 未置位双默认 / shared 0）+ TOML 键置位 | unit | `time go test ./cmd/wesh/ -count=1 -v -run 'TestStopTimeout\|TestValidateStartupWarnMerge' && time go test ./cmd/wesh/ -count=1` | ✅（同上） | ✅ green |
| 13-02-T1 | 13-02 | 1 | PC-08 | T-13-04/T-13-05/T-13-06/T-13-07 | spawn 双令牌桶 tracer：全局 8/s burst16 + per-IP 1/s burst4、1011 拒绝序列、throttled 计数器字段、单路径 e2e 断言 | unit + e2e | `go build ./... && go vet ./... && time go test -race -run 'TestPerClientSpawnThrottle' ./internal/server/ -count=1 -v && time go test -race ./internal/server/ -count=1` | ✅（perclient_test.go 扩展） | ✅ green |
| 13-02-T2 | 13-02 | 1 | PC-08 | T-13-07/T-13-08 | 全局/per-IP 桶独立判定、XFF 换键两态、15min 惰性过期、定值文案零敏感值 | unit | `time go test -race -run 'TestPerClientSpawn' ./internal/server/ -count=1 -v && time go test -race ./internal/server/ -count=1` | ✅（同上） | ✅ green |
| 13-03-T1 | 13-03 | 2 | PC-09 | T-13-09/T-13-10/T-13-11 | pcSupervisor 第二终结源 + 三形态触发端 + watcher 收割链 + WR-02 waitDone 非阻塞栅栏 + termOnce 单点 | unit（race） | `go build ./... && go vet ./... && time go test -race ./internal/server/ -count=1` | ✅（server/clients/perclient 机制 + 测试组扩展） | ✅ green |
| 13-03-T2 | 13-03 | 2 | PC-09, OPS-12 | T-13-09/T-13-10/T-13-12 | 退出码两时序逐位分叉 + exit-when-empty 三形态 255 + WR-02 栅栏白盒 + session_end schema 零敏感值 | unit + 进程级 | `time go test -race -run 'TestPerClientExitWhenEmpty\|TestPerClientOnce\|TestPerClientReapedFence\|TestPerClientSessionEnd\|TestEmptyExit' ./internal/server/ -count=1 -v && time go test -race ./internal/server/ -count=1` | ✅（emptyexit/events/perclient 测试扩展） | ✅ green |
| 13-04-T1 | 13-04 | 3 | PC-09 | T-13-13/T-13-14/T-13-15 | Shutdown N 进程组快照逐组 stop-signal + 有界 join + Broadcast 补行 + ESRCH 幂等 | unit | `go build ./... && go vet ./... && time go test -race ./internal/server/ -count=1` | ✅（shutdown_test.go 扩展） | ✅ green |
| 13-04-T2 | 13-04 | 3 | PC-09 | T-13-13/T-13-14 | 在线双端 + 断开未收割残留 + session_end 数==N + 有界 join 不拖死 | unit | `time go test -race -run 'TestPerClientShutdown' ./internal/server/ -count=1 -v && time go test -race ./internal/server/ -count=1` | ✅（同上） | ✅ green |
| 13-05-T1 | 13-05 | 3 | OPS-12 | T-13-16 | 四计数器 series + session_active 模式分支 + HELP 双模式 + 零身份 label 红线 + 镜像 17→21 | unit | `time go test -race -run 'TestMetrics' ./internal/server/ -count=1 -v && time go test -race ./internal/server/ -count=1` | ✅（metrics_test.go 扩展） | ✅ green |
| 13-05-T2 | 13-05 | 3 | OPS-12 | T-13-17/T-13-18 | healthz session_active 恒 true + session_start emit（pid+client_id 键集白名单）+ :421 断言翻转 | unit | `time go test -race ./internal/server/ -count=1` | ✅（perclient_test/events_test 扩展 + 既有断言翻转） | ✅ green |
| 13-06-T1 | 13-06 | 4 | SEC-09 | T-13-19/T-13-20 | StartOptions.RemoteUser + whitelistEnv 形参扩展（空串不出键）+ sanitize 清洗产物 | unit | `time go test ./internal/pty/ -count=1 -run 'TestEnvWhitelist' -v && time go test ./internal/pty/ -count=1` | ✅（spawn_test.go 扩展） | ✅ green |
| 13-06-T2 | 13-06 | 4 | SEC-09 | T-13-19/T-13-21 | SpawnFunc 签名扩散 + remoteUser 逐端到达断言（多客户端零串台） | unit + build | `go build ./... && go vet ./... && time go test -race ./... -count=1` | ✅（options_test/perclient_test 扩展） | ✅ green |
| 13-07-T1 | 13-07 | 5 | PC-08/PC-09/SEC-09/OPS-12 | T-13-22/T-13-23/T-13-24 | 协议层 UAT 六场景：节流 1011+XFF / KILL 兜底 5s trap '' HUP / 退出三形态 255 / Shutdown N 组 / metrics 四计数器 / env 可见 + 输出自净 | uat（协议层） | `node web/uat/phase13.mjs && node web/uat/phase13.mjs` | ✅（phase13.mjs 769 行，13-07-T1 自举落地） | ✅ green |
| 13-07-T2 | 13-07 | 5 | PC-08 | T-13-22 | churn 10rps×30s 合法票据，RSS/goroutine/fd 双采样差值有界 + throttled_total>0 生效证据 | load（build tag） | `time go test -tags=load -run TestChurn -count=1 ./internal/server/ -v && go vet ./internal/server/` | ✅（load_test.go 扩展） | ✅ green |
| 13-08-T1 | 13-08 | 6 | PC-08/PC-09/SEC-09/OPS-12 | T-13-25/T-13-26/T-13-SC | 收口闸六段式：静态面 + 全量 -race + darwin 双编译 + dist byte-identical + UAT 矩阵 + diff 白名单（shared 逐字未动/零新依赖） | 收口闸（race 全量 + 双编译 + uat） | `go vet ./... && time go test -race ./... -count=1 && GOOS=darwin GOARCH=amd64 go build ./... && GOOS=darwin GOARCH=arm64 go build ./... && node web/uat/phase13.mjs` | ✅（全量既有 + phase13.mjs 已由 13-07 落地） | ✅ green |
| 13-08-T2 | 13-08 | 6 | PC-08/PC-09/SEC-09/OPS-12 | — | 需求勾选 + Key Decisions 登记 + D-10 文档段 + SUMMARY 收口 | 文档断言 | `grep -c '\[x\] \*\*PC-08\*\*\\|\[x\] \*\*PC-09\*\*\\|\[x\] \*\*SEC-09\*\*\\|\[x\] \*\*OPS-12\*\*' .planning/REQUIREMENTS.md && grep -n 'stop-timeout' .planning/PROJECT.md && grep -n '1006' README.md` | ✅（既有文档 grep） | ✅ green |

*File Exists：✅ = 命令作用对象为既有文件（扩展/翻转/全量重跑）；13-07-T1 原 ❌ W0（phase13.mjs 随任务自举新建）已落地兑现（769 行，13-07 执行期两轮 29/29 + 本次审计复跑 29/29）。*

*Status: 13-01-T1 `✅ gate-passed` = manual-gate 人工裁决门通过（option-a 用户派发确认，决策登记 STATE.md——Nyquist 约定豁免 automated verify）；✅ green = 自动化验证在场且跑绿。*

---

## Wave 0 Requirements

- [x] 无新框架安装 — 既有 go test + web/uat 协议层脚本先例覆盖
- [x] churn 负载测试脚本骨架（合法票据 10rps × 30s，断言 RSS/goroutine/fd 有界）——load_test.go TestChurnPerClientSpawnThrottle 已落地（13-07-T2，-tags=load）

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 真实 OS 网卡栈断网时序下 churn 行为 | PC-08 | 平台原生行为显式豁免（CODEBUDDY.md 测试策略第 5 条） | UAT 中以 `skipped` + reason 记录，风险接受；用 TCP 转发器 kill/restore 模拟替代 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

*注：13-01-T1 为 checkpoint:decision 人工裁决门（manual-gate），按 Nyquist 约定豁免 automated verify——option-a 用户派发确认已通过、决策登记 STATE.md；其余 16 个实现任务全部携带 `<automated>` 命令且验证在场。MISSING 引用零命中，Wave 0 勾项成立（churn 骨架已落地兑现）。*

**Approval:** validated 2026-09-06（validate-phase §6 审计通过——零缺口，见下方 Audit Trail）

---

## Validation Audit 2026-09-06

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

**审计口径**（validate-phase State A 审计，非转述 SUMMARY 自报）：

1. **需求→任务→测试交叉比对**：17 任务（16 automated + 1 manual-gate）逐项核对——SUMMARY Verification Results（执行期）、13-VERIFICATION.md Behavioral Spot-Checks 9 项（verifier 独立复跑，2026-09-05T20:25:33Z）、13-08-SUMMARY 六段式收口闸证据（全量 -race 97s + darwin 双编译 + UAT 矩阵 17 轮 + churn + diff 白名单 23 文件逐文件吻合）三重证据链全绿。
2. **本次审计会话独立复跑**（git 干净、HEAD 2ea4b08，VERIFICATION 后仅 docs 提交、代码零变动）：
   - 静态面：`go build ./... && go vet ./...` 零输出
   - 13-01 定向组（cmd/wesh）：`TestStopTimeout|TestValidateStartupWarn` ok 0.004s
   - 13-06-T1（internal/pty -race）：`TestEnvWhitelist` ok 1.014s
   - 13-02~13-06 服务端定向组（-race，13 测名并集）：ok 8.818s
   - 13-07-T1（UAT）：`node web/uat/phase13.mjs` **29/29 PASS**（23.1s，SEC 自净 28 details 零命中）
   - 13-07-T2（churn 负载格）：`-tags=load TestChurn` ok 30.162s
3. **结论**：PC-08/PC-09/SEC-09/OPS-12 四需求全部 COVERED（REQUIREMENTS.md 勾选 + Traceability Complete + 行为证据双重复跑）；零 MISSING、零 PARTIAL——`nyquist_compliant: true` 置位，无需派发 gsd-nyquist-auditor 生成补充测试。
