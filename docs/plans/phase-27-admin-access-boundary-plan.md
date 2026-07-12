# Phase 27 Admin Access Boundary Implementation Plan

Date: 2026-07-12

Status: Approved; implemented in `b1c8365`; awaiting ship

Branch: `codex/phase-27-admin-access-boundary`

Source spec: [Phase 27 Admin Access Boundary Spec](../specs/phase-27-admin-access-boundary.md)

Source design: [Phase 27 Admin Access Boundary Design](../design/phase-27-admin-access-boundary.md)

## Autoplan Verdict

Proceed with the approved environment-aware fail-closed boundary. Keep Phase 27
focused on one operator credential and one local anonymous exception. Do not add
accounts, cookies, sessions, RBAC, proxy trust, or credential lifecycle APIs.

The implementation should separate runtime and admin authorization in the server
middleware, but reuse configuration parsing, secret redaction, request headers,
rate limiting, and the existing static admin application.

## Premise Review

| Premise | Verdict | Evidence |
| --- | --- | --- |
| Anonymous admin is the next highest-risk gap | Confirmed | Phase 26 security report records a verified high-severity access-control finding |
| Loopback binding is sufficient for deployment safety | Rejected | A reverse proxy or tunnel can expose a loopback listener |
| Full identity is required | Rejected | Current product has one configured tenant/operator and no multi-user requirement |
| Local anonymous use should survive | Confirmed | Default quick start and deterministic QA depend on frictionless loopback startup |
| Existing server key needs migration compatibility | Confirmed | Current secured deployments and smoke tooling know only `server.api_key` |

## Scope Decision

Mode: **Hold scope with selective implementation clarification**.

The approved product direction stands. Autoplan resolves one browser ambiguity:
the exact `/admin` response and static assets may contain the static console
structure, but must contain no tenant data, configuration values, or credential.
The application remains visually locked and performs no admin request until
capability discovery and, when required, credential validation succeed. This is
safer and smaller than authenticated HTML-fragment injection and does not weaken
the approved data boundary.

## What Already Exists

- `internal/config` parses YAML and environment overrides, validates loopback
  binding, summarizes configuration safely, and redacts configured secrets.
- `cmd/server.buildHandler` converts the legacy `server.api_key` into the handler's
  current runtime key set.
- `internal/server.Handler.ServeHTTP` authenticates protected routes before rate
  limiting and handler dispatch.
- `protectedRoute` already classifies chat, experience, and most admin APIs, but
  uses an enumerated mixed policy and does not protect the exact `/admin` page.
- `authorizedKey` already accepts Bearer and `X-API-Key` transports but uses a map
  lookup and turns an empty key set into anonymous access for every route class.
- `web/admin.js` owns all admin requests, including a shared `postJSON` helper,
  but direct GET calls still use `fetch` independently and startup requests fire
  immediately at module load.
- `web/admin.html` is static and contains placeholders only; operational data is
  loaded through `/admin/*` APIs.

## Not In Scope

- User identity, passwords, OAuth/OIDC, SSO, roles, permissions, or invitations.
- Cookie sessions, refresh tokens, CSRF tokens, or persistent browser credentials.
- API-key generation, rotation, revocation, hashing, or secret-manager integration.
- Reverse-proxy trusted headers, TLS, cloud deployment, network ACLs, or CORS work.
- Per-tenant key selection or client-controlled tenant context.
- Any knowledge, repair, eval, or trend behavior change.
- Changing chat/experience key semantics beyond keeping them compatible.

## Architecture

```text
                         config.Load
                             |
             +---------------+----------------+
             |                                |
     runtime API key                  effective admin key
     server.api_key              admin_api_key || api_key
             |                                |
             v                                v
     RuntimeAuthPolicy                 AdminAuthPolicy
 chat / experience routes       local+loopback anonymous?
             |                                |
             +---------------+----------------+
                             v
                    Handler.ServeHTTP
                             |
             +---------------+----------------+
             |                                |
       exact /admin and                 /admin/* data
       static web assets                and mutations
             |                                |
       public static shell              admin auth first
             |                                |
       GET /admin-access  ---- boolean ----> unlock/init
                                             |
                                     existing handlers
```

