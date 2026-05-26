# Backend runtime notes

Design and constraints for the Go / Echo v5 backend that are not obvious from the code alone.

## Module layout

- Go version is pinned via mise (`backend/.tool-versions`, currently `golang 1.26.3`). CI resolves Go through `jdx/mise-action` with `working_directory: backend`.
- Module name is the bare `backend` (see `backend/go.mod`). All internal imports start with `backend/...`.
- All `go` commands **must run from `backend/`** (CI sets `defaults.run.working-directory: backend`; match that locally).

## Runtime shape

The server entry (`backend/cmd/server/main.go`) is deliberately split thin so tests can drive the full lifecycle without spawning a subprocess:

- `newRouter()` — builds `*echo.Echo` with `RequestID` → `Recover` → `RequestLogger` middleware (in that registration order) and exposes `GET /` and `GET /health`. Tests hit it directly via `httptest.NewServer`.
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
| `SUPABASE_JWKS_URL` | yes | — | JWKS endpoint for JWT verification |
| `SUPABASE_JWT_AUDIENCE` | yes | — | Expected `aud` claim in incoming JWTs |
| `SUPABASE_JWT_ISSUER` | yes | — | Expected `iss` claim in incoming JWTs |
| `SUPABASE_DB_URL` | yes | — | Supabase Postgres DSN (`postgres://...?sslmode=require`) |
| `DB_MAX_CONNS` | no | `10` | Maximum pool connections |
| `DB_MIN_CONNS` | no | `0` | Minimum pool connections kept alive |
| `DB_MAX_CONN_LIFETIME` | no | `30m` | Maximum lifetime of a pooled connection |
| `DB_MAX_CONN_IDLE_TIME` | no | `5m` | Maximum idle time before a connection is evicted |
| `PING_TOKEN` | yes | — | Bearer token for `POST /internal/ping`. Server refuses to start if empty. |
| `NOTION_TOKEN` | no\* | — | Notion integration token used by the internal Notion sync job. |
| `NOTION_PAGE_IDS` | no\* | — | Comma-separated Notion page IDs to fetch. Empty entries are ignored. |
| `NOTION_TARGET_OWNER_ID` | no\* | — | Existing `public.users.id`/Supabase auth user UUID that owns the synced cardgroup. Required because cardgroups are user-owned in the current schema. |
| `NOTION_TARGET_CARDGROUP_NAME` | no\* | — | Destination cardgroup name. The backend finds or creates this cardgroup for `NOTION_TARGET_OWNER_ID`. |
| `NOTION_SYNC_TOKEN` | no\* | — | Bearer token for `POST /internal/notion-sync`. |
| `NOTION_MAX_ATTEMPTS` | no | `5` | Retry attempt cap for Notion 429/5xx responses. Must be positive when set. |
| `NOTION_MAX_ELAPSED` | no | `2m` | Maximum cumulative Notion retry wait per request. Must be a positive Go duration when set. |
| `SUPER_USER_EMAILS` | no | *(empty)* | Comma-separated trusted email addresses promoted to `admin` on first authenticated request. See `docs/backend-auth.md` § "Bootstrap admin". |

`PORT`, `SHUTDOWN_TIMEOUT`, `NOTION_MAX_ATTEMPTS`, `NOTION_MAX_ELAPSED`, and `SUPER_USER_EMAILS` are optional with safe defaults. The three `SUPABASE_JWT_*` variables, `SUPABASE_DB_URL`, and `PING_TOKEN` are strictly required — the server refuses to start if any is missing.

\* **Optional as a group.** When any of the five `NOTION_*` sync vars (`NOTION_TOKEN`, `NOTION_PAGE_IDS`, `NOTION_TARGET_OWNER_ID`, `NOTION_TARGET_CARDGROUP_NAME`, `NOTION_SYNC_TOKEN`) is absent or whitespace-only, the `POST /internal/notion-sync` route is disabled and the server still starts — a single `WARN` log line is emitted listing the missing var names (via `OptionalConfigFromEnv` in `run()`). The exception: when the group is otherwise present, `NOTION_PAGE_IDS` must contain at least one non-whitespace ID — a comma/whitespace-only value fails startup.

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

### Test-only exported constructors

Usecases that need an injectable `txRunner` for resolver-level wire tests (e.g., `NewCardUsecaseWithTx`) are exported solely to let `package resolver_test` inject a hand-rolled mock. The trade-off is intentional: `txRunner` is unexported, so callers outside the package cannot misuse the seam; the alternative — moving tests into `package resolver` — gives up the `_test`-package isolation convention. When adding similar usecases, prefer this pattern over exposing production internals to tests via the non-`_test` package.

### Consumer-defined narrow interfaces over re-using the full repository / service interface

