# Phase 12 Knowledge Space Management and Grounded Answering Spec

Date: 2026-06-30

Status: Approved by the user on 2026-06-30

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 10 Knowledge Base and Memory Control Spec](./phase-10-knowledge-base-memory-control.md)
- [Phase 11 Knowledge Retrieval Quality and RAG Evaluation Spec](./phase-11-knowledge-retrieval-quality-rag-evaluation.md)
- Current implementation: local file-backed knowledge documents, deterministic lexical retrieval, optional vector-style retrieval pipeline, retrieval diagnostics, RAG eval fixtures, grounded chat metadata, and `/admin` knowledge controls.
- User direction: "I have ambition; I also want knowledge base management, not only memory."

## Context

Phase 10 made knowledge manageable at the document level. Phase 11 made retrieval
quality inspectable and measurable. The product now has a credible local RAG
foundation, but it is still organized around individual documents and diagnostic
queries.

That is not yet how a professional digital human should manage knowledge.

A professional operator needs to curate domains of knowledge, not only upload
files. They need to know which sources are active for a conversation, why an
answer used one source instead of another, whether a document import is stale,
and whether a response is actually grounded in the selected knowledge scope.

The next product step is to turn "knowledge documents" into "knowledge spaces":
operator-managed collections with lifecycle, source scope, import status,
answer-grounding rules, and clear user-facing behavior.

## Goal

Deliver a Phase 12 slice that lets the digital human operate against curated
knowledge spaces:

1. create, list, update, enable, disable, and delete local knowledge spaces;
2. attach uploaded documents to one or more spaces without losing existing local
   storage guarantees;
3. filter retrieval and diagnostics by selected space, tags, document status,
   and source type;
4. expose import/index status clearly enough that operators can recover from
   failed or stale documents;
5. make chat answer grounding respect the selected knowledge scope;
6. keep all tests deterministic and local-first, with no SQLite, external vector
   database, or mandatory provider calls.

## Office-Hours Premise Challenge

The obvious next step is "make the knowledge admin prettier" or "add more file
types." Both are useful, but neither is the core issue.

The current product risk is that a user sees citations and assumes the digital
human knows which body of knowledge it should use. In reality, the system has a
flat document set. As documents grow, flat retrieval creates product confusion:

- unrelated documents can compete in retrieval;
- operators cannot stage a new knowledge set before enabling it;
- users cannot choose whether they are asking against product docs, personal
  notes, policies, or project planning;
- failed imports and stale indexes are diagnostic details instead of first-class
  operational states.

The better premise for Phase 12 is:

> Knowledge base management is a scope and operations problem before it is an
> ingestion-format problem.

This keeps the phase ambitious while avoiding a premature jump into PDF parsing,
cloud sync, or a managed search stack.

## Product Thesis

Phase 12 should make `/admin` feel like a small knowledge-operations console and
make `/app` feel like a grounded assistant working inside an explicit knowledge
scope.

The digital human should be able to communicate:

- "I am answering from this knowledge space."
- "This source is disabled, stale, failed indexing, or outside the selected
  scope."
- "I found related knowledge but not enough evidence to answer from it."
- "This answer cites sources from the selected scope only."

The value is not only better retrieval. It is trust through controllable context.

## In Scope

### Knowledge Spaces

- Add a local knowledge-space model with:
  - space ID;
  - tenant ID;
  - name;
  - description;
  - status: `active`, `disabled`, `archived`;
  - created/updated timestamps;
  - default retrieval mode;
  - optional tags or domain labels.
- Add lifecycle operations:
  - create space;
  - list spaces;
  - update name/description/default retrieval mode;
  - enable/disable space;
  - archive or delete space according to Stage 2 decision;
  - list documents in a space.
- Preserve a default space so existing Phase 10/11 documents continue to work
  after migration.

### Document-to-Space Membership

- Attach uploaded documents to a knowledge space at upload time.
- Allow changing membership for existing documents.
- Keep document IDs, chunk IDs, and index state stable when moving a document
  between spaces where possible.
- Retrieval must only consider ready/enabled documents inside the selected space
  unless the request explicitly asks for all spaces.

### Import and Index Operations

- Upgrade document state visibility from "ready/disabled" to an operator-facing
  lifecycle:
  - uploaded;
  - chunked;
  - lexical-ready;
  - vector-ready/vector-missing/vector-failed;
  - stale;
  - failed.
- Reindex can target:
  - one document;
  - all documents in a space.
