# Phase 12 Knowledge Space Management and Grounded Answering Design

Date: 2026-06-30

Status: Autoplan review complete, waiting for implementation approval

Source spec: [Phase 12 Knowledge Space Management and Grounded Answering Spec](../specs/phase-12-knowledge-space-management-grounded-answering.md)

## Office-Hours Summary

Phase 10 gave the digital human a real local knowledge base. Phase 11 made
retrieval quality measurable. Phase 12 should stop treating knowledge as a flat
document pile and make it an operator-managed set of knowledge spaces.

The key product move is scoped grounding:

- operators manage spaces;
- documents belong to a space;
- retrieval diagnostics run inside a selected space;
- chat answers cite only sources from the selected active space;
- no-source states are explicit instead of silently searching unrelated sources.

## Premises

1. The user wants a professional digital human, not a toy upload-and-chat demo.
2. Knowledge management must separate domains of knowledge before adding more
   ingestion formats.
3. Local file storage remains the persistence layer for now.
4. Phase 11 retrieval quality work should be reused, not rewritten.
5. A grounded answer is only trustworthy when the knowledge scope is visible and
   enforced.

## Narrowest Valuable Wedge

Add a single primary `space_id` to knowledge documents and guarantee a default
space.

That yields the highest leverage with manageable complexity:

- old documents keep working through default-space migration;
- new uploads target a selected space;
- retrieval filters by space;
- `/admin` becomes space-first;
- `/app` can show the active space and avoid cross-domain citations.

Many-to-many document membership, complex ACLs, and external ingestion can wait.

## Recommended Architecture

```text
KnowledgeSpace
  -> FileKnowledgeStore migration/default space
  -> KnowledgeService lifecycle APIs
  -> Space-scoped retrieval request
  -> Phase 11 retrieval pipeline
  -> Runtime grounding metadata
  -> /app selected-space source state
```

## Component Design

### `internal/admin`

Responsibilities:

- own `KnowledgeSpace` lifecycle;
- enforce tenant ID;
- validate names, statuses, and retrieval modes;
- assign default space for legacy documents;
- manage document `space_id` membership;
- expose safe import/index state.

Recommended API additions:

- `CreateKnowledgeSpace`
- `ListKnowledgeSpaces`
- `UpdateKnowledgeSpace`
- `DisableKnowledgeSpace`
- `ArchiveKnowledgeSpace`
- `ListKnowledgeDocuments(spaceID)`
- `MoveKnowledgeDocument(documentID, spaceID)`

### `internal/admin.FileKnowledgeStore`

Responsibilities:

- persist spaces and documents in the existing local JSON shape or a compatible
  extension;
- load old files that lack spaces;
- synthesize and persist a default space when needed;
- keep atomic write behavior.

Preferred first storage shape:

```json
{
  "spaces": [],
  "documents": []
}
```

If current data is already a document array, the loader should accept both shapes.

### `internal/knowledge`

Responsibilities:

- accept `space_id` in retrieval requests;
- filter candidate documents before lexical/vector stages;
- include scope metadata in diagnostics;
- preserve Phase 11 score explanations.

The retrieval pipeline should not own space lifecycle. It receives already
authorized and filtered candidate documents or a request that the service resolves
into candidates.

### `internal/runtime`

Responsibilities:

- resolve selected knowledge space from request/session/default;
- pass scoped retrieval context into persona generation;
- emit allowlisted metadata:
  - `knowledge_space_id`;
  - `knowledge_space_name`;
  - `knowledge_used`;
  - `knowledge_citations`;
  - `no_source_reason`;
  - `retrieval_mode`.

Runtime should not silently broaden scope if selected space has no result.

### `web/admin`

Information architecture:

1. space list;
2. selected-space document list;
3. selected document detail;
4. selected-space diagnostics.

The admin page should stay dense and operational. This is not a marketing screen.

### `web/app`

User-facing scope:

- show active space near conversation controls;
- allow a basic selector if Stage 2 keeps it small;
- show selected space in grounded assistant turns;
- show no-source reason compactly when relevant.

## Data Flow

### Legacy Load

1. Store loads knowledge data.
2. If no spaces exist, create a default active space.
3. If documents lack `space_id`, assign default space in memory and persist on
   the next write or explicit migration action.
4. Existing retrieval behavior remains available through default scope.

### Upload

1. `/admin` selects a space.
2. Upload request includes `space_id`.
3. Server enforces tenant and validates active space.
4. Knowledge service chunks and stores the document with that `space_id`.
5. Admin UI refreshes selected-space documents and diagnostics.

### Scoped Chat

1. `/app` sends the user turn with selected or default `space_id`.
2. Runtime validates the space is active.
3. Retrieval runs only against ready documents in that space.
4. Persona prompt receives only accepted chunks from that space.
5. SSE metadata includes selected space and citation/no-source state.
6. `/app` renders grounding state.

## Decisions for Stage 2

| Decision | Recommended Default | Why |
| --- | --- | --- |
| Membership | One primary `space_id` per document | Avoids ambiguity and keeps migration simple |
| Default space | Always present | Backward compatibility for Phase 10/11 data |
| Deleting spaces | Archive first | Prevents accidental document loss |
| App selector | Small selector if low-cost; otherwise default only | Grounding scope matters, but UI should not sprawl |
| Reindex | Document-level required; space-wide optional | Keeps first implementation tractable |
| Storage | Backward-compatible JSON envelope | Keeps local-first persistence and migration simple |

## Risk Register

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Migration loses or hides existing documents | High | Backward-compatible loader and default-space tests |
| Space scope silently broadens on no-result | High | Explicit no-source behavior and tests |
| Admin UI becomes cluttered | Medium | Space-first layout with compact tables |
| Many-to-many membership becomes necessary quickly | Medium | Design `space_id` so later `space_ids` migration is possible |
| Archive/delete semantics confuse operators | Medium | Stage 2 must choose conservative copy and behavior |
| Runtime metadata leaks internals | High | Continue explicit metadata allowlist |

## Success Signal

Phase 12 is successful when a developer can:

1. start with existing Phase 11 data and see it under a default space;
2. create a second space and upload a document into it;
3. ask the same query against different spaces and get space-scoped citations;
4. see clear no-source behavior when the selected space lacks support;
5. run local tests without DeepSeek, SQLite, or external vector services.

## Assignment for Stage 2

Turn this design into an implementation plan with:

- exact file-store migration behavior;
- API route list and request/response shapes;
- TDD test matrix ordered RED -> GREEN -> REFACTOR;
- UI states for empty, disabled, archived, failed, and stale spaces/documents;
- scoped retrieval and no-source acceptance criteria.

Recommended first Stage 2 decision:

> Use one primary `space_id` per document, create a guaranteed default space, and
> archive spaces rather than physically deleting them in Phase 12.
