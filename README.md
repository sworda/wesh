# wesh

`wesh` — 通过 Web 分享终端的命令行工具：`wesh [flags] -- <cmd> [args...]` 启动后在指定端口提供 HTTP/WebSocket 服务，浏览器打开页面即获得一个运行 `<cmd>` 的完整交互终端。

> ⚠️ **wesh 提供的是一个以你身份运行的 shell。Phase 3 起认证/TLS/Origin 白名单已落地，默认配置拒绝裸奔：**
>
> - 默认 `--bind 0.0.0.0` 下**无凭据拒绝启动**（需显式 `--no-auth` 或配置凭据）；
> - 凭据 + 明文 HTTP + 非 loopback **拒绝启动**（需显式 `--insecure-http` 或 `--tls-cert`/`--tls-key`）；
> - `--bind 127.0.0.1` 本机裸跑不受限（流量不出机）。
>
> **行为变更（Phase 3）**：Phase 1/2 的 `wesh -- bash` 用法在非 loopback 监听下现在需要 `--no-auth` 或凭据才能启动——这是刻意的安全收口，不是回归。认证/TLS/逃生门语义见下方「认证与传输安全」。

## 生命周期（Phase 5 行为变更）

Phase 5 起 wesh 原生支持多客户端共享同一会话：**客户端断开不再使服务端退出**（Phase 1-4 为单次语义——任何 WS 断开即整体退出，该行为已终结）。服务端生命周期只随子进程：**子进程退出仍正常广播关闭全部连接并按子进程退出码退出**；无客户端期间子进程继续运行，随时可重新 attach（含分享链接）。断线自动重连与 `--once` 等完整生命周期语义在 Phase 6 提供。

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
| `--write-policy` | `owner` | 多客户端写权限：`owner` = 首个 rw attach 可写、后续 rw 旁观并按 attach 顺序递补升格；`all` = 全员可写（协作排障）。不给 `--writable` 时本 flag 无效果（全员只读） |
| `--max-clients` | `32` | 最大并发 attach 客户端数；满员时新客户端在 `/api/attach` 与 WS 握手两处收到 503 |
| `--ping-interval` | `5s` | WS ping 保活间隔（防反代空闲超时断连）；`0` = 禁用 |
| `--credential` | — | Basic 认证凭据 `user:pass`，可重复（多组按人撤销）。**flag 值对同机用户可见（`ps`），生产建议用 `WESH_CREDENTIAL` env**（flag 非空时 env 整体忽略，flag 优先） |
| `--tls-cert` | — | TLS 证书文件；必须与 `--tls-key` 成对给出才启用 TLS |
| `--tls-key` | — | TLS 私钥文件；必须与 `--tls-cert` 成对给出 |
| `--no-auth` | `false` | 逃生门：允许无凭据监听非 loopback 地址（显式声明"我知道我在裸奔"） |
| `--insecure-http` | `false` | 逃生门：允许非 loopback 明文 HTTP 携带凭据（典型场景：TLS 终止型反代之后） |
| `--origin` | — | 允许的 Origin `scheme://host[:port]`，可重复；不配则维持同源校验（无 Origin 头放行）。IPv6 字面量 Origin（如 `https://[::1]:8443`）不支持配置进白名单——同源 IPv6 访问不受影响 |
| `--client-option` | — | 客户端偏好 `key=value`，可重复；白名单键：`fontSize`/`fontFamily`/`cursorBlink`/`cursorStyle`/`scrollback`/`lineHeight`/`letterSpacing`/`theme`/`resizeOverlay`/`confirmBeforeUnload`；值为 JSON（如 `fontSize=16`、`cursorBlink=false`、`'theme={"background":"#000"}'`——含引号的 JSON 值需整体单引号包裹，防 shell 剥引号）；key 不在白名单或值非法 JSON 启动报错 |
| `--osc52` | `false` | 开启 OSC52 剪贴板写入（只写不读，默认关）；只能经本 flag 开启——URL query 与 `--client-option` 均不可设置 |
| `--version` | — | 打印版本并退出 |
| `--help` | — | 打印用法 |

启动后打印三行（无 banner、无 emoji；启用 TLS 时 scheme 为 `https`）：

```
listening on http://host:port
share read-only:  http://host:port/s/<ro-token>/
share read-write: http://host:port/s/<rw-token>/   # 仅 --writable 时打印
```

浏览器打开 `listening on` 地址即进入终端；两条 `/s/` 分享链接复制即用（ro 只读旁观 / rw 可写），见下方「多客户端共享（Phase 5）」。多客户端共享同一会话，第二客户端不再收到 409——仅在满员（`--max-clients`）时收到 503。

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

**前端体验（Phase 4）**：

