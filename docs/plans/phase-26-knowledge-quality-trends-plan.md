# Phase 26 Knowledge Quality Trends Plan

Date: 2026-07-11

Status: Implemented; Stage 2 plan approved and Stage 3 complete

Source spec: [Phase 26 Knowledge Quality Trends Spec](../specs/phase-26-knowledge-quality-trends.md)

Source design: [Phase 26 Knowledge Quality Trends Design](../design/phase-26-knowledge-quality-trends.md)

Base branch: `main`

## Plan Summary

Build a read-only, tenant-scoped Quality Trends panel for the existing knowledge
admin page. It will aggregate bounded verification, recurrence, gap, promotion,
and new promoted-eval observation records for an optional space and a 1 to 90
day local-date range. It will not calculate a composite quality score, infer
accuracy, scan report files, or modify repair/release behavior.

## Approved Premises

The Stage 1 approval confirms these product premises:

1. Operators need evidence-backed repair trend visibility, not a universal
   knowledge-accuracy claim.
2. File metadata is not a valid eval observation time; explicit append-only
   observations are required.
3. Tenant and space isolation apply before every aggregation and trace list.
4. Empty and small samples must withhold rates and trends rather than look good.
5. The narrow wedge belongs in the existing admin page and existing eval path.

## CEO Review

### 0A. Premise Challenge

The original scope is correct once it is constrained to observed operational
evidence. A dashboard would be the wrong product if it claimed all knowledge
quality from repair activity. The plan keeps the valuable decision support while
rejecting the misleading score, prediction, and report-file scan variants.

### 0B. Existing Code Leverage

| Sub-problem | Existing leverage | Plan decision |
| --- | --- | --- |
| Verified repair facts | `RepairVerificationAttempt` and `RepairVerificationService` | Read append-only attempts; preserve current stale projection semantics. |
| Gap timing and scope | `KnowledgeGap` and `KnowledgeGapService` | Use canonical `created_at` and `space_id`; do not infer resolution timing. |
| Recurrence evidence | `RepairRecurrence` and `RepairRecurrenceService` | Read status timestamps and occurrence counts; no new matching logic. |
| Promotion provenance | `RepairEvalPromotionRevision` | Reuse active promotion IDs and revision fields in eval observations. |
| Eval outcome semantics | `evals.Runner`, `SuiteResult`, `CheckResult.Promotion` | Extract only required promoted RAG outcomes after the existing runner finishes. |
| Persistence pattern | File and in-memory stores under `internal/admin` | Add matching append-only observation stores; no report directory scans. |
| Admin delivery | `internal/server`, `web/admin.html`, `web/admin.js`, `web/app_static_test.go` | Add one GET route and a compact panel next to Repair Inbox. |

### 0C. Dream State Delta

```text
Current
  Operator opens individual repair histories and eval reports
  -> manually guesses whether recent repair work is holding

Phase 26
  Tenant + optional space + bounded dates
  -> honest observed counts, rates when sample permits, current stale snapshot
  -> capped links/IDs for the underlying repair evidence

12-month ideal (not this phase)
  Durable trend history, alerts, richer segmentation, and scheduled reporting
  -> only after operators demonstrate that this narrow panel changes decisions
```

### 0C-bis. Alternatives

| Option | Benefits | Cost and risk | Decision |
| --- | --- | --- | --- |
| Explicit eval observation ledger plus projection | Auditable time, tenant-safe storage, stable deterministic tests | Adds one storage contract and CLI write path | Selected |
| Read JSON reports at query time | Fewer new types at first | Files can overwrite/copy, lack business timestamps, and invite unbounded scans | Rejected |
| Composite score | Compact headline | Hides causes and falsely implies coverage of unobserved knowledge | Rejected |

### Scope Decisions

Accepted scope:

- Add `RepairEvalObservation` model, store, validation, append idempotency, and
  tenant-scoped listing in `internal/admin`.
- Write observations from `cmd/cli eval` after existing reports are written.
- Add `QualityTrendService` with injected clock and local timezone, explicit
  filter parsing, safe response DTOs, metric states, rankers, and buckets.
- Wire one `GET /admin/knowledge/quality-trends` endpoint and one admin panel.
- Cover the contracts through unit, server, CLI, and static-web tests.

Not in scope:

- Scores, alerts, background jobs, global views, semantic grouping, LLM judging,
  historical report import, and chart-library adoption. They either claim more
  than the evidence supports or expand beyond the operator's immediate decision.
- Changes to retrieval, repair lifecycle, recurrence detection, promotion
  eligibility, report format, and release-gate semantics.

### Error And Rescue Registry

