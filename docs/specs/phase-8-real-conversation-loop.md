# Phase 8 Real Conversation Loop Spec

Date: 2026-06-25

Status: Approved by the user on 2026-06-25

Mode: Open source / research

## Context

Phase 7 connected `PersonaAgent` to an OpenAI-compatible LLM provider and proved that
the backend can return a real generated answer. The next product gap is not provider
connectivity. It is the lack of a reliable conversation loop.

The current system has the pieces, but they are not connected end to end:

- `llm.Client.Stream` and `OpenAIClient.Stream` can receive provider chunks.
- `PersonaAgent.Run` still calls non-streaming `Chat`.
- `/chat/stream` waits for `Orchestrator.Handle` to finish and then sends the full
  answer as one `message_completed` event.
- `/experience/stream` adapts a completed answer into presentation events, so its
  `assistant_text_delta` is not a real model delta.
- The HTTP server does not load or append conversation history through `store.Store`.
- The Web client submits one user message per request and does not own durable history.

Phase 8 turns those disconnected capabilities into a testable, cancelable,
server-owned multi-turn conversation runtime.

## Goal

Deliver a reliable real-LLM conversation loop that:

1. preserves context across at least 10 turns;
2. emits safety-accepted text segments before provider completion;
3. cancels provider work when the client disconnects or explicitly cancels;
4. persists only complete, accepted conversation turns;
5. reports provider failures without mixing partial model output with a fallback answer;
6. remains deterministic in CI through fake clients and fake HTTP providers.

## User-Approved Scope Decisions

- The project remains an open-source/research system.
- Phase 8 must cover multi-turn context, real token streaming, cancellation, provider
  failure behavior, automated tests, and executable conversation scripts.
- Continue using local filesystem storage. Do not add SQLite or another database.
- Implement streaming first for `PersonaAgent`; autonomous Agent selection, tool
  planning, RAG answer generation, and voice-provider expansion remain out of scope.
- The server owns committed conversation history. Clients submit the new turn and
  conversation identity, not the complete trusted history.
- A canceled or incomplete assistant response is not appended to committed history.
- Provider fallback is allowed only before the first text delta. After output begins,
  failure ends the stream with an explicit error and no fallback text.
- CI must verify the 10-turn context chain with a deterministic fake provider rather
  than judging semantic quality through a paid model.

## Problem Statement

The current `/chat/stream` endpoint creates the appearance of streaming while sending
only a completed response. Each request is also effectively stateless unless the
client resends previous messages. This produces three user-visible failures:

- follow-up references such as "use the second option" may lose their meaning;
- the user waits silently for the full provider response instead of seeing progress;
- cancellation or provider failure has no precise partial-output contract.

The underlying problem is ownership. No single component currently owns the complete
transaction from loading history through committing the accepted assistant message.

## Recommended Approach

Add a runtime-level streaming conversation capability. The runtime coordinates
history loading, short-term windowing, routing, Agent streaming, guard validation,
event emission, cancellation, and atomic turn persistence. HTTP handlers translate
runtime events to SSE and do not implement conversation policy.

This is Approach B selected by the user.

## In Scope

### Streaming Contracts

- Add provider-neutral runtime stream events for:
  - request started;
  - route selected;
  - agent selected;
  - assistant text delta;
  - assistant message completed;
  - fallback selected;
  - canceled;
  - error;
  - done.
- Add a stream sink/callback contract with backpressure through returned errors.
- Add a streaming Agent capability without forcing every existing Agent to implement
  token streaming.
- Preserve the existing non-streaming `Handle` and `/chat` compatibility path.

### Server-Owned Conversation History

- Wire a `store.Store` implementation into the runtime/bootstrap.
- Continue using `store.LocalStore` for the server profile and `InMemoryStore` in tests.
- Treat `(tenant_id, user_id, conversation_id)` as the storage scope.
- Accept a turn request containing conversation identity, one logical turn ID, one
  attempt ID, plus exactly one new user message.
- Load existing committed history when the conversation already exists.
- Reject identity mismatches and malformed turns.
- Use `memory.ShortTermMemory` or an equivalent deterministic window before calling
  the model, while retaining the full committed transcript on disk.

### Persona Streaming

- Reuse `PersonaAgent` prompt construction, model transparency rule, persona check,
  and post-generation guard.
- Call `llm.Client.Stream` for configured LLM persona chat.
- Accumulate provider chunks into a candidate assistant message and release only
  safety-accepted segments.
- Require an explicit successful stream completion before committing the assistant
  message.
- Return usage metadata when the provider supplies it; absence of streaming usage is
  acceptable in Phase 8.