When a usecase only needs one or two methods of a repository or service, declare a local interface in the usecase file that lists *only* those methods. For example, `cardImportUsecase` consumes `CardImportCardRepository` (`UpsertManyTx` only) plus the shared `CardgroupOwnershipFinder` (`FindByID` only), rather than depending on the full card and cardgroup repository surfaces. The production constructor accepts the wide concrete repositories and the compiler checks structural conformance; tests pass stubs that implement just the narrow surface. This avoids the secondary problem of having to mock every method on the full interface in every test, and keeps the seam clear about which methods the usecase actually depends on.

### Resolver-level wire tests

To catch wire-format regressions that usecase-layer unit tests miss (e.g., `Int` codec changes, `extensions.code` shape), build a `handler.NewServer` against a `Resolver` whose UC fields point at hand-rolled mocks. The harness pattern is in `backend/graph/resolver/user_test.go`; `backend/graph/resolver/card_resolvers_test.go` is the second example. These tests verify that gqlgen correctly hydrates a generated input model AND that the resolver maps domain sentinels to the expected GraphQL error shape.

## Logging

`log/slog` with a `JSONHandler` on `os.Stderr`. `slog.SetDefault` registers the process-wide default, and `e.Logger = logger` shares the same logger with Echo so request logs and application logs use a single format.

## Backend gotchas

Library-quirk rules (Echo v5 signatures, GORM empty-`IN` behaviour, JWT algorithm whitelist, slog grouping, `crypto/subtle` length leak, etc.) live in `.claude/rules/go-library-gotchas.md`. The judgment-call patterns specific to this codebase remain below.

### Role repository sentinels

`repository.RoleRepository` exposes the full CRUD surface (`Create`, `Update`, `Delete`, `FindByID`, `FindByName`, `FindByIDs`, `ListAll`) plus the user-role join helpers (`AssignToUser`, `RevokeFromUser`, `ListByUser`, `ListByUserIDs`). Three sentinels classify the failure modes:

- `ErrUserNotFound` — joined with `ErrNotFound` (so legacy `errors.Is(_, ErrNotFound)` callers keep working).
- `ErrRoleNotFound` — joined with `ErrNotFound` for the same reason.
- `ErrRoleDuplicate` — **standalone**, not joined with `ErrNotFound`. A duplicate is a "found" condition; joining it would make a generic 404 mapper fire for a duplicate insert. See [`docs/backend/error-wrapping/sentinel-layering.md`](backend/error-wrapping/sentinel-layering.md).

`Create` and `Update` normalise the name with `strings.ToLower(strings.TrimSpace(name))` before insertion, and route Postgres `23505` unique violations through `classifyUniqueError` to `ErrRoleDuplicate`. The classifier anchors on the constraint-name fragment `"name"` rather than `"roles"` to avoid mis-routing `user_roles_pkey` into the role-name sentinel — see [`docs/backend/error-wrapping/postgres-unique-violation-23505.md`](backend/error-wrapping/postgres-unique-violation-23505.md).

`Delete` uses `RowsAffected == 0` to detect "id did not exist" rather than a separate existence check, because the `WHERE id = ?` predicate eliminates the empty-`IN` hazard that requires the `1=1` opt-out.

### Empty-patch optimization in repositories

A repository `Update` that receives a no-op patch (all fields nil or unchanged) should return the current row without issuing an `UPDATE`. This avoids unnecessarily touching `updated_at` and helps idempotent clients. Guard by checking whether the patch struct carries any non-nil field before building the GORM `Updates` call.

### Context cancellation propagation

When a resolver calls a downstream service (DB, JWKS, role lookup) and the caller's context is cancelled, the returned error wraps `context.Canceled` or `context.DeadlineExceeded`. **Forward those errors as-is** rather than wrapping them with `gqlerr.Internal` — wrapping them logs an ERROR line and emits an `INTERNAL` envelope for what is actually a client-driven cancellation (browser closed, navigation away, deadline hit). The pattern for blocking auth calls is:

```go
isAdmin, err := r.AuthSvc.IsAdmin(ctx, caller.Sub)
if err != nil {
    if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
        return nil, err
    }
    return nil, gqlerr.Internal(ctx, err)
}
```

Applies to every resolver that performs a blocking external call. Without this guard, `slog.ErrorContext` and any downstream alerting (Sentry, dashboards) get polluted by client cancellations that are not server bugs.

The same rule extends to **usecases that wrap a `db.Transaction` or call an injected service** (e.g. `CardgroupOwnershipFinder.FindByID`, `CardRepository.UpsertManyTx`). When a usecase is the layer that catches the error, return `context.Canceled` / `context.DeadlineExceeded` as-is and wrap only the genuine residual errors with the usecase's `eris` prefix. `cardImportUsecase.Import` applies this at both the ownership lookup and transaction-runner boundaries: owner lookup context errors pass through from `authorizeCardgroupOrBadInput`, and transaction errors pass through when `isContextDone(err)` matches.

