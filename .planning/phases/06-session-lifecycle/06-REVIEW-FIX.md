---
phase: 06-session-lifecycle
fixed_at: 2026-08-23T10:33:09Z
review_path: .planning/phases/06-session-lifecycle/06-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 6: Code Review Fix Report

**Fixed at:** 2026-08-23T10:33:09Z
**Source review:** .planning/phases/06-session-lifecycle/06-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3（Critical 1 + Warning 2；Info 4 项按 scope=critical_warning 不处理）
- Fixed: 3
- Skipped: 0

**执行环境说明（#2825 登记）：** 全部修复在隔离 worktree（`.codebuddy/worktrees/rf-06-*`，临时分支
`gsd-reviewfix/06-*`，源自 main ff21572）内完成与验证；node_modules 以 symlink 接入
（`web/node_modules`、`web/uat/node_modules` 指向主仓实例），验证命令均在 worktree 路径下执行。
三笔修复提交经 `git merge --ff-only` 回收至 main，worktree/临时分支/recovery sentinel 已按
事务序清理（ff → worktree remove → branch -D → sentinel rm）。`docs(06-review)` 一笔因
06-REVIEW.md 在主仓为未跟踪文件（worktree 不可见），在清理后主仓直接提交。

## Fixed Issues

### CR-01: connect() 迟到成功的旧 attempt 无守卫踩掉健康连接

**Files modified:** `web/src/main.ts`、`web/uat/phase06-dom.mjs`、`web/dist/index.html`（构建产物随源码同 commit，repo 既定惯例）
**Commit:** `010a3df` — `fix(06-review): CR-01 connect() 迟到成功通道加 gen+welcomeDone 代际双查，提交句柄前 close 被取代 socket；phase06-dom 补 D10 场景`

**Applied fix:**
- 模块级新增 `let connectGen = 0`；`connect()` 入口 `const gen = ++connectGen` 取本链代际序号。
- **代际复查①**（fetch resolve 后、状态码分派前）：`if (gen !== connectGen || welcomeDone) return;`
  ——对 reviewer 字面建议的**有意扩展**：字面建议只把守卫放在建 socket 前，但 401/429/503 等终态
  面板分支同属 resolve 通道的迟到污染面（stale 链迟到 503 会落「Server is full」覆盖健康会话，
  与 06-03 deviation #1 在 catch 半侧收口的面板污染同族）；前置于分派前一并堵住。
- **代际复查②**（`resp.json()` 二次挂起后、`ws = new WebSocket(...)` 提交前）：同款
  `if (gen !== connectGen || welcomeDone) return;`——每个 await 挂起点后行动前复查。
- **取代关闭**：复查②通过者先 `ws?.close()` 再赋新句柄。关闭的是「本链取代的在飞旧 socket」
  （重连窗口期已 onopen/发出 Hello/完成 attach 而 WELCOME 未处理的残骸——不 close 则成幽灵
  owner 使新连接降级 ro，即场景 B 双击形态）。reviewer 警告的「不可 `ws?.close()` 反向修补」
  指 stale 链不得关当前健康句柄——本实现中 stale 链在复查①/②先行 return，执行到 close 的
  必是最新链，而健康会话存在时 `welcomeDone=true` 使任何新链在复查处即返回，故 close 只会命中
  null/已死 socket（幂等空转）或在飞残骸（本意），不会误杀健康连接。
- **测试**：修复面为 DOM 可观测（构造计数/面板/标题），按 reviewer 建议在
  `web/uat/phase06-dom.mjs` 补 D10 场景（`reconnect.test.ts` 只锁纯函数，gen 逻辑不可纯函数化，
  不为其引入新抽象——防过度设计）。配套夹具：`loadTerminal` 新增 `holdAttachFetchN` 选项
  （hold 第 N 次 fetch 的 resolve，请求照常发出）+ `ctx.releaseHeldFetch()`。
  D10 流程：首连建立 → synthClose(1006) → attempt 1 的 fetch 被闸 → 点击 Reconnect now 起
  attempt 2 建立健康会话 → 放行 stale fetch → 断言零新构造（D10a）/面板保持隐藏（D10b）/
  标题无 `[ro] ` 前缀（D10c）。

