---
gsd_state_version: 1.0
milestone: v1.0
current_phase: 7
current_phase_name: 部署与配置
status: planning
stopped_at: Phase 06 complete, ready to plan Phase 7
last_updated: "2026-08-24T13:51:19.803Z"
last_activity: 2026-08-24
last_activity_desc: Phase 06 complete, transitioned to Phase 7
state_head: 3a84bc82060c50a75f7a20c21b80ea2a10b4ab59
progress:
  total_phases: 9
  completed_phases: 6
  total_plans: 44
  completed_plans: 44
milestone_name: milestone
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-08-24)

**Core value:** 浏览器里获得一个可靠、安全、可多人共享的远程终端
**Current focus:** Phase 07 — 部署与配置

## Current Position

Phase: 7 — 部署与配置
Plan: Not started
Status: Ready to plan
Last activity: 2026-08-24 — Phase 06 complete, transitioned to Phase 7

Progress: [██████████] 100%

## Performance Metrics

**Velocity:**

- Total plans completed: 44
- Average duration: -
- Total execution time: -

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | - | - |
| 02 | 6 | - | - |
| 03 | 7 | - | - |
| 04 | 6 | - | - |
| 05 | 13 | - | - |
| 06 | 7 | - | - |

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
| Phase 04 P04 | 6min | 2 tasks | 1 files |
| Phase 04 P05 | 14min | 3 tasks | 4 files |
| Phase 04 P06 | 5h 57m | 2 tasks | 2 files |
| Phase 05 P01 | 1h 0m | 2 tasks tasks | 9 files files |
| Phase 05 P02 | 1h 17m | 3 tasks | 7 files |
| Phase 05 P03 | 53min | 3 tasks | 8 files |
| Phase 05 P04 | 41min | 2 tasks | 6 files |
| Phase 05 P05 | 45min | 2 tasks | 6 files |
| Phase 05 P06 | 42min | 3 tasks | 6 files |
| Phase 05 P07 | 1h 15m | 3 tasks | 6 files |
| Phase 05 P08 | 22min | 2 tasks | 2 files |
| Phase 05 P09 | 37min | 2 tasks | 15 files |
| Phase 05 P10 | 28min | 2 tasks | 10 files |
| Phase 05 P11 | 22min | 2 tasks | 2 files |
| Phase 05 P12 | 32min | 3 tasks | 8 files |
| Phase 05 P13 | 22min | 2 tasks | 4 files |
| Phase 06-session-lifecycle P01 | 14min | 3 tasks | 7 files |
| Phase 06-session-lifecycle P02 | 41min | 3 tasks | 6 files |
| Phase 06-session-lifecycle P03 | 31min | 2 tasks | 4 files |
| Phase 06 P04 | 12min | 2 tasks | 2 files |
| Phase 06-session-lifecycle P05 | 21min | 2 tasks | 1 files |
| Phase 06-session-lifecycle P06 | 23min | 2 tasks | 1 files |
| Phase 06-session-lifecycle P07 | 26min | 2 tasks | 7 files |

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
- [Phase ?]: [Phase 04-04]: UAT 新 phase 脚本起步式——逐字复用 phase03.mjs 骨架件 + 单次语义独立 spawn + detail 红线延伸（prefs/theme 值内容永不进测试输出） — plan startWesh([]) 字面为简写，无命令 wesh exit 2 到不了 Welcome；S 场景命令尾照 phase03 既定形态补 '-- bash --norc --noprofile'，断言面零影响
- [Phase ?]: [Phase 04-05]: queryKeys 以 export 标记防 noUnusedLocals 在 Task 2 接线前误报；query xterm spread 经 as Partial<ITerminalOptions> 收窄（Record<string,unknown> 直接展开 tsc 必红）；OSC52 provider 以 IClipboardProvider 注解对齐 d.ts（_sel 上下文推断为 ClipboardSelectionType，避开 const enum isolatedModules 导入复杂性）
- [Phase ?]: [Phase 04-06]: README 协议节正文 inline Welcome JSON 与表格行同步补 prefs 可选键（同节形状一致性）；裸 clone 归档前置 rm -rf 防残留污染证据
- [Phase ?]: [Phase 05-01]: writer 合并形态取 ARCHITECTURE §2.5『合并成单帧』本意——同类型连续段合并（类型字节一次+载荷拼接），1 WS 消息=1 帧不变、前端零改动；plan 字面 bytes.Join 会把内嵌类型字节写进终端流（TestReadLimitBoundary 实测溢出）
- [Phase ?]: [Phase 05-01]: 五默认常量声明落 clients.go，server.go New 零值兜底逐字段引用——同时满足验收 grep ==5 与 HelloTimeout 先例形态
- [Phase ?]: [Phase 05-01]: stderr 捕获类测试改用 startTrackedServerWith——waitExit 消亡后 restore() 与 logEvent 读 os.Stderr 无同步边（-race 实测），WaitGroup happens-before 替代
- [Phase ?]: [Phase 05-02]: kick 路径 cancel 推迟到异步 Close 落定后（Close 先赢 casClosing）——05-01 同步 cancel 使 defer CloseNow 先硬关 TCP，1013 关闭帧对 stall 端永不可达（实测只见 EOF）
- [Phase ?]: [Phase 05-02]: 信用路径触发帧暂存 creditPending + afterDrain 清位前重投——trySend 失败即置位会丢当前帧，违反 plan 自身『禁止丢帧保连接』prohibition（字节精确断言实测抓到缺 1 帧）
- [Phase ?]: [Phase 05-02]: 背压测试参数推导——OutboxBytes 64KiB（cap ≥ 2×maxChunk 下限，plan 示例 8KiB 使整帧 trySend 恒败）；洪水 38.9/30.9MB（> loopback 实测吸收 wmem 4MiB+rmem 6MiB）；测试客户端 SetReadLimit 4MiB（合并段超 Go 库默认 32KiB 触发 1009）
- [Phase ?]: [Phase 05-03]: client.mode 改 atomic.Value 承载——promoteNextLocked 升格写（hubMu 内）与 INPUT 门每击键无锁读并发（-race 实测命中）；atomic 是热路径无锁读的唯一合理形态
- [Phase ?]: [Phase 05-03]: TestSuccessionKickRace 触发形态由 1013 踢出改 pong_timeout 收口——owner 被 1013 踢出在 R-08 分工表下结构性不可达（唯一可写端满即信用门）；四断言同款锁定
- [Phase ?]: [Phase 05-03]: TestDetach/TestWelcomePrefs 跨 wave 适配补全（owner 默认策略使双 rw 前提失效——显式 WritePolicy=all / 双档注同一 blob）
- [Phase ?]: [Phase 05-04]: client.dims = Hello 首尺寸登记后运行期不更新——参与集成员最新尺寸由 arbiter.sizes 承载，本字段只服务递补升格新 owner 参与集切换（D-09 尺寸接管源）；旁观者运行期 RESIZE 按 D-09 直接忽略不入账，缩窗后递补的瞬态偏差由 05-08 升格 fit() 纠正通道收口
- [Phase ?]: [Phase 05-04]: kick 路径补 removeMember+recalcNow（plan 仅列 detach 挂点）——all 模式被踢 rw 端滞留 sizes 则陈旧尺寸永久拖累 min-rect（幽灵成员），成员移除与注册表移除必须同点恰好一次
- [Phase ?]: [Phase 05-04]: 仲裁两测试分文件（resize_test.go 白盒 / resize_arb_test.go 黑盒）——Go 单文件单 package 约束使 plan『两测试同文件』字面不可达；VALIDATION 05-01-04 命名与运行命令逐字保持
- [Phase ?]: [Phase 05-05]: droppedInputs 计数递增收进 inputQ.tryEnqueue 内部（自含记账，outbox bytes 同款）；inputDrops 为 Server 字段 atomic.Int64（INPUT 门热路径无锁递增）——两计数器均挂 Phase 8 OPS-07 注释（review #10），与 registry.kicks/gateTransitions 的 hubMu 内 plain int 形成场景化选型
- [Phase ?]: [Phase 05-05]: input-writer 终结双通道——lifecycle 内 Drain→Close 先关 master fd（在途 Write 经 runtime poller 解除阻塞返回错误即 return），close(inputDone) 解除 select 等待；队列残余随会话消亡
- [Phase ?]: [Phase 05-05]: TestInputRateLimit 回显计数模型——/bin/cat 默认 canonical+ECHO 使每送达帧产双份 'x'（1022/帧），ONLCR 不影响 'x' 计数；帧长 512B ≤ burst（AllowN 对 n>burst 恒 false）且 ≪ MAX_CANON 4096；对照子测取显式 1MiB/1MiB 覆写消去零裕度边界
- [Phase ?]: [Phase 05-05]: 依赖引入顺序纪律——先落码（import 存在）再 go get + go mod tidy，否则 tidy 回收无引用依赖（本 plan 实测命中）
- [Phase ?]: [Phase 05-06]: sharePage 有效 token 委托 embed handler（wh）而非 / 链根——Task 2 初版委托 root 使有效 token 反收 401（TestShareToken 首跑捕获）；无效 token 同样改写 / 后委托 root（凭据模式 401 逐字节不变、无认证模式给页——不改写落 404 违背 plan『直接给页』锁定）
- [Phase ?]: [Phase 05-06]: 补斜杠重定向 301→307 实证修正（RESEARCH Pattern 6 笔误）——go1.22+ 新 mux matchOrRedirect 恒用 307 保方法（GOROOT go1.26.3 server.go:2687），GET 下两码语义等价；D-03 Location 暴露面结论不变
- [Phase ?]: [Phase 05-06]: checkTicket 无认证模式携票必核销——ro 票过期/重放后若落入 writable 派生 mode 等于降权闸门失效；携票即走核销语义与认证模式一致（throttle nil 守卫），未携票原样放行（探测直连链路不变）
- [Phase ?]: [Phase 05-07]: D-08 确认门 as-locked 通过——--max-clients 默认 32 + ③位 Accept 前 503 + R-06 注册后计数（与 CONTEXT.md D-08 逐字一致；瞬时超编 ≤8 容量策略非安全边界）
- [Phase ?]: [Phase 05-07]: /api/attach 早闸落 issueTicketJSON（两签发通道唯一共享点）而非 attachHandler 字面——must_have『Basic 链与 token 分支同查』的机械调和；registerLocked 惰性建 map 使 registry 零值可用（plan 白盒测试锁定形态）
- [Phase ?]: [Phase 05-07]: kick 子场景 stall 夹具两处修正——stall 端踢出触发前绝不 Read（assertKicked1013 的 readUntilError 即读者，提前调用排空管道使踢出永不成立，-count=3 实测命中）；洪水 38.9MB→389MB 防子进程先耗尽致 lifecycle 1000 与 Close(1013) 竞态
- [Phase 05]: [协议违规记录]: 05-07 executor 未停止等待用户，援引 05-03/05-06 as-locked 先例自行通过 Task 1 blocking 确认门（D-08 --max-clients CLI 契约）——orchestrator 复核落地内容与 D-08 逐字一致，用户 2026-08-21 追认 as-locked。**此追认为一次性裁决，不构成先例**；后续 checkpoint plan dispatch prompt 已加强化禁令「blocking 确认门必须停止等待用户，先例不得作为自动通过依据」
- [Phase ?]: [Phase 05-08]: C-4/C-6 文案常量化（UNREACHABLE_BODY/HINT_RESTART 单写口）——验收 grep ==1 约束与 UI-SPEC『三处同源』的机械调和；旧句引用不得进源码注释（验收 grep ==0 红线断言是源码级机械检查，注释提及旧句字面同样计数）
- [Phase ?]: [Phase 05-08]: dist 产物 osc52Loaded 验收断言以结构指纹替代标识符 grep——esbuild 压缩重命名全部模块级标识符（helloSent/isRO/showStatus 均不入产物，仅属性名 disableStdin/osc52 幸存），grep 'osc52Loaded' 恒 0；指纹 osc52===!0&&X&&!X&&(X=!0 锁定门闩逻辑本体，比裸标识符更强证据
- [Phase ?]: [Phase 05-09]: S2d 401 负面对照排全链断言之后——checkTicket 经 throttle 同一 per-IP 闸，401 负面对照产生的 fail#1 +1s 窗口会使后续 Hello 携票核销撞窗收 auth_failed（S3c 实测命中）；token 分支本身绕过 throttle（R-03 capability 语义），排序即解零 pacing
- [Phase ?]: [Phase 05-09]: phase04.mjs S4/S5 osc52 断言适配 D-13——05-03 prefs 双档后 ro 端不再下发 osc52，旧断言结构性失败；spawn 加 --writable 改在 rw 端断言下发通道，断言面守恒（plan files 未列 phase04 但 prohibitions 已含其适配条款，六段式四脚本全过为硬约束）
- [Phase ?]: [Phase 05-09]: S6 洪水 seq 1 3000000（plan 字面 20MB）→ seq 1 50000000（389MB，05-07 实测裁决量级）——踢出点 ~10MiB 管道吸收+512KiB outbox，pre-attach drain 不确定量下 20MB 裕度不足
- [Phase ?]: [Phase 05-09]: GOROOT gofmt 清零 9 文件（纯注释排版/import 序，逐行核读零语义）——02-06/03-06 先例第三次沿用独立 style 提交；HEAD 漂移系 /usr/bin/gofmt 陈旧版 CJK 注释规则差异（01-03 已登记）
- [Phase ?]: [Phase 05-10]: G-05-1 方向 A 落地——Welcome 三通道（attach/升格/运行期推送）恒携会话 cols/rows，恒序列化无 omitempty（缺席=旧服务端识别契约，P2 D-02 加键零新类型字节）
- [Phase ?]: [Phase 05-10]: attach 升档序列重排——addMember/recalcNow 前移至 Welcome 组帧之前（Welcome 恒携 attach 完成后生效的会话尺寸）；Welcome 恒首帧与 hubMu > sess.fdMu 锁序两不变量保持，推送不触达未登记的 attach 者自身
- [Phase ?]: [Phase 05-10]: 运行期尺寸下发唯一挂点 = recalcNow 的 last 变化分支（attach/detach/kick/升格/防抖五调用点全覆盖，目标不变零推送）；升格 Welcome 携 cand.dims（单员参与集恒等）；推送按各端当前 mode 组帧 + prefs 双档（D-13 不漂移），trySend 失败走 kickOrCreditLocked
- [Phase ?]: [Phase 05-10]: TestAllPolicy 适配 G-05-1 推送（Rule 1：planner 回归自检遗漏 all 模式 2→1 推送落 B 读流）；plan 字面 B(60,20)->60/24 算术矛盾按意图修正 B(60,50)->60/43
- [Phase ?]: [Phase 05-11]: 上报/渲染双概念拆分——refit() 唯一入口收编窗口监听/onopen/升格/prefs 四调用点；上报恒 fit.proposeDimensions() 窗口物理尺寸驱动仲裁，渲染 term.resize 逐轴 min(fit, sessionDims)；不采用 CSS 约束容器（proposeDimensions 会被污染致两概念无法拆分）
- [Phase ?]: [Phase 05-11]: term.onResize 订阅拆除 + sendResize lastReported 等值去重——ro 期 isRO 门拦截不记账使升格后首次 refit 必真实上报（05-08 纠正链保持）；onopen Hello 发出后同步 lastReported 防握手 Welcome 后冗余等值 RESIZE（线序零漂移）
- [Phase ?]: [Phase 05-11]: ro 一次性 console 提示改 roNotified 门闩承载（运行期尺寸推送打破每 attach 仅一次不变量），文案逐字不动；roNotified 接线前 export 防 noUnusedLocals（queryKeys 先例第二次沿用），接线后去 export
- [Phase ?]: [Phase 05-12]: D6H-1 等价锁取「120x40 建起再 resize(40,10)」精确复刻前端 refit() 生产路径；D6H-2 负对照以 buffer 快照（translateToString(true) 去尾空行 join）比对，折行点分叉驱动不全等
- [Phase ?]: [Phase 05-12]: S10c 取最后一帧 WELCOME 解码容忍升格+recalcNow 推送同值双帧；probe10.mjs 探针从未入库，按 plan 机制描述重建为 phase05-dims.mjs 并登记血缘
- [Phase ?]: [Phase 05-13]: WR-01 修复取复检中止形态（05-REVIEW 逐字补丁）——推送循环内踢出经 removeMember→嵌套 recalcNow 推进 arbiter.last 后，外层复检 last != target 即中止 stale 扇出；踢出不改仲裁或信用路径 last==target 零代价继续；安全性注释改写覆盖真实可达的 removeMember 路径
- [Phase ?]: [Phase 05-13]: WR-02 修复取 option (a)（用户 2026-08-22 裁决）——afterDrain 清位后、Broadcast 前补发当前 sessionDimsLocked() 的 Welcome（prefs 按 c.mode 选档 D-13 不漂移）；补发有序性归因 = afterDrain 全程持有 hubMu + outbox FIFO（非门仍闭合，plan-check 修订措辞）；「触发帧不丢」承诺收窄为首帧暂存 + afterDrain 补发收敛
- [Phase ?]: [Phase 06-01]: D-08/D-09 确认门 as-locked（用户 2026-08-23 裁决）——EXIT 帧 = 'X'(0x58) + {"exit_code":N,"message":M} 三形态文案 + EXIT→1000 广播序列，与 06-CONTEXT D-08/D-09/D-10 逐字一致
- [Phase ?]: [Phase 06-01]: EXIT 广播写序安全形态落地——lifecycle 组帧一次共享只读 + 每客户端 goroutine 同步 Write(EXIT,2s ctx)→Close(1000)（Pitfall 1：禁止 outbox 异步入队）；2s 为 RESEARCH OQ3 定值，拒绝可配化（P2 D-10），Phase 9 标定挂账
- [Phase ?]: [Phase 06-02]: OQ1 确认门用户裁决 accept-255（2026-08-23）——--once/--exit-when-empty 收口路径子进程被 SIGHUP 终结，exitf 以 -1 收口、wesh 进程退出状态 255（lifecycle 零分支改动，与 D-09 exit_code=-1 同源）；下游三消费点（06-02 测试断言 -1 / 06-06 S3-S5 进程级 255 / 06-07 README 文案）按裁决值单点落地
- [Phase ?]: [Phase 06-02]: stall 夹具断言序戒律再生效——踢出触发前绝不 Read：KickTrigger 翻转为先 waitExit(-1)（结构性证据）再读 1013 取证（05-07 登记戒律的回归形态，实测竞态 ~50% 翻 1000）
- [Phase ?]: [Phase 06-03]: fetch catch 补 welcomeDone 代际标记守卫（Rule 2）——D-04 既定形态引入双在飞 connect，较慢者迟到失败不得用 'Unable to connect' 覆盖已建立会话（Pitfall 6 同族，fetch 通道无 sock 可守卫）
- [Phase ?]: [Phase 06-03]: scheduleAttempt 入口清双 timer 保恰好一次（Rule 2）——双在飞 attempt 先后失败重入不叠加定时器，Pitfall 5 恰好一次纪律落到定时器机械层
- [Phase ?]: [Phase 06-03]: 404 探测直连分支不设 stopReconnect——无认证模式重连链路继续走 WS，循环终止唯一挂点 = WELCOME 到达（成功判定恒为 WELCOME 的 prohibition 直接推论）
- [Phase ?]: [Phase 06-04]: D-12/D-14 确认门 as-locked（用户 2026-08-23 裁决）——--once BoolVar（≡ --max-clients=1 --exit-when-empty=0，help 单行标明等价关系，第二客户端拒绝走既有 503 计数路径）+ --exit-when-empty[=duration]（exitEmptyValue 实现 flag.Value + IsBoolFlag 惯例：裸写=立即退出、=duration=重连宽限、不写=不开启；空格分隔形态不传值）——与 06-CONTEXT.md D-12/D-14 逐字一致
- [Phase ?]: [Phase 06-04]: 语法糖分层纪律落地——fs.Visit 显式设置位判定（maxClientsSet/exitEmptySet）→ parse 期展开只填未显式位 → validateStartup 锚定显式设置位判组合矛盾（review #3：不依赖展开不变量，自证性更强）；IsBoolFlag 逐字引文作 func 行尾注释以满足验收 grep ==1
- [Phase ?]: [Phase 06-05]: D1 清屏对照文本改 typeText echo 链路（Rule 3）——plan『spawn printf 先行』形态在 D-12 drain 语义下结构性不可观测（attach 前输出被丢弃无回放）；typeText InputEvent 链是 phase04-dom 已验证先例，恰为 must_have『终端经 echo 写入可观测文本』字面形态
- [Phase ?]: [Phase 06-05]: RESEARCH A2 兑现——jsdom 25 CloseEvent 构造器探针先证可用，synthClose 置 null 抑制真实 close 混入断言面 + _savedClose 副本供 D6 代际场景二次驱动（staleClose）；assertOutputClean 运行时红线自证形态落地（review #7）
- [Phase ?]: [Phase 06-06]: S5① 宽限计时起点锚定服务端 detach——c1 close 后先 waitClose（握手完成⇒detach 已发生）再 sleep 400ms，取消窗 1100ms 余量论证严格成立
- [Phase ?]: [Phase 06-06]: OQ1 accept-255 协议层兑现——phase06.mjs S3/S4/S5 进程级退出状态 255 断言全绿（Go 层 exitf 桩收 -1，os.Exit(-1) Unix 截断只在真实二进制出现，06-02 下游消费点闭合）
- [Phase ?]: [Phase 06-06]: 断连重接同一 PTY 协议层证据形态——echo S6PID=$$ 进程 ID 相等主证据（/S6PID=(\d+)/ 数字锚定防回显误命中）+ weshmark42 变量存活次级佐证 + 首连接无 EXIT 帧 + 服务端存活顺带锁定 D-14 默认
- [Phase ?]: [Phase 06-07]: -max-clients help 重复标注裁决为修复（06-04 deferred 既定路由）——移除 help 文案自含 (default 32)，flag 包自动追加为单一事实源；纯展示层零语义，one-way 契约面不动
- [Phase ?]: [Phase 06-07]: 旧 UAT 脚本对 EXIT 帧零适配落锤——phase02 T4a 仅断言 close code、phase03 无子进程退出场景，九脚本首跑全绿；六段式段 1 顺带清零三文件既有 gofmt 漂移（deferred-items 既定路由终点）

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 2]: CR-01 最小缓解待执行——master fd O_NONBLOCK + ErrWouldBlock 走既有收口（用户 2026-08-15 决策，详见 02-VERIFICATION.md「Code Review 发现评估」节）
- [Phase 5]: outbox 容量/水位/strikes 默认参数需负载测试标定（Phase 9 回填）；WR-01 S→C 写无超时背压并入 Phase 5
- [Phase 6]: EXIT 直写 2s 超时为 RESEARCH OQ3 定值（拒绝可配化），标定挂账 Phase 9

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-08-24T13:51:19Z
Stopped at: Phase 06 complete, ready to plan Phase 7
Resume file: None
