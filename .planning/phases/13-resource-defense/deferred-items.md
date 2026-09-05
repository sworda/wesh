# Phase 13 Deferred Items（范围外发现登记）

执行期发现的既有问题（非本 phase 各 plan 改动引入），按 executor SCOPE BOUNDARY
纪律不修、只登记。13-08 收口闸 diff 审查与 gofmt 闸时应知悉。

**13-08 处置记录**：下表三处 gofmt 存量已在 13-08 收口闸段① 以 `style(13-08)` 提交
（5310723）归一——收口闸 must_haves「GOROOT gofmt 零输出」要求清零；均为注释/空行
修正零断言触碰，且两文件均在 phase 白名单内。登记保留作历史轨迹。

| 发现 | 位置 | 引入 | 说明 | 登记于 | 处置 |
|------|------|------|------|--------|------|
| GOROOT gofmt（go1.26.3）CJK 标点接续注释命中：`//（fs.Visit 第八位...` 行需补空格 | cmd/wesh/main_test.go（TestStopTimeoutResolution doc 注释） | 13-01（bfe6291） | 13-01 SUMMARY 记「gofmt -l cmd/wesh/ 零输出」用的是 PATH 旧版 gofmt；GOROOT gofmt（10-05 收口闸工具）命中一行。一行注释空格修正，无行为面 | 13-02 | 13-08 已修（5310723） |
| GOROOT gofmt 同类命中：`//（洪水后令牌恢复...` 行需补空格 | internal/server/perclient_test.go:1535（限速洪水测 doc 注释） | Phase 12（f38a170） | 同上——PATH gofmt 双 clean 时代提交的存量行；13-02 Task 2 重建 diff 时刻意保持既有形态（白名单纯新增纪律） | 13-02 | 13-08 已修（5310723） |
| GOROOT gofmt 命中：dialHelloWithXFF 声明前双空行（顶层声明间应单空行） | internal/server/perclient_test.go:2007-2009 | 13-02（202348e） | 13-03 执行期新发现（13-02 SUMMARY 的 gofmt 核查遗漏——PATH gofmt 对该形态亦有命中，推测 13-02 核查时被 perclient_test.go 整体单文件命中遮蔽后逐 hunk 未审）。纯空行修正，无行为面 | 13-03 | 13-08 已修（5310723） |
