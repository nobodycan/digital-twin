# Phase 14 Knowledge Operations Console Design

Date: 2026-07-02

Status: Approved for implementation on 2026-07-02

Source spec: [Phase 14 Knowledge Operations Console Spec](../specs/phase-14-knowledge-operations-console.md)

## Office-Hours Summary

Phase 14 should not be "make admin prettier" and it should not jump straight to
PDF/DOCX/web ingestion. The next valuable product layer is operations: make it
clear which knowledge spaces are healthy, which documents need attention, why
retrieval did or did not support an answer, and which user questions reveal gaps
in the knowledge base.

This keeps the project moving toward a professional digital human rather than an
upload-and-chat demo.

## Premises

1. Knowledge scope already exists through Phase 12 spaces.
2. Retrieval diagnostics already exist through Phase 11 and should be reused.
3. The operator's next pain is maintenance and trust, not raw ingestion volume.
4. Local file storage remains the persistence layer.
5. Health signals must be deterministic and should not pretend to be semantic
   truth when they are only local operational heuristics.

## Narrowest Valuable Wedge

Add four linked surfaces:

1. knowledge-space health summary;
2. document detail and quality flags;
3. retrieval debug workbench;
4. local knowledge gap records.

Together, these answer the operator's core loop:

> What is broken, why did the assistant fail, and what should I fix?

## Recommended Architecture

```text
internal/admin
  KnowledgeService
    -> health summary from spaces + documents + index state
    -> document quality projection
    -> gap record lifecycle
    -> existing FileKnowledgeStore

internal/knowledge
  Pipeline
    -> retrieval diagnostics + no-source reasons
    -> debug workbench response

internal/runtime + internal/agents
  no-source / unsupported metadata
    -> local gap capture adapter

web/admin
  selected space
    -> health header
    -> document table + detail region
    -> retrieval debug panel
    -> gap queue
```

## Component Design

### `internal/admin`

Responsibilities:

- compute deterministic health summaries;
- expose document quality projections;
- persist and mutate knowledge gap records;
- keep store operations local and tenant-scoped;
- avoid owning ranking logic that belongs in `internal/knowledge`.

Likely additions:

- `KnowledgeHealthSummary`
- `KnowledgeDocumentQuality`
- `KnowledgeGap`
- `ListKnowledgeHealth`
- `GetKnowledgeDocumentDetail`
- `ListKnowledgeGaps`
- `CreateKnowledgeGap`
- `UpdateKnowledgeGapStatus`

### `internal/admin.FileKnowledgeStore`

Preferred persistence:

- keep spaces/documents in the existing knowledge envelope;
- Stage 2 should decide whether gaps live in the same envelope or a separate
  `knowledge_gaps.json`.

Recommendation:

- use a separate local gaps file for lower migration risk and clearer rollback;
- keep IDs deterministic enough for tests.

### `internal/knowledge`

Responsibilities:

- preserve current retrieval pipeline;
- expose workbench-friendly diagnostics from existing score explanations;
- make no-source reasons specific enough for operators:
  - empty space;
  - disabled space;
  - no ready documents;
  - no lexical match;
  - below grounding threshold;
  - vector unavailable.

### `internal/runtime`

Responsibilities:

- preserve current chat behavior;
- emit enough allowlisted metadata for gap capture;
- avoid storing sensitive raw provider errors or hidden prompts in gap records.

### `web/admin`

Information architecture:

1. selected space header and health summary;
2. document table with compact quality indicators;
3. document detail region;
4. retrieval debug workbench;
5. knowledge gap queue.

The page should remain dense and operational. No landing-page treatment, no large
hero, and no decorative visual panels.

### `web/app`

Keep `/app` focused:

- selected space;
- conversation;
- Presence summary;
- grounding/no-source/fallback signals.

Do not expose document health, chunk ranking, or gap queue there.

## Data Flow

### Health Summary

1. Admin selects a knowledge space.
2. Server loads spaces and documents.
3. Knowledge service derives counts and attention reasons.
4. `/admin` renders a compact health strip.

### Document Detail

1. Operator selects a document row.
2. Admin API returns metadata, chunks, index state, and quality flags.
3. UI renders detail without changing document lifecycle state.

### Retrieval Debug

1. Operator enters a query for selected space.
2. Existing diagnostics pipeline ranks chunks.
3. UI shows result explanations and grounding decision.
4. If no source is accepted, UI shows the no-source reason without inventing a
   citation.

### Gap Capture

1. `/app` turn completes with no-source or unsupported metadata.
2. Runtime or server adapter creates a local gap record when the event is safe to
   store.
3. `/admin` lists the gap under the selected space.
4. Operator can mark it ignored or resolved.

## Decisions For Stage 2

| Decision | Recommended Default | Why |
| --- | --- | --- |
| Gap storage | Separate local `knowledge_gaps.json` | Easier rollback and avoids bloating existing migration path |
| Health model | Enum plus attention reasons | Fast glance plus inspectable explanation |
| Duplicate detection | Content hash first | Deterministic and cheap; title similarity can follow |
| Gap capture | Automatic for knowledge-scoped no-source turns | Converts failures into an operator queue without extra user action |
| Document detail UI | Inline detail region | Avoids adding routing complexity to static admin page |
| Provider use | None required | Keeps CI deterministic and local-first |

## Risk Register

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Health score overclaims quality | High | Use operational labels and attention reasons, not fake confidence |
| Gap capture stores sensitive user text | High | Store only user-visible question text; keep tenant local; no provider payloads |
| Admin page becomes too dense | Medium | Use compact sections and a single selected-space context |
| Diagnostics logic forks from retrieval | Medium | Reuse existing pipeline outputs |
| Gap records become noisy | Medium | Add ignored/resolved states and deterministic dedupe rules |
| Existing knowledge behavior regresses | High | Stage 2 must include Phase 12 regression tests |

## Success Signal

Phase 14 is successful when an operator can:

1. open `/admin` and immediately see whether a space needs attention;
2. inspect a document and understand its chunks/index state;
3. debug a failed grounded answer without reading server logs;
4. see no-source questions become local knowledge gaps;
5. improve the knowledge base while `/app` remains clean and focused.

## Assignment For Stage 2

Turn this design into an implementation plan with:

- exact API request/response shapes;
- local persistence decision for gaps;
- ordered TDD work items;
- test matrix for health, document detail, diagnostics, gaps, and regressions;
- UI layout contract for `/admin`;
- security notes for stored user questions and safe error rendering.

Recommended Stage 2 default:

> Use separate local gap persistence, enum plus attention reasons for health,
> content-hash duplicate detection, automatic gap capture for knowledge-scoped
> no-source turns, and an inline document detail region in `/admin`.
