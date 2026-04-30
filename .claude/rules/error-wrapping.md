# Error wrapping convention

> Applies to: `backend/internal/`, `backend/cmd/`. Enforced by CI (`.github/workflows/backend.yml`).

The backend uses [`github.com/rotisserie/eris`](https://github.com/rotisserie/eris) as the **only** error-wrapping library inside `backend/internal/` and `backend/cmd/`. The convention is:

| Situation | Use |
|---|---|
| New error at the originating call site | `eris.New("layer: short message")` |
| Wrapping an error at a layer boundary | `eris.Wrap(err, "layer: context")` |
| Wrapping with format args | `eris.Wrapf(err, "layer: %s", id)` |
| Validation error without an underlying cause | `eris.Errorf("layer: %s must be >= %d", field, min)` |
| Sentinel that other code matches via `errors.Is` | `errors.New("...")` (do **not** use eris) |

`fmt.Errorf("...: %w", err)` is **forbidden** in `backend/internal/` and `backend/cmd/`. CI fails the build if any such call sneaks back in (see `.github/workflows/backend.yml`).

## Sentinels

Sentinels used today: `repository.ErrNotFound`, and domain-level sentinels such as `domain.ErrCardgroupNameRequired` / `domain.ErrCardgroupNameTooLong`. New sentinels are allowed when (a) callers need to branch on identity, and (b) a string-equality match is fragile. Keep sentinels as plain `errors.New` so `errors.Is` works without going through eris's chain walk.

## Logging

All error sites that produce a structured log entry must attach the eris chain as the `error_chain` attribute (output of `eris.ToJSON(err, true)`). The `internal/logging` package exposes two sibling helpers — `LogError(ctx, logger, msg, err, attrs...)` and `LogWarn(ctx, logger, msg, err, attrs...)` — so ERROR and WARN sites share one shape. Both:

- attach `error_chain` automatically,
- emit at the obvious level (`slog.LevelError` / `slog.LevelWarn`),
- are a no-op when `err == nil`.

Use `LogWarn` for any non-ERROR site that needs the `error_chain` attribute; do not hand-roll `slog.Warn(... eris.ToJSON ...)` calls.

Today's call sites:

1. **`gqlerr.Internal(ctx, err)`** — every internal-server error returned through GraphQL (uses `LogError`, ERROR level).
2. **`cmd/server/main.go main()`** — terminal error before `os.Exit(1)` (uses `LogError`, ERROR level).
3. **`gqlerr.Cancelled(ctx, err)`** — client cancellation / server timeout returned through GraphQL (uses `LogWarn`, WARN level).
4. **`auth/middleware.go reject(c, cause)`** — token-rejection log (uses `LogWarn`, WARN level).

## Why `eris` over alternatives

- **`fmt.Errorf("%w")`**: no stack trace, can only carry a string context.
- **`pkg/errors`**: last tagged release v0.9.1 in January 2020 with no upstream activity since; lacks the structured JSON chain serialization that this codebase relies on for the `error_chain` log attribute.
- **`cockroachdb/errors`**: heavier, drags in many transitive deps; revisit only when multi-service error portability or first-class Sentry SDK integration becomes a hard requirement.
- **`joomcode/errorx`**: typed-error focus, less aligned with our wrap-and-log need.

## What `error_chain` looks like

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
