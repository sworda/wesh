---
phase: 14-herdr-uat
verified: 2026-09-08T06:55:00Z
status: passed
score: 6/6 must-haves verified
behavior_unverified: 0 # 无 PRESENT_BEHAVIOR_UNVERIFIED 真值——全部行为依赖型真值均有行为证据（本轮独立复跑：全量 -race 五包 + check-mermaid 正/负双向 + diff 纯净性审计）或执行期用户 approved 实跑记录（Playwright 2026-09-07 / UAT 人工闸 2026-09-07 / mermaid 目检 2026-09-08）
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 5/5
  gaps_closed:
    - "G-14-34：docs/ARCHITECTURE.md 组件图 mermaid 渲染目检（前验唯一遗留人工项）——14-13 gap closure 闭合（68ea29c 校验载具 + 17962d5 词法修复 + Task 3 用户目检 approved 2026-09-08），UAT test 34 转 pass（34/34）"
  gaps_remaining: []
  regressions: [] # 本轮独立复跑全量 -race 五包全绿（server 155.100s，与 14-12 基线 2m37.986s / 前验复跑 155.311s 同量级逐项吻合）；gap closure 链（b69574e~1..HEAD）实证零触达 internal/web/cmd——前验独立实跑证据（负载矩阵/run-all 17/17/phase02 抽测）对同一字节代码态持续有效
---

# Phase 14: 双模式验证矩阵、标定与 herdr UAT Verification Report

**Phase Goal:** 双模式零回归双证据收口；herdr/tmux driving scenario 端到端恢复正确行为；并发进程资源标定与双模式文档义务落地
**Verified:** 2026-09-08T06:55:00Z
**Status:** passed
**Re-verification:** Yes — G-14-34 gap closure 后复验（前验 2026-09-08T01:57Z，status: human_needed，5/5，唯一遗留人工项为 mermaid 渲染目检）

## Goal Achievement

