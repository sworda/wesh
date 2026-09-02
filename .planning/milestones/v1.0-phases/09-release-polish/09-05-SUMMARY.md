---
phase: 09-release-polish
plan: 05
subsystem: web-serving
tags: [custom-index, protocol-uat, behavior-lock, red-line, ops-03]

requires:
  - phase: 09-release-polish
    provides: "09-04 --index 全实现面（四拒绝 exit 2 错误行零内容/双通道给页/gzip 预压/Vary 恒发/安全头同源/base-path/TOML 两键）——本 plan 的被测物与验收 grep 形态源头；09-03 dist 重建（phase06-dom 回归面）"
  - phase: 07-deployment
    provides: "phase07.mjs 骨架件先例——check/skip/assertOutputClean 三件套 + sensitiveTokens 闭包 + startWesh/spawnExpectExit 夹具 + redactArgs 脱敏 + 启动行解析取端口与分享链接"
provides:
  - "web/uat/phase09.mjs——OPS-03 协议层全链 UAT：S1 启动校验矩阵三拒绝（不存在/目录/超限，stderr 含路径与类别且零内容探针）/S2 三通道 byte-identity（/、/index.html、/s/{ro-token}/）+ 相对资源 404 + 六安全头与内建页逐头同值/S3 gzip 预压双态 + Vary 恒发 + Content-Type/S4 认证面照旧（/api/attach 404 + WS Hello→Welcome 全链 + 凭据 401/200）/S5 0 字节 200 空 body + base-path 组合/S6 TOML index 键生效 + CLI 覆盖——18 断言全绿 + SEC 自净"
  - "RED 判别力证明通道：git archive 09-03 HEAD（49ed5b2）构建 pre-09-04 二进制跑 phase09.mjs——S1 三断言类别不匹配 FAIL、S2-S5 场景异常（flag 未定义拒启）、S6 unknown keys FAIL、SEC 自净在失败通道同样成立（exit 1）"
affects: [09-09（README --index 节每一项承诺均有对真实二进制的可执行断言——文档即被测物纪律素材）, ship]

actuals:
  tokens: 7431
  tasks: 2
  commits: 2

tech-stack:
  added: []  # 零依赖 Node 原生 http/WebSocket/zlib（gunzipSync）——web/uat/ 既有 node_modules 无新增，pnpm-lock.yaml 不动
  patterns:
    - "node:http rawFetch 原始请求通道：undici fetch 自动加 accept-encoding 且按 Content-Encoding 透传解压，使自定义页明文伺服态结构性不可观测（09-04 Go transport 自动 gzip 同款语义适配的 JS 侧对偶）——rawFetch 不注入头、不透明解压、5s 超时护栏，gzip/明文双态 + 头面（Vary/Content-Type/CSP）均可观测"
    - "UAT 脚本 RED 证明形态（09-03 旧 dist RED 先例的协议层对偶）：被测实现已在先序 plan（09-04）落地时，git archive 先序 HEAD 构建旧二进制跑新脚本证判别力——fail-fast 规则调查后确认『种子即 PASS 为设计内回归门性质』（09-02 先例第三次沿用）"
    - "verification-only 任务的 --allow-empty 提交形态：plan 指定提交语但零文件改动——空提交记录回归里程碑（原子提交协议的验证型任务实例）"
    - "探针夹具前缀反断言：PROBE_PREFIX（CIDX-PROBE-）单写口构造唯一探针串入 sensitiveTokens，S1 三行 stderr 以前缀缺席作零内容探针反断言（比单串更强——任意探针内容均被覆盖）"

key-files:
  created:
    - web/uat/phase09.mjs
  modified: []

