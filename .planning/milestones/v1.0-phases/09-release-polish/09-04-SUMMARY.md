---
phase: 09-release-polish
plan: 04
subsystem: web-serving
tags: [custom-index, cli, toml, gzip, security-headers, behavior-lock, ops-03]

requires:
  - phase: 07-deployment
    provides: "cmd/wesh 两阶段合并 + fileConfig 指针标量 + validateStartup 校验矩阵（--cwd stat 预检同位先例）+ parseArgs 头注释 flag 计数纪律——index/index-max-size 两键与 --index flag 的宿主机制"
  - phase: 09-release-polish
    provides: "09-02 decodeFileConfig reader 接缝（config.go 两新键追加零扰动，全量既有子测零改动）；09-08 双机拓扑先例（本 plan 全部验证在 Linux 侧 Go 层完成）"
provides:
  - "CLI flag --index（cmd/wesh/main.go 第 31 个 flag，全名无短选项）+ config.index/indexMaxSize 字段 + loadCustomIndex 启动读入（LimitReader(max+1)）+ validateStartup stat 预检与 ≤0 拒绝 + Options.CustomIndex 透传"
  - "fileConfig Index/IndexMaxSize 两键（覆盖面 27→29 键）——index 键 flag 同名（配置铺底 CLI 覆盖）、index-max-size 整数字节纯配置键（无 CLI flag，D-08 明示例外）"
  - "func WithCustomIndex(h http.Handler, page []byte) http.Handler（web/embed.go——gzip 预压缓存装饰器：装饰期 BestCompression 预压一次 + Vary: Accept-Encoding 恒发 + acceptsGzip 复用 + index.html 路径整页替换）"
  - "Options.CustomIndex []byte + Server.customIndex + Handler() 单点装饰（wh 唯一持有点——凭据/无认证两分支与 sharePage 委托 page 参数同获装饰态，/ 与 /s/{token}/ 双通道统一，sharetoken.go 零改动）"
  - "TestLoadCustomIndex（读入矩阵：不可读/超限/恰顶/0 字节 + 红线探针）+ TestCustomIndex（伺服行为锁八子测：双通道 byte-identity/相对资源 404/gzip/Vary/安全头同源/0 字节/base-path/认证面不变/nil 兜底）+ TestStartupMatrix 四行 + TestParseArgs --index 行 + config_test 六子测"
affects: [09-05（协议层全链 UAT 的被测实现面）, 09-09（README --index 节与 index-max-size 例外写明 D-08）, ship]

actuals:
  tokens: 16157
  tasks: 2
  commits: 5

tech-stack:
  added: []  # compress/gzip 为 stdlib，零新依赖；go.mod/go.sum 逐字节不动（验收 git diff --exit-code 锁定）
  patterns:
    - "装饰器单点统一双通道：wh 唯一持有点（Handler() wh 获取与错误兜底之后）装饰，凭据/无认证两分支与 registerShareRoutes 的 page 参数同获装饰态——D-06 经 sharePage 委托上游统一，sharetoken.go 零改动（机械证据：git diff --exit-code）"
    - "gzip 装饰期预压一次缓存：定长 page 启动压缩运行期只读（BestCompression）；Vary: Accept-Encoding 恒发 + acceptsGzip 复用（零第二份 Accept-Encoding 解析器，grep ==2 锁定）"
    - "纯配置键形态：默认值直接赋值不经 flag 注册（D-08——P7 D-03 纪律明示例外；机械断言 ！grep 'Var(&cfg.indexMaxSize' 排除 flag 注册触碰）"
    - "LimitReader(max+1) 读入边界：恰顶放行/超顶即拒双锁（+1 消费多读一字节判定，防 io.ReadAll 无顶读入误指巨大文件 OOM，Pitfall 9/T-09-04b）"

key-files:
  created:
    - internal/server/customindex_test.go
  modified:
    - cmd/wesh/config.go
    - cmd/wesh/main.go
    - cmd/wesh/config_test.go
    - cmd/wesh/main_test.go
    - internal/server/server.go
    - web/embed.go

