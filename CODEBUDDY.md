# wesh 项目记忆

## 环境硬约束（永久）

本开发机为纯 headless 环境，**永不具备**：

- 图形界面 / 浏览器（无法访问 127.0.0.1 上的 web 服务做人工验证）
- Playwright 及其系统依赖（libatk-bridge/libgbm/libatspi 缺失，**明确禁止安装**——2026-08-19 用户裁决，已卸载全部 playwright 相关内容含 Python 包与 ~/.cache/ms-playwright）

## 测试策略（受此约束驱动）

所有验证必须在本机纯 Node 环境完成，分层：

1. **协议层**：`web/uat/phaseNN.mjs` 模式——Node 原生 WebSocket/fetch 零依赖脚本，spawn 真实二进制断言（先例：phase02/03/04.mjs）
2. **终端核心逻辑**：`@xterm/headless`（纯 JS 无原生依赖）——buffer/宽度/光标断言，与浏览器同代码路径（先例：phase 4 T1 宽度断言，注意需 `allowProposedApi: true`）
3. **前端 DOM 逻辑**：jsdom + mock（navigator.clipboard 等）——门控/防抖/条件注册等逻辑面
4. **平台原生行为显式豁免**：浏览器权限弹窗、原生 confirm 框、OS 真实 IME 栈、像素视觉——任何自动化（含 playwright）均不可测，不列为阻塞项，在 UAT 中以 `skipped` + reason 记录并风险接受

## 禁止事项

- 不要尝试 `apt-get install` 任何 GUI/X11/浏览器相关库（无 sudo 且用户已否决）
- 不要建议"装个浏览器/ playwright 再测"——此路永久封闭
- 不要在本机启动 wesh 实例等待人工浏览器访问（无人能访问）

## 工具偏好（继承全局，此处重申项目级）

- pnpm 而非 npm
- 构建命令带 `time` 前缀
