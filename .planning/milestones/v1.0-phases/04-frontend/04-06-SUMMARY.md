---
phase: 04-frontend
plan: 06
subsystem: docs
tags: [readme, uat, documentation, client-option, osc52, verification, phase-closeout]

# Dependency graph
requires:
  - phase: 04-frontend (plan 01)
    provides: --client-option/--osc52 flag 与 Welcome prefs 通道（文档化对象）
  - phase: 04-frontend (plan 02/03/05)
    provides: 前端接线路径（标题/超链接/剪贴板/浮层/beforeunload/prefs 应用）与 dist 产物——04-UAT.md 逐项对应的可观察行为来源
provides:
  - README 用户文档与 Phase 4 用户面能力同步（两新 flag 行 + 前端体验节 + Welcome prefs 协议行，D-11/D-12 文档义务闭合）
  - .planning/phases/04-frontend/04-UAT.md 渲染面人工确认清单（T1-T11，FE-02/FE-04/FE-05/FE-06/FE-07/CORE-03 全映射）
  - Phase 4 收口证据：六段式全绿 + 三套 UAT 回归（phase02 11/11、phase03 18/18、phase04 10/10）对同一新构建二进制
affects: [phase 04 verify-work（UAT 分流：D4 人工项）、ROADMAP 三条成功准则证据面、后续 phase 的 README 公开契约面]

# Actuals (#2632) — chars/4 over realized diff，与 plan estimate（20000, confidence: low）配对校准
actuals:
  tokens: 3042
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []  # 零新依赖（文档 plan；归档验证的 pnpm install 走 lockfile 既定链，T-04-SC accept 既定）
  patterns:
    - "六段式收口形态沿用 P1-P3 既定（GOROOT gofmt/vet/-race/web 构建/裸 clone 归档/启动冒烟）"
    - "UAT 分工先例沿用：协议层全自动化（phase04.mjs），渲染/平台行为面人工清单（04-UAT.md）"

key-files:
  created:
    - .planning/phases/04-frontend/04-UAT.md
  modified:
    - README.md

key-decisions:
  - "README 协议节正文 inline Welcome JSON 提及同步补 prefs 可选键——plan 指名表格行，同节正文与表格形状不一致属文档自相矛盾（Rule 1 一致性修复）"
  - "段 5 裸 clone 归档前置 rm -rf /tmp/wesh-bare——防历史归档残留污染本次 embed 链证据"

patterns-established:
  - "文档与 CLI 同源核对：flag 表新增行先对 main.go help 文案语义，再落 README（key_links 契约）"

requirements-completed: [CORE-03, FE-02, FE-04, FE-05, FE-06, FE-07]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "README 同步：flag 表 --client-option（白名单 10 键/值 JSON/非法启动报错）与 --osc52（默认关/只写不读/仅本 flag 可开启）两行；前端体验节（标题同步/超链接/剪贴板 HTTPS·localhost 明示/辅助交互/偏好优先级与 theme 保留调色板）；协议节 Welcome 行含可选 prefs 键；负向 grep 无 ?osc52 暗示"
    requirement: FE-07
    verification:
      - kind: other
        ref: "grep -q -- '--client-option'/'--osc52'/'HTTPS'/'prefs' README.md 全命中 && ! grep -q '?osc52' README.md（plan verify automated 逐字执行，退出 0）"
        status: pass
    human_judgment: false
  - id: D2
    description: "04-UAT.md 渲染面人工清单存在且覆盖 T1-T11（CJK/emoji、IME、标题同步、超链接、选中即复制、粘贴、明文降级、浮层、beforeunload 含 sticky activation 裁决记录、偏好下发与 query 覆盖、OSC52），逐项映射 FE-02/FE-04/FE-05/FE-06/FE-07/CORE-03"
    requirement: FE-06
    verification:
      - kind: other
        ref: "test -f .planning/phases/04-frontend/04-UAT.md + T1-T11 条目与需求映射人工核查"
        status: pass
    human_judgment: false
  - id: D3
    description: "Phase 4 收口自动化证据：六段式全绿（GOROOT gofmt 无输出 / go vet / go test -race 五包 ok / time pnpm -C web build 1.8s 产物时间戳新于源码且 dist 零漂移 / 裸 clone 归档 install+build+test 全绿 / 启动冒烟 listening 行含实际端口）+ 三套 UAT 对同一新构建二进制全 PASS（11/11、18/18、10/10）"
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./...（五包 ok）+ git archive 归档链 + node web/uat/phase02.mjs|phase03.mjs|phase04.mjs /tmp/wesh-uat/wesh"
        status: pass
    human_judgment: false
  - id: D4
    description: "04-UAT.md T1-T11 由用户在真实浏览器逐项确认（渲染/平台行为面：CJK/IME 视觉效果、hover tooltip、系统剪贴板、浏览器原生确认框、sticky activation 语义）"
    requirement: FE-02
    verification: []
    human_judgment: true
    rationale: "渲染/浏览器平台行为自动化结构性不可达（plan verify human-check 既定；UI-SPEC 🧪 backstop 行的人工出口）；清单已就绪待用户执行，全部 PASS 后 phase 渲染面方可宣告收口"

