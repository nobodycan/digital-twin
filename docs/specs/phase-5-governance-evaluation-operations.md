# Phase 5 Governance, Evaluation, Security, and Operations Spec

## Context

Phase 5 turns the Phase 4 mock/local-first digital-human product into something that can be evaluated, gated, audited, and operated with confidence. The product now has a working runtime, Web experience, local admin console, persona publishing, memory controls, knowledge uploads, tool policies, presentation events, and audit records. The remaining risk is not whether the system can answer once; it is whether it can keep answering safely, consistently, and cheaply as prompts, knowledge, persona, and model settings change.

This phase covers M10 from `plan.md`:

- M10.1-M10.5 AI behavior evaluation.
- M10.6-M10.9 memory, privacy, compliance, tenant isolation, audit, and disclosure.
- M10.10-M10.14 prompt-injection defense, high-risk content policy, model routing, release gates, rollback, human handoff, and feedback loops.

## Current State

| Area | Current state | Gap for Phase 5 |
| --- | --- | --- |
| Runtime | Local orchestrator, CLI, HTTP `/chat`, SSE `/chat/stream`, and Phase 4 `/experience/stream` exist | No repeatable eval runner or release gate around runtime behavior |
| Persona | Persona model, renderer, guard, admin draft/publish/rollback flow exist | No persona regression suite or publish gate that blocks unsafe persona/prompt changes |
| Knowledge | Local knowledge upload, chunk preview, citation test, vector abstraction, and deterministic skills exist | No RAG faithfulness eval, malicious-document prompt-injection eval, or stale citation checks |
| Tools | Skill validation, HTTP allowlist, local/private target blocking, admin tool-policy authorization check exist | Tool policy is not yet a full pre-execution governance gate across every tool path |
| Memory | Short/long memory abstractions and admin disable flow exist | No write policy, sensitivity classifier, retention/expiry runner, or explainable recall report |
| Audit | Phase 4 conversation audit records exist | Audit does not yet capture policy decisions, eval failures, release decisions, or feedback lifecycle |
| Security | PII, prompt-injection, risk, policy skills exist as deterministic placeholders | No central policy engine, high-risk taxonomy, or negative eval set for jailbreak/prompt injection |
| Operations | README and release notes track phase status | No versioned release candidate model, eval-before-publish gate, rollback decision record, or feedback triage queue |

One extra gap is now explicit: Phase 4 admin configuration is still mostly a control plane. Phase 5 must not assume that a published persona, tool policy, knowledge upload, or memory policy automatically changes runtime behavior until that wiring is proven by tests.

## Office-Hours Review

The user asked to proceed with Phase 5 under `AGENTS.md`. This Stage 1 spec challenges the broad M10 framing before implementation.

Hidden assumptions surfaced:

- "Governance" can become a vague dashboard project. For Phase 5, governance must first be an executable gate: inputs, checks, results, pass/fail, and audit trail.
- Eval quality matters more than eval volume. A small golden set with adversarial negative cases is more valuable than a large shallow suite.
- Safety is not only content moderation. For this product, safety also includes tenant isolation, memory write policy, tool authorization, prompt-injection resistance, and release rollback.
- Local file storage is still the constraint. Phase 5 must not smuggle in SQLite, SaaS eval platforms, queues, or external dashboards.
- Automated judges can be useful later, but first-pass tests should be deterministic enough to run in CI and on Windows.
- Phase 4 admin records are useful but not sufficient. Release and feedback records need their own explicit model rather than overloading conversation audit.
- A professional digital human must disclose that it is AI. This is a product safety requirement, not just a UI copy detail.
- The most dangerous hidden assumption is that having safety skills, an admin policy screen, and audit rows already equals governance. The real failure modes live between those parts: policy is not yet bound to every runtime execution path, evals do not yet block publish, memory writes do not yet pass a privacy policy, and audit cannot yet explain why a decision was allowed or denied.

## Proposed Change

