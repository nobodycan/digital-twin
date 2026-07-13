# Phase 28 Knowledge Quality Review Checkpoints Spec

Date: 2026-07-13

Status: Approved; Stage 1 gate passed

Prior phase: [Phase 26 Knowledge Quality Trends Spec](./phase-26-knowledge-quality-trends.md)

## Problem Statement

Phase 26 gives a local knowledge operator a bounded, evidence-backed view of
recent verification, recurrence, and promoted-eval activity. It deliberately
does not decide what that evidence means, remember a review decision, or link
a decision to the repair work that should follow. The operator must therefore
repeat the same interpretation on every visit, with no durable baseline for
the next review.

The tempting response is a quality score, automatic alert, or broad workflow
system. Those would overstate what the observed records prove and expand the
admin surface beyond the current local-first product. Phase 28 instead records
one explicit human conclusion over one existing bounded trend projection.

## Target User And Wedge

The target user is the local knowledge operator who has reviewed Quality
Trends for one tenant and optionally one knowledge space, and now needs to
record whether to observe, repair, or consciously accept a bounded risk.

The narrow wedge is an append-only **Quality Review Checkpoint** created from
the active Quality Trends filter. A checkpoint preserves the safe observed
metrics shown at review time, the operator's explicit conclusion, a short
rationale, and optional links to existing repair gaps. It is not a task
manager, alerting engine, or accuracy assessment.

## Office-Hours Findings

### The Actual Job

An operator needs an answer to: "What did we observe in this bounded period,
what did we decide, and which known repair items should we revisit?" The next
review must be able to compare with the most recent compatible decision
without reconstructing history from changing source records.

### Premises Challenged

1. **A composite quality score makes decisions comparable.** Rejected. The
   underlying signals are partial observations with different denominators;
   a score would imply unobserved knowledge has been measured.
2. **Saving only a free-form note is sufficient.** Rejected. It loses the
   date/space scope and evidence snapshot necessary to interpret the note.
3. **Creating a checkpoint should automatically change repair state.**
   Rejected. A review conclusion is not proof that a gap was fixed, assigned,
   or safe to close.
4. **Any historical checkpoint can be compared.** Rejected. Different tenant,
   space scope, timezone, or local-calendar window width creates a misleading
   delta. A prior period with the same scope and width may be compared only when
   its end date is earlier and both date windows and denominators stay visible.
5. **An accepted risk may be inferred from no action.** Rejected. Risk
   acceptance must be an explicit, durable human conclusion with a reason.

## Goals

1. Persist an append-only, tenant-scoped review checkpoint over one server
   recomputed Quality Trends projection.
2. Preserve the effective date range, timezone, optional space scope, and
   safe metric snapshot without storing raw answers, question text, sources,
   diagnostics, or evaluator output.
3. Require one explicit outcome: `observe`, `repair_required`, or
   `risk_accepted`.
4. Let `repair_required` and `risk_accepted` reference a bounded set of
   server-recomputed, evidence-visible, tenant/space-compatible repair gap
   IDs, without mutating them.
5. Show the most recent compatible earlier-window checkpoint and honest metric
   deltas only where both snapshot values are available and comparable.
6. Add a compact review and history surface beside existing Quality Trends.
7. Preserve all existing repair, verification, recurrence, promotion, eval,
   trend, admin-access, and local-mode behavior.

## Non-Goals

- A knowledge correctness, confidence, or composite quality score.
- Automatic alerts, scheduling, assignment, remediation, promotion, or gap
  status changes.
- Editing or deleting checkpoints in this phase.
- Cross-tenant, cross-space, cross-timezone, mixed-window-width, or
  same/later-window comparisons.
- New identity, role, approval, or audit-authorship semantics.
- Storing raw prompts, answers, source snippets, titles, diagnostic details,
  arbitrary user data, or unbounded trend trace lists.
- Export, notification, dashboard, or background job work.

## Product Decisions

### Server-Recomputed Snapshot

`POST /admin/knowledge/quality-review-checkpoints` accepts only the existing
Quality Trends filter plus a conclusion, rationale, selected gap IDs, and a
required opaque idempotency key. The server derives the tenant from the
existing admin context, recomputes the projection through
`QualityTrendService`, and persists the normalized filter and an allow-listed
metric snapshot. The client cannot submit counts, rates, timestamps, tenant
IDs, a prior checkpoint ID, or an eligible-candidate list.

The server normalizes the decision request and checks the idempotency key
before performing a new projection. The canonical request fingerprint contains
only the derived tenant, normalized filter, outcome, trimmed rationale, and
sorted selected gap IDs. It excludes all recomputed metrics and server times.
The same key and fingerprint returns the originally persisted checkpoint
without recomputing or appending; the same key with a different fingerprint is
a conflict. Only a previously unseen key proceeds to recomputation and append.

