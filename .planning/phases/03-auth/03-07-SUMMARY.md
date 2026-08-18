---
phase: 03-auth
plan: 07
subsystem: auth
tags: [tls, startup, resource-leak, docs, tdd]

requires:
  - phase: 03-auth
    provides: 03-02 server.TLSConfig 声明式下限组件；03-04 启动校验矩阵/ServeTLS 分岔/captureFd 测试纪律
provides:
  - G-03-5 根因①闭合——TLS 证书启动预检（pty.Start/net.Listen/listening 打印三者之前），坏证书零资源占用 exit 1
  - G-03-5 根因②闭合——Serve/ServeTLS 共享错误路径 _ = sess.Close() 与 listen 失败路径逐字对称，无 pty 孤儿
  - G-03-5 missing 第 3 条闭合——03-VERIFICATION.md/03-UAT.md 交互复现命令全部带 --writable
  - TestBadCertPreflight 两场景回归锁（print-then-die + 预检先于 listen 顺序锁）
affects: [verify-work, ship]

actuals:
  tokens: 2212
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - 启动预检零资源占用：运行时资源（TLS 证书）可读性验证置于 spawn/listen/listening 打印之前，失败即 exit 1
    - 证书加载单一事实源：tls.LoadX509KeyPair 预检 → hs.TLSConfig.Certificates 复用 → ServeTLS(ln, "", "") 空串约定
    - 启动失败回滚对称纪律：spawn 后任何失败路径必须 _ = sess.Close()，与既有 listen 路径逐字对照

key-files:
  created:
    - .planning/phases/03-auth/03-07-SUMMARY.md
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - .planning/phases/03-auth/03-VERIFICATION.md
    - .planning/phases/03-auth/03-UAT.md

key-decisions:
  - "坏证书预检 exit 1（运行时 I/O 档位，与 pty.Start/net.Listen 失败同档），区别于 validateStartup 的 exit 2 配置矩阵错误"
  - "ServeTLS(ln, \"\", \"\") + TLSConfig.Certificates 预加载复用——证书加载单一事实源，杜绝二次磁盘读取"
  - "README systemd 示例判定不致误解（:29 flag 表 + :113 默认只读节已明示 ro 姿态），零改动且不登记 deferred-items"

patterns-established:
  - "预检前置模式：运行时 I/O 资源在副作用（spawn/listen/打印）之前完成验证，拒绝路径零资源占用"
  - "回滚逐字对称锁定：无故障注入手段的路径以与可测路径逐字对称 + 代码评审锁定，并在测试注释中写明推理"

requirements-completed: [SEC-05, SEC-01]

coverage:
  - id: D1
    description: "坏 --tls-cert/--tls-key 路径下 wesh 在零资源占用阶段（无 spawn/无 listen/无 listening 打印）以 exit 1 报错退出，stderr 含证书路径"
    requirement: SEC-05
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestBadCertPreflight（free_port_no_print-then-die + occupied_port_preflight_precedes_listen）"
        status: pass
      - kind: other
        ref: "go test ./... -race -count=1（五包全 ok）；go vet ./cmd/wesh 零告警；GOROOT gofmt -l cmd/wesh 零输出"
        status: pass
    human_judgment: false
  - id: D2
    description: "net.Listen 与 Serve/ServeTLS 任一启动失败路径均回滚 sess.Close()，不留 pty 子进程孤儿"
    requirement: SEC-05
    verification:
      - kind: other
        ref: "代码评审：serve 错误路径 _ = sess.Close() 与 main.go listen 失败路径（:178-181 原位）逐字对称；TestBadCertPreflight 注释写明无注入手段推理"
        status: pass
    human_judgment: true
    rationale: "Serve 阻塞语义 + lifecycle os.Exit 不可在单测驱动故障注入（plan 既定裁决），该交付物以逐字对称 + 人工代码评审锁定"
  - id: D3
    description: "03-VERIFICATION.md（2 处）与 03-UAT.md Test 5（1 处）交互复现命令全部带 --writable，README.md 零改动"
    requirement: SEC-01
    verification:
      - kind: other
        ref: "grep 计数门：VERIFICATION '--credential user:pass --writable -- bash' = 2；UAT 'key.pem --writable' = 1；git diff --quiet README.md"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-08-18
