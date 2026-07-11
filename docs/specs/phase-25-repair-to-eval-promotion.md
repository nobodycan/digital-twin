# Phase 25 Repair-to-Eval Promotion Spec

Date: 2026-07-11

Status: Approved; Stage 1 gate passed

Source roadmap: [Phase 23-26 Repair Verification Roadmap](../design/phase-23-26-repair-verification-roadmap.md)

Prior phase: [Phase 24 Recurrence Watch Spec](./phase-24-recurrence-watch.md)

## Problem Statement

Phase 23 proves that a repaired gap is supported by the current reviewed
knowledge snapshot. Phase 24 notices later weak answers and returns confirmed
recurrences to the canonical gap workflow. Neither phase turns a successful
repair into a repeatable regression check.

Today an operator can verify a repair, but a later knowledge edit can break the
same question without failing the existing eval suite. Static files in
`evals/conversations` cannot safely be written by the running admin service,
and their fixture outputs do not execute the current retrieval state. Phase 25
needs a durable bridge from repair evidence to the existing eval runner and
release gate without creating a second eval system.

## Target User And Wedge

The target user is the local knowledge operator who has resolved and verified
a gap and wants that repair protected against future retrieval and knowledge
regressions.

The narrow wedge is one `Promote to eval` action in the existing Repair Inbox.
It creates a tenant-scoped required RAG case with a stable question, space, and
minimum support state. The case runs provider-free against current reviewed
knowledge and can block a knowledge release through the existing release gate.

## Goals

1. Promote only a resolved repair whose current verification projection is
   `verified`.
2. Block promotion while the repair has a pending `suspected` recurrence.
3. Persist durable provenance to the canonical gap and the exact verification
   attempt used for promotion.
4. Make repeated promotion of the same current repair idempotent.
5. Project promoted records into the existing `evals.Case`, runner, reports,
   and governance release gate.
6. Execute promoted cases against current active-reviewed knowledge without an
   LLM or provider dependency.
7. Fail a promoted case when support falls below its configured floor, including
   `unsupported` and `review_gated` outcomes.
8. Avoid requiring one exact source document by default so equivalent evidence
   can continue to pass.
9. Keep static eval fixtures and all existing CLI behavior backward compatible.

## Non-Goals

- Automatically promoting every verified repair.
- Semantic question generation, paraphrase generation, LLM judging, or answer
  text scoring.
- Requiring the original document, chunk, ranking, or evidence fingerprint by
  default.
- Editing source-controlled files under `evals/conversations` from the admin
  server.
- Adding trend dashboards, promotion pass-rate analytics, or time-window
  aggregation; those belong to Phase 26.
- Changing retrieval scoring, verification eligibility, gap lifecycle, or
  recurrence matching.
- Promoting open, investigating, ignored, unverified, stale, or recurrence-
  suspected repairs.
- Creating a second release gate or a repair-specific eval runner.

## Existing Building Blocks

- `KnowledgeGapService` owns the canonical repair question, space, lifecycle,
  and resolution evidence.
- `RepairVerificationService` exposes the current `verified`, `stale`, or
  `unverified` projection and the verified attempt ID/snapshot fingerprint.
- `RepairRecurrenceService` exposes an independent recurrence projection with
  `suspected`, `dismissed`, and `confirmed` history.
- `evals.Case`, `Runner`, and `RAGEvaluator` load deterministic cases and return
  suite/check results.
- `go run ./cmd/cli eval` loads static JSON fixtures and writes JSON/Markdown
  reports.
- `governance.ReleaseGate` blocks a release for a failed suite or a skipped
  required check.

## Product Decisions

### Human-Initiated Promotion

Promotion is an explicit operator action. Verification proves current support;
it does not decide that every repair deserves permanent release-gate coverage.
No background job or automatic threshold promotes repairs in Phase 25.

### Eligibility Is Evaluated At Write Time

A promotion request succeeds only when all conditions are true in the same
tenant:

1. The gap exists and is `resolved`.
2. The gap has a current `verified` projection, not `stale` or `unverified`.
3. The request names the same current verified attempt returned by the
   projection, preventing a stale browser action from promoting old evidence.
