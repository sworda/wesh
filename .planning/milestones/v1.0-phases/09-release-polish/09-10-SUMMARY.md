---
phase: 09-release-polish
plan: 10
subsystem: release-gate
tags: [regression, release-gate, six-stage, uat, fuzz, load, snapshot, ops-03, ops-10]

requires:
  - phase: 09-release-polish
    provides: "09-01 发布链定稿与 snapshot 断言组 / 09-03 D-18 三项清零 + phase06-dom D1-D13 / 09-04+09-05 --index 全行为面 / 09-06 负载矩阵 / 09-09 release.sh + README 定稿"
provides:
  - "发布前全绿证据链：六段式 + 全量 UAT 18 脚本 + fuzz 2×60s + load 矩阵 + snapshot 断言组五面同源复核——发布闸（Task 2 checkpoint:decision）呈堂证据"
  - "发布裁决记录：用户裁决 publish-later（2026-08-30）——v1.0.0 择机自行发布，发布操作指引在册（单命令 ./scripts/release.sh v1.0.0）；phase 以能力交付收尾"
affects: [ship（v1.0.0 就绪待发——发布指引在册，发布时由用户执行）, milestone v1.0（OPS-03/OPS-10 最后两条清零——44/44 全量收尾）]

actuals:
  tokens: 11650   # 全 plan 仓库内 realized diff 46,600 chars / 4（gofmt 五文件 42+/41- + SUMMARY 呈堂与定稿 + STATE/ROADMAP 追踪收口）；estimate 35000 高估约 3×——verification-only plan 的实际改动面远小于估算（验证成本在 agent 上下文不在 diff）
  tasks: 3        # Task 1 全量验证 + Task 2 发布闸（checkpoint:decision 用户裁决 publish-later）+ Task 3 publish-later 收尾
  commits: 4      # 8098244 style + 06bb4ed test + b06e645 docs 呈堂 + 终版 docs 定稿

tech-stack:
  added: []
  patterns:
    - "verification-only 收口 plan 的证据形态：五面（六段式/UAT/fuzz/load/snapshot）一次性同源复核，证据落 SUMMARY 作发布闸呈堂材料"
    - "GOROOT gofmt 漂移清零先例第六次沿用（02-06/03-06/05-09/08-05/09-10）——独立 style 提交 + git diff -w 零语义自证"
    - "publish-later 分支的 deferred 收尾形态：裁决记录 + 单命令发布指引（前置条件引用脚本自身 preflight 闸语义）+ 仓库零残留复核——发布能力交付与发布动作解耦"

key-files:
  created:
    - .planning/phases/09-release-polish/09-10-SUMMARY.md
  modified:
    - cmd/wesh/config_test.go          # gofmt 注释对齐（零语义）
    - internal/server/clients.go       # gofmt 字段对齐 + doc comment 列表缩进（零语义）
    - internal/server/emptyexit_test.go # gofmt 注释对齐（零语义）
    - internal/server/export_test.go   # gofmt CJK 全角括号前空格（零语义）
    - internal/server/load_test.go     # gofmt 注释对齐（零语义）

key-decisions:
  - "gofmt 漂移 5 文件按先例独立 style 提交清零（8098244）：全部为注释缩进/对齐/CJK 全角括号空格——go1.26 gofmt 规则差异，git diff -w 仅剩 clients.go 一处新增空注释行（doc comment 列表块分隔规范），零语义机械自证"
  - "Task 1 以 --allow-empty test 提交记录回归里程碑（06bb4ed，09-05 先例第二次沿用）：五面全绿证据进提交信息，verification-only 任务保持 per-task 原子提交协议"
  - "发布闸（Task 2 checkpoint:decision gate=\"blocking\"）用户裁决 publish-later（2026-08-30）——v1.0.0 择机自行发布，单命令 ./scripts/release.sh v1.0.0（前置：工作树干净、与远端同步）；phase 以能力交付收尾，release.yml 真实全链首证留待发布时（snapshot 已证形状——RESEARCH Pitfall 12 既定取舍，09-01 flagged_assumptions 既定验证取舍）"
  - "Blockers 节 Phase 5/6 标定挂账随本 plan 闭环（2026-08-30）：09-06 负载矩阵全量现值零证伪 + 09-09 README 标定表 12 行全量回填——STATE.md 挂账行标注闭合依据，milestone 无遗留挂账"

requirements-completed: [OPS-03, OPS-10]

duration: 40min    # Task 1 24min（2026-08-30T08:40..09:04Z）+ Task 3 收尾 ~16min（2026-08-30T10:30..10:46Z）；Task 2 checkpoint 用户裁决等待期不计执行时长
completed: 2026-08-30
status: complete
---

# Phase 9 Plan 10: 全量收口验证 + 发布闸 Summary

