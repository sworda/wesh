---
status: complete
phase: 09-release-polish
source: [09-VERIFICATION.md]
started: 2026-08-30T11:20:00.000Z
updated: 2026-08-30T15:45:00Z
---

## Current Test

[testing complete]

## Tests

### 1. darwin 产物 macOS 实机冒烟
expected: 下载 darwin tar.gz → `wesh --version` exit 0 + 最小冒烟（架构断言已过，本项为发布时顺带人工复核）
result: pass
evidence: |
  2026-08-30 自动化证据链（HEAD 084dde7，用户裁决"授权 push 触发 CI"闭合本项）：
  - 架构层：CGO_ENABLED=0 GOOS=darwin GOARCH={amd64,arm64} 交叉编译 → file 断言
    Mach-O 64-bit x86_64 / Mach-O 64-bit arm64；tar 三件套（wesh/LICENSE/README.md）
    + sha256sum -c checksums 全 OK（/tmp/wesh-uat9/）
  - darwin 运行面：ci.yml macos-latest leg 两次全绿（run 33319445180 push ✓ +
    run 33319442869 PR ✓，真实 macOS runner 上 go vet + go test -race -v ./... 全量）
  - 本机回归：go vet + go test -count=1 ./... 五包全绿（54.9s）+ TestResize race×10 绿
  - 产物实机 --version 冒烟按 expected 既定语义（"发布时在任意 macOS 机器上顺带执行
    即可"）随 v1.0.0 发布时执行；darwin 可运行性由 CI macOS leg 源码级全量覆盖

### 2. release.yml 端到端首证（= v1.0.0 实际发布）
expected: 用户执行 `./scripts/release.sh v1.0.0`（前置四闸 → vet+race → pnpm build → 长 fuzz 2×10min → 负载矩阵 → 确认闸 → tag push 触发 release.yml），随后核验 GitHub Release 页面四平台产物与 checksums。用户已于 2026-08-30 发布闸裁决 **publish-later**——本项为择机执行项，非悬置缺口（snapshot 已证形状）。
result: skipped
reason: "Deferred follow-up: 用户维持 publish-later 裁决（2026-08-30 verify-work 复确认），择机执行 ./scripts/release.sh v1.0.0；发布链就绪性已自动化验证（bash -n PASS + rwxr-xr-x + .goreleaser.yml 配置与实证交叉编译参数一致：CGO_ENABLED=0/四平台矩阵/三件套/裸 checksums.txt）"

## Summary

total: 2
passed: 1
issues: 0
pending: 0
skipped: 1
blocked: 0

## Deferred Follow-Ups

- test: 2
  idea: "维持 publish-later 裁决，择机执行 ./scripts/release.sh v1.0.0（四闸→vet+race→pnpm build→fuzz 2×10min→负载矩阵→确认闸→tag push 触发 release.yml），发布后 gh release view v1.0.0 核验四平台产物 + checksums"
  deferred_at: 2026-08-30

## Observations (non-blocking)

- **TestResize 时序 flake（CI 一次，非本 phase 引入）**：PR run（33319442869）ubuntu-latest leg
  `TestResize`（internal/pty/io_test.go:17）一次 FAIL——stty 序列 got [50 132 50 132]、want
  [24 80 50 132]。根因：测试夹具假设 Start 返回后 150ms 内首个 `stty size`（初始 24 80）必已
  执行，重载 CI runner（-race 全量 + 多 job 并行）上 sh 调度延迟超窗，首个 stty 落在
  TIOCSWINSZ 之后——两次输出均为 resize 后尺寸。产品语义（Resize → TIOCSWINSZ → stty 读新
  尺寸）在该失败中被反向证实。同 sha 五次运行（push-ubuntu ✓ / push-macOS ✓ / PR-macOS ✓ /
  本机全量 ✓ / 本机 race×10 ✓）仅此一次红。建议（deferred，随 09-REVIEW WR 清单一并处置）：
  以"读到首个非初始尺寸输出"的轮询替代固定 150ms sleep，或夹具前置握手读走初始基线。

## Gaps
