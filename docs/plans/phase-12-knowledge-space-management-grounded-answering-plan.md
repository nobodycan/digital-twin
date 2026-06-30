# Phase 12 Knowledge Space Management and Grounded Answering Plan

Date: 2026-06-30

Status: Draft, waiting for user approval

Source spec: [Phase 12 Knowledge Space Management and Grounded Answering Spec](../specs/phase-12-knowledge-space-management-grounded-answering.md)

Source design: [Phase 12 Knowledge Space Management and Grounded Answering Design](../design/phase-12-knowledge-space-management-grounded-answering.md)

## Autoplan Summary

Phase 12 should turn the existing flat local knowledge base into a scoped,
operator-managed knowledge-space system. The plan keeps Phase 11's retrieval
pipeline intact and adds the missing product boundary around it: default space
migration, document membership, scoped diagnostics, scoped grounded chat, and
compact UI state.

The implementation should be deliberately local-first:

- no SQLite or external search service;
- no provider-required embeddings in CI;
- no new ingestion formats;
- no many-to-many document membership in the first slice.

## What Already Exists

Current code gives Phase 12 useful foundations:

- `internal/admin.KnowledgeService` owns upload/list/get/disable/enable/delete/reindex.
- `internal/admin.FileKnowledgeStore` persists `knowledge.json` as a flat document array.
- `internal/knowledge.Pipeline` supports lexical/vector/hybrid/auto diagnostics.
- `internal/knowledge.Service` loads tenant documents and runs grounding/diagnostics.
- `internal/agents.PersonaAgent` augments prompts with retrieved sources and emits
  grounding metadata.
- `internal/runtime.allowlistedGenerationMetadata` controls which grounding fields
  reach SSE clients.
- `web/admin` already lists documents and runs retrieval diagnostics.
- `web/app` already renders citation chips and no-source state.

## Not In Scope

- multiple spaces per document;
- PDF/DOCX/web/Notion/GitHub ingestion;
- SQLite/Postgres/vector database migration;
- authentication/RBAC redesign;
- long-running background indexing workers;
- LLM-based source curation;
- full design-system redesign of `/admin` or `/app`.

## CEO Review

Phase 12 is product-correct because flat document retrieval will become confusing
as soon as the knowledge base grows. The right wedge is knowledge scope, not more
formats. A professional digital human must know which body of knowledge it is
answering from.

CEO decisions:

1. Use one primary `space_id` per document.
2. Create a guaranteed active default space.
3. Archive spaces instead of physically deleting them.
4. Keep document delete behavior unchanged for now, but never delete documents
   just because a space is archived.
5. Make runtime retrieval refuse to broaden scope silently.

Score: 9/10. The plan is ambitious enough to advance the product but still
bounded enough for TDD.

## Design Review

The UI should make scope visible without making chat feel like an admin console.
`/admin` should become space-first; `/app` should show the selected space and the
answer's source state compactly.

Design decisions:

1. `/admin` gets a space list/control area above the knowledge document table.
2. The document table remains dense and operational.
3. Diagnostics are scoped to the selected space.
4. `/app` gets a small knowledge-space selector or status row near session
   diagnostics.
5. Assistant turns include space name when knowledge is grounded or no-source.

Score: 8/10. The main risk is density. The fix is to keep the first UI
implementation simple and state-driven.

## Engineering Review

Phase 12 should extend existing package boundaries instead of introducing a new
storage layer.

Architecture:

```text
web/admin space controls
  -> server knowledge-space routes
  -> admin.KnowledgeService
  -> FileKnowledgeStore JSON envelope + legacy array loader
  -> knowledge.Service scoped documents
  -> knowledge.Pipeline unchanged ranking
  -> PersonaAgent grounding metadata
  -> runtime SSE allowlist
  -> web/app scope and citation UI
```

Key package decisions:

| Area | Decision |
| --- | --- |
| Model | Add `KnowledgeSpace` and `KnowledgeDocument.SpaceID`. |
| Storage | Load both old flat array and new envelope `{ "spaces": [], "documents": [] }`. |
| Migration | Synthesize/persist default space and assign legacy docs to it. |
| Retrieval | Add `SpaceID` to `knowledge.SearchRequest` and filter before pipeline stages. |
| Runtime | Carry selected/default `space_id` through conversation metadata or turn request metadata where least invasive. |
| Metadata | Add `knowledge_space_id`, `knowledge_space_name`, `knowledge_no_source_reason` to allowlist. |
| UI | Keep vanilla HTML/JS; no frontend framework. |