**发布前全绿证据链五面闭合（六段式六面全绿/全量 UAT 18 脚本 294 断言零 FAIL/fuzz 2×60s 双 PASS/load 矩阵 103s PASS/snapshot 断言组五层复演全过）+ 发布闸用户裁决 publish-later——v1.0.0 择机以 `./scripts/release.sh v1.0.0` 单命令自行发布（前置：工作树干净、与远端同步），phase 以能力交付收尾，milestone v1.0 44/44 需求全量清零**

## Task 1 五面证据（发布闸呈堂材料）

### 面 1：六段式全绿

| 段 | 命令 | 结果 |
|----|------|------|
| a) GOROOT gofmt | `bash -c '"$(go env GOROOT)/bin/gofmt" -l .'` | 零输出（既有漂移 5 文件已按先例清零——见 Deviations） |
| b) go vet | `go vet ./...` | PASS（五包零告警） |
| c) race 全量 | `go test -race -count=1 ./...` | 五包全绿：cmd/wesh 1.3s / internal/proto 1.0s / internal/pty 2.6s / internal/server 58.5s / web 1.0s（总 1m0.7s） |
| d) 前端构建 | `pnpm -C web install --frozen-lockfile && time pnpm -C web build` | install 467ms + build 2.3s，dist/index.html 500.07 kB（gzip 134.70 kB） |
| e) 裸 clone embed 链 | clone → `time go build ./...` + `go test ./...` | build 0.8s + 五包全绿（internal/server 53.9s）——dist 真实产物入库承诺实证 |
| f) 启动冒烟 | `--bind 127.0.0.1 --port 0` 启动行解析 + WS attach echo | 启动行解析 port=33281 → Welcome(mode=rw) → INPUT echo 标记串回显观测 PASS |

**flake 复演观察（orchestrator 通报）**：Wave 3 post-merge 曾现 internal/server shutdown 族偶发 FAIL 一次（负载相关疑似），随后三次重跑全绿——本轮全量 -race 单次全绿未复现，如实登记为「本轮观察点全绿」（零掩盖零改写）。

### 面 2：全量 UAT 回归（18 脚本零 FAIL）

| # | 脚本 | 计数 | skip（reason 登记） |
|---|------|------|------|
| 1 | phase02.mjs | 12/12 | — |
| 2 | phase03.mjs | 18/18 | — |
| 3 | phase04.mjs | 10/10 | — |
| 4 | phase04-dom.mjs | 37/37 | — |
| 5 | phase04-t1-width.mjs | T1 PASS（U11 4/4 + U6 对照 1/1） | — |
| 6 | phase05.mjs | 28/28 | 1（S7 像素层渲染——headless 硬约束豁免，人工清单 05-UAT.md） |
| 7 | phase05-dom.mjs | 19/19 | — |
| 8 | phase05-dims.mjs | DIMS PASS（D6H-1 等价锁 + D6H-2 负对照） | — |
| 9 | phase06.mjs | 23/23 | 1（S7 真实断网栈——headless 豁免，协议层等价物 S6） |
| 10 | phase06-dom.mjs | 40/40 | 2（D12b 真实 AT 栈 + D9 真实 OS 断网栈——平台原生豁免条款） |
| 11 | phase07.mjs | 34/34 | 1（S8c 真实弹浏览器——Windows 工作站人工层） |
| 12 | phase07-b1b5.sh | 7 PASS 0 FAIL | — |
| 13 | phase07-b2.mjs | 4/4 | — |
| 14 | phase07-b3.mjs | 3/3 | — |
| 15 | phase07-b6.sh | 7 PASS 0 FAIL | — |
| 16 | phase08.mjs | 21/21 | — |
| 17 | phase08-journal.mjs | 6/6 | — |
| 18 | phase09.mjs | 18/18 | — |

**合计 294 断言 PASS + 5 skip（全部有 reason 登记——CODEBUDDY.md 平台原生行为豁免条款引用 + 人工清单指向）**；二进制路径统一 /tmp/wesh-uat/wesh；各脚本 assertOutputClean 输出自净红线全过；`pgrep -x wesh` 零进程泄漏。

### 面 3：fuzz 短跑两调用（2×60s）

| 目标 | 包 | execs | 结果 |
|------|-----|-------|------|
| FuzzDecodeHello | internal/proto | 14,102,260（峰值 224,869/sec） | PASS 60.9s 零崩溃 |
| FuzzDecodeFileConfig | cmd/wesh | 337,805 | PASS 61.4s 零崩溃 |

### 面 4：load 矩阵全量

`go test -tags=load -count=1 -timeout=30m ./internal/server/` → **PASS 103.3s**（与 09-06 实测 104s 同量级）——fanout {1,4,16,32} 端逐字节一致 kicks=0 / legit-slow 限速读者零误踢 / memory bound Alloc 峰值 ≤64MiB / gate transitions 信用门不震颤 / defunct 三面（goroutine/fd/Z 态）回基线。

