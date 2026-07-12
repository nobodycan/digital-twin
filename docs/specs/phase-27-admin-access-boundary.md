# Phase 27 Admin Access Boundary Spec

Date: 2026-07-12

Status: Approved; Stage 1 gate passed

Prior phase: [Phase 26 Knowledge Quality Trends Spec](./phase-26-knowledge-quality-trends.md)

Security input: [Phase 26 Security Posture Report](../security/phase-26-knowledge-quality-trends-security.md)

## Problem Statement

The local-first server currently uses one optional `server.api_key` for chat,
experience, and admin routes. When no key is configured, `authorizedKey`
accepts an anonymous caller. Configuration validation prevents an unauthenticated
server from binding directly to a non-loopback host, but it does not establish a
separate admin trust boundary and cannot detect exposure through a reverse proxy,
tunnel, port forward, or incorrectly classified deployment.

This makes the committed local defaults convenient, but it creates an unsafe
deployment failure mode: a staging or production process can expose knowledge,
memory, persona, tool-policy, audit, repair, and quality-trend operations without
an administrator credential.

Phase 27 introduces an explicit, environment-aware admin access boundary. Local
development on a loopback listener remains usable without setup. Staging and
production fail closed: admin authentication must be configured before startup,
and every admin page and API request must enforce it.

## Target User And Narrowest Wedge

The target user is the operator who runs this project locally today and may later
place it behind a shared staging or production endpoint.

The narrowest complete wedge is:

1. preserve anonymous admin access only in an explicitly local environment bound
   to a loopback host;
2. require an admin credential in staging and production regardless of bind host;
3. reduce `/admin` to a data-free unlock shell and protect every `/admin/*`
   bootstrap, data, and mutation route;
4. let the existing browser admin console supply a credential without placing it
   in a URL, log, cookie, or durable browser storage; and
5. return stable, non-secret errors when authentication is missing or invalid.

## Office-Hours Findings

### Actual Job

The goal is not to build identity management. The operator needs a deployment
mistake to fail visibly before sensitive management surfaces become reachable,
without turning local development into an account-setup workflow.

### Premises Challenged

1. **Loopback binding proves the admin surface is private.** Rejected. Reverse
   proxies and tunnels can expose a loopback listener.
2. **The existing shared API key is already a complete admin boundary.** Rejected.
   It is optional, protects mixed route classes, and leaves the `/admin` document
   itself outside the current protected-route matcher.
3. **The safest fix is to require credentials in every local run.** Rejected.
   That is secure but unnecessarily damages the repo's local-first workflow.
4. **A login system or RBAC is required.** Rejected. Phase 27 has one operator
   role and one deployment boundary; accounts, sessions, and permissions would
   add storage and lifecycle risk without solving a present requirement.
5. **A browser can safely remember the admin secret permanently.** Rejected.
   Local storage, query parameters, and non-HttpOnly cookies increase exposure.
   The Phase 27 browser credential is tab-memory only and is lost on reload.

## Goals

1. Define one deterministic admin authentication policy from environment, bind
   host, and configured admin credential.
2. Fail server configuration validation in staging or production when no admin
   credential exists, even when the process binds to loopback.
3. Permit anonymous admin access only when both conditions hold: environment is
   `local` and server host is loopback.
4. Protect `/admin/` and every descendant route with one centralized route
   classification while keeping the exact `/admin` unlock shell data-free.
5. Keep chat and experience authentication behavior backward compatible.
6. Support a browser-supplied Bearer credential held only in JavaScript memory.
7. Prevent credentials from entering URLs, response bodies, logs, audit metadata,
   request IDs, persisted files, or rendered HTML.
8. Emit actionable startup and HTTP errors without revealing whether a candidate
   credential was close to or equal to any configured secret.

## Non-Goals

- User accounts, passwords, OAuth, OIDC, SSO, invitations, or password reset.
- Roles, permissions, teams, policy inheritance, or per-route authorization.
- Durable browser sessions, refresh tokens, remember-me behavior, or cookies.
- Credential creation, rotation, revocation, hashing, or secret-manager APIs.
- Multiple admin identities or administrator attribution beyond the existing
  sanitized API-key principal behavior.
- Changing tenant resolution or adding client-selectable tenant IDs.
- Protecting public health/readiness endpoints or redesigning chat authentication.
- TLS termination, reverse-proxy configuration, cloud deployment, or network ACLs.
- Retrofitting CSRF protection, because Phase 27 does not use ambient cookies.

## Chosen Approach

Phase 27 adopts **environment-aware Admin Fail-Closed**.

