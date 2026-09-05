---
phase: 13-resource-defense
plan: 07
subsystem: protocol-uat-load
tags: [phase13-mjs, churn-throttle, kill-backstop, exit-255, shutdown-ngroups, metrics-21, wesh-remote-user, load-churn, PC-08, PC-09, SEC-09, OPS-12]
requires:
  - "13-01 D-01/D-02 stop-timeout 双默认值（per-client 未设默认 5s + 显式 0 尊重+warn——S2 两半场的行为基座）"
  - "13-02 spawn 双令牌桶 + 拒绝序列 1011（S1 churn 节流断言对象 + churn 格 throttled_total>0 证据源）"
  - "13-03 pcSupervisor/last-reaped-code/session_end 审计（S3 退出 255 与 S4 session_end==N 的机制面）"
  - "13-04 Shutdown N 组快照逐组信号 + 有界 join（S4 双 pgid ESRCH + 进程退出的机制面）"
  - "13-05 metrics 21 series + session_active 模式分支 + session_start（S5 断言对象）"
  - "13-06 WESH_REMOTE_USER 注入链 whitelistEnv 第三参（S6 env 回读断言对象）"
  - "phase12.mjs 全文母本（文件头纪律三段式/check-skip 记录器/startWesh 夹具/assertOutputClean/收口模式逐字复用）"
provides:
  - "web/uat/phase13.mjs 六场景协议层 UAT（零依赖 Node 脚本，phase12.mjs 同构第三代）：13-01..13-06 全部机制产出的真实二进制 + 真实 wire + 真实信号验收"
  - "dialAttach 双形态 dial 夹具（Welcome=attached / Error+close=rejected 合流——churn 循环注入单元，后续 phase 可复用）"
  - "internal/server/load_test.go TestChurnPerClientSpawnThrottle churn 负载格（10rps×30s + 双采样差值断言 + throttled_total>0——PC-08 churn 语境操作化）"
  - "churnDial Go 侧双形态注入单元 + churn 格 LOADDATA 数据行（Phase 14 负载矩阵输入）"
affects:
  - "13-08（收口闸：phase13.mjs 两轮 + churn 格为其新证据面；PC-08/PC-09/SEC-09/OPS-12 四需求勾选由其 Task 2 承载——证据链最后一环已就位）"
tech-stack:
  added: []
  patterns:
    - "双形态 dial 合流（dialAttach：握手成功/拒绝两 resolve 形态——churn 循环单注入单元覆盖放行与拒绝两路径）"
    - "时序双断言 + 下界收紧（S2：~2s 存活 + ESRCH + elapsed≥4s 三面——1s 级错误默认在静默窗翻车、无兜底形态在 ESRCH 翻车）"
    - "缺席断言的程序序锚定（S6c：printenv 零输出以 echo 标记到达收口——标记到 ⇒ 缺席面结果必已到达，替代固定 sleep）"
    - "基线差值断言三件套（gor/mem/fd 双采样 + 回收窗口轮询 + 容差标定注释——Pitfall 7 假绿防线）"
key-files:
  created:
    - web/uat/phase13.mjs
  modified:
    - internal/server/load_test.go
decisions:
  - "S2b 时序双断言加 elapsed ≥4s 下界（plan 文本只要求『5s+护栏窗口内成立』）：~2s 存活 + ESRCH + 下界三面使 1s 级错误默认在静默窗翻车、无兜底形态在 ESRCH 翻车、远期默认（10s 级）在下界护栏内不翻车但 12s 护栏外翻车——判别力收紧非放宽（phase11 S8 同构）"
  - "S1c 融合 /metrics 黑盒 throttled 计数器断言（plan S1 文本只点名 stderr 事件行）：must_haves truth 3『防线生效过』的协议层承载——事件/计数器/拒绝数三方精确相等，一次 check 锁死 wire 聚合两通道一致性"
  - "S3d 取消形态以跨原到期点存活窗证明（c2 attach 后 3.2s > 残余宽限 ~1.4s + 1.8s 护栏）：仅断言『存活』对未取消计时器无判别力（stale 计时器在 1.4s 处退出）——跨期窗 + 再断开新宽限退出两段式锁定取消与重武装"
  - "S6c 缺席断言以 echo 标记收口（程序序锚定替代固定 sleep 缺席断言）：printenv 零输出无法轮询至完成——标记到达构成 printenv 结果行必已到达的同步边"
  - "churn 格复用 startPerClientServer + 新 churnDial 双形态注入单元：plan read_first 明示 perclient_test.go harness『桶参数保持生产默认』——mutate 传 nil 零覆写即生产镜像；300 次 attach 与 spawn_total 计数器精确相等（登记成功点程序序）实测 33==33"
  - "PC-08/PC-09/SEC-09/OPS-12 勾选留 13-08 收口 plan（11-01/12-01/13-01..13-06 先例延续）：13-08 Task 2 明确承载四条勾选与证据链映射——本 plan 落齐其证据链最后一环（六场景两轮 + churn 格）"
