---
phase: 09-release-polish
plan: 07
subsystem: infra
tags: [docker, scratch, tini, pid1, zombie-reaping, systemd, unit, restart-semantics, deployment]

requires:
  - phase: 07-deployment
    provides: "D-22 stop-signal 进程组序列（tini 不加 -g 的依据）；07-05 Shutdown 1001 广播挂钩（KillSignal 默认 SIGTERM 正确的依据）"
  - phase: 08-observability
    provides: "08-03/08-05 /healthz draining 两态 + 实机 systemctl 通道先例（~/.config/systemd/user/wesh-uat.service——本 plan 实测通道的字面对应物）"
  - phase: 06-session-lifecycle
    provides: "OQ1 accept-255 退出码语义（Restart=on-failure 选型与 ExecMainStatus=255 断言的依据）"
provides:
  - "Dockerfile：FROM scratch + tini v0.19.0 sha256 双钉 + ADD --checksum/--chmod=755 + ENTRYPOINT tini 的参考镜像（用户自建形态，D-16 不发布镜像）"
  - ".dockerignore：构建上下文排除集（**/node_modules 等六项），红线守护不忽略根 wesh 二进制"
  - "deploy/wesh.service：D-17 全配 systemd unit 模板（Restart=on-failure/RestartSec=2/TimeoutStopSec=15s/LimitNOFILE=65536/EnvironmentFile 600 通道 + 255 交互与加固张力注释块）"
  - "两通道实测证据：docker 四面（PID 1=tini/收割+负对照/scratch 契约/bind-mount 工作形态）+ systemctl 五证据（200/255 复活/503 draining/停后不复活/零残留）——README 部署节（09-09）引用素材"
affects: [09-09（README 部署节 Docker/systemd 两小节引用本实测同源承诺语）, ship]

actuals:
  tokens: 3400
  tasks: 2
  commits: 3

tech-stack:
  added: [tini v0.19.0（镜像内 ADD，宿主零安装）]
  patterns:
    - "ADD --checksum + --chmod=755 组合：远程 URL 制品供应链钉死与执行位一并解决（scratch 零 RUN 约束下的唯一形态）"
    - "systemd 通道停窗口确定性夹具：trap 忽略 TERM+HUP（KillMode=control-group 使 systemctl stop 直 TERM 全 cgroup，只 trap HUP 结构性失效）+ --stop-timeout 3s"
    - "ExecMainStatus 证据捕获须在新进程启动前的 auto-restart 窗口内（systemd 在新主进程启动时归零该值）"

key-files:
  created:
    - Dockerfile
    - .dockerignore
    - deploy/wesh.service
  modified: []

key-decisions:
  - "ADD 远程 URL 默认落 0600 无执行位——RESEARCH 定稿缺 --chmod 实测命中（/tini permission denied exit 126）；ADD --chmod=755 修复（dockerfile:1 frontend 支持，验收 grep 面不变）"
  - "实机 systemctl 通道 = systemd --user（/etc/systemd/system 无 root 不可写、sudo 沙箱拒绝）——核读发现 08-05 先例 wesh-uat.service 即 user manager 单元，本通道即 plan『08-05 同通道』的字面对应物；Restart/255/stop 语义为管理器同源逻辑，user manager 等效受力"
  - "验收 grep 机械纪律第六次沿用：unit 加固张力注释以散文说明（家目录隔离/根文件系统只读化），不写 ProtectHome/ProtectSystem 字面（注释提及同样计数，验收 ==0 是源码级机械检查）"
  - "systemd 239 行为纹理记录：systemctl stop 下 wesh 退出 255 → ActiveState=failed（Result: exit-code）但绝不复活——manual stop 语义与 restart 抑制正交"

patterns-established:
  - "容器僵尸收割断言形态：递归 ps --ppid 后代扫描 + 负对照夹具（--entrypoint /wesh 无 init）证夹具有牙（fail-first 纪律）——正例 Z=0 仅在负对照 Z=5 在场时可信"

requirements-completed: [OPS-10]

