# Phase 07 — UI Review

**Audited:** 2026-08-27
**Baseline:** abstract 6-pillar standards + inherited design language from 01/04/05/06-UI-SPEC (no 07-UI-SPEC exists; the phase contract explicitly mandates "零新 UI 组件" — reuse of the phase 04/06 panel design language is the contract)
**Screenshots:** not captured (no dev server — wesh cannot run on the Windows audit host; ports 3000/5173/8080 all dark; code-only audit per session constraints)
**Scope note:** Phase 07 is a deployment/ops phase. The only UI surfaces touched are (1) 07-01 relative fetch/WS URL construction (zero visual surface) and (2) 07-05 the 1001 "Server shutting down" terminal-state panel dispatch. README copy is ops documentation, out of pillar scope. Graded within the terminal-in-browser idiom (xterm.js + status panels) — marketing-page richness is not demanded.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | 1001 copy is contract-conformant three-part English, but the hint "Start wesh again from your shell" is wrong for the systemd auto-restart scenario D-23 itself cites as motivation |
| 2. Visuals | 3/4 | Panel structurally identical to phase 04/06 panels (single `showStatus` write path); inherited gap: `#status` has no ARIA live semantics, and the terminal-state set it fails to announce just grew by one |
| 3. Color | 4/4 | Zero CSS changes; accent discipline structurally enforced — exactly one accent consumer in the stylesheet (`.status-hint a`, `#729fcf`) and the 1001 panel uses the default slot |
| 4. Typography | 4/4 | Scale unchanged: 3 sizes (12/14/16), 2 weights (400/600); all 1001 text lands in the three existing Status tiers |
| 5. Spacing | 4/4 | 8-based scale (24/8/16 padding & gaps, 480px max-width); 1001 panel consumes the same component — zero new spacing values |
| 6. Experience Design | 4/4 | New state coverage is exemplary: steady-state + reconnect-context 1001 both runtime-locked by D11a–d UAT assertions; beforeunload removed; input gated; EXIT×1001 race explicitly modeled |

**Overall: 22/24**

---

## Top 3 Priority Fixes

1. **1001 hint copy mismatches the auto-restart deployment shape** — under `systemd Restart=always` (the scenario D-23 names as the UX-loop motivation), wesh restarts itself and the user needs no shell at all; the inherited C-7 hint "Start wesh again from your shell, then" sends them on a false errand — change the `case 1001` hintPrefix at `web/src/main.ts:903` to a conditional form, e.g. `"If wesh is not restarted for you, start it again from your shell, then"` (keeps the `{hint} [Reload this page].` structure; the reload action itself is already correct in all shapes).
2. **`#status` panel has no ARIA live semantics** — terminal states (now including server shutdown) are never announced to assistive technology; the panel is plain DOM so this is a one-attribute fix — add `role="alert"` to the `#status` container at `web/index.html:63` (zero visual impact; fixes the whole panel family at once, not just 1001).
3. **Server shutdown during the pre-`onopen` window lands on "Unable to connect" copy** — a client whose WS handshake hasn't completed when wesh goes down gets C-4 copy claiming the server "is refusing new connections", which mis-describes a shutdown — optionally dispatch `ev.code === 1001` to the shutdown panel ahead of the `!opened` branch at `web/src/main.ts:881-884` (one condition line; millisecond race, so WARNING-low).

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

**Contract baseline:** 05/06-UI-SPEC §Copywriting — English copy (P2 convention), three-part panel structure (title / body / hintPrefix + accent action link rendering as `{hint} [action].`), no generic labels.

**What checks out:**
- `case 1001` copy (`web/src/main.ts:900-904`): title `"Server shutting down"` / body `"The wesh server is shutting down. The session has ended."` / hint `"Start wesh again from your shell, then"` → renders `"Start wesh again from your shell, then [Reload this page]."` — three-part structure, English, no "Submit/OK/Cancel" patterns (repo-wide scan clean).
- Semantic boundary vs `case 1000` is explicit and correct: 1000 = process exited (body sourced from EXIT frame message), 1001 = service shutdown (fixed body, since no EXIT frame precedes it — `main.ts:895-899` comment documents the boundary). "The session has ended" is precisely true: the child process is killed by the server-side stop-signal sequence.
- Copy is locked at runtime: `web/uat/phase06-dom.mjs:608-611` (D11a) asserts body + hint verbatim, and the dist product contains the literal (`grep -c 'Server shutting down' web/dist/index.html` == 1).

