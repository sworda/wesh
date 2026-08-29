---
phase: 09
slug: release-polish
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-29
---

# Phase 09 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test（Go 1.26.3，含 fuzz `-fuzz`）+ Node 原生 WebSocket/fetch UAT 脚本（web/uat/phaseNN.mjs） |
| **Config file** | none — 现有 go.mod 与 web/uat/ 先例（phase02/03/04.mjs） |
| **Quick run command** | `go test ./...` |
| **Full suite command** | `go test ./... && go vet ./...` + 相关 `web/uat/phaseNN.mjs` 脚本 |
| **Estimated runtime** | ~60 秒（不含负载/模糊长测） |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run full suite + 该波次相关 UAT 脚本
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 秒

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-T1 | 09-01 | 1 | OPS-10 | T-09-01a/T-09-SC | 供应链钉版 + 静态二进制 | 本机预演 | `goreleaser check && goreleaser release --snapshot --clean` + 产物断言组 | ❌ Wave 0（.goreleaser.yml） | ⬜ pending |
| 09-01-T2 | 09-01 | 1 | OPS-10 | T-09-01b/c | 最小授权 + 显式编排 | 静态审查 | YAML 解析 + grep 断言组（release.yml） | ❌ Wave 0（release.yml） | ⬜ pending |
| 09-02-T1 | 09-02 | 1 | OPS-10 | T-09-02b/c | 值剥离红线 + 接缝行为保持 | unit + fuzz | `go test -count=1 ./cmd/wesh/ && go test -fuzz=FuzzDecodeFileConfig -fuzztime=60s ./cmd/wesh/` | ❌ Wave 0（fuzz_test.go + 接缝） | ⬜ pending |
| 09-02-T2 | 09-02 | 1 | OPS-10 | T-09-02a/d | 不 panic + ClampDim 不变量 | unit + fuzz | `go test -fuzz=FuzzDecodeHello -fuzztime=60s ./internal/proto/ && go test -fuzz=FuzzDecodeResize -fuzztime=60s ./internal/proto/` | ❌ Wave 0（fuzz_test.go + ci.yml job） | ⬜ pending |
| 09-03-T1 | 09-03 | 2 | OPS-10 | T-09-03a | 文案单写口 | typecheck | `pnpm -C web exec tsc --noEmit` + grep 断言组 | 既有文件修改 | ⬜ pending |
| 09-03-T2 | 09-03 | 2 | OPS-10 | T-09-03b/c | 远端语义精确落版 | jsdom | `time pnpm -C web build && node web/uat/phase06-dom.mjs && node web/uat/phase04-dom.mjs && node web/uat/phase05-dom.mjs` | 既有文件扩展 | ⬜ pending |
| 09-04-T1 | 09-04 | 2 | OPS-03 | —（确认门） | one-way 契约确认 | checkpoint | 用户 as-locked 裁决 | — | ⬜ pending |
| 09-04-T2 | 09-04 | 2 | OPS-03 | T-09-04a/b | 错误行零内容 + LimitReader | unit | `go test -race -count=1 ./cmd/wesh/ -run 'TestParseArgs\|TestStartupMatrix\|TestLoadFileConfig\|TestConfigMerge\|TestLoadCustomIndex'` | 既有文件扩展 | ⬜ pending |
| 09-04-T3 | 09-04 | 2 | OPS-03 | T-09-04c/d/e | byte-identity + 认证面不变 | Go e2e | `go test ./internal/server/ -race -count=1 -run 'TestCustomIndex' -v && go test -race -count=1 ./...` | ❌ Wave 0（customindex_test.go） | ⬜ pending |
| 09-05-T1 | 09-05 | 3 | OPS-03 | T-09-05a | UAT 红线三件套 | 协议 UAT | `go build -o /tmp/wesh-uat/wesh ./cmd/wesh && node web/uat/phase09.mjs /tmp/wesh-uat/wesh` | ❌ Wave 0（phase09.mjs） | ⬜ pending |
| 09-05-T2 | 09-05 | 3 | OPS-03 | T-09-05b | 零影响回归 | 协议/jsdom UAT | `node web/uat/phase03.mjs && node web/uat/phase05.mjs && node web/uat/phase06-dom.mjs && node web/uat/phase07.mjs` | 既有脚本 | ⬜ pending |
| 09-06-T1 | 09-06 | 1 | OPS-10 | T-09-06a/b | build tag 隔离 + 收口纪律 | load | `go test -tags=load -count=1 -timeout=20m -run 'TestLoad' ./internal/server/ -v` | ❌ Wave 0（load_test.go） | ⬜ pending |
| 09-06-T2 | 09-06 | 1 | OPS-10 | T-09-06c | 精确计数非弱化 | load | `go test -tags=load -count=1 -timeout=30m ./internal/server/ -v` | ❌ Wave 0（同上） | ⬜ pending |
| 09-07-T1 | 09-07 | 1 | OPS-10 | T-09-07a/d | tini sha256 钉死 | 本机实测 | `docker build -t wesh-test .` + PID 1/收割/负对照/bind-mount 断言组 | ❌ Wave 0（Dockerfile/.dockerignore） | ⬜ pending |
| 09-07-T2 | 09-07 | 1 | OPS-10 | T-09-07b/c | unit 零秘密 + Restart 语义 | 实机 | `systemd-analyze verify deploy/wesh.service` + systemctl 五证据 | ❌ Wave 0（wesh.service） | ⬜ pending |
| 09-08-T1 | 09-08 | 1 | OPS-10 | T-09-08a/c | Host 语义实证 + 凭据模式 | 协议层实测 | Caddy 四断言（401→200/echo/idle 65s/清理） | 实证环境 | ⬜ pending |
| 09-08-T2 | 09-08 | 1 | OPS-10 | T-09-08b | 零真实凭据入库 | 静态自检 | `bash -n web/uat/pw/phase09-caddy-ctl.sh && node --check web/uat/pw/phase09-caddy-pw.mjs` | ❌ Wave 0（两载具） | ⬜ pending |
| 09-08-T3 | 09-08 | 1 | OPS-10 | —（确认门） | 双机全链用户裁决 | checkpoint | Windows 侧 `node phase09-caddy-pw.mjs` | — | ⬜ pending |
| 09-09-T1 | 09-09 | 4 | OPS-10 | T-09-09a/c | 发布前四闸 | 干跑验证 | `bash -n scripts/release.sh` + 干跑四态（脏树/坏形态/已存在/好树） | ❌ Wave 0（release.sh） | ⬜ pending |
| 09-09-T2 | 09-09 | 4 | OPS-03/OPS-10 | T-09-09b/d | 文档实证分级 | 文档断言 | README grep 断言组 + `go test -race -count=1 ./...`（证伪分支） | 既有文件修改 | ⬜ pending |
| 09-10-T1 | 09-10 | 5 | OPS-03/OPS-10 | T-09-10a | 全绿证据链 | 全量验证 | 六段式 + 全量 UAT + fuzz 2×60s + load 矩阵 + snapshot 复演 | 全部既有 | ⬜ pending |
| 09-10-T2 | 09-10 | 5 | OPS-10 | —（发布闸） | one-way 公开动作裁决 | checkpoint | 用户 publish-now/publish-later 裁决 | — | ⬜ pending |
| 09-10-T3 | 09-10 | 5 | OPS-10 | T-09-10b | git 既有认证通道 | 发布核验 | `gh release view v1.0.0` + 产物清单/校验和断言（publish-now 分支） | 远端 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*（planner 产出 PLAN.md 后回填每任务行——2026-08-29 已回填）*

---

## Wave 0 Requirements

- [ ] `goreleaser` 本机安装（go install 或官方二进制）— 发布链任务前置（09-01 Task 1 步骤①内建安装通道，非用户动作）
- [ ] `loadFileConfig` reader 委托接缝重构 — TOML fuzz 目标前置（09-02 Task 1 步骤①内建，同任务交付）
- [ ] `docker` 与 `systemctl` 可用性确认（RESEARCH Environment Availability 已登记本机 ✓——09-07 实测通道前置）
- [ ] Caddy 官方二进制直装（09-08 Task 1 步骤①内建，/tmp 实证环境不入仓）

*若不需要：删除对应行。*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| darwin 产物 macOS 冒烟 | OPS-03 | 本机无 macOS；RESEARCH 已登记取舍 | scp 到 Mac 运行 `wesh --version` |
| Caddy/Cloudflare 空闲超时行为面 | OPS-10 | 官方文档被网络策略阻断，D-15 既定实证兜底 | 按部署文档配方在真实反代后观察空闲连接 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
