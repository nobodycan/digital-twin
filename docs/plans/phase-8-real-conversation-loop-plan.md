# Phase 8 Real Conversation Loop Plan

Date: 2026-06-25

Status: Pending user approval

Source artifacts:

- `docs/specs/phase-8-real-conversation-loop.md`
- `docs/design/phase-8-real-conversation-loop.md`

## Summary

Phase 8 turns the existing LLM-backed persona path into a durable, genuinely streaming
conversation loop. The implementation remains local-file-first, single-process, SSE
based, and deterministic in CI.

The work is intentionally ordered from contracts to behavior:

1. define typed turn, attempt, and stream contracts;
2. harden Store not-found semantics and conversation windowing;
3. add a single-process conversation coordinator and scoped lock;
4. harden provider stream completion;
5. add PersonaAgent streaming with safety-accepted segments;
6. add runtime orchestration and terminal-event invariants;
7. wire HTTP, experience, and Web cancellation behavior;
8. add deterministic 10-turn, restart, race, and optional DeepSeek smoke verification.

No production code may be written until this plan is approved.

## Review Mode

- CEO: HOLD SCOPE.
- Design: interaction-state polish only; no visual redesign.
- Engineering: full review.
- DX: DX POLISH for OSS contributors and Go backend developers.

## Premise Challenge

1. The missing product capability is a conversation transaction, not another model
   adapter.
2. Server-owned committed history is necessary because client-owned transcripts are
   untrusted and cannot provide restart persistence.
3. Raw provider chunks cannot be displayed safely; the visible unit is a
   safety-accepted segment.
4. One active turn per conversation is the correct local-first constraint. Supporting
   concurrent turns inside one conversation would create ordering ambiguity without
   improving the target experience.
5. A user message must survive canceled generation so retry is observable, but it must
   not be duplicated across attempts.
6. Existing `/chat` compatibility is more valuable than forcing every caller onto the
   new durable contract in one Phase.

These premises are inherited from the approved specification.

## What Already Exists

| Capability | Existing implementation | Phase 8 reuse |
| --- | --- | --- |
| Provider-neutral LLM | `internal/llm.Client` | Reuse `Stream`; harden completion semantics |
| OpenAI-compatible SSE | `internal/llm/openai.go` | Parse DeepSeek/OpenAI-compatible chunks |
| Persona generation | `internal/agents/PersonaAgent` | Reuse prompt, transparency, fallback, metadata |
| Persona guard | `internal/persona/Guard` | Reuse final guard; add incremental segment guard |
| Runtime routing | `internal/runtime/Orchestrator` | Add an additive streaming path |
| Runtime events | `internal/runtime/RuntimeEvent` | Extend with sequence, timestamp, payload, outcomes |
| Local storage | `internal/store/LocalStore` | Persist conversation and typed turn records |
| Memory window | `internal/memory/ShortTermMemory` | Make the window turn-aware and CJK-safe |
| SSE helpers | `internal/server/server.go` | Reuse safe multiline encoding and add flushing |
| Presentation events | `internal/presentation` | Adapt accepted runtime deltas |
| Cancellation | request context and `InterruptionController` | Propagate request cancellation end to end |
| Web stream parser | `web/app.js` | Append accepted deltas to one active assistant node |
| Observability | metrics, runtime recorder, admin audit | Add stream and persistence outcome fields |
| Local provider scripts | `scripts/start-deepseek.ps1`, `stop-server.ps1` | Reuse startup and cleanup flow |

## NOT in Scope

- SQLite, Postgres, Redis, queues, or cross-process locking.
- WebSockets, reconnect replay, or stream resume.
- Autonomous Agent selection, tool planning, or provider-native function calls.
- RAG answer generation and long-term semantic-memory redesign.
- Streaming TTS, real ASR, or avatar-provider integration.
- Hosted sandbox, cloud deployment expansion, OAuth, RBAC, billing, or compliance.
- Database-grade power-loss durability.
- A visual redesign of `/app` or `/admin`.
- Real paid-provider calls in CI.

## Dream State Delta

Phase 8 does not complete the full digital-human vision. It closes the most important
credibility gap:

```text
Phase 7
  real model answer, one request at a time, completed-text pseudo-stream

Phase 8
  durable multi-turn history
  + visible accepted segments before completion
  + cancellation
  + retry/idempotency
  + explicit failure/fallback outcomes
  + restart continuity

Future
  RAG + tool planning + voice streaming + distributed session ownership
```

## Locked Architecture Decisions

| # | Decision | Selected option | Rationale |
| --- | --- | --- | --- |
| 1 | Interface evolution | Add optional streaming interfaces | Preserves existing `Agent`, `Orchestrator`, and fakes |
| 2 | Contract package | Put serializable turn/event types in `pkg/types`; streaming capability interfaces in `internal/core` | Avoids `agents` importing `runtime` and keeps dependencies acyclic |
| 3 | Conversation ownership | Add `internal/conversation.Coordinator` | One component owns history, attempt state, windows, and commits |
| 4 | Turn persistence | Add typed `TurnRecord` data to `types.Conversation` | Keeps transcript and attempt state in one atomic JSON document |
| 5 | Not-found | Add `core.ErrConversationNotFound` | Missing file must not be confused with corrupt or unreadable data |
| 6 | Concurrency | Scoped in-process keyed lock held for the turn | Simplest correct single-process ordering |
| 7 | Completed replay | Return the previously completed result with `replayed=true` | Makes network retries idempotent and developer-friendly |
| 8 | Failed retry | Same `turn_id`, new `attempt_id` | Reuses one user message and fills the missing assistant side |
| 9 | Safety | Segment buffering for normal-confidence low-risk chat; full buffering below confidence `0.5` or for non-prefix rules | Matches existing guard threshold and prevents unretractable unsafe text |
| 10 | Provider completion | Require explicit `[DONE]` | EOF without done is a truncated failure |
| 11 | Pre-delta fallback | Persist accepted fallback before completed event | Future turns must see the answer the user saw |
| 12 | Post-delta failure | Error terminal, no fallback, no assistant commit | Prevents mixed-source answers and false completion |
| 13 | `/chat` | Keep current stateless JSON contract unchanged | Avoids breaking existing callers |
| 14 | `/chat/stream` | Adopt typed turn request and durable server history | New capability gets a precise contract |
| 15 | `/experience/stream` | Reuse runtime stream through a streaming presentation adapter | Avoids a second generation path |
| 16 | User cancellation | `AbortController` closes the request; server cancellation event is best effort | A disconnected transport cannot reliably receive a terminal event |

