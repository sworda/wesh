---
phase: 09-release-polish
plan: 08
subsystem: testing
tags: [caddy, reverse-proxy, websocket-upgrade, host-header, idle-timeout, dual-machine, playwright, uat]

requires:
  - phase: 07-deployment
    provides: "G-07-2 nginx 双机全链实证套路（scp 上传 + ssh BatchMode ctl 分派/Playwright 断言组/安全组放通先例 phase07-a2）——本 plan 的 Caddy 版复用源"
  - phase: 03-认证与传输安全
    provides: "Origin 同源校验语义（SEC-04）——Caddy reverse_proxy 默认 Host 透传能否天然过闸的受力面；Basic 凭据 401→200 穿透矩阵先例"
provides:
  - "Caddy 实证锚点（09-09 README Caddy 节素材）：v2.11.4 / 首证 2026-08-29 / 修复复验+双机全链 2026-08-30；三行为面结论——reverse_proxy 默认原样透传 Host（Origin 同源校验天然过，零 Host 配置行）、WS upgrade 内建自动、hijack 后无默认 WS idle 超时（65s 空闲存活双机实测）"
  - "Caddyfile LAN 监听形态实证：站点地址须裸 :PORT（绑定全网卡 + 匹配任意 Host）——http://0.0.0.0:PORT 系字面 Host 匹配（仅 Host: 0.0.0.0 命中，真实主机名请求落空走兜底空 200），与 nginx 监听语义相反（28ae2f2 勘误）"
  - "双机载具入库：web/uat/pw/phase09-caddy-ctl.sh（Linux 侧 setup/probe/teardown case 分派 + Caddy 二进制幂等部署 + CADDY_UP 就绪串）+ phase09-caddy-pw.mjs（Windows 侧 t1-t4 断言组）+ web/uat/pw/README.md 登记行"
affects: [09-09（README Caddy 节「实证日期/版本」标注与站点地址写法）, ship]

actuals:
  tokens: 2900
  tasks: 3
  commits: 3

tech-stack:
  added: [Caddy v2.11.4（GitHub release 官方静态二进制——仅实证环境 /tmp/wesh-uat/caddy/，不入仓）]
  patterns:
    - "反代实证探针目标必须用真实主机名/LAN IP 而非 curl 0.0.0.0 自身——探针 Host 字面命中使「外部 Host 被服务」断言假绿（本 plan 勘误根因，09-09 配方注释素材）"
    - "Caddy 站点地址与 nginx listen 语义相反：0.0.0.0:PORT 是字面 Host 匹配非绑定语义——LAN 监听配方写裸 :PORT"
    - "双机确认门（gate=blocking）收口形态：用户在 Windows 工作站实跑 pw.mjs 全链 4/4 → 裁决 PASSED → 续作 executor 落 SUMMARY；真实浏览器半侧是静态门（bash -n/node --check/grep）覆盖不到的行为面最终防线"

key-files:
  created:
    - web/uat/pw/phase09-caddy-ctl.sh
    - web/uat/pw/phase09-caddy-pw.mjs
  modified:
    - web/uat/pw/README.md

