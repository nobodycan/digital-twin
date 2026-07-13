<!-- /autoplan restore point: C:\Users\HW\.gstack\projects\digital-twin\codex-phase-28-knowledge-quality-review-checkpoints-autoplan-restore-20260713-214700.md -->

# Phase 28 Knowledge Quality Review Checkpoints Plan

Date: 2026-07-13

Status: Ready for Stage 2 approval; implementation not started

Source spec: [Phase 28 Knowledge Quality Review Checkpoints Spec](../specs/phase-28-knowledge-quality-review-checkpoints.md)

Source design: [Phase 28 Knowledge Quality Review Checkpoints Design](../design/phase-28-knowledge-quality-review-checkpoints.md)

Base branch: `main`

## Initial Plan

Add an append-only, tenant-scoped quality review checkpoint over the existing
Quality Trends projection. The server recomputes a safe metric snapshot, validates
the operator's explicit outcome and bounded repair-gap references, persists the
record through existing local ledger conventions, and exposes bounded history plus
strictly compatible comparisons in the existing admin surface.

Stage 2 must lock the data contract, store semantics, HTTP errors, comparison
rules, interaction states, implementation order, and RED-first test matrix. It
must not introduce a quality score, automatic repair mutation, reviewer identity,
background work, external infrastructure, or Docker.

## Autoplan Verdict

Phase 28 remains the right next product slice. The CEO review found that exact
`from`/`to` matching supports repeated annotation of one historical lens,
not periodic review of later windows. D1 selected equivalent earlier-window
comparison, so Phase 1 is complete and Phase 2 may proceed.

- Review mode: `SELECTIVE EXPANSION`.
- UI scope: detected; Phase 2 design review is required after D1.
- API/DX scope: detected; Phase 3.5 DX review is required.
- Implementation status: no production code has been written.
- External voice status: degraded; details are recorded below.

## Proposed Premises

All six premises are approved.

1. The operator needs a durable human conclusion over observed evidence, not a
   synthetic quality score.
2. The server, not the browser, must recompute and select every persisted metric.
3. A checkpoint is application-immutable and append-only; it never owns or
   mutates repair lifecycle state.
4. A stable, versioned allow-list snapshot is required. Persisting the complete
   `QualityTrendProjection` would couple history to future projection changes.
5. Risk acceptance without a trustworthy person identity is an unattributed
   local decision, not a compliance approval or attestation.
6. Periodic comparison uses the compatible checkpoint with the latest strictly
   earlier window end and the same tenant, space scope, timezone, and
   local-calendar window-day count. Both periods and denominators stay visible.

## CEO Review

### 0A. Premise Challenge

The core problem is supported: Phase 26 can show observed evidence, but it does
not preserve what an operator concluded or why. NIST's AI RMF calls for ongoing
monitoring, periodic review, documented human oversight, and mechanisms that
track risk over time. OWASP's logging guidance similarly favors chronological,
purpose-limited records and warns against collecting unnecessary sensitive
data. Those principles support a bounded decision ledger and the proposed
content exclusions:

- <https://airc.nist.gov/airmf-resources/airmf/5-sec-core/>
- <https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html>

The exact-filter comparison premise does not survive first-principles review.
With a fixed historical range, most event counts cannot change after the source
period has closed. With the default rolling 30-day UI, the next review has new
dates and therefore no compatible predecessor. The result would usually be
either a zero delta over the same old evidence or no comparison at all.

A second trust premise needs precise wording. The file ledger can prevent edits
through the application API, but it is not tamper-evident against direct local
file modification. Phase 28 must say `application-immutable`, not imply signed
or independently verifiable audit storage.

### 0B. What Already Exists

| Sub-problem | Existing leverage | Phase 28 decision |
| --- | --- | --- |
| Honest trend evidence | `QualityTrendService.Project` and typed metric states in `internal/admin/knowledge_quality_trends.go:10-190` | Recompute once for a previously unseen idempotency key; replay the original checkpoint for an unchanged retry. |
| Tenant and space gap lookup | `KnowledgeGapService.Get/ListAll` in `internal/admin/knowledge_phase14.go:278-292` | Validate every selected ID through the tenant-derived service boundary. |
| Local durable stores | In-memory and file stores in verification, recurrence, promotion, and eval observation modules | Reuse mutex, clone, temp-file, close/rename, and reopen patterns; add explicit file sync and qualify cross-platform replacement guarantees. |
| Admin authorization | Phase 27 admin route boundary in `internal/server/server.go:208-220` | Register both routes under `/admin/`; add no bypass or new credential. |
| Thin trend HTTP route | `handleKnowledgeQualityTrends` in `internal/server/server.go:1014-1031` | Keep checkpoint handlers as transport adapters over typed service errors. |
| Existing operator context | Repair Inbox and Quality Trends in `web/admin.html:150-184` | Add one nested review surface, not new navigation. |
| Safe browser rendering | Trend rows use `textContent` in `web/admin.js:1292-1327` | Render rationale and IDs as text only. |
| Static browser contracts | `web/app_static_test.go` asserts routes, controls, labels, and safe rendering | Extend the existing static contract suite. |

The primary leverage gap is that `QualityTrendProjection` has no explicit
`projected_at`, local date strings, or timezone name. Phase 28 needs an
additive self-describing projection context so the checkpoint can distinguish
evidence observation time from checkpoint creation time.

### 0C. Dream State Delta

    CURRENT
      bounded operational evidence
      -> no durable interpretation
      -> every visit starts from memory

    PHASE 28
      loaded evidence + explicit human outcome
      -> server-recomputed Snapshot V1
      -> application-immutable checkpoint
      -> bounded history and an honest prior-review relationship

    12-MONTH IDEAL
      review policies + trustworthy actor identity + accountable exceptions
      -> scheduled review cadence and ownership
      -> alerts and workflow only after real usage proves the need

Phase 28 should create the stable decision primitive needed by that future. It
should not pre-build assignment, notifications, approvals, or scoring.

### 0C-bis. Implementation Alternatives

| Option | Product result | Effort | Completeness | Risk | Decision |
| --- | --- | --- | --- | --- | --- |
| A. Equivalent-window checkpoint ledger | Compare the current review with the latest earlier review for the same tenant, scope, timezone, and local-calendar window width; show both windows and denominators | Human: about 3 days; Codex: about 5-7 hours | 10/10 | Requires careful labels and predecessor selection | Selected at D1 |
| B. Checkpoint history without metric deltas | Preserve safe evidence and decisions but make no numerical cross-review claim | Human: about 1.5 days; Codex: about 2-3 hours | 8/10 | Lower immediate decision support, but fully honest | Safe fallback |
| C. Exact-date predecessor | Implement the approved Stage 1 wording literally | Human: about 2 days; Codex: about 3-4 hours | 5/10 | Comparison is normally absent or repeats the same evidence | Not recommended |

Option A is the approved highest-completeness solution. Its predecessor must have an
earlier window end, the same local-calendar day count, and the same
tenant/space/timezone. Overlapping rolling windows are allowed but explicitly
labeled `review-to-review difference`, never improvement, regression, or
causal impact. An exact repeated window remains visible in history but is not a
periodic predecessor.

### 0D. Selective Expansion Decisions

| Candidate | Decision | Classification | Reason |
| --- | --- | --- | --- |
| Explicit `QualityReviewSnapshotV1` and `schema_version` | Include | Mechanical | Prevents persistence from inheriting trace, text, bucket, and future DTO fields. |
| `projected_at`, canonical local dates, timezone, and window-day count | Include | Mechanical | Makes the stored evidence scope self-describing and comparable. |
| Required client idempotency key with same-request replay | Include | Mechanical | Prevents double-click and network retry from creating duplicate decisions. |
| Checkbox/radio selection from currently visible eligible gap IDs | Include | Mechanical | Avoids typo-prone arbitrary ID entry while retaining server validation. |
| Ledger capacity and request/body limits | Include | Mechanical | Keeps a low-frequency local ledger bounded under accidental or abusive writes. |
| Tamper-evident hash chaining or signatures | Defer | Outside current trust claim | Valuable only with a defined threat model, key custody, and verification workflow. |
| Named reviewer, roles, and approval chains | Defer | Missing identity foundation | A free-form name would create false attribution. |
| Schedules, notifications, assignment, and automatic remediation | Defer | Separate workflow product | Not required to record one review decision. |
| Composite score, red/green direction, or automatic conclusion | Reject | Contradicts evidence model | Would overstate partial and differently sampled observations. |

For `observe`, zero to ten selected gap IDs are permitted and rationale is
optional. `repair_required` and `risk_accepted` require one to ten selected
IDs and a rationale. All selected IDs remain historical references only.

### 0E. Temporal Interrogation

| Stage 3 elapsed time | Expected proof, not implementation commitment |
| --- | --- |
| Hour 1 | RED store/model tests establish Snapshot V1, validation, clone safety, idempotency, ordering, and reopen behavior. |
| Hours 2-3 | RED service tests establish server recomputation, tenant/space validation, predecessor rules, pure delta helpers, and failure preservation. |
| Hour 4 | RED handler and composition tests establish route protection, request limits, typed errors, and startup wiring. |
| Hours 5-6 | RED static-web tests establish loaded-filter invalidation, selectable IDs, save/retry states, safe history, and accessible labels. |
| Hour 6+ | Full regression, lint, race-sensitive store checks, docs, browser QA, security review, and ship verification. |

If Stage 3 starts by editing `server.go` or `admin.js` before the domain tests
are red, the implementation sequence has drifted from the approved TDD policy.

### 0F. Mode Confirmation

`SELECTIVE EXPANSION` is correct. The versioned snapshot, idempotency,
self-describing time context, safe selector, and bounded ledger are all inside
the feature's blast radius and prevent foreseeable failure. Workflow,
identity, tamper evidence, alerts, and scoring remain separate products.

## CEO Dual Voices

The required external voices were attempted and degraded honestly:

- Claude CLI `2.1.175` is installed, but neither Claude credentials nor
  `ANTHROPIC_API_KEY` is available, so no Claude output was produced.
- Independent Codex CLI authenticated successfully but exceeded the 244-second
  read-only command timeout and produced no review.
- A fresh-context read-only subagent was also attempted, remained running
  without a result through the review window, and was closed.

This phase therefore completed in `single-reviewer` mode. No unavailable voice
is counted as agreement.

| Dimension | Claude | Independent Codex | Fresh subagent | Primary review |
| --- | --- | --- | --- | --- |
| Premises valid | N/A | N/A | N/A | Exact-date premise challenged |
| Right problem | N/A | N/A | N/A | Confirmed |
| Scope calibration | N/A | N/A | N/A | Confirmed after selective additions |
| Alternatives explored | N/A | N/A | N/A | Three executable options |
| Landscape risk | N/A | N/A | N/A | False compliance and false comparison flagged |
| Six-month trajectory | N/A | N/A | N/A | Sound with Snapshot V1 and D1 resolution |

Consensus count is `0/6` because all external voices are unavailable, not
because they disagreed. The user resolved the primary premise challenge by
selecting Option A.

## Review Sections

### Section 1. Architecture Review

The clean boundary is one checkpoint service in `internal/admin`, backed by a
small store interface and consuming narrow trend-projector and gap-reader
interfaces. The browser and HTTP handler never construct snapshots or deltas.

    browser loaded review context
             |
             v
    POST checkpoint handler -- derived tenant
             |
             v
    QualityReviewCheckpointService
       |-- QualityTrendProjector.Project once
       |-- KnowledgeGapReader.Get for <= 10 IDs
       |-- SnapshotV1 allow-list mapper
       |-- predecessor + pure delta projector
       +-- QualityReviewCheckpointStore.Append
                         |
                         v
               local JSON ledger

    GET history handler -- derived tenant + scope + limit <= 20
             |
             v
    service -> store list -> safe records + comparison metadata