key-decisions:
  - "Task 1 确认门 as-locked（用户 2026-08-30 裁决）：D-05 整页替换（ttyd -i 同款零模板注入面）/ D-06 全通道统一（/ 与 /s/{token}/ 同一字节源）/ D-07 --index 启动一次读入 + exit 2 fail-fast + TOML 键 index / D-08 默认 16MiB 硬顶 + index-max-size 仅 TOML 键——按 09-CONTEXT/09-UI-SPEC 锁定值逐字落地"
  - "Options.CustomIndex 字段提前至 Task 2 feat 提交落地（Rule 3 最小跨文件必要）：Task 2 验收 grep 要求 main.go Options 字面量含 CustomIndex: customIndex，字段缺失使 cmd/wesh 包不可编译；plan 级 files_modified 已含 internal/server/server.go，仅落 Options 字段（Server 字段/New 接线/Handler 装饰仍归 Task 3）"
  - "TestStartupMatrix 全部既有行注入 indexMaxSize: 16 << 20 基线（maxClients: 32 基线同步先例同款，表头注释登记）：validateStartup 新增无条件 ≤0 拒绝后直接构造的 config 零值被误拒——生产路径 parseArgs 恒注入默认，注入基线是测试直构形态的必要同步"
  - "gzip 测试断言按 Go transport 显式头形态：裸 http.Get 被 transport 自动加 Accept-Encoding: gzip 且透明解压（明文态结构性不可观测）——identity/gzip 两显式编码直证两伺服态（fetchPage helper 显式设头）"
  - "不可读拒绝的 root 豁免（os.Getuid()==0 skip）：root 无视 chmod 000，非 root 限定测试与 07-02 Chown EPERM 注入同款形态"

requirements-completed: [OPS-03]

coverage:
  - id: D1
    description: "CLI/config 半侧（Task 2）：--index flag（第 31 个）+ index/index-max-size 两 TOML 键 + validateStartup stat 预检（不存在/非常规）与 ≤0 拒绝 + loadCustomIndex 读入（不可读/超限含上限数值/恰顶/0 字节）+ Options 透传——错误行只含路径+类别+上限数值零内容字节（D-08 红线，单元+run 级探针反断言双锁）"
    requirement: OPS-03
    verification:
      - kind: unit
        ref: "go test -count=1 ./cmd/wesh/ -run 'TestParseArgs|TestStartupMatrix|TestLoadFileConfig|TestConfigMerge|TestLoadCustomIndex'（全 PASS）"
        status: pass
      - kind: other
        ref: "go test -race -count=1 ./cmd/wesh/ 全包绿 + go vet 清零 + 验收 grep 组全过（toml 两键各 ==1、flag 注册 ==1、无 indexMaxSize flag 注册、loadCustomIndex 签名 ==1、Options 字面量 ==1、头注释计数 31 ==1）+ git diff --exit-code go.mod go.sum 退出 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "伺服半侧（Task 3）：WithCustomIndex 装饰（gzip 预压 + Vary 恒发 + acceptsGzip 复用）+ Handler() 单点装饰双通道统一（sharetoken.go 零改动）+ TestCustomIndex 八子测行为锁（双通道 byte-identity/相对资源 404/gzip 双态/安全头同源/0 字节/base-path 组合/认证面不变/nil 兜底）"
    requirement: OPS-03
    verification:
      - kind: unit
        ref: "go test ./internal/server/ -race -count=1 -run 'TestCustomIndex' -v（八子测全 PASS）"
        status: pass
      - kind: other
        ref: "go test -race -count=1 ./... 五包全绿 + 验收 grep 组全过（WithCustomIndex 签名 ==1 / server.go 调用 ==1 / acceptsGzip 调用 ==2 / CustomIndex []byte ==2）+ git diff --exit-code internal/server/sharetoken.go 退出 0"
        status: pass
      - kind: other
        ref: "真实二进制冒烟（CGO_ENABLED=0 构建）：--index probe.html --port 0 启动——curl / 与 /index.html 返回探针字节、curl /x.css 404、curl -H 'Accept-Encoding: gzip' / 返回 Content-Encoding: gzip 体（gunzip = 探针）+ Vary/Content-Type/CSP 头在场；--index 不存在与目录二进制级 exit 2 错误行逐字正确"
        status: pass
    human_judgment: false

