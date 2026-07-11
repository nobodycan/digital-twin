# Phase 24 Recurrence Watch Design

Date: 2026-07-10

Status: Approved; Stage 1 spec accepted

Source spec: [Phase 24 Recurrence Watch Spec](../specs/phase-24-recurrence-watch.md)

## Design Summary

Phase 24 adds a narrow human-confirmed recurrence loop on top of the Phase 23
verification ledger. A later weak answer can create a `suspected` record only
for a currently verified, resolved gap identified by an explicit ID or exact
fallback key. The record is separate from verification freshness and gap
lifecycle state.

## Data Flow

```text
answer result
  -> audit record (already durable)
  -> recurrence detector (best effort)
       -> exact eligible verified-resolved gap match
       -> create/consolidate suspected recurrence
       -> handled: skip legacy new-gap creation
  -> legacy gap capture when not handled or detector fails

Repair Inbox
  -> verification projection: verified | stale | unverified
  -> recurrence projection: none | suspected | dismissed | confirmed
  -> operator confirms or dismisses a suspected recurrence
```

## Ownership

| Concern | Owner |
| --- | --- |
| Gap lifecycle | Existing `KnowledgeGapService` |
| Immutable proof attempts | Existing `RepairVerificationService` |
| Recurrence suspicion lifecycle | New `RepairRecurrenceService` |
| Weak-answer evidence | Existing `AuditService` |
| UI projection | Existing `KnowledgeRepairService` |

## Guardrails

- Exact matching only; ambiguity is a safe no-op.
- The detector runs after audit persistence and must never affect the response
  success path.
- Only resolved and currently verified gaps qualify.
- `RepairVerificationState` does not grow recurrence values.
- The recurrence record stores audit IDs and safe scalar evidence only.
- Confirmation uses the existing gap lifecycle transition; no automatic reopen.
- All new lists are tenant scoped and bounded.

## Deferred Work

Phase 25 may promote only a verified repair with no pending recurrence into an
eval case. Phase 26 may aggregate confirmed/dismissed recurrence history. This
phase does not add either capability.
