---
phase: 05-multi-client
plan: 06
subsystem: api
tags: [go, share-link, capability-url, subtle, mux-wildcard, basic-auth, ticket, multi-client, race]

# Dependency graph
requires:
  - phase: 05-multi-client plan 03
    provides: decideModeLocked 模式判定矩阵（token 绑定 mode 直接喂矩阵——ro token × 任意 → ro 行已覆盖）+ prefs 双档 + WritePolicy 常量
provides:
  - shareTokens 两条目 store（启动预哈希 SHA-256 + lookup 位或累积不短路）+ GenerateShareToken（crypto/rand 16B → base64url 22 字符）
  - GET /s/{token}/ 页面门禁（有效委托 embed 绕 Basic / 无效委托 / 链 401 challenge 计 D-08）+ path-only 405 fallback（Allow: GET）——凭据与无认证双模式注册（OQ1 正交）
  - POST /api/attach shareAttach token peek 分支（MaxBytesReader 4KiB + JSON{token}，命中按绑定 mode 签发绕过 Basic/throttle，未携/错 token 恢复 body 委托原链无 oracle）
  - 无认证模式 /api/attach 携 token 非 404（OQ1）；checkTicket 无认证携票必核销 + throttle nil 守卫
  - 启动打印 share read-only/read-write 两行（rw 行仅 --writable，D-05）+ outboundIPv4 UDP-dial 路由感知回填（双 fallback 链）
  - TestShareToken（lookup 矩阵 + /s/ 门禁 200/401/405/307 + attach 分支 mode 绑定/无 oracle/无认证正交，VALIDATION 05-02-01 Go 侧）
affects: [05-08 前端（/s/{token}/ 提取 + attach body 携 token + 升格翻转）, 05-09 README（反代访问日志 token 脱敏建议 + TLS 部署建议 + 暴露面清单，T-05-06/T-05-06e 残余登记）, 05-09 UAT phase05.mjs（stdout 链接解析断言锚点）]

# Actuals (#2632)
actuals:
  tokens: 10815
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "capability 通道装配形态：token peek 包装 handler 在原守卫链（Origin→throttle→Basic）之前——命中按绑定 mode 直签（绕过 throttle 是刻意 capability 语义，R-03：避免 NAT 出口 IP 误伤持票旁观者），未携/错 token 恢复 r.Body 后委托原链（401 同文同码无 oracle，失败经 recordFail 自然计入 D-08 统一计数器，零新代码）"
    - "sharePage 双层委托：命中/未命中均改写 r.URL.Path=/ ——命中委托 embed handler 裸 wh（token 即凭证绕 Basic），未命中委托 / 链 root（凭据模式 basicAuth 在文件伺服前拦截 401 逐字节不变；无认证模式 root=wh 给 index.html——不改写会落 FileServerFS 404 而非 plan 锁定的『直接给页』）"
    - "token 比较纪律：SHA-256 等长化（Pitfall 1 长度侧信道）+ ro/rw 两组 ConstantTimeCompare 位或累积不短路（耗时与命中哪个 token 正交——ro/rw 公开语义无需组序号防泄露，auth.go 修正形态同源）"
    - "mux 1.22+ 通配三坑处置：GET /s/{token}/ + path-only 405 fallback（Allow: GET——内建 405 被 / 子树吞掉，P3 同款纪律）；尾斜杠匿名多段通配（PathValue 恒取首段）；无尾斜杠内建 307 补斜杠（见 Deviation 2 实证修正）"

key-files:
  created:
    - internal/server/sharetoken.go
    - internal/server/sharetoken_test.go
  modified:
    - internal/server/server.go
    - internal/server/tickets.go
    - cmd/wesh/main.go
    - .planning/phases/05-multi-client/deferred-items.md

