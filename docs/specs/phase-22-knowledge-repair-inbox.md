# Phase 22 Knowledge Repair Inbox Spec

Date: 2026-07-09

Status: Draft; awaiting user approval

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 14 Knowledge Operations Console Spec](./phase-14-knowledge-operations-console.md)
- [Phase 16 Knowledge Workbench and Gap Resolution Spec](./phase-16-knowledge-workbench-gap-resolution.md)
- [Phase 17 Knowledge Curation and Source Management Spec](./phase-17-knowledge-curation-source-management.md)
- [Phase 19 Knowledge Review and Activation Spec](./phase-19-knowledge-review-activation.md)
- [Phase 20 Knowledge Evidence and Answer Trust Spec](./phase-20-knowledge-evidence-answer-trust.md)
- [Phase 21 Answer Audit Timeline Spec](./phase-21-answer-audit-timeline.md)
- Current implementation: local knowledge spaces, file-backed knowledge gaps,
  workbench notes, review activation, answer evidence panels, document detail
  relationships, and a bounded answer audit timeline with weak-answer cues.

## Context

Phase 21 made answer trust visible over time. The operator can now inspect
recent answers, filter weak states, open cited sources, and jump toward the
knowledge-gap workflow.

That exposes the next product question:

> Can an operator turn weak answers into a repeatable repair workflow?

Today the system has useful pieces:

- unsupported, partially supported, and review-gated answers can create local
  knowledge gaps;
- gaps have lifecycle states such as open, investigating, ignored, and resolved;
- `/admin` can run diagnostics from a gap question;
- `/admin` can create workbench notes linked to a source gap;
- document detail can show which gap created or resolved a document;
- the answer timeline can reveal repeated weak-answer patterns.

But the repair experience is still scattered. The operator sees a gap queue,
manual diagnostics, note creation, document review, and timeline context in
separate parts of the page. There is no prioritized repair inbox that answers:

- Which weak answer should I fix first?
- Why is this repair item here?
- What exact evidence or document action would close it?
- Has the fix been retested against the original question?
- Which repair items are duplicates, stale, ignored, or already resolved?

## Office-Hours Premise Challenge

The tempting version of Phase 22 is a large autonomous knowledge maintenance
system:

- automatic web crawling for missing sources;
- LLM-generated answers and documents;
- semantic clustering across all weak answers;
- reviewer assignment and SLA tracking;
- retest scheduling and notifications;
- analytics dashboards for support quality;
- multi-user workflow, RBAC, and approval queues.

That is too much for this local-first repo. It would also blur a crucial trust
line: the system should help an operator repair knowledge, not silently invent
evidence.

The opposite temptation is to keep the current gap queue and call it done.
That leaves too much coordination in the operator's head. Once the timeline can
show patterns, the product needs a place where those patterns become work.

The narrow move is a **Knowledge Repair Inbox**:

1. project gaps, recent weak timeline items, diagnostics, and linked documents
   into repair items;
2. sort and filter repair items by actionability;
3. keep every item local, explainable, and tied to a concrete original question;
4. let the operator investigate, create or link evidence, retest, and resolve;
5. avoid automatic content creation, external crawlers, and confidence theater.

## Product Thesis

A professional digital human should not only expose weak answers. It should
make the next repair action obvious.

The operator should be able to answer:

- What should I repair next?
- Is the problem missing evidence, pending review, low support, or a stale gap?
- Which original answer or timeline item created this repair need?
- Which source document could fix it?
- Has the fix actually changed answer support for the original question?
- Which items can safely be ignored because they are duplicates or irrelevant?

## Goal

Deliver a local-first Knowledge Repair Inbox:

1. define a stable repair-item projection over knowledge gaps, answer timeline
   data, diagnostics, and document relationships;
2. expose a bounded `/admin` inbox view with filters and next-action controls;
3. make each repair item traceable to the original question and answer state;
4. support operator-driven investigate, link/create evidence, retest, resolve,
   ignore, and reopen flows;
5. preserve review-gate safety and avoid automatic unsupported content;
6. add deterministic tests that do not require DeepSeek, network, SQLite,
   external vector services, or browser automation.

## Recommended Narrow Wedge

Ship Phase 22 as an operator repair queue, not as an autonomous repair agent.

The first slice should add:

- a `KnowledgeRepairItem` projection that joins gaps with recent weak answer
  timeline context where available;
- filters for repair status, reason, answer state, document linkage, and space;
- a deterministic priority model based on explicit local signals:
  - open before investigating before ignored/resolved;
  - repeated matching weak answers;
  - review-gated count;
  - no linked evidence document;
  - most recent weak answer time;
