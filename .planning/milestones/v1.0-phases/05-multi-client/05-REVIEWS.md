---
phase: 5
reviewers: [opencode]
reviewed_at: 2026-08-20T11:09:15+08:00
plans_reviewed: [05-01-PLAN.md, 05-02-PLAN.md, 05-03-PLAN.md, 05-04-PLAN.md, 05-05-PLAN.md, 05-06-PLAN.md, 05-07-PLAN.md, 05-08-PLAN.md, 05-09-PLAN.md]
---

# Cross-AI Plan Review — Phase 5

## OpenCode Review

> Orchestrator note: 本次运行环境中仅 `opencode` 一个 reviewer lane 可用（gemini/claude/codex/qwen/cursor/agy/kimi/ollama/lm_studio/llama_cpp 均未安装或未运行）。
> 第一次运行（完整 prompt ~290KB，含 RESEARCH.md）产出为离题的项目状态摘要，已丢弃；以下为第二次运行结果，prompt 裁剪为 ~215KB（省略 05-RESEARCH.md，其结论已含于各 PLAN 的假设/证据节），并在顶部强化了输出契约。

# Cross-AI Plan Review: Phase 5 Multi-Client Sharing

## Executive Summary

This review covers 9 implementation plans for Phase 5 (Multi-Client Sharing) of the wesh project. The plans demonstrate solid architectural thinking with proper layering, clear contracts, and thorough testing strategies. However, several areas warrant attention: the token-based sharing model has security surface considerations beyond those explicitly addressed; some critical behaviors rely on deferred tuning; and the long dependency chain (9 waves) creates integration risk. Overall risk level: **MEDIUM**.

---

## Plan-by-Plan Review

### Plan 05-01: Multi-client Fan-out Trunk

**Summary:** Foundational plan establishing the registry, hub, and outbox architecture. Well-designed with clear lifecycle semantics, but the shared frame lifetime management and outbox capacity tuning need careful validation.

**Strengths:**
- Clear tracer pattern correctly identifies this as the architectural foundation
- Proper separation of concerns: registry (state) + outbox (buffer) + writer (I/O isolation)
- Lifecycle semantics are correct: client detach ≠ process exit, only child exit terminates
- Test coverage spans fan-out consistency, detach behavior, and exit broadcast

**Concerns (HIGH):**
- **Shared frame lifetime:** `internal/pty/io.go:13-14` confirms ReadLoop reuses underlying buffer; plan correctly notes frames must be copied, but the inter-play between ReadLoop blocking (credit gate) and frame aliasing during Wait isn't explicitly proven safe in the plan. Need to verify frame is fully consumed or copied before credit gate Wait releases hubMu.

**Concerns (MEDIUM):**
- **Outbox capacity 512KiB default:** Plan marks this for Phase 9 tuning, but wrong size could cause either premature kicks (UX issue) or excessive memory. No guidance on sizing methodology.
- **Atomic gate removal is one-way:** The 409 atomic gate is fully removed. While correct for multi-client semantics, no rollback path exists if this plan fails late in testing.

**Concerns (LOW):**
- Kick reason string `slow_consumer` appears in both logEvent and Close reason - good for correlation but could expose internals.

**Suggestions:**
1. Add explicit assertion that frame copy happens *before* credit gate Wait, with code trace to prove.
2. Consider starting with smaller outbox (128KiB) for faster test failure, then scale up.
3. Add debug logging for outbox high-water marks during testing.

**Risk Assessment: MEDIUM** - Foundational correctness is critical; frame aliasing needs explicit proof.

---

### Plan 05-02: Global Credit Gate & Backpressure

**Summary:** Implements the credit gate mechanism to prevent slow clients from blocking PTY reads. The design is sound with proper broadcast semantics, but the half-watermark threshold and dead connection handling during gate closure need validation.

**Strengths:**
- Credit gate using sync.Cond on hubMu is the right primitive
- Clear ro/rw distinction: ro full → immediate kick; rw full → join credit set
- Proper deadlock prevention via broadcasts on all state transitions (detach/kick/attach/exit)
- SIGWINCH on new attach addresses the "black screen on join" UX issue nicely

