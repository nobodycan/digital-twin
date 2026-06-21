# Phase 4 Digital Human Experience and Admin Spec

## Context

Phase 4 turns the Phase 3 runtime API into a visible, configurable, and operable digital-human product surface. The system should no longer feel like only a chat API: users should see text, subtitle, voice placeholder, and avatar state events, while operators should be able to configure persona, memory, knowledge, tool permissions, and observe conversations.

This phase covers M8 and M9 from `plan.md`:

- M8.1 Avatar asset manifest.
- M8.2 TTS abstraction.
- M8.3 ASR abstraction.
- M8.4 Lip-sync and subtitle timeline.
- M8.5 Expression and motion state machine.
- M8.6 Realtime interaction protocol.
- M8.7 Interruption and half/full-duplex policy.
- M9.1 User-facing Web chat.
- M9.2 Voice and digital-human interaction surface.
- M9.3 Persona editor.
- M9.4 Memory management console.
- M9.5 Knowledge management console.
- M9.6 Tool permission configuration.
- M9.7 Conversation audit and operations dashboard.

## Current State

| Area | Current state | Gap for Phase 4 |
| --- | --- | --- |
| Runtime | Phase 3 has local orchestrator, HTTP `/chat`, and SSE `/chat/stream` | No product-specific presentation event envelope for avatar, subtitle, audio, and tool status |
| Presentation skills | Phase 2 has deterministic `tts_speak`, `asr_transcribe`, `avatar_state`, and `subtitle_timeline` skills | No first-class TTS/ASR client contracts, timeline model, or stream adapter |
| Avatar | No manifest or asset schema | No versioned avatar identity, state mapping, licensing metadata, or sample asset |
| Web UI | No `web/` app yet | No chat surface, voice mock flow, avatar panel, or stream renderer |
| Admin | No admin APIs or UI | No persona version publish flow, memory controls, knowledge upload workflow, tool policy editor, or audit dashboard |
| Storage | Local file store and memory/vector abstractions exist | No SQLite; Phase 4 should continue local-file-first unless explicitly revised |

## Proposed Change

Implement Phase 4 as a mock-first product slice with two linked tracks:

1. **Digital-human presentation foundation**: typed event protocol, TTS/ASR mocks, subtitle/speech timeline, avatar manifest, avatar state machine, and interruption semantics.
2. **Product and admin surface**: a user-facing Web chat/voice/avatar shell plus a minimal operator console for persona publishing, memory visibility/deletion, knowledge upload preview, tool permission editing, and conversation audit.

The first implementation should prove the product loop without real external voice or avatar providers. Real providers can be added after the protocol and UI behavior are stable.

## Scope

### Presentation Protocol

Add a transport-neutral presentation event model that can be delivered over Phase 3 SSE:

- `conversation_started`
- `user_text`
- `asr_partial`
- `asr_final`
- `assistant_text_delta`
- `subtitle`
- `audio_chunk`
- `avatar_state`
- `tool_status`
- `interrupted`
- `error`
- `done`

Each event must carry tenant, user, conversation, request, sequence, timestamp, and payload metadata. Event ordering must be deterministic in tests.

### Avatar Manifest and State

Add an avatar manifest format under `assets/avatar/` or `internal/presentation` test fixtures:

- Avatar ID, display name, version, type (`2d`, `live2d`, `3d`, `video`, `voice_only`).
- Asset references and content hash.
- Licensing and attribution metadata.
- Supported states and fallback state.
- Viseme/expression capabilities.

Add a small state machine for:

- `idle`
- `listening`
- `thinking`
- `speaking`
- `happy`
- `apologetic`
- `serious`
- `confused`
- `interrupted`
- `error`

State changes should be derived from runtime events, intent, agent result metadata, and safety/tool failures.

### TTS and ASR Contracts

Add provider-neutral interfaces:

- `TTSClient`: input text, voice, speed, emotion; output deterministic audio chunks or a local placeholder file/byte stream.
- `ASRClient`: input audio chunks or file; output transcript segments with confidence and timestamps.

The default implementation must be mock/local and deterministic. Tests must not require microphone hardware, browser permissions, cloud TTS, or cloud ASR.

### Speech Timeline

Add `SpeechTimeline` and subtitle generation:

- Segment assistant text into subtitle units.
- Associate each unit with start/end timestamps.
- Generate simplified viseme markers.
- Map each segment to avatar speaking state.

The first timeline may use rule-based duration estimates; professional lip-sync providers remain out of scope.

### Interruption Policy

Define interruption behavior for when the user speaks or sends text while the assistant is speaking:

- Stop current mock TTS stream.
- Emit `interrupted`.
- Cancel the active request context where possible.
- Preserve the prior conversation turn as interrupted metadata.
- Start a new request with the new user input.

Phase 4 should implement a testable mock interruption path before attempting true full-duplex audio.

### User Web Experience

Add a first usable Web surface, not a marketing page:

- Conversation list or current-session panel.
- Text input and send.
- Streamed assistant response.
- Citation/source display if present in result metadata.
- Error and retry states.
- Avatar panel using manifest/state events.
- Subtitle display.
- Mock voice controls for upload/record simulation if real microphone permissions are deferred.
- TTS playback from mock audio chunks or placeholder audio.