**Verification:**
- `node --test web/src/lib/*.test.ts` — 19/19 pass（exit 0）
- `time pnpm -C web --config.verify-deps-before-run=false build` — tsc + vite + gzip 全绿
  （exit 0，1.7s；dist 产物时间戳已验证新鲜）。注：pnpm 11 的 deps 预检会试图重整 symlinked
  node_modules 并安全拒绝（ERR_PNPM_UNSAFE_MODULES_DIR，未做任何修改），故加
  `--config.verify-deps-before-run=false` 跳过预检直跑脚本。
- `node web/uat/phase06-dom.mjs` — **33/33 pass + 1 skip（豁免），exit 0**（含新 D10 三断言）
- **负面对照**：stash 抠掉 main.ts 修复重建 dist 后 D10a FAIL（构造=3，踩占实证）、
  D10c FAIL（title="[ro] wesh"，ro 降级实证）；恢复修复后全绿。测试鉴别力与 bug 复现双证。

### WR-01: D7 会话建立 waitFor 硬编码 2000ms 与子进程 `sleep 2` 耦合

**Files modified:** `web/uat/phase06-dom.mjs`
**Commit:** `95ab12a` — `fix(06-review): WR-01 D7 会话建立 waitFor 期限 2000→5000ms，与子进程 sleep 2 自杀时点解耦`

**Applied fix:** 按 reviewer 建议逐字落地——D7 的
`waitFor(() => ctx.bu.on === 1, ..., 2000)` 期限 2000→5000（与 waitFor 默认族/waitReady 一致），
注释同步更新登记解耦理由（WS 帧按序到达 WELCOME→EXIT→close，等待无需抢在子进程退出前；
D7a 逐字文案/D7b 退出码/D7c 零新连接断言面不受影响）。

**Verification:**
- `node --check web/uat/phase06-dom.mjs` — syntax OK
- `node web/uat/phase06-dom.mjs` — 33/33 pass + 1 skip，exit 0（D7a/D7b/D7c 全 PASS）

### WR-02: UAT 异常通道绕过 assertOutputClean——启动超时 reject 原样回显 argv

**Files modified:** `web/uat/phase06.mjs`
**Commit:** `9d5e067` — `fix(06-review): WR-02 startWesh 启动超时 reject 凭据脱敏（redactArgs）+ 场景异常通道纳入 assertOutputClean 自净断言面`

**Applied fix:** reviewer 给出的两个选项（源头脱敏 / 异常消息纳入 emittedDetails）均落地，
纵深防御各堵一层：
1. **源头脱敏**：新增 `redactArgs()`——argv 中 `--credential` 后随值（空格形态）与
   `--credential=值`（等号形态）均替换为 `<redacted>`，flag 名保留不影响排障；
   startWesh 启动超时 reject 消息改用 `redactArgs(args)`。
2. **断言面延伸**：场景异常 catch 通道 `emittedDetails.push(String(e.message))`——
   assertOutputClean 自净扫描延伸到异常通道，未来任何异常消息携敏感值会显形为 SEC FAIL
   而非静默破线。

**Verification:**
- `node --check web/uat/phase06.mjs` — syntax OK
- redactArgs 逻辑单测（node -e 同款复刻）：空格/等号两形态 + 无凭据透传 + 无残留 4/4 OK
- `node web/uat/phase06.mjs` — **23/23 pass + 1 skip + SEC pass，exit 0**
- **负面对照**：`node web/uat/phase06.mjs /bin/true` 强制走启动超时通道——6 条场景异常消息
  全部显示 `--credential <redacted>`（S1/S3 形态），凭据值零出现；SEC 仍 PASS
  （details=7 含异常消息，证明异常通道已纳入扫描面）。

## Skipped Issues

None——全部 in-scope 发现均已修复。

**Out of scope（按 scope=critical_warning 未处理，留待人工裁决）：** IN-01（`shouldReconnect`
死导出收口）、IN-02（`--exit-when-empty=false` 文案）、IN-03（exiting 门置位窗口）、
IN-04（SIGHUP-only 无升级路径，D-13 锁定形态）。

---

_Fixed: 2026-08-23T10:33:09Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
