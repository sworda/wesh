# wesh

`wesh` — 通过 Web 分享终端的命令行工具：`wesh [flags] -- <cmd> [args...]` 启动后在指定端口提供 HTTP/WebSocket 服务，浏览器打开页面即获得一个运行 `<cmd>` 的完整交互终端。

> ⚠️ **wesh 提供的是一个以你身份运行的 shell。Phase 3 起认证/TLS/Origin 白名单已落地，默认配置拒绝裸奔：**
>
> - 默认 `--bind 0.0.0.0` 下**无凭据拒绝启动**（需显式 `--no-auth` 或配置凭据）；
> - 凭据 + 明文 HTTP + 非 loopback **拒绝启动**（需显式 `--insecure-http` 或 `--tls-cert`/`--tls-key`）；
> - `--bind 127.0.0.1` 本机裸跑不受限（流量不出机）。
>
> **行为变更（Phase 3）**：Phase 1/2 的 `wesh -- bash` 用法在非 loopback 监听下现在需要 `--no-auth` 或凭据才能启动——这是刻意的安全收口，不是回归。认证/TLS/逃生门语义见下方「认证与传输安全」。

## 生命周期（Phase 5/6 行为变更）

Phase 5 起 wesh 原生支持多客户端共享同一会话：**客户端断开不再使服务端退出**（Phase 1-4 为单次语义——任何 WS 断开即整体退出，该行为已终结）。默认配置下服务端生命周期只随子进程：无客户端期间子进程继续运行，随时可重新 attach（含分享链接）。

**子进程退出（Phase 6 终结帧）**：子进程退出时，服务端先向所有在线客户端广播 EXIT 帧（`'X'`，含退出码与人话消息——信号死亡 `exit_code=-1`、消息含大写信号名），再以 1000 正常关闭全部连接：客户端看到「Session ended」面板与退出码/信号提示，**非静默断开**。wesh 进程按子进程退出码退出。

**`--once`（单客户端单次语义）**：只接受一个客户端，其断开后服务端退出——等价关系：`--once` ≡ `--max-clients=1 --exit-when-empty=0`（语法糖，展开只填未显式给定的位；与显式矛盾值同给拒绝启动）。第二客户端在 `/api/attach` 与 WS 握手两处收到 503（既有 `--max-clients` 计数路径，409 不复活）。

**`--exit-when-empty[=duration]`（所有客户端断开后退出）**：三形态——不写 = 不开启（默认：无客户端时子进程继续运行）；裸写 = 最后一个客户端断开立即退出；`=duration`（如 `--exit-when-empty=30s`）= 重连宽限，计时内任一端 attach 成功即取消退出。「所有客户端断开」是最后一个客户端断开的**迁移事件**——启动后尚无客户端期间不触发退出（不是「启动时为空即退」）。duration 只能经 `=` 号形态传入（`--exit-when-empty 30s` 空格分隔形态不传值，`30s` 会被当作命令参数）。

**断开退出收口**：`--once` / `--exit-when-empty` 触发时服务端向子进程进程组发 SIGHUP——子进程被 SIGHUP 终结，wesh 退出状态 255。

### 断线自动重连（Phase 6）

浏览器端 WS **异常断开**（无码 1006 类：断网/TCP 断开/保活超时）后前端自动重连：

- **触发范围**：仅异常断开自动重连；服务端明确关闭（1000/1008/1009/1011）不自动重连，被踢（1013 慢消费者）维持手动刷新——被踢说明消费跟不上，自动重连只会再被踢。
- **退避**：1s 起翻倍、封顶 30s、无限重试；重连成功（收到 Welcome）退避清零。等待期面板显示 attempt 计数与下次重试倒计时，点「Reconnect now」立即跳过等待；浏览器 online 事件也会立即触发一次重试。
- **重连目标 = 同一 URL 的当前进程**：共享进程模型下重连即接回原 PTY 进程，输入输出一致。**服务端重启后是全新会话**——share token 重启即废，重启后需用新链接（旧链接页面停在连接失败面板）。
- **无滚动回放**：重连成功清屏后靠程序重绘（服务端 SIGWINCH 强制全屏程序秒级重绘）；行内 shell 历史不在重连窗口恢复——跨断线的屏幕现场由 tmux/herdr 覆盖（既定分工）。
- **写权限不恢复**：owner 断线重连按新 attach 走递补语义（降级旁观入队、`[ro] ` 前缀），不恢复写权限。

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
| `--once` | `false` | 只接受一个客户端并在其断开后退出；等价于 `--max-clients=1 --exit-when-empty=0`（语法糖——显式给定矛盾值时拒绝启动，一致冗余放行） |
| `--exit-when-empty` | 不开启 | 所有客户端断开后退出：裸写 = 最后一个客户端断开立即退出；`=duration` = 重连宽限（如 `--exit-when-empty=30s`，计时内任一端 attach 取消退出）；duration 只能经 `=` 号给出，空格分隔形态不传值 |
| `--ping-interval` | `5s` | WS ping 保活间隔（防反代空闲超时断连）；`0` = 禁用 |
| `--credential` | — | Basic 认证凭据 `user:pass`，可重复（多组按人撤销）。**flag 值对同机用户可见（`ps`），生产建议用 `WESH_CREDENTIAL` env**（flag 非空时 env 整体忽略，flag 优先） |
| `--tls-cert` | — | TLS 证书文件；必须与 `--tls-key` 成对给出才启用 TLS |
| `--tls-key` | — | TLS 私钥文件；必须与 `--tls-cert` 成对给出 |
| `--no-auth` | `false` | 逃生门：允许无凭据监听非 loopback 地址（显式声明"我知道我在裸奔"） |
| `--insecure-http` | `false` | 逃生门：允许非 loopback 明文 HTTP 携带凭据（典型场景：TLS 终止型反代之后） |
| `--origin` | — | 允许的 Origin `scheme://host[:port]`，可重复；不配则维持同源校验（无 Origin 头放行）。IPv6 字面量 Origin（如 `https://[::1]:8443`）不支持配置进白名单——同源 IPv6 访问不受影响 |
| `--client-option` | — | 客户端偏好 `key=value`，可重复；白名单键：`fontSize`/`fontFamily`/`cursorBlink`/`cursorStyle`/`scrollback`/`lineHeight`/`letterSpacing`/`theme`/`resizeOverlay`/`confirmBeforeUnload`；值为 JSON（如 `fontSize=16`、`cursorBlink=false`、`'theme={"background":"#000"}'`——含引号的 JSON 值需整体单引号包裹，防 shell 剥引号）；key 不在白名单或值非法 JSON 启动报错 |
| `--osc52` | `false` | 开启 OSC52 剪贴板写入（只写不读，默认关）；只能经本 flag 开启——URL query 与 `--client-option` 均不可设置 |
| `--config` | — | 加载 TOML 配置文件（仅显式指定路径；CLI 参数覆盖配置文件）——见「部署与配置」 |
| `--socket` | — | UNIX socket 监听路径（与显式 `--port`/`--bind` 互斥）——见「部署与配置」 |
| `--socket-mode` | `0660` | UNIX socket 权限位（八进制）；仅随 `--socket` 有意义 |
| `--socket-owner` | — | UNIX socket 属主 `user[:group]`；仅随 `--socket` 有意义 |
| `--base-path` | — | 反代子路径挂载前缀（如 `/wesh`；必须 `/` 开头、无尾斜杠）——见「部署与配置」 |
| `--index` | — | 自定义首页 HTML 文件路径：整页替换内建终端页（ttyd `-i` 同款），分享链接同效；启动一次读入，改文件需重启生效——见「部署与配置」 |
| `--auth-header` | — | 可信反代用户头名（如 `X-Remote-User`）；头值仅记录进服务端日志 `remote_user` 字段（审计归因，无认证效力），仅反代后部署——见「部署与配置」 |
| `--cwd` | 继承 | 子进程工作目录（默认继承服务端 cwd；启动时预检，不存在拒绝启动） |
| `--term` | `xterm-256color` | 子进程 TERM |
| `--stop-signal` | `HUP` | 关停时发给子进程进程组的信号：`HUP`\|`TERM`\|`INT`\|`KILL` |
| `--stop-timeout` | `0` | stop-signal 后补发 SIGKILL 的宽限（`0` = 不补发） |
| `--uid` | — | 降权目标 uid（数字；须与 `--gid` 成对给出） |
| `--gid` | — | 降权目标 gid（数字；须与 `--uid` 成对给出） |
| `--open` | `false` | 启动后以系统启动器打开分享链接（`--writable` 开 rw 链接，否则 ro 链接；headless 提示后跳过） |
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

