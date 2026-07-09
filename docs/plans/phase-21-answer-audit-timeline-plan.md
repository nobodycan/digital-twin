# Phase 21 Answer Audit Timeline Plan

Date: 2026-07-09

Status: Draft; awaiting user approval

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 21 Answer Audit Timeline Spec](../specs/phase-21-answer-audit-timeline.md)

Source design: [Phase 21 Answer Audit Timeline Design](../design/phase-21-answer-audit-timeline.md)

## Plan Summary

Phase 21 turns the current `/admin` audit table into a real answer-trust
timeline while keeping the existing audit endpoint stable. The implementation
will project existing `AuditRecord` data plus Phase 20 `knowledge_evidence`
metadata into bounded timeline items, expose a dedicated filtered endpoint,
render a scan-friendly `/admin` timeline, and connect weak answers to document
detail and knowledge-gap context without building a compliance ledger or a new
event store.

## What Already Exists

- `internal/admin/audit.go` defines `AuditRecord`, `AuditService.Record`, and
  `AuditService.Recent`.
- `internal/admin/audit_file_store.go` persists audit records in local
  file-backed `audit.json`.
- `internal/server/server.go` exposes `GET /admin/audit` and records audit
  entries after `/experience/stream` and `/experience/mock-voice/stream`.
- `recordAudit` already copies `knowledge_answer_state`,
  `knowledge_result_count`, and `knowledge_evidence` from agent result metadata.
- `captureKnowledgeGap` creates local gaps for `unsupported`,
  `partially_supported`, and `review_gated` turns.
- `web/admin.html` already has an Audit section.
- `web/admin.js` already renders audit trust and evidence summaries and can open
  document detail from citation document IDs.
- `web/app_static_test.go`, `internal/admin/audit_test.go`, and
  `internal/server/server_test.go` already cover audit/evidence wiring.

## NOT In Scope

- Compliance certification, SOC 2 evidence packs, legal audit exports, or
  immutable chain-of-custody.
- New event-sourcing store, SQLite, queue, or background worker.
- Provider prompt replay, raw token capture, hidden reasoning, or
  chain-of-thought storage.
- LLM-as-judge grading, confidence percentages, or semantic correctness scores.
- Cross-tenant analytics, RBAC, reviewer assignment, or multi-user audit policy.
- Bulk knowledge repair inbox; that remains a likely Phase 22.
- Chart-heavy evidence analytics; aggregate metrics should wait until the
  timeline projection is reliable.

## Autoplan Review Summary

### CEO Review

Score: 8.5/10

Verdict: build the timeline, not the dashboard.

Phase 20 made one answer inspectable. The next product risk is that operators
cannot see whether trust is improving or degrading over time. A chronological
timeline is the right foundation because it lets a solo operator trace repeated
weak answers, review-gated sources, and recurring source usage before adding
repair queues or analytics.

CEO challenge resolved: do not overclaim compliance. The feature should be
messaged as an operations timeline for local trust work, not a legal audit
archive.

### Design Review

Score: 8/10

Verdict: make the timeline dense, bounded, and action-oriented.

The existing admin console is an operations surface, so the timeline should feel
like a compact work log rather than a marketing dashboard. Items should show
state, time, conversation, source count, top sources, diagnostics, and a next
action. Long snippets and titles must be clamped. The current Audit table can
remain for compatibility while the new timeline proves itself.

Design risk: adding another admin section can make the page feel crowded. The
plan should group timeline filters tightly and keep metadata secondary.

### Engineering Review

Score: 8.5/10

Verdict: add a projection layer over `AuditRecord`; do not rewrite audit
persistence.

The existing audit service is already the correct source of truth for Phase 21.
The plan should add typed timeline projection helpers, filter semantics, and a
dedicated `GET /admin/audit/timeline` endpoint. New audit records should carry
bounded question summary, space ID, and no-source reason so the timeline can
explain weak answers and conservatively match gaps. Older records must degrade
to unknown/empty fields.

Engineering risk: gap matching can become incorrect if it relies on fuzzy text.
The plan limits matching to explicit evidence gap IDs when present, otherwise
tenant + space + bounded question + no-source reason with ambiguity returning no
link.

### DX Review

Score: 8/10

Verdict: keep endpoint and test names boring.

The developer path should be obvious:

- `AnswerAuditTimelineItem`;
- `AnswerAuditTimelineFilter`;
- `AuditService.Timeline`;
- `GET /admin/audit/timeline`;
- deterministic unit tests before server and frontend wiring.