duration: 28min
completed: 2026-08-30
status: complete
---

# Phase 9 Plan 04: 自定义首页 --index 全链（OPS-03）Summary

**OPS-03 自定义首页全链落地：--index flag（第 31 个）+ TOML index/index-max-size 两键（27→29 键，index-max-size 纯配置键无 flag）+ validateStartup stat 预检与 ≤0 拒绝 + loadCustomIndex LimitReader(max+1) 启动读入（四拒绝 exit 2、错误行零内容字节红线）+ WithCustomIndex 装饰器（gzip 预压缓存 + Vary 恒发 + acceptsGzip 复用）+ Handler() 单点装饰使 / 与 /s/{token}/ 双通道 byte-identity 统一（sharetoken.go 零改动）+ TestCustomIndex/TestLoadCustomIndex 行为锁全绿 + 真实二进制冒烟全过**

## Performance

- **Duration:** 28 min
- **Started:** 2026-08-30T14:55:00+08:00
- **Completed:** 2026-08-30T15:23:00+08:00
- **Tasks:** 2（Task 1 确认门由用户裁决 as-locked 完成；Task 2/3 本执行器落地）
- **Files modified:** 7（1 新建 + 6 修改）

## Accomplishments

- **CLI/config 半侧（Task 2）**：`--index` StringVar 注册（parseArgs 头注释 30→31 + Phase 9 行）；`indexMaxSize` 默认 16MiB 直接赋值不经 flag 注册（D-08 纯配置键——P7 D-03 纪律明示例外，`! grep 'Var(&cfg.indexMaxSize'` 机械断言）；fileConfig `Index *string`/`IndexMaxSize *int` 两键（27→29 键，index-max-size 整数字节 OQ1 推荐形态——字符串形态由 go-toml 类型不符自然拒绝）；validateStartup --cwd 同位 stat 预检（不存在/非常规两类别）+ 无条件 ≤0 拒绝；loadCustomIndex `io.LimitReader(max+1)` 读入（不可读/超限含上限数值，0 字节合法）；run() TLS 预检后 pty.Start 前调用（exit 2 零资源占用纪律）；Options 字面量 `CustomIndex: customIndex` 生产直传
- **伺服半侧（Task 3）**：web/embed.go `WithCustomIndex(h, page)` 装饰器——装饰期 gzip BestCompression 预压一次缓存（stdlib 零新依赖）、Vary: Accept-Encoding 恒发（Handler 同款纪律）、acceptsGzip 复用（零第二份解析器，grep ==2）、Content-Type 按 .html 扩展名推断；index.html 路径（含空路径回落）返回启动读入字节 byte-identity，其余路径照旧委托（相对资源 404 契约语义）；server.go Options.CustomIndex/Server.customIndex 字段 + Handler() wh 唯一持有点单点装饰——凭据/无认证两分支与 registerShareRoutes page 参数同获装饰态，/ 与 /s/{token}/ 经 sharePage 委托自然统一（D-06），sharetoken.go 零改动
- **测试面**：TestParseArgs --index 行 + indexMaxSize 默认 16MiB 全行恒定断言（CLI 面结构性无该 flag 的行为锁）；TestStartupMatrix 四新行 + 全部既有行 indexMaxSize 基线注入（maxClients 先例同款）；TestLoadCustomIndex 四拒绝矩阵（不可读 chmod 000 非 root/超限含探针反断言/恰顶放行/0 字节放行）；TestStartupRefusalNoResource run 级 17MiB 默认顶 exit 2 + stderr 零探针端到端；config_test 六新子测（两键解码/缺席 nil/字符串形态拒绝/index 配置铺底/CLI 覆盖/index-max-size 生效与 0 值拒绝）；TestCustomIndex 八子测行为锁——全部既有测试零回归（`go test -race -count=1 ./...` 五包全绿）
- **真实二进制冒烟**：CGO_ENABLED=0 构建静态二进制 `--index probe.html --port 0` 启动——curl `/` 与 `/index.html` 返回探针字节逐字一致、`/x.css` 404、gzip 请求返回 Content-Encoding: gzip 体（gunzip = 探针）且 Vary/Content-Type/CSP 安全头全在场；`--index` 不存在/目录二进制级 exit 2 错误行逐字正确（`invalid --index "...": file does not exist` / `not a regular file`）
- **零依赖证据**：go.mod/go.sum 逐字节不动（compress/gzip stdlib）；协议层全链 UAT（phase09.mjs）按计划属 09-05

