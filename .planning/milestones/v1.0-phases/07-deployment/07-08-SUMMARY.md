---
phase: 07-deployment
plan: 08
subsystem: docs
tags: [readme, deployment-docs, nginx-recipe, sec-07-revision, d-15, uat-manual-checklist, full-verification, six-stage, uat-regression, phase-closure]

requires:
  - phase: 07-deployment/07-01
    provides: --base-path 全链行为（307/StripPrefix/前端相对 URL）——README 反代小节素材
  - phase: 07-deployment/07-02
    provides: --socket 三 flag 与 listenSocket 序列、unix:// 打印退化——README socket 小节与 systemd 配方素材
  - phase: 07-deployment/07-03
    provides: --auth-header 审计归因全链（remote_user 第四字段、XFF 信任闸、D-16 警告）——README auth-header 小节与 SEC-07 修订语义来源
  - phase: 07-deployment/07-04
    provides: 子进程管理四 flag 与降权身份环境改写、supplementary groups 环境感知策略（07-08 复核联动项）——README 两小节素材
  - phase: 07-deployment/07-05
    provides: 1001 优雅下线序列与 --open 三形态——README 两小节素材；退出码 255 注记（RESEARCH OQ2 去向）兑现
  - phase: 07-deployment/07-06
    provides: TOML 配置 27 键两阶段合并与优先级链、D-07 权限警告——README 配置小节素材
  - phase: 07-deployment/07-07
    provides: phase07.mjs 八场景行为矩阵（README 事件行样例来源）与 flagged_assumptions 衔接清单
provides:
  - "README.md「部署与配置（Phase 7）」节（八小节：配置文件/UNIX socket/反代子路径/auth-header 与 XFF/子进程管理/降权运行/--open/优雅下线）+ 用法 flag 表补 Phase 7 全部 13 个新 flag + 关闭码表 1001 行翻正启用态"
  - ".planning/REQUIREMENTS.md SEC-07 文本 D-15 修订（服务端审计归因语义，原「作为子进程环境变量」作废）+ D-15 注记"
  - ".planning/ROADMAP.md Phase 7 成功准则 2 同步修订（同语义单一口径）"
  - ".planning/phases/07-deployment/07-UAT.md【新】人工 UAT 清单（manual-only 两项 + flagged assumptions 六复核项 + root 降权 nobody 可选场景 + A1 复核留档，9 勾选项）"
  - "RESEARCH A1 复核闭合证据：本机 nginx 1.14.1 实测 location /wesh/ 不匹配裸 /wesh（裸路径 404）——README 精确块确凿落文（非防御性建议）"
  - "全量六段式验证与十脚本回归全绿记录（本 SUMMARY）"
affects: [verify-work, phase-08, phase-09（部署文档面定型——nginx/systemd 配方为用户门面承诺）]

actuals:
  tokens: 5579
  tasks: 2
  commits: 1

tech-stack:
  added: []
  patterns:
    - "文档与实现一致性纪律：README 全部 flag 名/默认值以 cmd/wesh/main.go 与 --help 输出为单一事实源逐条比对（交付红线——文档与实现漂移是部署文档最大风险）"
    - "RESEARCH 待复核项执行期实证闭合形态：A1（nginx location 前缀匹配语义）以本机 nginx 1.14.1 最小配置实测（裸 /wesh → 404 vs /wesh/ → 200），复核结论回写 07-UAT.md 留档，README 配方按实证确凿落文"

key-files:
  created:
    - .planning/phases/07-deployment/07-UAT.md
  modified:
    - README.md
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md

key-decisions:
  - "README nginx 配方的 location = /wesh 精确块按 A1 实证确凿落文（本机 nginx 1.14.1 实测裸 /wesh 404）——plan 允许的「若复核未做则标注防御性建议」分支未触发"
  - "REQUIREMENTS.md SEC-07 勾选状态保持 [x] 不动（本任务只改文本——勾选状态属验证流程职责；现状已为 [x]，plan『保持 [ ] 不变』按『不变更勾选状态』意图执行）"
  - "README 关闭码表 1001 行随部署节优雅下线小节同步翻正为启用态（07-05 已启用发送路径——同主题文档漂移一次修正，07-02/07-05 先例）"
  - "README 用法 flag 表补 Phase 7 全部 13 个新 flag（05-09/06-07 收口 plan 先例——每 phase 新 flag 进速查表 + 专节详述的既定结构惯例）"

requirements-completed: []

coverage: []

duration: 24min
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 08: 部署文档收口与全量验证 Summary

