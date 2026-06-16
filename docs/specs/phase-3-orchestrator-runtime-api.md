# Phase 3 Orchestrator Runtime and API Spec

## Context

Phase 3 turns the Phase 2 capability layer into a callable runtime. The system should be able to accept a conversation, route it, pick an expert agent, execute the agent, emit observable runtime events, and expose the flow through CLI and HTTP entrypoints without requiring external providers in tests.

This Phase covers M6 and M7 from `plan.md`:

- M6.1 Orchestrator main loop.
- M6.2 Session state machine.
- M6.3 Concurrent scheduling.
- M6.4 Fault tolerance and graceful fallback.
- M6.5 Event bus and observability instrumentation.
- M6.6 Multi-turn conversation continuity.
- M7.1 CLI entrypoint.
- M7.2 HTTP API.
- M7.3 Optional gRPC boundary.
- M7.4 Authentication and rate limiting.
- M7.5 End-to-end tests.
- M7.6 Deployment materials.
- M7.7 Documentation and ADR.

## Current State

Phase 2 already provides:

| Area | Current state | Gap for Phase 3 |
| --- | --- | --- |
| Contracts | `core.Router`, `core.Agent`, `core.Skill`, `core.Orchestrator` interfaces | No production orchestrator implementation |
| Routing | Rule, LLM, and hybrid routers | Not wired into a runtime request flow |
| Agents | Persona, memory, knowledge, task, tool, and safety agents | No runtime registry bootstrap or execution pipeline |
| Skills | Deterministic skill library | No orchestration-level timeout, event, or fallback policy |
| Runtime | Local in-process `EventBus` | No typed runtime events or request correlation |
| Entrypoints | Placeholder CLI and server startup log | No chat command, HTTP `/chat`, SSE stream, `/health`, or `/metrics` |
| Storage | Local filesystem store and in-memory stores | No SQLite; Phase 3 should continue with local storage only |

## Proposed Change

Implement Phase 3 as a deterministic, test-first runtime and entrypoint layer. All production code must be introduced through failing tests first in Stage 3.

### Orchestrator

Add a production orchestrator under `internal/runtime` or a dedicated package approved in Stage 2. It should:

- Implement `core.Orchestrator`.
- Accept a `types.Conversation`.
- Validate conversation IDs, tenant IDs, user IDs, and non-empty user input.
- Call the configured router.
- Look up the matching agent in `core.AgentRegistry`.
- Run the agent with context deadlines.
- Return a `types.AgentResult`.
- Emit runtime events for request start, route chosen, agent selected, skill or agent errors, response completed, and fallback.
- Preserve deterministic behavior in tests with fake router, fake agent, fake clock, and local event bus.

### Session State Machine

Add a small state machine for conversation lifecycle:

- `received`
- `routing`
- `agent_running`
- `completed`
- `failed`
- `fallback`

The state machine should reject invalid transitions and expose transition metadata for events and tests.

### Fault Tolerance

Runtime failure handling should be explicit:

- Router error: fallback to `persona.chat` when safe.
- No matching agent: return a safe assistant message and metadata with the missing intent.
- Agent error: return a safe assistant message and metadata with the failed agent.
- Context cancellation or deadline: return the context error and emit timeout/cancel events.
- Panic in a runtime dependency: recover at the orchestrator boundary, emit a failure event, and return a wrapped provider/runtime error.

### Concurrency

Phase 3 should support safe concurrent conversations, but it should not overbuild a distributed scheduler. Requirements:

- The orchestrator must be goroutine-safe when using shared registries and event bus.
- Per-request context and metadata must not leak across requests.
- Optional internal fan-out may be used only for independent pre/post hooks approved in Stage 2.
- Tests should run multiple conversations concurrently and prove isolated results and event correlation.

### Multi-Turn Conversation

Phase 3 should support multi-turn input at the contract level:

- Preserve existing conversation history when routing.
- Append or return assistant messages without mutating caller-owned slices unexpectedly.
- Carry tenant/user/conversation IDs through metadata and events.
- Use local storage only if Stage 2 approves persisting conversation snapshots; otherwise keep the runtime pure and stateless.

### CLI Entry

Replace the placeholder CLI with a minimal local conversation entrypoint:

- Support a one-shot mode, for example `digital-twin ask "question"`.
- Support an interactive REPL only if Stage 2 approves it as a separate small task.
- Load local config.
- Use deterministic local bootstrap components by default.
- Print assistant text and optionally JSON output for tests.
- Exit non-zero on invalid config, invalid request, or runtime failure.

### HTTP API

Add HTTP server behavior under `cmd/server` and internal server packages:

- `GET /health`: returns service health and version-like metadata.
- `GET /metrics`: returns current in-memory Prometheus text export.
- `POST /chat`: accepts JSON conversation input and returns JSON `AgentResult`.
- `GET or POST /chat/stream`: supports Server-Sent Events for structured runtime events if Stage 2 keeps SSE in scope.

