---
phase: 07-deployment
plan: 01
subsystem: infra
tags: [base-path, reverse-proxy, http-mux, stripprefix, relative-url, deployment, nginx, tdd, tracer]

requires:
  - phase: 05-multi-client/05-06
    provides: share token 第三认证通道 + registerShareRoutes 现状形态 + GOROOT matchOrRedirect 恒 307 实证登记
  - phase: 03-auth/03-03
    provides: /api/attach path-only 405 fallback 先例（GOROOT server.go:2699-2710 n==nil 分支论证）
  - phase: 03-auth/03-04
    provides: TestParseArgs 命名字段扩展纪律 + parse 期校验插入点（showVersion 早退之后）先例
  - phase: 06-session-lifecycle
    provides: jsdom/协议层九套件 UAT harness（零回归断言面）+ startTestServerWith/dialHello 测试基建
provides:
  - --base-path CLI 公开契约（D-13：normalizeBasePath parse 期严格校验——合法原样/根 / 归一未配置/五族非法 exit 2，绝不宽容自动修正）
  - server.Options.BasePath + Handler() bp 前缀装配（StripPrefix 仅包静态伺服链；裸 {bp} 307 由 mux matchOrRedirect 免费获得）
  - registerShareRoutes(mux, bp, page, root) bp 前缀 share 两注册（含 405 fallback 单侧定义防线）
  - internal/server/basepath_test.go 全链断言（307/404/405/share 交叉/ticket/WS/零值零漂移）
  - 前端相对 URL 三改（share 正则不锚 ^ 兼作挂载点检测 + up='../../' 升级前缀 + fetch/WS 相对构造）+ dist 重建入库
  - main.go shareURLRO/shareURLRW 拼串单一事实源局部变量（07-05 --open 消费点）
affects: [phase-07 后续 plans（07-05 --open / 07-06 配置文件 base-path 键 / 07-08 README+phase07.mjs）, verify-work]

actuals:
  tokens: 24746
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "base-path mux 前缀装配：StripPrefix 仅包静态伺服链（唯一路径敏感 handler），路径无关 handler 注册模式串直接带 bp 前缀；裸 {bp} 与裸 {bp}/s/{token} 307 均由 matchOrRedirect 免费获得（注册子树即触发）"
    - "前端挂载点检测 + 升级前缀：share 正则不锚 ^ 兼作检测，up='../../' 回站根；fetch 相对构造 + new URL(up+'ws', location.href) 显式换 ws/wss protocol（Anti-Pattern 3 必做步）"

key-files:
  created:
    - internal/server/basepath_test.go
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go
    - internal/server/server.go
    - internal/server/sharetoken.go
    - web/src/main.ts
    - web/dist/index.html

key-decisions:
  - "无认证分支 bp 形态补注册 attach path-only 405 fallback（Rule 1 行为规约调和）：plan behavior 矩阵对无认证+BasePath 实例断言 405，现状无认证根挂载经 embed FS 404——取「bp 形态注册、根挂载不注册保持零漂移」，RESEARCH Pitfall 4 单侧定义防线落到两分支，差异以注释锚定"
  - "非法 --base-path 五族断言落 TestTLSKeyPairError 错误表（parse 期拒绝既定归属），TestParseArgs 只收合法/root 三行——既存行零改动纪律下错误族无法入其表结构（wantBasePath 命名字段扩展，03-04 先例）"
  - "dist 升级前缀验收 grep 适配为引号无关心形态：esbuild 以反引号模板字面量发射 '../../'（与现状 `/api/attach` 同形态）——字面量未被重命名，引号非断言面（05-09 先例断言面守恒）"

patterns-established:
  - "base-path 特性双面零漂移纪律：服务端 bp==\"\" 注册形态逐字节一致（TestBasePathEmptyUnchanged 锁定）+ 前端根挂载/分享挂载相对解析矩阵四行断言（RESEARCH Pattern 3）"
  - "CLI 部署契约值校验形态：normalizeBasePath 纯函数（isLoopbackBind 同位、NormalizeOrigin 先例）——空串/根归一空串，五族拒绝文案含原输入（值非敏感可回显，exitEmptyValue.Set 同纪律）"

requirements-completed: [OPS-02]