## Architecture

```text
POST /chat/stream
       |
       v
+--------------------+      StreamEvent       +-------------------+
| HTTP SSE Sink      |<-----------------------| Streaming Runtime |
| encode + flush     |                        +---------+---------+
+--------------------+                                  |
                                                        |
                         +------------------------------+----------------+
                         |                                               |
                         v                                               v
              +----------------------+                         +------------------+
              | Conversation         |                         | Router + Agent    |
              | Coordinator          |                         | Registry          |
              +----------+-----------+                         +--------+---------+
                         |                                              |
          +--------------+--------------+                               |
          |                             |                               v
          v                             v                     +------------------+
+-------------------+        +-------------------+            | PersonaAgent     |
| Scoped Keyed Lock |        | ShortTermMemory   |            | Stream           |
+-------------------+        +-------------------+            +--------+---------+
          |                                                           |
          v                                                           v
+-------------------+                                        +------------------+
| LocalStore        |                                        | llm.Client       |
| one JSON document |                                        | Stream           |
+-------------------+                                        +--------+---------+
                                                                    |
                                                                    v
                                                           DeepSeek / fake SSE
```

Dependency direction:

```text
pkg/types
    ^
    |
internal/core <--- internal/store, internal/memory
    ^                    ^
    |                    |
internal/agents     internal/conversation
    ^                    ^
    +---------- internal/runtime
                         ^
                         |
                  internal/server
```

## Data Contracts

### Turn Request

Add to `pkg/types`:

```go
type TurnRequest struct {
    ConversationID string    `json:"conversation_id"`
    TenantID       string    `json:"tenant_id"`
    UserID         string    `json:"user_id"`
    TurnID         string    `json:"turn_id"`
    AttemptID      string    `json:"attempt_id"`
    Message        Message   `json:"message"`
    Metadata       Metadata  `json:"metadata,omitempty"`
}
```

Validation:

- all five IDs are non-empty and path-safe where applicable;
- `Message.Role == user`;
- message ID and trimmed content are non-empty;
- no client system/assistant/tool message is accepted;
- timestamps may be zero and are normalized server-side;
- request metadata is copied and never trusted for authorization.

### Turn Record

```go
type TurnStatus string
type AttemptStatus string

const (
    TurnOpen      TurnStatus = "open"
    TurnCompleted TurnStatus = "completed"
    TurnFailed    TurnStatus = "failed"
    TurnCanceled  TurnStatus = "canceled"
)

const (
    AttemptGenerating AttemptStatus = "generating"
    AttemptCompleted  AttemptStatus = "completed"
    AttemptFailed     AttemptStatus = "failed"
    AttemptCanceled   AttemptStatus = "canceled"
    AttemptAbandoned  AttemptStatus = "abandoned"
    AttemptReplayed   AttemptStatus = "replayed"
)

type TurnAttempt struct {
    ID          string        `json:"id"`
    Status      AttemptStatus `json:"status"`
    RequestID   string        `json:"request_id,omitempty"`
    ErrorCode   string        `json:"error_code,omitempty"`
    StartedAt   time.Time     `json:"started_at"`
    CompletedAt time.Time     `json:"completed_at,omitempty"`
}

type TurnRecord struct {
    ID                 string        `json:"id"`
    UserMessageID      string        `json:"user_message_id"`
    AssistantMessageID string        `json:"assistant_message_id,omitempty"`
    Status             TurnStatus    `json:"status"`
    Attempts           []TurnAttempt `json:"attempts"`
    Result             *AgentResult  `json:"result,omitempty"`
}
```

Add `Turns []TurnRecord` to `types.Conversation`. Old stored JSON remains readable
because the field is additive and optional.

The persisted document is a durable ledger. The model-visible window is a projection:

- include completed user/assistant pairs;
- include the current active user message;
- exclude failed, canceled, and abandoned user-only turns from later model context;
- retain those incomplete turns for retry, audit, and diagnosis;
- allow a new turn without requiring the user to retry a failed turn first.

### Stream Event

Extend or replace `runtime.RuntimeEvent` with a transport-neutral typed event containing:

- name/topic;
- request, tenant, user, conversation, turn, and attempt IDs;
- monotonic sequence;
- UTC timestamp;
- payload and safe metadata.

Required names:

- `request_started`
- `route_selected`
- `agent_selected`
- `assistant_text_delta`
- `fallback_selected`
- `message_completed`
- `canceled`
- `error`
- `done`

The event name is selected from constants only. User/model content is data, never an
SSE event name.

## Additive Interfaces

Add to `internal/core/interfaces.go`:

```go
type StreamSink interface {
    Emit(context.Context, types.StreamEvent) error
}

type AssistantDeltaSink interface {
    EmitAssistantDelta(context.Context, string) error
}

type StreamingAgent interface {
    Agent
    Stream(context.Context, types.Conversation, types.Intent, AssistantDeltaSink) (types.AgentResult, error)
}

type StreamingOrchestrator interface {
    Orchestrator
    Stream(context.Context, types.TurnRequest, StreamSink) (types.AgentResult, error)
}
```

