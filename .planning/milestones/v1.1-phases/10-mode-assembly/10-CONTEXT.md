# Phase 10: 模式装配与接缝 - Context

**Gathered:** 2026-09-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 10 交付会话模式阀门与全部接缝一次装配（全部 inert）：`--session-mode=shared|per-client` flag + TOML `session-mode` 键 + parse 期枚举校验；`pty.StartWithSize` 导出（Start 委托，80×24 单一事实源纪律保持）；`Options.SessionMode/SpawnFunc` + `New` 互斥校验 fail-fast；`validateStartup` per-client 行（exec.LookPath 预检，SC4）+ write-policy warn 行（D-01/D-02）；配置 fuzz 语料扩展同 PR。默认 shared 逐字节零回归——本阶段结束时不存在任何 per-client 运行期行为，先锁定公开契约面（one-way flag 纪律），防散点 if/else 腐化（Pitfall 6 的唯一预防窗口）。

**In scope (from ROADMAP):** PC-01——`--session-mode` flag/TOML 键（默认 shared）；非法值 parse 期 exit 2 拒绝；优先级链 flag>env>TOML>默认对 session_mode 成立；per-client 启动预检（exec.LookPath 等 validateStartup 行）；pty.StartWithSize；Options.SessionMode/SpawnFunc + New 互斥校验；配置 fuzz 语料扩展（新键入白名单 + 非法值 parse 拒绝同 PR）；write-policy × per-client 组合处置（D-01/D-02 已裁决）。

**Out of scope (本阶段不做):** 任何 per-client 运行期行为（attach spawn/断开杀进程/EXIT 私有化——Phase 11）；resize 直通/ro 门控/重连 reset/停读续读（Phase 12）；Welcome 模式位下发与前端任何改动（Phase 12，本里程碑唯一前端改动面）；容量防线/stop-timeout 重议（Phase 13，裁决项①）；--once/exit-when-empty 第二终结源、metrics/审计 per-client 粒度、WESH_REMOTE_USER 注入（Phase 13，SEC-09 已裁决）；双模式 -race 门/协议 UAT/herdr UAT/标定回填（Phase 14）；per-client 完整模式语义文档段（Phase 14 PC-12）；env 兜底键（D-03 裁决不引入）。

**已锁定不重复决策：** TOML 平铺键 = flag 名（P7 D-03，29 键全连字符 + DisallowUnknownFields——ROADMAP/REQUIREMENTS 中 `session_mode` 下划线写法按此修正为 `session-mode`）；优先级链 flag>env>config>default（P7 D-05）+ fs.Visit 显式设置位合并（P7 D-02）；CLI flag 全名无短选项 + parse/validate 分层 + fail-fast（P2 D-15/P3）；启动面红线：凭据/token/文件内容永不回显（SEC-01/P4 记录式上报），非敏感枚举/路径值可回显（P5 write-policy %q/P7 --cwd 先例，CONFIGURATION.md:124 明示口径）；SpawnCols/SpawnRows 80×24 单一事实源（spawn.go:38-41，G-05-1）；Options 生产直传 + New 零值兜底注释先例（server.go:234+）；零新依赖（STACK.md：go-toml v2 既有机制覆盖新键）；全部 inert（ROADMAP 含：本阶段结束不存在任何 per-client 运行期行为）；D5/SEC-09 已裁决落定（STATE.md）——WESH_REMOTE_USER 归 Phase 13；每阶段收口闸 = shared 全量测试原样绿 + 期望值逐字未动（SUMMARY.md 方法论警告：最大风险 = 破坏既有不变量而不自知，禁止断言放宽成「两模式都接受」）。

</domain>

<decisions>
## Implementation Decisions

