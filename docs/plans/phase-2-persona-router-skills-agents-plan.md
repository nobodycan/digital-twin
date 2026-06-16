# Phase 2 Persona, Router, Skills, and Agents Plan

Status: IMPLEMENTED

Source artifacts:

- `docs/specs/phase-2-persona-router-skills-agents.md`
- `docs/design/phase-2-persona-router-skills-agents.md`
- `plan.md` Phase 2, M3, M4, M5

## Autoplan Review Summary

Phase 2 should proceed as a thin complete capability layer: persona, prompt rendering, guard, rule/LLM/hybrid routing, skill framework, concrete deterministic skills, BaseAgent, and expert agents. The implementation must stay inside Phase 2 boundaries and must not introduce Phase 3 runtime/API behavior.

UI scope: no. Design review skipped because Phase 2 does not ship user-facing screens.

DX scope: yes. This Phase defines developer-facing extension points for new skills and agents, so naming, test patterns, error messages, and examples matter.

## Premise Challenge

| Premise | Decision | Reason |
| --- | --- | --- |
| Persona must be a first-class model, not a hard-coded prompt | Accepted | Golden prompt tests and validation require stable structured input. |
| Routing should be hybrid: rules first, LLM fallback | Accepted | Keeps obvious requests deterministic and makes classifier cost/failure controllable. |
| Skills should be atomic and reusable | Accepted | Agents become thin and testable when side-effectful operations are isolated. |
| All provider-like behavior must remain deterministic in Phase 2 | Accepted | Real providers belong after runtime, security, and QA gates. |
| Phase 2 can still use local filesystem storage and in-memory/vector abstractions | Accepted | Matches the user's current "no SQLite" preference and Phase 1 implementation. |

Premise gate: passed by user approval of the Phase 2 spec.

## What Already Exists

| Sub-problem | Existing code to reuse |
| --- | --- |
| Agent, Skill, Router contracts | `internal/core/interfaces.go` |
| Agent and Skill registration | `internal/core/registry.go` |
| Shared domain errors | `internal/core/errors.go` |
| Message, conversation, intent, result contracts | `pkg/types/contracts.go` |
| LLM chat, stream, embed, summarize abstractions | `internal/llm/client.go` |
| Local store and vector store abstractions | `internal/store/store.go` |
| Memory abstraction and records | `internal/memory/memory.go` |
| Test fakes | `internal/testutil` |

## NOT In Scope

- HTTP chat API, SSE, WebSocket, gRPC, or CLI conversation runtime.
- Production Orchestrator behavior.
- Web UI, admin console, avatar rendering, real TTS, or real ASR.
- Real external LLM, search, calendar, vector DB, database, or provider credentials.
- SQLite or any new persistent service.
- Generated mock framework unless a later plan revision explicitly approves it.

## Recommended Architecture

```text
pkg/types
  |  shared Intent / Message / Result contracts
  v
internal/persona
  |-- Persona model + validation
  |-- prompt renderer + golden fixtures
  `-- persona guard

internal/router
  |-- rule router
  |-- llm router ----> internal/llm.Client
  `-- hybrid router

internal/skills
  |-- validation base
  |-- memory skills ------> internal/memory.Memory
  |-- knowledge skills ---> internal/store.VectorStore + internal/llm.Client
  |-- task skills
  |-- tool skills --------> allowlist + local fake clients
  |-- persona skills -----> internal/persona
  |-- safety skills
  `-- presentation skills -> deterministic placeholders

internal/agents
  |-- BaseAgent ---------> internal/core.SkillRegistry
  |-- PersonaAgent ------> persona skills + guard
  |-- MemoryAgent -------> memory skills
  |-- KnowledgeAgent ----> knowledge skills
  |-- TaskAgent ---------> task skills
  `-- ToolAgent ---------> tool skills
```

## Execution Strategy

Use Superpowers TDD in Stage 3 for every step:

1. RED: add or update the failing test first.
2. GREEN: implement the smallest production change that passes.
3. REFACTOR: simplify names, package boundaries, and duplicate validation code while tests stay green.

