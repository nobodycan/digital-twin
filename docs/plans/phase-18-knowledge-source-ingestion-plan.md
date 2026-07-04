# Phase 18 Knowledge Source Ingestion Plan

Date: 2026-07-04

Status: Draft; awaiting user approval

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 18 Knowledge Source Ingestion Spec](../specs/phase-18-knowledge-source-ingestion.md)

Source design: [Phase 18 Knowledge Source Ingestion Design](../design/phase-18-knowledge-source-ingestion.md)

## Plan Summary

Phase 18 will add a local-first knowledge source ingestion workflow on top of
the Phase 17 curation surface. The slice should prove the import spine before
adding rich adapters: import jobs, deterministic validation, exact content-hash
dedupe, source metadata, `/admin` import visibility, and reuse of existing
knowledge documents, diagnostics, retrieval, and grounded answers.

The plan deliberately keeps PDF, DOCX, server-side crawling, GitHub sync, cloud
drive sync, background workers, and LLM-authored maintenance out of scope.

## Autoplan Review Summary

### CEO Review

Score: 8.5/10

Verdict: build the ingestion spine now, but keep the first adapter set narrow.

Phase 17 made curation real enough that ingestion is the right next product
move. The important product shift is not "upload more file types"; it is
"operator can feed trusted sources into the digital human and still know what
happened." The first wedge should be:

```text
trusted text/Markdown source -> visible import job -> normal knowledge document
  -> curation/detail -> diagnostics -> grounded answer
```

CEO challenge resolved: hold the source adapters to text/Markdown file import
and manual URL text snapshots. That proves source intake without letting parser
and crawler complexity take over the phase.

### Design Review

Score: 8/10

Verdict: add ingestion as a compact operator panel inside existing `/admin`
knowledge management.

The UI should not become a wizard or a separate app. The operator is already in
a knowledge workspace with selected space, documents, health, diagnostics, and
gaps. Import belongs near those controls:

- source type selector;
- file or URL snapshot fields;
- source label;
- import action;
- recent job list;
- click-through into existing document detail.

Design constraint: keep job results as dense rows, not nested cards. Every job
row needs a clear status, source type, count, and failure/skip reason.

### Engineering Review

Score: 8.5/10

Verdict: add an import service/projection while creating documents through the
existing `KnowledgeService` path.

Observed code facts:

- `internal/admin/knowledge.go` already owns `KnowledgeUpload`, `KnowledgeDocument`,
  `KnowledgeService.Upload`, `Reindex`, `Update`, and `ListFiltered`.
- Phase 17 already validates safe metadata mutation through service-level
  allowlists rather than browser-controlled arbitrary metadata.
- `KnowledgeDocument` already has `ContentHash`, `ChunkCount`, `Chunks`,
  `Metadata`, `CreatedAt`, and `UpdatedAt`.
- Existing `/admin/knowledge/upload`, `/admin/knowledge/update`, detail,
  diagnostics, and space routes establish naming conventions.
- The local-first persistence approach avoids SQLite and external stores.

Engineering choice: implement a `KnowledgeImportService` or clearly separated
`KnowledgeService.Import` path that wraps existing upload/reindex behavior. Do
not duplicate chunking, hashing, or document persistence outside the knowledge
service boundary.

### DX Review

Score: 8/10

Verdict: make import APIs boring and inspectable.

The primary developer-facing surface is internal service and route shape, not an
SDK. Keep names predictable and copyable:

- `POST /admin/knowledge/import`;
- `GET /admin/knowledge/imports?space_id=...`;
- `KnowledgeImportRequest`;
- `KnowledgeImportSource`;
- `KnowledgeImportJob`.

