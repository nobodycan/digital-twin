# Phase 26 Knowledge Quality Trends Spec

Date: 2026-07-11

Status: Approved; Stage 1 gate passed

Source roadmap: [Phase 23-26 Repair Verification Roadmap](../design/phase-23-26-repair-verification-roadmap.md)

Prior phase: [Phase 25 Repair-to-Eval Promotion Spec](./phase-25-repair-to-eval-promotion.md)

## Problem Statement

Phases 23 through 25 capture three operational facts: a repair verification
attempt, a suspected or confirmed recurrence, and a promoted repair evaluated
by the normal eval runner. An operator can inspect each repair individually,
but cannot answer a basic operational question: are repairs becoming faster and
more durable for one tenant and knowledge space?

The tempting answer is a synthetic "knowledge quality" score. That would be
misleading: the system observes only repair, recurrence, and eval events, not
the correctness of all knowledge. Phase 26 therefore delivers a bounded trend
projection of observed evidence only. It never predicts accuracy, expresses
confidence, or treats missing data as success.

## Target User And Wedge

The target user is the local knowledge operator reviewing the outcome of recent
repair work for one tenant and, optionally, one knowledge space.

The narrow wedge is a read-only Quality Trends panel in the existing knowledge
admin surface. Its default view covers the previous 30 local calendar days and
answers five evidence-backed questions: how much was verified, how long verified
repairs took, how many repairs are currently stale, whether recurrence was
observed, and how promoted repair evals performed.

## Office-Hours Findings

### The Actual Job

The operator does not need a dashboard for its own sake. They need to decide
whether to investigate a space, improve repair practice, or trust that a recent
repair is holding. Every displayed number must point back to a bounded list of
safe record IDs and reasons.

### Premises Challenged

1. **A single quality score makes trends easier to read.** Rejected. It would
   combine unrelated observations and falsely imply coverage of unobserved
   questions.
2. **Report file modification time is an eval run timestamp.** Rejected. It is
   a filesystem artifact, can be copied or overwritten, and is not durable
   business evidence. Promoted eval outcomes need an explicit observation time.
3. **A recurrence rate can use every historical repair as its denominator.**
   Rejected. That silently compares different eras and makes a selected window
   meaningless. Both numerator and denominator must be scoped to the selected
   time window and filter.
4. **No events means improving quality.** Rejected. It may mean no repair,
   verification, audit, or eval activity. Empty and small samples must be
   presented as insufficient evidence.
5. **A cross-tenant overview is useful to operators.** Rejected. Tenant
   isolation is a hard boundary; the endpoint and projection have one resolved
   tenant only.

## Goals

1. Project verification throughput, time-to-verify, and stale-verification
   count from durable Phase 23 records.
2. Project suspected and confirmed recurrence counts and an honest bounded
   recurrence rate from Phase 24 records.
3. Identify a bounded set of repeated failing repair questions and spaces using
   recurrence and promoted-eval observations.
4. Persist and project promoted repair eval pass/fail observations with a real
   run timestamp and promotion provenance.
5. Support deterministic, bounded local date windows plus an optional space
   filter, always within the authenticated/admin tenant.
6. Make every metric traceable to safe IDs, counts, statuses, and reasons.
7. Preserve existing repair, verification, recurrence, promotion, eval, and
   release-gate behavior.

## Non-Goals

- Accuracy, correctness, confidence, satisfaction, or predictive quality
  scoring.
- LLM judging, semantic clustering, embeddings, or inferred matching.
- Automatic repair, promotion, recurrence confirmation, alerting, scheduling,
  assignment, or release decisions.
- Global, cross-tenant, or cross-space ranking.
- Historical reconstruction from report modification times or unbounded scans
  of arbitrary report directories.
- New retrieval semantics, repair eligibility rules, or eval assertions.
- Showing raw answers, diagnostic errors, citations, source snippets, or
  unbounded question content in a trend response.

## Existing Building Blocks

