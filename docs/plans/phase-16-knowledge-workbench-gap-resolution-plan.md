# Phase 16 Knowledge Workbench and Gap Resolution Plan

Date: 2026-07-04

Status: Waiting for implementation approval

Source spec: [Phase 16 Knowledge Workbench and Gap Resolution Spec](../specs/phase-16-knowledge-workbench-gap-resolution.md)

Source design: [Phase 16 Knowledge Workbench and Gap Resolution Design](../design/phase-16-knowledge-workbench-gap-resolution.md)

Mode: SDD Stage 2 / gstack autoplan

## Goal

Close the first operator-facing knowledge improvement loop:

```text
unsupported or partially supported answer
  -> local knowledge gap
  -> operator investigates
  -> operator creates a small knowledge note
  -> diagnostics proves the note is retrievable
  -> gap is resolved with evidence
  -> future answers can ground on the new note
```

The phase should make the digital human more maintainable without introducing a
database, hosted vector store, new frontend framework, or new ingestion format.

## Scope

### In

- Add `investigating` as a valid knowledge gap status.
- Keep existing `open`, `ignored`, and `resolved` gap records compatible.
- Add optional `resolution_note` to resolved gaps.
- Preserve `resolved_by_document_id` when a resolving document or note is known.
- Add local text/Markdown knowledge-note creation in `/admin`.
- Store notes as normal `KnowledgeDocument` records through the existing
  knowledge upload/chunk/index path.
- Mark workbench-created notes with metadata:
  - `source_type=workbench_note`
  - `source_gap_id=<gap-id>` when created from a gap
  - `created_from=knowledge_workbench`
- Let `/admin` run retrieval diagnostics from a gap question and space.
- Improve the existing knowledge admin section enough to connect gaps, notes,
  documents, and diagnostics.
- Keep `/app` Phase 15 answer-state behavior stable.

### Out

- SQLite/Postgres or hosted vector databases.
- PDF, DOCX, web, cloud-drive, GitHub, or background ingestion.
- Note editing after creation.
- Semantic duplicate detection for gaps.
- LLM-generated note drafting.
- Automatic gap resolution.
- Assignment, collaboration, RBAC, OAuth, or production tenant admin.
- Full `/admin` redesign or new frontend tooling.

## What Already Exists

- `internal/admin.KnowledgeGapService` can create, list, dedupe, and update
  knowledge gaps.
- `FileKnowledgeGapStore` and `InMemoryKnowledgeGapStore` persist gap records.
- `internal/admin.KnowledgeService.Upload` already validates spaces and sends
  local content through the document chunk/index pipeline.
- `POST /admin/knowledge/retrieval-diagnostics` already returns ranked chunks
  and no-source reasons.
- `web/admin.js` already renders gap rows, gap status actions, manual
  diagnostics, space controls, document lists, and document operations.
- `web/app.js` already renders Phase 15 answer-state and source metadata.

## Review Results

### CEO Review

Score: 9/10.

This phase is the right next wedge because it converts knowledge failure into an
operator workflow. Pure visual polish would make the product look better but
would not make the digital human easier to improve. More ingestion formats are
valuable later, but they need a workbench destination first.

Decision: ship the narrow closed loop before broadening ingestion.

### Design Review

Score: 8/10.

The UI has enough structure to extend, but the workbench must avoid becoming a
dense command wall. Keep actions inline for the first release, reuse existing
diagnostics rendering, and keep `/app` focused on reading the answer rather than
editing knowledge.

Decision: use inline gap actions and a compact note form in the existing
knowledge section. Defer a selected detail panel.

### Engineering Review

Score: 8.5/10.

The safest implementation is to reuse existing service contracts. A new note
storage model would duplicate document lifecycle code and create migration work.
The main risks are status compatibility, route validation, and frontend state
drift.

Decision: notes are normal knowledge documents. Add one note-specific server
route as a thin adapter over `KnowledgeService.Upload`.

### DX Review

Score: 8/10.

Local development remains simple if this phase stays deterministic and does not
need real DeepSeek calls. Developer clarity depends on explicit routes, stable
JSON fields, and focused tests that explain the gap-to-note-to-resolution loop.

Decision: tests use in-memory/file stores and fake knowledge inputs only. No
provider calls in CI.