# Metrics
duration: 5h 57m（挂钟；含执行环境约 6 小时挂起，实际作业约 15 min）
completed: 2026-08-18
status: complete
---

# Phase 4 Plan 06: Phase 收口——README 同步 + 04-UAT.md + 全量验证 Summary

**README 同步 Phase 4 全部用户面（--client-option/--osc52 flag 行 + 「前端体验（Phase 4）」节 + Welcome prefs 协议行，D-11/D-12 文档义务闭合）+ 04-UAT.md T1-T11 渲染面人工清单落地 + 六段式与三套 UAT 全绿收口证据**

## Performance

- **Duration:** 5h 57m（挂钟 2026-08-18T17:21:41Z → 2026-08-18T23:18:18Z；执行环境挂起约 6 小时，实际作业约 15 min）
- **Started:** 2026-08-18T17:21:41Z
- **Completed:** 2026-08-18T23:18:18Z
- **Tasks:** 2
- **Files modified:** 2（1 新建 + 1 修改）

## Accomplishments
- README 三处同步落地：flag 表新增 `--client-option`（白名单 10 键、值 JSON、非法启动报错）与 `--osc52`（默认关、只写不读、仅本 flag 可开启）两行且与 main.go help 文案同源；安全说明节新增「前端体验（Phase 4）」五小节（标题同步/超链接/剪贴板/辅助交互/偏好下发与覆盖）；协议节 Welcome 行更新为 `{"mode":"ro"|"rw","prefs":{...}?}`
- D-11 文档义务逐字落实：剪贴板「**需 HTTPS 或 localhost 访问**——明文 HTTP 非 localhost 下浏览器不暴露剪贴板 API，选中复制与粘贴静默不生效」明示在文；负向 grep 确认全文无 `?osc52` 之类 query 开启暗示（D-12 安全不对称文档面闭合）
- 04-UAT.md 新建：T1-T11 渲染面人工确认清单（沿用 03-UAT.md 分工先例——协议层已由 phase04.mjs 十场景自动化，本清单只列浏览器/平台行为面），逐项映射 FE-02/FE-04/FE-05/FE-06/FE-07/CORE-03，含 sticky activation 零交互不弹框的裁决记录（04-RESEARCH §OQ3，记录非缺陷）
- 收口证据齐：六段式各段退出 0（GOROOT gofmt 无输出 / go vet / `go test -race -count=1 ./...` 五包 ok / `time pnpm -C web build` 1.8s 且产物时间戳新于源码、dist 重建零字节漂移 / 裸 clone 归档 install+build+test 全绿 / 启动冒烟 listening 行含实际端口）；三套 UAT 对同一新构建二进制全 PASS（phase02 11/11、phase03 18/18、phase04 10/10）

