# Phase 11 Knowledge Retrieval Quality and RAG Evaluation Design

Date: 2026-06-29

Status: Autoplan review complete, waiting for implementation approval

Source spec: [Phase 11 Knowledge Retrieval Quality and RAG Evaluation Spec](../specs/phase-11-knowledge-retrieval-quality-rag-evaluation.md)

## Office-Hours Summary

The next product move should not be "just add embeddings." Phase 10 already proved
that the digital human can manage local knowledge and show citations. Phase 11
should make that knowledge behavior measurable, inspectable, and extensible.

The strongest version of this phase is a retrieval-quality layer:

- lexical retrieval stays as the deterministic baseline;
- vector retrieval becomes an optional signal, not a dependency;
- reranking produces explainable source choices;
- local golden evals measure whether answers are grounded in the right chunks;
- `/admin` becomes the place to debug retrieval, not only upload documents.

## Premises

1. The user wants a professional digital human, not a toy RAG demo.
2. Knowledge base value comes from trust, source control, and measurable answer
   quality, not from the mere presence of embeddings.
3. Local-first remains a product constraint: no SQLite, no managed vector DB, and
   no real provider calls in CI.
4. DeepSeek chat integration should not be treated as proof that embeddings are
   available.
5. Retrieval needs to fail legibly. If evidence is weak, the assistant should
   avoid fake grounded confidence.

## Narrowest Valuable Wedge

Build a hybrid retrieval spine that can run entirely locally:

- lexical stage: current deterministic retriever;
- vector stage: fake/local deterministic vector signal for tests and optional
  provider-backed embedding later;
- reranker: stable combination of available scores;
- diagnostics: safe score breakdown and index state in `/admin`;
- evals: local fixture cases for expected citations and no-source decisions.

This is the smallest step that improves the product's long-term shape without
requiring an external search service.

## Approaches Considered

### Approach A: Pure Embeddings

This would add embedding generation and vector search quickly.

It is attractive because recall improves for natural questions, but it is too
opaque without evals and too provider-sensitive for this repo's current stage.
If the provider fails, the product risks regressing from "deterministic and honest"
to "better on demos, confusing in practice."

### Approach B: Evals Only

This would add golden RAG tests around the current lexical retriever.

It is disciplined and safe, but it does not advance the user's ambition enough.
The product would gain measurement while still missing semantic retrieval
extension points.

### Approach C: Hybrid Retrieval Quality Layer

This combines a retrieval pipeline, optional vector signal, deterministic
reranking, index-state metadata, diagnostics, and local evals.

It is the recommended path because it improves quality and extensibility together.
It also gives future phases a clean place to add real embeddings, document types,
or managed search without rewriting the chat/runtime boundary.

## Recommended Approach

Use Approach C.

Phase 11 should produce a small but durable retrieval architecture:

```text
Knowledge documents + chunks
  -> lexical retrieval
  -> optional vector retrieval
  -> deterministic reranker
  -> retrieval explanations
  -> grounded prompt context
  -> app citations and admin diagnostics
```

## Key Design Decisions for Stage 2

| Decision | Recommended Default | Why |
| --- | --- | --- |
| Retrieval mode | `auto` | Lexical always works; vector contributes only when ready |
| Vector dependency | Optional | Keeps CI and local startup deterministic |
| Eval source | Go `testdata` plus documented examples | Tests stay executable, docs stay readable |
| Score display | Compact labels plus expandable numeric detail | Operators need clarity without dashboard noise |
| Low-confidence behavior | No grounded citation | Prevents fake source confidence |
| Index storage | Decide in Stage 2 | Same JSON is simpler; separate file may age better |

## Data Flow

### Indexing

1. Operator uploads or reindexes a document.
2. Knowledge service chunks content as it does today.
3. Lexical index state is marked ready.
4. If vector mode is configured and an embedding provider is available, chunk
   embeddings are generated through a provider-neutral boundary.
5. Vector failures are recorded per document/chunk but do not disable lexical
   search.

### Retrieval

1. Runtime or `/admin` sends a query to the retrieval pipeline.
2. Lexical stage runs for ready documents.
3. Vector stage runs only when index state and config allow it.
4. Reranker combines available signals.
5. Pipeline returns ranked results with explanations.
6. Runtime passes only accepted chunks into persona prompt context.
7. Metadata remains allowlisted before reaching SSE or UI.

### Evaluation

1. Fixture documents are loaded into a local knowledge store.
2. Each eval case runs retrieval and, where needed, a deterministic persona path.
3. The harness checks expected citations, no-source decisions, and hostile-source
   containment.
4. Failures show which source was selected and why.

## Risks

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Vector work expands into full search infrastructure | Medium | Keep vector optional and local/test-first |
| Scores become hard to explain | Medium | Require `RetrievalExplanation` in the contract |
| Provider embeddings make tests flaky | High | No real provider calls in CI |
| Hybrid retrieval cites weak evidence | High | Add threshold/no-source tests before production logic |
| Metadata leaks internals | High | Continue allowlisting outbound metadata |
| Admin UI becomes too dense | Medium | Use compact rows with expandable diagnostic details |

## Acceptance Signal

Phase 11 is successful when a developer can:

1. upload local knowledge;
2. run retrieval diagnostics;
3. see whether lexical, vector, or hybrid signals selected a source;
4. run local RAG evals without provider credentials;
5. prove that low-confidence or unsupported questions do not produce fake
   citations.

## Assignment for Stage 2

Turn this design into an approved implementation plan with:

- concrete package boundaries;
- exact data model changes;
- retrieval mode defaults;
- index persistence decision;
- RAG eval fixture format;
- UI diagnostic states;
- TDD test matrix ordered RED -> GREEN -> REFACTOR.

Recommended first Stage 2 decision:

> Keep lexical retrieval mandatory, add vector as an optional stage, and make
> `auto` mode the product default only when diagnostics can show which stages ran.
