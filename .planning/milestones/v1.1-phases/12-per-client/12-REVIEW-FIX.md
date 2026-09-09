---
phase: 12-per-client
fixed_at: 2026-09-04T16:32:50Z
review_path: .planning/phases/12-per-client/12-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 12: Code Review Fix Report

**Fixed at:** 2026-09-04T16:32:50Z
**Source review:** .planning/phases/12-per-client/12-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope（critical_warning）: 2（CR-01 Critical、WR-01 Warning；IN-01/IN-02 为 Info 不在范围）
- Fixed: 2
- Skipped: 0

## Fixed Issues

### CR-01: per-client 窗口放大后渲染钳制在过时 sessionDims——渲染尺寸与 PTY 实际尺寸分叉

**Files modified:** `web/src/main.ts`、`web/dist/index.html`（embed 链同 commit 重建）、`web/uat/phase12-dom.mjs`（补断言）
**Commit:** a3365a4
**Status:** fixed: requires human verification（逻辑修复——状态机逻辑已由 jsdom 红→绿端到端证据覆盖，残余人工面仅真实浏览器窗口 resize 观感，归 Phase 14 pw 层）

**Applied fix（reviewer 建议为指引，按实际代码状态适配扩展——最小补丁本身不足以消除可见症状）:**

1. **sendResize 同步 sessionDims（核心，reviewer 建议的落点 + 两处适配）**：per-client 分支在发送成功记账处同步 `sessionDims = {cols, rows}`，但有两处关键适配：
   - **镜像服务端 ClampDim [1,1000]**（`Math.min(v, 1000)`）：RESIZE 直通经 proto.go DecodeResize 钳制后才落 PTY，前端不镜像钳制则 fit>1000 时记账与 PTY 实际尺寸分叉（shared 下该保证由服务端 Welcome 推送回带，per-client 无推送须自证）。
   - **同步位置移到去重闸【前】而非发送成功处**：WELCOME 分支按帧赋值 sessionDims 后的统一 refit 走到 sendResize 时，窗口未变则上报被去重闸拦截——若同步只在发送成功处，该路径不再重申恒等式；attach 竞态（100ms resize 防抖恰在 Hello 后、WELCOME 处理前到期，真实上报先于 Welcome 到达）下 Welcome 携的 attach 时旧尺寸会覆盖较新的本端记账且 per-client 无后续 WELCOME 推送可自愈。去重闸前无条件重申使该覆盖在同一 refit 内即被纠正（红态无法直接构造该竞态，逻辑推演闭合）。
2. **refit() 顺序调整——上报先行**（reviewer 最小补丁的缺口）：原顺序「先算渲染 min(fit, sessionDims) → 后 sendResize」下，本事件渲染钳制取的还是上一代 sessionDims；per-client 无服务端 Welcome 推送可触发后续 refit，渲染将**永远落后事件一代**、停在 attach 尺寸——可见症状不消除。调整为 sendResize 先行后，同一事件内 `min(fit, sessionDims)` 即回恒等式（fit ≤ fit），渲染即时跟随。shared 路径 sendResize 不触 sessionDims（唯一刷新源仍是服务端 Welcome 推送），先报后算零行为差异——两侧均为同一同步任务内的本地操作，服务端对 RESIZE 的反应本就异步。
3. **零 W 帧 wire 契约保持**：修复全部在前端记账面，服务端零改动、不补发 Welcome（phase12.mjs S2c「双端零 W 帧」重跑实证保持）。
4. **jsdom 渲染跟随断言（reviewer 建议补，D2 现只查帧计数）**：phase12-dom.mjs 新增三条——
   - D2e（ro 半场 rows 轴）：布局突变 900x510 后 `.xterm-rows` 行 div 数 24→30（phase05-dom D6a 先例通道）；
   - D2f（rw 半场 wire 侧）：RESIZE 载荷精确 `{cols:98, rows:30}`（jsdom 桩下 fit 实测公式 `floor((900−14)/9)=98`、`510/17=30`——fit 插件 scrollback≠0 恒减 `ViewportConstants.DEFAULT_SCROLL_BAR_WIDTH=14`；既有注释的「80x24/100x30」为近似值，attach 实为 78x24）+ attach 基线 24 行；
   - D2g（rw 半场 cols 轴）：PTY 落定门（sleep(400) + `stty size` 回读 "30 98"，S2 协议层先例同款护栏）后输出 90 字符长行，断言单行 90 A 在场（98 cols 不折；缺陷态钳在 78 cols 折为 78+12——CR-01「折行错位」症状的直接判别；phase05-dom D6b `'A'.repeat(N)` 精确匹配先例通道）。
   - SpyWebSocket 夹具延伸：RESIZE 帧完整载荷记账（`sentResizePayloads`）。