## Task Commits

Each task was committed atomically:

1. **Task 1: README——flag 表两行 + 前端体验节 + Welcome 协议更新** - `e1d1a14` (docs)
2. **Task 2: 04-UAT.md 人工清单 + 全量六段式验证 + 三套 UAT 回归** - `9fe281f` (docs)

**Plan metadata:** 见尾部 docs 提交

## Files Created/Modified
- `README.md` - flag 表 +2 行（37-38）、「前端体验（Phase 4）」节（86-92）、协议节 Welcome 行与正文 inline 提及（94、99）
- `.planning/phases/04-frontend/04-UAT.md` - 新建：三组启动形态（A rw / B ro / C 明文降级）+ T1-T11 清单 + PASS/FAIL 记录位

## Decisions Made
- **README 协议节正文 inline Welcome JSON 同步补 prefs**（Task 1）：plan 指名更新表格行（89 行），但同节正文（84 行）inline 引用同一 JSON 形状——只改表格会造成同节自相矛盾，按一致性一并修订（见 Deviations）。
- **段 5 归档前置 `rm -rf /tmp/wesh-bare`**（Task 2）：plan 字面 `mkdir -p` 在历史残留下会把旧文件混入归档目录，前置清理保证 embed 链证据纯净（卫生步骤，零语义偏移）。
- **段 6 冒烟 SIGKILL 未触发**：`-- true` 子进程即退，单次语义下服务端随退 rc=0，启动行 `listening on http://127.0.0.1:39535` 含实际端口；`timeout -s KILL 10` 兜底在场未使用。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] README 协议节正文 inline Welcome 提及与表格行形状不一致**
- **Found during:** Task 1（协议节 Welcome 行更新）
- **Issue:** plan 指名更新表格行（89 行），但同节正文（84 行）inline 写着 `` Welcome `{"mode":"ro"|"rw"}` ``——只改表格会在同一协议节内留下两种 Welcome 形状，文档自相矛盾
- **Fix:** 正文 inline 提及同步修订为 `{"mode":"ro"|"rw","prefs":{...}?}`（`prefs` 为可选键）
- **Files modified:** README.md
- **Verification:** 全文 grep `prefs` 两处命中形状一致；plan verify automated 全绿
- **Committed in:** e1d1a14（Task 1 提交）

---

**Total deviations:** 1 auto-fixed (1 bug——文档内部一致性)
**Impact on plan:** 最小一致性修补，D-13 语义逐字保持；无范围蔓延。

## Issues Encountered
- 执行环境两次挂起合计约 6 小时（后台任务 17:23 派发、23:16 才执行完毕）；期间派发的段 3 后台任务句柄丢失，前台重跑 `go test -race -count=1 ./...` 五包全 ok，结果等价。对产物零影响。

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 4 自动化面全绿收口：六段式 + 三套 UAT（11/11、18/18、10/10）；README 与 Phase 4 全部用户面能力同步
- **待用户执行**：04-UAT.md T1-T11 真实浏览器逐项确认（coverage D4）——全部 PASS 后 phase 渲染面方可宣告收口；执行时按清单前置条件构建并起 A/B/C 三形态实例
- 无阻塞项

## Self-Check: PASSED

- 文件存在性：README.md、.planning/phases/04-frontend/04-UAT.md 均 FOUND
- 提交存在性：`e1d1a14`（Task 1 docs）、`9fe281f`（Task 2 docs）均在 git log
- 验收复核：plan verify automated 各组件逐条执行全绿（grep 五连 / gofmt 无输出 / vet / -race 五包 / 三套 UAT 11/11·18/18·10/10）；acceptance_criteria 三条（README 内容点 / 04-UAT.md T1-T11 覆盖 / 六段式+UAT 退出 0）逐项 PASS
- 两次任务提交零意外删除（post-commit deletion check 均空）；无新未跟踪产物文件（dist 重建零字节漂移）

---
*Phase: 04-frontend*
*Completed: 2026-08-18*
