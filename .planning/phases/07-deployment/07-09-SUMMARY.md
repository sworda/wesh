---
phase: 07-deployment
plan: 09
subsystem: docs
tags: [nginx, reverse-proxy, base-path, host-forwarding, origin-check, uat, playwright, gap-closure, out-of-band]

# Dependency graph
requires:
  - phase: 07-deployment
    provides: 07-01 base-path（D-13/D-14）；07-07 phase07.mjs 协议套件；07-08 README 反代配方原文；07-UAT G-07-2 诊断（root_cause/artifacts/missing）
provides:
  - G-07-2 闭合——README 反代子路径配方补 proxy_set_header Host $http_host;（跨机浏览器按文档部署 WS 升级 403→101）+ 精确块理据文案按 proxy_pass 301 实证改写
  - pw 回归载具镜像修正配方：ctl conf() Host $http_host（e4d46bb）+ T5 预期 301 校准（ddda3a7）
  - 双机全链回归 5/5 锁定（2026-08-27：T1 308 / T2 页面+提示符 / T3 echo / T4 空闲 65s / T5 无精确块 301+/wesh/ 200），a2-home/a2-idle 截图重生
affects: [verify-work, ship]

# Actuals (#2632) — chars/4 over the realized diff（~3200 chars / 4 ≈ 800；out-of-band 执行无 executor 计量）
actuals:
  tokens: 800
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - 文档即被测物：README 部署配方与回归载具（ctl 生成器）逐字镜像——配方漂移即回归失效，文档正确性由全链自动化锁定而非人工复核
    - nginx handler 形态敏感语义：裸子路径在 proxy_pass 形态下自动 301 补斜杠（官方文档行为），return 探针形态下 404——探针结论不向异构 handler 推广

key-files:
  created:
    - .planning/phases/07-deployment/07-09-SUMMARY.md
  modified:
    - README.md
    - web/uat/pw/phase07-a2-ctl.sh
    - web/uat/pw/phase07-a2-pw.mjs

key-decisions:
  - "Host 变量选型 $http_host 单一形态：nginx 默认转发 Host=$proxy_host（127.0.0.1:后端口）与浏览器 Origin 不同源被 wesh originAllowed 403（origin.go:73 EqualFold）；$host 剥端口在 origin 含非默认端口时仍不匹配（已实证）——$http_host 全端口通吃，不为默认 80/443 另给 $host 分档"
  - "精确块从『必需』校准为『推荐』：无精确块时裸 /wesh 在 proxy_pass 形态下 nginx 自动 301 补斜杠（GET 入口可工作），C1 的 404 结论基于 return 200 探针 handler 不向真实配方推广；精确块价值 = 308 保方法 + 显式规范化、行为与 handler 无关"

patterns-established:
  - "配方行必要性三事实点注释形态：默认 $proxy_host 不同源 403 / $host 剥端口仍不匹配 / 必须 $http_host——每个事实句都有全链实证依据（403→101 修正前后对照）"

requirements-completed: [OPS-02]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "按 README 配方原样配置真实 nginx 后，跨机浏览器经反代访问 /wesh：308 重定向、页面加载、WS 升级成功、终端可用、空闲 65s 不断连（G-07-2 truth 原文达成）"
    requirement: OPS-02
    verification:
      - kind: e2e
        ref: "node web/uat/pw/phase07-a2-pw.mjs — T1 308+Location 尾斜杠 / T2 页面+提示符 / T3 echo 全链 / T4 空闲 65s 无断连面板 PASS（2026-08-27 5/5）"
        status: pass
      - kind: static
        ref: "awk 区域闸：README location /wesh/ 块内 proxy_set_header Host $http_host; 恰一处；旧 404 理据字面零残留"
        status: pass
    human_judgment: false
  - id: D2
    description: "回归载具镜像修正配方（ctl Host $http_host）且 T5 预期校准为 301 + Location 尾斜杠断言，/wesh/ 仍 200；双机全链回归 5/5"
    requirement: OPS-02
    verification:
      - kind: e2e
        ref: "node web/uat/pw/phase07-a2-pw.mjs — T5 无精确块裸 /wesh → 301 PASS；结果: 5/5 项通过（2026-08-27）"
        status: pass
      - kind: static
        ref: "grep 三闸：ctl Host 行 =1；pw status()===301 ≥1；status()===404 =0"
        status: pass
    human_judgment: false

