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

### Layered sentinels via `errors.Join`

When introducing a more specific sentinel alongside an existing general one (e.g. adding `ErrUserNotFound` while `ErrNotFound` is still in use across the repository), return `errors.Join(specific, general)` from the new call site. Callers that already match `errors.Is(err, ErrNotFound)` keep working; callers that want the finer split can branch on the specific sentinel first. This avoids a flag-day rename across every caller and lets the finer sentinel migrate in at its own pace. **Always check the more specific sentinel before the general one** — `errors.Is` returns true for both, so reversing the order silently routes user-not-found into a generic 404 path.

### Postgres FK violation classification (`23503`)

A "validate parent rows exist, then insert" pattern carries a TOCTOU race: between the SELECT and the INSERT, another transaction can delete the parent row, and the INSERT then fails with a Postgres foreign-key violation. Without classification, the usecase maps the raw GORM error to `gqlerr.Internal` and the operator gets a 5xx alarm for what is actually client-supplied stale input.

Inspect the unwrapped driver error for `*pgconn.PgError` with `Code == "23503"` and read `ConstraintName` to decide which parent was missing — typical shape:

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23503" {
    switch {
    case strings.Contains(pgErr.ConstraintName, "user_id"):
        return errors.Join(ErrUserNotFound, ErrNotFound)
    case strings.Contains(pgErr.ConstraintName, "role_id"):
        return errors.Join(ErrRoleNotFound, ErrNotFound)
    }
}
```

The usecase then translates the specific sentinel to `gqlerr.BadUserInput` on the offending field. A race-deleted parent is a client-fixable input, not a server bug — keep it out of the ERROR log.

### Postgres unique-violation classification (`23505`)

The same shape applies when the DB rejects an `INSERT` or `UPDATE` for colliding with an existing row. Inspect `*pgconn.PgError` with `Code == "23505"` and read `ConstraintName`, then return a dedicated sentinel (e.g. `ErrRoleDuplicate`) the usecase can translate to `gqlerr.BadUserInput("name", "role name already exists")`. Without classification, the duplicate surfaces as `gqlerr.Internal` and the operator gets a 5xx alarm for a routine "name already taken" case.

**Anchor `ConstraintName` matches on the narrowest unambiguous fragment.** `strings.Contains(pgErr.ConstraintName, "roles")` would also match `user_roles_pkey` and route a join-table primary-key collision into `ErrRoleDuplicate`, which is wrong. The roles table emits its unique constraint on the `name` column, so the precise check is `strings.Contains(pgErr.ConstraintName, "name")`. The same rule extends to any future unique sentinel: pick the column or constraint suffix that no other constraint in the schema can collide with.

### Standalone sentinels: not every new sentinel joins `ErrNotFound`

The layered-sentinel pattern (`errors.Join(specific, general)`) only works when the specific case is **semantically a refinement** of the general case. `ErrUserNotFound` and `ErrRoleNotFound` refine `ErrNotFound`, so joining is correct: a caller branching only on `ErrNotFound` still gets the right behaviour. But `ErrRoleDuplicate` is the inverse condition — the row was *found* and that is precisely the failure. Joining it with `ErrNotFound` would make `errors.Is(err, ErrNotFound)` true for a duplicate insert, which is a lie that any general-purpose 404 mapper would happily act on. Keep "found" sentinels (duplicate, conflict, already-exists) standalone; only "missing" sentinels get the `errors.Join` treatment.

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
