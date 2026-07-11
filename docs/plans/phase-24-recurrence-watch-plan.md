# Phase 24 Recurrence Watch Plan

Date: 2026-07-11

Status: Approved; Stage 3-6 complete; Stage 7 PR created; land/deploy pending approval

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 24 Recurrence Watch Spec](../specs/phase-24-recurrence-watch.md)

Source design: [Phase 24 Recurrence Watch Design](../design/phase-24-recurrence-watch.md)

Source roadmap: [Phase 23-26 Repair Verification Roadmap](../design/phase-23-26-repair-verification-roadmap.md)

## Plan Summary

Phase 24 adds a tenant-scoped, durable recurrence watch. After a weak answer is
successfully recorded in the audit trail, a best-effort detector compares it to
currently verified, resolved repair gaps using only an explicit gap reference
or an exact fallback key. It creates or consolidates a `suspected` recurrence;
an operator alone may confirm reopening or dismiss it. Verification freshness,
gap lifecycle, existing retest behavior, and normal-answer delivery remain
separate and compatible.

## What Already Exists

- `KnowledgeGapService` owns the only gap lifecycle: `open`, `investigating`,
  `resolved`, and `ignored`.
- `AuditService` persists tenant-scoped weak-answer evidence with audit IDs,
  bounded question summaries, space, answer state, and no-source reasons.
- `RepairVerificationService` persists immutable attempts and derives
  `verified`, `stale`, and `unverified` states from active reviewed knowledge.
- Server answer handling records an audit entry before the legacy
  `captureKnowledgeGap` operation.
- `KnowledgeRepairService` projects the Repair Inbox and the admin UI already
  has text-only rendering and bounded inline history patterns.

## Autoplan Review Summary

### CEO Review: 8.5/10

The product value is early operator attention, not automated repair policy. A
recurrence signal is useful only when it is narrow enough to trust, so Phase 24
must prefer a missed ambiguous recurrence over a false auto-reopen. The operator
needs a single place to decide whether a later weak answer is a real regression.

Auto-decided: only a resolved and currently verified gap can produce a
recurrence suspicion. This keeps open work actionable through the existing
inbox rather than creating a second queue for the same issue.

### Design Review: 8/10

Keep recurrence inside the existing Repair Inbox. Show verification freshness
and recurrence as two compact lines: one answers whether the proof is current;
the other answers whether a new weak answer needs human attention. Pending
actions are explicit and reversible in effect: dismiss does not mutate a gap,
while confirm clearly says it will reopen the gap.

Auto-decided: no separate dashboard, no alert toast system, and no raw audit
content in the recurrence history. A bounded audit ID, state, reason, count,
and time explain the signal without leaking conversation content.

### Engineering Review: 9/10

Use a dedicated `RepairRecurrenceStore` and `RepairRecurrenceService`; do not
add lifecycle fields to verification attempts or `KnowledgeGap`. Detection runs
after audit persistence and returns an explicit `handled` result to decide
whether legacy gap creation should run. A durable per-observation idempotency
index prevents a replayed audit ID from incrementing an existing suspicion.

Auto-decided: detection failures are swallowed at the server boundary after
safe internal observability, then fall through to legacy gap capture. Confirm
uses existing `KnowledgeGapService.UpdateStatus(..., open, ...)` first, then
marks the recurrence confirmed; retry reconciles an already-open gap if a later
recurrence-store write failed.

### DX Review: 8/10

Use direct names: `RepairRecurrence`, `Detect`, `Confirm`, `Dismiss`, and
`GET /admin/knowledge/repairs/recurrences`. Admin errors must have stable codes
and never contain diagnostic error strings. The UI uses a small reason input
only when dismissing, retaining all current repair actions.

Auto-decided: list limits default to 20 and clamp at 100; matched-by values are
machine-readable but display as concise safe labels.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | Eligible repair | `resolved` plus current `verified` projection | A weak answer for active work is already visible; stale proof is not trustworthy. |
| 2 | Match order | Explicit gap ID, then exact fallback | Gives deterministic precision without semantic guessing. |
| 3 | Ambiguity | Safe no-op | Never pick a gap from multiple exact candidates. |
| 4 | State ownership | Separate recurrence projection | Proof freshness and later weak answers answer different questions. |
| 5 | Reopen policy | Human confirmation only | Prevents a weak event from mutating lifecycle automatically. |
| 6 | Repeat weak events | One pending record plus occurrence count | Keeps inbox scan-friendly while retaining observed recurrence severity. |
| 7 | Idempotency | Durable `(tenant, gap, audit)` observation key | Makes replay safe even when events arrive out of order. |
| 8 | Hook failure | Fall through to legacy capture | Detection cannot degrade normal answer delivery. |
| 9 | Confirm ordering | Open gap, then persist confirmation; retry reconciles | Reuses canonical lifecycle and avoids pretending cross-store transactions exist. |
| 10 | UI | Existing Repair Inbox, text-only bounded history | Preserves operator workflow and safe rendering. |
| 11 | Phase boundary | No semantic matching/evals/trends | Leaves Phases 25-26 independently shippable. |

