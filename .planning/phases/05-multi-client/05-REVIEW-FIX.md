---
phase: 05-multi-client
fixed_at: 2026-08-21T09:27:05Z
review_path: /data1/home/zexueli/open_src/wesh/.planning/phases/05-multi-client/05-REVIEW.md
iteration: 1
findings_in_scope: 7
fixed: 5
already_fixed: 2
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report

**Fixed at:** 2026-08-21T09:27:05Z
**Source review:** .planning/phases/05-multi-client/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 7（fix_scope=all：WR-01、WR-02、IN-01..IN-05）
- Fixed in this pass: 5（IN-01..IN-05）
- Already fixed previously: 2（WR-01 bcaf28a、WR-02 451a489，本轮回读验证确认在位，未重实现）
- Skipped: 0

**Verification environment:** 本轮修复在隔离 worktree（`.codebuddy/worktrees/rf-05-2811346-*`，临时分支 `gsd-reviewfix/05-2811346`）内执行并提交，清理尾部已将 main fast-forward 收录 5 个修复提交（5d77891..45d4bbf）——数字可从 main 分支当前树复现（Go 1.26.3，worktree 与主 checkout 同一代码路径；worktree 无 node_modules，web 侧 tsc 验证借主 checkout 依赖执行，详见 IN-01 条目）。Go 触及项（IN-03、IN-05）提交前门禁：`go build ./...` + `go test ./... -count=1` 全绿；UAT 脚本项（IN-04）门禁：`node --check` 四脚本 + phase02/phase05 真实二进制全量冒烟（30/30 协议断言通过，1 项平台豁免 skip）。

## Fixed Issues (this pass)

### IN-01: connect() 的 per-connection 重置块未含 isRO/welcomeDone（Phase 6 重连落地前必修）

**Files modified:** `web/src/main.ts`
**Commit:** 5d77891
**Applied fix:** `connect()` 重置块（原 352-355 行）补 `isRO = false; welcomeDone = false;` 并注释登记防漂移意图（Phase 6 自动重连前提）；`osc52Loaded`/`retriedAuth` 按 review 指示保持不重置（页面级门闩）。`isRO`（main.ts:175 `let isRO = false;`）与 `welcomeDone`（main.ts:185 `let welcomeDone = false;`）均为模块级布尔声明，赋值类型吻合。

