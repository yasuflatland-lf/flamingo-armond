# Backend authentication

> JWT verification (Supabase + Google OAuth), JWKS lifecycle, Echo middleware layering, role-based authorization. See `docs/backend.md` for runtime, `docs/backend-graphql.md` for resolver patterns, and `.claude/rules/error-wrapping.md` for the eris convention.

## Authentication

The backend uses an opt-in JWT verification model. Routes are divided into two groups:

```
GET  /            open (no auth)
GET  /health      open (no auth)
GET  /playground  open (no auth)
POST /query       AuthMiddleware → gqlgen handler
```

When an `Authorization` header is **absent**, the request passes through as anonymous — no `auth.AuthUser` is attached to the context. Resolvers themselves enforce identity per request via the usecase layer. When the header is **present and valid**, `auth.UserFrom(ctx)` returns the verified `*auth.AuthUser` (`Sub`, `Email`, `Role`). When the header is **present but invalid**, the middleware short-circuits with HTTP 401 and sets `WWW-Authenticate: Bearer realm="api"`.

### Middleware layering

`e.Group("/query", authMW)` registers `AuthMiddleware` only on the `/query` route group. Playground, health, and the root handler live outside the group and are never touched by auth logic.

### Resolver usage

```go
func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
    u := auth.UserFrom(ctx)
    if u == nil {
        // anonymous — return guest data or error depending on the resolver's policy
        return nil, nil
    }
    // u.Sub, u.Email, u.Role are available here
    _ = u.Sub
    return nil, nil
}
```

### 401 condition matrix

| Condition | Result |
|---|---|
| `Authorization` header absent | Anonymous passthrough (no 401) |
| Bearer token present, valid | 200 — `AuthUser` in context |
| Expired JWT (`exp` in past, outside 30 s leeway) | 401 |
| Tampered signature | 401 |
| Wrong `kid` (not in JWKS) | 401 |
| Missing or empty Bearer scheme | 401 |
| `alg=HS256` (confusion attack) | 401 |
| `alg=none` | 401 |
| Missing `exp` claim | 401 |
| Wrong `aud` | 401 |
| Wrong `iss` | 401 |

Accepted algorithms: ES256, RS256. A 30-second leeway is applied to `exp` to tolerate minor clock skew between services.

### WWW-Authenticate policy

All 401 responses carry `WWW-Authenticate: Bearer realm="api"`. The header deliberately omits `error` and `error_description` parameters (RFC 6750 §3.1) to minimize information disclosure — callers learn only that a valid Bearer token is required, not why verification failed.

### JWKS lifecycle

`NewJWKSKeyfunc` wraps `MicahParks/keyfunc/v3` with `NoErrorReturnFirstHTTPReq: false`. This means:

- **Initial fetch is required** — if the JWKS endpoint is unreachable at startup, `ConfigFromEnv` / server startup fails immediately rather than silently caching nothing.
- **Periodic refresh failures are non-fatal** — the keyfunc library keeps the last successfully fetched key set in memory and continues verifying tokens. A transient JWKS outage does not bring down the server.

`NoErrorReturnFirstHTTPReq` defaults to **`true`** in `keyfunc/v3`, meaning the plain `keyfunc.NewDefaultCtx(ctx, urls)` constructor silently discards initial-fetch errors and returns a keyfunc that will verify nothing. To enforce fail-fast boot you must use `keyfunc.NewDefaultOverrideCtx` and pass an `Override{NoErrorReturnFirstHTTPReq: new(bool)}` (a pointer to `false`). The field name's polarity is the opposite of what "No error = good" suggests — treat it as "suppress-error flag" and always override it to `false`.

### Environment variables (auth)

See also the general env-vars table above.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `SUPABASE_JWKS_URL` | yes | — | JWKS endpoint URL (e.g. `https://<project>.supabase.co/auth/v1/.well-known/jwks.json`) |
| `SUPABASE_JWT_AUDIENCE` | yes | — | Expected `aud` claim value |
| `SUPABASE_JWT_ISSUER` | yes | — | Expected `iss` claim value |
| `SUPER_USER_EMAILS` | no | *(empty)* | Comma-separated list of trusted email addresses (Supabase / Google OAuth). On the first authenticated request from a matching account whose JWT carries `email_verified=true`, the backend grants the `admin` role. Empty disables the feature. |

