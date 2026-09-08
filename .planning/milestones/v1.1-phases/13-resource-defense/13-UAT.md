---
status: complete
phase: 13-resource-defense
source: [13-01-SUMMARY.md, 13-02-SUMMARY.md, 13-03-SUMMARY.md, 13-04-SUMMARY.md, 13-05-SUMMARY.md, 13-06-SUMMARY.md, 13-07-SUMMARY.md, 13-08-SUMMARY.md]
started: 2026-09-06T02:21:44Z
updated: 2026-09-06T02:33:00Z
---

## Current Test

[testing complete]

## Tests

### 1. 冷启动冒烟（per-client 零配置）
expected: fresh 构建二进制后以 per-client 模式零配置启动（不传 --stop-timeout 等任何参数）：服务端正常监听，stderr 无 warn 无错误；curl /healthz 返回 200 且四字段键集 {status:"ok", clients, max_clients, session_active:true}；SIGTERM 干净退出无残留进程。
result: pass
note: 2026-09-06 自动化执行（/tmp/wesh-uat/smoke13.sh，fresh 构建 0.589s）：零配置启动正常监听；healthz {"status":"ok","clients":0,"max_clients":32,"session_active":true} 四字段键集正确；stderr 零 warn 零 error；SIGTERM 干净退出（无会话形态退出码 0——last-reaped 缺省 0 规则）；pgrep 零残留。

### 2. stop-timeout 默认 5s KILL 兜底（D-01）
expected: per-client 零配置（默认 5s 生效）attach 一个 HUP 免疫子进程（trap '' HUP）后断开：约 5s 后子进程被 SIGKILL 收割（pgrep/kill -0 检查 ESRCH），不会无限存活。
result: pass
note: phase13.mjs S2a/S2b 实测（本轮 29/29）：零配置 + trap '' HUP 免疫——~2s 时点存活（HUP 免疫 + KILL 未到期）+ 12s 护栏内 ESRCH + 断开至收割 ≈5.0s（标称 5s 零覆写实证）。

### 3. 显式 --stop-timeout=0 尊重 + 启动 warn（D-02）
expected: per-client 显式传 --stop-timeout=0 启动：启动 stderr 出现泄漏风险 warn（同时含 --stop-timeout=0 与 --session-mode=per-client 字样）；attach HUP 免疫子进程后断开，子进程不被 KILL（显式 0 被尊重，泄漏存活是用户显式选择）。
result: pass
note: phase13.mjs S2c 实测：显式 --stop-timeout=0 → stderr 泄漏 warn 行在场 + 6s 窗后免疫进程仍存活（无 KILL 兜底——显式位尊重的判别面；场景收口 kill -9 清理 + S2d pgrep 零残留）。

### 4. churn 节流拒绝（双令牌桶）
expected: per-client 1s 内快速连续 attach 超过 4 次（per-IP burst 4）：第 5 次起收到 Error 帧 "server is at capacity" 逐字文案 + close code 1011（非 1006，不触发前端重连放大）；stderr 出现 spawn_throttled 事件；等待令牌补给后新连接恢复放行。
result: pass
note: phase13.mjs S1a-S1f 实测：14 次高频 attach 成功 4（burst 放行）/拒绝 10 全部 close==1011；首拒绝端恰一 Error{server_error, "server is at capacity" 逐字}；三方精确相等 stderr 事件==10 == /metrics throttled_total==10 == 拒绝数；异 XFF 6/6 独立桶不挤占 + 同值 XFF 超被节流且事件 remote==XFF（换键直接证据）。

### 5. --once 退出码 255
expected: --once 模式 attach sh 后客户端断开：服务端进程退出且退出码为 255（客户端先断 -1 的进程级映射）；无残留子进程。
result: pass
note: phase13.mjs S3a 实测：--once 唯一客户端断开 → 服务端进程退出 code==255 逐值。

### 6. --exit-when-empty 宽限取消与重武装
expected: --exit-when-empty=2s 模式：attach 后断开 → 2s 宽限内重连 attach → 跨越原到期点（>3.5s）服务端仍存活且 echo 探针正常（宽限取消生效）→ 再次断开 → 2s 新宽限到期后服务端退出码 255（重武装不退化）。
result: pass
note: phase13.mjs S3c/S3d/S3e 实测：宽限到期无人归 → 255；宽限内重连 attach 成功 + echo 探针回读 + 跨原到期点 3.2s 存活窗（stale 计时器形态必翻车）；再断开后新宽限到期退出 255（重武装不退化）。