Every validation error should expose problem and cause in the existing JSON
error shape. README should get one local example after implementation, but no
credentials or external network should be required.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | First adapters | `.txt`/`.md` local text file and manual URL text snapshot | Proves ingestion with deterministic tests and no parser/crawler dependency. |
| 2 | Server-side URL fetch | Defer | Keeps CI deterministic and avoids crawler/security scope. |
| 3 | Import execution | Synchronous first, job-modeled | Simpler implementation while preserving a future worker-compatible state model. |
| 4 | Import service boundary | Prefer a small import service wrapping `KnowledgeService` | Keeps import job concerns separate without duplicating document indexing logic. |
| 5 | Job persistence | Local file-backed persistence behind existing store pattern | Matches local-first architecture and avoids SQLite. |
| 6 | Duplicate behavior | Skip exact duplicate content in the same space and record the existing document ID when available | Avoids source clutter and avoids surprising metadata mutation. |
| 7 | Multiple files | Support one source first unless existing handler code makes multi-source trivial | Reduces UI/server complexity while the job model is new. |
| 8 | Imported document status | Active immediately | Phase 19 can add draft/review; Phase 18 proves ingestion and grounding. |
| 9 | Prompt-injection handling | Deterministic warning flag, do not block by default | Imported text is data; source trust should be visible without over-filtering. |
| 10 | Rich formats | Defer PDF, DOCX, HTML extraction, GitHub/cloud sync | Adapter surface should come after the import spine. |

## Final Scope

### In Scope

- Add a local import job model.
- Persist and list import jobs by tenant/space.
- Add deterministic validation for `.txt`, `.md`, and manual URL snapshots.
- Add a synchronous import path that records `running`, `completed`, `partial`,
  and `failed` states.
- Create imported knowledge documents through existing document upload/indexing
  logic.
- Preserve allowlisted source metadata:
  - `source_type`;
  - `created_from=knowledge_ingestion`;
  - `import_job_id`;
  - `source_uri`;
  - `source_label`;
  - `source_size_bytes`;
  - `source_warning` when deterministic warning checks match.
- Deduplicate exact same content hash inside the same selected space.
- Add `POST /admin/knowledge/import`.
- Add `GET /admin/knowledge/imports?space_id=...`.
- Add compact `/admin` import controls and recent import job rendering.
- Link import results into existing Phase 17 document detail.
- Keep imported documents visible in existing filters, diagnostics, and grounded
  answer flows.
- Update README and release notes with true Phase 18 capabilities after build.

### Out of Scope

- PDF parsing.
- DOCX parsing.
- Server-side URL fetch or crawl.
- Recursive website import.
- GitHub sync.
- Cloud drive sync.
- Folder watching.
- Scheduled imports.
- Background workers.
- External database migration.
- Semantic deduplication.
- LLM-generated source summaries or edits.
- Draft/review/approval workflow for imported content.

## Architecture

```text
web/admin.html
  import controls
  recent import jobs table
  result links to existing document detail

web/admin.js
  collect selected space/source fields
  post import request
  render import job status/results
  refresh knowledge list/detail/health

internal/server
  POST /admin/knowledge/import
  GET /admin/knowledge/imports?space_id=

internal/admin
  KnowledgeImportService
    Validate source
    Create running job
    Dedupe by content hash in selected space
    Call KnowledgeService.Upload for new docs
    Persist final job state

local persistence
  existing knowledge document store
  local import job store/file
```

## Data Model Plan

### `KnowledgeImportRequest`

```go
type KnowledgeImportRequest struct {
    SpaceID     string                  `json:"space_id"`
    SourceType  KnowledgeImportSourceType `json:"source_type"`
    SourceLabel string                  `json:"source_label,omitempty"`
    Sources     []KnowledgeImportSource `json:"sources"`
}
```

Rules:

- `SpaceID` defaults through the existing default-space behavior.
- `SourceType` is required and allowlisted.
- `SourceLabel` is trimmed and bounded.
- `Sources` starts with exactly one source in Phase 18 unless Stage 3 finds
  multi-source support falls out cheaply from service tests.

### `KnowledgeImportSource`

```go
type KnowledgeImportSource struct {
    Name      string            `json:"name"`
    URI       string            `json:"uri,omitempty"`
    Content   string            `json:"content"`
    SizeBytes int64             `json:"size_bytes,omitempty"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}
