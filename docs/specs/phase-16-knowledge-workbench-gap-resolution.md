# Phase 16 Knowledge Workbench and Gap Resolution Spec

Date: 2026-07-04

Status: Approved by the user; Stage 2 plan review in progress

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 12 Knowledge Space Management and Grounded Answering Spec](./phase-12-knowledge-space-management-grounded-answering.md)
- [Phase 14 Knowledge Operations Console Spec](./phase-14-knowledge-operations-console.md)
- [Phase 15 Knowledge-Grounded Answer Loop Spec](./phase-15-knowledge-grounded-answer-loop.md)
- Current implementation: local knowledge spaces, document lifecycle, retrieval
  diagnostics, document health summaries, answer-state metadata, and local
  knowledge-gap capture.

## Context

The project now has a credible local knowledge loop:

- `/admin` can manage knowledge spaces and uploaded documents;
- retrieval diagnostics can explain why chunks did or did not rank;
- `/app` can show whether an answer was grounded, partially supported,
  unsupported, provider fallback, guard fallback, or local mode;
- unsupported and partially supported turns can create local knowledge gaps.

That is enough to detect a missing-knowledge problem, but not enough to operate
one. Today, when the digital human exposes a gap, the operator still has to
manually decide what to write, where to put it, how to reindex, and how to prove
the next answer improved.

The next product move should turn gap capture into a knowledge workbench:

> Answer fails -> gap appears -> operator drafts or links knowledge -> retrieval
> is retested -> gap is resolved -> next answer can be grounded.

## Office-Hours Premise Challenge

The obvious framing is "make the frontend prettier." That is directionally
useful, but incomplete.

The current UI can be cleaner, but the deeper value is operational: a
professional digital human should become easier to improve every time it is
wrong or unsupported. Pure visual polish makes the product nicer to look at.
A knowledge workbench makes it learnable and maintainable.

The second tempting framing is "add more ingestion formats." That is also
premature. PDFs, web import, cloud drives, and GitHub sync will eventually matter,
but they add surface area before the system proves that one missing fact can move
from gap to verified knowledge.

Phase 16 should therefore focus on the narrowest closed loop:

- use existing text/Markdown local storage;
- keep existing spaces and diagnostics;
- add operator workflow around gaps;
- add lightweight knowledge-note creation;
- make validation explicit before a gap is marked resolved.

## Product Thesis

A professional digital human needs an operator-facing knowledge workbench, not
just an upload list.

The operator should be able to answer these questions quickly:

- What questions did the assistant fail to support?
- Which knowledge space did each failure belong to?
- Is this gap new, being investigated, ignored, or resolved?
- Can I add a small knowledge note without preparing a full document upload?
- After adding knowledge, does retrieval now find supporting chunks?
- Which document or note resolved the gap?

The desired feeling is a calm operations console: not a dashboard for vanity
metrics, but a place where missing knowledge becomes concrete work.

## Goal

Deliver a local-first Knowledge Workbench and Gap Resolution Loop:

1. expand knowledge gap statuses from passive queue to actionable workflow;
2. allow operators to create small text/Markdown knowledge notes inside a space;
3. link a gap to the document or note intended to resolve it;
4. run retrieval diagnostics from the gap context;
5. only mark a gap resolved after an operator action, while preserving manual
   override for early local use;
6. improve `/admin` layout enough that spaces, documents, diagnostics, and gaps
   feel like one workbench;
7. keep `/app` focused on answer reading and Presence summary, not knowledge
   editing.

## In Scope

### Gap Workflow

- Extend gap status semantics to:
  - `open`;
  - `investigating`;
  - `resolved`;
  - `ignored`.
- Preserve compatibility with existing `open`, `resolved`, and `ignored`
  persisted records.
- Add optional gap fields if needed:
  - `answer_state`;
  - `conversation_id`;
  - `turn_id`;
  - `last_tested_at`;
  - `resolution_note`;
  - `resolved_by_document_id`;
  - `candidate_document_id`.
- Keep all fields local file-backed.
- Do not store hidden prompts, provider payloads, API keys, or raw provider
  errors.

### Knowledge Notes

- Add a lightweight note creation path in `/admin`.
- A note is a normal knowledge document with local text content and metadata
  indicating it was created from the workbench.
- Required inputs:
  - space ID;
  - title;
  - body text;
  - optional source gap ID.