metrics:
  duration: 27min
  completed: 2026-09-05
  tasks: 2
  commits: 2
status: complete
actuals:
  tokens: 13600
  tasks: 2
  commits: 2
---

# Phase 13 Plan 07: phase13.mjs 六场景 UAT + churn 负载格 Summary

Phase 13 协议层证据主体：phase13.mjs 六场景（churn 节流 1011+XFF 换键 / KILL 兜底默认 5s / 退出三形态 255 / Shutdown N 组 / metrics 四计数器 / WESH_REMOTE_USER）两轮 29/29 全绿 + load_test.go TestChurn 负载格（10rps×30s，goroutine/fd 精确回落基线、mem 差值 +110KB ≪ 16MiB 界、throttled=267）——13-01..13-06 全部机制产出的进程级验收。

## What Was Built

**Task 1（test 16d16f6）**：`web/uat/phase13.mjs`（新件 769 行，phase12.mjs 同构第三代——文件头纪律三段式/check-skip 记录器/startWesh 夹具/assertOutputClean 自净/收口模式逐字复用；dialHello 扩 headers 注入形态 phase07 先例）：

- **S1 churn 节流 + XFF 换键**：新 `dialAttach` 双形态 dial（Welcome=attached / Error+close=rejected 合流）驱动 14 次高频 attach 循环——成功 4（burst 放行）/拒绝 10（全部 close==1011）+ 首拒绝端逐值（恰一 Error{server_error, "server is at capacity" 逐字} + 1011）+ 三方精确相等（stderr spawn_throttled==10 == /metrics wesh_pty_spawn_throttled_total==10 == 拒绝数）+ max_clients==0 事件名分治 + attach==成功数；`--auth-header` 开启半场——异 XFF 交替 6/6 全过（独立桶）+ 同值 XFF 连发 4 成功后 1011 逐值 + spawn_throttled 事件 remote==XFF 链首（D-05 换键直接证据）
- **S2 KILL 兜底默认 5s**：零配置（不传 --stop-timeout）trap '' HUP 免疫——断开后 ~2s 时点存活（HUP 免疫 + KILL 未到期）+ 12s 护栏内 ESRCH + elapsed ≥4s（实测断开至收割 ≈5.0s 与标称吻合）；显式 `--stop-timeout=0` 对照——warn 行（validateStartup D-02）+ 6s 窗（> 5s 标称 + 1s 护栏）泄漏存活（显式 0 被误改回默认则此处已被 KILL——判别面）+ finally kill -9 + 场景尾 pgrep -f 免疫夹具形态零命中（UAT 自身零泄漏）
- **S3 退出三形态 255**：--once / --exit-when-empty 裸 flag（grace=0）/ =2s 宽限到期三形态各独立实例全部退出码 255 逐值；宽限取消形态——重连 attach + echo 探针 + 跨原到期点 3.2s 存活窗（stale 计时器 1.4s 处退出即翻车）+ 再断开新宽限到期退出 255（重武装不退化）
- **S4 Shutdown N 组**：双客户端双独立 sh（pid 不等）→ SIGTERM → 双端 close 1001 + reason 含 server_shutting_down + 双 pgid 各 5s 护栏内 ESRCH + stderr session_end==2 且全 signal==SIGHUP + 进程退出 255
- **S5 metrics**：per-client——四计数器 series 全在 + wesh_pty_spawn_total==1≥1 + wesh_session_active==1（活跃会话计数语义）+ HELP 会话计数文案 + 样本行零 label 花括号（build_info version 豁免）；shared 对照——四计数器恒 0 且 series 保留不摘 + session_active==1 探活 + HELP 探活文案逐字（PC-09 prohibition 对照面）
- **S6 WESH_REMOTE_USER**：per-client 携头 attach → printenv 结果行回读 alice（注入链全通）+ NEL 线形头值（'ca\u00C2\u0085rl' → 0xC2 0x85 → Go U+0085 → sanitize 剥离）回读 carl 且控制字符零出现；shared 对照同头 → printenv 零输出（echo 标记收口锚定）——D-15 收窄零漂移

