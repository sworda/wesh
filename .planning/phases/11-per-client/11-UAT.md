---
status: complete
phase: 11-per-client
source: [11-VERIFICATION.md]
started: 2026-09-04T02:30:00Z
updated: 2026-09-04T03:25:00Z
---

## Current Test

[testing complete]

## Tests

### 1. CI macOS leg darwin 测试实际结果确认

背景：11-02 的 dup-watch fail-closed 防御（errDupWatch + watch() dup 检查）代码/接线/编译面已全量核验
（GOOS=darwin build/vet 双闸通过），但两个 darwin-only 测试在本机（Linux）被 build tag 排除，
运行态不可观测。REVIEW IN-01 同指：竞态测试否定出路是 t.Skip 而非 FAIL，CI 绿不证明裁决成立。

expected: TestWatchDupPidFailClosed 与 TestKqueueExitZombieRace 实际运行且 PASS（CI macOS leg 日志或本机 macOS 实跑）；
若 TestKqueueExitZombieRace 为 SKIP，按 reap_darwin.go:12-15 兜底预案退化 awaitExit 并在代码注释锚定该裁决
result: pass
evidence: CI run 33832096581（macos-latest, go test -race -count=1 -v）：TestWatchDupPidFailClosed PASS (0.11s)、TestKqueueExitZombieRace PASS (1.09s, 非 SKIP)。Q1 裁决成立——kqueue 补发僵尸进程事件，reap_darwin.go:12-15 兜底预案条件不触发，awaitExit 无需退化（watcher 路径保持）。REVIEW IN-01 的「CI 绿不证明裁决」关切经 -v 日志逐测试行实证闭环。

### 2. TestPerClientTeardownRaceOnce macOS CI 失败（测试 1 调查副产物）

expected: Phase 11 全部测试在 CI macOS leg PASS（零回归收口闸的 darwin 运行面）
result: pass
reported: "CI run 33832096581 macOS leg: TestPerClientTeardownRaceOnce FAIL (0.01s)——perclient_test.go:1164: kill(-2650, 0) = operation not permitted, want ESRCH（进程组消失含收割完成）。Linux 本机同测试 PASS (1.30s, 14143fe 覆写形态)，macOS 首轮竞态注入即命中 EPERM 立即 Fatal"
severity: major
resolved: "G-11-2 修复链闭环——gap closure plan 11-07（TDD 对 afb77a8 test + 5aad25a fix：waitPgroupESRCHWithProbe 探针参数化核心，EPERM 归类「进程组仍存在」形态落入护栏轮询，护栏到期未 ESRCH 仍 Fatal + 其余错误立即 Fatal 两半边由 TestWaitPgroupESRCHProbeSemantics 四子测锁定）。CI 复验两轮：run 33843785651 macOS leg 被 ubuntu flaky 连带取消（ubuntu 上 TestPerClientInputEcho/TestPerClientSpawnFailure 因 shell 冷启动慢、PS1 晚于回显交错 `$ MARKER` 形态行首失配——纯测试时序 flaky 非生产面，9936f2b 以 `(?:\$ )?` 容忍修正，回显行排除与结果行锚定语义不变）；run 33844831146 全绿：macOS leg TestPerClientTeardownRaceOnce PASS (1.40s，33832096581 的 FAIL 现场测试)、DisconnectSIGHUP (0.04s)/ReconnectNewPid (0.05s)/StopTimeoutKillFallback (1.02s) 保持 PASS、TestWatchDupPidFailClosed PASS (0.11s)、TestKqueueExitZombieRace PASS (1.10s)、TestWaitPgroupESRCHProbeSemantics 四子测 PASS、internal/server ok (66.4s)；ubuntu leg 同绿。护栏翻车形态（EPERM 持续到期）未出现——EPERM 容忍修法充分性经运行面确认"

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-11-2
  truth: "Phase 11 测试套件在 CI macOS leg 全绿（darwin 运行面零回归）"
  status: closed
  reason: "已闭合（2026-09-04）：11-07 修复链（afb77a8 + 5aad25a）+ CI run 33844831146 macOS leg 全绿实证（TestPerClientTeardownRaceOnce PASS 1.40s = 33832096581 FAIL 现场测试转绿；同 helper 三调用点保持 PASS；internal/server ok 66.4s）——终局对账由 /gsd-verify-work 裁定"
  severity: major
  test: 2
  root_cause: "waitPgroupESRCH（perclient_test.go）把非 ESRCH 错误一律立即 Fatal，注释论断「同 uid 派生的进程组 EPERM 不可达，出现即环境异常」在 macOS 被证伪：XNU 上进程组成员处于退出过渡态（P_LIST_EXITED/proc_exit 进行中）时 kill(-pgid, 0) 对该瞬态返回 EPERM 而非 0（存活）或 ESRCH（消失）。TeardownRaceOnce 是唯一在 closeCh 确认后 µs 级即发首次 kill 的测试（exit 0 × Close 竞态注入设计使检查点紧贴退出窗口），故仅其命中；TestPerClientDisconnectSIGHUP (0.04s) 与 TestPerClientStopTimeoutKillFallback (1.01s) 同断言在 macOS PASS，排除环境级拦截（沙箱会拦截一切 kill）。POSIX 语义 EPERM = 目标存在但权限拒绝 ≠ 消失——立即 Fatal 属测试断言过严（平台兼容缺陷），生产收割语义未证实破坏（EPERM 瞬态意味着进程组尚在退出中，最终 ESRCH 收敛与否受护栏保护未被观测到）"
  artifacts:
    - path: "internal/server/perclient_test.go"
      issue: "waitPgroupESRCH 非 ESRCH 错误立即 Fatal 分支（EPERM 未按「目标存在」语义处理）；函数头注释「EPERM 不可达」论断需按 macOS 实测修正"
  missing:
    - "waitPgroupESRCH 将 EPERM 视为存活探针的一种形态：不 return 不 Fatal，fall through 到护栏到期检查（2s 内 ESRCH 仍收敛 PASS；到期未 ESRCH 仍 Fatal，僵尸残留检测能力保留）；其余非 ESRCH 错误维持立即 Fatal"
    - "函数头注释修正：EPERM 在 macOS 退出过渡态可达（引 CI run 33832096581 实证），「ESRCH ⊇ 死亡且收割完成」论证补充 macOS 组信号语义差异"
  resolution: "missing 两条全落地（5aad25a：探针参数化核心 + EPERM 护栏轮询归类 + 头注释五要点重写；四子测 TestWaitPgroupESRCHProbeSemantics 确定性锁定，本机 T1 复核 4/4 PASS + CI macOS leg 4/4 PASS）；护栏翻车形态未出现（33844831146 运行面确认修法充分性）"
  debug_session: ""