key-decisions:
  - "Caddyfile LAN 监听站点地址 = 裸 :PORT（write_caddyfile 勘误形态，28ae2f2）——http://0.0.0.0:10014 在 Caddy 是字面 Host 匹配（仅 Host: 0.0.0.0 命中），真实主机名请求落空走 Caddy 兜底空 200；裸 :PORT = 绑定全网卡 + 匹配任意 Host，与 nginx 的 0.0.0.0 监听语义相反——两平台配方互抄必错（Pitfall 6 第二实证点）"
  - "三行为面实证锁定（README Caddy 节标注「实证 2026-08-30，Caddy v2.11.4」的依据）：reverse_proxy 默认原样透传 Host——wesh Origin 同源校验天然通过，不配任何 Host 行（与 nginx 须 proxy_set_header Host $http_host 相反）；WS upgrade 内建自动；hijack 后无默认 WS idle 超时（65s 空闲存活，仅 wesh 5s ping 应用层流量）"
  - "Task 1 首证「外部 Host 照常被服务」结论勘误为假绿：proto-verify 就绪探针恰好 curl 0.0.0.0 自身（Host 字面命中站点地址），LAN IP 401/200 矩阵复验方为有效证据——外部 Host 行为面断言必须以真实主机名/LAN IP 为请求目标"
  - "pw t1 认证形态与 phase07-a2 的关键差异：Caddy 无认证层、wesh /ws 不收 HTTP 级认证——走 authedContext 预置 Authorization 避开 wesh 401→recordFail→429 节流链（httpCredentials 的 Chromium 即时重试撞 1s 窗口，lib/browser.mjs 头注释纪律）；裸 context 构造 t1 负面对照 + 401 后 sleep 1.2s 消解节流窗（05-09/07-07 pacing 纪律）"

patterns-established:
  - "反代实证断言面纪律：request 层 401/200 矩阵以 LAN IP 为目标 + WS 行为面经真实浏览器全链——自 curl 探针只能证监听不能证 Host 匹配语义；双机确认门是静态门覆盖不到的行为面最终防线（本 plan 假绿即由 Windows 真实浏览器半侧捕获）"

requirements-completed: [OPS-10]

coverage:
  - id: D1
    description: "Linux 协议层实证（/tmp/wesh-uat/ 一次性载具，不入仓）：Caddy v2.11.4 部署 + 七断言 a1/a2/b1/b2/b3/c1/d1——无凭据 401→带凭据 200 穿透、ticket 签发、WS 经 Caddy 升级 Hello→Welcome（Host 透传 Origin 同源天然过）、echo MK:42 全链、65s 空闲存活（无默认 WS idle 超时）、SIGKILL 两进程组后 18080/17682 端口归零"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "node /tmp/wesh-uat/phase09-caddy-proto-verify.mjs（2026-08-29 首证 7/7 全过退出 0；2026-08-30 修复后复验 7/7）"
        status: pass
      - kind: other
        ref: "LAN IP 401/200 矩阵复验（真实主机名目标 http://9.134.229.124:10014/，2026-08-30 修复后）"
        status: pass
    human_judgment: false
  - id: D2
    description: "双机载具入库：phase09-caddy-ctl.sh（setup/probe/teardown case 分派 + fuser -k 预清理 + Caddy 二进制幂等部署 + CADDY_UP 就绪串 + 裸 :10014 站点地址 Caddyfile 内嵌件含勘误注释）+ phase09-caddy-pw.mjs（t1-t4 断言组：401/200 矩阵/浏览器 echo 全链/idle 存活/端口归零）+ web/uat/pw/README.md 登记行"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "bash -n web/uat/pw/phase09-caddy-ctl.sh && node --check web/uat/pw/phase09-caddy-pw.mjs && grep -c CADDY_UP ==1 && grep -c phase09-caddy-ctl ==1 && grep -c 'proxy_set_header Host' ==0（plan Task 2 verify 块全过）"
        status: pass
    human_judgment: false
  - id: D3
    description: "Windows 工作站双机全链确认（Task 3 gate=blocking 确认门）：用户实跑 node phase09-caddy-pw.mjs——t1 无凭据 401/带凭据 200 终端页；t2 浏览器终端就绪 + echo 标记回读（WS 经 Caddy 升级全链）；t3 65s 空闲无断连状态面板 + idle 后 echo 仍可达；t4 teardown 后 10014/17682 端口归零"
    requirement: OPS-10
    verification:
      - kind: automated_ui
        ref: "node web/uat/pw/phase09-caddy-pw.mjs @ Windows 工作站（用户执行，2026-08-30 修复后复跑 4/4 PASS：t1 got=401→200 html=true；t2 CDYWS_1E2HY7:42 回读；t3 hidden=true + CDYIDLE_FKVF 可达；t4 caddy=0 wesh=0）"
        status: pass
    human_judgment: true
    rationale: "双机拓扑硬约束（CODEBUDDY.md）：Playwright 仅存在于 Windows 工作站，Linux 开发机禁装浏览器——确认门 gate=blocking 由用户实跑并裁决 PASSED（4/4），executor 无法自动运行浏览器半侧"

