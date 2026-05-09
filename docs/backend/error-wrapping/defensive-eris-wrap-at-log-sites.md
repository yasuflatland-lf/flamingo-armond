# Defensive `eris.Wrap` at log sites that consume narrow interfaces

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

When a log site receives an `error` from a method on a **consumer-defined narrow interface** (e.g. `auth.adminChecker.IsAdmin`, `auth.roleAssigner.AssignToUser`), the production implementation may return an eris-wrapped error today, but a future stub or alternate implementation is free to return a stdlib `errors.New` value — at which point `eris.ToJSON(err, true)` emits an `external`-only payload with no `root.stack`. The fix is to wrap once at the log site itself before handing off to `LogWarn` / `LogError`:

```go
isAdmin, err := p.checker.IsAdmin(ctx, u.Sub)
if err != nil {
    logging.LogWarn(ctx, slog.Default(), "superuser: admin check failed",
        eris.Wrap(err, "superuser: IsAdmin"),
        slog.String("user_id", u.Sub))
    return next(c)
}
```

The wrap is cheap (one frame of stack), idempotent (wrapping an already-eris error nests cleanly under `wrap[]` while preserving the original `root`), and guarantees the log line carries a `root.stack` regardless of which interface implementation produced the error. Used in `auth/superuser.go` at both the `IsAdmin` and `AssignToUser` failure sites.
