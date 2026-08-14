---
phase: 1
slug: pty
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-14
---

# Phase 1 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> 阶段：PTY 行走骨架（核心 PTY 管道）。验证方式：L1 grep-depth（register 在 plan 期已 authoring，ASVS=1，threats_open=0 命中短路规则，未 spawn auditor）。

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| 浏览器 ↔ WS/HTTP | 不可信客户端输入（INPUT/RESIZE 帧）从此进入服务端；Phase 1 无认证 | INPUT/RESIZE 帧、终端 I/O |
| 服务端 → 子进程 | argv 与 env 从此边界注入新进程（命令注入与密钥泄露面） | argv、env |
| 服务端 → 文件系统/embed | 静态资产构建期嵌入，运行时零磁盘依赖 | 单 HTML（go:embed） |
| npm/go 依赖安装 | 供应链边界（registry → 构建产物） | 构建产物 |
| CI runner → registry | CI 从 npm/go proxy 拉依赖构建 | 构建产物 |
| 内核 kqueue → 服务端进程 | darwin 平台事件边界（EVFILT_PROC/NOTE_EXIT） | 进程退出事件 |
| 文档 → 部署者 | README 是 D-05 accepted 风险的补偿控制面 | 部署指引 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Tampering | spawn.go exec 路径 | critical | mitigate | `exec.Command(argv[0], argv[1:]...)` 数组形式绝不经 shell（spawn.go:29）；TestExecArrayNoShell 断言 `$(id)` 不展开 | closed |
| T-01-02 | Information Disclosure | spawn.go env 注入 | high | mitigate | `cmd.Env = whitelistEnv()` 替换式注入（spawn.go:30，非 os.Environ() 追加）；TestEnvWhitelist 双层断言宿主注入 AWS_SECRET_ACCESS_KEY 不可见 | closed |
| T-01-03 | Elevation of Privilege | server.go WS Accept（CSWSH） | high | mitigate | `websocket.Accept(w, r, &websocket.AcceptOptions{})` 空字面量（server.go:81）——不跳过库默认 Origin 同源校验（同 Host 放行、跨源拒绝）；白名单属 Phase 3 | closed |
| T-01-04 | DoS | server.go WS 读路径（预认证放大） | medium | mitigate | SetReadLimit 库默认 32768B，超限自动 1009；未知帧 1002 关闭；三层上限完整版 Phase 2 | closed |
| T-01-05 | DoS | spawn 失败路径（ttyd close(0) 缺陷） | medium | mitigate | creack/pty 失败只关自己打开的 fd；TestSpawnFailKeepsStdio 回归 fd 0/1/2 存活 | closed |
| T-01-06 | DoS | 收割路径（僵尸耗尽进程表） | medium | mitigate | `cmd.Wait()` 唯一收割者（Linux=stdlib pidfd waitid）；TestReap 断言 /proc/<pid> 消失 | closed |
| T-01-07 | DoS | io.go drain（孙进程持 slave 挂死） | medium | mitigate | Wait 返回后 200ms 时限 drain，到点无条件 close(master)；残留孙进程下次读写得 EIO 自然消亡 | closed |
| T-01-08 | Tampering | 关闭码合法性 | low | mitigate | 主动发送仅 1000/1002；1009 由库默认产生；1006 永不写入（RFC6455 §7.4） | closed |
| T-01-09 | Information Disclosure | 监听面（无认证 + bind 0.0.0.0） | high | accept | D-05 用户锁定：Phase 1 接受 LAN 可达取舍；补偿控制 = README 首屏警示（行 5-7 已落地）+ Phase 3 认证/TLS 收口 | closed |
| T-01-10 | Tampering | RESIZE 恶意尺寸 | low | mitigate | proto.DecodeResize 钳制 cols/rows 到 [1,1000]；JSON 解码失败静默丢弃不关连接；TestResize 验证 | closed |
| T-01-SC | Tampering | npm/pnpm/go 依赖安装 | high | mitigate | RESEARCH §Package Legitimacy Audit 全 Approved；版本全钉死（vite 8.2.1/@xterm 6.0.0 等）；CI 走 pnpm --frozen-lockfile 与 go.sum 校验 | closed |
| T-02-01 | Tampering | spawn_test.go | critical | mitigate | TestExecArrayNoShell 固化 T-01-01（产品退化为 shell 拼接即红） | closed |
| T-02-02 | Information Disclosure | spawn_test.go | high | mitigate | TestEnvWhitelist 固化 T-01-02（白名单函数 + 子进程 env 输出双层断言） | closed |
| T-02-03 | DoS | spawn/io/reap 测试 | medium | mitigate | TestSpawnFailKeepsStdio / TestResize / TestReap 固化 T-01-05/06 | closed |
| T-02-04 | Tampering | 测试夹具（sh -c 编排） | low | accept | /bin/sh -c 仅出现在 TestResize 测试编排串，不进入产品 spawn 路径 | closed |
| T-03-01 | DoS | server.go 409 路径 | medium | mitigate | TestSecondClient409 固化 D-09：第二连接 Accept 前 409，不耗 PTY/WS 资源 | closed |
| T-03-02 | Tampering | 关闭码路径 | low | mitigate | TestUnknownFrame1002 固化未知帧→1002 且全程无 1006 | closed |
| T-03-03 | DoS | 生命周期退出路径 | medium | mitigate | TestExitCodePropagation / TestClientDisconnectSIGHUP 固化 D-10/D-11 | closed |
| T-03-04 | Tampering | CLI 参数解析 | low | mitigate | TestParseArgs 固化 D-02：`--` 后参数原样进 exec 数组 | closed |
| T-04-01 | DoS | reap_darwin.go | medium | mitigate | kqueue EV_ONESHOT 一次性注册 + cmd.Wait() 唯一收割；Q1 竞态 CI 裁决=watcher 成立（TestKqueueExitNormal/ZombieRace 均 PASS） | closed |
| T-04-02 | Tampering | ci.yml（测试静默失效） | medium | mitigate | 不设 CGO_ENABLED=0（race 需 cgo）；CI 明示 `go test -race -count=1 -v` | closed |
| T-04-SC | Tampering | CI 依赖安装 | high | mitigate | web job `pnpm -C web install --frozen-lockfile`；go.sum 校验；actions 全钉版（checkout@v7.0.1/setup-go@v7.0.0/pnpm-action-setup@v6.0.10/setup-node@v4） | closed |
| T-05-01 | Information Disclosure | README.md（误部署到不可信网络） | high | mitigate | 首屏醒目警示"Phase 1 无认证，仅在可信网络使用"（行 5-7）+ 单次语义说明——T-01-09 补偿控制落地 | closed |
| T-05-02 | Information Disclosure | README.md（env 白名单可预期性） | low | mitigate | README 列明 SEC-06 白名单变量集合 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-09 | T-01-09 | Phase 1 无认证 + 默认 bind 0.0.0.0（LAN 可达）。D-05 用户明确锁定的单次语义取舍：进程级单次会话模型下认证无意义，补偿控制为 README 首屏醒目警示 + Phase 3 认证/TLS 收口 | 用户（D-05，CONTEXT.md） | 2026-08-14 |
| AR-02-04 | T-02-04 | TestResize 测试编排串使用 /bin/sh -c，仅限测试代码、不进入产品 spawn 路径；产品 spawn 已由 TestExecArrayNoShell 独立断言不经 shell | 用户（plan 01-02 accept） | 2026-08-14 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-14 | 24 | 24 | 0 | gsd orchestrator（L1 grep-depth；register plan 期已 authoring，ASVS=1，短路规则命中） |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-14
