---
phase: 01-pty
verified: 2026-08-14T03:50:14Z
status: human_needed
score: 19/27 must-haves verified
behavior_unverified: 8
overrides_applied: 0
behavior_unverified_items:
  - truth: "浏览器打开页面即获得完整交互终端（xterm 渲染、键盘输入可用）"
    test: "go run ./cmd/wesh -- bash，浏览器开 http://localhost:7681，键入命令观察回显与实时输出；web shell 内执行 env 确认只见白名单变量；开第二个标签页应见 Unable to connect"
    expected: "终端渲染正常、输入输出双向实时；env 无服务端机密；第二标签显示三态面板"
    why_human: "xterm 渲染与键盘交互需真实浏览器；WS 数据面已由 TestEchoPTY 行为证明，渲染一半无法自动化"
  - truth: "前端 fit 自适应且远端 vim/htop 随 resize 正确重绘"
    test: "wesh -- vim 打开后拖动浏览器窗口，远端 stty size 跟随变化"
    expected: "stty size 跟随窗口变化，vim 无闪屏错乱"
    why_human: "TUI 重绘质量是视觉判断；TIOCSWINSZ 机制已由 TestResize 行为证明"
  - truth: "macOS kqueue（EVFILT_PROC/NOTE_EXIT）运行时收割"
    test: "推送触发 CI macos-latest leg，观察 TestKqueueExitNormal / TestKqueueExitZombieRace 结果"
    expected: "正常路径事件到达 + 退出码 42；竞态路径事件到达则 watcher 成立，Q1-VERDICT skip 则执行计划内兜底（awaitExit 退化为直接 cmd.Wait()）"
    why_human: "本机无 macOS（RESEARCH Environment Availability），运行时裁决由 CI 承担是 plan 01-04 既定设计"
  - truth: "UI-SPEC loading 态：WS 建立期间空黑终端、无白闪、无 spinner"
    test: "浏览器打开页面观察 WS 建立瞬间"
    expected: "黑底空终端即现，无白闪"
    why_human: "视觉时序判断；内联 CSS + singlefile 构建已静态验证"
  - truth: "UI-SPEC error 态：三态面板文案与遮罩下终端可读"
    test: "触发三种关闭路径（未连上 / exit 终结 / 异常断开）观察面板"
    expected: "分别显示 Unable to connect / Session ended / Connection lost，遮罩下终端输出可读"
    why_human: "视觉判断；三态文案字符串与 onerror/onclose 接线已静态验证"
  - truth: "UI-SPEC overflow 态：10000 行 scrollback 与原生滚动条、长行折行"
    test: "web shell 内产生超屏输出（如 seq 1 20000）并滚动"
    expected: "滚动条可用、scrollback 10000 行、长行按 cols 折行"
    why_human: "滚动交互需真实浏览器；scrollback: 10000 选项已静态验证"
  - truth: "FE-01：WebGL 失败或 GPU 上下文丢失自动回落 DOM 渲染器（backstop truth）"
    test: "DevTools 禁用 WebGL 后刷新页面；恢复正常后刷新"
    expected: "禁用时终端仍渲染可交互（DOM 回落）；恢复后渲染正常"
    why_human: "verification: backstop——非推断性 truth，需显式证据；onContextLoss 注册代码已静态验证但 GPU 上下文行为无法自动化"
  - truth: "Q1 darwin 僵尸注册竞态由 CI 双测试裁决"
    test: "CI macos leg 运行 TestKqueueExitZombieRace"
    expected: "事件到达（watcher 成立）或 t.Skip + Q1-VERDICT 标记（兜底退化）——两条出路均为计划内路径"
    why_human: "测试为 CI-only（//go:build darwin），本机 Linux 由 build tag 排除，需推送后 CI 执行"