The request uses the same inclusive local-date and 1-to-90-day validation as
Quality Trends. A checkpoint stores its creation time separately from the
projection's current stale-verification observation time.

### Explicit Outcomes And Bounded Rationale

The allowed outcomes are:

- `observe`: evidence is retained for the next review; no linked gap is
  required.
- `repair_required`: one to ten existing gap IDs must be linked; the rationale
  is required.
- `risk_accepted`: one to ten existing gap IDs must be linked; the rationale
  is required and records a deliberate exception, not a closure.

Rationale is plain text, trimmed, and capped at 500 UTF-8 characters. Empty,
duplicate, malformed, cross-tenant, or out-of-scope gap IDs are rejected.
For an all-spaces review, a selected gap may belong to any space in the tenant;
for a space-scoped review, every selected gap must match that space. In both
cases the ID must also occur in the candidate set derived from the save-time
projection: the deduplicated union of repeated-recurrence gap IDs and promoted
eval-failure gap IDs, capped at ten in stable projection order. Tenant/scope
membership alone is insufficient. The server does not capture a claimed
reviewer identity because the current local admin boundary has no durable,
trustworthy person identity to attribute.

### Immutable Local Ledger

Checkpoints are stored in a dedicated append-only local file-backed ledger
with an in-memory implementation for tests. The exact file path and locking
strategy are Stage 2 decisions. The implementation must define its real
single-process durability and replacement guarantees without claiming that
`os.Rename` is atomic on every platform. A failed append returns a controlled
error and never changes gap state.

Each record has a generated stable ID, server-derived tenant and filter,
creation timestamp, outcome, rationale, selected gap IDs, and safe snapshot.
The ledger does not store current trend trace arrays or repeated-question text.
Selected IDs remain historical references if a later data change removes or
alters a gap; reading a checkpoint must not fail because its source changed.

### Equivalent Earlier-Window Checkpoint Only

The create response and list projection may identify the compatible checkpoint
with the latest earlier window end. Compatibility requires the same tenant,
optional-space value, timezone, and local-calendar window-day count. The prior
window's end date must be strictly earlier than the current window's end date.
The latest window end wins, with creation time and checkpoint ID as stable
tie-breakers. An exact repeated `from`/`to` window remains visible in history
but is not a periodic predecessor. If no eligible earlier window exists, the UI
says there is no compatible prior review.

Count deltas are published only for allow-listed numeric counts present in both
snapshots. Rate and duration deltas are published only when both stored metric
values have status `observed`; `insufficient_sample`, `no_observations`, and
`unavailable` remain labels, never converted to zero or a direction. Current
stale-verification counts are explicitly compared as current-at-review
snapshots, not historical trend values. Every comparison shows both date
windows and both rate denominators and uses neutral `review-to-review
difference` language rather than improvement, regression, or causality.

### HTTP Contract Direction

Phase 28 adds these tenant-derived routes:

```text
POST /admin/knowledge/quality-review-checkpoints
GET  /admin/knowledge/quality-review-checkpoints?space_id=<optional>&limit=<n>
```

The list endpoint returns newest-first history for the derived tenant and exact
all-spaces or selected-space scope, across checkpoint date windows, capped at
20 records. It may return the compatible earlier-checkpoint ID and safe deltas
for each entry. An unavailable checkpoint service or ledger returns `503`;
malformed scope, limit, or request data returns `400`; an oversized request
returns `413`; an absent, foreign, wrong-scope, or non-candidate gap and an
idempotency-key mismatch return non-enumerating `409` responses. A first create
returns `201`; a same-key replay returns `200`. No endpoint accepts
`tenant_id`.

Every Phase 28 error response keeps the repository's top-level `error` machine
code and adds safe, human-readable `message` and `hint` fields. The existing
`X-Request-ID` response header is the correlation identifier. Stable codes
distinguish invalid input, idempotency conflict, gap eligibility conflict,
oversized input, evidence unavailability, checkpoint-service unavailability,
and storage unavailability without exposing tenant or gap existence details.

An indicative create request is:

```json
{
  "idempotency_key": "qr-01K0V8Q2M7E6W5T4R3Y2",
  "from": "2026-06-14",
  "to": "2026-07-13",
  "space_id": "support",
  "outcome": "repair_required",
  "rationale": "Repeated recurrence needs a focused repair review.",
  "gap_ids": ["gap-123"]
}
```

### UX Direction

Keep the feature inside the existing Knowledge Quality Trends section. After a
successful trend load, the operator can choose an outcome, enter a bounded
rationale, select evidence-derived IDs from the currently rendered repeated
recurrence and promoted-eval failure signals, and save a checkpoint. Candidates
do not come from Repair Inbox state or arbitrary text input. The interface must
clearly state that saving does not modify repair status.

