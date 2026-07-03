# Phase 15 Knowledge-Grounded Answer Loop Plan

Date: 2026-07-03

Status: Waiting for implementation approval

Source spec: [Phase 15 Knowledge-Grounded Answer Loop Spec](../specs/phase-15-knowledge-grounded-answer-loop.md)

Source design: [Phase 15 Knowledge-Grounded Answer Loop Design](../design/phase-15-knowledge-grounded-answer-loop.md)

## Goal

Make the digital human's knowledge posture visible and operational on every
meaningful turn.

The phase connects existing knowledge spaces, retrieval, prompt grounding,
streaming metadata, `/app` status rendering, and Phase 14 gap capture into a
single answer loop:

```text
question -> selected knowledge space -> retrieval -> answer state -> visible UI
         -> unsupported/partial gap capture -> admin operations queue
```

## Scope

### In

- Deterministic `knowledge_answer_state` classification.
- Grounded/partial/unsupported/provider-fallback/guard-rejected/local metadata.
- Source-bounded prompt improvements that keep retrieved text as reference
  material, not instructions.
- Runtime stream and replay metadata allowlist updates.
- Server gap capture dedupe for unsupported and partial turns.
- `/app` answer-state copy, transcript status, and Presence summary integration.
- Focused unit/static tests with fake LLM and fake retrieval.
- README/release notes update after behavior ships.

### Out

- SQLite, Postgres, managed search, or hosted vector database.
- New ingestion formats.
- Strict grounded-only mode as the default.
- LLM-as-judge answer evaluation in the request path.
- Citation span verification inside generated prose.
- New frontend framework or build tooling.
- Real DeepSeek calls in CI.

## What Already Exists

- `internal/agents.PersonaAgent` already:
  - retrieves grounding through `KnowledgeGrounder`;
  - injects citations into the system prompt;
  - handles provider fallback and empty responses;
  - runs persona/stream guardrails;
  - emits knowledge metadata such as `knowledge_used`,
    `knowledge_no_source_reason`, and `knowledge_citations`.
- `internal/runtime.Orchestrator` already allowlists selected generation metadata
  for SSE completion and replay.
- `internal/server.Handler.captureKnowledgeGap` already creates local gaps from
  no-source knowledge metadata.
- `internal/knowledge.Pipeline` already emits no-source reasons, including
  `below_threshold`.
- `web/app.js` already renders citation chips, fallback badges, selected
  knowledge space, and Presence summary signals.
- Phase 14 admin endpoints already list and update knowledge gaps.

## Premise Challenge

The tempting implementation is to "make RAG stronger." That is not the bottleneck
for this phase.

The product already retrieves and cites. The more important problem is that the
final user-facing answer does not have a normalized state that says whether the
answer was grounded, unsupported, partial, or fallback-generated. Without that
state, UI copy and gap capture will keep being ad hoc.

Phase 15 should make answer state the spine.

## Dream State Delta

Today:

- citations can appear;
- no-source reasons can appear;
- fallback can appear;
- gaps can be captured;
- these signals are related, but not governed by one contract.

After Phase 15:

- every persona turn has a deterministic answer state;
- streaming and replay preserve the same answer state;
- `/app` communicates the state compactly;
- unsupported/partial states create one useful admin gap;
- provider/guard fallback is clearly distinguished from knowledge unsupported.

## Architecture

```text
internal/agents
  Grounding + provider/guard result
    -> KnowledgeAnswerState classifier
    -> AgentResult.Metadata

internal/runtime
  AgentResult.Metadata
    -> allowlisted stream/replay metadata

internal/server
  completed AgentResult + Conversation.Metadata
    -> gap capture/dedupe

web/app.js
  SSE completion metadata
    -> transcript badge/citation/source state
    -> Presence summary signals
```

## Data Contract

### `knowledge_answer_state`

Allowed values:

