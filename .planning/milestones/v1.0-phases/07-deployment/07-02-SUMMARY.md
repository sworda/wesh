---
phase: 07-deployment
plan: 02
subsystem: infra
tags: [unix-socket, socket-mode, socket-owner, chmod, chown, listen-fork, systemd, deployment, tdd]

requires:
  - phase: 07-deployment/07-01
    provides: parse 期校验插入点（write-policy 枚举同位）与 TestTLSKeyPairError 错误表归属先例 + fs.Visit/validateStartup 扩展落地形态
  - phase: 03-auth/03-04
    provides: TestParseArgs 命名字段扩展纪律（既存行零改动）+ showVersion 早退后校验插入点先例
  - phase: 05-multi-client
    provides: writePolicySet/maxClientsSet 显式设置位先例（D-08 互斥锚定显式位的形态来源）
  - phase: 06-session-lifecycle/06-04
    provides: exitEmptySet 先例 + --once 组合校验双 flag 名文案形态 + TestStartupRefusalNoResource 零资源纪律
provides:
  - --socket/--socket-mode/--socket-owner CLI 公开契约（D-08/D-09 one-way）：独立 unix socket flag 与显式 --port/--bind 互斥、八进制权限位默认 0660（>0777 拒绝）、owner user[:group] parse 期解析为 uid/gid 数字对（未知用户/组 exit 2，未给 = -1 哨兵）
  - listenSocket(path, mode, uid, gid) helper：os.Remove（D-10 残留清理）→ net.Listen("unix") → os.Chmod（0660 确定性）→ os.Chown；任一步失败 Close 自动 unlink 回滚零残留（T-07-02a/b）
  - validateStartup 三新行：D-08 互斥（portSet/bindSet 显式位锚定，默认 port/bind 不误判）、D-09 单给矛盾（socketModeSet/socketOwnerSet）、D-11 unix 形态跳过 bind 安全矩阵（文件系统权限即认证边界，loopback 早退同款信任档位）
  - unix 启动打印 listening on unix://<path>（三斜杠形态）+ 分享链接退化单行；TCP 路径逐字节零漂移
  - TestListenSocket 四子测（残留清理+可拨通/0660/owner=self/失败回滚零残留含非 root Chown EPERM 注入）
affects: [phase-07 后续 plans（07-05 --open×--socket 组合校验消费 socket 字段 / 07-06 配置文件 socket 三键——RESEARCH Pattern 4 fileConfig 已预留 / 07-08 README+phase07.mjs unix socket 场景 TCP relay）, verify-work]

actuals:
  tokens: 9766
  tasks: 2
  commits: 4

tech-stack:
  added: []
  patterns:
    - "unix socket listen 序列：os.Remove → net.Listen(\"unix\") → os.Chmod → os.Chown，失败即 ln.Close() 回滚（UnixListener 默认 unlink:true 自动删文件）——Go 不 bind 前 unlink、socket mode=0777&~umask 内核行为，两确定性都必须显式达成（07-RESEARCH Pattern 1 GOROOT 实证）"
    - "CLI 互斥/单给组合校验锚定 fs.Visit 显式设置位而非终值（portSet/bindSet/socketModeSet/socketOwnerSet）——--socket 与默认 port/bind 同给不误判（write-policy/max-clients/exit-empty 三先例第四次沿用）"

key-files:
  created: []
  modified:
    - cmd/wesh/main.go
    - cmd/wesh/main_test.go

key-decisions:
  - "parseArgs 头注释 flag 计数 17→21 并补 Phase 7 行（Rule 1 文档漂移修复）——07-01 加 --base-path 后计数已陈旧，本 plan 再加三 flag 使漂移扩大；同区域主题直接相关，一次修正"
  - "TestListenSocket 失败回滚子测补 Chown EPERM 注入（Rule 2 覆盖强化）——plan 给定的 Listen 失败注入（父目录不存在）零残留断言平凡成立；非 root 下 chown 他人 uid EPERM 是 T-07-02a『Close 自动 unlink 回滚』mitigation 的真实可达证据（root 环境自动跳过该注入，四子测数不变）"

patterns-established:
  - "socket 组三 flag 分层纪律：parse = 形状（StringVar 原样 + 八进制 ParseUint + owner 名字解析），validate = 组合矛盾（互斥/单给/D-11 跳过）——与既有 17 flag 同分层，组合矛盾零窗口暴露"
  - "unix 形态启动打印退化形态：地址行 unix:// 前缀 + cfg.socket 原样（三斜杠自然达成），分享链接两行替换为单行提示（无 host:port 可拼时绝不拼误导性 TCP 链接）；ln.Addr().(*net.TCPAddr) 断言防御留 TCP 分支，unix 形态天然不拼端口"

