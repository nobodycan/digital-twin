# Phase 9 Experience and Provider Diagnostics Design

Date: 2026-06-27

Status: Draft plan review complete, waiting for implementation approval

Source spec: [Phase 9 Experience and Provider Diagnostics Spec](../specs/phase-9-experience-provider-diagnostics.md)

## Review Summary

### CEO Review

The strongest product move is not a broad avatar expansion. It is trust recovery:
the user must know whether the answer came from the configured model, local
fallback, or a hard provider error.

Decision: hold scope to a reliability and experience slice. Do not add real voice,
3D avatar providers, RAG, billing, or auth in this phase.

### Design Review

The current page communicates "test harness". Phase 9 should make `/app` feel like
a focused local workbench for a professional digital human:

- transcript first;
- right-side presence/status panel;
- visible provider/model/fallback mode;
- labeled fallback/error states;
- stable responsive behavior.

Decision: redesign the existing static Web assets instead of introducing a front-end
framework.

### Engineering Review

The safest architecture is to keep policy in backend runtime/presentation boundaries
and keep the browser as a renderer:

- LLM client classifies provider stream failures.
- PersonaAgent tags fallback/generation metadata.
- Runtime/presentation stream events carry sanitized metadata.
- Server exposes sanitized runtime status.
- Web renders those states without inferring secrets or provider internals.

Decision: avoid pushing provider reasoning into `web/app.js`.

### DX Review

The DeepSeek local path must be copy-paste runnable and diagnosable. A developer
should not need to read Go code to know if the provider is configured.

Decision: improve PowerShell scripts and smoke output as part of the same phase.

## Architecture

```text
Browser /app
  | GET /runtime/status
  | POST /experience/stream
  v
internal/server
  | sanitized config/status
  | presentation SSE
  v
internal/presentation
  | maps runtime events to UI events
  v
internal/runtime Orchestrator
  | streams durable turn events
  v
internal/agents PersonaAgent
  | generation metadata + fallback category
  v
internal/llm OpenAI-compatible client
  | strict stream parsing + sanitized provider errors
  v
DeepSeek / fake OpenAI-compatible test server
```

## Data Flow

### Startup Status

1. `cmd/server` loads config.
2. `internal/server` receives provider metadata from config.
3. `/runtime/status` returns sanitized fields:
   - environment;
   - provider;
   - model;
   - fallback policy;
   - generation mode hint;
   - base URL host only, if useful, never credentials or API key.
4. `/app` fetches this status on load and renders it in the provider strip.

### Successful LLM Stream

1. `/app` posts the current user message to `/experience/stream`.
2. Runtime emits `request_started`, route, agent, deltas, `message_completed`,
   and `done`.
3. Presentation adapter forwards assistant deltas and completion metadata.
4. `/app` appends deltas into one assistant line and marks the turn as generated
   by the configured provider/model.

### Pre-Output Provider Failure

1. LLM client returns a typed provider failure before any accepted segment.
2. `PersonaAgent` applies `fallback_to_local` or propagates the error under
   `fail_closed`.
3. If fallback is used, result metadata includes:
   - `generation_mode=fallback`;
   - `fallback_category`;
   - `llm_provider`;
   - `llm_model`.
4. Presentation emits a labeled fallback event or completion metadata.
5. `/app` shows a fallback badge and an actionable status line.

### Post-Output Provider Failure

1. Some assistant text has already been emitted.
2. Provider then fails, stream truncates, or guard rejects final output.
3. Runtime emits an error, does not persist a final assistant answer, and marks
   partial browser text as not saved.
4. `/app` shows the partial line with an error state and no fallback text appended.

## Provider Error Taxonomy

| Category | Trigger | User-Facing Meaning |
| --- | --- | --- |
| `provider_status` | non-2xx response | Provider rejected or failed the request |
| `provider_network` | HTTP/network failure | Local process could not reach provider |
| `provider_stream_decode` | malformed SSE JSON | Provider stream was not parseable |
| `provider_stream_truncated` | stream closed without terminal marker | Provider ended before completion could be trusted |
| `provider_empty_response` | completed with no usable text | Provider returned no answer content |
| `provider_callback` | sink/backpressure error | Local streaming consumer stopped accepting data |

Only categories and redacted cause summaries may leave the backend. Raw provider
bodies, headers, and secrets stay server-side.

## UI Design Direction

The app should feel like a restrained operator console, not a landing page.

Layout:

- left: transcript and composer;
- right: digital-human presence panel with avatar state, provider strip, turn status,
  and voice/mock controls;
- top of transcript: compact session header with environment/provider status.

Visual language:

- neutral base with distinct accents for provider states;
- no one-note blue/purple theme;
- no decorative blobs or marketing hero;
- cards only for repeated messages or contained tool panels;
- 8px or smaller radius.

Expected states:

- `ready`: app loaded, no active request;
- `thinking`: request accepted, no visible output yet;
- `speaking`: accepted assistant text is streaming;
- `fallback`: local fallback answer is displayed;
- `error`: no trusted answer was produced;
- `interrupted`: user canceled active stream.

## Developer Experience

`scripts/start-deepseek.ps1` should be the stable entry point:

```powershell
.\scripts\stop-server.ps1
$env:DIGITAL_TWIN_LLM_API_KEY="..."
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
```

The script should print:

- server PID;
- app URL;
- provider;
- model;
- base URL;
- fallback policy;
- smoke command.

Smoke tooling should be opt-in and should fail loudly when the real provider is not
working. CI must use fake servers only.

## Risk Register

| Risk | Severity | Mitigation |
| --- | --- | --- |
| UI hides real provider failure behind a friendly fallback | High | Explicit fallback badge and status text |
| Strict stream completion rejects a provider that legitimately omits `[DONE]` | Medium | Decide in Stage 3 whether to add an explicit compatibility flag |
| Status endpoint leaks secrets | High | Allowlist fields and tests for secret redaction |
| Visual refresh causes layout regressions | Medium | Static tests plus browser QA in Stage 5 |
| Scripts kill unrelated processes | Medium | Stop by pid file or matching repo command line, not broad port killing |

## Non-Goals

- real avatar provider integration;
- real TTS/ASR provider work;
- authentication/RBAC;
- new database;
- provider SDK migration;
- CI calls to DeepSeek.

## Review Scores

| Review | Score | Notes |
| --- | --- | --- |
| CEO | 8/10 | Scope is sharp if it stays trust-focused |
| Design | 7/10 -> 8/10 target | Needs implementation screenshots to verify |
| Engineering | 8/10 | Main risk is stream completion compatibility |
| DX | 7/10 -> 9/10 target | Start/smoke scripts can make local use much clearer |

## Decisions

| # | Decision | Rationale |
| --- | --- | --- |
| 1 | Add sanitized runtime status | UI needs provider truth before first message |
| 2 | Keep static Web stack | Existing app is simple; a framework adds churn |
| 3 | Make fallback explicit in UI | Trust depends on distinguishing fallback from real model output |
| 4 | Classify provider failures in backend | Browser should render policy, not infer it |
| 5 | Use fake DeepSeek-like servers in CI | Deterministic, no paid provider dependency |