### Server Policy Types

Extend `server.Config` with explicit fields rather than asking the HTTP layer to
infer deployment policy:

```go
type Config struct {
    // existing fields...
    APIKeys             []string
    AdminAPIKeys        []string
    AllowAnonymousAdmin bool
}
```

`cmd/server` computes these values from validated `config.AppConfig`:

- runtime keys: non-empty `server.api_key` only;
- admin keys: `server.admin_api_key` when present, otherwise non-empty
  `server.api_key` as migration fallback;
- anonymous admin: true only for `environment == local`, a loopback host, and no
  effective admin key.

The policy computation belongs in `internal/config` as exported, deterministic
helpers or methods so configuration tests can prove the matrix without building
the full application.

### Route Classification

Replace the mixed `protectedRoute` decision with two exact helpers:

```text
adminRoute(path)   = path == "/admin/" or prefix "/admin/"
runtimeRoute(path) = current chat and experience routes only
```

The exact `/admin` shell and `GET /admin-access` are public. Static `/web/*`
assets remain public. Every path under `/admin/`, including unknown future paths,
passes admin authentication before mux dispatch. Prefix lookalikes remain public
unless separately classified.

### Credential Validation

Create one header parser and one constant-time key matcher:

1. Read `Authorization`.
2. Accept it only when it has a case-sensitive `Bearer ` prefix and a non-empty
   trimmed value.
3. If Authorization is absent, read trimmed `X-API-Key`.
4. If Authorization is present but malformed, do not fall back to `X-API-Key`.
5. Compare candidate bytes against every configured key using
   `crypto/subtle.ConstantTimeCompare`, accumulating matches without early exit.
6. Return only the candidate-derived rate-limit principal after a match.

The principal used for rate limiting must not be emitted in responses or logs.
Admin and runtime requests share the parser/matcher but use separate key sets.

### Middleware Order

```text
request ID
  -> classify route
  -> authenticate required policy
  -> rate limit authenticated/allowed principal
  -> mux handler
```

An admin auth failure never increments rate-limit state and never reaches an
admin service. Anonymous local admin uses a fixed internal principal only inside
the allowed mode.

## Configuration And Migration

Add `AdminAPIKey string` to `config.ServerConfig`.

Supported configuration names:

```text
YAML: server.admin_api_key
Environment: DIGITAL_TWIN_SERVER_ADMIN_API_KEY
Compatibility alias: SERVER_ADMIN_API_KEY
```

Update `configs/app.yaml` with an empty local default. Add the value to
`SafeSummary`, `secretValues`, and redaction tests.

Validation order:

1. validate port and host;
2. derive whether environment is exactly local;
3. derive loopback host;
4. derive effective admin key with dedicated-key precedence;
5. reject absent admin auth unless exact local-loopback exception applies;
6. retain the existing non-loopback runtime-key requirement only if chat and
   experience are still intended to be protected on that listener.

Autoplan decision: **unknown environment names fail closed**. Only normalized
`local` receives the exception. Existing aliases `prod`, `production`, `stage`,
and `staging` remain non-local. Error text must name the problem, cause, and fix:

```text
server.admin_api_key is required outside local loopback mode; set
DIGITAL_TWIN_SERVER_ADMIN_API_KEY or server.admin_api_key
```

Do not mention whether the legacy fallback was inspected or expose values.

## Public Capability Endpoint

Add:

```text
GET /admin-access
```

Response:

```json
{"auth_required":true}
```

The endpoint returns only this boolean, is cache-disabled, and does not expose
environment, host, tenant, key source, key count, or deployment labels. Other
methods return the normal mux behavior. It exists only to choose the browser's
locked versus immediate-local startup path.

## Admin Browser Flow

### HTML

Add two sibling regions:

- `#admin-unlock`: heading, password input, submit button, and live status;
- `#admin-app`: the existing console content, hidden until initialization.

The initial HTML hides both while capability discovery is pending to avoid a
flash of controls. Static console labels are not sensitive state. No dynamic
admin record or configuration value is embedded in the document.

