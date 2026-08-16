---
phase: 02-protocol
plan: 06
subsystem: docs
tags: [readme, documentation, verification, wesh.v1, websocket, closeout, go]

requires:
  - phase: 02-protocol
    provides: 02-01 proto 契约（帧/关闭码/上限常量）、02-02 握手与 ro 基线（--writable/前端 onclose 分派）、02-03 守卫链（子协议 400/per-IP 429/hello 超时）、02-04 保活（--ping-interval/pinger）、02-05 上限攻击面五测（1009/stderr 事件）
provides:
  - README 与 wesh.v1 wire 实际一致：子协议握手/Hello·Welcome·Error 帧/关闭码全集表（1001·1013 占位标注）/两档消息上限/ro 默认语义/保活说明
  - --writable 与 --ping-interval 两新 flag 的用户文档（默认值与 0=禁用语义）
  - 补偿控制文档化：无认证警示保持首屏（协议基线 ≠ 公网安全）+ per-IP 半开上限直连部署限制（Pitfall 6 义务）
  - Phase 2 全量收口六段式绿证据（逐条命令 + 退出码）与浏览器 UAT 五项清单（end-of-phase 待确认）
affects: [verify-work（Phase 2 期末验证）, Phase 3 认证与传输安全, Phase 9 反代配方文档]

actuals:
  tokens: 1609
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "六段式收口：GOROOT gofmt / go vet / -race 全量 / pnpm 构建 / git archive 裸 clone 编译测试 / 启动冒烟（--help flag 断言 + 200/400 + 无残留）"
    - "gofmt 段 1 授权分支：-l 有输出则 -w 修正后重跑（plan 显式授权的应急分支，非偏差）"
    - "冒烟进程管理教训形态：复合命令 & 拿到的是包装 subshell PID，pgrep -f 需锚定 '^/path/bin' 防包装 shell 命令串误命中"

key-files:
  created:
    - .planning/phases/02-protocol/02-06-SUMMARY.md
  modified:
    - README.md
    - internal/proto/proto.go

key-decisions:
  - "proto.go 的 02-01 既存 gofmt 差异（02-03/02-04 记录的格式噪音）随段 1 授权分支清零——纯注释排版修正，后续 plan 的 gofmt 门禁不再受历史噪音干扰"
  - "冒烟用 --port 0 随机端口 + 启动行解析端口驱动 curl 断言（避免默认 7681 占用冲突，断言不绑定固定端口）"

patterns-established:
  - "文档与 wire 一致性断言形态：README 关键串 grep 门禁（wesh.v1/--writable/--ping-interval/1009/无认证）随 plan verify 固化"

requirements-completed: [CORE-04, CORE-06, SEC-08, RES-01]

coverage:
  - id: D1
    description: "README 同步 wesh.v1 现状：协议节（子协议 400 预检/Hello·Welcome·Error 帧/关闭码全集表含 1001·1013 占位/预认证 4KiB·稳态 16KiB 上限/ping-pong 保活）+ --writable·--ping-interval flag 表 + 默认只读语义（服务端丢弃/[ro] 标题/RESIZE 放行）+ per-IP 直连部署限制 + 无认证警示首屏（协议基线 ≠ 公网安全）"
    requirement: CORE-04
    verification:
      - kind: other
        ref: "grep 门禁五串全中（wesh.v1/--writable/--ping-interval/1009/无认证）+ 五项 acceptance criteria 逐条 grep 通过"
        status: pass
    human_judgment: false
  - id: D2
    description: "全量收口验证六段式全绿：GOROOT gofmt -l 空 / go vet exit 0 / go test -race -count=1 ./... exit 0（server 23 测含握手·守卫·保活·上限全组）/ pnpm install+build exit 0 且 index.html.gz 在 / 裸 clone（最终 HEAD）build+test exit 0 / 冒烟（--help 两新 flag、单行启动、/ 200、无子协议 /ws 400、无残留）"
    requirement: RES-01
    verification:
      - kind: other
        ref: "段 1-6 逐条命令退出码全部 0（详见下方『全量验证记录』）；plan 级 verify 四连命令（vet && test -race && web build && gofmt-empty）exit=0"
        status: pass
      - kind: e2e
        ref: "go test -race -count=1 ./...（四包 ok，server 含 TestHelloWelcome/TestSubprotocolRequired/TestHalfOpenPerIP429/TestHelloTimeout/TestPrematureFrame/TestVersionMismatch/TestReadOnlyDropsInput/TestReadOnlyAllowsResize/TestPingKeepalive/TestPongTimeout/TestPingDisabled/TestOversize1009/TestReadLimitBoundary/TestFragmentedFlood1009/TestEmptyFragmentFloodResilience/TestPreHelloReadLimit）"
        status: pass
    human_judgment: false
  - id: D3
    description: "浏览器人工 UAT 五项清单（ro 默认与首帧 Hello 顺序/rw 可写/ro 下 RESIZE 跟随/关闭码文案分派/第二标签 409）已汇总待 end-of-phase 确认"
    requirement: CORE-06
    verification: []
    human_judgment: true
    rationale: "真实浏览器行为（[ro] 标题前缀、键盘无响应、DevTools WS 帧面板 Hello 首帧与 5s ping/pong、onclose 按码文案、第二标签 Unable to connect）无法由自动化断言替代——按 config human_verify_mode=end-of-phase 汇总，由用户在期末验证确认"