human_verification:
  - test: "浏览器交互四项（VALIDATION 1-01-08/09 + 成功准则整体确认）"
    expected: "交互终端可用 + env 白名单 + 第二标签 409 面板；vim resize 跟随；WebGL 禁用回落 DOM；exit 后 Session ended 且服务端以子进程退出码退出"
    why_human: "渲染正确性、TUI 重绘质量、GPU 回落均为视觉/交互判断，plan 01-05 已设计为 end-of-phase 人工验证（human_verify_mode=end-of-phase）"
  - test: "UI-SPEC 五态视觉确认（loading/populated/overflow/error/long-text）"
    expected: "无白闪、面板 480px 内折行无截断、scrollback 滚动条可用、遮罩下终端可读"
    why_human: "视觉呈现 grep 不可证"
  - test: "推送触发 CI 首次运行：ubuntu+macos 双 leg go vet/-race 全测 + web 构建"
    expected: "三面全绿；macos leg 输出 Q1 裁决（watcher 成立或兜底 skip）"
    why_human: "远端 CI 执行需推送动作，plan 01-05 明示不在执行断言范围"
  - test: "judgment-tier prohibition 人工确认 1/3：wesh 无第三方运行时网络请求（无遥测/上报）"
    expected: "确认成立。验证者证据（非权威）：cmd/ internal/ web/src/ 全文无 http.Get/Post/Do/NewRequest、fetch、XMLHttpRequest、http.Client；前端仅 new WebSocket(location.host)；dist/index.html 零外部 URL"
    why_human: "plan 标记 verification: judgment（descriptor-less），交互式验证需人工逐项裁决，绝不静默通过"
  - test: "judgment-tier prohibition 人工确认 2/3：终端 I/O 不写入日志、不持久化磁盘"
    expected: "确认成立。验证者证据（非权威）：非测试生产代码无 log./slog/WriteFile/Create 调用（仅 reap_darwin.go 一处 TODO(Phase 8) 注释提及 slog）；onChunk 数据面仅写 WS"
    why_human: "同上，judgment-tier 需人工裁决"
  - test: "judgment-tier prohibition 人工确认 3/3：前端运行时零外部资源（离线可用单 HTML）"
    expected: "确认成立。验证者证据（非权威）：dist/index.html 无 http(s) 引用、无 <script src= 外链、无 webfont（system-ui 字体栈）；vite-plugin-singlefile 全量内联 + go:embed 硬约束"
    why_human: "同上，judgment-tier 需人工裁决"
---

# Phase 01: 行走骨架（核心 PTY 管道）Verification Report

