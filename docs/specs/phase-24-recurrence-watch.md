# Phase 24 Recurrence Watch Spec

Date: 2026-07-10

Status: Approved; Stage 1 gate passed

Source roadmap: [Phase 23-26 Repair Verification Roadmap](../design/phase-23-26-repair-verification-roadmap.md)

Prior phase: [Phase 23 Repair Verification Ledger Spec](./phase-23-repair-verification-ledger.md)

## Problem Statement

Phase 23 records deterministic proof that a repaired gap is supported by the
current active-reviewed knowledge snapshot. It does not notice when a later
answer for the same problem becomes weak again. Without a bounded recurrence
watch, an operator must manually compare answer history with verification
history, and an old resolved gap can silently coexist with a new open duplicate.

## Target User And Wedge

The target user is the local knowledge operator who has resolved and verified a
repair and needs to know when a new weak answer may invalidate that operational
conclusion.

The narrow wedge is an inline `suspected recurrence` signal in the existing
Repair Inbox. The operator can inspect the matched weak-answer audit evidence,
then explicitly confirm reopening or dismiss the suspicion. The system never
reopens a gap by itself.

## Goals

1. Detect a later weak answer that exactly matches a currently verified,
   resolved repair gap.
2. Persist a tenant-scoped recurrence record that references safe audit evidence
   and the matched verification context.
3. Deduplicate repeated processing of the same audit event and consolidate
   repeated weak events into one pending suspicion per repair gap.
4. Let an operator confirm, dismiss, or leave a suspected recurrence pending.
5. Reopen the canonical `KnowledgeGap` only after explicit confirmation.
6. Preserve Phase 23 verification history and verification-state semantics.
7. Keep recurrence detection out of the answer-response critical path.

## Non-Goals

- Semantic similarity, embeddings, fuzzy matching, LLM judgment, or confidence
  scores.
- Automatic gap reopening, automatic resolution, notifications, scheduling, or
  multi-user assignment.
- Replacing `KnowledgeGap` lifecycle ownership or adding recurrence states to
  `RepairVerificationState`.
- Eval promotion, release gating, trend dashboards, or cross-tenant reporting.
- Persisting raw user messages, answer text, citations, or review-gated
  snippets in recurrence records.
- Changing retrieval scoring, document review rules, or the provider-free
  diagnostics contract.

## Existing Building Blocks

- `AuditService` persists tenant-scoped `AuditRecord` values with bounded
  question summaries, answer state, space, no-source reason, and evidence
  metadata.
- `KnowledgeGapService` owns the canonical `open`, `investigating`, `resolved`,
  and `ignored` lifecycle and can reopen a gap through `UpdateStatus`.
- `RepairVerificationService` exposes current `verified`, `stale`, and
  `unverified` projections from immutable verification attempts and the current
  knowledge snapshot.
- `KnowledgeRepairService` already combines gaps, audits, documents, and
  verification state into the Repair Inbox.
- Server answer handling records an audit event before it runs the existing gap
  capture path.

## Product Decisions

### Eligibility

A repair is recurrence-eligible only when all conditions are true:

1. Its canonical gap is `resolved`.
2. Its current Phase 23 verification projection is `verified`, not `stale` or
   `unverified`.
3. The new audit event has a weak answer state: `unsupported`,
   `partially_supported`, or `review_gated`.
4. The audit event identifies the same tenant and space.

An open or investigating gap is already actionable and never receives a
recurrence suspicion. An ignored gap is not treated as a repaired outcome.

### Exact Matching Only

Phase 24 uses this deterministic match order:

1. Explicit gap ID from the audit evidence, when present and recurrence-eligible.
2. Otherwise, exactly one eligible resolved gap matching all of:
   `space_id`, normalized bounded question summary, and `no_source_reason`.

No match is created for zero matches or more than one exact fallback match.
The system does not choose among ambiguous candidates.

### Independent Projection Axes

Verification freshness and recurrence are separate facts. Phase 24 must keep
the Phase 23 verification state as `verified`, `stale`, or `unverified`, and
add a separate recurrence projection:

- `none`: no pending suspicion;
- `suspected`: at least one pending recurrence record;
- `dismissed`: only historical dismissed records exist;
- `confirmed`: most recent recurrence was confirmed.

The Repair Inbox may emphasize `suspected` visually, but must still show the
underlying verification state and last verified time.

## Data Contract

Add a tenant-scoped `RepairRecurrence` record. It is durable and has a small,
auditable lifecycle:

```json
{
  "id": "recurrence-...",
  "tenant_id": "default",
  "gap_id": "gap-...",
  "space_id": "default",
  "status": "suspected",
  "matched_by": "explicit_gap_id",
  "first_audit_id": "audit-...",
  "latest_audit_id": "audit-...",
  "occurrence_count": 2,
  "answer_state": "unsupported",
  "no_source_reason": "no_matching_chunks",
  "verified_attempt_id": "verification-...",
  "verified_snapshot_fingerprint": "sha256:...",
  "created_at": "2026-07-10T00:00:00Z",
  "updated_at": "2026-07-10T00:00:00Z",
  "dismissed_at": null,
  "dismiss_reason": "",
  "confirmed_at": null,
  "confirmed_by": ""
}
```

Allowed statuses are `suspected`, `dismissed`, and `confirmed`.

`first_audit_id` and `latest_audit_id` are stable audit references only.
Question content, assistant text, source snippets, and raw diagnostic errors are
not copied into the record. The UI resolves safe bounded summaries from the
existing audit timeline when needed.

