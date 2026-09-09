---
phase: 08-observability
plan: 05
subsystem: testing
tags: [uat, prometheus-exposition, healthz, slog, audit-events, readme-ops, regression, observability]

requires:
  - phase: 08-observability
    plan: 04
    provides: /metrics 手写 exposition 17 series 契约 + 认证闸跟随 + build_info{version}（S2/S3 断言面对象）；/healthz 免认证+draining 行为锁（08-03，S1/S4 断言面对象）；slog JSON 事件目录与 parseEvents 迁移形态（08-01/08-02，S5/S6 断言面对象）
provides:
  - web/uat/phase08.mjs：phase08 协议层 UAT 六场景 21 断言（S1 healthz 免认证+四字段+bp 固定+405 / S2 metrics 认证闸两态 / S3 exposition 17 series+数值锁+bp 固定 / S4 关停中 503 draining 确定性窗口 / S5 审计事件 JSON 检索 auth_failed·throttled·attach/detach·session_end / S6 控制字符剥离回归）+ PHASE08_ONLY 调试过滤 + assertOutputClean 运行时自净
  - README「## 运维（Phase 8）」节：健康检查/指标/结构化日志三小节——D-07 免认证唯一例外明示、Prometheus basic_auth 配方与 429 自锁明示（RESEARCH Pitfall 6 义务）、journald+jq 检索两则、红线重申
  - .planning/phases/08-observability/08-UAT.md：三条 flagged assumption 的人工复核路由（真实 Prometheus scrape / journald 实机检索 / draining 窗口编排观测率）
  - phase05-dom.mjs D5 踢出检测迁移修复（08-02 D-21 折入的漏检消费者——本 plan 全量回归捕获）
  - GOROOT gofmt 清零（multi_test/slowclient_test 既有漂移，deferred-items 既定路由终点）
affects: [Phase 9 发布与首页（OPS-03/OPS-10 在运维节与 build_info plumbing 之上收口）, phase 级 verify/ship 流程（08-UAT.md 三人工项为 uat-passed 谓词输入）]

actuals:
  tokens: 16188
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "确定性 draining 窗口夹具：trap 忽略 stop-signal（SIG_IGN 跨 exec 持久）+ --stop-timeout 3s 同步 sleep 拉宽关停窗口 → SIGTERM 后 50ms 轮询 /healthz 窗口内必观测 503（08-RESEARCH OQ3 定案 + 08-03 冒烟同款）"
    - "运维端点 UAT 形态：fetch 状态码/Content-Type/JSON 键集白名单（healthzOkShape 恰四键）+ exposition HELP 行逐字 17 series + metricValue name+空格前缀精确行解析（08-04 Go 侧同构镜像）"
    - "审计事件 UAT 断言形态：节流确定性驱动（首 401 + 快速连发 429 不 recordFail）+ waitEvent 轮询 stderr 异步落流竞态防线 + 速退进程 child.exitCode 兜底（exit 事件先于监听挂载竞态）"
    - "C1 线形探针的 UAT 双侧断言：NEL_WIRE 双码点上线（07-07 实证）→ JSON 解析后 remote_user 值断言 + remote 逐码点无 C0/C1/DEL 属性断言（proxy_test.go 属性断言的 UAT 镜像）"

key-files:
  created:
    - web/uat/phase08.mjs
    - .planning/phases/08-observability/08-UAT.md
  modified:
    - README.md
    - web/uat/phase05-dom.mjs
    - internal/server/multi_test.go
    - internal/server/slowclient_test.go

key-decisions:
  - "S4 draining 夹具取 trap 忽略 HUP + --stop-timeout 3s 组合（plan 字面 '--stop-timeout 3' 的意图兑现）——DurationVar 要求单位后缀；且进程终结由 lifecycle 子进程死亡路径收口（P1 硬约束），无 trap 时 bash 速死窗口 <1s（08-03 已实证），组合夹具才使 3s 窗口确定性成立"
  - "S2/S5 凭据场景排序即解零 pacing（phase07 S1 先例）：成功链路先行（recordSuccess 清零节流），401 负面对照排最后；S5 打满 429 取 phase03 场景 2 确定性形态（首 401 + 快速连发撞窗，429 不 recordFail 不延长窗口）后 sleep 1150ms 过窗再接正确凭据链路"
  - "S5 session_end 速退竞态双通道：waitExit 立即挂载 + child.exitCode 兜底——bash -c exit 42 可在 startWesh 50ms 落定窗内整体退出，exit 事件先于监听挂载时由 Node 侧 exitCode 属性补证"
  - "phase05-dom D5 踢出检测迁移为 parseEvents 字段断言（event==detach && reason==kick && code==1013）——08-02 D-21 折入后 'slow_consumer' stderr 子串形态终结；本脚本是 08-RESEARCH 迁移清单的漏检消费者（dom 变体不在 08-01..08-04 回归集），本 plan 全量回归捕获"

