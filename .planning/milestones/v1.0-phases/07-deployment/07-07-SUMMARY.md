---
phase: 07-deployment
plan: 07
subsystem: testing
tags: [uat, protocol-layer, zero-dependency, tcp-unix-relay, config-file, unix-socket, base-path, auth-header, xff, stop-signal, privilege-drop, graceful-shutdown-1001, open-browser, assert-output-clean]

requires:
  - phase: 07-deployment/07-01
    provides: --base-path 全链（307/StripPrefix/前端相对 URL）+ shareURL 拼串单一事实源（S3 场景与 S8b argv 断言的行为来源）
  - phase: 07-deployment/07-02
    provides: --socket 三 flag + listenSocket 序列 + unix:// 启动打印退化（S2 场景行为来源）
  - phase: 07-deployment/07-03
    provides: proxy.go 提取层 + logEvent remote_user 第四字段 + XFF 信任闸（S4 场景行为来源）
  - phase: 07-deployment/07-04
    provides: StartOptions/Credential 降权 + stopChildLocked stop-signal 序列 + trap 夹具 SIG_IGN 机理（S5/S6 场景行为来源）
  - phase: 07-deployment/07-05
    provides: Shutdown 1001 广播 + signal.NotifyContext 接线 + openBrowser 三形态（S7/S8 场景行为来源）
  - phase: 07-deployment/07-06
    provides: TOML 配置两阶段合并 + 值剥离红线 + D-07 权限警告（S1 场景行为来源）
  - phase: 06-session-lifecycle/06-06
    provides: phase06.mjs harness 五件逐结构形态 + assertOutputClean 运行时自净 + WR-02 异常通道收口（本 plan harness 复制源）
provides:
  - "web/uat/phase07.mjs【新】（779 行）：phase 7 协议层 UAT 单脚本八场景 33 断言 + 1 豁免——S1 配置合并与优先级七断言 / S2 unix socket relay 五断言 / S3 base-path 交叉八断言 / S4 auth-header+XFF 四断言 / S5 stop-signal 宽限两断言 / S6 降权 self 两断言 / S7 1001 序列两断言 / S8 --open 两断言+1豁免"
  - "TCP↔unix relay 夹具（net.createServer 管道转发，07-RESEARCH Pattern 7 探针形态转正）：Node 原生 WebSocket/fetch 不能直连 unix socket 的 15 行解法，复用全部既有 WS 断言机制"
  - "assertOutputClean 运行时自净：emittedDetails 全遍历 + S1 TOML 凭据探针与 share token 同口径 sensitiveTokens（红线 T-07-07a mitigate 的可执行形态）"
  - "九脚本零回归实证：phase02/03/04/05/05-dims/06 协议层 + 04-dom/05-dom/06-dom jsdom 各自退出 0（新代码下既有 UAT 资产零损失）"
affects: [07-08（README 特性节与 flagged_assumptions 人工复核可直接引用本 UAT 行为矩阵）, verify-work, phase-08]

actuals:
  tokens: 11142
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "TCP↔unix relay 夹具：net.createServer(c => c.pipe(net.createConnection(sock)).pipe(c)) + listen(0) 随机端口——unix socket 场景复用全部既有 WS 断言机制零改动（07-RESEARCH 探针实证转正）"
    - "startWesh 三选项扩展形态：defaultListen:false（配置驱动 bind/port 与 --socket 场景——D-08 互斥使 CLI bind/port 不能同给）/ unix:true（unix:// 启动行解析分支）/ env（--open 场景 DISPLAY 增删与 PATH 前置）"
    - "undici 头值 latin1 编码实证与 UTF-8 控制字符线形构造：JS 'ali\\u00C2\\u0085ce' 双码点上线字节 = 0xC2 0x85 = UTF-8 客户端发送 'ali\\u0085ce' 的等价线形——Go 侧复现 sanitize 剥离路径的唯一零依赖形态"
    - "attach+close 事件行触发形态：--exit-when-empty 实例的 exit_when_empty logEvent 携最后离开者 remote/remote_user——auth-header/XFF 审计归因的确定性即时断言通道"
    - "排序即解零 pacing：S1 的 401 负面对照排成功链路之后（fail#1 +1s 节流窗口无后续消费者，05-09 登记纪律第二次沿用）"

key-files:
  created:
    - web/uat/phase07.mjs
  modified: []

