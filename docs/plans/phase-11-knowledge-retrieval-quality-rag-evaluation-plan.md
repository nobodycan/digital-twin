# Phase 11 Knowledge Retrieval Quality and RAG Evaluation Plan

Date: 2026-06-29

Status: Draft plan review complete, waiting for user approval

Source spec: [Phase 11 Knowledge Retrieval Quality and RAG Evaluation Spec](../specs/phase-11-knowledge-retrieval-quality-rag-evaluation.md)

Source design: [Phase 11 Knowledge Retrieval Quality and RAG Evaluation Design](../design/phase-11-knowledge-retrieval-quality-rag-evaluation.md)

Mode: SDD Stage 2 / gstack autoplan

## Plan Summary

Phase 11 upgrades knowledge from "keyword-grounded answers" to an auditable retrieval
quality layer. The plan keeps lexical retrieval as the deterministic baseline, adds
optional local/test vector signals, introduces hybrid reranking with explanations,
adds local RAG eval fixtures, and upgrades `/admin` citation testing into retrieval
diagnostics.

No implementation code should be written until this plan is approved.

## Review Summary

### CEO Review

Score: 9/10.

The product direction is right: do not rush into a provider-specific vector demo.
The bigger product move is making the digital human a trustworthy knowledge worker
whose sources, confidence, and failure modes are inspectable.

Scope decision:

- Keep Phase 11 focused on retrieval quality and evaluation.
- Do not add PDF/DOCX/cloud ingestion, managed vector DBs, or RBAC.
- Treat embeddings as an optional signal behind local-first contracts.

### Design Review

Score: 8/10.

The UI scope is meaningful but should stay restrained. `/admin` needs retrieval
diagnostics; `/app` should remain compact and only surface low-confidence or
grounding state. Do not turn the main app into a search analytics dashboard.

Design decision:

- `/admin`: score breakdowns, retrieval mode, index state, no-result explanations.
- `/app`: compact grounded/no-source/insufficient-support state only.

### Engineering Review

Score: 8/10.

The existing code already has the right building blocks:

- `internal/knowledge.Retriever` for deterministic lexical ranking.
- `internal/knowledge.Service` and `internal/admin.KnowledgeService` for grounding
  and lifecycle.
- `llm.Client.Embed` and `store.VectorStore` for vector extension points.
- `/admin/knowledge/citation-test` as the natural API to evolve into diagnostics.
- `PersonaAgent` and runtime metadata allowlisted grounding path.

Engineering decision:

- Add a retrieval pipeline in `internal/knowledge` rather than moving retrieval
  into admin or runtime.
- Keep vector index local and optional.
- Add evals before wiring UI polish, so quality behavior is defined by tests.

### DX Review

Score: 8/10.

Developer experience should remain copy-paste local. No API keys should be required
to run tests. The happy path should be:

1. upload fixture knowledge;
2. run retrieval diagnostics;
3. run `go test ./internal/knowledge ./internal/admin ./internal/server`;
4. see whether lexical/vector/hybrid selected the expected source.

DX decision:

- Keep the first vector mode deterministic and local.
- Add docs and fixture names that explain the retrieval modes.
- Error messages must include problem, cause, and fix.

## Cross-Phase Themes

**Theme: optional vector, mandatory explainability.** CEO, Eng, and DX all point
to the same thing: vector retrieval is valuable only if it remains optional,
observable, and testable.

**Theme: no fake confidence.** CEO, Design, and Eng all flag unsupported grounded
answers as the main product risk. The implementation must include no-source and
low-confidence tests before chat metadata changes ship.

## Architecture

```text
web/admin.js
  | POST /admin/knowledge/retrieval-diagnostics
  v
internal/server
  | safe request/response contracts
  v
internal/admin.KnowledgeService
  | document lifecycle + index-state metadata
  v
internal/knowledge.Pipeline
  | lexical stage
  | optional vector stage
  | deterministic reranker
  | result explanations
  v
internal/knowledge eval harness
  | local fixtures + golden cases
  v
internal/app + PersonaAgent
  | grounded prompt context + low-confidence metadata
  v
web/app.js
```

## Package Boundaries

| Package | Responsibility | Notes |
| --- | --- | --- |
| `internal/knowledge` | Retrieval pipeline, lexical/vector stages, reranking, explanations, eval harness | Main Phase 11 package |
| `internal/admin` | Knowledge document lifecycle and index-state persistence | Do not make admin own ranking logic |
| `internal/server` | Admin diagnostics API and safe response mapping | JSON POST actions match current style |
| `internal/app` | Runtime assembly and dependency wiring | Keep provider/vector config optional |
| `internal/agents` | Grounding metadata and low-confidence behavior | Preserve prompt-injection boundary |
| `web` | Admin diagnostics UI and compact app grounding state | No dashboard sprawl |
| `docs` | README/release notes and eval usage docs | Update after behavior exists |

