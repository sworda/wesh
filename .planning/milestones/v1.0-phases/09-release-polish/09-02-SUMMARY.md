---
phase: 09-release-polish
plan: 02
subsystem: testing
tags: [go-fuzz, testing.F, toml, ci, github-actions, security-regression]

requires:
  - phase: 07-deployment
    provides: "cmd/wesh/config.go loadFileConfig + DisallowUnknownFields 严格模式 + 值剥离红线（configErr 单写口）——decodeFileConfig 接缝的迁移源"
  - phase: 02-protocol
    provides: "internal/proto DecodeHello/DecodeResize/ClampDim [1,1000] 契约——proto fuzz 不变量依据"
provides:
  - "func decodeFileConfig(path string, r io.Reader) (*fileConfig, error)（cmd/wesh/config.go——reader 委托接缝，错误分类/configErr 唯一副本；loadFileConfig 缩为 open-file + 委托 + D-07 警告，行为逐字节保持）"
  - "func FuzzDecodeFileConfig(f *testing.F)（cmd/wesh/fuzz_test.go，package main——值剥离红线 fuzz 断言形态第二锁）"
  - "func FuzzDecodeHello / FuzzDecodeResize（internal/proto/fuzz_test.go，package proto_test——ClampDim 契约 fuzz 断言形态）"
  - "ci.yml fuzz job（两独立 go test -fuzz 调用 ×60s，与 go/web 并列——D-10 CI 短跑回归门）"
affects: [09-09（发布脚本 2×10min 长 fuzz 目标已真实可跑）, ship, ci]

actuals:
  tokens: 2813
  tasks: 2
  commits: 4

tech-stack:
  added: []  # Go stdlib testing.F 零新依赖（D-09 裁决）；go.mod/go.sum 验收逐字节不动
  patterns:
    - "fuzz 接缝重构：path-in API 提取 reader 委托使 bytes-in 可测，错误分类/红线逻辑保持单写口不复制第二份"
    - "值剥离红线的 fuzz 断言形态：探针值入种子语料，err.Error() 不含探针即不变量（键名回显合法不在断言面）"
    - "CI fuzz 短跑门：独立 job 两调用 ×60s（-fuzz 单包单目标结构性约束）；种子/崩溃语料随常规 go test 零时长回归"

key-files:
  created:
    - cmd/wesh/fuzz_test.go
    - internal/proto/fuzz_test.go
  modified:
    - cmd/wesh/config.go
    - .github/workflows/ci.yml

key-decisions:
  - "proto fuzz 直挂既有导出函数零改造（plan must_have 既定）——RED 阶段种子即 PASS 是设计内行为（回归门性质，非新行为驱动），按 TDD fail-fast 规则调查后继续并登记"
  - "TDD 提交拆分：plan 字面每 task 单提交信息按 tdd=\"true\" 纪律拆为 test+feat 两提交（Task 1 RED 真编译失败；Task 2 test= fuzz 目标、feat=CI fuzz leg）"
  - "ci.yml YAML 静态审查经 ephemeral docker python:3.12-alpine + pyyaml（宿主无 yaml 模块，09-01 通道第二次沿用），并强化为全结构断言（三 job 集合/fuzz 步骤序/钉版/无 race）"

patterns-established:
  - "fuzz 目标文件头注释登记 D 编号 + 两不变量 + 种子五类 + 双层运行形态（CI 短跑/发布长跑）——后续 fuzz 目标同构"
  - "验收 grep 机械纪律第五次沿用：ci.yml 注释避开 fuzztime=60s / go test -race 字面（注释提及同样计数）"

requirements-completed: [OPS-10]

