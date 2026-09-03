---
phase: 07-deployment
plan: 04
subsystem: pty-spawn-lifecycle
tags: [cwd, term, stop-signal, stop-timeout, process-group, sigkill-escalation, uid-gid, privilege-drop, credential, nosetgroups, identity-env, sec-06, tdd]

requires:
  - phase: 07-deployment/07-03
    provides: logEvent variadic remote_user 第四参（exit_when_empty 家族事件行同口径承载）+ 07-03 全绿基线
  - phase: 06-session-lifecycle/06-02
    provides: maybeExitWhenEmptyLocked 两触发点与 exiting 门（stop-signal 序列换入点）+ accept-255 断言常量
  - phase: 01-foundation/01-03
    provides: 信号类断言落盘标记文件先例（stdout 标记 WS 断开后不可观测——stopseq 夹具同步形态来源）
  - phase: 05-multi-client/05-08
    provides: 验收 grep==0 红线断言纪律（旧标识符注释字面同清——SignalHangup 零残留的语义边界）
provides:
  - "pty.StartOptions{Dir,Term,Uid,Gid} + Start(argv, opts) + whitelistEnv(term, uid)（07-04 选项化；零值等价 TestStartZeroValueParity 锁定：Dir 空 = 继承、Term 空 = xterm-256color、Uid -1 = 不降权）"
  - "--cwd/--term CLI 公开契约（D-21 one-way）+ validateStartup --cwd os.Stat 预检 fail-fast exit 2（spawn 前零资源占用；纯函数纪律注释更新：os.Stat 只读探测允许）"
  - "SignalGroup(sig) 进程组信号泛化 + StopSignalByName 名→信号映射（linux/darwin 同签名同表；--stop-signal HUP|TERM|INT|KILL parse 期枚举校验唯一事实源）"
  - "--stop-signal/--stop-timeout CLI 公开契约（D-22 one-way）+ server.Options StopSignal（零值兜底 SIGHUP 现状语义）/StopTimeout（零值 = 不补 KILL 合法）+ clients.go stopChildLocked 统一出口（两触发点换入 + AfterFunc 异步补 KILL 不占 hubMu、ESRCH 幂等）——07-05 Shutdown 复用同一字段同一出口（Options 单一通道，双写即漂移）"
  - "--uid/--gid CLI 公开契约（D-24 one-way，数字直通成对强制）+ SysProcAttr.Credential 降权（fork 后 exec 前）+ supplementary groups 策略（root 清空 = 最小权限既定语义；非 root NoSetGroups 跳过——非 root 无 CAP_SETGID 清空必 EPERM）+ validateStartup 成对校验 exit 2 双 flag 名"
  - "whitelistEnv 身份环境改写（D-25）：uid>=0 时 LookupId 按 passwd 条目改写 HOME/USER/LOGNAME（不再继承宿主），查不到剔除三键（shell 自默认）；SEC-06 替换式注入纪律零漂移"
affects: [phase-07 后续 plans（07-05 Shutdown/1001 复用 stopSignal/stopTimeout 字段与 stopChildLocked 出口 / 07-06 配置文件 cwd/term/stop-signal/stop-timeout/uid/gid 键——RESEARCH Pattern 4 fileConfig 已预留 / 07-07 UAT stop-signal 宽限与降权场景 / 07-08 人工 UAT 复核：supplementary groups 策略裁决与 flagged_assumptions 两项）, verify-work]

actuals:
  tokens: 19780
  tasks: 3
  commits: 7

