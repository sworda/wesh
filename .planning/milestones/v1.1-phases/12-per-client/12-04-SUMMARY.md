---
phase: 12-per-client
plan: 04
subsystem: uat-protocol
tags: [uat, protocol, websocket, per-client, stall-resume, dwell-kick, raw-socket, backpressure, tdd]

# Dependency graph
requires:
  - phase: 12-per-client plan 01
    provides: WelcomePayload.Session 模式位（proto→wire，S1 对照断言面）
  - phase: 12-per-client plan 02
    provides: RESIZE per-client 直通分支（S2/S3 wire 行为）+ ro INPUT 门控/限速（S4）+ debouncer 共用件
  - phase: 12-per-client plan 03
    provides: 阻塞持帧 + dwell 看门狗 + outbox notFull 恢复信号（S5/S6 服务端语义）+ /healthz 轮询观测纪律 + PTY 微帧/内核管线量级认知
  - phase: 11-per-client
    provides: phase11.mjs 同构母本（骨架逐字）+ pollESRCH/pgroupAlive 形态
  - phase: 05-multi-client
    provides: rawStallClient raw socket 停读夹具纪律（phase05.mjs:157-192——undici 恒排空 TCP，内核级停读必须 raw socket）
provides:
  - web/uat/phase12.mjs（754 行）：Phase 12 协议层 UAT 六场景——S1 Welcome.session 双模式对照 / S2 resize 直通双端隔离+零 'W' 帧 / S3 ro RESIZE 直通 WINCH trap 回读+shared 对照 / S4 ro INPUT 丢弃+rw 限速保留 / S5 停读续读 34.9MB 字节级连续 / S6 真实 10s+ dwell 到期 1013+ESRCH
  - RawStallClient 通用 raw socket 停读客户端（手构升级 + RFC6455 masked 组帧 + 16/64 位扩展长度解析 + ping 自动回 pong + close 帧 echo 收口 + pause/resume 即停读/续读表达）——后续 phase UAT 停读场景直接复用
  - 执行期实证发现：coder/websocket writeControl 内层 5s 写超时 × pinger 单一 DeadlineExceeded 判读 → 默认 --ping-interval=5s 下 TCP 级停读客户端被 1006 pong_timeout 先杀（(停读+5s, 停读+10s] 窗口），dwell 1013 结构性后到——Phase 13 裁决材料（STATE Blockers 登记）
affects: [12-05 (PC-05/06/07/10/11 勾选证据链 = 本脚本六场景 + 12-01/02/03 Go 测 + phase12-dom 三场景), 13 (pinger/dwell 竞态裁决 + 默认配置 1006/1013 取舍), 14 (herdr E2E 参数化 harness 收编)]

# Actuals (#2632) — 与 plan estimate (60000 tokens) 同标尺。
# 口径注记：源码 diff chars/4（web/uat/phase12.mjs，基点 aadbe94）。
# 大幅低于 estimate 的结构性原因沿用 12-01/02/03 口径：骨架逐字继承 phase11.mjs
# （startWesh/dialHello/双通道/自净断言零新写），新增主体仅六场景函数与
# RawStallClient。
# 诚实注记（12-03 同款）：diff 标尺不覆盖执行期调查成本——S6 首跑 FAIL 的
# pong_timeout 根因调查（时间戳插桩 + vendored coder/websocket writeControl/
# mu.lock 源码级定位）约 10min，产出固化为 Phase 13 裁决材料。
actuals:
  tokens: 10479
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: [] # 零新依赖红线保持（T-12-SC）：Node 内建模块（net/crypto/fetch/WebSocket）零安装，package.json/pnpm-lock 零 diff
  patterns:
    - RawStallClient 停读夹具（phase05 rawStallClient 一般化：ping 自动回 pong 保持协议栈存活语义 + close 帧 echo 礼貌收口 + 合并大消息 64 位长度支持）
    - 洪水收齐终态联合形态信号（'3999999\r\n4000000\r\n'——回显行不可能包含；恢复期发 INPUT 标记会因 tty 回显与洪水共用输出流而破坏连续性校验面）
    - /healthz clients 轮询踢出观测（只读 HTTP 不打扰 WS stall 面，12-03 纪律的 UAT 首次落地）
key-files:
  created:
    - web/uat/phase12.mjs
  modified: []

