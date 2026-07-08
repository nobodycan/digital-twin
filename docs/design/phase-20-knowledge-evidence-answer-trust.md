# Phase 20 Knowledge Evidence and Answer Trust Design

Date: 2026-07-04

Status: Draft; waiting for spec approval

Source spec: [Phase 20 Knowledge Evidence and Answer Trust Spec](../specs/phase-20-knowledge-evidence-answer-trust.md)

## Office-Hours Summary

Phase 19 made the knowledge base safer by gating retrieval on reviewed active
sources. Phase 20 should make answers easier to trust by showing the evidence
behind them.

The product should not claim that it knows what is true. It should show what it
used.

The narrow design is:

```text
retrieved source chunks -> normalized evidence pack -> app evidence panel
                                           |
                                           v
                                  admin evidence inspection
```

## Executive Decision

Use a **Deterministic Evidence Pack**.

Do not add an LLM judge or confidence score in this phase. The system already
has enough deterministic signals: retrieved chunks, document metadata, review
status, answer state, diagnostics, and knowledge gaps. Phase 20 should assemble
those into a stable contract and a better UI.

## Product Shape

The user-facing shape is:

- `/app` shows a compact evidence panel under knowledge-backed answers;
- each source row names the document, source, review state, and snippet;
- unsupported and review-gated answers show an actionable reason;
- `/admin` exposes recent answer evidence and links to document detail;
- knowledge quality flags help operators improve weak sources.

This turns the current knowledge system from "it cites something" into "I can
inspect what it used and improve it."

## Premises

### Premise 1: Trust comes from inspectability, not decoration

A professional digital human does not need confidence theater. It needs
traceable evidence that a user can inspect.

### Premise 2: Retrieval output is the source of truth

Evidence should be assembled from the retrieval pipeline and answer-state
metadata. A second explanation path would create drift.

### Premise 3: Review gates must remain private by default

Pending, rejected, and archived documents can influence diagnostics at a reason
code or count level, but their raw text must not leak into user evidence.

### Premise 4: Quality signals are triage hints

Document quality flags should help an operator decide what to fix. They are not
truth scores and should not imply semantic correctness.

## Alternatives

### Approach A: UI-Only Citation Polish

Improve how current citations render without changing runtime metadata.

Pros:

- smallest implementation;
- low backend risk;
- useful visual improvement.

Cons:

- leaves admin inspection weak;
- cannot reliably explain review-gated or unsupported cases;
- keeps trust behavior spread across several metadata fields.

Verdict: insufficient.

### Approach B: LLM Judge and Confidence

Use a model to grade answer support, explain reasoning, and produce confidence.

Pros:

- richer language;
- potentially more nuanced;
- attractive demo.

Cons:

- provider-dependent;
- not deterministic in CI;
- risks inventing rationales;
- creates a stronger trust claim than the system can currently prove.

Verdict: defer.

### Approach C: Deterministic Evidence Pack

Normalize the evidence already produced by retrieval, citations, review state,
and diagnostics.

Pros:

- deterministic and testable;
- grounded in existing data;
- improves `/app` and `/admin`;
- creates a clean future extension point for semantic checks.

Cons:

- less magical than LLM explanation;
- Stage 2 must carefully decide what is persisted versus streamed.

Verdict: recommended.

## Recommended Architecture

```text
internal/knowledge
  CitationEvidence
  EvidenceSummary
  snippet normalization
  review-gate-safe diagnostics

internal/agents or runtime answer path
  assemble knowledge_evidence metadata
  preserve existing assistant text contract
  attach evidence to persisted turn if Stage 2 approves persistence

internal/admin
  source quality flags
  recent evidence query if persisted turns are used
  document-detail quality projection

internal/server
  expose evidence metadata in existing stream/final response
  optional admin endpoint for recent evidence inspection

web/app
  evidence panel below assistant turns
  support-state badge and bounded source rows

web/admin
  recent evidence / answer trust view
  quality flags on document detail or health view
```

## Evidence Contract

Stage 2 should lock exact Go types, but the design should converge on these
concepts:

```go
type KnowledgeEvidence struct {
    AnswerState string
    SpaceID     string
    Summary     string
    Citations   []CitationEvidence
    Gaps        []EvidenceGap
    Diagnostics EvidenceDiagnostics
}

type CitationEvidence struct {
    DocumentID   string
    Title        string
    SourceLabel  string
    SourceType   string
    ReviewStatus string
    ChunkID      string
    ChunkOrdinal int
    Snippet      string
    Score        float64
    MatchReason  string
}
```

Rules:

- snippet generation is deterministic;
- snippets have a hard maximum length;
- missing source labels render as an explicit quality flag;
- unknown review state is treated conservatively;
- gated document content is never copied into evidence.

