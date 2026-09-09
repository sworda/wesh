---
phase: 09-release-polish
fixed_at: 2026-08-30T15:07:15Z
review_path: .planning/phases/09-release-polish/09-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 9: Code Review Fix Report

**Fixed at:** 2026-08-30T15:07:15Z
**Source review:** .planning/phases/09-release-polish/09-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5（0 critical / 5 warning；fix_scope=critical_warning）
- Fixed: 5
- Skipped: 0（Info 级 4 项超出 fix_scope，见下方「Out of Scope」）

## Fixed Issues

### WR-01: 发布前置闸④「与远端同步」实际不校验远端真实状态

**Files modified:** `scripts/release.sh`
**Commit:** d11de4b
**Applied fix:** 闸④的 `git fetch --dry-run` 改为真实 `git fetch`（更新远端跟踪引用后再做 `HEAD..@{u}` 落后计数）；fetch 失败降级为跳过提示的语义逐字保持不变。补注释说明 `--dry-run` 不更新跟踪引用导致闸门假绿的机理。
**状态:** fixed: requires human verification（闸门行为变更，语法 `bash -n` 验证通过；建议下次发布以 `--dry-run` 干跑复核闸④行为）

### WR-02: 脏树闸③先于 `pnpm build` 执行，web/dist 与源码的漂移零检测

**Files modified:** `scripts/release.sh`
**Commit:** fb4ace9
**Applied fix:** `build_web` 在 `pnpm -C web build` 后补 dist 漂移闸——`git status --porcelain web/dist` 非空即 `die "web/dist differs from committed artifact; rebuild output must be committed before release"`（构建重写被跟踪 index.html = 已提交 dist 与源码不一致，须先提交新产物）；dry-run 步骤清单第 2 行同步标注 dist drift gate。漂移检测表达式已本地模拟验证（脏 dist 检出 → 恢复后放行）。
**状态:** fixed（闸表达式本地行为模拟通过 + `bash -n`；实际发布流跑一次即完整验证）

### WR-03: README 承诺「提交了 dist 的 .gz」与 .gitignore 事实相悖

**Files modified:** `README.md`
**Commit:** 323a86d
**Applied fix:** 选方案 (b) 修正文档与仓库事实一致：改为「仓库提交 `web/dist/index.html` 占位；`.gz` 预压产物由 `pnpm -C web build` 生成、不入库（`.gitignore` 忽略 `web/dist/*.gz`），发布构建在 CI 侧完成（发布二进制含 `.gz`）」。未选方案 (a)（移除 .gitignore 规则并提交 .gz 产物）的理由：embed.go 注释与 .gitignore 一贯政策是只提交 index.html 占位，提交 .gz 构建产物属仓库政策变更（且 dist 为构建产物），应由用户裁决而非 fixer 单方面改变。
**状态:** fixed（纯文档修正，与 `.gitignore:2` / `git ls-files web/dist/` / embed.go 注释三方交叉核对一致）

### WR-04: Caddy UAT rig 硬编码内网 IP、不支持 SSH 端口、两侧凭据来源不一致

**Files modified:** `web/uat/pw/phase09-caddy-pw.mjs`, `web/uat/pw/phase09-caddy-ctl.sh`
**Commit:** 4c25f67
**Applied fix:**
1. pw 侧改读环境变量，对齐 `lib/server.mjs` 同源形态：`WESH_UAT_SSH`（必填，缺省即抛错并指向 README）/ `WESH_UAT_SSH_PORT`（默认 22，ssh 加 `-p`、scp 加 `-P`）/ `WESH_UAT_TARGET_HOST`（缺省从 SSH 目标推导 host 部分），`BASE` 由 `TARGET_HOST` 推导——消除硬编码内网 IP；
2. 凭据单一事实源：pw setup 将 `CRED`（`WESH_UAT_CRED` 覆盖机制生效）经 ssh stdin 递交 ctl；ctl setup 在 stdin 非 TTY 时 `head -n1` 读首行、空读回落一次性默认 `user:pass`（ctl 手跑兼容）；
3. ctl 侧改经 `WESH_CREDENTIAL` env 递交凭据启动 wesh（替代 `--credential user:pass` argv 形态，不进 ps 可见面——顺带消除 IN-03 所指形态，虽 IN-03 本身超出本次 scope）。
**状态:** fixed: requires human verification（双机 rig 无法本地 E2E 实测；已做 `node --check`/`bash -n` 语法验证 + stdin 递交与空读回落机制本地模拟通过；建议下次实证按 README 环境变量形态双机复跑）

