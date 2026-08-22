---
schema_version: 1
open_count: 11
waived_count: 0
fixed_count: 1
total_count: 12
last_updated: 2026-08-22T04:15:52.151Z
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
  }
]
````
