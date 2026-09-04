---
phase: 11-per-client
verified: 2026-09-04T02:40:00Z
reverified: 2026-09-04T06:50:00Z
status: passed
score: 12/12 must-haves verified
behavior_unverified: 0
overrides_applied: 0
deferred:
  - truth: "per-client 输出闭包 trySend 失败直踢 1013，绕过 kickOrCreditLocked 的 attach 宽限与信用暂存两层判定（REVIEW WR-01）"
    addressed_in: "Phase 12"
    evidence: "STATE.md:98 登记「WR-01 → Phase 12」；ROADMAP Phase 12 SC4/SC5（停读续读背压 / 1013 踢出语义主场）与 REQUIREMENTS PC-10/PC-11 → Phase 12；不构成 Phase 11 四条 SC 任一缺口（慢客户端保护语义非本 phase 目标面）"
  - truth: "teardownPCLocked 的 reaped 栅栏存在 Wait-return→hubMu-acquire 微窗口，「kill-after-reap 结构性不可能」注释声明过强（REVIEW WR-02）"
    addressed_in: "Phase 13"
    evidence: "STATE.md:99 登记「WR-02 → Phase 13」（附零成本严格修法：waitDone 非阻塞 select 联合判定）；ROADMAP Phase 13 SC3/SC4（KILL 兜底收割 / 关停 N 进程组终结语义主场）；REVIEW 自评实际可利用性与 orphan 路径已接受窄窗口同档（pid 整轮回绕 + µs 窗，实际不可达）"
behavior_unverified_items: []
human_verification: []
---

# Phase 11: per-client 生命周期主干 Verification Report