duration: 55min
completed: 2026-08-30
status: complete
---

# Phase 9 Plan 08: Caddy 反代配方实证（D-15） Summary

**Caddy v2.11.4 反代配方双机实证闭合：Linux 协议层七断言（401→200 穿透/ticket 签发/WS 经 Caddy 升级 Hello→Welcome/echo 全链/65s 空闲存活/端口归零）+ Windows Playwright 双机全链 4/4；期间捕获并修复 Caddyfile 站点地址字面 Host 匹配假绿（`http://0.0.0.0:PORT` → 裸 `:PORT`，28ae2f2）——Host 默认透传/WS upgrade 内建/无默认 idle 超时三行为面锁定，README Caddy 节（09-09）「实证 2026-08-30，Caddy v2.11.4」锚点就绪**

## Performance

- **Duration:** 55 min（净工作；Task 3 确认门隔夜等待不计入）
- **Started:** 2026-08-29T16:00Z
- **Completed:** 2026-08-30T03:49Z
- **Tasks:** 3
- **Files modified:** 3（2 新建 + 1 登记行）

## 实证记录（09-09 README Caddy 节素材）

- **Caddy 版本**：`v2.11.4 h1:XKxkMTgNSizEvKG6QHue6cAsFOteU2qA61w2tKkCWi0=`（GitHub release 官方静态二进制直装至 /tmp/wesh-uat/caddy/——不入仓；CODEBUDDY 禁 apt 纪律不涉服务端软件，T-09-SC 供应链豁免面按 RESEARCH Package Legitimacy Audit 既定）
- **实证日期**：首证 2026-08-29（Linux 协议层 7/7）；修复复验 + 双机全链 2026-08-30（LAN IP 401/200 矩阵 + proto-verify 7/7 + Windows 双机 4/4）——README 标注取 2026-08-30（结论成立的最终形态日期）
- **三行为面结论**（与 nginx G-07-2 配方对照，Pitfall 6「两平台默认语义相反、配方互抄必错」的双侧实证闭合）：
  1. **reverse_proxy 默认原样透传 Host** → wesh Origin 同源校验天然通过，Caddyfile 零 Host 配置行（nginx 须显式 `proxy_set_header Host $http_host` 才等效——b2「不配 Host 行的实证点」+ t2 浏览器全链双证）
  2. **WS upgrade 内建自动处理**——b1/b2（协议层 Hello→Welcome→INPUT/OUTPUT）+ t2（真实 Chromium echo 标记回读 CDYWS_1E2HY7:42）双证，零 upgrade 配置行
  3. **hijack 后无默认 WS idle 超时**——c1/t3 双证：65s 空闲期间（仅 wesh 5s ping 应用层流量）连接存活、无断连状态面板（hidden=true）、idle 后 echo 仍可达（CDYIDLE_FKVF）；README「空闲超时与 ping 间隔关系」表素材：Caddy 反代形态下 CORE-06 默认 5s ping 无 idle 超时约束需匹配
- **站点地址写法（勘误后定稿）**：LAN 监听站点地址 = 裸 `:10014`（绑定全网卡 + 匹配任意 Host）；`http://0.0.0.0:10014` 系字面 Host 匹配——与 nginx 的 0.0.0.0 监听语义相反（详见 Deviations #1，README 配方须以裸 :PORT 形态落文）

## Accomplishments

