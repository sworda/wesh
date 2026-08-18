---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 04
current_phase_name: frontend
status: executing
stopped_at: Completed 04-03-PLAN.md
last_updated: "2026-08-18T16:38:34.224Z"
last_activity: 2026-08-18
last_activity_desc: Phase 03 execution started
progress:
  total_phases: 4
  completed_phases: 3
  total_plans: 24
  completed_plans: 21
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-15)

**Core value:** 浏览器里获得一个可靠、安全、可多人共享的远程终端
**Current focus:** Phase 04 — frontend

## Current Position

Phase: 04 (frontend) — EXECUTING
Plan: 4 of 6
Status: Ready to execute
Last activity: 2026-08-18 — Phase 04 execution started

Progress: [█████████░] 88%

## Performance Metrics

**Velocity:**

- Total plans completed: 18
- Average duration: -
- Total execution time: -

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | - | - |
| 02 | 6 | - | - |
| 03 | 7 | - | - |

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
| Phase 02 P05 | 9min | 2 tasks | 2 files |
| Phase 02 P06 | 2h 36m | 2 tasks | 2 files |
| Phase 03 P01 | 18min | 3 tasks | 6 files |
| Phase 03 P02 | 15min | 3 tasks | 7 files |
| Phase 03-auth P03 | 52min | 3 tasks | 7 files |
| Phase 03 P04 | 18min | 2 tasks | 3 files |
| Phase 03 P05 | 14min | 2 tasks | 2 files |
| Phase 03 P06 | 2h 05m | 2 tasks | 8 files |
| Phase 04 P01 | 25min | 2 tasks | 6 files |
| Phase 04 P02 | 16min | 3 tasks | 9 files |
| Phase 04 P03 | 11min | 3 tasks | 3 files |

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
- [Phase ?]: [Phase 02-06]: proto.go 02-01 既存 gofmt 差异随段 1 授权分支清零（纯注释排版）；冒烟以 --port 0 随机端口 + 启动行解析驱动断言
- [Phase 02]: [UAT]: 浏览器渲染层 UAT 在无显示机器上以「Node 原生 WS 客户端协议断言（web/uat/phase02.mjs，零依赖 11/11）+ 用户外部浏览器渲染层确认」分工完成——协议层全自动化，渲染层人工
- [Phase 02]: [UAT 决策]: CR-01（Attach 读循环同步写 PTY master 可永久阻塞）用户裁决立即最小缓解——master fd O_NONBLOCK + ErrWouldBlock 走既有收口；完整背压（有界输入队列+写 goroutine+1013 踢出）留 Phase 5
- [Phase ?]: [Phase 03-01]: TestDecodeHello ticket 断言加 checkTicket 闸——plan 三约束（wantTicket 字段+既有行补零值+禁止改 unknown 行）与统一断言冲突（unknown 行 ticket:"secret" 加字段后解码入 Ticket），闸化后仅两新行断言 Ticket，D-02 回归锁逐字不动
- [Phase ?]: [Phase 03-01]: ErrAuthFailed 入 TestProtocolConstants 逐字+形状锁定——D-10 costly 级公开契约常量按文件既定职责入锁（T-02-01 缓解形态）
- [Phase ?]: [Phase 03-02]: matchCredential 按 planner erratum 修正形态落地（|= 位或累积不短路；RESEARCH Pattern 2 的 &= 初值 0 恒 false 不可照抄），TestCredentialMatch 多组各自命中锁死该回归
- [Phase ?]: [Phase 03-02]: 空 pass 合法（ParseCredential("user:") 不额外禁止，passHash 为空串摘要）——文档化决策；空 user 仍拒（RFC 7617）
- [Phase ?]: [Phase 03-03]: logEvent 提为包级函数——plan 指定的 basicAuth 三参自由函数签名需调用日志唯一出口，logEvent 无 Server 状态依赖；HTTP 层事件 code 复用 HTTP 状态码值（websocket.StatusCode 底层 int）
- [Phase ?]: [Phase 03-03]: ServeMux 方法模式内建 405 被 / 子树吞掉（GOROOT server.go:2699-2710 n==nil 分支）——显式注册 /api/attach path-only 405 fallback（Allow: POST，与内建回退同文）补齐守卫链第一闸（Rule 1）
- [Phase ?]: [Phase 03-03]: TestOriginEndpoints 全 HTTP 层拒绝场景共用单实例（零 attach 零终结路径，waitExit 结构性不可达）；captureStderr 复用 limits_test.go 既有 helper；originMiddleware 落位 origin.go 与 originAllowed 内聚
- [Phase ?]: [Phase 03-04]: parse 期校验（TLS 成对/env 凭据）插入点在 showVersion 早退之后——纯信息路径不被配置校验阻断
- [Phase ?]: [Phase 03-04]: 启动警告串自含 wesh: warning: 前缀由 validateStartup 返回完整行；warn/err 文案不含凭据值（启动面红线，矩阵全行断言）
- [Phase ?]: [Phase 03-04]: TestParseArgs 表结构扩展走命名字段转换——Go 位置初始化不可扩展字段的唯一调和形态，既有行值/断言零改动
- [Phase ?]: [Phase 03-05]: ws 声明为模块级 WebSocket | null（非 plan 字面 let ws: WebSocket）+ onData/sendResize null 闸——fetch 异步窗口期用户敲击 ws.readyState 的 TypeError 回归（Rule 1）；connect() 内 const sock = ws 供 handler 闭包（TS 严格模式闭包不收窄可空 let）
- [Phase ?]: [Phase 03-05]: dist .gz 不入库按 .gitignore 既定策略（web/dist/*.gz 自建仓起生效，gzip 头嵌 mtime 每次构建字节漂移；embed.go 设计即 index.html 入库、gz 缺失明文伺服降级）；README:40 陈旧声明登记 deferred-items.md
- [Phase ?]: [Phase 03-06]: 场景 1 pacing 采用爬梯 sleep（1.15s/2.15s/4.3s）优先于独立实例备选——同时证明退避窗口恢复语义；场景 3 无 Origin 断言取 400 形态（不建 WS 连接不触发单次语义退出）；S1f 非法 ticket 独立 spawn 实例（单会话约束）
- [Phase ?]: [Phase 03-06]: 六段式段 1 gofmt 清零授权沿用 02-06 先例——4 文件纯注释排版差异 -w 修正后独立 style 提交（87f6e17），零语义改动
- [Phase 04]: --client-option 校验错误记录式上报（clientOptErr + Parse 后统一返回）——避开 flag 包 invalid value %q 包装回显值内容，守 SEC-01 启动面红线（04-01） — flag 包 failf 将回调错误包装为 invalid value %q（原始 key=value 串）并打印 stderr，plan 字面 return 形态必违反值内容红线；记录式两通道干净且 exit 2 语义不变
- [Phase 04]: js-base64 override 落 web/pnpm-workspace.yaml 而非 package.json pnpm 字段——pnpm 11.21.0（CI 同钉）WARN 明示不再读该字段，overrides 官方新家即 pnpm-workspace.yaml；钉 3.9.2 避 1 天新包意图逐字保持（04-02） — pnpm 11 settings 迁移导致 plan 字面机制不生效；迁移后 lockfile 三处解析均 3.9.2

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 2]: CR-01 最小缓解待执行——master fd O_NONBLOCK + ErrWouldBlock 走既有收口（用户 2026-08-15 决策，详见 02-VERIFICATION.md「Code Review 发现评估」节）
- [Phase 5]: outbox 容量/水位/strikes 默认参数需负载测试标定（Phase 9 回填）；WR-01 S→C 写无超时背压并入 Phase 5

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-08-18T16:38:34.208Z
Stopped at: Completed 04-03-PLAN.md
Resume file: None
