# Phase 1 Core Contracts and Infrastructure Plan

## Status

Stage 2 plan review draft. This plan depends on the approved Phase 1 spec and design:

- `docs/specs/phase-1-core-contracts-infrastructure.md`
- `docs/design/phase-1-core-contracts-infrastructure.md`

Do not start implementation until this plan is approved.

## Plan Summary

Phase 1 creates the contract-first foundation for the digital twin runtime. The work is deliberately limited to data contracts, interfaces, deterministic fakes, registries, retryable LLM client infrastructure, local filesystem storage, in-memory vector search, and memory primitives.

This phase must not implement persona behavior, router classification, expert agents, production APIs, real LLM calls, SQLite, or external database/vector/TTS/ASR providers.

## Review Scope

| Review | Result |
| --- | --- |
| CEO / scope | Hold the Phase 1 boundary. Add only local filesystem storage because the user explicitly rejected SQLite for now. |
| Design | Skipped for product UI because Phase 1 has no user-facing UI. Design concerns are limited to developer-facing APIs and docs. |
| Engineering | Proceed with TDD slices ordered by dependency: types, interfaces, fakes, registries, LLM, store, vector, memory. |
| DX | Treat future contributors as the first users. Keep package names obvious, examples minimal, errors actionable, and tests copy-paste runnable. |

## Premises Confirmed

| Premise | Decision | Rationale |
| --- | --- | --- |
| Contracts first | Accepted | Later phases need stable interfaces and fakes before behavior lands. |
| No external services in tests | Accepted | Phase 1 should be deterministic on a fresh machine. |
| No SQLite now | Accepted | User preference is local storage. File-backed storage gives persistence without database setup. |
| In-memory is not the main store | Accepted | In-memory store remains useful for fakes, but local filesystem storage is the real Phase 1 persistence path. |
| No Phase 2 behavior | Accepted | Persona, router classification, expert agents, and Skill library behavior belong in Phase 2. |

## What Already Exists

| Area | Existing files | Reuse |
| --- | --- | --- |
| Module/build | `go.mod`, `Makefile`, `scripts/dev.ps1`, `.github/workflows/ci.yml` | Use existing Go commands and Windows script for verification. |
| Config | `internal/config`, `configs/app.yaml` | Add optional local data dir config only if needed by store constructor tests. |
| Core errors | `internal/core/errors.go` | Reuse/widen typed errors for duplicate registration, not found, timeout, invalid input, and store failures. |
| Observability | `internal/observability` | Reuse logger/metrics hooks if implementations need instrumentation; do not duplicate. |
| Package skeletons | `pkg/types`, `internal/llm`, `internal/store`, `internal/memory`, `internal/runtime` | Replace placeholders with real contracts and tests. |

## Architecture

```text
pkg/types
  |
  +--> internal/core       Agent, Skill, Router, Orchestrator, registries
  |       |
  |       +--> internal/testutil fakes
  |
  +--> internal/llm        Client, retry decorator, OpenAI-compatible client
  |
  +--> internal/store      Store interface, local file store, in-memory fake,
  |                         VectorStore interface, in-memory vector store
  |
  +--> internal/memory     short-term window, long-term recall composition
  |
  +--> internal/runtime    EventBus interface and local implementation if needed
```

## Execution Slices

### Slice 1: Data Contracts

Files:

- `pkg/types/*.go`
- `pkg/types/*_test.go`

Implement `Message`, `Conversation`, `Intent`, `AgentResult`, `SkillResult`, `UserProfile`, `Tenant`, metadata aliases, roles, confidence helpers, and JSON tags.

Tests first:

- JSON round trip for every struct.
- Zero-value behavior for slices/maps.
- Role and confidence validation where helpers exist.

Done when:

- `go test ./pkg/types` passes.

### Slice 2: Core Interfaces and Errors

Files:

- `internal/core/*.go`
- `internal/core/*_test.go`

Implement documented `Agent`, `Skill`, `Router`, `Orchestrator` interfaces and typed errors for duplicate registration, not found, validation, timeout, and provider failure.

Tests first:

- Compile-time assertions with minimal local fakes.
- Error wrapping and `errors.Is` behavior.

Done when:

- `go test ./internal/core` passes.

### Slice 3: Shared Fakes

Files:

- `internal/testutil/*.go`
- `internal/testutil/*_test.go`

Implement hand-written fakes for Phase 1 interfaces. Fakes should be deterministic, thread-safe when recording calls, and configurable without sleeps or network calls.

Tests first:

- Fake records calls.
- Fake returns configured result/error.
- Concurrent call recording is safe where mutexes are used.

Done when:

- `go test ./internal/testutil ./internal/core` passes.

### Slice 4: Registries

Files:

- `internal/core/registry*.go`
- `internal/core/registry*_test.go`

