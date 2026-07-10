# Phase 23 Repair Verification Ledger Plan

Date: 2026-07-10

Status: Draft; awaiting Stage 2 approval

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 23 Repair Verification Ledger Spec](../specs/phase-23-repair-verification-ledger.md)

Source design: [Phase 23 Repair Verification Ledger Design](../design/phase-23-repair-verification-ledger.md)

Source roadmap: [Phase 23-26 Repair Verification Roadmap](../design/phase-23-26-repair-verification-roadmap.md)

## Plan Summary

Phase 23 adds a narrow, tenant-scoped, append-only repair verification ledger.
It persists the outcome of a provider-free local verification of an existing
knowledge gap, binds the result to deterministic knowledge snapshot and evidence
fingerprints, and projects `unverified`, `verified`, or `stale` into the
existing Repair Inbox. Gaps remain the only lifecycle source of truth; retest
remains ephemeral; recurrence, eval promotion, and trends remain later phases.

## What Already Exists

- `KnowledgeGapService` owns tenant-scoped gap creation, lookup, and lifecycle
  updates using in-memory and file-backed stores.
- `KnowledgeService` exposes documents with content hashes, review state,
  lifecycle state, indexed chunks, and file-backed atomic persistence.
- `knowledge.Service.Diagnostics` performs provider-free lexical diagnostics
  over tenant and space scoped documents.
- `KnowledgeRepairService` already projects gaps, audit context, documents, and
  current local repair state for `GET /admin/knowledge/repairs`.
- `POST /admin/knowledge/repairs/retest` already returns an ephemeral support
  check and must remain behaviorally unchanged.
- The admin Repair Inbox already renders safe text-only repair rows and reuses
  existing action styles.

## Autoplan Review Summary

### CEO Review: 8.5/10

Build durable proof before analytics or automation. The operator's immediate
pain is not a lack of charts; it is being unable to explain why a gap was
considered repaired and whether that conclusion is still current. A narrow
verify action creates the evidence substrate that Phases 24-26 require.

Auto-decided: do not auto-resolve after a passed verification. Retrieval support
is not a truth or policy decision, and automatic lifecycle mutation would erode
operator trust.

### Design Review: 8/10

Keep verification in the existing Repair Inbox. A concise state chip, last
verification time, and an inline bounded history affordance give the operator
the necessary answer without another dashboard. `stale` must read as "knowledge
changed; recheck" rather than a red failure state.

Auto-decided: history uses safe summaries and short fingerprint prefixes only;
no raw snippets or full hashes appear in the row.

### Engineering Review: 9/10

Use a dedicated `RepairVerificationStore` and `RepairVerificationService`.
Reusing audit storage would conflate answer events with operator verification.
The service builds canonical sorted payloads, hashes them with SHA-256, runs
the existing diagnostics, and appends a complete attempt only after required
inputs are available. A diagnostics error after snapshot creation persists one
bounded `retrieval_error` attempt; snapshot or evidence fingerprint failure
writes no record.

Auto-decided: snapshot fingerprint input is exactly active, review-active
documents in the selected space, sorted by document ID and represented by ID,
content hash, UTC updated timestamp, lifecycle/review status, and index state.

### DX Review: 8/10

Use obvious names and additive contracts: `RepairVerificationAttempt`,
`RepairVerificationService.Verify`, `POST /admin/knowledge/repairs/verify`,
and `GET /admin/knowledge/repairs/verifications`. The history limit defaults to
20 and clamps at 100. Failed verification responses contain a stable reason and
next action; they never expose internal provider errors.

