# Phase 6 — API Coverage Declaration

No external API integration: Phase 6 全部能力落在 first-party Go/TS 代码（proto 帧契约 / server 生命周期 / pty 信号 / CLI flag / 前端重连状态机），零新依赖、零外部服务接入（06-RESEARCH §Package Legitimacy Audit：无 `npm install` / `go get` 步骤）。api-coverage 检测器的唯一命中为 06-CONTEXT.md D-04「浏览器 WS API 不暴露 ping/pong」一句中 "API" 字样（浏览器内建 Web API 的语义引用，非外部 API 集成），属误报。