Implement Phase 5 as a local-first governance slice with five linked tracks:

1. **Runtime governance wiring**: prove published admin configuration affects the runtime path being evaluated.
2. **Evaluation gate**: golden conversation fixtures, deterministic evaluators, a Go eval runner, JSON/Markdown reports, and pass/fail thresholds.
3. **Policy and privacy governance**: memory write policy, PII/sensitive data handling, prompt-injection checks for user and knowledge inputs, high-risk content classification, and tenant isolation tests.
4. **Release and rollback operations**: versioned release candidates for persona/prompt/knowledge/tool/model settings, eval-before-publish decisions, rollback records, and audit events.
5. **Feedback loop**: user/operator feedback records, triage statuses, links from failed conversations to eval cases or knowledge/persona fixes.

The first implementation should make release readiness testable without external infrastructure. It should be possible to run Phase 5 checks from the CLI or `go test ./...` and get a clear answer: pass, fail, or skipped with reason.

The recommended first sub-slice is **Phase 5A: Launch Gate MVP**. It should not claim production compliance. Its job is to prove that persona, knowledge, tool, prompt/model, memory, and safety changes can be evaluated before publish and explained after failure.

Phase 5A has a hard prerequisite: the evaluated runtime must consume the same persona/tool/knowledge/memory policy surfaces that operators configure. A runner that only tests fixed bootstrap defaults is not sufficient.

## Scope

### Runtime Governance Wiring

Before release gates can be trusted, Phase 5 must close the control-plane/runtime gap:

- Published persona must be discoverable by the runtime or by a runtime adapter used in eval.
- Tool-policy decisions must sit on the path before `Skill.Run`, not only behind an admin test endpoint.
- Knowledge versions used by eval must be explicit and tenant-scoped.
- Memory write/read policy must be enforced before long-term memory persistence or recall.
- Eval reports must name which persona, knowledge, tool-policy, memory-policy, and model/cost versions were evaluated.

This does not require a production auth system, database, or external service. It requires testable wiring and honest metadata.

### Evaluation Runner

Add local golden conversations under `evals/conversations/`.

Each case should include:

- Stable ID, title, category, and risk level.
- Input conversation or presentation flow.
- Expected intent, allowed/denied tool behavior, expected citations, memory expectations, and safety expectations.
- Assertions for persona consistency, factual grounding, tool policy, memory write, prompt injection, and high-risk content.
- Negative examples for jailbreaks, malicious knowledge, cross-tenant access, and unsafe memory writes.

Add a runner that can:

- Load all cases from local files.
- Execute deterministic checks against existing runtime/service interfaces or stored sample outputs.
- Produce machine-readable JSON and human-readable Markdown summaries.
- Fail when required checks do not meet thresholds.
- Run without network, microphone, external model, external vector DB, or SQLite.

### Evaluators

The first evaluator set should be deterministic:

- **Persona evaluator**: checks identity disclosure, tone boundaries, forbidden claims, and low-confidence phrasing.
- **RAG evaluator**: checks citations reference known chunks and flags unsupported source claims.
- **Tool evaluator**: checks schema errors, denied tools, unauthorized tool use, and local/private network blocking.
- **Memory evaluator**: checks whether proposed memories are stable, useful, non-sensitive, explainable, and tenant/user scoped.
- **Safety evaluator**: checks prompt injection, sensitive content, and high-risk scenario policy decisions.
- **Cost/performance evaluator**: records local latency and estimated usage counters where available; starts with deterministic estimates when real token accounting is unavailable.

LLM-as-judge can be added later as an optional adapter, but must not be required for baseline Phase 5 tests.

### Memory and Privacy Governance

Add a policy model for memory writes:

- Allowed: stable user preferences, durable task context, explicitly confirmed profile facts.
- Denied: passwords, API keys, financial identifiers, medical/legal secrets, government IDs, private data from third parties, short-lived emotional noise, and unconfirmed inferences.
- Required metadata: source conversation, source message, tenant, user, reason, confidence, creation time, expiry time or retention class.

