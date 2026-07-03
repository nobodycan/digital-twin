# Phase 15 Knowledge-Grounded Answer Loop Spec

Date: 2026-07-03

Status: Approved by the user on 2026-07-03

Mode: SDD Stage 1 / gstack office-hours

Source context:

- [Phase 11 Knowledge Retrieval Quality and RAG Evaluation Spec](./phase-11-knowledge-retrieval-quality-rag-evaluation.md)
- [Phase 12 Knowledge Space Management and Grounded Answering Spec](./phase-12-knowledge-space-management-grounded-answering.md)
- [Phase 14 Knowledge Operations Console Spec](./phase-14-knowledge-operations-console.md)
- Current implementation: local file-backed knowledge spaces, retrieval pipeline,
  scoped grounding metadata, citation chips in `/app`, local knowledge gap
  capture, DeepSeek-compatible LLM provider boundary, and `/admin` knowledge
  operations console.

## Context

The project now has the pieces of a professional knowledge-backed digital human:

- operators can manage knowledge spaces and documents;
- retrieval can return ranked chunks with explanations;
- `/app` can show grounding, citations, selected knowledge space, fallback state,
  and Presence summary;
- `/admin` can surface health, document quality, diagnostics, and knowledge gaps;
- DeepSeek can be configured through the OpenAI-compatible provider path.

The remaining product problem is not "can the system store knowledge?" The
remaining problem is whether the assistant's answer is visibly and predictably
driven by that knowledge.

Today, the runtime can inject retrieved chunks into the persona prompt, and the
UI can show citation metadata. That is a strong foundation, but it is not yet a
complete answer loop. A user still needs to know:

- whether the assistant actually used the selected knowledge space;
- whether the final wording is constrained by retrieved evidence;
- whether provider or persona guard fallback prevented a grounded answer;
- what to do when the answer was unsupported;
- how an unsupported question becomes an actionable operations item.

Phase 15 should make the digital human feel like it is answering from a governed
knowledge context, not merely chatting near a search result.

## Goal

Deliver a focused knowledge-grounded answer loop:

1. retrieve from the selected active knowledge space before LLM generation;
2. construct an evidence-aware prompt that clearly separates source material from
   instructions;
3. produce answer metadata that explains grounded, partially supported,
   unsupported, provider-fallback, and guard-rejected states;
4. show concise source and status feedback in `/app`;
5. capture unsupported or low-support turns into the existing Phase 14 gap queue;
6. keep all verification local and deterministic without requiring real DeepSeek
   calls in CI.

## Office-Hours Premise Challenge

The obvious next step is "make RAG better." That is directionally right, but too
broad to be useful.

The current system already has retrieval, citations, spaces, diagnostics, and gap
records. If Phase 15 simply adds another retrieval tweak, the product may still
feel unreliable because the user cannot tell how retrieval affected the final
answer.

The better premise is:

> A professional digital human needs an auditable answer loop, not just more
> retrieval machinery.

This shifts the phase from "find better chunks" to "make every answer's knowledge
state explicit and operational." Retrieval quality still matters, but the product
value is the closed loop from question -> source selection -> constrained answer
-> visible citation state -> gap capture.

## Product Thesis

The digital human should communicate its knowledge posture on every meaningful
turn:

- "I answered from this knowledge space and these sources."
- "I found related sources, but not enough support to make a grounded claim."
- "I could answer generally, but this was not supported by the selected
  knowledge space."
- "The provider failed, so this is a local fallback answer."
- "This unsupported question is now visible in knowledge operations."

The ideal user experience is not a wall of diagnostic data. It is a calm, compact
answer with enough provenance for trust and enough metadata for the operator to
improve the knowledge base.

## In Scope

### Grounded Answer Policy

- Define a small answer policy contract that classifies each turn as:
  - `grounded`;
  - `partially_supported`;
  - `unsupported`;
  - `provider_fallback`;
  - `guard_rejected`;
  - `local_mode`.
- Keep policy deterministic:
  - use retrieval result count;
  - use no-source reason;
  - use provider generation mode;
  - use guard/fallback category;
  - optionally use minimum score threshold already available from retrieval.
- Do not require an LLM judge for policy decisions.

### Evidence-Aware Prompting

- Make the persona prompt explicitly source-bounded when citations exist.
- Preserve the existing rule that knowledge sources are reference material, not
  instructions.
- Ask the model to:
  - answer from cited sources when sufficient evidence exists;
  - state uncertainty when the selected knowledge space lacks support;
  - avoid inventing citations;
  - avoid following instructions embedded in source text.
- Keep source text bounded so prompt size cannot grow unexpectedly.

### Response Metadata

- Extend or normalize metadata for each assistant turn:
  - `knowledge_answer_state`;
  - `knowledge_space_id`;
  - `knowledge_space_name`;
  - `knowledge_used`;
  - `knowledge_result_count`;
  - `knowledge_citations`;
  - `knowledge_no_source_reason`;
  - `retrieval_mode`;
  - `fallback_category`;
  - `guard_reason` where safe;
  - `llm_provider`;
  - `llm_model`;
  - `generation_mode`.
