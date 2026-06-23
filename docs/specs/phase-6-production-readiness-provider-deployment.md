# Phase 6 Production Readiness, Provider Integration, and Deployment Spec

Date: 2026-06-22

## Current State

`digital-twin` has completed a local-first professional digital-human foundation through Phase 5:

- Core contracts, registry, local file storage, memory, event bus, router, skills, agents, orchestrator, CLI, HTTP API, SSE, Web app, admin console, mock voice, presentation events, local evals, release gates, rollback/feedback records, governance decisions, and tool authorization hooks exist.
- The product is still intentionally mock/local-first. It does not yet include real TTS/ASR providers, production deployment automation, production-grade auth/RBAC, external eval platforms, cloud moderation, compliance certification, or a complete governance dashboard.
- The user preference remains: do not introduce SQLite at this stage; keep local storage unless a later gate explicitly approves a database change.

Phase 6 should not pretend the system is "enterprise production ready" in one pass. It should make the current local product deployable, configurable, observable, and provider-ready while preserving deterministic tests and local operation.

## Office-hours Framing

The tempting framing is: "Now add real providers and ship it." That is too broad. Real providers introduce secrets, cost, latency, availability, policy drift, and flaky tests. Deployment introduces environment differences, data persistence expectations, health checks, logs, and rollback. Auth/RBAC introduces user/session models and operational risk.

The better Phase 6 question is:

> What is the smallest production-shaped slice that lets a professional digital-human runtime run outside a developer shell without losing the local-first safety and testability built in Phases 0-5?

## Premise Challenge

- Real providers are not the product. They are interchangeable adapters behind the existing voice/LLM/presentation contracts.
- Deployment is not only Docker. Deployment readiness means config validation, secret redaction, readiness probes, graceful shutdown, persistent local data volumes, release gates, and rollback instructions.
- Production auth is a separate product surface. Phase 6 can harden API/admin access boundaries without committing to OAuth, billing, SSO, or a full multi-tenant identity platform.
- External eval or cloud moderation can be useful later, but requiring them now would weaken the current deterministic CI path.
- A database migration would be premature. Local file storage is still the accepted constraint and should be made more operationally explicit before switching persistence technology.

## Recommended Scope: Phase 6A Provider and Deployment Readiness MVP

Phase 6 should be implemented as a production-readiness wedge with five linked tracks.

1. **Provider runtime boundary**
   - Add production-shaped provider adapters only behind existing interfaces.
   - Keep `local`/`mock` as the default provider.
   - Require fake-server tests for every real-provider adapter.
   - Never require real credentials or real network calls in CI.

2. **Configuration and secrets hardening**
   - Add explicit environment profiles such as `local`, `staging`, and `production-like`.
   - Validate provider configuration at startup and expose actionable errors.
   - Redact secrets from logs, health output, metrics, reports, and panic paths.
   - Provide `.env.example` without real keys.

3. **Deployable runtime package**
   - Add Dockerfile and local production-like compose setup.
   - Mount local data directories as volumes.
   - Expose HTTP health/readiness endpoints suitable for container orchestration.
   - Keep the deployment path runnable without external databases.

4. **Operational observability**
   - Add request IDs, provider latency/error counters, readiness status, and release-gate status to local metrics or structured logs.
   - Track provider fallback behavior honestly.
   - Label cost data as estimated unless it comes from actual provider usage metadata.

5. **Operator safety loop**
   - Wire Phase 5 release gate output into deployment readiness.
   - Add runbooks for local production-like startup, provider failure, rollback, and secret rotation.
   - Add smoke tests that prove the deployed runtime can answer, stream, serve admin static assets, and preserve local governance decisions.

## Non-goals

- No SQLite, Postgres, cloud database, or migration framework unless a later approved spec changes the persistence decision.
- No real provider calls in automated tests.
- No mandatory paid provider accounts.
- No OAuth/SSO/billing system.
- No SOC 2, GDPR, or security certification claim.
- No Kubernetes production platform unless the deployment target is explicitly approved.
- No full external eval platform or cloud moderation integration.
- No real 3D/Live2D/video avatar provider integration.

## User Value

After Phase 6, a developer or operator should be able to:

- Run the digital-human system in a production-like container setup.
- Choose local/mock providers by default and opt into configured real adapters safely.
- See whether the runtime is ready before traffic reaches it.
- Prove release gates passed before deployment.
- Diagnose provider failures without leaking secrets.
- Roll back to the previous local configuration or persona state using documented steps.

## Hidden Assumptions

- Phase 5 work is available on the implementation branch or has been merged before Phase 6 build begins.
- The first real-provider adapter can be selected during Stage 2; the Stage 1 recommendation is to start with TTS or ASR because the current UI already exposes audio state and provider metadata.
- Local file storage remains acceptable for production-like local deployment, with documented backup/restore boundaries.
- `go test ./...` remains the primary correctness signal.
- Provider tests will use `httptest.Server` or equivalent local fakes.

## Alternatives Considered

### Approach A: Provider and Deployment Readiness MVP (recommended)

Build provider adapters, config/secrets hardening, Docker/compose, readiness probes, observability, and runbooks as one small production-shaped slice.

Completeness: 8/10.

This is the best next step because it closes the gap between local product and deployable product without pretending to solve enterprise identity, compliance, or cloud operations.

### Approach B: Production Control Plane First

Build deeper admin auth, RBAC, tenant/user management, audit exports, and operator roles before touching providers or deployment.

Completeness: 7/10.

This improves safety, but it delays the first deployable runtime. It is more appropriate after Phase 6A proves the container/config/provider boundary.

### Approach C: External Eval and Cloud Moderation First

Integrate an external eval platform and cloud content moderation before deployment work.

Completeness: 6/10.

This could improve governance later, but it adds third-party dependencies before the deployment and provider lifecycle is stable.

### Approach D: Real Provider Demo First

Wire one vendor directly into the UI to produce impressive audio or avatar output quickly.

Completeness: 4/10.

This creates visible progress, but it risks provider lock-in, flaky tests, secret handling mistakes, and a false sense of production readiness.

## Acceptance Criteria for the Future Phase 6 Plan

Stage 2 must produce a plan that:

- Keeps real-provider tests deterministic and credential-free.
- Preserves local storage and clearly documents backup/restore behavior.
- Adds deployment artifacts that can be run locally.
- Adds readiness checks that fail when required provider/config inputs are invalid.
- Adds secret-redaction tests.
- Adds smoke tests for the production-like runtime path.
- Documents exact verification commands.
- States which provider adapter is first and why.

## Open Questions for Stage 2

- Which provider adapter should be first: TTS, ASR, LLM, or a presentation/avatar adapter?
- Should the deployment target be local Docker Compose only, or should a VPS/cloud target be included?
- Should API key auth be hardened further in Phase 6A, or should full auth/RBAC wait for Phase 6B?
- What is the minimum acceptable readiness signal: `/ready` only, or `/ready` plus provider self-checks and release-gate report checks?
- Should provider failure fall back to mock/local behavior, or fail closed for production-like profiles?

## Gate

This is a Stage 1 SDD artifact. Do not implement Phase 6 until the user approves this spec and Stage 2 produces an approved plan and test matrix.