4. No recurrence record for the gap is currently `suspected`.
5. The question and space are valid, bounded values from the canonical gap.

Dismissed recurrence history does not block promotion. A confirmed recurrence
reopens the gap and is therefore already ineligible through the resolved-gap
rule.

### Stable Case Identity

Each tenant and canonical gap has at most one active promoted eval identity:

`repair-eval-<tenant-id>-<gap-id>`

The persisted uniqueness key is `(tenant_id, gap_id)`. Repeating promotion with
the same current verified attempt and policy returns the existing record without
creating a second case or revision.

If a later knowledge change makes the verification stale, the case remains
active and should fail until the repair is restored. After a new successful
verification, an operator may re-promote the same gap. Re-promotion preserves
the stable case ID, appends a new promotion revision, and advances the active
provenance to the new verification attempt. Historical revisions are not
deleted or rewritten.

### Support-State Assertion, Not Exact Evidence

The default minimum expected state is `partially_supported`. Therefore:

- `grounded` passes;
- `partially_supported` passes;
- `unsupported` fails;
- `review_gated` fails;
- provider, guard, local-mode, missing-output, or execution-error states fail.

An operator may select the stricter `grounded` minimum during promotion. Exact
document IDs are optional advanced constraints and default to an empty list.
When omitted, a different active-reviewed document or chunk may satisfy the
case. This prevents evidence fingerprint and ranking churn from causing false
regressions.

### One Eval And Release-Gate Path

Promoted records are projected into normal `evals.Case` values with category
`rag`, risk `high`, and `required_checks=["rag"]`. The existing runner evaluates
static and promoted cases together. The existing release gate consumes the
resulting suite and blocks when a promoted required RAG check fails or is
skipped.

No second runner, report format, or gate decision type is introduced.

## Data Contract

Add an append-only `RepairEvalPromotionRevision` ledger plus a current
projection. A representative revision is:

```json
{
  "id": "promotion-gap-123-r1",
  "tenant_id": "default",
  "case_id": "repair-eval-default-gap-123",
  "gap_id": "gap-123",
  "space_id": "default",
  "question": "How do I start the service?",
  "verification_attempt_id": "verification-456",
  "verification_snapshot_fingerprint": "sha256:...",
  "minimum_support_state": "partially_supported",
  "required_document_ids": [],
  "revision": 1,
  "active": true,
  "promoted_by": "operator",
  "promoted_at": "2026-07-11T00:00:00Z"
}
```

Required invariants:

- IDs, tenant, gap, space, actor, and document references pass existing safe-ID
  validation.
- `question` is copied from the canonical bounded gap at promotion time and is
  never accepted from the client.
- `minimum_support_state` is only `partially_supported` or `grounded`.
- `required_document_ids` is deduplicated, sorted, bounded, and empty by
  default.
- Only one revision per `(tenant_id, gap_id)` is active.
- Revision numbers increase monotonically for each gap.
- A revision references one immutable verification attempt and snapshot.
- The ledger stores no answer text, source snippets, prompts, or raw diagnostic
  errors.

The promotion ledger is the provenance source. Gap and verification views join
against it by tenant plus `gap_id` or `verification_attempt_id`; immutable
verification attempts are not mutated to add backlinks.

## Promotion Contract

Input contains only operator choices and concurrency evidence:

```json
{
  "gap_id": "gap-123",
  "verification_attempt_id": "verification-456",
  "minimum_support_state": "partially_supported",
  "required_document_ids": [],
  "promoted_by": "operator"
}
```

The service must:

1. Load the tenant-scoped gap.
2. Recompute current verification and recurrence projections.
3. Reject the request if any eligibility invariant fails.
4. Confirm the supplied attempt is the current verified attempt.
5. Validate optional required documents are active-reviewed members of the same
   tenant and space.
6. Build a canonical policy fingerprint from the support floor and sorted
   document constraints.
7. Return the existing active revision unchanged when attempt and policy match.
8. Otherwise append a revision with the stable case ID and atomically make it
   active for the gap.

