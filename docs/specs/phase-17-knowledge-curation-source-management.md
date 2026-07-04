# Phase 17 Knowledge Curation and Source Management Spec

Date: 2026-07-04

Status: Approved by the user; Stage 2 plan review in progress

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 14 Knowledge Operations Console Spec](./phase-14-knowledge-operations-console.md)
- [Phase 15 Knowledge-Grounded Answer Loop Spec](./phase-15-knowledge-grounded-answer-loop.md)
- [Phase 16 Knowledge Workbench and Gap Resolution Spec](./phase-16-knowledge-workbench-gap-resolution.md)
- Current implementation: local knowledge spaces, document lifecycle,
  retrieval diagnostics, answer-state metadata, knowledge-gap queue,
  workbench note creation, and evidence-aware gap resolution.

## Context

Phase 16 closed the first operational loop:

> unsupported answer -> captured gap -> workbench note -> diagnostics ->
> resolved gap.

That makes the digital human repairable. The next problem is that repair notes
can pile up as one-off patches. Without curation, a knowledge base becomes a
drawer of fragments: useful in the moment, hard to maintain later, and risky as
the digital human starts citing stale or duplicated source material.

Phase 17 should turn local knowledge from "uploaded documents plus gap notes"
into a small but real knowledge asset manager.

## Office-Hours Premise Challenge

The tempting next step is "add more ingestion formats." That is not wrong, but
it is early. Importing PDFs, websites, and cloud documents will multiply the
number of records before the system has a good way to maintain the records it
already owns.

The stronger product move is curation:

- edit and correct a knowledge note after it is created;
- inspect where a document came from and what it resolved;
- search and filter documents by source, status, and gap linkage;
- see source relationships before trusting an answer;
- keep all of this local-first and deterministic.

The second tempting step is "make an LLM auto-maintain the knowledge base."
That can become powerful later, but in this phase it should remain assistive at
most. A professional digital human must not silently rewrite its own source of
truth.

## Product Thesis

A professional digital human needs a curated knowledge base, not just a pile of
retrievable chunks.

The operator should be able to answer these questions quickly:

- Which documents are original uploads, which are workbench notes, and which
  notes came from unresolved gaps?
- Can I correct a note without deleting and recreating it?
- Which gaps did this document resolve?
- Which documents are active, disabled, stale-looking, or missing source
  metadata?
- Can I find all notes created from the workbench?
- Can I run diagnostics from a document or note to understand where it appears
  in retrieval?

## Goal

Deliver local-first Knowledge Curation and Source Management:

1. add an edit/update path for local knowledge notes and text documents;
2. preserve source metadata and gap linkage across edits;
3. add document filtering/search in `/admin`;
4. expose source relationships between documents and gaps;
5. add curation status signals for stale, disabled, workbench-created, and
   gap-linked documents;
6. keep retrieval deterministic after edits by reusing the existing reindex
   path;
7. avoid new storage engines, background workers, and external ingestion.

## In Scope

### Knowledge Document Editing

- Support editing title/body for local text or Markdown knowledge records where
  content is stored in the local document model.
- Reindex edited documents through the existing knowledge pipeline.
- Preserve immutable identity fields:
  - document ID;
  - tenant ID;
  - space ID;
  - creation timestamp;
  - source metadata unless explicitly edited.
- Update mutable metadata:
  - title;
  - content hash;
  - chunk list;
  - updated timestamp or equivalent local freshness signal.
- Reject edits for disabled or archived spaces using existing space validation.

### Source Metadata Management

- Surface source metadata in `/admin`:
  - `source_type`;
  - `created_from`;
  - `source_gap_id`;
  - optional operator note;
  - last updated signal if supported.
- Allow editing a small safe subset of metadata if Stage 2 finds the model can
  support it cleanly:
  - title;
  - source label;
  - operator note.
- Do not allow arbitrary metadata keys from the browser without validation.

### Gap and Document Relationships

- Show documents linked to a gap through `source_gap_id` or
  `resolved_by_document_id`.