仓库提交 `web/dist/index.html` 占位（由 `go:embed` 嵌入二进制——裸 clone 即可直接 `go build` / `go test ./...` 并运行）；`.gz` 预压产物由 `pnpm -C web build` 生成、不入库（`.gitignore` 忽略 `web/dist/*.gz`），发布构建在 CI 侧完成（发布二进制含 `.gz`）。**修改 `web/` 前端源码后必须先重新 `pnpm -C web build` 再 `go build`**，否则二进制内嵌的仍是旧产物。

## 发布

发布物由 goreleaser 经 `.github/workflows/release.yml` 构建：**推送 `v*` tag 自动触发**（版本史与 git tag 同源，起点 v1.0.0）。产物命名族（linux/darwin × amd64/arm64 四平台——Windows 不在支持范围；CGO_ENABLED=0 全静态 + `-trimpath` + `-X main.version` 注入，`--version` 可核对）：

```
wesh_v1.0.0_linux_amd64.tar.gz
wesh_v1.0.0_linux_arm64.tar.gz
wesh_v1.0.0_darwin_amd64.tar.gz
wesh_v1.0.0_darwin_arm64.tar.gz
checksums.txt
```

每个 tar.gz 为三件套：`wesh` + `LICENSE` + `README.md`——解压即见文档，scp 单文件即用。

**完整性验证**：下载产物后以 `checksums.txt` 核对 sha256：

```sh
sha256sum -c checksums.txt --ignore-missing   # 只验本机已下载的产物
```

**发布流程 = `scripts/release.sh`，发布之前跑一次即可**（所有发布前操作单脚本整合，脚本即发布文档的可执行形态——本节描述流程、脚本承载流程，两者同源不漂移）：

```sh
./scripts/release.sh --dry-run v1.0.0   # 干跑：只跑前置校验四闸，打印步骤清单不执行
./scripts/release.sh v1.0.0             # 真实发布：前置校验 → 全量测试 → 前端构建
                                        #   → 长 fuzz ×2（每目标 10 分钟）→ 负载矩阵
                                        #   → 确认闸 → tag push
```

前置校验四闸：tag 形态（`vX.Y.Z`）/ tag 不存在 / 工作树干净 / 与远端同步（无网络或无上游时该闸降级为跳过提示，不阻塞）；确认闸回显将创建的 tag 与最近 5 条提交，应答 `yes` 才落 tag。fuzz 崩溃即中止——崩溃语料自动落 `testdata/fuzz/`，修复后重跑脚本。tag push 后 release.yml 接管：**pnpm build 先于 goreleaser**（workflow 步骤显式编排，不用 goreleaser before hooks）；`CGO_ENABLED=0` 仅属于发布构建（`.goreleaser.yml` 单侧持有——本地 `go test -race` 需要 cgo）。

**两裁决明示**：供应链文件仅 `checksums.txt`（无 cosign 签名/SBOM——个人运维工具威胁模型裁决）；**不发布容器镜像**（Dockerfile 入库用户自建，见「部署与配置 → Docker」，与单二进制 scp 哲学一致）。

## 安全说明

**env 白名单（SEC-06）**：子进程只能看到以下环境变量，服务端其余环境变量一律不透传：

- 固定注入：`TERM=xterm-256color`、`COLORTERM=truecolor`
- 按名继承：`PATH`、`HOME`、`USER`、`LOGNAME`、`SHELL`
- 按前缀继承：`LANG`、`LC_*`

在 web shell 里执行 `env` 不应看到任何服务端机密变量。

**认证与传输安全（Phase 3）**：

