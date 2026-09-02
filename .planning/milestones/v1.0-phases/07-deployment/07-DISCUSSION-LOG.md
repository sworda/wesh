# Phase 7: 部署与配置 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-25
**Phase:** 7-deployment
**Areas discussed:** 配置文件（OPS-09）、监听与 base-path（OPS-01/02）、auth-header 透传（SEC-07）、子进程与运行形态（OPS-04/05/11）

---

## 配置文件（OPS-09）

### Q1: 配置文件的路径发现策略

| Option | Description | Selected |
|--------|-------------|----------|
| 仅 --config 显式指定 | 不做任何隐式默认路径搜索；裸 wesh -- bash 行为与今天完全一致，零意外；systemd 场景显式路径最可控 | ✓ |
| --config + 默认路径搜索兜底 | 零配置开箱友好，但隐式加载可能让用户困惑「配置从哪来的」 | |
| 仅默认路径搜索 | 不支持 --config；systemd 多实例场景需要显式路径 | |

**User's choice:** 仅 --config 显式指定

### Q2: 可重复 flag 与配置文件同名列表的合并粒度

| Option | Description | Selected |
|--------|-------------|----------|
| CLI 替换整个列表 | 与 D-01 WESH_CREDENTIAL env 兜底先例一致；CLI 能完整表达最终状态（含移除条目） | ✓ |
| CLI 追加合并 | 两边并集，但 CLI 无法移除配置文件里已存在的条目 | |

**User's choice:** CLI 替换整个列表

### Q3: TOML 结构形状

| Option | Description | Selected |
|--------|-------------|----------|
| 平铺 key = value（键名 = flag 名） | 心智成本零；help 与文档单一事实源；go-toml v2 直解同构类型 | ✓ |
| 分组 sections | 分组语义清晰，但与 flag 名双写漂移风险 | |
| You decide | — | |

**User's choice:** 平铺 key = value（键名 = flag 名）

### Q4: 覆盖面：命令 argv 能否进配置文件

| Option | Description | Selected |
|--------|-------------|----------|
| 命令可入配置，CLI argv 覆盖 | command = ["bash", "-l"] exec 数组；systemd unit 只需 ExecStart=wesh --config /etc/wesh.toml | ✓ |
| 命令仅 CLI，配置文件不含 | 配置文件纯粹是「flag 的另一种写法」，职责边界最清 | |
| You decide | — | |

**User's choice:** 命令可入配置，CLI argv 覆盖
**Notes:** 边界确认为——其余长期运行 flag 全量入配置；--no-auth/--insecure-http 逃生门、--version/--help/--config 本身不入。

### Q5: WESH_CREDENTIAL env 与配置文件的优先级链

| Option | Description | Selected |
|--------|-------------|----------|
| flag > env > 配置文件 > 默认 | 与 D-01 现状完全兼容；env 作为 systemd EnvironmentFile= 600 通道仍优先于配置文件明文 | ✓ |
| flag > 配置文件 > env > 默认 | 配置文件优先于 env——与 D-01 先例有漂移 | |
| You decide | — | |

**User's choice:** flag > env > 配置文件 > 默认

### Q6: 配置文件加载失败与未知键的处理形态

| Option | Description | Selected |
|--------|-------------|----------|
| exit 2 fail-fast + 未知键拒绝 | 严格模式防拼写错误静默失效；与启动校验矩阵同档 | ✓ |
| fail-fast + 未知键仅警告 | 配置文件向前兼容，但拼写错误静默 | |
| You decide | — | |

**User's choice:** exit 2 fail-fast + 未知键拒绝

### Q7: 配置文件含凭据明文时的文件权限检查

| Option | Description | Selected |
|--------|-------------|----------|
| 非 600 警告放行 + 文档明示 | 不阻断——挂载盘/容器 secret 权限语义不可靠 | ✓ |
| 非 600 拒绝启动 | ssh 私钥同款，但挂载盘/容器场景误伤多 | |
| 不检查 | 纯文档建议 | |

**User's choice:** 非 600 警告放行 + 文档明示

---

## 监听与 base-path（OPS-01/02）

### Q1: UNIX socket 监听的 CLI 形态

