---
phase: 09-release-polish
plan: 09
subsystem: docs-release
tags: [release-script, readme, deployment-docs, calibration, d-13, d-14, ops-03, ops-10]

requires:
  - phase: 09-release-polish
    provides: "09-01 发布链定稿（release.yml 对接面）/ 09-04+09-05 --index 全行为面（README-1/README-2 承诺语的被测物）/ 09-06 负载矩阵实测数据（标定表回填源）/ 09-07 Docker+systemd 实测证据 / 09-08 Caddy 双机实证锚点"
  - phase: 07-deployment
    provides: "phase07-a2-ctl.sh bash 入库脚本形态先例（脚本头/case 分派/usage exit 2）+ README nginx 配方先例（Caddy/CF 节同构落点）"
provides:
  - "scripts/release.sh：D-14 发布脚本（preflight 四闸 → 六段式同口径 → pnpm 前端固化 → 长 fuzz ×2 各 10min → 负载矩阵 → 确认闸 → git tag+push 触发 release.yml）+ --dry-run 干跑形态"
  - "README.md 五面更新：发布节（产物命名族/checksums 验证/脚本同源流程）/ --index 节（README-1/README-2 逐字承诺语 + index-max-size 纯配置键例外）/ flag 表 --index 行 + 配置节 29 键更正 / Caddy+CF+Docker+systemd 部署节（实证分级标注）/ 标定表 D-13 回填（12 行全量 + 实测结论）"
affects: [09-10（v1.0.0 确认门——release.sh 真实首跑即首证）, ship（ROADMAP SC3 部署文档面 + SC1 发布脚本面闭合）]

actuals:
  tokens: 6300    # 25194 chars / 4（两 task 提交 diff——release.sh 147 行 + README +126/-12）
  tasks: 2
  commits: 2      # 2 task commits + 1 docs commit

tech-stack:
  added: []       # 零新依赖零新工具（脚本仅编排既有 go/pnpm/git）
  patterns:
    - "验收 grep 机械纪律第七次沿用：脚本注释/干跑步骤清单措辞避开 fuzztime=10m / -tags=load / git push origin 字面（注释提及同样计数，==N 是源码级机械检查）——干跑步骤清单用描述性措辞承载，命令字面只在执行段单次出现"
    - "干跑四态验证的窗口期形态：好树态利用脚本未入库（untracked 即脏树）的窗口，mv 脚本至 /tmp 副本对仓跑四闸全过——闸序钉死（形态/已存在先于脏树闸）是各态在窗口期独立可触发的前提"
    - "文档即被测物（实证分级）：Caddy 标注「已全链实证 2026-08-30，Caddy v2.11.4」、CF 显著「未实测」（D-15 唯一例外）、Docker/systemd 与 09-07 实测证据逐条同源——无防御性建议式表述"

key-files:
  created:
    - scripts/release.sh
  modified:
    - README.md

key-decisions:
  - "D-13 标定表回填走验证结论形态：09-06 三断言全量现值成立零证伪 → 常量默认值零改动（git diff 零 .go 文件），README 12 行全量清单中负载敏感项附实测数据摘要（可溯源 09-06 LOADDATA）、时序项写「行为测试已锁 + 一阶依据复核成立」；实测日期取 2026-08-29（09-06 实跑日）非「2026-08/09」——数据出处可溯源纪律"
  - "闸④ 远端同步闸本机实跑走正常放行通道（fetch 连通 + upstream 在位 + behind=0、ahead=13 放行）——降级通道（skip note）未触发但形态在位（无网络/无上游环境自动降级，钉死文案登记）"
  - "shellcheck 缺席（本机无安装）：按 plan「缺席不阻塞」以 bash -n + 干跑四态行为自证 POSIX bash 形态；安装 shellcheck 不在本 plan 授权面（Rule 3 包安装排除纪律）"
  - "输入限速/输入队列两行的标定结论措辞与 09-06 实测证据同源：矩阵不含输入洪水格（数据面 = kicks=0 + 洪水触发 INPUT 全格送达 + TestInputRateLimit 行为锁）——不虚称未测量的输入丢弃计数"