## Detection Contract

Detection runs best-effort after a weak-answer audit record is successfully
written and before the legacy automatic gap-capture step creates a new gap.

1. Read the just-recorded audit event using its tenant-scoped ID.
2. Exit without mutation when the event is not recurrence-eligible.
3. Find an eligible resolved repair using the exact match order above.
4. If a record already links the same `gap_id` and audit ID, do nothing.
5. If a pending `suspected` record exists for the same gap, update only its
   latest audit reference, occurrence count, answer state/reason, and timestamp.
6. Otherwise create one new `suspected` record.
7. If a suspicion was created or consolidated, skip the legacy gap-create path
   for this event so a duplicate open gap is not created.

If detection is unavailable or fails, it must not delay or fail the normal
answer response. The server retains existing gap-capture behavior as the safe
fallback, which may create a new open gap but never reopens the original one.

## Operator Actions

### Confirm Recurrence

`Confirm recurrence` is valid only for a tenant-scoped `suspected` record.
The service reopens the canonical gap with `KnowledgeGapStatusOpen`, clears old
resolution fields through the existing lifecycle method, then records the
recurrence as `confirmed` with timestamp and bounded actor label.

If persistence fails after the gap is reopened, the recurrence remains pending
and confirmation is retry-safe: a subsequent confirmation reconciles the record
with the already-open gap. Verification attempts are never edited or deleted.

### Dismiss Recurrence

`Dismiss` is valid only for a tenant-scoped `suspected` record. It changes that
record to `dismissed` and stores a required, bounded operator reason and time.
It never changes the gap lifecycle or verification attempts. A later distinct
weak audit event may create a new suspicion.

## API Contract

Additive admin endpoints:

- `GET /admin/knowledge/repairs/recurrences?gap_id=<id>&status=<status>&limit=<n>`
- `POST /admin/knowledge/repairs/recurrences/confirm`
- `POST /admin/knowledge/repairs/recurrences/dismiss`

List input is tenant-scoped, requires a safe `gap_id`, defaults to 20, and
clamps at 100. It returns newest-first recurrence records with safe fields.

Confirm input:

```json
{"recurrence_id":"recurrence-...","confirmed_by":"operator"}
```

Dismiss input:

```json
{"recurrence_id":"recurrence-...","reason":"Known transient provider outage"}
```

Invalid IDs, malformed JSON, invalid statuses, and missing required dismissal
reasons return `400`. Missing services return `503`. Tenant-scoped missing
records return a stable `400` or `404` error without leaking another tenant's
existence. Server errors do not include raw diagnostic causes.

## UI Contract

Extend each Repair Inbox row with:

- a compact recurrence line showing `suspected`, `dismissed`, `confirmed`, or
  no active suspicion;
- a prominent, text-only `suspected recurrence` signal when pending;
- bounded recurrence history, showing safe status, timestamps, occurrence count,
  weak answer state, no-source reason, and linked audit ID;
- `Confirm recurrence` and `Dismiss` actions only for pending records;
- a required dismissal-reason input with a bounded length and clear validation.

All recurrence text must be rendered via `textContent`. The UI must retain the
existing verify, retest, history, resolve, ignore, and reopen actions.

## Failure Modes

| Failure mode | Required behavior |
| --- | --- |
| Weak event has no eligible verified resolved repair | Do not create a recurrence record; retain existing gap capture behavior. |
| Exact fallback produces multiple candidates | Do not guess or create a record. |
| Same audit is processed twice | Do not increment count or create a duplicate. |
| Repeated weak audits match one pending repair | Consolidate into the pending record and increment count. |
| Detection store or matcher fails | Do not block answer generation; retain legacy gap capture fallback. |
| Confirm request targets dismissed/confirmed record | Reject without changing the gap. |
| Dismiss request lacks a reason | Reject without changing the record. |
| Confirm partially persists | Keep the original verification ledger untouched; make retry reconcile an already-open gap with the pending record. |
| Tenant or space differs | No match, projection, or API record crosses scope. |
| Current verification becomes stale | Do not create new recurrence suspicions from it. |

## Success Criteria

1. A weak answer exactly matching a verified resolved repair produces one
   tenant-scoped `suspected` recurrence record.
2. Unrelated, stale, open, ignored, unverified, or ambiguous candidates produce
   no recurrence record.
3. Duplicate processing is idempotent and repeated weak events consolidate into
   one pending record with an accurate occurrence count.
4. Confirmation reopens only the canonical gap and preserves its verification
   attempts; dismissal preserves the gap lifecycle and a required reason.
5. Detection errors never block answer generation or audit recording.
6. Existing retest, verification, gap lifecycle, and audit timeline behavior
   remain compatible outside the new recurrence projection.
7. API/UI responses expose only bounded safe metadata and preserve tenant and
   space isolation.

## Stage 2 Inputs

The Phase 24 plan must lock:

- exact Go types, file-store shape, locking and atomic-write semantics;
- the normal-answer hook order and an explicit `handled` contract with legacy
  gap capture;
- eligibility query strategy and whether an audit event carries an explicit gap
  ID at persistence time;
- exact idempotency key and repeated-event consolidation rules;
- confirmation reconciliation behavior without a cross-store transaction;
- current Repair Inbox sorting/filter behavior when a suspicion is pending;
- RED test matrix across service, server, file persistence, frontend, and
  normal-answer nonblocking behavior.

## Approval Gate

Approve this Phase 24 spec to enter Stage 2 `$gstack-autoplan`. No production
code may be written until the resulting plan is explicitly approved.
