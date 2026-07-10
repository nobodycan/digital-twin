# Phase 23 Repair Verification Ledger Spec

Date: 2026-07-10

Status: Draft; awaiting Stage 1 approval

Source roadmap: [Phase 23-26 Repair Verification Roadmap](../design/phase-23-26-repair-verification-roadmap.md)

Prior phase: [Phase 22 Knowledge Repair Inbox Spec](./phase-22-knowledge-repair-inbox.md)

## Problem Statement

Phase 22 lets an operator retest a knowledge gap against current local reviewed
knowledge, but that result is ephemeral. The operator cannot later answer:

- which repair was tested;
- which knowledge snapshot supported it;
- which bounded evidence was returned;
- whether a later knowledge edit made the proof obsolete.

This leaves resolved gaps with an informal human claim rather than durable,
deterministic repair evidence.

## Target User And Wedge

The target user is the solo knowledge operator who has created or linked
reviewed evidence for a repair gap and needs a lightweight way to record that
the original question is now locally supported.

The narrow wedge is one action in the existing Repair Inbox: `Verify repair`.
It runs the existing provider-free local retrieval path and appends an immutable
verification attempt. The inbox then displays the derived current state:
`unverified`, `verified`, or `stale`.

## Demand And Status Quo

The existing status quo is a gap lifecycle plus an ephemeral retest response.
An operator can resolve a gap after seeing a source, but repeated clicks cannot
distinguish a new observation from the original one, and later source changes
silently invalidate the operator's prior conclusion.

Phase 23 makes the evidence trail explicit without turning the local-first
admin console into a compliance system or multi-user workflow.

## Goals

1. Persist tenant-scoped, append-only repair verification attempts.
2. Prove that the original gap question is grounded by the current active
   reviewed knowledge snapshot, not that an answer is objectively true.
3. Compute deterministic knowledge snapshot and retrieval evidence fingerprints.
4. Derive a current verification state without copying the `KnowledgeGap`
   lifecycle.
5. Keep existing retest behavior ephemeral, provider-free, and compatible.
6. Give the existing Repair Inbox a compact verification state, last verified
   time, verify action, and bounded history entry point.

## Non-Goals

- Automatic gap resolution, reopening, or recurrence detection.
- Eval case creation, release gating, trend analytics, or retention policy.
- A generic event ledger, scheduler, queue, or database migration.
- LLM-as-judge, semantic correctness scoring, or numeric confidence scores.
- Cross-tenant analytics, RBAC, assignments, notifications, or comments.
- New document review states, source crawling, or changed retrieval scoring.
- Storage of raw review-gated text.

## Existing Building Blocks

- `KnowledgeGap` already owns `open`, `investigating`, `resolved`, and
  `ignored` lifecycle state.
- `KnowledgeRepairService` projects gaps, audit context, linked documents, and
  ephemeral retest support from existing services.
- `KnowledgeService` exposes tenant- and space-scoped documents, active review
  state, content hashes, chunks, index metadata, and file-backed persistence.
- `knowledge.Service.Diagnostics` provides provider-free lexical retrieval with
  bounded source summaries and review-gate diagnostics.
- Existing admin file stores demonstrate in-memory and file-backed test parity.

## Premises

1. A deterministic verification record is more useful than repeated manual
   retests because it binds the result to a concrete knowledge snapshot.
2. A stable fingerprint is sufficient for local evidence provenance; a mutable
   global version counter would add coordination state without improving proof.
3. The gap remains the canonical lifecycle record. The ledger records facts
   about verification attempts and must never own resolve/reopen transitions.
4. A changed active-reviewed knowledge snapshot makes a prior passed attempt
   `stale`, not failed, because history must remain truthful.
5. Explicit and exact source identity is safer than semantic similarity in a
   first verification slice.

## Approaches Considered

### A. Extend Existing Audit Records

Store verification fields in `AuditRecord` and expose them through the answer
timeline.

Rejected because audit records describe answer events, while verification is an
operator action over a repair gap and a named knowledge snapshot. Combining
them would blur retention, ordering, and Phase 24 recurrence semantics.

### B. Dedicated Append-Only Verification Ledger

Add a focused verification store and service in `internal/admin`, then project
current state into the existing Repair Inbox.

Selected because it keeps gap lifecycle canonical, preserves immutable evidence
history, matches existing local file-store patterns, and creates a clean input
for later recurrence and eval phases.

### C. Generic Knowledge Event Ledger

Introduce one broad event model for verification, recurrence, evals, and trends.

Rejected because it would pre-build Phases 24-26, widen migration risk, and
leave the operator with abstractions before a useful verify action exists.

## Product Contract

### Verification Attempt

Each attempt is append-only and contains at minimum:

```json
{
  "attempt_id": "verify-gap-1-...",
  "tenant_id": "default",
  "gap_id": "gap-...",
  "space_id": "default",
  "started_at": "2026-07-10T00:00:00Z",
  "completed_at": "2026-07-10T00:00:00Z",
  "before_state": "unsupported",
  "after_state": "grounded",
  "knowledge_snapshot_fingerprint": "sha256:...",
  "evidence_fingerprint": "sha256:...",
  "source_count": 1,
  "top_sources": [
    {
      "document_id": "kb-startup",
      "title": "Startup Playbook",
      "review_status": "active",
      "source_type": "workbench_note"
    }
  ],
  "result": "passed"
}
```

Failed attempts use `result: "failed"` and one bounded reason:

- `unsupported`;
- `review_gated`;
- `no_active_source`;
- `retrieval_error`.

Raw document, chunk, or review-gated text is never persisted by this ledger.

### Snapshot Fingerprint

For the selected tenant and space, use only active, review-active documents.
Build canonical entries from stable fields:

- document ID;
- content hash;
- document update timestamp in UTC;
- lifecycle status;
- review status;
- index-ready or index-error metadata.

Sort entries by document ID, serialize with an explicit schema version, then
hash the canonical bytes using SHA-256. Equivalent inputs must yield equivalent
fingerprints regardless of storage order.

### Evidence Fingerprint

Build canonical entries from the bounded diagnostics results:

- document ID;
- chunk ID or stable chunk position if available;
- source content hash when available;
- retrieval rank;
- review status.

Sort by rank then stable identity, serialize with an explicit schema version,
and hash with SHA-256. Do not include raw snippets.

### Current Verification State

The state is derived per repair item:

- `unverified`: no passed verification attempt exists;
- `verified`: latest passed attempt has the current snapshot fingerprint;
- `stale`: latest passed attempt differs from the current snapshot fingerprint.

Failed attempts are historical evidence but do not by themselves supersede a
prior passing attempt. Phase 24 may add recurrence states later; Phase 23 must
not reserve or implement them in production behavior.

## API Contract

Keep the existing endpoint unchanged:

- `POST /admin/knowledge/repairs/retest` remains ephemeral and mutation-free.

Additive Phase 23 endpoints:

- `POST /admin/knowledge/repairs/verify`
- `GET /admin/knowledge/repairs/verifications?gap_id=<id>&limit=<n>`

Verify request:

```json
{"gap_id":"gap-..."}
```

The verify response returns the persisted attempt plus the current derived state.
The history endpoint requires a safe gap ID, defaults to a small bounded limit,
clamps at 100, and returns attempts newest first.

All endpoints use the existing admin tenant, gap ID validation, provider-free
local diagnostics, and safe source-summary projection rules.

## UI Contract

Add to each Repair Inbox row:

- a verification chip: `unverified`, `verified`, or `stale`;
- last verified timestamp when present;
- a `Verify repair` action;
- a compact expandable or inline bounded history view.

The existing `Run retest` action remains available and does not write history.
The UI must use text nodes, preserve existing repair/gap actions, and keep long
values clamped. There is no new full-page dashboard in Phase 23.

## Failure Modes

| Failure mode | Required behavior |
| --- | --- |
| Gap is missing or tenant scoped elsewhere | Reject without creating an attempt. |
| Retrieval finds no active reviewed source | Persist a failed attempt with `no_active_source` or `unsupported`; do not mutate gap. |
| Retrieval is review gated | Persist a failed `review_gated` attempt with no gated text. |
| Diagnostics returns an error | If snapshot capture completed, persist one failed `retrieval_error` attempt with `after_state: unknown` and safe error code, then return that record. If snapshot capture fails, fail atomically with no record. |
| Fingerprint construction fails | Return an error and write no attempt. |
| Knowledge changes after verification | Project `stale`; retain the historical attempt unchanged. |
| File store reopens | Retain attempt order and fields. |
| Large history request | Clamp to 100 and return stable newest-first ordering. |

## Success Criteria

1. An operator can create a passed or failed verification record from an
   existing gap without using an external provider.
2. The same active-reviewed knowledge produces the same snapshot fingerprint.
3. The same diagnostics result produces the same evidence fingerprint.
4. A changed active-reviewed document causes a prior passed proof to become
   stale without changing the gap lifecycle.
5. Existing repair list and retest behavior remain compatible.
6. No tenant, gated text, or unbounded history leak crosses the admin API or UI.
7. Unit, server, persistence, and static UI tests cover the acceptance cases.

## Stage 2 Inputs

The Phase 23 plan must lock:

- exact Go types, store file shape, and migration behavior;
- whether failed diagnostics errors are persisted or only returned when no
  snapshot can be captured;
- canonical serialization schema and fingerprint field set;
- current verification projection ownership and list-query shape;
- exact frontend history interaction;
- RED test matrix, including deterministic clock and file reopen coverage.

## Approval Gate

Approve this Phase 23 spec to enter Stage 2 `$gstack-autoplan`. No production
code may be written until the resulting plan is explicitly approved.
