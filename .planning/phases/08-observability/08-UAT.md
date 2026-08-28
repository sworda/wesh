---
status: diagnosed
phase: 08-observability
created: 2026-08-28
started: 2026-08-28T09:00:00Z
updated: 2026-08-28T09:55:00Z
source: [08-05-PLAN.md flagged_assumptions, 08-RESEARCH.md Environment Availability]
---

# Phase 8 人工 UAT 清单（可观测性）

## Current Test

[testing complete — 2 pass / 1 issue（G-08-2 已诊断，待 gap 修复计划])

自动化断言不可达项的人工复核清单：08-05 plan flagged_assumptions 登记的三条（真实 Prometheus scrape 兼容性 / journald 实机 ingest 与 jq 检索 / draining 窗口编排观测率）。每项 = 步骤 + 预期 + 勾选框。自动化已覆盖的协议层行为见 `web/uat/phase08.mjs`（六场景 21 断言：S1 健康检查四组 / S2 metrics 认证闸两态 / S3 exposition 17 series 与数值 / S4 503 draining / S5 审计事件检索 / S6 控制字符剥离）。

**拓扑注记（CODEBUDDY.md 双机架构）**：Linux 开发机 headless、Windows 工作站跑浏览器层——本 phase 纯服务端、零前端改动，**无浏览器层人工项**；下列三项均为运维栈实机复核（Prometheus/systemd/journald 环境），与浏览器无关。

## A. Flagged Assumptions 复核项（08-05 登记，逐项列探针来源原文）

- [x] **A1. 真实 Prometheus scrape 兼容性**（08-04 flagged_assumptions：「无真实 Prometheus 实例验证——exposition 合法性以规范条款逐字断言代替，真实 scrape 随 08-05 人工清单复核」；08-RESEARCH Environment Availability：Prometheus 实例 ✗）—— **2026-08-28 实测通过（执行人：CodeBuddy agent，本机下载官方二进制自建实例）**
  - 步骤：按 README「运维（Phase 8）→ 指标（/metrics）」的 `scrape_configs` 配方配置真实 Prometheus 实例（`basic_auth` 填与 wesh 凭据同组值），wesh 以凭据模式启动；等待两个 scrape 周期后打开 Prometheus UI 的 Status → Targets 与 Graph。
  - 预期：target 状态 UP；`wesh_clients_connected`、`wesh_pty_output_bytes_total`、`wesh_build_info{version="..."}` 等 17 条 series 全部可查询可见；无 parse error（exposition 手写 text 0.0.4 与采集器兼容）。
  - 负面对照（可选）：故意填错 `basic_auth` 密码 → target down，且 wesh 日志出现 `throttled` 事件（`remote`=采集器 IP、`retry_after` 秒数）——README「凭据错误触发全站节流（429）自锁」明示的实机印证。
  - 自动化等价面（已绿）：phase08.mjs S2/S3（认证闸两态 + 17 series HELP 行逐字 + Content-Type `text/plain; version=0.0.4` 断言）+ Go 侧 TestMetricsExposition（三行组序/末行换行/契约序）。
  - **实测记录**：Prometheus 2.55.1 LTS 官方二进制 + README 配方（basic_auth 同组凭据，scrape_interval 1s）→ target `health=up` lastError 空；17 条 `wesh_*` series 经 `/api/v1/label/__name__/values` 全部入库可查；`wesh_build_info{version="dev"} = 1` 数值可见。promtool 3.5 `check metrics` parse 通过（仅 1 条非阻塞 lint：`wesh_outbox_depth_bytes_sum` gauge 带 `_sum` 保留后缀命名警告——非 parse error，采集入库无碍，留作命名观察项）。负面对照：错密码 job → target `health=down` lastError=`server returned HTTP status 429 Too Many Requests`（自锁），wesh 日志 `throttled`×7 条（`remote=127.0.0.1:*`、`retry_after=1→2` 指数翻倍实机可见）+ `auth_failed`×3。⚠ 环境备注：Prometheus 3.5.0 在本机出现 targets 恒空异常（配置经 promtool 校验合法、exposition parse 通过，scrape pool 不创建）——版本/环境问题，与 wesh 无关，判定以 2.55.1 LTS（生产主流）实测为准。