### 面 5：goreleaser snapshot 复演 + 09-01 断言组

`goreleaser check`（1 config validated）→ `time goreleaser release --snapshot --clean` 2.0s 成功 → 断言组五层全过：

- ① 命名族：`wesh_v0.0.0_{linux,darwin}_{amd64,arm64}.tar.gz` 恰 4 件
- ② checksums：`sha256sum -c checksums.txt` 四行全 OK
- ③ tar 三件套 ×4：每包恰含 wesh/LICENSE/README.md
- ④ 静态性：linux_amd64 = statically linked ELF（`ldd` → not a dynamic executable）；darwin 双产物 Mach-O x86_64 + arm64
- ⑤ 干净容器：`debian:stable-slim` 挂载运行 `--version` → exit 0，输出 `wesh 0.0.0-SNAPSHOT-8098244`（ldflags 注入生效、非 dev）

验毕 `rm -rf dist` 清理；工作树干净。

### ROADMAP 三准则终态核对

| SC | 准则 | 证据背书 |
|----|------|----------|
| SC1 | 四平台全静态二进制 + embed 单 HTML + scp 即跑 | 本轮面 5 断言组（snapshot + 干净容器双层） |
| SC2 | --index 生效 + 负载/模糊测试通过 | phase09.mjs 18/18（09-04/09-05 双层证据）+ 本轮面 3/面 4 |
| SC3 | 部署文档五配方（nginx/CF/Caddy/Docker/systemd） | 09-09 README 落文（实证分级——CF 唯一未实测标注），零漂移回归过 |

07 三项 UI WARNING 清零（D-18）：phase06-dom D1-D13 回归 40/40 全绿（含 D11a/D12/D13 行为锁）——v1.0.0 发布物无已知 WARNING 挂账。

## Task Commits

1. **gofmt 漂移清零（六段式段 a 先例路由）** - `8098244` (style)
2. **Task 1 全量收口验证（五面全绿回归里程碑）** - `06bb4ed` (test, --allow-empty——09-05 verification-only 先例)
3. **Task 1 SUMMARY 呈堂（五面证据初版落盘）** - `b06e645` (docs)
4. **Task 3 publish-later 收尾定稿（本文件终版 + STATE/ROADMAP 追踪收口）** - 终版 docs 提交（见 git log 最新 docs(09-10)）

## Task 2：发布闸（checkpoint:decision gate="blocking"）——用户裁决 publish-later

**裁决记录（2026-08-30）：publish-later——稍后自行发布。** phase 以能力交付收尾；release.yml 真实全链首证留待发布时（snapshot 已证形状——RESEARCH Pitfall 12 既定取舍）。resume-signal 解析：publish-later → Task 3 记录 deferred 收尾（publish-now 实发分支未启用）。

发布闸曾以 blocking 停止等待用户（Phase 05 协议违规记录：blocking 闸必须停止等待用户，先例不得作为自动通过依据）——本轮由用户明示裁决后续任 agent 执行 Task 3 publish-later 分支。

| 选项 | 内容 | 权衡 |
|------|------|------|
| publish-now | 立即以 `./scripts/release.sh v1.0.0` 执行真实发布（长 fuzz 2×10min → 负载矩阵 → 确认闸 → tag push 触发 release.yml + 公开 GitHub Release） | 发布链真实首证在本 phase 内闭合；milestone v1.0 实发收尾 / one-way 公开动作——产物即刻可查 |
| **publish-later（已选）** | 稍后自行发布（用户择机跑同一脚本） | phase 以能力交付收尾 / release.yml 真实全链首证留待发布时（snapshot 已证形状——RESEARCH Pitfall 12 既定取舍） |

## Task 3：publish-later 收尾——deferred 裁决记录 + 发布操作指引 + 仓库零残留确认

### ① deferred 裁决与发布操作指引（plan 字面路由）

**发布 v1.0.0 单命令：**

```
./scripts/release.sh v1.0.0
```

- **前置条件**（plan 字面）：工作树干净、与远端同步——即脚本 preflight 闸③（`git status --porcelain` 非空即拒）/ 闸④（落后远端即拒，ahead 放行——发布物本就是本地新增提交/tag；fetch 失败或无上游自动降级为跳过提示不阻塞）
- **脚本自动承载其余全部闸门**（D-14 定稿形态）：go vet + `go test -race -count=1 ./...`（与 CI 同口径）→ `pnpm -C web install --frozen-lockfile && pnpm -C web build`（dist 新鲜，embed 链本地验证）→ 长 fuzz ×2（FuzzDecodeHello/FuzzDecodeFileConfig 各 10 分钟，崩溃即中止——语料自动落 testdata/fuzz/）→ 负载矩阵（-tags=load，30 分钟上限）→ 确认闸（回显 tag + 最近 5 条提交，应答 yes 继续）→ `git tag v1.0.0 && git push origin v1.0.0` 触发 release.yml（pnpm build 先于 goreleaser：四平台全静态产物 + checksums.txt——D-01/D-03）
- **发布后核验面**（供参考，publish-now 分支验收口径）：`gh release view v1.0.0` 产物清单 = 4× `wesh_v1.0.0_{linux,darwin}_{amd64,arm64}.tar.gz` + `checksums.txt`（D-04 精确命名）；`sha256sum -c checksums.txt` 全 OK；linux_amd64 包 tar 三件套（wesh/LICENSE/README.md）+ 产物 `--version` 输出 `wesh 1.0.0`