Store implementations must support in-memory tests and file-backed local use.
File writes use the repository's atomic temporary-file replacement pattern and
must remain safe under concurrent promotion requests.

## Eval Projection Contract

An active promotion projects into an additive eval case:

```json
{
  "id": "repair-eval-default-gap-123",
  "title": "Verified repair: How do I start the service?",
  "tenant_id": "default",
  "category": "rag",
  "risk_level": "high",
  "required_checks": ["rag"],
  "conversation": [
    {
      "id": "repair-eval-default-gap-123-user",
      "role": "user",
      "content": "How do I start the service?"
    }
  ],
  "expected": {
    "rag": {
      "knowledge_space_id": "default",
      "minimum_support_state": "partially_supported",
      "required_document_ids": []
    }
  },
  "provenance": {
    "kind": "repair_promotion",
    "promotion_revision_id": "promotion-gap-123-r1",
    "gap_id": "gap-123",
    "verification_attempt_id": "verification-456"
  }
}
```

`evals.Case` gains additive `required_checks` and safe provenance fields.
Existing JSON fixtures without those fields retain current behavior. Promoted
cases require only the `rag` check; unrelated persona/tool/memory/safety checks
remain non-required when they are skipped.

`RAGExpectation` gains additive `knowledge_space_id`,
`minimum_support_state`, and `required_document_ids`. Existing
`required_citations` behavior remains supported for static fixtures.

`EvaluationOutput` gains additive `knowledge_answer_state`,
`knowledge_space_id`, and source document IDs. The runner marks a result
required only when its check name appears in the case's `required_checks`.

## Execution Contract

Promoted cases do not trust fixture `output`. Before evaluation, a provider-free
execution adapter runs the current retrieval/grounding diagnostic for the case
question, tenant, and fixed space using active-reviewed knowledge only.

The adapter emits only normalized evaluation evidence:

- resulting support state;
- selected knowledge space;
- safe document/chunk references needed by evaluators;
- no-source reason category;
- execution error category.

It does not generate assistant prose or call an LLM. Static fixture cases keep
using their embedded output. Missing promoted-case execution is a required
skipped check and therefore blocks the release gate rather than silently
passing.

## Release-Gate Contract

The eval command and release evaluation path load:

1. source-controlled static cases from `evals/conversations`;
2. active tenant-scoped promoted cases from the promotion store.

Case IDs must be unique across both sources. A collision is a suite setup error,
not last-write-wins behavior.

The promoted `rag` check is required. Any of these blocks a configured knowledge
release candidate:

- support below the configured minimum;
- review-gated or unsupported retrieval;
- wrong knowledge space;
- a missing explicitly required document;
- promoted-case execution error;
- required check skipped or missing output.

Reports include safe promotion provenance and failed case IDs so the operator
can navigate back to the repair. Existing release decisions and report formats
remain additive and backward compatible.

## API Contract

Additive admin endpoints:

- `POST /admin/knowledge/repairs/promotions`
- `GET /admin/knowledge/repairs/promotions?gap_id=<id>&active=<bool>&limit=<n>`

The POST body follows the Promotion Contract. The server derives tenant from
the admin context and derives question/space from the canonical gap.

The GET endpoint is tenant-scoped, newest-first, defaults to 20, and clamps at
100. It returns safe promotion revisions and current projection fields.

Stable error categories:

- `400` malformed input, unsafe IDs, invalid support floor, or invalid document
  constraints;
- `404` tenant-scoped missing gap/attempt without cross-tenant disclosure;
- `409` gap not resolved, verification stale/unverified, stale attempt, or
  pending recurrence;
- `503` promotion or verification dependencies unavailable;
- `500` persistence/internal failure with no raw diagnostic cause.

## Repair Inbox Contract

Extend repair rows with a separate eval-promotion projection:

- `not_promoted`;
- `promoted` with stable case ID, revision, support floor, and promoted time;
- `promotion_stale` when the active revision references an older verification
  attempt after a later successful re-verification.

