# Phase 17 Knowledge Curation and Source Management Plan

Date: 2026-07-04

Status: Draft; awaiting user approval

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 17 Knowledge Curation and Source Management Spec](../specs/phase-17-knowledge-curation-source-management.md)

Source design: [Phase 17 Knowledge Curation and Source Management Design](../design/phase-17-knowledge-curation-source-management.md)

## Plan Summary

Phase 17 will make the local knowledge base maintainable after Phase 16's gap
workbench. The implementation should add a small curation layer over the
existing local knowledge model: searchable/filterable documents, safe edit and
reindex, readable source metadata, and document-gap relationships.

The plan keeps the scope local-first and deterministic. It deliberately defers
rich ingestion, LLM-authored edits, semantic dedupe, graph storage, and
collaborative workflows.

## Autoplan Review Summary

### CEO Review

Score: 8/10

Verdict: hold the curation scope and do not expand into ingestion yet.

The strongest product move is making existing knowledge trustworthy and
maintainable. Rich ingestion would increase visible surface area, but it would
also create more records before the operator can curate them. The real product
flywheel is:

```text
gap -> note -> edit/correct -> relationship -> diagnostics -> reliable answer
```

CEO challenge resolved: keep the phase focused on curation, but make the source
relationship view explicit enough that future ingestion can plug into the same
surface.

### Design Review

Score: 8/10

Verdict: add curation controls inside the existing `/admin` knowledge section
without a full admin redesign.

The current admin page is already dense, so Phase 17 should avoid adding another
large panel. The right UI move is to make the document table more useful and the
detail area more intentional:

- compact filters above the document list;
- selected document detail with metadata and relationships;
- edit mode in the detail area;
- diagnostics reuse below the existing query/debug area.

Design constraint: avoid card-inside-card layouts and keep repeated items as
rows or compact blocks.

### Engineering Review

Score: 8.5/10

Verdict: reuse the existing service and store contracts, adding projections
rather than a second model.

Observed code facts:

- `KnowledgeDocument` already has `Name`, `ContentHash`, `Chunks`,
  `UpdatedAt`, `Tags`, and `Metadata`.
- `KnowledgeService.Reindex` already updates chunks, hash, `UpdatedAt`, and
  index metadata.
- `KnowledgeService.DocumentDetail` already returns a detail projection and
  quality flags.
- Phase 16 stores workbench provenance in metadata:
  `source_type=workbench_note`, `created_from=knowledge_workbench`, and
  `source_gap_id=<gap-id>`.
- `KnowledgeGap` already stores `ResolvedByDocumentID` and `ResolutionNote`.

Engineering choice: add `KnowledgeService.Update` as a named curation API that
can update document title/content while reusing the reindex logic internally.
Do not mutate the store directly from the server.

### DX Review

Score: 8/10

Verdict: API names should match existing `/admin/knowledge/*` conventions and
errors should include the current problem/cause pattern.

Phase 17 is operator-facing more than developer-facing, so no broad DX track is
needed. The main developer experience concern is keeping route names and tests
predictable:

- use `POST /admin/knowledge/update`;
- keep `GET /admin/knowledge` for list/filter;
- grow `GET /admin/knowledge/{document_id}/detail` for relationships;
- return existing JSON error shape: `{"error": "...", "cause": "..."}`.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | Filtering location | Server-side first | Keeps browser simple and makes query/status/source/gap filters testable in `internal/server`. |
| 2 | Editable scope | Local text/Markdown documents | The current model stores chunks, not raw files; local text/Markdown is the safe first editable class. |
| 3 | Update timestamp | Use existing `UpdatedAt` | The document model already has `UpdatedAt`; no new freshness field is needed. |
| 4 | Source labels | Validated metadata keys | Avoids a schema churn while preventing arbitrary browser metadata writes. |
| 5 | Relationship endpoint | Extend existing detail response | `DocumentDetail` already exists and is the natural selected-document projection. |
| 6 | Rich ingestion | Defer | Curation should exist before more sources are imported. |
| 7 | LLM-authored edits | Defer | Knowledge source of truth stays operator-controlled in this phase. |

## Final Scope

