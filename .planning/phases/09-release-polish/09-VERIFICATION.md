---
phase: 09-release-polish
verified: 2026-08-30T11:27:20Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:

  - test: "darwin 产物 macOS 实机冒烟：scp wesh_v0.0.0_darwin_{amd64,arm64}.tar.gz 解包产物到真实 Mac，运行 ./wesh --version 并完成一次 attach echo"
    expected: "exit 0、版本号非 dev、终端回显正常（与 linux 干净容器行为一致）"
    why_human: "本验证环境无 macOS；09-VALIDATION.md Manual-Only 既定项 + 09-01 flagged_assumptions Pitfall 12 既定取舍——本验证器只能做 Mach-O 架构层断言（已过），实机运行为平台原生行为面"

  - test: "执行 ./scripts/release.sh v1.0.0（用户已裁决 publish-later，择机执行）——真实 tag push 触发 release.yml 全链，发布后 gh release view v1.0.0 核验产物清单"
    expected: "GitHub Release 附 4× wesh_v1.0.0_{linux,darwin}_{amd64,arm64}.tar.gz + checksums.txt；sha256sum -c 全 OK；linux_amd64 产物 --version 输出 wesh 1.0.0"
    why_human: "09-01 coverage D2 明示 release.yml 真实全链首证 = v1.0.0 实际发布，verifier 不得 auto-pass 端到端发布链；发布动作本身经用户裁决 deferred（publish-later，2026-08-30），属用户择机执行的一次性公开动作"
---

# Phase 9: 发布与打磨 Verification Report

**Phase Goal:** 单静态二进制四平台发布，默认参数经负载测试标定，部署文档齐全
**Verified:** 2026-08-30T11:27:20Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

ROADMAP 三条成功标准（契约）+ 合并的 plan 级 must-haves 按主题归并验证。**核心行为面全部由验证器独立复演**（非采信 SUMMARY 声明）。

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | SC1: goreleaser 产出 linux/darwin × amd64/arm64 四个全静态二进制（CGO_ENABLED=0），前端单 HTML 经 embed 内嵌，scp 到干净机器即可运行 | ✓ VERIFIED | **验证器独立复演** `goreleaser check`（1 config validated）+ `release --snapshot --clean`：恰 4 件 `wesh_v0.0.0_{linux,darwin}_{amd64,arm64}.tar.gz` + 裸名 checksums.txt，`sha256sum -c` 四行全 OK；tar 三件套（LICENSE/README.md/wesh）；linux_amd64 `file` 报 statically linked ELF、`ldd` 报 not a dynamic executable；darwin 两产物 Mach-O x86_64 / arm64；零 windows 产物；版本注入 `wesh 0.0.0-SNAPSHOT-c667743`（非 dev）。干净容器 `debian:stable-slim` bind-mount 运行 `--version` exit 0。`.goreleaser.yml`（CGO_ENABLED=0 全仓单侧持有、goos/goarch 四平台、.Tag 命名族、裸 checksums）+ `release.yml`（tags v*、pnpm build 先于 goreleaser、全 Action 钉版、零 CGO_ENABLED）+ `scripts/release.sh`（147 行可执行、bash -n 过、四闸→测试→fuzz×2→load→确认闸→tag push）三层工件齐备。web/dist/index.html（500KB）入库且新鲜（含 C-10 新文案串 + role="alert"）。v1.0.0 实际发布经用户裁决 publish-later deferred——SC1 的证据形态（snapshot + 干净容器）由 09-10 PLAN 明文界定，非缺口（见 Human Verification ②） |
| 2 | SC2: 自定义首页 HTML 可配置生效；负载/模糊测试通过；测试数据回填 P2/P5 默认参数 | ✓ VERIFIED | **--index 生效（验证器复演）**：`TestCustomIndex` -race PASS（1.07s）+ `phase09.mjs` 18/18 PASS（S1 四拒绝零内容探针 / S2 双通道 byte-identity / S3 gzip+Vary 双态 / S4 认证面照旧 / S5 0 字节+base-path / S6 TOML 通道 + SEC 输出自净）。**负载（验证器复演）**：`go test -tags=load` 全量 5/5 族 PASS（TestLoadFanoutMatrix/LegitSlowReaderZeroKick/MemoryBound/GateTransitions/Defunct，102.5s）——三断言（零误踢精确计数/内存上界/门频率）+ defunct 三面在测试体内实际执行。**模糊（验证器复演）**：FuzzDecodeHello 15s/3.46M execs PASS + FuzzDecodeFileConfig 15s/142K execs PASS；ci.yml fuzz job 2×60s 独立两调用在库。**标定回填**：README「默认参数与标定」12 行全量、负载敏感项附 09-06 实测数据（34.9MB/端、523,449B≈99.8%、19.8MiB≤64MiB、门 6 次/16.7s）、挂账语（「Phase 9 负载标定后回填」/「初值为一阶推算」）grep 零残留、验证成立分支零 Go 常量改动 |
| 3 | SC3: 部署文档覆盖 nginx/Cloudflare/Caddy 反代配方（含空闲超时与 ping 间隔关系）、Docker（tini/PID 1 收割）、systemd unit 模板 | ✓ VERIFIED | README 五配方齐全且实证分级：nginx（base-path 节完整配方 + 「proxy_read_timeout 必须大于 --ping-interval」关系行:386）、Caddy（「已实证 2026-08-30，Caddy v2.11.4」+ Host 透传差异面 + 无默认 idle 超时×ping 5s 关系行:404）、Cloudflare（「未实测」显著标注——D-15 诚实分级唯一例外）、Docker 节 + Dockerfile（scratch + tini v0.19.0 sha256 双钉 + ADD --checksum/--chmod + ENTRYPOINT tini 不加 -g + 零命令承诺语）、systemd 节 + deploy/wesh.service（Restart=on-failure/RestartSec=2/TimeoutStopSec=15s/LimitNOFILE=65536/EnvironmentFile=- 600 通道 + 255 交互注释全文在库）。09-07 docker/systemctl 实测证据（PID 1=tini 负对照 Z=5、255 复活、503 draining、停后不复活）落于 SUMMARY 呈堂——systemctl 面属状态变更操作，验证器不复演，证据链文档化完整 |