- **Task 1 Linux 协议层实证七断言全绿**（/tmp 一次性载具，仓库零改动）：Caddyfile `{ admin off } + http://127.0.0.1:18080 { reverse_proxy 127.0.0.1:17682 }` 单指令形态 + detached spawn wesh/caddy 双进程组 + SIGKILL 收口——a1 无凭据 GET / 经 Caddy → 401（challenge 穿透反代）；a2 带凭据 → 200 终端页；b1 POST /api/attach → 200 签发 ticket；b2 WS 经 Caddy 升级 + Hello→Welcome（Host 透传 Origin 同源——不配 Host 行的实证点）；b3 echo 全链 INPUT 标记回读 OUTPUT 含 MK:42；c1 空闲 65s 后 echo 仍可达（无默认 WS idle 超时）；d1 SIGKILL 后 18080/17682 端口归零
- **Task 2 双机载具入库**（072a585）：`phase09-caddy-ctl.sh`（phase07-a2-ctl.sh 骨架复用——fuser -k 预清理/Caddy 二进制幂等部署/nohup 起 wesh+caddy/ss -ltn 双端口探活/CADDY_UP 就绪串/probe 端口计数回读/teardown 归零）+ `phase09-caddy-pw.mjs`（phase07-a2-pw.mjs 套路复用——scp 上传/ssh BatchMode ctl 分派/t1-t4 断言组/红线头注释逐字沿用）+ README 登记行；静态门全过（bash -n/node --check/CADDY_UP 唯一/引用一致/零 Host 配置行）
- **Task 3 Windows 双机全链 4/4 PASS（用户裁决）**：真实 Chromium 经 LAN `http://9.134.229.124:10014` 反代——t1 401/200 矩阵（got=401→got=200 html=true）；t2 终端就绪 + echo 标记回读 CDYWS_1E2HY7:42（WS 经 Caddy 升级全链）；t3 65s 空闲无断连面板（hidden=true）+ idle 后 echo 仍可达 CDYIDLE_FKVF；t4 teardown 后端口归零（caddy=0 wesh=0）
- **假绿捕获与修复（28ae2f2）**：Windows 首跑 0/2 暴露 Task 1「外部 Host 照常被服务」结论系假绿（proto-verify 探针 curl 0.0.0.0 自身 Host 字面命中）；根因 = Caddy 站点地址字面 Host 匹配语义；修复 = ctl `write_caddyfile` 改裸 `:10014` + pw t1b detail `r200.status()` 笔误修正；修复后三层复验全绿（LAN IP 矩阵 + proto 7/7 + Windows 4/4）——双机确认门（gate=blocking）的存在价值实证
- **零残留收口**：实证件全在 /tmp/wesh-uat/（caddy 二进制/Caddyfile/载具脚本不入仓）；t4/d1 端口归零双证；仓库 git status 干净

## Task Commits

Each task was committed atomically:

1. **Task 1: Caddy 二进制部署 + Linux 协议层实证（七断言）** — 无提交（实证在 /tmp/wesh-uat/，证据落本 SUMMARY；入库载具随 Task 2）
2. **Task 2: 双机载具入库（ctl.sh + pw.mjs + README 登记）** - `072a585` (test)
3. **Task 3: 确认门——Windows 双机全链执行（用户裁决 PASSED 4/4；含假绿修复）** - `28ae2f2` (fix)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP/WINDOWS）

## Files Created/Modified

- `web/uat/pw/phase09-caddy-ctl.sh` — Linux 侧双机控制脚本（Caddy v2.11.4 幂等部署 + 裸 :10014 站点地址 Caddyfile 内嵌件含勘误注释 + CADDY_UP 就绪协议 + probe/teardown）
- `web/uat/pw/phase09-caddy-pw.mjs` — Windows 侧 Playwright 双机全链断言组（t1 401/200 矩阵/t2 浏览器 echo 全链/t3 65s idle 存活 + 面板 hidden/t4 端口归零）
- `web/uat/pw/README.md` — 载具登记行（双机拓扑与断言面一行摘要）