tech-stack:
  added: []
  patterns:
    - "StartOptions 选项化承载 spawn 可配面：四字段一次定义分 task 陆续消费（Task 1 Dir/Term、Task 3 Uid/Gid），零值等价语义由专测锁定——未配置全部四 flag 时与选项化前逐字节一致"
    - "stop-signal 序列统一出口（stopChildLocked）：SignalGroup(stopSignal) + stopTimeout>0 时 AfterFunc 异步补 SIGKILL——不与 lifecycle 协调、不占 hubMu、ESRCH 幂等使子进程早死无害（Pitfall 8 纪律）；exit-when-empty 立即/宽限到期两触发点与 07-05 Shutdown 共用"
    - "supplementary groups 环境感知策略：euid==0 → NoSetGroups=false（清空附加组，root 的组永非目标身份的组，保留即提权泄露）；euid!=0 → NoSetGroups=true（非 root 无 CAP_SETGID，清空必 EPERM 实测命中；非 root 唯一可达降权是降回自身，保留自身附加组零提权面）"
    - "信号类行为测试夹具纪律：trap 安装与 detach 信号竞态经落盘标记文件同步（01-03 先例）；`trap \"\" TERM` 恒活机理 = SIG_IGN 跨 exec 持久（POSIX）整组免疫，捕获型 trap 则 exec 复位默认——两形态互补锁定忽略/送达语义"

key-files:
  created:
    - internal/pty/signal_test.go
    - internal/server/stopseq_test.go
  modified:
    - internal/pty/spawn.go
    - internal/pty/spawn_test.go
    - internal/pty/signal_linux.go
    - internal/pty/signal_darwin.go
    - internal/pty/io_test.go
    - internal/pty/reap_darwin_test.go
    - internal/pty/reap_test.go
    - internal/server/server.go
    - internal/server/clients.go
    - internal/server/e2e_test.go
    - internal/server/limits_test.go
    - internal/server/resize_arb_test.go
    - internal/server/sharetoken_test.go
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go

key-decisions:
  - "supplementary groups 环境感知策略（root 清空 / 非 root NoSetGroups 跳过）——plan 既定「NoSetGroups false 清空附加组」与 plan 自身要求的「降权到 self 免 root」测试结构性矛盾（GOROOT exec_linux.go:496-499 无条件 setgroups，非 root 无 CAP_SETGID 必 EPERM，本机实测 fork/exec: operation not permitted 命中）；root 部署（OPS-05 真实场景：root 启动降权 nobody）语义逐字保持，非 root 降回自身保留自身附加组零提权面；07-08 人工 UAT 复核项联动"
  - "stopChildLocked 统一出口承载 D-22 序列：exit-when-empty 两触发点与 07-05 Shutdown 共用同一函数同一对字段（must_have key_link『Options 单一通道，双写即漂移』的函数级兑现）"
  - "parse 期拒绝行归属纪律沿用：--stop-signal 非法枚举、--stop-timeout 负值、--uid/--gid 值域四行落 TestTLSKeyPairError 错误表（plan 字面 TestParseArgs 对错误行结构性不可达——表内 t.Fatalf on err；03-04『parse 期拒绝既定归属』先例第三次沿用）"
  - "SignalHangup 删除的 grep==0 语义边界：方法删除 + 调用点换入 + 测试机械换名（TestSignalGroupHangup）+ 注释字面同清（05-08 纪律——验收 grep 是源码级机械检查，注释提及旧名同样计数）"
  - "stopseq 夹具机理修正：KILL 测初版失败唯一根因 = trap 安装竞态（落盘标记同步修复）；`trap \"\" TERM; sleep 100` 单 sleep 夹具经真实二进制冒烟证明其实恒活（SIG_IGN 跨 exec 持久），plan success_criteria 字面夹具成立——初版探针（setsid fork 语义 + fish 作业控制干扰）误判已更正并留档"

patterns-established:
  - "pty 包测试零值等价适配形态：全部既有调用点统一传 StartOptions{Uid: -1, Gid: -1}（注释标注零值语义），darwin 标签测试件同步适配（GOOS=darwin go vet 交叉验证编译期形态）"
  - "euid 条件化系统调用策略形态：安全语义在有能力判何处严格（root 清组），在无能力判何处跳过（非 root 保留自身组）——以零提权面论证替代一刀切默认"
  - "进程组信号行为测试三件套：落盘标记同步 trap 安装 + 捕获型 trap 特异退出码作证送达（exit 43）+ 忽略型 trap 时序双断言（timeout 前静默 + 其后 -1）"