No production file should be introduced without a corresponding failing test in the same small step.

## Implementation Tasks

| ID | Task | Files | Test first |
| --- | --- | --- | --- |
| P2-01 | Add persona model and validation | `internal/persona` | `persona_test.go` validates success, missing identity, missing role, invalid boundaries, unsafe fragments |
| P2-02 | Add deterministic prompt renderer and golden fixture | `internal/persona` | golden test proves byte-stable output with injected clock |
| P2-03 | Add persona guard | `internal/persona` | tests cover pass, forbidden claim, low-confidence uncertainty, safe fallback metadata |
| P2-04 | Add Phase 2 intent names only if router tests require them | `pkg/types` | contract tests prove JSON round trip and naming stability |
| P2-05 | Add rule router | `internal/router` | tests cover knowledge, memory, task, tool, persona/small-talk, ambiguity priority |
| P2-06 | Add LLM classifier router | `internal/router` | fake LLM tests cover valid JSON, invalid JSON, low confidence, provider error, context cancel |
| P2-07 | Add hybrid router | `internal/router` | tests cover rule hit, LLM fallback, low-confidence fallback, full failure fallback |
| P2-08 | Add skill validation base | `internal/skills` | tests cover required params, optional defaults, type mismatch, structured validation errors |
| P2-09 | Add memory skills | `internal/skills` | `mem_store`, `mem_recall`, `summarize` valid/invalid/dependency failure tests |
| P2-10 | Add knowledge skills | `internal/skills` | `embed`, `vector_search`, `cite` valid/invalid/dependency failure tests |
| P2-11 | Add task skills | `internal/skills` | `task_decompose`, `plan`, `track` table-driven tests |
| P2-12 | Add tool skills with deny-by-default network policy | `internal/skills` | `http_call`, `search_web`, `calendar` reject non-allowlisted and private/local targets |
| P2-13 | Add persona and safety skills | `internal/skills` | `tone_adjust`, `persona_check`, `pii_detect`, `prompt_injection_check`, `risk_classify`, `policy_decide` tests |
| P2-14 | Add presentation placeholder skills | `internal/skills` | `tts_speak`, `asr_transcribe`, `avatar_state`, `subtitle_timeline` deterministic placeholder tests |
| P2-15 | Add BaseAgent | `internal/agents` | interface satisfaction, result helper, missing skill, dependency error tests |
| P2-16 | Add PersonaAgent | `internal/agents` | `CanHandle`, `Run`, registry registration, guard failure tests |
| P2-17 | Add MemoryAgent | `internal/agents` | `CanHandle`, `Run`, registry registration, memory skill failure tests |
| P2-18 | Add KnowledgeAgent | `internal/agents` | `CanHandle`, `Run`, registry registration, citation and empty result tests |
| P2-19 | Add TaskAgent | `internal/agents` | `CanHandle`, `Run`, registry registration, decomposition failure tests |
| P2-20 | Add ToolAgent and SafetyAgent | `internal/agents` | `CanHandle`, `Run`, registry registration, allowlist denial and safety classification tests |
| P2-21 | Final docs and release notes sync | `README.md`, `RELEASE_NOTES.md`, `docs` | read-only doc checks and `rg` verification |

## Parallelization Plan

Do not parallelize before P2-08. Persona and routing establish semantics that later work depends on.

After P2-08:

- Workstream A: P2-09 memory skills, then P2-17 MemoryAgent.
- Workstream B: P2-10 knowledge skills, then P2-18 KnowledgeAgent.
- Workstream C: P2-11 task skills, then P2-19 TaskAgent.
- Workstream D: P2-12 tool skills, then P2-20 ToolAgent.
- Workstream E: P2-13 and P2-14 persona/safety/presentation skills, then P2-16 PersonaAgent.

P2-15 BaseAgent blocks all expert agents. If using parallel agents, each workstream must own disjoint files and must not edit shared skill validation or BaseAgent files.

## Test Diagram

