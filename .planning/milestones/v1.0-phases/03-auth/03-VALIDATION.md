---
phase: 3
slug: auth
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-17
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go 1.26.3 stdlib `testing`) |
| **Config file** | none — stdlib |
| **Quick run command** | `go test ./... -run 'Ticket|Auth|Throttle|Origin|Redact|TLS' -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -run 'Ticket|Auth|Throttle|Origin|Redact|TLS' -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-T1 | 03-01 | 1 | SEC-02 | T-03-05 | ErrAuthFailed 契约 + HelloPayload.Ticket 往返（D-10 统一口径前提） | unit | `go test ./internal/proto/ -run TestDecodeHello -count=1` | ✅ proto_test.go（扩展） | ⬜ pending |
| 03-01-T2 | 03-01 | 1 | SEC-02 | T-03-01/02/04 | ticket 单次使用/60s TTL/mode 绑定/惰性清扫（now 注入零 sleep） | unit | `go test ./internal/server/ -run TestTicketStore -count=1` | ❌ 新建 tickets_test.go | ⬜ pending |
| 03-01-T3 | 03-01 | 1 | SEC-03 | T-03-03/04/05 | 退避级数 1/2/4/8/16/30s 封顶/成功清零/15min 惰性过期（now 注入） | unit | `go test ./internal/server/ -run TestThrottleStore -count=1` | ❌ 新建 throttle_test.go | ⬜ pending |
| 03-02-T1 | 03-02 | 1 | SEC-01 | T-03-06 | SHA-256 等长化 + subtle 逐组不短路比较（erratum 修正形态） | unit | `go test ./internal/server/ -run 'TestParseCredential\|TestCredentialMatch' -count=1` | ❌ 新建 auth_test.go | ⬜ pending |
| 03-02-T2 | 03-02 | 1 | SEC-04 | T-03-07/08 | Origin 规范化（剥默认端口/拒 glob）+ 四段放行 + null 拒绝 | unit | `go test ./internal/server/ -run 'TestNormalizeOrigin\|TestOriginAllowed' -count=1` | ❌ 新建 origin_test.go | ⬜ pending |
| 03-02-T3 | 03-02 | 1 | SEC-05 | T-03-09/10/11 | TLS 1.1 败/1.2 成/1.3 默认/CBC 败 + 安全头双分支精确值 | unit+integration(tls.Dial) | `go test ./internal/server/ -run 'TestTLSVersionAndCipherFloor\|TestSecurityHeaders' -count=1` | ❌ 新建 tls_test.go | ⬜ pending |
| 03-03-T1 | 03-03 | 2 | SEC-01/02/03/04 | T-03-12/13/15 | tracer 主链路（401 同文→ticket→Hello→Welcome）+ 非法 ticket + 无认证模式；ThrottleBase:50ms 注入 + pacing；retryAfter 访问器 | integration(e2e) | `go test ./internal/server/ -run 'TestAttachFlow\|TestTicketInvalid\|TestNoAuthMode' -count=1` | ❌ 新建 auth_e2e_test.go（throttle_test.go 追加断言段） | ⬜ pending |
| 03-03-T2 | 03-03 | 2 | SEC-01/02 | T-03-14/17 | 端点守卫链 405/413/403/200 三头 + TTL 过期同口径 + 日志红线运行时捕获 | integration | `go test ./internal/server/ -run 'TestAttachEndpoint\|TestTicketExpiry\|TestLogRedaction' -count=1` | ❌ auth_e2e_test.go 追加 | ⬜ pending |
| 03-03-T3 | 03-03 | 2 | SEC-03/04 | T-03-12/16/18 | 429+Retry-After/成功清零/级数重启（base=200ms pacing）+ D-08 共享计数器反证 + 双端点 Origin（g→h 间 sleep 过窗） | integration | `go test ./internal/server/ -run 'TestThrottleHTTP\|TestThrottleHelloSharedCounter\|TestOriginEndpoints' -count=1` | ❌ auth_e2e_test.go 追加 | ⬜ pending |
| 03-04-T1 | 03-04 | 3 | SEC-01/04/05 | T-03-21/23 | 6 flag parse 期校验 + WESH_CREDENTIAL 兜底 flag 优先 + 证书成对报错 | unit | `go test ./cmd/wesh/ -run 'TestParseArgs\|TestCredentialFlagEnv\|TestTLSKeyPairError' -count=1` | ✅ main_test.go（扩展） | ⬜ pending |
| 03-04-T2 | 03-04 | 3 | SEC-01/05 | T-03-19/20/22 | 启动矩阵八行精确文案/拒绝零资源占用/WESH_CREDENTIAL 子进程剥离 | unit+run 级 | `go test ./cmd/wesh/ -run 'TestStartupMatrix\|TestStartupRefusalNoResource' -count=1 && go test ./internal/pty/ -count=1` | ✅ main_test.go/spawn_test.go（扩展） | ⬜ pending |
| 03-05-T1 | 03-05 | 3 | SEC-02 | T-03-24/25/26 | connect() 取 ticket/Hello 携带/auth_failed 单次重试门闩/无新 UI | static(tsc)+build | `pnpm -C web exec tsc --noEmit && pnpm -C web build` | ✅ main.ts（改造） | ⬜ pending |
| 03-05-T2 | 03-05 | 3 | SEC-02 | T-03-24 | dist 产物含 /api/attach 与 wss 检索串（新流程入包证据） | build+grep | `pnpm -C web build && grep -q '/api/attach' web/dist/index.html && go build ./...` | ✅ dist（重建） | ⬜ pending |
| 03-06-T1 | 03-06 | 4 | SEC-01..05 | T-03-27 | 六场景协议 UAT（场景 1/5 按 1s/2s/4s 爬梯真实 sleep——生产二进制无 throttle flag）；凭据/ticket 不入输出 | e2e(真实二进制) | `go build -o /tmp/wesh-uat/wesh ./cmd/wesh && node web/uat/phase03.mjs /tmp/wesh-uat/wesh && node web/uat/phase02.mjs /tmp/wesh-uat/wesh` | ❌ 新建 phase03.mjs；✅ phase02.mjs（一行适配） | ⬜ pending |
| 03-06-T2 | 03-06 | 4 | SEC-01..05 | T-03-28/29 | 文档-wire 一致（六 flag/契约/auth_failed）+ 六段式收口（gofmt/vet/-race/前端构建/裸 clone/冒烟三形态） | build+grep+smoke | `go vet ./... && go test -race -count=1 ./... && pnpm -C web build && grep -q -- '--credential' README.md && grep -q 'testssl' .planning/phases/03-auth/03-UAT.md` | ❌ 新建 03-UAT.md；✅ README.md（更新） | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Planner 已按 6 plan / 15 task 回填；File Exists 标注测试载体现状（❌ 新建 = 由对应 task 创建，非 Wave 0 stub）。*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

无独立 Wave 0 基建需求（RESEARCH §Wave 0 Gaps 结论）：测试框架、黑盒装配（startTestServerWith/dialHello/waitExit，e2e_test.go 既有）、CI 全部就位。随测试任务共生的辅助件（归属已钉死，非独立 Wave 0）：TLS 自签证书 helper（03-02-T3）、stderr os.Pipe 捕获 helper（03-03-T2，复制 main_test.go captureFd 形态）、Options 注入字段 TicketTTL/ThrottleBase/ThrottleCap（03-03-T1，延续 HelloTimeout 覆写先例）、dialHelloTicket/attachURL/postAttach helper（03-03-T1）。

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| testssl.sh 无弱项 | SEC-05 | 外部扫描工具，需运行中的 TLS 端点 | 启动 wesh --tls 后运行 `testssl.sh https://localhost:7681`，确认无 LOW/MEDIUM/HIGH  findings |
| 浏览器 attach→WS 全流程 | SEC-01 | 需真实浏览器验证 fetch 带凭据 + Hello 首帧核销 | 登录后打开终端页面，DevTools 观察 POST /api/attach 200 且 WS 建立；刷新页面重放旧 ticket 被拒绝 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
