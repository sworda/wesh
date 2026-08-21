---
phase: 05-multi-client
plan: 09
subsystem: testing
tags: [uat, node-websocket, raw-socket-stall, share-link, max-clients, slow-consumer, readme, gofmt, multi-client]

# Dependency graph
requires:
  - phase: 05-multi-client plan 05/06/07/08
    provides: 输入限速与 CR-01 背压完整修复（05-05）；share read-only/read-write 启动打印与 /s/{token}/ 门禁 + /api/attach token 分支（05-06）；--max-clients ③位 503 与 /api/attach 早闸（05-07）；前端分派矩阵三专版与升格分支（05-08）
provides:
  - web/uat/phase05.mjs：S1 双客户端逐字节一致 / S2 ro 链接全链 / S3 rw 链接全链 + D-05 总闸负向 / S4 错 token 无 oracle / S5 满员双点位 503 / S6 1013 踢出活跃场景（raw-socket stall 夹具三断言）/ S7 skipped 豁免记录——18/18 通过 + 1 skipped
  - phase02.mjs 场景 5 多客户端改写（T5a 第二连接成功 + T5b 断开后存活可再 attach，断言 11→12 只增）；phase03.mjs S1f/S3d 注释与断言集适配（18/18 守恒）；phase04.mjs S4/S5 osc52 D-13 适配（10/10 守恒）
  - .planning/phases/05-multi-client/05-UAT.md：多客户端人工核对清单七组（review #8 S7 豁免闭环，04-UAT.md 同形态）
  - README 多客户端节：分享链接 + 反代脱敏 nginx 双形态 + 暴露面清单 + write-policy/resize/输入限速/1013/max-clients 超编说明 + Phase 9 标定方法论 + 行为变更明示
  - GOROOT gofmt 清零（02-06/03-06 先例独立 style 提交）——六段式段 1 恢复零差异
affects: [/gsd:verify-work 阶段验收（VALIDATION full suite 全绿证据）, Phase 6 CORE-05（1013 手动刷新边界确认）, Phase 8 OPS-07（五处计数器 stub 清单交接）, Phase 9（标定方法论与负载矩阵）]

# Actuals (#2632)
actuals:
  tokens: 16560   # 三提交 realized diff 66242 chars / 4（estimate 50000，confidence: low 命中——gofmt 清零与 phase04 适配不在原估计面）
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "raw-socket stall 夹具（本仓 UAT 首例）：net.Socket 手工 WS 握手（读至 \\r\\n\\r\\n 收 101）+ masked Hello 单帧（0x82 + mask 位 + 4 字节 mask XOR 载荷，<126 字节短形）+ socket.pause() 内核级 stall——Node WebSocket（undici）持续 drain TCP 结构性无法制造 stall，必须 raw socket"
    - "startWesh stdout 三行解析 + 落定窗：listening on + share read-only:（恒打印）齐备后 50ms 落定吸纳 rw 行管道分块；stderr 持续捕获缓冲暴露 stderrText() 供 logEvent 断言轮询"
    - "throttle 窗口与 token 通道的排序纪律：capability 分支（/s/ 有效 token GET、/api/attach token peek）绕过 throttle 无需 pacing；401 负面对照产生 fail#1 +1s 窗口且 checkTicket 同经该闸——负面对照必须排在全链断言之后（本 plan 实测命中）"
    - "多客户端 UAT 适配守恒律：『拒绝/退出』断言改写为『存活、可再 attach』断言，数量只增不减（phase02 11→12、phase03 18→18、phase04 10→10）"

key-files:
  created:
    - web/uat/phase05.mjs
    - .planning/phases/05-multi-client/05-UAT.md
  modified:
    - web/uat/phase02.mjs
    - web/uat/phase03.mjs
    - web/uat/phase04.mjs
    - README.md
    - cmd/wesh/main.go（gofmt 清零，零语义）
    - internal/server/{clients,e2e_test,multi_test,resize,resize_arb_test,server,slowclient_test}.go + internal/pty/io.go（gofmt 清零，零语义）

