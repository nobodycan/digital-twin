# Phase 10 Knowledge Base and Memory Control Plan

Date: 2026-06-29

Status: Draft, waiting for user approval

Spec: [Phase 10 Spec](../specs/phase-10-knowledge-base-memory-control.md)

Design: [Phase 10 Design](../design/phase-10-knowledge-base-memory-control.md)

## Goal

Turn the current knowledge/admin mock path into a real local knowledge base and
wire deterministic retrieval into persona chat, while making memory and knowledge
contribution visible and controllable.

## Scope

### In

- Local text/Markdown knowledge lifecycle.
- Deterministic lexical retrieval.
- Citation metadata through runtime, presentation, and Web UI.
- Memory list/disable visibility for active and disabled records.
- Prompt-injection safety for retrieved source context.
- README and smoke-style documentation for local verification.

### Out

- SQLite or any database migration.
- External vector DBs or mandatory remote embeddings.
- PDF/DOCX/cloud-source ingestion.
- Auth/RBAC.
- Background indexing workers.
- Real DeepSeek calls in CI.

## Architecture Work Items

| ID | Area | Files | Outcome |
| --- | --- | --- | --- |
| P10-01 | Knowledge contracts | `internal/admin`, `internal/knowledge` | Documents/chunks have lifecycle metadata and stable IDs |
| P10-02 | Knowledge file store | `internal/admin/*knowledge*` | Local store can save/list/get/update/delete safely |
| P10-03 | Lexical retriever | `internal/knowledge` | Deterministic ranked chunk search |
| P10-04 | Admin API | `internal/server`, `web/admin.*` | Operator can manage and test knowledge |
| P10-05 | Memory control | `internal/admin`, `internal/server`, `web/admin.*` | Active/disabled memory records are visible and disable remains reversible |
| P10-06 | Persona grounding | `internal/agents`, `internal/runtime`, `pkg/types` | Persona chat receives bounded source context and emits metadata |
| P10-07 | Presentation/Web citations | `internal/presentation`, `web/app.*` | `/app` renders source state and citation chips |
| P10-08 | Safety/DX/docs | `README.md`, tests | Hostile docs cannot override policy; local usage is documented |

## TDD Execution Plan

### P10-01 Knowledge Contracts

RED:

- Add tests proving uploaded documents include status, source type, hash, chunk
  count, timestamps, tags, and stable chunk ordinals.
- Add tests for disabled/indexing/failed status constants.

GREEN:

- Extend `KnowledgeDocument` and `KnowledgeChunk` fields.
- Keep JSON names stable and explicit.

REFACTOR:

- Add small constructors/helpers only where they reduce duplication.

Commands:

```powershell
go test ./internal/admin
```

### P10-02 Knowledge File Store Lifecycle

RED:

- Test `GetKnowledge`, `UpdateKnowledge`, and `DeleteKnowledge`.
- Test persistence across reopen.
- Test unsafe IDs cannot escape the data directory.
- Test atomic save leaves no temporary files after replacement.

GREEN:

- Extend the `KnowledgeStore` interface.
- Implement local JSON lifecycle operations.
- Use safe ID validation and write patterns aligned with `internal/store.LocalStore`.

REFACTOR:

- Keep file-store helpers private and boring.

Commands:

```powershell
go test ./internal/admin
```

### P10-03 Lexical Retriever

RED:

- Add `internal/knowledge` tests for ranking, zero-result behavior, disabled
  documents, CJK substring fallback, and stable tie-breaks.

GREEN:

- Implement tokenizer, scorer, and retriever.
- Search only `ready` documents unless explicitly asked otherwise.

REFACTOR:

- Keep scoring deterministic and simple; do not add embeddings.

Commands:

```powershell
go test ./internal/knowledge ./internal/admin
```

### P10-04 Admin Knowledge API

RED:

- Add server tests for:
  - `GET /admin/knowledge`;
  - `GET /admin/knowledge/{id}`;
  - disable/enable/delete/reindex;
  - citation-test returning ranked results;
  - secret/path-safe error payloads.

GREEN:

- Add handlers and route registration.
- Return actionable JSON errors with `problem`, `cause`, and `fix` where useful.

REFACTOR:

- Avoid duplicating tenant selection and JSON helpers.

Commands:

```powershell
go test ./internal/server ./internal/admin
```

### P10-05 Admin UI for Knowledge and Memory

RED:

- Extend static tests to require new endpoints in `web/admin.js`.
- Add DOM/string tests for document rows, status labels, actions, and citation
  results.
- Add tests proving memory list can show disabled records.

GREEN:

- Update `web/admin.html` and `web/admin.js`.
- Keep the layout dense and operator-focused.

REFACTOR:

- Extract small rendering helpers; keep static assets framework-free.

Commands:

```powershell
go test ./web ./internal/server
```

### P10-06 Persona Grounding Integration

RED:

- Add tests proving persona generation receives bounded knowledge context when
  retrieval returns chunks.
- Add tests proving no fake citation metadata is emitted when retrieval returns
  no chunks.
- Add hostile document tests:
  - ignore previous instructions;
  - reveal API key;
  - disable safety checks;
  - answer without citations.

GREEN:

- Add a retrieval dependency to local runtime/persona wiring.
- Build source-bounded prompt sections in `PersonaAgent`.
- Add allowlisted grounding metadata to `AgentResult`.

REFACTOR:

- Keep source prompt construction separate enough to test without a real provider.

Commands:

```powershell
go test ./internal/agents ./internal/runtime ./internal/app
```

### P10-07 Presentation and `/app` Citation UI

RED:

- Add presentation tests proving grounding metadata is preserved safely.
- Add Web static/regression tests for citation chips and source-state labels.
- Add mobile/desktop layout assertions if browser QA is available in Stage 5.

