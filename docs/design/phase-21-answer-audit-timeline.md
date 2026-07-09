# Phase 21 Answer Audit Timeline Design

Date: 2026-07-08

Status: Approved for Stage 2 planning

Source spec: [Phase 21 Answer Audit Timeline Spec](../specs/phase-21-answer-audit-timeline.md)

## Office-Hours Summary

Phase 20 made a single answer inspectable. Phase 21 should make answer trust
inspectable over time.

The product should not become a compliance console yet. It should help a local
operator see whether the knowledge system is improving, what evidence has been
used recently, and which weak answers still need attention.

The narrow design is:

```text
AuditRecord + knowledge_evidence
  -> AnswerAuditTimelineItem projection
  -> bounded filters
  -> /admin timeline
  -> document detail and gap queue actions
```

## Executive Decision

Use an **Answer Audit Timeline** built from existing audit records.

Do not add a new event-sourcing system, immutable ledger, provider replay
archive, or compliance export in this phase. The existing local audit records
already carry the right raw material: status, conversation ID, agent, latency,
answer state, source count, and evidence metadata.

## Product Shape

The user-facing shape is:

- `/admin` shows a chronological timeline of answer-trust events;
- filters let an operator narrow by weak answers, answer state, document, or
  conversation;
- each item shows trust state, source count, top evidence, diagnostics, and gap
  relationship;
- document buttons open existing document detail;
- weak-answer items point to the knowledge-gap workflow;
- older or incomplete audit records degrade gracefully.

This turns Phase 20 from "I can inspect this answer" into "I can see what has
been happening to answer trust."

## Premises

### Premise 1: Timeline before analytics

Operators need a reliable chronological view before charts or aggregate quality
scores. Timeline records make later analytics explainable.

### Premise 2: Existing audit records are the source of truth

Phase 21 should project existing `AuditRecord` data. A second store would
create consistency problems and unnecessary local persistence complexity.

### Premise 3: Weak answers need action links, not just labels

Unsupported, partially supported, review-gated, fallback, and guard-rejected
states should point to the next useful operator surface: document detail, review
queue, or knowledge gaps.

### Premise 4: Audit does not mean compliance

This feature improves local operations and traceability. It does not claim SOC
2, ISO, legal defensibility, or immutable chain-of-custody.

## Alternatives

### Approach A: Answer Audit Timeline

Project existing audit records into timeline items with filters, source links,
and gap relationship hints.

Pros:

- reuses existing audit and evidence metadata;
- fits the local-first architecture;
- gives operators immediate trust history;
- prepares a clean base for future repair and analytics phases.

Cons:

- limited by what current audit records contain;
- Stage 2 must decide how much user question text is safe to show.

Verdict: recommended.

### Approach B: Knowledge Repair Inbox

Build a task queue from unsupported and partially supported answers.

Pros:

- directly action-oriented;
- likely useful for daily knowledge maintenance;
- naturally connects to the existing gap workflow.

Cons:

- needs timeline context to prioritize well;
- risks duplicating the existing gap queue before its history is visible.

Verdict: defer to a likely Phase 22.

### Approach C: Evidence Quality Analytics

Add aggregate counters, charts, and document quality scorecards.

Pros:

- good high-level operations visibility;
- helps spot never-cited or overused documents;
- appealing for demos.

Cons:

- can overfit small local datasets;
- risks implying quantitative confidence;
- less actionable without the timeline record underneath.

Verdict: defer.

## Recommended Architecture

```text
internal/admin
  AnswerAuditTimelineItem
  TimelineFilter
  TimelineFromAuditRecords
  optional gap relationship resolver

internal/server
  GET /admin/audit/timeline or filtered GET /admin/audit
  parse bounded query filters
  return local deterministic JSON

web/admin.html
  timeline section near Audit
  filters: state, weak-only, document, conversation, limit

web/admin.js
  fetch timeline
  render bounded items
  source buttons call existing inspectKnowledgeDocument
  gap buttons reuse existing gap queue interactions where possible

web/app.css or web/admin styles
  timeline rows/cards with clamped snippets and stable dimensions
```

## Data Contract

Stage 2 should lock exact Go names, but the design should converge on this
shape:

```go
type AnswerAuditTimelineItem struct {
    AuditID        string
    ConversationID string
    UserID         string
    CreatedAt      time.Time
    Status         string
    AgentName      string
    LatencyMS      int64
    AnswerState    string
    SourceCount    int
    Summary        string
    TopSources     []AnswerAuditSource
    Diagnostics    AnswerAuditDiagnostics
    Gap            *AnswerAuditGap
}

type AnswerAuditSource struct {
    DocumentID   string
    Title        string
    ReviewStatus string
    SourceType   string
    Snippet      string
}
```