key-decisions:
  - "[Phase 05-09]: S2d 401 负面对照排全链断言之后——checkTicket 经 throttle 同一 per-IP 闸，401 负面对照产生的 fail#1 +1s 窗口会使后续 Hello 携票核销撞窗收 auth_failed（S3c 实测命中）；token 分支本身绕过 throttle（R-03 capability 语义），故排序即解，无需 pacing sleep"
  - "[Phase 05-09]: phase04.mjs S4/S5 osc52 断言适配 D-13——05-03 prefs 双档后 ro 端不再下发 osc52，旧断言（ro 会话断言 osc52 存在）结构性失败；spawn 加 --writable 改在 rw 端断言下发通道，断言面守恒（键存在性与值等式不变）；plan <files> 未列 phase04.mjs 但 prohibitions 已含其适配条款，六段式四脚本全过为硬约束"
  - "[Phase 05-09]: S6 洪水量 seq 1 3000000（约 20MB，plan 字面）→ seq 1 50000000（约 389MB）——05-07 实测裁决量级：踢出触发点 = stall 端管道 ~10MiB 最坏吸收 + outbox 512KiB，20MB 在 pre-attach drain 不确定量下裕度不足；389MB 提供数量级余量防 lifecycle 1000 与 Close(1013) 竞态"
  - "[Phase 05-09]: GOROOT gofmt 清零 9 文件（纯注释排版/import 序，逐行核读零语义）——02-06/03-06 先例第三次沿用，独立 style 提交；HEAD 漂移系 /usr/bin/gofmt 陈旧版 CJK 注释规则差异（01-03 已登记事项），随本收口 plan 六段式一并清零"

patterns-established:
  - "UAT 层 1013 活跃断言三件套：stderr logEvent 轮询（code=1013 reason=slow_consumer，10s 窗）+ 第二正常客户端字节计数单调增长（踢出后两窗口）+ stall 端 resume 后 10s 内 end/close——『Go 测试已覆盖』不构成 UAT 层豁免（review #8 层级论证）"
  - "skipped 豁免记录形态：results 以 ok=null 三态区分 skip（不计失败不计通过），汇总行显式列 skipped 项数；reason 串指向人工清单文件完成缓解闭环"