Legacy Agents continue to implement only `Agent`. The runtime adapts their completed
result to a single `message_completed` event.

Only the runtime owns request, routing, terminal, and `done` events. Agents may emit
accepted assistant text only.

## Conversation Coordinator

Recommended API:

```go
type Coordinator struct {
    store  store.Store
    memory *memory.ShortTermMemory
    gate   *ActiveConversationGate
    clock  Clock
}

func (c *Coordinator) Begin(ctx context.Context, req types.TurnRequest, requestID string) (TurnSession, error)
func (c *Coordinator) Complete(ctx context.Context, session TurnSession, result types.AgentResult) error
func (c *Coordinator) Fail(ctx context.Context, session TurnSession, code string) error
func (c *Coordinator) Cancel(ctx context.Context, session TurnSession) error
```

`Begin`:

1. performs a non-blocking scoped gate acquisition;
2. loads or explicitly creates the conversation;
3. validates logical replay/conflict rules;
4. appends the user message once;
5. appends a generating attempt;
6. saves the document;
7. returns full conversation, bounded model window, and an unlock-bearing session.

The session must release its lock exactly once on all terminal paths.

The gate is an active-key set protected by one mutex. A busy key returns immediately
instead of waiting. Release deletes the key, including panic/cancel paths, so the map
does not grow without bound.

`Complete` saves the assistant message and completed state before the runtime emits
`message_completed`.

`Fail`/`Cancel` persist the terminal attempt without an assistant message.

On load, a persisted `generating` attempt that is not owned by the current process is
recovered as `abandoned` with error code `server_restart`. A new `attempt_id` may then
retry the same logical turn.

Terminal attempt persistence uses a detached bounded context because the request
context may already be canceled:

```go
persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
defer cancel()
```

### Completed Replay

If `Begin` sees a completed matching `turn_id`:

- same user content: reconstruct and return the prior `AgentResult`;
- different content: `ErrTurnConflict`;
- no provider call;
- stream `message_completed` and `done` with `replayed=true`.

`TurnRecord.Result` persists the safe `AgentResult` fields required for exact replay:
agent name, assistant message, confidence, and safe metadata.

## Store Changes

- Add `ErrConversationNotFound`.
- `LocalStore.GetConversation` maps only `os.IsNotExist` to not-found.
- Permission, read, and JSON errors remain `ErrStoreFailure`.
- `InMemoryStore` matches the same behavior.
- Coordinator performs read-modify-write under its own scoped lock.
- Do not rely on `Store.AppendMessage` for Phase 8 transactional behavior.
- Replace the fixed `.tmp` name with a unique same-directory temporary file.
- Treat file replacement as recoverable local persistence, not database-grade
  atomicity. Use flush/close plus a backup/recovery helper compatible with Windows
  replacement behavior.

## Short-Term Window

Replace independent-message packing with complete-turn suffix packing:

1. always include trusted system messages;
2. always include the current user message, even when it alone exceeds the budget;
3. walk earlier messages backward in complete user/assistant turn pairs;
4. stop at the first pair that does not fit; do not skip it and include older data;
5. preserve chronological order;
6. estimate CJK and no-space text by rune count rather than treating it as one token.

The persisted transcript is never truncated.

An oversized current message creates an explicit budget overflow for that request: it
is sent intact and no older turns are included. Phase 8 never silently truncates or
drops the active user message.

## Provider Stream Hardening

`OpenAIClient.Stream` tracks:

- `sawContent`
- `sawDone`
- callback errors
- scanner errors
- canceled context

Rules:

- `[DONE]` calls the terminal chunk callback once and returns success;
- EOF without `[DONE]` returns a typed provider failure;
- malformed JSON returns a typed provider failure;
- empty `[DONE]` stream is handled by PersonaAgent as empty response;
- callback error stops reading and propagates unchanged;
- cancellation closes the HTTP request through context.

Increase scanner buffer to a bounded documented maximum so one large provider event
does not fail at the default 64 KiB limit. Oversized events return a named provider
error.

Provider failures are classified and redacted at this boundary. Raw response bodies
must not appear in returned errors, runtime events, audit records, or ordinary logs.

## Persona Streaming and Safety

Share prompt construction, guard, fallback, and result helpers between `Run` and
`Stream`, while keeping provider calls separate: `/chat` continues to call
`Client.Chat`; streaming paths call `Client.Stream`.

### Normal-Confidence Path

Applies when intent confidence is at least `0.5` and deterministic pre-checks classify
the request as low risk.

- collect provider chunks;
- hold a rolling pending buffer;
- release only at complete sentence boundaries;
- if an unterminated sentence exceeds 4 KiB, switch the request to full buffering;
- cap the complete candidate at 256 KiB;
- run forbidden-claim checks before releasing;
- emit only accepted non-empty segments;
- run final guard over the complete candidate.

### Full-Buffer Path

Applies when:

- intent confidence is below `0.5`;
- pre-generation safety/persona checks flag uncertainty;
- the required rule depends on whole-answer evidence.

The complete candidate is buffered, guarded, then emitted in bounded segments.

### Guard Rejection

- before any accepted segment: persist and return the safe fallback when policy allows;
- after accepted output: emit error, persist failed attempt, no assistant message.

## Runtime Stream State Machine

```text
validating
    |
    v
loading_history
    |
    v
routing --> route_fallback
    |
    v
agent_running
    |
    +--> no_visible_output --> local_fallback --> committing --> completed
    |
    +--> streaming --> committing --> completed
    |        |
    |        +--> provider/guard/store error --> failed
    |        |
    |        +--> context canceled --> canceled
    |
    +--> completed-turn replay --> completed
```

