---
phase: 05-multi-client
reviewed: 2026-08-21T00:53:15Z
depth: standard
files_reviewed: 29
files_reviewed_list:
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - go.mod
  - go.sum
  - internal/proto/proto.go
  - internal/pty/io.go
  - internal/server/auth_e2e_test.go
  - internal/server/clients.go
  - internal/server/clients_test.go
  - internal/server/e2e_test.go
  - internal/server/handshake_test.go
  - internal/server/keepalive_test.go
  - internal/server/limits_test.go
  - internal/server/multi_test.go
  - internal/server/resize_arb_test.go
  - internal/server/resize.go
  - internal/server/resize_test.go
  - internal/server/server.go
  - internal/server/sharetoken.go
  - internal/server/sharetoken_test.go
  - internal/server/slowclient_test.go
  - internal/server/tickets.go
  - README.md
  - web/dist/index.html
  - web/src/main.ts
  - web/uat/phase02.mjs
  - web/uat/phase03.mjs
  - web/uat/phase04.mjs
  - web/uat/phase05.mjs
findings:
  critical: 0
  warning: 2
  info: 5
  total: 7
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-08-21T00:53:15Z
**Depth:** standard
**Files Reviewed:** 29
**Status:** issues_found

## Summary

逐文件通读 29 个列入范围的文件（`web/dist/index.html` 为 vite-singlefile 构建产物/bundle，按范围规则仅做来源确认、不做缺陷分析）。重点核查了 Phase 5 新装的并发面与认证面：

- **并发正确性**：hubMu 单锁纪律（hubMu > outbox.mu 锁序、cond.Wait 原子放锁）、registry.n atomic 计数对称（removeLocked 单点收口）、client.mode atomic.Value 读写分界、信用门开闭/P5-7 四处 Broadcast 挂点——推演后未发现违例。信用门「暂存帧重投有序性」依赖的结构性不变量（任一时刻至多一个 creditBlocked 端、门闭合期间 outbox 只收重投帧）经全路径推演成立：kickOrCreditLocked 的分工表保证置位时其余可写端已全部 blocked 或已被踢，门开后首个 trySend 失败即把持信用端踢出，不存在「门开 + 他人持 pending 帧」窗口。
- **资源清理恰好一次**：halfOpen acquire→release（sync.Once + defer 兜底）、注册表移除（removeLocked 布尔判互斥）→ close(done)/cancel、writer/pinger/reader 三 goroutine 终结链路（kick 异步 Close→cancel 的 1013 可达性不变量）、input-writer 经 close(inputDone)+master Close 双通道收口——逐路径核对无泄漏、无双重释放。
- **认证/信息泄露红线**：SEC-01 运行期成立（server 包生产代码唯一输出面为 logEvent 三要素，无 token/ticket/凭据值参数）；SEC-02 单次使用（查即删）成立；share token subtle 比较 + SHA-256 等长化成立；401 无 oracle 同文有测试锁定；/s/ 门禁、token 第三通道、③位 503 闸与 /api/attach 早闸的守卫顺序与注释一致。
- **协议一致性**：帧常量两侧手工对齐无误；Welcome 运行期再推送复用 'W' 帧合规——但发现 writer 写合并对控制帧不安全的一处真实缺陷（WR-02）。

发现 2 个 WARNING（凭据 flag 值回显通道、writer 合并控制帧竞态）与 5 个 INFO，无 BLOCKER。

## Warnings

### WR-01: --credential 解析失败时 flag 包把原始值回显到 stderr（密码分量可落 journald）

**File:** `cmd/wesh/main.go:89-96`（二次打印点在 `:317`）
**Issue:** `--credential` 的 `fs.Func` 回调对 `ParseCredential` 错误直接 `return err`。flag 包（ContinueOnError）会把回调错误包装为 `invalid value %q for flag -credential: …` 并打印到 `fs.Output()`（stderr），其中 `%q` 处是**原始 flag 值全文**；`run()` 的错误出口（main.go:317）再把同一串打印第二次。`ParseCredential`（auth.go）拒绝的形态含空 user——`--credential ":S3cretP@ss"` 这类手误会把**密码分量**完整写进 stderr；README 推荐的 systemd 部署下 stderr 进 journald 持久化，构成 SEC-01「凭据任何形态永不进日志/错误文案」在启动面的破口。

