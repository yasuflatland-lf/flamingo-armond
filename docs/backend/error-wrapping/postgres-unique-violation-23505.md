# Postgres unique-violation classification (`23505`)

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

The same shape applies when the DB rejects an `INSERT` or `UPDATE` for colliding with an existing row. Inspect `*pgconn.PgError` with `Code == "23505"` and read `ConstraintName`, then return a dedicated sentinel (e.g. `ErrRoleDuplicate`) the usecase can translate to `gqlerr.BadUserInput("name", "role name already exists")`. Without classification, the duplicate surfaces as `gqlerr.Internal` and the operator gets a 5xx alarm for a routine "name already taken" case.

**Anchor `ConstraintName` matches on the narrowest unambiguous fragment.** `strings.Contains(pgErr.ConstraintName, "roles")` would also match `user_roles_pkey` and route a join-table primary-key collision into `ErrRoleDuplicate`, which is wrong. The roles table emits its unique constraint on the `name` column, so the precise check is `strings.Contains(pgErr.ConstraintName, "name")`. The same rule extends to any future unique sentinel: pick the column or constraint suffix that no other constraint in the schema can collide with.

**Add a negative integration test for any 23505 classification branch.** A positive test confirms that the target constraint maps to the sentinel; a negative test confirms that a *different* 23505 violation (e.g. a primary-key collision via `cards_pkey`) does **not** mis-route into the same sentinel. Without the negative test, widening the `strings.Contains` fragment in a future refactor silently routes unrelated unique violations through the wrong sentinel — the positive test stays green and the mis-classification ships undetected:

```go
// Force a 23505 on a different constraint (primary key) and assert the error
// is NOT classified as the typed sentinel.
second.ID = fixedID
err := repo.Create(ctx, second)
require.Error(t, err)
require.False(t, errors.Is(err, repository.ErrCardDuplicateFront),
    "cards_pkey violation must not be classified as ErrCardDuplicateFront; got %v", err)
var pgErr *pgconn.PgError
require.True(t, errors.As(err, &pgErr))
require.Equal(t, "23505", pgErr.Code) // confirm we hit the intended path
```