### write-policy × per-client 组合处置（ROADMAP 明示规划期裁决项，本讨论闭合）
- **D-01:** 组合处置 = **validateStartup warn 明示放行**：输出警告行（`--write-policy` 的 owner 仲裁/递补语义在 per-client 下不装配；ro/rw 权限级别仍按 ticket 生效）后正常启动。exit 2 拒绝被否决——ro/rw 级别仍被 ticket 消费（分享链接 = 按权限级别的独立进程入场券，FEATURES D2/T7），非 write-policy×writable 那样的纯配置矛盾；静默永不接受（ROADMAP 锁定）；warn 行先例既有（D-16 auth-header 暴露面警告同通道，warn 返回值机制现成） — **Reversibility:** one-way — validateStartup 行为契约：放行后收紧为拒绝会破坏依赖该组合启动的既有部署
- **D-02:** warn 触发锚定 **writePolicySet 显式设置位**：`--write-policy` 显式给出（owner|all 任一）× `--session-mode=per-client` 即 warn——owner|all 在 per-client 下均为真空语义，只 warn owner 不 warn all 是口径分裂；writePolicySet 字段已存在（main.go:43），配置来源同档（fc.WritePolicy 非 nil 即置位，07-06 合并收尾先例）——CLI 与 TOML 双源同档触发

### env 兜底键
- **D-03:** **不引入 `WESH_SESSION_MODE`**——env 层在 wesh 是敏感值专用通道（P3 D-01 WESH_CREDENTIAL 为 systemd EnvironmentFile=600 凭据不落盘场景设计）；session-mode 非敏感，CLI+TOML 已覆盖全部配置场景；SC3 的「env」层对无 env 键的 flag 真空成立（链形式保持），优先级链测试断言 flag>TOML>默认三层即可；不开「非敏感键也可 env」先例（27+ 非敏感键跟进压力）

### 枚举值回显口径（讨论中新浮出灰区——SC2 字面 vs 现行先例冲突裁决）
- **D-04:** 非法值报错**回显值**：`invalid --session-mode "banana": must be shared or per-client`——write-policy（main.go:619 `%q`）与 --cwd 路径回显先例同形态；TOML 源经 configErr 单写口同纪律（键名入文案合法）。SC2「错误文案不泄露用户输入值内容」解读为：凭据/token/文件内容红线保持（SEC-01 起源本义），枚举非敏感面豁免——与 CONFIGURATION.md:124「值域/枚举类 invalid …（值可回显，非敏感）」及 PITFALLS「值不敏感可回显」一致；值域是两个固定单词，用户输入无秘密可泄，回显助定位拼写错误（TOML 场景尤甚） — **Reversibility:** one-way — 错误文案形态被 Go 测试与 CONFIGURATION.md 校验矩阵表双重锁定，改口径动两个 face

### 文档面
- **D-05:** **最小明示**——`docs/CONFIGURATION.md` flag 表 + TOML 键表 + 校验矩阵表各加一行（注记「per-client 行为装配中，当前版本与 shared 等价」）；`README.md` 加一句同旨明示；`--help` 文案随 flag 注册同 PR。完整模式语义段留 Phase 14（PC-12）——flag 公开即文档义务（每 phase 收口先例），「装配中」注记防用户开了发现无新行为误以为 bug；不写完整语义段（行为尚不存在，文档先行于实现是漂移源）

### 验证面
- **D-06:** **零新 UAT 脚本**——Go 测试新增面（parse 枚举拒绝矩阵 CLI+TOML 双源 / 优先级链 flag>TOML>默认 / New 互斥校验 / fuzz 语料扩展 / StartWithSize 委托等价 / warn 触发双源两形态）+ 既有 phase02-09 协议 UAT 默认模式原样重跑 + -race 全量 = 零回归双证据；phase10.mjs 不建——全部 inert 无新协议行为可断言，新脚本只能重复既有脚本原样重跑已证明的等价性（SC1 锁定「既有协议 UAT 原样全绿」即此口径）