key-decisions:
  - "[Phase 05-06]: sharePage 有效 token 委托 embed handler（wh）而非 / 链根（root=basicAuth(wh)）——Task 2 初版委托 root 使有效 token 反收 401（TestShareToken 首跑捕获）；token 即凭证绕过 Basic 是 D-01 第三通道的存在意义；无效 token 同样改写 / 后委托 root（凭据模式 401 逐字节不变、无认证模式给页——不改写落 404 违背 plan『直接给页』锁定）"
  - "[Phase 05-06]: 补斜杠重定向 301→307 实证修正（RESEARCH Pattern 6 笔误）——go1.22+ 新 mux matchOrRedirect 恒用 StatusTemporaryRedirect 保方法（GOROOT go1.26.3 server.go:2687 RedirectHandler 直读），GET 下两码语义等价；D-03 Location 暴露面结论不变"
  - "[Phase 05-06]: checkTicket 无认证模式携票必核销——ro token 签发的 ticket 过期/重放后若落入 writable 派生 mode 等于降权闸门失效；携票即走核销语义与认证模式一致（throttle 面在无认证模式不存在，nil 守卫跳过），未携票原样放行（前端探测直连链路不变）"
  - "[Phase 05-06]: 空串 bind 纳入 host 回填分支（0.0.0.0/::/空串同视全网卡）——--bind \"\" 字面可达，原样打印会拼出 http://:port/ 坏链接；isLoopbackBind 已同视空串为全网卡，语义一致"
  - "[Phase 05-06]: TestShareToken 同包白盒 + 局部最小 harness（startShareServer/dialTicketMode——startTestServerWith/dialHelloTicket 的同包映射）——Go 单文件单 package 约束下 lookup 白盒矩阵与 handler 集成须同文件，05-04 两测试分文件先例的同族推论"

patterns-established:
  - "确认门恢复执行形态沿用：Task 1 checkpoint:decision 经用户 as-locked 裁决通过，纯确认门不产生独立代码提交，在 Task 2 提交消息与本 SUMMARY 登记（05-03 先例第二次沿用）"
  - "throttle 测试 pacing 形态：失败请求序列间 sleep 过窗（fail#N 窗口 = N×base，镜像 TestAttachFlow 的 50ms 基 100/150ms 节奏）；token 分支成功请求不触 throttle（capability 绕行）可任意穿插"
  - "secret 红线测试纪律的 Go 映射：token/ticket 值只存局部变量作断言材料，断言输出只含状态码/布尔/形状（phase04.mjs 同款纪律）"

requirements-completed: [MULTI-05]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "lookup 矩阵：ro/rw token 各命中返回绑定 mode；错 token/空串/22 字符同形异值/超长同归 (\"\", false) 无 oracle；生成形态 22 字符 base64.RawURLEncoding（16B）；任一空串/nil = 通道关闭"
    requirement: MULTI-05
    verification:
      - kind: unit
        ref: "internal/server/sharetoken_test.go#TestShareToken/lookup_矩阵与生成形态"
        status: pass
    human_judgment: false
  - id: D2
    description: "/s/ 门禁：凭据模式有效 ro/rw token GET 200 且无 WWW-Authenticate challenge（委托 embed 绕 Basic）；错 token → 401 challenge（委托 / 链，recordFail 计 D-08）；POST → 405 + Allow: GET；无尾斜杠 → 307 补斜杠（Location 形态）；无认证模式错 token → 200 给页"
    requirement: MULTI-05
    verification:
      - kind: integration
        ref: "internal/server/sharetoken_test.go#TestShareToken//s/_门禁"
        status: pass
    human_judgment: false
  - id: D3
    description: "/api/attach token 分支：凭据模式 ro/rw token body → ticket → Hello 核销 Welcome mode=ro/rw（全链绑定）；错 token 与无 token 的 401 响应逐字节一致（无 oracle，T-05-05）；无认证模式携 token 非 404 出 ticket 且 mode 绑定兑现（OQ1）、无 body/错 token 维持 404 探测信号"
    requirement: MULTI-05
    verification:
      - kind: integration
        ref: "internal/server/sharetoken_test.go#TestShareToken//api/attach_token_分支"
        status: pass
    human_judgment: false
  - id: D4
    description: "启动打印两行：ro 行恒打印、rw 行仅 --writable（D-05 总闸）；bind 0.0.0.0/:: → outboundIPv4 UDP-dial 路由感知回填（本机实证 eth1 9.134.229.124，避开 docker0/bridge）；loopback/具体 bind 原样；端口取 ln.Addr() 实际值；scheme 随 TLS 分岔"
    requirement: MULTI-05
    verification:
      - kind: integration
        ref: "冒烟：/tmp/wesh-share-smoke --bind 127.0.0.1/--bind 0.0.0.0 --port 0 ± --writable -- true（自终止 spawn，stdout 行形态实证）"
        status: pass
    human_judgment: false
  - id: D5
    description: "Basic 矩阵零回归：token 通道与凭据共存、无/错 token 时 P3 矩阵不变——server 包全量（auth_e2e/handshake/权限/背压/仲裁套件）-race 绿"
    requirement: MULTI-05
    verification:
      - kind: integration
        ref: "go test -race -count=1 ./internal/server/（35.2s 全绿）"
        status: pass
    human_judgment: false