requirements-completed: [OPS-04, OPS-05]

coverage:
  - id: D1
    description: "--cwd/--term 全链：opts.Dir 落 cmd.Dir（白盒 + sh -c pwd e2e）、opts.Term 参数化 TERM= 行（白名单单元 + $TERM e2e）、零值等价（Dir 空继承/Term 空 xterm-256color/Uid -1 无 Credential）；--cwd 不存在 validateStartup exit 2（TestStartupMatrix 两行）"
    requirement: OPS-04
    verification:
      - kind: unit
        ref: "internal/pty/spawn_test.go#TestStartOptionsDir/#TestStartOptionsTerm/#TestStartZeroValueParity + cmd/wesh/main_test.go#TestStartupMatrix（cwd 两行）/#TestParseArgs（cwd/term 行）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（--cwd /tmp --term vt100 -- sh -c 'pwd;echo $TERM' → /tmp 与 vt100 观测，产物已清理）"
        status: pass
    human_judgment: false
  - id: D2
    description: "stop-signal 序列：SignalGroup(sig) 负 pid 进程组（WaitStatus 精确断言 TERM/HUP 两形态）；exit-when-empty 两触发点换入（TERM 送达 trap exit 43 作证）；stop-timeout 后 AfterFunc 补 SIGKILL（忽略型 trap 时序双断言）；默认 HUP+0 零漂移（emptyexit 六测试）；SignalHangup grep==0"
    requirement: OPS-04
    verification:
      - kind: unit
        ref: "internal/pty/signal_test.go#TestStopSignalByName/#TestSignalGroup + internal/pty/io_test.go#TestSignalGroupHangup + internal/server/stopseq_test.go#TestExitWhenEmptyStopSignalTERM/#TestExitWhenEmptyStopTimeoutKills + emptyexit_test.go 六测试"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（--stop-signal TERM --stop-timeout 1s --exit-when-empty 两夹具：WS attach→close 后 close+1002ms 退出 255——TERM 忽略、1s KILL、accept-255 全链）"
        status: pass
    human_judgment: false
  - id: D3
    description: "uid/gid 降权与身份环境：Credential 白盒（Uid/Gid/NoSetGroups 策略断言）+ id -u e2e（降权 self 免 root）；LookupId 改写 HOME/USER/LOGNAME（宿主干扰值不得继承、每键恰好一行）；查不到剔除三键（其余键不受影响、uid<0 继承现状不变）；成对强制与值域 fail-fast"
    requirement: OPS-05
    verification:
      - kind: unit
        ref: "internal/pty/spawn_test.go#TestDropPrivilegesSelf/#TestDropPrivilegesIdentityEnv/#TestWhitelistEnvDropUnknownUid + cmd/wesh/main_test.go#TestStartupMatrix（成对三行）/#TestTLSKeyPairError（值域四行）/#TestParseArgs（uid gid pair 行）"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（--uid self --gid self → id -u/id -g/HOME/USER 全对；--uid 65534 单给 → exit 2『--uid and --gid must be given together』）+ go test -race -count=1 ./internal/pty"
        status: pass
    human_judgment: false

duration: 1h 20m
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 04: 子进程管理与降权（--cwd/--term/--stop-signal/--stop-timeout/--uid/--gid）Summary

**OPS-04/OPS-05 六 flag 全链落地：pty.Start 选项化（StartOptions 四字段零值等价）承载 --cwd/--term（含 stat 预检 exit 2）与 --uid/--gid（Credential 降权 + whitelistEnv 身份环境改写/剔除）；SignalHangup 泛化为 SignalGroup + StopSignalByName 唯一映射，exit-when-empty 两触发点换入可配 stop-signal 序列（stopChildLocked 统一出口，stop-timeout 后 AfterFunc 异步补 SIGKILL）——默认 HUP+0 与未配置四 flag 两零漂移经既有套件与专测双重锁定。**

## Performance