### JavaScript State

One module-level state object owns:

```text
adminCredential: string | null
adminAuthRequired: boolean | null
adminInitialized: boolean
```

Create `adminFetch(input, init)` as the sole transport for every `/admin/*`
request. It clones headers, attaches `Authorization: Bearer <credential>` only
when a credential exists, calls `fetch`, and invokes `lockAdmin()` on any `401`.
It never accepts a credential argument from callers.

Refactor every direct admin `fetch` call and `postJSON` to use `adminFetch`.
The implementation must verify this with a source-level test or a narrowly
exported test seam so new direct calls do not silently bypass auth handling.

### Startup State Machine

```text
page load
  -> GET /admin-access
     -> failure: show retryable capability error; no admin calls
     -> auth_required=false: reveal app; initialize once
     -> auth_required=true: reveal unlock form

unlock submit
  -> keep key in module memory
  -> GET /admin/persona/active through adminFetch as validation/bootstrap
     -> 2xx: reveal app; initialize remaining loaders once
     -> 401: clear key; keep locked; focus input
     -> other failure: clear key; show retryable non-auth error; keep locked

later admin request returns 401
  -> clear key
  -> hide app
  -> show locked message
  -> do not automatically retry the failed mutation
```

Use the password input type and disable duplicate submissions. Pressing Enter
submits. Status copy names what happened and how to recover without echoing the
credential. Reload naturally resets module memory and requires re-entry.

No code may reference `localStorage`, `sessionStorage`, `indexedDB`, cookies,
history mutation, query parameters, or URL fragments for credential handling.

## UI And Accessibility Review

### Design Scores

| Dimension | Before | Planned | Decision |
| --- | ---: | ---: | --- |
| Information architecture | 7 | 9 | Unlock is a single gate before the unchanged console |
| Interaction states | 4 | 9 | Loading, locked, validating, invalid, network error, unlocked, and relocked are explicit |
| User journey | 5 | 9 | Local path remains automatic; deployed path explains the one required action |
| Visual consistency | 8 | 9 | Reuse existing surface, form, button, and status styles |
| Accessibility | 6 | 9 | Labelled password input, form submit, focus recovery, live status, hidden regions |
| Responsive behavior | 8 | 9 | Unlock card uses the existing bounded responsive surface |
| AI-slop risk | 8 | 9 | No new dashboard, illustration, gradient, or decorative security theater |

The unlock experience is intentionally plain and trustworthy. Do not add lock
icons, fake encryption claims, password-strength UI, or a full-page marketing
panel. Mobile and desktop use the same one-field flow.

## Data Flow

```text
Admin key source
  -> config parsing
  -> validation/redaction
  -> effective admin key (memory only)
  -> server.Handler admin policy
  -> constant-time header comparison
  -> existing tenant resolution
  -> existing admin handler/service/store

Browser-entered key
  -> password input
  -> JS module variable
  -> Authorization request header
  -> discarded on 401/reload/tab close
```

No new persistent business record is introduced.

## Error And Rescue Registry

| Failure | User-visible result | Side effects | Recovery |
| --- | --- | --- | --- |
| Missing admin key in non-local config | Startup error naming field and env override | Listener never opens | Configure secret and restart |
| Invalid/malformed request key | Stable `401 unauthorized` | No handler or rate-limit mutation | Supply correct key |
| Capability request fails | Locked shell with retry instruction | No admin request starts | Retry after server/network recovery |
| Unlock key invalid | Inline invalid-credential status, input focused | Key cleared; no app initialization | Re-enter key |
| Unlock validation returns 5xx/network error | Generic connection error | Key cleared; no mutation retry | Retry manually |
| Later GET returns 401 | App locks and key clears | Failed GET not retried | Re-authenticate |
| Later mutation returns 401 | App locks and key clears | Mutation never automatically retried | Re-authenticate and deliberately repeat |
| Dedicated and legacy keys both set | Dedicated key exclusively controls admin | Legacy key still controls runtime routes | Use dedicated key for admin |
| Rate limit exceeded after auth | Existing `429 rate_limited` | Existing counter increments | Wait/restart per current policy |