- **标题同步**：终端程序经 OSC 0/2 设置的标题同步到浏览器标签页；只读模式下 `[ro] ` 前缀恒在最前（标题多次变化前缀不丢）。标题写入前经控制字符剥离与 128 字符截断防注入。
- **超链接**：终端输出中的 http(s) URL 自动识别为可点击链接；hover 悬停显示完整真实地址（可辨别显示文本与目标不一致的链接），单击在新标签页打开（noopener 形态）。
- **剪贴板**：选中即复制 + `Ctrl+Shift+V` 粘贴（现代 `navigator.clipboard` API）。**需 HTTPS 或 localhost 访问**——明文 HTTP 非 localhost 下浏览器不暴露剪贴板 API，选中复制与粘贴静默不生效（终端其余功能不受影响）。只读模式不读取剪贴板（不产生权限弹窗）。OSC52 远程写剪贴板默认关闭，`--osc52` 开启后只写不读。
- **辅助交互**：resize 期间右上角显示 `COLSxROWS` 浮层、离开页面前浏览器标准确认框拦截（均默认开；经 `--client-option resizeOverlay=false` / `confirmBeforeUnload=false` 或同名 URL query 关闭）。
- **偏好下发与覆盖**：上表白名单键可经 `--client-option` 下发；URL query 同键覆盖（如 `?fontSize=16&cursorBlink=false`，字符串值需 JSON 引号并 URL 编码）；优先级 URL query > `--client-option` > 内置默认；`theme` 为完整 JSON 对象，未指定的色键保留内置调色板；非法 query 静默忽略（终端不受影响）。

**协议（wesh.v1）**：WebSocket 连接必须协商子协议 `wesh.v1`（缺失或不含该值的请求在升级前以 HTTP 400 拒绝）。建连后客户端首帧必须是 Hello `{"version":"wesh.v1","cols":N,"rows":N}`——认证模式下 Hello 还须携带 `"ticket":"..."`（`POST /api/attach` 换取的一次性票；无认证模式省略该字段）；5s 内未收到合法 Hello 以 1008 关闭，抢跑（Hello 前的数据帧）或畸形帧以 1002 关闭，ticket 核销失败以 `auth_failed` + 1008 关闭。服务端握手成功回 Welcome `{"mode":"ro"|"rw","prefs":{...}?}`（`prefs` 为可选键）。所有帧为 WebSocket 二进制帧：1 字节类型 + 载荷。

| 类型字节 | 含义 | 载荷 |
|----------|------|------|
| `'H'` | Hello（C→S，必须为首帧） | JSON `{"version":"wesh.v1","cols":N,"rows":N,"ticket":"..."?}`（ticket 可选，仅认证模式） |
| `'W'` | Welcome（S→C，握手成功） | JSON `{"mode":"ro"\|"rw","prefs":{...}?}`（`prefs` 可选——`--client-option`/`--osc52` 下发时携带，无配置时该键缺席） |
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
| 1013 | 慢消费者踢出（Phase 5 启用）：接收过慢导致 outbox 写满的客户端被断开，close reason 为机器串 `slow_consumer`；手动刷新即可从最新输出重新 attach |
| 1001 | 已在协议占住（服务端下线），由后续阶段启用发送路径 |

1005/1006/1015 永不发送。

消息上限（C→S）：握手完成前（预认证窗口）4KiB，握手完成后稳态 16KiB——单帧与单消息累积字节同顶，超限由 WS 库自动以 1009 关闭并在服务端 stderr 打单行事件。保活：握手完成后服务端按 `--ping-interval`（默认 5s）发 WS ping，pong 超时 10s 主动断开连接；`0` = 禁用。

**默认只读**：不带 `--writable` 时浏览器键盘不产生输入（终端标题带 `[ro] ` 前缀），裸 WS 客户端发来的 INPUT 帧同样被服务端静默丢弃——只读是服务端边界，不只是前端行为。多客户端仲裁下 ro 端运行期 RESIZE 不参与尺寸仲裁（服务端直接忽略、前端 ro 形态不上报——连接不受影响）；纯 ro 会话取各端 Hello 首尺寸的最小公共矩形。`--writable` 开启后 Welcome 带 `mode=rw`，可写客户端输入写入终端。

**部署注意**：per-IP 半开连接上限（默认 8，超限 HTTP 429）在**直连部署**下有效；置于反向代理之后时所有客户端聚合为代理 IP，该限制可能误伤正常用户——可信头（X-Forwarded-For）透传属后续阶段（SEC-07）。

## 多客户端共享（Phase 5）

多个浏览器客户端可同时 attach 同一会话：输出实时扇出到全部客户端；慢客户端被保护性踢出，不拖累他人。

### 分享链接

启动时打印 ro/rw 两行分享链接，复制即用——浏览器打开链接直接进入终端（ro 只读旁观 / rw 可写），无需输入凭据；与 Basic 凭据通道正交（operator 走凭据、旁观者走链接）。token 每轮启动重新随机生成——**重启即废全部旧链接（吊销 = 重启）**；rw 行仅 `--writable` 时打印（不给 = 全员只读，现状语义不变）。