**Task 2（test ca9dc93）**：`internal/server/load_test.go` `TestChurnPerClientSpawnThrottle`（+179 行纯新增，`//go:build load` 首行逐字未动；imports 仅 +encoding/json）：

- 注入：`churnDial` 双形态单元（dial→Hello→读首帧判定——Welcome=attached 立即断开构成 connect→Hello→close 一循环 / Error→读至 CloseError=rejected）× 300 次 × 100ms 间隔 = 10rps×30s，per-client 服务端经 `startPerClientServer(t, ["sh"], nil)` 生产默认桶参数零覆写
- 断言三面（Pitfall 7 纪律，全部基线差值形态——region grep 自检零无基线绝对上限）：goroutine 终态 ≤ 基线+8（回收窗口 200ms 轮询至 session_active==0 且 gor 回落，10s 护栏；+8 容差标定注释：keep-alive scrape 连接池 + teardown straggler）/ mem 双采样差值 ≤ 16MiB（GC 后双 GC 形态——TestLoadMemoryBound 先例；标定来源注释承载）/ fd 差值 ≤ 基线+8（Linux-only，darwin 无 /proc 面跳过）/ wesh_pty_spawn_throttled_total > 0 + wesh_pty_spawn_total == attached 计数（登记成功点程序序精确对照）
- 观测：2s 间隔 scrapePeakSampler 全程轮询（gor/mem 峰值入 LOADDATA）

## Commits

| Task | Commit | Type | Summary |
|------|--------|------|---------|
| 1 | 16d16f6 | test | phase13.mjs 六场景协议层 UAT（769 行新件，两轮 29/29） |
| 2 | ca9dc93 | test | TestChurnPerClientSpawnThrottle churn 负载格（+179 行，两轮绿） |

## Verification Results

- `node web/uat/phase13.mjs` 两轮 29/29 全绿（23.1s/23.2s；SEC 自净 28 details 零 token/pid 命中两轮一致）——时序 flake 面复跑确认
- S2 实测断开至收割 ≈5.0s（两轮一致，标称 5s 零覆写实证）；S1 拒绝全 1011、三方计数 10==10==10 精确相等；S4 session_end==2 全 SIGHUP
- `time go test -tags=load -run TestChurn -count=1 ./internal/server/ -v` 两轮全绿（30.16s/30.16s）：LOADDATA attempts=300 attached=33 rejected=267 throttled=267 gor 8→8（峰 12）/ fd 11→11 / mem 869864→980040（差 +110KB ≪ 16MiB 界，峰 2.9MB）——资源精确回落基线
- `go build ./...` + `go vet ./internal/server/` + `go vet -tags=load ./internal/server/`（负载面）零输出；GOROOT gofmt（go1.26.3，10-05 收口闸工具）load_test.go/phase13.mjs clean
- `time go test -race ./... -count=1` 全仓五包全绿（1m36s：cmd/wesh 1.3s / proto 1.0s / pty 2.7s / server 95.7s / web 1.0s）——build tag 隔离默认面零影响
- ci.yml 零改动（无负载 leg——load 手动跑 D-11 既有纪律，plan「无则零改动」）；go.mod/go.sum 零 diff（T-13-SC 零新依赖）
- 断言面 region grep 自检：TestChurn 区域全部资源断言为 `endGor > baseGor+8` / `endMem-baseMem > 16MiB` / `endFDs > baseFDs+8` 基线差值形态，无基线对照的绝对上限零命中（PC-08 prohibition）
- UAT 收口残留：`pgrep -f "trap '' HUP"` 零匹配（exit 1）——脚本结束后零残留进程（must_haves truth 4）

## Deviations from Plan

