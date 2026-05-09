# GORM exact-match `FindBy*` helpers: callers own trimming, repos own nothing

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A `FindByXxx` method that uses `WHERE col = ?` performs byte-for-byte equality — no `TRIM`, no `LOWER`, no `LIKE`. The caller (usecase layer) is responsible for `strings.TrimSpace` before invoking the repo. The repo must not silently normalise input, because any future LIKE/LOWER "convenience" change would make the unique-index enforcement and the lookup disagree. Regression-guard the contract with a negative integration test:

```go
// Padded front must not match an exactly-stored value.
_, err := repo.FindByCardgroupAndFront(ctx, cg.ID, " apple ")
require.True(t, errors.Is(err, repository.ErrNotFound),
    "padded front must not match exact-stored value; got %v", err)
```

This pairs with the [GORM `LIKE` / `ILIKE` escaping rule](gorm-like-ilike-escape.md): both rules block accidental broadening of match semantics on a column that backs a unique index.
