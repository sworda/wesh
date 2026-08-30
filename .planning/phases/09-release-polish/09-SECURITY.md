---
phase: 09
slug: release-polish
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-30
---

# Phase 09 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

> 审计口径：10/10 PLAN 含 `<threat_model>` 块（register_authored_at_plan_time: true）。
> 原始 STRIDE 条目 44 行（T-09-SC 供应链威胁在各 PLAN 重复声明 10 次），去重后 35 个唯一威胁。
> L1（grep-depth）分类证据源：09-VERIFICATION.md 独立复演（行为面不以 SUMMARY 声明计） +
> 2026-08-30 verify-work 自动化复演（全量 go vet+test 五包绿 / darwin 双架构 Mach-O 断言 /
> CI macos-latest 双 run 全绿 / release.sh bash -n）。asvs_level 1 + threats_open 0 +
> plan-time register → 按 secure-phase workflow §3 short-circuit 跳过深层 auditor。

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CI runner → GitHub Release | GITHUB_TOKEN 创建发布物——权限最小化是唯一闸门 | 发布产物（公开） |
| 第三方 Action/goreleaser/tini 下载 → 构建环境 | 供应链投毒面（发布物直接被用户 scp 运行） | 构建工具链（完整性关键） |
| git tag → 版本史 | 错 tag/脏树发布会把错误产物钉进版本史 | 发布元数据 |
| 用户自定义首页文件 → wesh 伺服面 | --index 读入的用户字节进 HTTP 响应 | 用户内容（不可信输入） |
| 配置文件/TOML → 进程凭据 | credential 解析错误回显面 | Basic 认证凭据（敏感） |
| 反代（nginx/Caddy/Cloudflare）→ wesh | 空闲超时/Host 语义差异面 | 终端流量 |
| 负载/fuzz 输入 → 解码器 | 畸形帧/TOML panic 面 | 远程不可信输入 |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-09-01a | Tampering | Action/goreleaser 下载供应链投毒 | high | mitigate | 全 Action 钉版（@v7.0.1/@v6.0.10/@v4/@v7.0.0/@v7.2.3）；goreleaser 内建 checksums 自校验；RESEARCH 制品审计 OK | closed |
| T-09-01b | Tampering | 脏树/错 tag 发布漂移 | medium | mitigate | release.sh 四闸（worktree 干净/tag 形态/tag 不存在/远端同步）+ 确认闸；本 session bash -n 复演 PASS | closed |
| T-09-01c | Elevation of Privilege | GITHUB_TOKEN 权限过大 | medium | mitigate | release.yml permissions 仅 contents: write 单键于 release job | closed |
| T-09-01d | Information Disclosure | 发布物/CI 日志携带敏感值 | low | mitigate | workflow 零秘密；tar 内容白名单三件套机械断言（本 session tar 复演吻合） | closed |
| T-09-SC | Tampering | npm/pip/cargo 包安装供应链（全 plan 声明） | high | mitigate | 零新运行时依赖（前端/Go 双侧）；goreleaser 经 RESEARCH 审计 | closed |
| T-09-02a | Denial of Service | 畸形帧/畸形 TOML 解码 panic | high | mitigate | FuzzDecodeHello/Resize/FileConfig 探针反断言；验证器 3.46M/142K execs 零崩溃 | closed |
| T-09-02b | Information Disclosure | TOML 错误文本回显 credential 值 | high | mitigate | 值剥离红线（错误行仅键名/类别/位置）；fuzz 探针反断言过 | closed |
| T-09-02c | Tampering | decodeFileConfig 接缝重构漂移 | medium | mitigate | io.Reader 接缝签名 + 行为锁测试；全量测试绿（本 session 复演） | closed |
| T-09-02d | Denial of Service | fuzz CI 时间预算失控 | low | mitigate | ci.yml 2×60s 钉死（:41-42）；本 session CI 双 run 全绿 | closed |
| T-09-03a | Tampering | 关停文案分叉（双写口漂移） | medium | mitigate | HINT_SHUTDOWN 常量 + showShutdown 单写口（pre-onopen/steady 同 helper） | closed |
| T-09-03b | Information Disclosure | 面板渲染远端内容钓鱼 | low | mitigate | 静态编译期文案常量，非运行时远端内容 | closed |
| T-09-03c | Denial of Service | role="alert" assertive 被 1Hz 重连刷屏 | low | accept | plan 期裁决接受（见 Accepted Risks Log） | closed |
| T-09-04a | Information Disclosure | 启动错误行回显自定义页内容字节 | high | mitigate | 错误文案仅路径+类别+上限值；phase09.mjs S1 探针反断言过 | closed |
| T-09-04b | Denial of Service | 误指巨大文件读入 OOM | high | mitigate | IndexMaxSize LimitReader（默认 16MiB）+ WR-05 修复 2GiB 上界钳制（c667743 后 084dde7） | closed |
| T-09-04c | Tampering | 自定义页被注入/模板化 | medium | mitigate | WithCustomIndex 整页 byte-identity 替换，零拼接；UAT S2 断言过 | closed |
| T-09-04d | Elevation of Privilege | --index 绕过认证/门禁面 | high | mitigate | 装饰器单点在 Handler() 认证之后；TestCustomIndex 认证面子测 + UAT S4 全链过 | closed |
| T-09-04e | Information Disclosure | 自定义页相对路径资源被误伺服 | low | mitigate | 单文件整页替换语义（无目录伺服面） | closed |
| T-09-05a | Information Disclosure | share token/凭据进 UAT check detail/日志 | high | mitigate | phase09.mjs sensitiveTokens 红线闭包（18/18 含 SEC 输出自净） | closed |
| T-09-05b | Denial of Service | 测试实例泄漏（端口/僵尸进程） | medium | mitigate | UAT 脚本 spawn/清理纪律；本 session 全量 UAT 脚本运行零残留 | closed |
| T-09-06a | Denial of Service | load 测试进常规 CI | high | mitigate | load_test.go 首行 //go:build load 隔离；常规全量测试无 load 族 | closed |
| T-09-06b | Denial of Service | 洪水/建销负载拖垮开发机 | medium | mitigate | -timeout 钉死 + 内存上界断言（19.8MiB≤64MiB 实测） | closed |
| T-09-06c | Tampering | 标定数据失真（flaky 断言误判） | medium | mitigate | kicks/gate/Alloc 三断言精确计数（非弱化计数）；5/5 族 PASS | closed |
| T-09-07a | Tampering | tini 下载投毒（供应链进 PID 1） | high | mitigate | Dockerfile ADD --checksum sha256 双钉 tini v0.19.0 | closed |
| T-09-07b | Information Disclosure | systemd unit 携凭据 | high | mitigate | deploy/wesh.service 零秘密（EnvironmentFile=-600 通道外部化） | closed |
| T-09-07c | Denial of Service | Restart 语义误判（always 复活/stop 卡死） | medium | mitigate | Restart=on-failure + TimeoutStopSec=15s；09-07 五证据实测（255 复活/503 draining/停后不复活） | closed |
| T-09-07d | Elevation of Privilege | 容器内进程逃逸面 | low | mitigate | scratch 空镜像 + tini PID 1 收割；无 shell 工具面 | closed |
| T-09-08a | Tampering | 反代配方互抄漂移（Host/超时语义） | medium | mitigate | README 实证分级（nginx/Caddy 已实证、Cloudflare 显著标注未实测）+ 关系行落版 | closed |
| T-09-08b | Information Disclosure | Caddy UAT rig 凭据入库 | high | mitigate | WR-04 修复（4c25f67）环境变量单一事实源；grep 断言过 | closed |
| T-09-08c | Elevation of Privilege | LAN 监听形态被反代暴露无凭据实例 | medium | mitigate | 认证面默认照旧；UAT S4 + Caddy 401→200 全链断言过 | closed |
| T-09-09a | Tampering | 脏树/错 tag 发布漂移（09-09 面） | high | mitigate | release.sh 四闸（同 T-09-01b，09-09 干跑四态已验证） | closed |
| T-09-09b | Information Disclosure | README/脚本泄露敏感值 | medium | mitigate | 文档 grep 断言组过；本 session CI 日志无泄漏 | closed |
| T-09-09c | Denial of Service | 发布脚本长 fuzz/负载失控 | low | mitigate | timeout 钉死（fuzz 2×10min、load 30m） | closed |
| T-09-09d | Tampering | 文档承诺与实测漂移 | medium | mitigate | 实证分级纪律（「已实证/未实测」显式标注）；挂账语 grep 零残留 | closed |
| T-09-10a | Tampering | 未验证/陈旧状态被发布 | high | mitigate | release.sh 六段式前置（四闸→vet+race→build→fuzz→load→确认闸） | closed |
| T-09-10b | Elevation of Privilege | 发布动作越权 | medium | mitigate | git 既有认证通道（ssh）；无独立凭据面 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-09-01 | T-09-03c | role="alert" assertive 播报在 1Hz 重连期间可能重复播报——影响面为屏幕阅读器用户的冗余播报（非功能性/非安全性），plan 期（09-03）裁决接受，避免为门控播报引入状态机复杂度 | plan 09-03 threat_model（disposition: accept） | 2026-08-29 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-30 | 35（唯一；原始条目 44 含 SC×10 重复） | 35 | 0 | Claude (gsd-secure-phase, L1 short-circuit) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-30
