# wesh

`wesh` — 通过 Web 分享终端的命令行工具：`wesh [flags] -- <cmd> [args...]` 启动后在指定端口提供 HTTP/WebSocket 服务，浏览器打开页面即获得一个运行 `<cmd>` 的完整交互终端。

> ⚠️ **wesh 提供的是一个以你身份运行的 shell。Phase 3 起认证/TLS/Origin 白名单已落地，默认配置拒绝裸奔：**
>
> - 默认 `--bind 0.0.0.0` 下**无凭据拒绝启动**（需显式 `--no-auth` 或配置凭据）；
> - 凭据 + 明文 HTTP + 非 loopback **拒绝启动**（需显式 `--insecure-http` 或 `--tls-cert`/`--tls-key`）；
> - `--bind 127.0.0.1` 本机裸跑不受限（流量不出机）。
>
> **行为变更（Phase 3）**：Phase 1/2 的 `wesh -- bash` 用法在非 loopback 监听下现在需要 `--no-auth` 或凭据才能启动——这是刻意的安全收口，不是回归。认证/TLS/逃生门语义见下方「认证与传输安全」。

## 单次语义（Phase 1）

Phase 1 为单次语义：**子进程退出或 WS 断开，服务端即整体退出**（退出码 = 子进程退出码）。断线重连与 `--once` 等完整生命周期语义在后续阶段（Phase 6）提供——**WS 断开即退出是当前阶段的预期行为，不是 bug**。

## 用法

```
wesh [flags] -- <cmd> [args...]
```

`--` 之后的命令及参数原样传递（exec 数组形式，不经 shell）。

| Flag | 默认值 | 说明 |
|------|--------|------|
| `--port` | `7681` | 监听端口；`0` = 随机端口，启动时打印实际端口 |
| `--bind` | `0.0.0.0` | 监听地址 |
| `--writable` | `false` | 只读模式：客户端输入被丢弃；开启后客户端输入写入终端 |
| `--ping-interval` | `5s` | WS ping 保活间隔（防反代空闲超时断连）；`0` = 禁用 |
| `--credential` | — | Basic 认证凭据 `user:pass`，可重复（多组按人撤销）。**flag 值对同机用户可见（`ps`），生产建议用 `WESH_CREDENTIAL` env**（flag 非空时 env 整体忽略，flag 优先） |
| `--tls-cert` | — | TLS 证书文件；必须与 `--tls-key` 成对给出才启用 TLS |
| `--tls-key` | — | TLS 私钥文件；必须与 `--tls-cert` 成对给出 |
| `--no-auth` | `false` | 逃生门：允许无凭据监听非 loopback 地址（显式声明"我知道我在裸奔"） |
| `--insecure-http` | `false` | 逃生门：允许非 loopback 明文 HTTP 携带凭据（典型场景：TLS 终止型反代之后） |
| `--origin` | — | 允许的 Origin `scheme://host[:port]`，可重复；不配则维持同源校验（无 Origin 头放行）。IPv6 字面量 Origin（如 `https://[::1]:8443`）不支持配置进白名单——同源 IPv6 访问不受影响 |
| `--version` | — | 打印版本并退出 |
| `--help` | — | 打印用法 |

启动后仅打印单行：`listening on http(s)://host:port`（无 banner、无 emoji；启用 TLS 时为 `https`）。浏览器打开该地址即进入终端；Phase 1 同时只允许一个客户端连接，第二个连接会收到 409。

## 构建

构建顺序是硬依赖：**前端构建必须先于 go build**（`go:embed all:dist` 编译期要求 `web/dist/` 存在）。

