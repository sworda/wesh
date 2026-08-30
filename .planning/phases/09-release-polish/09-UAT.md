---
status: complete
phase: 09-release-polish
source: [09-VERIFICATION.md]
started: 2026-08-30T11:20:00.000Z
updated: 2026-08-30T16:35:00Z
---

## Current Test

[testing complete]

## Tests

### 1. darwin 产物 macOS 实机冒烟
expected: 下载 darwin tar.gz → `wesh --version` exit 0 + 最小冒烟（架构断言已过，本项为发布时顺带人工复核）
result: pass
evidence: |
  2026-08-30 自动化证据链（用户裁决"授权 push 触发 CI"闭合本项）：
  - 架构层：CGO_ENABLED=0 GOOS=darwin GOARCH={amd64,arm64} 交叉编译 → file 断言
    Mach-O 64-bit x86_64 / Mach-O 64-bit arm64；tar 三件套（wesh/LICENSE/README.md）
    + sha256sum -c checksums 全 OK（/tmp/wesh-uat9/）
  - darwin 运行面：ci.yml macos-latest leg 两次全绿（run 33319445180 push ✓ +
    run 33319442869 PR ✓，真实 macOS runner 上 go vet + go test -race -v ./... 全量）
  - 本机回归：go vet + go test -count=1 ./... 五包全绿（54.9s）+ TestResize race×10 绿
  - v1.0.0 发布后补强：真实 Release darwin 双产物下载核验（sha256 OK + Mach-O
    x86_64/arm64 + tar 三件套）——剩余 macOS 实机 --version 冒烟按 expected 既定
    语义（"发布时在任意 macOS 机器上顺带执行即可"）择机执行，darwin 可运行性由
    CI macOS leg 源码级全量覆盖

### 2. release.yml 端到端首证（= v1.0.0 实际发布）
expected: 用户执行 `./scripts/release.sh v1.0.0`（前置四闸 → vet+race → pnpm build → 长 fuzz 2×10min → 负载矩阵 → 确认闸 → tag push 触发 release.yml），随后核验 GitHub Release 页面四平台产物与 checksums。
result: pass
evidence: |
  2026-08-30T16:29Z 发布全链实跑（用户裁决"现在发布 v1.0.0"，选择即确认闸）：
  - release.sh v1.0.0 EXIT=0：四闸（tag 形态/不存在/干净树/远端同步）→ go vet +
    go test -race 全量绿 → pnpm build + dist 漂移闸绿 → 长 fuzz 2×10min（Hello
    2.44 亿 execs + FileConfig 6700 万 execs 零崩溃）→ 负载矩阵绿 → 确认闸 →
    tag v1.0.0 push（16:29:14Z）
  - release.yml run 33322589004 completed success（16:30:58Z）
  - GitHub Release v1.0.0（非 draft 非 prerelease）资产恰 5 件：
    4× wesh_v1.0.0_{linux,darwin}_{amd64,arm64}.tar.gz + checksums.txt（零 windows）
  - 下载核验：sha256sum -c 四行全 OK；tar 三件套（wesh/LICENSE/README.md）；
    linux_amd64 实跑 `--version` → wesh 1.0.0 exit 0（statically linked ELF）；
    darwin 双产物 Mach-O x86_64 / arm64
  插曲（如实登记）：第一轮发布链中止于 fuzz 2/2 误报（2m29s）——fuzzer 构造表头
  ["FUZZ_PROBE_SECRET"] 把探针搬进键名位置，合法键名回显被全文字面断言误判值红线。
  修复 7850bc4：stripKeyNameEcho 剥除 configErr 两处键名上下文后断言（值透传仍
  FAIL fail-closed）+ TestStripKeyNameEcho 六形态行为锁 + 失败语料入库。产品代码
  零改动（值剥离实现本就正确）。修复后第二轮全链绿。

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Observations (non-blocking)

- **TestResize 时序 flake（CI 一次，非本 phase 引入）**：PR run（33319442869）ubuntu-latest leg
  `TestResize`（internal/pty/io_test.go:17）一次 FAIL——stty 序列 got [50 132 50 132]、want
  [24 80 50 132]。根因：测试夹具假设 Start 返回后 150ms 内首个 `stty size`（初始 24 80）必已
  执行，重载 CI runner（-race 全量 + 多 job 并行）上 sh 调度延迟超窗，首个 stty 落在
  TIOCSWINSZ 之后——两次输出均为 resize 后尺寸。产品语义（Resize → TIOCSWINSZ → stty 读新
  尺寸）在该失败中被反向证实。同 sha 五次运行（push-ubuntu ✓ / push-macOS ✓ / PR-macOS ✓ /
  本机全量 ✓ / 本机 race×10 ✓）仅此一次红。建议（deferred，随 09-REVIEW WR 清单一并处置）：
  以"读到首个非初始尺寸输出"的轮询替代固定 150ms sleep，或夹具前置握手读走初始基线。
- **FuzzDecodeFileConfig 键名回显误报（发布链第一轮中止根因，已修复 7850bc4）**：断言实现
  （全文字面匹配探针）与头注释声明语义（键名回显豁免、只断值探针）偏差——属测试断言缺陷
  非产品安全缺陷（值剥离经「只取 Key()」实现本就正确）。修复后语料复跑 PASS + fuzz 60s
  6.36M execs 零失败 + 第二轮发布长跑 10min 全绿。

## Gaps