## Failure Modes Registry

| Failure mode | Severity | Prevention/test |
| --- | --- | --- |
| New admin route bypasses auth | Critical | Prefix classification tests with unknown future path |
| Exact `/admin` embeds operational data | High | Static response test and browser inspection |
| UI sends one direct unauthenticated fetch | High | Central transport refactor plus source guard/test seam |
| Credential persists in browser | High | Browser tests spy on storage/cookies/history and reload |
| Legacy and dedicated keys both authorize admin | High | Dedicated-precedence integration test |
| Malformed Authorization falls back to alternate header | Medium | Conflicting-header table tests |
| Auth failure reaches handler or rate limiter | High | sentinel handler and counter tests |
| Capability response leaks environment/tenant | Medium | exact JSON response assertion |
| Error/log contains credential | Critical | config, startup, response, and log redaction tests |
| Existing chat auth changes | High | unchanged runtime auth regression suite |

## Observability

Do not log credentials or credential-derived principals. Existing request IDs and
status codes are sufficient for Phase 27. A future phase may add bounded auth
failure counters, but adding high-cardinality labels or key fingerprints is out
of scope.

The readiness safe summary includes only `server.admin_api_key=<redacted>` when
configured. `/admin-access` is cache-disabled with `Cache-Control: no-store`.

## Performance

Configured key sets contain at most the dedicated key or one compatibility key,
so constant-time linear comparison is negligible. Capability discovery adds one
small request only on admin page load. Admin initialization remains the dominant
request fan-out and must execute once, not once per unlock loader.

## Rollout And Compatibility

1. Add configuration parsing and validation before server middleware changes.
2. Preserve default local config (`local`, `127.0.0.1`, no keys).
3. Preserve legacy secured deployments through `server.api_key` fallback.
4. Document `admin_api_key` and the browser unlock flow in README.
5. Do not remove fallback or emit a deprecation deadline in Phase 27.
6. CI remains provider-free and deterministic.

Rollback is additive: reverting Phase 27 restores the old optional shared-key
behavior; no data migration is required. Security review must explicitly verify
that a rollback would reintroduce the accepted high-severity exposure.

## Developer Experience Review

### Developer Journey

| Stage | Local developer | Deployed operator | Planned friction |
| --- | --- | --- | --- |
| Discover | README quick start | Startup error names new requirement | Low |
| Configure | No change | Set one env var | Low |
| Start | Existing command | Existing command after secret injection | Low |
| Open admin | Automatic local boot | Unlock prompt | Low |
| Authenticate | None | Enter key once per page lifetime | Medium, intentional |
| Operate | Existing console | Existing console | None |
| Error | Existing statuses | Actionable lock/startup message | Low |
| Reload | Existing automatic boot | Re-enter key | Medium, security tradeoff |
| Upgrade | No change | Legacy key fallback | Low |

### Empathy Narrative

As a local developer, I still want `go run ./cmd/server` and `/admin` to work
without setup. As a deployed operator, I need startup to tell me exactly which
secret is missing rather than exposing a half-working console. When my key is
wrong, I need the page to stay locked without erasing admin data or retrying a
mutation. I accept re-entering the key after reload because the alternative is
leaving an administrator bearer secret in durable browser storage.

### DX Scorecard

| Dimension | Score | Rationale |
| --- | ---: | --- |
| Getting started | 9/10 | Default local flow is unchanged |
| Configuration naming | 9/10 | `server.admin_api_key` mirrors existing naming |
| Error quality | 9/10 | Problem, cause, and fix are specified |
| Documentation | 8/10 | README/config examples required before ship |
| Upgrade safety | 9/10 | Legacy fallback prevents immediate breakage |
| Developer tooling | 8/10 | Existing Go/JS checks cover the change |
| Escape hatches | 8/10 | Explicit local-loopback mode; no unsafe override elsewhere |
| Feedback loops | 8/10 | Stable status codes and deterministic tests; no new metrics |