5. **dist 重建同 commit**：`cd web && time pnpm build`（tsc 类型检查 + vite singlefile + gzip），构建确定性已验证（终态树重跑 pnpm build 后 git status 零差异——dist 与提交字节一致）。

**红→绿验证证据**（新断言先于修复对旧 dist 运行）：
- 红态（修复前 dist + 新断言）：D2e FAIL（渲染行数=24，钳制实证）、D2g FAIL（无单行 90A，折行错位实证）、D2f PASS（wire 侧上报恒为 fit 全值——证明缺陷纯在渲染记账面）、D1/D2a-d/D3/SEC 全 PASS（既有断言零扰动）——16/17。
- 绿态（修复后）：17/17 全 PASS。

**测试落地过程中的三处实测教训（已固化为断言注释，非产品缺陷）**：stty 回读须在服务端 50ms 防抖落定窗后（紧贴 rowsFollowed 时与防抖竞态读到旧尺寸）；`printf` 单参即 format 串需内层字面 `\n` 输出换行（否则 prompt 粘行破坏精确匹配）；typeText 末尾真 `\n` 是 Enter 派发约定（漏它命令不执行）。

### WR-01: phase12.mjs S5 未隔离 pinger/dwell 竞态——慢 CI 上存在窄假阳窗口

**Files modified:** `web/uat/phase12.mjs`
**Commit:** 56e0923
**Status:** fixed

**Applied fix（reviewer 建议直接采纳）:** S5 startWesh 实参加 `--ping-interval=0`（与 S6 line 671 同款隔离），并在落点登记裁决依据注释：S6 本阶段实证的交互（默认 ping 5s 下，stall 端 writer 阻塞持 writeFrameMu 时 ping tick 的 writeControl 内层 5s 写超时返回 DeadlineExceeded，被 pinger 误读为 pong 超时 → 1006 先杀）同样作用于 S5 的 RawStallClient——停读窗标称 ~3s，慢 CI 上 B 探针（echoMark 5s 超时上界）+ 30.9MB 管线排空延迟可把 writer mu 阻塞窗拉过首个 ping tick（attach+5s），tick+5s 处触发 1006 → S5a/S5b 假阳。S5 断言面（序号连续/连接存活/对端无扰）零依赖保活，隔离零弱化。phase12.mjs 全六场景重跑 20/20（S5a/b/c 含 34888896 字节序号连续校验全过）。

## Verification

**Where gates ran（#2825 记录）:** 全部验证在隔离 worktree（repo 相对路径 `.codebuddy/worktrees/rf-12-3310636-1788538073`，分支 `gsd-reviewfix/12-3310636`，含独立 node_modules：web/ `pnpm install --frozen-lockfile` + web/uat/ `pnpm install --ignore-workspace --frozen-lockfile`）内执行。worktree 树与 fast-forward 后的 main 逐字节一致（fix 提交即在其上），主仓检出自 fast-forward 起可直接复现全部数字。wesh 二进制均由 worktree 源码构建（`go build -o /tmp/wesh-uat/wesh ./cmd/wesh`）。

| Gate | 结果 |
|---|---|
| `go build ./...` | ✓ 通过（0.65s） |
| `go test ./internal/... -count=1` | ✓ 全 ok（proto 0.004s / pty 1.6s / server 70.8s；本次修复零 Go 源码改动，embed 链变更下全量复跑闭环） |
| `cd web && time pnpm build` | ✓（tsc 零错误 + vite 2.5s；重建后 git status 零差异 = 构建确定性） |
| `node uat/phase12-dom.mjs` | ✓ 17/17（红态 16/17 → 绿态全过；既有 D1/D2a-d/D3/SEC 断言零破坏） |
| `node uat/phase12.mjs` | ✓ 20/20（六场景；S2c 零 W 帧 wire 契约保持；S6 真实 dwell 10.3s 到期） |
| `node uat/phase06-dom.mjs`（零修改重跑） | ✓ 40/40 + 2 skipped（平台豁免）——refit 顺序调整对 shared 路径零行为差异的回归实证 |

## Out of Scope

- **IN-01**（`skip` 帮助函数零调用）、**IN-02**（phase12-dom 夹具面零消费）：Info 级，`fix_scope: critical_warning` 不含，未处理——留待 Phase 13/14 裁决（IN-02 reviewer 自注「Phase 14 参数化收编时若需要可保留」）。

---

_Fixed: 2026-09-04T16:32:50Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