# Metrics
duration: out-of-band（提交跨 2026-08-26 20:40–22:49 +0800）+ 回归 2min（2026-08-27）
completed: 2026-08-27
status: complete
---

# Phase 07 Plan 09: Gap Closure（G-07-2）Summary

**README 反代子路径 nginx 配方补 proxy_set_header Host $http_host; 并按 proxy_pass 301 实证重写精确块理据（G-07-2 闭合），pw 回归载具逐字镜像修正配方，双机全链回归 5/5 锁定——跨机浏览器按文档部署不再即坏**

## Performance

- **Duration:** out-of-band 执行（提交跨 2026-08-26 20:40–22:49 +0800）；回归运行 ~2 min（2026-08-27 10:05–10:07 +0800）
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- G-07-2（major，OPS-02）闭合：README「部署与配置 → 反代子路径」配方 `location /wesh/` 块新增 `proxy_set_header Host $http_host;`（README.md:321）+ 三事实点必要性注释——nginx 默认转发 Host=$proxy_host 与浏览器 Origin 不同源被 wesh originAllowed 同源校验 403（origin.go:73 EqualFold）；$host 剥端口仍不匹配（已实证），必须 $http_host
- 精确块理据文案按实证改写：删去「不经精确块则 404」旧句（return 200 探针结论，不向 proxy_pass 推广——A2 T5 证伪）；新文案 = 精确块推荐（308 保方法 + 显式规范化、handler 无关），缺省时本配方下 nginx 自动 301 补斜杠；节首表述校准为「前缀块必需；精确块推荐」
- 回归载具同步：ctl conf() Host 行 $http_host（e4d46bb 提前落地）；pw T5 由 404 等值断言校准为 301 + Location 尾斜杠断言（resp404→resp301），Check 名称与头注释同步 301 语义
- 双机全链回归 5/5（2026-08-27，Windows Playwright Chromium → LAN nginx 1.14.1 → wesh）：T1 308+Location / T2 落定 /wesh/ 终端就绪 / T3 echo 全链（A2WS_REPLM2）/ T4 空闲 65s 无断连面板且终端仍可用（A2IDLE_9M39NM）/ T5 无精确块 301 + /wesh/ 200；a2-home.png/a2-idle.png 截图重生留档（gitignore 纪律不变）

## Task Commits

out-of-band 执行（未经 /gsd:execute-phase 编排，提交直落 phase-07 分支），按时间序：

1. **Task 2 前置: ctl conf() Host $http_host 同步** - `e4d46bb` (test，2026-08-26 20:40；前次 Edit/cp 同块竞态致仓库遗留 $host 的修正，plan Task 2 action ① 的提前落地)
2. **plan 元数据: Task 2 action ② 补 L78 注释 301 改写指示** - `85febbf` (docs，21:19；仅 07-09-PLAN.md 一行)
3. **Task 1: README nginx 配方 Host 转发行 + 精确块理据 301 改写** - `7eae376` (docs，22:44)
4. **Task 2: pw T5 预期 301 校准（载具同步）** - `ddda3a7` (test，22:49)

**Plan metadata:** 本文件（2026-08-27 verify-work 流程追溯补档）

## Files Created/Modified

- `README.md` - 反代子路径配方 Host 转发行 + 必要性注释 + 精确块理据重写 + 节首必需/推荐校准（7eae376，+7/-4）
- `web/uat/pw/phase07-a2-ctl.sh` - conf() location /wesh/ 块 Host $http_host（e4d46bb）
- `web/uat/pw/phase07-a2-pw.mjs` - T5 301 校准：resp404→resp301、等值断言 301、Location 尾斜杠断言、Check 名称与头注释同步（ddda3a7，+7/-5）
- `.planning/phases/07-deployment/07-09-SUMMARY.md` - 本文件

## Decisions Made