**Phase 7 收口：README「部署与配置」八小节落地（含 ttyd -H 模型差异公开承诺——D-15 one-way 交付物）、SEC-07 需求文本与 ROADMAP SC2 双文件同步修订（单一口径）、07-UAT.md 人工清单（9 勾选项）、RESEARCH A1 经本机 nginx 1.14.1 实测闭合、全量六段式验证与十脚本回归全绿——Phase 7 交付物（代码/测试/文档/UAT）齐备可密封。**

## Performance

- **Duration:** 24 min
- **Started:** 2026-08-26T06:42:12Z
- **Completed:** 2026-08-26T07:06:22Z
- **Tasks:** 2/2
- **Files modified:** 4（1 新建 + 3 修改；194 insertions / 3 deletions）

## Accomplishments

- **README「部署与配置（Phase 7）」节（八小节）**：配置文件（TOML 27 键/优先级链 flag>env>config>默认/列表替换/逃生门五键/严格模式/chmod 600 + WESH_CREDENTIAL 优先）、UNIX socket（三 flag + 残留清理 + 文件系统权限即认证边界 + systemd 配方）、反代子路径（值规则/307 规范化/nginx 两块配方/proxy_read_timeout > --ping-interval）、auth-header 与 XFF（审计归因语义/信任闸/裸信任+暴露面警告/仅反代后部署/**ttyd -H 模型差异段落**——D-15 公开承诺）、子进程管理四 flag、降权运行（数字直通成对/身份环境改写/附加组策略）、--open 三形态、优雅下线（1001 序列 + 退出码 255 运维注记 + systemd SuccessExitStatus= 部署侧自决——RESEARCH OQ2 去向兑现）
- **SEC-07 D-15 双文件修订**：REQUIREMENTS.md SEC-07 文本修订为「可信 HTTP 头注入的用户名记录进服务端审计日志（remote_user 审计归因）」+ D-15 注记（原「作为子进程环境变量」语义在 GoTTY 共享进程模型下结构性不成立）；ROADMAP.md Phase 7 成功准则 2 后半句同步修订——两处单一口径防文档间漂移；勾选状态与 Traceability Phase 7 映射保持不动
- **07-UAT.md 人工清单**：VALIDATION manual-only 两项（真实浏览器 --open / 真实 nginx 反代观感）+ flagged assumptions 六复核项逐项列探针原文（OPS-01 并发中断/SEC-07 多值头/OPS-04 symlink·TERM·stop-timeout 极大值/OPS-05 nobody 无 shell 与附加组清空含 root 可选场景步骤/OPS-09 TOML 语法变体与空 command/OPS-11 macOS open 与 TLS 组合）+ A1 复核留档——9 勾选项，56 行
- **RESEARCH A1 复核闭合**：本机 nginx 1.14.1 最小配置实测——`location /wesh/ { return 200; }` 下裸 `/wesh` → **404**、`/wesh/` → **200**；精确重定向块必需成立，README 配方确凿落文（plan「复核未做则标注防御性建议」分支未触发）
- **README 用法 flag 表补 Phase 7 全部 13 个新 flag**；关闭码表 1001 行翻正为启用态（07-05 已启用发送路径）
- **全量六段式验证全绿 + 十脚本回归全绿**（详见下方逐段记录）；README 全部 flag 名/默认值与 `--help` 输出逐条一致（比对记录见下）

## Task Commits

1. **Task 1: README 部署与配置节 + SEC-07/SC2 修订 + 07-UAT.md** - `7ac957c` (docs)
2. **Task 2: 全量六段式验证 + 十脚本回归** - 无提交（仅验证，零文件改动；gofmt 零新增漂移故无 style 提交）

**Plan metadata:** docs 提交在本 SUMMARY 之后（`docs(07-08): complete deployment-closure plan`，hash 见 git log）。

## Files Created/Modified

- `README.md` - 「部署与配置（Phase 7）」节新增（八小节，置于「多客户端共享」之后「测试」之前）；用法 flag 表补 13 行（--config/--socket/--socket-mode/--socket-owner/--base-path/--auth-header/--cwd/--term/--stop-signal/--stop-timeout/--uid/--gid/--open）；关闭码表 1001 行翻正启用态
- `.planning/REQUIREMENTS.md` - SEC-07 文本 D-15 修订 + 注记（L49；勾选 [x] 与 Traceability L141 Phase 7 映射不动）
- `.planning/ROADMAP.md` - Phase 7 成功准则 2 后半句 D-15 同步修订（准则编号与前后半句不动）
- `.planning/phases/07-deployment/07-UAT.md`【新，56 行】- A 节 manual-only 两项 + B 节 flagged assumptions 六复核项 + C 节 A1 复核留档（已勾选）

## 全量六段式验证记录（Task 2）

| 段 | 命令 | 结果 |
|----|------|------|
| 1 | GOROOT gofmt -l（/data1/home/zexueli/softwares/go/bin/gofmt） | ✓ 零新增漂移——输出仅 `internal/server/multi_test.go` 与 `internal/server/slowclient_test.go` 两 HEAD 既有漂移文件（deferred-items.md 2026-08-26 登记，git show HEAD 复验非本 phase 引入；验证口径 = 零新增漂移，与 07-01..07-07 各 plan 同口径） |
| 2 | `go vet ./...` | ✓ 退出 0 |
| 3 | `go test -race -count=1 ./...` | ✓ 五包全绿 52.3s（cmd 1.1s / proto 1.0s / pty 2.6s / server 51.1s / web 1.0s） |
| 4 | `time pnpm -C web build` | ✓ 退出 0（2.3s）；产物时间戳验证（dist/index.html 14:57:45 新于 main.ts 12:40:32）；git status 构建后干净（产物与库内一致） |
| 5 | 裸 clone embed 链（rm -rf 前置 → git clone 本仓 /tmp/wesh-clone-verify → go build） | ✓ build 0.78s 成功——embed 内嵌单 HTML 伺服成立（dist 已入库无需前端构建） |
| 6 | 启动冒烟（裸 clone 产物驱动） | ✓ 8/8 PASS：--port 0 随机端口启动行解析 / GET / 200 / WS Hello→Welcome / SIGTERM 优雅退出 / unix:// 启动行 / socket 文件 0660 / 分享链接退化单行 / unix 实例退出（--socket 形态顺带复测） |

**十脚本 UAT 回归（逐个执行各自退出 0）：**

| 脚本 | 结果 | 脚本 | 结果 |
|------|------|------|------|
| phase02.mjs | ✓ 12/12 | phase06.mjs | ✓ 23/23 +1豁免 |
| phase03.mjs | ✓ 18/18 | phase07.mjs | ✓ 33/33 +1豁免（SEC 自净 PASS details=33 零命中） |
| phase04.mjs | ✓ 10/10 | phase04-dom.mjs | ✓ 37/37 |
| phase05.mjs | ✓ 28/28 +1豁免 | phase05-dom.mjs | ✓ 19/19 |
| phase05-dims.mjs | ✓ DIMS PASS | phase06-dom.mjs | ✓ 37/37 +1豁免 |

## README flag 名/默认值一致性比对（acceptance 要求逐条记录）

README（flag 表 + 部署节）出现的全部 19 个 flag 名与 `go run ./cmd/wesh --help` 输出逐条比对——**全部一致**：

| Flag | README 默认值表述 | --help 输出 | 一致 |
|------|------------------|-------------|------|
| --config | —（仅显式指定） | 无 default 后缀 | ✓ |
| --socket | — | 无 default 后缀 | ✓ |
| --socket-mode | 0660 | `(default "0660")` | ✓ |
| --socket-owner | — | 无 default 后缀 | ✓ |
| --base-path | —（/ 开头无尾斜杠） | 无 default 后缀 | ✓ |
| --auth-header | — | 无 default 后缀 | ✓ |
| --cwd | 继承（服务端 cwd） | `(default: inherit)` | ✓ |
| --term | xterm-256color | `(default: xterm-256color)` | ✓ |
| --stop-signal | HUP | `(default "HUP")` | ✓ |
| --stop-timeout | 0（不补发） | 无 default 后缀（DurationVar 零值） | ✓ |
| --uid / --gid | —（成对强制） | `(default -1)`（-1 = 不降权哨兵） | ✓ |
| --open | false | 无 default 后缀（BoolVar false 不打印） | ✓ |
| --ping-interval | 5s | `(default 5s)` | ✓ |
| --port / --bind / --writable / --write-policy / --max-clients / --once / --exit-when-empty / --credential / --tls-cert / --tls-key / --no-auth / --insecure-http / --origin / --client-option / --osc52 / --version / --help | （flag 表既有行，前序 phase 已核） | 逐字一致 | ✓ |

部署节内行为表述与实现交叉核对：--socket×--port/--bind 互斥、--socket-mode/--socket-owner 单给拒绝、--socket×--open 拒绝、--base-path 五族拒绝与根 / 归一、307 规范化、auth-header「无认证效力」、伪造头不绕过认证、--uid/--gid 成对强制 exit 2、--stop-timeout 0 = 不补 KILL、headless 跳过不阻断、退出码 255 同源——均与 main.go validateStartup/parseArgs 及 server 实现逐字对应。

## Decisions Made

- **A1 复核结论确凿落文**——plan 允许两分支（pw/人工复核后落文 或 标注防御性建议）；执行期以本机 nginx 1.14.1 实测闭合 A1（强于推演级复核），README 按「精确块必需」确凿落文，07-UAT.md C 节留档。
- **SEC-07 勾选状态解读**——plan 字面「勾选状态保持 [ ] 不变」与现状（已为 [x]，07-07 闭合时勾选）存在出入；按「本任务只改文本、勾选状态属验证流程职责」意图执行：保持 [x] 现状不变更。
- **1001 关闭码表行与 flag 表补全**——均属同主题文档完整性修正（Rule 2 文档面），先例明确（07-02/07-05 头注释计数修正、05-09/06-07 flag 表先例）。

## Deviations from Plan

None - plan executed exactly as written（Task 2 验证夹具初版两处缺陷属 /tmp 临时冒烟脚本自身问题，非仓内资产与产品缺陷，修正后全绿——见 Issues Encountered）。

## Issues Encountered

- **/tmp 冒烟脚本初版两处夹具缺陷（非产品缺陷，随验证修正）：** ① `waitLine` 顺序两次监听同一 stdout stream，第二次等待丢失第一次已读数据（unix 退化行超时）——改共享缓冲 waiter；② Node 原生 WebSocket 默认 `binaryType='blob'`，`new Uint8Array(ev.data)` 读不到帧（WS Welcome 超时）——按 phase07.mjs 既有形态置 `binaryType='arraybuffer'`。修正后冒烟 8/8 PASS；临时脚本与 clone 目录已清理（/tmp/wesh-uat/wesh 保留为 UAT 脚本既定二进制路径）。