patterns-established:
  - "运维端点 UAT 三件套：healthzOkShape 键集白名单（恰四键多/少皆 FAIL）+ metricValue 精确行解析 + SERIES17 HELP 行逐字清单——后续运维端点变更的回归锁基座"
  - "回归集含 dom/width/dims 变体的价值实证：迁移类变更的漏检消费者只会在全量变体回归中暴露——phase05-dom D5 即本 plan 段 ④ 捕获的 08-02 残余"

requirements-completed: [OPS-06, OPS-07, OPS-08]

coverage:
  - id: D1
    description: "phase08.mjs 六场景 21 断言全绿：S1 healthz（免认证 200 恰四键/凭据模式无头 200+对照 401/bp 固定 200+404/POST 405+Allow:GET）、S2 metrics 认证闸两态、S3 exposition 17 series 与数值锁+bp 固定、S4 503 draining 确定性窗口+255 退出、S5 审计事件四族断言、S6 控制字符剥离三断言；SEC 输出自净通过"
    requirement: OPS-06
    verification:
      - kind: e2e
        ref: "node web/uat/phase08.mjs 退出 0（21/21，两连跑稳定；PHASE08_ONLY=S1 单独绿）"
        status: pass
      - kind: integration
        ref: "go test -race -count=1 ./... 五包全绿（server 59.4s）"
        status: pass
    human_judgment: false
  - id: D2
    description: "README「## 运维（Phase 8）」节三面文档化：/healthz 免认证唯一例外粗体明示（D-07 防例外蔓延义务）、Prometheus basic_auth 配方+429 自锁明示（RESEARCH Pitfall 6 义务）、事件目录表+journald/jq 检索两则+红线重申；认证节 Basic 闸补 /healthz 例外交叉引用"
    requirement: OPS-07
    verification:
      - kind: other
        ref: "grep 验收：'## 运维（Phase 8）'==1、三小节标题在场、'唯一例外'≥1、'basic_auth'≥1、'select(.event'≥1"
        status: pass
      - kind: e2e
        ref: "配方行为面由 phase08.mjs S2/S3 在真实二进制上锁定（认证闸两态/exposition 形态）"
        status: pass
    human_judgment: false
  - id: D3
    description: "08-UAT.md 三人工复核项落地（真实 Prometheus scrape/journald 实机检索/draining 窗口编排观测率——三条 flagged assumption 复核路由）+ 全量六段式与 14 UAT 脚本回归全绿（含 phase05-dom D5 迁移修复、GOROOT gofmt 清零）"
    requirement: OPS-08
    verification:
      - kind: e2e
        ref: "协议九（phase02..08 含 07-b2/07-b3）+ dom/width/dims 五共 14 脚本全退出 0；六段式（gofmt/vet/-race/web build 零漂移/裸 clone/冒烟）全绿"
        status: pass
      - kind: manual_procedural
        ref: ".planning/phases/08-observability/08-UAT.md A1/A2/A3（实机运维栈复核，自动化等价面已全绿）"
        status: unknown
    human_judgment: true
    rationale: "真实 Prometheus 实例、systemd/journald 实机 ingest、编排重启窗口观测率三项结构性依赖外部运维栈环境，自动化断言不可达——exposition 规范逐字断言与协议层全链断言已代替可自动化面，实机面按 08-UAT.md 清单人工复核"

duration: 49min
completed: 2026-08-28
status: complete
---

# Phase 8 Plan 05: 可观测性收口（UAT 六场景 + README 运维节 + 全量回归） Summary

**phase08.mjs 六场景 21 断言在真实二进制上端到端闭环（healthz 免认证例外与 bp 固定 / metrics 认证闸两态与 17 series 数值锁 / 关停 503 draining 确定性窗口 / 审计事件 JSON 检索与 client_id 关联 / NEL 控制字符剥离回归），README 运维节三面文档化（例外明示/Prometheus 配方与 429 自锁明示/jq 检索），08-UAT.md 三人工复核路由落地；全量回归捕获并修复 08-02 迁移漏检消费者 phase05-dom D5；六段式全绿——Phase 8 交付态，OPS-06/07/08 三需求收口**

## Performance