## Task Commits

Each task was committed atomically (TDD discipline: test → feat):

1. **Task 2 RED: --index/两键/四拒绝失败测试（编译失败于 undefined 字段）** - `61b86ee` (test)
2. **Task 2 GREEN: --index flag + 校验矩阵 + 两 TOML 键 + 启动读入** - `8319467` (feat)
3. **config.go 覆盖面注释 gofmt doc-comment 重排收尾** - `cef99d0` (style)
4. **Task 3 RED: TestCustomIndex 行为锁八子测（运行时失败——无接线）** - `b8c3da4` (test)
5. **Task 3 GREEN: WithCustomIndex 装饰器 + server.go 接线** - `93b7d91` (feat)

**Plan metadata:** 见末尾 docs 提交（docs(09-04): complete ...）

## Files Created/Modified

- `cmd/wesh/config.go` - fileConfig Index/IndexMaxSize 两键（覆盖面注释 27→29 键，D-08 纯配置键例外登记）
- `cmd/wesh/main.go` - config.index/indexMaxSize 字段、--index flag 注册（第 31 个）、默认值铺底与 fc 合并、validateStartup stat 预检与 ≤0 拒绝、loadCustomIndex 函数、run() 读入调用与 Options 透传、io import
- `cmd/wesh/config_test.go` - 六新子测（index keys decode/absent-nil、index-max-size 字符串形态拒绝、index 配置铺底/CLI 覆盖、index-max-size 生效/0 值拒绝）；既有子测零改动
- `cmd/wesh/main_test.go` - TestParseArgs wantIndex 行与 indexMaxSize 恒定断言；TestStartupMatrix 四新行 + 既有行 16MiB 基线注入；TestLoadCustomIndex 新测试；TestStartupRefusalNoResource run 级超限子测
- `internal/server/server.go` - Options.CustomIndex 字段（Task 2 最小必要）+ Server.customIndex 字段 + New 直传 + Handler() 单点装饰
- `web/embed.go` - WithCustomIndex 装饰器（bytes/compress-gzip import + 函数尾追加）
- `internal/server/customindex_test.go` - TestCustomIndex 黑盒行为锁（package server_test，fetchPage/assertCustomPage helper + 八子测）

## Decisions Made

- **Options.CustomIndex 字段提前至 Task 2**（Rule 3 最小跨文件必要）：Task 2 验收 grep 要求 main.go Options 字面量含 `CustomIndex: customIndex`——字段缺失使包不可编译；plan 级 files_modified 已含 server.go，Task 2 只落 Options 字段，其余接线归 Task 3（见 Deviations）
- **TestStartupMatrix 基线注入**：validateStartup 无条件 ≤0 拒绝与直构 config 零值的调和——既有行全部注入 `indexMaxSize: 16 << 20`（maxClients: 32 基线同步先例同款，表头注释登记依据）
- **gzip 测试编码策略**：Go transport 显式 Accept-Encoding 头不经自动协商与透传解压——identity/gzip 两显式编码直证明文/gzip 两伺服态（裸 http.Get 的 transport 自动 gzip 使明文态不可观测）
- **gzip.NewWriterLevel 错误防御**：BestCompression 常量恒合法使 err 分支结构性不可达，防御性回落 NewWriter 保持预压不变量
- **无效 token 无认证模式给同一自定义页**：sharePage 无效 token 改写 "/" 委托 root 链（无认证模式 root = 装饰态 wh）——现状语义保持的回归锁（TestCustomIndex dual channel 第四行）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Options.CustomIndex 字段提前至 Task 2 feat 提交**
- **Found during:** Task 2 GREEN（实现 main.go Options 字面量时）
- **Issue:** plan Task 2 验收 grep 要求 `grep -c 'CustomIndex: customIndex' cmd/wesh/main.go` == 1，但 Options.CustomIndex 字段按 task 划分归 Task 3（server.go 三处接线之一）——字段缺失使 cmd/wesh 包不可编译，Task 2 verify 无法执行
- **Fix:** Task 2 feat 提交内联落 Options.CustomIndex 字段声明（含生产直传注释）；Server.customIndex 字段/New 直传/Handler() 装饰仍归 Task 3——plan 级 files_modified 已含 internal/server/server.go，无范围蔓延
- **Files modified:** internal/server/server.go（Options 字段 +8 行注释）
- **Verification:** Task 2/Task 3 两 verify 命令分别全绿；Task 3 验收 grep（CustomIndex []byte == 2）仍按原样满足
- **Committed in:** `8319467`（Task 2 feat）

