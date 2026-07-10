# Phase 22 Knowledge Repair Inbox Plan

Date: 2026-07-09

Status: Approved; implemented and verified through Stage 4 review

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 22 Knowledge Repair Inbox Spec](../specs/phase-22-knowledge-repair-inbox.md)

Source design: [Phase 22 Knowledge Repair Inbox Design](../design/phase-22-knowledge-repair-inbox.md)

## Plan Summary

Phase 22 turns weak-answer evidence from Phase 21 into a compact local repair
inbox. The implementation will add a deterministic `KnowledgeRepairItem`
projection over knowledge gaps, answer timeline records, and linked documents;
expose it through a new admin endpoint; render it in `/admin`; and add a
provider-free retest action that shows whether local reviewed knowledge now
supports the gap question. It does not create a new queue store, scheduler,
autonomous writing agent, or confidence scoring system.

## What Already Exists

- `internal/admin/knowledge_phase14.go` defines `KnowledgeGap`,
  `KnowledgeGapService`, gap statuses, local stores, and status updates.
- `internal/admin/audit.go` defines `AuditRecord`,
  `AnswerAuditTimelineItem`, `AuditService.Timeline`, weak-answer filtering,
  evidence diagnostics, source summaries, and explicit gap projection.
- `internal/server/server.go` exposes `GET /admin/knowledge/gaps`,
  `POST /admin/knowledge/gaps/update`, `POST /admin/knowledge/notes/create`,
  `POST /admin/knowledge/retrieval-diagnostics`, and
  `GET /admin/audit/timeline`.
- `handleAuditTimeline` already attaches conservative gap matches when timeline
  evidence lacks an explicit gap ID.
- `recordAudit` stores bounded `question_summary`, `knowledge_space_id`,
  answer state, source count, no-source reason, and knowledge evidence.
- `web/admin.html` and `web/admin.js` already render knowledge health,
  retrieval diagnostics, gap queue actions, workbench notes, document details,
  and the answer timeline.
- Existing tests cover audit projection, server routing, gap status updates,
  admin static selectors, and knowledge service behavior.

## NOT In Scope

- A persistent repair queue separate from existing gaps/audit/documents.
- Background workers, schedulers, SQLite, vector DB changes, or external queues.
- Autonomous document generation, web crawling, LLM clustering, or LLM-as-judge.
- Numeric confidence scores, correctness claims, or compliance audit language.
- Assignments, due dates, comments, owners, notifications, or multi-user RBAC.
- Replacing the existing knowledge gap queue or deleting the answer timeline.
- Browser automation or provider-backed tests in CI.

## Autoplan Review Summary

### CEO Review

Score: 8.5/10

Verdict: build an operator repair loop, not a task manager.

Phase 21 made answer trust visible. The next product risk is that a solo
operator can see weak answers but still has to manually connect gaps, evidence,
diagnostics, and notes. A repair inbox is the right next wedge because it
compresses that workflow into one actionable surface while keeping the existing
local-first primitives intact.

CEO challenge resolved: the feature must not claim that the system can repair
knowledge autonomously. The strongest product story is "show me what needs
human repair and whether local sources now support it."

### Design Review

Score: 8/10

Verdict: make the inbox dense, ranked, and explainable.

The admin console already has many panels, so the repair inbox should feel like
a bounded operations queue rather than a new dashboard. Each row needs a clear
priority, status, answer state, reason, recurrence signal, linked evidence,
and one obvious next action. Long questions and snippets must be clamped.

Design risk: a repair row can become a wall of metadata. The plan keeps the
primary line human-readable and moves reasoning into short chips such as
`repeated_weak_answer`, `no_linked_evidence`, and `review_gated`.

### Engineering Review

Score: 8.5/10

Verdict: add a projection service over current stores.

The existing gap store is already the repair-item source of truth, and the
audit timeline is the recurrence/source context. Phase 22 should add a
`KnowledgeRepairService` or equivalent projection helper in `internal/admin`
that composes `KnowledgeGapService`, `AuditService`, and `KnowledgeService`.
It should not persist derived repair rows.

Engineering risk: retest can accidentally depend on provider behavior or fuzzy
matching. The plan limits retest to local retrieval over active reviewed
knowledge, returning a bounded support-state result without mutating gap status.