### In Scope

- Add `KnowledgeUpdate` input and `KnowledgeService.Update`.
- Allow title/content edits for local text/Markdown documents.
- Preserve document ID, tenant, space, created timestamp, tags, and existing
  source metadata across updates.
- Rebuild chunks, content hash, `UpdatedAt`, and index metadata on content
  updates.
- Add `KnowledgeDocumentFilter` and server-side list filtering.
- Extend `KnowledgeDocumentDetail` with a relationship projection.
- Show source metadata and relationships in `/admin`.
- Add compact filters to `/admin` knowledge list.
- Add edit/save/cancel controls in selected document detail.
- Let diagnostics use the edited document's title/body text as a convenient
  query source.
- Keep Phase 16 note creation and gap resolution working.

### Out of Scope

- SQLite/Postgres/vector DB migration.
- PDF/DOCX/web/GitHub/cloud-drive import.
- Background ingestion workers.
- LLM-suggested or LLM-authored edits.
- Semantic deduplication.
- Collaborative review, assignment, or approvals.
- Full admin redesign.
- `/app` knowledge editing.

## Architecture

```text
web/admin.html
  filter controls
  document detail edit controls
  relationship rendering anchors

web/admin.js
  loadKnowledge(filters)
  renderKnowledgeDetail(detail)
  edit/save/cancel selected document
  diagnostics from selected document text

internal/server
  GET /admin/knowledge?space_id=&query=&status=&source_type=&gap_linked=
  GET /admin/knowledge/{documentID}/detail
  POST /admin/knowledge/update

internal/admin
  KnowledgeService.Update
  KnowledgeService.ListFiltered
  KnowledgeService.DocumentDetail with relations
  KnowledgeGapService.List for relationship projection

local file store
  existing knowledge.json / knowledge_gaps.json
```

## Data Model Plan

### `KnowledgeUpdate`

Add a service input:

```go
type KnowledgeUpdate struct {
    DocumentID  string
    Name        string
    Content     string
    SourceLabel string
}
```

Rules:

- `DocumentID` is required and must pass existing knowledge ID validation.
- `Name` is trimmed and required when provided through the update route.
- `Content` must produce at least one chunk.
- `SourceLabel` is optional and stored only under an allowlisted metadata key,
  for example `source_label`.
- Existing metadata is cloned before mutation.

### `KnowledgeDocumentFilter`

Add a service input:

```go
type KnowledgeDocumentFilter struct {
    SpaceID       string
    Query         string
    Status        KnowledgeStatus
    SourceType    string
    GapLinkedOnly bool
}
```

Rules:

- `SpaceID` defaults to `default`.
- `Query` matches document `Name`, `ID`, `metadata.source_label`, and
  `metadata.source_gap_id`.
- `Status` supports empty, `ready`, `disabled`, `indexing`, and `failed`.
- `SourceType` supports empty, `workbench_note`, `upload`, and `unknown`.
- `GapLinkedOnly` matches documents with `metadata.source_gap_id`.

### `KnowledgeDocumentRelations`

Extend detail with a read-only projection:

```go
type KnowledgeDocumentRelations struct {
    SourceGap    *KnowledgeGap  `json:"source_gap,omitempty"`
    ResolvedGaps []KnowledgeGap `json:"resolved_gaps,omitempty"`
}
```

This is derived from:

- document metadata `source_gap_id`;
- gaps whose `resolved_by_document_id` equals the document ID;
- same tenant and same knowledge space.

## Endpoint Plan

### `GET /admin/knowledge`

Extend existing list route with optional query params:

- `space_id`
- `query`
- `status`
- `source_type`
- `gap_linked`

Response remains `[]KnowledgeDocument`.

### `GET /admin/knowledge/{documentID}/detail`

Extend existing response:

```json
{
  "document": {},
  "quality_flags": [],
  "relations": {
    "source_gap": {},
    "resolved_gaps": []
  }
}
```

If no `KnowledgeGapService` is configured, relationships should be empty rather
than failing document detail.

### `POST /admin/knowledge/update`

Request:

```json
{
  "document_id": "note-123",
  "name": "Updated note.md",
  "content": "Durable source text.",
  "source_label": "operator correction"
}
```