- **整站 Basic 认证**：配置凭据后，`/` 与 `POST /api/attach` 均返回 401 challenge（`WWW-Authenticate: Basic realm="wesh"`）——浏览器打开页面弹原生登录框，输入一次后同源请求自动携带缓存凭据；无/错凭据响应完全同文（无枚举 oracle）。唯一例外：`GET /healthz` 探活端点免认证（零敏感信息，探活器结构性带不了凭据）——见「运维（Phase 8）」。
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

**协议（wesh.v1）**：WebSocket 连接必须协商子协议 `wesh.v1`（缺失或不含该值的请求在升级前以 HTTP 400 拒绝）。建连后客户端首帧必须是 Hello `{"version":"wesh.v1","cols":N,"rows":N}`——认证模式下 Hello 还须携带 `"ticket":"..."`（`POST /api/attach` 换取的一次性票；无认证模式省略该字段）；分享 token 通道（含无认证模式）Hello 同样携 ticket——token 经 `/api/attach` 换一次性 ticket 后随 Hello 核销；5s 内未收到合法 Hello 以 1008 关闭，抢跑（Hello 前的数据帧）或畸形帧以 1002 关闭，ticket 核销失败以 `auth_failed` + 1008 关闭。服务端握手成功回 Welcome `{"mode":"ro"|"rw","cols":N,"rows":N,"prefs":{...}?}`（`cols`/`rows` 恒在 = 当前会话尺寸；`prefs` 为可选键）。所有帧为 WebSocket 二进制帧：1 字节类型 + 载荷。

| 类型字节 | 含义 | 载荷 |
|----------|------|------|
| `'H'` | Hello（C→S，必须为首帧） | JSON `{"version":"wesh.v1","cols":N,"rows":N,"ticket":"..."?}`（ticket 可选，认证模式或分享 token 通道携带） |
| `'W'` | Welcome（S→C，握手成功） | JSON `{"mode":"ro"\|"rw","cols":N,"rows":N,"prefs":{...}?}`（`cols`/`rows` 恒在 = 当前会话尺寸；运行期尺寸变化经 `'W'` 帧再推送——递补升格推送同通道先例——前端据以约束视口渲染；`prefs` 可选——`--client-option`/`--osc52` 下发时携带，无配置时该键缺席） |
| `'E'` | Error（S→C） | JSON `{"code":"...","message":"..."}` |
| `'0'` | INPUT（C→S）/ OUTPUT（S→C） | 原始字节 |
| `'1'` | RESIZE（C→S） | JSON `{"cols":N,"rows":N}`，钳制 [1,1000] |
| `'X'` | EXIT（S→C，子进程退出终结帧） | JSON `{"exit_code":N,"message":M}`——信号死亡 `exit_code=-1`、`message` 含大写信号名；先于 1000 广播，前端「Session ended」面板直显 message |

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
| 1001 | 服务端优雅下线（Phase 7 启用）：wesh 捕获 SIGTERM/SIGINT 后向全部客户端广播，close reason 为机器串 `server_shutting_down`；前端显示「Server shutting down」终态面板，**不进入自动重连**——见「部署与配置 → 优雅下线」 |

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

≥2 客户端时终端尺寸取参与集最小公共矩形（会话尺寸），经 Welcome 帧下发、运行期尺寸变化经 `'W'` 帧再推送更新；窗口大于会话矩形的端约束视口到会话尺寸渲染（超出面积为页面背景留白，行编辑回显等相对寻址流异尺寸双端逐屏一致）；窗口小于会话尺寸的轴按窗口渲染（裁剪语义不变）；减员到 1 端恢复该端尺寸（last-wins，推送解除约束）。参与集按写权限分层：owner 模式仅 owner 参与（递补后新 owner 尺寸接管）；all 模式全部 rw 端参与；纯 ro 会话取各端 Hello 首尺寸。**纯 ro 会话中旁观者运行期窗口缩放不上报**（省流量裁决）——缩到小于 PTY 尺寸的旁观者看到裁剪画面，重新 attach 恢复。

### 输入限速

每客户端持续输入超过约 **32KiB/s**、或单次粘贴超过 64KiB burst 的部分会被**静默丢弃**（连接不受影响，也不会收到错误提示）——大粘贴建议分段进行。

### 慢客户端（1013）

接收过慢的客户端会被服务端以 1013 主动断开（outbox 有界写满即踢，他人不受影响）——页面显示 Disconnected 面板，**手动刷新**即可从最新输出看起（URL 中 token 保留，刷新凭原链接重新 attach）。1013 被踢不自动重连（消费跟不上的端重连只会再被踢）——Phase 6 自动重连仅覆盖异常断开，见「生命周期」节。

### 容量（--max-clients）

`--max-clients` 默认 32；满员时新客户端在 `/api/attach` 与 WS 握手两处收到 503（前端显示 Server is full 面板），槽位随断开/踢出释放。计数口径为注册成功后计数——**单源 IP 瞬时超编 ≤ per-IP 半开帽（默认 8）**（容量策略非安全边界）。

### 默认参数与标定

下列默认值已经负载矩阵实测验证（2026-08-29，`go test -tags=load`——internal/server 黑盒负载测试；「验证为主、证伪才改」纪律）：全部现值成立，零证伪、零常量改动。

