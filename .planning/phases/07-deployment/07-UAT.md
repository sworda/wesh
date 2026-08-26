---
status: partial
phase: 07-deployment
created: 2026-08-26
source: [07-VERIFICATION.md, 07-VALIDATION.md, 07-08-PLAN.md]
started: 2026-08-26T07:10:00Z
updated: 2026-08-26T12:10:00Z
---

# Phase 7 人工 UAT 清单（部署与配置）

## Current Test

[testing complete]

自动化断言不可达项的人工复核清单：VALIDATION 登记的 manual-only 两项 + 各 plan flagged assumptions 复核项 + root 降权 nobody 可选场景。
每项 = 步骤 + 预期 + 勾选框。自动化已覆盖的协议层行为见 `web/uat/phase07.mjs`（八场景 33 断言）。

## A. Manual-Only 项（07-VALIDATION.md 登记）

- [ ] **A1. 浏览器自动打开真实效果（OPS-11）**— **blocked（平台拓扑限制）**（2026-08-26）
  - 步骤：在有 GUI 的环境（桌面 Linux / macOS / Windows 工作站）运行 `wesh --open --writable -- bash`。
  - 预期：启动后系统默认浏览器自动打开 rw 分享链接（含 token 免交互直接进入终端），无需手动复制 URL；终端可用。
  - 备注：macOS 真实弹窗为本机（Linux headless）未实测面——CI macOS 跑同款单测形态整体 Skip（07-05 flagged_assumptions）。
  - **blocked_by: physical-device** — reason: "--open 的拉起方为 wesh 运行机系统启动器。双机拓扑下 wesh 只能跑在 Linux 开发机（headless，真实弹窗不可达）；Windows 工作站有 GUI 但 wesh 不能 Windows 原生构建运行（既定 Out of Scope）。可自动化面已闭合：phase07.mjs S8a（headless 跳过提示+服务正常）PASS、S8b（fake xdg-open argv == rw 分享链接全等）PASS、b6.sh（stub opener + TLS 组合 https:// ro 链接）6/7 PASS；真实浏览器弹窗观感为平台原生行为，phase07.mjs S8c 已按 CODEBUDDY.md 平台豁免条款登记 skip+reason。若需彻底闭环需桌面 Linux/macOS 机器。"

- [ ] **A2. 真实 nginx 反代挂载观感（OPS-02）**— **issue: major**（2026-08-26 全链自动化实证，见 Gaps G-07-2）
  - 步骤：按 README「部署与配置 → 反代子路径」配方配置真实 nginx（`location = /wesh` 精确块 + `location /wesh/` 前缀块，后端 `wesh --base-path /wesh -- bash`）；浏览器访问 `http://<host>/wesh`（裸路径，无尾斜杠）。
  - 预期：308 重定向到 `/wesh/` 后页面正常加载、WS 升级成功、终端可用；idle >60s 连接不断（`proxy_read_timeout 3600s` > `--ping-interval 5s`）；裸路径不经精确块时 404（复核精确块必要性）。截图留档。
  - 实测（phase07-a2-pw.mjs，Windows Playwright Chromium → LAN 真实 nginx 1.14.1 → wesh，全链）：
    - **按 README 配方原样（无 Host 转发）**：裸 /wesh → 308 ✓、页面 200 ✓、但 **WS 升级 403**——nginx 默认转发 `Host: $proxy_host`（127.0.0.1:后端口），与浏览器 Origin（http://真实主机：端口）不同源，coder/websocket 库默认同源校验拒绝（origin.go:73 EqualFold(r.Host, u.Host)）；**跨机浏览器访问按文档部署即坏**。
    - **修正配方（+`proxy_set_header Host $http_host;`，$host 剥端口仍不匹配已实证）**：T1 308 ✓ / T2 页面+提示符 ✓ / T3 echo 全链 ✓ / T4 空闲 65s 无断连面板、终端仍可用 ✓ / T5 前缀块 /wesh/ 200 ✓。截图：`web/uat/pw/screenshots/a2-home.png`、`a2-idle.png`。
    - **T5 的 404 预期证伪（预期校准错误）**：无精确块时裸 /wesh 实测为 **301**（nginx 官方文档行为——prefix location 以 `/` 结尾且 handler 为 proxy_pass 时，裸路径自动 301 加斜杠）；C1 复核的 404 结论基于 `return 200` 探针（非 proxy_pass handler），不向真实配方推广。精确块仍有益（308 保方法、显式规范化），但「否则 404、必需非可选」的理据文案对 proxy_pass 形态不成立——并入 G-07-2 文案修正。
    - 附带确认：无 --credential 时 /api/attach 404 为既定设计（无认证模式探测信号，auth_e2e_test.go:220），前端照常走 WS。