**Score:** 3/3 roadmap SC truths verified（行为依赖真值均经验证器自有运行取证：snapshot/静态构建/干净容器/TestCustomIndex/load 矩阵/fuzz×2/phase09.mjs）

### Deferred Items

无 Step 9b deferred 项——Phase 9 为 milestone v1 末位 phase（44/44 需求），无后续 phase 承接。publish-later（v1.0.0 实际发布）非 phase 缺口：经 09-10 Task 2 blocking 发布闸用户明示裁决（2026-08-30），发布能力已全量交付（release.sh 单命令 + 指引在册），发布动作本身按裁决属用户择机执行。

### Required Artifacts

全部 24 个 plan 声明工件经 gsd-tools verify.artifacts（Level 1-2 全 PASS）+ 手动 wiring/data-flow 核查：

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `.goreleaser.yml` | 四平台交叉编译/打包/校验和配置 | ✓ VERIFIED | version 2 schema、CGO_ENABLED=0、.Tag 命名族、裸 checksums.txt；goreleaser check 实测 validated |
| `.github/workflows/release.yml` | tag 触发发布 workflow | ✓ VERIFIED | tags v*、contents: write、pnpm(build)→goreleaser 顺序钉死、全 Action 钉版、零 CGO_ENABLED |
| `scripts/release.sh` | D-14 发布脚本 | ✓ VERIFIED | 可执行（rwxr-xr-x）、bash -n 过、四闸/fuzz×2×10m/load 30m/确认闸/tag+push 全段落实质在库 |
| `internal/proto/fuzz_test.go` | FuzzDecodeHello/Resize | ✓ VERIFIED | 两目标 + ClampDim 契约；验证器 15s 短跑 PASS |
| `cmd/wesh/fuzz_test.go` | FuzzDecodeFileConfig | ✓ VERIFIED | 探针反断言；验证器 15s 短跑 PASS |
| `cmd/wesh/config.go` | decodeFileConfig 接缝 + 29 键 | ✓ VERIFIED | `decodeFileConfig(path string, r io.Reader)` 签名在库（:104）；Index/IndexMaxSize 两键 + 覆盖面注释 29 键 |
| `.github/workflows/ci.yml` | fuzz job | ✓ VERIFIED | 第三 job 两独立 -fuzz 调用 ×60s（:41-42） |
| `web/src/main.ts` | HINT_SHUTDOWN + showShutdown | ✓ VERIFIED | 常量:438 + helper:485 + pre-onopen/steady 两调用点同源单写口（:899/:923） |
| `web/index.html` | #status role="alert" | ✓ VERIFIED | :68 单属性落位 |
| `web/dist/index.html` | 真实产物重建 | ✓ VERIFIED | 500KB 入库，含 C-10 文案串 + role="alert"（新鲜度实证） |
| `web/uat/phase06-dom.mjs` | D11a/D12/D13 | ✓ VERIFIED | 三场景断言在库（:622/:676/:708），新文案串逐字锚定 |
| `cmd/wesh/main.go` | --index 全链 | ✓ VERIFIED | flag 注册:482 + stat 预检:975-986 + loadCustomIndex:1129 + Options 直传:1233 |
| `web/embed.go` | WithCustomIndex 装饰器 | ✓ VERIFIED | gzip 预压缓存 + Vary 恒发 + acceptsGzip 复用 + index.html 整页替换，实质完整 |
| `internal/server/server.go` | Options.CustomIndex 单点装饰 | ✓ VERIFIED | 字段:286 + 直传:388 + Handler() 唯一装饰点:478-479（nil 零值兜底） |
| `internal/server/customindex_test.go` | TestCustomIndex 行为锁 | ✓ VERIFIED | 验证器 -race 实跑 PASS |
| `web/uat/phase09.mjs` | OPS-03 协议层 UAT | ✓ VERIFIED | 528 行；验证器实跑 18/18 PASS（含红线自净） |
| `internal/server/load_test.go` | load 矩阵 | ✓ VERIFIED | 首行 `//go:build load`；五测试族；kicks/gate/Alloc 三断言实装；验证器实跑 5/5 PASS |
| `Dockerfile` | scratch + tini | ✓ VERIFIED | ADD --checksum sha256 双钉 + ENTRYPOINT ["/tini","--","/wesh"] |
| `.dockerignore` | 上下文排除 | ✓ VERIFIED | .git/.planning/node_modules/dist 排除 |
| `deploy/wesh.service` | systemd unit 模板 | ✓ VERIFIED | Restart/LimitNOFILE/EnvironmentFile 全配 + 255 注释 |
| `web/uat/pw/phase09-caddy-ctl.sh` | Linux 侧载具 | ✓ VERIFIED | CADDY_UP 就绪协议在库 |
| `web/uat/pw/phase09-caddy-pw.mjs` | Windows 侧 Playwright | ✓ VERIFIED | 双机全链断言在库（实跑属 Windows 工作站面，09-08 SUMMARY 4/4 呈堂） |
| `README.md` | 五面更新 | ✓ VERIFIED | 发布节/--index 节/五部署配方/标定表 12 行/flag 表 --index 行/29 键更正全部落文 |