```

Rules:

- local file imports require `.txt` or `.md` name.
- URL snapshots require `http` or `https` URI.
- `Content` is required after trimming.
- `Metadata` is not blindly persisted; service allowlist decides final keys.

### `KnowledgeImportJob`

```go
type KnowledgeImportJob struct {
    ID                  string                         `json:"id"`
    TenantID            string                         `json:"tenant_id"`
    SpaceID             string                         `json:"space_id"`
    SourceType          KnowledgeImportSourceType      `json:"source_type"`
    SourceLabel         string                         `json:"source_label,omitempty"`
    Status              KnowledgeImportStatus          `json:"status"`
    SourceCount         int                            `json:"source_count"`
    ImportedDocumentIDs []string                       `json:"imported_document_ids"`
    SkippedSources      []KnowledgeImportSkippedSource `json:"skipped_sources"`
    FailureCode         string                         `json:"failure_code,omitempty"`
    FailureCause        string                         `json:"failure_cause,omitempty"`
    Metadata            map[string]string              `json:"metadata,omitempty"`
    CreatedAt           time.Time                      `json:"created_at"`
    UpdatedAt           time.Time                      `json:"updated_at"`
}
```

### `KnowledgeImportSkippedSource`

```go
type KnowledgeImportSkippedSource struct {
    Name               string `json:"name"`
    Reason             string `json:"reason"`
    ExistingDocumentID string `json:"existing_document_id,omitempty"`
}
```

## Validation Rules

| Rule | Behavior | Error/failure code |
| --- | --- | --- |
| Unknown source type | Reject request | `unsupported_source_type` |
| `.txt`/`.md` extension missing for local file | Reject request | `unsupported_extension` |
| URL snapshot without URL | Reject request | `invalid_url` |
| URL snapshot with non-http scheme | Reject request | `invalid_url` |
| Empty content after trim | Reject request | `empty_source_content` |
| Oversized content | Reject request | `source_too_large` |
| Disabled/archived space | Reject request through existing space validation | existing knowledge space error |
| Duplicate content in same space | Complete/partial job with skipped source | `duplicate_content` |
| Prompt-injection-looking text | Import allowed with metadata warning | `source_warning=instruction_like_text` |

Stage 3 should start with a conservative size limit in service tests. The plan
recommends `1 MiB` for the first slice unless existing upload constraints already
define a better value.

## Data Flow

### Import Local Text/Markdown

```text
Admin UI
  -> POST /admin/knowledge/import
  -> server validates JSON shape/auth
  -> KnowledgeImportService.Import
  -> validate selected space and source
  -> compute content hash
  -> search existing docs in same space for exact hash
  -> skip duplicate OR call KnowledgeService.Upload
  -> persist import job
  -> return job JSON
  -> UI refreshes jobs + knowledge list
```

### Import Manual URL Snapshot

```text
Admin UI URL + pasted text
  -> POST /admin/knowledge/import
  -> validate http/https URL and content
  -> source metadata includes URL
  -> existing upload/indexing path creates knowledge doc
  -> import job links created document
```

### Grounding After Import

```text
Imported KnowledgeDocument
  -> existing retrieval diagnostics
  -> existing selected-space chat grounding
  -> existing /app answer-state UI