Docs should make it clear that this is an admin operations feature and give one
copy-paste verification flow: create a grounded answer, create a weak answer,
open `/admin`, filter weak-only, and jump to a source or gap.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | Scope | Answer Audit Timeline | The next missing primitive is trust over time, not repair automation. |
| 2 | Source of truth | Reuse `AuditRecord` | Avoids a second event store and keeps local-first persistence simple. |
| 3 | Endpoint | Add `GET /admin/audit/timeline` | Keeps `GET /admin/audit` stable while exposing projection-specific filters. |
| 4 | Existing Audit table | Preserve it in Phase 21 | Reduces regression risk and lets the timeline prove itself beside existing UI. |
| 5 | Question text | Store bounded `question_summary` on new audit records | Makes the timeline useful while limiting privacy and layout risk. |
| 6 | Gap matching | Explicit gap IDs first; conservative matching second | Prevents wrong links from sending operators to unrelated gaps. |
| 7 | Timeline limits | Clamp server-side limit with deterministic default | Protects file-backed local storage and admin responsiveness. |
| 8 | Review-gated handling | Reason/count only, no gated text | Preserves the Phase 19/20 review boundary. |
| 9 | UI style | Dense operations log, not chart dashboard | Matches `/admin` and avoids premature analytics. |
| 10 | Compatibility | Additive JSON fields only | Older audit records and existing clients keep working. |

## Final Scope

### In Scope

- Add `AnswerAuditTimelineItem`, `AnswerAuditTimelineSource`,
  `AnswerAuditTimelineDiagnostics`, `AnswerAuditTimelineGap`, and
  `AnswerAuditTimelineFilter` types in `internal/admin`.
- Add `AuditService.Timeline(tenantID, filter)` that:
  - orders newest-first;
  - uses stable ID tie-breaks;
  - applies answer-state, weak-only, document ID, conversation ID, and limit
    filters;
  - projects old records safely when evidence is missing.
- Extend new audit records with additive fields:
  - `question_summary`;
  - `knowledge_space_id`;
  - `knowledge_no_source_reason`.
- Add bounded question-summary extraction in `recordAudit`.
- Add conservative gap relationship projection:
  - use explicit `knowledge_evidence.gaps` when available;
  - otherwise match gaps by tenant, space, question summary, and no-source
    reason only when there is a single unambiguous match.
- Add `GET /admin/audit/timeline` with query parameters:
  - `state`;
  - `weak_only`;
  - `document_id`;
  - `conversation_id`;
  - `limit`.
- Render an `/admin` Answer Timeline section with compact filters, timeline
  items, source buttons, and gap status/next-action copy.
- Update docs and release notes after implementation with true shipped
  behavior.

### Out of Scope

- Replacing or deleting `GET /admin/audit`.
- Capturing full user prompts in audit records.
- Fuzzy semantic gap matching.
- Aggregated charts, scorecards, and document quality analytics.
- Creating gaps from timeline reads.
- Provider-backed tests or browser automation in CI.

## Architecture

```text
internal/server
  /experience/stream
    -> recordAudit(additive question/space/reason metadata)
    -> captureKnowledgeGap(existing behavior)

internal/admin
  AuditRecord
    -> AuditService.Recent(existing)
    -> AuditService.Timeline(new projection)
         -> filters
         -> top source summaries
         -> diagnostics
         -> conservative gap relationship

internal/server
  GET /admin/audit/timeline
    -> parse bounded filters
    -> auditAdmin.Timeline(...)
    -> JSON timeline items

web/admin
  Answer Timeline filters
  timeline item renderer
  existing inspectKnowledgeDocument button reuse
  existing gap queue remains source of truth
```

## Data Contract

Recommended additive audit record fields:

```json
{
  "question_summary": "How should I verify DeepSeek startup?",
  "knowledge_space_id": "default",
  "knowledge_no_source_reason": "no_matching_chunks"
}
```

Recommended timeline response item:

```json
{
  "audit_id": "audit-conv-1",
  "conversation_id": "conv-1",
  "user_id": "user-1",
  "created_at": "2026-07-09T10:00:00Z",
  "status": "completed",
  "agent_name": "persona-agent",
  "latency_ms": 842,
  "answer_state": "grounded",
  "source_count": 2,
  "question_summary": "How should I verify DeepSeek startup?",
  "summary": "Grounded by 2 reviewed sources.",
  "top_sources": [
    {
      "document_id": "kb-1",
      "title": "Support Playbook",
      "review_status": "active",
      "source_type": "local_text_file",
      "snippet": "Run the smoke conversation script..."
    }
  ],
  "diagnostics": {
    "no_source_reason": "",
    "review_gated_count": 0
  },
  "gap": {
    "gap_id": "",
    "status": "",
    "reason": ""
  }
}
```