## B. Flagged Assumptions 复核项（各 plan 登记，逐项列探针问题原文）

- [ ] **B1. OPS-01 并发中断语义**（07-02 登记）— **issue: major**（2026-08-26 自动化实证，见 Gaps G-07-3）
  - 探针原文：「If interrupted or run in parallel, what is guaranteed?」
  - 步骤：两个 wesh 实例同时以同一 `--socket` 路径启动；随后 kill 掉已 listen 的实例，再次启动。
  - 预期：后者 `bind: address already in use` exit 1（无静默赢者之外的保证）；kill 后再次启动时残留 socket 文件被自动清理（listen 前 Remove）、启动成功。
  - 实测（b1b5.sh @ Linux，HEAD e32ef03 构建）：**B1a 背离**——存活实例同路径第二实例**未**收 EADDRINUSE，而是 unlink 存活 socket 后 listen 成功（静默赢者，证据：b.log 首行 `listening on unix://...`）；B1c SIGKILL 后 socket 文件真实残留 ✓；B1d 残留自动清理启动成功 ✓。根因：`cmd/wesh/main.go:1023` Lstat 类型闸后**无条件** `os.Remove`——CR-01 收窄只拒非 socket 文件，存活 socket 与残留 socket 不可区分同被删。

- [x] **B2. SEC-07 多值头 / 空值头**（07-03 登记）— **pass**（2026-08-26 自动化实证，codebuddy）
  - 探针原文：「多值头（重复 X-Remote-User 头行）取 Header.Get 首值；空串头值 → sanitize 后空 → 不出键（与缺席同态）」
  - 步骤：`--auth-header X-Remote-User` 实例下，curl 携两个不同值的 `X-Remote-User` 头行请求；再携空串头值请求。
  - 预期：事件行 `remote_user` 取首值；空串头值时事件行不出现 `remote_user` 键（与缺席同态）。（proxy_test.go 表驱动已覆盖——人工复核真实反代行为一致性。）
  - 实测（b2.mjs @ Linux 4/4）：原始 socket 手构 WS Upgrade 携两行 `X-Remote-User`（alice+bob）+ 无效 ticket → 101 升级 + 1008 auth_failed，事件行 `remote_user=alice` 且 bob 零泄漏 ✓；空串头值 → 事件行无 `remote_user` 键（与缺席同态）✓。

- [x] **B3. OPS-04 symlink / TERM 任意值 / stop-timeout 极大值**（07-04 登记）— **pass**（2026-08-26 自动化实证，codebuddy）
  - 探针原文：「--cwd 为符号链接时按内核语义解析（不额外规范化）；--term 任意字符串不校验（TERM 值合法性由终端数据库承担，wesh 不立场）；--stop-timeout 极大值只推迟 KILL 不阻塞 exitf（AfterFunc 异步）」
  - 步骤：① `--cwd` 给符号链接路径启动；② `--term` 给非标准字符串（如 `--term foobar`）启动后终端内 `echo $TERM`；③ `--stop-timeout 1h` 触发关停后中途手动 kill 子进程。
  - 预期：① 正常启动，子进程 cwd 按内核语义解析；② 启动不校验，`$TERM` 原样为 `foobar`；③ 子进程死亡后 wesh 即时退出（不等满 1h——补 KILL 是异步兜底，不阻塞收口）。

- [ ] **B4. OPS-05 降权 nobody 无 shell 与附加组清空（root 可选场景）**（07-04 登记 + 07-04 SUMMARY 复核联动）— **blocked**（2026-08-26）
  - 探针原文：「降权到存在但无登录 shell 的 uid（如 nobody）时 shell 自默认行为由子进程命令承担；supplementary groups 不设置（清空附加组）——与『最小权限』一致，有意为之」
  - 步骤（需 root）：`sudo ./wesh --bind 127.0.0.1 --uid 65534 --gid 65534 -- bash`；浏览器 attach 后依次执行 `id`、`echo $HOME $USER $LOGNAME`；Ctrl 台 SIGTERM 关停。
  - 预期：`id` 输出 `uid=65534(nobody) gid=65534(nogroup)` 且**附加组清空**（groups 仅 nogroup——root 启动清空附加组是最小权限既定语义；非 root 降回自身则保留自身附加组，07-04 环境感知策略复核联动）；`HOME`/`USER`/`LOGNAME` 按 nobody passwd 条目改写（无条目则剔除三键、shell 自默认）；SIGTERM 后 1001 优雅下线序列、退出码 255。
  - **blocked_by: other** — reason: "需要 root 执行降权，但 Linux 开发机 sudo 损坏（/usr/bin/sudo → /usr/local/sa/tjj/bin/sudo-64-v30000.tlinux3 属主/suid 位异常，sudo -n 报 'must be owned by uid 0 and have the setuid bit set'），当前用户 zexueli(uid=51714) 非 root，无可提权通道。降权 self 面已由 phase07.mjs S6a/S6b 自动化覆盖（PASS），nobody 场景为 root-only 残余。"