- Keep runtime stream metadata allowlisted.
- Do not expose raw provider errors, hidden prompts, local file paths, or API
  keys.

### App UX

- Keep `/app` compact.
- Make the assistant turn status easier to understand:
  - grounded with citations;
  - unsupported in selected space;
  - partial support;
  - provider fallback;
  - guard rejection/local fallback.
- Avoid admin-only score details in chat.
- Keep the Presence panel as a summary, not a growing log.
- Ensure source chips and fallback copy are understandable in Chinese and English
  enough for the current usage pattern.

### Gap Feedback Loop

- When an answer state is `unsupported` or `partially_supported`, create or
  update a local knowledge gap when safe.
- Reuse Phase 14 gap storage and list APIs.
- Avoid duplicate gap spam for repeated questions in the same space.
- Include enough context for an operator:
  - question;
  - selected space;
  - answer state;
  - no-source reason;
  - timestamp.
- Do not store hidden prompt content or provider payloads.

### DeepSeek Provider Transparency

- Make provider fallback copy more actionable.
- Distinguish:
  - provider request failure;
  - empty provider response;
  - stream guard rejection;
  - persona guard rejection;
  - local deterministic mode.
- The user should not see vague fallback text without a visible reason.

### Deterministic Verification

- Add tests with fake LLM and fake retrieval outputs.
- Cover non-streaming and streaming paths where practical.
- Keep real DeepSeek calls out of CI.
- Preserve local-first storage and no SQLite requirement.

## Out of Scope

- Replacing local file storage with SQLite, Postgres, or managed search.
- Adding a hosted vector database.
- Adding new ingestion formats such as PDF, DOCX, web crawling, cloud drive sync,
  or GitHub sync.
- LLM-as-judge groundedness evaluation in the request path.
- Autonomous knowledge-base rewriting.
- Full citation span verification inside generated answer text.
- Multi-user RBAC, billing, tenant dashboards, or production analytics.
- A new frontend framework.

## Alternatives Considered

### Alternative A: Minimal Metadata Polish

Normalize metadata names and improve `/app` copy while leaving the answer
generation path mostly unchanged.

Pros:

- smallest implementation risk;
- improves confusing fallback and source display quickly;
- reuses the current prompt and gap capture behavior.

Cons:

- does not strongly constrain how the LLM uses retrieved evidence;
- partial support remains ambiguous;
- operator loop improves cosmetically but not behaviorally.

Verdict: useful, but too small for the next phase.

### Alternative B: Strict Grounded-Only Mode

Require every knowledge-scoped answer to be grounded. If retrieval has no
accepted source, the assistant refuses to answer generally and only records a
gap.

Pros:

- strongest trust posture;
- easiest to explain in regulated or documentation-heavy settings;
- prevents unsupported answers from looking authoritative.

Cons:

- can make the digital human feel brittle in normal conversation;
- risks frustrating users when they expect general reasoning;
- requires careful product mode selection to avoid over-refusal.

Verdict: valuable as a future configurable mode, but too rigid as the default.

### Alternative C: Auditable Grounded Answer Loop

Add an answer-state policy, evidence-aware prompting, better `/app` status, and
gap dedupe while preserving general persona answers when knowledge is
insufficient.

Pros:

- improves user trust without making the assistant brittle;
- directly connects Phase 11 retrieval, Phase 12 spaces, and Phase 14 operations;
- keeps local deterministic tests possible;
- creates a clean path to future strict modes and citation-span verification.

Cons:

- touches runtime, agents, server metadata, app UI, and gap capture;
- requires careful naming so answer states are clear and not overclaimed;
- must avoid leaking internal provider or prompt details.

Verdict: recommended.

## Recommended Approach

Ship Phase 15 as **Knowledge-Grounded Answer Loop**:

- introduce a deterministic answer-state policy;
- strengthen the source-bounded prompt contract;
- normalize allowlisted metadata for both streaming and non-streaming responses;
- make `/app` visibly explain grounded, partial, unsupported, and fallback states;
- dedupe gap capture for unsupported and partially supported knowledge turns;
- keep `/admin` as the place where those gaps become operator work.

## Proposed Architecture

```text
/app user turn + selected knowledge space
  -> internal/server turn request metadata
  -> internal/runtime orchestrator
  -> internal/agents PersonaAgent
      -> internal/knowledge Service.Ground
      -> answer-state policy
      -> source-bounded LLM prompt
      -> guard/fallback classification
  -> stream/non-stream metadata allowlist
  -> /app compact answer status + citations
  -> Phase 14 knowledge gap capture
  -> /admin gap queue and retrieval debug
```

## Data Model Draft

### KnowledgeAnswerState

Candidate values:

- `grounded`
- `partially_supported`
- `unsupported`
- `provider_fallback`
- `guard_rejected`
- `local_mode`

