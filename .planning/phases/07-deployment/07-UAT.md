---
status: testing
phase: 07-deployment
created: 2026-08-26
source: [07-VERIFICATION.md, 07-VALIDATION.md, 07-08-PLAN.md]
started: 2026-08-26T07:10:00Z
updated: 2026-08-26T07:10:00Z
---

# Phase 7 人工 UAT 清单（部署与配置）

自动化断言不可达项的人工复核清单：VALIDATION 登记的 manual-only 两项 + 各 plan flagged assumptions 复核项 + root 降权 nobody 可选场景。
每项 = 步骤 + 预期 + 勾选框。自动化已覆盖的协议层行为见 `web/uat/phase07.mjs`（八场景 33 断言）。

## A. Manual-Only 项（07-VALIDATION.md 登记）

- [ ] **A1. 浏览器自动打开真实效果（OPS-11）**
  - 步骤：在有 GUI 的环境（桌面 Linux / macOS / Windows 工作站）运行 `wesh --open --writable -- bash`。
  - 预期：启动后系统默认浏览器自动打开 rw 分享链接（含 token 免交互直接进入终端），无需手动复制 URL；终端可用。
  - 备注：macOS 真实弹窗为本机（Linux headless）未实测面——CI macOS 跑同款单测形态整体 Skip（07-05 flagged_assumptions）。

- [ ] **A2. 真实 nginx 反代挂载观感（OPS-02）**
  - 步骤：按 README「部署与配置 → 反代子路径」配方配置真实 nginx（`location = /wesh` 精确块 + `location /wesh/` 前缀块，后端 `wesh --base-path /wesh -- bash`）；浏览器访问 `http://<host>/wesh`（裸路径，无尾斜杠）。
  - 预期：308 重定向到 `/wesh/` 后页面正常加载、WS 升级成功、终端可用；idle >60s 连接不断（`proxy_read_timeout 3600s` > `--ping-interval 5s`）；裸路径不经精确块时 404（复核精确块必要性）。截图留档。

## B. Flagged Assumptions 复核项（各 plan 登记，逐项列探针问题原文）

- [ ] **B1. OPS-01 并发中断语义**（07-02 登记）
  - 探针原文：「If interrupted or run in parallel, what is guaranteed?」
  - 步骤：两个 wesh 实例同时以同一 `--socket` 路径启动；随后 kill 掉已 listen 的实例，再次启动。
  - 预期：后者 `bind: address already in use` exit 1（无静默赢者之外的保证）；kill 后再次启动时残留 socket 文件被自动清理（listen 前 Remove）、启动成功。

- [ ] **B2. SEC-07 多值头 / 空值头**（07-03 登记）
  - 探针原文：「多值头（重复 X-Remote-User 头行）取 Header.Get 首值；空串头值 → sanitize 后空 → 不出键（与缺席同态）」
  - 步骤：`--auth-header X-Remote-User` 实例下，curl 携两个不同值的 `X-Remote-User` 头行请求；再携空串头值请求。
  - 预期：事件行 `remote_user` 取首值；空串头值时事件行不出现 `remote_user` 键（与缺席同态）。（proxy_test.go 表驱动已覆盖——人工复核真实反代行为一致性。）

- [ ] **B3. OPS-04 symlink / TERM 任意值 / stop-timeout 极大值**（07-04 登记）
  - 探针原文：「--cwd 为符号链接时按内核语义解析（不额外规范化）；--term 任意字符串不校验（TERM 值合法性由终端数据库承担，wesh 不立场）；--stop-timeout 极大值只推迟 KILL 不阻塞 exitf（AfterFunc 异步）」
  - 步骤：① `--cwd` 给符号链接路径启动；② `--term` 给非标准字符串（如 `--term foobar`）启动后终端内 `echo $TERM`；③ `--stop-timeout 1h` 触发关停后中途手动 kill 子进程。
  - 预期：① 正常启动，子进程 cwd 按内核语义解析；② 启动不校验，`$TERM` 原样为 `foobar`；③ 子进程死亡后 wesh 即时退出（不等满 1h——补 KILL 是异步兜底，不阻塞收口）。

- [ ] **B4. OPS-05 降权 nobody 无 shell 与附加组清空（root 可选场景）**（07-04 登记 + 07-04 SUMMARY 复核联动）
  - 探针原文：「降权到存在但无登录 shell 的 uid（如 nobody）时 shell 自默认行为由子进程命令承担；supplementary groups 不设置（清空附加组）——与『最小权限』一致，有意为之」
  - 步骤（需 root）：`sudo ./wesh --bind 127.0.0.1 --uid 65534 --gid 65534 -- bash`；浏览器 attach 后依次执行 `id`、`echo $HOME $USER $LOGNAME`；Ctrl 台 SIGTERM 关停。
  - 预期：`id` 输出 `uid=65534(nobody) gid=65534(nogroup)` 且**附加组清空**（groups 仅 nogroup——root 启动清空附加组是最小权限既定语义；非 root 降回自身则保留自身附加组，07-04 环境感知策略复核联动）；`HOME`/`USER`/`LOGNAME` 按 nobody passwd 条目改写（无条目则剔除三键、shell 自默认）；SIGTERM 后 1001 优雅下线序列、退出码 255。

- [ ] **B5. OPS-09 TOML 语法变体与空 command**（07-06 登记）
  - 探针原文：「TOML 多行数组/内联表等合法 TOML 语法按 go-toml 语义接受（平铺形状是约定不是语法强制）；配置重复键由 TOML 解析器拒绝（规范行为）；command 空数组 `command = []` 等价缺席」
  - 步骤：① 配置文件用多行数组/内联表写法；② 配置文件写重复键；③ 配置文件写 `command = []`。
  - 预期：① 正常加载（go-toml 语义接受）；② exit 2（TOML 解析器规范拒绝）；③ 等价缺席 → `missing command` exit 2（与 CLI `--` 空 argv 同档）。

- [ ] **B6. OPS-11 macOS open 与 TLS 组合**（07-05 登记）
  - 探针原文：「macOS open 行为未在本机实测；xdg-open 存在但返回非零（桌面异常）只警告不阻断（D-27）；--open 与 TLS 组合打开 https:// 链接（自签证书浏览器警告属用户预期面）」
  - 步骤：① macOS 上 `wesh --open -- bash`；② `wesh --open --tls-cert <cert> --tls-key <key> -- bash`。
  - 预期：① `open` 命令拉起默认浏览器打开 ro 分享链接；② 打开 `https://` 链接，自签证书浏览器警告属用户预期面；xdg-open/open 返回非零时仅 stderr 警告、服务不阻断。

## C. RESEARCH A1 复核（已闭合——留档）

- [x] **C1. nginx `location /wesh/` 不匹配裸 `/wesh`**（07-RESEARCH A1，README 精确块必要性依据）
  - 复核结论：本机 nginx 1.14.1 实测（2026-08-26，07-08 执行期）——`location /wesh/ { return 200; }` 配置下 `GET /wesh` → **404**、`GET /wesh/` → **200**。A1 成立：`location = /wesh { return 308 /wesh/; }` 精确重定向块**必需**，README 配方据此确凿落文（非防御性建议）。

---

*自动化覆盖边界：协议层全场景（配置合并/unix socket/base-path/auth-header/XFF/stop-signal/降权 self/1001/--open）见 phase07.mjs；本清单仅收自动化不可达项。完成后勾选并注明日期与执行人。*