coverage:
  - id: D1
    description: "Dockerfile（D-16 定稿形态 + --chmod 实证修正）+ .dockerignore 入库：scratch+tini sha256 钉死，本机 docker 24.0.6 构建实测四面全取证——PID 1=/tini（docker top 三层树）/孤儿收割 Z=0 + 负对照 Z=5 defunct/纯 scratch spawn bash 失败契约/bind-mount 三件套 listening 行 + bash -c echo ok exit_code=0"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "time CGO_ENABLED=0 go build -trimpath -ldflags=\"-s -w\" -o wesh ./cmd/wesh && time docker build -t wesh-test . && docker run --rm wesh-test --version（plan verify 块端到端 VERIFY_ALL_PASS）"
        status: pass
      - kind: other
        ref: "docker top wesh-t1：/tini(PID 2315150)→/wesh(2315178)→sleep 300(2315184)；递归扫描 wesh-t2 ZOMBIES=0 vs 负对照 wesh-t3 ZOMBIES=5（[sleep] <defunct>×5）"
        status: pass
      - kind: other
        ref: "docker run --rm wesh-test --bind 127.0.0.1 -- bash → exec: \"bash\": executable file not found in $PATH exit 1（契约行为）；bind-mount 形态 listening on http://127.0.0.1:43619 打印"
        status: pass
      - kind: other
        ref: "验收 grep：FROM scratch/ADD --checksum=sha256/ENTRYPOINT tini/tini-static 各 ==1；git status 零残留（wesh 二进制已 rm，镜像已 rmi）"
        status: pass
    human_judgment: false
  - id: D2
    description: "deploy/wesh.service（D-17 全配五键 + 255 交互/加固张力注释块）入库：systemd-analyze verify 零警告 + 实机 systemctl 五证据（start→healthz 200/SIGTERM 杀进程 ExecMainStatus=255→NRestarts=1 复活 200/stop 窗口 503 draining×49 轮询/stop+4s 不复活/清理零残留）"
    requirement: OPS-10
    verification:
      - kind: other
        ref: "systemd-analyze verify deploy/wesh.service 零输出（临时置 /usr/local/bin/wesh 供路径解析，验后移除）+ 验收 grep 1/1/1/0（Restart=on-failure/LimitNOFILE=65536/EnvironmentFile=- 各 1，加固指令字面 0）"
        status: pass
      - kind: integration
        ref: "systemd --user 实机实测（08-05 同通道）：E1 200 status ok；E2 auto-restart 窗口 ExecMainStatus=255 + NRestarts=1 + 复活 200；E3 503 draining×49（窗口 3.0s，stderr shutdown→session_end 恰 3.0s signal=SIGKILL）；E4 is-active=failed healthz=000 不复活；residue=0"
        status: pass
    human_judgment: false

duration: 28min
completed: 2026-08-29
status: complete
---

# Phase 9 Plan 07: 部署两件套（Dockerfile + systemd unit） Summary

**Dockerfile（scratch + tini v0.19.0 sha256 双钉 + ADD --checksum/--chmod=755）与 deploy/wesh.service（Restart=on-failure/LimitNOFILE=65536/EnvironmentFile 600 全配）入库：本机 docker 构建实测四面（PID 1=tini 收割经负对照 Z=5 证夹具有牙、纯 scratch spawn 失败契约、bind-mount 工作形态）与实机 systemctl 五证据（255 复活/503 draining×49/停后不复活）双双闭合——D-16/D-17 验证项全闭，README 部署节（09-09）引用素材就绪**

## Performance

- **Duration:** 28 min
- **Started:** 2026-08-29T15:28:27Z
- **Completed:** 2026-08-29T15:56:08Z
- **Tasks:** 2
- **Files modified:** 3（均新建）

## Accomplishments

