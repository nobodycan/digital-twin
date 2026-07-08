# Phase 20 Knowledge Evidence and Answer Trust Spec

Date: 2026-07-04

Status: Draft; waiting for user approval

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 15 Knowledge-Grounded Answer Loop Spec](./phase-15-knowledge-grounded-answer-loop.md)
- [Phase 16 Knowledge Workbench and Gap Resolution Spec](./phase-16-knowledge-workbench-gap-resolution.md)
- [Phase 17 Knowledge Curation and Source Management Spec](./phase-17-knowledge-curation-source-management.md)
- [Phase 18 Knowledge Source Ingestion Spec](./phase-18-knowledge-source-ingestion.md)
- [Phase 19 Knowledge Review and Activation Spec](./phase-19-knowledge-review-activation.md)
- Current implementation: local knowledge spaces, document lifecycle, retrieval
  diagnostics, grounded answer state, gap-to-note workflow, document curation,
  source ingestion, review activation, and review-gated retrieval.

## Context

Phase 19 made knowledge activation trustworthy: imported sources default to
review and only active reviewed documents participate in normal retrieval.

The next product question is:

> Can the user understand why a professional digital human answered the way it
> answered?

Today the system has useful primitives: answer state, citations, retrieval
diagnostics, source metadata, review status, and knowledge gaps. But the
experience is still too indirect. A user can see that an answer was grounded,
but not enough evidence is assembled into a coherent trust story.

Phase 20 should make every knowledge-backed answer inspectable:

- which documents supported it;
- which snippets were considered evidence;
- whether those snippets came from active reviewed knowledge;
- whether support was strong, partial, missing, or gated by review;
- what the operator should do next when support is weak.

## Office-Hours Premise Challenge

The tempting version of this phase is to build a large "explainability" system:

- LLM-generated rationales for every answer;
- semantic entailment scoring;
- citation span verification;
- source graph visualizations;
- confidence percentages;
- multi-hop reasoning traces;
- automatic source quality grades;
- full audit-event replay.

That is premature. The current product is local-first, deterministic in CI, and
still proving the core knowledge loop. A complex explainability stack would add
new provider dependence and could make the product sound more certain than it
is.

The opposite temptation is to stop at Phase 19 because retrieval is now gated.
That also misses the professional digital-human bar. A trusted answer is not
just "allowed to cite"; it must be inspectable by a human.

The narrow move is an **Evidence Pack**:

1. the runtime emits a compact evidence summary for each knowledge answer;
2. citations carry source, review, chunk, and snippet fields consistently;
3. `/app` shows a human-readable evidence panel instead of raw diagnostics;
4. `/admin` can inspect recent answer evidence and jump to source documents;
5. unsupported or review-gated answers keep feeding the existing gap workflow.

## Product Thesis

A professional digital human should make its knowledge posture obvious.

The user should be able to answer:

- What did it use as evidence?
- Was the evidence from reviewed active knowledge?
- Which source should I open if I want to inspect the claim?
- Did the system avoid using a pending or rejected source?
- Was the answer unsupported, partially supported, or fully grounded?
- What can an operator do to improve the answer next time?

## Goal

Deliver local-first Knowledge Evidence and Answer Trust:

1. define a stable answer evidence contract;
2. make citations more inspectable without leaking gated source text;
3. show a compact evidence panel in `/app`;
4. expose recent answer evidence or diagnostics in `/admin`;
5. connect weak or missing evidence to the existing knowledge-gap workflow;
6. add deterministic source-quality signals that are useful but not overclaimed;
7. keep CI independent of DeepSeek, network calls, SQLite, and external vector
   services.

## Recommended Narrow Wedge

Ship Phase 20 as an evidence-inspection layer, not as semantic truth scoring.

The first slice should add:

- `knowledge_evidence` metadata on assistant turns;
- normalized citation evidence with document ID, title, source label, space,
  review status, chunk ordinal, snippet, and match reason;
- an evidence summary that maps to existing `knowledge_answer_state`;
- app-side rendering for evidence, support state, and next action;
- admin-side inspection of recent evidence and direct document links;
- deterministic document/source quality indicators based on metadata and
  retrieval usage.

## In Scope

### Evidence Contract

- Add a stable answer-level evidence object for knowledge-backed responses.
- Reuse existing citation and diagnostics data rather than adding a second
  retrieval pipeline.
- Include enough fields to render evidence in `/app` and inspect it in `/admin`.
- Keep evidence deterministic in tests.

### Citation Detail

- Each citation should identify:
  - knowledge space;
  - document ID;
  - document title;
  - source label or source type;
  - review status;
  - chunk ordinal or stable chunk ID;
  - bounded snippet;
  - score or match reason if already available.
- Pending, rejected, and archived document text must not appear in normal user
  evidence.

### App Evidence Panel

- Show the answer support state in a compact, readable way.
- Show supporting sources as bounded rows, not as a wall of metadata.
- Show partial/unsupported/review-gated reasons in operator-friendly language.
- Avoid stretching the chat layout with long snippets.

### Admin Evidence Inspection

- Let operators inspect recent evidence-bearing turns.
- Link from evidence rows to the underlying knowledge document detail where
  possible.
- Surface weak or missing evidence alongside the existing knowledge-gap loop.
- Keep this local-first and file-backed.

### Source Quality Signals

- Add deterministic quality flags that help operators triage content:
  - unreviewed or review-gated;
  - missing source label;
  - very short content;
  - duplicate content hash;
  - disabled or failed lifecycle status;
  - never cited or recently cited if usage is already available.