**Phase Goal:** 用户运行 `wesh -- <command>` 后在浏览器获得一个可用的完整交互终端
**Verified:** 2026-08-14T03:50:14Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth   | Status     | Evidence       |
| --- | ------- | ---------- | -------------- |
| 1 | SC1a：WS 双向实时数据面（INPUT→PTY→OUTPUT 同字节回环，CORE-01） | ✓ VERIFIED | TestEchoPTY/TestSecondClient409 行为测试绿（本验证复跑）；冒烟 GET / 200 且伺服真实构建 HTML |
| 2 | SC1b：浏览器打开页面即获得完整交互终端（xterm 渲染、键盘可用） | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | main.ts onmessage→term.write / onData→ws.send 接线在；浏览器渲染行为无测试覆盖，见人工验证 1 |
| 3 | SC2a：RESIZE→钳制[1,1000]→TIOCSWINSZ 服务端链路（CORE-02） | ✓ VERIFIED | TestResize 行为测试绿（stty 序列 24 80→50 132）；proto.DecodeResize/ClampDim 在 server reader 回路中接线 |
| 4 | SC2b：前端 fit 自适应 + vim/htop 随 resize 重绘（FE-03） | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | FitAddon+100ms debounce+sendResize 防护代码在；TUI 重绘质量需人工，见人工验证 1 |
| 5 | SC3a：Linux 收割零额外线程、无僵尸（pidfd waitid） | ✓ VERIFIED | TestReap 绿（20 次循环 /proc/\<pid\> 消失）；reap_linux.go=cmd.Wait()（stdlib pidfd） |
| 6 | SC3b：macOS kqueue 收割运行时成立 | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | reap_darwin.go 编译期完备且交叉编译绿；运行时裁决需 CI macos leg，见人工验证 3 |
| 7 | SC3c：env 白名单最小集（SEC-06） | ✓ VERIFIED | TestEnvWhitelist 双层绿（白名单函数 + 子进程 /usr/bin/env 实际输出，宿主注入 AWS_SECRET_ACCESS_KEY 不可见） |
| 8 | argv exec 数组绝不经 shell（$(id) 字面量） | ✓ VERIFIED | TestExecArrayNoShell 绿；spawn.go exec.Command(argv[0], argv[1:]...) |
| 9 | spawn 失败不伤服务端 fd 0/1/2（ttyd close(0) 回归） | ✓ VERIFIED | TestSpawnFailKeepsStdio 绿（Fsync 探测非 EBADF） |
| 10 | D-12：attach 前 ReadLoop 持续 drain（防 64KiB 内核缓冲写阻塞） | ✓ VERIFIED | TestDrainBeforeAttach 绿（1.3MB 输出无客户端照常退出）；ReadLoop 启动点在 New 内，Attach 内无 ReadLoop |
| 11 | D-09：第二 WS 连接 Accept 前收 HTTP 409 | ✓ VERIFIED | TestSecondClient409 绿（409 且首连接 echo 不受影响） |
| 12 | D-10：子进程退出 → 客户端先收 1000 → exitf(子进程退出码) | ✓ VERIFIED | TestExitCodePropagation 绿（exit 42 → 1000 → exitf(42)）；childExited 标志消除 D-11 竞态（01-03 修复） |
| 13 | D-11：WS 断开 → SIGHUP 进程组（负 pid）→ exitf(0) | ✓ VERIFIED | TestClientDisconnectSIGHUP 绿（落盘 GOT_SIGHUP 标记）；server.go syscall.Kill(-pid, SIGHUP) |
| 14 | 未知类型帧 → 1002 关闭且全程无 1006 | ✓ VERIFIED | TestUnknownFrame1002 绿 |
| 15 | CLI 契约（D-02~D-06：-- 透传、默认值 0.0.0.0:7681、无命令 usage 退 2、--version） | ✓ VERIFIED | TestParseArgs/TestNoCommandError/TestVersionFlag 绿 |
| 16 | UI-SPEC loading 态（黑底无白闪无 spinner） | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | index.html 内联 CSS（background:#000000）+ singlefile 构建在；视觉时序需人工，见人工验证 2 |
| 17 | UI-SPEC error 态（三态面板文案、遮罩下可读） | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | 三条文案与 showStatus 接线在、dist bundle 含三字符串；视觉需人工 |
| 18 | UI-SPEC overflow 态（10000 scrollback、原生滚动条、长行折行） | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | scrollback:10000 选项在；滚动行为需人工 |
| 19 | FE-01：WebGL 失败/上下文丢失自动回落 DOM（backstop） | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | WebglAddon try/catch + onContextLoss(dispose) 在；verification:backstop 需显式人工证据 |
| 20 | darwin amd64/arm64 交叉编译 + vet | ✓ VERIFIED | 本验证复跑 GOOS=darwin 双架构 build/vet 全绿 |
| 21 | Q1 竞态裁决双测试经 CI 运行 | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | reap_darwin_test.go 双测试就位（CI-only）；未推送未运行，见人工验证 3 |
| 22 | CI 双平台配置（ubuntu+macos -race；web pnpm 构建；无 CGO_ENABLED=0） | ✓ VERIFIED | ci.yml 结构核实：matrix 双 os、go vet、go test -race -count=1、pnpm install --frozen-lockfile、pnpm build；actions 全钉版 |
| 23 | README 首屏无认证警示（D-05 补偿控制） | ✓ VERIFIED | README.md 行 5-7 引用块警示，位置仅次于标题段 |
| 24 | README 单次语义说明（WS 断开即退出为预期行为） | ✓ VERIFIED | README.md 行 9-11 |
| 25 | README 构建顺序（pnpm 先于 go build）+ SEC-06 白名单集合 | ✓ VERIFIED | README.md 行 32-46 |
| 26 | 干净检出 embed 占位链（裸 clone 可 build/test） | ✓ VERIFIED | 本验证 git archive → /tmp 干净目录 go build + go test 全绿（已清理） |
| 27 | 全量验证：-race 全测 + pnpm build + 构建可重现 | ✓ VERIFIED | 本验证复跑全绿；pnpm build 后 git status 干净（产物字节级确定） |

