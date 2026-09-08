# Phase 14 Deferred Items

执行期发现的超范围问题登记（scope boundary：与本 plan 改动无直接因果的不修，留后续 plan/收口闸知悉）。

## [14-06 执行期发现] TestMaxClients503/mode=per-client 槽位释放重 attach 偶发 1011 capacity 帧（14-02 遗留时序 flake）

- **现象**：`go test -count=N -run 'TestMaxClients503' ./internal/server/`（非 -race、-run 过滤隔离形态）高概率失败——cE 轮询 dial 升级成功后首帧得 `Error{"code":"server_error","message":"server is at capacity"}` 而非 Welcome（multi_test.go:1309 断言翻红）。
- **根因（两生产者共用 D-04 wire 聚合串，疑为后者）**：cA CloseNow 后 registry ③位槽位随 detach 即释放（轮询 dial 过 ③位、WS 升级成功），但 per-client pre-spawn 容量再闸（perclient.go hubMu 内 `len(pcSessions) >= maxClients`）在 A 会话收割完成前仍见满员 → `rejectCapacity` 发 1011 capacity 帧；轮询重试只处理 HTTP 503 形态，不覆盖 WS 级 1011 capacity 帧。另一可能生产者 = spawn 节流（同串），但本测试场景 per-IP 桶消耗 ≤3（burst 4）不应触发。
- **与本 plan 无关的证据**：multi_test.go 零改动；基线提交 9af7ce1（14-05 收口后）隔离复跑 3/3 失败（temp worktree 实证）；`-run` 过滤形态下本 plan 全部测试体不执行。全量 -race 套件两次全绿（16:44/16:48，157s 级），CI 同款命令不受影响——flake 为隔离/时序敏感形态。
- **处置**：超范围不修（executor scope boundary）。修复方向供后续：① 轮询重试扩展覆盖 1011 capacity 帧（读 Error 帧后继续重 dial）；② mutate 覆写放宽 spawn per-IP 桶（13-02 TestPerClientTeardownRaceOnce 先例）；③ cA 收割完成同步边（PCSessionsLenForTest 轮询）后再触发重 attach。14-12 收口闸知悉（全量 -race 门不受影响，但若 CI 命中同形态需按此登记归因）。