- Do not present these as semantic correctness scores.

## Out of Scope

- LLM-as-judge grading.
- Semantic entailment verification.
- Confidence percentages that imply statistical calibration.
- Provider-required answer evaluation.
- PDF/DOCX/web crawler adapters.
- Full audit-event replay.
- Multi-user RBAC or reviewer assignment.
- External databases or vector services.
- Automatic claim extraction.
- Rewriting answers to force citation spans.

## Alternatives Considered

### A. Keep Current Citations Only

This is the lowest implementation cost and avoids changing app/admin UI.

Rejected because it leaves the trust story scattered across metadata,
diagnostics, and admin tools. Users still have to infer why the answer is
trustworthy.

### B. LLM Explainability Layer

Ask the model to explain why each answer is supported and grade its confidence.

Rejected for Phase 20 because it introduces provider dependence, can produce
plausible but unverifiable rationales, and is hard to test deterministically.

### C. Deterministic Evidence Pack

Assemble a compact evidence object from retrieval results, citations, review
state, and answer-state diagnostics.

Recommended because it improves trust while preserving the current local-first
architecture and testing model.

## Proposed Architecture

```text
knowledge retrieval pipeline
  -> returns active reviewed citations and diagnostics
  -> annotates citation evidence with bounded snippets and source metadata

persona / runtime answer path
  -> maps retrieval outcome to knowledge_answer_state
  -> emits knowledge_evidence metadata on assistant turn

/app
  -> renders evidence panel from knowledge_evidence
  -> shows source rows, support state, and next action

/admin
  -> lists recent evidence-bearing turns or diagnostics
  -> links evidence to document detail and gap workflow

knowledge quality helpers
  -> compute deterministic flags from document metadata, lifecycle, review,
     source fields, duplicate hash, and usage if available
```

## Evidence Object Draft

Stage 2 should lock exact names. The minimum contract is:

```text
knowledge_evidence:
  answer_state: grounded | partially_supported | unsupported | provider_fallback | guard_rejected | review_gated | local_mode
  space_id: string
  summary: short deterministic text
  citations:
    - document_id
      title
      source_label
      source_type
      review_status
      chunk_id
      chunk_ordinal
      snippet
      score
      match_reason
  gaps:
    - gap_id
      status
      reason
  diagnostics:
    no_source_reason
    stages_skipped
    gated_counts
```

Rules:

- snippets must be bounded;
- gated documents can contribute counts or reason codes, not raw text;
- evidence must not claim semantic truth beyond retrieved support;
- old clients should continue to work if they ignore the new metadata.

## UX Requirements

### `/app`

- Keep the chat first; evidence is secondary but easy to inspect.
- Show support state using compact labels and source rows.
- Show snippets only when they fit in bounded containers.
- For unsupported answers, show the next operator action if a gap was created.
- For review-gated answers, explain that matching knowledge needs activation.

### `/admin`

- Add a recent evidence or answer-trust view near the knowledge operations
  surface.
- Show answer state, query, source count, gap status, and linked document IDs.
- Let operators jump from evidence to document detail.
- Keep long answer text and snippets bounded.

## Acceptance Criteria

1. Knowledge-backed assistant turns include stable `knowledge_evidence`
   metadata.
2. Existing clients that only read assistant text continue to work.
3. Evidence citations include document, source, review, chunk, and snippet
   fields.
4. Snippets are bounded and deterministic.
5. Review-gated documents do not leak raw text through evidence.
6. `/app` renders grounded, partially supported, unsupported, and review-gated
   states clearly.
7. `/admin` can inspect recent evidence-bearing turns and link to source
   documents where available.
8. Unsupported or weakly supported turns still feed the existing gap workflow.
9. Deterministic quality flags are available for knowledge triage.
10. Tests do not require DeepSeek, network, SQLite, external vector services, or
    browser automation.

## Test Matrix Seed

| Area | Scenario | Expected result |
| --- | --- | --- |
| Evidence contract | Grounded answer with one active citation | Metadata includes answer state, source, review status, chunk, and snippet |
| Compatibility | Client ignores `knowledge_evidence` | Text response remains unchanged |
| Snippet bounds | Long matching chunk | Evidence snippet is truncated deterministically |
| Review gate | Only pending/rejected matches exist | Evidence has review-gated reason without raw gated text |
| Partial support | Low or limited citations | Evidence state remains partially supported with source rows |
| Unsupported | No usable source | Gap flow still records the unsupported turn |
| Admin inspection | Recent grounded turn exists | Admin view lists evidence and links to source document |
| Quality flags | Document has no source label | Deterministic flag is emitted |
| Quality flags | Duplicate content hash exists | Duplicate flag is emitted without deleting either doc |
| UI layout | Long titles/snippets | `/app` and `/admin` remain bounded and scannable |

## Open Questions For Stage 2

1. Should evidence be stored on persisted conversation turns, returned only in
   stream metadata, or both?
2. Should the first admin evidence view read from turn history or from derived
   diagnostics only?
3. What exact answer states should Phase 20 expose: reuse existing values only,
   or add `review_gated` as a first-class state?
4. Should quality flags live on document detail only, or also on space health?
5. Should citation snippets come from retrieval chunks directly or from a
   separate evidence normalizer?

## Stage 1 Recommendation

Approve Phase 20 as the Deterministic Evidence Pack.

After approval, Stage 2 should run `$gstack-autoplan` and lock:

- evidence metadata schema;
- persistence boundary;
- app/admin UI placement;
- quality flag model;
- TDD slice order;
- exact regression test matrix.