key-decisions:
  - "S6 场景形态裁决（Rule 3，执行期实证驱动）：默认 --ping-interval=5s 下 dwell 1013 被 1006 pong_timeout 结构性先杀——coder/websocket writeControl 内层 5s 写超时（write.go:277-279）使 ping tick 落在 writer 持锁阻塞于满 TCP 的窗口时 mu.lock 超时返回 DeadlineExceeded，被 pinger 单一 errors.Is 判读误认为 pong 超时（实测 detach 恰于 attach+10.0007s = tick+5.0007s，reason=pong_timeout）。S6 以生产 CLI flag --ping-interval=0（D-16「0 = 禁用保活」公开契约，Go harness PingInterval 零值同构）隔离 dwell 看门狗；dwell 本身生产 10s 零覆写真实等待（两轮实测 10.6s/10.4s）。默认配置竞态取舍归 Phase 13"
  - "洪水量修正（Rule 3）：plan 文本「seq 1 400000 级，超 outbox 512KiB 即足量」与 TCP 吸收带事实不符——loopback 单连接吸收 ≈ wmem 4MiB + rmem 6MiB + outbox 512KiB + PTY 64KiB ≈ 10.6MiB（slowclient_test.go:8-11 纪律），2.7MB 洪水下停读永不形成、场景空转假绿；上调至 seq 1 4000000（30.9MB ≈ 3× 吸收带，Go seqFlood Linux 分支同款），停读态由管线物理上限保证形成"
  - "S5 恢复期零输入纪律：恢复后不发任何 INPUT 直至洪水收齐——tty 回显与洪水输出共用 PTY 输出流，恢复期发标记会中途插入回显字节破坏连续性校验面；收齐信号 = 尾窗终态联合形态，POST 存活探针在收齐后发送（回显字节落在已校验洪水之后零污染）"
  - "S4a 零回显断言形态：断标记零命中 + 连接存活而非零输出字节——bash 启动提示符晚到（500ms 落定窗后仍可能在途）使零字节断言有假阳面；INPUT 若通过 mode 闸则回显必含标记串，标记零命中即覆盖「零回显」语义"
  - "S1/S2/S4 子进程形态 bash --norc --noprofile（phase05 先例）：plan 文本「-- bash」直落会读 ~/.bashrc 产生噪音；S3/S6 经 bash -c / sh 的 exec 数组传递不经 shell 拼接（plan action 3 纪律）"
  - "PC-05/06/07/10/11 五需求勾选按既定先例统一留 12-05（ID 跨 12-01..05 共享——12-01/02/03 SUMMARY 先例延续）"

patterns-established:
  - "RawStallClient 类形态（构造即握手可复用）：手构 Upgrade + masked 短帧 + 扩展长度解析 + ping 自动 pong + close echo——后续停读/慢客户端 UAT 场景的即用夹具"
  - "writeControl 5s 写超时 × pinger DeadlineExceeded 单一判读的交互链：任何「ping 写不出去持续 5s」的连接（writer 持锁阻塞于满 TCP）都会被 pinger 以 pong_timeout 收口——Phase 13 pinger/dwell 竞态裁决与 herdr 客户端行为评估的实证基线"
  - "洪水收齐终态联合形态信号（相邻末两行联合锚）：大流量场景「收齐」判定的回显免疫形态，后续 UAT 洪水场景复用"

requirements-completed: []  # PC-05/06/07/10/11 跨 plan 共享，按既定先例留 phase 末 12-05 勾选

