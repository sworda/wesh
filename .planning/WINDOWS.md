---
schema_version: 1
open_count: 6
waived_count: 0
fixed_count: 1
total_count: 7
last_updated: 2026-08-20T09:37:49.973Z
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
  }
]
````