- **Duration:** 1h 20m
- **Started:** 2026-08-26T02:32:47Z
- **Completed:** 2026-08-26T03:52:32Z
- **Tasks:** 3/3
- **Files modified:** 17（新建 2：signal_test.go + stopseq_test.go；修改 15；合计 836 insertions / 78 deletions，plan 全 diff 79122 chars）

## Accomplishments

- `wesh --cwd /tmp --term vt100 -- sh -c 'pwd; echo $TERM'` 真实二进制冒烟输出 `/tmp` 与 `vt100`——opts.Dir 落 cmd.Dir、opts.Term 参数化白名单 TERM= 行全链兑现；--cwd 不存在时 validateStartup os.Stat 预检 exit 2（spawn 前零资源占用，纯函数纪律注释更新为「os.Stat 只读探测允许」）
- `wesh --stop-signal TERM --stop-timeout 1s --exit-when-empty -- sh -c 'trap "" TERM; ...'` 真实二进制冒烟（plan success_criteria 字面夹具 + while 循环夹具两形态）：WS attach→close 后 wesh 在 close+1002ms 以退出码 255 退出——TERM 被整组忽略（SIG_IGN 跨 exec 持久，见 Deviations #3）、1s 后 AfterFunc 补 SIGKILL、信号死亡 -1 → accept-255；默认 HUP+0 纯单信号现状由 emptyexit 既有六测试零改动全绿锁定
- `wesh --uid $(id -u) --gid $(id -g) -- sh -c 'id -u; id -g; echo $HOME $USER'` 真实二进制冒烟：uid/gid 与 self 全等、HOME/USER 取自 passwd 条目；`wesh --uid 65534 -- bash` → exit 2 文案 `--uid and --gid must be given together`（双 flag 名，成对强制零窗口暴露）
- 停止信号进程组语义锁定：SignalGroup(sig) 负 pid 组信号（setsid pgid==pid 既定不变量注释逐字沿用）+ ESRCH 幂等静默 + KILL 补发不占 hubMu 不阻塞 lifecycle（Pitfall 8）；「Close master 内核 SIGHUP」免费通道保留为兼容底层（lifecycle Drain→Close 零改动）
- SEC-06 替换式注入纪律零漂移：身份改写走白名单通道内替换（每键恰好一行、宿主干扰值不得继承），查不到 passwd 条目剔除三键（不 append，shell 自默认）；TestEnvWhitelist 既有两层断言（AWS_SECRET_ACCESS_KEY/WESH_CREDENTIAL 零泄露）全绿

## Task Commits

每个任务 TDD 两提交（RED/GREEN）+ 一次夹具机理勘误提交：

1. **Task 1 RED: StartOptions/--cwd/--term 失败测试** - `36464be` (test)
2. **Task 1 GREEN: StartOptions Dir/Term with --cwd/--term flags** - `548990c` (feat)
3. **Task 2 RED: SignalGroup/stop-signal/stop 序列失败测试** - `8ebe477` (test)
4. **Task 2 GREEN: configurable stop-signal sequence for process group shutdown** - `29a3c8d` (feat)
5. **Task 3 RED: uid/gid 降权与身份改写失败测试** - `ddbff57` (test)
6. **Task 3 GREEN: uid/gid privilege drop with identity env rewrite** - `d710bd5` (feat)
7. **夹具机理勘误: SIG_IGN 持久性发现更正 stopseq 注释** - `261a7e7` (docs)

**Plan metadata:** docs 提交在本 SUMMARY 之后（`docs(07-04): complete process-management plan`，hash 见 git log）。

## Files Created/Modified

