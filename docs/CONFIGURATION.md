<!-- generated-by: gsd-doc-writer -->

# wesh 配置参考

wesh 是单二进制 Go 程序，无隐式配置文件搜索、无注册表——全部配置经 **CLI flag、环境变量、TOML 配置文件**三条通道给出，按固定优先级合并：

```
CLI flag > 环境变量（WESH_CREDENTIAL）> 配置文件（--config）> 内置默认值
```

完整 flag 清单可随时以 `wesh --help` 查看；本文以 `cmd/wesh/main.go`（`parseArgs`/`validateStartup`）与 `cmd/wesh/config.go`（`fileConfig`）为事实源。

## 环境变量

wesh 只消费一个配置型环境变量；另有两个只读探测变量影响 `--open` 行为：

| 变量 | 必填 | 默认 | 说明 |
|------|------|------|------|
| `WESH_CREDENTIAL` | 可选 | 空（不设置） | 单组 Basic 认证凭据 `user:pass`。仅在 `--credential` flag 未给出任何凭据时生效（flag 非空时 env 整体忽略）；非空时配置文件 `credential` 列表同样不应用。畸形值（缺 `:`）启动报错。**生产推荐通道**——flag 值对同机用户可见（`ps`），env 无此暴露面 |
| `DISPLAY` | — | — | 只读探测：Linux 下与 `WAYLAND_DISPLAY` 均为空时 `--open` 判定 headless，打提示后跳过浏览器启动（不阻断） |
| `WAYLAND_DISPLAY` | — | — | 同上，Wayland 会话检测 |

凭据 env 的典型注入通道是 systemd `EnvironmentFile=`（文件 `chmod 600`），见仓库模板 `deploy/wesh.service`：

```ini
EnvironmentFile=-/etc/wesh/credentials   # 内容为 WESH_CREDENTIAL=user:pass；- 前缀 = 文件缺席不拒绝启动
```

## 配置文件格式（TOML）

经 `--config /path/to/wesh.toml` **显式指定**——零隐式默认路径搜索，裸 `wesh -- bash` 的行为与无配置文件时逐字节一致。

形状为平铺 `key = value`（拒绝分组 section），**键名 = flag 名**（连字符形态）。共 29 键：27 个长期运行 flag 同名键 + `command` exec 数组 + `index-max-size` 纯配置键。

```toml
# /etc/wesh/wesh.toml —— 含 credential 键时建议 chmod 600
bind = "127.0.0.1"
port = 7681
credential = ["alice:pw-of-alice"]   # 可重复 flag ↔ TOML 数组
base-path = "/wesh"
index = "/srv/wesh/index.html"       # 自定义首页（--index 同名键）
index-max-size = 33554432            # 纯配置键：整数字节，默认 16MiB
max-clients = 16
ping-interval = "5s"                 # duration 键为字符串形态
exit-when-empty = "30s"              # "true"/"0"/"30s" 与 CLI 三形态同语义
command = ["bash", "-l"]             # exec 数组；CLI `--` 后 argv 非空则覆盖
```

### 全部 29 个配置键

