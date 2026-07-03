# Phase 15 Knowledge-Grounded Answer Loop Design

Date: 2026-07-03

Status: Autoplan review complete, waiting for implementation approval

Source spec: [Phase 15 Knowledge-Grounded Answer Loop Spec](../specs/phase-15-knowledge-grounded-answer-loop.md)

## Office-Hours Summary

Phase 15 should not be another generic RAG improvement pass. The system already
has spaces, retrieval, citations, operations diagnostics, gap records, and an
OpenAI-compatible DeepSeek provider boundary.

The next valuable move is to make every answer's knowledge posture explicit:

- did the assistant answer from the selected space;
- did it have enough evidence;
- did the provider fail;
- did guardrails reject the output;
- did the unsupported turn become an operator-visible gap.

This keeps the product moving from "chat with uploaded files" toward a
professional digital human that can be trusted, debugged, and improved.

## Premises

1. Knowledge spaces and scoped retrieval already exist and should be reused.
2. Phase 14 gap records already exist and should remain the operations queue.
3. The current UI already has compact source/fallback rendering hooks.
4. DeepSeek must stay optional in CI; fake clients and deterministic local tests
   remain the verification path.
5. The answer loop should improve trust without making the assistant refuse every
   unsupported general conversation.

## Narrowest Valuable Wedge

Add a deterministic `knowledge_answer_state` that flows from persona generation
through runtime streaming metadata into `/app` and gap capture.

That single spine lets the product explain:

- `grounded`: retrieved citations supported the answer;
- `partially_supported`: retrieval found weak or below-threshold support;
- `unsupported`: selected knowledge space could not support the question;
- `provider_fallback`: configured provider failed or returned no usable answer;
- `guard_rejected`: persona/stream guard rejected model output;
- `local_mode`: no external LLM is configured.

The wedge is intentionally metadata-first. It makes the answer loop auditable
before adding strict grounded-only modes, citation-span verification, or new
ingestion systems.

## Recommended Architecture

```text
web/app selected space
  -> server TurnRequest.Conversation.Metadata
  -> runtime.Orchestrator
  -> agents.PersonaAgent
      -> knowledge.Service.Ground
      -> answerstate classifier
      -> source-bounded prompt
      -> provider/guard fallback classifier
  -> runtime metadata allowlist
  -> server gap capture
  -> web/app compact answer status
  -> admin gap queue
```

## Component Design

### `internal/agents`

Responsibilities:

- own answer-state classification for persona generation results;
- attach `knowledge_answer_state` to every persona result;
- preserve existing provider/fallback metadata;
- strengthen `augmentPromptWithGrounding` without growing a new prompt engine.

Recommended additions:

- `KnowledgeAnswerState` constants near `Grounding`;
- `classifyKnowledgeAnswerState(generationMode, fallbackCategory string, grounding Grounding) string`;
- prompt helper tests that assert source text is reference material and not
  instruction material.

Answer-state rules:

| State | First implementation rule |
| --- | --- |
| `grounded` | generation mode is `llm` and citations exist |
| `partially_supported` | grounding no-source reason is `below_threshold` |
| `unsupported` | grounding has no citations and has a no-source reason |
| `provider_fallback` | fallback category indicates provider error or empty response |
| `guard_rejected` | fallback category is `guard_rejected` |
| `local_mode` | no client or generation mode is `local` |

If multiple rules match, fallback and guard states win over retrieval states
because the user needs to know why the final answer is not a provider-backed
grounded answer.

### `internal/runtime`

Responsibilities:

- allowlist `knowledge_answer_state` in streaming completion and replay metadata;
- keep replay behavior consistent with fresh turns;
- avoid exposing raw provider or prompt internals.

No new runtime state machine is required.

### `internal/server`

Responsibilities:

- keep turn request metadata carrying selected knowledge space;
- update gap capture to use `knowledge_answer_state`;
- dedupe unsupported/partial gaps so repeated questions do not spam the queue;
- keep capture optional when gap services are nil.

Recommended first dedupe behavior:

- compute deterministic dedupe key from tenant ID, space ID, normalized question,
  and answer state/no-source reason;
- if an open gap already matches, skip creating a duplicate;
- defer count/timestamp update unless Stage 3 proves it is cheap.

### `internal/admin`

Responsibilities:

- support enough gap lookup behavior for server dedupe.

Preferred implementation:

- add a small service helper that lists gaps for tenant/space and checks for an
  open matching normalized question/reason;
- avoid changing the persisted gap schema unless needed.

If schema change is necessary, add optional fields only.

### `web/app`

Responsibilities:

- render answer state compactly in transcript status and Presence summary;
- distinguish grounded, partial, unsupported, provider fallback, and guard
  rejection copy;
- keep source chips for grounded turns;
- keep Presence bounded and summary-first.