The service must not persist `QualityTrendProjection` directly. It stores only
the explicit V1 fields required for review. The new projection context is
additive to Phase 26. A hard store-record or file-size ceiling is required so
the temp-file rewrite remains bounded. No cache or background worker is needed.

Rollback is a code revert: the additive ledger file remains ignored by old
code. The feature is reversible, with no migration and no mutation of existing
ledgers.

### Section 2. Error And Rescue Review

Every failure before append returns without a checkpoint. Every append failure
returns without repair mutation. Exact typed errors must cross the
admin-to-server boundary; the new handlers must not classify errors by matching
message strings. The consolidated registry appears below.

The only graceful partial result is a syntactically valid ledger envelope with
one invalid record: skip that record, increment `excluded_count`, and never
compute a delta from it. Invalid JSON or an unsupported top-level schema makes
history unavailable rather than returning a plausible partial ledger.

### Section 3. Security And Threat Model

| Threat | Likelihood | Impact | Planned mitigation |
| --- | --- | --- | --- |
| Client submits forged metrics or tenant ID | Medium | High | Request DTO accepts neither; server derives tenant and recomputes projection. |
| Cross-tenant or cross-space gap reference | Medium | High | Resolve every ID through tenant-scoped lookup and return one non-enumerating conflict code. |
| Stored XSS through rationale or IDs | Medium | High | Bounded plain text, safe ID grammar, JSON encoding, and DOM `textContent` only. |
| Request/disk exhaustion | Medium | Medium | Body, rationale, ID count, history, record count, and file size caps plus existing admin rate limit. |
| Local file is presented as tamper-proof | Medium | High | Call it application-immutable; defer cryptographic integrity claims. |
| Unattributed `risk_accepted` is treated as approval | Medium | High | UI and docs say local unattributed decision, not approval; no claimed reviewer field. |
| Sensitive projection text enters history | Low | High | Snapshot V1 compile-time allow-list; absence tests for question, source, trace, diagnostics, and evaluator text. |

No dependency, secret, network egress, template execution, SQL, command
execution, or LLM prompt surface is added.

### Section 4. Data Flow And Interaction Edge Cases

    INPUT
      filter + outcome + rationale + gap_ids + idempotency_key
        |
        +-- missing/oversized/invalid -> typed 400, no projection, no write
        v
    PROJECT
      derived tenant -> normalized filter -> safe evidence
        |
        +-- dependency error -> typed 503, no write
        v
    VALIDATE LINKS
      <= 10 tenant/scope-compatible gaps
        |
        +-- absent/wrong scope -> same 409, no write
        v
    MAP + APPEND
      SnapshotV1 -> idempotent bounded store
        |
        +-- same key/same request -> prior success
        +-- same key/different request -> 409
        +-- storage failure/capacity -> controlled unavailable
        v
    OUTPUT
      checkpoint + predecessor state + safe deltas

| Interaction | Edge case | Resolution |
| --- | --- | --- |
| Trend review | Operator edits from/to/space after load | Immediately invalidate the loaded context and disable save until refresh succeeds. |
| Save | Double click or retry after lost response | Disable while in flight and reuse one idempotency key until success. |
| Save | Outcome changes requirements | Revalidate rationale and selected IDs in browser for guidance and again on server for authority. |
| Save | Trend source changes after browser load | Server recomputation wins; response shows the persisted `projected_at`. |
| Gap selection | No visible eligible IDs | Disable repair/risk save with an explanatory message; observe remains possible. |
| History | Zero records | Explicit empty state, not success language. |
| History | One malformed record | Exclude it, show a neutral incomplete-history notice, and compute no delta from it. |
| Filter refresh | Request fails | Keep old values visible but mark them stale and keep checkpoint save disabled. |

### Section 5. Code Quality Review

Add one focused domain file and test file rather than growing the handler with
validation and comparison branches. Use consumer-owned interfaces for trend
projection, gap lookup, and storage; use pure helpers for normalization,
Snapshot V1 mapping, compatibility, and delta calculation.

Do not duplicate the date parser. The checkpoint service consumes the
normalized filter returned by the trend projector. New server code maps typed
sentinel errors instead of copying the existing string-matching pattern at
`internal/server/server.go:1023`. JavaScript retains the current vanilla DOM
style; no framework or generic form abstraction is justified.

### Section 6. Test Review

    NEW UX FLOWS
      load -> decide -> select -> save -> history
      filter change -> invalidated -> reload
      save failure -> retained form -> idempotent retry

    NEW DATA FLOWS
      request -> projection -> gap validation -> SnapshotV1 -> append
      history query -> scoped list -> predecessor -> delta DTO

    NEW CODEPATHS
      3 outcomes
      optional/required rationale
      0/1/10/11 IDs
      same/different idempotency replay
      prior/no-prior/non-comparable metric
      valid/malformed/corrupt ledger

    NEW ASYNC OR EXTERNAL WORK
      none

The Friday-night confidence test is a mixed-tenant file-store reopen test that
creates a checkpoint through HTTP, retries it, restarts the store, lists only
the intended scope, and proves no raw projection fields or repair mutations
exist. The hostile test submits forged tenant/metrics, duplicate and cross-scope
IDs, 501-rune rationale, oversized JSON, stale browser context, and a conflicting
idempotency key. The chaos test injects temp creation, sync, close, and rename
failures and proves the prior ledger remains readable.

The final explicit RED-first matrix is a Phase 3 deliverable, but it must include
unit, file-store, service, handler/composition, static-web, full regression, vet,
lint, and race-sensitive concurrency coverage.

### Section 7. Performance Review

Checkpoint creation intentionally pays for one server-side trend recomputation;
trusting a client snapshot is not an acceptable optimization. It validates at
most ten gaps and performs one serialized best-effort whole-file replacement.
History never recomputes
trends and returns at most twenty records.

The store must cap total records or serialized bytes before append. Comparisons
should be computed in one newest-first pass with indexed in-memory scope keys,
not nested scans per checkpoint. No cache is needed at current local volume.
The first 10x pressure point is the file rewrite; the deferred database or
segmented-log migration should occur only if real checkpoint volume approaches
the cap.

### Section 8. Observability And Debuggability Review

Stable HTTP error codes, the existing request ID, `projected_at`,
`created_at`, `schema_version`, and history `excluded_count` provide the
minimum reconstruction surface. Storage errors may be logged with operation,
tenant-safe scope, request ID, and checkpoint ID, but never rationale, raw
metrics payloads, question text, credentials, or admin tokens.

No alert, dashboard, or background health job is justified for a low-frequency
local feature. The browser must show whether failure occurred while loading
evidence, validating links, saving, or loading history instead of one generic
`failed` message.

### Section 9. Deployment And Rollout Review

The rollout is additive: compose the new store and service from the existing
admin data directory, register protected routes, then expose the UI. No database
migration, feature flag, Docker image, credential, or startup order change is
required.

Post-deploy smoke checks create one `observe` checkpoint, retry the same
idempotency key, reload history, restart the server, and verify the record
survives exactly once. Rollback removes route/UI wiring; old binaries ignore the
new file and all repair/eval behavior remains unchanged.

### Section 10. Long-Term Trajectory Review

Reversibility is `5/5` if Snapshot V1 and the application-immutable wording are
used. The main path dependency is comparison identity. A stable scope key and
window semantics let later phases add cadence policies without rewriting old
records; exact dates as the only key would strand history in isolated lenses.

Deferred named identity must be added before this becomes an approval system.
Deferred tamper evidence needs a canonical record format and key-management
decision. Neither should be faked now. The V1 envelope, typed errors, and pure
comparison projector make those future additions explicit rather than hidden.

### Section 11. Design And UX Review

The feature stays inside Quality Trends and follows this order:

    choose filter -> load observed evidence -> review labels
      -> choose human outcome -> select visible gap IDs -> explain
      -> save application-immutable checkpoint
      -> read newest history and prior-review difference

| Surface | Loading | Empty | Error | Success | Partial/stale |
| --- | --- | --- | --- | --- | --- |
| Trends | Save disabled | Evidence empty, observe allowed after load | Prior values marked stale, save disabled | Loaded scope chip and projected time | Withheld metrics retain status |
| Review form | Save disabled | No eligible IDs message | Inputs retained | New idempotency key after save | Filter edit invalidates form |
| History | Existing rows retained | No checkpoints for this scope | Controlled history error | Newest first, max 20 | Excluded-record notice |
| Comparison | Not shown | No prior comparable review | Never fabricate delta | Both windows, denominators, neutral signed change | Non-observed metric says not comparable |

Use a fieldset with native outcome radios, a 500-character textarea counter,
and bounded checkboxes generated only from visible eligible IDs. The immutable
and no-repair-mutation note sits beside the save control, not in distant help
text. Keyboard focus moves to the live success/error status. On narrow screens,
filters, decision controls, and history rows stack without horizontal scrolling.
No red/green quality color, score badge, chart, modal, or new top-level
navigation is introduced.

## NOT In Scope

- Quality, correctness, confidence, severity, or risk scores.
- Automatic repair mutation, recurrence confirmation, promotion, deactivation,
  assignment, notification, schedule, SLA, or alert.
- Named reviewer, role, approval chain, signature, or formal risk attestation.
- Edit, delete, supersede, revoke, or redact APIs for checkpoints.
- Cryptographic hash chains, signatures, remote WORM storage, or compliance
  claims.
- Cross-tenant comparison, cross-space comparison, mixed window-width
  comparison, causal improvement language, or inferred conclusions.
- Raw question, answer, source, trace, diagnostic, evaluator, credential, or
  admin-token storage.
- Export, dashboard aggregation, database migration, external service, Docker,
  or background job work.

The Phase 3 engineering review promoted the durable deferrals into
`TODOS.md`; none is part of the active implementation scope.

## Error And Rescue Registry

| Method or path | Failure | Typed result | Rescue action | User sees |
| --- | --- | --- | --- | --- |
| POST decoder | Invalid or oversized JSON | invalid request | Bound body and stop before service | Specific invalid/too-large message |
| Request normalizer | Invalid filter/outcome/rationale/IDs/key | invalid request | Return all field violations deterministically | Correctable form guidance |
| Trend projector | Dependency/read failure | unavailable | No append; retain form | Evidence unavailable, retry |
| Gap validator | Missing, foreign, or wrong-space ID | gap conflict | Same non-enumerating response | Selected repair is not eligible |
| Store append | Same key and same canonical request | replay | Return original checkpoint | Already saved, no duplicate |
| Store append | Same key and different request | idempotency conflict | No append | Refresh/retry with a new decision |
| Store append | Temp/sync/close/rename failure | storage unavailable | Keep prior file; no repair mutation | Checkpoint was not saved |
| Store append | Capacity reached | capacity unavailable | Reject before rewrite | Local checkpoint capacity reached |
| Store list | Valid envelope with invalid entry | excluded record | Skip and count; no delta from it | History incomplete notice |
| Store list | Corrupt JSON or unsupported envelope | storage unavailable | Fail closed; return no partial history | History temporarily unavailable |
| Browser load/save | Network or HTTP failure | UI state error | Preserve filters/form; disable unsafe save | Stage-specific error and retry |

## Failure Modes Registry