Terminal invariants:

- one runtime terminal outcome: completed, canceled, or error;
- `done` at most once and only while sink is writable;
- no delta after terminal;
- `message_completed` only after persistence succeeds;
- accepted segment order equals final assistant content order;
- a sink error cancels provider work and produces an internal canceled/failed outcome.

Commit and transport semantics:

- sink failure before assistant commit: do not commit the assistant;
- assistant commit succeeds before `message_completed`;
- sink failure after commit does not roll back the assistant;
- a retry replays the committed result without calling the provider.

## HTTP Contracts

### `POST /chat`

Unchanged:

- accepts full `types.Conversation`;
- returns `types.AgentResult`;
- stateless;
- no durable-history side effect.

### `POST /chat/stream`

Accepts `types.TurnRequest`.

Preflight is completed before the first stream event: request validation,
non-blocking conversation gate acquisition, load/create, replay/conflict checks, and
user/attempt persistence. The SSE sink writes headers lazily on its first `Emit`.

Before SSE headers:

- invalid JSON: `400 invalid_json`;
- invalid turn: `400 invalid_turn`;
- active same-conversation request: `409 conversation_busy`;
- turn/content conflict: `409 turn_conflict`;
- orchestrator unavailable: `503 orchestrator_unavailable`.

After SSE headers, errors are terminal SSE events:

```json
{
  "problem": "provider stream ended before completion",
  "cause": "truncated_stream",
  "fix": "retry the same turn with a new attempt_id"
}
```

The handler calls `http.Flusher.Flush()` after every frame. Request cancellation or
sink failure stops runtime work.

### `POST /experience/stream`

Adopts the same turn request. A streaming presentation adapter:

- emits `conversation_started`;
- maps accepted runtime deltas to `assistant_text_delta`;
- moves avatar `thinking -> speaking`;
- emits subtitle/TTS only after completion;
- maps failure/cancellation to error/interrupted and idle terminal state.

## Web Interaction Specification

No layout redesign is required.

States:

```text
idle -> submitting -> thinking -> streaming -> completed -> idle
                       |             |
                       |             +--> canceling -> canceled -> idle
                       +-----------------> failed ----------> idle
```

Required behavior:

- keep a stable conversation ID for the browser session;
- generate one `turn_id` and one `attempt_id` per submit;
- create one assistant line, then append deltas into it;
- disable submit while the active turn owns the conversation;
- show a stop icon button only while active, with an accessible label and tooltip;
- use `AbortController.abort()` for stop;
- render `canceled` locally on `AbortError`;
- do not show `done` after error or cancellation;
- preserve partial visible text after cancellation, but label it canceled and never
  treat it as committed history;
- preserve partial visible text after provider or persistence failure, label it
  `not saved`, and never present it as completed history;
- treat `message_completed` as the visible success outcome; `done` is transport-only
  and creates no transcript line;
- show accepted local fallback with a distinct fallback badge;
- restore controls on all terminal paths;
- auto-scroll only when the user is already near the bottom.

Accessibility:

- status changes use a polite live region;
- stop button is keyboard reachable and at least 44px;
- focus returns to input after terminal state;
- state is not conveyed by color alone.

## Error and Rescue Registry

| Error code | Trigger | Caught by | User/wire outcome | Persistence | Test |
| --- | --- | --- | --- | --- | --- |
| `invalid_turn` | missing/invalid ID, role, or content | HTTP validation | 400 with problem/cause/fix | none | handler table test |
| `conversation_not_found` | no stored file | coordinator | create new conversation | new user turn | store/coordinator test |
| `store_failure` | permission, decode, write, rename | coordinator/runtime | SSE error or 500 before headers | no false completion | corruption/write tests |
| `conversation_busy` | active scoped lock | HTTP/runtime | 409 before SSE | none | concurrency test |
| `turn_conflict` | reused turn with changed content | coordinator | 409 before SSE | unchanged | idempotency test |
| `attempt_conflict` | reused attempt ID | coordinator | 409 before SSE | unchanged | attempt test |
| `truncated_stream` | EOF without `[DONE]` | LLM client | fallback before output, error after output | policy-dependent | fake SSE test |
| `malformed_chunk` | invalid provider JSON | LLM client | fallback/error by first-output state | policy-dependent | fake SSE test |
| `empty_stream` | done with no usable content | PersonaAgent | persisted fallback or fail closed | accepted fallback only | agent/runtime test |
| `provider_error` | non-2xx/network error | PersonaAgent/runtime | fallback before output, error after output | policy-dependent | fake server test |
| `provider_timeout` | deadline | runtime | canceled/error; no mixed fallback after output | no partial assistant | timeout test |
| `guard_rejected` | forbidden claim or final rule | PersonaAgent | safe fallback before output, error after output | no unsafe assistant | split-claim tests |
| `client_canceled` | AbortController/disconnect | runtime | local canceled UI; wire best effort | canceled attempt | cancellation test |
| `sink_failed` | response write/flush fails | runtime | connection ends | canceled/failed attempt | failing sink test |
| `persistence_failed` | final commit fails | runtime | error, never message_completed | no completed assistant | failing store test |
| `abandoned_attempt` | restart finds generating attempt | coordinator | mark abandoned and permit new attempt | ledger only | recovery test |

## Failure Modes Registry

