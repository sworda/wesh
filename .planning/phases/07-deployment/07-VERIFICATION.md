---
phase: 07-deployment
verified: 2026-08-26T09:05:00Z
status: human_needed
score: 46/46 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "A1 真实浏览器 --open 弹窗（OPS-11）：有 GUI 环境运行 wesh --open --writable -- bash"
    expected: "系统默认浏览器自动打开 rw 分享链接（含 token 免交互），终端可用"
    why_human: "真实 GUI 弹窗属平台原生行为豁免（CODEBUDDY.md 测试策略第 5 条）；协议层等价物已由 phase07.mjs S8a（headless 跳过）+ S8b（fake xdg-open argv 全等）覆盖——07-UAT.md A1"
  - test: "A2 真实 nginx 反代挂载观感（OPS-02）：按 README 配方配置真实 nginx（location = /wesh 精确块 + location /wesh/ 前缀块），浏览器访问裸 /wesh"
    expected: "308 重定向后页面加载、WS 升级成功、终端可用；idle >60s 不断连（proxy_read_timeout 3600s > ping 5s）"
    why_human: "真实反代栈观感属平台原生行为豁免；协议层等价物已由 phase07.mjs S3a-h 全链覆盖，nginx 匹配语义 A1 已经本机 nginx 1.14.1 实测闭合（07-UAT.md C1 留档）——07-UAT.md A2"
  - test: "B4 root 降权 nobody（OPS-05，需 root）：sudo ./wesh --bind 127.0.0.1 --uid 65534 --gid 65534 -- bash，attach 后 id / echo $HOME $USER $LOGNAME"
    expected: "id 显示 nobody/nogroup 且附加组清空；HOME/USER/LOGNAME 按 nobody passwd 条目改写；SIGTERM 后 1001 序列、退出码 255"
    why_human: "需 root 权限的真实降权（自动断言只能降权 self——phase07.mjs S6 已覆盖 self 全链 + 身份改写）；附加组清空语义与 nobody 无 shell 行为属人工复核——07-UAT.md B4"
  - test: "B6 macOS open 与 TLS 组合（OPS-11）：macOS 上 wesh --open -- bash；及 --open + --tls-cert/--tls-key 组合"
    expected: "open 命令拉起默认浏览器打开 ro 链接；TLS 组合打开 https:// 链接（自签证书警告属预期）；xdg-open/open 返回非零仅警告不阻断"
    why_human: "macOS 平台行为本机（Linux headless）不可达，属平台豁免；darwin 分支为构建标签差异——07-UAT.md B6"
  - test: "B1/B2/B3/B5 flagged assumptions 复核：socket 并发中断语义 / 多值与空值 auth-header / --cwd symlink·--term 任意值·--stop-timeout 极大值 / TOML 语法变体与空 command"
    expected: "各探针原文预期行为与真实部署一致（07-UAT.md B 节逐项步骤与预期）"
    why_human: "边缘语义均有 Go 测试/表驱动覆盖，复核项为真实部署行为一致性确认而非缺失实现——07-UAT.md B1/B2/B3/B5"
---

# Phase 7: 部署与配置 验证报告

**Phase Goal:** 真实运维场景可部署——监听形态齐全、配置文件落地、反代友好
**Verified:** 2026-08-26T09:05:00Z（HEAD e022a8b）
**Status:** human_needed
**Re-verification:** 否——初次验证

## Goal Achievement

