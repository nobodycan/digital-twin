# Phase 11 Knowledge Retrieval Quality and RAG Evaluation Spec

Date: 2026-06-29

Status: Approved by the user on 2026-06-29

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 10 Knowledge Base and Memory Control Spec](./phase-10-knowledge-base-memory-control.md)
- [Phase 10 Knowledge Base and Memory Control Design](../design/phase-10-knowledge-base-memory-control.md)
- User direction: "I have ambition; I also want knowledge base management, not only memory."
- Current implementation: local file-backed knowledge lifecycle, deterministic lexical retrieval, grounded chat metadata, `/admin` lifecycle controls, and `/app` citation display.

## Context

Phase 10 made knowledge real: an operator can upload local text/Markdown documents,
inspect them, enable/disable them, run citation tests, and see source grounding in
the digital-human app.

The system is now useful, but the retrieval layer is intentionally modest. It uses
deterministic lexical matching so CI and local operation stay stable. That was the
right first move. The next risk is that the product looks like a knowledge
assistant but behaves like a keyword search box with a chat UI on top.

A professional digital human needs stronger knowledge behavior:

- it should retrieve semantically related material, not only literal term matches;
- it should preserve source trust and citation discipline;
- it should know when retrieved evidence is too weak to answer;
- it should be measurable with local golden questions before adding heavier
  provider dependencies;
- it should make retrieval strategy visible enough that operators can debug bad
  answers.

## Goal

Deliver a Phase 11 slice that improves knowledge quality without abandoning the
local-first architecture:

1. add a retrieval pipeline abstraction that can combine lexical and vector-style
   signals;
2. introduce an embedding provider boundary with deterministic local/test modes;
3. support hybrid retrieval contracts: lexical baseline, optional vector index,
   and deterministic reranking;
4. add a local RAG evaluation harness with golden question/answer/citation cases;
5. expose enough retrieval diagnostics in `/admin` to explain why a source was
   selected or skipped;
6. preserve safe fallback behavior when embedding or provider calls are missing.

## Office-Hours Premise Challenge

The obvious next step is "add embeddings." That is directionally correct, but too
implementation-first.

Embeddings do not automatically make a knowledge base trustworthy. They can improve
recall, but they also introduce new failure modes:

- provider availability can break indexing;
- embedding model changes can silently change retrieval behavior;
- semantically close chunks can be confidently wrong for the user's question;
- vector scores are hard for operators to interpret;
- CI can become flaky if real provider calls are required.

The better premise for Phase 11 is:

> The digital human needs a measurable knowledge retrieval layer before it needs a
> specific vector provider.

This keeps the ambition high while protecting the product from becoming an opaque
RAG demo.

## Product Thesis

Phase 11 should turn the knowledge base from "managed documents plus citations"
into "auditable retrieval quality."

The professional digital human should be able to show:

- "I chose these sources because lexical/vector/rerank signals agreed."
- "I found related material, but confidence is too low to answer from it."
- "This document is searchable lexically but has no vector index yet."
- "This answer passed or failed a local golden evaluation case."

The product value is not just better answers. It is better operator control over
why answers are grounded.

## In Scope

### Retrieval Pipeline

- Introduce an internal retrieval pipeline contract that accepts:
  - tenant ID;
  - query;
  - top K;
  - retrieval mode;
  - optional filters such as document status, tags, source type, and recency.
- Preserve the existing lexical retriever as a required deterministic stage.
- Add optional vector retrieval behind explicit config and dependency checks.
- Add a deterministic reranking stage that can combine:
  - lexical score;
  - vector score when available;
  - exact phrase boost;
  - document freshness or status penalty if designed in Stage 2;
  - stable tie-breakers.
- Return structured explanations for each result:
  - matched lexical terms;
  - vector score if used;
  - final score;
  - rank reason;
  - retrieval mode.

### Embedding Provider Boundary

- Add a provider-neutral embedding interface rather than binding to a single vendor.
- Keep deterministic fake/local embeddings for tests.
- Support a local hash or token-vector embedding mode for development if useful.
- Allow OpenAI-compatible embedding providers later, but do not require real calls
  in CI.
- Treat DeepSeek chat configuration as separate from embedding support. If the
  configured provider does not support embeddings, the system must keep lexical
  retrieval working and report a clear status.

### Knowledge Index State

- Extend knowledge document/chunk metadata to represent index readiness:
  - lexical-ready;
  - vector-ready;
  - vector-missing;
  - vector-failed;
  - embedding model/version;
  - indexed timestamp.
- Reindex should rebuild the available indexes deterministically.
- Failed vector indexing must not make the document unusable for lexical retrieval.

