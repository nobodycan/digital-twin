# Phase 7 LLM-Driven Persona Plan

Date: 2026-06-23

Status: Approved by the user on 2026-06-23. Stage 3 implementation complete, Stage 4 review complete, Stage 5 local QA complete on 2026-06-24, and Stage 6 security check found no high-severity blockers. Pending Stage 7 ship/integration.

Gate: passed on 2026-06-23.

## Plan Summary

Phase 7A turns the most visible local product gap into a real capability: `PersonaAgent` should generate context-aware persona chat through a configured LLM client instead of always returning the deterministic placeholder.

The plan keeps the system local-first by default. A fresh clone still runs without secrets, CI never calls a paid provider, and all LLM behavior is tested with fake clients or fake HTTP servers.

## Premise Confirmation

The Stage 1 spec has been approved by the user. These premises are accepted for Stage 2:

| Premise | Decision | Rationale |
| --- | --- | --- |
| Persona chat is the right first LLM integration | Accepted | It fixes the exact visible failure reported during local usage. |
| Router/RAG/tool planning stay deterministic in Phase 7A | Accepted | Avoids stacking multiple probabilistic failure modes. |
| Local/mock remains default | Accepted | Preserves current onboarding, tests, and smoke checks. |
| Real providers are never called in CI | Accepted | Prevents flaky, paid, secret-dependent tests. |
| Post-generation guard is required | Accepted | Model output must pass the same persona/safety boundary as deterministic output. |

## What Already Exists

| Capability | Existing file(s) | Reuse in Phase 7 |
| --- | --- | --- |
| Provider-neutral LLM interface | `internal/llm/client.go` | Use `llm.Client.Chat` as the generation boundary. |
| OpenAI-compatible adapter | `internal/llm/openai.go`, `internal/llm/openai_test.go` | Reuse as first real provider path via a small factory. |
| Deterministic fake LLM | `internal/testutil/infra_fakes.go` | Reuse or mirror for unit tests; no real network. |
| LLM router fallback patterns | `internal/router/llm.go`, `internal/router/llm_test.go` | Reuse metadata/fallback style, but do not enable router LLM by default. |
| Persona model/render/guard | `internal/persona` | Build prompt from renderer; enforce guard after generation. |
| PersonaAgent placeholder | `internal/agents/experts.go` | Replace only the persona chat generation path. |
| Runtime bootstrap | `internal/app/bootstrap.go` | Inject generation dependencies into `PersonaAgent`. |
| Server config/redaction | `internal/config/config.go`, `cmd/server/main.go` | Expand LLM config and redact all secrets. |
| HTTP/SSE compatibility | `internal/server/server.go`, `internal/server/server_test.go` | Keep `/chat`, `/chat/stream`, `/experience/stream` contracts unchanged. |
| CLI ask path | `cmd/cli/main.go` | Preserve deterministic default; optionally support config follow-up later. |

## NOT In Scope

- LLM-first routing.
- RAG answer generation.
- Agentic tool planning.
- Token-by-token streaming.
- Real TTS/ASR/avatar provider changes.
- SQLite/Postgres, queues, Kubernetes, OAuth/RBAC, billing, or compliance certification.
- Tests that require provider credentials or internet access.

## CEO Review

### Scope Challenge

The highest-leverage Phase 7 is not "make everything LLM-powered." The right wedge is narrower: make the persona path honestly generated when configured, while preserving all deterministic foundations around routing, governance, local storage, and presentation.

Dream state delta:

```text
CURRENT
  PersonaAgent returns one fixed sentence.

PHASE 7A
  PersonaAgent can generate through configured LLM, falls back locally,
  exposes safe provider metadata, and guards generated output.

12-MONTH IDEAL
  Policy-governed multi-agent digital human with LLM planning, RAG,
  memory summarization, tool authorization, streaming, voice/avatar providers,
  tenant-aware observability, and offline eval gates.
```

### Alternatives Reviewed

| Approach | Decision | Reason |
| --- | --- | --- |
| A. LLM-driven `PersonaAgent` only | Selected | Highest value/risk ratio; directly fixes fixed replies. |
| B. LLM router plus persona generation | Deferred | Adds wrong-route and wrong-answer failures together. |
| C. Full LLM agent orchestration | Deferred | Too much blast radius for one phase. |
| D. Voice/avatar provider first | Deferred | Improves presentation without fixing answer quality. |