- `internal/pty/spawn.go` - StartOptions 结构体（D-21/D-24 决策号 + creack/pty 兼容链注释）+ Start(argv, opts)（cmd.Dir 实装注释预留位；Uid>=0 分支 Credential + supplementary groups 策略注释）+ whitelistEnv(term, uid)（TERM= 参数化 + D-25 身份改写/剔除分支）
- `internal/pty/signal_linux.go` / `signal_darwin.go` - SignalHangup 泛化为 SignalGroup(sig)（五段注释纪律逐字沿用）+ StopSignalByName 四枚举映射（与 server.go signalName 方向差异登记；平台对件同签名同表）
- `internal/pty/signal_test.go`【新】- TestStopSignalByName（四命中+四拒绝）/ TestSignalGroup（WaitStatus 精确断言 TERM 送达 + ESRCH 幂等）
- `internal/pty/spawn_test.go` - 既有调用点适配新签名 + TestStartOptionsDir/Term/ZeroValueParity（Task 1）+ TestDropPrivilegesSelf/IdentityEnv/WhitelistEnvDropUnknownUid（Task 3）
- `internal/pty/io_test.go` - 调用点适配 + TestSignalHangup 机械换名 TestSignalGroupHangup（SignalGroup(SIGHUP) 送达语义锁定）
- `internal/pty/reap_test.go` / `reap_darwin_test.go` - 调用点零值等价适配（darwin 件 GOOS=darwin vet 交叉验证）
- `internal/server/server.go` - Options.StopSignal/StopTimeout（生产直传 + 零值兜底说明分档）+ New 兜底 SIGHUP + Server.stopSignal/stopTimeout 装配（07-05 复用注释标注）
- `internal/server/clients.go` - maybeExitWhenEmptyLocked 两触发点换入 + stopChildLocked 统一出口（Pitfall 8 注释登记：AfterFunc 异步、不占 hubMu、ESRCH 幂等）
- `internal/server/stopseq_test.go`【新】- TERM 送达（trap exit 43）与 KILL 补发（忽略 + 时序双断言）两行为测试 + 夹具纪律文件头（竞态/SIG_IGN 机理/选型）
- `internal/server/e2e_test.go` / `limits_test.go` / `resize_arb_test.go` / `sharetoken_test.go` - 测试 helper 零值等价适配 ×7
- `cmd/wesh/main.go` - config 六字段（cwd/term/stopSignal/stopTimeout/stopSignalSig/uid/gid）+ 六 flag 注册 + parse 期校验（stop-signal 枚举/stop-timeout 负值/uid/gid 值域）+ validateStartup（--cwd stat 预检 + uid/gid 成对强制）+ pty.Start/Options 接线 + parseArgs 头注释 22→28
- `cmd/wesh/main_test.go` - TestParseArgs 五断言位扩展（03-04 先例）+ TestStartupMatrix 五行 + TestTLSKeyPairError 七行

## Decisions Made

- **supplementary groups 环境感知策略**（详见 Deviations #2）——root 清空（既定最小权限语义逐字保持）/ 非 root NoSetGroups 跳过（零提权面论证 + 本机 EPERM 实测）；spawn.go Credential 分支注释登记 GOROOT 行号与裁决日期，07-08 人工 UAT 复核项联动。
- **stopChildLocked 统一出口**——两触发点换入时提取公共序列为函数（07-05 Shutdown 复用同一出口同一对字段），must_have key_link「Options 单一通道，双写即漂移」的函数级兑现。
- **parse 期拒绝行归属沿用**——plan 字面「TestParseArgs 新行（非法值/负值）」对错误行结构性不可达（表内 err 即 t.Fatalf），全部拒绝行落 TestTLSKeyPairError 错误表（03-04 既定归属先例第三次沿用），TestParseArgs 只收合法形态行。
- **grep==0 语义边界**——SignalHangup 删除连带注释字面同清（05-08 纪律：验收 grep 是源码级机械检查）；测试机械换名 TestSignalGroupHangup 保持送达语义锁定连续。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - 夹具竞态] stopseq KILL/TERM 测 trap 安装竞态修复（落盘标记同步）**
- **Found during:** Task 2 GREEN 验证（`go test -run 'TestExitWhenEmpty|TestStop'` 组合运行两新测试失败——单独运行通过，组合负载下翻车）
- **Issue:** 子进程经 setsid+exec 后安装 trap 需要非零时间，dialHello 完成不等价 trap 已就位——TERM 先于 trap 到达时 sh 按默认动作死亡（TERM 测收 -1 want 43；KILL 测在 stop-timeout 前收 -1）
- **Fix:** 两测试以落盘标记文件同步「trap 已安装」（`touch marker` 于 trap 之后；waitMarker 轮询 5s 护栏）——Phase 01-03 决策先例（stdout 标记在 WS 断开后被 onChunk 丢弃不可观测）
- **Files modified:** internal/server/stopseq_test.go
- **Verification:** 组合运行全绿；`go test ./internal/server -count=1` 46.7s 全绿
- **Committed in:** 29a3c8d

