# Phase 9 Experience and Provider Diagnostics Plan

Date: 2026-06-27

Status: Draft, waiting for user approval

Spec: [Phase 9 Spec](../specs/phase-9-experience-provider-diagnostics.md)

Design: [Phase 9 Design](../design/phase-9-experience-provider-diagnostics.md)

## Goal

Make the local digital-human experience credible and diagnosable by polishing `/app`
and making provider/fallback/error states explicit across backend events, Web UI,
and DeepSeek startup tooling.

## Scope

### In

- `/app` layout and styling refresh.
- Sanitized provider/runtime status.
- Provider error categories and stream completion tests.
- Fallback metadata and UI labeling.
- DeepSeek start/smoke script improvements.
- Docs updates for local provider troubleshooting.

### Out

- Real 3D/Live2D/video avatar.
- Real TTS/ASR provider integration.
- Auth/RBAC.
- New database.
- RAG/tool planning.
- Real DeepSeek calls in CI.

## Architecture Work Items

| ID | Area | Files | Outcome |
| --- | --- | --- | --- |
| P9-01 | Contracts | `pkg/types`, `internal/presentation` | Stream/completion metadata can carry generation and fallback state |
| P9-02 | Provider parsing | `internal/llm` | OpenAI-compatible stream failures are classified and redacted |
| P9-03 | Persona fallback | `internal/agents` | Fallback responses carry category and user-helpful copy |
| P9-04 | Server status | `internal/server`, `cmd/server`, `internal/config` | `/runtime/status` exposes sanitized config |
| P9-05 | Web state model | `web/app.js` | UI renders provider, fallback, error, and active turn states |
| P9-06 | Web polish | `web/app.html`, `web/app.css` | Professional responsive digital-human workspace |
| P9-07 | Scripts | `scripts/start-deepseek.ps1`, `scripts/smoke-conversation.ps1` | Stable local startup and provider smoke diagnostics |
| P9-08 | Docs | `README.md`, release notes if needed | Local DeepSeek troubleshooting is documented |

## TDD Execution Plan

### P9-01 Stream Metadata Contract

RED:

- Add tests proving presentation completion events preserve `generation_mode`,
  `fallback_category`, `llm_provider`, and `llm_model`.

GREEN:

- Extend event payload/metadata mapping with allowlisted generation fields.

REFACTOR:

- Keep helper functions small and avoid leaking arbitrary metadata.

Commands:

```powershell
go test ./internal/presentation ./pkg/types
```

### P9-02 Provider Failure Classification

RED:

- Add fake server tests for non-2xx, malformed chunk, empty stream, and EOF without
  `[DONE]`.
- Add a redaction assertion for API-key-shaped strings.

GREEN:

- Return typed provider failures with safe categories.
- Treat EOF without terminal marker as `provider_stream_truncated`, unless Stage 3
  introduces an explicit compatibility flag.

REFACTOR:

- Centralize safe provider error construction in `internal/llm`.

Commands:

```powershell
go test ./internal/llm
```

### P9-03 Persona Fallback Semantics

RED:

- Add tests for:
  - `fallback_to_local` before visible output returns labeled fallback metadata;
  - `fail_closed` returns an error;
  - Chinese user input receives helpful non-jarring fallback copy;
  - post-output provider failure does not append fallback.

GREEN:

- Update `PersonaAgent` fallback copy and metadata.

REFACTOR:

- Share fallback construction between `Run` and `Stream`.

Commands:

```powershell
go test ./internal/agents
```

### P9-04 Runtime Status Endpoint

RED:

- Add server tests for `GET /runtime/status`.
- Assert provider/model/fallback policy are visible.
- Assert API key and raw secrets are absent.

GREEN:

- Add handler and route.
- Pass sanitized config through server config.

REFACTOR:

- Keep status DTO minimal and stable.

Commands:

```powershell
go test ./internal/server ./cmd/server ./internal/config
```

### P9-05 Web State Rendering

RED:

- Add static/JS tests proving:
  - app fetches `/runtime/status`;
  - fallback metadata renders a fallback badge/status;
  - error after visible output marks partial text as not saved;
  - provider/model strip is populated without secrets.

GREEN:

- Update `web/app.js` state handling.

REFACTOR:

- Extract small DOM update helpers.

Commands:

```powershell
go test ./web
```

### P9-06 Web Visual Polish

RED:

- Add/extend static tests for required elements/classes:
  - provider strip;
  - status chip;
  - avatar state panel;
  - transcript message grouping;
  - mobile-safe composer structure.

GREEN:

- Update `web/app.html` and `web/app.css`.

REFACTOR:

- Remove stale demo wording and keep CSS organized by component.

Commands:

```powershell
go test ./web
```

Stage 5 QA will verify actual browser screenshots after implementation.

### P9-07 DeepSeek Scripts and Smoke

RED:

- Add PowerShell/static tests if existing repo pattern supports them, otherwise add
  Go/static tests that inspect script content for required output and safe behavior.
- Smoke script bad URL should produce non-zero status with sanitized diagnostic.

GREEN:

- Update `start-deepseek.ps1`.
- Update `smoke-conversation.ps1` to print provider-oriented diagnostics.

REFACTOR:

- Keep script defaults consistent with README.

Commands:

```powershell
go test ./...
```

Manual opt-in:

```powershell
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
.\scripts\smoke-conversation.ps1 -BaseUrl http://localhost:18080
```