requirements-completed: [OPS-03, OPS-10]

coverage:
  - id: D1
    description: "scripts/release.sh（D-14）：发布之前跑一次即可的全部操作单脚本整合——preflight 四闸（tag 形态/tag 不存在/工作树干净/远端同步降级）→ go vet + go test -race → pnpm install+build → 长 fuzz ×2（FuzzDecodeHello/FuzzDecodeFileConfig 各 10 分钟两次独立调用）→ -tags=load 负载矩阵（30 分钟上限）→ 确认闸（回显 tag+最近提交）→ git tag + push；--dry-run 只跑 preflight 打印步骤清单"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "bash -n + test -x + plan verify 块端到端 VERIFY_ALL_PASS：干跑四态按钉死文案分流（invalid tag format / already exists / working tree not clean / 好树步骤清单含 git tag 行 + exit 0 + 零副作用——无 tag 创建、树仍干净），临时 tag v9.9.9 与 .release-probe 探针随各态验毕即清、/tmp 捕获文件全清、脚本 mv 回"
        status: pass
      - kind: other
        ref: "验收 grep 组：set -euo pipefail ==1 / fuzztime=10m ==2 / -tags=load ==1 / git push origin ==1 且行号序在 confirm 之后（103 定义/144 调用 < 147 push）/ 首行 shebang #!/usr/bin/env bash"
        status: pass
    human_judgment: false
  - id: D2
    description: "README 五面更新：发布节（wesh_v1.0.0_* 四平台三件套 + checksums.txt sha256sum -c + 脚本同源流程 + D-02/D-16 两裁决写明）/ --index 节（README-1/README-2 逐字 + 16MiB/index-max-size 纯配置键例外 + 四拒绝零内容）/ flag 表 --index 行 + 配置节 29 键 + 两键示例 / Caddy（实证 2026-08-30 v2.11.4 四差异面）+ CF（未实测显著标注）+ Docker（scratch 零命令承诺）+ systemd（deploy/wesh.service 全配 + 255 交互）/ 标定表 D-13 回填 12 行全量"
    requirement: OPS-03
    verification:
      - kind: other
        ref: "plan verify 块全过：wesh_v1.0.0_linux_amd64.tar.gz ≥1 / index-max-size ≥1 / 未实测 ≥1 / 默认参数与标定 ==1 / !Phase 9 负载标定后回填 / deploy/wesh.service ≥1 / --index ≥1 + go test -race -count=1 ./... 五包全绿"
        status: pass
      - kind: other
        ref: "附加验收：挂账语 grep ==0（Phase 9 标定零残留）/ README-1 逐字（否则根路径与分享链接将失去终端功能）/ README-2 逐字（CSP 允许内联脚本与同源 WS 连接，但阻断外部源资源…与内建页同约束）/ 共 29 键 ==1 / 无对应 CLI flag ==2 / 验证成立分支 git diff 零 .go 文件"
        status: pass
    human_judgment: false

duration: 19min
completed: 2026-08-30
status: complete
---

# Phase 9 Plan 09: 发布脚本 + README 全量更新（D-13/D-14 落地）Summary

**scripts/release.sh（D-14：preflight 四闸 → 六段式 → pnpm 固化 → 长 fuzz ×2 → 负载矩阵 → 确认闸 → tag push）入库并经干跑四态机械断言（钉死文案逐字命中 + 好树零副作用）；README 五面落文——发布节产物族与脚本同源、--index 节两承诺语逐字（09-UI-SPEC §Copywriting）、Caddy/CF/Docker/systemd 部署节实证分级齐全、标定表 D-13 挂账清零（12 行全量实测结论，09-06 数据源），验证成立分支零 Go 改动**

## Performance

- **Duration:** 19 min（2026-08-30T08:15..08:34Z）
- **Tasks:** 2
- **Files modified:** 2（1 新建 + 1 修改）
- **Commits:** 2 task commits + 1 docs commit