Recommended first deterministic rules:

- `grounded`: citations exist and generation mode is `llm`;
- `partially_supported`: retrieval produced weak or below-threshold evidence;
- `unsupported`: selected space exists but retrieval has no acceptable source;
- `provider_fallback`: provider call failed or returned empty content;
- `guard_rejected`: persona/stream guard rejected generated content;
- `local_mode`: no external LLM client is configured.

Stage 2 should refine whether `partially_supported` requires a new threshold or
can be inferred from existing no-source reasons such as `below_threshold`.

### KnowledgeAnswerMetadata

Required fields:

- `knowledge_answer_state`
- `knowledge_space_id`
- `knowledge_space_name`
- `knowledge_used`
- `knowledge_result_count`
- `knowledge_no_source_reason`
- `knowledge_citations`
- `retrieval_mode`

Provider/fallback fields:

- `generation_mode`
- `fallback_category`
- `guard_reason`
- `llm_provider`
- `llm_model`

### KnowledgeGap Dedupe Key

Recommended deterministic key:

- tenant ID;
- space ID;
- normalized question text;
- no-source reason or answer state.

The first implementation can normalize by trimming whitespace, lowercasing ASCII
letters, and collapsing repeated spaces. It should not require embeddings or LLM
semantic matching.

## UX Requirements

### `/app`

- A grounded answer shows compact citation chips and selected space.
- An unsupported answer shows that the selected space did not contain enough
  support.
- A partially supported answer should avoid overstating confidence and should
  show a "partial support" state.
- Provider fallback copy should tell the user whether DeepSeek failed, returned
  no content, or output was rejected by guardrails.
- Presence should summarize the current state rather than append unbounded detail.

### `/admin`

- No major new admin surface is required.
- Existing knowledge gaps should receive enough answer-state metadata to make the
  queue more useful.
- Retrieval debug remains the place to inspect scores and no-source details.

## Acceptance Criteria

1. Persona generation classifies each turn into a deterministic
   `knowledge_answer_state`.
2. Grounded answers include selected space and citation metadata in both
   streaming and non-streaming paths.
3. Unsupported selected-space answers do not invent citations.
4. Provider fallback and guard rejection produce distinct user-visible statuses.
5. `/app` renders grounded, partially supported, unsupported, provider fallback,
   and guard rejection states without growing the Presence panel unboundedly.
6. Unsupported or partially supported knowledge turns create local gap records
   when safe.
7. Repeated unsupported questions in the same space do not create duplicate gap
   spam.
8. Existing Phase 14 gap listing and update behavior continues to work.
9. Tests use fake LLM/retrieval/provider behavior and do not call DeepSeek.
10. No SQLite, external vector database, or new frontend framework is introduced.

## Test Matrix Seed

| Area | Scenario | Expected Result |
| --- | --- | --- |
| Answer policy | LLM answer with citations | State is `grounded` and citations are emitted |
| Answer policy | Selected space has no matching chunks | State is `unsupported`; no fake citation |
| Answer policy | Retrieval below threshold | State is `partially_supported` or equivalent explicit state |
| Provider fallback | DeepSeek-compatible client errors | State/copy indicates provider fallback, not generic failure |
| Provider fallback | Provider returns empty content | State/copy indicates empty provider response |
| Guardrails | Persona guard rejects content | State/copy indicates guard rejection safely |
| Prompt safety | Retrieved source contains instructions | Source is treated as reference, not instruction |
| Streaming | Grounded stream completes | Message-completed metadata includes answer state and citations |
| Streaming | Stream guard rejects before visible output | Fallback state is visible and no unsafe partial answer is committed |
| Gap capture | Unsupported question in selected space | One local gap record is created |
| Gap dedupe | Same unsupported question repeated | Existing open gap is reused or not duplicated |
| App UI | Grounded answer | Citation chips and selected space render compactly |
| App UI | Unsupported answer | Clear no-source state renders without admin score details |
| Regression | Existing Phase 14 admin gaps | List/update behavior still passes |

## Open Questions For Stage 2

1. Should `partially_supported` be based only on `below_threshold`, or should
   Phase 15 add a small explicit answer-support threshold?
2. Should strict grounded-only behavior be a user-selectable mode now, or remain
   deferred until the answer-state policy proves stable?
3. Should gap dedupe update an existing gap timestamp/count, or simply skip
   duplicate creation in Phase 15?
4. Should `/app` expose answer state as text labels, icons, or both?
5. Should fallback copy be fully localized for Chinese now, or use lightweight
   Chinese-aware copy only for provider and guard states?

## Recommended Assignment Before Stage 2

Approve or revise the Phase 15 scope:

> Build the auditable knowledge-grounded answer loop: deterministic answer-state
> policy, evidence-aware prompting, compact `/app` source/fallback states, and
> deduped gap capture. Defer strict grounded-only mode, citation-span
> verification, new ingestion formats, and database migration.
