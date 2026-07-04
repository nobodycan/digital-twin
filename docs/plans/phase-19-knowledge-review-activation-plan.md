# Phase 19 Knowledge Review and Activation Plan

Date: 2026-07-04

Status: Draft; awaiting user approval

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 19 Knowledge Review and Activation Spec](../specs/phase-19-knowledge-review-activation.md)

Source design: [Phase 19 Knowledge Review and Activation Design](../design/phase-19-knowledge-review-activation.md)

## Plan Summary

Phase 19 adds a lightweight trust gate between Phase 18 ingestion and normal
retrieval. Imported knowledge should be stored with source provenance, but it
should not shape answers until an operator activates it. Existing knowledge
remains active by default to avoid migration breakage.

The implementation should add an explicit document review state, a compact
admin review queue, review actions, retrieval filtering, and deterministic
diagnostics. It should not add RBAC, reviewer assignment, LLM auto-approval,
external storage, or rich source adapters.

## Autoplan Review Summary

### CEO Review

Score: 8.5/10

Verdict: build the trust gate now, but keep it small.

Phase 18 intentionally made imported documents active immediately so ingestion
could ship as a clean first slice. That was correct then. The next product risk
is different: the digital human can now ingest larger, more realistic sources,
so the operator needs a visible decision point before imported content becomes
answer evidence.

The wedge should be:

```text
import source -> pending review -> operator decision -> active retrieval
```

CEO challenge resolved: do not build an enterprise approval suite. The first
user is still a local operator. Review state, provenance, and retrieval
exclusion are enough to make the professional trust story credible.

### Design Review

Score: 8/10

Verdict: add review as a bounded admin queue inside the existing knowledge
surface.

The UI should not become a new workflow app. The operator already uses `/admin`
for spaces, source filters, document detail, import jobs, diagnostics, gaps,
and curation. Review belongs in that flow:

- compact review queue;
- status badges on document rows/detail;
- warning badge for `source_warning`;
- approve/reject/archive/reactivate actions in detail;
- import result links into pending review.

Design constraint: the queue must be summary-first. It should not render raw
document content in the list, and it must not stretch panels with long source
text. Chunk preview remains bounded in document detail.

### Engineering Review

Score: 8.5/10

Verdict: add first-class review fields to `KnowledgeDocument` and keep
lifecycle status separate.

Observed code facts:

- `internal/admin/knowledge.go` owns `KnowledgeDocument`, `KnowledgeStatus`,
  `KnowledgeDocumentFilter`, upload/update/disable/enable, and in-memory store
  behavior.
- `internal/admin/knowledge_file_store.go` persists documents as JSON and can
  tolerate additive JSON fields.
- `internal/admin/knowledge_import.go` creates imported documents through
  `KnowledgeService.Upload`.
- `internal/knowledge/pipeline.go` filters documents through
  `readyKnowledgeDocuments` before lexical/vector search.
- `internal/server/server.go` already has `/admin/knowledge`, upload, import,
  disable, enable, update, reindex, diagnostics, and import-list routes.
- `web/admin.js` already has knowledge list/detail/import rendering and can add
  queue controls without new framework dependencies.

Engineering choice: add a typed `KnowledgeReviewStatus` field to
`KnowledgeDocument`, plus small review metadata fields. Do not overload
`KnowledgeStatus`, and do not rely only on arbitrary metadata keys for the
retrieval trust boundary.

### DX Review

Score: 8/10

Verdict: keep the API boring and visible.

Developer-facing names should match existing admin route style:

- `review_status` query parameter on `GET /admin/knowledge`;
- `POST /admin/knowledge/review`;
- `KnowledgeReviewStatus`;
- `KnowledgeReviewUpdate`;
- `ReviewStatus`, `ReviewReason`, `ReviewedBy`, `ReviewedAt`,
  `ActivatedAt` fields on document JSON.

Every error should keep the current shape:

```json
{"error":"knowledge_review_failed","cause":"invalid review status"}
```