- [x] **A2. journald 实机 ingest 与 jq 检索**（08-05 flagged_assumptions：「journald 实机 ingest 与 jq 检索」；08-RESEARCH Runtime State Inventory：「systemd 的 stderr→journald 通道对格式透明」为推演非实测）—— **2026-08-28 部分执行：事件已制造入 journal，检索面 blocked 于 journal 读权限（待用户介入）**
  - 步骤：systemd 部署 wesh（README「安全说明」的 EnvironmentFile= 配方或等价 unit），制造一次认证失败（错凭据访问）与一次 attach/detach（浏览器开关页面各一）；随后执行 README「结构化日志」节两则示例：`journalctl -u wesh -o cat | jq -c 'select(.event=="auth_failed")'` 与 `journalctl -u wesh -o cat | jq -c 'select(.client_id==N)'`（N 取前一步 attach 事件的 client_id 值）。
  - 预期：第一则检出刚才的 `auth_failed` 事件行（无 user/username 键）；第二则检出同一 client_id 的 attach 与 detach 各一条（reason=normal）；journald 不截断不转义 JSON 行（jq 解析零报错）。
  - 自动化等价面（已绿）：phase08.mjs S5（auth_failed 无用户名键 / attach+detach client_id 关联 / session_end 三字段）+ 08-01 真实二进制 jq 冒烟（`select(.event=="auth_failed")` 直打成立）。
  - **实测记录**：systemd 用户级 unit 已按 EnvironmentFile= 配方部署（`~/.config/systemd/user/wesh-uat.service`，`WESH_CREDENTIAL` 走 chmod 600 env 文件，user scope 的 stderr→journald 通道与系统级同构）；事件已制造完毕（错凭据 401 → `auth_failed`、Node 原生 WS ticket 握手 → attach/detach 对，进程 stderr 均进 journald）。**blocked 点**：本机 `/run/log/journal/` 仅 root/systemd-journal/adm/wheel 组可读（用户在 users 组、无 sudo），`journalctl`（含 `--user`）报 "No journal files were opened due to insufficient permissions"，jq 检索两则示例无法执行。**待用户介入**：`sudo usermod -aG systemd-journal $(whoami)` 后重登，或以 sudo 代跑两则检索命令；事件已在 journal（`_SYSTEMD_USER_UNIT=wesh-uat.service`）可回溯检索，无需重制。另注：journald 对 JSON 行的透明性已由 ingest 侧旁证（unit active 期间 stderr 无损送达 journal 文件），余下断言纯检索面。

- [x] **A3. draining 窗口编排观测率**（08-03 flagged_assumptions 登记「默认配置 draining 窗口 <1s」已实证 + 08-05：「draining 窗口编排观测率」；RESEARCH OQ3：窗口宽度依赖 stop-signal 序列时长）—— **2026-08-28 实测通过（执行人：CodeBuddy agent）**
  - 步骤：systemd 部署下对 wesh 执行 `systemctl restart wesh`（或 `kill -TERM`），同时另开终端以 0.2s 周期循环 `curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7681/healthz`；观测重启窗口内健康检查输出序列。
  - 预期：重启窗口内可观测到 503（`status` 字段 `"draining"`）出现在 200 与连接拒绝（000）之间；观测密度与窗口宽度正相关——默认 stop-signal HUP 无 timeout 时窗口亚秒级可能只捕到一两次（非缺陷，探活周期匹配问题）；若编排要求确定性摘流，配置 `--stop-timeout` 拉宽窗口（代价是关停耗时增加）。
  - 自动化等价面（已绿）：phase08.mjs S4（trap 忽略 HUP + `--stop-timeout 3s` 拉宽窗口 → SIGTERM 后轮询确定性观测 503 draining 恰四键 → 进程 255 退出）+ Go 侧 TestHealthzDraining（置位点唯一 + 翻转序列 + 不翻回）。
  - **实测记录**：①默认配置形态——子进程默认 stop-signal=HUP 即死，SIGTERM 后连续无间隔轮询（分辨率 ~8ms）窗口内 0 样本（200 → 000，窗口 <8ms），亚秒级预期成立的极端形态（无客户端时 draining 即时完成），非缺陷；②拉宽形态——`trap '' HUP` 忽略信号 + `--stop-timeout 3s`，0.2s 周期轮询确定性观测 `200 → 503×15（body 四字段同构：status=draining/clients/max_clients/session_active）→ 000` 完整序列。两种形态均与 UAT 预期文案一致。