TTHW target remains under five minutes locally. A deployed operator should move
from startup failure to authenticated admin in under three configuration steps:
set secret, restart, enter secret.

## TDD Build Sequence

All tasks follow RED -> GREEN -> REFACTOR. Do not combine red tests for later
tasks with production changes for an earlier task.

### P27-01: Configuration Contract And Policy

Files:

- `internal/config/config.go`
- `internal/config/config_test.go`
- `configs/app.yaml`

RED:

- YAML and both environment names load `admin_api_key`.
- Full environment/host/effective-key matrix fails or passes as specified.
- Dedicated key takes precedence over legacy key.
- unknown environment fails closed.
- safe summary and arbitrary-text redaction never expose the key.

GREEN:

- add config field, parsing, env overrides, redaction, policy helpers, and
  actionable validation.

REFACTOR:

- centralize normalized environment/loopback/effective-key logic without
  changing provider validation.

### P27-02: Server Authorization Policies

Files:

- `internal/server/server.go`
- `internal/server/server_test.go`

RED:

- admin route boundary table;
- exact public shell/capability behavior;
- missing, malformed, conflicting, valid, and invalid headers;
- dedicated admin key isolation from runtime key;
- constant-time matcher helper contract;
- auth-before-rate-limit and auth-before-handler sentinels;
- anonymous local admin versus required admin mode.

GREEN:

- add explicit handler config, route classifiers, shared header parser,
  constant-time matcher, middleware branches, and `/admin-access`.

REFACTOR:

- keep runtime behavior stable and remove admin paths from the old enumerated
  protected-route expression.

### P27-03: Server Wiring And Regression

Files:

- `cmd/server/main.go`
- `cmd/server/main_test.go`

RED:

- build handler receives dedicated key, fallback key, and anonymous-local policy;
- dedicated key does not become a chat/experience key;
- startup summary redacts both key classes.

GREEN:

- wire config policy into `server.Config`.

REFACTOR:

- isolate auth-policy construction from large `buildHandler` setup.

### P27-04: Locked Admin Browser State

Files:

- `web/admin.html`
- `web/admin.js`
- `web/app.css`
- `web/web_test.go`

RED:

- capability loading prevents eager admin calls;
- anonymous capability initializes once;
- required mode shows labelled unlock form;
- valid key adds Bearer header to GET and POST calls;
- invalid/later `401` clears key and relocks;
- mutation `401` is not retried;
- reload has no retained credential;
- no storage, cookie, history, query, or fragment credential path exists;
- every `/admin/*` fetch routes through `adminFetch`.

GREEN:

- add locked/app regions, state machine, central transport, startup gate, and
  accessible statuses.

REFACTOR:

- group all module-load calls under one idempotent `initializeAdmin()` and remove
  duplicate audit/timeline startup calls already visible at the file tail.

### P27-05: Documentation And Full Regression

Files:

- `README.md`
- `RELEASE_NOTES.md` only during ship/release preparation

RED/evidence:

- documentation checklist proves local and deployed examples are complete;
- security regression exercise starts production-like config without a key and
  expects failure, then starts with a key and checks shell, capability, `401`,
  valid admin access, and unchanged chat behavior.

GREEN:

- document configuration, migration fallback, unlock lifetime, TLS requirement,
  and troubleshooting.

REFACTOR:

- update stale README phase status and endpoint list without rewriting unrelated
  historical sections.

## Explicit Test Matrix