# Metrics
duration: 42min
completed: 2026-08-20
status: complete
---

# Phase 05 Plan 06: 分享链接（/s/{token}/ 第三认证通道 + 启动打印）Summary

**MULTI-05 落地：ro/rw 两条 /s/{token}/ 链接启动即打即用——shareTokens 两条目 store（SHA-256 预哈希 + subtle 位或不短路）+ /s/{token}/ 页面门禁（有效委托 embed 绕 Basic、无效委托 / 链 401 计 D-08）+ /api/attach token peek 分支（mode 绑定签发、绕过 Basic/throttle、错 token 无 oracle）+ 无认证模式正交（OQ1）+ 启动打印两行与 outboundIPv4 UDP-dial 回填（本机实证 eth1）；确认门 as-locked 通过，TestShareToken 三面锁定，server/cmd/proto/pty 全量 -race 绿。**

## Performance

- **Duration:** 42 min
- **Started:** 2026-08-20T14:58:15Z
- **Completed:** 2026-08-20T15:40:40Z
- **Tasks:** 3（Task 1 确认门 as-locked 通过 + Task 2 通道主干 + Task 3 打印/回填/测试）
- **Files modified:** 6（2 新建：sharetoken.go + sharetoken_test.go；4 修改：server.go/tickets.go/main.go/deferred-items.md）

## Accomplishments

- **确认门（Task 1）**：D-01 分享链接认证语义 + D-03 URL 形态两道 one-way 门经用户裁决 **as-locked** 通过——token 独立第三认证通道（绕过 Basic 换 mode 绑定 ticket）+ /s/{token}/ 路径段 + token 复用至重启（吊销=重启）+ host 出站回填，与 CONTEXT.md D-01..D-04 逐字一致；纯确认门不产生独立代码提交（恢复指令授权，05-03 先例沿用）
- **shareTokens store**（sharetoken.go 新文件）：`struct{ ro, rw [sha256.Size]byte }` 两条目启动预哈希（R-04：不用 map 无 janitor，生命周期=进程）；lookup 走 matchCredential 同款修正形态——SHA-256 等长化 + 两组 ConstantTimeCompare 位或累积不短路；GenerateShareToken 导出供 main 生成（crypto/rand 16B → base64url 22 字符，tickets.go:45-49 形态逐字复用）；token 值红线注释登记（D-03：永不入 logEvent/事件流/错误响应/metrics）
- **/s/{token}/ 门禁**（双模式注册，OQ1 正交）：GET 模式 + path-only 405 fallback（Allow: GET，内建 405 被 / 子树吞掉的 P3 同款纪律）；GOROOT 三坑注释登记（尾斜杠匿名多段通配 / 无尾斜杠 307 补斜杠 / 单段通配天然限长）
- **attach token 分支**（server.go shareAttach）：MaxBytesReader 4KiB 防御 + JSON{token} 解析（失败=未携 token 不回显）→ 命中按绑定 mode issueTicketJSON 直签（绕过 Basic/throttle，capability 语义 R-03）；未携/错 token 恢复 r.Body 委托原链（401 同文同码无 oracle——TestShareToken 逐字节断言锁定）；无认证模式携 token 非 404（OQ1）、否则 404 探测信号不变
- **checkTicket 适配**：无认证模式携票必核销（防 ro 票过期落入 writable 派生 mode 的降权闸门失效），throttle nil 守卫；tickets.go:16 mode 占位注释兑现两签发通道（结构零改动）
- **启动打印**（main.go）：listening on 行不动，追加 share read-only:（两空格对齐）/ share read-write: 两行（rw 行仅 --writable，D-05 总闸）；host 回填 D-04/R-04——bind 0.0.0.0/::/空串 → outboundIPv4()（UDP-dial 192.0.2.1:80 零流量路由感知，本机实证回填 eth1 9.134.229.124 避开 docker0）→ net.Interfaces() 索引序兜底 → 全失败 bind 原样不阻断启动；端口取 ln.Addr()；scheme 随 TLS 分岔
- **TestShareToken 三面锁定**（VALIDATION 05-02-01 Go 侧）：lookup 白盒矩阵（含通道关闭语义）+ /s/ 门禁（200 无 challenge / 401 challenge / 405+Allow / 307+Location / 无认证给页）+ attach 分支（mode 全链 / 401 逐字节无 oracle / 无认证正交与 404 探测）；token 红线纪律（值只存局部变量，断言只打状态码/布尔/形状）；-race 3 连跑零失败
- **冒烟实证**（自终止 spawn `-- true`）：loopback bind 打印 loopback、0.0.0.0 bind 回填 eth1、ro-only 单行 / writable 两行形态全部正确

