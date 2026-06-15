# Phase 1 Core Contracts and Infrastructure Spec

## Context

Phase 1 turns the Phase 0 engineering baseline into a contract-first foundation for the digital twin runtime. Later work on Persona, Router, Agent, Skill, Orchestrator, and UI must be able to compile and test against these contracts without calling real LLMs, databases, vector stores, TTS, or ASR providers.

This Phase covers M1 and M2 from `plan.md`:

- M1.1 Base data contracts.
- M1.2 Core interfaces.
- M1.3 Infrastructure interfaces.
- M1.4 Mock implementations.
- M1.5 Agent and Skill registries.
- M2.1 LLM client abstraction and retry wrapper.
- M2.2 OpenAI-compatible chat client, tested with a local fake server.
- M2.3 Persistence storage layer.
- M2.4 In-memory vector store.
- M2.5 Short-term memory window.
- M2.6 Long-term memory summarization and semantic recall.

## Current State

Phase 0 already provides:

| Area | Current state | Gap for Phase 1 |
| --- | --- | --- |
| Module/build | `go.mod`, Makefile, PowerShell script, CI | No domain contracts yet |
| Types | `pkg/types/doc.go` only | No serializable message/conversation/intent/result structs |
| Core | domain errors only | No Agent/Skill/Router/Orchestrator interfaces |
| Infrastructure | package placeholders only | No LLM, memory, store, vector, event, TTS, ASR, safety, eval interfaces |
| Tests | Phase 0 unit tests | No contract, registry, retry, storage, vector, or memory tests |

## Proposed Change

Implement Phase 1 as a contract-first Go foundation with no external runtime services.

### Data Contracts

Add JSON-serializable structs under `pkg/types`:

- `Message`
- `Conversation`
- `Intent`
- `AgentResult`
- `SkillResult`
- `UserProfile`
- `Tenant`
- supporting aliases/constants for roles, intent names, confidence, and metadata.

All exported fields must include JSON tags. Tests must cover marshal/unmarshal round trips.

### Core Interfaces

Add interfaces under `internal/core`:

- `Agent`
- `Skill`
- `Router`
- `Orchestrator`

Interfaces must use `context.Context` and `pkg/types` contracts. They must be minimal and documented. Do not add Phase 2 persona behavior here.

### Infrastructure Interfaces

Add interfaces in their appropriate packages:

- `internal/llm`: `Client`, chat request/response structs, streaming callback type.
- `internal/memory`: `Memory`.
- `internal/store`: `Store` and `VectorStore`.
- `internal/runtime`: `EventBus`.
- `internal/observability` already exists and should be reused, not duplicated.
- `internal/core` or dedicated packages may define `TTSClient`, `ASRClient`, `SafetyGuard`, and `EvalRunner` if they are only contracts.

### Mocks

Add hand-written mocks/fakes in `internal/testutil` for all Phase 1 interfaces. Avoid generated mock tooling in Phase 1.

Mocks must be simple, deterministic, thread-safe where they store calls, and usable by later Phase tests.

### Registries

Add `AgentRegistry` and `SkillRegistry` under `internal/core` or a small `internal/registry` package. Registries must:

- Register by stable name.
- Reject duplicate names.
- Return clear not-found errors.
- List registered names in deterministic order.
- Support lookup by intent for agents.

### LLM Infrastructure

Implement:

- Retry wrapper with timeout/backoff settings.
- OpenAI-compatible `/chat/completions` client for non-streaming and basic streaming response parsing.
- Tests using `httptest.Server`; no real external API calls.

### Store Infrastructure

Phase 1 must not use SQLite. Implement local filesystem storage first, with an in-memory implementation kept for tests and lightweight fakes. The interface design should still allow SQLite/Postgres/object storage in later phases, but no database driver should be added now.

The store must support:

- Save/get conversation.
- Append/list messages.
- Basic tenant/user scoping through IDs already present in contracts.
- Configurable local data directory.
- Deterministic file layout by tenant, user, and conversation ID.
- Atomic write behavior for full-document saves where practical on the local filesystem.
- JSON or JSONL encoding with tests that read the persisted files back after reopening the store.