Show `Promote to eval` only for currently eligible rows. The action uses the
current verified attempt ID already rendered by the repair projection. The
default support floor is `partially_supported`; `grounded` is the only stricter
choice. Exact document constraints stay collapsed behind an optional control.

The row must explain blocked eligibility without implying promotion succeeded:

- verify first;
- verification is stale;
- resolve the gap first;
- review suspected recurrence first.

All server values render through `textContent`. Promotion history is bounded to
20 records in the inline panel.

## Failure And Recovery Behavior

- Concurrent identical promotion requests converge on one active revision.
- A persistence failure does not alter gap, verification, or recurrence state.
- Eval projection failure does not delete or deactivate promotion provenance.
- An execution failure produces a required failing/skipped check and cannot
  accidentally permit a release.
- A stale promotion remains visible and active as regression coverage; it is
  not auto-updated to new verification evidence.
- Re-promotion after successful verification creates a new revision while
  preserving the stable case ID.
- Deactivation, deletion, waiver, and bulk promotion are deferred; Phase 25
  does not add a bypass around a required regression case.

## Security And Privacy

- Every read, write, projection, and action is tenant-scoped.
- Client-supplied tenant, question, space, case ID, revision, and timestamps are
  ignored or rejected.
- The ledger stores the bounded canonical gap question but no raw conversation,
  assistant answer, source snippet, or hidden reasoning.
- Required document constraints are validated against the same tenant/space and
  active-review state.
- API errors use stable categories and do not reveal another tenant's records.
- UI rendering uses text-only DOM operations.
- Release reports expose safe IDs/states, not knowledge content.

## Observability

Add bounded counters for:

- promotion created;
- promotion idempotent replay;
- promotion revised;
- promotion rejected by eligibility reason;
- promoted eval passed/failed/skipped;
- release blocked by promoted case.

Logs may include tenant, gap, case, revision, and error category. They must not
include question text, answer text, snippets, or document content.

## Acceptance Criteria

1. A resolved, currently verified repair with no pending recurrence can be
   promoted.
2. Open, investigating, ignored, unverified, stale, or suspected-recurrence
   repairs cannot be promoted.
3. A stale client attempt ID is rejected even when the gap has another current
   verified attempt.
4. Identical concurrent requests create one active revision and stable case ID.
5. Re-promotion after a new successful verification appends a revision and
   preserves the stable case ID/history.
6. The active record retains tenant, gap, exact verification attempt, snapshot,
   question, space, policy, actor, and time provenance.
7. Default promoted cases pass for `grounded` or `partially_supported` current
   evidence, even when supporting document/chunk identity changes.
8. Promoted cases fail for `unsupported`, `review_gated`, wrong-space,
   execution-error, or missing required-document outcomes.
9. A failed or skipped promoted required case blocks a knowledge release through
   the existing `ReleaseGate`.
10. Static eval fixtures load and execute unchanged.
11. Static and promoted case ID collisions fail closed.
12. File-backed promotion records and active revision survive restart and
    concurrent writes without orphan temporary files.
13. API list/action behavior is bounded, tenant-isolated, and uses stable error
    categories.
14. Repair Inbox state, blocked reasons, promotion controls, and bounded history
    render safely.
15. No Phase 26 trend aggregation or dashboard enters the implementation.

## Stage 2 Planning Questions

Stage 2 must lock:

1. the exact promotion store interface and atomic active-revision update;
2. how promoted cases are supplied to CLI/release runs without coupling
   `internal/evals` to `internal/admin`;
3. the provider-free execution adapter boundary and support-state classifier;
4. required-check propagation and duplicate-case failure semantics;
5. API DTOs, UI states, metrics names, and the full TDD matrix;
6. local QA mechanics for producing a promoted case and demonstrating a release
   block.

## Phase Boundary

Phase 25 ends with deterministic repair-derived regression cases and release-
gate enforcement. Phase 26 may aggregate promotion and run history into quality
trends, but Phase 25 does not add trend calculations, charts, alerts, scheduled
runs, or cross-tenant analytics.

## Approval Gate

Approve this spec to enter Stage 2 autoplan. No production code may be written
before the Phase 25 plan is approved.
