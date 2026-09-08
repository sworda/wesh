---
phase: 14-herdr-uat
reviewed: 2026-09-08T06:39:38Z
depth: standard
files_reviewed: 38
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
  - scripts/check-mermaid.mjs
  - scripts/package.json
  - scripts/pnpm-lock.yaml
  - web/uat/minrepro-p14.mjs
  - web/uat/phase14.mjs
  - web/uat/pw/minrepro-win.mjs
  - web/uat/pw/phase14-pw.mjs
  - web/uat/run-all.mjs
findings:
  critical: 0
  warning: 6
  info: 7
  total: 13
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-09-08T06:39:38Z
**Depth:** standard
**Files Reviewed:** 38
**Status:** issues_found

## Summary

Phase 14 是 verification-matrix 阶段：26 个 `_test.go` 文件经 `harness_test.go` 的 `newTestServer` 小族迁移为双模式 `t.Run` 参数化、load_test.go 增加 per-client 负载矩阵、新增 `scripts/check-mermaid.mjs` 词法校验载具（14-13）、web/uat 新增 phase14 协议层与 Playwright 观感层脚本、README/ARCHITECTURE/CONFIGURATION 文档回填标定数据。产品代码零改动（测试断言既有行为）。本报告覆盖完整 38 文件 diff（base `03268f0^`），取代早前 35 文件范围的旧版报告。

**验证证据（本次评审实测）**：
- `go vet ./internal/server/` 与 `go vet -tags=load ./internal/server/` 均零告警——26 个测试文件 + harness + load 削面全部编译通过，跨文件符号引用（`startPerClientServer` 三变体、`readPump`/`frameRes`/`drainQuiet`/`accumFramesUntil`/`waitPgroupESRCH`/`readSessionPid`/`eventsNamed`/`parseEvents`/`countByEvent`/`healthzClients` 等）完整无缺失；已删除的 `startShutdownServerWith`/旧 resize_arb 本地 helper 无残留引用。
- `node scripts/check-mermaid.mjs` 实跑：docs/ 全部 3 个 mermaid 块 PASS，退出码 0。
- harness 双模式基线对称性核实：shared 分支 `server.Options{Writable: true}` 与 per-client 母本（perclient_test.go:77/135）同基线，mutate 语义两列同构。
- 节流类测试的 pacing 算术逐测复核（TestAttachFlow/TestThrottleHTTP/TestMetricsAuth/TestMetricsValues/TestOriginEndpoints/sharetoken 失败流殿后）——退避级数 `base<<min(fails-1,5)` 与各 sleep 窗口全部覆盖，无 429 误伤路径。
- minrepro/phase14 脚本以 OUTPUT 字节（0x30）发送 INPUT 帧经 proto.go 核实为协议定义（Input 与 Output 同为 '0'），非缺陷。

**总体评估**：迁移机械（小族直传母本、断言分叉表、Cleanup 纪律）执行质量高，未发现 BLOCKER 级问题——无安全漏洞（token/pid 红线在运行时自净断言中兑现）、无会导致核心断言假绿的门禁性缺陷、无产品行为回归面。发现的问题集中在三类：**用户可见文档的数据主体/契约准确性**（WR-01/WR-02）、**pw 层观感断言的假绿面与陈旧注释**（WR-03/WR-04）、**工具与自检脚本的健壮性**（WR-05/WR-06）。

## Critical Issues

（无——本阶段未发现 Critical 级问题。）

## Warnings

### WR-01: README 标定数据主体错标——"bash" 实测夹具为 `sh`

**File:** `README.md:127-152`（对照 `internal/server/load_test.go:1051`）
**Issue:** README「per-client 资源义务与实测标定」节的驻留剖面表头写「N 会话 **bash** idle 空转」，`--max-clients` 建议值表写「按实测每会话 **bash** 子进程 ~3.7MiB …折算」；但数据源 `TestLoadPerClientResident` 的夹具是 `defaultPCSpawnFn([]string{"sh"})`（`TestChurnPerClientSpawnThrottle` 同为 `"sh"`）。实测主体与文档声明不一致：在 /bin/sh → dash 的发行版上真实 bash 会话的每会话 RSS 通常显著高于 dash，低配 VPS 分档建议（8 = 按 ~3.7MiB/会话折算）会低估内存义务。洪水剖面（gatedFloodArgv 经 `bash -c`）不受影响，仅驻留剖面错标。
**Fix:** 二选一：(a) 把 `TestLoadPerClientResident` 的 spawn argv 改为 `[]string{"bash"}` 后重新标定回填；(b) README 两处「bash」改为「sh（/bin/sh）」并在建议值表注明「bash 等重 shell 按实测另行上浮」。文档声称「文档即被测物」纪律时，测量主体必须与被测物一致。