## Architecture

```text
normal answer
  -> AuditService.Record (durable AuditRecord)
  -> RepairRecurrenceService.Detect(audit)
       -> eligible resolved + verified repair lookup
       -> exact match / ambiguity no-op
       -> recurrence store: observation dedupe + suspect create/consolidate
       -> handled true only when a suspicion was recorded
  -> legacy captureKnowledgeGap when !handled or detect error

Repair Inbox projection
  -> KnowledgeGap lifecycle
  -> RepairVerificationService.Project
  -> RepairRecurrenceService.Project

Operator action
  -> Confirm: gap UpdateStatus(open), then recurrence confirmed
  -> Dismiss: recurrence dismissed with bounded reason
```

## Data Model

Add to `internal/admin`:

```go
type RepairRecurrenceStatus string
const (
    RepairRecurrenceSuspected RepairRecurrenceStatus = "suspected"
    RepairRecurrenceDismissed RepairRecurrenceStatus = "dismissed"
    RepairRecurrenceConfirmed RepairRecurrenceStatus = "confirmed"
)

type RepairRecurrenceMatch string
const (
    RepairRecurrenceMatchExplicit RepairRecurrenceMatch = "explicit_gap_id"
    RepairRecurrenceMatchExact    RepairRecurrenceMatch = "exact_fallback"
)

type RepairRecurrence struct {
    ID                          string
    TenantID                    string
    GapID                       string
    SpaceID                     string
    Status                      RepairRecurrenceStatus
    MatchedBy                   RepairRecurrenceMatch
    FirstAuditID                string
    LatestAuditID               string
    OccurrenceCount             int
    AnswerState                 string
    NoSourceReason              string
    VerifiedAttemptID           string
    VerifiedSnapshotFingerprint string
    CreatedAt                   time.Time
    UpdatedAt                   time.Time
    DismissedAt                 *time.Time
    DismissReason               string
    ConfirmedAt                 *time.Time
    ConfirmedBy                 string
}

type RepairRecurrenceObservation struct {
    TenantID     string
    GapID        string
    AuditID      string
    RecurrenceID string
    RecordedAt   time.Time
}
```

`RepairRecurrenceStore` persists recurrence records and observation keys in one
file-backed atomic payload. Observation records are internal idempotency facts;
history endpoints return only recurrence records. In-memory and file stores use
the same tenant filtering and a mutex around read-modify-write operations.

Extend verification projection with the latest passing attempt ID and snapshot
fingerprint needed for recurrence provenance. Do not modify existing attempt
records.

## Detection Rules

1. Accept one persisted `AuditRecord`, never raw request/result values.
2. Ignore non-weak states and events without bounded question, space, or reason.
3. Find explicit candidate from safe audit evidence only when it refers to a
   tenant-scoped eligible gap.
4. Otherwise list candidate gaps in the same tenant and space; retain only
   `resolved` gaps whose verification projection is `verified`; require exactly
   one equal `(question_summary, no_source_reason)` candidate.
5. Check the persistent observation key before changing a recurrence.
6. If an observation exists, return `handled: true` with no mutation.
7. If the selected gap has one pending recurrence, update its latest audit,
   count, state/reason, and timestamp, then persist the observation.
8. Otherwise create one suspected recurrence with verification provenance and
   persist its first observation atomically.
9. `handled: false` means no eligible/unique match; the server runs existing
   gap capture. An error also results in legacy capture after it is contained.

## API Plan

### `GET /admin/knowledge/repairs/recurrences`

Required `gap_id`; optional `status` and `limit`. Limit defaults to 20 and
clamps to 100. Return newest-first, tenant-scoped safe records.

### `POST /admin/knowledge/repairs/recurrences/confirm`

Input: `{ "recurrence_id": "recurrence-...", "confirmed_by": "operator" }`.

