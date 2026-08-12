# Pitfalls Research

**Domain:** Web 终端分享工具（ttyd 类：PTY over WebSocket 到浏览器）
**Researched:** 2026-08-13
**Confidence:** HIGH（ttyd 缺陷为源码审计一手核实）/ MEDIUM（生态通用陷阱，多源交叉验证）

> **下游使用说明**：每条陷阱含预警信号、预防策略、建议处理阶段。阶段编号引用建议的路线图骨架：
> - **P1 协议与核心 IO 管道**：WS 协议设计（握手即认证、类型化帧、长度上限、合规关闭码）、PTY 双向转发、resize（单客户端）、子进程收割
> - **P2 会话保持与多客户端**：会话-连接解耦、滚动缓冲回放、多客户端 attach、扇出背压、多客户端 resize 策略
> - **P3 认证与安全**：常数时间比较、令牌体系、失败节流、Origin 允许列表、TLS、env 白名单、URL 参数校验、转义序列策略
> - **P4 前端体验与部署运维**：xterm.js addons、重连 UX、反代配方、base-path、healthz/metrics/结构化日志、容器镜像

## Critical Pitfalls

### Pitfall 1: 手写 WebSocket 分片重组 + 认证前无资源上限（预认证 DoS/崩溃）

**What goes wrong:**
ttyd 1.7.7 两处最严重漏洞都在此：空 WS 帧触发手写重组逻辑空指针解引用，任何未认证客户端可远程崩溃整服（utils.c:34, protocol.c:298）；分片累积缓冲在认证检查之前无限 realloc，构成预认证内存放大（protocol.c:288-296）。这不是 ttyd 独有——2026 年 7 月披露的 Bandit（Elixir）GHSA-vg8x-66vg-5pxh（CVSS 8.7）：字节上限（8MB）不限帧数，攻击者发百万个 1 字节 continuation 帧，重组时每次遍历整个累积缓冲，O(n²) CPU 耗尽，预认证可触发；同库此前还有 CVE-2026-42786（重组累积完全无上限）；undici CVE-2026-12151 同类。这是 WS 服务端的**系统性事故高发区**。

**Why it happens:**
开发者把"认证"当成第一道工序，把"资源限制"当成第二道；实际攻击面在认证之前就已暴露。裸 libwebsockets 回调模型把分片重组甩给应用手写，等于在库已解决的问题上重新造错。Go 生态同样危险：gorilla/websocket **默认无读限制**，不显式调 `SetReadLimit` 等于裸奔。

**How to avoid:**
- 绝不手写分片重组。选内置上限的成熟库：Rust tungstenite 默认 `max_message_size=64MiB`、`max_frame_size=16MiB`（Context7 核实）；Go 必须显式 `SetReadLimit`。
- 限制要三层齐下：**单帧大小 + 累积消息字节 + 分片帧数**（Bandit 教训：只限字节数没用）。
- 认证并入 WS 握手（首帧到达前完成），认证通过前不为该连接分配任何消息级缓冲。
- 超限处理走合规关闭码（1009 Message Too Big），不是静默丢弃或崩溃。

**Warning signs:**
- 代码里出现自写的 continuation-frame 追加循环 / `realloc` 累积缓冲。
- 认证检查位于"消息完整重组"之后。
- 压测缺失"1 字节 × 百万帧"和"空帧"用例。

**Phase to address:** P1（协议层一次性设计到位，事后补洞要动协议）

---

### Pitfall 2: 慢客户端背压楔死整个 PTY 扇出

**What goes wrong:**
多客户端 attach 同一会话时，若服务端按连接顺序同步写 WS：一个 stalled 客户端（笔记本合盖、弱网、浏览器标签页冻结）写阻塞 → 事件循环卡住或 PTY 读取停止 → 内核 PTY 缓冲写满 → 子进程 write 阻塞 → **所有客户端一起卡死，连 shell 本身都hang**。即使单客户端场景，无写超时也会让服务端内存随未消费输出线性增长。注意 Rust 也不天然免疫：tungstenite 默认 `max_write_buffer_size = usize::MAX`（写缓冲无限，Context7 核实），背压上限需手动设置。

**Why it happens:**
PTY 只有一个读端，扇出逻辑天然想"读到一块数据，依次写给每个人"；把"发送成功"当成同步即时事件，忽略 TCP 发送缓冲会满。库默认值偏向"不丢数据"而非"不被拖死"。