### WR-02: CONFIGURATION.md 退出码契约缺口——per-client SIGTERM 关停存在 exit 0 路径，文档仅记 255

**File:** `docs/CONFIGURATION.md:142`（对照 `internal/server/health_test.go:347-351`、`internal/server/shutdown_test.go:429`）
**Issue:** CONFIGURATION.md 退出码表断言「SIGTERM 优雅下线 → 255（systemd `Restart=on-failure` 视其为失败并自愈重启）」。但本阶段双跑测试锁定的既有行为存在两条 exit-0 路径：① `TestHealthzDraining` per-client 列断言零会话形态 Shutdown → `exitf(0)`（last-reaped-code 缺省 0）——真实二进制即「per-client wesh 在任何客户端 attach 前收到 SIGTERM → 退出码 0」；② `TestPerClientShutdownDeadlineExits` 断言 stopTimeout=0 + 免疫子进程的 join 到期收口同为 `exitf(0)`。运维方按文档表写 systemd `Restart=on-failure` 时，路径①会把「被 SIGTERM 停掉的 per-client 实例」判为成功而不重启——与 shared 模式（恒 255）行为分叉且无文档。产品行为非本阶段改动，但文档（本阶段在审）未覆盖该分叉。
**Fix:** 在 CONFIGURATION.md 退出码表 255 行追加 per-client 例外注记，例如：「`per-client` 模式下 SIGTERM 关停若无任何已收割会话（零 attach 即停 / stop-timeout=0 且子进程免疫）→ 退出码 0」；或在文档层面声明该路径为已知行为分叉并给出 systemd 编写建议（`Restart=always`）。

### WR-03: phase14-pw.mjs 硬编码 `/tmp/wesh-uat/server.log`，与可覆写的 `WESH_UAT_REMOTE_DIR` 脱钩

**File:** `web/uat/pw/phase14-pw.mjs:366`（对照 `web/uat/pw/lib/server.mjs:8,57`）
**Issue:** 头注释声明支持 `WESH_UAT_REMOTE_DIR` 环境变量覆写远端目录，`lib/server.mjs` 的 `startWesh` 也按 `${REMOTE_DIR}/server.log` 重定向；但 T0 尾段的进程证明读取硬编码 `cat /tmp/wesh-uat/server.log`。当操作者覆写 `WESH_UAT_REMOTE_DIR` 时：(a) 读不到本轮日志 → `session_start` 解析为 0 → 假红；(b) 更危险的是若默认目录存在**上一轮**的陈旧 server.log（含 3 条 pid 两两不等的 session_start），`startPids.length === 3 && distinctPids.size === 3` 会**凭陈旧日志假绿**——per-client 模式自证（T0 核心）被旁路。
**Fix:** 从 `lib/server.mjs` 导出 `REMOTE_DIR`（或新增 `serverLogPath()` 访问器），phase14-pw.mjs 改为 `` ssh(`bash -lc 'cat ${REMOTE_DIR}/server.log'`) ``；顺带可在 `startWesh` 重定向前 `rm -f` 旧日志以结构性消除陈旧日志面。

### WR-04: phase14-pw.mjs 头注释宣称的「stty 标记假绿防线」已被移除——注释误导 + T1 存在空转通过洞

