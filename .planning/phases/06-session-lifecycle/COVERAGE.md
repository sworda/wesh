# Phase 6 — API Coverage Declaration

No external API integration: 全部能力为 first-party Go/TS 代码，零新依赖、零外部服务接入；检测器唯一命中系 06-CONTEXT.md D-04 对浏览器内建 Web API 的语义引用，属误报。

详：proto 帧契约 / server 生命周期 / pty 信号 / CLI flag / 前端重连状态机均由本仓实现；06-RESEARCH §Package Legitimacy Audit 记录无 `npm install` / `go get` 步骤；命中句为 D-04「浏览器 WS API 不暴露 ping/pong」中的 "API" 字样。