## Accomplishments

- **scripts/release.sh（147 行，D-14 定稿形态）**：`#!/usr/bin/env bash` + `set -euo pipefail`（POSIX bash 入库可移植——非 fish）；顶部用法注释登记「发布之前跑一次即可」D-14 原话；函数化分段——preflight()（四闸钉死闸序：① tag 形态 `^v[0-9]+\.[0-9]+\.[0-9]+$` → ② `git rev-parse -q --verify refs/tags/` 存在即拒 → ③ `git status --porcelain` 非空即拒 → ④ fetch --dry-run + @{u} 落后即拒、ahead 放行、fetch 失败或无上游降级为 `release: upstream check skipped (no network or upstream)` 跳过提示不阻塞）；run_tests/build_web（`time pnpm -C web build`）/run_fuzz（两目标两次独立调用——Pitfall 4 单包单目标约束）/run_load（30 分钟上限）/confirm()（回显 tag + 最近 5 条提交 + release.yml 触发说明，应答非 yes 即中止）；尾部 `git tag "$V"` + `git push origin "$V"`（push 行号序在 confirm 之后——机械断言过）；`--dry-run` 只跑 preflight 并打印 b)-g) 七步描述性清单
- **干跑四态机械验证（plan verify 块端到端 VERIFY_ALL_PASS）**：坏形态 `1.0.0` → `release: invalid tag format (want vX.Y.Z)`；已存在临时 `v9.9.9` → `release: tag already exists`（验毕即删）；脏树探针 `.release-probe` → `release: working tree not clean`（验毕即清）；好树——利用脚本未入库窗口期 mv 至 /tmp 副本对仓跑 → 四闸全过（闸④ 本机 fetch 连通 + upstream 在位 + behind=0 正常放行，ahead 13 属发布物本就是本地新增的既定放行语义）+ 七步清单打印（含 `git tag v0.0.0` 行）+ exit 0 + 零副作用（无 tag 创建、树仍干净、脚本 mv 回原位）；仓库零残留（`git status --porcelain` 除 scripts 外零行）
- **README 发布节（新增，「构建」节后）**：产物命名族 `wesh_v1.0.0_(linux|darwin)_(amd64|arm64).tar.gz` 四平台三件套（wesh+LICENSE+README.md）+ `checksums.txt` 验证指引（`sha256sum -c --ignore-missing`）+ 发布流程 = `scripts/release.sh` 发布之前跑一次即可（脚本即发布文档的可执行形态——描述流程/承载流程同源不漂移）+ release.yml 编排说明（pnpm build 先于 goreleaser、CGO_ENABLED=0 仅发布构建单侧持有）+ D-02（仅 checksums.txt 无签名/SBOM）/D-16（不发布镜像）两裁决写明
- **README --index 节（新增，「部署与配置」内）**：整页替换语义（ttyd `-i` 同款）+ 双通道统一 + 改文件重启生效 + README-1/README-2 两义务承诺语逐字落文（终端功能须自行实现 POST /api/attach 换 ticket + wesh.v1 WS 协议回连 / 自包含单 HTML——CSP 允许内联与同源 WS、阻断外部源 CDN/webfont）+ 16MiB 默认上限与 `index-max-size` 纯配置键（整数字节、无对应 CLI flag 的明示例外）+ 四拒绝 exit 2 错误行零内容字节红线
- **README 更正**：flag 表加 `--index` 行（默认 —，整页替换/分享链接同效/重启生效）；配置文件节「26 个长期运行 flag 同名键 + command 共 27 键」→「27 个同名键 + command + index-max-size 纯配置键，共 29 键」+ TOML 示例补 index/index-max-size 两键 + 例外说明 bullet
- **README 部署节扩充（实证分级——文档即被测物）**：Caddy 配方（已全链实证 2026-08-30 Caddy v2.11.4；reverse_proxy 单指令 + 与 nginx 三差异面：Host 默认透传零 Host 行/WS upgrade 内建/站点地址裸 :PORT 字面 Host 匹配陷阱 + 无默认 WS idle 超时 × ping 5s 关系行）；Cloudflare 配方（显著「未实测」标注——D-15 诚实分级唯一例外：橙云代理/WebSockets 默认开启/空闲 ~100s 社区共识 × ping 5s「默认即安全」/TLS Full (strict)/`/s/{token}/` CF 明文可见脱敏延伸）；Docker 节（Dockerfile 引用 + CGO_ENABLED=0 构建前置 + 「本镜像不含任何可执行命令——`--` 后命令须来自 bind-mount 或 FROM 派生自建」承诺语 + tini PID 1 不加 -g + --socket volume + 不发布镜像）；systemd 节（deploy/wesh.service 引用 + Restart=on-failure/RestartSec=2/TimeoutStopSec=15s/LimitNOFILE=65536/EnvironmentFile 600 全配说明 + 255 交互段：on-failure 自主终结重启属服务形态期望、systemctl stop 永不触发重启、期望会话完即停改 Restart=no、manual stop 后 failed 态为正常纹理——与既有 SuccessExitStatus 注记口径调和）
- **标定表 D-13 回填**：表头「默认参数与 Phase 9 标定」→「默认参数与标定」；导引句从「初值为一阶推算…负载标定后回填」改写为已验证表述（2026-08-29 负载矩阵实测，`go test -tags=load`，验证为主证伪才改——零证伪零常量改动）；表格扩至 12 行全量挂账清单（新增 input queue 256KiB/attachGrace 500ms/pong 10s/hello 5s/EXIT 2s/stop-timeout 0/exit-when-empty 宽限七行）——负载敏感项（outbox/水位/input rate+burst/input queue/max-clients/attachGrace）附实测数据摘要（{1,4,16,32} 端逐字节一致 kicks=0/outbox 峰值 99.8% 精确转信用/Alloc 19.8MiB ≤ 64MiB/门 0.36/s 不震颤），时序项写「行为测试已锁 + 一阶依据复核成立」；方法论段保留并注明已兑现
- **验证成立分支零 Go 改动**：09-06 三断言全量现值成立零证伪 → git diff 零 .go 文件（D-12 证伪才改常量的既定分支）；全量 `go test -race -count=1 ./...` 五包全绿（README 变更零行为面影响的回归证据）

