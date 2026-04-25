# Backend runtime notes

Design and constraints for the Go / Echo v5 backend that are not obvious from the code alone.

## Module layout

- Go version is pinned via mise (`backend/.tool-versions`, currently `golang 1.26.2`). CI resolves Go through `jdx/mise-action` with `working_directory: backend`.
- Module name is the bare `backend` (see `backend/go.mod`). All internal imports start with `backend/...`.
- All `go` commands **must run from `backend/`** (CI sets `defaults.run.working-directory: backend`; match that locally).

## Runtime shape

The server entry (`backend/cmd/server/main.go`) is deliberately split thin so tests can drive the full lifecycle without spawning a subprocess:

- `newRouter()` — builds `*echo.Echo` with `RequestLogger` + `Recover` middleware and exposes `GET /` and `GET /health`. Tests hit it directly via `httptest.NewServer`.
- `run(ctx, logger) error` — owns the `http.Server` and the shutdown goroutine. Reads `PORT` and `SHUTDOWN_TIMEOUT` here.
- `main()` — wires `slog.NewJSONHandler(os.Stderr, ...)` and `signal.NotifyContext(SIGTERM, SIGINT)`, then calls `run`. Exits 1 on error.

When adding features, **keep `run(ctx, logger) error` as the seam**. `TestRunGracefulShutdown` cancels its own context to assert a clean return; regressions there mean the shutdown plumbing is broken.

## Echo v5 requires a user-managed `http.Server`

Echo v5 **intentionally removed** `e.Shutdown`, `e.Close`, `e.StartServer` (v4 had them) — the design philosophy is that the framework should not hide the standard library. Anything touching server lifecycle (start, stop, timeouts) has to be done on a `http.Server{Handler: e}` built by the caller.

## Graceful shutdown contract

Production (Render) sends **SIGTERM and then SIGKILL after 30 seconds**. Therefore:

- `SHUTDOWN_TIMEOUT` defaults are kept **under the 30 s budget** (currently 25 s), leaving headroom for future in-flight cleanup (DB pools, queue flushes).
- Both SIGTERM and SIGINT are handled (prod = SIGTERM, local Ctrl+C = SIGINT).
- Shutdown is expressed as a `context.Context` via `signal.NotifyContext`; `errgroup.WithContext` unifies error propagation between the serve goroutine and the shutdown goroutine.

If `srv.Shutdown(ctx)` returns `context.DeadlineExceeded`, in-flight requests did not finish inside the timeout. This is treated as a **warning + normal exit (`return nil`)**, not an error — we don't want to trigger Render's crash-restart loop over graceful-shutdown timeouts.

## HTTP server security timeouts

Running `http.Server` with no timeouts is a **Slowloris DoS surface** (gosec G112). The service sets:

- `ReadHeaderTimeout` — header-receive bound. Most important for Slowloris.
- `ReadTimeout` / `WriteTimeout` — whole-request / whole-response bounds.
- `IdleTimeout` — keep-alive dwell bound (Render's load balancer reuses keep-alive).

These values target a public API on Render. Revisit if the threat model or deployment target changes.

## Environment variables

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `PORT` | no | `1323` | Listen port |
| `SHUTDOWN_TIMEOUT` | no | `25s` | Go duration for graceful shutdown. Invalid or `<= 0` values log a warning and fall back to the default. |
| `SUPABASE_JWKS_URL` | yes | — | JWKS endpoint for JWT verification |
| `SUPABASE_JWT_AUDIENCE` | yes | — | Expected `aud` claim in incoming JWTs |
| `SUPABASE_JWT_ISSUER` | yes | — | Expected `iss` claim in incoming JWTs |

`PORT` and `SHUTDOWN_TIMEOUT` are optional with safe defaults. The three `SUPABASE_*` variables are all required — the server refuses to start if any is missing (fail-fast via `ConfigFromEnv`).

## Testing patterns

- **Handler unit tests use `t.Parallel()`** (see `TestHealthEndpoint`, `TestRootEndpoint`).
- **Lifecycle tests are sequential** (no `t.Parallel()`). They affect process-wide state.
- **Free port discovery**: `net.Listen("tcp", "127.0.0.1:0")` → read `Addr()` → `Close()` → start the server on that port (`freePort` + `waitHealthy` polling). Never hardcode ports.
- **Do not send real signals to the test process** (e.g. `syscall.Kill(os.Getpid(), SIGTERM)`). Signals are delivered process-wide and race with `t.Parallel()` tests and the test runner itself. Reproduce the meaning of `signal.NotifyContext` by **cancelling a `context.WithCancel` directly**.
- **Silence logs in tests** with `slog.New(slog.DiscardHandler)`.
- **`t.Parallel()` is incompatible with `t.Setenv()`** — `t.Setenv` mutates process-global env state and the Go test framework will panic if a parallel test calls it. Tests that manipulate env vars must be sequential (no `t.Parallel()`).

## Logging

`log/slog` with a `JSONHandler` on `os.Stderr`. `slog.SetDefault` registers the process-wide default, and `e.Logger = logger` shares the same logger with Echo so request logs and application logs use a single format.

## GraphQL endpoint

`POST /query` is served by gqlgen. The schema lives under `schema/*.graphql` at the repo root and is consumed by `backend/gqlgen.yml` via a relative glob (`../schema/*.graphql`), so both backend (gqlgen) and frontend (graphql-codegen) see the same source of truth.

### Regeneration

The gqlgen CLI is pinned through the `tool` directive in `backend/go.mod` (Go 1.24+). Regenerate from the `backend/` directory:

```bash
go tool gqlgen generate
```

CI regenerates these artifacts before every build — they are intentionally **git-ignored**. Only hand-written resolver implementations under `backend/graph/resolver/*.resolvers.go` are committed. CI still fails the build if `go tool gqlgen generate` produces a diff against committed resolver stubs, so editing `schema/*.graphql` obliges you to regenerate locally and commit any new stub that appears under `backend/graph/resolver/`.

### Resolver layout

- `backend/graph/resolver/resolver.go` — hand-written root `Resolver` struct. DI-only; gqlgen never rewrites this file.
- `backend/graph/resolver/*.resolvers.go` — generated per schema object but committed, because function bodies hold hand-written implementation. gqlgen appends new stubs on regenerate but **never rewrites existing function bodies**.
- `backend/graph/generated/generated.go` — gqlgen runtime. **Git-ignored**, regenerated by CI before build.
- `backend/graph/model/models_gen.go` — generated struct types. **Git-ignored**, regenerated by CI. Custom scalars / type overrides belong in `gqlgen.yml` under `models:`. Hand-written helpers in the same package go in `doc.go` (or sibling files).

### Why `graph/model/doc.go` exists

gqlgen deletes `graph/model/models_gen.go` at the start of every run before regenerating it. If `models_gen.go` is the only file in the package, the package becomes unparseable during that window, and the `autobind: - backend/graph/model` entry in `gqlgen.yml` fails to resolve. `graph/model/doc.go` is a three-line hand-written package declaration that keeps the package loadable across gqlgen runs. Keep it even after real model types land — deleting it will reintroduce the chicken-and-egg autobind failure on the next clean regeneration.

### Playground

`GET /playground` exposes a browser UI for hand-crafted queries against `/query`. It is currently enabled in all environments. Introspection and this route should be env-gated before production use.

### Resolver DI seam

`newRouter(resolvers *resolver.Resolver, authMW echo.MiddlewareFunc) *echo.Echo` is the DI wiring seam. `run(ctx, logger) error` is the lifecycle seam — it constructs the `Resolver`, passes it to `newRouter`, and owns the `http.Server`. Middleware-shaped dependencies (auth, future per-request observability) are passed as `echo.MiddlewareFunc` parameters to `newRouter`, while data-access dependencies (DB, dataloaders) add fields to `Resolver` and wire them in `run`.

## Authentication

The backend uses an opt-in JWT verification model. Routes are divided into two groups:

```
GET  /            open (no auth)
GET  /health      open (no auth)
GET  /playground  open (no auth)
POST /query       AuthMiddleware → gqlgen handler
```

When an `Authorization` header is **absent**, the request passes through as anonymous — no `auth.AuthUser` is attached to the context. This allows unauthenticated queries to proceed until individual resolvers start enforcing identity (PR9). When the header is **present and valid**, `auth.UserFrom(ctx)` returns the verified `*auth.AuthUser` (`Sub`, `Email`, `Role`). When the header is **present but invalid**, the middleware short-circuits with HTTP 401 and sets `WWW-Authenticate: Bearer realm="api"`.

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

### Echo v5 + gqlgen error propagation

`echo.WrapHandler` (v5) converts a `http.Handler` into an `echo.HandlerFunc` that always returns `nil`. gqlgen's `handler.Server` is an `http.Handler`: it writes GraphQL errors into the response body as `{"errors":[...]}` with HTTP 200, and only ever writes a 5xx for catastrophic transport failures. Because `WrapHandler` returns `nil`, Echo's central error pipeline never sees these, which is fine: the GraphQL error is already transported in-band. Do **not** wrap gqlgen with a custom adapter that translates non-2xx into `echo.NewHTTPError` — that would cause a double write on the already-committed `ResponseWriter`.

## Backend gotchas

### Echo v5 handler signature uses a pointer receiver

Echo v5 handler and middleware signatures changed from v4. Every handler and middleware factory must use `*echo.Context` (pointer), not the v4 interface form:

```go
// v5 — correct
func handler(c *echo.Context) error { ... }

// v4 — will not compile or will behave wrong in v5
func handler(c echo.Context) error { ... }
```

Online samples, AI-generated code, and the official Echo v4 docs all use the interface form. Any paste from those sources requires this fix.

### `echo.NewHTTPError` discards manually-set response headers

Echo's default `HTTPErrorHandler` serializes the error and writes a fresh response, discarding any headers set on the context before returning the error. Setting `c.Response().Header().Set("WWW-Authenticate", "...")` and then `return echo.NewHTTPError(401, "...")` will drop the header in the rendered response. The fix is to write the response directly and return `nil`:

```go
c.Response().Header().Set("WWW-Authenticate", `Bearer realm="api"`)
return c.String(http.StatusUnauthorized, "unauthorized")
```

Any future middleware that must send headers on an error response must use this pattern.

### Config struct validation belongs at the consumption boundary, not only the constructor

Validating fields only inside `ConfigFromEnv` is bypassable — callers can construct `Config{}` directly and pass empty strings to `jwt.WithAudience("")` / `jwt.WithIssuer("")`, which silently match any token claim. Add a `Validate() error` method to the config type and call it at the consumption point (e.g., the middleware factory). The factory should return `(echo.MiddlewareFunc, error)` so the DI wiring in `run()` catches misconfiguration at boot rather than at the first request. This pattern generalises to any config struct whose zero value is semantically dangerous.

### JWT algorithm confusion: always whitelist valid algorithms

Without `jwt.WithValidMethods([]string{"ES256", "RS256"})`, an attacker can re-sign a token with `HS256` using the JWKS public key as the HMAC secret, or use `alg=none` to bypass signature verification entirely. `golang-jwt/v5` does not reject these by default if the keyfunc returns a key. Always pass `WithValidMethods` with the exact set of algorithms your JWKS endpoint issues.