The API should be local-testable with `httptest.Server` and must not require real external provider credentials.

### Authentication and Rate Limiting

Phase 3 should include a minimal production-shaped boundary:

- Optional API key auth controlled by config.
- Missing or invalid API key returns `401`.
- Per-key or per-tenant in-memory rate limiting returns `429`.
- Auth and rate limit middleware must be unit-tested with `httptest`.

JWT can remain out of scope unless Stage 2 explicitly approves it. API key auth is enough for Phase 3.

### SSE Events

If SSE remains in scope after Stage 2, event output should use stable event names:

- `request_started`
- `state_changed`
- `route_selected`
- `agent_selected`
- `message_delta`
- `message_completed`
- `runtime_error`
- `done`

Phase 3 does not need real token streaming from external LLMs. It can stream runtime events and final message chunks derived from deterministic agent results.

### Deployment Materials

Add deployment-facing materials without shipping a production deploy:

- Document local run commands.
- Add a minimal Dockerfile only if Stage 2 approves it.
- Document required environment variables.
- Add an ADR for HTTP/SSE and local-storage-first choices.

## Acceptance Criteria

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. `go build ./cmd/server` passes.
4. `go build ./cmd/cli` passes.
5. Orchestrator tests cover successful route-agent-result flow.
6. Orchestrator tests cover router error fallback, missing agent, agent error, context cancellation, and panic recovery.
7. State machine tests cover valid transitions and invalid transition rejection.
8. Runtime event tests prove ordered events and request correlation metadata.
9. Concurrent runtime tests prove isolated results for multiple conversations.
10. CLI tests cover one-shot request success and invalid request failure.
11. HTTP tests cover `/health`, `/metrics`, `/chat`, auth failure, rate limit failure, and JSON validation.
12. SSE tests cover stable event names and final `done` event if SSE remains in scope.
13. E2E tests run a full local deterministic conversation through CLI or HTTP without real external services.
14. README and release notes describe only real Phase 3 capabilities.
15. No SQLite, Postgres, Qdrant, Redis, external search, real TTS, real ASR, or avatar runtime is required for tests.

## Testing Plan

| Layer | What | Expected |
| --- | --- | --- |
| Unit | Orchestrator success path | Router, registry, agent, and result are connected |
| Unit | Orchestrator failures | Safe fallback or explicit error with events |
| Unit | State machine | Valid and invalid transitions are deterministic |
| Unit | Event emission | Ordered events include conversation/request IDs |
| Unit | Auth middleware | Missing/invalid API key returns `401` |
| Unit | Rate limit middleware | Exceeded quota returns `429` |
| HTTP | `/health` | Returns `200` JSON |
| HTTP | `/metrics` | Returns Prometheus text |
| HTTP | `/chat` | Returns deterministic `AgentResult` |
| HTTP | SSE stream | Emits stable event sequence if included |
| CLI | one-shot ask | Prints assistant response and exits `0` |
| E2E | local runtime | One conversation runs through router, agent, event bus, and entrypoint |
| Command | `go test ./...` | Pass |
| Command | `go vet ./...` | Pass |
| Command | `go build ./cmd/server` | Pass |
| Command | `go build ./cmd/cli` | Pass |

## Files Reference

| File or area | Change |
| --- | --- |
| `internal/runtime` | Orchestrator, state machine, runtime event types, event recorder helpers |
| `internal/server` or approved equivalent | HTTP handlers and middleware |
| `cmd/server` | Real server startup and route wiring |
| `cmd/cli` | One-shot CLI and optional REPL |
| `internal/config` | API auth, rate limit, and runtime timeout config |
| `internal/observability` | Metrics integration for HTTP `/metrics` and runtime counters |
| `internal/testutil` | Runtime fakes and HTTP test helpers only where needed |
| `docs/specs/phase-3-orchestrator-runtime-api.md` | This spec |
| `docs/design/phase-3-orchestrator-runtime-api.md` | Stage 1 design rationale |
| `docs/plans/phase-3-orchestrator-runtime-api-plan.md` | Stage 2 output after plan approval |

## Out of Scope

- Web UI, admin console, avatar rendering, real TTS, real ASR, or live voice loop.
- SQLite or any new database service.
- Real external LLM/search/calendar/TTS/ASR providers in tests.
- gRPC unless Stage 2 explicitly keeps it in scope.
- Distributed queues, worker pools, Kubernetes manifests, or production deployment automation.
- Multi-tenant billing, audit retention policy, eval runner, and release gates from Phase 5.

## Rollback Plan

Revert the Phase 3 commit(s). Phase 3 should add runtime code, local entrypoints, tests, and docs without data migration. If local conversation snapshots are approved in Stage 2, they must be stored in a versioned local directory that can be safely ignored or removed during rollback.