| 键 | TOML 类型 | 默认值 | 说明 |
|----|-----------|--------|------|
| `port` | 整数 | `7681` | 监听端口；`0` = 随机端口，启动打印实际端口 |
| `bind` | 字符串 | `"0.0.0.0"` | 监听地址 |
| `writable` | 布尔 | `false` | 客户端输入总闸（默认只读） |
| `write-policy` | 字符串 | `"owner"` | `owner`（首写者独占，断线递补）或 `all`（全员可写）；仅 `writable` 开启时有意义 |
| `max-clients` | 整数 | `32` | 最大并发 attach 客户端数；满员新客户端收到 503 |
| `once` | 布尔 | `false` | 只接受一个客户端并在其断开后退出（≡ `max-clients=1` + `exit-when-empty` 立即退出） |
| `exit-when-empty` | 字符串 | 不开启 | 所有客户端断开后退出：`"true"`/`"0"` = 立即；`"30s"` = 重连宽限 |
| `ping-interval` | duration 串 | `"5s"` | WS ping 保活间隔（防反代空闲超时断连）；`"0"` = 禁用 |
| `credential` | 字符串数组 | — | Basic 认证凭据 `user:pass`，多组按人撤销 |
| `origin` | 字符串数组 | — | 允许的 Origin `scheme://host[:port]` 白名单 |
| `client-option` | 字符串数组 | — | 客户端偏好 `key=value`（白名单键，值为 JSON） |
| `tls-cert` | 字符串 | — | TLS 证书文件路径（与 `tls-key` 成对才启用 TLS） |
| `tls-key` | 字符串 | — | TLS 私钥文件路径（与 `tls-cert` 成对） |
| `osc52` | 布尔 | `false` | OSC52 剪贴板写入开关（只写不读） |
| `socket` | 字符串 | — | UNIX socket 监听路径（与显式 `port`/`bind` 互斥） |
| `socket-mode` | 八进制串 | `"0660"` | socket 权限位（字符串形态，如 `"0660"`）；仅随 `socket` 有意义 |
| `socket-owner` | 字符串 | — | socket 属主 `user[:group]`；仅随 `socket` 有意义 |
| `base-path` | 字符串 | — | 反代子路径前缀（如 `/wesh`；`/` 开头、无尾斜杠） |
| `index` | 字符串 | — | 自定义首页 HTML 文件路径（整页替换内建页） |
| `auth-header` | 字符串 | — | 可信反代用户头名（如 `X-Remote-User`）；仅审计归因，无认证效力 |
| `cwd` | 字符串 | 继承 | 子进程工作目录 |
| `term` | 字符串 | `xterm-256color` 语义 | 子进程 TERM；空串按未配置处理 |
| `stop-signal` | 字符串 | `"HUP"` | 关停时发子进程进程组的信号：`HUP`/`TERM`/`INT`/`KILL` |
| `stop-timeout` | duration 串 | `"0"` | stop-signal 后补发 SIGKILL 的宽限（`0` = 不补发） |
| `uid` | 整数 | `-1`（不降权） | 降权目标 uid（与 `gid` 成对强制） |
| `gid` | 整数 | `-1`（不降权） | 降权目标 gid（与 `uid` 成对强制） |
| `open` | 布尔 | `false` | 启动后自动打开分享链接（headless 提示后跳过） |
| `command` | 字符串数组 | — | **纯配置键**：子命令 exec 数组；CLI `--` 后 argv 非空则覆盖；空数组等价缺席 |
| `index-max-size` | 整数 | `16777216`（16MiB） | **纯配置键**：自定义首页读入上限（字节）；无对应 CLI flag |

### 仅 CLI flag（不入配置文件）

五个逃生门/信息键只能经 CLI 显式给出，写入配置文件按**未知键**拒绝启动（exit 2）——逃生门必须显式说出口，写在配置文件里等于没说：

| Flag | 默认值 | 说明 |
|------|--------|------|
| `--no-auth` | `false` | 逃生门：允许无凭据监听非 loopback 地址（显式声明「我知道我在裸奔」） |
| `--insecure-http` | `false` | 逃生门：允许非 loopback 明文 HTTP 携带凭据（典型场景：TLS 终止型反代之后） |
| `--config` | — | 指定 TOML 配置文件路径（见本文各节；支持 `--config=<path>` 与 `--config <path>` 两形态） |
| `--version` | — | 打印版本并退出（版本由发布构建注入，开发构建为 `dev`） |
| `--help` | — | 打印用法 |

### 严格模式与加载语义