**2. [Rule 3 - 计划内矛盾] supplementary groups 策略：root 清空 / 非 root NoSetGroups 跳过**
- **Found during:** Task 3 GREEN 验证（TestDropPrivilegesSelf 失败：`fork/exec /usr/bin/id: operation not permitted`）
- **Issue:** plan flagged_assumptions 既定「NoSetGroups false 默认 = 清空附加组，有意为之」与 plan behavior 自身要求的「降权到 self 免 root」测试结构性矛盾——GOROOT exec_linux.go:496-499 在 Credential != nil && !NoSetGroups 时无条件 setgroups，而非 root 无 CAP_SETGID 必收 EPERM（本机实测命中）；非 root 下 --uid 任何形态（含降回自身）全部不可用
- **Fix:** 环境感知策略——euid==0 时 NoSetGroups=false（清空附加组：root 的附加组永非目标身份的组，保留即提权泄露，OPS-05 真实部署场景 root→nobody 语义逐字保持）；euid!=0 时 NoSetGroups=true（非 root 唯一可达降权是降回自身——setuid 他者必 EPERM——自身保留自身附加组零提权面）；白盒断言锁定策略（Credential.NoSetGroups == (euid != 0)）
- **Files modified:** internal/pty/spawn.go, internal/pty/spawn_test.go
- **Verification:** TestDropPrivilegesSelf PASS（id -u 输出相等）；`go test -race -count=1 ./internal/pty` 全绿；真实二进制冒烟降权 self 全链 PASS
- **Committed in:** d710bd5

**3. [Rule 1 - 探针误判自纠] 初版 setsid 探针误判单 sleep 夹具行为，冒烟修正并留档**
- **Found during:** Task 2 GREEN 后的 success_criteria 冒烟
- **Issue:** Task 2 期间以 `setsid sh -c '...' &` 探针判定「`trap "" TERM; sleep 100` 在 TERM 后 exit 0」（据此把 KILL 测夹具改为 while 循环并在注释/提交信息留论）；真实二进制冒烟显示 plan success_criteria 字面夹具在 close+1002ms 经 KILL 退出 255——单 sleep 夹具其实恒活：POSIX SIG_IGN 跨 exec 持久，`trap "" TERM` 使 fork+exec 的 sleep 继承忽略，整组免疫 TERM。初版探针受 setsid fork 语义 + fish 作业控制干扰（TERM 发到空进程组）误判
- **Fix:** stopseq_test.go 文件头夹具纪律改写为修正后机理（竞态是 KILL 测失败唯一根因；SIG_IGN 持久 vs 捕获型 trap exec 复位两形态互补）；while 循环形态保留（不依赖 SIG_IGN 机理的显式恒活，更易诊断）；独立 docs 提交留档更正
- **Files modified:** internal/server/stopseq_test.go（注释勘误，零语义改动）
- **Verification:** `go test ./internal/server -run 'TestExitWhenEmpty|TestStop' -count=1` 全绿；两夹具冒烟均 close+1002ms 退出 255
- **Committed in:** 261a7e7

**4. [Rule 3 - 归属机械调和] parse 期拒绝行落 TestTLSKeyPairError 而非 plan 字面 TestParseArgs**
- **Found during:** Task 2/Task 3 RED 编写
- **Issue:** plan 字面「TestParseArgs 新行（--stop-signal 非法值 + --stop-timeout 负值）」——TestParseArgs 表内 parseArgs 返回 err 即 t.Fatalf，错误行结构性不可达
- **Fix:** 拒绝行全部落 TestTLSKeyPairError 错误表（--stop-signal 小写/未知名、--stop-timeout 负值、--uid/--gid 值域四行共七行），TestParseArgs 只收合法形态（四枚举 + 正值 + 成对）——03-04『parse 期拒绝既定归属』先例第三次沿用
- **Files modified:** cmd/wesh/main_test.go
- **Verification:** 三测试全绿
- **Committed in:** 8ebe477 / ddbff57