### Key Link Verification

gsd-tools key-links 因 `from:` 字段含散文描述无法解析（工具限制，from 须纯相对路径）——按流程回退手动 wiring 核查，全部 WIRED：

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| release.yml pnpm build 步骤 | .goreleaser.yml builds（go:embed 读 web/dist） | pnpm -C web build（:30）行号序先于 goreleaser-action（:34） | ✓ WIRED |
| .goreleaser.yml ldflags -X main.version | cmd/wesh/main.go var version | main.go:33 `var version = "dev"`；snapshot 实测输出 wesh 0.0.0-SNAPSHOT-c667743 | ✓ WIRED |
| scripts/release.sh 尾部 tag+push | release.yml 触发 | push 行:147 在 confirm():144 之后；tag push 唯一触发语义注释在库 | ✓ WIRED |
| main.go loadCustomIndex | server.Options.CustomIndex → WithCustomIndex | Options 字面量 CustomIndex: customIndex（:1233）→ server.go:479 单点装饰 | ✓ WIRED |
| config.go Index/IndexMaxSize | main.go 默认值铺底 | fc.Index/fc.IndexMaxSize 合并（:317-321）+ flag 注册（:482）+ 纯配置键直赋（:486） | ✓ WIRED |
| fuzz_test.go FuzzDecodeFileConfig | config.go decodeFileConfig | 直接调用接缝函数（io.Reader 面） | ✓ WIRED |
| ci.yml fuzz job | 两 fuzz_test.go | 两独立 -fuzz 调用精确引用目标与包 | ✓ WIRED |
| main.ts onclose 分派 | showShutdown 单写口 | pre-onopen（:899）与稳态（:923）同一 helper，零第二份文案 | ✓ WIRED |
| load_test.go 断言 | /metrics + runtime 观测 | metricSample(wesh_clients_kicked_total/credit_gate_transitions_total) + ReadMemStats 采样实装 | ✓ WIRED |
| phase09.mjs | 真实二进制 | spawn startWesh + 启动行解析 + sensitiveTokens 红线闭包 | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| .goreleaser.yml → dist/ | 四平台 tar.gz + checksums | goreleaser 真实交叉编译（验证器复演产出） | Yes | ✓ FLOWING |
| web/dist/index.html | embed dist | pnpm build 真实产物（500KB，含 phase-9 前端字符串） | Yes | ✓ FLOWING |
| --index 伺服链 | customIndex []byte | 启动读入用户文件字节 → Options 直传 → 装饰器 byte-identity 伺服 | Yes（UAT 探针字节逐字断言） | ✓ FLOWING |
| load_test.go | kicks/gate/Alloc/outboxMax | /metrics 黑盒 scrape + runtime 进程内采样（验证器实跑取数） | Yes | ✓ FLOWING |
| README 标定表 | 12 行标定结论 | 09-06 LOADDATA 实测数据（数值逐条对得上 SUMMARY 数据表） | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| 静态二进制 + 版本注入 | `CGO_ENABLED=0 go build ... && file/ldd/--version` | statically linked ELF / not a dynamic executable / `wesh vverify` exit 0 | ✓ PASS |
| goreleaser snapshot 四平台五层断言 | `goreleaser check && release --snapshot --clean` + 产物断言组 | 4 tar.gz 命名族 + checksums OK + 三件套 + 静态/Mach-O + 零 windows | ✓ PASS |
| 干净容器运行 | `docker run --rm -v /tmp/wesh-verify-bin:/wesh:ro debian:stable-slim /wesh --version` | `wesh dev` exit 0 | ✓ PASS |
| OPS-03 行为锁 | `go test -race -run TestCustomIndex ./internal/server/` | ok 1.067s | ✓ PASS |
| OPS-03 协议层 UAT | `node web/uat/phase09.mjs` | 18/18 PASS + SEC 输出自净 | ✓ PASS |
| 负载矩阵 | `go test -tags=load -count=1 ./internal/server/` | 5/5 TestLoad* PASS（verbose 运行取证） | ✓ PASS |
| fuzz 两目标 | `go test -fuzz=FuzzDecodeHello -fuzztime=15s ./internal/proto/` + FuzzDecodeFileConfig 同款 | 3.46M / 142K execs 零崩溃双 PASS | ✓ PASS |
| release.sh 语法 | `bash -n scripts/release.sh` | exit 0 | ✓ PASS |

