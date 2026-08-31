# Phase 9: 发布与打磨 - Deferred Items

## gofmt 漂移（GOROOT gofmt 1.26.3 口径）

| Item | Found At | 说明 | 处置路由 |
|------|----------|------|----------|
| `cmd/wesh/config_test.go` GOROOT gofmt 漂移 | 09-02 Task 1（gofmt -l 检查时发现） | HEAD 版本即漂移（`git show HEAD:cmd/wesh/config_test.go \| gofmt -l` 命中），非本 plan 改动引入——纯注释排版类（CJK 宽度口径差异，/usr/bin/gofmt 陈旧版与 GOROOT 版分歧，01-03 已登记根因）；09-04 追加触达该文件但漂移 hunk 在 TestConfigRedLines 区（非 09-04 增量），保持既定路由 | 按 02-06/03-06/05-09/08-05 既定先例：随后续 plan 六段式段 1 或独立 style 提交清零，零语义改动 |
| `internal/server/clients.go` GOROOT gofmt 漂移 | 09-04 收尾（gofmt -l 全量检查发现） | HEAD 版本即漂移（`git show HEAD:internal/server/clients.go` 命中），非 09-04 改动引入——纯排版类同根因；09-04 未触达该文件（scope boundary） | 同上：随后续 plan 六段式段 1 或独立 style 提交清零 |
| `internal/server/emptyexit_test.go` GOROOT gofmt 漂移 | 09-04 收尾（gofmt -l 全量检查发现） | HEAD 版本即漂移（`git show HEAD:internal/server/emptyexit_test.go` 命中），非 09-04 改动引入——纯排版类同根因；09-04 未触达该文件（scope boundary） | 同上：随后续 plan 六段式段 1 或独立 style 提交清零 |