key-decisions:
  - "S4c 控制字符探针取 UTF-8 线形等价物构造（本机三探针实证）：plan 字面 'alice\\r\\nFAKE' 的 C0 \\r\\n 在真实 HTTP 头值不可传输（客户端栈即拒）；朴素 JS 'ali\\u0085ce' 经 undici latin1 上线为单字节 0x85（Go 侧非法 UTF-8 → U+FFFD，复现不了剥离路径，S4c 首跑实测命中）——'ali\\u00C2\\u0085ce' 上线 = 0xC2 0x85，Go 解码得 U+0085 NEL 按 D-19 剥离 → 'alice'"
  - "S4 事件行触发形态 = --exit-when-empty 的 exit_when_empty logEvent：attach+close 链路唯一确定性即时事件（正常 detach 无独立事件行），携 c.remote/c.remoteUser（Attach 入口提取值），进程随后 HUP 收口 255 顺带锁定"
  - "S5a 时序断言加 900ms 下界（plan 字面只有 <10s 上界）：下界证 1s 宽限真实经过（TERM 未被忽略时 elapsed 远小于 900ms），实测 1001-1002ms 窗口稳定"
  - "S1 断言排序：A attach → B 503 → 401 负面对照排最后（fail#1 +1s 节流窗口无后续消费者，零 sleep 零 pacing）"

patterns-established:
  - "UAT 配置文件场景形态：t 临时目录落 TOML（chmod 600 免 D-07 警告噪音；警告场景显式 chmod 644）+ spawn --config defaultListen:false + 凭据值探针入 sensitiveTokens 同口径 + spawnExpectExit 拒绝路径（exit 2 + 值剥离双断言）"
  - "fake xdg-open 测试形态：临时目录可执行脚本 printf '%s\\n' \"$@\" 落盘 + PATH 前置 + DISPLAY=:99 + 轮询记录文件 5s + 闭包内 URL 全等比对（detail 只报布尔——token 红线）"
  - "场景过滤调试纪律：PHASE07_ONLY=S1,S3 环境变量过滤（仅调试用，提交形态恒全场景开启）"

requirements-completed: [OPS-01, OPS-02, OPS-04, OPS-05, OPS-09, OPS-11, SEC-07]

coverage:
  - id: D1
    description: "S1 配置文件合并与优先级：TOML 铺底（凭据 401 + max-clients=1 503）生效、CLI --max-clients 2 覆盖两客户端并存、未知键/不存在文件 exit 2 且值剥离、chmod 644 权限警告不含值"
    requirement: OPS-09
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S1a-S1g 七断言全 PASS）"
        status: pass
    human_judgment: false
  - id: D2
    description: "S2 unix socket 全链：D-10 残留垃圾文件清理 spawn 成功、unix:// 启动行、socket 文件 0660、stdout 无 http:// 退化单行、relay 转发 dialHello + echo 全链"
    requirement: OPS-01
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S2a-S2e 五断言全 PASS）"
        status: pass
    human_judgment: false
  - id: D3
    description: "S3 base-path 交叉：/wesh 307 Location 含前缀、/wesh/ 200、/ 404、WS 双路径隔离（裸 /ws 404、/wesh/ws 101）、share 行含 /wesh/s/ 前缀、share token×base-path 全链 attach（rw）"
    requirement: OPS-02
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S3a-S3h 八断言全 PASS）"
        status: pass
    human_judgment: false
  - id: D4
    description: "S4 auth-header/XFF：attach+close 事件行 remote=XFF 链首 198.51.100.7 + remote_user=alice、NEL 线形探针 sanitize（alice 保留 + 控制字符零泄漏）、对照组现状（remote=127.0.0.1 系、无 remote_user 键、XFF 忽略）"
    requirement: SEC-07
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S4a-S4d 四断言全 PASS）"
        status: pass
    human_judgment: false
  - id: D5
    description: "S5 stop-signal 宽限：trap 忽略 TERM 后 900ms..10s 窗口补 KILL 退出 255（实测 1001-1002ms）；对照 TERM 无 timeout 即终结 255"
    requirement: OPS-04
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S5a/S5b 两断言全 PASS）"
        status: pass
    human_judgment: false
  - id: D6
    description: "S6 降权 self 免 root：echo 回读 id -u == self uid、HOME/USER 与当前用户 passwd 条目一致（D-25 身份环境改写）"
    requirement: OPS-05
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S6a/S6b 两断言全 PASS）"
        status: pass
    human_judgment: false
  - id: D7
    description: "S7 1001 关停序列：dialHello 完成后 SIGTERM → 客户端收 close 1001 + reason server_shutting_down → wesh 15s 护栏内退出 255"
    requirement: OPS-11
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S7a/S7b 两断言全 PASS）"
        status: pass
    human_judgment: false
  - id: D8
    description: "S8 --open：headless 提示跳过不阻断 + 服务 200；fake xdg-open argv == 启动打印 rw 分享 URL（单一事实源两消费点一致）；真实弹浏览器 skip+reason 平台豁免"
    requirement: OPS-11
    verification:
      - kind: e2e
        ref: "node web/uat/phase07.mjs（S8a/S8b PASS + S8c skipped 豁免）"
        status: pass
    human_judgment: false
  - id: D9
    description: "真实弹浏览器拉起与标签页观感（Windows 工作站人工层）"
    requirement: OPS-11
    verification: []
    human_judgment: true
    rationale: "headless 硬约束——真实 GUI 属 Windows 工作站人工层（CODEBUDDY.md 平台原生行为豁免条款）；协议层等价物 S8a/S8b 已覆盖调用链与参数全等，真实弹窗观感列 07-08 人工复核"