### Cancellation

- Propagate `http.Request.Context()` through runtime, Agent, and LLM client.
- Treat client disconnect and explicit cancellation as `context.Canceled`.
- Stop reading provider chunks promptly after cancellation.
- Record a canceled terminal outcome internally. Emitting a canceled wire event is
  best effort because a disconnected client is no longer writable.
- Do not append a partial assistant message after cancellation.
- The accepted user message may be persisted before generation so a later retry can
  be represented as a new attempt; the assistant side remains absent until success.

### Failure and Fallback Semantics

- Before the first text delta:
  - `fallback_to_local` may produce one complete safe local response;
  - `fail_closed` emits an error and terminates.
- A complete safe local fallback is an accepted assistant response. It is persisted
  first, then emitted as `message_completed` with fallback metadata.
- After the first text delta:
  - never append fallback text to the partial model output;
  - emit an error terminal event;
  - do not persist the partial assistant response;
  - record a safe error category without provider response bodies or secrets.
- Empty successful streams are treated as provider failures.
- Malformed provider chunks are treated as provider failures.
- Persona guard rejection after accumulation replaces the candidate with a safe
  fallback only if no accepted segment has been exposed. The streaming design must
  apply incremental checks and fully buffer requests whose required guard is not
  prefix-monotonic, including low-confidence uncertainty checks.

### HTTP and Web Experience

- `/chat/stream` emits real runtime SSE events and flushes after each event.
- `/experience/stream` consumes or adapts the same runtime deltas instead of
  regenerating a completed text delta.
- Web rendering appends deltas to one active assistant message instead of creating one
  transcript line per chunk.
- Add a visible stop/cancel command for an active generation.
- Keep `/chat` as the non-streaming compatibility endpoint.

### Verification Assets

- Add deterministic fake-stream tests with controlled chunks, blocking points,
  failures, and cancellation.
- Add a fake OpenAI-compatible SSE server test.
- Add a 10-turn conversation script that proves each request contains the committed
  prior context expected for that turn.
- Add a local DeepSeek smoke script that is opt-in and requires an environment-provided
  API key; it must not run in CI.
- Document startup, streaming smoke, cancellation, and fallback verification.

### Backward Compatibility

- `/chat` keeps its current request and response JSON: full `types.Conversation` in,
  `types.AgentResult` out.
- `/chat` remains stateless in Phase 8 and does not read or write server-owned
  conversation history.
- `/chat` retains its current status and error behavior and trusts the submitted
  transcript only for that request.
- `/chat/stream` adopts the new turn request and server-owned history behavior.
- Existing `/chat` clients require no migration during Phase 8.

## Out of Scope

- SQLite, Postgres, Redis, message queues, or distributed session ownership.
- Cross-process stream resume or replay after server restart.
- WebSocket migration; SSE remains the transport.
- Autonomous Agent or Skill selection by the LLM.
- Tool-call streaming or provider-native function calling.
- RAG-generated answers over uploaded knowledge.
- Long-term semantic memory redesign.
- Real TTS/ASR/avatar providers.
- Token-level safety classification through a separate paid model.
- Production billing, quotas, OAuth, or multi-region deployment.

## Conversation Ownership Rules

1. The client supplies `tenant_id`, `user_id`, `conversation_id`, `turn_id`,
   `attempt_id`, and one new user message.
2. The server loads existing history using all three identifiers.
3. The server appends the new user message once, using `turn_id` as the logical
   idempotency identity and the message ID as a content-integrity check.
4. The runtime creates a bounded model window from committed history.
5. The assistant candidate exists only in request-local memory during streaming.
6. A completed, accepted candidate is appended exactly once.
7. Canceled, failed, empty, or rejected partial candidates are never committed.
8. Concurrent turns for the same conversation are serialized or rejected with a
   conflict. Interleaved writes are not allowed.

### Turn and Attempt State

Each logical turn has one user message and zero or one accepted assistant message.
Attempts are operational records and are not inserted as duplicate user messages.

```text
new -> user_committed -> generating -> completed
                                  -> failed
                                  -> canceled
```

- Retrying a failed or canceled `turn_id` requires a new `attempt_id`, reuses the
  single committed user message, and may create the one missing assistant message.
- Reusing an `attempt_id` is rejected.
- Reusing a `turn_id` with different user content is rejected.
- A completed turn cannot create a second assistant message.
- A successful retry leaves exactly one user and one assistant message in history.
- Stage 2 must choose whether replaying a completed turn returns the prior result or a
  typed conflict, then test that behavior.