### P9-08 Docs

RED:

- Add documentation checks with `rg` for required phrases.

GREEN:

- Update README with:
  - local DeepSeek startup;
  - fallback policy choice;
  - status endpoint;
  - troubleshooting table.

REFACTOR:

- Keep Phase status table accurate.

Commands:

```powershell
rg -n "Phase 9|runtime/status|fail_closed|fallback_to_local|DeepSeek" README.md docs scripts
```

## Test Matrix

| ID | Area | Scenario | Expected |
| --- | --- | --- | --- |
| T9-01 | Presentation | completion metadata contains `generation_mode=fallback` | Event preserves fallback label |
| T9-02 | LLM | fake success stream with `[DONE]` | Emits content and done |
| T9-03 | LLM | non-2xx provider body with secret | Returns provider failure with redacted cause |
| T9-04 | LLM | malformed SSE JSON | `provider_stream_decode` failure |
| T9-05 | LLM | no content before done | `provider_empty_response` path above agent layer or fallback below |
| T9-06 | LLM | EOF without `[DONE]` | `provider_stream_truncated` failure |
| T9-07 | Agent | pre-output provider error and fallback policy | Safe labeled fallback result |
| T9-08 | Agent | pre-output provider error and fail-closed policy | Error propagates, no fallback answer |
| T9-09 | Agent | post-output provider error | No fallback text appended |
| T9-10 | Server | `/runtime/status` local mode | Returns local provider info and no secrets |
| T9-11 | Server | `/runtime/status` DeepSeek config | Returns provider/model/fallback policy and no API key |
| T9-12 | Web | app startup | Fetches and renders provider strip |
| T9-13 | Web | fallback completion event | Shows fallback badge and status |
| T9-14 | Web | stream error after partial text | Marks line not saved |
| T9-15 | Web | mobile layout | Required responsive classes exist; Stage 5 browser QA verifies no overlap |
| T9-16 | Scripts | start script | Prints PID, app URL, provider/model, fallback policy, smoke command |
| T9-17 | Scripts | smoke bad URL | Non-zero exit with safe diagnostic |
| T9-18 | Full repo | all tests | `go test ./...` passes |

## Implementation Order

1. P9-01 metadata plumbing.
2. P9-02 provider failure classification.
3. P9-03 fallback semantics.
4. P9-04 runtime status endpoint.
5. P9-05 Web state rendering.
6. P9-06 visual polish.
7. P9-07 scripts and smoke diagnostics.
8. P9-08 docs.

This order avoids building UI states before backend metadata exists.

## Parallelization

Can parallelize after P9-01:

- Provider classification and server status can proceed independently.
- Web visual polish can proceed after the DOM structure decision is fixed.
- Script updates can proceed independently, but final docs should wait for script
  command shapes.

Do not parallelize P9-02 and P9-03 blindly if both touch error category naming.

## Review Findings Folded Into Plan

| Source | Finding | Resolution |
| --- | --- | --- |
| CEO | Scope could balloon into avatar/provider platform | Explicit non-goals and local-first slice |
| Design | Pretty page without state truth would reduce trust | Provider/fallback/error status is first-class UI |
| Eng | Browser must not infer provider policy | Backend owns error categories and metadata |
| DX | DeepSeek path must be self-diagnosing | Start/smoke scripts and README troubleshooting included |

## Acceptance Commands

```powershell
go test ./internal/llm
go test ./internal/agents
go test ./internal/presentation ./internal/server ./cmd/server ./web
go test ./...
rg -n "Phase 9|runtime/status|DeepSeek|fail_closed|fallback_to_local" README.md docs scripts
```

Manual QA after Stage 3:

```powershell
.\scripts\stop-server.ps1
$env:DIGITAL_TWIN_LLM_API_KEY="<your-key>"
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
.\scripts\smoke-conversation.ps1 -BaseUrl http://localhost:18080
```

Then open:

```text
http://localhost:18080/app
```

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Keep Phase 9 to experience + diagnostics | Auto-decided | Scope discipline | Solves visible trust issue without avatar/provider sprawl | Real avatar/voice expansion |
| 2 | Design | Polish `/app` as an operator workspace | Auto-decided | User trust | Current UI is too demo-like for a professional digital human | Landing page or marketing hero |
| 3 | Eng | Backend owns provider categories and sanitized metadata | Auto-decided | Separation of concerns | Prevents browser from guessing provider policy | UI-only fallback detection |
| 4 | DX | Make DeepSeek startup fail-closed during verification | Taste decision | Debuggability | Avoids fallback hiding real provider issues while testing | Always defaulting all local runs to fail-closed |

## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
| --- | --- | --- | --- | --- | --- |
| CEO Review | `$gstack-autoplan` | Scope & strategy | 1 | clear | Trust-focused slice accepted; avatar/provider expansion deferred |
| Design Review | `$gstack-autoplan` | UI/UX gaps | 1 | clear | `/app` must expose provider/fallback/error states, not only restyle |
| Eng Review | `$gstack-autoplan` | Architecture & tests | 1 | clear | Metadata and error taxonomy stay backend-owned with deterministic tests |
| DX Review | `$gstack-autoplan` | Developer experience | 1 | clear | DeepSeek start/smoke path must be copy-paste and diagnostic |

**VERDICT:** CEO + DESIGN + ENG + DX reviewed. Ready for Stage 3 after user approval.

NO UNRESOLVED DECISIONS