- `RepairVerificationAttempt` is an append-only, tenant-scoped record with
  `started_at`, `completed_at`, `space_id`, result, and safe failure reason.
- `KnowledgeGap` supplies the canonical gap, its space, creation time, and
  current verification projection.
- `RepairRecurrence` supplies tenant, gap, space, status, occurrence count,
  answer state, audit references, and creation/update/confirmation timestamps.
- `RepairEvalPromotionRevision` supplies active promoted repair provenance,
  tenant, gap, space, case ID, and promotion time.
- `evals.SuiteResult` and `CheckResult.Promotion` already identify pass/fail
  checks attributable to a promoted repair, but currently lack an authoritative
  observation timestamp.

## Product Decisions

### One Explicit Projection Request

Phase 26 adds one read-only tenant-scoped projection request:

```text
GET /admin/knowledge/quality-trends?from=YYYY-MM-DD&to=YYYY-MM-DD&space_id=<optional>
```

The server derives the tenant from existing admin context; it never accepts a
tenant ID from the query. `from` and `to` are inclusive local calendar dates.
The service converts them to an explicit half-open instant range and returns
the effective timezone and timestamps in the response. Defaults are the last
30 local calendar days ending today. The maximum range is 90 days;
invalid, reversed, or oversized ranges return a stable `400` response.

The implementation must inject its clock and location for deterministic tests.
The Stage 2 engineering plan must name the configured server-local location
and verify daylight-saving boundary behavior where applicable.

### Bounded Evidence, Not a Score

The response has separate metric groups and a `sample` object for every rate or
duration. A metric is either:

- `observed`: enough matching records exist to calculate it;
- `insufficient_sample`: fewer than three denominator records exist;
- `no_observations`: no matching records exist; or
- `unavailable`: a required source could not be read.

`insufficient_sample` returns the observed count but no percentage, direction,
or inferred conclusion. A zero count with a readable source is an observed zero,
not an unavailable result. The UI must use these labels verbatim rather than
green/red quality language.

### Verification Metrics

For attempts whose `completed_at` falls inside the requested range and whose
tenant/space match the projection:

- **Throughput:** count of completed attempts, plus count of distinct gaps with
  at least one passed attempt. Failures remain included in attempt throughput.
- **Time to verify:** for each gap with its first passed attempt in the range,
  `completed_at - gap.created_at`; publish median and sample size only. A
  missing or future gap timestamp excludes that sample and increments an
  excluded-record count.
- **Stale verification count:** current count, evaluated at projection time, of
  matching resolved gaps whose current verification projection is `stale`.
  It is explicitly a current snapshot, not a historical count within the date
  range, and includes the projection timestamp.

The system must not call a failed attempt a completed repair, nor derive a
duration from gap resolution time because that event does not prove support.

### Recurrence Metrics

The projection selects recurrence records whose first observed recurrence
(`created_at`) falls in the date range and matches tenant/space. It returns:

- count of unique recurrence records first observed as `suspected`;
- count whose `confirmed_at` falls in the range;
- count whose `dismissed_at` falls in the range; and
- recurrence rate: unique selected gaps with a recurrence first observed in the
  range divided by unique selected gaps with a first passed verification in the
  same range and filter.

The rate follows the sample-state rules above. It is labelled **observed
recurrence rate among newly verified repairs**, never a durability or accuracy
rate. A confirmed recurrence can be counted even when it was first suspected
earlier; it does not alter the historical suspected count.

### Repeated Failure Signals

The panel returns at most five entries for each ranked list:

- **Repeated questions:** canonical repair gaps ordered by total selected
  recurrence occurrences, then confirmed count, then stable gap ID.
- **Repeated spaces:** spaces ordered by the same aggregate across their
  matching gaps.
- **Promoted eval failures:** active promotion case IDs ordered by failed
  observed runs, then latest observed time, then case ID.

