# wesh UAT 浏览器实测层（Playwright）

驱动真实 Chromium 对 wesh 服务端做全链观感断言——面板文案/倒计时/标题前缀/清屏重绘等
协议层与 jsdom 层覆盖不到的用户可观测面。对应测试策略第 4 层（见根 CODEBUDDY.md）。

## 双机运行模型

wesh 只能在 Linux/macOS 构建运行（internal/pty 构建标签）；浏览器自动化在具备 GUI 的
Windows 工作站侧执行。本包运行在**浏览器侧**，wesh 服务端经 SSH 在 **Linux 侧**管理：

```
Windows 工作站（本包 + Chromium）
  └─ 本机 TCP 转发器（127.0.0.1:PORT_BASE+n，kill/restore 模拟断网）
       └─ SSH 可达的 Linux 机器（wesh 服务端，:7681）
```

断网模拟原理：转发器 killNet() 毁掉在飞连接（双端 RST）→ 浏览器 WS 合成 1006 →
触发客户端重连状态机；restore() 恢复转发。恢复快路径经合成 `online` 事件触发（与真实
断网恢复时 OS 派发的 online 事件命中同一监听器）。**禁止**操作真实网卡/系统网络配置。

## 前置

- 本机：Node ≥ 22、`pnpm -C web/uat/pw install --ignore-workspace`（独立包不挂 web/ workspace）、首次运行需 `npx playwright install chromium`
- Linux 侧：wesh 仓库已构建（`go build -o /tmp/wesh-uat/wesh ./cmd/wesh`），SSH BatchMode 可达
- 环境变量：

| 变量 | 默认 | 说明 |
|------|------|------|
| `WESH_UAT_SSH` | （必填） | SSH 目标，形态 `user@host` |
| `WESH_UAT_SSH_PORT` | `22` | SSH 端口 |
| `WESH_UAT_TARGET_HOST` | SSH host | 转发器目标主机 |
| `WESH_UAT_TARGET_PORT` | `7681` | 转发器目标端口 |
| `WESH_UAT_CRED` | `user:pass` | 服务端 Basic 凭据（仅测试环境一次性凭据） |
| `WESH_UAT_PORT_BASE` | `17681` | 本机转发器起始端口（各测试依次 +n） |
| `WESH_UAT_REMOTE_DIR` | `/tmp/wesh-uat` | 远端工作目录（run.sh/exit.status/server.log） |

## 运行

```bash
# 全量（约 2 分钟，T1 含 30s 退避观测窗）
WESH_UAT_SSH=user@host WESH_UAT_SSH_PORT=36000 pnpm -C web/uat/pw uat:06

# 单项调试
node web/uat/pw/phase06-pw.mjs t1
```

产物：`results.json`（结构化结果）、`screenshots/`（观感留档，gitignore）。

## 载具登记

| 载具 | 形态 | 运行 |
|------|------|------|
| `phase06-pw.mjs` | 断网重连/观感全链（经本机 TCP 转发器 kill/restore） | `pnpm -C web/uat/pw uat:06` |
| `phase07-a2-pw.mjs` + `phase07-a2-ctl.sh` | 真 nginx 反代子路径双机全链（G-07-2 实证锚点） | `node web/uat/pw/phase07-a2-pw.mjs` |
| `phase09-caddy-pw.mjs` + `phase09-caddy-ctl.sh` | Caddy 反代双机全链（09-08 D-15：Host 默认透传/WS upgrade 内建/无默认 idle 超时实证面；LAN :10014 → loopback :17682） | `node web/uat/pw/phase09-caddy-pw.mjs` |

## 新 phase 复用

新建 `phaseNN-pw.mjs`，复用 `lib/` 四件套：

- `forwarder.mjs` — Forwarder(listenPort, targetHost, targetPort)：killNet()/restore()
- `server.mjs` — ssh()/startWesh(argsTail)/stopWesh()/ensureNormal()/exitStatus()（退出码捕获）
- `browser.mjs` — launch()/authedContext()/openSession()/runCmd()/waitTermText()/
  getShellPid()/panel()/waitPanel()/waitPanelHidden()/fireOnline()/waitTitle()
- `check.mjs` — Check 断言收集（PASS/FAIL 行 + summary）

断言纪律沿用既有 UAT 红线：凭据/token 值永不进 detail/控制台输出（只打状态码/布尔/
形状/退出码/文案常量）。