| Failure | User-visible outcome | Rescue behavior |
| --- | --- | --- |
| Invalid date/space query | `400 invalid_quality_trend_filter` | Preserve the panel and show a concise filter error. |
| Trend dependency absent | `503 knowledge_quality_trends_unavailable` | Do not show partial cross-tenant or synthetic data. |
| One source read fails | Affected metric is `unavailable` with controlled code | Other independently readable metrics remain visible. |
| Malformed historical record | Excluded count increases | Never panic, expose raw data, or classify it as pass. |
| Observation write fails after reports | CLI exits nonzero after report paths are printed | Operators know trend evidence is incomplete; normal report remains available. |
| Small or empty sample | Counts remain visible, rate/median withheld | Explain `insufficient_sample` or `no_observations`. |

### Failure Modes Registry

| Mode | Prevention/proof |
| --- | --- |
| Cross-tenant data leaks via aggregation or traces | Server derives tenant; stores filter before group; mixed-tenant tests assert IDs, ranks, and errors. |
| Eval writes duplicate or changing history | Unique observation key and append-only in-memory/file-store tests. |
| A skipped required check reads as pass | Map absent/malformed/skipped required promoted RAG result to `unavailable`. |
| Stale count is displayed as history | Separate `as_of` snapshot field and dedicated UI label/test. |
| Order changes across runs | Stable tie-break IDs and deterministic fixtures. |
| User interprets no events as success | Explicit state labels; no direction/color claim for empty or small samples. |

## Design Review

### Information Architecture

Place `Quality trends` immediately after the Repair Inbox. It shares the
operator's repair context rather than adding top-level navigation. The panel has
one compact filter row: from date, to date, optional current space, and Refresh.
It then presents four scan-friendly groups: verification, recurrence, promoted
evals, and repeated signals.

### Interaction States

- Loading: keep previous values visible and mark the panel as refreshing.
- Valid response: render metric label, count, sample state, and current/stale
  qualifier without color-only meaning.
- Empty: say `No matching observations in this window`.
- Small sample: say `Rate withheld: fewer than 3 observations`.
- Unavailable/error: show controlled source or request failure and retain filters.
- Narrow viewport: stack metric groups and preserve native date controls.

### Accessibility And Visual Rules

- Use semantic headings, labels for every date/space control, and live status
  text for refresh success/failure.
- Use text labels in addition to any state color. Do not create a chart whose
  meaning depends only on red/green.
- Render safe bounded question summaries via `textContent`; no HTML insertion
  from ledger data.
- Keep the existing visual system and vanilla JavaScript patterns; no new UI
  framework or chart dependency.

### Design Decisions

- Show weekly promoted-eval buckets as a compact text/table list, not a chart.
  The sample is often small and the no-chart form makes zero buckets and
  unavailable states readable without visual overclaim.
- Current stale verification count is a snapshot card, not a time-series point.
- Repeated question summaries remain truncated to 160 characters in the API and
  are bounded again in the DOM renderer.

## Engineering Review

### Dependency And Data Flow

```text
cmd/cli eval
  -> evals.Runner.Run (unchanged)
  -> evals.WriteReports (unchanged)
  -> main-package adapter extracts promoted required RAG results
  -> admin.RepairEvalObservationService.Append

server handler
  -> QualityTrendService.Project(derived tenant, validated filter)
  -> verification/gap/recurrence/promotion/observation readers
  -> safe QualityTrendProjection JSON
  -> web/admin.js renders bounded values
```

`internal/evals` remains independent of `internal/admin`. The CLI composition
root is allowed to import both packages and is the sole Phase 26 observation
writer. The server and CLI each construct the new file store from their existing
admin data directory.

### Contracts

1. Add `RepairEvalObservation` with only tenant/gap/space/promotion/run/result
   identifiers, promotion revision, `observed_at`, controlled status, and
   controlled failure category. Do not persist question or evaluator message.
2. Add `RepairEvalObservationStore` methods to append idempotently and list by
   one tenant. File persistence uses the existing temp-file/rename pattern,
   reopen support, and bounded validation.
3. Add a dedicated read-only `QualityTrendSource` boundary. It must request an
   explicit tenant, optional space, half-open time range, and per-source cap;
   its adapters must query/filter before aggregation. Do not use the existing
   Repair Inbox list methods: gap listing defaults an empty space to `default`,
   and public verification history is capped at 100 records. The source needs
   an all-space gap query plus time-filtered verification, recurrence,
   promotion, and observation reads with a controlled truncation result.
4. Add `QualityTrendFilter` with `From`, `To`, and optional `SpaceID`; convert
   local calendar dates to `[start, end)` in an injected `*time.Location`.