同文件 main.go:113-117 的 client-option 注释自己逐字论证了这一泄露通道（「flag 包会把回调返回的错误包装为 invalid value %q…，%q 处正是原始串（值内容随之泄露）」）并为 `--client-option` 落地了记录式上报——但该纪律只应用于敏感度更低的 prefs 值，凭据 flag 反而裸奔，属同一红线的不一致应用。

**Fix:** credential 回调改 client-option 同款记录式（值内容零回显）：

```go
var credErr error
fs.Func("credential", "basic auth credential user:pass (repeatable; ...)", func(s string) error {
    c, err := server.ParseCredential(s)
    if err != nil {
        credErr = errors.New("invalid --credential: must be user:pass") // 只含错误类别，禁含值
        return nil
    }
    cfg.credentials = append(cfg.credentials, c)
    return nil
})
// ... fs.Parse 返回后（showVersion 早退之后，clientOptErr 同点位）：
if credErr != nil {
    return cfg, nil, credErr
}
```

注意 TestTLSKeyPairError 的 "malformed credential" 用例断言子串 `credential must be user:pass`——改记录式后文案需保持含该子串（上例新文案以冒号衔接，子串断言仍绿），并补一条「err 不含值内容」断言（TestClientOptionError forbiddenSub 先例）。

### WR-02: writer 批内同类型合并不区分帧类型——attach Welcome 与升格 Welcome 相邻合并后前端整帧丢弃

**File:** `internal/server/clients.go:567-578`（两处 Welcome 入队点：`internal/server/server.go:682` 升档、`internal/server/clients.go:531` promoteNextLocked）
**Issue:** writer 的批内合并把「同类型字节的连续帧」拼成单条 WS 消息，条件只看 `batch[j][0] == batch[i][0]`，不限帧类型。两帧 Welcome 相邻同批时合并产物的载荷是两段 JSON 直接拼接（`W{...}{...}`），前端 `JSON.parse` 抛错走入 catch 整帧丢弃（web/src/main.ts "discard malformed WELCOME"）。

可达时序：B 升档时 attach Welcome 先入队（server.go:682，hubMu 内）、`go s.writer(cl)` 在 hubMu 释放后才启动；此间隙 owner A 的 reader 终结 → `detach` → `promoteNextLocked` 命中 B → 第二帧 Welcome（升格 rw）入队到同一 outbox（clients.go:531）。B 的子进程若恰好无输出（静默 shell 常见），批内即 `[W1(ro), W2(rw)]` 相邻。窗口是 goroutine 调度间隙（µs–ms 级），高负载下可达。

后果：被升格端丢失该 Welcome 的**全部**应用——prefs 不应用、`welcomeDone` 永不置位（resize 浮层与 beforeunload 离开确认整会话不生效）、`--osc52` 的 rw 端 ClipboardAddon 不加载。mode 层面因前端默认态（isRO=false、stdin 启用）恰好等于升格后 rw 而「意外正确」，无提权后果，但功能降级静默发生。clients.go:551-553 注释宣称「1 WS message = 1 帧的线上纪律不变」，该断言只对 OUTPUT 合并成立，对 JSON 控制帧不成立。

**Fix:** 合并仅限 OUTPUT 数据帧，控制帧（W/E）恒单发：

```go
for j < len(batch) && len(batch[j]) > 0 && batch[j][0] == batch[i][0] && batch[i][0] == proto.Output {
    j++
}
```

回归测试建议：白盒构造 outbox 连续入队两帧 Welcome 后跑 writer 合并段，断言产生两条独立 WS 消息（或对合并函数抽出的判定逻辑做表测）。

## Info

### IN-01: connect() 的 per-connection 重置块未含 isRO/welcomeDone（Phase 6 重连落地前必修）

