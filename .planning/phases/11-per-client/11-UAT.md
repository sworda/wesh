---
status: testing
phase: 11-per-client
source: [11-VERIFICATION.md]
started: 2026-09-04T02:30:00Z
updated: 2026-09-04T02:30:00Z
---

## Current Test

number: 1
name: CI macOS leg darwin 测试实际结果确认
expected: |
  TestWatchDupPidFailClosed 与 TestKqueueExitZombieRace 在 macOS 上实际运行且 PASS；
  若 ZombieRace 为 SKIP，按 reap_darwin.go:12-15 兜底预案退化 awaitExit 并锚定裁决
awaiting: user response

## Tests

### 1. CI macOS leg darwin 测试实际结果确认

背景：11-02 的 dup-watch fail-closed 防御（errDupWatch + watch() dup 检查）代码/接线/编译面已全量核验
（GOOS=darwin build/vet 双闸通过），但两个 darwin-only 测试在本机（Linux）被 build tag 排除，
运行态不可观测。REVIEW IN-01 同指：竞态测试否定出路是 t.Skip 而非 FAIL，CI 绿不证明裁决成立。

expected: TestWatchDupPidFailClosed 与 TestKqueueExitZombieRace 实际运行且 PASS（CI macOS leg 日志或本机 macOS 实跑）；
若 TestKqueueExitZombieRace 为 SKIP，按 reap_darwin.go:12-15 兜底预案退化 awaitExit 并在代码注释锚定该裁决
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