## Task Commits

Each task was committed atomically:

1. **Task 1 (checkpoint:decision): 分享链接认证语义与 URL 形态确认门** - 无代码提交（用户裁决 as-locked 通过）
2. **Task 2: sharetoken.go + attachHandler token 分支 + tickets 注释兑现** - `5ce217b` (feat)
3. **Task 2 修正（Task 3 测试捕获的 Rule 1）: sharePage 委托 embed handler + 307 实证** - `b2a1b63` (fix)
4. **Task 3a: 启动打印两行 + outboundIPv4 回填** - `c64afaf` (feat)
5. **Task 3b: TestShareToken 三面锁定 + deferred-items 登记** - `09d7634` (test)

**Plan metadata:** 见文末 final docs 提交（SUMMARY.md + STATE.md + ROADMAP.md + REQUIREMENTS.md）

## Files Created/Modified

- `internal/server/sharetoken.go`（新）- 文件头红线注释（D-03 输出面白名单）；shareTokens 两条目 struct；newShareTokens（任一空串=通道关闭 nil）；GenerateShareToken 导出生成点；lookup（SHA-256 等长化 + 位或累积不短路 + nil receiver 恒 miss）；sharePage(page, root) 双层委托；registerShareRoutes（GET 模式 + 405 fallback + GOROOT 三坑登记）
- `internal/server/server.go` - Server.shares 字段（New 装配期固化只读）；Options.ShareTokenRO/ShareTokenRW；New 内 shares 构造 + ticket store 构造条件扩展（credentials 或 shares 任一存在）+ throttle 仍仅凭据模式；Handler 双模式注册 /s/ 两路由 + 凭据模式 attach token peek 包装（委托原链注释登记 Origin 排序理由）+ 无认证模式 404 分支 token peek；shareAttach（4KiB MaxBytesReader + JSON{token} + body 恢复委托）；issueTicketJSON 两通道共用提取；attachHandler 改调 issueTicketJSON（语义零漂移）；checkTicket 无认证携票核销分支 + throttle nil 守卫
- `internal/server/tickets.go` - ticketEntry.mode 注释兑现（Basic 全局模式 / token 绑定两签发通道；issue/redeem 签名与不变量零改动）
- `cmd/wesh/main.go` - outboundIPv4()（UDP-dial 路由感知 + Interfaces 兜底 + 全失败 ""）；run() 内 ro/rw token 生成（GenerateShareToken ×2）+ Options 传参 + 启动打印两行（D-05 rw 行总闸、read-only 后两空格对齐、ln.Addr() 端口、scheme 分岔）
- `internal/server/sharetoken_test.go`（新）- TestShareToken 三子测 + 同包最小 harness（startShareServer/dialTicketMode/postAttachBody/readBody/issueViaToken）
- `.planning/phases/05-multi-client/deferred-items.md` - 负载 flake 第二次出现登记（同签名维持原判）

