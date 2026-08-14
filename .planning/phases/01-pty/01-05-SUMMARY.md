---
phase: 01-pty
plan: 05
subsystem: docs
tags: [readme, verification, race-detector, embed, gzip, smoke-test]

requires:
  - phase: 01-pty
    provides: 01-01..01-04 全部代码与测试（pty/server/web/CI）
provides:
  - README.md（用法/构建顺序/无认证警示/单次语义/env 白名单/协议帧）
  - Phase 1 全量收口验证记录（-race、前端构建、裸 clone embed 链、运行冒烟）
affects: [verify-work, ship, phase-02-protocol]

actuals:
  tokens: 820
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "文档补偿控制：D-05 accepted 风险（无认证+0.0.0.0）由 README 首屏警示兜底"

key-files:
  created: [README.md, .planning/phases/01-pty/deferred-items.md]
  modified: []

key-decisions:
  - "[Phase 01-05]: README 按现状描述裸 clone——dist 已提交真实构建产物（非占位），改前端源码才需先 pnpm -C web build"

patterns-established:
  - "全量收口验证模式：gofmt(GOROOT)/vet/-race 全测/前端构建/裸 clone 归档编译测试/运行冒烟 六段式"

requirements-completed: [CORE-01, FE-01, FE-03]

coverage:
  - id: D1
    description: "README 含 CONTEXT 强制内容：首屏无认证警示、单次语义说明、构建顺序、四 flag、SEC-06 env 白名单、协议帧形状"
    requirement: CORE-01
    verification:
      - kind: other
        ref: "grep 'Phase 1 无认证' / 'pnpm -C web build' / 'TERM=xterm-256color' README.md（全部命中）"
        status: pass
    human_judgment: false
  - id: D2
    description: "全量收口验证：-race 全测试、前端构建、裸 clone embed 链、运行冒烟（200/gzip/426/无残留）"
    requirement: CORE-01
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./... && pnpm -C web build && git archive 干净检出 build+test && curl 冒烟（见正文逐条退出码）"
        status: pass
    human_judgment: false
  - id: D3
    description: "浏览器手动四项：交互+env 白名单+第二连接 409、vim resize（FE-03）、WebGL→DOM 回落（FE-01）、exit 终结语义"
    requirement: FE-01
    verification: []
    human_judgment: true
    rationale: "渲染正确性、vim 重绘无闪屏、WebGL 禁用后回落等视觉/交互判断无法自动化断言；human_verify_mode=end-of-phase，由 end-of-phase 人工验证确认"
  - id: D4
    description: "浏览器手动 resize 同步（vim/远端 stty size 跟随）"
    requirement: FE-03
    verification: []
    human_judgment: true
    rationale: "同上——end-of-phase 人工验证项（VALIDATION 1-01-09）"

duration: 21min
completed: 2026-08-14
status: complete
---

# Phase 1 Plan 05: Phase 收口——README + 全量验证 Summary

**README 落地 D-05 文档补偿控制（首屏无认证警示 + 单次语义说明），六段式全量验证全绿（-race/构建/裸 clone/冒烟），Phase 1 进入可验证状态。**

## Performance

- **Duration:** 21 min
- **Started:** 2026-08-14T02:14:04Z
- **Completed:** 2026-08-14T02:35:29Z
- **Tasks:** 2
- **Files modified:** 2 created（README.md、deferred-items.md）

## Accomplishments

- README.md：首屏醒目警示"Phase 1 无认证，仅在可信网络使用"（引用块、仅次于标题）+ 单次语义说明（WS 断开即退出为预期行为，断线重连属后续阶段）——CONTEXT specifics 行 103/104 两项强制内容就位（T-05-01 缓解落地）
- README 完整覆盖：四 flag 及默认值（7681/0.0.0.0）、构建顺序硬依赖（pnpm -C web build 先于 go build，D-18）、SEC-06 env 白名单集合（TERM/COLORTERM 固定 + PATH/HOME/USER/LOGNAME/SHELL 按名 + LANG/LC_* 按前缀，T-05-02 缓解）、协议帧形状（'0'/'1'/未知 1002）
- 全量验证全绿（逐条退出码见下）；裸 clone embed 链经 git archive 干净检出双重证明；冒烟证明 gzip 双路径与 /ws 非升级拒绝
- human-check 四项留待 end-of-phase 人工验证（human_verify_mode=end-of-phase）

## Task Commits

Each task was committed atomically:

1. **Task 1: README.md** - `4fe5deb` (docs)
2. **Task 1 修正: 裸 clone 说明按现状修正** - `d787d3c` (docs，Rule 1 deviation)
3. **Task 2 附带: deferred-items 登记** - `def2dd2` (chore)

**Plan metadata:** 见最终 docs commit（complete 01-05 plan）

## 全量验证记录（Task 2，逐条命令 + 退出码）

