# Phase 03-auth Deferred Items

执行期发现的范围外问题（不修、不阻塞，登记待后续处理）。

| Item | Found During | Detail | Suggested Owner |
|------|--------------|--------|-----------------|
| README:40 声称 `.gz` 随产物入库，与 `.gitignore`（`web/dist/*.gz`，自建仓 c055b41 起）矛盾 | 03-05 Task 2 | 实际从未跟踪 .gz（gzip 头嵌 mtime，每次构建字节漂移，跟踪会次次脏库）；embed.go:4 注释亦只提 index.html 入库。README 构建节措辞需修正为「index.html 入库，.gz 为本地构建旁路」 | 后续文档维护者 / Phase 9 发布整理 |