status: complete
---

# Phase 03 Plan 07: G-03-5 Gap Closure Summary

**TLS 坏证书启动预检（print-then-die 修复）+ serve 失败 pty 回滚 + 复现文档 --writable 清扫——G-03-5 三条 missing 全部落地**

## Performance

- **Duration:** 8 min
- **Started:** 2026-08-18T02:08:28Z
- **Completed:** 2026-08-18T02:16:43Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- 坏 --tls-cert/--tls-key 路径下 wesh 先于 pty.Start/net.Listen/listening 打印报错退出（exit 1、零资源占用），print-then-die 时序缺陷修复；预加载证书对复用到 hs.TLSConfig.Certificates，ServeTLS 空串 certFile/keyFile——证书加载单一事实源
- Serve/ServeTLS 共享错误路径补 `_ = sess.Close()`，与 net.Listen 失败路径逐字对称——任何启动失败均不留 pty 子进程孤儿
- TestBadCertPreflight 两场景回归锁（free port print-then-die + occupied port 预检先于 listen 顺序锁），TDD RED→GREEN 门齐全
- 03-VERIFICATION.md 两处 + 03-UAT.md Test 5 一处复现命令补 --writable；README.md 经核查零改动

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): TestBadCertPreflight 两场景失败测试** - `de7ccbf` (test)
2. **Task 1 (GREEN): TLS 证书预检 + serve 失败 sess.Close() 回滚** - `481d741` (fix)
3. **Task 2: 文档清扫——复现命令补 --writable** - `e319cbc` (docs)

**Plan metadata:** 见下方最终 docs 提交（complete gap-closure plan 03-07）

_Note: Task 1 为 TDD 任务，RED（test）→ GREEN（fix）双提交；无 REFACTOR 需求_

## Files Created/Modified

- `cmd/wesh/main.go` - run() 增 TLS 证书预检（import crypto/tls）；ServeTLS 复用预加载证书对；serve 失败路径补 sess.Close() 回滚
- `cmd/wesh/main_test.go` - TestBadCertPreflight 两子场景（import 增 net/strconv）；注释写明零资源占用同构纪律与根因②无注入手段推理
- `.planning/phases/03-auth/03-VERIFICATION.md` - frontmatter human_verification #1 与 Human Verification Required #1 命令补 --writable
- `.planning/phases/03-auth/03-UAT.md` - Test 5 TLS 实例命令尾部补 --writable（判定字段零改动）

## Decisions Made

- 坏证书预检 exit 1（运行时 I/O 档位，与 pty.Start/net.Listen 失败路径同档），区别于 validateStartup 的 exit 2 配置矩阵错误——plan 既定，落地确认
- README.md 判定不改动也不登记 deferred-items：:11 为 Phase 1/2 历史行为变更说明（非复现命令），:79 systemd 示例展示默认安全姿态且 :29 flag 表 / :113 默认只读节已充分明示 ro 语义，不构成误解（plan 授权判断点）
- serve 失败回滚以逐字对称 + 代码评审锁定（Serve 阻塞语义 + lifecycle os.Exit 无单测故障注入手段）——plan 既定裁决，已写入测试与实现注释

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- RED 阶段场景 A 整测试二进制 panic（`unexpected call to os.Exit(0)`）：当前代码 spawn `true` + server.New 后才死于证书加载，`true` 退出触发 lifecycle goroutine 的 os.Exit——这正是 G-03-5 根因①「拒绝路径非零资源占用」副作用的实证，而非新问题。GREEN 后预检先于 pty.Start，无 spawn 无 lifecycle，panic 路径结构性消除（-race 全量复跑五包全 ok）。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- G-03-5 三条 missing 全部落地，可由 /gsd-verify-work 对账关闭；Phase 03（auth）gap 清零
- must_haves truths 全满足：坏证书零资源占用 exit 1（TestBadCertPreflight 锁）、启动失败全路径 sess.Close() 回滚（对称锁定）、文档复现命令 --writable 计数门全过

## Self-Check: PASSED