- **Duration:** 49 min
- **Started:** 2026-08-28T03:04:56Z
- **Completed:** 2026-08-28T03:53:47Z
- **Tasks:** 3/3
- **Files modified:** 6（2 新建 + 4 修改；plan files_modified 三件之外多 phase05-dom.mjs 修复与 multi/slowclient_test gofmt 清零——见 Deviations）

## Accomplishments

- **phase08.mjs 六场景端到端（21/21，两连跑稳定）**：S1 healthz 四组（无认证 200+恰四键白名单形状 / 凭据模式无头 200+对照 GET / 401 例外不蔓延 / bp=/wesh 下根路径 200 与 bp 内 404 / POST 405+Allow:GET）；S2 metrics 认证闸两态（正确凭据 200+Content-Type 含 text/plain 与 version=0.0.4、无凭据 401、--no-auth 直通）；S3 exposition（SERIES17 HELP 行逐字全在场 + 双客户端 /bin/cat 回显驱动 connected==2/total==2/pty_output>0/ws_sent≥pty_output/session_active==1 + build_info{version="dev"} 逐字行 + bp 固定）；S4 503 draining（trap 忽略 HUP + --stop-timeout 3s 拉宽窗口 → SIGTERM 立即轮询确定性观测 503+status=draining 恰四键 → 255 退出）；S5 审计事件（auth_failed 在场零 user/username 键 / throttled 携 retry_after≥1 / attach+detach 各恰 1 条 client_id 相等 reason=normal / exit 42 → session_end exit_code=42+duration_seconds>0+无 signal 键+进程 42 退出）；S6 控制字符剥离（NEL 线形探针 remote_user=alice + stderr 零 NEL / XFF 链首注入 remote 逐码点无 C0/C1/DEL / 对照组无 remote_user 键防空捕获假绿）
- **README 运维节三面闭环（D-07/Pitfall 6 明示义务兑现）**：「## 运维（Phase 8）」节——健康检查小节含 200 四字段 JSON 示例、503 draining 语义与**免认证唯一例外**粗体明示（双前提+不蔓延）；指标小节含 17 series 一览表、认证闸跟随与 --no-auth 暴露面明示、Prometheus scrape_config basic_auth 配方与**凭据错误触发全站节流 429 自锁**粗体明示（throttled 事件排查通道）；结构化日志小节含事件目录表（认证/连接/会话生命周期三面+exit_when_empty 族+协议守卫同 schema 注记）、字段口径、journald+jq 检索两则、凭据/token 永不入日志与控制字符剥离红线重申、启动行人读文本保持（D-14/D-15）
- **08-UAT.md 人工清单落地**：三条 flagged assumption 各配步骤/预期/自动化等价面引用（phase08.mjs 与 Go 侧行为锁），拓扑注记标注本 phase 无浏览器层人工项；canonical Tests 区三项 pending 待实机执行
- **全量回归捕获真实残余**：phase05-dom D5 的 stderr 子串 'slow_consumer' 消费在 08-02 D-21 折入后结构性失效（15s 超时）——迁移为 parseEvents 字段断言后 19/19 恢复；六段式全绿（GOROOT gofmt 清零两测试文件既有漂移 / vet 净 / -race 五包绿 / web build 后 index.html 零 diff / 裸 clone 归档编译测试五包绿 / 启动冒烟三断言）

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): phase08.mjs harness + S1 healthz scenario** - `f2a92a4` (test)
2. **Task 2: S2-S6 scenario matrix** - `792d59b` (test)
3. **Task 3 段(a): GOROOT gofmt 清零 multi_test/slowclient_test** - `15b863f` (style)
4. **Task 3 段④: phase05-dom D5 踢出检测迁移修复（回归捕获）** - `dd91032` (test)
5. **Task 3: README 运维节 + 08-UAT.md + 全量六段式与全脚本回归** - `132ad98` (docs)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP/REQUIREMENTS）

_Tracer feedback gate（autonomous，08-01..08-04 同款）：Task 1 提交后 verify 端到端重跑通过（build + PHASE08_ONLY=S1 5/5），方进入 Task 2 扩展面。_

## Files Created/Modified