The UI should feel like a working product surface. It should not be a landing page.

### Admin Console

Add a minimal operator console with local-first persistence:

- Persona editor: draft, validate, publish, rollback.
- Memory manager: list, inspect metadata, delete/disable.
- Knowledge manager: upload local document, parse/preview chunks, mark ready, run citation test.
- Tool permissions: edit allowlist and simple approval policy per tenant/persona.
- Conversation audit: list recent conversations, status, latency, selected agent, safety/tool errors, and stream event summary.

Admin operations must be testable without external databases. Use local file storage or in-memory fakes, not SQLite.

## Non-Goals

- No real TTS/ASR provider requirement for tests.
- No realistic 3D avatar, Live2D rigging, or video avatar generation requirement.
- No SQLite, Postgres, Redis, object storage, or external vector database requirement.
- No production authentication system beyond Phase 3 API key boundary unless Stage 2 explicitly adds it.
- No billing, multi-region deployment, or SOC/compliance workflow.
- No full Phase 5 eval/cost governance implementation.

## Acceptance Criteria

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. Existing Phase 3 CLI and HTTP behavior remains compatible.
4. Presentation event tests prove ordered `subtitle`, `audio_chunk`, `avatar_state`, and `done` events.
5. Avatar manifest tests validate required fields, version, supported states, and fallback behavior.
6. TTS mock tests return deterministic audio chunks or placeholder audio metadata.
7. ASR mock tests return deterministic transcript segments with timestamps.
8. Speech timeline tests generate subtitles and simplified viseme markers from text.
9. Avatar state machine tests map listening/thinking/speaking/error/interrupted flows correctly.
10. Interruption tests prove an active mock stream can be cancelled and a new request started.
11. Web UI tests or browser QA prove a user can send text, receive streamed content, see subtitle/avatar state changes, and recover from an error.
12. Mock voice UI tests or QA prove a user can submit mock audio and see ASR/TTS/subtitle/avatar updates.
13. Persona admin tests prove draft, publish, rollback, and new-session effective persona behavior.
14. Memory admin tests prove deletion/disable prevents future recall.
15. Knowledge admin tests prove upload/parse/chunk preview and citation test with local storage.
16. Tool permission tests prove unauthorized tool use is rejected and authorized use is allowed.
17. Audit dashboard tests prove a conversation produces inspectable audit records.
18. README and release notes describe only real Phase 4 capabilities and keep real providers clearly marked as future work.

## Test Matrix

| Level | Scenario | Evidence |
| --- | --- | --- |
| Unit | Avatar manifest validation | Missing license/state/version rejected |
| Unit | TTS mock | Stable chunks, voice metadata, no provider call |
| Unit | ASR mock | Stable transcript segments and timestamps |
| Unit | Speech timeline | Text to subtitles, duration, viseme markers |
| Unit | Avatar state machine | Valid transitions and failure states |
| Unit | Interruption policy | Context cancellation and `interrupted` event |
| Integration | Runtime to presentation adapter | Phase 3 result becomes text/subtitle/audio/avatar events |
| HTTP/SSE | Stream contract | Ordered events with sequence IDs and final `done` |
| Web | Text chat | User sends text and sees streamed response |
| Web | Voice mock | Mock audio input produces transcript, response, audio, subtitle |
| Admin | Persona publish | New session uses published version |
| Admin | Memory deletion | Deleted memory no longer appears in recall |
| Admin | Knowledge upload | Document chunks preview and citation test pass |
| Admin | Tool policy | Unauthorized tool call rejected |
| Admin | Audit | Conversation visible with key metadata |

## Failure Modes

| Failure | Expected behavior |
| --- | --- |
| TTS mock fails | Emit `error`, keep text/subtitle visible, do not fail whole chat |
| ASR mock fails | Show transcript error and let user fall back to text input |
| Avatar manifest missing state | Fall back to `idle` and record validation warning |
| SSE disconnect | Stop streaming work and leave audit status as cancelled/interrupted |
| Persona draft invalid | Block publish and show validation errors |
| Knowledge parse fails | Mark upload failed and keep prior knowledge version active |
| Memory delete requested | Disable/delete locally and invalidate any cached recall state |
| Tool permission missing | Reject call before skill execution |

## Rollback

Phase 4 should be revertible as code and local files only. No schema migration or external provider state should be required. Local generated assets, uploads, and audit fixtures must live in clearly documented local directories that can be ignored or cleaned.

## Open Questions for Stage 2

1. Should the first Web app use Go-served static HTML/TypeScript or introduce a dedicated frontend toolchain?
2. Should Phase 4 build one combined Web app with user/admin routes, or split user and admin surfaces immediately?
3. Should mock voice use browser microphone capture in the first pass, or use upload/simulated chunks first?
4. Should persona/memory/knowledge admin endpoints be private API-only first, or UI-first with thin handlers?
5. Should Phase 4 include the open Phase 3 polish PR as a prerequisite if it has not landed?