Response: updated `KnowledgeDocument`.

Errors:

- invalid JSON -> `invalid_json`;
- missing service -> `knowledge_admin_unavailable`;
- validation/update failure -> `knowledge_update_failed` with `cause`.

## UI Plan

### Knowledge List Filters

Add compact controls near the knowledge section:

- search input;
- status select;
- source select;
- gap-linked checkbox;
- apply/reset action.

`loadKnowledge()` should read these filters and keep `selectedKnowledgeSpaceId`
as the primary scope.

### Document Detail

Render the selected document with:

- name, status, space, index state, last error;
- source metadata labels;
- relationship summary;
- chunk preview;
- edit button when editable.

Edit mode:

- name input;
- content textarea seeded from chunks;
- optional source label input;
- save/cancel buttons.

### Diagnostics From Document

After selecting or editing a document, the UI can set the diagnostics query to
the document name or selected chunk text. It should still use the existing
retrieval diagnostics route and renderer.

## TDD Implementation Slices

### Slice 1: Admin Service Update

RED:

- `KnowledgeService.Update` preserves document ID, space, created timestamp, and
  source metadata.
- `KnowledgeService.Update` changes name/content hash/chunks/updated timestamp.
- Empty content fails with `ErrKnowledgeUploadEmpty`.
- Disabled or archived spaces reject updates.

GREEN:

- Implement `KnowledgeUpdate`.
- Reuse `chunkKnowledge`, `hashKnowledgeContent`, `applyIndexMetadata`, and
  `requireWritableSpace`.

REFACTOR:

- Share chunk/reindex helper between `Reindex` and `Update` if duplication grows.

### Slice 2: Filtering

RED:

- filter by selected space and query;
- filter by status;
- filter by workbench note source type;
- filter gap-linked only;
- legacy documents without metadata remain listable.

GREEN:

- Implement `KnowledgeDocumentFilter` and `ListFiltered`.
- Extend server list handler to parse query params.

REFACTOR:

- Keep matching helpers private to `internal/admin`.

### Slice 3: Relationships

RED:

- detail for a document with `source_gap_id` includes `source_gap`;
- detail for a document referenced by `resolved_by_document_id` includes
  `resolved_gaps`;
- detail still works if gap service is nil.

GREEN:

- Add relation projection.
- Let server inject gap service into detail rendering if needed.

REFACTOR:

- Keep relationship computation read-only and deterministic.

### Slice 4: Server Update Route

RED:

- route rejects invalid JSON;
- route rejects empty content/title;
- route updates document and returns JSON;
- route preserves Phase 16 metadata;
- route returns `knowledge_update_failed` for validation errors.

GREEN:

- Add `POST /admin/knowledge/update`.
- Add `knowledgeUpdateRequest`.

REFACTOR:

- Reuse existing document mutation helper only if it keeps validation clear.

### Slice 5: Admin UI Curation

RED:

- static tests assert filter controls and update route constant exist;
- static tests assert edit/save/cancel controls exist;
- static tests assert relationship rendering exists;
- static tests assert Phase 16 note/gap controls still exist.

GREEN:

- Update `web/admin.html`, `web/admin.js`, and `web/app.css`.
- Keep text set via `textContent`.

REFACTOR:

- Split `renderKnowledgeDetail` into small render helpers if it gets too long.

### Slice 6: Regression and Browser QA

RED:

- add or extend integration tests for note create -> edit -> detail relation.

GREEN:

- Run local server and verify `/admin` flow:
  1. create or upload local note;
  2. filter to workbench notes;
  3. inspect detail;
  4. edit note;
  5. run diagnostics;
  6. confirm gap relationship still renders.

REFACTOR:

- Fix any layout growth or wrapping issue found during browser QA.

## Test Matrix