## Decisions Made

- **sharePage 双层委托形态（Rule 1 修正定稿，Deviation 1）**：有效 token → 改写 `/` 后委托 **embed handler（wh）**——Task 2 初版委托 / 链根（凭据模式 = basicAuth(wh)）使有效 token 反收 401 challenge，TestShareToken 首跑即捕获；token 即凭证绕过 Basic 是 D-01 第三通道的存在意义。无效 token → 同样改写 `/` 后委托 / 链根：凭据模式 basicAuth 在文件伺服前拦截，401 响应与不改写逐字节一致；无认证模式不改写会落 FileServerFS 404 而非 plan 锁定的「直接给页」——双层统一改写是两模式正确性的最小形态。
- **补斜杠重定向 301→307（Rule 1，Deviation 2，RESEARCH 笔误修正）**：RESEARCH Pattern 6 标注「[VERIFIED GOROOT server.go:2721-2743] 301」，但 go1.26.3 直读 matchOrRedirect 恒用 `RedirectHandler(u.String(), StatusTemporaryRedirect)`（server.go:2671/2687/2696）——go1.22+ 新 mux 为保方法统一 307。GET 下两码语义等价，D-03「token 出现在 Location 头」暴露面结论不变；测试断言 307 + 注释登记实证出处。
- **checkTicket 无认证携票必核销（Rule 2 防线）**：无认证 + token 通道开启时，Hello 未携 ticket 原样放行（writable 派生 mode，探测直连链路不变）；携 ticket 则必须核销成功——否则 ro token 签发的 ticket 过期/重放后会落入 writable 派生 mode，降权闸门失效（携票即走核销语义与认证模式一致；throttle 在无认证模式不存在，nil 守卫跳过）。
- **空串 bind 纳入回填分支（Rule 2）**：`--bind ""` 与 0.0.0.0/:: 同视全网卡走 outboundIPv4 回填——原样打印会拼出 `http://:port/` 坏链接；isLoopbackBind 已同视空串为全网卡，语义一致。
- **同包白盒 harness 局部复刻**：lookup 白盒矩阵（须 package server）与 handler 集成（e2e 夹具在 package server_test）在 Go 单文件单 package 约束下不可兼得——harness 局部最小复刻（startShareServer/dialTicketMode ≈60 行），05-04『两测试分文件』先例的同族推论；VALIDATION 命名（TestShareToken）与文件位置逐字保持。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] sharePage 有效 token 委托目标错误：/ 链根 → embed handler**
- **Found during:** Task 3（TestShareToken /s/ 门禁子测首跑：有效 ro token GET 收 401 + auth_failed logEvent）
- **Issue:** Task 2 初版 sharePage 命中分支委托 root（凭据模式 = basicAuth(wh)）——有效 token 持有者被 basicAuth 拦截收 401 challenge，D-01「持有效 token 可 GET 页面（绕过 Basic）」落空；plan action 字面「命中 → 调既有 embed handler」的委托对象被误取为 / 链根
- **Fix:** sharePage 签名改 (page, root) 双 handler——命中委托 page（wh 裸 embed）；同时发现无效分支在无认证模式持原始 /s/... 路径委托会落 FileServerFS 404（plan must_have 锁定「无认证模式直接给页」），统一为命中/未命中均改写 `r.URL.Path="/"`：凭据模式 401 逐字节不变（basicAuth 在文件伺服前拦截）、无认证模式 200 给页
- **Files modified:** internal/server/sharetoken.go, internal/server/server.go（两调用点）
- **Verification:** TestShareToken /s/ 门禁子测全绿（200 无 challenge / 401 challenge / 无认证给页）；server 包全量 -race 绿
- **Committed in:** b2a1b63

