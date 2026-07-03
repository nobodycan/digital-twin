# Phase 14 Knowledge Operations Console Spec

Date: 2026-07-02

Status: Approved by the user on 2026-07-02

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 10 Knowledge Base and Memory Control Spec](./phase-10-knowledge-base-memory-control.md)
- [Phase 11 Knowledge Retrieval Quality and RAG Evaluation Spec](./phase-11-knowledge-retrieval-quality-rag-evaluation.md)
- [Phase 12 Knowledge Space Management and Grounded Answering Spec](./phase-12-knowledge-space-management-grounded-answering.md)
- [Phase 13 Presence Summary Panel Spec](./phase-13-presence-summary-panel.md)
- Current implementation: local file-backed knowledge spaces, scoped retrieval, diagnostics, grounded answer metadata, `/admin` controls, and `/app` Presence summary signals.

## Context

The project now has the core ingredients of a professional digital human:

- it can talk through a real provider boundary;
- it can keep local conversation state;
- it can retrieve from scoped local knowledge spaces;
- it can show whether a reply was grounded, unsupported, or fallback-generated.

The knowledge system is real, but the operator experience is still closer to a
developer control panel than a knowledge operations console. Operators can upload,
inspect, and diagnose documents, but they cannot yet quickly answer:

- Which spaces are healthy?
- Which documents are stale, duplicated, empty, disabled, or never cited?
- Which user questions are unsupported because the knowledge base lacks coverage?
- Why did a given retrieval result win?
- What should I fix next to make the digital human more useful?

Phase 14 should move knowledge from "manageable" to "operable."

## Goal

Deliver a focused Knowledge Operations Console that helps an operator maintain
and improve the digital human's knowledge base without introducing a database,
external search service, or provider-required ingestion dependency.

The console should make local knowledge health visible and actionable:

1. summarize knowledge-space health at a glance;
2. expose document-level quality and index signals;
3. provide a retrieval debugging workbench for one question at a time;
4. surface gaps where the assistant could not ground an answer;
5. keep all data local, deterministic, and testable.

## Office-Hours Premise Challenge

The obvious next step is "make `/admin` prettier." That would help, but it is not
the real product constraint.

The deeper issue is that a professional digital human needs knowledge operations,
not just knowledge CRUD. A prettier upload table still leaves the operator guessing
which sources are helping, which are stale, and why the assistant failed to cite
anything.

The better premise for Phase 14 is:

> Knowledge management becomes valuable when it tells the operator what to fix next.

That keeps the phase ambitious but practical. We should improve decision quality
for the operator before adding more ingestion formats such as PDF, DOCX, web
crawling, or cloud sync.

## Product Thesis

The knowledge base should behave like an operational system with a health surface.

The operator should be able to see:

- the selected space's readiness;
- document freshness and chunk/index state;
- recent or saved unsupported questions;
- retrieval result explanations;
- the next best maintenance actions.

The first version does not need autonomous curation. It needs honest, local,
inspectable signals.

## In Scope

### Knowledge Space Dashboard

- Add a compact dashboard for each knowledge space.
- Show aggregate counts:
  - active documents;
  - disabled documents;
  - failed/stale documents;
  - total chunks;
  - last updated time;
  - diagnostic-ready status.
- Show a health status derived from deterministic local rules:
  - healthy;
  - needs attention;
  - empty;
  - disabled;
  - stale or failed.

### Document Detail And Quality Signals

- Add a document detail view or detail region in `/admin`.
- Show:
  - metadata;
  - chunk preview;
  - index state;
  - last error code if present;
  - retrieval participation;
  - simple quality hints such as empty content, duplicate title/content hash, no chunks, disabled, stale, or failed.
- Keep actions operational:
  - enable/disable;
  - reindex;
  - inspect chunks;
  - run diagnostics against this document or its space.

### Retrieval Debug Workbench

- Add a space-scoped debug panel for one query.
- Show:
  - selected space;
  - retrieval mode;
  - ranked chunks;
  - score breakdown;
  - citation decision;
  - no-source reason;
  - included/excluded counts where available.
- Make it obvious when no answer should be grounded.

### Knowledge Gap Capture

- Capture unsupported or no-source questions as local gap records when the app
  has enough metadata to do so safely.
- Show gaps in `/admin` by space:
  - question;
  - timestamp;
  - no-source reason;
  - selected space;
  - optional status: open, ignored, resolved.
- Keep this deterministic and local-first. Gap capture should not require LLM
  evaluation.

### App To Admin Bridge

- `/app` should continue to stay compact.
- When a turn is not grounded, the metadata should make it possible for `/admin`
  to show a gap.
- Do not turn `/app` into the operations console.

## Out Of Scope

- SQLite, Postgres, hosted vector DB, or managed search migration.
- PDF/DOCX/web crawler ingestion.
- Automatic document rewriting or autonomous source curation.
- Full analytics warehouse or event streaming backend.
- RBAC, SaaS workspaces, billing, or tenant-level dashboards.
- LLM-generated health scores in CI.
- A new frontend framework.

## Alternatives Considered

### Alternative A: Visual Redesign Only

Restyle `/admin` into a cleaner page while keeping the same information model.

Pros:

- fast visible improvement;
- low backend risk;
- useful for perceived polish.

Cons:

- still does not tell the operator what is broken;
- hides the fact that knowledge health is underspecified;
- risks making the product look mature without being more operable.

Verdict: not enough for Phase 14.

### Alternative B: Add More Ingestion Formats

Add PDF, DOCX, web page, or cloud-drive import.

Pros:

- strong demo value;
- closer to real-world document workflows;
- expands the data surface quickly.

Cons:

- creates more content before health and gap visibility exist;
- parsing and binary dependencies increase test complexity;
- does not explain why answers fail or why citations are weak.

Verdict: important later, premature now.

### Alternative C: Knowledge Operations Console

Add health, document quality signals, retrieval debugging, and gap capture.

Pros:

- directly improves operator trust and maintenance;
- builds on Phase 10-12 data instead of replacing it;
- creates a clean foundation for future ingestion formats;
- stays local-first and deterministic.

Cons:

- touches admin UI, knowledge service, runtime metadata, and tests;
- requires careful copy so health signals do not overclaim semantic quality.

Verdict: recommended.

## Recommended Scope

Ship Phase 14 as **Knowledge Operations Console**:

- add a knowledge-space dashboard in `/admin`;
- add document quality/detail signals;
- expand retrieval diagnostics into a clearer debug workbench;
- record local knowledge gaps from no-source/unsupported turns;
- keep `/app` focused while letting its metadata feed admin operations;
- keep persistence local file-backed and tests deterministic.

## Proposed Architecture

```text
/app grounded/no-source turn metadata
  -> local gap capture
  -> admin knowledge gap list

/admin selected space
  -> knowledge health summary
  -> document quality/detail signals
  -> retrieval debug workbench
  -> maintenance actions

internal/admin KnowledgeService
  -> local file store envelope
  -> knowledge documents + spaces + gap records

internal/knowledge Pipeline
  -> score breakdown + no-source reasons
  -> admin debug response
```

## Data Model Draft

### KnowledgeHealthSummary

Required fields:

- `space_id`
- `space_name`
- `status`
- `active_document_count`
- `disabled_document_count`
- `failed_document_count`
- `stale_document_count`
- `chunk_count`
- `last_updated_at`
- `attention_reasons`

### KnowledgeDocumentQuality

Required fields:

- `document_id`
- `space_id`
- `status`
- `chunk_count`
- `content_hash`
- `index_state`
- `last_indexed_at`
- `last_error_code`
- `quality_flags`

### KnowledgeGap

Required fields:

- `id`
- `tenant_id`
- `space_id`
- `question`
- `no_source_reason`
- `status`
- `created_at`
- `updated_at`
- optional `resolved_by_document_id`

The first slice should use local file storage and deterministic IDs. No database
migration should be introduced.

## UX Requirements

### `/admin`

- The selected knowledge space should have a compact health header.
- Document rows should expose quality flags without becoming a wall of badges.
- Selecting a document should reveal chunks, metadata, and maintenance actions.
- Retrieval debug should show why a chunk ranked or why no source was accepted.
- Knowledge gaps should be visible as a small queue, not a separate product.

### `/app`

- Keep the current compact space selector and Presence summary.
- Continue showing grounding/citation/no-source state.
- Do not add admin-only diagnostics to the chat surface.

## Acceptance Criteria

1. `/admin` can show a health summary for the selected knowledge space.
2. Health summary is deterministic and derived from local document/index state.
3. Document detail exposes chunks, index state, safe error code, and quality flags.
4. Retrieval debug workbench shows ranked chunks and no-source decisions for a query.
5. No-source or unsupported turns can produce local knowledge gap records.
6. Gap records can be listed and marked ignored/resolved without provider calls.
7. `/app` remains compact and does not become an admin dashboard.
8. All tests run locally without SQLite, external vector DB, or real DeepSeek calls.
9. Existing Phase 10-13 behavior remains backward compatible.

## Test Matrix Seed

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Health | Empty active space | Health status is `empty` with clear attention reason |
| Health | Failed/stale document exists | Health status becomes `needs_attention` |
| Health | Active ready documents | Summary counts documents and chunks deterministically |
| Document detail | Inspect document | Metadata, chunks, index state, and safe error code render |
| Quality | Duplicate content hash | Document quality flags include duplicate candidate |
| Diagnostics | Query with source | Ranked chunks and score breakdown are visible |
| Diagnostics | Query with no support | No-source reason is visible; no fake citation appears |
| Gap capture | Unsupported `/app` turn | Local gap record is created with selected space |
| Gap lifecycle | Mark gap ignored/resolved | Status persists after reload |
| Regression | Existing knowledge upload/list/chat | Phase 12 behavior remains intact |

## Open Questions For Stage 2

1. Should Phase 14 persist gap records in the existing knowledge envelope or in a
   separate local file?
2. Should health status be one top-level enum or a list of attention reasons only?
3. Should duplicate detection use content hash only, title similarity only, or
   both?
4. Should gap capture be automatic for every no-source turn, or only when the user
   asks in knowledge-grounded mode?
5. Should document detail be an inline panel in the existing admin page or a
   separate route later?

## Recommended Assignment Before Stage 2

Approve or revise the Phase 14 scope:

> Build the Knowledge Operations Console as a local-first admin slice: space
> health, document quality/detail, retrieval debug, and gap records. Defer new
> ingestion formats and database migration.