| 参数 | 默认值 | 标定结论 |
|------|--------|----------|
| outbox 字节容量/客户端 | 512KiB | **实测成立**——{1,4,16,32} 端洪水（34.9MB/端，32 端格总扇出 ≈1.1GB）全端收流逐字节一致、kicks=0；活跃读格 outbox 峰值 ≤133KiB（裕度 ≈4×）；限速承压格峰值 523,449B ≈ 容量 99.8% 精确转信用，无溢出无踢出 |
| 信用门恢复水位 | 50% | **实测成立**——突发 2.2MB/s × 限速 600KB/s 承压格 16.7s 内门开闭 6 次（0.36/s），半水位迟滞不震颤 |
| 输入限速 rate / burst | 32KiB/s / 64KiB | **实测成立**——矩阵全格 kicks=0、洪水触发 INPUT 全格正常送达；行为测试已锁（持续超限静默丢弃、burst 内放行） |
| 会话输入队列容量 | 256KiB | **实测成立**——矩阵全格输入链路正常（限速器在前、队列满为设计罕见位）；行为测试已锁（对照组全量送达） |
| `--max-clients` | 32 | **实测成立**——32 端洪水 Alloc 峰值 19.8MiB ≤ 64MiB（账面最坏 4×），GC 后回基线；200 轮高频建销 goroutine/fd 精确回基线、零 Z 态 |
| resize 防抖 | 50ms | 行为测试已锁 + 一阶依据复核成立（SIGWINCH 风暴防线） |
| attach 宽限 | 500ms | **实测成立**——{1,4,16,32} 端洪水 + 新端 attach 全程 kicks=0：宽限内满箱一律转信用不误踢（三轮 kick_fail 误踢现场封堵后零回归） |
| pong 超时 | 10s | 行为测试已锁 + 一阶依据复核成立（ping 间隔 5s × 2） |
| Hello 超时 | 5s | 行为测试已锁 + 一阶依据复核成立 |
| EXIT 直写超时 | 2s | 行为测试已锁 + 一阶依据复核成立（stall 客户端不拖延全局终结） |
| `--stop-timeout` 默认 | 0 | 行为测试已锁 + 一阶依据复核成立（纯单信号形态；`>0` 时超时补发 SIGKILL） |
| `--exit-when-empty` 宽限 | 无内置默认 | 行为测试已锁（立即退出/宽限取消/宽限到期三形态）+ 一阶依据复核成立（`duration` 由用户给定，无默认值可标定） |

标定方法 = **负载矩阵**（客户端数 1/4/16/32 × 输出速率 × 慢链路注入），验收标准 = 合法慢端零误踢 + 内存上界成立 + 信用门开闭频率可接受；数据源 = /metrics 计数器（踢出数/门开闭次数/输入丢弃计数/注册数）+ runtime 内存/fd/goroutine 采样。该方法论已兑现——上表实测结论即其产出：负载敏感类参数以矩阵实测数据回填，时序类参数以既有行为测试锁定为一阶依据复核。

### 行为变更（单客户端 → 多客户端）

- **客户端断开不再使服务端退出**（旧版单次语义终结）；子进程退出仍正常关闭全部连接并退出。
- 第二客户端不再收到 409——仅满员（`--max-clients`）时收到 503。

## 部署与配置（Phase 7）

生产部署完整面：TOML 配置文件、自定义首页、UNIX socket、反代（nginx/Caddy/Cloudflare）、反代身份透传、子进程管理、降权运行、自动开浏览器、Docker、systemd、优雅下线。

### 配置文件（--config）

`--config /etc/wesh/wesh.toml` 显式指定 TOML 配置文件——**仅显式指定路径，零隐式默认路径搜索**（裸 `wesh -- bash` 行为与无配置文件时逐字节一致）。

平铺 `key = value` 形状，**键名 = flag 名**：27 个长期运行 flag 同名键 + `command` exec 数组 + `index-max-size` 纯配置键，共 29 键。

```toml
# /etc/wesh/wesh.toml —— 含 credential 键时建议 chmod 600
bind = "127.0.0.1"
port = 7681
credential = ["alice:pw-of-alice"]   # 可重复 flag ↔ TOML 数组
base-path = "/wesh"
index = "/srv/wesh/index.html"       # 自定义首页（--index 同名键，见「自定义首页」节）
index-max-size = 33554432            # 纯配置键：自定义首页读入上限（整数字节，默认 16MiB）
max-clients = 16
ping-interval = "5s"                 # duration 键为字符串形态
exit-when-empty = "30s"              # "true"/"0"/"30s" 与 CLI 三形态同语义
command = ["bash", "-l"]             # exec 数组；CLI `--` 后 argv 非空则覆盖
```

- **优先级链**：CLI flag > env（`WESH_CREDENTIAL`）> 配置文件 > 内置默认。
- **列表替换语义**：`credential`/`origin`/`client-option` 三列表键——CLI 给出则配置列表整体替换（不应用）；`credential` 另有 env 夹层（`WESH_CREDENTIAL` 非空时配置列表同样不应用）。CLI 未给且配置键存在时，配置列表逐项经与 CLI 相同的 parse 期校验。
- **`command` 键**：CLI `--` 后 argv 非空则覆盖；`command = []` 空数组等价缺席。
- **逃生门五键不可入配置**：`no-auth`/`insecure-http`/`version`/`help`/`config`——写入配置文件按未知键拒绝（逃生门必须显式说出口，写在配置里等于没说）。
- **严格模式**：文件不存在、TOML 解析失败、未知键均以 exit 2 拒绝启动；错误文案只含类别 + 键名 + 行号，**不回显配置值**。
- **`index-max-size` 纯配置键例外**：自定义首页读入上限（整数字节，默认 16MiB）只经配置文件调整，**无对应 CLI flag**——「配置键 = flag 名」纪律的明示例外（上限属低频调参，配置文件承载；此例外不蔓延，详见「自定义首页」节）。
- **权限建议（chmod 600）**：含 `credential` 键且文件权限非 600/400 时 stderr 警告放行（不阻断——挂载盘/容器 secret 权限语义不可靠），建议 `chmod 600`。**`WESH_CREDENTIAL` env 优先于配置文件明文**——生产凭据首选 env（systemd `EnvironmentFile=` 600 通道，见「安全说明」），不写入配置文件。

### 自定义首页（--index）

`--index /srv/wesh/index.html` 用你的 HTML **整页替换**内建终端页（ttyd `-i` 同款语义）：启动时一次读入内存常驻，运行期零磁盘依赖——**改文件需重启生效**。`/` 根路径与 `/s/{token}/` 分享链接两通道统一替换（同一字节源）；`/api/attach`、`/ws`、`/healthz`、`/metrics` 照旧暴露不受影响。可经 TOML 配置同名键 `index` 给定（CLI flag 覆盖配置）。

自定义页是用户自治页面——wesh 零注入、零模板、零内容校验；页面的可访问性、视觉质量、终端逻辑正确性由你自负。两义务承诺：