| Failure mode | Severity | Detection | Required behavior |
| --- | --- | --- | --- |
| Unsafe phrase spans chunks | High | overlap guard test | phrase never emitted |
| Final-only uncertainty rule streams early | High | low-confidence test | full buffer |
| Provider sends data then closes without done | High | truncated fake server | error, no commit |
| Fallback appended to partial output | High | post-delta provider failure | prohibited |
| Assistant committed before store success | High | failing final save | no completed event |
| Duplicate user on retry | High | same turn/new attempt test | one user message |
| Two assistants for one turn | High | replay/concurrent tests | one assistant maximum |
| Store corruption treated as new chat | High | invalid JSON fixture | store failure, no overwrite |
| Same conversation requests interleave | High | blocking concurrency test | 409 or serialization |
| Different conversations block each other | Medium | concurrency timing test | independent progress |
| Disconnect leaks goroutine/body | High | cancel + goroutine/race test | provider exits promptly |
| SSE injection through content | High | multiline adversarial content | content remains data |
| Web displays one line per chunk | Medium | static/browser test | one accumulating line |
| Browser shows success after cancellation | Medium | AbortError test | canceled only |
| Window drops latest user message | High | oversized/CJK test | latest always retained |
| Keyed-lock map grows forever | Medium | repeated-key lifecycle test | unused entries removed |
| Stale generating attempt blocks after restart | High | restart recovery test | mark abandoned and permit retry |
| Failed/canceled user-only turn enters later prompt | High | model-window projection test | retain in ledger, exclude from prompt |
| Provider body leaks through errors | High | secret sentinel test | typed redacted error |

## Observability

Add metrics:

- `conversation_stream_requests_total{outcome}`
- `conversation_stream_time_to_first_delta_ms`
- `conversation_stream_duration_ms`
- `conversation_stream_segments_total`
- `conversation_stream_fallbacks_total{category}`
- `conversation_stream_cancellations_total{source}`
- `conversation_stream_abandoned_attempts_total`
- `conversation_persistence_failures_total{stage}`
- `conversation_busy_total`

Safe event/audit metadata:

- request, conversation, turn, attempt IDs;
- provider and model;
- generation mode;
- outcome and error category;
- segment count and latency;
- replayed flag.

Never include API keys, authorization headers, raw provider bodies, or full private
prompts.

Metric labels use bounded categories only. Request, conversation, turn, attempt, and
user IDs belong in structured logs/audit records, not metric labels.

Resource limits:

- HTTP turn request body: 128 KiB;
- one user message: 32 KiB;
- one provider SSE event: 1 MiB;
- accumulated assistant candidate: 256 KiB;
- no per-delta persistence and no per-delta append to the global `EventRecorder`.

## Design Review

Initial interaction completeness: 7/10.

Target after plan: 9/10.

| Dimension | Score | Plan treatment |
| --- | --- | --- |
| Information architecture | 9 | Existing conversation surface remains primary |
| Interaction states | 9 | Idle/thinking/streaming/cancel/error/completed specified |
| User journey | 9 | Stop, retry, and terminal recovery are explicit |
| AI slop risk | 9 | No new cards, hero, decorative UI, or explanatory clutter |
| Design-system alignment | 8 | Reuse existing styles; add only active assistant and stop states |
| Responsive/accessibility | 8 | 44px stop target, live region, keyboard/focus behavior |
| Unresolved design | 9 | No visual direction choice remains |

Mockups are not required: the plan changes interaction state inside the existing
conversation panel and does not change layout, visual hierarchy, branding, or page
composition.

## Developer Experience

### Target Persona

```text
Who:       OSS contributor or Go backend developer evaluating a local digital-human runtime
Context:   Clones the repo, wants local mode first, then optionally configures DeepSeek
Tolerance: Five minutes and a few copy-paste commands before trust drops
Expects:   No paid calls in tests, explicit env vars, observable SSE, actionable errors
```

### Developer Perspective

I clone the repository and want to see one real conversation without learning the
whole Agent architecture. Local mode should start without credentials, while DeepSeek
mode should require only an environment key and one script. I need a copy-paste stream
command that visibly produces multiple deltas, then a deterministic smoke script that
proves history and cancellation. When something fails, I need the response to tell me
whether the request was invalid, the conversation was busy, the provider stream was
truncated, or persistence failed, plus what ID to retry. I should never have to inspect
raw provider logs or guess whether a partial response was saved.

### Magical Moment

The first `scripts/smoke-conversation.ps1` run shows:

1. three or more text segments arriving before completion;
2. turn 2 correctly receiving turn 1 context;
3. a cancellation proving no partial assistant was committed;
4. a final PASS summary.

Target time to hello world:

- local deterministic stream: under 2 minutes;
- optional DeepSeek stream: under 5 minutes after obtaining a key.

### DX Scorecard

| Dimension | Before | Planned |
| --- | --- | --- |
| Getting started | 6 | 9 |
| API naming and defaults | 7 | 9 |
| Error messages | 6 | 9 |
| Documentation | 6 | 9 |
| Upgrade compatibility | 8 | 9 |
| Environment/tooling | 8 | 9 |
| Community/contribution | 6 | 8 |
| Measurement/feedback | 7 | 9 |

## TDD Implementation Slices

Every slice follows RED -> GREEN -> REFACTOR and ends with its focused verification
command before the next slice starts.

### P8-01 Typed Turn and Stream Contracts

RED:

- JSON round-trip and validation tests for `TurnRequest`, `TurnRecord`, attempts,
  statuses, and stream event fields.
- compile-time tests for additive streaming interfaces.

GREEN:

- add typed contracts and interfaces without changing existing `Agent`/`Orchestrator`.

Verify:

```powershell
go test ./pkg/types ./internal/core
```

### P8-02 Store Not-Found and Atomic Temp Files

RED:

- missing conversation returns `ErrConversationNotFound`;
- corrupt JSON and permission/read failure return `ErrStoreFailure`;
- temp writes use unique same-directory files and leave no stale file after success.

GREEN:

- add sentinel and align Local/InMemory stores.

Verify:

```powershell
go test ./internal/core ./internal/store
```