- `/admin` UI that renders a compact repair inbox near the existing knowledge
  gap tools;
- item actions that reuse existing primitives:
  - investigate;
  - run diagnostics;
  - create workbench note;
  - link or inspect a candidate document;
  - retest the original question;
  - resolve, ignore, or reopen;
- no background worker, no generated source text, no external retrieval, and no
  multi-user assignment.

## In Scope

### Repair Item Contract

- Add a typed projection for repair inbox items.
- Reuse existing knowledge gaps, audit timeline items, diagnostics, and document
  relationships rather than creating a second workflow store.
- Include fields needed for `/admin` rendering:
  - repair item ID;
  - gap ID;
  - space ID and name where available;
  - original question summary;
  - gap status;
  - answer state;
  - no-source or weak-support reason;
  - source count;
  - last seen timestamp;
  - occurrence count;
  - candidate evidence document IDs;
  - linked source/resolution document IDs;
  - recommended next action.

### Inbox Filters

- Filter by knowledge space.
- Filter by repair status: open, investigating, resolved, ignored.
- Filter by reason: no matching chunks, review-gated documents,
  no active reviewed documents, partial support, provider/guard fallback where
  represented by timeline data.
- Filter weak-only and unresolved-only.
- Filter by linked/unlinked evidence.
- Limit results deterministically.

### Admin Repair UX

- Add an `/admin` Knowledge Repair Inbox surface close to the existing knowledge
  gap queue and answer timeline.
- Keep the current gap queue behavior stable during the phase.
- Each item should show:
  - priority;
  - question summary;
  - reason;
  - status;
  - occurrence count and last seen time;
  - linked/candidate evidence;
  - next action.
- Long questions and snippets must remain bounded.

### Operator Actions

- Investigate an open item.
- Run diagnostics for the original question.
- Create a workbench note from the repair item.
- Inspect or link a candidate document.
- Retest the original question against the selected space.
- Resolve an item with evidence document and note.
- Ignore or reopen an item.

### Retest Loop

- Provide a deterministic local retest path that reuses existing retrieval
  diagnostics and answer-state metadata where possible.
- Show before/after support state for the original question.
- Keep retest results local and bounded.
- Do not require provider calls in CI.

### Safety And Privacy

- Do not store full prompt transcripts beyond existing bounded summaries.
- Do not expose raw pending, rejected, or archived source text.
- Do not generate repair notes automatically from the model.
- Do not claim semantic correctness from a successful retrieval retest.
- Keep all repair item lists tenant- and space-scoped through existing local
  admin boundaries.

## Out Of Scope

- Autonomous document generation.
- Web crawling, remote fetching, or source discovery.
- LLM clustering, semantic duplicate detection, or confidence scoring.
- Multi-user assignment, due dates, notifications, or SLAs.
- Cross-tenant analytics.
- External database, queue, scheduler, or background worker.
- Compliance export or immutable repair ledger.
- Replacing the existing gap queue in this phase.

## Alternatives Considered

### A. Keep The Existing Gap Queue

This is the lowest implementation cost and preserves the current admin page.

Rejected because Phase 21 now reveals repeated weak answers and source patterns.
A raw gap list does not prioritize or guide repair work well enough.

### B. Autonomous Knowledge Repair Agent

Let the system draft notes, fetch sources, cluster gaps, retest answers, and
resolve items automatically.

Rejected for Phase 22 because it would introduce new trust, provider, and
security risks. The product should first make human repair work repeatable.

### C. Knowledge Repair Inbox

Project existing gaps and weak-answer context into a prioritized operator
inbox, then reuse existing diagnostics, note creation, review, and resolution
flows.

Recommended because it turns Phase 21 visibility into action while staying
local-first and deterministic.

## Proposed Architecture

```text
knowledge gaps
  -> repair inbox projection
       -> status/reason/space filters
       -> priority and next action

answer audit timeline
  -> weak answer context
       -> last seen time
       -> occurrence count
       -> answer state and source count

knowledge documents + relations
  -> linked evidence
       -> source_gap_id
       -> resolved_by_document_id
       -> candidate diagnostics

/admin
  -> repair inbox view
  -> existing gap queue
  -> diagnostics
  -> workbench note creation
  -> document detail
  -> retest and resolve
```

## Repair Item Draft

Stage 2 should lock exact names. The minimum contract is:

```text
knowledge_repair_item:
  repair_id: string
  gap_id: string
  space_id: string
  space_name: string
  question_summary: string
  status: open | investigating | resolved | ignored
  answer_state: unsupported | partially_supported | review_gated | provider_fallback | guard_rejected | unknown
  reason: string
  priority: high | medium | low
  priority_reasons:
    - repeated_weak_answer
    - no_linked_evidence
    - review_gated
    - recent
  last_seen_at: timestamp
  occurrence_count: number
  source_count: number
  linked_documents:
    - document_id
      role: source_note | resolution | candidate
      title
      review_status
  next_action:
    kind: investigate | run_diagnostics | create_note | review_source | retest | resolve | none
    label: string
```

Rules:

- repair items are projections, not a new source of truth;
- priority is deterministic and explainable;
- candidate documents come from diagnostics or explicit relationships, not
  semantic guessing;
- missing timeline context must not break old gap records;
- resolved and ignored items remain inspectable but do not dominate the default
  view.

## UX Requirements

### `/admin`

- Keep the admin page operational and dense.
- Place the repair inbox near the current knowledge gaps and answer timeline.
- Default view should show unresolved repair work first.
- Filters should support:
  - space;
  - status;
  - reason;
  - linked/unlinked evidence;
  - weak answer type;
  - recent limit.
- Actions should be visible but not noisy; the primary next action should be
  first.
- Existing gap queue, document detail, review queue, and audit timeline should
  keep working.

### Operator Workflow

1. Open `/admin`.
2. Inspect the Knowledge Repair Inbox.
3. Pick a high-priority open item.
4. Run diagnostics from the original question.
5. Create or inspect evidence.
6. Review/activate candidate evidence if needed.
7. Retest the original question.
8. Resolve or ignore the repair item.
9. Confirm the answer timeline no longer shows the same repeated weak pattern.

## Acceptance Criteria

1. The system exposes a deterministic repair inbox projection.
2. Repair items are scoped by tenant and knowledge space.
3. Repair items join gap status with available weak-answer timeline context.
4. Default ordering prioritizes actionable unresolved items with explainable
   priority reasons.
5. Filters support status, reason, weak-only/unresolved-only, document linkage,
   space, and bounded limit.
6. `/admin` renders a bounded repair inbox without regressing the existing gap
   queue or audit timeline.
7. Item actions reuse existing investigate, diagnostics, note creation, document
   inspection, resolve, ignore, and reopen flows.
8. Retest results show support-state change without requiring provider calls in
   CI.
9. Review-gated source text does not leak through repair item output.
10. Tests do not require DeepSeek, network, SQLite, external vector services, or
    browser automation.

## Test Matrix Seed

| Area | Scenario | Expected result |
| --- | --- | --- |
| Projection | Open gap with no timeline context | Repair item renders with unknown answer state and open status |
| Projection | Unsupported gap has repeated weak timeline items | Repair item occurrence count and last seen are populated |
| Priority | Open repeated weak answer with no evidence | Item priority is high with deterministic reasons |
| Priority | Resolved gap with evidence document | Item drops below unresolved work |
| Filtering | Filter by status investigating | Only investigating repair items return |
| Filtering | Unresolved-only | Open and investigating items return; resolved/ignored do not |
| Filtering | Linked evidence only | Only items with source or resolution documents return |
| Safety | Review-gated item | Reason/count show without raw gated source text |
| Retest | Diagnostics find active reviewed source | Retest shows improved support state locally |
| Retest | Diagnostics still find no source | Item remains unresolved with no-source reason |
| Admin UI | Long question summary | Inbox remains bounded and scannable |
| Compatibility | Existing gap queue | Existing `/admin/knowledge/gaps` behavior remains unchanged |

## Open Questions For Stage 2

1. Should the repair inbox be backed by a new endpoint such as
   `/admin/knowledge/repairs`, or extend the existing gaps endpoint with a
   projection mode?
2. Should retest persist a small repair attempt history, or stay ephemeral in
   Phase 22?
3. Should priority be represented as high/medium/low, numeric score, or ordered
   reason list only?
4. Should provider fallback and guard-rejected timeline items create repair
   items, or remain timeline-only signals unless a gap exists?
5. Should candidate documents come only from diagnostics run on demand, or also
   from recent citations in related timeline items?

## Stage 1 Recommendation

Approve Phase 22 as the Knowledge Repair Inbox.

After approval, Stage 2 should run `$gstack-autoplan` and lock:

- repair item data contract;
- endpoint shape;
- priority semantics;
- retest persistence boundary;
- admin UI placement;
- TDD slice order;
- exact regression test matrix.