### WR-05: loadCustomIndex 的 `int64(max)+1` 在 index-max-size 极值处溢出——静默伺服空白页

**Files modified:** `cmd/wesh/main.go`, `cmd/wesh/main_test.go`
**Commit:** 24c86df
**Applied fix:** `validateStartup` 在 `indexMaxSize <= 0` 拒绝同位补上界钳制：`indexMaxSize > 1<<31-1`（2GiB 硬顶）即 exit 2 fail-fast，错误文案 `invalid index-max-size: exceeds 2GiB cap`；MaxInt64「实际无限大」笔误不再进入 loadCustomIndex 的 `int64(max)+1` 回绕路径。TestStartupMatrix 补 `math.MaxInt64` 拒绝行（wantErrSub2 锁定文案），防止回归。
**状态:** fixed（`go build ./...` + `go vet ./cmd/wesh/` + `go test -race -count=1 ./cmd/wesh/` 全绿，含新拒绝行）

## Skipped Issues

None — 5 项 in-scope finding 全部修复。

## Out of Scope（Info 级 4 项，fix_scope=critical_warning 不含）

以下 Info 级发现本次未尝试修复，仅登记（原因：info 级超出 fix_scope=critical_warning）：

### IN-01: WithCustomIndex 对任意 HTTP 方法都回页

**File:** `web/embed.go:103-122`
**Reason:** info 级超出 fix_scope。属未定义行为面（无安全影响，纯读页面），补方法闸会改变 `--index` 开启时的 POST / 行为面（当前 200 → 405），宜由用户裁决是否对齐 FileServerFS 语义并补测试。

### IN-02: systemd unit 模板的 ExecStart 依赖 /etc/wesh/wesh.toml 存在；TimeoutStopSec 与 --stop-timeout 隐性耦合

**File:** `deploy/wesh.service:13,21`
**Reason:** info 级超出 fix_scope。仅为 unit 注释补两句提示的文档性改进，无行为变更需求，可随下个 phase 顺手处理。

### IN-03: Caddy UAT rig 以命令行 `--credential user:pass` 启动 wesh——与项目自身「凭据勿走 ps 可见面」指引相悖

**File:** `web/uat/pw/phase09-caddy-ctl.sh:58`
**Reason:** info 级超出 fix_scope。**注：WR-04 修复已顺带消除该形态**——ctl 侧已改为 `WESH_CREDENTIAL` env 前缀递交（commit 4c25f67），凭据不再进 argv。

### IN-04: pw 脚本 ssh 通道用 JSON.stringify 当 shell 引用——未来命令含 $/反引号时会被远端 shell 展开

**File:** `web/uat/pw/phase09-caddy-pw.mjs:29`
**Reason:** info 级超出 fix_scope。当前 cmd 均为静态安全串、无实际注入；属潜在模式缺陷，改单引号转义或 `bash -s` + stdin 属 rig 加固项，宜与 WR-04 修复一并双机复跑时验证后再动。

## Verification

**验证执行位置：** 全部验证在隔离 worktree（repo 内 `.codebuddy/worktrees/rf-09-1456271-1788101158`，分支 `gsd-reviewfix/09-1456271`）中执行；worktree 于报告完成后移除，验证数字不可从主 checkout 复现——已验证状态由下列提交承载（fast-forward 回 `gsd/phase-09-release-polish`）：d11de4b / fb4ace9 / 323a86d / 4c25f67 / 24c86df。

- `bash -n scripts/release.sh`（WR-01/WR-02 修改后）— 通过
- WR-02 漂移闸表达式本地模拟（脏 `web/dist/index.html` 检出 → `git checkout --` 恢复后放行）— 通过
- `bash -n web/uat/pw/phase09-caddy-ctl.sh` + `node --check web/uat/pw/phase09-caddy-pw.mjs`（WR-04）— 通过
- WR-04 凭据 stdin 递交机制本地模拟（管道读首行 / 空读回落默认）— 通过
- `go build ./...`（WR-05）— 通过（0.77s，无错误）
- `go vet ./cmd/wesh/`（WR-05）— 通过
- `go test -race -count=1 ./cmd/wesh/`（WR-05，含新增 MaxInt64 拒绝行的 TestStartupMatrix 与全包既有测试）— ok, 1.297s
- 逐项 Tier 1 重读：修改区段与上下文完整，无损坏

**未执行（超出 per-fix 验证范围）：** 全量测试套件 / 长 fuzz / 负载矩阵 / 双机 Playwright 实测——属 verifier phase 与下次双机实证的职责面。

---

_Fixed: 2026-08-30T15:07:15Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
