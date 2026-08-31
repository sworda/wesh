---
phase: 09-release-polish
reviewed: 2026-08-30T11:04:07Z
depth: standard
files_reviewed: 29
files_reviewed_list:
  - cmd/wesh/config.go
  - cmd/wesh/config_test.go
  - cmd/wesh/fuzz_test.go
  - cmd/wesh/main.go
  - cmd/wesh/main_test.go
  - deploy/wesh.service
  - Dockerfile
  - .dockerignore
  - .github/workflows/ci.yml
  - .github/workflows/release.yml
  - .goreleaser.yml
  - internal/proto/fuzz_test.go
  - internal/server/clients.go
  - internal/server/customindex_test.go
  - internal/server/emptyexit_test.go
  - internal/server/export_test.go
  - internal/server/load_test.go
  - internal/server/server.go
  - README.md
  - scripts/release.sh
  - web/dist/index.html
  - web/embed.go
  - web/index.html
  - web/src/main.ts
  - web/uat/phase06-dom.mjs
  - web/uat/phase09.mjs
  - web/uat/pw/phase09-caddy-ctl.sh
  - web/uat/pw/phase09-caddy-pw.mjs
  - web/uat/pw/README.md
findings:
  critical: 0
  warning: 5
  info: 4
  total: 9
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-08-30T11:04:07Z
**Depth:** standard
**Files Reviewed:** 29（diff base = a7110194^，即 09-01 首提交的父提交）
**Status:** issues_found

## Summary

对 phase 9（release-polish）全部 29 个变更文件做了 standard 深度评审：逐文件通读 Go/TS/Shell/YAML 源码，交叉核对跨模块事实（`WithCustomIndex` 装饰链与 sharetoken.go/basicAuth 的装配序、release.sh 闸门与 git 语义、dist 产物与源码新鲜度、.goreleaser.yml 键名与上游 schema）。

**总体评估**：核心代码路径质量高——TOML 配置层（指针标量 nil/零值区分、值剥离红线）、`--index` 自定义首页装饰器（gzip 预压 + Vary + 安全头同源）、三处 UI 告警清除（role=alert、C-10 条件句式、pre-onopen 1001 分派）、fuzz 目标与负载矩阵测试的断言面均经交叉验证未见缺陷；`.goreleaser.yml` 的 `checksum:`/`archives.formats` 键名已对照 goreleaser 上游 schema 确认有效；提交入库的 `web/dist/index.html` 已验证为新鲜产物（含全部 phase-9 前端字符串，`index.html.gz` 解压后与 index.html 逐字节一致）。

**未发现 Critical 级问题**。发现集中在发布链闸门与交付物一致性：release.sh 的「与远端同步」闸门因 `git fetch --dry-run` 不更新远端跟踪引用而失效；README 声称提交了 `web/dist/*.gz` 但 `.gitignore` 明确忽略它（裸 clone 构建出的二进制不含预压资产，与 README 承诺相悖）；`loadCustomIndex` 存在 `int64(max)+1` 溢出边界；Caddy UAT rig 硬编码内网 IP 且两侧凭据来源不一致。

## Narrative Findings (AI reviewer)

### Warnings

#### WR-01: 发布前置闸④「与远端同步」实际不校验远端真实状态

**File:** `scripts/release.sh:63-75`
**Issue:** 闸④用 `git fetch --dry-run` 做网络连通探测，但 `--dry-run` **不更新任何远端跟踪引用**（git 语义：show what would be done, without making any changes）。随后的 `behind=$(git rev-list --count 'HEAD..@{u}')` 计算的是 HEAD 落后于**本地陈旧的 `origin/<branch>` 引用**的提交数——若远端在上次真实 fetch 之后新增了提交，`behind` 仍为 0，闸门放行。注释声称「落后/分叉即拒」，实际只在「上次 fetch 已知落后」时才拒；发布者据此以为已同步，tag 可能打在缺少远端最新提交的旧代码上。
**Fix:**
```bash
# 把 --dry-run 换成真实 fetch（更新远端跟踪引用后再比较）；失败降级语义保持不变
if ! git fetch >/dev/null 2>&1; then
    echo "release: upstream check skipped (no network or upstream)"
    return 0
fi
```

#### WR-02: 脏树闸③先于 `pnpm build` 执行，web/dist 与源码的漂移零检测

