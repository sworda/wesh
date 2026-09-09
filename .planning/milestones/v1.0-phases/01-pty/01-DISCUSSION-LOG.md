# Phase 1: 行走骨架（核心 PTY 管道）- Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-13
**Phase:** 01-pty
**Areas discussed:** CLI 形态与项目落盘, 监听默认值与启动行为, Phase 1 临时多客户端与生命周期, attach 前输出处理

---

## CLI 形态与项目落盘

### Q1: 项目目录与 Go module path

| Option | Description | Selected |
|--------|-------------|----------|
| 重命名目录为 wesh + module github.com/sworda/wesh | 名字对齐，后期进 GitHub release/homebrew 不重名 | |
| 目录保留 stow，go module 用 github.com/sworda/wesh | 少一次 mv，但目录名与项目名不一致 | |
| 不上 GitHub，module 用本地路径 | 仅本地，不发布 | |

**User's choice:** 项目名 wesh，目录重命名为 wesh，**Go module path = `github.com/sworda/wesh`**（用户主动提供仓库地址 `git@github.com:sworda/wesh.git`，非 GitHub）
**Notes:** 用户纠正了 module path 的 host——不是 github.com 而是公司内 github.com。

### Q2: CLI 形态

| Option | Description | Selected |
|--------|-------------|----------|
| `wesh [flags] -- <cmd> [args...]` | 与 ssh/docker exec/kubectl exec 一致，-- 后原样传递 | ✓ |
| `wesh [flags] <cmd> [args...]` | 少打两个字符，但有歧义 | |
| 两种都接受 | 表现友好但解析复杂度上升 | |

**User's choice:** `wesh [flags] -- <cmd> [args...]`

### Q3: 无命令默认行为

| Option | Description | Selected |
|--------|-------------|----------|
| 报错并要求提供命令 | 失败快，不会意多启个 shell | ✓ |
| 默认启动 $SHELL | 拿起 SHELL 环境变量 | |
| 打印 help 后退出 | 介于两者之间 | |

**User's choice:** 报错并要求提供命令

### Q4: Phase 1 flag 集合

| Option | Description | Selected |
|--------|-------------|----------|
| 最简 4 个 flag（--port/--bind/--help/--version） | 骨架阶段保持最简 | ✓ |
| 骨架阶段同时接受后期 flag | 提前加 --read-only/--once/--title 等 | |
| 你决定 | Claude 裁量 | |

**User's choice:** 最简 4 个 flag

---

## 监听默认值与启动行为

### Q1: 默认 bind 地址

| Option | Description | Selected |
|--------|-------------|----------|
| 默认 127.0.0.1，--bind 可覆盖 | Phase 1 无认证，安全默认 | |
| 默认 0.0.0.0 | 起服即全机可访问，与 ttyd 默认一致 | ✓ |
| 默认 127.0.0.1 且端口随机 | 避免冲突 | |

**User's choice:** 默认 0.0.0.0
**Notes:** 用户明确选择 LAN 可达，接受了 Phase 1 无认证期的暴露面。CONTEXT.md 与 README 需显式标注"Phase 1 仅在可信网络使用"。

### Q2: 默认端口

| Option | Description | Selected |
|--------|-------------|----------|
| 默认 7681（ttyd 同款） | 运维记忆友好 | ✓ |
| 默认 0（随机并打印） | 避免占用冲突 | |
| 默认 7682 或其他 | 与 ttyd 不重叠 | |

**User's choice:** 默认 7681

### Q3: 启动后行为

| Option | Description | Selected |
|--------|-------------|----------|
| 仅打 URL，不自动开浏览器 | 与 Unix 工具传统一致 | ✓ |
| 提供 --open flag 可选自动打开 | OPS-11 提前到 Phase 1 | |
| 默认自动打开浏览器 | 服务器场景会报错 | |

**User's choice:** 仅打 URL

### Q4: 启动打印形式

| Option | Description | Selected |
|--------|-------------|----------|
| 单行：`listening on http://host:port` | 一行能看懂 | ✓ |
| 多行横幅 | 虚假丰富感 | |
| 你决定 | Claude 裁量 | |

**User's choice:** 单行

---

## Phase 1 临时多客户端与生命周期

### Q1: 第二浏览器 attach

| Option | Description | Selected |
|--------|-------------|----------|
| 第二个连接拒绝（HTTP 409） | 实现最简，避免双客户端交错 | ✓ |
| 后者顶前者（last-wins） | 与 ttyd 实际表现接近 | |
| 都连上（无互斥） | 实现反而最复杂 | |

**User's choice:** HTTP 409 拒绝
**Notes:** Phase 5 才改为多客户端 fan-out。

### Q2: 子进程退出后服务端行为

| Option | Description | Selected |
|--------|-------------|----------|
| 服务端跟随退出 | 与"wesh -- cmd 就是跑一次 cmd"直觉一致 | ✓ |
| 服务端不退出，但 WS 不再接受 | GoTTY 模型下 PTY 不能重生，无意义 | |

**User's choice:** 服务端跟随退出
**Notes:** 退出前给当前客户端发 1000 正常关闭。

### Q3: WS 客户端断开时

| Option | Description | Selected |
|--------|-------------|----------|
| kill 进程组（SIGHUP）后退出 | Phase 1 单次语义 | ✓ |
| 仅打日志，服务端空转 | 骨架阶段代码最简 | |
| 你决定 | Claude 裁量 | |

**User's choice:** SIGHUP 进程组后退出
**Notes:** Phase 6 才引入"断线保持 + 重连接回同一进程"。CONTEXT.md 已标注这是过渡语义。

---

## attach 前输出处理

### Q1: 首客户端 attach 前的 PTY 输出

| Option | Description | Selected |
|--------|-------------|----------|
| 直接丢弃 + drain | 启动后即起 drain goroutine，防 PTY 64KiB 缓冲阻塞 | ✓ |
| 环形缓冲尾部，attach 后重放 | shell 横幅可见，但与 Phase 6 重叠 | |
| 等首客户端 attach 才 spawn | 违背 GoTTY 模型 | |

**User's choice:** 直接丢弃 + drain（研究推荐）

---

## Claude's Discretion

- 项目目录结构（`cmd/wesh/`、`internal/{proto,pty,server,web}`、`web/`）——以 `01-RESEARCH.md` §Recommended Project Structure 为准。
- CI yaml 细节（runner 镜像、node 版本、setup-node 主版本）——executor 按 GitHub Actions 当前实际版本微调。
- 前端脚手架文件具体内容（index.html、tsconfig.json 字段）——以 `pnpm create vite` 实际生成物为准。

## Deferred Ideas

- **OPS-11 启动后自动开浏览器** — Phase 7
- **Phase 6 完整生命周期语义** — `--once` / 无人退出 / 类型化终结帧 / 断线重连保持
- **Phase 2 协议扩展** — 子协议协商、Hello/Welcome、类型化错误帧、ping/pong 保活、三层上限完整版
- **Phase 5 多客户端** — fan-out、ro/rw、慢客户端背压、resize 仲裁、分享链接（推翻 Phase 1 的 409 拒绝）
- **Phase 3 认证与 TLS** — 一次性 ticket、Origin 白名单、TLS 1.2+（收口 Phase 1 默认 0.0.0.0 的暴露面）
- **darwin kqueue 僵尸注册竞态运行时验证** — CI macos-latest leg 承担
