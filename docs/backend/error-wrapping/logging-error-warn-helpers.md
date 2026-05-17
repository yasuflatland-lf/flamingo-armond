# Logging: `LogError` / `LogWarn` helpers

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

All error sites that produce a structured log entry must attach the eris chain as the `error_chain` attribute (output of `eris.ToJSON(err, true)`). The `internal/logging` package exposes two sibling helpers — `LogError(ctx, logger, msg, err, attrs...)` and `LogWarn(ctx, logger, msg, err, attrs...)` — so ERROR and WARN sites share one shape. Both:

- attach `error_chain` automatically,
- emit at the obvious level (`slog.LevelError` / `slog.LevelWarn`),
- are a no-op when `err == nil`.

Use `LogWarn` for any non-ERROR site that needs the `error_chain` attribute; do not hand-roll `slog.Warn(... eris.ToJSON ...)` calls.

When a wrapper already forwards to a variadic helper (e.g. `logging.LogError(ctx, logger, msg, err, attrs...)`), expose the variadic slot at the wrapper rather than minting a sibling function. `gqlerr.Internal(ctx, err, attrs ...slog.Attr)` follows this shape: existing two-arg call sites keep compiling unchanged, and new sites can attach structured triage context (e.g. `slog.String("cardgroup_id", id)`) to the log line without a name-change cascade. The extra attrs appear only in the ERROR log — the wire response is always the same `{code: INTERNAL, message: "internal server error"}` envelope.

### Don't pass `context.Background()` to context-aware loggers

`slog.Default()` is wired via `NewContextHandler` to extract `request_id` from the
context. Calls that pass `context.Background()` — for example, the package-level
convenience functions `slog.Warn(msg)`, `slog.Error(msg)`, which route through
`slog.Default().Log(context.Background(), ...)` — lose `request_id` correlation even
though the log line still lands. Always thread the caller-provided `ctx` to context-aware
logging:

```go
// Wrong: request_id is lost
slog.Warn("something happened", "key", val)

// Correct: request_id propagates from the incoming request context
slog.WarnContext(ctx, "something happened", "key", val)
```

Prefer `slog.<Level>Context(ctx, msg, args...)` from anywhere that has a real ctx
(resolvers, usecases, helpers reached from the request goroutine). `context.Background()`
is acceptable only in process-level startup or shutdown paths where no request context
exists (e.g. `main()` before the HTTP server starts).

Today's call sites:

1. **`gqlerr.Internal(ctx, err)`** — every internal-server error returned through GraphQL (uses `LogError`, ERROR level).
2. **`cmd/server/main.go main()`** — terminal error before `os.Exit(1)` (uses `LogError`, ERROR level).
3. **`gqlerr.Cancelled(ctx, err)`** — client cancellation / server timeout returned through GraphQL (uses `LogWarn`, WARN level).
4. **`auth/middleware.go reject(c, cause)`** — token-rejection log (uses `LogWarn`, WARN level).

### Usecase-layer logger dependency injection

Production code under `backend/internal/usecase/` MUST log through an injected `uc.logger *slog.Logger` field — never via the bare package-level `slog.Warn` / `slog.WarnContext` / `slog.Info` / `slog.InfoContext` / `slog.Error` / `slog.ErrorContext` / `slog.Default()` / `slog.LogAttrs` calls. Every usecase struct carries a logger field set via constructor; the constructor panics if the logger is nil (matches the [`constructor-panics-for-non-empty-config`](../library-gotchas/constructor-panics-for-non-empty-config.md) rule). The composition root in `cmd/server/main.go` plumbs the application's `*slog.Logger` into every `usecase.NewXxx(...)` call as the trailing positional argument.

The `gqlerr.Internal` / `gqlerr.Cancelled` constructors in `backend/internal/gqlerr/` are deliberately exempt — the transport layer is the correct seam for the process-default logger because it has no upstream caller to inject from. Usecase code does not have that excuse.

Acceptance check (run from the repo root):

```bash
grep -rnE 'slog\.(Default|SetDefault|Warn|WarnContext|Info|InfoContext|Error|ErrorContext|LogAttrs)\(' \
    backend/internal/usecase/ --include='*.go' | grep -v '_test.go'
```

Expected: empty. Any match is a regression.
