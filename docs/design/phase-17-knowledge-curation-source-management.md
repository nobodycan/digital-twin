# Phase 17 Knowledge Curation and Source Management Design

Date: 2026-07-04

Status: Spec approved by the user; Stage 2 plan review in progress

Source spec: [Phase 17 Knowledge Curation and Source Management Spec](../specs/phase-17-knowledge-curation-source-management.md)

## Office-Hours Summary

Phase 17 should not race into richer ingestion. The system can already create
knowledge records from workbench gaps, but those records are effectively
append-only from an operator's perspective.

The next product step is to make local knowledge maintainable:

1. find the document or note that matters;
2. understand where it came from;
3. edit it safely;
4. reindex it deterministically;
5. see which gaps it created from or resolved;
6. verify retrieval sees the corrected source.

That is the smallest step from "knowledge base exists" to "knowledge base can
be operated."

## Six Forcing Questions

### 1. Who desperately needs this?

The first user is still the single operator maintaining a professional digital
human locally. They need to correct and curate their own knowledge faster than
the assistant accumulates unsupported answers.

### 2. What is the status quo?

The operator can upload documents, create notes from gaps, inspect health, and
run diagnostics. But once a note is wrong or stale, the workflow is clumsy:
delete, recreate, re-test, and remember which gap it was tied to.

### 3. What is the narrowest painful wedge?

Edit a workbench note, keep its gap provenance, reindex it, and verify retrieval
with diagnostics.

### 4. What should we avoid building too early?

Avoid rich file ingestion, cloud sync, semantic dedupe, LLM-authored edits, and
workflow assignment. Those are real later features, but they are not needed to
prove curation.

### 5. What can we observe after shipping?

We can observe whether operators filter to workbench notes, edit records instead
of deleting them, and use diagnostics after editing.

### 6. How does this fit the future?

Future ingestion formats need source management. Future LLM suggestions need a
human-reviewed edit path. Future governance needs document-gap provenance. Phase
17 lays that groundwork locally.

## Recommended Product Shape

Build a **Curation View** inside the existing `/admin` knowledge section.

It should add:

- searchable/filterable document list;
- document detail with readable source metadata;
- edit mode for local text/Markdown records;
- relationship projection for gap provenance;
- diagnostics entry points from selected documents.

It should not create a second knowledge system. It should reuse existing
document storage, indexing, diagnostics, and gap stores.

## Architecture

```text
internal/admin
  KnowledgeService
    List/ListBySpace with optional filter projection
    Update document content/title
    DocumentDetail with source metadata
    DocumentRelations from KnowledgeGapService

internal/server
  GET /admin/knowledge?space_id=&query=&source_type=&status=&gap_linked=
  GET /admin/knowledge/{document_id}/detail
  POST /admin/knowledge/update
  POST /admin/knowledge/retrieval-diagnostics

web/admin.js
  filter controls
  selected document detail
  edit form
  relationship rendering
  diagnostics from selected document
```

Stage 2 should verify whether a new detail route is needed for relations or
whether the existing detail response can grow safely.

## Component Design

### `internal/admin`

Recommended service additions:

- `KnowledgeUpdate` input:
  - `DocumentID`
  - `Title`
  - `Content`
  - optional validated source label
- `KnowledgeDocumentFilter` input:
  - `SpaceID`
  - `Query`
  - `Status`
  - `SourceType`
  - `GapLinkedOnly`
- `KnowledgeDocumentRelations` projection:
  - `SourceGap`
  - `ResolvedGaps`

Implementation preference:

- update documents through `KnowledgeService`, not directly through the store;
- reuse existing chunking/hash/index-state logic;
- keep metadata validation centralized in the service;
- treat unknown legacy metadata as readable but not browser-editable.

### `internal/server`

Candidate route:

```text
POST /admin/knowledge/update
```

Request:

```json
{
  "document_id": "note-gap-123",
  "title": "DeepSeek startup command",
  "content": "Use scripts/start-deepseek.ps1 ...",
  "source_label": "operator note"
}
```

Response: updated knowledge document JSON, including chunks/index-state.

Validation:

- document ID is required;
- title and content cannot both become empty;
- unsafe metadata keys are rejected;
- document space must be writable;
- archived or disabled spaces reject updates.

### `web/admin`

Add curation controls near the knowledge list:

- search input;
- status selector;
- source-type selector;
- gap-linked toggle;
- edit button in selected document detail;
- save/cancel edit actions;
- relationship block.

The UI should preserve the current space selection. Filtering should feel like
working inside the selected space, not across all tenants or hidden scopes.

## Data Flow

### Edit Knowledge Document

1. Operator selects a document in `/admin`.
2. Detail panel shows source metadata and relationships.
3. Operator enters edit mode.
4. UI posts validated title/content to `/admin/knowledge/update`.
5. Server calls `KnowledgeService.Update`.
6. Service validates writable space and editable document type.
7. Service reuses the existing chunk/hash/index path.
8. Store persists updated document.
9. UI refreshes list, detail, health, and diagnostics context.

### Render Source Relationships

1. Detail request loads the document.
2. Service reads gap records for the same tenant and space.
3. Projection matches:
   - document metadata `source_gap_id`;
   - gaps with `resolved_by_document_id == document.ID`.
4. UI renders readable relationship rows.

### Verify Edited Content

1. Operator saves an edit.
2. Operator runs diagnostics using manual query or suggested title/body snippet.
3. Existing diagnostics response renders ranked chunks and reasons.
4. Operator can confirm the edited document appears as expected.

## Scope Guardrails

- No new persistence engine.
- No background worker.
- No automatic provider-written edits.
- No arbitrary metadata mutation from the browser.
- No user-facing `/app` knowledge editor.
- No rich ingestion formats in this phase.

## Stage 2 Decisions

Stage 2 autoplan should lock:

1. server-side versus client-side document filtering;
2. exact editable document type policy;
3. whether `UpdatedAt` becomes a stored field;
4. whether relations are returned by document detail or a new endpoint;
5. exact test order for TDD;
6. browser QA flow for curation.

## Risk Register

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Edit path corrupts chunk/index metadata | High | Reuse existing reindex path and add store reload tests |
| Browser can mutate arbitrary metadata | High | Validate an allowlist in admin service/server |
| Curation UI becomes cluttered | Medium | Keep filters compact and detail-driven |
| Relationship projection is slow later | Low now | Local file store is small; defer indexing until needed |
| Legacy documents lack freshness fields | Medium | Treat missing fields as backward-compatible unknown |
| Disabled space edits slip through | Medium | Reuse writable-space validation from upload/reindex |

## Success Signal

Phase 17 succeeds when a user can:

1. open `/admin`;
2. filter to workbench notes in a knowledge space;
3. select a note created from a gap;
4. edit its content;
5. save without losing the document ID or source gap metadata;
6. see any resolved gaps linked to that document;
7. run diagnostics and find the updated chunk.

## Assignment For Stage 2

Run `$gstack-autoplan` against this spec and design. The plan should produce a
TDD matrix for:

- document update/reindex behavior;
- metadata validation and preservation;
- document filtering/search;
- document-gap relationship projection;
- server route validation;
- admin UI curation controls;
- regression coverage for Phase 16 gap and note workflows.