coverage:
  - id: D1
    description: "--base-path CLI 契约：合法值（/wesh、/a/b）原样接收；根 / 归一为未配置；非法五族（无前导斜杠/尾斜杠/重复斜杠/../非 path 安全字符）parse 期拒绝 exit 2"
    requirement: OPS-02
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs（base-path 三行）+ #TestTLSKeyPairError（非法九行）"
        status: pass
    human_judgment: false
  - id: D2
    description: "服务端 bp 前缀装配全链：GET /wesh 307 且 Location=/wesh/（RawQuery 保留）、GET /wesh/ 200 终端页、bp 外 / 与 /api/attach 404、非 POST /wesh/api/attach 405+Allow:POST、/wesh/s/{token}/ 给页、裸 /wesh/s/{token} 307、POST /wesh/api/attach 携 token 签 ticket、WS /wesh/ws Hello/Welcome 握手；零值实例 / 与 /ws 存活零漂移"
    requirement: OPS-02
    verification:
      - kind: integration
        ref: "internal/server/basepath_test.go#TestBasePathRoutes + #TestBasePathWS + #TestBasePathEmptyUnchanged"
        status: pass
    human_judgment: false
  - id: D3
    description: "前端相对 URL 三改 + dist 入库：share 正则不锚 ^、up 升级前缀、fetch/WS 相对构造（显式换 protocol）；四类挂载点解析矩阵成立；既有九套件零回归"
    requirement: OPS-02
    verification:
      - kind: e2e
        ref: "node web/uat/phase04-dom.mjs（37/37）+ phase05-dom.mjs（19/19，含 /s/{token}/ 挂载点）+ phase06-dom.mjs（33/33+1豁免）"
        status: pass
      - kind: e2e
        ref: "node web/uat/phase02/03/04/05/05-dims/06.mjs 协议层六套件（12/12、18/18、10/10、28/28+1豁免、DIMS PASS、23/23+1豁免）"
        status: pass
    human_judgment: false
  - id: D4
    description: "启动打印分享链接两行含 base-path 前缀（scheme://host:port<bp>/s/<token>/）；非法值进程级 exit 2；根 / 归一后正常进入启动矩阵；未配置时打印逐字节不变"
    requirement: OPS-02
    verification:
      - kind: other
        ref: "真实二进制冒烟（/tmp/wesh-bp-smoke）：curl /wesh→307、/wesh/→200、/→404、stdout 两行含 /wesh/s/ 前缀、--base-path wesh 与 /wesh/ 均 exit=2、--base-path / 进入 validateStartup"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 01: base-path 反代子路径挂载 Summary

**--base-path 反代子路径挂载全链落地：CLI 严格校验（D-13）+ mux 前缀装配（StripPrefix 仅静态伺服、裸 {bp} 307 免费）+ 前端相对 URL 三改含 share 页 '../../' 升级前缀（D-14），未配置时前后端行为零漂移。**

## Performance

- **Duration:** 35 min
- **Started:** 2026-08-26T00:07:36Z
- **Completed:** 2026-08-26T00:42:06Z
- **Tasks:** 2/2
- **Files modified:** 7（1 新建 + 6 修改）

## Accomplishments

- `--base-path /wesh` 下页面/WS/share 三链路全通：GET /wesh → 307（Location=/wesh/、RawQuery 保留）、GET /wesh/ → 200 终端页、WS /wesh/ws 完成 Hello/Welcome 握手；/wesh/s/{token}/ 给页、裸路径 307 补斜杠、POST /wesh/api/attach 携 token 签发 ticket（tracer 全链打穿）
- D-13 严格校验落地：normalizeBasePath 纯函数五族拒绝（无前导斜杠/尾斜杠/重复斜杠/../非 path 安全字符）parse 期 exit 2，根 / 归一为未配置，绝不宽容自动修正
- 前端相对 URL 三改 + dist 重建入库，相对解析矩阵四行成立（/ 、/wesh/ 、/s/{t}/ 、/wesh/s/{t}/ 均解析到正确路由）
- 零漂移双证：bp=="" 时 Handler() 注册形态逐字节一致（TestBasePathEmptyUnchanged + 全量 -race 套件 5 包全绿）；九 UAT 套件（3 jsdom + 6 协议层）对新 dist + 新二进制全绿
- 分享链接打印两行含 base-path 前缀（冒烟实证）；shareURLRO/shareURLRW 拼串单一事实源抽出供 07-05 --open 复用