**2. [Rule 1 - Bug] config.go 覆盖面注释经 gofmt doc-comment 重排语义断裂**
- **Found during:** Task 2 gofmt 收尾
- **Issue:** 头注释续行以 `+ index-max-size ...` 开头被 gofmt（1.26 doc comment 格式化）解析为独立列表项，D-04 条目首行语义截断
- **Fix:** 改写为单列表项完整表述（「27 个长期运行 flag 同名键、command exec 数组与 index-max-size 纯配置键，共 29 键」）；独立 style 提交落地
- **Files modified:** cmd/wesh/config.go（2 行注释）
- **Verification:** gofmt -l 清零 + go build 通过
- **Committed in:** `cef99d0`

---

**Total deviations:** 2 auto-fixed（1 Rule 3 task 边界最小必要、1 Rule 1 注释排版语义修正）
**Impact on plan:** 交付物与 must_haves 逐字一致；无范围蔓延。

## Issues Encountered

- GOROOT gofmt 全量检查发现 `internal/server/clients.go` 与 `internal/server/emptyexit_test.go` HEAD 即存在漂移（非本 plan 引入、本 plan 未触达——scope boundary）——已登记 `.planning/phases/09-release-polish/deferred-items.md`（config_test.go 既有漂移条目同步补注 09-04 触达情况），按既定先例随后续 plan 六段式段 1 或独立 style 提交清零
- fish shell 内 `cmd & sleep ...` 后台链的组合行为使首轮冒烟端口探测失准（背景进程随 Bash 调用消亡）——改用 run_in_background 任务 + 服务端输出端口直取后全过（执行通道修正，非交付物问题）

## Known Stubs

None —— 无占位/硬编码空值/TODO；全部行为面经单元（-race）+ run 级 + 真实二进制冒烟三层实证。协议层全链 UAT（phase09.mjs）按计划属 09-05（wave 3），非 stub。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 09-05 协议层 UAT（phase09.mjs）获得完整被测实现面：--index 四拒绝 exit 2 与错误行零内容、双通道给页、gzip/Vary、相对资源 404、/api/attach 与 /ws 照旧、base-path 组合、TOML 配置通道——Go 层行为锁（TestCustomIndex/TestLoadCustomIndex）已全部预锁，UAT 对真实二进制复验同一行为面
- 09-09 README 节素材：--index 整页替换语义 + README-1「自定义页须自行实现终端逻辑，否则分享链接失去终端功能」（D-06 承诺语）+ README-2「自包含单 HTML：相对资源 404、CSP 阻断外部源」（§5 推论）+ index-max-size 纯配置键例外写明（D-08 防例外蔓延）——错误文案常量（`invalid --index %q: ...` 四类别 + `invalid index-max-size: must be positive`）已定型可直接引用
- loadCustomIndex/WithCustomIndex/Options.CustomIndex 均可被后续 plan 直接引用（BasePath/AuthHeader/Version 生产直传先例形态）

## Self-Check: PASSED

- Files: cmd/wesh/config.go / cmd/wesh/main.go / cmd/wesh/config_test.go / cmd/wesh/main_test.go / internal/server/server.go / web/embed.go / internal/server/customindex_test.go / 09-04-SUMMARY.md 均 FOUND
- Commits: 61b86ee（Task 2 RED）/ 8319467（Task 2 GREEN）/ cef99d0（style）/ b8c3da4（Task 3 RED）/ 93b7d91（Task 3 GREEN）均 FOUND

---
*Phase: 09-release-polish*
*Completed: 2026-08-30*