```text
Persona config
  |-- Validate() -------------------- unit: valid / invalid / contradiction
  |-- RenderSystemPrompt() ---------- golden: stable text / sorted fields / clock
  `-- Guard.Check() ----------------- unit: allowed / forbidden / uncertainty

Conversation
  `-- HybridRouter.Route()
       |-- RuleRouter.Route() ------- unit: direct matches / ambiguity
       |-- LLMRouter.Route() -------- unit: fake LLM JSON / invalid / low confidence
       `-- Fallback ----------------- unit: persona fallback / unknown

AgentRegistry.Find(intent)
  `-- ExpertAgent.Run()
       |-- SkillRegistry.Get(name) -- unit: missing skill / duplicate unaffected
       |-- Skill.Run(params) -------- unit: valid / invalid / dependency failure
       `-- AgentResult ------------- unit: metadata / confidence / safe message

Tool skill execution
  |-- allowlist check --------------- unit: allowed / denied
  |-- local/private target guard ----- unit: denied by default
  `-- fake transport ---------------- unit: no real network
```

## Explicit Test Plan

Run these after each relevant small step:

- Persona: `go test ./internal/persona`
- Router: `go test ./internal/router`
- Skills: `go test ./internal/skills`
- Agents: `go test ./internal/agents`
- Types touched: `go test ./pkg/types`

