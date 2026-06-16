# Phase 3 Orchestrator Runtime and API Plan

Status: PLANNED

Source artifacts:

- `docs/specs/phase-3-orchestrator-runtime-api.md`
- `docs/design/phase-3-orchestrator-runtime-api.md`
- `plan.md` Phase 3, M6, M7

## Autoplan Review Summary

Phase 3 should proceed as a transport-independent runtime plus minimal entrypoint layer. The orchestrator is the primary driver: it validates a conversation, routes intent, selects an agent, runs the agent, emits runtime events, and returns a structured result. CLI and HTTP must remain thin adapters over that same runtime.

UI scope: no. Phase 3 has HTTP/SSE surfaces, but no Web UI, avatar surface, voice loop, or admin console.

DX scope: yes. Phase 3 introduces developer-facing commands, HTTP APIs, error responses, middleware, and local verification flows.

## Premise Challenge

| Premise | Decision | Reason |
| --- | --- | --- |
| Phase 3 should include both CLI and HTTP | Accepted with constraint | CLI gives fast local feedback; HTTP is needed for Phase 4. Keep CLI one-shot and HTTP minimal. |
| SSE should be part of Phase 3 | Accepted with constraint | SSE should stream runtime events and final message data, not fake real LLM token streaming. |
| gRPC is part of M7 | Deferred | HTTP/SSE is sufficient for Phase 4 and reduces surface area. |
| REPL is needed for CLI | Deferred | One-shot `ask` is easier to test and enough for local e2e. |
| Auth should use JWT | Rejected for Phase 3 | API key auth is testable and adequate for a local runtime boundary. |
| Concurrency requires a scheduler | Rejected for Phase 3 | Goroutine-safe concurrent requests are enough; no distributed work model exists yet. |
| Local persistence should be added | Deferred by default | User does not want SQLite; stateless runtime plus local deterministic components is safer. |

Premise gate: passed by user approval of the Phase 3 spec.

## What Already Exists

| Sub-problem | Existing code to reuse |
| --- | --- |
| Conversation/result contracts | `pkg/types/contracts.go` |
| Orchestrator interface | `internal/core/interfaces.go` |
| Router interface and hybrid router | `internal/core/interfaces.go`, `internal/router` |
| Agent registry and expert agents | `internal/core/registry.go`, `internal/agents` |
| Skill registry and deterministic skills | `internal/core/registry.go`, `internal/skills` |
| Local event bus | `internal/runtime/eventbus.go` |
| Metrics exporter | `internal/observability` |
| Config loader | `internal/config` |
| Placeholder CLI/server entrypoints | `cmd/cli`, `cmd/server` |

## NOT In Scope

- SQLite or any new database service.
- Real external LLM/search/calendar/TTS/ASR/avatar provider calls in tests.
- gRPC.
- CLI REPL unless a later explicit revision adds it.
- Web UI, admin console, avatar rendering, live voice loop, real TTS, or real ASR.
- Distributed queues, worker pools, Kubernetes manifests, or production deployment automation.
- Phase 5 eval gates, cost accounting, billing, or audit retention policy.

## Recommended Architecture

```text
cmd/cli
  `-- one-shot ask command
       `-- core.Orchestrator

cmd/server
  `-- internal/server HTTP routes
       |-- auth middleware
       |-- rate limit middleware
       |-- /health
       |-- /metrics
       |-- /chat
       `-- /chat/stream
            `-- core.Orchestrator

internal/runtime
  |-- Orchestrator
  |-- StateMachine
  |-- RuntimeEvent
  |-- EventRecorder
  `-- LocalEventBus

Orchestrator
  |-- Router
  |-- AgentRegistry
  |-- EventBus
  |-- Metrics hooks
  `-- safe fallback policy

Agent
  `-- SkillRegistry
```

## Execution Strategy

Use Superpowers TDD in Stage 3 for every task:

1. RED: write the failing test first.
2. GREEN: add the smallest production code that passes.
3. REFACTOR: simplify names and boundaries while tests stay green.

No production file should be introduced without a corresponding failing test in the same small step.

## Implementation Tasks

