# Defensive `eris.Wrap` at log sites that consume narrow interfaces

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

When a log site receives an `error` from a method on a **consumer-defined narrow interface** (e.g. `adminCounter.CountAdmins` in `backend/cmd/server/main.go`), the production implementation may return an eris-wrapped error today, but a future stub or alternate implementation is free to return a stdlib `errors.New` value — at which point `eris.ToJSON(err, true)` emits an `external`-only payload with no `root.stack`. The fix is to wrap once at the log site itself before handing off to `LogWarn` / `LogError`:

```go
adminCount, err := counter.CountAdmins(ctx)
if err != nil {
    logging.LogWarn(ctx, logger, "admin bootstrap: admin count check failed",
        eris.Wrap(err, "run: count admin users for bootstrap check"))
    return
}
```

The wrap is cheap (one frame of stack), idempotent (wrapping an already-eris error nests cleanly under `wrap[]` while preserving the original `root`), and guarantees the log line carries a `root.stack` regardless of which interface implementation produced the error. Used in `warnIfNoAdmin` (`backend/cmd/server/main.go`).