**Concerns (HIGH):**
- **Half-watermark oscillation risk:** Plan specifies 50% recovery threshold (`internal/server/server.go:85-133` patterns show 256KiB for 512KiB default). This could cause gate oscillation if a client drains to 257KiB → gate opens → immediately fills back to 513KiB → gate closes. The plan notes "迟滞防门震颤" but doesn't analyze the oscillation frequency or provide mitigation if it occurs.

**Concerns (MEDIUM):**
- **Dead connections during gate closure:** Plan correctly notes pinger will catch dead connections via `mu.lock(ctx)` timeout (02-04 discipline). However, the interaction between credit gate Wait (holds hubMu) and pinger's detection path isn't traced explicitly. Need to verify pinger can signal even when gate is held.
- **SIGWINCH on Darwin:** Plan notes A1 assumption about same-size not sending signal. CI should validate this; a test that explicitly checks process receives SIGWINCH on attach would be better than assuming Linux behavior.

**Concerns (LOW):**
- `allWritableBlockedLocked()` O(n) walk of registry on every chunk - acceptable for expected client counts but worth noting.

**Suggestions:**
1. Add hysteresis: gate opens at <40%, closes at >60% to prevent oscillation.
2. Add explicit test for "dead owner during gate closure" scenario.
3. Add CI test validating SIGWINCH delivery on both Linux and Darwin.

**Risk Assessment: MEDIUM** - Gate correctness is critical; oscillation and dead connection scenarios need testing.

---

### Plan 05-03: Write Permission System

**Summary:** Implements owner/all write policy with FIFO succession. Design is clean and well-specified, but the succession race conditions and permission_denied unused code need attention.

**Strengths:**
- Owner FIFO succession is deterministic and fair
- Welcome frame reuse for promotion is elegant (no new frame types)
- Prefs dual-file (ro/rw) correctly addresses OSC52 hijacking concern
- Clear mode determination matrix

**Concerns (HIGH):**
- **Succession during slow owner:** If owner is slow (outbox filling), gets kicked (1013), then immediately reconnects - the succession race between reconnect and existing rwEligible clients isn't clearly resolved. Plan says "order FIFO 首个 rwEligible 在线者" but reconnect creates a *new* client entry at end of order. Need explicit specification of timing.

**Concerns (MEDIUM):**
- **permission_denied code unused:** Plan correctly notes this stays as placeholder, but the unused constant in `proto.go` could confuse future readers. Better to remove or add comment explaining why it exists but isn't used.

**Concerns (LOW):**
- The one-way flag (--write-policy) is correct but the checkpoint task structure is unusual for a plan review - typically implementation decisions don't require blocking gates.

**Suggestions:**
1. Add explicit specification: "on owner kick, promotion happens before kicked owner's reconnect can complete (hubMu held during kick sequence)".
2. Remove or clearly document unused permission_denied constant.
3. Add test for "owner kicked, reconnects before promotion completes" race.

**Risk Assessment: MEDIUM** - Succession semantics are critical; race conditions need explicit proof.

---

### Plan 05-04: Resize Arbitration

**Summary:** Implements resize arbitration with min-rect algorithm. The pure function design is clean, but the ro client clipping behavior and parameter ordering traps need careful handling.

**Strengths:**
- Pure `arbitrate()` function is easily testable
- Participation set split by write mode is logically correct
- Debounce using single timer (not hand-rolled loops) follows good practices
- Getsize integration testing validates actual PTY size

**Concerns (HIGH):**
- **Ro client clipping is poor UX:** Plan notes ro clients may see clipped output if they resize smaller than PTY. For a "demo/spectator" use case, this is a significant UX issue. The plan says "README 明示" but spectators won't read README during a demo. Consider at minimum a console warning or visual indicator.

