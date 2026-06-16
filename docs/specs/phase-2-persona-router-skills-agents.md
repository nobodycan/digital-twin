# Phase 2 Persona, Router, Skills, and Agents Spec

## Context

Phase 2 turns the Phase 1 contracts and local infrastructure into the first useful intelligent layer of `digital-twin`. The system should be able to load a stable professional persona, render deterministic system prompts, classify user requests, execute atomic skills, and route work to expert agents without requiring real external providers in tests.

This Phase covers M3, M4, and M5 from `plan.md`:

- M3.1 Persona data model.
- M3.2 System prompt renderer.
- M3.3 Persona consistency guard.
- M3.4 Rule router.
- M3.5 LLM classification router.
- M3.6 Hybrid routing and fallback.
- M4.1 BaseAgent.
- M4.2 PersonaAgent.
- M4.3 MemoryAgent.
- M4.4 KnowledgeAgent.
- M4.5 TaskAgent.
- M4.6 ToolAgent.
- M5.1 Skill base and parameter validation framework.
- M5.2-M5.8 Memory, knowledge, task, tool, persona, safety, and presentation skills.

## Current State

Phase 1 already provides:

| Area | Current state | Gap for Phase 2 |
| --- | --- | --- |
| Contracts | `pkg/types` Message, Conversation, Intent, AgentResult, SkillResult | No persona contract or additional Phase 2 intent names |
| Core | `internal/core` Agent, Skill, Router, Orchestrator interfaces and registries | No concrete router, skill framework, or agent implementations |
| LLM | Provider-neutral client, retry wrapper, OpenAI-compatible client | No prompt renderer, no LLM classifier adapter |
| Memory | Short-term and long-term memory components | No memory skills or MemoryAgent |
| Store/vector | Local Store and in-memory VectorStore | No knowledge skills or KnowledgeAgent |
| Testutil | Fakes for contracts | No Phase 2 persona/router/skill/agent fakes or golden prompts |

The existing `core.Agent` interface uses `Run(context.Context, types.Conversation, types.Intent)`, and `core.Skill` uses `Run(context.Context, map[string]any)`. Phase 2 must align to these method names and must not introduce incompatible `Handle` method names.

## Proposed Change

Implement Phase 2 as a deterministic, test-first capability layer. All production code must be introduced through failing tests first in Stage 3.

### Persona Model

Add a persona package, preferably `internal/persona`, with:

- `Persona` data model containing stable identity, role, expertise, tone, boundaries, allowed claims, forbidden claims, locale, and optional metadata.
- Validation that rejects missing identity, empty role, empty tone, contradictory or empty boundary sets, and unsafe prompt fragments where practical.
- Loader from local JSON or YAML-like config if this can be done without new dependencies; otherwise use JSON first.
- Golden fixtures for at least one professional advisor persona.

### Prompt Renderer

Add a deterministic system prompt renderer that:

- Renders a `Persona` plus runtime variables into a system prompt.
- Sorts map-like fields before rendering.
- Accepts an injectable clock for tests.
- Produces stable output for the same persona and variables.
- Has golden-file tests for stable output.

### Persona Guard

Add a consistency guard that:

- Checks outgoing assistant content against persona boundaries.
- Flags obvious out-of-persona claims, forbidden topics, and missing uncertainty wording for low-confidence outputs.
- Returns structured pass/fail data in metadata-friendly form.
- Can be used later by the orchestrator, but Phase 2 only needs unit-level behavior and agent integration where local.

### Router

Add concrete routers under `internal/router`:

- Rule router: deterministic keyword/entity rules for knowledge, memory, task, tool, and small-talk/persona fallback.
- LLM classification router: calls the Phase 1 LLM client through a small prompt and parses a strict JSON intent response.
- Hybrid router: rule-first, then LLM fallback, then low-confidence persona fallback.

Routing must preserve the original query, set confidence, include route source metadata, and return `types.IntentUnknown` or a persona fallback intent when classification is unsafe.

Phase 2 may extend `types.IntentName` with persona/small-talk and safety-oriented intent names if tests prove the need.

### Skill Framework

Add a skill base under `internal/skills`:

- Stable skill naming.
- Required/optional parameter validation without introducing a JSON schema dependency unless the plan explicitly approves one.
- Type helpers for string, bool, number, object, and string slice params.
- Structured error metadata for invalid params, dependency failures, and denied operations.

Implement each skill as a small unit with table-driven tests:

- Memory: `mem_store`, `mem_recall`, `summarize`.
- Knowledge: `embed`, `vector_search`, `cite`.
- Task: `task_decompose`, `plan`, `track`.
- Tool: `http_call`, `search_web`, `calendar`.
- Persona: `tone_adjust`, `persona_check`.
- Safety: `pii_detect`, `prompt_injection_check`, `risk_classify`, `policy_decide`.
- Presentation: `tts_speak`, `asr_transcribe`, `avatar_state`, `subtitle_timeline`.

