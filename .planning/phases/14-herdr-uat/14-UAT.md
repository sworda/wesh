---
status: diagnosed
phase: 14-herdr-uat
source: [14-VERIFICATION.md]
started: 2026-09-07T01:31:45Z
updated: 2026-09-08T10:45:00Z
---

## Current Test

[testing complete]

## Tests

<!-- #1-29: coverage 模式自动化覆盖项（7 个 SUMMARY 全部 all_auto_covered，验证引用见各自 SUMMARY frontmatter coverage 块） -->

### 1. [14-01 D1] newTestServer 小族四形态装配点
expected: harness_test.go 四形态（普通/tracked/handle/sess）+ defaultPCSpawnFn 生产闭包镜像，TestExitFrame 双模式 PASS + vet/gofmt 零输出
result: pass
source: automated
coverage_id: 14-01/D1

### 2. [14-01 D2] startPerClientServerTrackedWithSpawn 姊妹变体
expected: per-client 族 handler 追踪补齐，TestOversize1009/mode=per-client tracked 形态运行期首证
result: pass
source: automated
coverage_id: 14-01/D2

### 3. [14-01 D3] exit_test EXIT 广播双模式分叉表
expected: shared 全员广播 v1.0 逐字 / per-client 属主私有化 + 他端零感知（PC-04），4 子测试 PASS
result: pass
source: automated
coverage_id: 14-01/D3

### 4. [14-01 D4] stopseq 两测双跑
expected: 信号序列发到该会话自身进程组，exitf 收口链分叉（lifecycle 直收 vs pcSupervisor last-reaped-code）
result: pass
source: automated
coverage_id: 14-01/D4

### 5. [14-01 D5] health 双模式
expected: TestHealthz 五子测 + TestHealthzDraining 双跑；session_active 语义分叉（shared 生命周期跟随 / per-client 恒 true）
result: pass
source: automated
coverage_id: 14-01/D5

### 6. [14-01 D6] slowclient 双模式
expected: TestSlowConsumerKick 满即踢分叉（R-08 1:1 退化）+ TestGlobalCredit D-03 信用门未装配可证伪
result: pass
source: automated
coverage_id: 14-01/D6

### 7. [14-01 D7] limits 四测双跑
expected: TestOversize1009 经 newTrackedTestServer + 洪水类同断言双跑 + TestReadLimitBoundary 单跑 D-02 偏差登记
result: pass
source: automated
coverage_id: 14-01/D7

### 8. [14-02 D1] e2e 生命周期测双模式断言分叉表
expected: 断开/重连语义两列显式成表（shared=进程存活可重连 / per-client=SIGHUP 杀进程组+新 pid），CORE-05 反转面可证伪
result: pass
source: automated
coverage_id: 14-02/D1

### 9. [14-02 D2] fanout/EXIT 广播双模式分叉
expected: shared 逐字节一致扇出 / per-client 双标记串交叉断言 + 私有化他端零感知
result: pass
source: automated
coverage_id: 14-02/D2

### 10. [14-02 D3] owner 四测 D-03 未装配可证伪列
expected: B 恒 rw + 零升格 Welcome 静默窗 + 重连直接 rw + 零扇出旁观端，零 t.Skip
result: pass
source: automated
coverage_id: 14-02/D3

### 11. [14-02 D4] MaxClients spawn-intent 口径
expected: 503 闸先于 spawn——wesh_pty_spawn_total 程序序精确对照（==2 拒绝点 / ==3 cE 后）
result: pass
source: automated
coverage_id: 14-02/D4

### 12. [14-02 D5] Sigwinch D-03 列 + 尺寸语义分叉
expected: shared-only 装配可证伪锁 + TestWelcomeSessionDims 恒等式 vs min-rect 分叉
result: pass
source: automated
coverage_id: 14-02/D5

### 13. [14-02 D6] shared 列零回归证据
expected: 期望值文案逐字存活（e2e 29 + multi 58）+ 母本函数零 diff + 依赖面零 diff
result: pass
source: automated
coverage_id: 14-02/D6

### 14. [14-03 D1] emptyexit 九测双模式归一
expected: 第二终结源列显式成表（14 子测试 PASS）
result: pass
source: automated
coverage_id: 14-03/D1

### 15. [14-03 D2] shutdown 六测双模式归一
expected: N 组信号 + 有界 join 列 + startShutdownServerWith 散点删除
result: pass
source: automated
coverage_id: 14-03/D2

### 16. [14-03 D3] metrics 六测双模式归一
expected: session_active 语义 + series 镜像 + HELP 文案三分叉面（30 子测试）
result: pass
source: automated
coverage_id: 14-03/D3

