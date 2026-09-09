---
phase: 6
reviewers: [opencode]
reviewed_at: 2026-08-23T00:58:19Z
plans_reviewed: [06-01-PLAN.md, 06-02-PLAN.md, 06-03-PLAN.md, 06-04-PLAN.md, 06-05-PLAN.md, 06-06-PLAN.md, 06-07-PLAN.md]
---

# Cross-AI Plan Review — Phase 6

> 单一 reviewer 运行(opencode 为本机唯一可用的外部 CLI;gemini/claude/codex/qwen/cursor/agy/kimi 及本地 ollama/lm-studio/llama.cpp 均未安装或未运行)。共识节仅反映 OpenCode 一家的发现,权重据此打折。

## OpenCode Review

I'll review each implementation plan systematically, verifying claims against the source code where accessible.
Now I'll provide a comprehensive review of all 7 plans.

---

# Cross-AI Plan Review: Phase 6 Implementation

## Plan 00: EXIT Frame Implementation

### Summary
This plan implements the EXIT frame protocol (SESS-03) - adding a new S→C frame type `'X'` (0x58) to signal child process exit with exit code to all connected clients before the 1000 normal close. The plan correctly identifies the critical write-ordering problem between EXIT frame and 1000 close frame, proposing a synchronous write approach within each client's goroutine.

### Strengths
- **Correctly identifies Pitfall 1 (write-order race)**: The plan explicitly addresses the race condition where EXIT frames sent via outbox could be reordered with 1000 close frames. The solution (synchronous Write→Close in same goroutine) is sound.
- **Clear protocol contract definition**: One-way door decisions (D-08/D-09) are properly documented with reversibility warnings.
- **Detailed signal name handling**: Pitfall 3 (Signal.String() returns lowercase "hangup" not "SIGHUP") is correctly identified with explicit mapping table.
- **Goroutine safety verified**: Parallel close with WaitGroup is already implemented (server.go:980-988), and adding EXIT write preserves this pattern.

### Concerns

**HIGH**: The plan claims `proto.go:32` has a placeholder comment for `'X'`, but the actual comment reads:
```go
// 'X' EXIT / 'T' TITLE / 'P' PREFS —— 类型字节本 phase 占住，语义分属 Phase 6/4（D-01）
```
This is correct, but the plan doesn't verify the actual implementation impact on the existing Error frame handling. Need to confirm that 'E' (Error) and 'X' (EXIT) can coexist without client-side confusion.

**MEDIUM**: The exitMessage() function groups signal name mapping inline. Consider extracting to a separate helper:
```go
func signalName(sig syscall.Signal) string {
    // mapping table
}
```
This would improve testability and reduce lifecycle() complexity.

**LOW**: The 2s timeout for EXIT Write is arbitrary. While documented as "Phase 9 calibration", this could impact slow network clients. Consider making this configurable or documenting the tradeoff explicitly.

### Suggestions
- Add explicit test case for `Error` vs `EXIT` frame handling to ensure frontend can distinguish them
- Consider adding a comment in proto.go explaining why EXIT is a separate frame type from Error (semantic distinction: termination vs error)
- The exitMessage() function should handle the edge case where `WaitStatus` type assertion fails (the plan mentions this but should verify the fallback message)

### Risk Assessment: **LOW** - Well-designed with clear understanding of existing architecture and race conditions. The synchronous write approach is correct.

---

## Plan 01: SIGHUP and Empty Registry Trigger

### Summary
Implements server-side logic for SESS-01/02 - triggering process termination when registry becomes empty. Uses SIGHUP to kill process group, with optional grace period timer. Critical invariant: all triggers funnel through existing lifecycle() → terminate() path (no new exitf branches).

