# Phase 19 Knowledge Review and Activation Spec

Date: 2026-07-04

Status: Draft; waiting for user approval

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 12 Knowledge Space Management and Grounded Answering Spec](./phase-12-knowledge-space-management-grounded-answering.md)
- [Phase 14 Knowledge Operations Console Spec](./phase-14-knowledge-operations-console.md)
- [Phase 15 Knowledge-Grounded Answer Loop Spec](./phase-15-knowledge-grounded-answer-loop.md)
- [Phase 16 Knowledge Workbench and Gap Resolution Spec](./phase-16-knowledge-workbench-gap-resolution.md)
- [Phase 17 Knowledge Curation and Source Management Spec](./phase-17-knowledge-curation-source-management.md)
- [Phase 18 Knowledge Source Ingestion Spec](./phase-18-knowledge-source-ingestion.md)
- Current implementation: local knowledge spaces, document lifecycle, retrieval
  diagnostics, grounded answer state, gap-to-note workflow, document curation,
  import jobs, source metadata, content hash dedupe, and prompt-injection-like
  import warnings.

## Context

Phase 18 made the knowledge system materially more useful: operators can now
bring real source material into the local knowledge base through import jobs.
That also changes the risk profile. Imported material can now move from
"external text" to "retrieval source" quickly.

The next product question is:

> Can the operator decide which imported knowledge is trusted enough to shape
> the digital human's answers?

Today, the retrieval path correctly filters on `KnowledgeReady`, but readiness
mostly means the document was accepted and indexed. It does not capture whether
the operator reviewed imported content, accepted warnings, or rejected a source
as untrusted.

Phase 19 should add a small activation workflow between ingestion and normal
retrieval. The goal is not enterprise approvals. The goal is a visible,
local-first trust boundary.

## Office-Hours Premise Challenge

The tempting version of this feature is a full approval platform:

- reviewer assignments;
- multi-step approvals;
- RBAC;
- comments and review threads;
- trust scoring;
- LLM-generated summaries and verdicts;
- automatic quarantine rules for every source type.

That is too much for the current product. The system has one primary operator,
local storage, and deterministic CI. A large workflow would add friction before
the knowledge import loop has been proven with real use.

The opposite temptation is to do nothing because imported documents already have
status values. That also misses the product need. `ready` means mechanically
usable; it does not mean professionally trusted.

The narrow move is a **Lightweight Trust Gate**:

1. imported documents receive an explicit review state;
2. only active reviewed documents participate in normal retrieval;
3. the admin console shows pending, warning, rejected, and archived sources;
4. the operator can approve or reject with a short reason;
5. existing documents remain active by default so older knowledge does not
   disappear.

## Product Thesis

A professional digital human should not treat every imported sentence as trusted
knowledge the moment it parses.

The operator should be able to answer:

- Which imported sources are waiting for review?
- Which sources contain deterministic safety warnings?
- Which sources are active in retrieval?
- Why was a source rejected or archived?
- Did an answer exclude knowledge because it was not activated?
- Can I recover or reactivate a document without losing provenance?

## Goal

Deliver local-first Knowledge Review and Activation:

1. add an explicit review/activation state for knowledge documents;
2. default newly imported documents into a reviewable state;
3. preserve backward compatibility by treating existing documents as active;
4. make review state visible and actionable in `/admin`;
5. exclude unapproved imported content from normal retrieval and grounded
   answers;
6. expose enough diagnostics to explain when review state affects retrieval;
7. keep the implementation deterministic, local-first, and free of external
   provider calls.

## Recommended Narrow Wedge

Ship Phase 19 as a document-level review workflow:

- `pending_review`: imported document exists but is not active in normal
  retrieval yet;
- `active`: document can participate in retrieval and grounded answers;
- `rejected`: document is retained for audit/provenance but excluded from
  retrieval;
- `archived`: document is intentionally hidden from active operations while
  remaining recoverable.

Stage 2 should lock the exact storage shape. The behavior contract matters most:
normal retrieval must only use active documents, while admin surfaces can show
all review states.

## In Scope

### Review State Model

- Add a review state concept separate from mechanical indexing status.
- Keep `KnowledgeReady`, `KnowledgeDisabled`, `KnowledgeIndexing`, and
  `KnowledgeFailed` focused on document lifecycle/index health.
- Ensure missing review state on old documents derives as `active`.
- Record review timestamp, optional reviewer identity, and optional reason.

### Import Integration

- Phase 18 imported documents should be created as `pending_review` unless
  Stage 2 deliberately chooses a compatibility exception.
- Import result rows should surface the created document's review state.
- Documents with `source_warning` should be visually obvious in the review
  queue.

### Admin Review Queue

- `/admin` should show pending imported knowledge separately from already
  active documents.
- The operator can approve, reject, archive, and reactivate documents.
- The document detail view should show source provenance, warnings, review
  state, review reason, and review timestamps.
- Existing filters should gain review-state awareness where useful.

### Retrieval and Diagnostics

- Normal retrieval should include only documents that are both mechanically
  ready and review-active.
- Pending, rejected, archived, disabled, indexing, and failed documents should
  not affect normal answers.
- Diagnostics should make review-state exclusion explainable without leaking
  large document content.
- Existing knowledge-grounded answer behavior should remain deterministic.

### Audit and Provenance

- Review actions should preserve enough information to answer who or what
  changed the activation state and why.
- The first slice can store this directly on the document metadata if Stage 2
  decides that is the smallest safe change.
- A future audit-event table is allowed but not required for Phase 19.

## Out of Scope