coverage:
  - id: S1
    description: "Welcome.session 双模式对照：per-client 实例收 \"per-client\"、shared 实例收 \"shared\"，mode/cols/rows 既有键同帧共存（D-08 协议面端到端）"
    requirement: PC-06
    verification:
      - kind: uat
        ref: "web/uat/phase12.mjs#S1a/S1b（两轮全绿）"
        status: pass
    human_judgment: false
  - id: S2
    description: "resize 直通双端隔离：A RESIZE(120,50)→stty \"50 120\"、B 仍 Hello 尺寸 \"28 90\"；双端 attach Welcome 后零 'W' 帧（PC-05 SC1）"
    requirement: PC-05
    verification:
      - kind: uat
        ref: "web/uat/phase12.mjs#S2a/S2b/S2c（两轮全绿）"
        status: pass
    human_judgment: false
  - id: S3
    description: "ro RESIZE 直通：per-client ro 端 RESIZE(133,55)→SIGWINCH trap 回读 \"55 133\" 到达 OUTPUT；shared 同操作对照零新输出（D-06/D-09 第二闸 shared 逐字保留）"
    requirement: PC-05
    verification:
      - kind: uat
        ref: "web/uat/phase12.mjs#S3a-S3d（两轮全绿）"
        status: pass
    human_judgment: false
  - id: S4
    description: "ro INPUT 丢弃（零回显零断开）+ rw 限速保留（120×15KiB/2.4s 超速率 flood 不踢不断 + 探针回显，RES-02 drop 语义）"
    requirement: PC-07
    verification:
      - kind: uat
        ref: "web/uat/phase12.mjs#S4a-S4c（两轮全绿）"
        status: pass
    human_judgment: false
  - id: S5
    description: "停读期输出不丢、恢复后完整到达：3s 停读窗（≪ dwell 10s）→ 恢复读后 seq 1..4000000 字节级严格 +1 连续无缺口（34.9MB 校验）+ 连接存活无 1013 + B 全程不受影响"
    requirement: PC-11
    verification:
      - kind: uat
        ref: "web/uat/phase12.mjs#S5a/S5b/S5c（两轮全绿）"
        status: pass
    human_judgment: false
  - id: S6
    description: "真实 10s+ dwell 到期 → 1013 slow_consumer 端到端：生产 defaultSlowDwell=10s 零覆写真实等待（实测 10.6s/10.4s）→ CloseError 1013 机器串逐字 → pollESRCH 进程组收割（PC-03 挂点联动）"
    requirement: PC-10
    verification:
      - kind: uat
        ref: "web/uat/phase12.mjs#S6a-S6d（两轮全绿，--ping-interval=0 隔离裁决见 key-decisions）"
        status: pass
    human_judgment: false

# Metrics
duration: 22min
completed: 2026-09-04
status: complete
---

# Phase 12 Plan 04: 协议层 UAT 六场景 Summary

**phase12.mjs（754 行，phase11.mjs 同构骨架 + RawStallClient raw socket 停读夹具）以真实二进制在真实协议面上锁定 Phase 12 五需求 wire 行为——Welcome.session 双模式、resize 直通隔离、ro 双闸配对、ro 门控+限速、停读续读 34.9MB 字节级连续、真实 10s+ dwell 1013；两轮连跑 20/20 退出码 0**

## Performance

- **Duration:** 22 min（Started 2026-09-04T14:33:40Z / Completed 14:56Z）
- **Tasks:** 2（Task 1 骨架+S1-S4 / Task 2 S5-S6+收口）
- **Files modified:** 1（web/uat/phase12.mjs 新建）
- **单轮运行:** ~26.6s（六场景 + 300ms 场景间隔 + S5 洪水收齐 + S6 真实 dwell 等待）

## Accomplishments