External-facing or high-risk skills must use local deterministic implementations or fakes in Phase 2. For example, `http_call` should require an allowlist and must reject local/private network targets unless explicitly allowed by configuration and tests.

### Agents

Add expert agents under `internal/agents`:

- `BaseAgent`: common LLM, skill registry, logger/metrics hooks, and result-building helpers.
- `PersonaAgent`: small talk, style fallback, persona-safe replies.
- `MemoryAgent`: memory recall/store flows using memory skills.
- `KnowledgeAgent`: vector search plus citation flow using knowledge skills.
- `TaskAgent`: task decomposition and planning using task skills.
- `ToolAgent`: allowlisted tool execution using tool skills.
- `SafetyAgent`: risk classification and policy-oriented safety checks using safety skills.

Each agent must:

- Implement `core.Agent`.
- Use `CanHandle(types.Intent)` for routing compatibility.
- Use `Run(...)`, not `Handle(...)`.
- Return `types.AgentResult` with agent name, assistant message, confidence, and useful metadata.
- Register cleanly in `core.AgentRegistry`.
- Avoid real network and real LLM calls in tests.

## Acceptance Criteria

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. `go build ./cmd/server` passes.
4. Persona validation returns clear errors for missing or contradictory fields.
5. Same persona plus same renderer variables produces byte-stable system prompt output.
6. Prompt golden tests cover at least one professional advisor persona.
7. Persona guard tests cover pass, forbidden claim, low-confidence uncertainty, and safe fallback behavior.
8. Rule router tests cover knowledge, memory, task, tool, and persona fallback paths.
9. LLM router tests use a fake LLM client and cover valid JSON, invalid JSON, low confidence, and provider error.
10. Hybrid router tests cover rule hit, LLM fallback, low-confidence persona fallback, and failure fallback.
11. Skill framework tests cover required params, type mismatch, optional defaults, dependency failure, and structured errors.
12. Every implemented skill has tests for valid params, invalid params, and error path.
13. Every implemented agent has tests for `CanHandle`, `Run`, registry registration, dependency failure, and result metadata.
14. Tool skills reject non-allowlisted outbound calls and local/private network targets by default.
15. No Phase 3 orchestration/API behavior is implemented: no HTTP chat API, no SSE/WebSocket runtime, no production orchestrator flow.
16. No real external LLM, DB, vector service, search provider, calendar provider, TTS, ASR, or avatar provider is required for tests.

## Testing Plan

| Layer | What | Expected |
| --- | --- | --- |
| Unit | Persona validation | Clear success and failure cases |
| Unit | Prompt renderer golden files | Stable system prompt output |
| Unit | Persona guard | Structured pass/fail decisions |
| Unit | Rule router | Deterministic intent matches |
| Unit | LLM router with fake client | Strict JSON parse and fallback behavior |
| Unit | Hybrid router | Rule-first, LLM fallback, persona fallback |
| Unit | Skill framework | Param validation and structured errors |
| Unit | Each skill | Valid params, invalid params, dependency error |
| Unit | BaseAgent | Shared helper behavior and interface satisfaction |
| Unit | Each expert agent | `CanHandle`, `Run`, registry registration, failure behavior |
| Command | `go test ./...` | Pass |
| Command | `go vet ./...` | Pass |
| Command | `go build ./cmd/server` | Pass |

## Files Reference

| File or area | Change |
| --- | --- |
| `pkg/types` | Add Phase 2 intent names or metadata helpers only if required by tests |
| `internal/persona` | Persona model, validation, prompt renderer, guard, fixtures |
| `internal/router` | Rule, LLM, and hybrid routers |
| `internal/skills` | Skill framework and concrete skills |
| `internal/agents` | BaseAgent and expert agents, including SafetyAgent for `safety.check` |
| `internal/testutil` | Additional fakes only where Phase 1 fakes are insufficient |
| `docs/specs/phase-2-persona-router-skills-agents.md` | This spec |
| `docs/design/phase-2-persona-router-skills-agents.md` | Office-hours design rationale |
| `docs/plans/phase-2-persona-router-skills-agents-plan.md` | Stage 2 output after plan approval |

## Out of Scope

- Production Orchestrator implementation.
- CLI chat loop, HTTP API, SSE, WebSocket, or gRPC.
- Web UI, admin console, avatar rendering, real TTS, real ASR.
- Real provider credentials or external service calls in tests.
- SQLite, Postgres, Qdrant, pgvector, or any new persistent service.
- Generated mock tooling unless Stage 2 explicitly approves it.

## Rollback Plan

Revert the Phase 2 commit(s). Phase 2 should only add local code, fixtures, and tests behind existing interfaces, so rollback does not require data migration. If local persona fixtures or test output directories are created, remove them with the reverted files.