## User Setup Required

None - no external service configuration required.

## Threat Flags

None——本 plan 为文档与验证交付，无代码面变更；plan `<threat_model>` 两条按 mitigate 兑现：T-07-08a（全部示例用占位值——`alice:pw-of-alice` 假凭据，示例 TOML 无真实 secret）、T-07-08b（「--auth-header 仅反代后部署」明示 + 暴露面警告机制交叉引用 + ttyd -H 模型差异段落防误用预期）。无未建模的信任边界扩张。

## Known Stubs

None——无占位实现；README 全部配方与表述经六段式/十脚本/冒烟实测支撑，flag 一致性逐条比对一致。07-UAT.md 的 A/B 节为「人工待办清单」属本 plan 既定交付物形态（manual-only 项的清单化承诺），非 stub。

## Next Phase Readiness

- **Phase 7 可进入 verify 流程**：OPS-01/02/04/05/09/11、SEC-07 全部经 Go 单测 + phase07.mjs 真实二进制端到端 + 文档收口锁定；07-UAT.md 人工清单 8 项待办（A1-A2/B1-B6）为 verify/UAT 阶段人工执行面
- **新 operator 按 README 可完成部署六形态**（配置文件含 chmod 600 / unix socket + systemd / nginx /wesh/ 子路径反代 / auth-header 反代归因 / 降权运行 / 优雅下线）——success_criteria 达成，无需读源码
- **SEC-07 D-15 修订双文件单一口径**落位（REQUIREMENTS.md + ROADMAP.md），前端身份显示与 --trusted-proxy 维持 CONTEXT deferred 登记不动