**Score:** 19/27 truths verified（8 项 present + wired 但行为未经测试行使，全部转入人工验证）

### Required Artifacts

| Artifact | Expected    | Status | Details |
| -------- | ----------- | ------ | ------- |
| `go.mod` | module github.com/sworda/wesh | ✓ VERIFIED | 首行匹配；go 1.26.3 |
| `cmd/wesh/main.go` | CLI 解析 + 装配（main/run/parseArgs） | ✓ VERIFIED | 88 行实质实现；pty.Start→net.Listen→server.New(os.Exit)→单行打印→http.Serve |
| `internal/proto/proto.go` | 协议帧单一事实源 | ✓ VERIFIED | Input/Resize/Output 常量 + DecodeResize/ClampDim，钳制 [1,1000] |
| `internal/pty/spawn.go` | exec 数组 + env 白名单 + 80x24 | ✓ VERIFIED | Start/Session；whitelistEnv 替换式注入，无 os.Environ() 追加 |
| `internal/pty/io.go` | ReadLoop/Resize/Close/drain | ✓ VERIFIED | EIO/EOF 归一；Setsize；Drain(200ms) 时限 |
| `internal/pty/reap_linux.go` | awaitExit=cmd.Wait() | ✓ VERIFIED | `//go:build linux` + pidfd 文档注释 |
| `internal/pty/reap_darwin.go` | 共享 kqueue watcher | ✓ VERIFIED | EVFILT_PROC/NOTE_EXIT/EV_ONESHOT；签名与 linux 统一；注册失败摘 subs 防泄漏 |
| `internal/server/server.go` | WS 网关 + 数据泵 + 生命周期 | ✓ VERIFIED | New 钉死 ReadLoop/lifecycle 两启动点；409/1000/1002/SIGHUP/sync.Once 全在 |
| `internal/server/e2e_test.go` | echo + drain + 生命周期四测 | ✓ VERIFIED | 6 个测试函数全部绿（含 -race） |
| `internal/pty/spawn_test.go` | spawn 三专项 | ✓ VERIFIED | 3 测试绿 |
| `internal/pty/io_test.go` | TestResize | ✓ VERIFIED | 绿 |
| `internal/pty/reap_test.go` | TestReap（linux） | ✓ VERIFIED | `//go:build linux`，绿 |
| `internal/pty/reap_darwin_test.go` | Q1 双测试（CI-only） | ✓ VERIFIED | 编译通过；运行时待 CI |
| `cmd/wesh/main_test.go` | CLI 三测 | ✓ VERIFIED | 绿 |
| `web/embed.go` | go:embed all:dist + gzip 协商 | ✓ VERIFIED | 冒烟实证 gzip 双路径 |
| `web/dist/index.html` | 真实单 HTML 构建产物 | ✓ VERIFIED | 456KB，零外链，含三态文案；pnpm build 复建字节一致 |
| `web/src/main.ts` | xterm + WS + fit + 三态面板 | ✓ VERIFIED | 165 行实质实现；Terminal Options 全钉死（16 色 theme/scrollback/cursorBlink） |
| `web/index.html` | 页面壳契约 | ✓ VERIFIED | 两顶层元素 + 内联 CSS + 无外部资源 |
| `web/package.json` | 依赖钉死 | ✓ VERIFIED | @xterm 6.0.0/addon-fit 0.11.0/addon-webgl 0.19.0；vite 8.2.1/singlefile 2.3.3 |
| `.github/workflows/ci.yml` | 双平台 CI | ✓ VERIFIED | go 矩阵 + web job；无 CGO_ENABLED=0 |
| `README.md` | 警示/单次语义/构建/白名单 | ✓ VERIFIED | 全部强制内容就位 |

### Key Link Verification