**Phase Goal:** per-client 模式下每个浏览器客户端 attach 即获得独立 PTY 子进程，其生死只影响自己——核心 E2E 最长链成立
**Verified:** 2026-09-04T02:40:00Z（初验）
**Re-verified:** 2026-09-04T06:50:00Z
**Status:** passed（re-verification 闭合 darwin 运行面）

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1a：两个客户端 attach 后各自获得独立 shell（两个不同 pid、输出互不串台） | ✓ VERIFIED | 行为实证（本次自跑）：`node web/uat/phase11.mjs` S1b pid不等=true / S1c B端1.5s静默窗零帧零命中；`go test -race` TestPerClientTwoClientsTwoPids PASS (2.01s)；代码面 perclient.go:151 `s.spawnFunc(h.Cols, h.Rows)` 每 attach 独立 spawn |
| 2 | SC1b：首帧 winsize = Hello 上报尺寸经钳制（无 80x24 中间态闪烁） | ✓ VERIFIED | 行为实证（自跑）：UAT S2a 首帧 Welcome cols==111/rows==44、S2b stty size 回读 "44 111" 逐字一致；TestPerClientWelcomeDims PASS；代码面 perclient.go:147-151 注释锚定 ClampDim 直通 StartWithSize + :217 Welcome 恒首帧入队先于注册 |
| 3 | SC2：spawn 失败该客户端收类型化 Error 帧（通用文案，绝不拼 err.Error()）+ 1011 关闭；服务端与他端不受影响 | ✓ VERIFIED | 行为实证（自跑）：UAT S3b 恰一帧 Error{server_error,"failed to start process" 逐字}+close 1011、S3c A echo 照常+healthz 200；TestPerClientSpawnFailure PASS；代码面 perclient.go:160 定值常量、`grep err.Error()` 零命中；logEvent spawn_failed 四段 schema |
| 4 | SC3a：客户端断开（正常关闭）后子进程进程组立即收 SIGHUP（--stop-signal 可配），无宽限、无僵尸残留 | ✓ VERIFIED | 行为实证（自跑）：UAT S4a 断开后 2s 护栏内 pgid ESRCH；TestPerClientDisconnectSIGHUP PASS（pcSessions 收敛 0）；代码面 teardownPCLocked perclient.go:337 `SignalGroup(s.stopSignal)`（默认 HUP，OPS-04 通道继承）+ detach/kick 双挂点 clients.go:620/:847 覆盖一切断开形态。1006 真实异常形态 = S4b skipped 平台豁免（CODEBUDDY.md §5 风险接受，勿判缺口） |
| 5 | SC3b：信号与收割锁内序列化（reaped 栅栏）、KILL 兜底、teardown 恰好一次 | ✓ VERIFIED | 行为实证（自跑）：TestPerClientStopTimeoutKillFallback PASS（trap 免疫 + StopTimeout=1s，到期 ESRCH）；TestPerClientTeardownRaceOnce PASS（10 轮 exit 0×Close 竞态注入，quiescent 四件套 + exitf 零调用 :1179 + 零 panic，14143fe 覆写形态 1.30s）；UAT S8a/S8b 时序双断言 PASS；代码面 reaped 栅栏 perclient.go:336-346 覆盖 SIGHUP 与补 KILL 两发信号点。残留理论窗（WR-02）已登记归 Phase 13（见 Deferred） |
| 6 | SC4：子进程退出（exit 42 / 信号死亡）后仅该客户端收私有 EXIT 帧（exit_code，信号 -1）+ 1000 关闭；服务端与他端继续运行 | ✓ VERIFIED | 行为实证（自跑）：UAT S5a exit_code=42+close 1000、S5b B端1.5s零帧、S5c 窗后 B echo 照常、S5d exit_code=-1+大写 SIGHUP+1000；TestPerClientExitPrivate42/ExitSignalMinus1 PASS；代码面 sessionWatcher perclient.go:291-314 直写序（组帧一次→Write 2s ctx→Close 1000，禁 outbox）+ exitMessage -1 语义 server.go:1372-1396 |
| 7 | 启动期零子进程（per-client sess=nil）+ sess×mode 装配契约 panic | ✓ VERIFIED | 行为实证（自跑）：UAT S1a attach 前 pgrep -P 空输出；TestNewModeSessContract 两子测 PASS；代码面 main.go:1350-1354 per-client 分支 sess 保持 nil、server.go:412-417 两形态 panic |
| 8 | Welcome 恒 S→C 首帧且回显本端 Hello 钳制尺寸；五 goroutine 注册后启动 | ✓ VERIFIED | 代码面 perclient.go:217-220 Welcome 入队先于 registerLocked+pcSessions 登记且全程持 hubMu；:243-271 startSessionGoroutines 五件装配；行为面 S2a/TestPerClientWelcomeDims PASS 同证 |
| 9 | INPUT case 读循环零分支（cl.inQ 间接字段），CR-01 读循环永不直写 master | ✓ VERIFIED | 代码面 server.go:1178 `cl.inQ.tryEnqueue(data[1:])`、clients.go:781 inputWriter 包级参数化（shared 装配 :531 `inputWriter(s.sess, s.inputQ, s.inputDone)`、per-client 装配 perclient.go:269）；TestPerClientInputEcho PASS（自跑） |
| 10 | D-02 容量再闸（1011+容量文案 wire 形态）+ D-03 注册点复检回收（并发子进程数 ≤ maxClients 硬不变量） | ✓ VERIFIED | 行为实证（自跑）：UAT S6b Error{server_error,"server is at capacity" 逐字}+close 1011；TestPerClientCapacityGate（linger 形态）与 TestPerClientCapacityRecheckRace（barrier 竞态：恰一胜一负+终态==1+败者 ESRCH）PASS；代码面 perclient.go:139-145 pre-spawn 闸、:184-189 复检、:381-397 reapOrphanSession 完整 SignalGroup→Drain→Close→Wait |
| 11 | shared 模式逐字节零回归（零回归收口闸） | ✓ VERIFIED | 行为实证（自跑）：`go test -race -count=1 ./...` 5 包全 ok（1m5s）；`node web/uat/phase02.mjs` 12/12 PASS 冒烟；diff 审查（自跑）：基点 954da7c 以来删除文件 0、新增恰 3 文件、修改恰 6 文件、红线路径（proto/web/src/metrics/health/resize/go.mod/go.sum）零出现、两白名单测试文件删除行==0、shared 行为文件删除行逐条为计划内变换（注释更新/startOpts 收编/inputWriter 参数化/inQ 切换） |
| 12 | darwin 共享 kqueue watcher dup-watch fail-closed（errDupWatch + awaitExit 退化 cmd.Wait()，Pitfall 9） | ✓ VERIFIED（re-verification） | 初验在案（代码/接线/编译闸）+ 运行面闭合：CI run 33832096581 macOS leg TestWatchDupPidFailClosed PASS (0.11s)、TestKqueueExitZombieRace PASS (1.10s, 非 SKIP)；CI run 33844831146 macOS leg 双测复证 PASS——darwin 运行面经两轮 CI 实证（UAT 测试 1 证据） |