- Show gaps linked to a selected document:
  - created-from gap;
  - resolved gaps;
  - open gaps that still mention or depend on the document, only if this can be
    derived deterministically from existing data.
- Keep relationships derived from local stores; no graph database.

### Search and Filtering

- Add local `/admin` filtering for knowledge documents:
  - text query against title and source label;
  - status: active, disabled;
  - source type: upload, workbench note, unknown;
  - gap-linked only;
  - selected knowledge space.
- Filtering can be server-side or client-side; Stage 2 should choose based on
  existing handler shape and testability.

### Admin UX

- Upgrade `/admin` knowledge area from workbench-only to curation-oriented:
  - searchable document list;
  - selected document detail;
  - edit action for editable text/Markdown records;
  - source relationship section;
  - gap linkage visible without hunting through raw metadata.
- Keep the interface calm and dense. This is an operator console, not a landing
  page.

### Retrieval Diagnostics

- Allow a selected document or note to seed diagnostics:
  - use title/body snippets as query suggestions;
  - keep existing manual diagnostics input.
- Do not make diagnostics mutate gap or document state in this phase unless
  Stage 2 explicitly chooses that as a small, tested enhancement.

## Out of Scope

- SQLite, Postgres, hosted vector databases, or cloud storage.
- PDF, DOCX, browser capture, website crawling, GitHub sync, cloud drive sync,
  or scheduled imports.
- Collaborative review queues, assignments, approvals, or RBAC.
- LLM-authored automatic edits.
- Semantic deduplication or clustering.
- Full provenance graph or timeline visualization.
- Production multi-tenant admin hardening beyond existing local auth boundary.

## Alternatives Considered

### Alternative A: Add Rich Ingestion First

Add PDF, DOCX, web, or GitHub ingestion before curation.

Pros:

- unlocks more real-world source material;
- makes the knowledge feature feel bigger;
- creates a natural reason to revisit chunking and source metadata.

Cons:

- increases parser, security, and QA surface area;
- creates more documents before operators can maintain them well;
- does not solve stale notes, broken source labels, or gap traceability.

Verdict: defer. Ingestion becomes more valuable after curation exists.

### Alternative B: LLM-Assisted Knowledge Maintenance

Use the provider model to draft edits, merge notes, or suggest missing sources.

Pros:

- can feel magical;
- matches the digital-human theme;
- may reduce operator writing effort.

Cons:

- risks silent source-of-truth drift;
- requires trust boundaries, diff review, and audit semantics;
- CI cannot depend on provider behavior.

Verdict: defer autonomous writing. Phase 17 can leave room for future
suggestions, but editing should remain operator-controlled.

### Alternative C: Local Curation and Source Management

Make existing knowledge records editable, searchable, and traceable.

Pros:

- compounds Phase 16 directly;
- keeps storage local and deterministic;
- improves day-to-day operator workflow;
- gives future ingestion formats a maintainable destination.

Cons:

- touches admin service, server routes, store persistence, and admin UI;
- needs careful validation so metadata does not become arbitrary browser input;
- edit/reindex behavior must be covered by regression tests.

Verdict: recommended.

## Recommended Approach

Ship Phase 17 as **Knowledge Curation and Source Management**:

- add document edit/update with deterministic reindex;
- expose curated source metadata;
- add document search/filter controls;
- show document-gap relationships;
- keep LLM assistance and rich ingestion out of this phase.

## Narrowest Valuable Wedge

The smallest useful loop is:

1. operator opens `/admin`;
2. filters to workbench notes in a selected space;
3. selects a note created from a gap;
4. edits the note body;
5. document is reindexed under the same ID;
6. document detail still shows source gap and resolved gap linkage;
7. diagnostics can find the edited text.

If this loop works well, the knowledge base becomes maintainable instead of
just append-only.

## Proposed Architecture

```text
/admin knowledge curation
  -> list/search/filter documents
  -> select document detail
  -> edit local text/Markdown document
  -> KnowledgeService update/reindex path
  -> FileKnowledgeStore persists updated document
  -> relationship view joins gaps by source_gap_id/resolved_by_document_id
  -> retrieval diagnostics verifies updated content
```