Admin memory disable already exists. Phase 5 should add policy decisions before write and tests proving denied memory never appears in recall.

### Tenant Isolation

Phase 5 must treat tenant isolation as a testable invariant:

- Conversations, memories, knowledge, tool policies, audit records, feedback, and release records must carry tenant scope.
- Eval cases must prove tenant A cannot read tenant B memory, knowledge, audit, or release state.
- Any new local file store must structure data so tenant leakage is hard to introduce and easy to test.

### Prompt Injection and High-Risk Policy

Add a central policy decision model that can be used by evaluators, tools, and future runtime hooks:

- Prompt-injection source: user input, knowledge document, tool output, or web/search result.
- Action: allow, warn, sanitize, deny, escalate, or human handoff.
- Reason code and evidence.
- Audit-safe explanation.

High-risk categories should include:

- Medical, legal, financial, self-harm, minors, illegal activity, privacy, credential handling, and destructive operations.

The first policy can be rule-based and conservative. The important acceptance point is that negative cases are explicit and repeatable.

### Release Gates

Add local-first release candidate records for:

- Persona and prompt versions.
- Knowledge corpus versions.
- Tool-policy versions.
- Model routing and cost-policy versions.

Publishing a governed version should require an eval summary:

- Passing required suites permits publish.
- Failing required suites blocks publish.
- Skipped suites require an explicit reason.
- Rollback creates an audit record with previous and target version IDs.

Phase 4 persona publish exists. Phase 5 should either wrap it behind an eval gate or define a new gate service used by future admin publish actions.

At minimum, a governed publish decision should record:

- Release candidate ID and version target.
- Eval run ID and required suite threshold.
- Policy version.
- Actor, tenant, timestamp, and decision.
- Failing case IDs when blocked.
- Rollback target when reverted.

### Runtime Tool Governance

Phase 4 added a tool-policy admin service and a `/admin/tools/authorize` endpoint. Phase 5 must close the gap between that endpoint and real tool execution:

- Tool calls must be checked before skill execution, not only through an admin test endpoint.
- Tool decisions must include tenant, persona, tool name, parameters or safe parameter summary, reason code, and decision.
- Denied calls must not invoke the underlying skill.
- Denied and allowed decisions should be audit-visible.
- Eval fixtures must include allowed, denied, malformed, and local/private network cases.

### Feedback Loop

Add feedback records:

- User/operator, conversation ID, message ID, category, severity, free-text note, created time, status.
- Statuses: new, triaged, eval-added, knowledge-fix-needed, persona-fix-needed, dismissed, resolved.
- Link feedback to eval case creation or release gate evidence.

The first Web/admin UI can remain minimal or deferred to Stage 2, but the service model and tests should exist if feedback is included in the build plan.

## Non-Goals

- No external eval platform.
- No cloud moderation provider requirement.
- No real billing, invoicing, or payment-cost integration.
- No SOC 2, legal compliance certification, or retention policy enforcement beyond local deterministic behavior.
- No SQLite/Postgres/Redis/queue introduction.
- No production-grade RBAC or OAuth.
- No mandatory LLM judge in CI.
- No full browser dashboard rewrite.
- No real deployment automation beyond local release-gate records unless Stage 2 explicitly narrows it.