### Legacy ports: revisit boundaries before re-translating

When porting an older module from a previous incarnation of the project, do not translate it line-for-line. Earlier abstractions almost always carry boundaries that were drawn for a system you no longer have — the textdic port temporarily kept ~200 lines of dead scaffolding (`StructuredError` interface, a token-translation table, a parser wrapper with its own mutex, a `wrappedParser` interface) until a code-simplifier sweep deleted them and the LoC dropped 32%. The cheaper path is to reread the call sites first, decide which boundaries the new code actually needs, and only port those. Surface area stripped out at port time is surface area no future reviewer has to argue about.

### Config struct validation belongs at the consumption boundary, not only the constructor

Validating fields only inside `ConfigFromEnv` is bypassable — callers can construct `Config{}` directly and pass empty strings to `jwt.WithAudience("")` / `jwt.WithIssuer("")`, which silently match any token claim. Add a `Validate() error` method to the config type and call it at the consumption point (e.g., the middleware factory). The factory should return `(echo.MiddlewareFunc, error)` so the DI wiring in `run()` catches misconfiguration at boot rather than at the first request. This pattern generalises to any config struct whose zero value is semantically dangerous.

## Observability

Tracing model, request-ID contract, and APQ wire format are documented in `docs/observability.md` (single source of truth across backend and frontend). Backend-only implementation notes follow.

### Tracing impl

The backend uses `github.com/ravilushqa/otelgqlgen` to instrument every GraphQL operation, resolver, and scalar field. Spans are exported via `otlptracehttp` to the endpoint named by `OTEL_EXPORTER_OTLP_ENDPOINT`.

**Library compatibility pins.** `github.com/ravilushqa/otelgqlgen` must be pinned to **v0.17.0** when using `github.com/99designs/gqlgen v0.17.66`. Upgrading to `otelgqlgen` v0.19.x transitively bumps gqlgen to v0.17.73, which changes the generated `ComplexityRoot` signature and breaks `backend/graph/generated/`. The v0.17.x line of `otelgqlgen` matches the gqlgen v0.17.66 generics era.

The APQ LRU cache must be constructed as `lru.New[string](100)` (generic form). gqlgen v0.17.66 made the cache interface generic; the non-generic `lru.New(100)` form shown in older online docs no longer compiles against this version.

**`otelgqlgen` span naming (empirically verified against v0.17.0).**

- **Operation span**: bare operation name — `Me`, `UpdateProfile`, `Health`. Not `query Me` and not `graphql.execute`.
- **Resolver / field spans**: `<ObjectType>/<fieldName>` — e.g. `Query/me`, `User/displayName`.

Test assertions that match span names (e.g. `strings.HasPrefix(name, "User/")`) are anchored to `otelgqlgen@v0.17.0`. If `otelgqlgen` is upgraded, run the integration tests — a span-naming change will surface immediately as a failing assertion before it silently breaks production dashboards.

**Noop fallback.** If `OTEL_EXPORTER_OTLP_ENDPOINT` is empty or missing, the server installs a no-op TracerProvider and logs `telemetry disabled`. Startup is **not** blocked on collector availability — this is deliberate so dev / CI do not require a running Jaeger.

**Sampler env-var validation.** `OTEL_TRACES_SAMPLER_ARG` parsing emits `slog.Warn` on two distinct conditions: a parse error (e.g. `0,1` with a comma instead of a decimal point) and an out-of-range value (outside `[0, 1]`). Both fall back to `1.0`. The two warnings are kept separate so operators can triage env config issues quickly — a comma typo produces a different message than a value of `1.5`.

**OTLP exporter goroutine ownership.** `otlptracehttp.New` starts background goroutines and a persistent HTTP client. If `Init` creates the exporter but a later step (e.g. resource construction) errors before the SDK takes ownership, the exporter is orphaned and leaks. The fix is to call `exp.Shutdown(ctx)` in the failure path of `Init` so the background goroutines are always cleaned up.

**`resource.New` partial-error handling.** `go.opentelemetry.io/otel/sdk/resource` returns `resource.ErrPartialResource` or `resource.ErrSchemaURLConflict` **alongside a usable `*Resource`** when one detector fails (e.g. a host-info detector blocked by container restrictions). Do not `return err` on these — use `errors.Is` to identify them, emit a `slog.Warn`, and pass the partial resource to `sdktrace.WithResource`. Aborting init on a partial-detector failure crashes the server in sandboxed environments where host introspection is restricted.