### Vector Infrastructure

Implement an in-memory vector store with cosine similarity:

- Upsert vector documents.
- Search top-k by vector.
- Reject dimension mismatches.
- Return deterministic ordering for equal scores.

### Memory Infrastructure

Implement:

- Short-term memory window that keeps system messages and the most recent messages within a budget.
- A simple token estimator based on rune or word count; no tokenizer dependency in Phase 1.
- Long-term memory that uses summarizer and embedder interfaces from `llm` and `VectorStore`; tests use mocks only.

## Acceptance Criteria

1. `go test ./...` passes.
2. `go build ./cmd/server` passes.
3. `go vet ./...` passes.
4. `pkg/types` round-trip JSON tests pass for all Phase 1 contracts.
5. Every Phase 1 interface has documentation comments.
6. Hand-written mocks exist for Agent, Skill, Router, Orchestrator, LLM, Memory, Store, VectorStore, and EventBus.
7. Registry tests cover registration, duplicate rejection, deterministic listing, not-found errors, and intent lookup.
8. LLM retry tests cover success, retry-after-failure, timeout/cancellation, and no real API calls.
9. OpenAI-compatible client tests verify request shape and response parsing through a local fake server.
10. Store tests cover conversation save/get, message append/list, reopening persisted local files, tenant/user scoping, and corrupt-file error handling.
11. Vector store tests cover top-k search, dimension mismatch, empty results, and deterministic ordering.
12. Short-term memory tests cover budget trimming and system-message preservation.
13. Long-term memory tests cover storing summarized facts and semantic recall using mocks.
14. No Phase 2 behavior is implemented: no persona prompt rendering, no real router classifier, no expert agent behavior, no Skill library behavior.
15. No real LLM/DB/vector/TTS/ASR provider calls are required for tests.

## Testing Plan

| Layer | What | Expected |
| --- | --- | --- |
| Unit | `pkg/types` JSON round trips | Pass |
| Unit | interface mock compile-time assertions | Pass |
| Unit | registries | Pass |
| Unit | LLM retry wrapper and OpenAI fake server | Pass |
| Unit | local filesystem store and in-memory fake store | Pass |
| Unit | in-memory vector store | Pass |
| Unit | short-term and long-term memory | Pass |
| Command | `go test ./...` | Pass |
| Command | `go build ./cmd/server` | Pass |
| Command | `go vet ./...` | Pass |
| Command | `go test -race ./...` | Pass on supported platforms; unsupported on local `windows/386` |

## Files Reference

| File or area | Change |
| --- | --- |
| `pkg/types` | Data contracts and tests |
| `internal/core` | Core interfaces and registries |
| `internal/testutil` | Hand-written mocks/fakes |
| `internal/llm` | LLM interfaces, retry wrapper, OpenAI-compatible client |
| `internal/store` | Store and vector interfaces plus local filesystem store, in-memory fake store, and in-memory vector implementation |
| `internal/memory` | Short-term and long-term memory implementations |
| `internal/runtime` | EventBus contract and in-memory implementation if needed |
| `docs/specs/phase-1-core-contracts-infrastructure.md` | This spec |
| `docs/design/phase-1-core-contracts-infrastructure.md` | Design rationale |

## Out of Scope

- Persona model, prompt rendering, persona guard, and intent classifier behavior.
- Expert Agent implementation.
- Skill library implementation beyond mockable contracts.
- HTTP `/chat`, `/metrics`, gRPC, CLI REPL, Web UI, deployment manifests.
- SQLite, Postgres, Qdrant, pgvector, OpenAI, TTS, or ASR provider integration.
- External mock generation tools.

## Rollback Plan

Revert the Phase 1 commit and remove the configured local data directory if test data was created. Phase 1 only adds contracts plus local/mock infrastructure, so no database migration rollback is required.