- **Dockerfile 定稿入库并经本机构建实测**：`# syntax=docker/dockerfile:1` + FROM scratch + ARG TARGETARCH=amd64 + ARG TINI_SHA256（amd64/arm64 双钉值注释，升级同步义务登记）+ ADD --checksum=sha256 --chmod=755 拉取 tini-static + COPY wesh /wesh + EXPOSE 7681 + ENTRYPOINT ["/tini", "--", "/wesh"]（不加 -g——进程组信号由 wesh stop-signal 序列承担，孤儿由 PID 1=tini 收割）。构建 7.2s 全绿（A6 假设闭合：buildx 0.11.2 + dockerfile:1 frontend 支持 ADD --checksum；上下文 8.20MB 证 .dockerignore 生效）；镜像 SIZE=9066754（8.6MB）
- **docker 实测四面全取证**：a) docker top 三层树 /tini→/wesh→sleep 300；b) 孤儿收割正例 ZOMBIES=0（TREE_SIZE=4）+ 负对照（--entrypoint /wesh 直跑无 init）ZOMBIES=5（[sleep] <defunct>×5——fail-first 纪律：夹具有牙才信正例）；c) 纯 scratch `wesh -- bash` → `exec: "bash": executable file not found in $PATH` exit 1（契约行为，README 承诺语素材）；d) bind-mount 三件套 listening 行打印 + `bash -c 'echo ok'` 子进程 session_end exit_code=0
- **deploy/wesh.service 定稿入库**：[Unit] After/Wants=network-online.target；[Service] Type=simple + EnvironmentFile=-/etc/wesh/credentials（600 通道注释，unit 零秘密）+ ExecStart=/usr/local/bin/wesh --config /etc/wesh/wesh.toml + Restart=on-failure + RestartSec=2 + TimeoutStopSec=15s + LimitNOFILE=65536 + KillSignal 默认 SIGTERM 注释；255 交互注释块（on-failure 下自主终结会重启属服务形态期望、systemctl stop 永不触发重启、期望「会话完即停」改 Restart=no）+ 加固张力说明（散文形态，不写加固指令字面）；[Install] WantedBy=multi-user.target
- **systemd 双通道验证闭合**：systemd-analyze verify 零警告（临时置 /usr/local/bin/wesh 供 ExecStart 路径解析，验后移除）；实机 systemctl（--user 通道 = 08-05 wesh-uat.service 先例同通道）五证据——E1 start→/healthz 200 status ok（恰四键）；E2 kill -TERM MainPID→auto-restart 窗口 ExecMainStatus=**255**→NRestarts=1→复活 200（Pitfall 10 核心实证：on-failure 把 255 分类 failure，A5 MEDIUM 假设闭合）；E3 systemctl stop 窗口 50ms 轮询捕获 **503 draining×49**（body 恰四键 status=draining；stderr 事件 shutdown→session_end 恰 3.0s signal=SIGKILL——TimeoutStopSec=15s 覆盖 stop-timeout 3s 序列的实证）；E4 stop+4s is-active=failed healthz=000 不复活（systemd 发起的停止永不触发 Restart=）；清理零残留
- **零残留交付**：仓库根 wesh 测试二进制 rm、wesh-test 镜像 rmi、临时 /usr/local/bin/wesh 移除、user units 目录零 residue、/tmp 夹具脚本与日志全清——git status 干净

## Task Commits

Each task was committed atomically:

1. **Task 1: Dockerfile + .dockerignore 入库 + 本机 docker 构建实测四面** - `c355669` (feat)
2. **Task 2: deploy/wesh.service 入库 + systemd-analyze verify + 实机 systemctl 最小实测** - `920bcc3` (feat)

**Plan metadata:** 见本条之后的 docs 提交（SUMMARY/STATE/ROADMAP/REQUIREMENTS）

## Files Created/Modified