coverage:
  - id: D1
    description: "decodeFileConfig reader 委托接缝（cmd/wesh/config.go）——解码+错误分类三分支+configErr 包装逐字迁入单写口；loadFileConfig 缩为 open-file+委托+D-07 警告；config_test.go 既有全部测试零改动全绿（行为保持证据）"
    requirement: OPS-10
    verification:
      - kind: unit
        ref: "go test -count=1 ./cmd/wesh/（全包绿，含 unknown keys/invalid toml/cannot read/D-07 警告/值剥离探针全部既有子测）"
        status: pass
      - kind: other
        ref: "验收 grep 组：decodeFileConfig 签名 ==1 / DisallowUnknownFields ==1 / configErr( ==5(≥4)；git diff --exit-code go.mod go.sum 退出 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "FuzzDecodeFileConfig（cmd/wesh/fuzz_test.go）——种子五类单测化 PASS + 60s 实跑 PASS（~5.0M execs）无崩溃语料；不变量：不 panic + err.Error() 不含 FUZZ_PROBE_SECRET"
    requirement: OPS-10
    verification:
      - kind: unit
        ref: "go test -run FuzzDecodeFileConfig ./cmd/wesh/（seed#0..#4 全 PASS）"
        status: pass
      - kind: other
        ref: "go test -fuzz=FuzzDecodeFileConfig -fuzztime=60s ./cmd/wesh/ → PASS，cmd/wesh/testdata 无崩溃语料产出"
        status: pass
    human_judgment: false
  - id: D3
    description: "FuzzDecodeHello / FuzzDecodeResize（internal/proto/fuzz_test.go）——种子五类单测化 PASS + 各 60s 实跑 PASS；ClampDim 契约不变量（成功 ⇒ Cols/Rows ∈ [1,1000]）+ 不 panic"
    requirement: OPS-10
    verification:
      - kind: unit
        ref: "go test -run 'FuzzDecode' ./internal/proto/（两目标 seed#0..#4 全 PASS）"
        status: pass
      - kind: other
        ref: "go test -fuzz=FuzzDecodeHello -fuzztime=60s ./internal/proto/ 与 go test -fuzz=FuzzDecodeResize -fuzztime=60s ./internal/proto/（两次独立调用）→ 各 PASS，internal/proto/testdata 无崩溃语料"
        status: pass
    human_judgment: false
  - id: D4
    description: "ci.yml fuzz job（D-10 CI 短跑门）——与 go/web 并列第三 job，两次独立 go test -fuzz 调用 ×60s，不加 -race；go/web 两 job 逐字节不动；全量 go test -race 零回归"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "ephemeral docker pyyaml 全结构断言（jobs=={go,web,fuzz}/步骤序/钉版/runs 精确两调用/无 race）+ 验收 grep 组（fuzztime=60s ==2 / ^  fuzz: ==1 / go test -race ==1）"
        status: pass
      - kind: other
        ref: "go vet ./... && go test -race -count=1 ./... 全量绿（5 包，接缝重构+新文件对既有面零回归）"
        status: pass
    human_judgment: true
    rationale: "fuzz job 的 CI 实跑首证 = 下次 push/PR 触发 GitHub Actions（与 09-01 release.yml 同类既定验证取舍）；本 plan 交付静态锁定形态 + 本机等价命令全绿证据"

duration: 21min
completed: 2026-08-29
status: complete
---

# Phase 9 Plan 02: fuzz 验证面落地（proto 帧解码 + TOML 配置解析 + CI 短跑门） Summary

**D-09/D-10 fuzz 面三目标全绿入库：decodeFileConfig reader 接缝（错误分类单写口逐字迁移、config_test.go 全量零改动）+ FuzzDecodeFileConfig 值剥离红线 fuzz 断言（60s ~5.0M execs 零崩溃）+ FuzzDecodeHello/FuzzDecodeResize ClampDim 契约断言（双 60s 零崩溃）+ ci.yml 第三 job 2×60s 短跑回归门**

## Performance

- **Duration:** 21 min
- **Started:** 2026-08-29T13:51:05Z
- **Completed:** 2026-08-29T14:11:54Z
- **Tasks:** 2
- **Files modified:** 4（2 新建 + 2 修改）

## Accomplishments