**验证方法注记**：本验证不信任 SUMMARY 声明。G-14-34 闭合链按完整三级验证（存在/实质/接线 + 行为复跑），前验已过五条 SC 按快速回归核验（存在性 + 基本健全性 + 代码态不变性实证）。本轮独立实跑四项核心证据：全量 -race 五包、check-mermaid 正向（全 PASS exit 0）与**负向对照**（修复前内容 FAIL exit 1——判别力独立复现，排除恒绿假阳）、diff 纯净性审计（16+/16- 且 hunk 域 L11-56）。关键不变性证据：gap closure 提交链 `git diff --name-status b69574e~1..HEAD` 实证仅触达 docs/ARCHITECTURE.md + scripts/ 三件 + .planning 五件，**internal/ web/ cmd/ 零文件**——前验的全部独立实跑证据（负载双剖面 26.8s、run-all 17/17、phase02 抽测 12/12）对字节不变的代码持续有效。

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | SC1：-race 双模式全量 Go 测试全绿（三维归类落地）；phase02-09 既有协议 UAT 默认 shared 零修改重跑全过（零回归双证据） | ✓ VERIFIED | **本轮独立复跑（复验）**：`go test -race -count=1 ./...` 五包全 ok——cmd/wesh 1.327s / internal/proto 1.016s / internal/pty 2.657s / **internal/server 155.100s** / web 1.011s，零 FAIL exit 0；与 14-12 收口基线（2m37.986s）及前验复跑（155.311s）逐项同量级吻合。**代码态不变性**：gap closure 链零触达 internal/web/cmd（git diff 实证 0 文件）——前验已证的 216 mode= 子测全过、81 双跑调用点、web/uat 零修改（5 A 零 M）、run-all.log 17/17 在场、phase02 抽测 12/12 证据链对同一字节代码持续成立 |
| 2 | SC2：协议层 UAT 断言 per-client 全链六项（双端双 pid、EXIT 不串台、resize 隔离、ro 门控、--once 退 255、spawn 失败 1011） | ✓ VERIFIED | 六项断言载体全部在场且字节不变（本轮 wc -l 实证：phase11/12/13 于 run-all.mjs 17 项矩阵内，phase14.mjs 572 行在场）；前验实证的 run-all.log 17/17（phase11 21/21+1skip、phase12 20/20、phase13 29/29）+ 头注释 D-13 纪律对不变代码持续有效 |
| 3 | SC3：herdr per-client 多客户端 attach 移动端不再压缩桌面 + Windows Playwright 全链观感断言通过 | ✓ VERIFIED | 载体在场且字节不变：phase14.mjs（572 行 19 check，S1 三通道互证 + S2 ro 汇聚）+ phase14-pw.mjs（416 行四 Check）本轮 wc -l 实证与前验记录逐项一致；三轮 18/18 协议层记录 + Windows 两轮 4/4 + 截图六帧人工复核——**14-09 Task 2 checkpoint:human-verify 用户 approved（2026-09-07）**，执行期人工确认门已闭合（前验已核对，代码零变更故证据持续有效） |
| 4 | SC4：并发进程负载矩阵（1/4/16/32 会话）实测资源有界 + 数据回填 maxClients 建议值与 README 资源义务段 | ✓ VERIFIED | load_test.go 在场（1183 行，本轮 wc -l 实证）且字节不变——前验独立复跑（26.8s 双剖面全绿，LOADDATA 八行：spawn_total==N、kicks=0、gor Δ=5N+1 精确、fd Δ=4N 精确、rss_sum≤160MB）对同一代码持续有效；README「per-client 资源义务与实测标定」节（:121）+ CONFIGURATION.md :155 per-client 语义行（兼任并发进程上限 + spawn 前复检）本轮 grep 实证在场 |
| 5 | SC5：README/CONFIGURATION/ARCHITECTURE 补 per-client 模型段 + GoTTY 误记修正 | ✓ VERIFIED | 本轮逐项实证：README「会话模式」节 :96（模式语义 + --stop-timeout 双默认值分岔）+ 标定节 :121；ARCHITECTURE.md :7 GoTTY 勘误在场（「GoTTY 实为 per-connection spawn，源码已核实」）+ 双模式架构节 + goroutine 拓扑对比图（**该图渲染目检已由 G-14-34 闭合闭环，见 truth 6**）；CONFIGURATION.md :155 语义行。REQUIREMENTS.md:91-92 PC-12/PC-13 双 [x] + Traceability Complete（本轮 grep 实证 :202-203） |
| 6 | G-14-34：docs/ARCHITECTURE.md 组件图在 GitHub/渲染器正常渲染（4 subgraph 成形、边完整、标签显示、无错误占位） | ✓ VERIFIED | **三级验证全过**。存在+实质：scripts/check-mermaid.mjs 69 行实质实现（jsdom 构造 DOM 注入 globalThis.window/document → dynamic import('mermaid') 顺序不可颠倒 → 逐块 `mermaid.parse` + exit-code 门）；修复实态：当前 HEAD L11-56 组件图 4 subgraph 合法 id + 引号标题（CMDWESH/SRV/PTYLAYER/WEBPKG）+ 12 处带标签边引号化（含 `FE -->|"GET / · /s/{token}/"| EMBED` 主修复点）本轮 sed 实读逐项在场。**行为证据（本轮独立复跑双向）**：正向 `node scripts/check-mermaid.mjs` → docs/ 全部 3 块 PASS（ARCHITECTURE×2 flowchart-v2 + TESTING×1）exit 0；负向对照（对 68ea29c 修复前内容运行）→ 块 1 FAIL「Lexical error on line 4. Unrecognized text.」exit 1——判别力独立复现，非恒绿。**diff 纯净性（本轮独立审计）**：git diff 68ea29c..17962d5 删/增各恰 16 行、7 个 hunk 旧/新侧行域全部落于 L15-56 组件图块内（块 2 双模式拓扑图/节点行/无标签边/围栏外正文零字节改动）。**人工门**：Task 3 blocking checkpoint:human-verify 用户 approved（2026-09-08）——4 subgraph 框成形含全角括号标题、边箭头完整、`GET / · /s/{token}/` 边标签完整显示、无语法错误占位、第二块如常零回归（14-13-SUMMARY + 14-UAT test 34 + STATE.md:205 三处记录一致）。UAT 34/34、G-14-34 status: resolved |