- **fail-fast**：文件不存在、TOML 解析失败、未知键均以 exit 2 拒绝启动。
- **错误文案值剥离**：错误只含「类别 + 键名 + 行号」（如 `invalid config file /etc/wesh/wesh.toml: unknown keys (no-auth)`），**绝不回显配置值**——凭据不会落 stderr/journald。
- **列表替换语义**：`credential`/`origin`/`client-option` 三列表键——CLI flag 给出则整个列表替换配置值（配置不应用、不校验）；CLI 未给且配置键存在时逐项经与 CLI 相同的校验。
- **配置键显式位**：`port`/`bind`/`socket-mode`/`socket-owner`/`write-policy` 在配置文件中出现即视为「显式设置」，与 CLI 同档参与互斥/组合校验（如配置同时写 `socket` + `port` 会拒绝启动）。
- **文件内自相矛盾拒绝**：同一文件内 `once = true` 与 `max-clients ≠ 1` 或 `exit-when-empty` 宽限 ≠ 0 同给即拒。
- **权限警告**：文件含 `credential` 键且权限非 600/400 时 stderr 警告放行（不阻断）；生产凭据首选 `WESH_CREDENTIAL` env，不写入配置文件。

## 必填与可选设置

wesh 采取「显式哲学」：绝大多数键可选且有默认值，以下情况**启动即失败**（配置校验错误 exit 2）：

| 校验 | 失败条件 | 错误信息（类别） |
|------|----------|------------------|
| 子命令必填 | CLI `--` 后 argv 为空且配置 `command` 键缺席/空数组 | `missing command` |
| 非 loopback 必须有凭据 | bind 非 loopback、无任何凭据、未给 `--no-auth` | refusing to listen on non-loopback address without credentials |
| 非 loopback 明文闸 | 非 loopback + 凭据 + 无 TLS，未给 `--insecure-http` | refusing to serve credentials over plaintext HTTP |
| TLS 成对 | `--tls-cert`/`--tls-key` 只给其一 | must give both --tls-cert and --tls-key |
| 降权成对 | `--uid`/`--gid` 只给其一 | --uid and --gid must be given together |
| socket 互斥 | `--socket` 与显式 `--port`/`--bind` 同给 | --socket conflicts with --port/--bind |
| socket 族单给 | `--socket-mode`/`--socket-owner` 未随 `--socket` 给出 | --socket-mode/--socket-owner require --socket |
| `--open` × `--socket` | 同给（unix socket 无 http URL 可开） | --open conflicts with --socket |
| 写策略组合 | 显式 `--write-policy` 却未开 `--writable` | --write-policy is set but --writable is not |
| `--once` 矛盾值 | `--once` 与显式 `--max-clients ≠ 1` 或非零宽限 `--exit-when-empty` 同给 | --once conflicts with … |
| `--max-clients` 正数 | ≤ 0 | --max-clients must be positive |
| `--index` 预检 | 文件不存在 / 非常规文件（目录、设备、socket） | invalid --index … |
| `index-max-size` 值域 | ≤ 0 或 > 2GiB | invalid index-max-size … |
| `--cwd` 预检 | 目录不存在 | invalid --cwd … |
| 值域/枚举 | `--write-policy`/`--stop-signal` 枚举、`--socket-mode` 八进制、`--uid`/`--gid` 0..4294967295、duration 键非负等 | invalid …（值可回显，非敏感） |

**放行但警告**（stderr 醒目提示，不阻断启动）：

- `--no-auth` 非 loopback：任何人可达该端口即得终端。
- `--insecure-http` 非 loopback：凭据经明文 HTTP 传输（TLS 终止型反代之后的典型合法场景）。
- `--no-auth` + `--auth-header` 非 loopback：直连客户端可伪造审计头。

**退出码约定**（供 systemd `Restart=` 与脚本编排参考）：