- Import/index failures must be recoverable and secret-safe:
  - expose safe error codes;
  - never expose API keys, raw local paths, or hidden prompts;
  - preserve lexical retrieval when vector indexing fails.

### Scoped Retrieval Diagnostics

- Extend retrieval diagnostics to accept:
  - `space_id`;
  - tags;
  - document status filters;
  - retrieval mode override;
  - top K.
- Diagnostics should show:
  - selected space;
  - included/excluded document counts;
  - result score breakdown from Phase 11;
  - no-source reason such as no ready documents, disabled space, no lexical
    match, vector unavailable, or below grounding threshold.

### Grounded Answering

- `/app` should send or infer a selected knowledge space.
- Runtime should retrieve only from the selected active space.
- Assistant metadata should include:
  - selected knowledge space ID/name;
  - knowledge used;
  - citation list;
  - no-source reason when no usable source supports the answer;
  - retrieval mode.
- If the selected space is disabled or empty, the assistant should not silently
  fall back to unrelated documents.
- The system should support two answer modes:
  - general persona answer with visible "no source used" state;
  - grounded answer when sufficient evidence exists in the selected space.

### Admin UX

- `/admin` should show spaces first, then documents inside the selected space.
- Knowledge document rows should remain dense and operational:
  - name;
  - status;
  - space;
  - chunk count;
  - index state;
  - updated time;
  - actions.
- Add a space-scoped diagnostics panel.
- Keep memory management separate from knowledge management.

### App UX

- `/app` should show the current knowledge scope without turning the chat into a
  dashboard.
- Assistant turns should make source state clear:
  - knowledge space used;
  - source citations;
  - insufficient support;
  - no source used;
  - fallback/provider state from earlier phases.
- The first implementation can use a simple selector or session default rather
  than a full workspace navigation model.

## Out of Scope

- SQLite, Postgres, managed search, hosted vector databases, or cloud object
  storage.
- PDF, DOCX, web crawling, Notion, Google Drive, GitHub sync, or browser capture
  ingestion.
- Real-time collaborative editing.
- Full RBAC or per-document ACLs.
- Billing, quota enforcement, or SaaS workspace administration.
- LLM-based autonomous source curation.
- Provider-required embeddings in CI.
- Replacing the Phase 11 retrieval pipeline.

## Alternatives Considered

### Alternative A: Visual Polish Only

Restyle `/admin` and `/app` around the existing flat document model.

Pros:

- visible improvement quickly;
- low backend risk;
- useful after Phase 11 diagnostics made the page denser.

Cons:

- does not solve source scope;
- flat retrieval still becomes confusing as documents grow;
- makes the product look more mature than its knowledge operations really are.

Verdict: worth doing as part of Phase 12 UI work, but not sufficient as the phase
spine.

### Alternative B: More Ingestion Formats First

Add PDF/DOCX/web import before changing knowledge organization.

Pros:

- exciting demo value;
- users have many real documents in these formats;
- expands the apparent knowledge-base surface.

Cons:

- creates more content before the product can manage scope well;
- parsing quality and binary dependencies increase implementation risk;
- does not improve grounded answering discipline.

Verdict: attractive, but premature.

### Alternative C: Knowledge Spaces and Scoped Grounding

Add knowledge spaces, document membership, scoped retrieval diagnostics, import
state, and grounded answer behavior.

Pros:

- turns knowledge into an operator-managed product area;
- preserves local-first storage;
- reduces accidental cross-domain citations;
- gives future ingestion formats a clean destination;
- makes user-facing answers more trustworthy.

Cons:

- touches admin, runtime, retrieval, and app surfaces;
- requires a careful migration/default-space story;
- Stage 2 must keep the UI compact to avoid dashboard bloat.

Verdict: recommended.

## Recommended Approach

Ship Phase 12 as **Knowledge Space Management and Grounded Answering**:

- add a first-class `KnowledgeSpace` model and local file persistence;
- migrate existing documents into a default space;
- make upload, list, inspect, reindex, and diagnostics space-aware;
- make runtime retrieval respect selected space scope;
- show selected space and grounding state in `/app`;
- improve `/admin` information architecture around spaces, documents, and
  diagnostics;
- keep retrieval quality logic from Phase 11 intact.

## Proposed Architecture

