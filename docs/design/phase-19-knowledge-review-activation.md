# Phase 19 Knowledge Review and Activation Design

Date: 2026-07-04

Status: Draft; waiting for spec approval

Source spec: [Phase 19 Knowledge Review and Activation Spec](../specs/phase-19-knowledge-review-activation.md)

## Office-Hours Summary

Phase 18 proved source ingestion. Phase 19 should make activation explicit.

The design should not turn the product into a workflow suite. It should add a
small trust boundary between imported source material and answer generation:

```text
imported source -> pending review -> operator activation -> retrieval
```

The system already has enough building blocks:

1. knowledge spaces;
2. document lifecycle status;
3. import jobs and source metadata;
4. source warnings;
5. admin document detail;
6. retrieval diagnostics;
7. grounded answer state.

Phase 19 should connect those pieces with a review state and bounded admin UI.

## Executive Decision

Use a **Lightweight Trust Gate**.

Imported documents should be mechanically stored and indexed, but they should
not automatically become trusted answer sources. Existing documents stay active
for backward compatibility. The operator gains a clear review queue and
approve/reject/archive actions.

## Product Shape

The user-facing shape is:

- a Review Queue inside `/admin`;
- document badges for review state;
- warning indicators for imported content that looks instruction-like;
- approve/reject/archive/reactivate actions in document detail;
- retrieval diagnostics that can say knowledge was skipped by review state.

This keeps the knowledge experience coherent. The operator should not need to
learn a second product surface to decide whether a source is trusted.

## Premises

### Premise 1: Lifecycle status is not trust status

`KnowledgeReady` means the document is usable by the system. It should not also
mean the operator has reviewed the source. Phase 19 separates these meanings.

### Premise 2: Existing knowledge must not disappear

Older records do not have review metadata. Treat missing review state as
`active`; otherwise a migration would silently break retrieval.

### Premise 3: Warning is not rejection

`source_warning` should raise attention, not decide. A deterministic warning can
put text in the review queue, but only the operator should approve or reject in
this phase.

### Premise 4: Retrieval is the real trust boundary

Admin display is useful, but the important product guarantee is that pending,
rejected, and archived documents do not become citations in normal answers.

## Alternatives

### Approach A: Status Reuse

Reuse `KnowledgeDisabled` and `KnowledgeReady` for review state.

Pros:

- smallest data change;
- existing retrieval filter already excludes disabled documents.

Cons:

- overloads lifecycle status;
- cannot distinguish "operator rejected" from "document disabled";
- makes future diagnostics and audit harder.

Verdict: not recommended.

### Approach B: Separate Review Entity

Create a separate review record with state transitions and audit events.

Pros:

- clean history model;
- future multi-reviewer workflow is easier;
- no metadata overload.

Cons:

- more storage and API surface;
- larger local persistence change;
- premature for a solo-operator phase.

Verdict: defer until review history becomes a real requirement.

### Approach C: Document Review State

Attach a review state and small review metadata to each knowledge document.

Pros:

- simple retrieval filter;
- easy admin list/detail display;
- works with local file persistence;
- enough provenance for the first activation workflow.

Cons:

- less complete than an event-sourced audit trail;
- Stage 2 must carefully preserve backward compatibility.

Verdict: recommended.

## Recommended Architecture

```text
internal/admin
  KnowledgeReviewStatus
  KnowledgeReviewUpdate
  transition validation
  list/filter by review state
  approve/reject/archive/reactivate methods

internal/admin/knowledge_import.go
  create imported documents with pending_review
  include review state in import results

internal/knowledge
  retrieval filter requires lifecycle ready and review active
  diagnostics include review-gated reason when applicable

internal/server
  admin endpoints for review state updates
  admin endpoints for review-state-aware listing

web/admin.html and web/admin.js
  bounded review queue
  document badges
  warning display
  review actions in detail panel

local store
  persist review metadata
  derive active for missing review status
```

## State Machine

```text
pending_review -> active
pending_review -> rejected
pending_review -> archived

active -> archived
active -> rejected

rejected -> pending_review
rejected -> archived

archived -> pending_review
archived -> active
```

Validation rules:

- unknown review status is rejected at write time;
- empty reason is allowed for approve/archive/reactivate;
- reject should accept an optional reason and Stage 2 should decide whether it
  is required;
- lifecycle status still wins over review state for retrieval exclusion;
- failed or indexing documents can have review state but cannot be retrieved.

