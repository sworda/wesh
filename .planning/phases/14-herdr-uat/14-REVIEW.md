---
phase: 14-herdr-uat
reviewed: 2026-09-08T00:40:34Z
depth: standard
files_reviewed: 35
files_reviewed_list:
  - README.md
  - docs/ARCHITECTURE.md
  - docs/CONFIGURATION.md
  - internal/server/auth_e2e_test.go
  - internal/server/auth_test.go
  - internal/server/basepath_test.go
  - internal/server/customindex_test.go
  - internal/server/e2e_test.go
  - internal/server/emptyexit_test.go
  - internal/server/exit_test.go
  - internal/server/handshake_test.go
  - internal/server/harness_test.go
  - internal/server/health_test.go
  - internal/server/keepalive_test.go
  - internal/server/limits_test.go
  - internal/server/load_test.go
  - internal/server/metrics_test.go
  - internal/server/multi_test.go
  - internal/server/origin_test.go
  - internal/server/perclient_test.go
  - internal/server/proxy_e2e_test.go
  - internal/server/proxy_test.go
  - internal/server/resize_arb_test.go
  - internal/server/sharetoken_test.go
  - internal/server/shutdown_test.go
  - internal/server/slowclient_test.go
  - internal/server/stopseq_test.go
  - internal/server/throttle_test.go
  - internal/server/tickets_test.go
  - internal/server/tls_test.go
  - web/uat/phase14.mjs
  - web/uat/run-all.mjs
  - web/uat/minrepro-p14.mjs
  - web/uat/pw/minrepro-win.mjs
  - web/uat/pw/phase14-pw.mjs
