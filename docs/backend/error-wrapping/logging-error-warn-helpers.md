# Logging: `LogError` / `LogWarn` helpers

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

All error sites that produce a structured log entry must attach the eris chain as the `error_chain` attribute (output of `eris.ToJSON(err, true)`). The `internal/logging` package exposes two sibling helpers — `LogError(ctx, logger, msg, err, attrs...)` and `LogWarn(ctx, logger, msg, err, attrs...)` — so ERROR and WARN sites share one shape. Both:

- attach `error_chain` automatically,
- emit at the obvious level (`slog.LevelError` / `slog.LevelWarn`),
- are a no-op when `err == nil`.

Use `LogWarn` for any non-ERROR site that needs the `error_chain` attribute; do not hand-roll `slog.Warn(... eris.ToJSON ...)` calls.

When a wrapper already forwards to a variadic helper (e.g. `logging.LogError(ctx, logger, msg, err, attrs...)`), expose the variadic slot at the wrapper rather than minting a sibling function. `gqlerr.Internal(ctx, err, attrs ...slog.Attr)` follows this shape: existing two-arg call sites keep compiling unchanged, and new sites can attach structured triage context (e.g. `slog.String("cardgroup_id", id)`) to the log line without a name-change cascade. The extra attrs appear only in the ERROR log — the wire response is always the same `{code: INTERNAL, message: "internal server error"}` envelope.

Today's call sites:

1. **`gqlerr.Internal(ctx, err)`** — every internal-server error returned through GraphQL (uses `LogError`, ERROR level).
2. **`cmd/server/main.go main()`** — terminal error before `os.Exit(1)` (uses `LogError`, ERROR level).
3. **`gqlerr.Cancelled(ctx, err)`** — client cancellation / server timeout returned through GraphQL (uses `LogWarn`, WARN level).
4. **`auth/middleware.go reject(c, cause)`** — token-rejection log (uses `LogWarn`, WARN level).
