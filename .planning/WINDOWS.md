---
schema_version: 1
open_count: 21
waived_count: 0
fixed_count: 3
total_count: 24
last_updated: 2026-08-30T03:53:54.769Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 01 | deviation | internal/server/server.go |  | Rule 1 修复：D-10 退出码被 D-11 竞态覆盖，childExited 标志收口（d5f67ab） | open |  | 2026-08-14T01:56:56.364Z |  |
| 2 | 01 | todo | internal/pty/reap_darwin.go | 76 | loop() 错误路径 TODO(Phase 8) 接 slog——计划内延期（RESEARCH 骨架原文，Phase 1 进程级致命即可） | open |  | 2026-08-14T02:10:23.476Z |  |
| 3 | 01 | unrun-verify | internal/pty/reap_darwin_test.go |  | TestKqueue* 运行时裁决待 CI macos-latest leg 首推运行（计划内 CI-only，本机无 macOS） | open |  | 2026-08-14T02:10:23.638Z |  |
| 4 | 02 | deviation | internal/server/limits_test.go |  | TestReadLimitBoundary 载荷 zeros→'A'（PTY ECHOCTL 实测 2× 回显失真） | open |  | 2026-08-15T09:18:23.988Z |  |
| 5 | 03 | deviation | internal/server/server.go |  | 03-03: ServeMux 405 被 / 子树吞掉，补显式同文 fallback（已修复并验证） | open |  | 2026-08-17T08:48:16.807Z |  |
| 6 | 04 | deviation | web/pnpm-workspace.yaml |  | pnpm.overrides 机制迁移：package.json pnpm 字段 → pnpm-workspace.yaml（pnpm 11 不读前者） | fixed |  | 2026-08-18T16:18:48.026Z | 2026-08-18T16:20:13.593Z |
| 7 | 05 | stub | internal/server/clients.go |  | registry.gateTransitions 门开闭周期计数器（观测性 stub，review #10 授权；Phase 8 OPS-07 进 metrics 时消费） | open |  | 2026-08-20T09:37:49.973Z |  |
| 8 | 05 | stub | internal/server/server.go |  | inputDrops 限速丢弃计数器（观测性 stub，review #10 授权；Phase 8 OPS-07 进 metrics 时消费） | open |  | 2026-08-20T14:24:41.015Z |  |
| 9 | 05 | stub | internal/server/clients.go |  | inputQ.droppedInputs 队列满丢弃计数器（观测性 stub，review #10 授权；Phase 8 OPS-07 进 metrics 时消费） | open |  | 2026-08-20T14:24:41.172Z |  |
| 10 | 05 | deviation | internal/server/multi_test.go |  | TestAllPolicy 适配 G-05-1 运行期推送（2→1 last-wins 推送帧显式消费断言，planner 回归自检遗漏面） | open |  | 2026-08-22T03:42:50.177Z |  |
| 11 | 05 | deviation | internal/server/multi_test.go |  | plan 字面 all 子测 B(60,20)->60/24 算术矛盾，按意图修正 B(60,50)->60/43 rows 维区分度 | open |  | 2026-08-22T03:42:50.331Z |  |
| 12 | 05 | deviation | web/src/main.ts |  | 05-11: roNotified Task 1 export 防 noUnusedLocals 接线前误报（queryKeys 04-05 先例第二次沿用），Task 2 接线后去 export（ced81ed/31d8a68） | open |  | 2026-08-22T04:15:52.151Z |  |
| 13 | 05 | deviation | web/uat/phase05-dims.mjs |  | 05-12: probe10.mjs 探针从未入库（G-05-1 诊断期一次性用具），按 plan Task 2 机制描述重建转正为门禁断言，文件头注释登记血缘 | open |  | 2026-08-22T04:55:53.043Z |  |
| 14 | 05 | deviation | web/uat/phase05-dom.mjs |  | 05-12: phase05-dom.mjs/phase05-flood-driver.mjs 为 05-09 产物但从未入库（git 历史零记录），随 Task 2 补录使 D6 修改可提交（ce91dc5） | open |  | 2026-08-22T04:55:53.195Z |  |
| 15 | 05 | deviation | .planning/phases/05-multi-client/05-VALIDATION.md |  | 05-12: plan 字面 05-10-01 行 go test -run 选择器 '\\\|' 在 RE2 下为字面管道符零匹配假绿，改裸 '\|' 实测 green 回填（fd26ebe） | open |  | 2026-08-22T04:55:53.347Z |  |
| 16 | 06 | deviation | cmd/wesh/main.go |  | 06-04: IsBoolFlag 逐字引文改作 func 行尾注释（plan『注释逐字引 GOROOT』与验收 grep IsBoolFlag==1 两约束机械调和，语义零损失） | open |  | 2026-08-23T07:07:13.884Z |  |
| 17 | 06 | unrun-verify | web/uat/phase06-dom.mjs |  | D9 真实 OS 断网栈/浏览器原生 online/offline 事件时序按 headless 硬约束豁免（skipped+reason，指向 06-UAT.md 人工清单） | open |  | 2026-08-23T07:42:10.089Z |  |
| 18 | 06 | unrun-verify | web/uat/phase06.mjs |  | S7 真实断网栈/浏览器原生事件序列 skipped（headless 硬约束豁免）——人工清单见 .planning/phases/06-session-lifecycle/06-UAT.md（06-07 产出）；协议层等价物 S6 已覆盖 | open |  | 2026-08-23T08:24:59.916Z |  |
| 19 | 07 | deviation | internal/server/server.go |  | 07-01 Rule 1 调和：无认证分支 bp 形态补注册 attach path-only 405 fallback（bp=="" 根挂载保持 embed 404 零漂移，差异注释锚定，23d72c8） | open |  | 2026-08-26T00:47:53.956Z |  |
| 20 | 07 | deviation | web/dist/index.html |  | 07-01 Rule 3 验收闸适配：dist 升级前缀 grep 改引号无关心形态（esbuild 反引号模板字面量发射，字面量未重命名，断言面守恒，4f1fc8e） | open |  | 2026-08-26T00:47:54.121Z |  |
| 21 | 07 | deviation | web/uat/phase07.mjs |  | 07-07: S4c NEL 探针改线形等价物构造（undici latin1 编码实证——朴素 'ali\\u0085ce' 上线单字节 0x85 复现不了剥离路径，'ali\\u00C2\\u0085ce' 双码点 = UTF-8 NEL 等价线形） | open |  | 2026-08-26T06:30:22.013Z |  |
| 22 | 07 | unrun-verify | web/uat/phase07.mjs |  | S8c 真实弹浏览器拉起与标签页观感 skipped（headless 硬约束豁免——真实 GUI 属 Windows 工作站人工层，CODEBUDDY.md 平台原生行为豁免条款）；协议层等价物 S8a（headless 跳过）+ S8b（fake xdg-open argv 全等）已覆盖，人工复核项见 07-08 plan | open |  | 2026-08-26T06:30:42.875Z |  |
| 23 | 09 | deviation | .planning/phases/09-release-polish/09-01-PLAN.md | 118 | 09-01 plan verify 块 goreleaser 产物目录路径（dist/<os>_<arch>/）与 v2.18.0 实证布局（dist/wesh_<os>_<arch>_<variant>/）不符——执行已按实证路径，断言全绿；PLAN 文档字面未改（历史产物） | fixed |  | 2026-08-29T13:45:46.256Z | 2026-08-29T13:46:51.606Z |
| 24 | 09 | deviation | web/uat/pw/phase09-caddy-ctl.sh | 26 | 09-08: Caddyfile 站点地址 http://0.0.0.0:10014 系字面 Host 匹配（仅 Host: 0.0.0.0 命中，真实主机名落空走兜底空 200）——Task 1『外部 Host 照常被服务』结论系 proto-verify 探针 curl 0.0.0.0 自身的假绿，Windows 首跑 0/2 捕获；改裸 :10014（绑定全网卡+匹配任意 Host）三层复验全绿：LAN IP 矩阵 + proto 7/7 + Windows 4/4（28ae2f2） | fixed |  | 2026-08-30T03:53:43.425Z | 2026-08-30T03:53:54.769Z |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "01",
    "file": "internal/server/server.go",
    "line": null,
    "description": "Rule 1 修复：D-10 退出码被 D-11 竞态覆盖，childExited 标志收口（d5f67ab）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-14T01:56:56.364Z",
    "resolved_at": null
  },
  {
    "id": 2,
    "kind": "todo",
    "phase": "01",
    "file": "internal/pty/reap_darwin.go",
    "line": 76,
    "description": "loop() 错误路径 TODO(Phase 8) 接 slog——计划内延期（RESEARCH 骨架原文，Phase 1 进程级致命即可）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-14T02:10:23.476Z",
    "resolved_at": null
  },
  {
    "id": 3,
    "kind": "unrun-verify",
    "phase": "01",
    "file": "internal/pty/reap_darwin_test.go",
    "line": null,
    "description": "TestKqueue* 运行时裁决待 CI macos-latest leg 首推运行（计划内 CI-only，本机无 macOS）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-14T02:10:23.638Z",
    "resolved_at": null
  },
  {
    "id": 4,
    "kind": "deviation",
    "phase": "02",
    "file": "internal/server/limits_test.go",
    "line": null,
    "description": "TestReadLimitBoundary 载荷 zeros→'A'（PTY ECHOCTL 实测 2× 回显失真）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-15T09:18:23.988Z",
    "resolved_at": null
  },
  {
    "id": 5,
    "kind": "deviation",
    "phase": "03",
    "file": "internal/server/server.go",
    "line": null,
    "description": "03-03: ServeMux 405 被 / 子树吞掉，补显式同文 fallback（已修复并验证）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-17T08:48:16.807Z",
    "resolved_at": null
  },
  {
    "id": 6,
    "kind": "deviation",
    "phase": "04",
    "file": "web/pnpm-workspace.yaml",
    "line": null,
    "description": "pnpm.overrides 机制迁移：package.json pnpm 字段 → pnpm-workspace.yaml（pnpm 11 不读前者）",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-18T16:18:48.026Z",
    "resolved_at": "2026-08-18T16:20:13.593Z"
  },
  {
    "id": 7,
    "kind": "stub",
    "phase": "05",
    "file": "internal/server/clients.go",
    "line": null,
    "description": "registry.gateTransitions 门开闭周期计数器（观测性 stub，review #10 授权；Phase 8 OPS-07 进 metrics 时消费）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-20T09:37:49.973Z",
    "resolved_at": null
  },
  {
    "id": 8,
    "kind": "stub",
    "phase": "05",
    "file": "internal/server/server.go",
    "line": null,
    "description": "inputDrops 限速丢弃计数器（观测性 stub，review #10 授权；Phase 8 OPS-07 进 metrics 时消费）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-20T14:24:41.015Z",
    "resolved_at": null
  },
  {
    "id": 9,
    "kind": "stub",
    "phase": "05",
    "file": "internal/server/clients.go",
    "line": null,
    "description": "inputQ.droppedInputs 队列满丢弃计数器（观测性 stub，review #10 授权；Phase 8 OPS-07 进 metrics 时消费）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-20T14:24:41.172Z",
    "resolved_at": null
  },
  {
    "id": 10,
    "kind": "deviation",
    "phase": "05",
    "file": "internal/server/multi_test.go",
    "line": null,
    "description": "TestAllPolicy 适配 G-05-1 运行期推送（2→1 last-wins 推送帧显式消费断言，planner 回归自检遗漏面）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-22T03:42:50.177Z",
    "resolved_at": null
  },
  {
    "id": 11,
    "kind": "deviation",
    "phase": "05",
    "file": "internal/server/multi_test.go",
    "line": null,
    "description": "plan 字面 all 子测 B(60,20)->60/24 算术矛盾，按意图修正 B(60,50)->60/43 rows 维区分度",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-22T03:42:50.331Z",
    "resolved_at": null
  },
  {
    "id": 12,
    "kind": "deviation",
    "phase": "05",
    "file": "web/src/main.ts",
    "line": null,
    "description": "05-11: roNotified Task 1 export 防 noUnusedLocals 接线前误报（queryKeys 04-05 先例第二次沿用），Task 2 接线后去 export（ced81ed/31d8a68）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-22T04:15:52.151Z",
    "resolved_at": null
  },
  {
    "id": 13,
    "kind": "deviation",
    "phase": "05",
    "file": "web/uat/phase05-dims.mjs",
    "line": null,
    "description": "05-12: probe10.mjs 探针从未入库（G-05-1 诊断期一次性用具），按 plan Task 2 机制描述重建转正为门禁断言，文件头注释登记血缘",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-22T04:55:53.043Z",
    "resolved_at": null
  },
  {
    "id": 14,
    "kind": "deviation",
    "phase": "05",
    "file": "web/uat/phase05-dom.mjs",
    "line": null,
    "description": "05-12: phase05-dom.mjs/phase05-flood-driver.mjs 为 05-09 产物但从未入库（git 历史零记录），随 Task 2 补录使 D6 修改可提交（ce91dc5）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-22T04:55:53.195Z",
    "resolved_at": null
  },
  {
    "id": 15,
    "kind": "deviation",
    "phase": "05",
    "file": ".planning/phases/05-multi-client/05-VALIDATION.md",
    "line": null,
    "description": "05-12: plan 字面 05-10-01 行 go test -run 选择器 '\\|' 在 RE2 下为字面管道符零匹配假绿，改裸 '|' 实测 green 回填（fd26ebe）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-22T04:55:53.347Z",
    "resolved_at": null
  },
  {
    "id": 16,
    "kind": "deviation",
    "phase": "06",
    "file": "cmd/wesh/main.go",
    "line": null,
    "description": "06-04: IsBoolFlag 逐字引文改作 func 行尾注释（plan『注释逐字引 GOROOT』与验收 grep IsBoolFlag==1 两约束机械调和，语义零损失）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-23T07:07:13.884Z",
    "resolved_at": null
  },
  {
    "id": 17,
    "kind": "unrun-verify",
    "phase": "06",
    "file": "web/uat/phase06-dom.mjs",
    "line": null,
    "description": "D9 真实 OS 断网栈/浏览器原生 online/offline 事件时序按 headless 硬约束豁免（skipped+reason，指向 06-UAT.md 人工清单）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-23T07:42:10.089Z",
    "resolved_at": null
  },
  {
    "id": 18,
    "kind": "unrun-verify",
    "phase": "06",
    "file": "web/uat/phase06.mjs",
    "line": null,
    "description": "S7 真实断网栈/浏览器原生事件序列 skipped（headless 硬约束豁免）——人工清单见 .planning/phases/06-session-lifecycle/06-UAT.md（06-07 产出）；协议层等价物 S6 已覆盖",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-23T08:24:59.916Z",
    "resolved_at": null
  },
  {
    "id": 19,
    "kind": "deviation",
    "phase": "07",
    "file": "internal/server/server.go",
    "line": null,
    "description": "07-01 Rule 1 调和：无认证分支 bp 形态补注册 attach path-only 405 fallback（bp==\"\" 根挂载保持 embed 404 零漂移，差异注释锚定，23d72c8）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-26T00:47:53.956Z",
    "resolved_at": null
  },
  {
    "id": 20,
    "kind": "deviation",
    "phase": "07",
    "file": "web/dist/index.html",
    "line": null,
    "description": "07-01 Rule 3 验收闸适配：dist 升级前缀 grep 改引号无关心形态（esbuild 反引号模板字面量发射，字面量未重命名，断言面守恒，4f1fc8e）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-26T00:47:54.121Z",
    "resolved_at": null
  },
  {
    "id": 21,
    "kind": "deviation",
    "phase": "07",
    "file": "web/uat/phase07.mjs",
    "line": null,
    "description": "07-07: S4c NEL 探针改线形等价物构造（undici latin1 编码实证——朴素 'ali\\u0085ce' 上线单字节 0x85 复现不了剥离路径，'ali\\u00C2\\u0085ce' 双码点 = UTF-8 NEL 等价线形）",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-26T06:30:22.013Z",
    "resolved_at": null
  },
  {
    "id": 22,
    "kind": "unrun-verify",
    "phase": "07",
    "file": "web/uat/phase07.mjs",
    "line": null,
    "description": "S8c 真实弹浏览器拉起与标签页观感 skipped（headless 硬约束豁免——真实 GUI 属 Windows 工作站人工层，CODEBUDDY.md 平台原生行为豁免条款）；协议层等价物 S8a（headless 跳过）+ S8b（fake xdg-open argv 全等）已覆盖，人工复核项见 07-08 plan",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-26T06:30:42.875Z",
    "resolved_at": null
  },
  {
    "id": 23,
    "kind": "deviation",
    "phase": "09",
    "file": ".planning/phases/09-release-polish/09-01-PLAN.md",
    "line": 118,
    "description": "09-01 plan verify 块 goreleaser 产物目录路径（dist/<os>_<arch>/）与 v2.18.0 实证布局（dist/wesh_<os>_<arch>_<variant>/）不符——执行已按实证路径，断言全绿；PLAN 文档字面未改（历史产物）",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-29T13:45:46.256Z",
    "resolved_at": "2026-08-29T13:46:51.606Z"
  },
  {
    "id": 24,
    "kind": "deviation",
    "phase": "09",
    "file": "web/uat/pw/phase09-caddy-ctl.sh",
    "line": 26,
    "description": "09-08: Caddyfile 站点地址 http://0.0.0.0:10014 系字面 Host 匹配（仅 Host: 0.0.0.0 命中，真实主机名落空走兜底空 200）——Task 1『外部 Host 照常被服务』结论系 proto-verify 探针 curl 0.0.0.0 自身的假绿，Windows 首跑 0/2 捕获；改裸 :10014（绑定全网卡+匹配任意 Host）三层复验全绿：LAN IP 矩阵 + proto 7/7 + Windows 4/4（28ae2f2）",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-30T03:53:43.425Z",
    "resolved_at": "2026-08-30T03:53:54.769Z"
  }
]
````