### Observable Truths（ROADMAP 成功准则——契约层）

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | 端口（0=随机并打印实际端口）/绑定地址/UNIX socket（含属主）可配置；TOML 配置文件支持，CLI 参数覆盖配置文件 | ✓ VERIFIED | --help 实证 13 新 flag 全在册；phase07.mjs S1a-g（配置凭据 401/max-clients 503 生效/CLI 覆盖/未知键 exit 2 不含探针值/不存在 exit 2/chmod 644 警告不含值）+ S2a-f（unix:// 打印/0660 权限/残留清理/relay 全链/CR-01 拒绝）真实二进制全过；码：listenSocket Lstat 闸+Remove→Listen→Chmod→Chown+回滚（main.go:1015-1037）、fileConfig 27 键+DisallowUnknownFields（config.go:42-69,104） |
| 2 | 反代子路径挂载（/wesh/ base-path）下页面与 WS 升级均正常（尾斜杠规范化）；反代注入的可信用户头记录进服务端审计日志（remote_user 审计归因——D-15 修订后文本） | ✓ VERIFIED | phase07.mjs S3a-h（307 Location 含前缀/200 页面/404 隔离/WS 双路径/share×base-path 全链）+ S4a-d（remote=XFF 链首+remote_user=alice/NEL 控制字符剥离/对照组零漂移）全过；码：StripPrefix 仅包静态伺服（server.go:404,435）、normalizeBasePath 五族拒绝（main.go:804-827）、sanitizeRemoteUser+ParseIP 闸（proxy.go:55,93）；REQUIREMENTS.md SEC-07 与 ROADMAP SC2 均已修订为「审计归因」单一口径 |
| 3 | 子进程以指定 cwd/TERM 启动，停止信号发给进程组（可配 TERM→KILL 宽限）；可以指定 uid/gid 降权运行；可选启动后自动打开浏览器 | ✓ VERIFIED | phase07.mjs S5a/b（trap 忽略 TERM→1s 宽限补 KILL 退出 255/对照 TERM 即终结）+ S6a/b（降权 self id -u 全等+HOME/USER passwd 条目一致）+ S8a/b（headless 跳过不阻断+fake xdg-open argv==rw 分享 URL）全过；码：StartOptions+Credential fork 后 exec 前（spawn.go:54,86）、SignalGroup 负 pid 进程组（signal_linux.go:16）、LookupId 身份改写（spawn.go:121）、openBrowser headless 检测（main.go:1235） |

**Score:** 46/46 truths verified（3 契约 SC + 43 plan must-have truths；0 项 behavior-unverified）

### Plan must-have Truths 逐条核验（43/43）