### ② 仓库零残留确认

- `git status --porcelain` = 0 行（收尾提交后复核）
- 无 `dist/` 残留（snapshot 验毕已 `rm -rf dist`）；`/tmp/wesh-fresh-clone` 已清
- `pgrep -x wesh` 零进程泄漏
- `/tmp/wesh-uat` 为跨 phase UAT 常驻工作区约定路径（repo 外 /tmp，非仓库残留；phase 收尾保留，可随时手动清除）

### verify 结果（plan 字面命令，fish-safe bash -c if/else 形态）

```
git status --porcelain | grep -c . | grep -qx 0 && bash -c 'if git tag -l v1.0.0 | grep -q v1.0.0; then gh release view v1.0.0 ... | grep -qx 4; else echo "publish-later: no tag, deferred recorded"; fi'
```

→ 输出 **`publish-later: no tag, deferred recorded`**、exit 0（工作树 0 行 + 无 v1.0.0 tag——deferred 分支预期态，收尾提交后终态复核同过）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] GOROOT gofmt 既有漂移 5 文件（段 a 拦截）**
- **Found during:** Task 1 段 a（gofmt 检查）
- **Issue:** `gofmt -l` 非空——cmd/wesh/config_test.go、internal/server/{clients,emptyexit_test,export_test,load_test}.go 五文件存在排版漂移（09-02/09-06 后新提交累积 + go1.26 CJK 注释规则差异族）
- **Fix:** 按 plan 段 a 既定路由（02-06/03-06/05-09/08-05 先例第六次沿用）：`gofmt -w` 清零后独立 style 提交；`git diff -w` 自证仅剩 clients.go 一处新增空注释行（doc comment 列表块分隔规范）——零语义
- **Files modified:** 上述五文件（42+/41-）
- **Verification:** 复检 `gofmt -l .` 零输出；vet/race 全量随后全绿
- **Committed in:** `8098244`

---

**Total deviations:** 1 auto-fixed（Rule 3 先例路由的 gofmt 清零）——Task 2/Task 3 publish-later 分支零偏差（plan 字面执行：deferred 裁决与指引落 SUMMARY、仓库零残留）
**Impact on plan:** plan 段 a 自带该路由授权（「既有漂移若有按先例独立 style 提交清零」），交付物与 must_haves 逐字一致；无范围蔓延。

## Known Stubs

None —— 纯验证 plan，无占位/硬编码空值/TODO；五面证据全部为对真实构建产物的可执行断言结果。publish-later 为 plan 既定两分支之一（用户明示裁决），非 stub——发布能力（release.sh + release.yml + snapshot 证据链）已全量交付，发布动作本身按裁决 deferred。

## Self-Check: PASSED

- **Files:** 09-10-SUMMARY.md（本文件终版）/ cmd/wesh/config_test.go、internal/server/{clients,emptyexit_test,export_test,load_test}.go（gofmt 清零五文件）均 FOUND
- **Commits:** `8098244`（style）/ `06bb4ed`（test）/ `b06e645`（docs 呈堂）均 FOUND（git log --oneline）；Task 3 终版 docs 提交随后落于 HEAD
- **Task 3 verify（plan 字面命令）:** `publish-later: no tag, deferred recorded` + exit 0——工作树 0 行、无 v1.0.0 tag（deferred 分支预期态）
- **仓库零残留:** 无 dist/、`pgrep -x wesh` 零进程、/tmp/wesh-fresh-clone 已清（/tmp/wesh-uat 为 UAT 常驻工作区——repo 外约定路径非残留）
- **追踪收口:** STATE.md 70/70 plans 100%（status: verifying——phase 9 执行收尾待 verify 门）；ROADMAP Phase 9 10/10；REQUIREMENTS.md OPS-03/OPS-10 已 Complete（幂等复核）

---
*Phase: 09-release-polish*
*Task 1 completed: 2026-08-30T09:04Z — 发布闸呈堂*
*Task 2 裁决: 2026-08-30 — publish-later（用户）*
*Task 3 completed: 2026-08-30T10:46Z — publish-later 收尾定稿*