No developer should need DeepSeek, network, SQLite, or browser automation to
run the Phase 19 test suite.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | Scope | Lightweight Trust Gate | Solves source trust without enterprise workflow bloat. |
| 2 | Storage shape | First-class fields on `KnowledgeDocument` | Retrieval and admin code should not depend on loose metadata for trust state. |
| 3 | Backward compatibility | Missing review status derives `active` | Existing knowledge must remain retrievable after upgrade. |
| 4 | Imported default | Phase 18 import-created documents default to `pending_review` | Imported source should not shape answers before review. |
| 5 | Manual upload default | Existing manual upload path defaults to `active` | Preserves current admin upload behavior and avoids surprising users. |
| 6 | Workbench note default | Workbench-created notes default to `active` | Notes are operator-authored in-app curation, not untrusted imported source. |
| 7 | Review statuses | `pending_review`, `active`, `rejected`, `archived` | Covers first trust loop without assignment or approval chains. |
| 8 | Review endpoint | `POST /admin/knowledge/review` | Matches existing admin route style and keeps review separate from lifecycle enable/disable. |
| 9 | List filter | `GET /admin/knowledge?review_status=...` | Reuses existing list endpoint and avoids a second document list API. |
| 10 | Retrieval filter | lifecycle `ready` and review `active` are both required | Retrieval is the actual trust boundary. |
| 11 | Diagnostics | Reason codes only for review-gated docs | Avoids leaking raw pending/rejected source content. |
| 12 | Audit depth | Store current review metadata, defer event log | Enough provenance for first slice; event stream can come later. |

## Final Scope

### In Scope

- Add `KnowledgeReviewStatus` with:
  - `pending_review`;
  - `active`;
  - `rejected`;
  - `archived`.
- Add review fields to `KnowledgeDocument`:
  - `ReviewStatus`;
  - `ReviewReason`;
  - `ReviewedBy`;
  - `ReviewedAt`;
  - `ActivatedAt`.
- Add helper functions for defaulting and predicates:
  - missing review status derives active;
  - active review state plus ready lifecycle state is retrievable;
  - invalid statuses are rejected on write.
- Add `KnowledgeReviewUpdate` and service method for review changes.
- Add `ReviewStatus` to `KnowledgeDocumentFilter`.
- Update import flow so imported documents default to `pending_review`.
- Keep manual upload and workbench note uploads active by default.
- Add `POST /admin/knowledge/review`.
- Extend `GET /admin/knowledge` with `review_status`.
- Update retrieval pipeline to exclude non-active review states.
- Add a concise diagnostic reason when ready documents exist but all are
  review-gated.
- Update `/admin` UI with a bounded review queue and document-detail actions.
- Update README and release notes after implementation with true shipped
  behavior.

### Out of Scope

- RBAC, reviewer assignment, approval chains, or comments.
- LLM auto-approval, source rewriting, or trust scoring.
- PDF, DOCX, crawler, GitHub, cloud-drive, or scheduled sync adapters.
- External database migrations.
- Event-sourced audit table.
- Background workers.
- Automatic review decisions based on `source_warning`.

## Architecture

```text
internal/admin/knowledge.go
  KnowledgeReviewStatus
  KnowledgeReviewUpdate
  KnowledgeDocument review fields
  review helpers and transition validation
  ListFiltered review_status support
  Review(...) service method

internal/admin/knowledge_import.go
  import metadata stays source/provenance focused
  import-created KnowledgeUpload sets pending_review

internal/admin/knowledge_file_store.go
  JSON persistence accepts additive document fields
  normalize old documents to derived active status on read/save path

internal/knowledge/pipeline.go
  readyKnowledgeDocuments -> retrievableKnowledgeDocuments
  require lifecycle ready and review active
  no_review_active_documents diagnostic reason

internal/server/server.go
  GET /admin/knowledge?review_status=pending_review
  POST /admin/knowledge/review
  existing error JSON shape

web/admin.html
  compact review queue container
  detail review controls

web/admin.js
  load/render review queue
  badges and source warnings
  post review updates and refresh list/detail/import jobs
```

## Data Model

Recommended structs:

```go
type KnowledgeReviewStatus string

const (
    KnowledgeReviewPending KnowledgeReviewStatus = "pending_review"
    KnowledgeReviewActive  KnowledgeReviewStatus = "active"
    KnowledgeReviewRejected KnowledgeReviewStatus = "rejected"
    KnowledgeReviewArchived KnowledgeReviewStatus = "archived"
)

type KnowledgeReviewUpdate struct {
    DocumentID   string
    ReviewStatus KnowledgeReviewStatus
    Reason       string
    ReviewedBy   string
}
```

`KnowledgeDocument` should gain JSON fields:

```text
review_status
review_reason
reviewed_by
reviewed_at
activated_at
```

Defaulting rules:

