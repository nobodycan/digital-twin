# Phase 26 Security Posture Report

Date: 2026-07-12

Scope: Phase 26 diff and the authentication boundary inherited by its new admin
trend endpoint.

## Finding 1: Admin Trends Are Anonymous With Default Configuration

- Severity: HIGH
- Confidence: 10/10
- Status: VERIFIED; accepted for current local stage, deferred before public deployment
- Category: OWASP A01 Broken Access Control / A05 Security Misconfiguration
- Evidence: [configs/app.yaml](../../configs/app.yaml:3), [internal/server/server.go](../../internal/server/server.go:1924), [internal/server/server.go](../../internal/server/server.go:189)

### Exploit Scenario

1. Deploy the repository with the committed default configuration, where
   `server.api_key` is empty.
2. Send an unauthenticated GET request to
   `/admin/knowledge/quality-trends`.
3. `protectedRoute` recognizes the admin path, but `authorizedKey` returns
   `anonymous, true` when no API keys are configured.
4. The attacker receives tenant-scoped trend counts, space identifiers, eval
   case identifiers, recurrence summaries, and trace IDs.

The same inherited behavior applies to existing admin routes; Phase 26 adds a
new read-only information surface to that boundary.

### Remediation Options

- Require a non-empty API key for admin routes outside an explicitly local
  development mode, returning a startup/configuration error otherwise.
- Or add a separate authenticated admin middleware with an intentional local
  development override and document the deployment requirement.

This requires a product/deployment decision because making the default strict
could change local development behavior. No code change was applied in Phase 26
without approval. The current-stage decision is to defer the global auth change;
this is not approval to expose the default configuration to a public network.

## Verified Safe Areas

- Trend requests derive tenant from server context and ignore a client-supplied
  `tenant_id` query parameter.
- Space filtering occurs before aggregation and supports an explicit all-space
  mode distinct from the `default` space.
- Observation records validate controlled statuses/reasons and do not persist
  evaluator messages or raw answer content.
- Admin UI renders trend values with `textContent`, not HTML interpolation.
- File writes use temp files and rename, with replay idempotency by tenant/run/
  case/promotion/revision identity.

## Verification Limits

- No staging URL was provided, so production/staging browser QA was not run.
- Local HTTP QA passed against the current source on `127.0.0.1:8080`.
- `golangci-lint` is not installed locally; `go vet ./...` passed.
- `go test -race` is unsupported by the workstation's `windows/386` toolchain.
