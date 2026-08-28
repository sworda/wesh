---
phase: 08-observability
plan: 03
subsystem: infra
tags: [healthz, ops-endpoint, graceful-shutdown, go-stdlib, observability, atomic-state]

requires:
  - phase: 08-observability
    plan: 02
    provides: s.exiting/closeBroadcastCode 同源置位先例（Shutdown 入口挂点形态）+ shutdown 事件 emit（同函数不同点约定已在注释登记）+ startShutdownServerWith 夹具
provides:
  - GET /healthz 免认证探活端点（D-07/D-10）：200 + `{"status":"ok","clients":N,"max_clients":M,"session_active":bool}`——凭据模式无 Authorization 头同样 200（整站 Basic 闸唯一窄例外），k8s liveness/反代健康检查结构性可用
  - 优雅关停 503 draining（D-11）：Shutdown() 首行 s.draining.Store(true)（与 s.exiting 同源触发点）——1001 广播开始前即翻转，关停全程探活器不再向将死实例导新流
  - 根路径固定（D-09）：bp=/wesh 实例下 /healthz 仍 200、/wesh/healthz 不可达（无认证 404 / 凭据 401）；拒绝双挂
  - 405 成对注册：`"GET /healthz"` 方法模式 + `"/healthz"` path-only fallback（Allow: GET，sharetoken.go:122-128 先例——内建 405 会被 / 子树吞掉）
  - sessionAlive/draining 两 atomic.Bool（hubMu 外读取选型，registry.n 先例同构）——sessionAlive 翻转点 = lifecycle sess.Wait 返回与退出码提取完成后（session_end 同区段）
  - 键集白名单行为锁完整形态（T-08-03a）：四键恰好（多/少一键皆 FAIL），200/503 两态同锁
affects: [08-04 metrics（draining/sessionAlive 数据源复用：wesh_session_active gauge 读取端已就位）、08-05 UAT/README 收口（phase08.mjs S4 draining 场景的 Go 侧行为锁即本 plan）]

actuals:
  tokens: 5006
  tasks: 2
  commits: 5

tech-stack:
  added: []
  patterns:
    - "hubMu 外 atomic 状态位选型：/healthz handler 不取 hubMu——draining/sessionAlive 为 atomic.Bool（与 registry.n 的『hubMu 外 atomic load』先例同构），maxClients 为 New 装配期固化只读，R-07 纪律下零新锁"
    - "探活例外注册结构：免认证端点注册在 Handler() 认证/无认证两分支之外唯一一处 + 注释登记『防例外蔓延』（T-08-03b 缓解的结构性半侧，行为半侧 = 对照 401 子测）"
    - "键集白名单断言完整形态：map 解码 → 四键逐一命中并 delete → 残余非空即 FAIL（多键）+ 缺键即 FAIL（少键）——prohibition『body 键集白名单断言』字面兑现，200/503 两态同锁"

key-files:
  created:
    - internal/server/health.go
    - internal/server/health_test.go
  modified:
    - internal/server/server.go

key-decisions:
  - "/healthz 键集白名单锁取完整形态——四键恰好（多/少一键皆 FAIL），200 与 503 两态同锁（T-08-03a prohibition 的『body 键集白名单断言』字面兑现）；draining body 同构四字段（RESEARCH A5）"
  - "draining 置位落 Shutdown 首行（emitEvent shutdown 之前，hubMu 锁定之前）——plan『首行（hubMu 锁定之前）』字面；两原子位注释登记 T-08-03d（无网络可达置位路径）"
  - "sessionAlive 置 false 挂点 = lifecycle 的 session_end emit 之后、Drain 之前（plan『同区段』的精确落点）——waitExit 收码即测试侧同步边（exitf 在 lifecycle 末尾，程序序保证）"

patterns-established:
  - "运维端点测试三件套：httpBaseOf（wsURL→http base 推导）+ getHealthz（两态状态码 + Content-Type + 键集白名单 + 类型化解码）+ assertHealthz（四字段逐值锁）——08-04 metrics 测试可复用同构形态"
  - "原子位生命周期注释纪律：字段声明处登记置位点/读取点/选型论证（draining=Shutdown 入口、sessionAlive=New 置 true/lifecycle 置 false、hubMu 外读故 atomic）——后续 plan 挂点零考古"

requirements-completed: [OPS-06]