**Score:** 6/6 truths verified（0 present, behavior-unverified）

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/server/harness_test.go` | newTestServer 四形态小族装配点 | ✓ VERIFIED | 163 行在场（本轮 wc -l）；字节不变（gap closure 链零触达）；前验实证 19 文件 81 调用点 wired |
| `internal/server/load_test.go` | FloodMatrix + Resident 双剖面 + LOADDATA | ✓ VERIFIED | 1183 行在场；字节不变；前验独立实跑全绿 |
| `web/uat/phase14.mjs` | herdr driving S1 + ro 汇聚 S2 | ✓ VERIFIED | 572 行在场；字节不变；三轮 18/18 |
| `web/uat/pw/phase14-pw.mjs` | Windows 双 tab 观感断言 | ✓ VERIFIED | 416 行在场；字节不变；两轮 4/4 用户 approved |
| `web/uat/run-all.mjs` | 17 项 UAT 矩阵 runner | ✓ VERIFIED | 150 行在场（前验记录 151 行——尾行差异为 wc 统计口径，内容零变更 git 实证）；17/17 记录在场 |
| `web/uat/minrepro-p14.mjs` + `web/uat/pw/minrepro-win.mjs` | 海森bug 最小复现双件套 | ✓ VERIFIED | 98+96 行在场；字节不变 |
| `scripts/check-mermaid.mjs` | mermaid 词法校验载具（G-14-34 防复发件） | ✓ VERIFIED | **本轮新增核验**：69 行实质实现；**wired**：本轮独立实跑正/负双向（3 块 PASS exit 0 / 修复前 FAIL exit 1）——判别力已证，非常驻孤儿 |
| `scripts/package.json` + `scripts/pnpm-lock.yaml` | 载具依赖（jsdom ^30.0.1 + mermaid ^11.17.2） | ✓ VERIFIED | package.json private + type:module + 双依赖钉版在场；lockfile 1182 行冻结；**红线**：不进 CI/生产（ci.yml 零 diff 本轮实证），node_modules 被 .gitignore 覆盖 |
| `docs/ARCHITECTURE.md` | 组件图词法修复 + GoTTY 勘误 + 双模式架构段 | ✓ VERIFIED | :7 勘误 + :92 节 + 组件图修复实态本轮 sed 实读；diff 纯净性审计通过（16+/16-，hunk 域 L11-56） |
| `README.md` | 「会话模式」节 + 标定表 + 建议值表 | ✓ VERIFIED | :96 / :121 本轮 grep 实证；数据锚定 LOADDATA 实测（前验 30 项程序化核对 + 复跑同源） |
| `docs/CONFIGURATION.md` | max-clients per-client 语义行 | ✓ VERIFIED | :155 行在场（兼任并发进程上限 + spawn 前复检——本轮 grep 实证） |
| `.planning/REQUIREMENTS.md` | PC-12/PC-13 勾选 + Traceability | ✓ VERIFIED | :91-92 双 [x] + :202-203 Traceability Complete 本轮 grep 实证 |
| `.planning/ROADMAP.md` | Phase 14 收口登记（13/13） | ✓ VERIFIED | 14-13 gap closure [x] + 「Plans: 13/13 plans executed」本轮 roadmap query 实证；Phases 列表复选框按流程惯例由 orchestrator 验证后勾选，非缺口 |
| `.planning/phases/14-herdr-uat/14-UAT.md` | UAT 矩阵 34/34 + G-14-34 resolved | ✓ VERIFIED | Summary: total 34 / passed 34 / issues 0 / pending 0 / skipped 0；Gaps: G-14-34 status: resolved（resolved_by 14-13-PLAN.md, 2026-09-08）本轮实读 |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| check-mermaid.mjs | mermaid 11.17.2 parse 引擎 | jsdom 注入链（window/document → dynamic import） | ✓ WIRED | 本轮独立实跑双向证接线：正向 3 块 PASS exit 0；负向（修复前内容）FAIL exit 1——链路真实承载判定而非空转 |
| 负对照（68ea29c 修复前） | 修复（17962d5） | 红绿链（exit 1 → exit 0） | ✓ WIRED | 本轮独立复现负对照（git show 68ea29c 内容运行得 Lexical error FAIL）——判别力先于修复成立，T-14-35 威胁面（恒绿假阳）排除 |
| internal/server 19 测试文件 | harness_test.go | newTestServer 小族（81 调用点） | ✓ WIRED | 前验 grep 实证；代码字节不变；本轮 -race 五包全绿复核 |
| load_test.go LOADDATA | README 标定表 | 实测行回填 | ✓ WIRED | 前验 30 项程序化核对 + 复跑同源；README/CONFIGURATION 本轮在场实证 |
| README herdr 配方 | web/uat/phase14.mjs | argv 逐字一致（文档即被测物） | ✓ WIRED | 前验逐字核对；两侧字节不变 |
| run-all.mjs | 17 项 UAT 脚本 | 串行 spawn 聚合 exit code | ✓ WIRED | SCRIPTS 17 项与磁盘核对（前验）；run-all.log 17/17 在场 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| check-mermaid.mjs 判定输出 | 每块 PASS/FAIL + diagramType | mermaid.parse 实际解析结果（非静态返回） | ✓ 是——负对照证明 FAIL 路径真实可达（修复前内容触发 Lexical error） | ✓ FLOWING |
| README 标定表 | N=1/4/16/32 各列 | load_test.go LOADDATA 实测 | ✓ 是（前验复跑同源，代码不变） | ✓ FLOWING |
| ARCHITECTURE 拓扑账面 | shared 3+3N / per-client 1+6N | perclient.go/server.go 装配事实源 + 实测交叉引用 | ✓ 是 | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| G-14-34 正向：全 docs mermaid 块词法合法 | `node scripts/check-mermaid.mjs` | 3 块全 PASS（ARCHITECTURE×2 flowchart-v2 + TESTING×1），exit 0 | ✓ PASS |
| G-14-34 负对照：判别力自证 | `git show 68ea29c:docs/ARCHITECTURE.md > /tmp/arch-prefix.md && node scripts/check-mermaid.mjs /tmp/arch-prefix.md` | 块 1 FAIL「Lexical error on line 4. Unrecognized text.」+ 块 2 PASS，exit 1 | ✓ PASS |
| diff 纯净性：修复恰 16+/16- 且域内 | `git diff -U0 68ea29c 17962d5 -- docs/ARCHITECTURE.md`（awk 行计数 + hunk 域审计） | deleted=16 added=16；7 hunk 旧/新侧全部落于 L15-56；ALL-HUNKS-WITHIN-L11-56 | ✓ PASS |
| SC1 回归：全量 -race 五包（本轮独立复跑） | `go test -race -count=1 ./...` | 五包全 ok（server 155.100s，总 2m35s），零 FAIL | ✓ PASS |
| gap closure 链代码面隔离 | `git diff --name-only b69574e~1..HEAD -- internal/ web/ cmd/` | 0 文件——产品/测试代码零触达 | ✓ PASS |
| 红线：CI 与依赖面 | `git diff 03268f0^..HEAD --stat -- .github/ go.mod go.sum` | 0 行 | ✓ PASS |
| debt-marker 扫描（14-13 新件） | grep TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER on scripts/check-mermaid.mjs, package.json, ARCHITECTURE.md | 零命中 | ✓ PASS |

### Probe Execution

无 `scripts/*/tests/probe-*.sh` 形式声明探针（本 phase 验证载体为 Go 测试、UAT 脚本与 check-mermaid.mjs——后者为本轮实际执行的探针等效物，见 Behavioral Spot-Checks 前两行：正向 exit 0 + 负向 exit 1 双向证据）。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| PC-12 | 14-07, 14-11, 14-12, 14-13 | 模式语义文档三件套 + GoTTY 误记修正 | ✓ SATISFIED | README/ARCHITECTURE/CONFIGURATION 三件全在场（本轮逐项实证）；**G-14-34 闭合后 ARCHITECTURE 组件图渲染缺陷消除**——14-13 为 PC-12 文档渲染面的 gap closure（SUMMARY 明示不重复勾选）；REQUIREMENTS.md [x] + Traceability Complete |
| PC-13 | 14-01..06, 14-08, 14-09, 14-10, 14-12 | herdr/tmux 多客户端互不干扰 + 协议层 UAT + Windows Playwright | ✓ SATISFIED | 六项协议断言 + phase14.mjs 三轮 18/18 + phase14-pw.mjs 两轮 4/4 用户 approved + SC1/SC2 零回归双证据（代码态不变性实证，前验证据链持续有效）；REQUIREMENTS.md [x] + Traceability Complete |

Orphaned requirements：无——REQUIREMENTS.md 映射到 Phase 14 的仅 PC-12/PC-13，均被 plan 声明并验证。v1.1 里程碑 15/15 全闭合。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| （14-13 新件扫描：check-mermaid.mjs / package.json / ARCHITECTURE.md） | — | TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER | — | **零命中**（debt-marker gate 通过） |
| scripts/check-mermaid.mjs | :41 | MERMAID_BLOCK 正则 `\n` 硬编码不兼容 CRLF——CRLF 文档静默跳过假 PASS（14-REVIEW WR-06） | ⚠️ Warning | 潜伏失效（当前仓库全 LF，本轮实跑 3 块 PASS 真实绿）；修复一行（`\r?\n`）已登记 |
| README.md | :127-152 | 驻留剖面标定主体「bash」实为 `sh` 夹具（14-REVIEW WR-01） | ⚠️ Warning | 文档数据主体错标（dash 与 bash RSS 差异）；洪水剖面不受影响；修复方向已登记 |
| docs/CONFIGURATION.md | :142 | 退出码表缺 per-client SIGTERM exit 0 路径注记（WR-02） | ⚠️ Warning | 既有行为分叉未文档化（产品行为非本阶段改动）；修复方向已登记 |
| web/uat/pw/phase14-pw.mjs | :366 / :35-37 | 硬编码 server.log 路径（WR-03）+ 头注释宣称已移除的 stty 防线（WR-04） | ⚠️ Warning | 测试证据完整性缺陷非产品缺陷；默认配置实跑 4/4 不受影响 |
| web/uat/phase14.mjs | :542-549 | pid 泄漏自检无边界锚定，数字碰撞假红面（WR-05） | ⚠️ Warning | 假红方向（不产生假绿）；长期 flake 源，修复方向已登记 |
| TestMaxClients503/mode=per-client | multi_test.go | 非 -race 隔离 `-run` 形态时序 flake | ⚠️ Warning（已登记非回归） | deferred-items.md 登记在案（根因 + 三条修复方向 + 基线 9af7ce1 复现 3/3 实证为 14-02 遗留）；**本轮 -race 全量形态复跑全绿复核成立**（项目验证标准自 14-12 起为 -race） |
| SECURITY.md | — | 尚不存在（secure-phase hook active） | ℹ️ Info | 独立跟进项，非 PC-12/PC-13 义务面 |

14-REVIEW（2026-09-08T06:39Z，38 文件全量，supersede 早前 35 文件版）：**0 Critical / 6 Warning / 7 Info**——无安全漏洞、无核心断言假绿门禁缺陷、无产品行为回归面；产品代码零改动（`git diff 03268f0^..HEAD` 过滤 `_test.go` 后产品面为空）。全部 Warning 为 advisory（文档准确性/脚本健壮性），修复方向均已登记于 14-REVIEW.md，不构成 phase 缺口。

### Human Verification Required

（无——全部人工项已闭合）

前验唯一遗留人工项（mermaid 渲染目检）已经 14-13 Task 3 blocking checkpoint:human-verify **用户 approved（2026-09-08）**闭合：4 个 subgraph 框成形（含全角括号标题）、边箭头完整、`GET / · /s/{token}/` 边标签完整显示、无 mermaid 语法错误占位、第二块双模式拓扑图如常零回归——记录于 14-13-SUMMARY.md + 14-UAT.md test 34（source: human, result: pass）+ STATE.md:205，三处一致。且该项现有自动化等价物（check-mermaid.mjs mermaid.parse，本轮独立复跑正/负双向）常驻固化。

执行期已闭合的另两处人工门（前验已核对，不重复列出）：Windows Playwright 观感断言（14-09 Task 2，2026-09-07 用户 approved）、UAT 矩阵人工闸（14-12 Task 2，2026-09-07 用户 approved）。

### Gaps Summary

**无缺口。** 六条真值全部 VERIFIED：五条 Success Criteria 经快速回归核验成立（本轮 -race 五包独立复跑全绿 + gap closure 链零触达产品/测试代码的不变性实证，前验全部独立实跑证据对同一字节代码持续有效）；G-14-34 经完整三级验证 + 双向行为复跑闭合（正向 3 块 PASS exit 0 / 负向修复前 FAIL exit 1 判别力独立复现 / diff 16+/16- 域内纯净 / 用户目检 approved）。UAT 34/34、gap 清零、REQUIREMENTS PC-12/PC-13 双 SATISFIED、debt-marker 零命中、红线四查全零（CI/依赖面/产品代码/web 基点）。

遗留事项（不构成缺口，均已登记）：
1. **14-REVIEW 6 Warning / 7 Info**——advisory（WR-01/02 文档准确性、WR-03..06 脚本健壮性含 check-mermaid CRLF 潜伏面、IN-01..07 信息级），修复方向全部登记于 14-REVIEW.md
2. **TestMaxClients503 非 -race 隔离形态时序 flake**——deferred-items.md 登记（根因 + 修复三方向），-race 验证标准形态不受影响（本轮复核全绿）
3. **SECURITY.md**——secure-phase hook 独立跟进项
4. **v1.1 里程碑收口**（/gsd:complete-milestone + release.sh）——14-CONTEXT 既定 deferred，非本 phase 范围

---

_Verified: 2026-09-08T06:55:00Z_
_Verifier: Claude (gsd-verifier)_