## Storage Contract

Stage 2 should choose between first-class fields and metadata keys.

Recommended first implementation:

- use stable fields if the local store format can be updated with low risk;
- otherwise use stable metadata keys and expose typed accessors from
  `internal/admin`.

Stable behavior:

```text
missing review_status -> active
import-created review_status -> pending_review
review_status active + lifecycle ready -> retrievable
review_status not active -> not retrievable
lifecycle not ready -> not retrievable
```

Suggested metadata keys if metadata-backed:

```text
review_status
review_reason
reviewed_by
reviewed_at
activated_at
```

## API Shape Draft

Stage 2 should lock exact endpoint names. A minimal shape is:

```text
GET  /admin/knowledge?review_status=pending_review
POST /admin/knowledge/review
```

Example review request:

```json
{
  "document_id": "doc-123",
  "review_status": "active",
  "reason": "Reviewed source and approved for retrieval."
}
```

The response should return the updated document summary so the UI can refresh
without guessing.

## Admin UX Design

### Review Queue

Add a compact queue to the existing knowledge admin area:

- pending count;
- warning count;
- rows with name, source type, space, warning badge, imported time, and status;
- click row to open existing document detail;
- action buttons stay in detail, not every row, to reduce accidental changes.

### Document Detail

Add a review section near source metadata:

- review status badge;
- source warning badge if present;
- review reason;
- reviewed timestamp;
- approve/reject/archive/reactivate controls;
- bounded chunk preview remains unchanged.

### Empty States

- no pending review: show a quiet "No pending review" state;
- only rejected/archived docs: keep them filterable but not visually dominant;
- import creates pending docs: show a direct link to review queue.

## Retrieval Design

The retrieval pipeline currently filters ready documents. Phase 19 should add a
review-active predicate before ranking.

Suggested order:

1. trim empty query;
2. derive ready documents;
3. derive review-active documents;
4. filter by space;
5. run lexical/vector ranking.

Diagnostics should be concise:

- `no_review_active_documents` when ready documents exist but none are active;
- `review_gated_documents` in skipped stages or explanation metadata if useful;
- avoid returning raw pending/rejected content.

Normal citations must never include pending, rejected, or archived documents.

## Security and Abuse Notes

- Do not use an LLM to auto-approve imported knowledge.
- Do not treat deterministic source warnings as proof of malicious content.
- Do not leak raw rejected/pending text through diagnostics.
- Keep review actions admin-only under existing admin route boundaries.
- Preserve source provenance so a rejected source can be audited.
- Ensure local file persistence cannot be corrupted by unknown review states.

## Test Strategy

Stage 3 should implement these with Superpowers TDD:

1. RED: imported document defaults to pending review.
2. GREEN: import service sets review metadata/state.
3. RED: missing review state derives active for old documents.
4. GREEN: typed review-state helper handles defaults.
5. RED: retrieval excludes pending/rejected/archived documents.
6. GREEN: pipeline filters by active review state.
7. RED: admin review action updates state and reason.
8. GREEN: service validates transitions and persists.
9. RED: `/admin` API returns deterministic errors for invalid review state.
10. GREEN: server handler and tests.
11. RED: admin UI renders bounded review queue and actions.
12. GREEN: web static tests and UI logic.

No tests should require DeepSeek, network, SQLite, or browser automation unless
Stage 5 QA explicitly runs a local server.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Imported knowledge seems to disappear | User confusion | Import results link directly to review queue |
| Review state overloads lifecycle status | Harder diagnostics | Keep status and review state separate |
| Backward compatibility breaks retrieval | Severe regression | Missing review state derives active |
| Admin UI gets noisy | Lower operator trust | Use compact queue and detail actions |
| Rejected content leaks in diagnostics | Trust/security issue | Reason codes only; no raw rejected text |

## Stage 2 Decisions

Autoplan should lock:

1. first-class field vs metadata-backed storage;
2. default review state for manual non-import uploads;
3. exact review-state transition validation;
4. API endpoint names and payloads;
5. UI placement and copy;
6. retrieval diagnostic reason codes;
7. TDD slice order and test matrix.

## Office-Hours Handoff

Recommended approval:

> Approve Phase 19 as a Lightweight Trust Gate that adds explicit knowledge
> review state, a bounded admin review queue, and retrieval filtering so only
> active reviewed knowledge can ground answers.

After approval, run Stage 2 with `$gstack-autoplan` before any implementation.