requirements-completed: [MULTI-01, MULTI-03, MULTI-05, RES-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "phase05.mjs 场景矩阵：S1 双客户端 OUTPUT 逐字节一致（338958 字节量）；S2/S3 ro/rw 链接全链（GET 200 无 challenge → POST 出 ticket → Welcome mode 绑定）+ S2d Basic 矩阵负面对照 + S3d D-05 总闸负向；S4 错 token 401 challenge + 无 oracle 形状断言；S5 满员双点位 503；S6 1013 踢出三断言（stderr 事件 + 他人推进 + resume 终结）"
    requirement: MULTI-05
    verification:
      - kind: e2e
        ref: "node web/uat/phase05.mjs（18/18 协议断言通过 + 1 skipped，两连跑稳定，exit 0）"
        status: pass
    human_judgment: false
  - id: D2
    description: "前序 UAT 多客户端适配零断言丢失：phase02 场景 5 改写（12/12，11→12 只增）、phase03 S1f/S3d 适配（18/18 守恒）、phase04 S4/S5 D-13 适配（10/10 守恒）"
    requirement: MULTI-01
    verification:
      - kind: e2e
        ref: "node web/uat/phase02.mjs && phase03.mjs && phase04.mjs（全 exit 0）"
        status: pass
    human_judgment: false
  - id: D3
    description: "README 多客户端节：分享链接用法 + 反代脱敏 nginx 双形态示例（access_log off / map \$uri \$sanitized_uri + log_format）+ 暴露面清单（PCAP/屏幕共享/桌面索引 ≥2）+ write-policy/max-clients（≥2）+ 32KiB 输入限速 + 瞬时超编 + Phase 9 标定段 + 断开不再使服务端退出行为变更"
    requirement: MULTI-05
    verification:
      - kind: other
        ref: "acceptance grep 九条全过（write-policy|max-clients=8、access.*log|脱敏=4、access_log|log_format=3、PCAP|屏幕共享|桌面=3、tls-cert=6、32KiB=3、超编=1、标定=3、断开不再使服务端退出=2）"
        status: pass
    human_judgment: false
  - id: D4
    description: "RES-03 满员 503 双点位协议层端到端（/api/attach 早闸 + WS ③位 Accept 前 503）"
    requirement: RES-03
    verification:
      - kind: e2e
        ref: "web/uat/phase05.mjs#S5（--max-clients 1 spawn，S5a/S5b/S5c 三断言）"
        status: pass
    human_judgment: false
  - id: D5
    description: "MULTI-03 1013 慢消费者踢出 UAT 层活跃断言（raw-socket stall 夹具）"
    requirement: MULTI-03
    verification:
      - kind: e2e
        ref: "web/uat/phase05.mjs#S6（S6a stderr code=1013 reason=slow_consumer 命中 / S6b 窗口 8.7MB<32.2MB<56.6MB 单调增长 / S6c resume 终结）"
        status: pass
    human_judgment: false
  - id: D6
    description: "多客户端像素层渲染一致性（MULTI-01 渲染层）：浏览器多端逐屏一致 + 六组关联人工核对"
    requirement: MULTI-01
    verification:
      - kind: manual_procedural
        ref: ".planning/phases/05-multi-client/05-UAT.md 七组清单（外部浏览器执行）；phase05.mjs S7 skipped+reason 记录在案"
        status: unknown
    human_judgment: true
    rationale: "headless 硬约束——本机永不具备浏览器，任何自动化（含 playwright）结构性不可测（CODEBUDDY.md 平台原生行为显式豁免条款）；协议层等价断言已由 phase05.mjs S1b 逐字节一致覆盖，像素层人工核对为缓解闭环"

# Metrics
duration: 37min
completed: 2026-08-21
status: complete
---

# Phase 05 Plan 09: Phase 收口（phase05.mjs 协议层 UAT + 前序适配 + README 多客户端节）Summary

**phase05.mjs 七场景协议层 UAT 落地（分享链接全链/双客户端逐字节一致/错 token 无 oracle/满员双点位 503/1013 raw-socket stall 活跃三断言，18/18 + 1 skipped），phase02/03/04.mjs 多客户端适配零断言丢失（12/12、18/18、10/10），05-UAT.md 七组人工清单完成 S7 豁免闭环，README 多客户端节覆盖 D-03 脱敏 nginx 示例/暴露面清单/输入限速/超编说明/Phase 9 标定方法论/行为变更，GOROOT gofmt 清零后六段式全绿——phase 验收就绪。**

## Performance

- **Duration:** 37 min
- **Started:** 2026-08-20T23:50:46Z
- **Completed:** 2026-08-21T00:27:12Z
- **Tasks:** 2（Task 1 phase05.mjs + Task 2 适配/清单/README/六段式）
- **Files modified:** 15（2 新建：phase05.mjs + 05-UAT.md；4 UAT/README 修改；9 gofmt 清零零语义）

## Accomplishments

- **phase05.mjs 场景矩阵**（VALIDATION 05-02-01 UAT 侧 + 05-02-02 UAT 侧）：S1 双客户端（80x24 rw owner / 132x43 ro 递补，D-07 形态）各自累积收齐同一 seq payload 逐字节一致（338958 字节）；S2 ro 链接全链（GET 200 无 challenge → POST body 携 token 出 ticket → Welcome mode=ro）+ GET / 401 负面对照；S3 rw 链接全链 Welcome mode=rw（owner 空位）+ 无 --writable 实例 stdout 无 rw 行的 D-05 总闸负向；S4 错 token /s/ 401 challenge + /api/attach 错 token 与无 token 401 同文同码无 oracle（throttle 爬梯 pacing 沿用 phase03 纪律）；S5 --max-clients 1 双点位 503（/api/attach 早闸 + WS ③位 rawUpgrade 形态）；**S6 1013 活跃场景**（review #8）——raw net.Socket 手工握手 + masked Hello + socket.pause() 内核级 stall 夹具，三断言全过（stderr logEvent code=1013 reason=slow_consumer 10s 内命中 / 第二客户端踢出后窗口 8.7MB→32.2MB→56.6MB 单调增长 / resume 后 TCP 终结）；S7 skipped+reason 指向 05-UAT.md
- **startWesh 扩展**：stdout 三行解析（listening on + share read-only: 恒打印行齐备 + 50ms 落定窗吸纳 rw 行分块）+ stderr 持续捕获 stderrText() 暴露；token 值只存闭包变量——detail 只打状态码/布尔/形状/字节数（phase04.mjs:6-9 红线沿用，人工核读通过）
- **前序 UAT 适配**（P5-4 UAT 侧，逐处见下表明细）：phase02 场景 5『第二连接 409』改写为 T5a 第二连接成功 + T5b 全断开后存活可再 attach（11→12）；phase03 S1f 独立 spawn 注释改写（纪律保留，理由=节流隔离）+ S3d 收紧为确定性 400（①位子协议预检，Accept 前拒绝不触发会话状态变更）；phase04 S4/S5 osc52 断言加 --writable 适配 D-13 ro 档不下发（10/10 守恒）
- **05-UAT.md 人工清单七组**（review #8 S7 闭环，04-UAT.md 同形态）：双客户端视觉一致/新客首屏（D-11 SIGWINCH）/ro 形态三要素/递补升格/1013 专版（手动刷新链路）/503 专版/无效链接——每项含 expected/steps/note 与协议层等价断言指针，status: draft 待外部浏览器执行
- **README 多客户端节**（九组验收 grep 全过）：分享链接用法（重启即废）+ **反代访问日志脱敏 nginx 双形态示例**（map $uri $sanitized_uri + log_format / location /s/ access_log off——D-03 specifics 锁定项，review #6 具体化）+ **暴露面清单**（浏览器历史含屏幕共享旁观/扩展/桌面索引/AV 扫描/明文 PCAP → 不可信网络务必 TLS，吊销=重启）+ write-policy=owner|all 与递补 + all 模式输入交错不排序承诺 + resize 裁剪行为（重新 attach 恢复）+ **输入限速丢弃语义**（>32KiB/s 或超 64KiB burst 静默丢弃，大粘贴分段——review #5）+ 1013 手动刷新（无自动重连 Phase 6）+ --max-clients 含**瞬时超编 ≤8**（review #7）+ **默认参数与 Phase 9 标定方法论**（review #9，见下节）+ 行为变更明示；同步修正四处陈旧表述（单次语义节→生命周期节 / flags 表两新行 / 启动打印三行 / 关闭码表 1013 启用行 / 默认只读 RESIZE 仲裁语义）
- **六段式全绿**：GOROOT gofmt 零差异（清零后）+ go vet + go test -race -count=1 ./...（server 38.1s 全绿）+ time pnpm -C web build（2.3s，dist mtime 2026-08-21 08:19:52 +0800 本次构建时刻）+ go build + 四 UAT 脚本全 exit 0

## 六段式耗时与产物时间戳

| 段 | 命令 | 结果 | 耗时/时间戳 |
|----|------|------|------------|
| 1 | `test -z "$($(go env GOROOT)/bin/gofmt -l .)"` | 零差异（清零后复验） | <1s |
| 2 | `go vet ./... && go test -race -count=1 ./...` | 全绿 | vet <1s；test 42s（cmd 1.0s / proto 1.0s / pty 2.0s / server 38.1s / web 1.0s） |
| 3 | `time pnpm -C web build` | exit 0 | real 2.262s；dist/index.html mtime 2026-08-21 08:19:52.929 +0800（本次构建时刻验证通过，内容零 diff 未入库变更） |
| 4 | `go build -o /tmp/wesh-uat/wesh ./cmd/wesh` | exit 0 | <1s |
| 5 | `node web/uat/phase05.mjs` | 18/18 + 1 skipped，exit 0 | ~13s（两连跑稳定） |
| 5 | `node web/uat/phase02.mjs` | 12/12，exit 0 | ~25s |
| 5 | `node web/uat/phase03.mjs` | 18/18，exit 0 | ~35s |
| 5 | `node web/uat/phase04.mjs` | 10/10，exit 0 | ~15s |

## 前序 UAT 适配明细（逐处改写对照）

| 文件 | 位置 | 旧形态 | 新形态 | 断言计数 |
|------|------|--------|--------|----------|
| phase02.mjs | 场景 5（T5） | 第二连接握手 → HTTP 409（单客户端门） | T5a 第二连接成功 Welcome(mode=ro) + T5b 全部断开后服务端存活可再 attach | 1→2（11→12 只增） |
| phase03.mjs | S1f 注释 | 『单次语义下同进程第二次 WS 建连不可行』 | 纪律保留，理由改写为节流计数隔离（与单次语义无关） | 不变 |
| phase03.mjs | S3d | `[400, 101, 409]` 任一 | 确定性 `=== 400`（①位子协议预检；409 随单客户端门拆除；注释补『不建 WS 连接不触发会话状态变更』） | 不变（收紧） |
| phase04.mjs | S4/S5 | ro 会话断言 prefs.osc52 下发 | spawn 加 --writable 在 rw 端断言（D-13：ro 档不下发 osc52——05-03 prefs 双档后旧断言结构性失败） | 不变（守恒） |
| phase04.mjs | 头注 :11-12 | 单次语义纪律 | 场景隔离纪律（独立 spawn 仅为零状态干扰） | 不变 |

容量上限断言未丢失——由 phase05.mjs S5 双点位 503 承接（强于原 409 单点）。

## 观测性 stub 清单（review #10 → Phase 8 OPS-07 交接）

五处计数器已在本 phase 各 plan 埋点并就绪，Phase 8 OPS-07 直接接线进 metrics 即可，无需新增采集点：

| 计数器 | 位置 | 语义 | 形态 | 消费方式（Phase 8） |
|--------|------|------|------|---------------------|
| `registry.kicks` | internal/server/clients.go:255（递增 :474） | 1013 慢消费者踢出累计次数 | hubMu 内 plain int（R-07 单锁纪律） | metrics 导出踢出速率/总量；标定验收『合法慢端零误踢』的直接证据源 |
| `registry.gateTransitions` | clients.go:259（递增 :414/:445） | 全局信用门开闭周期计数（置位 creditBlocked 与 afterDrain 清位各递增） | hubMu 内 plain int | 门开闭频率——标定验收第三维度；高频震颤 = resume 水位/容量待调 |
| `s.inputDrops` | internal/server/server.go:103（递增 :753） | 每服务端输入限速丢弃帧数（rate.Limiter AllowN 拒绝） | atomic.Int64（INPUT 门热路径无锁） | 限速触发面观测；合法用户粘贴被误伤的告警源 |
| `inputQ.droppedInputs` | clients.go:191（递增 :207，tryEnqueue 内自含记账） | 会话级输入队列满丢弃帧数（CR-01 背压完整修复的丢弃臂） | atomic.Int64 | 队列容量裕度观测；限速器在前故稳态应≈0，非零即异常信号 |
| `registry.n` | clients.go（registerLocked +1 / removeLocked -1 唯一收口点） | 当前注册客户端数（R-06 口径） | atomic.Int64（③位闸 hubMu 外 load 故须 atomic） | 在线连接数 gauge；容量规划与超编观测 |

## Phase 9 标定方法论记录（review #9）

README『默认参数与 Phase 9 标定』节已落地用户面版本；本节选录 executor 侧完整口径：

- **五项初值（一阶推算，非实测）**：outbox 512KiB/客户端（16×32KiB 读块；100KB/s 链路 ~5s 抖动容忍；32 客户端账面最坏 16MiB 共享帧实占更低）/ 信用门恢复水位 50%（迟滞防震颤）/ 输入 32KiB/s + burst 64KiB（击键 ~10B/s、快粘 ~50KB 瞬时）/ --max-clients 32（团队围观区间下沿）/ resize 防抖 50ms（SIGWINCH 风暴防线）。
- **标定方法 = 负载矩阵**：客户端数（1 / 4 / 16 / 32）× 输出速率（行内 / 持续洪水 / 突发）× 慢链路注入（无 / 100KB/s / 极端 stall）三维度笛卡尔积。
- **验收标准**：① 合法慢端零误踢（100KB/s 慢链路客户端在抖动窗内不被 1013）；② 内存上界成立（N 客户端总驻留 ≤ N×512KiB + 共享帧实际滞后量）；③ 信用门开闭频率可接受（无高频震颤——gateTransitions 增速有界）。
- **数据源**：上节五处计数器 + pinger/outbox 既有事件流；采集基础设施 = Phase 8 OPS-07 metrics 导出。
- **滴漏振荡评估**（RESEARCH Open Question 3 裁决挂账）：连续信用阻塞 >30s 是否加 dwell 计时器也踢——随标定数据一并评估。

## Task Commits

Each task was committed atomically:

1. **Task 1: web/uat/phase05.mjs（S1-S7 全场景）** - `e446475` (test)
2. **Task 2a: GOROOT gofmt 清零（9 文件零语义，02-06/03-06 先例独立 style 提交）** - `0e3be88` (style)
3. **Task 2b: phase02/03/04.mjs 适配 + 05-UAT.md + README 多客户端节** - `27adcac` (feat)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `web/uat/phase05.mjs`（新，450 行）- 七场景；startWesh 三行解析+stderr 缓冲；dialHello（ticket/尺寸参数化）/rawUpgrade/rawStallClient 夹具；三态 check/skip 汇总
- `web/uat/phase02.mjs` - 场景 5 改写 scenarioMultiClient（T5a/T5b）
- `web/uat/phase03.mjs` - S1f/S3d 注释与断言集适配
- `web/uat/phase04.mjs` - S4/S5 --writable 适配 + 头注纪律更新
- `README.md` - 多客户端节（分享链接/脱敏/暴露面/write-policy/resize/限速/1013/容量/标定/行为变更）+ 生命周期节改写 + flags 表两行 + 关闭码表 1013 行 + 默认只读 RESIZE 语义
- `.planning/phases/05-multi-client/05-UAT.md`（新）- 七组人工核对清单
- 9 个 .go 文件 - gofmt 清零（纯注释排版/import 序，零语义）

## Decisions Made

见 frontmatter key-decisions 四条（S2d throttle 窗口排序 / phase04 D-13 适配 / S6 洪水 389MB 量级 / gofmt 清零先例沿用）——全部经实测驱动并留有断言证据。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] S2d 负面对照撞 throttle 窗口致 S3c auth_failed**
- **Found during:** Task 1（phase05.mjs 首跑：S3c 握手被关闭 code=1008 reason=auth_failed，16/16 断言全过但场景异常 exit 1）
- **Issue:** S2d GET / 无 token 401（fail#1）产生 +1s throttle 窗口；checkTicket 同经该 per-IP 闸——紧随其后的 S3c Hello 携票核销撞窗收 auth_failed。token 分支（S3a/S3b）绕过 throttle（R-03 capability 语义）不受影响，唯有 WS 侧 ticket 核销过闸
- **Fix:** S2d 负面对照移至同实例全链断言（S2a-c/S3a-c）之后——排序即解，零 pacing sleep 引入；注释登记排序理由
- **Files modified:** web/uat/phase05.mjs
- **Verification:** 复跑 18/18 + 1 skipped exit 0，两连跑稳定
- **Committed in:** e446475