coverage:
  - id: D1
    description: "/healthz 200 四字段 + 免认证窄例外 + bp 固定 + 405：TestHealthz 五子测（ok 四字段 clients 0→1 / 凭据无头 200 + 对照 GET / 401 / bp=/wesh 两模式 /healthz 200 与 /wesh/healthz 404·401 / POST 405+Allow:GET / session_active 翻转）"
    requirement: OPS-06
    verification:
      - kind: unit
        ref: "internal/server/health_test.go#TestHealthz（五子测 -race 全绿）"
        status: pass
      - kind: integration
        ref: "真实二进制冒烟：curl /healthz → 200 `{\"status\":\"ok\",\"clients\":0,\"max_clients\":32,\"session_active\":true}`；POST → 405 Allow:GET"
        status: pass
    human_judgment: false
  - id: D2
    description: "关停 503 draining（D-11）：Shutdown 置位点唯一（grep==1）+ 200/ok → Shutdown → 503/draining 翻转序列 + 子进程死亡后复访仍 503（不翻回）+ Shutdown 行为零回归"
    requirement: OPS-06
    verification:
      - kind: unit
        ref: "internal/server/health_test.go#TestHealthzDraining + TestShutdown1001/TestShutdownStopTimeout/TestShutdownEvent（-race 全绿）"
        status: pass
      - kind: integration
        ref: "真实二进制冒烟（trap \"\" HUP + --stop-timeout 3s 拉宽窗口）：SIGTERM 后 0.5s/1.5s 两轮 curl 均 503 `{\"status\":\"draining\",...}`，进程最终 255 退出"
        status: pass
    human_judgment: false
  - id: D3
    description: "零回归：全仓 -race 五包绿 + 既有 UAT 八脚本 132 断言全过"
    requirement: OPS-06
    verification:
      - kind: unit
        ref: "go test -race -count=1 ./...（cmd/wesh、proto、pty、server、web 五包全绿，server 54.9s）"
        status: pass
      - kind: e2e
        ref: "node web/uat/{phase02,phase03,phase04,phase05,phase06,phase07,phase07-b2,phase07-b3}.mjs 全退出 0（12/12、18/18、10/10、28/28、23/23、34/34、4/4、3/3）"
        status: pass
    human_judgment: false

duration: 28min
completed: 2026-08-28
status: complete
---

# Phase 8 Plan 03: /healthz 探活端点 Summary

**OPS-06 全行为落地：health.go 新文件装 healthzHandler（draining 分支 + encoding/json 四字段，零新依赖），Handler() 根路径双注册（GET 方法模式 + 405 fallback 成对、免认证两模式、bp 前缀之外），sessionAlive/draining 两 atomic.Bool 挂进 New/lifecycle/Shutdown——凭据模式无头 200、bp=/wesh 下 /healthz 仍 200 而 /wesh/healthz 不可达、POST 405+Allow:GET、SIGTERM 后 503 draining 全行为锁；全仓 -race 绿 + 八 UAT 脚本 132 断言零回归**

## Performance

- **Duration:** 28 min
- **Started:** 2026-08-28T01:20:31Z
- **Completed:** 2026-08-28T01:48:39Z
- **Tasks:** 2/2
- **Files modified:** 3（2 新建 + 1 修改，与 plan files_modified 清单逐一对应）

## Accomplishments

- **D-10 状态 JSON 四字段端到端**：无认证实例 GET /healthz → 200 application/json，`status/clients/max_clients/session_active` 逐字键名——clients 数据源 `registry.n.Load()`（dialHello 前后 0→1 行为锁）、max_clients 默认 32、session_active 经新 sessionAlive atomic.Bool（New 置 true，lifecycle sess.Wait 返回与退出码提取完成后置 false——子进程 exit 42 后 200 + session_active==false 翻转锁定）；body 经 `json.Marshal` 匿名 struct（Don't Hand-Roll 纪律，零手写拼接）
- **D-07 免认证窄例外双前提行为锁**：凭据实例无 Authorization 头 GET /healthz → 200 同形态；对照 GET / 同请求仍 401（例外不蔓延的结构半侧 = 注册点在认证两分支之外唯一一处 + health.go 零 basicAuth 引用，行为半侧 = 本对照子测）
- **D-09 根路径固定**：bp=/wesh 两模式 GET /healthz → 200；GET /wesh/healthz → 无认证 404（embed FS 无此路径）/ 凭据 401（bp 子树经 Basic 闸）——拒绝双挂，探活路径可写死进 k8s probe/Prometheus 静态配置
- **405 成对注册**：POST /healthz → 405 + Allow: GET（sharetoken.go:122-128 方法模式 + path-only fallback 先例——内建 405 会被 "/" 子树吞进静态伺服，RESEARCH Pitfall 7 防线）
- **D-11 draining 503**：Shutdown() 首行 `s.draining.Store(true)`（hubMu 锁定之前，与 s.exiting 同源触发点；grep==1 唯一置位点）——200/ok → Shutdown → 503/draining 翻转序列锁定，子进程死亡后复访仍 503（draining 不翻回）；真实二进制冒烟（trap 忽略 HUP + --stop-timeout 3s 拉宽窗口）：SIGTERM 后 0.5s/1.5s 两轮 503 实证，进程 255 退出（accept-255 同源）
- **T-08-03a prohibition 完整形态**：getHealthz 键集白名单锁——四键恰好（多一键混入版本/身份/错误细节或少一键皆 FAIL），200/503 两态同锁

## Task Commits

Each task was committed atomically（TDD RED→GREEN 每任务两提交）:

