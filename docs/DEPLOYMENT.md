<!-- generated-by: gsd-doc-writer -->

# wesh 部署指南

wesh 是单二进制程序（前端页面经 `go:embed` 内嵌），部署哲学与 scp 一致：**一个文件拷过去即用**。本文覆盖部署目标与形态、构建/发布流水线、生产环境设置、回滚与监控。配置项（flag/TOML/环境变量）的完整语义见 [CONFIGURATION.md](CONFIGURATION.md)。

## 部署目标

| 形态 | 配置文件 | 说明 |
|------|----------|------|
| 单二进制直跑 | — | 预编译产物解压即用（scp 部署） |
| systemd 常驻服务 | `deploy/wesh.service` | 生产推荐：凭据 EnvironmentFile 通道 + 崩溃自愈 |
| Docker 容器 | `Dockerfile` | 参考镜像（scratch + tini），用户自建——**不发布镜像** |
| 反向代理之后 | —（配方见本文「反向代理」节） | nginx/Caddy 已实证；Cloudflare 未实测 |
| UNIX socket + 反代 | — | 生产推荐形态之一：文件系统权限即认证边界 |

平台边界：linux/darwin × amd64/arm64 四平台（Windows 不在支持范围——PTY 层仅 linux/darwin 构建标签）。

### 单二进制直跑