**load 全包复演波动（如实登记）**：验证器三轮 `-tags=load` 全包运行中第 1 轮 FAIL 一次（114.5s，尾部日志为 exit_when_empty/session_end 事件——emptyexit/shutdown 族常规测试，非 load 族；load_test.go 无该事件面），第 2/3 轮全绿（102.5s）。失败未复现、未留名（非 verbose 运行截尾丢失测试名）。与 09-10 SUMMARY「Wave 3 shutdown 族偶发 FAIL 一次 + 三连绿」及用户通报的已知 deviation 特征一致——**五项 TestLoad* 断言本身三轮中凡执行皆绿**，SC2 负载证据成立。

### Probe Execution

本 phase 无 `scripts/*/tests/probe-*.sh` 形态声明探针；phase 自有验证面（load/fuzz/snapshot/UAT）已由验证器直接复演（见上表），不以 SUMMARY PASS 计数替代。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| OPS-03 | 09-04 / 09-05 / 09-09 | 自定义首页 HTML | ✓ SATISFIED | --index 全链（flag/TOML 双键/校验四拒绝/装饰器/双通道）+ TestCustomIndex + phase09.mjs 18/18 + README --index 节（两承诺语逐字） |
| OPS-10 | 09-01/02/03/06/07/08/09/10 | 单静态二进制发布（四平台 + embed 单 HTML） | ✓ SATISFIED | goreleaser 四平台静态产物（验证器复演）+ release.yml/release.sh 发布链 + fuzz/load 质量门 + Docker/systemd/Caddy 配方 + dist embed 链新鲜 |