1. **Task 1 RED: /healthz 失败测试先行** - `397cfd9` (test)
2. **Task 1 GREEN (tracer): healthzHandler + 双注册 + sessionAlive 接线** - `81f0b2b` (feat)
3. **Task 2 RED: TestHealthzDraining 失败测试先行** - `f32a663` (test)
4. **Task 2 GREEN: Shutdown 入口 draining 置位** - `97b9b66` (feat)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP）

_Tracer feedback gate（autonomous，08-01/08-02 同款）：Task 1 提交后 verify 三腿端到端重跑通过（TestHealthz -race PASS / go vet OK / 全包 50.1s 绿），方进入 Task 2 扩展面。_

## Files Created/Modified

- `internal/server/health.go`（新建，54 行）— D-07/D-09/D-10/D-11 注释头 + 枚举 oracle 红线登记（body 恒四字段粗粒度容量面，version 只在需认证的 /metrics build_info）；healthzHandler：draining.Load() 分支 → 503/"draining" 否则 200/"ok"；json.Marshal 匿名 struct 编码；hubMu 外全 atomic/只读取数（R-07 纪律零新锁）
- `internal/server/health_test.go`（新建，281 行）— TestHealthz 五子测 + TestHealthzDraining；httpBaseOf/getHealthz（键集白名单完整形态）/getStatus/assertHealthz 断言件；夹具复用 startTestServerWith/startShutdownServerWith/dialHello/waitExit/assertNoExit 零新装配
- `internal/server/server.go` — Server struct 加 draining/sessionAlive 两 atomic.Bool（注释登记置位点/读取点/registry.n 同构选型论证）；New 尾部 sessionAlive.Store(true)（goroutine 启动前）；lifecycle session_end emit 后 Store(false)；Handler() 认证两分支之外注册 `"GET /healthz"` + `"/healthz"` 405 fallback（注释登记 D-07 防例外蔓延 + D-09 拒绝双挂）；Shutdown() 首行 draining.Store(true)（08-02 预留注释更新为落地形态）

## Decisions Made

- **键集白名单锁完整形态**（Task 2 action ②『四键仍在场』与 plan prohibition『body 键集白名单断言』的机械调和）：map 解码四键逐一 delete 后残余非空即 FAIL（多键方向）+ 缺键即 FAIL（少键方向），200/503 两态同锁——draining body 同构四字段（RESEARCH A5 落锤）
- **draining 置位精确落点 = Shutdown 首行**（emitEvent shutdown 之前）：plan『首行（hubMu 锁定之前）』字面执行；08-02 注释预留的「同函数不同点」约定兑现
- **sessionAlive 置 false 落点 = session_end emit 之后、Drain 之前**：plan『同区段』内与事件 emit 相邻，审计语义无碍；waitExit 收码即测试同步边（exitf 在 lifecycle 末尾程序序保证）

## Deviations from Plan

None - plan executed exactly as written.（Rule 1-4 零触发；getHealthz 白名单强化与 TestHealthzDraining 复访断言属 plan prohibition/behavior 字面内的测试形态落实，已登记 Decisions Made；冒烟首轮 harness 参数修正（--bind 127.0.0.1 loopback 要求 + --stop-timeout 时长单位）为执行侧夹具迭代，非代码面偏差）

## Authentication Gates

None——纯本地执行，无认证门。

## Known Stubs

无——四字段全部真实数据源接线（registry.n.Load()/maxClients/sessionAlive.Load()/draining.Load()），无占位值/空数据流/TODO。

## Next Phase Readiness

- **08-04 数据源零新挂点**：`wesh_session_active` gauge 的读取端 = `s.sessionAlive.Load()`（本 plan 就位，与 D-05 口径一致）；draining 位若需进 metrics 亦同法读取；health.go 的「version 只在 /metrics build_info」红线注释为 08-04 的 D-10 镜像面
- **08-05 S4 draining 场景的行为锁已就位**：phase08.mjs 可用本 plan 冒烟同款夹具（trap 忽略 stop-signal + --stop-timeout 拉宽窗口）确定性断言 503；flagged_assumptions 登记的「默认配置 draining 窗口 <1s」已实证（bash 速死场景 0.3s 后进程已退出，HTTP 000）
- **无阻塞项**

## Self-Check: PASSED

- 文件：health.go ✓（新建）、health_test.go ✓（新建）、server.go ✓（修改）、08-03-SUMMARY.md ✓
- 提交：397cfd9 ✓、81f0b2b ✓、f32a663 ✓、97b9b66 ✓
- 关键指纹：health.go `func (s *Server) healthzHandler` ×1 ✓、server.go `"GET /healthz"` ×1 ✓ / `sessionAlive` ×4（≥3）✓ / `draining.Store(true)` ×1 ✓、health_test.go `TestHealthzDraining` ×2 ✓、health.go `basicAuth` ×0 ✓
- 验收命令全过：`go test -race -count=1 ./...` 五包绿（server 54.9s）；`go vet ./internal/server/` 净；GOROOT gofmt 三文件净；八 UAT 脚本退出 0（132 断言）