Auto-decided: `Run retest` remains the zero-persistence troubleshooting action;
`Verify repair` is the explicit persistence action.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | Lifecycle owner | `KnowledgeGap` only | Avoids two state machines and permits later recurrence review. |
| 2 | Verification history | New append-only store | Audit and verification records have different meaning and retention shape. |
| 3 | Snapshot versioning | Canonical SHA-256 fingerprint | Avoids mutable global coordination state. |
| 4 | Evidence fingerprint | Canonical safe source identity and rank | Allows deterministic comparison without raw text. |
| 5 | Passed attempt | Grounded + active reviewed source + both fingerprints | Makes support claim bounded and inspectable. |
| 6 | Diagnostics error | Persist failed `retrieval_error` after snapshot exists | Records the operator action without partial data. |
| 7 | Snapshot failure | Return error, write nothing | Preserves append-only ledger integrity. |
| 8 | Stale semantics | Changed snapshot means `stale` | Prior proof remains historical rather than becoming failed. |
| 9 | Existing retest | Keep ephemeral and unchanged | Maintains Phase 22 compatibility and a fast troubleshooting path. |
| 10 | UI placement | Existing Repair Inbox row and bounded history | Preserves operator flow and avoids dashboard sprawl. |
| 11 | Phase boundary | No recurrence/eval/trends | Keeps Phase 23 independently shippable. |

## Architecture

```text
cmd/server
  -> creates FileRepairVerificationStore in admin data directory
  -> wires RepairVerificationService into server config

internal/admin
  KnowledgeGapService ----------- owns gap lifecycle
  KnowledgeService -------------- lists documents/source state
  RepairVerificationStore ------- persists immutable attempts
  RepairVerificationService ----- snapshots, fingerprints, verify/list/project
  KnowledgeRepairService -------- attaches current verification projection

internal/server
  POST /admin/knowledge/repairs/verify
  GET  /admin/knowledge/repairs/verifications
  GET  /admin/knowledge/repairs (adds verification fields)

web/admin
  Repair Inbox row -> state chip, timestamp, Verify repair, bounded history
  Existing Run retest -> remains unchanged
```

## Data Model

Add in `internal/admin`:

```go
type RepairVerificationResult string
const (
    RepairVerificationPassed RepairVerificationResult = "passed"
    RepairVerificationFailed RepairVerificationResult = "failed"
)

type RepairVerificationFailureReason string

// Safe metadata only; never reuse answer-audit source types that carry snippets.
type RepairVerificationSource struct {
    DocumentID   string
    Title        string
    ReviewStatus string
    SourceType   string
}

type RepairVerificationAttempt struct {
    ID                           string
    TenantID                     string
    GapID                        string
    SpaceID                      string
    StartedAt                    time.Time
    CompletedAt                  time.Time
    BeforeState                  string
    AfterState                   string
    KnowledgeSnapshotFingerprint string
    EvidenceFingerprint          string
    SourceCount                  int
    TopSources                   []RepairVerificationSource
    Result                       RepairVerificationResult
    FailureReason                RepairVerificationFailureReason
}

type RepairVerificationState string
// unverified | verified | stale
```

Implement this exact safe-source boundary (with JSON tags, validation, tenant
scoping, and append-only behavior). `AnswerAuditTimelineSource` is not usable
for ledger records because it can contain diagnostic snippets.

## Canonical Fingerprint Rules

### Snapshot

1. List documents for the request tenant and selected gap space.
2. Keep only active lifecycle and `active` review documents.
3. Normalize each entry into the exact field tuple:
   document ID, content hash, updated timestamp in UTC RFC3339Nano, document
   status, review status, lexical-ready state, and vector status.
4. Sort by document ID.
5. Marshal a versioned canonical payload such as `repair_snapshot_v1`.
6. SHA-256 the bytes and encode as `sha256:<hex>`.

### Evidence

1. Use up to the diagnostics result limit already returned by local retrieval.
2. Join each diagnostics result to the already tenant-and-space-scoped document
   snapshot by document ID. Normalize document ID, chunk ID or stable source
   identity, the joined document content hash (or the canonical empty string
   when the document is absent), retrieval rank, and review status.
3. Sort by rank then stable identity.
4. Marshal a versioned `repair_evidence_v1` payload and SHA-256 it.
5. Persist safe source summaries separately; never hash or store raw snippets.
   The safe summary type is limited to document ID, title, review status, and
   source type; it has no snippet field.

## API Plan

### `POST /admin/knowledge/repairs/verify`

Input: `{ "gap_id": "gap-..." }`.

Behavior:

1. Require gap, verification service, knowledge service, and retriever wiring.
2. Validate safe gap ID and load the tenant-scoped gap.
3. Capture current snapshot fingerprint.
4. Resolve current before state from the repair projection where available.
5. Run lexical diagnostics with the original gap question and space, limit 3.
6. Build evidence fingerprint for diagnostics results or use an empty canonical
   evidence set for safe failed states.
