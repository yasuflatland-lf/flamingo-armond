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

**DB pool close ordering is sensitive.** The pool must be closed only after `srv.Shutdown` returns, never before. Closing the pool while in-flight requests still hold connections produces `connection closed` errors that leak into client responses. The correct sequence inside the shutdown goroutine is: call `srv.Shutdown(sctx)` → stash the return value → call `db.Close()` unconditionally → evaluate the stashed error. This guarantees the pool outlives all active HTTP handlers.

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
| `SWIPE_NEXT_BATCH_SIZE` | no | `10` | Number of due cards returned by `handleSwipe` after applying one rating. Invalid or `<= 0` values log a warning and fall back to the default. |
| `SUPABASE_JWKS_URL` | yes | — | JWKS endpoint for JWT verification |
| `SUPABASE_JWT_AUDIENCE` | yes | — | Expected `aud` claim in incoming JWTs |
| `SUPABASE_JWT_ISSUER` | yes | — | Expected `iss` claim in incoming JWTs |
| `SUPABASE_DB_URL` | yes | — | Supabase Postgres DSN (`postgres://...?sslmode=require`) |
| `DB_MAX_CONNS` | no | `10` | Maximum pool connections |
| `DB_MIN_CONNS` | no | `0` | Minimum pool connections kept alive |
| `DB_MAX_CONN_LIFETIME` | no | `30m` | Maximum lifetime of a pooled connection |
| `DB_MAX_CONN_IDLE_TIME` | no | `5m` | Maximum idle time before a connection is evicted |
| `PING_TOKEN` | yes | — | Bearer token for `POST /internal/ping`. Server refuses to start if empty. |

`PORT`, `SHUTDOWN_TIMEOUT`, and `SWIPE_NEXT_BATCH_SIZE` are optional with safe defaults. The three `SUPABASE_JWT_*` variables, `SUPABASE_DB_URL`, and `PING_TOKEN` are all required — the server refuses to start if any is missing (fail-fast via `ConfigFromEnv` or inline check in `run()`).

## Testing patterns

- **Handler unit tests use `t.Parallel()`** (see `TestHealthEndpoint`, `TestRootEndpoint`).
- **Lifecycle tests are sequential** (no `t.Parallel()`). They affect process-wide state.
- **Free port discovery**: `net.Listen("tcp", "127.0.0.1:0")` → read `Addr()` → `Close()` → start the server on that port (`freePort` + `waitHealthy` polling). Never hardcode ports.
- **Do not send real signals to the test process** (e.g. `syscall.Kill(os.Getpid(), SIGTERM)`). Signals are delivered process-wide and race with `t.Parallel()` tests and the test runner itself. Reproduce the meaning of `signal.NotifyContext` by **cancelling a `context.WithCancel` directly**.
- **Silence logs in tests** with `slog.New(slog.DiscardHandler)`.
- **`t.Parallel()` is incompatible with `t.Setenv()`** — `t.Setenv` mutates process-global env state and the Go test framework will panic if a parallel test calls it. Tests that manipulate env vars must be sequential (no `t.Parallel()`).
- **`slog.SetDefault` regression tests must not use `t.Parallel()`** — Swapping the process-wide default logger is a global mutation. The pattern is: swap `slog.SetDefault` with a `slog.NewJSONHandler` backed by a `bytes.Buffer` inside `t.Cleanup` to restore the original, decode the buffer as `map[string]any`, and assert on field presence and shape. Do not assert on raw byte substrings — that is weaker than shape assertions. Because the test mutates a global, it must run sequentially.
- **Packages that use `testcontainers-go` each define their own `TestMain`** — testcontainers containers cannot be shared across process boundaries, so container lifecycle must be scoped to the package. Current packages with `TestMain`: `cmd/server`, `internal/repository`, `internal/database`.
- **`bootstrapAuthSchema` fixture is required before migration** — migrations include a trigger that references `auth.users`. In production Supabase provides this schema, but the Postgres test container does not. Tests must call `bootstrapAuthSchema` to create the `auth` schema and `auth.users` table before applying migrations.
- **Assert trigger behavior, do not assume it** — the `handle_new_user` trigger must fire and create a `public.users` row when a user is inserted into `auth.users`. Do not add a fallback INSERT that silently masks trigger regressions; use `t.Fatalf` if the expected row is absent.
- **Always pass `tcpostgres.BasicWaitStrategies()`** when starting a Postgres container — this prevents race conditions caused by the container's init-time restart cycle. Omitting it can cause connections to fail intermittently before the server is ready.

## Logging

`log/slog` with a `JSONHandler` on `os.Stderr`. `slog.SetDefault` registers the process-wide default, and `e.Logger = logger` shares the same logger with Echo so request logs and application logs use a single format.

## GraphQL endpoint

`POST /query` is served by gqlgen. The schema lives under `schema/*.graphql` at the repo root and is consumed by `backend/gqlgen.yml` via a relative glob (`../schema/*.graphql`), so both backend (gqlgen) and frontend (graphql-codegen) see the same source of truth.

### Schema extension rules

- `extend type Query { ... }` works without ceremony — gqlgen merges all `extend type Query` blocks automatically. Use it freely when adding fields to the root query type.
- For mutations, declare `type Mutation { ... }` (not `extend`) for the **first** mutation definition in the schema. Subsequent additions use `extend type Mutation { ... }`.
- **Paginated lists use Relay-style Connection types** (`*Connection` / `*Edge` / `PageInfo`), not bare `[T!]!`. See `docs/pagination.md` for the full design and migration contract.

### Regeneration

The gqlgen CLI is pinned through the `tool` directive in `backend/go.mod` (Go 1.24+). Regenerate from the `backend/` directory:

```bash
go tool gqlgen generate
```

CI regenerates these artifacts before every build — they are intentionally **git-ignored**. Only hand-written resolver implementations under `backend/graph/resolver/*.resolvers.go` are committed. CI still fails the build if `go tool gqlgen generate` produces a diff against committed resolver stubs, so editing `schema/*.graphql` obliges you to regenerate locally and commit any new stub that appears under `backend/graph/resolver/`.

### Resolver layout

- `backend/graph/resolver/resolver.go` — hand-written root `Resolver` struct. DI-only; gqlgen never rewrites this file.
- `backend/graph/resolver/*.resolvers.go` — generated per schema object but committed, because function bodies hold hand-written implementation. gqlgen appends new stubs on regenerate but **never rewrites existing function bodies**. However, gqlgen **always rewrites the file's import block and its managed regions**. Place any hand-written helpers (e.g. `toUserModel`) at the **bottom of the file, outside all `// Code generated by ...`-marked regions**, so they survive regeneration.
- `backend/graph/generated/generated.go` — gqlgen runtime. **Git-ignored**, regenerated by CI before build.
- `backend/graph/model/models_gen.go` — generated struct types. **Git-ignored**, regenerated by CI. Custom scalars / type overrides belong in `gqlgen.yml` under `models:`. Hand-written helpers in the same package go in `doc.go` (or sibling files).

