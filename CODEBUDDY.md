# wesh 项目记忆

## 开发拓扑（2026-08-24 修订）

双机架构：

- **Windows 11 工作站（GUI）**：浏览器自动化宿主。Playwright 可用（Chromium 缓存就绪），负责浏览器半侧的 UAT 实测。本机无 Go/WSL——**wesh 不能在本机构建运行**（internal/pty 仅 linux/darwin 构建标签；Windows/ConPTY 支持属 PROJECT.md 既定 Out of Scope）
- **Linux 开发机（headless）**：构建/运行 wesh 服务端；Go 单测、协议层/DOM 层 UAT 脚本均在此侧执行。**该侧维持 2026-08-19 原硬约束**：无 GUI/浏览器，禁装 playwright 及 X11 依赖

历史：本文件 2026-08-19 版以「本机=Linux 开发机」单机视角写成「永久 headless、禁 playwright」；2026-08-24 用户确认 Windows 工作站侧具备图形界面并授权 Playwright 自动化（当日双机全链实证通过）。

## 测试策略（分层）

1. **协议层**：`web/uat/phaseNN.mjs` 模式——Node 原生 WebSocket/fetch 零依赖脚本，spawn 真实二进制断言（先例：phase02/03/04.mjs）
2. **终端核心逻辑**：`@xterm/headless`（纯 JS 无原生依赖）——buffer/宽度/光标断言，与浏览器同代码路径（注意需 `allowProposedApi: true`）
3. **前端 DOM 逻辑**：jsdom + mock（navigator.clipboard 等）——门控/防抖/条件注册等逻辑面
4. **浏览器实测层**：`web/uat/pw/`——Windows 侧 Playwright 驱动真实 Chromium，经本机 TCP 转发器（kill/restore 模拟断网 RST 语义）连接 Linux 侧 wesh 服务端；面板文案/倒计时/标题前缀/清屏重绘等观感全链断言（先例：phase06-pw.mjs 六项 46/46）
5. **平台原生行为显式豁免**：真实 OS 网卡栈断网时序、浏览器权限弹窗、原生 confirm 框、OS 真实 IME 栈、像素视觉（截图留档人工复核）——不列为阻塞项，在 UAT 中以 `skipped` + reason 记录并风险接受

## 禁止事项

- Linux 开发机侧：不要 `apt-get install` 任何 GUI/X11/浏览器相关库；不要在该侧安装 playwright/浏览器；不要在该侧启动 wesh 实例等待人工浏览器访问（无人能访问）
- Windows 工作站侧：不要尝试在本机构建/运行 wesh 服务端（无 Windows PTY 支持，既定 Out of Scope）；不要操作真实网卡/系统网络配置来模拟断网（一律用 TCP 转发器 kill/restore）
- Playwright 浏览器安装与运行仅在 Windows 工作站侧进行

## 工具偏好（继承全局，此处重申项目级）

- pnpm 而非 npm
- 构建命令带 `time` 前缀