| Option | Description | Selected |
|--------|-------------|----------|
| 独立 --socket flag，与 --port/--bind 互斥 | 组合冲突进 validateStartup fail-fast；显式零字符串解析歧义 | ✓ |
| --bind 值特判 | ttyd -i 同款单口，但字符串特判引入解析歧义 | |
| You decide | — | |

**User's choice:** 独立 --socket flag，与 --port/--bind 互斥

### Q2: socket 文件属主与权限位的配置形态

| Option | Description | Selected |
|--------|-------------|----------|
| --socket-mode + --socket-owner 两 flag | 默认 0660；user.Lookup 解析；单独给出 = 配置矛盾 fail-fast | ✓ |
| 仅 --socket-mode | 属主继承运行用户；反代跨用户场景要手工 chown | |
| You decide | — | |

**User's choice:** --socket-mode + --socket-owner 两 flag

### Q3: 既有 socket 文件的处理

| Option | Description | Selected |
|--------|-------------|----------|
| 启动时 unlink 旧文件 | IPC 端点残留即垃圾；systemd Restart= 场景零人工干预 | ✓ |
| 存在即报错 | 最保守但 systemd 自动重启会死在旧文件上 | |
| You decide | — | |

**User's choice:** 启动时 unlink 旧文件

### Q4: 前端 URL 构造改造策略

| Option | Description | Selected |
|--------|-------------|----------|
| 前端相对路径 + 尾斜杠 307 规范化 | go:embed 静态伺服零改动、无模板注入面；/wesh → /wesh/ 是配套硬要求 | ✓ |
| 服务端注入 base 到页面 | go:embed 静态伺服变模板替换，引入渲染面与 CSP 复杂度 | |
| You decide | — | |

**User's choice:** 前端相对路径 + 尾斜杠 307 规范化
**Notes:** 现状 main.ts 硬编码 '/api/attach'（L510）、'/ws'（L601）、share 正则 ^/s/([^/]+)/$（L500）三处改造点。

### Q5: unix socket 形态下 validateStartup 的 bind 安全校验矩阵

| Option | Description | Selected |
|--------|-------------|----------|
| unix socket = 本机信任，跳过非 loopback 校验 | 文件系统权限即认证边界（--socket-mode/owner 承担访问控制） | ✓ |
| unix socket 下仍走凭据校验矩阵 | 纵深防御多一层，但与文件权限冗余、运维平添 Basic 弹窗 | |
| You decide | — | |

**User's choice:** unix socket = 本机信任，跳过非 loopback 校验

### Q6: --base-path 值校验规则

| Option | Description | Selected |
|--------|-------------|----------|
| 严格校验：/foo 形态，拒绝尾斜杠与 .. | parse 期规范化+校验（NormalizeOrigin 先例），非法值 exit 2 | ✓ |
| 宽容规范化自动修正 | 用户少踩坑，但输入与生效值不一致（配置漂移隐蔽） | |
| You decide | — | |

**User's choice:** 严格校验：/foo 形态，拒绝尾斜杠与 ..

### Q7: unix socket 形态下启动打印的形态

| Option | Description | Selected |
|--------|-------------|----------|
| unix socket 下打印 unix:// 提示行 | 无 host:port 可拼；明示反代后链接由反代 URL 决定 | ✓ |
| 仍拼 TCP 链接 | 误导——TCP 根本没监听 | |
| You decide | — | |

**User's choice:** unix socket 下打印 unix:// 提示行

---

## auth-header 透传（SEC-07）

### Q0: SEC-07 在共享进程模型下怎么落（架构张力前置说明）

**背景说明（向用户展示）：** SEC-07 原文「可信 HTTP 头注入的用户名作为子进程环境变量」源自 ttyd -H。ttyd 是 per-connection spawn（每连接 fork 独立 shell），env 各写各的；wesh 已锁定 GoTTY 共享进程模型（PTY 随服务端启动、多客户端共享一个 shell），spawn 时无 HTTP 请求、env 是一次性快照——per-client env 注入结构性不成立。

用户追问注入用途后，展示：ttyd -H 的两个用途 = ① shell 内感知身份（per-connection 模型才成立）② 服务端审计归因（与进程模型无关）。

| Option | Description | Selected |
|--------|-------------|----------|
| 只要审计归因（logEvent 记录） | 共享 shell 下「shell 内身份感知」本质不成立；SEC-07 需求文本修订为服务端侧身份记录 | ✓ |
| 日志 + 前端身份显示 | 用户名经 Welcome 帧下发前端显示，占协议字段 | |
| 想要 env 注入的变体 | 如只在 --once 单客户端模式下支持 | |