**2. [Rule 1 - 夹具可靠性] S6 洪水量 20MB → 389MB（05-07 实测裁决量级沿用）**
- **Found during:** Task 1 设计推演（落码前量级核算）
- **Issue:** plan 字面 `seq 1 3000000`（约 20MB）：pre-attach drain 量不确定（D-12 无 ring，spawn 至双客户端就位期间产出全部丢弃）+ 踢出触发点本身需 ~10MiB 管道吸收 + 512KiB outbox——20MB 裕度不足 2×，且 05-07 已实测 38.9MB 量级下子进程先耗尽致 lifecycle 1000 与 Close(1013) 竞态
- **Fix:** 洪水取 05-07 裁决量级 `seq 1 50000000`（约 389MB，数量级余量）；注释登记裁决出处
- **Files modified:** web/uat/phase05.mjs
- **Verification:** S6 三断言两连跑稳定（踢出点窗口 8.7MB/6.0MB 处命中，远早于洪水耗尽）
- **Committed in:** e446475

**3. [Rule 3 - Blocking] phase04.mjs S4/S5 osc52 断言 D-13 结构性失败**
- **Found during:** Task 2 六段式段 5（phase04.mjs 8/10——S4/S5 断言 prefs.osc52===true 失败，keys 为空/仅 fontSize）
- **Issue:** 05-03 prefs 双档（D-13 旁观端 osc52 强制关）落地后，ro 会话不再下发 osc52——phase04 旧断言（ro 会话断言下发）结构性失败；plan <files> 未列 phase04.mjs，但 prohibitions 明含『phase02/03/04.mjs 适配不得削弱原断言面』且六段式要求四脚本全过
- **Fix:** S4/S5 spawn 加 --writable，osc52 下发通道断言改在 rw 端进行（键存在性与值等式不变——断言面守恒）；头注单次语义纪律同步更新；适配明细见上表
- **Files modified:** web/uat/phase04.mjs
- **Verification:** phase04.mjs 10/10 exit 0
- **Committed in:** 27adcac

