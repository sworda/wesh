---
schema_version: 1
open_count: 3
waived_count: 0
fixed_count: 0
total_count: 3
last_updated: 2026-08-14T02:10:23.638Z
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
  }
]
````
