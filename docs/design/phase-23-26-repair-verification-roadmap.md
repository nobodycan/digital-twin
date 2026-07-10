# Phase 23-26 Repair Verification Roadmap

Date: 2026-07-10

Status: Approved roadmap; each phase still requires its own Stage 1 spec and approval

Source context:

- Phase 21 made answer trust inspectable over time.
- Phase 22 added a derived Knowledge Repair Inbox and ephemeral local retest.
- The operator now needs durable proof that a repair worked and remained effective.

## Product Direction

The next product arc should deepen the knowledge repair loop instead of adding
another independent admin panel. The primary success criterion is:

> After an operator repairs a knowledge gap, the system can show durable,
> deterministic evidence that the original problem is supported and has not
> subsequently recurred.

The approved sequence is:

1. versioned repair verification;
2. suspected recurrence detection with human-confirmed reopen;
3. promotion of verified repairs into regression evals;
4. quality trends derived from real verification and recurrence records.

## Non-Goals For This Arc

- Autonomous knowledge writing or autonomous gap resolution.
- LLM-as-judge correctness or numeric confidence scores.
- Fuzzy semantic recurrence matching in the first recurrence phase.
- Automatic reopening of verified gaps.
- Cross-tenant analytics.
- Reviewer assignment, SLAs, notifications, or multi-user workflow.
- PDF, DOCX, crawling, cloud sync, RBAC, or database migration.
- Replacing the existing knowledge gap lifecycle.

## Core Architecture

The existing `KnowledgeGap` remains the only source of lifecycle state:

- `open`;
- `investigating`;
- `resolved`;
- `ignored`.

A new append-only verification ledger records evidence history. It does not copy
or own the gap lifecycle. Current verification state is projected from ledger
records, current reviewed knowledge, and later recurrence records.

```text
KnowledgeGap
    |
    v
RepairVerificationService
    |-- current active-reviewed knowledge snapshot
    |-- deterministic local retrieval
    |-- evidence fingerprint
    v
VerificationAttempt ledger
    |
    +--> current verification projection
    +--> recurrence detector
    +--> regression eval promotion
    +--> quality trend projection
```

## Fingerprint Semantics

### Knowledge Snapshot Fingerprint

The system should not introduce a mutable global version counter. A knowledge
snapshot fingerprint is computed from a stable sort of the selected space's
active, reviewed documents using fields such as:

- document ID;
- updated timestamp;
- content hash;
- lifecycle status;
- review status;
- index status.

The fingerprint changes when evidence-relevant knowledge changes. A changed
snapshot makes an earlier proof `stale`; it does not rewrite history or classify
the earlier attempt as failed.

### Evidence Fingerprint

The evidence fingerprint is computed from stable, bounded retrieval evidence:

- document ID;
- chunk ID or stable chunk position;
- document or chunk content hash;
- retrieval rank;
- review status.

It supports before/after comparison without storing gated raw text.

## Verification Attempt Contract

A verification attempt is append-only and tenant scoped. Roadmap-level fields:

```text
attempt_id
tenant_id
gap_id
space_id
started_at
completed_at
before_state
after_state
knowledge_snapshot_fingerprint
evidence_fingerprint
bounded_source_summaries
result: passed | failed
failure_reason:
  unsupported | review_gated | retrieval_error | no_active_source
```

A passed attempt requires all of the following:

- deterministic local retrieval completes successfully;
- the result is grounded;
- at least one source is active and reviewed;
- no review-gated raw text crosses the projection boundary;
- snapshot and evidence fingerprints are recorded atomically.

## Current Verification Projection

The projection is derived, not independently persisted:

- `unverified`: no passed attempt exists;
- `verified`: the latest passed attempt matches the current snapshot;
- `stale`: the latest passed attempt belongs to an older snapshot;
- `suspected_recurrence`: a later weak answer matches the verified repair;
- `confirmed_recurrence`: an operator confirmed the recurrence and reopened the gap.

`stale` means the proof expired because knowledge changed. It is not equivalent
to a failed repair.

## Phase 23 - Repair Verification Ledger

### Outcome

An operator can persist deterministic proof that a repair is supported by the
current active-reviewed knowledge snapshot.

### Scope

- Add a tenant-scoped append-only verification attempt store and service.
- Compute knowledge snapshot and evidence fingerprints deterministically.
- Preserve the existing ephemeral retest endpoint.
- Add a separate persistent verify action.
- Project current verification state into the Repair Inbox.
- Show bounded verification history without a new full-page dashboard.

### Recommended API Direction