**4. [Rule 3 - Blocking，先例授权路径] GOROOT gofmt 段 1 九文件预存漂移清零**
- **Found during:** Task 2 六段式段 1（gofmt -l 输出 9 文件——05-07 Issues 已实测登记的 /usr/bin/gofmt 陈旧版 CJK 注释规则差异，HEAD 预存，非本 plan 引入）
- **Issue:** 段 1 零差异是 plan must_have 硬约束；预存漂移使六段式结构性不过
- **Fix:** 02-06/03-06 先例第三次沿用——GOROOT gofmt -w 九文件，逐行核读全为注释排版（//（→// （）/尾注释对齐/注释块空行/import 字母序归位，零语义；独立 style 提交
- **Files modified:** cmd/wesh/main.go、internal/pty/io.go、internal/server/{clients,e2e_test,multi_test,resize,resize_arb_test,server,slowclient_test}.go
- **Verification:** 清零后 `gofmt -l .` 输出为空；go vet + go test -race -count=1 ./... 全绿（server 38.1s）
- **Committed in:** 0e3be88

---

**Total deviations:** 4 auto-fixed（2 Rule 1 - Bug/夹具可靠性，2 Rule 3 - Blocking——其一为先例授权路径）
**Impact on plan:** 四处全部服务于 plan 自身锁定的验收（六段式全绿、S6 活跃三断言、断言面守恒）；plan 的机制形态（场景矩阵/适配纪律/README 条目/grep 验收点）全部逐字保持。phase04.mjs 列入适配面是 prohibitions 明文授权与六段式硬约束的机械调和。