key-decisions:
  - "rawFetch 取 node:http 而非 undici fetch（给页断言通道选型）：undici fetch 双重自动化（自动 accept-encoding + 透明解压）使 S3 明文态与 Content-Encoding 头面结构性不可观测——09-04 已实证 Go transport 同款问题并以显式编码直证；JS 侧以原始请求通道一次解决（不注入头不解压），gzip/明文/头面三断言面全可观测"
  - "task 级 tdd RED 形态裁决：被测实现属先序 plan（09-04 已落地），failing-first 提交结构性不可达（脚本即交付物、无独立实现步）——RED 以 pre-09-04 二进制（git archive 49ed5b2 构建）跑新脚本证判别力（S1 类别不匹配/S2-S5 拒启/S6 unknown key 全 FAIL、exit 1），GREEN 当前二进制 18/18 exit 0；plan action 单 test 提交字面保持（09-02『plan type=execute 不适用 plan 级 RED/GREEN 门序列』先例）"
  - "Task 2 提交取 --allow-empty（回归里程碑记录）：plan 指定『提交：test(09-05): regression pass over existing UAT suites』但 verification-only 任务零文件改动——空提交保持 per-task 原子提交协议"
  - "S2e 安全头断言扩展为六头逐头同值（must_haves『与内建页响应同值』兑现）：plan behavior 字面仅『CSP 含 connect-src \\'self\\'』，must_haves 真值行要求同源同值——对照实例（无 --index 内建页 spawn）六安全头逐头比对（CSP/X-Frame-Options/X-Content-Type-Options/Referrer-Policy/COOP/CORP），强于 plan 字面且断言面在 scope 内"
  - "startWesh 省略 unix 分支、dialHello 收窄无参形态（phase07 夹具复用的本 phase 消费面收窄）：phase09 全 TCP 场景无 --socket/自定义头/路径需求——核心语义（defaultListen 前置、redactArgs、启动行解析、sensitiveTokens 登记）逐字保持，注释登记省略依据"
  - "S1 探针反断言取 PROBE_PREFIX 前缀而非单串：三行 stderr 共用前缀缺席断言覆盖任意探针内容（writeProbe 每场景生成唯一串）——前缀同时是验收 grep -cF 'CIDX-PROBE' 材料"

requirements-completed: [OPS-03]

coverage:
  - id: D1
    description: "Task 1：phase09.mjs 全场景对真实二进制 PASS（S1a-c 启动校验矩阵三拒绝 exit 2 + stderr 含路径与类别 + 零内容探针；S2a-e 三通道 byte-identity + /x.css 404 + CSP connect-src 'self' 与六头同值；S3a-b gzip 预压 gunzip byte-identity/明文双态 + Vary 恒在 + Content-Type；S4a-c /api/attach 404 + WS Hello→Welcome + 凭据 401/200；S5a-b 0 字节 200 空 body + base-path /wesh/ 探针字节与 / 404；S6a-b TOML index 生效 + CLI 覆盖）+ RED 判别力证明（pre-09-04 二进制全 FAIL）"
    requirement: OPS-03
    verification:
      - kind: other
        ref: "time go build -o /tmp/wesh-uat/wesh ./cmd/wesh && node web/uat/phase09.mjs /tmp/wesh-uat/wesh（18/18 PASS、零 SKIP/FAIL 行、exit 0）"
        status: pass
      - kind: other
        ref: "验收 grep 组全过：assertOutputClean ==8（≥1）、sensitiveTokens ==9（≥2）、CIDX-PROBE ==3（≥1）+ node --check 语法通过 + pgrep -x wesh 零泄漏（进程组收口）"
        status: pass
      - kind: other
        ref: "RED 证明：git archive 49ed5b2（09-03 HEAD）构建 /tmp/wesh-uat/wesh-pred → node web/uat/phase09.mjs /tmp/wesh-uat/wesh-pred（S1a-c FAIL 类别/路径不匹配、S2-S5 场景异常 flag 未定义、S6 unknown keys(index)、SEC PASS、exit 1）"
        status: pass
    human_judgment: false
  - id: D2
    description: "Task 2：既有 UAT 回归四脚本全绿——phase03 18/18、phase05 28/28（+1 平台豁免 skip）、phase06-dom 40/40（+2 平台豁免 skip 含 D12b AT 栈）、phase07 34/34（+1 平台豁免 skip）——零 FAIL 行、各脚本 SEC 自净通过、exit 码全 0（--index 未配置时逐字节现状 + D-18 后 dom 面守恒的跨 plan 证据）"
    requirement: OPS-03
    verification:
      - kind: other
        ref: "node web/uat/phase03.mjs /tmp/wesh-uat/wesh && node web/uat/phase05.mjs /tmp/wesh-uat/wesh && node web/uat/phase06-dom.mjs /tmp/wesh-uat/wesh && node web/uat/phase07.mjs /tmp/wesh-uat/wesh（四脚本顺序全绿，各 exit 0）"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-30