duration: 29min
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 07: phase07.mjs 协议层 UAT 全场景 Summary

**phase 7 协议层 UAT 单脚本落地：web/uat/phase07.mjs（779 行）八场景 33 断言全绿 + 1 平台豁免——配置合并/unix socket（TCP relay 夹具）/base-path 交叉/auth-header+XFF/stop-signal 宽限/降权 self/1001 序列/--open 三形态对真实二进制全链锁定，assertOutputClean 运行时自净零凭据泄漏，既有九脚本零回归。**

## Performance

- **Duration:** 29 min
- **Started:** 2026-08-26T06:00:13Z
- **Completed:** 2026-08-26T06:29:00Z
- **Tasks:** 2/2
- **Files modified:** 1（web/uat/phase07.mjs 新建，779 行）

## Accomplishments

- `node web/uat/phase07.mjs` 退出 0：S1..S8 全场景 33 断言 PASS + 1 项 skipped（S8c 真实弹浏览器平台豁免）+ assertOutputClean PASS（details=33 零命中）——每个场景 spawn 真实 wesh 二进制（/tmp/wesh-uat/wesh，go build 自当前 HEAD + dist 经 `time pnpm -C web build` 确认无漂移后重建）
- S1 配置文件七断言：TOML 铺底（port=0/bind/credential 两组/max-clients=1/command 全配置驱动，spawn 零 CLI listen 参数）→ 凭据 401 与 503 生效；CLI --max-clients 2 覆盖两客户端并存；未知键 TOML 与不存在文件 exit 2 且 stderr 不含凭据探针值（值剥离红线）；chmod 644 权限警告行不含值且服务放行
- S2 unix socket 五断言：预建垃圾文件被 D-10 清理 spawn 成功、启动行 unix://、socket 文件恰 0660（stat mode & 0o777）、stdout 无 http:// 分享链接且退化单行存在；**TCP↔unix relay 夹具**（net.createServer 管道转发，RESEARCH 探针形态转正）后 dialHello + echo 全链——Node 原生 WS 不能直连 unix socket 的 15 行解法，既有断言机制零改动复用
- S3 base-path 八断言：裸 /wesh 307（Location 含 /wesh/）、/wesh/ 200 HTML、/ 404、WS 层双路径隔离（裸 /ws 404 对照 /wesh/ws 101）、share 两行含 /wesh/s/ 前缀、GET /wesh/s/{token}/ 200、POST /wesh/api/attach 携 token → ticket → Hello 经 /wesh/ws → Welcome（rw）——share×base-path 交叉全链（RESEARCH Pitfall 3 的 401 回归链防线）
- S4 auth-header/XFF 四断言：--exit-when-empty 实例 attach+close → exit_when_empty 事件行 remote=198.51.100.7（XFF 链首，多值链形态）+ remote_user=alice；NEL 线形探针 sanitize（'ali\u00C2\u0085ce' 上线 0xC2 0x85 → 剥离后 'alice' 保留且控制字符零泄漏）；对照组无 --auth-header 时 remote=127.0.0.1 系 host:port、无 remote_user 键、XFF 完全忽略（D-20 单一信任闸）
- S5 stop-signal 宽限两子测：trap "" TERM 整组免疫下 1s 宽限补 KILL 退出 255（elapsed 实测 1001-1002ms，900ms 下界证宽限真实经过 + 10s 宽松护栏）；对照 --stop-signal TERM 无 timeout 即终结 255（elapsed 1-2ms）
- S6 降权 self 免 root：--uid/--gid 取 process.getuid/getgid → echo 回读 id -u == self uid；HOME/USER 与当前用户 passwd 条目一致（D-25 LookupId 身份环境改写，数字锚定防回显误命中纪律沿用）
- S7 1001 关停序列：dialHello 完成后 SIGTERM → 客户端收 close 1001 + reason 含 server_shutting_down → wesh 15s 护栏内退出 255（accept-255 同源）
- S8 --open 三形态：headless（env 清 DISPLAY/WAYLAND_DISPLAY）stderr 提示行 + GET / 200 服务正常；fake xdg-open（PATH 前置 + DISPLAY=:99）argv 与启动打印 rw 分享 URL 全等（闭包内比对，detail 只报布尔——token 红线）；真实弹浏览器 skip+reason（双机拓扑纪律）
- 九脚本回归全绿零回归（逐个执行各自退出 0）：phase02 12/12、phase03 18/18、phase04 10/10、phase05 28/28+1豁免、phase05-dims DIMS PASS、phase06 23/23+1豁免、phase04-dom 37/37、phase05-dom 19/19、phase06-dom 37/37+1豁免（含 07-05 所加 1001 断言——无重复冲突）