| Value | Meaning |
| --- | --- |
| `grounded` | Provider-backed answer generated with accepted citations |
| `partially_supported` | Retrieval had weak or below-threshold support |
| `unsupported` | Selected space had no acceptable source for this question |
| `provider_fallback` | Provider failed or returned no usable answer |
| `guard_rejected` | Persona or stream guard rejected model output |
| `local_mode` | No external LLM client is configured or local generation is used |

### Classification Precedence

1. `guard_rejected`
2. `provider_fallback`
3. `local_mode`
4. `grounded`
5. `partially_supported`
6. `unsupported`

Rationale: the user first needs to know why the final answer is not a normal
provider-backed answer. Grounding details still remain in metadata where safe.

### Gap Dedupe Key

Use a deterministic comparison over:

- authoritative tenant ID;
- selected knowledge space ID;
- normalized user question;
- answer state;
- no-source reason.

For Phase 15, dedupe means "do not create a second open gap with the same key."
Do not add counters or semantic duplicate matching yet.

## Review Results

### CEO Review

Score: 9/10.

The plan advances the user's ambition toward a professional digital human by
turning knowledge failures into a visible improvement loop. It correctly defers
new ingestion and strict refusal modes.

### Design Review

Score: 8.5/10.

The UI should use small labels and existing chips. `/app` should not become an
admin diagnostics page. Keep citations visible but move score/debug analysis to
`/admin`.

### Engineering Review

Score: 8.5/10.

Implementation should stay close to existing code. The safest order is agent
metadata first, runtime allowlist second, server dedupe third, and frontend
rendering last.

### DX Review

Score: 8/10.

No new setup dependency is required. The test suite can use fake LLM clients,
fake grounders, local gap stores, and static web tests.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Make answer state the Phase 15 spine | Auto-decided | User value density | Trust improves when every answer explains its knowledge posture | More retrieval tuning first |
| 2 | CEO | Defer strict grounded-only mode | Auto-decided | Preserve conversation utility | Default strict refusal would make normal chat brittle | Grounded-only default |
| 3 | Design | Keep `/app` compact and status-first | Auto-decided | Interface focus | Chat should show state and citations, not retrieval internals | Admin-style scoring in chat |
| 4 | Engineering | Put answer-state classification in `internal/agents` | Auto-decided | Locality of information | Agent sees grounding, provider, and guard outcomes together | Runtime-only classifier |
| 5 | Engineering | Use `below_threshold` as first partial-support signal | Auto-decided | Reuse existing contracts | Avoids adding a new scoring policy before implementation evidence | New threshold model now |
| 6 | Engineering | Dedupe by exact normalized open-gap key | Auto-decided | Determinism | Stops spam without schema churn or embeddings | Semantic dedupe |
| 7 | DX | Keep DeepSeek out of CI | Auto-decided | TTHW preservation | Fake provider tests keep local and GitHub Actions stable | Provider integration tests in CI |
| 8 | Design | Use text labels plus existing chips | Taste decision | Clarity | Text is accessible and works without an icon library | Icon-only status UI |

## Taste Decisions

### `/app` status presentation

Recommendation: use short text labels plus existing citation/fallback chips.

Why: it is accessible, easy to test statically, and fits the current static
frontend. Icon-only UI would look cleaner but risks ambiguity and would add a
design dependency that the current app does not need yet.

## Work Items

