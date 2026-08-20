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