Rules:

- projection is read-only;
- `AuditRecord` remains the durable source;
- timeline limits are bounded;
- missing metadata maps to stable empty values;
- snippets are bounded before they reach the UI;
- review-gated text remains unavailable.

## Endpoint Design

Stage 2 should choose between:

```text
GET /admin/audit/timeline?state=grounded&limit=50
```

and:

```text
GET /admin/audit?view=timeline&state=grounded&limit=50
```

Recommended first implementation:

- add `GET /admin/audit/timeline`;
- keep the existing audit endpoint/table stable;
- parse `state`, `weak_only`, `document_id`, `conversation_id`, and `limit`;
- clamp `limit` to a small maximum such as 100.

This keeps the old behavior untouched while making the new projection explicit.

## Gap Matching Design

Best case:

- timeline item already has gap IDs in `knowledge_evidence.gaps`;
- render those directly.

Fallback:

- for weak states, match open/investigating/resolved gaps by tenant, space ID,
  no-source reason, and exact question if question text is safely available.

Conservative rule:

- if matching is ambiguous, show "gap unknown" rather than linking the wrong
  record.

Phase 21 should not create new gaps from timeline reads.

## Admin UX Design

### Layout

Use a dedicated "Answer timeline" section near Audit.

Filters:

- state select;
- weak-only checkbox;
- document ID text field or selected document shortcut;
- conversation ID text field;
- limit select or numeric input;
- refresh button.

Timeline item:

```text
[grounded] 2026-07-08 14:21  conv-123
2 sources - Support Playbook, Pricing FAQ
Grounded by 2 reviewed sources.
Agent: persona | completed | 842ms
[Open source] [Filter conversation]
```

Weak item:

```text
[unsupported] 2026-07-08 14:26  conv-123
No supporting source in Default.
Gap: open - needs workbench note
[Open gap queue] [Filter weak answers]
```

### Visual Guardrails

- timeline rows should be dense but readable;
- no nested cards;
- snippets clamp to two or three lines;
- metadata is secondary;
- action buttons should be predictable and small;
- missing data should render as muted text, not raw `undefined`.

## Security And Privacy Design

- Do not expose raw gated source text.
- Do not capture hidden model reasoning.
- Do not show provider secrets or absolute local file paths.
- Keep filter parameters simple strings and bounded limits.
- Treat audit timeline output as operator-local diagnostics, not public UX.

## Test Strategy

Stage 3 should implement with Superpowers TDD:

1. RED: timeline projection orders audit records newest-first.
2. GREEN: add projection helper over `AuditRecord`.
3. RED: same-timestamp records use deterministic ID tie-break.
4. GREEN: stabilize sort.
5. RED: state and weak-only filters return expected records.
6. GREEN: add `TimelineFilter`.
7. RED: document filter matches citation evidence document IDs.
8. GREEN: inspect `knowledge_evidence.citations`.
9. RED: review-gated timeline output never includes raw gated source text.
10. GREEN: diagnostics projection uses reason/count only.
11. RED: server endpoint returns bounded timeline JSON.
12. GREEN: add route and query parsing.
13. RED: `/admin` static tests assert timeline controls and render hooks.
14. GREEN: render timeline UI with bounded source rows.

No tests should require DeepSeek, network, SQLite, external vector services, or
browser automation.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Timeline becomes compliance theater | Users overtrust local diagnostics | Say operations timeline, not compliance archive |
| Wrong gap link | Operator edits wrong source/gap | Link only on explicit or conservative matches |
| Gated text leaks | Review boundary break | Reason/count only for gated diagnostics |
| UI becomes noisy | Operators ignore it | Filters, dense rows, bounded snippets |
| Old audit records break rendering | Admin regression | Unknown/empty fallback values |
| File-backed list grows too large | Slow admin page | Bounded limit and newest-first projection |

## Stage 2 Decisions

Autoplan should lock:

1. endpoint shape;
2. exact projection types and JSON names;
3. default and maximum timeline limits;
4. gap matching rules;
5. whether question text appears in timeline items;
6. whether timeline replaces or complements the existing Audit table;
7. TDD slice order and QA expectations.

## Office-Hours Handoff

Recommended approval:

> Approve Phase 21 as an Answer Audit Timeline that projects existing audit
> records and `knowledge_evidence` into a bounded, filterable `/admin`
> timeline with source links, weak-answer diagnostics, and gap relationship
> hints, without adding compliance export, chain-of-thought capture, or a new
> event store.

After approval, run Stage 2 with `$gstack-autoplan` before any implementation.