| Plan | Truths | Status | 关键证据 |
|------|--------|--------|----------|
| 07-01（OPS-02） | 6/6 | ✓ VERIFIED | S3a-h 真实二进制全链；basepath_test.go 229 行（min 80）；main.ts up/'../../'/new URL 三改落位（L509,611）；dist 产物含 '../../' 与 'Server shutting down'；normalizeBasePath 五族拒绝；shareURLRO/RW 拼串注入 basePath |
| 07-02（OPS-01） | 5/5 | ✓ VERIFIED | S2a-f 全过；listenSocket 序列+回滚在码；TestListenSocket 五子测（main_test.go:896）；validateStartup D-08 互斥/D-09 单给/D-11 跳过三行在码（main.go:942-965）；unix:// 打印与退化单行 |
| 07-03（SEC-07） | 6/6 | ✓ VERIFIED | S4a-d 全过（含对照组 XFF 忽略）；proxy.go sanitize/ParseIP 闸；auth.go p.clientIP/p.remote 换键（L103,108,116）；logEvent variadic 第四参（server.go:1071）；D-16 暴露面警告行在码（main.go:976-978）；TestAuthHeaderNoAuthBypass/TestXFFThrottleKey/TestRemoteUserLogging（proxy_e2e_test.go）；旧 clientIP 自由函数零残留（3 命中均为新方法调用或注释） |
| 07-04（OPS-04/05） | 6/6 | ✓ VERIFIED | S5a/b+S6a/b 全过；StartOptions/Credential/LookupId 在码；SignalGroup linux/darwin 同签名对件；StopSignalByName 四枚举；SignalHangup 零残留（grep 0）；--uid/--gid 成对校验（main.go:935）；--cwd stat 预检 |
| 07-05（OPS-11+D-23） | 5/5 | ✓ VERIFIED | S7a/b（1001+reason server_shutting_down+退出 255）+S8a/b 全过；Server.Shutdown 在码（server.go:1264）；proto.go 1001 翻正启用；main.ts case 1001 终态面板不调 startReconnect（L895-905）；phase06-dom D11a-c 三重断言 PASS（面板+守候窗零新连接+重连上下文终止）；--socket×--open 冲突校验在码 |
| 07-06（OPS-09） | 6/6 | ✓ VERIFIED | S1a-g 全过；fileConfig 恰 27 键（26 flag 同名+command）；DisallowUnknownFields 严格模式；prescanConfigPath 仅扫显式 --config（D-01 零隐式路径）；go-toml/v2 v2.4.3 入 go.mod；优先级链 flag>env>config>default 经 TestParseArgsWithConfig/TestConfigMerge 锁定；值剥离红线经 TestConfigRedLines |
| 07-07（全需求 UAT） | 4/4 | ✓ VERIFIED | phase07.mjs 797 行（min 400）；34/34 PASS+1 豁免（S8c 平台豁免）；SEC 自净 34 detail 零命中；本验证复跑既有九脚本全绿（phase02 12/12、phase03 18/18、phase04 10/10、phase05 28/28+1、phase05-dims PASS、phase06 23/23+1、phase04-dom 37/37、phase05-dom 19/19、phase06-dom 37/37+1） |
| 07-08（收口文档） | 5/5 | ✓ VERIFIED | README 部署节八小节（--base-path×3/--socket×8/--auth-header×5/proxy_read_timeout×2 grep 达标；ttyd -H 模型差异段 L338/340）；REQUIREMENTS.md SEC-07「审计归因」修订+D-15 注记（L49-50）；ROADMAP SC2 同步修订；07-UAT.md 56 行 9 勾选项；六段式本验证复跑：go vet 0/-race 五包绿 51s/pnpm build 0 且 dist 无 diff/go build 0 |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/wesh/main.go` | 13 新 flag + 校验矩阵 + listenSocket + openBrowser + NotifyContext + 两阶段合并 | ✓ VERIFIED | 全部符号在码且经 UAT/单测行为实证 |
| `cmd/wesh/config.go` | fileConfig 27 键 + DisallowUnknownFields + D-07 + 值剥离 | ✓ VERIFIED | 新文件 165+ 行；27 toml 键逐字=flag 名 |
| `internal/server/proxy.go` | sanitizeRemoteUser + proxyInfo 三提取 | ✓ VERIFIED | 新文件；ParseIP 闸（CR-02）在码 |
| `internal/server/server.go` | BasePath/AuthHeader/StopSignal Options + Shutdown + logEvent 第四参 | ✓ VERIFIED | 装配链路完整（New L327-330） |
| `internal/pty/spawn.go` | StartOptions + Credential + whitelistEnv 身份改写 | ✓ VERIFIED | S6 真实降权实证 |
| `internal/pty/signal_{linux,darwin}.go` | SignalGroup + StopSignalByName 对件 | ✓ VERIFIED | 两平台同签名同表 |
| `internal/server/basepath_test.go` | 307/404/405/share/WS 断言（min 80 行） | ✓ VERIFIED | 229 行，-race 绿 |
| `internal/server/proxy_test.go` + `proxy_e2e_test.go` | sanitize/clientIP/记录/换键/正交五测（min 100 行） | ✓ VERIFIED | 138 行 + e2e 分件四测，-race 绿 |
| `internal/server/shutdown_test.go` | TestShutdown1001/StopTimeout（min 60 行） | ✓ VERIFIED | 164 行，-race 绿 |
| `internal/server/stopseq_test.go` | TERM 终结 + KILL 补发 | ✓ VERIFIED | 123 行，-race 绿 |
| `cmd/wesh/config_test.go` | 加载/合并/优先级/红线（min 150 行） | ✓ VERIFIED | 954 行，-race 绿 |
| `web/src/main.ts` | 相对 URL 三改 + case 1001 | ✓ VERIFIED | jsdom 三套件全绿 |
| `web/dist/index.html` | 产物入库且与源码一致 | ✓ VERIFIED | pnpm build 后 git 零 diff |
| `web/uat/phase07.mjs` | 八场景 + relay 夹具 + 自净（min 400 行） | ✓ VERIFIED | 797 行，34/34 实证 |
| `README.md` / `REQUIREMENTS.md` / `ROADMAP.md` / `07-UAT.md` | 部署文档 + D-15 双文件修订 + 人工清单 | ✓ VERIFIED | grep 与读码核验单一口径 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| main.ts connect() | server Handler() bp 路由 | 相对 fetch/WS URL → mux 前缀 | ✓ WIRED | S3e/S3h WS 全链实证；up+'api/attach'、new URL(up+'ws') 在码 |
| main.go 分享链接打印 | registerShareRoutes bp 注册 | 同一 basePath 值拼串与注册 | ✓ WIRED | S3f 启动行含 /wesh/s/ + S3g 页面 200 实证 |
| auth.go basicAuth / server Attach | throttle per-IP 计数器 | proxyInfo.clientIP 单一换键 | ✓ WIRED | TestXFFThrottleKey 双 XFF 独立计数 + 对照组共享；S4b 日志 remote=XFF 链首 |
| main.go --auth-header | server New() proxyInfo | Options.AuthHeader 生产直传 | ✓ WIRED | main.go:1114 Options 接线 → server.go:330 装配 |
| main.go --stop-signal/--stop-timeout | server Options → clients.go 收口 | 单一 Options 通道 | ✓ WIRED | Options 接线在码；S5 真实宽限补 KILL 实证；Shutdown 复用同字段 |
| main.go --uid/--gid | pty Start → SysProcAttr.Credential | StartOptions 直通 | ✓ WIRED | S6a id -u 全等实证 |
| main.go NotifyContext | Server.Shutdown → pty.SignalGroup | 触发源非 exitf 分支 | ✓ WIRED | S7 SIGTERM→1001→退出 255 实证；exitf 单一收口注释在码 |
| Shutdown 1001 广播 | main.ts onclose case 1001 | 关闭码+reason 前后端对齐 | ✓ WIRED | S7a 客户端实收 1001+reason；D11a-c 面板+不重连实证 |
| config.go loadFileConfig | parseArgs 合并段 | fileConfig 指针标量 nil 判定 | ✓ WIRED | S1a-g 配置生效/CLI 覆盖/列表替换实证 |
| README nginx 配方 | base-path 装配 + ping 保活 | proxy_read_timeout > --ping-interval | ✓ WIRED | README 配方两小节在文；A1 经本机 nginx 1.14.1 实测闭合 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| logEvent remote_user | proxyInfo.remoteUser(r) | 配置头名对应 HTTP 头（sanitize 后） | ✓（S4b 实收 alice） | ✓ FLOWING |
| logEvent remote / throttle 键 | proxyInfo.clientIP(r) | XFF 链首（ParseIP 闸）或 TCP 对端 | ✓（S4b/S4d 双向实证） | ✓ FLOWING |
| socket 权限位 | cfg.socketMode | --socket-mode 八进制 parse / 配置键 | ✓（S2b stat 0660） | ✓ FLOWING |
| 子进程身份 | StartOptions.Uid/Gid → Credential | --uid/--gid / 配置键 | ✓（S6a id -u 回读） | ✓ FLOWING |
| --open URL | shareURLRO/RW 拼串 | scheme+host:port+basePath+自生成 token | ✓（S8b argv 全等） | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| phase07 八场景全链（真实二进制） | `node web/uat/phase07.mjs`（HEAD e022a8b 重构建 /tmp/wesh-uat/wesh） | 34/34 PASS + 1 skipped（S8c 平台豁免）+ SEC 自净 34 detail 零命中 | ✓ PASS |
| 全仓测试（含竞态） | `go test -race -count=1 ./...` | 五包全绿 51s（cmd/proto/pty/server/web） | ✓ PASS |
| 既有九 UAT 脚本回归 | 逐脚本复跑 | 全部退出 0（02:12/12、03:18/18、04:10/10、05:28/28+1、05-dims:PASS、06:23/23+1、04-dom:37/37、05-dom:19/19、06-dom:37/37+1） | ✓ PASS |
| 静态检查 | `go vet ./...` + GOROOT gofmt -l | vet 0；gofmt 仅两 HEAD 既有漂移文件（deferred-items.md 登记，非本 phase 引入——零新增漂移口径成立） | ✓ PASS |
| 前端构建产物一致性 | `time pnpm -C web build` + git status | 1.05s 退出 0；dist 构建后零 diff（产物与库内一致） | ✓ PASS |
| CLI 契约可见性 | `wesh --help` | 13 新 flag 全在册（config/socket×3/base-path/auth-header/cwd/term/stop×2/uid/gid/open） | ✓ PASS |
| 1001 不重连行为锁 | `node web/uat/phase06-dom.mjs` D11 | D11a-c PASS（终态面板逐字文案+守候窗零新连接+重连上下文终止） | ✓ PASS |

### Probe Execution

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| 无 scripts/\*/tests/probe-\*.sh 约定探针 | — | 本 phase 验证面为 web/uat/\*.mjs 真实二进制 UAT（已如上全量执行） | N/A |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| OPS-01 | 07-02/07-07/07-08 | 监听配置：端口/绑定/UNIX socket（含属主） | ✓ SATISFIED | S2a-f 全链 + listenSocket 在码 + TestListenSocket 五子测 |
| OPS-02 | 07-01/07-07/07-08 | 反代子路径挂载（base-path） | ✓ SATISFIED | S3a-h 全链 + basepath_test.go + 前端相对 URL 三改 |
| OPS-04 | 07-04/07-07/07-08 | 子进程 cwd/TERM/关闭信号可配（信号发进程组） | ✓ SATISFIED | S5a/b + SignalGroup 负 pid + StartOptions Dir/Term |
| OPS-05 | 07-04/07-07/07-08 | 降权运行（setuid/setgid） | ✓ SATISFIED | S6a/b + Credential + LookupId 身份改写 + 成对校验 |
| OPS-09 | 07-06/07-07/07-08 | 配置文件支持，CLI 参数覆盖配置文件 | ✓ SATISFIED | S1a-g + fileConfig 27 键 + 两阶段合并 + 优先级链测试 |
| OPS-11 | 07-05/07-07/07-08 | 可选启动后自动打开浏览器 | ✓ SATISFIED | S8a/b + openBrowser 三形态 + --socket×--open 拒绝 |
| SEC-07 | 07-03/07-07/07-08 | auth-header 透传（D-15 修订：服务端审计归因） | ✓ SATISFIED | S4a-d + sanitize/ParseIP + D-16 警告 + 正交回归锁 + 需求文本双文件修订 |

孤儿需求检查：REQUIREMENTS.md Traceability 表 Phase 7 映射恰为 OPS-01/02/04/05/09/11 + SEC-07 七条，与 plan frontmatter `requirements` 字段双向一致，无孤儿。

### 锁定决策抽查（用户指定关键项）

| 决策 | 要求 | Status | Evidence |
|------|------|--------|----------|
| D-01 | 零隐式配置路径（仅 --config 显式指定） | ✓ 遵守 | prescanConfigPath 仅扫 `--config=`/`--config ` 两形态、`--` 后即停（config.go:145-165）；裸启动零路径搜索 |
| D-03 | token/ticket/凭据永不入 logEvent（红线） | ✓ 遵守 | logEvent 红线注释随第四字段延伸（server.go:1063-1069）；CR-03 凭据头名 parse 期拒绝（main.go:603-608）；phase07 SEC 自净 34 detail 零命中运行时自证 |
| D-17 | auth-header 正交（只记录不做认证决定） | ✓ 遵守 | proxy.remoteUser 仅进 logEvent；TestAuthHeaderNoAuthBypass 伪造头 401 照旧回归锁；S4 全链认证语义零改动 |
| D-20 | XFF 单一信任闸零双轨 | ✓ 遵守 | proxyInfo.trust = AuthHeader!="" 唯一开关（server.go:330）；clientIP 仅 trust 时读 XFF；S4d 对照组 XFF 完全忽略 |
| D-23 | 1001 不进重连触发集 | ✓ 遵守 | main.ts case 1001 终态面板不调 startReconnect；D11b 守候窗零新连接构造实证；仅 1006 触发 |

### 代码评审 6 项修复回退核验（CR-01..IN-01）

| 修复 | 核验 | Status |
|------|------|--------|
| CR-01 listenSocket Lstat 类型闸 | main.go:1016-1023 在码（非 socket 拒绝启动、文件零触碰）；S2f 真实文件占位拒绝+内容保留实证；S2a 真实残留 socket 清理实证 | ✓ 无回退 |
| CR-02 XFF 链首 ParseIP 校验 | proxy.go:93 在码；TestProxyClientIP 垃圾/CSI/zone 六行 -race 绿 | ✓ 无回退 |
| CR-03 凭据头名拒绝 | main.go:603-608 CanonicalHeaderKey 归一拒四名在码；config 来源同闸（fc.AuthHeader 经默认值替换落同一终值） | ✓ 无回退 |
| WR-01 关停期第二次信号强杀 | main.go:1183-1186 goroutine 内 stopSignals()→Shutdown 在码（NotifyContext 官方形态） | ✓ 无回退 |
| WR-02 配置内 once 矛盾合并期拒绝 | main.go:533-546 在码；CLI 覆盖语义不受影响（TestConfigMerge 既有行绿） | ✓ 无回退 |
| IN-01 origin 错误值剥离 | main.go:723 detail 改 `key "origin"` 在码；TestConfigRedLines 子串断言锁 | ✓ 无回退 |

修复后回归面：`go test -race` 五包全绿（当前 HEAD）；phase07.mjs 于 HEAD 重构建二进制 34/34——修复未引入 must_haves 回退。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| 无 | — | TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER 扫描九文件零命中；空实现/硬编码空数据扫描零命中 | — | — |

既有事项（非本 phase 引入，已登记路由）：internal/server/multi_test.go 与 slowclient_test.go 两 HEAD 既有 GOROOT gofmt 漂移（deferred-items.md 2026-08-26 登记 open，style 提交清零路由）——零新增漂移口径下不判 gap。

### Human Verification Required

仅余平台原生行为豁免类人工项（CODEBUDDY.md 测试策略第 5 条既定豁免），全部已清单化于 07-UAT.md：

1. **A1 真实浏览器 --open 弹窗** — 协议层等价物 S8a/S8b 已覆盖；真实 GUI 观感人工确认
2. **A2 真实 nginx 反代观感** — 协议层 S3 全链已覆盖；nginx 匹配语义 A1 已经本机 nginx 1.14.1 实测闭合
3. **B4 root 降权 nobody** — 自动断言降权 self 已覆盖（S6）；root→nobody + 附加组清空需 root 人工
4. **B6 macOS open 与 TLS 组合** — darwin 分支构建标签差异，需 macOS 人工
5. **B1/B2/B3/B5 flagged assumptions 复核** — 边缘语义均有自动化覆盖，属真实部署一致性复核

### Gaps Summary

无 gap。七条需求（OPS-01/02/04/05/09/11 + SEC-07）全部经「静态符号 + 装配接线 + 真实二进制行为」三层核验落地；ROADMAP 三条成功准则（SC2 按 D-15 修订后文本）逐条达成；D-01/D-03/D-17/D-20/D-23 抽查全守；评审六项修复无回退；十 UAT 脚本 + go test -race 全绿。仅余平台豁免类人工项待 UAT 阶段执行，故 status=human_needed 而非 passed。

---

_Verified: 2026-08-26T09:05:00Z_
_Verifier: Claude (gsd-verifier)_
