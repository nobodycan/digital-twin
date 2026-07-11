# Phase 25 Repair-to-Eval Promotion Design

Date: 2026-07-11

Status: Draft; accompanies the Phase 25 Stage 1 spec

Source spec: [Phase 25 Repair-to-Eval Promotion Spec](../specs/phase-25-repair-to-eval-promotion.md)

## Design Summary

Phase 25 adds a durable, tenant-scoped promotion ledger between the Repair Inbox
and the existing eval system. An operator promotes one currently verified,
resolved repair. The active promotion projects into a required RAG eval case and
runs provider-free against current active-reviewed knowledge. Existing static
fixtures, reports, runner, and release gate remain the single eval path.

## Recommended Architecture

```text
Repair Inbox
  -> Promotion service
       -> re-read gap + current verification + recurrence
       -> append promotion revision / advance active projection
       -> promotion store

Eval case source
  -> static JSON cases
  -> active promotion projections
  -> duplicate-ID validation
  -> existing eval runner
       -> static cases use fixture output
       -> promoted cases use provider-free retrieval execution
  -> existing suite/report
  -> existing ReleaseGate
```

The promotion package owns persistence, eligibility, idempotency, revisions,
and conversion into a neutral promotion DTO. The eval package owns conversion
from that DTO into `evals.Case`, execution evidence, and assertions. This avoids
an `internal/evals -> internal/admin` dependency and keeps the release gate
unaware of admin storage.

## Premise Challenge

### Premise: Promotion Should Write Static JSON Fixtures

Rejected. The running admin service should not mutate a source checkout, and a
file under `evals/conversations` cannot represent tenant-scoped runtime state
cleanly. Static fixtures remain valuable for hand-authored cross-cutting cases;
promoted cases belong in a data store and are projected at run time.

### Premise: Verification Evidence Should Be Frozen Exactly

Rejected by default. Verification provenance must be frozen, but the eval
assertion should protect the supported behavior rather than one document,
chunk, rank, or evidence hash. Optional exact document constraints remain
available for repairs whose contract genuinely depends on a named source.

### Premise: A Separate Repair Eval Runner Is Simpler

Rejected. It is locally simpler but creates two report formats, two failure
semantics, and two release-gate integrations. Extending the existing case and
output contracts additively is the smaller system-level design.

## Approaches Considered

### A. Runtime Promotion Ledger Into Existing Eval Path (Recommended)

Persist append-only promotion revisions, project the active revision into an
ordinary required RAG case, execute it through a provider-free adapter, and feed
the result to the existing runner/release gate.

Benefits: one eval truth, durable tenant provenance, no source-tree writes,
stable case identity, and direct release enforcement. Cost: Stage 2 must define
a clean adapter boundary between admin storage and eval execution.

### B. Generate Source-Controlled Eval JSON

Generate `evals/conversations/*.json` when an operator promotes a repair.

Benefits: immediate compatibility with `LoadCases`. Costs: unsafe runtime repo
mutation, difficult multi-tenant behavior, merge conflicts, no clean revision
history, and fixture output that does not test current retrieval. Rejected.

### C. Dedicated Repair Regression Runner

Keep promotion records in admin storage and run them through a new repair-only
evaluator and report.

Benefits: minimal changes to existing eval types. Costs: duplicate execution,
reporting, required-check, and release-gate semantics. Rejected.

## Key Contracts

- One tenant+gap-derived stable case ID; append-only promotion revisions.
- Promotion requires resolved + current verified + exact current attempt + no
  pending suspected recurrence.
- Default floor is `partially_supported`; `grounded` is optional and stricter.
- Exact source requirements are optional and empty by default.
- Promoted cases require the `rag` check and use risk `high`; unrelated skipped
  evaluator checks remain non-required.
- Missing execution fails closed at the release gate.
- Static fixture behavior remains unchanged.

## Data Flow

### Promotion

1. UI submits gap ID, displayed current attempt ID, support floor, optional
   document constraints, and actor.
2. Service re-reads canonical state and rejects stale UI decisions.
3. Service validates optional documents in the same tenant/space.
4. Store returns the identical active revision for an idempotent replay or
   appends the next revision atomically.
5. Repair Inbox projects the stable case ID and current revision.

### Evaluation

1. A case-source coordinator loads static cases and active promotion DTOs.
2. Promotion DTOs convert into required RAG cases with safe provenance.
3. Duplicate case IDs fail suite setup.
4. Static cases use fixture outputs; promoted cases execute current retrieval.
5. RAG evaluation compares normalized support state, space, and optional
   required documents.
6. Suite/report and release gate process all checks without a second path.

## Error Handling

- Promotion eligibility conflicts return stable `409` categories.
- Cross-tenant missing records remain indistinguishable from ordinary missing
  records.
- Store failures leave repair state unchanged.
- Execution failures become required failed/skipped checks, never passes.
- Duplicate static/promoted IDs abort the suite before evaluation.
- Re-promotion does not erase old provenance or change the stable case ID.

## Test Strategy Direction

Stage 2 should derive tests across five boundaries:

1. promotion eligibility and stale-action rejection;
2. store idempotency, revisions, tenant isolation, restart, and concurrency;
3. projection compatibility and duplicate case detection;
4. provider-free execution plus support/document assertions;
5. required-check release blocking and Repair Inbox/API behavior.

The end-to-end proof is: verify a resolved gap, promote it, change reviewed
knowledge so retrieval becomes unsupported or review-gated, run eval, and
observe the existing release gate block the knowledge candidate.

## Assignment For Stage 2

Use autoplan to lock package boundaries and a test matrix before implementation.
The engineering review must specifically prevent `internal/evals` from importing
admin persistence and must prove that promoted cases execute current retrieval
rather than embedded fixture output.
