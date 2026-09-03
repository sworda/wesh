---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: per-client 会话模式
current_phase: 11
current_phase_name: per-client 生命周期主干
status: executing
stopped_at: Completed 11-04-PLAN.md
last_updated: "2026-09-03T18:30:28.402Z"
last_activity: 2026-09-04
last_activity_desc: Phase 11 execution started
progress:
  total_phases: 2
  completed_phases: 1
  total_plans: 11
  completed_plans: 9
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-03)

**Core value:** 浏览器里获得一个可靠、安全、可多人共享的远程终端
**Current focus:** Phase 11 — per-client 生命周期主干

## Current Position

Phase: 11 (per-client 生命周期主干) — EXECUTING
Plan: 5 of 6
Status: Ready to execute
Last activity: 2026-09-04 — Phase 11 execution started

Progress: [████████░░] 82%（v1.1；v1.0 已 9/9 阶段 70/70 计划收口，v1.0.0 已发布）

## Performance Metrics

**Velocity:**

- Total plans completed: 70（v1.0）
- Average duration: -
- Total execution time: -

**By Phase (v1.0):**

| Phase | Plans | Phase | Plans |
|-------|-------|-------|-------|
| 01 | 5 | 06 | 7 |
| 02 | 6 | 07 | 10 |
| 03 | 7 | 08 | 6 |
| 04 | 6 | 09 | 10 |
| 05 | 13 | | |

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 10 P01 | 32min | 2 tasks | 6 files |
| Phase 10 P02 | 35 min | 2 tasks | 4 files |
| Phase 10 P03 | 18 min | 2 tasks | 2 files |
| Phase 10 P04 | 25 min | 2 tasks | 2 files |
| Phase 10 P05 | 31min | 3 tasks | 3 files |
| Phase 11-per-client P01 | 40min | 2 tasks | 5 files |
| Phase 11-per-client P02 | 9min | 2 tasks | 2 files |
| Phase 11-per-client P03 | 19min | 2 tasks | 3 files |
| Phase 11-per-client P04 | 30min | 2 tasks | 1 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Milestone]: v1.0 全量收口（44/44 需求），v1.0.0 于 2026-08-30 实发布上架（四平台 + checksums 核验全 OK）
- [Roadmap v1.1]: 五阶段沿研究骨架与依赖链——装配阀门(10) ≺ 生命周期主干(11) ≺ 交互背压(12) ≺ 资源防线与终结语义(13) ≺ 标定/UAT(14)；2026-09-03 原 13/14 合并（原 13 经 Phase 11 D-01/D-03 机制先行收窄后独立 phase 开销过重，合并同时提前闭合 --once 窗口期缺口）、原 15 重编号 14
- [Research v1.1]: 零新增依赖；「装配期一次分岔、运行期零分岔」不抽象 session 接口（6-7 显式分支点）；最大风险=破坏既有不变量而不自知（D-10 唯一终结/D-13 零新 exitf/唯一收割者/Welcome 恒首帧/零身份 label）
- [Requirements v1.1]: D5 裁决落定——SEC-09 per-client 下 WESH_REMOTE_USER 注入子进程 env（D-15 收窄理由结构性消失），shared 保持收窄语义；反特性五条入 Out of Scope（reattach/linger/运行期切模式/默认 per-client/ro 共享进程）
- [Phase ?]: [Phase 10-01] run() 两模式均经启动期 pty.Start（sess=nil 与 New 体 sess.Cmd.Process.Pid 取引用冲突，归 Phase 11）；SpawnFunc 闭包 inert 零调用方
- [Phase ?]: [Phase 10-01] ValidateOptions 包级互斥校验 option (b) 落地：per-client×SpawnFunc=nil / shared×SpawnFunc≠nil fail-fast，零值归一 shared 与 New 兜底同口径
- [Phase ?]: [Phase 10-05] GOROOT gofmt（go1.26.3 现代 doc-comment 规则）定为收口闸工具：10-01 遗留两行 CJK 标点接续注释补空格归一（a412a87），新旧 gofmt 双 clean；历史闸用 PATH 旧版 gofmt 故未报
- [Phase ?]: [Phase 11-01] perclient_test.go 落 package server_test（plan 文本 package server 与「同包复用 e2e_test.go helper」矛盾，按后者裁决）
- [Phase ?]: [Phase 11-01] Task 1 TDD 以 plan 显式单 feat 提交收口；PC-02/03/04 需求勾选留给 phase 末 plan 11-06（ID 跨 6 plan 共享）
- [Phase ?]: [Phase 11-02] darwin exit watcher dup-watch fail-closed 落地（Pitfall 9 挂账兑现）：errDupWatch 包级错误值 + watch() w.mu 内 dup 检查；awaitExit 既有分支退化 cmd.Wait() 兜底零新面；TestWatchDupPidFailClosed 由 CI macOS leg 承担运行
- [Phase ?]: [Phase 11-03] D-02 容量再闸落地：capacityMessage 常量 + rejectCapacity 单点（两容量拒绝路径 wire 不可区分是有意为之）；Task 2 TDD 单 test 提交延续 11-01 先例
- [Phase ?]: [Phase 11-03] D-03 复检回收落地：硬不变量「并发子进程数 ≤ maxClients」Phase 11 即成立——Phase 13 裁决项④提前消解，Phase 13 规划时移除 STATE Blockers ④
- [Phase ?]: [Phase 11-04] plan 文本 kill -TERM $$ 勘误为 kill -HUP $$：交互 shell 无 trap 忽略 SIGTERM（实测不致死），HUP 致死且与 exit_test.go 信号夹具同款——后续 plan 信号夹具选型应直接用 HUP/trap 形态
- [Phase ?]: [Phase 11-04] 竞态注入测关闭观测形态：客户端主动 Close 后 Read 恒 net.ErrClosed（库 prepareRead 语义），「读至 CloseError」经并发泵 + 1000 证据双通道（泵 CloseError / Close nil 返回）实现
- [Phase ?]: [Phase 11-04] PC-03/PC-04 需求勾选延续既定归 phase 末 11-06（跨 6 plan 共享 ID）

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 9 遗留]: TestResize CI 时序 flake（CI 观察一次，重载 runner 调度延迟所致，非产品缺陷）——择机以轮询替代固定 sleep 修复
- [Phase 9 遗留]: README.md:96「及其 `.gz`」Phase 1 遗留文档债——随 WR 清单择机处置
- [v1.1 规划期裁决项]: ① per-client stop-timeout 默认值重议（0=不补 KILL 在新模式下=HUP 免疫泄漏，公开契约变更，Pitfall 8）→ Phase 13；③ healthz/metrics 四个 OQ（session_alive 语义/series 双语义/1013 vs 阻塞/spawn 失败 wire 面，研究均有推荐答案）→ Phase 13（② write-policy×per-client 经 Phase 10 D-01/D-02 闭合；④ spawn-intent 口径经 Phase 11 D-03 复检回收提前消解）
- [v1.1 测试拓扑]: 协议层 UAT 在 Linux 开发机（headless 禁浏览器/禁 playwright）；Playwright 浏览器全链在 Windows 工作站（TCP 转发器 kill/restore 模拟断网）——见 CODEBUDDY.md 双机拓扑

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-09-03T18:30:28.388Z
Stopped at: Completed 11-04-PLAN.md
Resume file: None