## Stream Event Contract

Every event must include:

- `request_id`;
- `conversation_id`;
- monotonically increasing `sequence`;
- event name;
- timestamp;
- payload;
- redacted metadata.

Required terminal behavior:

- exactly one of `message_completed`, `canceled`, or `error` describes the outcome in
  the runtime record;
- `done` is emitted at most once after the outcome event when the connection remains
  writable;
- no text delta is emitted after a terminal outcome;
- event sequence numbers never repeat or decrease.

## Safety Position

Streaming creates a conflict between immediacy and post-generation safety. A guard
that checks only the final answer cannot retract text already shown to the user.

Phase 8 uses safety-accepted segments, not raw provider chunks:

- normal-confidence, deterministic-low-risk persona chat may use a rolling segment
  buffer and release segments only after prefix-monotonic checks pass;
- low-confidence requests and any rule requiring whole-answer evidence are fully
  buffered until final guard approval;
- forbidden phrases split across provider chunks remain detectable through a retained
  suffix;
- Stage 2 must lock exact confidence/risk thresholds and buffer limits.

It is not acceptable to stream arbitrary provider text and guard it only after display.

## Acceptance Criteria

### Multi-Turn Context

- A deterministic 10-turn script uses one conversation ID.
- Turn 10 receives the committed messages required to resolve references established
  in earlier turns.
- The client sends only the new user message on each turn.
- Restarting the server with the same local data directory preserves committed
  history.
- Local storage provides namespace isolation by tenant/user/conversation identifiers.
  Phase 8 does not claim authorization isolation because API keys are not bound to
  tenant or user identities.

### Real Streaming

- A fake provider emits at least three chunks with controlled delays.
- `/chat/stream` exposes at least one safety-accepted segment before provider
  completion for a low-risk request.
- The HTTP writer is flushed after each SSE event.
- The final committed assistant message equals the ordered concatenation of accepted
  segments.
- `/experience/stream` emits matching incremental text events.

### Cancellation

- Cancel before the first chunk terminates the provider request and persists no
  assistant message.
- Cancel after one or more chunks terminates the provider request and persists no
  partial assistant message.
- Client disconnect records cancellation internally; a canceled SSE event is best
  effort only.
- The Web client renders canceled state locally when `AbortController` raises
  `AbortError`.
- No goroutine or response-body leak remains after cancellation.
- A later retry can complete the conversation normally.

### Failure Semantics

- Failure before the first chunk follows configured fallback policy.
- Failure after the first chunk emits an error without fallback text.
- Empty stream, malformed chunk, timeout, and provider non-2xx responses are covered.
- EOF without an explicit provider completion marker is a truncated-stream failure.
- A successfully persisted pre-delta local fallback appears in later context.
- Logs, SSE metadata, audit records, and stored messages contain no API key or raw
  provider error body.

### Compatibility and Quality

- Existing `/chat` behavior remains compatible.
- Existing local/mock startup remains credential-free.
- Existing runtime, server, presentation, and Web tests continue to pass.
- `go test ./...`, `go test -race ./...`, lint, and smoke checks pass.
- CI performs no real DeepSeek or paid-provider call.
- The model window always includes the current user message, retains a contiguous
  suffix of complete turns, and has explicit CJK and oversized-message tests.
- LocalStore distinguishes conversation-not-found from corruption, permission, and
  decoding failures; only explicit not-found may create a conversation.

## Research Questions for Stage 2

- Should streaming be exposed through optional interfaces such as `StreamingAgent`
  and `StreamingOrchestrator`, or should the existing core interfaces be generalized?
- The Phase 8 deployment boundary is one server process with one shared LocalStore
  instance. Stage 2 must choose an in-process scoped lock or local version/CAS design
  within that boundary; cross-process correctness is out of scope.
- What deterministic incremental guard policy gives safe output without turning every
  response into full buffering?
- The user message is committed once before generation under its logical `turn_id`.
  Attempts are tracked separately so cancellation remains observable and retries do
  not duplicate the user message.
- Should `/experience/stream` directly consume runtime events or use a streaming
  presentation adapter layered over them?

## Distribution

Phase 8 ships through the existing Go server and Web application. No new binary or
package is introduced. The repository README and PowerShell scripts are the developer
entry point. GitHub Actions remains the release verification path.

## Assignment

Run Stage 2 `$gstack-autoplan` against this specification. It must lock the streaming
interfaces, storage transaction rules, incremental guard policy, concurrency model,
and a RED/GREEN test matrix before any production code is written.

## Gate

Stage 1 gate passed: the user approved this spec on 2026-06-25.