- **$http_host 单一形态全端口通吃**（plan flagged_assumptions 登记）：非默认端口必需（实证）；默认 80/443 下 $host 亦可工作但不另给分档——配方取单一形态，默认端口形态按同一变量语义安全子集推断
- **精确块降档为推荐**：301 自动跳转是 proxy_pass 系 handler 特例（换 return/fastcgi 即不存在）；精确块保留价值 = 308 保方法 + 显式规范化——后续 handler 形态变化的读者不被旧 404 理据误导

## Deviations from Plan

### Process deviation（追溯登记）

**1. out-of-band 执行 + 回归证据 deferred**
- **Found during:** 2026-08-27 /gsd:verify-work 07 会话—— reconcile_gaps 发现 07-09 无 SUMMARY 工件
- **Issue:** 四提交于 2026-08-26 晚直落 phase-07 分支（未经 execute-phase 编排，无 SUMMARY）；且 plan Task 2 验收「回归 5/5 + 截图重生」在提交时点未执行——截图仍为 2026-08-26 19:57 修复前勘查轮产物
- **Fix:** 本 session 补跑 `node web/uat/pw/phase07-a2-pw.mjs`——5/5 通过（T4 含 65s 空闲窗，全程 ~2min），截图重生归档 web/uat/pw/screenshots/；本 SUMMARY 追溯补档
- **Verification:** 静态验收 grep 七闸全过（Task1: Host 行=1/旧理据=0/既有行≥1/围栏≥1；Task2: ctl=1/301≥1/404=0）；动态回归退出 0，`结果: 5/5 项通过`
- **Impact on plan:** 零 scope creep——改动面与 plan files_modified 三文件精确一致；Go 零改动（phase07.mjs 协议面无回归需求，07-10 已顺带兜底 34/34）

---

**Total deviations:** 1 process deviation（执行路径与证据时序，非实现变更）

## Issues Encountered

None——回归一次跑通 5/5，无 ctl diag/probe 通道介入。

## Verification Evidence

- **静态验收（2026-08-27 核验在案）**：
  - Task 1：`awk '/location \/wesh\/ \{/,/\}/' README.md | grep -cF 'proxy_set_header Host $http_host;'` = 1；`grep -cF '落不到反代块' README.md` = 0；`proxy_read_timeout 3600s` ≥1；`` ```nginx `` 围栏 ≥1
  - Task 2：`grep -cF 'proxy_set_header Host $http_host;' web/uat/pw/phase07-a2-ctl.sh` = 1；`grep -c 'status() === 301' web/uat/pw/phase07-a2-pw.mjs` = 1；`status() === 404` = 0
- **动态回归（2026-08-27 10:05–10:07 +0800）**：`node web/uat/pw/phase07-a2-pw.mjs` 退出 0——T1 308+Location 尾斜杠 PASS / T2 落定 URL 含 /wesh/ + 终端就绪 PASS / T3 echo 标记回读 PASS / T4 空闲 65s 无断连面板 + 终端仍可用 PASS / T5 无精确块 301 + Location 尾斜杠 + /wesh/ 200 PASS；`结果: 5/5 项通过`
- **截图留档**：web/uat/pw/screenshots/a2-home.png（10:06）、a2-idle.png（10:07）重生
- **红线保持**：pw 输出仅状态码/布尔/文案常量，凭据/token 零入 detail（T-07-09c 缓解核验）

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 07 三项 UAT gap 全部闭合（G-07-2 本 plan；G-07-3/G-07-8 见 07-10-SUMMARY）
- 07-UAT.md 残余：A1（平台豁免 skipped·风险接受）、B4（环境限制 skipped·风险接受）——2026-08-27 用户裁决
- README 配方为部署信任链终态；后续 handler 形态决策有实证理据可依

## Self-Check: PASSED

- FOUND: README.md / web/uat/pw/phase07-a2-ctl.sh / web/uat/pw/phase07-a2-pw.mjs（三改动文件均在）
- FOUND: e4d46bb / 85febbf / 7eae376 / ddda3a7（四提交均在 git log）
- FOUND: 回归证据 5/5 输出 + 两截图（2026-08-27 时间戳）

---
*Phase: 07-deployment*
*Completed: 2026-08-27（追溯补档——提交落地 2026-08-26）*