### DX Review

Score: 8/10

Verdict: keep names and commands obvious.

The developer path should be simple:

- `KnowledgeRepairItem`;
- `KnowledgeRepairFilter`;
- `KnowledgeRepairService.List`;
- `KnowledgeRepairService.Retest`;
- `GET /admin/knowledge/repairs`;
- `POST /admin/knowledge/repairs/retest`;
- deterministic unit tests before server and frontend wiring.

DX risk: repair logic could spread across server handlers and frontend code.
The plan keeps sorting, filtering, recurrence counting, and safety redaction in
Go tests, while the frontend only renders returned fields and reuses existing
gap/document/note/diagnostic actions.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | Endpoint shape | Add `GET /admin/knowledge/repairs` | Keeps the existing gaps endpoint stable while exposing projection-specific filters. |
| 2 | Retest endpoint | Add `POST /admin/knowledge/repairs/retest` | Gives a single provider-free action for support-state checks. |
| 3 | Source of truth | Gaps remain canonical; repairs are derived | Avoids duplicate lifecycle state and queue drift. |
| 4 | Persistence | Do not persist repair rows or retest attempts in Phase 22 | Keeps the feature local, deterministic, and reversible. |
| 5 | Priority | `high`, `medium`, `low` plus ordered `priority_reasons` | Explainable ranking beats opaque scores. |
| 6 | Provider fallback and guard rejected | Timeline context only unless a gap exists | Not every fallback is a knowledge repair item. |
| 7 | Candidate documents | Explicit links plus on-demand diagnostics | Avoids guessing unrelated citations into repair evidence. |
| 8 | Gated content | Reason/count/status only; no raw gated text | Preserves review boundaries from prior phases. |
| 9 | UI placement | Add repair inbox inside the Knowledge section above the gap queue | The operator sees the action surface before raw gap records. |
| 10 | Compatibility | Additive endpoints and selectors only | Existing gap queue, timeline, and audit flows keep working. |

## Final Scope

### In Scope

- Add repair projection types in `internal/admin`:
  - `KnowledgeRepairItem`;
  - `KnowledgeRepairFilter`;
  - `KnowledgeRepairPriority`;
  - `KnowledgeRepairLinkedDocument`;
  - `KnowledgeRepairRetestResult`.
- Add a repair projection service/helper that:
  - lists gaps for a tenant and space;
  - joins matching answer timeline context by explicit gap ID when available;
  - falls back to exact question + no-source reason + space matching;
  - computes occurrence count and last seen timestamp;
  - identifies linked documents from `ResolvedByDocumentID` and source-gap
    metadata;
  - calculates priority and ordered priority reasons;
  - applies status, reason, weak-only, unresolved-only, linked-evidence,
    space, and limit filters;
  - redacts review-gated raw text and returns only safe summaries.
- Add `GET /admin/knowledge/repairs` with bounded query parameters:
  - `space_id`;
  - `status`;
  - `reason`;
  - `weak_only`;
  - `unresolved_only`;
  - `linked_evidence`;
  - `limit`.
- Add `POST /admin/knowledge/repairs/retest` that:
  - accepts `gap_id`;
  - fetches the gap by tenant;
  - runs local retrieval/diagnostics for the gap question and space;
  - returns before/after support state, source count, top active reviewed
    source summaries, and a next-action message;
  - does not call a provider and does not mutate the gap automatically.
- Render a Knowledge Repair Inbox in `/admin`:
  - compact filters;
  - bounded row body;
  - priority and reason chips;
  - occurrence/last-seen context;
  - linked document buttons;
  - actions for investigate, diagnostics, create note, retest, resolve, ignore,
    and reopen by reusing existing flows where possible.
- Add docs/release note updates after implementation with true shipped
  behavior.

### Out of Scope

- Fuzzy semantic clustering across unrelated questions.
- Retest history, audit export, or trend analytics.
- Auto-resolving a gap after retest.
- New document review statuses.
- Persisting candidate document recommendations.
- Changing retrieval scoring or chunking behavior.

## Architecture