| Failure mode | Severity | Prevention and proof |
| --- | --- | --- |
| Periodic review normally has no predecessor | P0 resolved | D1 selected equivalent earlier windows; Phase 3 must prove predecessor rules. |
| Cross-tenant checkpoint or gap leak | P0 | Server-derived tenant, scoped lookup/list/compare, mixed-tenant tests. |
| Checkpoint write mutates repair/eval state | P0 | No mutator dependency; before/after regression assertions. |
| Future trend DTO field silently enters history | P1 | Explicit Snapshot V1 mapping and forbidden-field tests. |
| Double click or retry creates duplicates | P1 | Required idempotency key and replay/conflict tests. |
| Filter changes after evidence load | P1 | Loaded-context token, change invalidation, UI tests. |
| Non-observed metric becomes zero or direction | P1 | Status-aware delta helper and withheld-state tests. |
| Different timezone or window width is compared | P1 | Canonical compatibility key and DST/calendar tests. |
| Risk acceptance appears formally approved | P1 | Unattributed-local-decision copy and no reviewer field. |
| Malformed history produces favorable partial data | P1 | Exclusion or fail-closed rules; no comparison from excluded rows. |
| Authorized client fills local disk | P1 | Body, count, history, record, and file-size caps. |
| Rationale executes or leaks through logs | P1 | `textContent`, bounded plain text, redacted structured logging tests. |

## CEO Implementation Tasks

Synthesized from the CEO findings. Phase 3 must refine these into the final TDD
sequence; none authorizes production code before the Stage 2 gate.

- [x] **T1 (P1, human: about 1h / Codex: about 10min)** - Product contract -
  Resolve D1 and synchronize comparison semantics.
  Surfaced by: 0A - exact dates do not support the stated periodic-review job.
  Files: `docs/specs/phase-28-knowledge-quality-review-checkpoints.md`,
  `docs/design/phase-28-knowledge-quality-review-checkpoints.md`, this plan.
  Verify: no source artifact describes a different predecessor rule.
- [ ] **T2 (P1, human: about 4h / Codex: about 45min)** - Domain contract -
  Add Snapshot V1 and self-describing projection context.
  Surfaced by: Architecture - the current projection has no projected time,
  canonical local dates, or timezone and must not be persisted wholesale.
  Files: `internal/admin/knowledge_quality_trends.go`, new checkpoint domain
  and tests.
  Verify: focused admin tests prove the allow-list and forbidden fields.
- [ ] **T3 (P1, human: about 5h / Codex: about 1h)** - Persistence - Build an
  idempotent, capacity-bounded, single-process application-immutable ledger
  with honest best-effort whole-file replacement semantics.
  Surfaced by: Data Flow, Security, and Performance - retries, corruption, and
  unbounded file rewrites otherwise break trust.
  Files: new `internal/admin` checkpoint store and tests.
  Verify: reopen, race-sensitive concurrency, replay/conflict, corruption, and
  injected write-failure tests.
- [ ] **T4 (P1, human: about 3h / Codex: about 30min)** - HTTP/composition -
  Derive tenant, bound requests, validate links, and map typed errors.
  Surfaced by: Error and Security reviews - no client metric/tenant trust or
  message-string error classification.
  Files: `internal/server/server.go`, `cmd/server/main.go`, focused server
  and composition tests.
  Verify: mixed-tenant handler tests and full existing admin-access regression.
- [ ] **T5 (P1, human: about 4h / Codex: about 45min)** - Admin UX - Build the
  loaded-context review form and safe history state machine.
  Surfaced by: Section 11 - filter edits, double submits, no eligible gaps, and
  history failures need explicit states.
  Files: `web/admin.html`, `web/admin.js`, `web/app.css`,
  `web/app_static_test.go`.
  Verify: static contracts plus Stage 5 keyboard, narrow-screen, retry, and
  safe-render browser QA.
- [ ] **T6 (P2, human: about 1h / Codex: about 10min)** - Documentation - State
  the trust boundary and deferred capabilities without compliance overclaim.
  Surfaced by: Security and Long-Term reviews - application immutability and
  unattributed risk decisions are narrower than formal audit approval.
  Files: Phase 28 docs and `README.md`.
  Verify: docs contain no tamper-proof, named-approval, score, Docker, or repair
  mutation claim.

The CEO JSONL task artifact was not written because `jq` is unavailable.
Per the review skill, the fallback is to skip rather than hand-roll JSONL.

## Decision Audit Trail

| # | Decision | Classification | Principle | Outcome |
| --- | --- | --- | --- | --- |
| 1 | Keep a human checkpoint layer over Phase 26 | Premise | Complete product | Accepted |
| 2 | Challenge exact-date comparison | Premise gate | Correctness over literalism | Option A approved |
| 3 | Use Snapshot V1 instead of serializing projection | Mechanical | Simpler stable boundary | Accepted |
| 4 | Add projected time and canonical timezone/window context | Mechanical | Self-describing evidence | Accepted |
| 5 | Add required idempotency | Mechanical | Complete edge coverage | Accepted |
| 6 | Use visible selection controls plus server validation | Mechanical | Prevent avoidable user error | Accepted |
| 7 | Bound total local ledger growth | Mechanical | Safe local operation | Accepted |
| 8 | Describe immutability honestly; defer tamper evidence | Scope | Avoid false assurance | Accepted |
| 9 | Keep risk acceptance unattributed and non-approving | Scope | Do not invent identity | Accepted |
| 10 | Reject workflow, scoring, alerts, and Docker | Scope | Outside immediate job | Accepted |
| 11 | Treat the Go backend/full-stack maintainer as the primary developer persona | DX | Infer from README, Go module, and admin API surface | Accepted |
| 12 | Deliver the first checkpoint/replay as copy-paste PowerShell and curl examples | DX taste | Lowest-effort path to visible value | Accepted |
| 13 | Add stable machine codes plus safe message, hint, and request correlation | DX | Fight uncertainty without leaking resource details | Accepted |
| 14 | Do not add a seed command; use a valid empty-window `observe` checkpoint for TTHW | DX taste | Simpler complete path; fixtures remain tests/QA | Accepted |
| 15 | Document Windows and Linux equivalent commands without adding Docker or parallel script suites | DX | Meet contributors where they work with low scope | Accepted |
| 16 | Defer OpenAPI, SDKs, docs site, and public-community infrastructure | DX scope | Internal local API does not justify ecosystem expansion | Accepted |

## CEO Completion Summary

| Item | Result |
| --- | --- |
| Right problem | Yes: preserve the operator's interpretation of bounded evidence. |
| Mode | Selective expansion. |
| Accepted additions | Snapshot V1, projected context, idempotency, safe selector, bounded ledger. |
| Deferred systems | Identity/approvals, workflow, tamper evidence, alerts, exports, database. |
| Critical open issue | None; D1 selected equivalent earlier-window comparison. |
| External voices | Unavailable after auth, timeout, and no-result degradation; single-reviewer mode. |
| Reversibility | 5/5 with additive file and no lifecycle mutation. |
| Ready for Phase 2 | Yes; D1 passed with Option A. |

Phase 1 is complete. Independent Codex produced no result before timeout,
Claude produced no result because authentication is absent, and the fresh
subagent produced no result before shutdown. Consensus is therefore 0/6
external dimensions. The user resolved the primary-review premise challenge by
selecting Option A.

## Premise Gate D1

Passed. The user selected `A`: compare the compatible checkpoint with the
latest strictly earlier window end and the same tenant, space scope, timezone,
and local-calendar window width. Both windows and rate denominators remain
visible, and the UI uses neutral `review-to-review difference` language.

The source spec and design have been synchronized with this decision.

## Design Review

### Design Scope Assessment

Initial design completeness was `7/10`. The CEO review already defined the
main journey, safety copy, and a state matrix, but implementation could still
collapse evidence, decision, comparison, and history into the existing single
`#quality-trends-body` text stream. A 10/10 plan needs a named hierarchy,
separate region states, exact comparison grammar, candidate provenance, focus
behavior, and breakpoint-specific rules.

No `DESIGN.md` or gstack design binary is available. Visual mockups were
therefore skipped and this review uses the repository's implemented UI as the
de facto design system. The reviewed plan reaches `9/10` overall; the missing
point is the absence of a rendered mockup/formal design system, not an
unresolved implementation decision.

### Existing Design Leverage

| Existing pattern | Location | Reuse |
| --- | --- | --- |
| Warm neutral surfaces and teal accent variables | `web/app.css:1-15` | Use existing variables; add no checkpoint palette. |
| IBM Plex Sans and Aptos Display hierarchy | `web/app.css:21-29,103-109` | Keep current fonts and heading weight. |
| Native controls with disabled state | `web/app.css:32-55` | Reuse buttons, dates, radios, checkboxes, textarea, and disabled semantics. |
| Admin two-column to one-column shell | `web/app.css:527-543,809-819` | Keep page layout; make the checkpoint flow one vertical workspace inside Quality Trends. |
| Action rows and bordered operational rows | `web/app.css:558-563,678-703` | Reuse compact spacing and hairline separation. |
| Live trend status and safe `textContent` rendering | `web/admin.html:182-183`, `web/admin.js:1292-1344` | Add separate live regions and keep all stored strings as text nodes. |
| Static DOM contract tests | `web/app_static_test.go` | Assert every new landmark, ID, route, and safety label. |

The established interface is compact and utility-first. Phase 28 must not add a
dashboard-card mosaic, decorative icon system, gradient, modal workflow, or
chart.

### Design Dual Voices

- Claude remains unavailable because local Claude authentication is absent.
- The fresh-context subagent did not return within 120 seconds and was closed.
- Independent Codex completed after network retries and raised nine actionable
  concerns: missing four-part hierarchy, region-specific states, post-save
  focus, task-specific labels, candidate provenance, exact comparison grammar,
  visible dirty-filter feedback, breakpoint rules, and executable accessibility
  requirements.

Independent Codex recommended showing eligible and ineligible candidate groups.
The primary review rejects the ineligible group: out-of-scope or non-visible
gaps are not part of the current evidence review and should not be disclosed or
explained. The selected design shows only evidence-derived eligible candidates,
states their source, and gives one explicit no-candidate state.

### Design Litmus Scorecard

| Litmus check | Independent Codex before fixes | Planned result | Resolution |
| --- | --- | --- | --- |
| Product purpose is clear in the section | No | Yes | Four named regions and a loaded-scope anchor. |
| One strong visual anchor exists | No | Yes | Loaded evidence context strip with scope, dates, timezone, and projected time. |
| Headings alone explain the flow | No | Yes | Evidence, Record decision, Latest comparison, Review history. |
| Every region has one job | No | Yes | Separate DOM and status regions; no shared catch-all body. |
| Card boundaries are necessary | Yes | Partly | Only immutable history `details` rows earn record boundaries; metrics stay rows. |
| Motion improves the task | No | No | No animation; native disclosure and focus are sufficient. |
| Hierarchy works without decorative shadows | Yes | Yes | Typography, whitespace, hairlines, and state labels carry hierarchy. |

External consensus is `6/7` on the problem/fix direction. The only partial
disagreement is card use, resolved as interactive record disclosures rather
than a card grid.

### Pass 1. Information Architecture: 6/10 -> 10/10

The existing Quality Trends section becomes one vertical review workspace with
four independently addressable regions:

    Knowledge Quality Trends
      |
      +-- 1. Observed evidence
      |     filters -> refresh -> loaded evidence context -> metric rows
      |
      +-- 2. Record review decision
      |     outcome -> rationale -> evidence-derived gap candidates -> save
      |
      +-- 3. Latest review comparison
      |     persisted current checkpoint <-> compatible earlier checkpoint
      |
      +-- 4. Review history
            newest open details row -> older collapsed details rows, max 20

The loaded context strip is the visual anchor and reads in this order:
`space or all spaces`, `from - to`, `timezone`, `N calendar days`,
`evidence projected at`. It is absent before a successful trend response.

Use distinct containers and status targets:

- `#quality-trends-status` and `#quality-trends-body` for evidence only.
- `#quality-review-context` for the loaded/dirty scope anchor.
- `#quality-review-form` and `#quality-review-status` for decision entry.
- `#quality-review-comparison` for the latest persisted comparison.
- `#quality-review-history-status` and `#quality-review-history` for history.

No region may reuse a generic `No items found` message or append unrelated
content to the trend metric body.

### Pass 2. Interaction State Coverage: 7/10 -> 10/10

| Region | Loading | Empty | Error | Success | Partial or stale |
| --- | --- | --- | --- | --- | --- |
| Evidence | Keep prior metrics visually present but mark refreshing; disable review save | `No observed records in this window`; `observe` remains available after load | `Observed evidence unavailable`; prior metrics marked stale; review form disabled | Context strip plus bounded metric rows | Withheld rates retain exact source status and explanation |
| Decision | Entire fieldset disabled before evidence load; save says `Saving checkpoint...` in flight | No candidate gaps: explain `Observe` is available and repair/risk needs a different evidence window | Preserve values; focus first invalid field or save status; keep retry key for unchanged request | Clear form after save, issue a new key, open newest history row | Any live filter edit marks context dirty and disables save until refresh |
| Comparison | Empty skeleton is not shown | `No compatible earlier review` | `Latest comparison unavailable`; never remove saved checkpoint confirmation | Show prior and current windows plus neutral differences | Each non-observed metric says why it is not comparable |
| History | Keep old rows and mark refreshing | `No saved reviews for this scope` | Preserve old rows and show `Review history unavailable` | Newest first, max 20, latest disclosure open | `History incomplete: N invalid records excluded`; excluded rows never compare |

History failure alone does not erase a successfully loaded trend projection or
the operator's decision draft. A save response always renders the server's
persisted snapshot, not the earlier browser projection. If `projected_at`
changed, the success status says the checkpoint was recomputed at save time.

### Pass 3. User Journey And Emotional Arc: 7/10 -> 10/10

| Step | Operator does | Desired feeling | Design support |
| --- | --- | --- | --- |
| 1 | Chooses scope and dates | Oriented | One compact filter row with explicit labels. |
| 2 | Loads observed evidence | Confident about what is current | Context strip proves scope and projection time. |
| 3 | Interprets metrics | Cautious, not scored | Honest sample states and no favorable color/direction. |
| 4 | Chooses a conclusion | Responsible | Radio consequences, rationale guidance, and candidate provenance. |
| 5 | Saves | Safe from accidental mutation | Application-immutable and `does not change repairs` copy beside save. |
| 6 | Confirms result | Certain it was recorded once | Focused live success, newest row opened, persisted time visible. |
| 7 | Returns later | Able to reconstruct | Scope history and equivalent earlier-window comparison. |

At five seconds, the operator can identify the loaded window and whether it is
dirty. At five minutes, they can record one decision without typing an ID. At
five years, history still distinguishes evidence, human conclusion, and repair
state without implying formal approval.

After success, focus moves to `#quality-review-status` with
`tabindex="-1"`; the newest history disclosure opens and uses
`scrollIntoView({block: "nearest"})` only when outside the viewport. Validation
failure focuses the first invalid control. Network/storage failure retains the
draft and focuses the save status.

### Pass 4. AI Slop Risk: 8/10 -> 10/10

This is app UI, not a landing page. Use utility copy and one job per region.
Specific labels are required:

- `Observed evidence`
- `Record review decision`
- `Latest compatible earlier review`
- `Review-to-review difference`
- `No compatible earlier review`
- `Current stale count at review time`
- `Saving a checkpoint does not change repair status`
- `Risk accepted is an unattributed local decision, not formal approval`

Metrics remain compact rows. Outcome choices remain native radio labels, not
three promotional cards. The comparison is a structured record panel. History
uses native `details/summary` because each record is a disclosure interaction;
the newest is open by default. No icons, badges with decorative gradients,
progress rings, charts, modal, confetti, entrance animation, or red/green
quality language is allowed.

### Pass 5. Design System Alignment: 8/10 -> 9/10

There is no formal `DESIGN.md`, so 10/10 cannot be claimed. The implementation
must treat `web/app.css` as the existing vocabulary:

- use `--text`, `--muted`, `--accent`, `--line`, `--surface-strong`,
  `--warning`, and existing typefaces;
- use 6-8px radii, hairline borders, and existing 8-12px internal gaps;
- avoid additional shadows inside the already elevated admin surface;
- keep checkpoint controls in one column inside the half-width desktop admin
  section rather than forcing a cramped internal grid;
- add only feature-scoped classes; do not restyle unrelated admin controls.

### Pass 6. Responsive And Accessibility: 6/10 -> 10/10

| Viewport | Required behavior |
| --- | --- |
| Above 960px | Preserve the existing two-column admin shell; the review workspace itself stays one column. Comparison may use two equal columns for prior/current windows. |
| 721-960px | Existing shell becomes one column. Filters wrap, review controls remain one column, and comparison keeps two columns only when labels do not overflow. |
| 720px and below | Filters and buttons become full-width rows; comparison becomes prior-above-current; history summary metadata uses two wrapped lines; no horizontal scroll. |

Accessibility requirements:

- Use `h3` headings and `aria-labelledby` for the four regions.
- Wrap outcomes and candidate gaps in separate `fieldset/legend` groups.
- Give every native checkbox a text label containing gap ID, space, and signal
  source; deduplicate by gap ID and preserve stable visible order.
- Link rationale help, 500-character count, and field error through
  `aria-describedby`; do not announce every character as a live event.
- Use a polite live status for load/save success and a field-linked error
  summary for validation failures.
- Keep save disabled while pending and change its visible text to
  `Saving checkpoint...`.
- Give buttons and complete radio/checkbox label rows a minimum 44px target.
- Add a visible `:focus-visible` outline using the existing accent.
- Let native `details/summary` retain keyboard and screen-reader behavior.
- Convey state and comparison with text, never color alone.

### Pass 7. Resolved Design Decisions: 6/10 -> 10/10

| Decision | Selected rule | Rejected alternative |
| --- | --- | --- |
| Candidate source | Union of currently rendered repeated-recurrence and promoted-eval failure gap IDs, deduplicated, maximum 10 | Repair Inbox state, arbitrary text IDs, or an ineligible-record list |
| No candidate behavior | Keep all outcomes visible; explain that repair/risk cannot save and offer Observe or reload another scope/window | Silently disable radio choices |
| Dirty filter | Compare live filter values with immutable loaded context; show warning, clear comparison, disable save | Disabled button without explanation |
| Save payload | Read dates/space from immutable loaded context, never current controls | Trust mutable filter inputs |
| Idempotency UX | Generate key at first submit; retain for an unchanged failed request; clear when draft changes or save succeeds | New key on every retry |
| Comparison syntax | Prior -> current value, both windows/denominators, signed neutral difference | Delta-only badge or better/worse label |
| Rate syntax | `2/8 (25.0%) -> 1/9 (11.1%); difference -13.9 pp` | Relative percentage change |
| History density | Native details rows, newest open, older collapsed | Twenty fully expanded cards or a modal |
| Post-save movement | Focus status and reveal nearest newest row | Forced page jump or toast-only confirmation |
| Motion | None beyond native disclosure | Decorative transition |

No unresolved design decision remains.

### Design NOT In Scope

- A redesign of the whole admin console or a new formal design system.
- New font, palette, icon library, chart, animation, modal, toast framework, or
  client router.
- Selecting arbitrary, invisible, cross-scope, or ineligible repair records.
- Repair Inbox filtering or row-layout changes.
- Mobile navigation changes outside the checkpoint regions.
- Quality direction, score, favorable color, or formal approval visuals.

### Design Implementation Tasks

- [ ] **D1 (P1, human: about 2h / Codex: about 20min)** - Markup hierarchy -
  Add four semantic regions and independent status targets.
  Surfaced by: Pass 1 and independent Codex hierarchy finding.
  Files: `web/admin.html`, `web/app_static_test.go`.
  Verify: selectors, headings, fieldsets, labels, live regions, and safety copy
  are asserted statically.
- [ ] **D2 (P1, human: about 4h / Codex: about 45min)** - Interaction state -
  Implement immutable loaded context, dirty invalidation, draft fingerprint,
  idempotent retry, candidate derivation, and focus rules.
  Surfaced by: Passes 2, 3, and 7.
  Files: `web/admin.js`, `web/app_static_test.go`.
  Verify: pure helper/static tests plus Stage 5 browser flows for edit, retry,
  no-candidate, success, and failure states.
- [ ] **D3 (P1, human: about 3h / Codex: about 30min)** - Comparison/history -
  Render explicit prior/current values and accessible disclosure history.
  Surfaced by: Passes 1 and 4.
  Files: `web/admin.js`, `web/app_static_test.go`.
  Verify: windows, denominators, percentage points, withheld states, newest-open
  order, excluded-history notice, and `textContent` safety.
- [ ] **D4 (P2, human: about 2h / Codex: about 20min)** - Responsive/a11y CSS -
  Add feature-scoped layout, 44px targets, wrapping, and focus-visible rules.
  Surfaced by: Passes 5 and 6.
  Files: `web/app.css`, `web/app_static_test.go`.
  Verify: 1280px, 800px, and 375px browser QA with keyboard-only navigation.

The design JSONL task artifact was skipped because `jq` is unavailable, as
required by the review skill's non-hand-written fallback.

### Design Completion Summary

| Item | Result |
| --- | --- |
| Initial completeness | 7/10 |
| Final planned completeness | 9/10 overall |
| Information architecture | 10/10 |
| Interaction states | 10/10 |
| User journey | 10/10 |
| AI slop resistance | 10/10 |
| Design alignment | 9/10; no formal DESIGN.md |
| Responsive/accessibility | 10/10 |
| Unresolved decisions | 10/10; none open |
| External voices | Independent Codex: 9 concerns; Claude/subagent unavailable |
| Mockups | Skipped because the design binary is unavailable |

Phase 2 is complete. Independent Codex produced nine actionable concerns, the
fresh subagent produced no result, and Claude was unavailable. Six of seven
litmus directions were confirmed; the card-use disagreement was resolved with
native record disclosures. The reviewed design is ready for Phase 3 engineering
review.

## Engineering Review

### Scope Challenge And Complexity Decision

The implementation crosses the domain, persistence, HTTP composition, and
existing admin UI because a trustworthy checkpoint is only useful when all four
boundaries agree. That naturally touches more than eight files, which triggers
the engineering complexity smell. Reducing the file count would either hide
storage failure semantics inside the service, leave the UI unverified, or skip
composition tests. The scope is therefore retained, but complexity is bounded
by these rules:

- Add exactly one domain service: `QualityReviewCheckpointService`.
- Add no database, repository framework, background worker, event bus, generic
  state-machine library, or comparison service.
- Split the new domain/service and file-store implementation into two focused
  production files rather than forcing typed errors, comparison helpers, raw
  envelope decoding, and file operations into one monolith.
- Reuse the existing Quality Trends projector and Knowledge Gap reader through
  narrow consumer interfaces; do not copy their aggregation or gap lookup.
- Keep comparison derived and read-only. It is never persisted and never
  mutates repair, recurrence, verification, promotion, or eval state.

Expected implementation surface is 14 code/test files plus documentation. The
cross-layer count is justified by separate RED tests at each trust boundary;
the number of new domain services remains one.

### Locked Architecture