### 17. [14-03 D4] 双证据链零削弱
expected: shared 列与 Phase 13 per-client 断言两列字面均逐字保持
result: pass
source: automated
coverage_id: 14-03/D4

### 18. [14-04 D1] resize_arb D-03 双模式改造
expected: shared 四子测试仲裁断言逐字 + per-client 未装配三重可证伪断言 + startResizeServer 收编删除（全仓零引用）
result: pass
source: automated
coverage_id: 14-04/D1

### 19. [14-04 D2] shared 列期望值逐字一致
expected: 剥离缩进 diff 自审——四子测试断言行零改动
result: pass
source: automated
coverage_id: 14-04/D2

### 20. [14-04 D3] 纯白盒面判定登记
expected: clients_test/resize_test 六测零改动单跑 + 蓝本三则归属偏差 CONTEXT 回写
result: pass
source: automated
coverage_id: 14-04/D3

### 21. [14-04 D4] 全量回归零回归证据
expected: 包 -race 全量 ok 147.067s + 全仓 5 包 ok
result: pass
source: automated
coverage_id: 14-04/D4

### 22. [14-05 D1] 协议守卫批双模式同断言双跑
expected: handshake 7 测 + keepalive 3 测，20 个 mode= 子测试全 PASS；唯一分叉面 TestReadOnlyAllowsResize 按断言分叉表
result: pass
source: automated
coverage_id: 14-05/D1

### 23. [14-05 D2] 认证面批双模式同断言双跑
expected: auth_e2e 9 测双跑（18 mode= 子测试）+ 纯函数 6 测单跑判定登记；断言字面量全集 diff 零差异
result: pass
source: automated
coverage_id: 14-05/D2

### 24. [14-07 D1] 洪水格 TestLoadPerClientFloodMatrix 四档
expected: N∈{1,4,16,32} 每会话独立洪水 + spawn_total==N + kicks==0 + 收流完整（四连跑 16 格全绿）
result: pass
source: automated
coverage_id: 14-07/D1

### 25. [14-07 D2] 驻留格 TestLoadPerClientResident 四档
expected: 差值断言三面（mem/gor/fd 账面证真）+ VmRSS ≤15MB/进程（三连跑 12 格全绿）
result: pass
source: automated
coverage_id: 14-07/D2

### 26. [14-07 D3] 既有负载格零回归
expected: note() 夹具修正后全套件 -tags=load PASS（3m22s）
result: pass
source: automated
coverage_id: 14-07/D3

### 27. [14-08 D1] S1 herdr driving 场景
expected: herdr area 翻转链四步 + 流层增量互证（254B/55B << 97049B）+ wesh 层三断言（双 session_start 双 pid / 双端 Welcome per-client / maxCol 120 vs 40）
result: pass
source: automated
coverage_id: 14-08/D1

### 28. [14-08 D2] S2 ro 汇聚场景
expected: rw 桌面 + ro 移动经 server 汇聚同会话——ticket 全链唯一通道 + ro INPUT 后 pane read 逐字不变 + rw 对照标记可见
result: pass
source: automated
coverage_id: 14-08/D2

### 29. [14-08 D3] 红线自净 + 门禁收口
expected: assertOutputClean 五面零泄漏 + exit code 门禁负对照自证（exit 1）+ 双会话清理零残留
result: pass
source: automated
coverage_id: 14-08/D3

<!-- #30-33: 人工检查点（14-06 legacy prose 提取 + 冷启动复验） -->

### 30. 冷启动全仓 -race 回归（五包）
expected: time go test -race -count=1 ./... 从当前工作区执行，五个包全部 ok 零 FAIL；internal/server 包 ~150-160s（Phase 14 双模式矩阵既定基线）
result: pass
source: automated-re-run
evidence: "2026-09-07T01:50Z 实测：cmd/wesh 1.336s / internal/proto 1.016s / internal/pty 2.655s / internal/server 157.060s / internal/web 1.011s 五包全 ok，零 FAIL，exit 0（-v 形态跑全仓，总时长 2m37s）；server 包时长落在既定基线内；TestMaxClients503 全量形态绿（deferred-items.md 登记口径一致）"