---

**Total deviations:** 4 auto-fixed（1 夹具竞态 Rule 1，1 计划内矛盾 Rule 3，1 探针误判自纠 Rule 1，1 归属调和 Rule 3）
**Impact on plan:** 全部 must_have truths 与 prohibition 逐字达成——停止信号只发子进程进程组（负 pid + setsid 不变量 + ESRCH 幂等）；uid/gid 成对强制 exit 2 零窗口暴露；SEC-06 替换式注入严禁 os.Environ() 追加红线零漂移。Deviation #2 触及 flagged_assumptions 的 supplementary groups 条目：root 部署语义逐字保持，非 root 路径从「结构性不可用」修正为「降回自身可用且零提权面」——已列入 07-08 人工 UAT 复核联动（见 Next Phase Readiness）。

## TDD Gate Compliance

三 task 均 tdd="true"，gate 序列逐 task 核验（git log 顺序 + RED 失败证据）：

| Task | RED（test 提交，失败确认） | GREEN（feat 提交，全绿确认） | 序列 |
|------|---------------------------|------------------------------|------|
| 1 | `36464be`（pty/cmd 两包编译失败——新 API 未存在） | `548990c` | ✓ |
| 2 | `8ebe477`（三包含新 API 处编译失败） | `29a3c8d` | ✓ |
| 3 | `ddbff57`（三 pty 测试 + StartupMatrix 成对行运行期失败） | `d710bd5` | ✓ |

RED 阶段无测试意外通过（fail-fast 规则未触发）；Task 2/3 的 RED 为运行期失败形态（Task 1 已落地的字段使编译通过、行为未实现而失败）——同属有效 RED。REFACTOR 无需（GREEN 即最终形态；夹具机理勘误 `261a7e7` 为注释 docs 提交，非行为变更）。

## Issues Encountered

- **非 root setgroups EPERM（本 plan 最重要发现）：** GOROOT 在 Credential != nil && !NoSetGroups 时无条件 setgroups——任何非 root wesh 的 --uid 用法（含降回自身）全部 `fork/exec: operation not permitted`。修正见 Deviations #2；root 部署（OPS-05 真实场景）行为零变化。
- **初版探针方法论教训：** `setsid X &` 在 fish 交互作业控制下 setsid 会 fork（进程组长身份），$! 指向即刻退出的父进程，组信号发到空 pgid——探针结论与真实 PTY 环境分叉。信号类探针须以真实二进制（或显式 fork 控制）为准；本轮以 wesh 二进制自身为探针 harness 修正（Deviations #3）。
- **gofmt 零新增漂移：** 全程 GOROOT gofmt（/usr/bin/gofmt 陈旧版 CJK 注释规则差异，01-03 登记）；漂移清单仅剩 multi_test.go/slowclient_test.go 两既有文件，按 SCOPE BOUNDARY 未触碰。

## User Setup Required

None - no external service configuration required.

## Threat Flags

None——全部新表面均在 plan `<threat_model>` T-07-04a/b/c/d 四条登记内：负 pid 组信号 + setsid pgid==pid 不变量 + KILL ESRCH 幂等（T-07-04a mitigate，SignalGroup/WaitStatus 双测试锁定）；uid/gid 成对强制 exit 2 + 数字直通 + Credential fork 后 exec 前（T-07-04b mitigate，成对/值域测试锁定；supplementary groups 策略调整维持 root 侧缓解逐字不变，非 root 侧零提权面论证在案）；D-25 LookupId 改写/剔除（T-07-04c mitigate，三组测试锁定）；--cwd stat 预检 fail-fast（T-07-04d mitigate，矩阵两行锁定）。无未建模的信任边界扩张。