Show a bounded newest-first checkpoint history across date windows for the
active scope, with filter, created time, outcome, rationale, linked safe IDs,
and compatible comparison state. A comparison shows both windows and both
denominators. A linked gap ID should reuse the existing Repair Inbox
detail/action context where one exists; it must not manufacture a new route or
claim a repair was completed. The UI uses text nodes and clear labels such as
"observed evidence", "current stale count", and "review-to-review difference",
never green/red quality language or a success score.

## Security And Failure Boundaries

- Tenant derives exclusively from the existing admin context for trend reads,
  gap validation, ledger writes, lists, previous-checkpoint lookup, and output.
- The server validates every linked gap against the save-time projection's
  eligible candidate set and the resolved tenant/filter scope before writing;
  client-provided candidate lists or question text are never accepted.
- Requests, rationale, IDs, history, and stored snapshots are bounded.
- Stored metrics reuse Phase 26's honest sample statuses; unavailable and
  withheld values cannot become zero or a favorable conclusion.
- The checkpoint ledger stores no credentials, admin tokens, raw knowledge,
  answers, source content, audit payloads, or evaluator errors.
- Read/list failures return controlled errors; malformed historical entries are
  excluded with controlled diagnostics and cannot panic the admin handler.
- This feature adds no background process, network egress, Docker dependency,
  or new authentication bypass.

## Acceptance Criteria

1. A create request persists one server-recomputed, tenant-scoped safe snapshot
   with an immutable ID and timestamp; it never trusts client metrics.
2. `observe`, `repair_required`, and `risk_accepted` are the only outcomes;
   repair and risk outcomes require a bounded rationale and one to ten valid
   gap IDs.
3. A checkpoint create or storage failure never changes gap, verification,
   recurrence, promotion, eval, or release-gate state.
4. Tenant and space scope prevent a checkpoint from linking or exposing any
   other tenant's gap or metric data.
5. The ledger survives reopen and returns no more than 20 newest matching
   records in deterministic order.
6. A prior comparison is produced only for the same tenant/scope/timezone and
   local-calendar window width with a strictly earlier end date; both windows
   and denominators are shown, and missing or withheld values remain
   non-directional and never become zero.
7. The UI saves only after a trends projection is loaded, renders safe history,
   and states that checkpoints do not alter repairs.
8. Existing Quality Trends filter behavior, small-sample labels, Repair Inbox,
   admin access boundary, and local unauthenticated mode remain unchanged.
9. A first idempotent create returns one checkpoint, an unchanged retry returns
   that original checkpoint without recomputation, and key reuse for a changed
   normalized decision returns a conflict without writing.
10. README documentation provides copy-paste PowerShell and curl create,
    replay, and list examples, expected statuses, error recovery, local file
    location, and additive schema-v1 rollback behavior.

## Test Matrix Direction

Stage 2 must turn these into RED-first tests:

| Boundary | Required proof |
| --- | --- |
| Ledger | append-only write, generated ID, reopen, replacement failure, deterministic newest-first bounded list, same-key replay/conflict |
| Snapshot | server recomputation, allow-listed fields, creation versus projection time, no trace/question/source data |
| Outcome validation | allowed enum, UTF-8 rationale cap, required rationale/IDs, duplicate and malformed IDs |
| Tenant and scope | mixed tenants/spaces and non-candidate same-tenant gaps cannot link, list, compare, or leak through errors |
| Comparison | same scope/timezone/window width, strictly earlier end, deterministic predecessor, exact-window exclusion, count/rate/duration status rules, visible windows/denominators, no-prior state, stale snapshot label |
| HTTP | tenant derivation, filter reuse, stable 201/200/400/409/413/503 responses, list cap |
| UI | disabled-before-load state, create flow, safe text rendering, history and comparison labels, no repair mutation |
| Regression | existing admin, repair, verification, recurrence, promotion, eval, trend, and access-boundary tests stay green |

## Alternatives Considered

### A. Equivalent-Window Review Checkpoints Over Existing Trends (Recommended)

Records an explicit human decision and a safe snapshot without pretending the
system can score knowledge quality or manage a full workflow. Periodic
comparisons use the same local-calendar window width and make both periods and
sample denominators explicit.

### B. Repair-Eval Lifecycle Controls

Could add promotion deactivation, waiver, or revision history. This remains a
valuable future operational feature, but it does not answer how an operator
records and compares a broad observed quality review.

### C. Composite Score And Alert Thresholds

Rejected. The available evidence is partial, sample-bounded, and mixed in
meaning; an automatic score or threshold would create false certainty.

## Stage 2 Assignment

Run autoplan against this spec. The engineering review must lock the service
and store package boundaries, exact persisted snapshot schema, safe error
taxonomy, honest whole-file replacement semantics, equivalent-window predecessor key and
comparison allow-list, and UI integration.
It must retain append-only human decisions and reject automatic repair changes,
identity attribution, a composite score, and Docker work.

## Stage 1 Gate

No implementation has been started. Approval of this spec is required before
entering Stage 2 autoplan.