7. Append exactly one complete attempt.
8. Return attempt plus current `unverified`/`verified`/`stale` projection.

Status handling:

- bad JSON, unsafe ID, or missing gap: 400;
- missing required service: 503;
- snapshot/evidence canonicalization failure before append: 500, no record;
- diagnostics error after a snapshot: 200 with persisted `retrieval_error`
  attempt and no raw cause;
- successful or expected failed verification: 200.

### `GET /admin/knowledge/repairs/verifications`

Input: `gap_id` required; `limit` optional, default 20, clamped 100.

Returns newest-first tenant-scoped attempts. Invalid IDs or limits return 400;
missing verification service returns 503.

### Repair List Extension

Add only:

- `verification_state`;
- `last_verified_at` when a passed attempt exists;
- `last_verification_result`;
- optional bounded last failure reason.

Existing fields, filters, sort semantics, and retest response remain stable.

## UI Plan

1. Extend static tests first for selectors, paths, state rendering, verify action,
   and history rendering.
2. Add verification endpoint constants and minimal row rendering helpers.
3. Render a compact state line:
   `verification: verified | 2026-07-10T...`;
   `stale: knowledge changed; verify again`;
   `unverified: no passed verification`.
4. Add `Verify repair` alongside existing `Run retest`.
5. Add an inline `View history` action that fetches at most 20 attempts and
   renders timestamp, result, safe reason, state transition, source count, and
   short fingerprint prefixes via `textContent`.
6. Refresh the repair list after verify; preserve existing action behavior.

## Failure And Rescue Registry

| Failure | User-visible result | Rescue action |
| --- | --- | --- |
| No active reviewed source | Failed verification with `no_active_source` | Create or activate reviewed evidence, then verify again. |
| Review-gated source | Failed verification with `review_gated` | Review or activate source, then verify again. |
| Unsupported result | Failed verification with `unsupported` | Run diagnostics or add relevant evidence. |
| Retrieval error after snapshot | Recorded `retrieval_error` without raw cause | Retry later; inspect diagnostics separately. |
| Snapshot cannot be built | No record written; stable server error | Repair source metadata/index state, then retry. |
| Passed proof becomes stale | `stale` state, history preserved | Verify against current knowledge. |

## Failure Modes Registry

| Failure mode | Prevention |
| --- | --- |
| Cross-tenant attempt access | Store/list/get methods require tenant and tests cover isolation. |
| Gap lifecycle drift | Verification service has no gap update dependency. |
| Nondeterministic fingerprint | Explicit canonical structs, ordering, UTC formatting, and golden tests. |
| Gated content leakage | Fingerprints use metadata only; UI/API use safe source summaries. |
| Partial append | Build snapshot/evidence before store write; atomic file replacement. |
| History growth degrades UI | Server limit defaults/clamps and UI renders bounded rows. |
| Old retest behavior changes | Preserve endpoint and add regression tests before wiring verify. |
| Future Phase 24 leaks in | No recurrence record, matcher, or reopen endpoint in this diff. |

## Implementation Tasks

1. Add RED tests for canonical snapshot/evidence fingerprint helpers.
2. Add RED tests for in-memory verification store append/list and tenant scoping.
3. Add RED file-store persistence/reopen and append-order tests.
4. Implement verification types, validation, stores, and canonical fingerprints.
5. Add RED service tests for passed, unsupported, review-gated, no-active-source,
   retrieval-error, and atomic failure cases.
6. Implement `RepairVerificationService.Verify`, `List`, and current-state
   projection without gap mutation.
7. Add RED tests for Repair Inbox verification projection, including stale after
   active reviewed document mutation.
8. Extend `KnowledgeRepairService` with additive verification fields.
9. Add RED handler tests for verify/history validation, tenant scope, limits,
   and unchanged retest behavior.
10. Wire stores and services in `cmd/server` and `internal/server`.
11. Add RED static frontend tests, then render verify state/action/history.
12. Run full verification and Stage 4 review after implementation.

## Explicit Test Plan