### Claude's Discretion
- `pty.StartWithSize` 精确签名（尺寸参数位置/类型）与 Start 委托形态、零值等价测试形态（TestStartZeroValueParity 先例）
- `Options.SessionMode` 字段类型（string vs 自定义枚举）与 `SpawnFunc` 签名形态（研究建议：run() 分岔处闭包捕获 argv+StartOptions）；New 互斥校验的失败形态（New 现无 error 返回——fail-fast 实现选型）
- run() 分岔点精确位置与 sess nil 语义（shared=pty.Start 直传现状行 / per-client=SpawnFunc 装配 inert 闭包）
- validateStartup per-client 行（SC4 exec.LookPath(argv[0]) 预检）的文案与落点（loopback 早退前同位先例；只读探测纪律内）
- warn 行精确文案（双 flag 名进文案纪律；auth-header 暴露面警告先例形态）
- fuzz 语料精确样本集（session-mode 合法/非法/边界形态）与 TestConfigMerge/Precedence/RedLines 的扩展断言面
- `--help` 文案措辞与 CONFIGURATION.md 三处加行的精确措辞
- Go 测试文件归属（main_test.go/config_test.go/fuzz_test.go 扩 vs 新文件）与 server 包互斥校验测试落点（export_test.go 先例）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 需求与路线图
- `.planning/ROADMAP.md` §Phase 10 — 成功准则 4 条（双源接受+零回归 / 非法值 parse 期 exit 2 / 优先级链 / LookPath 预检）与「含」清单（本 phase 全部交付物枚举）
- `.planning/REQUIREMENTS.md` §PC-01 — 需求原文（默认 shared、v1.0 行为逐字节不变）
- `.planning/PROJECT.md` §Current Milestone v1.1 — 里程碑目标（「装配期一次分岔、运行期零分岔」不抽象 session 接口）
- `.planning/STATE.md` §Blockers/Concerns — v1.1 规划期裁决项②（write-policy×per-client——本 phase D-01/D-02 闭合；①③ 归 Phase 13 不在本阶段，④ 经 Phase 11 D-03 提前消解）

### v1.1 调研结论（2026-09-02，HIGH 置信）
- `.planning/research/SUMMARY.md` §Phase 1（模式装配与接缝）— 阶段交付物与 Pitfall 6 先行理据；方法论警告（最大风险 = 破坏既有不变量而不自知；禁止断言放宽）
- `.planning/research/ARCHITECTURE.md` §11（修改文件清单——main.go/config.go/spawn.go/server.go 精确改动面）+ §12（PC-1 装配与阀门行）
- `.planning/research/FEATURES.md` §T1（session-mode flag 行）+ §Anti-Features A5（默认 shared）/A6（per-client 分支不装配 shared 组件）
- `.planning/research/PITFALLS.md` §P10 阶段映射 + Pitfall 6（接缝先行是唯一预防窗口）+ Pitfall 11（fuzz 语料/红线测试同 PR 纪律）+「配置面 × session_mode」行（值不敏感可回显——D-04 依据）
- `.planning/research/STACK.md` §go-toml v2 行 — 零新依赖定案（CLI>env>file 优先级与 DisallowUnknownFields 既有机制覆盖新键）

### 前序 phase 决策（机制先例）
- `.planning/milestones/v1.0-phases/07-deployment/07-CONTEXT.md` — D-01..D-07（TOML 机制全集：仅 --config 显式/合并语义/平铺键=flag 名/覆盖面/优先级链/严格模式/权限警告）；D-16（warn 通道先例）；validateStartup 组合校验落点先例
- `.planning/milestones/v1.0-phases/09-release-polish/09-CONTEXT.md` — D-09/D-10（fuzz 目标面与运行形态：FuzzDecodeFileConfig + testdata/fuzz 语料入库机制）