**User's choice:** 只要审计归因（logEvent 记录）
**Notes:** 这是本 phase 最重要的架构裁决——需求文本与架构模型冲突的正式闭合。

### Q1: 信任模型（防直连伪造头）

| Option | Description | Selected |
|--------|-------------|----------|
| 裸信任 + 暴露面启动警告 | ttyd 同款；bind 非 loopback 且无凭据时 stderr 警告「可被直连伪造」 | ✓ |
| 可信来源 IP 校验 | --trusted-proxy CIDR；防伪最强但多一个 flag 与解析面 | |
| You decide | — | |

**User's choice:** 裸信任 + 暴露面启动警告

### Q2: auth-header 与 Basic 认证的关系

| Option | Description | Selected |
|--------|-------------|----------|
| 正交提取，认证体系不变 | 只做用户名提取进 logEvent，不做任何认证决定 | ✓ |
| 头存在跳过 Basic | SSO 反代后零摩擦，但与裸信任叠加伪造头即绕过认证 | |
| You decide | — | |

**User's choice:** 正交提取，认证体系不变

### Q3: remote_user 值进 logEvent 前的清洗

| Option | Description | Selected |
|--------|-------------|----------|
| 剥离控制字符 + 截断 128 | P4 D-03 标题 sanitize 同款；Phase 8 OPS-08 既定方向先行 | ✓ |
| 仅截断，等 Phase 8 统一处理 | logEvent 是 stderr 单行文本，控制字符注入风险当期就存在 | |
| You decide | — | |

**User's choice:** 剥离控制字符 + 截断 128

### Q4: 头名形态

| Option | Description | Selected |
|--------|-------------|----------|
| --auth-header 可配头名（单个） | 反代生态头名不统一（authelia Remote-User / oauth2-proxy X-Forwarded-User） | ✓ |
| 固定 X-Remote-User | 最简但用户得改反代配置迁就 | |
| 可重复多个头名 | 覆盖最全但反代串联语义复杂 | |
| You decide | — | |

**User's choice:** --auth-header 可配头名（单个）

### Q5: X-Forwarded-For 是否同批做

| Option | Description | Selected |
|--------|-------------|----------|
| 同批做 X-Forwarded-For | 与 auth-header 同属「反代信任」主题；P3 deferred 并列两项 | ✓ |
| XFF 另立迭代 | 涉及 throttle 计数键变更，安全语义敏感 | |
| You decide | — | |

**User's choice:** 同批做 X-Forwarded-For

### Q6: XFF 的信任闸与消费范围

| Option | Description | Selected |
|--------|-------------|----------|
| XFF 与 auth-header 共用信任闸 | --auth-header 给定 = 「信任反代」总开关；logEvent remote 与 throttle per-IP 键同换 | ✓ |
| XFF 独立 flag | 两能力独立部署，但信任模型双轨 | |
| You decide | — | |

**User's choice:** XFF 与 auth-header 共用信任闸

---

## 子进程与运行形态（OPS-04/05/11）

### Q1: 子进程 cwd 与 TERM 的 CLI 形态与默认值

| Option | Description | Selected |
|--------|-------------|----------|
| --cwd + --term 两 flag，默认保持现状 | 落 cmd.Dir 与 whitelistEnv TERM= 行（spawn.go:50,65 预留位）；--cwd stat 预检 fail-fast | ✓ |
| 仅 --cwd | TERM 固定 xterm-256color（前端真实能力即此） | |
| You decide | — | |

**User's choice:** --cwd + --term 两 flag，默认保持现状

### Q2: 停止信号可配的 CLI 形态

| Option | Description | Selected |
|--------|-------------|----------|
| --stop-signal + --stop-timeout 两 flag | 默认 HUP 保持现状；timeout 默认 0 纯单信号；超时补 SIGKILL | ✓ |
| 仅 --stop-timeout | 信号固定 HUP；「先发 TERM 让程序自己清理」场景覆盖不到 | |
| You decide | — | |

**User's choice:** --stop-signal + --stop-timeout 两 flag
**Notes:** 现状 P1 D-11/P6 D-13 是 SIGHUP 进程组立即收口；Close master 内核 SIGHUP 免费通道保留为兼容底层。