The admin policy has two modes:

```text
local + loopback + no admin credential -> anonymous local admin allowed
all other configurations              -> admin credential required
```

An explicitly configured admin credential always enables authenticated mode,
including local loopback development. Staging and production never receive an
anonymous override. A non-loopback host never receives an anonymous override,
regardless of the environment label.

## Configuration Contract

Add a distinct optional configuration field:

```yaml
server:
  admin_api_key: ""
```

Environment override:

```text
DIGITAL_TWIN_SERVER_ADMIN_API_KEY
```

The value is secret and must use the same redaction guarantees as existing
provider and server API keys.

### Compatibility Rule

For one migration phase, a non-empty `server.admin_api_key` is preferred and a
non-empty legacy `server.api_key` may satisfy the admin credential requirement.
This prevents existing secured deployments from failing immediately. The
effective admin keys are not combined into a broader privilege set:

- when `admin_api_key` is set, only that value authorizes admin routes;
- otherwise, a configured `api_key` authorizes admin routes as a compatibility
  fallback;
- when neither is set, only the explicit local-loopback anonymous mode applies.

The server must not print which source supplied the effective credential.

### Validation Matrix

| Environment | Host | Effective admin key | Result |
| --- | --- | --- | --- |
| `local` | loopback | absent | valid; anonymous local admin |
| `local` | loopback | present | valid; authenticated admin |
| `local` | non-loopback | absent | startup error |
| `local` | non-loopback | present | valid; authenticated admin |
| `staging` | any | absent | startup error |
| `staging` | any | present | valid; authenticated admin |
| `production` | any | absent | startup error |
| `production` | any | present | valid; authenticated admin |

Unknown non-local environment names must be treated as requiring a credential;
an unrecognized label must not weaken the boundary.

## HTTP Authentication Contract

### Route Classification

Admin API classification is prefix-safe and centralized:

- the exact `/admin` path is the public, data-free unlock shell;
- `/admin/` is protected;
- every path beginning `/admin/` is admin;
- lookalikes such as `/administrator` and `/administer` are not admin routes.

No data or mutation handler may opt itself out. New `/admin/*` handlers inherit
protection without editing a route allowlist. The shell may reveal only static
assets and whether authentication is required; it must not serialize admin state.

### Credential Transport

Authenticated admin requests accept the existing transports:

```text
Authorization: Bearer <admin-key>
X-API-Key: <admin-key>
```

`Authorization: Bearer` takes precedence when both headers are present. Empty,
malformed, or invalid values are unauthorized. Credential comparison must avoid
obvious timing-dependent string comparison; Stage 2 must select and test the
constant-time comparison primitive.

Credentials are never accepted through query parameters, form fields, URL
fragments, or cookies.

### Responses

- Missing or invalid credentials on protected `/admin/*` routes return `401` with
  `{"error":"unauthorized"}`.
- Authenticated requests continue to their existing handler behavior.
- Rate limiting remains after successful authentication and returns the existing
  `429` response.
- Responses do not distinguish missing from invalid credentials.
- `WWW-Authenticate` may identify the Bearer scheme but must not include secret,
  tenant, or deployment details.

## Admin Browser Experience

When admin authentication is required, loading `/admin` without a credential
cannot itself reveal the full admin document. Because browser navigation cannot
attach a Bearer header, Phase 27 uses a small public unlock shell at `/admin`:

1. the shell contains only product-neutral credential input and status text;
2. the operator enters the admin key;
3. the key is held in a module variable for the lifetime of the page only;
4. the shell calls an authenticated bootstrap endpoint under `/admin/`;
5. after success, the existing admin application initializes and every fetch
   attaches `Authorization: Bearer <key>`;
6. `401` clears the in-memory key and returns to the locked state.

The shell must not embed admin data or server configuration. The key must not be
stored in local storage, session storage, IndexedDB, cookies, DOM attributes, or
the URL. Reloading or closing the tab requires re-entry.

In anonymous local-loopback mode, the same page skips the unlock prompt and
initializes the existing admin application directly. The server supplies only a
boolean capability such as `admin_auth_required`; it never supplies credential
material or the credential source.

Stage 2 must choose the smallest bootstrap endpoint or shell split that preserves
the existing static-file organization and does not duplicate the admin UI.

## Security And Privacy Boundaries

- Configuration summaries redact `admin_api_key` exactly as other secrets.
- Startup errors state that a credential is required and name the config field,
  but never include its value.
- Access logs, audit records, traces, metrics labels, and panic output must not
  capture credential headers.
