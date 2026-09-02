---
phase: 03-auth
plan: 06
subsystem: testing
tags: [uat, node-websocket, basic-auth, tls, origin, hsts, testssl, readme]

# Dependency graph
requires:
  - phase: 03-auth (plans 03-01..03-05)
    provides: 凭据/subtle 比较（03-01）、节流+日志红线（03-02）、/api/attach+Origin 端点（03-03）、CLI flag+启动矩阵+TLS 分岔（03-04）、前端认证感知连接（03-05）
provides:
  - web/uat/phase03.mjs 六场景协议层自动化 UAT（18 断言全绿）
  - phase02.mjs D-03 适配（--bind 127.0.0.1，11/11 保持全过）
  - README 认证/TLS/Origin 文档 + D-03 行为变更双重明示 + 协议节同步
  - .planning/phases/03-auth/03-UAT.md 五项人工验证清单（status: pending-human）
  - 全量收口六段式验证记录（本文件「全量收口验证」节）
affects: [verify-work, ship, end-of-phase 人工确认]

# Actuals (#2632)
actuals:
  tokens: 10845    # chars/4 over realized diff（43383 chars，git diff 80c4020..623f85e）
  tasks: 2
  commits: 3       # task commits（68aa605/87f6e17/623f85e）+ 1 plan metadata commit

# Tech tracking
tech-stack:
  added: []        # 零新增依赖（RESEARCH §Package Legitimacy Audit 锁定）
  patterns:
    - "spawnExpectExit：拒绝路径 UAT 形态（spawn + exit 事件 + stderr 累积 + 3s 超时 SIGKILL）——不走 listening 行端口等待"
    - "节流爬梯 pacing：生产二进制无 throttle flag 覆写，失败断言间按 1s/2s/4s 真实 sleep 过窗"
    - "端口解析正则 scheme 感知（https?）——TLS 场景启动行打印 https://"
    - "UAT 输出红线：凭据/ticket 值只作协议构造材料，check detail 只打印状态码/布尔/枚举名"

key-files:
  created:
    - web/uat/phase03.mjs
    - .planning/phases/03-auth/03-UAT.md
  modified:
    - web/uat/phase02.mjs
    - README.md
    - internal/server/auth.go        # gofmt 纯排版清零
    - internal/server/auth_e2e_test.go
    - internal/server/e2e_test.go
    - internal/server/server.go

key-decisions:
  - "[Phase 03-06]: 场景 1 pacing 采用爬梯 sleep 形态（1.15s/2.15s/4.3s）而非每断言独立实例——同时证明退避窗口恢复语义（plan 备选形态裁决：优先爬梯）"
  - "[Phase 03-06]: 场景 3 /ws 无 Origin 断言取 400 形态（越过 Origin 闸撞子协议预检）——不建立 WS 连接不触发单次语义退出，比 101 形态干净"
  - "[Phase 03-06]: 非法 ticket 断言（S1f）用独立 spawn 实例——单次语义下同进程第二次 WS 建连不可行，新实例节流状态全新无 pacing 需求（plan 明示）"

patterns-established:
  - "拒绝路径端到端断言模式：spawnExpectExit 捕获 exit code + stderr 文案子串，拒绝文案进 wire 契约（S6a/S6b）"
  - "UAT 文档双层结构：自动化层覆盖表（场景→准则映射）+ 人工清单（why_human + auto_evidence 先行填入）"