### 现状代码（扩展点）
- `cmd/wesh/main.go` — parseArgs（:181，flag 注册/fs.Visit:504/配置合并/枚举校验落点 :618-619 write-policy 先例）；writePolicySet 显式位（:43）；validateStartup（:930，warn/fail-fast 两位先例）；run()（:1156，pty.Start 直传现状行 :1212——分岔点）
- `cmd/wesh/config.go` — fileConfig 29 键（:47-79，全连字符命名；+session-mode 为第 30 键）；configErr 单写口
- `cmd/wesh/fuzz_test.go` + `cmd/wesh/testdata/fuzz/` — FuzzDecodeFileConfig 语料扩展点（09-02 机制）
- `cmd/wesh/main_test.go` / `config_test.go` — TestStartupMatrix/TestParseArgs/TestConfigMerge/Precedence/RedLines 测试宿主先例
- `internal/pty/spawn.go` — Start（:64）/StartOptions（:54）/SpawnCols-SpawnRows（:38-41，80×24 单一事实源）——StartWithSize 导出点
- `internal/server/server.go` — Options（:234）/New（:313）——SessionMode/SpawnFunc 字段与互斥校验宿主
- `internal/server/clients.go` — WritePolicyOwner/WritePolicyAll 常量（:75-76）——session-mode 枚举常量同形态参照
- `internal/server/export_test.go` — 内部件测试暴露先例
- `web/uat/phase02-09.mjs` — 既有协议 UAT 回归面（D-06 零回归脚本级证据，原样重跑零修改）
- `docs/CONFIGURATION.md` — flag 表（:56 行格式）/TOML 键表/校验矩阵表（:118-124）/优先级链文档——D-05 文档落点

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `config struct + fs.Visit 显式设置位 + 两阶段合并 + DisallowUnknownFields`（main.go:181-520 区、config.go:47-79）— session-mode 键宿主机制全现成：标量覆盖靠显式位，writePolicySet/maxClientsSet/exitEmptySet/portSet 四先例已立；fc.SessionMode 非 nil 即置位（07-06 合并收尾第一档同形态）
- `write-policy 枚举校验`（main.go:618-619）— parse 返回处枚举校验落点与 `%q` 回显文案先例（D-04 直接同形态）
- `validateStartup warn/fail-fast 双通道`（main.go:930+）— D-01 warn 行经 warn 返回值通道（auth-header 暴露面警告先例）；fail-fast 组合校验落点（loopback 早退前）先例
- `SpawnCols/SpawnRows 常量`（spawn.go:38-41）— StartWithSize 尺寸单一事实源（注释已预言本 phase 的导出形态：「StartWithSize 的 Winsize 字面量与……必须同源」）
- `configErr 单写口`（config.go）— TOML 源错误三要素形态（D-04 TOML 双源拒绝同纪律）
- `FuzzDecodeFileConfig + testdata/fuzz/`（fuzz_test.go，09-02）— session-mode 语料扩展机制现成（崩溃语料入库永久回归先例）
- `TestStartupMatrix/TestParseArgs/TestConfigMerge/Precedence/RedLines`（main_test.go/config_test.go）— 新键全部测试面的宿主先例
- `export_test.go`（server 包）— New 互斥校验测试的内部暴露先例

### Established Patterns
- **CLI flag 全名无短选项 + parse/validate 分层 + fail-fast**（P2 D-15/P3）— --session-mode 同纪律：枚举校验在 Parse 返回处（write-policy 同位），组合矛盾归 validateStartup
- **配置键存在即显式位**（07-06 合并收尾）— D-02 配置来源同档 warn 的机制依据
- **生产直传 + New 零值兜底注释分档**（Options.StopSignal/Version/CustomIndex 先例）— SessionMode 零值 = shared（零值等价纪律：v1.0 逐字节不变的结构性保证）
- **值域/枚举非敏感可回显**（CONFIGURATION.md:124 + write-policy/--cwd 先例）— D-04 口径来源
- **文档即被测物 + 每 phase README 收口** — D-05 最小明示的落点纪律（CONFIGURATION.md 表行 + README 一句，精确措辞为 Discretion）
- **每阶段收口闸：shared 全量测试原样绿 + 期望值逐字未动**（SUMMARY.md 方法论警告）— D-06 的零回归证明口径；禁止断言放宽成「两模式都接受」

