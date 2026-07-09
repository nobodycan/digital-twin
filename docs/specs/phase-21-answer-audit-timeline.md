# Phase 21 Answer Audit Timeline Spec

Date: 2026-07-08

Status: Approved for Stage 2 planning

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 8 Real Conversation Loop Spec](./phase-8-real-conversation-loop.md)
- [Phase 14 Knowledge Operations Console Spec](./phase-14-knowledge-operations-console.md)
- [Phase 15 Knowledge-Grounded Answer Loop Spec](./phase-15-knowledge-grounded-answer-loop.md)
- [Phase 19 Knowledge Review and Activation Spec](./phase-19-knowledge-review-activation.md)
- [Phase 20 Knowledge Evidence and Answer Trust Spec](./phase-20-knowledge-evidence-answer-trust.md)
- Current implementation: durable conversation turns, local admin audit records,
  `knowledge_answer_state`, persisted `knowledge_evidence`, recent `/admin`
  audit rows, knowledge gaps, review-gated retrieval, and document detail links.

## Context

Phase 20 made individual knowledge-backed answers inspectable. A user can now
see answer state, citations, snippets, review-safe diagnostics, and source
quality signals for a single turn.

The next product question is:

> Can an operator understand how answer trust changes over time?

Today the system has enough pieces to answer that question partially:

- `internal/admin.AuditRecord` stores conversation ID, status, agent, latency,
  event summary, `knowledge_answer_state`, source count, and
  `knowledge_evidence`;
- `/admin` already renders a compact audit table with trust and evidence;
- knowledge gaps capture unsupported, partially supported, and review-gated
  turns;
- document detail can show source metadata, review state, quality flags, and
  source/gap relationships.

But the experience is still record-shaped rather than timeline-shaped. It is
hard to see patterns: repeated unsupported questions, repeated review-gated
events, the same document being cited across turns, or a gap that later becomes
resolved evidence.

## Office-Hours Premise Challenge

The tempting version of Phase 21 is a broad compliance console:

- immutable audit-event ledger;
- exportable SOC 2 or ISO evidence packages;
- full replay of answer generation;
- user/session identity timelines;
- model prompt and token capture;
- policy decision graphs;
- security incident workflows;
- cross-tenant reporting.

That is too large for this local-first phase. It also risks collecting more
sensitive text than the product needs.

The opposite temptation is to stop at the Phase 20 audit table. That would miss
the professional operator workflow. A single row can prove that one answer had
evidence; it cannot show whether the knowledge base is getting healthier.

The narrow move is an **Answer Audit Timeline**:

1. project existing audit records into a chronological, filterable timeline;
2. summarize answer trust, evidence count, cited documents, and gap status per
   turn;
3. connect each timeline item to document detail and gap workflow;
4. keep the data local-first and bounded;
5. avoid compliance claims, raw prompt capture, and chain-of-thought replay.

## Landscape Notes