requirements-completed: [OPS-01]

coverage:
  - id: D1
    description: "--socket 三 flag parse 契约：路径原样入 cfg；--socket-mode 默认 0660 与自定义八进制解析（非八进制/>0777 含特殊位 parse 期 exit 2）；--socket-owner 经 os/user.Lookup[/LookupGroup] 解析为 uid/gid 数字对（未知用户/未知组 exit 2，未给 = -1/-1 哨兵）"
    requirement: OPS-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestParseArgs（socket 四行）+ #TestTLSKeyPairError（非法 mode/owner 四行）"
        status: pass
    human_judgment: false
  - id: D2
    description: "validateStartup 组合矛盾矩阵三新行：--socket × 显式 --port/--bind 互斥 exit 2（双 flag 名进文案，默认 port/bind 不误判）；--socket-mode/--socket-owner 单给 exit 2；D-11 unix 形态跳过 bind 安全矩阵（零值 bind 无凭据不拒不警告）；拒绝路径零 listen 零 spawn"
    requirement: OPS-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestStartupMatrix（socket 五行）+ #TestStartupRefusalNoResource（socket 两子测 exit 2）"
        status: pass
    human_judgment: false
  - id: D3
    description: "listenSocket 序列：残留垃圾文件 listen 前清理且可拨通（D-10）；stat 权限位恰为 0660（显式 Chmod 不靠 umask）；owner=self 属主正确；失败回滚零残留（Listen 失败注入 + 非 root Chown EPERM 注入——Close 自动 unlink）"
    requirement: OPS-01
    verification:
      - kind: unit
        ref: "cmd/wesh/main_test.go#TestListenSocket（四子测全 PASS，-race 全量套件五包绿）"
        status: pass
    human_judgment: false
  - id: D4
    description: "unix 启动打印与 TCP 零漂移：stdout 首行 listening on unix://<path>（三斜杠）+ 分享链接退化单行（无 http:// 链接行）；curl --unix-socket 建连 HTTP 200；--socket+--port / --socket-mode 单给 / 非法 mode / 未知 owner 均 exit 2 文案含双 flag 名；TCP 默认形态（--bind 127.0.0.1 --port 0）listening/share 两行与随机端口打印逐字节不变"
    requirement: OPS-01
    verification:
      - kind: other
        ref: "真实二进制冒烟（go build ./cmd/wesh → /tmp 临时二进制，socket 0660/curl 200/五组 exit 2 文案/TCP 打印逐项观测；产物已清理）"
        status: pass
    human_judgment: false

duration: 32min
completed: 2026-08-26
status: complete
---

# Phase 07 Plan 02: UNIX socket 监听形态 Summary

**--socket/--socket-mode/--socket-owner 三 flag 全链落地：parse 期八进制/owner 名字解析 + validateStartup 互斥/单给/D-11 跳过矩阵 + listenSocket Remove→Listen→Chmod→Chown 序列（失败回滚零残留）+ unix:// 启动打印与分享链接退化，TCP 路径逐字节零漂移。**

## Performance

- **Duration:** 32 min
- **Started:** 2026-08-26T00:55:41Z
- **Completed:** 2026-08-26T01:27:37Z
- **Tasks:** 2/2
- **Files modified:** 2（cmd/wesh/main.go +234/-47 行区、cmd/wesh/main_test.go +219/-10 行区；合计 406 insertions / 47 deletions）

## Accomplishments