**Concerns (MEDIUM):**
- **Parameter order trap:** Plan correctly notes `(cols, rows)` vs `(rows, cols)` danger (`internal/pty/io_test.go:24-25`). But Getsize returns `(rows, cols)` (reverse of Resize). The test needs to explicitly verify this mismatch is handled correctly.
- **Debounce 50ms might be aggressive:** For terminals running vim/htop in network conditions, 50ms might process intermediate sizes that users don't intend. Consider making this configurable.

**Concerns (LOW):**
- Plan notes terminal apps must tolerate "pseudo SIGWINCH" - true but some apps do behave poorly with unnecessary signals.

**Suggestions:**
1. Add console warning when ro client window < PTY size.
2. Add visual indicator (e.g., red border) for clipped ro clients.
3. Make debounce configurable via client option.
4. Add test explicitly checking Getsize/Resize parameter ordering.

**Risk Assessment: MEDIUM** - Ro clipping UX is problematic for the primary use case.

---

### Plan 05-05: Input Rate Limiting

**Summary:** Implements per-client input rate limiting and input queue to fix CR-01. The design is solid using x/time/rate, but the silent drops and queue sizing need consideration.

**Strengths:**
- Using x/time/rate is the correct choice (standard library, well-tested)
- Input-writer goroutine correctly isolates blocking writes
- Drop-on-overload is the right semantic (vs. blocking or disconnecting)
- Queue sizing (256KiB) provides reasonable buffer for bursts

**Concerns (HIGH):**
- **Silent input drops:** User pastes 100KB, only 64KB arrives (burst limit), rest is silently dropped. No feedback mechanism. In collaboration scenarios, this could cause significant confusion ("I pasted the command, why did only part run?").

**Concerns (MEDIUM):**
- **x/time dependency:** Plan verifies legitimacy but x/time isn't part of main Go distribution. Module caching, version drift, and potential future deprecation are risks. Consider pinning to specific version and documenting why.
- **Rate limits unconfigurable at runtime:** 32KiB/s + 64KiB burst are hardcoded in Phase 5. Fast typists with large pastes might hit limits legitimately. Phase 7 config file work should address this.

**Concerns (LOW):**
- Queue overflow handling (tryEnqueue false path) counts but doesn't log - appropriate but should be documented.

**Suggestions:**
1. Add optional visual feedback when input is rate-limited (e.g., terminal flash or status line).
2. Make rate/burst configurable via command-line flags in Phase 7.
3. Add metrics for input drop count (noted for Phase 8, but add stub now).
4. Document the x/time version pinning rationale.

**Risk Assessment: MEDIUM** - Silent drops are problematic for collaboration UX; rate tuning is deferred.

---

### Plan 05-06: Share Token Authentication

**Summary:** Implements token-based share links. The design addresses the use case well, but the token exposure surface is larger than the plan acknowledges.

**Strengths:**
- Token as independent auth channel is clean separation
- /s/{token}/ path is simple and bookmarkable
- Token never appears in server logs (SEC-01 discipline)
- Mode binding (ro/rw) by token type is intuitive

**Concerns (HIGH):**
- **Token exposure vectors not fully addressed:** Plan acknowledges URL appears in:
  - Browser URL bar
  - Browser history
  - Referrer headers
  - Reverse proxy access logs (addressed in README)
  
  But misses:
  - Browser extensions (password managers, screenshot tools)
  - Desktop search/indexing
  - Antivirus URL scanning
  - Network monitoring/PCAP
  - User screenshots/screen shares
  
  The plan says "token 路径进反代访问日志的显式取舍" but this is a broader exposure issue.

**Concerns (MEDIUM):**
- **128-bit entropy is excessive:** 128-bit = 3.4×10^38 possibilities. For a session that lasts hours/days until restart, this is overkill. 64-bit (1.8×10^19) would still be unguessable and reduce URL length. The plan defends 128-bit for "future-proofing" but the exposure vectors make the extra bits moot.
- **UDP dial for outbound IP is fragile:** The `outboundIPv4()` function uses `net.Dial("udp", "192.0.2.1:80")` trick. While clever, this can fail in various network configurations (VPNs, proxies, custom routing). The fallback to interface enumeration is good but not guaranteed to produce correct address either.