- 自定义页完全替代内建终端页：**终端功能须自行实现**（POST /api/attach 换 ticket + wesh.v1 WS 协议回连），否则根路径与分享链接将失去终端功能。
- 自定义页须为**自包含单 HTML**（内联一切脚本/样式/资源）：wesh 不伺服其引用的相对路径资源（404）；wesh 安全头 CSP 允许内联脚本与同源 WS 连接，但阻断外部源资源（CDN 脚本/样式/图片/webfont），与内建页同约束。

**大小上限**：默认 **16MiB** 硬顶（启动读入上限，防误指大文件 OOM）。上限经 TOML 纯配置键 `index-max-size`（整数字节）调整——该键**无对应 CLI flag**，是「配置键 = flag 名」纪律的明示例外（见上节）；`0`/负值拒绝启动。

**启动校验（exit 2 fail-fast）**：文件不存在 / 不可读 / 非常规文件（目录、设备、socket）/ 超上限，四态拒绝启动；错误行只含路径与原因类别，**不含文件内容任何字节**（启动面红线——路径非敏感可回显，HTML 内容是探针面）。

### UNIX socket（--socket）

三 flag：`--socket /run/wesh/wesh.sock`（与显式 `--port`/`--bind` **互斥**，组合冲突拒绝启动）+ `--socket-mode 0660`（八进制，默认 `0660`）+ `--socket-owner user[:group]`。后两者仅随 `--socket` 有意义——单独给出拒绝启动。

- **残留清理**：listen 前自动删除既有 socket 文件（崩溃/systemd Restart= 场景零人工干预）；listen 后显式 Chmod/Chown——权限位确定性不依赖进程 umask。
- **文件系统权限即认证边界**：unix socket 形态跳过 bind 安全校验矩阵（无凭据/TLS 均放行免警告）——访问控制由 `--socket-mode`/`--socket-owner` 承担，流量不出机。
- **启动打印退化**：地址行 `listening on unix:///run/wesh/wesh.sock`；分享链接两行退化为单行提示（无 host:port 可拼——反代后的分享链接由反代 URL 决定）。

systemd 配方（unix socket + 反代是生产推荐形态之一）：

```ini
# /etc/systemd/system/wesh.service
[Service]
RuntimeDirectory=wesh
EnvironmentFile=/etc/wesh/credentials   # chmod 600，内容为 WESH_CREDENTIAL=user:pass
ExecStart=/usr/local/bin/wesh --socket /run/wesh/wesh.sock --socket-owner www-data:www-data -- bash
```

### 反代子路径（--base-path）

`--base-path /wesh` 把 wesh 挂到子路径下——值必须 `/` 开头、无尾斜杠（根 `/` 视为未配置；拒绝 `..`/重复斜杠/非 URL path 安全字符，非法值拒绝启动，绝不宽容自动修正）。裸 `/wesh`（无尾斜杠）由服务端 307 规范化到 `/wesh/`——尾斜杠是前端相对 URL 正确解析的硬要求。分享链接打印含 base-path 前缀。

nginx 配方（前缀块必需；精确块推荐——理据见块内注释）：

```nginx
# 把 Connection 头映射为 upgrade/close（WS 升级必需）
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    # 精确块（推荐）：location /wesh/ 是前缀匹配，不匹配裸 /wesh（无尾斜杠）；本配方
    # （proxy_pass handler）下 nginx 对裸 /wesh 会自动 301 补斜杠（GET 入口可工作），
    # 但该自动跳转是 proxy_pass 系 handler 特例——换 return/fastcgi 等形态即不存在。
    # 此块显式 308 重定向：308 保方法 + 规范化行为与 handler 形态无关，故推荐保留。
    location = /wesh { return 308 /wesh/; }

    location /wesh/ {
        proxy_pass http://127.0.0.1:7681;
        proxy_http_version 1.1;
        # Host 必须原样转发：nginx 默认转发 $proxy_host（127.0.0.1:后端口），与浏览器 Origin 不同源会被 wesh WS 同源校验 403；$host 剥端口在 Origin 含非默认端口时仍不匹配——必须 $http_host（已全链实证）
        proxy_set_header Host $http_host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 3600s;
    }
}
```

**`proxy_read_timeout` 必须大于 `--ping-interval`（默认 `5s`）**：反代空闲超时看应用层流量——WS 建立后若无数据往来，超时到期反代主动断连。wesh 服务端每个 ping 间隔发一帧 WS ping（应用层流量），故 `proxy_read_timeout` 大于 ping 间隔即不会误断空闲连接；`3600s` 是充裕值。`--ping-interval 0` 禁用保活时，空闲连接存活性完全取决于反代超时。

### 反向代理：Caddy（已实证）

nginx 之外的轻量反代选项。已全链实证（2026-08-30，Caddy v2.11.4；Linux 协议层七断言 + Windows 浏览器双机全链）。根路径反代只需一条指令：

```caddyfile
wesh.example.com {
    reverse_proxy 127.0.0.1:7681
}
```

与 nginx 的三个关键差异（两平台默认语义相反，**配方互抄必错**）：

- **Host 默认原样透传**——wesh Origin 同源校验天然通过，**不需要任何 Host 配置行**（nginx 默认转发 `$proxy_host`，必须显式 `proxy_set_header Host $http_host`，见上节配方）。
- **WS upgrade 内建自动处理**——零 upgrade 配置行（nginx 须 `proxy_set_header Upgrade`/`Connection` 映射）。
- **站点地址语义相反**：Caddyfile 站点地址写 `http://0.0.0.0:PORT` 是**字面 Host 匹配**（仅 Host: 0.0.0.0 的请求命中），不是 nginx 的「绑定全网卡」监听语义——LAN 监听站点地址须写**裸 `:PORT`**（绑定全网卡 + 匹配任意 Host）；上例域名形态不受影响。

XFF 默认添加（`--auth-header` 可选消费）。**空闲超时**：Caddy 对 hijack 后的 WS 连接无默认 idle 超时（65s 空闲存活实测）——wesh 默认 `--ping-interval 5s` 远小于任何中间盒 idle 阈值，无需额外配置；`--ping-interval 0` 禁用保活时，连接存活性取决于路径上的其他中间设备。

### 反向代理：Cloudflare（未实测）