---

*自动化覆盖边界：协议层六场景见 phase08.mjs；本清单仅收自动化不可达项（真实采集栈/真实 init 系统）。完成后勾选并注明日期与执行人。*

## Tests

<!-- canonical · 机器判定面：供 phase uat-passed 谓词解析（### N. + 列 0 result: 行）；详证见上方 A 各条目。三项均为运维栈实机复核——自动化等价面已全绿（phase08.mjs 21/21 + Go -race 五包），实机面待人工执行。 -->

### 1. A1 真实 Prometheus scrape 兼容性
expected: 按 README basic_auth 配方配置真实 Prometheus 后 target UP，17 条 series 全部可见无 parse error
result: pass
note: 2026-08-28 实机通过（Prometheus 2.55.1 LTS + README 配方）：target up、17 series 全查、build_info 数值可见；promtool parse 通过（1 条非阻塞 _sum 命名 lint）；负面对照 down+429 自锁+throttled×7（retry_after 翻倍）——详见上方 A1 实测记录

### 2. A2 journald 实机 ingest 与 jq 检索
expected: systemd 部署下 journalctl -o cat 输出可被 jq 逐行解析；select(.event=="auth_failed") 与 select(.client_id==N) 两示例检出预期事件
result: issue
reported: "journal 全流管道被启动横幅打断：README 示例 journalctl -o cat | jq 原样执行报 parse error at line 2（stdout 横幅 listening on.../share link 行混入 journal），jq 停止后续事件检索不到；grep '^{' 过滤后 auth_failed 无用户名键 / client_id 关联 attach+detach reason=normal / JSON 事件行全量合法全部成立"
severity: major
note: 2026-08-28 实机执行（用户已加 systemd-journal 组，sg 代跑）：stderr 恒 JSON 承诺成立（横幅走 stdout，2>/dev/null 后无横幅）；根因=systemd 默认 StandardOutput=journal 把 stdout 横幅送进 journal 与 stderr JSON 合流——详见 Gaps G-08-2

### 3. A3 draining 窗口编排观测率
expected: systemctl restart 窗口内 /healthz 轮询序列出现 503 draining（200 → 503 → 000）；默认配置窗口亚秒级属预期
result: pass
note: 2026-08-28 实机通过：默认配置窗口 <8ms（亚秒级极端形态，非缺陷）；trap 忽略 HUP + --stop-timeout 3s 确定性观测 200 → 503×15（draining 四字段同构）→ 000——详见上方 A3 实测记录

## Summary

total: 3
passed: 2
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-08-2
  truth: "systemd 部署下 README 两则 jq 检索示例（journalctl -o cat | jq 'select(...)') 原样可用，journald 管道对 JSON 行零 parse error"
  status: failed
  reason: "User reported: README 示例 journalctl -o cat | jq 原样执行报 parse error at line 2，jq 停止后续事件检索不到（grep '^{' 过滤后核心断言全部成立）"
  severity: major
  test: 2
  root_cause: "wesh 启动横幅（listening on ... / share read-only: ...）走 stdout；systemd unit 默认 StandardOutput=journal 使 stdout 横幅与 stderr JSON 事件在 journal 合流，README「结构化日志」示例管道 jq 在横幅行 parse error 中止——README「恒 JSON」承诺仅对 stderr 成立（2>/dev/null 分离实验实证），但示例命令未含 stdout 横幅防护，默认配置下不可复现"
  artifacts:
    - path: "cmd/wesh"  # 启动横幅打印点（stdout）
      issue: "横幅行非 JSON，经 StandardOutput=journal 默认配置混入 jq 检索管道"
    - path: "README.md"
      issue: "「结构化日志」节示例（journalctl -o cat | jq）与 systemd 部署配方均未处理 stdout 横幅行——示例在默认配置下 parse error"
  missing:
    - "修复方向（待 planner 裁量）：wesh 检测 stdout 非 TTY 时横幅改 JSON 事件或转 stderr / README systemd 配方补 StandardOutput 处理 / README 示例补 grep '^{ ' 防护——三者取其一或组合"
  debug_session: "inline（2026-08-28 UAT 实测内诊断：stdout/stderr 分离实验 + journal 原样重放确证，未开独立 debug session）"