**Concerns (LOW):**
- Token in browser URL bar is visible to anyone looking at screen - relevant for shared-screen demos.

**Suggestions:**
1. Add security documentation section listing all token exposure vectors.
2. Consider shorter tokens (64-bit) or configurable entropy.
3. Add option to disable share links entirely for high-security deployments.
4. Add timeout warning: "links expire on wesh restart".
5. Improve outbound IP detection with multiple fallback strategies.

**Risk Assessment: MEDIUM-HIGH** - Token exposure surface is broader than acknowledged; security implications need broader analysis.

---

### Plan 05-07: Max Clients Limit

**Summary:** Implements connection limit with dual rejection points. The design is simple and effective, but the transient overrun acceptance and counter maintenance need attention.

**Strengths:**
- Dual rejection points (early /api/attach + WS Accept) prevent resource allocation
- Atomic counter is simple and correct
- Proper counter maintenance (register +1, detach -1)
- Clear flag semantics with reasonable default (32)

**Concerns (HIGH):**
- **Counter drift risk:** Plan says "register/detach 唯一收口点加减" but need to verify ALL exit paths (including error paths, panics, context cancellations) hit these points. A single missed decrement could cause permanent capacity reduction.

**Concerns (MEDIUM):**
- **Transient overrun acceptance:** Plan accepts ≤8 transient overrun due to concurrent handshakes. While noted as "容量策略非安全边界", in practice this could cause confusing behavior (33 clients connected when limit is 32).
- **No differentiation in limit:** All clients count equally toward limit. A deployment might want to limit ro and rw separately (e.g., 10 rw + 100 ro).

**Concerns (LOW):**
- Default 32 is arbitrary - plan notes Phase 9 tuning but provides no sizing guidance.

**Suggestions:**
1. Add invariant check: verify counter matches actual registry size periodically.
2. Add metrics for max-clients hits and current count.
3. Consider per-mode limits in future.
4. Document transient overrun behavior for operators.
5. Add admin endpoint to inspect current client count.

**Risk Assessment: MEDIUM** - Counter correctness is critical; drift could cause persistent issues.

---

### Plan 05-08: Frontend Changes

**Summary:** Implements frontend support for multi-client features. The design is clean with proper token handling discipline, but the "no auto-reconnect" decision has UX implications and several ro-mode behaviors need better handling.

**Strengths:**
- Token handling is properly constrained (closure-only, not persisted)
- Response dispatch matrix covers all cases clearly
- 1013 handling with manual reload is correct semantic
- OSC52 gate prevents bystander clipboard hijacking
- Promote transition is clean (no new UI components)

**Concerns (HIGH):**
- **Token persistence in URL:** The plan prohibits `history.replaceState` to remove token from URL. This means:
  1. Refreshing page after token compromised → attacker still has access
  2. Browser history retains token indefinitely
  3. Back/forward navigation re-uses old tokens
  
  The plan says "D-03 路径段是分享/书签契约" but this conflicts with security best practices. Consider at minimum adding an option to strip token after successful attach.

**Concerns (MEDIUM):**
- **No auto-reconnect decision:** Plan explicitly rejects auto-reconnect for 1013, deferring to Phase 6. While the reasoning ("后台标签页重连→再被踢循环比手动刷新更差") has merit, users will experience this as a regression from single-client behavior. At minimum, should show clear "reload needed" UI.
- **Ro mode UX issues:**
  - No visual indicator that client is in ro mode
  - Input silently ignored (no feedback)
  - Resize clipping not indicated
  - OSC52 operations fail silently

**Concerns (LOW):**
- `shareToken` is closure variable but JavaScript closures can be inspected in dev tools - acceptable for expected threat model.

**Suggestions:**
1. Add option to strip token from URL after successful attach (configurable).
2. Add prominent visual indicator for ro mode (e.g., status bar "READ-ONLY").
3. Add console message when input is dropped in ro mode.
4. Add visual indicator when ro client window < PTY size.
5. Reconsider auto-reconnect with exponential backoff for 1013 case.

**Risk Assessment: MEDIUM** - Token persistence and ro-mode UX are significant concerns.