### Strengths
- **Correctly maintains D-13 invariant**: "exitf+sync.Once single termination point" - the plan correctly routes everything through the existing terminate() function
- **Proper detection of "became empty" vs "is empty"**: Pitfall 2 (startup with empty registry shouldn't trigger) is correctly handled by only checking at detach/kick removal points
- **Grace period timer discipline**: All timer operations inside hubMu lock, matching the existing arbiter timer pattern (resize.go:72-80)
- **Good git history reference**: Restores SIGHUP process group kill from P1 implementation (cc03c79~1)

### Concerns

**HIGH**: The plan doesn't adequately address the interaction between the new `exiting` flag and the existing lifecycle code. Looking at server.go:974-988:
```go
s.hubMu.Lock()
clients := make([]*client, 0, len(s.registry.set))
for c := range s.registry.set {
    clients = append(clients, c)
}
s.hubMu.Unlock()
```
The `exiting` flag must be set **before** this snapshot, not after. The plan correctly identifies this (line 5 of Task 2 action), but needs to verify that all code paths are covered.

**HIGH**: The OQ1 decision (exit status 255 vs 0 vs 129) has significant operational implications. The plan correctly flags this as requiring user decision, but the blocking checkpoint doesn't specify what happens if the user chooses different values - the test assertions would need corresponding updates.

**MEDIUM**: The `maybeExitWhenEmptyLocked()` function is called from two places (detach and kick). Need to verify that kick path properly handles the case where the kicked client was the last one:
```go
// From clients.go kickSlowConsumerLocked
s.registry.removeLocked(c) // Does this return true?
```
The plan assumes removeLocked returns true, but should verify this behavior.

**MEDIUM**: Timer callback re-checks `exiting` flag, but doesn't check if SIGHUP has already been sent. If SIGHUP fails (process already dead), the callback would still proceed to the (already-dead) process. This is harmless but could log confusing errors.

### Suggestions
- Add explicit test for `exiting` flag preventing double-SIGHUP when grace timer fires after lifecycle has already started
- Verify that `removeLocked()` return value is reliable for triggering the empty check
- Consider adding a `sighupSent` flag to make timer callback logic clearer (even if not strictly necessary)
- Document the exact exit status values in README based on OQ1 decision

### Risk Assessment: **MEDIUM** - Core logic is sound, but the timing of `exiting` flag relative to registry snapshot is subtle and critical. Multiple timer/event interactions need careful verification.

---

## Plan 02: Frontend Reconnection State Machine

### Summary
Implements CORE-05 frontend logic - automatic reconnection triggered only by 1006 close code, with exponential backoff (1s×2, cap 30s), single-instance reconnect loop, generation guards, and term.clear() on successful reconnect.

### Strengths
- **Precise trigger predicate**: Correctly identifies that only 1006 should trigger reconnect. The existing main.ts:745 default handler handles other codes appropriately.
- **Generation guard pattern**: Using `const sock = ws` closure to detect stale events is elegant and matches existing pattern (main.ts:476)
- **Online/offline event handling**: Properly handles the dual-trigger problem (Pitfall 5) with singleton reconnect loop
- **Clear backoff formula**: `Math.min(1000 * 2 ** attempt, 30000)` matches the throttle.go pattern (1s base, 30s cap)

### Concerns

**HIGH**: The plan claims showStatus needs parameterization for action links, but the current implementation (main.ts:365-381) uses hardcoded "Reload this page":
```typescript
const hint = document.createElement('a');
hint.textContent = 'Reload this page';
hint.href = '#';
```
This is a larger change than implied. The plan should detail the full refactoring:
1. Add optional `action` parameter to showStatus
2. Change all existing callers (there are many)
3. Verify rendering is unchanged for all existing calls

**HIGH**: The `term.clear()` call location is subtle. The plan says it should happen "when WELCOME arrives and reconnecting is set", but the WELCOME handler (main.ts:634-637) doesn't currently check `reconnecting`:
```typescript
welcomeDone = true;
if (beforeunloadEnabled) {
    window.addEventListener('beforeunload', onBeforeUnload);
}
```
Need to add the `if (reconnecting)` check and clear logic.

**MEDIUM**: The plan mentions "panel protection" (Pitfall 7) but doesn't detail how to pass reconnect context to `connect()`. The existing connect() function (main.ts:390-757) has no context parameter. Either:
1. Add a `reconnecting: boolean` parameter to connect(), OR
2. Use module-level state (the plan chooses option 2)

Option 2 is simpler but needs careful state management (the plan's per-connection reset block handles this).

**MEDIUM**: The `lastExit` variable for EXIT frame is defined as module-level, but the plan doesn't specify where in main.ts it should be declared. Should be near `lastError` (around line 643-648).

**LOW**: The "Reconnect now" button in the Reconnecting panel should prevent default to avoid page navigation:
```typescript
action.onClick(); // Should include preventDefault
```

### Suggestions
- Create separate `showStatusWithAction()` function to avoid modifying all existing callers
- Add explicit test for generation guard: old socket close event should not hide new session's panel
- Document the exact placement of `lastExit` and `reconnecting` variables in main.ts
- Verify that `term.clear()` doesn't interfere with OSC 52 clipboard state

### Risk Assessment: **MEDIUM** - The showStatus parameterization is more invasive than described, and the placement of term.clear() needs careful verification. The state machine logic itself is well-designed.

---

## Plan 03: CLI Flags (--once and --exit-when-empty)

### Summary
Implements CLI flags for SESS-01/02 with `--once` as syntax sugar for `--max-clients=1 --exit-when-empty=0`, and `--exit-when-empty[=duration]` using IsBoolFlag pattern for optional value handling.

### Strengths
- **Correct IsBoolFlag usage**: The GOROOT pattern (flag.go:350-356) is correctly applied. When `IsBoolFlag() bool` returns true, `--flag` is equivalent to `--flag=true`, preventing the next argument from being consumed.
- **fs.Visit pattern reused**: Matches existing `writePolicySet` detection pattern (main.go:159-163)
- **Clear layer separation**: parse-time expansion vs validate-time conflict checking is well-structured
- **Conflict detection is comprehensive**: All combinations are enumerated

### Concerns

**HIGH**: The plan's conflict detection logic has a subtle issue. It says:
> `--once` 展开后 maxClients≠1 → 拒绝

But after expansion, how can maxClients≠1? The expansion sets it to 1 if not already set. The conflict should be:
> User explicitly sets `--max-clients` to value ≠ 1 **AND** also sets `--once`

The plan's validation logic needs to check **explicit setting**, not just final value.

**MEDIUM**: The plan's error messages for conflicts use hardcoded strings. These should match the existing error message style in main.go (e.g., `errors.New("--once conflicts with --max-clients: ...")`).

**MEDIUM**: The `exitEmptyValue.Set()` method validates `d < 0`, but `time.ParseDuration` already handles this (returns error for negative durations). The check is redundant but harmless.

**LOW**: The plan says "裸写 = 最后一个客户端断开立即退出", but the flag name `--exit-when-empty` could be misinterpreted as "exit when there are zero clients at startup". The README should clarify this is about the transition to empty state, not the initial empty state.

### Suggestions
- Fix the conflict detection to check explicit setting:
```go
if cfg.once && cfg.maxClientsSet && cfg.maxClients != 1 {
    return "", errors.New("--once conflicts with --max-clients: --once implies --max-clients=1")
}
```
- Add integration test verifying `--exit-when-empty` doesn't trigger at startup
- Document in README that `--once` and explicit `--max-clients=1` are equivalent (not conflicting)

### Risk Assessment: **LOW** - The plan correctly uses Go flag patterns. The conflict detection issue is minor and easily fixed.

---

## Plan 04: jsdom UAT for Reconnection

### Summary
Tests the reconnection state machine (CORE-05) using jsdom with SpyWebSocket to inject synthetic CloseEvent{code:1006} and verify panel transitions, backoff timing, term.clear(), and generation guards.

### Strengths
- **Correct test architecture**: jsdom + real wesh binary + SpyWebSocket matches phase05-dom.mjs pattern
- **Eight scenarios cover critical paths**: D1-D8 map directly to plan risks (Pitfall 5/6/7)
- **beforeunload accounting**: Wraps addEventListener/removeEventListener to verify proper cleanup
- **Skips browser-native events**: Correctly identifies headless limitation and defers to manual testing

### Concerns

**HIGH**: The D1 scenario (reconnect full chain) has a complex test setup. The plan says:
> 以终端 DOM 出现 echo 文本为可观测代理

But the SpyWebSocket can't send INPUT from jsdom (no keyboard events). The plan's fallback (spawn `printf D1BANNER`) is clever but needs verification:
1. Does bash output `printf` result to terminal DOM?
2. After `term.clear()`, is the DOM truly cleared?

**MEDIUM**: The D6 scenario (generation guard) requires keeping a reference to the "old" SpyWebSocket instance. The plan mentions `synthClose` nulls the onclose handler, so it needs a separate mechanism to invoke the stale handler. The plan suggests "synthClose 保留一份处理器副本供二次调用", but this is complex. Simpler approach:
```javascript
// Store handler before synthClose
const oldHandler = oldSock.onclose;
oldSock.synthClose(1006); // Reconnect succeeds
// Manually invoke stale handler
oldHandler({ code: 1006 }); // Should be ignored
```

**MEDIUM**: The plan mentions "token 值永不进 check detail" (red line from phase04.mjs:6-9), but doesn't specify how to verify this in the test script. Need explicit check:
```javascript
// Verify no token in any check() detail or console.log
```

**LOW**: D8 (online fast path) timing is sensitive. The test needs to dispatch `online` event within the 1s backoff window. The plan should specify timing tolerances.

### Suggestions
- Simplify D1: Use server-side marker (`spawn 'echo READY; exec bash'`) and check for READY in terminal DOM
- For D6, use direct handler invocation instead of complex synthClose modification
- Add explicit check that no token/share URLs appear in UAT output (grep for `/s/` patterns)
- Consider using `sinon` or similar for more precise timer control

### Risk Assessment: **MEDIUM** - The test scenarios are well-designed, but the D1 and D6 setup complexity could lead to fragile tests. The token red-line verification is missing.

---

## Plan 05: Protocol Layer UAT

### Summary
End-to-end tests for SESS-01/02/03/CORE-05 using real wesh binary and Node native WebSocket. Seven scenarios verify EXIT frame broadcasting, SIGHUP death, --once behavior, --exit-when-empty modes, and PTY reconnection.

### Strengths
- **Comprehensive scenario coverage**: S1-S6 map directly to ROADMAP success criteria
- **Reuses phase05.mjs infrastructure**: startWesh, dialHello, waitClose patterns are proven
- **EXIT frame assertions are precise**: Checks exit_code, message, frame ordering, and process exit status
- **S3 correctly tests --once**: Verifies both 503 rejection points (HTTP and WS upgrade)

### Concerns

**HIGH**: S5 (grace period tests) has timing sensitivity. The plan says:
> 宽限内再 attach 成功 + echo 会话存活 + 再次断开后到期退出

But doesn't specify:
1. How long to wait before re-attach (400ms is mentioned but not in action steps)
2. How to verify echo works (INPUT/OUTPUT round-trip)
3. What happens if re-attach takes slightly longer than grace period (test flake)

**HIGH**: S6 (PTY reconnection) is the most critical test but has weak evidence:
> 'echo $X\r' 输出含 weshmark42（同一进程证据——变量跨断连存活）

This assumes bash preserves variable across disconnect, but wesh only preserves the PTY. The test should:
1. Write to a file in /tmp (more reliable than shell variable)
2. Or use a unique process ID check

**MEDIUM**: The plan mentions "token 值永不进 check detail" but doesn't implement the check. Add:
```javascript
// After all checks, grep for token patterns in output
```

**MEDIUM**: S3's assertion on process exit status depends on OQ1 decision (06-02 Task 1). If OQ1 is unresolved, the test can't be written.

**LOW**: The `waitExit` helper needs timeout handling. What if wesh hangs? Add watchdog timer.

### Suggestions
- For S5, use exact timing with setTimeout/Atomics.wait for precise grace period testing
- For S6, use file-based evidence: `X=$(cat /tmp/wesh-test-marker-$$)` instead of shell variable
- Add explicit token redaction check in test script
- Document OQ1 decision impact on test assertions

### Risk Assessment: **MEDIUM** - The protocol tests are comprehensive, but S5 timing and S6 evidence method need refinement. The OQ1 dependency is a blocking concern.

---

## Plan 06: Documentation and Final Validation

### Summary
Updates README with lifecycle semantics, creates 06-UAT.md for manual browser testing, syncs 06-VALIDATION.md with actual test commands, and runs full six-segment validation (gofmt, vet, test, build, node --test, UAT).

### Strengths
- **Documentation matches implementation**: README accurately reflects D-12/D-13/D-14 decisions
- **Manual UAT checklist is practical**: Six items cover browser-native behaviors (offline/online events, tmux redraw)
- **Six-segment validation is thorough**: Matches phase05-09 pattern
- **OLD script adaptation discipline**: Clear rules (only add EXIT frame to expected sequences, never remove security assertions)

### Concerns

**MEDIUM**: The plan mentions updating phase02/03 scripts for EXIT frame, but doesn't detail which specific tests need updates. Looking at phase02.mjs, it tests:
- Hello/Welcome exchange
- Error frames
- Close codes (1000, 1002, 1008, 1009, 1011)

Which of these involve child process exit? Only the 1000 normal close case. The plan should explicitly identify which test needs EXIT frame added.

**MEDIUM**: The plan says "go build -o /tmp/wesh-uat/wesh" for smoketest, but doesn't specify what the smoketest checks. Add explicit smoketest assertion:
```sh
# Smoketest: verify --port 0 prints actual port
output=$(/tmp/wesh-uat/wesh --port 0 -- bash -c 'exit 0' 2>&1)
echo "$output" | grep -q "listening on" || exit 1
```

**LOW**: The 06-UAT.md manual checklist items need to be actionable. Instead of:
> 断网 30s 恢复自动重连观感

Should be:
> 1. Open wesh session in browser
> 2. Disable network (airplane mode)
> 3. Wait 30 seconds
> 4. Re-enable network
> 5. Verify: Panel shows "Reconnecting", then automatically reconnects within 5s

### Suggestions
- Explicitly list which phase02/03 test cases need EXIT frame updates
- Add concrete smoketest assertions
- Make 06-UAT.md items more step-by-step actionable
- Add checklist for verifying README token redaction (no real tokens in examples)

### Risk Assessment: **LOW** - This is primarily a documentation and validation plan with minimal implementation risk. The main concerns are completeness and clarity.

---

## Overall Phase Risk Assessment: **MEDIUM**

### Key Findings

1. **Plan 00 (EXIT frame)** is well-designed with correct handling of write-ordering race. LOW risk.

2. **Plan 01 (SIGHUP/Empty trigger)** has subtle timing issues with `exiting` flag placement. MEDIUM risk.

3. **Plan 02 (Reconnection state machine)** requires more invasive showStatus refactoring than described. MEDIUM risk.

4. **Plan 03 (CLI flags)** correctly implements IsBoolFlag pattern but needs conflict detection fix. LOW risk.

5. **Plan 04 (jsdom UAT)** has complex test setup for D1/D6 scenarios. MEDIUM risk.

6. **Plan 05 (Protocol UAT)** depends on OQ1 decision and needs stronger evidence for S6. MEDIUM risk.

7. **Plan 06 (Documentation/validation)** is thorough but needs more concrete action items. LOW risk.

### Critical Blocking Issues

1. **OQ1 Decision Required**: Plan 02's test assertions depend on whether exit status is 255, 0, or 129. This blocks:
   - Plan 02 Task 1 (checkpoint)
   - Plan 05 S3 assertions
   - README documentation

2. **showStatus Refactoring Scope**: Plan 02's parameterization affects all existing callers. Need complete impact analysis before proceeding.

3. **Timing Verification**: Plans 04 and 05 rely on precise timing (400ms grace, 1s backoff, 30s reconnect). Tests may be fragile.

### Recommendations

1. **Resolve OQ1 immediately**: Accept 255 as-is (simplest), map to 0 (matches P1 semantics), or map to 129 (shell convention). Document decision prominently.

2. **Prototype showStatus changes**: Before full implementation, create a branch to verify parameterization doesn't break existing panels.

3. **Add timing tolerance**: In UAT scripts, use polling with tolerance windows rather than exact timing assertions.

4. **Strengthen S6 evidence**: Use file-based or process-ID-based evidence for "same PTY" verification.

5. **Verify token redaction**: Add explicit checks in all UAT scripts that no real tokens appear in output.

---

## Consensus Summary

单 reviewer(OpenCode)——以下为该评审的核心发现,供 `/gsd-plan-phase 6 --reviews` 吸收。

### Agreed Strengths
- EXIT 帧写序竞态(outbox 乱序 vs 1000 close)识别正确,同步 Write→Close 方案成立(06-01)
- D-13 exitf+sync.Once 单一终结点不变式保持;"became empty ≠ is empty" 判定只在移除点触发(06-02)
- 1006 显式触发谓词、`const sock = ws` 代际守卫、backoff 公式与 throttle.go 同构(06-03)
- IsBoolFlag + fs.Visit 显式设置判定、parse/validate 分层(06-04)
- 测试矩阵完整:S1-S6 / D1-D8 直接映射成功准则与陷阱(06-05/06-06)

### Agreed Concerns(最高优先)
1. **OQ1 阻塞链**(HIGH):退出状态 255/0/129 未裁决会阻塞 06-02 测试断言、06-06 S3、README——已在计划中作为 06-02 Task 1 blocking checkpoint 处理,review 确认其关键性
2. **showStatus 参数化侵入面**(HIGH, 06-03):现有全部调用点是硬编码 "Reload this page",需完整影响面分析;建议独立 `showStatusWithAction()` 避免改全部调用方
3. **--once 冲突检测逻辑**(HIGH, 06-04):应检查"显式设置"(cfg.maxClientsSet && cfg.maxClients != 1)而非展开后终值
4. **S6 同-PTY 证据偏弱**(HIGH, 06-06):shell 变量跨断连存活的断言建议改为文件证据(/tmp marker)或进程 ID
5. **exiting 标志与注册表快照时序**(HIGH, 06-02):须先于快照置位;kick 路径 removeLocked 返回值可靠性待验
6. **UAT 时序敏感性**(HIGH, 06-05/06-06):400ms 宽限、1s backoff 窗口建议轮询+容差而非精确断言
7. **token 红线检查缺失**(MEDIUM, 06-05/06-06):需显式断言 token 不进 check detail/输出

### Divergent Views
无(单 reviewer)。

### 处置建议
- 可立即吸收(低争议、指向明确):#3 冲突检测改显式设置判定、#4 S6 文件证据、#7 token 红线断言、06-07 UAT 手册步骤化、smoketest 断言具体化
- 需设计者裁决:#1 OQ1(计划已设门,维持)、#2 showStatus 重构形态(独立函数 vs 参数化)、#5/#6 时序与容差策略(执行期工程判断 vs 计划层锁死)