5. Add `QualityTrendService` that reads the dedicated bounded source, preserves
   its truncation/exclusion state, and returns typed metric states plus capped
   traces. A truncated source must yield `unavailable` for dependent rates,
   never a plausible partial percentage.
6. Expose `GET /admin/knowledge/quality-trends`; parse only `from`, `to`, and
   `space_id`; derive tenant via `h.adminTenantID()`; map invalid input and
   unavailable dependencies to stable codes.
7. Extend `web/admin.html`, `web/admin.js`, and `web/styles.css` only as needed
   for a native compact panel. Extend static assertions for every new selector
   and route string.

### Metric Algorithms

| Metric | Source and algorithm |
| --- | --- |
| Verification throughput | Count attempts with `completed_at` in `[start,end)`; separately count distinct gaps with a passed attempt. |
| Median time to verify | For each gap, select its earliest passed attempt in range; subtract canonical `gap.created_at`; sort durations and return deterministic median plus exclusions. |
| Current stale count | Re-project matching resolved gaps at `now`; return count and `as_of`, independent of date window. |
| Suspected/confirmed/dismissed recurrence | Select by relevant record timestamp, tenant, space, then count distinct records. |
| Observed recurrence rate | Unique gaps first recurring in range / unique gaps first passed in range, same tenant and space; withhold under denominator 3. |
| Promoted eval rate | Pass / total observations in range, with unavailable included; withhold under total 3. |
| Buckets/rankings | Max 13 seven-day local-date buckets; max five ranked items; stable ID tie breaks; traces capped at 20 newest records. |

### Task Sequence

1. RED: add focused admin tests for observation validation, idempotent append,
   file reopen, tenant isolation, and controlled statuses.
2. GREEN: implement the observation model/store/service in `internal/admin`.
3. RED: add CLI cases for passed, failed, missing/skipped required promoted RAG
   results, static fixtures, and observation-store write failure after reports.
4. GREEN: add the CLI adapter/writer without touching eval package contracts.
5. RED: add dedicated trend-source fixtures for all-space selection, time
   filtering, source caps/truncation, and mixed tenant/space isolation.
6. GREEN: implement the bounded source adapters without changing existing
   Repair Inbox list semantics.
7. RED: add quality-trend service fixtures for date conversion, all metric
   states, tenant/space isolation, rankings, malformed records, truncation, and
   buckets.
8. GREEN: implement service/filter/projection DTOs and safe aggregation helpers.
9. RED: add handler tests for route/auth, filter errors, tenant derivation,
   unavailable services, and projection JSON.
10. GREEN: wire config/handler route and compose stores in `cmd/server`.
11. RED: extend static web tests for panel controls, route, safe state labels,
   and renderer behavior.
12. GREEN: add the panel, refresh flow, and responsive CSS using current admin
    patterns. Run all existing regression suites after each completed slice.

### Performance

The maximum time range is 90 days, bucket count is 13, rank lists are five, and
trace lists are 20. Phase 26 uses existing local stores; Stage 3 must keep reads
bounded and avoid per-row re-scans by grouping source records once per request.
No endpoint may recursively scan the report directory or invoke retrieval.

## DX Review

### Developer Persona And Journey

The primary developer is a maintainer adding or operating a local Go knowledge
service. Their critical path is: run `go run ./cmd/cli eval ...`, see existing
reports, know whether promoted outcomes were persisted, then query the admin
endpoint or panel. The plan keeps all existing flags and report paths stable.

### DX Requirements

- Keep CLI output explicit: report paths appear before an observation-write
  failure; the failure states that trend observation persistence failed.
- Add package-level Go docs for new exported types and stable error/status
  categories where the repository convention requires them.
- Provide endpoint examples and metric-state definitions in the Phase 26 docs;
  no separate public SDK documentation is needed for an internal admin route.
- Test error strings/codes that operators and scripts consume, avoiding raw
  storage errors in HTTP responses.
- Avoid new service setup, database migration, environment variable, or
  dependency installation requirements.

### DX Scorecard

| Dimension | Before | Planned | Reason |
| --- | --- | --- | --- |
| Existing eval workflow | 8/10 | 9/10 | Reports remain stable and missing observation is explicit. |
| Admin API discoverability | 5/10 | 8/10 | One named route with bounded query contract and docs. |
| Failure diagnosis | 6/10 | 9/10 | Controlled source states and post-report CLI failure. |
| Setup burden | 9/10 | 9/10 | Reuses admin data directory and file-store pattern. |

## Explicit Test Plan