### CEO Consensus Table

Dual external voices were not available in the current Codex tool surface; this table records the primary review.

| Dimension | Primary review | Consensus |
| --- | --- | --- |
| Premises valid? | Yes | Confirmed |
| Right problem to solve? | Yes | Confirmed |
| Scope calibration correct? | Yes, if token streaming stays deferred | Confirmed |
| Alternatives sufficiently explored? | Yes | Confirmed |
| Competitive/product risk covered? | Partially; answer quality needs eval follow-up | Confirmed with watch item |
| 6-month trajectory sound? | Yes | Confirmed |

## Design Review

Phase 7A has no new first-class UI surface. Existing Web/Admin/experience streams must continue to render the final assistant result and metadata without requiring layout changes.

Design review is therefore scoped to interaction behavior:

| Dimension | Score | Decision |
| --- | --- | --- |
| User-visible response quality | 8/10 target | Generated persona output replaces fixed placeholder. |
| Transparency | 9/10 target | Model identity question has explicit local vs configured answer. |
| Error state | 8/10 target | Provider failures degrade to safe fallback with metadata. |
| Streaming expectation | 7/10 target | Existing SSE remains event/final-message based; token streaming deferred. |
| Admin/docs clarity | 8/10 target | README documents configuration and no-real-provider CI policy. |

No visual mockups are required for Phase 7A.

## Engineering Review

### Architecture

```text
cmd/server config
  |
  v
internal/config.LLMConfig
  |
  v
internal/llm factory
  |-- local/mock client
  |-- OpenAI-compatible client
  |
  v
internal/app.NewLocalRuntime
  |
  v
agents.PersonaAgent
  |-- persona.Renderer builds system prompt
  |-- llm.Client.Chat generates assistant text
  |-- persona.Guard checks generated text
  |-- deterministic fallback handles local/provider/guard failures
  |
  v
runtime.Orchestrator -> CLI / HTTP / SSE / experience stream
```

### Engineering Decisions

| # | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- |
| 1 | Add a small `internal/llm` factory instead of constructing clients in `cmd/server` | Mechanical | Explicit over clever | Keeps provider selection testable and avoids bloating server startup. | Inline construction in `cmd/server`. |
| 2 | Reuse and expand `config.LLMConfig` instead of reusing generic `ProviderConfig` directly | Taste | DRY with domain clarity | LLM needs model, timeout, fallback policy; `ProviderConfig` lacks model. | Force-fit `ProviderConfig`. |
| 3 | Keep local/mock as default and require explicit provider config for real LLM mode | Mechanical | Pragmatic | Preserves current tests and local developer path. | Enable real provider when only API key exists. |
| 4 | Add explicit model-transparency handling before or inside PersonaAgent generation | Mechanical | Completeness | Prevents misleading answers to "what model are you". | Let provider improvise identity. |
| 5 | Preserve SSE event contract and defer token streaming | Taste | Bias toward action | Non-streaming generation fixes the main gap; token streaming can follow safely. | Implement token streaming now. |
| 6 | Guard model output after generation and fallback on rejection | Mechanical | Safety boundary | The model is untrusted output; guard must not be only pre-generation. | Trust prompt instructions only. |

### Implementation Slices

Each slice must follow Superpowers TDD in Stage 3: write the failing test first, make it pass, then refactor.