| From | To  | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| cmd/wesh/main.go | internal/pty/spawn.go | `pty.Start(argv)` 行 66 | ✓ WIRED | 读码确认 |
| cmd/wesh/main.go | internal/server/server.go | `server.New(sess, os.Exit)` 行 76 | ✓ WIRED | exitf=os.Exit 注入 |
| internal/server/server.go | internal/pty/io.go | `go sess.ReadLoop(s.onChunk)` 行 51（New 内） | ✓ WIRED | Attach 函数体内无 ReadLoop（读码确认） |
| internal/server/server.go | internal/proto/proto.go | reader switch：proto.Input→master.Write / proto.Resize→DecodeResize→sess.Resize 行 106-111 | ✓ WIRED | 未知类型 1002 |
| web/src/main.ts | internal/proto/proto.go | OUTPUT/INPUT=0x30、RESIZE=0x31 行 7-9 | ✓ WIRED | 与 proto 常量一致（D-16） |
| web/embed.go | web/dist/index.html | `//go:embed all:dist` 行 15 | ✓ WIRED | 冒烟实证伺服真实产物 |
| web/src/main.ts | /ws | `new WebSocket('ws://'+location.host+'/ws')` 行 65 | ✓ WIRED | binaryType=arraybuffer |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| server.onChunk | chunk | PTY master Read（32KiB 循环） | ✓ 真实 PTY 输出 | ✓ FLOWING（TestEchoPTY 行为证明） |
| server reader | data[1:] | WS INPUT 帧 | ✓ 真实键盘输入→master.Write | ✓ FLOWING |
| sess.Resize | cols/rows | RESIZE 帧 JSON→DecodeResize 钳制 | ✓ 真实尺寸→TIOCSWINSZ | ✓ FLOWING（TestResize 行为证明） |
| main.ts term.write | buf.subarray(1) | WS OUTPUT 帧 | ✓ 无静态/模拟数据 | ✓ FLOWING |
| main.ts sendResize | term.cols/rows | FitAddon 实际布局 | ✓ 含整数防护 | ✓ FLOWING |

无 STATIC/DISCONNECTED/HOLLOW_PROP 项。

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| 静态检查 | `go vet ./...` | exit 0 | ✓ PASS |
| 全量测试 | `go test ./... -count=1` | cmd/wesh、internal/pty、internal/server 全 ok | ✓ PASS |
| 竞态检测 | `go test -race -count=1 ./...` | 全 ok 无竞态报告 | ✓ PASS |
| darwin 交叉编译 | `GOOS=darwin GOARCH=arm64 go build ./... && GOOS=darwin GOARCH=amd64 go vet ./...` | exit 0 | ✓ PASS |
| 运行冒烟 | 构建二进制 + `--port 17681 -- /bin/cat` | 单行 `listening on http://[::]:17681`；GET / 200；gzip 协商命中 Content-Encoding: gzip；页面含 `<title>wesh</title>`；/ws 非升级 426；进程已清理无残留 | ✓ PASS |
| 裸 clone embed 链 | `git archive` → /tmp 干净目录 `go build && go test` | 全绿 | ✓ PASS |
| 前端构建可重现 | `pnpm -C web build` 后 `git status --short web/` | 构建 exit 0；git status 干净（字节级确定） | ✓ PASS |
| dist 无外部引用 | grep `<script src=` / `https?://` dist/index.html | 均 0 命中；含三条三态文案 | ✓ PASS |

注：启动行渲染 `[::]` 为 01-05 已记录的本机工具链双栈行为（curl 127.0.0.1 实测连通），D-07 契约（恰一行、含实际地址端口）满足。

### Probe Execution

SKIPPED — 本阶段非 migration/tooling 阶段，PLAN/SUMMARY 未声明 probe，`scripts/` 不存在。

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| CORE-01 | 01-01/01-03/01-04/01-05 | CLI 指定任意命令，浏览器获完整交互终端（PTY 双向转发） | ✓ SATISFIED | TestEchoPTY + 生命周期五测 + CLI 三测全绿；冒烟实证 |
| CORE-02 | 01-01/01-02 | 前端 resize 时服务端 TIOCSWINSZ 同步 | ✓ SATISFIED | TestResize 行为证明（24 80→50 132）；前端 RESIZE 回路接线在 |
| FE-01 | 01-01/01-05 | xterm.js 6 渲染，WebGL 失败自动回落 DOM | ? NEEDS HUMAN | 代码在（try/catch + onContextLoss）；GPU 回落行为需真实浏览器（人工验证 1） |
| FE-03 | 01-01/01-05 | 浏览器窗口变化终端自动 fit | ? NEEDS HUMAN | 代码在（FitAddon+debounce+防护）；浏览器行为需人工（人工验证 1） |
| SEC-06 | 01-01/01-02 | 子进程 env 白名单不继承服务端全量 env | ✓ SATISFIED | TestEnvWhitelist 双层行为证明（含阳性对照防假绿） |

