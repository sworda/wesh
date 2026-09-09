---
phase: 04
slug: frontend
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-19
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> 登记源自 6 份 PLAN 的 `<threat_model>` 块（04-01 ~ 04-06）；验证深度 ASVS L1（grep 级证据）。

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CLI flag 输入 → prefs blob | `--client-option` 值是半可信输入（systemd/脚本拼装），未经白名单+JSON 校验可注入任意 xterm option | 运维侧配置 → 客户端运行时 |
| 服务端 → 浏览器客户端（Welcome prefs） | prefs blob 经 Welcome 帧驱动 term.options 运行时行为；危险键（allowProposedApi 类）可扩大攻击面 | 服务端配置 → 浏览器 xterm 行为 |
| 远程终端输出 → 浏览器 UI 面 | PTY 对面程序可发任意 OSC 0/2/8 序列驱动标题与链接——远程内容是不可信输入 | 远程 PTY → 标签页标题/可点击链接 |
| npm registry → 构建链 | 三个新 addon 与传递依赖 js-base64 进入单 HTML 产物，供应链完整性即运行时完整性 | npm registry → dist/index.html |
| 浏览器剪贴板 ↔ 终端会话 | 系统剪贴板是跨应用共享面——读取必须绑定显式用户手势，写入失败不得干扰终端主流程 | 浏览器 OS 剪贴板 ↔ 终端 |
| 页面生命周期 ↔ 会话生命周期 | 单次语义下关页=会话终结；beforeunload 是唯一误关防线，但不得成为会话结束后的拦路石 | 浏览器页面 ↔ PTY 会话 |
| 测试脚本 → CI/控制台输出 | UAT 输出进 CI 日志面——值内容不得经 detail 泄漏（红线延伸到测试面） | UAT 脚本 → CI 日志 |
| 测试实例 ↔ 用户环境 | spawn 的 wesh 实例必须隔离于用户服务（临时端口 + loopback + 临时路径二进制） | UAT 实例 ↔ 用户部署 |
| URL query → 终端配置 | query 是用户侧输入——非法值不得让终端不可用，白名单外键不得生效 | URL query → xterm 配置 |
| 文档 → 用户部署行为 | README 是用户安全配置的唯一指引面——弱化剪贴板/OSC52 表述会直接把用户导向风险部署 | README → 用户部署决策 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01 | Elevation of Privilege | `--client-option` → prefs 通道注入白名单外 option（allowProposedApi 等） | high | mitigate | `proto.ValidClientOptionKey` 恰 10 键白名单 parse 期 fail-fast（internal/proto/proto.go:128，cmd/wesh/main.go:107）；TestValidClientOptionKey 表驱动回归锁（internal/proto/proto_test.go:113） | closed |
| T-04-02 | Tampering | osc52 经 client-option 等用户侧通道绕过服务端意图开启剪贴板写 | high | mitigate | osc52 结构性排除出白名单（D-12 安全不对称），仅 `--osc52` flag 服务端开启（cmd/wesh/main.go:121, 164-176 aggregateClientPrefs 注释明示"osc52 不在白名单"） | closed |
| T-04-03 | Information Disclosure | 启动报错文案回显 prefs 值内容（systemd 日志面） | low | mitigate | 错误串只含 key 名与错误类别；main_test 红线断言 | closed |
| T-04-04 | Spoofing | OSC 0/2 标题注入伪装主机名/路径（钓鱼终端上下文） | high | mitigate | `sanitizeTitle` 剥离 C0/DEL/C1/bidi/ZWSP/BOM + 128 code point 截断（web/src/lib/title.ts）；title.test.ts 含 RLO 视觉反转钓鱼回归锁 | closed |
| T-04-05 | Spoofing | OSC 8 钓鱼链接（显示 github.com 点开 evil.com） | high | mitigate | 双通道 hover tooltip 展示完整真实 URL 不截断（main.ts:139-145 xterm-hover）；`allowNonHttpProtocols` 保持默认 false（javascript:/file: 结构性忽略，main.ts:125）；`linkHandler` 显式接管（main.ts:127） | closed |
| T-04-06 | Tampering | reverse tabnapping（window.opener 劫持新标签页） | medium | mitigate | `linkHandler.activate` 显式 `w.opener = null`（main.ts:131，等价 rel=noopener） | closed |
| T-04-07 | DoS | 超长/控制字符标题使标签页 UI 异常 | low | mitigate | 128 code point 截断 + 空串回退 `'wesh'`（title.ts） | closed |
| T-04-08 | Information Disclosure | 无手势/后台读剪贴板（隐私越界） | medium | mitigate | `readText` 仅 Ctrl+Shift+V 触发（main.ts:302）+ ro 永不读（isRO 门）+ `clipboardOK` 存在性门控（main.ts:276） | closed |
| T-04-09 | DoS | 剪贴板权限拒绝/不可用打断终端主流程 | medium | mitigate | 写读失败一律 `.catch → console.warn` 静默（main.ts:302）；非安全上下文整体降级不抛 TypeError | closed |
| T-04-10 | Tampering | 会话终结后 beforeunload 残留拦截关页（UI 劫持感） | low | mitigate | onclose 任意路径首行移除 listener；重试路径先移除再由新 WELCOME 按开关重注册 | closed |
| T-04-11 | Information Disclosure | UAT detail 打印 prefs/theme 值内容进 CI 日志 | low | mitigate | phase04.mjs 全部 10 个 check 的 detail 只含键形状/布尔/退出码，值内容零进输出（04-04-SUMMARY 红线延伸锁定） | closed |
| T-04-12 | Tampering | 测试实例占用固定端口/干扰用户服务 | low | mitigate | `--port 0` 随机端口 + `--bind 127.0.0.1` + `/tmp/wesh-uat` 临时路径 + SIGKILL 收口（phase04.mjs 沿用 phase03.mjs 形态） | closed |
| T-04-13 | Elevation of Privilege | query/prefs 注入白名单外 option | high | mitigate | 前端 `XTERM_PREF_KEYS`/`BEHAVIOR_PREF_KEYS` 与 Go 侧语义同源（web/src/lib/prefs.ts:6,18）；osc52 结构性排除出 query（prefs.ts:22,34）+ prefs.test.ts:30 专项锁 | closed |
| T-04-14 | Information Disclosure | OSC52 远程读剪贴板（Warp CVE-2025-48725 形态） | high | mitigate | write-only provider——`readText` 恒 resolve `''`（main.ts:454-462）；默认关 + 仅 `--osc52` 服务端开启 + `clipboardOK` 门 | closed |
| T-04-15 | Tampering | OSC52 读查询触发 unhandled rejection 噪音 | low | mitigate | `readText` resolve `''` 替代 reject（RESEARCH §Pitfall 4：核心异步 OSC 链 rethrow rejected promise） | closed |
| T-04-16 | DoS | 非法 query 值使终端启动失败 | medium | mitigate | 逐键 `JSON.parse` try/catch 静默忽略 + `console.warn`（prefs.ts:36-39，main.ts:61,72）+ prefs.test 容错行 | closed |
| T-04-17 | Information Disclosure | 文档省略剪贴板 HTTPS/localhost 明示，用户在明文 HTTP 下误判 | medium | mitigate | README.md:90 明示"需 HTTPS 或 localhost 访问——明文 HTTP 非 localhost 下浏览器不暴露剪贴板 API" | closed |
| T-04-18 | Tampering | 文档暗示 osc52 用户侧开启路径，绕过 D-12 服务端意图 | medium | mitigate | README.md:38 明示"只能经本 flag 开启——URL query 与 `--client-option` 均不可设置"；README.md:90 重申"`--osc52` 开启后只写不读" | closed |
| T-04-SC | Tampering | npm/pip/cargo installs 供应链 | high | accept | 04-01/03/04/05/06 五 plan 零新依赖；04-02 三包精确钉版 + `pnpm-workspace.yaml` overrides 钉 `js-base64: 3.9.2`（避开 2026-08-17 发布仅 1 天的 3.9.3）+ RESEARCH §Package Legitimacy Audit 全 OK + postinstall 空已核实 | closed (accepted) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on=high count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-SC (04-01) | 纯 Go 零新依赖（encoding/json 标准库）；前端三包审计与钉版划归 04-02 | plan-author | 2026-08-19 |
| AR-04-02 | T-04-SC (04-03) | 零新依赖（Clipboard API 浏览器内建；addon-clipboard 的加载接线在 04-05，钉版已于 04-02 完成） | plan-author | 2026-08-19 |
| AR-04-03 | T-04-SC (04-04) | 脚本零依赖（Node 原生 WebSocket/fetch）——无安装面 | plan-author | 2026-08-19 |
| AR-04-04 | T-04-SC (04-05) | 钉版与 js-base64 override 已于 04-02 完成；本 plan 零新安装 | plan-author | 2026-08-19 |
| AR-04-05 | T-04-SC (04-06) | 零新依赖（文档 plan；裸 clone 验证的 pnpm install 走 lockfile 既定链） | plan-author | 2026-08-19 |

*注：04-02 的 T-04-SC 为 mitigate（钉版 + 审计 + override）已落地，非 accept。*

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-19 | 19 | 19 | 0 | gsd-security-auditor (L1 grep-depth) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-19