**⚠️ 反代访问日志脱敏建议**：`/s/{token}/` 的 token 在 URL 路径段，nginx/Cloudflare/Caddy 等反代的访问日志会记录完整路径——token 即访问凭证，置于反代之后时务必脱敏。nginx 二选一：

```nginx
# 形态一：map 脱敏后写入日志（保留日志，隐藏 token）
map $uri $sanitized_uri {
    ~*^/s/   /s/<redacted>/;
    default  $uri;
}
log_format wesh_sanitized '$remote_addr [$time_local] "$request_method $sanitized_uri $server_protocol" $status $body_bytes_sent';
access_log /var/log/nginx/wesh-access.log wesh_sanitized;

# 形态二：/s/ 路径直接关闭访问日志
location /s/ {
    access_log off;
    proxy_pass http://127.0.0.1:7681;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

**分享链接暴露面清单**（转发链接前请知悉）：

- 浏览器历史与地址栏可见完整链接——含屏幕共享与截屏旁观，演示时注意；
- 浏览器扩展、桌面搜索索引、杀毒软件 URL 扫描可能读取历史中的链接；
- 明文 `http://` 部署时网络监听（PCAP）可见完整 URL——**不可信网络务必 TLS**（`--tls-cert`/`--tls-key`）；
- 怀疑泄露时重启 wesh 即废全部旧链接（吊销 = 重启）。

### 写权限与递补（--write-policy）

- `owner`（默认）：首个以 rw 身份 attach 的客户端成为 owner 可写；后续 rw attach 降级旁观（`[ro] ` 标题前缀、键盘禁用）并进入递补队列——owner 断开后按 attach 顺序自动升格（标题前缀消失、键盘激活，无 toast/badge 通知组件）。
- `all`：全员可写（协作排障场景）。**多写者输入交错不做排序承诺**（与 screen 同款语义）。

### resize 行为

≥2 客户端时终端尺寸取参与集最小公共矩形，各端按自己窗口渲染、多余面积留白；减员到 1 端恢复该端尺寸。参与集按写权限分层：owner 模式仅 owner 参与（递补后新 owner 尺寸接管）；all 模式全部 rw 端参与；纯 ro 会话取各端 Hello 首尺寸。**纯 ro 会话中旁观者运行期窗口缩放不上报**（省流量裁决）——缩到小于 PTY 尺寸的旁观者看到裁剪画面，重新 attach 恢复。

### 输入限速

每客户端持续输入超过约 **32KiB/s**、或单次粘贴超过 64KiB burst 的部分会被**静默丢弃**（连接不受影响，也不会收到错误提示）——大粘贴建议分段进行。

### 慢客户端（1013）

接收过慢的客户端会被服务端以 1013 主动断开（outbox 有界写满即踢，他人不受影响）——页面显示 Disconnected 面板，**手动刷新**即可从最新输出看起（URL 中 token 保留，刷新凭原链接重新 attach）。当前版本无自动重连（Phase 6 提供）。

### 容量（--max-clients）

`--max-clients` 默认 32；满员时新客户端在 `/api/attach` 与 WS 握手两处收到 503（前端显示 Server is full 面板），槽位随断开/踢出释放。计数口径为注册成功后计数——**并发握手瞬时超编 ≤8**（容量策略非安全边界，per-IP 半开帽为界）。

### 默认参数与 Phase 9 标定

下列初值为一阶推算的合理值（非负载实测），Phase 9 负载标定后回填：

| 参数 | 初值 | 一阶依据 |
|------|------|----------|
| outbox 字节容量/客户端 | 512KiB | 16×32KiB 读块；100KB/s 慢链路约 5s 抖动容忍；32 客户端账面最坏 16MiB（共享帧实占更低） |
| 信用门恢复水位 | 50% | 半水位迟滞防门震颤 |
| 输入限速 rate / burst | 32KiB/s / 64KiB | 人类击键 ~10B/s、快粘 ~50KB 瞬时；持续超限远超合法、远低于洪水 |
| `--max-clients` | 32 | 团队围观/教学场景区间下沿；账面内存与 goroutine 开销微小 |
| resize 防抖 | 50ms | SIGWINCH 风暴防线 |

标定方法 = **负载矩阵**（客户端数 1/4/16/32 × 输出速率 × 慢链路注入），验收标准 = 合法慢端零误踢 + 内存上界成立 + 信用门开闭频率可接受；数据源 = 本 phase 已埋计数器（踢出数/门开闭次数/输入丢弃计数/注册数，Phase 8 接入 metrics）。

### 行为变更（单客户端 → 多客户端）

- **客户端断开不再使服务端退出**（旧版单次语义终结）；子进程退出仍正常关闭全部连接并退出。
- 第二客户端不再收到 409——仅满员（`--max-clients`）时收到 503。

## 测试

```sh
go test -race -count=1 ./...
```

CI 为双平台矩阵（ubuntu + macos，含 macOS kqueue 收割验证）加独立 web 构建 job。注意 `-race` 需要 CGO，测试环境不要设 `CGO_ENABLED=0`。