| Layer | Tests |
| --- | --- |
| `internal/admin` observations | validation, required fields, controlled status/category, append idempotency key, clone safety, file temp-write/reopen, concurrent append, tenant-scoped list, malformed file behavior |
| `internal/admin` trends | default date range, valid/invalid/reversed/oversized dates, DST/local midnight, all-space selection, source cap/truncation, mixed tenants/spaces, completed/pass/fail attempts, first-pass median/exclusions, current stale snapshot, recurrence timestamps/rate, small/empty/unavailable states, rank ties, capped traces/buckets, malformed entry exclusion |
| `cmd/cli` | static cases write no observations; promoted pass/fail writes exactly one; skipped/missing required RAG writes unavailable; reports precede write failure; repeated run is idempotent |
| `internal/server` | protected route, derived tenant, query parsing, `400` filter, `503` unavailable, no raw cause, JSON shape, space isolation |
| `web` static | new route string, panel IDs, date/space labels, refresh control, state labels, `textContent` rendering, existing Repair Inbox contracts remain present |
| regression | `go test ./...`, `go vet ./...`, `golangci-lint run ./...`, CLI eval smoke run with promoted fixtures |

## Rollout And Rollback

Release is additive. Existing repair ledgers, promoted eval reports, and release
gates remain valid if the new panel is absent or the endpoint is disabled.
Rollback removes the new handler/UI wiring and stops observation writes; it does
not rewrite observation, verification, recurrence, promotion, or report data.

## Review Audit Trail

| Decision | Classification | Outcome | Reason |
| --- | --- | --- | --- |
| Explicit eval observation ledger | Mechanical | Accepted | Required for an authoritative business timestamp. |
| One projection service in admin | Mechanical | Accepted | Reuses tenant-scoped stores and avoids eval-to-admin import. |
| Dedicated bounded trend-source reader | Mechanical | Accepted | Existing inbox/history readers default a space or cap history, which would silently undercount a trend. |
| 30-day default, 90-day maximum | Mechanical | Accepted | Satisfies scope while bounding local file-store work. |
| Text/table buckets instead of chart | Taste | Accepted | Better expresses sparse/zero/unavailable observations without visual overclaim. |
| Put panel beside Repair Inbox | Taste | Accepted | Reuses operator context and avoids analytics navigation. |
| No composite quality score | User-approved premise | Accepted | Avoids an unsupported product claim. |

## Implementation Tasks

- [x] Add observation data contract, stores, and tests.
- [x] Add CLI observation extraction/writer and tests.
- [x] Add trend filter, projection, aggregation service, and tests.
- [x] Wire server construction, handler, route, and handler tests.
- [x] Add admin panel, renderer, styling, and static web tests.
- [x] Run required regression, vet, syntax, and smoke verification.

## Stage 3 Completion

Implemented on `codex/phase-26-knowledge-quality-trends-spec` with RED-first
tests for the observation store, all-space source reads, DST-safe filters,
verification/recurrence/eval projections, HTTP behavior, CLI persistence, and
admin static assets. The local workstation does not provide `golangci-lint`, and
`go test -race` is unsupported by its `windows/386` toolchain; `go vet` and all
non-race tests pass.

## Unresolved Decisions

None blocking. The server's local timezone will be explicit at composition time
and tested through injected locations; it is not an end-user configurable
setting in Phase 26.

## GSTACK REVIEW REPORT

### Review Completion

- CEO review: complete. Scope is coherent after rejecting score/report-scan
  shortcuts; no user challenge remains after Stage 1 approval.
- Design review: complete. UI scope exists and the approved panel states,
  placement, accessibility, and sparse-data treatment are specified.
- Engineering review: complete. Package boundaries, data flow, failure modes,
  task order, and RED-first test matrix are explicit.
- DX review: complete. CLI/API operational behavior and no-new-setup guarantee
  are documented.
- Independent Codex voice: unavailable for this run. The read-only CLI review
  exceeded the 184-second execution limit and returned no findings; its absence
  is not treated as confirmation. The local review added the dedicated bounded
  source-reader requirement after inspecting existing list limits.

### Approval Gate

Stage 2 and Stage 3 are complete. Stage 4 review completed with two local fixes.
Stage 5 local HTTP QA completed; staging/browser QA remains unavailable without
a staging URL. Stage 6 security review found one HIGH inherited admin-auth
finding documented in [the security report](../security/phase-26-knowledge-quality-trends-security.md).
The current-stage disposition is to defer the auth change while keeping the
finding visible; Stage 7 ship remains blocked for public deployment until the
deployment is explicitly accepted as local-only or the auth remediation is
approved.