NIST's AI Risk Management Framework emphasizes managing AI risk across the AI
lifecycle and names transparency/documentation as important for reducing
inscrutability: [NIST AI RMF](https://www.nist.gov/itl/ai-risk-management-framework).

Microsoft's responsible AI guidance similarly frames transparency as explaining
why an AI solution was chosen, how it is monitored, and how it is updated:
[Microsoft Copilot Studio responsible AI guidance](https://learn.microsoft.com/en-us/microsoft-copilot-studio/guidance/responsible-ai).

OWASP's LLM work keeps security attention on monitoring, logging, and controls
for LLM applications rather than treating model output as inherently trustworthy:
[OWASP Top 10 for LLM Applications](https://owasp.org/www-project-top-10-for-large-language-model-applications/).

Phase 21 should borrow the principle, not the enterprise scope: reconstruct what
happened well enough for a local operator to inspect and improve trust.

## Product Thesis

A professional digital human should make its trust trajectory visible.

The operator should be able to answer:

- Which recent answers were grounded, partial, unsupported, review-gated, or
  fallback?
- Which source documents repeatedly supported answers?
- Which questions repeatedly failed to find usable evidence?
- Which review-gated events need source activation?
- Which gaps were created from weak answers, and which have since been resolved?
- Did recent knowledge changes improve answer trust?

## Goal

Deliver a local-first Answer Audit Timeline:

1. define a stable audit timeline projection from existing audit and knowledge
   records;
2. expose a bounded `/admin` timeline view with filters and document/gap links;
3. keep each timeline item compact, chronological, and inspectable;
4. preserve review-gate safety and avoid raw gated source text;
5. connect weak-answer states to existing knowledge-gap operations;
6. add deterministic tests that do not require DeepSeek, network, SQLite,
   external vector services, or browser automation.

## Recommended Narrow Wedge

Ship Phase 21 as an operations timeline, not as a compliance archive.

The first slice should add:

- an `AnswerAuditTimelineItem` projection over existing `AuditRecord` values;
- filters for answer state, document ID, gap status, conversation ID, and recent
  limit;
- deterministic timeline ordering by `created_at` descending with stable ID
  tie-breaks;
- `/admin` UI that renders timeline rows/cards with trust state, question or
  turn summary when safely available, source count, top sources, gap status,
  and action buttons;
- links from timeline evidence sources to document detail;
- links from weak timeline items to the knowledge-gap queue when a matching gap
  exists;
- no new database, no provider call, no compliance export, and no raw hidden
  reasoning capture.

## In Scope

### Timeline Contract

- Add a typed projection for answer-audit timeline items.
- Reuse existing audit records and knowledge evidence rather than introducing a
  second event store.
- Include fields needed for `/admin` rendering and future Stage 2 planning:
  - audit ID;
  - conversation ID;
  - user ID;
  - created at;
  - status;
  - agent name;
  - latency;
  - answer state;
  - source count;
  - top citation summaries;
  - gap relationship summary where available;
  - review-gated/no-source reason where available.

### Timeline Filters

- Filter by answer state.
- Filter by document ID when citation evidence contains that document.
- Filter by conversation ID.
- Filter by weak-only states: unsupported, partially supported, review-gated,
  provider fallback, guard rejected.
- Limit result count deterministically.

### Admin Timeline UX

- Add an `/admin` answer-audit timeline surface near the current Audit section.
- Show chronological timeline items, not a dense raw audit table.
- Keep evidence snippets bounded.
- Make document links and gap actions visible without turning the page into a
  dashboard wall.
- Preserve the existing Audit table behavior unless Stage 2 deliberately merges
  it into the timeline.

### Knowledge Gap Connection

- For unsupported, partially supported, and review-gated states, surface whether
  a matching open/investigating/resolved gap exists.
- Prefer deterministic matching by stored IDs if already present; otherwise use
  conservative query/space/reason matching.
- Do not create new gaps from timeline rendering. Gap creation remains tied to
  answer completion.

### Safety And Privacy

- Do not store chain-of-thought or hidden reasoning.
- Do not expose raw pending, rejected, or archived source text.
- Do not include API keys, local absolute file paths, or provider secrets.
- Keep timeline limits bounded so local file-backed storage remains responsive.

## Out Of Scope

- Compliance certification or SOC 2/ISO evidence export.
- Immutable append-only ledger.
- Cross-tenant analytics.
- RBAC, reviewer assignment, or multi-user audit policy.
- Prompt/token capture beyond existing sanitized metadata.
- Full replay of provider requests.
- LLM-as-judge grading or confidence percentages.
- New database, queue, or event-sourcing system.
- Bulk knowledge repair workflow; that belongs in a later phase.

## Alternatives Considered

### A. Answer Audit Timeline

Project existing audit records into a filterable timeline with trust state,
evidence summaries, source links, and gap relationship hints.

Recommended because Phase 20 already created per-answer evidence. The missing
layer is time.

### B. Knowledge Repair Inbox

Turn weak answers into a task queue for operators to prioritize, fix, retest,
and close.

Deferred because repair work is stronger once timeline patterns are visible.
This should likely become Phase 22.

### C. Evidence Quality Analytics

Aggregate source usage, weak-answer frequency, never-cited documents, and
review-gated rates into charts or scorecards.

Deferred because analytics can overfit small local data and may become a
dashboard before the underlying event projection is reliable.

## Proposed Architecture

```text
admin audit records
  -> timeline projection
  -> answer-state/document/conversation filters
  -> /admin timeline endpoint or existing audit endpoint extension
  -> web/admin timeline surface

knowledge gaps + evidence citations
  -> optional gap relationship summary
  -> document detail and gap queue links
```

## Timeline Object Draft

Stage 2 should lock exact names. The minimum contract is:

```text
answer_audit_timeline_item:
  audit_id: string
  conversation_id: string
  user_id: string
  created_at: timestamp
  status: completed | cancelled | failed
  agent_name: string
  latency_ms: number
  answer_state: grounded | partially_supported | unsupported | review_gated | provider_fallback | guard_rejected | local_mode | unknown
  source_count: number
  summary: string
  top_sources:
    - document_id
      title
      review_status
      source_type
      snippet
  diagnostics:
    no_source_reason
    review_gated_count
  gap:
    gap_id
    status
    reason
```

Rules:

- timeline items are projections, not a new source of truth;
- snippets remain bounded;
- gated source content remains hidden;
- unknown or missing metadata renders as `unknown` instead of breaking the page;
- timeline endpoint returns a bounded result set.

## UX Requirements

### `/admin`

- Keep the page operational and scan-friendly.
- Place the timeline near the current Audit area or replace the table with a
  timeline when Stage 2 approves.
- Provide compact filters for answer state, weak-only, document ID, and recent
  limit.
- Each item should show:
  - time;
  - answer state;
  - conversation ID;
  - source count;
  - top source chips/buttons;
  - gap status or next action;
  - agent/status metadata in a secondary line.
- Long snippets and titles must clamp or wrap cleanly.

### Operator Actions

- Open source document detail from a cited source.
- Jump to knowledge gaps when a weak answer has a matching gap.
- Refresh the timeline.
- Narrow to weak answers without losing the ability to inspect all answers.

## Acceptance Criteria

1. The system exposes a deterministic answer-audit timeline projection.
2. Timeline items are ordered newest-first with stable tie-breaks.
3. Timeline filters support answer state, weak-only, document ID, conversation
   ID, and bounded limit.
4. Timeline items include answer state, evidence summary, source count, top
   source links, diagnostics, and gap relationship summary where available.
5. Review-gated source text does not leak through timeline output.
6. `/admin` renders a bounded, scan-friendly timeline surface.
7. Existing audit table or audit refresh behavior does not regress unless Stage
   2 explicitly replaces it.
8. Timeline rendering handles missing/older audit records gracefully.
9. Tests do not require DeepSeek, network, SQLite, external vector services, or
   browser automation.
10. Documentation explains that this is an operations timeline, not a compliance
    archive.

## Test Matrix Seed

| Area | Scenario | Expected result |
| --- | --- | --- |
| Projection | Grounded audit with two citations | Timeline item shows grounded state, source count, and top sources |
| Ordering | Two records have different timestamps | Newest record appears first |
| Ordering | Two records share timestamp | Stable ID tie-break keeps deterministic order |
| Filtering | Filter by answer state | Only matching states are returned |
| Filtering | Weak-only filter | Unsupported, partial, review-gated, fallback, and guard states are returned |
| Filtering | Filter by document ID | Only items citing that document are returned |
| Safety | Review-gated evidence exists | Timeline shows reason/count without raw gated content |
| Compatibility | Older audit record has no evidence | Timeline renders unknown/empty evidence safely |
| Gap link | Unsupported answer has matching gap | Timeline item includes gap status/link |
| Admin UI | Long titles/snippets | Timeline remains bounded and scannable |

## Open Questions For Stage 2

1. Should Phase 21 add a new `/admin/audit/timeline` endpoint or extend the
   existing audit endpoint with query parameters?
2. Should gap matching rely only on explicit IDs, or allow conservative
   space/question/reason matching for older records?
3. Should the current Audit table remain beside the timeline, or should the
   timeline replace it?
4. Should timeline items include a redacted user question summary if available,
   or avoid question text entirely in this phase?
5. What is the default retention/limit for local file-backed timeline queries?

## Stage 1 Recommendation

Approve Phase 21 as the Answer Audit Timeline.

After approval, Stage 2 should run `$gstack-autoplan` and lock:

- timeline data contract;
- endpoint shape;
- filter semantics;
- gap matching rules;
- admin UI placement;
- TDD slice order;
- exact regression test matrix.