## Self-Check: PASSED

- 文件存在性：4/4 FOUND——README.md 含「## 部署与配置（Phase 7）」节；.planning/REQUIREMENTS.md SEC-07 修订行含「审计归因」（L49）；.planning/ROADMAP.md 含「审计归因」（≥1）；.planning/phases/07-deployment/07-UAT.md 存在（56 行 ≥ 40，9 勾选项 ≥ 8）+ 本 SUMMARY FOUND
- 提交存在性：1/1 FOUND（7ac957c，git log --oneline 核验）
- must_have 机械断言：`grep -c -- '--base-path' README.md` == 3 ≥ 3；`grep -c -- '--socket' README.md` == 8 ≥ 3；`grep -c -- '--auth-header' README.md` == 5 ≥ 2；`grep -c 'proxy_read_timeout' README.md` == 2 ≥ 1；ttyd 在部署节上下文命中 ×2（L338/L340）；`grep -c '审计归因' .planning/ROADMAP.md` == 1 ≥ 1
- 验收逐项：README flag 名/默认值与 --help 逐条一致（比对表见上）；六段式每段退出 0（gofmt 零新增漂移；git status 构建后干净）；裸 clone 产物冒烟通过（GET / 200 + Welcome）；十脚本输出全 PASS（结果表见上）

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