## Task Commits

每个任务原子提交：

1. **Task 1: phase07.mjs harness + S1 配置合并 / S2 unix socket（relay）/ S3 base-path 交叉 / S4 auth-header+XFF** - `96af989` (test)
2. **Task 2: S5 stop-signal 宽限 / S6 降权 self / S7 1001 序列 / S8 --open + 全量九脚本回归** - `a4d0931` (test)

**Plan metadata:** docs 提交在本 SUMMARY 之后（`docs(07-07): complete phase07 protocol UAT plan`，hash 见 git log）。

## Files Created/Modified

- `web/uat/phase07.mjs`【新，779 行】- 文件头覆盖范围段+红线段+运行方式行（phase06.mjs 同款纪律）；harness 五件逐结构复制 + startWesh 三选项扩展（defaultListen/unix/env）+ dialHello 扩 path/headers + spawnExpectExit 拒绝路径 + TCP↔unix relay 夹具 + waitOutput/pollMatch echo 轮询 + mkTmp/rmTmp 临时目录纪律；S1-S8 八场景函数 + 场景数组（PHASE07_ONLY 调试过滤，提交形态全场景开启）+ assertOutputClean + WR-02 异常通道收口 + 汇总行退出码

## Decisions Made

- **S4c 控制字符探针取 UTF-8 线形等价物构造**（详见 Deviations #1）——plan 字面 C0 \r\n 不可传输；朴素 JS NEL 串经 undici latin1 编码上线为单字节 0x85 复现不了剥离路径；双码点构造上线 0xC2 0x85 = UTF-8 客户端等价线形。
- **S4 事件行触发形态 = --exit-when-empty 的 exit_when_empty logEvent**——正常 detach 无独立事件行，exit_when_empty 是 attach+close 链路的确定性即时事件（携 Attach 入口提取的 c.remote/c.remote_user）；进程随后 HUP 收口 255 顺带锁定（三实例同一形态）。
- **S5a 时序断言加 900ms 下界**——plan 字面只给 <10s 宽松上界；下界使「宽限真实经过 + KILL 补发」可证（TERM 未被忽略时 elapsed ≈ 0 远小于 900ms），实测 1001-1002ms 稳定窗口。
- **S1 断言排序即解零 pacing**——401 负面对照排成功链路之后（fail#1 +1s 节流窗口无后续消费者），零 sleep（05-09 登记纪律第二次沿用）。
- **S8b URL 比对全在闭包内**——记录文件落临时目录随 rmTmp 清理，detail 只报「调用到达/argv相等」布尔（token 红线延伸：磁盘残留也不进仓库/输出）。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - 探针形态缺陷] S4c NEL 探针改 UTF-8 线形等价物构造**
- **Found during:** Task 1（S4 首跑 S4c FAIL：alice保留=false）
- **Issue:** plan behavior 字面探针 'alice\r\nFAKE' 的 C0 \r\n 在真实 HTTP 头值不可传输（httpguts/undici 双侧客户端栈即拒）；改用的朴素 JS 'ali\u0085ce' 经 undici 头值 latin1 编码上线为单字节 0x85——Go 侧非法 UTF-8 解码为 U+FFFD（sanitize 不剥 U+FFFD，日志 'ali\uFFFDce'），复现不了 D-19 剥离路径（调试实测 stderr 证实）
- **Fix:** 双码点线形构造 'ali\u00C2\u0085ce'——undici latin1 上线字节 = 0xC2 0x85 = UTF-8 客户端（Go http/curl）发送 'ali\u0085ce' 的等价线形；Go 解码得 U+0085 NEL 按 D-19 剥离 → 'alice'。本机三探针实证（WS headers 自定义头可传 / NEL 单字节形态 / 双码点线形），源码注释登记机理
- **Files modified:** web/uat/phase07.mjs
- **Verification:** S4c PASS（alice保留=true 控制字符缺席=true）+ 全量 33/33 绿
- **Committed in:** 96af989（Task 1 提交内）