## Known Stubs

None — 本 plan 无新增占位 stub（无硬编码空值/占位文案/TODO；全部 verify 均已运行；S7 skipped+reason 是 headless 豁免的设计形态而非 stub——缓解闭环 05-UAT.md 已落地）。既有挂账项保持：上节五处计数器（Phase 8 OPS-07 消费，本次以交接清单形式显式化）+ permission_denied 占位注释。

## Threat Model 处置

| Threat ID | 处置 | 证据 |
|-----------|------|------|
| T-05-06c（token 经测试输出/README 泄露，high） | **mitigate 已落地** | phase05.mjs 红线注释逐字沿用（token 只存闭包变量，detail 只打状态码/布尔/形状/字节数——人工核读通过）；README 示例链接全部占位符（<ro-token>/<rw-token>/<redacted>）；两连跑输出复核零 token 字面 |
| T-05-06d（反代访问日志记录 /s/{token}/ 路径，medium） | **mitigate 已落地** | README 反代脱敏节含 nginx 双形态具体示例（map $uri $sanitized_uri + log_format 引用 / location /s/ access_log off）+ 暴露面清单 + TLS 部署建议 + 吊销=重启——D-03 specifics 锁定项与 review #6 具体化全部兑现，acceptance grep 断言通过 |

无新增威胁面——本 plan 为验证与文档收口：无新端点/新协议帧/新依赖；UAT spawn 实例全部 loopback 随机端口，测试凭据为仓内既有 UAT 常量。