- `wesh --socket $TMP/wesh.sock -- bash` 全链实证：残留垃圾文件被 listen 前 os.Remove 清理（D-10——Go listenStream 直接 syscall.Bind 无 unlink，不 Remove 必收 EADDRINUSE），socket 文件权限恰为 0660（显式 Chmod 达成，不靠 umask 漂移），stdout 首行 `listening on unix://<path>` 三斜杠形态 + 分享链接退化单行（无误导性 TCP 链接），curl --unix-socket 建连 HTTP 200
- 组合矛盾零窗口暴露：`--socket`×显式 `--port`/`--bind` 互斥、`--socket-mode`/`--socket-owner` 单给均 validateStartup fail-fast exit 2（双 flag 名进文案；显式设置位锚定——`--socket` 与默认 port/bind 同给不误判）；非法 socket-mode（非八进制/>0777）与未知 owner parse 期 exit 2
- D-11 落地：unix 形态跳过 bind 安全矩阵（文件系统权限即认证边界——零值 bind 无凭据也不拒不警告，loopback 早退同款信任档位），拒绝路径零 listen 零 spawn（exit 2 与运行时 exit 1 档位区分佐证）
- TestListenSocket 四子测锁定 listen 序列：残留清理+可拨通 / 权限位 0660 / owner=self 属主 / 失败回滚零残留（含非 root Chown EPERM 注入——UnixListener Close 自动 unlink 真实可达证据）
- TCP 路径零漂移双证：`--bind 127.0.0.1 --port 0 -- bash` listening/share 两行与随机端口打印逐字节不变（冒烟观测）+ 全量 `go test -race -count=1 ./...` 五包全绿

## Task Commits

每个任务原子提交（两任务均 TDD，各含 RED/GREEN 两提交）：

1. **Task 1 RED: --socket parse/validate 失败测试** - `bb4ee92` (test)
2. **Task 1 GREEN: 三 flag + parse 解析 + validateStartup 扩展** - `81ef8ee` (feat)
3. **Task 2 RED: TestListenSocket 失败测试（四子测）** - `2077c7e` (test)
4. **Task 2 GREEN: listenSocket + run() unix 分岔 + 启动打印退化** - `a51e56c` (feat)

**Plan metadata:** docs 提交在本 SUMMARY 之后（`docs(07-02): complete unix-socket plan`，hash 见 git log）。

## Files Created/Modified

- `cmd/wesh/main.go` - config 监听组九字段（socket/socketModeStr/socketMode/socketOwner/socketUid/socketGid + portSet/bindSet/socketModeSet/socketOwnerSet 显式位）+ 三 flag 注册 + fs.Visit 四赋值 + Parse 返回处 socket-mode 八进制 ParseUint（>0777 拒绝）与 socket-owner os/user 解析（-1 哨兵初始化）+ validateStartup 三新行（D-08/D-09/D-11）+ listenSocket helper（Remove→Listen→Chmod→Chown，失败 Close 回滚）+ run() listen 分岔与 unix 启动打印分岔（TCP 分支逐字不动）
- `cmd/wesh/main_test.go` - TestParseArgs socket 四行（含 owner self/self:group 数字对断言字段扩展）+ TestTLSKeyPairError 非法 mode/owner 四行 + TestStartupMatrix socket 五行（互斥两/单给两/D-11 跳过）+ TestStartupRefusalNoResource 子测化重构加 socket 两行 + TestListenSocket 四子测

## Decisions Made

- **parseArgs 头注释 flag 计数同步修复**（详见 Deviations #1）——计数 17→21 并补 Phase 7 flag 行，07-01 遗留陈旧随本 plan 三 flag 加入一次修正。
- **失败回滚测试补 Chown EPERM 注入**（详见 Deviations #2）——plan 给定的 Listen 失败注入保留，另加非 root 下 chown uid 1 EPERM 注入，使 T-07-02a『Close 自动 unlink』mitigation 有真实可达证据。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - 文档漂移] parseArgs 头注释 flag 计数 17→21 并补 Phase 7 行**
- **Found during:** Task 1 GREEN（编辑 parseArgs 头注释区域时发现）
- **Issue:** 头注释仍写「共 17 个」且无 Phase 7 行——07-01 加 --base-path 后已陈旧（实际 18），本 plan 再加三 flag 后漂移扩大（实际 21）；plan 未列该项，但漂移区域与本 task 修改主题（flag 清单）直接同一处
- **Fix:** 计数改 21 并补 Phase 7 行（--base-path 07-01 D-13 + --socket 三 flag D-08/D-09 及分层指向）
- **Files modified:** cmd/wesh/main.go
- **Verification:** 头注释与 flag 注册实数一致（21 = 17 既有 + base-path + socket 三）；go vet/build/全量测试绿
- **Committed in:** 81ef8ee（Task 1 GREEN 提交内）