Score: 8/10. The risk is accidentally creating a parallel knowledge abstraction.
Keep lifecycle in `internal/admin` and retrieval in `internal/knowledge`.

## DX Review

The developer path should remain copy-paste simple:

1. start server;
2. open `/admin`;
3. create or use the default space;
4. upload mock knowledge into a space;
5. run scoped diagnostics;
6. open `/app`;
7. ask a question and see space-scoped citations.

TTHW target: under 5 minutes after `go run ./cmd/server`.

DX decisions:

1. API errors must include stable codes such as `knowledge_space_missing`,
   `knowledge_space_disabled`, and `knowledge_scope_empty`.
2. README should document default-space migration and scoped diagnostics.
3. Tests should not require pre-existing local data.
4. Existing DeepSeek startup scripts remain valid.

Score: 8/10. The main DX risk is unclear migration behavior; tests and docs must
make the default-space story explicit.

## Data Model

### `KnowledgeSpace`

```go
type KnowledgeSpace struct {
    ID                   string
    TenantID             string
    Name                 string
    Description          string
    Status               KnowledgeSpaceStatus
    DefaultRetrievalMode string
    Tags                 []string
    CreatedAt            time.Time
    UpdatedAt            time.Time
}
```

Statuses:

- `active`
- `disabled`
- `archived`

### `KnowledgeDocument`

Add:

- `SpaceID string`
- optional `ImportStatus string` only if implementation finds the existing
  `Status` + metadata insufficient.

Recommended first step: add only `SpaceID`, reuse existing status and metadata
for index state.

### Storage Envelope

```json
{
  "spaces": [
    {
      "id": "default",
      "tenant_id": "tenant-1",
      "name": "Default",
      "status": "active",
      "default_retrieval_mode": "auto"
    }
  ],
  "documents": []
}
```

Legacy loader must still accept:

```json
[
  { "id": "old-doc", "tenant_id": "tenant-1" }
]
```

## API Plan

Add routes:

- `GET /admin/knowledge/spaces`
- `POST /admin/knowledge/spaces/create`
- `POST /admin/knowledge/spaces/update`
- `POST /admin/knowledge/spaces/disable`
- `POST /admin/knowledge/spaces/enable`
- `POST /admin/knowledge/spaces/archive`
- `POST /admin/knowledge/move`

Extend existing routes:

- `GET /admin/knowledge?space_id=default`
- `POST /admin/knowledge/upload` accepts `space_id`
- `POST /admin/knowledge/retrieval-diagnostics` accepts `space_id`
- `/experience/stream` accepts selected space through request/conversation
  metadata if available; otherwise uses default space.

Keep JSON POST mutations to match current server style.

## Implementation Tasks

### P12-01 Space Model and Validation

Files:

- `internal/admin/knowledge.go`
- `internal/admin/knowledge_test.go`

Tasks:

- Add `KnowledgeSpace` model, statuses, errors, and ID validation reuse.
- Add default space constants.
- Add tests for invalid IDs, missing names, active/disabled/archive transitions.

RED tests first:

- default space has stable ID and active status;
- invalid space ID rejects path traversal;
- disabled/archived statuses validate.

### P12-02 Store Envelope and Legacy Migration

Files:

- `internal/admin/knowledge_file_store.go`
- `internal/admin/knowledge_file_store_test.go`

Tasks:

- Replace internal load/save with a storage envelope.
- Accept legacy flat document array.
- Assign missing `space_id` to default space.
- Persist the envelope on first mutation.
- Preserve atomic write behavior.

RED tests first:

- old flat JSON loads and lists default space;
- old documents appear under `default`;
- saving a migrated document writes envelope form;
- corrupt JSON returns safe error.

### P12-03 Knowledge Service Space Lifecycle

Files:

- `internal/admin/knowledge.go`
- `internal/admin/knowledge_test.go`

Tasks:

- Add `CreateSpace`, `ListSpaces`, `UpdateSpace`, `DisableSpace`,
  `EnableSpace`, `ArchiveSpace`.
- Add `ListBySpace`.
- Extend `Upload` with `SpaceID`.
- Add `MoveDocument`.

RED tests first:

- upload to disabled space fails;
- list by space excludes other spaces;
- archived space does not delete documents;
- move document changes retrieval scope.

### P12-04 Scoped Retrieval Service

