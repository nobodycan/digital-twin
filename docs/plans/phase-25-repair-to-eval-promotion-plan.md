# Phase 25 Repair-to-Eval Promotion Plan

Date: 2026-07-11

Status: Approved; Stage 2 gate passed; Stage 3 in progress

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 25 Repair-to-Eval Promotion Spec](../specs/phase-25-repair-to-eval-promotion.md)

Source design: [Phase 25 Repair-to-Eval Promotion Design](../design/phase-25-repair-to-eval-promotion.md)

Prior phase: [Phase 24 Recurrence Watch Plan](./phase-24-recurrence-watch-plan.md)

## Plan Summary

Phase 25 adds an operator-initiated, tenant-scoped promotion ledger. An active
promotion projects into the existing eval system as one required RAG check. It
runs provider-free against current reviewed knowledge, writes normal suite
reports, and blocks a release through the existing `ReleaseGate` when support
falls below its configured floor.

The plan does not write runtime state into source-controlled fixtures, build a
second eval runner, call an LLM, auto-promote repairs, add promotion waivers, or
implement Phase 26 trends.

## Autoplan Review

### CEO: 8.5/10

The narrow wedge is correct: promote a verified repair only when an operator
decides that this question deserves permanent regression coverage. The product
must protect support state, not one historical source, otherwise benign evidence
changes create noisy release failures.

### Design: 8/10

Promotion stays in the Repair Inbox. A row must explain whether it is eligible
before exposing `Promote to eval`; default support floor is
`partially_supported`, while exact-source selection remains collapsed advanced
UI. Required states are eligible, stale/unverified blocked, recurrence blocked,
submitting, promoted, promotion-stale, and request failure.

### Engineering: 9/10

`internal/admin` owns promotion eligibility/storage, `internal/evals` owns
neutral case/evaluation contracts, and command/server composition roots are the
only adapters between them. The critical protections are atomic revision
advance, stale-action rejection, tenant+gap case IDs, required-RAG-only
propagation, duplicate case failure, and fail-closed dynamic execution.

### DX: 8/10

Keep static eval invocations working while adding explicit `--tenant`,
`--admin-data`, and `--include-promoted` CLI flags. A report must show case ID,
promotion revision, support state, and a recovery action without raw knowledge
content. Target time to the first promoted local eval is under five minutes.

### Outside Voices

No independent external subagent was available in this Codex session. This plan
therefore records single-model review status and compensates with an explicit
failure registry and broad TDD matrix. Stage 4 still requires fresh-context
diff review.

## Premise Challenge

| Premise | Decision | Reason |
| --- | --- | --- |
| Auto-promote every verified repair | Reject | An operator must decide what deserves release-gate cost. |
| Write `evals/conversations` from admin | Reject | Runtime state must not mutate a source checkout or merge state. |
| Freeze original source/chunk/rank | Reject by default | Equivalent active-reviewed evidence should continue to pass. |
| Build a repair-only eval runner | Reject | It creates duplicate reports and release decisions. |
| Default support floor is grounded | Reject | `partially_supported` is valid support; grounded remains optional. |
| Verification alone permits promotion | Reject | A pending suspected recurrence is unresolved risk. |

The user approved the Phase 25 spec containing these premises; that approval is
the premise confirmation for this autoplan run.

## Existing Leverage

| Need | Existing code | Extension |
| --- | --- | --- |
| Gap lifecycle/question | `admin.KnowledgeGapService` | Re-read canonical state at promotion time. |
| Proof freshness | `RepairVerificationService.CurrentProjection` | Require the exact current verified attempt. |
| Recurrence safety | `RepairRecurrenceService.Project` | Reject only pending `suspected` recurrence. |
| Repair Inbox | `KnowledgeRepairService`, `web/admin.js` | Add promotion projection/action/history. |
| Durable records | file/in-memory admin stores | Add append-only promotion revisions. |
| Eval/reports | `internal/evals` | Add promoted-case input and RAG support assertions. |
| Release gate | `governance.ReleaseGate` | Reuse required-check failure semantics. |