Commands:

```powershell
go test ./internal/admin ./internal/server ./web
go test ./...
go vet ./...
```

Manual smoke after implementation:

1. Start a local server with isolated admin/runtime data.
2. Create a gap and active reviewed note for its original question.
3. Verify the repair; confirm a passed record and `verified` inbox state.
4. Restart the server; confirm history persists.
5. Change, disable, or remove the active reviewed evidence; confirm `stale`.
6. Run old `Run retest`; confirm it returns a result without adding history.
7. Attempt a verify with no active source or review-gated source; confirm a safe
   failed attempt and unchanged gap status.

## Test Matrix

| ID | Scenario | Expected coverage |
| --- | --- | --- |
| P23-T01 | Same active reviewed docs in different storage order | Equal snapshot fingerprint. |
| P23-T02 | Content hash, review, lifecycle, or index metadata changes | Snapshot fingerprint changes. |
| P23-T03 | Same diagnostics source sequence | Equal evidence fingerprint. |
| P23-T04 | Grounded active reviewed source | Passed attempt persists with both fingerprints. |
| P23-T05 | No active reviewed source | Failed `no_active_source`; gap unchanged. |
| P23-T06 | Review-gated source | Failed `review_gated`; no raw gated text. |
| P23-T07 | Unsupported diagnostics | Failed `unsupported`; gap unchanged. |
| P23-T08 | Diagnostics error after snapshot | One safe `retrieval_error` attempt persists. |
| P23-T09 | Snapshot/fingerprint error | No attempt persists. |
| P23-T10 | File store reopen | Attempts survive and remain newest first. |
| P23-T11 | Different tenant or space | Attempts/documents never cross scope. |
| P23-T12 | Current passed attempt matches snapshot | Repair state is `verified`. |
| P23-T13 | Active reviewed document changes after pass | Repair state becomes `stale`; history unchanged. |
| P23-T14 | Only failed attempts exist | Repair state remains `unverified`. |
| P23-T15 | Verify API invalid/missing gap ID | 400 with no record. |
| P23-T16 | Verify/history services unavailable | 503 stable error code. |
| P23-T17 | History limit invalid/large | 400 or clamp at 100. |
| P23-T18 | Existing retest route | Remains ephemeral and provider free. |
| P23-T19 | Admin static UI | Selectors, paths, verify action, state, and history use safe rendering. |
| P23-T20 | Long history/source labels | UI stays bounded and scan-friendly. |

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Verify before analytics | Auto-decided | Complete the operator loop | Durable proof is the immediate user value and powers later phases. | Trends first. |
| 2 | CEO | No automatic resolve | Auto-decided | Trust boundary | Retrieval support is not an autonomous lifecycle decision. | Auto-resolve on pass. |
| 3 | Design | Inline inbox history | Auto-decided | Subtraction | The operator already works from the repair row. | New dashboard. |
| 4 | Engineering | Dedicated append-only store | Auto-decided | Single ownership | Audit events and verification facts have different semantics. | Reuse audit store. |
| 5 | Engineering | Canonical SHA-256 snapshots | Auto-decided | Determinism | Existing document metadata supports stable proof without a counter. | Global version counter. |
| 6 | Engineering | Persist retrieval errors after snapshot | Auto-decided | Honest history | Operator action should be visible if safe snapshot context exists. | Drop all failed actions. |
| 7 | DX | Separate retest and verify actions | Auto-decided | Obvious naming | Persistence must be intentional and troubleshooting stays fast. | Implicit persistence on retest. |

## Review Scores

| Review | Score | Verdict |
| --- | --- | --- |
| CEO | 8.5/10 | Build the proof substrate before broader operations. |
| Design | 8/10 | Keep verification inside the existing inbox. |
| Engineering | 9/10 | Separate ledger, canonical fingerprints, no lifecycle mutation. |
| DX | 8/10 | Explicit additive APIs and actions; bounded history. |

## Approval Gate

Recommended approval phrase:

`批准 Phase 23 plan，进入 Stage 3 TDD。`

After approval, Stage 3 must use Superpowers TDD with the matrix above. No
recurrence, eval-promotion, or trend code may enter the Phase 23 build.