**File:** `scripts/release.sh:57-59, 85-88`
**Issue:** 闸③（工作树干净）在 `preflight` 阶段执行，而 `build_web`（重写被 git 跟踪的 `web/dist/index.html`）在其之后运行。README §构建 明确警告「修改 web/ 前端源码后必须先重新 build 再 go build，否则二进制内嵌旧产物」——但发布脚本对「已提交的 dist 是否与当前前端源码一致」没有任何校验：源码改了、dist 忘记重建并提交，四闸全绿照常发 tag。发布产物本身不受影响（release.yml 在 CI 里重建 dist），但仓库内承诺的「dist 即产物」契约破坏且无告警，本地 `go build` 分发的二进制内嵌旧前端。
**Fix:** build_web 之后补一道 dist 漂移闸：
```bash
build_web() {
    pnpm -C web install --frozen-lockfile
    time pnpm -C web build
    # dist 漂移闸：构建重写了被跟踪文件 → 已提交 dist 与源码不一致，发布前须提交新产物
    if [ -n "$(git status --porcelain web/dist)" ]; then
        die "web/dist differs from committed artifact; rebuild output must be committed before release"
    fi
}
```

#### WR-03: README 承诺「提交了 dist 的 .gz」与 .gitignore 事实相悖——裸 clone 构建缺预压资产

**File:** `README.md:96`（佐证：`.gitignore:2` = `web/dist/*.gz`；`git ls-files web/dist/` 仅含 `index.html`）
**Issue:** README §构建 写「仓库提交了前端构建产物（`web/dist/index.html` **及其 `.gz`**，由 `go:embed` 嵌入二进制）——裸 clone 即可直接 `go build` 并运行」。实际 `.gitignore` 明确忽略 `web/dist/*.gz`，仓库只提交了 `index.html`。后果：裸 clone 后本地 `go build` 出的二进制里 `web.Handler()` 的 `.gz` 旁路（embed.go:39-49）永不命中，内建页对 gzip 客户端始终明文伺服（功能可用但行为与 release.yml 构建的发布二进制不一致——发布产物因 CI 先跑 `pnpm build` 而含 .gz）；embed.go 自己的注释只承诺「提交 index.html 占位」，README 是超出事实的声明。该文案属本 phase（09-09）README 更新交付物的一部分，属交付物事实性错误。
**Fix:** 二选一保持文档与仓库一致：(a) 从 `.gitignore` 移除 `web/dist/*.gz` 并提交该产物（与 README 承诺对齐，裸 clone 构建行为与发布一致）；或 (b) 修正 README 该句为「仓库提交 `web/dist/index.html` 占位（`.gz` 由 `pnpm -C web build` 生成，发布构建在 CI 侧完成）」。

#### WR-04: Caddy UAT rig 硬编码内网 IP、不支持 SSH 端口、两侧凭据来源不一致

**File:** `web/uat/pw/phase09-caddy-pw.mjs:21-22, 26, 29`；`web/uat/pw/phase09-caddy-ctl.sh:58`
**Issue:** 三处叠加的可复现性缺陷：(1) `const SSH = '9.134.229.124'` 与 `BASE = 'http://9.134.229.124:10014'` 硬编码某台 Linux 开发机的内网 IP，完全绕过本目录 README 与 `lib/server.mjs` 文档化的 `WESH_UAT_SSH`/`WESH_UAT_TARGET_HOST` 环境变量机制——换机器/换 IP 即失效，换人不可运行；(2) `ssh -o BatchMode=yes ${SSH}` 不支持 `WESH_UAT_SSH_PORT`（README 明示该变量，phase07 系脚本支持）；(3) 凭据来源分叉：pw 侧 `AUTH_HEADER` 经 `lib/browser.mjs` 的 `CRED = process.env.WESH_UAT_CRED || 'user:pass'`（可被环境变量覆盖），ctl 侧硬编码 `--credential user:pass`——operator 按 README 设置 `WESH_UAT_CRED` 后 T1/T2 的带凭据请求必 401，rig 静默变红。
**Fix:** 与 lib 既有机制对齐：
```js
const SSH = process.env.WESH_UAT_SSH || '';
const SSH_PORT = process.env.WESH_UAT_SSH_PORT || '22';
if (!SSH) throw new Error('WESH_UAT_SSH 未设置（见 web/uat/pw/README.md）');
const ssh = (cmd) => execSync(`ssh -o BatchMode=yes -p ${SSH_PORT} ${SSH} ${JSON.stringify(cmd)}`, ...);
```
凭据两侧统一：ctl 脚本改读 `WESH_UAT_CRED` 环境变量（经 ssh 传递或写入一次性 EnvironmentFile），或 pw 侧固定使用默认值并在 README 载具行注明「WESH_UAT_CRED 不适用于 phase09 rig」。

#### WR-05: loadCustomIndex 的 `int64(max)+1` 在 index-max-size 极值处溢出——静默伺服空白页

