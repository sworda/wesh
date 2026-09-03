# Phase 3: 认证与传输安全 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-17
**Phase:** 3-认证与传输安全
**Areas discussed:** 凭据形态与认证入口, TLS 形态与证书来源, 失败节流与核销语义, Origin 白名单语义

---

## 凭据形态与认证入口

### Q1: 凭据从哪里来、什么形态？

| Option | Description | Selected |
|--------|-------------|----------|
| --credential user:pass 可重复 | ttyd 对齐，运维记忆零成本；多组凭据支持按人撤销；代价是密码出现在 ps/shell 历史 | |
| 环境变量 WESH_CREDENTIAL | systemd EnvironmentFile 600 友好，不进 ps；手动跑不如 flag 顺手 | |
| flag + env 双通道 | flag 优先、env 兜底，两种部署形态都顺 | ✓ |
| 启动随机生成并打印 | 零配置即安全（jupyter token 式）；重启即变，书签/脚本不便 | |

**User's choice:** flag + env 双通道
**Notes:** 常数时间比较对每组凭据轮询。

### Q2: 认证入口 UI 形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 整站 Basic | / 与 /api/attach 均 401 challenge，浏览器原生弹窗，同源 fetch 自动带凭据，前端零新 UI；静态页受保护纵深防御（ttyd 同形态） | ✓ |
| 页内登录表单 | 页面公开，自绘表单存内存手动加 Authorization 头；体验好但多一个表单组件与状态机 | |
| 页面公开 + 仅 /api/attach 401 | 依赖浏览器对 fetch 401 弹窗，Chrome 不弹，跨浏览器不一致，不推荐 | |

**User's choice:** 整站 Basic
**Notes:** 用户明确要求后续问题恢复选项形式（Q1 曾因选项未显示改纯文本）。

### Q3: 无凭据裸跑是否保留？

| Option | Description | Selected |
|--------|-------------|----------|
| 保留裸跑 + 醒目警告 | 现状延续；bind 非 loopback 警告升级 | |
| 认证强制 | 无凭据拒绝启动；破坏 Phase 1/2 既有用法 | |
| loopback 免警告，非 loopback 需显式 --no-auth | 本机调试顺滑；裸奔变显式动作 | ✓ |

**User's choice:** loopback 免警告，非 loopback 需显式 --no-auth
**Notes:** 默认 bind 0.0.0.0，Phase 1 D-05 临时取舍在此收口。

---

## TLS 形态与证书来源

### Q1: TLS 什么时候开、证书从哪来？

| Option | Description | Selected |
|--------|-------------|----------|
| 显式证书才开（ttyd 式） | --tls-cert/--tls-key 成对给出才 ServeTLS，否则明文 + 醒目警告；反代终止 TLS 部署干净 | ✓ |
| 默认自签 | 无证书自动生成内存态自签；浏览器每次点警告，指纹每次重启变，与反代部署冲突 | |
| 三态：显式证书 / --tls-auto 自签 / 明文警告 | 覆盖面最广但 flag 契约多一个 | |

**User's choice:** 显式证书才开（ttyd 式）

### Q2: 配了凭据但没开 TLS 时怎么办？

| Option | Description | Selected |
|--------|-------------|----------|
| loopback 豁免，否则需 --insecure-http | 本机调试顺滑；非 loopback 明文+凭据需显式逃生门，与 --no-auth 同构 | ✓ |
| 一律拒绝 | 红线最硬但本机调试也要先搞证书 | |
| 警告放行 | 最灵活但红线被破，与 ttyd"看起来能用就停了"同路径 | |

**User's choice:** loopback 豁免，否则需 --insecure-http

### Q3: 安全响应头发到哪个程度？

| Option | Description | Selected |
|--------|-------------|----------|
| 最小集 | HSTS（仅 TLS）+ X-Content-Type-Options: nosniff | |
| 推荐加固集 | 最小集 + Referrer-Policy + frame-ancestors/X-Frame-Options + 实用化 CSP | |
| 交 Claude/research 定 | 按 OWASP 基线与单文件全内联现实定稿 | ✓ |

**User's choice:** 交 Claude/research 定

### Q4: TLS 强度的验证形态？

| Option | Description | Selected |
|--------|-------------|----------|
| Go 断言自动化 + testssl.sh 手动 UAT | 协议/cipher 下限 Go 测试回归；testssl.sh 手动清单文档化 | ✓ |
| testssl.sh 进 CI | 覆盖最全但重外部依赖，CI 变慢变脆 | |
| 仅手动 UAT | 无自动化回归防线 | |

**User's choice:** Go 断言自动化 + testssl.sh 手动 UAT

---