GREEN:

- Extend presentation metadata mapping.
- Render `Knowledge grounded`, `No source used`, and `Memory considered` states.

REFACTOR:

- Keep provider/fallback UI from Phase 9 intact.

Commands:

```powershell
go test ./internal/presentation ./web
```

### P10-08 Documentation and Local Verification

RED:

- Add static script/docs tests where practical for the documented commands and
  endpoints.

GREEN:

- Update `README.md` with:
  - upload knowledge;
  - run citation test;
  - ask grounded question;
  - interpret citation UI.
- Update `RELEASE_NOTES.md` after implementation.

REFACTOR:

- Keep DeepSeek instructions focused and avoid implying real provider calls in CI.

Commands:

```powershell
go test ./...
rg -n "Phase 10|knowledge|citation|memory|SQLite" README.md docs RELEASE_NOTES.md
```

## Test Matrix

| Area | Test | Command |
| --- | --- | --- |
| Knowledge contracts | Metadata, statuses, chunk IDs | `go test ./internal/admin` |
| File store | Reopen, update, delete, unsafe IDs | `go test ./internal/admin` |
| Retrieval | Ranking, zero results, disabled docs, CJK | `go test ./internal/knowledge` |
| Admin API | Lifecycle endpoints and safe errors | `go test ./internal/server` |
| Memory API | Active/disabled records and disable behavior | `go test ./internal/admin ./internal/server` |
| Persona | Source-bounded prompt and citation metadata | `go test ./internal/agents` |
| Runtime | Grounding context flows through streaming turn | `go test ./internal/runtime ./internal/app` |
| Presentation | Allowlisted grounding metadata | `go test ./internal/presentation` |
| Web admin | Knowledge manager controls render | `go test ./web` |
| Web app | Citation chips and source labels render | `go test ./web` |
| Full suite | Regression coverage | `go test ./...` |

## Failure Modes and Mitigations

| Failure Mode | Impact | Mitigation |
| --- | --- | --- |
| Uploaded document contains prompt injection | Model may follow source commands | Source boundary prompt and hostile-doc tests |
| Disabled document still retrieves | User loses control over source usage | Retriever filters status; lifecycle tests cover it |
| No retrieved chunks but UI shows citations | False trust signal | Metadata only includes returned chunks; Web tests |
| Knowledge JSON corrupts on write | Local knowledge base breaks | Atomic writes and reopen tests |
| Memory disable does not affect recall | User cannot control personalization | Active recall tests use disabled record |
| Large source leaks through stream metadata | Privacy/noise issue | Citation previews and allowlisted metadata |
| Lexical retrieval misses obvious semantic match | Weak answer quality | Document limitation; later embedding extension point |

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Knowledge management becomes Phase 10 spine | Auto-decided | Product leverage | User ambition points beyond memory into source-managed digital-human work | Memory-only phase |
| 2 | CEO | Defer vector DB and external embeddings | Auto-decided | Local-first constraint | Deterministic CI and DeepSeek setup should not depend on embedding support | Full semantic RAG now |
| 3 | Design | `/admin` owns lifecycle, `/app` owns compact source proof | Auto-decided | Interface clarity | Separates operator controls from conversation reading flow | Put all controls in `/app` |
| 4 | Eng | Add `internal/knowledge` retriever package | Auto-decided | Separation of concerns | Keeps admin lifecycle separate from retrieval/ranking logic | Expand admin service into retrieval engine |
| 5 | Eng | Retrieval on by default when ready docs exist | Taste decision | Product behavior | A professional knowledge worker should use managed sources naturally, with visible state | Require user toggle before using knowledge |
| 6 | Eng | Memory delete remains out of scope | Auto-decided | Reversibility | Disable is safer until irreversible privacy UX is designed | Add hard delete for memory now |
| 7 | DX | CI uses lexical/fake clients only | Auto-decided | Repeatability | Real provider calls are unsuitable for CI and local contributors | Real DeepSeek smoke in CI |

## Review Scores

| Review | Score | Summary |
| --- | --- | --- |
| CEO | 9/10 | Strong next product step; scope is ambitious but bounded |
| Design | 8/10 | Clear split between admin controls and app source proof |
| Engineering | 8/10 | Fits current architecture; main risk is prompt/metadata boundary |
| DX | 8/10 | Local deterministic retrieval makes verification fast |

## User Choices

### Choice 1: Retrieval Default

Recommendation: keep retrieval **on by default** when ready knowledge exists.

Why: this makes the digital human feel like it actually works from managed sources.
The UI will still show whether knowledge was used, so the behavior is not hidden.

Alternative: add a per-session `/app` toggle before using knowledge. This gives
more control but makes the first useful experience weaker and adds UI state now.

## Implementation Order

1. Knowledge contracts and file lifecycle.
2. Lexical retriever.
3. Admin API lifecycle endpoints.
4. Admin UI for knowledge and memory states.
5. Persona/runtime grounding integration.
6. Presentation and `/app` citation rendering.
7. Docs and full regression pass.

## Acceptance Gate for Stage 3

Stage 3 may begin only after this plan is approved.

Required implementation discipline:

- follow RED -> GREEN -> REFACTOR for each work item;
- no production code before a failing test;
- keep commits small if committing during build;
- preserve local file storage and no SQLite.

## Verification Commands

```powershell
go test ./internal/admin
go test ./internal/knowledge
go test ./internal/server
go test ./internal/agents ./internal/runtime ./internal/app
go test ./internal/presentation ./web
go test ./...
rg -n "Phase 10|knowledge|citation|memory|SQLite" README.md docs RELEASE_NOTES.md
git status -sb
```