```text
internal/admin
  KnowledgeGapService.List/Get/UpdateStatus
  AuditService.Timeline
  KnowledgeService.ListBySpace/Get/DocumentDetail
        |
        v
  KnowledgeRepairService
    -> list repair projection
    -> compute priority/reasons
    -> filter and limit
    -> retest via local retrieval diagnostics

internal/server
  GET  /admin/knowledge/repairs
  POST /admin/knowledge/repairs/retest
    -> admin tenant
    -> parse bounded filters/request
    -> return JSON

web/admin
  Knowledge Repair Inbox
    -> filters
    -> rows
    -> existing gap actions
    -> existing document detail
    -> existing note creation
    -> retest result panel
```

## Data Contract

Recommended repair item response:

```json
{
  "repair_id": "repair-gap-1",
  "gap_id": "gap-1",
  "space_id": "default",
  "space_name": "Default",
  "question_summary": "How should I verify DeepSeek startup?",
  "status": "open",
  "answer_state": "unsupported",
  "reason": "no_matching_chunks",
  "priority": "high",
  "priority_reasons": [
    "unresolved",
    "repeated_weak_answer",
    "no_linked_evidence"
  ],
  "last_seen_at": "2026-07-09T10:00:00Z",
  "occurrence_count": 3,
  "source_count": 0,
  "linked_documents": [],
  "next_action": "Run diagnostics or create a reviewed note."
}
```

Recommended retest request:

```json
{
  "gap_id": "gap-1"
}
```

Recommended retest response:

```json
{
  "gap_id": "gap-1",
  "question_summary": "How should I verify DeepSeek startup?",
  "before_state": "unsupported",
  "after_state": "grounded",
  "source_count": 1,
  "top_sources": [
    {
      "document_id": "kb-1",
      "title": "Startup Playbook",
      "review_status": "active",
      "source_type": "workbench_note",
      "snippet": "Run the smoke conversation script after boot."
    }
  ],
  "next_action": "Active reviewed source found. Resolve the gap if the source is sufficient."
}
```

Compatibility rules:

- Missing timeline context must still produce a repair item from the gap.
- Missing linked evidence must return `linked_documents: []`, not `null`.
- Unknown or older answer state must degrade to `unknown`.
- `limit` must default to a small bounded value and clamp at 100.
- Review-gated results may expose `review_gated` and counts, but not raw gated
  document text.

## Priority Rules

Default sort order:

1. unresolved statuses before `resolved` and `ignored`;
2. `high` before `medium` before `low`;
3. higher `occurrence_count`;
4. newer `last_seen_at`;
5. stable `gap_id` tie-break.

Priority mapping:

| Priority | Conditions |
| --- | --- |
| `high` | unresolved gap with repeated weak answers, no linked evidence, or review-gated reason |
| `medium` | unresolved gap with one weak answer or linked evidence needing confirmation |
| `low` | resolved or ignored gap retained for traceability |

Priority reasons:

- `unresolved`;
- `repeated_weak_answer`;
- `no_linked_evidence`;
- `linked_evidence_present`;
- `review_gated`;
- `partially_supported`;
- `resolved`;
- `ignored`;
- `no_timeline_context`.

## Failure Modes Registry

| Failure Mode | Prevention |
| --- | --- |
| Wrong gap matched to timeline row | Prefer explicit gap ID; otherwise exact question + reason + space only. |
| Repair inbox leaks gated text | Server-side projection returns safe state/reason/count only. |
| Retest relies on live provider | Retest uses local retrieval/diagnostics only. |
| Derived queue drifts from gaps | Repair items are not persisted. |
| Admin page gets too heavy | Clamp limits and render compact rows. |
| Existing gap queue regresses | Additive selectors and static tests preserve existing IDs. |
| Resolved gaps dominate inbox | Default unresolved-first sort and unresolved filter. |
| Operator assumes retest proves truth | Copy says "support improved", never "verified true". |

## Implementation Tasks

1. Add RED unit tests for repair projection in `internal/admin`.
2. Implement `KnowledgeRepairItem`, filters, priority rules, linked-document
   projection, recurrence joins, and deterministic sorting.
3. Add RED server tests for `GET /admin/knowledge/repairs` parsing, tenant
   scoping, filter behavior, and bounded limits.
4. Wire `GET /admin/knowledge/repairs`.
5. Add RED retest tests for local support improvement and unresolved/no-source
   behavior.