### P8-03 Turn-Aware Short-Term Window

RED:

- latest user message always retained;
- complete contiguous suffix only;
- CJK/no-space content consumes bounded budget;
- oversized latest message does not admit older messages.

GREEN:

- replace independent-message packing with turn suffix packing.

Verify:

```powershell
go test ./internal/memory
```

### P8-04 Conversation Coordinator and Keyed Lock

RED:

- create, begin, complete, fail, cancel;
- same turn/new attempt retry;
- completed replay;
- changed-content conflict;
- duplicate attempt conflict;
- same conversation conflict;
- different conversation parallel progress;
- lock entry cleanup;
- final commit failure.
- detached terminal persistence after request cancellation;
- stale generating recovery after restart;
- failed/canceled turns excluded from later model windows.

GREEN:

- add `internal/conversation` coordinator, session, clock, and keyed lock.

Verify:

```powershell
go test ./internal/conversation -race
```

Concurrency tests use channels and wait groups to prove provider exit, response-body
close, runtime return, and gate cleanup. They do not assert global goroutine counts.

### P8-05 OpenAI-Compatible Stream Completion

RED:

- delayed ordered chunks and explicit done;
- EOF without done;
- malformed chunk;
- empty done;
- callback failure;
- context cancellation;
- oversized SSE event.
- typed redacted non-2xx errors without raw provider body.

GREEN:

- harden scanner, completion tracking, and typed errors.

Verify:

```powershell
go test ./internal/llm -race
```

### P8-06 Incremental Persona Guard

RED:

- ordinary accepted sentence segments;
- forbidden claim inside one chunk;
- forbidden claim split across chunks;
- retained suffix correctness;
- low-confidence full buffering;
- final guard rejection before/after visible output.

GREEN:

- add incremental buffering/checking helper in `internal/persona`.

Verify:

```powershell
go test ./internal/persona
```

### P8-07 PersonaAgent Streaming

RED:

- prompt/history sent to `Client.Stream`;
- accepted segment ordering;
- model transparency without provider call;
- empty stream fallback;
- pre-delta provider fallback persisted by caller contract;
- post-delta provider error returns no fallback;
- cancellation propagates;
- metadata remains secret-free;
- `Run` and `Stream` share generation behavior.

GREEN:

- implement `StreamingAgent` for PersonaAgent and refactor common generation.

Verify:

```powershell
go test ./internal/agents -race
```

### P8-08 Streaming Runtime

RED:

- complete event order;
- monotonic sequence;
- exactly one terminal outcome;
- no delta after terminal;
- legacy Agent compatibility;
- completed replay;
- route fallback;
- sink failure cancellation;
- commit-before-completed invariant;
- canceled/failed attempt persistence.

GREEN:

- add `Orchestrator.Stream` using coordinator and optional StreamingAgent.

Verify:

```powershell
go test ./internal/runtime -race
```

### P8-09 Bootstrap and LocalStore Wiring

RED:

- one shared LocalStore instance is passed to coordinator/runtime;
- configured data directory survives runtime rebuild;
- local/no-secret runtime remains valid.

GREEN:

- extend `LocalRuntimeConfig` and server bootstrap.

Verify:

```powershell
go test ./internal/app ./cmd/server
```

### P8-10 HTTP SSE Contract

RED:

- new turn JSON validation and pre-header status codes;
- first accepted segment arrives before provider completion;
- flush per event;
- safe multiline data;
- post-header failures are SSE events;
- disconnect cancels provider;
- `/chat` JSON and side effects remain unchanged.

GREEN:

- add SSE sink and wire `StreamingOrchestrator`.

Verify:

```powershell
go test ./internal/server -race
```

### P8-11 Streaming Presentation

RED:

- accepted delta mapping;
- thinking -> speaking -> idle;
- completed subtitle/TTS;
- canceled/interrupted and error states;
- no TTS for failed partial output.

GREEN:

- add streaming presentation sink/adapter.

Verify:

```powershell
go test ./internal/presentation ./internal/server
```

### P8-12 Web Streaming and Stop

RED:

- static tests for AbortController, one active assistant node, stop button, stable
  conversation ID, turn/attempt IDs, and local canceled rendering;
- parser tests preserve segmented SSE.
- behavior tests for hidden `done`, fallback badge, and retained `not saved` partial
  text after cancellation/failure.

GREEN:

- update `web/app.html`, `app.js`, and existing CSS with minimal controls/states.

Verify:

```powershell
go test ./web
```

### P8-13 Deterministic Conversation Smoke

RED:

- fake-provider 10-turn history check;
- server restart continuity;
- cancellation creates no assistant message;
- retry completes one user/one assistant pair.

GREEN:

- add Go e2e test and `scripts/smoke-conversation.ps1`.
- support deterministic fake-provider verification by default and an opt-in
  `-DeepSeek` mode reading credentials only from the environment.

Verify:

```powershell
go test ./test/e2e -race
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-conversation.ps1
```

If the repository keeps e2e tests under existing packages instead of `test/e2e`, Stage
3 may place the Go test under `internal/server` while preserving the same test matrix.

### P8-14 Docs, Release Notes, and Optional DeepSeek Smoke

RED:

- documentation grep checks for new request body, stream events, retry semantics,
  cancellation, and no-real-provider CI policy.

GREEN:

- update README and release notes;
- document `start-deepseek.ps1`, `stop-server.ps1`, and smoke command;
- optional manual DeepSeek smoke reads API key only from environment.

Verify:

```powershell
rg -n "turn_id|attempt_id|assistant_text_delta|smoke-conversation|AbortController|DeepSeek" README.md RELEASE_NOTES.md scripts docs
```

## Test Matrix