**`tracetest.InMemoryExporter` shutdown clears the buffer.** `exporter.Shutdown(ctx)` internally calls `Reset()`, which clears the recorded span buffer. The natural pattern `defer shutdown(); ...; spans := exp.GetSpans()` returns zero spans because the buffer was already cleared by the deferred shutdown. To read spans correctly: call `tp.ForceFlush(ctx)` to drain pending spans **before** calling `Shutdown`, or call `exp.GetSpans()` before `Shutdown` returns.

**Environment variables.**

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | *(empty)* | OTLP HTTP endpoint (e.g. `http://localhost:4318`). Empty = tracing disabled. |
| `OTEL_TRACES_SAMPLER_ARG` | no | `1.0` | Float in `[0, 1]`. Parse errors and out-of-range values each emit a `slog.Warn` and fall back to `1.0`. |
| `APP_ENV` | no | `development` | Used as `deployment.environment` resource attribute. Set to `production` on Render. |

### Request ID middleware

The middleware lives at `backend/internal/middleware/request_id.go` and reads `X-Request-ID` from the incoming request. Generation, length cap, and the contract with the frontend are documented in `docs/observability.md`.

**ID generation fallback.** When no valid upstream ID is present, the middleware generates one with `uuid.NewV7()`. If `NewV7` fails (e.g. the random source is temporarily unavailable), the fallback is a nanosecond timestamp string (`fmt.Sprintf("fallback-%d", time.Now().UnixNano())`).

**Do not replace the fallback with `uuid.NewString()`.** `uuid.NewString` calls `Must(uuid.NewRandom())` internally, which panics on the same `crypto/rand` failure that caused `NewV7` to fail — it is not a safe fallback.

**Middleware position.**

```go
e.Use(internalmw.RequestID())     // first — establishes the request ID for everything downstream
e.Use(middleware.Recover())        // second — its panic-recovery context already carries the ID
e.Use(middleware.RequestLogger())  // third — logs with the ID already in context
```

`RequestID` is registered **first** so the request ID is established before any other middleware runs. Because `Recover` and `RequestLogger` are registered after it, both see the ID already in context — panic-recovery error responses and request log lines all carry it. It applies globally — `/health`, `/playground`, and `/query` all receive it. The middleware sets `X-Request-ID` on the **response** before calling `next(c)`, so error paths also expose the header to callers.

**slog integration.** `main()` wires the request-aware logger:

```go
logger := slog.New(
    logging.NewContextHandler(
        slog.NewJSONHandler(os.Stderr, nil),
        internalmw.RequestIDFromContext,
    ),
)
slog.SetDefault(logger)
```

`logging.NewContextHandler` wraps any `slog.Handler`. On every `Handle` call it reads the request ID from the record's context via the supplied `ContextLookup` function and appends it as `"request_id"` before delegating to the inner handler.

**Decoupling via `ContextLookup`.** `logging` does not import `middleware`. The wiring above passes `internalmw.RequestIDFromContext` as a plain `func(context.Context) string` so the two packages remain independent of each other's import graph.

**Helper.**

```go
id := internalmw.RequestIDFromContext(ctx) // returns "" when not present
```

**Shutdown ordering.** The graceful-shutdown goroutine shuts down in this order:

1. `srv.Shutdown(sctx)` — drain in-flight HTTP requests.
2. `db.Close()` — release the pgx pool.
3. `tracerShutdown(sctx)` — flush pending spans to the exporter.

Tracer shutdown is last so that DB-layer spans emitted during request drain still reach the exporter. Tracer shutdown errors log a warning but do not fail the process.

## Error wrapping convention

The backend uses `eris` as the only wrapping library; `fmt.Errorf("%w")` is forbidden in `backend/internal/` and `backend/cmd/` and CI enforces this. Full convention (rule table, sentinel policy, `LogError` / `LogWarn` logging shape, and the `error_chain` JSON schema) lives in `.claude/rules/error-wrapping.md`.

## Further reading

| Topic | Doc |
|---|---|
| GraphQL endpoint, gqlgen, resolvers, error helpers, DataLoader, complexity, validation | `docs/backend-graphql.md` |
| Authentication, JWT, JWKS, role authz | `docs/backend-auth.md` |
| Database, migrations, RLS, schema_migrations, GORM patterns | `docs/backend-db.md` |
| Tracing + Request ID + APQ contract (cross-side) | `docs/observability.md` |
| Error wrapping (eris convention) | `.claude/rules/error-wrapping.md` |
| Library quirks (Echo v5, GORM, JWT, slog, crypto/subtle) | `.claude/rules/go-library-gotchas.md` |
| Pagination contract | `.claude/rules/pagination.md` |