### RAG Evaluation Harness

- Add a local evaluation format for knowledge quality cases:
  - fixture documents;
  - user question;
  - expected source document/chunk;
  - optional expected answer fragments;
  - expected grounded/not-grounded decision.
- Add a command or package-level test harness that runs without real providers.
- Measure at least:
  - top-1 citation hit;
  - top-K citation hit;
  - no-source precision;
  - prompt-injection refusal/containment;
  - disabled-document exclusion.
- Keep initial metrics simple and deterministic; do not add dashboards yet.

### Admin Diagnostics

- Upgrade `/admin` citation test into retrieval diagnostics:
  - selected retrieval mode;
  - top results with per-stage scores;
  - whether vector index was used;
  - skipped/disabled document visibility when safe;
  - no-result explanation.
- Do not expose secrets, provider API keys, raw local paths, or hidden prompts.

### Chat Grounding Behavior

- Runtime should continue to cite only retrieved chunks.
- If retrieval returns weak or no evidence, chat should not fabricate citations.
- Add a grounded-answer threshold or decision contract in Stage 2.
- Preserve existing `fallback_to_local` and `fail_closed` provider semantics.
- Keep knowledge retrieval independent from conversational memory.

## Out of Scope

- Replacing local file storage with SQLite, Postgres, or a managed vector DB.
- Mandatory external embedding calls.
- PDF, DOCX, web crawler, Notion, Google Drive, GitHub, or browser ingestion.
- Full RBAC, document ACLs, billing, multi-tenant cloud operations.
- Human feedback training loops.
- Autonomous web browsing or self-updating knowledge.
- Complex LLM reranking that requires provider calls in CI.
- Claiming production-grade enterprise knowledge management.

## Alternatives Considered

### Alternative A: Pure Embedding Upgrade

Add embeddings and vector search as the primary retrieval path.

Pros:

- likely improves semantic recall for many natural-language questions;
- aligns with common RAG architecture;
- makes use of existing vector store and embedding skill concepts.

Cons:

- can make the product provider-dependent before evals exist;
- does not explain retrieval failures to the operator;
- may regress deterministic CI if real embeddings are required;
- ignores citation quality and no-answer behavior.

Verdict: useful later, but too narrow and too opaque as the main Phase 11 frame.

### Alternative B: Retrieval Evaluation First, No Vector Work

Build only golden tests, metrics, and retrieval diagnostics around the existing
lexical retriever.

Pros:

- safest and most deterministic;
- improves confidence before changing retrieval behavior;
- small implementation surface.

Cons:

- does not materially improve recall;
- delays the user's knowledge-base ambition;
- may overfit evaluation around a known-limited retriever.

Verdict: strong discipline, but too conservative for the next product step.

### Alternative C: Hybrid Retrieval Quality Layer

Build a retrieval pipeline with lexical baseline, optional vector signals,
deterministic reranking, index-state metadata, and a local RAG evaluation harness.

Pros:

- improves quality while preserving local-first behavior;
- keeps lexical retrieval as a stable fallback;
- makes vector retrieval an extension point instead of a product dependency;
- creates objective tests for future provider/model changes;
- gives operators diagnostics when retrieval is wrong.

Cons:

- larger than a simple embedding patch;
- requires careful contracts for scores and thresholds;
- Stage 2 must keep scope tight to avoid building a full search platform.

Verdict: recommended.

## Recommended Approach

Ship Phase 11 as a **Retrieval Quality and RAG Evaluation** slice:

- define a retrieval pipeline abstraction in or near `internal/knowledge`;
- keep lexical retrieval mandatory and deterministic;
- add optional vector retrieval through a provider-neutral embedding/index boundary;
- add stable hybrid reranking and result explanations;
- add local golden evaluation fixtures for knowledge QA and citations;
- expose retrieval diagnostics in `/admin`;
- preserve existing `/app` grounding semantics while adding confidence/no-source
  behavior where needed.

## Proposed Architecture

```text
/admin retrieval diagnostics
  | query, mode, filters
  v
internal/server
  | safe diagnostic API
  v
internal/knowledge Pipeline
  | lexical stage
  | optional vector stage
  | deterministic reranker
  | explanations
  v
internal/admin KnowledgeService + FileKnowledgeStore
  | document/chunk/index metadata
  v
internal/runtime + PersonaAgent
  | grounded prompt context + no-source decision
  v
/app citation/source state

docs/evals or internal/evals fixtures
  | golden documents + questions + expected citations
  v
local test harness
```

## Data Model Draft

### RetrievalMode

Candidate values:

- `lexical`
- `vector`
- `hybrid`
- `auto`

Stage 2 should decide the default. Recommended default:

- `auto`, where lexical always runs and vector contributes only when index state
  is ready.

### ChunkIndexState

Required fields:

- `chunk_id`
- `lexical_ready`
- `vector_status`
- `embedding_model`
- `embedding_version`
- `indexed_at`
- `last_error_code`

### RetrievalExplanation

Required fields:

- `retrieval_mode`
- `lexical_score`
- `vector_score`
- `final_score`
- `matched_terms`
- `rank_reason`
- `index_status`

### RAGEvalCase

Required fields:

- `id`
- `documents`
- `question`
- `expected_document_ids`
- `expected_chunk_ids`
- `expect_grounded`
- `expected_answer_contains`
- `notes`

## UX Requirements

### `/admin`

- Citation test becomes retrieval diagnostics.
- Operator can choose or view retrieval mode.
- Results show score breakdowns without overwhelming the page.
- Missing vector index is visible as a state, not a generic error.
- No-source results explain whether the issue is no matching text, disabled docs,
  or vector index unavailable.

### `/app`

- Keep the current compact citation chips.
- Add or preserve a clear no-source state.
- Do not expose raw scoring internals in the normal chat surface.
- If answer confidence is low, the assistant should say the local knowledge does
  not contain enough support instead of presenting an unsupported answer as
  grounded.

## Acceptance Criteria

- Existing Phase 10 lexical retrieval behavior continues to pass.
- Retrieval pipeline supports lexical mode with deterministic ranking.
- Hybrid mode can combine lexical and fake/local vector scores in tests.
- Vector unavailable or failed state falls back to lexical without losing the
  document.
- Retrieval results include safe explanations for admin diagnostics.
- Knowledge document/chunk metadata can represent vector-ready and vector-failed
  states.
- RAG eval fixtures can assert expected citations and grounded/no-grounded
  decisions without real provider calls.
- Prompt-injection fixture remains contained when hybrid retrieval selects the
  hostile chunk.
- `/admin` can display retrieval mode, score breakdown, and index state.
- `/app` does not show fake citations when retrieval confidence is below the
  grounded threshold.
- CI remains deterministic and does not require DeepSeek or external embeddings.

## Test Matrix Seed

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Retrieval pipeline | Lexical mode query matches one chunk | Same stable ranking as Phase 10 |
| Retrieval pipeline | Hybrid mode with fake vector agreement | Final score ranks agreed result first |
| Retrieval pipeline | Hybrid mode with vector unavailable | Lexical results return with vector-missing explanation |
| Retrieval pipeline | Tie scores | Ranking is stable by score, document ID, chunk ordinal |
| Index state | Reindex document | Lexical-ready and vector status update deterministically |
| Index state | Embedding provider fails | Document remains lexical-searchable and vector status is failed |
| Eval harness | Golden question expected citation | Top-K hit is recorded as pass |
| Eval harness | No supporting source | Case passes only when no grounded citation is emitted |
| Eval harness | Disabled expected source | Disabled document is excluded and case fails visibly if cited |
| Prompt safety | Hostile chunk selected | Source text cannot override persona/safety/tool policies |
| Admin API | Diagnostics request | Returns safe score breakdown and index state |
| Admin UI | Missing vector index | Shows actionable state instead of generic failure |
| App UI | Low-confidence retrieval | Shows no-source/insufficient-support state, not fake citations |

## Open Questions for Stage 2

1. Should Phase 11 default retrieval mode be `auto` or keep `lexical` until vector
   indexing is explicitly enabled?
2. Should vector embeddings be stored in the same local knowledge JSON, or in a
   separate local index file to keep document metadata readable?
3. What is the first grounded threshold: minimum final score, minimum rank count,
   or an explicit reranker decision?
4. Should eval fixtures live under `docs/evals`, `internal/evals/testdata`, or
   both with docs as examples and Go testdata as execution source?
5. Should `/admin` expose score numbers directly, or use labels such as strong,
   partial, weak with expandable details?
6. Should the first vector mode use a deterministic local hash embedding only, or
   also wire OpenAI-compatible embeddings behind disabled-by-default config?

## Recommended Assignment Before Stage 2

Decide the Phase 11 default:

> Use `auto` retrieval mode, but make vector contribution opt-in until an index is
> ready. Lexical retrieval remains the guaranteed baseline.

This gives the system a clean path to semantic retrieval without breaking local
usage or CI. Stage 2 should turn this spec into an implementation plan with a
small test matrix first, then TDD every pipeline, index-state, eval, and UI
diagnostic behavior.