| 退出码 | 语义 |
|--------|------|
| 子进程退出码 N | 子进程自然退出（如用户键入 `exit`）→ wesh 原样传递子进程真实退出码（`exit 42` → 42，正常 `exit` → 0） |
| `0` | `--version`/`--help` 信息路径 |
| `2` | 配置/参数校验失败（parse + 启动校验矩阵 + 配置文件加载 + 自定义首页读入） |
| `1` | 运行时 I/O 错误（TLS 证书加载、listen 失败、serve 失败——失败路径回滚已 spawn 的子进程，不留孤儿） |
| `255` | 子进程信号死亡（`ExitCode=-1`），以及 `--once`/`--exit-when-empty` 自主终结与 SIGTERM 优雅下线（后两者经 stop-signal 序列使子进程信号死亡，同路收口）——`os.Exit(-1)` 被 Unix 截断为退出状态 255（systemd `Restart=on-failure` 视其为失败并自愈重启） |

## 默认值

未配置时的内置默认值（`cmd/wesh/main.go` `parseArgs` 铺底）：

| 设置 | 默认值 | 说明 |
|------|--------|------|
| `port` | `7681` | `0` = 随机端口 |
| `bind` | `0.0.0.0` | 全网卡（非 loopback——触发凭据/明文校验矩阵） |
| `writable` | `false` | 只读会话 |
| `write-policy` | `owner` | 首写者独占 + 按序递补 |
| `max-clients` | `32` | 满员 503 |
| `ping-interval` | `5s` | `0` = 禁用保活 |
| `osc52` | `false` | 剪贴板写默认关 |
| `socket-mode` | `0660` | listen 后显式 Chmod 达成，不随 umask 漂移 |
| `stop-signal` | `HUP` | 纯单信号关停 |
| `stop-timeout` | `0` | 不补发 SIGKILL |
| `term` | `xterm-256color` | 空串按未配置处理 |
| `uid`/`gid` | `-1` | 不降权（`-1` 哨兵；`0` 是 root 合法值） |
| `index-max-size` | `16777216`（16MiB） | 自定义首页读入硬顶（上限上界 2GiB） |
| `credential`/`origin`/`client-option`/`command` | 空 | 未配置 |
| `exit-when-empty` | 不开启 | 无客户端时子进程继续运行 |
| `tls-cert`/`tls-key`/`socket`/`socket-owner`/`base-path`/`auth-header`/`cwd`/`index` | 空串 | 未配置 |

TLS 未配置（`--tls-cert`/`--tls-key` 成对给出才启用）时为明文 HTTP——非 loopback + 凭据场景会被启动校验矩阵拒绝，除非给 `--insecure-http`。

## 按环境覆盖

wesh 是单二进制，**没有 `.env.development`/`.env.production` 之类的多环境文件机制**——环境差异化由「不同的 `--config` 路径 + env 变量注入」组合承载：

```bash
# 开发：loopback + 随机端口 + 默认值即可
wesh -- bash

# 生产（systemd）：TOML 承载长期运行参数 + EnvironmentFile 承载凭据
ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml
EnvironmentFile=-/etc/wesh/credentials   # WESH_CREDENTIAL=user:pass，chmod 600
```

各部署形态的配置承载方式：

| 形态 | 配置通道 | 参考文件 |
|------|----------|----------|
| systemd | `--config` TOML + `EnvironmentFile=`（凭据） | `deploy/wesh.service` |
| Docker | 命令行 flag 直传（scratch 镜像无 shell 自定义配置面） | `Dockerfile`（ENTRYPOINT `/tini -- /wesh`，flag 跟在镜像命令后） |
| 手工/脚本 | CLI flag 直传，或 `--config` 指向各环境各自的 TOML | — |

注意事项：

- **凭据优先 env**：`WESH_CREDENTIAL` 经 systemd `EnvironmentFile=`（chmod 600）注入优先于配置文件明文；配置文件中的 `credential` 明文仅在 env 与 flag 均未给时生效。
- **自定义首页改文件需重启**：`--index` 启动时一次读入内存，运行期零磁盘依赖。
- **socket 残留自清理**：`--socket` 路径存在残留 socket 端点时自动清理（systemd `Restart=` 场景零人工干预）；存在非 socket 文件则拒绝启动。
- **分享 token 重启即废**：ro/rw 分享链接的 token 每轮启动重新随机——吊销全部旧链接的语义就是重启进程。