**How to avoid:**
- 每客户端独立的**有界出站队列**（如 256 帧 / 1-4MB），队列满时明确策略：断开该慢客户端（gorilla chat 示例的经典模式）或丢最旧帧并标记"输出有缺口，触发重同步"。
- PTY 读取循环永远不因任何客户端状态阻塞；写操作全部有 deadline。
- 终端数据可容忍丢帧（配合 C3 的屏幕快照重同步），不要为慢客户端保留无限历史。
- 压测必须含"attach 后停止读取的 TCP 客户端"用例。

**Warning signs:**
- 所有客户端共用一个发送缓冲或互斥锁包住整个扇出循环。
- 写路径无超时、无队列长度指标。
- 演示时两人同屏流畅，但没测过"一个人断网不关页面"。

**Phase to address:** P2（扇出在 P2 引入；P1 单客户端也要先有写超时/写缓冲上限）

---

### Pitfall 3: 原始字节流回放 ≠ 屏幕状态——全屏 TUI 重连后花屏

**What goes wrong:**
实现会话保持时最自然的做法是"服务端记录 PTY 输出字节流，重连时回放"。对行模式输出（bash 回显、日志）有效，但 vim/htop/less/tmux 这类全屏 TUI 使用 alternate screen + 绝对光标定位 + 局部擦除：从字节流中间任意点开始重放，客户端解析器缺少前文状态，画面叠字、错位、残留。同类项目实测实证：relay 项目 ND-23 决策记录——跨设备 attach 后 TUI 花屏，即便尺寸对齐仍有"replay-during-attach 与 TUI 光标 save/restore 交互不良"问题。dtach 的教训更直接：它**没有终端模拟层**，重连后只能靠发 Ctrl-L 或 WINCH 让应用自觉重绘，不响应的应用就是花的。tmux 能做到无损 attach 是因为它在服务端维护了一整块虚拟屏幕。

**Why it happens:**
字节流是"过程"，屏幕是"状态"；从过程的中段恢复不出状态。开发者用行模式程序测试通过就以为功能完成。

**How to avoid:**
- 服务端内嵌 VT 解析器维护当前屏幕快照（Rust 有 vt100/avt 类 crate；Go 有同等库），重连时下发"清屏 + 当前屏幕快照 + 增量流"，而非原始字节流。这也顺带解决 C2 中"丢帧后重同步"。
- 若 v1 不上服务端 VT 状态机，则明确降级语义并在文档写明：行模式应用无损；全屏 TUI attach 后需用户 `Ctrl-L` 重绘（dtach 语义），不要假装支持。
- alt-screen 期间产生的输出不应混入主屏幕回放缓冲。

**Warning signs:**
- attach 后 vim 打开的文件画面错乱、htop 只剩残影。
- 回放体积随会话时长线性膨胀（说明在存原始流而非状态）。
- 测试只跑了 `bash` + `ls`，没跑过 vim。

**Phase to address:** P2（会话保持的核心架构决策，P1 不需要）

---

### Pitfall 4: 滚动缓冲无上限 → 服务端与浏览器双侧内存膨胀

**What goes wrong:**
会话保持要求服务端为每个会话保留回放历史。无上限 = 长跑会话（`tail -f`、编译输出）把服务器内存吃光。浏览器侧同样有坑（Context7 核实 xterm.js 源码）：`WriteBuffer` 待解析数据超过 **DISCARD_WATERMARK ≈ 50MB** 时 `write()` 直接 throw `'write data discarded, use flow control to avoid losing data'`——重连时一次性灌入大回放不但丢数据还可能让前端崩掉。xterm.js 默认 scrollback 仅 1000 行。

**Why it happens:**
"回放多少"没有产品决策，默认成"全部"；前端把 `term.write()` 当成同步无成本调用。

**How to avoid:**
- 服务端：每会话**字节级**环形缓冲上限（默认 1-4MB 可配；按字节不按行——终端输出是字节流，按行切会切断转义序列）。超限丢弃最旧数据，回放时标注缺口。
- 前端：回放分块（如 64KB/块），用 `term.write(data, callback)` 回调驱动下一块（这是 xterm.js 官方流控机制）；回放期间显示进度/禁止输入。
- 回放体积上限与服务端缓冲上限对齐；xterm.js scrollback 选项按产品需要调大（如 10k 行）但认知其内存代价。