| ID | Layer | Scenario | Expected result |
| --- | --- | --- | --- |
| T8-01 | Types | TurnRequest JSON | stable field round-trip |
| T8-02 | Types | invalid role/content/IDs | invalid input |
| T8-03 | Store | missing file | conversation not found |
| T8-04 | Store | corrupt JSON | store failure, no overwrite |
| T8-05 | Store | reopen | completed history preserved |
| T8-06 | Memory | 10-turn suffix | latest complete turns retained |
| T8-07 | Memory | CJK | budget is not one token |
| T8-08 | Memory | oversized current user | current user only, always retained |
| T8-09 | Coordinator | first turn | user committed once |
| T8-10 | Coordinator | success | one assistant, completed state |
| T8-11 | Coordinator | cancel | canceled attempt, no assistant |
| T8-12 | Coordinator | retry | same user, new attempt, one assistant |
| T8-13 | Coordinator | completed replay | prior result, no provider call |
| T8-14 | Coordinator | changed content | turn conflict |
| T8-15 | Coordinator | reused attempt | attempt conflict |
| T8-16 | Concurrency | same conversation | busy/conflict, no interleaving |
| T8-17 | Concurrency | different conversations | parallel progress |
| T8-18 | LLM | explicit done | ordered chunks, success |
| T8-19 | LLM | EOF without done | truncated failure |
| T8-20 | LLM | malformed JSON | provider failure |
| T8-21 | LLM | callback error | exact error propagated |
| T8-22 | LLM | cancellation | request exits promptly |
| T8-23 | Guard | split forbidden phrase | rejected before emission |
| T8-24 | Guard | confidence below 0.5 | full buffer |
| T8-25 | Agent | normal streaming | accepted segments + final result |
| T8-26 | Agent | pre-delta provider failure | safe fallback or fail closed |
| T8-27 | Agent | post-delta failure | error, no fallback |
| T8-28 | Agent | model identity | deterministic metadata response |
| T8-29 | Runtime | success event order | exact order and one done |
| T8-30 | Runtime | legacy Agent | completed compatibility |
| T8-31 | Runtime | sink failure | provider canceled, attempt terminal |
| T8-32 | Runtime | persistence failure | no message_completed |
| T8-33 | HTTP | delayed fake provider | delta observable before done |
| T8-34 | HTTP | validation/conflict | 400/409 before SSE headers |
| T8-35 | HTTP | error after headers | terminal SSE error |
| T8-36 | HTTP | multiline model content | no event injection |
| T8-37 | HTTP | `/chat` regression | request/response and stateless behavior unchanged |
| T8-38 | Experience | incremental mapping | matching text deltas |
| T8-39 | Experience | cancel/fail | interrupted/error, no TTS |
| T8-40 | Web | multiple chunks | one assistant line accumulates |
| T8-41 | Web | stop | AbortController and canceled state |
| T8-42 | E2E | 10 turns | final provider request contains required history |
| T8-43 | E2E | restart | next turn loads prior transcript |
| T8-44 | Security | metadata/logs | no API key or raw body |
| T8-45 | Race | concurrent streams | no data races or leaks |
| T8-46 | Recovery | restart during generating | abandoned; retry allowed |
| T8-47 | Context | failed/canceled historical turn | excluded from provider prompt |
| T8-48 | Security | provider body contains secret sentinel | absent from errors/logs/events |
| T8-49 | Runtime | sink fails after assistant commit | assistant remains replayable |
| T8-50 | HTTP | preflight conflict | 409 with no SSE headers |
| T8-51 | Web | done after terminal event | no visible success marker |
| T8-52 | Web | partial failed/canceled answer | retained and marked not saved |

## Files

Expected new files:

- `internal/conversation/coordinator.go`
- `internal/conversation/coordinator_test.go`
- `internal/conversation/lock.go`
- `internal/persona/stream_guard.go`
- `internal/persona/stream_guard_test.go`
- `internal/presentation/stream_adapter.go`
- `scripts/smoke-conversation.ps1`
- optional `test/e2e/conversation_stream_test.go`

Expected modified files:

- `pkg/types/contracts.go`
- `pkg/types/contracts_test.go`
- `internal/core/errors.go`
- `internal/core/errors_test.go`
- `internal/core/interfaces.go`
- `internal/core/interfaces_test.go`
- `internal/store/local.go`
- `internal/store/local_test.go`
- `internal/memory/short.go`
- `internal/memory/short_test.go`
- `internal/llm/openai.go`
- `internal/llm/openai_test.go`
- `internal/agents/experts.go`
- `internal/agents/experts_test.go`
- `internal/runtime/events.go`
- `internal/runtime/events_test.go`
- `internal/runtime/orchestrator.go`
- `internal/runtime/orchestrator_test.go`
- `internal/app/bootstrap.go`
- `internal/app/bootstrap_test.go`
- `internal/server/server.go`
- `internal/server/server_test.go`
- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/smoke/main.go`
- `cmd/smoke/main_test.go`
- `web/app.html`
- `web/app.js`
- `web/app.css`
- `web/app_static_test.go`
- `README.md`
- `RELEASE_NOTES.md`

The apparent file count is justified by cross-layer behavior. Implementation remains
incremental: no existing package is replaced, and each slice has a focused test gate.

## Parallelization Strategy

After P8-01 contracts land:

```text
Lane A: P8-02 Store -> P8-03 Memory -> P8-04 Coordinator
Lane B: P8-05 LLM stream -> P8-06 Guard -> P8-07 PersonaAgent

Barrier: A + B complete

Lane C: P8-08 Runtime -> P8-09 Bootstrap -> P8-10 HTTP
Lane D: P8-11 Presentation -> P8-12 Web

Barrier: C + D complete

