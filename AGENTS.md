# Sprint Workflow: SDD (gstack) + TDD (Superpowers)

Run every feature through this fixed pipeline. Each stage feeds the next.
Do NOT skip stages. Do NOT write code before the plan is approved.
Stop at each [GATE] and wait for my explicit approval before continuing.

In Codex, invoke skills with the `$` prefix and the gstack namespace,
e.g. `$gstack-office-hours`. Use `$skills` to list what's available.

## Stage 1 — SPEC (gstack, SDD)
- Run `$gstack-office-hours`: interrogate the requirement, challenge my
  framing, surface hidden assumptions, propose alternatives. Write a design doc.
- [GATE] I approve the spec.

## Stage 2 — PLAN & REVIEW (gstack, SDD)
- Run `$gstack-autoplan` (runs CEO + eng + design review, locks architecture,
  data flow, edge cases, and produces a test matrix from the design doc).
- Output: an approved plan with an explicit test plan.
- [GATE] I approve the plan.

## Stage 3 — BUILD (Superpowers, TDD)
- Implement strictly with Superpowers `$test-driven-development`:
  RED → GREEN → REFACTOR. No production code without a failing test first.
  Tests come from Stage 2's test matrix.
- Use `$executing-plans`; for independent workstreams use
  `$dispatching-parallel-agents`.

## Stage 4 — CODE REVIEW (gstack)
- Run `$gstack-review` (auto-fix obvious issues, flag the rest).
- Codex IS the model here, so a separate `$codex` second-opinion isn't useful;
  instead optionally run a fresh-context review pass.
- [GATE] I approve fixes for anything flagged [ASK].

## Stage 5 — QA (gstack)
- Run `$gstack-qa <staging-url>`: real-browser flows, find bugs, fix with
  atomic commits, generate a regression test per fix.

## Stage 6 — SECURITY (gstack)
- Run `$gstack-cso`: OWASP Top 10 + STRIDE. Each finding needs a concrete
  exploit scenario and a fix.
- [GATE] I approve before shipping if any finding is >= high severity.

## Stage 7 — SHIP (Superpowers + gstack)
- Run Superpowers `$verification-before-completion`, then `$gstack-ship`
  (sync main, run tests, coverage audit, open PR).
- On approval, `$gstack-land-and-deploy`.

## Rules
- One feature = one pass through this pipeline.
- Each stage must reference the artifact from the previous stage; if it can't
  find it, stop and tell me.
- For trivial changes (typo, config, one-liner), skip to Stage 3+4 only and
  say you're doing so.