**Score:** 12/12 truths verified（初验 11/12 + re-verification 补 1：darwin 运行面经 CI 两轮实证闭合）

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | REVIEW WR-01：per-client 输出闭包 trySend 失败直踢 1013，丢失 attach 宽限与「全满置信用」两层判定（慢链路新端瞬态满箱误踢面） | Phase 12 | STATE.md:98 登记；ROADMAP Phase 12 SC4/SC5（停读续读/1013 踢出语义主场）；REQUIREMENTS PC-10/PC-11 → Phase 12。核验结论：不触及本 phase 四条 SC 任一（慢客户端保护语义非 Phase 11 目标面），不构成本 phase 目标缺口 |
| 2 | REVIEW WR-02：reaped 栅栏 Wait-return→hubMu-acquire 微窗口（kill-after-reap 理论面，「结构性不可能」注释声明过强） | Phase 13 | STATE.md:99 登记（附零成本修法）；ROADMAP Phase 13 SC3/SC4（终结语义主场）；REVIEW 自评与 orphan 路径已接受窄窗口同档（pid 回绕+µs 窗，实际不可达）。核验结论：序列化机制（hubMu 内 reaped）本体存在且经 10 轮竞态注入实测，残留为理论窗口加固项，不构成本 phase 目标缺口 |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/server/perclient.go` | pcSession + upgradePerClient + 五件装配 + sessionWatcher + teardownPCLocked + reapOrphanSession | ✓ VERIFIED | 397 行实质实现；`type pcSession struct` :62、capacityMessage/rejectCapacity :100/:110、全部六函数在案且被调用 |
| `internal/server/server.go` | New 契约 panic + 尾部分岔 + Attach 分岔 + INPUT cl.inQ + Shutdown 守卫 | ✓ VERIFIED | :412-417 panic、:514-517 per-client 仅初始化 pcSessions、:1026-1030 upgradePerClient 调用点、:1178 cl.inQ、:1570 Shutdown 守卫 |
| `internal/server/clients.go` | client +inQ/pc 字段 + inputWriter 参数化 + detach/kick 双挂点 + 早退守卫 | ✓ VERIFIED | :158/:162 字段、:781 inputWriter、:620/:847 teardownPCLocked 双挂点、:913-915 maybeExitWhenEmptyLocked 早退 |
| `cmd/wesh/main.go` | run() per-client sess=nil + startOpts 收编 + 回滚 nil 守护 | ✓ VERIFIED | :1327-1333 startOpts+spawnFunc 闭包（StartWithSize 直通）、:1350-1354 sess=nil、:1377 回滚 nil 守护、:1396 Options 尾部两键 |
| `internal/server/perclient_test.go` | harness + 十四测 | ✓ VERIFIED | 14 测名全部在案（:212-:1079），本次 -race 自跑 14/14 PASS |
| `internal/pty/reap_darwin.go` | errDupWatch + watch() dup fail-closed | ⚠️ ORPHANED-RUNTIME（编译面 VERIFIED） | 代码与接线在案；运行面测试归 CI macOS leg，本机不可执行（build tag） |
| `internal/pty/reap_darwin_test.go` | TestWatchDupPidFailClosed（append-only） | ✓ VERIFIED（存在性/append-only） | :137 在案；基点以来删除行==0（自跑 diff 实证） |
| `internal/server/export_test.go` | PCSessionsLenForTest 出口 | ✓ VERIFIED | :48 在案，被 5 处测试调用；append-only 删除行==0 |
| `web/uat/phase11.mjs` | 协议层八场景 UAT | ✓ VERIFIED | 595 行；S1-S8 场景函数齐全；本次自跑 21/21 PASS + S4b skipped |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| server.go Attach 升档分岔 | perclient.go upgradePerClient → SpawnFunc 闭包 | server.go:1026-1027 调用；spawn 在 ticket 核销（:1016 close(helloDone)）之后、hubMu 之外（perclient.go:151） | ✓ WIRED | S1a 启动期零子进程 pgrep 实证 spawn 点后置（SEC-08 行为面） |
| clients.go detach / kickSlowConsumerLocked | perclient.go teardownPCLocked | clients.go:619-621（kick）/:846-848（detach），removeLocked 后、maybeExitWhenEmptyLocked 同位 | ✓ WIRED | 双挂点覆盖全断开形态；teardownOnce 恰好一次经竞态注入实测 |
| perclient.go sessionWatcher | proto.ExitFrame + server.go exitMessage | perclient.go:306 `proto.ExitFrame(code, exitMessage(err, code))` → 同步 Write 2s ctx → Close(1000) | ✓ WIRED | UAT S5a/S5d 帧序 EXIT 先于 1000、exit_code 42/-1 两形态实证 |
| server.go INPUT case | clients.go client.inQ | server.go:1178 `cl.inQ.tryEnqueue`；shared 升档赋 s.inputQ（:1051）、per-client 赋 pc.inQ（perclient.go:203） | ✓ WIRED | TestPerClientInputEcho 全链回显 PASS |
| teardownPCLocked 慢半段 | pcSessions 移除 | perclient.go:349-357 goroutine 内 Drain(200ms)→Close→<-waitDone→hubMu delete→close(teardownDone) | ✓ WIRED | 阻塞面不占 hubMu；pcSessions 收敛 0 经测试断言（:950 等） |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| upgradePerClient Welcome | cols/rows | h.Cols/h.Rows（DecodeHello ClampDim 钳制后真实客户端上报） | ✓ | FLOWING（S2a/S2b stty 回读实证） |
| ReadLoop 闭包 OUTPUT 帧 | chunk | pc.sess.ReadLoop 真实 PTY 输出（mc.ptyOutputBytes 同计数器） | ✓ | FLOWING（S1c/S5b 静默窗断言依赖真实帧流） |
| sessionWatcher EXIT 帧 | code | pc.sess.Wait() 真实退出码（exec.ExitError 提取） | ✓ | FLOWING（S5a exit 42 / S5d 信号 -1 实证） |
| 容量闸计数 | len(s.pcSessions) | hubMu 内真实注册表计数 | ✓ | FLOWING（S6 linger 注入确定性触发实证） |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| phase11 协议八场景 | `node web/uat/phase11.mjs /tmp/wesh-verify/wesh`（新构建二进制） | 21/21 PASS + 1 skipped（S4b 豁免），exit 0 | ✓ PASS |
| 十四测（行为依赖真相的行为证据） | `go test -race -count=1 ./internal/server/ -run 'TestPerClient\|TestNewModeSessContract' -v` | 14/14 PASS（含两子测），7.457s | ✓ PASS |
| 全量 -race（零回归第一证据） | `go test -race -count=1 ./...` | 5/5 包 ok，1m5s | ✓ PASS |
| shared 协议层冒烟 | `node web/uat/phase02.mjs /tmp/wesh-verify/wesh` | 12/12 PASS | ✓ PASS |
| darwin 编译闸 | `GOOS=darwin go build ./... && GOOS=darwin go vet ./...` | 双 exit 0 | ✓ PASS |
| 跑后进程泄漏 | `ps aux` 排查 wesh/trap 滞留 | 零滞留（pgrep 命中经 ps 实证为自匹配） | ✓ PASS |
| SUMMARY 提交存在性 | `git cat-file -t` ×11 | 11/11 存在（8850986…14143fe） | ✓ PASS |

### Probe Execution

SKIPPED（本 phase 无 probe 类验证约定——验证载体为 Go 测试与 UAT 脚本，已在 Behavioral Spot-Checks 全部自跑）

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PC-02 | 11-01/11-03/11-05/11-06 | attach 认证后独立 spawn（Hello 钳制尺寸作初始 winsize）；spawn 失败类型化 Error+1011，服务端与他端不受影响 | ✓ SATISFIED | Truths #1/#2/#3/#10：UAT S1/S2/S3/S6 + Go 四测 + 代码面全链 |
| PC-03 | 11-01/11-02/11-04/11-05/11-06 | 断开（含异常）进程组立即 SIGHUP（--stop-signal 可配）无宽限；信号与收割序列化杜绝 kill-after-reap | ✓ SATISFIED | Truths #4/#5：UAT S4a/S7/S8 + Go 四测（含 10 轮竞态注入）；darwin 运行面残留 → human 项；1006 形态平台豁免登记非缺口 |
| PC-04 | 11-01/11-04/11-05/11-06 | 子进程退出仅该客户端收私有 EXIT 帧（exit_code，信号 -1）+1000；服务端与他端继续运行 | ✓ SATISFIED | Truth #6：UAT S5 四断言 + Go 两测 + sessionWatcher 直写序 |

**孤儿需求检查：** REQUIREMENTS.md 追溯表映射 Phase 11 = PC-02/PC-03/PC-04 恰三条，全部出现于各 plan `requirements` 字段（11-01/11-05/11-06 三条全载，11-02 PC-03，11-03 PC-02，11-04 PC-03/PC-04）——无孤儿。三条需求勾选状态 Complete（11-06 承载）。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | 无 | — | 九文件 TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER 全零命中；perclient.go 零 err.Error() 拼接；无空实现/硬编码空数据渲染面；失败与容量文案双定值常量 |

### Human Verification Required（已闭合）

#### 1. CI macOS leg darwin 测试实际结果确认

**Result:** CLOSED（UAT 测试 1，2026-09-04）——CI run 33832096581 macOS leg（go test -race -count=1 -v）：TestWatchDupPidFailClosed PASS (0.11s)、TestKqueueExitZombieRace PASS (1.09s, 非 SKIP)。Q1 裁决成立——kqueue 补发僵尸进程事件，reap_darwin.go:12-15 兜底预案条件不触发，awaitExit 无需退化。REVIEW IN-01「CI 绿不证明裁决」关切经 -v 日志逐测试行实证闭环；CI run 33844831146 双测复证 PASS。

### Re-verification Record (2026-09-04T06:50:00Z)

初验（2026-09-04T02:40:00Z）以 human_needed 收口，唯一未闭合项 = darwin 运行面。Re-verification 证据链：

1. **darwin 运行面闭合**：UAT 测试 1（CI run 33832096581 -v 日志逐行核对）——TestWatchDupPidFailClosed / TestKqueueExitZombieRace 实际运行且 PASS，非 SKIP。Truth #12 由 PRESENT_BEHAVIOR_UNVERIFIED → VERIFIED。
2. **G-11-2 闭环**（UAT 测试 2 初报 issue 的处置）：gap closure plan 11-07（afb77a8 test + 5aad25a fix：waitPgroupESRCHWithProbe 探针参数化，EPERM 归类存活形态落入护栏轮询，护栏保留与他错立即 Fatal 两半边由 TestWaitPgroupESRCHProbeSemantics 四子测锁定）+ CI run 33844831146 macOS leg 全绿（TestPerClientTeardownRaceOnce PASS 1.40s = FAIL 现场测试转绿；同 helper 三调用点保持 PASS；internal/server ok 66.4s）——「Phase 11 测试套件在 CI macOS leg 全绿」truth 达成。
3. **CI 复验副产**：ubuntu flaky（PS1 交错，9936f2b 修正）——纯测试时序鲁棒性，不触及产品行为面。
4. **Linux 零回归三证据复核**（2026-09-04 ship 期）：全量 -race 五包 ok（两轮 1m6.6s / 1m7.2s）+ darwin build/vet 双闸 + gofmt + 单一文件 diff 门（975af23..HEAD -- internal/ cmd/ web/ == internal/server/perclient_test.go）。
5. **SECURITY.md 就位**（secure-phase 2026-09-04）：21 威胁全 closed，threats_open: 0。

**Score 12/12；Gaps 零；UAT 2/2 passed。Phase 11 verification passed。**

### Gaps Summary

无代码面缺口。四条 ROADMAP Success Criteria 全部经本次自跑的行为测试实证（非转述 SUMMARY）：SC1（S1/S2 + 双 Go 测）、SC2（S3/S6 + 双 Go 测）、SC3（S4a/S7/S8 + 四 Go 测含竞态注入与 KILL 兜底时序）、SC4（S5 四形态 + 双 Go 测）；PC-02/PC-03/PC-04 三需求全数 SATISFIED 无孤儿；零回归面经全量 -race 5 包 + phase02 协议冒烟 + diff 四件套审查三重自跑实证；REVIEW 两条 WARNING 经核验均不构成本 phase 目标缺口（分属 Phase 12 慢客户端语义主场与 Phase 13 终结语义主场，STATE.md 已登记，列入 deferred 跟踪）。

唯一未闭合项为 darwin 运行面：11-02 的 dup-watch fail-closed 代码与接线在案、编译闸自跑通过、测试已挂 CI macOS leg，但 darwin-only 测试本机不可执行且 CI 结果外部不可观测（REVIEW IN-01 同指）——以 human_needed 收口，待一项轻量人工确认。

---

_Verified: 2026-09-04T02:40:00Z_
_Verifier: Claude (gsd-verifier)_