## Task Commits

Each task was committed atomically:

1. **Task 1: scripts/release.sh——D-14 发布脚本 + 干跑四态验证** - `bf9182e` (feat)
2. **Task 2: README 五面更新——发布/--index/部署节/标定表** - `a932d21` (docs)

**Plan metadata:** 见末尾 docs 提交（docs(09-09): complete ...）

## Files Created/Modified

- `scripts/release.sh`（新建，147 行，可执行）——函数化七段（preflight/run_tests/build_web/run_fuzz/run_load/confirm/main flow）；`--dry-run` 干跑形态；工具存在性检查（git/go/pnpm）
- `README.md`（+126/-12）——发布节、--index 节、Caddy/CF/Docker/systemd 四部署节、flag 表 --index 行、配置节 29 键、标定表 12 行回填

## Decisions Made

- **干跑步骤清单取描述性措辞**（验收 grep 机械纪律第七次沿用）：`fuzztime=10m`/`-tags=load`/`git push origin` 的命令字面只在执行段单次出现以满足 ==N 计数——干跑清单以「long fuzz 1/2: FuzzDecodeHello …(10 minutes)」「build tag: load」「push it to origin」等描述承载，信息量不减
- **标定表实测日期取 2026-08-29**（09-06 实跑日）而非 plan 字面「2026-08/09」——数据出处可溯源；结论措辞与 09-06 LOADDATA 逐条对得上（34.9MB/端、523,449B、19.76MiB、6 次/16.7s 全部转录自 09-06 SUMMARY 数据表）
- **输入限速/输入队列两行结论措辞与实测同源**：09-06 矩阵不含输入洪水格，结论列写「矩阵全格 kicks=0、洪水触发 INPUT 全格正常送达 + TestInputRateLimit 行为锁」——不虚称未测量的输入丢弃计数（文档即被测物纪律的诚实面）
- **闸④ 无 timeout 包装**：GNU timeout 非 darwin 默认可用（脚本须 Linux+macOS 可移植）；fetch 失败/无上游的降级通道已覆盖网络不可达，挂起风险由 git 自身连接超时承担
- **README-1/README-2 逐字落文不加代码标记包裹**：与 09-UI-SPEC §Copywriting 字面逐字一致（POST /api/attach、wesh.v1 等保持原文形态），便于验收 grep 逐句锚定

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] plan 标定表措辞「2026-08/09 负载矩阵实测」与实跑日期不符**
- **Found during:** Task 2 ⑤（标定表导引句撰写）
- **Issue:** plan 字面给「2026-08/09」，09-06 实际全量实跑日为 2026-08-29（其 SUMMARY Performance 节）——数据出处可溯源纪律要求日期与实跑记录一致
- **Fix:** 导引句写「2026-08-29，`go test -tags=load`——internal/server 黑盒负载测试」
- **Files modified:** README.md
- **Verification:** 与 09-06 SUMMARY Started/Completed 时间戳一致
- **Committed in:** `a932d21`