### 7. SIGTERM 优雅关停（N 进程组）
expected: 双客户端各自 attach 独立 sh 后向服务端发 SIGTERM：双端各收 close 1001 + reason 含 server_shutting_down；两个子进程组均在护栏时间内被收割（ESRCH）；服务端进程退出（255）无泄漏。
result: pass
note: phase13.mjs S4a-S4e 实测：双客户端双独立 sh（pid 不等）→ SIGTERM → 双端各收 1001 + reason 含 server_shutting_down → 服务端退出 255 + 双 pgid 各 5s 护栏内 ESRCH + stderr session_end==2 全 signal==SIGHUP（审计零丢失）。

### 8. /metrics 21 series 双模式
expected: per-client attach 后 curl /metrics：四新计数器 series（wesh_pty_spawn_total/spawn_failures/kills/spawn_throttled_total）在场且 spawn_total≥1；wesh_session_active 为活跃会话计数（attach 2 时==2，断开一端收敛 1）；HELP 文案 "Number of active per-client PTY sessions."；样本行零 label 花括号。shared 模式对照：四 series 恒 0 但保留在场，session_active 为 0/1 探活语义 + 探活 HELP 文案。
result: pass
note: phase13.mjs S5a-S5e 实测：per-client 四计数器 series 全在 + spawn_total==1 + session_active==1 + HELP 会话计数文案 + 全部样本行零 label 花括号（build_info version 豁免）；shared 对照四 series 恒 0 保留不摘 + session_active==1 探活 + HELP 探活文案逐字。gauge 计数语义（attach 2 断一收敛 1）由 Go 级 TestMetricsPerClient 承载（-race 全绿）。

### 9. /healthz session_active + session 审计事件串联
expected: per-client /healthz 的 session_active 恒为 true（会话服务可用语义，编排探活只看 200/503）；attach → 使用 → 退出全程 stderr 审计：session_start 事件（pid + client_id）与 session_end 事件（exit_code + duration_seconds + client_id）同 client_id 串联闭合单个会话全生命周期。
result: pass
note: 2026-09-06 自动化执行（/tmp/wesh-uat/audit13.mjs，--once + exit 42 子先死形态）：healthz attach 前 session_active==true 四字段；服务端退出码 42（last-reaped-code 透传）；session_start 恰一（pid>0 + client_id）+ session_end 恰一（exit_code==42 + duration_seconds>0 + 无 signal 键）+ 两事件同 client_id==1（全生命周期串联）。

### 10. WESH_REMOTE_USER 注入链
expected: per-client + --auth-header X-Remote-User（trust 开启）携头 "alice" attach：Web shell 内 echo/printenv 可见 WESH_REMOTE_USER=alice（注入链全通）；携控制字符头值（NEL）被剥离后注入；shared 模式同头对照：env 中无 WESH_REMOTE_USER 键（D-15 收窄零漂移）。
result: pass
note: phase13.mjs S6a/S6b/S6c 实测：携头 attach → printenv 回读 WESH_REMOTE_USER=alice（注入链全通）；NEL 线形头值（U+0085）→ 回读剥离产物 carl 且控制字符零出现（sanitize 先于注入）；shared 对照同头 → env 无该键 + echo 标记收口到达（会话照常）。

### 11. 用户文档核对（README/CONFIGURATION）
expected: README.md 与 docs/CONFIGURATION.md：per-client stop-timeout 默认 5s 双默认值语义已记载；无「per-client 行为装配中，当前版本与 shared 等价」失实残留；README 含「保活先杀时序」段、CONFIGURATION 含「ping-interval 与断开时序」小节（1006 先杀语义）。
result: pass
note: 2026-09-06 grep 核对：README:96 双默认值语义（shared 0 / per-client 默认 5s / 显式 0 尊重+warn）；CONFIGURATION:77 键表 + :160 默认值表双行；失实残留（"当前版本与 shared 等价"/"行为装配中"）零命中；README:98 保活先杀时序段 + CONFIGURATION:170-178 「ping-interval 与断开时序」小节（1006 先杀五要点 + 测试注记）全部在场。

## Summary

total: 11
passed: 11
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