**File:** `web/uat/pw/phase14-pw.mjs:35-37`（对照同文件 `:361-365`）
**Issue:** 文件头「判别面」节声称「假绿防线：移动端断言前必须等其 buffer 渲染出驱动端键入的 stty 标记（herdr 全量首帧 = pane 内容共享证据）」，但 14-09 重设计后驱动端保持静默、键入全部延后（T0 尾段注释自认「A5 stty 材料移除——几何断言由协议层 phase14.mjs 承载」）——**代码中不存在该防线**。后果：T1 的 `restored1` 轮询条件（结构行 ≥10）与基线判定同值，若桌面 tab 在移动 attach 后完全收不到增量帧（输出路由断裂类缺陷），桌面 DOM 停留在基线快照 → `diff1 === 0` 空转通过。该洞目前仅由另一载具（phase14.mjs S1c 增量断言）跨脚本兜底，本文件头注释却声称防线在场，误导后续维护者。
**Fix:** 修正头注释：删除「假绿防线：…stty 标记」句，改为如实声明「T1 的增量流动前提由 phase14.mjs S1c 承载（跨载具分层）」；如需本载具自足，可在 T1 前对桌面 tab 做一次轻量活性探针（如驱动端键入一个空格后轮询桌面 DOM 任意变化）。

### WR-05: phase14.mjs `assertOutputClean` 的 pid 泄漏自检为无锚定子串匹配——与数字型 detail 碰撞即假红

**File:** `web/uat/phase14.mjs:542-549`
**Issue:** SEC 自净断言对全部 detail 做 `d.includes(String(p))`（p 为 session pid 数值）。detail 含大量裸数字：`status=200`、`增量=${inc1}B`、`全量=${d0}B`、`maxCol=40/76`、`新几何=...`。当任一会话 pid 数值与其中某个数字子串碰撞（容器环境 pid 常为 2-3 位数，与 `inc1`/`maxCol`/`200` 碰撞概率实际存在；开发机上 5-6 位 pid 与 `d0` 字节数碰撞为低概率事件）→ SEC 误报 FAIL，且失败信息只打 `命中=true` 无命中物指认，排障成本高。这是假红方向的缺陷（不产生假绿），但随 run-all 矩阵化后是长期 flake 源。
**Fix:** pid 匹配加数字边界锚定，例如：
```js
const pidHit = sensitivePids.some((p) =>
  new RegExp(`(^|[^0-9])${p}([^0-9]|$)`).test(d));
```
（token/标记串为高熵串可维持 includes；pid 必须边界化。）

### WR-06: check-mermaid.mjs 块正则不兼容 CRLF——CRLF 文档静默输出 "no mermaid blocks" 假 PASS

**File:** `scripts/check-mermaid.mjs:41`
**Issue:** `MERMAID_BLOCK = /```mermaid\n([\s\S]*?)```/g` 对 `\n` 硬编码。若 docs 下任一 `.md` 被以 CRLF 行尾保存（Windows 工作站编辑后提交是本仓库双机拓扑的现实路径），`"```mermaid\r\n"` 不匹配 → 该文件所有块被静默跳过（`no mermaid blocks, skipped`，退出码 0）。对一个以「G-14-34 防复发件」为使命的校验载具，静默跳过即假 PASS——正是它要防的漏检形态。当前仓库全 LF 故实跑 PASS，属潜伏失效。
**Fix:**
```js
const MERMAID_BLOCK = /```mermaid\r?\n([\s\S]*?)```/g;
```
顺带建议把开栏锚定到行首（`/(^|\n)```mermaid\r?\n/`）以排除行中误匹配。

## Info

### IN-01: 洪水量级注释不一致——load_test.go "≈ 33.8MB" vs README "33.3MiB/端"

**File:** `internal/server/load_test.go:254-255`（对照 `README.md:136`）
**Issue:** `loadFloodLast` 注释称「seq 1 N 总量 ≈ 33.8MB（N=4000000，含 ONLCR 膨胀）」；实际计算 26,891,896 数字位 + 4M 换行 + 4M ONLCR 膨胀 = 34,891,896 B ≈ 33.28 MiB。README 的「33.3MiB/端」准确，load_test 注释的 33.8MB 既非 MiB 也非 MB 口径。
**Fix:** 注释改为「≈ 33.3MiB（≈ 34.9MB）」与 README 对齐。

### IN-02: harness_test.go 注释行号漂移

**File:** `internal/server/harness_test.go:28-29,44`
**Issue:** 「killServer e2e_test.go:120-134」实际位于 e2e_test.go:193-199；「perclient_test.go:116-120」（默认闭包）实际位于 :180-184。本项目重度依赖注释行号交叉引用，漂移会误导后续按图索骥。
**Fix:** 更新为现行号或改为函数名引用（`killServer`/`startPerClientServer` 默认闭包）去行号化。

### IN-03: ARCHITECTURE.md 目录结构未登记 14-13 新增的 scripts 工具链

**File:** `docs/ARCHITECTURE.md:179`
**Issue:** 目录结构注释 `scripts/ # release.sh 发布脚本…` 未提及本阶段新增的 `check-mermaid.mjs` + `package.json`（mermaid 词法校验载具）。ARCHITECTURE 本阶段刚被修改过（mermaid 修复），目录树却未同步。
**Fix:** 补记：`scripts/  # release.sh 发布脚本 + check-mermaid.mjs 文档 mermaid 词法校验（jsdom+mermaid，dev-tool）`。