## Locked Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Note editing | Defer editing | Create-only notes close the loop with less data-lifecycle risk. |
| Note storage | Store notes as `KnowledgeDocument` | Reuses validation, chunking, indexing, and document listing. |
| Note route | Add `POST /admin/knowledge/notes/create` | Keeps UI intent clear and avoids overloading upload form semantics. |
| Diagnostics persistence | Read-only diagnostics in MVP | Avoid mutating gaps just because an operator probed retrieval. |
| Gap fields | Add `resolution_note`; keep `resolved_by_document_id` | Enough evidence without expanding into workflow assignment. |
| Resolve without doc ID | Allow manual override | Preserves local flexibility; UI must show when evidence is weak. |
| Repeated unsupported turns | Do not auto-reopen resolved gaps | Existing open-gap dedupe stays simple; future semantic reopen can be explicit. |
| Workbench layout | Inline gap actions | Lowest-risk improvement that fits existing admin shell. |
| `/app` changes | Regression only | Workbench belongs in `/admin`; chat should stay clean. |

## Architecture

```text
web/app.js
  unsupported/partial turn metadata
    -> internal/server.captureKnowledgeGap
      -> admin.KnowledgeGapService
        -> FileKnowledgeGapStore

web/admin.js
  gap row action: investigate / ignore / resolve
    -> POST /admin/knowledge/gaps/update

web/admin.js
  gap row action: create note
    -> POST /admin/knowledge/notes/create
      -> admin.KnowledgeService.Upload
        -> existing chunk/index pipeline
        -> KnowledgeDocument.Metadata

web/admin.js
  gap row action: run diagnostics
    -> POST /admin/knowledge/retrieval-diagnostics
      -> existing diagnostics result renderer
```

## Data Contract

### Gap Status

Allowed values after this phase:

| Value | Meaning |
| --- | --- |
| `open` | Captured and not yet handled. |
| `investigating` | Operator has acknowledged and is working the gap. |
| `resolved` | Operator marked the gap resolved, optionally with evidence. |
| `ignored` | Operator intentionally dismissed the gap. |

### Gap Update Request

Existing fields remain valid:

```json
{
  "gap_id": "gap-123",
  "status": "resolved",
  "resolved_by_document_id": "doc-456"
}
```

Add optional:

```json
{
  "resolution_note": "Covered by the new deployment checklist note."
}
```

### Note Create Request

```json
{
  "space_id": "default",
  "title": "Deployment smoke command",
  "body": "Use scripts/smoke-conversation.ps1 after starting DeepSeek mode.",
  "source_gap_id": "gap-123"
}
```

Response: normal `KnowledgeDocument` JSON.

### Note Metadata

```json
{
  "source_type": "workbench_note",
  "source_gap_id": "gap-123",
  "created_from": "knowledge_workbench"
}
```

## TDD Implementation Slices

### P16-01 Gap Status And Resolution Metadata

RED:

- Add admin tests proving `investigating` persists in memory and file stores.
- Add admin tests proving invalid statuses still fail.
- Add admin tests proving `resolution_note` persists and is cleared or ignored
  appropriately when status is not `resolved`.

GREEN:

- Extend `KnowledgeGapStatus`.
- Add `ResolutionNote string` to `KnowledgeGap`.
- Extend `KnowledgeGapService.UpdateStatus` with resolution note support.

REFACTOR:

- Keep validation centralized and preserve old call sites with minimal churn.

Verification:

```powershell
go test ./internal/admin -run KnowledgeGap
```

### P16-02 Server Gap Update Contract

RED:

- Add server tests for `status=investigating`.
- Add server tests for resolving with `resolved_by_document_id` and
  `resolution_note`.
- Add server tests for invalid status and missing service behavior.

GREEN:

- Extend request decoding for `resolution_note`.
- Route existing update handler through the updated service method.

REFACTOR:

- Keep JSON error responses consistent with existing admin endpoints.

Verification:

```powershell
go test ./internal/server -run "KnowledgeGap"
```

### P16-03 Knowledge Note Creation Service/API

RED:

- Add admin/service or server tests for creating a note in a writable space.
- Add tests that empty title/body fail safely.
- Add tests that disabled space validation follows existing document upload
  behavior.
- Add tests that gap-created notes carry workbench metadata.

GREEN:

- Add `POST /admin/knowledge/notes/create`.
- Implement a thin request adapter that calls `KnowledgeService.Upload` with
  text content and metadata.
- Return the created document.

REFACTOR:

- Extract only small helpers if upload request shaping would otherwise duplicate
  logic.

Verification:

```powershell
go test ./internal/admin -run "Knowledge|Upload|Note"
go test ./internal/server -run "Knowledge.*Note|Knowledge.*Upload"
```

### P16-04 Gap-Centered Diagnostics

RED:

- Add server tests proving diagnostics can be called with a gap question/space
  payload and still uses the existing diagnostics contract.
- Add frontend static tests for gap diagnostics function names and route usage.

GREEN:

- Reuse `POST /admin/knowledge/retrieval-diagnostics`; no new backend route is
  needed unless a test exposes a contract gap.
- Add frontend helper to run diagnostics from a gap object.

REFACTOR:

- Keep manual diagnostics and gap diagnostics rendering through the same result
  path.

Verification:

```powershell
go test ./internal/server -run RetrievalDiagnostics
go test ./web -run Admin
```

### P16-05 Admin Workbench UI

RED:

- Extend `web/app_static_test.go` checks for:
  - `/admin/knowledge/notes/create`;
  - `knowledgeGapInvestigate`;
  - `createKnowledgeNoteFromGap`;
  - `runKnowledgeGapDiagnostics`;
  - `resolveKnowledgeGap`;
  - `resolution_note`;
  - `source_gap_id`;
  - `investigating`.

GREEN:

- Add inline gap actions:
  - Investigate;
  - Run diagnostics;
  - Create note;
  - Resolve;
  - Ignore.
- Add a compact note form near the gap workbench.
- Use selected gap space for note creation and diagnostics.
- Show a visible warning when resolving without diagnostics evidence or without
  a linked document.

REFACTOR:

- Keep DOM helpers small and avoid large one-off HTML string blocks where
  existing helper patterns already exist.

Verification:

```powershell
go test ./web
```

### P16-06 Regression, Docs, And Release Notes

RED:

- Add/extend tests that Phase 15 `/app` answer-state strings still exist.
- Add documentation search checks for Phase 16 route/status/metadata names.

GREEN:

- Update README/release notes only after implementation behavior exists.
- Keep docs honest: local-first, planning/implementation status, no new
  external provider requirement.

REFACTOR:

- Remove obsolete wording if any docs imply automatic resolution or new
  ingestion formats.

Verification:

```powershell
go test ./...
rg -n "Phase 16|Knowledge Workbench|investigating|notes/create|source_gap_id|resolution_note" .
```

## Test Matrix

| Area | Test | Expected |
| --- | --- | --- |
| Gap model | Existing `open` gap loads | Record remains valid |
| Gap model | Status update to `investigating` | Status persists in memory and file stores |
| Gap model | Invalid status | Stable validation error |
| Gap model | Resolve with document ID | `resolved_by_document_id` persists |
| Gap model | Resolve with note | `resolution_note` persists |
| Gap model | Non-resolved update with note | Note is not treated as resolution evidence |
| Server | Gap update to investigating | JSON response contains updated status |
| Server | Gap resolve with note/doc | Response contains both evidence fields |
| Server | Missing gap service | Existing unavailable-service error behavior remains |
| Knowledge notes | Create note in writable space | Document is ready/chunked/indexed |
| Knowledge notes | Empty title/body | Request fails with clear validation error |
| Knowledge notes | Disabled space | Request fails through existing writable-space guard |
| Knowledge notes | Create from gap | Metadata contains `source_gap_id` |
| Diagnostics | Run diagnostics from gap question | Uses gap question and gap space |
| Diagnostics | Note matches gap question | Ranked result includes note document |
| Web admin | Gap row actions render | Investigate, diagnostics, create note, resolve, ignore exist |
| Web admin | Create note from gap | Sends space ID, title/body, and source gap ID |
| Web admin | Resolve warning | UI shows weak-evidence warning before resolution |
| Web admin | Manual diagnostics unchanged | Existing manual diagnostics path still works |
| Regression | `/app` Phase 15 states | Grounded/partial/unsupported/fallback strings remain |
| Regression | Full test suite | `go test ./...` passes locally |

## Failure Modes And Mitigations