## Data Contracts

### RetrievalMode

Add typed mode values:

- `lexical`
- `vector`
- `hybrid`
- `auto`

Default:

- `auto`, where lexical always runs and vector contributes only when index state
  is ready and vector config is enabled.

### RetrievalPipelineRequest

Fields:

- `tenant_id`
- `query`
- `limit`
- `mode`
- `tags`
- `include_disabled`
- `min_score`

`include_disabled` is for admin diagnostics only. Runtime grounding must keep
disabled documents excluded.

### RetrievalPipelineResult

Fields:

- `mode`
- `results`
- `explanations`
- `no_source_reason`
- `stages_run`
- `stages_skipped`

### RetrievalExplanation

Fields:

- `document_id`
- `document_name`
- `chunk_id`
- `rank`
- `lexical_score`
- `vector_score`
- `final_score`
- `matched_terms`
- `rank_reason`
- `index_status`

### Index Metadata

Add metadata without breaking existing JSON:

- `lexical_ready`
- `vector_status`: `missing`, `ready`, `failed`
- `embedding_model`
- `embedding_version`
- `indexed_at`
- `last_error_code`

Recommended persistence:

- Start by storing index-state metadata in the existing knowledge JSON.
- Do not persist vector arrays in Phase 11 unless Stage 3 proves it is needed.
- Keep fake/local vector values reconstructible for tests.

## Implementation Workstreams

### P11-01: Retrieval Contract and Lexical Stage

Goal: move current lexical retrieval behind a pipeline contract without changing
existing lexical behavior.

Tasks:

1. Add failing tests for `RetrievalPipeline` lexical mode.
2. Preserve current deterministic ranking and disabled-document filtering.
3. Add result explanations with lexical score and matched terms.
4. Keep `Retriever.Search` either as a stage dependency or compatibility wrapper.

Acceptance:

- Existing `internal/knowledge` tests pass.
- New lexical pipeline tests prove ranking parity with Phase 10.

### P11-02: Deterministic Vector Stage

Goal: add optional vector signal without real provider dependencies.

Tasks:

1. Add failing tests for fake/local vector agreement improving rank.
2. Add a local deterministic vectorizer or adapter around `llm.Client.Embed`.
3. Add vector-unavailable behavior that records skipped stage and falls back to
   lexical.
4. Do not call real DeepSeek or external embeddings in tests.

Acceptance:

- Hybrid mode can use fake/local vector scores.
- Vector failure never disables lexical retrieval.

### P11-03: Hybrid Reranker and Low-Confidence Decision

Goal: make source selection explainable and prevent weak citations.

Tasks:

1. Add failing tests for hybrid score combination.
2. Add stable tie-breakers by final score, document ID, chunk ordinal.
3. Add low-confidence/no-source decision contract.
4. Ensure runtime metadata does not emit citations below threshold.

Acceptance:

- Hybrid ranking is deterministic.
- Weak retrieval produces `no_source_reason` and no fake citation metadata.

### P11-04: Knowledge Index State

Goal: represent lexical/vector readiness in document/chunk metadata.

Tasks:

1. Add failing admin store tests for index-state round trip.
2. Add index-state fields with backward-compatible JSON behavior.
3. Update upload/reindex to mark lexical ready.
4. Mark vector missing/ready/failed according to stage outcome.

Acceptance:

- Existing Phase 10 knowledge JSON remains loadable.
- Reindex updates index state deterministically.

### P11-05: Admin Retrieval Diagnostics API

Goal: evolve citation test into diagnostics while preserving compatibility.

Tasks:

1. Add `POST /admin/knowledge/retrieval-diagnostics`.
2. Keep existing `/admin/knowledge/citation-test` behavior or map it through the
   new pipeline.
3. Return safe explanations and no-source reasons.
4. Ensure admin diagnostics can optionally show disabled/skipped state without
   runtime using disabled chunks.

Acceptance:

- Server tests cover success, no-source, vector-missing, and invalid JSON.
- Response does not expose local paths, API keys, or hidden prompts.

### P11-06: RAG Evaluation Harness

Goal: create deterministic quality tests for citations and grounded decisions.

Tasks:

1. Add fixture format under `internal/knowledge/testdata` or `internal/evals/testdata`.
2. Add golden cases for expected citations, disabled docs, no-source, and hostile
   source content.
3. Add test runner helpers that load fixtures into the local knowledge service.
4. Keep the harness package-level and CI-safe.

Acceptance:

- `go test ./internal/knowledge ./internal/evals` or equivalent runs the evals.
- Eval failures show expected source, actual source, and explanation.

### P11-07: Runtime and App Grounding Integration

Goal: use pipeline decisions in persona grounding without expanding hidden context.

Tasks:

1. Add failing agent/runtime tests for low-confidence no-citation behavior.
2. Wire pipeline grounding result into existing `KnowledgeGrounder` adapter.
3. Preserve prompt boundary: source text is reference, not instructions.
4. Add metadata fields only through the existing allowlist.

Acceptance:

- `/app` can distinguish grounded, no-source, and insufficient-support states.
- Prompt-injection tests still pass when hostile chunks are retrieved.

### P11-08: Admin UI Diagnostics

Goal: make retrieval failures debuggable by operators.

Tasks:

1. Add static tests for new admin DOM hooks and safe rendering.
2. Add UI controls for retrieval mode and diagnostics query.
3. Render result rows with score labels and expandable detail text.
4. Render vector-missing/failed states as actionable copy.

Acceptance:

- Admin diagnostics render without console assumptions.
- Text fits existing admin layout and does not replace unrelated admin sections.

### P11-09: Docs and Release Notes

Goal: keep local operation understandable.

Tasks:

1. Update README knowledge workflow with retrieval modes and eval command.
2. Update RELEASE_NOTES under Unreleased.
3. Document that vector retrieval is optional and local/test-first.
4. Add a short no-provider test path.

Acceptance:

- README explains how to prove Phase 11 locally without credentials.
- Release notes describe only shipped behavior.

## Recommended Build Order

1. `P11-01` Retrieval contract and lexical stage.
2. `P11-03` Hybrid reranker skeleton with lexical-only path.
3. `P11-02` Deterministic vector stage.
4. `P11-04` Index state persistence.
5. `P11-06` RAG eval harness.
6. `P11-05` Admin diagnostics API.
7. `P11-07` Runtime/app grounding integration.
8. `P11-08` Admin UI diagnostics.
9. `P11-09` Docs and release notes.

This order keeps the core retrieval behavior testable before server and UI work.

## Parallelization Plan

Parallel-safe after `P11-01` interfaces are stable:

- Workstream A: `internal/knowledge` pipeline, reranker, vector stage.
- Workstream B: admin/server diagnostics API.
- Workstream C: eval fixtures and docs.
- Workstream D: web UI diagnostics after API response is fixed.

Do not parallelize runtime grounding before the low-confidence contract is finalized.

## Test Plan

### Unit Tests

| Package | Test Focus |
| --- | --- |
| `internal/knowledge` | lexical parity, hybrid scoring, vector fallback, explanations, thresholds |
| `internal/admin` | index-state persistence, upload/reindex metadata, backward compatibility |
| `internal/server` | diagnostics API, safe errors, citation-test compatibility |
| `internal/agents` | prompt boundary, no fake citations, hostile chunk containment |
| `internal/app` | dependency wiring, vector-disabled default |
| `web` | static asset hooks, diagnostics rendering helpers if testable |

### Eval Tests

| Case | Expected |
| --- | --- |
| exact citation | expected chunk appears in top K |
| semantic fake-vector agreement | hybrid ranks vector-agreed chunk first |
| no supporting source | result is not grounded |
| disabled source | disabled document is never cited at runtime |
| hostile source | source text cannot override persona/safety/tool policy |
| vector failure | lexical fallback result includes vector-failed explanation |

### Integration Tests

- Upload document, diagnostics query, grounded chat.
- Reindex document and verify changed source selection.
- Low-confidence query and verify no citation in SSE metadata.
- Admin diagnostics response redacts unsafe internals.

### Browser QA

Use gstack/browser only after implementation:

1. Start server in local deterministic mode.
2. Open `/admin`.
3. Upload fixture knowledge.
4. Run diagnostics in lexical and auto modes.
5. Verify vector-missing copy is visible and not alarming.
6. Open `/app`.
7. Ask a grounded question and an unsupported question.
8. Verify citation and no-source states.

### Verification Commands

Run after Stage 3 implementation:

```powershell
go test ./internal/knowledge ./internal/admin ./internal/server ./internal/agents ./internal/app ./web
go test ./...
go vet ./...
```