**2. [Rule 2 - 覆盖强化] TestListenSocket 失败回滚子测补 Chown EPERM 注入**
- **Found during:** Task 2 RED（按 plan action ④ 写四子测时）
- **Issue:** plan 给定的失败注入（父目录不存在使 Listen 失败）下「回滚零残留」断言平凡成立——失败发生在文件创建之前，无文件可残留；must_have truth『listen 后任一步失败回滚零残留（UnixListener Close 自动 unlink）』的 Listen 之后失败路径（Chmod/Chown 失败）无真实可达证据
- **Fix:** 同一子测内补 Chown 失败注入——非 root 时 chown 他人 uid(1) 必收 EPERM，断言 error 且已建 socket 文件被 Close 自动 unlink 删除（root 环境自动跳过该注入保持确定性；四子测数与 plan 字面一致）
- **Files modified:** cmd/wesh/main_test.go
- **Verification:** TestListenSocket/failure_rollback_leaves_no_residue PASS（本机 uid 51714 非 root，EPERM 注入实际走到）；T-07-02a mitigation 证据闭合
- **Committed in:** 2077c7e（Task 2 RED 提交内）

---

**Total deviations:** 2 auto-fixed（1 文档漂移 Rule 1，1 覆盖强化 Rule 2）
**Impact on plan:** 两修正均为 plan 自身目标（flag 清单准确性 / must_have truth 证据强度）的忠实落地，零范围蔓延；prohibition（权限不得由 umask 漂移决定、Chmod/Chown 失败必须回滚而非带病放行）严格保持。

## Issues Encountered

- **main.go parseArgs 哨兵赋值编辑首回未落盘：** Edit 工具首回报告成功但 `cfg.socketUid, cfg.socketGid = -1, -1` 未出现在文件（同批其余三处编辑均在——07-01 登记的工具层偶发同族再现）；TestParseArgs 以「socketUid/socketGid = 0/0, want -1/-1」在 23 个既存行当场捕获（零值断言覆盖既存行的扩展纪律发挥回归哨兵作用），重新应用同内容编辑后落盘（main.go:137），全套件转绿。工具层偶发，无代码语义影响。
- **gofmt 对齐漂移（本 task 引入，随 task 修正）：** config 监听组字段注释列对齐与 socketOwnerSet 最长字段名不齐，GOROOT gofmt -w 修正后零漂移（cmd/ 目录 gofmt -l 无输出）；internal/server 两既有漂移文件按 SCOPE BOUNDARY 未触碰（deferred-items.md 既定路由）。

## User Setup Required

None - no external service configuration required.

## Threat Flags

None——全部新表面（三 flag、listenSocket 序列、unix 打印退化）均在 plan `<threat_model>` T-07-02a/b/c/d 四条登记内：残留清理（T-07-02a/T-07-02d mitigate）、0660 显式 Chmod + 失败回滚（T-07-02b mitigate）、umask 窗口（T-07-02c accept——Chmod/Chown 先于 listening 打印完成，窗口内无客户端被指引）逐条兑现，无未建模的信任边界扩张。

## Known Stubs

None——无占位实现；全部 must_have truths 经 Go 单测 + 真实二进制冒烟达成（listen unix/权限位 0660/残留清理/失败回滚零残留/互斥与单给 exit 2/D-11 跳过/分享链接退化/TCP 零漂移）。

## Next Phase Readiness

- 07-03..07-08 可直接开工无阻塞：07-05 --open×--socket 组合校验（RESEARCH OQ1 已定 fail-fast 形态）可消费 cfg.socket 与显式位先例；07-06 配置文件 socket 三键（fileConfig.Socket/SocketMode/SocketOwner——RESEARCH Pattern 4 结构体已预留）经同一 parse 期校验函数复用；07-08 phase07.mjs unix socket 全链场景经 TCP relay 夹具（RESEARCH Pattern 7 探针实证）
- OPS-01 三条（端口 0=随机/绑定地址/UNIX socket 含属主）全部闭合：端口 0 与绑定地址为现状行为（main.go 既有），unix socket 半条本 plan 落地

## Self-Check: PASSED

- 文件存在性：2/2 FOUND（cmd/wesh/main.go 含 func listenSocket ×1、listening on unix:// ×1、unavailable on unix socket ×1；cmd/wesh/main_test.go 含 TestListenSocket ×1）+ 本 SUMMARY FOUND
- 提交存在性：4/4 FOUND（bb4ee92 / 81ef8ee / 2077c7e / a51e56c）
- must_have 内容断言：grep 三闸 ==1（见上）；`grep -c 'socketModeSet\|socketOwnerSet\|portSet\|bindSet' cmd/wesh/main.go` = 11 ≥ 8；go vet ./cmd/wesh 退出 0；GOROOT gofmt -l cmd/ 零输出；`go test -race -count=1 ./...` 五包全绿

---
*Phase: 07-deployment*
*Completed: 2026-08-26*