## Acceptance Criteria

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. Existing Phase 3 and Phase 4 APIs remain compatible.
4. Runtime governance wiring tests prove at least persona/tool policy or their eval adapters consume operator-controlled versions rather than fixed bootstrap defaults.
5. Golden conversation fixtures exist for happy path and adversarial path.
6. Eval runner loads fixtures and emits JSON and Markdown summaries.
7. Persona evaluator flags identity/persona boundary violations.
8. RAG evaluator flags unsupported citation or invented source claims.
9. Tool evaluator flags unauthorized tools, schema failures, and private/local HTTP targets.
10. Memory policy evaluator rejects sensitive or unstable memory writes.
11. Tenant isolation tests prove memory, knowledge, audit, feedback, and release records do not cross tenant boundaries.
12. Prompt-injection evaluator flags malicious user input and malicious knowledge content.
13. High-risk policy evaluator returns deny/escalate/safe-completion actions for configured categories.
14. Release gate blocks publish when required eval suites fail.
15. Rollback records previous and restored version IDs.
16. Feedback records can be created, triaged, and linked to an eval or remediation target.
17. Audit records include policy, eval, release, rollback, and feedback decisions where implemented.
18. Runtime tool execution uses the same policy semantics as the admin authorization path.
19. Default local/demo behavior remains documented and is not presented as production authentication or authorization.
20. README and release notes describe only implemented Phase 5 behavior.

## Test Matrix

| Level | Scenario | Evidence |
| --- | --- | --- |
| Unit | Eval fixture parser | Valid fixtures load; malformed fixtures fail with useful error |
| Unit | Persona evaluator | Off-persona or misleading identity response fails |
| Unit | RAG evaluator | Unsupported citation and fabricated source fail |
| Unit | Tool evaluator | Denied tool and bad schema fail |
| Unit | Memory policy | Password/email/API key/private inference denied |
| Unit | Prompt injection policy | Knowledge text saying "ignore previous instructions" is denied/sanitized |
| Unit | High-risk classifier | Medical/legal/financial/self-harm samples produce expected policy action |
| Unit | Tenant scope | Store keys and query filters enforce tenant/user boundaries |
| Integration | Runtime governance wiring | Evaluated runtime consumes governed persona/tool/knowledge/memory versions |
| Integration | Eval runner | Runs golden suite and writes JSON/Markdown result |
| Integration | Release gate | Failing eval blocks persona/prompt/knowledge/tool/model publish |
| Integration | Rollback | Rollback creates release and audit records |
| Integration | Feedback | Feedback becomes triage item and can reference eval case |
| HTTP/Admin | Optional Stage 2 UI/API | Admin can inspect eval/release/feedback records if included |
| Regression | Existing Phase 4 flow | `/app`, `/admin`, `/experience/stream` still work |

## Failure Modes

| Failure | Expected behavior |
| --- | --- |
| Eval fixture malformed | Runner reports file, field, and fix; suite fails fast |
| Required evaluator unavailable | Suite fails unless explicitly marked optional with reason |
| Optional evaluator unavailable | Suite reports skipped with reason |
| Release gate fails | Publish is blocked and records failing case IDs |
| Rollback target missing | Rollback fails without changing active version |
| Feedback references missing conversation | Feedback is rejected or recorded as orphaned with explicit status |
| Memory policy denies write | No long-term memory is persisted and denial is audit-visible |
| Tenant mismatch | Request returns no data or explicit forbidden; never returns other tenant records |
| Prompt injection detected in knowledge | Document/chunk is quarantined or marked unsafe; system prompt remains unaffected |
| Cost budget exceeded | Release/eval report fails budget threshold or marks degraded route |

## Rollback

Phase 5 should remain local-file and code-only. A rollback should not require database migrations or external state cleanup. Generated eval reports and local release records must live in documented local directories that can be ignored or cleaned.

## Open Questions for Stage 2

1. Should the first implementation expose eval/release/feedback through CLI only, admin UI only, or both?
2. Should Phase 5 wrap Phase 4 persona publish immediately, or first create a separate gate service and tests?
3. Should runtime governance wiring be implemented directly in the production runtime path, or through a governed runtime adapter used by eval first?
4. Should feedback be implemented in the first Phase 5 slice, or deferred until eval and release gates are stable?
5. Should cost/performance checks use deterministic estimates only, or add real latency measurements from local HTTP/browser QA?
6. Which suites are required for a release gate in v0.5.0: persona, RAG, tools, memory, safety, tenant isolation, cost?