The three `SUPABASE_JWT_*` variables are required. `ConfigFromEnv()` returns an error and the server fails to start if any is missing or empty — silent misconfiguration is not allowed. `SUPER_USER_EMAILS` is optional; when empty the bootstrap-admin middleware is constructed as a zero-cost pass-through.

### Authorization gates: object-level vs. field-level

A `@hasRole(ADMIN)`-style gate on a top-level query (e.g. `Query.users`) does **not** protect fields on the returned type that any other resolver might also expose. `User.roles` is reachable from `me`, `cardgroup.owner`, and any future resolver that returns a `User` — the admin-only gate on `Query.users` covers exactly one of those entry points.

Field-level resolvers that expose privileged data must perform their own admin-or-self check inside the field resolver itself:

```go
func (r *userResolver) Roles(ctx context.Context, obj *model.User) ([]*model.Role, error) {
    caller := auth.UserFrom(ctx)
    if caller == nil {
        return nil, gqlerr.Unauthenticated()
    }
    if caller.Sub != obj.ID {
        isAdmin, err := r.AuthSvc.IsAdmin(ctx, caller.Sub)
        if err != nil { /* propagate context.Canceled, else gqlerr.Internal */ }
        if !isAdmin {
            return nil, gqlerr.NewForbidden("forbidden")
        }
    }
    // ...
}
```

The "self or admin" check is the right granularity for fields where the owning user has a legitimate read interest in their own data; pure admin-only fields drop the `caller.Sub != obj.ID` branch.

### Self-demotion guard

A user who is allowed to edit final role sets can remove their own admin role and lock the system out of admin operations. The usecase layer must reject "the caller is removing the admin role from themselves" before the DB write:

1. Compare `callerID == targetUserID`.
2. Resolve the submitted final `roleIds` to role names.
3. If the final set for the caller no longer contains the `"admin"` role, return the `CannotRevokeOwnAdminRoleError` union variant from `adminEditUser`.

This guard belongs in the usecase, not in the UI: the UI is one of N possible callers, and a CLI / API consumer / Apollo Studio request can hit the resolver directly. The role-name lookup is mandatory — comparing role IDs would couple the guard to seed data that varies between environments. The hardcoded `"admin"` matches `auth.Service.IsAdmin`'s same hardcoded literal; both move together when a second privileged role is introduced.

### Bootstrap admin via `SUPER_USER_EMAILS`

A fresh deployment has zero admin rows, but every existing admin-management mutation is gated on `AdminGate.Require` and the `user_roles` RLS policy requires `is_admin(auth.uid())` — a chicken-and-egg deadlock. The `auth.SuperUserPromoter` post-auth Echo middleware breaks the loop without weakening either gate: when the first authenticated request from an email listed in `SUPER_USER_EMAILS` arrives, the middleware grants that user the `admin` role and lets the request continue. Subsequent requests short-circuit on the `IsAdmin == true` branch, so steady-state cost is one cached role lookup. See `backend/internal/auth/superuser.go`.