**Verification:** Tier 1 回读确认重置块完整、周边代码无损；`tsc --noEmit`（借主 checkout node_modules 对 worktree tsconfig 执行）报错全部为 worktree 无 node_modules 导致的模块解析失败（@xterm/*、vite 找不到）及其级联 implicit-any，属环境性预存错误，352-359 行修改区间零报错——与修复前基线同集，非本次引入。

### IN-02: README 协议节两处与 05-06/05-07 落地语义漂移

**Files modified:** `README.md`
**Commit:** 481b9da
**Applied fix:** ① 协议节（:104）补「分享 token 通道（含无认证模式）Hello 同样携 ticket——token 经 `/api/attach` 换一次性 ticket 后随 Hello 核销」；帧表（:108）ticket 注记由「仅认证模式」同步为「认证模式或分享 token 通道携带」（同段内第二处「仅认证模式」表述，一并消除漂移）。② 容量节（:195）「并发握手瞬时超编 ≤8（…per-IP 半开帽为界）」改为「单源 IP 瞬时超编 ≤ per-IP 半开帽（默认 8）」，消除多源 IP 口径误读。

**Verification:** Tier 1 回读三处修订点确认（markdown，无语法检查器，Tier 3 兜底）。

### IN-03: multi_test.go 混用字符串字面量与常量表示 write-policy

**Files modified:** `internal/server/multi_test.go`
**Commit:** 2950b05
**Applied fix:** :91 与 :150 两处 `WritePolicy: "all"` 改为 `server.WritePolicyAll`，与同文件 306/374/412/483/923 行常量用法齐整——值域变更时编译期报错而非静默失真。

**Verification:** `go build ./...` ✓（0.6s）；`go test ./... -count=1` 全绿（cmd/wesh、proto、pty、server 33.5s、web）。

### IN-04: phase02/03/04.mjs 按 stdout chunk 匹配启动行；各 UAT dialHello 无总超时

**Files modified:** `web/uat/phase02.mjs`, `web/uat/phase03.mjs`, `web/uat/phase04.mjs`, `web/uat/phase05.mjs`
**Commit:** 45d4bbf
**Applied fix:** ① 02/03/04 的 startWesh 回填 phase05.mjs:71-85 的累积缓冲形态——新增 `stdoutBuf` 逐 chunk 累加、对缓冲整体做 listening 行正则（跨 chunk 分块不再永不命中）；各脚本既有 scheme 感知正则与注释逐字保留。phase05 的 share 行双段解析与 50ms 落定窗为 05 专属形态，未回流到 02/03/04（无分享链接可解析）。② 四个脚本 dialHello 统一加 10s 总超时 watchdog：未收到 Welcome 即 `clearInterval(poll)` + reject（`握手总超时：10s 未收到 Welcome`），resolve/onclose 两路径同步 `clearTimeout(watchdog)`——被测二进制挂死时 UAT 失败收尾而非永久悬挂。

**Verification:** `node --check` 四脚本全过；功能冒烟（`go build -o /tmp/wesh-uat/wesh ./cmd/wesh` 后实跑）：phase02.mjs 12/12 协议断言通过（含 4c 不发 Hello 负路径——watchdog 不干扰 hello_timeout 关闭断言），phase05.mjs 18/18 通过 + 1 项 headless 平台豁免 skip。

### IN-05: unknown-frame 关闭 reason 为自然语言串，与机器串纪律不一致

**Files modified:** `internal/server/server.go`
**Commit:** 7df5952
**Applied fix:** server.go:779 close reason `"unknown frame type"` → `"unknown_frame"`，与 hello_timeout/frame_before_hello/malformed_hello/slow_consumer/auth_failed 命名族齐整；proto.go:45 头部注释本就按 `unknown_frame` 机器串表述，代码与注释由此对齐。前端只认 code 不认 reason（main.ts:600-601），零功能影响。全库 grep 确认无旧文案残留引用。

**Verification:** `go build ./...` ✓（0.6s）；`go test ./... -count=1` 全绿（server 32.3s）。

## Already Fixed Previously (verified this pass)

### WR-01: --credential 解析失败时 flag 包把原始值回显到 stderr（密码分量可落 journald）

**Commit:** bcaf28a（前一轮 fix pass，fix_scope=critical_warning）
**Files modified:** `cmd/wesh/main.go`, `cmd/wesh/main_test.go`
**Verified state:** 本轮回读确认记录式上报在位——main.go:95 `var credErr error`、:99 回调错误记入 `credErr`（`invalid --credential: credential must be user:pass`，只含错误类别禁含值）、:169-170 fs.Parse 后统一上报点。未重实现、未改动。详见前一轮报告（本文件前版，git 历史 afb3e73）。

### WR-02: writer 批内同类型合并不区分帧类型——attach Welcome 与升格 Welcome 相邻合并后前端整帧丢弃

**Commit:** 451a489（前一轮 fix pass，fix_scope=critical_warning）
**Files modified:** `internal/server/clients.go`, `internal/server/clients_test.go`
**Verified state:** 本轮回读确认合并判定限 OUTPUT 在位——clients.go:566 `for j < len(batch) && len(batch[j]) > 0 && batch[j][0] == batch[i][0] && batch[i][0] == proto.Output`，控制帧（W/E）恒单发；本轮 IN-03/IN-05 的 `go test ./... -count=1` 全量运行亦含 WR-02 回归测试（TestWriterMergeControlFramesOnly）持续绿色。未重实现、未改动。

## Skipped Issues

None — 7/7 发现全部修复（2 项前轮已修、本轮验证；5 项本轮修复）。

---

_Fixed: 2026-08-21T09:27:05Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