---

### Plan 05-09: Integration Testing & Documentation

**Summary:** Final integration plan with UAT scripts and README updates. Good coverage of main scenarios but skips some critical tests with weak rationale.

**Strengths:**
- Comprehensive UAT script structure
- Good coverage of share link flows, permission modes, and capacity limits
- README updates document new behaviors clearly
- Proper skipped test recording with rationale

**Concerns (HIGH):**
- **Critical tests skipped:**
  - S6 (1013 kick): Marked "skipped" because "Go 集成测试 TestSlowConsumerKick 已覆盖". But UAT tests different layer (full binary, real network) and the backpressure behavior is critical. This is a core differentiator of the phase.
  - S7 (visual multi-client consistency): Marked "skipped" because "headless 硬约束". But this is the primary user-visible behavior of the phase. At minimum, a manual test checklist should be provided.

**Concerns (MEDIUM):**
- **No load/stress testing:** Plans defer parameter tuning to Phase 9 but don't include load tests that would inform tuning.
- **Error recovery scenarios missing:** What happens when:
  - Owner crashes mid-operation
  - Network partition between clients
  - Server restart during active session
  - Client reconnect with stale ticket

**Concerns (LOW):**
- README "脱敏建议" is good but could be more specific (example nginx config).

**Suggestions:**
1. Add S6 with configurable test (use smaller outbox, add artificial delay).
2. Provide manual test checklist for S7 with expected visual behaviors.
3. Add basic load test: 10 clients all typing simultaneously.
4. Add error recovery test scenarios.
5. Add specific nginx config example for log sanitization.

**Risk Assessment: MEDIUM** - Critical behaviors are undertested; deferred testing creates risk.

---

## Cross-Cutting Analysis

### Dependency Chain Risk

The 9-wave dependency chain (01→02→03→04→05→06→07→08→09) creates significant integration risk:
- Any issue discovered in late waves could require re-architecting early waves
- No parallelization opportunities within the phase
- Long execution time increases chance of context loss between waves

**Suggestion:** Consider identifying parallelizable work within waves (e.g., 06 could potentially start after 03 instead of after 05, since it only needs permission model, not resize/credit gates).

### Parameter Tuning Deferral

Multiple parameters are "Phase 9 标定回填":
- Outbox capacity (512KiB)
- Input rate/burst (32KiB/s, 64KiB)
- Max clients (32)
- Resize debounce (50ms)
- Credit gate half-watermark (50%)

This deferral is problematic because:
1. Wrong defaults could cause UX issues in Phase 5 deployment
2. No methodology provided for Phase 9 tuning
3. Parameters are interconnected (e.g., outbox size vs. credit gate threshold)

**Suggestion:** Define tuning methodology and acceptance criteria now, even if execution is deferred.

### Security Surface Analysis

The token-based sharing introduces attack vectors that need broader analysis:

1. **Token exposure** (Plan 06, 08): Beyond what's documented
2. **Permission escalation** (Plan 03): Succession race conditions
3. **Resource exhaustion** (Plan 02, 05): Credit gate and input limiting could be DoS vectors
4. **Side channels**: Timing of operations (promotion, kicks) could leak information

**Suggestion:** Consider dedicated security review phase before release.

### Observability Gap

Many metrics deferred to Phase 8:
- Outbox depths
- Credit gate open/close cycles
- Input drop counts
- Client counts/modes
- Resize arbitration events

Phase 5 will operate with limited visibility into these critical behaviors.

**Suggestion:** Add minimal instrumentation stubs now (even if just structured logging), rather than waiting for Phase 8.

---

## Overall Risk Assessment

**Risk Level: MEDIUM**

**Justification:**
- Architectural foundation (Plans 01-04) is sound with clear semantics
- Token-based sharing (Plans 06, 08) has security surface concerns beyond those addressed
- Critical behaviors (1013 kick, multi-client visual) are undertested (Plan 09)
- Parameter tuning is deferred without methodology
- Long dependency chain increases integration risk

