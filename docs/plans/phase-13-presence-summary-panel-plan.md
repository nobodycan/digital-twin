# Phase 13 Presence Summary Panel Plan

Date: 2026-07-02

Status: Shipped on 2026-07-02 via PR #14

Spec: [Phase 13 Spec](../specs/phase-13-presence-summary-panel.md)

## Goal

Turn the `/app` Presence rail from a large state tile into a compact, summary-first
operator panel that stays bounded, communicates the latest trusted turn at a glance,
and preserves the transcript as the primary conversation record.

## Scope

### In

- Presence card restructure in `web/app.html`.
- Bounded-height layout and summary styling in `web/app.css`.
- Frontend summary derivation and state rendering in `web/app.js`.
- Static tests for summary DOM, summary heuristics, and failure-state copy.
- Documentation note in release notes if the shipped UI contract changes materially.

### Out

- Backend-generated semantic summaries.
- New SSE event types or server-side summary fields for the first cut.
- Avatar/image generation work.
- Transcript redesign.
- Admin page redesign.

## What Already Exists

- The right rail already has a stable container: `presence-panel`.
- `avatar-face` already models runtime states such as `thinking`, `fallback`,
  and `error`.
- `subtitle-line` already receives transient text from subtitle/stream events.
- `renderGroundingState()` already exposes knowledge and memory outcome metadata.
- Static tests in [web/app_static_test.go](D:\digital-twin\web\app_static_test.go)
  already guard key `/app` structure.

This means we do not need a new backend contract to fix the user-visible problem.
The right move is to reshape the existing front-end state model.

## What Is Not In Scope

- Summarizing the full transcript history.
- Building a collapsible multi-card timeline in the Presence area.
- Storing summary history server-side.
- Adding model calls purely to summarize model output.

## Premise Challenge

The user's framing is directionally right: the panel should summarize instead of
stretch. The one premise worth challenging is "the left side is getting longer
because of the image itself." In the current code, the more structural issue is
that the Presence card has no bounded summary contract. Even if the visual tile is
small today, any richer text/media will recreate the same problem.

So the plan should solve the deeper premise:

- Presence must be a stable information product, not an unbounded rendering slot.

## Dream State Delta

Today, Presence mostly answers "what state is the avatar in?" The desired state is:

- top glance: current live state;
- second glance: what the assistant just concluded;
- third glance: whether that conclusion was grounded, memory-assisted, or fallback.

That is a stronger operator console than simply capping an image height.

## Architecture

```text
SSE events from /experience/stream
  -> web/app.js event state machine
    -> latest trusted assistant text
    -> summary heuristic
    -> presence status signals
      -> web/app.html summary slots
      -> web/app.css bounded layout

Transcript remains full-fidelity log in parallel.
Presence becomes latest-turn summary view only.
```

## Data Flow

### Startup

1. `/app` loads current runtime/provider status as it does today.
2. Presence panel renders default summary copy such as `Ready for the next turn`.
3. No historical transcript content is copied into Presence.

### Active turn

1. User submits a message.
2. `app.js` moves avatar/status to `thinking`.
3. Presence summary shows transient active copy:
   - `Thinking through the request`, or
   - a short rolling excerpt if we keep lightweight streaming preview.
4. Transcript continues to stream full assistant deltas independently.

### Completed trusted turn

1. `done` event arrives with completion metadata.
2. `app.js` derives a concise summary from the final assistant text.
3. Presence summary freezes to the latest-turn summary.
4. Signal chips/rows update from completion metadata:
   - knowledge used / not used;
   - memory considered / not considered;
   - fallback/local mode if applicable.

### Failure / interruption

1. If the request fails, is interrupted, or falls back:
   - summary copy comes from state-first fallback strings, not transcript text.
2. Transcript still shows partial/error detail.
3. Presence remains bounded and truthful.

## UI Contract

The top card should be restructured into four compact regions:

1. **Presence header**
   - label such as `Presence`;
   - live-state chip/value.

2. **Visual slot**
   - bounded avatar tile retained;
   - fixed aspect and max height;
   - never expands because of text.

3. **Summary block**
   - new node, e.g. `presence-summary`;
   - 1-3 line clamp;
   - latest final summary or active-state copy.

4. **Signal strip**
   - compact chips or small rows for:
     - knowledge space;
     - grounded/not grounded;
     - memory considered;
     - fallback/local/trusted mode.

Optional:

- detail disclosure for raw subtitle snippet if it can be added without increasing
  cognitive noise. This is not required for the first implementation.

## Summary Heuristic Decision

Decision: keep summary generation frontend-derived for the first cut.

Rationale:

- existing final assistant text is already available in `app.js`;
- summary here is presentation logic, not domain logic;
- we avoid adding new backend metadata and tests across runtime/server layers;
- if the heuristic feels weak after QA, we can add a backend summary field in a
  later phase without undoing the UI structure.

## Summary Heuristic Rules

`app.js` should derive summary text in this order:

1. failure-state copy if the turn is `fallback`, `error`, or `interrupted`;
2. final assistant text, normalized and trimmed;
3. first meaningful sentence or clause;
4. character clamp and line clamp;
5. if no trustworthy content, use stable idle/active copy.

Recommended state copy:

- idle: `Ready for the next turn`
- thinking: `Thinking through the request`
- speaking: `Responding now`
- fallback: `Used a local fallback reply`
- error: `Provider response could not be trusted`
- interrupted: `Request was interrupted`

## Design Review Conclusions

### CEO

The highest-value move is not adding more presence media. It is clarifying what
matters now. This plan keeps scope disciplined and directly fixes an operator
usability issue.

### Design

The rail should become denser and calmer:

- one summary card instead of a visually dominant empty stage;
- bounded text, no runaway height;
- diagnostics subordinated below the summary layer;
- clear visual hierarchy between live state, latest takeaway, and support signals.

### Engineering

The cheapest robust implementation is frontend-only. The browser already has the
event stream and completion metadata needed to synthesize this summary. New server
contracts would add blast radius without being necessary for the bug fix.

### DX

The plan keeps implementation local to `web/` static assets and tests. That makes
the TTHW for contributors low: change DOM/CSS/JS, run `go test ./web`, verify in
browser.

## Architecture Work Items

| ID | Area | Files | Outcome |
| --- | --- | --- | --- |
| P13-01 | DOM contract | `web/app.html` | Presence card includes summary and signal regions |
| P13-02 | Layout contract | `web/app.css` | Presence rail has bounded summary layout with stable dimensions |
| P13-03 | Summary logic | `web/app.js` | Latest trusted turn is converted to concise summary text |
| P13-04 | Failure semantics | `web/app.js` | Fallback/error/interrupted states render explicit summary copy |
| P13-05 | Static regression coverage | `web/app_static_test.go` | Required selectors/strings for summary UI are enforced |
| P13-06 | Optional doc note | `README.md`, `RELEASE_NOTES.md` | User-facing app description stays accurate if needed |

## TDD Execution Plan

### P13-01 Presence Summary DOM

RED:

- Add static tests requiring new summary nodes and labels in `app.html`.
- Assert the old Presence card still includes `avatar-face` but now also includes
  summary and signal containers.

GREEN:

- Update `web/app.html` with summary-first structure.

REFACTOR:

- Keep semantic labels short and consistent with existing app wording.

Commands:

```powershell
go test ./web -run Presence
```

### P13-02 Bounded Summary Layout

RED:

- Add static tests for new CSS hooks such as summary block, clamped text region,
  and bounded visual slot.
- Assert presence styles contain explicit stable sizing primitives such as
  `max-height`, `minmax`, `aspect-ratio`, or equivalent bounded layout rules.

GREEN:

- Update `web/app.css` to enforce bounded height and clear component regions.

REFACTOR:

- Keep CSS grouped by Presence panel components.

Commands:

```powershell
go test ./web -run Styles
```

### P13-03 Summary Derivation

RED:

- Add JS/static tests requiring:
  - summary DOM references;
  - helper for summary derivation;
  - final-turn summary updates on `done`;
  - active-state summary updates while thinking/speaking.

GREEN:

- Implement summary helpers in `web/app.js`.

REFACTOR:

- Keep state rendering helpers small and avoid mixing transcript logic with Presence
  logic more than necessary.

Commands:

```powershell
go test ./web -run AppScript
```

### P13-04 Failure-State Copy

RED:

- Add tests requiring explicit summary copy or code paths for:
  - fallback;
  - error;
  - interrupted.

GREEN:

- Update `web/app.js` to prefer state-first copy over raw text in these cases.

REFACTOR:

- Centralize presence summary copy in one helper or constant map.

Commands:

```powershell
go test ./web -run AppScript
```

### P13-05 Responsive Integrity

RED:

- Extend static tests for mobile-safe Presence structure.
- Ensure the summary region exists without relying on transcript duplication.

GREEN:

- Finalize layout adjustments in `web/app.css`.

REFACTOR:

- Remove stale selectors if the old subtitle-only presentation becomes redundant.

Commands:

```powershell
go test ./web
```

### P13-06 Docs

RED:

- Only if final shipped wording changes app usage expectations.

GREEN:

- Update `README.md` or `RELEASE_NOTES.md` with one concise note.

REFACTOR:

- Keep docs truthful and lightweight.

Commands:

```powershell
rg -n "Presence|summary|operator" README.md RELEASE_NOTES.md web
```