**`email_verified=true` is a mandatory security gate, not a heuristic.** Supabase only sets the claim once the OAuth provider has confirmed the user controls the address. Promoting on email-match alone would let any account that *claims* an env-listed address (e.g. via a misconfigured identity provider) inherit admin. The middleware reads the claim from `AuthUser.EmailVerified` (threaded through `supabaseClaims.EmailVerified` and `auth/middleware.go`'s `AuthUser` constructor) and returns `next(c)` without any DB call when the claim is missing or false. Because `encoding/json` leaves an absent boolean at zero (`false`), the absence-equals-deny posture is automatic — the JSON `omitempty` tag on `EmailVerified` only affects marshal output and never the decode path.

**No automatic revocation.** Removing an email from `SUPER_USER_EMAILS` does not strip the role; an admin must update the user's final role set through `adminEditUser`. This is deliberate: a typo in the env var should not silently lock the service out of every admin operation on the next deploy.

**Failure mode: WARN + continue, never 5xx.** Both `IsAdmin` and `AssignToUser` failures are logged via `logging.LogWarn` (carrying the eris `error_chain`) and the middleware falls through to `next(c)`. The promotion is best-effort — a transient DB blip during a routine page load should not surface as a user-facing error. Any downstream resolver that actually requires admin remains protected by `AdminGate.Require`, which is fail-closed.

**Concurrent first-login is safe.** `repository.RoleRepository.AssignToUser` uses `INSERT ... ON CONFLICT DO NOTHING`, so two simultaneous requests from the same user that both read `IsAdmin == false` produce two harmless inserts — both return `nil`, both proceed.

**Constructor invariants are enforced via panic.** When `emails` is non-empty but any of `checker`, `assigner`, or `adminRoleID` is nil/empty, `NewSuperUserPromoter` panics during `run()`. This is a fail-fast for operator misconfiguration: the alternative — returning an error or silently building a half-configured promoter — would either bury the misconfiguration in a startup log or leave a per-request nil-deref hazard. Empty-emails callers (the OFF path) intentionally pass `nil, "", nil, nil`; the panic guard only fires when the operator opted into the feature but wired it wrong.

### Confirming the bootstrap is armed

The backend logs a single INFO line on successful startup when `SUPER_USER_EMAILS` is non-empty:

```
{"level":"INFO","msg":"super-user bootstrap enabled","email_count":N}
```

`email_count` is the number of normalised, deduplicated entries the parser accepted from the env var. If `email_count` differs from what you put in the env (or is `0` when you expected a non-zero value), the parser dropped malformed entries silently — recheck for stray quotes or empty comma-separated fields.

When `SUPER_USER_EMAILS` is empty AND no row in `public.user_roles` references the `admin` role, the backend additionally logs:

```
{"level":"WARN","msg":"super-user bootstrap: no admin configured and no admin role-holder exists","admin_count":0}
```

This is the deliberate "you have no escape hatch" warning — the next signed-in user has no path to admin without operator intervention. Set `SUPER_USER_EMAILS` and restart, or run the SQL fallback below. The check tolerates DB unavailability: a failed count query logs `eris`-wrapped WARN context but does not block startup.

### Manual SQL fallback (post-`make db-reset`)

`make db-reset` re-runs all migrations and resets `public.user_roles` to empty, so any previously bootstrapped admin loses the role until the user makes their next authenticated request with a listed address (a page reload suffices — no new OAuth login is required). To re-promote without waiting for the next request:

```bash
psql "$(supabase status -o env | grep '^DB_URL=' | cut -d= -f2- | tr -d '"')" <<'SQL'
INSERT INTO public.user_roles (user_id, role_id)
SELECT u.id, r.id
  FROM auth.users u, public.roles r
  WHERE u.email = 'you@example.com' AND r.name = 'admin'
ON CONFLICT DO NOTHING;
SQL
```

Or use the Make target wrapper:

```
make seed-admin EMAIL=you@example.com
```

The Make target executes the same INSERT through the local Supabase Postgres container (no `Authorization` header round-trip required).

### Multi-layer security test coverage

Authorization rules implemented at the usecase level need tests at **both** the usecase layer and the resolver layer. Usecase tests confirm the rule (right sentinel returned, right error code mapped) but cannot catch wire-format regressions: an `extensions.code` typo, a resolver that swallows the usecase error and returns `nil`, or a gqlgen codec change that drops the `field` extension. Resolver-level wire tests built against `handler.NewServer` (see `docs/backend.md` § "Resolver-level wire tests") are the only layer that exercises the full request envelope. Apply this dual-layer rule to every guard whose failure mode is "user gains access they should not have" — privilege checks, owner checks, self-demotion, and role-mutation paths.

### Echo v5 + gqlgen error propagation

`echo.WrapHandler` (v5) converts a `http.Handler` into an `echo.HandlerFunc` that always returns `nil`. gqlgen's `handler.Server` is an `http.Handler`: it writes GraphQL errors into the response body as `{"errors":[...]}` with HTTP 200, and only ever writes a 5xx for catastrophic transport failures. Because `WrapHandler` returns `nil`, Echo's central error pipeline never sees these, which is fine: the GraphQL error is already transported in-band. Do **not** wrap gqlgen with a custom adapter that translates non-2xx into `echo.NewHTTPError` — that would cause a double write on the already-committed `ResponseWriter`.

### Custom access token hook

The Supabase Custom Access Token Hook (`public.custom_access_token_hook`) joins `public.user_roles` at JWT mint time and emits `app_metadata.role = "admin"` into the access token. The frontend root layout reads this via `supabase.auth.getClaims()` and forwards `isAdmin` to `AppShell` without a GraphQL round-trip. See [`docs/backend/custom-access-token-hook.md`](backend/custom-access-token-hook.md) for the design decisions (join-at-mint vs sync-trigger, fail-closed on malformed events, stale-claim removal, canonical return shape, DROP-auto-revoke, operator precondition for the down migration). The consumer side is documented in [`docs/frontend/auth-supabase.md`](frontend/auth-supabase.md).
