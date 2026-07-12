# digital-twin

Planning and implementation repo for a local-first professional digital human system in Go.

## Status

Current stage: `Phase 27 - Admin Access Boundary`

What is already working:

- local-first chat runtime with durable conversation history
- persona agent with local mode, OpenAI-compatible provider mode, and fallback policy
- streaming `/chat/stream` and `/experience/stream`
- `/app` operator-facing digital human workspace
- `/admin` local operations console
- knowledge spaces with a default local scope and per-space document membership
- local knowledge document lifecycle: upload, list, inspect, disable, enable, delete, and reindex
- deterministic lexical knowledge retrieval with source metadata and citations
- retrieval diagnostics pipeline with lexical, vector, hybrid, and auto modes
- no-source and below-threshold grounding decisions with explainable ranking metadata
- grounded persona replies that can surface knowledge-space usage and citation summaries in `/app`
- summary-first Presence panel in `/app` with bounded latest-turn takeaway and trust signals
- knowledge-space health summaries, document quality detail, retrieval debug, and local knowledge-gap queues in `/admin`
- deterministic `knowledge_answer_state` metadata for grounded, partially supported, unsupported, provider fallback, guard-rejected, and local-mode turns
- knowledge workbench actions in `/admin`: investigate gaps, create local note documents, run gap-centered diagnostics, and resolve with evidence
- knowledge curation controls in `/admin`: filter documents by space, source type, status, gap linkage, and free-text query
- knowledge document detail with source-gap and resolved-gap relationships, plus in-place local editing for curated notes and uploaded text content
- local-first knowledge import jobs for text/Markdown files and pasted URL snapshots, with deterministic dedupe and import history
- knowledge review and activation controls: imported sources default to pending review, `/admin` exposes a review queue, and only review-active documents participate in normal retrieval
- deterministic `knowledge_evidence` metadata for grounded, partially supported, unsupported, provider fallback, guard-rejected, and local-mode turns
- compact evidence panels in `/app` with support state, source rows, bounded snippets, and operator next-action cues
- answer-trust inspection in `/admin`, including recent evidence-bearing turns and document quality flags
- answer audit timeline in `/admin`, including bounded question summaries, answer-state filters, source links, and weak-answer gap workflow cues
- knowledge repair inbox in `/admin`, with explainable priority, recurrence context, linked evidence, provider-free retest, and gap actions
- repair verification ledger in `/admin`, with durable verification history, deterministic snapshot/evidence fingerprints, and verified/stale/unverified state projection
- recurrence watch in `/admin`, with exact-match suspected recurrence records, bounded history, and human-confirmed reopen/dismiss actions
- `/runtime/status` for sanitized provider diagnostics
- DeepSeek-friendly local startup and smoke scripts

What is still intentionally out of scope in this repo:

- real 3D avatar or Live2D
- real TTS / ASR providers in CI
- user accounts, RBAC, billing, and durable admin sessions
- cloud deployment platform work
- SQLite or other DB migration in the current local-first slice

## Core ideas

- `Persona`: stable assistant identity with guardrails
- `Memory`: durable local conversation state and replay-safe attempts
- `Knowledge`: operator-managed local documents with retrieval and citations
- `Runtime`: router, agent registry, orchestrator, turn persistence
- `Provider boundary`: OpenAI-compatible LLM client with sanitized diagnostics
- `Experience`: SSE-driven web workspace with provider, fallback, and error visibility
- `Governance`: evals, decision records, audit-oriented admin surfaces

## Architecture

```mermaid
flowchart TD
    User["User / Operator"] --> Web["/app and /admin"]
    Web --> API["HTTP + SSE"]
    API --> Server["internal/server"]
    Server --> Runtime["orchestrator / turn coordinator"]
    Runtime --> Agents["persona / memory / knowledge / task / tool / safety"]
    Agents --> Skills["skills + guards + adapters"]
    Agents --> LLM["OpenAI-compatible client"]
    Runtime --> Store["local file storage"]
    Runtime --> Presentation["presentation adapter"]
    Presentation --> Web
    Server --> Status["/runtime/status"]
```

## Main endpoints