### `resolver: true` field binding

Mark a field in `gqlgen.yml` as `resolver: true` to make gqlgen generate a dedicated resolver method instead of populating the struct field directly. This is required for cross-aggregate references resolved via DataLoader:

```yaml
# gqlgen.yml
models:
  Cardgroup:
    fields:
      owner:
        resolver: true
```

gqlgen generates a `cardgroupResolver.Owner(ctx, obj)` method. The struct field stays in the generated model for JSON marshalling but is left `nil` in the repository helper; the resolver populates it lazily via the User DataLoader. Any resolver that calls a loader must wrap the error before returning — bare loader errors lack `extensions.code`:

```go
u, err := loader.For(ctx).User.Load(ctx, obj.OwnerID)()
if err != nil {
    return nil, gqlerr.Internal(ctx, err)
}
```

### Resolver field-name collision

When the schema has both a query named `cardgroup(id:)` and a type named `Cardgroup`, the `*Resolver` struct cannot have a field also named `Cardgroup` — it collides with the gqlgen-generated `Cardgroup() CardgroupResolver` method. Name the struct field `CardgroupUC` (or another non-colliding name). Future entities with a query name matching their type name (Card, Swipe, etc.) will hit the same issue.

### `Time` scalar binding

`gqlgen.yml` binds the `Time` scalar to `github.com/99designs/gqlgen/graphql.Time`, which wraps `time.Time` with RFC-3339 marshalling. No custom scalar implementation is needed:

```yaml
models:
  Time:
    model: github.com/99designs/gqlgen/graphql.Time
```

### Why `graph/model/doc.go` exists

gqlgen deletes `graph/model/models_gen.go` at the start of every run before regenerating it. If `models_gen.go` is the only file in the package, the package becomes unparseable during that window, and the `autobind: - backend/graph/model` entry in `gqlgen.yml` fails to resolve. `graph/model/doc.go` is a three-line hand-written package declaration that keeps the package loadable across gqlgen runs. Keep it even after real model types land — deleting it will reintroduce the chicken-and-egg autobind failure on the next clean regeneration.

### Playground