- Authentication occurs before tenant-scoped admin handlers execute.
- Anonymous local mode is a deployment capability, not an identity; it must not
  be represented as an authenticated administrator outside that mode.
- The UI must use safe DOM APIs and must not interpolate the credential into HTML.
- Existing tenant isolation remains mandatory after authentication; possessing an
  admin key does not permit selecting another tenant through request input.

## Failure Behavior

- Invalid deployment configuration stops startup before the listener opens.
- Missing or invalid request credentials fail before rate-limit accounting and
  before handler side effects.
- A browser bootstrap `401` leaves the admin application uninitialized.
- A later `401` locks the page and discards the in-memory credential.
- A malformed auth header never falls back to anonymous mode when authentication
  is required.
- Configuration or auth failures never fall back from admin key to anonymous
  merely because the bind address is loopback in a non-local environment.

## Compatibility

- Default `configs/app.yaml` remains valid for local loopback startup.
- Existing local scripts continue to work without an admin key when they use the
  default local listener.
- Existing secured deployments using only `server.api_key` remain authenticated
  through the compatibility fallback.
- Chat and experience routes retain existing `server.api_key` behavior.
- Existing admin APIs retain request and response payloads after authentication.
- The compatibility fallback is documented as transitional; removing it requires
  a later approved phase and migration notice.

## Acceptance Criteria

1. Default local-loopback startup succeeds and the local admin UI remains usable
   without a credential.
2. Staging and production startup fail when neither admin nor legacy server key is
   configured, including when bound to `127.0.0.1`.
3. A local non-loopback listener fails without an effective admin key.
4. Unknown non-local environments fail closed without an effective admin key.
5. The exact `/admin` path returns only the unlock shell; `/admin/` and arbitrary
   future `/admin/*` paths are protected; prefix lookalikes are not.
6. An invalid or missing key returns the same stable `401` response before an
   admin handler runs.
7. `admin_api_key` overrides the legacy key for admin authorization instead of
   accepting both.
8. Legacy `server.api_key` remains a valid admin fallback only when no dedicated
   admin key is configured.
9. Chat and experience authentication tests remain unchanged in behavior.
10. The authenticated admin UI attaches the key to all admin fetches, clears it
    after `401`, and never persists it in browser storage or URLs.
11. Anonymous local mode bypasses the unlock interaction without weakening any
    non-local or non-local-environment configuration.
12. Config summaries, logs, HTTP responses, audit events, and rendered HTML do not
    contain the admin credential.
13. Existing admin endpoint tests pass when configured with the correct boundary
    mode and no endpoint payload contract changes.
14. `go test ./...`, `go vet ./...`, frontend syntax checks, and CI lint pass.

## Stage 2 Test Matrix Requirements

Autoplan must turn the acceptance criteria into explicit tests covering:

- the complete environment/host/key validation table;
- dedicated-key precedence and legacy-key fallback;
- route classification boundary and prefix-lookalike cases;
- missing, malformed, conflicting, valid, and invalid auth headers;
- auth-before-handler and auth-before-rate-limit ordering;
- constant-time credential comparison through the selected helper contract;
- admin unlock, authenticated bootstrap, fetch attachment, reload, and relock;
- no local/session storage writes and no credential-bearing URLs;
- secret redaction in configuration summaries and controlled errors;
- regression coverage for chat, experience, tenant isolation, and all existing
  admin API families.

## Dependencies And Risks

- The existing admin frontend assumes anonymous same-origin fetches; Stage 2 must
  inventory every admin request path before changing the fetch wrapper.
- A public unlock shell intentionally reveals that an admin surface exists. It
  must expose no operational data and is not a replacement for network controls.
- Static assets needed by the shell must not accidentally contain preloaded admin
  state.
- API keys are bearer secrets. Phase 27 improves enforcement but does not provide
  rotation, attribution, phishing resistance, or multi-user accountability.
- Production still requires TLS and responsible secret injection outside this
  repository.

## Alternatives Rejected

### Require A Key In Every Environment

This is simpler but imposes credential setup on every local deterministic run and
does not materially improve the explicitly loopback-only development case.

### Full Login And RBAC

This would add identity storage, password or identity-provider integration,
sessions, cookies, CSRF controls, role design, and migration concerns. Those are
valid future capabilities, but they are not required to close the current
anonymous-admin deployment risk.

## Stage 1 Gate

Approve this Phase 27 spec to enter Stage 2 `$gstack-autoplan`. No production code
may be changed until the resulting plan and test matrix receive separate approval.