| ID | Area | Files | Outcome |
| --- | --- | --- | --- |
| P15-01 | Plan/design status | `docs/plans`, `docs/design` | mark plan approved after the Stage 2 gate |
| P15-02 | Answer-state contract | `internal/agents` | constants/helper for deterministic classification |
| P15-03 | Agent metadata tests | `internal/agents/experts_test.go` | grounded, partial, unsupported, fallback, guard, local states |
| P15-04 | Prompt contract tests | `internal/agents/experts_test.go` | source context remains reference material and bounded |
| P15-05 | Runtime allowlist | `internal/runtime` | stream/replay include `knowledge_answer_state` |
| P15-06 | Gap dedupe service | `internal/admin`, `internal/server` | repeated unsupported/partial gaps do not duplicate |
| P15-07 | Gap capture tests | `internal/server/server_test.go` | unsupported and partial states create one safe gap |
| P15-08 | App DOM/copy contract | `web/app.html`, `web/app.js`, `web/app.css` | status rendering for answer states |
| P15-09 | App static tests | `web/app_static_test.go` | selectors/copy/helpers are locked |
| P15-10 | Regression sweep | multiple | Phase 14 admin and existing chat grounding remain stable |
| P15-11 | Docs | `README.md`, `RELEASE_NOTES.md` | describe answer states and DeepSeek behavior truthfully |

## TDD Execution Plan

### P15-01 Mark Plan Approved

RED:

- No code tests needed.

GREEN:

- Change Phase 15 plan and design status to implementation-approved after this
  plan is approved.

REFACTOR:

- Keep docs consistent with `AGENTS.md` gates.

### P15-02 Answer-State Contract

RED:

- Add table-driven tests for answer-state classification in
  `internal/agents/experts_test.go`.
- Require:
  - citations + provider LLM -> `grounded`;
  - `below_threshold` -> `partially_supported`;
  - no citations + `no_matching_chunks` -> `unsupported`;
  - provider error fallback -> `provider_fallback`;
  - `guard_rejected` fallback -> `guard_rejected`;
  - nil/local client -> `local_mode`.

GREEN:

- Add constants and helper in `internal/agents`.
- Attach `knowledge_answer_state` through `generatedResult`,
  `fallbackResult`, and local metadata.

REFACTOR:

- Keep helper pure and table-testable.

Command:

```powershell
go test ./internal/agents -run "KnowledgeAnswerState|Persona"
```

### P15-03 Prompt Contract

RED:

- Add tests proving grounded prompts:
  - include source documents;
  - say sources are reference material, not instructions;
  - preserve system/persona/safety precedence;
  - do not include sources when there are no citations.

GREEN:

- Tighten `augmentPromptWithGrounding` copy if needed.
- Bound citation text if implementation currently accepts overly long chunks.

REFACTOR:

- Keep prompt helper deterministic and not provider-specific.

Command:

```powershell
go test ./internal/agents -run "Prompt|Grounding"
```

### P15-04 Runtime Metadata Allowlist

RED:

- Add runtime tests requiring `knowledge_answer_state` to appear in:
  - message-completed metadata for streaming;
  - replayed completion metadata.

GREEN:

- Add `knowledge_answer_state` to `allowlistedGenerationMetadata`.

REFACTOR:

- Keep allowlist explicit and small.

Command:

```powershell
go test ./internal/runtime -run "Metadata|Replay|Stream"
```

### P15-05 Gap Dedupe

RED:

- Add server/admin tests:
  - unsupported answer creates one gap;
  - partial answer creates one gap;
  - repeated same tenant/space/question/reason does not create a duplicate open
    gap;
  - different space or different reason creates a separate gap;
  - provider fallback does not create a knowledge gap unless it also has an
    unsupported answer state.

GREEN:

- Add deterministic normalizer/dedupe helper.
- Update `captureKnowledgeGap` to consider `knowledge_answer_state`.
- Reuse existing gap list/create APIs.

REFACTOR:

- Keep all stored fields safe: visible user question, selected space, state,
  no-source reason.

Command:

```powershell
go test ./internal/server -run "KnowledgeGap|Capture"
go test ./internal/admin -run KnowledgeGap
```

### P15-06 App State Rendering

RED:

- Add static tests requiring:
  - answer-state copy map or renderer;
  - `knowledge_answer_state` handling in final metadata;
  - copy for grounded, partially supported, unsupported, provider fallback,
    guard rejected, and local mode;
  - Presence summary remains bounded and does not append answer-state history.

