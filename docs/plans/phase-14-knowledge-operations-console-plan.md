# Phase 14 Knowledge Operations Console Plan

Date: 2026-07-02

Status: Approved by the user on 2026-07-02

Source spec: [Phase 14 Knowledge Operations Console Spec](../specs/phase-14-knowledge-operations-console.md)

Source design: [Phase 14 Knowledge Operations Console Design](../design/phase-14-knowledge-operations-console.md)

## Goal

Turn the existing `/admin` knowledge area into a local-first Knowledge Operations
Console that helps an operator understand knowledge health, inspect document
quality, debug retrieval decisions, and turn unsupported questions into local
knowledge-gap records.

This phase keeps the product honest: it does not add new ingestion formats or a
database before the operator can see what is broken and what to fix next.

## Scope

### In

- Space-level health summaries derived from local documents and index metadata.
- Document detail and quality flags for the selected knowledge document.
- Retrieval debug workbench built on the existing Phase 11 diagnostics pipeline.
- Local knowledge-gap records for knowledge-scoped no-source or unsupported turns.
- `/admin` UI updates for health, detail, debug, and gap queue.
- Static web tests, service tests, store tests, server handler tests, and focused
  app/admin regression coverage.
- README and release notes updates after implementation.

### Out

- SQLite, Postgres, object storage, managed search, or hosted vector database.
- PDF, DOCX, web crawler, Drive, Notion, GitHub, or browser-capture ingestion.
- LLM-generated knowledge health scores.
- Automatic document rewriting or autonomous curation.
- RBAC, billing, SaaS workspace dashboards, or tenant admin.
- A new frontend framework.

## What Already Exists

- `internal/admin.KnowledgeService` owns document upload, list, get, move,
  enable/disable, delete, reindex, spaces, and default-space behavior.
- `internal/admin.FileKnowledgeStore` persists a local JSON envelope with spaces
  and documents, and still loads legacy flat document arrays.
- `internal/knowledge.Pipeline` already emits ranked explanations, stage
  execution, skipped stages, and no-source reasons.
- `internal/server` already exposes knowledge space, document, citation-test, and
  retrieval-diagnostics endpoints.
- `web/admin.html` and `web/admin.js` already have a knowledge space selector,
  upload action, diagnostics query, document table, and chunk preview area.
- `/app` already sends a selected knowledge space and shows compact grounding
  state through Phase 12/13 UI.

## Premise Challenge

The user's ambition points toward a larger knowledge management product. The
temptation is to add more import types immediately. That would create more content
but not more operational clarity.

Phase 14 should instead answer:

> Which knowledge area is unhealthy, why did retrieval fail, and what should the
> operator fix next?

This is the better foundation for future ingestion, because each new source type
will need the same health, debugging, and gap workflow.

## Dream State Delta

Today:

- operators can upload and inspect documents;
- diagnostics can show ranked chunks;
- no-source behavior appears in chat metadata;
- the admin page is functional but not yet a maintenance cockpit.

After Phase 14:

- each selected space has a health summary;
- each document can be inspected with quality flags and index state;
- diagnostics read like a retrieval debug workbench;
- unsupported questions become a small operator queue;
- the app remains clean while admin owns deeper diagnosis.

## Architecture

```text
/app no-source metadata
  -> server/runtime gap capture adapter
  -> admin.KnowledgeGapService
  -> local knowledge_gaps.json
  -> /admin gap queue

/admin selected space
  -> admin.KnowledgeService.HealthSummary
  -> admin.KnowledgeService.DocumentDetail
  -> knowledge.Pipeline diagnostics
  -> web/admin.js operational panels
```

## Data Flow

### Health Summary

1. `/admin` selects a knowledge space.
2. Browser requests `GET /admin/knowledge/health?space_id=...`.
3. Server asks `KnowledgeService` to compute a deterministic summary from spaces,
   documents, chunk counts, statuses, and index metadata.
4. UI renders status and attention reasons above the document table.