**WARNING — hint inherited from C-7 is wrong for the headline scenario (finding #1):** the hint is byte-identical to `case 1000` (`main.ts:892`), where it is precisely true — process exit means wesh has stopped and only an operator shell can bring it back. For 1001, D-23's stated motivation is the `systemd restart` UX loop (07-CONTEXT D-23; 06-CONTEXT §specifics "1001 的 UX 闭环价值"), and under `Restart=always` wesh returns in about a second with **no shell action required** — the only correct user action is the reload link itself. The panel copy instructs the user to do something the system is already doing. Impact is confusion, not task breakage (the reload action works in all deployment shapes; a share-link client reloading into regenerated tokens degrades gracefully to the C-3 "Invalid share link" panel). Hence 3/4, not 2/4.

**Below fix-bar observation:** title and body both use present progressive ("shutting down") although shutdown is often complete by render time; the "Reconnecting" precedent makes progressive aspect house-legal, and "The session has ended" carries the durable truth. Not flagged as a defect.

### Pillar 2: Visuals (3/4)

**Contract baseline:** 01/04-UI-SPEC §Page Shell — three top-level elements (`#terminal` / `#status` / `#resize-overlay`), z-order 900 < 901 < 1000, single-panel invariant.

**What checks out:**
- Zero new visual elements this phase. The 1001 panel dispatches through the same `showStatus` write path (`main.ts:455-475`) into the same `#status` markup (`web/index.html:63-69`) — visual consistency with the phase 04/06 session-ended/disconnect panels is guaranteed **by construction**, not by diligence.
- Focal point: full-viewport dim (`rgba(0,0,0,0.6)`) + centered panel at z-1000 over the terminal; when the panel shows, it is the only readable element. ✓
- Hierarchy: 16/600 white title → 14/400 `#a3a3a3` body → 12/400 `#a3a3a3` hint with a single accent link. Size, weight, and color all differentiate tiers. ✓
- No icon-only buttons exist anywhere in the app — aria-label pairing not applicable.
- Dist product verified to carry the panel CSS (`status-hint` ×4 in `web/dist/index.html`).

**WARNING — no ARIA live semantics on the panel family (finding #2, inherited):** `#status` is a bare `<div hidden>` (`web/index.html:63`); nothing announces its appearance. Pre-existing since phase 01 and out of phase-07 scope to mandate, but surfaced here because the phase enlarged the terminal-state set it fails to announce — a screen-reader user whose server shuts down receives no notification at all. Graded 3/4: the phase introduced no visual defect, but the pillar audits the implemented UI as it stands and this is a real, cheaply-fixable gap (`role="alert"` on the container announces every panel variant at once).

### Pillar 3: Color (4/4)

**Contract baseline:** accent reserved for the status-hint action-link slot; at most one accent element visible at any time (06-UI-SPEC §Color: "accent 使用点结构上唯一不变").

**Evidence:**
- Stylesheet color inventory is six values, all on-palette: `#000000` (page bg), `#161616` (panel/tooltip bg), `#2a2a2a` (borders), `#ffffff` (title/foreground), `#a3a3a3` (secondary text), `#729fcf` (accent, exactly one consumer: `.status-hint a`, `web/index.html:31`). The accent is tango brightBlue — the same hue as the terminal palette's `brightBlue` (`web/src/main.ts:54`), so the single accent element is coherent with the dominant surface.
- Distribution: black terminal canvas ≈60 / dark-gray panel chrome ≈30 / one accent link <10. The 1001 panel uses the **default** action slot ("Reload this page") — no new accent consumption point, and the single-panel invariant caps simultaneous accent elements at one.
- Hardcoded hex is the established single-file design-token discipline (one `<style>` block, byte-stable since phase 01, zero external resources by design) — accepted by design, not flagged.
- Phase 07 changed zero CSS. Verified: no color-related diff in `web/index.html` this phase (changes were confined to `main.ts` URL construction + the onclose case).

### Pillar 4: Typography (4/4)

**Contract baseline:** 3 sizes (12/14/16), 2 weights (400/600); panel text lands in Status heading (16/600/1.2) / Status body (14/400/1.5) / Status hint (12/400/1.5) tiers (06-UI-SPEC §Typography).

**Evidence:**
- All three 1001 strings render through the existing tier classes (`web/index.html:28-30`) — no new size, weight, or line-height consumption point.
- Font-family boundary preserved: system-ui sans for the panel, mono stack for terminal/resize-overlay/link-tooltip (`index.html:19,42,54`).
- 07-01's URL changes and 07-05's dispatch added zero typographic surface. Scale discipline: 3 sizes / 2 weights is within the ≤4/≤2 abstract ceiling with headroom.

### Pillar 5: Spacing (4/4)

**Contract baseline:** panel padding 24px, title→body 8px, body→hint 16px, max-width 480px, radius 8px (05/06-UI-SPEC §Spacing, "零新间距消费点").

**Evidence:**
- The 1001 panel consumes the same component — spacing inherits 24/8/16/480/8 verbatim (`web/index.html:24-27,29-30`). All values are multiples of 8 (or 4, the sub-step): overlay `4px 8px` padding, 8px viewport offsets, tooltip 8px pointer offset — one coherent scale, zero arbitrary values.
- No arbitrary-value patterns (`[Npx]`/`[Nrem]`) exist; the project has no utility-class layer at all — spacing lives in one audited `<style>` block, which makes drift structurally difficult.

### Pillar 6: Experience Design (4/4)

**Contract baseline:** 06-UI-SPEC §State Coverage — every close code has a designed terminal state; 1001's contract (07-CONTEXT D-23): terminal panel, never reconnect, UX loop closed for systemd restart.

**State coverage evidence (the strongest pillar this phase):**
- **Steady-state 1001** → terminal panel, zero reconnect: runtime-locked by D11a (verbatim panel) + D11b (2.5s watch window — zero new `WebSocket` constructions, so the backoff loop provably never started) (`web/uat/phase06-dom.mjs:603-617`). The plan's prohibition ("MUST NOT auto-reconnect on 1001") has a *runtime regression lock*, not just a code comment.
- **Reconnect-context 1001** → loop terminated, backoff timer cleared, terminal panel dispatched: D11c/d (`phase06-dom.mjs:621-649`); code path verified at `main.ts:874-880` (1006 reschedules, any other code stops the loop, then the switch dispatches).
- **Post-panel hygiene:** `beforeunload` listener removed on any close (`main.ts:858-860`) — the page can be closed after shutdown without a native "leave site?" prompt; keyboard input during the panel is silently gated by the readyState check (`main.ts:305-309`, per UI-SPEC §Interaction Contract).
- **Race honesty:** EXIT×1001 ordering is explicitly modeled (T-07-05d) — the client may show either "Session ended" or "Server shutting down" depending on arrival order, and both are semantically true. Accepted with reasoning, not discovered by accident.
- **Loading state:** the open black terminal *is* the loading state (terminal opens synchronously before WS, `main.ts:110` — deliberate, documented); the 1001 panel is a terminal state needing no spinner. Empty/disabled/destructive-confirmation states: not applicable to this surface (no buttons, no lists, no destructive actions).
- **Existing dispatch zero-regression:** 1000/1006/1013 paths byte-unchanged and locked by D1/D2/D3/D7 in the same suite (37/37 pass per 07-05-SUMMARY).

**Below fix-bar observation (finding #3, demoted):** a client whose `onopen` hasn't fired when the server goes down lands on the generic C-4 "Unable to connect" copy (`main.ts:881-884`), which says the server "is refusing new connections" — directionally right for a dead server, imprecise for a graceful shutdown. The window is milliseconds (and a pre-Hello client isn't in the hub registry, so it won't even receive a 1001 — it sees the socket die when the process exits). Listed as optional polish, not a defect; the dispatch shape is inherited and the copy failure mode is benign.

---

## Files Audited

- `web/src/main.ts` (972 lines — full read; focus: `showStatus` L455-475, relative URL construction L505-613, onclose dispatch L854-953, copy constants L426-447)
- `web/index.html` (74 lines — full read; panel/overlay/tooltip CSS and markup)
- `web/uat/phase06-dom.mjs` (D11 scenario L591-649; scenario registry L672)
- `web/dist/index.html` (product verification via grep: `Server shutting down` ×1, `../../` ×1 as template literal, `api/attach` ×1, `status-hint` ×4)
- `.planning/phases/07-deployment/07-01-PLAN.md`, `07-01-SUMMARY.md`, `07-05-PLAN.md`, `07-05-SUMMARY.md`, `07-CONTEXT.md`
- Design-language baseline: `.planning/phases/06-session-lifecycle/06-UI-SPEC.md`, `.planning/phases/05-multi-client/05-UI-SPEC.md` (Copywriting/Color/Typography/Spacing/State Coverage sections)

**Registry audit:** skipped — no `components.json` (no shadcn) and no 07-UI-SPEC listing third-party registries.

**Screenshot gate:** `.planning/ui-reviews/.gitignore` created before any capture attempt; dev-server detection attempted on ports 3000/5173/8080 (all dark) — no screenshots captured, code-only audit as session constraints prescribe.

---
*Phase: 07-deployment*
*Audited: 2026-08-27*