**Key Risks:**
1. **Frame aliasing during credit gate wait** (Plan 01) - needs explicit proof
2. **Token exposure surface** (Plans 06, 08) - broader than documented
3. **Succession race conditions** (Plan 03) - timing not fully specified
4. **Ro client UX** (Plans 04, 08) - clipping and silent drops are problematic
5. **Counter drift** (Plan 07) - single missed decrement causes permanent capacity reduction

**Recommendations:**
1. Add explicit proof or additional copy for frame aliasing during credit gate
2. Expand security documentation for token exposure vectors
3. Add visual indicators for ro mode and size clipping
4. Add minimal observability (logging) for deferred metrics
5. Add manual test checklist for multi-client visual consistency
6. Consider shortening dependency chain where possible

The plans demonstrate solid engineering fundamentals but need attention to the identified gaps before implementation.

---

## Consensus Summary

本次仅 1 个 reviewer（opencode）完成评审，以下为该 reviewer 的跨 plan 综合，不构成多系统共识。建议后续在有第二可用 CLI 的环境中重跑 `/gsd-review --phase 5 --all` 以对 HIGH 项做交叉验证。

### Agreed Strengths

- 架构分层清晰：registry（状态）/ outbox（缓冲）/ writer（I/O 隔离）职责分离，信用门用 `sync.Cond` 是正确原语（05-01/05-02）
- 协议复用克制：Welcome 帧复用做 promotion 不新增帧类型，OSC52 ro/rw 双闸门防剪贴板劫持，延续 Phase 2/3 安全纪律（05-03/05-08）
- 纯函数化设计（resize `arbitrate()`）与 Getsize 集成测试，可测性好（05-04）
- 双拒绝点（/api/attach + WS Accept）防资源分配在前（05-07）

### Agreed Concerns

1. **[HIGH] Frame 别名与信用门 Wait 的交互未被证伪**（05-01/05-02）：`internal/pty/io.go:13-14` ReadLoop 复用底层 buffer，需显式证明 frame 在 credit gate Wait 释放 hubMu 前已完整拷贝/消费。
2. **[HIGH] Token 暴露面大于 plan 所述**（05-06/05-08）：除 README 已记的反代日志外，浏览器扩展/桌面索引/AV 扫描/截屏共享/PCAP 均未覆盖；前端禁止 `history.replaceState` 剥离 token 与安全最佳实践冲突。
3. **[HIGH] Owner 继承竞态**（05-03）：owner 被 1013 踢出后立即重连与既有 rwEligible 晋升的时序未完全规格化。
4. **[HIGH] 关键行为欠测试**（05-09）：S6（1013 踢出）与 S7（多客户端视觉一致）被跳过，理由偏弱——UAT 层与 Go 集成测试层级不同，且 S7 是本 phase 的主要用户可见行为。
5. **[HIGH] ro 客户端 UX**（05-04/05-08）：窗口小于 PTY 被裁剪无任何视觉提示，ro 模式输入静默丢弃。
6. **[HIGH] 客户端计数漂移**（05-07）：需验证所有退出路径（含错误/panic/cancel）都经过唯一收口点，单次漏减导致永久容量损失。
7. **[MEDIUM] 参数标定整体推迟 Phase 9 且无方法论**（跨切面）：outbox 512KiB、输入 32KiB/s+64KiB burst、max-clients 32、debounce 50ms、半水位 50% 相互耦合，错误默认值会直接损害 Phase 5 部署体验。
8. **[MEDIUM] 观测性缺口**（跨切面）：outbox 深度、门开闭周期、输入丢弃数等指标全部推迟 Phase 8，Phase 5 将低可见度运行。
9. **[MEDIUM] 9 波串行依赖链集成风险**（跨切面）：建议识别可并行波次（如 06 可在 03 之后启动）。

### Divergent Views

无（单一 reviewer）。

### Overall Risk

**MEDIUM**（05-06 单 plan 评为 MEDIUM-HIGH）。架构地基（05-01~05-04）语义清晰、工程基本面扎实，但 token 安全面、继承竞态、关键行为测试缺口需在实现前补齐。