**2. [Rule 1 - RESEARCH 笔误] 补斜杠重定向 301 → 307 实证修正**
- **Found during:** Task 3（探针实测 GET /s/abc 返回 307 非 plan/RESEARCH 所述 301）
- **Issue:** RESEARCH Pattern 6 标注「301 [VERIFIED: GOROOT go1.26.3 server.go:2721-2743]」——直读该 GOROOT，matchOrRedirect 三处重定向恒用 StatusTemporaryRedirect（server.go:2671/2687/2696）；go1.22+ 新 mux 为保方法统一 307，VERIFIED 标注系笔误
- **Fix:** 测试断言 307 + sharetoken.go/registerShareRoutes 注释登记实证出处（GOROOT go1.26.3 server.go:2687）；GET 下 301/307 语义等价（方法保持），D-03 Location 暴露面结论不变
- **Files modified:** internal/server/sharetoken_test.go, internal/server/sharetoken.go（注释）
- **Verification:** /s/ 门禁子测 307 + Location 后缀断言绿
- **Committed in:** b2a1b63（注释）+ 09d7634（断言）

**3. [Rule 2 - Missing Critical] checkTicket 无认证模式携票核销闸 + 空串 bind 回填**
- **Found during:** Task 2 设计推演（无认证正交链路）与 Task 3 冒烟前自查
- **Issue:** ① 无认证 + token 通道开启时若维持「tickets 非 nil 但携票不核销」，ro token 签发的 ticket 过期/重放后落入 writable 派生 mode——降权闸门失效（OQ1 mode 绑定的反向漏洞）；② `--bind ""` 原样打印会拼出 `http://:port/` 坏链接
- **Fix:** ① checkTicket 加「ticket=="" && 无凭据 → 原样放行；携票 → 必核销（throttle nil 守卫）」分支；② 回填条件加 `cfg.bind == ""`（与 isLoopbackBind 空串=全网卡判定语义一致）
- **Files modified:** internal/server/server.go, cmd/wesh/main.go
- **Verification:** TestShareToken 无认证子测（携票 mode 绑定 / 无票 404 探测 / Hello 链路）绿；冒烟 0.0.0.0 回填 eth1 实证
- **Committed in:** 5ce217b（①）+ c64afaf（②）

---

**Total deviations:** 3 auto-fixed（2 Rule 1 - Bug/RESEARCH 笔误，1 Rule 2 - Missing Critical）
**Impact on plan:** 三处修正全部服务于 plan 自身锁定的行为（D-01 绕 Basic 给页、无认证给页、mode 绑定安全）；plan 的机制形态（两条目 store / subtle 比较 / mux 装配 / peek 包装 / 打印回填）全部逐字保持。Deviation 2 仅修正 RESEARCH 文档笔误的断言值。

## Known Stubs

None — 本 plan 无新增占位 stub（无硬编码空值/占位文案/TODO；全部 verify 均已运行）。既有挂账项保持：inputDrops/droppedInputs/registry.kicks/gateTransitions（Phase 8 OPS-07）、permission_denied 占位注释。

## Threat Model 处置

| Threat ID | 处置 | 证据 |
|-----------|------|------|
| T-05-05（token 在线爆破/枚举，high） | **mitigate 已落地** | 128bit 随机空间（crypto/rand 16B，C6）+ subtle 位或不短路比较（sharetoken.go lookup，ConstantTimeCompare ×2 grep==3）+ 失败经既有 401 路径计入 D-08 统一 per-IP 退避（R-03 零新代码——/s/ 无效 token 委托 basicAuth recordFail 自动发生）；TestShareToken 无 oracle 断言（错 token 与无 token 401 逐字节一致） |
| T-05-06（token 经日志/错误响应泄露，high） | **mitigate 已落地** | token 值永不入 logEvent 参数（sharetoken.go/server.go 全部调用点审查——logEvent 三要素只有 remote/code/reason）；shareAttach 解析失败=未携 token 不回显 body；测试红线纪律（值只存局部变量）；残余面（URL 路径进反代访问日志）D-03 已裁决接受 → README 脱敏建议 05-09 落地 |
| T-05-05b（token 通道削弱 Basic 矩阵/throttle，medium） | **mitigate 已落地** | 无/错 token 时 P3 Basic 矩阵零改动（server 包全量 -race 绿，auth_e2e 套件回归锁）；有效 token 优先于 throttle 放行是 capability 语义有意设计（注释登记 R-03——避免 NAT 出口 IP 误伤持票旁观者）；128bit 空间使无节流枚举无意义 |
| T-05-06e（端点环境面泄露，medium） | **accept（登记维持）** | 浏览器历史/扩展/桌面索引/AV 扫描/截屏/PCAP 属端点环境面，自托管工具威胁模型外；README 暴露面清单 + TLS 部署建议由 05-09 落地；吊销语义=重启（D-02）；replaceState 剥离建议与 D-03 锁定冲突记录维持 05-08 处置 |