**Warning signs:**
- 服务端 RSS 随会话存活时间单调上涨。
- 重连长会话后浏览器标签页卡顿数秒或直接 throw。
- 环形缓冲按"行"实现。

**Phase to address:** P2

---

### Pitfall 5: 终端转义序列注入（OSC 52 剪贴板 / OSC 8 钓鱼链接 / 标题 / 日志）

**What goes wrong:**
共享终端场景放大了经典终端攻击面：**远端命令的输出对每一个 attach 的浏览器都是不可信输入**。
- **OSC 52**：任何能写 stdout 的程序都能写用户系统剪贴板（`printf '\e]52;c;<base64>\a'`）；若开放剪贴板读，可静默窃取用户刚复制的密码/密钥。Warp 终端 2025 年因此中招（CVE-2025-48725，GHSA 评级）。
- **OSC 8 超链接**：显示文本与 URL 独立——显示 `github.com` 点开 `evil.com`，`git log`、包管理器输出均可携带。
- **OSC 0/1/2 标题**：可被远程内容改写伪装主机名/路径；标题回读查询（CSI 21 t）曾因注入风险被现代终端禁用。
- **日志逃逸**：含 ESC 的用户输入进日志，管理员 `cat` 时序列激活，可隐藏行、改标题、写剪贴板。
- xterm.js 自身也不是天然免疫：CVE-2019-0542（GHSA-mc23-976p-j42x，CVSS 8.8，CWE-94 代码注入），npm xterm <3.8.1 特殊字符处理不当致 RCE。

**Why it happens:**
"终端输出只是文本"的直觉错误；多人共享场景下信任边界消失——你 attach 的会话里跑的程序未必是你启动的。

**How to avoid:**
- 保持 xterm.js 最新（供应链层面跟进其安全修复）。
- `@xterm/addon-clipboard`（OSC 52 支持）默认**关闭**；开启时只允许写、禁止读（浏览器 Clipboard API 读有权限门控，但写很宽松）；多客户端旁观模式强制关。
- 服务端可选的序列过滤策略（剥离/改写 OSC 52、OSC 8 需策略评估），至少提供开关。
- web 链接 addon 点击前 hover 显示真实 URL（xterm.js 支持自定义 hover/ handler）。
- wesh 自身日志：所有用户可控字段（URL 参数、客户端输入元数据）入库前剥离 0x00-0x1F 控制字符。

**Warning signs:**
- 演示时跑 `printf '\e]52;c;SGVsbG8=\a'` 后剪贴板被改且无提示。
- 依赖锁文件里 xterm.js 版本停滞。
- 日志里出现裸 ESC 字节。

**Phase to address:** P3（过滤策略与默认配置）；P4（前端 addon 接线与权限默认）

---

### Pitfall 6: 认证子系统连环错（ttyd 五项全中的反面教材）

**What goes wrong:**
ttyd 源码审计确认的认证缺陷清单：**strcmp 非时序安全比较**（逐字节短路泄露前缀匹配信息）；**凭据 base64 明文打印进日志**（server.c:142，base64 不是加密）；**/token 端点把凭据明文返回**给通过 HTTP 认证的客户端（扩大暴露面）；**AuthToken 与 Basic 凭据同一 secret 复用**；**无失败节流**，可在线爆破。任一单点都够写 CVE，组合拳等于没有认证。

**Why it happens:**
Basic Auth"看起来能用"就停了；token 机制为前端 WS 认证服务而设，没意识到它成了凭据分发端点；日志调试时图省事。

**How to avoid:**
- 常数时间比较（Go `crypto/subtle.ConstantTimeCompare`、Rust `subtle` crate）；比较前先等长哈希（SHA-256）消除长度侧信道。
- 一次性短时令牌（TTL 秒级、绑定会话、单次使用）替代可重放静态 token；令牌与静态凭据是独立 secret。
- 认证失败节流：每 IP/每连接指数退避 + 临时锁定；失败计数进 metrics。
- 日志红线：凭据、令牌、Authorization 头任何形态（含 base64）永不进日志；加日志脱敏测试。
- Basic 凭据只走 TLS；明文 HTTP 模式启动时打醒目警告。

**Warning signs:**
- 代码里搜到 `==` / `strcmp` 比较 secret。
- 日志样例里出现 Authorization 或 base64 串。
- `/token` 类端点响应体含长期有效凭据。