- `Dockerfile` — D-16 参考镜像（scratch + tini PID 1 收割；注释登记构建前置/arm64 构建命令/不加 -g 理由/镜像零命令契约/--socket 容器内语义）
- `.dockerignore` — 构建上下文排除六项（.git/.planning/**/node_modules/web/dist/dist/*.tar.gz）；红线守护注释——不忽略根 wesh 二进制
- `deploy/wesh.service` — D-17 systemd unit 模板（全配五键 + 255 交互/加固张力注释块）

## Decisions Made

- **ADD --chmod=755 补钉**（详见 Deviations #1）——ADD 远程 URL 默认落 0600，scratch 无 shell 无法 RUN chmod 补救；--chmod 是 dockerfile:1 frontend 内建能力，与 --checksum 同行进仓
- **实机 systemctl 通道取 systemd --user**：/etc/systemd/system 无 root 不可写、sudo 被沙箱拒绝；核读 ~/.config/systemd/user/ 发现 08-05 先例单元 wesh-uat.service 即 user manager——plan「08-05 同通道」的字面对应物就是 user manager，非降级而是对号；Restart=on-failure/exit-255 分类/manual-stop 抑制均为管理器同源逻辑，user/system 等效受力（LimitNOFILE=65536 在 hard cap 1000000 下无特权落地，模板 [Service] 行全量经受力）
- **E2/E3 夹具两处 systemd 通道适配**（详见 Deviations #4/#5）——ExecMainStatus 在 auto-restart 窗口内捕获；停窗口夹具 trap 须覆盖 TERM+HUP（KillMode=control-group 语义）
- **加固张力注释散文形态**：unit 注释说明「systemd 家目录隔离/根文件系统只读化等加固指令会破坏 shell 工作流，不做默认加固；NoNewPrivileges=yes 可自叠加」——不直写指令名（验收 grep ==0 机械纪律第六次沿用，must_have「说明注释」义务语义全额保持）

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RESEARCH/plan 定稿 Dockerfile 缺 --chmod=755——/tini 无执行位容器启动必败**
- **Found during:** Task 1（首跑 `docker run --rm wesh-test --version`）
- **Issue:** ADD 远程 URL 制品默认落盘 0600 无执行位；scratch 零 RUN（无 shell）无法 RUN chmod 补救——`OCI runtime create failed: exec: "/tini": permission denied` exit 126 实测命中。RESEARCH 定稿形态（plan must_have 逐字引用源）未含 --chmod，逐字执行必交付坏镜像
- **Fix:** ADD 行补 `--chmod=755`（dockerfile:1 frontend 内建支持，与 --checksum 同行）+ 注释登记理由
- **Files modified:** Dockerfile
- **Verification:** 重建 1.5s 后 --version 输出 `wesh dev` exit 0；实测四面全取证；验收 grep 四条全 ==1（`ADD --checksum=sha256` 前缀不变）
- **Committed in:** `c355669`（Task 1 提交）

**2. [Rule 1 - Bug] plan 实测 (a)(b) 容器命令字面缺 loopback 绑定——默认 0.0.0.0 无凭据拒绝启动**
- **Found during:** Task 1（实测 a 首跑容器秒退）
- **Issue:** plan 字面 `docker run -d wesh-test -- sleep 300` 走默认 bind 0.0.0.0:7681 且无凭据——Phase 3 既定启动安全矩阵拒绝（`refusing to listen on non-loopback address without credentials` exit 2），根本到不了 spawn sleep
- **Fix:** 实测 (a)(b)(c)(d) 容器命令统一补 `--bind 127.0.0.1`（容器内 loopback 免凭据过矩阵，与 plan verify 块 `--bind 127.0.0.1 --port 0` 形态同源）
- **Files modified:** 无（执行期命令修正；交付物不受影响）
- **Verification:** 修正后四面全取证（docker top 三层树/收割+负对照/契约/工作形态）
- **Committed in:** `c355669`（Task 1 提交）

**3. [Rule 1 - Bug] plan 实测 (b) 时序字面不可达——夹具 sleep 3 + 宿主 sleep 4 使扫描时容器已退出**
- **Found during:** Task 1（实测 b 夹具设计核读）
- **Issue:** plan 字面夹具 `...; sleep 3` + 宿主 `sleep 4 后 docker inspect`——主 bash 于容器内 t=3s 退出 → wesh 255 → 容器停；t=4s 时 State.Pid=0，ps 扫描结构性空转（证据真空）
- **Fix:** 夹具主命令改 `sleep 30`（孤儿生成脚本逐字不动），宿主 sleep 2 扫描（孤儿 t≈0.3s 死亡之后、容器存活期内），扫描窗从「不存在」变为 27s+
- **Files modified:** 无（/tmp 夹具脚本，不进仓）
- **Verification:** 正例 ZOMBIES=0 TREE_SIZE=4、负对照 ZOMBIES=5 双双确定性取证
- **Committed in:** `c355669`（Task 1 提交）

**4. [Rule 1 - Bug] E2 ExecMainStatus 在新进程启动后归零——证据捕获须移至 auto-restart 窗口**
- **Found during:** Task 2（实机首跑 E2 FAIL：ExecMainStatus=0）
- **Issue:** kill -TERM → wesh 3s 关停 → exit 255 → RestartSec=2 → 新进程启动时 systemd 归零 ExecMainStatus；plan 形态（sleep 4 后断言）读到的恒为 0——255 证据存在但读取时点错误（NRestarts=1 + 复活 200 佐证重启路径本身正确）
- **Fix:** kill 后 100ms 轮询 SubState，在 auto-restart 窗口内捕获 ExecMainStatus=255，再等复活断言 200
- **Files modified:** 无（/tmp 夹具脚本）
- **Verification:** 复跑 E2 ExecMainStatus=255 + NRestarts=1 + HTTP 200 三证据齐
- **Committed in:** `920bcc3`（Task 2 提交）

**5. [Rule 1 - Bug] E3 停窗口夹具只 trap HUP 在 systemd 通道下结构性失效——KillMode=control-group 直 TERM 全 cgroup**
- **Found during:** Task 2（实机首跑 E3 FAIL：503×0）
- **Issue:** 08-05 协议层夹具 `trap "" HUP` 防的是 wesh stop-signal；但 systemctl stop 默认 KillMode=control-group 直接 SIGTERM 全 cgroup——子进程 bash 被 systemd 的 TERM 瞬杀（stderr 实证：session_end signal=SIGTERM 距 shutdown 仅 0.4ms），wesh 生命周期即收口，draining 窗口 ≈0 不可观测
- **Fix:** 夹具改 `trap "" TERM HUP`——子进程挺过 systemd TERM 与 wesh HUP 两侧信号，--stop-timeout 3s 窗口确定性成立（50ms 轮询后台日志先行再发 stop 消竞态；轮询体每轮截断 body 文件防陈旧内容混入日志）
- **Files modified:** 无（/tmp 夹具脚本）
- **Verification:** 复跑 E3 503 draining×49（200×5→503×49→000 转换序列），stderr shutdown→session_end 恰 3.0s signal=SIGKILL（子进程走完 stop-timeout 全程被 wesh 补 KILL——07-05 序列在 systemd 通道下的完整实证）
- **Committed in:** `920bcc3`（Task 2 提交）

---

**Total deviations:** 5 auto-fixed（全 Rule 1——1 交付物修复 + 4 执行/夹具层修正；无 Rule 4 架构变更、无认证门、零包安装）
**Impact on plan:** 交付物形态与 plan must_haves 逐字一致（Dockerfile 仅多 --chmod=755 必要修复）；四处执行层修正全部使 plan 既定验证意图从「字面不可达/证据真空」变为「确定性取证」——无 scope creep。RESEARCH 登记项变化：A3（tini 只向直接子进程转发信号）经 docker top+收割实测闭合、A5（on-failure 分类 255/stop 不复活）经实机闭合、A6（ADD --checksum buildx 可用）经构建闭合——三 MEDIUM/LOW 假设全转实证。

## Issues Encountered

- **systemd-analyze verify 的 ExecStart 路径解析**：模板指向部署机路径 /usr/local/bin/wesh（本机未装 → verify 报 not executable exit 1）。本机 /usr/local/bin 恰为用户所有可写——临时构建置入供 verify 解析，验后立即移除（零残留纪律）；此环境纹理供 09-09 与后续 unit 变更复验参考
- **journalctl --user 不可读**（用户不在 systemd-journal 组）——改经 drop-in `StandardOutput/Error=file:` 落盘取 wesh 侧事件证据（shutdown/session_end 序列），比 journal 检索更直接
- **systemd 239 行为纹理（文档素材）**：systemctl stop 下 wesh 退出 255 → ActiveState=failed（Result: exit-code）但绝不复活——「failed」状态字面可能误导 operator 以为异常，09-09 README systemd 节可引用本实测说明（manual stop 后 failed 态 = 255 语义的正常纹理）

## Authentication Gates

None——纯本机执行（docker daemon 直访 + systemd --user），无认证门。

## Known Stubs

无——三交付物全部为经实测的真实配置（Dockerfile 四面实证、unit 双通道验证）；无 TODO/FIXME/占位值/空数据流。

## User Setup Required

None - no external service configuration required.（镜像不发布为 D-16 既定——用户按 Dockerfile 注释的构建前置自行构建；unit 安装为 operator 部署动作，模板自带安装注释行。）

## Next Phase Readiness

- **09-09 README 部署节素材就绪且与实测同源**：Docker 小节承诺语可直接引用——「本镜像不含任何可执行命令，`--` 后命令须来自 bind-mount 或 FROM 派生自建」（实测 c 契约行为）+ bind-mount 三件套形态（实测 d）+ 构建前置命令；systemd 小节可引用五证据 + 「manual stop 后 failed 态 = 255 语义正常纹理」说明 + KillMode=control-group 与 stop-signal 序列的交互纹理
- **D-16/D-17 验证项闭合**：ROADMAP SC3 部署文档两件的「文档即被测物」实证半侧完成（剩余 Caddy/CF 为 09-08 面）
- **OPS-10 需求面**：本 plan 交付 Dockerfile/.dockerignore 属 OPS-10 部署形态配套（发布链本体已于 09-01 闭合）
- **无阻塞项**

## Self-Check: PASSED

- 文件：Dockerfile ✓、.dockerignore ✓、deploy/wesh.service ✓、09-07-SUMMARY.md ✓（本文件落盘）
- 提交：c355669（Task 1）✓、920bcc3（Task 2）✓——git log --oneline 双 FOUND
- 验收命令全过：Task 1（plan verify 块端到端 VERIFY_ALL_PASS + grep 1/1/1/1 + 四面实测取证）✓；Task 2（systemd-analyze verify 零警告 + grep 1/1/1/0 + 实机五证据 SYSTEMD_LIVE_ALL_PASS）✓
- 零残留：git status 干净（无 untracked）、wesh-test 镜像已 rmi、/usr/local/bin/wesh 已移除、user units 目录 residue=0、/tmp 夹具全清 ✓

---
*Phase: 09-release-polish*
*Completed: 2026-08-29*