无新增威胁面——/s/ 端点与 token 分支即 plan 威胁模型的本体；无新帧类型/协议改动（P2 D-01 纪律保持）。

## Issues Encountered

- **负载 flake 第二次出现**：Task 3 全量验证首跑 1 败（47.1s vs 典型 35s，尾部 slow_consumer 1013 常规事件——与 05-05 deferred-items.md 登记同签名）；随即全量重跑绿（35.0s）+ 时序簇定向 -count=2 绿（34.4s）。本 plan diff 对 OUTPUT/信用门/踢出路径零改动（既有测试无一走新路径——无凭据无 shares 时 ticket store 保持 nil，checkTicket 零漂移），维持越界不修复原判，第二次出现已登记 deferred-items.md。
- **sharePage 委托目标的 plan 字面歧义**：plan action「未命中 → 原样委托 / 处理链」与 must_have「无认证模式直接给页」在机械层面冲突（原样委托 = 持原始路径，FileServerFS 对 /s/... 返回 404 ≠ 给页）——按 must_have 锁定行为调和（统一改写 / 后委托），凭据模式响应逐字节不变（见 Deviation 1）。

## User Setup Required

None - no external service configuration required.

## 遗留事项（plan 授权的占位与后续挂点）

- **05-08 前端**：/s/{token}/ 路径探测提取 token → fetch /api/attach body 携 {token} → ticket → Hello 链路（RESEARCH Pattern 9 连接流程）；无认证模式前端探测逻辑适配（URL 携 token 时必走 fetch）
- **05-09 README/UAT**：反代访问日志 token 脱敏建议 + TLS 部署建议 + 暴露面清单（T-05-06/T-05-06e 残余登记）；phase05.mjs stdout 链接解析断言（UAT 侧实证锚点——两行打印形态已在本 plan 冒烟锁定）
- **重启即吊销语义**（D-02）：README 明示「重启即废全部旧链接」

## Next Phase Readiness

- 05-07 max-clients：/api/attach 双点位 503 早闸（OQ2 裁决）挂点已明确——shareAttach 命中分支与 attachHandler 原链两处；Attach 守卫区 ③ 位顺序纪律不变
- 05-08 前端：token 通道全部服务端面就绪（/s/ 门禁 + attach body 分支 + 无认证正交）；/s/{token}/ URL 契约与响应形态锁定
- 无阻塞项；server/cmd/proto/pty 四包 -race 全绿，TestShareToken -race 3 连跑稳定

## Self-Check: PASSED

- FOUND: internal/server/sharetoken.go（`func (st *shareTokens) lookup` + `func (s *Server) sharePage` == 2；ConstantTimeCompare == 3；/s/{token}/ == 5）
- FOUND: internal/server/server.go（/s/{token}/ 注释 == 1；MaxBytesReader == 5；shareAttach/issueTicketJSON 就位）
- FOUND: internal/server/sharetoken_test.go（`func TestShareToken` == 1，三子测齐备；-race 3 连跑绿）
- FOUND: cmd/wesh/main.go（share read-only:/read-write: == 2；`func outboundIPv4` == 1 且含 192.0.2.1）
- FOUND: commit 5ce217b（Task 2）、b2a1b63（fix）、c64afaf（Task 3a）、09d7634（Task 3b）均在 git log；四提交均无意外文件删除（--diff-filter=D 检查通过）
- go build/vet 退出 0；go test -race -count=1 ./internal/server/（35.2s 全绿）./cmd/wesh/（1.0s）./internal/proto/ ./internal/pty/ 全绿
- 冒烟实证：loopback/0.0.0.0 两形态打印正确（0.0.0.0 回填 9.134.229.124 = eth1），ro-only 单行 / writable 两行

---
*Phase: 05-multi-client*
*Completed: 2026-08-20*
