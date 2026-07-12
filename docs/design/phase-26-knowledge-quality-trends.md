# Phase 26 Knowledge Quality Trends Design

Date: 2026-07-11

Status: Approved; Stage 1 gate passed

Source spec: [Phase 26 Knowledge Quality Trends Spec](../specs/phase-26-knowledge-quality-trends.md)

## Design Summary

Phase 26 adds one tenant-scoped, read-only projection of observed repair
quality signals. It aggregates existing verification, recurrence, gap, and
promotion evidence, plus a new append-only observation of required promoted RAG
eval outcomes. The result is a bounded Quality Trends panel for one optional
space and a maximum 90-day local-date range.

The feature intentionally does not calculate a knowledge-quality score. It
shows only what the product observed, labels small samples honestly, and keeps
current stale state separate from historical event trends.

## Recommended Architecture

```text
Verification attempts --+
Knowledge gaps ---------+--> QualityTrendService --> bounded tenant/space projection
Recurrence records -----+                                  +--> admin Quality Trends panel
Promotion revisions ----+
Eval observations ------+

Existing eval runner
  -> report JSON/Markdown (unchanged)
  -> extract required promoted RAG results
  -> append RepairEvalObservation records
```

The trend service belongs on the admin side because its data sources are
tenant-scoped operational ledgers. The eval package continues to own suite and
check semantics. A narrow adapter converts completed promoted required checks
into neutral observation DTOs at the CLI/composition boundary; `internal/evals`
must not import admin storage.

## Data Ownership

- Verification service owns immutable verification attempts and current
  freshness projection.
- Gap service owns canonical question, space, and creation time.
- Recurrence service owns recurrence lifecycle and occurrence evidence.
- Promotion store owns durable promotion provenance.
- Eval runner owns check status and required-check semantics.
- New observation store owns append-only timestamped promoted-eval outcomes.
- Quality trend service owns no source data; it only validates filters and
  produces a safe projection.

## Key Decisions

### Time Is a Domain Fact

`SuiteResult` currently has no run timestamp. The new observation ledger records
`observed_at` when the existing eval command completes. File modification time
is never used for trends. The projection accepts date-only local ranges, returns
the resolved half-open instant range, and receives a test clock and location.

### Rates Need Their Denominators

The only Phase 26 rate is observed recurrence among repairs first successfully
verified in the same window and filter. Promoted eval pass rate uses all outcome
states, including unavailable. Both rates are withheld below three denominator
records. Counts are still shown.

### Isolation Before Aggregation

Tenant comes from the handler context and space filtering happens at every
source read before grouping, ranking, bucket generation, or trace collection.
There is no administrative global view in this phase.

### Honest UI States

The projection uses `observed`, `insufficient_sample`, `no_observations`, and
`unavailable`. The UI does not color missing data as good or create a direction
arrow for a withheld rate. A current stale count is explicitly labeled as a
snapshot.

## Failure Behavior

- A source-store failure marks only dependent metrics unavailable and returns a
  controlled request failure only when no safe projection can be formed.
- Bad historical entries are excluded and counted, without exposing payloads.
- Failure to append an eval observation makes the CLI evaluation command fail
  after its normal reports are written; it cannot silently claim trend coverage.
- Read paths are side-effect free. Disabling Phase 26 leaves prior ledgers,
  reports, and release gating usable.

## Test Strategy Direction

Use deterministic fixtures across tenants, spaces, date edges, timezones,
verification outcomes, recurrence states, and promoted eval outcomes. Assert
both calculated metrics and non-calculation: no rate below the sample threshold,
no report-file-time fallback, no raw content, and no cross-tenant trace ID.

## Stage 2 Focus

Autoplan should determine the exact DTO/store interfaces, compose the new eval
observation writer without an import cycle, and derive the implementation test
matrix from the accepted specification.