| ID | Level | Scenario | Expected evidence |
| --- | --- | --- | --- |
| C01 | Unit | local + IPv4 loopback + no keys | config valid, anonymous admin true |
| C02 | Unit | local + IPv6 loopback/localhost + no keys | config valid |
| C03 | Unit | local + non-loopback + no keys | actionable startup error |
| C04 | Unit | staging/production aliases + any host + no keys | actionable startup error |
| C05 | Unit | unknown environment + loopback + no keys | fail closed |
| C06 | Unit | dedicated key configured | effective admin key is dedicated |
| C07 | Unit | only legacy key configured | effective admin key is legacy fallback |
| C08 | Unit | both keys configured | legacy key cannot authorize admin |
| C09 | Unit | safe summary/redaction | no admin secret appears |
| S01 | Unit | `/admin` exact path | public static shell only |
| S02 | Unit | `/admin/` and unknown `/admin/future` | admin auth required |
| S03 | Unit | `/administrator`, `/administer` | not classified as admin |
| S04 | Unit | no/malformed/empty credential | identical `401` body |
| S05 | Unit | Bearer and X-API-Key separately valid | handler executes |
| S06 | Unit | malformed Authorization plus valid X-API-Key | `401`, no fallback |
| S07 | Unit | valid Authorization plus conflicting X-API-Key | Authorization wins |
| S08 | Unit | invalid auth with rate limiting enabled | counter unchanged |
| S09 | Unit | invalid auth against sentinel handler | handler untouched |
| S10 | Unit | local anonymous mode | admin handler executes with fixed internal principal |
| S11 | Unit | `/admin-access` | exact boolean-only no-store response |
| S12 | Regression | chat/experience with empty/configured legacy key | existing behavior unchanged |
| W01 | Frontend | capability says anonymous | app reveals and initializes once |
| W02 | Frontend | capability says required | only unlock state active, no admin fetch |
| W03 | Frontend | valid unlock | Bearer attached; app initializes once |
| W04 | Frontend | invalid unlock | key cleared, input focused, app hidden |
| W05 | Frontend | later GET `401` | relock, no automatic retry |
| W06 | Frontend | mutation `401` | relock, mutation not repeated |
| W07 | Frontend | network/5xx during validation | recoverable error, app remains locked |
| W08 | Frontend | reload/new module instance | no key retained |
| W09 | Static | storage/cookie/history/URL APIs | no credential persistence path |
| W10 | Static | direct `/admin/*` fetch inventory | all use `adminFetch` |
| I01 | Integration | production-like process without key | exits before listener |
| I02 | Integration | production-like process with dedicated key | capability true, API `401` then `200` |
| I03 | Integration | local default process | capability false, admin API available |
| I04 | Integration | dedicated admin key and legacy runtime key | each key authorizes only its route class |
| I05 | Integration | tenant query manipulation after auth | existing tenant boundary still wins |

## Verification Commands

Run targeted commands after each TDD slice, then the full gate:

```powershell
go test ./internal/config
go test ./internal/server
go test ./cmd/server
go test ./web
go test ./...
go vet ./...
node --check web/admin.js
git diff --check
```

If the workstation still uses `windows/386`, record that `go test -race` is not
available and rely on CI's supported platform rather than claiming race coverage.
CI `golangci-lint` remains mandatory before merge.

## QA Plan

Run the server in two modes and use real browser QA:

1. local loopback with no key: `/admin` opens directly and every existing panel
   can load;
2. production-like loopback with a dedicated key: page starts locked, wrong key
   stays locked, correct key unlocks, one GET and one safe mutation carry auth,
   reload relocks, and no key appears in URL/storage/console/network response;
3. expire/change the configured key between requests where practical, then verify
   the next `401` relocks without retrying a mutation;
4. verify mobile-width form layout, keyboard submit, focus, and live status;
5. verify `/app`, health, readiness, and runtime status remain unaffected.

No staging claim may be made without a staging URL.

## Security Review Checklist

- Attempt every `/admin/*` family without credentials.
- Attempt route confusion with encoded paths, trailing slashes, duplicate slashes,
  prefix lookalikes, and unsupported methods.
- Search source, logs, HTML, responses, audit files, and browser storage for the
  configured test credential.
- Verify dedicated-key precedence and runtime/admin key separation.
- Verify startup fail-closed behavior through direct config and environment input.
- Verify no cookie means no new CSRF trust path.
- Re-check tenant isolation after successful authentication.

Any high-severity finding stops shipping for explicit approval under `AGENTS.md`.

## Implementation Tasks

- [ ] **P27-01 (P1) - Configuration and environment policy**
- [ ] **P27-02 (P1) - Separate admin/runtime middleware and constant-time auth**
- [ ] **P27-03 (P1) - Server wiring and compatibility regression**
- [ ] **P27-04 (P1) - In-memory browser unlock and centralized admin fetch**
- [ ] **P27-05 (P2) - Documentation, browser QA, and full verification**