## 失败节流与核销语义

### Q1: 失败节流覆盖哪些失败、按什么维度？

| Option | Description | Selected |
|--------|-------------|----------|
| 两处统一 per-IP 退避 | /api/attach 凭据失败与 Hello ticket 核销失败计入同一计数器 | ✓ |
| 仅 /api/attach 凭据失败 | ticket 128bit 空间爆破无意义，防线更薄实现最简 | |
| per-IP + per-username 双维度 | 多用户共享出口 IP 不误伤；个人工具收益低 | |

**User's choice:** 两处统一 per-IP 退避

### Q2: 退避参数与锁定形态？

| Option | Description | Selected |
|--------|-------------|----------|
| 纯指数退避不硬锁 | 1s→2s→…封顶 30s，成功清零；防误锁自己 | |
| 退避 + 临时锁定 | 连续 N 次锁 5 分钟；共享出口 IP 连坐 | |
| 交 research 标定 | 参照 fail2ban 生态标定初始/倍率/封顶/锁定 | ✓ |

**User's choice:** 交 research 标定
**Notes:** 验收锚点 = ROADMAP 准则 2"爆破 100 次触发可观测退避"。

### Q3: ticket 核销失败给客户端什么反馈？

| Option | Description | Selected |
|--------|-------------|----------|
| 统一 auth_failed + 1008 | 过期/非法/重放同口径零 oracle；前端拿机器码自动重取 ticket 静默重试一次 | ✓ |
| 静默 1008 | 攻击面零反馈最硬；正常用户 ticket 过期莫名断开 | |
| 分层反馈 | 过期发 Error、非法/重放静默；区分即 oracle，不推荐 | |

**User's choice:** 统一 auth_failed + 1008

### Q4: ticket 绑权限怎么绑？

| Option | Description | Selected |
|--------|-------------|----------|
| ticket 继承全局模式 | /api/attach 请求体为空，ticket 内部携带 mode = 全局 --writable；Phase 5 加可选 mode 参数即向后兼容 | ✓ |
| 现在就收 mode 参数 | Phase 5 语义未定，提前加参数有返工风险 | |

**User's choice:** ticket 继承全局模式

---

## Origin 白名单语义

### Q1: --origin 的 flag 语法与匹配语义？

| Option | Description | Selected |
|--------|-------------|----------|
| 可重复 + 精确 origin | scheme://host[:port] 规范化后精确比较；无通配 = 无误配面 | ✓ |
| 精确 + 后缀通配 | *.example.com 覆盖子域；通配增加误配与匹配歧义面 | |
| 逗号分隔单 flag | 不符合现有 CLI 风格，值含逗号转义麻烦 | |

**User's choice:** 可重复 + 精确 origin

### Q2: 无 Origin 头的客户端怎么处理？

| Option | Description | Selected |
|--------|-------------|----------|
| 无 Origin 放行 | 非浏览器客户端零摩擦；CSWSH 威胁模型只约束浏览器 | ✓ |
| 无 Origin 也拒 | 最严；e2e/UAT/脚本全要带 Origin 头，安全收益≈零 | |

**User's choice:** 无 Origin 放行

### Q3: Origin 校验罩哪些端点？

| Option | Description | Selected |
|--------|-------------|----------|
| /ws + /api/attach 都查 | 纵深防御成本零；规则统一"有 Origin 必过白名单" | ✓ |
| 仅 /ws 握手 | SEC-04 字面；/api/attach 靠 Basic + 节流双防线 | |

**User's choice:** /ws + /api/attach 都查
**Notes:** 静态页 GET 不查（无副作用）。

---

## Claude's Discretion

- 安全响应头具体集合（Q3 交 research，OWASP 基线 + 单文件内联现实）
- 退避参数初始值/倍率/封顶/锁定（Q2 交 research 标定）
- ticket 存储结构、核销原子性、无认证模式下 /api/attach 行为与前端探测顺序
- /api/attach POST-only/body 上限/响应 JSON 字段名
- Origin 规范化细节、Basic realm、401/403 文案、日志脱敏测试形态、TLS 1.2 cipher 清单

## Deferred Ideas

- ro/rw 分别签发 ticket 与分享链接 — Phase 5（MULTI-05）
- permission_denied Error code — Phase 5（P2 deferred 同批挂账）
- X-Forwarded-For/可信代理与 auth-header 透传（SEC-07）— Phase 7；server.go 注释误写"Phase 3 SEC-07"顺手校正
- 节流失败计数进 metrics / slog 结构化审计 — Phase 8
- ACME 自动证书 — v2（V2-ACME）
- flag 可配性收口进配置文件 — Phase 7（OPS-09）