Run these at Phase 2 close:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/server`

Optional on a supported platform:

- `go test -race ./...`

## Failure Modes Registry

| Failure | Severity | Test requirement | Handling |
| --- | --- | --- | --- |
| Persona config invalid but accepted | High | Invalid persona tests | Return clear validation error with field name |
| Prompt output unstable | High | Golden test with injected clock | Sort dynamic fields and freeze time in tests |
| Router chooses wrong agent for obvious request | High | Rule priority tests | Deterministic rule ordering and metadata |
| LLM router accepts malformed JSON | High | Fake LLM invalid JSON test | Fall back to persona/unknown intent |
| Skill mutates state after invalid params | High | Invalid param side-effect test | Validate before dependency calls |
| Tool skill reaches private/local network | Critical | Allowlist and private IP tests | Deny by default |
| Agent panics on missing skill | High | Missing skill test | Return safe result or wrapped `ErrSkillNotFound` |
| Knowledge answer lacks citation metadata | Medium | Citation tests | Include source IDs and empty-result behavior |
| Presentation skill implies real TTS/ASR exists | Medium | Placeholder metadata tests | Return deterministic placeholder result |
| Phase 3 runtime leaks into Phase 2 | High | Diff review | Keep HTTP/API/orchestrator out of scope |

## Error And Rescue Registry

| Error class | User/developer impact | Required rescue |
| --- | --- | --- |
| Validation error | Developer knows which config or params failed | Error message includes field, expected type/rule, and safe value guidance |
| Dependency error | Agent cannot complete a skill-backed operation | AgentResult metadata records dependency and returns safe assistant message |
| Classifier error | User request cannot be confidently classified | Fallback intent routes to PersonaAgent or unknown with low confidence |
| Policy denial | Tool request is unsafe or disallowed | SkillResult metadata includes denied reason and no side effects |
| Empty retrieval | Knowledge/memory lookup finds nothing | Return empty result with explicit metadata, not an error |

## DX Review

Developer persona: a Go contributor adding or modifying an internal digital-twin capability.

Developer journey:

| Stage | Expected path | Plan requirement |
| --- | --- | --- |
| Discover | Read README and Phase 2 plan | README links spec/design/plan |
| Evaluate | Understand package boundaries | Architecture diagram and task table |
| Implement | Pick one P2 task | Each task has files and test-first instruction |
| Test | Run package test | Each workstream lists package command |
| Debug | Interpret failures | Validation and dependency errors include field/cause/fix |
| Extend | Add a new skill or agent | Patterns in `internal/skills` and `internal/agents` stay explicit |
| Review | Check scope and safety | Failure modes and out-of-scope list prevent leaks |
| Upgrade | Later phases build on contracts | No provider lock-in or DB migration |
| Operate | Phase 3 consumes outputs | Router/Agent/Skill metadata ready for observability |

TTHW estimate for a new contributor after plan approval: 8-12 minutes to add a simple deterministic skill with tests. Target after Phase 2 docs sync: under 5 minutes by adding a short "How to add a skill" note in the final documentation step.

DX scorecard:

| Dimension | Score | Reason |
| --- | --- | --- |
| Getting started | 7/10 | Clear tasks, but no concrete skill example until implementation exists. |
| API/package naming | 8/10 | Package boundaries are explicit and align with Phase 1 contracts. |
| Error messages/debugging | 8/10 | Error/rescue registry requires field/cause/fix style. |
| Documentation/learning | 7/10 | Spec/design/plan are clear; extension guide deferred to final docs sync. |
| Upgrade path | 8/10 | No provider or database lock-in. |
| Developer environment | 8/10 | Existing Go commands are simple and local. |
| Testing support | 9/10 | Every task has RED/GREEN tests and fake dependencies. |
| Feedback loops | 7/10 | Package tests exist; runtime evals start in later phases. |

Overall DX: 7.8/10.

## Review Scores

| Review | Result |
| --- | --- |
| CEO | Hold scope with thin complete implementation; no user challenge. |
| Design | Skipped, no UI scope. |
| Eng | Approved with strong sequencing: persona/router, skill base, concrete skills, BaseAgent, agents. |
| DX | Approved with one deferred docs improvement: add an extension guide once patterns exist. |

Cross-phase theme: keep provider-like behavior deterministic and local until Phase 3+ security/QA gates.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Keep Phase 2 scope thin but complete | Mechanical | Choose completeness | Satisfies M3/M4/M5 without claiming runtime/API behavior | Minimal vertical slice only |
| 2 | CEO | Skip UI design review | Mechanical | Explicit over clever | Phase 2 has no screens or UI states to review | Forcing UI review |
| 3 | Eng | Use `internal/persona` separate from agents | Mechanical | DRY | Persona model/render/guard are reused by router, agents, and skills | Embed persona logic in PersonaAgent |
| 4 | Eng | Rule router before LLM router | Mechanical | Pragmatic | Deterministic obvious routing reduces cost and failure paths | LLM-first routing |
| 5 | Eng | Hand-written skill validation first | Taste | Explicit over clever | Avoids new schema dependency until repeated patterns prove need | JSON Schema dependency now |
| 6 | Eng | Deny network-like tool calls by default | Mechanical | Choose completeness | Prevents SSRF/local-network risk before security phase | Permit by default in tests |
| 7 | Eng | Implement provider-shaped skills as deterministic placeholders/fakes | Mechanical | Bias toward action | Preserves API shape without external providers | Real providers in Phase 2 |
| 8 | Eng | BaseAgent blocks expert agents | Mechanical | DRY | Shared result and skill lookup behavior should be tested once | Duplicate shared logic per agent |
| 9 | DX | Add extension guide in final docs sync | Mechanical | Bias toward action | Patterns are clearer after implementation exists | Write speculative guide before code |
| 10 | Review | Add SafetyAgent for `safety.check` | Mechanical | Enum completeness | New intent must have a registry consumer | Let `safety.check` fall through to not found |

## Taste Decisions

**T1: Skill validation style**

Recommendation: start with hand-written validation helpers. JSON Schema is viable later if validation becomes repetitive across many skills, but adding it before the first real skill set is premature.

## User Challenges

None. The plan preserves the user-approved spec and current no-SQLite direction.

## Pre-Gate Verification

| Required output | Status |
| --- | --- |
| Premise challenge | Produced |
| What already exists | Produced |
| NOT in scope | Produced |
| Architecture diagram | Produced |
| Test diagram | Produced |
| Explicit test plan | Produced |
| Failure modes registry | Produced |
| Error/rescue registry | Produced |
| DX journey and scorecard | Produced |
| Decision audit trail | Produced |

## Approval Gate

The plan approval gate passed, and Stage 3 BUILD implemented this plan with TDD.

Stage 4 CODE REVIEW and later gates remain next in the `AGENTS.md` pipeline.
