# ADR 0001: Phase 3 Runtime, HTTP/SSE, and Local-First Operation

## Status

Accepted for Phase 3.

## Context

Phase 3 wires persona, router, skills, and expert agents into a callable runtime. The project needs local verification through CLI and HTTP before Phase 4 builds the digital-human presentation layer.

The user has stated that SQLite should not be used at this stage. Tests must remain deterministic and must not require real external providers.

## Decision

- Keep orchestration transport-independent in `internal/runtime`.
- Keep HTTP concerns in `internal/server`.
- Use `cmd/cli ask` as the first CLI surface instead of a REPL.
- Expose `/health`, `/metrics`, `/chat`, and `/chat/stream`.
- Use SSE for runtime events and final message output, not real token streaming.
- Use optional API key authentication and in-memory rate limiting.
- Stay local-first: no SQLite, Redis, external vector DB, search provider, TTS, ASR, or avatar provider is required for tests.

## Consequences

- Phase 4 can consume a stable HTTP/SSE boundary without inheriting orchestration logic.
- Local development remains fast and cheap.
- SSE semantics are honest about current capabilities: runtime events first, real token/audio/avatar streaming later.
- API key auth and in-memory rate limiting are not a final production security model, but they provide a testable boundary for Phase 3.