## Persistence Decision

Stage 2 should choose one of three paths:

1. stream-only evidence;
2. persisted evidence on assistant turns;
3. persisted summary plus recomputed detail.

Recommended first implementation:

- persist evidence metadata with assistant turns if the existing turn store can
  accept additive metadata safely;
- otherwise return evidence in stream/final response and keep admin evidence
  inspection limited to current diagnostics.

The stronger product is persisted evidence because `/admin` can inspect recent
answers. But persistence should not destabilize the conversation store.

## App UX Design

### Assistant Turn

For each knowledge-backed answer:

- show the answer text first;
- show a compact support badge;
- show expandable evidence rows if citations exist;
- show source title, source label, review status, and bounded snippet;
- show a "needs knowledge" or "needs review" reason when unsupported or
  review-gated.

### Layout Guardrails

- evidence rows have fixed maximum height;
- long titles and snippets wrap or clamp;
- no raw JSON in user-facing panels;
- no large diagnostics wall in chat;
- support state should be legible without requiring operator expertise.

## Admin UX Design

### Answer Trust View

Add a compact operator view near existing knowledge operations:

- recent query or turn summary;
- answer state;
- source count;
- gap status;
- review-gated reason if present;
- linked source documents.

### Document Quality Flags

Show deterministic flags in document detail and optionally health summary:

- `missing_source_label`;
- `short_content`;
- `duplicate_hash`;
- `review_pending`;
- `review_rejected`;
- `lifecycle_not_ready`;
- `never_cited` if usage data is available.

These flags should read as operational hints, not correctness grades.

## Retrieval and Diagnostics Design

Phase 20 should not change ranking semantics unless Stage 2 finds a small bug.

The flow should be:

1. retrieve active reviewed citations as Phase 19 already does;
2. normalize citation evidence;
3. derive answer support state from existing answer-state logic;
4. attach review-gate-safe diagnostics;
5. emit or persist `knowledge_evidence`.

Diagnostic examples:

```text
grounded: citations present and answer state grounded
partially_supported: limited citations or answer-state partial
unsupported: no usable citations and gap created or available
review_gated: matching ready docs exist but review state excluded them
provider_fallback: provider failed and local fallback was used
```

## Security and Abuse Notes

- Do not expose raw pending, rejected, or archived source text.
- Do not let evidence metadata include API keys, file paths outside source
  labels, or local absolute paths unless already intentionally exposed.
- Do not claim confidence percentages.
- Do not ask the LLM to explain hidden chain-of-thought.
- Do not let review-gated diagnostics reveal sensitive source content.

## Test Strategy

Stage 3 should implement with Superpowers TDD:

1. RED: evidence normalizer emits citation details for active reviewed docs.
2. GREEN: build citation evidence from existing retrieval results.
3. RED: long snippets are bounded deterministically.
4. GREEN: add snippet normalization helper.
5. RED: review-gated diagnostics do not include raw gated content.
6. GREEN: evidence diagnostics use reason codes and counts.
7. RED: assistant turn metadata includes `knowledge_evidence`.
8. GREEN: wire evidence through answer path without changing text contract.
9. RED: quality flags identify missing source labels and short content.
10. GREEN: add deterministic quality helper.
11. RED: `/app` static tests assert evidence panel selectors and bounded UI.
12. GREEN: render evidence panel.
13. RED: `/admin` static/server tests assert evidence or quality inspection.
14. GREEN: render admin trust/quality view.

No tests should require DeepSeek, network, SQLite, browser automation, or
external vector services.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Evidence overclaims truth | User trusts unsupported claims | Avoid confidence percentages and LLM rationales |
| Gated source text leaks | Trust/security issue | Reason codes and counts only for gated docs |
| UI becomes noisy | Lower usability | Compact badges, expandable rows, bounded snippets |
| Metadata breaks old clients | Regression | Additive metadata only; text response unchanged |
| Admin view needs unavailable history | Incomplete feature | Stage 2 decides persistence boundary before build |
| Quality flags feel punitive | Operator confusion | Label as triage hints, not scores |

## Stage 2 Decisions

Autoplan should lock:

1. exact evidence Go types and JSON names;
2. persistence boundary for evidence metadata;
3. whether `review_gated` becomes a first-class answer state;
4. source-quality flag names and placement;
5. app evidence panel layout;
6. admin evidence inspection endpoint or derived view;
7. TDD slice order and browser QA expectations.

## Office-Hours Handoff

Recommended approval:

> Approve Phase 20 as a Deterministic Evidence Pack that turns retrieved
> citations, review state, diagnostics, and gaps into inspectable answer
> evidence across `/app` and `/admin`, without adding LLM judging or confidence
> theater.

After approval, run Stage 2 with `$gstack-autoplan` before any implementation.