| Area | Test | Expected |
| --- | --- | --- |
| Admin service | update document body | Same ID, new hash, new chunks |
| Admin service | update document name | List/detail show new name |
| Admin service | update preserves metadata | `source_gap_id` remains |
| Admin service | update source label | only allowlisted metadata key changes |
| Admin service | empty content update | `ErrKnowledgeUploadEmpty` |
| Admin service | disabled space update | `ErrKnowledgeSpaceDisabled` |
| Admin service | archived space update | `ErrKnowledgeSpaceArchived` |
| Admin service | filter by query | matches name/id/source metadata |
| Admin service | filter by status | returns only requested status |
| Admin service | filter by source type | workbench notes separated from uploads |
| Admin service | gap-linked only | returns documents with `source_gap_id` |
| Detail | source gap relation | detail includes source gap |
| Detail | resolved gap relation | detail includes resolved gaps |
| Detail | no gap admin | detail still returns document and flags |
| Server | update route invalid JSON | `400 invalid_json` |
| Server | update route validation | `400 knowledge_update_failed` |
| Server | update route success | returns updated document |
| Server | list filters | query params affect document list |
| Web static | filter controls | HTML contains search/status/source/gap filters |
| Web static | edit controls | HTML/JS contain update flow |
| Web static | relationships | JS renders source/resolved gap text |
| Regression | Phase 16 note create | still creates indexed note |
| Regression | Phase 16 gap resolve | still records resolving document |
| Regression | app answer state | `/app` metadata display unchanged |

## Validation Commands

Run after implementation:

```powershell
go test ./internal/admin ./internal/server ./web
go test ./...
go vet ./...
git diff --check
```

Browser QA after build:

```powershell
$env:DIGITAL_TWIN_SERVER_PORT="19082"
go run ./cmd/server
```

Then verify `/admin` on `http://127.0.0.1:19082/admin`.

## Implementation Order

1. Admin service update tests and implementation.
2. Admin service filtering tests and implementation.
3. Detail relationship tests and implementation.
4. Server route and list-filter tests.
5. Static web tests for controls and route constants.
6. UI implementation.
7. Full tests and browser QA.
8. README and release notes update.

## Review Findings Folded Into Plan

| Finding | Action |
| --- | --- |
| Rich ingestion is tempting but early | Deferred to a later phase after curation works |
| LLM edits could corrupt source truth | Deferred until diff/review/audit exists |
| Filtering in browser only would hide server bugs | Server-side filters first |
| New relationship graph would overbuild local mode | Derived detail projection only |
| Existing reindex route is close but not expressive enough | Add named `Update` API that reuses reindex internals |
| Admin UI could get cluttered | Compact filters plus selected-document detail, no full redesign |

## Deferred Work

- Rich source ingestion.
- LLM-suggested note edits.
- Semantic duplicate detection.
- Knowledge timeline or graph view.
- Multi-operator approvals.
- Production RBAC.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Curation before ingestion | Auto-decided | Narrowest compounding loop | Existing notes need maintenance before more source volume | PDF/web/cloud import in Phase 17 |
| 2 | Design | Inline curation in knowledge section | Auto-decided | Preserve operator flow | Existing admin already groups knowledge work in one place | Full admin redesign |
| 3 | Eng | Extend service/detail instead of new model | Auto-decided | Reuse local contracts | Existing document/gap fields already support the projection | Separate note/graph store |
| 4 | Eng | Server-side filtering first | Auto-decided | Testability | Backend behavior can be verified without browser state | Client-only filtering |
| 5 | DX | Keep route naming under `/admin/knowledge` | Auto-decided | Consistency | Current API already uses this namespace and error shape | New curation namespace |

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | --- | --- | --- |
| CEO Review | `$gstack-autoplan` | Scope and strategy | 1 | Clear | Hold curation scope; defer ingestion and autonomous edits |
| Design Review | `$gstack-autoplan` | UI/UX gaps | 1 | Clear | Compact filters and selected-document detail are sufficient |
| Eng Review | `$gstack-autoplan` | Architecture and tests | 1 | Clear | Reuse `KnowledgeService`, `DocumentDetail`, and gap projections |
| DX Review | `$gstack-autoplan` | API consistency | 1 | Clear | Use existing `/admin/knowledge` route style and error shape |

**VERDICT:** CEO + Design + Eng + DX reviewed; ready for Stage 3 after user approves the plan.

NO UNRESOLVED DECISIONS