duration: 2h 36m
completed: 2026-08-15
status: complete
---

# Phase 2 Plan 06: 收口——README 同步 wesh.v1 + 全量验证六段式 Summary

**README 协议节重写为 wesh.v1 现状（子协议握手/Hello·Welcome·Error/关闭码全集/两档上限/保活），新增 --writable 与 --ping-interval 文档与默认只读、per-IP 直连限制两节，无认证警示保持首屏并补充「协议基线 ≠ 公网安全」；六段式收口（GOROOT gofmt/vet/-race 全量/web 构建/裸 clone/冒烟）逐条绿，Phase 2 以可推送状态收口**

## Performance

- **Duration:** 2h 36m
- **Started:** 2026-08-15T09:23:10Z
- **Completed:** 2026-08-15T11:59:10Z
- **Tasks:** 2
- **Files modified:** 2（README.md、internal/proto/proto.go）

## Accomplishments

- README 协议节直译 D-03/D-05/D-06/D-13~D-16 决策为 wesh.v1 现状文档：子协议 `wesh.v1` 缺失 HTTP 400；首帧 Hello（5s 超时 1008、抢跑/畸形 1002）；Welcome{mode}；五类帧表；Error 仅 version_mismatch/server_error 两可见码；关闭码全集表（1000/1002/1008/1009/1011 + 1001·1013 占位标注后续阶段启用）；预认证 4KiB/稳态 16KiB 上限（超限 1009 + stderr 单行事件）；ping/pong 保活（pong 超时 10s）
- 用法节 flag 表新增 `--writable`（默认 false 只读）与 `--ping-interval`（默认 5s，`0` = 禁用）两行；默认只读小节钉死「只读是服务端边界」——裸 WS 客户端 INPUT 同样被丢弃、`[ro] ` 标题前缀、RESIZE 放行
- 部署注意小节落地 Pitfall 6 文档化义务：per-IP 半开上限（默认 8，HTTP 429）直连部署有效，反代聚合为代理 IP 是已知限制，可信头透传归 SEC-07 后续阶段
- 无认证警示保持首屏醒目位置，措辞补充「Phase 2 协议基线已就位**不等于**可公网暴露」（T-02-21 补偿控制）
- 六段式全量收口逐条绿（命令 + 退出码见下），proto.go 的 02-01 既存 gofmt 差异随段 1 授权分支清零；裸 clone 段对最终 HEAD 复跑确认收口状态可重现

## Task Commits

1. **Task 1: README 同步 wesh.v1 协议语义与新 flag** - `7d2a8a0` (docs)
2. **Task 2: 六段式收口验证 + proto.go gofmt 修正** - `62eb40b` (style)

**Plan metadata:** 见下方 docs 提交（docs(02-06): complete ...）