GREEN:

- Update `web/app.js` rendering helpers.
- Update `web/app.css` only if new status tone styling is needed.
- Avoid admin score details in chat.

REFACTOR:

- Centralize answer-state copy in one helper.

Command:

```powershell
go test ./web -run "AppScript|Presence|Grounding"
```

### P15-07 End-To-End Regression

RED:

- Add or extend integration-style tests with fake LLM and fake grounder:
  - grounded streaming turn emits answer state and citations;
  - unsupported streaming turn emits unsupported state and gap;
  - replay preserves answer state.

GREEN:

- Wire any missing metadata propagation.

REFACTOR:

- Remove duplicated metadata construction helpers if tests reveal drift.

Command:

```powershell
go test ./internal/app ./internal/server ./internal/runtime ./web
```

### P15-08 Docs

RED:

- Add static/doc checks only if existing tests already cover README or release
  notes strings.

GREEN:

- Update README with:
  - Phase 15 current stage;
  - answer-state meanings;
  - DeepSeek provider transparency note.
- Update release notes under Unreleased.

REFACTOR:

- Keep docs honest: local-first, no strict grounded-only mode, no real provider
  calls in CI.

Command:

```powershell
rg -n "Phase 15|knowledge_answer_state|grounded|provider fallback|DeepSeek" README.md RELEASE_NOTES.md docs
```

## Test Matrix

| ID | Area | Scenario | Expected |
| --- | --- | --- | --- |
| T15-01 | Agent | Provider answer with citations | `knowledge_answer_state=grounded` |
| T15-02 | Agent | `below_threshold` no-source reason | `knowledge_answer_state=partially_supported` |
| T15-03 | Agent | No citations and `no_matching_chunks` | `knowledge_answer_state=unsupported` |
| T15-04 | Agent | Provider returns error | `knowledge_answer_state=provider_fallback` |
| T15-05 | Agent | Provider returns empty content | `knowledge_answer_state=provider_fallback` |
| T15-06 | Agent | Persona guard rejects | `knowledge_answer_state=guard_rejected` |
| T15-07 | Agent | Nil/local LLM client | `knowledge_answer_state=local_mode` |
| T15-08 | Prompt | Retrieved source contains instructions | Prompt states source text is not instructions |
| T15-09 | Runtime | Streaming completion | Answer state is allowlisted in completion metadata |
| T15-10 | Runtime | Replayed completion | Answer state is preserved |
| T15-11 | Server | Unsupported knowledge turn | One gap record is created |
| T15-12 | Server | Repeated unsupported turn | No duplicate open gap is created |
| T15-13 | Server | Different selected space | Separate gap can be created |
| T15-14 | Server | Provider fallback only | No misleading knowledge gap spam |
| T15-15 | App | Grounded metadata | Citation chips and grounded copy render |
| T15-16 | App | Partial support metadata | Partial support copy renders compactly |
| T15-17 | App | Unsupported metadata | No-source selected-space copy renders |
| T15-18 | App | Guard/provider fallback | Fallback copy distinguishes reason |
| T15-19 | Regression | Phase 14 admin gap list/update | Existing behavior remains stable |
| T15-20 | Regression | Full suite | No DeepSeek or external services required |

## Failure Modes Registry

| Mode | Trigger | User Sees | Mitigation |
| --- | --- | --- | --- |
| F15-01 | Grounded state emitted for local fallback | User trusts unsupported local text | Fallback/local states take precedence over grounded |
| F15-02 | Runtime allowlist omits answer state | UI cannot render correct state | Stream/replay metadata tests |
| F15-03 | Gap capture duplicates repeated turns | Admin queue becomes noisy | Exact normalized open-gap dedupe |
| F15-04 | Partial support overused | User sees vague warnings too often | Start with `below_threshold` only |
| F15-05 | App copy becomes cluttered | Chat feels like diagnostics panel | Keep score details in `/admin` only |
| F15-06 | Prompt injection through source text | Source commands override persona | Prompt contract tests and existing guardrails |