## Data Model Draft

### Editable Knowledge Document

Candidate mutable fields:

- `Title`
- `Content`
- `Metadata.SourceLabel` or a validated metadata field

Candidate derived fields:

- `ContentHash`
- `Chunks`
- `UpdatedAt`
- `IndexState`

Stage 2 should verify the exact current model names before locking this.

### Source Relationship Projection

Add a read projection rather than a new persisted graph:

```text
KnowledgeDocumentRelations
  document_id
  source_gap
  resolved_gaps[]
  metadata
```

This can be computed from:

- document metadata `source_gap_id`;
- gap field `resolved_by_document_id`;
- selected space ID.

## UX Requirements

### `/admin`

- Document list supports search and filters without losing selected space.
- Document detail shows source metadata in readable labels.
- Editable documents expose an edit action.
- Edit form clearly separates content from source metadata.
- Relationship section shows:
  - "Created from gap" when present;
  - "Resolved gaps" when present;
  - empty state when no relationship exists.
- Diagnostics can be run after edit with the same existing diagnostics result
  renderer.

### `/app`

- No required changes.
- `/app` should continue showing answer-state and citations from prior phases.
- Do not expose knowledge editing in the user-facing chat.

## Acceptance Criteria

1. Existing knowledge documents still list and retrieve correctly.
2. Editable text/Markdown records can update title/body and preserve document
   ID.
3. Edited records reindex deterministically and retrieval sees updated content.
4. Source metadata from Phase 16 workbench notes is preserved after edits.
5. Invalid or unsafe metadata updates are rejected.
6. Disabled or archived spaces reject edit attempts.
7. `/admin` can filter documents by source type and gap linkage.
8. `/admin` can search document title/source labels inside a selected space.
9. Document detail shows created-from and resolved-gap relationships when data
   exists.
10. Existing Phase 16 gap resolution flow does not regress.
11. CI remains local and deterministic; no real DeepSeek call is required.

## Test Matrix Seed

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Admin service | Update editable document body | Same document ID, new chunks |
| Admin service | Update title | List/detail show new title |
| Admin service | Preserve source metadata | `source_gap_id` remains after edit |
| Admin service | Reject disabled-space edit | Stable validation error |
| Store | Persist edited document | Reload sees updated content/chunks |
| Store | Existing documents without `updated_at` | Load remains backward compatible |
| Relationships | Document has `source_gap_id` | Detail shows created-from gap |
| Relationships | Gap has `resolved_by_document_id` | Detail shows resolved gap |
| Server | Edit route validates body/title | Empty unsafe inputs fail |
| Server | List route accepts filters | Returns scoped filtered documents |
| Web admin | Filter controls render | Search/status/source/gap filters exist |
| Web admin | Edit form renders | Title/body fields and save action exist |
| Web admin | Save edit | Sends validated payload and refreshes detail |
| Web admin | Relationship section | Shows gap linkage labels |
| Regression | Phase 16 note create | Still creates indexed workbench note |
| Regression | Phase 16 gap resolve | Still records resolving document |

## Open Questions For Stage 2

1. Should filtering be implemented server-side first, or client-side over the
   selected space document list?
2. Which document types are safe to edit in Phase 17: all local text records, or
   only `workbench_note` records?
3. Should `UpdatedAt` be introduced as a persisted field, or should reindex
   metadata be used as the freshness signal?
4. Should source labels be a first-class field or a validated metadata key?
5. Should document relationship detail be a new endpoint or folded into the
   existing detail endpoint?

## Recommended Assignment Before Stage 2

Approve or revise the Phase 17 scope:

> Build Knowledge Curation and Source Management: editable local knowledge
> notes/documents, deterministic reindex after edits, source metadata display,
> document search/filter, and document-gap relationship detail. Defer rich
> ingestion, automatic LLM rewriting, collaborative workflows, and graph-style
> provenance.
