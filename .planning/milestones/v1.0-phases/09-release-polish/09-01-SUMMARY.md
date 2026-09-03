---
phase: 09-release-polish
plan: 01
subsystem: infra
tags: [goreleaser, github-actions, release, cross-compile, pnpm, static-binary]

requires:
  - phase: 01-foundation
    provides: "cmd/wesh/main.go:32 var version = \"dev\" ldflags 注入点；web/dist embed 链（P1 D-18 构建顺序）"
  - phase: 03-security
    provides: ".gitignore web/dist/*.gz 不入库既定（release 二进制走 gzip 旁路）"
provides:
  - ".goreleaser.yml：四平台（linux/darwin × amd64/arm64）全静态交叉编译/打包/校验和配置（version: 2 schema）"
  - ".github/workflows/release.yml：推 v* tag → pnpm build → goreleaser 显式编排的 GitHub Release 链（D-01/D-03）"
  - "本机 snapshot 预演全绿证据：命名族/三件套内容/裸 checksums.txt/静态性/干净容器运行五层断言"
affects: [09-09（发布脚本最后一步真实 tag 首证）, ship, README 发布节（09-09 Task 2）]

actuals:
  tokens: 733
  tasks: 2
  commits: 3

tech-stack:
  added: [goreleaser v2.18.0（本机预演用，/tmp 安装不进仓）, goreleaser-action v7.2.3（CI 引用）]
  patterns:
    - "发布链显式编排：workflow 步骤承载构建顺序（pnpm → go），拒绝 goreleaser before hooks 环境隐式"
    - "验收 grep 机械纪律：注释提及目标字面同样计数（05-08 先例第四次沿用）"

key-files:
  created:
    - .goreleaser.yml
    - .github/workflows/release.yml
  modified: []

key-decisions:
  - "goreleaser v2.18.0 构建目录实为 dist/wesh_<os>_<arch>_<variant>/（plan verify 块按 dist/<os>_<arch>/ 写——按实证路径执行，断言面零变化）"
  - "YAML 静态审查经 ephemeral docker python:3.12-alpine + pyyaml（宿主无 yaml 模块/ruby/yq；容器即用即弃零宿主零仓库污染），并由纯语法检查强化为全结构断言（tags/permissions/七步骤序/钉版）"

patterns-established:
  - "发布配置头注释登记决策与 Pitfall 钉点时避免引用模板字面量（{{ .Tag }}），防验收 grep 计数污染"

requirements-completed: [OPS-10]

coverage:
  - id: D1
    description: ".goreleaser.yml 四平台全静态二进制发布配置——snapshot 预演产出 4 tar.gz（wesh+LICENSE+README.md 三件套）+ 裸 checksums.txt，linux/amd64 静态性 + 干净容器运行实证"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "goreleaser check && goreleaser release --snapshot --clean（tracer gate 二次全链复跑全绿）"
        status: pass
      - kind: other
        ref: "命名族 grep ==4 / sha256sum -c 全 OK / tar 三件套 ×4 / file+ldd 静态断言 / file Mach-O ×2"
        status: pass
      - kind: other
        ref: "docker run --rm -v dist/wesh_linux_amd64_v1/wesh debian:stable-slim /wesh --version → exit 0 且非 dev"
        status: pass
    human_judgment: false
  - id: D2
    description: ".github/workflows/release.yml tag 触发发布 workflow（D-01/D-03 显式编排 + 全 Action 钉版 + contents: write 最小授权）——静态形态锁定"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "docker python:3.12-alpine pyyaml 结构断言（7 步骤序/tags[v*]/permissions 单键/钉版）+ plan grep 断言组"
        status: pass
    human_judgment: true
    rationale: "release.yml 真实全链（推 v* tag → GitHub Release 四平台产物）首证 = v1.0.0 实际发布，属 09-09 发布脚本最后一步（plan flagged_assumptions ③既定验证取舍）；本 plan 仅交付静态锁定形态，verifier 不得据此 auto-pass 端到端发布链"

duration: 19min
completed: 2026-08-29
status: complete
---

# Phase 9 Plan 01: 发布链落地（goreleaser + release.yml） Summary