## Issues Encountered

- **S2d/S3c throttle 窗口排序**与 **phase04 osc52 结构性失败**（见 Deviations 1/3）——均首跑即捕获、修正后稳定。
- 无其他问题；Go 侧零生产代码改动。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的后续挂点）

- **/gsd:verify-work**：phase 验收就绪——VALIDATION full suite command 全绿（本 SUMMARY 六段式表）；05-UAT.md 七组人工清单待外部浏览器执行（S7 豁免闭环的最后一环，status: draft）。
- **Phase 6 CORE-05**：自动重连完整能力——README/05-UAT.md 均已明示当前手动刷新边界（D-10）。
- **Phase 8 OPS-07**：五处计数器交接清单（上节表格）直接接线。
- **Phase 9 标定**：负载矩阵 × 验收标准 × 数据源已锁定（上节）；defaultMaxClients=32 等五项初值随标定回填。

## Next Phase Readiness

- phase 验收（/gsd:verify-work）全部前置条件满足：phase05.mjs 全过 + phase02/03/04.mjs 回归全过（VALIDATION.md phase gate 条件字面达成）
- README 使用户无需读代码即可正确使用分享链接与权限模式，并知晓反代日志暴露面与全部行为变更（success_criteria 第二条达成）
- 无阻塞项