requirements-completed: [SEC-01, SEC-02, SEC-03, SEC-04, SEC-05]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "认证完整链路：401 challenge → Basic → ticket（no-store）→ Hello → Welcome(mode=rw) → 非法 ticket auth_failed+1008（含节流爬梯过窗与无/错凭据同文）"
    requirement: SEC-02
    verification:
      - kind: e2e
        ref: "web/uat/phase03.mjs#scenarioAuthFlow（S1a..S1f）— 2026-08-17 全绿"
      - kind: integration
        ref: "internal/server/auth_e2e_test.go#TestAttachFlow/TestTicketInvalid — -race PASS"
    human_judgment: false
  - id: D2
    description: "爆破节流：8 次错凭据连发首 401 后续 429+Retry-After；subtle 常数时间比较；日志红线"
    requirement: SEC-03
    verification:
      - kind: e2e
        ref: "web/uat/phase03.mjs#scenarioThrottle（S2a）— 2026-08-17 全绿"
      - kind: integration
        ref: "internal/server/throttle_test.go#TestThrottle* + auth_test.go#TestCredentialMatch/TestLogRedaction — -race PASS"
    human_judgment: false
  - id: D3
    description: "Origin 白名单：/api/attach 邪恶 403/白名单 200；/ws 邪恶 403/无 Origin 放行（D-13）"
    requirement: SEC-04
    verification:
      - kind: e2e
        ref: "web/uat/phase03.mjs#scenarioOrigin（S3a..S3d）— 2026-08-17 全绿"
      - kind: integration
        ref: "internal/server/origin_test.go#TestOrigin* — -race PASS"
    human_judgment: false
  - id: D4
    description: "TLS 加固与安全头：wss 全链路 + HSTS(max-age=63072000)/nosniff/CSP；无认证模式 404 探测+直连；D-03/D-05 启动拒绝文案端到端"
    requirement: SEC-05
    verification:
      - kind: e2e
        ref: "web/uat/phase03.mjs#scenarioTLS/scenarioNoAuth/scenarioStartupMatrix（S4a..S6b）— 2026-08-17 全绿"
      - kind: integration
        ref: "internal/server/tls_test.go#TestTLS*/TestSecurityHeaders + cmd/wesh/main_test.go#TestStartupMatrix — -race PASS"
    human_judgment: false
  - id: D5
    description: "README 认证/TLS/Origin 文档与 D-03 行为变更双重明示（首屏警示区+用法节）+ 协议节同步"
    requirement: SEC-01
    verification:
      - kind: other
        ref: "grep 断言：--credential/--no-auth/auth_failed 命中 README；无 InsecureSkipVerify/关闭证书校验类内容；verify 命令链退出 0"
        status: pass
    human_judgment: false
  - id: D6
    description: "浏览器 Basic 弹窗一次输入后同源 fetch 自动带缓存凭据（A2 假设）；ticket 过期静默重试；429 面板；testssl.sh 无弱项；明文无 HSTS 复核"
    verification:
      - kind: manual_procedural
        ref: ".planning/phases/03-auth/03-UAT.md Tests 1-5 — status: pending-human（end-of-phase 确认）"
        status: unknown
    human_judgment: true
    rationale: "浏览器原生弹窗/凭据缓存（A2）与 testssl.sh 外部扫描为浏览器/工具行为，Go/Node 自动化不可驱动；must_haves 两项 backstop 按 honest-verifier 标记 human_needed 而非静默 pass"

# Metrics
duration: 2h 05m
completed: 2026-08-17
status: complete
---

# Phase 3 Plan 6: phase-closeout（UAT + 文档 + 全量收口）Summary

**Phase 3 收口：phase03.mjs 六场景协议 UAT（18/18 全绿）+ README 认证/TLS/行为变更明示 + 03-UAT.md 人工清单 + 六段式全量验证全绿，ROADMAP 三条成功准则证据链闭环。**

## Performance

- **Duration:** 2h 05m
- **Started:** 2026-08-17T09:48:59Z
- **Completed:** 2026-08-17T11:53:58Z
- **Tasks:** 2/2
- **Files modified:** 8（2 新建 + 6 修改）

## Accomplishments