**Phase to address:** P3

---

### Pitfall 7: 子进程环境继承泄密 + URL 参数拼接注入

**What goes wrong:**
ttyd 审计确认：子进程**继承服务端全部环境变量**（pty.c:441-444）——运维机器 env 里的云密钥、API token 全部泄露给 web shell 的任何人（`env` 一条命令即得）；`?arg=` URL 参数**无校验无上限**拼接到执行命令（protocol.c:241-249），若服务端用 shell 拼接即命令注入；另有 pty_spawn 失败路径误 `close(0)`（pty.c:87,112）。

**Why it happens:**
`forkpty`+`execvp` 默认继承 environ，不显式构造环境就全漏；URL 传参功能（-a）为便利性设计，安全校验是后加的而 ttyd 没加。

**How to avoid:**
- 子进程环境**白名单**：最小集（TERM、HOME、USER、PATH、LANG/LC_*、SHELL），其余显式传入才给。
- `?arg=` 若保留：字符白名单（拒绝 shell 元字符）、条数上限、单条/总长度上限；**永远以 exec 数组传递，绝不经过 shell 展开**。
- spawn 失败路径单独测试：失败时不得关闭服务端自身 fd（0/1/2），错误要以类型化错误帧传给客户端（ttyd 做不到，协议缺陷）。
- 降权运行（uid/gid）与 chroot/cwd 配置在 P1 就预留接口。

**Warning signs:**
- web shell 里 `env` 能看到服务器密钥。
- 命令构造路径出现字符串拼接 + `sh -c`。
- spawn 失败时服务端 stdin 被关、日志断流。

**Phase to address:** P3（白名单与校验）；P1（spawn 路径正确性、exec 数组）

---

### Pitfall 8: 僵尸进程泄漏与每客户端 waitpid 线程

**What goes wrong:**
forkpty 子进程退出后父进程不 reap → 僵尸累积占进程表，最终 `fork: Resource temporarily unavailable` 整服无法新建会话。ttyd 的做法是**每客户端独占一条 waitpid 线程**（pty.c:483）——不泄漏但线程数随连接数线性涨，且与 libuv 单循环跨域交互，是 UAF 温床。容器场景加倍危险：wesh 若以 PID 1 运行，必须承担全容器僵尸收割义务。

**Why it happens:**
SIGCHLD 异步语义与事件循环整合有门槛，开线程 waitpid 是最省心的错误答案；`SA_NOCLDWAIT` 能免僵尸但**丢退出状态**（无法向客户端报告 exit code）。

**How to avoid:**
- 统一收割：SIGCHLD handler 内循环 `waitpid(-1, WNOHANG)` 至 ECHILD，或 Linux 5.3+ 用 pidfd 把子进程死亡并入事件循环；单线程/事件驱动，零额外线程。
- 退出状态要保留并传给 attach 的客户端（类型化会话结束帧）。
- 容器镜像内置 tini/dumb-init，或自身实现 PID 1 收割逻辑（文档二选一）。
- 压测含"短生命周期会话高频创建销毁"，监控 `ps aux | grep defunct`。

**Warning signs:**
- `top` 里线程数 ≈ 客户端数。
- 长时间运行后 `defunct` 进程堆积。
- 容器内僵尸属于 PID 1。

**Phase to address:** P1

---

### Pitfall 9: RFC6455 协议合规失守（关闭码 1006、UTF-8、mask）

**What goes wrong:**
ttyd 审计确认把 **1006 写进 close frame**（protocol.c:90,105）——RFC6455 §7.4 明确 1005/1006/1015 为保留值，**MUST NOT** 由端点写入 Close 帧；1006 只供应用内部表示"异常关闭（未收发 Close 帧）"。严格客户端/代理库遇到非法关闭码会报协议错误，行为分歧难排查。同类合规点：客户端帧必须带 mask（服务端须校验）、文本帧 UTF-8 校验跨分片边界（逐帧校验会误杀合法的跨帧多字节序列）、ping/pong 语义。

**Why it happens:**
把"内部状态码"和"线上协议码"混用；协议细节凭直觉实现不查 RFC。

