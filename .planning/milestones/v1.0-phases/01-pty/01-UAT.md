---
status: complete
phase: 01-pty
source: [01-VERIFICATION.md]
started: 2026-08-14T03:54:12Z
updated: 2026-08-14T09:20:00Z
---

## Current Test

[testing complete]

## Tests

### 1. 浏览器交互四项（VALIDATION 1-01-08/09 + 成功准则整体确认）

test: |
  `go run ./cmd/wesh -- bash` 开 http://localhost:7681：
  (a) 键入命令看回显；
  (b) web shell 内 `env` 确认白名单；
  (c) 第二标签页应见 "Unable to connect"；
  (d) `wesh -- vim` 拖窗口看 stty size 跟随；
  (e) DevTools 禁 WebGL 刷新看 DOM 回落；
  (f) web shell 内 `exit` 看 "Session ended" 且 shell 侧 `echo $?` 为子进程退出码
expected: 四项全部符合预期
result: pass
verified_at: 2026-08-14
note: 用户浏览器实测全部通过（含 1d vim resize 跟随、1e WebGL 回落、1f exit 语义）。vim 退出后光标不闪经诊断为标准终端行为（terminfo cnorm=\E[?12l），非 bug。

### 2. UI-SPEC 五态视觉确认

test: |
  观察 loading（无白闪）/populated/overflow（scrollback 滚动条）/error（三态面板）/long-text（480px 折行）
expected: 无白闪、面板 480px 内折行无截断、scrollback 滚动条可用、遮罩下终端可读
result: pass
verified_at: 2026-08-14
note: 用户浏览器实测五态全部通过。期间发现 Reload 链接点击无反应，已修复（显式 location.reload() + #status z-index 1000，commit cb3c2cc）并复测通过。

### 3. CI 首次运行（含 macOS kqueue Q1 裁决）

test: |
  推送仓库，观察 GitHub Actions：ubuntu/macos 双 leg `go vet` + `go test -race -count=1 ./...`，web job `pnpm install --frozen-lockfile && pnpm -C web build`；macos leg 的 TestKqueueExitNormal/TestKqueueExitZombieRace 结果
expected: 三面全绿；若 TestKqueueExitZombieRace 出现 Q1-VERDICT skip，执行计划内兜底（awaitExit 退化为直接 cmd.Wait()）
result: pass
verified_at: 2026-08-14
note: |
  仓库托管 GitHub（github.com/sworda/wesh，SSH 推送），CI 三面全绿（ubuntu-latest + macos-latest + web）。
  **Q1 裁决 = watcher 成立**（取最优路径）：
  - TestKqueueExitNormal PASS (0.12s)——正常路径 NOTE_EXIT 事件到达 + 退出码 42
  - TestKqueueExitZombieRace PASS (1.09s)——走 <-exited 分支非 SKIP：kqueue 对僵尸进程补发 NOTE_EXIT，共享 watcher 无注册竞态
  - 兜底路径（awaitExit 退化为 cmd.Wait()）保持休眠，共享 kqueue watcher 经运行时验证，Phase 5 可放心基于其构建
  - 为读裁决给 CI go test 加了 -v（commit a99980b，包级 ok 无法区分 PASS/SKIP）

### 4. judgment-tier prohibition 1/3：wesh 无第三方运行时网络请求

test: |
  人工确认。验证者非权威 grep 证据：`cmd/ internal/ web/src/` 全文无 http.Get/Post/Do/NewRequest、fetch、XMLHttpRequest、http.Client；前端仅 new WebSocket(location.host)；dist/index.html 零外部 URL
expected: 确认成立
result: pass
source: automated
verified_at: 2026-08-14T04:53:03Z
evidence: |
  - `grep -rnE "http\.(Get|Post|Do|NewRequest)|http\.Client|fetch\(|XMLHttpRequest|axios|got\." cmd/ internal/ web/src/` 排除 _test.go → 0 hits
  - `grep -rnE "new WebSocket" web/src/` → 唯一命中 `web/src/main.ts:65` `new WebSocket('ws://' + location.host + '/ws')`（绑死 location.host，无外部主机）
  - `grep -oE 'https?://[^"'"' ]+' web/dist/index.html` 排除 w3.org DTD → 0 hits

### 5. judgment-tier prohibition 2/3：终端 I/O 不入日志不落盘

test: |
  人工确认。验证者非权威 grep 证据：非测试生产代码无 log./slog/WriteFile/Create 调用（仅 reap_darwin.go 一处 TODO(Phase 8) 注释提及 slog）；onChunk 数据面仅写 WS
expected: 确认成立
result: pass
source: automated
verified_at: 2026-08-14T04:53:03Z
evidence: |
  - `grep -rnE "\blog\.(Print|Fatal|Panic)|\bslog\.|os\.WriteFile|os\.Create|ioutil\.WriteFile|fmt\.Fprintln\(os\.(Stderr|Stdout)" cmd/ internal/ web/` 排除 _test.go 与构建产物 → 0 hits
  - `cmd/wesh/main.go` 中 `fmt.Fprintf/Printf` 全部为 CLI 契约输出（usage / version / `listening on ...` 恰一行 / 错误退出），无终端 I/O 经过
  - `internal/server/server.go:122 onChunk` 数据面只 `conn.Load()` + 后续仅 WS 写入，无日志/落盘

### 6. judgment-tier prohibition 3/3：前端运行时零外部资源（离线可用单 HTML）

test: |
  人工确认。验证者非权威 grep 证据：dist/index.html 无 http(s) 引用、无 <script src= 外链、无 webfont（system-ui 字体栈）；vite-plugin-singlefile 全量内联 + go:embed 硬约束
expected: 确认成立
result: pass
source: automated
verified_at: 2026-08-14T04:53:03Z
evidence: |
  - `grep -cE '<script[^>]*src=|<link[^>]*href=|@import|url\(https?' web/dist/index.html` → 0
  - `grep -E 'src=|href=|@import' web/index.html` 排除 `/src/` 与 `data:` → 0 external
  - vite-plugin-singlefile 全量内联 + `web/embed.go` `//go:embed all:dist` 硬约束

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
