# Phase 22 Knowledge Repair Inbox Design

Date: 2026-07-09

Status: Draft; awaiting spec approval

Source spec: [Phase 22 Knowledge Repair Inbox Spec](../specs/phase-22-knowledge-repair-inbox.md)

## Design Intent

Phase 22 turns weak-answer visibility into repair work. Phase 21 made answer
trust visible over time; the repair inbox should make the next operator action
obvious without pretending the system can automatically know the truth.

The interface should feel like a compact operations queue. It should not become
a dashboard, task manager, or autonomous content writer. Each item exists
because a real answer had weak support, and each action should move the local
knowledge base closer to being inspectable and reviewed.

## User

Primary user: a local operator maintaining knowledge for a professional digital
human.

This user is not trying to analyze charts. They are trying to answer:

- what needs repair;
- why it needs repair;
- what source or note would fix it;
- whether the fix improved the original answer.

## Core Surface

Add a Knowledge Repair Inbox section to `/admin`, near the current knowledge
gap queue and answer timeline.

Default view:

- unresolved items first;
- high-priority items before low-priority items;
- repeated weak answers above one-off gaps;
- items with no linked evidence above items already tied to a document;
- resolved and ignored items available through filters but quiet by default.

## Repair Item Anatomy

Each item should show:

- priority label and reasons;
- bounded original question summary;
- status;
- answer state;
- no-source or weak-support reason;
- occurrence count;
- last seen time;
- linked or candidate documents;
- one primary next action.

Suggested compact layout:

```text
[high] unsupported · no_matching_chunks · open
What is the refund window?
Seen 3 times · last seen 2026-07-09 · no linked evidence

Primary: Run diagnostics
Secondary: Investigate · Create note · Ignore
```

## Workflow

1. Operator opens `/admin`.
2. Inbox shows unresolved work ordered by actionability.
3. Operator opens a high-priority item.
4. Operator runs diagnostics from the original question.
5. If diagnostics find a candidate source, operator inspects or reviews it.
6. If no source exists, operator creates a workbench note.
7. Operator retests the original question locally.
8. Operator resolves the item with an evidence document and note, or ignores it
   with a reason.
9. The answer timeline remains available to confirm whether the weak pattern
   stops appearing.

## Interaction Principles

- Keep every action explicit and reversible where possible.
- Show why an item is prioritized; do not show opaque scores.
- Prefer deterministic labels over confidence percentages.
- Make missing evidence feel like work to do, not a product failure.
- Keep review-gated content protected: show reason and counts, not raw gated
  text.
- Preserve the existing gap queue while the inbox proves itself.

## Filters

Minimum useful filters:

- space;
- status;
- reason;
- weak answer state;
- unresolved only;
- linked evidence only;
- limit.

Filters should use small native controls consistent with the existing admin
page. The design should avoid a new navigation model.

## Empty States

No unresolved items:

```text
No unresolved repair items
```

No matching filtered items:

```text
No repair items match these filters
```

Retest has not run:

```text
No retest run yet
```

## Safety Copy

Use careful operational language:

- `support improved`
- `active reviewed source found`
- `no reviewed source found`
- `review required`
- `resolved with evidence`

Avoid overclaiming:

- do not say `answer is true`;
- do not say `verified by AI`;
- do not say `confidence score`.

## Deferred Design

- analytics charts;
- assignment and ownership;
- due dates;
- background retest queue;
- automatic source discovery;
- LLM-authored repair notes;
- cross-space clustering.

## Handoff To Stage 2

Stage 2 should lock:

- exact repair item schema;
- whether the endpoint is `/admin/knowledge/repairs` or a projection on gaps;
- priority ordering;
- whether retest results persist;
- how candidate documents are sourced;
- UI placement and filter defaults;
- deterministic test matrix.