## Task Commits

每个任务原子提交（Task 1 为 tracer + TDD，含 RED/GREEN 两提交）：

1. **Task 1 (tracer, TDD) RED: --base-path 失败测试** - `245b245` (test)
2. **Task 1 (tracer, TDD) GREEN: --base-path flag + 服务端装配** - `23d72c8` (feat)
3. **Task 2: 前端相对 URL 三改（src）** - `82af1b8` (feat)
4. **Task 2: dist 重建（产物）** - `4f1fc8e` (chore)

**Plan metadata:** docs 提交在本 SUMMARY 之后（`docs(07-01): complete base-path plan`，hash 见 git log）。

_Tracer 反馈门：plan 为 autonomous 且无 checkpoint 任务（Pattern A），按自治形态在 Task 2 前重跑 plan verify 全链（TestBasePath + TestParseArgs/TestStartupMatrix/TestStartupRefusalNoResource + go build）通过后扩展（TRACER_GATE_PASS）。_

## Files Created/Modified

- `internal/server/basepath_test.go`【新】- TestBasePathRoutes（307/404/405/share 交叉/ticket 表驱动）+ TestBasePathWS（/wesh/ws 握手全链）+ TestBasePathEmptyUnchanged（零值零漂移）
- `cmd/wesh/main.go` - config.basePath 字段（Phase 7 分组注释）+ --base-path flag 注册 + normalizeBasePath 纯函数（isLoopbackBind 同位）+ Parse 返回处校验接线 + server.New Options.BasePath 接线 + shareURLRO/shareURLRW 拼串单一事实源
- `cmd/wesh/main_test.go` - TestParseArgs wantBasePath 命名字段扩展 + 三新行（合法两族/根归一）；TestTLSKeyPairError 九新行（非法五族细分）
- `internal/server/server.go` - Options.BasePath + Server.basePath + New 装配直传 + Handler() bp 前缀装配（StripPrefix 仅包静态伺服；两分支 405 fallback 单侧定义防线；bp=="" 逐字节不变）
- `internal/server/sharetoken.go` - registerShareRoutes 加 bp 首参（"GET "+bp+"/s/{token}/" + 405 fallback）；GOROOT 通配语义三坑注释补 bp 形态 307 补斜杠登记
- `web/src/main.ts` - share 正则不锚 ^ + up 升级前缀常量 + fetch(up+'api/attach') + new URL(up+'ws', location.href) 显式换 protocol
- `web/dist/index.html` - 产物重建（time pnpm -C web build；.gz 不入库既定策略）

## Decisions Made

- **无认证分支 bp 形态补注册 attach 405 fallback**（详见 Deviations #1）——plan behavior 矩阵与现状无认证形态的调和取「bp 形态注册、根挂载零漂移」，两形态差异以注释锚定。
- **非法值断言归属 TestTLSKeyPairError 错误表**——TestParseArgs 表结构（既存行零改动纪律）不承载错误族；plan 字面「TestParseArgs 新行：合法/非法/根斜杠三族」按三族齐全、归属分置解读（合法/根斜杠在 TestParseArgs，非法在错误表）。
- **dist 验收 grep 引号无关心适配**（详见 Deviations #2）——字面量在场证据不改，引号形态不属断言面。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug/行为规约调和] 无认证分支 bp 形态补注册 attach path-only 405 fallback**
- **Found during:** Task 1 GREEN（TestBasePathRoutes 首跑：GET /wesh/api/attach = 404，behavior 矩阵要求 405）
- **Issue:** plan behavior 对「Options{BasePath:/wesh, ShareTokenRO/RW}」（无认证）实例断言「POST 之外方法打 /wesh/api/attach → 405 且 Allow: POST」；现状无认证分支从未注册 attach 405 fallback（GET /api/attach 经 "/" 子树漏进 embed FS 404），凭据分支才有——action ②『path-only 405 fallback 两条均带前缀注册』未限定分支
- **Fix:** 无认证分支在 bp!="" 时补注册同文同码 405 fallback（Allow: POST）；bp=="" 不注册——根挂载无认证现状 404（embed FS）逐字节保持（零漂移红线），两形态差异以注释锚定（Pitfall 4 单侧定义防线落到两分支）
- **Files modified:** internal/server/server.go
- **Verification:** TestBasePathRoutes 405 行转绿；bp=="" 零漂移由 TestBasePathEmptyUnchanged + 全量 -race 套件（50.4s 全绿）双证
- **Committed in:** 23d72c8（Task 1 GREEN 提交内）