Suggested dependency order: `P27-01 -> P27-02 -> P27-03 -> P27-04 -> P27-05`.
The tasks are intentionally sequential because each later slice consumes the
policy contract of the prior slice; parallel work would create avoidable merge and
test-fixture conflicts in `server.Config` and the admin startup flow.

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale | Rejected |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | CEO | Hold scope at one operator credential | Auto-decided | Solve verified pain first | Removes known high risk without speculative identity platform | RBAC expansion |
| 2 | CEO | Preserve exact local-loopback exception | Approved direction | Preserve working defaults | Local quick start remains under five minutes | Universal key |
| 3 | Eng | Separate admin and runtime key sets | Auto-decided | Correctness over reuse | Dedicated admin key must not silently authorize chat | One combined set |
| 4 | Eng | Dedicated key overrides rather than joins legacy key | Auto-decided | Secure migration | Prevents two simultaneous admin credentials after migration | Union of keys |
| 5 | Eng | Unknown environment fails closed | Auto-decided | Safe default | Typos cannot activate local anonymous mode | Production-like allowlist only |
| 6 | Eng | Constant-time linear key comparison | Auto-decided | Security and simplicity | Key set is tiny; no map timing shortcut needed | Plain map lookup |
| 7 | Design | Static console markup may be public but contains no data | Implementation clarification | Simpler over clever | Avoids protected fragment injection while preserving data boundary | HTML fragment bootstrap |
| 8 | Design | Page-memory credential only | Approved direction | Minimize secret lifetime | Avoids durable browser exposure and cookie/CSRF complexity | Storage/cookie |
| 9 | Design | Never retry a failed mutation after relock | Auto-decided | Prevent hidden side effects | Operator must deliberately repeat after re-auth | Automatic retry |
| 10 | DX | Keep legacy key fallback for this phase | Auto-decided | Credible upgrade path | Existing secured deployments do not fail immediately | Hard cutover |
| 11 | DX | Actionable startup error names field and env var | Auto-decided | Fight uncertainty | Operator can recover without inspecting code | Generic invalid config |
| 12 | Eng | No new auth metrics in Phase 27 | Auto-decided | Avoid scope and cardinality risk | Status codes and request IDs are adequate for first boundary | Credential fingerprints |

## Review Scores

- CEO: 9/10. The phase solves a verified release blocker with disciplined scope.
- Design: 9/10 planned. The lock state is explicit, accessible, and visually
  consistent without adding security theater.
- Engineering: 9/10 planned. Policy ownership, middleware order, failure boundaries,
  and regression seams are explicit.
- DX: 8.6/10 planned. Local flow remains unchanged and deployed migration is
  actionable; reload re-entry is the intentional security cost.

Independent Claude/subagent voices were unavailable in this Codex environment. A
fresh-context `codex exec` review was attempted but timed out after 124 seconds
without findings output, so it is not counted as an independent voice. The primary
Codex review examined product, design, engineering, DX, and security perspectives
against the accepted spec and current source. This limitation is recorded rather
than represented as dual-model consensus.

## Cross-Phase Themes

1. **Fail closed without breaking local-first use** appeared in product,
   engineering, design, and DX review. It is the central acceptance boundary.
2. **One credential path and one request helper** appeared in security,
   engineering, and frontend review. Duplication is the highest bypass risk.
3. **Migration without broadening privilege** appeared in product, engineering,
   and DX review. Dedicated-key precedence is therefore mandatory.

## Completion Summary

The plan is implementation-ready and contains no unresolved product decision.
Stage 3 can proceed in five sequential TDD slices after approval. The highest-risk
areas are route classification, frontend request inventory, credential leakage,
and compatibility behavior; each has explicit unit, integration, browser, and
security evidence requirements.

## Stage 2 Gate

Approve this plan to enter Stage 3. Implementation must use Superpowers TDD in
the listed order and must stop for any scope change that contradicts the approved
Phase 27 spec.