Each item contains tenant-scoped IDs, space ID, bounded question summary (at
most 160 characters), counts, statuses, and the latest relevant timestamp.
It must not contain raw answer text, evidence snippets, source titles, audit
payloads, or internal errors. Ties use stable lexical IDs so deterministic
fixtures produce deterministic output.

### Durable Promoted-Eval Observations

Phase 26 adds an append-only `RepairEvalObservation` ledger written only after
the existing eval runner produces a suite result. Each observation contains:

```json
{
  "id": "repair-eval-observation-...",
  "tenant_id": "tenant-a",
  "run_id": "release-2026-07-11",
  "case_id": "repair-eval-tenant-a-gap-1",
  "gap_id": "gap-1",
  "space_id": "support",
  "promotion_id": "repair-eval-promotion-...",
  "promotion_revision": 1,
  "observed_at": "2026-07-11T09:00:00Z",
  "status": "passed",
  "failure_reason": ""
}
```

One observation represents the required RAG result for one promoted case in one
run. `status` is `passed`, `failed`, or `unavailable`; skipped or malformed
required results persist as `unavailable` with a bounded category. The writer
deduplicates `(tenant_id, run_id, case_id, promotion_id, promotion_revision)`
without rewriting history. Static fixture checks without promotion provenance
are excluded. Failure reasons are controlled categories, never evaluator text.

The normal eval command remains the only writer during Phase 26. The write is
additive: an observation-store failure must make the command fail after reports
are written, rather than claim a complete trend record. Existing report JSON
and release-gate semantics do not change.

### Promoted Eval Trend

For observations whose `observed_at` falls in the range and match the filter,
the panel returns passed, failed, unavailable, and total counts. It returns a
pass rate only when at least three observations exist; `unavailable` is part of
the denominator and never passes by omission. A maximum of 13 seven-day buckets
is returned for the 90-day maximum range. Bucket boundaries are local-date
aligned and include zero-observation buckets so the UI cannot invent continuity.

## Response Contract Direction

The endpoint returns a stable JSON projection shaped like:

```json
{
  "tenant_id": "derived-not-client-supplied",
  "filter": {
    "space_id": "support",
    "from": "2026-06-12",
    "to": "2026-07-11",
    "timezone": "Asia/Shanghai",
    "start_at": "2026-06-12T00:00:00+08:00",
    "end_exclusive_at": "2026-07-12T00:00:00+08:00"
  },
  "verification": { "throughput": {}, "time_to_verify": {}, "stale": {} },
  "recurrence": { "suspected": 0, "confirmed": 0, "dismissed": 0, "rate": {} },
  "promoted_eval": { "passed": 0, "failed": 0, "unavailable": 0, "rate": {}, "buckets": [] },
  "repeated_failures": { "questions": [], "spaces": [], "promoted_cases": [] },
  "trace": { "verification_attempt_ids": [], "recurrence_ids": [], "eval_observation_ids": [] }
}
```

All trace arrays are capped at 20 newest matching IDs and explicitly expose a
`truncated` flag. The detailed source lists remain the existing bounded repair,
verification, recurrence, promotion, and report surfaces; this projection does
not become a raw ledger dump.

## UX Direction

Add a `Quality trends` panel beside the Repair Inbox, not a separate analytics
application. It provides a 30-day default and bounded date/space controls, then
renders:

1. verification throughput, median time to verify, and current stale count;
2. suspected/confirmed recurrence with the observed-rate label and sample;
3. promoted eval pass/fail/unavailable counts plus weekly buckets; and
4. the three capped repeated-failure lists with safe identifiers and counts.

Empty states say which observation source has no records. Small-sample states
say that a rate or median is withheld. A stale current count is visually marked
as current state rather than plotted as a historical trend. Links, if added,
reuse existing tenant-scoped detail routes and preserve the selected space.

## Security And Failure Boundaries

- Every source store call and every projection filter receives the server-side
  tenant; no cross-tenant input enters aggregation or trace output.
- Space filtering occurs before aggregation. An unknown space behaves like the
  existing scoped admin views and reveals no other-space information.
