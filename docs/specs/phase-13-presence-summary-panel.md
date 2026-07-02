# Phase 13 Presence Summary Panel Spec

Date: 2026-07-02

Status: Draft, waiting for spec approval

Source request: user feedback on `/app` Presence panel growing indefinitely because
the left-side module keeps stretching with accumulated visual output instead of
surfacing concise state.

## Problem

The current `/app` Presence panel is structurally stable but semantically weak:

- the panel is dominated by one large visual block;
- it does not distill the digital human's current state into compact, useful
  information;
- if richer media or repeated visual updates are introduced, the panel risks
  becoming taller and visually noisier instead of more informative;
- the transcript already preserves full turn history, so the Presence area should
  not try to become a second, elongated history surface.

In operator terms: the transcript answers "what was said," while Presence should
answer "what is happening now" and "what matters from the last turn."

## User Need

The user wants the left-side Presence module to behave like an information
summary, not an ever-expanding image container. The panel should extract key
signals and remain compact even as the conversation continues.

## Why This Matters

If Presence keeps expanding or repeating low-value visual state:

- the page loses hierarchy;
- useful status information gets buried;
- the interface feels less professional and less trustworthy;
- future additions such as emotion, memory, grounding, or action summaries will
  have nowhere clean to land.

## Product Reframe

Presence is not a portrait wall.

Presence is a compact operational summary of the digital human's live state and
the most relevant output from the latest completed turn.

That means the panel should prioritize:

1. current runtime state;
2. concise summary of the most recent response;
3. grounding / memory / mode signals;
4. optional visual presence as a bounded accent, not the dominant payload.

## Goals

1. Keep the Presence panel height visually bounded and predictable.
2. Replace low-signal "large static visual" emphasis with high-signal summary
   content.
3. Preserve immediate readability of the digital human's current state.
4. Reuse existing runtime/presentation metadata where possible instead of
   inventing a heavy new backend dependency.
5. Leave room for future multimodal presence without redesigning the page again.

## Non-Goals

- building a full avatar renderer;
- introducing real image generation into the conversation loop;
- summarizing the entire conversation inside Presence;
- replacing the transcript as the primary record;
- adding a new frontend framework.

## Current State

The current `web/app.html` right rail contains:

- `avatar-face`: state-only visual tile;
- `subtitle-line`: freeform textual line;
- diagnostics rows for knowledge, mode, fallback, and audio.

This is a useful base, but it lacks a compact summary model. The panel currently
shows state, not distilled meaning.

## Proposed Experience

Reframe the top card from a pure "Presence" tile into a structured summary card.

### Proposed card structure

1. **Live State**
   - avatar/state chip remains;
   - shows `idle`, `thinking`, `speaking`, `fallback`, `error`, `interrupted`.

2. **Turn Summary**
   - one short summary from the latest assistant turn;
   - target length: 1-3 lines;
   - if the turn is still streaming, show transient text such as "Responding..."
     or a short rolling excerpt;
   - once completed, freeze into a concise summary instead of replaying the full
     assistant response.

3. **Key Signals**
   - compact chips or rows for:
     - knowledge space used;
     - whether knowledge grounding was used;
     - whether memory was used;
     - whether fallback/local response was used.

4. **Optional Detail Toggle**
   - allow expansion into raw subtitle/current response snippet only when needed;
   - default state remains collapsed.

## Summary Strategy

The Presence summary should be intentionally lossy.

Preferred descending strategy:

1. **Explicit backend summary field** if available in future.
2. **Frontend heuristic summary** derived from the final assistant text:
   - trim whitespace;
   - collapse to the first meaningful sentence or clause;
   - clamp by character count and line count;
   - avoid showing boilerplate fallback wording as the main summary if a better
     sentence exists.
3. **State fallback copy** when no trusted final response exists:
   - `Thinking through the request`
   - `Used local fallback response`
   - `Provider response could not be trusted`
   - `Request was interrupted`

This keeps the first implementation cheap while leaving a clear upgrade path for
true model-generated summaries later.

## Design Principles

### 1. Bounded height

The summary card must have stable internal regions and text clamps. No part of
the Presence rail should stretch indefinitely because of long response content.

### 2. Summary over duplication

Do not paste the full assistant response into Presence. The transcript already owns
that job.

### 3. Last-turn focus

Presence should summarize the latest completed turn, not accumulate every turn.

### 4. Trust clarity

If the response was fallback-generated, rejected, or interrupted, Presence should
say that plainly.

### 5. Compatible with future multimodality

If a real image/video/avatar layer is added later, it must fit inside a bounded
slot and never force the summary content off-screen.

## UX Options Considered

### Option A — Keep the large visual panel, just cap its height

Pros:

- smallest implementation;
- low backend impact.

Cons:

- solves stretching, not meaning;
- still leaves the Presence area low-signal.

Verdict: insufficient.

### Option B — Convert Presence into a summary-first card

Pros:

- directly addresses the user's complaint;
- increases information density without adding clutter;
- scales better into later phases.

Cons:

- requires some state derivation and UI restructuring.

Verdict: recommended.

### Option C — Move Presence below transcript on smaller layouts only

Pros:

- helps mobile stacking.

Cons:

- does not fix the core summary problem on desktop;
- treats layout as the cause when the real issue is content strategy.

Verdict: partial improvement only.

## Proposed Scope For Implementation

This feature should be implemented as a focused UI/runtime slice:

### Frontend

- redesign the Presence card layout in `web/app.html` and `web/app.css`;
- add summary-specific DOM nodes in `web/app.js`;
- derive and render compact per-turn summary text;
- clamp overflow and keep card regions stable;
- keep diagnostics readable but visually subordinate.

### Backend

Backend changes are optional for the first cut.

If existing SSE metadata is enough, implement without server changes. Only add a
dedicated summary field if frontend heuristics prove too brittle during Stage 2.

## Acceptance Criteria

1. Presence panel no longer grows with long assistant replies.
2. The top card shows a concise summary of the latest turn rather than full
   repeated content.
3. Fallback, interrupted, and error states produce distinct summary copy.
4. Knowledge grounding and memory signals remain visible without overwhelming the
   card.
5. Desktop and mobile layouts keep readable hierarchy with no text overlap.
6. Existing transcript behavior remains unchanged as the canonical conversation log.

## Risks

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Summary heuristics produce awkward snippets | Medium | Use conservative sentence clamp and fallback state copy |
| Designers over-pack the right rail | Medium | Keep one summary card plus one diagnostics card |
| Streaming text flickers too much | Low | Show stable "responding" copy until final text lands |
| Summary hides important failure state | High | Explicit failure/fallback copy wins over content snippet |

## Test Expectations For Planning Stage

Stage 2 should produce tests covering at least:

- static DOM presence of summary panel elements;
- summary text clamping/formatting behavior;
- fallback/error/interrupted summary rendering;
- no regression to transcript rendering;
- responsive layout checks for the right rail.

## Recommendation

Proceed with a summary-first Presence redesign using frontend heuristics first.

This is the narrowest change that fixes the current UX bug while also improving
the architecture of the app's operator surface.

## Assignment

For the next stage, lock a concrete UI contract around:

1. the exact summary fields shown in Presence;
2. whether summary text is purely frontend-derived or needs a backend field;
3. the responsive layout rules for desktop and mobile.