**goreleaser v2.18.0 四平台全静态发布链：snapshot 预演产出 wesh_v*_(linux|darwin)_(amd64|arm64).tar.gz 四件 + 裸 checksums.txt，linux/amd64 干净容器（debian:stable-slim）--version 注入生效；release.yml 推 v* tag 即发布的 pnpm→go 显式编排经全结构静态断言锁定**

## Performance

- **Duration:** 19 min
- **Started:** 2026-08-29T13:23:28Z
- **Completed:** 2026-08-29T13:43:05Z
- **Tasks:** 2
- **Files modified:** 2（均新建）

## Accomplishments

- `.goreleaser.yml` 定稿入库（version: 2 schema）：builds.main ./cmd/wesh、CGO_ENABLED=0 全仓唯一出现点、-trimpath、`-s -w -X main.version={{.Version}}` 挂 main.go:32 既有注入点、mod_timestamp 可复现构建；D-04 命名族用 `.Tag` 保 v 前缀（Pitfall 2 实证：snapshot 伪 tag v0.0.0 → 产物 `wesh_v0.0.0_*`）；D-02 裸 checksums.txt 显式钉死（Pitfall 1）；changelog filters 排除 docs/test/chore/ci/style
- 本机 snapshot 端到端预演两轮全绿（第二轮为 tracer feedback gate 全链复跑）：4 tar.gz 命名族正则全命中、sha256sum -c 全 OK、每 tar 恰含 wesh/LICENSE/README.md、linux_amd64 为 statically linked ELF（ldd 非动态）、darwin 两产物 Mach-O x86_64/arm64、干净容器运行 exit 0 输出 `wesh 0.0.0-SNAPSHOT-c3495e0`（ldflags 注入生效、非 dev）
- `.github/workflows/release.yml` 定稿入库：on.push.tags ["v*"]、permissions 仅 contents: write、七步骤显式编排（checkout fetch-depth: 0 → pnpm/action-setup@v6.0.10 钉 11.21.0 → setup-node@v4 node 24 → pnpm install --frozen-lockfile → pnpm build → setup-go@v7.0.0 go-version-file → goreleaser-action@v7.2.3 release --clean），零 CGO_ENABLED（单侧持有纪律），全 Action 钉版
- 零新依赖证据：`git diff --exit-code go.mod go.sum web/pnpm-lock.yaml` 通过；预演后 dist/ 已清理，仓库除两新文件外零残留

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): .goreleaser.yml 定稿 + goreleaser 安装 + snapshot 预演 + 四平台分层断言** - `a711019` (feat)
2. **Task 2: release.yml 定稿 + 静态审查断言** - `04dd88b` (feat)

**Plan metadata:** 见末尾 docs 提交（docs(09-01): complete ...）

_Tracer feedback gate：Task 1 提交后全链 verify 复跑全绿（第二轮 snapshot 1.7s，Go 构建缓存命中）方进 Task 2。_

## Files Created/Modified

- `.goreleaser.yml` - 四平台交叉编译/打包/校验和/changelog 配置（头注释登记 D-01..D-04 与 Pitfall 1/2/3 钉点）
- `.github/workflows/release.yml` - tag 触发发布 workflow（D-03 显式编排注释 + 钉版七步骤）

## Decisions Made

- **goreleaser 构建目录布局以实证为准**：v2.18.0 实际产出 `dist/wesh_<os>_<arch>_<variant>/`（amd64→v1、arm64→v8.0 变体后缀），plan verify 块按 `dist/<os>_<arch>/` 书写——执行与 tracer gate 复跑均按实证路径，断言语义（file/ldd/docker）逐字不变
- **YAML 静态审查通道**：宿主 python3 无 yaml 模块、无 ruby/yq，采 ephemeral docker python:3.12-alpine + 容器内 pip pyyaml（容器 --rm 即弃，宿主与仓库零污染）；并将 plan 的纯语法检查强化为全结构断言（tags==["v*"]、permissions 单键、七步骤 uses/run 逐项 + with 值、env GITHUB_TOKEN 引用形态）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] plan verify 块 goreleaser 产物目录路径与 v2.18.0 实证不符**
- **Found during:** Task 1（静态性断言阶段）
- **Issue:** plan verify 按 `dist/linux_amd64/wesh` 书写；v2.18.0 实际布局为 `dist/wesh_linux_amd64_v1/wesh`（含 goamd64/v8.0 变体后缀），逐字执行必失败
- **Fix:** 按实证路径执行全部分层断言与 tracer gate 复跑；断言面（ELF 静态/ldd/Mach-O/干净容器/tar 内容/checksum/命名族）零变化
- **Files modified:** 无（执行期路径修正；.goreleaser.yml 内容不受影响）
- **Verification:** 两轮 snapshot 全部断言通过
- **Committed in:** `a711019`（Task 1 提交）

