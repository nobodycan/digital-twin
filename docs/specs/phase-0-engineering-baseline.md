# Phase 0 Engineering Baseline Spec

## Context

Phase 0 turns `digital-twin` from planning documents into a buildable, testable Go project. The goal is not to implement digital human business capabilities yet; it is to establish the engineering baseline that every later Phase depends on.

## Current State

Before Phase 0, the repository contained planning documents only:

| Area | Current state before Phase 0 | Gap |
| --- | --- | --- |
| Go project | No `go.mod` or packages | Cannot compile or test |
| Directory structure | Described in `plan.md`, not present on disk | Later work has no stable layout |
| Configuration | Planned only | Runtime cannot load app settings |
| Observability | Planned only | No structured startup signal or metrics hook |
| Errors | Planned only | No domain error vocabulary |
| Developer commands | Planned only | No build/test/lint/run entrypoints |
| CI | Planned only | No automated verification |

## Proposed Change

Create the Phase 0 engineering baseline with these capabilities:

- Initialize a Go module for `github.com/nobodycan/digital-twin`.
- Create the planned top-level and internal package directories with buildable placeholder packages.
- Add developer commands for build, test, race test, lint, vet, run, and clean.
- Add a Windows PowerShell developer script equivalent for environments without `make`.
- Add default application configuration in `configs/app.yaml`.
- Implement `internal/config` with fixed-structure YAML loading and environment variable overrides.
- Implement `internal/observability` with structured logging, memory metrics, Prometheus text export, and a no-op trace hook.
- Implement `internal/core` domain errors and a generic `Result[T]` envelope.
- Add CI workflow that builds, tests, race-tests on supported CI architecture, runs golangci-lint, and runs `go vet`.
- Update README and release notes to reflect Phase 0 status.

## Acceptance Criteria

1. `go test ./...` passes.
2. `go build ./cmd/server` passes.
3. `go vet ./...` passes.
4. `go run ./cmd/server --config configs/app.yaml` emits a JSON structured startup log.
5. `scripts/dev.ps1 test`, `scripts/dev.ps1 build`, `scripts/dev.ps1 lint`, and `scripts/dev.ps1 clean` run on Windows PowerShell.
6. `Makefile` exposes `build`, `test`, `test-race`, `lint`, `vet`, `run`, and `clean`.
7. `make test` runs normal tests without forcing `-race`.
8. `make test-race` runs race tests on supported platforms.
9. Unknown config keys fail fast with a clear error.
10. Invalid server ports outside `1..65535` fail validation.
11. Config supports both unprefixed and `DIGITAL_TWIN_` environment variable overrides.
12. Server supports config path selection through `--config` and `DIGITAL_TWIN_CONFIG`.
13. Build artifacts such as `bin/` and `*.exe` are ignored and not left in the working tree after verification.
14. No business features from Phase 1+ are implemented.

## Testing Plan

| Layer | What | Expected |
| --- | --- | --- |
| Unit | `internal/config` default load, env overrides, unknown keys, invalid ports, comments and quotes | Pass |
| Unit | `internal/core` domain errors, wrapping, predicates, `Result[T]` | Pass |
| Unit | `internal/observability` JSON logger, memory metrics, Prometheus exporter, no-op tracer | Pass |
| Command | `go test ./...` | Pass |
| Command | `go build ./cmd/server` | Pass |
| Command | `go vet ./...` | Pass |
| Command | `go run ./cmd/server --config configs/app.yaml` | Emits structured JSON startup log |
| Command | `go test -race ./...` | Pass on supported platforms; documented as unsupported on `windows/386` |

## Files Reference

| File or area | Change |
| --- | --- |
| `go.mod` | Defines Go module and language version |
| `cmd/server` | Minimal startup command that loads config and logs startup state |
| `cmd/cli` | Placeholder CLI entrypoint |
| `configs/app.yaml` | Default runtime config template |
| `internal/config` | Config loading, validation, env overrides, tests |
| `internal/observability` | Logger, metrics, Prometheus exporter, trace hook, tests |
| `internal/core` | Domain errors, `Result[T]`, tests |
| `Makefile` | POSIX developer commands |
| `scripts/dev.ps1` | Windows PowerShell developer commands |
| `.github/workflows/ci.yml` | CI verification workflow |
| `.golangci.yml` | Linter configuration |
| `README.md` | Phase 0 status and developer entrypoints |
| `RELEASE_NOTES.md` | Phase 0 release notes |

## Out of Scope

- Agent, Skill, Router, Orchestrator, LLM, Memory, Store, VectorStore, TTS, ASR, HTTP API, gRPC, Web UI, deployment manifests, and real Prometheus/OpenTelemetry provider wiring.
- Adding external Go dependencies.
- Implementing application business behavior beyond a startup smoke path.

## Rollback Plan

Revert the Phase 0 commit. Since this Phase only adds project structure, local configuration, documentation, tests, and CI scaffolding, no data migration or runtime rollback is required.