```text
Quality Trends controls
        |
        | GET existing projection
        v
QualityTrendService ---- self-describing projection/context
        ^                              |
        |                              | evidence-derived candidate IDs
        |                              v
POST /admin/knowledge/quality-review-checkpoints
        | derives tenant, bounds/decodes request
        v
QualityReviewCheckpointService
        |-- preflight idempotency lookup --------------------+
        |                                                    |
        | new key                                            | replay
        |-- QualityTrendProjector.Project                    |
        |-- KnowledgeGapReader.Get + candidate/scope check   |
        |-- Snapshot V1 mapper                               |
        |-- store.Append with locked key recheck             |
        v                                                    v
QualityReviewCheckpointStore <---- original persisted checkpoint
        | in-memory tests
        | file envelope v1, []json.RawMessage
        v
quality_review_checkpoints.json
        |
        +--> exact-scope bounded history
        +--> one-pass compatible predecessor selection
        +--> neutral latest comparison DTO
```

New production files:

- `internal/admin/knowledge_quality_review_checkpoints.go` owns public types,
  typed errors, consumer interfaces, the service, normalization, Snapshot V1,
  candidate derivation, comparison helpers, and the in-memory store.
- `internal/admin/knowledge_quality_review_checkpoint_file_store.go` owns the
  versioned raw-message envelope, hard capacity checks, file operations, and
  the file store.

The split is a code-quality boundary, not a second service or storage
abstraction layer.

### Self-Describing Projection Contract

`QualityTrendProjection` receives additive review context rather than changing
existing aggregation semantics:

```text
filter.from                 inclusive local YYYY-MM-DD
filter.to                   inclusive local YYYY-MM-DD
filter.timezone             injected process location name
filter.window_days          inclusive local-calendar day count
filter.start_at             existing start instant
filter.end_exclusive_at     existing end instant
projected_at                one service-clock instant for this projection
```

`QualityTrendService.Project` captures `now := s.now()` once and uses that
instant for filter parsing and `projected_at`. The location is resolved once in
the composition root and explicitly injected; no checkpoint request accepts a
timezone. The persisted timezone name and window-day count form part of the
compatibility key. DST tests prove calendar-day semantics rather than elapsed
24-hour arithmetic.

### Persisted Schema V1

The file envelope and every record use schema version `1`. The record is
internal persistence data, not the HTTP response DTO:

```text
QualityReviewCheckpoint
  schema_version
  id, tenant_id, created_at
  filter
    from, to, space_id, all_spaces, timezone, window_days
    start_at, end_exclusive_at
  outcome, rationale, gap_ids
  snapshot
    projected_at
    verification
      attempt_count, passed_gap_count, time_to_verify, current_stale
    recurrence
      suspected_count, confirmed_count, dismissed_count, rate
    promoted_eval
      passed_count, failed_count, unavailable_count, rate
  idempotency_key_hash, request_fingerprint
```

`QualityReviewMetricSnapshotV1` is a distinct allow-listed type. It copies only
`status`, `count`, `denominator`, `value`, `median_ms`, and `excluded_count` as
applicable. It does not embed `QualityTrendProjection` or reuse a future-open
JSON blob. Buckets, repeated failures, question summaries, case rows, trace
IDs, sources, answers, diagnostics, evaluator details, credentials, and admin
tokens are structurally impossible to persist through the mapper.

The safe response DTO omits `tenant_id`, raw or hashed idempotency data, and
request fingerprints. Selected gap IDs remain historical references and are
not revalidated on read.

### Canonical Idempotency Flow

The protocol closes the lost-response and save-time-reprojection race:

1. The handler bounds and strictly decodes one request object.
2. The service derives tenant, trims and validates fields, rejects duplicate
   gap IDs, sorts the accepted IDs, and computes a canonical request
   fingerprint over tenant, normalized filter, outcome, rationale, and IDs.
3. The raw idempotency key is validated as 16-128 safe ASCII ID characters and
   hashed as SHA-256 over `tenant + NUL + key`; the raw key is never persisted,
   logged, labeled, or returned.
4. The store is checked before projection. Same key hash and same fingerprint
   returns the original record with `created=false`; same key hash and a
   different fingerprint returns typed conflict.
5. Only a new key invokes Quality Trends, candidate/gap validation, Snapshot V1
   mapping, ID/time generation, and append.
6. Append repeats the key/fingerprint check while holding the store mutex. Two
   concurrent new-key requests can both project, but only one record is
   written; the loser receives the first persisted record.
7. The HTTP handler returns `201` when `created=true` and `200` for replay.

The fingerprint explicitly excludes checkpoint ID, `created_at`,
`projected_at`, snapshot values, history, and comparison. An unchanged retry
therefore replays the original evidence even if live trend sources changed
after the first successful write.

### Candidate, Tenant, And Scope Boundary

The browser never defines eligibility. For each new create, the service derives
the candidate set from the recomputed projection in this stable order:

1. repeated-recurrence question rows;
2. promoted-eval failure rows;
3. first occurrence of each gap ID wins;
4. stop at ten.

Space aggregate rows, Repair Inbox state, hidden gaps, and arbitrary IDs do not
enter the set. Every selected ID must be in that set and resolve through
`KnowledgeGapReader.Get(derivedTenant, gapID)`. For a specific-space request,
the candidate row and resolved gap must both match that space. For all-spaces,
the resolved gap's space must match the candidate row's space; being any gap in
the tenant is not enough. Missing, foreign, wrong-space, and non-candidate gaps
collapse to one typed non-enumerating conflict.

`observe` accepts zero to ten eligible links. `repair_required` and
`risk_accepted` require one to ten plus a non-empty rationale. No service in
this call graph can update a gap or repair state.

### Store And File Replacement Semantics

The store interface supports idempotency lookup, append, and tenant-scoped
load. The service owns exact-scope filtering, sorting, public limits, and
comparison so in-memory and file implementations cannot drift.

The file format is:

```json
{"schema_version":1,"records":[/* individually decoded records */]}
```

The file store first checks file size, decodes the envelope, verifies version,
then decodes `records` through `[]json.RawMessage`. List can exclude an invalid
individual record and return `excluded_records`; excluded records never enter
history or predecessor selection. Append is stricter: any excluded or
unsupported existing record fails closed before rewrite, preserving the file
byte-for-byte. Corrupt top-level JSON, unsupported envelope version, or a
missing records array always fails closed.

Hard limits are locked at 10,000 records, 64 KiB per encoded record, and 32 MiB
for the existing and post-append file. The public list defaults to and caps at
20. These are pre-replacement checks, not UI-only limits.

Under one store instance, a mutex serializes read/append/replacement. Save uses
same-directory `CreateTemp`, write, file `Sync`, close, and `os.Rename`, with a
deferred best-effort temp cleanup. Create/write/sync/close/rename failures are
injectable in tests and must preserve the prior target before a successful
replace. Go documents that `os.Rename` is not guaranteed atomic on non-Unix
platforms, so the contract is **single-process best-effort whole-file
replacement**, not a universal atomic or tamper-proof ledger. Directory fsync,
cross-process locks, remote durability, and WORM semantics are out of scope and
recorded in `TODOS.md`.

### Comparison And History Projection

GET accepts only `space_id` and `limit`. Omitted or blank `space_id` means exact
all-spaces history; a safe non-empty value means exact specific-space history.
The limit defaults to 20 and must be 1-20. History sorts by `created_at`
descending, then checkpoint ID descending.

The API returns:

```text
create: { checkpoint, created, comparison }
list:   { checkpoints, comparison_for_newest, excluded_records }
```

Create comparison treats the persisted created/replayed record as current.
List comparison treats the newest returned record as current. Predecessor
selection scans all valid matching records, including records beyond the
public 20, in one pass. Compatibility requires equal tenant, equal all-space
flag and normalized space, equal timezone, equal window days, and a strictly
earlier `end_exclusive_at`. Latest earlier end wins, then latest `created_at`,
then checkpoint ID. Same-end and later records remain history but are excluded
as predecessors.

Count differences are signed and limited to named Snapshot V1 counts. Rate
differences require both statuses and values to be observed and are expressed
in percentage points. Duration differences require both observed medians.
Every comparable rate carries prior/current count and denominator. There is no
quality direction, favorable color, score, cause, improvement, or regression
field. Current stale count is labeled `current_at_review`.

### HTTP And Typed Error Contract

The new routes are behind the existing admin authentication boundary:

```text
POST /admin/knowledge/quality-review-checkpoints
GET  /admin/knowledge/quality-review-checkpoints?space_id=<optional>&limit=<1-20>
```

POST uses a 16 KiB `http.MaxBytesReader`, `DisallowUnknownFields`, one decode,
and an EOF check that rejects trailing JSON. Typed errors use `errors.Is`; this
handler adds no message-string classification.

| Status | Stable meaning |
| --- | --- |
| `201` | First checkpoint persisted |
| `200` | Idempotent replay or successful list |
| `400` | Malformed JSON/query or invalid filter/outcome/rationale/key/ID shape; field-safe code/hint |
| `409` | Reused key with changed request or selected gap not eligible; distinct code, non-enumerating message |
| `413` | Request body exceeds 16 KiB |
| `503` | Distinguish checkpoint service, evidence dependency, and storage unavailability without raw cause |

Every error body follows the existing top-level convention and adds actionable
safe text:

```json
{
  "error": "quality_review_gap_not_eligible",
  "message": "One or more selected gaps are no longer eligible for this evidence window.",
  "hint": "Refresh Quality Trends and select gaps from the refreshed candidate list."
}
```

Stable Phase 28 codes include `invalid_quality_review_request`,
`quality_review_idempotency_conflict`,
`quality_review_gap_not_eligible`, `quality_review_request_too_large`,
`quality_review_service_unavailable`,
`quality_review_evidence_unavailable`, and
`quality_review_storage_unavailable`. Validation errors may identify the
submitted field but never foreign resource details. The browser appends the
existing `X-Request-ID` response header to visible failure guidance when
present. Messages explain the problem; hints state whether to correct input,
refresh evidence, reuse or rotate a key, retry, or inspect local service health.

Service-unavailable and corrupt checkpoint storage affect only these routes;
the server and unrelated admin routes continue to start and operate. No error
body includes tenant, rationale, gap existence details, raw record bytes, file
paths, or underlying error strings.

### Browser State Contract

The loaded review context is an immutable copy of the last successful Quality
Trends response. It contains the normalized filter and a token/fingerprint of
only the Quality Trends controls. It never reads `selectedKnowledgeSpaceId` or
Repair Inbox state.

Changing live `from`, `to`, or Quality Trends `space_id` marks the loaded
context dirty, clears latest comparison, disables save, and preserves old
history with a loaded-scope label until the operator refreshes evidence. A
successful refresh replaces context, candidates, comparison, and history for
the new scope. POST always uses immutable loaded context, never mutable inputs.

The browser creates an idempotency key with `crypto.randomUUID()` on first
submit. An unchanged failed request retains it; any normalized outcome,
rationale, selected-ID, or loaded-context change clears it; success clears it
after rendering the persisted response. Double submit is disabled while
pending. Stored strings use `textContent` and native controls only.

For API failures, the browser parses only the stable `error`, safe `message`,
and `hint` fields and reads `X-Request-ID` from the response header. It never
renders raw server causes. Field validation focuses the matching control;
conflict guidance refreshes evidence or rotates the key as appropriate;
unavailable guidance preserves the draft and exact retry key.

### Performance And Resource Review

- A new create performs one existing bounded trend projection, at most ten gap
  reads, one bounded ledger scan, and one whole-file replacement.
- A replay performs the idempotency lookup and comparison only; it does not
  project trends or validate live gaps again.
- List scans at most 10,000 records and computes only one latest comparison,
  avoiding per-row O(N squared) comparison work.
- Public response size is bounded by 20 records; each persisted record is at
  most 64 KiB and never contains trend trace/text collections.