**本节按 Cloudflare 官方文档书写，未经实测**（SaaS 反代无本机复现条件——部署配方实证分级中唯一例外；nginx/Caddy 均经全链实证）。要点：

- **DNS 橙云代理**：wesh 域名记录开代理（橙云）即经 CF 边缘；**WebSockets 默认开启**（Network 面板）。
- **空闲超时**：社区共识约 **~100s 无流量关连**（该数值未能从官方文档直取，为多源社区共识）——wesh 默认 `--ping-interval 5s` 应用层 ping 使连接恒有流量，**默认即安全**；`--ping-interval 0` 禁用保活时空闲连接将在 ~100s 被 CF 关闭（前端 1006 自动重连可恢复，见「断线自动重连」）。
- **TLS**：CF 边缘终止；源站建议 **Full (strict)**（wesh 配 `--tls-cert`/`--tls-key` 真实证书），或源站仅 loopback 监听 + CF 回源（源站不直接暴露）。
- **Host 默认保持**（同 Caddy）；`--base-path` 子路径挂载在 CF 后照常可用。
- **`/s/{token}/` 对 CF 明文可见**：分享 token 经 CF 边缘与访问日志（见「分享链接」节脱敏建议——在 CF 语境同样适用，且 CF 侧日志不在你的掌控内，转发分享链接前请知悉）。

### 反代身份透传（--auth-header）与 X-Forwarded-For

**语义 = 服务端审计归因**：`--auth-header X-Remote-User` 配置即信任该头——反代（authelia/oauth2-proxy 等 SSO）注入的用户名经清洗（剥离控制字符、截断 128 字符）后记录进服务端 stderr 事件行的 `remote_user` 字段：

```json
{"time":"2026-08-28T10:40:01.013456789+08:00","level":"INFO","msg":"event","event":"detach","remote":"198.51.100.7","client_id":1,"code":1000,"reason":"normal","remote_user":"alice"}
```

**X-Forwarded-For 同闸**：`--auth-header` 给定即「信任反代」总开关——XFF 链首 IP 同时换入日志 `remote` 字段与 per-IP 节流计数键（反代后 per-IP 计数不再全聚合为代理 IP）；未配置时 XFF 完全忽略（直连客户端自设 XFF 零效果）。

**信任模型 = 裸信任 + 暴露面警告，仅反代后部署**：配置即信任该头（ttyd `-H` 同款）。非 loopback 监听且无凭据时 stderr 醒目警告「可被直连伪造，确保 wesh 不直接暴露」。伪造头**不能绕过认证**——头值只做日志记录，Basic/ticket/share token 三通道语义全不变；但直连伪造会污染审计归因，故**必须置于设置该头的反代之后**，不得直接暴露。

**与 ttyd `-H` 的模型差异（重要）**：ttyd `-H` 把头值注入**每个连接的子进程环境变量**——ttyd 是 per-connection spawn 模型（每个客户端连接独立 spawn 一个 shell，连接建立时 HTTP 请求在手）。wesh 采用 GoTTY 共享进程模型：PTY 随服务端启动创建（spawn 时无任何 HTTP 请求）、多客户端共享同一个 shell（env 是一次性快照，写谁的名字都错）——「shell 内感知当前用户身份」在共享模型下**结构性不成立**。故 wesh 的 `--auth-header` 收窄为**服务端审计归因**：身份记录进服务端日志，不进子进程环境。

### 子进程管理（--cwd/--term/--stop-signal/--stop-timeout）

| Flag | 默认值 | 说明 |
|------|--------|------|
| `--cwd` | 继承服务端 cwd | 子进程工作目录；启动时预检，不存在拒绝启动 |
| `--term` | `xterm-256color` | 子进程 TERM |
| `--stop-signal` | `HUP` | 关停时发给子进程**进程组**的信号：`HUP`\|`TERM`\|`INT`\|`KILL`（shell 的孩子如 vim 同组同收，不留孤儿） |
| `--stop-timeout` | `0` | stop-signal 后的宽限——超时仍存活补发 SIGKILL；`0` = 不补发（纯单信号） |

### 降权运行（--uid/--gid）

`--uid <数字> --gid <数字>` 降权运行子进程（fork 后 exec 前生效）。**数字直通、成对强制**——只给一个拒绝启动；数字免 NSS 解析差异（极简容器无 /etc/passwd），名字解析请先 `id -u`/`id -g` 查好。

**身份环境联动改写**：降权后按目标 uid 的 passwd 条目自动改写子进程环境白名单中的 `HOME`/`USER`/`LOGNAME`（查不到条目则剔除三键让 shell 自默认）——「降权运行」连身份环境一起降：root 降权到 nobody 不会出现 `HOME=/root` 的权限错乱。附加组处理：root 启动时清空（最小权限——root 的附加组永非目标身份的组）；非 root 降回自身时保留自身附加组（无提权面）。

### 自动打开浏览器（--open）

`--open` 启动后以系统启动器打开分享链接（Linux `xdg-open` / macOS `open`）：`--writable` 开 rw 链接，否则开 ro 链接（含 token 免交互即打即用）。headless 环境（无 `DISPLAY`/`WAYLAND_DISPLAY`）stderr 提示后跳过、**不阻断启动**；`--socket` × `--open` 组合矛盾拒绝启动（unix socket 无 http URL 可开）。

### Docker（用户自建）

仓库根 `Dockerfile` 是参考镜像（FROM scratch + 静态二进制 + tini 作 PID 1 收割僵尸；tini 以 sha256 钉死拉取）——**不发布镜像**（与单二进制 scp 哲学一致，用户自建）。本机 docker 构建与 PID 1 收割行为已实测（正例零僵尸 + 无 init 负对照 5 僵尸的判别形态）。

```sh
# 构建前置：先产出静态二进制到仓库根（scratch 无动态库，wesh 必须纯静态）
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wesh ./cmd/wesh
docker build -t wesh .
docker run --rm wesh --version
```