- Notes should chunk and index through the same existing knowledge pipeline.
- Notes should be editable only if the existing document model can support it
  safely in Stage 2; otherwise Phase 16 should start with create-only notes and
  defer editing.

### Gap-Centered Diagnostics

- From a gap row/detail, allow the operator to run retrieval diagnostics using
  the gap question and selected gap space.
- Show:
  - retrieval mode;
  - no-source reason;
  - top ranked chunks;
  - document ID/title;
  - score breakdown already available from diagnostics.
- Keep this in `/admin`, not `/app`.

### Resolution Flow

- Support linking a gap to a resolving document or note.
- Marking as `resolved` should retain `resolved_by_document_id` when provided.
- If retrieval diagnostics still finds no ranked chunks, the UI should warn the
  operator before resolving, but the first implementation can still allow a
  manual override.
- Repeated unsupported turns for an already resolved gap should create a new gap
  only if the no-source reason or space changes; Stage 2 should decide exact
  reopen behavior.

### Admin UX

- Improve the knowledge section into a workbench:
  - space controls remain visible;
  - document list stays scannable;
  - gap queue gets stronger status/action affordances;
  - diagnostics can be run from both manual query and gap context;
  - note creation is close to the gap workflow.
- Avoid a full visual redesign of the entire admin console in this phase.
- Avoid a new frontend framework.

### App UX

- Keep `/app` answer-state display from Phase 15.
- Optionally add a subtle indication that unsupported or partially supported
  turns were sent to the knowledge workbench.
- Do not add knowledge editing controls to `/app`.

## Out of Scope

- SQLite, Postgres, hosted vector databases, or cloud storage.
- PDF, DOCX, web crawling, browser import, cloud drive sync, GitHub sync, or
  background ingestion workers.
- Collaborative editing or multi-operator assignment.
- Semantic duplicate detection for gaps.
- LLM-generated knowledge-note drafting in the first implementation.
- Automatic resolution without operator confirmation.
- Full admin information architecture redesign.
- RBAC, OAuth, billing, or production tenant admin.

## Alternatives Considered

### Alternative A: Visual Polish First

Focus Phase 16 on making `/app` and `/admin` look more refined.

Pros:

- immediately improves perceived quality;
- low backend risk;
- useful after many functional phases.

Cons:

- does not close the missing-knowledge loop;
- leaves gaps as passive records;
- makes the product nicer without making it more self-improving.

Verdict: worth doing, but should be attached to the workbench flow rather than
becoming the whole phase.

### Alternative B: More Ingestion Formats

Add PDF, DOCX, or web import so the knowledge base can absorb richer sources.

Pros:

- obviously useful for real users;
- expands the kind of knowledge the digital human can use;
- makes the system feel more practical.

Cons:

- increases parsing/security/test surface area;
- does not prove the gap-resolution loop;
- risks producing more content without better operations.

Verdict: defer until the local text/Markdown workbench is effective.

### Alternative C: Knowledge Workbench and Gap Resolution

Build the operator loop around existing gaps, spaces, notes, diagnostics, and
answer states.

Pros:

- directly compounds Phase 14 and Phase 15;
- creates a real product flywheel;
- stays local-first and deterministic;
- gives future ingestion formats a useful destination.

Cons:

- touches admin service, server handlers, web admin UI, tests, and docs;
- requires careful status semantics;
- can become too broad if editing, assignment, and automation are added at once.

Verdict: recommended.

## Recommended Approach

Ship Phase 16 as **Knowledge Workbench and Gap Resolution Loop**:

- make knowledge gaps actionable;
- add local knowledge-note creation from the workbench;
- connect gap questions to retrieval diagnostics;
- link resolved gaps to documents/notes;
- refine `/admin` knowledge layout enough to support the workflow;
- keep `/app` focused and compact.

## Narrowest Valuable Wedge

The smallest valuable wedge is:

1. select an open gap in `/admin`;
2. create a note in the same knowledge space from that gap;
3. run diagnostics using the gap question;
4. see the new note among ranked chunks;
5. mark the gap resolved with the note as evidence.

If Phase 16 only delivers that loop well, it will be a stronger product step than
adding many loosely connected admin controls.

## Proposed Architecture

