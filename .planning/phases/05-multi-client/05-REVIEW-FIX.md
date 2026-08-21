---
phase: 05-multi-client
fixed_at: 2026-08-21T02:54:34Z
review_path: /data1/home/zexueli/open_src/wesh/.planning/phases/05-multi-client/05-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report

**Fixed at:** 2026-08-21T02:54:34Z
**Source review:** .planning/phases/05-multi-client/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2（fix_scope=critical_warning：WR-01、WR-02；5 个 Info 项 IN-01..IN-05 不在本轮范围）
- Fixed: 2
- Skipped: 0

**Verification environment:** 全部构建/测试门禁在隔离 worktree（`.codebuddy/worktrees/rf-05-1906342-*`，临时分支 `gsd-reviewfix/05-1906342`）内执行——该 worktree 已随清理尾部拆除，main 分支已 fast-forward 收录两个修复提交，数字可从 main 分支当前树复现（Go 1.26.3，worktree 与主 checkout 同一代码路径）。每项修复提交前门禁：`go build ./...` + `go test ./...`（WR-01  scoped `./cmd/...` + 冒烟，WR-02 全量 `-count=1`）均绿。

## Fixed Issues

### WR-01: --credential 解析失败时 flag 包把原始值回显到 stderr（密码分量可落 journald）

**Files modified:** `cmd/wesh/main.go`, `cmd/wesh/main_test.go`
**Commit:** bcaf28a
**Applied fix:** 按同文件 client-option（clientOptErr）既有先例改记录式上报——`fs.Func("credential", …)` 回调内 `ParseCredential` 错误不再直接 `return err`（该路径会被 flag 包包装为 `invalid value %q for flag -credential: …` 并把原始 flag 值全文打印到 stderr），改为记入 `credErr`（`errors.New("invalid --credential: credential must be user:pass")`，只含错误类别、禁含值）并 `return nil`；于 `fs.Parse` 返回后、showVersion 早退之下的 clientOptErr 同点位统一上报。run() 的二次打印点（main.go 原 :317）因错误串本身零值内容而同步闭合。exit 2 fail-fast 语义与"配置错误零窗口暴露"不变。

与 review 建议的一处偏差：review 示例文案 `invalid --credential: must be user:pass` 并不含 TestTLSKeyPairError 断言的子串 `credential must be user:pass`（"credential:" 与 "must" 间有冒号隔断），按编排层指示采用含该子串的文案 `invalid --credential: credential must be user:pass`，既有断言保持绿色。

**Test 变更：** TestTLSKeyPairError 表新增 `forbiddenSub` 列（TestClientOptionError 同款先例），"malformed credential" 行断言 err 不含 flag 值 `no-colon-here`；其余行空串跳过。测试 doc 注释同步更新（记录式上报红线说明）。

**Verification:** `go build ./...` ✓；`go test ./cmd/... -count=1` ✓；行为冒烟： `go run ./cmd/wesh --credential ":S3cretP@ss" -- bash` 的 stderr 输出为 `wesh: invalid --credential: credential must be user:pass; usage: …`——密码分量零回显，exit status 2。

### WR-02: writer 批内同类型合并不区分帧类型——attach Welcome 与升格 Welcome 相邻合并后前端整帧丢弃

**Files modified:** `internal/server/clients.go`, `internal/server/clients_test.go`
**Commit:** 451a489
**Applied fix:** writer 的批内合并段抽出为纯函数 `mergeBatch`（clients.go writer 前邻），合并判定追加 `batch[i][0] == proto.Output`——合并仅限 OUTPUT 数据帧（字节流语义，拼接后与逐帧接收完全相同），控制帧（W/E，载荷为独立 JSON 文档）恒单发，杜绝 `W{...}{...}` 拼接产物触发前端 `JSON.parse` 抛错整帧丢弃。writer 主体改为对 `mergeBatch(batch)` 返回序列逐条 `conn.Write`（msgs 元素数 = 线上 WS 消息数），单帧零拷贝直写与合并帧先拷贝再拼接（P5-1 别名纪律）逐字保留。writer 与 mergeBatch 注释按库纪律补齐 WR-02 红线论证与可达时序说明。

**Test 变更：** clients_test.go（同包白盒，tickets_test.go/TestClientCountInvariant 先例）新增 `TestWriterMergeControlFramesOnly` 表测：两帧 Welcome 相邻 → 两条独立消息逐字节原样（WR-02 核心回归）；两帧 Error 相邻 → 两条独立；OUTPUT 连续段 → 合并单条（§2.5 既有行为不变）；混合 [O,O,W,W,O] → [合并 O, W, W, 单 O]；Welcome–OUTPUT 相邻 → 各自单发；单帧 Welcome 直通；空批 → nil。

**Verification:** `go build ./...` ✓；`go test ./... -count=1` 全绿（cmd/wesh、proto、pty、server 33.3s、web）；新回归测试 7 子例全过。

---

_Fixed: 2026-08-21T02:54:34Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