| Failure Mode | Severity | Mitigation |
| --- | --- | --- |
| Existing gap records stop loading | High | Add only optional JSON fields and test file-store reload. |
| Notes bypass indexing | High | Implement notes through `KnowledgeService.Upload`, not a new store. |
| Resolution becomes false confidence | High | Preserve evidence fields and show UI warning for weak evidence. |
| Admin UI becomes cluttered | Medium | Inline actions only; defer detail panel and full redesign. |
| Disabled spaces accept notes | Medium | Reuse existing writable-space validation. |
| Diagnostics mutates state unexpectedly | Medium | Keep diagnostics read-only in this phase. |
| `/app` regresses while `/admin` changes | Medium | Keep app changes out of scope and retain static tests. |

## Implementation Order

1. P16-01: gap status and metadata.
2. P16-02: server gap update contract.
3. P16-03: note creation API.
4. P16-04: gap-centered diagnostics helper.
5. P16-05: admin workbench UI.
6. P16-06: docs, release notes, and full regression.

Parallelizable after P16-02:

- P16-03 backend note creation and P16-05 frontend static scaffolding can proceed
  independently if tests define the JSON contract first.
- P16-06 documentation can draft after route/status names are final, but should
  not claim behavior until tests pass.

Blocking dependencies:

- P16-05 depends on P16-02 and P16-03 route contracts.
- P16-04 frontend diagnostics depends on existing diagnostics result shape.
- P16-06 depends on the final API names and accepted statuses.

## Verification Plan

Run the focused loop after each slice, then the full suite before completion:

```powershell
go test ./internal/admin -run KnowledgeGap
go test ./internal/server -run "KnowledgeGap|Knowledge.*Note|RetrievalDiagnostics"
go test ./web
go test ./...
rg -n "Phase 16|Knowledge Workbench|investigating|notes/create|source_gap_id|resolution_note" .
git status -sb
```

No command should require a real DeepSeek key or network provider call.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Build the gap-resolution workbench before broad ingestion | Auto-decided | Narrowest valuable wedge | It closes the product loop with existing local primitives. | PDF/DOCX/web ingestion in this phase |
| 2 | Design | Use inline gap actions for MVP | Auto-decided | Simpler operator flow | Existing admin UI can support the workflow without a new layout system. | Selected gap detail panel |
| 3 | Engineering | Store notes as normal knowledge documents | Auto-decided | Reuse proven path | Reuses validation, chunking, indexing, and document lifecycle. | Separate note store/model |
| 4 | Engineering | Add `POST /admin/knowledge/notes/create` | Auto-decided | Explicit API intent | Keeps workbench note semantics clear while still delegating to upload. | Overload the upload UI contract |
| 5 | Engineering | Keep diagnostics read-only | Auto-decided | Avoid surprising mutation | Running a test should not change workflow state in the MVP. | Persist `last_tested_at` now |
| 6 | Engineering | Allow manual resolve without document ID | Taste decision | Local-first flexibility | Operators may need to close noise early; UI will show weak evidence. | Require document evidence for every resolve |
| 7 | Engineering | Do not auto-reopen resolved gaps | Auto-decided | Conservative state machine | Reopen semantics need semantic dedupe and history, which are out of scope. | Automatic reopen |
| 8 | DX | Keep all tests local/deterministic | Auto-decided | Fast reliable CI | Provider calls would make the feature brittle and expensive to verify. | DeepSeek smoke in CI |

## Implementation Tasks

- [ ] P16-01: Add TDD coverage and implementation for `investigating` and
  `resolution_note`.
- [ ] P16-02: Extend server gap update request/response tests and handler.
- [ ] P16-03: Add note-create API via existing knowledge upload path.
- [ ] P16-04: Add gap-centered diagnostics frontend helper and focused tests.
- [ ] P16-05: Improve admin workbench UI with inline actions and warnings.
- [ ] P16-06: Run regression, update docs/release notes, and verify status.

## Stage 3 TDD Rules

- Start every slice with a failing test from the matrix.
- Do not add production code for behavior that is not represented by a test.
- Keep commits small enough that each can be reviewed as one behavior change.
- Prefer existing admin/server/web patterns over new abstractions.
- Keep external LLM/provider calls out of test paths.

## Completion Criteria

Phase 16 is complete when:

- an unsupported or partially supported turn can create a gap;
- the gap can move to `investigating`;
- `/admin` can create a knowledge note from that gap;
- the note is indexed as a normal knowledge document;
- diagnostics from the gap can find the note when content matches;
- the gap can be resolved with document/note evidence and optional note text;
- `/app` answer-state rendering from Phase 15 still passes regression tests;
- `go test ./...` passes.