Only a suspected record can be confirmed. It reopens the matched gap through
the canonical lifecycle service, then stores `confirmed`. If retry finds the
gap already open and the record still suspected, it completes the recurrence
state without a second lifecycle mutation.

### `POST /admin/knowledge/repairs/recurrences/dismiss`

Input: `{ "recurrence_id": "recurrence-...", "reason": "..." }`.

Reason is required, trimmed, and bounded to 240 characters. It
changes only a suspected recurrence to dismissed; it never touches the gap.

Status semantics:

- malformed JSON, invalid IDs/statuses, invalid actor/reason, non-pending
  transition, or missing record: 400;
- missing required services: 503;
- persistence/internal error: 500 with stable error code and no raw cause.

## Repair Inbox And UI Plan

1. Extend `KnowledgeRepairItem` with recurrence projection fields only:
   `recurrence_state`, `recurrence_count`, `latest_recurrence_at`, and optional
   safe latest answer state/reason.
2. Add a compact line that keeps verification and recurrence state separate.
3. For `suspected`, add a text-only `Confirm recurrence` action and a `Dismiss`
   action that opens an inline reason field; do not invoke either without a
   pending record.
4. Add `View recurrence history`, fetching no more than 20 records and using
   `textContent` for every value.
5. Refresh the Repair Inbox after detection actions; preserve all Phase 22-23
   action behavior and selectors.

## Failure And Rescue Registry

| Failure | Operator-visible result | Rescue action |
| --- | --- | --- |
| No exact eligible repair | No recurrence badge; normal gap workflow continues | Investigate the new or existing gap. |
| Ambiguous fallback | No recurrence badge; no guessed match | Use existing repair actions; improve explicit linkage later. |
| Pending suspect | `suspected recurrence` with count | Confirm reopening or dismiss with a reason. |
| Confirm store write after reopen fails | Gap is open; suspect remains pending | Retry confirmation to reconcile safely. |
| Detection error | Normal answer and legacy capture continue | Inspect server diagnostics; no user retry is required. |
| Dismiss invalid/missing reason | Stable validation error | Provide a concise operator reason. |

## Failure Modes Registry

| Failure mode | Prevention |
| --- | --- |
| Cross-tenant recurrence access | Every store/query/action requires tenant and has isolation tests. |
| Auto-reopen regression | Only confirm handler calls `UpdateStatus(open)`. |
| Duplicate counts on replay | Durable observation keys and duplicate/reordered audit tests. |
| False match from similar question | Exact normalized question + reason + space only; ambiguity is no-op. |
| Weak event blocks user answer | Server catches detector errors and runs legacy capture. |
| Stale proof creates suspicion | Eligibility requires fresh `verified` projection. |
| Raw content leakage | Recurrence records store IDs/scalars; UI uses text nodes and bounded safe fields. |
| Partial confirm mismatch | Retry reconciliation with already-open gap state. |
| History grows unbounded in UI | Endpoint and UI clamp to 100/20 respectively. |

## Implementation Tasks

1. Add RED tests for recurrence eligibility, exact matching, ambiguity, and
   verification-provenance selection.
2. Add RED tests for recurrence and observation in-memory store tenant scope,
   replay idempotency, pending consolidation, and newest-first ordering.
3. Add RED file-store reopen, atomic write, and concurrent detect tests.
4. Implement recurrence types, stores, validation, and safe projections.
5. Add RED service tests for suspected creation, unrelated/stale/open/ignored
   no-ops, duplicate and reordered audit events, confirmation, dismissal, and
   confirm-retry reconciliation.
6. Implement `RepairRecurrenceService.Detect`, `List`, `Project`, `Confirm`,
   and `Dismiss` without mutating verification attempts.
7. Add RED server tests that demonstrate audit-first detection, duplicate-gap
   suppression when handled, legacy capture fallback on detector error, and
   unchanged normal response delivery.
8. Wire the recurrence service/store in `cmd/server` and `internal/server`.
9. Add RED handler tests for tenant scope, status/limit validation, stable
   errors, confirm/dismiss action guards, and existing routes.
10. Add RED Repair Inbox projection tests, then extend `KnowledgeRepairService`.
11. Add RED static frontend tests, then implement recurrence state/history and
   confirm/dismiss interactions using text-only rendering.
12. Run Stage 4 review, Stage 5 browser QA, Stage 6 security review, and final
   verification after the implementation is complete.

## Explicit Test Plan

Commands:

```powershell
go test ./internal/admin ./internal/server ./web
go test ./...
go vet ./...
node --check web/admin.js
```

Manual smoke after implementation:

1. Create and resolve a gap, then create a passed Phase 23 verification.
2. Produce an exact later weak audit event; confirm one pending recurrence is
   displayed and no duplicate open gap is created.
3. Replay that event and confirm count does not change; produce a distinct
   matching event and confirm the count increases in the same pending record.
4. Dismiss with a reason; confirm gap remains resolved and history persists.
5. Produce another distinct matching event; confirm a new pending record can
   appear.
6. Confirm a pending recurrence; confirm the canonical gap becomes open and
   Phase 23 verification attempts remain intact.
7. Change active reviewed knowledge so proof is stale; produce a weak event and
   confirm no recurrence suspicion is created.
8. Force detector-store failure; confirm answer delivery succeeds and legacy
   gap capture still runs.

## Test Matrix

| ID | Scenario | Expected coverage |
| --- | --- | --- |
| P24-T01 | Resolved verified gap + exact weak audit | One suspected recurrence with verification provenance. |
| P24-T02 | Explicit eligible gap ID | Explicit match wins over fallback. |
| P24-T03 | No explicit ID + exact fallback | One safe fallback match. |
| P24-T04 | Zero or multiple fallback candidates | No recurrence and no guessed match. |
| P24-T05 | Open, investigating, ignored, stale, or unverified gap | No recurrence. |
| P24-T06 | Non-weak audit state | No detector mutation. |
| P24-T07 | Same audit replayed | No duplicate record or count increment. |
| P24-T08 | Reordered replay after a later event | No duplicate increment. |
| P24-T09 | Distinct matching weak events | One pending record with incremented count/latest audit. |
| P24-T10 | File store reopen | Records and observation keys survive restart. |
| P24-T11 | Concurrent matching detection | One pending record and correct observation count. |
| P24-T12 | Different tenant or space | No cross-scope record, lookup, or action. |
| P24-T13 | Dismiss with valid bounded reason | Record dismissed; gap and attempts unchanged. |
| P24-T14 | Dismiss invalid/missing reason | Rejected without mutation. |
| P24-T15 | Confirm pending record | Canonical gap reopens; verification attempts preserved. |
| P24-T16 | Confirm retry after record-write failure | Already-open gap reconciles to confirmed record. |
| P24-T17 | Confirm dismissed/confirmed record | Rejected without gap mutation. |
| P24-T18 | Detector succeeds in answer flow | Audit persists; legacy duplicate gap capture skipped. |
| P24-T19 | Detector failure in answer flow | Answer succeeds and legacy capture remains available. |
| P24-T20 | Recurrence list API validation/limits | 400/503 behavior, tenant scope, and 100 clamp. |
| P24-T21 | Repair Inbox projection | Separate verification and recurrence states render correctly. |
| P24-T22 | Static admin UI | Safe text rendering, bounded history, confirm/dismiss controls. |

## Decision Audit Trail

| # | Review | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Human-confirmed reopen | Auto-decided | Trust boundary | A weak answer is a signal, not an autonomous lifecycle decision. | Auto-reopen. |
| 2 | CEO | Resolved and verified eligibility | Auto-decided | Complete the operator loop | Limits recurrence to repairs that were actually closed with fresh proof. | Any historical gap. |
| 3 | Design | Two compact state lines | Auto-decided | Clarity | Verification freshness and recurrence need different explanations. | One overloaded status enum. |
| 4 | Engineering | Durable observation keys | Auto-decided | Determinism | Handles duplicate and re-ordered audit delivery correctly. | Latest-ID-only dedupe. |
| 5 | Engineering | Error falls through to legacy capture | Auto-decided | Resilience | Monitoring must not interrupt normal answers. | Fail request or silently drop gap capture. |
| 6 | DX | Inline actions with reason on dismiss | Auto-decided | Obvious naming | Operator intent is explicit and the decision remains explainable. | Hidden side-panel workflow. |

## Review Scores

| Review | Score | Verdict |
| --- | --- | --- |
| CEO | 8.5/10 | Build trusted detection before analytics or automation. |
| Design | 8/10 | Keep the decision in the existing Repair Inbox. |
| Engineering | 9/10 | Separate stores and durable dedupe make replay behavior explicit. |
| DX | 8/10 | Direct APIs/actions and bounded safe history. |

## Approval Gate

Recommended approval phrase:

`批准 Phase 24 plan，进入 Stage 3 TDD。`

After approval, Stage 3 must use Superpowers TDD with the matrix above. No eval
promotion, trend aggregation, semantic matching, or automatic reopen code may
enter the Phase 24 build.
