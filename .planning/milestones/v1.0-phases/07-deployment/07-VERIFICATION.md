---
phase: 07-deployment
verified: 2026-08-27T02:30:00Z
status: passed
score: 55/55 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 46/46
  gaps_closed:
    - "G-07-2（major，OPS-02）：README nginx 配方补 proxy_set_header Host $http_host;（README.md:321）+ 精确块理据按 proxy_pass 301 实证改写 + pw 回归载具同步 5/5（07-09）"
    - "G-07-3（major，OPS-01）：listenSocket 活性探测——存活 socket 拒绝 EADDRINUSE 同形态文案 exit 1（main.go:1038-1042），静默赢者结构性消除（07-10）"
    - "G-07-8（minor，OPS-11）：openBrowser goroutine Wait 收割 + 非零退出 stderr 警告行（main.go:1280-1284，选项 A）（07-10）"
    - "human_verification 四项 discharge：A1 平台豁免风险接受 / A2 经 07-09 闭合 / B4 环境限制风险接受 / B6 经 07-10 闭合（07-UAT.md status: complete，1191efc）"
  gaps_remaining: []
  regressions: []
---

# Phase 7: 部署与配置 验证报告（复验）

**Phase Goal:** 真实运维场景可部署——监听形态齐全、配置文件落地、反代友好
**Verified:** 2026-08-27T02:30:00Z（HEAD 98db3a9，phase-07 分支，工作区净）
**Status:** passed
**Re-verification:** 是——gap closure（07-09/07-10）与人工 UAT discharge 落地后复验，替换 2026-08-26T09:05:00Z 陈旧报告

## Goal Achievement

### Observable Truths（ROADMAP 成功准则——契约层）

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 端口（0=随机并打印实际端口）/绑定地址/UNIX socket（含属主）可配置；TOML 配置文件支持，CLI 参数覆盖配置文件 | ✓ VERIFIED | 前验 46/46 基线（S1a-g/S2a-f + fileConfig 27 键 + listenSocket 序列）回归无回退；G-07-3 加固在码：listenSocket 活性探测（main.go:1038-1042，net.Dial unix 连通即拒、文案与 net.Listen EADDRINUSE 逐字全等）；复验新证据：TestListenSocket 六子测远端全 PASS（含新 live_instance_refused 子测）、b1b5.sh 7/7（B1a=exit1-eaddrinuse，b.log 首行实证）、B1c/B1d 残留清理不回归 |
| 2 | 反代子路径挂载（/wesh/ base-path）下页面与 WS 升级均正常（尾斜杠规范化）；反代注入的可信用户头记录进服务端审计日志（remote_user 审计归因——D-15 修订后文本） | ✓ VERIFIED | 前验基线（S3a-h/S4a-d）回归无回退（phase07.mjs 34/34 复跑）；G-07-2 闭合在文：README.md:321 `proxy_set_header Host $http_host;` + 三事实点必要性注释 + 精确块理据按 301 实证改写（节首校准「前缀块必需；精确块推荐」）；回归载具逐字镜像（a2-ctl.sh:35 $http_host、a2-pw.mjs T5=301+Location 尾斜杠）；双机全链回归 5/5（2026-08-27 实跑，截图 a2-home 10:06/a2-idle 10:07 重生）——跨机浏览器按文档部署不再即坏 |
| 3 | 子进程以指定 cwd/TERM 启动，停止信号发给进程组（可配 TERM→KILL 宽限）；可以指定 uid/gid 降权运行；可选启动后自动打开浏览器 | ✓ VERIFIED | 前验基线（S5/S6/S8）回归无回退；G-07-8 闭合在码：openBrowser Start 成功后 goroutine cmd.Wait() 收割（零僵尸）+ 非零退出 stderr 警告行（main.go:1280-1284，不含 URL——Wait err 结构性无 argv）；复验新证据：TestOpenBrowser 三子测远端全 PASS（含新 non-zero_opener_warns 子测，URL 占位串零命中反断言）、b6.sh 7/7（B6f 警告行 PASS、B6e/B6g 不阻断不回归、B6c https:// 链接 PASS） |

