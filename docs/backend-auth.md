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

All three are required. `ConfigFromEnv()` returns an error and the server fails to start if any is missing or empty — silent misconfiguration is not allowed.

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

A user who is allowed to assign and revoke roles can also revoke their own admin role and lock the system out of admin operations. The usecase layer must reject "the caller is removing the admin role from themselves" before the DB write:

1. Compare `callerID == targetUserID`.
2. Look up the role being revoked and check whether its name is `"admin"`.
3. If both, return `gqlerr.NewForbidden("cannot remove your own admin role")`.

This guard belongs in the usecase, not in the UI: the UI is one of N possible callers, and a CLI / API consumer / Apollo Studio request can hit the resolver directly. The role-name lookup is mandatory — comparing role IDs would couple the guard to seed data that varies between environments. The hardcoded `"admin"` matches `auth.Service.IsAdmin`'s same hardcoded literal; both move together when a second privileged role is introduced.

### Multi-layer security test coverage

Authorization rules implemented at the usecase level need tests at **both** the usecase layer and the resolver layer. Usecase tests confirm the rule (right sentinel returned, right error code mapped) but cannot catch wire-format regressions: an `extensions.code` typo, a resolver that swallows the usecase error and returns `nil`, or a gqlgen codec change that drops the `field` extension. Resolver-level wire tests built against `handler.NewServer` (see `docs/backend.md` § "Resolver-level wire tests") are the only layer that exercises the full request envelope. Apply this dual-layer rule to every guard whose failure mode is "user gains access they should not have" — privilege checks, owner checks, self-demotion, and role-mutation paths.

### Echo v5 + gqlgen error propagation

`echo.WrapHandler` (v5) converts a `http.Handler` into an `echo.HandlerFunc` that always returns `nil`. gqlgen's `handler.Server` is an `http.Handler`: it writes GraphQL errors into the response body as `{"errors":[...]}` with HTTP 200, and only ever writes a 5xx for catastrophic transport failures. Because `WrapHandler` returns `nil`, Echo's central error pipeline never sees these, which is fine: the GraphQL error is already transported in-band. Do **not** wrap gqlgen with a custom adapter that translates non-2xx into `echo.NewHTTPError` — that would cause a double write on the already-committed `ResponseWriter`.