status: complete
---

# Phase 9 Plan 05: OPS-03 协议层全链 UAT（phase09.mjs）Summary

**OPS-03 协议层全链 UAT 落地：web/uat/phase09.mjs（phase07.mjs 骨架件逐字复用——check/skip/assertOutputClean 三件套 + sensitiveTokens 闭包 + startWesh/spawnExpectExit 夹具 + redactArgs 脱敏）对真实二进制锁定 09-04 --index 全行为面——S1 三拒绝 exit 2 零内容探针、S2 三通道 byte-identity + 六安全头同值、S3 gzip 预压双态 + Vary 恒发、S4 认证面照旧 + WS 握手全链、S5 0 字节 + base-path、S6 TOML 通道与 CLI 覆盖，18/18 全绿；RED 判别力经 pre-09-04 二进制证伪（全 FAIL）+ 既有四脚本回归全绿（120 断言零 FAIL）**

## Performance

- **Duration:** 25 min（2026-08-30T15:30..15:55 +08:00）
- **Tasks:** 2
- **Files created:** 1（web/uat/phase09.mjs，528 行）
- **Commits:** 2（Task 1 test / Task 2 regression 空提交）+ 1 docs

## Accomplishments

- **S1 启动校验矩阵（D-07/D-08 启动面红线反断言）**：三独立 spawn 期望 exit 2 不启动——`--index` 不存在文件（`file does not exist` 类别）、目录（`not a regular file` 类别）、TOML `index-max-size = 64` + 129 字节探针文件（`exceeds index-max-size (64 bytes)` 含上限数值）；三行 stderr 均含路径与类别且零内容探针（`CIDX-PROBE-` 前缀缺席反断言——探针串只存 sensitiveTokens 闭包）
- **S2 给页三通道 byte-identity（D-05/D-06）**：GET `/`（空路径回落）、`/index.html`（显式路径）、`/s/{ro-token}/`（启动行解析取 ro 分享链接——装饰层在 sharePage 委托上游的全通道统一行为证明）三路径 body 与探针文件字节逐字节相等；GET `/x.css` → 404（相对资源契约语义）；S2e 安全头同源——CSP 含 `connect-src 'self'`（自定义页自实现终端可回连 /ws）且六安全头（CSP/X-Frame-Options/X-Content-Type-Options/Referrer-Policy/COOP/CORP）与内建页响应逐头同值（对照实例 spawn）
- **S3 gzip/Vary（契约行 4/6）**：Accept-Encoding 显式 gzip → 200 + `Content-Encoding: gzip` + `gunzipSync` 解压后 byte-identity；无 Accept-Encoding → 明文 byte-identity + 零 Content-Encoding；两态 `Vary: Accept-Encoding` 恒在 + `Content-Type: text/html; charset=utf-8`——经 node:http rawFetch 原始请求通道观测（undici fetch 双重自动化使明文态不可观测的适配，09-04 Go transport 同款语义的 JS 侧对偶）
- **S4 认证面照旧（D-05「WS 端点照旧暴露」+ 契约行 7）**：无认证模式 POST `/api/attach` → 404 探测信号照旧；WS `/ws` Hello→Welcome 全链（dialHello 复用 phase07 形态收窄无参版）；凭据模式带凭据 GET `/` → 200 自定义字节、无凭据 → 401（认证闸在装饰链外层；排序即解零 pacing——成功链路在前 401 负面对照排最后）
- **S5 0 字节与 base-path（D-07/契约行 6）**：`--index` 空文件 → GET `/` 200 空 body（拒绝列表不含空文件）；`--index` + `--base-path /wesh` → GET `/wesh/` 200 探针字节、GET `/` → 404（根无挂载）
- **S6 配置通道（D-07 TOML 同名键 + flag > 配置优先级链）**：TOML `index` 键（无 CLI，bind/port/command/index 全 TOML 配置驱动启动）→ GET `/` 给自定义字节；配置 index + CLI `--index` 另文件 → CLI 覆盖生效（body 为 CLI 文件字节且配置文件字节缺席双断言）
- **RED 判别力证明（task 级 tdd 兑现形态）**：`git archive 49ed5b2`（09-03 HEAD，09-04 之前）构建 pre-09-04 二进制——phase09.mjs 对其运行 S1a-c FAIL（exit 2 但类别/路径断言不匹配——错误文案为 flag 未定义而非 --index 校验）、S2-S5 场景异常（`flag provided but not defined: -index` 拒启）、S6 unknown keys(index) FAIL、SEC 自净在失败通道同样 PASS、exit 1；对当前二进制 18/18 PASS exit 0——脚本断言面非空转的机械证明
- **既有 UAT 回归（Task 2）**：phase03 18/18、phase05 28/28（+1 豁免）、phase06-dom 40/40（+2 豁免含 D12b AT 栈）、phase07 34/34（+1 豁免）——四脚本零 FAIL 行、各 SEC 自净通过、exit 码全 0；`--index` 未配置时伺服/认证/协议面逐字节现状（零值兜底纪律回归证据）+ D-18 后 D1-D13 dom 面守恒跨 plan 二次确认