Recommended UI copy:

| State | User-facing copy |
| --- | --- |
| `grounded` | `Grounded in <space>` |
| `partially_supported` | `Partially supported by <space>` |
| `unsupported` | `No supporting source in <space>` |
| `provider_fallback` | `Provider fallback` |
| `guard_rejected` | `Guardrail fallback` |
| `local_mode` | `Local mode` |

Detailed score explanations stay in `/admin`, not `/app`.

## Data Flow

### Grounded LLM Turn

1. `/app` sends user text with selected `knowledge_space_id`.
2. `knowledge.Service.Ground` returns citations for that space.
3. `PersonaAgent` adds source-bounded context to the system prompt.
4. Fake or real LLM returns content.
5. `PersonaAgent` classifies `knowledge_answer_state=grounded`.
6. Runtime emits citations and answer state in message-completed metadata.
7. `/app` shows citation chips and grounded copy.

### Unsupported Knowledge Turn

1. Retrieval returns no citations and a no-source reason such as
   `no_matching_chunks`.
2. `PersonaAgent` still allows a general persona answer if provider generation
   succeeds, but metadata says `knowledge_answer_state=unsupported`.
3. Runtime/server emits allowlisted state and reason.
4. Server gap capture creates one local gap if no matching open gap exists.
5. `/app` shows no supporting source in the selected space.
6. `/admin` gap queue shows the operator what to fix.

### Provider Or Guard Fallback

1. Retrieval may or may not have found citations.
2. Provider fails, returns empty text, or guard rejects content.
3. Final result becomes fallback output.
4. Answer state becomes `provider_fallback` or `guard_rejected`.
5. `/app` explains fallback category instead of implying grounded confidence.

## Autoplan Review Results

### CEO Review

Score: 9/10.

The plan correctly focuses on the trust loop rather than raw retrieval expansion.
The strongest product move is to make unsupported answers become operator work
without making normal chat brittle. Strict grounded-only behavior is worth
deferring until the answer-state policy has real usage behind it.

### Design Review

Score: 8.5/10.

The `/app` change should be compact and calm. The chat surface should show answer
state and citations, not ranking internals. Presence remains a summary panel,
and `/admin` remains the debug surface.

### Engineering Review

Score: 8.5/10.

The architecture is low-risk if the implementation stays close to existing
structures: `PersonaAgent` owns metadata, runtime allowlists metadata, server
captures gaps, and web renders the already emitted state. The risk is naming and
dedupe consistency, not infrastructure.

### DX Review

Score: 8/10.

Developer experience remains strong if the phase adds no new setup dependency,
keeps fake LLM/retrieval tests, and documents DeepSeek runtime behavior without
requiring real provider calls in CI.

## Decisions For Stage 3

| Decision | Recommended Default | Why |
| --- | --- | --- |
| Answer-state location | `internal/agents` | Persona generation sees grounding, provider, and guard outcomes |
| Partial support rule | start with `below_threshold` | Reuses existing retrieval signal and avoids new scoring policy |
| Strict grounded-only mode | defer | Avoids making general conversation brittle before telemetry/usage |
| Gap dedupe | skip duplicate open gaps | Simple, deterministic, avoids schema churn |
| App status display | text plus existing chips | Clearer than icon-only and works without new icon dependencies |
| Chinese fallback copy | lightweight Chinese-aware copy | Matches current usage without a full localization framework |

## Risk Register

| Risk | Severity | Mitigation |
| --- | --- | --- |
| Answer state overclaims grounding | High | `grounded` only when citations exist and generation mode is provider-backed |
| Fallback hides useful retrieval state | Medium | Fallback state wins, but keep knowledge metadata where safe |
| Gap dedupe misses near-duplicates | Medium | Deterministic exact normalized dedupe first; semantic dedupe later |
| App UI becomes diagnostic-heavy | Medium | Keep score details out of `/app`; link behavior through admin queue |
| Prompt context grows too large | Medium | Keep citation count limited and source text bounded |
| Stream/replay metadata diverges | High | Add runtime tests for fresh stream and replay paths |

## Success Signal

Phase 15 is successful when a user can:

1. ask a question that matches uploaded knowledge and see a grounded answer with
   citations;
2. ask a question outside the selected space and see an unsupported state instead
   of fake certainty;
3. see provider or guard fallback explained clearly;
4. return to `/admin` and find one useful gap record, not a noisy duplicate pile;
5. run all tests locally without DeepSeek, SQLite, or external search services.

## Assignment For Stage 3

Implement the answer-state spine first:

> Add deterministic answer-state classification in `PersonaAgent`, allowlist it
> through runtime streaming metadata, render it in `/app`, and use it to dedupe
> unsupported/partial gap capture.

Keep Stage 3 strictly TDD: each production change starts with a failing test from
the Phase 15 plan.