## Architecture

```text
Repair Inbox
  -> RepairEvalPromotionService
       -> gap: resolved + canonical question/space
       -> verification: current verified attempt
       -> recurrence: no pending suspect
       -> promotion revision store

CLI/server composition root
  -> static fixture source -----+
  -> active promotion source ---+-> eval case coordinator -> evals.Runner
  -> provider-free executor ----+                         -> reports
                                                              -> ReleaseGate
```

### Package Boundaries

- `internal/admin`: promotion records, store, eligibility, safe projections;
  it never imports `internal/evals`.
- `internal/evals`: neutral promoted-case input, eval case conversion, execution
  interface, RAG assertions, duplicate validation; it never imports admin stores.
- `cmd/cli` and `cmd/server`: composition roots that may import both packages.

This boundary prevents an admin/evals dependency cycle and makes the existing
runner/release gate the single source of evaluation truth.

## Domain Contract

Add append-only `RepairEvalPromotionRevision` records and a current projection.
Every revision contains safe tenant/gap/space IDs, tenant+gap-derived stable case
ID, canonical bounded question, verification attempt/snapshot provenance, support
floor, sorted optional document IDs, revision number, active flag, actor, and
timestamps. The store atomically maintains one active revision per tenant+gap.

The case ID is `repair-eval-<tenant-id>-<gap-id>`. Repeating an equal request
returns the existing active revision. Re-promotion after a later verified attempt
appends a revision while preserving the case ID and historical provenance.

Promotion eligibility is:

```text
resolved + verified + no suspected recurrence + current attempt -> eligible
same attempt and policy                                 -> idempotent replay
new verified attempt or policy                          -> next revision
anything else                                           -> stable conflict/no mutation
```

Promotion never changes gap lifecycle, verification attempts, recurrence records,
or knowledge documents. A stale verification leaves its active regression case
in place so the later eval can reveal the regression.

## Eval And Release Contract

Extend `evals.Case` additively with `RequiredChecks` and safe provenance. Extend
`RAGExpectation` with fixed knowledge space, minimum support state, and optional
required document IDs. Extend `EvaluationOutput` with normalized support state,
space, safe source IDs, no-source category, and execution category.

Promoted cases set exactly `required_checks: ["rag"]`. The runner marks only
named checks as required. Skipped persona/tool/memory/safety checks stay
non-required; a missing promoted RAG executor is itself a required skipped or
failed result. This preserves existing static fixture behavior and prevents
unrelated skips from blocking releases.

The case coordinator loads static fixtures plus active promoted inputs, validates
case ID uniqueness before execution, and fails closed on collision. Promoted
inputs always use a provider-free executor against current active-reviewed
knowledge; fixture output is never trusted for them.

The default support floor is `partially_supported`: grounded and partially
supported pass; unsupported, review-gated, provider/local/guard states, wrong
space, and execution errors fail. `grounded` is an optional stricter floor.
Exact documents are optional and empty by default.

## API And UI Plan

Add:

- `POST /admin/knowledge/repairs/promotions`
- `GET /admin/knowledge/repairs/promotions?gap_id=&active=&limit=`

POST accepts only gap ID, displayed verification attempt ID, support floor,
optional document IDs, and bounded actor. Server derives tenant, question,
space, case ID, timestamps, and provenance. Use `400` input, `404` scoped
missing, `409` eligibility/stale action, `503` unavailable dependency, and
`500` internal persistence errors without raw causes.

Repair rows gain promotion state, case ID, revision, promoted time, support floor,
and a stable blocked reason. Eligible rows show the floor selector and promote
button; promoted rows show history; ineligible rows show the next repair action.
Use `textContent`, a bounded 20-item history, keyboard-accessible native controls,
and the existing status region/fetch helpers.

## Implementation Slices

### P25-01 Promotion Store

Create types, validation, cloning, in-memory/file stores, atomic active revision
advance, safe sorting/limits, restart, and concurrency behavior.