```text
/admin spaces
  | create/list/update/disable/archive
  v
internal/admin KnowledgeService
  | spaces + document membership + import/index state
  v
local file knowledge store
  | default-space migration + documents + chunks + index metadata
  v
internal/knowledge Pipeline
  | scoped retrieval filters
  v
internal/runtime
  | selected space + grounding policy
  v
internal/agents PersonaAgent
  | source-bounded prompt context
  v
/app
  | compact scope + citations + no-source state
```

## Data Model Draft

### KnowledgeSpace

Required fields:

- `id`
- `tenant_id`
- `name`
- `description`
- `status`
- `default_retrieval_mode`
- `tags`
- `created_at`
- `updated_at`

### KnowledgeDocument Additions

Required or migrated fields:

- `space_ids` or `space_id` depending on Stage 2 membership decision;
- `import_status`;
- `index_status`;
- `last_indexed_at`;
- `last_error_code`;

Stage 2 should decide whether documents can belong to multiple spaces. The
recommended first implementation is one primary `space_id` to reduce retrieval
ambiguity.

### ScopedRetrievalRequest

Required fields:

- `tenant_id`
- `space_id`
- `query`
- `mode`
- `top_k`
- `filters`

### Grounding Metadata Additions

Required fields:

- `knowledge_space_id`
- `knowledge_space_name`
- `knowledge_used`
- `knowledge_citations`
- `retrieval_mode`
- `no_source_reason`

## UX Requirements

### `/admin`

- Space list is the primary navigation for knowledge.
- Selecting a space filters documents and diagnostics.
- Empty state tells the operator how to add the first document.
- Disabled or archived spaces are visibly distinct and excluded from default
  chat retrieval.
- Reindex actions show whether they target one document or a whole space.
- Diagnostic results explain excluded documents without leaking unsafe details.

### `/app`

- Current knowledge space is visible near the conversation controls.
- A user can switch space or use the default space.
- Assistant turns show source state compactly.
- If selected space has no usable knowledge, the assistant should say so in
  metadata/UI rather than silently citing another space.

## Acceptance Criteria

- Existing Phase 10/11 knowledge documents are assigned to a default space on
  load or migration.
- Admin API can create, list, update, disable, and inspect knowledge spaces.
- Upload can target a space; listing documents can filter by space.
- Retrieval diagnostics accept a space ID and only rank documents in that space.
- Runtime chat retrieval respects the selected active space.
- Disabled spaces and disabled documents are excluded from chat retrieval.
- `/app` shows selected knowledge space and citation/no-source state.
- `/admin` makes space, document status, and index state visible.
- Reindex supports at least one document; Stage 2 should decide whether
  space-wide reindex is included in the first implementation.
- CI remains deterministic and uses local/fake retrieval and LLM clients only.
- No SQLite, external vector DB, or mandatory embedding provider is introduced.

## Test Matrix Seed

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Space store | Load old documents with no space | Documents appear under default space |
| Space lifecycle | Create/update/disable space | Persisted state reloads correctly |
| Document upload | Upload into selected space | Document lists only under that space |
| Retrieval scope | Same term exists in two spaces | Selected space result wins; other space excluded |
| Disabled space | Chat asks with disabled space selected | No retrieval; safe no-source reason |
| Disabled document | Document disabled inside active space | Retrieval excludes it |
| Diagnostics | Space-scoped query | Response includes space and included/excluded counts |
| Index state | Vector failed, lexical ready | Diagnostics remain usable with clear vector state |
| App metadata | Grounded answer in selected space | Turn includes space name and citations |
| App metadata | No support in selected space | No fake citation; no-source state is visible |
| Admin UI | Empty space | Shows clear empty state and upload action |
| Migration | Existing knowledge JSON | Backward-compatible load without data loss |

## Open Questions for Stage 2

1. Should Phase 12 support multiple spaces per document, or one primary space
   only?
2. Should deleting a space physically delete documents, archive the space, or
   move documents to the default space?
3. Should `/app` expose a user-controlled space selector immediately, or use the
   tenant default space until the admin UI is stronger?
4. Should space-wide reindex ship in Phase 12, or remain document-level to keep
   the first implementation smaller?
5. Should no-source behavior be answer-level only, or should the assistant text
   explicitly say the selected space lacks support?

## Recommended Assignment Before Stage 2

Decide the Phase 12 membership model:

> Start with one primary `space_id` per document and a guaranteed `default` space.

This is the cleanest local-first wedge. It prevents cross-space citation
confusion now, while leaving many-to-many memberships as a later enhancement
once the product proves it needs them.