- `web/uat/phase03.mjs` 新建：六场景 18 断言一次全绿——完整链路（401 challenge→同文 401→ticket+no-store→Welcome(rw)→auth_failed+1008）、节流 429+Retry-After、Origin 双端点 403/放行、无认证 404+直连、wss+HSTS/nosniff/CSP、D-03/D-05 拒绝文案 spawn-exit 断言；红线延伸到输出（grep 双脚本零凭据/ticket 命中）
- `web/uat/phase02.mjs` D-03 一行适配（--bind 127.0.0.1）后 11/11 保持全过，无协议语义漂移
- README：首屏「默认拒绝裸奔」+ 行为变更醒目段、六 flag 表、「认证与传输安全」小节（systemd EnvironmentFile 600/testssl.sh/HSTS 粘性）、协议节同步（Hello ticket/auth_failed//api/attach 契约）；无关闭证书校验教程（prohibition grep 零命中）
- `03-UAT.md` 新建：自动化层覆盖表 + 五项人工清单（Basic 弹窗缓存 A2 / ticket 过期静默重试 / 429 面板 / Origin 抽查 / testssl.sh+HSTS 粘性），status: pending-human 待 end-of-phase 确认
- 全量收口六段式全绿（逐条记录见下节）

## Task Commits

Each task was committed atomically:

1. **Task 1: phase03.mjs 六场景 + phase02.mjs D-03 适配** - `68aa605` (test)
2. **Task 2: README + 03-UAT.md + 六段式** - `623f85e` (docs)
3. **六段式段 1 gofmt 清零（deviation）** - `87f6e17` (style)

## Files Created/Modified

- `web/uat/phase03.mjs` - Phase 3 协议层自动化 UAT（六场景，零依赖 Node 原生 WebSocket/fetch）
- `web/uat/phase02.mjs` - startWesh 加 --bind 127.0.0.1（D-03 适配 + 注释）
- `README.md` - 认证/TLS/Origin 文档、行为变更明示、协议节同步
- `.planning/phases/03-auth/03-UAT.md` - 浏览器与 testssl.sh 人工验证清单
- `internal/server/{auth,server}.go`、`{auth_e2e,e2e}_test.go` - GOROOT gofmt 纯注释排版清零（零语义）

## Decisions Made

- 场景 1 pacing 采用爬梯 sleep（1.15s/2.15s/4.3s）优先于独立实例备选形态——爬梯同时证明退避窗口恢复语义（plan 明示优先采用）
- 场景 3 S3d 取 400 形态断言「无 Origin 非 403」——不建立 WS 连接、不触发单次语义退出，比 101/409 形态干净（plan 允许 400/101/409 任一）
- S1f 非法 ticket 用独立 spawn 实例（单会话约束下 wire 级二次建连不可行；新实例节流全新）——plan 明示形态落地

## 全量收口验证（六段式逐条记录）

| 段 | 命令 | 结果 |
|----|------|------|
| 1 | `"$(go env GOROOT)/bin/gofmt" -l .` | 初报 4 文件（纯注释排版）→ `-w` 清零后**输出为空** |
| 1 | `go vet ./...` | 退出 0 |
| 2 | `time go test -race -count=1 ./...` | 退出 0（real 7.7s；五包全 ok；Ticket/Auth/Throttle/Origin/Redact/TLS/StartupMatrix 组在列，77 个 RUN/PASS） |
| 3 | `pnpm -C web install --frozen-lockfile && pnpm -C web build` | 退出 0（build 190ms）；`test -f web/dist/index.html.gz` 成立 |
| 4 | `git archive HEAD → /tmp/wesh-clean && go build ./... && go test ./... -count=1` | 退出 0（对最终 HEAD 623f85e 复跑；五包全 ok）；目录已清理 |
| 5a | `/tmp/wesh-bin -- /bin/cat`（默认 bind 无凭据） | 退出 2 + stderr 含 "refusing to listen on non-loopback"（D-03 断言） |
| 5b | `--bind 127.0.0.1 --port 0 -- /bin/cat` | 单行 `listening on http://` + `curl /` 200 + 无子协议 `/ws` 400；kill 后无残留 |
| 5c | `WESH_CREDENTIAL=... --tls-cert/--tls-key`（自签夹具） | 单行 `listening on https://` + `curl -k /` 401 + `WWW-Authenticate: Basic realm="wesh"` + HSTS 头；kill 后 `pgrep -x wesh-bin` 无残留；夹具已清理 |
| 6 | 本 SUMMARY 逐条记录 | — |

