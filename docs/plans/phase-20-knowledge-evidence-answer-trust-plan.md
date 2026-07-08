# Phase 20 Knowledge Evidence and Answer Trust Plan

Date: 2026-07-04

Status: Draft; awaiting user approval

Mode: SDD Stage 2 / gstack autoplan

Source spec: [Phase 20 Knowledge Evidence and Answer Trust Spec](../specs/phase-20-knowledge-evidence-answer-trust.md)

Source design: [Phase 20 Knowledge Evidence and Answer Trust Design](../design/phase-20-knowledge-evidence-answer-trust.md)

## Plan Summary

Phase 20 turns the existing knowledge-grounded answer loop into an inspectable
trust experience. The system already knows answer state, citations, retrieval
diagnostics, review status, and knowledge gaps. The plan is to normalize those
signals into a deterministic `knowledge_evidence` object, render it in `/app`,
surface answer-trust inspection in `/admin`, and add deterministic quality flags
without adding an LLM judge or confidence theater.

## What Already Exists

- `internal/knowledge/pipeline.go` produces retrieval diagnostics, source
  reasons, stages, citations, and review-gated no-source behavior.
- `internal/app/bootstrap.go` adapts knowledge pipeline output into agent
  grounding citations and metadata.
- `internal/agents` emits `knowledge_answer_state`, `knowledge_citations`,
  no-source metadata, provider fallback, guard rejection, and local-mode states.
- `internal/runtime/orchestrator.go` allowlists knowledge metadata into streaming
  completion events and replayed turns.
- `pkg/types/contracts.go` already supports additive `Metadata` on messages,
  turns, attempts, stream events, and conversations.
- `internal/conversation` persists completed turn results, so additive assistant
  metadata can survive replay if passed through the runtime result.
- `web/app.js` already renders citation summaries and answer state from
  metadata.
- `web/admin.js` already renders knowledge health, document detail, quality
  flags, retrieval diagnostics, gaps, imports, and review queue.
- `web/app_static_test.go` already checks frontend selectors and knowledge
  metadata wiring.

## NOT In Scope

- LLM-as-judge or semantic entailment scoring.
- Numeric confidence percentages.
- Chain-of-thought or hidden reasoning display.
- PDF/DOCX/crawler ingestion.
- External vector services, SQLite, or cloud storage.
- RBAC, reviewer assignment, or multi-user audit history.
- Ranking algorithm rewrite.
- Automatic answer rewriting based on citation spans.

## Autoplan Review Summary

### CEO Review

Score: 8.5/10

Verdict: build answer trust now, but keep it deterministic.

Phase 19 solved source activation. The next product risk is that a professional
digital human may still feel opaque: it can answer and cite, but the user cannot
quickly inspect what support exists, what was gated, and what action closes the
gap. This is the right time to make evidence visible because the knowledge
system has enough trustworthy primitives.

CEO challenge resolved: do not sell "truth." Sell "inspectable support." The
phase should make the answer's evidence posture obvious without pretending the
system performed semantic proof.

### Design Review

Score: 8/10

Verdict: evidence should be compact, secondary, and operator-actionable.

The `/app` chat must remain the primary experience. Evidence belongs beneath an
assistant turn as a bounded, expandable source panel: support state, source
count, document rows, snippets, and next action. The `/admin` view should not
become another dashboard wall; it should connect recent answer evidence to
document detail, quality flags, and gaps.

Design risk: long snippets and titles can stretch the interface. The plan
requires clamped rows, bounded snippets, and no raw JSON in user-facing panels.

### Engineering Review

Score: 8.5/10

Verdict: introduce typed evidence helpers, then pass additive metadata through
the existing runtime.

Observed implementation facts:

- metadata is already the compatibility layer;
- turn persistence can retain result metadata;
- runtime streaming uses an allowlist, so `knowledge_evidence` must be added
  deliberately;
- citations already exist, but need richer normalization;
- review-gated diagnostics already exist, but must not leak gated text;
- admin document quality logic already exists and can be extended with new
  flags rather than replaced.

Engineering choice: add deterministic evidence structs/helpers near the
knowledge or agent boundary, but expose them as JSON-compatible metadata. Do
not add a second retrieval path.