发布物为四平台 tar.gz（每个含 `wesh` + `LICENSE` + `README.md` 三件套），从 [GitHub Releases](https://github.com/sworda/wesh/releases) 下载：

```sh
curl -LO https://github.com/sworda/wesh/releases/download/v1.0.0/wesh_v1.0.0_linux_amd64.tar.gz
tar xzf wesh_v1.0.0_linux_amd64.tar.gz
sha256sum -c checksums.txt --ignore-missing   # 完整性核对（只验本机已下载产物）
./wesh --version   # 核对版本（发布构建经 ldflags 注入）
```

### systemd（deploy/wesh.service）

完整 unit 模板入库于 `deploy/wesh.service`（实机 systemctl 通道验证：255 复活语义/draining 窗口/停后不复活）：

```sh
cp deploy/wesh.service /etc/systemd/system/ && systemctl daemon-reload && systemctl enable --now wesh
```

模板要点（`[Service]` 段）：

```ini
EnvironmentFile=-/etc/wesh/credentials   # chmod 600，内容 WESH_CREDENTIAL=user:pass（- 前缀 = 缺席不拒）
ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml
Restart=on-failure
RestartSec=2
TimeoutStopSec=15s      # 覆盖 1001 广播 + stall 客户端关闭内建 5s+5s 上界（不撞 90s 默认）
LimitNOFILE=65536
```

**退出码 255 与 Restart 交互**：wesh 优雅关停（SIGTERM）与会话终结（`--once`/`--exit-when-empty`）均以 255 退出（`os.Exit(-1)` 的 Unix 截断，语义详见 [CONFIGURATION.md](CONFIGURATION.md) 退出码表）。`Restart=on-failure` 把非零分类为 failure，故自主终结也会重启——服务常驻形态的期望行为（崩溃与自终结均自愈）；而 `systemctl stop`/`restart` 发起的停止**永不触发重启**（systemd 知道是自己发起的停止——实测 systemd 239：stop 后 ActiveState=failed 但不复活，failed 态是 255 语义的正常纹理而非异常）。

- 期望「会话完即停」→ 改 `Restart=no`。
- 希望把 255 视为正常关停 → 配置 `SuccessExitStatus=255`。

**凭据通道纪律**：unit 文件 world-readable，凭据绝不写进 unit 本体——一律走 `EnvironmentFile=`（chmod 600）。

**UNIX socket 形态的 unit 变体**（socket + 反代生产推荐）：

```ini
[Service]
RuntimeDirectory=wesh
EnvironmentFile=/etc/wesh/credentials
ExecStart=/usr/local/bin/wesh --socket /run/wesh/wesh.sock --socket-owner www-data:www-data -- bash
```

### Docker（用户自建）

仓库根 `Dockerfile` 是参考镜像：`FROM scratch` + 静态二进制 + tini 作 PID 1（tini 以 sha256 钉死拉取）。**不发布镜像**——与单二进制 scp 哲学一致，用户自建。本机 docker 构建与 PID 1 收割行为已实测（正例零僵尸 + 无 init 负对照 5 僵尸的判别形态）。

```sh
# 构建前置：先产出静态二进制到仓库根（scratch 无动态库，wesh 必须纯静态）
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o wesh ./cmd/wesh
docker build -t wesh .
docker run --rm wesh --version
```

要点：

- **本镜像不含任何可执行命令**——scratch 零内容，`--` 后命令须来自 bind-mount（如 `-v /bin:/bin:ro -v /lib:/lib:ro -v /lib64:/lib64:ro`，实测形态）或 `FROM` 本镜像派生自建。
- **PID 1 = tini，不加 `-g`**：tini 只向直接子进程（wesh）转发信号——wesh 自管 stop-signal 进程组序列，`-g` 会双重信号；孤儿孙进程由 tini 收割。
- **flag 直传**：`ENTRYPOINT ["/tini", "--", "/wesh"]`，flag 跟在镜像命令后。
- `--socket` 在容器内需配合 volume 把 socket 文件暴露给宿主反代；容器内 loopback 免凭据矩阵同样适用。
- arm64 构建：`docker build --build-arg TARGETARCH=arm64 --build-arg TINI_SHA256=eae1d3aa... .`

## 反向代理

### nginx（已实证）

子路径挂载配方（`--base-path /wesh`）——注意 `Host` 头与 WS upgrade 两个关键点：

```nginx
# 把 Connection 头映射为 upgrade/close（WS 升级必需）
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    # 精确块：裸 /wesh（无尾斜杠）显式 308 规范化到 /wesh/——308 保方法 +
    # 行为与 handler 形态无关（proxy_pass 系 handler 的自动 301 补斜杠是特例）
    location = /wesh { return 308 /wesh/; }

    location /wesh/ {
        proxy_pass http://127.0.0.1:7681;
        proxy_http_version 1.1;
        # Host 必须原样转发：nginx 默认转发 $proxy_host（127.0.0.1:端口），与浏览器
        # Origin 不同源会被 wesh WS 同源校验 403；$host 剥端口在 Origin 含非默认端口时
        # 仍不匹配——必须 $http_host（已全链实证）
        proxy_set_header Host $http_host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 3600s;
    }
}
```

**`proxy_read_timeout` 必须大于 `--ping-interval`（默认 `5s`）**：反代空闲超时看应用层流量——WS 建立后若无数据往来，超时到期反代主动断连。wesh 服务端每个 ping 间隔发一帧 WS ping（应用层流量），故反代超时大于 ping 间隔即不会误断空闲连接；`3600s` 是充裕值。`--ping-interval 0` 禁用保活时，空闲连接存活性完全取决于反代超时。

### Caddy（已实证）

已全链实证（2026-08-30，Caddy v2.11.4；Linux 协议层七断言 + Windows 浏览器双机全链）。根路径反代只需一条指令：

```caddyfile
wesh.example.com {
    reverse_proxy 127.0.0.1:7681
}
```

与 nginx 的三个关键差异（两平台默认语义相反，**配方互抄必错**）：

- **Host 默认原样透传**——wesh Origin 同源校验天然通过，**不需要任何 Host 配置行**（nginx 必须显式 `proxy_set_header Host $http_host`）。
- **WS upgrade 内建自动处理**——零 upgrade 配置行（nginx 须 `Upgrade`/`Connection` 映射）。
- **站点地址语义相反**：Caddyfile 站点地址写 `http://0.0.0.0:PORT` 是**字面 Host 匹配**（仅 Host: 0.0.0.0 的请求命中），不是 nginx 的「绑定全网卡」监听语义——LAN 监听站点地址须写**裸 `:PORT`**；域名形态不受影响。

XFF 默认添加（`--auth-header` 可选消费）。**空闲超时**：Caddy 对 hijack 后的 WS 连接无默认 idle 超时——wesh 默认 `--ping-interval 5s` 远小于任何中间盒 idle 阈值，无需额外配置。

### Cloudflare（未实测）

**本节按 Cloudflare 官方文档书写，未经实测**（SaaS 反代无本机复现条件；nginx/Caddy 均经全链实证）。要点：

- **DNS 橙云代理**：wesh 域名记录开代理（橙云）即经 CF 边缘；WebSockets 默认开启（Network 面板）。
- **空闲超时**：社区共识约 **~100s 无流量关连**（未能从官方文档直取，为多源社区共识）<!-- VERIFY: Cloudflare 对无流量 WebSocket 连接的空闲超时约 100s（社区共识，非官方文档直取） -->——wesh 默认 `--ping-interval 5s` 应用层 ping 使连接恒有流量，**默认即安全**；`--ping-interval 0` 禁用保活时空闲连接将在 ~100s 被 CF 关闭（前端 1006 自动重连可恢复）。
- **TLS**：CF 边缘终止；源站建议 Full (strict)（wesh 配 `--tls-cert`/`--tls-key` 真实证书），或源站仅 loopback 监听 + CF 回源（源站不直接暴露）。
- **Host 默认保持**（同 Caddy）；`--base-path` 子路径挂载在 CF 后照常可用。
- **`/s/{token}/` 对 CF 明文可见**：分享 token 经 CF 边缘与访问日志（CF 侧日志不在你的掌控内，转发分享链接前请知悉）。

### 反代身份透传与 per-IP 限流键

`--auth-header X-Remote-User` 给定即「信任反代」总开关（**仅反代后部署**，不得直接暴露——直连客户端可伪造该头污染审计归因；伪造头不能绕过认证，身份/ticket/token 三通道语义不变）：

- **审计归因**：反代（authelia/oauth2-proxy 等 SSO）注入的用户名经清洗（控制字符剥离、128 字符截断）记录进服务端日志 `remote_user` 字段。
- **XFF 链首换入 per-IP 计数键**：配置后 `X-Forwarded-For` 链首 IP 同时作为节流与 per-IP 半开连接上限的计数键（`throttle`/`halfOpen` 共用同一 clientIP 键，半开上限默认 8、超限 429）——反代后 per-IP 计数不再聚合为代理 IP。
- **未配置时 XFF 完全忽略**：反代之后不配 `--auth-header`，节流与半开限流对全体客户端聚合到代理 IP 单键上——高并发正常用户可能被误伤（429）。

## 构建与发布流水线

### CI（.github/workflows/ci.yml）

触发：push + pull_request。三个 job：

| Job | 内容 |
|-----|------|
| `go` | ubuntu/macos 双平台矩阵（darwin leg 兼担 kqueue 运行时裁决）：`go vet ./...` + `go test -race -count=1 -v ./...`（**不设 CGO_ENABLED**——`-race` 需要 cgo） |
| `web` | ubuntu：`pnpm -C web install --frozen-lockfile` + `pnpm -C web build`（tsc 类型检查 + vite 构建一体） |
| `fuzz` | ubuntu：`FuzzDecodeHello` 与 `FuzzDecodeFileConfig` 各 60s 短跑回归门（两目标两次独立调用） |

### 发布（scripts/release.sh → tag push → release.yml）

**发布流程 = `scripts/release.sh`**（发布前跑一次，脚本即发布文档的可执行形态）：

```sh
./scripts/release.sh --dry-run v1.0.0   # 干跑：只跑前置校验四闸，打印步骤清单不执行
./scripts/release.sh v1.0.0             # 真实发布：前置校验 → 全量测试 → 前端构建
                                        #   → 长 fuzz ×2（每目标 10 分钟）→ 负载矩阵
                                        #   → 确认闸 → tag push
```

前置校验四闸：tag 形态（`vX.Y.Z`）/ tag 不存在 / 工作树干净 / 与远端同步（无网络或无上游时降级为跳过提示）。fuzz 崩溃即中止——崩溃语料自动落 `testdata/fuzz/`，修复后重跑。确认闸回显将创建的 tag 与最近 5 条提交，应答 `yes` 才落 tag。

**tag push 是唯一发布触发**（`v*` tag），此后 `.github/workflows/release.yml` 接管：

1. `pnpm -C web install --frozen-lockfile` + `pnpm -C web build`（**显式编排 pnpm build 先于 goreleaser**，不用 before hooks；dist 真实产物经 `go:embed` 进入发布二进制）
2. goreleaser `release --clean`（`.goreleaser.yml`）：`CGO_ENABLED=0` 全静态 + `-trimpath` + `-X main.version={{.Version}}` 注入 + `mod_timestamp` 可复现构建；linux/darwin × amd64/arm64 四平台
3. 产物：`wesh_v<TAG>_<os>_<arch>.tar.gz` ×4 + `checksums.txt`（供应链文件仅 checksums，无 cosign 签名/SBOM——个人运维工具威胁模型裁决）

### 本地构建

构建顺序是硬依赖：**前端构建必须先于 `go build`**（`go:embed all:dist` 编译期要求 `web/dist/` 存在）：

```sh
pnpm -C web install && pnpm -C web build && go build -o wesh ./cmd/wesh
```

仓库提交 `web/dist/index.html` 构建产物（真实终端页；裸 clone 可直接 `go build`/`go test ./...`）；修改 `web/` 前端源码后必须先重新 `pnpm -C web build` 再 `go build`，否则二进制内嵌的仍是旧产物。`.gz` 预压产物不入库，发布构建在 CI 侧完成。

## 生产环境设置

配置项的完整语义（全部 flag、TOML 29 键、`WESH_CREDENTIAL` 环境变量、启动校验矩阵、退出码表）见 [CONFIGURATION.md](CONFIGURATION.md)。生产部署典型布局：

```
/etc/wesh/
├── wesh.toml       # TOML 配置（--config 显式指定；长期运行参数）
├── credentials     # chmod 600，内容 WESH_CREDENTIAL=user:pass（systemd EnvironmentFile）
├── cert.pem        # TLS 证书（--tls-cert）
└── key.pem         # TLS 私钥（--tls-key）
```

环境变量只有一个配置型变量 `WESH_CREDENTIAL`——生产凭据**首选此通道**（systemd `EnvironmentFile=` 600），不写入配置文件、不走 flag（flag 值对同机用户可见于 `ps`）。优先级链：CLI flag > `WESH_CREDENTIAL` env > TOML 配置文件 > 内置默认。

生产检查清单：

- [ ] 非 loopback 监听必须配置凭据（否则拒绝启动；`--no-auth` 是显式逃生门并打警告）
- [ ] 非 loopback + 凭据必须配 TLS，或显式 `--insecure-http`（TLS 终止型反代之后的典型合法场景）
- [ ] 反代部署：`proxy_read_timeout` > `--ping-interval`；需要 per-IP 限流语义时配 `--auth-header`
- [ ] 子路径挂载：`--base-path /wesh`（`/` 开头、无尾斜杠；非法值拒绝启动）
- [ ] 降权运行：`--uid`/`--gid` 数字成对给出（容器内无 NSS 时名字解析不可用，先 `id -u`/`id -g` 查好）
- [ ] `--once`/`--exit-when-empty` 会话终结形态与 systemd `Restart=on-failure` 组合会循环重启——常驻服务确认这是期望行为，否则 `Restart=no`
- [ ] TLS 部署注意 HSTS 粘性（`max-age` 两年——访问过 TLS 实例的浏览器在过期前对同一 host:port 强制 HTTPS，改回 HTTP 需清浏览器 HSTS 缓存或换端口）

## 回滚

wesh 无状态持久层（无数据库、无迁移），回滚 = **换回旧二进制重启**：

1. 从 [Releases](https://github.com/sworda/wesh/releases) 下载前一版本 tar.gz，`sha256sum -c checksums.txt --ignore-missing` 核对后替换 `/usr/local/bin/wesh`；
2. `systemctl restart wesh`——重启即完成回滚（前端随 `go:embed` 内嵌于二进制，前后端同版本无分别回滚问题）；
3. **重启副作用**：ro/rw 分享 token 每次启动重新随机生成——回滚（或任何重启）自动吊销全部旧分享链接，需重新分发。

systemd 语义补充：

- `systemctl stop`/`restart` 发起的停止永不触发 `Restart=` 复活（systemd 知道是自己发起的停止）——回滚操作不会与自愈重启竞争。
- 自身崩溃（非零退出）在 `Restart=on-failure` 下自动拉起（RestartSec=2s）——二进制损坏/缺失导致的连续崩溃会进 systemd 启动限速，`systemctl status wesh` 可见 failed 态。

## 监控

可观测性三面内建于二进制：`/healthz` 探活、`/metrics` Prometheus 指标、stderr JSON 结构化审计日志。两个端点与主服务**同端口**且**根路径固定**——不受 `--base-path` 影响（探活器/采集器直连后端端口不经反代，路径恒定可写死进 k8s probe 与 Prometheus 静态配置）。

### 健康检查（/healthz）

`GET /healthz` 返回 200 + 状态 JSON：

```json
{"status":"ok","clients":2,"max_clients":32,"session_active":true}
```

- `session_active`：PTY 会话存活（子进程退出后翻 false）；`clients`/`max_clients`：当前 attach 数与容量。
- **优雅关停进行中返回 503 + `"draining"`**——反代/编排健康检查在关停窗口内不再向将死实例导新流。
- `/healthz` 免认证是整站 Basic 认证闸的唯一例外（探活器结构性携带不了凭据，且端点零敏感信息）；`/metrics` 与其余路径照常过认证闸。

### 指标（/metrics）

`GET /metrics` 返回 Prometheus text 0.0.4 exposition（`Content-Type: text/plain; version=0.0.4; charset=utf-8`），17 条 series 全 gauge/counter，stdlib 零外部依赖：

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
| `wesh_goroutines` | gauge | 当前 goroutine 数（泄漏观测） |
| `wesh_mem_alloc_bytes` | gauge | 堆内存占用 |
| `wesh_build_info{version="..."}` | gauge(=1) | 构建元信息（发布构建注入，开发构建为 `dev`） |

全部 series 不带 remote/remote_user/client_id 等身份 label（隐私 + label 基数纪律）——per-IP/每连接明细查日志事件，metrics 只看总量与聚合。配置凭据后 `/metrics` 过同一 Basic 认证闸，Prometheus 原生支持：

```yaml
scrape_configs:
  - job_name: wesh
    static_configs:
      - targets: ['127.0.0.1:7681']   # 直连后端端口，不经反代
    basic_auth:
      username: alice                 # 与 wesh 凭据同组（建议与 WESH_CREDENTIAL 同源管理）
      password: pw-of-alice
```

**⚠️ 凭据错误会触发全站节流（429）导致采集目标自锁**：scrape 凭据配错时失败计入与浏览器登录同一 per-IP 指数退避计数器（1s 起翻倍、封顶 30s）——scrape 是高频自动客户端，采集器 IP 会长期锁在退避窗口内，表现为 target 持续 down。修正凭据后等待退避窗口过期（最长 30s）即恢复。

### JSON 审计日志

运行期事件恒为 stderr 单行 JSON（stdlib slog JSONHandler，恒 JSON 恒 INFO，无开关——人读走 jq）。systemd 部署下 stderr 自动进 journal：

```bash
# 认证失败审计
journalctl -u wesh -o cat | grep '^\{' | jq -c 'select(.event=="auth_failed")'
# 单连接生命周期关联（attach → detach 同 client_id）
journalctl -u wesh -o cat | grep '^\{' | jq -c 'select(.client_id==7)'
```

（`grep '^\{'` 预滤 JSON 事件行——systemd 默认把 stdout 启动横幅与 stderr JSON 合流入同一 journal，jq 遇非 JSON 行即中止整条管道。）

事件目录覆盖认证（`auth_failed`/`throttled`）、连接（`attach`/`detach`）、会话生命周期（`session_start`/`session_end`/`shutdown`/`exit_when_empty` 族）三面；凭据、ticket、share token 任何形态永不入日志。

### 优雅下线

`SIGTERM`/`SIGINT`（含 `systemctl stop`/`restart`）触发优雅下线序列：向全部在线客户端广播 **1001 Going Away**（前端显示终态面板，不进入自动重连）→ 对子进程进程组执行 stop-signal 序列 → 进程退出（255）。`TimeoutStopSec=15s` 覆盖该序列上界。