Files: `internal/admin/knowledge_repair_promotion.go` and tests.

### P25-02 Promotion Service And Repair Projection

Implement eligibility/idempotency/revision service, tenant/document validation,
and promotion fields in `KnowledgeRepairItem`.

Files: promotion service/tests and `internal/admin/knowledge_repair.go`.

### P25-03 Eval Contracts

Add neutral promoted input, case/output/RAG fields, required-check behavior,
duplicate validation, support-floor checks, and safe report provenance.

Files: `internal/evals/case.go`, `evaluator.go`, `runner.go`, reporter/tests.

### P25-04 Provider-Free Execution And Composition

Add promoted executor interface/adapters and compose static plus promoted sources
in CLI/server paths. Add tenant/admin-data/include-promoted CLI flags and ensure
the release path uses the same suite result.

Files: `internal/evals/*`, `cmd/cli/main.go`, `cmd/server/main.go`, relevant
server/governance tests.

### P25-05 Admin Endpoints And Repair Inbox

Wire the store/service, endpoints, list/projection DTOs, inline controls/history,
and static web assertions.

Files: `cmd/server/main.go`, `internal/server/server.go`, `web/admin.js`, tests.

### P25-06 Docs And Gates

Update README/release notes and then run Stage 4 review, Stage 5 browser QA,
Stage 6 CSO, and Stage 7 verification/ship.

## TDD Matrix

| ID | Scenario | Expected result |
| --- | --- | --- |
| P25-T01 | Resolved, verified, no suspect | Creates revision 1 and stable case ID. |
| P25-T02 | Open/investigating/ignored/stale/unverified | Stable conflict and no mutation. |
| P25-T03 | Pending suspected recurrence | `recurrence_pending`, no mutation. |
| P25-T04 | Dismissed recurrence history | Still eligible. |
| P25-T05 | Supplied attempt is stale | Conflict and no mutation. |
| P25-T06 | Identical replay | Same revision/case, no append. |
| P25-T07 | New verified attempt | Next revision, stable case, old history retained. |
| P25-T08 | Concurrent same request | One active revision; no temp files. |
| P25-T09 | Same gap ID across tenants | Separate cases; no cross-tenant action. |
| P25-T10 | Invalid optional document | Reject before persistence. |
| P25-T11 | File store reopen | Revision/projection survive restart. |
| P25-T12 | Repair Inbox projection | Promotion state independent from proof/recurrence. |
| P25-T13 | Old static fixture | Existing loading/eval behavior unchanged. |
| P25-T14 | Promoted conversion | Fixed question/space/provenance and required RAG only. |
| P25-T15 | Required RAG, unrelated skips | Only RAG is required. |
| P25-T16 | Grounded/default floor | Pass. |
| P25-T17 | Partially supported/default floor | Pass. |
| P25-T18 | Unsupported/review-gated/error modes | Fail closed. |
| P25-T19 | Grounded floor + partial result | Fail. |
| P25-T20 | Equivalent source changes | Pass with no document constraint. |
| P25-T21 | Required document absent | Fail with safe ID. |
| P25-T22 | Missing promoted executor | Required RAG fails/skips; gate blocks. |
| P25-T23 | Static/promoted ID collision | Suite setup fails before eval. |
| P25-T24 | Mixed case suite | Reports include safe promotion provenance. |
| P25-T25 | Failed promoted RAG | Existing release gate blocks. |
| P25-T26 | Passing promoted RAG | Existing release gate permits. |
| P25-T27 | No promotion store | Static CLI eval still passes. |
| P25-T28 | CLI tenant/admin-data flags | Loads selected tenant/path only. |
| P25-T29 | Promote/list API validation | Bounded tenant-isolated stable errors. |
| P25-T30 | Persistence/executor API error | No raw cause and no partial state. |
| P25-T31 | Static UI contract | Safe text, controls, states, bounded history. |
| P25-T32 | Local browser QA | Promote, force unsupported evidence, eval, release block. |

## Test Diagram