Implement agent and skill registries. Registries register by stable name, reject duplicates, return deterministic sorted names, return typed not-found errors, and support agent lookup by intent through `CanHandle`.

Tests first:

- Register/get.
- Duplicate rejection.
- Deterministic list order.
- Not found.
- Intent lookup, including no match.

Done when:

- `go test ./internal/core` passes.

### Slice 5: LLM Contracts, Retry, and OpenAI-Compatible Client

Files:

- `internal/llm/*.go`
- `internal/llm/*_test.go`

Implement provider-neutral chat/embedding/summarization contracts, retry decorator, and OpenAI-compatible HTTP client for local fake-server tests. No real API calls.

Tests first:

- Retry success after transient error.
- Max attempts exhausted.
- Context cancellation and timeout.
- Fake server validates request body and authorization header.
- Response parsing for non-streaming and basic streaming chunks.

Done when:

- `go test ./internal/llm` passes.

### Slice 6: Local Store

Files:

- `internal/store/*.go`
- `internal/store/*_test.go`

Implement `Store`, local filesystem store, and in-memory fake store. Local store should use deterministic paths under a configurable data directory, safe path handling, JSON/JSONL encoding, and atomic full-document writes where practical.

Tests first:

- Save/get conversation.
- Append/list messages.
- Reopen store and read persisted data.
- Tenant/user scoping.
- Missing conversation.
- Corrupt file returns actionable error.
- Path traversal IDs are rejected or sanitized.

Done when:

- `go test ./internal/store` passes.

### Slice 7: Vector Store

Files:

- `internal/store/vector*.go`
- `internal/store/vector*_test.go`

Implement in-memory vector store with cosine similarity and deterministic tie ordering.

Tests first:

- Upsert and top-k search.
- Dimension mismatch.
- Empty result.
- Equal-score deterministic order.
- Invalid top-k handling.

Done when:

- `go test ./internal/store` passes.

### Slice 8: Memory

Files:

- `internal/memory/*.go`
- `internal/memory/*_test.go`

Implement short-term memory window and long-term memory composition using LLM summarizer/embedder contracts plus `VectorStore`. Keep the estimator deterministic; do not add tokenizer dependency.

Tests first:

- Short-term memory preserves system messages.
- Short-term memory trims newest viable messages within budget.
- Empty and too-small budgets.
- Long-term summarize, embed, upsert.
- Recall by semantic vector search using fakes.
- Summarizer/embedder/store failures do not write partial memory.

Done when:

- `go test ./internal/memory ./internal/store ./internal/llm` passes.

### Slice 9: Runtime Event Contract

Files:

- `internal/runtime/*.go`
- `internal/runtime/*_test.go`

Implement minimal `EventBus` contract and local implementation only if needed by fakes or memory orchestration tests. Otherwise keep this slice to interface and fake assertions.

Tests first:

- Subscribe/publish.
- Context cancellation.
- Multiple subscribers.
- No goroutine leak in close path if implementation exists.

Done when:

- `go test ./internal/runtime` passes.

### Slice 10: Integration Verification and Docs

Files:

- `README.md`
- `RELEASE_NOTES.md`
- relevant package docs

Update docs to describe implemented Phase 1 capabilities without claiming Phase 2 behavior.