```sh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

仓库提交了前端构建产物（`web/dist/index.html` 及其 `.gz`，由 `go:embed` 嵌入二进制）——裸 clone 即可直接 `go build` / `go test ./...` 并运行。**修改 `web/` 前端源码后必须先重新 `pnpm -C web build` 再 `go build`**，否则二进制内嵌的仍是旧产物。

## 安全说明

**env 白名单（SEC-06）**：子进程只能看到以下环境变量，服务端其余环境变量一律不透传：

- 固定注入：`TERM=xterm-256color`、`COLORTERM=truecolor`
- 按名继承：`PATH`、`HOME`、`USER`、`LOGNAME`、`SHELL`
- 按前缀继承：`LANG`、`LC_*`

在 web shell 里执行 `env` 不应看到任何服务端机密变量。

**认证与传输安全（Phase 3）**：

- **整站 Basic 认证**：配置凭据后，`/` 与 `POST /api/attach` 均返回 401 challenge（`WWW-Authenticate: Basic realm="wesh"`）——浏览器打开页面弹原生登录框，输入一次后同源请求自动携带缓存凭据；无/错凭据响应完全同文（无枚举 oracle）。
- **一次性 ticket**：认证通过后前端 `POST /api/attach` 换取一次性 ticket（128bit `crypto/rand`、单次使用、60s TTL、绑定只读/可写模式），WS 握手 Hello 帧携带核销；过期/非法/重放统一 `auth_failed` + 1008 关闭，不给攻击者区分 oracle。ticket 与静态凭据是独立 secret，替代 ttyd 的 `/token` 明文下发。
- **失败节流（SEC-03）**：凭据失败与 ticket 核销失败计入同一 per-IP 指数退避计数器（1s 起翻倍、封顶 30s、认证成功清零），窗口内请求收到 429 + `Retry-After`——爆破 100 次累计等待 ≥47 分钟。
- **常数时间比较（SEC-01）**：凭据先 SHA-256 等长化再用 `crypto/subtle` 逐组比较（不短路，耗时与组数正交）；**凭据、ticket、Authorization 头任何形态（含 base64）永不进入任何日志**。
- **Origin 白名单（SEC-04）**：无 Origin 头放行（curl/脚本等非浏览器客户端）；有 Origin 必查——同源放行，`--origin` 列表内放行，否则 `/ws` 与 `/api/attach` 一律 403。
- **TLS 加固（SEC-05）**：`--tls-cert`/`--tls-key` 成对启用；MinVersion TLS 1.2（默认协商 1.3）、仅 AEAD cipher；安全响应头集合（CSP/X-Frame-Options/nosniff/Referrer-Policy/COOP/CORP 恒在，**HSTS `max-age=63072000` 仅 TLS 时发送**）。**CSP trade-off 说明**：`script-src`/`style-src` 含 `'unsafe-inline'` 是单文件全内联（`vite-plugin-singlefile` 产物）现实的已裁决接受项——`go:embed` 单 HTML 内联全部 JS/CSS 使部署只需一个二进制，代价是放弃 inline script/style 的 CSP 防护；后续阶段将评估把可行脚本拆为外部文件以移除 `'unsafe-inline'`。
- **启动校验矩阵**：默认 `0.0.0.0` 无凭据 → 拒绝启动（`--no-auth` 放行并 stderr 醒目警告）；非 loopback + 凭据 + 明文 → 拒绝启动（`--insecure-http` 放行并警告）；loopback 裸跑不受限。

**已知残余风险（DNS rebinding / CSWSH）**：同源 Origin 检查基于 Host 与 Origin host 比较，无 Host 白名单兜底——loopback 裸跑（无凭据）模式下，攻击者可经 DNS rebinding 借受害者浏览器绕过同源检查：默认只读下可实时观看终端输出，`--writable` 下升级为完整交互 shell。认证模式下一次性 ticket 闸使该路径实际不可利用——**在不可信网页浏览环境使用 loopback 裸跑时，建议配置凭据**。Host 白名单校验将随 Phase 7 SEC-07 落地。

**systemd 部署推荐形态**（凭据不进 `ps` 输出）：

```ini
[Service]
EnvironmentFile=/etc/wesh/credentials   # chmod 600，内容为 WESH_CREDENTIAL=user:pass
ExecStart=/usr/local/bin/wesh --tls-cert /etc/wesh/cert.pem --tls-key /etc/wesh/key.pem -- bash
```

**TLS 验证与证书**：手动安全审计用 testssl.sh（docker）：`docker run --rm -ti drwetter/testssl.sh --protocols --std --server-defaults --header host:port`（全量漏洞扫描加 `-U`）。自签证书请走 mkcert 或私有 CA 方向。⚠️ **HSTS 粘性提示**：`max-age` 为两年——访问过 TLS 实例的浏览器在过期前会对该 host:port 强制 HTTPS，改回 HTTP 部署需清除浏览器 HSTS 缓存或更换端口。

**协议（wesh.v1）**：WebSocket 连接必须协商子协议 `wesh.v1`（缺失或不含该值的请求在升级前以 HTTP 400 拒绝）。建连后客户端首帧必须是 Hello `{"version":"wesh.v1","cols":N,"rows":N}`——认证模式下 Hello 还须携带 `"ticket":"..."`（`POST /api/attach` 换取的一次性票；无认证模式省略该字段）；5s 内未收到合法 Hello 以 1008 关闭，抢跑（Hello 前的数据帧）或畸形帧以 1002 关闭，ticket 核销失败以 `auth_failed` + 1008 关闭。服务端握手成功回 Welcome `{"mode":"ro"|"rw"}`。所有帧为 WebSocket 二进制帧：1 字节类型 + 载荷。

| 类型字节 | 含义 | 载荷 |
|----------|------|------|
| `'H'` | Hello（C→S，必须为首帧） | JSON `{"version":"wesh.v1","cols":N,"rows":N,"ticket":"..."?}`（ticket 可选，仅认证模式） |
| `'W'` | Welcome（S→C，握手成功） | JSON `{"mode":"ro"\|"rw"}` |
| `'E'` | Error（S→C） | JSON `{"code":"...","message":"..."}` |
| `'0'` | INPUT（C→S）/ OUTPUT（S→C） | 原始字节 |
| `'1'` | RESIZE（C→S） | JSON `{"cols":N,"rows":N}`，钳制 [1,1000] |

**`POST /api/attach` 端点契约**（认证模式）：仅接受 POST（其他方法 405 + `Allow: POST`）；请求体须为空（上限 1KiB，超限 413）；认证通过返回 `200 {"ticket":"..."}` + `Cache-Control: no-store`；无/错凭据 401；Origin 不允许 403；节流窗口内 429 + `Retry-After`。无认证模式（`--no-auth`/loopback 裸跑）该端点返回 404——前端据此探测并跳过取 ticket 直连 WS。

Error 帧含三个正常客户端可见码：`version_mismatch`（随后以 1008 关闭）、`auth_failed`（ticket 过期/非法/重放/节流中统一口径，随后以 1008 关闭——前端收到后静默重取 ticket 重试一次，失败才展示）、`server_error`（随后以 1011 关闭）；攻击面路径（未知/抢跑/畸形帧、超限）直接关闭连接、不发 Error 帧——不给攻击者反馈面。

> **wire 协议稳定性契约**：`auth_failed` / `version_mismatch` / `server_error` 三个 Error code 常量字符串与「Error 帧 + 关闭码 + close reason 与 Error code 同名」的组合行为属**公开协议契约**——前端 `auth_failed` 静默重试、运维排障脚本与第三方客户端依赖该形态。变更这些常量或组合行为是向后不兼容的破坏性改动，需在 CHANGELOG/RELEASE NOTES 显著标注并同步前端实现。

关闭码全集：

| 关闭码 | 含义 |
|--------|------|
| 1000 | 正常关闭 |
| 1002 | 协议错误（未知帧/抢跑/畸形） |
| 1008 | 策略违反（Hello 超时/版本不符/认证失败） |
| 1009 | 超出消息上限 |
| 1011 | 内部错误 |
| 1001 / 1013 | 已在协议占住（服务端下线 / 踢出可重试），由后续阶段启用发送路径 |

1005/1006/1015 永不发送。

消息上限（C→S）：握手完成前（预认证窗口）4KiB，握手完成后稳态 16KiB——单帧与单消息累积字节同顶，超限由 WS 库自动以 1009 关闭并在服务端 stderr 打单行事件。保活：握手完成后服务端按 `--ping-interval`（默认 5s）发 WS ping，pong 超时 10s 主动断开连接；`0` = 禁用。

**默认只读**：不带 `--writable` 时浏览器键盘不产生输入（终端标题带 `[ro] ` 前缀），裸 WS 客户端发来的 INPUT 帧同样被服务端静默丢弃——只读是服务端边界，不只是前端行为；RESIZE 在只读下照常生效。`--writable` 开启后 Welcome 带 `mode=rw`，客户端输入写入终端。

**部署注意**：per-IP 半开连接上限（默认 8，超限 HTTP 429）在**直连部署**下有效；置于反向代理之后时所有客户端聚合为代理 IP，该限制可能误伤正常用户——可信头（X-Forwarded-For）透传属后续阶段（SEC-07）。

## 测试

```sh
go test -race -count=1 ./...
```

CI 为双平台矩阵（ubuntu + macos，含 macOS kqueue 收割验证）加独立 web 构建 job。注意 `-race` 需要 CGO，测试环境不要设 `CGO_ENABLED=0`。