Orphaned requirements：无。REQUIREMENTS.md 映射 Phase 1 的恰为 CORE-01/CORE-02/FE-01/FE-03/SEC-06 五条，全部在 plan frontmatter 中出现并被验证覆盖。

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| internal/pty/reap_darwin.go | 72 | `TODO(Phase 8): 接 slog` | ℹ️ Info | 引用 ROADMAP Phase 8 正式后续工作，非阻断 |
| .planning/phases/01-pty/01-VALIDATION.md | 48-49 | 状态表 1-01-06/07 仍标 pending（实际测试已绿） | ℹ️ Info | 文档簿记漂移，非代码缺陷；建议 validate-phase 时刷新 |
| （全仓扫描） | — | TBD/FIXME/XXX/PLACEHOLDER/空实现/硬编码空数据 | 无命中 | 阻断级债务标记为零 |

### Human Verification Required

#### 1. 浏览器交互四项（VALIDATION 1-01-08/09 + 成功准则整体确认）

**Test:** `go run ./cmd/wesh -- bash` 开 http://localhost:7681：键入命令看回显；web shell 内 `env` 确认白名单；第二标签页应见 "Unable to connect"；`wesh -- vim` 拖窗口看 stty size 跟随；DevTools 禁 WebGL 刷新看 DOM 回落；web shell 内 `exit` 看 "Session ended" 且 shell 侧 `echo $?` 为子进程退出码
**Expected:** 四项全部符合预期
**Why human:** 渲染/TUI 重绘/GPU 回落为视觉交互判断，plan 01-05 既定 end-of-phase 人工验证

#### 2. UI-SPEC 五态视觉确认

**Test:** 观察 loading（无白闪）/populated/overflow（scrollback 滚动条）/error（三态面板）/long-text（480px 折行）
**Expected:** 符合 UI-SPEC 契约
**Why human:** 视觉呈现 grep 不可证

#### 3. CI 首次运行（含 macOS kqueue Q1 裁决）

**Test:** 推送仓库，观察 GitHub Actions：ubuntu/macos 双 leg `go vet` + `go test -race -count=1 ./...`，web job `pnpm install --frozen-lockfile && pnpm -C web build`；macos leg 的 TestKqueueExitNormal/TestKqueueExitZombieRace 结果
**Expected:** 三面全绿；若 TestKqueueExitZombieRace 出现 Q1-VERDICT skip，执行计划内兜底（awaitExit 退化为直接 cmd.Wait()）
**Why human:** 远端 CI 需推送动作；本机无 macOS

#### 4-6. 三条 judgment-tier prohibition 人工确认

**Test:** 逐项确认——(1) 无第三方运行时网络请求（遥测/上报）；(2) 终端 I/O 不入日志不落盘；(3) 前端运行时零外部资源
**Expected:** 三条均确认成立。验证者非权威 grep 证据已附于 frontmatter human_verification 条目，均支持"成立"
**Why human:** plan 标记 verification: judgment（descriptor-less），交互式验证要求人工逐项裁决——绝不静默通过

### Gaps Summary

无代码缺口。全部 21 个产物文件存在且实质、7 条关键接线全部贯通、27 条 truth 中 19 条经行为测试或命令实证、8 条 present+wired 但行为需人工（浏览器视觉 5 条、macOS/CI 运行时 2 条、backstop 1 条——其中 6 条本机 Linux 环境在结构上无法自动化）。另有 3 条 judgment-tier prohibition 需人工逐项确认（grep 证据均支持成立）。阶段目标的机制面（PTY 双向转发、resize 同步、收割、env 白名单、生命周期单次语义）已全部自动化证明；剩余为既定的 end-of-phase 人工确认项。

---

_Verified: 2026-08-14T03:50:14Z_
_Verifier: Claude (gsd-verifier)_
