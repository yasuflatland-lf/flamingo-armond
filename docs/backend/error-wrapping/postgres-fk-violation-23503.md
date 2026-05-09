# Postgres FK violation classification (`23503`)

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

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