### Q3: 1001 优雅下线的触发面与序列

| Option | Description | Selected |
|--------|-------------|----------|
| SIGTERM/INT → 1001 广播 → stop-signal 序列 | 1001 不在 CORE-05 重连触发集（仅 1006）；「Server shutting down」面板而非重连循环 | ✓ |
| 仅 1001，子进程靠内核兼容层 | 最简，但与 stop-signal 宽限语义不一致 | |
| You decide | — | |

**User's choice:** SIGTERM/INT → 1001 广播 → stop-signal 序列

### Q4: 降权运行的 CLI 形态

| Option | Description | Selected |
|--------|-------------|----------|
| --uid/--gid 数字，成对校验 | 避免静态二进制极简容器（无 /etc/passwd）NSS 解析差异 | ✓ |
| --user/--group 名字 + 数字兼容 | 运维手感好，但容器内名字解析失败形态要文档化 | |
| You decide | — | |

**User's choice:** --uid/--gid 数字，成对校验

### Q5: 降权后 env 白名单 HOME/USER/LOGNAME 处理

| Option | Description | Selected |
|--------|-------------|----------|
| 自动按目标 uid 改写 HOME/USER/LOGNAME | 降权直觉语义 = 连身份环境一起降；查不到则剔除三键 | ✓ |
| 不改写，文档明示 | 最简但开箱即坑（HOME 指向原用户家目录权限错乱） | |
| You decide | — | |

**User's choice:** 自动按目标 uid 改写 HOME/USER/LOGNAME

### Q6: --open 打开哪个 URL

| Option | Description | Selected |
|--------|-------------|----------|
| --open 开分享链接（rw 优先/ro 兜底） | 含 token 免交互即打即用；token 通道绕过 Basic（P5 D-01） | ✓ |
| --open 开根路径 / | Basic 弹窗输凭据的正式通道，但多点一次 | |
| You decide | — | |

**User's choice:** --open 开分享链接（rw 优先/ro 兜底）

### Q7: headless 环境下 --open 的行为

| Option | Description | Selected |
|--------|-------------|----------|
| xdg-open/open + headless 警告跳过 | 无 DISPLAY/WAYLAND_DISPLAY 时 stderr 提示不阻断（headless 是常态部署形态） | ✓ |
| headless 下 fail-fast | ssh 到服务器忘删 --open 会起不来 | |
| You decide | — | |

**User's choice:** xdg-open/open + headless 警告跳过

---

## Claude's Discretion

用户未显式选择 "You decide"，以下项由讨论中推荐形态自然落入实现自由度（已在 CONTEXT.md Claude's Discretion 节列明）：

- 配置文件两阶段合并的装配顺序与实现形态（二次解析 vs 单 pass）
- --stop-timeout flag 形态与 KILL 补发后退出码语义（P6 OQ1 accept-255 同源）
- 1001 广播与慢客户端 outbox 的写序（P6 EXIT 帧先例参照，2s 定值复用与否）
- SIGTERM/SIGINT 捕获挂点与 run() 返回收口形态（exitf 单一收口保持）
- base-path 前缀剥离的服务端装配形态（mux 模式串 vs StripPrefix 中间件）
- XFF 解析细节（多值取首个、非法值回退、空格清洗）
- --open 实现位置与 xdg-open 失败处理
- remote_user 在 logEvent 三要素的字段位置、share token 通道是否同样提取
- UAT 场景矩阵（phase07.mjs）

## Deferred Ideas

- remote_user 进 slog 结构化审计事件 — Phase 8 OPS-08
- 前端身份显示（Welcome 帧携 remote_user）— SEC-07 收窄时被裁掉的第二层价值，按真实 SSO 部署反馈再评估
- auth-header 可信来源 IP 校验（--trusted-proxy CIDR）— D-16 裸信任+警告的升级路径
- /healthz、/metrics、XFF 下指标口径 — Phase 8 OPS-06/07
- 配置文件热重载（SIGHUP reload）与多配置文件 — 无需求支撑
- 自定义首页 HTML（--index）— Phase 9 OPS-03
- 负载测试标定回填（stop-timeout 合理默认等）— Phase 9
- Windows 平台 --open 与整体 Windows 支持 — PROJECT Out of Scope（终局不做）