| ID | Task | Files | Test first |
| --- | --- | --- | --- |
| P3-01 | Add runtime event names and payload helpers | `internal/runtime` | Event serialization/correlation tests |
| P3-02 | Add session state machine | `internal/runtime` | Valid transitions, invalid transitions, terminal state tests |
| P3-03 | Add orchestrator success path | `internal/runtime` | Fake router + fake agent returns deterministic `AgentResult` |
| P3-04 | Add orchestrator validation | `internal/runtime` | Missing conversation ID, tenant ID, user ID, and user message tests |
| P3-05 | Add router fallback behavior | `internal/runtime` | Router error and low confidence route to persona fallback |
| P3-06 | Add missing-agent and agent-error fallback | `internal/runtime` | Safe result metadata for not found and dependency failure |
| P3-07 | Add context cancellation and panic recovery | `internal/runtime` | Cancel/deadline tests and panic recovery test |
| P3-08 | Add event emission and recorder helpers | `internal/runtime` | Ordered event sequence with request/conversation IDs |
| P3-09 | Add concurrency isolation tests | `internal/runtime` | Multiple concurrent conversations do not share mutable metadata |
| P3-10 | Add deterministic local runtime bootstrap | `internal/runtime` or `internal/app` | Bootstrap registers router, skills, agents without external providers |
| P3-11 | Add CLI one-shot `ask` | `cmd/cli`, internal CLI helper if needed | Successful ask, invalid input, JSON output, non-zero failure tests |
| P3-12 | Add HTTP router package | `internal/server` | Route registration and method handling tests |
| P3-13 | Add `/health` endpoint | `internal/server`, `cmd/server` | `httptest` returns 200 JSON |
| P3-14 | Add `/metrics` endpoint | `internal/server`, `internal/observability` | Prometheus text output test |
| P3-15 | Add `/chat` endpoint | `internal/server` | Valid request returns `AgentResult`; invalid JSON returns 400 |
| P3-16 | Add API key middleware | `internal/server`, `internal/config` | Disabled auth, missing key, invalid key, valid key tests |
| P3-17 | Add in-memory rate limiter middleware | `internal/server`, `internal/config` | Allowed request and exceeded quota returns 429 |
| P3-18 | Add SSE runtime event stream | `internal/server` | Stable event names, `done`, cancellation cleanup |
| P3-19 | Wire `cmd/server` lifecycle | `cmd/server` | Build-level test plus handler package coverage; graceful shutdown where practical |
| P3-20 | Add local deterministic e2e tests | `internal/server` or `internal/runtime` | Full local conversation through orchestrator and HTTP |
| P3-21 | Add ADR for HTTP/SSE/local-storage-first | `docs/adr` | Read-only doc check |
| P3-22 | Sync README and release notes | `README.md`, `RELEASE_NOTES.md` | `rg` checks for Phase 3 status and no Phase 4 claims |

## Parallelization Plan

Do not parallelize before P3-03. Runtime event semantics, state machine, and the orchestrator success path establish the shape of the rest of the work.

After P3-08:

- Workstream A: P3-11 CLI one-shot.
- Workstream B: P3-12 through P3-15 HTTP basics.
- Workstream C: P3-16 and P3-17 middleware.
- Workstream D: P3-18 SSE.
- Workstream E: P3-21 and P3-22 docs.

P3-10 bootstrap blocks CLI, HTTP `/chat`, SSE, and e2e tests. If using parallel agents, each workstream must own disjoint files and must not edit shared runtime/bootstrap files at the same time.

## Test Diagram

```text
Conversation
  |-- Validate ---------------------- unit: IDs / user message / metadata
  `-- Orchestrator.Handle()
       |-- StateMachine ------------- unit: received -> routing -> agent_running -> completed
       |-- Router.Route ------------- fake: success / error / low confidence
       |-- AgentRegistry.Find ------- fake/real: found / not found
       |-- Agent.Run ---------------- fake: success / error / panic / context cancel
       |-- EventBus.Publish --------- unit: ordered events / correlation IDs
       `-- AgentResult -------------- unit: success / safe fallback metadata

HTTP request
  |-- Auth middleware ---------------- unit: disabled / missing / invalid / valid
  |-- Rate limiter ------------------- unit: under limit / over limit
  |-- Handler validation ------------- unit: invalid JSON / invalid conversation
  `-- Orchestrator ------------------- httptest: /chat and /chat/stream