- The 32 MiB file cap bounds read, decode, copy, and encode memory. The feature
  is low-frequency local admin work; no cache, index, database, or background
  compaction is justified in this phase.

Expected complexity is O(S + G + N log N) for first create, where existing
trend source scans `S` are already capped at 10,000 per source, `G <= 10`, and
checkpoint records `N <= 10,000`. Predecessor selection itself is O(N).

### Observability And Debugging

The handler increments
`knowledge_quality_review_checkpoint_requests_total` with only fixed labels:
`operation=create|list` and
`result=created|replayed|success|invalid|conflict|too_large|unavailable`.
Tenant, space, gap, checkpoint, rationale, file path, and idempotency values are
forbidden labels. The existing request ID header remains the correlation aid;
Phase 28 does not introduce a new logger or log operator text.

The list response exposes only the numeric `excluded_records` diagnostic. A
save success always uses the server-persisted checkpoint and `projected_at`, so
operators can distinguish recomputation from the earlier browser view.

### Deployment, Rollback, And Long-Term Boundary

The change is additive and has no data migration or background job. The file
is created lazily as `quality_review_checkpoints.json` in the existing admin
data directory. Rolling back leaves the file unused; a later compatible binary
can reopen schema v1. Unsupported future schema makes only checkpoint routes
unavailable rather than rewriting or downgrading data.

`TODOS.md` now records trustworthy reviewer identity, tamper-evident storage,
scheduled workflow, multi-process durability, and retention/redaction. Each
requires a new spec and is excluded from Phase 28.

### Error And Failure Registry

| Boundary | Failure | Rescue and proof |
| --- | --- | --- |
| Decoder | Too large, unknown field, trailing JSON | `413` or `400`; service call count remains zero |
| Normalizer | Bad filter/outcome/rationale/key/IDs | Deterministic `400`; no projection or append |
| Idempotency preflight | Same request | Return original `200`; no projection |
| Idempotency preflight | Changed request | Generic `409`; no projection or append |
| Projector | Source read or projection failure | `503`; preserve draft; no append |
| Candidate/gap validator | Missing, foreign, wrong-scope, non-candidate | Generic `409`; no existence oracle |
| Append race | Concurrent same key | Locked recheck yields one record/ID |
| Ledger list | One malformed record in valid v1 envelope | Exclude/count; never compare it |
| Ledger append | Existing malformed record | Fail closed; preserve original bytes |
| Ledger load | Corrupt JSON/unknown schema/oversize | `503`; no partial history or rewrite |
| Replacement | Create/write/sync/close/rename failure | `503`; prior file preserved before successful replace |
| Capacity | Count/record/file cap reached | `503`; reject before replacement |
| Comparison | Withheld/missing value | Preserve state; no zero/direction |
| Browser | Dirty filter/network/storage failure | Disable unsafe save; retain loaded history/draft/key as specified |

### Explicit RED-First Test Plan

The executable Stage 3 test artifact is:

`C:\Users\HW\.gstack\projects\digital-twin\nobodycan-codex-phase-28-knowledge-quality-review-checkpoints-test-plan-20260713-224023.md`

It locks eight RED/GREEN batches:

1. self-describing trend projection and DST/calendar semantics;
2. Snapshot V1 allow-list and evidence-derived candidates;
3. predecessor selection and neutral status-aware differences;
4. service validation, no mutation, replay/conflict, and concurrency;
5. in-memory/file ledger reopen, corruption, caps, fault injection, and race;
6. HTTP tenant/auth/body/status/DTO/metric contracts;
7. composition, explicit location, file path, and reopen;
8. static UI contracts followed by Stage 5 browser flows.

The artifact names each first-failing test, focused RED command, expected
failure, full regression sequence, browser matrix, and acceptance traceability.
Stage 3 must execute it in order under RED -> GREEN -> REFACTOR. A race or lint
command unavailable in the local Windows toolchain must be reported honestly
and still run in CI; unavailable never means passed.

### Implementation Sequence And Parallelism

- [ ] **E1 (P0, human: about 2h / Codex: about 20min)** - Trend context RED -
  Add failing projection-context/DST tests, then additive context fields.
  Files: `internal/admin/knowledge_quality_trends.go` and test.
- [ ] **E2 (P0, human: about 6h / Codex: about 1h)** - Domain RED - Add
  Snapshot V1, candidate, comparison, service, idempotency, and in-memory-store
  tests before implementation.
  Files: new domain file and test.
- [ ] **E3 (P0, human: about 6h / Codex: about 1h)** - Ledger RED - Add raw
  envelope, fail-closed append, cap, fault, reopen, and race tests before the
  file store.
  Files: new file-store file and test.
- [ ] **E4 (P1, human: about 4h / Codex: about 45min)** - HTTP/composition RED -
  Add route, strict decode, typed response, auth, metrics, path, and reopen
  tests before wiring.
  Files: `internal/server/server.go`, new Phase 28 server test,
  `cmd/server/main.go`, and test.
- [ ] **E5 (P1, human: about 5h / Codex: about 1h)** - UI RED - Add static
  contracts before markup/state/rendering/CSS; preserve the Design Review
  hierarchy and accessibility contract.
  Files: `web/admin.html`, `web/admin.js`, `web/app.css`, static test.
- [ ] **E6 (P1, human: about 3h / Codex: about 30min)** - Regression/docs - Run
  all verification; update README status/endpoints with copy-paste PowerShell
  and curl create/replay/list examples, expected responses and errors; document
  `data/admin/quality_review_checkpoints.json` or
  `DIGITAL_TWIN_ADMIN_DATA`, schema-v1 upgrade/backup/rollback behavior, and
  Windows/Linux test commands; update Release Notes and check deferred scope.

E1 precedes E2. Once E2's public types/interfaces are green, E3 and the static
markup/CSS portion of E5 have disjoint files and may run in parallel. E4 waits
for E2/E3. JavaScript API integration waits for E4's response DTO. Final
regression and docs wait for all streams. Any parallel worker must own a
disjoint write set and may not bypass the RED evidence requirement.

### Engineering Dual Voices

- Claude was unavailable because the local CLI has no credentials.
- A fresh-context subagent remained running without a result for 120 seconds
  and was closed; no finding is attributed to it.
- Independent Codex completed a read-only repository review after 481 seconds.
  It found three P0 gaps: missing explicit RED matrix, ambiguous replay versus
  recomputation, and tenant-only rather than evidence-candidate gap validation.
  All three are accepted and closed in this review.
- Its P1 findings on raw-message envelopes, Windows rename wording, fixed HTTP
  statuses, explicit timezone source, isolated trend-control dirty state,
  spec/design idempotency drift, and numeric capacity limits are also accepted.
- Its P2 recommendation to split the domain and file store is accepted. The
  suggested new timezone configuration is narrowed to explicit injection of
  the existing process location; a user-selectable timezone would expand the
  Phase 26 contract and is not required for self-describing checkpoints.
- Both primary and independent reviews reject a database, hash chain, formal
  reviewer identity, scheduler, generalized frontend framework, and exposure
  of ineligible gap records in Phase 28.

After the accepted fixes, engineering consensus is `12/12` on the architecture,
trust boundaries, and testability. No engineering decision remains unresolved.

### Engineering Completion Summary

| Item | Result |
| --- | --- |
| Architecture | One new service, narrow readers, split domain/file store |
| Data flow | Replay-before-project; new-key recompute; locked append recheck |
| Snapshot safety | Explicit schema v1 allow-list and forbidden fields |
| Tenant/scope safety | Derived tenant plus projection candidate and gap checks |
| Persistence | Bounded raw envelope; list quarantine; append fail-closed |
| Cross-platform claim | Best-effort single-process replacement, not universal atomicity |
| Error contract | Fixed `201/200/400/409/413/503` mapping |
| Performance | Bounded scans/file/response; one-pass latest comparison |
| Observability | One fixed-label counter; no operator/scope identifiers |
| Test plan | Eight RED-first batches in a separate executable artifact |
| Deferred work | Captured in `TODOS.md` |
| Unresolved decisions | None |

Phase 3 engineering review is complete and the plan is technically executable,
but Stage 3 implementation remains blocked until Phase 3.5 DX review completes
and the user explicitly approves the final Stage 2 plan.

## DX Review

### DX Scope Assessment

Phase 28 is primarily an internal **API/Service** enhancement with a secondary
admin UI and contributor-facing repository workflow. DX POLISH is the selected
mode: the product and local quick start already exist, so this review tightens
the new endpoint, errors, docs, test commands, and upgrade path without adding
an SDK, public platform, or new setup system.

Initial DX completeness was `6.0/10`. A maintainer can start the current server
quickly, but the pre-review plan did not give a complete create/replay/list
example, actionable Phase 28 error bodies, or a findable schema-v1 runbook. The
estimated time from a Go-ready checkout to confidently proving one persisted
checkpoint was about seven minutes of reading across README, spec, and plan.

### Developer Persona Card

| Field | Primary persona |
| --- | --- |
| Who | Backend/full-stack Go contributor maintaining the local-first service and admin UI |
| Context | Extends an existing tenant-scoped admin API, file store, and vanilla JS operations console |
| Tolerance | About 10 minutes before assuming the contract or local setup is incomplete |
| Expects | `go run`, focused tests, typed Go errors, deterministic fixtures, copy-paste HTTP examples, and honest file semantics |
| Avoids | Docker-only setup, hidden global state, magic migrations, raw string error matching, and docs that require reading implementation code |

### Developer Perspective

I open the README and see a Go service with a one-command local start, a long
list of admin endpoints, and a clear `AGENTS.md` workflow. That gives me
confidence that I can make a focused change without assembling infrastructure.
I run `go run ./cmd/server`, open `/admin`, and then look for Phase 28. Before
this DX review, the README still stopped at Phase 27 and the only create example
was a JSON body deep in the spec. I could not tell what response a first write
or replay should return, whether an empty trend window was enough to try the
feature, or how a storage failure differed from an ineligible gap.

I start reading server handlers and tests to reconstruct the contract. I find
`X-Request-ID`, but the current trend UI reduces many failures to a status
number. I also find a useful PowerShell quick start and a Unix-oriented
Makefile, but no Phase 28 curl equivalent or rollback recipe. At that point I
can probably implement the feature, but I am carrying assumptions that should
be executable documentation. The revised plan gives me a three-minute path:
start locally, save a valid `observe` checkpoint over an empty or populated
window, replay the exact request and see the same ID, then list it. When it
fails, the response tells me what happened, the safe cause, what to do next,
and which request ID to report.

### DX What Already Exists

| Existing asset | Evidence | Reuse decision |
| --- | --- | --- |
| One-command local mode | `README.md:122-133` uses `go run ./cmd/server` and loopback URLs | Keep as the zero-credential default path |
| Admin credential discovery | `README.md:153-173` and `GET /admin-access` | Examples explain optional `X-Admin-Key`; do not invent new auth |
| PowerShell operational path | README and `scripts/*.ps1` | Keep Windows first-class; add curl equivalents for this endpoint only |
| Go verification commands | `README.md:301-310`, Makefile, GitHub Actions | Reuse focused and full `go test`, vet, lint, and race commands |
| Design/spec/plan hierarchy | README repo guide and `docs/{specs,design,plans}` | Link Phase 28 directly from README status/highlights |
| Release history | `RELEASE_NOTES.md` has one section per shipped phase | Add schema, endpoint, rollback, and no-migration notes at ship time |
| Request correlation | `internal/server/server.go:208-211` sets `X-Request-ID` | Display it in checkpoint failures; do not add a second correlation ID |
| Actionable auth error | `web/admin.js:30` explains missing/invalid admin key | Match this problem/cause/fix quality for checkpoint errors |
| Static web contracts | `web/app_static_test.go` | Lock labels, route, safe rendering, and failure guidance before JS changes |