**How to avoid:**
- 关闭码映射表写死进协议层：正常 1000、离开 1001、协议错 1002、策略违例 1008（认证失败/Origin 拒绝）、消息过大 1009、内部错误 1011；**永不写 1005/1006/1015**。
- 前端以 `event.wasClean` / `code` 缺失判断异常，而非等 1006（线上永远收不到 1006）。
- 用成熟库（tungstenite/gorilla）获得 mask/UTF-8/分片合规，不自解析帧。
- 协议测试引入 Autobahn Test Suite（WS 实现合规事实标准）。

**Warning signs:**
- 代码里出现字面量 1006 的发送路径。
- 前端重连逻辑以"收到 1006"为分支条件。
- 自写的帧头解析。

**Phase to address:** P1（协议层）；P4（前端重连判断）

---

### Pitfall 10: 多客户端 resize 策略错误（一个 PTY 只有一个 winsize）

**What goes wrong:**
PTY 内核只保存一个 winsize；多个不同尺寸的客户端 attach 同一会话时，无论跟随谁，其余客户端的 TUI 绝对定位必然画出可视区 → 花屏。relay 项目实测（ND-23→ND-39 修订）：先选 last-writer-wins，多客户端并发 attach 后截图实证 TUI 错乱，最终修订为——**单客户端时 last-writer-wins（同 tmux/screen 惯例）；≥2 客户端时服务端钳制到最小公共矩形 `min(cols)×min(rows)`，2→1 转换时恢复**；resize 值须上限钳制（如 1000×1000）防恶意/异常客户端。另有边界坑：会话创建到首个 attach 之间 PTY 用默认尺寸（如 120×32），此窗口内 TUI 首帧会按错误尺寸绘制——首个 attach 的 resize 帧应尽早（claim 之前）到达。

**Why it happens:**
按单客户端思维设计 resize，多客户端是后加的功能；TIOCSWINSZ 会向前台进程组发 SIGWINCH，高频 resize（拖拽窗口）还会引发 TUI 重绘风暴。

**How to avoid:**
- resize 作为协议独立帧、独立于读写权限的旁路通道（只读旁观者也影响尺寸是设计决策，需明示——建议只读旁观不参与尺寸协商）。
- 多客户端策略按 ND-39 结论实现；尺寸变化时主动触发屏幕重同步（配合 C3 的快照机制）。
- resize 帧服务端防抖/合并（如 50ms 合并窗口）防 SIGWINCH 风暴。
- 前端 fit addon 的 `proposeDimensions` 在元素 `display:none` 时返回无效值，须判空 + debounce（约 100ms）后再上报。

**Warning signs:**
- 两人不同尺寸窗口同屏时一方画面出现边框残留/叠字。
- 拖浏览器窗口时远端 TUI 疯狂闪屏。
- resize 处理里没有钳制和防抖。