None——plan executed exactly as written（无 Rule 1-4 偏差；两任务各单 test 提交收口）。

### Plan 措辞与实证的微小出入（不构成偏差）

- S2b 加 elapsed ≥4s 下界（plan 文本「5s+护栏窗口内成立」）：三面判别力（见 Decisions）——收紧非放宽，phase11 S8「~300ms 存活 + 5s 护栏」同构扩展
- S1c 融合 /metrics throttled 计数器断言（plan S1 文本只点名 stderr 事件行）：must_haves truth 3 的协议层承载（10rps 级注入结构性触发节流）——事件/计数器/拒绝数三方精确相等一次锁死
- S6c printenv 缺席断言以 echo 标记收口（plan 文本「env 无该键」的判定面细化）：程序序锚定替代固定 sleep 缺席断言——标记到达构成 printenv 结果行必已到达的同步边
- churn 格 mem 断言在回收轮询后加双 GC（TestLoadMemoryBound 形态）：in-process 装配使测试侧 GC 即服务端 GC——差值断言不受未回收瞬态垃圾干扰（plan「churn 停止后等回收窗口再终点采样」的 GC 强化实现）
- phase13.mjs 的 dialHello 采 phase07 headers 注入形态（phase12 母本无 headers 参数）：S1 XFF/S6 X-Remote-User 注入需要——`{ headers, protocols }` 第二参形态 phase07 本机探针先例

## Requirements Trace

PC-08/PC-09/SEC-09/OPS-12 勾选**不随本 plan 执行**（11-01/12-01/13-01..13-06 先例延续）——13-08 收口 plan Task 2 明确承载四条勾选与证据链映射（「Go 新测组 + phase13.mjs 六场景两轮 + churn 负载格 + diff 审查」）。本 plan 落齐证据链最后一环：13-06 SUMMARY 登记「SEC-09 勾选留 phase 末（13-07 S6 进程级 env 回读未齐）」、13-03/13-04「PC-09/OPS-12 留 phase 末（13-07 S3/S4 进程级断言未齐）」、13-02「PC-08 留 phase 末（13-07 压测面证据未齐）」——S1-S6 与 churn 格全部就位。

## Known Stubs

None——全链无桩：六场景与 churn 格全部为真实二进制 + 真实 wire + 真实信号断言；无 skip 项（本 phase 无平台豁免面——协议层与负载格均在 Linux 开发机执行）。

## Threat Mitigations Applied

| Threat | Disposition | Evidence |
|--------|-------------|----------|
| T-13-22 (DoS: churn 负载下资源无界——RSS/goroutine/fd 泄漏未检出) | mitigate | TestChurn 双采样差值上界 + 回收窗口轮询 + throttled>0（gor 8→8 / fd 11→11 / mem +110KB 实测回落基线）+ phase13.mjs S1 协议层节流全链（两轮绿） |
| T-13-23 (DoS: UAT 自身泄漏——trap 免疫对照进程残留) | mitigate | S2 对照形态 finally kill -9 + 场景尾 pgrep -f 免疫夹具形态零命中断言（S2d）+ 收口后 pgrep 全局复核 exit 1 |
| T-13-24 (Information Disclosure: UAT 控制台输出泄漏 token/pid) | mitigate | emittedDetails 收集 + assertOutputClean 运行时自证（两轮 28 details 零命中；detail 只打印状态码/布尔/计数/退出码/文案常量）|
| T-13-SC (Tampering: 依赖面) | mitigate | 零新依赖——phase13.mjs 为 Node 原生 WebSocket/fetch/child_process 零依赖脚本；go.mod/go.sum 零 diff；go test -race 默认面五包全绿零影响 |

## Self-Check: PASSED

- 文件存在：web/uat/phase13.mjs（769 行 ≥ min_lines 400，六场景与红线/时序纪律注释段在场）/ internal/server/load_test.go（TestChurnPerClientSpawnThrottle + churnDial；//go:build load 首行逐字未动）——全部 FOUND
- 提交存在：16d16f6（test）、ca9dc93（test）——git log 确认 FOUND
- diff 白名单：恰两文件（plan files_modified 枚举一致）纯新增 948 行零删除；全量 -race 五包绿（默认面零影响）；UAT 零残留进程