- [x] **B5. OPS-09 TOML 语法变体与空 command**（07-06 登记）— **pass**（2026-08-26 自动化实证，codebuddy）
  - 探针原文：「TOML 多行数组/内联表等合法 TOML 语法按 go-toml 语义接受（平铺形状是约定不是语法强制）；配置重复键由 TOML 解析器拒绝（规范行为）；command 空数组 `command = []` 等价缺席」
  - 步骤：① 配置文件用多行数组/内联表写法；② 配置文件写重复键；③ 配置文件写 `command = []`。
  - 预期：① 正常加载（go-toml 语义接受）；② exit 2（TOML 解析器规范拒绝）；③ 等价缺席 → `missing command` exit 2（与 CLI `--` 空 argv 同档）。
  - 实测（b1b5.sh @ Linux 4/4）：① 多行数组 + 引用键 `"port" = 0` 配置正常加载 listening ✓；② 重复 `bind` 键 → exit 2，报 `invalid toml (key "bind" line 2)` ✓；③ `command = []` → exit 2 报 `missing command`（与 CLI 空 argv 同档）✓。注：配置 schema 为平铺（无嵌套表键），「内联表」维度以多行数组+引用键语法变体实证 go-toml 语义接受面。

- [ ] **B6. OPS-11 macOS open 与 TLS 组合**（07-05 登记）— **issue: minor**（2026-08-26 自动化实证可及面，见 Gaps G-07-8；macOS 真实弹窗面 blocked）
  - 探针原文：「macOS open 行为未在本机实测；xdg-open 存在但返回非零（桌面异常）只警告不阻断（D-27）；--open 与 TLS 组合打开 https:// 链接（自签证书浏览器警告属用户预期面）」
  - 步骤：① macOS 上 `wesh --open -- bash`；② `wesh --open --tls-cert <cert> --tls-key <key> -- bash`。
  - 预期：① `open` 命令拉起默认浏览器打开 ro 分享链接；② 打开 `https://` 链接，自签证书浏览器警告属用户预期面；xdg-open/open 返回非零时仅 stderr 警告、服务不阻断。
  - 实测（b6.sh @ Linux 6/7）：TLS 实例 scheme=https ✓；stub xdg-open（DISPLAY 置位）被调用且 argv == 启动打印的 ro 分享链接 ✓；**--open × TLS 打开 https:// 链接** ✓；opener 返回非零时服务仍启动且 GET / 200（不阻断 ✓）——但**无 stderr 警告行**：`openBrowser`（cmd/wesh/main.go:1244）为 fire-and-forget `.Start()`，非零退出不可观测（与 07-RESEARCH Pattern 8 配方逐字一致；与 plan D-27「返回非零只警告」字面有差——文档/实现对齐问题，见 Gaps G-07-8）。macOS `open` 真实弹窗面：**blocked**（无 Mac 环境；phase07.mjs S8c 已按平台豁免登记 skip）。

## C. RESEARCH A1 复核（已闭合——留档）

- [x] **C1. nginx `location /wesh/` 不匹配裸 `/wesh`**（07-RESEARCH A1，README 精确块必要性依据）
  - 复核结论：本机 nginx 1.14.1 实测（2026-08-26，07-08 执行期）——`location /wesh/ { return 200; }` 配置下 `GET /wesh` → **404**、`GET /wesh/` → **200**。A1 成立：`location = /wesh { return 308 /wesh/; }` 精确重定向块**必需**，README 配方据此确凿落文（非防御性建议）。

---

*自动化覆盖边界：协议层全场景（配置合并/unix socket/base-path/auth-header/XFF/stop-signal/降权 self/1001/--open）见 phase07.mjs；本清单仅收自动化不可达项。完成后勾选并注明日期与执行人。*

## Summary

total: 8
passed: 3（B2、B3、B5）
issues: 3（A2=major/G-07-2、B1=major/G-07-3、B6=minor/G-07-8）
pending: 0
skipped: 0
blocked: 2（A1=physical-device 平台拓扑、B4=other 无 root 通道）

另：phase07.mjs 协议层 34/34 PASS（S8c 平台豁免 skip 1 项）；A2 修正配方回归 phase07-a2-pw.mjs 4/5 PASS（T5 为预期校准错误已证伪改写）；B 组证据脚本已固化仓库（web/uat/phase07-b*.sh/.mjs、web/uat/pw/phase07-a2-*）。