| Slice | Goal | Main files | RED test first | GREEN implementation | Verification |
| --- | --- | --- | --- | --- | --- |
| P7-01 | Expand LLM config and redaction | `internal/config/config.go`, `internal/config/config_test.go`, `configs/app.yaml` | Config tests for provider/model/base URL/API key/env/summary redaction fail | Add fields, env names, YAML keys, validation, safe summary | `go test ./internal/config` |
| P7-02 | Add LLM client factory | `internal/llm/factory.go`, `internal/llm/factory_test.go` | Factory tests for local default, OpenAI-compatible config, unsupported provider, no construction network fail | Return deterministic local client or `OpenAIClient`; validate unsupported providers | `go test ./internal/llm` |
| P7-03 | Add persona generation options | `internal/agents/experts.go`, `internal/agents/experts_test.go` | `PersonaAgent` with fake LLM should return fake generated text, not placeholder | Add constructor/options or config struct for optional client/metadata/fallback | `go test ./internal/agents` |
| P7-04 | Build prompt from persona/context | `internal/agents`, `internal/persona` if needed | Fake LLM captures messages and fails until system prompt + conversation are included | Render persona prompt and include recent conversation messages | `go test ./internal/agents ./internal/persona` |
| P7-05 | Add transparency behavior | `internal/agents/experts.go`, tests | Questions like "你背后是什么模型" fail until local/configured answers are explicit and secret-free | Add deterministic transparency branch or constrained prompt path | `go test ./internal/agents` |
| P7-06 | Add fallback and guard behavior | `internal/agents/experts.go`, tests | Provider error/empty output/guard rejection tests fail | Return safe fallback with `generation_mode` and reason metadata | `go test ./internal/agents` |
| P7-07 | Wire runtime/server bootstrap | `internal/app/bootstrap.go`, `cmd/server/main.go`, tests | Configured runtime test fails until `PersonaAgent` receives LLM client | Pass LLM dependencies through `LocalRuntimeConfig`; build from `cfg.LLM` | `go test ./internal/app ./cmd/server` |
| P7-08 | Preserve transport compatibility | `cmd/cli`, `internal/server`, smoke tests | Tests assert `/chat`, `/chat/stream`, `/experience/stream`, CLI still work in local mode and configured fake mode | Keep endpoint schemas stable; metadata additive only | `go test ./cmd/cli ./internal/server ./cmd/smoke` |
| P7-09 | Docs and release notes | `README.md`, `RELEASE_NOTES.md`, Phase 7 docs | Grep/doc checks fail until local vs LLM mode is documented | Add configuration examples, CI policy, limitations | `rg -n "LLM mode|local/mock|DIGITAL_TWIN_LLM" README.md RELEASE_NOTES.md docs` |

## Test Diagram

```text
Config load
  |-- local default ---------------------------- unit
  |-- env/YAML provider override --------------- unit
  |-- prod-like missing model/base/key ---------- unit
  |-- SafeSummary/RedactSecrets ---------------- unit

LLM factory
  |-- local/mock client ------------------------ unit
  |-- openai-compatible client ----------------- unit
  |-- unsupported provider --------------------- unit
  |-- no network at construction --------------- fake server assertion

PersonaAgent
  |-- no client fallback ----------------------- unit
  |-- configured fake client success ----------- unit
  |-- prompt includes persona + conversation ---- unit
  |-- model identity transparency -------------- unit
  |-- provider error fallback ------------------ unit
  |-- timeout/context cancellation ------------- unit
  |-- empty response fallback ------------------ unit
  |-- guard rejection fallback ----------------- unit
  |-- metadata has provider/model/no secrets ---- unit

Runtime transports
  |-- app bootstrap local default -------------- integration-ish unit
  |-- server configured fake/OpenAI test -------- httptest/fake server
  |-- /chat unchanged response shape ----------- HTTP test
  |-- /chat/stream final-message SSE ----------- HTTP test
  |-- /experience/stream presentation events ---- HTTP test
  |-- CLI ask local default -------------------- command test
```

## Test Matrix