Files:

- `internal/knowledge/service.go`
- `internal/knowledge/pipeline.go`
- `internal/knowledge/service_test.go`
- `internal/knowledge/pipeline_test.go`

Tasks:

- Add `SpaceID` to `SearchRequest`.
- Filter documents by space before ready-status filtering.
- Add included/excluded counts to diagnostics if small enough.
- Add no-source reasons: `space_missing`, `space_disabled`,
  `no_ready_documents_in_space`, `no_matching_chunks`.

RED tests first:

- same query in two spaces returns selected-space result;
- disabled space returns no source;
- vector-unavailable still reports scoped lexical fallback.

### P12-05 Runtime and Persona Metadata

Files:

- `pkg/types/contracts.go`
- `internal/app/bootstrap.go`
- `internal/agents/experts.go`
- `internal/runtime/orchestrator.go`
- related tests

Tasks:

- Carry selected space ID through conversation/turn metadata using the existing
  metadata pattern if available.
- Extend `agents.Grounding` with `SpaceID` and `SpaceName`.
- Add grounding metadata keys:
  - `knowledge_space_id`
  - `knowledge_space_name`
  - `knowledge_no_source_reason`
- Add those keys to runtime allowlist.
- Ensure no-source in selected space does not cite another space.

RED tests first:

- generated result includes space metadata when grounded;
- no-source includes selected space and no citations;
- SSE metadata preserves space fields.

### P12-06 Server Routes

Files:

- `internal/server/server.go`
- `internal/server/server_test.go`

Tasks:

- Add space lifecycle handlers.
- Extend document list/upload/diagnostics request structs.
- Keep tenant authoritative.
- Return safe stable error codes.

RED tests first:

- `GET /admin/knowledge/spaces` returns default space;
- upload with `space_id` stores in that space;
- diagnostics with `space_id` excludes other spaces;
- disabled space returns actionable error/no-source response.

### P12-07 Admin UI

Files:

- `web/admin.html`
- `web/admin.js`
- `web/app.css`
- `web/app_static_test.go`

Tasks:

- Add space selector/list and create input.
- Filter document table by selected space.
- Include selected space in upload and diagnostics requests.
- Show space empty/disabled/archived states.
- Keep table layout stable.

RED tests first:

- static test sees `knowledge-space` controls;
- JS contains space route calls;
- diagnostics request includes `space_id`.

### P12-08 App UI

Files:

- `web/app.html`
- `web/app.js`
- `web/app.css`
- `web/app_phase8_regression_test.go`

Tasks:

- Add compact current-space display or selector.
- Include selected `space_id` in conversation payload metadata.
- Render citation chips with space label.
- Render no-source state with selected space.

RED tests first:

- app payload includes selected knowledge space;
- metadata renders space label;
- no-source state does not show citation chips.

### P12-09 Docs and Release Notes

Files:

- `README.md`
- `RELEASE_NOTES.md`
- `docs/specs/phase-12-knowledge-space-management-grounded-answering.md`
- `docs/design/phase-12-knowledge-space-management-grounded-answering.md`
- this plan

Tasks:

- Document Phase 12 behavior.
- Add copy-paste local workflow.
- Explain default-space migration and local-first limits.
- Update release notes once implementation ships.

## Test Matrix

| ID | Area | Scenario | Expected Result |
| --- | --- | --- | --- |
| T12-01 | Model | Invalid space ID contains slash | Validation fails |
| T12-02 | Store | Legacy flat `knowledge.json` | Loads with default active space |
| T12-03 | Store | Legacy docs lack `space_id` | Docs list under default space |
| T12-04 | Store | Save after migration | Writes envelope with spaces/documents |
| T12-05 | Service | Create/update/list space | State persists and reloads |
| T12-06 | Service | Upload into disabled space | Request fails with stable error |
| T12-07 | Service | Move document between spaces | Old space no longer lists it |
| T12-08 | Retrieval | Same query in two spaces | Selected space result only |
| T12-09 | Retrieval | Selected space disabled | No citations, disabled-space reason |
| T12-10 | Retrieval | No ready docs in selected space | No-source reason is scoped |
| T12-11 | Runtime | Grounded answer | Metadata includes space and citations |
| T12-12 | Runtime | Unsupported selected space | No fake citation from other spaces |
| T12-13 | Server | List spaces route | Default space appears |
| T12-14 | Server | Upload with `space_id` | Document stored in selected space |
| T12-15 | Server | Diagnostics with `space_id` | Response is scoped |
| T12-16 | Web admin | Space controls present | Static tests pass |
| T12-17 | Web admin | Selected space changes | Documents/diagnostics use selected space |
| T12-18 | Web app | Payload includes selected space | Backend can scope retrieval |
| T12-19 | Web app | Grounded metadata | Space label and citations render |
| T12-20 | Docs | README/release notes | Explain local workflow and limits |

