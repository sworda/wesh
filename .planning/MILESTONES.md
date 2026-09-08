# Milestones

## v1.1 per-client 会话模式 (Shipped: 2026-09-08)

**Phases completed:** 5 phases, 38 plans, 69 tasks

**Key accomplishments:**

- --session-mode flag/TOML 键/parse 枚举闸（D-04 文案）+ Options.SessionMode/SpawnFunc 接缝 + ValidateOptions 互斥 fail-fast + run() 装配期一次分岔 + pty.StartWithSize 导出——CLI/TOML 输入 → 校验 → Options → server.New 完整路径一次打通，全部接缝 inert 零 per-client 运行期行为，v1.0 全量 -race 原样绿。
- write-policy×per-client warn（D-01/D-02）与 per-client LookPath 启动预检（SC4）落地，warn 累积合并形态零遮蔽既有安全警告；ValidateOptions 三态与 StartWithSize 委托等价经新测试锁定，shared 路径零漂移（全量 -race 绿）。
- TOML 侧 merge 铺底/CLI 覆盖/三层优先级链/下划线拒绝/非法枚举同文案/类型不符六面全部经表驱动锁定（append-only），fuzz 五种子落地且 30s 短跑 3.04M execs 零崩溃零红线破口。
- session-mode 文档五处 + README 一句落地（D-05 最小明示，--help 口径一致）；收口闸六段全 PASS——Go 全量 -race 绿 + 既有八 UAT 脚本原样全绿的零回归双证据成立，SpawnFunc 零调用方 inert 证明闭合。
- 10-VERIFICATION 唯二缺口（WR-01 Blocker / WR-02 Warning）断言优先闭合——六形态进程级冒烟矩阵实证 truth #5 由 FAILED 转 PASS（per-client × --cwd × ./run.sh 由 exit 2 误拒转 listening on），ValidateOptions 位序 V<P<L 数值锁定，fix 提交后 main HEAD 零回归双证据（-race 五包 + 八 UAT 原样）首跑全绿。
- per-client 模式端到端打通：attach（ticket 核销）→ hubMu 外 spawn 独立 PTY（Hello 钳制尺寸直通）→ Welcome 回显 → 输入/输出双向管道 → 断开即 SIGHUP（teardown Once 全序列含 KILL 兜底）→ 子死仅本端私有 EXIT+1000；shared 模式逐字节零回归（全量 -race 绿 + 八 UAT 脚本基线对齐）。
- darwin 共享 kqueue exitWatcher 的 Pitfall 9 挂账兑现：watch() 对重复 pid 注册 fail-closed（errDupWatch），影子注册从「会话收割挂死」变「可观测错误 → awaitExit 既有分支退化 cmd.Wait() 兜底」；TestWatchDupPidFailClosed append-only 锁进 CI macOS leg；Linux 侧零漂移、全仓 -race 全绿。
- per-client 昂贵资源（每连接一进程）硬帽机制落地：D-02 pre-spawn 容量再闸（1011+「server is at capacity」wire 形态，与 spawn 失败同码同串、事件名细分）+ D-03 注册点复检回收（reapOrphanSession 完整 SignalGroup→Drain→Close→Wait 序列，「并发子进程数 ≤ maxClients」硬不变量竞态注入实测成立，Phase 13 裁决项④提前消解）+ spawn 失败 Pitfall 5 清理清单逐条测试锁定；shared 模式全量 -race 原样绿、既有测试零改动。
- per-client 生命周期全部可观测面六测锁定：EXIT 私有化两形态（exit 42 / 信号死亡 -1，本端末帧断言 + 他端 1.5s 零帧强形态）、断开 SIGHUP 无宽限无僵尸（pgid ESRCH + pcSessions 收敛）、重连新 pid、D-01 KILL 兜底时序双断言（trap 免疫下实测断开→ESRCH 1.0055-1.0059s，stop-timeout=1s 精度）、teardown 恰好一次 10 轮竞态注入（quiescent 四件套 + exitf 零调用 + 零 panic）；Pitfall 2/3/8 的 Phase 11 侧防线全部收口，shared 全量 -race 原样绿、既有测试逐字未动。
- D-06 八场景一次建齐全绿：web/uat/phase11.mjs（595 行零依赖协议层 UAT）对真实二进制实证 PC-02/PC-03/PC-04 全部用户可观测面——双端独立 pid + 启动期零子进程（pgrep 实证 spawn 点后置）、首帧 winsize 111x44 直通（无 80x24 中间态）、运行期删命令 spawn 失败 1011 他端零影响、断开 pgid ESRCH 无僵尸、EXIT 私有化两形态他端 1.5s 零帧、容量再闸 linger 注入 1011 逐字文案、重连新 pid、trap 免疫 KILL 兜底时序双断言；21/21 PASS ×3 连跑无 flake，与 Go 测试（11-03/11-04）形成双证据对照。
- Phase 11 零回归收口闸六段式一次跑齐全绿：静态面（GOROOT gofmt 零输出/vet 干净/build 0.570s）+ 全量 -race 5 包 1m5.656s（shared 原样绿 + per-client 十四测 -v 名单对账 5+3+6=14 全 PASS）+ GOOS=darwin build/vet 双闸 + 既有八 UAT 脚本两轮零修改重跑 exit 全 0（PASS 计数 12/18/10/28/23/34/21/18 与 v1.0 基线逐脚本一致）+ phase11.mjs 八场景 21/21+1 skipped + 期望值逐字未动 diff 审查四件套全过（既有测试零修改/append-only 零删除行/新增白名单三文件/红线六面零出现/依赖零漂移）；prohibitions 19 条人工确认零违反，PC-02/PC-03/PC-04 双证据链闭合并勾选。
- G-11-2 闭合：waitPgroupESRCH 重构为探针参数化双层形态（waitPgroupESRCHWithProbe 核心 + 薄包装），EPERM 按 POSIX「目标存在」语义归类存活形态落入护栏轮询——护栏保留与他错立即 Fatal 两半边由四子测确定性锁定；CI 复验链闭环（33844831146 双平台全绿，FAIL 现场测试转绿）；副产 PS1 交错容忍修正三处。
- Welcome 帧加 session 模式位恒序列化键（proto→五组帧调用点→wire→前端解析→per-client 重连静默 terminal.reset() 清旧屏残影），jsdom alt-screen 判别通道端到端锁定
- per-client RESIZE 直通本会话 TIOCSWINSZ（debouncer 单组件双消费 + 每会话 50ms 防抖）+ ro 双闸配对放行（服务端 D-06 直通分支 × 前端 D-07 模式位闸）+ ro INPUT 门控与限速保留，Go 五测 + jsdom D2 双向断言锁定，shared 路径 diff 纯新增零回归
- per-client 输出闭包从「trySend 失败直踢 1013」改造为「outbox notFull 恢复信号 + 零锁阻塞持帧 + dwell 看门狗」——停读不丢数据（帧在闭包栈上，内核缓冲积压子进程写阻塞，ttyd pty_pause parity）→ 恢复自动续读（每次续读重置计时）→ 持续过载 dwell 到期才 1013；WR-01（STATE.md:99）按 D-04「dwell 涵盖不复刻」形态在代码与注释双侧闭合
- phase12.mjs（754 行，phase11.mjs 同构骨架 + RawStallClient raw socket 停读夹具）以真实二进制在真实协议面上锁定 Phase 12 五需求 wire 行为——Welcome.session 双模式、resize 直通隔离、ro 双闸配对、ro 门控+限速、停读续读 34.9MB 字节级连续、真实 10s+ dwell 1013；两轮连跑 20/20 退出码 0
- Phase 12 零回归收口闸六段式一次跑齐全绿：静态面（GOROOT gofmt 零输出/vet 零输出）+ 全量 -race 5 包 1m21.441s（Phase 12 新测 12/12 逐名 PASS）+ GOOS=darwin amd64/arm64 双编译闸 + web 构建 dist byte-identical + UAT 矩阵 16 轮全绿（既有 10 协议脚本默认 shared 零修改重跑与基线逐脚本一致 + 3 jsdom + phase12.mjs 两轮 20/20 + phase12-dom 14/14）+ 期望值逐字未动 diff 审查（白名单三处吻合 + 补充项①如实登记 + 放宽形态零命中 + 红线文件零 diff + 零新依赖终审）；PC-05/06/07/10/11 五需求勾选收口，WR-01 按 D-04「dwell 涵盖不复刻」形态闭合回指登记。
- Task 1（checkpoint:decision，D-01 one-way 确认门）
- Task 1（tracer，feat 46f28ac，TDD——RED 观察于任务内完成即转 GREEN，11-01 先例单 feat 提交）
- Task 1（机制，feat 2c5fd6e）
- Task 1（机制，feat fe3ca4a）
- Task 1（feat 1685485，TDD——RED 观察于任务内完成即转 GREEN，11-01/13-02/13-03 先例单 feat 提交）
- Task 1（feat a09f756，TDD——RED 编译红观察于任务内完成即转 GREEN，11-01/13-05 先例单 feat 提交）
- Task 1（test 16d16f6）
- Phase 13 零回归收口闸六段式一次跑齐全绿：静态面（GOROOT gofmt 零输出——三处 deferred 存量归一 + go vet 零输出）+ 全量 -race 5 包 1m37.2s（新测 26 测函数逐名 PASS）+ darwin amd64/arm64 双编译闸 + dist byte-identical + UAT 矩阵 17 轮全绿（既有 15 脚本零修改基线一致 + phase13.mjs 两轮 29/29）+ churn 负载格 + diff 白名单审查（23 文件全白名单内/放宽形态零命中/零新依赖 0 行 diff）；PC-08/PC-09/SEC-09/OPS-12 四需求勾选收口，Key Decisions 两行 + D-10 文档段落地，WR-02 闭合回指登记。
- newTestServer 四形态小族装配点收编 shared/per-client 两装配族 + per-client tracked 姊妹变体，五文件 13 测 36 个 mode= 子测试 -race 双模式逐名绿，shared 列期望值逐字未动（D-02 零回归证据）
- e2e_test.go 7 测 + multi_test.go 11 测双模式断言分叉表落地——断开/重连（CORE-05 反转面）、fanout 双标记串交叉断言、EXIT 私有化他端零感知、owner 四测 D-03 未装配可证伪列、MaxClients spawn-intent 程序序对照，50 个 mode= 子测试 -race 逐名绿，shared 期望值 87 条文案逐字零漂移
- mode-mapped 终结/观测批归一落地——21 测收编为 16 测试体（7+4+5）经 newTestServer 小族双 t.Run：第二终结源列（exitf(42)/(-1) 与触发事件 0/1 真值相反对照）、N 组信号 + 有界 join 列（session_end==N/时长双锚）、session_active 语义 + series 镜像 + HELP 三分叉面显式成表；三文件 16 处两装配族调用点归一，startShutdownServerWith 散点删除；48 个 mode= 子测试（本批）+ 52 含 stopseq 既有 -race 逐名绿，shared 列与 Phase 13 per-client 断言两列字面逐字保持
- resize_arb_test.go 落 D-03 双模式形态——shared 列四子测试仲裁语义逐字（min-rect/2to1/防抖/参与集与 ro 闸/尺寸推送），per-client 列仲裁器未装配三重可证伪断言（异尺寸双端各自直通 + 无 min-rect 收敛 + 零 'W' 帧）；startResizeServer 收编删除（全仓代码面零引用）；clients_test/resize_test 六测纯白盒单跑判定 + 蓝本三则归属偏差登记落地（SUMMARY + CONTEXT 双通道）
- 协议守卫 10 测 + 认证面 9 测共 38 个 mode= 子测试双跑落地（两列期望值逐字一致机器自审），纯函数 6 测形态判定单跑登记，auth_e2e :403 特殊调用点收编 newTrackedTestServer，startTestServer 兼容包装零引用收编删除——mode-agnostic 前半批（D-01）收口
- 实测口径
- 洪水格四档每会话独立 seq 洪水（spawn_total 程序序对照 + 零误踢 + 收流完整逐端一致）与驻留格四档 idle 双采样差值三面断言（mem/gor/fd 账面全部证真）落地，D-11 两段式裁决：32 会话资源实测远低于账面——maxClients=32 默认值不动，LOADDATA 八行数据产出供 14-11 README 标定表回填。
- phase14.mjs 两场景 18 断言两轮全绿：herdr api snapshot area 翻转链（is_foreground 仲裁 + per-client area 渲染）+ 流层增量互证 + wesh 层三断言三通道锁定「移动端 attach 桌面端不被压缩」，ro 汇聚经 pane read 实证输入门控——PC-13 协议面收口（v1.1 存在意义的可证伪证据）
- Windows 工作站双 tab 真实 Chromium 观感收口：两轮 4/4 全绿（T0 进程自证/T1 attach 不压缩/T2 resize 不压缩/CL 零残留）+ 截图六帧人工复核通过——PC-13 浏览器面收口；执行期海森bug（浏览器 tab 首键入后输出流非确定性死亡）经 10 轮实跑 + 23 探针裁决为载具级（herdr/wesh 服务端无罪），架构绕道为裸 WS 驱动端 + 被动渲染观测面，并交付 minrepro 双件套供 herdr 侧定位
- 17 项 UAT 一键矩阵 runner（串行 spawn 聚合 + 超时护栏 + 汇总表）——全矩阵首跑 17/17 全绿 213.4s，逐脚本计数与 13-08 基线完全一致，D-04 零修改重跑形式化与 D-13 SC2 证据成型。
- README「会话模式」节（模式语义表/三语义行/herdr 配方/tmux 对照句 + 14-07 LOADDATA 八行实测标定表双剖面 + max-clients 建议值表）+ ARCHITECTURE「双模式架构」段（七分支点分岔说明 + goroutine 拓扑 mermaid 对比图 + keep/degrade/vanish 表）与 :7 GoTTY 误记修正 + CONFIGURATION max-clients per-client 语义行——PC-12 三件套收口，全部数据可溯实测、配方与 phase14.mjs argv 逐字一致。
- Phase 14 零回归收口闸六段式一次跑齐全绿：静态面三命令零输出 + 全量 -race 五包 2m37.986s（mode= 子测 216/216 与 14-06 审计一致）+ darwin 双编译闸四命令零错 + dist byte-identical + UAT 矩阵 17/17 零修改重跑基线逐脚本一致（人工闸 user approved）+ diff 白名单终审 35 文件零外改动（565 真删除行全归类、红线四查全零）；PC-12/PC-13 勾选收口——v1.1 里程碑 15/15 需求全闭合，Key Decisions 三行登记 + flagged_assumptions 终验落档。
- ARCHITECTURE.md 组件图 16 行词法修复（subgraph 合法 id + 引号标题 + 12 处边标签引号化含 L41 `{token}` DIAMOND_START 主修复点）红转绿，词法校验载具 check-mermaid.mjs 常驻固化，用户渲染目检 approved——UAT gap G-14-34 / test 34 闭合（34/34）

---