## Decisions Made

- **Caddyfile 站点地址裸 :PORT 定稿**（详见 Deviations #1）——`http://0.0.0.0:PORT` 在 Caddy 是字面 Host 匹配非绑定语义，与 nginx 相反；09-09 README Caddy 节配方必须以裸 :PORT 形态落文并把该差异写进注释（第二个「配方互抄必错」实证点，与 Host 透传差异并列）
- **三行为面结论取「修复形态」为实证基准**——Host 默认透传/WS upgrade 内建/无默认 idle 超时三结论均在裸 :PORT 修复形态下复验成立（首证形态的「外部 Host」面系假绿已作废），D-15 兜底通道承诺全额兑现
- **pw t1 认证形态选 authedContext 预置头**——Caddy 无认证层（与 a2 的 nginx auth_basic 不同），httpCredentials 的 Chromium 即时重试会撞 wesh 401→recordFail→429 节流 1s 窗口；裸 context 负面对照 + sleep 1.2s pacing 消解（05-09/07-07 纪律沿用）
- **proto-verify 就绪探针假绿教训升格为纪律**——探活目标 0.0.0.0 使 Host 字面命中站点地址，「外部 Host 被服务」断言失去受力面；外部 Host 行为面断言必须以真实主机名/LAN IP 为请求目标（patterns-established 登记，09-09 配方注释素材）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Caddyfile 站点地址 `http://0.0.0.0:10014` 系字面 Host 匹配——Task 1「外部 Host 照常被服务」结论假绿，Windows 首跑 0/2**
- **Found during:** Task 3（Windows 工作站首跑 t1：无凭据 GET / got=200 空 body——真实主机名请求未命中任何站点，落 Caddy 兜底空 200）
- **Issue:** Caddy 站点地址 `http://0.0.0.0:PORT` 不是「绑定全网卡」的监听语义（nginx 语义），而是**字面 Host 匹配**——仅 Host: 0.0.0.0 的请求命中该站点，真实主机名/LAN IP 请求落空走 Caddy 兜底空 200。Task 1 原结论「外部 Host 头照常被服务」系假绿：proto-verify 就绪探针恰好 curl `0.0.0.0` 自身（Host 字面命中）；Task 2 静态门（bash -n/node --check/grep）只锁「零 Host 配置行」，锁不到站点地址绑定语义
- **Fix:** ctl `write_caddyfile` 站点地址改裸 `:10014`（绑定全网卡 + 匹配任意 Host）+ 注释登记实证勘误说明（nginx 语义相反警示）；`http://127.0.0.1:18080`（Task 1 loopback 形态）不受影响——curl 127.0.0.1 自身 Host 字面命中，loopback 语义恒成立
- **Files modified:** web/uat/pw/phase09-caddy-ctl.sh
- **Verification:** 修复后三层复验全绿——LAN IP 401/200 矩阵正确（真实主机名目标 http://9.134.229.124:10014/）+ proto-verify 七断言 7/7 + Windows 双机复跑 4/4（t1 got=401→200 html=true）
- **Committed in:** `28ae2f2`（Task 3 修复提交）

**2. [Rule 1 - Bug] pw t1b detail `r200.status` 缺调用括号——失败时打印函数源码而非状态码**
- **Found during:** Task 3（Windows 首跑失败 detail 输出形态核读——Playwright APIResponse.status 是方法，模板串直接引用打印 function source）
- **Issue:** 失败取证通道失真：detail 显示函数源码而非 `got=200` 形态，妨碍首跑快速定位（首跑根因定位实际靠 t1 负面对照的 got=200 与空 body 形态）
- **Fix:** `r200.status` → `r200.status()`
- **Files modified:** web/uat/pw/phase09-caddy-pw.mjs
- **Verification:** Windows 复跑 t1 detail `got=401`/`got=200 html=true` 形态正确
- **Committed in:** `28ae2f2`（与 Deviation #1 同提交）