## RED -> GREEN -> REFACTOR Order

1. P12-01 model tests and constants.
2. P12-02 file-store migration tests.
3. P12-03 service lifecycle tests.
4. P12-04 scoped retrieval tests.
5. P12-05 runtime/persona metadata tests.
6. P12-06 server route tests.
7. P12-07 admin UI static/behavior tests.
8. P12-08 app UI static/behavior tests.
9. P12-09 docs checks.

Do not implement UI before storage/retrieval tests pass. Otherwise the frontend
will encode assumptions that the backend has not proven.

## Failure Modes Registry

| Failure | Severity | Detection | Recovery |
| --- | --- | --- | --- |
| Legacy data hidden after migration | High | Store migration tests | Synthesize default space and assign docs |
| Cross-space citation | High | Retrieval/runtime tests | Filter by `space_id` before ranking |
| Disabled space still used | High | Retrieval/server tests | Return scoped no-source reason |
| Metadata dropped from SSE | Medium | Runtime/presentation tests | Extend allowlist |
| Admin creates unreachable space | Medium | Server/UI tests | Validate and refresh selected space |
| UI overcrowding | Medium | Static tests and manual QA | Keep compact rows and status labels |
| File-store write corruption | Medium | Existing atomic write pattern tests | Continue temp-file rename |

## Error and Rescue Registry

| Error Code | Cause | User-Facing Fix |
| --- | --- | --- |
| `knowledge_space_missing` | Selected space ID does not exist | Select an existing space or create it |
| `knowledge_space_disabled` | Selected space is disabled | Enable the space or choose another |
| `knowledge_space_archived` | Selected space is archived | Restore/enable a different space |
| `knowledge_scope_empty` | Space has no ready documents | Upload or enable documents in this space |
| `knowledge_document_wrong_space` | Document action targets another space | Refresh and retry inside the selected space |

## Verification Commands

```powershell
go test ./internal/admin ./internal/knowledge ./internal/agents ./internal/runtime ./internal/server ./web
go test ./...
go vet ./...
rg -n "Phase 12|Knowledge Space|space_id|knowledge_space_id|default space" docs README.md RELEASE_NOTES.md internal web
```

Optional local smoke after implementation:

```powershell
.\scripts\start-deepseek.ps1 -Port 18080
# open http://127.0.0.1:18080/admin
# create or use Default, upload mock knowledge, run scoped diagnostics
# open http://127.0.0.1:18080/app and ask a source-grounded question
.\scripts\stop-server.ps1 -Port 18080
```

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Use one primary `space_id` per document | Scope | Simplicity | Prevents retrieval ambiguity and keeps migration small | Many-to-many membership |
| 2 | CEO | Guarantee a default active space | Architecture | Backward compatibility | Existing Phase 10/11 data keeps working | Manual migration |
| 3 | CEO | Archive spaces instead of deleting them | Product safety | Reversibility | Avoids accidental document loss | Physical space delete |
| 4 | Engineering | Extend `FileKnowledgeStore` with envelope loader | Architecture | Local-first | Keeps current storage while enabling spaces | New DB |
| 5 | Engineering | Reuse Phase 11 pipeline | Architecture | Preserve working contracts | Retrieval quality stays stable | Rewrite retriever |
| 6 | Design | Space-first admin layout | UX | Context visibility | Operators manage scope before documents | Flat document table |
| 7 | DX | Stable error codes for space issues | DX | Actionable errors | Users know what to fix | Generic 400/500 causes |

## Open Items for Implementation

1. Decide exact request shape for passing `space_id` from `/app` to backend:
   conversation metadata is preferred if it already exists in `pkg/types`.
2. Decide whether the first UI includes create-space editing inline or a minimal
   create button plus selected-space list.
3. Decide whether space-wide reindex ships in Phase 12 or remains document-level.

Recommended defaults:

- pass `space_id` through metadata if available;
- ship minimal create/list/select UI;
- defer space-wide reindex unless it falls out cheaply from service code.

