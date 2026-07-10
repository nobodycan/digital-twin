# Phase 23 Repair Verification Ledger Design

Date: 2026-07-10

Status: Draft; Stage 1 design for approval

Source spec: [Phase 23 Repair Verification Ledger Spec](../specs/phase-23-repair-verification-ledger.md)

Source roadmap: [Phase 23-26 Repair Verification Roadmap](./phase-23-26-repair-verification-roadmap.md)

## Design Summary

Phase 23 turns the existing ephemeral local retest into a durable, explainable
verification record. It adds one narrow persistence boundary: an append-only
ledger of verification attempts. The existing gap record remains canonical for
repair lifecycle, while the Repair Inbox derives current proof state from the
latest passed attempt and the current knowledge snapshot.

## Service Boundaries

```text
KnowledgeGapService             owns gap lifecycle
KnowledgeService                owns documents and reviewed source state
knowledge.Service.Diagnostics   owns provider-free retrieval diagnostics
RepairVerificationService       owns attempts, fingerprints, and current proof projection
KnowledgeRepairService          composes gap, audit, document, and verification projections
```

`RepairVerificationService` must accept explicit service dependencies rather
than reading server globals. This keeps fingerprints, persistence, and tests
deterministic.

## Persistence Shape

Use a dedicated store interface and matching in-memory/file implementations:

```text
SaveRepairVerificationAttempt(attempt)
ListRepairVerificationAttempts(tenantID, gapID, limit)
```

The file store may use one JSON array or envelope under the existing admin data
directory. It must preserve append order, write atomically using the repository
file-store pattern, and tolerate an absent file as an empty ledger. There is no
backfill or migration from audit records because audit rows are not verification
attempts.

## Verification Flow

1. Validate tenant-scoped `gap_id` and load the gap.
2. List active-reviewed documents for the gap space.
3. Build a canonical knowledge snapshot fingerprint before writing anything.
4. Run existing lexical local diagnostics with the original gap question.
5. Build a canonical evidence fingerprint from bounded diagnostics results.
6. Derive passed/failed result and safe failure reason.
7. Append one complete attempt.
8. Recompute the current verification projection and return it.

The server must never auto-resolve or auto-reopen the gap in this flow.

## Atomicity Rule

Fingerprint generation and retrieval happen before persistence. If snapshot or
evidence fingerprint generation cannot complete, no attempt is written. If
diagnostics fails after a valid snapshot is available, append one failed
`retrieval_error` attempt with `after_state: unknown`, zero sources, and a safe
error code; then return that persisted record. This documents the operator action
without leaking provider or document details.

## Current Projection

For each repair item, list verification attempts newest first and find the most
recent passed attempt. Compare its snapshot fingerprint with the current space
snapshot:

```text
no passed attempt              -> unverified
same current snapshot          -> verified
different current snapshot     -> stale
```

Failed attempts do not downgrade a still-current passed proof. They remain
visible in history so the operator can investigate transient retrieval changes.

## UI Flow

The repair row stays the operator's single work surface:

```text
unverified -> Verify repair -> passed/failed result -> refreshed state chip
verified   -> show last verification time and history
stale      -> show stale cue and Verify repair action
```

The history view is bounded, text-only, and shows timestamp, result, before and
after support state, safe reason, source count, and fingerprint prefixes. It
does not display full hashes, raw snippets, or gated text.

## Security And Privacy

- Every store and projection is tenant scoped.
- Snapshot and evidence fingerprints are hashes over canonical metadata, not
  secret material or raw gated content.
- API errors use stable codes and avoid leaking records from another tenant.
- History limits are parsed and clamped server-side.
- The frontend uses `textContent` and existing safe fetch helpers.

## Compatibility

- Existing gaps and file data need no migration.
- Existing retest remains exact and ephemeral.
- Existing repair-list filters preserve their semantics; verification state is
  additive to the response.
- Later recurrence, eval, and trend phases consume ledger records but are not
  represented as Phase 23 state transitions.

## Testing Direction

The Stage 2 plan must include RED tests for:

- stable snapshot fingerprints under reordered document storage;
- changed content, review status, lifecycle status, or index metadata producing
  a changed snapshot;
- stable evidence fingerprints under deterministic diagnostics;
- passed, unsupported, review-gated, no-active-source, and retrieval-error
  attempt behavior;
- no partial write when fingerprint creation fails;
- file reopen persistence and newest-first bounded history;
- tenant and space isolation;
- verified-to-stale projection after a document mutation;
- server validation and frontend selector/action coverage;
- unchanged ephemeral retest behavior.

## Rejected Design Moves

- Reusing audit storage: verification and answer events have different meaning.
- Storing a mutable knowledge version counter: it adds drift and coordination.
- Including raw snippets in fingerprint input: it risks gated-text leakage.
- Auto-resolving after a pass: retrieval support is not a truth or policy claim.
- Adding recurrence records now: that is Phase 24's lifecycle and review gate.

## Stage 1 Approval Request

If approved, run `$gstack-autoplan` using this spec/design and the Phase 22
plan. Stage 2 must lock exact types, persistence behavior, APIs, tests, and the
minimal UI interaction before any TDD implementation begins.