---

**Total deviations:** 1 auto-fixed（Rule 1 探针形态）
**Impact on plan:** plan behavior 意图（控制字符剥离 + 内容保留 + 零泄漏三断言）逐字达成，线形构造是可传输层唯一零依赖形态；prohibition（凭据/token 永不进 detail——assertOutputClean 运行时自证）严格保持。

## TDD Gate Compliance

两 task 均为 type="auto"（Task 1 tdd="true"），plan frontmatter type=execute 非 plan 级 tdd；config workflow.tdd_mode=false（运行时门未激活）。本 plan 交付物本身是测试脚本——被测行为由 07-01..07-06 六 plan 落地（各自含 RED/GREEN TDD 门序）。Task 1 首跑 S4c FAIL 即 RED 信号（探针形态缺陷，非产品缺陷），Rule 1 修正后全绿；两提交均按 plan action 字面的 test(07-07) 类型（commit type 表：test = 测试变更）。无 REFACTOR 需求。

## Issues Encountered

- **S4c 首跑失败（见 Deviations #1）：** undici 头值编码行为是本 plan 唯一实测新发现——header value 按 latin1 上线（码点 >0xFF 行为未探，本场景不需要）；该发现已注释登记进 phase07.mjs（后续 UAT 控制字符场景直接复用线形构造形态）。
- **main.go Edit 工具转义歧义：** 含 \uXXXX 字面的 Edit old_string 会被工具按转义后内容匹配（两次未命中），改 node 脚本精确替换落盘——无代码语义影响。

## User Setup Required

None - no external service configuration required.

## Threat Flags

None——全部新表面均在 plan `<threat_model>` T-07-07a 单条登记内（UAT 输出泄露凭据/token）：assertOutputClean 运行时自净（emittedDetails 全遍历含场景异常通道 + TOML 凭据探针与 share token 同口径 sensitiveTokens + redactArgs 脱敏） mitigate 逐条兑现，33 条 detail 零命中实测。无未建模的信任边界扩张。

## Known Stubs

None——无占位实现；全部 must_have truths 经真实二进制全链断言达成（全场景覆盖 / 全 PASS 退出 0 / assertOutputClean 零泄漏 / 九脚本零回归）。S8c 为 CODEBUDDY.md 显式豁免条款内的平台豁免（skipped+reason），非 stub。

## Next Phase Readiness

- **07-08（README + 人工 UAT + 收尾）直接可用面：** 八场景行为矩阵即 README 部署节素材（配置样例/unix socket 反代/base-path nginx 配方/auth-header 审计行形态/停止信号与退出码 255 运维注记/--open 三形态）；flagged_assumptions 人工复核清单与本 UAT 的 skipped 项（S8c 真实弹浏览器）衔接
- **verify-work：** coverage D1-D8 全部 human_judgment:false + verification pass（deterministic 自动通过面），D9 真实弹浏览器人工复核单条
- **phase 7 闭合度：** OPS-01/02/04/05/09/11、SEC-07、D-23 全部经真实二进制端到端锁定；VALIDATION Wave 0 首项（phase07.mjs）交付

## Self-Check: PASSED

- 文件存在性：web/uat/phase07.mjs FOUND（779 行 ≥ 400 min_lines；grep -c 'node:net' == 1 ≥ 1；grep -c 'remote_user=' == 7 ≥ 1；grep -c 'net\.createServer' == 1 ≥ 1 key_link 形态）+ 本 SUMMARY FOUND
- 提交存在性：2/2 FOUND（96af989 / a4d0931，git log --oneline 核验）
- must_have truths 四条逐条达成：①全场景覆盖（S1 配置合并与优先级 / S2 unix socket 经 TCP relay / S3 base-path 页面+WS 升级+share 交叉 / S4 auth-header 记录与 sanitize + XFF 换键 / S5 stop-signal 宽限补 KILL / S6 降权 self / S7 1001 关停序列 / S8 --open headless 跳过与 fake xdg-open argv）；②每场景 spawn 真实二进制断言全 PASS 退出 0（33/33）；③assertOutputClean 运行时自净（details=33 零命中，含场景异常通道与 TOML 凭据探针同口径）；④既有九脚本全绿零回归（退出码逐个核验）
- WINDOWS.md 登记：deviation id 21（S4c 线形构造）+ unrun-verify id 22（S8c 豁免）

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