### IN-04: minrepro-p14.mjs `CRED` 常量未使用（死代码）

**File:** `web/uat/minrepro-p14.mjs:11`
**Issue:** `const CRED = 'user:pass'` 定义后全程未引用（该探针 spawn 参数无 --credential）。对照 minrepro-win.mjs 的 A/B 开关形态，属残留。
**Fix:** 删除该行，或注释标记「保留供 ticket 模式扩展」。

### IN-05: TestMaxClients503 kick 子测试的 darwin 守卫成死代码

**File:** `internal/server/multi_test.go:1452,1555`
**Issue:** 子测试顶部 `if runtime.GOOS == "darwin" { t.Skip(...) }` 已整段跳过 darwin，后续 `if runtime.GOOS != "darwin"` 的 12MiB 等待守卫在可达路径上恒真。两层防御无害但后者已是死分支，暗示守卫序列经过演化未清理。
**Fix:** 删除内层 `runtime.GOOS != "darwin"` 条件包裹（保留循环体），或加注释说明为防御性冗余。

### IN-06: TestGlobalCredit 接合点后缀容差理论上弱化不连续断言

**File:** `internal/server/slowclient_test.go:414-424`
**Issue:** 行中切面容差判别（fields[0] 为 second-1 的严格尾缀即从 fields[1] 起续链）在理论上可放过「真丢帧且首字段恰为 second-1 尾缀」的形态（如 fields[0]="23"、second=124）。实践上 c2 中途接入使首字段恒为 6 位数、洪水区间约束使误放形态不可达，且注释已完整论证——仅作登记，不要求改动。
**Fix:** （可选）在容差分支追加帧跨度上界检查（如 `second-first ≤ 2` 才容差）进一步收紧。

### IN-07: phase14.mjs `tokenFromUrl` 对非 /s/ 形态链接的空指针

**File:** `web/uat/phase14.mjs:116,445`
**Issue:** `tokenFromUrl` 在正则不匹配时对 `exec()` 返回值直接取 `[1]` → TypeError。S2 消费 `tokenFromUrl(inst.shareRW)`：startWesh 的 50ms 落定窗若漏收 rw 行（管道分块边界），shareRW 为 null → 场景以 "Cannot read properties of null" 异常收场。失败可见、无敏感值泄漏、概率低——防御性缺口而非功能缺陷。
**Fix:** `tokenFromUrl` 改为返回 null 并在消费点显式判空落 check FAIL（`ticket非空=false` 通道已有），避免以 TypeError 形态终结。

---

## 评审过程记录

- **scope**：38 个文件全量读取；`scripts/pnpm-lock.yaml` 为生成物（锁文件），按规则不作逐行发现项，仅核对 importers 与 package.json 一致（jsdom ^30.0.1 / mermaid ^11.17.2 ✓）。
- **深度**：standard + 定向跨文件追踪——harness 小族 ↔ perclient/e2e 母本的装配等价性、被引用 helper 的存在性（grep 全命中）、双模式 Writable 基线对称（perclient_test.go:77/135 均为 true）、节流 pacing 算术、minrepro 的 0x30 双向字节与 proto.go 对齐、phase14-pw 对 lib/server.mjs REMOTE_DIR 的耦合。
- **实测验证**：`go vet`（含 `-tags=load`）双零告警；`node scripts/check-mermaid.mjs` 全 PASS。
- **红线审计**：share token/ticket/pid 的输出自净纪律在 Go 测试与 UAT 脚本两侧均落实（断言消息只含状态码/布尔/形状）；`redactArgs` 脱敏、`sensitiveTokens/sensitivePids/sensitiveMarkers` 闭包管理形态正确（WR-05 仅涉及其中的匹配算法）。

_Reviewed: 2026-09-08T06:39:38Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
