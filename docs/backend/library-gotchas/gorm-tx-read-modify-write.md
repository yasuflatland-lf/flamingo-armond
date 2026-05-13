# Transactional read-modify-write must use the `tx` handle, not `r.db`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a usecase performs a read → compute → write sequence inside a `db.Transaction` closure, every database call in the closure — including the initial read — must use the `tx *gorm.DB` handle passed to that closure. Using the repository's base `r.db` handle for the read breaks snapshot isolation: a concurrent writer can commit between the non-transactional read and the subsequent write, and the stale value silently overwrites the newer one.

```go
// Wrong: read uses r.db (non-transactional) even though the write uses tx.
err = u.tx(ctx, func(tx *gorm.DB) error {
    existing, err := u.fsrsRepo.FindByUserAndCardIDs(ctx, userID, cardIDs) // r.db inside!
    if err != nil {
        return err
    }
    newState := compute(existing)
    return u.fsrsRepo.UpsertTx(ctx, tx, newState) // tx here
})

// Correct: both operations run on the same tx handle.
err = u.tx(ctx, func(tx *gorm.DB) error {
    existing, err := u.fsrsRepo.FindByUserAndCardIDsTx(ctx, tx, userID, cardIDs)
    if err != nil {
        return err
    }
    newState := compute(existing)
    return u.fsrsRepo.UpsertTx(ctx, tx, newState)
})
```

**How to expose a transactional read:** follow the shared-helper pattern described in [Tx and non-Tx repository methods share a private helper to avoid drift](repo-tx-and-nontx-share-private-helper.md). Add a `FindXxxTx(ctx, tx, ...)` method that delegates to a private `findXxxOn(db *gorm.DB, ...)` helper. The standalone `FindXxx` calls the helper with `r.db`; the transactional version calls it with `tx`. Both the interface consumed by the usecase and the narrow interface (`XxxRepoForYyy`) must declare the `Tx` variant so the closure can call it.

**Why this matters:** GORM's `r.db` handle opens a new connection on each call. Two concurrent swipes targeting the same `(userID, cardID)` row each read the current FSRS state independently, compute a new state, then both write. The second write sees the _original_ state rather than the result of the first swipe, silently discarding the scheduling update. The `tx` handle holds the transaction open and ensures the read is part of the same snapshot as the write.

Reference: `backend/internal/repository/user_card_fsrs.go` — `FindByUserAndCardIDs` (non-tx) and `FindByUserAndCardIDsTx` (tx variant). `backend/internal/usecase/swipe.go` — `HandleSwipe` uses `FindByUserAndCardIDsTx` inside its transaction closure.