### DX Review

Score: 8/10

Verdict: use boring names, additive metadata, and copy-paste verification.

The feature should be discoverable through:

- `knowledge_evidence` metadata;
- `knowledge_evidence.citations`;
- `knowledge_evidence.answer_state`;
- `knowledge_evidence.diagnostics`;
- quality flag names that match existing admin style.

Developer friction stays low if tests remain local and if the README shows one
copy-paste flow to create an answer, inspect evidence in `/app`, and inspect the
same source in `/admin`.

## Locked Decisions

| # | Decision | Result | Rationale |
| --- | --- | --- | --- |
| 1 | Scope | Deterministic Evidence Pack | Improves trust without adding provider-dependent judging. |
| 2 | Evidence source | Reuse retrieval, citations, diagnostics, review state, and gaps | Avoids drift between answer and explanation. |
| 3 | Metadata name | `knowledge_evidence` | Matches existing `knowledge_*` metadata convention. |
| 4 | Compatibility | Additive metadata only | Existing text-only clients continue to work. |
| 5 | Persistence | Persist evidence when it rides existing assistant result metadata | Enables replay/admin inspection without a new store. |
| 6 | Admin first slice | Recent answer trust from persisted turn metadata plus document quality flags | Useful without building audit replay. |
| 7 | App UX | Compact evidence panel under assistant turns | Keeps chat primary and evidence inspectable. |
| 8 | Review-gated handling | Reason codes/counts only, no raw gated text | Preserves Phase 19 trust boundary. |
| 9 | Answer states | Reuse existing states and add `review_gated` only if metadata already has review-gate reason | Avoids broad state-machine churn. |
| 10 | Quality signals | Deterministic flags, not semantic scores | Helps operators triage without overclaiming truth. |

## Final Scope

### In Scope

- Add a typed `KnowledgeEvidence` projection that serializes into
  `knowledge_evidence`.
- Normalize citation evidence with:
  - document ID;
  - title;
  - space ID/name where available;
  - source label;
  - source type;
  - review status;
  - chunk ID or ordinal;
  - bounded snippet;
  - score or match reason.
- Add snippet normalization with deterministic maximum length.
- Add review-gate-safe diagnostics inside evidence.
- Add runtime streaming allowlist support for `knowledge_evidence`.
- Preserve replay behavior so completed turns can replay evidence metadata.
- Render evidence in `/app` below assistant turns.
- Add admin answer-trust inspection or recent-evidence projection using persisted
  turn metadata where available.
- Add or extend deterministic quality flags for:
  - `missing_source_label`;
  - `short_content`;
  - `duplicate_hash`;
  - `review_pending`;
  - `review_rejected`;
  - `review_archived`;
  - `lifecycle_not_ready`.
- Update README and release notes after implementation with true shipped
  behavior.

### Out of Scope

- Provider-required evaluation.
- LLM-generated evidence explanations.
- Confidence percentages.
- Claim extraction.
- New database or audit-event store.
- New ingestion adapters.
- Ranking rewrite.
- Bulk document remediation flows.

## Architecture

```text
internal/knowledge
  EvidenceFromGrounding / citation normalization
  bounded snippet helper
  review-gate-safe diagnostics

internal/agents
  attach knowledge_evidence to persona result metadata
  keep existing knowledge_answer_state and knowledge_citations

internal/runtime
  allowlist knowledge_evidence for stream/replay metadata
  keep result text and existing metadata compatibility

internal/admin
  deterministic document quality flags
  optional recent evidence projection from conversation store

internal/server
  expose admin recent evidence endpoint if persistence boundary is available
  no provider calls in evidence endpoints

web/app.html / web/app.js / web/app.css
  bounded evidence panel under assistant turns
  support badge, source rows, snippets, next action

web/admin.html / web/admin.js
  answer-trust or recent evidence surface
  document links and quality flags
```

## Data Contract

Recommended JSON metadata:

```json
{
  "knowledge_evidence": {
    "answer_state": "grounded",
    "space_id": "default",
    "space_name": "Default",
    "summary": "Grounded by 2 reviewed sources.",
    "citations": [
      {
        "document_id": "doc-123",
        "title": "Support Playbook",
        "source_label": "support.md",
        "source_type": "local_text_file",
        "review_status": "active",
        "chunk_id": "doc-123:0",
        "chunk_ordinal": 0,
        "snippet": "Bounded source text...",
        "score": 0.82,
        "match_reason": "lexical"
      }
    ],
    "diagnostics": {
      "no_source_reason": "",
      "stages_skipped": [],
      "review_gated_count": 0
    },
    "gaps": []
  }
}
```

Compatibility rules:

- keep existing `knowledge_answer_state`;
- keep existing `knowledge_citations`;
- add `knowledge_evidence` without removing old keys;
- old clients can ignore `knowledge_evidence`;
- replayed stream completion events include the same allowlisted evidence.

## Error And Rescue Registry

| Error | Cause | User-visible rescue |
| --- | --- | --- |
| `knowledge_evidence_unavailable` | Evidence could not be assembled from metadata | Show existing answer state/citations and log cause in diagnostics |
| `knowledge_evidence_empty` | No citations and no diagnostics/gap reason | Show "No supporting evidence recorded" without blocking answer |
| `knowledge_evidence_gated` | Matching docs exist but review gate excluded them | Point operator to Review queue |
| `knowledge_recent_evidence_failed` | Admin endpoint cannot read turn history | Keep admin page usable and show problem/cause/fix |
| `quality_flags_unavailable` | Document detail missing optional fields | Render known flags and omit unknown ones |

## Failure Modes Registry

| Failure mode | Severity | Detection | Mitigation |
| --- | --- | --- | --- |
| Gated document text leaks in evidence | Critical | evidence unit tests and stream metadata tests | Reason codes/counts only for gated docs |
| Evidence metadata breaks stream replay | High | runtime replay tests | Additive allowlist and replay assertions |
| App panel stretches on long snippets | Medium | static tests and browser QA | Hard snippet length and CSS max heights |
| Evidence says grounded when no citation exists | High | agent/knowledge tests | Derive summary from answer state plus citations |
| Admin cannot link evidence to document | Medium | server/admin tests | Gracefully render unlinked evidence rows |
| Quality flags look like truth score | Medium | UI copy/static tests | Use operational flag labels only |
| Existing citation chips disappear | Medium | web regression tests | Keep old citation summary rendering while adding panel |

## Implementation Tasks

Use Superpowers TDD in Stage 3. Each production slice starts with a failing
test.

| ID | Slice | Files | Tests first |
| --- | --- | --- | --- |
| P20-01 | Evidence model and snippet helper | `internal/knowledge`, tests | Long snippets truncate deterministically; empty snippets stay empty |
| P20-02 | Citation evidence normalization | `internal/knowledge`, `internal/app`, tests | Active citation becomes evidence with doc/source/review/chunk fields |
| P20-03 | Review-gate-safe diagnostics | `internal/knowledge`, tests | Review-gated result includes reason/count but no raw gated content |
| P20-04 | Agent metadata wiring | `internal/agents`, tests | `knowledge_evidence` appears beside existing answer-state metadata |
| P20-05 | Runtime stream/replay allowlist | `internal/runtime`, tests | completion and replay events include evidence metadata |
| P20-06 | Conversation persistence regression | `internal/conversation` or `internal/app`, tests | completed turn retains evidence metadata after rebuild/replay |
| P20-07 | Document quality flags | `internal/admin`, tests | missing source label, short content, duplicate hash, review/lifecycle flags emit deterministically |
| P20-08 | Admin recent evidence endpoint/projection | `internal/server`, tests | recent evidence lists answer state/source count and handles no history |
| P20-09 | `/app` evidence panel | `web/app.html`, `web/app.js`, `web/app.css`, tests | selectors render support state, source rows, and bounded snippets |
| P20-10 | `/admin` trust/quality UI | `web/admin.html`, `web/admin.js`, tests | selectors render recent evidence and quality flags |
| P20-11 | Docs sync | `README.md`, `RELEASE_NOTES.md`, docs | `rg` verifies Phase 20 true capability wording |

## Test Plan