## Known Stubs

None——无占位实现；六条 must_have truths 全部经 Go 单测 + -race 全量 + 真实二进制冒烟达成（cwd/term 落点与预检 / stop-signal 组信号 + 补 KILL + 默认零漂移 / 进程组边界 / Credential 降权与成对校验 / 身份环境改写与剔除 / 未配置四 flag 逐字节零漂移）。

## Next Phase Readiness

- **07-05 直接可用面：** Server.stopSignal/stopTimeout 字段与 stopChildLocked 出口就位（Shutdown 复用同一 Options 通道同一函数——双写漂移结构性排除）；SignalGroup 泛化面完整（1001 关停序列直接消费）
- **07-06 配置文件六键：** RESEARCH Pattern 4 fileConfig 已预留 Cwd/Term/StopSignal/StopTimeout/Uid/Gid 键位——本 plan 的 parse 期校验函数（StopSignalByName/值域检查）与 validateStartup 两新行直接复用，零双写
- **07-07 UAT 素材：** stop-signal 宽限场景（trap 忽略 + KILL 计时观测）与降权场景（降权 self 免 root；降权 nobody 需 root 列可选）夹具形态本 plan 已验证（落盘标记同步纪律必须沿用）
- **07-08 人工 UAT 复核联动（两项）：** ① supplementary groups 策略裁决（Deviation #2——root 清空 / 非 root NoSetGroups 跳过的环境感知形态，flagged_assumptions OPS-05 条目按此修正后复核）；② flagged_assumptions 两项（--cwd 符号链接内核语义 / --term 任意字符串 / --stop-timeout 极大值 / 无登录 shell uid 降权）维持原登记，补充「单 sleep trap 夹具 SIG_IGN 继承机理」作为 stop-signal 场景构造指引
- **OPS-04/OPS-05 闭合：** 六 flag 全链可用、进程组语义锁定、降权身份环境直觉语义成立；--cwd 符号链接与 --term 合法性不立场等灰区已按 flagged_assumptions 登记

## Self-Check: PASSED

- 文件存在性：17/17 FOUND（signal_test.go 含 TestStopSignalByName/TestSignalGroup；stopseq_test.go 含 TestExitWhenEmptyStopSignalTERM/TestExitWhenEmptyStopTimeoutKills；spawn.go 含 type StartOptions struct ×1/syscall.Credential ×1/LookupId ×3；signal_linux.go/signal_darwin.go 各含 func (s \*Session) SignalGroup ×1 + StopSignalByName ×1；server.go 含 StopSignal/StopTimeout Options 字段与 New 兜底；clients.go 含 stopChildLocked ×1 定义 ×2 调用；main.go 含六 flag 注册/值域与成对校验/两处接线；main_test.go 含五断言位与十二新行）+ 本 SUMMARY FOUND
- 提交存在性：7/7 FOUND（36464be / 548990c / 8ebe477 / 29a3c8d / ddbff57 / d710bd5 / 261a7e7）
- must_have 机械断言：`grep -rn 'SignalHangup' --include='*.go' . | wc -l` == 0；`grep -c 'type StartOptions struct' internal/pty/spawn.go` == 1；`grep -c 'func (s \*Session) SignalGroup' internal/pty/signal_linux.go internal/pty/signal_darwin.go` 各自 == 1；`grep -c 'syscall.Credential\|LookupId' internal/pty/spawn.go` ≥ 1
- 全量验证：`go test -race -count=1 ./internal/pty`（2.6s）+ `go test -race -count=1 ./internal/server ./cmd/wesh`（49.8s+1.0s）全绿；`go vet ./...` 零告警；GOROOT gofmt 漂移清单 == 两既有文件（本 plan 零新增）；GOOS=darwin go vet ./internal/pty 交叉验证通过
- 冒烟三 success_criteria 逐条观测达成（--cwd/--term 输出 / stop-signal 两夹具 close+1002ms 退出 255 / 降权 self 身份全对 + --uid 单给 exit 2 双 flag 名）

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
