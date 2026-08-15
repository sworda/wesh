---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 02
current_phase_name: protocol
status: executing
stopped_at: Completed 02-04-PLAN.md
last_updated: "2026-08-15T08:32:29.760Z"
last_activity: 2026-08-15
last_activity_desc: Phase 02 planning complete
progress:
  total_phases: 2
  completed_phases: 1
  total_plans: 11
  completed_plans: 9
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-14)

**Core value:** 浏览器里获得一个可靠、安全、可多人共享的远程终端
**Current focus:** Phase 02 — protocol

## Current Position

Phase: 02 (protocol) — EXECUTING
Plan: 5 of 6
Status: Ready to execute
Last activity: 2026-08-15 — Phase 02 execution started

Progress: [████████░░] 82%

## Performance Metrics

**Velocity:**

- Total plans completed: 5
- Average duration: -
- Total execution time: -

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | - | - |

**Recent Trend:**

- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P01 | 34min | 3 tasks | 18 files |
| Phase 01 P02 | 4min | 2 tasks | 3 files |
| Phase 01 P03 | 18min | 2 tasks | 3 files |
| Phase 01 P04 | 4min | 3 tasks | 3 files |
| Phase 01 P05 | 21min | 2 tasks | 2 files |
| Phase 02 P01 | 5min | 2 tasks | 2 files |
| Phase 02 P02 | 1h 27m | 2 tasks | 9 files |
| Phase 02 P03 | 16min | 2 tasks | 3 files |
| Phase 02 P04 | 13min | 2 tasks | 4 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: v1 采用 GoTTY 共享进程模型（PTY 随服务端启动、多客户端共享），ARCHITECTURE.md 的会话解耦阶段（ring 回放/保活回收）取消；outbox/fan-out 结构保留
- [Roadmap]: 多客户端 resize 仲裁分歧闭合——以需求 MULTI-04 为准（≥2 客户端一律最小公共矩形），ARCHITECTURE §2.9 "owner 跟随"作废
- [Roadmap]: WS 三层上限 + RFC 合规关闭码在 Phase 2 协议层一次性到位（事后补洞要动协议）
- [Roadmap]: SEC-06 env 白名单提前至 Phase 1 spawn 路径一次到位（PITFALLS C7）
- [Roadmap]: REQUIREMENTS.md 实际 v1 需求为 44 条（原文 Coverage 误写 42），已按 44 修正
- [Phase ?]: 仓库 stow/ 重命名为 wesh/，module path github.com/sworda/wesh 落地（D-01）
- [Phase ?]: server.New 钉死 ReadLoop drain（D-12）与 lifecycle（D-10）启动点；/ws handler 命名 Attach；exitf 经 sync.Once 收口
- [Phase ?]: 前端 typescript 钉 5.9.3（避 TS7 原生工具链风险）；build = tsc && vite build && gzip -k9
- [Phase ?]: [Phase 01-02]: fd 活性探测用 syscall.Fsync 而非 os.NewFile（finalizer 会误关真实 fd 0/1/2）；PTY 输出断言按 strings.Fields 切分免疫 ONLCR
- [Phase ?]: [Phase 01-03]: D-10/D-11 终结竞态修复——lifecycle 先置位 childExited 再发 1000 关闭帧，wsDisconnected 见置位即放弃 exitf 竞争，退出码传递确定化
- [Phase ?]: [Phase 01-03]: SIGHUP 送达证据用落盘标记文件（stdout 标记在 WS 断开后被 onChunk 丢弃不可观测）；/usr/bin/gofmt 陈旧须用 GOROOT 版本
- [Phase ?]: [Phase 01-04]: darwin awaitExit 经包级 sync.Once 单例 watcher，初始化/注册失败均退化为直接 cmd.Wait()（兜底不致命）
- [Phase ?]: [Phase 01-04]: CI 显式钉 pnpm 11.21.0（web/package.json 无 packageManager 字段，pnpm/action-setup 需版本源）
- [Phase ?]: [Phase 01-05]: README 按现状描述裸 clone——dist 已提交真实构建产物（非占位），改前端源码才需先 pnpm -C web build
- [Phase 02]: [Phase 02-02]: 握手违规路径（empty_frame/frame_before_hello/malformed_hello/version_mismatch）只关 conn 落入读循环，经既有 wsDisconnected→terminate 单一路径收口（CONTEXT L92）；非 plan 字面 return——02-03 TestVersionMismatch waitExit(0) 可达性共同锁定
- [Phase 02]: [Phase 02-02]: s.conn 推迟到 Welcome 发出后才上线——Welcome 恒为 S→C 首帧无时序竞态，预认证窗口零 PTY 输出
- [Phase 02]: [Phase 02-02]: pty.Session 增 fdMu+closed 修 Resize↔Close master fd 竞态（creack/pty Setsize 裸 Fd() 不过 fdmu，-race 实测命中）；Resize 见 closed 返回 os.ErrClosed
- [Phase ?]: [Phase 02-03]: per-IP release 恰好一次实现选型——局部 sync.Once + defer 兜底，显式释放仅挂 409/Accept/assert/升档四时点，其余一切 return 路径由 defer 收口
- [Phase ?]: [Phase 02-03]: ro 静默窗口测试形态——goroutine Read(context.Background()) + 缓冲 channel + select time.After 竞速，客户端 Read 永不带 deadline ctx（Pitfall 2 回归锁）
- [Phase 02]: [Phase 02-04]: pinger 错误路径精确分类——仅 errors.Is(err, context.DeadlineExceeded) 才 pong_timeout+CloseNow（父 ctx 为 WithCancel 无 deadline，DeadlineExceeded 唯一来源即 pctx 到期）；其余错误静默返回由 reader 路径收口 — 正常终结竞态下初版按 plan 字面 err!=nil 即打事件会误报 pong_timeout（TestPingKeepalive 实测命中），污染 D-12② 事件流可信度
- [Phase 02]: [Phase 02-04]: pinger 终结挂 Attach 内 WithCancel+defer cancel（ctx 由 Background 直派生改造），pinger 签名为 (ctx,c,remote,interval)——remote 供 logEvent 三要素 — 零新 exitf 分支（CONTEXT L92 硬约束）；plan 三参字面与 logEvent 三要素要求的调和

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 2]: WS 三层上限默认值需实测标定
- [Phase 5]: outbox 容量/水位/strikes 默认参数需负载测试标定（Phase 9 回填）

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-08-15T08:32:29.748Z
Stopped at: Completed 02-04-PLAN.md
Resume file: None