- `cmd/wesh/config.go` 接缝重构落地：`decodeFileConfig(path string, r io.Reader) (*fileConfig, error)` 承载 toml 解码 + 错误分类三分支（StrictMissingError 键名清单 / DecodeError Key()+Position() / 兜底 cannot parse）+ configErr 包装——逐字迁移单写口不复制第二份；`loadFileConfig` 缩为 os.Open（cannot read 原位）+ defer Close + 委托 + D-07 权限警告（Stat 仍按 path）；既有 config_test.go 全部测试零改动全绿（行为保持硬证据）；验收 grep 组（签名 ==1 / DisallowUnknownFields ==1 / configErr( ≥4）全过
- `cmd/wesh/fuzz_test.go` FuzzDecodeFileConfig：种子五类（合法键 / FUZZ_PROBE_SECRET 探针键 / 未知键 / 类型不符 / 非 UTF-8 二进制）单测化全 PASS；60s 实跑 PASS（~5.0M execs，峰值 ~800K/s）零崩溃零语料产出；值剥离红线获 fuzz 断言形态第二锁（config_test 探针子测之外）——T-09-02b 缓解落地
- `internal/proto/fuzz_test.go` FuzzDecodeHello/FuzzDecodeResize（package proto_test）：直挂既有导出函数零改造，种子五类单测化全 PASS；双 60s 实跑（两次独立调用，Pitfall 4）各 PASS 零崩溃；ClampDim 契约（成功 ⇒ 尺寸恒在 [1,1000]）+ 不 panic 两不变量获持续回归门——T-09-02a 远程输入面缓解落地
- `.github/workflows/ci.yml` 新增 fuzz job（与 go/web 并列顶层第三 job）：checkout@v7.0.1 → setup-go@v7.0.0（go-version-file）→ 两次独立 go test -fuzz ×60s；不加 -race；go/web 两 job 逐字节不动；ephemeral pyyaml 全结构断言 + 验收 grep 组全过；go vet ./... && go test -race -count=1 ./... 全量绿（5 包零回归）
- 零新依赖证据：go.mod/go.sum 逐字节不动（Go stdlib testing.F，D-09 裁决）；60s 冒烟产物 $GOCACHE/fuzz 未进仓；两 testdata/fuzz/ 入库点首跑无发现未创建（D-10 通道就绪）

## Task Commits

Each task was committed atomically (TDD discipline: test → feat):

1. **Task 1 RED: FuzzDecodeFileConfig 失败测试（decodeFileConfig 未提取，编译失败）** - `6b8b03c` (test)
2. **Task 1 GREEN: decodeFileConfig reader 委托接缝 + 60s 冒烟全绿** - `fab61a0` (feat)
3. **Task 2 RED: proto fuzz 两目标（回归门性质，种子即 PASS——设计内，见 Decisions）** - `674df33` (test)
4. **Task 2 feat: ci.yml fuzz job（2×60s 短跑门）+ 全量 -race 零回归** - `5728d96` (feat)

**Plan metadata:** 见末尾 docs 提交（docs(09-02): complete ...）

## Files Created/Modified

- `cmd/wesh/config.go` - decodeFileConfig reader 委托接缝提取（+io import；错误分类/configErr 唯一副本迁入委托；loadFileConfig 缩为 open+委托+D-07 警告）
- `cmd/wesh/fuzz_test.go` - FuzzDecodeFileConfig（package main，调未导出接缝；头注释登记 D-09/D-10 与两不变量）
- `internal/proto/fuzz_test.go` - FuzzDecodeHello/FuzzDecodeResize（package proto_test；头注释登记 D-09/D-10、bytes-in 零改造、帧拆分型字节面 server.go len 守卫无独立目标必要）
- `.github/workflows/ci.yml` - fuzz job（D-10 CI 短跑回归门注释 + 两独立调用 ×60s）

## Decisions Made

- **proto 侧 RED 即 PASS 按设计处理并登记**：plan must_have 明确「直挂既有导出函数零改造」——ClampDim 契约在被测函数内既存，fuzz 目标是回归门而非新行为驱动；按 TDD fail-fast 规则调查（「feature may already exist」——确为设计内），测试确在断言真实不变量（t.Fatalf 路径经种子负值/超大输入覆盖 ClampDim 上下界），继续执行。plan frontmatter type=execute（非 type: tdd），plan 级 RED/GREEN 门序列不适用；task 级 tdd="true" 纪律以 test→feat 提交拆分兑现
- **TDD 提交拆分**：plan 字面每 task 单条提交信息按 tdd="true" 拆为 test+feat——Task 1 RED 真实编译失败（undefined: decodeFileConfig）满足 MUST-fail；Task 2 test 提交 = fuzz 目标文件、feat 提交 = CI fuzz leg（plan 提交信息语义由两提交合并承载）
- **ci.yml 注释避开验收 grep 字面**（05-08/09-01 机械纪律第五次沿用）：fuzz job 注释写「-fuzz 每次只能匹配单包单目标」「不加竞态检测器」，不出现 fuzztime=60s / go test -race 字面（注释提及同样计数）；:15 CGO 纪律注释逐字不动（09-01 truths 引用它）
- **YAML 静态审查通道第二次沿用**：宿主 python3 无 yaml 模块，ephemeral docker python:3.12-alpine + pyyaml（容器 --rm 即弃零污染）；由纯语法检查强化为全结构断言（jobs 集合恰 {go,web,fuzz} / fuzz 步骤 uses 序与钉版 / runs 两调用逐字 / go-version-file / 无 race）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] plan read_first 描述 proto_test.go 为「package proto_test 外部包形态」，实际为 package proto**
- **Found during:** Task 2（read_first 文件核查）
- **Issue:** plan 与 PATTERNS §8 均称 proto_test.go 是外部包先例；实证 proto_test.go 为 `package proto`（内部包）。must_have 本身（fuzz_test.go 用 package proto_test）不受影响——Go 允许同目录内/外测试包共存
- **Fix:** 按 must_have 逐字落地：fuzz_test.go 采 package proto_test（外部包只触达导出 API，纪律更优且与 plan 交付物一致）；proto_test.go 零改动
- **Files modified:** 无（仅 plan 文档描述偏差；交付物与 must_haves 逐字一致）
- **Verification:** go test ./internal/proto/ 编译运行全绿（两测试包共存实证）
- **Committed in:** `674df33`（Task 2 test 提交）