- `GET /health`
- `GET /ready`
- `GET /metrics`
- `GET /runtime/status`
- `GET /admin-access`
- `GET /admin/audit`
- `GET /admin/audit/timeline`
- `GET /admin/knowledge`
- `GET /admin/knowledge/health`
- `GET /admin/knowledge/{document_id}`
- `GET /admin/knowledge/{document_id}/detail`
- `GET /admin/knowledge/gaps`
- `GET /admin/knowledge/repairs`
- `GET /admin/knowledge/repairs/verifications`
- `GET /admin/knowledge/repairs/recurrences`
- `GET /admin/knowledge/imports`
- `GET /admin/knowledge/spaces`
- `POST /chat`
- `POST /chat/stream`
- `POST /experience/stream`
- `POST /experience/mock-voice/stream`
- `POST /admin/knowledge/upload`
- `POST /admin/knowledge/notes/create`
- `POST /admin/knowledge/import`
- `POST /admin/knowledge/review`
- `POST /admin/knowledge/update`
- `POST /admin/knowledge/gaps/update`
- `POST /admin/knowledge/repairs/retest`
- `POST /admin/knowledge/repairs/verify`
- `POST /admin/knowledge/repairs/recurrences/confirm`
- `POST /admin/knowledge/repairs/recurrences/dismiss`
- `POST /admin/knowledge/spaces/create`
- `POST /admin/knowledge/spaces/update`
- `POST /admin/knowledge/spaces/disable`
- `POST /admin/knowledge/spaces/enable`
- `POST /admin/knowledge/spaces/archive`
- `POST /admin/knowledge/disable`
- `POST /admin/knowledge/enable`
- `POST /admin/knowledge/delete`
- `POST /admin/knowledge/reindex`
- `POST /admin/knowledge/citation-test`
- `POST /admin/knowledge/retrieval-diagnostics`
- `GET /app`
- `GET /admin`

## Local quick start

Local deterministic mode:

```powershell
go run ./cmd/server
```

Then open:

- [http://localhost:8080/app](http://localhost:8080/app)
- [http://localhost:8080/admin](http://localhost:8080/admin)

DeepSeek via the OpenAI-compatible boundary:

```powershell
$env:DIGITAL_TWIN_LLM_API_KEY="your-api-key"
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
```

Then open:

- [http://localhost:18080/app](http://localhost:18080/app)
- [http://localhost:18080/admin](http://localhost:18080/admin)

Stop the tracked server:

```powershell
.\scripts\stop-server.ps1
```

## Admin access

The default `local` server on a loopback host keeps the admin console usable
without a key. Staging, production, unknown environments, and non-loopback
listeners require an admin credential before startup. Configure it with either
the YAML field below or the environment variable:

```yaml
server:
  admin_api_key: "set-a-secret-outside-source-control"
```

```powershell
$env:DIGITAL_TWIN_SERVER_ADMIN_API_KEY = "set-a-secret-outside-source-control"
```

For one migration phase, an existing `server.api_key` is accepted as the admin
fallback when `server.admin_api_key` is empty. When both are configured, the
dedicated admin key is required for `/admin/*`; the legacy key continues to
protect chat and experience routes. The browser keeps the entered admin key only
in page memory, so a reload requires entering it again. Use TLS and a real secret
in any shared environment; this project does not provide accounts or RBAC.

## Runtime status and fallback policy

`/runtime/status` returns sanitized session diagnostics for the web app and local operators.

Example fields:

- `environment`
- `provider`
- `model`
- `fallback_policy`
- `generation_mode_hint`
- `base_url`

Fallback policies:

- `fallback_to_local`: if the provider fails before usable output, return an explicit local fallback reply
- `fail_closed`: if the provider fails, surface the error and do not silently hide it behind a normal assistant answer

Recommended verification mode when testing DeepSeek:

```powershell
.\scripts\start-deepseek.ps1 -Port 18080 -FallbackPolicy fail_closed
```

## Smoke checks

Conversation and persistence smoke:

```powershell
.\scripts\smoke-conversation.ps1 -BaseUrl http://localhost:18080
```

The smoke script now:

- fetches `/runtime/status`
- prints a sanitized provider diagnostic
- runs two streaming turns plus one replay attempt
- verifies durable local conversation history

## Knowledge workflow

Phase 21 extends the local knowledge loop into an inspectable evidence, answer-trust, and audit-timeline workspace:

1. Start the server.
2. Open [http://localhost:18080/admin](http://localhost:18080/admin).
3. Use the default knowledge space or create a new one.
4. Use `Knowledge import` to ingest either local `.txt` / `.md` files or a pasted URL text snapshot into the selected space.
5. Check `Recent imports` to see imported, skipped, and failed sources for each job.
6. Use `Review queue` to inspect newly imported pending sources before activation.
7. Approve, reject, archive, or reactivate a source from document detail.
8. Filter the document table by source type, lifecycle status, review status, free-text query, or whether a document is linked to a knowledge gap.
9. Inspect document detail to see quality flags, source metadata, review state, and source/resolution relationships.
10. Edit a local document title, source label, or content in place when curation is needed.
11. Ask a related or unsupported question in `/app` with the same selected space.
12. Inspect the compact evidence panel below the assistant turn, including support state, source rows, bounded snippets, and next action.
13. Return to `/admin` and inspect the Answer timeline with filters for weak answers, source documents, or a single conversation.
14. Open a cited source from the timeline or jump to the gap workflow for unsupported and review-gated answers.
15. Move a gap to `investigating`, create a workbench note, and run diagnostics from the gap question.
16. Resolve the gap with an optional document ID and resolution note once evidence is visible.

When a turn completes, `/app` can now show:

- `Knowledge grounded (Space Name)`
- `Partially supported (Space Name)` when retrieval is below threshold
- `No supporting source (Space Name)` when the selected space cannot support the question
- `Provider fallback` or `Guardrail fallback` when the final answer is not a normal provider-backed grounded turn
- source citation chips
- evidence panels with reviewed source rows, snippets, diagnostics, and next-action cues
- `Memory considered` when memory metadata is present

Local verification for Phase 21:

```powershell
go test ./internal/knowledge ./internal/admin ./internal/server ./internal/agents ./internal/app ./web
go test ./...
go vet ./...
```

The retrieval pipeline is still local-first:

- lexical retrieval is always available
- vector retrieval is optional
- CI does not require DeepSeek, embeddings, or an external vector database
- `internal/knowledge/testdata` contains deterministic RAG eval fixtures

## Phase 21 highlights

Phase 21 focuses on making answer trust inspectable over time:

- `/admin` now exposes a bounded Answer Timeline powered by persisted audit records and `knowledge_evidence`
- timeline items include answer state, question summary, source count, top source links, diagnostics, and weak-answer action cues
- the existing Audit table remains available while timeline filters support weak-only, document, conversation, and bounded recent-history views

## Phase 20 highlights

Phase 20 focuses on making knowledge-backed answers inspectable without overclaiming truth:

- assistant turns now carry additive `knowledge_evidence` metadata alongside existing answer-state and citation metadata
- `/app` renders compact evidence panels with support summaries, source rows, bounded snippets, and review-gate-safe diagnostics
- `/admin` surfaces recent answer-trust evidence and deterministic document quality flags for local triage

## Phase 19 highlights

Phase 19 focuses on adding a trust gate between import and retrieval:

- imported knowledge now defaults to `pending_review` instead of becoming active immediately
- `/admin` exposes a compact review queue plus approve, reject, archive, and reactivate actions
- retrieval excludes non-active review states and returns explicit review-gated no-source reasons

## Phase 18 highlights

Phase 18 focuses on turning curation into a small but real ingestion loop:

- `/admin` now supports import jobs for local text/Markdown files and pasted URL snapshots
- imports persist job history in the local file-backed knowledge store, including imported document IDs plus skipped and failed sources
- ingestion is deterministic and local-first: no crawler, no remote fetch, no binary parser, and exact duplicate-content skipping inside a space

## Phase 17 highlights

Phase 17 focuses on turning the knowledge workbench into a curation surface:

- `/admin` supports knowledge list filtering across space, document status, source type, gap-linked state, and free-text query
- document detail projects source-gap and resolved-gap relationships so operators can see why a note exists and what evidence it closed
- local knowledge documents can be edited in place for title, source label, and content, then reindexed through the existing deterministic pipeline

## Developer workflow

Useful commands:

```powershell
go test ./...
go vet ./...
go build ./cmd/server
go build ./cmd/cli
go build ./cmd/smoke
```

## Repo guide

- [AGENTS.md](./AGENTS.md): required SDD + TDD workflow
- [docs/specs](./docs/specs): approved feature specs
- [docs/design](./docs/design): design docs
- [docs/plans](./docs/plans): implementation plans and test matrices
- [RELEASE_NOTES.md](./RELEASE_NOTES.md): document and implementation release history

## Phase 16 highlights

Phase 16 focuses on extending the local knowledge loop into an operator workbench:

- `investigating` is now a first-class local knowledge-gap status
- `/admin` can create workbench notes that preserve `source_gap_id`, `source_type`, and local provenance metadata
- operators can run diagnostics from a gap question and resolve a gap with an optional evidence document and resolution note

## Phase 15 highlights

Phase 15 focuses on turning knowledge grounding into a visible answer-state loop:

- `PersonaAgent` now emits deterministic `knowledge_answer_state` metadata
- runtime streaming and replay allowlist the answer state for `/app`
- unsupported and partially supported turns can create one local gap without duplicate open-gap spam
- `/app` now distinguishes grounded, partial, unsupported, provider fallback, guardrail fallback, and local mode states

## Phase 14 highlights

Phase 14 focuses on turning knowledge management into an operator workflow:

- `/admin` now exposes selected-space health summaries with deterministic counts and attention reasons
- document inspection now includes quality flags such as disabled, index failed, vector missing, and duplicate content hash
- retrieval diagnostics are rendered as a structured debug workbench instead of only raw text dumps
- unsupported knowledge-scoped turns can become local knowledge-gap records that operators can ignore or resolve

## Phase 13 highlights

Phase 13 focuses on making the `/app` Presence rail more useful and less visually noisy:

- Presence now summarizes the latest assistant turn instead of behaving like a growing visual/history area
- the avatar slot is bounded so long replies do not stretch the side rail
- grounding, memory, and fallback signals remain visible as compact operational cues
- transcript remains the canonical full conversation record

## Phase 12 highlights

Phase 12 focuses on turning the knowledge base into a scoped operator workspace:

- `/admin` now manages knowledge spaces before documents
- legacy flat knowledge data is migrated into a guaranteed `default` space
- retrieval diagnostics can filter by `space_id`
- `/app` carries the selected knowledge space into grounded answers
- the implementation stays local-first and deterministic: no SQLite, no real provider calls in CI