| # | 命令 | 结果 |
|---|------|------|
| 1 | `gofmt -l .`（/usr/bin/gofmt） | 误报 syntax error——陈旧 gofmt 不识别泛型（已知问题，STATE 决策 01-03 已记录）；改用 GOROOT 版本 |
| 1' | `$(go env GOROOT)/bin/gofmt -l .` | 输出为空，exit 0 |
| 2 | `go vet ./...` | exit 0 |
| 3 | `go test -race -count=1 ./...` | exit 0（cmd/wesh 1.014s、internal/pty 2.033s、internal/server 1.170s；proto/web 无测试文件） |
| 4 | `pnpm -C web install --frozen-lockfile` | exit 0（Already up to date，pnpm 11.21.0） |
| 5 | `pnpm -C web build` | exit 0（1.748s；dist/index.html 456KB singlefile） |
| 6 | `test -f web/dist/index.html.gz` | exit 0 |
| 7 | `git archive → /tmp/wesh-clean && go build ./...` | exit 0 |
| 8 | `/tmp/wesh-clean: go test ./... -count=1` | exit 0（全包 ok）；事后已清理 /tmp/wesh-clean |
| 9 | 冒烟：`wesh-bin -- /bin/cat` 启动输出 | 恰一行 `listening on http://[::]:7681`（见 Deviation 2 环境说明） |
| 10 | `curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:7681/` | 200 |
| 11 | `curl -H 'Accept-Encoding: gzip' -D -` | 命中 `Content-Encoding: gzip`（Pattern 4 双路径） |
| 12 | `curl http://127.0.0.1:7681/ws`（非升级） | 426 Upgrade Required（非 200，符合预期） |
| 13 | kill 后 `pgrep -x wesh-bin` | exit 1（无残留）；`ss -tln` 确认 7681 已释放 |

`git status --short web/dist` 在构建前后均为空——已提交产物与真实构建字节一致，步骤 6 无需额外提交。

## Files Created/Modified

- `README.md` — 用法/构建顺序/安全警示/单次语义/env 白名单/协议帧/测试说明
- `.planning/phases/01-pty/deferred-items.md` — 出范围发现登记（研究缓存未 gitignore）

## Decisions Made

- README 裸 clone 段落按仓库现状撰写：dist 提交的是真实构建产物（01-01 be75125 覆盖占位），裸 clone 可直接构建运行；"先 pnpm build 再 go build"的硬约束仅在修改前端源码后适用。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] README 裸 clone 说明与仓库现状不符**
- **Found during:** Task 2（步骤 6 核对 dist 状态时）
- **Issue:** 计划文本假设仓库提交的是 `web/dist/index.html` 占位文件，README 初稿照此撰写；实际 01-01（be75125）已用真实构建产物覆盖占位，`git status` 构建后无改动（产物字节级确定性）
- **Fix:** README 构建段改写为现状描述（产物已提交、改前端源码须先重新构建）；embed 可编译性由 git archive 干净检出独立证明
- **Files modified:** README.md
- **Verification:** 三条 grep 强制内容复核通过；干净检出 build+test exit 0
- **Committed in:** `d787d3c`

**2. [Rule 1 - 环境差异] 冒烟启动行渲染为 `[::]` 而非 `0.0.0.0`**
- **Found during:** Task 2 冒烟
- **Issue:** 计划断言启动输出匹配 `listening on http://0.0.0.0:7681`；本机 Go 1.26.3 工具链对 `net.Listen("tcp","0.0.0.0:7681")` 直接发起 AF_INET6 `::` 绑定（strace 证实；`tcp4` 显式绑定则正常返回 0.0.0.0），属双栈监听（v4-mapped，curl 127.0.0.1 实测连通）。D-07 契约核心是"恰一行、无 banner、含实际地址与端口"——该契约满足；host 渲染为平台工具链行为，非代码缺陷
- **Fix:** 不改代码（修改 main.go 以迎合字符串属行为定制，且 `[::]` 渲染如实反映双栈监听事实）；按"恰一行 + `listening on http://<addr>` 格式 + IPv4 可达性实证"判定通过，标准 Linux/macOS 工具链下渲染为 `0.0.0.0:7681`（CI ubuntu/macos 可复核）
- **Files modified:** 无
- **Verification:** 单行 wc -l=1；curl 127.0.0.1:7681 返回 200

---

**Total deviations:** 2 auto-fixed（1 文档 bug、1 环境差异说明）
**Impact on plan:** 均为如实性修正，无范围蔓延；两项 must_have 强制内容不受影响。

## Issues Encountered

- /usr/bin/gofmt 陈旧（不识别泛型语法）报假 syntax error——沿用 01-03 已知结论改用 GOROOT gofmt，输出为空。
- 冒烟 kill 误杀背景子壳导致 wesh-bin 短暂残留——pkill -x 补杀并复核端口释放；最终无残留。

## Known Stubs

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 1 全部 5 个 plan 执行完毕；gofmt/vet/-race/前端构建全绿，仓库处于可推送状态
- **待人工**：end-of-phase 浏览器手动四项（交互+env+409、vim resize、WebGL 回落、exit 终结语义）——VALIDATION 1-01-08/09 + 成功准则整体确认
- CI 远端首次运行结果待推送后确认（计划明示不在本 plan 断言范围）

## Self-Check: PASSED

- 文件：README.md、deferred-items.md、01-05-SUMMARY.md 均存在
- 提交：4fe5deb、d787d3c、def2dd2 均在 git log 中

---
*Phase: 01-pty*
*Completed: 2026-08-14*