## Test Matrix

| ID | Area | Scenario | Expected |
| --- | --- | --- | --- |
| T13-01 | HTML | app shell includes new Presence summary node | Presence card exposes summary-first structure |
| T13-02 | HTML | app shell includes signal region | Grounding/memory/fallback signals have dedicated slot |
| T13-03 | CSS | Presence layout defines bounded visual slot | Avatar/media area cannot grow indefinitely |
| T13-04 | CSS | Summary text region is clamped/bounded | Long assistant replies do not stretch the rail |
| T13-05 | JS | startup state | Presence summary renders idle default copy |
| T13-06 | JS | thinking state | Presence summary switches to active-state copy |
| T13-07 | JS | successful done event | Presence summary derives compact final summary |
| T13-08 | JS | fallback done event | Presence summary shows explicit fallback copy |
| T13-09 | JS | error event after partial stream | Presence summary shows trusted failure copy |
| T13-10 | JS | interrupted request | Presence summary shows interruption copy |
| T13-11 | JS | grounding metadata | Presence signal strip still reflects knowledge/memory state |
| T13-12 | Web | full static suite | Existing transcript/provider controls do not regress |

## Failure Modes Registry

| Mode | Trigger | User Sees | Mitigation |
| --- | --- | --- | --- |
| F13-01 | Long assistant output | Presence rail grows or feels duplicated | Summary clamp + transcript/presence separation |
| F13-02 | Fallback reply | Summary misleadingly looks like a trusted model answer | State-first fallback copy and signal chip |
| F13-03 | Stream error after partial output | Summary shows broken fragment as if final | Error copy overrides raw partial text |
| F13-04 | Mobile narrow width | Summary/signals overlap or wrap badly | Stable layout primitives and Stage 5 QA |

## Error And Rescue Registry

| Error | Likely Cause | Rescue |
| --- | --- | --- |
| Static tests fail on missing selectors | DOM/CSS/JS contract drift | Update implementation to match locked selectors |
| Presence summary duplicates transcript too much | Heuristic too naive | Tighten clause extraction and clamp rules |
| Signal strip becomes noisy | Too many rows/chips | Reduce to the four approved signals only |
| Subtitle behavior conflicts with summary | Shared state mutation in `app.js` | Separate transient subtitle text from final summary state |

## Implementation Order

1. P13-01 DOM contract.
2. P13-02 bounded layout hooks.
3. P13-03 summary derivation logic.
4. P13-04 failure-state semantics.
5. P13-05 full static regression pass.
6. P13-06 docs only if wording changed.

## Parallelization

Safe parallelism after DOM contract is settled:

- CSS bounded-layout work and JS summary logic can proceed independently.
- Docs can wait until the end.

Do not parallelize DOM restructuring and JS selector wiring blindly, because both
touch the same UI contract.

## Acceptance Commands

```powershell
go test ./web
rg -n "presence-summary|summary|fallback|interrupted|knowledge-space-status" web
```

Manual QA after Stage 3:

1. Start the local server.
2. Open `http://localhost:8080/app` or the configured local port.
3. Send:
   - a short normal message;
   - a long message that produces a longer answer;
   - a flow that triggers fallback or interruption.
4. Confirm:
   - transcript keeps full answer;
   - Presence shows only compact summary;
   - right rail height remains visually stable.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Solve the Presence problem with summary-first information design | Auto-decided | User value density | The user asked for key information extraction, not a bigger portrait treatment | Visual-only height cap |
| 2 | Design | Keep a bounded visual slot but demote it below summary semantics | Auto-decided | Hierarchy clarity | The interface should prioritize what matters from the last turn | Visual-first card |
| 3 | Eng | Keep the first cut frontend-only with no new backend summary contract | Auto-decided | Blast radius control | Existing stream state is enough for a reliable first implementation | New SSE/server summary field |
| 4 | DX | Keep work scoped to `web/` plus static tests | Auto-decided | Fast iteration | Contributors can validate quickly without touching backend packages | Cross-layer refactor for a UI bug |

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | --- | --- | --- |
| CEO Review | `$gstack-autoplan` | Scope and product hierarchy | 1 | clear | Summary-first is the right product slice; richer media deferred |
| Design Review | `$gstack-autoplan` | UI/UX contract | 1 | clear | Presence needs bounded regions and latest-turn abstraction |
| Eng Review | `$gstack-autoplan` | Architecture and testability | 1 | clear | Frontend-only implementation minimizes risk and is testable |
| DX Review | `$gstack-autoplan` | Contributor flow | 1 | clear | `web/` static tests give fast feedback and low TTHW |

**VERDICT:** CEO + DESIGN + ENG + DX reviewed. Ready for Stage 3 after user approval.

NO UNRESOLVED DECISIONS