## Task Commits

1. **Task 1: phase09.mjs——OPS-03 协议层全链断言（S1-S6 十七场景 + SEC 自净）** - `1649639` (test)
2. **Task 2: 既有 UAT 回归四脚本全绿（回归里程碑空提交）** - `95f06f0` (test)

**Plan metadata:** 见末尾 docs 提交（docs(09-05): complete ...）

## Files Created/Modified

- `web/uat/phase09.mjs`（新建，528 行）——脚本头覆盖清单（S1-S6 对应 09-04 行为面 + D-05..D-08）与红线声明（phase06.mjs:11-13/phase07.mjs:14-17 纪律逐字沿用）；三件套与夹具逐字复用 phase07.mjs 形态（check/skip（保留零调用）/assertOutputClean/sensitiveTokens/startWesh（TCP 场景收窄）/spawnExpectExit/redactArgs/启动行解析）；rawFetch（node:http 原始请求 + 5s 超时护栏）；dialHello（phase07 形态收窄无参版）；writeProbe 探针夹具（唯一探针串生成 + sensitiveTokens 登记）；S1-S6 十七场景 + SEC 自净 + PHASE09_ONLY 调试过滤

## Decisions Made

- **rawFetch 取 node:http 原始请求通道**（给页断言通道选型）：undici fetch 自动加 `accept-encoding: gzip, deflate` 且按 Content-Encoding 透传解压——S3 明文态与 Content-Encoding 头面结构性不可观测；09-04 Go 层以显式编码直证同款问题，JS 侧以不注入头/不透明解压的原始请求一次解决，gzip/明文/头面三断言面全可观测
- **task 级 tdd RED 形态**：被测实现属先序 plan（09-04 已落地），failing-first 提交结构性不可达（交付物即测试、无独立实现步）——RED 以 pre-09-04 二进制（git archive 49ed5b2 构建）证判别力，GREEN 当前二进制全绿；plan action 单 `test(09-05)` 提交字面保持（09-02「plan type=execute 不适用 plan 级 RED/GREEN 门序列」先例沿用）
- **Task 2 --allow-empty 提交**：verification-only 任务零文件改动，plan 指定提交语——空提交记录回归里程碑，保持 per-task 原子提交协议
- **S2e 六头同值扩展**：must_haves「与内建页响应同值」的完整兑现（plan behavior 字面仅 CSP connect-src）——对照实例（无 --index spawn）逐头比对，断言面强于 plan 字面且在 scope 内
- **S1 探针反断言取前缀**：`PROBE_PREFIX`（CIDX-PROBE-）缺席断言覆盖任意探针内容（每场景唯一串），同时是验收 grep 材料单写口

