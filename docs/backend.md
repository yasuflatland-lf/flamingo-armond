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

- `PORT` — listen port. Default `1323`.
- `SHUTDOWN_TIMEOUT` — Go duration (e.g. `10s`, `1m`). Default `25s`. Invalid or `<= 0` values log a warning and fall back to the default — **do not remove this guard**; it prevents env-var typos from being silently ignored in production.

## Testing patterns

- **Handler unit tests use `t.Parallel()`** (see `TestHealthEndpoint`, `TestRootEndpoint`).
- **Lifecycle tests are sequential** (no `t.Parallel()`). They affect process-wide state.
- **Free port discovery**: `net.Listen("tcp", "127.0.0.1:0")` → read `Addr()` → `Close()` → start the server on that port (`freePort` + `waitHealthy` polling). Never hardcode ports.
- **Do not send real signals to the test process** (e.g. `syscall.Kill(os.Getpid(), SIGTERM)`). Signals are delivered process-wide and race with `t.Parallel()` tests and the test runner itself. Reproduce the meaning of `signal.NotifyContext` by **cancelling a `context.WithCancel` directly**.
- **Silence logs in tests** with `slog.New(slog.DiscardHandler)`.

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

### Playground

`GET /playground` exposes a browser UI for hand-crafted queries against `/query`. It is currently enabled in all environments. Introspection and this route should be env-gated before production use.

### Resolver DI seam

`newRouter(resolvers *resolver.Resolver) *echo.Echo` is the new wiring seam. `run(ctx, logger) error` remains the lifecycle seam — it constructs the `Resolver`, passes it to `newRouter`, and owns the `http.Server`. Future dependencies (auth, DB, loaders) add fields to `Resolver` and wire them in `run`.