### Document Detail

1. Operator clicks a document row.
2. Browser requests the existing `GET /admin/knowledge/{documentID}` or a new
   detail projection endpoint if Stage 3 proves cleaner.
3. Server returns document metadata, chunks, index state, safe error code, and
   quality flags.
4. UI shows a bounded detail panel with chunks and maintenance hints.

### Retrieval Debug

1. Operator enters a query and selected mode.
2. Browser calls existing `POST /admin/knowledge/retrieval-diagnostics`.
3. UI renders mode, stages, no-source reason, ranked chunks, matched terms, final
   score, and rank reason as structured rows instead of only raw text.

### Gap Capture

1. A knowledge-scoped `/app` turn completes with no-source or unsupported metadata.
2. Server creates a local gap record with tenant, space, question, reason, and
   status `open`.
3. `/admin` lists gaps by selected space.
4. Operator marks gaps `ignored` or `resolved`; status persists after reload.

## Data Model

### `KnowledgeHealthSummary`

```go
type KnowledgeHealthStatus string

const (
    KnowledgeHealthHealthy        KnowledgeHealthStatus = "healthy"
    KnowledgeHealthNeedsAttention KnowledgeHealthStatus = "needs_attention"
    KnowledgeHealthEmpty          KnowledgeHealthStatus = "empty"
    KnowledgeHealthDisabled       KnowledgeHealthStatus = "disabled"
)

type KnowledgeHealthSummary struct {
    SpaceID               string                `json:"space_id"`
    SpaceName             string                `json:"space_name"`
    Status                KnowledgeHealthStatus `json:"status"`
    ActiveDocumentCount   int                   `json:"active_document_count"`
    DisabledDocumentCount int                   `json:"disabled_document_count"`
    FailedDocumentCount   int                   `json:"failed_document_count"`
    StaleDocumentCount    int                   `json:"stale_document_count"`
    ChunkCount            int                   `json:"chunk_count"`
    LastUpdatedAt         time.Time             `json:"last_updated_at"`
    AttentionReasons      []string              `json:"attention_reasons,omitempty"`
}
```

### `KnowledgeDocumentDetail`

```go
type KnowledgeDocumentDetail struct {
    Document     KnowledgeDocument `json:"document"`
    QualityFlags []string          `json:"quality_flags,omitempty"`
}
```

Recommended first flags:

- `empty_document`
- `no_chunks`
- `disabled`
- `index_failed`
- `vector_missing`
- `duplicate_content_hash`
- `last_error_present`

### `KnowledgeGap`

```go
type KnowledgeGapStatus string

const (
    KnowledgeGapOpen     KnowledgeGapStatus = "open"
    KnowledgeGapIgnored  KnowledgeGapStatus = "ignored"
    KnowledgeGapResolved KnowledgeGapStatus = "resolved"
)

type KnowledgeGap struct {
    ID                   string             `json:"id"`
    TenantID             string             `json:"tenant_id"`
    SpaceID              string             `json:"space_id"`
    Question             string             `json:"question"`
    NoSourceReason       string             `json:"no_source_reason"`
    Status               KnowledgeGapStatus `json:"status"`
    CreatedAt            time.Time          `json:"created_at"`
    UpdatedAt            time.Time          `json:"updated_at"`
    ResolvedByDocumentID string             `json:"resolved_by_document_id,omitempty"`
}
```

## API Contract

