# Roadmap: wesh

## Milestones

- ✅ **v1.0** — Phases 1-9（shipped 2026-08-31，v1.0.0 四平台发布上架，44/44 需求收口）
- ✅ **v1.1 per-client 会话模式** — Phases 10-14（shipped 2026-09-08，15/15 需求收口）

## Phases

<details>
<summary>✅ v1.0（Phases 1-9）— SHIPPED 2026-08-31</summary>

- [x] Phase 1: 行走骨架（核心 PTY 管道）(5/5 plans) — completed 2026-08-14
- [x] Phase 2: 协议基线 (6/6 plans) — completed 2026-08-15
- [x] Phase 3: 认证与传输安全 (7/7 plans) — completed 2026-08-18
- [x] Phase 4: 前端体验 (6/6 plans) — completed 2026-08-19
- [x] Phase 5: 多客户端共享 (13/13 plans) — completed 2026-08-22
- [x] Phase 6: 会话生命周期与重连 (7/7 plans) — completed 2026-08-24
- [x] Phase 7: 部署与配置 (10/10 plans) — completed 2026-08-27
- [x] Phase 8: 可观测性 (6/6 plans) — completed 2026-08-28
- [x] Phase 9: 发布与打磨 (10/10 plans) — completed 2026-08-31

阶段详情见归档 [milestones/v1.0-phases/](milestones/v1.0-phases/)（phase 目录与 SUMMARY 全套）。

</details>

<details>
<summary>✅ v1.1 per-client 会话模式（Phases 10-14）— SHIPPED 2026-09-08</summary>

**Milestone Goal**: wesh 支持 ttyd 式 per-connection spawn——每个 WebSocket 客户端独立 PTY 子进程，使 herdr 等自带多客户端仲裁（is_foreground + per-client area 渲染）的应用在 wesh 下恢复正确行为；shared 共享模式保持默认、零回归。架构形态「装配期一次分岔，运行期零分岔」，不抽象 session 接口，两模式共享面 ≥90%。

- [x] Phase 10: 模式装配与接缝 (5/5 plans) — completed 2026-09-03
- [x] Phase 11: per-client 生命周期主干 (7/7 plans) — completed 2026-09-04
- [x] Phase 12: per-client 交互与背压语义 (5/5 plans) — completed 2026-09-04
- [x] Phase 13: 资源防线与终结语义 (8/8 plans) — completed 2026-09-05
- [x] Phase 14: 双模式验证矩阵、标定与 herdr UAT (13/13 plans) — completed 2026-09-08

完整阶段详情见 [milestones/v1.1-ROADMAP.md](milestones/v1.1-ROADMAP.md)。

</details>

---

*Archives: .planning/milestones/ — v1.0-ROADMAP.md（含 v1.0-phases/）、v1.1-ROADMAP.md（含 v1.1-phases/）、v1.1-REQUIREMENTS.md*