Run focused tests after each slice:

```powershell
go test ./internal/knowledge
go test ./internal/agents
go test ./internal/runtime
go test ./internal/app
go test ./internal/admin
go test ./internal/server
go test ./web
```

Run full verification before Stage 4 review:

```powershell
gofmt -w .\internal\knowledge .\internal\agents .\internal\runtime .\internal\app .\internal\admin .\internal\server
go test ./...
go vet ./...
go test -cover ./...
```

Run race/lint where local environment supports them:

```powershell
go test -race ./...
golangci-lint run ./...
```

Documentation verification:

```powershell
rg -n "Phase 20|Knowledge Evidence|knowledge_evidence|answer trust|review_gated" README.md RELEASE_NOTES.md docs internal web
```

Manual QA after implementation:

```powershell
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
```

Then:

1. import a small `.md` source in `/admin`;
2. approve it in Review queue;
3. ask a source-backed question in `/app`;
4. confirm the answer shows an evidence panel with source rows and snippets;
5. archive/reject the source;
6. ask again and confirm review-gated evidence does not expose raw source text;
7. inspect `/admin` answer-trust or recent evidence view and jump to document
   detail.

## Review Scores

| Review | Score | Summary |
| --- | --- | --- |
| CEO | 8.5/10 | Right next trust layer; avoid semantic truth claims. |
| Design | 8/10 | Useful if compact, bounded, and secondary to chat. |
| Engineering | 8.5/10 | Existing metadata/runtime/store paths can carry additive evidence. |
| DX | 8/10 | Boring names and local tests keep the contributor path simple. |

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Build Deterministic Evidence Pack | Auto-decided | Trust by inspectability | Users need to inspect support without overclaimed truth. | Stop at citations; LLM judge |
| 2 | CEO | Avoid confidence percentages | Auto-decided | Do not overclaim | The system is not statistically calibrated. | Numeric confidence |
| 3 | Design | Evidence panel stays compact under assistant turns | Auto-decided | Chat remains primary | Evidence should help, not dominate the conversation. | Separate full-page answer inspector |
| 4 | Design | Use bounded snippets and rows | Auto-decided | Layout stability | Prevents long source text from stretching UI. | Raw diagnostics wall |
| 5 | Engineering | Reuse existing metadata path | Auto-decided | Additive compatibility | Existing clients and replay can keep working. | New response envelope |
| 6 | Engineering | Add runtime allowlist entry for `knowledge_evidence` | Auto-decided | Explicit trust boundary | Streaming metadata is intentionally filtered. | Pass all metadata blindly |
| 7 | Engineering | Do not rewrite retrieval ranking | Auto-decided | Preserve working behavior | Phase 20 is inspection, not retrieval quality. | Ranking refactor |
| 8 | Security | Gated content never appears in evidence | Auto-decided | Preserve review gate | Pending/rejected text should not leak. | Include gated snippets for debugging |
| 9 | DX | Keep old citation metadata | Auto-decided | Backward compatibility | Existing UI/tests/tools may rely on old keys. | Replace `knowledge_citations` |
| 10 | DX | Local deterministic tests only | Auto-decided | Fast feedback | CI must not require DeepSeek or network. | Provider-backed test suite |

## Cross-Phase Themes

**Theme: trust without overclaiming** - CEO, engineering, design, and DX all
point to the same boundary: show evidence, do not claim proof.

**Theme: additive compatibility** - engineering and DX both require old
metadata, text responses, and replay paths to keep working.

**Theme: bounded UI** - design and product both flag long snippets/diagnostics
as the main experience risk.

## Deferred

- LLM-as-judge answer grading.
- Citation-span entailment verification.
- Full answer audit timeline.
- Bulk source remediation workflow.
- Confidence calibration.
- Cross-document source graph visualization.
- External document adapters.

## Final Approval Gate

Approve this plan to enter Stage 3 BUILD with Superpowers TDD.

Recommended approval:

> Approve Phase 20 as planned: deterministic `knowledge_evidence`, richer
> citation evidence, review-gate-safe diagnostics, app evidence panel, admin
> answer-trust inspection, document quality flags, and local deterministic
> tests.