Compatibility rules:

- old audit records without `knowledge_evidence` return `answer_state:
  "unknown"` and empty `top_sources`;
- `question_summary` is optional and bounded;
- `source_count` prefers existing `knowledge_source_count`, then citation
  count;
- timeline response must not include raw gated source text.

## Endpoint Contract

```text
GET /admin/audit/timeline
GET /admin/audit/timeline?weak_only=true&limit=25
GET /admin/audit/timeline?state=grounded&document_id=kb-1
GET /admin/audit/timeline?conversation_id=conv-1
```

Server behavior:

- default `limit`: 50;
- maximum `limit`: 100;
- invalid `limit`: `400 invalid_limit`;
- unknown `state`: allowed as a literal filter, returning zero rows if no match;
- `weak_only=true` includes `unsupported`, `partially_supported`,
  `review_gated`, `provider_fallback`, and `guard_rejected`;
- `document_id` matches any citation document ID in `knowledge_evidence`;
- all responses are tenant-scoped through the existing admin tenant.

## Error And Rescue Registry

| Error | Cause | User-visible rescue |
| --- | --- | --- |
| `audit_timeline_unavailable` | Audit service not configured | Keep admin page usable and show that audit is disabled |
| `audit_timeline_failed` | Store read or projection failure | Show cause in admin status and keep old Audit table untouched |
| `invalid_limit` | Non-numeric or negative limit | Ask operator to use a limit between 1 and 100 |
| `timeline_evidence_missing` | Older audit record has no evidence metadata | Render unknown state and "No evidence recorded" |
| `timeline_gap_ambiguous` | Conservative matching found multiple gaps | Render "gap unknown" instead of linking the wrong gap |

## Failure Modes Registry

| Failure mode | Severity | Detection | Mitigation |
| --- | --- | --- | --- |
| Wrong gap link sends operator to unrelated issue | High | admin timeline unit tests with ambiguous gaps | Link only explicit or single conservative matches |
| Review-gated source text leaks in timeline | Critical | projection tests with gated diagnostics | Reason/count only, never gated snippets |
| Existing audit endpoint regresses | High | server regression tests for `/admin/audit` | Add new endpoint; keep old handler stable |
| Timeline loads too much local data | Medium | limit parsing tests | Default 50, max 100, server-side clamp |
| Old audit records break UI | Medium | projection and static web tests | Unknown/empty fallbacks |
| Admin page becomes noisy | Medium | static tests and manual QA | Dense filters, bounded item layout, no charts |
| Question summaries expose too much text | Medium | summary helper tests | Bound length and avoid full prompt capture |

## Implementation Tasks

Use Superpowers TDD in Stage 3. Each production slice starts with a failing
test.

| ID | Slice | Files | Tests first |
| --- | --- | --- | --- |
| P21-01 | Timeline projection types and ordering | `internal/admin`, tests | Newest-first ordering and stable same-time ID tie-break |
| P21-02 | Timeline filters | `internal/admin`, tests | State, weak-only, document ID, conversation ID, and limit filters |
| P21-03 | Evidence projection | `internal/admin`, tests | Grounded evidence maps source count, summary, and top sources |
| P21-04 | Safety projection | `internal/admin`, tests | Review-gated diagnostics show reason/count without raw gated text |
| P21-05 | Bounded question summary | `internal/server`, tests | Long user questions become deterministic bounded summaries |
| P21-06 | Audit record additive metadata | `internal/server`, tests | `/experience/stream` audit includes question summary, space, and no-source reason |
| P21-07 | Gap relationship resolver | `internal/admin` or `internal/server`, tests | Explicit gap ID wins; ambiguous conservative matches produce no link |
| P21-08 | Timeline endpoint | `internal/server`, tests | `GET /admin/audit/timeline` returns filtered bounded JSON and invalid-limit errors |
| P21-09 | Admin timeline markup | `web/admin.html`, tests | Static selectors for timeline section, filters, refresh, and body |
| P21-10 | Admin timeline renderer | `web/admin.js`, tests | Fetches endpoint, renders state/source/gap rows, opens document detail |
| P21-11 | Admin timeline styling | `web/app.css` or admin CSS, tests | Bounded snippets and stable timeline item classes |
| P21-12 | Docs sync | `README.md`, `RELEASE_NOTES.md`, docs | `rg` verifies Phase 21 true capability wording |