**2. [Rule 3 - 验收闸适配] dist 升级前缀 grep 改引号无关心形态**
- **Found during:** Task 2 dist 验收（`grep -c "'\.\./\.\./'" web/dist/index.html` 返 0）
- **Issue:** esbuild 以反引号模板字面量发射 '../../'（现状产物 `/api/attach` 同形态——vite/esbuild 对该体积字符串的既定发射选择）；验收 grep 的单引号字面形态因此结构性为 0
- **Fix:** 断言改 bare grep `'\.\./\.\./'`（==1 实测）+ 上下文片段人工复核 `` r=t?`../../`:`` `` 在场；plan 该验收的本意（升级前缀字面进产物不被重命名，05-08 结构指纹先例）完整保持——字面量未被重命名，引号非断言面（05-09 先例断言面守恒）
- **Files modified:** 无（验证方式适配，不改源码/产物）
- **Verification:** bare grep ==1 + 产物片段复核
- **Committed in:** 4f1fc8e（dist 提交信息内登记）

---

**Total deviations:** 2 auto-fixed（1 行为规约调和 Rule 1，1 验收闸适配 Rule 3）
**Impact on plan:** 两修正均为 plan 自身 behavior/验收条款的忠实落地所必需，零范围蔓延；prohibition（MUST NOT 宽容修正非法值）严格保持。

## Issues Encountered

- **main.go flag 注册编辑首回未落盘：** Edit 工具首回报告成功但 fs.StringVar(&cfg.basePath...) 未出现在文件（同批其余五处编辑均在）；TestParseArgs 以「flag provided but not defined: -base-path」当场捕获，重新应用同内容编辑后落盘（L211），全套件转绿。工具层偶发，无代码语义影响。
- **Out-of-scope 发现（已登记 deferred-items.md，未修）：** internal/server/multi_test.go 与 internal/server/slowclient_test.go 存在 HEAD 既有 GOROOT gofmt 漂移（git show HEAD 版本复验确认非本 plan 引入；CJK 注释空格规则差异家族，01-03/05-09 登记同族，/usr/bin/gofmt 陈旧为诱因）——按 SCOPE BOUNDARY 纪律不随本 plan 修复，登记至 .planning/phases/07-deployment/deferred-items.md。

## User Setup Required

None - no external service configuration required.

## Threat Flags

None——全部新表面（bp 前缀路由族、405 fallback、相对 URL 解析）均在 plan `<threat_model>` T-07-01a/b/c/d 四条登记内，无未建模的信任边界扩张。

## Known Stubs

None——无占位实现；全部 must_have truths 经 Go 测试/冒烟/九套件断言达成。

## Next Phase Readiness

- 07-02（监听形态 --socket 三 flag）与 07-06（配置文件 base-path 键——fileConfig.BasePath 经同一 normalizeBasePath 校验，RESEARCH Pattern 4 结构体已预留）可直接开工，无阻塞
- 07-05 --open 消费 shareURLRO/shareURLRW 单一事实源已就位（main.go 注释标注）
- 07-08 可在 phase07.mjs 增 base-path 协议场景（本 plan Go 层 + 冒烟已全链实证；Playwright 观感层列可选非阻塞，双机拓扑既定）

## Self-Check: PASSED

- 文件存在性：8/8 FOUND（含本 SUMMARY 与全部 must_have artifacts）
- 提交存在性：4/4 FOUND（245b245 / 23d72c8 / 82af1b8 / 4f1fc8e）
- must_have 内容断言：normalizeBasePath ∈ cmd/wesh/main.go（×5）；'../../' ∈ web/src/main.ts（×1）与 web/dist/index.html（×1）；basepath_test.go 229 行 ≥ 80；BasePath ∈ internal/server/server.go（×5 ≥ 3）；http.StripPrefix ∈ internal/server/server.go（×2 ≥ 1）

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