findings:
  critical: 0
  warning: 3
  info: 5
  total: 8
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-09-08T00:40:34Z
**Depth:** standard
**Files Reviewed:** 35（3 docs + 27 Go 测试 + 5 UAT 脚本；另交叉核对 web/uat/pw/lib/*.mjs 4 个载具文件与产品侧 metrics.go/health.go/perclient.go 锚点）
**Status:** issues_found

## Summary

本 phase 为测试/UAT/文档收口，按设计零产品代码改动——**已实证确认**：`git diff 03268f0^..HEAD -- internal/ cmd/ web/src/ web/embed.go` 过滤 `_test.go` 后为空（NO_PRODUCT_CODE_CHANGES），无意外产品代码漂移。

**总体评估：质量高于平均水平。** 27 个 Go 测试文件的双模式 fork-table 改造（`for-mode + t.Run` 双跑、shared 列期望值逐字保持、per-client 列显式可证伪断言）经逐文件走查未发现断言级错误；测试断言的产品侧锚点（metrics HELP 双模式文案、healthz `session_active` 恒 true、`server is at capacity`/`failed to start process`/`slow_consumer` 字面量、proto `Welcome.session` 恒序列化）逐一与产品源码核对一致。三份文档的关键数据（驻留剖面 5N+1 goroutine 增量、洪水剖面 33.3MiB/端、`--max-clients` 兼任进程闸、双模式 stop-timeout 默认值分岔、herdr 配方 argv 与 phase14.mjs 被测物逐字一致）与测试/实测口径互证成立。编译级验证：`go vet ./internal/...` 与 `go test -count=1 -run ZZZNOMATCH`（含 `-tags load`）全绿，5 个 UAT mjs 全过 `node --check`；run-all 矩阵 17 项与 `web/uat/` 实际文件逐字核对无缺漏。

发现的问题集中在 **UAT 脚本的失败路径健壮性**（3 Warning）与探针脚本的细节卫生（5 Info）：一个 Linux 侧最小复现探针在失败时泄漏 wesh 孤儿进程与 herdr 会话且含无护栏死循环；pw 层 T0 断言硬编码远端日志路径，在自定义 `WESH_UAT_REMOTE_DIR` 的受支持配置下存在条件性假绿/假红面；phase14.mjs S2 的前提失败 early-return 恰好跳过残留核验 check。无 BLOCKER 级发现——无安全漏洞、无错误断言导致的假绿（主链路）、无产品行为回归。

## Critical Issues

（无）

## Warnings

### WR-01: 失败路径零资源收口 + 无护栏落定循环——泄漏 wesh 孤儿进程与 herdr 会话

**File:** `web/uat/minrepro-p14.mjs:52-98`（主流程无 try/finally）、`:56`（无护栏 do-while）、`:96-97`（kill 仅 happy path）
**Issue:** 两项独立缺陷：
(a) 主流程全部顶层 `await`（`startWesh` → `dialHello` → 观测循环）无 try/catch/finally 包裹。任何一处 reject（如 herdr 缺席导致 dialHello 超时、WS 握手失败）→ 未捕获 rejection → 进程非零退出，但 spawn 的 wesh 服务端子进程**不被杀死**（孤儿服务进程持续监听随机端口），且 herdr 会话 `wesh-uat-minrepro-<pid>` **永不 stop/delete**——herdr server 在 wesh 死后存活是特性（脚本生态内 phase14.mjs/minrepro-win.mjs 均按此语义显式清理），本探针违反同族 D-09 零污染纪律。
(b) 首帧落定循环 `do { b1 = stats.bytes; await sleep(150); b2 = stats.bytes; } while (b1 !== b2 || b1 === 0);` 无护栏上限——herdr 无输出（如会话连接异常静默）时脚本无限挂起；探针不在 run-all 矩阵内，10min 逐脚本护栏不覆盖，手动运行只能靠人工 kill。
**Fix:** 参照 `minrepro-win.mjs:39-95` 形态重构——主流程包 try/catch/finally，finally 依次 `ws.close()` → `kill()`（SIGTERM + 护栏 + SIGKILL 兜底）→ `herdr session stop/delete` + `session list` 核验；落定循环加护栏上限（`pollUntil(fn, guardMs)` 同款形态，护栏到期打印诊断并退出）。

### WR-02: T0 尾段硬编码 `/tmp/wesh-uat/server.log`，忽略受支持的 `WESH_UAT_REMOTE_DIR`——条件性假绿/假红面

**File:** `web/uat/pw/phase14-pw.mjs:366`
**Issue:** `const srvLog = await ssh("bash -lc 'cat /tmp/wesh-uat/server.log'")` 硬编码默认远端目录；而本脚本头注释（:58）明示 `WESH_UAT_REMOTE_DIR` 为受支持环境变量，`lib/server.mjs:8,57` 将 server.log 写入 `${REMOTE_DIR}`（且 `startWesh` 以 `>` 截断重建）。当操作者自定义该变量时：(a) 本 run 的日志在自定义路径，断言读取默认路径下的**陈旧日志**——若陈旧日志恰含 ≥3 个互异 pid 的 session_start（此前默认目录运行的残留），T0「三次 session_start 且 pid 两两不等」在未验证本 run 的情况下**假绿**；(b) 默认路径无文件时 `startPids.length === 0` → 假红。测试证据完整性缺陷（非产品缺陷）。
**Fix:** 从 `lib/server.mjs` export `REMOTE_DIR` 并插值：`bash -lc 'cat ${REMOTE_DIR}/server.log'`；或将该读取封装进 server.mjs 与 startWesh 同源。

### WR-03: S2 前提失败 early-return 跳过 S2g 残留核验——泄漏检测网恰在最需要处失效

**File:** `web/uat/phase14.mjs:455-457`（rw===null）、`:474`（ro===null）、`:495`（paneId===null）、`:534-535`（S2g）
**Issue:** `s2RoConvergence` 三处 `return` 位于 try 块内——finally 清理序列会执行，但 finally **之后**的 `check('S2g', ...)`（herdr session list 零残留核验）被跳过。三处 return 均发生在部分 attach/观测失败的路径上（前序 check 已 FAIL，无假绿），但这些恰是 herdr 会话残留风险最高的路径（半建立状态、异常中断），S2g 的设计目的（"清理核验——失败可见而非静默"）在失败路径上落空。对照 S1 无 early-return、S1j 恒执行。
**Fix:** 将 S2g 核验移入 finally 尾部（finally 内落 check，phase14.mjs 既有形态支持）；或以嵌套 `if (paneId !== null) { ... }` 替代三处 `return` 使控制流自然落到核验行。

## Info

### IN-01: session_start 提取正则耦合 slog JSON 键序

**File:** `web/uat/pw/phase14-pw.mjs:367`
**Issue:** `/"event":"session_start","pid":(\d+)/g` 依赖 logEvent 属性按 event→pid 顺序序列化。当前产品侧成立（14-09 实跑 4/4 绿），但属性序不是契约——未来 logEvent 增删/重排属性即静默失配（count=0 → T0 假红，fail-closed 方向，无假绿风险）。
**Fix:** 改用 phase14.mjs `parseEvents` 的 JSON 行解析形态后按 `m.event === 'session_start'` 过滤取 `m.pid`。

### IN-02: 探针死代码与误导性细节

**File:** `web/uat/minrepro-p14.mjs:11,22,63`
**Issue:** (a) `const CRED = 'user:pass'`（:11）从未使用——死代码；(b) `--insecure-http`（:22）在 `--bind 127.0.0.1` loopback 形态下冗余（该逃生门仅对非 loopback 有意义）；(c) INPUT 帧以 `new Uint8Array([OUTPUT])` 发送（:63，协议定义两方向同字节 0x30 故正确，但语义误导——建议独立 INPUT 常量或注释就地说明）。
**Fix:** 删除 CRED；去掉 `--insecure-http` 或注释其存在理由；发送处改用语义命名的常量。

### IN-03: spawn 'error' 路径未冲刷 pending 半行输出

**File:** `web/uat/run-all.mjs:115-118`
**Issue:** `attachForward` 的 pending 半行缓冲仅在 'close' 路径冲刷（:121-124）；'error'（如 `node` 不在 PATH 的 spawn 失败）路径直接 resolve，`flushOut()`/`flushErr()` 不被调用，子进程已产出的尾部输出丢失。发生在已失败的路径上，仅影响诊断信息完整性。
**Fix:** 'error' 分支同样调用 `flushOut()`/`flushErr()` 冲刷后再 resolve。

### IN-04: dialHello reject 路径未关闭 WebSocket 句柄

**File:** `web/uat/phase14.mjs:169-187`
**Issue:** watchdog 超时 / onclose reject 后未调用 `ws.close()`，连接句柄悬置到脚本末尾 `process.exit()` 强制退出才释放。当前被显式 process.exit 掩蔽；若该 helper 被复用于常驻进程或循环重试场景会累积句柄。
**Fix:** 各 reject 前补 `try { ws.close(); } catch {}`（undici close 同步无返回，phase14-pw.mjs:388 既有形态）。

### IN-05: 洪水量级陈旧注释与 README 新标定数字并存

**File:** `internal/server/load_test.go:240`
**Issue:** 注释「seq 1 N 总量 ≈ 33.8MB（N=4000000，含 ONLCR 膨胀）」算术不精确：digits+\n ≈ 30.9MB，ONLCR 膨胀后 ≈ 34.9MB（十进制）= **33.3MiB**——README 本期新回填的「33.3MiB/端」（README.md:136）才是准确值。该注释为基线 03268f0 前已存在文本（非本 phase 引入、仅重新缩进），但两处数字并存易误导后续标定对账与护栏上调判断。
**Fix:** 将注释订正为「≈ 34.9MB（= 33.3MiB）」与 README 对齐。

---

## 验证证据（评审过程中执行）

| 验证项 | 结果 |
|---|---|
| 产品代码零漂移：`git diff 03268f0^..HEAD -- internal/ cmd/ web/src/ web/embed.go` 剔除 `_test.go` | 空（NO_PRODUCT_CODE_CHANGES） |
| `go vet ./internal/...` | 通过 |
| `go test -count=1 -run ZZZNOMATCH ./internal/server/`（含 `-tags load` 变体）| 编译链接全绿 |
| 5 个 UAT mjs `node --check` | 全部通过 |
| run-all 矩阵 17 项 vs `web/uat/` 实际文件 | 逐字核对一致（辅助脚本 7 项按头注释明示排除） |
| 测试断言字面量 vs 产品源码（metrics.go HELP 双模式、health.go sessionActive、perclient.go capacityMessage、clients.go slow_consumer、proto.go Welcome.session） | 全部一致 |
| README/ARCHITECTURE/CONFIGURATION 关键数据互证（5N+1 goroutine、33.3MiB/端、6N 账面、herdr 配方 argv = phase14.mjs 被测物、30 配置键计数、5 个仅 CLI 键） | 成立 |
| shared 列期望值与基线等价性（`git show 03268f0` 抽查 startTestServer 基线 `{Writable: true}`、TestHelloWelcome/TestWelcomePrefs 原始 Options） | 转换基线忠实 |

_Reviewed: 2026-09-08T00:40:34Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