**Phase to address:** P2（策略）；P4（前端 fit 防抖）

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| 每连接一进程，靠 `ttyd tmux new -A` 变相共享 | 服务端极简 | 会话保持/多客户端永远无法原生；tmux 键绑定、尺寸、转义双层透传全是用户侧坑 | **Never**——本项目的存在意义就是替代它 |
| 字节流回放当"会话保持" | 一天能写完 | 全屏 TUI 花屏（C3），上线即口碑事故 | v1 可接受"行模式无损+TUI 靠 Ctrl-L"的明示降级，但协议要预留快照帧位 |
| 引入停更依赖（zmodem.js 2017 停更需本地 patch、decko 停更） | 快速凑功能 | 安全修复无人维护，打包工具链腐烂 | **Never**——v1 已把 ZMODEM 移出范围 |
| `execCommand('copy')` 选中即复制 | 兼容旧浏览器 | API 已废弃，权限行为不一致 | Never，用 `navigator.clipboard`（需 HTTPS/localhost 安全上下文） |
| 硬编码默认 PTY 尺寸 80×24/120×32 | 少一个配置 | 首个 attach 前 TUI 首帧画错 | 首帧窗口可接受，但默认值必须可配且文档化 |
| 认证"能用即可"先上线 | 提前一周 | C6 全家桶，事后补要动协议与前端 | Never（安全是本项目核心卖点） |
| 结构化日志/metrics 后置 | 核心先跑 | 生产事故无观测手段，ttyd 即如此 | v1 内必须补 `/healthz` + 基础 metrics（P4 前半），不可拖到 v2 |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| nginx 反代 | 不配 `Upgrade`/`Connection` 头；`proxy_read_timeout` 默认 **60s** 切断空闲 WS，浏览器侧表现为 1006 重连循环 | `map $http_upgrade $connection_upgrade` + `proxy_read_timeout` 调大（如 3600s）；服务端 WS ping 间隔显著小于代理空闲超时（ttyd 默认 5s 足够）；文档给完整 server 块配方 |
| Cloudflare 等 CDN | 免费计划 WS 空闲约 100s 被切 | ping 间隔 < 100s；部署文档注明各 CDN 空闲上限 |
| Docker 容器 | 服务作为 PID 1 不收割僵尸，defunct 堆积 | 镜像内置 `tini`/`dumb-init` 或自身实现 PID 1 收割（C8）；`--init` 说明写进 README |
| systemd | 无 `Restart=on-failure`、`LimitNOFILE` 过低、凭据写进 unit 文件（world-readable） | 官方 unit 模板：`Restart=on-failure`、`LimitNOFILE=65536`、`EnvironmentFile=` 600 权限加载凭据 |
| tmux 内运行 wesh / wesh 内跑 tmux | 双层 PTY 尺寸与转义透传冲突 | 文档说明嵌套行为；wesh 会话内 TERM 固定为前端真实能力（xterm-256color） |
| Caddy / Traefik | 自动 HTTPS 但 WS 路径未显式放行 | 部署文档给 Caddyfile/labels 配方，注明 WS 无需特殊配置但 idle timeout 要查 |
| base-path 子路径挂载 | 尾斜杠处理不一致（`/wesh` vs `/wesh/`）301 丢 WebSocket 升级 | 301 仅对 HTTP 页面；WS 路径规范化单侧定义；反代下测试升级请求 |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| 每客户端 waitpid 线程（ttyd 模式） | 线程数=连接数，RSS/调度开销上涨 | SIGCHLD/pidfd 统一收割（C8） | >100 并发连接 |
| 固定 64KB 读缓冲、读后即停（ttyd 模式） | `cat` 大文件吞吐远低于管道直写 | 读到 EAGAIN 为止 + 自适应缓冲 | 高吞吐输出（编译、日志刷屏） |
| 每数据块 3-4 次拷贝（ttyd 模式） | CPU 占用高、延迟抖动 | 零拷贝管道 / buffer pool；协议帧就地复用 | 高吞吐 + 多客户端扇出叠加 |
| 重连回放突发灌满前端 | 浏览器卡秒级、xterm.js 50MB watermark throw 丢数据 | 分块 + write callback 流控（C4） | 长会话重连 |
| 逐帧 UTF-8 校验 | 跨分片多字节字符被误杀、连接误关 | 流式 UTF-8 校验器（库自带） | CJK/emoji 输出 + 小 MTU 分片 |
| 扇出 O(客户端数) 每帧遍历+逐帧加锁 | 客户端多时输出延迟随人数线性涨 | 每客户端独立队列 + 单写者协程 | >20 旁观客户端 |
| permessage-deflate 全开 + context takeover | 每连接 zlib 状态内存常驻、CPU 尖峰；终端二进制流压缩收益低 | 关闭 WS 压缩或禁 context takeover；终端数据本就高熵 | 高连接数 + 大输出 |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| strcmp/== 比较凭据 | 时序侧信道逐字节恢复凭据 | 常数时间比较 + 先哈希等长（C6） |
| 凭据/token 任何形式进日志 | base64=明文，日志收集系统成凭据库 | 日志脱敏 + CI 日志样例扫描（C6） |
| /token 明文下发长期凭据 | 中间人/日志/浏览器历史全暴露 | 一次性短时绑定会话令牌（C6） |
| 无认证失败节流 | 在线爆破 | 指数退避 + 每 IP 限速 + 锁定（C6） |
| Origin 仅字符串比对 | CSWSH（跨站 WebSocket 劫持）绕过 | Origin 允许列表 + 规范化比较 + 失败拒绝（ttyd -O 反例：protocol.c:51-71） |
| 子进程继承全部 env | 服务器密钥泄露给 web shell | env 白名单（C7） |
| `?arg=` 无校验拼接 | 命令注入/参数注入 | 白名单字符 + 上限 + exec 数组（C7） |
| OSC 52 读开放 | 静默窃取剪贴板（密码管理器内容） | 默认禁读，写也需明示开关（C5） |
| TLS 只禁 1.0/1.1 | 弱 cipher、无 HSTS、无安全响应头 | TLS1.2+ 现代 cipher 套件 + 安全响应头；testssl.sh 过检（P3） |
| 明文 HTTP 无警告 | 凭据/sniffing 全裸奔 | 非 TLS 启动打醒目警告，绑定非 loopback 时警告升级 |
| 关闭信号误发进程组 | 误杀无关进程 | kill 前校验 PGID 确属该会话（ttyd -s 发 -pid 的先例需保留校验） |
| 自签证书 + 客户端 InsecureSkipVerify 教程 | 用户照抄后中间人敞开 | 文档推荐 mkcert/CA 方案，不给 skip-verify 教程 |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| 断线即杀进程（ttyd 默认） | 刷新/断网 = 会话全丢 | 会话保持是本项目核心价值，P2 最高优先 |
| 重连开新会话却不告知 | 用户以为恢复了，实际状态已丢 | attach 原会话；失败时明示"新会话" |
| 只读默认无任何界面提示 | 用户敲键盘无反应以为卡死 | 明显的只读徽标 + 键盘事件响应"当前为只读"浮层 |
| resize 无防抖无浮层 | 拖窗口闪烁、不知当前尺寸 | debounce + COLSxROWS 浮层（ttyd 已有正确先例） |
| 标题不同步 | 多标签页分不清哪个会话 | OSC 标题 → document.title（含主机名前缀） |
| 重连无退避无限刷 | 服务端重启期间被重连风暴打 | 指数退避 + 上限 + 手动重连入口 |
| 移动端/小屏无处理 | 软键盘顶起视口后终端尺寸错乱 | visualViewport 监听 + fit 重算（P4 验证项） |