```text
/app unsupported turn
  -> server gap capture
  -> FileKnowledgeGapStore
  -> /admin knowledge workbench
      -> gap detail/actions
      -> create knowledge note
      -> existing KnowledgeService upload/index path
      -> retrieval diagnostics by gap question
      -> update gap status and resolved document
  -> future /app turn can ground from new note
```

## Data Model Draft

### Gap Status

Existing:

- `open`
- `ignored`
- `resolved`

Add:

- `investigating`

Recommended compatibility rule:

- unknown statuses should be rejected on update;
- old stored records remain valid;
- missing optional fields are treated as empty.

### Knowledge Note Metadata

Candidate metadata fields on `KnowledgeDocument.Metadata`:

- `source_type=workbench_note`
- `source_gap_id=<gap-id>`
- `created_from=knowledge_workbench`

This avoids a second storage model for notes.

### Gap Resolution Metadata

Optional fields on `KnowledgeGap`:

- `AnswerState`
- `ConversationID`
- `TurnID`
- `CandidateDocumentID`
- `LastTestedAt`
- `ResolutionNote`

Stage 2 should decide the minimum set to implement. The recommended MVP is:

- add `investigating`;
- keep `ResolvedByDocumentID`;
- add `ResolutionNote`;
- add `LastTestedAt` only if diagnostics writes back to the gap.

## UX Requirements

### `/admin`

- Gap rows show question, status, no-source reason, and space.
- Gap actions include:
  - investigate;
  - ignore;
  - create note;
  - run diagnostics;
  - resolve.
- Note creation should not require leaving the knowledge section.
- Diagnostics result should reuse the current structured debug rendering.
- Resolution should show which document/note was used.
- Empty states should be concise and operational.

### `/app`

- Unsupported/partial state remains visible from Phase 15.
- The UI may show that a gap was captured only if that metadata is already
  safely available.
- No admin editing controls in chat.

## Acceptance Criteria

1. Existing gap records still load and update.
2. `investigating` is a valid gap status and persists in file and in-memory
   stores.
3. `/admin` can create a text/Markdown knowledge note in the selected space.
4. A note created from a gap stores metadata linking it to that gap.
5. Notes use the existing knowledge chunking/indexing path.
6. `/admin` can run diagnostics using a gap question and space.
7. A resolved gap can record `resolved_by_document_id`.
8. The UI warns or clearly shows when diagnostics still finds no source before
   resolution.
9. `/app` behavior from Phase 15 does not regress.
10. All tests remain local and deterministic; no real DeepSeek call is required
    in CI.

## Test Matrix Seed

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Gap model | Existing `open` gap loads | Record remains valid |
| Gap model | Update status to `investigating` | Status persists |
| Gap model | Invalid status | Stable validation error |
| Gap store | Resolved gap with document ID | `resolved_by_document_id` persists |
| Knowledge notes | Create note in active space | Document is ready/chunked |
| Knowledge notes | Create note from gap | Metadata contains source gap ID |
| Knowledge notes | Create note in disabled space | Request fails with existing space error |
| Diagnostics | Run by gap question | Uses gap space and question |
| Diagnostics | New note matches gap question | Ranked chunk includes note document |
| Server | Gap update route supports investigating | JSON response contains updated gap |
| Server | Note create route validates title/body | Empty inputs fail safely |
| Web admin | Gap row actions render | Investigate, ignore, diagnostics, resolve controls exist |
| Web admin | Create note from gap | Sends space ID and gap ID |
| Web admin | Diagnostics from gap | Uses existing debug output |
| Regression | `/app` selected space payload | Still includes knowledge-space metadata |

## Open Questions For Stage 2

1. Should note editing ship in Phase 16, or should the MVP be create-only notes?
2. Should diagnostics write `last_tested_at` to the gap, or remain read-only?
3. Should resolving a gap require a `resolved_by_document_id`, or allow manual
   resolution without a document?
4. Should repeated unsupported turns reopen resolved gaps, create new gaps, or
   leave resolved gaps untouched?
5. Should the workbench introduce a selected gap detail panel, or keep actions
   inline for the first implementation?

## Recommended Assignment Before Stage 2

Approve or revise the Phase 16 scope:

> Build the Knowledge Workbench and Gap Resolution Loop: actionable gap statuses,
> local note creation from gaps, gap-centered retrieval diagnostics, and
> resolution linked to a knowledge document or note. Defer new ingestion formats,
> collaborative workflows, semantic duplicate detection, and automatic
> resolution.