## Test Plan

Focused tests after each slice:

```powershell
go test ./internal/admin
go test ./internal/server
go test ./web
```

Full verification before Stage 4 review:

```powershell
gofmt -w .\internal\admin .\internal\server
go test ./...
go vet ./...
go test -cover ./...
```

Race/lint where local environment supports them:

```powershell
go test -race ./...
golangci-lint run ./...
```

Documentation verification:

```powershell
rg -n "Phase 21|Answer Audit Timeline|answer_audit|/admin/audit/timeline|question_summary" README.md RELEASE_NOTES.md docs internal web
```

Manual QA after implementation:

```powershell
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
```

Then:

1. open `/admin` and confirm the Answer Timeline section loads;
2. import and approve a small knowledge source;
3. ask a source-backed question in `/app`;
4. ask an unsupported knowledge-scoped question in `/app`;
5. return to `/admin` and refresh the timeline;
6. filter `weak_only=true`;
7. open a cited source from a grounded timeline item;
8. inspect weak item gap status and confirm no wrong gap link appears;
9. confirm old Audit table still renders.

## Review Scores

| Review | Score | Summary |
| --- | --- | --- |
| CEO | 8.5/10 | Correct next foundation; keep it operations-focused, not compliance-heavy. |
| Design | 8/10 | Useful if dense, bounded, filterable, and action-oriented. |
| Engineering | 8.5/10 | Existing audit/evidence paths support an additive projection cleanly. |
| DX | 8/10 | Straightforward endpoint/types/tests; docs need one copy-paste verification flow. |

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Build Answer Audit Timeline | Auto-decided | Trust over time | Operators need chronology before repair automation or analytics. | Repair inbox first; analytics first |
| 2 | CEO | Avoid compliance archive claims | Auto-decided | Do not overclaim | Local audit records are useful operations data, not legal evidence. | SOC/ISO framing |
| 3 | Design | Preserve current Audit table while adding timeline | Auto-decided | Minimize regression | Existing audit table is already tested and useful. | Replace table immediately |
| 4 | Design | Use dense operations log layout | Auto-decided | Fit the admin context | `/admin` is work-focused; charts would be premature. | Dashboard-style analytics |
| 5 | Engineering | Add dedicated `/admin/audit/timeline` endpoint | Auto-decided | API clarity | Timeline filters are projection-specific and should not disturb `/admin/audit`. | Overload existing endpoint |
| 6 | Engineering | Extend audit records additively | Auto-decided | Compatibility | Old records and clients keep working. | New store or breaking schema |
| 7 | Engineering | Store bounded question summary | Taste decision | Traceability vs privacy | The timeline needs question context, but full prompts are unnecessary. | No question text; full prompt capture |
| 8 | Engineering | Conservative gap matching | Auto-decided | Avoid wrong actions | A missing link is safer than an incorrect gap link. | Fuzzy semantic matching |
| 9 | Security | No raw gated text in timeline | Auto-decided | Preserve review gate | Pending/rejected source content must not leak through audit. | Include gated snippets for debugging |
| 10 | DX | Boring type and endpoint names | Auto-decided | Guessability | Future contributors should find the feature quickly. | Clever naming |

## Cross-Phase Themes

**Theme: traceability before automation** - CEO, engineering, and DX all point
to a timeline projection as the foundation for future repair inboxes and
analytics.

**Theme: additive local-first compatibility** - engineering and DX both prefer
new projection helpers and endpoint behavior over replacing the existing audit
store or table.

**Theme: bounded trust language** - CEO, design, and security all reject
compliance/archive framing and require review-gate-safe diagnostics.

## Deferred

- Knowledge Repair Inbox for prioritizing, assigning, retesting, and closing
  weak-answer work.
- Evidence Quality Analytics for aggregate source usage and weak-answer rates.
- Compliance export or immutable audit ledger.
- Full prompt/provider replay.
- Cross-tenant reporting and RBAC.
- Semantic gap matching or LLM-based clustering.

## Final Approval Gate

Approve this plan to enter Stage 3 BUILD with Superpowers TDD.

Recommended approval:

> Approve Phase 21 as planned: add a bounded Answer Audit Timeline projection
> over existing audit records and `knowledge_evidence`, expose
> `GET /admin/audit/timeline`, preserve current audit behavior, add bounded
> question summaries, conservative gap links, admin timeline UI, documentation,
> and local deterministic tests.