- **骨架逐字继承 phase11.mjs**：头注释（PC-05/06/07/10/11 六场景清单 + 红线 + 时序纪律 + 运行方式四件）、check/skip 双通道 + emittedDetails、startWesh（--bind 127.0.0.1 --port 0 + stdout 两行解析 + 50ms 落定窗 + SIGKILL 收口）、dialHello（Welcome 到达=握手完成 + 10s watchdog）、帧工具（帧常量区注释补 Welcome session 键互指——proto.go D-08/D-16 同款）、assertOutputClean 运行时自净逐字保留
- **S1-S4 四场景**（Task 1）：Welcome.session 双模式对照（D-08 additive 不挤压既有键）/ resize 直通双端隔离（A "50 120" B "28 90" + 全程零 'W' 帧）/ ro RESIZE 直通（SIGWINCH trap 回读 "55 133" + shared 对照零新输出——D-09 第二闸逐字保留）/ ro INPUT 丢弃 + rw 限速保留（120×15KiB 超速率 flood 不踢不断 + '#' 注释载荷经 "\r" 收口防探针误丢——12-02 两步纪律）
- **S5 停读续读不丢**（Task 2）：RawStallClient 停读 3s（≪ dwell 10s）+ seq 1 4000000 洪水（30.9MB ≈ 3× TCP 吸收带）→ 恢复读后 **34,888,896 字节算术步进严格 +1 连续无缺口**（Go assertSeqContinuity 同语义的字节级强化）+ 连接存活无 1013 + B 全程 echo 照常——PC-11 停读期输出不丢的端到端证据
- **S6 真实 dwell 到期 1013**（Task 2）：生产 defaultSlowDwell=10s 零覆写零测试钩子真实等待（两轮实测 10.6s/10.4s）→ /healthz clients 归零轮询检测踢出（12-03 只读 HTTP 观测纪律）→ 恢复读收 **CloseError 1013 slow_consumer 机器串逐字**（writeFrameMu 解锁后 writeClose 补发——clients.go 关闭帧可达性不变量）→ pollESRCH 进程组收割复核（PC-03 挂点联动）
- **执行期实证发现（S6 根因调查产出）**：默认 --ping-interval=5s 下，coder/websocket writeControl 内层 5s 写超时使 ping tick 落在 writer 持锁阻塞于满 TCP 的窗口时以 DeadlineExceeded 返回，被 pinger 单一判读误认为 pong 超时——TCP 级停读客户端在 (停读+5s, 停读+10s] 被 1006 pong_timeout 先杀，dwell 1013 结构性后到（实测 detach 恰于 attach+10.0007s）。Go 侧 12-03 四测未暴露（harness PingInterval 零值即禁用）。真实浏览器端网络栈自动回 pong 不触发该路径；herdr 类自管 socket 客户端可触发——Phase 13 裁决材料（STATE Blockers 登记）
- **两轮连跑基线一致**：20/20 全绿退出码 0 ×2（11-06 收口两轮基线先例）；SEC 输出自净 19 detail 零 token/pid/'/s/' 命中

## Task Commits

Each task was committed atomically:

1. **Task 1: phase12.mjs 骨架 + S1-S4（Welcome.session 双模式 / resize 隔离 / ro RESIZE 直通 / ro INPUT+rw 限速）** - `a1c2598` (test)
2. **Task 2: S5-S6 停读续读不丢 + 真实 dwell 1013 + 全脚本收口** - `2bcb40c` (test)

**Plan metadata:** docs commit（本 SUMMARY + STATE/ROADMAP 更新）

## Files Created/Modified

- `web/uat/phase12.mjs` - 新建 754 行：六场景 + RawStallClient（手构 WS 升级 + RFC6455 masked 组帧 + 16/64 位扩展长度解析 + ping 自动回 pong + close 帧 echo + pause/resume 停读/续读表达）+ seqContinuity 字节级算术步进 + /healthz clients 轮询观测

## Decisions Made

（全部登记于 frontmatter key-decisions，此处列要点）

- **S6 --ping-interval=0 隔离裁决**：见 Accomplishments 发现段——dwell 本身生产 10s 零覆写；ping 是正交保活子系统，禁用经公开 CLI flag（D-16）非测试钩子，与 Go harness 零值形态同构
- **洪水量从 plan 文本 400000 上调至 4000000**：2.7MB < 吸收带 10.6MiB → 停读永不形成 → 场景空转假绿（无判别力的通过比失败更危险）；30.9MB 使停读态由 TCP 物理上限保证形成
- **S5 恢复期零输入纪律 + 终态联合形态收齐信号**：tty 回显与洪水共用 PTY 输出流——恢复期发标记的回显字节会插进洪水中间破坏校验面
- **S4a 断标记零命中而非零输出字节**：prompt 晚到假阳面免疫

## TDD Gate Compliance

