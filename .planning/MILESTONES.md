# Milestones

## v1.1 per-client 会话模式 (Shipped: 2026-09-08)

**Phases completed:** 5 phases, 38 plans, 69 tasks
**Timeline:** 2026-09-02 → 2026-09-08（7 天，232 commits，+43,844/-5,778，Go 27,844 LOC）
**Requirements:** 15/15 闭合（PC-01..13 + SEC-09 + OPS-12）

**Key accomplishments:**

1. **Phase 10 — 模式装配与接缝**：`--session-mode` 阀门一次装配（flag/TOML 键/枚举闸/Options.SessionMode/SpawnFunc/run() 装配期分岔/pty.StartWithSize），全部接缝 inert、shared 逐字节零回归；ValidateOptions 互斥 fail-fast + 启动预检 --cwd 感知（10-05 gap closure：WR-01/WR-02）。
2. **Phase 11 — per-client 生命周期主干**：attach 独立 spawn（Hello 尺寸钳制直通）→ 断开 SIGHUP teardown 恰好一次（KILL 兜底）→ EXIT 帧私有化（exit_code/信号 -1）；pre-spawn 容量再闸 + 注册点复检回收使「并发子进程数 ≤ maxClients」硬不变量竞态注入成立；darwin kqueue dup-watch fail-closed；G-11-2 EPERM 容忍语义 gap closure（探针参数化）。
3. **Phase 12 — 交互与背压语义**：Welcome.session 模式位恒序列化 + per-client 重连 terminal.reset() 清旧屏；RESIZE 直通（debouncer 单组件双消费、零 'W' 约束帧）；ro 双闸配对放行；停读/续读背压（outbox notFull 恢复信号 + 零锁阻塞持帧 + dwell 10s 看门狗 → 1013）——WR-01 按「dwell 涵盖不复刻」闭合。
4. **Phase 13 — 资源防线与终结语义**：spawn 双令牌桶（全局 8/16 + per-IP 1/4）；per-client stop-timeout 双默认值（未设 5s / 显式 0 尊重+warn）；pcSupervisor 第二终结源 + last-reaped-code 退出码双时序对齐；Shutdown N 进程组快照逐组信号 + 有界 join + D-state 兜底 terminate；metrics 四计数器 + 审计 pid 归因（零身份 label 红线保持）；WESH_REMOTE_USER 注入（SEC-09）；churn 负载格 RSS/goroutine/fd 有界。
5. **Phase 14 — 双模式验证矩阵与 herdr UAT**：server 包 32 文件三维归类（18 文件 81 测双跑 = 216 mode= 子测 -race 全绿）；run-all.mjs 17 项 UAT 一键矩阵 runner；herdr 全链收口（协议层 18/18 + Windows Playwright 观感 4/4 + 截图六帧人工复核）；负载矩阵实测回填标定表（maxClients=32 实证不动，README 三档建议值）；README/ARCHITECTURE/CONFIGURATION 双模式文档三件套 + GoTTY 误记修正；G-14-34 mermaid 词法修复 gap closure（check-mermaid.mjs 载具常驻）。

**Known deferred at close (override_closeout，用户裁决 2026-09-08):**

- TestMaxClients503/mode=per-client 隔离复跑 flake（pcSessions linger 窗口竞态；全量 -race 与 CI 命令不受影响，修复方向三条已登记 deferred-items.md）
- GOROOT gofmt CJK 注释三处（实际已修 5310723，登记状态未清）
- debug session knowledge-base 状态 unknown

---

*注：plan 级逐条成就见归档 [milestones/v1.1-phases/](milestones/v1.1-phases/) 各 *-SUMMARY.md。*
