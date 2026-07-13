# Phase 28 Knowledge Quality Review Checkpoints Design

Date: 2026-07-13

Status: Approved; Stage 1 gate passed

Source spec: [Phase 28 Knowledge Quality Review Checkpoints Spec](../specs/phase-28-knowledge-quality-review-checkpoints.md)

## Design Summary

Phase 28 adds a small durable decision layer on top of the existing read-only
Quality Trends projection. A local operator saves an immutable checkpoint only
after reviewing a bounded tenant and optional-space trend view. The server
recomputes and stores an allow-listed snapshot, the operator selects an
explicit outcome, and selected repair gap IDs become references rather than
workflow mutations.

The design deliberately separates observed evidence from human judgment. It
does not produce a score, infer an owner, close repairs, or automate an alert.

## Recommended Architecture

```text
Quality Trends filter + outcome/rationale/gap IDs + idempotency key
                    |
                    v
Admin HTTP handler -- derives tenant --> QualityReviewCheckpointService
                                            |          |
                                            |          +--> recomputed projection defines candidates
                                            |          +--> KnowledgeGapService validates linked IDs
                                            |
                                            +--> QualityTrendService recomputes safe snapshot
                                            |
                                            +--> append-only checkpoint ledger
                                                        |
                                                        v
                                              bounded admin history + compatible delta
```

The checkpoint service belongs in `internal/admin`, beside trend and repair
projections. The server composition root injects the existing trend and gap
services plus a dedicated checkpoint store. No eval or retrieval package
depends on checkpoint persistence.

## Data Boundaries

The persisted record is an immutable operational summary:

```text
QualityReviewCheckpoint
  schema_version, id, tenant_id, created_at
  filter: from, to, space_id, timezone, window_days,
          start_at, end_exclusive_at
  outcome, rationale, gap_ids
  snapshot: projected_at plus allow-listed counts and Phase 26 metric states/values
  internal replay metadata: hashed idempotency key, canonical request fingerprint
```

Snapshot fields exclude trend trace lists, question summaries, repeated-failure
text, audit references, document data, answers, source excerpts, diagnostics,
and evaluator output. Raw idempotency keys are not persisted or returned. A
checkpoint keeps selected gap IDs as historical references, so later source
mutations do not rewrite past decisions.

## Interaction Model

1. The operator loads the existing Quality Trends projection.
2. The checkpoint form adopts the active filter and remains unavailable before
   a successful load.
3. The operator selects `observe`, `repair_required`, or `risk_accepted`.
4. Repair/risk outcomes require a short rationale and one to ten eligible gap
   IDs. The browser creates one opaque idempotency key for the normalized draft
   and retains it across unchanged retries.
5. The server normalizes the decision and checks the key first. A matching
   replay returns the original checkpoint without recomputation; conflicting
   reuse returns `409`.
6. For a new key, the server recomputes the trend projection, derives the
   eligible candidate set from repeated recurrence and promoted-eval failures,
   validates selected gaps against that set plus tenant/scope, appends the
   record, then returns it and any compatible earlier-window predecessor.
7. The panel refreshes bounded history. It labels a missing predecessor or
   non-observed metric as unavailable for directional comparison.
8. API failures retain the draft and show the stable error code's safe message,
   corrective hint, and `X-Request-ID` when present.

## Comparison Rules

Compatibility is intentionally strict about scope and intentionally useful
across time: tenant, optional-space value, timezone, and local-calendar
window-day count must match, while the predecessor's end date must be strictly
earlier. Select the candidate with the latest earlier end date, then use
creation time and checkpoint ID as deterministic tie-breakers. An exact
repeated date window remains history but is not a periodic predecessor.

Counts may show a signed change when both snapshots contain the count. Rates
and durations may show a change only if both source metric states are
`observed`. All other sample states remain visible as their original labels.
Current stale verification is labeled as a current-at-review snapshot in both
the original projection and comparison. The UI shows both date windows and both
rate denominators and calls the result a `review-to-review difference`, never
an improvement, regression, or causal effect.

## Failure Semantics

- Invalid filters, outcomes, rationale, or IDs return a controlled client
  error before persistence.
- Oversized input returns `413`; malformed input returns `400`.
- An absent, scope-incompatible, or save-time non-candidate selected gap
  returns a non-enumerating `409` and reveals no other-tenant details.
- Same-key/same-request replay returns `200`; same-key/different-request reuse
  returns `409`; first creation returns `201`.
- A trend dependency or checkpoint store failure returns unavailable and makes
  no write or repair-state change.
- A malformed historical record cannot panic a list response or generate a
  favorable comparison.
- Phase 28 errors use top-level `error`, `message`, and `hint`; internal causes,
  file paths, tenant details, and gap-existence details remain hidden.

## Deferred Work

- Named reviewer attribution, roles, and approval chains.
- Editing, deleting, or superseding a checkpoint.
- Assignment, notification, automation, and repair-status transitions.
- Cross-scope, cross-timezone, mixed-window-width, same/later-window comparison,
  exports, scorecards, and dashboard aggregation.

## Stage 1 Gate

This design is intentionally implementation-free. Stage 2 autoplan must use
the source spec to select exact types, storage path, error codes, and test
sequence before TDD implementation begins.
