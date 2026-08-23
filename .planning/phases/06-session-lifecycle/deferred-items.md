# Phase 06 — Deferred Items

| Item | Found during | Category | Disposition |
|------|-------------|----------|-------------|
| `internal/server/clients.go` / `clients_test.go` / `resize.go` 存在既有 GOROOT gofmt 漂移（CJK 注释 `//（` 排版差异，零语义——01-03/05-09 登记的 /usr/bin/gofmt 陈旧同源问题） | 06-01 Task 2 提交前检查 | pre-existing lint drift | 非本 plan 改动引入，本 plan 未授权 style 清零段（02-06/03-06/05-09 先例均由 plan 显式授权）；留待后续 plan 的六段式段 1 或独立 style 提交处理 |
