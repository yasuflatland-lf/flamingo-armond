# Postgres advisory lock for race-safe ensure-by-name when no UNIQUE constraint exists

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A repository method shaped like `EnsureByName(ctx, ownerID, name) (*X, error)` (lookup → create-when-absent) has a check-then-act race: two callers entering the function with the same `(ownerID, name)` can both observe `gorm.ErrRecordNotFound` and both insert, producing duplicate rows. The race window survives `WithContext` and `Transaction` because the `SELECT` and the `INSERT` are separate statements — the transaction isolation level alone does not serialize them.

The fix used by `cardgroupRepo.EnsureByName` is `pg_advisory_xact_lock(hashtext(?), hashtext(?))` taken at the top of the transaction:

```go
err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    if err := tx.Exec(
        "SELECT pg_advisory_xact_lock(hashtext(?), hashtext(?))",
        ownerID, name,
    ).Error; err != nil {
        return eris.Wrap(err, "...: advisory lock")
    }

    var row gormCardgroup
    err := tx.Where("owner_id = ? AND name = ?", ownerID, name).Take(&row).Error
    // existing-row branch …
    // create branch …
})
```

The two `hashtext` arguments encode the lock key — the lock is keyed on `(owner_id, name)` so concurrent callers with different keys don't serialize against each other. The `_xact_` variant releases the lock at transaction commit, so callers cannot leak locks by forgetting to release.

## When this beats adding a `UNIQUE` constraint

A `UNIQUE(owner_id, name)` constraint would be simpler. The reason `cardgroups` does not have one is that the table's *public* duplicate-name semantics are intentional: two different owners may share a cardgroup name, and a single owner may *retain* historical duplicates created before the ensure-path existed. Adding `UNIQUE(owner_id, name)` would change behavior for non-ensure callers (e.g. `Create`) — failing existing inserts that the schema currently allows. The advisory lock localizes the serialization to the ensure-path without touching the public constraint surface.

## Hash collisions are a perf concern, not a correctness concern

`hashtext` is a 32-bit hash, so two distinct `(owner_id, name)` pairs can map to the same lock key. Colliding pairs serialize against each other unnecessarily, but the SELECT-then-INSERT logic inside the lock is still correct — the `WHERE owner_id = ? AND name = ?` predicate uses the real values, not the hash, so a collision produces a queue, not a wrong row. The collision rate is low enough at any plausible cardgroup count that the perf cost is negligible.

## Cross-aggregate use

The same pattern works for any `Ensure-by-natural-key` repository method on a table without a matching UNIQUE constraint. Two arguments are enough for a composite key.