## "Looks Done But Isn't" Checklist

- [ ] **认证:** 能挡住错误密码 ≠ 完成 — verify 常数时间比较（时序测试）、失败节流（脚本爆破 100 次观察退避）、日志无凭据（grep base64 样例）
- [ ] **会话保持:** `bash`+`ls` 重连正常 ≠ 完成 — verify 断线前开 vim/htop，重连后画面无损（C3 是最大架构风险）
- [ ] **多客户端:** 两人能同屏 ≠ 完成 — verify 不同窗口尺寸（C10 最小矩形）、一人拔网线和一人停止读取 TCP 流（C2）其他人无感
- [ ] **资源上限:** 设了消息字节上限 ≠ 完成 — verify 1 字节×百万 continuation 帧（帧数上限，C1 Bandit 教训）、空帧（崩溃回归）、握手未完成即灌数据
- [ ] **TLS:** 能握手 ≠ 完成 — verify testssl.sh 无弱 cipher、HSTS/安全响应头在
- [ ] **resize:** 单客户端正常 ≠ 完成 — verify 拖拽防抖、display:none 时不发 NaN、多客户端策略符合 C10
- [ ] **子进程:** 能起 shell ≠ 完成 — verify `env` 输出无服务器密钥（C7）、spawn 失败时服务端 fd 0/1/2 完好、退出状态传回客户端
- [ ] **关闭路径:** 能断开 ≠ 完成 — verify 线上关闭码只在 1000/1001/1002/1008/1009/1011 集合内，前端 wasClean 判断正确
- [ ] **重连:** 能重连 ≠ 完成 — verify 回到**同一会话**（不是新进程）、回放有流控不卡浏览器、断网 30s 后输入输出一致
- [ ] **容器:** 镜像能跑 ≠ 完成 — verify 高频建销会话后容器内无 defunct（C8）

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| 预认证 DoS/崩溃被打 | HIGH | 临时前置限流（nginx limit_req/连接数）；根治须动协议层（C1），无热补丁 |
| 背压楔死 | LOW | 断开慢客户端即恢复；根治在 C2 队列化 |
| 会话屏幕状态损坏 | MEDIUM | 用户侧 Ctrl-L / 服务端主动重发快照；反复出现说明 C3 机制缺陷 |
| 回放撑爆浏览器 | LOW | 降回放窗口+分块；用户刷新重连 |
| 凭据泄露（日志/token 端点） | HIGH | 轮换全部凭据+审计访问日志+通知；代码修 C6 全套 |
| OSC 52 剪贴板被改写 | LOW | 即时危害低但信任损失大；关闭 addon、加服务端过滤 |
| 僵尸堆积到 fork 失败 | MEDIUM | 重启服务；根治 C8 收割重构 |
| TLS 误配暴露弱套件 | MEDIUM | 改配置重载即可；用 testssl.sh 回归 |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| C1 分片重组/预认证无上限 | P1 | 空帧/百万小帧/超限帧模糊测试；认证前零分配代码走查；Autobahn 套件 |
| C2 慢客户端背压 | P2（P1 先加写超时） | stall 客户端混沌测试；每客户端队列长度 metric |
| C3 字节流回放花屏 | P2 | vim/htop/less/tmux 断线重连画面比对测试 |
| C4 缓冲无上限 | P2 | 长跑会话 RSS 平稳；大回放不触发 50MB throw |
| C5 转义序列注入 | P3（策略）+ P4（前端默认） | OSC52/OSC8/title 攻击向量 e2e 用例；xterm.js 版本跟进机制 |
| C6 认证连环错 | P3 | 时序测试、爆破测试、日志扫描、token TTL/单次性测试 |
| C7 env 继承/参数注入 | P3 + P1（spawn 路径） | web shell `env` 审计；`?arg=` 模糊测试；spawn 失败注入测试 |
| C8 僵尸/waitpid 线程 | P1 | 高频建销压测 + defunct 监控；容器内 PID 1 场景测试 |
| C9 RFC6455 合规 | P1 + P4（前端判断） | Autobahn Test Suite 全绿；关闭码断言 |
| C10 多客户端 resize | P2 + P4（fit 防抖） | 异尺寸多客户端截图比对；resize 风暴测试 |
| nginx/代理超时 | P4 | 反代下空闲 5 分钟连接存活 e2e |
| 容器 PID 1 僵尸 | P4 | 官方镜像内置 init + 文档 |
| permessage-deflate | P1（协议协商默认关） | 高连接压测 CPU/内存基线 |
| 结构化日志/metrics 缺失 | P4 前半（不拖 v2） | /healthz、连接/会话/字节数指标可抓 |