**Score:** 55/55 truths verified（3 契约 SC + 43 前验 plan truths 回归 + 07-09 四条 + 07-10 五条；0 项 behavior-unverified）

### Plan must-have Truths 逐条核验

| Plan | Truths | Status | 关键证据 |
|------|--------|--------|----------|
| 07-01（OPS-02） | 6/6 | ✓ VERIFIED（回归） | 前验证据链未受 gap closure 触碰（main.go 改动面为 listenSocket/openBrowser 两函数）；phase07.mjs 34/34 复跑含 S3 全链 |
| 07-02（OPS-01） | 5/5 | ✓ VERIFIED（回归+加固） | G-07-3 加固落同一函数：类型闸→活性探测两级收窄链注释在码（main.go:1010-1015,1029-1037）；stale/non-socket 等五既有子测零改动全 PASS（远端复跑） |
| 07-03（SEC-07） | 6/6 | ✓ VERIFIED（回归） | S4a-d 在 34/34 复跑内；proxy.go sanitize/ParseIP 闸未触碰 |
| 07-04（OPS-04/05） | 6/6 | ✓ VERIFIED（回归） | S5/S6 在 34/34 复跑内；spawn/signal 文件零改动（git diff e022a8b..98db3a9 仅 main.go/main_test.go/b6.sh/README/pw 载具 + 文档） |
| 07-05（OPS-11+D-23） | 5/5 | ✓ VERIFIED（回归+加固） | G-07-8 落同一函数：headless 跳过行（main.go:1266）与 Start 失败警告行（L1275）逐字未动（prohibition 核验 grep==1 各一）；S8a/S8b 断言面零漂移（34/34 复跑） |
| 07-06（OPS-09） | 6/6 | ✓ VERIFIED（回归） | config.go 零改动；S1a-g 在 34/34 复跑内 |
| 07-07（全需求 UAT） | 4/4 | ✓ VERIFIED（复跑） | 本验证独立复跑 phase07.mjs（当前 HEAD 树新构二进制）：34/34 PASS + 1 平台豁免 skip（S8c）+ SEC 自净 34 detail 零命中 |
| 07-08（收口文档） | 5/5 | ✓ VERIFIED（回归） | README 部署节在 07-09 修正后复核（配方块/理据/节首句）；REQUIREMENTS.md SEC-07 审计归因口径与 ROADMAP SC2 一致 |
| 07-09（G-07-2 闭合） | 4/4 | ✓ VERIFIED | README.md:321 Host 行 grep==1（location /wesh/ 块内）、旧「落不到反代块」理据 grep==0、proxy_read_timeout 3600s 保留、节首句校准在文；a2-ctl.sh conf() $http_host 镜像（L35）；a2-pw.mjs T5 status()===301（L85）+ Location 尾斜杠断言（L86）+ status()===404 grep==0；双机全链 5/5 实跑（2026-08-27，截图重生）；e4d46bb/7eae376/ddda3a7 提交在 git log |
| 07-10（G-07-3/G-07-8 闭合） | 5/5 | ✓ VERIFIED | net.Dial("unix" grep==1（main.go:1038）+ EADDRINUSE 文案 grep==1（L1041）；cmd.Wait() grep==1（L1281）+ 警告行文案 grep==1（L1282）；两新子测在码（main_test.go:927,1125）且远端 -race 全 PASS；b6.sh B6f 轮询化在码（L60 seq 1 50）；b1b5 7/7 + b6 7/7 本验证独立复跑；6dd552e/0fda7a0/f5659a7/f19be02/47ea9e0 提交在 git log |

### Required Artifacts（gap closure 新增/改动面；前验 15 件基线回归无回退）

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/wesh/main.go` | listenSocket 活性探测 + openBrowser goroutine Wait/警告行 | ✓ VERIFIED | 两段实现逐行核读（L1021-1060/L1264-1285）：探测-拒绝-清理三态完备、TOCTOU 两向安全降级注释在码、headless/Start 失败两既有路径逐字未动；非 stub——远端单测+二进制双通道行为实证 |
| `cmd/wesh/main_test.go` | TestListenSocket 第六子测 + TestOpenBrowser 第三子测 | ✓ VERIFIED | live_instance_refused（拒绝+文件仍在+仍连通三断言，sun_path 夹具纪律注释在码）、non-zero_opener_warns（os.Pipe 异步捕获 + URL 占位串零命中反断言）——远端复跑全 PASS |
| `web/uat/phase07-b6.sh` | B6f 警告行断言轮询化 | ✓ VERIFIED | L60 `for i in $(seq 1 50); do grep -qi "warn" ...` 在码；7/7 复跑实证 |
| `README.md` | Host 转发行 + 精确块理据 301 改写 + 节首校准 | ✓ VERIFIED | L311-321 逐行核读：三事实点注释（$proxy_host 不同源 403 / $host 剥端口仍不匹配 / 必须 $http_host）、308 保方法+handler 无关理据、缺省 301 为 proxy_pass 特例；四道静态验收闸全过 |
| `web/uat/pw/phase07-a2-ctl.sh` | conf() 镜像修正配方 | ✓ VERIFIED | L35 Host $http_host 在码（单引号 EOF 段 $ 字面安全）；exact/noexact 双 variant 共用 |
| `web/uat/pw/phase07-a2-pw.mjs` | T5 预期 301 校准 | ✓ VERIFIED | L41 Check 名、L80-87 断言体（resp301 + 301 等值 + Location 尾斜杠 + /wesh/ 200）在码；404 断言零残留 |
| `.planning/phases/07-deployment/07-09-SUMMARY.md` | 追溯补档（out-of-band 执行） | ✓ VERIFIED | 165 行完整 frontmatter+coverage+偏差登记；与 git log 四提交互证 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| listenSocket Lstat 类型闸 | net.Dial("unix") 活性探测 | CR-01 类型收窄后 G-07-3 活性再分 | ✓ WIRED | main.go:1025-1043 逐行核读：ModeSocket 闸→Dial→连通拒绝/失败落 Remove；b1b5 B1a/B1c/B1d 三行为二进制实证 |
| listenSocket 拒绝 error | run() listen 失败通道 → exit 1 | 与 net.Listen 失败同档 | ✓ WIRED | b.log 首行实证 `wesh: listen unix ...: bind: address already in use` + 进程 exit 1（b1b5 B1a 断言 rc==1） |
| openBrowser Start 成功分支 | goroutine cmd.Wait() → stderr 警告行 | D-27 运行期非零覆盖 | ✓ WIRED | main.go:1280-1284；TestOpenBrowser 新子测 2s 轮询观测到警告行（远端 PASS）+ b6 B6f PASS |
| README nginx 配方 | wesh originAllowed 同源校验（origin.go:73） | proxy_set_header Host $http_host | ✓ WIRED | 配方行在文（README.md:321）；全链实证 5/5（T3 echo 经 WS 升级成功——同源校验放行的用户可观测等价物） |
| a2-ctl.sh conf() | README 配方 | 回归载具逐字镜像（文档即被测物） | ✓ WIRED | 两文件块结构逐行比对一致（Host/Upgrade/Connection/proxy_read_timeout 四行+精确块 variant） |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| listenSocket 拒绝决策 | net.Dial 返回值 | 真实 unix socket 建连（内核） | ✓（B1a 存活拒绝 / B1d 残留清理双向实证） | ✓ FLOWING |
| openBrowser 警告行 | cmd.Wait() 返回 err | opener 子进程真实退出码 | ✓（b6 B6f stub exit 1 触发警告、exit 0 零输出） | ✓ FLOWING |
| pw T3 echo 断言 | 终端回读标记 A2WS_\* | 浏览器→nginx→wesh→bash 全链 | ✓（5/5 实跑，标记含随机后缀非硬编码） | ✓ FLOWING |

### Behavioral Spot-Checks（本验证独立执行——Linux 9.134.229.124，仓库 /data1/home/zexueli/open_src/wesh @ ddda3a7，与本地 98db3a9 的 Go 代码零差异：git diff ddda3a7..98db3a9 仅三件 .planning 文档）

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| G-07-3/G-07-8 两新单测 + 邻域子测 | `go test ./cmd/wesh -run "TestListenSocket\|TestOpenBrowser" -count=1 -v` | TestListenSocket 六子测全 PASS（含 live_instance_refused）、TestOpenBrowser 三子测全 PASS（含 non-zero_opener_warns） | ✓ PASS |
| G-07-3 二进制直证 | 当前树新构二进制后 `bash web/uat/phase07-b1b5.sh` | 7 PASS, 0 FAIL——B1a=exit1-eaddrinuse（b.log 首行实证）、B1c/B1d 残留链不回归、B5 四项顺带 | ✓ PASS |
| G-07-8 二进制直证 | `bash web/uat/phase07-b6.sh` | 7 PASS, 0 FAIL——B6f 警告行在、B6c https:// 链接、B6e/B6g 不阻断 | ✓ PASS |
| 协议层零漂移（S2/S8 直触改动面） | `node web/uat/phase07.mjs` | 34/34 PASS + 1 平台豁免 skip（S8c）+ SEC 自净 34 detail 零命中 | ✓ PASS |
| 静态检查 | `go vet ./...` | VET_OK（退出 0） | ✓ PASS |
| 全仓测试（含竞态，单次全量） | `go test -race -count=1 ./...` | 五包全 ok 53s（cmd 1.1s/proto 1.0s/pty 2.7s/server 50.4s/web 1.0s） | ✓ PASS |
| G-07-2 双机全链回归 | `node web/uat/pw/phase07-a2-pw.mjs`（2026-08-27 已实跑——本验证核验证据链） | 5/5（T1 308/T2 页面+提示符/T3 echo/T4 空闲 65s/T5 301+/wesh/ 200）；截图 a2-home 10:06、a2-idle 10:07 当日时间戳在盘；载具与配方镜像关系静态核验 | ✓ PASS（证据核验） |

### Probe Execution

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| 无 scripts/\*/tests/probe-\*.sh 约定探针 | — | 本 phase 验证面为 web/uat/\* 真实二进制 UAT（b1b5/b6/phase07.mjs 已如上独立复跑） | N/A |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| OPS-01 | 07-02/07-07/07-08/07-10 | 监听配置：端口/绑定/UNIX socket（含属主） | ✓ SATISFIED | 前验基线 + G-07-3 活性探测（存活拒绝 EADDRINUSE 设计答案达成） |
| OPS-02 | 07-01/07-07/07-08/07-09 | 反代子路径挂载（base-path） | ✓ SATISFIED | 前验基线 + G-07-2 配方修正（跨机浏览器按文档部署全链 5/5 实证） |
| OPS-04 | 07-04/07-07/07-08 | 子进程 cwd/TERM/关闭信号可配（信号发进程组） | ✓ SATISFIED | 前验基线回归无回退 |
| OPS-05 | 07-04/07-07/07-08 | 降权运行（setuid/setgid） | ✓ SATISFIED | 前验基线回归无回退（self 面 S6 自动化；root→nobody 残余 UAT 风险接受） |
| OPS-09 | 07-06/07-07/07-08 | 配置文件支持，CLI 参数覆盖配置文件 | ✓ SATISFIED | 前验基线回归无回退 |
| OPS-11 | 07-05/07-07/07-08/07-10 | 可选启动后自动打开浏览器 | ✓ SATISFIED | 前验基线 + G-07-8 选项 A（非零警告+僵尸收割，D-27 字面从实现侧闭合） |
| SEC-07 | 07-03/07-07/07-08 | auth-header 透传（D-15 修订：服务端审计归因） | ✓ SATISFIED | 前验基线回归无回退 |

孤儿需求检查：REQUIREMENTS.md Traceability 表 Phase 7 映射恰为 OPS-01/02/04/05/09/11 + SEC-07 七条（全部 Complete），gap closure 两 plan 声明的 OPS-01/02/11 均在映射内，无孤儿。

### Prohibitions 核验（07-09/07-10 plan must_haves.prohibitions）

| Prohibition | Tier | Status | Evidence |
|-------------|------|--------|----------|
| 07-09：凭据/token 永不入 pw detail/控制台输出 | judgment（resolved） | ✓ 保持 | pw 脚本逐行核读：Check.ok detail 仅状态码/布尔/常量文案（L53-86）；SUMMARY 红线声明与脚本结构互证 |
| 07-10：分享 URL（含 token）永不入 stderr 警告行 | judgment（resolved，测试运行时强化） | ✓ 保持 | 结构性达成（Wait err 仅 exit status N 无 argv）+ TestOpenBrowser 新子测 URL 占位串零命中反断言（远端 PASS） |
| 07-10：headless 跳过行/Start 失败警告行/listening 打印字面零漂移 | judgment（resolved） | ✓ 保持 | 三字面 grep 各 ==1 原位（main.go:1266/1275）；phase07.mjs S8a/S8b 断言面 34/34 复跑零漂移 |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| 无 | — | gap closure 六改动文件 TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER 扫描零命中（b6.sh:8 mktemp 模板 `b6.XXXXXX` 为伪命中）；空实现/硬编码空数据扫描零命中 | — | — |

既有事项（非本 phase 引入，已登记路由不变）：internal/server/multi_test.go 与 slowclient_test.go 两 HEAD 既有 GOROOT gofmt 漂移（deferred-items.md 登记 open，style 提交清零路由）。

### Human Verification Required

无残余。前验四项人工条目全部经 07-UAT.md（status: complete，1191efc）discharge，逐条核验如下：

1. **A1 真实浏览器 --open 弹窗** → skipped（平台豁免·风险接受，2026-08-27 用户裁决）——CODEBUDDY.md 测试策略第 5 条既定豁免；协议层等价物 S8a/S8b 自动化覆盖且本次 34/34 复跑在绿。discharge 依据成立。
2. **A2 真实 nginx 反代观感** → issue → RESOLVED（G-07-2，07-09）——修正配方双机全链 5/5 当日实跑，截图重生。discharge 依据成立。
3. **B4 root 降权 nobody** → skipped（环境限制·风险接受，2026-08-27 用户裁决）——sudo 损坏无提权通道；可自动化面（降权 self 全链+身份改写）已由 S6a/b 覆盖。discharge 依据成立。
4. **B6 macOS open 与 TLS 组合** → issue → RESOLVED（G-07-8，07-10 选项 A）——警告行/僵尸收割/TLS https:// 链接经 b6.sh 7/7 二进制直证；macOS 真实弹窗面平台豁免。discharge 依据成立。
5. **B1/B2/B3/B5 flagged assumptions** → B1 RESOLVED（G-07-3，b1b5 B1a 转 exit1-eaddrinuse）；B2/B3/B5 已于 2026-08-26 pass。discharge 依据成立。

### Gaps Summary

无 gap。三项 UAT gap（G-07-2/3/8）全部经「静态符号 + 单测 + 二进制直证 +（G-07-2）双机全链」四层核验闭合，本验证对 Go 侧三层均独立复跑取证（非转述 SUMMARY）；前验 46/46 基线经改动面分析（e022a8b..98db3a9 代码增量仅 07-09/07-10 两 plan 范围）+ 全量 -race + 协议套件复跑确认零回退；四项人工条目 discharge 依据逐条核验成立。status: passed。

**信息级登记（不判 gap，orchestrator 收口时处理）**：① ROADMAP.md L312 07-09 复选框未勾、「Plans: 9/10」与进度表未反映 07-09 完成（07-09-SUMMARY 为 2026-08-27 追溯补档所致簿记滞后）；② 07-UAT.md A2/B1/B6 条目头复选框仍为 `- [ ]` issue 字样，权威状态以同文件 Gaps 节 status: resolved + Summary 计数为准；③ STATE.md「Plan: 2 of 10」为陈旧游标。三者均为文档簿记，不触代码面。

---

_Verified: 2026-08-27T02:30:00Z_
_Verifier: CodeBuddy (gsd-verifier)_