```text
promotion POST -> eligibility (T01-T05,T10,T29-T30)
               -> replay/revision/concurrency (T06-T11)
               -> Inbox projection (T12,T31)
promotion -> eval projection -> compatibility/duplicates (T13-T15,T23-T24)
                          -> support assertions (T16-T22)
                          -> CLI source selection (T27-T28)
                          -> ReleaseGate (T25-T26)
                          -> browser workflow (T32)
```

## Error And Rescue Registry

| Failure | Operator feedback | Recovery |
| --- | --- | --- |
| Gap not resolved | Resolve repair first | Use existing resolve flow. |
| Verification stale/unverified | Current verification required | Run verify again. |
| Pending recurrence | Recurrence needs review | Confirm or dismiss it. |
| Stale browser attempt | Repair state changed | Reload Inbox and retry. |
| Invalid document constraint | Safe document/category error | Pick active same-space source or remove it. |
| Store failure | Stable server error | Retry; repair state was unchanged. |
| Dynamic eval failure | Required case failure | Restore executor and rerun eval. |
| Release blocked | Case/revision/state in report | Repair, verify, re-promote, rerun. |

## Failure Modes

| Mode | Severity | Mitigation | Tests |
| --- | --- | --- | --- |
| Cross-tenant record access | High | Tenant in every key/query/action | T09,T29 |
| Duplicate active revisions | High | Atomic store and fingerprint | T06-T08,T11 |
| Stale browser promotion | High | Current verified attempt required | T05 |
| Pending recurrence bypass | High | Eligibility projection check | T03-T04 |
| Fixture overrides dynamic case | High | Duplicate-ID validation | T23 |
| Irrelevant skipped check blocks | High | Named required checks only | T15 |
| Missing executor passes release | High | Required fail-closed RAG | T22 |
| Valid source churn blocks | Medium | State floor/default no document pin | T16-T21 |
| Raw knowledge leak | High | Safe scalar-only DTO/report/UI | T24,T30-T31 |

## Performance, Security, And Accessibility

- Promotion reads bounded records/documents for one gap and space only; list/history
  defaults to 20 and clamps at 100.
- Case source duplicate validation is O(n) with a set; promoted execution runs once
  per case and makes no LLM calls.
- Reject client tenant/question/space/case/timestamp inputs; validate constrained
  documents in the same tenant, space, and active-review state.
- No raw prompts, answers, snippets, or diagnostic errors enter records/reports/UI.
- UI controls have labels, keyboard support, no color-only state, submission lock,
  responsive bounded history, and text-only DOM rendering.

## NOT In Scope

Batch/automatic promotion, deletion/deactivation/waivers, scheduled evals,
semantic matching, answer judging, trends, dashboards, alerts, cross-tenant
analytics, retrieval scoring changes, document-review changes, or a new release gate.

## Decision Audit Trail

| # | Phase | Decision | Class | Principle | Rejected |
| --- | --- | --- | --- | --- | --- |
| 1 | CEO | Operator promotion only | Mechanical | Explicit | Auto-promotion |
| 2 | CEO | State floor over source fingerprint | Mechanical | Completeness | Exact source by default |
| 3 | Eng | Ledger into existing eval path | Mechanical | DRY | Second runner |
| 4 | Eng | Tenant+gap stable case ID | Mechanical | Explicit | Gap-only ID |
| 5 | Eng | Named required RAG check | Mechanical | Explicit | Case-wide required flag |
| 6 | Eng | Missing executor fails closed | Mechanical | Completeness | Fixture fallback |
| 7 | Design | Partial-support default floor | Taste | Pragmatic | Grounded-only default |
| 8 | Design | Advanced collapsed source constraint | Mechanical | Subtraction | Always-visible picker |
| 9 | DX | Promotions included by default in CLI | Taste | Action | Separate command |

## Stage 2 Approval Gate

Approve this plan to enter Stage 3 TDD. Implementation must preserve package
boundaries, required-RAG-only semantics, and the complete test matrix above.