**2. [Rule 1 - Bug] .goreleaser.yml 头注释字面引用 `{{ .Tag }}` 致验收 grep 计数为 2（要求 ==1）**
- **Found during:** Task 1（验收 grep 阶段）
- **Issue:** 文件头 Pitfall 2 登记注释含模板字面量 `{{ .Tag }}`，与 name_template 行合并计数为 2——验收 grep 是源码级机械检查，注释提及同样计数（05-08 既定纪律）
- **Fix:** 头注释改写为「.Tag 模板变量」（去双花括号），登记语义不变
- **Files modified:** .goreleaser.yml
- **Verification:** 四条验收 grep 全部 ==1；goreleaser check 复验通过
- **Committed in:** `a711019`（Task 1 提交）

**3. [Rule 3 - Blocking] 宿主无 YAML 解析器（python3 缺 yaml 模块、无 ruby/yq），plan 语法检查无法按字面执行**
- **Found during:** Task 2（静态审查阶段）
- **Issue:** plan verify 要求 `python3 -c "import yaml..."`；宿主 python3.12 无 yaml；ruby/yq 均不存在；node_modules 无 yaml/js-yaml；禁 pip/npm 装包入宿主（Rule 3 排除纪律）
- **Fix:** ephemeral docker python:3.12-alpine 容器内 pip pyyaml 后运行检查脚本（脚本文件挂载防多层 shell 转义吃掉 `${{ }}`）；容器 --rm 即弃零残留。同通道将检查强化为全结构断言（见 Decisions）
- **Files modified:** 无（/tmp 检查脚本随容器生命周期，不进仓）
- **Verification:** 结构断言全过 + plan grep 断言组全过 + D-03 行号序断言（pnpm build :30 < goreleaser uses :34）
- **Committed in:** `04dd88b`（Task 2 提交）

---

**Total deviations:** 3 auto-fixed（2 Rule 1 计划文档/注释字面修正，1 Rule 3 验证环境适配）
**Impact on plan:** 全部为执行通道修正，交付物形态与 plan must_haves 逐字一致；无范围蔓延。

## Issues Encountered

- `${{ secrets.GITHUB_TOKEN }}` 在 fish → docker sh → python 三层引用下被中间层展开（bad substitution）——改脚本文件挂载后一次性通过
- docker 拉取 debian:stable-slim / python:3.12-alpine 两镜像为本 plan 验证所需的一次性环境成本（均已缓存）

## Known Stubs

None —— 本 plan 交付物为两份 YAML 配置，无占位/硬编码空值/TODO；全部行为面经 snapshot 实证或静态断言。

## User Setup Required

None - no external service configuration required.（真实 v1.0.0 发布由 09-09 发布脚本承载，届时 tag push 触发 release.yml 首证全链。）

## Next Phase Readiness

- 09-02 起的后续 plan 可直接引用 `.goreleaser.yml`/`release.yml` 定稿形态；09-09 发布脚本最后一步（git tag v1.0.0 && push）即 release.yml 全链首证
- 人工复核项（plan flagged_assumptions 登记，不阻塞）：① darwin 产物 macOS 运行冒烟（09-VALIDATION Manual-Only）；② D-04 精确命名 `wesh_v1.0.0_linux_amd64.tar.gz` 逐字首证 = 真实 tag 发布；③ release.yml 全链首证 = v1.0.0
- 本机预演通道留存：/tmp/grl/goreleaser（v2.18.0，重启即失，09-09 前如需复跑按 Task 1 ① 重装即可）

## Self-Check: PASSED

- Files: .goreleaser.yml / .github/workflows/release.yml / 09-01-SUMMARY.md 均 FOUND
- Commits: a711019（Task 1）/ 04dd88b（Task 2）均 FOUND

---
*Phase: 09-release-polish*
*Completed: 2026-08-29*
