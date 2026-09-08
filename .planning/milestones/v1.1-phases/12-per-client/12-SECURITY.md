---
phase: 12
slug: per-client
status: verified
threats_open: 0
asvs_level: 1
created: 2026-09-05
---

# Phase 12 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| server→client (Welcome 帧) | session 键为服务端→客户端的可信通道新增键；前端解析为不可信输入处理（值域白名单校验，非法值降级 shared + warn） | session 枚举字符串（公开值） |
| client→server (RESIZE 帧) | ro 客户端在 per-client 新获 RESIZE 生效面——输入经 DecodeResize 钳制 [1,1000] + 每会话 50ms 防抖两防线 | 尺寸整数（钳制后） |
| client→server (消费速率) | 慢/stall 客户端是背压语义的不可信输入面——资源占用上界由 dwell + maxClients 双闸收口 | PTY 输出流（会话自有） |
| 测试脚本→真实二进制 | UAT 以真实 spawn 二进制 + 真实 WS 握手为证据面；红线 = 敏感值（token/pid）永不落盘/日志 | 断言输出（脱敏） |
| 规划文档↔代码事实 | 勾选/回指声明必须与代码事实一致——diff 审查为唯一可信证据面 | 文档声明 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-12-01 | Tampering | main.ts WELCOME 分支 session 解析 | low | mitigate | 值域白名单（仅两枚举值赋值，其余 warn 保持 shared，main.ts:696-700）——非法值不触发 reset；phase12-dom D3a-c 断言锁定 | closed |
| T-12-02 | Information Disclosure | WelcomePayload.session | low | accept | 值为公开枚举（与 CLI flag 同词），零敏感面；additive 键不含任何会话/身份信息 | closed |
| T-12-03 | DoS | terminal.reset() 调用点 | low | accept | reset 为本地 UI 操作，频率上界 = Welcome 帧率（每连接一次）；无放大面 | closed |
| T-12-04 | DoS | RESIZE 直通路径（ro 放行面） | low | mitigate | 钳制 [1,1000] 在 Decode 层既有（proto.go:219-225 ClampDim）；50ms 每会话防抖合并风暴；资源上界 = maxClients 硬顶 | closed |
| T-12-05 | Tampering | 锁序（resizeMu/fdMu/hubMu） | medium | mitigate | resizeMu 叶锁不嵌套 + 直通仅 fdMu 不持 hubMu；回调函数体 hubMu 零命中源码断言 + 全量 -race | closed |
| T-12-06 | Elevation of Privilege | ro INPUT 门 | high | mitigate | ro INPUT 丢弃闸零改动（server.go:1178 `cl == nil \|\| cl.mode.Load() == proto.ModeRO → continue`）；TestPerClientROInputDropped 行为断言锁定；权限不得由客户端请求获得 | closed |
| T-12-07 | DoS | 阻塞持帧（stall 驻留面） | medium | mitigate | dwell 10s 到期 1013 踢出 + 每 stall 端资源确定（1 outbox ≤512KiB + 1 闭包）+ maxClients 硬顶——最坏驻留 32×512KiB 有界 | closed |
| T-12-08 | Tampering | dwell 计时器（陈旧回调误踢） | medium | mitigate | AfterFunc 三件套（身份比对 pc.dwellTimer != t 不动作 + cl.done 早退 + removeLocked 幂等兜底，perclient.go:337/367） | closed |
| T-12-09 | DoS | 恢复信号通道（信号风暴） | low | mitigate | cap-1 信号量 select default 非阻塞发送（clients.go:256）——drain 频率上界 = writer 吞吐；单消费者无广播放大面 | closed |
| T-12-10 | Information Disclosure | 1013 踢出事件 | low | accept | detach reason=kick code=1013 既有 schema 零新字段（审计窗口期 Phase 11→13 已明示接受） | closed |
| T-12-11 | Information Disclosure | phase12.mjs 测试输出 | medium | mitigate | assertOutputClean() 运行时自净逐字保留 + detail 只打状态码/布尔/形状/退出码/文案常量 | closed |
| T-12-12 | DoS | UAT 资源残留 | low | mitigate | startWesh SIGKILL 收口 + 场景间 300ms + pollESRCH 复核——泄漏子进程零残留 | closed |
| T-12-13 | Repudiation | 需求勾选（无证据勾选面） | medium | mitigate | 勾选前六段式全绿 + 三证据链（Go 测/UAT 双脚本）在 SUMMARY 落笔；diff 白名单审查防断言放宽换绿 | closed |
| T-12-SC | Tampering | 依赖面（pnpm build / go.mod） | high | mitigate | 零新依赖红线——git diff e8b39c0..HEAD 对 go.mod/go.sum/pnpm-lock.yaml/uat package.json 零 diff 实证；供应链 legitimacy 门无安装任务不触发 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-12-01 | T-12-02 | session 值为公开枚举（"shared"/"per-client"，与 CLI flag 同词），additive 键零敏感面——信息泄露面不存在 | user（ship 裁决） | 2026-09-05 |
| AR-12-02 | T-12-03 | terminal.reset() 为本地 UI 操作，频率上界 = Welcome 帧率（每连接一次），无放大面——DoS 面不存在 | user（ship 裁决） | 2026-09-05 |
| AR-12-03 | T-12-10 | 1013 踢出事件沿用既有 detach schema（reason=kick code=1013），零新字段——审计窗口期 Phase 11→13 计划内接受 | user（ship 裁决） | 2026-09-05 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-05 | 14 | 14 | 0 | CodeBuddy (gsd-secure-phase, ASVS L1 grep-depth + VERIFICATION.md 2026-09-04 独立复跑证据) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-05
