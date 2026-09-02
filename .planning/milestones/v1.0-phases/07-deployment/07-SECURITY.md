---
phase: 07
slug: deployment
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-27
---

# Phase 07 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| reverse-proxy → wesh (HTTP path/headers) | base-path 前缀与 X-Remote-User/X-Forwarded-For 头是不可信输入——仅 operator 配置且反代后可采信 | 请求 path、身份头（用户可控） |
| filesystem (go:embed dist) | StripPrefix 后的路径只进 go:embed 静态 FS，不触 OS 文件系统 | 静态资源路径 |
| local filesystem → wesh (socket 路径) | socket 路径是 operator 配置；路径上既有文件可能是攻击者预置（/run 或共享 tmp） | socket 文件系统对象 |
| local user → unix socket | socket 文件权限位是唯一访问控制（D-11 文件系统即认证边界） | 本地 IPC 通道 |
| wesh → stderr (logEvent / config errors) | remote_user 是用户可控字段；go-toml 错误上下文可能回显源行值 | 审计日志行、错误输出 |
| wesh → OS (process spawn / signals) | cwd/TERM/uid/gid 是 operator 配置；stop-signal 发进程组——组边界错误误杀无关进程 | 子进程身份/环境、信号 |
| OS → wesh (signals) | SIGTERM/SIGINT 来自 init/operator——优雅下线触发源 | 关停信号 |
| wesh → OS desktop (xdg-open/open) | --open 把含 token 的 URL 传给系统启动器（wesh 自构，非用户输入） | 分享 URL（含 token） |
| filesystem (TOML) → wesh | 配置文件是 operator 控制输入，但可能含凭据——权限与错误输出是泄露面 | 配置凭据 |
| UAT/pw 脚本 → CI 日志/控制台 | detail 输出是泄露面——凭据/token 只作断言材料，永不进输出 | 测试输出 |
| 文档 → operator 部署行为 | README 配方错误会把部署引向不安全形态（裸暴露/WS 403 即坏） | 部署配方 |
| 并发 wesh 实例 → 同一 socket 路径 | 同机第二实例（手误/systemd 竞态）可能夺走存活实例端点 | socket 端点所有权 |
| wesh → opener 子进程（xdg-open/open） | 桌面态外部进程，退出码是异常信号；URL 含 share token 流经 argv | 退出码、argv |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-07-01a | Tampering | --base-path 值校验 | medium | mitigate | D-13 严格字符集/尾斜杠/`..` 拒绝 parse 期 exit 2（cmd/wesh/main.go:672 normalizeBasePath） | closed |
| T-07-01b | Information Disclosure | StripPrefix 静态伺服 | medium | mitigate | 前缀不匹配请求结构性 404；目标仅 go:embed FS 无 OS 路径逃逸面 | closed |
| T-07-01c | Tampering | 裸路径 308 重定向 Location | low | accept | 见 Accepted Risks AR-07-01 | closed |
| T-07-01d | Spoofing | Origin 校验 × base-path | low | accept | 见 Accepted Risks AR-07-02 | closed |
| T-07-02a | Tampering | listenSocket 序列 | medium | mitigate | D-10 listen 前 Remove + listen 后显式 Chmod/Chown 失败回滚；README /run root 目录建议 | closed |
| T-07-02b | Elevation of Privilege | socket 文件权限位 | high | mitigate | --socket-mode 默认 0660 listen 后显式 Chmod 达成（cmd/wesh/main.go:403 注释锁定），拒绝 umask 漂移；失败回滚 exit 1 | closed |
| T-07-02c | Information Disclosure | Chmod 与 listen 间 umask 窗口 | low | accept | 见 Accepted Risks AR-07-03 | closed |
| T-07-02d | Denial of Service | 残留 socket 致重启失败 | low | mitigate | D-10 Remove 使 systemd Restart= 零人工干预；活性探测后 ECONNREFUSED 残留照旧清理（07-10 加固） | closed |
| T-07-03a | Spoofing | XFF/auth-header 采信 | high | mitigate | 裸信任 + D-16 非 loopback 无凭据 stderr 暴露面警告 + D-17 零认证决定（伪造头只污染日志不越权）+ README「仅反代后部署」明示（README.md:66/331） | closed |
| T-07-03b | Tampering | remote_user 日志注入 | medium | mitigate | D-19 C0/C1/DEL 剥离 + 128 rune 截断（internal/server/proxy.go:55 sanitizeRemoteUser，title.ts 同款纪律 Go 移植） | closed |
| T-07-03c | Information Disclosure | remote_user 误录 token | high | mitigate | 提取源限定配置头名对应 HTTP 头（结构性不可能是路径 token/Hello ticket）+ 07-07 assertOutputClean 运行时自净 | closed |
| T-07-03d | Denial of Service | XFF 换键后节流被绕 | medium | accept | 见 Accepted Risks AR-07-04 | closed |
| T-07-04a | Denial of Service | SignalGroup 误发无关进程组 | medium | mitigate | 负 pid 组信号 + setsid 组长 pgid==pid 不变量（internal/pty/signal_linux.go）；KILL 补发 ESRCH 幂等静默 | closed |
| T-07-04b | Elevation of Privilege | 降权半配置静默放行 | high | mitigate | uid/gid 成对强制 exit 2 零窗口（cmd/wesh/main.go:78 哨兵 -1 设计）；数字直通避开容器 NSS 差异；Credential fork 后 exec 前生效 | closed |
| T-07-04c | Information Disclosure | 降权后 HOME 指向原用户 | medium | mitigate | D-25 user.LookupId 改写 HOME/USER/LOGNAME，查不到剔除三键（internal/pty/spawn.go:121） | closed |
| T-07-04d | Tampering | --cwd 指向非预期目录 | low | mitigate | stat 预检 fail-fast spawn 前零资源占用；cwd 是 operator 显式配置非攻击面 | closed |
| T-07-05a | Denial of Service | 1001 广播期间 stall 客户端拖延关停 | low | mitigate | Close 内建 5s+5s 上界（internal/server/close.go）；exitf 与 Shutdown 并发收口最坏 10s 不阻塞退出 | closed |
| T-07-05b | Tampering | --open URL 注入 | low | mitigate | URL wesh 自构 + exec.Command argv 分离不经 shell；--socket×--open 冲突拒绝 | closed |
| T-07-05c | Information Disclosure | --open token URL 暴露给桌面环境 | low | accept | 见 Accepted Risks AR-07-05 | closed |
| T-07-05d | Denial of Service | 1001×EXIT 竞态致客户端语义错乱 | low | accept | 见 Accepted Risks AR-07-06 | closed |
| T-07-06a | Information Disclosure | 配置文件凭据 world-readable | high | mitigate | D-07 权限非 0600/0400 stderr 警告不阻断（cmd/wesh/config.go:93）+ WESH_CREDENTIAL env 优先 + README chmod 600 建议 | closed |
| T-07-06b | Information Disclosure | 解析/校验错误回显敏感值 | high | mitigate | configErr 值剥离包装（类别+键名+行号三要素，cmd/wesh/config.go:76）；探针串运行时自证零出现 | closed |
| T-07-06c | Tampering | 未知键静默失效 | medium | mitigate | D-06 DisallowUnknownFields 严格模式 exit 2（cmd/wesh/config.go:104） | closed |
| T-07-06d | Tampering | 隐式配置加载意外行为 | medium | mitigate | D-01 仅 --config 显式指定；裸启动零路径搜索零行为漂移 | closed |
| T-07-SC | Tampering | go-toml 新依赖供应链 | high | mitigate | go-toml/v2 v2.4.3 三通道核实（proxy.golang.org + pkg.go.dev + STACK.md）；go.sum 锁定 + go mod verify；module zip 纯源码无安装期执行面 | closed |
| T-07-07a | Information Disclosure | UAT 输出泄露凭据/token | high | mitigate | assertOutputClean 运行时自净遍历断言零命中（web/uat/phase07.mjs:17/63/77）+ redactArgs 脱敏 + TOML 凭据探针同口径 | closed |
| T-07-08a | Information Disclosure | README 配方示例泄露真实凭据 | medium | mitigate | 全部示例占位值（alice:pw 假凭据/192.0.2.1 文档地址）；示例 TOML 零真实 secret | closed |
| T-07-08b | Spoofing | nginx 配方漏 auth-header 部署约束 | high | mitigate | 「--auth-header 仅反代后部署」明示 + 暴露面警告交叉引用 + ttyd -H 模型差异段落（README.md:331-343） | closed |
| T-07-09a | Tampering | README nginx 配方 | high | mitigate | 配方每行有全链实证依据；phase07-a2-pw.mjs 回归锁（文档即被测物，载具逐字镜像） | closed |
| T-07-09b | Denial of Service | WS 升级 403 误配 | high | mitigate | G-07-2 本体——proxy_set_header Host $http_host 入配方（README.md:321）+ T1-T4 全链断言锁（7eae376/ddda3a7） | closed |
| T-07-09c | Information Disclosure | pw 脚本凭据/token | medium | mitigate | 红线保持：值只作构造材料，detail/控制台只打状态码/布尔/文案常量 | closed |
| T-07-10a | Tampering | listenSocket 活性探测 | high | mitigate | G-07-3——net.Dial unix 连通即拒 EADDRINUSE 同形态 exit 1（cmd/wesh/main.go:1038）；TOCTOU 两向安全降级；存活竞争子测锁定 | closed |
| T-07-10b | Denial of Service | 残留清理回归 | medium | mitigate | ECONNREFUSED 残留照旧 Remove（D-10 不回归）；既有 stale 子测 + b1b5.sh B1c/B1d 二进制锁 | closed |
| T-07-10c | Information Disclosure | --open 警告行 | high | mitigate | G-07-8——警告行不含 URL（Wait err 仅 exit status N 结构性无 argv，cmd/wesh/main.go:1282）；测试断言 URL 占位串零命中 | closed |
| T-07-10d | Denial of Service | opener 僵尸驻留 | low | mitigate | goroutine Wait() 收割（Start 成功路径必达，f19be02） | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-07-01 | T-07-01c | token 出现在 Location 头属 P5 D-03 已接受暴露面（sharetoken.go:113 既有登记）；308 保方法是 PITFALLS 表正式规避 | plan 期裁决（07-01 D-13 记录） | 2026-08-26 |
| AR-07-02 | T-07-01d | Origin 头无 path 分量（origin.go:64-82 只看 scheme://host），base-path 不影响 Origin 判定（P3 D-12 张力已闭合） | plan 期裁决（07-01） | 2026-08-26 |
| AR-07-03 | T-07-02c | RESEARCH A5：Chmod/Chown 在 listening 打印前完成，窗口内无客户端被指引；本地同机攻击者模型内风险极低 | plan 期裁决（07-02） | 2026-08-26 |
| AR-07-04 | T-07-03d | 攻击者轮换 XFF 首 IP 可获独立节流配额——D-20 裁决接受（反代部署语义正确性优先）；无凭据反代部署本无节流面 | plan 期裁决（07-03 D-20） | 2026-08-26 |
| AR-07-05 | T-07-05c | D-26 裁决：operator 本机操作无泄露面（token 通道免交互即打即用是 P5 D-01 既定语义）；启动打印本已含同 token | plan 期裁决（07-05 D-26） | 2026-08-26 |
| AR-07-06 | T-07-05d | RESEARCH Pattern 7：Close 幂等，先到码分派两语义都正确（进程死 vs 服务关停） | plan 期裁决（07-05） | 2026-08-26 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-27 | 35 | 35 | 0 | codebuddy（verify:post 挂钩触发；ASVS L1 grep 级短路——register 全部 plan 期建档、threats_open=0，缓解锚点逐一核验在案） |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-27
