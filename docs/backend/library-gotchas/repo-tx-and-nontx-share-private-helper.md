# Tx and non-Tx repository methods share a private helper to avoid drift

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a repository needs to expose both a standalone version and a transactional
version of the same query, do not copy the body. Extract a private helper that
accepts `*gorm.DB` directly and call it from both public methods:

```go
// backend/internal/repository/card.go

// FindDueCards is the standalone version — uses the repo's default DB handle.
func (r *cardRepo) FindDueCards(
    ctx context.Context,
    cardgroupID string,
    now time.Time,
    limit int,
) ([]*domain.Card, error) {
    return findDueCardsOn(r.db.WithContext(ctx), cardgroupID, now, limit)
}

// FindDueCardsTx is the transaction-aware version — operates on the caller's tx.
func (r *cardRepo) FindDueCardsTx(
    ctx context.Context,
    tx *gorm.DB,
    cardgroupID string,
    now time.Time,
    limit int,
) ([]*domain.Card, error) {
    return findDueCardsOn(tx.WithContext(ctx), cardgroupID, now, limit)
}

// findDueCardsOn holds the shared implementation.
// db is either the default handle (from FindDueCards) or a transaction handle
// (from FindDueCardsTx).
func findDueCardsOn(db *gorm.DB, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error) {
    if limit <= 0 {
        return []*domain.Card{}, nil
    }
    var rows []gormCard
    if err := db.
        Where("cardgroup_id = ? AND due <= ?", cardgroupID, now).
        Order("due ASC, id ASC").
        Limit(limit).
        Find(&rows).Error; err != nil {
        return nil, eris.Wrap(err, "repository: find due cards")
    }
    out := make([]*domain.Card, len(rows))
    for i := range rows {
        out[i] = cardToDomain(rows[i])
    }
    return out, nil
}
```

**Why this matters:** a copy-pasted body will eventually drift. The standalone
version might receive an index hint or a filter clause that the Tx version misses,
or the Tx version picks up a bug fix that the standalone version never gets. The
shared helper is the single source of truth — fix it once, both callers benefit.

**Naming convention:** the private helper is named after the operation with an `On`
suffix, accepting `*gorm.DB` as its first argument. This pattern is consistent
with the existing `findCardByID(ctx, db, id)` helper used by `FindByID` and
`FindByIDTx`.

**Reference:** `backend/internal/repository/card.go` — `FindDueCards`,
`FindDueCardsTx`, and `findDueCardsOn`. The same `On`-suffix pattern is used by
`findCardByID` for `FindByID` / `FindByIDTx`.