**2. [Rule 3 - Blocking] 宿主 python3 无 yaml 模块，plan verify 的 ci.yml YAML 检查无法按字面执行**
- **Found during:** Task 2（ci.yml 验证阶段）
- **Issue:** plan verify 要求 `python3 -c "import yaml; yaml.safe_load(...)"`；宿主无 yaml 模块（09-01 已登记同问题）；禁 pip 装包入宿主（Rule 3 排除纪律）
- **Fix:** ephemeral docker python:3.12-alpine + 容器内 pip pyyaml（09-01 既定通道第二次沿用；检查脚本写 /tmp 文件挂载防 shell 多层转义）；同通道将纯语法检查强化为全结构断言（见 Decisions）
- **Files modified:** 无（/tmp 检查脚本随容器生命周期，不进仓）
- **Verification:** CI_YAML_STRUCTURE_OK + 验收 grep 组全过
- **Committed in:** `5728d96`（Task 2 feat 提交）

---

**Total deviations:** 2 auto-fixed（1 Rule 1 plan 文档描述修正，1 Rule 3 验证环境适配——09-01 通道复用）
**Impact on plan:** 全部为执行通道修正，交付物形态与 plan must_haves 逐字一致；无范围蔓延。

## Issues Encountered

- 新建 fuzz_test.go 两文件的 f.Add 行尾注释对齐不符 GOROOT gofmt CJK 宽度口径——各自提交前 gofmt -w 清零（纯排版）；顺带发现 `cmd/wesh/config_test.go` 在 HEAD 即存在 GOROOT gofmt 漂移（非本 plan 引入，超 scope boundary）——已登记 `.planning/phases/09-release-polish/deferred-items.md`，按既定先例随后续 style 提交清零
- TOML fuzz 初期 execs/s 有爬坡段（覆盖率引导 warmup，48s 后 ~5.0M 总量平稳）——非问题，60s 内零崩溃即验收

## Known Stubs

None —— 无占位/硬编码空值/TODO；全部行为面经单测 + 60s 实跑 + 全量 -race 实证。崩溃语料 testdata/fuzz/ 两入库点首跑无发现未创建（D-10 既定「首跑发现即入库」——通道就绪非 stub）。

## User Setup Required

None - no external service configuration required.（CI fuzz job 随下次 push/PR 自动首跑；发布前 2×10min 长跑由 09-09 发布脚本承载，D-14。）

## Next Phase Readiness

- 09-09 发布脚本的长 fuzz 步骤获得真实可跑目标：`go test -fuzz=FuzzDecodeHello -fuzztime=10m ./internal/proto/`、`go test -fuzz=FuzzDecodeResize -fuzztime=10m ./internal/proto/`、`go test -fuzz=FuzzDecodeFileConfig -fuzztime=10m ./cmd/wesh/`（逐目标调用，Pitfall 4）
- 崩溃语料入库通道就绪：任一目标实跑发现即落 `<pkg>/testdata/fuzz/` 入库，随 go leg 常规测试零时长回归（D-10 闭环）
- 后续 plan 可直接引用 decodeFileConfig 接缝（bytes-in 配置解析面）与 ci.yml fuzz job 形态
- 人工复核项（不阻塞）：fuzz job 的 GitHub Actions 实跑首证 = 下次 push/PR（09-VALIDATION 可登记观测）

## Self-Check: PASSED

- Files: cmd/wesh/fuzz_test.go / internal/proto/fuzz_test.go / cmd/wesh/config.go / .github/workflows/ci.yml / 09-02-SUMMARY.md / deferred-items.md 均 FOUND
- Commits: 6b8b03c（Task 1 RED）/ fab61a0（Task 1 GREEN）/ 674df33（Task 2 test）/ 5728d96（Task 2 feat）均 FOUND

---
*Phase: 09-release-polish*
*Completed: 2026-08-29*