Prefer adding small admin endpoints over overloading existing document endpoints.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/admin/knowledge/health?space_id=...` | selected-space health summary |
| `GET` | `/admin/knowledge/{documentID}/detail` | document detail plus quality flags |
| `GET` | `/admin/knowledge/gaps?space_id=...` | list gap records for a space |
| `POST` | `/admin/knowledge/gaps/update` | mark gap ignored/resolved/open |

Use existing endpoint:

| Method | Path | Phase 14 use |
| --- | --- | --- |
| `POST` | `/admin/knowledge/retrieval-diagnostics` | retrieval debug workbench data |

Internal gap creation can be a service method first. Expose an explicit create
endpoint only if tests show app/server integration needs it.

## UI Contract

Update the existing `knowledge-admin` section in `web/admin.html`.

Recommended layout:

1. **Space controls**
   - existing space selector/create action.

2. **Health strip**
   - compact status;
   - document/chunk counts;
   - attention reasons.

3. **Document table**
   - name;
   - status;
   - chunks;
   - quality;
   - actions.

4. **Document detail**
   - metadata;
   - index state;
   - safe error code;
   - chunks;
   - quality flags.

5. **Retrieval debug**
   - query/mode controls;
   - structured result list;
   - no-source reason.

6. **Gap queue**
   - open gaps for selected space;
   - mark ignored/resolved.

Design constraints:

- Keep it dense and operational.
- Avoid hero sections, decorative panels, or marketing copy.
- Keep long chunk text bounded so the page does not sprawl.
- Use existing static HTML/CSS/JS stack.

## Review Results

### CEO Review

Score: 8.5/10.

The plan correctly treats Phase 14 as an operator-value phase rather than a polish
phase. The strongest move is converting no-source failures into a queue of things
to fix. Defer more ingestion formats until health and gaps exist.

### Design Review

Score: 8/10.

The UI scope is valid but must stay compact. The knowledge admin page already has
many controls, so Phase 14 should use bounded strips, tables, and inline detail
regions instead of new large cards everywhere.

### Engineering Review

Score: 8.5/10.

The architecture should be low-risk because the required primitives already exist:
spaces, documents, metadata, diagnostics, and local file stores. The highest-risk
engineering choice is gap capture from `/app`; implement service/store first, then
wire capture only through allowlisted metadata.

### DX Review

Score: 8/10.

Developer workflow remains good if the phase is split into small service, server,
web, and docs slices. Keep examples copy-pasteable and preserve `go test ./...`.
No new setup dependency is acceptable for this phase.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Make knowledge operations the Phase 14 spine | Auto-decided | User value density | Operators need to know what to fix, not just upload more files | Visual-only admin polish |
| 2 | CEO | Defer PDF/DOCX/web ingestion | Auto-decided | Sequence matters | More sources create more maintenance burden before health exists | More ingestion first |
| 3 | Design | Keep `/app` compact and move deep diagnostics to `/admin` | Auto-decided | Interface focus | Chat should remain a user workspace, admin owns operations | Put chunk ranking in `/app` |
| 4 | Design | Use bounded inline admin sections | Auto-decided | Professional density | Current static UI favors compact tables and panels | New routed admin app |
| 5 | Engineering | Store gaps in a separate local file | Taste decision | Rollback simplicity | Avoids touching the existing knowledge envelope migration path | Store gaps inside `knowledge.json` |
| 6 | Engineering | Health uses enum plus attention reasons | Auto-decided | Truthful diagnostics | Gives a fast signal without fake semantic confidence | Single opaque score |
| 7 | Engineering | Duplicate detection starts with content hash | Auto-decided | Determinism | Hash-based duplicate flags are cheap and testable | LLM/title similarity |
| 8 | Engineering | Reuse existing retrieval diagnostics endpoint | Auto-decided | Avoid duplicate logic | Pipeline already emits explanations and no-source reasons | Fork a separate debug engine |
| 9 | DX | Keep Phase 14 dependency-free | Auto-decided | TTHW preservation | Contributors can run local tests without providers or databases | Add DB/search dependency |

## Taste Decisions

### Gap storage

Recommendation: store gap records in a separate local file such as
`knowledge_gaps.json`.

Why: this keeps the existing `knowledge.json` migration path calmer and makes
rollback easier if the gap model changes.

Alternative: store gaps inside the existing knowledge envelope. That reduces file
count but couples operational events to the knowledge document migration format.

## Work Items

| ID | Area | Files | Outcome |
| --- | --- | --- | --- |
| P14-01 | Health contracts | `internal/admin` | health summary/status structs and service method |
| P14-02 | Health tests | `internal/admin/*test.go` | empty, disabled, healthy, failed/stale, and count cases |
| P14-03 | Document detail | `internal/admin` | detail projection with quality flags and duplicate hash detection |
| P14-04 | Gap contracts/store | `internal/admin` | local gap model, file/in-memory store, list/create/update |
| P14-05 | Gap service tests | `internal/admin/*test.go` | safe IDs, lifecycle, reload persistence, tenant/space filtering |
| P14-06 | Server endpoints | `internal/server` | health/detail/gap list/update handlers |
| P14-07 | Gap capture | `internal/server` or runtime adapter | no-source metadata can create safe local gap records |
| P14-08 | Admin DOM | `web/admin.html` | health strip, detail panel, debug result list, gap queue nodes |
| P14-09 | Admin JS | `web/admin.js` | load/render health, details, diagnostics, and gaps per selected space |
| P14-10 | Admin CSS | `web/app.css` | dense, bounded, non-overlapping admin operations layout |
| P14-11 | Static tests | `web/app_static_test.go` | selectors and copy for Phase 14 admin UI |
| P14-12 | Regression tests | multiple | Phase 10-13 upload/list/chat/presence behavior stays intact |
| P14-13 | Docs | `README.md`, `RELEASE_NOTES.md` | Phase 14 usage and release notes |

## TDD Execution Plan

### P14-01 Health Summary Service

RED:

- Add tests requiring `KnowledgeService.HealthSummary(tenantID, spaceID)`.
- Cover empty active space, disabled space, ready docs, disabled docs, failed docs,
  vector-missing metadata, and chunk totals.

GREEN:

- Add `KnowledgeHealthSummary`, status constants, attention reason derivation, and
  `HealthSummary`.

REFACTOR:

- Keep health derivation in `internal/admin`; do not move it into retrieval.

Command:

```powershell
go test ./internal/admin -run Knowledge
```

### P14-02 Document Detail And Quality Flags

RED:

- Add tests for `KnowledgeService.DocumentDetail`.
- Require quality flags for no chunks, disabled, vector missing, index failed, last
  error, and duplicate content hash.

GREEN:

- Implement detail projection and quality flag helpers.

REFACTOR:

- Keep flag names stable and documented in tests.

Command:

```powershell
go test ./internal/admin -run "Knowledge.*Detail|Knowledge.*Quality"
```

### P14-03 Gap Store And Service

RED:

- Add in-memory and file-backed tests for create/list/update gap records.
- Require tenant/space filtering, deterministic status changes, reload
  persistence, and unsafe ID rejection.

GREEN:

- Add `KnowledgeGap`, status constants, `KnowledgeGapStore`, in-memory fake, file
  store, and `KnowledgeGapService`.

REFACTOR:

- Keep gap file separate from `knowledge.json` unless implementation proves a
  stronger reason to merge.

Command:

```powershell
go test ./internal/admin -run KnowledgeGap
```

### P14-04 Server Endpoints

RED:

- Add handler tests for:
  - `GET /admin/knowledge/health?space_id=default`;
  - `GET /admin/knowledge/{documentID}/detail`;
  - `GET /admin/knowledge/gaps?space_id=default`;
  - `POST /admin/knowledge/gaps/update`.
- Require safe JSON errors and no secret/raw path leakage.

GREEN:

- Add handler config fields for gap service if needed.
- Register and implement endpoints.

REFACTOR:

- Reuse existing tenant defaults and JSON helpers.

Command:

```powershell
go test ./internal/server -run Knowledge
```

### P14-05 Gap Capture From App Turns

RED:

- Add focused tests proving a knowledge-scoped no-source completion creates one
  local gap record with selected space, question text, reason, and no provider
  payload.
- Add dedupe behavior if the same question/space/reason repeats in one session.

GREEN:

- Wire gap capture through allowlisted metadata in the server/presentation path.

REFACTOR:

- Keep capture optional when gap service is nil so existing tests and local modes
  keep working.

Command:

```powershell
go test ./internal/server ./internal/app ./internal/presentation
```

### P14-06 Admin UI Structure

RED:

- Add static tests requiring:
  - `knowledge-health-summary`;
  - `knowledge-health-status`;
  - `knowledge-attention-reasons`;
  - `knowledge-detail-panel`;
  - `knowledge-debug-results`;
  - `knowledge-gap-queue`.

GREEN:

- Update `web/admin.html` with the locked nodes.

REFACTOR:

- Keep labels short and operational.

Command:

```powershell
go test ./web -run Admin
```

### P14-07 Admin Rendering

RED:

- Add static tests requiring JS refs and functions:
  - `loadKnowledgeHealth`;
  - `renderKnowledgeHealth`;
  - `renderKnowledgeDetail`;
  - `renderKnowledgeDebugResults`;
  - `loadKnowledgeGaps`;
  - `renderKnowledgeGapRow`.

GREEN:

- Update `web/admin.js` to fetch/render health, detail, structured diagnostics,
  and gaps whenever selected space changes.

REFACTOR:

- Keep DOM mutation through `textContent` and created elements, not `innerHTML`.

Command:

```powershell
go test ./web -run Admin
```

### P14-08 Admin Layout

RED:

- Add static style tests for bounded admin detail/debug/gap panels.

GREEN:

- Update `web/app.css` with dense admin sections, stable table/detail sizing, and
  bounded chunk/debug text.

REFACTOR:

- Avoid nested cards and keep one operations console visual language.

Command:

```powershell
go test ./web
```

### P14-09 Docs And Regression

RED:

- Add doc/static checks only after shipped behavior is known.

GREEN:

- Update README and release notes with Phase 14 usage.

REFACTOR:

- Keep docs truthful: local-first, no new ingestion formats, no database.

Commands:

```powershell
go test ./...
go vet ./...
rg -n "Phase 14|Knowledge Operations Console|knowledge gap|health summary" README.md RELEASE_NOTES.md docs web internal
```

## Test Matrix

| ID | Area | Scenario | Expected |
| --- | --- | --- | --- |
| T14-01 | Health | Empty active space | status `empty`, reason includes no active documents |
| T14-02 | Health | Disabled space | status `disabled`, documents are not treated as chat-ready |
| T14-03 | Health | Ready documents | active counts and chunk counts are deterministic |
| T14-04 | Health | Failed or stale documents | status `needs_attention` with attention reasons |
| T14-05 | Detail | Ready document | metadata, chunks, index state, and flags render |
| T14-06 | Detail | Disabled document | `disabled` quality flag appears |
| T14-07 | Detail | Duplicate content hash | duplicate flag appears for matching same-space content hash |
| T14-08 | Diagnostics | Matching query | ranked chunks and score breakdown render as structured rows |
| T14-09 | Diagnostics | Unsupported query | no-source reason appears and no fake citation is shown |
| T14-10 | Gap store | Create/list gap | gap appears only for selected tenant and space |
| T14-11 | Gap store | Update status | ignored/resolved/open persists across reload |
| T14-12 | Gap capture | `/app` no-source turn | safe local gap is created without provider payload |
| T14-13 | Server | Admin endpoints unavailable deps | return service-unavailable JSON instead of panic |
| T14-14 | Security | Malicious question text | rendered through `textContent`, no script execution |
| T14-15 | Regression | Existing upload/list/reindex/delete | Phase 10/12 knowledge lifecycle still passes |
| T14-16 | Regression | `/app` Presence summary | Phase 13 summary behavior remains intact |

## Failure Modes Registry

| Mode | Trigger | User Sees | Mitigation |
| --- | --- | --- | --- |
| F14-01 | Health label overclaims semantic quality | Operator trusts weak sources too much | Use operational statuses and explicit reasons only |
| F14-02 | Gap queue stores sensitive text unexpectedly | Local record contains user question | Store only visible user question and no provider/internal payloads |
| F14-03 | Gap capture creates duplicates | Same no-source query repeated | Dedupe by tenant, space, normalized question, and reason |
| F14-04 | Admin page becomes visually crowded | Operator cannot scan health/debug/gaps | Compact bounded regions and clear selected-space context |
| F14-05 | Diagnostics fork from retrieval behavior | Debug result differs from chat grounding | Reuse existing pipeline and endpoint |
| F14-06 | Existing knowledge migration breaks | Old local knowledge disappears | Keep Phase 12 store tests and add regression coverage |

## Error And Rescue Registry

| Error | Likely Cause | Rescue |
| --- | --- | --- |
| `knowledge_admin_unavailable` | server not configured with knowledge service | Start through existing bootstrap/server path |
| `knowledge_gap_admin_unavailable` | gap service nil in a test or custom handler | Return 503 JSON and keep chat path functional |
| `knowledge_space_not_found` | stale selected space in browser | Reload spaces and fall back to default if available |
| `invalid_gap_status` | unsupported status update | Return 400 with allowed statuses in tests/docs |
| `diagnostics_failed` | retrieval service unavailable | Preserve health/detail UI and show safe error |

## Security Notes

- Do not store provider payloads, API keys, hidden prompts, raw stack traces, or
  local filesystem paths in gap records.
- Sanitize and bound question text before persistence.
- Render all admin-provided text with `textContent` / DOM nodes, not `innerHTML`.
- Keep API-key protection behavior unchanged for protected routes.
- Keep tenant/space filtering on every list endpoint.

## Parallelization

Safe independent tracks after contracts are defined:

- Health/detail service work.
- Gap store/service work.
- Admin static DOM/CSS work.

Do not parallelize endpoint wiring and frontend fetch contracts until the route
shapes are locked.

## Implementation Order

1. Health summary contracts and tests.
2. Document detail quality flags.
3. Gap model, store, service.
4. Server endpoints.
5. Gap capture from no-source turns.
6. Admin DOM and JS rendering.
7. Admin CSS polish and browser QA.
8. Docs and release notes.

## Acceptance Commands

```powershell
go test ./internal/admin
go test ./internal/knowledge
go test ./internal/server
go test ./web
go test ./...
go vet ./...
rg -n "Phase 14|Knowledge Operations Console|KnowledgeGap|KnowledgeHealthSummary|knowledge-health|knowledge-gap" .
```

Manual QA after Stage 3:

1. Start the server on a free local port.
2. Open `/admin`.
3. Create or select a space.
4. Upload two documents, including one duplicate or disabled document.
5. Confirm health counts and attention reasons.
6. Inspect document detail and chunk preview.
7. Run a matching diagnostics query and an unsupported query.
8. Open `/app`, ask an unsupported knowledge-scoped question.
9. Return to `/admin` and confirm a gap appears.
10. Mark the gap ignored/resolved and reload.

## Cross-Phase Themes

1. **Truthful trust signals** appeared in CEO, design, engineering, and DX review.
   Phase 14 must not invent confidence or imply semantic quality beyond what local
   signals prove.
2. **Local-first continuity** remains a hard constraint. The phase should not add
   SQLite, external vector stores, real provider calls in CI, or new setup steps.
3. **Admin owns operations; app owns conversation.** Deep diagnostics belong in
   `/admin`, while `/app` should remain compact after Phase 13.

## Deferred

- PDF/DOCX/web ingestion.
- LLM-assisted document summarization or curation.
- Many-to-many document membership.
- Full analytics dashboard.
- Dedicated frontend build pipeline.
- Database migration.

## Plan Approval Gate

This plan is ready for Stage 3 after the user approves it.

Recommended approval:

> Approve Phase 14 as a local-first Knowledge Operations Console with health
> summary, document detail/quality, retrieval debug, and gap records. Use separate
> local gap persistence, deterministic health rules, and no new infrastructure.