---

**Total deviations:** 1 auto-fixed（Rule 1 日期溯源修正）
**Impact on plan:** 交付物与 must_haves 逐字一致；无范围蔓延。

## Issues Encountered

- shellcheck 本机缺席——按 plan「缺席不阻塞」以 bash -n + 干跑四态行为自证；bash -n 过 + 四态输出/退出码全符预期（POSIX bash 形态的行为级自证）
- 闸④ 远端同步闸在本机走正常放行通道（fetch 连通、upstream 在位、behind=0/ahead=13）——降级通道文案在位但未触发；无网络环境下运行时降级为 skip note（脚本内钉死）

## Known Stubs

None —— release.sh 全部段落为可执行真实形态（真实首跑 = v1.0.0 发布，属 09-10 确认门用户裁决范围——plan done 标准既定）；README 五节全部有实测/行为面证据锚点（发布节↔09-01 预演、--index 节↔09-04/09-05、Caddy↔09-08 双机、Docker/systemd↔09-07 实测、标定表↔09-06 LOADDATA），无占位/TODO/防御性建议式表述。

## User Setup Required

None - no external service configuration required.（真实 v1.0.0 发布由 09-10 确认门承载：`./scripts/release.sh v1.0.0` 跑完前置校验+测试+fuzz+负载+确认闸后 tag push 触发 release.yml 首证全链。）

## Next Phase Readiness

- **09-10（收尾确认门）**：release.sh 真实首跑 = v1.0.0 发布首证（D-04 命名族逐字首证 + release.yml 全链首证，09-01 flagged_assumptions 既定验证取舍的兑现点）
- **ROADMAP SC3/SC1 闭合**：部署文档面（nginx+CF+Caddy+Docker+systemd 全配方）与发布脚本面（发布之前跑一次即可）双双落文；SC 标定面挂账清零（每个默认参数都有验证结论与数据出处）
- **ship 门**：README 即发布物三件套之一——发布节/checksums 指引随 tar.gz 分发
- 无阻塞项

## Self-Check: PASSED

- Files: scripts/release.sh（可执行，147 行）/ README.md（+126/-12）/ 09-09-SUMMARY.md（本文件）均 FOUND
- Commits: bf9182e（Task 1 feat）/ a932d21（Task 2 docs）均 FOUND（git log --oneline）
- 验收全过：Task 1 verify 块 VERIFY_ALL_PASS + 验收 grep 组（==1/==2/==1/==1/行号序）；Task 2 verify 块 + 附加验收（挂账语 ==0/README-1/README-2 逐字/29 键/零 .go 改动）+ go test -race 五包全绿

---
*Phase: 09-release-polish*
*Completed: 2026-08-30*
