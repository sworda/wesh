# Phase 05 Deferred Items

执行期发现的越界项（非当前 plan 任务面，登记不修复）。

## 2026-08-20（05-05 执行期）

- **[load-flake] server 包全量 -race 单次失败未复现**：05-05 Task 1 落地后首次全量
  运行 1 败（44.7s vs 典型 32s，尾部为 slow_consumer 1013 常规事件），随后 14 次
  全量 + 3 次时序簇定向压力（TestGlobalCredit/TestSlowConsumerKick/
  TestSuccessionKickRace -count=3）全绿未复现。失败形态（+12s）符合 OUTPUT 洪水
  测试内部超时窗命中机器负载抖动；05-05 diff 对 OUTPUT/信用门/踢出路径零改动
  （仅 INPUT 门链 + inputQ + input-writer + lifecycle close(inputDone)），时序簇
  测试全程不发 INPUT，共享资源仅 CPU 调度。分类：环境负载敏感 flaky（05-02 既有
  洪水测试），越界不修复。若 CI 复现需单独排查 TestGlobalCredit 15s 开门窗口。

## 2026-08-20（05-06 执行期）

- **[load-flake] 同簇第二次出现**：05-06 Task 3 全量验证首次运行 1 败（47.1s vs
  典型 35s，尾部 slow_consumer 1013 常规事件——与上方 05-05 登记同签名）；随即
  全量重跑绿（35.0s）+ 时序簇定向 -count=2 绿（34.4s）。05-06 diff 对 OUTPUT/
  信用门/踢出路径零改动（仅 /s/ 路由、attach token 分支、checkTicket 无认证携票
  分支、main.go 打印——既有测试无一走这些新路径：无凭据无 shares 时 ticket
  store 保持 nil，checkTicket 零漂移）。维持原判：环境负载敏感 flaky，越界不修复。

## 2026-08-22（05-12 执行期）

- **[gofmt-drift] GOROOT gofmt 标记 05-10 三文件 CJK 注释排版**：`$(go env GOROOT)/bin/gofmt -l .`
  输出 internal/server/clients.go、clients_test.go、resize.go——diff 纯为 `//（` →
  `// （`（全角括号前补半角空格）注释排版，零语义改动，系 05-10 提交（75e4def/9cc76f4）
  引入的 HEAD 漂移（01-03/02-06/03-06/05-09 已登记同类陷阱的又一次实例）。05-12 plan
  零 Go 文件改动，按 plan 授权跳过段 1 gofmt 清零；越界不修复，留给下一次 Go 文件
  触碰 plan 按先例独立 style 提交清零。
- **[untracked-artifact] 仓库根 `wesh` 二进制未跟踪且未被 .gitignore 覆盖**：标准构建
  命令 `go build -o wesh ./cmd/wesh` 的默认产物路径即在仓库根，历史上各 plan 验证均
  产生该文件但从未入库（构建产物不该入库）。建议后续在 .gitignore 补 `/wesh` 一行；
  05-12 面外，登记不处理。
