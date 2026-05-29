# Tx and non-Tx repository methods share a private helper to avoid drift

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a repository needs to expose both a standalone version and a transactional
version of the same query, do not copy the body. Extract a private helper that
accepts `*gorm.DB` directly and call it from both public methods:

```go
// backend/internal/repository/card.go

// FindDueCardsForUser is the standalone version — uses the repo's default DB handle.
func (r *cardRepo) FindDueCardsForUser(
    ctx context.Context,
    userID, cardgroupID string,
    now time.Time,
    limit int,
) ([]domain.DueCard, error) {
    return findDueCardsOn(r.db.WithContext(ctx), userID, cardgroupID, now, limit)
}

// FindDueCardsForUserTx is the transaction-aware version — operates on the caller's tx.
func (r *cardRepo) FindDueCardsForUserTx(
    ctx context.Context,
    tx *gorm.DB,
    userID, cardgroupID string,
    now time.Time,
    limit int,
) ([]domain.DueCard, error) {
    return findDueCardsOn(tx.WithContext(ctx), userID, cardgroupID, now, limit)
}

// findDueCardsOn holds the shared implementation.
// db is either the default handle (from FindDueCardsForUser) or a transaction handle
// (from FindDueCardsForUserTx).
func findDueCardsOn(db *gorm.DB, userID, cardgroupID string, now time.Time, limit int) ([]domain.DueCard, error) {
    userID = coalesceUserIDForJoin(userID)
    if limit <= 0 {
        return []domain.DueCard{}, nil
    }
    var rows []dueCardRow
    if err := db.
        Table("cards").
        Select("cards.id, cards.cardgroup_id, cards.front, cards.back, cards.created_at, cards.updated_at, cards.position, ucs.state, ucs.due").
        Joins("LEFT JOIN user_card_fsrs ucs ON ucs.user_id = ? AND ucs.card_id = cards.id", userID).
        Where("cards.cardgroup_id = ? AND (ucs.due IS NULL OR ucs.due <= ?)", cardgroupID, now).
        Order("COALESCE(ucs.due, cards.created_at) ASC, cards.position ASC, cards.id ASC").
        Limit(limit).
        Find(&rows).Error; err != nil {
        return nil, eris.Wrap(err, "repository: card: find due cards")
    }
    // ... build []domain.DueCard from rows ...
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

**Per-user variant naming:** when the query takes a `userID` because it joins
against a per-user table (here `user_card_fsrs`), the public methods carry a
`ForUser` segment between the operation and the `Tx` suffix
(`FindDueCardsForUser` / `FindDueCardsForUserTx`). The private helper drops the
`ForUser` segment and treats `userID` as a positional argument — the
shared-helper name describes the operation; the per-user variant lives only on
the public surface that callers grep against. Note also that
[GORM scan targets with embedded TableName methods](gorm-embedded-tablename-scan-confusion.md)
can silently break LEFT JOIN projection, so the private helper uses a flat
scan target (`dueCardRow`) rather than embedding `gormCard`.

**Reference:** `backend/internal/repository/card.go` — `FindDueCardsForUser`,
`FindDueCardsForUserTx`, and `findDueCardsOn`. The same `On`-suffix pattern is used by
`findCardByID` for `FindByID` / `FindByIDTx`.