**File:** `cmd/wesh/main.go:1135`
**Issue:** `io.ReadAll(io.LimitReader(f, int64(max)+1))`：TOML 里 `index-max-size = 9223372036854775807`（MaxInt64，validateStartup 的 `<=0` 闸放行）时 `int64(max)+1` 回绕为负，`io.LimitedReader` 对 `N <= 0` 立即返回 EOF——ReadAll 得到 0 字节，`len(data) > max` 不成立，函数返回空 data：wesh 以 exit 0 正常启动并**对全部页通道伺服 200 空 body**，而非报错或读入文件。运维把上限设为「实际无限大」这一自然笔误得到最难排查的静默失败形态（无任何错误行）。
**Fix:** 在 validateStartup 对上限做合理上界钳制，或在 loadCustomIndex 做防溢出读取：
```go
// validateStartup 中（与 <=0 拒绝同位）：
if cfg.indexMaxSize > 1<<31-1 { // 2GiB 硬顶：防 int64(max)+1 溢出与无界读入
    return "", errors.New("invalid index-max-size: exceeds 2GiB cap")
}
```

### Info

#### IN-01: WithCustomIndex 对任意 HTTP 方法都回页——与内建页 FileServerFS 的非 GET 行为漂移

**File:** `web/embed.go:103-122`
**Issue:** 装饰器对 `name == "index.html"` 的请求不检查 `r.Method`：`POST /`、`PUT /index.html` 等均返回 200 + 自定义页字节；未配置 `--index` 时同路径走 `http.FileServerFS`，对非 GET/HEAD 返回 405。行为面（POST / 的状态码）随 `--index` 开关漂移，且该面无测试覆盖（customindex_test/phase09.mjs 均只测 GET）。无安全影响（纯读页面），属未定义面。
**Fix:** 装饰器入口补方法闸与 FileServerFS 语义对齐：
```go
if r.Method != http.MethodGet && r.Method != http.MethodHead {
    w.Header().Set("Allow", "GET, HEAD")
    http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
    return
}
```

#### IN-02: systemd unit 模板的 ExecStart 依赖 /etc/wesh/wesh.toml 存在；TimeoutStopSec 与 --stop-timeout 存在隐性耦合

**File:** `deploy/wesh.service:13, 21`
**Issue:** (1) 模板默认 `ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml`——operator 只拷 unit 不建配置文件时，wesh 以 exit 2（配置文件不存在）退出，`Restart=on-failure` 下每 2s 重启直至 systemd 默认 StartLimit 速率限制进入 failed 态；模板注释未提示该依赖。(2) `TimeoutStopSec=15s` 上界对默认 `--stop-timeout 0` 成立，但配置文件里若设 `stop-timeout > 15s`，systemd 会在 wesh 自身宽限到期前 SIGKILL 整个 cgroup，自管 stop-signal 序列被截断——注释只论证了 15s 覆盖默认形态。
**Fix:** unit 注释补两句：「ExecStart 依赖 /etc/wesh/wesh.toml 存在（缺文件 exit 2 触发重启，直至速率限制）」；「若配置 stop-timeout > 15s，须同步调大 TimeoutStopSec」。

#### IN-03: Caddy UAT rig 以命令行 `--credential user:pass` 启动 wesh——与项目自身「凭据勿走 ps 可见面」指引相悖

**File:** `web/uat/pw/phase09-caddy-ctl.sh:58`
**Issue:** wesh README/flag help 反复强调「flag 值对同机用户可见（ps），生产建议用 WESH_CREDENTIAL env」。测试 rig 用一次性弱凭据、暴露窗口仅实证期间，风险可接受，但 rig 语义上可零成本对齐自身指引。
**Fix:** ctl 脚本改用 `WESH_CREDENTIAL=user:pass nohup $WESH_BIN ...`（env 前缀传递，不进 argv）。

#### IN-04: pw 脚本 ssh 通道用 JSON.stringify 当 shell 引用——未来命令含 $/反引号时会被远端 shell 展开

**File:** `web/uat/pw/phase09-caddy-pw.mjs:29`
**Issue:** `` execSync(`ssh ... ${JSON.stringify(cmd)}`) `` 产出双引号串，远端 shell 对双引号内的 `$VAR`、反引号做展开。当前 cmd 均为静态安全串（`bash /tmp/... setup` 等），无实际注入；属潜在模式缺陷——后续往 cmd 里拼动态值（如含 `$` 的路径/凭据）时会静默变形。
**Fix:** 用单引号包裹并转义单引号：`"'" + cmd.replace(/'/g, `'\\''`) + "'"`，或经 `bash -s` + stdin 传命令体（彻底不经远端 shell 解析参数）。

---

_Reviewed: 2026-08-30T11:04:07Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