6. Implement `POST /admin/knowledge/repairs/retest` using local retrieval or
   diagnostics.
7. Add RED static/frontend tests for new admin selectors and action wiring.
8. Render the repair inbox in `web/admin.html` and `web/admin.js`.
9. Reuse existing actions for investigate, diagnostics, note creation, resolve,
   ignore, reopen, and document detail.
10. Add docs/release notes after tests pass.
11. Run full verification commands.
12. Enter Stage 4 review after implementation.

## Explicit Test Plan

Commands:

```powershell
go test ./internal/admin ./internal/server ./web
go test ./...
```

Manual local smoke after implementation:

```powershell
go run ./cmd/digital-twin
```

Then open `/admin`, create or use a weak-answer gap, inspect the Knowledge
Repair Inbox, run diagnostics, create/link a note, retest, and confirm the
existing gap queue and answer timeline still render.

## Test Matrix

| ID | Scenario | Expected Coverage |
| --- | --- | --- |
| P22-T01 | Open gap with no timeline context | Repair item appears with `unknown`, `no_timeline_context`, and safe next action. |
| P22-T02 | Unsupported gap with repeated timeline rows | `occurrence_count` and `last_seen_at` derive from matching rows. |
| P22-T03 | Open repeated weak gap without evidence | Priority is `high` with `unresolved`, `repeated_weak_answer`, and `no_linked_evidence`. |
| P22-T04 | Resolved gap with linked document | Priority is `low`; linked document summary is present. |
| P22-T05 | `status=investigating` filter | Only investigating repair items are returned. |
| P22-T06 | `unresolved_only=true` filter | Open and investigating items are returned; resolved and ignored are excluded. |
| P22-T07 | `linked_evidence=true` filter | Only items with resolution/source-gap documents are returned. |
| P22-T08 | `reason=no_matching_chunks` filter | Only matching no-source reason items are returned. |
| P22-T09 | `weak_only=true` filter | Items without weak timeline context are excluded unless their gap reason maps to weak unknown-source work. |
| P22-T10 | Review-gated source context | Response contains state/reason/count but no raw gated text. |
| P22-T11 | Retest after active reviewed note | Returns improved support state and top source summary without mutating gap status. |
| P22-T12 | Retest with no active source | Returns unresolved/unsupported result and next action to create or review evidence. |
| P22-T13 | Long question | Server/frontend return bounded summary and UI remains scan-friendly. |
| P22-T14 | Existing `/admin/knowledge/gaps` | Endpoint and frontend gap queue behavior remain unchanged. |
| P22-T15 | Tenant/space scoping | Items from other tenants/spaces are not returned. |
| P22-T16 | Limit parsing | Invalid limit returns 400; large limit clamps to 100. |
| P22-T17 | Stable tie-break | Equal priority/time items sort by stable gap ID. |
| P22-T18 | Provider fallback timeline without gap | Does not create a repair item by itself. |

## Review Scores

| Review | Score | Gate |
| --- | --- | --- |
| CEO | 8.5/10 | Pass |
| Design | 8/10 | Pass |
| Engineering | 8.5/10 | Pass |
| DX | 8/10 | Pass |

## Decision Audit Trail

| Question | Decision | Why |
| --- | --- | --- |
| New endpoint or extend gaps? | New `GET /admin/knowledge/repairs` | Repairs have projection filters and timeline joins that should not overload raw gap list semantics. |
| Persist retest attempts? | No, return ephemeral result in Phase 22 | Keeps this phase deterministic and avoids inventing history before usage proves it. |
| Priority format? | `high`/`medium`/`low` plus reasons | Operators need explainability, not a magic score. |
| Provider fallback/guard rejected items? | Only if an actual gap exists | Some failures are provider/policy issues, not knowledge repair work. |
| Candidate docs source? | Explicit links plus diagnostics on demand | Prevents accidental linking of unrelated citations. |
| Auto-resolve after retest? | No | Human operator owns the repair decision. |

## Approval Gate

Recommended Stage 2 approval phrase:

`批准 Phase 22 plan，进入 Stage 3 TDD。`

After approval, Stage 3 must follow RED -> GREEN -> REFACTOR and implement only
the tests and production changes needed for this plan.