**本镜像不含任何可执行命令**——scratch 零内容，`--` 后命令须来自 bind-mount（如 `-v /bin:/bin:ro -v /lib:/lib:ro -v /lib64:/lib64:ro`，实测形态）或 `FROM` 本镜像派生自建。PID 1 = tini：孤儿孙进程由 tini 收割；不加 `-g`——wesh 自管 stop-signal 进程组序列（tini 只向直接子进程 wesh 转发信号，正确的形态）。`--socket` 在容器内需配合 volume 把 socket 文件暴露给宿主反代；容器内 loopback 免凭据矩阵同样适用。

### systemd（deploy/wesh.service）

完整 unit 模板入库于 `deploy/wesh.service`（实机 systemctl 通道验证：255 复活语义/draining 窗口/停后不复活）。全配要点：

```ini
[Service]
EnvironmentFile=-/etc/wesh/credentials   # chmod 600，内容 WESH_CREDENTIAL=user:pass（- 前缀 = 缺席不拒）
ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml
Restart=on-failure
RestartSec=2
TimeoutStopSec=15s
LimitNOFILE=65536
```

**255 交互**（退出码语义见「优雅下线」节注记）：wesh 优雅关停（SIGTERM）与会话终结（`--once`/`--exit-when-empty`）均以 **255** 退出。`Restart=on-failure` 下自主终结（255 归为失败类）会自动重启——服务常驻形态的期望行为；而 `systemctl stop`/`restart` 发起的停止**永不触发重启**（systemd 知道是自己发起的停止——实测 systemd 239：stop 后 ActiveState=failed 但不复活，failed 态是 255 语义的正常纹理而非异常）。期望「会话完即停」改 `Restart=no`；希望把 255 视为正常关停可配置 `SuccessExitStatus=255`。`TimeoutStopSec=15s` 覆盖 1001 广播 + stall 客户端关闭内建 5s+5s 上界（不撞 90s 默认）。

### 优雅下线（1001）

`SIGTERM`/`SIGINT`（含 `systemctl stop`/`restart`）触发优雅下线序列：

1. 向全部在线客户端发送 **1001 Going Away**（前端显示「Server shutting down」终态面板，**不进入自动重连**——打的是一个正在关停的服务，重连无意义）；
2. 对子进程进程组执行 stop-signal 序列（`--stop-signal` → `--stop-timeout` 宽限 → SIGKILL）；
3. 进程退出。

**退出码 255 运维注记**：子进程被信号终结时 wesh 退出状态为 **255**（信号死亡按 -1 收口，Unix 进程退出状态截断为 255——与 `--once`/`--exit-when-empty` 收口路径同源）。systemd 部署若希望把 255 视为正常关停，可自行配置 `SuccessExitStatus=255`。

## 运维（Phase 8）

可观测性三面：`/healthz` 探活端点、`/metrics` Prometheus 文本指标、运行期事件 slog JSON 结构化审计日志。两个端点与主服务**同端口**且**根路径固定**——不受 `--base-path` 影响（探活器/采集器直连后端端口不经反代，路径恒定可写死进 k8s probe 与 Prometheus 静态配置；base-path 是浏览器用户面挂载形态，与运维面正交）。

### 健康检查（/healthz）

`GET /healthz` 返回 200 + 状态 JSON 四字段：

```json
{"status":"ok","clients":2,"max_clients":32,"session_active":true}
```

- `clients`/`max_clients`：当前 attach 数与容量上限；`session_active`：PTY 会话存活（子进程退出后翻 false）。
- **优雅关停进行中返回 503 + `"draining"`**：SIGTERM/SIGINT（含 `systemctl stop`/`restart`）触发优雅下线后、进程退出前，`status` 翻转为 `"draining"` 且状态码 503——反代/编排健康检查在关停窗口内不再向将死实例导新流。body 与 200 态同构四字段。
- body 恒为这四个粗粒度容量字段——无版本号、无客户端身份、无内部错误细节（version 只在需认证的 `/metrics` 的 `build_info`）；非 GET 方法 405 + `Allow: GET`。

**`/healthz` 免认证是整站 Basic 认证闸的唯一例外**：探活器（k8s liveness/反代健康检查）结构性携带不了凭据，且端点只暴露「进程活着」零敏感信息——两个前提同时成立才开此例外。该例外**不蔓延**：`/metrics` 与其余一切路径照常过认证闸，未来新端点不得以此为例外先例。

### 指标（/metrics）

`GET /metrics` 返回手写 Prometheus text 0.0.4 exposition（`Content-Type: text/plain; version=0.0.4; charset=utf-8`），17 条 series 全 gauge/counter，stdlib 零外部依赖：

| Series | 类型 | 语义 |
|--------|------|------|
| `wesh_clients_connected` | gauge | 当前 attach 的 WS 客户端数 |
| `wesh_clients_total` | counter | 进程启动以来累计 attach 数 |
| `wesh_clients_kicked_total` | counter | 1013 慢消费者踢出数 |
| `wesh_session_active` | gauge | PTY 会话存活（1/0） |
| `wesh_outbox_depth_bytes_max` | gauge | 每客户端 outbox 深度聚合 max（慢客户端检测信号） |
| `wesh_outbox_depth_bytes_sum` | gauge | outbox 深度聚合 sum |
| `wesh_pty_output_bytes_total` | counter | PTY 源读取字节（fan-out 源，单计） |
| `wesh_ws_sent_bytes_total` | counter | WS 下行字节（fan-out ×N 真实带宽；÷ pty_output 即吞吐放大比） |
| `wesh_ws_recv_bytes_total` | counter | WS 上行字节 |
| `wesh_auth_failed_total` | counter | 认证失败（HTTP 401 + WS Hello ticket 核销失败） |
| `wesh_auth_throttled_total` | counter | 节流闸拒绝（HTTP 429） |
| `wesh_input_rate_dropped_total` | counter | 每客户端输入限速丢弃的 INPUT 帧 |
| `wesh_input_queue_dropped_total` | counter | 有界会话输入队列丢弃的 INPUT 载荷 |
| `wesh_credit_gate_transitions_total` | counter | 全局信用门开闭次数 |
| `wesh_goroutines` | gauge | 当前 goroutine 数（goroutine 生命周期纪律的泄漏观测） |
| `wesh_mem_alloc_bytes` | gauge | 堆内存占用（`runtime.MemStats.Alloc`） |
| `wesh_build_info{version="..."}` | gauge(=1) | 构建元信息（version 单 label；发布构建 ldflags 注入，开发构建为 `dev`） |