CLI ask
  |-- argument parsing --------------- unit: missing prompt / JSON flag
  |-- local bootstrap ---------------- unit: deterministic dependencies
  `-- orchestrator ------------------- command/helper test: printed assistant result
```

## Explicit Test Plan

Run these after each relevant small step:

- Runtime: `go test ./internal/runtime`
- Server: `go test ./internal/server`
- Config touched: `go test ./internal/config`
- Observability touched: `go test ./internal/observability`
- CLI touched: `go test ./cmd/cli`
- Server command touched: `go test ./cmd/server`

Run these at Phase 3 close:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/server`
- `go build ./cmd/cli`

Optional on a supported platform:

- `go test -race ./...`

Manual/local QA after implementation:

- Start server locally.
- `curl /health`
- `curl /metrics`
- `curl /chat`
- `curl /chat/stream` if SSE is implemented.

## Failure Modes Registry

| Failure | Severity | Test requirement | Handling |
| --- | --- | --- | --- |
| Empty request reaches router | High | Orchestrator validation tests | Return validation error before routing |
| Router error crashes request | High | Router error fallback test | Emit error event and route to persona fallback |
| Agent not found returns 500-like mystery | High | Missing agent test | Return safe assistant result with `agent_not_found` metadata |
| Agent error leaks internal detail | High | Agent error test | Safe assistant message plus structured metadata |
| Context cancellation ignored | High | Cancellation/deadline test | Return context error and emit canceled/timeout event |
| Panic kills server process | Critical | Panic recovery test | Recover at orchestrator boundary and emit runtime error |
| Runtime events lack correlation | Medium | Event correlation test | Include request, conversation, tenant, and user IDs |
| Concurrent requests share metadata | High | Concurrent isolation test | Clone metadata and avoid shared mutable request state |
| Auth disabled accidentally in production-like config | Medium | Config + middleware tests | Auth is explicit config; docs explain default |
| Rate limit not enforced | Medium | Over-limit middleware test | Return 429 with structured error |
| SSE leaks goroutine on disconnect | High | Cancellation test | Stop streaming when request context is done |
| README claims Phase 4 capabilities | Medium | Doc grep test | Describe only runtime/API capabilities |

## Error And Rescue Registry

| Error class | User/developer impact | Required rescue |
| --- | --- | --- |
| Validation error | Caller sent bad conversation payload | Error includes field, problem, and expected shape |
| Routing error | Intent cannot be classified | Fallback to persona intent when context is valid |
| Agent lookup error | No agent handles the intent | Safe response with missing intent metadata |
| Agent execution error | Expert capability failed | Safe response with failed agent and cause category metadata |
| Auth error | Request lacks valid API key | HTTP 401 with short problem/cause/fix message |
| Rate limit error | Client exceeded configured quota | HTTP 429 with retry guidance if available |
| SSE disconnect | Client closed stream | Stop event loop without logging as server failure |
| Config error | Server/CLI cannot start | Exit non-zero with config key and corrective hint |

## DX Review

Developer persona: a Go contributor wiring runtime behavior or consuming the local API.

Developer journey:

| Stage | Expected path | Plan requirement |
| --- | --- | --- |
| Discover | Read README and Phase 3 docs | README links spec/design/plan |
| Configure | Edit `configs/app.yaml` or env | Config docs list API key, rate limit, timeout |
| Run CLI | `go run ./cmd/cli ask "hello"` | One-shot path works without external credentials |
| Run server | `go run ./cmd/server` | Logs address and config errors clearly |
| Verify health | `curl /health` | Copy-paste command in README |
| Call chat | `curl /chat` | Copy-paste JSON example |
| Stream events | `curl /chat/stream` | Stable event names documented |
| Debug | Read structured errors/events | Problem/cause/fix pattern |
| Extend | Add runtime hook or endpoint | Package boundaries keep runtime separate from transport |

TTHW target after Phase 3: under 5 minutes from clone to local `/health` and one deterministic `/chat` response.

DX scorecard:

| Dimension | Score | Reason |
| --- | --- | --- |
| Getting started | 8/10 | One-shot CLI and curl examples make the runtime easy to try. |
| API/CLI naming | 8/10 | `/health`, `/metrics`, `/chat`, `ask` are predictable. |
| Error messages/debugging | 8/10 | Plan requires problem/cause/fix style errors and runtime events. |
| Documentation/learning | 8/10 | Spec/design/plan plus ADR should explain decisions. |
| Upgrade path | 7/10 | API remains small; versioning can wait until Phase 4/5. |
| Developer environment | 8/10 | No external services required. |
| Testing support | 9/10 | Every runtime surface has package-level and e2e tests. |
| Feedback loops | 8/10 | CLI, HTTP, and events provide fast validation loops. |

Overall DX: 8.0/10.

## Review Scores

| Review | Result |
| --- | --- |
| CEO | Hold scope. Runtime/API is required, but defer REPL, gRPC, real providers, and persistence. |
| Design | Skipped. No user-facing UI or visual surface in Phase 3. |
| Eng | Approved with sequencing: runtime events/state, orchestrator, bootstrap, CLI/HTTP, middleware, SSE, e2e. |
| DX | Approved with requirements for copy-paste CLI/curl examples and actionable errors. |

Cross-phase theme: keep the runtime honest. It should expose real local orchestration and API behavior without claiming Phase 4 avatar/voice/UI behavior.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Keep Phase 3 as runtime/API only | Mechanical | Hold scope | Satisfies M6/M7 while avoiding Phase 4 product surface | Build Web UI/avatar now |
| 2 | CEO | Include HTTP `/chat` and CLI one-shot | Mechanical | Choose completeness | Both are needed for local verification and future UI integration | CLI-only vertical slice |
| 3 | CEO | Defer gRPC | Mechanical | Simpler over clever | HTTP/SSE covers the immediate integration need | Add gRPC now |
| 4 | CEO | Defer CLI REPL | Taste | Bias toward action | One-shot CLI is easier to test and enough for TTHW | Build REPL now |
| 5 | Eng | Put orchestration in `internal/runtime` | Mechanical | Explicit boundaries | Runtime owns lifecycle/events; server owns HTTP | Handler-level orchestration |
| 6 | Eng | Stream SSE runtime events first | Mechanical | Tell the truth | Current agents are not token-streaming providers | Fake token streaming |
| 7 | Eng | Use API key auth, not JWT | Mechanical | Simpler over clever | Minimal auth boundary with lower implementation risk | JWT in Phase 3 |
| 8 | Eng | Use in-memory rate limiting | Mechanical | Local-first | No Redis or external dependency required | Distributed limiter |
| 9 | Eng | Default to stateless conversations | Taste | Respect constraints | User does not want SQLite; snapshots can be added later | Persist every conversation now |
| 10 | DX | Require copy-paste CLI and curl docs | Mechanical | Optimize feedback loops | TTHW target depends on direct local commands | Docs only as prose |

## Taste Decisions

**T1: CLI REPL**

Recommendation: defer REPL and implement one-shot `ask` first. A REPL is useful, but it adds terminal state, cancellation, and history concerns before the runtime is proven.

**T2: Conversation snapshots**

Recommendation: keep the runtime stateless by default. Add local snapshot persistence later only if debugging or e2e evidence shows it is needed.

## User Challenges

None. The plan preserves the user-approved spec and the current no-SQLite direction.

## Pre-Gate Verification

| Required output | Status |
| --- | --- |
| Premise challenge | Produced |
| What already exists | Produced |
| NOT in scope | Produced |
| Architecture diagram | Produced |
| Test diagram | Produced |
| Explicit test plan | Produced |
| Failure modes registry | Produced |
| Error/rescue registry | Produced |
| DX journey and scorecard | Produced |
| Decision audit trail | Produced |
| Cross-phase theme | Produced |

## Approval Gate

Stage 2 plan approval is required before Stage 3 BUILD.

Reply with `I approve the plan` to begin Superpowers TDD implementation.

