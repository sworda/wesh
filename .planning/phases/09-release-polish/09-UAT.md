---
status: testing
phase: 09-release-polish
source: [09-VERIFICATION.md]
started: 2026-08-30T11:20:00.000Z
updated: 2026-08-30T11:20:00.000Z
---

## Current Test

number: 1
name: darwin 产物 macOS 实机冒烟
expected: |
  下载 GitHub Release 的 darwin_amd64 / darwin_arm64 tar.gz，解压后在真实 macOS 上执行
  `wesh --version`（exit 0，版本号正确）与最小冒烟 `wesh --port 7681 -- bash --norc --noprofile`
  后浏览器可连接使用。架构层断言（双 Mach-O、tar 三件套）已由 verifier 独立复演通过；
  本项为 09-VALIDATION Manual-Only 既定项——发布时在任意 macOS 机器上顺带执行即可。
awaiting: user response

## Tests

### 1. darwin 产物 macOS 实机冒烟
expected: 下载 darwin tar.gz → `wesh --version` exit 0 + 最小冒烟（架构断言已过，本项为发布时顺带人工复核）
result: [pending]

### 2. release.yml 端到端首证（= v1.0.0 实际发布）
expected: 用户执行 `./scripts/release.sh v1.0.0`（前置四闸 → vet+race → pnpm build → 长 fuzz 2×10min → 负载矩阵 → 确认闸 → tag push 触发 release.yml），随后核验 GitHub Release 页面四平台产物与 checksums。用户已于 2026-08-30 发布闸裁决 **publish-later**——本项为择机执行项，非悬置缺口（snapshot 已证形状）。
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