## Files Created/Modified

- `README.md` - 警示段微调；flag 表 +2 行；协议节重写为 wesh.v1 现状；新增默认只读与部署注意两小节
- `internal/proto/proto.go` - GOROOT gofmt 注释排版修正（4 行，零逻辑改动）

## 全量验证记录（逐条命令 + 退出码）

| 段 | 命令 | 结果 |
|----|------|------|
| 1a | `"$(go env GOROOT)/bin/gofmt" -l .` | 首跑命中 `internal/proto/proto.go`（02-01 既存格式差异）→ 段 1 授权分支 `-w` 修正 → 重跑输出为空，exit 0 |
| 1b | `go vet ./...` | exit 0 |
| 2 | `go test -race -count=1 ./...` | exit 0（四包 ok；server 23 测——握手 TestHelloWelcome、守卫七测、保活三测、上限五测全部在列） |
| 3a | `pnpm -C web install --frozen-lockfile` | exit 0 |
| 3b | `pnpm -C web build` | exit 0（tsc + vite + gzip，280ms） |
| 3c | `test -f web/dist/index.html.gz` | exit 0（产物时间戳 2026-08-15 17:30 新鲜；构建确定性——git status 无 web/ 变更，段 6 无需产物提交） |
| 4 | `git archive HEAD → /tmp/wesh-clean && go build ./... && go test ./... -count=1` | 对最终 HEAD 复跑：build exit 0、test exit 0（四包 ok）；目录已清理 |
| 5a | `go build -o /tmp/wesh-bin ./cmd/wesh` | exit 0 |
| 5b | `/tmp/wesh-bin --help` | 含 `-writable` 与 `-ping-interval duration` 两行（Go flag 包规范单横线渲染，即两个新 flag） |
| 5c | `/tmp/wesh-bin --port 0 --writable --ping-interval 1s -- /bin/cat` 启动 | 输出恰一行且匹配 `listening on http://`（实测 `http://[::]:33007`） |
| 5d | `curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:33007/` | 200 |
| 5e | `curl ... /ws`（无子协议） | 400（子协议预检冒烟命中） |
| 5f | kill 后 `pgrep -f '^/tmp/wesh-bin'` | 空——无 wesh 进程残留，亦无残留 cat 子进程 |
| 闸 | plan 级 verify 四连：`go vet ./... && go test -race -count=1 ./... && pnpm -C web build && test -z "$(gofmt -l .)"` | exit 0（终态复跑） |

## 浏览器人工 UAT 清单（end-of-phase 待用户确认，对应 ROADMAP 准则 2/3 与 D-12①/D-14）

1. **只读默认**：`wesh -- bash` 后浏览器打开页面——标题带 `[ro] ` 前缀，键盘敲击终端无反应；DevTools → Network → WS 面板可见**首帧必为 Hello('H')**（若首帧是 RESIZE 则 helloSent 门失效——02-02 顺序硬约束的回归点，服务端将以 1002 frame_before_hello 断连）与 Welcome(mode=ro)，后续每 5s 一个 ping/pong 帧对。
2. **可写模式**：`wesh --writable -- bash` 刷新页面——键入命令正常回显，Welcome(mode=rw)。
3. **resize**：ro 模式下拖动浏览器窗口，web shell 里无法输入但全屏程序（先以 --writable 起 vim 观察）尺寸跟随——或直接经 DevTools 观察 RESIZE 帧照常发出。
4. **关闭码文案**：`wesh -- bash` 后在 shell 里 Ctrl-C 或让子进程 exit，页面显示 Session ended；用 wscat 或 DevTools 伪造 wesh.v9 版本 Hello（或不发 Hello 等 5s），页面分别显示 Connection refused（含 version_mismatch 文案）/被 1008 关闭——onclose 按码分派生效。
5. **单客户端**：另开第二个标签访问同地址显示 Unable to connect（409 语义不变）。

## ROADMAP Phase 2 成功准则证据归集