- `web/uat/phase08.mjs`（新建，619 行）— harness 逐字复用 phase07.mjs 骨架七件 + 内联 parseEvents + waitEvent 事件流轮询 + healthzOkShape 键集白名单 + SERIES17/metricValue exposition 断言件 + 六场景函数 + PHASE08_ONLY 过滤 + assertOutputClean 自净
- `README.md` — 新增「## 运维（Phase 8）」节（三小节 + 配方代码块 + 粗体明示两处）；认证节 Basic 闸 bullet 补 /healthz 例外交叉引用
- `.planning/phases/08-observability/08-UAT.md`（新建）— 三人工复核项（Prometheus scrape/journald 检索/draining 编排观测率）+ canonical Tests 区（pending×3）+ 拓扑注记
- `web/uat/phase05-dom.mjs` — D5 踢出检测迁移（子串 'slow_consumer' → parseEvents detach reason=kick code=1013）+ 内联 parseEvents 补挂（08-01 自含形态）
- `internal/server/multi_test.go`、`internal/server/slowclient_test.go` — GOROOT gofmt -w 纯排版清零（49+49 行注释/缩进重排，逐行核读零语义）

## Decisions Made

- **S4 draining 夹具 = trap 忽略 HUP + --stop-timeout 3s 组合**（详见 Deviations #1）
- **S2/S5 凭据场景排序即解零 pacing**（phase07 S1/phase03 先例沿用）：成功链路先行、401 负面对照排最后；S5 的 429 打满取 phase03 场景 2 确定性形态（连发撞窗零 sleep），过窗 1150ms 仅一次
- **S5e 速退竞态双通道**（waitExit 立即挂载 + child.exitCode 兜底）：bash -c exit 42 可在 startWesh 落定窗内整体退出——竞态真实存在，双通道使断言确定性成立
- **phase05-dom D5 迁移形态与防空捕获纪律**：kick 检测字段化（detach/reason/code 三元组精确断言强于原子串）；本修复属 08-02 迁移收尾（详见 Deviations #2）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] S4 plan 字面 '--stop-timeout 3' 按意图修正为 trap 夹具 + '3s'**
- **Found during:** Task 2（S4 夹具设计——plan action ③ 与 key_links/RESEARCH OQ3 交叉核读）
- **Issue:** plan 字面 spawn 加 `'--stop-timeout', '3'` 有两层不可达：① --stop-timeout 是 DurationVar（main.go:437），`time.ParseDuration("3")` 缺单位必解析失败 exit 2（phase07 S5 用 '1s'、08-03 冒烟用 '3s' 同款先例）；② 即使单位修正，进程终结由 lifecycle 子进程死亡路径收口（main.go:1191 P1 硬约束）——子进程不 trap 时 bash 速死窗口 <1s（08-03 flagged_assumptions 已实证 0.3s 后进程已退出），轮询 503 有真实竞态；RESEARCH OQ3 定案的「延迟 KILL 补发」机制前提即子进程挺过 stop-signal
- **Fix:** 夹具取 08-03 冒烟同款组合：`--stop-timeout 3s` + 子进程 `sh -c 'trap "" HUP; sleep 100'`（trap 忽略型经 SIG_IGN 跨 exec 持久整组免疫，07-04 实证）——Shutdown 同步 sleep 3s（07-05 形态）内进程存活且 draining 已置位，50ms 轮询 2.5s 护栏确定性观测
- **Files modified:** web/uat/phase08.mjs（S4 场景内注释登记机制论证）
- **Verification:** S4a/S4b 两连跑稳定 PASS（503+draining 恰四键 + 255 退出）
- **Committed in:** 792d59b（Task 2 提交）

**2. [Rule 1 - Bug] phase05-dom.mjs D5 踢出检测子串消费在 08-02 D-21 折入后失效**
- **Found during:** Task 3 段④（全脚本回归——phase05-dom「16/16 DOM 断言通过，1 个场景异常：15s 内未见 slow_consumer 踢出」）
- **Issue:** D5 stall 夹具以 `stderrText().includes('slow_consumer')` 检测踢出——08-02 把 slow_consumer 独立事件行折入 detach reason=kick（wire Close(1013,"slow_consumer") 逐字不动但 stderr 文本形态终结），子串恒 false 致夹具 15s 超时；08-RESEARCH Runtime State Inventory 迁移清单漏检本消费者（dom/width/dims 变体不在 08-01..08-04 的回归集内，四连 plan 均未暴露）
- **Fix:** 内联 parseEvents 补挂本脚本（08-01 自含形态）+ 检测迁移为 `event=='detach' && reason=='kick' && code==1013` 字段断言（禁止子串断言 JSON 行纪律保持）；超时错误文案同步更新
- **Files modified:** web/uat/phase05-dom.mjs
- **Verification:** phase05-dom.mjs 退出 0（19/19——D5a/D5b/D5c 恢复执行）；全量 14 脚本复跑全绿
- **Committed in:** dd91032（独立 test 提交）