- Multi-user RBAC, reviewer assignment, or approval chains.
- LLM-written review summaries, LLM approval decisions, or automatic trust
  classification.
- PDF, DOCX, crawler, GitHub, cloud-drive, or scheduled sync adapters.
- External database migrations.
- Background workers unless Stage 2 proves they are already needed.
- Rewriting imported source text during review.
- Organization-level compliance workflows.

## Alternatives Considered

### A. Keep Imports Active Immediately

This is the lowest implementation cost. It keeps Phase 18 behavior unchanged.

Rejected because it weakens the trust story exactly when ingestion starts
bringing in more real-world content. It also makes `source_warning` informative
but not operational.

### B. Full Enterprise Approval Workflow

This would add assignments, reviewer roles, approval threads, event history, and
policy configuration.

Rejected for now because it overfits a future team workflow before the solo
operator loop is proven. It also increases UI and storage complexity without
improving the first activation decision.

### C. Lightweight Trust Gate

This adds one review-state layer and a small admin queue. It separates "parsed
and indexed" from "trusted for answers" while preserving local-first behavior.

Recommended because it directly supports professional digital-human operation
without turning the knowledge system into a workflow suite.

## Proposed Architecture

```text
Phase 18 import job
  -> creates knowledge document
  -> sets review state to pending_review
  -> records source metadata and warnings

/admin knowledge review queue
  -> lists pending/warned/rejected/archived documents
  -> approve/reject/archive/reactivate actions
  -> writes review metadata

knowledge retrieval pipeline
  -> filters lifecycle status == ready
  -> filters review state == active
  -> returns diagnostics for review-state exclusion

/app grounded answer loop
  -> receives only active retrieved citations
  -> can show no-source or diagnostic reason when review gate excluded sources
```

## Data Model Draft

Stage 2 should choose one of two storage approaches:

1. first-class fields on `KnowledgeDocument`;
2. metadata-backed fields using stable metadata keys.

The minimum contract is:

```text
review_status: pending_review | active | rejected | archived
review_reason: optional short operator text
reviewed_by: optional operator/system identifier
reviewed_at: optional timestamp
activated_at: optional timestamp
```

Backward compatibility rule:

- if `review_status` is missing on an existing document, derive `active`;
- if a new imported document is created by Phase 18 ingestion, set
  `pending_review`;
- if a document is disabled or failed, review state must not override lifecycle
  exclusion.

## UX Requirements

### Admin Console

- Show a compact "Review queue" for pending imported knowledge.
- Show badges for `pending_review`, `active`, `rejected`, and `archived`.
- Highlight deterministic source warnings such as instruction-like text.
- Add approve/reject/archive/reactivate actions in document detail.
- Preserve existing document list, source filters, gap links, and chunk preview.
- Avoid making the left or right panels grow unbounded; use summary rows and
  bounded detail areas.

### App Experience

- Normal users should not see pending/rejected knowledge as citations.
- If an answer cannot use available-looking knowledge because it is pending
  review, diagnostics may surface a concise reason for operators.
- The assistant should never claim a rejected source as evidence.

## Acceptance Criteria

1. Existing knowledge documents without review metadata remain active.
2. Newly imported documents can be represented as pending review.
3. The admin UI can list pending review documents.
4. The operator can approve a pending document and make it eligible for normal
   retrieval.
5. The operator can reject a document with an optional reason and keep it out of
   retrieval.
6. The operator can archive and reactivate without losing provenance.
7. Documents with `source_warning` are visible in the review experience.
8. Normal retrieval excludes non-active review states.
9. Retrieval diagnostics explain review-gated exclusions at a reason-code level.
10. Existing disabled, failed, and indexing lifecycle behavior still wins over
    review state.
11. Local file persistence remains backward compatible with old documents.
12. Tests use local fakes and do not require DeepSeek, network, SQLite, or cloud
    services.

## Test Matrix Seed

| Area | Scenario | Expected result |
| --- | --- | --- |
| Backward compatibility | Load document without review metadata | Derived review state is active |
| Import integration | Import local text document | Document is pending review and retains import metadata |
| Warning visibility | Import instruction-like text | Pending item shows warning metadata |
| Review action | Approve pending document | Document becomes active and is retrievable |
| Review action | Reject pending document | Document is retained but not retrievable |
| Review action | Archive active document | Document is hidden from normal retrieval |
| Review action | Reactivate archived document | Document becomes retrievable again if lifecycle status is ready |
| Retrieval | Search with only pending documents | No normal citations; diagnostic reason mentions review gate |
| Lifecycle precedence | Active review state but disabled lifecycle status | Document remains excluded |
| Admin API | Invalid review state transition | Request fails with deterministic error |
| Persistence | Restart with local store | Review state and reason survive |
| UI | Review queue with many documents | Layout remains bounded and scannable |

## Open Questions For Stage 2

1. Should Phase 19 store review state as first-class fields or stable metadata
   keys for the first implementation?
2. Should manual non-import uploads default to `active` or `pending_review`?
3. Should rejected documents remain visible in the main list by default or only
   through a review-state filter?
4. Should diagnostics expose counts of review-gated documents, or only a reason
   code?
5. Should the review queue live inside the existing knowledge panel or as a
   dedicated admin tab?

## Stage 1 Recommendation

Approve Phase 19 as the Lightweight Trust Gate.

After approval, Stage 2 should run `$gstack-autoplan` and lock:

- storage shape;
- API endpoints and transition validation;
- UI placement;
- retrieval diagnostic contract;
- exact TDD slices for Superpowers RED/GREEN/REFACTOR implementation.