**File:** `web/src/main.ts:352-355`
**Issue:** `connect()` 每次尝试重置 `opened/helloSent/lastError`，但 `isRO`（ro 判定，sendResize/粘贴门共用）与 `welcomeDone`（浮层/离开确认会话门）不在重置列。当前唯一重入路径（auth_failed 静默重试一次）发生在 Welcome 到达前，故无活体影响；但 Phase 6 自动重连落地后，旧会话的 ro 态会残留至新 Welcome 到达前（吞 RESIZE/粘贴门），welcomeDone 残留使浮层/离开确认在新会话握手完成前即激活。
**Fix:** 重置块补 `isRO = false; welcomeDone = false;`（`osc52Loaded`/`retriedAuth` 为页面级门闩，保持不重置）。现阶段登记防漂移即可。

### IN-02: README 协议节两处与 05-06/05-07 落地语义漂移

**File:** `README.md:104-114` 与 `README.md:195`
**Issue:** ① 协议节称 Hello ticket「仅认证模式」「无认证模式省略该字段」——05-06 OQ1 已落地无认证模式分享 token 通道（token→/api/attach→一次性 ticket→Hello 核销，sharetoken_test.go:344-372 锁定），协议节未同步。② 容量节「并发握手瞬时超编 ≤8（per-IP 半开帽为界）」是**单源 IP**口径；多源 IP 并发握手时超编上界为 8×源 IP 数（每 IP 独立半开帽、注册前不计数）。容量策略非安全边界已声明，性质可接受，但措辞应精确免误读。
**Fix:** ① 协议节补「分享 token 通道（含无认证模式）Hello 同样携 ticket」一句；② 改为「单源 IP 瞬时超编 ≤ per-IP 半开帽（默认 8）」。

### IN-03: multi_test.go 混用字符串字面量与常量表示 write-policy

**File:** `internal/server/multi_test.go:91`、`internal/server/multi_test.go:150`
**Issue:** 两处 `WritePolicy: "all"` 用字面量，同文件 306/374/412 行用 `server.WritePolicyOwner`/`server.WritePolicyAll` 常量——双写漂移面（值域若变更，字面量处静默失真而非编译期报错）。
**Fix:** 统一为 `server.WritePolicyAll`。

### IN-04: phase02/03/04.mjs 按 stdout chunk 匹配启动行；各 UAT dialHello 无总超时

**File:** `web/uat/phase02.mjs:40-46`、`web/uat/phase03.mjs:63-70`、`web/uat/phase04.mjs:51-58`、`web/uat/phase05.mjs:93-107`
**Issue:** ① 02/03/04 的 startWesh 在每个 stdout chunk 上直接正则匹配 `listening on`——该行若跨 chunk 分块则永不命中，8s 超时假失败；phase05.mjs:71-85 已改为累积缓冲（stdoutBuf）形态，前三个脚本未回填。② 全部四个脚本的 dialHello 仅有「Welcome 到达」正路径与 onclose 负路径，无总超时 watchdog——被测二进制挂死时 UAT 永久悬挂而非失败收尾。
**Fix:** ① startWesh 累积匹配形态回填 02/03/04；② dialHello 加 `setTimeout` 拒绝分支（如 10s 未 Welcome 即 reject）。

### IN-05: unknown-frame 关闭 reason 为自然语言串，与机器串纪律不一致

**File:** `internal/server/server.go:779`
**Issue:** `_ = c.Close(websocket.StatusProtocolError, "unknown frame type")` 的 close reason 含空格自然语言；全库其余 reason 均为 snake_case 机器串（hello_timeout/frame_before_hello/malformed_hello/slow_consumer/auth_failed…），proto.go 头部关闭码纪律注释亦按机器串表述。前端只认 code 不认 reason（main.ts:600-601 注释明示），无功能影响，纯一致性。
**Fix:** 改 `"unknown_frame"`（≤123 字节约束不变，一并齐整 R-10 命名族）。

---

_Reviewed: 2026-08-21T00:53:15Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