`GET /playground` exposes a browser UI for hand-crafted queries against `/query`. The route is enabled in all environments; the playground UI loads regardless of `GRAPHQL_INTROSPECTION`, but introspection queries (`__schema` / `__type`) are blocked when the variable is set to `off`. See [Introspection gating](#introspection-gating).

### Resolver DI seam

`newRouter(resolvers *resolver.Resolver, authMW echo.MiddlewareFunc, userRepo repository.UserRepository, roleRepo repository.RoleRepository) *echo.Echo` is the DI wiring seam. `run(ctx, logger) error` is the lifecycle seam — it constructs the `Resolver`, passes it to `newRouter`, and owns the `http.Server`. Middleware-shaped dependencies (auth, future per-request observability) are passed as `echo.MiddlewareFunc` parameters to `newRouter`; resources required by middleware factories (e.g. `loader.Middleware` needs repositories) are passed as additional `newRouter` arguments rather than hidden inside the middleware closure.

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

### Echo v5 + gqlgen error propagation

`echo.WrapHandler` (v5) converts a `http.Handler` into an `echo.HandlerFunc` that always returns `nil`. gqlgen's `handler.Server` is an `http.Handler`: it writes GraphQL errors into the response body as `{"errors":[...]}` with HTTP 200, and only ever writes a 5xx for catastrophic transport failures. Because `WrapHandler` returns `nil`, Echo's central error pipeline never sees these, which is fine: the GraphQL error is already transported in-band. Do **not** wrap gqlgen with a custom adapter that translates non-2xx into `echo.NewHTTPError` — that would cause a double write on the already-committed `ResponseWriter`.

## Database

### Pool and interface design

A single `pgxpool.Pool` is created at startup. `stdlib.OpenDBFromPool` converts it into a `*sql.DB`, which is then handed to GORM. The result is **one pool, two interfaces** — pgx native for low-level queries and GORM for the ORM layer — without double-consuming Render free tier's connection limit.

### Migrations

Migrations use `golang-migrate` with `*.up.sql` / `*.down.sql` files. Raw SQL lets you express triggers, foreign keys, and `SECURITY DEFINER` functions directly, none of which GORM's AutoMigrate can model. AutoMigrate is therefore not used.

Migration files live under `backend/internal/database/migrations/`. Go's `//go:embed` directive does not allow `..` path components, so the migration directory must sit inside the package tree rather than at the repo root.

**Filename format: `yyyymmddhhmmss_<short_snake_case_description>.{up,down}.sql`** — the 14-digit timestamp prefix is the numeric version `golang-migrate` records in `public.schema_migrations` and uses to order files. New migrations therefore need a timestamp strictly greater than every existing file (UTC is fine; the values just need to sort correctly). The trailing description is for human readers and is not parsed — pick a short verb-led summary like `create_cards`, `enable_rls_deny_all`, or `initial_schema`. Up and down halves must share the same prefix and description so `golang-migrate` can pair them.

When a migration fails mid-run, `schema_migrations.dirty=true` is set. Recovery requires an operator to run `migrate force <version>`. The `run()` function treats any migration error as fatal and returns immediately (fail-fast). See `docs/playbook-patterns.md` § "Recovering from a dirty migration" for the operator runbook.

#### golang-migrate transaction behaviour

**The `pgx/v5` driver does NOT auto-wrap each migration file in a transaction.** Every migration that requires atomicity must open its own `BEGIN; ... COMMIT;` block explicitly. A migration file that omits `BEGIN/COMMIT` and mixes DDL with privilege-sensitive statements (e.g. `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`) can partially succeed: the DDL commits, the later statement fails, and `schema_migrations.dirty=true` persists because golang-migrate commits that flag in its own separate transaction *before* it begins executing the migration SQL.

The structural consequence: **sensitive ALTER operations (RLS, GRANT, REVOKE) belong in their own dated migration file**, separate from the DDL that creates the tables. A failure in one file leaves the other file's work untouched, limiting blast radius.

#### Migration test quality bar

Tests that invoke the migration runner must assert *post-conditions*, not just "migrate ran without error". The model is `TestMigrations_AllPublicTablesHaveRLSEnabled` in `internal/database/pool_test.go`: it queries `pg_class` after migration and asserts every expected table has `relrowsecurity = true`. RLS policy behavior is covered by `internal/database/rls_test.go`, which connects as the Supabase-style `authenticated` role and sets local JWT claims before querying. Procedural success alone does not verify security posture.

#### `schema_migrations` and RLS

`public.schema_migrations` is golang-migrate's internal bookkeeping table. It is intentionally **excluded from the RLS-enable migration** for two reasons: (a) golang-migrate connects as the table owner, which in PostgreSQL bypasses RLS unless `FORCE ROW LEVEL SECURITY` is set, so enabling RLS on `schema_migrations` adds no security value; (b) if ownership ever changes and RLS without policies takes effect, golang-migrate would be blocked from updating the version record, bricking future deploys. Leave `schema_migrations` without RLS.

**Renaming or renumbering migration files is not transparent to the DB.** `golang-migrate` records the numeric version of each applied migration in `public.schema_migrations`. Renaming a file (e.g. `0001_create_profiles.up.sql` → `20250101000000_create_profiles.up.sql`) rewrites the source tree but **not** the DB row, so the next boot fails with `no migration found for version <N>: read down for version <N> migrations: file does not exist` — migrate's source-state reconciliation expects the recorded version to exist on disk. When the rename is identifier-only (up/down SQL bodies are byte-identical, `git log -M` reports an `R100` rename), the safe recovery is `UPDATE public.schema_migrations SET version = <new_version>, dirty = false WHERE version = <old_version>` against the production DB; the runbook lives in `playbooks/setup-prod/recover-migration-version-rebase.sql`. Do **not** apply this shortcut when the rename also changed migration content — in that case, squash the changes and use `migrate force <version>` against a known-good source state so the new content actually runs.

#### SECURITY DEFINER helper recipe

`SECURITY DEFINER` SQL functions used by RLS policies (e.g. `public.is_admin(uid uuid)`) must replicate this exact shape — each attribute has a load-bearing reason:

```sql
CREATE OR REPLACE FUNCTION public.is_admin(uid uuid)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$ ... $$;

REVOKE ALL ON FUNCTION public.is_admin(uuid) FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        GRANT EXECUTE ON FUNCTION public.is_admin(uuid) TO authenticated;
    END IF;
END
$$;
```

- `STABLE`, **not** `IMMUTABLE` — the function reads tables, and marking a table-reading function `IMMUTABLE` corrupts the planner's plan cache (the planner assumes the result is constant for fixed inputs).
- `SECURITY DEFINER` — the function executes as its owner, so RLS-enabled callers can probe role membership without needing direct read on `roles` / `user_roles`.
- `SET search_path = public` — neutralises the classic `SECURITY DEFINER` injection vector where an attacker creates a `pg_temp` shim function (e.g. their own `roles` table) that the function would otherwise resolve before the real one.
- `REVOKE ALL FROM PUBLIC` then narrow `GRANT EXECUTE` — without revoking from `PUBLIC`, anonymous PostgREST callers (`anon` role) could invoke the helper as an oracle to enumerate role assignments. The grant is intentionally limited to the Supabase-managed `authenticated` role.
- `DO $$ ... IF EXISTS pg_roles ... GRANT END $$` portability guard — plain PostgreSQL does not include Supabase-managed roles (`authenticated`, `anon`, `service_role`) by default. Wrapping role-specific GRANTs in this conditional `DO` block keeps the migration applicable outside Supabase. Testcontainers create a minimal `authenticated` role fixture so RLS behavior can be exercised directly. The Go backend connects as the table owner and bypasses RLS, so the GRANT path is used only by direct PostgREST / Edge callers in production.

### Startup order

```
database.Migrate(url)
→ database.Open(ctx, cfg)
→ repository.NewUserRepository(db.GORM)
→ repository.NewRoleRepository(db.GORM)
→ repository.NewCardgroupRepository(db.GORM)
→ repository.NewPingRecordRepository(db.GORM)
→ server start
```

### Environment variables (database)

See also the general env-vars table above.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `SUPABASE_DB_URL` | yes | — | Supabase Postgres DSN (`postgres://...?sslmode=require`) |
| `DB_MAX_CONNS` | no | `10` | Maximum pool connections |
| `DB_MIN_CONNS` | no | `0` | Minimum pool connections kept alive |
| `DB_MAX_CONN_LIFETIME` | no | `30m` | Maximum lifetime of a pooled connection |
| `DB_MAX_CONN_IDLE_TIME` | no | `5m` | Maximum idle time before a connection is evicted |

## GraphQL operations

### Returning GraphQL errors from resolvers

Use `*gqlerror.Error` (from `github.com/vektah/gqlparser/v2/gqlerror`) as the error type returned from resolvers and usecases. It is the only type that gqlgen's transport layer encodes into the `{"errors":[...]}` envelope. Plain `errors.New(...)` values are also transported, but they lose the ability to carry `extensions` (e.g. `code`, `field`). `errors.Is`/`errors.As` work on `*gqlerror.Error` as usual.

### `me` query and `updateProfile` mutation

Both operations require an authenticated caller. When `auth.UserFrom(ctx)` returns `nil` (no valid JWT), the usecase layer returns a `*gqlerror.Error` with `extensions.code = "UNAUTHENTICATED"`. gqlgen translates this into the standard GraphQL error envelope:

```json
{ "errors": [{ "message": "unauthenticated", "extensions": { "code": "UNAUTHENTICATED" } }], "data": null }
```

`data` is `null` — not omitted — because the field is non-nullable in the schema.

### Partial-update semantics for `bio`

`UpdateProfileInput.Bio` is `*string` throughout the stack:

- `nil` (field omitted in the JSON input) → "leave unchanged" — the repository skips the column in the `UPDATE`.
- `""` (field present, empty string) → "explicit clear" — the repository writes an empty string.

This three-state pointer distinction is preserved end-to-end: schema (`bio: String`) → gqlgen model (`Bio *string`) → `usecase.UpdateUserInput.Bio *string` → `repository.UserUpdate.Bio *string`. Never collapse it to a plain `string` default.

### Layering rule

```
resolver (schema.resolvers.go)
  └─ usecase (internal/usecase/user.go)
       └─ repository (internal/repository/)
```

Resolvers are intentionally thin: extract `model.UpdateProfileInput`, map it to `usecase.UpdateUserInput`, delegate, and return. Auth checks, validation, and `gqlerror.Error` construction live in the usecase layer. Shared error helpers live in `backend/internal/gqlerr` — see [Error helpers](#error-helpers-backendinternalgqlerr).

### DI pattern

`backend/graph/resolver/resolver.go` holds:

```go
type Resolver struct {
    User        *usecase.UserUsecase
    CardgroupUC *usecase.CardgroupUsecase  // "UC" suffix avoids collision with the Cardgroup() resolver method
}
```

`cmd/server/main.go::run()` wires it:

```go
userRepo      := repository.NewUserRepository(db.GORM)
roleRepo      := repository.NewRoleRepository(db.GORM)
cardgroupRepo := repository.NewCardgroupRepository(db.GORM)
cardRepo      := repository.NewCardRepository(db.GORM)
resolvers := &resolver.Resolver{
    User:        usecase.NewUserUsecase(userRepo),
    CardgroupUC: usecase.NewCardgroupUsecase(cardgroupRepo),
    CardUC:      usecase.NewCardUsecase(cardRepo, cardgroupRepo),
}
```

Adding a new feature: build a usecase, add a field to `Resolver`, wire it in `run()`. See [Resolver field-name collision](#resolver-field-name-collision) if the entity name matches a query name.

### Aggregate boundary policy

Cross-aggregate references use IDs only — never embed a pointer to another aggregate's struct. For example, `domain.Cardgroup` holds `OwnerID string`, not `Owner *domain.User`, and `domain.Card` holds `CardgroupID string`, not `Cardgroup *domain.Cardgroup`. This prevents cyclic imports, keeps aggregates independently serialisable, and enforces the DDD consistency boundary. The actual object is resolved lazily by GraphQL field resolvers via the per-request DataLoader.

### Cursor pagination

Relay-style Connection queries (e.g. `cardsByCardgroupConnection`) follow a fixed shape across schema, resolver, usecase, and repository. See `docs/pagination.md` for the full design (tuple comparison, `+1` fetch trick, `totalCount` trade-off, cross-aggregate validation, three-layer enum sync, and `cursorFieldValue` error handling).

### Consumer-driven repository interfaces

The usecase layer defines its own narrow repository interface — a strict subset of the concrete repository. For example, `usecase.CardgroupRepository` omits `FindByIDs`, which is used only by the DataLoader batch function and therefore belongs in `repository.CardgroupRepository`, not in the usecase interface. This reduces the usecase test surface and makes intent explicit: if a method is not in the usecase interface, the usecase layer never calls it.

### Authorization at the usecase layer

Owner checks live in the usecase, not in Postgres RLS. The asymmetry for read vs. write is intentional per the security model:

- Non-owner `cardgroup(id:)` read → return `null` (the field is nullable by spec; ID enumeration on a nullable field is acceptable).
- Non-owner write (`updateCardgroup`, `deleteCardgroup`) → return `UNAUTHENTICATED`.

Although authorization itself is not delegated to Postgres, every application table in the `public` schema has Row Level Security enabled. Core user data tables now have concrete policies for direct Supabase callers:

- `users`: a caller can select or update only their own row; admins can select or update any row.
- `cardgroups` and `cards`: owners have full row access through `cardgroups.owner_id`; admins bypass ownership.
- `swipe_records`: callers can select their own review log; admins can read all logs. Inserts are limited to `user_id = auth.uid()` so admins cannot write swipes for another user.
- `roles`: selectable by all callers; mutations are admin-only.
- `user_roles`: callers can read their own assignments; admins can read and mutate all assignments.

The Go backend connects as the table-owner role, which bypasses RLS unless `FORCE ROW LEVEL SECURITY` is set, so application queries and migrations are unaffected. `FORCE ROW LEVEL SECURITY` is intentionally not enabled. The `schema_migrations` bookkeeping table is excluded from RLS — see [schema_migrations and RLS](#schema_migrations-and-rls) for the rationale. If a future flow needs Supabase JS to read a new table directly, add a targeted policy alongside the access pattern; do not disable RLS to "make it work".

### Role-based authorization (`auth.Service`)

The `auth` package exposes two distinct types with different lifetimes and data sources. They answer different questions and must not be conflated:

| Type | Lifetime | Source | Question |
|---|---|---|---|
| `AuthUser` (`auth/user.go`) | Request-scoped | JWT claims (Supabase) | Who is the caller? |
| `auth.Service` (`auth/role.go`) | Boot-scoped | DB-backed (`UserRoleRepository`) | What can the caller do? |

The split is deliberate: the JWT does not carry roles in this project, so every role check goes through the DB. `auth.Service` is constructed once at boot in `run()` against the `UserRoleRepository` and injected into resolvers/usecases that need to gate on role membership.

`auth.Service.IsAdmin(ctx, userID)` hardcodes the literal `"admin"` role name in the method body — callers cannot pass a role string. This prevents drift to bespoke role names; add a new dedicated method (e.g. `IsModerator`) when a second role is needed rather than parameterising `IsAdmin`.

The DB side of the same check is `public.is_admin(uid uuid) RETURNS boolean`, defined in migration `20260502000000_add_rbac_helpers`. See [SECURITY DEFINER helper recipe](#security-definer-helper-recipe) for the function shape RLS policies and future RBAC helpers must replicate.

### Sentinel errors and domain validation

`domain.Cardgroup.Validate()` returns typed sentinels (`ErrCardgroupNameRequired`, `ErrCardgroupNameTooLong`). The usecase translates them via a dedicated helper (`translateCardgroupNameErr`) to `gqlerr.BadUserInput`. The rule itself lives only in the domain; the usecase holds only the domain→GraphQL mapping. Apply this pattern to every new aggregate.

### Validation rules in `usecase.UpdateUser`

- `displayName` is `strings.TrimSpace`-ed, then validated as 1–50 grapheme clusters via `rivo/uniseg`.
- `bio` accepts up to 500 grapheme clusters (no trim; whitespace is preserved).
- Out-of-bounds returns `gqlerr.BadUserInput("displayName", ...)` (extensions.code = "BAD_USER_INPUT", field = "displayName"). See [Validation (grapheme clusters)](#validation-grapheme-clusters) for the WHY.

### Card bulk delete + FSRS override

The `deleteCards(ids: [ID!]!) -> Int` mutation deletes cards owned by the authenticated caller and returns the count of rows actually deleted. Ownership is enforced exclusively by the SQL subselect in `DeleteByIDsTx` (one DELETE scoped to cardgroups owned by the caller); foreign-owned ids are silently skipped at the SQL layer and the response count reflects only rows actually deleted — clients use this to update their cache. At most `maxBulkDelete = 100` ids may be supplied per call; exceeding the cap returns `BAD_USER_INPUT` on the `ids` field. The repository's `DeleteByIDsTx` short-circuits on an empty slice to avoid `WHERE id IN ()`, which triggers a full-table scan in Postgres.

`NewCardInput` accepts optional all-or-nothing FSRS state overrides: nine fields (`due, stability, difficulty, elapsedDays, scheduledDays, reps, lapses, state, lastReview`) or none. Mixed input triggers `domain.ErrFSRSOverridePartial`; invalid state values trigger `domain.ErrFSRSOverrideStateInvalid`. This all-or-nothing semantics prevents mixed-state cards when importing a dictionary — a single bad value would otherwise corrupt the set. The `domain.NewFSRSStateFromInput` function centralizes the validation and sentinels; the usecase translates them to `gqlerr.BadUserInput` before returning.

### Integration tests

`backend/cmd/server/main_test.go` contains `TestGraphQL_Me_Anonymous`, `TestGraphQL_Me_Authenticated`, and `TestGraphQL_UpdateProfile_Authenticated`. These tests use the testcontainer Postgres, a local JWKS HTTP server (`jwtFixture`), and ECDSA-signed JWTs. Future GraphQL integration tests should reuse the same `jwtFixture` + `startServer` helpers rather than re-inventing the JWKS mock.

## Backend hardening

Five operational guardrails surround the `/query` endpoint: DataLoader
(N+1 prevention), typed error helpers, query complexity limits, introspection
gating, and grapheme-cluster validation. Each lives behind its own seam rather
than as a cross-cutting concern bolted on to middleware.

### DataLoader (per-request)

**Why:** Without batching, a list resolver that fetches N users or roles issues N
separate `SELECT` statements. DataLoader collapses those into a single
`SELECT ... WHERE id = ANY($1)`.

`backend/internal/loader/` exposes a per-request `Loaders` struct injected
via `loader.Middleware(userRepo, roleRepo, cardgroupRepo, cardRepo, swipeRecordRepo)`. The middleware is registered on the `/query`
group alongside `authMW`:

```go
q := e.Group("/query", authMW, loader.Middleware(userRepo, roleRepo, cardgroupRepo, cardRepo, swipeRecordRepo))
```

A fresh `Loaders` instance is created for every request so the per-request
cache never bleeds across authenticated users. Resolvers pull it out of `ctx`:

```go
user, err := loader.For(ctx).User.Load(ctx, userID)()
```

`loader.For` returns `nil` when the middleware was not installed; dereferencing
the returned pointer (`.User.Load(...)`) will then panic. Keep the middleware
wired to every route that touches a loader. Once a resolver actually calls a
loader in production, consider replacing `For` with a `MustFor` variant (panics
with a clear message on nil) or a `(loaders, error)` two-value return so
middleware misconfiguration is caught explicitly rather than as a nil-dereference.

**Library:** `github.com/graph-gophers/dataloader/v7` (generics edition).
The batch function receives `[]string` keys and must return
`[]*dataloader.Result[*domain.T]` with the **same length and index order**
as the input keys. dataloader/v7 enforces this 1:1 invariant at runtime.

**Adding a new entity (five touch points):**

1. Add `FindByIDs(ctx context.Context, ids []string) (map[string]*domain.X, error)` to the repository interface.
2. Create `backend/internal/loader/<entity>.go` with an `<entity>BatchFunc(repo)` that maps the result map back to the ordered output slice (see `user.go` for the pattern).
3. Add `<Entity> *dataloader.Loader[string, *domain.<Entity>]` to `Loaders`.
4. Initialise it in `loader.New(...)` and pass its repository through `loader.Middleware(...)`.
5. Add the new repository parameter to `newGraphQLTestServer` (and any test-server variants) in `cmd/server/main_test.go`.

**Loader test contract:** A loader test must verify both (a) the batch function was called exactly once (dedup working) AND (b) it received the expected number of distinct keys. Without (b), key collisions or dedup bugs can go undetected.

**Loader error wrapping:** Every resolver that calls `loaders.X.Load(ctx, key)()` must wrap the returned error via `gqlerr.Internal(ctx, err)` (or another typed gqlerr) before returning. Bare loader errors have no `extensions.code` and leak internal details.

**Transactional usecases must not call DataLoader:** DataLoaders are request-scoped and use the normal repository DB handle, not the `*gorm.DB` transaction handle passed into `db.Transaction(...)`. A usecase that needs read-your-writes consistency must call transaction-aware repository methods such as `FindByIDTx`, `UpdateFSRSStateTx`, or `FindDueCardsTx` directly with the `tx` argument. Keep `loader.For(ctx)` out of `internal/usecase/*` files.

### Error helpers (`backend/internal/gqlerr`)

**Why:** Raw `&gqlerror.Error{...}` literals scattered across resolvers and
usecases make `extensions.code` values inconsistent and hard to grep. The
`gqlerr` package centralises them behind three typed constructors.

| Helper | `extensions.code` | Notes |
|---|---|---|
| `gqlerr.Unauthenticated()` | `"UNAUTHENTICATED"` | No `field`; caller is not authenticated |
| `gqlerr.BadUserInput(field, message)` | `"BAD_USER_INPUT"` | `extensions.field` carries the form field name for FE error display |
| `gqlerr.NewForbidden(msg)` | `"FORBIDDEN"` | Caller is authenticated but lacks the required role/permission. Caller supplies a generic message — do not include sensitive details (e.g. "user X is not admin") |
| `gqlerr.Internal(ctx, err)` | `"INTERNAL"` | Message fixed to `"internal server error"`; original `err` logged via `slog.ErrorContext` and never sent to the client |

**Logging convention.** `Internal(ctx, err)` is the only constructor that logs — internal errors are server bugs and must surface in the structured log stream. The 4xx constructors (`Unauthenticated`, `BadUserInput`, `NewForbidden`) deliberately do **not** log: they describe expected client-side faults, and emitting an ERROR/WARN line on every malformed request would drown the signal. Call sites that want 401/403/400 observability must log themselves before constructing the error (a `slog.WarnContext` with the relevant attrs is the canonical pattern; see `auth/middleware.go` `reject()`).

Usage in the usecase layer:

```go
if user == nil {
    return nil, gqlerr.Unauthenticated()
}
if n > displayNameMax {
    return nil, gqlerr.BadUserInput("displayName",
        fmt.Sprintf("displayName must be at most %d characters", displayNameMax))
}
if err := repo.Update(ctx, id, patch); err != nil {
    return nil, gqlerr.Internal(ctx, err)
}
```

**Do not** write `&gqlerror.Error{Extensions: map[string]any{"code": "..."}}` in
resolvers or usecases. The `Code` type in `gqlerr` is a string alias that
makes ad-hoc code strings a compile error, and grep should always return zero
raw `gqlerror.Error` literals outside the `gqlerr` package itself.

`gqlerr.IsCode(err, gqlerr.CodeUnauthenticated)` is the canonical way to
inspect codes in tests and middleware.

**Why `gqlerr.Internal` is mandatory for repo/DB errors:** gqlgen's default error presenter forwards any error whose message is not already masked directly into the GraphQL response body. Unwrapped repository or database errors therefore leak internal details (table names, SQL, driver messages) to clients. Always wrap with `gqlerr.Internal(ctx, err)` before returning from a resolver or usecase — the helper logs the original error via `slog.ErrorContext` and replaces the message with the fixed string `"internal server error"`.

**Note:** `Code` is a string alias, not a true enum — `Code("ANYTHING")` is a valid expression. If ad-hoc code strings become a maintenance concern, introduce `enumcheck` (or a similar linter) to enforce that only declared constants are used.

### Complexity limit

**Why:** An unbounded GraphQL query can fan out into thousands of resolver
calls. A fixed limit caps the worst-case DB load without per-field tuning.

The gqlgen `extension.FixedComplexityLimit(100)` extension is registered
inside `newGraphQLServer`:

```go
srv.Use(extension.FixedComplexityLimit(100))
```

Each scalar field costs 1 point by default. When a query exceeds 100 points,
gqlgen rejects it during validation and returns an HTTP 200 with an
`errors[].message` that contains `"complexity"` — no resolver is invoked.

To raise the limit for a feature that genuinely needs it, change the single
constant argument. Values of 200 or 500 are reasonable stepping stones; avoid
setting it above 1000 without profiling.

**Tip:** When a query is rejected, `extensions.code` is `"COMPLEXITY_LIMIT_EXCEEDED"` (the gqlparser standard value). Client code can branch on this code to show a specific "query too complex" message rather than a generic error.

### Introspection gating

**Why:** Introspection exposes the full schema to anyone who can reach
`/query`. Disabling it in production prevents schema enumeration by
unauthenticated clients while keeping it on in dev for playground and
codegen tooling.

Control is via the `GRAPHQL_INTROSPECTION` environment variable:

| Value | Effect |
|---|---|
| `off` | Introspection disabled; `__schema` queries return a validation error |
| anything else (including unset) | Introspection enabled |

The `GET /playground` route is unaffected — the playground UI loads regardless.
Only the `__schema` and `__type` queries are blocked when introspection is off.

Production sets `GRAPHQL_INTROSPECTION=off` on the Render service (Settings → Environment). Dev and CI leave the variable unset, so the playground remains fully functional.

**Tip:** When introspection is disabled, the error message is exactly `"introspection disabled"` (lowercase, no trailing punctuation). Test assertions can match on this literal string.

### Validation (grapheme clusters)

**Why:** String length measured in bytes or UTF-16 code units does not match
what users perceive as "characters". A 👨‍👩‍👧‍👦 ZWJ sequence is 11 bytes but one
visible character. Backend and frontend must agree on the same counting rule to
avoid inconsistent rejections.

`backend/internal/usecase/user.go` uses
`github.com/rivo/uniseg` (UAX #29 compliant) via
`uniseg.GraphemeClusterCount(s)`:

| Field | Rule | Trim before check? |
|---|---|---|
| `displayName` | 1–50 grapheme clusters | Yes (`strings.TrimSpace`) |
| `bio` | 0–500 grapheme clusters | No |

`bio` accepts `nil` (field omitted → leave unchanged) and `*""` (field present
but empty → explicit clear). See [Partial-update semantics for `bio`](#partial-update-semantics-for-bio).

The frontend uses `Intl.Segmenter` with the same UAX #29 algorithm, so
character counts agree between the JS form validator and the Go backend. When
they diverge (edge cases in older browsers), the backend error is canonical and
must be surfaced as a form-level `BAD_USER_INPUT` error on the client.

**DB CHECK vs. domain bound divergence:** Postgres `char_length(trim(name)) BETWEEN 1 AND 100` counts Unicode code points, not grapheme clusters. A 100-grapheme ZWJ-emoji string can exceed 500 code points and be rejected by the DB even though the domain accepts it. The domain bound is authoritative; the DB constraint is a coarse floor only. Do not rely on the DB constraint to enforce business rules — the domain `Validate()` method is the single source of truth.

## Backend gotchas

### `uuid.NewV7` failure must propagate

`uuid.NewV7()` fails only when `crypto/rand` is unavailable, meaning the system is already unhealthy. A silent fallback to `uuid.NewV4()` is not safe — `NewV4` calls the same random source and will also fail. Helper functions that generate IDs must return `(string, error)` and let callers map the failure to `gqlerr.Internal`. The request-ID middleware is the one deliberate exception because it uses a timestamp string as fallback; that is specific to logging context, not business-logic IDs.

### Empty-patch optimization in repositories

A repository `Update` that receives a no-op patch (all fields nil or unchanged) should return the current row without issuing an `UPDATE`. This avoids unnecessarily touching `updated_at` and helps idempotent clients. Guard by checking whether the patch struct carries any non-nil field before building the GORM `Updates` call.

### Echo v5 handler signature uses a pointer receiver

Echo v5 handler and middleware signatures changed from v4. Every handler and middleware factory must use `*echo.Context` (pointer), not the v4 interface form:

```go
// v5 — correct
func handler(c *echo.Context) error { ... }

// v4 — will not compile or will behave wrong in v5
func handler(c echo.Context) error { ... }
```

Online samples, AI-generated code, and the official Echo v4 docs all use the interface form. Any paste from those sources requires this fix.

The type is `*echo.Context` — a **pointer to a concrete struct**, not an interface. v5 removed the `echo.Context` interface entirely, so there is no interface to embed or assert against.

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

### GORM `WHERE id IN ?` with an empty slice returns all rows

Passing an empty `[]string{}` to a GORM query like `db.Where("id IN ?", ids).Find(&rows)` does **not** emit `WHERE id IN ()`. GORM silently drops the clause and performs an unfiltered full-table scan, returning every row. Guard every batch-fetch path with an early return:

```go
if len(ids) == 0 {
    return nil, nil
}
```

This matters most in DataLoader batch functions, where an empty key slice is a normal edge case.

### GORM v1 string-typed primary key with DB-generated UUID requires `default:` tag

GORM's `BeforeCreate` hook auto-generates a UUID when a primary key field is a `string` type and is zero-valued — but only when the field carries `gorm:"default:..."` in its tag. Without the tag, GORM leaves the field empty and the INSERT fails.

Peer models (`gormUser`, `gormCard`) avoid this by supplying the UUID in the application layer before calling `Create`. `gormPingRecord` is different: `Create` inserts a row without any caller-supplied ID, so the DB must generate it via `gen_random_uuid()`. The tag `gorm:"default:gen_random_uuid()"` is therefore load-bearing even though `AutoMigrate` is not used and the column default is already defined in the migration SQL.

### GORM rejects unconditional `Delete` — use `Where("1 = 1")` to opt out

GORM v2+ refuses a `Delete` call that has no `WHERE` clause as a safety net against accidental full-table deletes. It returns an `ErrMissingWhereClause` error.

The deliberate opt-out for legitimate full-table deletes is:

```go
result := db.Where("1 = 1").Delete(&gormPingRecord{})
```

This makes the intent explicit and satisfies GORM's guard without suppressing the error check.

### `subtle.ConstantTimeCompare` leaks token length — pair with a rate limiter

`crypto/subtle.ConstantTimeCompare` returns early (0) when the two byte slices differ in length, leaking length via timing. For equal-length inputs the comparison runs in constant time. The practical impact for bearer-token checking is small when the token length is public knowledge (e.g. a fixed 64-hex-char token), but the leak becomes meaningful for variable-length or secret-length tokens without an external mitigation.

The `/internal/ping` handler pairs `ConstantTimeCompare` with a per-IP rate limiter (1 req/s, burst 5). The rate limiter makes the length-oracle non-exploitable in practice: an attacker cannot iterate quickly enough to extract useful information before being throttled. Any future endpoint that adopts bearer-token comparison **without** a rate limiter must also add one — or switch to a constant-time scheme that does not branch on length.

### slog context enrichment must precede the log call that announces the enrichment

When a middleware sets a value in the context and then logs a message about
that action, the `slog.*Context` call must come **after**
`c.SetRequest(c.Request().WithContext(ctx))`. Logging before the context is
stored means the very line that announces the event carries no `request_id` (or
other context attribute) itself. The pattern in
`backend/internal/middleware/request_id.go` — enriching the context first, then
calling `slog.WarnContext(ctx, ...)` — is the correct template for any
context-enriched slog handler.

### `slog.Handler.WithGroup` nests subsequent attrs inside the group object

Calling `handler.WithGroup("g")` on a `slog.JSONHandler` (or any handler that
wraps one, such as `logging.ContextHandler`) causes **all** attrs added
afterward — including those injected by `Handle` via `r.AddAttrs` — to appear
under the `"g"` JSON key, not at the top level. In `logging.ContextHandler`,
`request_id` is added via `r.AddAttrs` inside `Handle`, so after
`WithGroup("grp")` the log line becomes `{"grp":{"request_id":"...","k":"v"}}`.
Log queries and tests that expect top-level `request_id` will miss it.
`backend/internal/logging/handler_test.go` (`TestContextHandler_WithAttrsAndWithGroupPreserveRequestID`)
documents and asserts this shape.

## Observability

The backend emits OpenTelemetry traces via `otelgqlgen` for every GraphQL operation, resolver, and scalar field. Spans are exported via OTLP HTTP (port 4318) to whatever collector `OTEL_EXPORTER_OTLP_ENDPOINT` points to.

### What gets traced

Per GraphQL request, the following spans are emitted (concrete names depend on the `otelgqlgen` version):

- One operation-level span named after the GraphQL operation (`Me`, `UpdateProfile`, `Health`).
- One resolver span per top-level resolver (`Query.me`, `Mutation.updateProfile`).
- One child span per field resolver for non-trivial fields (`User.displayName`, `User.bio`, `User.avatarUrl`).
- Span attributes include `graphql.operation.type`, `graphql.operation.name`, and `graphql.error.code` (when the operation errors).

### Noop fallback

If `OTEL_EXPORTER_OTLP_ENDPOINT` is empty or missing, the server installs a no-op TracerProvider and logs `telemetry disabled`. Startup is **not** blocked on collector availability — this is deliberate so dev / CI do not require a running Jaeger.

### Sampler

`ParentBased(TraceIDRatioBased(ratio))`. The `ratio` is read from `OTEL_TRACES_SAMPLER_ARG` (default `1.0`, i.e. sample everything). In production, set this to a low value (e.g. `0.1`) via Render env. `ParentBased` means that if a caller provides a sampling decision via `traceparent`, we respect it.

### Trace context gap from the frontend

The current backend does not forward `traceparent` from the Next.js frontend — each backend request is its own root trace. Frontend-side OTel + `traceparent` propagation is tracked in [#22](https://github.com/yasuflatland-lf/flamingo-armond/issues/22) and will introduce a parent span that backend spans nest under.

### Shutdown ordering

The graceful-shutdown goroutine shuts down in this order:

1. `srv.Shutdown(sctx)` — drain in-flight HTTP requests.
2. `db.Close()` — release the pgx pool.
3. `tracerShutdown(sctx)` — flush pending spans to the exporter.

Tracer shutdown is last so that DB-layer spans emitted during request drain still reach the exporter. Tracer shutdown errors log a warning but do not fail the process.

### Environment variables (observability)

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | *(empty)* | OTLP HTTP endpoint (e.g. `http://localhost:4318`). Empty = tracing disabled. |
| `OTEL_TRACES_SAMPLER_ARG` | no | `1.0` | Float in `[0, 1]`. Parse errors and out-of-range values each emit a `slog.Warn` and fall back to `1.0`. |
| `APP_ENV` | no | `development` | Used as `deployment.environment` resource attribute. Set to `production` on Render. |

## Request ID propagation

Every HTTP request handled by the backend carries an `X-Request-ID` header. The
header provides a cheap, grep-friendly correlation ID that flows from the
frontend through the backend without requiring a full distributed-tracing stack.
It complements, rather than replaces, the OTel span tree tracked in
[#22](https://github.com/yasuflatland-lf/flamingo-armond/issues/22).

### Header and length cap

The middleware (`backend/internal/middleware/request_id.go`) reads
`X-Request-ID` from the incoming request. An upstream value is accepted as-is
when it is non-empty and at most **128 characters**. Values longer than that are
dropped and replaced by a freshly generated ID. The cap prevents log injection
and guards against accidentally forwarding an unbounded upstream payload into
structured log fields.

### ID generation

When no valid upstream ID is present, the middleware generates one with
`uuid.NewV7()` (timestamp-prefixed, lexicographically sortable). If `NewV7`
fails (e.g. the random source is temporarily unavailable), the fallback is a
nanosecond timestamp string (`fmt.Sprintf("fallback-%d", time.Now().UnixNano())`).

**Do not replace the fallback with `uuid.NewString()`.** `uuid.NewString` calls
`Must(uuid.NewRandom())` internally, which panics on the same `crypto/rand`
failure that caused `NewV7` to fail — it is not a safe fallback.

### Middleware position

```go
e.Use(middleware.RequestLogger())
e.Use(middleware.Recover())
e.Use(internalmw.RequestID())  // third — runs on every route
```

`RequestID` is registered immediately after `Recover` so that even error
responses produced by panicking handlers carry the header. It applies globally —
`/health`, `/playground`, and `/query` all receive it.

The middleware sets `X-Request-ID` on the **response** before calling `next(c)`,
so error paths also expose the header to callers.

### slog integration

`main()` wires the request-aware logger:

```go
logger := slog.New(
    logging.NewContextHandler(
        slog.NewJSONHandler(os.Stderr, nil),
        internalmw.RequestIDFromContext,
    ),
)
slog.SetDefault(logger)
```

`logging.NewContextHandler` wraps any `slog.Handler`. On every `Handle` call it
reads the request ID from the record's context via the supplied `ContextLookup`
function and appends it as `"request_id"` before delegating to the inner
handler. The result: **every `slog.*Context` call** (`slog.InfoContext`,
`slog.ErrorContext`, etc.) automatically includes `"request_id"` in the JSON
log line. No resolver or usecase needs to pass it manually.

`slog` calls made **without** a context (`slog.Info(...)`) do not carry a
request ID — that is by design; they represent process-level events rather than
per-request ones.

### Decoupling via `ContextLookup`

`logging` does not import `middleware`. The wiring above passes
`internalmw.RequestIDFromContext` as a plain `func(context.Context) string` so
the two packages remain independent of each other's import graph.

### Helper

```go
id := internalmw.RequestIDFromContext(ctx) // returns "" when not present
```

## Automatic Persisted Queries

`extension.AutomaticPersistedQuery{Cache: lru.New(100)}` is always registered on the gqlgen handler. There is no env gate — hash-less queries still work as normal POST bodies, so dev / playground flows are unaffected. The first request that contains `extensions.persistedQuery.sha256Hash` populates the LRU cache; subsequent hash-only requests from the same client short-circuit to the cached query text without sending the full document.

### APQ / Observability implementation notes

#### Library compatibility pins

`github.com/ravilushqa/otelgqlgen` must be pinned to **v0.17.0** when using `github.com/99designs/gqlgen v0.17.66`. Upgrading to `otelgqlgen` v0.19.x transitively bumps gqlgen to v0.17.73, which changes the generated `ComplexityRoot` signature and breaks `backend/graph/generated/`. The v0.17.x line of `otelgqlgen` matches the gqlgen v0.17.66 generics era.

The APQ LRU cache must be constructed as `lru.New[string](100)` (generic form). gqlgen v0.17.66 made the cache interface generic; the non-generic `lru.New(100)` form shown in older online docs no longer compiles against this version.

#### `otelgqlgen` span naming (empirically verified against v0.17.0)

- **Operation span**: bare operation name — `Me`, `UpdateProfile`, `Health`. Not `query Me` and not `graphql.execute`.
- **Resolver / field spans**: `<ObjectType>/<fieldName>` — e.g. `Query/me`, `User/displayName`.

Test assertions that match span names (e.g. `strings.HasPrefix(name, "User/")`) are anchored to `otelgqlgen@v0.17.0`. If `otelgqlgen` is upgraded, run the integration tests — a span-naming change will surface immediately as a failing assertion before it silently breaks production dashboards.

#### `tracetest.InMemoryExporter` shutdown clears the buffer

`exporter.Shutdown(ctx)` internally calls `Reset()`, which clears the recorded span buffer. The natural pattern `defer shutdown(); ...; spans := exp.GetSpans()` returns zero spans because the buffer was already cleared by the deferred shutdown. To read spans correctly: call `tp.ForceFlush(ctx)` to drain pending spans **before** calling `Shutdown`, or call `exp.GetSpans()` before `Shutdown` returns.

#### `resource.New` partial-error handling

`go.opentelemetry.io/otel/sdk/resource` returns `resource.ErrPartialResource` or `resource.ErrSchemaURLConflict` **alongside a usable `*Resource`** when one detector fails (e.g. a host-info detector blocked by container restrictions). Do not `return err` on these — use `errors.Is` to identify them, emit a `slog.Warn`, and pass the partial resource to `sdktrace.WithResource`. Aborting init on a partial-detector failure crashes the server in sandboxed environments where host introspection is restricted.

#### OTLP exporter goroutine ownership

`otlptracehttp.New` starts background goroutines and a persistent HTTP client. If `Init` creates the exporter but a later step (e.g. resource construction) errors before the SDK takes ownership, the exporter is orphaned and leaks. The fix is to call `exp.Shutdown(ctx)` in the failure path of `Init` so the background goroutines are always cleaned up.

#### Sampler env-var validation

`OTEL_TRACES_SAMPLER_ARG` parsing emits `slog.Warn` on two distinct conditions: a parse error (e.g. `0,1` with a comma instead of a decimal point) and an out-of-range value (outside `[0, 1]`). Both fall back to `1.0`. The two warnings are kept separate so operators can triage env config issues quickly — a comma typo produces a different message than a value of `1.5`.

## Error wrapping convention

The backend uses [`github.com/rotisserie/eris`](https://github.com/rotisserie/eris) as the **only** error-wrapping library inside `backend/internal/` and `backend/cmd/`. The convention is:

| Situation | Use |
|---|---|
| New error at the originating call site | `eris.New("layer: short message")` |
| Wrapping an error at a layer boundary | `eris.Wrap(err, "layer: context")` |
| Wrapping with format args | `eris.Wrapf(err, "layer: %s", id)` |
| Validation error without an underlying cause | `eris.Errorf("layer: %s must be >= %d", field, min)` |
| Sentinel that other code matches via `errors.Is` | `errors.New("...")` (do **not** use eris) |

`fmt.Errorf("...: %w", err)` is **forbidden** in `backend/internal/` and `backend/cmd/`. CI fails the build if any such call sneaks back in (see `.github/workflows/backend.yml`).

### Sentinels

Sentinels used today: `repository.ErrNotFound`, and domain-level sentinels such as `domain.ErrCardgroupNameRequired` / `domain.ErrCardgroupNameTooLong`. New sentinels are allowed when (a) callers need to branch on identity, and (b) a string-equality match is fragile. Keep sentinels as plain `errors.New` so `errors.Is` works without going through eris's chain walk.

### Logging

All error sites that produce a structured log entry must attach the eris chain as the `error_chain` attribute (output of `eris.ToJSON(err, true)`). ERROR-level sites use `internal/logging.LogError(ctx, logger, msg, err, attrs...)`, which:

- attaches `error_chain` automatically,
- emits at `slog.LevelError`,
- is a no-op when `err == nil`.

Non-ERROR sites (e.g. `slog.Warn` for client-side faults) attach the same `error_chain` value manually so log shape stays consistent.

Today's call sites:

1. **`gqlerr.Internal(ctx, err)`** — every internal-server error returned through GraphQL (uses `LogError`, ERROR level).
2. **`cmd/server/main.go main()`** — terminal error before `os.Exit(1)` (uses `LogError`, ERROR level).
3. **`auth/middleware.go reject(c, cause)`** — token-rejection log; uses `slog.Warn` directly with `eris.ToJSON(cause, true)` for log-shape parity.

### Why `eris` over alternatives

- **`fmt.Errorf("%w")`**: no stack trace, can only carry a string context.
- **`pkg/errors`**: last tagged release v0.9.1 in January 2020 with no upstream activity since; lacks the structured JSON chain serialization that this codebase relies on for the `error_chain` log attribute.
- **`cockroachdb/errors`**: heavier, drags in many transitive deps; revisit only when multi-service error portability or first-class Sentry SDK integration becomes a hard requirement.
- **`joomcode/errorx`**: typed-error focus, less aligned with our wrap-and-log need.

### What `error_chain` looks like

For an error `eris.Wrap(eris.New("inner failure"), "outer context")`, `eris.ToJSON(err, true)` produces (abbreviated):

```json
{
  "root": {
    "message": "inner failure",
    "stack": [
      "main.run:/path/server/main.go:42",
      "..."
    ]
  },
  "wrap": [
    {
      "message": "outer context",
      "stack": "main.run:/path/server/main.go:51"
    }
  ]
}
```

This is emitted as the `error_chain` field in the JSON log line, alongside `level=ERROR` and the human-readable `msg`. Render and any downstream collector (Datadog / Sentry / Cloud Logging) ingest this JSON without further parsing.

**`root.stack` vs `wrap[].stack` shapes differ.** `root.stack` is a JSON array of `"Method:File:Line"` strings. Each entry in `wrap[]` has a `stack` field that is a **single** `"Method:File:Line"` string, not an array. Writing a log parser or assertion that treats both as arrays is a silent bug — only `root.stack` is iterable.
