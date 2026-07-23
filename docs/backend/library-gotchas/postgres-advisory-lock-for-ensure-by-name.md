# Postgres advisory lock for race-safe ensure-by-name when no UNIQUE constraint exists

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A repository method shaped like `masterCardgroupRepo.EnsureByName(ctx, name) (*X, error)` (lookup → create-when-absent) has a check-then-act race: two callers entering the function with the same name can both observe `gorm.ErrRecordNotFound` and both insert, producing duplicate rows. The race window survives `WithContext` and `Transaction` because the `SELECT` and the `INSERT` are separate statements — the transaction isolation level alone does not serialize them.

The fix used by `masterCardgroupRepo.EnsureByName` is `pg_advisory_xact_lock(hashtext(?), hashtext(?))` taken at the top of the transaction:

```go
err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    if err := tx.Exec(
        "SELECT pg_advisory_xact_lock(hashtext(?), hashtext(?))",
        "master", canonical,
    ).Error; err != nil {
        return eris.Wrap(err, "...: advisory lock")
    }

    var row gormMasterCardgroup
    err := tx.Where("name = ?", canonical).Take(&row).Error
    // existing-row branch …
    // create branch …
})
```

The two `hashtext` arguments encode the lock key — a fixed `'master'` namespace plus the parsed (trimmed) name — so concurrent callers with different names don't serialize against each other. The `_xact_` variant releases the lock at transaction commit, so callers cannot leak locks by forgetting to release.

## When this beats adding a `UNIQUE` constraint

A `UNIQUE(name)` constraint would be simpler. The reason `master_cardgroups` does not have one is that the constraint would apply to *every* insert path, not just the ensure-path: `Create` currently permits duplicate names, and tightening that is a public-semantics change for its callers. The advisory lock localizes the serialization to the ensure-path without touching the public constraint surface.

## Hash collisions are a perf concern, not a correctness concern

`hashtext` is a 32-bit hash, so two distinct lock keys can collide. Colliding keys serialize against each other unnecessarily, but the SELECT-then-INSERT logic inside the lock is still correct — the `WHERE name = ?` predicate uses the real value, not the hash, so a collision produces a queue, not a wrong row. The collision rate is low enough at any plausible cardgroup count that the perf cost is negligible.

## Cross-aggregate use

The same pattern works for any `Ensure-by-natural-key` repository method on a table without a matching UNIQUE constraint. Two `hashtext` arguments are enough for a composite key — key the lock on the natural key's columns (e.g. `(owner_id, name)`) instead of a fixed namespace.