Optional local smoke after implementation:

```powershell
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
.\scripts\smoke-conversation.ps1 -BaseUrl http://localhost:18080
.\scripts\stop-server.ps1
```

No command in the required plan depends on real DeepSeek or paid embeddings.

## Failure Modes Registry

| Failure Mode | Severity | Required Test or Mitigation |
| --- | --- | --- |
| Vector provider unavailable breaks all retrieval | High | Vector-unavailable fallback test |
| Weak source is cited as grounded | High | Low-confidence no-citation test |
| Disabled document appears in runtime answer | High | Disabled runtime exclusion test |
| Hostile document overrides prompt or tool policy | High | Hostile fixture and persona guard tests |
| Score explanations leak hidden prompt or local path | High | Safe response tests |
| Existing Phase 10 JSON cannot load | Medium | Backward compatibility store test |
| Admin UI overwhelms operator | Medium | Compact diagnostics design and browser QA |
| Hybrid scoring unstable across map iteration | Medium | Stable tie-breaker tests |

## Error and Rescue Registry

| Error | User-Facing Cause | Fix |
| --- | --- | --- |
| `retrieval_no_source` | No ready source matched strongly enough | Upload/reindex a relevant document or lower diagnostics threshold |
| `vector_index_missing` | Vector retrieval is enabled but no vector index exists | Reindex or keep lexical/auto mode |
| `vector_index_failed` | Embedding/indexing failed for this document | Inspect provider config, then reindex |
| `retrieval_invalid_mode` | Request used an unsupported retrieval mode | Use `lexical`, `vector`, `hybrid`, or `auto` |
| `knowledge_diagnostics_unavailable` | Knowledge service is not configured | Start server with local data directory and knowledge admin enabled |

## Decisions

| # | Decision | Classification | Rationale |
| --- | --- | --- | --- |
| 1 | Use `auto` retrieval as the target default, with lexical guaranteed | Auto-decided | Best product behavior without sacrificing local determinism |
| 2 | Keep vector optional and local/test-first in Phase 11 | Auto-decided | Avoid provider dependency and flaky CI |
| 3 | Store index-state metadata with knowledge JSON first | Auto-decided | Smallest compatible persistence change |
| 4 | Do not persist vector arrays unless implementation proves need | Auto-decided | Keeps local file readable and avoids large JSON churn |
| 5 | Add diagnostics endpoint instead of overloading citation-test only | Auto-decided | Preserves Phase 10 compatibility and creates a richer admin contract |
| 6 | Place eval fixtures under testdata, docs as explanation only | Auto-decided | Keeps evals executable and CI-owned |
| 7 | Keep `/app` scoring details hidden | Auto-decided | User-facing app should show trust state, not ranking internals |

## Implementation Tasks

- [ ] P11-01: Add retrieval pipeline lexical contract and parity tests.
- [ ] P11-02: Add deterministic vector stage and vector-unavailable fallback tests.
- [ ] P11-03: Add hybrid reranker and low-confidence/no-source contract.
- [ ] P11-04: Add index-state metadata and store compatibility tests.
- [ ] P11-05: Add admin retrieval diagnostics API and safe response tests.
- [ ] P11-06: Add local RAG eval fixtures and harness.
- [ ] P11-07: Wire grounding decisions into runtime/persona metadata.
- [ ] P11-08: Add admin diagnostics UI.
- [ ] P11-09: Update README and release notes.

## Definition of Done

- All acceptance criteria in the spec are covered by tests or documented as out
  of scope.
- Existing Phase 10 knowledge behavior remains compatible.
- `go test ./...` passes.
- No real provider calls are required in CI.
- `/admin` can explain source selection and vector/index state.
- `/app` avoids fake citations when evidence is weak.

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | --- | --- | --- |
| CEO Review | `$gstack-autoplan` | Scope and strategy | 1 | Clear | Phase should optimize for auditable retrieval quality, not provider-specific embeddings |
| Design Review | `$gstack-autoplan` | UI/UX gaps | 1 | Clear | Admin diagnostics needed; app should stay compact |
| Eng Review | `$gstack-autoplan` | Architecture and tests | 1 | Clear | Pipeline belongs in `internal/knowledge`; vector optional; evals before UI |
| DX Review | `$gstack-autoplan` | Developer experience | 1 | Clear | Local deterministic tests and copy-paste commands remain mandatory |

**VERDICT:** CEO + Design + Eng + DX cleared for Stage 3 implementation after user plan approval.

NO UNRESOLVED DECISIONS