Verification:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/server`
- `.\scripts\dev.ps1 test`
- `.\scripts\dev.ps1 build`
- `.\scripts\dev.ps1 lint`

`go test -race ./...` is expected to be unsupported on the current local `windows/386` environment and should be run on a supported platform or CI.

## Parallel Workstreams

| Stream | Slices | Can start after | Notes |
| --- | --- | --- | --- |
| Contracts | 1, 2 | Plan approval | Must land first. |
| Fakes and registries | 3, 4 | Slices 1-2 | Can be split across agents after interface names settle. |
| LLM | 5 | Slice 1 | Does not depend on store or memory. |
| Store/vector | 6, 7 | Slice 1 | Local store and vector can run in parallel after shared types settle. |
| Memory | 8 | Slices 5-7 | Depends on LLM and store contracts. |
| Runtime/docs | 9, 10 | Slices 1-8 | Runtime may stay thin if no caller needs more. |

## Test Matrix

| Component | Test file target | Required coverage |
| --- | --- | --- |
| Types | `pkg/types/*_test.go` | JSON round trips, zero values, validation helpers. |
| Core interfaces/errors | `internal/core/*_test.go` | Compile-time assertions, wrapping, typed errors. |
| Test fakes | `internal/testutil/*_test.go` | Configured return values, call recording, concurrency where applicable. |
| Registries | `internal/core/registry*_test.go` | Duplicate, not found, sorted list, intent match. |
| LLM retry | `internal/llm/retry*_test.go` | Success after retry, max attempts, cancellation, timeout. |
| OpenAI client | `internal/llm/openai*_test.go` | Fake server request shape, auth, response parse, streaming parse, non-2xx. |
| Local store | `internal/store/local*_test.go` | Persist/reopen, append/list, scoping, corrupt files, path safety. |
| Vector store | `internal/store/vector*_test.go` | Top-k, dimension validation, empty search, deterministic ties. |
| Short-term memory | `internal/memory/short*_test.go` | System preservation, trimming, empty input, tiny budget. |
| Long-term memory | `internal/memory/long*_test.go` | Summarize/embed/upsert, recall, partial failure safety. |
| Runtime EventBus | `internal/runtime/*_test.go` | Publish/subscribe, cancel, close semantics if implemented. |

## Failure Modes Registry

| Failure | Prevention or expected behavior | Test |
| --- | --- | --- |
| Duplicate registry entry | Reject with typed duplicate error. | Registry duplicate test. |
| Missing registry entry | Return typed not-found error with name. | Registry not-found test. |
| Real network accidentally used | All provider tests use `httptest.Server`; no API key required. | OpenAI fake-server tests. |
| Retry ignores cancellation | Retry decorator exits on context cancellation. | Retry cancellation test. |
| Local store path traversal | Reject or sanitize unsafe IDs before path creation. | Store path safety test. |
| Partial file write | Full-document saves use temp file and rename where practical. | Reopen persisted file test. |
| Corrupt persisted JSON | Return actionable decode error; do not panic. | Corrupt file test. |
| Vector dimension mismatch | Return validation error, do not mutate existing data. | Dimension mismatch test. |
| Equal vector scores reorder randomly | Tie-break by document ID. | Deterministic tie test. |
| Memory failure writes partial long-term data | Summarizer/embedder failures abort before upsert. | Long-term failure tests. |
| Phase 2 behavior slips in | Keep persona/router classifier/agent behavior out of scope. | Review diff and package tests. |

## DX Checklist

| Touchpoint | Requirement |
| --- | --- |
| New contributor starts | README points to spec, design, and this plan. |
| Running tests | Commands are explicit and Windows-friendly. |
| Understanding package roles | Package docs explain contract boundaries. |
| Debugging errors | Error messages include operation, cause, and safe context. |
| Local store inspection | File layout is deterministic and human-readable. |
| Mocking later phases | `internal/testutil` fakes are documented and reusable. |

## Implementation Tasks

- [ ] T1: Add `pkg/types` contracts and round-trip tests.
- [ ] T2: Add `internal/core` interfaces and typed errors.
- [ ] T3: Add `internal/testutil` fakes with compile-time assertions.
- [ ] T4: Add agent and skill registries.
- [ ] T5: Add `internal/llm` contracts, retry decorator, and OpenAI-compatible fake-server tested client.
- [ ] T6: Add local filesystem store and in-memory fake store.
- [ ] T7: Add in-memory vector store.
- [ ] T8: Add short-term and long-term memory.
- [ ] T9: Add or confirm minimal runtime event contract.
- [ ] T10: Update README and release notes for implemented Phase 1 reality.
- [ ] T11: Run final verification commands and document any local platform limits.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Hold Phase 1 to contracts and infrastructure only. | Mechanical | Explicit over clever | Prevents persona/router/agent behavior from leaking out of Phase 2. | Implementing early behavior. |
| 2 | CEO | Use local filesystem storage instead of SQLite. | User preference confirmed | Bias toward action | User explicitly does not want SQLite now; local files satisfy persistence without DB setup. | SQLite driver. |
| 3 | Eng | Implement with TDD slices ordered by dependency. | Mechanical | Completeness | Types and interfaces must stabilize before fakes, registries, and memory. | Big-bang implementation. |
| 4 | Eng | Keep in-memory store as fake, not primary persistence. | Mechanical | DRY / explicit | Avoids conflating tests with real local persistence. | Memory-only Phase 1 persistence. |
| 5 | DX | Keep docs and error messages part of acceptance. | Mechanical | DX is product quality | Later contributors need the contracts to be understandable without reading implementation internals. | Code-only delivery. |

## GSTACK REVIEW REPORT

### Scores

| Area | Score | Notes |
| --- | --- | --- |
| CEO scope | 9/10 | Scope is tight and user preference on storage is reflected. |
| Design | N/A | No product UI scope in Phase 1. |
| Engineering | 8/10 | Architecture is sound; main risk is interface sprawl, controlled by small package-owned interfaces. |
| DX | 8/10 | Good test and doc path; improve during implementation with package examples where useful. |

### Approval Gate

Plan is ready for approval. After approval, Stage 3 must use Superpowers TDD: RED, GREEN, REFACTOR for each implementation task.
