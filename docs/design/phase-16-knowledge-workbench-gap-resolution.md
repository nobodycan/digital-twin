# Phase 16 Knowledge Workbench and Gap Resolution Design

Date: 2026-07-04

Status: Spec approved by the user; Stage 2 plan review in progress

Source spec: [Phase 16 Knowledge Workbench and Gap Resolution Spec](../specs/phase-16-knowledge-workbench-gap-resolution.md)

## Office-Hours Summary

Phase 16 should not be a generic admin redesign and should not jump straight to
new ingestion formats. The strongest next product move is to turn Phase 15's
unsupported-answer signal into an operator workflow.

The product should prove one simple loop:

1. the assistant cannot support a question;
2. the system captures a gap;
3. the operator adds or links knowledge;
4. retrieval diagnostics prove the knowledge is now findable;
5. the gap is resolved with evidence.

That loop gives the digital human a way to improve over time without pretending
to have autonomous knowledge management.

## Six Forcing Questions

### 1. Who desperately needs this?

The first user is the operator building and maintaining their own professional
digital human. They are not asking for enterprise KM yet. They need to see why
the assistant failed and patch the local knowledge base quickly.

### 2. What is the status quo?

Today the operator can upload a document, run diagnostics, and see a gap queue.
But these are separate surfaces. A gap does not guide the next action.

### 3. What is the narrowest painful wedge?

An unsupported question should become a work item that can be fixed with a small
knowledge note and verified by diagnostics.

### 4. What should we avoid building too early?

Avoid PDF/DOCX ingestion, web crawling, collaborative assignment, semantic gap
dedupe, auto-generated knowledge notes, and full admin redesign.

### 5. What can we observe after shipping?

We can observe whether gaps move from open to investigating to resolved, whether
notes created from gaps show up in diagnostics, and whether future turns ground
against those notes.

### 6. How does this fit the future?

Future ingestion formats, workflow assignment, and automated suggestions all
need a destination. The workbench is that destination.

## Recommended Product Shape

Build a **Knowledge Workbench** inside the existing `/admin` knowledge section.

The workbench has four connected surfaces:

- space/document context;
- gap queue and gap actions;
- note creation;
- retrieval diagnostics.

It should feel like a local operations tool, not a marketing dashboard.

## Architecture

```text
internal/server
  GET /admin/knowledge/gaps
  POST /admin/knowledge/gaps/update
  POST /admin/knowledge/notes/create
  POST /admin/knowledge/retrieval-diagnostics

internal/admin
  KnowledgeGapService
  KnowledgeService
  FileKnowledgeGapStore
  FileKnowledgeStore

web/admin.js
  render gap rows
  create note from gap
  run diagnostics from gap
  resolve with document ID
```

The implementation should reuse existing services:

- notes are normal `KnowledgeDocument` records;
- note chunking uses the existing upload/index path;
- diagnostics use the existing retrieval diagnostics endpoint;
- gap status updates use the existing gap update endpoint, extended only as
  needed.

## Component Design

### `internal/admin`

Extend `KnowledgeGapStatus` with:

- `KnowledgeGapInvestigating = "investigating"`

Keep existing statuses:

- `open`
- `ignored`
- `resolved`

Recommended additions to `KnowledgeGap`:

- `ResolutionNote string`

Optional, only if Stage 2 chooses to persist diagnostic attempts:

- `LastTestedAt time.Time`

Avoid a separate knowledge-note model. Instead, notes should be created through
`KnowledgeService.Upload` with metadata:

- `source_type=workbench_note`
- `source_gap_id=<gap-id>`
- `created_from=knowledge_workbench`

### `internal/server`

Recommended server additions:

- accept `investigating` in `/admin/knowledge/gaps/update`;
- allow `resolved_by_document_id` and optional `resolution_note`;
- add a note-create handler if existing upload JSON is too awkward for the UI.

Candidate route:

```text
POST /admin/knowledge/notes/create
```

Request:

```json
{
  "space_id": "default",
  "title": "Deployment smoke command",
  "body": "Use scripts/smoke-conversation.ps1 ...",
  "source_gap_id": "gap-123"
}
```

Response: normal knowledge document JSON.

If Stage 2 finds the existing upload route already supports this cleanly, prefer
reusing it and only adding metadata support.

### `web/admin`

The UI should stay small but more purposeful.

Gap row actions:

- `Investigate`
- `Run diagnostics`
- `Create note`
- `Resolve`
- `Ignore`

Recommended first layout:

```text
Knowledge space controls
Knowledge health summary

Documents                  Gap workbench
---------                  -------------
document list              selected/open gaps
document detail            gap actions

Diagnostics / note editor  Diagnostics results
```

No card-inside-card nesting. Keep repeated items as simple rows.

### `web/app`

No major app change is required. Optionally, if safe metadata exists, add a small
status such as:

```text
Knowledge gap captured
```

Do not add admin editing controls to `/app`.

## Data Flow

### Create Note From Gap

1. Operator sees an open gap in `/admin`.
2. Operator clicks `Create note`.
3. UI pre-fills note title/body from the gap question where helpful.
4. Operator writes source text.
5. Server creates a normal knowledge document in the gap's space.
6. Document chunks/indexes through existing knowledge pipeline.
7. Document metadata records `source_gap_id`.

### Verify Gap

1. Operator clicks `Run diagnostics` on a gap.
2. UI posts the gap question and space ID to retrieval diagnostics.
3. Diagnostics render ranked chunks.
4. If the new note is ranked, operator can resolve the gap with that document.

### Resolve Gap

1. Operator selects or confirms a resolving document.
2. UI posts status `resolved` plus `resolved_by_document_id`.
3. Gap persists as resolved and remains inspectable.

## Scope Guardrails

- Do not add a database.
- Do not add a new editor framework.
- Do not introduce background workers.
- Do not create automatic LLM-written knowledge as the default.
- Do not make `/app` an admin surface.

## Stage 2 Decisions

Stage 2 autoplan should lock these:

1. whether notes are created via a new route or existing upload route;
2. whether note editing is included or deferred;
3. exact persisted gap fields;
4. whether diagnostics writes back `last_tested_at`;
5. whether gap detail is inline or a right-side panel;
6. exact test matrix and TDD slice order.

## Risk Register

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Workbench scope balloons | High | MVP is one gap -> one note -> one diagnostic -> resolve |
| Notes duplicate document model | Medium | Store notes as normal knowledge documents with metadata |
| Resolution becomes fake confidence | High | Link resolved gaps to documents and show diagnostics before resolve |
| UI becomes cluttered | Medium | Keep gap actions concise and diagnostics in admin only |
| Existing gap records break | High | Add backward-compatible optional fields only |
| Disabled space accepts notes | Medium | Reuse existing writable-space validation |

## Success Signal

Phase 16 succeeds when a user can:

1. ask an unsupported question in `/app`;
2. see the gap in `/admin`;
3. move it to investigating;
4. create a small knowledge note in the same space;
5. run diagnostics from the gap;
6. see the note ranked;
7. mark the gap resolved with the note as evidence;
8. ask again and get a grounded or partially supported answer from the updated
   knowledge base.

## Assignment For Stage 2

Run `$gstack-autoplan` against this spec and design. The plan should produce a
TDD matrix for:

- gap status/model compatibility;
- note creation and metadata;
- diagnostics from gap context;
- server routes;
- admin UI workbench actions;
- regression coverage for Phase 15 answer-state behavior.