- missing or empty `review_status` is treated as `active` for old documents;
- `KnowledgeService.Upload` defaults to `active`;
- `KnowledgeImportService.Import` explicitly sets `pending_review`;
- `KnowledgeService.Review` sets `ReviewedAt` on every review update;
- `ActivatedAt` is set when review state becomes `active`;
- review fields do not override disabled, failed, or indexing lifecycle
  status.

## State Transitions

Allowed:

```text
pending_review -> active
pending_review -> rejected
pending_review -> archived

active -> rejected
active -> archived

rejected -> pending_review
rejected -> archived

archived -> pending_review
archived -> active
```

Rejected:

- unknown status;
- missing document ID;
- empty target status;
- review update for missing document;
- review update that tries to mutate lifecycle status.

Reason policy:

- approve/archive/reactivate can omit reason;
- reject should allow an empty reason in the first slice to avoid blocking
  operations, but the UI should encourage a short reason.

## Retrieval Behavior

Normal retrieval includes a document only when:

```text
document.Status == KnowledgeReady
AND EffectiveKnowledgeReviewStatus(document) == KnowledgeReviewActive
```

If lifecycle-ready documents exist but none are review-active, return:

```text
no_source_reason = "no_review_active_documents"
stages_skipped includes "review_gated_documents"
```

The response must not include raw pending/rejected/archived chunk text.

## Admin UX

### Review Queue

Place a compact queue near the existing import controls:

- pending count;
- warning count;
- rows: document name, space, source type, warning, imported time, review
  status;
- row click opens existing document detail;
- no raw chunk text in queue rows.

### Document Detail

Add a review section near source metadata:

- review status badge;
- warning badge if `source_warning` exists;
- review reason;
- reviewed timestamp;
- approve/reject/archive/reactivate controls.

### Interaction Rules

- After import completes, refresh review queue.
- After review action, refresh document list, queue, and detail.
- Keep chunk preview bounded.
- Keep controls disabled if no document is selected.

## Implementation Tasks

Use Superpowers TDD in Stage 3. Each task starts with a failing test.

| ID | Slice | Files | Tests first |
| --- | --- | --- | --- |
| P19-01 | Review status model and defaults | `internal/admin/knowledge.go`, `internal/admin/knowledge_test.go` | Missing review status derives active; invalid status rejected |
| P19-02 | Upload default behavior | `internal/admin/knowledge.go`, tests | Manual upload defaults active |
| P19-03 | Import default behavior | `internal/admin/knowledge_import.go`, tests | Imported docs default pending review and keep source metadata |
| P19-04 | Review service method | `internal/admin/knowledge.go`, tests | Approve/reject/archive/reactivate persist fields and timestamps |
| P19-05 | Filter by review status | `internal/admin/knowledge.go`, tests | `ListFiltered` returns only requested review state |
| P19-06 | File store compatibility | `internal/admin/knowledge_file_store.go`, tests | Old JSON docs load as active; review fields survive reload |
| P19-07 | Retrieval gate | `internal/knowledge/pipeline.go`, tests | Pending/rejected/archived docs excluded; active docs retrieve |
| P19-08 | Retrieval diagnostics | `internal/knowledge/pipeline.go`, tests | `no_review_active_documents` emitted without raw gated text |
| P19-09 | Server review endpoint | `internal/server/server.go`, tests | `POST /admin/knowledge/review` updates status and rejects invalid payloads |
| P19-10 | Server list filter | `internal/server/server.go`, tests | `GET /admin/knowledge?review_status=pending_review` filters |
| P19-11 | Admin static UI | `web/admin.html`, `web/admin.js`, `web/app_static_test.go` | Review queue/actions/status badges are wired |
| P19-12 | Docs sync | `README.md`, `RELEASE_NOTES.md` | `rg` verifies Phase 19 true capability wording |

## Test Plan

Run after each relevant slice:

```powershell
go test ./internal/admin
go test ./internal/knowledge
go test ./internal/server
go test ./web
```

Run before Stage 4 review:

```powershell
gofmt -w .\internal\admin .\internal\knowledge .\internal\server
go test ./...
go test -race ./...
golangci-lint run ./...
```

Documentation verification:

```powershell
rg -n "Phase 19|Knowledge Review|review_status|pending_review|no_review_active_documents" README.md RELEASE_NOTES.md docs internal web
```

Manual smoke after implementation:

```powershell
.\scripts\start-deepseek.ps1
```

Then in `/admin`:

1. import a `.md` source;
2. confirm it appears pending review;
3. ask a related question in `/app` and confirm the pending source is not cited;
4. approve it in `/admin`;
5. ask again and confirm the approved source can ground the answer;
6. reject/archive it and confirm retrieval excludes it again.

## Failure Modes Registry

| Failure mode | Severity | Detection | Mitigation |
| --- | --- | --- | --- |
| Old documents stop retrieving | Critical | admin/knowledge tests and pipeline tests | Missing review status derives active |
| Imported content still retrieves before review | High | import + retrieval tests | Import sets pending and retrieval requires active |
| Lifecycle disabled document retrieves because review is active | High | lifecycle precedence test | Retrieval requires both ready and active |
| Rejected content leaks through diagnostics | High | diagnostics tests | Use reason codes only, no gated chunks |
| UI queue grows into raw content wall | Medium | static tests and Stage 5 QA | Summary rows and bounded detail preview |
| Review state hidden from import results | Medium | server/UI tests | Refresh queue after import and show badges |
| Invalid review state corrupts store | Medium | service/server validation tests | Validate status before save |
| Timestamp tests become flaky | Medium | fake clock in service tests | Use service `now` injection |

## Error and Rescue Registry

| Error | Cause | User-visible rescue |
| --- | --- | --- |
| `knowledge_review_failed` | invalid JSON, invalid status, missing document, or store error | Show problem and cause in admin status line |
| `no_review_active_documents` | ready docs exist but none are review-active | Tell operator to approve a source in review queue |
| `knowledge_list_failed` | invalid filter or store failure | Keep existing list error display |
| `knowledge_import_failed` with pending docs absent | import validation failed before document creation | Keep Phase 18 import error display |

## Review Scores

| Review | Score | Summary |
| --- | --- | --- |
| CEO | 8.5/10 | Correct next trust feature; keep solo-operator scope. |
| Design | 8/10 | Useful UI if compact and bounded inside existing admin flow. |
| Engineering | 8.5/10 | Clear service, storage, and retrieval seams already exist. |
| DX | 8/10 | Predictable route and type names; local tests stay simple. |

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Build Lightweight Trust Gate | Auto-decided | Narrowest useful wedge | Separates imported source trust from retrieval without enterprise workflow. | Do nothing; full workflow suite |
| 2 | Eng | Use first-class document review fields | Auto-decided | Explicit over clever | Retrieval trust state is core behavior, not loose provenance metadata. | Metadata-only review state |
| 3 | Eng | Preserve active default for old docs | Auto-decided | Backward compatibility | Prevents silent retrieval regression after additive JSON fields. | Require migration before retrieval |
| 4 | Product | Imported docs default pending review | Auto-decided | Trust boundary first | Imported source should not ground answers before operator activation. | Keep Phase 18 active-immediately behavior |
| 5 | Product | Manual uploads and workbench notes default active | Auto-decided | Preserve established workflow | These are current operator-authored flows; changing them would surprise users. | Make every document pending |
| 6 | Design | Place queue in existing admin knowledge surface | Auto-decided | Reuse before expansion | Operators already manage sources and diagnostics there. | New review app/tab |
| 7 | Security | No LLM auto-approval | Auto-decided | Human trust decision | Source warnings are signals, not verdicts. | LLM trust classifier |
| 8 | Engineering | Defer audit event stream | Auto-decided | Smallest complete slice | Current review fields satisfy first provenance needs. | Separate review event store |

## Cross-Phase Themes

**Theme: trust state must be explicit** - CEO, engineering, and security all
flag the same boundary: mechanical readiness is not the same as trusted answer
evidence.

**Theme: keep the UI summary-first** - design and DX both point to bounded,
scannable rows rather than another large content surface.

**Theme: backward compatibility is non-negotiable** - engineering and DX both
require old documents to keep working without migration commands.

## Deferred

- Event-sourced review audit history.
- Reviewer identity integration with real auth.
- Required rejection reason policy.
- Bulk approve/reject.
- LLM-generated source summaries.
- Trust scoring.
- Scheduled imports and external adapters.

## Final Approval Gate

Approve this plan to enter Stage 3 BUILD with Superpowers TDD.

Recommended approval:

> Approve Phase 19 as planned: first-class document review state, imported
> documents pending by default, old/manual/operator-authored documents active by
> default, admin review queue, review endpoint, retrieval gate, diagnostics, and
> docs sync.