- **认证闸跟随**：配置凭据后 `/metrics` 过同一 Basic 认证与节流闸（Prometheus `scrape_config` 原生支持 `basic_auth`，见下方配方）；`--no-auth`/loopback 裸跑模式直通——**注意此时 `/metrics` 对该端口的一切可达者暴露服务行为轮廓**（连接数/失败计数/字节量），非 loopback 部署务必保持认证开启。
- **label 零身份面**：全部 series 不带 remote/remote_user/client_id 等身份 label（日志红线在 metrics 面的镜像——隐私 + label 基数纪律）；per-IP/每连接明细查日志事件（`client_id` 关联），metrics 只看总量与聚合。非 GET 方法 405 + `Allow: GET`。

Prometheus 配方（`scrape_configs` 片段）：

```yaml
scrape_configs:
  - job_name: wesh
    static_configs:
      - targets: ['127.0.0.1:7681']   # 直连后端端口，不经反代（/metrics 根路径固定）
    basic_auth:
      username: alice                 # 与 wesh 凭据同组（建议与环境变量 WESH_CREDENTIAL 同源管理）
      password: pw-of-alice
```

**⚠️ 凭据错误会触发全站节流（429）导致采集目标自锁**：scrape 凭据配错/过期时，失败计入与浏览器登录同一 per-IP 指数退避计数器（1s 起翻倍、封顶 30s）——scrape 是高频自动客户端，失败计数涨得比人工快得多，采集器 IP 会长期锁在退避窗口内，表现为 target 持续 down。排查通道：`throttled` 日志事件（`remote` = 采集器 IP、`retry_after` 秒数，见「结构化日志」）；修正凭据后等待退避窗口过期（最长 30s）即恢复。

### 结构化日志（JSON 审计事件）

运行期事件恒为 stderr 单行 JSON（stdlib slog JSONHandler——**恒 JSON 恒 INFO，无 `--log-format`/`--log-level` 开关**，人读走 jq）：

```json
{"time":"2026-08-28T10:40:01.013456789+08:00","level":"INFO","msg":"event","event":"attach","remote":"198.51.100.7","remote_user":"alice","client_id":1,"mode":"rw"}
```

- **schema**：`msg` 恒 `"event"`，事件名走独立 `event` 字段，其余字段平铺（`time`/`level` 为 slog 默认键）——jq/Loki 按 `event=="x"` 直打字段索引。
- **client_id 关联**：同一连接的 `attach`/`detach` 携同一进程内单调递增序号（从 1 起，重启归零，无隐私面）——单连接生命周期可关联检索。

事件目录（审计三面：认证 / 连接 / 会话生命周期）：

| event | 字段 | 语义 |
|-------|------|------|
| `auth_failed` | remote, code=401/1008(, remote_user) | 认证失败（HTTP Basic / WS Hello ticket 核销）；**结构性不含用户名**——凭据任何形态永不入日志（remote_user 为配置 `--auth-header` 时的反代可信头值，非凭据） |
| `throttled` | remote, code=429, retry_after(, remote_user) | 节流闸拒绝（retry_after = `Retry-After` 响应头同值秒数，排查爆破节奏） |
| `attach` | remote, client_id, mode(, remote_user) | 客户端握手完成 |
| `detach` | remote, client_id, code, reason(, remote_user) | 连接断开单入口；reason = `normal`/`kick`/`pong_timeout`/`shutdown` 四值（kick/pong_timeout 不再单独打行，计数走 metrics） |
| `session_start` | pid | PTY 子进程启动 |
| `session_end` | exit_code, duration_seconds(, signal) | 子进程退出——「活多久、怎么死的」单事件齐备（信号死亡 exit_code=-1 且出 signal 键，与 EXIT 帧同源） |
| `shutdown` | — | SIGTERM/SIGINT 优雅下线序列开始 |
| `exit_when_empty` 族 | remote, code=1000(, remote_user) | `--exit-when-empty` 触发（`exit_when_empty`）/ 宽限开始（`_wait`）/ 宽限取消（`_cancel`） |

另：协议守卫事件同 schema 同出口（`message_too_big`/`max_clients`/`hello_timeout`/`subprotocol_required`/`version_mismatch` 等，code 携 HTTP 状态码或 WS 关闭码）。

journald + jq 检索示例（systemd 部署下 stderr 自动进 journal）。⚠️ 合流陷阱：systemd 默认 `StandardOutput=journal` 会把 stdout 启动横幅（人读文本——分流决策见本节末「红线重申」段）与 stderr JSON 事件合流入同一 journal，jq 遇非 JSON 行即中止整条管道；故示例统一在 journalctl 与 jq 之间插入 `| grep '^\{'` 预滤 JSON 事件行——与自动化测试 parseEvents 的滤行约定（滤 `{` 起始行）同款：

```bash
# 认证失败审计（event 字段直打索引）
journalctl -u wesh -o cat | grep '^\{' | jq -c 'select(.event=="auth_failed")'
# 单连接生命周期关联（attach → detach）
journalctl -u wesh -o cat | grep '^\{' | jq -c 'select(.client_id==7)'
```

**红线重申**：凭据、ticket、share token、Authorization 头任何形态（含 base64、含用户名）**永不进入日志**；用户可控字段（`remote` 的 XFF 链首、`remote_user` 头值）经 C0/C1/DEL 控制字符剥离 + 128 字符截断后才入日志（JSON 转义只覆盖 C0，C1 如 NEL 原样穿透——清洗是防伪造日志行的唯一防线）。启动行/分享链接行保持人读文本（stdout）不 JSON 化——operator 交互输出与机器审计事件分流，既有启动行解析（含 UAT 与部署脚本）零破坏。

## 测试

```sh
go test -race -count=1 ./...
```

CI 为双平台矩阵（ubuntu + macos，含 macOS kqueue 收割验证）加独立 web 构建 job。注意 `-race` 需要 CGO，测试环境不要设 `CGO_ENABLED=0`。