There is no OpenAPI document, SDK, public docs site, formal CONTRIBUTING guide,
or JavaScript test harness. None is required to make this internal Phase 28 API
safe and usable.

### Competitive DX Benchmark

Exact public TTHW measurements are not published for these API design
references, so the time estimates below are explicit inferences from their
documented steps after credentials and runtime prerequisites exist.

| Reference | Estimated first proof | Notable DX choice | Applied Phase 28 rule |
| --- | --- | --- | --- |
| Stripe API | Under 2 minutes for one documented request | Same idempotency key replays the original result; changed parameters conflict | Show create and unchanged replay together; document when to rotate the key ([official docs](https://docs.stripe.com/api/idempotent_requests?lang=curl)) |
| Microsoft Azure API guidelines | Design reference, not an install flow | `201` for synchronous POST create, structured code/message, request correlation | Preserve status semantics and add machine code plus safe message/hint ([official guidelines](https://github.com/microsoft/api-guidelines/blob/vNext/azure/Guidelines.md)) |
| GitHub REST API | About 2-5 minutes after token setup | Copy-paste HTTP examples and explicit retry/troubleshooting guidance | Put retry and `X-Request-ID` guidance beside the examples ([official docs](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api)) |
| Phase 28 before DX review | About 7 minutes | Contract distributed across three feature docs | Not competitive |
| Phase 28 target | 3 minutes from Go-ready checkout | One local start plus create/replay/list proof | Competitive tier |

The target is the 2-5 minute competitive tier. A hosted playground would be
disproportionate for a local admin API; a complete terminal example reaches the
same learning goal with no new infrastructure.

### Magical Moment Specification

The magical moment is seeing the **same persisted checkpoint ID** twice: first
as `201 created=true`, then as `200 created=false` after the identical request
is replayed, followed by one history row after list or restart. This proves the
three most important promises at once: server persistence, safe retry, and no
duplicate human decision.

Delivery vehicle is a copy-paste PowerShell block and an equivalent Linux curl
block in README. Both generate dates and an opaque key, create an `observe`
checkpoint with zero linked gaps, replay the exact body, list exact all-space
history, and show the expected status and ID relationship. Local loopback needs
no credential; the example has one clearly marked optional admin-key header for
shared environments.

No seed command is added. `observe` is valid after a successful empty-window
trend projection, so the first proof does not require synthetic recurrence,
verification, or eval data. Repair/risk candidate behavior remains covered by
deterministic Go tests and Stage 5 QA. This is a DX taste decision in favor of a
smaller production surface and a faster complete first run.

### Developer Journey Map

| Stage | Developer does | Pre-review friction | Locked resolution |
| --- | --- | --- | --- |
| 1. Discover | Opens README and current status | README stops at Phase 27 | Update status/highlights and link Phase 28 spec/design/plan |
| 2. Evaluate | Reads endpoint and trust boundary | Success, replay, and failure behavior are distributed | Add one compact contract table and full examples |
| 3. Install | Clones repo with Go 1.26 available | Windows path obvious; Linux path inferred | Keep `go run`; state Go version and equivalent shell commands |
| 4. First value | Creates an `observe` checkpoint | No complete request/response pair | Copy-paste create, replay, and list in <=3 minutes |
| 5. Integrate | Adds browser/API behavior | Defaults and key rotation easy to miss | Document all-space/default 20, status codes, immutable loaded context, retry rules |
| 6. Debug | Handles invalid, conflict, or unavailable | Status-only UI and generic bodies | Stable code, safe message, hint, and visible request ID |
| 7. Test/CI | Runs focused then full suites | Race/lint availability can differ on Windows | Publish exact focused/full commands and report limitations honestly |
| 8. Upgrade/operate | Deploys additive binary and watches local file | File/schema location and health signal are implicit | Document path/env override, lazy create, schema v1, backup, route-only `503` |
| 9. Rollback/migrate | Restores older binary or future backend | Fear of hidden migration/rewrite | State no migration; old binary ignores file; never auto-downgrade unknown schema |

### First-Time Developer Confusion Report

| Time | Pre-review experience | Resolution in this plan |
| --- | --- | --- |
| T+0:00 | README says Phase 27, so Phase 28 is not discoverable | README status and direct artifact links become an E6 ship requirement |
| T+0:45 | `go run ./cmd/server` works and local admin needs no key | Preserve this exact fast path |
| T+1:30 | Endpoint body exists in spec, but no full HTTP request or response | Add PowerShell and curl create/replay/list examples |
| T+3:00 | Unsure whether data must be seeded before saving | State that empty-window `observe` is the first valid proof |
| T+4:00 | Unsure when to retain or rotate idempotency key | Put retry/rotation rules beside the example |
| T+5:00 | A `409` or `503` does not say what to do next | Add stable code, message, hint, and request ID display |
| T+7:00 | Reads code to infer file path and rollback behavior | Add schema-v1 local-data runbook and release note |
| Target T+3:00 | First create, same-ID replay, and one-row list all succeed | Stage 5 measures this path rather than assuming it |

### DX Dual Voices

The fresh-context subagent remained running without a result for 120 seconds
and was closed. Independent Codex completed a read-only DX review after 370
seconds and raised nine actionable concerns: missing end-to-end proof, stale
README status, incomplete examples, weak error guidance, status-only UI errors,
missing rollback runbook, asymmetric shell guidance, hidden defaults, and
global-versus-loaded state risk.

Eight concerns are accepted directly. The ninth recommended a new production
seed command; the primary review accepts the underlying first-run concern but
resolves it with a legal empty-window `observe` flow. Adding synthetic repair
evidence to production is rejected as unnecessary scope. Claude was unavailable
because the local CLI has no credentials, so no item can be labeled cross-model
confirmed.

| Dimension | Claude | Codex | Cross-model status | Autoplan resolution |
| --- | --- | --- | --- | --- |
| Getting started under 5 minutes | N/A | Gap | Not confirmed | Seedless create/replay/list path, target 3 minutes |
| API naming/defaults | N/A | Mostly sound; hidden defaults | Not confirmed | Keep names; document defaults and statuses |
| Actionable errors | N/A | Gap | Not confirmed | Structured code/message/hint plus request ID |
| Findable complete docs | N/A | Gap | Not confirmed | README entry, full examples, direct artifact links |
| Upgrade path | N/A | Gap | Not confirmed | Schema/path/backup/rollback runbook |
| Development environment | N/A | Gap | Not confirmed | PowerShell and Linux commands, no Docker |

DX voice result is `0/6` cross-model confirmed because one voice was unavailable,
with six single-voice findings adjudicated and one taste resolution. There is no
two-model user challenge.

### Pass 1. Getting Started Experience: 5/10 -> 9/10

The current README already passes the hardest installation test for this
persona: `go run ./cmd/server` produces a local service without an account,
credit card, provider key, database, or Docker. The Phase 28 first-value path
was still a five-file scavenger hunt because it lacked one executable HTTP
sequence and did not state that `observe` works over empty evidence.

The revised plan gives an ideal three-step sequence: start the server (about 45
seconds), run create/replay/list examples in a second terminal (about 90
seconds), and optionally open the newest record in `/admin` (about 45 seconds).
Expected statuses and the repeated ID are shown. Final score is 9 rather than 10
because repair/risk examples still require real evidence, deliberately left to
tests and advanced workflow rather than a production seed command.

### Pass 2. API Design: 7/10 -> 9/10

The plural resource route and `from`, `to`, `space_id`, `outcome`, `rationale`,
`gap_ids`, and `idempotency_key` names match existing snake-case admin JSON and
the operator's mental model. Server-derived tenant, metrics, candidates, and
comparison keep the simple request production-safe instead of exposing escape
hatches that would violate trust.

The review makes the implicit defaults explicit: blank/omitted `space_id` is an
exact all-space scope, list `limit` defaults to and caps at 20, dates are
inclusive local calendar dates spanning 1-90 days, first create is `201`, and
unchanged replay is `200`. Required idempotency is intentionally opinionated;
the escape hatch is caller-controlled opaque key generation and deterministic
conflict, not disabling safety. No SDK is needed for one internal endpoint.

### Pass 3. Error Messages And Debugging: 5/10 -> 9/10

The existing admin unlock error is a useful pattern, but current Quality Trends
failures often collapse to `unavailable (status)`. Three Phase 28 paths are now
traced and locked:

| Path | Problem | Safe cause | Fix |
| --- | --- | --- | --- |
| `400 invalid_quality_review_request` | Submitted decision cannot be accepted | Field-safe date/outcome/rationale/key/ID reason | Correct the named field; dates use `YYYY-MM-DD` and 1-90 days |
| `409 quality_review_gap_not_eligible` | One or more links no longer match loaded evidence | Candidate/scope changed; no existence detail | Refresh Quality Trends and select from refreshed candidates |
| `503 quality_review_storage_unavailable` | Checkpoint was not saved/listed | Local checkpoint storage cannot complete safely | Preserve draft/key, retry, and use `X-Request-ID` to inspect service health |

Idempotency conflict has its own `409` code and tells raw API callers to use the
same key only for the identical normalized request, or a new key after a draft
change. Evidence and service unavailability have separate safe codes. The UI
renders only code/message/hint and request ID through text nodes; it never shows
raw Go errors, paths, tenant details, or foreign-resource existence.

### Pass 4. Documentation And Learning: 5/10 -> 9/10

The repository already has strong per-phase spec, design, plan, README, and
release-note conventions, but contributors should not need the implementation
plan to call a shipped endpoint. README becomes the two-minute discovery layer:
current status, endpoint list, browser quick path, full PowerShell/curl examples,
response/error table, defaults, and data-file/rollback notes. It links to the
deeper Phase 28 artifacts for architecture and threat boundaries.

Examples must be executable against local loopback, show optional remote admin
auth separately, and include expected output rather than placeholders alone.
Release Notes records the shipped schema and compatibility story. A generated
API site or OpenAPI document would add maintenance cost without improving this
single internal workflow and is not part of Phase 28.

### Pass 5. Upgrade And Migration: 7/10 -> 9/10

The engineering plan already selects an additive route and lazy-created schema
v1 file, so there is no startup migration, deprecation, or existing-client
break. This review turns that architecture into an operator-facing runbook:
document the default `data/admin/quality_review_checkpoints.json`, the
`DIGITAL_TWIN_ADMIN_DATA` override, backup before binary upgrade, create/replay/
list verification after upgrade, and route-specific behavior for corruption or
unknown schema.

Rollback is intentionally boring: stop the new binary, restore the old binary,
and leave the v1 file untouched because old code does not know the route. A
future schema must fail checkpoint routes closed and never auto-downgrade or
rewrite unknown records. Codemods and migration tooling are not applicable to
this additive first schema.

### Pass 6. Developer Environment And Tooling: 7/10 -> 9/10

Go 1.26, local deterministic mode, in-memory fakes, file-store temp directories,
and focused package tests fit the primary contributor well. CI needs no LLM,
embedding provider, database, or Docker. The separate RED-first artifact gives
exact focused commands and a full verification ladder.

README will show PowerShell and Linux curl/run/test equivalents for Phase 28,
not create a second suite of platform-specific scripts. `go test -race` remains
mandatory where the toolchain supports it; a Windows compiler limitation is
reported as a limitation and covered in supported CI, never described as a
pass. This reaches cross-platform clarity without containerizing local work.

### Pass 7. Community And Ecosystem: 6/10 -> 8/10

The source, issue/PR history, README repo guide, AGENTS workflow, per-phase docs,
and release notes make the implementation process inspectable. A contributor
can find architecture and verification expectations even though there is no
formal CONTRIBUTING file or community channel.

Phase 28 is an internal feature slice, not an ecosystem launch. Adding an SDK,
plugin model, public support forum, pricing/free-tier material, or contribution
program would not improve the checkpoint job and is explicitly excluded. The
score remains 8 because public community onboarding is not established, but
that gap is not Phase 28 debt unless the project chooses an external contributor
strategy.

### Pass 8. DX Measurement And Feedback: 6/10 -> 9/10

The plan already has low-cardinality request outcomes, deterministic server and
web tests, a Stage 5 browser matrix, and an explicit TTHW target. This review
adds measurable first-run acceptance: time a Go-ready checkout through create,
same-key replay, list, and UI history; target <=3 minutes and no source-code
lookup. Failure QA records whether the operator can recover using only the safe
message, hint, and request ID.

No behavioral analytics, NPS, or external telemetry is justified for a local
admin workflow. The boomerang is Stage 5 plus a post-implementation
`devex-review`: compare actual TTHW, error recovery, Windows/Linux commands, and
docs findability against this plan. Request counters measure reliability, not
developer identity or operator content.

### DX NOT In Scope

- OpenAPI generation, SDKs, typed client packages, or a public API portal.
- A production seed command or synthetic repair/eval records; empty-window
  `observe` is the first-run path and test fixtures cover complex states.
- A general CLI wrapper for checkpoint create/list or a new package manager.
- Docker, container-only examples, or a rewrite of existing PowerShell scripts.
- A repository-wide error-envelope migration; Phase 28 adds actionable fields
  compatibly to its new endpoints.
- A JavaScript test framework, frontend state library, or admin console rewrite.
- Public contribution/community/pricing infrastructure without a broader OSS
  product decision.
- User-selectable timezone, arbitrary gap links, or developer escape hatches
  that bypass tenant, candidate, snapshot, or idempotency safety.

### DX Scorecard

| Dimension | Initial | Final | Resolution |
| --- | ---: | ---: | --- |
| Getting Started | 5/10 | 9/10 | Three-minute seedless create/replay/list path |
| API Design | 7/10 | 9/10 | Explicit defaults, statuses, retry semantics |
| Error Messages | 5/10 | 9/10 | Code + safe message + hint + request ID |
| Documentation | 5/10 | 9/10 | README examples, links, outputs, runbook |
| Upgrade Path | 7/10 | 9/10 | Schema-v1 backup, verify, rollback, fail-closed rules |
| Dev Environment | 7/10 | 9/10 | PowerShell/Linux commands, honest race/CI behavior |
| Community | 6/10 | 8/10 | Existing repo guide retained; ecosystem expansion excluded |
| DX Measurement | 6/10 | 9/10 | Timed TTHW and recovery QA plus fixed-label counters |
| **Overall** | **6.0/10** | **8.9/10** | All dimensions at least 8 |

| Scorecard field | Result |
| --- | --- |
| TTHW | About 7 minutes -> target 3 minutes from Go-ready checkout |
| Competitive rank | Competitive (2-5 minutes) |
| Magical moment | Designed: same-ID create/replay via copy-paste terminal example |
| Product type | API/Service plus admin UI |
| Mode | DX POLISH |
| Zero friction | Covered for local loopback |
| Learn by doing | Covered by runnable create/replay/list example |
| Fight uncertainty | Covered by actionable safe errors |
| Opinionated plus escape hatch | Safe defaults; caller key and scope controls retained |
| Code in context | Covered by auth, retry, list, and expected output |
| Measurement | Covered by Stage 5 timing and recovery checks |

### DX Implementation Checklist

- [ ] A Go-ready contributor reaches first create/replay/list in <=3 minutes.
- [ ] README status and endpoint list name Phase 28 and link its artifacts.
- [ ] PowerShell and Linux curl examples run as written in local loopback mode.
- [ ] Examples show optional shared-environment admin authentication separately.
- [ ] First create shows `201/created=true`; exact replay shows the same ID and
  `200/created=false`; list shows one record.
- [ ] Defaults for dates, all-space scope, limit, outcomes, and key rotation are
  next to the examples.
- [ ] Every Phase 28 error supplies stable code, safe message, corrective hint,
  and `X-Request-ID` correlation.
- [ ] Browser failures preserve draft/key correctly and show only safe fields.
- [ ] README documents default/overridden file path, schema v1, backup, upgrade
  verification, rollback, and unknown-schema behavior.
- [ ] Focused tests, full tests, race, vet, lint, and platform limitations are
  documented and verified honestly.
- [ ] Release Notes records additive route/schema and no-migration behavior.
- [ ] Stage 5 times the happy path and verifies three error-recovery paths at
  1280px, 800px, and 375px with keyboard use.

### DX Implementation Tasks

- [ ] **DX1 (P1, human: about 2h / Codex: about 20min)** - Actionable errors -
  Add stable validation/conflict/unavailable codes, safe message/hint, and
  request-ID UI rendering.
  Surfaced by: Pass 3 and independent Codex error/UI findings.
  Files: `internal/server/server.go`, Phase 28 server test, `web/admin.js`,
  `web/app_static_test.go`.
  Verify: error contract tests and Stage 5 recovery flows.
- [ ] **DX2 (P1, human: about 2h / Codex: about 20min)** - First-value docs -
  Update README status/endpoints and add seedless PowerShell/curl
  create/replay/list examples with expected output and defaults.
  Surfaced by: Passes 1, 2, and 4.
  Files: `README.md`, Phase 28 docs.
  Verify: execute both examples against local mode; target <=3 minutes.
- [ ] **DX3 (P2, human: about 1h / Codex: about 10min)** - Upgrade runbook -
  Document path/env, schema, backup, verification, rollback, and unknown-schema
  behavior; update release notes.
  Surfaced by: Pass 5.
  Files: `README.md`, `RELEASE_NOTES.md`.
  Verify: file-path/reopen tests and a rollback documentation audit.
- [ ] **DX4 (P2, human: about 1h / Codex: about 10min)** - Cross-platform proof -
  Publish equivalent Windows/Linux feature commands and record race/lint
  limitations without adding Docker or duplicate script suites.
  Surfaced by: Pass 6.
  Files: `README.md`, test-plan artifact.
  Verify: PowerShell locally, Linux commands in CI, `git diff --check`.

DX1 merges into E4/E5; DX2-DX4 merge into E6. The DX JSONL task artifact is
skipped because `jq` is unavailable; per skill rules it is not hand-written.

### DX Completion Summary

| Item | Result |
| --- | --- |
| Persona | Go backend/full-stack contributor for local API/admin UI |
| Mode | DX POLISH |
| Initial -> final | 6.0/10 -> 8.9/10 |
| TTHW | About 7 minutes -> target 3 minutes |
| Lowest score | Community 8/10; not Phase 28 product scope |
| Magical moment | Same persisted ID on create and replay |
| Outside voice | Independent Codex: 9 concerns; subagent/Claude unavailable |
| Taste decision | Seedless `observe` quick start instead of production seed command |
| Unresolved decisions | None |

Phase 3.5 is complete. The independent voice's first-run, documentation, error,
upgrade, tooling, defaults, and state-isolation concerns are closed in the plan.
The reviewed DX is executable after implementation and ready for the final
Stage 2 approval gate.

## Cross-Phase Themes

### 1. Trust Comes From Bounded, Self-Describing Evidence

CEO, design, engineering, and DX reviews independently converged on the same
rule: a checkpoint must make its tenant/scope/window/projection time and sample
state visible without storing raw evidence or inventing a score. Snapshot V1,
loaded context, explicit windows/denominators, neutral comparison, and README
examples all implement that theme.

### 2. Retry Safety Must Be Understandable, Not Merely Correct

Engineering found the replay-before-projection race; design found the browser
draft/key lifecycle; DX found the missing copy-paste and error guidance. The
combined contract is one stable normalized request per key, original-record
replay, conflict on changed input, disabled double-submit, and documentation
that shows the same ID twice.

### 3. Scope Is A Server Boundary And A Visible UI State

CEO and engineering rejected tenant-only or UI-only gap validation. Design and
DX rejected reliance on mutable global Repair Inbox state. The result is a
server-derived tenant, save-time projection candidate set, resolved gap/space
check, immutable loaded context, dirty-filter invalidation, and non-enumerating
errors.

### 4. Local Durability Requires Honest Language And Rescue Paths

CEO rejected compliance/tamper-proof claims; engineering qualified Windows
rename and corruption behavior; DX required file location and rollback
instructions. The ledger is application-immutable, bounded, single-process,
and fail-closed on unsafe append, with no claim of universal atomicity or WORM.

### 5. Complete Does Not Mean Broad

Every phase rejected scores, automatic repair, identity/approvals, scheduling,
database migration, Docker, and generalized frontend/API infrastructure. The
plan is complete because it covers edge cases, tests, docs, and failure rescue,
not because it expands into adjacent systems.

## Consolidated Build Sequence

The aggregated task JSONL is unavailable because `jq` is not installed. The
reviewed markdown tasks are consolidated manually without inventing new work:

1. **RED 1 - Projection context:** E1 and test-plan Batch 1.
2. **RED 2 - Domain contract:** E2 plus Snapshot/candidate/comparison/service
   Batches 2-4.
3. **RED 3 - Durable store:** E3 and Batch 5.
4. **RED 4 - HTTP/composition/errors:** E4 plus DX1 and Batches 6-7.
5. **RED 5 - Admin interaction:** D1-D4, E5, and Batch 8.
6. **REFACTOR/VERIFY - Full regression:** E6 plus DX2-DX4, README, Release
   Notes, Stage 4 review, Stage 5 browser QA, and Stage 6 security review.

The detailed executable commands and first-failing test names remain in:

`C:\Users\HW\.gstack\projects\digital-twin\nobodycan-codex-phase-28-knowledge-quality-review-checkpoints-test-plan-20260713-224023.md`

## Stage 2 Gate

All Stage 2 reviews are complete: CEO/product, design, engineering, and DX.
The spec and design are synchronized, the architecture and failure semantics
are locked, `TODOS.md` contains durable deferrals, and the explicit eight-batch
RED-first test plan exists on disk. No production code has been written.

Stage 3 remains blocked until the user explicitly approves this final plan.
Approval authorizes only the consolidated RED -> GREEN -> REFACTOR sequence;
it does not authorize any item in DX NOT In Scope or `TODOS.md`.

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | ---: | --- | --- |
| CEO Review | `/plan-ceo-review` via `/autoplan` | Scope and strategy | 1 | CLEAR | Equivalent earlier-window comparison selected; bounded snapshot and deferrals locked |
| Codex Review | Independent `codex exec` passes | Independent second opinion | 3 | CLEAR | Design, engineering, and DX concerns integrated; one seed-command taste recommendation narrowed |
| Eng Review | `/plan-eng-review` via `/autoplan` | Architecture and RED-first tests | 1 | CLEAR | 3 P0 gaps closed; one service, schema v1, typed errors, bounded fail-closed store |
| Design Review | `/plan-design-review` via `/autoplan` | UI/UX states and accessibility | 1 | CLEAR | Score 7/10 -> 9/10; four-region flow and full state matrix |
| DX Review | `/plan-devex-review` via `/autoplan` | Contributor/API experience | 1 | CLEAR | Score 6.0/10 -> 8.9/10; TTHW about 7 -> target 3 minutes |

**CODEX:** Independent passes surfaced the candidate-boundary, idempotency,
corruption, docs, error, and first-run gaps; all were closed or explicitly
resolved as a taste decision.

**VERDICT:** CEO + DESIGN + ENG + DX CLEARED. The plan is ready for explicit
Stage 2 approval; implementation has not started.

NO UNRESOLVED DECISIONS