- **准则 1（两层硬顶 + 等效防线）**：02-05 五测自动化成立——16384/16385 边界精确、1 字节分片洪水在累积 16385 处 1009、5000 空消息洪水存活且内存平坦、预认证 4KiB 档 1009；本 plan 段 2 全量 -race 复跑全绿。
- **准则 2（ro 默认 + 关闭码集合）**：02-02 TestHelloWelcome（ro/rw 两半侧）+ 02-03 TestReadOnlyDropsInput/TestReadOnlyAllowsResize（服务端边界）自动化成立；线上关闭码集合 {1000,1002,1008,1009,1011} 经 README 全集表与 proto 常量注释双锚定；浏览器侧 [ro] 标题/onclose 文案并入上方 UAT 清单项 1/4 待确认。
- **准则 3（ping/pong 保活）**：02-04 三测（存活/pong 超时断开/0 禁用反证）自动化成立；devtools 观察 5s ping/pong 帧对并入 UAT 清单项 1 待确认。

## Decisions Made

- **proto.go 既存 gofmt 差异随收口清零**：02-03/02-04 均记录「proto.go 被 GOROOT gofmt 标出属 02-01 既存格式偏好差异，按 scope boundary 不顺手改」；本 plan 段 1 显式授权「如有输出则 -w 修正后重跑」，差异（`//（` → `// （` 注释排版，4 行零逻辑）随收口清零，后续 plan 的 gofmt 门禁不再受历史噪音干扰。
- **冒烟端口选型 `--port 0`**：随机端口 + 从启动行解析实际端口驱动 curl 断言，避免默认 7681 被占用造成的假失败；启动行格式断言（恰一行 + `listening on http://` 前缀）不受影响。

## Deviations from Plan

None - plan executed exactly as written（段 1 的 gofmt `-w` 修正与段 4 对最终 HEAD 的复跑均为 plan 文本显式覆盖的执行分支，非计划外工作）。

## Issues Encountered

- **冒烟 kill 目标的 harness 假象**：`rm ... && /tmp/wesh-bin ... &` 的 `&` 作用于整条复合命令，拿到的 PID 是包装 subshell 而非 wesh-bin 本体——首轮 kill 误杀包装壳，`pgrep -f wesh-bin` 还把 harness 自身内嵌命令串（含 "wesh-bin" 字样）报为残留。以 `pkill -f '^/tmp/wesh-bin'`（锚定路径）正确收口并复验无残留。与产品代码无关；已固化为 patterns-established（复合命令后台化 + pgrep 锚定形态）。
- **`--help` 渲染形态**：Go flag 包以单横线规范输出（`-writable`），plan 文本的 `--writable` 指 flag 本体——两行均在输出中，验收语义满足。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 仓库处于可推送状态：gofmt（GOROOT）/vet/-race 全量/前端构建/裸 clone/冒烟六段全绿，工作区 tracked 文件干净
- `/gsd:verify-work` 就绪：VALIDATION.md Per-Task Verification Map 全部 26 个测试行均有自动化绿证据；唯二待确认面为上方浏览器 UAT 五项（coverage D3 已标注 human_judgment）
- Phase 3（认证与传输安全）协议挂点齐备：子协议/Hello 未知字段忽略纪律（D-02）允许 ticket 字段无协议破坏加入；关闭码 1001/1013 占位与 per-IP 可信头（SEC-07）文档已为用户明示边界
- 无阻塞项

## Self-Check: PASSED

- [x] `README.md` 存在且含 wesh.v1（grep 计数 2）/ --writable / --ping-interval / 1009 / 无认证全部命中
- [x] 提交 `7d2a8a0`（docs README）/ `62eb40b`（style gofmt）均见于 `git log`
- [x] 两次任务提交的 `git diff --diff-filter=D HEAD~1 HEAD` 均为空（无意外文件删除）
- [x] 段 1-6 全部命令退出码已逐条记录（上表）；plan 级 verify 四连命令终态复跑 exit=0
- [x] 工作区 tracked 文件干净（仅 .planning/research/.cache 与 tags 等 plan 外未跟踪文件，非本 plan 产物）

---
*Phase: 02-protocol*
*Completed: 2026-08-15*