### 31. 部署面测试批双模式（14-06）
expected: go test -race -count=1 -v -run 'TestShareToken|TestCustomIndex|TestBasePath|TestRemoteUserLogging|TestXFFThrottleKey|TestAuthHeaderNoAuthBypass|TestShareChannelRemoteUser' ./internal/server/ → 9 测全 PASS，每测含 mode=shared/mode=per-client 双列子测试
result: pass
source: automated-re-run
evidence: "2026-09-07T01:34Z 实测：9 测全 PASS（TestShareToken 0.68s / TestBasePathRoutes 0.33s / TestBasePathWS / TestBasePathEmptyUnchanged 0.11s / TestCustomIndex 0.10s / TestRemoteUserLogging / TestXFFThrottleKey / TestAuthHeaderNoAuthBypass / TestShareChannelRemoteUser），全部含 mode=shared+mode=per-client 双列，ok 2.318s exit 0"

### 32. SC1 三维归类收口核对：mode= 子测试计数（14-06）
expected: go test -race -count=1 -v ./internal/server/ 2>&1 | grep -cE -- '--- PASS: .*mode=' == 216（18 文件 81 测双跑存量 + 嵌套子测的收口核对口径）
result: pass
source: automated-re-run
evidence: "2026-09-07T01:50Z 实测：全仓 -v 运行（/tmp/uat14-full-v.log）中 '--- PASS: .*mode=' 计数恰为 216，与 14-06 SUMMARY 收口核对口径逐字一致"

### 33. herdr 协议层 UAT 端到端复跑（14-08 交付物冷启动）
expected: node web/uat/phase14.mjs → S1 driving + S2 ro 汇聚两场景 18/18 断言全绿，exit code 0，herdr 会话清理零残留
result: pass
source: automated-re-run
evidence: "2026-09-07T01:35Z 实测：18/18 协议断言通过，exit 0；herdr 0.8.100；关键数据与 14-08 SUMMARY 记录逐项一致（增量 254B/55B、全量 97049B、maxCol 120/40/74、双 session_start 双 pid）；S1j/S2g 会话零残留 + SEC 输出自净零命中"

### 34. mermaid 渲染目检（14-11 D3 递延项 / 14-VERIFICATION human_verification）
expected: GitHub 或 mermaid 渲染器打开 docs/ARCHITECTURE.md，双模式架构节两个 goroutine 拓扑图正常渲染（subgraph 成形、边箭头完整、无错误占位）
result: issue
reported: "文档中的组件图mermaid部分无法正常渲染，排查下是否有语法错误"
severity: major
source: human
evidence: "14-11 SUMMARY：mermaid 结构化校验通过，渲染目检因 Linux 侧禁浏览器显式递延至验证阶段；14-VERIFICATION.md 列为唯一 human_verification 项"

## Summary

total: 34
passed: 33
issues: 1
pending: 0
skipped: 0

## Gaps

- gap_id: G-14-34
  truth: "docs/ARCHITECTURE.md 中 mermaid 图在 GitHub/渲染器正常渲染（无错误占位）"
  status: failed
  reason: "User reported: 文档中的组件图mermaid部分无法正常渲染，排查下是否有语法错误"
  severity: major
  test: 34
  root_cause: "组件图（第一个 mermaid 块，L11-56）存在两类 mermaid 语法错误（Node 端 mermaid.parse 11.17.2 实证）：(1) L15/L19/L29 三处 subgraph 的 id 含非法字符 /（subgraph cmd/wesh（CLI 装配层）、internal/server（网关层）、internal/pty（数据面））触发词法错误 'Lexical error: Unrecognized text'；(2) L41 边标签 |GET / · /s/{token}/| 中 { 被词法解析为 DIAMOND_START（菱形节点起始符）触发 parse error。第二块双模式拓扑图（L98-132）用规范形式（subgraph ID[\"标题\"] + -->|\"标签\"|）parse PASS 无问题。14-11 的结构化校验只查结构完整性（节点数/边目标存在/subgraph-end 配对/围栏成对），不查 mermaid 词法合法性，因此漏检"
  artifacts:
    - path: "docs/ARCHITECTURE.md"
      issue: "L15/L19/L29 subgraph id 含 /；L41 边标签 {token} 未加引号"
  missing:
    - "4 个 subgraph 改为合法 id + 引号标题：CMDWESH[\"cmd/wesh（CLI 装配层）\"] / SRV[\"internal/server（网关层）\"] / PTYLAYER[\"internal/pty（数据面）\"] / WEBPKG[\"web（前端装配）\"]（新 id 不与现有节点冲突）"
    - "所有含特殊字符的边标签加引号（-->|\"...\"|，与双模式拓扑图 L98 块风格统一）"
    - "修复后用 Node 端 mermaid.parse 复验（/tmp/mermaid-check 已预验证修复草案 parse PASS flowchart-v2）"
  debug_session: ""