```

## Implementation Tasks

### T1 - Import Types and Store Contract

Add import job/source/status types and local persistence. Keep this package-level
work independent from server and web.

Expected files:

- `internal/admin/knowledge_import.go`
- `internal/admin/knowledge_import_test.go`
- possible local store helpers in existing store/admin files

Tests first:

- job status JSON round-trip;
- import job persists and reloads;
- existing knowledge data loads when no import job file exists.

### T2 - Import Service Validation

Implement service-level validation and deterministic warning detection.

Tests first:

- rejects unknown source type;
- rejects unsupported extension;
- rejects invalid URL scheme;
- rejects empty source content;
- rejects oversized content;
- marks instruction-like text with `source_warning`.

### T3 - Import Creates Knowledge Documents

Wire import to existing knowledge upload/indexing.

Tests first:

- local Markdown import creates a ready document in selected space;
- URL snapshot import stores URL metadata;
- imported document chunks and hash match existing knowledge behavior;
- disabled/archived space rejects import.

### T4 - Exact Dedupe

Add same-space exact content-hash dedupe.

Tests first:

- same content in same space skips duplicate and records existing document ID;
- same content in different space follows the locked Stage 2 behavior: allowed
  as a separate space-scoped document;
- duplicate job status is `completed` when all sources are skipped, or `partial`
  only if mixed imported/skipped sources exist.

### T5 - Server Routes

Expose the import service through admin-protected routes.

Tests first:

- `POST /admin/knowledge/import` validates request body;
- successful import returns job JSON;
- failed validation returns existing JSON error shape;
- `GET /admin/knowledge/imports?space_id=...` scopes jobs to selected space;
- routes are protected by existing admin/API-key guard.

### T6 - Admin UI Import Panel

Add compact import controls and recent job rendering.

Tests first:

- static tests prove import controls exist;
- static tests prove HTML-like imported text is rendered through safe text
  rendering patterns;
- JS tests/static checks cover source type branches and job rendering labels.

### T7 - Diagnostics and Grounding Regression

Prove imported documents participate in existing knowledge flows.

Tests first:

- retrieval diagnostics finds imported content;
- selected-space grounded answer test can cite imported document;
- Phase 17 curation edit/filter/detail tests still pass.

### T8 - Docs and Release Notes

Update docs only after behavior exists.

Checks:

- README current stage becomes Phase 18;
- README includes local import workflow and verification commands;
- RELEASE_NOTES records only true shipped Phase 18 behavior;
- no docs claim PDF/DOCX/crawling support.

## Test Matrix

| Area | Test | Expected Result |
| --- | --- | --- |
| Types | import job JSON round-trip | status, source type, IDs, timestamps survive |
| Store | import jobs persist locally | reload returns prior jobs |
| Store | no import file exists | existing knowledge data still loads |
| Service validation | unknown source type | stable validation error |
| Service validation | unsupported extension | stable validation error |
| Service validation | invalid URL scheme | stable validation error |
| Service validation | empty content | stable validation error |
| Service validation | oversized content | stable validation error |
| Service warning | instruction-like text | imported with deterministic warning metadata |
| Service import | Markdown file | document created with chunks/hash/source metadata |
| Service import | URL snapshot | document created with URL metadata |
| Service import | disabled space | import rejected |
| Dedupe | same content same space | skipped with existing document ID |
| Dedupe | same content different space | separate import allowed |
| Server | `POST /admin/knowledge/import` success | job JSON returned |
| Server | bad import payload | JSON error and cause returned |
| Server | `GET /admin/knowledge/imports` | jobs scoped by space |
| Security | imported HTML-like text | rendered as text, not HTML |
| Web | controls exist | source selector, inputs, import action, job list |
| Web | job result rendering | status/skipped/failure/imported docs visible |
| Regression | Phase 17 curation | edit/filter/detail still pass |
| Regression | Phase 15 answer state | imported document can ground answer |

## Verification Commands

```powershell
go test ./internal/admin ./internal/server ./internal/knowledge ./internal/agents ./web
go test ./...
go vet ./...
go build ./cmd/server ./cmd/cli ./cmd/smoke
rg -n "Phase 18|Knowledge Source Ingestion|/admin/knowledge/import|local_text_file|url_text_snapshot" README.md RELEASE_NOTES.md docs internal web
```

Browser QA after implementation:

1. Start local server on a non-conflicting port.
2. Open `/admin`.
3. Select or create a knowledge space.
4. Import a Markdown source.
5. Verify job status and document link.
6. Inspect imported document detail.
7. Run diagnostics for imported text.
8. Open `/app`, select same space, ask a covered question.
9. Repeat the same import and verify duplicate/skip is visible.

## Failure Modes Registry

| Failure Mode | Severity | Detection | Mitigation |
| --- | --- | --- | --- |
| Duplicate imports clutter source list | Medium | Dedupe tests and browser QA repeat import | Exact content-hash skip inside same space |
| Imported text executes as HTML | High | Web static test and browser QA | Render with text APIs only |
| Import job lost after restart | Medium | Store reload test | File-backed job persistence |
| URL import accidentally fetches network in CI | High | No server-side fetch implementation | Manual URL text snapshot only |
| Imported source bypasses space validation | High | disabled-space service/server tests | Reuse existing writable-space validation |
| Parser scope expands into binary formats | Medium | plan guardrails and docs rg checks | Keep PDF/DOCX out of Phase 18 |
| Error message hides cause | Medium | server validation tests | Existing JSON `error`/`cause` shape |
| Prompt-injection-like source hidden from operator | Medium | warning detection test | Store/render deterministic warning metadata |

## Error and Rescue Registry

| Error | User sees | Rescue |
| --- | --- | --- |
| Unsupported source type | Import failed with cause | Choose text/Markdown or URL snapshot |
| Unsupported extension | Import failed with cause | Rename/use `.txt` or `.md` source |
| Empty source content | Import failed with cause | Add source text before importing |
| Invalid URL | Import failed with cause | Use `http` or `https` URL for snapshots |
| Duplicate content | Completed/skipped job row | Click existing document ID |
| Source too large | Import failed with cause | Split file or reduce content |
| Disabled space | Import failed with space cause | Enable or choose writable space |

## DX Checklist

- Route names match existing `/admin/knowledge/*` conventions.
- Error JSON includes both high-level error and cause.
- README example uses only local deterministic inputs.
- Smoke/verification commands do not require DeepSeek.
- Import job output includes enough IDs for debugging.
- Tests use temp local storage and deterministic source fixtures.

## NOT in Scope

- Rich binary parsing.
- Crawling or network fetch.
- Async worker implementation.
- Review/approval state.
- External storage.
- Real LLM source processing.

## What Already Exists

- Knowledge spaces and selected-space behavior.
- Knowledge upload, reindex, update, disable, enable, delete.
- Knowledge document metadata and content hash.
- Retrieval diagnostics.
- Knowledge gaps and workbench notes.
- `/admin` knowledge operations page.
- `/app` selected-space grounding and answer-state UI.
- Local-first tests that do not require DeepSeek.

## Cross-Phase Themes

The same theme appears in CEO, Design, Eng, and DX review: **ingestion must be
observable before it becomes broad**. Users should see what was imported, what
was skipped, and why. That visibility is the product wedge and the engineering
guardrail.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Hold adapter scope to text/Markdown and URL snapshots | Auto-decided | Narrowest useful wedge | Proves ingestion without parser/crawler risk | PDF/DOCX/crawling in Phase 18 |
| 2 | CEO | Build import job model despite synchronous execution | Auto-decided | Future-compatible simplicity | Makes status visible now and worker-ready later | Direct upload-only path |
| 3 | Design | Add import controls inside existing `/admin` knowledge section | Auto-decided | Preserve operator context | Space, documents, diagnostics, and gaps already live there | Separate import page |
| 4 | Design | Render jobs as dense rows | Auto-decided | Console ergonomics | Operators scan status/failures faster | Decorative cards/wizard |
| 5 | Eng | Use small import service wrapping `KnowledgeService` | Auto-decided | Reuse established boundaries | Avoids duplicated chunk/hash/index logic | Direct store mutation |
| 6 | Eng | Skip duplicates by same-space content hash | Auto-decided | Explicit source truth | Avoids clutter and surprise metadata mutation | Always create duplicate documents |
| 7 | Eng | Persist import jobs locally | Auto-decided | Local-first architecture | Matches current no-SQLite preference | SQLite/job queue |
| 8 | DX | Keep route names under `/admin/knowledge/*` | Auto-decided | Guessable API | Matches existing route family | New `/imports` namespace |
| 9 | Security | Warn on instruction-like source text, do not block by default | Auto-decided | Data transparency | Imported text is source data but should be visible as risky | Silent import or hard block |

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | --- | --- | --- |
| CEO Review | `$gstack-autoplan` | Scope and strategy | 1 | Clear | Hold ingestion spine; defer rich adapters |
| Design Review | `$gstack-autoplan` | UI/UX gaps | 1 | Clear | Compact admin panel, dense job rows |
| Eng Review | `$gstack-autoplan` | Architecture and tests | 1 | Clear | Import service wraps existing knowledge path |
| DX Review | `$gstack-autoplan` | Developer experience | 1 | Clear | Predictable routes, local examples, actionable errors |

**VERDICT:** CEO + DESIGN + ENG + DX CLEARED for Stage 3 once the user approves this plan.

NO UNRESOLVED DECISIONS