- Task 1（tdd=true，断言收口形态）：S1-S4 对 12-01/02/02 已落地实现编写（机制先于本 plan 落地），首跑 13/13 全绿——12-02 Task 3 / 12-03 Task 2 断言收口先例延续（断言收口形态下 RED 由机制落地前的 Go/构造证据承载，本 plan 的判别力证据 = S6 修正过程中 FAIL→PASS 双态实测）
- Task 2（tdd=true）：RED 实测——S6 首跑 S6b/S6c FAIL（dwellWait 9.97s < 10s + close 未到达）驱动根因调查（pong_timeout 先杀发现）→ 形态修正（--ping-interval=0）→ GREEN 20/20；S5 首跑即绿（字节级连续校验判别力由 34.9MB 全量步进承载）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3] S6 场景形态修正：默认 ping 下 dwell 1013 被 pong_timeout 1006 先杀**
- **Found during:** Task 2（S6 首跑 S6b/S6c FAIL）
- **Issue:** coder/websocket writeControl 内层 5s 写超时（write.go:277-279）+ pinger 单一 errors.Is(DeadlineExceeded) 判读——默认 --ping-interval=5s 下 TCP 级停读客户端在 (停读+5s, 停读+10s] 被 1006 pong_timeout 先杀，plan 预期的 dwell 1013 结构性不可达（plan 未预见 pinger 交互；Go 测试 harness PingInterval 零值未暴露）
- **Fix:** S6 实例加生产 CLI flag --ping-interval=0（D-16「0 = 禁用保活」公开契约）隔离 dwell 看门狗；dwell 生产 10s 零覆写真实等待保持
- **Files modified:** web/uat/phase12.mjs（S6 场景函数 + 裁决依据注释）
- **Commit:** 2bcb40c

**2. [Rule 3] 洪水量修正：plan 文本「seq 1 400000 级」不足以形成停读态**
- **Found during:** Task 2 设计期（吸收带核算）
- **Issue:** 400000 级 ≈ 2.7MB < loopback 单连接吸收带 ~10.6MiB（wmem 4MiB + rmem 6MiB + outbox 512KiB + PTY 64KiB，slowclient_test.go:8-11 纪律）——outbox 永不涨满、停读永不形成，S5/S6 将空转假绿
- **Fix:** seq 1 4000000（30.9MB ≈ 3× 吸收带，Go seqFlood Linux 分支同款）
- **Files modified:** web/uat/phase12.mjs
- **Commit:** 2bcb40c

**3. [Rule 1] S4a「零回显」断言的假阳面免疫**
- **Found during:** Task 1 设计期
- **Issue:** 若断「静默窗零输出字节」，bash 启动提示符晚到（dialHello 后 >500ms 在途）会假阳失败——提示符输出与 INPUT 丢弃语义无关
- **Fix:** 断标记零命中（INPUT 若通过 mode 闸则回显必含标记串）+ 连接存活；500ms 提示符落定窗后记基线
- **Files modified:** web/uat/phase12.mjs
- **Commit:** a1c2598

## Issues Encountered

- S6 首跑双 FAIL → 时间戳插桩诊断脚本（scratch /tmp，未入提交）+ vendored coder/websocket 源码定位（writeControl/mu.lock/pinger 三点对照）→ detach 事件 reason=pong_timeout 定位根因，~10min；诊断件已删除
- 全脚本零 skip（六场景全部可执行——无平台豁免面）

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- **12-05（收口）**：PC-05/06/07/10/11 勾选证据链全齐——Go 测（12-01 协议面 / 12-02 五测 / 12-03 四测）+ jsdom（phase12-dom D1/D2/D3）+ 协议层（本脚本六场景两轮基线）
- **Phase 13**：pinger/dwell 竞态裁决材料就绪（writeControl 5s 写超时交互链 + 默认配置 1006 先杀实测）——建议裁决面：pinger 区分「写阻塞超时」与「pong 等待超时」（lib 错误链可区分：failed to acquire lock vs failed to wait for pong）或接受 1006 语义（死连接更早收口）；herdr 客户端评估该路径
- **Phase 14**：herdr E2E 收编时 RawStallClient 可直接复用（停读/慢客户端场景夹具）

## Threat Flags

无新增威胁面——T-12-11（assertOutputClean 逐字保留 + detail 只打布尔/状态码/尺寸常量，SEC 19 detail 零命中两轮实证）、T-12-12（SIGKILL 收口 + 场景间 300ms + S6 显式进程组 SIGKILL 清场 + pollESRCH）、T-12-SC（零新依赖：Node 内建模块零安装，package.json 零 diff）三项 mitigate 处置全部落地验收。

---

*Phase: 12-per-client*
*Completed: 2026-09-04*

## Self-Check: PASSED

- 1/1 关键文件在场（web/uat/phase12.mjs，754 行 ≥ 400 门槛，含 "S6" ×11）
- 2/2 任务提交在场（a1c2598 / 2bcb40c）
- 锚点验证：六场景函数名各 ×1；RawStallClient ×8；两轮连跑 20/20 退出码 0 实测（14:49 与 14:53 两轮日志）；SEC 自净两轮过