- Lists, windows, bucket counts, summaries, IDs, and trace arrays are bounded.
- The endpoint returns controlled error codes and no raw storage, evaluator, or
  retrieval errors.
- The observation ledger stores no question text, assistant output, citation,
  source content, document title, or audit payload.
- A malformed historical record is excluded with a controlled excluded count;
  it cannot panic the projection or be silently reclassified as a pass.
- Trend reads have no writes. Observation writes are append-only and rollback
  is an additive feature disablement, leaving original verification, recurrence,
  promotion, reports, and release gates intact.

## Acceptance Criteria

1. Every displayed metric and trace ID is attributable to a verification,
   recurrence, gap, promotion, or explicit eval-observation record.
2. Empty, unavailable, and fewer-than-three-sample cases produce distinct,
   honest stable states; no percentage or direction is emitted for small samples.
3. A 1 to 90 day local-date range is deterministic under an injected clock and
   location; invalid ranges are rejected without reading unbounded data.
4. Tenant A can never affect Tenant B's counts, ranked entries, bucket values,
   trace IDs, or error distinctions.
5. A space filter excludes all other spaces before aggregation and preserves
   stable ordering for ties.
6. Verification metrics use completed attempts and first successful verification
   timestamps exactly as defined; stale count is labeled current.
7. Recurrence rate has a same-window/same-filter denominator and is withheld
   for insufficient samples.
8. Each promoted required RAG outcome produces at most one idempotent explicit
   observation per run/provenance key; missing/invalid required output becomes
   `unavailable`, never pass.
9. Promoted eval buckets include zero-event periods and never use report file
   metadata as business time.
10. Existing eval reports, static fixture behavior, repair workflows, and
    release gate outcomes remain backward compatible.

## Test Matrix Direction

Stage 2 must turn these into RED-first tests:

| Boundary | Required proof |
| --- | --- |
| Window parsing | defaults, inclusive dates, 1/90-day bounds, reversal, invalid date, local midnight, DST location |
| Tenant and space | mixed tenant/space fixtures cannot leak through counts, ranks, buckets, traces, or missing-record responses |
| Verification | completed/pass/fail throughput, first pass per gap, median, missing timestamps, current stale distinction |
| Recurrence | suspected/confirmed/dismissed timestamps, same-window denominator, small samples, occurrence aggregation, stable ties |
| Eval observation | append-only persistence, reopen, idempotency, promotion provenance, required failed/skipped/unavailable mapping, no static-fixture observation |
| Projection | empty/unavailable/small states, capped traces/lists, stable buckets, malformed-record exclusion, controlled failures |
| HTTP and UI | query validation, tenant derivation, space preservation, honest labels, safe summaries, existing admin behavior |
| Regression | existing repair, verification, recurrence, promotion, eval report, and release-gate tests remain green |

## Alternatives Considered

### A. Explicit Observation Ledger Into a Single Trends Projection (Recommended)

Persist only promoted required-eval outcomes with their actual observation time,
then aggregate all existing ledgers through one bounded tenant/space projection.
This keeps the trend source auditable and avoids treating report files as a
database.

### B. Scan Eval Report Files at Query Time

Rejected. Reports can be overwritten, renamed, copied, or lack an authoritative
run timestamp. Scanning also creates unbounded filesystem and tenancy risks.

### C. Compute a Composite Knowledge-Quality Score

Rejected. The inputs cover only observed repair activity; a score would make
unobserved knowledge look measured and hide the individual causes of change.

## Stage 2 Assignment

Run autoplan against this spec. The engineering review must lock the package
boundary for the eval-observation writer, define the clock/location contract,
choose the exact storage composition, and prevent `internal/evals` from
depending on admin persistence. The review must retain the narrow wedge and
reject a composite score or report-file-time reconstruction.

## Stage 1 Gate

No implementation has been started. Approval of this spec is required before
entering Stage 2 autoplan.
