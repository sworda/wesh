# Phase 07: 部署与配置 - Deferred Items

执行期发现的范围外事项登记（SCOPE BOUNDARY 纪律：不随发现 plan 修复，按既定路由后续批次清零）。

| Date | Plan | Category | Item | Status |
|------|------|----------|------|--------|
| 2026-08-26 | 07-01 | gofmt 漂移 | internal/server/multi_test.go 与 internal/server/slowclient_test.go 存在 HEAD 既有 GOROOT gofmt 漂移（git show HEAD 版本复验确认非 07-01 引入；CJK 注释 `//（` → `// （` 空格规则差异家族——01-03「/usr/bin/gofmt 陈旧须用 GOROOT 版本」与 05-09 九文件清零同族，02-06/03-06 六段式段 1 独立 style 提交先例为清零路由） | open |
| 2026-08-27 | 07-05 | UI 文案 | 1001 关停面板 hint 与 systemd Restart=always 部署形态不匹配——wesh 自重启时「Start wesh again from your shell」为无效指引；修：web/src/main.ts:903 按场景条件化 hintPrefix（如 "If wesh is not restarted for you, ..."）。07-UI-REVIEW.md WARNING#1 | open |
| 2026-08-27 | 07-05 | UI 可访问性 | #status 面板族无 ARIA live 语义（phase 04/06 继承面，1001 终态同样不向辅助技术播报）；修：web/index.html:63 `#status` 加 role="alert"（单属性零视觉影响）。07-UI-REVIEW.md WARNING#2 | open |
| 2026-08-27 | 07-05 | UI 竞态（低） | pre-onopen 到达的 1001 关停落「Unable to connect / refusing new connections」文案（误述优雅关停）；可选修：web/src/main.ts:881-884 在 !opened 分支前先分派 ev.code===1001（毫秒级窗口，WARNING-low）。07-UI-REVIEW.md WARNING#3 | open |