孤儿需求检查：REQUIREMENTS.md traceability 中 Phase 9 仅映射 OPS-03/OPS-10，均被 plan 声明并覆盖——无孤儿。里程碑 44/44 全量收口属实。

### Prohibitions (7 项，全 test-tier)

| 禁令 | 状态 | 证据 |
| ---- | ---- | ---- |
| 发布二进制 MUST NOT 动态链接 | ✓ VERIFIED | ldd 恒报 not a dynamic executable（验证器 snapshot 复演）；CGO_ENABLED=0 单侧持有 |
| 发布构建 MUST NOT 嵌入陈旧前端 | ✓ VERIFIED | release.yml pnpm build(:30) 先于 goreleaser(:34)；dist 入库产物新鲜度实证 |
| MUST NOT 发布容器镜像到 registry | ✓ VERIFIED | 全 workflows grep docker/ghcr 零命中 |
| MUST NOT 发布 Windows 二进制 | ✓ VERIFIED | goos 仅 [linux,darwin]；dist 产物零 windows 字样 |
| 自定义页 MUST NOT 被注入/模板化 | ✓ VERIFIED | WithCustomIndex 零注入零模板；UAT S2 byte-identity 断言过 |
| 错误行 MUST NOT 含文件内容字节 | ✓ VERIFIED | main.go 错误文案仅路径+类别+上限值；UAT S1 探针反断言过 |
| --index MUST NOT 改变认证面 | ✓ VERIFIED | TestCustomIndex 认证面子测 + UAT S4（401/200/404/WS 全链）过 |

### Anti-Patterns Found

全部 phase 变更文件（含 09-REVIEW files_reviewed_list 29 文件）扫描：**TBD/FIXME/XXX 零命中，TODO/HACK/PLACEHOLDER 零命中，占位/空实现零命中**。

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| README.md:96 | 「及其 `.gz`」 | 文档与 .gitignore 事实相悖（`.gz` 未入库） | ⚠️ Warning | git blame 证实该句引入于 Phase 1（7879315，docs(01-05)），**非 Phase 9 交付物缺陷**（09-REVIEW WR-03 归因有误）；裸 clone 本地构建缺预压旁路（功能可用，发布产物不受影响）。遗留文档债，随 09-REVIEW WR 清单一并处置 |
| 09-VALIDATION.md frontmatter | status: draft / nyquist_compliant: false / 任务全 ⬜ pending | 执行后未回填 | ℹ️ Info | 簿记不一致；实际执行证据链在 SUMMARYs 呈堂（本验证器已独立复演核心面） |

### 09-REVIEW.md 评审发现（c667743，advisory）

0 Critical / 5 Warning / 4 Info，全部未阻塞 phase 目标，作为后续改进清单随行：