P8-13 E2E -> P8-14 Docs
```

Suitable subagent split:

- Worker A owns `internal/store`, `internal/memory`, `internal/conversation`.
- Worker B owns `internal/llm`, `internal/persona`, `internal/agents`.
- Worker C owns `internal/runtime`, `internal/app`, `internal/server`, `cmd`.
- Worker D owns `internal/presentation`, `web`.

Workers must not overlap file ownership. Main agent owns shared contracts and final
integration.

## Verification

Focused checks run after every slice. Before Stage 4:

```powershell
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
go build ./cmd/server
go build ./cmd/cli
go build ./cmd/smoke
powershell -ExecutionPolicy Bypass -File .\scripts\verify_deploy.ps1
```

Manual local verification:

```powershell
$env:DIGITAL_TWIN_LLM_API_KEY="your-rotated-key"
powershell -ExecutionPolicy Bypass -File .\scripts\start-deepseek.ps1 -Port 8080
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-conversation.ps1 -Port 8080
powershell -ExecutionPolicy Bypass -File .\scripts\stop-server.ps1 -Port 8080
```

The previously exposed API key must be rotated; documentation must never contain it.

## Implementation Tasks

- [ ] **P8-01 (P1)** Typed turn, attempt, and stream contracts.
- [ ] **P8-02 (P1)** Explicit Store not-found and safe atomic replacement.
- [ ] **P8-03 (P1)** Turn-aware CJK-safe conversation window.
- [ ] **P8-04 (P1)** Coordinator, keyed lock, retry, and replay.
- [ ] **P8-05 (P1)** Explicit provider stream completion and cancellation.
- [ ] **P8-06 (P1)** Incremental and full-buffer persona guard.
- [ ] **P8-07 (P1)** PersonaAgent streaming path.
- [ ] **P8-08 (P1)** Runtime streaming state machine and invariants.
- [ ] **P8-09 (P1)** Shared LocalStore bootstrap wiring.
- [ ] **P8-10 (P1)** HTTP SSE contract and `/chat` compatibility.
- [ ] **P8-11 (P2)** Streaming presentation adaptation.
- [ ] **P8-12 (P2)** Web active-message and stop interaction.
- [ ] **P8-13 (P1)** Deterministic 10-turn/restart/cancel smoke.
- [ ] **P8-14 (P2)** README, release notes, and optional DeepSeek smoke.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Hold approved Phase 8 scope | Mechanical | Focus as subtraction | Completes the conversation loop without stacking RAG/tool uncertainty | Scope expansion |
| 2 | Eng | Add interfaces instead of replacing core contracts | Mechanical | Incremental over revolutionary | Preserves existing Agents and tests | Breaking interface rewrite |
| 3 | Eng | Typed turn state in Conversation JSON | Taste | Explicit over clever | One atomic document and inspectable local data | Untyped metadata / second file |
| 4 | Eng | Hold scoped lock during generation | Taste | Boring by default | Correct ordering in the declared single-process profile | Early CAS complexity |
| 5 | Eng | Replay completed turns | Taste | Fight uncertainty | Safe retries after lost connections | Always return conflict |
| 6 | Eng | Require provider `[DONE]` | Mechanical | Zero silent failure | EOF is not proof of completion | Treat EOF as success |
| 7 | Safety | Accepted segments, not raw chunks | Mechanical | Design for trust | Unsafe output cannot be retracted | Raw token pass-through |
| 8 | Safety | Full-buffer low-confidence output | Mechanical | Completeness | Existing guard requires whole-answer uncertainty evidence | Prefix-only checking |
| 9 | API | Keep `/chat` stateless | Mechanical | Compatibility | Avoids unrelated migration | Force durable contract everywhere |
| 10 | Design | No visual redesign | Mechanical | Subtraction default | Existing layout supports the new states | New page or component system |
| 11 | DX | Add one deterministic smoke script | Mechanical | Learn by doing | Demonstrates streaming/history/cancel in one command | Docs-only verification |
| 12 | Security | Namespace isolation only | Mechanical | Honest boundaries | API key is not tenant identity | Claim authorization isolation |

## Cross-Phase Themes

**Theme: completion must be explicit.** Product, engineering, safety, and DX all rely
on the same invariant: provider EOF, visible text, and persisted state are not
completion until `[DONE]`, guard approval, and Store success all occur.

**Theme: retry is a first-class user flow.** Turn/attempt identity appears in
architecture, error messages, Web cancellation, smoke tests, and observability.

**Theme: compatibility enables incremental adoption.** Additive interfaces and an
unchanged `/chat` path keep the Phase 8 change reversible.

## Deferred

- Add authenticated tenant/user identity binding before claiming authorization
  isolation.
- Add stream replay/reconnect only when a durable event log or database is introduced.
- Add streaming TTS after text streaming and cancellation are stable.
- Add RAG/tool streaming as separate SDD features using the runtime contract created
  here.

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | --- | --- | --- |
| CEO Review | `$gstack-autoplan` | Scope and strategy | 1 | CLEAR | HOLD SCOPE; no expansion required |
| Independent Spec Review | Stage 1 reviewer | Completeness and feasibility | 2 | CLEAR | 9 issues fixed; second pass 9/10 |
| Eng Review | `$gstack-autoplan` | Architecture and tests | 2 | CLEAR | Initial P1 findings resolved; 52-test matrix locked |
| Design Review | `$gstack-autoplan` | Interaction states | 1 | CLEAR | 7/10 to planned 9/10; no mockup required |
| DX Review | `$gstack-autoplan` | Developer workflow | 1 | CLEAR | Planned local TTHW under 2 min, DeepSeek under 5 min |

**VERDICT:** CEO + DESIGN + ENG + DX CLEARED. Ready for implementation after user approval.

NO UNRESOLVED DECISIONS
