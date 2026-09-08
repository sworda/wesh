# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.1 — per-client 会话模式

**Shipped:** 2026-09-08
**Phases:** 5 | **Plans:** 38 | **Tasks:** 69
**Timeline:** 2026-09-02 → 2026-09-08（7 天，232 commits）

### What Was Built

- `--session-mode=shared|per-client` 双模式：per-connection spawn 完整生命周期（attach spawn / 断开 SIGHUP / EXIT 私有化 / 重连全新进程），shared 默认零回归
- 资源防线：进程硬顶硬不变量、spawn 双令牌桶、KILL 兜底、Shutdown N 进程组、第二终结源、退出码对齐
- 可观测性 per-client 粒度：metrics 四计数器、审计 pid 归因、WESH_REMOTE_USER 注入
- 双模式验证体系：32 测试文件三维归类（216 mode= 子测双跑）、17 项 UAT 矩阵 runner、herdr 全链 UAT（协议层 + Windows Playwright）
- 负载矩阵实测标定：maxClients=32 实证不动，README 三档建议值表

### What Worked

- **接缝先行（Phase 10 inert 阀门）**：模式阀门与全部挂点一次装配、全部 inert，后续 4 个 phase 无散点 if/else 腐化，分支点收敛为 6-7 个显式位置
- **零回归双证据纪律**：期望值逐字未动 diff 白名单审查 + UAT 基线逐脚本计数对齐，使「shared 逐字节不变」始终可证伪
- **收口闸六段式**：每 phase 末一次跑齐（静态面/-race/darwin 闸/dist/UAT 矩阵/diff 审查），五个 phase 全部一次全绿
- **三维归类执行机械**：newTestServer 四形态小族单一装配点 + t.Run 双跑，CI 零新 leg（ci.yml 零 diff）天然覆盖双模式
- **协议层 UAT 先例复用**：phaseNN.mjs 同构骨架（phase11→12→13→14 演进），RawStallClient 停读夹具跨 phase 复用

### What Was Inefficient

- Phase 14 P09（19h50min）与 P12（9h20min）两个超长 plan——海森 bug 诊断（10 轮实跑 + 23 探针）与收口闸长跑不可拆分
- CI flake 多轮修复：G-11-2（macOS EPERM 过渡态）、dwell NoKick 三连修（CI 慢 runner 时序）、TestMaxClients503 隔离复跑 flake（遗留至收口，未修）
- 收口 SUMMARY 逐条成就进入 MILESTONES.md 时为 plan 级噪音（38 条），需手工压缩为 phase 级

### Patterns Established

- phase 基点 = 首提交父提交（branching_strategy=none 下 merge-base 退化的等价形态，11-06/12-05/13-08/14-12 先例）
- 需求勾选归 phase 末收口 plan（跨 plan 共享需求 ID，11-01/12-01/13-01 先例链）
- 公开契约变更走 one-way 确认门（用户派发 option-a：stop-timeout 双默认值、Welcome.session 恒序列化、maxClients 默认不动）
- 收口闸六段式 + diff 白名单审查（35 文件零外改动形态）为 phase 验收标准载具
- 终端类测试断言容忍 PS1/bracketed-paste 交错形态（(?:\$ )? 容忍正则）
- herdr 观测通道钉定：api snapshot area 翻转链 + pane read + 版本钉定 0.8.100

### Key Lessons

1. **双模式改造最大风险是破坏既有不变量而不自知**——逐字 diff 审查 + 基线计数对齐 + mode= 子测分叉表（shared 列期望值逐字未动）三层防线缺一不可
2. **CI flake 多为环境时序而非产品缺陷**（PS1 交错、macOS EPERM 过渡态、慢 runner 冷启动）——容忍形态修正优于放大等待阈值；诊断时先做基线 worktree 复现定位归属
3. **per-connection spawn 模型下 stop-timeout 默认 0 = 泄漏防线默认关闭**——产品语义相反的场景需要双默认值（13-01 one-way 门先例）
4. **负载账面推算必须实测回填**（14-07 D-11：32 会话实测 ≈ 账面 9%——账面高估 10 倍，默认值决策应以实测为准）
5. **载具级海森 bug 的裁决方法**：多形态 minrepro 双件套 + 服务端无罪反证链 + 架构绕道（裸 WS 驱动端）保住断言面

### Cost Observations

- Model mix: unknown（未统计）
- Sessions: 未逐 session 统计；plan 级中位时长 ~25min，双长尾 19h50min / 9h20min
- Notable: 零新依赖全程保持（供应链红线零 diff）；全量 -race 从 ~98.5s 增至 ~2m38s（双模式双跑累积，CI 时长预算已知悉）

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Timeline | Phases | Plans | Key Change |
|-----------|----------|--------|-------|------------|
| v1.0 | 2026-08-13 → 08-31（19 天） | 9 | 70 | 行走骨架 → 发布链；零回归纪律建立 |
| v1.1 | 2026-09-02 → 09-08（7 天） | 5 | 38 | 接缝先行 + 三维归类 + 收口闸六段式标准化；UAT 矩阵 runner 形式化 |

### Cumulative Quality

| Milestone | Go LOC | 零新依赖 | 全量 -race | UAT 脚本 |
|-----------|--------|---------|-----------|----------|
| v1.0 | ~21k | ✓ | ~98.5s 五包 | 16 脚本 |
| v1.1 | 27,844 | ✓（保持） | ~2m38s 五包（216 mode= 子测） | 17 项矩阵（run-all.mjs） |

### Top Lessons (Verified Across Milestones)

1. 期望值逐字未动 diff 审查是回归面最廉价的高判别力证据（v1.0 建立、v1.1 双模式语境下升级为白名单终审）
2. 协议层 UAT 脚本（web/uat/phaseNN.mjs 零依赖模式）跨 phase 复用与同构演进，双证据链（Go 测试 × UAT）持续成立
3. 公开契约变更必须 one-way 确认门 + 文档双源落地（flag/TOML 同步）