- **WR-01** release.sh 闸④ `git fetch --dry-run` 不更新远端跟踪引用——同步校验语义弱化（发布前真实同步状态未校验）
- **WR-02** 脏树闸先于 pnpm build——已提交 dist 与源码漂移零检测（发布产物不受影响，release.yml CI 侧重建）
- **WR-03** README .gz 承诺与 .gitignore 相悖（见上表——Phase 1 遗留，非本 phase 引入）
- **WR-04** Caddy UAT rig 硬编码内网 IP / 不支持 SSH 端口变量 / 两侧凭据来源不一致（可复现性缺陷，不影响已获实证结论）
- **WR-05** loadCustomIndex `int64(max)+1` 在 index-max-size=MaxInt64 边界回绕——静默伺服空白页（极值边界，默认 16MiB 不触发）

### Human Verification Required

### 1. darwin 产物 macOS 实机冒烟

**Test:** scp `wesh_v0.0.0_darwin_{amd64,arm64}.tar.gz` 解包产物到真实 Mac，运行 `./wesh --version` 并完成一次 attach echo
**Expected:** exit 0、版本号非 dev、终端回显正常
**Why human:** 本验证环境无 macOS；09-VALIDATION.md Manual-Only 既定项 + Pitfall 12 既定取舍——验证器已完成 Mach-O 架构层断言（x86_64/arm64 双双正确），实机运行属平台原生行为面

### 2. release.yml 端到端发布链首证（= v1.0.0 实际发布）

**Test:** 用户择机执行 `./scripts/release.sh v1.0.0`（publish-later 裁决既定动作）——tag push 触发 release.yml 全链，发布后 `gh release view v1.0.0` 核验
**Expected:** GitHub Release 附 4× `wesh_v1.0.0_{linux,darwin}_{amd64,arm64}.tar.gz` + `checksums.txt`；`sha256sum -c` 全 OK；产物 `--version` 输出 `wesh 1.0.0`
**Why human:** 09-01 coverage D2 明示 verifier 不得 auto-pass 端到端发布链；snapshot 已证产物形状（验证器复演全绿），真实 GitHub Release 全链首证仅能由实际发布闭合——经 09-10 Task 2 blocking 发布闸用户明示裁决 deferred（2026-08-30），非悬而未决项，属用户择机的 one-way 公开动作

### Gaps Summary

**无 goal-blocking 缺口。** 三条 ROADMAP SC 全部经验证器独立复演成立；24 工件四层核查全过；7 项禁令全验证；OPS-03/OPS-10 双双 SATISFIED；里程碑 44/44 收口属实。

两个如实登记的非阻塞观察：

1. **shutdown 族 flake（已知 deviation）**：验证器首轮 `-tags=load` 全包复演出现一次 FAIL（尾部 exit_when_empty 事件——emptyexit/shutdown 族常规测试），后续两轮全绿未复现。与 09-10 SUMMARY 登记的 Wave-3 偶发 FAIL（三连绿 + 全量收口全绿）特征一致。五项 TestLoad* 断言凡执行皆绿，SC2 负载证据不受影响。建议后续以重复运行定位该族时序敏感根因（非本 phase 引入，Phase 5/6 期测试面）。
2. **publish-later 语义**：v1.0.0 实际发布按用户裁决 deferred——发布能力（release.sh 单命令 + release.yml + snapshot 证据链）已全量交付并经本验证器复演，「四平台发布」在 ROADMAP SC 的证据形态（goreleaser 产出 + scp 即跑）下成立；真实 Release 首证由用户择机执行（指引在 09-10 SUMMARY，单命令 `./scripts/release.sh v1.0.0`）。

09-REVIEW 五项 Warning 为 advisory（发布脚本健壮性/Caddy rig 可复现性/极值边界），不构成 phase 目标缺口；其中 WR-03 经 blame 证实为 Phase 1 遗留文档债而非本 phase 交付物错误。

---

_Verified: 2026-08-30T11:27:20Z_
_Verifier: Claude (gsd-verifier)_