| ID | Area | Scenario | Expected |
| --- | --- | --- | --- |
| T7-01 | Config | Empty config | `cfg.LLM.Provider` defaults to `local` or equivalent local mode. |
| T7-02 | Config | Env overrides provider/base URL/model/API key/timeout/fallback | Fields load correctly. |
| T7-03 | Config | Production-like `openai-compatible` missing model/base URL/API key | Validation returns actionable error. |
| T7-04 | Config | Safe summary with LLM base URL credentials/API key | Secrets and URL credentials are redacted. |
| T7-05 | Factory | Local/mock provider | Returns deterministic client; construction does not call network. |
| T7-06 | Factory | OpenAI-compatible provider | Returns `OpenAIClient` with configured base URL/model/key. |
| T7-07 | Factory | Unsupported provider | Returns config error naming supported providers. |
| T7-08 | OpenAI adapter | Fake server receives chat request | Request has model, messages, auth, non-streaming flag. |
| T7-09 | PersonaAgent | No LLM client | Returns deterministic local response with `generation_mode=local`. |
| T7-10 | PersonaAgent | Fake LLM returns generated content | Result content is generated, not the placeholder. |
| T7-11 | PersonaAgent | Fake LLM captures request | System prompt and user conversation are included. |
| T7-12 | PersonaAgent | User asks "你背后是什么模型" in local mode | Response honestly says local deterministic/no configured model. |
| T7-13 | PersonaAgent | User asks model identity in configured mode | Response names provider/model only, no secrets. |
| T7-14 | PersonaAgent | Provider error | Safe fallback with `generation_mode=fallback` and redacted reason. |
| T7-15 | PersonaAgent | Empty provider response | Safe fallback; no empty assistant message leaks. |
| T7-16 | PersonaAgent | Context cancellation/timeout | Returns typed or wrapped timeout/cancel behavior consistent with runtime. |
| T7-17 | PersonaAgent | Generated output violates persona guard | Guard fallback replaces unsafe content. |
| T7-18 | Metadata | Generated result | Includes `intent`, `llm_provider`, `llm_model`, `generation_mode`, optional usage. |
| T7-19 | Metadata | Any result | Does not include API key or full secret-bearing URL. |
| T7-20 | Runtime | `NewLocalRuntime` default | Existing deterministic local tests continue to pass. |
| T7-21 | Server | Configured fake/OpenAI-compatible fake server | `/chat` returns generated content through runtime path. |
| T7-22 | SSE | `/chat/stream` | Event names and final message framing remain compatible. |
| T7-23 | Experience | `/experience/stream` | Presentation events still derive from final assistant text. |
| T7-24 | CLI | `go run ./cmd/cli ask "你是谁"` in local mode | Works without credentials and is transparent about local mode. |
| T7-25 | Docs | README and release notes | Explain local/mock mode, LLM mode env vars, CI fake-provider policy. |

## Failure Modes Registry

| Failure mode | Severity | Detection | Planned response |
| --- | --- | --- | --- |
| Real provider call in CI | High | Tests must use fake client/server; no real env required | Fail test review if any test requires credentials. |
| API key leak in logs/readiness/metadata | High | Redaction tests and metadata assertions | Redact secret values and URL credentials. |
| Provider timeout hangs request | High | Timeout/cancel tests | Use configured timeout and context propagation. |
| Provider returns malformed/empty response | Medium | Fake client/server tests | Safe fallback with reason metadata. |
| Generated text violates persona boundaries | High | Guard rejection test | Replace with guard fallback. |
| Model identity answer is misleading | Medium | Transparency tests | Deterministic transparency branch or constrained prompt. |
| Scope expands into router/RAG/tool planning | High | Plan gate and file review | Defer to Phase 7B/8. |
| Existing local smoke breaks | High | `go test ./...`, smoke commands | Local/mock default remains first-class. |

## Error And Rescue Registry

| Error | User-visible rescue | Developer-visible rescue |
| --- | --- | --- |
| Missing provider config in production-like mode | Startup/readiness reports config failure without secret leakage | Error names missing `llm.model`, `llm.base_url`, or `llm.api_key`. |
| Provider unreachable | Assistant returns safe fallback if policy allows | Metadata/log category says provider error, redacted. |
| Unsupported provider | Startup/build handler fails fast | Error lists supported providers. |
| Timeout | Safe fallback or typed error, depending policy | Timeout value/config path is documented. |
| Guard rejection | Assistant returns persona-safe fallback | Metadata includes guard reason without unsafe content. |

## DX Review

Phase 7A is developer-facing because developers must configure LLM mode safely.

Developer journey:

| Stage | Expected path | Plan requirement |
| --- | --- | --- |
| Discover | README explains local vs LLM mode | Add a short "LLM mode" section. |
| Install | Fresh clone still needs no secrets | Local/mock default. |
| Hello world | `go run ./cmd/cli ask "你是谁"` works | Local transparent response. |
| Configure | Set env vars for provider/model/base/key | Copy-paste PowerShell examples. |
| Verify | Run server and `/chat` | Include curl example. |
| Debug | Config errors explain missing fields | Problem/cause/fix style errors. |
| Test | Run `go test ./...` | No provider credentials. |
| Upgrade | Existing config with only `llm.api_key` should not silently mislead | Document new fields and defaults. |
| Operate | Readiness/log summary redacts secrets | Safe summary tests. |

DX score target:

| Dimension | Current | Target |
| --- | --- | --- |
| Time to hello world | 7/10 | 9/10 |
| Config clarity | 4/10 | 8/10 |
| Error actionability | 6/10 | 8/10 |
| Docs findability | 6/10 | 8/10 |
| Secret-safety confidence | 7/10 | 9/10 |
| Test ergonomics | 8/10 | 9/10 |
| Local fallback clarity | 5/10 | 9/10 |
| Overall DX | 6/10 | 8.5/10 |

## Build Order

1. P7-01 config/redaction.
2. P7-02 LLM factory.
3. P7-03 `PersonaAgent` LLM injection.
4. P7-04 prompt construction.
5. P7-05 transparency behavior.
6. P7-06 guard/fallback behavior.
7. P7-07 runtime/server wiring.
8. P7-08 transport compatibility.
9. P7-09 docs/release notes.

Parallelism after approval:

- Config/factory work can proceed before agent prompt work.
- Docs can start after config names are fixed.
- HTTP/SSE compatibility should wait until runtime wiring exists.

## Verification Commands

Run after the relevant slices and before Stage 4 review:

```powershell
go test ./internal/config
go test ./internal/llm
go test ./internal/agents
go test ./internal/app ./cmd/server
go test ./cmd/cli ./internal/server ./cmd/smoke
go test ./...
go test -race ./...
go vet ./...
rg -n "LLM mode|local/mock|DIGITAL_TWIN_LLM|openai-compatible" README.md RELEASE_NOTES.md docs
```

Optional manual local checks after implementation:

```powershell
go run ./cmd/cli ask "你是谁"
$env:DIGITAL_TWIN_LLM_PROVIDER="openai-compatible"
$env:DIGITAL_TWIN_LLM_BASE_URL="http://localhost:9999/v1"
$env:DIGITAL_TWIN_LLM_MODEL="test-model"
$env:DIGITAL_TWIN_LLM_API_KEY="test-key"
go run ./cmd/server
```

The configured-provider manual command requires a local fake or compatible endpoint. It must not be part of CI unless the fake endpoint is started inside the test.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Keep Phase 7A scoped to persona generation only | Mechanical | Pragmatic | Fixes visible product gap with contained blast radius. | Full LLM orchestration. |
| 2 | CEO | Defer token streaming | Taste | Bias toward action | Non-streaming generation unlocks real answers first. | Implement token streaming now. |
| 3 | Eng | Add LLM factory | Mechanical | Explicit over clever | Centralizes provider creation and testing. | Inline server construction. |
| 4 | Eng | Expand `LLMConfig` | Taste | DRY with domain clarity | LLM-specific fields do not fit generic provider config cleanly. | Generic `ProviderConfig` only. |
| 5 | Eng | Guard generated output | Mechanical | Completeness | Model output is an untrusted boundary. | Prompt-only safety. |
| 6 | DX | Document local/mock and configured LLM modes | Mechanical | Explicit over clever | Prevents confusion about why local answers differ from provider answers. | Rely on config names alone. |

## Review Scores

| Review | Result |
| --- | --- |
| CEO | 8.5/10. Scope is correctly narrow; follow-up eval quality remains a later concern. |
| Design | 8/10. No new UI required; behavior and transparency states are specified. |
| Engineering | 8/10. Architecture reuses existing contracts; main risks are config validation and guard/fallback coverage. |
| DX | 8/10 target after docs; current docs must be updated during implementation. |

## Cross-Phase Themes

- Keep local/mock as the default. This appears in CEO, engineering, DX, and test planning.
- Treat LLM output as untrusted. This appears in engineering, security, and failure-mode planning.
- Be honest about model identity. This appears in product, UX, and DX planning.
- Do not expand into full agentic behavior yet. This appears in CEO and engineering planning.

## Stage 3 Handoff

After the user approves this plan, Stage 3 must use Superpowers `$test-driven-development` and `$executing-plans`.

Rules for implementation:

- No production code before a failing test.
- Keep each slice small enough to verify independently.
- Do not modify unrelated Phase 0-6 status-alignment docs unless documentation updates explicitly require it.
- Do not call real external LLM providers from tests.
- Preserve existing public endpoint schemas; metadata can be additive.

## Gate

Stage 2 gate passed: the user approved this plan on 2026-06-23.