### Integration Points
- `cmd/wesh/main.go parseArgs` — `--session-mode` flag 注册 + fs.Visit 显式位 + 配置合并（fc.SessionMode 非 nil 置位）+ Parse 返回处枚举校验（D-04 文案）
- `cmd/wesh/config.go fileConfig` — +`session-mode` 键（*string，29→30 键）
- `cmd/wesh/main.go validateStartup` — +writePolicySet×per-client warn 行（D-01/D-02）+ per-client LookPath(argv[0]) 预检行（SC4，命令缺失启动期暴露）
- `cmd/wesh/main.go run()` — 分岔：shared=pty.Start 直传（现状行 :1212）/ per-client=SpawnFunc 闭包捕获 argv+StartOptions（inert 装配；研究 §11 形态）
- `internal/pty/spawn.go` — +StartWithSize 导出，Start 委托（80×24 单一事实源纪律保持）
- `internal/server/server.go` — Options +SessionMode/SpawnFunc 两键；New 互斥校验 fail-fast（SessionMode=per-client × SpawnFunc=nil 拒绝；SpawnFunc≠nil × SessionMode=shared 拒绝——ROADMAP 含锁定）
- `cmd/wesh/fuzz_test.go` + `testdata/fuzz/` — session-mode 键语料扩展（合法/非法/边界；Pitfall 11 同 PR 纪律）
- `docs/CONFIGURATION.md` + `README.md` — D-05 三处表行 + 一句明示

</code_context>

<specifics>
## Specific Ideas

- **「warn 或拒绝，规划期裁决——静默永不接受」**（ROADMAP 原文）——本讨论 D-01/D-02 落定：warn 明示放行 + 显式设置位锚定；用户否决 exit 2 的关键理据是「ro/rw 权限级别仍被 ticket 消费，非纯配置矛盾」——write-policy×writable 的 fail-fast 先例不适用于部分语义失效场景
- **SC2 字面 vs 先例的张力裁决口径**（D-04）——「启动面红线保持」= 凭据/token/文件内容不回显（SEC-01 本义）；枚举非敏感面按 P5/P7 豁免先例回显。此解读记录防下游 agent 再撞同一冲突（PITFALLS 研究与 CONFIGURATION.md:124 均支持回显）
- **TOML 键名修正记录**：ROADMAP SC1/SC3 与 REQUIREMENTS PC-01 写 `session_mode`（下划线）——以 P7 D-03 锁定惯例为准修正为 `session-mode`：29 个既有键全连字符命名，且 DisallowUnknownFields 严格模式下 `session_mode` 会被当未知键拒绝。下游 agent 实现与测试一律用 `session-mode`
- **「装配期一次分岔、运行期零分岔」**：per-client 分支挂点唯一化（Options.SessionMode/SpawnFunc + New 互斥校验）是本阶段防 Pitfall 6（模式分支漂移）的核心动机——接缝先行是唯一预防窗口，散点 if/else 一旦落地收编成本随时间递增
- **env 键否决的深层理据**（D-03）：env 层在 wesh 的定位是「敏感值不落盘」专用通道而非通用配置层——为单个非敏感键开先例会把 27+ 非敏感键的 env 跟进变成开放问题；SC3 的 env 层真空成立是用户认可的链形式解读

</specifics>

<deferred>
## Deferred Ideas

- **`WESH_SESSION_MODE` env 键** — D-03 裁决不引入；添加为放松变更向后兼容，若容器/systemd 部署出现真实 env 注入需求再评估
- **phase10.mjs 协议 UAT** — D-06 裁决不建；per-client 真实协议行为 UAT 随 Phase 11+ 建设（phaseNN.mjs 编号随 roadmap）
- **per-client 完整模式语义文档段** — Phase 14（PC-12）：分享链接=独立进程入场券、ro=自有进程输入门控、herdr/tmux 汇聚叙事
- **write-policy warn→reject 收紧** — D-01 放行后收紧为拒绝属行为破坏（Reversibility 注记）；仅真实配置漂移事故支撑时重议

</deferred>

---

*Phase: 10-mode-assembly*
*Context gathered: 2026-09-02*