Task 2 automated verify 命令链（vet + race 全量 + web build + gofmt 空 + 四 grep 断言）：**VERIFY_CHAIN_ALL_OK**。

phase gate（VALIDATION.md Sampling Rate）：全量 + 双 UAT（`node web/uat/phase03.mjs` 18/18、`node web/uat/phase02.mjs` 11/11）已执行并记录；testssl.sh 与浏览器五项按 D-07/human_verify_mode=end-of-phase 入 03-UAT.md 待确认。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] GOROOT gofmt 清零 4 个前序 plan 文件的注释排版差异**
- **Found during:** Task 2（六段式段 1）
- **Issue:** `"$(go env GOROOT)/bin/gofmt" -l .` 报告 auth.go/auth_e2e_test.go/e2e_test.go/server.go——03-01..03-05 期间积累的 gofmt 版本排版差异（行首 `//（` → `// （`、行尾注释对齐），阻塞段 1「输出为空」验收
- **Fix:** `gofmt -w` 清零（纯注释排版零语义），复跑 `go test -race -count=1 ./...` 全绿后单独 style 提交
- **Files modified:** internal/server/auth.go, auth_e2e_test.go, e2e_test.go, server.go
- **Verification:** gofmt -l 输出为空；-race 全量复跑五包全 ok；裸 clone 复跑绿
- **Committed in:** `87f6e17`（独立 style commit；02-06「proto.go 既存 gofmt 差异随段 1 授权分支清零」先例同形态）

---

**Total deviations:** 1 auto-fixed (Rule 3 - blocking)
**Impact on plan:** 纯排版清零，零语义改动，无 scope creep。

## Issues Encountered

- 冒烟 5c 首轮 shell 编排误伤：`pkill -f '/tmp/wesh-bin'` 匹配到包装 shell 自身命令行致其收 SIGTERM——WWW-Authenticate 头断言与残留确认补验一轮完成（`grep -ci` 命中 1、`pgrep -x` 无残留）；被测二进制行为无异常，非代码问题。

## User Setup Required

None - no external service configuration required.

## Known Stubs

None——本 plan 无 stub/占位实现。两项 must_haves backstop（testssl.sh 扫描、浏览器凭据缓存 A2）非静默 pass：已按 honest-verifier 落入 `03-UAT.md`（status: pending-human，五项人工清单）与 coverage D6（human_judgment: true），待 end-of-phase 用户确认。

## Next Phase Readiness

- 仓库处于可推送状态：gofmt（GOROOT）/vet/-race 全量/前端构建/裸 clone/双 UAT 全绿
- ROADMAP Phase 3 三条成功准则证据链闭环：准则 1（Go 集成 + UAT 场景 1）、准则 2（03-01/03-02/03-03 测试 + UAT 场景 2）、准则 3（03-02 TLS 矩阵 + UAT 场景 3/5）
- SEC-01..SEC-05 在 REQUIREMENTS.md 可追溯至具体测试/UAT 场景（本 SUMMARY coverage 块）
- **待用户确认**：03-UAT.md 五项人工清单（end-of-phase）；CI 远端运行由用户决定推送时机

---
*Phase: 03-auth*
*Completed: 2026-08-17*

## Self-Check: PASSED

- 文件全部存在：web/uat/phase03.mjs、web/uat/phase02.mjs、README.md、03-UAT.md、03-06-SUMMARY.md（5/5 FOUND）
- 提交全部存在：68aa605（test）、87f6e17（style）、623f85e（docs）（3/3 FOUND）