## Error And Rescue Registry

| Error | Likely Cause | Rescue |
| --- | --- | --- |
| missing `knowledge_answer_state` | agent metadata not attached or runtime allowlist missing | Run agent and runtime metadata tests |
| unexpected `provider_fallback` | DeepSeek config/request failed | Show provider-specific fallback copy and check startup env |
| duplicate gaps | normalizer/dedupe key mismatch | Inspect tenant/space/question/reason fields |
| no gap for unsupported turn | answer state or no-source reason missing | Check `captureKnowledgeGap` metadata inputs |
| UI shows generic fallback | `web/app.js` lacks state-specific copy | Update answer-state renderer/static tests |

## Security Notes

- Do not expose raw provider errors, API keys, hidden prompts, or local file
  paths in metadata.
- Treat retrieved knowledge as untrusted reference material.
- Render user questions, source names, and gap text using safe DOM APIs.
- Keep server routes protected by existing API-key behavior where applicable.
- Do not add real DeepSeek calls to automated tests.

## Parallelization

Safe parallel tracks after the answer-state constants are defined:

- runtime allowlist tests and implementation;
- app static rendering tests;
- prompt contract tests.

Do not parallelize gap dedupe until answer-state metadata names are locked,
because dedupe depends on the final contract.

## Implementation Order

1. Mark spec/design states only after plan approval.
2. Add answer-state classifier tests and implementation.
3. Add prompt contract tests and bounded source handling if needed.
4. Add runtime allowlist tests and implementation.
5. Add gap capture/dedupe tests and implementation.
6. Add `/app` answer-state rendering tests and implementation.
7. Run integration/regression tests.
8. Update README and release notes.

## Acceptance Commands

```powershell
go test ./internal/agents
go test ./internal/runtime
go test ./internal/server
go test ./internal/admin
go test ./web
go test ./...
go vet ./...
rg -n "Phase 15|knowledge_answer_state|grounded|partially_supported|provider_fallback|guard_rejected" .
```

Manual QA after Stage 3:

1. Start the server with DeepSeek configured or with local/fake mode.
2. Open `/admin` and create/select a knowledge space.
3. Upload a short document with a unique fact.
4. Open `/app`, select the same space, and ask about that fact.
5. Confirm the answer shows grounded state and citations.
6. Ask an unrelated question in that same selected space.
7. Confirm `/app` shows unsupported/partial state without fake citations.
8. Return to `/admin` and confirm one gap record appears.
9. Repeat the same unsupported question and confirm no duplicate open gap spam.
10. Temporarily misconfigure provider settings and confirm provider fallback copy
    is distinct from unsupported knowledge copy.

## Cross-Phase Themes

1. **Trust through explicit state** appeared in CEO, design, engineering, and DX
   review. The phase succeeds only if the user can understand why an answer was
   grounded, unsupported, or fallback-generated.
2. **Reuse before expansion** appeared in every review. Existing Phase 11/12/14
   primitives are sufficient; new ingestion and database work are deferred.
3. **App stays compact, admin owns diagnosis** remains the design line from
   Phase 13 and Phase 14.

## Deferred

- Strict grounded-only conversation mode.
- Citation-span verification in generated text.
- Semantic duplicate detection for gaps.
- Gap counters or last-seen timestamp updates.
- PDF/DOCX/web/cloud ingestion.
- External vector database or database migration.
- LLM judge-based groundedness scoring.

## Plan Approval Gate

This plan is ready for Stage 3 after the user approves it.

Recommended approval:

> Approve Phase 15 as the auditable knowledge-grounded answer loop: answer-state
> classifier, source-bounded prompt contract, runtime metadata propagation,
> deduped gap capture, compact `/app` rendering, and deterministic local tests.