## Sources

- **ttyd 1.7.7 全量源码审计**（本项目 PROJECT.md Context 节，含行号证据，2026-08-13）— HIGH，一手核实
- terminfo.dev《Terminal Security: Clipboard, Paste Injection, Escape Attacks》— https://terminfo.dev/fundamentals/security — MEDIUM
- GHSA-vg8x-66vg-5pxh / CVE-2026-65623（Bandit WS 分片重组 O(n²)，CVSS 8.7）— https://github.com/mtrudel/bandit/security/advisories/GHSA-vg8x-66vg-5pxh — MEDIUM（官方 advisory）
- CVE-2026-42786（Bandit 重组累积无上限预认证内存放大）、CVE-2026-12151（undici WS DoS）— cvereports.com — MEDIUM
- GHSA-mc23-976p-j42x / CVE-2019-0542（xterm.js RCE，<3.8.1/3.9.0-3.9.1/3.10.0，CWE-94）— https://github.com/advisories/GHSA-mc23-976p-j42x — MEDIUM（官方 advisory）
- CVE-2025-48725（Warp OSC 52 剪贴板访问控制）— MEDIUM
- relay 项目 ND-23/ND-39 决策记录（PTY 尺寸协商与多客户端 attach 实测）— https://github.com/TechGardenCode/relay/blob/main/docs/decisions/ND-23-pty-size-negotiation-and-sigwinch-forwarding-for-attach-clients.md — MEDIUM
- RFC 6455 §7.4（1005/1006/1015 保留值 MUST NOT 写入 Close 帧）— https://datatracker.ietf.org/doc/html/rfc6455 — HIGH（标准原文）
- Context7 文档核实：tungstenite `WebSocketConfig` 默认值（max_message_size 64MiB / max_frame_size 16MiB / max_write_buffer_size 无限）；gorilla/websocket 并发模型与 SetReadLimit；xterm.js WriteBuffer DISCARD_WATERMARK≈50MB、scrollback 默认 1000、write callback 流控 — MEDIUM
- nginx WS 反代超时与配置（proxy_read_timeout 默认 60s）多源交叉 — MEDIUM
- dtach 无终端模拟层（重绘靠 Ctrl-L/WINCH）项目文档 — MEDIUM
- gorilla/websocket chat 示例（有界 channel + 满即断开的扇出模式）— MEDIUM

---
*Pitfalls research for: Web 终端分享工具（wesh，ttyd 现代化重写）*
*Researched: 2026-08-13*