### 授权内清零（非代码偏差）

**3. [plan 明示授权] GOROOT gofmt 清零 multi_test.go/slowclient_test.go 既有漂移**
- plan 段(a) 字面「纯排版差异 -w 修正后独立 style 提交先例」+ must_have「GOROOT gofmt -l 清零」——两文件漂移为 HEAD 预存（07-deployment/deferred-items.md 登记同族，08-01/08-02 按 SCOPE BOUNDARY 不动留待清零路由）；diff 经 -d 预览逐行核读为纯注释/缩进重排（CJK 注释规则的 gofmt 版本差异，/usr/bin/gofmt 陈旧——01-03 登记），独立 style 提交 15b863f（02-06/03-06/05-09/06-07 先例第五次沿用）

---

**Total deviations:** 2 auto-fixed（1 blocking 夹具意图修正 + 1 回归测试迁移）+ 1 授权内排版清零
**Impact on plan:** 全部锁定语义与 plan must_haves 逐字一致；S4 夹具修正确保 draining 断言确定性（plan 意图的机械可达形态）；D5 修复是 plan 自身「既有 UAT 全量回归绿」真理的直接兑现；零 scope creep；无 Rule 4 架构变更、无认证门、无包安装（零新依赖纪律保持）

## Issues Encountered

- **Edit 工具对 `\uXXXX` 字面量的转义歧义**（08-02 登记同族再现）：S6 的 NEL_WIRE 初版经 Edit 落盘为解码后字符（0xC3 0x85 = Å，上线即成单字节 0xC5 非探针形态），经 perl `-pe` 精确落为显式转义形态（`'ali\u00C2\u0085ce'` 与 `includes('\u0085')`，hexdump 字节级核验）——与 phase07.mjs S4c 先例逐字同形态
- **全量 -race 与 UAT 回归的时序编排**：08-04 登记过 CPU 竞争 flake 先例，本 plan 将 -race 与 pnpm build/UAT 串行化执行（后台等待而非并发），一轮全绿零 flake

## Authentication Gates

None——纯本地执行，无认证门。

## Known Stubs

无——phase08.mjs 全部断言消费真实二进制的真实数据面（端点响应/事件流/计数器数值）；08-UAT.md 三人工项为复核路由而非占位（自动化等价面均已绿并逐项引用）；无 TODO/FIXME/空数据流。

## Next Phase Readiness

- **Phase 8 交付态**：ROADMAP 三条 Success Criteria 全成立——/healthz 探活可用（S1/S4 + TestHealthz 族）、/metrics 五类指标暴露（S2/S3 + TestMetrics 族）、JSON 审计日志可检索且凭据零入日志 + 控制字符剥离（S5/S6 + events/log 测试族）；OPS-06/07/08 达成交付态
- **Phase 9 就绪面**：OPS-03（自定义首页）与 OPS-10（单二进制发布）为剩余 Active 项；build_info 的 `var version` → Options.Version → escLabel plumbing 已贯通（08-04），发布构建 ldflags 注入单点即可
- **人工复核挂账**：08-UAT.md A1/A2/A3（真实 Prometheus/journald/draining 编排观测）——自动化等价面全绿，实机面随 phase uat 流程执行
- **无阻塞项**

## Self-Check: PASSED

- 文件：web/uat/phase08.mjs ✓（新建）、README.md ✓（修改）、.planning/phases/08-observability/08-UAT.md ✓（新建）、web/uat/phase05-dom.mjs ✓（修复）、internal/server/multi_test.go+slowclient_test.go ✓（gofmt）、08-05-SUMMARY.md ✓
- 提交：f2a92a4 ✓、792d59b ✓、15b863f ✓、dd91032 ✓、132ad98 ✓
- 关键指纹：phase08.mjs `PHASE08_ONLY` ×3（≥1）✓ / `assertOutputClean` ×8（≥2）✓ / `parseEvents` ×6（≥1）✓；README `## 运维（Phase 8）` ×1 ✓ / `唯一例外` ×2 ✓ / `basic_auth` ×2 ✓ / `select(.event` ×1 ✓ / 三小节标题 ×3 ✓；08-UAT.md 三复核项（A1 Prometheus/A2 journald/A3 draining）✓
- 验收命令全过：Task 1（PHASE08_ONLY=S1 退出 0）✓；Task 2（全场景 21/21 两连跑）✓；Task 3（gofmt 清零 / vet 净 / -race 五包绿 / web build+index.html 零 diff / 裸 clone 五包绿 / 冒烟三断言 / 14 UAT 脚本全退出 0）✓