- Keep `POST /admin/knowledge/repairs/retest` ephemeral.
- Add `POST /admin/knowledge/repairs/verify` for a persisted attempt.
- Add `GET /admin/knowledge/repairs/verifications?gap_id=...` for bounded history.

Exact contracts must be locked by the Phase 23 Stage 2 plan.

### Acceptance Evidence

- Identical knowledge snapshots produce identical fingerprints.
- A successful verify writes one complete attempt.
- Unsupported, review-gated, no-active-source, and retrieval-error attempts are
  recorded as failed without changing gap status.
- A partial write cannot occur if fingerprinting or persistence fails.
- A document change makes the current projection stale while preserving history.
- Tenant and space isolation are covered by tests.
- Existing retest remains provider-free and mutation-free.

## Phase 24 - Recurrence Watch

### Outcome

The system detects when a previously verified problem may have returned, but a
human remains responsible for reopening the gap.

### Scope

- Compare new weak-answer audit evidence against verified repairs.
- Match only by explicit gap ID or the existing exact fallback key:
  space + original question + no-source reason.
- Create deduplicated suspected-recurrence records.
- Let an operator confirm, dismiss, or leave a suspicion pending.
- Reopen a gap only after explicit confirmation.

### Acceptance Evidence

- Unrelated weak answers do not create recurrence records.
- Duplicate weak events do not create duplicate pending suspicions.
- Dismissal preserves the record and reason.
- Confirmation reopens the gap and preserves the original verification attempts.
- Detection failure never blocks the normal answer path.
- No semantic or cross-tenant guessing is introduced.

## Phase 25 - Repair-to-Eval Promotion

### Outcome

An operator can turn a verified repair into a deterministic regression case that
protects future retrieval and knowledge changes.

### Scope

- Promote only `verified`, non-stale repairs.
- Persist provenance back to the verification attempt and gap.
- Fix the question, knowledge space, and minimum expected support state.
- Avoid requiring one exact source document unless explicitly configured.
- Run promoted cases through the existing eval and release-gate path.

### Acceptance Evidence

- Unverified, stale, or suspected-recurrence repairs cannot be promoted.
- Promotion is idempotent.
- A promoted case fails when the answer becomes unsupported or review gated.
- A failing promoted case can block the configured release gate.
- Evidence changes can pass when support remains valid; exact-document
  overfitting is avoided by default.

## Phase 26 - Knowledge Quality Trends

### Outcome

The operator can see whether repairs are getting faster and more durable using
only observed verification, recurrence, and eval records.

### Scope

- Verification throughput and time-to-verify.
- Stale verification count.
- Suspected and confirmed recurrence rate.
- Repeatedly failing questions or spaces.
- Promoted eval pass/fail trend.
- Bounded local time windows and tenant/space filters.

### Acceptance Evidence

- Every metric can be traced to ledger, recurrence, or eval records.
- Empty and small samples render honestly.
- Cross-tenant data cannot enter a projection.
- No subjective accuracy percentage or confidence score is presented.
- Trend queries are bounded and stable over deterministic fixtures.

## Failure Boundaries

- Fingerprint failure writes no partial verification attempt.
- Knowledge changes invalidate the current proof but never mutate old attempts.
- Deleted, disabled, rejected, or archived evidence remains in historical
  summaries while making the current projection `stale` because the active
  reviewed knowledge snapshot changed.
- Recurrence detection errors are observable but do not block answers.
- All lists are bounded and tenant/space scoped.
- Review-gated records expose state, reason, and counts only.
- Each phase owns additive records and APIs that can be rolled back without
  requiring the next phase.

## Delivery Rules

This roadmap does not approve implementation. Under `AGENTS.md`, each phase is
one independent feature and must run the full pipeline:

1. Stage 1: `$gstack-office-hours` produces that phase's spec/design artifact;
2. user approval gate;
3. Stage 2: `$gstack-autoplan` locks architecture and test matrix;
4. user approval gate;
5. Stage 3: Superpowers TDD, RED -> GREEN -> REFACTOR;
6. Stage 4: `$gstack-review` and any required approval;
7. Stage 5: `$gstack-qa` against a running target;
8. Stage 6: `$gstack-cso` and high-severity approval gate;
9. Stage 7: verification, ship, and later land/deploy approval.

No later phase may leak into an earlier phase merely because its data model is
visible in this roadmap.

## Next Assignment

Start Phase 23 Stage 1 only. Run `$gstack-office-hours` using this roadmap and
the shipped Phase 22 spec/design/plan as inputs. Challenge the exact definition
of `verified`, fingerprint inputs, persistence shape, stale behavior, and the
minimum admin UI before producing the Phase 23 spec.