---

**Total deviations:** 2 auto-fixed（全 Rule 1——1 载具行为面修复 + 1 取证 detail 笔误；无 Rule 4 架构变更、无认证门、零包安装——Caddy 为 GitHub release 制品按 plan 既定通道直装）
**Impact on plan:** D-15 三行为面结论不受影响（Host 默认透传/WS upgrade 内建/无默认 idle 超时均在修复形态下复验成立）；新增第四行为面结论（站点地址裸 :PORT 写法 + 0.0.0.0 字面 Host 匹配陷阱）进 09-09 README 配方注释——交付物语义增强而非 scope creep。T-09-08a 缓解（「Caddyfile 断言零 Host 配置行」机械检查）被证明必要但不充分：Host 行缺失防住了 nginx 配方照抄，站点地址形态仍需真实浏览器半侧把关——双机确认门价值实证。

## Issues Encountered

- **Task 2 静态门的覆盖边界**：bash -n/node --check/grep 只能锁语法与配置行字面，锁不到站点地址的 Host 匹配语义——静态绿 + 协议层探针绿（探针目标自身字面命中）叠加仍产出假绿结论；最终由 Windows 真实浏览器半侧（用户实跑）捕获。教训：探针目标与断言语义必须正交（外部 Host 断言不能用字面命中目标的探针自证）
- **修复后复验的证据分层**：loopback proto-verify（Host 字面命中恒过，证协议行为面）+ LAN IP 矩阵（真实主机名目标，证站点地址匹配面）+ Windows 浏览器全链（端到端）——三层各证一面，单层不足

## Authentication Gates

None——纯本机 + LAN 实证（GitHub release 下载 + ssh BatchMode 既有通道），无认证门。Task 3 确认门为人工裁决门（gate=blocking），非认证门。

## Known Stubs

无——双载具全部为经真实执行的门禁件（Linux 侧 7/7 + Windows 侧 4/4）；无 TODO/FIXME/占位值/空数据流。

## User Setup Required

None - no external service configuration required.（Caddy 仅实证环境 /tmp 直装不入仓；09-09 README Caddy 节将提供生产配方。）

## Next Phase Readiness

- **09-09 README Caddy 节素材齐备且与实测同源**：「实证 2026-08-30，Caddy v2.11.4」标注 + 三行为面结论（Host 默认透传零 Host 行/WS upgrade 内建/无默认 idle 超时）+ 站点地址裸 :PORT 写法与 0.0.0.0 字面 Host 匹配陷阱注释 + 空闲超时与 ping 间隔关系（CORE-06 默认 5s ping 下无 idle 超时约束需匹配）
- **D-15 反代配方验证深度闭合**：nginx（G-07-2）与 Caddy（本 plan）均已双机实证；Cloudflare 按 D-15 既定「按官方文档写 + 标注未实测」处理（SaaS 无本机条件，风险接受）
- **ROADMAP SC3 部署文档面**：Docker/systemd（09-07）+ Caddy（09-08）实证半侧全闭，09-09 落文即收口
- **双机载具复用价值**：phase09-caddy-ctl/pw 可重跑回归反代行为面（Caddy 版本升级复验通道）

## Self-Check: PASSED

- [x] 前置提交存在：`072a585`（test(09-08)）与 `28ae2f2`（fix(09-08)）均在 git log（gsd/phase-09-release-polish）
- [x] 交付文件存在：web/uat/pw/phase09-caddy-ctl.sh（80 行）、web/uat/pw/phase09-caddy-pw.mjs（108 行）、web/uat/pw/README.md 登记行（:55）
- [x] 实证环境在位：/tmp/wesh-uat/caddy/caddy version → v2.11.4（首证与复验同二进制）

---
*Phase: 09-release-polish*
*Completed: 2026-08-30*