## Deviations from Plan

### Auto-fixed Issues

**1. [TDD 形态] task 级 tdd 的 RED 以 pre-09-04 二进制证判别力（非 failing-first 提交）**
- **Found during:** Task 1 RED 阶段
- **Issue:** plan 标记 `tdd="true"` 但交付物即测试脚本、被测实现已在 09-04 落地——failing-first 提交与 RED→GREEN 双提交结构不可达（无实现步）
- **Fix:** `git archive 49ed5b2`（09-03 HEAD）构建 pre-09-04 二进制，新脚本对其运行全 FAIL（S1 类别不匹配/S2-S5 拒启/S6 unknown key）证判别力后对当前二进制全绿；按 plan action 单 test 提交（09-02/09-03 先例——fail-fast 规则调查后确认设计内性质）
- **Verification:** RED exit 1（四场景异常 + 三断言 FAIL）/ GREEN 18/18 exit 0
- **Committed in:** `1649639`

**2. [Rule 3 - Blocking] Task 2 提交取 --allow-empty**
- **Found during:** Task 2 提交步
- **Issue:** plan 指定「提交：test(09-05): regression pass over existing UAT suites」但 verification-only 任务零文件改动——`git commit` 无 --allow-empty 必失败，per-task 原子提交协议无法保持
- **Fix:** `git commit --allow-empty` 记录回归里程碑（四脚本 PASS 计数与零 FAIL 证据入提交信息与 SUMMARY）
- **Verification:** `git log --oneline -2` 两任务提交齐备
- **Committed in:** `95f06f0`

---

**Total deviations:** 2（1 TDD 形态裁决、1 Rule 3 提交通道必要）
**Impact on plan:** 交付物与 must_haves 逐字一致；无范围蔓延。

## Issues Encountered

- 首次冒烟（执行通道，非交付物问题）：S1c 探针文件初版 59 字节 < 64 上限未触发超限拒绝——writeProbe 定稿为 ~129 字节模板（> 64 上限恒触发，触发形态不再依赖字节计数巧合）

## Known Stubs

None —— 无占位/硬编码空值/TODO；phase09.mjs 十七场景 + SEC 自净全实证，且 RED 证明断言面非空转。skip helper 按三件套形态保留但零调用（本 phase 无平台豁免面——协议层全链 headless 可断言）。

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 09-09 README --index 节素材：每项承诺（整页替换/双通道统一/四拒绝 exit 2 零内容/16MiB 硬顶 + index-max-size 纯配置键例外/自包含单 HTML 推论）均有 phase09.mjs 可执行断言锚点——文档即被测物纪律的断言面齐备
- OPS-03 双层验证闭环：Go 层行为锁（09-04 TestCustomIndex/TestLoadCustomIndex 单元级）+ 协议层全链 UAT（phase09.mjs 真实二进制级）——req 面覆盖完备
- 既有 UAT 回归通道（phase03/05/06-dom/07 四脚本）可作为 09-09/09-10 文档与收尾变更的零漂移门禁

## Self-Check: PASSED

- Files: web/uat/phase09.mjs / .planning/phases/09-release-polish/09-05-SUMMARY.md 均 FOUND
- Commits: 1649639（Task 1）/ 95f06f0（Task 2）均 FOUND

---
*Phase: 09-release-polish*
*Completed: 2026-08-30*