## Gaps

- gap_id: G-07-2
  truth: "按 README「部署与配置 → 反代子路径」配方配置真实 nginx 后，浏览器经反代访问 /wesh 应页面加载、WS 升级成功、终端可用"
  status: failed
  reason: "全链实证（phase07-a2-pw.mjs，Windows Chromium → LAN nginx 1.14.1 → wesh）：按配方原样部署，页面 200 但 WS 升级 403——nginx 默认转发 Host=$proxy_host（127.0.0.1:后端口），浏览器 Origin（http://真实主机：端口）与之不同源，wesh 库默认同源校验拒绝；跨机浏览器访问按文档部署即坏。另：配方理据文案『裸路径不经精确块会 404』对 proxy_pass 形态不成立（实为 nginx 自动 301）"
  severity: major
  test: 2
  root_cause: "README 配方缺 proxy_set_header Host 行：nginx proxy_pass 默认 Host=$proxy_host（上游地址 127.0.0.1:port）；wesh originAllowed 以 EqualFold(r.Host, Origin.Host) 同源校验（internal/server/origin.go:73，coder/websocket 库默认语义）→ 反代后跨机访问必 403。修正实证：proxy_set_header Host $http_host;（$host 剥端口仍不匹配——origin 含非默认端口时必须 $http_host）后 attach 200 / WS 101 / 终端全链通"
  artifacts:
    - path: "README.md"
      issue: "反代子路径 nginx 配方缺 proxy_set_header Host $http_host;（L316-322 location /wesh/ 块）；精确块 404 理据文案与 proxy_pass 实际行为（自动 301）不符"
  missing:
    - "README 配方 location /wesh/ 块补 proxy_set_header Host $http_host; 一行 + 理据文案修正（301 自动跳转语义 vs 精确块 308 显式规范化）；建议配 phase07-a2-pw.mjs 全链回归（LAN nginx + Playwright）"
  debug_session: ""

- gap_id: G-07-3
  truth: "两个 wesh 实例以同一 --socket 路径启动时，后者收 bind: address already in use exit 1（无静默赢者之外的保证，07-02 OPS-01 设计答案）"
  status: failed
  reason: "自动化实证（b1b5.sh）：存活实例同路径第二实例未收 EADDRINUSE——unlink 存活 socket 后 listen 成功（静默赢者；证据 b.log 首行 listening on unix://...）。存活实例被孤儿化（进程在跑但 socket 路径已被夺走）"
  severity: major
  test: 3
  root_cause: "cmd/wesh/main.go:1023 listenSocket 在 Lstat 类型闸后无条件 os.Remove(path)——07-review CR-01 收窄只拒绝非 socket 文件，存活 socket 与残留 socket 在文件类型上不可区分，同被删除；main_test.go 仅覆盖残留场景（TestListenSocket/stale），无存活实例竞争用例"
  artifacts:
    - path: "cmd/wesh/main.go"
      issue: "listenSocket 无活性探测，无条件 os.Remove 存活 socket"
  missing:
    - "Remove 前活性探测（net.Dial unix：ECONNREFUSED=残留→清理；连通=存活→返回 EADDRINUSE 错误 exit 1）；竞争用例单测（存活实例 + 第二实例 → exit 1）"
  debug_session: ""

- gap_id: G-07-8
  truth: "xdg-open/open 返回非零（桌面异常）时仅 stderr 警告、服务不阻断（07-05 D-27 字面）"
  status: failed
  reason: "自动化实证（b6.sh）：stub xdg-open 返回非零时服务正常启动且 GET / 200（不阻断 ✓），但 stderr 无警告行。openBrowser 为 fire-and-forget .Start()（cmd/wesh/main.go:1244），启动器非零退出不可观测——与 07-RESEARCH Pattern 8 配方逐字一致，plan D-27 字面『返回非零只警告』与配方/实现有差（文档-实现对齐问题）"
  severity: minor
  test: 8
  root_cause: "exec.Command(...).Start() 不 Wait()，子进程退出码不可见；警告仅覆盖『启动器无法启动』（binary 缺失/exec 错误），不覆盖『启动器运行但非零退出』"
  artifacts:
    - path: "cmd/wesh/main.go"
      issue: "openBrowser fire-and-forget，非零退出静默"
  missing:
    - "二选一：goroutine Wait() 后非零退出补 stderr 警告行；或对齐 D-27/探针文档表述为『启动器不可启动时警告，运行期非零退出静默不阻断』"
  debug_session: ""