## Self-Check: PASSED

- FOUND: web/uat/phase05.mjs（`socket.pause()` == 3 ≥ 1；`Sec-WebSocket-Protocol` == 3 ≥ 1；`share read-only:` == 3 ≥ 1；两连跑 18/18 + 1 skipped exit 0）
- FOUND: .planning/phases/05-multi-client/05-UAT.md（七组清单关键词 grep == 7 ≥ 7）
- FOUND: README.md（write-policy|max-clients == 8 ≥ 2；access.*log|脱敏 == 4 ≥ 1；access_log|log_format == 3 ≥ 1；PCAP|屏幕共享|桌面 == 3 ≥ 2；tls-cert == 6 ≥ 1；32KiB == 3 ≥ 1；超编 == 1 ≥ 1；标定 == 3 ≥ 1；断开不再使服务端退出 == 2 ≥ 1）
- FOUND: commit e446475（Task 1）、0e3be88（gofmt style）、27adcac（Task 2）均在 git log；三提交均无意外文件删除（--diff-filter=D 检查通过）
- 六段式复验：GOROOT gofmt -l 输出为空；go vet exit 0；go test -race -count=1 ./... 全绿（server 38.1s）；time pnpm -C web build exit 0（2.262s）且 dist mtime 2026-08-21 08:19:52 +0800 为本次构建时刻；phase05/02/03/04.mjs 依次 exit 0（18/12/18/10）

---
*Phase: 05-multi-client*
*Completed: 2026-08-21*
